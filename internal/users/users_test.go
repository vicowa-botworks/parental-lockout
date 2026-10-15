package users

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixtures are written per test rather than shared, because most of what this
// package is about is what a file says when it is odd, and a shared fixture turns
// into a fixture nobody may change without breaking six tests.

func fixture(t *testing.T, passwd, defs string) Source {
	t.Helper()
	dir := t.TempDir()
	passwdPath := filepath.Join(dir, "passwd")
	defsPath := filepath.Join(dir, "login.defs")
	if err := os.WriteFile(passwdPath, []byte(passwd), 0o600); err != nil {
		t.Fatalf("write passwd: %v", err)
	}
	if err := os.WriteFile(defsPath, []byte(defs), 0o600); err != nil {
		t.Fatalf("write login.defs: %v", err)
	}
	return Source{
		PasswdPath:    passwdPath,
		LoginDefsPath: defsPath,
		// Every home in the fixtures exists unless a test says otherwise. The
		// home-absent path is its own test, because it is the one place the package
		// deliberately keeps rather than drops.
		HomeExists: func(string) bool { return true },
	}
}

func names(l *Listing) []string {
	var out []string
	for _, a := range l.Accounts {
		out = append(out, a.Username)
	}
	return out
}

func wantNames(t *testing.T, l *Listing, want ...string) {
	t.Helper()
	got := names(l)
	if len(got) != len(want) {
		t.Fatalf("accounts = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("accounts = %v, want %v", got, want)
		}
	}
}

func joined(notes []string) string { return strings.Join(notes, "\n") }

// The ordinary case, on a Debian-shaped machine: humans from 1000, system accounts
// below it, and the service accounts that live above the floor.
func TestListReturnsHumanAccountsAndDropsTheRest(t *testing.T) {
	src := fixture(t, `root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
messagebus:x:100:101::/nonexistent:/usr/sbin/nologin
sshd:x:105:65534::/run/sshd:/usr/sbin/nologin
casey:x:1000:1000:Casey:/home/casey:/bin/bash
quinn:x:1001:1001:Quinn:/home/quinn:/bin/sh
nobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin
`, `UID_MIN			 1000
UID_MAX			60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	wantNames(t, l, "casey", "quinn")
	if l.UIDMin != 1000 || l.UIDMax != 60000 {
		t.Errorf("bounds = %d..%d, want 1000..60000", l.UIDMin, l.UIDMax)
	}
	// Nothing was guessed here, so nothing may be murmured about it. A note list
	// that is always non-empty trains the operator to ignore it.
	if len(l.Notes) != 0 {
		t.Errorf("notes on a clean file: %v", l.Notes)
	}
}

// The fields the dashboard needs have to survive the parse, not just the name.
func TestListCarriesTheFieldsTheDashboardShows(t *testing.T) {
	src := fixture(t, `casey:x:1000:100:Casey:/home/casey:/bin/bash
`, `UID_MIN 1000
UID_MAX 60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	a := l.Accounts[0]
	if a.UID != 1000 || a.GID != 100 {
		t.Errorf("ids = %d/%d, want 1000/100", a.UID, a.GID)
	}
	if a.Home != "/home/casey" || a.Shell != "/bin/bash" {
		t.Errorf("home/shell = %q %q", a.Home, a.Shell)
	}
}

// An old Red Hat machine puts its first human at 500. A floor of 1000 — the modern
// default — would hide that child from the page, which is the one failure this
// package cannot afford, so the floor in the file wins and is reported.
func TestListHonoursALowUIDMin(t *testing.T) {
	src := fixture(t, `rinus:x:500:500:Rinus:/home/rinus:/bin/bash
`, `UID_MIN 500
UID_MAX 60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	wantNames(t, l, "rinus")
}

// The ceiling is a real filter, not decoration: a container or LDAP account parked
// above it is not a local child.
func TestListHonoursTheUIDMaxCeiling(t *testing.T) {
	src := fixture(t, `casey:x:1000:1000::/home/casey:/bin/bash
svc:x:700000:700000::/srv/svc:/bin/bash
`, `UID_MIN 1000
UID_MAX 60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	wantNames(t, l, "casey")
}

// A file with no UID_MIN is not a file the package may silently supply its own
// opinion about, because its opinion hides anyone below 1000.
func TestMissingUIDMinIsAssumedAndSaidOutLoud(t *testing.T) {
	src := fixture(t, `casey:x:1000:1000::/home/casey:/bin/bash
`, `# created by a provisioning tool that never set a range
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	wantNames(t, l, "casey")
	if l.UIDMin != DefaultUIDMin {
		t.Errorf("UIDMin = %d, want the documented default %d", l.UIDMin, DefaultUIDMin)
	}
	if !strings.Contains(joined(l.Notes), "no UID_MIN") {
		t.Errorf("the assumption was made silently: %v", l.Notes)
	}
	if !strings.Contains(joined(l.Notes), "hides any account below") {
		t.Errorf("the note does not say what the assumption costs: %v", l.Notes)
	}
}

// A corrupt value must not blank the screen. The cost of being strict here is a
// dashboard that lists nobody.
func TestNonNumericBoundsFallBackAndSaySo(t *testing.T) {
	src := fixture(t, `casey:x:1000:1000::/home/casey:/bin/bash
`, `UID_MIN one-thousand
UID_MAX 60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("a bad UID_MIN must not fail the listing: %v", err)
	}
	wantNames(t, l, "casey")
	if !strings.Contains(joined(l.Notes), "UID_MIN") {
		t.Errorf("the fallback was silent: %v", l.Notes)
	}
}

// "+1000" parses as a number to Go and means nothing to login.defs. Treating it as
// valid would apply a floor the maintainer did not write.
func TestAPlusPrefixedBoundIsNotAcceptedSilently(t *testing.T) {
	src := fixture(t, `casey:x:1000:1000::/home/casey:/bin/bash
`, `UID_MIN +1000
UID_MAX 60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !strings.Contains(joined(l.Notes), "not a number") {
		t.Errorf("+1000 was accepted without a note: %v", l.Notes)
	}
}

// An inverted range is a file that contradicts itself. Swapping the two would
// invent a policy the operator never wrote, so it is reported and the empty result
// is explained by the numbers themselves.
func TestInvertedRangeIsReportedNotRepaired(t *testing.T) {
	src := fixture(t, `casey:x:1000:1000::/home/casey:/bin/bash
`, `UID_MIN 60000
UID_MAX 1000
`)
	l, err := List(src)
	if !errors.Is(err, ErrNoAccounts) {
		t.Fatalf("err = %v, want ErrNoAccounts", err)
	}
	if l == nil {
		t.Fatal("the bounds must travel with the failure")
	}
	if l.UIDMin != 60000 || l.UIDMax != 1000 {
		t.Errorf("bounds were rewritten to %d..%d", l.UIDMin, l.UIDMax)
	}
	if !strings.Contains(joined(l.Notes), "no account can be in range") {
		t.Errorf("the contradiction was not reported: %v", l.Notes)
	}
}

// Every spelling of "no interactive login" has to be caught, and none of them by a
// substring match that could also catch a real shell.
func TestNoLoginShellsAreExcludedByExactPath(t *testing.T) {
	src := fixture(t, `a:x:1000:1000::/home/a:/bin/false
b:x:1001:1001::/home/b:/usr/bin/false
c:x:1002:1002::/home/c:/sbin/nologin
d:x:1003:1003::/home/d:/bin/nologin
e:x:1004:1004::/home/e:/usr/bin/nologin
f:x:1005:1005::/home/f:/usr/sbin/nologin
g:x:1006:1006::/home/g:
kept:x:1007:1007::/home/kept:/bin/my-nologin-compat-shell
`, `UID_MIN 1000
UID_MAX 60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// The last row is the point of the test: a real shell whose name happens to
	// contain "nologin" is a shell.
	wantNames(t, l, "kept")
}

// A home that is merely missing from disk is not a declaration of no home. Dropping
// it would remove a real child over a mount that has not happened yet.
func TestAMissingHomeIsKeptAndNoted(t *testing.T) {
	src := fixture(t, `casey:x:1000:1000::/home/casey:/bin/bash
`, `UID_MIN 1000
UID_MAX 60000
`)
	src.HomeExists = func(path string) bool { return false }
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	wantNames(t, l, "casey")
	if !strings.Contains(joined(l.Notes), "kept anyway") {
		t.Errorf("the account was kept in silence: %v", l.Notes)
	}
}

// The sentinel is what useradd writes when told there is no home. That is a
// statement about the account, so it does exclude.
func TestADeclaredNoHomeExcludesAndSaysWhichAccount(t *testing.T) {
	src := fixture(t, `ghost:x:1000:1000::/nonexistent:/bin/bash
casey:x:1001:1001::/home/casey:/bin/bash
`, `UID_MIN 1000
UID_MAX 60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	wantNames(t, l, "casey")
	if !strings.Contains(joined(l.Notes), "ghost") {
		t.Errorf("an exclusion was performed without naming who: %v", l.Notes)
	}
}

// A truncated password file is the event that makes accounts vanish. Skipping a
// record that cannot be parsed is forced; skipping it quietly is not.
func TestAMalformedRecordIsSkippedOutLoudAndTheRestSurvive(t *testing.T) {
	src := fixture(t, `broken:x:1000:1000
casey:x:1001:1001:Casey:/home/casey:/bin/bash
`, `UID_MIN 1000
UID_MAX 60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	wantNames(t, l, "casey")
	if !strings.Contains(joined(l.Notes), "line 1") {
		t.Errorf("the skipped record was not located: %v", l.Notes)
	}
}

// Extra trailing columns are harmless, but the parse is now relying on the first
// seven fields and the operator is entitled to know the file is not what it claims.
func TestExtraFieldsAreIgnoredAndMentioned(t *testing.T) {
	src := fixture(t, `casey:x:1000:1000:Casey:/home/casey:/bin/bash:extra
`, `UID_MIN 1000
UID_MAX 60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	wantNames(t, l, "casey")
	if !strings.Contains(joined(l.Notes), "extra ones were ignored") {
		t.Errorf("the extra column went unmentioned: %v", l.Notes)
	}
}

// Two records for one name means the file is inconsistent. Which one wins has to be
// stated, because the two could carry different homes.
func TestADuplicateNameKeepsTheFirstAndReports(t *testing.T) {
	src := fixture(t, `casey:x:1000:1000::/home/casey:/bin/bash
casey:x:1001:1001::/home/casey-two:/bin/bash
`, `UID_MIN 1000
UID_MAX 60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	wantNames(t, l, "casey")
	if l.Accounts[0].UID != 1000 {
		t.Errorf("the second entry won: uid %d", l.Accounts[0].UID)
	}
	if !strings.Contains(joined(l.Notes), "more than once") {
		t.Errorf("the duplicate was resolved silently: %v", l.Notes)
	}
}

// An empty login name cannot be shown or acted on, and it is not a filter working,
// so it is noted rather than dropped in silence.
func TestAnEmptyLoginNameIsNoted(t *testing.T) {
	src := fixture(t, `:x:1000:1000::/home/nobody:/bin/bash
casey:x:1001:1001::/home/casey:/bin/bash
`, `UID_MIN 1000
UID_MAX 60000
`)
	l, err := List(src)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	wantNames(t, l, "casey")
	if !strings.Contains(joined(l.Notes), "empty login name") {
		t.Errorf("the record vanished without explanation: %v", l.Notes)
	}
}

// "Nothing looked human" and "there is nobody here" are different facts. The first
// is a configuration problem and has to be distinguishable.
func TestAnEmptyResultIsADistinctErrorCarryingTheBounds(t *testing.T) {
	src := fixture(t, `root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
`, `UID_MIN 1000
UID_MAX 60000
`)
	l, err := List(src)
	if !errors.Is(err, ErrNoAccounts) {
		t.Fatalf("err = %v, want ErrNoAccounts", err)
	}
	if l == nil || l.UIDMin != 1000 {
		t.Errorf("the bounds did not travel with the error: %+v", l)
	}
}

// A missing login.defs is not "no bounds configured" — it is a machine this package
// cannot read, and an empty list would be indistinguishable from an empty machine.
func TestAnUnreadableLoginDefsIsAnErrorNotAnEmptyList(t *testing.T) {
	src := fixture(t, "casey:x:1000:1000::/home/casey:/bin/bash\n", "UID_MIN 1000\n")
	src.LoginDefsPath = filepath.Join(t.TempDir(), "absent")
	_, err := List(src)
	if err == nil {
		t.Fatal("an unreadable login.defs produced a listing")
	}
	if errors.Is(err, ErrNoAccounts) {
		t.Errorf("a read failure was reported as an empty machine: %v", err)
	}
	if !strings.Contains(err.Error(), "absent") {
		t.Errorf("the error does not name the file that could not be read: %v", err)
	}
}

func TestAnUnreadablePasswdIsAnErrorNamingTheFile(t *testing.T) {
	src := fixture(t, "casey:x:1000:1000::/home/casey:/bin/bash\n", "UID_MIN 1000\n")
	src.PasswdPath = filepath.Join(t.TempDir(), "gone")
	_, err := List(src)
	if err == nil {
		t.Fatal("an unreadable passwd produced a listing")
	}
	if errors.Is(err, ErrNoAccounts) {
		t.Errorf("a read failure was reported as an empty machine: %v", err)
	}
	if !strings.Contains(err.Error(), "gone") {
		t.Errorf("the error does not name the file: %v", err)
	}
}

// The zero Source is the real machine, so the defaults have to be the real paths: a
// typo here means the dashboard opens a file that does not exist and reports an
// empty machine.
//
// This asserts the constants and deliberately does NOT call List(Source{}) to see
// whether "the machine answers". Whether the host has an account at or above
// UID_MIN with a login shell and an existing home is a fact about whoever built the
// image, and a test that depends on it fails for reasons that have nothing to do
// with this package — the same class of mistake as a fixture that only passed
// because the author's own uid happened to be the one written into it. The real
// paths are exercised by `plock doctor`, which is allowed to report a host fact
// because a human reads the answer.
func TestZeroSourceMeansTheRealMachine(t *testing.T) {
	if DefaultPasswdPath != "/etc/passwd" {
		t.Errorf("DefaultPasswdPath = %q", DefaultPasswdPath)
	}
	if DefaultLoginDefsPath != "/etc/login.defs" {
		t.Errorf("DefaultLoginDefsPath = %q", DefaultLoginDefsPath)
	}
}

// The zero Source has to resolve to those constants rather than to the empty
// string, which would open a file named "" and fail with an error that points at
// nothing the operator can check.
// An unset field must become the real path, not stay "". A blank path opens a file
// named "" and fails with an error pointing at nothing the operator can go and look
// at, and the zero Source is what a caller gets when the web layer has not been
// configured, so this is the common path rather than an edge one.
func TestZeroSourceResolvesToTheDefaultPaths(t *testing.T) {
	passwd, defs, homes := Source{}.resolve()
	if passwd != DefaultPasswdPath {
		t.Errorf("passwd = %q, want %q", passwd, DefaultPasswdPath)
	}
	if defs != DefaultLoginDefsPath {
		t.Errorf("defs = %q, want %q", defs, DefaultLoginDefsPath)
	}
	if homes == nil {
		t.Fatal("no home check was resolved; every account would be judged by a nil func")
	}

	// And a caller that set one field keeps it while the others default.
	passwd, _, _ = Source{PasswdPath: "/tmp/fixture-passwd"}.resolve()
	if passwd != "/tmp/fixture-passwd" {
		t.Errorf("an explicit path was overwritten with the default: %q", passwd)
	}
}
