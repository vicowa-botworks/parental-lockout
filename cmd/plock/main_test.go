package main

import (
	"bytes"
	"strings"
	"testing"
)

func runCmd(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, err bytes.Buffer
	code = run(args, &out, &err)
	return code, out.String(), err.String()
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
	for _, arg := range []string{"help", "-h", "--help"} {
		code, stdout, _ := runCmd(t, arg)
		if code != exitOK {
			t.Errorf("%s: exit code = %d, want %d", arg, code, exitOK)
		}
		for _, cmd := range []string{"serve", "init", "status", "lock", "unlock", "doctor", "version"} {
			if !strings.Contains(stdout, cmd) {
				t.Errorf("%s: usage does not list %q", arg, cmd)
			}
		}
	}
}

func TestVersionPrintsIdentity(t *testing.T) {
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
}

func TestVersionRejectsUnknownFlag(t *testing.T) {
	code, _, _ := runCmd(t, "version", "--nonsense")
	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
}

func TestVersionHelpSucceeds(t *testing.T) {
	code, _, stderr := runCmd(t, "version", "-h")
	if code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stderr, "Usage: plock version") {
		t.Errorf("stderr = %q, want version usage", stderr)
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
		for _, help := range []string{"-h", "--help"} {
			code, _, stderr := runCmd(t, command, help)
			if code != exitOK {
				t.Errorf("%s %s: exit code = %d, want %d", command, help, code, exitOK)
			}
			if !strings.Contains(stderr, "Usage: plock "+command) {
				t.Errorf("%s %s: stderr = %q, want a usage line", command, help, stderr)
			}
			// The synopsis is the contract later tickets implement; a stray
			// empty string here would silently document nothing.
			if synopsis == "" || !strings.Contains(stderr, synopsis) {
				t.Errorf("%s %s: stderr = %q, want synopsis %q", command, help, stderr, synopsis)
			}
		}
	}
}
