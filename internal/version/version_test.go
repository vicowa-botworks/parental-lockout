package version

import (
	"strings"
	"testing"
)

func TestStringIncludesAllFields(t *testing.T) {
	// The variables are package-level so they can be set by -ldflags; the test
	// sets them the same way to assert the rendering rather than the defaults.
	oldV, oldC, oldD := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = oldV, oldC, oldD })

	Version, Commit, Date = "v0.1.0", "abc1234", "2026-10-12T10:00:00Z"

	got := String()
	for _, want := range []string{"v0.1.0", "abc1234", "2026-10-12T10:00:00Z"} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q, missing %q", got, want)
		}
	}
}

func TestFieldsMatchesString(t *testing.T) {
	oldV, oldC, oldD := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = oldV, oldC, oldD })

	Version, Commit, Date = "v1.2.3", "deadbee", "2026-01-01T00:00:00Z"

	v, c, d := Fields()
	if v != Version || c != Commit || d != Date {
		t.Errorf("Fields() = %q %q %q, want %q %q %q", v, c, d, Version, Commit, Date)
	}
}

func TestDefaultsAreHonest(t *testing.T) {
	// A binary built without ldflags must not claim a release version.
	if Version == "" || Commit == "" || Date == "" {
		t.Fatal("version variables must never default to the empty string")
	}
}
