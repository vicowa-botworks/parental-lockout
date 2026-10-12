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
