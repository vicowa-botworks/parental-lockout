package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// wantCommands is the contract the ticket sets: the command surface declared
// from day one. It is hard-coded on purpose — deriving it from `planned` or
// from globalUsage would let the surface shrink to nothing and keep the suite
// green, which is exactly the failure these tests exist to catch.
var wantCommands = []string{"serve", "init", "status", "lock", "unlock", "doctor", "version"}

// TestMain re-executes this test binary as the CLI so the tests can observe the
// process exit code. `run` returning the right code is not the contract;
// `main` exiting with it is.
func TestMain(m *testing.M) {
	if os.Getenv("PLOCK_TEST_ARGV") != "" {
		os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

func runCmd(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, err bytes.Buffer
	code = run(args, &out, &err)
	return code, out.String(), err.String()
}

// runProcess exercises the real binary: argv in, exit code and streams out.
func runProcess(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "PLOCK_TEST_ARGV=1")
	var out, err bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &err
	if runErr := cmd.Run(); runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			t.Fatalf("running %v: %v", args, runErr)
		}
		code = exitErr.ExitCode()
	}
	return code, out.String(), err.String()
}

func TestCommandSurfaceIsComplete(t *testing.T) {
	var stubs []string
	for command := range planned {
		stubs = append(stubs, command)
	}
	slices.Sort(stubs)

	wantStubs := slices.DeleteFunc(slices.Clone(wantCommands), func(c string) bool {
		return c == "version"
	})
	slices.Sort(wantStubs)

	if !slices.Equal(stubs, wantStubs) {
		t.Errorf("planned = %v, want %v — a command that vanishes from the surface "+
			"silently changes `plock <cmd>` from exit 1 to exit 2", stubs, wantStubs)
	}
}

func TestUsageListsExactlyTheRealCommands(t *testing.T) {
	lines := strings.Split(globalUsage, "\n")
	var listed []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasPrefix(line, "  ") &&
			slices.Contains(wantCommands, fields[0]) {
			listed = append(listed, fields[0])
		}
	}
	slices.Sort(listed)
	want := slices.Clone(wantCommands)
	slices.Sort(want)

	if !slices.Equal(listed, want) {
		t.Errorf("globalUsage lists %v, want %v", listed, want)
	}
}

func TestNoArgsIsUsageError(t *testing.T) {
	code, stdout, stderr := runCmd(t)
	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if stdout != "" {
		t.Errorf("usage must go to stderr, got stdout %q", stdout)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr = %q, want usage text", stderr)
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	for _, arg := range []string{"help", "-h", "-help", "--help"} {
		code, stdout, _ := runCmd(t, arg)
		if code != exitOK {
			t.Errorf("%s: exit code = %d, want %d", arg, code, exitOK)
		}
		for _, cmd := range wantCommands {
			if !strings.Contains(stdout, cmd) {
				t.Errorf("%s: usage does not list %q", arg, cmd)
			}
		}
	}
}

func TestHelpOnUnknownCommandIsUsageError(t *testing.T) {
	code, stdout, stderr := runCmd(t, "help", "frobnicate")
	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing on stdout for an unknown topic", stdout)
	}
	if !strings.Contains(stderr, `unknown command "frobnicate"`) {
		t.Errorf("stderr = %q, want the topic echoed", stderr)
	}
}

func TestHelpOnKnownCommandShowsItsSynopsis(t *testing.T) {
	for command, synopsis := range planned {
		code, stdout, _ := runCmd(t, "help", command)
		if code != exitOK {
			t.Errorf("help %s: exit code = %d, want %d", command, code, exitOK)
		}
		if !strings.Contains(stdout, "Usage: plock "+command) || !strings.Contains(stdout, synopsis) {
			t.Errorf("help %s: stdout = %q, want synopsis %q", command, stdout, synopsis)
		}
	}
}

func TestVersionPrintsFullIdentity(t *testing.T) {
	code, stdout, stderr := runCmd(t, "version")
	if code != exitOK {
		t.Errorf("exit code = %d, want %d (stderr %q)", code, exitOK, stderr)
	}
	if !strings.HasPrefix(stdout, "plock ") {
		t.Errorf("stdout = %q, want it to start with %q", stdout, "plock ")
	}
	if !strings.HasSuffix(stdout, "\n") {
		t.Errorf("stdout = %q, want a trailing newline", stdout)
	}
	// The ticket's acceptance criterion is version *and* commit *and* date.
	// Asserting only the prefix would let the commit and date disappear.
	for _, want := range []string{"commit ", "built "} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, missing %q", stdout, want)
		}
	}
}

func TestVersionRejectsExtraArguments(t *testing.T) {
	for _, args := range [][]string{
		{"--nonsense"},
		{"extra-arg"},
		{"--", "--nonsense"},
		{"version", "extra"},
	} {
		code, stdout, _ := runCmd(t, args...)
		if code != exitUsage {
			t.Errorf("plock %v: exit code = %d, want %d (stdout %q)", args, code, exitUsage, stdout)
		}
		if stdout != "" {
			t.Errorf("plock %v: stdout = %q, want nothing printed on a usage error", args, stdout)
		}
	}
}

func TestVersionHelpSucceeds(t *testing.T) {
	for _, flag := range []string{"-h", "-help", "--help"} {
		code, stdout, _ := runCmd(t, "version", flag)
		if code != exitOK {
			t.Errorf("version %s: exit code = %d, want %d", flag, code, exitOK)
		}
		if !strings.Contains(stdout, "Usage: plock version") {
			t.Errorf("version %s: stdout = %q, want version usage", flag, stdout)
		}
	}
}

func TestUnknownCommandIsUsageError(t *testing.T) {
	code, _, stderr := runCmd(t, "frobnicate")
	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, `unknown command "frobnicate"`) {
		t.Errorf("stderr = %q, want the command name echoed", stderr)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr = %q, want usage text after the error", stderr)
	}
}

func TestPlannedCommandsFailLoudly(t *testing.T) {
	for command := range planned {
		code, stdout, stderr := runCmd(t, command)
		if code != exitFailure {
			t.Errorf("%s: exit code = %d, want %d — a stub must never report success",
				command, code, exitFailure)
		}
		if stdout != "" {
			t.Errorf("%s: wrote to stdout %q, want only a stderr message", command, stdout)
		}
		if !strings.Contains(stderr, "not implemented") {
			t.Errorf("%s: stderr = %q, want a not-implemented message", command, stderr)
		}
	}
}

func TestPlannedCommandsDocumentTheirFlags(t *testing.T) {
	for command, synopsis := range planned {
		for _, flag := range []string{"-h", "-help", "--help"} {
			code, stdout, _ := runCmd(t, command, flag)
			if code != exitOK {
				t.Errorf("%s %s: exit code = %d, want %d", command, flag, code, exitOK)
			}
			// The synopsis is the contract later tickets implement; an empty
			// string here would silently document nothing.
			if synopsis == "" || !strings.Contains(stdout, synopsis) {
				t.Errorf("%s %s: stdout = %q, want synopsis %q", command, flag, stdout, synopsis)
			}
			if !strings.Contains(stdout, "Usage: plock "+command) {
				t.Errorf("%s %s: stdout = %q, want a usage line", command, flag, stdout)
			}
		}
	}
}

// TestHelpFlagIsOnlyRecognisedInFlagPosition guards the exit-code contract
// against the shape later tickets inherit: `--reason -h` carries -h as a value,
// and anything after `--` is data. Neither may make a command that did nothing
// exit 0.
func TestHelpFlagIsOnlyRecognisedInFlagPosition(t *testing.T) {
	cases := [][]string{
		{"lock", "--reason", "-h"},
		{"unlock", "kid", "--minutes", "5", "--", "-h"},
		{"doctor", "--try", "--help"},
		{"status", "extra", "-h"},
	}
	for _, args := range cases {
		code, stdout, _ := runCmd(t, args...)
		if code != exitFailure {
			t.Errorf("plock %v: exit code = %d, want %d — a help flag outside flag "+
				"position must not turn a no-op into success", args, code, exitFailure)
		}
		if stdout != "" {
			t.Errorf("plock %v: stdout = %q, want nothing", args, stdout)
		}
	}
}

// The tests below run the actual process, because `main` is what turns a return
// code into an exit status.

func TestProcessExitCodes(t *testing.T) {
	cases := []struct {
		args []string
		want int
	}{
		{nil, exitUsage},
		{[]string{"frobnicate"}, exitUsage},
		{[]string{""}, exitUsage},
		{[]string{"help"}, exitOK},
		{[]string{"help", "frobnicate"}, exitUsage},
		{[]string{"version"}, exitOK},
		{[]string{"version", "extra"}, exitUsage},
		{[]string{"serve"}, exitFailure},
		{[]string{"doctor"}, exitFailure},
		{[]string{"lock", "--reason", "-h"}, exitFailure},
	}
	for _, tc := range cases {
		code, _, _ := runProcess(t, tc.args...)
		if code != tc.want {
			t.Errorf("plock %v exited %d, want %d", tc.args, code, tc.want)
		}
	}
}

func TestProcessVersionOutput(t *testing.T) {
	code, stdout, stderr := runProcess(t, "version")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, exitOK, stderr)
	}
	if !strings.HasPrefix(stdout, "plock ") || !strings.Contains(stdout, "commit ") {
		t.Errorf("stdout = %q, want the full version identity", stdout)
	}
}

// TestVersionWriteFailure covers the rule that a command whose output cannot be
// written must not report success — scripts piped into a consumer depend on it.
func TestVersionWriteFailure(t *testing.T) {
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("/dev/full not available on this platform")
	}
	f, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("cannot open /dev/full: %v", err)
	}
	defer f.Close()

	code := run([]string{"version"}, f, io.Discard)
	if code == exitOK {
		t.Errorf("exit code = %d writing to /dev/full, want a failure", code)
	}
}
