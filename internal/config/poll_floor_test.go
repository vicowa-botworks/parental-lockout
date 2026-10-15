package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// deploy/run-dev.sh refuses a PLOCK_DEV_POLL below the daemon's floor, and the number it
// refuses with is written in a shell script because a shell script cannot read a Go
// constant. That is a copy of a constant in a second language, which is exactly how the
// two come to disagree: somebody changes MinPollSeconds, the Go side keeps working, and
// the dev script goes on accepting a value the daemon will reject at `plock init` -- after
// it has already written a dev root.
//
// The alternative considered was letting `plock init` produce the refusal and deleting the
// script's check. Rejected because that refusal names settings.pollSeconds, not
// PLOCK_DEV_POLL, and arrives after the files exist.
func TestTheDevScriptsPollFloor(t *testing.T) {
	src := readRepoFile(t, "deploy/run-dev.sh")

	// Every integer the script compares PLOCK_DEV_POLL against. Matched by the
	// comparison rather than by a bare number so an unrelated 5 elsewhere in the file
	// -- a default, a comment, a retry count -- cannot satisfy this test.
	re := regexp.MustCompile(`\$POLL"?[[:space:]]*-lt[[:space:]]*([0-9]+)`)
	matches := re.FindAllStringSubmatch(src, -1)
	if len(matches) == 0 {
		t.Fatal("deploy/run-dev.sh no longer compares $POLL with -lt, so the floor check " +
			"this test exists to pin is gone and a too-small PLOCK_DEV_POLL will fail later, " +
			"after the dev root has been written")
	}

	for _, m := range matches {
		got, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("the script's floor %q is not a number: %v", m[1], err)
		}
		if got != MinPollSeconds {
			t.Errorf("deploy/run-dev.sh refuses PLOCK_DEV_POLL below %d, but config.MinPollSeconds is %d -- "+
				"the script would write a dev root the daemon then rejects at init", got, MinPollSeconds)
		}
	}
}

// readRepoFile reads a file relative to the repo root from a package's test directory.
//
// Walking up rather than hard-coding "../.." twice is what keeps this working if the test
// moves a directory deeper, and it refuses rather than skipping: a helper that returns ""
// when it cannot find the file would make every assertion above vacuously true, which is
// the meta-test blindness that took three rounds to find in cmd/plock's help-flag guard.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, rel)
		b, err := os.ReadFile(candidate)
		if err == nil {
			return string(b)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("cannot find %s walking up from the test's directory; the check that reads it "+
		"must fail rather than pass with no content", rel)
	return ""
}
