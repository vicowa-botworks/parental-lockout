//go:build unix

package main

import (
	"os"
	"syscall"
)

// fileOwnerUID reads the uid that owns a file.
//
// Reported rather than enforced: whether the owner is "right" depends on how the box was set
// up, and a tool that guessed would be wrong on some install. What is not a judgement is the
// number itself — an operator who sees `uid 1000` on /etc/plock/config.json knows
// immediately that a child owns the file holding their own schedule.
func fileOwnerUID(fi os.FileInfo) int {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return int(st.Uid)
	}
	// No Stat_t means a platform this build was not written for. Saying so is better than
	// returning 0, which would read as "owned by root" on the box where that is a lie.
	return -1
}

// osStat and osAccess are the seams the config checks take, so a test can state a mode
// without creating a file with a mode the test runner's umask would not allow.
var (
	osStat   = os.Stat
	osAccess = syscall.Access
)

// currentUID is the seam for the root check. It is a variable rather than a call so a test
// can state "not root" without a setuid harness.
var currentUID = os.Geteuid
