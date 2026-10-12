//go:build !unix

package config

import "os"

// fileUID has no answer on this platform. Reporting that is deliberate: the
// tamper guard refuses to write rather than skipping the ownership check, so a
// future Windows port has to implement this instead of quietly losing the rule.
func fileUID(os.FileInfo) (int, error) {
	return 0, errOwnershipUnsupported
}
