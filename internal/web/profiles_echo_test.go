package web

import (
	"html"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// Every refusal on the profiles page is a page now, not a wall of text: the sentence arrives above
// the form and the form still holds what was typed (#138). Each test below is one form and asserts
// both halves, because the two fail independently -- a page that re-renders with empty boxes is the
// old failure with a nicer wrapper, and a page that puts the values in the sentence is the thing
// this package has refused to do since handleDeleteProfile said so.

// refusalParagraph returns the text of the refusal paragraph, unescaped, or "" when the response
// carries no refusal sentence at all.
//
// It is the profiles-page sibling of password_test.go's refusalSentence and deliberately not the
// same function: that one answers "no problem element" with the whole body, because a refusal that
// could not re-render the form answers in plain text. On this page the body legitimately carries
// request text now, in the boxes, so a test that fell back to the body would fail its own "the
// sentence quotes nothing typed" check for the wrong reason. Here an absent element is reported as
// absent, and the caller says what that means.
//
// The unescape is the other half. The sentence used to be the whole body of a text/plain 422 and
// the tests quoted it with an apostrophe in it; in HTML, html/template turns that apostrophe into
// &#39;, so a test that greps the raw body asserts on an encoding rather than on the sentence a
// parent reads. The sentence is also the only place the "no request text" rule can still be
// checked: the body carries request text now, in the boxes.
//
// It anchors on the paragraph's id rather than on class="problem" because a profile row can carry a
// problem sentence of its own -- the evaluator's degraded report -- and the first one in the
// document would be that one.
func refusalParagraph(body string) string {
	const open = `id="refusal"`
	i := strings.Index(body, open)
	if i < 0 {
		return ""
	}
	rest := body[i:]
	gt := strings.Index(rest, ">")
	if gt < 0 {
		return ""
	}
	rest = rest[gt+1:]
	j := strings.Index(rest, "</p>")
	if j < 0 {
		return ""
	}
	return html.UnescapeString(rest[:j])
}

// inputTag returns the `<input ...>` element with that id, or "" when there is none.
//
// By id rather than by position because the profiles page has four inputs called "name" -- one per
// profile plus the create form's -- and a test that took the first would pass on whichever form
// rendered first.
func inputTag(body, id string) string {
	marker := `id="` + id + `"`
	i := strings.Index(body, marker)
	if i < 0 {
		return ""
	}
	tag := body[i:]
	end := strings.Index(tag, ">")
	if end < 0 {
		return ""
	}
	return tag[:end]
}

// echoedValue returns what the named input carries, unescaped, and whether the input is there.
// Escaping is undone here so the assertion is about what the browser puts in the box, and so a fix
// that stripped tags instead of escaping them shows up as a changed value rather than passing.
func echoedValue(body, id string) (string, bool) {
	tag := inputTag(body, id)
	if tag == "" {
		return "", false
	}
	v := strings.Index(tag, `value="`)
	if v < 0 {
		return "", true
	}
	rest := tag[v+len(`value="`):]
	q := strings.Index(rest, `"`)
	if q < 0 {
		return "", true
	}
	return html.UnescapeString(rest[:q]), true
}

// postRefusal posts a form that is going to be refused and hands back the page and its sentence,
// having first checked the half of the contract that goes missing quietly: a refusal page that
// answers 200 is indistinguishable from an accepted save to anything not reading the prose.
func postRefusal(t *testing.T, s *Server, path, form string) (body, sentence string) {
	t.Helper()
	rec := authPost(t, s, path, form)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body:\n%s", rec.Code, rec.Body.String())
	}
	body = rec.Body.String()
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("a refused write answered %q; the page is supposed to come back, and this is what "+
			"came instead:\n%s", ct, body)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store: a refusal page in the browser's history is a "+
			"form the parent can walk back into", cc)
	}
	sentence = refusalParagraph(body)
	if sentence == "" {
		t.Errorf("the refusal page carries no sentence at %s:\n%s", path, body)
	}
	return body, sentence
}

// TestARenameRefusalPutsTheNameBack is the rename form: the box is the only thing on it, so a
// refusal that loses the text costs the whole submission.
func TestARenameRefusalPutsTheNameBack(t *testing.T) {
	fx := newOverrideFixture(t)
	before := fx.savesCount()
	// The fixture holds one profile, so the refusal this form can be given is the ceiling rather
	// than a collision. It is spelled with q's so the assertion below cannot pass by accident.
	typed := strings.Repeat("q", config.MaxNameLen+1)

	body, sentence := postRefusal(t, fx.s, "/profiles/school/rename", "name="+typed)
	if got, ok := echoedValue(body, "rename-school"); !ok {
		t.Error("the rename box is gone from the refusal page")
	} else if got != typed {
		t.Errorf("the rename box holds %d characters, want the %d that were typed back (#138)",
			len(got), len(typed))
	}
	if strings.Contains(sentence, "qqqq") {
		t.Errorf("the refusal sentence quotes what was typed: %q", sentence)
	}
	if n := fx.savesCount() - before; n != 0 {
		t.Errorf("a refused rename still saved %d documents", n)
	}
}

// TestARenameOntoAnotherProfilesNamePutsTheNameBack is this route's collision refusal, the sibling
// of TestACopyRefusalPutsTheNameBack two forms down. The test above does not cover it: that one
// drives the length ceiling, which is a different call site on the same route, so the route could
// echo one refusal and answer the other in plain text. That is the exact state #138 exists to end,
// and W279 had to hollow this call site out before the suite noticed it was still there.
func TestARenameOntoAnotherProfilesNamePutsTheNameBack(t *testing.T) {
	fx := newOverrideFixture(t)
	if rec := authPost(t, fx.s, "/profiles", "name=Kids&timezone=UTC"); rec.Code != http.StatusSeeOther {
		t.Fatalf("the second profile was not created: %d %q", rec.Code, rec.Body.String())
	}
	before := fx.savesCount()

	// A collision rather than the ceiling, because this is the refusal whose fix is to change the
	// word in the box. The ceiling refusal cannot show that: the typed name is 61 q's there, and
	// putting it back is a detail, not the way out.
	body, sentence := postRefusal(t, fx.s, "/profiles/school/rename", "name=Kids")
	if got, ok := echoedValue(body, "rename-school"); !ok {
		t.Error("the rename box is gone from the refusal page")
	} else if got != "Kids" {
		t.Errorf("the rename box holds %q, want the typed name back (#138)", got)
	}
	// The shared predicate's own sentence. It carries no name, so this also pins the rule that
	// request text stays out of the prose now that it has somewhere else to go.
	if !strings.Contains(sentence, "already has that name") {
		t.Errorf("the refusal does not say what collided: %q", sentence)
	}
	if strings.Contains(sentence, "Kids") {
		t.Errorf("the refusal sentence quotes what was typed: %q", sentence)
	}
	if n := fx.savesCount() - before; n != 0 {
		t.Errorf("a refused rename still saved %d documents", n)
	}
}

// TestATimezoneRefusalPutsTheZoneBack is the form whose handler used to say the typed value would
// deliberately not be echoed. #138 kept the sentence free of request text and moved the value into
// the box, so the parent corrects the zone they looked up instead of re-looking it up.
func TestATimezoneRefusalPutsTheZoneBack(t *testing.T) {
	fx := newOverrideFixture(t)
	before := fx.savesCount()

	body, sentence := postRefusal(t, fx.s, "/profiles/school/timezone",
		"timezone=Mars%2FOlympus_Mons")
	if got, ok := echoedValue(body, "tz-school"); !ok {
		t.Error("the timezone box is gone from the refusal page")
	} else if got != "Mars/Olympus_Mons" {
		t.Errorf("the timezone box holds %q, want the typed zone back (#138)", got)
	}
	if strings.Contains(sentence, "Olympus") {
		t.Errorf("the refusal sentence quotes what was typed: %q", sentence)
	}
	// Everything that is not a box is still the machine. The row's own zone is what its meta line
	// says, or the refusal page would describe a profile that does not exist.
	if !strings.Contains(body, "timezone <code>UTC</code>") {
		t.Errorf("the refusal page no longer shows the zone the profile actually runs in:\n%s", body)
	}
	if n := fx.savesCount() - before; n != 0 {
		t.Errorf("a refused timezone still saved %d documents", n)
	}
}

// TestARuleRefusalPutsEveryFieldBack is the worst case in the package and the reason #138 is one
// issue rather than five: a refusal about one clock used to cost a name, seven checkboxes, two
// times and a box, all of it re-entered from memory on a 360 px screen.
func TestARuleRefusalPutsEveryFieldBack(t *testing.T) {
	fx := newOverrideFixture(t)
	before := fx.savesCount()

	body, _ := postRefusal(t, fx.s, "/profiles/school/rules",
		"name=School+night&days=mon&days=fri&start=21%3A00&end=notaclock")
	for _, want := range []struct{ id, value string }{
		{"name-school", "School night"},
		{"start-school", "21:00"},
		{"end-school", "notaclock"},
	} {
		if got, ok := echoedValue(body, want.id); !ok {
			t.Errorf("the %q box is gone from the refusal page", want.id)
		} else if got != want.value {
			t.Errorf("the %q box holds %q, want %q (#138)", want.id, got, want.value)
		}
	}
	// The checkboxes are the part no `value` attribute covers, and the part a fix that only
	// handled text inputs would silently drop.
	for _, day := range []string{"mon", "fri"} {
		if !strings.Contains(inputTag(body, "days-"+day+"-school"), "checked") {
			t.Errorf("the %q day box came back unticked:\n%s", day, body)
		}
	}
	for _, day := range []string{"tue", "wed"} {
		if strings.Contains(inputTag(body, "days-"+day+"-school"), "checked") {
			t.Errorf("the %q day box came back ticked, and it was never ticked", day)
		}
	}
	// The eighth box on the same form, asserted the same way. It can only fail by adding
	// something: a Checked() that asks "did a rule form come back refused" instead of "did this
	// form say all-day" ticks the box on every rule refusal, so a parent who never touched it is
	// told back that the window runs all day -- on the one page where they are deciding what the
	// window does. TestARuleRefusalKeepsTheAllDayBox cannot see it, because that one submits
	// allDay=on, and a plain GET cannot either, because there is no echoed form to ask.
	if tag := inputTag(body, "allday-school"); strings.Contains(tag, "checked") {
		t.Errorf("the all-day box ticked itself on a form that never ticked it; tag was %q", tag)
	}
	if n := fx.savesCount() - before; n != 0 {
		t.Errorf("a refused rule still saved %d documents", n)
	}
}

// TestARuleRefusalKeepsTheAllDayBox is the same form's fourth field, separated because a checkbox's
// value is not a value: the box carries `value="on"` whether ticked or not, so the only thing that
// can come back is `checked`.
func TestARuleRefusalKeepsTheAllDayBox(t *testing.T) {
	fx := newOverrideFixture(t)

	body, _ := postRefusal(t, fx.s, "/profiles/school/rules", "name=&days=mon&allDay=on")
	if tag := inputTag(body, "allday-school"); !strings.Contains(tag, "checked") {
		t.Errorf("the all-day box came back unticked; tag was %q", tag)
	}
}

// TestACreateRefusalPutsBothBoxesBack: the create form is two fields, and a refusal about the zone
// must not cost the name.
func TestACreateRefusalPutsBothBoxesBack(t *testing.T) {
	fx := newOverrideFixture(t)
	before := fx.savesCount()

	body, sentence := postRefusal(t, fx.s, "/profiles", "name=Weekend+rules&timezone=Local")
	if got, ok := echoedValue(body, "new-profile-name"); !ok || got != "Weekend rules" {
		t.Errorf("the create name box holds %q (present=%v), want the typed name back", got, ok)
	}
	if got, ok := echoedValue(body, "new-profile-tz"); !ok || got != "Local" {
		t.Errorf("the create timezone box holds %q (present=%v), want the typed zone back", got, ok)
	}
	if strings.Contains(sentence, "Weekend") {
		t.Errorf("the refusal sentence quotes what was typed: %q", sentence)
	}
	if n := fx.savesCount() - before; n != 0 {
		t.Errorf("a refused create still saved %d documents", n)
	}
}

// TestACopyRefusalPutsTheNameBack: on the copy form the discarded field is the only field, which is
// what made #135 sharpen a bug the page had always had.
func TestACopyRefusalPutsTheNameBack(t *testing.T) {
	fx := newOverrideFixture(t)
	before := fx.savesCount()

	// The fixture's one profile is called "School nights", so this is the collision refusal -- the
	// one where keeping the text is worth something, because the fix is to edit the name in the box
	// rather than remember it and retype it.
	body, sentence := postRefusal(t, fx.s, "/profiles/school/copy", "name=School+nights")
	if got, ok := echoedValue(body, "copy-school"); !ok || got != "School nights" {
		t.Errorf("the copy box holds %q (present=%v), want the typed name back (#138)", got, ok)
	}
	if strings.Contains(sentence, "School nights") {
		t.Errorf("the refusal sentence quotes what was typed: %q", sentence)
	}
	if n := fx.savesCount() - before; n != 0 {
		t.Errorf("a refused copy still saved %d documents", n)
	}
}

// TestAnEchoedValueCannotEscapeItsAttribute is the objection formTimezone used to answer with,
// taken seriously rather than waved at: the value now crosses from the request into the response,
// so the escaping is the whole reason that is allowed.
//
// It asserts the negative in the shape that would actually hurt -- the quote and angle brackets are
// gone from what the browser parses -- and the positive in the shape that proves the parent still
// sees their own text, because a fix that HTML-strips the value rather than escaping it would pass
// a "no <script> in the body" check while quietly eating what was typed.
func TestAnEchoedValueCannotEscapeItsAttribute(t *testing.T) {
	fx := newOverrideFixture(t)
	// Repeated past the ceiling so the form is actually refused: a name that would have been
	// accepted would redirect, and the escaping claim would be about a page that was never built.
	typed := strings.Repeat(`"><b>bold</b>`, config.MaxNameLen/10+1)

	body, sentence := postRefusal(t, fx.s, "/profiles/school/rename",
		"name="+url.QueryEscape(typed))
	if strings.Contains(body, `<b>bold</b>`) {
		t.Error("the echoed name closed its attribute and opened markup of its own")
	}
	if strings.Contains(body, `value="`+typed+`"`) {
		t.Error("the echoed name reached the page unescaped")
	}
	if got, ok := echoedValue(body, "rename-school"); !ok || got != typed {
		t.Errorf("the box holds %q (present=%v); escaping must not eat what was typed", got, ok)
	}
	if strings.Contains(sentence, "bold") {
		t.Errorf("the refusal sentence carries request text: %q", sentence)
	}
}

// TestARefusalPageStillShowsTheRealProfiles is the other half of the split renderSettingsRefusal
// makes: the boxes come from the request and every row comes from the document. A page built out of
// the submission would tell a parent a profile exists that was never written.
func TestARefusalPageStillShowsTheRealProfiles(t *testing.T) {
	fx := newOverrideFixture(t)

	body, _ := postRefusal(t, fx.s, "/profiles", "name=Brand+new&timezone=Local")
	if got, want := strings.Count(body, `<section class="profile">`), profileCount(t, fx); got != want {
		t.Errorf("the refusal page renders %d profile sections, want the %d the document holds: the "+
			"refused submission has been rendered as if it had been written", got, want)
	}
	if strings.Contains(body, "<h2>Brand new</h2>") {
		t.Error("the refused create appears on the page as a profile heading")
	}
}

// profileCount reads the document the fixture is serving, so the assertion above is against the
// machine rather than a number this file would have to keep current.
func profileCount(t *testing.T, fx *overrideFixture) int {
	t.Helper()
	cfg, err := fx.s.readConfig()
	if err != nil {
		t.Fatalf("the fixture's configuration could not be read: %v", err)
	}
	return len(cfg.Profiles)
}

// TestANormalLoadCarriesNoRefusal is the control. A page that printed the sentence on every load, or
// filled every box from somewhere, would pass every test above.
func TestANormalLoadCarriesNoRefusal(t *testing.T) {
	fx := newOverrideFixture(t)
	rec := authGet(t, fx.s, "/profiles")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /profiles = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if s := refusalParagraph(body); s != "" {
		t.Errorf("a normal load carries a refusal sentence: %q", s)
	}
	for _, want := range []struct{ id, value string }{
		// The two boxes the document does fill, filled from the document.
		{"rename-school", "School nights"}, {"tz-school", "UTC"},
		// The three the document says nothing about, empty rather than invented.
		{"copy-school", ""}, {"new-profile-name", ""}, {"new-profile-tz", ""},
	} {
		if got, ok := echoedValue(body, want.id); !ok {
			t.Errorf("%q is missing from a plain GET /profiles", want.id)
		} else if got != want.value {
			t.Errorf("%q carries %q on a page nobody submitted anything to, want %q",
				want.id, got, want.value)
		}
	}
	for _, day := range []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"} {
		if tag := inputTag(body, "days-"+day+"-school"); strings.Contains(tag, "checked") {
			t.Errorf("the %q day box is ticked on a page nobody submitted anything to", day)
		}
	}
	// The eighth box, for the same reason. It does not catch W281 -- on a plain GET there is no
	// echoed form, so Checked() takes its nil arm and the mutant changes nothing here -- and it is
	// still the assertion this page owes: an all-day window nobody asked for is wrong on every
	// render, and this is the only render that proves the page starts clean.
	if tag := inputTag(body, "allday-school"); strings.Contains(tag, "checked") {
		t.Errorf("the all-day box is ticked on a page nobody submitted anything to")
	}
}

// TestASaveFailureIsStillNotAForm pins the half of the seam this PR deliberately did not move.
//
// The echo is for refusals the operator can act on: the form came back because what was typed is
// the problem. A seam failure is not that, and answering it with the form would be worse than the
// plain error page -- it implies the typed text caused a disk failure, and invites the parent to
// correct a field that was never the problem and submit again while the mount is still read-only.
// Nothing else in the package reaches that branch: the 503 tests drive the delete route, which
// carries no refuser, so the five routes that now do could silently start routing 5xx through the
// echo and stay green. W284 is that mutation.
func TestASaveFailureIsStillNotAForm(t *testing.T) {
	fx := newOverrideFixture(t)
	fx.mu.Lock()
	fx.failOn = fs.ErrPermission
	fx.mu.Unlock()

	rec := authPost(t, fx.s, "/profiles/school/rename", "name=Renamed+again")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body %.200q", rec.Code, rec.Body.String())
	}
	if s := refusalParagraph(rec.Body.String()); s != "" {
		t.Errorf("a server failure was answered with a refusal page carrying %q", s)
	}
	// The typed name must not be sitting in a box on the error page either: that is the same
	// mistake with the paragraph removed.
	if got, ok := echoedValue(rec.Body.String(), "rename-school"); ok {
		t.Errorf("the error page carries a form holding %q; a seam failure is not a form problem", got)
	}
}
