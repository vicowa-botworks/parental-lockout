package platform

import (
	"path"
	"testing"
)

// The desktop backend resolves gdbus through PATH and execs the absolute path it found, so
// argv[0] is no longer the literal "gdbus". Two things follow for tests, and both are pinned
// here rather than repeated at each call site.

// stubGdbusPath fixes what the backend resolves gdbus to, and is the only reason the desktop
// tests still say anything about a specific path.
//
// Without it the assertions would depend on whether the machine running them happens to have
// gdbus installed, and on where -- two things that differ between a developer box and CI while
// the code under test is identical. A test that passes because the host had the binary is not
// testing the backend.
func stubGdbusPath(t *testing.T, p string) {
	t.Helper()
	prev := lookPath
	lookPath = func(string) (string, error) { return p, nil }
	t.Cleanup(func() { lookPath = prev })
}

// stubGdbusMissing makes the resolution fail, which is the shape where the tool is not there at
// all. Probe reports this; Lock must refuse rather than build an argv around a path it never had.
func stubGdbusMissing(t *testing.T) {
	t.Helper()
	prev := lookPath
	lookPath = func(string) (string, error) { return "", errLookPathMissing }
	t.Cleanup(func() { lookPath = prev })
}

// errLookPathMissing stands in for the exec package's own "not found" so a test can assert the
// refusal without depending on the host's PATH.
var errLookPathMissing = errString("executable file not found in $PATH")

type errString string

func (e errString) Error() string { return string(e) }

// isProgram reports whether argv invokes the named tool, ignoring the directory it was resolved
// from. Comparing argv[0] to a bare name would now be false on every real host, and a check that
// is quietly false is worse than no check: the assertion that confirming never calls gdbus would
// pass even if confirming called nothing but gdbus.
func isProgram(argv []string, name string) bool {
	return len(argv) > 0 && path.Base(argv[0]) == name
}
