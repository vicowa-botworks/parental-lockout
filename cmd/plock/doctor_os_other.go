//go:build !unix

package main

import (
	"errors"
	"os"
)

// Non-unix counterparts for the three host facts the doctor reads. The unix file carries
// the real ones; this file exists because without it the symbols vanish on windows and
// the package stops compiling -- which is the failure mode where both halves of a build-tag
// pair drop out and nothing until CI asks for a platform nobody tested.
//
// Nothing here pretends to succeed. A stand-in that returned 0 would read as "owned by
// root" and "running as root" on the one platform where both are lies, and a doctor check
// that lies in the passing direction is worse than a check that is missing.

// fileOwnerUID reports that the owner is unknown. The unix version returns -1 for the same
// reason when a FileInfo carries no Stat_t, so the caller already has to render -1 as
// "unknown" rather than as a uid.
func fileOwnerUID(os.FileInfo) int { return -1 }

var (
	// osStat is the same seam as the unix one; os.Stat is portable.
	osStat = os.Stat

	// osAccess has no portable meaning. POSIX access(2) answers "could this real uid
	// read/write/execute this path", and there is no equivalent to ask on the platforms
	// this file builds for -- an ACL is not a mode, and Go's FileInfo reports only the
	// owner-write bit there. So it refuses instead of guessing: a caller that treats an
	// error as "the check could not run" stays honest, and a caller that treated a nil
	// here as "writable" would have been wrong anyway.
	osAccess = func(string, uint32) error {
		return errors.New("access checks are not supported on this platform")
	}

	// currentUID is os.Geteuid, which returns -1 on Windows. checkRoot then reports "not
	// root", which is the accurate answer for a daemon this build cannot run anyway.
	currentUID = os.Geteuid
)
