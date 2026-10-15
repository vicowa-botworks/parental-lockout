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

// installServeRunner swaps the process seam for the duration of one test.
//
// Every serve test needs this now that the startup guard asks the backends whether
// this machine can lock anybody: without it the suite refuses to start on a headless
// runner and passes on a desktop, which is the exact defect the guard is meant to
// remove.
func installServeRunner(t *testing.T, fn serveRunnerFunc) {
	t.Helper()
	previous := serveRunner
	serveRunner = fn
	t.Cleanup(func() { serveRunner = previous })
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
func startServe(t *testing.T, extra ...string) (*syncBuffer, *syncBuffer, context.CancelFunc, <-chan int) {
	t.Helper()
	installServeRunner(t, workingLoginctl)
	dir := t.TempDir()
	cfgPath := writeServeConfig(t, dir)
	var out, errOut syncBuffer

	ctx, cancel := context.WithCancel(context.Background())
	code := make(chan int, 1)
	go func() {
		args := []string{"--config", cfgPath, "--state-dir", dir,
			"--audit-path", filepath.Join(dir, "audit.jsonl")}
		code <- runServeContext(ctx, &out, io.MultiWriter(&errOut), args)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-code:
		case <-time.After(15 * time.Second):
		}
	})
	return &out, &errOut, cancel, code
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
	go func() { done <- runServeContext(ctx, out, errOut, args) }()
	select {
	case code := <-done:
		return code
	case <-ctx.Done():
		t.Fatal("serve did not return; it was expected to refuse and exit")
		return 0
	}
}
