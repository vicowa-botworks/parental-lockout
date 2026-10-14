//go:build unix

package platform

import (
	"os"
	"syscall"
)

// fileOwner returns the uid that owns a file.
//
// It is behind a function with a non-Unix twin because syscall.Stat_t does not exist
// everywhere the package is compiled, and because the caller wants "I could not tell"
// to be a value it can act on rather than a panic on a type assertion.
func fileOwner(path string) (uid uint32, ok bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return st.Uid, true
}
