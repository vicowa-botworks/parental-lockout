package platform

import (
	"context"
	"fmt"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"strings"
)

// desktop locks the screen through the freedesktop ScreenSaver interface, inside the
// session user's own bus.
//
// One implementation covers both desktops this daemon targets, because that is what
// was measured on the target platform rather than what the ticket assumed. On Plasma
// 6.6.6 the lock method is org.freedesktop.ScreenSaver.Lock at object path
// /ScreenSaver, exported under three well-known names (org.kde.screensaver,
// org.freedesktop.ScreenSaver, org.kde.ksmserver) — but not by one process, which is what
// the ordered dests list is for. On an X11 session ksmserver owns them; on a Wayland
// session kwin_wayland owns the first two and serves /ScreenSaver, leaving ksmserver only
// org.kde.ksmserver (measured 2026-10-28 in a KDE guest, #179). The two
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
	// corroborate says this flavour has a second signal to confirm with, and that
	// LockConfirmed therefore withholds a yes unless both halves agree. See
	// LockConfirmed and locker.go.
	//
	// It is a field rather than a check on id because the two flavours are otherwise
	// one mechanism, and the day GNOME grows an equivalent signal the difference
	// should be one line in one registration rather than a branch on a name.
	corroborate bool
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
	Register("kde", func(r Runner, _ config.Settings) (Platform, error) {
		return &desktop{
			id:    "kde",
			dests: []string{"org.kde.screensaver", "org.freedesktop.ScreenSaver", "org.kde.ksmserver"},
			r:     r,
			hint:  &logind{r: r},
			// KDE spawns a separate locker, so KDE has something to corroborate
			// with. Measured; see locker.go.
			corroborate: true,
		}, nil
	})
	Register("gnome", func(r Runner, _ config.Settings) (Platform, error) {
		return &desktop{
			id:    "gnome",
			dests: []string{"org.gnome.ScreenSaver", "org.freedesktop.ScreenSaver"},
			r:     r,
			hint:  &logind{r: r},
			// No corroborating signal yet. GNOME draws its lock screen inside
			// gnome-shell, so there is no separate locker to find, and the
			// candidate signal (org.gnome.ScreenSaver.GetActive) has not been
			// measured — the KDE spelling of that call answers (true,) on an
			// unlocked desktop, so it cannot be assumed to mean "locked" here.
			// Guessing would turn the corroborator into a rubber stamp.
			corroborate: false,
		}, nil
	})
}

func (d *desktop) Name() string { return d.id }

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

	// Resolve once, here, and hand the runner an absolute path.
	//
	// ExecRunner.Run now resolves any bare program name itself, so loginctl's four call
	// sites and every future one are covered without each backend repeating the check. This
	// backend asks anyway, for one reason specific to it: `kde` tries several bus names in
	// order, and if each attempt resolved on its own, the second attempt could exec a
	// different binary than the first. Resolving once and passing the result as a path makes
	// that impossible -- Run takes a name containing a separator as given.
	//
	// What the resolution is NOT is a defence against the session user choosing the binary.
	// It cannot be. The lookup happens in the daemon's process, from the daemon's
	// os.Getenv("PATH"), and exec.CommandContext takes that answer when it is constructed --
	// before cmd.Env is assigned, so the PATH handed to the child never enters into it.
	// Measured with the repo's own ordering and two copies of one tool: cmd.Path is the
	// daemon's copy, and the daemon's copy is what runs.
	//
	// The exposure that does exist is a directory the session user can write sitting on the
	// daemon's own PATH. LookPath walks that PATH and finds an impostor exactly as happily as
	// exec would, so nothing here closes it, and it is the same for loginctl as for gdbus.
	// Closing it means declining to run a program whose location cannot be trusted, which
	// fails toward not locking anyone out at all. That trade is a decision rather than a
	// detail, so it is written up in docs/security.md instead of being taken quietly.
	gdbusPath, err := lookPath(gdbusProgram)
	if err != nil {
		return fmt.Errorf("%s: %s is not runnable: %w", d.id, gdbusProgram, err)
	}

	var errs []error
	for _, dest := range d.dests {
		argv := []string{
			gdbusPath, "call",
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

// LockConfirmed reads logind's LockedHint, corroborated on KDE by the locker process,
// because the screensaver interface cannot answer this question.
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
// What is left, LockedHint, is written through Session.SetLockedHint, which login1
// authorizes by uid alone: the controlled user owns it (#29, measured). So on a flavour
// that has a second signal the hint is only half an answer, and a yes needs both.
// locker.go records what the second half is and what it does not prove.
//
// Whether ksmserver writes the hint at all is still the open question recorded in
// docs/platforms.md's manual recipe. It is orthogonal to this: if it never does, a KDE
// lock is reported unconfirmed, which is loud and safe.
func (d *desktop) LockConfirmed(ctx context.Context, s Session) (bool, error) {
	if s.ID == "" {
		return false, fmt.Errorf("%s: cannot confirm a session with no id", d.id)
	}
	yes, err := d.hint.LockConfirmed(ctx, s)
	if err != nil || !yes {
		return false, err
	}
	if !d.corroborate {
		return true, nil
	}
	alive, err := lockerRunningCached(ctx, s.UID)
	if err != nil {
		// Not the same as "unlocked". The hint said locked and the corroborator
		// could not look; that belongs in the fault column, and the chain already
		// has a sentence for a backend that could not be consulted.
		return false, fmt.Errorf("%s: the session reports itself locked but the locker could not be checked: %w", d.id, err)
	}
	return alive, nil
}

// Corroborated reports that a yes from this backend stands on something other than the
// hint the controlled user can write. Chain.Lock uses it to decide whether a lock that
// was already reported locked beforehand still needs believing.
func (d *desktop) Corroborated() bool { return d.corroborate }

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
		//
		// Whether confirmation here is corroborated is operator-facing for the same
		// reason: it is the difference between "unconfirmed means we could not look"
		// and "unconfirmed means this backend can never confirm anything".
		Detail: fmt.Sprintf("%s is installed and would call %s.Lock on %s as the session user. "+
			"That is all Probe can see: it has no user session to ask, so \"usable\" means the tool "+
			"is present, not that a lock would land here. Enumerates no sessions of its own, and "+
			"confirms via logind LockedHint (the screensaver interface exposes no lock state) %s.",
			gdbusProgram, screensaverIface, strings.Join(d.dests, ", "), corroborationNote(d.corroborate)),
	}}
}

// corroborationNote states, for Probe, what a confirmation from this backend is worth.
// LockedHint is writable by the session user, so an uncorroborated yes is the request
// landing rather than the screen being locked.
func corroborationNote(corroborated bool) string {
	if corroborated {
		return "corroborated by a live kscreenlocker_greet parented to ksmserver, which the " +
			"session user cannot produce by writing a property"
	}
	return "uncorroborated: LockedHint is writable by the session user, so a yes here is weaker " +
		"than the screen being locked and Chain.Lock discounts it unless the state changed"
}
