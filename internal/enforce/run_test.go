package enforce

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// Run is the only part of this package that touches a real timer, so these are the
// only tests that wait for anything. They wait on effects, never on a fixed sleep:
// a test that sleeps ten times longer than it needs to is a test that gets deleted
// when the suite gets slow.

// waitFor polls cond until it holds or the deadline passes. The deadline is generous
// and the poll is tight, so a healthy machine finishes in milliseconds and a loaded
// CI box still passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// startRun runs e in the background and returns a cancel func plus a channel that
// closes when Run returns.
func startRun(t *testing.T, e *Engine) (context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	// A separate signal for "Run has returned", so a test that drains done itself
	// does not leave the cleanup waiting for a value that has already been taken.
	returned := make(chan struct{})
	go func() {
		err := e.Run(ctx)
		done <- err
		close(returned)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-returned:
		case <-time.After(8 * time.Second):
			t.Error("Run did not return after its context was cancelled")
		}
	})
	return cancel, done
}

func TestRunLocksBeforeItEverWaits(t *testing.T) {
	e, p, _, _, _ := newEngine(t, kidDoc(), refInside)
	// An hour: if Run waited for the first tick, this test would take an hour.
	e.Interval = time.Hour
	e.WatchInterval = -1 // no file poller; this test is about the first tick

	p.gateEntered()
	cancel, done := startRun(t, e)

	select {
	case <-p.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("no lock within 5s: a daemon restarted during a curfew must lock on start, " +
			"not hand back a whole tick interval first")
	}

	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("Run returned %v, want context.Canceled", err)
	}
}

func TestRunKeepsTickingAtTheConfiguredPeriod(t *testing.T) {
	e, p, _, _, _ := newEngine(t, kidDoc(), refInside)
	e.Interval = 5 * time.Millisecond
	e.WatchInterval = -1

	p.gateEntered()
	cancel, _ := startRun(t, e)
	defer cancel()

	waitFor(t, "at least 5 ticks", func() bool { return p.totalLocks() >= 5 })
}

func TestRunSkipsATickThatOutlivesItsPeriod(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	e.Interval = 5 * time.Millisecond
	e.WatchInterval = -1

	gate := make(chan struct{})
	p.mu.Lock()
	p.block = gate
	p.entered = make(chan struct{})
	p.mu.Unlock()
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})

	cancel, _ := startRun(t, e)
	defer cancel()

	waitFor(t, "a skipped tick", func() bool { return e.Skipped() > 0 })

	// One tick in flight, and everything after it refused rather than stacked. Two
	// ticks locking the same session at once would double every audit line and race
	// the engine's own state map.
	if got := p.totalLocks(); got != 1 {
		t.Errorf("%d Lock calls while one was blocked, want 1: a late tick must be refused, not queued", got)
	}
	if sink.countOf(KindTickSkipped) == 0 {
		t.Error("no tick.skipped event: a daemon that falls behind leaves no trace in the audit log")
	}
	if e.Skipped() == 0 {
		t.Error("Skipped() is 0 although ticks were refused")
	}
}

func TestRunAppliesAConfigEditWithoutWaitingForTheNextTick(t *testing.T) {
	doc := kidDoc()
	// Shut at refInside, so nothing locks until the file changes.
	doc.Profiles[0].Rules = []config.Rule{{
		ID: "r-brief", Name: "Brief", Days: []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"},
		Start: "09:00", End: "10:00", Enabled: true,
	}}
	e, p, _, _, path := newEngine(t, doc, refInside)
	e.Interval = time.Hour // the tick would not come for an hour
	e.WatchInterval = 2 * time.Millisecond

	cancel, _ := startRun(t, e)
	defer cancel()

	// Give the first tick time to run and do nothing, so a lock that appears later
	// can only have come from the watcher.
	time.Sleep(50 * time.Millisecond)
	if got := p.totalLocks(); got != 0 {
		t.Fatalf("locked %d times outside the window, want 0", got)
	}

	doc.Profiles[0].Rules = []config.Rule{curfew()}
	writeDoc(t, path, doc)

	waitFor(t, "the edit to be picked up", func() bool { return p.totalLocks() > 0 })
}

// TestRunIsRaceCleanWhileTheConfigIsEdited is the -race acceptance criterion: the
// reloader and the tick running concurrently. It edits the file from one goroutine
// while the loop ticks from another, which is exactly the interleaving #10's web UI
// produces.
//
// The Lock is deliberately slow and State() is read in a tight loop. A tick that
// finishes in nanoseconds is a race that never happens, and a guard the detector
// never observes is a guard that was never tested — removing State()'s lock survived
// the first version of this test for exactly that reason.
func TestRunIsRaceCleanWhileTheConfigIsEdited(t *testing.T) {
	e, p, _, clock, path := newEngine(t, kidDoc(), refInside)
	e.Interval = time.Millisecond
	e.WatchInterval = time.Millisecond
	p.mu.Lock()
	p.slow = 2 * time.Millisecond
	p.mu.Unlock()

	p.gateEntered()
	cancel, _ := startRun(t, e)
	defer cancel()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	// Flip the rule open and shut as fast as the disk allows.
	go func() {
		defer wg.Done()
		open := kidDoc()
		shut := kidDoc()
		shut.Profiles[0].Rules = []config.Rule{{
			ID: "r-brief", Name: "Brief", Days: []string{"mon"}, Start: "09:00", End: "10:00", Enabled: true,
		}}
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			doc := open
			if i%2 == 0 {
				doc = shut
			}
			b, err := json.Marshal(doc)
			if err != nil {
				t.Errorf("marshal: %v", err)
				return
			}
			if err := os.WriteFile(path, b, 0o600); err != nil {
				t.Errorf("write: %v", err)
				return
			}
		}
	}()

	// And read the engine's own state while it is mid-tick, across the window
	// boundary, so both the map and the clock are being written underneath.
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			if i%2 == 0 {
				clock.Set(refOutside.AddDate(0, 0, 1))
			} else {
				clock.Set(refInside)
			}
			_ = e.State()
			_ = e.Skipped()
		}
	}()

	// Long enough that a tick with a 2ms Lock is certainly in flight while State()
	// is being read, and short enough to stay cheap.
	time.Sleep(250 * time.Millisecond)
	close(stop)
	wg.Wait()

	// The assertion is the absence of a race report. This only checks the loop is
	// still alive and working afterwards.
	waitFor(t, "the loop to still be ticking", func() bool { return p.totalLocks() > 0 })
}

func TestTryTickRefusesWhileATickIsInFlight(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	e.WatchInterval = -1

	gate := make(chan struct{})
	p.mu.Lock()
	p.block = gate
	p.mu.Unlock()
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})

	inFlight := make(chan Report, 1)
	go func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		inFlight <- e.Tick(context.Background())
	}()
	waitFor(t, "the first tick to reach the platform", func() bool { return p.totalLocks() == 1 })

	if _, ok := e.TryTick(context.Background()); ok {
		t.Error("TryTick ran while another tick was in flight")
	}
	// A refused manual tick is not "enforcement fell behind", so it must not
	// contaminate the counter that means that.
	if got := e.Skipped(); got != 0 {
		t.Errorf("Skipped() = %d after a refused TryTick, want 0", got)
	}
	if sink.countOf(KindTickSkipped) != 0 {
		t.Error("a refused TryTick emitted tick.skipped")
	}

	close(gate)
	<-inFlight

	if rep, ok := e.TryTick(context.Background()); !ok {
		t.Error("TryTick refused after the in-flight tick finished")
	} else if rep.Asserted != 1 {
		t.Errorf("TryTick reported Asserted = %d, want 1", rep.Asserted)
	}
}

// TestIntervalFollowsSettingsPins the wiring between the config file and the loop's
// timer. Run reads the published number rather than e.cfg, because e.cfg belongs to
// the tick goroutine — so the hand-off has to actually happen, and a test is the only
// place that is observable without waiting thirty seconds.
func TestIntervalFollowsSettings(t *testing.T) {
	doc := kidDoc()
	doc.Settings.PollSeconds = 7
	e, _, _, _, path := newEngine(t, doc, refInside)

	if got := e.interval(); got != config.DefaultPollSeconds*time.Second {
		t.Errorf("interval before any tick = %v, want the documented default", got)
	}

	tick(e)
	if got := e.interval(); got != 7*time.Second {
		t.Errorf("interval after a tick = %v, want 7s from settings.pollSeconds", got)
	}

	// A parent edits the interval; the next tick's reload moves the timer.
	doc.Settings.PollSeconds = 90
	writeDoc(t, path, doc)
	tick(e)
	if got := e.interval(); got != 90*time.Second {
		t.Errorf("interval after the edit = %v, want 90s", got)
	}
}

// TestRunStopsWhenCancelledWithNoTickInFlight is the clean-shutdown path: no
// platform call outstanding, so Run returns as soon as the context goes.
func TestRunStopsWhenCancelledWithNoTickInFlight(t *testing.T) {
	e, _, _, _, _ := newEngine(t, kidDoc(), refOutside) // no window: nothing to lock
	e.Interval = time.Millisecond
	e.WatchInterval = 2 * time.Millisecond

	cancel, done := startRun(t, e)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}

// TestStateIsSortedAndOnlyNamesEnforcedSessions keeps `plock status` honest: the
// dashboard shows this list as "who is locked out right now".
func TestStateIsSortedAndOnlyNamesEnforcedSessions(t *testing.T) {
	doc := kidDoc()
	doc.Users = append(doc.Users, config.User{Username: "kid2", ProfileID: "p-default", Enabled: true})
	e, p, _, _, _ := newEngine(t, doc, refInside)
	p.setSessions(desktop("9", "kid2"), desktop("2", "kid"))

	tick(e)

	got := e.State()
	want := []string{"2", "9"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("State() = %v, want %v (sorted, so two calls print identically)", got, want)
	}

	// A lock that never once succeeded is not an enforced lockout, and the status
	// page must not claim one.
	p.setLockErr("2", errors.New("no backend"))
	p.setLockErr("9", errors.New("no backend"))
	for i := 0; i < unenforceableAfter; i++ {
		tick(e)
	}
	if got := e.State(); len(got) != 2 {
		t.Errorf("State() = %v, want both sessions still listed", got)
	}

	// The distinction State() draws is "inside a lockout the engine is enforcing",
	// not "confirmed locked right now". Both of these sessions were locked
	// successfully at the first tick and are still inside the window, so a later
	// failure is reported as an error while the lockout stands — clearing it would
	// audit a lockout.exited that never happened and re-enter on the next tick.
	// A session whose very first lock failed is NOT listed, which is what
	// TestASessionNothingCanLockIsEscalatedOnce pins.
	if got := e.State(); got[0] != "2" || got[1] != "9" {
		t.Errorf("State() = %v, want [2 9]: the window is still open", got)
	}
}

// TestCanNeverConfirmIsOnlyTheVT keeps the escalation predicate narrow. Widening it
// to "any session that failed to confirm" would tell every KDE parent their child is
// unenforceable, because the screensaver interface exposes no Locked property and a
// genuinely locked screen can therefore read as unconfirmed forever.
func TestCanNeverConfirmIsOnlyTheVT(t *testing.T) {
	vt := desktop("1", "kid")
	vt.Type = "tty"
	if !canNeverConfirm(vt) {
		t.Error("a raw VT is not recognised as never-confirmable; its permanent ErrUnconfirmed would never be escalated")
	}
	for _, typ := range []string{"wayland", "x11", "mir", ""} {
		s := desktop("1", "kid")
		s.Type = typ
		if canNeverConfirm(s) {
			t.Errorf("a %q session is called never-confirmable; a desktop that cannot confirm today may still be locked", typ)
		}
	}
	_ = platform.Session{}
}

// TestAForcedHintOnAnUnconfirmableDesktopSaysNeitherProvesAnything covers the pair
// the two failure modes make together: the child writes LockedHint and the desktop
// cannot confirm. Neither half is evidence, and the line has to refuse both rather
// than letting the forged one read as the reason.
func TestAForcedHintOnAnUnconfirmableDesktopSaysNeitherProvesAnything(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	forged := desktop("2", "kid")
	forged.Locked = true
	p.setSessions(forged)
	p.setLockErr("2", platform.ErrUnconfirmed)

	tick(e)

	ev := sink.kindsOf(KindLockUnconfirmed)
	if len(ev) != 1 {
		t.Fatalf("got %d lock.unconfirmed events, want 1", len(ev))
	}
	if !strings.Contains(ev[0].Detail, "claimed to be locked") {
		t.Errorf("detail = %q, want it to name the hint that was ignored", ev[0].Detail)
	}
	if !strings.Contains(ev[0].Detail, "neither proves") && !strings.Contains(ev[0].Detail, "nothing confirmed") {
		t.Errorf("detail = %q, want it to say the lock is still unproven", ev[0].Detail)
	}
}

// TestTheDefaultsAreUsableWithoutAnythingInjected pins the zero-value paths of the
// seams. Every test injects a clock and a logger, so without this one nothing would
// notice if the fallbacks were dropped: the daemon in production injects neither, and
// an engine built by New would either panic or stamp events at the zero time — an
// audit trail whose every line reads 0001-01-01.
func TestTheDefaultsAreUsableWithoutAnythingInjected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	writeDoc(t, path, kidDoc())

	p := &fakePlatform{sessions: []platform.Session{desktop("2", "kid")}}
	sink := &recSink{}
	e := New(path, p, sink) // no Clock, no Logf

	if got := e.watchInterval(); got != time.Second {
		t.Errorf("watchInterval default = %v, want 1s", got)
	}

	// The real clock, so nothing here can assert a window is open — the point is
	// that the engine works with no seams at all: it enumerates, it stamps events
	// with a real time rather than the zero time, and it reloads.
	rep := tick(e)
	if rep.Sessions != 1 {
		t.Fatalf("Report.Sessions = %d, want the one session the platform listed", rep.Sessions)
	}
	if rep.At.IsZero() {
		t.Error("Report stamped the zero time: the Clock fallback is not being used")
	}
	before := time.Now().Add(-time.Minute)
	for _, ev := range rep.Events {
		if ev.At.Before(before) {
			t.Errorf("%s stamped %s, want the real clock", ev.Kind, ev.At)
		}
	}

	// A second tick that reloads the file goes through the default logger, which is
	// the only thing standing between a missing Logf and a daemon that says nothing.
	doc := kidDoc()
	doc.Settings.PollSeconds = 45
	writeDoc(t, path, doc)

	// Captured rather than assumed: the claim is that an engine built by New and
	// never configured still tells somebody when the config comes back, and the
	// somebody is the journal.
	var logged bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(previous) })

	if rep := tick(e); rep.Sessions != 1 {
		t.Errorf("second tick enumerated %d sessions, want 1", rep.Sessions)
	}
	if got := e.interval(); got != 45*time.Second {
		t.Errorf("interval = %v, want 45s from the reloaded file", got)
	}
	if !strings.Contains(logged.String(), "config readable again") {
		t.Errorf("default logger got %q, want the reload diagnostic", logged.String())
	}
}
