package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ensureDirUsable exists to answer a question ensureDir does not ask: what may the directory
// holding the config look like?
//
// Its ceiling is 0755, not the state directory's 0700, and the reason is in its comment: a
// packager may well have created /etc/plock at the mode every other /etc subdirectory has, and
// the secret in there is the file at 0600. Refusing 0755 would make this command the odd one out
// on machines that are configured correctly.
//
// Nothing tested that. The only loose-directory test in this package drives the *state*
// directory, whose ceiling is 0700 -- and a ceiling of 0700 refuses 0775 exactly as loudly as a
// ceiling of 0755 does. So the existing test passes whether the config ceiling is 0755, 0700 or
// 0500, which means the property the comment argues for had no test at all. Both halves are
// pinned here, because the ceiling's value is only meaningful as a pair: accept 0755, refuse
// anything more open.

func TestInitAcceptsAConfigDirectoryAtThePackagerMode(t *testing.T) {
	cfgPath, state := initPaths(t)
	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Skipf("cannot chmod: %v", err)
	}
	// The premise: the directory really is 0755, or the test below proves nothing.
	if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm() != 0o755 {
		t.Fatalf("fixture directory is %v, want 0755", fi.Mode().Perm())
	}

	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state)
	if code != exitOK {
		t.Fatalf("init refused a 0755 config directory, which is what a packager creates:\n%s", stderr)
	}
}

func TestInitRefusesAConfigDirectoryMoreOpenThanThePackagerMode(t *testing.T) {
	cfgPath, state := initPaths(t)
	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Group-writable: one bit past the ceiling.
	if err := os.Chmod(dir, 0o775); err != nil {
		t.Skipf("cannot chmod: %v", err)
	}

	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state)
	if code == exitOK {
		t.Fatal("init worked in a config directory the group can write to")
	}
	if !strings.Contains(stderr, "0775") {
		t.Errorf("the refusal should name the mode it objected to, got:\n%s", stderr)
	}
	// Refusing must not silently repair: chmodding a directory we did not create is the
	// arbitrary-chmod primitive this package's own rules forbid.
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o775 {
		t.Errorf("init changed the mode of a directory it did not create: %04o", fi.Mode().Perm())
	}
}
