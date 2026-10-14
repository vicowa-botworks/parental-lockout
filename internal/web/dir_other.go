//go:build !unix

package web

import (
	"fmt"
	"os"
)

// ensurePrivateDir has no O_NOFOLLOW and no Fchmod to ask for on this platform.
//
// The daemon only ever runs on Linux — it shells out to loginctl and reads
// /proc — so this file exists so the package compiles for a developer on another
// OS, not so the product works there. It therefore declines rather than
// pretending: a certificate store that silently skips the private-directory
// checks would be the one copy of this code that does not enforce the property the
// key's security depends on.
func ensurePrivateDir(path string) error {
	if li, err := os.Stat(path); err == nil {
		if !li.IsDir() {
			return fmt.Errorf("%s is not a directory", path)
		}
		return nil
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	return nil
}
