package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// The override control, read out of the rendered page rather than assumed.
//
// #66's review found that nothing on any page posted to the override routes: the handler,
// the CSRF gate and the audit line were all tested, and the buttons the ticket describes
// were never in the markup, so the only way to grant an override was to hand-craft the
// request. That is the same shape as #59's dead sign-out button -- a control that exists in
// the code and not on the screen -- and it survived a green suite for the same reason: every
// test built its own body.
//
// So these tests render the dashboard, find the form in the HTML, and post what the page
// actually says to post. A dropped hidden field, a renamed action, a button that stops
// carrying its minutes, or a token that is not this session's all fail here.

// submitRe matches a named submit button. A browser posts the clicked button's name and
// value and no other button's, so the three duration buttons are three different requests
// from the same form and the test has to pick one the way a finger does.
var submitRe = regexp.MustCompile(`(?s)<button\b[^>]*name="([^"]+)"[^>]*value="([^"]*)"[^>]*>`)

func formButtons(fragment string) [][2]string {
	var out [][2]string
	for _, m := range submitRe.FindAllStringSubmatch(fragment, -1) {
		out = append(out, [2]string{m[1], m[2]})
	}
	return out
}

// renderedGet fetches a page as a browser under one session and returns the body plus that
// session's token, so the caller can check the page was handed the real one.
func renderedGet(t *testing.T, s *Server, cookies []*http.Cookie) (string, *httptest.ResponseRecorder) {
	t.Helper()
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
	return rec.Body.String(), rec
}

// postAsRendered submits exactly the fields the page emitted, plus the one button a click
// would add, plus whatever the human typed into a blank field.
func postAsRendered(t *testing.T, s *Server, cookies []*http.Cookie, action string,
	fields map[string]string, button [2]string, typed map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}
	for k, v := range typed {
		form.Set(k, v)
	}
	if button[0] != "" {
		form.Set(button[0], button[1])
	}
	req := httptest.NewRequest("POST", action, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.RemoteAddr = "192.0.2.44:5555"
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// pickButton finds the named button with the given value, and fails the test rather than
// silently posting without it -- a form whose buttons lost their name would otherwise be
// "tested" by a request that carries no duration at all.
func pickButton(t *testing.T, buttons [][2]string, name, value string) [2]string {
	t.Helper()
	for _, b := range buttons {
		if b[0] == name && b[1] == value {
			return b
		}
	}
	t.Fatalf("the rendered form has no %s=%s button; the page offers no way to send it. buttons: %v",
		name, value, buttons)
	return [2]string{}
}

// TestTheDashboardOverrideFormGrantsAsRendered is the whole point: the button on the screen
// produces a granted override through the real chain.
func TestTheDashboardOverrideFormGrantsAsRendered(t *testing.T) {
	f := newOverrideFixture(t)
	resp, wantToken := login(t, f.s, "parent", "hunter2")
	cookies := resp.Cookies()

	body, _ := renderedGet(t, f.s, cookies)
	action, fields := parseForm(t, body, "/users/casey/override")

	got, ok := fields["csrf"]
	if !ok {
		t.Fatalf("the rendered override form carries no csrf field, so every click is refused by the "+
			"CSRF gate. Rendered form: %s", formFragment(body, action))
	}
	if got != wantToken {
		t.Errorf("the form carries csrf %q, want this session's token %q", got, wantToken)
	}
	// The reason field is blank by design, so it carries no value attribute and parseForm --
	// which reads name/value pairs -- cannot see it. Looking for the input itself is the
	// honest check: what matters is that the page gives the operator a way to satisfy the
	// requirement the handler refuses without.
	if !regexp.MustCompile(`(?s)<input\b[^>]*name="reason"`).MatchString(formFragment(body, action)) {
		t.Errorf("the rendered form has no reason field, so there is no way to satisfy the requirement "+
			"the handler refuses without. Rendered form: %s", formFragment(body, action))
	}

	buttons := formButtons(formFragment(body, action))
	click := pickButton(t, buttons, "minutes", "15")

	// The reason is the one field a human types, so the rendered value is empty by design
	// and the test supplies it; everything else is sent exactly as emitted.
	rec := postAsRendered(t, f.s, cookies, action, fields, click,
		map[string]string{"reason": "homework at a friend's"})
	if rec.Code == http.StatusForbidden {
		t.Fatal("the rendered override form was rejected by the CSRF check; the button does nothing")
	}
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST as rendered = %d, want 303; body %q", rec.Code, rec.Body.String())
	}

	got2 := f.overrides()
	if len(got2) != 1 {
		t.Fatalf("the document holds %d overrides after one rendered grant, want 1: %+v", len(got2), got2)
	}
	if got2[0].Username != "casey" || got2[0].Reason != "homework at a friend's" {
		t.Errorf("granted %+v, want casey with the reason that was typed", got2[0])
	}
	// 15 minutes, not some other button's duration: the clicked button is the only one a
	// browser posts, so a form that leaked all three would apply the last.
	if d := got2[0].Until.Sub(insideWindow); d != 15*time.Minute {
		t.Errorf("the override runs %v from now, want 15m (the button that was clicked)", d)
	}
}

// TestTheDashboardWindowButtonEndsAtTheWindow covers the fourth button. It is the one whose
// value the page cannot compute -- the instant comes from the schedule -- so a form that
// posted a stale end time would look identical in the HTML.
func TestTheDashboardWindowButtonEndsAtTheWindow(t *testing.T) {
	f := newOverrideFixture(t)
	resp, _ := login(t, f.s, "parent", "hunter2")
	cookies := resp.Cookies()

	body, _ := renderedGet(t, f.s, cookies)
	action, fields := parseForm(t, body, "/users/casey/override")
	click := pickButton(t, formButtons(formFragment(body, action)), "scope", "window")

	rec := postAsRendered(t, f.s, cookies, action, fields, click,
		map[string]string{"reason": "school trip"})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST as rendered = %d, want 303; body %q", rec.Code, rec.Body.String())
	}
	got := f.overrides()
	if len(got) != 1 {
		t.Fatalf("overrides = %+v, want one", got)
	}
	// The fixture's window closes 06:30 the next day; the override must land on that instant
	// rather than on a duration, which is the difference this button exists for.
	if d := got[0].Until.Sub(insideWindow); d.Minutes() != 450 {
		t.Errorf("the window override ends %v from now, want 7h30m (the window end)", d)
	}
}

// TestTheDashboardClearFormRemovesTheOverrideAsRendered runs the pair end to end: grant with
// the form the page renders, then clear with the form that replaces it once an override is
// open. The clear form is a second piece of markup and can be broken on its own.
func TestTheDashboardClearFormRemovesTheOverrideAsRendered(t *testing.T) {
	f := newOverrideFixture(t)
	resp, _ := login(t, f.s, "parent", "hunter2")
	cookies := resp.Cookies()

	body, _ := renderedGet(t, f.s, cookies)
	action, fields := parseForm(t, body, "/users/casey/override")
	click := pickButton(t, formButtons(formFragment(body, action)), "minutes", "30")
	if rec := postAsRendered(t, f.s, cookies, action, fields, click,
		map[string]string{"reason": "birthday"}); rec.Code != http.StatusSeeOther {
		t.Fatalf("setup grant = %d, want 303; body %q", rec.Code, rec.Body.String())
	}
	if len(f.overrides()) != 1 {
		t.Fatal("setup did not grant an override, so the clear form has nothing to clear")
	}

	// Re-render: the clear form only exists on a row that says "open override".
	body, _ = renderedGet(t, f.s, cookies)
	clearAction, clearFields := parseForm(t, body, "/users/casey/override/clear")
	if _, ok := clearFields["csrf"]; !ok {
		t.Fatalf("the rendered clear form carries no csrf field, so taking an override away is "+
			"impossible from the page. Rendered form: %s", formFragment(body, clearAction))
	}
	rec := postAsRendered(t, f.s, cookies, clearAction, clearFields, [2]string{}, nil)
	if rec.Code == http.StatusForbidden {
		t.Fatal("the rendered clear form was rejected by the CSRF check")
	}
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST clear as rendered = %d, want 303; body %q", rec.Code, rec.Body.String())
	}
	if got := f.overrides(); len(got) != 0 {
		t.Errorf("overrides = %+v after the rendered clear, want none", got)
	}
}

// TestTheDashboardCarriesOneFormPerAccount is what stops a single hand-written row from
// standing in for the page. A form built outside the range would render one control for the
// whole table and still pass every test above.
func TestTheDashboardCarriesOneFormPerAccount(t *testing.T) {
	f := newOverrideFixture(t)
	f.mu.Lock()
	f.doc.Users = append(f.doc.Users,
		config.User{Username: "alexis", ProfileID: "school", Enabled: true},
		config.User{Username: "robin", ProfileID: "school", Enabled: true})
	f.mu.Unlock()

	resp, _ := login(t, f.s, "parent", "hunter2")
	body, _ := renderedGet(t, f.s, resp.Cookies())

	for _, name := range []string{"casey", "alexis", "robin"} {
		if !strings.Contains(body, `action="/users/`+name+`/override"`) {
			t.Errorf("no override form for %q on the rendered dashboard; the page controls one "+
				"account and not the others", name)
		}
	}
}
