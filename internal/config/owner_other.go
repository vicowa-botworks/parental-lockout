//go:build !unix

package config

import "os"

// fileUID has no answer on this platform. Reporting that is deliberate: the
// tamper guard refuses to write rather than skipping the ownership check, so a
// future Windows port has to implement this instead of quietly losing the rule.
func fileUID(os.FileInfo) (int, error) {
	return 0, errOwnershipUnsupported
}

// fileIno has no answer here, so the change detector falls back to mtime and
// size alone — the behaviour before the inode was added, not a new risk.
func fileIno(os.FileInfo) uint64 {
	return 0
}
