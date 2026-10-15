package web

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"
)

// webSession is a session as the request path sees it: the identity from
// internal/auth plus the CSRF token this package owns.
type webSession struct {
	Username  string
	Token     string
	ExpiresAt time.Time
	CSRFToken string
}

// sessionFrom resolves the caller's session from its cookie.
//
// The cookie holds the session token, which is 256 bits from crypto/rand and is
// never derived from anything about the user. It is compared by lookup, not by
// recomputation, so there is no secret to time-leak here.
func (s *Server) sessionFrom(r *http.Request) (webSession, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return webSession{}, false
	}
	sess, ok := s.Sessions.Lookup(c.Value)
	if !ok {
		return webSession{}, false
	}
	csrf, ok := s.csrf.lookup(c.Value)
	if !ok {
		// The session exists but its CSRF token does not. That is only possible if
		// the two stores disagree about lifetime, and the safe reading of a token
		// this package cannot produce is "no session" — otherwise the caller gets
		// past the auth gate and then every POST they make 403s with no explanation.
		return webSession{}, false
	}
	return webSession{
		Username:  sess.Username,
		Token:     c.Value,
		ExpiresAt: sess.ExpiresAt,
		CSRFToken: csrf,
	}, true
}

// startSession creates the session and sets the cookie.
//
// The cookie is the only state the browser keeps, so this is the one place that
// decides what "logged in" means on the wire.
func (s *Server) startSession(w http.ResponseWriter, username string) (webSession, error) {
	sess, err := s.Sessions.Start(username)
	if err != nil {
		return webSession{}, err
	}
	token, err := newToken()
	if err != nil {
		s.Sessions.Revoke(sess.Token)
		return webSession{}, err
	}
	// The CSRF token expires with the session it belongs to, so a session cannot
	// outlive its own token and leave a logged-in browser unable to post.
	s.csrf.put(sess.Token, token, sess.ExpiresAt)

	http.SetCookie(w, s.sessionCookie(sess.Token, sess.ExpiresAt))
	return webSession{Username: sess.Username, Token: sess.Token, ExpiresAt: sess.ExpiresAt, CSRFToken: token}, nil
}

// sessionCookie builds the Set-Cookie value.
//
// HttpOnly because a script that can read this cookie is a script running as the
// admin, at which point CSRF is the least of the problems. SameSite=Strict because
// there is no legitimate cross-site navigation to this UI — it is reached by typing
// an address, not by following a link from somewhere else — and Strict is the only
// value that also protects the GETs.
//
// Secure is dropped when TLS is off, and only then: setting Secure on a plain-HTTP
// listener means the browser silently discards the cookie and the operator sees an
// infinite redirect to /login with no error anywhere. That is the one case where
// correctness forces us to accept what the attribute would otherwise prevent, and
// the startup warning about TLS being off is what covers it.
func (s *Server) sessionCookie(token string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.TLSMode != TLSOff,
		SameSite: http.SameSiteStrictMode,
	}
}

// clearSession expires the cookie without revoking other sessions.
//
// The Max-Age is negative rather than an Expires in the past because that is what
// the RFC prescribes and what every browser implements; an Expires header with a
// stale clock on the client can fail to delete anything.
func (s *Server) clearSession(w http.ResponseWriter) {
	c := s.sessionCookie("", time.Time{})
	c.MaxAge = -1
	c.Expires = time.Unix(0, 0).UTC()
	http.SetCookie(w, c)
}

// csrfStore holds the per-session CSRF token.
//
// It lives here rather than inside auth.Session because CSRF is a property of this
// transport, not of the credential: a CLI that authenticates the same account has
// no token and needs none. Putting it in the session type would force every
// consumer of internal/auth to carry a field it cannot use.
//
// The key is the session token, so a session revoked anywhere loses its token on
// the next lookup rather than keeping a live one behind.
type csrfStore struct {
	mu     sync.Mutex
	tokens map[string]csrfEntry
	now    func() time.Time
}

type csrfEntry struct {
	token     string
	expiresAt time.Time
}

func newCSRFStore() *csrfStore {
	return &csrfStore{tokens: map[string]csrfEntry{}, now: time.Now}
}

func (c *csrfStore) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (c *csrfStore) put(sessionToken, csrfToken string, expiresAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tokens == nil {
		c.tokens = map[string]csrfEntry{}
	}
	now := c.clock()
	for k, e := range c.tokens {
		if !now.Before(e.expiresAt) {
			delete(c.tokens, k)
		}
	}
	c.tokens[sessionToken] = csrfEntry{token: csrfToken, expiresAt: expiresAt}
}

func (c *csrfStore) lookup(sessionToken string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.tokens[sessionToken]
	if !ok {
		return "", false
	}
	if !c.clock().Before(e.expiresAt) {
		delete(c.tokens, sessionToken)
		return "", false
	}
	return e.token, true
}

func (c *csrfStore) drop(sessionToken string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.tokens, sessionToken)
}

// newToken is 32 random bytes, hex.
func newToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", errors.New("no entropy for a session token")
	}
	return hex.EncodeToString(b[:]), nil
}
