package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
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
// TestServeHandsTheEngineItsLogSink pins the other thing serveEngine wires.
//
// enforce.New defaults Logf to the standard logger, because an engine left unconfigured
// would otherwise drop the three loudest things it has to say. That default is a safety net
// for a library caller, not for the daemon: a daemon whose operator redirects stdout gets
// nothing, and the lines that vanish are a tick that fell behind and a config that would not
// reload -- exactly the two a parent needs to hear when enforcement quietly stopped. So the
// sink has to arrive through the assembly, and a field assigned once at startup is only
// visible if a test can call the assembly and look.
func TestServeHandsTheEngineItsLogSink(t *testing.T) {
	dir := t.TempDir()
	path := writeServeConfig(t, dir)

	var logged strings.Builder
	eng := serveEngine(path, nil, &audit.Writer{Path: filepath.Join(dir, "trail.jsonl")},
		platform.SelfLocation{}, func(format string, args ...any) {
			fmt.Fprintf(&logged, format+"\n", args...)
		})
	if eng.Logf == nil {
		t.Fatal("the assembled engine has no log sink at all")
	}
	eng.Logf("tick %d fell behind", 3)
	if got := logged.String(); !strings.Contains(got, "tick 3 fell behind") {
		t.Errorf("the engine's diagnostics did not reach the sink serve was given; got %q, and "+
			"anything the engine logs instead went to the process's default logger, which a "+
			"redirect cannot capture", got)
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

	// The half the name has always promised and the body never checked. Issue #72 is the
	// record of what that cost: engine.Self could be deleted, the loop would run at the
	// zero value, the rule would never fire, and the daemon would lock the session it runs
	// behind -- the bug #67 was filed for -- with the suite green throughout. The loop's
	// own tests set the field by hand, so only an assembly test can see whether serve
	// actually hands it over.
	eng := serveEngine(path, nil, &audit.Writer{Path: filepath.Join(dir, "trail.jsonl")},
		self, func(string, ...any) {})
	if eng.Self != self {
		t.Errorf("the enforcement loop was handed %+v, want %+v; a dropped field here means "+
			"the daemon locks the login session it runs inside, and the dashboard still "+
			"reports that account as declined", eng.Self, self)
	}
	// One value, not two that happen to agree today. The dashboard explaining a declined
	// account the loop did not decline is the same lie in the other direction.
	if eng.Self != srv.Self {
		t.Errorf("serve hands the loop %+v and the dashboard %+v; they must be one decision",
			eng.Self, srv.Self)
	}
}
