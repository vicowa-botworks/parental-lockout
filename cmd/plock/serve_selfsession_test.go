package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// installServeSelfLocation says, for one test, which login session the daemon is
// running inside. Without it the answer comes from /proc, so the test would pass on a
// desktop and mean something else on a headless runner -- the shape of mistake this
// project has now been shown several times.
func installServeSelfLocation(t *testing.T, uid uint32, inside bool) {
	t.Helper()
	previous := serveSelfLocation
	serveSelfLocation = func() platform.SelfLocation {
		return platform.SelfLocation{UID: uid, InUserSlice: inside}
	}
	t.Cleanup(func() { serveSelfLocation = previous })
}

// A daemon that has decided not to lock an account has to say so where the operator
// will see it. The alternative is a parent reading a dashboard that shows a child
// "enforced" while the enforcement loop is declining to act.
//
// It goes on stderr with the other things wrong about the start, and it names the fix,
// because "I will not lock this uid" with no next step is not actionable.
func TestServeSaysWhenItWillNotLockItsOwnSession(t *testing.T) {
	installServeSelfLocation(t, 1001, true)

	_, errOut, _, _ := startServe(t)
	waitFor(t, errOut, "will NOT be locked", 10*time.Second)

	got := errOut.String()
	if !strings.Contains(got, "uid 1001") {
		t.Errorf("the warning does not name which uid is exempt, so an operator cannot tell "+
			"whose account is unenforced: %q", got)
	}
	if !strings.Contains(got, "deploy/plock.service") {
		t.Errorf("the warning does not name the fix: %q", got)
	}
}

// The supported deployment must not print this. A warning that fires on every correct
// start trains the operator to scroll past it, and the day it matters they will have
// already scrolled.
func TestServeStaysQuietAboutItsOwnSessionWhenItHasNone(t *testing.T) {
	// What a systemd system unit resolves to: no user slice, so no session to skip.
	installServeSelfLocation(t, 0, false)

	_, errOut, _, _ := startServe(t)
	// Prove stderr is live before asserting absence, or this passes on a daemon that
	// wrote nothing anywhere.
	waitFor(t, errOut, "TLS IS DISABLED", 10*time.Second)

	if got := errOut.String(); strings.Contains(got, "will NOT be locked") {
		t.Errorf("a daemon with no login session warned about one; the warning would then fire "+
			"on every correct start: %q", got)
	}
}

// The dashboard is handed the same location the enforcement loop was handed, and this
// asserts it through the production assembly rather than a hand-built web.Options.
//
// The value is deliberately odd. A test that passes the zero value and asserts the zero
// value came back would pass when serve drops the field, because dropping it and
// forwarding it look identical at zero -- and zero is what a systemd unit gets anyway,
// so the drop would be invisible in the deployment where the rule never fires and fatal
// in the one where it does: the page would announce the child as locked out while the
// engine declines to lock them.
func TestServeHandsTheDashboardTheSameLocationItHandsTheEngine(t *testing.T) {
	dir := t.TempDir()
	path := writeServeConfig(t, dir)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	self := platform.SelfLocation{UID: 4123, InUserSlice: true}

	srv, err := serveServer(cfg, nil, users.Source{}, self, path, filepath.Join(dir, "state"), filepath.Join(dir, "trail.jsonl"),
		context.Background(), os.Stdout, os.Stderr)
	if err != nil {
		t.Fatalf("serveServer: %v", err)
	}
	if srv.Self != self {
		t.Errorf("the served server was handed %+v, want %+v; the enforcement loop is told "+
			"it is inside uid 4123's session and the dashboard is not, so the page can report "+
			"that account as locked out while the loop declines to lock it", srv.Self, self)
	}
}
