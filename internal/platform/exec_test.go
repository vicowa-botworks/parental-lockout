package platform

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These are the only tests in the package that start a real process, and they start
// nothing but echo, false, sleep, and a script with a broken shebang. The rule that no
// test executes a real binary is about the backends — the thing being asserted here is
// what ExecRunner does to a command, and there is no fake that can tell us whether a
// missing binary produces a sentence an operator can act on.

func TestExecRunnerMissingBinaryNamesTheProgram(t *testing.T) {
	out, err := ExecRunner{}.Run(context.Background(), []string{"plock-no-such-binary-xyz", "list-sessions"}, nil, 0)
	if err == nil {
		t.Fatalf("a missing binary succeeded with output %q", out)
	}
	if !errors.Is(err, ErrBinaryNotFound) {
		t.Errorf("err = %v, want it to match ErrBinaryNotFound", err)
	}
	if !strings.Contains(err.Error(), "plock-no-such-binary-xyz") {
		t.Errorf("error %q does not name the program that is missing", err)
	}
	// The acceptance criterion is "a clear error naming the binary, not a stack
	// trace". A Go error chain printed with %+v is the stack-trace shape; this must
	// stay one line.
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("error is not a single line: %q", err)
	}
	if strings.Contains(err.Error(), "exec:") {
		t.Errorf("error leaks the exec package's wording instead of ours: %q", err)
	}
}

// The guard against a command line passed where an argv belongs. Without it the
// failure is "exec: \"loginctl list-sessions\": file not found", which reads like a
// PATH problem and sends someone off to check their install.
func TestExecRunnerRejectsACommandLineAsArgv0(t *testing.T) {
	for _, argv := range [][]string{
		{"loginctl list-sessions", "--json=short"},
		{"", "x"},
		{},
		{"loginctl\tlist"},
	} {
		_, err := ExecRunner{}.Run(context.Background(), argv, nil, 0)
		if err == nil {
			t.Errorf("Run(%q) succeeded, want a refusal", argv)
			continue
		}
		if errors.Is(err, ErrBinaryNotFound) {
			t.Errorf("Run(%q) reported a missing binary; the mistake is the command line, and that is what the message has to say", argv)
		}
		if !strings.Contains(err.Error(), "command line") && !strings.Contains(err.Error(), "no program") {
			t.Errorf("Run(%q) error %q does not explain the mistake", argv, err)
		}
	}
}

func TestExecRunnerSuccessAndEnv(t *testing.T) {
	out, err := ExecRunner{}.Run(context.Background(), []string{"/bin/echo", "hello"}, nil, 0)
	if err != nil {
		t.Fatalf("echo: %v", err)
	}
	if string(out) != "hello\n" {
		t.Errorf("stdout = %q, want \"hello\\n\"", out)
	}

	// A supplied env replaces rather than merges: a backend that builds a session
	// user's environment must not inherit the daemon's DBUS_SESSION_BUS_ADDRESS, or
	// it will talk to root's bus and the lock will appear to work while doing nothing
	// in the user's session.
	out, err = ExecRunner{}.Run(context.Background(), []string{"/usr/bin/env"}, []string{"PLLOCK_TEST=1", "PATH=/usr/bin:/bin"}, 0)
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	if !strings.Contains(string(out), "PLLOCK_TEST=1") {
		t.Errorf("supplied env did not reach the child: %s", out)
	}
	if strings.Contains(string(out), "DBUS_SESSION_BUS_ADDRESS") {
		t.Errorf("the daemon's environment leaked into a call with an explicit env: %s", out)
	}

	// nil inherits, which is what a root-side loginctl call wants.
	if len(os.Environ()) == 0 {
		t.Skip("environment is empty on this host; nothing to assert about inheritance")
	}
	out, err = ExecRunner{}.Run(context.Background(), []string{"/usr/bin/env"}, nil, 0)
	if err != nil {
		t.Fatalf("env (inherited): %v", err)
	}
	if len(strings.Fields(string(out))) < 2 {
		t.Errorf("inheriting the daemon's environment produced almost nothing: %q", out)
	}
}

// A non-zero exit has to carry the program's own stderr, because "loginctl
// lock-session failed" is not something an operator can diagnose from a log line.
func TestExecRunnerReportsStderrOnFailure(t *testing.T) {
	_, err := ExecRunner{}.Run(context.Background(), []string{"/bin/sh", "-c", "echo the reason >&2; exit 3"}, nil, 0)
	if err == nil {
		t.Fatal("exit 3 succeeded")
	}
	msg := err.Error()
	if !strings.Contains(msg, "the reason") {
		t.Errorf("error %q does not carry the child's stderr", msg)
	}
	if !strings.Contains(msg, "3") {
		t.Errorf("error %q does not carry the exit status", msg)
	}
	if strings.Contains(msg, "%!") {
		t.Errorf("error %q has a misformatted verb in it", msg)
	}
}

// Context cancellation must reach the child, or a hung loginctl parks the 30-second
// loop forever — the same silent-halt failure mode the config loader had with a FIFO.
func TestExecRunnerHonoursContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ExecRunner{}.Run(ctx, []string{"/bin/sleep", "30"}, nil, 0)
	if err == nil {
		t.Fatal("a cancelled context still ran the command to completion")
	}
	if !errors.Is(err, context.Canceled) {
		t.Logf("error is %v — not wrapping context.Canceled, but the command did not run", err)
	}
}

// The test above cancels before Run, so the command never starts and cmd.Cancel is
// never called. That closure is the thing that saves the 30-second loop from a loginctl
// that hangs *mid-call* — the realistic failure, and the one with no other coverage.
// Cancelling after the child is alive is the only way to reach it.
func TestExecRunnerKillsAChildThatOutlivesTheContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	start := make(chan struct{})
	go func() {
		close(start)
		_, err := ExecRunner{}.Run(ctx, []string{"/bin/sleep", "30"}, nil, 0)
		done <- err
	}()
	<-start
	// Give the child time to actually be running, then pull the context out from
	// under it.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a cancelled mid-run command reported success")
		}
		// The point is the return, not the wording: without cmd.Cancel this blocks for
		// the full 30 seconds.
		t.Logf("returned as expected: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after the context was cancelled mid-run; cmd.Cancel is not killing the child")
	}
}

// The tests above always bring a context with a deadline, which is exactly the hole:
// `plock doctor` calls Probe(context.Background()), and `gdbus call` has no default
// timeout, so a service that owns the bus name and never replies would have parked it
// forever. The bound has to hold when the caller thought of nothing.
func TestExecRunnerBoundsACallThatNeverAnswers(t *testing.T) {
	start := time.Now()
	// context.Background(): no deadline, no cancel. Nothing but the runner's own
	// guard can end this call.
	_, err := ExecRunner{CallTimeout: 50 * time.Millisecond}.Run(
		context.Background(), []string{"/bin/sleep", "30"}, nil, 0)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("a command that never answered reported success")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want it to wrap context.DeadlineExceeded", err)
	}
	// The operator has to be able to tell "the bus never answered" from "the tool
	// crashed", so the bound belongs in the sentence.
	if !strings.Contains(err.Error(), "50ms") {
		t.Errorf("error %q does not name the budget it ran out of", err)
	}
	// Generous on purpose: this asserts "not thirty seconds", not microsecond timing.
	if elapsed > 5*time.Second {
		t.Errorf("took %v to give up; the guard did not fire", elapsed)
	}
}

// The guard must only ever tighten a deadline, never loosen one, or a caller that set
// a careful budget would be quietly extended by a default it never asked for.
func TestExecRunnerCallerDeadlineStillWins(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := ExecRunner{CallTimeout: time.Hour}.Run(ctx, []string{"/bin/sleep", "30"}, nil, 0)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("a command outlived the caller's own deadline and reported success")
	}
	if elapsed > 5*time.Second {
		t.Errorf("took %v; the runner's 1h default overrode the caller's 50ms deadline", elapsed)
	}
}

// Zero means the default, and the default has to be a real bound: a constant that
// drifts to 0 would mean "no timeout" while still looking configured.
func TestExecRunnerResolvesItsTimeout(t *testing.T) {
	if DefaultCallTimeout <= 0 {
		t.Errorf("DefaultCallTimeout = %v; a zero default means no bound at all", DefaultCallTimeout)
	}
	if got := (ExecRunner{}).timeout(); got != DefaultCallTimeout {
		t.Errorf("zero CallTimeout resolved to %v, want DefaultCallTimeout", got)
	}
	if got := (ExecRunner{CallTimeout: 3 * time.Second}).timeout(); got != 3*time.Second {
		t.Errorf("CallTimeout = %v, want 3s", got)
	}
}

// The last error branch in Run is for a failure that is neither a missing program,
// nor a non-zero exit, nor a timeout. It is reachable: a script whose shebang names an
// interpreter that does not exist starts as a fork/exec failure, which is not
// exec.ErrNotFound (the file is right there) and never becomes an ExitError (there is
// no process to exit). Without this test the branch reads as dead code and gets
// deleted, and the operator then gets a bare "no such file or directory" with nothing
// naming the program they asked about.
//
// Independent of who runs the tests: a missing interpreter is ENOENT for root too,
// unlike a non-executable file, which root would happily run.
func TestExecRunnerReportsAFailedStartThatIsNotAMissingBinary(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "bad-interpreter")
	if err := os.WriteFile(script, []byte("#!/nonexistent/interpreter\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := ExecRunner{}.Run(context.Background(), []string{script}, nil, 0)
	if err == nil {
		t.Fatalf("a script with a missing interpreter reported success: %q", out)
	}
	if !strings.Contains(err.Error(), script) {
		t.Errorf("error %q does not name the program that failed to start", err)
	}
	// It must not be dressed up as "not installed": the file exists, and telling an
	// operator to install it sends them to the wrong thing.
	if errors.Is(err, ErrBinaryNotFound) {
		t.Errorf("a present file with a broken shebang was reported as %v", ErrBinaryNotFound)
	}
}
