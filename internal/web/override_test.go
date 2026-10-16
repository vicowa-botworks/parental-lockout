package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// The write path, and the reason it is tested differently from the read path.
//
// A read bug shows the parent a wrong screen. A write bug silently loses a decision they
// already made, which is worse and much harder to notice -- so every acceptance test here
// asserts the stored document, not just the status code, and every refusal test asserts that
// nothing was stored.

// overrideFixture is a server whose config seam is backed by a real in-memory document, so a
// mutation has somewhere to land and a lost write is observable rather than theoretical.
type overrideFixture struct {
	s       *Server
	logged  *logSink
	audited *logSink

	mu     sync.Mutex
	doc    *config.Config
	saves  int
	failOn error
}

func (f *overrideFixture) load() (*config.Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := *f.doc
	return &out, nil
}

func (f *overrideFixture) mutate(fn func(*config.Config) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn != nil {
		return f.failOn
	}
	if err := fn(f.doc); err != nil {
		return err
	}
	f.saves++
	return nil
}

// savesCount is read under the fixture's mutex: the seam writes it from whichever
// request goroutine reached it, so an unlocked read in a test is a data race, not a
// shortcut.
func (f *overrideFixture) savesCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.saves
}

func (f *overrideFixture) overrides() []config.Override {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]config.Override(nil), f.doc.Overrides...)
}

func newOverrideFixture(t *testing.T) *overrideFixture {
	t.Helper()
	s, logged, audited := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	m, err := AssetMap(Assets())
	if err != nil {
		t.Fatalf("assets: %v", err)
	}
	s.assets = m
	s.Now = func() time.Time { return insideWindow }
	f := &overrideFixture{s: s, logged: logged, audited: audited, doc: pageConfig()}
	s.Config = f.load
	s.Lockables = func() ([]platform.Session, error) { return nil, nil }
	s.Accounts = func() (*users.Listing, error) {
		return &users.Listing{UIDMin: users.DefaultUIDMin, UIDMax: users.DefaultUIDMax}, nil
	}
	s.Mutate = f.mutate
	return f
}

// authPost signs in for real and posts a form with this session's CSRF token, so the tests
// exercise the gate rather than routing around it.
func authPost(t *testing.T, s *Server, path, form string) *httptest.ResponseRecorder {
	t.Helper()
	resp, _ := login(t, s, "parent", "hunter2")
	c := cookieOf(t, resp)
	// The token is looked up rather than parsed out of a header, because the server mints it
	// and hands it to the page -- a test that invented one would be testing its own string.
	token, ok := s.csrf.lookup(c.Value)
	if !ok {
		t.Fatal("the login minted no CSRF token")
	}
	sep := "&"
	if form == "" {
		sep = ""
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", path,
		strings.NewReader(form+sep+"csrf="+token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(c)
	req.RemoteAddr = "192.0.2.71:1"
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestAnUnauthenticatedOverrideIsRefusedAndStoresNothing(t *testing.T) {
	f := newOverrideFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/users/casey/override",
		strings.NewReader("minutes=30&reason=homework"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.RemoteAddr = "192.0.2.71:1"
	f.s.Handler().ServeHTTP(rec, req)

	if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || loc != "/login" {
		t.Errorf("an anonymous override answered %d Location=%q, want a redirect to /login",
			rec.Code, loc)
	}
	// The status alone cannot settle this: the rejection and the grant are both 303. This is
	// the assertion that actually says the mutation did not happen.
	if got := f.overrides(); len(got) != 0 {
		t.Errorf("the anonymous request stored %d overrides, want none", len(got))
	}
}

// The CSRF gate has to be shown to cover this route rather than assumed. The middleware wraps
// the mux, so a new POST is protected -- but "so I think it is protected" is exactly what a
// route registered outside the gate would also look like from the outside.
func TestAnOverrideWithoutACSRFTokenIsRefusedAndStoresNothing(t *testing.T) {
	f := newOverrideFixture(t)
	resp, _ := login(t, f.s, "parent", "hunter2")
	c := cookieOf(t, resp)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/users/casey/override",
		strings.NewReader("minutes=30&reason=homework"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(c)
	req.RemoteAddr = "192.0.2.71:1"
	f.s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 from the CSRF gate", rec.Code)
	}
	if got := f.overrides(); len(got) != 0 {
		t.Errorf("a request the CSRF gate refused still stored %d overrides", len(got))
	}
}

func TestAGrantedOverrideIsStoredWithWhoAndWhy(t *testing.T) {
	f := newOverrideFixture(t)
	rec := authPost(t, f.s, "/users/casey/override", "minutes=30&reason=film%20night")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}

	got := f.overrides()
	if len(got) != 1 {
		t.Fatalf("stored %d overrides, want exactly 1", len(got))
	}
	o := got[0]
	if o.Username != "casey" {
		t.Errorf("override stored for %q, want casey", o.Username)
	}
	if !o.Until.Equal(insideWindow.Add(30 * time.Minute)) {
		t.Errorf("override ends %s, want %s", o.Until, insideWindow.Add(30*time.Minute))
	}
	if o.Reason != "film night" {
		t.Errorf("reason = %q, want the reason the parent typed", o.Reason)
	}
	// By is who granted it, taken from the session rather than from the form: a form field
	// is a claim the client makes, and "who let this child out" is not a question to put to
	// the client.
	if o.By != "parent" {
		t.Errorf("granted by %q, want the signed-in admin", o.By)
	}
	if got := strings.Join(f.logged.Lines(), "\n"); !strings.Contains(got, "override granted for casey") {
		t.Errorf("the grant was not logged; log was:\n%s", got)
	}

	// The trail, not just the log. "Who let this child out" gets answered from
	// audit.jsonl months later, and a request log a journald rotation has already
	// moved on is not that answer. All four parts have to be in the one line: the
	// account, the instant it stops, who granted it, and the reason they gave.
	audit := strings.Join(f.audited.Lines(), "\n")
	for _, want := range []string{"override.granted", "casey", "parent", "film night"} {
		if !strings.Contains(audit, want) {
			t.Errorf("the audit line does not carry %q; the trail was:\n%s", want, audit)
		}
	}
}

// A missing reason is refused rather than defaulted. An override nobody can explain is the
// entry the audit trail exists to settle and cannot.
func TestAnOverrideWithNoReasonIsRefusedAndNamesTheRuleThatRefused(t *testing.T) {
	f := newOverrideFixture(t)
	rec := authPost(t, f.s, "/users/casey/override", "minutes=30")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "reason") {
		t.Errorf("the refusal does not say a reason was what was missing:\n%s", body)
	}
	if got := f.overrides(); len(got) != 0 {
		t.Errorf("a refused override still stored %d entries", len(got))
	}
}

// The duration is a set, not a number. The refusal has to name the allowed values, because
// the alternative is an operator guessing at a magic constant.
func TestAnOverrideLengthOutsideTheSetIsRefusedWithTheAllowedValues(t *testing.T) {
	f := newOverrideFixture(t)
	rec := authPost(t, f.s, "/users/casey/override", "minutes=5&reason=because")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"15", "30", "60"} {
		if !strings.Contains(body, want) {
			t.Errorf("the refusal never lists %s as an allowed length:\n%s", want, body)
		}
	}
	if got := f.overrides(); len(got) != 0 {
		t.Errorf("an out-of-set override still stored %d entries", len(got))
	}
}

func TestAnOverrideForAnUnmanagedAccountIsRefused(t *testing.T) {
	f := newOverrideFixture(t)
	rec := authPost(t, f.s, "/users/root/override", "minutes=15&reason=trying")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if got := f.overrides(); len(got) != 0 {
		t.Errorf("an override for an unmanaged account was stored: %+v", got)
	}
}

// "Until the window ends" resolves to a concrete instant, and the instant is the evaluator's
// answer -- the same one the enforcement loop acts on. If the two derived it differently, a
// parent could grant an override that outlived or fell short of the window it named.
func TestUnlockingUntilTheWindowEndUsesTheEvaluatorsAnswer(t *testing.T) {
	f := newOverrideFixture(t)
	rec := authPost(t, f.s, "/users/casey/override", "scope=window&reason=early%20pickup")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}
	got := f.overrides()
	if len(got) != 1 {
		t.Fatalf("stored %d overrides, want 1", len(got))
	}

	want, ok := windowEnd(f.doc, "casey", insideWindow)
	if !ok {
		t.Fatal("the fixture's account is not inside a window; the test asserts nothing")
	}
	if !got[0].Until.Equal(want) {
		t.Errorf("override ends %s, want the evaluator's window end %s", got[0].Until, want)
	}
	// A concrete instant, not a sentinel: 0001-01-01 would mean "never" to the engine.
	if got[0].Until.IsZero() {
		t.Error("the stored end is the zero instant, which reads as no bound at all")
	}
}

// Asking for the window when there is no window is a refusal with an explanation, not a
// zero-time override that silently means "forever".
func TestUnlockingUntilAWindowThatIsNotOpenIsRefused(t *testing.T) {
	f := newOverrideFixture(t)
	f.s.Now = func() time.Time { return time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC) }
	rec := authPost(t, f.s, "/users/casey/override", "scope=window&reason=nothing%20open")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "window") {
		t.Errorf("the refusal does not say the window is what was missing:\n%s", body)
	}
	if got := f.overrides(); len(got) != 0 {
		t.Errorf("a refused window override stored %d entries", len(got))
	}
}

func TestClearingAnOverrideTakesTheChildBackUnderEnforcement(t *testing.T) {
	f := newOverrideFixture(t)
	if rec := authPost(t, f.s, "/users/casey/override", "minutes=60&reason=homework"); rec.Code != http.StatusSeeOther {
		t.Fatalf("grant: status %d", rec.Code)
	}
	if rec := authPost(t, f.s, "/users/casey/override/clear", ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("clear: status %d; body %q", rec.Code, rec.Body.String())
	}
	if got := f.overrides(); len(got) != 0 {
		t.Errorf("the override survived the clear: %+v", got)
	}
	// The audit hook is a separate sink from the request log. It is what lands in
	// /var/log/plock/audit.jsonl, and a mutation that is logged but not audited leaves the
	// trail with a hole exactly where a parent starts reading.
	if got := strings.Join(f.audited.Lines(), "\n"); !strings.Contains(got, "override.cleared") {
		t.Errorf("clearing was not audited; the audit trail was:\n%s", got)
	}
}

// Clearing when there is nothing to clear is a refusal, not a success. The operator asked for
// a change that did not happen, and 303 would say it did.
func TestClearingAnOverrideThatDoesNotExistSaysSo(t *testing.T) {
	f := newOverrideFixture(t)
	rec := authPost(t, f.s, "/users/casey/override/clear", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// The write lock is the server's, so a seam with no lock of its own cannot lose an edit.
//
// This is the property the mutex exists for, and it is the one a test of the handler alone
// cannot see: two sequential requests each pass, and the lost write needs genuine overlap.
// The seam here counts entries rather than replacing them, so a serialized pair ends with
// two and an unserialized pair ends with one -- and an unserialized pair says so the instant
// it happens, because the second entry is what the counter is watching for.
// TestTwoOverlappingMutationsDoNotLoseOne is the reason the server holds a mutex.
//
// config.Save is atomic, and it is easy to read that as the end of the story. It is not: the
// hazard is not a torn file, it is two load-mutate-save cycles that both start from the same
// document, where the second rename silently deletes the first change. So this seam behaves
// like the real one -- load a copy, mutate the copy, write the whole copy back -- and the two
// requests are held inside that window at once.
//
// With the mutex the second request cannot load until the first has saved, and both overrides
// are in the document. Without it both load the same starting document, and the file ends with
// one override: a parent granted two and the daemon kept one, with nothing on screen to say so.
func TestTwoOverlappingMutationsDoNotLoseOne(t *testing.T) {
	f := newOverrideFixture(t)
	// The second managed account is what makes a lost write visible. Both requests have
	// to name a DIFFERENT child: AddOverride replaces by username, so two grants for the
	// same account are one entry whether or not the lock exists -- a test written that
	// way fails for the wrong reason, which is worse than not failing at all.
	f.mu.Lock()
	f.doc.Users = append(f.doc.Users, config.User{Username: "alexis", ProfileID: "school", Enabled: true})
	f.mu.Unlock()

	entered := make(chan struct{})
	arrived := make(chan struct{})
	release := make(chan struct{})
	var entries sync.Once
	var inside sync.WaitGroup
	// Counting entries into the seam is the whole instrument, and where the mutex lives is
	// what makes the count meaningful: the lock is in mutateConfig, OUTSIDE this function,
	// so a second entry here can only mean the lock did not hold.
	//
	// It also means the obvious construction -- wait for the second entry, then release --
	// deadlocks on a working lock, because a working lock is exactly why the second entry
	// never happens. So the second entry is observed, not waited for.
	var entryCount atomic.Int32
	f.s.Mutate = func(fn func(*config.Config) error) error {
		entries.Do(func() { close(entered) })
		if entryCount.Add(1) == 2 {
			close(arrived)
		}
		inside.Add(1)
		defer inside.Done()

		// Load a copy, exactly as serve wires it. Holding the copy across the park is the
		// whole point: it is the stale read a real second request would otherwise not get.
		f.mu.Lock()
		doc := *f.doc
		doc.Overrides = append([]config.Override(nil), f.doc.Overrides...)
		f.mu.Unlock()

		<-release

		if err := fn(&doc); err != nil {
			return err
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.doc = &doc
		f.saves++
		return nil
	}

	// Neither request runs on the test goroutine: that goroutine is the only one left to let
	// them out, and a request that waits for the mutex cannot also be the one releasing it.
	first := make(chan int, 1)
	go func() {
		first <- authPost(t, f.s, "/users/casey/override", "minutes=15&reason=first").Code
	}()
	// Bounded, for the reason recorded on the drain test's equivalent wait: `<-entered` with no
	// deadline sat at the 900s package timeout under W25 (nothing is public, so `/login` itself
	// is gated), because the seam was never entered and `entered` therefore never closes. The
	// suite neither passed nor failed, it just occupied the run -- and the runner read that
	// timeout as a catch the test never earned. (#86)
	//
	// Which branch fires was measured, not assumed. Under W25 the deadline wins: the login
	// helper fails inside the request goroutine ("no plock_session cookie was set") and its
	// t.Fatalf ends that goroutine before anything is sent, so the response channel stays empty
	// for the whole window. The response branch covers the other shape -- a gate that answers
	// the grant instead of breaking the login -- where the status is the diagnosis.
	select {
	case <-entered:
	case code := <-first:
		t.Fatalf("the first grant never reached the write seam, so there was nothing to overlap: status=%d", code)
	case <-time.After(10 * time.Second):
		t.Fatal("the first request never reached the write seam within 10s")
	}
	second := make(chan int, 1)
	go func() {
		second <- authPost(t, f.s, "/users/alexis/override", "minutes=30&reason=second").Code
	}()

	// Did the two requests end up inside the seam together?
	//
	// This is a bounded observation, not a timing assumption, and the difference is which
	// outcome depends on speed. When the lock is missing, both goroutines park inside the
	// seam on their own and `arrived` closes in microseconds -- the race runtime slowing the
	// login round-trip does not help, because nothing has to happen quickly for the overlap
	// to be seen. When the lock works, the second request is parked on the mutex and
	// `arrived` never closes at all, so the window expiring IS the pass case.
	//
	// The version this replaces slept for 50 ms and then released, which needed the overlap
	// to happen before the release. Under `go test -race` -- what `make verify` runs -- the
	// slowed-down login overran the sleep, the two requests never overlapped, and the test
	// passed while checking nothing.
	overlap := false
	select {
	case <-arrived:
		overlap = true
	case <-time.After(time.Second):
	}
	close(release)
	if overlap {
		t.Error("two mutateConfig calls ran inside the seam at the same time; the server's " +
			"write lock does not serialize them")
	}

	// Drain both responses before sampling anything shared: the seam finishing is not the
	// request finishing, and the handler still audits and logs after it returns.
	if code := <-first; code != http.StatusSeeOther {
		t.Errorf("first request status = %d, want 303", code)
	}
	if code := <-second; code != http.StatusSeeOther {
		t.Errorf("second request status = %d, want 303", code)
	}
	inside.Wait()

	got := f.overrides()
	if len(got) != 2 {
		t.Errorf("the document holds %d overrides after two grants, want 2 -- a lost write "+
			"leaves one, because the second save was built from a document that predates the "+
			"first: %+v", len(got), got)
	}
}

// TestAnUnmanagedAccountIsRefusedWithoutTakingTheWriteLock is the only thing that separates
// the two account checks.
//
// The check inside the mutation is the one that is correct under concurrency, and it answers a
// mistyped name with the same 404 and the same sentence, so a response-level assertion cannot
// tell whether the outer check ran at all. What only the outer check provides is refusing
// before the mutex: without it, every request for an account that does not exist queues behind
// whatever write is in flight, and one mistyped bookmark stalls the next parent's real grant.
func TestAnUnmanagedAccountIsRefusedWithoutTakingTheWriteLock(t *testing.T) {
	f := newOverrideFixture(t)

	// A write that holds the server's serialization until the test lets it go.
	occupied := make(chan struct{})
	release := make(chan struct{})
	f.s.Mutate = func(fn func(*config.Config) error) error {
		close(occupied)
		<-release
		return fn(f.doc)
	}

	busy := make(chan int, 1)
	go func() {
		busy <- authPost(t, f.s, "/users/casey/override", "minutes=15&reason=holding+the+lock").Code
	}()
	// The same bound, and it hung the same way: under W25 the holding grant is gated before it
	// reaches the seam, `occupied` never closes, and the test waits on a precondition that has
	// already failed. See the note on `<-entered` above for which branch fires and why. (#86)
	select {
	case <-occupied:
	case code := <-busy:
		t.Fatalf("the holding grant never reached the write seam, so no write was in flight: status=%d", code)
	case <-time.After(10 * time.Second):
		t.Fatal("the holding request never reached the write seam within 10s")
	}

	// The same request the earlier test makes, now while a write is in flight. With the outer
	// check it answers immediately; without it, it waits for `release`.
	done := make(chan int, 1)
	go func() {
		done <- authPost(t, f.s, "/users/nobody/override", "minutes=15&reason=typo").Code
	}()

	select {
	case code := <-done:
		if code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", code)
		}
	case <-time.After(2 * time.Second):
		t.Error("a request for an unmanaged account waited for the write lock instead of " +
			"being refused up front; the pre-lock account check is what keeps a typo from " +
			"queueing behind somebody else's write")
	}

	close(release)
	if code := <-busy; code != http.StatusSeeOther {
		t.Errorf("the queued grant status = %d, want 303", code)
	}
}

// A server with no write path refuses instead of pretending. The read seams established the
// rule: a missing seam is an error the page says out loud, never a silent success.
func TestAnOverrideWithNoWritePathRefusesRatherThanPretending(t *testing.T) {
	f := newOverrideFixture(t)
	f.s.Mutate = nil
	rec := authPost(t, f.s, "/users/casey/override", "minutes=15&reason=whatever")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503; a server that cannot write must not answer 303", rec.Code)
	}
	if got := f.overrides(); len(got) != 0 {
		t.Errorf("a server with no write path stored %+v", got)
	}
}

// A save failure is reported as the daemon's problem, not as a successful grant.
func TestAFailedSaveIsNotReportedAsAGrant(t *testing.T) {
	f := newOverrideFixture(t)
	f.failOn = errors.New("the disk said no")
	rec := authPost(t, f.s, "/users/casey/override", "minutes=15&reason=whatever")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	if strings.Contains(strings.Join(f.logged.Lines(), "\n"), "override granted") {
		t.Error("a failed save was logged as a grant")
	}
}

// The account check is repeated inside the write lock, because the document read outside it
// can be stale. Reaching that branch through a real race is not something a test should
// depend on, so the seam is driven directly with a document that lost the account.
func TestAnAccountThatVanishedUnderTheLockIsRefused(t *testing.T) {
	f := newOverrideFixture(t)
	// The seam hands the handler a document with no managed users, which is what a
	// concurrent `user rm` would leave behind.
	f.s.Mutate = func(fn func(*config.Config) error) error {
		empty := config.Default()
		return fn(empty)
	}
	// The outer check uses Config, which still knows casey, so the request reaches the
	// mutation and the inner check is what refuses.
	rec := authPost(t, f.s, "/users/casey/override", "minutes=15&reason=now%20or%20never")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 from the check inside the write lock; body %q",
			rec.Code, rec.Body.String())
	}
}

func TestWindowEndAgreesWithTheEvaluator(t *testing.T) {
	c := pageConfig()
	end, ok := windowEnd(c, "casey", insideWindow)
	if !ok {
		t.Fatal("casey is inside the fixture's window at insideWindow; the helper says not")
	}
	if end.Before(insideWindow) {
		t.Errorf("window end %s is before the instant it was read at, %s", end, insideWindow)
	}
	if _, ok := windowEnd(c, "nobody", insideWindow); ok {
		t.Error("an account with no config entry reported a window")
	}
}
