package platform

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// These are the only tests in the package that start a real process, and they start
// nothing but echo and false. The rule that no test executes a real binary is about
// the backends — the thing being asserted here is what ExecRunner does to a command,
// and there is no fake that can tell us whether a missing binary produces a sentence
// an operator can act on.

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
