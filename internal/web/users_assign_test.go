package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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

	rec := postRowForm(t, f, "oldkid")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if f.assignment("oldkid") != nil {
		t.Error("the stale entry survived")
	}
}

// The form on the page, posted exactly as the page renders it.
//
// Every other test in this file hand-builds its body, which is what let the shipped logout
// form post a token-less request for as long as it did: a hand-built body passes while the
// control a browser actually submits is broken. Here the action, the token and the chosen
// option all come out of the rendered HTML, so a template that drops the hidden field or
// renames a control fails this test and nothing else.
func TestTheRenderedAssignmentFormPostsWhatTheHandlerAccepts(t *testing.T) {
	// What the page MARKS and what the operator CHOOSES are different things, and conflating them is
	// what kept the old fallback invisible for a whole round. The template marks the option equal to
	// the account's CURRENT profile (users.tmpl:88), so on a row that already has an entry the marked
	// value, the posted value and the pre-existing document are all the same string -- "the stored id
	// equals the marked one" is then satisfied by the document as it stood BEFORE the POST. Measured:
	// deleting the positive write in users_write.go (UpsertUser -> `_ = enabled`) left this test green
	// while TestAssigningAProfileStoresTheAssignmentAndAuditsIt and
	// TestAnAssignmentWithTheCheckboxClearIsStoredSwitchedOff both went red. So each case names the
	// choice the operator makes and posts an option the page OFFERS but does not mark. It is still
	// read out of the rendered markup -- the option list is the page's own output -- which is what
	// keeps this from being the guess the previous commit removed.
	for _, tc := range []struct {
		name     string
		username string
		choose   string
	}{
		// A managed row moving to a different profile: the page marks `school`, the operator picks
		// the other thing the page offers.
		{name: "reassigning a managed account", username: "casey", choose: "weekend"},
		// The first-time assign: the row marks nothing, and this is the verb retargeting away from
		// ada had given up. Naming the choice is what makes it reachable without inventing it.
		{name: "assigning an account with no entry yet", username: "ada", choose: "school"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAssignFixture(t)
			f.mu.Lock()
			f.doc.Profiles = append(append([]config.Profile(nil), f.doc.Profiles...),
				config.Profile{ID: "weekend", Name: "Weekend curfew", Timezone: "UTC"})
			f.mu.Unlock()
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

			action, fields := parseForm(t, body, "/users/"+tc.username+"/profile")
			token, ok := fields["csrf"]
			if !ok || token == "" {
				t.Fatalf("the rendered form carries no CSRF token; the form is:\n%s",
					formFragment(body, action))
			}
			// The profile comes from a <select>, which parseForm does not read -- it walks <input>
			// elements -- so the chosen option is taken from the markup. From the OPTION LIST, not
			// from the marked option: see the table above.
			frag := formFragment(body, action)
			profile := offeredOption(t, frag, tc.choose)
			enabled := ""
			if strings.Contains(frag, `name="enabled" value="on" checked`) {
				enabled = "&enabled=on"
			}

			post := httptest.NewRecorder()
			req := httptest.NewRequest("POST", action,
				strings.NewReader("csrf="+token+"&profile="+url.QueryEscape(profile)+enabled))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Accept", "text/html")
			req.AddCookie(c)
			req.RemoteAddr = "192.0.2.71:1"
			f.s.Handler().ServeHTTP(post, req)

			if post.Code != http.StatusSeeOther {
				t.Fatalf("posting the rendered form was refused: %d %s", post.Code, post.Body.String())
			}
			got := f.assignment(tc.username)
			if got == nil {
				t.Fatalf("the form posted successfully but stored nothing (posted profile=%q)", profile)
			}
			// "Something got stored" cannot say WHICH profile it posted, and that is exactly how an
			// invented one stayed invisible.
			if got.ProfileID != profile {
				t.Errorf("the form posted profile %q and the handler stored %q", profile, got.ProfileID)
			}
			// The body carries `enabled` too, read off the row's `checked`, and the class is covered
			// elsewhere -- but not HERE, which is the point of a test that reads the page. Mutant
			// `enabled := r.FormValue("enabled") != ""` -> `enabled := true` (users_write.go:63) left
			// this test taking the 303 and checking only the profile it was already checking for.
			// The teeth are in the `ada` case; `casey`'s entry is already enabled:true in the fixture,
			// so her row cannot distinguish the two -- the same asymmetry the table exists to cover.
			if got.Enabled != (enabled != "") {
				t.Errorf("the form posted enabled=%v and the handler stored enabled=%v",
					enabled != "", got.Enabled)
			}
		})
	}
}

// offeredOption returns want if the rendered <select> actually offers it, and fails otherwise. It
// exists so a test can name the profile the OPERATOR picked without inventing one: reading the
// marked option says what the page already thinks, and `<option value="([^"]+)">` -- the fallback
// this file used until #197 -- silently skips the value="" placeholder and hands back the first real
// profile, which is a different VERB on a row that marks nothing (the browser would submit removal).
// The guard is membership in the page's own option list, so a profile renamed or removed upstream
// fails here rather than posting a value nothing offered.
func offeredOption(t *testing.T, frag, want string) string {
	t.Helper()
	for _, m := range regexp.MustCompile(`(?s)<option value="([^"]*)"`).FindAllStringSubmatch(frag, -1) {
		if m[1] == want {
			return want
		}
	}
	t.Fatalf("the page does not offer profile %q, so the test may not post it; the row is:\n%s",
		want, frag)
	return ""
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

// #192: a machine with no profiles rendered a dropdown containing exactly one option.
//
// The report was that the Accounts page "only has the Not Managed option in the drop down box, and
// there is no way to make it managed" -- true on every fresh install, because the dropdown's real
// options are `range $.Profiles` and that is empty until somebody makes a profile. The enabled
// checkbox then stored a flag with nothing to enable, and Save was refused with a sentence about
// *stopping* management. The Profiles page had already been given this treatment (#135) and pinned;
// the Accounts page had never been rendered with zero profiles by any test.

// TestTheUsersPageOffersNoDeadDropdownWithoutProfiles is the reported symptom, asserted on the
// rendered page rather than on the config that produced it.
func TestTheUsersPageOffersNoDeadDropdownWithoutProfiles(t *testing.T) {
	f := newAssignFixture(t)
	f.mu.Lock()
	f.doc.Profiles = nil
	f.doc.Users = nil
	f.mu.Unlock()

	rec := authGet(t, f.s, "/users")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	// A <select> whose only option is the empty one is a control that cannot express an action.
	// Asserting the absence of the form, not the presence of a sentence, is what makes this fail
	// on a page that added an apology next to the same dead dropdown. Absence is a whole-page
	// claim; the two positive ones are row-scoped, because `href="/profiles"` is in the shell's
	// nav (page.tmpl) on every page in the product, so over the whole body that assertion can
	// never fail -- pointing the paragraph's link at /users left the package green.
	if strings.Contains(body, `name="profile"`) {
		t.Errorf("the page still offers a profile dropdown with nothing in it:\n%s", body)
	}
	row := userRow(t, body, "ada")
	for _, want := range []string{`href="/profiles"`, "no profile to assign"} {
		if !strings.Contains(row, want) {
			t.Errorf("ada's row does not say what is missing or where to fix it (missing %q):\n%s",
				want, row)
		}
	}
}

// The other zero-profile row: an account still in the document whose profile was the one that got
// deleted. Its only valid action IS removal, and taking the dropdown away without offering that
// would strand the row -- the same trap the stale-account branch above already avoids.
//
// Both halves are asserted because the handler half alone is not enough: with the render branch
// `{{if .Configured}}` mutated to `false` -- the exact stranding this test names -- all three
// zero-profile tests stayed green, because a POST never asks what the page offered.
func TestTheUsersPageStillLetsADanglingEntryBeUnmanaged(t *testing.T) {
	f := newAssignFixture(t)
	f.mu.Lock()
	f.doc.Profiles = nil
	f.mu.Unlock()

	// The render half: the row must offer the action, not the apology. Scoped to the row,
	// because the page holds a second account that is correctly told to go make a profile --
	// asserting on the whole body would fail on a page that got both rows right.
	rec := authGet(t, f.s, "/users")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	row := userRow(t, rec.Body.String(), "casey")
	if !strings.Contains(row, "Stop managing") {
		t.Errorf("the dangling entry is offered no action at all:\n%s", row)
	}
	if strings.Contains(row, "no profile to assign") {
		t.Errorf("the dangling entry is told to go make a profile instead of being given its way out:\n%s", row)
	}

	// The handler half, through the form the row rendered rather than a body typed beside it.
	post := postRowForm(t, f, "casey")
	if post.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %s", post.Code, post.Body.String())
	}
	if f.assignment("casey") != nil {
		t.Error("the entry outlived the only action the page could offer for it")
	}
}

// The refusal must name the thing that is missing. The old message was not merely unhelpful here:
// it described stopping management, which is the opposite of the request that produced it.
func TestAnEmptyAssignmentWithNoProfilesNamesTheProfileNotRemoval(t *testing.T) {
	f := newAssignFixture(t)
	f.mu.Lock()
	f.doc.Profiles = nil
	f.mu.Unlock()

	rec := authPost(t, f.s, "/users/ada/profile", "profile=")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(strings.ToLower(body), "stop managing") {
		t.Errorf("the refusal describes the opposite of the attempted action:\n%s", body)
	}
	if !strings.Contains(strings.ToLower(body), "profile") {
		t.Errorf("the refusal does not name what is missing:\n%s", body)
	}
	if f.savesCount() != 0 {
		t.Errorf("the refusal still reached the write seam %d times", f.savesCount())
	}
	if f.assignment("ada") != nil {
		t.Error("the refusal wrote an entry for the account")
	}
}

// userRow cuts one account's <tr> out of the rendered accounts page. Row scope is the point:
// the zero-profile page deliberately renders two different rows -- the dangling entry gets its
// removal form, an unmanaged account gets the "no profile to assign" paragraph -- and an
// assertion over the whole body cannot tell "the wrong row" from "both rows right".
func userRow(t *testing.T, body, name string) string {
	t.Helper()
	marker := `<th scope="row">` + name + `</th>`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("the page has no row for %q:\n%s", name, body)
	}
	start := strings.LastIndex(body[:i], "<tr")
	end := strings.Index(body[i:], "</tr>")
	if start < 0 || end < 0 {
		t.Fatalf("could not bound the row for %q", name)
	}
	return body[start : i+end]
}

// postRowForm posts the form the page ACTUALLY RENDERED for one account's row, and returns the
// response. It exists because a hand-written body proves the handler accepts a request, and
// nothing connects that request to the control the operator was shown. Two mutants inside the
// removal arm survived a POST and a render that were never joined: `value=""` -> `value="p-gone"`
// on the hidden profile field (the button then answers 404 "no profile has that id" -- the row's
// only action, broken) and the CSRF value emptied (the #75 shape: 403, the parent presses the
// button, nothing happens, nothing says why). Both are green when the body is typed next to the
// template and red when it is taken from it.
//
// TestEveryFormOnEveryPageCarriesTheToken cannot cover this cell: the stale arm and the
// zero-profile arm render the same formShape string, so the one pageStates does render satisfies
// reach for the one it does not, and hasToken matches the field NAME, never its value.
func postRowForm(t *testing.T, f *assignFixture, username string) *httptest.ResponseRecorder {
	t.Helper()
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

	action, fields := parseForm(t, body, "/users/"+username+"/profile")
	if fields["csrf"] == "" {
		t.Errorf("the rendered form carries no CSRF token, so the button cannot be pressed at all:\n%s",
			formFragment(body, action))
	}
	// parseForm walks <input> elements and nothing else, so a row that chooses its profile from a
	// <select> would come back with NO profile key at all -- and the handler reads an absent profile
	// as removal. Measured on a machine with profiles, on the dropdown arm: `STATUS 303`, entry
	// `{casey school enabled}` before, `<nil>` after. A test that said "the assignment the operator
	// chose round-trips" would go green while DELETING it, which is the exact shape this helper was
	// written to end -- a payload assembled beside the template instead of out of it, this time
	// assembled by the fix. Refusing is the only pin available, because the defect would live in
	// test code and no mutant reaches there. The dropdown arm is covered by
	// TestTheRenderedAssignmentFormPostsWhatTheHandlerAccepts, which reads the selected option.
	if frag := formFragment(body, action); strings.Contains(frag, "<select") {
		t.Fatalf("postRowForm reads <input> only; this row renders a <select>, so posting it would "+
			"drop the chosen profile and ask for removal. Read the selected option as "+
			"TestTheRenderedAssignmentFormPostsWhatTheHandlerAccepts does:\n%s", frag)
	}
	// The payload is the row's own, including the empty profile that MEANS removal. Rebuilding
	// the body here is what let a non-empty value pass unnoticed.
	q := url.Values{}
	for k, v := range fields {
		q.Set(k, v)
	}

	post := httptest.NewRecorder()
	req := httptest.NewRequest("POST", action, strings.NewReader(q.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// A browser submitting this form is a top-level navigation and sends Accept: text/html; authPost
	// sets it on every write it makes. Without it the helper exercises the programmatic-caller shape,
	// which is the one thing its "send what the page sends" argument does not send.
	req.Header.Set("Accept", "text/html")
	req.AddCookie(c)
	f.s.Handler().ServeHTTP(post, req)
	return post
}
