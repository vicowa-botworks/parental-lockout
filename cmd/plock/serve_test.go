package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// syncBuffer is a bytes.Buffer a test can read while the daemon is still writing.
//
// The commands under test write from several goroutines (the request log comes from
// the server, the startup lines from the command itself), and a plain
// bytes.Buffer read concurrently is both a race and a test that passes or fails
// depending on how the scheduler felt.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// waitFor polls until the buffer contains want, so a test asserts that a line
// eventually appears rather than that it appeared before an arbitrary sleep.
func waitFor(t *testing.T, b *syncBuffer, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(b.String(), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("did not see %q within %s; output so far:\n%s", want, timeout, b.String())
}

// waitForEither waits until one of two buffers contains the line asked of it, so a
// test can wait for the daemon to reach a decision point without pre-judging which way
// it decided.
//
// It deliberately does not report which line arrived: the caller's own assertions are
// what turn the two outcomes into a verdict, and a helper that pronounced would
// replace the daemon's reason with the helper's words. It fails only when NEITHER line
// arrives, which is the one state the caller's assertions do not describe -- a daemon
// that announced a start and then wrote neither report nor refusal.
//
// Why this exists beside waitFor: waitFor pins one line, which is the right shape when
// the line asserted on is the line waited for. It is the wrong shape when the line
// waited for is printed EARLIER than the line asserted on, because then the wait
// proves only that the daemon got that far. runServeContext prints the startup line,
// then probes, and the probe's answer is what both assertions in
// TestServeStartsWhenOneBackendIsUsable read -- on stdout when a backend answered, on
// stderr when none did. Measured on run 43296841286's runner: that test failed there
// with `the probe report is missing` and a transcript holding nothing but the startup
// line, on the same commit that passed 5/5 on this box and passed the run 44 minutes
// earlier on the same branch, whose only diff touches factory scripts. The window is
// reproducible without waiting for a scheduler: a 3s sleep between the two prints in
// runServeContext fails the test with the runner's message and transcript byte for
// byte (serve_test.go:407), and the test passes with this wait in place instead.
func waitForEither(t *testing.T, a *syncBuffer, aWant string, b *syncBuffer, bWant string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(a.String(), aWant) || strings.Contains(b.String(), bWant) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("saw neither %q nor %q within %s; first stream:\n%s\nsecond stream:\n%s",
		aWant, bWant, timeout, a.String(), b.String())
}

// A config the daemon will accept, written where serve expects to find one.
// serveRunnerFunc adapts a function to platform.Runner so a test can decide what
// loginctl does rather than what the host's does.
type serveRunnerFunc func(ctx context.Context, argv []string, env []string, uid uint32) ([]byte, error)

func (f serveRunnerFunc) Run(ctx context.Context, argv, env []string, uid uint32) ([]byte, error) {
	return f(ctx, argv, env, uid)
}

// workingLoginctl answers a listing with one desktop session, which is all a probe
// needs to call this machine lockable.
//
// The table is the shape loginctl prints today: nine columns, SINCE last because it
// is the only multi-word one. The parser maps columns by header name, so this fixture
// is a statement about the names rather than about positions.
func workingLoginctl(context.Context, []string, []string, uint32) ([]byte, error) {
	return []byte("SESSION  UID USER   SEAT   LEADER CLASS TTY  IDLE SINCE\n" +
		"2        1000 casey  seat0  4001   user  tty2 no  5min\n"), nil
}

// brokenLoginctl is the machine the refusal guard exists for: the binary is there, it
// answers, and what it says is no.
func brokenLoginctl(_ context.Context, _ []string, _ []string, _ uint32) ([]byte, error) {
	return nil, errors.New("loginctl exited with 1: exit status 1")
}

// liveServeDaemons counts the daemons started through startServe that have not
// proven they stopped, and it is the question a production seam asks before going back.
//
// The question is set by cleanup order: t.Cleanup runs LIFO and startServe registers
// its wait AFTER installServeRunner, so the wait runs first and the seam restores run
// after it. A restore is therefore only safe when that wait found the daemon gone.
// When it does not, the goroutine is still running, and runServeContext reads both
// seams exactly once during its own startup -- the runner it hands the backend chain
// and the login-session location it hands the engine and the dashboard -- so a daemon
// that has not reached those reads yet takes the real platform.ExecRunner and the real
// /proc read with it: `loginctl lock-session` against whatever session is really on
// the box running the suite, with the arguments of a test that has finished. That is
// the exact thing every stub in this file exists to prevent, and `go test`'s package
// timeout does not catch it, because such a goroutine is not blocked, it is serving.
//
// So a refused restore leaves its stub installed and says nothing. The leak only ever
// runs in one direction -- a later test finds an earlier stub where production used to
// be, and every daemon a later test starts still installs the runner it asked for --
// and the message belongs to startServe's wait, which is the one place that actually
// saw the daemon running. TestSeamRestorationIsHeldWhileADaemonHasNotStopped pins what
// the refusal does, in both directions.
//
// A counter rather than a flag carried by the helper, because the two seam installers
// are called BEFORE startServe runs (installServeSelfLocation is the line above the
// call), so the guard has to be reachable from both. No test in this package calls
// t.Parallel, so one number describes one test at a time.
//
// It counts every daemon this package starts, not only the ones startServe made, and
// that is what refuseWithin's share of it is for: three tests — the broken-backend, the
// no-audit-trail-behind and the unusable-audit-path refusals — install the runner and
// reach runServeContext through refuseWithin rather than startServe, so they have no wait
// on `stopped` at all, and on its deadline arm refuseWithin reports and returns while its
// own goroutine may still be inside runServeContext. Those restores ask this same number,
// so the number has to move with those goroutines.
//
// Atomic rather than a plain int because an arm can read it while a goroutine is still
// running, and a guard that trips the race detector to protect the suite is no guard.
// "> 0" and not "== 1": a test that starts two daemons and cancels one must not restore,
// and over-counting can only ever withhold a restore, which is the recoverable direction —
// a stale stub fails the next test loudly, a real `loginctl` does not fail anything on
// this box before it locks a screen. TestMain reads the count at package exit, because a
// refused restore is silent by design and exit is the only moment the fact is visible.
var liveServeDaemons atomic.Int64

// installServeRunner swaps the process seam for the duration of one test.
//
// Every serve test needs this now that the startup guard asks the backends whether
// this machine can lock anybody: without it the suite refuses to start on a headless
// runner and passes on a desktop, which is the exact defect the guard is meant to
// remove.
//
// The restore asks liveServeDaemons first, because putting the real runner back under
// a daemon that has not proven it stopped is what turns a slow test into one test's
// daemon locking a real session.
func installServeRunner(t *testing.T, fn serveRunnerFunc) {
	t.Helper()
	previous := serveRunner
	serveRunner = fn
	t.Cleanup(func() {
		if liveServeDaemons.Load() > 0 {
			return // see liveServeDaemons: the stub stays, production stays out of reach
		}
		serveRunner = previous
	})
}

func writeServeConfig(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "config.json")
	doc := `{
	  "version": 1,
	  "settings": {"listen": "127.0.0.1:0", "tls": "off", "pollSeconds": 30,
	               "enforcement": true, "lockBackends": ["logind"]},
	  "admin": {"username": "parent",
	            "password": "pbkdf2-sha256$100000$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
	  "profiles": [{"id": "p", "name": "P", "timezone": "UTC", "rules": []}],
	  "users": []
	}`
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// startServe runs the daemon against a temp config and returns it once the line
// the test is waiting for appears.
//
// Every test that starts the daemon goes through here so none of them decides the
// daemon is "up" by sleeping.
// startServe returns both streams. The second one matters: the disabled-transport
// warning is deliberately written to stderr, so a test that reads only stdout would
// "prove" the warning works by asserting on a stream it was never written to.
//
// The exit code is the test's to take or leave. The helper's own cleanup never reads
// it, because a buffered token is one reading: a test body that takes it empties the
// buffer and the cleanup then waits on a send that already happened. Completion is
// announced on a channel the helper keeps to itself, so a test that wants the code and
// a cleanup that wants the stop ask two different questions, and a daemon that does not
// stop in time is a failure with a message rather than a silent 15 seconds.
func startServe(t *testing.T, extra ...string) (*syncBuffer, *syncBuffer, context.CancelFunc, <-chan int) {
	t.Helper()
	installServeRunner(t, workingLoginctl)
	dir := t.TempDir()
	cfgPath := writeServeConfig(t, dir)
	var out, errOut syncBuffer

	ctx, cancel := context.WithCancel(context.Background())
	code := make(chan int, 1)
	stopped := make(chan struct{})
	liveServeDaemons.Add(1)
	go func() {
		// Released BEFORE the close. The cleanup's wait returns on the close and the
		// seam restores that follow it ask liveServeDaemons; releasing after the close
		// would let a restore read a daemon that had already returned as still live,
		// and withhold a seam that was safe to restore.
		defer func() {
			liveServeDaemons.Add(-1)
			close(stopped)
		}()
		args := []string{"--config", cfgPath, "--state-dir", dir,
			"--audit-path", filepath.Join(dir, "audit.jsonl")}
		code <- runServeContext(ctx, &out, io.MultiWriter(&errOut), args)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-stopped:
		case <-time.After(15 * time.Second):
			// The arm the inline wait three tests down has always had, and the reason
			// it says something now: an overrun is not just slow, it is the one state
			// where the restores about to run below this one are unsafe.
			t.Errorf("serve was still running 15s after its context was cancelled, so its goroutine " +
				"may not have read its process seams yet: the stubs this test installed stay installed " +
				"rather than being restored to production under a live daemon. A daemon left running " +
				"this way can lock the session of whoever is running the suite.")
		}
	})
	return &out, &errOut, cancel, code
}

// canaryRunner answers with its own name, so a test can tell WHICH stub a seam holds
// rather than only that it holds a stub. Existence is not identity here: a restore that
// put the wrong value back would satisfy an assertion on "some stub", and which value a
// live daemon is left holding is the entire claim the guard makes.
func canaryRunner(name string) serveRunnerFunc {
	return func(context.Context, []string, []string, uint32) ([]byte, error) {
		return []byte(name), nil
	}
}

// heldRunner names the canary the runner seam is holding, and it never calls the thing
// it finds when that thing is the real runner: the seam exists to stand between a test
// and `loginctl`, so a read that invoked platform.ExecRunner would put a subprocess on
// the machine running the suite. The type says which it is, because production is a
// struct value and every stub this file writes is a serveRunnerFunc.
//
// A canary answers with its name; workingLoginctl answers with its session table;
// anything else reads as "".
func heldRunner() string {
	switch fn := serveRunner.(type) {
	case platform.ExecRunner:
		return "(production)"
	case serveRunnerFunc:
		out, err := fn(context.Background(), nil, nil, 0)
		if err != nil {
			return ""
		}
		return string(out)
	default:
		return ""
	}
}

// seamUID is a canary for the login-session seam. The uids below are from the 70000s,
// which no account on this machine and no container namespace owns, so a value outside
// them means the seam is not holding the stub the test installed.
type seamUID uint32

const (
	seamBaselineUID  seamUID = 70_000
	seamInstalledUID seamUID = 70_001
)

// heldUID reads the login-session seam back. Unlike heldRunner this one is safe to call
// even when it is production: selfLocation is os.ReadFile of /proc/self/cgroup and a
// parse, so the read runs no subprocess and writes nothing. A uid outside the canary
// range is therefore an answer, not a hazard -- and it is the answer that says the seam
// is not holding what the test installed.
func heldUID() seamUID {
	if serveSelfLocation == nil {
		return 0
	}
	return seamUID(serveSelfLocation().UID)
}

// seamLeg runs one seam install inside a throwaway subtest and asserts what the seam
// held once that subtest's cleanups had all run -- the only moment a restore's verdict
// is readable, because the restore IS a cleanup. The window is a subtest of its own so
// the assertion can name the leg, and so the window's own pass or fail cannot be
// confused with the verdict about it.
//
// The order is the whole trick, and it is the order a real overrun leaves behind:
// t.Cleanup runs LIFO, so the readback registered first runs LAST, the daemon released
// second runs SECOND, and the install registered last runs FIRST. The restore is thus
// attempted while a daemon is still unproven, exactly as it is when startServe's 15 s
// wait gives up with the goroutine still running. With live=false nothing was ever
// taken, which is every ordinary pass.
func seamLeg[T comparable](t *testing.T, live bool, want T, install func(*testing.T), readback func() T, why string) {
	t.Helper()
	var left T
	t.Run("window", func(t *testing.T) {
		t.Cleanup(func() { left = readback() })
		if live {
			liveServeDaemons.Add(1)
			t.Cleanup(func() { liveServeDaemons.Add(-1) })
		}
		install(t)
	})
	if left != want {
		t.Errorf("the seam reads %v at the end of the window, and it should read %v. %s", left, want, why)
	}
}

// TestSeamRestorationIsHeldWhileADaemonHasNotStopped pins the half of the helper's
// contract the wasted 15 s used to hide: startServe registers its wait AFTER the seam
// installs, so the wait runs first and the restores run after it, and on an overrun the
// daemon is still running when they do. A restore that lands then hands the real
// platform.ExecRunner -- and the real /proc read -- to a daemon whose test is over.
//
// The overrun is reproduced, not waited for: waiting for one would mean a test costing
// 15 s to prove a 15 s timeout, and the state is fully describable without it. Each
// seam has two legs and the second is why the first is evidence: a restore that never
// ran at all would satisfy the live leg just as well as a guard that works.
func TestSeamRestorationIsHeldWhileADaemonHasNotStopped(t *testing.T) {
	// Both seams are written directly below, so this is what puts the process back
	// whichever leg withholds its restore.
	previousRunner, previousLocation := serveRunner, serveSelfLocation
	t.Cleanup(func() { serveRunner, serveSelfLocation = previousRunner, previousLocation })

	// A baseline that is itself a canary, so every leg asks one question: which canary
	// does the seam hold when the cleanups are done?
	baselineRunner := func() { serveRunner = canaryRunner("baseline") }
	baselineLocation := func() {
		serveSelfLocation = func() platform.SelfLocation {
			return platform.SelfLocation{UID: uint32(seamBaselineUID)}
		}
	}

	const runnerHazard = "startServe's wait runs ahead of this restore, so on an overrun the daemon is " +
		"still running, and a runServeContext that has not built its backend chain yet takes the real " +
		"platform.ExecRunner from here: `loginctl lock-session` on the session of whoever is running the suite."

	baselineRunner()
	t.Run("the runner seam is held while a daemon has not stopped", func(t *testing.T) {
		seamLeg(t, true, "installed",
			func(t *testing.T) { installServeRunner(t, canaryRunner("installed")) }, heldRunner, runnerHazard)
	})

	baselineRunner()
	t.Run("the runner seam goes back when nothing is live", func(t *testing.T) {
		seamLeg(t, false, "baseline",
			func(t *testing.T) { installServeRunner(t, canaryRunner("installed")) }, heldRunner,
			"Nothing was live, so the restore should have run. A guard that always withheld would pass "+
				"the leg above just as well and leave every later test on a stale stub.")
	})

	baselineLocation()
	t.Run("the login-session seam is held while a daemon has not stopped", func(t *testing.T) {
		seamLeg(t, true, seamInstalledUID,
			func(t *testing.T) { installServeSelfLocation(t, uint32(seamInstalledUID), true) }, heldUID,
			"runServeContext hands the engine and the dashboard whatever this read says, so a stub dropped "+
				"under a live daemon leaves the page explaining a session the daemon never declined, and "+
				"the uid the enforcement loop locks is no longer the one the test wrote.")
	})

	baselineLocation()
	t.Run("the login-session seam goes back when nothing is live", func(t *testing.T) {
		seamLeg(t, false, seamBaselineUID,
			func(t *testing.T) { installServeSelfLocation(t, uint32(seamInstalledUID), true) }, heldUID,
			"Nothing was live, so the restore should have run, and a guard that always withheld would "+
				"pass the leg above just as well.")
	})
}

func TestServeRejectsABrokenConfigAtStartup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut syncBuffer
	code := runServeContext(context.Background(), &out, &errOut, []string{"--config", path})
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if errOut.String() == "" {
		t.Error("a broken config produced no message on stderr")
	}
}

// A config that parses but is not usable must fail here too.
//
// The distinction matters because the two look the same from the outside and are
// nothing alike to fix, and because a daemon that starts on an invalid document
// then enforces whatever partial reading it made of it.
func TestServeRejectsAnInvalidConfigAtStartup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	doc := `{"version": 1, "settings": {"listen": "", "tls": "maybe", "pollSeconds": 30}}`
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut syncBuffer
	if code := runServeContext(context.Background(), &out, &errOut, []string{"--config", path}); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestServeUsageErrorIsExitTwo(t *testing.T) {
	var out, errOut syncBuffer
	if code := runServeContext(context.Background(), &out, &errOut, []string{"--nonsense"}); code != 2 {
		t.Errorf("exit code = %d, want 2 (a usage error is not an operational failure)", code)
	}
}

// One runner failing must stop the others.
//
// This is the property the shared context exists for. A web server that keeps
// serving after the enforcement loop died is a dashboard that says everything is
// fine on a machine where nothing is being locked out — which is the failure this
// project treats as worse than a crash, because a crash is at least visible.
func TestOneRunnerFailingCancelsTheOthers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	died := errors.New("the enforcement loop stopped")
	started := make(chan struct{})
	stopped := make(chan struct{})

	err := runAll(ctx, []func(context.Context) error{
		func(context.Context) error { return died },
		func(other context.Context) error {
			close(started)
			<-other.Done()
			close(stopped)
			return other.Err()
		},
	})

	if !errors.Is(err, died) {
		t.Errorf("runAll returned %v, want the runner's own error", err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("the second runner never started")
	}
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("the surviving runner was left running after the other one died")
	}
}

// A shutdown cancels the context every runner is holding.
//
// The web server's own drain is tested in internal/web; what is tested here is the
// part that belongs to the command: that the signal handler's context is the one
// the runners hold, because the enforcement loop is not a socket and will only stop
// when the context says so.
func TestServeStopsWhenTheContextIsCancelled(t *testing.T) {
	out, _, cancel, code := startServe(t)
	waitFor(t, out, "listening on", 10*time.Second)

	cancel()
	select {
	case got := <-code:
		if got != 0 {
			t.Errorf("a clean shutdown returned exit code %d, want 0", got)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("serve did not return after the context was cancelled")
	}
}

// The daemon's whole job is enforcement, so a build that serves the UI without the
// loop is worse than one that crashes: the operator sees a healthy dashboard.
//
// This test used to assert the opposite -- that serve printed "the enforcement loop
// is not wired into this build" -- because internal/enforce was not on this branch
// yet. It is now, so the notice was deleted rather than left as insurance, and what
// is pinned here is the two ways the daemon can get this wrong.
func TestServeRunsTheEnforcementLoop(t *testing.T) {
	out, _, cancel, code := startServe(t)
	waitFor(t, out, "listening on", 10*time.Second)

	// The engine ticks immediately rather than waiting out the first period, so a
	// running loop announces itself within a second of the listener coming up.
	// Asserting on the tick rather than on a "starting" banner is deliberate: a
	// banner proves the code was reached, a tick proves the loop is running.
	waitFor(t, out, "enforce: running", 15*time.Second)

	cancel()
	select {
	case got := <-code:
		if got != 0 {
			t.Errorf("a clean shutdown with enforcement running returned %d, want 0", got)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("serve did not return after the context was cancelled")
	}
}

// Serving an admin UI that locks nothing is the failure mode this project ranks below
// crashing, so a machine with no usable backend must not come up at all.
func TestServeRefusesToStartWithNoUsableBackend(t *testing.T) {
	// platform.New never returns nil -- it returns a Chain plus an error naming what
	// it could not build -- so the premise here is a chain with no backends in it,
	// which is what an unregistered backend id produces.
	if c, _ := platform.New(config.Settings{LockBackends: []string{"definitely-not-a-backend"}}, platform.ExecRunner{}); len(c.Names()) != 0 {
		t.Skip("this host resolved a backend that should not exist")
	}

	dir := t.TempDir()
	cfgPath := writeServeConfig(t, dir)
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(raw), `"logind"`, `"definitely-not-a-backend"`, 1)
	if broken == string(raw) {
		t.Skip("the fixture config does not name logind; the substitution would be a no-op")
	}
	if err := os.WriteFile(cfgPath, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut syncBuffer
	code := refuseWithin(t, &out, &errOut, []string{
		"--config", cfgPath, "--state-dir", dir, "--audit-path", filepath.Join(dir, "audit.jsonl"),
	})
	if code != 1 {
		t.Errorf("exit code = %d, want 1: a daemon that enforces nothing must not come up", code)
	}
	if !strings.Contains(errOut.String(), "refusing to start") {
		t.Errorf("stderr does not say it refused:\n%s", errOut.String())
	}
}

// The machine the guard exists for, and the one the previous version of the guard
// let through.
//
// The config is well-formed and every named backend exists -- so the chain is not
// empty and Names() is not zero -- but loginctl answers with an error. That is a
// desktop where polkit refuses the lock, or a loginctl that is present and broken, and
// it is also every headless CI runner there is. The daemon used to come up anyway:
// it announced that enforcement was running, served the admin UI, and could lock
// nobody. A dashboard that looks healthy on exactly that machine is the failure this
// project ranks worse than crashing.
//
// The config-typo case above is the weaker test and it is kept, not merged into this
// one: it pins that an unknown id is also refused. It is not sufficient, because it
// exercises a chain with nothing in it, and a guard that merely counts names passes
// it while failing here.
func TestServeRefusesToStartWhenEveryBackendIsPresentAndBroken(t *testing.T) {
	installServeRunner(t, brokenLoginctl)
	dir := t.TempDir()
	cfgPath := writeServeConfig(t, dir)

	// The premise, checked rather than assumed: the config names a backend that
	// really exists. If this stopped being true the test would silently be the
	// config-typo case again.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("fixture config: %v", err)
	}
	if c, _ := platform.New(cfg.Settings, platform.ExecRunner{}); len(c.Names()) == 0 {
		t.Fatalf("the fixture names no constructible backend (%v); this would be testing the wrong refusal", cfg.Settings.LockBackends)
	}

	var out, errOut syncBuffer
	code := refuseWithin(t, &out, &errOut, []string{
		"--config", cfgPath, "--state-dir", dir, "--audit-path", filepath.Join(dir, "audit.jsonl"),
	})
	if code != 1 {
		t.Errorf("exit code = %d, want 1: every backend failed, so nothing can be locked", code)
	}
	if !strings.Contains(errOut.String(), "refusing to start") {
		t.Errorf("stderr does not say it refused:\n%s", errOut.String())
	}
	// The refusal must not be preceded by the announcement that enforcement is
	// running. Saying both is how a healthy-looking log gets written about a machine
	// that enforces nothing.
	if strings.Contains(out.String(), "enforce: running") {
		t.Errorf("the daemon announced enforcement and then refused:\n%s", out.String())
	}
}

// A refusal has to leave nothing behind, and the audit trail is the artifact that matters.
//
// The guard used to sit below trail.Open(), so a daemon that refused to start still
// created the audit directory and a 0-byte audit.jsonl. That is a record in the one place
// the operator looks to find out what the daemon did, on a machine where the daemon
// enforced nothing -- and the comment above the guard asserted the opposite, in words,
// with no test anywhere near it.
//
// So the assertion is on the filesystem, not on the log: the refusal is the last thing
// that happens, and the trail is not among its effects.
func TestARefusalLeavesNoAuditTrailBehind(t *testing.T) {
	installServeRunner(t, brokenLoginctl)
	dir := t.TempDir()
	cfgPath := writeServeConfig(t, dir)
	auditPath := filepath.Join(dir, "trail", "audit.jsonl")

	var out, errOut syncBuffer
	if code := refuseWithin(t, &out, &errOut, []string{
		"--config", cfgPath, "--state-dir", dir, "--audit-path", auditPath,
	}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if _, err := os.Stat(auditPath); err == nil {
		t.Errorf("the refusal created %s: a 0-byte audit file on a machine that enforced nothing", auditPath)
	} else if !os.IsNotExist(err) {
		t.Errorf("stat %s: %v", auditPath, err)
	}
	if _, err := os.Stat(filepath.Dir(auditPath)); err == nil {
		t.Errorf("the refusal created %s/: the trail directory is the daemon's, and it never opened one", filepath.Dir(auditPath))
	}
}

// The other side of the guard: a machine that can lock somebody must come up, and
// must not be talked out of it by a backend that merely is not installed.
//
// Without this the refusal could be satisfied by refusing always, which would look
// safe in the log and would take the product down with it.
func TestServeStartsWhenOneBackendIsUsable(t *testing.T) {
	// startServe installs the working runner and writes a config that names logind,
	// which is the shape being tested here; nothing needs to be passed in.
	out, errOut, cancel, code := startServe(t)
	waitFor(t, out, "starting, config", 10*time.Second)
	// The startup line comes FIRST, ahead of the probe, so waiting for it says only
	// that the daemon began starting. Both assertions below read what the probe
	// answered, and the probe answers on one stream or the other -- so the wait has to
	// be for either answer, not for the line before it. See waitForEither for the
	// measurement behind this line.
	waitForEither(t, out, "backend logind is available", errOut, "refusing to start", 10*time.Second)
	if strings.Contains(errOut.String(), "refusing to start") {
		t.Errorf("a machine with a usable backend was refused:\n%s", errOut.String())
	}
	if !strings.Contains(out.String(), "backend logind is available") {
		t.Errorf("the probe report is missing, so a partial chain could not be diagnosed:\n%s", out.String())
	}
	cancel()
	select {
	case <-code:
	case <-time.After(15 * time.Second):
		t.Error("serve did not exit after cancel")
	}
}

// An audit trail that cannot be opened has to be a startup failure.
//
// The writer opens lazily, so without the explicit pre-flight this would surface as
// the first dropped event hours later -- discovered by whoever reads the gap in the
// trail next month rather than by the operator who could have fixed it.
func TestServeRefusesToStartWhenTheAuditPathIsUnusable(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeServeConfig(t, dir)
	// The backend has to be answered for, not inherited. The startup guard now sits
	// ahead of trail.Open() -- that is the point of it, so a refusal cannot leave a
	// 0-byte trail behind -- which means on a machine where no backend is usable the
	// backend refusal wins and this test never reaches the guard it is about. It passed
	// here and failed on a headless runner for exactly that reason, so the runner is
	// installed rather than the assertion loosened: the audit path is the fault under
	// test, and the backend must not be whatever the machine happens to be.
	installServeRunner(t, workingLoginctl)

	var out, errOut syncBuffer
	code := refuseWithin(t, &out, &errOut, []string{
		"--config", cfgPath, "--state-dir", dir, "--audit-path", dir,
	})
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "audit trail") {
		t.Errorf("stderr does not name the audit trail:\n%s", errOut.String())
	}
}

// tls: off is a loud warning on every start, not a quiet mode.
//
// It is asserted on the writers the caller passed rather than on the process's streams,
// because the failure mode this guards is the warning going somewhere the operator's
// redirect does not catch it. It is asserted on stderr in particular: stdout is the
// stream a daemon's operator redirects, and a warning that lands there is a warning that
// gets read by nobody.
func TestServeWarnsEveryTimeTLSIsOff(t *testing.T) {
	_, errOut, _, _ := startServe(t)
	waitFor(t, errOut, "TLS IS DISABLED", 10*time.Second)
}

// The other half of the last test. "The warning is on stderr" is only useful if it is not
// also being buried in the stream that gets redirected -- and a change that sent every
// line to both streams would satisfy the test above while proving nothing about where the
// operator looks.
func TestTheTLSSWarningIsNotOnStdout(t *testing.T) {
	out, errOut, _, _ := startServe(t)
	waitFor(t, errOut, "TLS IS DISABLED", 10*time.Second)
	// Something else must already be on stdout, or this passes for the wrong reason:
	// "the warning is not on stdout" is only evidence of a split when stdout is
	// carrying something.
	//
	// The line asserted on is the startup line, which is printed unconditionally. It
	// used to assert on any "plock serve" text, and that was true only on a machine
	// where a lock backend answered the probe -- the backend-available line is
	// conditional. On a headless runner stdout held nothing but the engine's own
	// startup, so the test failed here and passed on the author's desktop, which is
	// the shape of mistake this project keeps being shown.
	if !strings.Contains(out.String(), "starting, config") {
		t.Errorf("stdout carries no unconditional startup line, so this test cannot tell a split from a silence: %q", out.String())
	}
	if strings.Contains(out.String(), "TLS IS DISABLED") {
		t.Errorf("the warning is on stdout, the stream an operator redirects away: %q", out.String())
	}

}

// A daemon that comes up says so, and says which config it opened.
//
// Both halves are load-bearing. The announcement is the only evidence in a log that
// the process started at all, because everything else this daemon writes is an
// exception. The path is the guard against the one mistake that looks like success:
// a wrong --config still loads, still validates, and still gets written back, so the
// operator gets a healthy-looking dashboard for a document nobody meant to edit.
func TestServeAnnouncesItsStart(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeServeConfig(t, dir)
	var out, errOut syncBuffer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	code := make(chan int, 1)
	go func() {
		code <- runServeContext(ctx, &out, &errOut, []string{
			"--config", cfgPath, "--state-dir", dir,
			"--audit-path", filepath.Join(dir, "audit.jsonl"),
		})
	}()

	waitFor(t, &out, "starting, config", 10*time.Second)
	if !strings.Contains(out.String(), "plock serve") {
		t.Errorf("the startup line does not identify the program: %q", out.String())
	}
	// The exact path, not merely the word "config". The line exists to catch a start
	// against the wrong document, and an assertion that any path-looking text
	// appears would survive the line naming a file nobody asked for.
	if !strings.Contains(out.String(), cfgPath) {
		t.Errorf("the startup line does not name the config that was opened (%s): %q", cfgPath, out.String())
	}

	cancel()
	select {
	case <-code:
	case <-time.After(15 * time.Second):
		t.Error("serve did not exit after cancel")
	}
}

// The warning has to arrive through the sink the caller configured, not the default
// logger — New is where it is emitted, so a Logf assigned afterwards would miss it.
//
// The default logger stamps every line with a date, so the stamp's absence is what
// proves the line came through the caller's writer.
func TestTheTLSSWarningReachesTheConfiguredSink(t *testing.T) {
	_, errOut, _, _ := startServe(t)
	waitFor(t, errOut, "TLS IS DISABLED", 10*time.Second)

	for _, line := range strings.Split(errOut.String(), "\n") {
		if !strings.Contains(line, "TLS IS DISABLED") {
			continue
		}
		// log.Printf's prefix is "2006/01/02 15:04:05 ".
		if len(line) > 10 && line[0] == '2' && line[4] == '-' && line[7] == '-' {
			t.Errorf("the warning went to the default logger, so an operator's redirect would miss it: %q", line)
		}
	}
}

// refuseWithin runs serve and insists it returns.
//
// A refusal test that calls runServeContext with context.Background() has no way to
// fail: if serve does not refuse, it starts serving and the call never returns, so
// the test hangs until the package timeout instead of reporting the defect. The
// deadline turns "it never refused" into a failure with a name.
func refuseWithin(t *testing.T, out, errOut *syncBuffer, args []string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	done := make(chan int, 1)
	// The same count startServe's daemons move, because the deadline arm below returns
	// while this goroutine may still be inside runServeContext — and the three refusal
	// tests that come through here install the runner seam with no wait on `stopped`
	// behind them.
	//
	// Released BEFORE the code is handed over, the same order startServe's goroutine
	// uses for its close: whoever receives the code then also sees the release, and a
	// restore run by a cleanup right after this returns is never withheld by a daemon
	// that has already gone.
	liveServeDaemons.Add(1)
	go func() {
		refused := runServeContext(ctx, out, errOut, args)
		liveServeDaemons.Add(-1)
		done <- refused
	}()
	select {
	case code := <-done:
		return code
	case <-ctx.Done():
		t.Fatal("serve did not return; it was expected to refuse and exit")
		return 0
	}
}
