//go:build unix

package config

import (
	"os"
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
