package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// Ticket #102's two writes against a profile: the timezone every window is read in, and the
// name the page heads it with.
//
// The timezone is the half with a failure mode. A wrong zone mislabels nothing -- the
// dashboard keeps rendering a lockout computed in the wrong zone as though it were the one the
// operator set, and no cell on any page looks wrong. So the load-bearing test here is
// TestTheTimezoneWriteMovesTheClockOnTheDashboard, which asserts the instant the operator is
// actually released rather than the string that got stored.

// profileFrom reads one profile out of the fixture's document through the production helper,
// so a test cannot pass by looking somewhere the handler did not write.
func profileFrom(t *testing.T, f *overrideFixture, id string) (config.Profile, bool) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	return findProfile(f.doc, id)
}

// snapshot takes the document the way the enforcement loop takes it: through the Config seam,
// which hands back a copy of the struct over slices the next write will not touch. Holding on
// to the result is exactly what the loop does across a tick.
func snapshot(t *testing.T, f *overrideFixture) *config.Config {
	t.Helper()
	c, err := f.load()
	if err != nil {
		t.Fatalf("the config seam failed: %v", err)
	}
	return c
}

func TestTheTimezoneWriteSavesAndAuditsTheProfile(t *testing.T) {
	f := newOverrideFixture(t)
	before := f.savesCount()

	rec := authPost(t, f.s, "/profiles/school/timezone", "timezone=Europe%2FParis")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}

	p, ok := profileFrom(t, f, "school")
	if !ok {
		t.Fatal("the profile the form posted to is gone from the document")
	}
	if p.Timezone != "Europe/Paris" {
		t.Errorf("the document says timezone %q, want Europe/Paris", p.Timezone)
	}
	if p.ID != "school" {
		t.Errorf("the timezone write changed the profile's id to %q; accounts point at it", p.ID)
	}
	if n := f.savesCount() - before; n != 1 {
		t.Errorf("the write saved %d documents, want exactly one", n)
	}

	audit := strings.Join(f.audited.Lines(), "\n")
	// Both zones, not just the new one. This is the edit that moves every window an account
	// has, and "what were the hours before?" is the first question anybody investigating a
	// lockout is going to ask.
	if !strings.Contains(audit, "profile.rezoned") ||
		!strings.Contains(audit, "UTC -> Europe/Paris") {
		t.Errorf("the audit trail does not name the change it made:\n%s", audit)
	}
	if !strings.Contains(audit, "parent") {
		t.Errorf("the audit line does not name who made it:\n%s", audit)
	}
}

// The dashboard's clock, not the config string.
//
// windowEnd is the web layer's own source for "when is this child free", and the dashboard row
// copies EndsAt out of the same verdict rather than re-deriving it (mid-DST-fold, re-deriving
// from the rule's clock text is wrong). So the assertion is on the instant, in absolute time.
//
// The instant is Wed 21 Oct 2026 04:00 UTC. The fixture's rule is 21:00-06:30 on sun-thu, so
// that moment sits inside Tuesday's window in either zone -- the account is locked before and
// after, which is what makes this a test about *when* rather than *whether*. Europe/Paris does
// not leave summer time until 25 Oct 2026, so on the 21st the zone is UTC+2 and the same wall
// clock closes the window two hours earlier in absolute time.
func TestTheTimezoneWriteMovesTheClockOnTheDashboard(t *testing.T) {
	f := newOverrideFixture(t)
	at := time.Date(2026, 10, 21, 4, 0, 0, 0, time.UTC) // a Wednesday
	f.s.Now = func() time.Time { return at }

	held := snapshot(t, f)
	beforeEnd, wasActive := windowEnd(held, "casey", at)
	if !wasActive {
		t.Fatal("the fixture is not inside a window at the chosen instant, so this test " +
			"would prove nothing about when the window ends")
	}
	wantBefore := time.Date(2026, 10, 21, 6, 30, 0, 0, time.UTC)
	if !beforeEnd.Equal(wantBefore) {
		t.Fatalf("before the write the window ends %s, want %s", beforeEnd, wantBefore)
	}

	rec := authPost(t, f.s, "/profiles/school/timezone", "timezone=Europe%2FParis")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}

	afterEnd, isActive := windowEnd(snapshot(t, f), "casey", at)
	if !isActive {
		t.Fatal("the account stopped being locked out at an instant that is inside the " +
			"window in both zones")
	}
	// Wed 06:30 in Paris is 04:30 UTC. The wall clock the parent wrote has not moved; the
	// moment it means has.
	wantAfter := time.Date(2026, 10, 21, 4, 30, 0, 0, time.UTC)
	if !afterEnd.Equal(wantAfter) {
		t.Errorf("after the write the window ends %s, want %s -- the zone changed but the "+
			"instant did not, so the dashboard would still report the old hours", afterEnd, wantAfter)
	}

	// The same instant rendered. The cell carries data-unix precisely so the answer is not a
	// string a test has to re-format and get wrong in the same way the page did.
	f.s.Now = func() time.Time { return at }
	body := authGet(t, f.s, "/").Body.String()
	want := `data-unix="` + strconv.FormatInt(wantAfter.Unix(), 10) + `"`
	if !strings.Contains(body, want) {
		t.Errorf("the dashboard does not render the new instant (%s);\nthe page says:\n%s",
			want, clockCells(body))
	}

	// And the document the loop was already holding must still read the old zone. UpsertProfile
	// replaces the slice rather than writing through the element, which is the whole reason a
	// tick in flight cannot see a half-applied edit.
	if still, _ := windowEnd(held, "casey", at); !still.Equal(wantBefore) {
		t.Errorf("a document held across the write now reports the window ending %s; it should "+
			"still read %s, or a tick in flight sees an edit half way through", still, wantBefore)
	}
}

// clockCells pulls the rendered clock cells out of a dashboard body, so a failure says what the
// page actually showed instead of dumping eleven kilobytes of HTML.
func clockCells(body string) string {
	var found []string
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "data-unix=") {
			found = append(found, strings.TrimSpace(line))
		}
	}
	if len(found) == 0 {
		return "no clock cell at all"
	}
	return strings.Join(found, "\n")
}

func TestAnUnusableTimezoneIsRefusedAndStoresNothing(t *testing.T) {
	f := newOverrideFixture(t)
	before := f.savesCount()

	for _, zone := range []string{"Not/AZone", "Europe/Pariss", "Paris"} {
		rec := authPost(t, f.s, "/profiles/school/timezone",
			"timezone="+url.QueryEscape(zone))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%q answered %d, want 422; body %q", zone, rec.Code, rec.Body.String())
		}
	}

	if p, _ := profileFrom(t, f, "school"); p.Timezone != "UTC" {
		t.Errorf("a refused timezone still changed the document to %q", p.Timezone)
	}
	if n := f.savesCount() - before; n != 0 {
		t.Errorf("%d refusals saved %d documents, want none", 3, n)
	}
	if audit := strings.Join(f.audited.Lines(), "\n"); strings.Contains(audit, "profile.rezoned") {
		t.Errorf("a refused write is in the audit trail, so the trail no longer says what "+
			"actually happened:\n%s", audit)
	}
}

// The typed value is deliberately not echoed. The refusal is a sentence about the field, and
// reflecting request text into a response body is a habit this package does not have even when
// the body is plain text.
func TestTheTimezoneRefusalDoesNotEchoWhatWasTyped(t *testing.T) {
	f := newOverrideFixture(t)
	const typed = "Not/AZone<img>"

	rec := authPost(t, f.s, "/profiles/school/timezone",
		"timezone="+url.QueryEscape(typed))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, typed) ||
		strings.Contains(body, "<img>") {
		t.Errorf("the refusal repeats the submitted value back at the caller:\n%s", body)
	}
}

// A blank field is not "leave it alone" and not "no zone": time.LoadLocation("") answers UTC.
// Saving the blank would move every window in the profile to UTC while the page still looked
// like a profile that had simply not been filled in.
func TestABlankTimezoneIsRefusedRatherThanSavedAsUTC(t *testing.T) {
	f := newOverrideFixture(t)
	before := f.savesCount()

	rec := authPost(t, f.s, "/profiles/school/timezone", "timezone=+%09+")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
	}
	if p, _ := profileFrom(t, f, "school"); p.Timezone != "UTC" {
		t.Errorf("the blank field was stored, and the profile now reads %q", p.Timezone)
	}
	if n := f.savesCount() - before; n != 0 {
		t.Errorf("the refusal saved %d documents, want none", n)
	}
}

// An id from the URL that no profile carries is a 404, and it must not be a 404 that also
// wrote something -- the same shape as the delete route's, for the same reason.
func TestAWriteToAnUnknownProfileIsANotFoundThatChangedNothing(t *testing.T) {
	f := newOverrideFixture(t)
	before := f.savesCount()

	for _, tc := range []struct{ path, form string }{
		{"/profiles/nope/timezone", "timezone=Europe%2FParis"},
		{"/profiles/nope/rename", "name=Renamed"},
	} {
		rec := authPost(t, f.s, tc.path, tc.form)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s answered %d, want 404; body %q", tc.path, rec.Code, rec.Body.String())
		}
	}

	if p, _ := profileFrom(t, f, "school"); p.Timezone != "UTC" || p.Name != "School nights" {
		t.Errorf("a write to a profile that does not exist changed the one that does: %+v", p)
	}
	if n := f.savesCount() - before; n != 0 {
		t.Errorf("the two refusals saved %d documents, want none", n)
	}
	audit := strings.Join(f.audited.Lines(), "\n")
	if strings.Contains(audit, "profile.rezoned") || strings.Contains(audit, "profile.renamed") {
		t.Errorf("refused writes are in the audit trail:\n%s", audit)
	}
}

func TestARenameRelabelsTheProfileWithoutTouchingAnythingElse(t *testing.T) {
	f := newOverrideFixture(t)
	before := f.savesCount()

	rec := authPost(t, f.s, "/profiles/school/rename", "name=+Weeknight+curfew")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}

	p, ok := profileFrom(t, f, "school")
	if !ok {
		t.Fatal("the renamed profile is gone from the document")
	}
	if p.Name != "Weeknight curfew" {
		t.Errorf("name = %q, want %q", p.Name, "Weeknight curfew")
	}
	// The id is what every account points at. A rename that re-derived it would orphan casey
	// and make Validate refuse the document.
	if p.ID != "school" {
		t.Errorf("the rename changed the id to %q; accounts point at it", p.ID)
	}
	if n := f.savesCount() - before; n != 1 {
		t.Errorf("the rename saved %d documents, want exactly one", n)
	}
	audit := strings.Join(f.audited.Lines(), "\n")
	if !strings.Contains(audit, "profile.renamed") ||
		!strings.Contains(audit, `"School nights" -> "Weeknight curfew"`) {
		t.Errorf("the audit trail does not name the change it made:\n%s", audit)
	}
}

// The invariant the ticket names: two rules may already share a name, and a rename must not
// renumber or reorder them. Nothing here re-derives an id, and that is the point -- but "nothing
// here" is exactly what a later refactor breaks, so it is asserted.
func TestARenameLeavesTheRuleIDsAndOrderAlone(t *testing.T) {
	f := newOverrideFixture(t)
	f.mu.Lock()
	p, _ := findProfile(f.doc, "school")
	// Two rules of the same name, the shape TestTwoRulesOfTheSameNameGetDifferentIDs pins.
	p.Rules = append(append([]config.Rule(nil), p.Rules...),
		config.Rule{ID: "r2", Name: "weeknights", Days: []string{"fri"},
			AllDay: true, Enabled: true},
		config.Rule{ID: "r3", Name: "weeknights", Days: []string{"sat"},
			AllDay: true, Enabled: true})
	f.doc.UpsertProfile(p)
	before := append([]string(nil), ruleIDs(p.Rules)...)
	f.mu.Unlock()

	rec := authPost(t, f.s, "/profiles/school/rename", "name=Weeknights")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}

	after, _ := profileFrom(t, f, "school")
	if got := ruleIDs(after.Rules); strings.Join(got, ",") != strings.Join(before, ",") {
		t.Errorf("the rename renumbered or reordered the rules: was %v, now %v", before, got)
	}
}

func ruleIDs(rules []config.Rule) []string {
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		out = append(out, r.ID)
	}
	return out
}

// config.Validate checks ids, never names, and nothing else will: the page heads each profile
// with its name and falls back to the id only when the name is empty, so two profiles called
// the same thing render two identical headings, two identical captions and two identical
// delete confirmations for different documents.
func TestARenameOntoAnotherProfilesNameIsRefused(t *testing.T) {
	f := newOverrideFixture(t)
	before := f.savesCount()
	f.mu.Lock()
	f.doc.UpsertProfile(config.Profile{ID: "weekend", Name: "Weekend rules",
		Timezone: "UTC", Rules: []config.Rule{{ID: "w1", Name: "sat",
			Days: []string{"sat"}, AllDay: true, Enabled: true}}})
	f.mu.Unlock()

	rec := authPost(t, f.s, "/profiles/school/rename", "name=Weekend+rules")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
	}
	if p, _ := profileFrom(t, f, "school"); p.Name != "School nights" {
		t.Errorf("the collision was stored anyway: %q", p.Name)
	}
	if p, _ := profileFrom(t, f, "weekend"); p.Name != "Weekend rules" {
		t.Errorf("the collision check wrote through the other profile: %q", p.Name)
	}
	if n := f.savesCount() - before; n != 0 {
		t.Errorf("the refusal saved %d documents, want none", n)
	}
	audit := strings.Join(f.audited.Lines(), "\n")
	if strings.Contains(audit, "profile.renamed") {
		t.Errorf("a refused rename is in the audit trail:\n%s", audit)
	}
}

// Saving the name that is already there is not a collision. Without the id check in the loop
// this is the refusal an operator cannot get out of: open the page, press Save name, be told
// the name is taken by a profile that is the one being edited.
func TestSavingAProfileItsOwnNameIsNotACollision(t *testing.T) {
	f := newOverrideFixture(t)

	rec := authPost(t, f.s, "/profiles/school/rename", "name=School+nights")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}
	if p, _ := profileFrom(t, f, "school"); p.Name != "School nights" {
		t.Errorf("name = %q, want it unchanged", p.Name)
	}
}

// A blank name is refused rather than stored. The template falls back to the id when the name
// is empty, so a blank field would not clear anything -- it would silently change what the
// heading says while looking like a form that had not been filled in.
func TestABlankNameIsRefusedRatherThanStoredAsEmpty(t *testing.T) {
	f := newOverrideFixture(t)
	before := f.savesCount()

	rec := authPost(t, f.s, "/profiles/school/rename", "name=+++")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
	}
	if p, _ := profileFrom(t, f, "school"); p.Name != "School nights" {
		t.Errorf("the blank name was stored: %q", p.Name)
	}
	if n := f.savesCount() - before; n != 0 {
		t.Errorf("the refusal saved %d documents, want none", n)
	}
}

// Both writes have to survive the trip through the page: the forms are what an operator
// actually submits, and a form whose action does not match a registered route, or that posts a
// field the handler does not read, is invisible to every test above.
func TestTheProfilePageCarriesAWorkingFormForBothWrites(t *testing.T) {
	f := newOverrideFixture(t)

	// One session for the whole test. authGet logs in again, and a second login mints a
	// second session with its own token -- extracting the token from one page and posting it
	// under another is a csrf failure that has nothing to do with the form.
	resp, _ := login(t, f.s, "parent", "hunter2")
	c := cookieOf(t, resp)
	body := getWithSession(t, f.s, "/profiles", c)
	for _, want := range []string{
		`action="/profiles/school/rename"`,
		`action="/profiles/school/timezone"`,
		`name="name"`,
		`name="timezone"`,
		`value="School nights"`,
		`value="UTC"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the profiles page does not carry %s", want)
		}
	}

	// Submit exactly what the page emitted, token included, rather than a body the test
	// assembled: this is the only assertion that the rendered form and the handler agree.
	token := csrfTokenIn(body)
	if token == "" {
		t.Fatal("the rendered page carries no CSRF token to submit")
	}
	rec := submitWithSession(t, f.s, c, "/profiles/school/timezone",
		"timezone=Europe%2FBerlin&csrf="+url.QueryEscape(token))
	// The Location is the assertion, not a detail: a refusal to authenticate and a saved
	// write are both 303, and only where it points tells them apart.
	if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || loc != "/profiles" {
		t.Fatalf("submitting the rendered timezone form answered %d Location=%q, want 303 to "+
			"/profiles; body %q", rec.Code, loc, rec.Body.String())
	}
	if p, _ := profileFrom(t, f, "school"); p.Timezone != "Europe/Berlin" {
		t.Errorf("the rendered form wrote %q, want Europe/Berlin", p.Timezone)
	}

	// The rename form is a second copy of the same shape, and a second copy is where a field
	// name drifts from the one the handler reads. TestEveryFormOnEveryPageCarriesTheToken
	// already proves a token is present on every rendered form; what nothing else proves is
	// that this form round-trips -- that the field the template emits is the field the handler
	// reads, and that the answer is a saved write rather than a refusal wearing the same 303.
	rec = submitWithSession(t, f.s, c, "/profiles/school/rename",
		"name=Weeknight+curfew&csrf="+url.QueryEscape(token))
	if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || loc != "/profiles" {
		t.Fatalf("submitting the rendered rename form answered %d Location=%q, want 303 to "+
			"/profiles; body %q", rec.Code, loc, rec.Body.String())
	}
	if p, _ := profileFrom(t, f, "school"); p.Name != "Weeknight curfew" {
		t.Errorf("the rendered rename form wrote %q, want Weeknight curfew", p.Name)
	}
}

// getWithSession fetches a page under a session the caller already holds, so the token read
// out of the body belongs to the same session the form is submitted with.
func getWithSession(t *testing.T, s *Server, path string, c *http.Cookie) string {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Accept", "text/html")
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s under the session = %d, want 200", path, rec.Code)
	}
	return rec.Body.String()
}

// submitWithSession posts a body a real page emitted, under the session that page was fetched
// with, changing nothing about the body. authPost appends its own token, which is right for
// testing the gate and wrong for testing a form: the point here is that the token the page put
// in the field is the one the handler accepts, and that the field names match.
func submitWithSession(t *testing.T, s *Server, c *http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(c)
	req.RemoteAddr = "192.0.2.71:1"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func csrfTokenIn(body string) string {
	const marker = `name="csrf" value="`
	i := strings.Index(body, marker)
	if i < 0 {
		return ""
	}
	rest := body[i+len(marker):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}
