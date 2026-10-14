package platform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The KDE locker is a separate process, and on this desktop it is the only thing a lock
// leaves behind that the controlled user did not put there.
//
// #29 established why a second signal is needed at all. logind's Session.SetLockedHint
// is authorized by uid alone — org.freedesktop.login1.policy ships 37 actions and none
// of them covers the hint, because desktops are assumed to be the only writer — so the
// property this package confirms with is owned by the party being locked out. One file
// in ~/.config/autostart holds it true forever.
//
// What follows is narrow but real. The daemon cannot prove the screen is locked; it can
// refuse to accept a lock that left no trace in the process tree.
//
// Measured on the target (Plasma 6.x, Ubuntu, 2026-10-14), with the session genuinely
// locked:
//
//	$ ps -eo pid=,ppid=,uid=,args= | grep kscreenlocker
//	2224790    7619  1000 /usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet --graceTime 5000 --ksldfd 56
//	$ ps -o args= -p 7619
//	/usr/bin/ksmserver
//
// so the locker is a child of ksmserver — the process that owns
// org.freedesktop.ScreenSaver and answers the Lock call this backend sends. Requiring
// both halves, the locker and that specific parent, moves the impostor's job from
// "write a property" to "get code running as a child of the session's own compositor".
// The locker runs as uid 1000, the same uid as the child, so that is a bar raised and
// not a wall; docs/platforms.md's threat model says so rather than implying otherwise.
const (
	kdeLockerProgram = "kscreenlocker_greet"
	ksmserverProgram = "ksmserver"
)

// procRoot is /proc behind a variable so the walk can be tested against a fixture — the
// same reason lookPath exists.
//
// Reading /proc directly rather than running ps is deliberate and not only about speed.
// This is a security check, and a check whose answer comes from a PATH lookup can be
// answered by whatever the session user placed earlier in PATH; the session user owns
// the environment this backend runs commands in.
var procRoot = "/proc"

// lockerRunning reports whether a KDE locker owned by uid is alive as a child of the
// session's compositor.
//
// A process that vanishes mid-walk is skipped rather than reported: it was not the
// locker, and a machine under load loses short-lived processes constantly. A procRoot
// that cannot be read at all is the opposite case and returns an error, because
// "cannot look" must not arrive in the same shape as "looked and found nothing" — the
// caller treats the second as unconfirmed and the first as a fault worth naming.
func lockerRunning(ctx context.Context, uid uint32) (bool, error) {
	dir, err := os.Open(procRoot)
	if err != nil {
		return false, fmt.Errorf("cannot open %s to look for the locker: %w", procRoot, err)
	}
	defer dir.Close()

	names, err := dir.Readdirnames(-1)
	if err != nil {
		return false, fmt.Errorf("cannot enumerate %s: %w", procRoot, err)
	}

	for _, name := range names {
		// The walk is short but it is not free, and it runs inside the lock budget.
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if _, err := strconv.Atoi(name); err != nil {
			continue // not a process directory
		}
		argv, err := procArgv(name)
		if err != nil || len(argv) == 0 {
			continue // vanished, or a kernel thread with no argv
		}
		if filepath.Base(argv[0]) != kdeLockerProgram {
			continue
		}
		ppid, puid, err := procParent(name)
		if err != nil {
			continue
		}
		// The locker belongs to the session it is locking. Without this, another
		// user's locked session — or a root-owned one — would stand in as evidence
		// for this one.
		if puid != uid {
			continue
		}
		parentArgv, err := procArgv(strconv.Itoa(ppid))
		if err != nil || len(parentArgv) == 0 {
			continue
		}
		if filepath.Base(parentArgv[0]) == ksmserverProgram {
			return true, nil
		}
	}
	return false, nil
}

// procArgv reads /proc/<pid>/cmdline: NUL-separated, with a trailing NUL after the last
// argument, and empty for a kernel thread.
func procArgv(pid string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(procRoot, pid, "cmdline"))
	if err != nil {
		return nil, err
	}
	fields := strings.Split(string(raw), "\x00")
	// The final NUL leaves one empty field, and some kernels emit several. Keeping
	// them would make argv[1] an empty string on every process on the machine.
	for len(fields) > 0 && fields[len(fields)-1] == "" {
		fields = fields[:len(fields)-1]
	}
	return fields, nil
}

// procParent returns a process's parent pid and its effective uid, both from
// /proc/<pid>/status.
//
// status rather than stat: field 4 of stat is the parent pid, but field 2 is the
// command name in parentheses and a command name may contain spaces and parentheses,
// which shifts every field after it for anything parsing by position. status is
// line-oriented and keyed by name, which is the same reason the loginctl parser maps
// its columns by header.
func procParent(pid string) (ppid int, uid uint32, err error) {
	raw, err := os.ReadFile(filepath.Join(procRoot, pid, "status"))
	if err != nil {
		return 0, 0, err
	}
	seen := 0
	for _, line := range strings.Split(string(raw), "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch key {
		case "PPid":
			if ppid, err = strconv.Atoi(val); err != nil {
				return 0, 0, fmt.Errorf("/proc/%s/status has an unparsable PPid %q: %w", pid, val, err)
			}
			seen++
		case "Uid":
			// real, effective, saved, filesystem. The effective uid is the one that
			// owns the locker's D-Bus connection, and is what a setuid helper would
			// differ on.
			fields := strings.Fields(val)
			if len(fields) < 2 {
				return 0, 0, fmt.Errorf("/proc/%s/status has an unparsable Uid %q", pid, val)
			}
			u, err := strconv.ParseUint(fields[1], 10, 32)
			if err != nil {
				return 0, 0, fmt.Errorf("/proc/%s/status has an unparsable effective uid %q: %w", pid, fields[1], err)
			}
			uid = uint32(u)
			seen++
		}
	}
	if seen < 2 {
		return 0, 0, fmt.Errorf("/proc/%s/status carries neither PPid nor Uid", pid)
	}
	return ppid, uid, nil
}
