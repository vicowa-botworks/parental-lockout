package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// The settings page is a write path, so it is tested the way the repo tests write paths and not
// the way it tests pages: every acceptance test here asserts the stored document, and every
// refusal test asserts that nothing was stored and nothing was audited.
//
// The asymmetry is the reason. A page bug shows the operator a wrong screen. A settings bug
// silently flips the one control whose whole job is whether children get locked out, and the
// screen keeps saying the thing that is no longer true.

// settingsFixture is a server whose config seam is backed by one in-memory document, so a save
// has somewhere to land and a lost write -- or an unrequested one -- is observable.
type settingsFixture struct {
	s       *Server
	audited *logSink

	mu    sync.Mutex
	doc   *config.Config
	saves int
}

func (f *settingsFixture) load() (*config.Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := *f.doc
	return &out, nil
}

func (f *settingsFixture) mutate(fn func(*config.Config) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := fn(f.doc); err != nil {
		return err
	}
	f.saves++
	return nil
}

func (f *settingsFixture) savesCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.saves
}

// settings is the document's current settings, copied so the test cannot mutate the fixture's
// own value through the returned struct.
func (f *settingsFixture) settings() config.Settings {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.doc.Settings
}

// set puts the document in a known position before a submit, so a test never depends on which
// default config.Default happens to ship.
func (f *settingsFixture) set(enforcement bool, poll int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.doc.Settings.Enforcement = enforcement
	f.doc.Settings.PollSeconds = poll
}

func newSettingsFixture(t *testing.T) *settingsFixture {
	t.Helper()
	s, _, audited := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	m, err := AssetMap(Assets())
	if err != nil {
		t.Fatalf("assets: %v", err)
	}
	s.assets = m
	f := &settingsFixture{s: s, audited: audited, doc: pageConfig()}
	s.Config = f.load
	s.Lockables = func() ([]platform.Session, error) { return nil, nil }
	s.Accounts = func() (*users.Listing, error) {
		return &users.Listing{UIDMin: users.DefaultUIDMin, UIDMax: users.DefaultUIDMax}, nil
	}
	s.Mutate = f.mutate
	return f
}

// auditLines returns the trail lines the server wrote.
func (f *settingsFixture) auditLines() []string { return f.audited.Lines() }

// settingsAuditLines narrows the sink to this page's own event.
//
// The narrowing matters for the refusal tests: a login writes auth.success into the same sink, so
// "no lines at all" would fail for a reason that has nothing to do with the submit under test.
func (f *settingsFixture) settingsAuditLines() []string {
	var out []string
	for _, l := range f.audited.Lines() {
		if strings.HasPrefix(l, "settings.changed") {
			out = append(out, l)
		}
	}
	return out
}

func TestTheSettingsPageShowsBothStoredValues(t *testing.T) {
	f := newSettingsFixture(t)
	f.set(true, 45)

	body := authGet(t, f.s, "/settings").Body.String()
	if !strings.Contains(body, `value="45"`) {
		t.Errorf("the interval box does not hold the stored value; want an input whose value is 45.\n%s",
			body)
	}
	// The checked position is the page's only answer to "where is the switch now", and it has
	// to be an attribute a browser acts on rather than prose a parent has to cross-reference.
	if !strings.Contains(body, `value="on" required`) || !strings.Contains(body, "checked") {
		t.Errorf("the enforcement control does not mark the stored position as checked.\n%s", body)
	}
	// The bounds are stated from config's constants, so a page that drifted from validate.go
	// would be caught here rather than in a browser.
	if !strings.Contains(body, `min="`+strconv.Itoa(config.MinPollSeconds)+`"`) ||
		!strings.Contains(body, `max="`+strconv.Itoa(config.MaxPollSeconds)+`"`) {
		t.Errorf("the interval box does not carry config's own bounds of %d..%d.\n%s",
			config.MinPollSeconds, config.MaxPollSeconds, body)
	}
}

func TestTheSettingsPageNamesWhatSwitchingEnforcementOffMeans(t *testing.T) {
	f := newSettingsFixture(t)
	f.set(false, 30)

	body := authGet(t, f.s, "/settings").Body.String()
	// The page has to say what the state does, not merely name it: "off" beside a list of
	// profiles reads like a filter to a parent who has never seen the config file.
	for _, want := range []string{"nothing is being locked out", "whatever the schedule says"} {
		if !strings.Contains(body, want) {
			t.Errorf("with enforcement off the page never says %q, so the fail-open state is only "+
				"named and never explained.\n%s", want, body)
		}
	}
}

func TestAnInvalidIntervalIsRefusedAndTheFormComesBackWithWhatWasTyped(t *testing.T) {
	f := newSettingsFixture(t)
	f.set(true, 30)

	rec := authPost(t, f.s, "/settings", "enforcement=on&pollSeconds=2s")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("a non-numeric interval answered %d, want 422: the operator caused this and the "+
			"server cannot retry it", rec.Code)
	}
	body := rec.Body.String()
	// The form has to still be there. A plain-text refusal sends the parent back through the
	// navigation to retype a number they were only asked to correct.
	if !strings.Contains(body, `<form`) || !strings.Contains(body, `action="/settings"`) {
		t.Errorf("the refusal is not the form; the operator has to navigate back to correct it.\n%s", body)
	}
	if !strings.Contains(body, `value="2s"`) {
		t.Errorf("the box does not hold the text that was typed, so the refusal reads as being "+
			"about something else.\n%s", body)
	}
	if !strings.Contains(body, "settings.pollSeconds") {
		t.Errorf("the refusal does not name the field.\n%s", body)
	}
	if got := f.settings(); got.PollSeconds != 30 || !got.Enforcement {
		t.Errorf("a refused submit changed the document to %+v", got)
	}
	if n := f.savesCount(); n != 0 {
		t.Errorf("a refused submit saved %d times; the config file was renamed for nothing", n)
	}
	if lines := f.settingsAuditLines(); len(lines) != 0 {
		t.Errorf("a refused submit wrote %v; the trail is for things that happened", lines)
	}
}

func TestAnOutOfRangeIntervalIsRefusedRatherThanCorrected(t *testing.T) {
	// Zero is the case that matters. config.normalized() turns a stored 0 into the default of
	// its own accord on the next load, so a form that accepted 0 would save a number the daemon
	// silently rewrites -- the exact "silent default" the ticket refuses. The handler's range
	// check is what stops 0 ever reaching the file.
	for _, raw := range []string{"0", "-5", "1", strconv.Itoa(config.MaxPollSeconds + 1)} {
		f := newSettingsFixture(t)
		f.set(true, 30)

		rec := authPost(t, f.s, "/settings", "enforcement=on&pollSeconds="+raw)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("pollSeconds=%s answered %d, want a refusal", raw, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "settings.pollSeconds") {
			t.Errorf("pollSeconds=%s was refused without naming the field.\n%s", raw, rec.Body.String())
		}
		if got := f.settings(); got.PollSeconds != 30 {
			t.Errorf("pollSeconds=%s changed the stored interval to %d", raw, got.PollSeconds)
		}
		if lines := f.settingsAuditLines(); len(lines) != 0 {
			t.Errorf("pollSeconds=%s was refused and still wrote %v", raw, lines)
		}
	}
}

func TestAMissingEnforcementFieldIsRefusedRatherThanReadAsOff(t *testing.T) {
	// Every other checkbox in this UI is written so that absence is the safe direction. This
	// control is the reverse, so a request that does not carry it has to be a refusal: the
	// alternative reading is "stop locking children out", and this project does not make that
	// call from a missing field (#873).
	f := newSettingsFixture(t)
	f.set(true, 30)

	rec := authPost(t, f.s, "/settings", "pollSeconds=60")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("a submit with no enforcement field answered %d, want a refusal rather than a "+
			"guessed position", rec.Code)
	}
	if got := f.settings(); got.Enforcement != true {
		t.Errorf("a submit with no enforcement field left enforcement %v; the missing field was "+
			"read as a decision to stop enforcing", got.Enforcement)
	}
	if got := f.settings(); got.PollSeconds != 30 {
		t.Errorf("a submit refused for its switch still wrote the interval to %d; a refusal that "+
			"partially applies is worse than one that applies nowhere", got.PollSeconds)
	}
	if n := f.savesCount(); n != 0 {
		t.Errorf("a refused submit saved %d times", n)
	}
	if lines := f.settingsAuditLines(); len(lines) != 0 {
		t.Errorf("a refused submit wrote %v", lines)
	}
}

func TestAnUnrecognisedEnforcementValueIsRefused(t *testing.T) {
	f := newSettingsFixture(t)
	f.set(true, 30)

	rec := authPost(t, f.s, "/settings", "enforcement=maybe&pollSeconds=60")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("enforcement=maybe answered %d, want a refusal", rec.Code)
	}
	if got := f.settings(); got.Enforcement != true || got.PollSeconds != 30 {
		t.Errorf("a refused submit left the document at %+v", got)
	}
}

func TestAnAcceptedSubmitWritesBothFieldsAndAuditsThem(t *testing.T) {
	f := newSettingsFixture(t)
	f.set(true, 30)

	rec := authPost(t, f.s, "/settings", "enforcement=off&pollSeconds=60")
	if rec.Code != http.StatusOK {
		t.Errorf("an accepted submit answered %d, want the page carrying its confirmation.\n%s",
			rec.Code, rec.Body.String())
	}
	got := f.settings()
	if got.Enforcement || got.PollSeconds != 60 {
		t.Errorf("the document holds %+v, want enforcement off and pollSeconds 60", got)
	}
	if n := f.savesCount(); n != 1 {
		t.Errorf("an accepted submit saved %d times, want exactly 1", n)
	}

	lines := f.settingsAuditLines()
	if len(lines) != 1 {
		t.Fatalf("an accepted submit wrote %d trail lines, want exactly 1: %v", len(lines), lines)
	}
	// Both ends of both changes, and the who. "Who turned enforcement off at 2am" is the
	// question the trail gets read to answer, and a line saying only "settings changed" cannot
	// answer it after the next save overwrites the value.
	line := lines[0]
	for _, want := range []string{"settings.changed", "enforcement on to off",
		"pollSeconds 30 to 60", "by parent"} {
		if !strings.Contains(line, want) {
			t.Errorf("the trail line %q never says %q", line, want)
		}
	}
}

// statusParagraph returns the text of the page's role="status" element -- the confirmation slot --
// or "" when the page has none.
//
// Scoping to the element is what makes the confirmation tests mean anything. The page also renders
// a banner describing the stored state, and after a submit that turns enforcement off the banner
// says "nothing is being locked out" on its own; a test that searched the whole body would pass on
// the banner and never notice the confirmation had been reduced to "Settings saved".
func statusParagraph(body string) string {
	return betweenMarkers(body, `role="status">`, "</p>")
}

// settingsForm returns the markup of the form that posts to /settings, or "" when the page has
// none.
//
// Same reason as statusParagraph: the shared shell already carries a CSRF field for the sign-out
// form, so "the body contains this session's token" is true of every page in the app whether or not
// the settings form can be submitted at all.
func settingsForm(body string) string {
	return betweenMarkers(body, `<form method="post" action="/settings"`, "</form>")
}

func betweenMarkers(body, open, close string) string {
	i := strings.Index(body, open)
	if i < 0 {
		return ""
	}
	rest := body[i+len(open):]
	j := strings.Index(rest, close)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

func TestTheConfirmationNamesWhatStopsHappening(t *testing.T) {
	f := newSettingsFixture(t)
	f.set(true, 30)

	body := authPost(t, f.s, "/settings", "enforcement=off&pollSeconds=30").Body.String()
	said := statusParagraph(body)
	if said == "" {
		t.Fatalf("the accepted submit rendered no status paragraph at all.\n%s", body)
	}
	for _, want := range []string{"nothing is being locked out", "whatever the schedule says",
		"switched back on"} {
		if !strings.Contains(said, want) {
			t.Errorf("the confirmation says %q without %q, which leaves a fail-open control "+
				"reading like a routine edit. The banner is not the confirmation: it describes the "+
				"stored state and would say this even if the submit had been refused.", said, want)
		}
	}
	// The other direction has to be just as specific, or the page teaches the parent that the
	// switch is the thing that locks a screen this second.
	f.set(false, 30)
	said = statusParagraph(authPost(t, f.s, "/settings", "enforcement=on&pollSeconds=30").Body.String())
	for _, want := range []string{"Enforcement is on", "next tick"} {
		if !strings.Contains(said, want) {
			t.Errorf("switching enforcement on confirmed (%q) without saying %q", said, want)
		}
	}
}

func TestASubmitThatChangesNothingWritesNothing(t *testing.T) {
	f := newSettingsFixture(t)
	f.set(true, 30)

	rec := authPost(t, f.s, "/settings", "enforcement=on&pollSeconds=30")
	if rec.Code != http.StatusOK {
		t.Errorf("a no-op submit answered %d, want the page saying so.\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Nothing changed") {
		t.Errorf("a no-op submit did not say so; silence here reads as a save that worked.\n%s",
			rec.Body.String())
	}
	if n := f.savesCount(); n != 0 {
		t.Errorf("a submit that changed nothing renamed the config file %d times", n)
	}
	if lines := f.settingsAuditLines(); len(lines) != 0 {
		t.Errorf("a submit that changed nothing wrote %v; the trail would claim a change that "+
			"never happened", lines)
	}
}

func TestOnlyTheIntervalChangeIsReportedAsAnIntervalChange(t *testing.T) {
	// The confirmation picks its clause from whether the switch moved, so a submit that only
	// retimed the tick must not print a warning about enforcement. A page that told a parent
	// "enforcement is switched off" after they changed 30 to 60 would be believed, and would be
	// lying.
	f := newSettingsFixture(t)
	f.set(true, 30)

	said := statusParagraph(authPost(t, f.s, "/settings", "enforcement=on&pollSeconds=60").Body.String())
	if strings.Contains(said, "Enforcement is switched off") {
		t.Errorf("an interval-only submit warned about enforcement being off: %q", said)
	}
	if !strings.Contains(said, "pollSeconds 30 to 60") {
		t.Errorf("an interval-only submit did not report the change it made: %q", said)
	}
}

// TestTheSettingsRoutesAreRegistered pins the two registrations by name.
//
// The route-coverage guard cannot do this job for them: it enumerates what is registered and asks
// whether each route is refused, so a route nobody registered is simply absent from its list and
// nothing is missing. That is the shape #59 shipped in -- every page's own tests passing about a
// route the served mux did not have -- and the only check that sees it is one that names the route
// and asks whether it is there.
func TestTheSettingsRoutesAreRegistered(t *testing.T) {
	want := map[string]bool{"GET /settings": false, "POST /settings": false}
	for _, d := range registeredRoutes(t) {
		if _, ok := want[d.pattern]; ok {
			want[d.pattern] = true
		}
	}
	for pattern, found := range want {
		if !found {
			t.Errorf("%s is not registered, so the page is a link to a 404", pattern)
		}
	}
}

// The refusal page and the confirmation page are both rendered by a POST, so the page-by-page CSP
// loop in pages_test.go cannot reach them: that loop can only see what a GET renders. This is the
// sibling call that comment asks for, and without it a style="" attribute added to settings.tmpl
// would ship with the guard green.
func TestTheSettingsPagesRenderedFromAPostHaveNothingTheCSPWouldBlock(t *testing.T) {
	f := newSettingsFixture(t)
	f.set(true, 30)
	s := f.s

	refusal := authPost(t, s, "/settings", "enforcement=on&pollSeconds=2s")
	for _, complaint := range cspComplaints(refusal.Header().Get("Content-Security-Policy"),
		refusal.Body.String()) {
		t.Errorf("the refused submit on /settings: %s", complaint)
	}

	accepted := authPost(t, s, "/settings", "enforcement=off&pollSeconds=60")
	for _, complaint := range cspComplaints(accepted.Header().Get("Content-Security-Policy"),
		accepted.Body.String()) {
		t.Errorf("the accepted submit on /settings: %s", complaint)
	}
}

func TestTheSettingsFormCarriesTheSessionToken(t *testing.T) {
	// The shipped form is what the acceptance capture posts, and what a parent's browser posts.
	// A form without the field is a page whose every submit fails as a CSRF error, which is #59's
	// dead-seam defect arriving as a working-looking screen.
	f := newSettingsFixture(t)
	// One login, and the same cookie used to look the token up and to fetch the page. authGet
	// signs in again for its own request, and a second session mints a second token -- comparing
	// the page against the first one would be asserting that two different sessions disagree.
	resp, _ := login(t, f.s, "parent", "hunter2")
	c := cookieOf(t, resp)
	token, ok := f.s.csrf.lookup(c.Value)
	if !ok || token == "" {
		t.Fatal("the login minted no CSRF token")
	}

	req := httptest.NewRequest("GET", "/settings", nil)
	req.Header.Set("Accept", "text/html")
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(rec, req)
	form := settingsForm(rec.Body.String())
	if form == "" {
		t.Fatalf("the page renders no form posting to /settings.\n%s", rec.Body.String())
	}
	if !strings.Contains(form, `name="csrf" value="`+token+`"`) {
		t.Errorf("the settings form does not carry this session's token; a submit from the page as "+
			"rendered would be refused. The form as rendered:\n%s", form)
	}
}

func TestAnAbsentEnforcementKeyStillRendersAsOn(t *testing.T) {
	// The document on disk is allowed to omit `enforcement` entirely, and the rule is that an
	// omitted key means on (#872, #873). The page is where that rule becomes visible to a human:
	// if the handler read the Go zero value instead of the decoded default, the page would say
	// "enforcement is switched off" about a machine that is enforcing, and the parent would then
	// "fix" it by switching it off twice.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body := `{"version":1,"admin":{"username":"parent","password":"x"},"settings":{"pollSeconds":30}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.Settings.Enforcement {
		t.Fatal("the fixture document decoded an absent enforcement key as off")
	}
	page := settingsView(cfg)
	if !page.Enforcement {
		t.Errorf("the page built from a document with no enforcement key says enforcement is off")
	}
}

func TestTheSettingsPageIsReachableFromTheNavigation(t *testing.T) {
	// A page nothing links to is a page that does not exist to anyone who has not read the
	// source. This is the same claim the dashboard's lock button gets pinned with, for the same
	// reason: the route works whether or not anything on screen reaches it.
	f := newSettingsFixture(t)
	body := authGet(t, f.s, "/").Body.String()
	if !strings.Contains(body, `href="/settings"`) {
		t.Errorf("the status page never links to /settings.\n%s", body)
	}
}
