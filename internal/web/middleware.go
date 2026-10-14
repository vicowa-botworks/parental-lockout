package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// The middleware chain, outermost first:
//
//	recover  ->  request log  ->  security headers  ->  CSRF  ->  router
//
// The order is a decision, not a default.
//
// Recovery is outermost so a panic in *any* of the others still produces a 500
// rather than a closed connection, and so the panic is logged with the request ID
// that the caller was given.
//
// The log is next so a request rejected by headers or CSRF is still counted. A
// rejected request is the thing you want in the log.
//
// Headers come before CSRF so that a 403 carries them too — a bare 403 written by
// the CSRF layer would otherwise be the one response on the server without
// X-Frame-Options on it.
//
// CSRF is innermost, wrapping the router, because it needs to know the request is
// addressed to a real route and needs the session the auth gate resolved.

// requestIDKey is the context key carrying this request's ID.
type contextKey int

const requestIDKey contextKey = iota

// requestIDHeader is echoed to the client and written to the log.
//
// It is the whole link between "the page said an error happened" and the line in
// the journal, on a machine where the parent has no way to grep a log in real
// time. It has to be visible in the UI for that to work, which is #10's job; the
// server's job is to mint it and never trust a client-supplied one.
const requestIDHeader = "X-Request-Id"

// newRequestID is 8 hex bytes. Long enough to be unique across a day of a home
// network's traffic, short enough to read out over the phone.
func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// A machine with no entropy is not a machine this daemon can run on — the
		// session tokens come from the same source. Fall back to a timestamp so the
		// ID still correlates the log line with the response instead of being empty.
		return fmt.Sprintf("t%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// requestID returns this request's ID, or "" outside the chain.
func requestID(r *http.Request) string {
	v, _ := r.Context().Value(requestIDKey).(string)
	return v
}

// requestLog stamps the request with its ID, echoes that ID in a header, and
// writes one line per request on the way out.
func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		start := s.now()

		lf := &logFields{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lf, r.WithContext(ctx))

		// The path is logged; the query string is not.
		//
		// A query string is where a session token ends up the moment somebody
		// pastes a URL, and a log line is the one place a secret can outlive the
		// session it belongs to — journald is readable by the wheel group, and a
		// token in a log is a token that closing the browser no longer revokes.
		//
		// The same reasoning covers headers: nothing here logs a header value at
		// all, so neither the Cookie nor the password field can reach the log
		// through a debug line added later.
		s.logf("web: %s %s %s %d %dB %s rid=%s",
			r.RemoteAddr, r.Method, r.URL.Path, lf.status, lf.bytes,
			s.now().Sub(start).Round(time.Microsecond), id)
	})
}

// logFields is the response writer the log layer wraps the real one in, so the
// status and byte count it reports are the ones the client actually got.
type logFields struct {
	http.ResponseWriter
	status int
	bytes  int
	// wroteHeader is set by the first Write or WriteHeader. A handler that writes
	// without calling WriteHeader gets an implicit 200 from the underlying writer,
	// and a log line that says 0 makes the log useless for finding the 500s.
	wroteHeader bool
}

func (l *logFields) WriteHeader(code int) {
	if !l.wroteHeader {
		l.status = code
		l.wroteHeader = true
	}
	l.ResponseWriter.WriteHeader(code)
}

func (l *logFields) Write(b []byte) (int, error) {
	if !l.wroteHeader {
		l.status = http.StatusOK
		l.wroteHeader = true
	}
	n, err := l.ResponseWriter.Write(b)
	l.bytes += n
	return n, err
}

// recoverMiddleware turns a panic into a 500 and a log line.
//
// The stack goes to the log and never to the client. A panic page from net/http
// names the handler and the file, which on a box reachable from the LAN is a map
// of the binary to anybody who can find a request that trips it.
//
// It recovers and continues rather than crashing: this process is what keeps the
// child locked out, and a panic in the admin UI should not unlock the house.
func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			lf, _ := w.(*logFields)
			if lf != nil && lf.wroteHeader {
				// The handler already started a response; there is nothing left to
				// send. Saying so is better than pretending the 500 went out.
				s.logf("web: %s panic after the response started: %v\n%s", requestID(r), rec, debug.Stack())
				return
			}
			s.logf("web: %s panic: %v\n%s", requestID(r), rec, debug.Stack())
			http.Error(w, "internal error", http.StatusInternalServerError)
			if lf != nil {
				lf.status = http.StatusInternalServerError
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// securityHeaders sets the response headers, and strips Cache-Control from
// anything dynamic.
//
// `no-store` on everything except the hashed assets is deliberate: this UI shows
// who is locked out and until when, and a shared machine's cache keeping that
// readable after logout is a small leak of exactly the wrong thing.
//
// frame-ancestors 'none' and X-Frame-Options: DENY are both set because the CSP
// directive is what modern browsers obey and the header is what an older one
// obeys; a login form inside an iframe on a hostile page is the classic way a
// password gets typed somewhere it should not be.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		// The hashed assets set their own long-lived Cache-Control; they are
		// content-addressed, so caching them cannot leak anything.
		if !strings.HasPrefix(r.URL.Path, "/static/") {
			h.Set("Cache-Control", "no-store")
			h.Set("Pragma", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}

// requireAuth is the gate.
//
// A browser gets a 302 to /login because that is what makes the UI work when the
// session expires while a page is open. A client that asked for JSON gets a 401
// instead, because following a redirect to a login form and then parsing the HTML
// as an API response is worse than a status code that says what happened.
//
// The distinction is made on Accept, not on a header the client has to be told
// about: `Accept: application/json` is already what any HTTP client sends by
// default when it is not a browser, and a browser sends `text/html`.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.sessionFrom(r); !ok {
			s.wantsLogin(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// wantsLogin answers an unauthenticated request without saying why.
//
// There is no "your session expired" versus "you were never logged in" distinction
// on a protected page, because there is nothing here worth probing for.
func (s *Server) wantsLogin(w http.ResponseWriter, r *http.Request) {
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprintf(w, `{"error":"not authenticated","requestId":%q}`+"\n", requestID(r))
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func wantsJSON(r *http.Request) bool {
	// Accept is a preference list; a client that sends nothing means "anything",
	// which for a programmatic caller is the common case, so an empty Accept is
	// treated as JSON and only an explicit text/html gets the redirect.
	accept := r.Header.Get("Accept")
	if accept == "" {
		return true
	}
	return strings.Contains(accept, "application/json")
}

// csrfMiddleware rejects a state-changing request that does not carry the session's
// token.
//
// SameSite=Strict on the cookie already stops the classic cross-site POST, and
// this is not a replacement for it — it is the second control, for the cases
// Strict does not cover: a request that *is* same-site because it came from
// something already running on the machine, and any future route that ends up on
// a response where the cookie attribute is not doing the work.
//
// GET, HEAD and OPTIONS are exempt because they must not change anything anyway;
// requiring a token on them would break the browser's own prefetches for no gain.
func (s *Server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		// The login POST is exempt, and it is worth being explicit about why
		// because it is the one state-changing route with no token on it.
		//
		// A CSRF token is bound to a session, and the person filling in a login
		// form has no session yet — there is nothing to bind it to. Minting a
		// throwaway token per page view would not fix that either: an attacker can
		// load the login page too, so the token would prove nothing about where the
		// POST came from. What actually protects a login is the throttle, and that
		// is applied in the handler before any credential is touched.
		//
		// The consequence is worth stating plainly: a cross-site form that posts a
		// guessed password is not stopped here. It is stopped by SameSite=Strict
		// (the cookie the response sets is not sent on the attacker's navigation)
		// and by the throttle. This layer's job starts where a session exists.
		if isPublic(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		sess, ok := s.sessionFrom(r)
		if !ok {
			// Not authenticated: the route's own gate answers, with the status that
			// matches the caller. Reporting 403 here would tell an attacker they had
			// found a route worth aiming at.
			s.wantsLogin(w, r)
			return
		}

		got := r.FormValue(csrfField)
		if got == "" {
			got = r.Header.Get(csrfHeader)
		}
		if !tokenEqual(got, sess.CSRFToken) {
			s.audit("csrf.rejected", r.URL.Path)
			s.logf("web: %s %s rejected: the CSRF token is missing or wrong", r.Method, r.URL.Path)
			http.Error(w, "csrf check failed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isPublic is the set of paths reachable without a session.
//
// It is one function rather than a decision at each layer because the auth gate and
// the CSRF layer must agree exactly. Two lists that drift produce either a public
// route that demands a session or, far worse, a protected route that stops
// demanding a token.
func isPublic(path string) bool {
	switch path {
	case "/healthz", "/login":
		return true
	}
	// Assets carry no authority: a page that cannot load its stylesheet is a page
	// the parent will not trust, and there is nothing in a stylesheet to steal.
	return strings.HasPrefix(path, "/static/")
}

const (
	csrfField  = "csrf"
	csrfHeader = "X-CSRF-Token"
)

// tokenEqual compares two tokens without a timing signal.
//
// The length check comes first and is not a leak: the expected length is fixed and
// public, and comparing lengths does not depend on any secret.
func tokenEqual(got, want string) bool {
	if got == "" || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
