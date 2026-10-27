package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
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
	lockErrFor  map[string]error // per-session override, keyed by session ID
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
	if err, ok := f.lockErrFor[sess.ID]; ok {
		return err
	}
	return f.lockErr
}

// setLockErrFor answers one session differently from the rest. It exists for the page's
// partial verdict -- one row proved, one row not -- which is the only state where the
// banner has to do arithmetic instead of printing a word. A fixture where every session
// takes the same branch can never render "1 of 2", so the guard that stops a partial proof
// being printed as "casey is locked" would never be exercised by anything.
func (f *lockFixture) setLockErrFor(sessionID string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lockErrFor == nil {
		f.lockErrFor = map[string]error{}
	}
	f.lockErrFor[sessionID] = err
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

// The banner and the flag are the two sentences an operator quotes to someone else, so the
// verdict tests read them whole rather than grepping a phrase out of them. A grep on
// "is locked" cannot see a clause appended after it, and an appended clause is exactly how
// this page would start claiming a lock it did not prove: "1 of 2 of casey's sessions are
// locked; the rest are locked as well." passes every phrase-level check on the page. Pinning
// the sentence means rewording it is a deliberate edit -- which is the point, because the
// sentence is the product.
var (
	bannerRe = regexp.MustCompile(`(?s)<p class="banner">(.*?)</p>`)
	flagRe   = regexp.MustCompile(`(?s)<p class="flag"[^>]*>(.*?)</p>`)
	// The Result column's verdict spans, class and text together: the only way to say "this
	// element must not claim a lock" without pinning the sentence it happens to use. The
	// package's existing tagRe strips the markup.
	cellRe = regexp.MustCompile(`<span class="(state-[a-z]+)">([^<]*)</span>`)
	// The proved-lock colour, wherever it is worn. See heldIsOnlyEverALockedVerdict.
	heldRe    = regexp.MustCompile(`(?s)<span class="state-held">(.*?)</span>`)
	captionRe = regexp.MustCompile(`(?s)<caption>(.*?)</caption>`)
)

// heldIsOnlyEverALockedVerdict is the page-wide form of the colour rule. The cell-level checks
// cover the Result column, but any element on the page can borrow --held: the muted seat marker
// could, and so could the self-session note, and neither is a verdict cell that a cell-level
// rule ever looks at. The colour is what the eye reads before the words, so the rule is stated
// once for the whole document -- whatever wears the proved-lock colour has to be a cell saying
// "locked".
func heldIsOnlyEverALockedVerdict(t *testing.T, raw string) {
	t.Helper()
	for _, m := range heldRe.FindAllStringSubmatch(raw, -1) {
		if got := flat(m[1]); got != "locked" {
			t.Errorf("something the page does not prove wears the locked colour: %q", got)
		}
	}
}

// prose returns one of the page's prose elements with its markup gone and its whitespace
// collapsed, so a test compares what a reader sees rather than what the template wraps.
func prose(re *regexp.Regexp, t *testing.T, body, what string) string {
	t.Helper()
	m := re.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("the lock page rendered no %s paragraph", what)
	}
	return strings.TrimSpace(tagRe.ReplaceAllString(flat(m[1]), ""))
}

// verdictCells lists the Result column's cells in row order as class-and-text pairs.
func verdictCells(body string) [][2]string {
	var out [][2]string
	for _, m := range cellRe.FindAllStringSubmatch(body, -1) {
		out = append(out, [2]string{m[1], flat(m[2])})
	}
	return out
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
//
// #168 changed what happens when the read came back partly full. The listing in this test
// still names casey's session, and answering 503 about it was answering about a child whose
// session was in the same slice: the page now locks what it read and says what it could not.
// What has not changed is that a failed read is never rendered as an account with no session.
func TestAFailedSessionListingIsNotReportedAsNoSession(t *testing.T) {
	f := newLockFixture(t)
	f.mu.Lock()
	f.sessionsErr = errors.New("loginctl: signal: killed")
	f.mu.Unlock()

	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Errorf("a listing that still named the session answered %d, want %d: %s",
			rec.Code, http.StatusOK, body)
	}
	if got := f.askedFor(); len(got) != 1 || got[0] != "2" {
		t.Errorf("the session that was read was not locked; asked for %v", got)
	}
	if !strings.Contains(body, "loginctl: signal: killed") {
		t.Errorf("the page does not name what it could not read: %s", body)
	}
	if strings.Contains(body, "has no session") {
		t.Error("a listing that failed was rendered as an account with no session")
	}
	// The trail carries the gap too. "locked 1" with no mention of the session that could
	// not be read is a record that understates what was attempted, and the audit page is the
	// surface an operator reads afterwards as evidence.
	sawGap := false
	for _, l := range f.audited.Lines() {
		if strings.HasPrefix(l, "lock.listing_incomplete") {
			sawGap = true
		}
	}
	if !sawGap {
		t.Errorf("the incomplete read wrote no audit line; wrote %v", f.audited.Lines())
	}
}

// TestASessionListingThatReadNothingIsStillAnError is the other arm, so that "use what read"
// cannot become "ignore errors". With no rows there is nothing to lock, and the page says it
// could not look rather than implying the account is not logged in.
func TestASessionListingThatReadNothingIsStillAnError(t *testing.T) {
	f := newLockFixture(t)
	f.setSessions(nil)
	f.mu.Lock()
	f.sessionsErr = errors.New("loginctl: signal: killed")
	f.mu.Unlock()

	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("a listing that read nothing answered %d, want %d",
			rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), "session table could not be read") {
		t.Errorf("the page did not say the listing failed: %s", rec.Body.String())
	}
	if got := f.askedFor(); len(got) != 0 {
		t.Errorf("a read that named no session locked %v", got)
	}
}

// TestACompleteReadAddsNoNoteToTheLockPage keeps the note honest in its other direction. The
// committed 360 px artifact is a photograph of this page on a complete read, and a note that
// rendered when nothing was missing would silently change a screenshot nobody re-took (#144
// is that failure in general).
func TestACompleteReadAddsNoNoteToTheLockPage(t *testing.T) {
	f := newLockFixture(t)
	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("a complete read answered %d, so the check below proves nothing: %s", rec.Code, body)
	}
	if !strings.Contains(body, `scope="row">2<`) {
		t.Fatalf("the session row is missing, so the check below proves nothing: %s", body)
	}
	if strings.Contains(body, "partly readable") {
		t.Error("the partial-read note rendered on a read that was complete")
	}
	for _, l := range f.audited.Lines() {
		if strings.HasPrefix(l, "lock.listing_incomplete") {
			t.Errorf("a complete read wrote an incomplete-listing audit line: %s", l)
		}
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
		// #168 moved the partly-readable listing out of this table: it now locks the rows it
		// read, so it is an accepted action with an audit line, tested above. What is still a
		// refusal is a read that named nothing — and that one must stay silent.
		{"a listing that read nothing", "/users/casey/lock-now", func(f *lockFixture) {
			f.setSessions(nil)
			f.mu.Lock()
			f.sessionsErr = errors.New("loginctl: signal: killed")
			f.mu.Unlock()
		}},
		// The two refusals that happen before the sessions are even looked at. A handler with
		// one logging point below them passes a test that only walks the session-shaped
		// refusals, and the trail then records a lock request the daemon never made.
		{"a server with no lock path", "/users/casey/lock-now",
			func(f *lockFixture) { f.s.Lock = nil }},
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

	// The config refusal is walked after sign-in rather than before it. The admin credential is
	// read through this same seam, so breaking it first fails the login and the case proves
	// nothing about the lock.
	f3 := newLockFixture(t)
	authPostAfterBreaking(t, f3.s, func(s *Server) {
		s.Config = func() (*config.Config, error) {
			return nil, errors.New("config: read /etc/plock/config.json")
		}
	}, "/users/casey/lock-now")
	if lines := lockLines(f3); len(lines) != 0 {
		t.Errorf("a config that would not read wrote an audit line: %v", lines)
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
	if got := prose(bannerRe, t, body, "banner"); got != "casey is locked." {
		t.Errorf("a proved lock's banner reads %q, want exactly %q", got, "casey is locked.")
	}
	if strings.Contains(body, "unconfirmed") {
		t.Error("a proved lock mentioned being unconfirmed")
	}
	if got := f.askedFor(); len(got) != 1 || got[0] != "2" {
		t.Errorf("the seam was asked for sessions %v, want exactly [2]", got)
	}
}

// TestAPartialProofIsNotPrintedAsTheWholeAccountIsLocked is the banner's arithmetic. An
// account can hold a desktop row and a raw virtual terminal, and the backend can prove one
// and refuse the other: the banner has to count. Dropping the "and that is all of them" half
// of the condition prints "casey is locked" over a page whose second row says the backend
// refused -- the one sentence on this page the operator quotes to a child. (A greeter row
// cannot stand in for the refused one: platform refuses it before the seam is ever asked.)
func TestAPartialProofIsNotPrintedAsTheWholeAccountIsLocked(t *testing.T) {
	f := newLockFixture(t)
	f.setSessions([]platform.Session{
		{ID: "2", Username: "casey", UID: 1000, Class: "user", Seat: "seat0", Type: "x11", State: "active"},
		{ID: "3", Username: "casey", UID: 1000, Class: "user", Seat: "tty3", Type: "tty"},
	})
	f.setLockErrFor("3", errors.New("session 3 is gone"))
	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("a partial lock answered %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if got := f.askedFor(); len(got) != 2 {
		t.Fatalf("the seam was asked for sessions %v, want both rows", got)
	}
	if got, want := prose(bannerRe, t, body, "banner"),
		"1 of 2 of casey's sessions are locked; the rest are below."; got != want {
		t.Errorf("a lock proved on one of two sessions reads %q, want exactly %q", got, want)
	}
	// The whole-account sentence, not merely the word: "are locked" is the partial banner's
	// own phrasing, so a check on "locked" alone would pass on the correct page too.
	if strings.Contains(body, "is locked") {
		t.Error("a proof covering one of two sessions was printed as the account being locked")
	}
	if !strings.Contains(body, "session 3 is gone") {
		t.Errorf("the refused row did not carry the backend's own reason: %s", body)
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
	// "unconfirmed" alone is a word, not an instruction. Both sentences whole -- the flag is
	// the paragraph that exists precisely to stop the page implying a lock, and a grep on
	// "nothing could prove" survives "…and the rest are locked." appended to it.
	if got, want := prose(bannerRe, t, body, "banner"), "casey was not locked."; got != want {
		t.Errorf("an unconfirmed lock's banner reads %q, want exactly %q", got, want)
	}
	const wantFlag = "1 of 1 session was asked to lock and nothing could prove the screen went down."
	if got := prose(flagRe, t, body, "flag"); !strings.HasPrefix(got, wantFlag) {
		t.Errorf("the unconfirmed flag opens with %q, want %q", got, wantFlag)
	}
	// And the word is only half of it. The cell's class carries the colour, which is what the
	// eye reads before the sentence: --held is the colour that means "proved". A row wearing
	// it while saying unconfirmed teaches the operator to trust the colour over the word, and
	// a whole-body check on the word cannot see that happen (#107).
	heldIsOnlyEverALockedVerdict(t, body)
	// The flag paragraph carries one conditional sentence for the mixed case -- the sessions
	// that say locked are a different case, those were proved -- and on a page where nothing
	// was proved that is a claim about locks that do not exist. It is held back by a single
	// comparison, so the sentence is one deleted condition away from printing always.
	if strings.Contains(body, "those were proved") {
		t.Error("the page claimed a proved lock on a page where nothing was proved")
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
	if got, want := prose(bannerRe, t, body, "banner"), "casey was not locked."; got != want {
		t.Errorf("a refused lock's banner reads %q, want exactly %q", got, want)
	}
	if !strings.Contains(body, "session 2 is gone") {
		t.Errorf("the page did not carry the backend's own reason: %s", body)
	}
	if strings.Contains(body, "nothing could prove") {
		t.Error("a refusal was rendered with the unconfirmed sentence")
	}
	// The same colour rule as the unconfirmed row, and this is the sharper of the two: a
	// refused session is the row that means "the child is still sitting at their screen",
	// and --held is the class this page reserves for a lock a backend proved. The word can
	// say "not locked" in the same cell while the colour says otherwise, and an assertion on
	// the word cannot see it (#107).
	heldIsOnlyEverALockedVerdict(t, body)
	// Word and class, read off the element rather than grepped out of the page. Pinning the
	// whole markup string instead would fail a better sentence -- "not locked (the backend
	// refused)" is the same verdict in more words -- while a grep on the word alone cannot see
	// the cell reading "locked" in the not-locked colour. Both are checked here, on the cell.
	cells := verdictCells(body)
	if len(cells) != 1 {
		t.Fatalf("the page rendered %d verdict cells, want the one refused session", len(cells))
	}
	if cells[0][0] == "state-held" {
		t.Error("a session that was not locked was rendered with the locked styling")
	}
	if w := strings.ToLower(cells[0][1]); w == "locked" || strings.HasPrefix(w, "locked ") {
		t.Errorf("the refused row's Result cell claims a lock: %q", cells[0][1])
	}
	// The caption is the third sentence an operator reads on this page and the one no
	// assertion looked at: reworded from "and what asking it to lock produced" to "and the
	// locks that were proved", it claims locks on a page where the backend refused.
	if got, want := prose(captionRe, t, body, "caption"),
		"Every session casey holds on this machine, and what asking it to lock produced."; got != want {
		t.Errorf("the caption reads %q, want exactly %q", got, want)
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
	raw := rec.Body.String()
	body := flat(raw)
	if !strings.Contains(body, "the daemon runs inside") {
		t.Errorf("the self-session note is missing: %s", body)
	}
	// Both halves, together. A page that only said "declined" would contradict the lock it
	// just performed; one that only said "locked" would leave the operator unable to explain
	// why this account is never held by its schedule.
	if !strings.Contains(body, "Its schedule never locks it") {
		t.Error("the note does not explain the schedule half")
	}
	// This is the one page in the file where the locked colour is legitimate, so it is also
	// the only place the rule can be checked against the note: the note is prose, and prose
	// that borrows --held reads as a proved lock no matter what it says.
	heldIsOnlyEverALockedVerdict(t, raw)
	// The control: with the seam unwired the note is absent. Without this the two assertions
	// above only prove the page prints the paragraph, not that it prints it on the daemon's
	// own row -- and on any other row its last clause, "you asked for this one, so it
	// happened", is a claim that a lock happened.
	f2 := newLockFixture(t)
	control := flat(authPost(t, f2.s, "/users/casey/lock-now", "").Body.String())
	if strings.Contains(control, "the daemon runs inside") {
		t.Error("the self-session note printed on a row that is not the daemon's own session")
	}
}

// TestTheSelfSessionNoteDoesNotRunIntoTheDetail is a measurement, not a reading. The Detail
// cell can hold two sentences at once -- what the lock produced, and the schedule note -- and
// the template trims the whitespace on both sides of each action, so on the raw bytes they
// arrive as one word: "...not confirmedThis is the session...". flat() cannot see this,
// because it collapses whitespace that is not there, and a browser collapses nothing; the
// defect only showed up in a 360 px screenshot of a real unconfirmed lock. So this asserts on
// the raw body, in two halves: something must sit between the detail's `</span>` and the note
// at all (#103), and the detail's own text must end in a sentence terminator rather than a
// space (#108). Both halves are in assertTheDetailCellEndsWhereTheNoteBegins, and each is
// pinned on each page: the separation half by W204 here and W205 on the probe page, the
// terminator half by W202 here and W203 there, with W201 taking it out of the helper itself.
func TestTheSelfSessionNoteDoesNotRunIntoTheDetail(t *testing.T) {
	f := newLockFixture(t)
	f.s.Self = platform.SelfLocation{InUserSlice: true, UID: 1000}
	f.setLockErr(platform.ErrUnconfirmed)
	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	assertTheDetailCellEndsWhereTheNoteBegins(t, rec.Body.String())
}

// assertTheDetailCellEndsWhereTheNoteBegins is the seam contract on the raw rendered bytes,
// and it asserts both halves of it: that something separates the two spans (#103) and that
// the separation is a sentence boundary rather than a space (#108).
//
// The seam has to be a sentence boundary, not merely a gap. #103 fixed a collision -- zero
// characters between the two spans -- by putting a space between them, and a space is not a
// terminator: the page then read "...accepted the request but cannot confirm it This is the
// session the daemon runs inside.", where the only thing marking the end of one sentence and
// the start of the next is a capital letter.
//
// The assertion is on the detail's own text, cut out of the raw bytes. Not on the byte
// adjacent to the note's tag, because that byte is the `>` of `</span>` and reads the same
// whether the detail is terminated or not; and not on a whitespace-flattened rendering,
// because flattening is exactly what hides the difference between a space and a period.
//
// It is a helper rather than a body because two pages render this cell: the probe result
// page copied it out of lock.tmpl wholesale, note and all, and had no test that ever
// rendered the note at all.
func assertTheDetailCellEndsWhereTheNoteBegins(t *testing.T, raw string) {
	t.Helper()
	// Both halves have to be present or the assertion below is vacuous: a row with no
	// detail has nothing to run into anything.
	i := strings.Index(raw, `<span class="override">`)
	if i < 0 {
		t.Fatalf("the row rendered no self-session note, so nothing was tested: %s", raw)
	}
	// Both spans are found by searching back from the note, not forward from the top of the
	// page. The probe page puts a class="problem" span in its banner, so the first one on
	// the page is not the cell under test -- and an assertion that quietly measures the
	// banner passes or fails for reasons that have nothing to do with the seam.
	end := strings.LastIndex(raw[:i], `</span>`)
	if end < 0 {
		t.Fatalf("nothing closes a span before the self-session note, so nothing was tested: %s", raw)
	}
	head := strings.LastIndex(raw[:end], `class="problem">`)
	if head < 0 {
		t.Fatalf("the row rendered no detail span before the note, so nothing was tested: %s", raw)
	}
	detail := raw[head+len(`class="problem">`) : end]
	// The span found by searching backwards is only this cell's own if nothing else sits
	// between them. A row with no detail renders the note alone -- a lockable self session on
	// the probe page, which is the ordinary case on a real machine -- and then the nearest
	// closed span belongs to another cell. Measured: with the fixture's session changed from
	// `greeter` to `user`, the helper reached back to the "Would lock" cell and reported
	// `separated by "</td>\n      <td data-label=\"Detail\">"` on a page with no seam at all.
	// A failure that points at markup nowhere near the seam is worse than no assertion, so
	// this says what is missing instead of measuring the wrong thing.
	gap := raw[end+len(`</span>`) : i]
	// "Another cell" is decided by cell boundaries, not by any markup. `<wbr>` and `<!--x-->`
	// both render nothing, so a separator made of either IS the #103 run-on and belongs to the
	// check below. Measured with `<` as the test: both produced "this row rendered no detail of
	// its own" about a row that had one, sending the reader to the fixture instead of the
	// template line.
	if strings.Contains(gap, "</td") || strings.Contains(gap, "<td") ||
		strings.Contains(gap, "</tr") || strings.Contains(gap, "<tr") {
		t.Fatalf("the nearest closed span before the self-session note is in another cell (%q between them): "+
			"this row rendered no detail of its own, so there is no seam to test", gap)
	}
	// #103's contract, kept whole: something has to sit between the two spans at all. Go's
	// `{{if}}`/`{{end}}` trim the whitespace around them, so if the separator action is
	// dropped the spans land against each other and the cell renders
	// "...confirmed.This is the session..." -- a worse run-on than the one #108 is about, and
	// one that a check on the detail's last character cannot see, because the detail is still
	// correctly terminated. Measured: with the separator deleted from both templates, the
	// terminator assertion alone passed the whole package.
	// An entity that renders a space is a space. This page's whole constraint is 360 px, so
	// `&nbsp;` is a plausible way to write the separator, and calling it a run-on would assert
	// the opposite of what the browser shows.
	rendered := strings.NewReplacer("&nbsp;", " ", "&#160;", " ", "&#xA0;", " ", "&#xa0;", " ").Replace(gap)
	if rendered == "" || strings.TrimSpace(rendered) != "" {
		t.Errorf("the detail span and the self-session note are separated by %q, not by whitespace; "+
			"the cell renders as one run-on word: %q", gap, around(raw, i, 60, 40))
	}
	// #108's contract: the boundary has to be a sentence terminator, not merely a gap.
	detail = strings.TrimRight(detail, " \t\r\n")
	if detail == "" {
		t.Fatalf("the detail span rendered no text, so nothing was tested: %s", raw)
	}
	runes := []rune(detail)
	last := runes[len(runes)-1]
	switch last {
	case '.', '!', '?':
	default:
		t.Errorf("the detail cell ends with %q, not with a sentence terminator, so it runs into "+
			"the self-session note: %q", string(last), around(raw, i, 60, 40))
	}
}

// TestTheSelfSessionNoteDoesNotClaimALockThatWasRefused is the other half of that note. Its
// last clause used to read "you asked for this one, so it happened" wherever the note printed,
// and the note prints on the daemon's own row whether or not that row was lockable: a session
// with no session type is refused before the seam is asked, so the page put "not locked" in one
// column and "it happened" in the next. The clause is now gated on the row's verdict, which is
// the same rule every other sentence on this page follows.
func TestTheSelfSessionNoteDoesNotClaimALockThatWasRefused(t *testing.T) {
	f := newLockFixture(t)
	f.s.Self = platform.SelfLocation{InUserSlice: true, UID: 1000}
	// The daemon's own session, with no session type: platform refuses it before the seam is
	// asked, which is the state the note used to describe as a lock.
	f.setSessions([]platform.Session{
		{ID: "2", Username: "casey", UID: 1000, Class: "user", Seat: "seat0"},
	})
	body := flat(authPost(t, f.s, "/users/casey/lock-now", "").Body.String())
	if !strings.Contains(body, "the daemon runs inside") {
		t.Fatalf("the row rendered no self-session note, so nothing was tested: %s", body)
	}
	if strings.Contains(body, "so it happened") {
		t.Error("the note claims a lock happened on a session the daemon never asked to lock")
	}
	if !strings.Contains(body, "says what came of it") {
		t.Error("the refused self row does not send the reader to the verdict column")
	}
	if got := f.askedFor(); len(got) != 0 {
		t.Errorf("the seam was asked for sessions %v on a row that is not lockable", got)
	}
}

// TestANotLockableSessionIsNotDescribedAsAnUnconfirmedLock renders the page a greeter row
// produces -- refused before the seam was asked, so nothing is locked and nothing is
// unconfirmed -- and holds it to the rule the rest of the page follows. Nothing else in this
// file looks at that page's body: the audit test uses a greeter row and reads only the trail,
// so every conditional in the template keyed on Refused was unobserved, and widening the flag
// paragraph's condition to include refused sessions survived the whole suite. The flag speaks
// of sessions "asked to lock", and a refused session was never asked.
func TestANotLockableSessionIsNotDescribedAsAnUnconfirmedLock(t *testing.T) {
	f := newLockFixture(t)
	// A greeter row with no seat, so this is also the only page in the file that renders the
	// muted "no seat" span rather than a verdict the colour rules already cover.
	f.setSessions([]platform.Session{
		{ID: "9", Username: "casey", UID: 1000, Class: "greeter", Type: "x11"},
	})
	body := authPost(t, f.s, "/users/casey/lock-now", "").Body.String()
	if got := f.askedFor(); len(got) != 0 {
		t.Errorf("the seam was asked for sessions %v on a row that is not lockable", got)
	}
	if strings.Contains(body, `class="flag"`) {
		t.Error("the page showed the unconfirmed flag for a session that was never asked")
	}
	// Nothing here was proved, so no element on the page may wear the proved-lock colour --
	// including the seat marker, which is not a verdict cell at all and so is invisible to
	// every cell-level rule.
	if !strings.Contains(body, "no seat") {
		t.Fatal("the row rendered no seat marker, so the colour check below was vacuous")
	}
	heldIsOnlyEverALockedVerdict(t, body)
	if got, want := prose(bannerRe, t, body, "banner"), "casey was not locked."; got != want {
		t.Errorf("a refused lock's banner reads %q, want exactly %q", got, want)
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

	// The refused bucket, on its own terms. The fixture above has nothing in it -- every
	// session is lockable and unconfirmed -- so folding refused into locked passes it
	// untouched. A greeter row is refused before any backend is asked, and a trail that
	// records it as a lock is the audit log asserting something no backend ever saw.
	f2 := newLockFixture(t)
	f2.setSessions([]platform.Session{
		{ID: "2", Username: "casey", UID: 1000, Class: "user", Seat: "seat0", Type: "x11", State: "active"},
		{ID: "9", Username: "casey", UID: 1000, Class: "greeter", Seat: "seat0", Type: "x11"},
	})
	authPost(t, f2.s, "/users/casey/lock-now", "")
	trail := strings.Join(f2.audited.Lines(), "\n")
	if !strings.Contains(trail, "locked 1") || !strings.Contains(trail, "refused 1") {
		t.Errorf("the audit line does not keep a refused session out of the locked count: %s", trail)
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
