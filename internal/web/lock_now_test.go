package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// Locking a session from the dashboard, and the standard its output is held to.
//
// The engine already refuses to say "locked" when all it did was ask (issue #67), and the
// CLI prints three verdicts for the same reason. A web page is a worse place to lose that
// distinction than a terminal, because a terminal scrolls away and a page is what the
// operator quotes to someone else: "it said locked" has to mean a backend proved the screen
// went down. So every test here asserts on which of the three sentences was rendered, not on
// the status code alone -- all three verdicts are 200, and only the words separate them.
//
// The refusals are tested against the seam as well as the page. A 404 that still locked the
// session would be invisible in the body.

// lockFixture is a server whose lock seam records what it was asked to do, so "the page
// asked for the right session" and "the page asked at all" are both observable rather than
// inferred from a status code.
type lockFixture struct {
	s       *Server
	logged  *logSink
	audited *logSink

	mu          sync.Mutex
	asked       []string // session IDs handed to the seam, in order
	lockErr     error
	sessions    []platform.Session
	sessionsErr error
}

func newLockFixture(t *testing.T) *lockFixture {
	t.Helper()
	s, logged, audited := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	m, err := AssetMap(Assets())
	if err != nil {
		t.Fatalf("assets: %v", err)
	}
	s.assets = m
	f := &lockFixture{
		s: s, logged: logged, audited: audited,
		// One session on seat0, which is what a desktop child looks like: class=user, a
		// seat, and a real session type, because IsLockable refuses anything else and a
		// fixture it refuses would have every test below asserting on the refusal instead
		// of on the lock.
		sessions: []platform.Session{{ID: "2", Username: "casey", UID: 1000,
			Class: "user", Seat: "seat0", Type: "x11", State: "active"}},
	}
	s.Config = func() (*config.Config, error) { return pageConfig(), nil }
	s.Lockables = func() ([]platform.Session, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.sessions, f.sessionsErr
	}
	s.Accounts = func() (*users.Listing, error) {
		return &users.Listing{UIDMin: users.DefaultUIDMin, UIDMax: users.DefaultUIDMax}, nil
	}
	s.Lock = f.lock
	return f
}

func (f *lockFixture) lock(_ context.Context, sess platform.Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, sess.ID)
	return f.lockErr
}

func (f *lockFixture) askedFor() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.asked...)
}

func (f *lockFixture) setSessions(s []platform.Session) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions = s
}

func (f *lockFixture) setLockErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lockErr = err
}

// TestTheDashboardOffersToLockAManagedAccount is the control for the whole feature. A handler
// registered on a route no page links to is a feature that does not exist, and this package
// has already shipped one of those once (#59).
func TestTheDashboardOffersToLockAManagedAccount(t *testing.T) {
	f := newLockFixture(t)
	rec := authGet(t, f.s, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard: %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `action="/users/casey/lock-now"`) {
		t.Error("the dashboard has no form that posts to /users/casey/lock-now")
	}
	// The gate reads the token out of the body, so a form without one is a button that
	// cannot work no matter what the handler does.
	if !strings.Contains(body, `name="csrf"`) {
		t.Error("the lock form carries no csrf field")
	}
	// And the form needs the control itself. A form with an action, a token and nothing to
	// press is a row that looks handled and cannot be -- the same shipped-dead shape as a
	// route nobody registered, one layer closer to the operator.
	if !strings.Contains(body, `type="submit">Lock now</button>`) {
		t.Error("the lock form has no submit control, so nothing can post it")
	}
}

// TestAnUnauthenticatedLockNowIsRefusedWithoutTouchingTheSeam. The CSRF middleware wraps the
// mux rather than sitting in each handler, which is why a new POST route is covered the
// moment it is registered -- but "covered by construction" is exactly the kind of claim that
// has to be shown on the route it is claimed for, because a route registered above the gate
// is silently unguarded.
func TestAnUnauthenticatedLockNowIsRefusedWithoutTouchingTheSeam(t *testing.T) {
	f := newLockFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/users/casey/lock-now", nil)
	req.Header.Set("Accept", "text/html")
	req.RemoteAddr = "192.0.2.71:1"
	f.s.Handler().ServeHTTP(rec, req)

	if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || loc != "/login" {
		t.Errorf("an anonymous lock-now answered %d Location=%q, want a redirect to /login",
			rec.Code, loc)
	}
	if got := f.askedFor(); len(got) != 0 {
		t.Errorf("the anonymous request locked %v; a screen is not something to lock on a guess", got)
	}
}

// TestAForgedLockNowFormIsRefusedWithoutTouchingTheSeam posts a real session's cookie with a
// token the server never minted.
func TestAForgedLockNowFormIsRefusedWithoutTouchingTheSeam(t *testing.T) {
	f := newLockFixture(t)
	resp, _ := login(t, f.s, "parent", "hunter2")
	c := cookieOf(t, resp)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/users/casey/lock-now", strings.NewReader("csrf=not-a-token"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(c)
	req.RemoteAddr = "192.0.2.71:1"
	f.s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("forged token answered %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got := f.askedFor(); len(got) != 0 {
		t.Errorf("a forged form locked %v", got)
	}
}

// TestLockingAnUnmanagedAccountIsRefused is the positive control for the name check: the same
// request shape is accepted one line later, against the account the config does manage.
func TestLockingAnUnmanagedAccountIsRefused(t *testing.T) {
	f := newLockFixture(t)
	rec := authPost(t, f.s, "/users/mallory/lock-now", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("an unmanaged account answered %d, want %d", rec.Code, http.StatusNotFound)
	}
	if !strings.Contains(rec.Body.String(), "no managed account") {
		t.Errorf("the refusal does not say what was refused: %s", rec.Body.String())
	}
	if got := f.askedFor(); len(got) != 0 {
		t.Errorf("locking an unmanaged account reached the seam with %v", got)
	}

	// The same request for the managed account is not refused, so the check above cannot be
	// a handler that refuses everything.
	if rec := authPost(t, f.s, "/users/casey/lock-now", ""); rec.Code != http.StatusOK {
		t.Errorf("the managed account answered %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestLockingAnAccountWithNoSessionSaysSo is the case the page must not guess about. "No
// session" is a fact about the machine at this instant, and the operator is entitled to know
// the request never reached a desktop rather than to conclude the lock silently failed.
func TestLockingAnAccountWithNoSessionSaysSo(t *testing.T) {
	f := newLockFixture(t)
	f.setSessions(nil)
	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	if rec.Code != http.StatusConflict {
		t.Errorf("an account with no session answered %d, want %d", rec.Code, http.StatusConflict)
	}
	if !strings.Contains(rec.Body.String(), "has no session on this machine") {
		t.Errorf("the refusal does not say there was nothing to lock: %s", rec.Body.String())
	}
	if got := f.askedFor(); len(got) != 0 {
		t.Errorf("an account with no session reached the seam with %v", got)
	}
}

// TestAFailedSessionListingIsNotReportedAsNoSession is the fail-open shape on a write surface.
// "I could not look" and "there is nothing there" are different claims, and the second one is
// a reassurance the daemon has not earned.
func TestAFailedSessionListingIsNotReportedAsNoSession(t *testing.T) {
	f := newLockFixture(t)
	f.mu.Lock()
	f.sessionsErr = errors.New("loginctl: signal: killed")
	f.mu.Unlock()

	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("a failed session listing answered %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "session table could not be read") {
		t.Errorf("the page did not say the listing failed: %s", body)
	}
	if strings.Contains(body, "has no session") {
		t.Error("a listing that failed was rendered as an account with no session")
	}
}

// TestARefusedLockWritesNoAuditLine is #100's third criterion: every accepted action writes a
// line, a refused one writes none. The trail is what an operator reads afterwards as proof
// that a lock was asked for, so a refusal that logs is a record claiming an action that never
// happened -- and it is the same class of lie as the page saying "locked" for a lock that was
// only asked for, because both make the trail wider than the events.
//
// Every refusal path is walked, not one: a handler with a single logging point above the
// refusals passes a one-case test while recording requests it never carried out.
func TestARefusedLockWritesNoAuditLine(t *testing.T) {
	// The sink also carries the sign-in that authPost performs, and the CSRF layer's own
	// line, so the assertion is about this action's lines rather than about the sink being
	// empty -- a check that failed for an unrelated line would be abandoned rather than fixed.
	lockLines := func(f *lockFixture) []string {
		var out []string
		for _, l := range f.audited.Lines() {
			if strings.HasPrefix(l, "lock.") {
				out = append(out, l)
			}
		}
		return out
	}
	cases := []struct {
		name string
		path string
		prep func(f *lockFixture)
	}{
		{"an unmanaged account", "/users/mallory/lock-now", nil},
		{"an account with no session", "/users/casey/lock-now",
			func(f *lockFixture) { f.setSessions(nil) }},
		{"a listing that failed", "/users/casey/lock-now", func(f *lockFixture) {
			f.mu.Lock()
			f.sessionsErr = errors.New("loginctl: signal: killed")
			f.mu.Unlock()
		}},
	}
	for _, tc := range cases {
		f := newLockFixture(t)
		if tc.prep != nil {
			tc.prep(f)
		}
		authPost(t, f.s, tc.path, "")
		if lines := lockLines(f); len(lines) != 0 {
			t.Errorf("%s wrote an audit line: %v", tc.name, lines)
		}
	}

	// The anonymous and forged cases go through the middleware rather than the handler, so
	// they are posted the way an attacker posts them and checked the same way.
	f := newLockFixture(t)
	resp, _ := login(t, f.s, "parent", "hunter2")
	for _, req := range []*http.Request{
		httptest.NewRequest("POST", "/users/casey/lock-now", nil),
		func() *http.Request {
			r := httptest.NewRequest("POST", "/users/casey/lock-now",
				strings.NewReader("csrf=not-a-token"))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.AddCookie(cookieOf(t, resp))
			return r
		}(),
	} {
		rec := httptest.NewRecorder()
		req.Header.Set("Accept", "text/html")
		req.RemoteAddr = "192.0.2.71:1"
		f.s.Handler().ServeHTTP(rec, req)
		if lines := lockLines(f); len(lines) != 0 {
			t.Errorf("%s %s wrote an audit line: %v", req.Method, req.URL, lines)
		}
	}

	// The control: the accepted request does write, so none of the above passed because
	// nothing is ever audited.
	f2 := newLockFixture(t)
	authPost(t, f2.s, "/users/casey/lock-now", "")
	if lines := lockLines(f2); len(lines) == 0 {
		t.Error("the accepted request wrote no audit line, so the refusals above prove nothing")
	}
}

// TestAProvedLockIsReportedAsLocked: a nil from the seam means a backend confirmed the lock,
// and only then may the page use the word "locked" without a qualifier.
func TestAProvedLockIsReportedAsLocked(t *testing.T) {
	f := newLockFixture(t)
	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("a proved lock answered %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<strong>casey</strong> is locked") {
		t.Errorf("a proved lock did not say so: %s", body)
	}
	if strings.Contains(body, "unconfirmed") {
		t.Error("a proved lock mentioned being unconfirmed")
	}
	if got := f.askedFor(); len(got) != 1 || got[0] != "2" {
		t.Errorf("the seam was asked for sessions %v, want exactly [2]", got)
	}
}

// TestAnUnconfirmedLockIsReportedAsUnconfirmed is the sentence this feature exists to make
// possible. A raw virtual terminal answers this way forever, and a page that rounded it to
// either success or failure would teach the operator to distrust the only signal the machine
// has.
func TestAnUnconfirmedLockIsReportedAsUnconfirmed(t *testing.T) {
	f := newLockFixture(t)
	f.setLockErr(platform.ErrUnconfirmed)
	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("an unconfirmed lock answered %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "unconfirmed") {
		t.Errorf("an unconfirmed lock did not say so: %s", body)
	}
	if strings.Contains(body, "<strong>casey</strong> is locked") {
		t.Error("an unconfirmed lock was reported as locked")
	}
	// Not merely a different word: the page has to say what the operator should do, because
	// "unconfirmed" alone is a word, not an instruction.
	if !strings.Contains(body, "nothing could prove") {
		t.Error("the page did not explain what unconfirmed means")
	}
	// And the word is only half of it. The cell's class carries the colour, which is what the
	// eye reads before the sentence: --held is the colour that means "proved". A row wearing
	// it while saying unconfirmed teaches the operator to trust the colour over the word, and
	// a whole-body check on the word cannot see that happen (#107).
	if strings.Contains(body, "state-held") {
		t.Error("an unconfirmed lock was rendered with the locked styling")
	}
}

// TestARefusedLockNamesTheSessionItCouldNotReach. A hard error is a different sentence from
// "asked, cannot prove", and the engine keeps them in different buckets for the same reason.
func TestARefusedLockNamesTheSessionItCouldNotReach(t *testing.T) {
	f := newLockFixture(t)
	f.setLockErr(errors.New("session 2 is gone"))
	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("a refused lock answered %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "was not locked") {
		t.Errorf("a refused lock was not reported as a refusal: %s", body)
	}
	if !strings.Contains(body, "session 2 is gone") {
		t.Errorf("the page did not carry the backend's own reason: %s", body)
	}
	if strings.Contains(body, "nothing could prove") {
		t.Error("a refusal was rendered with the unconfirmed sentence")
	}
}

// TestAServerWithNoLockPathSaysSo rather than rendering a page that claims nothing happened
// in a way that reads as "the machine is fine". This is the same rule the other seams follow:
// an unwired seam is a claim about the wiring, and it is not allowed to look like a verdict.
func TestAServerWithNoLockPathSaysSo(t *testing.T) {
	f := newLockFixture(t)
	f.s.Lock = nil
	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("no lock path answered %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), "locking is not available") {
		t.Errorf("the page did not say the daemon cannot lock: %s", rec.Body.String())
	}
	// The page deliberately does not print the internal reason, so the log is the only
	// place that says which half was unwired.
	if !strings.Contains(strings.Join(f.logged.Lines(), "\n"), "no lock seam is wired") {
		t.Errorf("the log does not name the missing seam: %v", f.logged.Lines())
	}
}

// TestLockingTheDaemonsOwnAccountSaysBothHalves. The self-session rule and a manual lock point
// in opposite directions for exactly this account, and the page has to say both: the schedule
// will never hold it, and the operator just did.
func TestLockingTheDaemonsOwnAccountSaysBothHalves(t *testing.T) {
	f := newLockFixture(t)
	f.s.Self = platform.SelfLocation{InUserSlice: true, UID: 1000}
	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	// flat, not the raw body: the template wraps this prose to keep the source readable, and
	// a browser collapses the newline the same way it collapses any other space. Matching the
	// raw bytes would pin the template's line length, which is not a behaviour.
	body := flat(rec.Body.String())
	if !strings.Contains(body, "the daemon runs inside") {
		t.Errorf("the self-session note is missing: %s", body)
	}
	// Both halves, together. A page that only said "declined" would contradict the lock it
	// just performed; one that only said "locked" would leave the operator unable to explain
	// why this account is never held by its schedule.
	if !strings.Contains(body, "Its schedule never locks it") {
		t.Error("the note does not explain the schedule half")
	}
}

// TestTheSelfSessionNoteDoesNotRunIntoTheDetail is a measurement, not a reading. The Detail
// cell can hold two sentences at once -- what the lock produced, and the schedule note -- and
// the template trims the whitespace on both sides of each action, so on the raw bytes they
// arrive as one word: "...not confirmedThis is the session...". flat() cannot see this,
// because it collapses whitespace that is not there, and a browser collapses nothing; the
// defect only showed up in a 360 px screenshot of a real unconfirmed lock. So this asserts on
// the raw body, at the byte before the note begins.
func TestTheSelfSessionNoteDoesNotRunIntoTheDetail(t *testing.T) {
	f := newLockFixture(t)
	f.s.Self = platform.SelfLocation{InUserSlice: true, UID: 1000}
	f.setLockErr(platform.ErrUnconfirmed)
	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	raw := rec.Body.String()
	// Both halves have to be present or the assertion below is vacuous: a row with no
	// detail has nothing to run into anything.
	if !strings.Contains(raw, `class="problem"`) {
		t.Fatalf("the row rendered no detail, so nothing was tested: %s", raw)
	}
	i := strings.Index(raw, `<span class="override">`)
	if i < 0 {
		t.Fatalf("the row rendered no self-session note, so nothing was tested: %s", raw)
	}
	switch prev := raw[i-1]; prev {
	case ' ', '\t', '\n', '\r':
	default:
		t.Errorf("the detail and the self-session note are separated by %q, not by whitespace; "+
			"the cell renders as one run-on word: %q", string(prev), around(raw, i, 60, 40))
	}
}

// around quotes the bytes either side of a match, clamped to the body, so a failure names the
// seam it is complaining about rather than dumping the whole page.
func around(s string, i, before, after int) string {
	if i-before < 0 {
		before = i
	}
	if i+after > len(s) {
		after = len(s) - i
	}
	return s[i-before : i+after]
}

// TestALockRequestIsAuditedWithCountsNotAVerdict. The trail is what an operator reads the next
// week to work out what actually happened, so it carries the three counts rather than a
// summary word that would have to collapse them.
func TestALockRequestIsAuditedWithCountsNotAVerdict(t *testing.T) {
	f := newLockFixture(t)
	f.setLockErr(platform.ErrUnconfirmed)
	authPost(t, f.s, "/users/casey/lock-now", "")

	lines := strings.Join(f.audited.Lines(), "\n")
	if !strings.Contains(lines, "lock.requested") {
		t.Fatalf("nothing was audited: %s", lines)
	}
	if !strings.Contains(lines, "locked 0") || !strings.Contains(lines, "unconfirmed 1") {
		t.Errorf("the audit line does not carry the counts: %s", lines)
	}
	if strings.Contains(lines, "casey locked") {
		t.Errorf("the audit line claims a lock that was never confirmed: %s", lines)
	}
}

// TestTheLockResultPageKeepsItsNarrowWidthContract. The narrow-width guard enumerates the
// pages a signed-in caller can GET, and this page is only ever reached by POST -- so the
// shared contract has to be asserted here or it goes unguarded on the one page this feature
// adds.
func TestTheLockResultPageKeepsItsNarrowWidthContract(t *testing.T) {
	f := newLockFixture(t)
	f.setLockErr(platform.ErrUnconfirmed)
	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("lock-now answered %d", rec.Code)
	}
	bad, cells := rowsTableComplaints("/users/casey/lock-now", rec.Body.String())
	if cells == 0 {
		t.Fatal("the result page has no rows table to check")
	}
	if len(bad) > 0 {
		t.Errorf("the result page breaks the narrow-width contract: %s", strings.Join(bad, "; "))
	}
}

// TestTheLockResultPageHasNothingTheCSPWouldBlock. The shared CSP guard walks the pages a
// signed-in caller can GET, so a page that only ever answers a POST sits outside it -- the
// same ceiling the width contract above had to work around, for the same reason. This page
// prints operator-facing sentences inside spans, which is exactly where a style attribute
// creeps in, so the check runs on the body this handler actually rendered.
func TestTheLockResultPageHasNothingTheCSPWouldBlock(t *testing.T) {
	f := newLockFixture(t)
	f.setLockErr(platform.ErrUnconfirmed)
	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("lock-now answered %d", rec.Code)
	}
	body := rec.Body.String()
	// Without the table this is a refusal page, and a refusal has no verdict markup to
	// check -- the guard would pass by checking nothing.
	if !strings.Contains(body, `<table class="rows"`) {
		t.Fatalf("the result page rendered no rows table, so nothing was checked: %s", body)
	}
	for _, complaint := range cspComplaints(rec.Header().Get("Content-Security-Policy"), body) {
		t.Errorf("/users/casey/lock-now: %s", complaint)
	}
}
