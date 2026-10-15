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
//
// The empty shell field is in this test as the case that is NOT excluded, because it was
// excluded once. passwd(5) says an empty shell field "defaults to the value /bin/sh", so
// `g` is a person with a shell; treating the blank as a second spelling of nologin read
// the format backwards and dropped them with no note at all.
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
	// Two things are being pinned here. A real shell whose name happens to contain
	// "nologin" is a shell, and an account that wrote no shell down is a person.
	wantNames(t, l, "g", "kept")
	for _, a := range l.Accounts {
		if a.Username == "kept" {
			continue
		}
		if a.Shell != "" {
			t.Errorf("%s was kept with shell %q; only an empty shell is a default, a real "+
				"nologin path means the account cannot hold a session", a.Username, a.Shell)
		}
	}
	// The inference is a judgement, and every judgement this package makes is recorded.
	var noted bool
	for _, n := range l.Notes {
		if strings.Contains(n, "g (uid 1006)") && strings.Contains(n, defaultShell) {
			noted = true
		}
	}
	if !noted {
		t.Errorf("the account with no shell was kept silently; notes = %v, want one naming %s as "+
			"the shell that gets used", l.Notes, defaultShell)
	}
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

// login.defs(5): "Numbers (both regular and long) may be either decimal values, octal
// values (precede the value with 0) or hexadecimal values (precede the value with 0x)."
//
// Reading `UID_MIN 01750` with base 10 gives 1750, not the 1000 the maintainer wrote.
// The damage is silent and it is the bad direction: every child between 1000 and 1749
// vanishes from the only screen where a parent could add a rule for them, and nothing is
// noted, because the value parsed perfectly well.
func TestLoginDefsAcceptsTheBasesItDocuments(t *testing.T) {
	const passwd = `kidA:x:1000:1000::/home/kidA:/bin/bash
kidB:x:1200:1200::/home/kidB:/bin/bash
kidC:x:1800:1800::/home/kidC:/bin/bash
`
	for _, tc := range []struct {
		name, defs string
		lo         uint32
		want       []string
	}{
		{"decimal", "UID_MIN 1000\nUID_MAX 60000\n", 1000, []string{"kidA", "kidB", "kidC"}},
		{"octal", "UID_MIN 01750\nUID_MAX 60000\n", 1000, []string{"kidA", "kidB", "kidC"}},
		{"hex", "UID_MIN 0x3e8\nUID_MAX 60000\n", 1000, []string{"kidA", "kidB", "kidC"}},
		// 02700 is 1472, which really is above kidA and kidB, so they are correctly out.
		{"octal that really does exclude", "UID_MIN 02700\nUID_MAX 60000\n", 1472, []string{"kidC"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, err := List(fixture(t, passwd, tc.defs))
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if l.UIDMin != tc.lo {
				t.Errorf("UID_MIN = %d, want %d", l.UIDMin, tc.lo)
			}
			wantNames(t, l, tc.want...)
		})
	}
}

// A sign is still not a number, in any base. This is the guard against a future edit
// swapping in ParseInt or strconv.Atoi on the way to "support more bases".
func TestASignIsStillRejectedInEveryBase(t *testing.T) {
	// No whitespace case: strings.Fields splits on whitespace before the value is ever
	// seen, so "UID_MIN  1000" arrives as "1000" and that is correct, not a hole.
	for _, v := range []string{"+1000", "-1000", "1e3", "1_000", "0b111", "08"} {
		t.Run(v, func(t *testing.T) {
			l, err := List(fixture(t, `casey:x:1000:1000::/home/casey:/bin/bash
`, "UID_MIN "+v+"\nUID_MAX 60000\n"))
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if l.UIDMin != DefaultUIDMin {
				t.Errorf("UID_MIN %q was taken as %d; it is not in the grammar login.defs documents", v, l.UIDMin)
			}
			if !strings.Contains(joined(l.Notes), "not a number") {
				t.Errorf("UID_MIN %q was rejected without saying so: %v", v, l.Notes)
			}
		})
	}
}

// Two sentences the file did not say, both produced by one value.
//
// A UID_MIN that is present but unusable used to emit "is not a number" AND "sets no
// UID_MIN". The second is false -- the key is sitting right there -- and it sends an
// operator to grep for something they can already see. And a value that is a perfectly
// good number which merely does not fit was reported as a typo in the digits.
func TestAKeyThatIsPresentButUnusableSaysSoOnce(t *testing.T) {
	for _, tc := range []struct{ name, value, phrase string }{
		{"too large for the field", "4294967296", "too large"},
		{"not a number at all", "1000k", "not a number"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, err := List(fixture(t, `casey:x:1000:1000::/home/casey:/bin/bash
`, "UID_MIN "+tc.value+"\nUID_MAX 60000\n"))
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			joined := joined(l.Notes)
			if !strings.Contains(joined, tc.phrase) {
				t.Errorf("notes = %v, want one saying %q", l.Notes, tc.phrase)
			}
			if strings.Contains(joined, "sets no UID_MIN") {
				t.Errorf("the file does set UID_MIN; notes = %v must not claim otherwise", l.Notes)
			}
			// Exactly one note about this key: the per-line failure already names the
			// fallback, so a second "assuming 1000" line is noise that hides the cause.
			if n := strings.Count(joined, "UID_MIN"); n != 1 {
				t.Errorf("UID_MIN is mentioned %d time(s) in %v, want the failure said once", n, l.Notes)
			}
		})
	}
}

// The "sets no UID_MIN" note is still owed when the key genuinely is absent -- that is
// the case it was written for, and DefaultUIDMin of 1000 hides the first human on an old
// Red Hat box where humans start at 500.
func TestAFileWithNoUIDMinAtAllStillSaysSo(t *testing.T) {
	l, err := List(fixture(t, `casey:x:1000:1000::/home/casey:/bin/bash
`, `UID_MAX 60000
`))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !strings.Contains(joined(l.Notes), "sets no UID_MIN") {
		t.Errorf("a file with no UID_MIN produced %v", l.Notes)
	}
}

// A config that spells a name differently from /etc/passwd used to split into two rows
// with nothing to connect them: `Casey` stale (an account that no longer exists, so
// nothing can be locked out) and `casey` unmanaged. One kid, a ghost and a gap, and no
// note anywhere -- config's validation only rejects empty and byte-identical duplicates,
// so nothing upstream stops the mismatch either.
//
// The rows stay separate. On Linux `casey` and `Casey` really are two accounts, and
// joining them on a folded key would hand one child another child's profile, which is
// worse than a confusing table. What changed is that the split is now said.
func TestNamesDifferingOnlyByCaseAreSaidToCollide(t *testing.T) {
	rows := Union(
		[]Account{{Username: "casey", UID: 1000, GID: 1000, Home: "/home/casey", Shell: "/bin/bash"}},
		[]ConfiguredUser{{Username: "Casey", ProfileID: "school nights", Enabled: true}},
	)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2; folding them together would give one child the other's profile", len(rows))
	}
	for _, r := range rows {
		if len(r.Problems) == 0 {
			t.Errorf("%q carries no problem note; a parent sees a ghost account and an unmanaged "+
				"child for the same kid", r.Username)
			continue
		}
		if !strings.Contains(strings.Join(r.Problems, " "), "case") {
			t.Errorf("%q's problems = %v, want one naming a case-only difference", r.Username, r.Problems)
		}
	}
}

// Two machine accounts that differ only by case are also worth saying, and two names
// that genuinely differ must not be flagged.
func TestACaseCollisionIsNotInventedWhereThereIsNone(t *testing.T) {
	rows := Union([]Account{
		{Username: "ada", UID: 1000, GID: 1000, Home: "/home/ada", Shell: "/bin/bash"},
		{Username: "casey", UID: 1001, GID: 1001, Home: "/home/casey", Shell: "/bin/bash"},
	}, nil)
	for _, r := range rows {
		if len(r.Problems) != 0 {
			t.Errorf("%q was flagged as colliding: %v", r.Username, r.Problems)
		}
	}
}
