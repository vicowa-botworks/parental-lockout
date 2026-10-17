package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// The probe behind `plock status`'s declined marks. Two steps, two ways to fail:
// systemd may be unreachable, and the pid it names may have a cgroup this user cannot
// read. Each has to stay its own answer, because each supports a different sentence and
// a report that blurs them asserts something the machine did not say.

// recRunner is runnerFor plus the record of what was asked: the property name is the
// whole contract with systemctl, and a probe that silently asks for something else reads
// an empty answer as "no daemon".
type recRunner struct {
	out  []byte
	err  error
	argv [][]string
}

func (r *recRunner) Run(_ context.Context, argv []string, _ []string, _ uint32) ([]byte, error) {
	r.argv = append(r.argv, argv)
	return r.out, r.err
}

func TestParseMainPID(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want int
	}{
		{"the property as printed", "MainPID=4321\n", 4321},
		{"no trailing newline", "MainPID=4321", 4321},
		{"an inactive unit prints the key with no value", "MainPID=\n", 0},
		{"an unknown unit prints nothing", "", 0},
		{"a zero pid", "MainPID=0\n", 0},
		{"whitespace around the value", "  MainPID= 77 \n", 77},
		{"a value that is not a number", "MainPID=none\n", 0},
		{"a negative pid is not a process", "MainPID=-3\n", 0},
		{"a pid that will not fit a pid_t", "MainPID=99999999999999\n", 0},
		{"another property, then the one asked for", "Id=plock.service\nMainPID=12\n", 12},
		{"a key that merely ends in the name", "ExecMainPID=99\n", 0},
		{"a key that merely contains the name", "XMainPID=99\n", 0},
		// systemd really does print both of these keys, and ExecMainPID first. Matching
		// the name anywhere in the line stops at the decoy and answers 0 -- "no daemon"
		// -- for a service that is running, which is the wrong sentence on a healthy
		// machine.
		{"the decoy property before the one asked for", "ExecMainPID=99\nMainPID=12\n", 12},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseMainPID([]byte(tc.out)); got != tc.want {
				t.Errorf("parseMainPID(%q) = %d, want %d", tc.out, got, tc.want)
			}
		})
	}
}

func TestAskDaemonLocation(t *testing.T) {
	userSlice := []byte("0::/user.slice/user-1000.slice/user@1000.service/app.slice/plock.service\n")
	systemSlice := []byte("0::/system.slice/plock.service\n")

	t.Run("the pid's own cgroup is what gets read", func(t *testing.T) {
		var paths []string
		read := func(p string) ([]byte, error) { paths = append(paths, p); return userSlice, nil }
		run := &recRunner{out: []byte("MainPID=4321\n")}
		got := askDaemonLocation(run, read)
		if len(paths) != 1 || paths[0] != "/proc/4321/cgroup" {
			t.Errorf("read %v, want exactly the daemon's own cgroup", paths)
		}
		if !got.SystemdKnown || !got.Running || !got.LocationKnown {
			t.Errorf("got %+v, want known/running/known", got)
		}
		if !got.Location.Declines(1000) {
			t.Errorf("location %+v does not decline uid 1000", got.Location)
		}
		if got.Location.Declines(1001) {
			t.Error("location declines a different account")
		}
	})

	t.Run("it asks systemd for the main pid and nothing else", func(t *testing.T) {
		run := &recRunner{out: []byte("MainPID=1\n")}
		askDaemonLocation(run, func(string) ([]byte, error) { return systemSlice, nil })
		if len(run.argv) != 1 {
			t.Fatalf("made %d calls, want exactly one: %v", len(run.argv), run.argv)
		}
		joined := strings.Join(run.argv[0], " ")
		if joined != "systemctl show plock --property=MainPID" {
			t.Errorf("asked %q", joined)
		}
	})

	t.Run("a system unit runs nowhere anyone is logged in", func(t *testing.T) {
		got := askDaemonLocation(&recRunner{out: []byte("MainPID=9\n")},
			func(string) ([]byte, error) { return systemSlice, nil })
		if !got.Running || !got.LocationKnown {
			t.Fatalf("got %+v, want running with a known location", got)
		}
		if got.Location != (platform.SelfLocation{}) {
			t.Errorf("location %+v, want the zero value", got.Location)
		}
	})

	t.Run("no systemd is not the same answer as not running", func(t *testing.T) {
		// The error ExecRunner actually returns. The stdlib's own sentence
		// (`exec: "systemctl": executable file not found in $PATH`) was this fixture
		// until it was measured that lookupReason rewrites it on purpose -- so the
		// branch this test pins was matched by a string production never produces, and
		// a machine with no systemctl was being told its service is stopped.
		got := askDaemonLocation(&recRunner{err: fmt.Errorf("%s: %w", "systemctl", platform.ErrBinaryNotFound)},
			func(string) ([]byte, error) { return nil, errors.New("must not be called") })
		if got.SystemdKnown {
			t.Error("SystemdKnown true with no systemctl; that is the collapse this separates")
		}
		if got.Running {
			t.Error("Running true with no systemctl")
		}
		if got.AskFailed {
			t.Error("AskFailed true where there was nothing to ask; \"no init system\" and \"it refused\" are different sentences")
		}
	})

	// systemctl installed and refusing: a container that was not booted into systemd, a
	// stale SYSTEMD_ADDRESS, a bus nobody owns. None of these says anything about whether
	// a unit is running, and the probe used to answer "systemd answered, nothing is
	// running" for all of them.
	t.Run("systemctl refusing to answer is neither running nor stopped", func(t *testing.T) {
		refusal := errors.New("systemctl exited with 1: System has not been booted with systemd as init system (PID 1). Can't operate.")
		got := askDaemonLocation(&recRunner{err: refusal},
			func(string) ([]byte, error) { return nil, errors.New("must not be called") })
		if got.AskFailed == false {
			t.Error("a refusal must be its own state, not the absence of an answer")
		}
		if got.Running {
			t.Error("Running true on a call that failed")
		}
		if !got.SystemdKnown {
			t.Error("SystemdKnown false although systemctl was found and answered with an error")
		}
		if !strings.Contains(got.Err.Error(), "not been booted") {
			t.Errorf("the operator loses systemctl's reason: %v", got.Err)
		}
	})

	// The prose classifier's other direction. A bus failure whose text happens to name
	// ENOENT in lowercase read as "there is no init system here", sending the operator to
	// install systemd on a machine that has it.
	t.Run("a bus error naming ENOENT is still not 'no init system'", func(t *testing.T) {
		got := askDaemonLocation(&recRunner{err: errors.New(
			"systemctl exited with 1: Failed to connect to user scope bus via private socket: no such file or directory")},
			func(string) ([]byte, error) { return nil, errors.New("must not be called") })
		if !got.SystemdKnown {
			t.Error("concluded there is no init system from a bus error's prose")
		}
		if !got.AskFailed {
			t.Error("a bus nobody owns is a refusal, not an answer")
		}
	})

	// The kernel's answer does not depend on the call that failed, so a refusal that
	// printed a pid anyway still gets a location -- and the report says which half came
	// from where.
	t.Run("a refusal that printed a pid still has a location", func(t *testing.T) {
		got := askDaemonLocation(&recRunner{out: []byte("MainPID=4321\n"), err: errors.New("systemctl exited with 1: noisy")},
			func(string) ([]byte, error) { return userSlice, nil })
		if !got.AskFailed || !got.Running || !got.LocationKnown {
			t.Fatalf("got %+v, want refused, running, location known", got)
		}
		if !got.Location.InUserSlice || got.Location.UID != 1000 {
			t.Errorf("location %+v, want uid 1000's user slice", got.Location)
		}
	})

	t.Run("an inactive unit is a systemd that answered", func(t *testing.T) {
		got := askDaemonLocation(&recRunner{out: []byte("MainPID=\n")},
			func(string) ([]byte, error) { return nil, errors.New("must not be called") })
		if !got.SystemdKnown || got.Running {
			t.Errorf("got %+v, want systemd known and not running", got)
		}
		if got.LocationKnown {
			t.Error("LocationKnown true for a daemon that is not running")
		}
	})

	// The case the LocationKnown flag exists for. Rendering this as "declines nobody"
	// is the reassurance with nothing behind it, which is the failure the whole
	// three-state design is arranged to prevent.
	t.Run("an unreadable cgroup is known-unknown, not outside", func(t *testing.T) {
		got := askDaemonLocation(&recRunner{out: []byte("MainPID=4321\n")},
			func(string) ([]byte, error) { return nil, errors.New("permission denied") })
		if !got.Running {
			t.Error("Running false although systemd named a pid")
		}
		if got.LocationKnown {
			t.Error("LocationKnown true after a failed read")
		}
		if got.Location != (platform.SelfLocation{}) {
			t.Errorf("location %+v carried alongside a failed read", got.Location)
		}
	})

	t.Run("a nil runner answers nothing", func(t *testing.T) {
		got := askDaemonLocation(nil, func(string) ([]byte, error) { return userSlice, nil })
		if got != (daemonState{}) {
			t.Errorf("got %+v, want the zero state", got)
		}
	})
}
