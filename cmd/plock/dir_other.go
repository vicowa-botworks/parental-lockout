//go:build !unix

package main

import (
	"fmt"
	"os"
)

// ensureDirPrivate makes sure path exists as a private directory.
//
// This platform has no POSIX permission bits, so "private" has no meaning here
// and the check cannot be performed. That is reported as a failure rather than
// skipped: a state directory whose privacy cannot be established is not one this
// command should be writing credentials beside.
func ensureDirPrivate(path string, mode os.FileMode) error {
	return ensureDir(path, mode, 0o700)
}

// ensureDirUsable is ensureDirPrivate with a looser ceiling, for a directory
// whose contents rather than whose listing is the secret.
func ensureDirUsable(path string, mode os.FileMode) error {
	return ensureDir(path, mode, 0o755)
}

func ensureDir(path string, mode, _ os.FileMode) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	return fmt.Errorf("%s: this platform cannot report or enforce directory permissions, so the state directory cannot be verified as private", path)
}
