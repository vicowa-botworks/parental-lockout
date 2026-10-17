package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// Where the running daemon is executing, as far as the self-session rule cares.
//
// `plock status` runs as the operator, not as the daemon, so the only honest source for
// "which sessions is the engine declining?" is the daemon's own cgroup. The operator's
// own slice answers a different question and gets the opposite answer in both
// directions: an operator at a desktop with the daemon installed as a system unit would
// see every one of their own sessions marked declined, and an operator in an SSH session
// with the daemon started from the desktop would see none. Both are reports that assert
// the reverse of what the engine does, on the surface people paste into a bug report.
type daemonState struct {
	// SystemdKnown is false when systemctl could not be executed at all. It is kept
	// apart from Running because "no init system here" and "the service is not
	// running" are different sentences, and collapsing them is how a diagnostic
	// invents the problem it was asked to look for.
	SystemdKnown bool
	// Running is the unit having a main process right now — the only thing the
	// self-session rule can attach to.
	Running bool
	// LocationKnown is false when the daemon is running but its cgroup could not be
	// read. Callers must not read that as "declines nobody": it is the same shape as
	// a failed account listing rendering as "no such account", and it is why the
	// answer is a flag rather than a zero SelfLocation.
	LocationKnown bool
	Location      platform.SelfLocation
}

// daemonProbe is the seam: a test states the daemon's location rather than asserting on
// whether the machine running the test happens to have plock installed and running.
// Same reason systemdProbe and serveSelfLocation are vars.
var daemonProbe = func() daemonState { return askDaemonLocation(platform.ExecRunner{}, os.ReadFile) }

// askDaemonLocation asks systemd for the unit's main pid and then reads that pid's
// cgroup. Two steps because the init system knows the process and the kernel knows
// where the process lives; neither step is guessed when the other fails.
func askDaemonLocation(run platform.Runner, read func(string) ([]byte, error)) daemonState {
	if run == nil {
		return daemonState{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// `show --property=MainPID` rather than `is-active`: the state word answers "is
	// there a daemon" and the pid answers "which process do I read the cgroup of", and
	// one call that returns the pid cannot disagree with a second call that returns
	// the state. An inactive or unknown unit prints MainPID with no value, which
	// parses as 0 and means exactly what it says.
	out, err := run.Run(ctx, []string{"systemctl", "show", "plock", "--property=MainPID"}, nil, 0)
	if err != nil && isNotFound(err) {
		return daemonState{}
	}
	pid := parseMainPID(out)
	if pid <= 0 {
		// A failed systemctl with no pid is "systemd answered, nothing is running";
		// a failed systemctl that somehow printed a pid still gets read, because the
		// pid is the fact and the exit status only says the asking was noisy.
		return daemonState{SystemdKnown: true}
	}
	loc, lerr := platform.LocationOf(pid, read)
	return daemonState{SystemdKnown: true, Running: true, LocationKnown: lerr == nil, Location: loc}
}

// parseMainPID reads the MainPID property out of systemctl's `key=value` output.
//
// It is a prefix scan rather than a split on "=", because `show` prints one property per
// line and a future property whose name contains "MainPID" as a substring must not be
// the one that answers — the longest match is not the point, the exact key is.
func parseMainPID(out []byte) int {
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "MainPID=") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(line, "MainPID="))
		if v == "" {
			return 0
		}
		// ParseUint, not Atoi: a negative pid is not a process, and a value that will
		// not parse is answered 0 ("no daemon") rather than guessed at.
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return 0
		}
		return int(n)
	}
	return 0
}
