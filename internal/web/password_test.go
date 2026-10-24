package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/auth"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// The admin-password form (#132). The mechanism it drives -- a rotation ends the sessions minted
// under the old credential -- was already pinned in rotation_test.go; what is under test here is
// the only thing that was missing, which is the way a person reaches it.
//
// Two assertions repeat throughout on purpose, because they are the two this page can get wrong
// silently. One: the stored document, on every path, refusals included -- a rotation that reports
// success without writing is the failure the page exists to prevent. Two: neither password appears
// in the response body, the audit trail, or the log, on any path. A leaked secret does not announce
// itself, and "we do not log passwords" is only a claim until something looks for the bytes.

// The credentials these tests use.
//
// pwCurrent is long enough to clear auth.MinPasswordLen on purpose. It started as "hunter2" and the
// same-as-current case below silently became a length refusal instead: the shape check runs before
// the credential check, so a fixture whose current password is short can never reach the branch
// that compares the two. The fixture mints its own verifier rather than reusing pageVerifier, which
// is why the helpers here log in by hand instead of calling authGet and authPost.
const (
	pwCurrent = "the-current-passphrase"
	pwNext    = "a-longer-passphrase-indeed"
	pwShort   = "short123" // 8 characters: under the floor, and long enough that a length-only rule is what refuses it
)

// pwFixture is a server whose config seam is one in-memory document carrying a real admin
// verifier, so a rotation has somewhere to land and a lost -- or unrequested -- write is visible.
//
// The stored verifier is minted at MinIterations like every other page fixture, for the reason
// pages_test.go gives: a fixture that cannot log in is not testing the page. The verifier this
// handler *writes* is at the production cost, because that is the handler's own choice and a test
// that lowered it would be testing a different handler.
type pwFixture struct {
	s       *Server
	logged  *logSink
	audited *logSink

	mu      sync.Mutex
	doc     *config.Config
	saves   int
	readErr error
}

func (f *pwFixture) load() (*config.Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// readErr makes the seam fail the way a real file fails: the credential exists and the daemon
	// cannot see it, which is a different answer from "wrong password" and must not be dressed as it.
	if f.readErr != nil {
		return nil, f.readErr
	}
	out := *f.doc
	return &out, nil
}

func (f *pwFixture) mutate(fn func(*config.Config) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := fn(f.doc); err != nil {
		return err
	}
	f.saves++
	return nil
}

func (f *pwFixture) savesCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.saves
}

// admin is the stored credential, copied.
func (f *pwFixture) admin() config.Admin {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.doc.Admin
}

// setVerifier replaces the stored credential without going through the handler, which is how a test
// reaches a state the form itself cannot produce: an account whose verifier was cleared, or a file
// that stopped being readable, after somebody was already signed in.
func (f *pwFixture) setVerifier(v string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := *f.doc
	c.Admin.Password = v
	f.doc = &c
}

// profileIDs is how these tests say "nothing else moved": the schedule is the part of the document
// a credential write has no business touching, and a lockout change hiding inside a password change
// shows up here as a string that is no longer "school:1".
func (f *pwFixture) profileIDs() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	for _, p := range f.doc.Profiles {
		ids = append(ids, p.ID+":"+strconv.Itoa(len(p.Rules)))
	}
	return strings.Join(ids, ",")
}

func (f *pwFixture) auditedJoined() string { return strings.Join(f.audited.Lines(), "\n") }

// everythingTheServerSaid is every byte the request produced plus every line the server recorded,
// joined for a substring search. A secret can end up in the page, the trail or the journal, and a
// check that looks in one of the three is a check that passes in the wrong place.
func (f *pwFixture) everythingTheServerSaid(bodies ...string) string {
	parts := append([]string{}, bodies...)
	parts = append(parts, f.audited.Lines()...)
	parts = append(parts, f.logged.Lines()...)
	return strings.Join(parts, "\n")
}

func newPWFixture(t *testing.T) *pwFixture {
	t.Helper()
	s, logged, audited := newTestServer(t, nil)
	m, err := AssetMap(Assets())
	if err != nil {
		t.Fatalf("assets: %v", err)
	}
	s.assets = m
	doc := pageConfig()
	doc.Admin = config.Admin{Username: "parent", Password: mintVerifier(t, pwCurrent)}
	f := &pwFixture{s: s, logged: logged, audited: audited, doc: doc}
	s.Config = f.load
	s.Lockables = func() ([]platform.Session, error) { return nil, nil }
	s.Accounts = func() (*users.Listing, error) {
		return &users.Listing{UIDMin: users.DefaultUIDMin, UIDMax: users.DefaultUIDMax}, nil
	}
	s.Mutate = f.mutate
	return f
}

// signIn signs in for real and returns the session cookie with its CSRF token. The token is looked
// up rather than invented, because a test that made one up would be testing its own string.
func signIn(t *testing.T, s *Server, password string) (*http.Cookie, string) {
	t.Helper()
	resp, token := login(t, s, "parent", password)
	return cookieOf(t, resp), token
}

// postRotate sends the rotation form with the session it is handed, and returns the cookie too:
// several tests below have to keep using that session after the submit, which is the only way to
// say anything about whose session survives.
func postRotate(t *testing.T, s *Server, c *http.Cookie, token, form string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/settings/password", strings.NewReader(form+"&csrf="+token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(c)
	req.RemoteAddr = "192.0.2.71:1"
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// rotate signs in and posts, which is every test that does not care about the session afterwards.
func rotate(t *testing.T, s *Server, form string) (*httptest.ResponseRecorder, *http.Cookie) {
	t.Helper()
	c, token := signIn(t, s, pwCurrent)
	return postRotate(t, s, c, token, form), c
}

func rotateForm(current, next, confirm string) string {
	return "currentPassword=" + current + "&newPassword=" + next + "&confirmPassword=" + confirm
}

// authGetPW is authGet with this fixture's credential, for the two tests that only need a page.
func authGetPW(t *testing.T, f *pwFixture, path string) *httptest.ResponseRecorder {
	t.Helper()
	c, _ := signIn(t, f.s, pwCurrent)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Accept", "text/html")
	req.AddCookie(c)
	f.s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body.String())
	}
	return rec
}

// refusalSentence pulls the sentence the page refused with, out of the page around it.
//
// The whole body cannot be searched for a leaked word: the form on this page is *about* changing a
// password, so it necessarily contains "current password" and "Change password". The claim worth
// testing is narrower and stronger -- the sentence that refuses must be the login page's sentence,
// and must not name a rotation.
// refusalSentence finds the sentence in the page that refused.
//
// A refusal that could not re-render the form -- the one case being when the config itself is
// unreadable -- answers in plain text, so the whole body is the sentence there. Both shapes are
// checked by the same helper rather than one of them being untested.
func refusalSentence(t *testing.T, body string) string {
	t.Helper()
	i := strings.Index(body, `class="problem"`)
	if i < 0 {
		return body
	}
	rest := body[i:]
	end := strings.Index(rest, "</p>")
	if end < 0 {
		t.Fatalf("the problem sentence is unclosed:\n%s", body)
	}
	return rest[:end]
}

func TestTheSettingsPageOffersTheRotationForm(t *testing.T) {
	f := newPWFixture(t)
	body := authGetPW(t, f, "/settings").Body.String()

	for _, want := range []string{
		`action="/settings/password"`,
		`name="currentPassword"`, `name="newPassword"`, `name="confirmPassword"`,
		`type="password"`,
		// The floor and the poll interval, both stated as the server's own numbers.
		`minlength="` + strconv.Itoa(auth.MinPasswordLen) + `"`,
		"At least " + strconv.Itoa(auth.MinPasswordLen) + " characters",
		"within " + DefaultCredentialPoll.String(),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not say %s:\n%s", want, body)
		}
	}

	// No password field is ever rendered with a value. That is the deliberate difference from the
	// interval box above it: that one comes back holding what you typed, and a box that did the same
	// with a secret would put it in the page source, the browser cache and every screenshot.
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, `type="password"`) && strings.Contains(line, "value=") {
			t.Errorf("a password field is rendered with a value: %s", strings.TrimSpace(line))
		}
	}
}

func TestTheCredentialIsItsOwnSubmit(t *testing.T) {
	f := newPWFixture(t)
	body := authGetPW(t, f, "/settings").Body.String()

	settingsAt := strings.Index(body, `action="/settings"`)
	passwordAt := strings.Index(body, `action="/settings/password"`)
	if settingsAt < 0 || passwordAt < 0 || passwordAt < settingsAt {
		t.Fatalf("the page does not carry two separate forms (settings at %d, credential at %d)",
			settingsAt, passwordAt)
	}
	// A field that rides along with the enforcement switch puts the secret in the same body as two
	// non-secret values, which is how a future "log the parsed form" debug line becomes a credential
	// leak -- and it makes the audit line for a rotation able to read like an interval change.
	settingsForm := body[settingsAt:passwordAt]
	if strings.Contains(settingsForm, "assword") {
		t.Errorf("a password field is inside the settings form:\n%s", settingsForm)
	}
}

func TestARotationStoresTheNewCredentialAndNothingElse(t *testing.T) {
	f := newPWFixture(t)
	before := f.admin()
	rec, _ := rotate(t, f.s, rotateForm(pwCurrent, pwNext, pwNext))
	if rec.Code != http.StatusOK {
		t.Fatalf("a correct rotation was refused: %d %s", rec.Code, rec.Body.String())
	}
	if got := f.savesCount(); got != 1 {
		t.Fatalf("the rotation wrote %d times, want exactly 1", got)
	}
	after := f.admin()
	if after.Password == before.Password {
		t.Fatal("the stored verifier did not change")
	}
	h := auth.Hasher{}
	if !h.Verify(pwNext, after.Password) {
		t.Error("the stored verifier does not verify the new password")
	}
	if h.Verify(pwCurrent, after.Password) {
		t.Error("the stored verifier still verifies the old password")
	}
	if after.Username != "parent" {
		t.Errorf("the rotation renamed the account: %q", after.Username)
	}
	if got := f.profileIDs(); got != "school:1" {
		t.Errorf("the schedule moved as a side effect of a credential write: %q", got)
	}
	if after.UpdatedAt.IsZero() {
		t.Error("Admin.UpdatedAt was left at zero by the one event it exists to describe")
	}
	if got := f.auditedJoined(); !strings.Contains(got, auditPasswordRotated) {
		t.Errorf("a completed rotation was not audited:\n%s", got)
	}
}

func TestTheConfirmationSaysWhenTheOldPasswordStopsWorking(t *testing.T) {
	f := newPWFixture(t)
	rec, _ := rotate(t, f.s, rotateForm(pwCurrent, pwNext, pwNext))
	body := rec.Body.String()
	// Two different timings, and the copy has to keep them apart. The password stops being accepted
	// as soon as the file is written, because the login path reads its credential through the seam
	// per attempt (#1092). The *sessions* end on the watcher's next poll. A confirmation that blurred
	// the two would be wrong about the one security property this page provides.
	if !strings.Contains(body, "Every session opened under it") {
		t.Errorf("the confirmation does not say the sessions end:\n%s", body)
	}
	if !strings.Contains(body, "within "+DefaultCredentialPoll.String()) {
		t.Errorf("the confirmation does not name the interval the daemon actually polls on:\n%s", body)
	}
	for _, lie := range []string{"immediately", "instantly", "have been ended", "are now revoked"} {
		if strings.Contains(strings.ToLower(body), lie) {
			t.Errorf("the confirmation claims the revocation took effect %q, which is up to one poll away", lie)
		}
	}
}

func TestAWrongCurrentPasswordIsRefusedWithTheLoginSentence(t *testing.T) {
	f := newPWFixture(t)
	before := f.admin()
	rec, _ := rotate(t, f.s, rotateForm("not-the-password", pwNext, pwNext))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("a wrong current credential returned %d, want 401: %s", rec.Code, rec.Body.String())
	}
	sentence := refusalSentence(t, rec.Body.String())
	if !strings.Contains(sentence, "wrong username or password") {
		t.Errorf("the refusal is not the login page's sentence: %s", sentence)
	}
	// Nothing in the sentence may name a rotation. This page is reachable by anyone who can reach
	// the login page, and a refusal that reads differently from a failed login is a way to enumerate
	// what the machine is listening for (#1140).
	for _, leak := range []string{"rotation", "rotated", "current", "change", "new password"} {
		if strings.Contains(strings.ToLower(sentence), leak) {
			t.Errorf("the refusal sentence names %q, which a failed login does not say: %s", leak, sentence)
		}
	}
	if f.savesCount() != 0 {
		t.Error("a refused rotation wrote the configuration")
	}
	if f.admin().Password != before.Password {
		t.Error("a refused rotation changed the stored credential")
	}
	// The CSP guard enumerates the GET pages, and its own comment names the blind spot: a page that
	// only ever appears as the answer to a POST is never measured. This refusal is a settings page
	// carrying a problem sentence, so the sentence is measured here, against the header the POST
	// actually carried.
	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("the refusal sent no Content-Security-Policy header")
	} else {
		for _, complaint := range cspComplaints(got, rec.Body.String()) {
			t.Errorf("the refusal page: %s", complaint)
		}
	}
	lines := f.auditedJoined()
	if !strings.Contains(lines, "auth.failure") {
		t.Errorf("a wrong credential was not audited as a login failure:\n%s", lines)
	}
	if strings.Contains(lines, auditPasswordRotated) {
		t.Errorf("a refusal was audited as a rotation:\n%s", lines)
	}
}

func TestARefusalNeverCarriesEitherPassword(t *testing.T) {
	// Every path, checked for both secrets -- the success path included, because the confirmation is
	// rendered after the new credential was accepted and is the most likely place for a
	// well-meaning "you set <password>" line.
	cases := []struct{ name, form string }{
		{"wrong current", rotateForm("not-the-password", pwNext, pwNext)},
		{"mismatched", rotateForm(pwCurrent, pwNext, pwNext+"x")},
		{"too short", rotateForm(pwCurrent, pwShort, pwShort)},
		{"same as current", rotateForm(pwCurrent, pwCurrent, pwCurrent)},
		{"empty new", rotateForm(pwCurrent, "", "")},
		{"accepted", rotateForm(pwCurrent, pwNext, pwNext)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPWFixture(t)
			rec, _ := rotate(t, f.s, tc.form)
			got := f.everythingTheServerSaid(rec.Body.String())
			for _, secret := range []string{pwCurrent, pwNext, pwShort, "not-the-password"} {
				if strings.Contains(got, secret) {
					t.Errorf("%q appears in the page, the audit trail or the log:\n%s", secret, got)
				}
			}
		})
	}
}

func TestAMismatchedConfirmationIsRefusedBeforeTheCredentialIsChecked(t *testing.T) {
	f := newPWFixture(t)
	rec := postRotateWithWrongLogin(t, f, rotateForm("not-the-password", pwNext, pwNext+"x"))
	// The sentence is about the two entries, not about the credential. An operator who mistyped the
	// confirmation and also mistyped the current password must not be told the password was wrong --
	// that is a guess about which half they got right, and it is the enumeration the login page
	// refuses to do.
	if !strings.Contains(refusalSentence(t, rec.Body.String()), "do not match") {
		t.Errorf("a mismatched confirmation was not refused as a mismatch:\n%s", rec.Body.String())
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status %d, want 422", rec.Code)
	}
	if strings.Contains(f.auditedJoined(), "auth.failure") {
		t.Error("a mismatched confirmation was charged to the login failure trail; it is not a guess at a credential")
	}
	if f.savesCount() != 0 {
		t.Error("a mismatched confirmation wrote the configuration")
	}
}

// postRotateWithWrongLogin signs in with the *correct* credential and then posts a form whose
// current-password field is wrong, so the shape checks are the thing being measured.
func postRotateWithWrongLogin(t *testing.T, f *pwFixture, form string) *httptest.ResponseRecorder {
	t.Helper()
	c, token := signIn(t, f.s, pwCurrent)
	return postRotate(t, f.s, c, token, form)
}

func TestAShortNewPasswordIsRefusedByTheSameFloorAsTheCLI(t *testing.T) {
	f := newPWFixture(t)
	rec, _ := rotate(t, f.s, rotateForm(pwCurrent, pwShort, pwShort))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a %d-character password returned %d, want 422", len(pwShort), rec.Code)
	}
	sentence := refusalSentence(t, rec.Body.String())
	// The number is the shared constant, stated next to what was typed. If the CLI's floor and this
	// page's floor ever disagree, this is the assertion that notices -- the drift #127 was filed for,
	// arriving in the other direction.
	if !strings.Contains(sentence, strconv.Itoa(auth.MinPasswordLen)) ||
		!strings.Contains(sentence, strconv.Itoa(len(pwShort))+" characters") {
		t.Errorf("the refusal does not state the shared floor and what was typed: %s", sentence)
	}
	if f.savesCount() != 0 {
		t.Error("a refused password was stored anyway")
	}
}

func TestAnEmptyNewPasswordIsRefusedRatherThanHashed(t *testing.T) {
	// The shape checks run before the credential check, so this is also the case where an empty
	// string would otherwise reach the KDF and be stored as a credential. A verifier for "" is a
	// machine whose admin account has no password and a login page that says "wrong username or
	// password" about it.
	f := newPWFixture(t)
	before := f.admin()
	rec, _ := rotate(t, f.s, rotateForm(pwCurrent, "", ""))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("an empty new password returned %d, want 422", rec.Code)
	}
	if !strings.Contains(refusalSentence(t, rec.Body.String()), "all three password fields") {
		t.Errorf("the refusal does not say which fields were missing:\n%s", rec.Body.String())
	}
	if f.savesCount() != 0 {
		t.Error("an empty password was stored")
	}
	if f.admin().Password != before.Password {
		t.Error("an empty password replaced the stored verifier")
	}
	if strings.Contains(f.auditedJoined(), auditPasswordRotated) {
		t.Error("an empty submit was audited as a rotation")
	}
}

func TestThePageNamesThePollIntervalThisServerActuallyRuns(t *testing.T) {
	// The confirmation quotes a number the daemon chose, not a constant the page copied. A server
	// started with a different CredentialPoll -- which Start exposes and tests set -- has to say
	// that number, or the sentence is a promise about a poll that is not happening.
	f := newPWFixture(t)
	f.s.CredentialPoll = time.Second
	body := authGetPW(t, f, "/settings").Body.String()
	if !strings.Contains(body, "within 1s") {
		t.Errorf("the page states an interval other than the one this server polls on:\n%s", body)
	}
	if strings.Contains(body, "within 5s") {
		t.Error("the page states DefaultCredentialPoll over the interval the server was given")
	}
}

func TestRotatingToTheSamePasswordIsRefused(t *testing.T) {
	f := newPWFixture(t)
	c, token := signIn(t, f.s, pwCurrent)
	rec := postRotate(t, f.s, c, token, rotateForm(pwCurrent, pwCurrent, pwCurrent))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("re-setting the same password returned %d, want 422: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(refusalSentence(t, rec.Body.String()), "same as the current one") {
		t.Errorf("the refusal does not say what was wrong:\n%s", rec.Body.String())
	}
	if f.savesCount() != 0 {
		t.Fatal("re-setting the same password wrote a new verifier")
	}
	// The consequence the refusal exists to prevent: a fresh salt makes a re-set of the same
	// password look like a rotation to the watcher, so accepting it would end every session --
	// including this one -- for no change in security.
	if _, ok := f.s.Sessions.Lookup(c.Value); !ok {
		t.Error("the refused rotation ended the operator's session")
	}
}

func TestTheRotationSharesTheLoginThrottle(t *testing.T) {
	f := newPWFixture(t)
	c, token := signIn(t, f.s, pwCurrent)
	// Driven to the edge rather than to a number copied out of auth: the limit is the throttle's
	// business, and a test that hard-codes it fails for the wrong reason the day it is retuned. What
	// matters is that repeated wrong credentials stop being answered at all, and that the refusal
	// arrives before any credential work.
	var throttledAt = -1
	for i := 0; i < 25 && throttledAt < 0; i++ {
		rec := postRotate(t, f.s, c, token, rotateForm("guess-"+strconv.Itoa(i), pwNext, pwNext))
		switch rec.Code {
		case http.StatusUnauthorized:
		case http.StatusTooManyRequests:
			throttledAt = i
			if rec.Header().Get("Retry-After") == "" {
				t.Error("a throttled refusal gives no Retry-After, so the operator cannot be told when to come back")
			}
		default:
			t.Fatalf("attempt %d returned %d, want 401 then 429", i, rec.Code)
		}
	}
	if throttledAt < 1 {
		t.Fatalf("the rotation form was never throttled (first refusal at attempt %d)", throttledAt)
	}
	lines := f.auditedJoined()
	if !strings.Contains(lines, "auth.throttled") {
		t.Errorf("the throttle did not audit:\n%s", lines)
	}
	if strings.Contains(lines, auditPasswordRotated) {
		t.Error("a throttled submit was audited as a rotation")
	}
	if f.savesCount() != 0 {
		t.Error("a throttled submit wrote the configuration")
	}
}

func TestAMachineWithNoCredentialSaysSoRatherThanRefusing(t *testing.T) {
	f := newPWFixture(t)
	c, token := signIn(t, f.s, pwCurrent)
	// The verifier is cleared after the sign-in: a machine whose admin record lost its verifier
	// mid-session is the state this branch answers, and it is not reachable by an operator who
	// cannot log in in the first place.
	f.setVerifier("")
	rec := postRotate(t, f.s, c, token, rotateForm(pwCurrent, pwNext, pwNext))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("a machine with no admin credential returned %d, want 503", rec.Code)
	}
	if !strings.Contains(refusalSentence(t, rec.Body.String()), "no administrator account yet") {
		t.Errorf("the refusal does not name the actual problem:\n%s", rec.Body.String())
	}
	if f.savesCount() != 0 {
		t.Error("a rotation on a machine with no credential wrote a configuration")
	}
}

func TestAConfigThatCannotBeReadIsNotAWrongPassword(t *testing.T) {
	f := newPWFixture(t)
	c, token := signIn(t, f.s, pwCurrent)
	f.readErr = errors.New("read /etc/plock/config.json: permission denied")
	rec := postRotate(t, f.s, c, token, rotateForm(pwCurrent, pwNext, pwNext))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("an unreadable credential returned %d, want 503", rec.Code)
	}
	sentence := refusalSentence(t, rec.Body.String())
	if !strings.Contains(sentence, "could not be read") {
		t.Errorf("the refusal does not say the credential was unreadable: %s", sentence)
	}
	if strings.Contains(sentence, "wrong username or password") {
		t.Error("an unreadable credential was reported as a wrong password, which sends the operator at the wrong file")
	}
	if f.savesCount() != 0 {
		t.Error("a rotation that could not read the credential wrote it")
	}
}

// The property the whole page rests on, asserted against the watcher that provides it rather than
// against a sentence on the page: the sessions minted under the old credential end, and the
// operator's own session ends with them.
func TestTheRotationEndsEverySessionUnderTheOldOneIncludingTheOperators(t *testing.T) {
	f := newPWFixture(t)
	f.s.CredentialPoll = 10 * time.Millisecond
	serveFor(t, f.s)

	theirs, _ := signIn(t, f.s, pwCurrent)
	other, _ := signIn(t, f.s, pwCurrent)
	theirsToo := other
	if _, ok := f.s.Sessions.Lookup(theirs.Value); !ok {
		t.Fatal("the operator has no session; the rest of this test proves nothing")
	}
	token, _ := f.s.csrf.lookup(theirs.Value)

	rec := postRotate(t, f.s, theirs, token, rotateForm(pwCurrent, pwNext, pwNext))
	if rec.Code != http.StatusOK {
		t.Fatalf("the rotation was refused: %d %s", rec.Code, rec.Body.String())
	}

	// Waiting on the log line rather than on the sessions vanishing: the watcher revokes and then
	// logs, so the effect exists before the record. Waiting on the record bounds both halves, which
	// is the reasoning rotation_test.go gives for the same shape.
	if !eventually(func() bool {
		return strings.Contains(strings.Join(f.logged.Lines(), "\n"), "admin credential changed")
	}, 3*time.Second) {
		t.Fatalf("the rotation never ended the sessions under the old credential:\n%s",
			strings.Join(f.logged.Lines(), "\n"))
	}

	// Which rule the page's copy has to describe, stated rather than left to be inferred: the
	// operator's own session dies too. That is the existing revocation rule -- RevokeAll, keyed on
	// the verifier -- and it is the right one: a rotation performed because a session is suspected
	// stolen, which quietly kept the operator's own session alive, would keep the stolen one alive
	// as well. The cost is signing in again, which is why the confirmation says so before the button
	// is ever pressed.
	for _, c := range []*http.Cookie{theirs, theirsToo} {
		if _, ok := f.s.Sessions.Lookup(c.Value); ok {
			t.Error("a session minted under the old credential survived the rotation")
		}
	}

	// And the new credential works while the old one does not, through the login path rather than
	// through the handler's own arithmetic. login itself fails the test if the login fails.
	login(t, f.s, "parent", pwNext)
	rec2 := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", strings.NewReader("username=parent&password="+pwCurrent))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	f.s.Handler().ServeHTTP(rec2, req)
	if rec2.Code == http.StatusSeeOther {
		t.Error("the old password still signs in after the rotation")
	}
}

func TestTheRouteRefusesAnAnonymousSubmit(t *testing.T) {
	// The route-coverage guard enumerates registrations from source and refuses any route not listed
	// as anonymous, so every route including ones added next month is covered. It is asserted
	// directly here too, because this route changes a credential and the guard's answer is a
	// property of a test file rather than of the router.
	f := newPWFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/settings/password",
		strings.NewReader(rotateForm(pwCurrent, pwNext, pwNext)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	f.s.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusOK || rec.Code == http.StatusSeeOther {
		t.Fatalf("an anonymous submit was accepted: %d", rec.Code)
	}
	if f.savesCount() != 0 {
		t.Error("an anonymous submit wrote the credential")
	}
}
