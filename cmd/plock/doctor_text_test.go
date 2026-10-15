package main

import (
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// writeTextErr is what a parent at a terminal actually reads, and it is a different renderer
// from --json: the JSON is field names for a script, this is the sentence plus the fix.
// Both have to carry the same verdict, so both need their own test -- an assertion on one
// says nothing about the other.

func TestDoctorTextReportPutsTheFixUnderTheCheckItBelongsTo(t *testing.T) {
	r := &report{
		Checks: []check{
			{Name: "root", Status: verdictOK, Detail: "running as root"},
			{Name: "configMode", Status: verdictFail, Detail: "mode 0644, want 0600", Fix: "chmod 0600 /etc/plock/config.json"},
		},
		Failed: []string{"configMode"},
	}
	var b strings.Builder
	r.writeTextErr(&b)
	got := b.String()

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	// The fix is indented under its own check. A fix printed at column 0 reads as a
	// separate finding, and with several checks on the page an operator cannot tell which
	// problem the command is telling them to solve.
	if len(lines) < 3 {
		t.Fatalf("want a line per check plus the fix, got:\n%s", got)
	}
	if !strings.HasPrefix(lines[0], "ok     root") {
		t.Errorf("first line = %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "fail   configMode") {
		t.Errorf("second line = %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "       fix: ") {
		t.Errorf("the fix is not indented under its check: %q", lines[2])
	}
	if !strings.Contains(lines[2], "chmod 0600") {
		t.Errorf("the fix text is not the command to run: %q", lines[2])
	}
	// The summary names the failing checks, because the whole point of the exit code is
	// that something is wrong and the first question after that is which.
	summary := lines[len(lines)-1]
	if !strings.Contains(summary, "1 failed") || !strings.Contains(summary, "configMode") {
		t.Errorf("summary = %q", summary)
	}
}

func TestDoctorTextReportSaysSoWhenNothingFailed(t *testing.T) {
	r := &report{Checks: []check{{Name: "root", Status: verdictOK, Detail: "running as root"}}}
	var b strings.Builder
	r.writeTextErr(&b)
	got := b.String()
	if !strings.Contains(got, "nothing failed") {
		t.Errorf("a clean report does not say so:\n%s", got)
	}
	// "0 failed, 0 warned" is technically true and reads like a bug report header. The
	// sentence a person wants is the one that lets them close the terminal.
	if strings.Contains(got, "0 failed") {
		t.Errorf("a clean report prints a zero tally instead of a sentence:\n%s", got)
	}
}

func TestDoctorTextReportCountsWarnedSeparatelyFromFailed(t *testing.T) {
	r := &report{
		Checks: []check{
			{Name: "root", Status: verdictWarn, Detail: "not root", Fix: "sudo"},
			{Name: "backends", Status: verdictFail, Detail: "none installed"},
		},
		Warned: []string{"root"},
		Failed: []string{"backends"},
	}
	var b strings.Builder
	r.writeTextErr(&b)
	got := b.String()
	if !strings.Contains(got, "1 failed, 1 warned") {
		t.Errorf("the two verdicts are not counted separately:\n%s", got)
	}
	// Only failures are listed by name: a warning is "cannot tell", and listing every
	// "cannot tell" next to the real fault makes the fault one item in a to-do list.
	if strings.Contains(got, "root, backends") || strings.Contains(got, "backends, root") {
		t.Errorf("the summary lists warnings alongside failures:\n%s", got)
	}
}

// sessionWhyUnlockable is the sentence that explains why a logged-in account was not
// locked. It is the difference between "plock is broken" and "that is a SSH session, it has
// no screen to lock", so every branch needs to be reachable and named.
func TestSessionWhyUnlockableNamesTheReason(t *testing.T) {
	for _, tc := range []struct {
		name string
		sess platform.Session
		want string
	}{
		{"remote", platform.Session{Username: "casey", Remote: true, Seat: "seat0", Type: "x11", Class: "user"}, "remote"},
		{"no seat", platform.Session{Username: "casey", Type: "x11", Class: "user"}, "no seat"},
		{"no type", platform.Session{Username: "casey", Seat: "seat0", Class: "user"}, "no session type"},
		{"manager class", platform.Session{Username: "casey", Type: "unspecified", Class: "manager", Seat: "seat0"}, "class manager"},
		// The precedence is structural-first: Remote, then seat, then type, then class. A
		// real logind manager session has no seat AND class=manager, and reports "no seat".
		// That is not a worse answer -- it is the nearer fact, and the class branch is
		// reached by the sessions that have a console but are not the user's own, which is
		// the greeter.
		{"greeter on a seat", platform.Session{Username: "lightdm", Seat: "seat0", Type: "wayland", Class: "greeter"}, "class greeter"},
		// The fallback has to exist. A session that fails the lockable test for a reason
		// this function has not been told about must not be reported as one of the four
		// named reasons, or the operator is sent chasing the wrong cause.
		{"unrecognised", platform.Session{Username: "casey", Seat: "seat0", Type: "wayland", Class: "user", State: "closing"}, "not a lockable desktop session"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sessionWhyUnlockable(tc.sess); got != tc.want {
				t.Errorf("sessionWhyUnlockable(%+v) = %q, want %q", tc.sess, got, tc.want)
			}
		})
	}
}

func TestSessionWhyUnlockableNeverReturnsAnEmptyReason(t *testing.T) {
	// The empty session is not hypothetical: a row the parser could not fill in is exactly
	// what reaches here, and an empty reason renders as "casey: " -- a blank where the
	// explanation should be, which reads as a bug in the tool rather than in the session.
	if got := sessionWhyUnlockable(platform.Session{}); strings.TrimSpace(got) == "" {
		t.Error("an unfillable session produces no reason at all")
	}
}
