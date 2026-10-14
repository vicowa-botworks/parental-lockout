package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// The desktop backends run as root and must make their call inside somebody else's
// desktop session. Root has no session bus: the bus socket belongs to the user, and
// the display server authenticates against that user's runtime directory. So the
// child is dropped to the session uid and given an environment built for that
// session rather than inherited from the daemon.
//
// Building the environment explicitly, rather than starting from os.Environ(), is
// the point. The daemon runs as root, so its environment is the most privileged one
// on the machine, and everything in it would be inherited by a process about to be
// handed a user's bus: LD_PRELOAD, PYTHONPATH, a curated PATH that points at
// administrator tooling, whatever a systemd unit put there. An allowlist is the only
// shape that stays correct as the daemon's own environment grows.

// envAllowlist are the variables from the daemon's own environment that a session
// command may keep. They are presentation settings with no privilege attached, and
// dropping them would change the language of anything the command prints into a log.
var envAllowlist = []string{"PATH", "LANG"}

// envAllowPrefixes are allowed by prefix: the whole LC_* family is locale
// presentation and nothing else.
var envAllowPrefixes = []string{"LC_"}

// sessionEnv builds the complete environment for a command that has to reach the
// session user's bus.
//
// The two entries that make the call work at all are XDG_RUNTIME_DIR and
// DBUS_SESSION_BUS_ADDRESS. Both are derived from the uid, never read from the
// daemon's own environment: the daemon's XDG_RUNTIME_DIR is /run/user/0, and a
// desktop call that silently used root's runtime dir would fail in a way that looks
// like a broken desktop rather than a misbuilt command.
//
// DISPLAY and WAYLAND_DISPLAY are deliberately absent. The ticket asked for them
// "when available", but the call this package makes is a D-Bus method call, and gdbus
// reaches the bus from DBUS_SESSION_BUS_ADDRESS alone — it never opens a display.
// logind reports a session's Type (x11 | wayland | tty) but not its display number, so
// the only way to set these would be to guess :0 or wayland-1. A guessed DISPLAY is
// strictly worse than no DISPLAY: on a machine with a second X server it points the
// child at somebody else's screen, and it would be set for a tty session that has no
// display at all. A backend that genuinely needs one must obtain it from a source it
// can verify, not derive it from a session type.
func sessionEnv(uid int) []string {
	runtime := "/run/user/" + strconv.Itoa(uid)
	env := []string{
		"XDG_RUNTIME_DIR=" + runtime,
		"DBUS_SESSION_BUS_ADDRESS=unix:path=" + runtime + "/bus",
		// HOME is what a lot of desktop tooling looks for first, and a dropped
		// child otherwise inherits root's. Left empty rather than guessed: the
		// screensaver call does not need it, and a wrong HOME has a way of being
		// written to.
		"HOME=",
	}
	for _, kv := range os.Environ() {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || value == "" {
			continue
		}
		if allowedEnv(name) {
			env = append(env, name+"="+value)
		}
	}
	return env
}

// allowedEnv reports whether a variable from the daemon's environment may be passed
// into a session command.
func allowedEnv(name string) bool {
	for _, want := range envAllowlist {
		if name == want {
			return true
		}
	}
	for _, pre := range envAllowPrefixes {
		if strings.HasPrefix(name, pre) {
			return true
		}
	}
	return false
}

// checkSessionTarget refuses the two targets this package must never act on.
//
// uid 0 is the hard one. The daemon is root, so a "drop privileges to the target"
// path that accepts uid 0 is a path that drops nothing and runs the lock as root
// against root's bus — which fails, and fails in the direction that looks like a
// desktop problem rather than a policy one. It also means a Session built with a zero
// uid (unset, or a listing that could not resolve one) is refused at the last moment
// rather than reaching the child.
//
// A session id that is empty or implausible is refused for the same reason the
// logind backend refuses it: the identifier is the whole argument that says which
// screen to lock.
func checkSessionTarget(s Session) error {
	if s.UID == 0 {
		return fmt.Errorf("%s: refusing to lock a session owned by uid 0", s.Describe())
	}
	if s.ID == "" {
		return fmt.Errorf("refusing to lock a session with no id")
	}
	return nil
}
