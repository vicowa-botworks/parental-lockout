package version

import (
	"strings"
	"testing"
)

// withIdentity sets the linker-overridable variables for one test and puts the
// real values back afterwards.
func withIdentity(t *testing.T, ver, commit, date string) {
	t.Helper()
	oldV, oldC, oldD := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = oldV, oldC, oldD })
	Version, Commit, Date = ver, commit, date
}

func TestStringRendersIdentityInDocumentedOrder(t *testing.T) {
	withIdentity(t, "v0.1.0", "abc1234", "2026-10-12T10:00:00Z")

	got := String()
	// Not just "contains": the labels are the output contract, so the values
	// must sit under the right label.
	want := "plock v0.1.0 (commit abc1234, built 2026-10-12T10:00:00Z)"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// TestDefaultsAreHonest guards the property named in the package doc: a binary
// built with a bare `go build` must not claim a release version. Checking only
// for non-emptiness would let `Version = "v9.9.9"` through.
func TestDefaultsAreHonest(t *testing.T) {
	if Version != "dev" {
		t.Errorf("Version = %q, want %q — a build without -ldflags must not claim a release",
			Version, "dev")
	}
	if Commit != "none" {
		t.Errorf("Commit = %q, want %q", Commit, "none")
	}
	if Date != "unknown" {
		t.Errorf("Date = %q, want %q", Date, "unknown")
	}
	if !strings.HasPrefix(String(), "plock dev ") {
		t.Errorf("String() = %q, want it to start with %q", String(), "plock dev ")
	}
}
