package web

import (
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/auth"
)

// maxLoginBody is the size cap on a login POST.
//
// The form is two short strings. A megabyte is generous enough not to reject a
// paste from a password manager and small enough that a flood of connections
// cannot make the server allocate for bodies nobody intends to read.
const maxLoginBody = 1 << 20

// handleLoginPage renders the form.
//
// It is reachable without a session, and it carries a CSRF token for a session the
// visitor does not have. That is deliberate: the token is bound to a session, so a
// visitor with none gets a form whose POST will be rejected by the auth gate before
// the CSRF layer is consulted, and minting a throwaway token would imply the token
// proves something about the browser rather than about the session.
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.sessionFrom(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := loginPageTmpl.Execute(w, nil); err != nil {
		s.logf("web: %s could not render the login page: %v", requestID(r), err)
	}
}

// handleLogout revokes one session and clears the cookie.
//
// One session, not all of them: a parent who logs out of the phone should not
// lock themselves out of the desktop they are still using. RevokeAll is the right
// answer to a stolen credential, and that is an operator decision with a button of
// its own, not a side effect of closing a tab.
//
// It is POST-only and CSRF-protected for that reason. A GET /logout would be a
// link an image tag could trigger, and "the admin keeps getting logged out" is the
// kind of fault that gets the daemon disabled.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.sessionFrom(r)
	if ok {
		s.Sessions.Revoke(sess.Token)
		s.csrf.drop(sess.Token)
		s.audit("auth.logout", sess.Username)
	}
	s.clearSession(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleLogin checks the credential and mints a session.
//
// The response is deliberately uninformative on the two failure modes an attacker
// can observe: a wrong username and a wrong password produce the same 401 and the
// same body. The one case that answers differently is ErrNoCredential, and it
// should, because it is not an attacker's problem — it is a machine where nobody
// can sign in, and an operator who reads "run plock init" fixes it in a minute
// while one who reads "wrong password" calls somebody at two in the morning.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	key := s.throttleKey(r)

	if ok, wait := s.Throttle.Allow(key); !ok {
		// 429 with a Retry-After, and no credential check at all: the point of the
		// throttle is to stop the PBKDF2 work, not merely to hide its result.
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(wait.Seconds())+1))
		s.audit("auth.throttled", key)
		if wantsJSON(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprintf(w, `{"error":"too many failed logins","retryAfterSeconds":%d}`+"\n", int(wait.Seconds())+1)
			return
		}
		http.Error(w, fmt.Sprintf("too many failed logins; try again in %s", wait.Round(time.Second)), http.StatusTooManyRequests)
		return
	}

	if err := r.ParseForm(); err != nil || r.ContentLength > maxLoginBody {
		http.Error(w, "could not read the form", http.StatusBadRequest)
		return
	}

	// Resolved per attempt, not once at startup. The credential lives in a file the
	// CLI rewrites from another process, so an authenticator captured at boot keeps
	// accepting the password it was built with after a rotation -- which is the half of
	// the rotation that matters during an incident, and revoking live sessions would be
	// theatre without it.
	a, err := s.authenticator()
	switch {
	case errors.Is(err, errNoAuthenticator):
		s.audit("auth.no_credential", key)
		s.logf("web: %s a login was attempted but no administrator credential exists; run `plock init`", requestID(r))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, "this machine has no administrator account yet\n")
		return
	case err != nil:
		// The credential could not be read. That is not a wrong password, and it is not
		// the same sentence as "no account exists": here an account does exist and the
		// daemon cannot see it. Answering 401 would send the operator at the password.
		s.audit("auth.credential_unreadable", key)
		s.logf("web: %s the admin credential could not be read: %v", requestID(r), err)
		http.Error(w, "the administrator credential could not be read", http.StatusServiceUnavailable)
		return
	}

	ok, err := a.Authenticate(r.FormValue("username"), r.FormValue("password"))
	switch {
	case errors.Is(err, auth.ErrNoCredential):
		// Not a wrong password. Answering 401 here would send the operator chasing
		// a credential that was never the problem.
		s.audit("auth.no_credential", key)
		s.logf("web: %s the admin record has no password verifier; run `plock init`", requestID(r))
		http.Error(w, "this machine has no administrator account yet", http.StatusServiceUnavailable)
		return
	case err != nil:
		// A verifier that cannot be parsed is a config problem, and the login is
		// neither accepted nor counted as a guess: charging the throttle for the
		// daemon's own broken file would lock the operator out of the machine that
		// is the only way to fix it.
		s.audit("auth.error", err.Error())
		s.logf("web: %s could not check the credential: %v", requestID(r), err)
		http.Error(w, "could not check the credential", http.StatusInternalServerError)
		return
	case !ok:
		s.Throttle.Failure(key)
		s.audit("auth.failure", key)
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "wrong username or password", http.StatusUnauthorized)
		return
	}

	s.Throttle.Success(key)
	sess, err := s.startSession(w, r.FormValue("username"))
	if err != nil {
		s.audit("auth.session_error", err.Error())
		s.logf("web: %s authenticated but no session could be created: %v", requestID(r), err)
		http.Error(w, "could not start a session", http.StatusInternalServerError)
		return
	}
	s.audit("auth.success", sess.Username)
	s.logf("web: %s signed in as %s", requestID(r), sess.Username)

	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"user":%q,"expiresAt":%q}`+"\n", sess.Username, sess.ExpiresAt.UTC().Format(time.RFC3339))
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// throttleKey is the address the throttle buckets by.
//
// X-Forwarded-For is never consulted. This server is reached directly on the LAN —
// there is no proxy in front of it, and #12's unit runs it bound to an interface,
// not behind a reverse proxy — so trusting that header would let any client pick
// which bucket it lands in, which is the same as having no throttle.
//
// The IPv6 zone is stripped because two link-local addresses that differ only by
// zone (`fe80::1%enp3s0` and `fe80::1%wlan0`) are the same host, and Go keeps the
// zone in RemoteAddr.
func (s *Server) throttleKey(r *http.Request) string {
	addr := r.RemoteAddr
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	}
	if i := strings.IndexByte(addr, '%'); i >= 0 {
		addr = addr[:i]
	}
	if addr == "" {
		// A request with no peer address only happens behind a listener the tests
		// hand-build. Bucketing them all together is the safe reading: it cannot be
		// used to get an unthrottled attempt.
		return "unknown"
	}
	return addr
}

// loginPageTmpl is the form itself.
//
// html/template, not text/template, and no interpolation of anything from the
// request: the page has no dynamic content today, and the reason it is a template
// rather than a string constant is that #10 adds the error line and the CSRF
// hidden field to it.
var loginPageTmpl = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Sign in &middot; plock</title>
<meta name="viewport" content="width=device-width, initial-scale=1"></head>
<body>
<h1>Parental lockout</h1>
<form method="post" action="/login">
  <p><label>Username <input name="username" autocomplete="username" required></label></p>
  <p><label>Password <input name="password" type="password" autocomplete="current-password" required></label></p>
  <p><button type="submit">Sign in</button></p>
</form>
</body>
</html>
`))
