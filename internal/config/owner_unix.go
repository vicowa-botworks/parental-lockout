//go:build unix

package config

import (
	"errors"
	"os"
	"syscall"
)

// fileUID returns the owning uid of an existing file.
func fileUID(fi os.FileInfo) (int, error) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("no ownership data available from the operating system")
	}
	return int(st.Uid), nil
}

// fileIno returns the inode number, which changes whenever a file is replaced
// even if the size and timestamp are reproduced exactly.
func fileIno(fi os.FileInfo) uint64 {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return st.Ino
}

// openNoFollow opens an existing file for reading without following a final
// symlink and without ever blocking in the open itself. The caller has already
// rejected the symlink case; this is what makes the check and the open one
// operation instead of a race.
//
// O_NONBLOCK has nothing to do with non-blocking I/O here. Opening a FIFO for
// reading waits for a writer, and for a config file no writer will ever come:
// the call would never return, and the enforcement loop would stop with no error
// and no timeout. With the flag the open fails immediately instead. It is
// ignored for regular files.
func openNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

// fchmodFile sets the mode of an open file. Working on the descriptor is what
// keeps "tighten the file we read" from becoming "tighten whatever the path
// points at now".
func fchmodFile(f *os.File, perm os.FileMode) error {
	return syscall.Fchmod(int(f.Fd()), uint32(perm.Perm()))
}
