//go:build unix

package web

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ensurePrivateDir makes dir exist as a directory only this process can read.
//
// The certificate directory holds a private key, so "created with 0700" is not
// enough: the mode on mkdir is masked by the umask, and an existing directory is
// whatever the previous installer left. So the mode is set with Fchmod on the
// descriptor this call opened, never chmod on a path — a root daemon that chmods a
// path it did not open becomes an arbitrary-chmod primitive the moment a symlink
// appears in it.
//
// An existing directory is refused rather than corrected. Silently tightening a
// directory somebody else made hides the fact that something else is writing here
// too, and the key is the thing that would be exposed.
func ensurePrivateDir(path string) error {
	// O_NOFOLLOW refuses to walk a symlink at the final component, and O_NONBLOCK
	// makes a FIFO at that path fail immediately instead of parking the open — a
	// blocking open here would stall startup forever, which for this daemon means
	// nobody gets locked out and nothing says so.
	fd, err := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err == nil {
		defer fd.Close()
		li, err := fd.Stat()
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if !li.IsDir() {
			return fmt.Errorf("%s is not a directory", path)
		}
		if li.Mode().Perm()&^0o700 != 0 {
			return fmt.Errorf("%s is mode %04o, which is more open than 0700: refusing to keep a private key in a directory somebody else can read. Run: chmod 700 %s", path, li.Mode().Perm(), path)
		}
		if st, ok := li.Sys().(*syscall.Stat_t); ok && st.Uid != uint32(os.Geteuid()) {
			return fmt.Errorf("%s is owned by uid %d and this process is uid %d", path, st.Uid, os.Geteuid())
		}
		// Re-assert the mode through the held descriptor. It is already correct if
		// the check above passed; this is here for the case where the directory was
		// created a moment ago by this same call and the umask had narrowed it.
		if err := syscall.Fchmod(int(fd.Fd()), 0o700); err != nil {
			return fmt.Errorf("chmod %s: %w", path, err)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("open %s: %w", path, err)
	}

	// Creating it: MkdirAll so a fresh machine's /var/lib/plock/tls works in one
	// step, then the check above runs on the result by recursing once. The mode
	// argument is a request the umask may narrow; the Fchmod in the recursive call
	// is the result.
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	return ensurePrivateDir(path)
}
