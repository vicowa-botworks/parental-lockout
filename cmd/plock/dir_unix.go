//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// deepestExistingAncestor walks up from path to the last component that is there.
//
// It stops at the first hit rather than resolving the whole chain because the
// question being answered is "which of these paths is the file", and the deepest
// existing one is the only answer an operator can act on.
func deepestExistingAncestor(path string) (string, bool) {
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		if fi, err := os.Lstat(dir); err == nil {
			return dir, !fi.IsDir()
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
	}
}

// ensureAncestor makes a directory that will hold the one we were asked for.
//
// It creates what is missing and tightens only what it created, and it does not
// judge a directory it did not make. If the ancestor exists at 0755 that is the
// system's decision about a system directory, and refusing it would mean this
// command cannot run on a normal machine.
func ensureAncestor(path string, mode os.FileMode) error {
	if li, err := os.Lstat(path); err == nil {
		if li.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink: refusing to create a directory through it", path)
		}
		if !li.IsDir() {
			return fmt.Errorf("%s exists and is not a directory", path)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	parent := filepath.Dir(path)
	if parent == path {
		return fmt.Errorf("refusing to create %q", path)
	}
	if err := ensureAncestor(parent, mode); err != nil {
		return err
	}
	pf, err := os.OpenFile(parent, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", parent, err)
	}
	defer pf.Close()
	leaf := filepath.Base(path)
	if err := syscall.Mkdirat(int(pf.Fd()), leaf, uint32(mode.Perm())); err != nil && !errors.Is(err, syscall.EEXIST) {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if err := syscall.Fchmodat(int(pf.Fd()), leaf, uint32(mode.Perm()), 0); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}

// geteuid is the account this command writes as, as a variable so the ownership
// guard can be exercised by an unprivileged test. The rule only fires for a
// directory owned by somebody else, and a test running as uid 1000 cannot create
// one — so without the seam the guard ships having never fired anywhere.
var geteuid = func() uint32 { return uint32(os.Geteuid()) }

// ensureDirPrivate makes sure path exists as a private directory at mode.
//
// A directory this command creates is this command's to set, and the mode is set
// relative to the parent's descriptor rather than by chmod on the path. A root CLI
// that chmods a path it did not create is an arbitrary-chmod primitive pointed at
// whatever --state-dir happens to say, and the mode in MkdirAll is only a request:
// the umask masks it, so a strict UMask= on the invoking unit can leave a
// directory the creator cannot open.
//
// A directory that already exists is checked and refused, never corrected. It came
// from somebody else's decision, and silently rewriting it both hides that a
// change happened and takes away the choice.
func ensureDirPrivate(path string, mode os.FileMode) error {
	return ensureDir(path, mode, 0o700)
}

// ensureDirUsable is ensureDir for a directory that holds the config.
//
// The ceiling is 0755 rather than 0700 because /etc/plock is a directory a
// packager may well have created at the mode every other /etc subdirectory has,
// and the secret in there is the file, which is 0600. Refusing 0755 would make
// this command the odd one out on machines that are configured correctly;
// refusing a world-writable one is the rule that actually protects anything.
func ensureDirUsable(path string, mode os.FileMode) error {
	return ensureDir(path, mode, 0o755)
}

func ensureDir(path string, mode, maxPerm os.FileMode) error {
	li, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		// ENOTDIR on the leaf means something above it is a file, and naming the
		// leaf is useless: the operator would look at the path they typed, which is
		// exactly what they meant. Walk up to the last thing that does exist and
		// name that instead, because that is the path to go and fix.
		if errors.Is(err, syscall.ENOTDIR) {
			if bad, ok := deepestExistingAncestor(path); ok {
				return fmt.Errorf("%s exists and is not a directory, so %s cannot be created inside it", bad, path)
			}
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if err == nil {
		if li.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink: refusing to treat it as the state directory", path)
		}
		if !li.IsDir() {
			return fmt.Errorf("%s is not a directory", path)
		}
		if li.Mode().Perm()&^maxPerm != 0 {
			return fmt.Errorf("%s is mode %04o, which is more open than %04o: refusing to work in a directory somebody else can reach", path, li.Mode().Perm(), maxPerm)
		}
		if st, ok := li.Sys().(*syscall.Stat_t); ok && st.Uid != geteuid() {
			return fmt.Errorf("%s is owned by uid %d and this process is uid %d", path, st.Uid, geteuid())
		}
		return nil
	}

	// The path is created one component at a time and each new component is
	// tightened immediately, rather than with MkdirAll.
	//
	// MkdirAll applies its mode to every level it creates, and that mode is masked
	// by the umask -- so under a hardened unit (UMask=0700) the *intermediate*
	// directory comes out with no permissions at all and the next component cannot
	// be created inside it. The error then says "permission denied" about a
	// directory this command made a moment ago, which reads like somebody else's
	// misconfiguration. Creating one level at a time and chmodding it before
	// descending means the directory the next mkdir writes into is always one this
	// process can actually use.
	parent := filepath.Dir(path)
	leaf := filepath.Base(path)
	if parent == path {
		return fmt.Errorf("refusing to treat %q as a directory to create inside its own parent", path)
	}
	// Ancestors are created but not policed. /var/lib is 0755 on every distribution
	// that has one and /etc is 0755 on all of them; applying the leaf's ceiling up
	// the chain would refuse a correctly configured machine. The check below is
	// only ever applied to the directory this command was pointed at.
	if err := ensureAncestor(parent, mode); err != nil {
		return err
	}
	pf, err := os.OpenFile(parent, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", parent, err)
	}
	defer pf.Close()
	// The mode here is a request and the umask masks it; the Fchmodat below is the
	// result. EEXIST is fine: a concurrent creator made exactly what we wanted and
	// the chmod still applies.
	if err := syscall.Mkdirat(int(pf.Fd()), leaf, uint32(mode.Perm())); err != nil && !errors.Is(err, syscall.EEXIST) {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if err := syscall.Fchmodat(int(pf.Fd()), leaf, uint32(mode.Perm()), 0); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}
