package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// installLockSelfLocation says, for one test, whose login session the operator typing the
// command is sitting in. The real answer comes from /proc, so a test that inherited it would
// warn on a desktop and stay quiet on a headless runner -- the shape of mistake this project
// has now been shown several times.
func installLockSelfLocation(t *testing.T, self platform.SelfLocation) {
	t.Helper()
	prev := lockSelfLocation
	lockSelfLocation = func() platform.SelfLocation { return self }
	t.Cleanup(func() { lockSelfLocation = prev })
}

// TestLockAllWarnsWhenItIsAboutToLockYourOwnSession covers the asymmetry docs/security.md §4
// records: the enforcement loop declines the daemon's own login session and the CLI does not,
// because a lock you asked for once is one you can undo. Not locking it would break the smoke
// test for the person who means it; locking it without a word is how someone discovers the
// rule by losing their desktop mid-command.
//
// So the assertion has both halves: the warning is there, and the lock still happens.
func TestLockAllWarnsWhenItIsAboutToLockYourOwnSession(t *testing.T) {
	f := &fakeLock{sessions: []platform.Session{desktopSession("2", "casey"), desktopSession("4", "alex")}}
	path := withLockChain(t, f)
	// Derived from the fixture rather than typed, so the test keeps meaning "the operator's
	// own uid" if the fixture's uid ever moves.
	installLockSelfLocation(t, platform.SelfLocation{UID: f.sessions[0].UID, InUserSlice: true})

	code, out, errOut := runCmd(t, "lock", "--all", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d, want 0: the warning is not a refusal\n%s\n%s", code, out, errOut)
	}
	if len(f.locked) != 2 {
		t.Errorf("locked %v, want both sessions; warning about the operator's own session must not "+
			"stop the smoke test it is warning during", f.locked)
	}
	if !strings.Contains(errOut, "your own session") || !strings.Contains(errOut, "casey") {
		t.Errorf("stderr does not name the session it is about to lock:\n%s", errOut)
	}
	if !strings.Contains(errOut, "decline") {
		t.Errorf("the warning should say the daemon would decline this one, or the operator has no way "+
			"to reconcile it with the rule they read:\n%s", errOut)
	}
}

// TestLockAllStaysQuietForAnOperatorWithNoLoginSession is the control. A warning that fires on
// every --all run is noise, and noise here trains the operator to scroll past the one line
// that tells them their own screen is going dark.
func TestLockAllStaysQuietForAnOperatorWithNoLoginSession(t *testing.T) {
	f := &fakeLock{sessions: []platform.Session{desktopSession("2", "casey")}}
	path := withLockChain(t, f)
	installLockSelfLocation(t, platform.SelfLocation{})

	code, _, errOut := runCmd(t, "lock", "--all", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	if strings.Contains(errOut, "your own session") {
		t.Errorf("warned about an operator who is not inside a login session:\n%s", errOut)
	}
}

// TestLockAllStaysQuietWhenYourAccountIsNotInTheSet pins the uid comparison rather than the
// slice flag alone: being inside somebody's login session is not the same as being inside the
// session about to be locked.
func TestLockAllStaysQuietWhenYourAccountIsNotInTheSet(t *testing.T) {
	f := &fakeLock{sessions: []platform.Session{desktopSession("2", "casey")}}
	path := withLockChain(t, f)
	installLockSelfLocation(t, platform.SelfLocation{UID: f.sessions[0].UID + 1, InUserSlice: true})

	code, _, errOut := runCmd(t, "lock", "--all", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	if strings.Contains(errOut, "your own session") {
		t.Errorf("warned about a session belonging to a different uid:\n%s", errOut)
	}
}

// TestLockOneUserDoesNotWarnAboutTheNameTheyTyped pins the scope. Naming yourself is a
// decision; --all is a sweep that happens to include you, and warning on the first would make
// the warning mean "you typed your own name", which is not news.
func TestLockOneUserDoesNotWarnAboutTheNameTheyTyped(t *testing.T) {
	f := &fakeLock{sessions: []platform.Session{desktopSession("2", "casey")}}
	path := withLockChain(t, f)
	installLockSelfLocation(t, platform.SelfLocation{UID: f.sessions[0].UID, InUserSlice: true})

	code, _, errOut := runCmd(t, "lock", "casey", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	if strings.Contains(errOut, "your own session") {
		t.Errorf("warned about a session the operator named explicitly:\n%s", errOut)
	}
}

// TestLockAllWarningDoesNotPolluteJSON keeps the warning on the stream that carries warnings.
// --json is one document on stdout, and a line of prose in front of it breaks every script
// that reads the report -- which is the same reason the missing-backend warning is there too.
func TestLockAllWarningDoesNotPolluteJSON(t *testing.T) {
	f := &fakeLock{sessions: []platform.Session{desktopSession("2", "casey")}}
	path := withLockChain(t, f)
	installLockSelfLocation(t, platform.SelfLocation{UID: f.sessions[0].UID, InUserSlice: true})

	code, out, errOut := runCmd(t, "lock", "--all", "--json", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%q", err, out)
	}
	if !strings.Contains(errOut, "your own session") {
		t.Errorf("the warning is missing from stderr, which is where it belongs:\n%s", errOut)
	}
}
