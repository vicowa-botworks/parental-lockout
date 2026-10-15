package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// wantCommands is the contract the ticket sets: the command surface declared
// from day one. It is hard-coded on purpose — deriving it from `planned` or
// from globalUsage would let the surface shrink to nothing and keep the suite
// green, which is exactly the failure these tests exist to catch.
var wantCommands = []string{"serve", "init", "status", "lock", "unlock", "override", "profile", "user", "doctor", "version"}

// plockPath is the real cmd/plock binary, built once for this package, or empty
// when the source tree could not be found. The process tests exec that artifact
// rather than re-entering run() from a test binary: `main` is what turns a
// return code into an exit status, and a test that calls run() directly cannot
// notice os.Exit being dropped.
var plockPath string

// packageDir locates this package's source directory from the compile-time path
// of this file, so the build works no matter which directory the test binary is
// invoked from.
func packageDir() (string, bool) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", false
	}
	dir := filepath.Dir(file)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", false
	}
	return dir, true
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "plock-cli-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: %v\n", err)
		os.Exit(1)
	}
	bin := filepath.Join(dir, "plock")

	pkgDir, haveSrc := packageDir()
	if !haveSrc {
		// The binary is being run somewhere the sources are not (a copied test
		// binary, say). The in-process tests still mean something, so keep them
		// and say plainly that the process tests will be skipped.
		fmt.Fprintln(os.Stderr, "TestMain: cmd/plock sources not found; process-level tests will be skipped")
	} else {
		build := exec.Command("go", "build", "-o", bin, ".")
		build.Dir = pkgDir
		if out, err := build.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "TestMain: building cmd/plock: %v\n%s\n", err, out)
			os.RemoveAll(dir)
			os.Exit(1)
		}
		plockPath = bin
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func runCmd(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, err bytes.Buffer
	code = run(args, &out, &err)
	return code, out.String(), err.String()
}

// runProcess exercises the built binary: argv in, exit code and streams out.
func runProcess(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	if plockPath == "" {
		t.Skip("cmd/plock could not be built from the test binary's location")
	}
	cmd := exec.Command(plockPath, args...)
	// Hermetic: the CLI must behave the same whatever the developer's or the
	// runner's environment happens to hold.
	cmd.Env = nil
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

// implementedCommands are the commands with real behaviour behind them, so they are
// deliberately absent from `planned`. It is a list rather than a single name because
// the surface grows one entry at a time, and the day somebody forgets to move a command
// out of `planned` the test below says so instead of `plock doctor -h` printing a
// "not implemented" synopsis forever.
var implementedCommands = []string{"version", "doctor", "init", "serve", "status", "unlock", "override", "lock", "profile", "user"}

func TestCommandSurfaceIsComplete(t *testing.T) {
	var stubs []string
	for command := range planned {
		stubs = append(stubs, command)
	}
	slices.Sort(stubs)

	wantStubs := slices.DeleteFunc(slices.Clone(wantCommands), func(c string) bool {
		return slices.Contains(implementedCommands, c)
	})
	slices.Sort(wantStubs)

	if !slices.Equal(stubs, wantStubs) {
		t.Errorf("planned = %v, want %v — a command that vanishes from the surface "+
			"silently changes `plock <cmd>` from exit 1 to exit 2", stubs, wantStubs)
	}
}

// An implemented command must not also sit in `planned`: notImplemented would then
// answer before the real one, and `-h` would advertise the wrong flags.
func TestImplementedCommandsAreNotAlsoPlanned(t *testing.T) {
	for _, c := range implementedCommands {
		if synopsis, ok := planned[c]; ok {
			t.Errorf("%q is implemented but still in planned as %q, so `plock %s` reaches the stub instead of the command",
				c, synopsis, c)
		}
		if !strings.Contains(globalUsage, "\n  "+c+" ") {
			t.Errorf("%q is implemented but not listed in globalUsage", c)
		}
	}
}

// commandsIn reads the indented command list under "Commands:". Set equality is
// asserted in both directions, so the help text can neither omit a real command
// nor advertise one that does not exist.
func commandsIn(usage string) []string {
	var out []string
	inBlock := false
	for _, line := range strings.Split(usage, "\n") {
		if !inBlock {
			inBlock = line == "Commands:"
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			break
		}
		if fields := strings.Fields(line); len(fields) >= 1 {
			// One field is enough: a line that names a command but omits the
			// description column still advertises it, and the whole point of
			// this parser is to notice an advertised command that does not exist.
			out = append(out, fields[0])
		}
	}
	return out
}

func TestUsageListsExactlyTheRealCommands(t *testing.T) {
	got := slices.Clone(commandsIn(globalUsage))
	slices.Sort(got)
	want := slices.Clone(wantCommands)
	slices.Sort(want)

	if !slices.Equal(got, want) {
		t.Errorf("globalUsage lists %v, want %v", got, want)
	}
}

// TestCommandsInDetectsExtras keeps the helper above from becoming vacuous: if
// it stopped reading the block, the equality test would fail for the wrong
// reason or, worse, pass against an empty list.
func TestCommandsInDetectsExtras(t *testing.T) {
	extra := strings.Replace(globalUsage,
		"  version   print version information\n",
		"  version   print version information\n  frobnicate  pretend to work\n", 1)
	if got := commandsIn(extra); !slices.Contains(got, "frobnicate") {
		t.Errorf("commandsIn() = %v, want it to include the injected frobnicate", got)
	}

	// The same injection without a description column: `plock -h` would list a
	// command that exits 2, and a parser keyed to two-column rows would miss it.
	undescribed := strings.Replace(globalUsage,
		"  version   print version information\n",
		"  frobnicate\n  version   print version information\n", 1)
	if got := commandsIn(undescribed); !slices.Contains(got, "frobnicate") {
		t.Errorf("commandsIn() = %v, want it to include a command line with no description", got)
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

func TestHelpAboutHelpIsTheOverview(t *testing.T) {
	for _, args := range [][]string{{"help", "help"}, {"help", "-h"}, {"-h", "-h"}, {"help", "--help"}} {
		code, stdout, stderr := runCmd(t, args...)
		if code != exitOK {
			t.Errorf("plock %v: exit code = %d, want %d (stderr %q)", args, code, exitOK, stderr)
		}
		if !strings.Contains(stdout, "Usage:") {
			t.Errorf("plock %v: stdout = %q, want the overview", args, stdout)
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
	} {
		code, stdout, _ := runCmd(t, args...)
		if code != exitUsage {
			t.Errorf("plock version %v: exit code = %d, want %d (stdout %q)", args, code, exitUsage, stdout)
		}
		if stdout != "" {
			t.Errorf("plock version %v: stdout = %q, want nothing printed on a usage error", args, stdout)
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
	// Stubs: notImplemented prints its synopsis and exits 1. That exact code is the
	// stub contract, so it stays pinned.
	// Every command in the ticket's list is implemented now, so there is no stub left to
	// pin. The shape this test exists for -- a help flag outside flag position turning a
	// no-op into success -- is pinned for the real commands in their own test files.
	stubCases := [][]string{}
	for _, args := range stubCases {
		code, stdout, _ := runCmd(t, args...)
		if code != exitFailure {
			t.Errorf("plock %v: exit code = %d, want %d — a help flag outside flag "+
				"position must not turn a no-op into success", args, code, exitFailure)
		}
		if stdout != "" {
			t.Errorf("plock %v: stdout = %q, want nothing", args, stdout)
		}
	}

	// Real commands: a help flag carried as a value, or after `--`, must not print the
	// help text on stdout and must not exit 0. The code here is whatever the command's
	// own argument handling produces (2 for an unparseable flag), so only the two
	// properties the contract actually promises are asserted.
	realCases := [][]string{
		{"doctor", "--try", "--help"},
		{"doctor", "--", "--help"},
	}
	for _, args := range realCases {
		code, stdout, _ := runCmd(t, args...)
		if code == exitOK {
			t.Errorf("plock %v: exit 0 — a help flag outside flag position produced a success", args)
		}
		if strings.Contains(stdout, "usage: plock doctor") {
			t.Errorf("plock %v: printed the help text on stdout (%q) instead of reporting the argument", args, stdout)
		}
	}
}

// The tests below run the built binary, because `main` is what turns a return
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
		{[]string{"help", "help"}, exitOK},
		{[]string{"help", "frobnicate"}, exitUsage},
		{[]string{"version"}, exitOK},
		{[]string{"version", "extra"}, exitUsage},
		{[]string{"serve"}, exitFailure},
		{[]string{"doctor"}, exitFailure},
		// `lock` is implemented now, so an unknown flag is a usage error rather than the
		// stub's failure. --reason is not one of its flags at all.
		{[]string{"lock", "--reason", "-h"}, exitUsage},
		{[]string{"lock"}, exitUsage},
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

// TestWriteFailureIsReported covers the rule that a command whose output cannot
// be written must not report success — scripts piped into a consumer depend on
// it. Every output path goes through /dev/full, including the writeAll helper
// that exists solely to enforce this.
func TestWriteFailureIsReported(t *testing.T) {
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("/dev/full not available on this platform")
	}
	cases := [][]string{
		nil,
		{"-h"},
		{"help"},
		{"help", "frobnicate"},
		{"frobnicate"},
		{"version"},
		{"serve"},
		// A stub asked for help is the only path in notImplemented that returns
		// exitOK, so it is the one whose write error could pass unnoticed.
		{"serve", "-h"},
		{"lock", "--help"},
	}
	for _, args := range cases {
		f, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
		if err != nil {
			t.Skipf("cannot open /dev/full: %v", err)
		}
		code := run(args, f, f)
		f.Close()

		if code == exitOK {
			t.Errorf("plock %v to /dev/full: exit code = %d, want a failure", args, code)
		}
	}
}

func TestProcessWriteFailure(t *testing.T) {
	if plockPath == "" {
		t.Skip("cmd/plock could not be built from the test binary's location")
	}
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("/dev/full not available on this platform")
	}

	for _, args := range [][]string{{"-h"}, {"version"}, {"serve", "-h"}} {
		f, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
		if err != nil {
			t.Skipf("cannot open /dev/full: %v", err)
		}
		cmd := exec.Command(plockPath, args...)
		cmd.Env = nil
		cmd.Stdout = f
		cmd.Stderr = io.Discard
		err = cmd.Run()
		f.Close()

		if err == nil {
			t.Errorf("plock %v to /dev/full exited 0, want a failure", args)
		}
	}
}

// TestEveryCommandHasHelp closes the hole the helpTopics table opened. A command added to
// the dispatch and the overview but not to that table answers `plock <cmd> -h` with "no
// help for unknown command" -- which reads as though the command does not exist, on a tool
// people are reading help from precisely because they do not know the surface yet.
func TestEveryCommandHasHelp(t *testing.T) {
	topics := map[string]bool{}
	for _, h := range helpTopics {
		topics[h.topic] = true
	}
	for _, cmd := range wantCommands {
		if topics[cmd] {
			continue
		}
		if _, stub := planned[cmd]; stub {
			continue // a stub's synopsis is its help, until it has its own
		}
		t.Errorf("%q is on the command surface with no help topic", cmd)
	}
	// And the reverse: a help topic for a command nobody can invoke is a lie about the
	// surface.
	for topic := range topics {
		var listed bool
		for _, cmd := range wantCommands {
			if cmd == topic {
				listed = true
			}
		}
		if !listed {
			t.Errorf("help topic %q is not on the command surface", topic)
		}
	}
}

func TestHelpForEveryCommandIsNotAnError(t *testing.T) {
	for _, cmd := range wantCommands {
		code, stdout, stderr := runCmd(t, "help", cmd)
		if code != exitOK {
			t.Errorf("plock help %s: exit %d, want 0\n%s", cmd, code, stderr)
			continue
		}
		if stdout == "" {
			t.Errorf("plock help %s: empty", cmd)
		}
		if stderr != "" {
			t.Errorf("plock help %s: stderr = %q", cmd, stderr)
		}
	}
}
