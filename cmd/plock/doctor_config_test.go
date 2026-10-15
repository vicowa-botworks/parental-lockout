package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// The config-file checks are the ones that tell a parent the machine is leaking the very
// thing the product protects. A world-readable /etc/plock/config.json holds the admin
// password verifier and every child's schedule, and `doctor` is the only place that says so
// before the daemon is trusted with it.
//
// These run against real files with real modes rather than a fake FileInfo. A fake reports
// what the test asserts about it; a file reports what the filesystem actually says, which
// is the thing the check has to get right.

func TestCheckConfigPathRefusesAGroupOrWorldReadableDocument(t *testing.T) {
	for _, tc := range []struct {
		mode os.FileMode
		fail bool
	}{
		{0o600, false},
		{0o400, false},
		// The three that leak. Each is a different bit, and a check written as
		// `perm&0o007` would pass the group cases and catch only these.
		{0o640, true},
		{0o604, true},
		{0o644, true},
		{0o666, true},
	} {
		t.Run(fmt.Sprintf("%04o", tc.mode), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte("{}"), tc.mode); err != nil {
				t.Fatal(err)
			}
			// WriteFile masks the mode with the umask, so set it outright: the check is
			// about the mode on the file, not the mode requested.
			if err := os.Chmod(path, tc.mode); err != nil {
				t.Fatal(err)
			}

			got := checkConfigPath(path, os.Stat)
			if tc.fail && got.Status == verdictOK {
				t.Errorf("a mode %04o config was reported as fine; it holds the admin credential", tc.mode)
			}
			if !tc.fail && got.Status != verdictOK {
				t.Errorf("a mode %04o config was refused: %s", tc.mode, got.Detail)
			}
			if tc.fail {
				// The remedy has to be in the message: this check is read over SSH by
				// somebody who is about to fix it, and "insecure permissions" without
				// the number is a sentence that sends them to a man page.
				if !strings.Contains(got.Detail, fmt.Sprintf("%04o", tc.mode)) {
					t.Errorf("the refusal does not name the mode it saw: %q", got.Detail)
				}
			}
		})
	}
}

// The advice must be the mode that fixes it, not a fixed 0600. A file at 0640 needs 0600;
// one at 0604 needs 0600; but a file whose owner bits are already narrower than 0600 must
// not be told to gain bits.
func TestCheckConfigPathAdviceOnlyRemovesBits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	got := checkConfigPath(path, os.Stat)
	if got.Fix == "" {
		t.Fatal("no remedy offered for a readable config")
	}
	if !strings.Contains(got.Fix, "chmod 600") {
		t.Errorf("hint is %q, expected it to name chmod 600", got.Fix)
	}
}

// A FIFO at the config path is not a config file, and the daemon refuses it. doctor
// agreeing with the daemon is the point: a check that says "looks fine" about a file the
// daemon will not open is a check that sends the operator to the wrong component.
func TestCheckConfigPathRefusesANonRegularFile(t *testing.T) {
	dir := t.TempDir()
	pipe := filepath.Join(dir, "config.json")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatalf("cannot create a FIFO for the fixture: %v", err)
	}

	got := checkConfigPath(pipe, os.Stat)
	if got.Status == verdictOK {
		t.Fatal("a FIFO was reported as a usable config path")
	}
	if !strings.Contains(got.Detail, "regular file") {
		t.Errorf("refused without naming the shape: %q", got.Detail)
	}
}

// A missing config is the most common real state of a new machine, and the answer has to
// point at `plock init` rather than at a generic read error.
func TestCheckConfigPathPointsAtInitWhenNothingIsThere(t *testing.T) {
	got := checkConfigPath(filepath.Join(t.TempDir(), "absent.json"), os.Stat)
	if got.Status == verdictOK {
		t.Fatal("a missing config was reported as fine")
	}
	if !strings.Contains(got.Fix, "plock init") {
		t.Errorf("hint is %q, expected it to name plock init", got.Fix)
	}
}

// The stat error itself must reach the message. A check that reports "not found" for every
// failure hides an EACCES (wrong directory permissions, the usual cause) behind the advice
// for a missing file, and the operator creates a second config somewhere else.
func TestCheckConfigPathCarriesTheStatError(t *testing.T) {
	boom := errors.New("permission denied")
	got := checkConfigPath("/etc/plock/config.json", func(string) (os.FileInfo, error) {
		return nil, boom
	})
	if got.Status == verdictOK {
		t.Fatal("a failing stat was reported as fine")
	}
	if !strings.Contains(got.Detail, "permission denied") {
		t.Errorf("the underlying error is missing from %q", got.Detail)
	}
}

// config.Save writes a temporary file in the directory and renames it over the document, so
// a writable file inside a read-only directory still cannot be saved. The two fail
// independently and both have to be reported, or the operator chmods the file and the
// daemon still cannot persist an override.
func TestCheckConfigWritableChecksTheDirectoryToo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Both writable: the control, so the two failures below cannot be an artifact of the
	// fixture being unwritable for an unrelated reason.
	okAccess := func(string, int) error { return nil }
	if got := checkConfigWritable(path, okAccess); got.Status != verdictOK {
		t.Fatalf("control refused a writable file and directory: %s", got.Detail)
	}

	// The file writable, the directory not.
	fileOnly := func(p string, _ int) error {
		if p == path {
			return nil
		}
		return errors.New("read-only file system")
	}
	got := checkConfigWritable(path, fileOnly)
	if got.Status == verdictOK {
		t.Fatal("a writable file in a read-only directory was reported as saveable")
	}
	// The message must name the directory as the problem. Naming the file would send the
	// operator to chmod the wrong thing.
	if !strings.Contains(got.Detail, "directory") {
		t.Errorf("the refusal does not name the directory: %q", got.Detail)
	}

	// The directory writable, the file not: the read-only bind mount over a writable
	// directory, which is a real container layout.
	dirOnly := func(p string, _ int) error {
		if p == path {
			return errors.New("read-only file system")
		}
		return nil
	}
	got = checkConfigWritable(path, dirOnly)
	if got.Status == verdictOK {
		t.Fatal("a read-only file was reported as saveable")
	}
	if strings.Contains(got.Detail, "directory holding") {
		t.Errorf("the file failure was reported as a directory failure: %q", got.Detail)
	}
}
