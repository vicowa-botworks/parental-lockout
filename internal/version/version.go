// Package version carries build metadata stamped in by the linker.
//
// The three variables are deliberately empty-by-default so a binary built with
// a bare `go build` still reports something honest ("dev") rather than a stale
// version number. Release builds set them via -ldflags -X (see the Makefile).
package version

import "fmt"

// Build identity. Overridden at link time.
var (
	// Version is `git describe --tags`, or "dev" for an untagged tree.
	Version = "dev"
	// Commit is the short SHA of the tree the binary was built from.
	Commit = "none"
	// Date is the UTC build timestamp in RFC 3339 form.
	Date = "unknown"
)

// String renders the one-line form printed by `plock version`.
func String() string {
	return fmt.Sprintf("plock %s (commit %s, built %s)", Version, Commit, Date)
}

// Fields returns the identity as separate values for machine-readable output.
func Fields() (ver, commit, date string) {
	return Version, Commit, Date
}
