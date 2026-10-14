//go:build !unix

package platform

// fileOwner has no owner to ask about on this platform.
//
// Reporting "unknown" is the only honest answer, and the caller treats unknown as
// "not trusted", so a build that cannot ask never corroborates. The daemon only ever
// runs on Linux — it reads /proc and shells out to loginctl — so this file exists so
// the package compiles elsewhere, not so the product works there.
func fileOwner(string) (uid uint32, ok bool) { return 0, false }
