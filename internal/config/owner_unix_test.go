//go:build unix

package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// fakeInfo stands in for a file on disk. The ownership rule cannot be tested
// against real files without a second account on the machine, and the rule is
// exactly the kind that must not be trusted to be right.
type fakeInfo struct {
	mode os.FileMode
	uid  uint32
	sys  any
}

func (f fakeInfo) Name() string       { return "config.json" }
func (f fakeInfo) Size() int64        { return 100 }
func (f fakeInfo) Mode() os.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() any           { return f.sys }

func TestCheckTargetAcceptsAFileWeOwn(t *testing.T) {
	fi := fakeInfo{mode: 0o600, uid: 1000, sys: &syscall.Stat_t{Uid: 1000}}
	if err := checkTarget("/etc/plock/config.json", fi, 1000); err != nil {
		t.Errorf("checkTarget refused a file we own: %v", err)
	}
}

func TestCheckTargetRefuses(t *testing.T) {
	cases := []struct {
		name string
		fi   os.FileInfo
		euid int
		want string
	}{
		{
			name: "owned by another user",
			fi:   fakeInfo{mode: 0o600, uid: 65534, sys: &syscall.Stat_t{Uid: 65534}},
			euid: 1000,
			want: "owned by uid 65534",
		},
		{
			// The realistic attack: a child replaces the file with one they own
			// and waits for the daemon to overwrite it as root.
			name: "root overwriting a child-owned file",
			fi:   fakeInfo{mode: 0o666, uid: 1001, sys: &syscall.Stat_t{Uid: 1001}},
			euid: 0,
			want: "owned by uid 1001",
		},
		{
			name: "symlink",
			fi:   fakeInfo{mode: os.ModeSymlink | 0o777, uid: 0, sys: &syscall.Stat_t{Uid: 0}},
			euid: 0,
			want: "symlink",
		},
		{
			name: "directory",
			fi:   fakeInfo{mode: os.ModeDir | 0o700, uid: 0, sys: &syscall.Stat_t{Uid: 0}},
			euid: 0,
			want: "not a regular file",
		},
		{
			name: "device node",
			fi:   fakeInfo{mode: os.ModeDevice | 0o660, uid: 0, sys: &syscall.Stat_t{Uid: 0}},
			euid: 0,
			want: "not a regular file",
		},
		{
			// No ownership data means no verdict, and no verdict is a refusal.
			name: "no stat data from the operating system",
			fi:   fakeInfo{mode: 0o600, sys: nil},
			euid: 0,
			want: "cannot check the owner",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkTarget("/etc/plock/config.json", tc.fi, tc.euid)
			if err == nil {
				t.Fatal("checkTarget accepted the file")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// The shape check has to come before the open: opening a FIFO for reading waits
// for a writer, so a config replaced by a pipe would stop the enforcement loop
// where it stands, with no error and no timeout.
func TestLoadDoesNotBlockOnAFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("cannot create a FIFO here: %v", err)
	}

	done := make(chan error, 1)
	go func() { _, err := Load(path); done <- err }()

	select {
	case err := <-done:
		if err == nil {
			t.Error("Load of a FIFO succeeded, want refusal")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Load blocked opening a FIFO")
	}
}

// The guarantee lives in the open, not in the check before it. Whoever can write
// to the configuration directory can replace the file with a FIFO after the
// shape has been checked and before the open lands, and an open that waits for a
// writer stops the enforcement loop with no error and no timeout.
//
// O_NONBLOCK does not make this open fail — a read-side open of a FIFO with no
// writer succeeds and would read as EOF — so what is asserted here is only that
// it never waits. Refusing the result is openReadable's job, on the descriptor.
func TestOpenNoFollowNeverBlocksOnAFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("cannot create a FIFO here: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		f, err := openNoFollow(path)
		if err == nil {
			f.Close()
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("openNoFollow blocked waiting for a FIFO writer that will never come")
	}
}

// The half the open cannot do: a FIFO that appeared behind the path after the
// shape was checked has to be refused, not read as an empty document. An empty
// read would reach the decoder as "unexpected EOF" and the daemon would keep the
// last good config — but a device or a socket could answer with anything.
func TestOpenReadableRefusesAFIFOSwappedInAfterTheCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("cannot create a FIFO here: %v", err)
	}

	f, err := openReadable(path)
	if err == nil {
		f.Close()
		t.Fatal("openReadable opened a FIFO, want refusal")
	}
	if !strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("error = %v, want it to say the path is not a regular file", err)
	}
}

// The same window, aimed at a symlink: without O_NOFOLLOW the open follows the
// link that appeared after the check, and the read path is back to reading
// whatever the path points at.
func TestOpenNoFollowRefusesASymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "shadow")
	if err := os.WriteFile(target, []byte("not the config"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	link := filepath.Join(dir, "config.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	f, err := openNoFollow(link)
	if err == nil {
		f.Close()
		t.Error("openNoFollow opened through a symlink, want refusal")
	}
}

// The end-to-end shape of the attack: a child with write access to the config
// directory flips the path between a file and a FIFO while the daemon polls,
// which is what Load does every tick. This can only fail when the bug is
// present — if the window is never hit the loop simply finishes — so it is a
// regression test rather than a coin toss.
func TestLoadSurvivesAPathSwappedUnderIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body, err := json.Marshal(sample())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	stop := make(chan struct{})
	flipped := make(chan struct{})
	go func() {
		defer close(flipped)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			os.Remove(path)
			if i%2 == 0 {
				syscall.Mkfifo(path, 0o600)
			} else {
				os.WriteFile(path, body, 0o600)
			}
		}
	}()

	opens := make(chan struct{})
	go func() {
		defer close(opens)
		for i := 0; i < 20000; i++ {
			if f, err := openReadable(path); err == nil {
				f.Close()
			}
		}
	}()

	select {
	case <-opens:
		close(stop)
		<-flipped
	case <-time.After(20 * time.Second):
		os.Remove(path)
		close(stop)
		<-flipped
		t.Fatal("openReadable parked: a path swapped for a FIFO stopped the read path with no error and no timeout")
	}
}

// The path says "regular file". What the descriptor holds is a pipe.
//
// This is the attack one step later than the swap the open closes, and it is why
// the shape is judged on the file we hold rather than on the name: reading the
// pipe would answer the daemon with whatever a writer chose, while the name swore
// it was a configuration. Renaming the pipe out of the way and putting an
// ordinary file at the name is enough to tell the two checks apart, because the
// open descriptor keeps the pipe and the path keeps the name.
func TestCheckRegularJudgesTheDescriptorNotThePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("cannot create a FIFO here: %v", err)
	}

	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Skipf("cannot open a FIFO without a writer here: %v", err)
	}
	defer f.Close()

	if err := os.Rename(path, filepath.Join(dir, "pipe")); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err = checkRegular(f)
	if err == nil {
		t.Fatal("checkRegular accepted a pipe because the path now names a regular file")
	}
	if !errors.Is(err, errNotRegular) {
		t.Errorf("error = %v, want errNotRegular — the file has to be refused for what it is, not because the name moved", err)
	}
}
