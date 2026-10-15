package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// The shell's sign-out form is the only interactive control on every page of the admin
// UI, and it shipped unable to work: it posted to /logout with no `csrf` field, and
// csrfMiddleware demands the session token on that route. Through the real chain with a
// valid session, the rendered form answered 403 "csrf check failed" and left the session
// live -- Sign out did nothing, everywhere.
//
// No test could have caught it. Every logout test built its own `csrf=<token>` body, and
// the one helper that could have handed a test a genuine token returned "" while being
// documented as returning one. Nothing in the suite ever asked what the browser was
// actually told to send.
//
// So this test renders the page, reads the form out of the HTML rather than assuming its
// shape, and posts exactly that. If the template drops the field, the test fails; if the
// field is renamed, it fails; if the token it carries stops matching the session's, it
// fails.
func TestTheSignOutFormAsRenderedActuallySignsOut(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)

	// One session for the whole round trip. authGet logs in again, which would render
	// the page under a second session and make the token comparison below meaningless.
	resp, wantToken := login(t, s, "parent", "hunter2")
	cookies := resp.Cookies()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "text/html")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200; body %q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	action, fields := parseForm(t, body, "/logout")
	if action != "/logout" {
		t.Errorf("form action = %q, want /logout", action)
	}
	got, ok := fields["csrf"]
	if !ok {
		t.Fatalf("the rendered sign-out form carries no csrf field; the middleware requires one, so "+
			"Sign out 403s on every page. Rendered form: %s", formFragment(body, "/logout"))
	}
	if got != wantToken {
		t.Errorf("the form carries csrf %q, want the session's own token %q; a token that is not "+
			"this session's is rejected the same way a forged one is", got, wantToken)
	}

	// Post exactly what the browser would post.
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}
	postReq := httptest.NewRequest("POST", "/logout", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.Header.Set("Accept", "text/html")
	postReq.RemoteAddr = "192.0.2.10:5555"
	for _, c := range cookies {
		postReq.AddCookie(c)
	}
	post := httptest.NewRecorder()
	s.Handler().ServeHTTP(post, postReq)

	if post.Code == http.StatusForbidden {
		t.Fatal("the rendered sign-out form was rejected by the CSRF check; the button does nothing " +
			"on every page of the admin UI")
	}
	if post.Code != http.StatusSeeOther {
		t.Errorf("POST /logout as rendered = %d, want 303", post.Code)
	}
	// The session has to be gone, or "signed out" is a redirect and nothing else.
	if _, ok := s.Sessions.Lookup(sessionTokenOf(cookies)); ok {
		t.Error("the sign-out round trip left the session live")
	}
}

// The other hook static/app.js needs. Its csrfToken() reads meta[name="csrf-token"], and
// a shell that never emits it makes every fetch the script issues carry an empty token --
// a second integration point that existed only in the script.
func TestTheShellEmitsTheCSRFMetaTagAppJSReads(t *testing.T) {
	rec := authGet(t, pageServer(t, pageConfig(), nil, nil, nil, nil), "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `name="csrf-token"`) {
		t.Fatal(`no <meta name="csrf-token"> in the shell; static/app.js reads that element and ` +
			`finds nothing, so every request it makes is rejected`)
	}
	// The CSP forbids inline script, so the token must arrive as data, not as code.
	if strings.Contains(body, "<script>") {
		t.Error("the shell emits an inline <script>, which its own CSP forbids")
	}
}

var (
	formRe  = regexp.MustCompile(`(?s)<form\b[^>]*action="([^"]+)"[^>]*>(.*?)</form>`)
	inputRe = regexp.MustCompile(`(?s)<input\b[^>]*name="([^"]+)"[^>]*value="([^"]*)"[^>]*>`)
	inputRb = regexp.MustCompile(`(?s)<input\b[^>]*value="([^"]*)"[^>]*name="([^"]+)"[^>]*>`)
)

// parseForm reads the form that posts to action out of rendered HTML. Reading it rather
// than reconstructing it is the point: a test that builds its own body tests the body it
// built.
func parseForm(t *testing.T, body, action string) (string, map[string]string) {
	t.Helper()
	for _, m := range formRe.FindAllStringSubmatch(body, -1) {
		if m[1] != action {
			continue
		}
		fields := map[string]string{}
		for _, in := range inputRe.FindAllStringSubmatch(m[2], -1) {
			fields[in[1]] = in[2]
		}
		// Attribute order is the template's business, not the test's.
		for _, in := range inputRb.FindAllStringSubmatch(m[2], -1) {
			if _, dup := fields[in[2]]; !dup {
				fields[in[2]] = in[1]
			}
		}
		return m[1], fields
	}
	t.Fatalf("no form posting to %q in the rendered page; body starts %s", action, body)
	return "", nil
}

func formFragment(body, action string) string {
	for _, m := range formRe.FindAllStringSubmatchIndex(body, -1) {
		// m[2]:m[3] is the captured action; check it before trusting the span.
		if body[m[2]:m[3]] != action {
			continue
		}
		return body[m[0]:m[1]]
	}
	return "(no such form)"
}

// sessionTokenOf pulls the session cookie's value, which is the key the store is indexed
// by.
func sessionTokenOf(cookies []*http.Cookie) string {
	for _, c := range cookies {
		if c.Name == sessionCookie {
			return c.Value
		}
	}
	return ""
}
