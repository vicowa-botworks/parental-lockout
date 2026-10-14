package platform

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// desktop locks the screen through the freedesktop ScreenSaver interface, inside the
// session user's own bus.
//
// One implementation covers both desktops this daemon targets, because that is what
// was measured on the target platform rather than what the ticket assumed. On Plasma
// 6.6.6 the lock method is org.freedesktop.ScreenSaver.Lock at object path
// /ScreenSaver, served by ksmserver under three well-known names
// (org.kde.screensaver, org.freedesktop.ScreenSaver, org.kde.ksmserver). The two
// commands the ticket named do not exist there at all: org.kde.KScreen is owned by
// kscreen_backend and exposes only /backend, and /KSMServer offers no lockScreen.
// GNOME exposes the same interface under org.gnome.ScreenSaver, so "kde" and "gnome"
// are the same mechanism with a different order of preference for the bus name — which
// is why they are one type here rather than two copies of the same call.
type desktop struct {
	id string
	// dests are the bus names to try, in order. More than one because the same
	// object is exported under several names and which one a given release exports
	// is a packaging detail; a backend that hard-codes one stops working when the
	// desktop renames it.
	dests []string
	r     Runner
	// hint confirms the lock. The screensaver interface offers nothing to confirm
	// with — see LockConfirmed — so confirmation is logind's LockedHint, read
	// through the same runner.
	hint Confirmer
}

// screensaverPath and screensaverIface are the object path and interface measured on
// the target: the interface is org.freedesktop.ScreenSaver even under the
// org.kde.screensaver bus name.
const (
	screensaverPath  = "/ScreenSaver"
	screensaverIface = "org.freedesktop.ScreenSaver"

	// gdbusProgram is the tool this backend shells out to: glib, present on KDE and
	// GNOME alike, and free of the qtchooser selection step that makes qdbus fail with
	// exit 0 on Ubuntu. See Lock.
	gdbusProgram = "gdbus"
)

func init() {
	Register("kde", func(r Runner) (Platform, error) {
		return &desktop{
			id:    "kde",
			dests: []string{"org.kde.screensaver", "org.freedesktop.ScreenSaver", "org.kde.ksmserver"},
			r:     r,
			hint:  &logind{r: r},
		}, nil
	})
	Register("gnome", func(r Runner) (Platform, error) {
		return &desktop{
			id:    "gnome",
			dests: []string{"org.gnome.ScreenSaver", "org.freedesktop.ScreenSaver"},
			r:     r,
			hint:  &logind{r: r},
		}, nil
	})
}

func (d *desktop) Name() string { return d.id }

// lookPath is exec.LookPath behind a variable, for the same reason the logind backend
// takes a Runner: Probe's two answers are the whole of what an operator reads when a
// lock never happens, and a test cannot make gdbus appear or disappear from the host.
// Without the seam, one of the two branches is untestable on any given machine and
// silently unchecked.
var lookPath = exec.LookPath

// Sessions returns nothing, deliberately. Session discovery is logind's job; this
// backend exists to reach a bus that root cannot reach. A chain configured without
// logind therefore finds no sessions and enforces nothing, which Probe says out loud
// rather than pretending the backend is complete on its own.
func (d *desktop) Sessions(context.Context) ([]Session, error) { return nil, nil }

// Lock calls org.freedesktop.ScreenSaver.Lock as the session user.
//
// gdbus, not qdbus. On Ubuntu /usr/bin/qdbus is a symlink to qtchooser, and with
// QT_SELECT unset it prints "qdbus: could not find a Qt installation of ”" to stdout
// and exits 0. A backend that read that zero as success would report a lock every
// thirty seconds forever while the screen stayed up, and — because the chain only
// advances when a lock verifiably took — a false success would also stop the fallback
// chain dead. gdbus is glib, present on KDE and GNOME alike, and has no Qt selection
// step to get wrong.
//
// The reply is checked, not just the exit status. Lock is a void method, so gdbus
// prints exactly "()" on success; measured against the live interface, a void call
// returns "()" and a value-returning one returns "(true,)". Anything else — including
// empty output on a zero exit — is a failure. That is the general defence against the
// qdbus class of problem rather than a fix for one binary.
func (d *desktop) Lock(ctx context.Context, s Session) error {
	if err := checkSessionTarget(s); err != nil {
		return err
	}
	env := sessionEnv(int(s.UID))

	var errs []error
	for _, dest := range d.dests {
		argv := []string{
			"gdbus", "call",
			"--session",
			"--dest", dest,
			"--object-path", screensaverPath,
			"--method", screensaverIface + ".Lock",
		}
		out, err := d.r.Run(ctx, argv, env, uint32(s.UID))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", dest, err))
			continue
		}
		if reply := strings.TrimSpace(string(out)); reply != "()" {
			// A zero exit that did not produce the void reply is not a lock. The
			// reply is kept in the message because it is the only evidence of what
			// actually answered.
			errs = append(errs, fmt.Errorf("%s: expected the void reply () from %s.Lock, got %q",
				dest, screensaverIface, firstLine(out)))
			continue
		}
		return nil
	}
	return fmt.Errorf("%s: no screensaver service accepted the lock: %w", d.id, joinErrs(errs...))
}

// LockConfirmed reads logind's LockedHint, because the screensaver interface cannot
// answer this question.
//
// Two tempting signals were measured on the live desktop and both are wrong:
//
//   - org.freedesktop.ScreenSaver has no Locked property at all ("Property
//     ... .Locked was not found in object /ScreenSaver"), so there is nothing to read.
//   - GetActive() returns (true,) on this box while the desktop is unlocked and in
//     active use. Whatever it means — enabled rather than engaged, most likely — it is
//     not "locked", and a backend that confirmed with it would report every lock as
//     confirmed. That is the exact failure this package exists to prevent, so the
//     method is not called anywhere in this file.
//
// LockedHint is written by the desktop when it locks. Whether ksmserver writes it is
// the open question recorded in docs/platforms.md's manual recipe, with the expected
// answer stated: answering it needs a real lock, and a machine with a person using it
// is not the place to take one.
func (d *desktop) LockConfirmed(ctx context.Context, s Session) (bool, error) {
	if s.ID == "" {
		return false, fmt.Errorf("%s: cannot confirm a session with no id", d.id)
	}
	return d.hint.LockConfirmed(ctx, s)
}

// Probe reports what this backend would target and why it might not work, without
// locking anything.
func (d *desktop) Probe(ctx context.Context) []BackendStatus {
	// The one thing an operator needs first is whether the tool is even here, since
	// the whole design rests on gdbus being present.
	if _, err := lookPath(gdbusProgram); err != nil {
		return []BackendStatus{{
			Name:      d.id,
			Available: false,
			Detail:    fmt.Sprintf("%s not found: %v", gdbusProgram, err),
		}}
	}
	return []BackendStatus{{
		Name:      d.id,
		Available: true,
		// "usable" here means only that the tool is installed. Probe has no user
		// session to ask, so it cannot see whether a screensaver service is reachable
		// — and on a machine with no desktop it would otherwise report a backend as
		// usable that could never lock anything. Saying what was not checked is the
		// difference between a diagnostic and a reassurance.
		Detail: fmt.Sprintf("%s is installed and would call %s.Lock on %s as the session user. "+
			"That is all Probe can see: it has no user session to ask, so \"usable\" means the tool "+
			"is present, not that a lock would land here. Enumerates no sessions of its own, and "+
			"confirms via logind LockedHint (the screensaver interface exposes no lock state).",
			gdbusProgram, screensaverIface, strings.Join(d.dests, ", ")),
	}}
}
