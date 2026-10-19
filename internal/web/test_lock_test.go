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
	"github.com/vicowa-botworks/parental-lockout/internal/lockop"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// Probing the backends from the dashboard, and the standard that surface is held to.
//
// The probe is the only way an operator can check a verdict the page asserts for them, so the
// failure this file is written against is the probe that answers the wrong question: one that
// locks a screen while it asks (which makes it unpressable while the child is working, and
// makes the audit trail claim an event that was never requested), or one that renders an empty
// table as "nothing works" when what failed was the wiring.
//
// Every test therefore asserts on both seams: what the probe seam was asked, and that the lock
// seam was not. The second half is the one a handler can violate by accident while adding a
// feature, and it is the half no page can reveal.

// probeFixture is a server whose probe seam answers from a fixed table and whose lock seam
// records any request, so "it asked the backends" and "it never locked anything" are both
// observable rather than inferred from a status code.
type probeFixture struct {
	s       *Server
	logged  *logSink
	audited *logSink

	mu          sync.Mutex
	probeCalls  int
	statuses    []platform.BackendStatus
	asked       []string // session IDs handed to the LOCK seam; must stay empty
	sessions    []platform.Session
	sessionsErr error
}

func newProbeFixture(t *testing.T) *probeFixture {
	t.Helper()
	s, logged, audited := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	m, err := AssetMap(Assets())
	if err != nil {
		t.Fatalf("assets: %v", err)
	}
	s.assets = m
	f := &probeFixture{
		s:       s,
		logged:  logged,
		audited: audited,
		// One reachable backend and one that is missing its binary: the shape a real
		// misconfiguration produces, and the only fixture that can tell "some backends
		// answer" apart from "all of them do" and "none does".
		statuses: []platform.BackendStatus{
			{Name: "kde", Available: true, Detail: "gdbus found"},
			{Name: "logind", Available: false, Detail: "/usr/bin/loginctl: no such file"},
		},
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
	s.Lock = func(_ context.Context, sess platform.Session) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.asked = append(f.asked, sess.ID)
		return nil
	}
	s.Probe = func(context.Context) []platform.BackendStatus {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.probeCalls++
		return append([]platform.BackendStatus(nil), f.statuses...)
	}
	return f
}

func (f *probeFixture) setStatuses(s ...platform.BackendStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses = append([]platform.BackendStatus(nil), s...)
}

func (f *probeFixture) setSessions(s []platform.Session) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions = s
}

func (f *probeFixture) setSessionsErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessionsErr = err
}

func (f *probeFixture) calls() (probes int, locked []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.probeCalls, append([]string(nil), f.asked...)
}

// TestTheDashboardOffersToCheckTheBackends is the control for the whole feature: a handler on
// a route no page links to is a feature that does not exist, and this package has shipped one
// of those before (#59).
func TestTheDashboardOffersToCheckTheBackends(t *testing.T) {
	f := newProbeFixture(t)
	rec := authGet(t, f.s, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard: %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `action="/users/casey/test-lock"`) {
		t.Error("the dashboard has no form that posts to /users/casey/test-lock")
	}
	if !strings.Contains(body, `type="submit">Check backends</button>`) {
		t.Error("the probe form has no submit control, so nothing can post it")
	}
	// The two buttons in one row have to be distinguishable by their own labels, not by
	// which cell they sit in: one locks a child's screen and one reads a table.
	if strings.Contains(body, `>Check backends and lock<`) {
		t.Error("the probe button's label mentions locking")
	}
}

// TestAProbeNeverLocksASession is the difference between this route and the one beside it. A
// probe that locked as a side effect could not be pressed while the child is working, which is
// precisely when the question gets asked -- and it would write a lock event the operator never
// requested into the trail the operator reads as evidence.
func TestAProbeNeverLocksASession(t *testing.T) {
	f := newProbeFixture(t)
	rec := authPost(t, f.s, "/users/casey/test-lock", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("test-lock answered %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	probes, locked := f.calls()
	if probes != 1 {
		t.Errorf("the probe seam was called %d times, want 1", probes)
	}
	if len(locked) != 0 {
		t.Errorf("the probe locked sessions %v; a probe asks, it does not lock", locked)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Nothing was locked") {
		t.Error("the result page does not say that nothing was locked")
	}
}

// TestAnUnauthenticatedProbeIsRefusedWithoutAskingAnything shows the gate on the route it is
// claimed for: the CSRF and auth layers wrap the mux, but "covered by construction" is only
// worth anything once it has been demonstrated on the new registration.
func TestAnUnauthenticatedProbeIsRefusedWithoutAskingAnything(t *testing.T) {
	f := newProbeFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/users/casey/test-lock", nil)
	req.Header.Set("Accept", "text/html")
	req.RemoteAddr = "192.0.2.71:1"
	f.s.Handler().ServeHTTP(rec, req)

	if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || loc != "/login" {
		t.Errorf("an anonymous probe answered %d Location=%q, want a redirect to /login",
			rec.Code, loc)
	}
	if probes, locked := f.calls(); probes != 0 || len(locked) != 0 {
		t.Errorf("an anonymous request reached the seams: %d probes, locks %v", probes, locked)
	}
}

// TestAForgedProbeFormIsRefusedWithoutAskingAnything posts a real session's cookie with a
// token the server never minted.
func TestAForgedProbeFormIsRefusedWithoutAskingAnything(t *testing.T) {
	f := newProbeFixture(t)
	resp, _ := login(t, f.s, "parent", "hunter2")
	c := cookieOf(t, resp)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/users/casey/test-lock", strings.NewReader("csrf=not-a-token"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(c)
	req.RemoteAddr = "192.0.2.71:1"
	f.s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("forged token answered %d, want %d", rec.Code, http.StatusForbidden)
	}
	if probes, _ := f.calls(); probes != 0 {
		t.Errorf("a forged form reached the probe seam %d times", probes)
	}
}

// TestProbingAnUnmanagedAccountIsRefused is the positive control for the name check: the same
// request shape is accepted one line later against the account the config does manage.
func TestProbingAnUnmanagedAccountIsRefused(t *testing.T) {
	f := newProbeFixture(t)
	rec := authPost(t, f.s, "/users/mallory/test-lock", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("an unmanaged account answered %d, want %d", rec.Code, http.StatusNotFound)
	}
	if !strings.Contains(rec.Body.String(), "no managed account") {
		t.Errorf("the refusal does not say what was refused: %s", rec.Body.String())
	}
	if probes, _ := f.calls(); probes != 0 {
		t.Errorf("probing an unmanaged account reached the seam %d times", probes)
	}

	if rec := authPost(t, f.s, "/users/casey/test-lock", ""); rec.Code != http.StatusOK {
		t.Errorf("the managed account answered %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestEachBackendIsReportedWithItsOwnAnswer is the reason the page exists. A single
// "backends healthy" line would be the dashboard's problem again: an operator needs to see
// which backend answered and what the unreachable one said was missing.
func TestEachBackendIsReportedWithItsOwnAnswer(t *testing.T) {
	f := newProbeFixture(t)
	rec := authPost(t, f.s, "/users/casey/test-lock", "")
	body := rec.Body.String()
	for _, want := range []string{"kde", "logind", "/usr/bin/loginctl: no such file"} {
		if !strings.Contains(body, want) {
			t.Errorf("the result page never says %q: %s", want, body)
		}
	}
	if !strings.Contains(body, "1 of 2 configured lock backends answer") {
		t.Errorf("the banner does not count the reachable backends: %s", body)
	}
}

// TestAllBackendsUnreachableIsNotSoftened is the refusal direction this repository keeps having
// to relearn: a diagnostic that rounds "nothing works" up to a reassuring sentence is worse
// than no diagnostic, because the operator stops looking.
func TestAllBackendsUnreachableIsNotSoftened(t *testing.T) {
	f := newProbeFixture(t)
	f.setStatuses(platform.BackendStatus{Name: "kde", Detail: "gdbus: not found"})
	rec := authPost(t, f.s, "/users/casey/test-lock", "")
	body := rec.Body.String()
	if !strings.Contains(body, "None of the 1 configured lock backends") {
		t.Errorf("a machine with no working backend was not said plainly: %s", body)
	}
	// Scoped to the backend table on purpose: the session table below still says a session
	// would be lockable, and a whole-page check would read that as a softened backend.
	if strings.Contains(firstRowsTable(body), "state-held") {
		t.Error("an unreachable backend was rendered with the reachable styling")
	}
}

// firstRowsTable is the first table on the page, which on the probe result page is always the
// backend table. Assertions about one table have to be scoped to it, because the page carries
// a second one whose cells mean something else.
func firstRowsTable(body string) string {
	i := strings.Index(body, `<table class="rows"`)
	if i < 0 {
		return ""
	}
	j := strings.Index(body[i:], "</table>")
	if j < 0 {
		return body[i:]
	}
	return body[i : i+j]
}

// TestNoBackendsConfiguredSaysTheDaemonHasNone distinguishes "the machine cannot lock a screen"
// from "nobody was ever asked", which an empty table cannot do on its own.
func TestNoBackendsConfiguredSaysTheDaemonHasNone(t *testing.T) {
	f := newProbeFixture(t)
	f.setStatuses()
	rec := authPost(t, f.s, "/users/casey/test-lock", "")
	if body := rec.Body.String(); !strings.Contains(body, "No lock backend is configured") {
		t.Errorf("an empty probe was rendered as a table with no rows instead of a sentence: %s", body)
	}
}

// TestAServerWithNoProbePathSaysSo. A nil seam is the assembled-wrong case, and rendering an
// empty table for it would turn a wiring mistake into a statement about the operator's desktop.
func TestAServerWithNoProbePathSaysSo(t *testing.T) {
	f := newProbeFixture(t)
	f.s.Probe = nil
	rec := authPost(t, f.s, "/users/casey/test-lock", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("an unwired probe seam answered %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "backend probe is not available") {
		t.Errorf("the page did not say the daemon is the broken half: %s", body)
	}
	if probes, _ := f.calls(); probes != 0 {
		t.Errorf("an unwired seam still probed %d times", probes)
	}
}

// TestAFailedSessionListingStillReportsTheBackends is the half-failure shape. The backend
// answers were obtained and stand on their own; the session half failed and must say so rather
// than rendering as "this account has no session" -- the same rule the lock route holds,
// applied where the two reads have different status.
func TestAFailedSessionListingStillReportsTheBackends(t *testing.T) {
	f := newProbeFixture(t)
	f.setSessionsErr(errors.New("loginctl: signal: killed"))
	rec := authPost(t, f.s, "/users/casey/test-lock", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("a failed session read threw away the backend answers: %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "session table could not be read") {
		t.Errorf("the page did not say the listing failed: %s", body)
	}
	if strings.Contains(body, "has no session on this machine") {
		t.Error("a listing that failed was rendered as an account with no session")
	}
	if !strings.Contains(body, "kde") {
		t.Error("the backend answers were dropped along with the failed read")
	}
}

// TestAnAccountWithNoSessionStillGetsTheBackendAnswer. "Nobody is logged in" does not make the
// capability question unanswerable, and refusing here would leave the operator with no way to
// check the machine before the child logs in.
func TestAnAccountWithNoSessionStillGetsTheBackendAnswer(t *testing.T) {
	f := newProbeFixture(t)
	f.setSessions(nil)
	rec := authPost(t, f.s, "/users/casey/test-lock", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("an account with no session answered %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "has no session on this machine right") {
		t.Errorf("the page did not say there was nothing to lock: %s", body)
	}
	if !strings.Contains(body, "kde") {
		t.Error("the backend answers were withheld because the session read found nobody")
	}
}

// TestANonLockableSessionIsRefusedInTheLoopsOwnWords. The page explains a refusal the
// enforcement loop would make, so it must use the loop's sentence rather than writing its own:
// two explanations of one rule is how an operator ends up trusting the wrong one.
func TestANonLockableSessionIsRefusedInTheLoopsOwnWords(t *testing.T) {
	f := newProbeFixture(t)
	f.setSessions([]platform.Session{{ID: "3", Username: "casey", UID: 1000,
		Class: "greeter", Seat: "seat0", Type: "x11"}})
	rec := authPost(t, f.s, "/users/casey/test-lock", "")
	body := rec.Body.String()
	want := lockop.NotLockable(platform.Session{Class: "greeter", Seat: "seat0", Type: "x11"})
	if !strings.Contains(body, want) {
		t.Errorf("the refusal is not the loop's own sentence (%q): %s", want, body)
	}
	if !strings.Contains(body, "Would lock") {
		t.Error("the session table has no column saying whether a lock would be attempted")
	}
}

// TestTheProbePagesSelfSessionNoteDoesNotRunIntoItsDetail is #108's contract on the page
// that copied the Detail cell.
//
// #106 lifted the cell out of lock.tmpl wholesale -- the note, the separator and the
// comment explaining why the separator has to be in the text. The copy inherited the defect
// along with the layout, and nothing here noticed, because no test in this file ever set
// the self location, so the note never rendered and the seam was never on screen. A cell
// that has been copied is a cell that has to be checked on both pages.
func TestTheProbePagesSelfSessionNoteDoesNotRunIntoItsDetail(t *testing.T) {
	f := newProbeFixture(t)
	f.s.Self = platform.SelfLocation{InUserSlice: true, UID: 1000}
	// A greeter session, so the row carries a refusal: the seam only exists where a detail
	// and the note are both present.
	f.setSessions([]platform.Session{{ID: "3", Username: "casey", UID: 1000,
		Class: "greeter", Seat: "seat0", Type: "x11"}})
	rec := authPost(t, f.s, "/users/casey/test-lock", "")
	assertTheDetailCellEndsWhereTheNoteBegins(t, rec.Body.String())
}

// probeAuditLines is this action's slice of the trail. The sink also carries the sign-in that
// authPost performs, so the assertion is about lock.* lines rather than about emptiness.
func probeAuditLines(f *probeFixture) []string {
	var out []string
	for _, l := range f.audited.Lines() {
		if strings.HasPrefix(l, "lock.") {
			out = append(out, l)
		}
	}
	return out
}

// authPostAfterBreaking signs in first and then breaks the server, so a seam can be made to
// fail on the request under test rather than on the sign-in that gets there. Breaking it before
// the login would only ever exercise the login page's own refusal.
func authPostAfterBreaking(t *testing.T, s *Server, breakIt func(*Server), path string) *httptest.ResponseRecorder {
	t.Helper()
	resp, _ := login(t, s, "parent", "hunter2")
	c := cookieOf(t, resp)
	token, ok := s.csrf.lookup(c.Value)
	if !ok {
		t.Fatal("the login minted no CSRF token")
	}
	breakIt(s)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", path, strings.NewReader("csrf="+token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(c)
	req.RemoteAddr = "192.0.2.71:1"
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// TestARefusedProbeWritesNoAuditLine is #100's third criterion, walked on every refusal path
// rather than one of them: a handler with a single logging point above the refusals passes a
// one-case test while recording requests it never carried out.
func TestARefusedProbeWritesNoAuditLine(t *testing.T) {
	cases := []struct {
		name string
		path string
		prep func(f *probeFixture)
	}{
		{"an unmanaged account", "/users/mallory/test-lock", nil},
		{"an unwired seam", "/users/casey/test-lock", func(f *probeFixture) { f.s.Probe = nil }},
		// Broken after the sign-in: the login itself reads the credential through the same
		// seam, so breaking it first tests the login page and never reaches this handler.
		{"an unreadable config", "/users/casey/test-lock", func(f *probeFixture) {
			f.s.Config = func() (*config.Config, error) { return nil, errors.New("config: unreadable") }
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newProbeFixture(t)
			rec := authPostAfterBreaking(t, f.s, func(s *Server) {
				if tc.prep != nil {
					tc.prep(f)
				}
			}, tc.path)
			if rec.Code == http.StatusOK {
				t.Fatalf("%s was accepted, so the case does not test a refusal", tc.name)
			}
			if got := probeAuditLines(f); len(got) != 0 {
				t.Errorf("%s wrote audit lines %v", tc.name, got)
			}
		})
	}
}

// TestAProbeIsAuditedWithCountsNotAVerdict. The trail is read back as evidence later, so it
// records what was established -- how many backends answered -- and not a conclusion the
// machine did not earn.
func TestAProbeIsAuditedWithCountsNotAVerdict(t *testing.T) {
	f := newProbeFixture(t)
	if rec := authPost(t, f.s, "/users/casey/test-lock", ""); rec.Code != http.StatusOK {
		t.Fatalf("test-lock answered %d", rec.Code)
	}
	got := probeAuditLines(f)
	if len(got) != 1 {
		t.Fatalf("a probe wrote %d audit lines, want 1: %v", len(got), got)
	}
	if !strings.Contains(got[0], "lock.probed") || !strings.Contains(got[0], "1 of 2 backends reachable") {
		t.Errorf("the audit line does not carry the counts: %q", got[0])
	}
	if strings.Contains(got[0], "locked") {
		t.Errorf("a probe wrote a line claiming a lock: %q", got[0])
	}
}

// TestTheProbeResultPageKeepsItsNarrowWidthContract. The shared width guard walks the pages a
// signed-in caller can GET, so a page that only ever answers a POST sits outside it whatever
// path is added to its list -- the GET would be a 405 and the check would then run against a
// refusal. The contract is asserted here, on the body this handler actually rendered.
func TestTheProbeResultPageKeepsItsNarrowWidthContract(t *testing.T) {
	f := newProbeFixture(t)
	rec := authPost(t, f.s, "/users/casey/test-lock", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("test-lock answered %d", rec.Code)
	}
	bad, cells := rowsTableComplaints("/users/casey/test-lock", rec.Body.String())
	if cells == 0 {
		t.Fatal("the result page has no rows table to check")
	}
	if len(bad) > 0 {
		t.Errorf("the result page breaks the narrow-width contract: %s", strings.Join(bad, "; "))
	}
}

// TestTheProbeResultPageHasNothingTheCSPWouldBlock, for the same GET-only reason and on the
// same body. This page prints operator-facing sentences inside spans, which is where a style
// attribute creeps in.
func TestTheProbeResultPageHasNothingTheCSPWouldBlock(t *testing.T) {
	f := newProbeFixture(t)
	rec := authPost(t, f.s, "/users/casey/test-lock", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("test-lock answered %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<table class="rows"`) {
		t.Fatalf("the result page rendered no rows table, so nothing was checked: %s", body)
	}
	for _, complaint := range cspComplaints(rec.Header().Get("Content-Security-Policy"), body) {
		t.Errorf("/users/casey/test-lock: %s", complaint)
	}
}
