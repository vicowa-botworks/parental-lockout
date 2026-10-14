//go:build unix

package audit

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// openAppend opens the audit file for appending without following a symlink and
// without ever blocking.
//
// O_NOFOLLOW is the tamper guard: the audit directory is root-owned, but a
// daemon that follows a link from a path it did not create is a daemon that
// writes root's trail into whatever a privileged-enough attacker pointed it at.
//
// O_NONBLOCK is the one people leave out and the one that matters most here.
// Without it, a FIFO planted at the audit path makes the open *wait for a writer
// that never comes* — and this code runs inside the enforcement tick. A tick
// parked on a FIFO stops re-locking, so a child who can create a named pipe
// turns off enforcement without touching the daemon. With it, the open fails
// immediately with ENXIO and the event is counted as dropped, which is loud.
//
// The mode is set by the open itself so there is no interval in which the trail
// exists and is readable by the accounts it describes.
func openAppend(path string, mode os.FileMode) (*os.File, error) {
	f, err := os.OpenFile(path,
		os.O_APPEND|os.O_WRONLY|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK,
		mode)
	if errors.Is(err, syscall.ELOOP) {
		// "Too many levels of symbolic links" is not a sentence that describes an
		// attack on a log file, and this is the one place that knows the flag that
		// produced it was deliberate.
		return nil, fmt.Errorf("%s is a symlink: refusing to write the trail through it: %w", path, err)
	}
	if errors.Is(err, syscall.ENXIO) {
		return nil, fmt.Errorf("%s is a FIFO: the audit path must be a regular file: %w", path, err)
	}
	return f, err
}

// geteuid is the effective uid this process writes the trail as. It is a
// variable so the ownership guard can be exercised by an unprivileged test: the
// rule only ever fires for a file owned by somebody else, and a test running as
// uid 1000 cannot create one. A guard that has never fired on any machine is a
// guard nobody has checked, and CI is the only machine this runs on unprivileged.
var geteuid = func() uint32 { return uint32(os.Geteuid()) }

// openDir opens a directory to change its mode on the descriptor rather than the
// path, and without following a link that appeared between the check and the open.
func openDir(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

// setDirMode sets the mode on a directory this process just created, addressed
// relative to a descriptor on its parent.
//
// The obvious code — open the new directory and Fchmod it — does not work. A
// umask strict enough to matter here (UMask=0700 on the unit, say) strips the
// owner's own bits, so the directory MkdirAll just made is one we cannot open,
// and the daemon would fail to start and be restarted by systemd into a loop that
// never locks anyone out.
//
// Fchmodat against the parent's descriptor avoids both that and the usual
// objection to chmod-by-path: the name is resolved inside a directory this
// process has already established is its own and not world-writable, so nobody
// else can have swapped it for a link between the create and the chmod.
func setDirMode(parent *os.File, name string, mode os.FileMode) error {
	return syscall.Fchmodat(int(parent.Fd()), name, uint32(mode.Perm()), 0)
}

// ownerIsWriter reports whether fi is owned by the process that is about to
// append to it.
//
// PLAN §10 says "owned by root", and the daemon is root, so on the target this is
// the same question. It is phrased against the effective uid because the rule is
// really about ownership *by the writer*: a trail owned by somebody else is one
// somebody else can rewrite, and that is true whether the writer is root or the
// uid a test happens to run under. Comparing against a literal 0 would make the
// guard untestable off a root daemon, and a guard nobody has seen fire is a guard
// nobody can vouch for.
func ownerIsWriter(fi os.FileInfo) (bool, error) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		// Reporting "unknown" rather than "yes" keeps the guard failing closed if
		// the runtime ever hands back something it does not recognise.
		return false, errOwnershipUnsupported
	}
	return st.Uid == geteuid(), nil
}

// tightenMode sets the mode on the descriptor we are holding, never on the path.
//
// chmod by path follows symlinks, so a root daemon that chmods a name it did not
// open is an arbitrary-chmod primitive the moment a link appears in that
// directory. The descriptor is the only thing whose identity is stable between
// the check and the change.
func tightenMode(f *os.File, mode os.FileMode) error {
	return syscall.Fchmod(int(f.Fd()), uint32(mode.Perm()))
}
