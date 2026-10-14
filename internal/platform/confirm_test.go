package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMain makes the confirmation settle instant for the whole package.
//
// The settle window is a second of real waiting in production, and several tests walk
// the "asked but never confirmed" path; making them sleep would add seconds per case
// and prove nothing about the clock. What those tests need is the retry logic, so the
// park becomes instant here, and sleepCtx's own two branches are pinned directly by
// TestSleepCtxBothBranches rather than being left untested behind the substitution.
func TestMain(m *testing.M) {
	// A process re-exec'd to be part of a forged process tree never runs the suite. See
	// locker_forged_test.go, which needs a real /proc entry whose argv[0] is a lie.
	if os.Getenv(forgedEnv) == "child" {
		runForgedParent()
		return
	}
	settleWait = func(context.Context, time.Duration) bool { return true }
	os.Exit(m.Run())
}

// flipConfirmer answers "is it locked?" from a counter, so a test can say "the desktop
// sets the hint on the third read" — which is exactly the race the settle exists for.
type flipConfirmer struct {
	*Mock
	reads    int
	trueFrom int   // answer yes from this read onward; 0 means never
	errOn    int   // fail on this read; 0 means never
	err      error // what to fail with
}

func (f *flipConfirmer) LockConfirmed(context.Context, Session) (bool, error) {
	f.reads++
	if f.errOn != 0 && f.reads == f.errOn {
		return false, f.err
	}
	return f.trueFrom != 0 && f.reads >= f.trueFrom, nil
}

func TestChainLockWaitsForAnAsynchronousConfirmation(t *testing.T) {
	m := &Mock{BackendName: "logind"}
	f := &flipConfirmer{Mock: m, trueFrom: 3}
	c := &Chain{backends: []Platform{f}}

	if err := c.Lock(context.Background(), desktopSession("3")); err != nil {
		t.Fatalf("Lock: %v — the lock landed on the third read, which is inside the window", err)
	}
	if f.reads < 3 {
		t.Errorf("confirmation read %d time(s), want at least 3: the retry is the whole point", f.reads)
	}
	if n := m.LockCount(); n != 1 {
		t.Errorf("the backend was asked to lock %d time(s), want 1", n)
	}
}

// Without the settle this is the failure the reviewer measured: the request succeeds,
// the hint lands a few milliseconds later, and the chain reports the lock as failed —
// then goes on to ask the next backend, which locks it a second time.
func TestChainLockDoesNotGiveUpOnTheFirstNegative(t *testing.T) {
	m := &Mock{BackendName: "logind"}
	second := &confirming{Mock: &Mock{BackendName: "kde"}, yes: false}
	c := &Chain{backends: []Platform{&flipConfirmer{Mock: m, trueFrom: 2}, second}}

	if err := c.Lock(context.Background(), desktopSession("3")); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if n := second.LockCount(); n != 0 {
		t.Errorf("the second backend was asked to lock %d time(s) after the first one confirmed; the chain stopped too late", n)
	}
}

func TestChainLockGivesUpAfterTheSettleWindow(t *testing.T) {
	m := &Mock{BackendName: "logind"}
	f := &flipConfirmer{Mock: m} // never confirms
	c := &Chain{backends: []Platform{f}}

	err := c.Lock(context.Background(), desktopSession("3"))
	if !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("Lock = %v, want ErrUnconfirmed", err)
	}
	// Bounded: the state read before anyone was asked, plus the first read of the
	// settle window, plus the window's worth of retries, and no more. A settle with no
	// bound would park the enforcement loop on a machine where the desktop never
	// answers.
	if want := 2 + confirmReads; f.reads != want {
		t.Errorf("confirmation read %d time(s), want exactly %d: the window must be bounded", f.reads, want)
	}
}

func TestChainLockDoesNotRetryAnError(t *testing.T) {
	m := &Mock{BackendName: "logind"}
	boom := errors.New("dbus gone")
	// errOn 2, not 1: the first read is the state Chain.Lock takes before asking
	// anybody, and an error spent there is spent on a different question. What this
	// test is about is the settle window not retrying, so the failure has to land on
	// the first read of the window. TestChainLockDiscountsAYesWhoseBeforeStateIsUnreadable
	// pins the pre-read's own failure.
	f := &flipConfirmer{Mock: m, errOn: 2, err: boom}
	c := &Chain{backends: []Platform{f}}

	err := c.Lock(context.Background(), desktopSession("3"))
	if err == nil {
		t.Fatal("Lock succeeded while the confirmation query was failing")
	}
	if !errors.Is(err, boom) {
		t.Errorf("error %q loses why the check failed", err)
	}
	if f.reads != 2 {
		t.Errorf("confirmation read %d time(s), want 2: a backend that cannot be queried will not start answering between polls", f.reads)
	}
}

// The same rule has to hold for a read that fails *during* the settle window, and that
// is a different branch: the first read is answered by the guard before the loop, so a
// test that only ever fails read 1 leaves the in-loop check completely unpinned. That is
// exactly how this mutant survived its first battery -- the behaviour looked covered,
// and the one line that decides it was never executed with an error.
//
// What the discard costs is the reason. The loop's fall-through returns `false, nil`, so
// a backend that starts failing mid-settle reports "not confirmed" with nothing attached,
// and a session whose bus died becomes indistinguishable from a desktop that is merely
// slow to write LockedHint. Those need different follow-ups: one is a machine to fix, the
// other is a millisecond.
func TestChainLockSurfacesAnErrorThatArrivesMidSettle(t *testing.T) {
	m := &Mock{BackendName: "logind"}
	boom := errors.New("connection closed by bus")
	// Read 1 is the state Chain.Lock takes before asking anybody and read 2 is the
	// first read of the settle window; both answer "not yet", so the retry loop starts
	// and read 3 fails *inside* it. The distinction is the whole of this test: an error
	// on the pre-loop read is returned by a different statement, and TestChainLockDoesNotRetryAnError
	// covers that one. Aiming both at the same read would leave the loop's own error
	// path unpinned -- it is the one that turns a broken bus into silence.
	f := &flipConfirmer{Mock: m, errOn: 3, err: boom}
	c := &Chain{backends: []Platform{f}}

	err := c.Lock(context.Background(), desktopSession("3"))
	if err == nil {
		t.Fatal("Lock succeeded although the confirmation query failed mid-settle")
	}
	if !errors.Is(err, boom) {
		t.Errorf("error %q does not wrap the reason the check failed; a discarded error here leaves the operator with \"not confirmed\" and no way to tell a slow desktop from a broken bus", err)
	}
	// Both at once, deliberately. The lock *request* succeeded, so this genuinely is
	// "asked and could not prove it" -- the loop must keep re-arming, which is what
	// ErrUnconfirmed means. It must also say why. Losing either half is the bug: drop
	// the cause and a broken bus reads as a slow desktop; drop ErrUnconfirmed and the
	// loop stops re-arming on a screen that may well still be unlocked.
	if !errors.Is(err, ErrUnconfirmed) {
		t.Errorf("a failed confirmation stopped being ErrUnconfirmed: %q -- the loop would stop re-arming a session that may not be locked", err)
	}
	if f.reads != 3 {
		t.Errorf("confirmation read %d time(s), want 3: a failed read is an answer, and retrying it turns a hard failure into silence", f.reads)
	}
}

// A context cancelled mid-window has to stop the settle. The enforcement loop aborts a
// sweep on shutdown, and a settle that ignored the context would hold the exit open.
func TestChainLockSettleStopsOnCancelledContext(t *testing.T) {
	m := &Mock{BackendName: "logind"}
	f := &flipConfirmer{Mock: m}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// The production park, not the instant one TestMain installs: the whole point is
	// that a cancelled context wins the select against the timer. That needs a timer
	// which is NOT already ready — settleWait with a zero duration would put two
	// ready cases in front of select and Go picks one at random, which is how this
	// test flaked once every eight runs before it was ever committed.
	orig := settleWait
	settleWait = sleepCtx
	defer func() { settleWait = orig }()

	yes, err := confirmWithSettle(ctx, f, desktopSession("3"))
	if yes || err != nil {
		t.Errorf("confirmWithSettle on a cancelled context = %v,%v, want false,nil", yes, err)
	}
	if f.reads != 1 {
		t.Errorf("read %d time(s) after cancellation, want 1", f.reads)
	}
}

// settleWait is swapped out for every other test in this package, so sleepCtx — the
// thing production actually calls — would otherwise be uncovered. Both branches are
// the ones that matter: the timer firing, and the context winning the race.
func TestSleepCtxBothBranches(t *testing.T) {
	if !sleepCtx(context.Background(), time.Millisecond) {
		t.Error("sleepCtx reported a cancelled context for a timer that fired")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleepCtx(ctx, time.Hour) {
		t.Error("sleepCtx waited past a cancelled context; the settle would not abort on shutdown")
	}
}

// One backend failing to answer and another positively saying "unlocked" is the mixed
// verdict, and it was unpinned: the code returns unlocked WITH the reasons attached,
// which is the right answer but was indistinguishable from a regression that turned
// any error into an error-only verdict.
func TestChainLockConfirmedMixedErrorAndUnlocked(t *testing.T) {
	boom := errors.New("kde: no display")
	s := desktopSession("3")

	yes, err := (&Chain{backends: []Platform{
		&confirming{Mock: &Mock{BackendName: "logind"}, err: boom},
		&confirming{Mock: &Mock{BackendName: "kde"}, yes: false},
	}}).LockConfirmed(context.Background(), s)
	if yes {
		t.Error("reported locked on one failure and one unlocked")
	}
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("the unlocked verdict dropped the reason: %v", err)
	}

	// The other order must agree. A verdict that depends on which backend happened to
	// be listed first is not a verdict.
	yes2, err2 := (&Chain{backends: []Platform{
		&confirming{Mock: &Mock{BackendName: "kde"}, yes: false},
		&confirming{Mock: &Mock{BackendName: "logind"}, err: boom},
	}}).LockConfirmed(context.Background(), s)
	if yes2 || err2 == nil || !errors.Is(err2, boom) {
		t.Errorf("the same two backends in the other order gave %v,%v, want false with the reason", yes2, err2)
	}

	// And a backend that says yes still wins over one that errors.
	yes3, err3 := (&Chain{backends: []Platform{
		&confirming{Mock: &Mock{BackendName: "logind"}, err: boom},
		&confirming{Mock: &Mock{BackendName: "kde"}, yes: true},
	}}).LockConfirmed(context.Background(), s)
	if !yes3 || err3 != nil {
		t.Errorf("a backend that sees the lock was vetoed by one that errored: %v,%v", yes3, err3)
	}
}

// A raw VT is not enforceable by logind: session_send_lock only emits the Lock signal
// and returns, and LockedHint is written by the desktop environment calling
// SetLockedHint. Nothing on a VT ever calls it, so the hint stays false forever and
// loginctl still exits 0.
//
// The tempting fix is to refuse tty sessions in isLockable, and it is wrong. Refusing
// means Sessions() reports nobody for that child, the loop finds nothing to do, and the
// machine goes quiet while the child keeps typing. Asking anyway produces
// ErrUnconfirmed on every sweep — which is the same outcome for the child and a loud,
// permanent signal to the operator that this session cannot be enforced. For a daemon
// whose failure mode is silence, the loud one is the safe one.
//
// So this test pins the refusal NOT to happen, and #7 is where the verdict belongs:
// ErrUnconfirmed on a tty session is expected and must be surfaced as "unenforceable",
// not retried into the log.
func TestIsLockableKeepsARawVTSession(t *testing.T) {
	vt := Session{ID: "5", Username: "kid", UID: 1001, Class: "user",
		Seat: "seat0", Type: "tty", State: "active"}
	if !isLockable(vt) {
		t.Error("a tty session is refused: the loop would then report nothing at all for that child")
	}
	// CanLock is not a usable substitute either — it is class-based, so logind says
	// yes for a VT. Nothing in the listing distinguishes a session that will honour
	// the signal from one that will drop it.
	if got := vt.Type; got != "tty" {
		t.Errorf("type = %q", got)
	}
}

// An absent Remote property reads as "not remote", which keeps the session lockable.
//
// Both reviewers flagged this direction, and the alternative — refuse to lock when the
// property is missing — is the fail-open this package exists to prevent: a loginctl
// that never prints Remote would leave every child's session unenforced, silently.
// Remote is also ancient, so absence is far more likely to mean "this build does not
// print it" than "this is a remote session".
//
// The absence is not left invisible: Probe names every property the backend did not
// answer, which is where an operator would look. What this test does is pin the
// direction, so a future change to either half is a decision rather than a surprise.
func TestSessionsRemotePropertyAbsentIsStillLockable(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if sub(argv, 0) == "list-sessions" {
			return []byte(`[{"session":"2","uid":1001,"user":"kid","seat":"seat0","leader":9,"class":"user","tty":"tty2","idle":false,"since":null}]`), nil
		}
		// No Remote line at all — the shape of a loginctl that does not have it.
		return []byte("Name=kid\nType=x11\nState=active\nLockedHint=no\n"), nil
	}}
	got, err := newLogind(t, r).Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %v, want the session kept: dropping it would be the fail-open", got)
	}
	if got[0].Remote {
		t.Error("absent Remote read as remote")
	}
	if !isLockable(got[0]) {
		t.Error("a session whose Remote property was never reported is not lockable")
	}
	// The same session with Remote=yes in words is refused. Absence and presence
	// must not be the same question.
	r2 := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if sub(argv, 0) == "list-sessions" {
			return []byte(`[{"session":"2","uid":1001,"user":"kid","seat":"seat0","leader":9,"class":"user","tty":"tty2","idle":false,"since":null}]`), nil
		}
		return []byte("Name=kid\nType=x11\nState=active\nRemote=yes\nLockedHint=no\n"), nil
	}}
	if got2, _ := newLogind(t, r2).Sessions(context.Background()); len(got2) != 0 {
		t.Errorf("a session logind calls remote was kept: %v", got2)
	}
}

// Leader and TTY are carried through from the listing because they are the only fields
// that can answer "is this the session I am running in?". A daemon that locks its own
// session can never unlock it, and a uid policy does not catch that when the admin and
// the child share an account.
func TestSessionsCarriesLeaderAndTTY(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if sub(argv, 0) == "list-sessions" {
			return fixture(t, "list-sessions.json"), nil
		}
		return fixture(t, "show-session-desktop.txt"), nil
	}}
	got, err := newLogind(t, r).Sessions(context.Background())
	if err != nil || len(got) != 1 {
		t.Fatalf("Sessions = %v, %v", got, err)
	}
	s := got[0]
	if s.Leader == 0 {
		t.Errorf("Leader = 0; the listing carries the session leader pid and the caller needs it to recognise its own session")
	}
	if s.TTY == "" {
		t.Errorf("TTY is empty for a session the listing gives a tty for: %+v", s)
	}
}

// Probe is the one place that can say "this loginctl does not answer property X",
// because on the wire absence is invisible: an unknown property is exit 0 with no line,
// identical to a property that is set to nothing.
func TestLogindProbeNamesUnreportedProperties(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if sub(argv, 0) == "list-sessions" {
			return []byte(`[{"session":"2","uid":1001,"user":"kid","seat":"seat0","leader":9,"class":"user","tty":"tty2","idle":false,"since":null}]`), nil
		}
		// A stripped-down loginctl: no Remote, no LockedHint.
		return []byte("Name=kid\nType=x11\nState=active\n"), nil
	}}
	st := newLogind(t, r).Probe(context.Background())
	if len(st) != 1 || !st[0].Available {
		t.Fatalf("Probe = %+v, want logind available", st)
	}
	for _, want := range []string{"Remote", "LockedHint"} {
		if !strings.Contains(st[0].Detail, want) {
			t.Errorf("Probe detail %q does not name %s, which this loginctl never reported", st[0].Detail, want)
		}
	}
	// LockedHint in particular is worth a sentence: with nothing to read, no backend
	// can ever confirm, so every one of them is called on every poll.
	if !strings.Contains(st[0].Detail, "2 session") && !strings.Contains(st[0].Detail, "1 session") {
		t.Errorf("Probe detail %q lost the session count", st[0].Detail)
	}
}

// A complete backend must not come back looking degraded.
func TestLogindProbeQuietWhenEverythingIsReported(t *testing.T) {
	r := serve(t, "list-sessions.json", map[string]string{"3": "show-session-desktop.txt"})
	st := newLogind(t, r).Probe(context.Background())
	if len(st) != 1 {
		t.Fatalf("Probe = %+v", st)
	}
	if strings.Contains(st[0].Detail, "does not report") {
		t.Errorf("Probe calls a complete loginctl degraded: %q", st[0].Detail)
	}
}

// The probe's extra read must never turn a working backend into a failed one. A
// session can vanish between the listing and this call exactly as it can during a
// sweep, and "doctor says logind is broken" would be a false alarm.
func TestLogindProbeSurvivesAVanishedSession(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if sub(argv, 0) == "list-sessions" {
			return []byte(`[{"session":"9","uid":1001,"user":"kid","seat":"seat0","leader":9,"class":"user","tty":"tty2","idle":false,"since":null}]`), nil
		}
		return nil, fmt.Errorf("loginctl show-session 9 exited with 1: Failed to get path for session '9': No session '9' known")
	}}
	st := newLogind(t, r).Probe(context.Background())
	if len(st) != 1 || !st[0].Available {
		t.Fatalf("Probe = %+v, want logind still available after the session went away", st)
	}
}

// Nobody home is the healthy state, and Probe has to say so rather than reporting a
// broken backend — the same contract Sessions has, and the reason W1 mattered.
func TestLogindProbeIdleMachineIsAvailable(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if sub(argv, 0) == "list-sessions" {
			return []byte("[]"), nil
		}
		return nil, fmt.Errorf("unexpected command: %v", argv)
	}}
	st := newLogind(t, r).Probe(context.Background())
	if len(st) != 1 || !st[0].Available {
		t.Fatalf("Probe on an idle machine = %+v, want available", st)
	}
	if !strings.Contains(st[0].Detail, "0 session") {
		t.Errorf("Probe detail %q does not say the machine is idle", st[0].Detail)
	}
}
