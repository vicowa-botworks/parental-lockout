package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/schedule"
)

// Ticket #10's two remaining acceptance criteria, both asserted against the evaluator rather
// than against a rendering:
//
//   "Creating a rule through the UI produces a config that schedule.Evaluate agrees with --
//    an assertion, not a screenshot."
//   "Deleting a profile in use is refused with a message naming the blocking users."

// postForm posts a form body as a signed-in parent, without the helper appending the token.
// The rendered-form test needs to submit exactly what the page emitted, token included.
func postForm(t *testing.T, s *Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.RemoteAddr = "192.0.2.71:1"
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// Criterion 2, the load-bearing one.
//
// The risk this pins is a UI that writes a rule the evaluator reads differently from the way
// the form meant it -- a day list the evaluator widens, a window crossing midnight attached
// to the wrong day, an all-day checkbox that stores a near-miss 00:00-23:59. Every one of
// those still renders a plausible page. Only the evaluator notices.
func TestARuleCreatedThroughTheUIIsOneTheEvaluatorAgreesWith(t *testing.T) {
	f := newOverrideFixture(t)

	rec := authPost(t, f.s, "/profiles/school/rules",
		"name=Weekend+curfew&days=sat&days=sun&start=22%3A30&end=07%3A00")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}

	f.mu.Lock()
	prof, ok := findProfile(f.doc, "school")
	f.mu.Unlock()
	if !ok {
		t.Fatal("the profile the form posted to is gone from the document")
	}
	if len(prof.Rules) != 2 {
		t.Fatalf("profile holds %d rules, want the original plus the new one", len(prof.Rules))
	}

	var added *config.Rule
	for i := range prof.Rules {
		if prof.Rules[i].Name == "Weekend curfew" {
			added = &prof.Rules[i]
		}
	}
	if added == nil {
		t.Fatalf("the new rule is not in the document: %+v", prof.Rules)
	}
	if added.ID == "" || added.ID == "r1" {
		t.Errorf("the new rule has no usable id (%q); a duplicate would make Validate refuse "+
			"the whole document", added.ID)
	}
	if !added.Enabled {
		t.Error("a rule a parent just created is stored disabled, so it enforces nothing")
	}

	// The assertion the criterion asks for: the evaluator, on the saved document.
	// Saturday 23:00 is inside the window.
	inSat := time.Date(2026, 10, 17, 23, 0, 0, 0, time.UTC) // a Saturday
	v := schedule.Evaluate(&prof, inSat)
	if !v.Active {
		t.Errorf("Evaluate says nothing is enforcing at Sat 23:00; the rule the UI wrote is %s %s %s",
			strings.Join(added.Days, ","), added.Start, added.End)
	}
	if v.Rule == nil || v.Rule.ID != added.ID {
		t.Errorf("Evaluate is active on the wrong rule: %+v", v.Rule)
	}

	// Sunday 02:00 is inside the same rule's window, which crosses midnight and so
	// belongs to the starting day. A UI that attached the window to the ending day
	// would still look right on the page.
	inSunMorning := time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC) // a Sunday
	if v := schedule.Evaluate(&prof, inSunMorning); !v.Active {
		t.Errorf("the window crossing midnight is not active on the morning after; it is "+
			"attached to the wrong day: %s %s-%s", strings.Join(added.Days, ","), added.Start, added.End)
	}

	// Monday 12:00 is outside every window in this profile.
	outMonday := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC) // a Monday
	if v := schedule.Evaluate(&prof, outMonday); v.Active {
		t.Errorf("the new rule is enforcing on Monday at noon; days=%s window=%s-%s",
			strings.Join(added.Days, ","), added.Start, added.End)
	}
}

// An all-day rule must store the flag, not a near-miss clock pair. 00:00-23:59 is a minute
// short of all day, and the evaluator has a distinct path for the flag.
func TestAnAllDayRuleFromTheUIUsesTheFlagNotAClockPair(t *testing.T) {
	f := newOverrideFixture(t)

	rec := authPost(t, f.s, "/profiles/school/rules",
		"name=Exams&days=mon&days=tue&allDay=on")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}

	f.mu.Lock()
	prof, _ := findProfile(f.doc, "school")
	f.mu.Unlock()

	var added *config.Rule
	for i := range prof.Rules {
		if prof.Rules[i].Name == "Exams" {
			added = &prof.Rules[i]
		}
	}
	if added == nil {
		t.Fatal("the all-day rule was not stored")
	}
	if !added.AllDay {
		t.Fatalf("the all-day checkbox stored a clock rule instead: %+v", added)
	}

	// Monday 15:00 is mid-afternoon: only an actual all-day rule is active there.
	midAfternoon := time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)
	if v := schedule.Evaluate(&prof, midAfternoon); !v.Active {
		t.Error("the stored all-day rule is not enforcing mid-afternoon Monday")
	}
}

// Criterion 3.
func TestDeletingAProfileInUseIsRefusedNamingEveryAccount(t *testing.T) {
	f := newOverrideFixture(t)

	// Two accounts share the profile, which is what a profile is for.
	f.mu.Lock()
	f.doc.UpsertUser(config.User{Username: "alexis", ProfileID: "school", Enabled: true})
	f.doc.UpsertUser(config.User{Username: "bridget", ProfileID: "school", Enabled: true})
	f.mu.Unlock()

	before := f.savesCount()
	rec := authPost(t, f.s, "/profiles/school/delete", "")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"casey", "alexis", "bridget"} {
		if !strings.Contains(body, want) {
			t.Errorf("the refusal does not name %q, so the operator has to discover it one "+
				"account per retry; body:\n%s", want, body)
		}
	}
	if got := f.savesCount(); got != before {
		t.Errorf("the refused delete still wrote: saves went %d -> %d", before, got)
	}
	f.mu.Lock()
	_, stillThere := findProfile(f.doc, "school")
	f.mu.Unlock()
	if !stillThere {
		t.Error("the profile was removed despite the refusal")
	}
}

// A profile nothing is assigned to really is deleted. Without this the refusal test passes
// against a handler that refuses everything.
func TestDeletingAnUnassignedProfileSucceeds(t *testing.T) {
	f := newOverrideFixture(t)

	f.mu.Lock()
	f.doc.UpsertProfile(config.Profile{ID: "spare", Name: "Spare", Timezone: "UTC"})
	f.mu.Unlock()

	rec := authPost(t, f.s, "/profiles/spare/delete", "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}
	f.mu.Lock()
	_, stillThere := findProfile(f.doc, "spare")
	f.mu.Unlock()
	if stillThere {
		t.Error("an unassigned profile survived a delete that should have removed it")
	}
}

// The class #59's review found on the logout button: a form that renders without its token
// can never be accepted, and every test that posts a hand-built body misses it. So this
// submits the form the template actually produced.
func TestTheRenderedRuleFormCanBeSubmitted(t *testing.T) {
	f := newOverrideFixture(t)

	// One session for both the GET and the POST. authGet would log in again, and the
	// token read from the first session's page would then be checked against the second
	// session -- a guaranteed 403 that says nothing about the form.
	resp, _ := login(t, f.s, "parent", "hunter2")
	c := cookieOf(t, resp)

	get := httptest.NewRequest("GET", "/profiles", nil)
	get.Header.Set("Accept", "text/html")
	get.AddCookie(c)
	rec := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(rec, get)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %q", rec.Code, rec.Body.String())
	}
	page := rec.Body.String()

	action, fields := parseForm(t, page, "/profiles/school/rules")
	if action != "/profiles/school/rules" {
		t.Errorf("the form posts to %q, not to the route that creates a rule", action)
	}
	token := fields["csrf"]
	if token == "" {
		t.Fatalf("the rendered rule form carries no csrf field, so every submit of it is "+
			"refused -- the exact defect found on the logout form. Form as rendered:\n%s",
			formFragment(page, "/profiles/school/rules"))
	}

	// Fill in what a parent types, keeping the token the page emitted.
	body := url.Values{
		"csrf":  {token},
		"name":  {"Library run"},
		"days":  {"fri"},
		"start": {"16:00"},
		"end":   {"18:00"},
	}.Encode()

	req := httptest.NewRequest("POST", action, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(c)
	post := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(post, req)

	if post.Code != http.StatusSeeOther {
		t.Fatalf("the form as rendered was refused: status %d, body %q", post.Code, post.Body.String())
	}
}

// A rule the evaluator would skip must not be savable through the UI either. This is the
// validate-before-save path doing work for rules that it did for overrides.
func TestARuleThatEnforcesNothingIsRefusedRatherThanSaved(t *testing.T) {
	f := newOverrideFixture(t)

	before := f.savesCount()
	// start == end is an empty window: it locks out nobody, and Validate refuses it so
	// the typo is caught here rather than on a night the child had the run of the box.
	rec := authPost(t, f.s, "/profiles/school/rules",
		"name=Typo&days=fri&start=21%3A00&end=21%3A00")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
	}
	if got := f.savesCount(); got != before {
		t.Errorf("an empty window was saved: saves went %d -> %d", before, got)
	}
	if !strings.Contains(rec.Body.String(), "start") {
		t.Errorf("the refusal does not point at the field to fix; body:\n%s", rec.Body.String())
	}
}

// A rule with no day ticked is refused by the handler with the form's own words, not by
// Validate with the document's.
func TestARuleWithNoDayTickedIsRefusedWithTheFormsOwnWords(t *testing.T) {
	f := newOverrideFixture(t)

	before := f.savesCount()
	rec := authPost(t, f.s, "/profiles/school/rules", "name=NoDays&start=21%3A00&end=06%3A00")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
	}
	if got := f.savesCount(); got != before {
		t.Errorf("saves went %d -> %d", before, got)
	}
	if !strings.Contains(rec.Body.String(), "at least one day") {
		t.Errorf("the refusal is not the form's own sentence; body:\n%s", rec.Body.String())
	}
}

// Posting to a profile that does not exist is a 404, not a write into an unrelated profile.
func TestARuleForAnUnknownProfileIsNotFound(t *testing.T) {
	f := newOverrideFixture(t)

	before := f.savesCount()
	rec := authPost(t, f.s, "/profiles/no-such/rules",
		"name=Orphan&days=fri&start=16%3A00&end=18%3A00")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %q", rec.Code, rec.Body.String())
	}
	if got := f.savesCount(); got != before {
		t.Errorf("saves went %d -> %d", before, got)
	}
}

// Two rules with the same name must not collide on the derived id: a duplicate id makes
// Validate refuse the whole document, which would lock the operator out of the UI entirely.
func TestTwoRulesOfTheSameNameGetDifferentIDs(t *testing.T) {
	f := newOverrideFixture(t)

	for i := 0; i < 2; i++ {
		rec := authPost(t, f.s, "/profiles/school/rules",
			"name=School+night&days=mon&start=21%3A00&end=06%3A30")
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("submission %d: status %d, body %q", i, rec.Code, rec.Body.String())
		}
	}

	f.mu.Lock()
	prof, _ := findProfile(f.doc, "school")
	f.mu.Unlock()

	seen := map[string]bool{}
	for _, r := range prof.Rules {
		if seen[r.ID] {
			t.Fatalf("two rules share id %q; Validate refuses that, so the next save of any "+
				"kind would fail: %+v", r.ID, prof.Rules)
		}
		seen[r.ID] = true
	}
	if len(prof.Rules) != 3 {
		t.Errorf("stored %d rules, want the original plus two", len(prof.Rules))
	}
}

// The page has to show the evaluator's degraded report, or a profile that enforces nothing
// looks identical to one that does.
func TestTheProfilesPageShowsTheEvaluatorsComplaint(t *testing.T) {
	f := newOverrideFixture(t)

	f.mu.Lock()
	f.doc.UpsertProfile(config.Profile{
		ID: "broken", Name: "Broken", Timezone: "UTC",
		// A rule that can never fire is the degraded shape ProfileErrors reports.
		Rules: []config.Rule{{ID: "never", Name: "never",
			Days: []string{"mon"}, Start: "00:00", End: "00:00", Enabled: true}},
	})
	f.mu.Unlock()

	rec := authGet(t, f.s, "/profiles")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Broken") {
		t.Fatalf("the profile is not on the page at all")
	}
	// Whatever the evaluator's sentence is, the page must carry some of it rather than
	// presenting the profile as healthy.
	if strings.Contains(body, `class="problem"`) == false {
		t.Errorf("the page shows no problem for a profile the evaluator reports as degraded;\n%s", body)
	}
}

// The page must not render a rule as healthy when enforcement is switched off globally.
func TestTheProfilesPageSaysWhenEnforcementIsOff(t *testing.T) {
	f := newOverrideFixture(t)

	f.mu.Lock()
	off := false
	f.doc.Settings.Enforcement = off
	f.mu.Unlock()

	rec := authGet(t, f.s, "/profiles")
	if !strings.Contains(rec.Body.String(), "switched off") {
		t.Errorf("the page does not say enforcement is off, so a rule added here looks like "+
			"it locks someone out;\n%s", rec.Body.String())
	}
}

// Criterion: "Every route has a handler test: unauthenticated -> rejected".
//
// One table over every route rather than a test per route, because the per-route form
// rots: a new route ships without its test and nothing notices. A table that has to be
// read against the router is the same problem, so this one asserts the route set it covers
// is the set the server actually serves.
func TestEveryRouteRefusesAnUnauthenticatedRequest(t *testing.T) {
	f := newOverrideFixture(t)
	before := f.savesCount()

	// A refusal is answered the way the caller asked: a browser is redirected to the
	// form it can render, a program gets 401 it can read. Asserting "not 200" would
	// pass on either, and would also pass on a 303 that meant success.
	gets := []string{"/", "/users", "/profiles", "/login"}
	posts := []string{
		"/profiles/school/rules",
		"/profiles/school/delete",
		"/users/casey/override",
		"/users/casey/override/clear",
	}

	// The login form is the one page an anonymous caller is allowed to see; everything
	// else must send them there.
	allowed := map[string]bool{"/login": true}

	check := func(method, path string) {
		t.Helper()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader("csrf=bogus&name=x&days=mon"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "text/html")
		f.s.Handler().ServeHTTP(rec, req)

		if allowed[path] {
			if rec.Code != http.StatusOK {
				t.Errorf("%s %s: status = %d, want 200 -- nothing can sign in otherwise",
					method, path, rec.Code)
			}
		} else if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
			t.Errorf("%s %s: anonymous browser request got %d (Location %q), want 303 to /login",
				method, path, rec.Code, rec.Header().Get("Location"))
		}

		// A program gets 401, not a redirect into an HTML form it cannot render.
		rec2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(method, path, nil)
		req2.Header.Set("Accept", "application/json")
		f.s.Handler().ServeHTTP(rec2, req2)
		if !allowed[path] && rec2.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: anonymous JSON request got %d, want 401", method, path, rec2.Code)
		}
	}

	for _, path := range gets {
		check("GET", path)
	}
	for _, path := range posts {
		check("POST", path)
	}

	if got := f.savesCount(); got != before {
		t.Errorf("an unauthenticated request wrote anyway: saves went %d -> %d", before, got)
	}
}

// A rule name is attacker-influenced for the same reason a username is: the config file is
// root-owned, but a name typed once on this UI is read back on every page forever.
func TestARuleNameIsRenderedAsTextNotMarkup(t *testing.T) {
	f := newOverrideFixture(t)

	name := `<script>alert(1)</script>`
	rec := authPost(t, f.s, "/profiles/school/rules",
		"name="+url.QueryEscape(name)+"&days=mon&start=21%3A00&end=06%3A30")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}

	page := authGet(t, f.s, "/profiles").Body.String()
	if strings.Contains(page, "<script>alert(1)</script>") {
		t.Error("the rule name was written into the document as markup")
	}
	if !strings.Contains(page, "&lt;script&gt;") {
		t.Error("the rule name is not in the page at all, escaped or otherwise")
	}
	// The delete confirmation is the other place a name reappears, and it is a form
	// attribute as well as text.
	if n := strings.Count(page, "<script>"); n != 0 {
		t.Errorf("the page contains %d live <script> tags after rendering a hostile name", n)
	}
}

// A hostile name must also be inert where it is used as an identifier rather than as text:
// the rule id reaches a form action, and an action is a URL.
func TestAHostileProfileIDCannotEscapeAFormAction(t *testing.T) {
	f := newOverrideFixture(t)

	f.mu.Lock()
	f.doc.UpsertProfile(config.Profile{
		ID: `x" formaction="/evil`, Name: "Weird", Timezone: "UTC",
		Rules: []config.Rule{{ID: "r", Name: "r", Days: []string{"mon"},
			Start: "21:00", End: "06:00", Enabled: true}},
	})
	f.mu.Unlock()

	page := authGet(t, f.s, "/profiles").Body.String()
	if strings.Contains(page, `formaction="/evil`) {
		t.Error("a profile id escaped its attribute and injected a competing form action")
	}
	if !strings.Contains(page, "Weird") {
		t.Error("the profile is not on the page, so the escape test proves nothing")
	}
}
