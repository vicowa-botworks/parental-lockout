package users

import (
	"strings"
	"testing"
)

// bounds_test.go covers the three ways the UID range can be gotten quietly wrong:
// a key spelled the way a human types it, a ceiling nobody set, and a home check
// that stopped checking. All three fail toward hiding an account, which is the
// direction this package is not allowed to fail in.

// login.defs keys are conventionally upper case, but the file is plain text a person
// edits, and a lower-case uid_min is not a syntax error anywhere -- it just stops
// matching. Silently falling back to the built-in floor is the fail-open direction:
// accounts below it vanish from the listing and nothing says so.
func TestALowerCaseKeyIsStillTheSameKey(t *testing.T) {
	src := fixture(t, `casey:x:1500:1000::/home/casey:/bin/bash
`, `# someone typed this in lower case
uid_min 1200
uid_max 1800
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if l.UIDMin != 1200 || l.UIDMax != 1800 {
		t.Errorf("bounds = %d..%d, want 1200..1800 -- the keys were matched case-sensitively and the defaults used instead",
			l.UIDMin, l.UIDMax)
	}
}

// The floor gets a loud note when it is missing because it hides accounts. The ceiling
// gets one for the same reason in the other direction: a file that stops at UID_MIN
// leaves the built-in ceiling in force, and an account above it is invisible.
func TestAnAbsentCeilingIsAnnounced(t *testing.T) {
	src := fixture(t, `casey:x:1000:1000::/home/casey:/bin/bash
`, "UID_MIN 1000\n")
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if l.UIDMax != DefaultUIDMax {
		t.Fatalf("UIDMax = %d, want the default %d", l.UIDMax, DefaultUIDMax)
	}
	if !strings.Contains(joined(l.Notes), "sets no UID_MAX") {
		t.Errorf("the assumed ceiling passed for the file's own value: %v", l.Notes)
	}
}

// The mirror half: a file that does set UID_MAX must not complain about it, or the note
// stops meaning anything.
func TestACeilingThatIsSetIsNotAnnounced(t *testing.T) {
	src := fixture(t, `casey:x:1000:1000::/home/casey:/bin/bash
`, "UID_MIN 1000\nUID_MAX 2000\n")
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if strings.Contains(joined(l.Notes), "sets no UID_MAX") {
		t.Errorf("a file that sets UID_MAX was reported as not setting it: %v", l.Notes)
	}
}

// The zero Source has to arrive at a home check that actually looks. The existing test
// only proves the func is not nil, and a func that always says yes is not nil -- it is
// worse than nil, because nil would crash and this quietly keeps every account whose
// home directory was deleted along with the account itself.
func TestTheDefaultHomeCheckActuallyLooks(t *testing.T) {
	dir := t.TempDir()
	_, _, homes := Source{}.resolve()
	if homes == nil {
		t.Fatal("no home check was resolved")
	}
	if !homes(dir) {
		t.Error("the default home check says a directory that exists does not")
	}
	if homes(dir + "/never-created") {
		t.Error("the default home check says every path exists, so no phantom home is ever pruned")
	}
}
