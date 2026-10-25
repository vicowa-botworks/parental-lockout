package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// #135: making a profile from the browser.
//
// The page could already rename, re-zone, add windows to and delete a profile, and there was
// still no way to make one -- so a fresh machine rendered "There are no profiles yet" above
// nothing to click, and a second child on a different schedule needed a root shell.
//
// What these tests are built around is the one way this write could be quietly destructive.
// Every profile write here stores through config.UpsertProfile, which REPLACES whatever already
// carries the id it is handed. A create form that took an id would therefore be an overwrite
// path wearing a create button: one keystroke matching an existing id would replace that
// profile's windows and silently re-decide every account pointing at it, and the response would
// still look like a success. The id is derived for that reason, and
// TestACreateNeverReplacesTheProfileThatAlreadyHasTheID is the test that says so -- it is
// deliberately constructed so the name check passes and only the id would collide.

// TestACreatedProfileIsInTheDocumentAndAudited is the happy path, read back through the same
// findProfile the handlers use rather than through a field a test assembled itself.
func TestACreatedProfileIsInTheDocumentAndAudited(t *testing.T) {
	f := newOverrideFixture(t)
	before := f.savesCount()

	rec := authPost(t, f.s, "/profiles", "name=Weekend+rules&timezone=Europe%2FParis")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}

	p, ok := profileFrom(t, f, "weekend-rules")
	if !ok {
		t.Fatalf("no profile was created under the derived id; document %+v", f.doc.Profiles)
	}
	if p.Name != "Weekend rules" {
		t.Errorf("name = %q, want Weekend rules", p.Name)
	}
	if p.Timezone != "Europe/Paris" {
		t.Errorf("timezone = %q, want Europe/Paris", p.Timezone)
	}
	// Empty is the point, not an omission: the page says a profile with no windows locks
	// nobody out, so a create that silently copied something would be the surprise.
	if len(p.Rules) != 0 {
		t.Errorf("a new profile carries %d windows, want none", len(p.Rules))
	}
	if n := f.savesCount() - before; n != 1 {
		t.Errorf("the write saved %d documents, want exactly one", n)
	}

	audit := strings.Join(f.audited.Lines(), "\n")
	if !strings.Contains(audit, "profile.created") ||
		!strings.Contains(audit, "weekend-rules") ||
		!strings.Contains(audit, "Europe/Paris") {
		t.Errorf("the audit trail does not name the profile it made:\n%s", audit)
	}
	if !strings.Contains(audit, "parent") {
		t.Errorf("the audit line does not name who made it:\n%s", audit)
	}
}

// TestACreateNeverReplacesTheProfileThatAlreadyHasTheID is the acceptance criterion, and the
// fixture is chosen so nothing else would catch it.
//
// The existing profile is id "school", name "School nights". Posting the name "school" is not a
// name collision -- ProfileNameTaken compares names, and "school" is not "School nights" -- so
// the only thing standing between this request and a destroyed schedule is that the id is
// derived rather than taken from the field. Without freshProfileID this test does not fail loudly:
// it is the two assertions on the ORIGINAL profile that fail, because the handler would have
// replaced it and the new one would look perfectly healthy.
func TestACreateNeverReplacesTheProfileThatAlreadyHasTheID(t *testing.T) {
	f := newOverrideFixture(t)

	rec := authPost(t, f.s, "/profiles", "name=school&timezone=UTC")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}

	school, ok := profileFrom(t, f, "school")
	if !ok {
		t.Fatal("the profile that already had the id is gone -- the create replaced it")
	}
	if school.Name != "School nights" {
		t.Errorf("the existing profile's name became %q, want School nights", school.Name)
	}
	if len(school.Rules) != 1 || school.Rules[0].ID != "r1" {
		t.Errorf("the existing profile's windows changed: %+v", school.Rules)
	}

	fresh, ok := profileFrom(t, f, "school-2")
	if !ok {
		t.Fatalf("the create did not step aside to a free id; document %+v", f.doc.Profiles)
	}
	if fresh.Name != "school" {
		t.Errorf("name = %q, want school", fresh.Name)
	}

	// And the account that points at the original still resolves to it. This is the child whose
	// bedtime would have moved without anyone touching it.
	cfg := snapshot(t, f)
	for _, u := range cfg.Users {
		if u.Username == "casey" && u.ProfileID != "school" {
			t.Errorf("casey was moved onto profile %q", u.ProfileID)
		}
	}
}

func TestACreateRefusesANameAnotherProfileAlreadyWears(t *testing.T) {
	f := newOverrideFixture(t)
	if rec := authPost(t, f.s, "/profiles", "name=Kids&timezone=UTC"); rec.Code != http.StatusSeeOther {
		t.Fatalf("the first create failed: %d %q", rec.Code, rec.Body.String())
	}
	before := f.savesCount()

	rec := authPost(t, f.s, "/profiles", "name=Kids&timezone=Europe%2FParis")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
	}
	if n := f.savesCount() - before; n != 0 {
		t.Errorf("a refused create still saved %d documents", n)
	}
	// The shared predicate's own sentence, not a system-fault wording: a collision is something
	// the operator can fix by typing a different word.
	body := rec.Body.String()
	if !strings.Contains(body, "already has that name") {
		t.Errorf("the refusal does not say what collided:\n%s", body)
	}
}

// TestACreateRefusesEveryTimezoneTheTimezoneFormRefuses is the test for the sharing, not for the
// checks. Both handlers now go through formTimezone, so the interesting failure is a create that
// accepted something the rename path refuses -- two lists of what a zone may be, drifting one
// value at a time, which is the drift the helper exists to prevent.
func TestACreateRefusesEveryTimezoneTheTimezoneFormRefuses(t *testing.T) {
	cases := []struct {
		name, zone string
		want       string
	}{
		{"blank", "", "a blank one is read as UTC"},
		{"unloadable", "Mars/Olympus_Mons", "not a timezone this machine can load"},
		{"Local", "Local", "it follows whatever the machine's clock is"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newOverrideFixture(t)
			before := f.savesCount()

			rec := authPost(t, f.s, "/profiles", "name=Any+name&timezone="+c.zone)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), c.want) {
				t.Errorf("the refusal does not say why %q is no good:\n%s", c.zone, rec.Body.String())
			}
			if n := f.savesCount() - before; n != 0 {
				t.Errorf("a refused create still saved %d documents", n)
			}
		})
	}
}

// TestEveryNameFormRefusesANameTheDocumentCannotHold is the form-level half of #137.
//
// config.Validate owns the ceiling, so a request that gets past this check is still refused before
// anything is written -- which is why this is about the sentence rather than about the rule. A
// parent who typed 400 characters into a box needs to be told the box has a limit, not that
// "profiles[0].name" is overlong in a document they have never seen.
//
// All four forms are asked, because the ceiling is one number and #127 is the reason: a rule the
// page enforces while another path walks past it is worse than no rule, because the half that
// exists reads as coverage. The rename form is the one that could otherwise slip through, since it
// predates the create form and had its own checks written first.
//
// The second loop is the control. A check that refused every name would pass the first loop
// perfectly, and "the ceiling is 200" and "the ceiling is 0" are indistinguishable from the
// refusals alone -- only the accepted case tells them apart.
func TestEveryNameFormRefusesANameTheDocumentCannotHold(t *testing.T) {
	over := strings.Repeat("n", config.MaxNameLen+1)
	justFits := strings.Repeat("n", config.MaxNameLen)

	// The suffix is appended, not Sprintf'd into a template. A first version carried the whole
	// body as a format string and the percent-encoding in `start=20%3A00` was read as a verb, so
	// the clock fields arrived empty and the rule form refused for a reason that had nothing to
	// do with the name. URL-encoded text and a format string do not belong in the same argument.
	forms := []struct{ name, path, suffix string }{
		{"create", "/profiles", "&timezone=UTC"},
		{"rename", "/profiles/school/rename", ""},
		{"copy", "/profiles/school/copy", ""},
		{"add a window", "/profiles/school/rules", "&start=20%3A00&end=06%3A00&days=mon"},
	}

	for _, f := range forms {
		t.Run(f.name+" refuses", func(t *testing.T) {
			fx := newOverrideFixture(t)
			before := fx.savesCount()

			rec := authPost(t, fx.s, f.path, "name="+over+f.suffix)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "too long") {
				t.Errorf("the refusal does not say the name is too long:\n%s", rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), over) {
				t.Error("the refusal quotes the oversized name back into the response body")
			}
			if n := fx.savesCount() - before; n != 0 {
				t.Errorf("a refused write still saved %d documents", n)
			}
		})
	}

	for _, f := range forms {
		t.Run(f.name+" accepts one character less", func(t *testing.T) {
			fx := newOverrideFixture(t)

			rec := authPost(t, fx.s, f.path, "name="+justFits+f.suffix)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("a name of exactly %d characters was refused with %d; body:\n%s",
					config.MaxNameLen, rec.Code, rec.Body.String())
			}
		})
	}

	// The ceiling in characters, at the form. `justFits` is pure ASCII, so runes and bytes agree
	// on it and this loop cannot tell the two implementations apart -- which would leave a form
	// that counts bytes shipping green while refusing every accented name a household writes.
	// Same length, different byte count, and config.Validate already accepts it: the form must
	// not be stricter than the document it feeds.
	t.Run("rename accepts the same length in two-byte characters", func(t *testing.T) {
		fx := newOverrideFixture(t)
		accented := strings.Repeat("é", config.MaxNameLen)

		rec := authPost(t, fx.s, "/profiles/school/rename", "name="+accented)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("a %d-character name was refused with %d; the form is counting bytes "+
				"(%d of them) rather than characters. body:\n%s",
				config.MaxNameLen, rec.Code, len(accented), rec.Body.String())
		}
	})
}

func TestACreateRefusesAnEmptyName(t *testing.T) {
	f := newOverrideFixture(t)
	before := f.savesCount()

	rec := authPost(t, f.s, "/profiles", "name=++&timezone=UTC")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
	}
	if n := f.savesCount() - before; n != 0 {
		t.Errorf("a refused create still saved %d documents", n)
	}
}

// TestACopyCarriesTheWindowsAndTheZoneAndNoAccounts is the reason the copy exists: a second child
// on the same hours, without re-typing them.
func TestACopyCarriesTheWindowsAndTheZoneAndNoAccounts(t *testing.T) {
	f := newOverrideFixture(t)
	// The source is moved off UTC first. The fixture's profile is already UTC, and a copy that
	// hardcoded a zone would look exactly right against it -- the zone only proves it travelled
	// when the two profiles can be told apart.
	if rec := authPost(t, f.s, "/profiles/school/timezone", "timezone=Europe%2FParis"); rec.Code != http.StatusSeeOther {
		t.Fatalf("re-zoning the source failed: %d %q", rec.Code, rec.Body.String())
	}
	// Counted after the setup write, not before it. Counting first made this assert one save and
	// observe two, and the mutation runner's baseline guard refused the whole battery over it --
	// which is the only reason it was caught before it was quoted.
	before := f.savesCount()

	rec := authPost(t, f.s, "/profiles/school/copy", "name=Other+child")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}

	p, ok := profileFrom(t, f, "other-child")
	if !ok {
		t.Fatalf("no copy was created; document %+v", f.doc.Profiles)
	}
	if len(p.Rules) != 1 || p.Rules[0].ID != "r1" || p.Rules[0].Start != "21:00" {
		t.Errorf("the copy did not carry the source's windows: %+v", p.Rules)
	}
	// The zone travels with the rules on purpose. A rule is a wall-clock time, so a copy that
	// took a different zone while carrying the same text would move every window by the offset
	// and look identical to the profile it came from.
	if p.Timezone != "Europe/Paris" {
		t.Errorf("the copy's timezone = %q, want the source's Europe/Paris", p.Timezone)
	}
	if p.ID == "school" || p.Name != "Other child" {
		t.Errorf("the copy kept the source's identity: %+v", p)
	}

	cfg := snapshot(t, f)
	for _, u := range cfg.Users {
		if u.ProfileID == "other-child" {
			t.Errorf("%s was moved onto the copy; a copy starts with nobody", u.Username)
		}
	}
	if n := f.savesCount() - before; n != 1 {
		t.Errorf("the write saved %d documents, want exactly one", n)
	}
	audit := strings.Join(f.audited.Lines(), "\n")
	if !strings.Contains(audit, "profile.copied") || !strings.Contains(audit, "copied from school") {
		t.Errorf("the audit trail does not name the profile the copy came from:\n%s", audit)
	}
}

func TestACopyRefusesAProfileThatDoesNotExist(t *testing.T) {
	f := newOverrideFixture(t)
	before := f.savesCount()

	rec := authPost(t, f.s, "/profiles/no-such-profile/copy", "name=Ghost")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %q", rec.Code, rec.Body.String())
	}
	if n := f.savesCount() - before; n != 0 {
		t.Errorf("a refused copy still saved %d documents", n)
	}
}

func TestACopyRefusesANameThatIsTakenOrBlank(t *testing.T) {
	for _, c := range []struct{ name, form, want string }{
		{"taken", "name=School+nights", "already has that name"},
		{"blank", "name=++", "the copy needs its own name"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newOverrideFixture(t)
			before := f.savesCount()

			rec := authPost(t, f.s, "/profiles/school/copy", c.form)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), c.want) {
				t.Errorf("the refusal does not say what is wrong:\n%s", rec.Body.String())
			}
			if n := f.savesCount() - before; n != 0 {
				t.Errorf("a refused copy still saved %d documents", n)
			}
		})
	}
}

// TestACopyDoesNotShareTheSourcesRuleBackingArray is #881 seen from the new write.
//
// The copy is built with append([]config.Rule(nil), src.Rules...), and the failure mode of
// getting that wrong is invisible at the moment of copying: both profiles would look right, and
// the damage appears one write later, when handleAddRule appends into a backing array two
// profiles share. So the assertion is made after a later write, and it is made on a snapshot
// taken before the copy -- which is what the enforcement loop holds across a tick.
func TestACopyDoesNotShareTheSourcesRuleBackingArray(t *testing.T) {
	f := newOverrideFixture(t)
	held := snapshot(t, f)
	heldSchool := held.Profiles[0]

	if rec := authPost(t, f.s, "/profiles/school/copy", "name=Other+child"); rec.Code != http.StatusSeeOther {
		t.Fatalf("the copy failed: %d %q", rec.Code, rec.Body.String())
	}
	if rec := authPost(t, f.s, "/profiles/other-child/rules",
		"name=Saturday&days=sat&start=09:00&end=18:00"); rec.Code != http.StatusSeeOther {
		t.Fatalf("adding a window to the copy failed: %d %q", rec.Code, rec.Body.String())
	}

	if got := len(heldSchool.Rules); got != 1 {
		t.Errorf("the snapshot the loop was holding now reports %d windows, want 1", got)
	}
	src, _ := profileFrom(t, f, "school")
	if len(src.Rules) != 1 {
		t.Errorf("the source profile gained the copy's window: %+v", src.Rules)
	}
	dst, _ := profileFrom(t, f, "other-child")
	if len(dst.Rules) != 2 {
		t.Errorf("the copy has %d windows, want its own 2", len(dst.Rules))
	}
}

// TestTheProfilesPageOffersAWayToCreateTheFirstOne is the criterion the whole issue is about,
// stated as what a parent sees on a machine with nothing configured.
//
// The create form is outside the {{range .Profiles}} block for exactly this reason, and a test
// that rendered the page with one profile in it would pass whether the form were inside or
// outside -- so this one empties the document.
func TestTheProfilesPageOffersAWayToCreateTheFirstOne(t *testing.T) {
	f := newOverrideFixture(t)
	f.mu.Lock()
	f.doc.Profiles = nil
	f.doc.Users = nil
	f.mu.Unlock()

	rec := authGet(t, f.s, "/profiles")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `action="/profiles"`) {
		t.Errorf("a machine with no profiles has no way to make one:\n%s", body)
	}
	for _, field := range []string{`name="name"`, `name="timezone"`, `name="csrf"`} {
		if !strings.Contains(body, field) {
			t.Errorf("the create form is missing %s", field)
		}
	}
	// The banner still tells the truth about what an empty machine enforces.
	if !strings.Contains(body, "no account can be locked out") {
		t.Errorf("the empty-state banner does not say what is enforced:\n%s", body)
	}
}

// TestTheCopyFormSaysWhatTheCopyStartsWith is the "what is enforced now vs not" criterion for the
// copy. The number and the zone come from the row, so the sentence cannot drift from the profile
// it is describing -- a fixed string would still render "the same windows" beside a profile that
// has none.
//
// It reads the count at one, at several and at none because the version this replaced rendered
// "the same 1 windows", and a test asserted that string -- which is how a wrong sentence becomes
// a requirement. The count is the fixture's, not the sentence's.
// copySentence returns the one sentence under test, not the page it sits on.
//
// The assertions this replaces searched the whole body, and a count is a substring of other
// counts: "0 windows" is the tail of "10 windows" and "1 windows" the tail of "11 windows". A
// fixture with ten windows on `school` therefore made the zero case fail on correct output -- the
// page legitimately said "the same 10 windows" and the test read that as a bare zero. A negative
// assertion over a page is a claim about every number on it, which is not what any of these rules
// are about. Anchoring on the leading space would have worked while the count always follows
// "the same ", and would have broken the first time the sentence moved.
func copySentence(t *testing.T, body, id string) string {
	t.Helper()
	form := `action="/profiles/` + id + `/copy"`
	at := strings.Index(body, form)
	if at < 0 {
		t.Fatalf("no copy form for %q, so there is no sentence to read:\n%s", id, body)
	}
	start := strings.Index(body[at:], `<p class="muted">`)
	if start < 0 {
		t.Fatalf("the copy form for %q has no description paragraph", id)
	}
	start += at
	end := strings.Index(body[start:], `</p>`)
	if end < 0 {
		t.Fatalf("the copy description for %q is never closed", id)
	}
	return strings.Join(strings.Fields(body[start:start+end]), " ")
}

func TestTheCopyFormSaysWhatTheCopyStartsWith(t *testing.T) {
	f := newOverrideFixture(t)

	body := authGet(t, f.s, "/profiles").Body.String()
	if !strings.Contains(body, `action="/profiles/school/copy"`) {
		t.Fatalf("no copy form on the profile's own section:\n%s", body)
	}
	sent := copySentence(t, body, "school")
	if !strings.Contains(sent, "the same 1 window,") {
		t.Errorf("the copy form does not say how many windows come across: %q", sent)
	}
	if strings.Contains(sent, "1 windows") {
		t.Errorf("the copy form pluralises one: %q", sent)
	}
	if !strings.Contains(sent, "no account assigned") {
		t.Errorf("the copy form implies the child is covered, when a copy starts with nobody: %q",
			sent)
	}

	if rec := authPost(t, f.s, "/profiles/school/rules",
		"name=Second&days=sat&start=09:00&end=18:00"); rec.Code != http.StatusSeeOther {
		t.Fatalf("adding the second window failed: %d %q", rec.Code, rec.Body.String())
	}
	body = authGet(t, f.s, "/profiles").Body.String()
	if sent = copySentence(t, body, "school"); !strings.Contains(sent, "the same 2 windows,") {
		t.Errorf("the copy form did not count the second window: %q", sent)
	}

	// An empty profile is the case a bare count cannot say anything about: "the same 0 windows"
	// reads as done, and the table above it already spells the same state as a sentence.
	if rec := authPost(t, f.s, "/profiles", "name=Brand+new&timezone=UTC"); rec.Code != http.StatusSeeOther {
		t.Fatalf("the create failed: %d %q", rec.Code, rec.Body.String())
	}
	body = authGet(t, f.s, "/profiles").Body.String()
	sent = copySentence(t, body, "brand-new")
	if !strings.Contains(sent, "starts with no windows,") {
		t.Errorf("the empty profile's copy sentence counts zero instead of saying none: %q", sent)
	}
	if strings.Contains(sent, "0 windows") {
		t.Errorf("a bare zero is rendered in the empty profile's sentence: %q", sent)
	}
	// The two sentences are read separately on purpose, and this is the assertion that says why:
	// a profile holding a count whose tail is a digit the other profile must not print has to be
	// able to sit on the same page without implicating it.
	if school := copySentence(t, body, "school"); !strings.Contains(school, "the same 2 windows,") {
		t.Errorf("the populated profile's sentence changed while the empty one was being read: %q",
			school)
	}
}

// TestAFreshProfileIsListedAsLockingNobodyOut keeps the confirmation honest where a redirect
// would have been decorative. The page is the confirmation, so the row has to say what the new
// profile does -- which, until a window and an account are added, is nothing.
func TestAFreshProfileIsListedAsLockingNobodyOut(t *testing.T) {
	f := newOverrideFixture(t)
	if rec := authPost(t, f.s, "/profiles", "name=Brand+new&timezone=UTC"); rec.Code != http.StatusSeeOther {
		t.Fatalf("the create failed: %d %q", rec.Code, rec.Body.String())
	}

	body := authGet(t, f.s, "/profiles").Body.String()
	if !strings.Contains(body, "No windows yet, so this profile locks nobody out.") {
		t.Errorf("an empty profile is not described as empty:\n%s", body)
	}
	if !strings.Contains(body, "No account is assigned to this profile.") {
		t.Errorf("an unassigned profile is not described as unassigned:\n%s", body)
	}
}

// TestTheDerivedIDFallsBackWhenTheSlugIsAllPunctuation covers the slug's empty result. A name of
// nothing but punctuation slugs to "", and an empty id is a document Validate refuses -- so the
// fallback is what keeps a whimsical name from being a failed write.
func TestTheDerivedIDFallsBackWhenTheSlugIsAllPunctuation(t *testing.T) {
	f := newOverrideFixture(t)

	rec := authPost(t, f.s, "/profiles", "name=%21%40%23&timezone=UTC")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}
	p, ok := profileFrom(t, f, "profile")
	if !ok {
		t.Fatalf("no profile under the fallback id; document %+v", f.doc.Profiles)
	}
	if p.ID == "" {
		t.Error("a profile was stored with an empty id")
	}
	if got := config.ProfileNameTaken(f.doc.Profiles, "!", ""); got {
		t.Error("the punctuation name collides with something it should not")
	}
}
