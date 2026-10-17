package platform

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// LocationOf answers the self-session question about another process, which is what
// `plock status` needs in order to say why it will not lock a session. The two things
// it must never do are answer about the wrong pid, and answer "outside" when the read
// failed — the second is how "could not tell" ships as "declines nobody".

func TestLocationOfReadsThePidItWasGiven(t *testing.T) {
	var got []string
	read := func(path string) ([]byte, error) {
		got = append(got, path)
		return []byte("0::/user.slice/user-1000.slice/user@1000.service/app.slice/plock.service\n"), nil
	}
	loc, err := LocationOf(4321, read)
	if err != nil {
		t.Fatalf("LocationOf: %v", err)
	}
	if len(got) != 1 || got[0] != "/proc/4321/cgroup" {
		t.Errorf("read %v, want exactly [/proc/4321/cgroup]", got)
	}
	if !loc.InUserSlice || loc.UID != 1000 {
		t.Errorf("got %+v, want uid 1000 inside a user slice", loc)
	}
}

// A system unit is the supported deployment and a real answer, not a missing one: it is
// the case where the engine declines nobody and says so.
func TestLocationOfAnswersOutsideForASystemUnit(t *testing.T) {
	loc, err := LocationOf(7, func(string) ([]byte, error) {
		return []byte("0::/system.slice/plock.service\n"), nil
	})
	if err != nil {
		t.Fatalf("LocationOf: %v", err)
	}
	if loc != (SelfLocation{}) {
		t.Errorf("got %+v, want the zero location (outside anybody's session)", loc)
	}
}

// The reason this function returns an error where ReadSelfUserSlice swallows one: a
// reporting surface that renders a failed read as "outside" is asserting that no
// session is declined on evidence that it could not read anything.
func TestLocationOfReportsAFailedReadRatherThanAnsweringOutside(t *testing.T) {
	boom := errors.New("permission denied")
	loc, err := LocationOf(9, func(string) ([]byte, error) { return nil, boom })
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the reader's own error", err)
	}
	if loc != (SelfLocation{}) {
		t.Errorf("got %+v alongside an error, want the zero value", loc)
	}
}

// A pid of 0 is what systemd prints for a unit that is not running, and a negative one
// is not a process. Neither may reach the filesystem: "/proc/0/cgroup" is a real path
// that belongs to the kernel's swapper, and reading it would answer about a process
// nobody asked about.
func TestLocationOfRefusesAPidThatIsNotAProcess(t *testing.T) {
	for _, pid := range []int{0, -1} {
		t.Run(strconv.Itoa(pid), func(t *testing.T) {
			var touched string
			loc, err := LocationOf(pid, func(path string) ([]byte, error) {
				touched = path
				return []byte("0::/user.slice/user-1000.slice/user@1000.service\n"), nil
			})
			if err == nil {
				t.Errorf("pid %d: err = nil, want a refusal", pid)
			}
			if touched != "" {
				t.Errorf("pid %d read %q, want no read at all", pid, touched)
			}
			if loc != (SelfLocation{}) {
				t.Errorf("pid %d: got %+v with an error, want the zero value", pid, loc)
			}
		})
	}
}

// A nil reader is a wiring mistake, not a machine without /proc. Answering "outside"
// here would hide it as a deployment fact.
func TestLocationOfRefusesANilReader(t *testing.T) {
	if _, err := LocationOf(12, nil); err == nil {
		t.Fatal("nil reader: err = nil, want a refusal")
	}
}

// The path is built by this package, so the exact shape is part of what the tests above
// depend on: a pid interpolated into the wrong element reads some other process's
// cgroup and answers confidently about the wrong thing.
func TestCgroupPathOfIsTheKernelLocationForThatPid(t *testing.T) {
	if got := CgroupPathOf(4321); got != "/proc/4321/cgroup" {
		t.Errorf("got %q", got)
	}
	if strings.Contains(CgroupPathOf(1), "//") {
		t.Error("path contains an empty element")
	}
}
