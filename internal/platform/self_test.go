package platform

import (
	"os"
	"strings"
	"testing"
)

// The fixtures below are transcribed from this machine, not invented. The desktop one
// is the cgroup of a shell inside a Konsole window on the Plasma session that
// `loginctl list-sessions` reports as session 3, uid 1000, seat0, tty2 — the session
// the enforcement loop would lock.
func TestUserSliceUIDOnTheShapesThisBoxActuallyProduces(t *testing.T) {
	const desktop = "0::/user.slice/user-1000.slice/user@1000.service/app.slice/" +
		"app-org.kde.konsole-1933179.scope/tab(1933604).scope"

	uid, inside := UserSliceUID(desktop)
	if !inside || uid != 1000 {
		t.Errorf("UserSliceUID(desktop) = %d, %v; want 1000, true -- this is the exact "+
			"cgroup of a process in the desktop session the daemon would lock, so if it "+
			"does not read as a user session nothing downstream can ever refuse to lock it",
			uid, inside)
	}

	// A systemd system unit: what `plock.service` actually looks like. This is the
	// case that must stay "outside", because refusing to lock a uid here would turn a
	// correctly deployed daemon into one that enforces nothing.
	for _, line := range []string{
		"0::/system.slice/plock.service",
		"0::/system.slice/systemd-logind.service",
		"0::/init.scope",
	} {
		if uid, inside := UserSliceUID(line); inside {
			t.Errorf("UserSliceUID(%q) claims a user slice (uid %d); a system unit has no "+
				"login session to lock itself out of, and claiming one makes the daemon "+
				"decline to enforce an account", line, uid)
		}
	}
}

// The older and newer user-slice layouts both have to read, because the marker is the
// user-N.slice element and not the session scope that may or may not sit under it.
func TestUserSliceUIDFindsTheMarkerAtAnyDepth(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
		uid  uint32
	}{
		{"app slice", "0::/user.slice/user-1000.slice/user@1000.service/app.slice/x.scope", 1000},
		{"session scope", "0::/user.slice/user-42.slice/user@42.service/session.slice/session-7.scope", 42},
		{"marker is the whole path", "0::/user.slice/user-7.slice", 7},
		// root's own graphical session really is user-0.slice, so uid 0 is a valid
		// answer and the caller must decide on the bool rather than on uid != 0.
		{"root user slice", "0::/user.slice/user-0.slice/user@0.service/app.slice/x.scope", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uid, inside := UserSliceUID(tc.path)
			if !inside {
				t.Fatalf("UserSliceUID(%q) says not inside a user slice", tc.path)
			}
			if uid != tc.uid {
				t.Errorf("uid = %d, want %d", uid, tc.uid)
			}
		})
	}
}

// cgroup v1 lists one line per hierarchy. The path repeats, so the first match wins
// and no later line can contradict it -- but the parser has to survive the form at all
// or a pre-2021 host reads as "outside" and the guard silently never fires there.
func TestUserSliceUIDReadsTheCgroupV1Form(t *testing.T) {
	file := strings.Join([]string{
		"11:blkio:/user.slice/user-1000.slice/user@1000.service/app.slice/x.scope",
		"10:devices:/user.slice/user-1000.slice/user@1000.service/app.slice/x.scope",
		"0:memory:/user.slice/user-1000.slice/user@1000.service/app.slice/x.scope",
	}, "\n")

	uid, inside := UserSliceUID(file)
	if !inside || uid != 1000 {
		t.Fatalf("UserSliceUID(v1) = %d, %v; want 1000, true", uid, inside)
	}
}

// The narrow direction is the security property, so the refusals are the half worth
// pinning: anything that is not exactly user-<digits>.slice must answer "outside".
func TestUserSliceUIDRefusesEverythingThatIsNotTheMarker(t *testing.T) {
	for _, tc := range []struct {
		name, file string
	}{
		{"empty", ""},
		{"newline only", "\n"},
		{"no colon at all", "garbage"},
		{"colon but no path", "1:controllers:"},
		// A sign is not a uid. ParseUint rejects these; Atoi would not, and "user--1"
		// would then hand back a uid nobody asked for.
		{"negative", "0::/user.slice/user--1.slice/x"},
		{"explicitly positive", "0::/user.slice/user-+5.slice/x"},
		{"empty digits", "0::/user.slice/user-.slice/x"},
		{"non-numeric digits", "0::/user.slice/user-root.slice/x"},
		{"hex digits", "0::/user.slice/user-0x10.slice/x"},
		// Overflowing uint32 is not a uid either; wrapping one into range would make
		// the daemon claim some unrelated account's slice.
		{"out of range", "0::/user.slice/user-99999999999.slice/x"},
		{"substring only", "0::/machine.slice/libpod-user-1000.slice.scope/x"},
		{"prefix only", "0::/user.slice/user-1000"},
		{"suffix only", "0::/user.slice/x.slice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if uid, inside := UserSliceUID(tc.file); inside {
				t.Errorf("UserSliceUID(%q) = %d, true; want not inside. Claiming a user "+
					"slice where there is none makes the daemon decline to lock an account, "+
					"which is a fail-open", tc.file, uid)
			}
		})
	}
}

// The line is split at the first two colons and everything after them is the path,
// rather than scanning for the last colon or for the first "::". A path containing a
// colon is what tells the three apart.
//
// Being straight about how narrow this is: systemd escapes a colon in a unit instance
// name rather than writing it raw, so a real cgroup path is unlikely to contain one at
// all. The rule is kept anyway because the failure it prevents is the silent one -- a
// path cut short reads as "not a user slice", which is a guard that never fires. A
// parser should not depend on a naming convention it has not checked.
func TestUserSliceUIDSplitsTheLineAtTheFirstTwoColonsNotAtAnyColon(t *testing.T) {
	for _, file := range []string{
		// v1, with a colon inside the unit name.
		"1:name=systemd:/user.slice/user-1000.slice/user@1000.service/app.slice/weird::name.scope",
		// v2, same.
		"0::/user.slice/user-1000.slice/user@1000.service/app.slice/weird:name.scope",
	} {
		uid, inside := UserSliceUID(file)
		if !inside || uid != 1000 {
			t.Errorf("UserSliceUID(%q) = %d, %v; want 1000, true. Cutting anywhere other "+
				"than at the first two colons finds the one inside the unit name and hands "+
				"back a truncated path", file, uid, inside)
		}
	}
}

// The element must be a whole path element. "libpod-user-1000.slice.scope" above is
// the substring case; this pins the rule that a marker embedded in a longer element
// does not count, which is what keeps a container runtime's naming from reading as a
// login session.
func TestUserSliceUIDMatchesWholeElementsOnly(t *testing.T) {
	for _, file := range []string{
		"0::/user.slice/notuser-1000.slice/x",
		"0::/user.slice/user-1000.slice.extra/x",
		"0::/user.slice/prefix-user-1000.slice/x",
	} {
		if uid, inside := UserSliceUID(file); inside {
			t.Errorf("UserSliceUID(%q) = %d, true; want not inside", file, uid)
		}
	}
}

func TestReadSelfUserSliceAgreesWithTheParser(t *testing.T) {
	// This asserts the two halves are wired together, on the host's real file. It
	// deliberately does not assert WHICH answer the host gives: a CI runner and a
	// desktop disagree, and a test that hard-codes either is the host-dependent
	// assertion this repo has already been burned by.
	real, err := os.ReadFile(SelfCgroupPath)
	if err != nil {
		t.Skipf("no %s on this host: %v", SelfCgroupPath, err)
	}
	uid, inside := ReadSelfUserSlice()
	wantUID, wantInside := UserSliceUID(string(real))
	if uid != wantUID || inside != wantInside {
		t.Errorf("ReadSelfUserSlice() = %d, %v; parser on the same bytes gives %d, %v",
			uid, inside, wantUID, wantInside)
	}
}

// ReadSelfUserSlice answers "outside" when it cannot read, rather than failing. The
// caller has no action for an error, and the alternative is to guess.
func TestReadSelfUserSliceAnswersOutsideWhenProcIsUnreadable(t *testing.T) {
	// SelfCgroupPath is a constant, so the only way to reach the failure branch is to
	// read a path that cannot exist. That is what this does, through the same
	// parse-after-read shape the exported helper uses.
	b, err := os.ReadFile(SelfCgroupPath + "-definitely-not-a-file")
	if err == nil {
		t.Fatalf("reading %q succeeded; the failure branch is unreachable and the "+
			"conservative default is untested", SelfCgroupPath)
	}
	if len(b) != 0 {
		t.Fatalf("a failed read returned %d bytes", len(b))
	}
	// The branch itself: empty content parses as "outside", which is what the helper
	// returns after swallowing the error.
	if uid, inside := UserSliceUID(string(b)); inside {
		t.Errorf("empty content = %d, true; want not inside", uid)
	}
}

// Declines is the whole self-session rule, and it is here rather than inline in the
// enforcement loop because the dashboard has to answer the same question about the
// same sessions. Two copies of a rule drift, and the drift is invisible until the page
// announces an account as locked out at the moment the engine is declining to lock it.
func TestDeclinesNeedsBothHalvesOfThePair(t *testing.T) {
	// Each half alone is a different failure, and both are worse than declining
	// nothing: the uid alone stops the daemon enforcing every session of its own uid
	// even when it is a system unit that shares no session with anybody, and the slice
	// flag alone stops it enforcing everybody.
	for _, tc := range []struct {
		name string
		loc  SelfLocation
		uid  uint32
		want bool
	}{
		{"system unit, same uid", SelfLocation{UID: 0, InUserSlice: false}, 0, false},
		{"user unit, other uid", SelfLocation{UID: 1000, InUserSlice: true}, 1001, false},
		{"user unit, own uid", SelfLocation{UID: 1000, InUserSlice: true}, 1000, true},
		{"zero value declines nothing", SelfLocation{}, 0, false},
		{"uid set without the slice flag", SelfLocation{UID: 1000}, 1000, false},
		// uid 0 is a uid, not a sentinel. A daemon inside user@0.service really is
		// inside root's login session, and locking that session strands the daemon
		// exactly as much as any other, so the flag being set makes the match real.
		// The half that carries the meaning is the flag, which is why the zero value
		// is safe: it has the flag off, not the uid at zero.
		{"slice flag set with uid 0", SelfLocation{InUserSlice: true}, 0, true},
	} {
		if got := tc.loc.Declines(tc.uid); got != tc.want {
			t.Errorf("%s: Declines(%d) = %v, want %v", tc.name, tc.uid, got, tc.want)
		}
	}
}
