package users

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every note string in this package is a claim that something will be explained to
// an operator. A note whose branch never runs is a claim nobody has tested, which is
// worse than no note at all: the code reads as if the case is handled. These tests
// exist to make each branch fire at least once.

func TestALoginDefsWithABareKeyIsSkippedNotGuessedAt(t *testing.T) {
	// "UID_MIN" with no value is a line a human editing in a hurry produces. Taking
	// the empty string as the value would fall back to the default silently; the
	// line must be skipped so the absent-key note is the one that fires.
	src := fixture(t, `casey:x:1000:1000::/home/casey:/bin/bash
`, `UID_MIN
UID_MAX 60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if l.UIDMin != DefaultUIDMin {
		t.Errorf("UIDMin = %d, want the default", l.UIDMin)
	}
	if !strings.Contains(joined(l.Notes), "no UID_MIN") {
		t.Errorf("a bare key was treated as a value rather than as an absent key: %v", l.Notes)
	}
}

func TestANonNumericUIDMaxIsReportedSeparatelyFromTheMin(t *testing.T) {
	src := fixture(t, `casey:x:1000:1000::/home/casey:/bin/bash
`, `UID_MIN 1000
UID_MAX sixty-k
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if l.UIDMax != DefaultUIDMax {
		t.Errorf("UIDMax = %d, want the default %d", l.UIDMax, DefaultUIDMax)
	}
	if !strings.Contains(joined(l.Notes), "UID_MAX") {
		t.Errorf("the ceiling fallback was silent: %v", l.Notes)
	}
}

// A record whose uid is not a number cannot be placed in or out of range, so it is
// dropped — and dropping a record without saying so is exactly how an account
// disappears.
func TestANonNumericUIDIsSkippedAndNamed(t *testing.T) {
	src := fixture(t, `oops:x:not-a-number:1000::/home/oops:/bin/bash
casey:x:1001:1001::/home/casey:/bin/bash
`, `UID_MIN 1000
UID_MAX 60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	wantNames(t, l, "casey")
	if !strings.Contains(joined(l.Notes), "oops") || !strings.Contains(joined(l.Notes), "uid") {
		t.Errorf("the dropped record was not identified: %v", l.Notes)
	}
}

func TestANonNumericGIDIsSkippedAndNamed(t *testing.T) {
	src := fixture(t, `oops:x:1000:group-name:Oops:/home/oops:/bin/bash
casey:x:1001:1001::/home/casey:/bin/bash
`, `UID_MIN 1000
UID_MAX 60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	wantNames(t, l, "casey")
	if !strings.Contains(joined(l.Notes), "gid") {
		t.Errorf("the gid problem was reported as something else: %v", l.Notes)
	}
}

// A directory opens cleanly and fails on the first read. That is the only way to
// reach the scanner's error path without inventing a filesystem fault, and it is a
// realistic one: /etc/passwd replaced by a directory is a mistake someone makes
// while mounting something.
func TestAPathThatOpensButCannotBeReadIsAnError(t *testing.T) {
	t.Run("passwd", func(t *testing.T) {
		src := fixture(t, "casey:x:1000:1000::/home/casey:/bin/bash\n", "UID_MIN 1000\n")
		src.PasswdPath = t.TempDir()
		_, err := List(src)
		if err == nil {
			t.Fatal("a directory read as a password file produced a listing")
		}
		if errors.Is(err, ErrNoAccounts) {
			t.Errorf("a read failure was reported as an empty machine: %v", err)
		}
	})
	t.Run("login.defs", func(t *testing.T) {
		src := fixture(t, "casey:x:1000:1000::/home/casey:/bin/bash\n", "UID_MIN 1000\n")
		src.LoginDefsPath = t.TempDir()
		_, err := List(src)
		if err == nil {
			t.Fatal("a directory read as login.defs produced a listing")
		}
		if errors.Is(err, ErrNoAccounts) {
			t.Errorf("a read failure was reported as an empty machine: %v", err)
		}
	})
}

// The default home check is what runs in production; every other test injects its
// own. A default that is never called is a default that can be wrong in ways nobody
// sees, which is the same silent-drop shape as a log sink nobody wired up.
func TestPathExistsAnswersForRealPaths(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "present")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !pathExists(dir) {
		t.Error("a directory that exists reported false")
	}
	if !pathExists(file) {
		t.Error("a file that exists reported false")
	}
	if pathExists(filepath.Join(dir, "absent")) {
		t.Error("a path that does not exist reported true")
	}
	// The zero Source has to land on this function rather than nil, or every account
	// is judged by a nil call and the process dies on the dashboard's first request.
	if _, _, homes := (Source{}).resolve(); homes == nil {
		t.Error("the zero Source resolved no home check")
	} else if homes(dir) != true {
		t.Error("the resolved home check is not the real one")
	}
}
