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
