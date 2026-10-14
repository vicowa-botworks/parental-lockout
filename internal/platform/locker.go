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
// both halves, the locker and that specific parent, is what makes the check about a
// process tree rather than one property. Neither half is decided by a name: argv[0] is
// whatever the caller passed to execve, and the reviewer showed that two execve calls as
// the session user produce a tree whose names are both lies. Identity comes from
// /proc/<pid>/exe, and the resolved binary must be one the session user could not have
// written. The locker runs as uid 1000, the same uid as the child, so that is a bar raised
// and not a wall; docs/platforms.md's threat model says so rather than implying otherwise.
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
		// The uid filter runs before anything is resolved. It is the cheapest read in
		// this loop and it discards the whole machine except the session being locked,
		// and the walk runs on every confirmation inside the settle window.
		_, puid, err := procParent(name)
		if err != nil || puid != uid {
			continue
		}
		// The name is only a candidate filter now. It cannot be the answer, and the
		// reason is in the next paragraph.
		argv, err := procArgv(name)
		if err != nil || len(argv) == 0 {
			continue // vanished, or a kernel thread with no argv
		}
		if filepath.Base(argv[0]) != kdeLockerProgram {
			continue
		}
		ppid, _, err := procParent(name)
		if err != nil {
			continue
		}
		if !isLockerBinary(name, uid) {
			continue
		}
		parentArgv, err := procArgv(strconv.Itoa(ppid))
		if err != nil || len(parentArgv) == 0 {
			continue
		}
		if filepath.Base(parentArgv[0]) != ksmserverProgram {
			continue
		}
		if !isCompositorBinary(strconv.Itoa(ppid), uid) {
			continue
		}
		return true, nil
	}
	return false, nil
}

// isLockerBinary and isCompositorBinary decide whether a pid is what its argv[0] claims,
// by asking the kernel rather than the process.
//
// argv[0] is whatever the caller passed to execve. The session user can therefore build
// the exact process tree this check used to accept with two execve calls and no
// privilege at all — a helper started from ~/.config/autostart with
// argv[0] = "/usr/bin/ksmserver" that spawns /bin/sleep with
// argv[0] = "kscreenlocker_greet" is a parent and child whose names are both lies, and
// the old check reported a corroborated lock. That is #29's failure mode arriving
// through the mitigation, so the name is kept only as a cheap pre-filter.
//
// /proc/<pid>/exe is a symlink the kernel maintains and a user cannot point at
// something else, so it answers "what is actually running". Two things are then
// required of the resolved path, for both halves of the tree:
//
//   - its basename is the program expected, so a copy installed under another name is
//     not accepted, and
//   - the file is not writable by the session user — not owned by them, and not
//     group- or world-writable.
//
// The second requirement is what actually raises the bar. It is not the path: the
// canonical location differs by distribution (/usr/lib/x86_64-linux-gnu/libexec on
// Ubuntu, /usr/libexec on Fedora), so pinning one absolute path would break the real
// desktop in order to stop the fake one. What does not vary is that the session user
// cannot write to the directory the real locker lives in, while they can put a binary
// anywhere they do own. So the impostor now needs a binary named
// kscreenlocker_greet that the session user cannot modify, parented by one named
// ksmserver that the session user cannot modify — which means either the real pair, or
// a write that needs root.
func isLockerBinary(pid string, uid uint32) bool {
	return resolvedBinaryIsTrusted(pid, kdeLockerProgram, procRoot, uid)
}

func isCompositorBinary(pid string, uid uint32) bool {
	return resolvedBinaryIsTrusted(pid, ksmserverProgram, procRoot, uid)
}

func resolvedBinaryIsTrusted(pid, wantProgram, root string, uid uint32) bool {
	exe, err := procExe(root, pid)
	if err != nil || exe == "" {
		return false
	}
	if filepath.Base(exe) != wantProgram {
		return false
	}
	return !binaryWritableBy(exe, uid)
}

// procExe resolves the binary a process is actually running.
//
// The kernel appends " (deleted)" when the file was unlinked or replaced under a
// running process. That is not a locker we can vouch for — the bytes at that path are
// gone and whatever is there now was put there by somebody else — so it is reported as
// no answer rather than a path.
func procExe(root, pid string) (string, error) {
	link, err := os.Readlink(filepath.Join(root, pid, "exe"))
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(link, " (deleted)") {
		return "", fmt.Errorf("%s: the binary was deleted while running", pid)
	}
	return link, nil
}

// binaryWritableBy reports whether uid could have put these bytes here.
//
// Owned by the target uid is disqualifying on its own: they chose the contents. So is
// group- or world-writable, because a binary anybody can append to is a binary the
// session user can change without touching its owner. A root-owned 0755 binary under
// /usr passes both, which is the real desktop.
//
// A file that cannot be stat'ed is not trusted. "Cannot tell" is answered on the side
// of not corroborating, because the cost of a false "locked" is an audit line saying a
// child was locked out when they were not.
func binaryWritableBy(path string, uid uint32) bool {
	owner, ok := fileOwner(path)
	if !ok {
		return true
	}
	if owner == uid {
		return true
	}
	fi, err := os.Stat(path)
	if err != nil {
		return true
	}
	return fi.Mode().Perm()&0o022 != 0
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
