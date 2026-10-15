package platform

import (
	"os"
	"strconv"
	"strings"
)

// SelfCgroupPath is where the kernel records the cgroup of the process doing the
// reading. It is exported rather than hidden so the caller reads it explicitly: the
// only untestable part of this file is that one ReadFile, and everything above it is
// a pure function a test can hand any string to.
const SelfCgroupPath = "/proc/self/cgroup"

// UserSliceUID reports the uid whose per-user slice a cgroup file places a process
// in, and whether it is inside one at all.
//
// The question it answers is "could this process be running inside a login session?",
// which is the question #67 needs answered before the enforcement loop decides to
// lock a screen: a daemon that locks the session it is running in cannot be stopped,
// because the only way to stop it is at a keyboard behind that screen.
//
// The cgroup is the signal to use, and the reason to use it is the #29 test — is this
// something the controlled user can write?
//
//   - XDG_SESSION_ID is inherited from whoever started the process, so it is an
//     assertion about the environment rather than about the process.
//   - loginctl's LockedHint is writable by the session's own uid with no privilege,
//     which is why nothing may decide when enforcement stops based on it.
//   - /proc/self/cgroup is authored by the kernel from the cgroup the process was
//     placed in at activation. A process cannot move itself, and a controlled user
//     cannot place a root service in their slice.
//
// It is also the only one of the three that gives the right answer here. Process
// ancestry — the comparison Session.Leader's own comment proposes — was measured on a
// live Plasma session and matches only the user@1000.service "manager" row, because a
// systemd user manager reparents GUI processes onto itself rather than onto the
// session leader. So ancestry never matches the desktop session actually on the
// screen, and a check built on it would silently never fire. That is the shape this
// package refuses: a guard that cannot fire reads as a guard that passed.
//
// The parser is deliberately narrow, and the narrowness is the safe direction. A
// false "inside" makes the daemon decline to lock one account, which is a fail-open;
// a false "outside" makes it lock its own screen, which is the bug. Neither is
// harmless, so the rule claims "inside" only on the one unambiguous marker: a path
// element of the exact form user-<digits>.slice. Anything else — machine.slice, a
// container runtime's own layout, a truncated read, an empty file — answers "outside",
// which is today's behaviour and the behaviour of every supported deployment, because
// a systemd system unit lives under system.slice and has no user slice to find.
func UserSliceUID(cgroupFile string) (uid uint32, insideUserSlice bool) {
	for _, line := range strings.Split(cgroupFile, "\n") {
		// cgroup v2 is "0::/path"; v1 is "hier:controllers:/path". Both are the same
		// shape once you take the first two fields and call the rest the path, and
		// SplitN is what makes that correct: a unit name may contain a colon, and
		// scanning for the LAST one (or for "::") would cut inside the path and hand
		// back a truncated one. A truncated path reads as "not a user slice", which is
		// a guard that silently never fires.
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		path := parts[2]
		if u, ok := userSliceInPath(path); ok {
			return u, true
		}
	}
	return 0, false
}

// userSliceInPath looks for the one path element this rule recognises.
func userSliceInPath(path string) (uint32, bool) {
	const (
		prefix = "user-"
		suffix = ".slice"
	)
	for _, elem := range strings.Split(path, "/") {
		if !strings.HasPrefix(elem, prefix) || !strings.HasSuffix(elem, suffix) {
			continue
		}
		digits := elem[len(prefix) : len(elem)-len(suffix)]
		// ParseUint, not Atoi: it rejects a leading sign, so "user--1.slice" and
		// "user-+5.slice" are not a uid. It also rejects the empty digits that
		// "user-.slice" would otherwise hand to an empty-string parse.
		n, err := strconv.ParseUint(digits, 10, 32)
		if err != nil {
			continue
		}
		return uint32(n), true
	}
	return 0, false
}

// ReadSelfUserSlice answers the question for the calling process.
//
// A failure to read is answered "outside", not with an error. The caller has no
// useful action for one: the alternative to answering here is to guess, and the guess
// that keeps the daemon alive on a machine where /proc is unusual is the guess every
// supported deployment already gets.
func ReadSelfUserSlice() (uid uint32, insideUserSlice bool) {
	b, err := os.ReadFile(SelfCgroupPath)
	if err != nil {
		return 0, false
	}
	return UserSliceUID(string(b))
}

// SelfLocation is where this process is running, as far as the lockout decision
// cares: whose login session it is inside, if it is inside anybody's.
//
// It is one value rather than a uid and a flag because the two are only meaningful
// as a pair. A uid on its own matches every session of that account, including the
// ones a system daemon should still lock; the flag on its own skips everything.
// Holding them together is what makes "half the pair" unrepresentable, which is the
// mistake a caller would otherwise make by setting one field and forgetting the other.
//
// The zero value means "outside anybody's login session", which is the supported
// deployment and the answer that keeps enforcement on.
type SelfLocation struct {
	UID         uint32
	InUserSlice bool
}

// Declines reports whether a session belonging to uid must not be locked, because
// locking it would take away the screen this process is stopped from.
//
// This is the whole rule, in one place, on purpose. The enforcement loop and the
// dashboard both have to answer it, and if they each carry their own copy then the
// dashboard can announce an account as locked out at exactly the moment the engine
// is declining to lock it — a claim that reads as reassurance and is false.
func (l SelfLocation) Declines(uid uint32) bool {
	return l.InUserSlice && uid == l.UID
}
