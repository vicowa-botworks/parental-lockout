package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// Assigning a profile is the write the whole web admin has been waiting for: until it
// existed, a parent could describe a lockout in detail and had no way to put it under a
// child. The tests below are therefore written against the stored document rather than the
// status code, and every refusal asserts that nothing was stored -- a lost assignment is
// invisible on the screen that lost it.

// assignFixture is the override fixture with an account listing it can talk about. The
// listing matters here in a way it did not for overrides: this handler asks whether the
// name in the URL is an account at all, so a fixture whose listing is empty would quietly
// turn every test into the "unknown account" case.
type assignFixture struct {
	*overrideFixture
}

func newAssignFixture(t *testing.T) *assignFixture {
	t.Helper()
	f := &assignFixture{overrideFixture: newOverrideFixture(t)}
	// casey is in pageConfig's document and on the machine; ada is on the machine and
	// not yet managed. Those are the two rows a parent actually meets, and the third
	// (in the document, not on the machine) is added by the stale test on its own.
	f.overrideFixture.s.Accounts = func() (*users.Listing, error) {
		return &users.Listing{
			Accounts: []users.Account{
				{Username: "casey", UID: 1001, Home: "/home/casey", Shell: "/bin/bash"},
				{Username: "ada", UID: 1002, Home: "/home/ada", Shell: "/bin/bash"},
			},
			UIDMin: users.DefaultUIDMin, UIDMax: users.DefaultUIDMax,
		}, nil
	}
	return f
}

// usersNow reads the stored document's assignments under the fixture's mutex.
func (f *assignFixture) usersNow() []config.User {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]config.User(nil), f.doc.Users...)
}

// assignment is the stored entry for one account, or nil when the document no longer holds
// one. A test that searched the slice inline could not tell "absent" from "present and
// switched off" without re-reading the loop, and that distinction is half of what this
// route is for.
func (f *assignFixture) assignment(name string) *config.User {
	for _, u := range f.usersNow() {
		if u.Username == name {
			out := u
			return &out
		}
	}
	return nil
}

func TestAnUnauthenticatedAssignmentIsRefusedAndStoresNothing(t *testing.T) {
	// Two contracts, both measured rather than assumed: a browser is sent to sign in, a
	// programmatic caller is refused in place. Asserting only one would let the other
	// regress unnoticed, and "the unauthenticated case" is not one answer.
	for _, tc := range []struct {
		name   string
		accept string
		want   int
	}{
		{"browser", "text/html", http.StatusSeeOther},
		{"programmatic", "application/json", http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAssignFixture(t)
			before := len(f.usersNow())
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/users/ada/profile",
				strings.NewReader("profile=school&enabled=on"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			req.RemoteAddr = "192.0.2.71:1"
			f.s.Handler().ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
			if tc.want == http.StatusSeeOther && rec.Header().Get("Location") != "/login" {
				t.Errorf("Location = %q; want /login", rec.Header().Get("Location"))
			}
			if got := len(f.usersNow()); got != before {
				t.Errorf("an unauthenticated post changed the document: %d users, was %d", got, before)
			}
			if f.savesCount() != 0 {
				t.Errorf("an unauthenticated post still reached the write seam %d times", f.savesCount())
			}
		})
	}
}

func TestAnAssignmentWithoutACSRFTokenIsRefusedAndStoresNothing(t *testing.T) {
	f := newAssignFixture(t)
	resp, _ := login(t, f.s, "parent", "hunter2")
	c := cookieOf(t, resp)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/users/ada/profile",
		strings.NewReader("profile=school&enabled=on"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(c)
	req.RemoteAddr = "192.0.2.71:1"
	f.s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 from the CSRF gate", rec.Code)
	}
	if f.assignment("ada") != nil {
		t.Error("a request the CSRF gate refused still assigned a profile")
	}
}

func TestAssigningAProfileStoresTheAssignmentAndAuditsIt(t *testing.T) {
	f := newAssignFixture(t)

	rec := authPost(t, f.s, "/users/ada/profile", "profile=school&enabled=on")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	got := f.assignment("ada")
	if got == nil {
		t.Fatalf("nothing was stored; the document holds %v", f.usersNow())
	}
	if got.ProfileID != "school" || !got.Enabled {
		t.Errorf("stored %+v, want profile school and enabled", *got)
	}
	line := strings.Join(f.audited.Lines(), "\n")
	// The audit line has to carry the switch as well as the profile: "ada → school"
	// recorded against an entry that is switched off is a trail that describes a lockout
	// that will never happen.
	for _, want := range []string{"user.assigned", "ada", "school", "enabled"} {
		if !strings.Contains(line, want) {
			t.Errorf("the audit line does not mention %q; the audit was:\n%s", want, line)
		}
	}
}

// The checkbox is absent from the post when it is clear, which is the only way a browser
// can say "off". Read the other way -- defaulting to enabled -- every save from a row that
// was already switched off would silently switch it back on.
func TestAnAssignmentWithTheCheckboxClearIsStoredSwitchedOff(t *testing.T) {
	f := newAssignFixture(t)

	rec := authPost(t, f.s, "/users/ada/profile", "profile=school")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	got := f.assignment("ada")
	if got == nil {
		t.Fatal("nothing was stored")
	}
	if got.Enabled {
		t.Error("an absent checkbox was read as enabled")
	}
	if line := strings.Join(f.audited.Lines(), "\n"); !strings.Contains(line, "switched off") {
		t.Errorf("the audit does not say the entry is off; the audit was:\n%s", line)
	}
}

func TestAssigningAnUnknownProfileIsRefusedAndStoresNothing(t *testing.T) {
	f := newAssignFixture(t)

	rec := authPost(t, f.s, "/users/ada/profile", "profile=nonexistent&enabled=on")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body %s", rec.Code, rec.Body.String())
	}
	if f.assignment("ada") != nil {
		t.Error("an assignment naming a profile that does not exist was stored")
	}
	if f.savesCount() != 0 {
		t.Errorf("the refusal still reached the write seam %d times", f.savesCount())
	}
}

// The route takes the account name out of the URL, so nothing about the form limits it to a
// real account. config.Validate cannot catch this one either -- "username is non-empty and
// unique" is true of every string -- so the handler is the only place that knows whether
// anybody by that name exists.
func TestAssigningAnAccountNobodyHasIsRefused(t *testing.T) {
	f := newAssignFixture(t)

	rec := authPost(t, f.s, "/users/ghost/profile", "profile=school&enabled=on")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body %s", rec.Code, rec.Body.String())
	}
	if f.assignment("ghost") != nil {
		t.Error("the document now manages an account that does not exist")
	}
}

// A listing that could not be read is the server's failure, and it must not arrive as a
// statement about the name the operator typed. This is the same rule the audit page holds:
// a failed read never answers "no such account".
func TestAFailedAccountListingIsNotAnsweredAsNoSuchAccount(t *testing.T) {
	f := newAssignFixture(t)
	f.s.Accounts = func() (*users.Listing, error) {
		return nil, errors.New("getpwent: no such file")
	}

	rec := authPost(t, f.s, "/users/ada/profile", "profile=school&enabled=on")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 -- a failed read must not be reported as a wrong name", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "no account on this machine") {
		t.Errorf("the refusal claims the account does not exist after a read that could not answer; body:\n%s", body)
	}
	if f.assignment("ada") != nil {
		t.Error("a request the server could not check still wrote the document")
	}
}

// Stopping management goes through RemoveUser rather than an empty profile id, and the
// difference is observable: RemoveUser takes the account's override with it, so a grant
// made for one child is not inherited by whoever next uses the name.
func TestStoppingManagementTakesTheOverrideWithTheAssignment(t *testing.T) {
	f := newAssignFixture(t)
	if rec := authPost(t, f.s, "/users/casey/override", "minutes=60&reason=homework"); rec.Code != http.StatusSeeOther {
		t.Fatalf("setup grant: status %d, body %s", rec.Code, rec.Body.String())
	}
	if len(f.overrides()) != 1 {
		t.Fatalf("setup: expected one override, got %v", f.overrides())
	}

	rec := authPost(t, f.s, "/users/casey/profile", "profile=")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if f.assignment("casey") != nil {
		t.Error("the account is still managed")
	}
	if got := f.overrides(); len(got) != 0 {
		t.Errorf("the override outlived the account it was granted to: %+v", got)
	}
	if line := strings.Join(f.audited.Lines(), "\n"); !strings.Contains(line, "user.unassigned") {
		t.Errorf("unassigning was not audited; the audit was:\n%s", line)
	}
}

// An account on the machine that was never managed has nothing to stop managing, and 303
// would say it did.
func TestStoppingManagementOfAnAccountThatWasNeverManagedSaysSo(t *testing.T) {
	f := newAssignFixture(t)

	rec := authPost(t, f.s, "/users/ada/profile", "profile=")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body %s", rec.Code, rec.Body.String())
	}
	if f.savesCount() != 0 {
		t.Errorf("the refusal still reached the write seam %d times", f.savesCount())
	}
}

// A row in the document whose account is gone is the one row the operator needs a way out
// of, and the listing is what would have called it unknown.
func TestAStaleEntryCanBeUnmanaged(t *testing.T) {
	f := newAssignFixture(t)
	f.mu.Lock()
	f.doc.Users = append(append([]config.User(nil), f.doc.Users...),
		config.User{Username: "oldkid", ProfileID: "school", Enabled: true})
	f.mu.Unlock()

	rec := authPost(t, f.s, "/users/oldkid/profile", "profile=")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if f.assignment("oldkid") != nil {
		t.Error("the stale entry survived")
	}
}

var selectedOptionRe = regexp.MustCompile(`(?s)<option value="([^"]*)"[^>]*selected`)

// The form on the page, posted exactly as the page renders it.
//
// Every other test in this file hand-builds its body, which is what let the shipped logout
// form post a token-less request for as long as it did: a hand-built body passes while the
// control a browser actually submits is broken. Here the action, the token and the chosen
// option all come out of the rendered HTML, so a template that drops the hidden field or
// renames a control fails this test and nothing else.
func TestTheRenderedAssignmentFormPostsWhatTheHandlerAccepts(t *testing.T) {
	f := newAssignFixture(t)
	resp, _ := login(t, f.s, "parent", "hunter2")
	c := cookieOf(t, resp)

	get := httptest.NewRequest("GET", "/users", nil)
	get.Header.Set("Accept", "text/html")
	get.AddCookie(c)
	rec := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(rec, get)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /users: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	action, fields := parseForm(t, body, "/users/ada/profile")
	token, ok := fields["csrf"]
	if !ok || token == "" {
		t.Fatalf("the rendered form carries no CSRF token; the form is:\n%s",
			formFragment(body, action))
	}
	// The profile comes from a <select>, which parseForm does not read -- it walks
	// <input> elements. So the chosen option is taken from the markup the same way a
	// browser would: the option marked selected, or the first one if none is.
	profile := ""
	if m := selectedOptionRe.FindStringSubmatch(formFragment(body, action)); m != nil {
		profile = m[1]
	} else {
		m := regexp.MustCompile(`(?s)<option value="([^"]+)"`).FindStringSubmatch(formFragment(body, action))
		if m == nil {
			t.Fatalf("the form offers no profile to assign; the form is:\n%s", formFragment(body, action))
		}
		profile = m[1]
	}
	enabled := ""
	if strings.Contains(formFragment(body, action), `name="enabled" value="on" checked`) {
		enabled = "&enabled=on"
	}

	post := httptest.NewRecorder()
	req := httptest.NewRequest("POST", action,
		strings.NewReader("csrf="+token+"&profile="+profile+enabled))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(c)
	req.RemoteAddr = "192.0.2.71:1"
	f.s.Handler().ServeHTTP(post, req)

	if post.Code != http.StatusSeeOther {
		t.Fatalf("posting the rendered form was refused: %d %s", post.Code, post.Body.String())
	}
	if f.assignment("ada") == nil {
		t.Errorf("the form posted successfully but stored nothing (posted profile=%q)", profile)
	}
}

// The dropdown is the only place a parent learns which profiles can be chosen, so an empty
// one is the page saying "there is nothing to assign" on a machine that has profiles.
func TestTheAssignmentFormOffersEveryProfile(t *testing.T) {
	f := newAssignFixture(t)
	f.mu.Lock()
	f.doc.Profiles = append(f.doc.Profiles,
		config.Profile{ID: "weekend", Name: "Weekend curfew", Timezone: "UTC"})
	f.mu.Unlock()

	rec := authGet(t, f.s, "/users")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`value="school"`, `value="weekend"`, "Weekend curfew", "School nights"} {
		if !strings.Contains(body, want) {
			t.Errorf("the dropdown does not offer %q", want)
		}
	}
	// The row already under a profile has to arrive with that profile chosen, or saving
	// any other field on the row silently moves the child to whatever the browser shows
	// first.
	if !strings.Contains(body, `value="school" selected`) {
		t.Error("casey's row does not show its current profile as selected")
	}
}

// The order the dropdown offers profiles in is the only thing distinguishing two profiles
// with the same name, and a control whose options follow document order reshuffles whenever
// a profile is created or deleted elsewhere in the file.
func TestTheAssignmentDropdownIsInNameOrderNotDocumentOrder(t *testing.T) {
	f := newAssignFixture(t)
	f.mu.Lock()
	// Deliberately last in the document and first by name: the two orders disagree, so
	// the rendering says which one it used.
	f.doc.Profiles = append(f.doc.Profiles,
		config.Profile{ID: "z9", Name: "Alpha study", Timezone: "UTC"})
	f.mu.Unlock()

	rec := authGet(t, f.s, "/users")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	form := formFragment(rec.Body.String(), "/users/ada/profile")
	alpha := strings.Index(form, `value="z9"`)
	school := strings.Index(form, `value="school"`)
	if alpha < 0 || school < 0 {
		t.Fatalf("both options are not offered; the form is:\n%s", form)
	}
	if alpha > school {
		t.Error("the dropdown follows document order; a profile added later should not move the others")
	}
}
