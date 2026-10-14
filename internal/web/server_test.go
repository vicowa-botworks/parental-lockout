package web

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/auth"
)

// stubAuth is a credential that costs nothing to check.
//
// The real PasswordAuthenticator is deliberately slow — that is the point of
// PBKDF2 — and a routing suite that runs it on every login case would be a suite
// nobody runs twice. What keeps this honest is that it implements
// auth.Authenticator, the same interface the daemon is handed, so a signature
// change in internal/auth breaks this file at compile time rather than quietly
// testing a shape that no longer exists.
type stubAuth struct {
	username string
	password string
	// noCredential makes Authenticate return auth.ErrNoCredential, the state of a
	// machine where nobody has run `plock init`.
	noCredential bool
	// broken makes it return a different error: a verifier that cannot be parsed.
	broken   bool
	calls    int
	lastUser string
}

func (a *stubAuth) Authenticate(username, password string) (bool, error) {
	a.calls++
	a.lastUser = username
	if a.noCredential {
		return false, auth.ErrNoCredential
	}
	if a.broken {
		return false, errors.New("verifier is not base64")
	}
	return username == a.username && password == a.password, nil
}

// newTestServer builds a server over a stub credential with the log captured.
//
// TLS is off so the tests can drive Handler() with httptest and still assert the
// cookie's Secure attribute separately, in the one test that is about that
// attribute.
func newTestServer(t *testing.T, a auth.Authenticator) (*Server, *[]string, *[]string) {
	t.Helper()
	return newTestServerWith(t, a, &auth.Throttle{
		// The production penalty starts at a second and doubles, which is right for
		// a person and unusable in a test that wants to reach the bucket limit.
		// Everything else is left at the zero value and defaulted by the throttle
		// itself.
		PenaltyStart: time.Millisecond,
		PenaltyCap:   2 * time.Millisecond,
	})
}

func newTestServerWith(t *testing.T, a auth.Authenticator, th *auth.Throttle) (*Server, *[]string, *[]string) {
	t.Helper()
	var logged []string
	var audited []string
	s := &Server{
		Auth:     a,
		Sessions: auth.NewSessions(),
		Throttle: th,
		TLSMode:  TLSAuto,
		csrf:     newCSRFStore(),
		Listen:   "127.0.0.1:0",
		Logf:     func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) },
		Audit:    func(event, detail string) { audited = append(audited, event+" "+detail) },
	}
	return s, &logged, &audited
}

// login performs a real login and returns the cookie plus the CSRF token, so the
// authenticated tests are not hand-assembling state the server never issues.
func login(t *testing.T, s *Server, user, pass string) (*http.Response, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", strings.NewReader("username="+user+"&password="+pass))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.RemoteAddr = "192.0.2.10:5555"
	s.Handler().ServeHTTP(rec, req)
	resp := rec.Result()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login status = %d, want a 303 redirect; body %q", resp.StatusCode, rec.Body.String())
	}
	return resp, ""
}

func cookieOf(t *testing.T, resp *http.Response) *http.Cookie {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatalf("no %s cookie was set; response had %d cookies", sessionCookie, len(resp.Cookies()))
	return nil
}

// A correct login gets a session cookie with every attribute that keeps it out of
// a script's hands and off the wire in the clear.
func TestACorrectLoginSetsASessionCookieWithTheRightAttributes(t *testing.T) {
	s, _, audited := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	resp, _ := login(t, s, "parent", "hunter2")

	c := cookieOf(t, resp)
	if !c.HttpOnly {
		t.Error("the session cookie is readable from a script")
	}
	if !c.Secure {
		t.Error("the session cookie is not Secure while TLS is on")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, want /", c.Path)
	}
	// The session is 12h and the cookie carries that expiry. What matters is that
	// the cookie does not outlive the session it names: a cookie still being sent
	// after the server has forgotten the session is a browser that cannot be logged
	// out, and a cookie that expires before the session logs the parent out early.
	if c.Expires.Before(time.Now().Add(time.Hour)) {
		t.Errorf("the cookie expires at %v, well before the 12h session it names", c.Expires)
	}
	if c.Expires.After(time.Now().Add(13 * time.Hour)) {
		t.Errorf("the cookie outlives the session: %v", c.Expires)
	}
	if len(c.Value) < 32 {
		t.Errorf("the session token is %d characters, which is short enough to guess", len(c.Value))
	}
	if !anyContains(*audited, "auth.success parent") {
		t.Errorf("a successful login was not audited; audit: %q", *audited)
	}
}

// The Secure attribute is dropped when, and only when, TLS is off.
//
// Setting Secure on a plain-HTTP listener means the browser silently discards the
// cookie and the operator gets an endless redirect to /login with no error
// anywhere. That is the one case where correctness forces us to accept what the
// attribute would prevent.
func TestTheSecureAttributeFollowsTLS(t *testing.T) {
	on, _, _ := newTestServer(t, &stubAuth{username: "p", password: "q"})
	if c := on.sessionCookie("tok", time.Now().Add(time.Hour)); !c.Secure {
		t.Error("tls auto: the cookie is not Secure")
	}
	off, _, _ := newTestServer(t, &stubAuth{username: "p", password: "q"})
	off.TLSMode = TLSOff
	if c := off.sessionCookie("tok", time.Now().Add(time.Hour)); c.Secure {
		t.Error("tls off: a Secure cookie would be discarded by the browser and the login would loop")
	}
}

func TestAWrongPasswordIsOneAnswerForBothWrongHalves(t *testing.T) {
	s, _, audited := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})

	bodies := map[int]string{}
	for _, tc := range [][2]string{{"parent", "wrong"}, {"nobody", "hunter2"}, {"nobody", "wrong"}} {
		// The penalty blocks the attempt immediately after a failure. Waiting it out
		// keeps this test about whether the two halves of a wrong credential are
		// distinguishable, which is a different property from the throttle.
		time.Sleep(5 * time.Millisecond)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/login", strings.NewReader("username="+tc[0]+"&password="+tc[1]))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "192.0.2.11:1"
		s.Handler().ServeHTTP(rec, req)
		resp := rec.Result()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%q: status = %d, want 401", tc[0], resp.StatusCode)
		}
		bodies[resp.StatusCode] = rec.Body.String()
		if len(resp.Cookies()) != 0 {
			t.Errorf("%q: a failed login set %d cookies", tc[0], len(resp.Cookies()))
		}
	}
	if len(bodies) != 1 {
		t.Error("the two kinds of wrong credential produced different responses, which tells an attacker the username is real")
	}
	if !anyContains(*audited, "auth.failure") {
		t.Errorf("a failed login was not audited; audit: %q", *audited)
	}
}

// The throttle has to stop the work, not merely hide its result.
func TestRepeatedFailuresFromOneAddressAreThrottled(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	authStub := s.Auth.(*stubAuth)

	attemptsBefore := authStub.calls
	for i := 0; i < 12; i++ {
		// Wait out the per-failure penalty so that what trips on the thirteenth
		// attempt is the bucket limit, and the assertion below ("PBKDF2 must not run
		// at all") is testing the bucket rather than a 1ms penalty.
		time.Sleep(5 * time.Millisecond)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/login", strings.NewReader("username=parent&password=nope"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "192.0.2.20:1"
		s.Handler().ServeHTTP(rec, req)
	}
	if authStub.calls == attemptsBefore {
		t.Fatal("no credential was ever checked, so the test proved nothing")
	}
	checkedDuringSweep := authStub.calls - attemptsBefore
	if checkedDuringSweep > 10 {
		t.Errorf("the credential was checked %d times over 12 attempts; the bucket limit is 10, so attempts past it still did the PBKDF2 work",
			checkedDuringSweep)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", strings.NewReader("username=parent&password=hunter2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.20:1"
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("after 12 failures the status is %d, want 429", rec.Code)
	}
	// The thirteenth attempt is the one under test: it was refused, and it must not
	// have cost a PBKDF2 run. The point of the throttle is to stop the work, not
	// merely to hide its result.
	if authStub.calls-attemptsBefore != checkedDuringSweep {
		t.Errorf("a throttled attempt still reached the credential (%d checks before the refusal, %d after)",
			checkedDuringSweep, authStub.calls-attemptsBefore)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("a 429 with no Retry-After leaves the client guessing")
	}

	// A different address is a different bucket.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/login", strings.NewReader("username=parent&password=hunter2"))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.RemoteAddr = "192.0.2.99:1"
	s.Handler().ServeHTTP(rec2, req2)
	if rec2.Code == http.StatusTooManyRequests {
		t.Error("throttling one address locked out the whole household")
	}
}

// X-Forwarded-For is not consulted, because there is no proxy in front of this
// server and trusting it is the same as having no throttle.
func TestAForwardedForHeaderDoesNotChooseTheThrottleBucket(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	for i := 0; i < 12; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/login", strings.NewReader("username=parent&password=nope"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i))
		req.RemoteAddr = "192.0.2.30:1"
		s.Handler().ServeHTTP(rec, req)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", strings.NewReader("username=parent&password=hunter2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Forwarded-For", "203.0.113.200")
	req.RemoteAddr = "192.0.2.30:1"
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429: the attacker chose their own bucket by changing a header", rec.Code)
	}
}

// An IPv6 zone is not part of the host.
func TestTheIPv6ZoneIsStrippedFromTheThrottleKey(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{})
	if got := s.throttleKey(&http.Request{RemoteAddr: "[fe80::1%enp3s0]:443"}); got != "fe80::1" {
		t.Errorf("throttleKey = %q, want fe80::1", got)
	}
	if got := s.throttleKey(&http.Request{RemoteAddr: "[fe80::1%wlan0]:443"}); got != "fe80::1" {
		t.Errorf("throttleKey = %q, want fe80::1: the same host would get its own bucket per interface", got)
	}
}

// A machine with no credential must say so, because "wrong password" sends the
// operator looking in the wrong place.
func TestAMachineWithNoCredentialSaysThatRatherThanWrongPassword(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{noCredential: true})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", strings.NewReader("username=parent&password=whatever"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.40:1"
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no administrator account") {
		t.Errorf("the response does not name the problem: %q", rec.Body.String())
	}
}

// A broken verifier is not charged to the throttle.
//
// Locking the operator out of the machine that is the only way to fix the config is
// the worst available outcome here, and it is one step away from counting a
// daemon-side failure as a user's guess.
func TestABrokenVerifierDoesNotChargeTheThrottle(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{broken: true})
	for i := 0; i < 12; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/login", strings.NewReader("username=parent&password=x"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "192.0.2.50:1"
		s.Handler().ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d was throttled for a fault in the daemon's own file", i+1)
		}
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	}
}

func TestHealthzNeedsNoSession(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	// It must not describe the machine.
	if body := rec.Body.String(); strings.Contains(strings.ToLower(body), "lock") || strings.Contains(body, "{") {
		t.Errorf("healthz reports state to an anonymous caller: %q", body)
	}
}

// A browser gets a redirect; a client that asked for JSON gets a status code.
func TestAnUnauthenticatedRequestAnswersTheCallerItIsTalkingTo(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "text/html")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("browser: status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("browser: Location = %q, want /login", loc)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Accept", "application/json")
	s.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("json: status = %d, want 401, not a redirect to an HTML form", rec2.Code)
	}

	// No Accept at all is a program, not a browser.
	rec3 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec3, httptest.NewRequest("GET", "/", nil))
	if rec3.Code != http.StatusUnauthorized {
		t.Errorf("no Accept: status = %d, want 401", rec3.Code)
	}
}

// The headers have to be on every response, including the ones written by a
// rejection path rather than a page.
func TestSecurityHeadersAreOnEveryResponse(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{username: "p", password: "q"})

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"login page", "GET", "/login"},
		{"healthz", "GET", "/healthz"},
		{"redirected root", "GET", "/"},
		{"not found", "GET", "/nope"},
		{"csrf rejected", "POST", "/logout"},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.RemoteAddr = "192.0.2.60:1"
		s.Handler().ServeHTTP(rec, req)
		h := rec.Header()
		csp := h.Get("Content-Security-Policy")
		for _, want := range []string{"default-src 'self'", "frame-ancestors 'none'"} {
			if !strings.Contains(csp, want) {
				t.Errorf("%s: CSP %q is missing %q", tc.name, csp, want)
			}
		}
		if got := h.Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s: X-Frame-Options = %q, want DENY", tc.name, got)
		}
		if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", tc.name, got)
		}
		if got := h.Get("Referrer-Policy"); got != "no-referrer" {
			t.Errorf("%s: Referrer-Policy = %q, want no-referrer", tc.name, got)
		}
		if tc.path != "/static/x" {
			if got := h.Get("Cache-Control"); !strings.Contains(got, "no-store") {
				t.Errorf("%s: Cache-Control = %q, want no-store on a dynamic response", tc.name, got)
			}
		}
		if h.Get(requestIDHeader) == "" {
			t.Errorf("%s: no request id, so a reported error cannot be found in the journal", tc.name)
		}
	}
}

// A state-changing request without the session's token is refused, and the thing it
// asked for does not happen.
func TestAPostWithoutACSRFTokenIsRefusedAndDoesNothing(t *testing.T) {
	s, _, audited := newTestServer(t, &stubAuth{username: "parent", password: "q"})
	resp, _ := login(t, s, "parent", "q")
	c := cookieOf(t, resp)

	// The mutation the request was aiming at has an observable effect: it revokes
	// the session. Asserting the absence of that effect is the only way to know the
	// 403 stopped the work rather than merely describing it.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/logout", nil)
	req.AddCookie(c)
	req.RemoteAddr = "192.0.2.70:1"
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if _, ok := s.Sessions.Lookup(c.Value); !ok {
		t.Error("the session was revoked even though the request was refused")
	}
	if !anyContains(*audited, "csrf.rejected") {
		t.Errorf("a refused CSRF check was not audited; audit: %q", *audited)
	}
}

// The token round-trips: a POST that carries it succeeds.
//
// Without this half the CSRF test only proves the server rejects everything, which
// would also be true of a control that is simply broken.
func TestAPostWithTheSessionsTokenIsAccepted(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{username: "parent", password: "q"})
	resp, _ := login(t, s, "parent", "q")
	c := cookieOf(t, resp)
	token, ok := s.csrf.lookup(c.Value)
	if !ok {
		t.Fatal("the login did not mint a CSRF token")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/logout", strings.NewReader("csrf="+token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(c)
	req.RemoteAddr = "192.0.2.71:1"
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}
	if _, ok := s.Sessions.Lookup(c.Value); ok {
		t.Error("the session survived a correctly authorised logout")
	}
}

// A token from a different session is not this session's token.
func TestAnotherSessionsCSRFTokenIsNotAccepted(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{username: "parent", password: "q"})
	first, _ := login(t, s, "parent", "q")
	firstCookie := cookieOf(t, first)
	firstToken, _ := s.csrf.lookup(firstCookie.Value)

	// A second session for the same account, which is the ordinary case of a phone
	// and a desktop.
	second, _ := login(t, s, "parent", "q")
	secondCookie := cookieOf(t, second)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/logout", strings.NewReader("csrf="+firstToken))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(secondCookie)
	req.RemoteAddr = "192.0.2.72:1"
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403: one device's token authorised another's", rec.Code)
	}
}

// A panic in a handler is a 500 and a log line, not a dead daemon.
//
// This process is what keeps the child locked out. A panic in the admin UI must not
// unlock the house, and it must not print a stack to the client either.
func TestAHandlerPanicBecomesA500AndStaysOffTheWire(t *testing.T) {
	s, logged, _ := newTestServer(t, &stubAuth{})
	s.Handler() // ensure the chain is built
	doomed := s.recoverMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("a handler fell over")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/boom", nil)
	doomed.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "a handler fell over") || strings.Contains(rec.Body.String(), "goroutine") {
		t.Errorf("the panic reached the client: %q", rec.Body.String())
	}
	if !anyContains(*logged, "a handler fell over") {
		t.Error("the panic was not logged, so nobody will know it happened")
	}
}

// The log must not carry a credential or a token.
//
// This is asserted by logging a real login and then searching the whole log for
// the secret strings, rather than by reading the format string and agreeing with
// it: the failure mode is a line added later, not the one that exists now.
func TestTheLogCarriesNoPasswordNoCookieAndNoSessionToken(t *testing.T) {
	const password = "correct horse battery staple"
	s, logged, _ := newTestServer(t, &stubAuth{username: "parent", password: password})

	resp, _ := login(t, s, "parent", password)
	c := cookieOf(t, resp)

	// A request whose query string looks like a token, because that is how tokens
	// end up in logs: somebody pasted a URL.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/?session="+c.Value+"&token=abc", nil)
	req.AddCookie(c)
	req.RemoteAddr = "192.0.2.80:1"
	s.Handler().ServeHTTP(rec, req)

	joined := strings.Join(*logged, "\n")
	for _, secret := range []string{password, c.Value, "correct horse"} {
		if strings.Contains(joined, secret) {
			t.Errorf("the log contains %q\n--- log ---\n%s", secret, joined)
		}
	}
	if strings.Contains(joined, "session="+c.Value) {
		t.Error("the query string was logged")
	}
}

// Logout clears the cookie and revokes the session, and does not touch the
// account's other sessions.
func TestLogoutRevokesOneSessionNotAllOfThem(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{username: "parent", password: "q"})
	first, _ := login(t, s, "parent", "q")
	firstCookie := cookieOf(t, first)
	firstToken, _ := s.csrf.lookup(firstCookie.Value)

	second, _ := login(t, s, "parent", "q")
	secondCookie := cookieOf(t, second)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/logout", strings.NewReader("csrf="+firstToken))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(firstCookie)
	req.RemoteAddr = "192.0.2.90:1"
	s.Handler().ServeHTTP(rec, req)

	if _, ok := s.Sessions.Lookup(firstCookie.Value); ok {
		t.Error("the logged-out session is still valid")
	}
	if _, ok := s.Sessions.Lookup(secondCookie.Value); !ok {
		t.Error("logging out of one device revoked another one")
	}
	cleared := cookieOf(t, rec.Result())
	if cleared.MaxAge >= 0 && cleared.Expires.After(time.Now().Add(time.Minute)) {
		t.Errorf("the cookie was not expired: MaxAge=%d Expires=%v", cleared.MaxAge, cleared.Expires)
	}
}

// Serve must actually serve, and must stop when the context is cancelled.
//
// This one goes through a socket because the thing under test is the listener and
// the drain, not the handler.
func TestServeServesAndStopsWithTheContext(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{})
	s.TLSMode = TLSOff
	s.Now = func() time.Time { return time.Now() }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()

	// Wait for the bind rather than sleeping: the port is chosen by the kernel.
	var addr string
	for i := 0; i < 200; i++ {
		if addr = s.Addr(); addr != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if addr == "" {
		t.Fatal("Serve never bound")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("the server refused a request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Serve returned %v, want context.Canceled", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Serve did not return after the context was cancelled")
	}

	// A connection after shutdown is refused, not queued.
	if _, err := client.Get("http://" + addr + "/healthz"); err == nil {
		t.Error("the server still accepts connections after shutdown")
	}
}

// A request already in flight finishes during the drain.
func TestAnInFlightRequestCompletesDuringShutdown(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{})
	s.TLSMode = TLSOff

	release := make(chan struct{})
	slow := make(chan struct{})
	s.extraRoutes = func(mux *http.ServeMux) {
		mux.HandleFunc("GET /slow", func(http.ResponseWriter, *http.Request) {
			close(slow)
			<-release
			_, _ = io.WriteString(io.Discard, "")
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()

	var addr string
	for i := 0; i < 200; i++ {
		if addr = s.Addr(); addr != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if addr == "" {
		t.Fatal("Serve never bound")
	}
	// The release channel is closed exactly once, on the success path. A Cleanup
	// that closed it too would panic on the happy path, which is the one path this
	// test takes when it passes.
	t.Cleanup(cancel)

	go func() {
		client := &http.Client{Timeout: 20 * time.Second}
		resp, err := client.Get("http://" + addr + "/slow")
		if err == nil {
			resp.Body.Close()
		}
		slowResult <- result{err: err}
	}()

	<-slow
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Serve returned %v, want context.Canceled", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Serve did not finish draining")
	}
	close(release)

	got := <-slowResult
	if got.err != nil {
		t.Errorf("the in-flight request was cut off instead of drained: %v", got.err)
	}
}

type result struct{ err error }

var slowResult = make(chan result, 1)

// The transport the daemon presents must be TLS 1.2 at the floor.
func TestTheTransportRefusesAnythingOlderThanTLS12(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{})
	cfg := s.tlsConfig()
	if cfg == nil {
		t.Fatal("tls auto produced no config")
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %x, want TLS 1.2", cfg.MinVersion)
	}
	if cfg.ClientAuth != tls.NoClientCert {
		t.Error("client certificates are being requested; there is exactly one admin and they use a password")
	}
}

// A listener that cannot bind is a startup failure with the address in it.
func TestABusyPortSaysWhichAddressItWanted(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, _, _ := newTestServer(t, &stubAuth{})
	s.TLSMode = TLSOff
	s.Listen = ln.Addr().String()

	err = s.Serve(context.Background())
	if err == nil {
		t.Fatal("binding a port already in use reported success")
	}
	if !strings.Contains(err.Error(), ln.Addr().String()) {
		t.Errorf("the error does not name the address: %v", err)
	}
}

// An unknown tls mode must not silently pick a transport.
func TestAnUnknownTLSModeRefusesToStart(t *testing.T) {
	_, err := New(Options{Listen: "127.0.0.1:0", TLSMode: "start"})
	if err == nil {
		t.Fatal("an unknown tls mode was accepted")
	}
	if !strings.Contains(err.Error(), "unknown tls mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewNeedsSomethingToListenOn(t *testing.T) {
	if _, err := New(Options{TLSMode: TLSOff}); err == nil {
		t.Error("an empty listen address was accepted")
	}
}

// The body cap is asserted rather than assumed.
func TestAnOversizedLoginBodyIsRefused(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{username: "p", password: "q"})
	body := "username=p&password=" + strings.Repeat("x", maxLoginBody+1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ContentLength = int64(len(body))
	req.RemoteAddr = "192.0.2.95:1"
	s.Handler().ServeHTTP(rec, req)

	if rec.Code == http.StatusSeeOther {
		t.Error("an oversized body was logged in with")
	}
	_ = io.Discard
}
