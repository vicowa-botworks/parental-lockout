package platform

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The parsers are pinned directly rather than only through Sessions, because each
// branch here decides whether a session ends up on the lock list, and a branch that
// no end-to-end test reaches is a branch that can be wrong in silence.

func TestParseProps(t *testing.T) {
	got := parseProps([]byte("Name=kid\n\n  \nRemote=no\nType=x11\nnolineshere\nState=active\n"))
	if len(got) != 4 {
		t.Errorf("parsed %d properties (%v), want 4 — blank lines and a line without = must be skipped", len(got), map[string]string(got))
	}
	if got.get("Name") != "kid" || got.get("State") != "active" {
		t.Errorf("parsed = %v", map[string]string(got))
	}
	// A property whose value contains "=" keeps everything after the first one.
	got = parseProps([]byte("Name=a=b\n"))
	if got.get("Name") != "a=b" {
		t.Errorf("value with = read as %q, want a=b", got.get("Name"))
	}
	// Absent is the empty string, not a panic and not an error.
	if got.get("LockedHint") != "" || got.flag("LockedHint") {
		t.Errorf("absent property read as %q/%v", got.get("LockedHint"), got.flag("LockedHint"))
	}
}

func TestPropsFlag(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"yes", true}, {"no", false}, {"true", true}, {"false", false},
		{"1", true}, {"0", false}, {"", false}, {"garbage", false},
		// logind prints lowercase yes/no; anything else is tolerated case-insensitively
		// rather than read as a truth.
		{"YES", true}, {"No", false},
	} {
		if got := (props{"k": tc.in}).flag("k"); got != tc.want {
			t.Errorf("flag(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestIsUnknownSession(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("no session '9' known"), true},
		{errors.New("Failed to get path for session '9': No session '9' known"), true},
		// Anything else is a real failure and must not be mistaken for a logout.
		{errors.New("connection reset by peer"), false},
		{errors.New("no session"), false}, // truncated: not the full phrase
		{errors.New("not known"), false},  // ditto
		{errors.New("permission denied"), false},
	} {
		if got := isUnknownSession(tc.err); got != tc.want {
			t.Errorf("isUnknownSession(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestValidSessionID(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{"2", true}, {"c1", true}, {"13", true}, {"session-3", true},
		{"a_b.c-d", true},
		{"", false},
		{"--help", false}, {"-p", false}, // would be read as an option
		{"2/tty", false}, {"a b", false}, {"a\tb", false},
		{"a;b", false}, {"a|b", false}, {"$(x)", false},
		{strings.Repeat("x", 65), false},
		{strings.Repeat("x", 64), true},
	} {
		if got := validSessionID(tc.id); got != tc.want {
			t.Errorf("validSessionID(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestTableStrAndField(t *testing.T) {
	if got := tableStr("-"); got != "" {
		t.Errorf(`tableStr("-") = %q, want ""`, got)
	}
	if got := tableStr("seat0"); got != "seat0" {
		t.Errorf(`tableStr("seat0") = %q`, got)
	}
	// A literal "-" is not a legal seat name, so mapping it to absent is safe.
	if f := tableField("-"); f != nil {
		t.Errorf(`tableField("-") = %q, want nil`, *f)
	}
	if f := tableField(""); f != nil {
		t.Errorf(`tableField("") = %q, want nil`, *f)
	}
	f := tableField("seat0")
	if f == nil || *f != "seat0" {
		t.Errorf(`tableField("seat0") = %v, want &seat0`, f)
	}
}

func TestFirstLine(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"one\ntwo\n", "one"},
		{"  only  ", "only"},
		{"", "(empty output)"},
		{"   \n\n", "(empty output)"},
		{strings.Repeat("x", 200), strings.Repeat("x", 120) + "..."},
	} {
		if got := firstLine([]byte(tc.in)); got != tc.want {
			t.Errorf("firstLine(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseSessionTableRejectsWhatItCannotRead(t *testing.T) {
	real := fixture(t, "list-sessions.table")
	entries, err := parseSessionTable(real)
	if err != nil {
		t.Fatalf("the captured table did not parse: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("parsed %d rows, want 2", len(entries))
	}
	if entries[0].Session != "3" || entries[0].UID != 1000 || entries[0].User != "rodney" ||
		entries[0].Class != "user" || entries[0].Seat == nil || *entries[0].Seat != "seat0" {
		t.Errorf("row 0 = %+v", entries[0])
	}
	if entries[1].Seat != nil {
		t.Errorf(`manager row's seat = %q, want nil — "-" and null must arrive identically`, *entries[1].Seat)
	}
	if entries[1].Class != "manager" {
		t.Errorf("manager row class = %q", entries[1].Class)
	}

	// No header at all: refuse rather than guess columns.
	if _, err := parseSessionTable([]byte("garbage\nmore garbage\n")); err == nil {
		t.Error("a table with no header parsed successfully")
	}
	// Header but no rows is NOT in this test: that is the healthy idle state and it
	// parses to zero sessions. See TestParseSessionTableEmptyTableIsZeroSessions.
	// A row whose LEADER is not a number is not a session row.
	if _, err := parseSessionTable([]byte("SESSION UID USER SEAT LEADER CLASS TTY IDLE SINCE\nx y z - q user - no -\n")); err == nil {
		t.Error("a row with a non-numeric leader parsed successfully")
	}
}

// Nobody being logged in is the state of every machine at 3am, and it has to read as
// zero sessions rather than as a broken backend. Making it an error would have the
// enforcement loop log a failure twice a minute until nobody reads them, and would
// have `plock doctor` tell an operator that logind is broken when logind is fine.
//
// All three shapes loginctl can produce are checked, because they do not fail alike:
// the bare header, the count trailer, and the "No sessions." legend that upstream
// actually prints for an empty table (loginctl.c: printf("No %s.\n", type)). Only the
// second was recognised before this existed, and the third reached the row parser and
// failed on its UID column.
func TestParseSessionTableEmptyTableIsZeroSessions(t *testing.T) {
	header := "SESSION  UID USER SEAT LEADER CLASS TTY IDLE SINCE"
	for _, tc := range []struct{ name, in string }{
		{"header only", header + "\n"},
		{"count trailer", header + "\n0 sessions listed.\n"},
		{"upstream empty legend", header + "\nNo sessions.\n"},
		{"blank line then trailer", header + "\n\n2 sessions listed.\n"},
		{"blank line then legend", header + "\n\nNo sessions.\n"},
	} {
		entries, err := parseSessionTable([]byte(tc.in))
		if err != nil {
			t.Errorf("%s: %v — an idle machine must not read as a broken backend", tc.name, err)
			continue
		}
		if len(entries) != 0 {
			t.Errorf("%s: parsed %d rows, want 0", tc.name, len(entries))
		}
	}

	// The legend skip must not become a way for a real row to vanish. A session row
	// carries a value for every column, so it can never be two words wide; anything
	// two words wide that is not the legend is refused, not skipped.
	if _, err := parseSessionTable([]byte(header + "\nNope.\n")); err == nil {
		t.Error("a two-word line that is not the legend was skipped instead of refused")
	}
	// "No sessions." is the legend; a line that merely starts with No is not.
	if _, err := parseSessionTable([]byte(header + "\nNo sessions\n")); err == nil {
		t.Error("a line without the trailing period was skipped instead of refused")
	}
	// The word has to be a lowercase noun. "No Sessions." is not something loginctl
	// prints, and accepting it would widen the skip for no benefit — every character
	// accepted here is a line that stops being checked.
	if _, err := parseSessionTable([]byte(header + "\nNo Sessions.\n")); err == nil {
		t.Error("a capitalised two-word line was skipped instead of refused")
	}
	// "No ." has an empty noun, which is not a legend either.
	if _, err := parseSessionTable([]byte(header + "\nNo .\n")); err == nil {
		t.Error("an empty legend noun was skipped instead of refused")
	}
	// Other listing types use the same printf, and a hyphenated noun is still a noun.
	for _, legend := range []string{"No seats.", "No users.", "No user-sessions."} {
		entries, err := parseSessionTable([]byte(header + "\n" + legend + "\n"))
		if err != nil || len(entries) != 0 {
			t.Errorf("%q: got %d rows, err %v — want the empty-table legend", legend, len(entries), err)
		}
	}
}

func TestParseSessionJSONRejectsATable(t *testing.T) {
	if _, err := parseSessionJSON(fixture(t, "list-sessions.table")); err == nil {
		t.Error("the table parsed as JSON")
	}
	if _, err := parseSessionJSON([]byte("")); err == nil {
		t.Error("empty input parsed as JSON")
	}
	// An empty array is a legitimate answer, not a parse failure: a machine with no
	// sessions logged in must not fall through to the table parser.
	entries, err := parseSessionJSON([]byte("[]"))
	if err != nil {
		t.Errorf("an empty array failed to parse: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("empty array gave %d entries", len(entries))
	}
}

// The listing's username is the value that cannot be missing, so it fills in when
// show-session does not report Name.
func TestSessionsFallsBackToTheListingForNameAndUID(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if sub(argv, 0) == "list-sessions" {
			return []byte(`[{"session":"9","uid":1002,"user":"kid","seat":"seat0","leader":5,"class":"user","tty":null,"idle":false,"since":null}]`), nil
		}
		// No Name, no UID.
		return []byte("Type=x11\nState=active\nRemote=no\n"), nil
	}}
	got, err := newLogind(t, r).Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %v, want one session", got)
	}
	if got[0].Username != "kid" {
		t.Errorf("Username = %q, want the listing's kid", got[0].Username)
	}
	if got[0].UID != 1002 {
		t.Errorf("UID = %d, want the listing's 1002", got[0].UID)
	}
}

// A row with no id is not a session; it must not become a session with an empty id
// that later reaches lock-session.
func TestSessionsSkipsARowWithNoID(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if sub(argv, 0) == "list-sessions" {
			return []byte(`[{"session":"","uid":1002,"user":"kid","seat":"seat0","leader":5,"class":"user","tty":null,"idle":false,"since":null}]`), nil
		}
		t.Errorf("queried a session that had no id: %v", argv)
		return nil, errors.New("unexpected")
	}}
	got, err := newLogind(t, r).Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want nothing", got)
	}
}

// A session id that could not have come from logind is refused before the query, so
// a hostile or corrupt /run cannot get an option into show-session.
func TestShowRefusesAnImplausibleID(t *testing.T) {
	l := &logind{r: &fakeRunner{}}
	if _, err := l.show(context.Background(), "--help"); err == nil {
		t.Error("show-session accepted an id that would be read as a flag")
	}
}

// Both listing attempts failing has to report the first error, which is the one that
// means something.
func TestListReportsTheFirstFailureWhenTheFallbackAlsoFails(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if hasFlag(argv, "--json=short") {
			return nil, errors.New("loginctl: not found in PATH")
		}
		return nil, errors.New("loginctl: not found in PATH")
	}}
	_, err := (&logind{r: r}).list(context.Background())
	if err == nil {
		t.Fatal("list succeeded with loginctl missing")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q does not carry the original failure", err)
	}
}

func TestLogindConstructorRefusesANilRunner(t *testing.T) {
	if _, err := New(configSettings("logind"), nil); err == nil {
		t.Fatal("New built a logind backend with no runner; every command would panic")
	}
}

func TestLockWrapsAFailureWithTheBackendName(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		return nil, errors.New("Interactive authentication required")
	}}
	err := newLogind(t, r).Lock(context.Background(), desktopSession("3"))
	if err == nil {
		t.Fatal("Lock succeeded against a failing loginctl")
	}
	for _, want := range []string{"logind", "Interactive authentication required"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q", err, want)
		}
	}
}

func TestSessionDescribe(t *testing.T) {
	for _, tc := range []struct {
		s    Session
		want string
	}{
		{Session{ID: "3", Username: "kid", Type: "x11"}, "kid:3(x11)"},
		{Session{ID: "3", Username: "kid", Type: "x11", Locked: true}, "kid:3(x11) locked"},
		{Session{ID: "3", Username: "kid", Type: "x11", Remote: true}, "kid:3(x11) remote"},
		{Session{ID: "3", Username: "kid", Type: "x11", Remote: true, Locked: true}, "kid:3(x11) remote locked"},
	} {
		if got := tc.s.Describe(); got != tc.want {
			t.Errorf("Describe() = %q, want %q", got, tc.want)
		}
	}
}

func TestChainName(t *testing.T) {
	c := &Chain{backends: []Platform{&Mock{BackendName: "logind"}, &Mock{BackendName: "kde"}}}
	if got := c.Name(); got != "chain(logind kde)" {
		t.Errorf("Name() = %q, want chain(logind kde)", got)
	}
	empty := &Chain{}
	if got := empty.Name(); got != "chain()" {
		t.Errorf("empty chain Name() = %q", got)
	}
	if got := empty.Name(); !strings.HasPrefix(got, "chain(") {
		t.Errorf("Name() = %q, want it to identify the type", got)
	}
}

// A command that fails without saying why still has to produce a usable line — the
// fallback is the exec error's own text, and it is the only thing that will reach the
// log.
func TestExecRunnerFallsBackToTheExitError(t *testing.T) {
	_, err := ExecRunner{}.Run(context.Background(), []string{"/bin/false"}, nil, 0)
	if err == nil {
		t.Fatal("/bin/false succeeded")
	}
	if strings.Contains(err.Error(), "%!") || !strings.Contains(err.Error(), ": ") {
		t.Errorf("error %q is not a readable sentence", err)
	}
	if !strings.Contains(err.Error(), "false") {
		t.Errorf("error %q does not name the program", err)
	}
}

// The listing is the only place the uid comes from.
//
// This test used to assert the opposite: it scripted a `UID=1005` line back from
// show-session and pinned Sessions reading it. That line does not exist.
// org.freedesktop.login1.Session has no UID property — the uid is on the User object —
// and logind answers an unknown property with exit 0 and no line at all, so the
// fallback could never fire against a real loginctl while still passing here. Verified
// live:
//
//	$ loginctl show-session 3 -p Name -p Type -p State -p Remote -p UID -p LockedHint
//	Name=rodney / Remote=no / Type=x11 / State=active / LockedHint=yes
//
// So the property is not asked for, and what is pinned instead is (a) that a session
// whose listing carries uid 0 is still reported, because profiles are keyed by the
// username and the username cannot be missing, and (b) that no show-session call asks
// for UID — which is the half that would silently come back if someone "fixed" this by
// re-adding the property.
func TestSessionsTakesTheUIDFromTheListingOnly(t *testing.T) {
	var showArgvs [][]string
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if sub(argv, 0) == "list-sessions" {
			return []byte(`[{"session":"9","uid":0,"user":"kid","seat":"seat0","leader":5,"class":"user","tty":null,"idle":false,"since":null}]`), nil
		}
		showArgvs = append(showArgvs, argv)
		// Exactly what a real loginctl returns for this set: no UID line, and no
		// error about the one that was not asked for.
		return []byte("Name=kid\nType=x11\nState=active\nRemote=no\nLockedHint=no\n"), nil
	}}
	got, err := newLogind(t, r).Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %v, want the unresolved-uid session reported anyway", got)
	}
	if got[0].UID != 0 {
		t.Errorf("uid = %d, want 0: nothing may invent one the listing did not carry", got[0].UID)
	}
	if got[0].Username != "kid" {
		t.Errorf("username = %q, want kid — this is the value profiles are keyed by", got[0].Username)
	}
	if len(showArgvs) == 0 {
		t.Fatal("show-session was never called, so the property check below proves nothing")
	}
	for _, argv := range showArgvs {
		for _, a := range argv {
			if a == "UID" || a == "-pUID" {
				t.Errorf("show-session asks for UID, a property loginctl does not have: %v", argv)
			}
		}
	}
}

// The backend's own guard, reached when it is used without the chain. It is not
// redundant: nothing stops a caller holding a Platform directly.
func TestLogindLockRefusesBeforeAnythingElse(t *testing.T) {
	r := &fakeRunner{}
	l := &logind{r: r}
	for _, s := range []Session{
		{ID: "3", Type: "x11", State: "active", Remote: true},
		{ID: "", Type: "x11", State: "active"},
	} {
		if err := l.Lock(context.Background(), s); err == nil {
			t.Errorf("Lock(%+v) succeeded", s)
		}
	}
	if n := len(r.Calls()); n != 0 {
		t.Errorf("%d command(s) ran, want none: %v", n, r.Argvs())
	}
}

// The flag was refused, the fallback ran, and its output is unreadable. Both failures
// belong in the message: the first explains why a second command was tried at all.
func TestListReportsBothFailuresWhenTheFallbackIsUnreadable(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if hasFlag(argv, "--json=short") {
			return nil, errors.New("Unknown option --json=short")
		}
		return []byte("total nonsense\n"), nil
	}}
	_, err := (&logind{r: r}).list(context.Background())
	if err == nil {
		t.Fatal("unreadable output parsed successfully")
	}
	for _, want := range []string{"Unknown option", "no column header"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q", err, want)
		}
	}
}

// A table row whose SESSION column is "-" is not a session and must not become one
// with an empty id.
func TestParseSessionTableSkipsARowWithNoID(t *testing.T) {
	in := []byte("SESSION  UID USER   SEAT  LEADER CLASS TTY  IDLE SINCE\n- 1000 rodney seat0 6654 user tty2 no -\n")
	if _, err := parseSessionTable(in); err == nil {
		t.Error("a row with no session id parsed into a session")
	}
}

// The duplicate-id panic is the other half of Register's contract. The registry is
// package state, so the test swaps it out and puts it back rather than leaving a
// test-only backend name behind for every other test to trip over.
func TestRegisterRejectsADuplicateID(t *testing.T) {
	saved := registry
	defer func() { registry = saved }()
	registry = map[string]Constructor{}
	ctor := func(r Runner) (Platform, error) { return &Mock{}, nil }
	Register("temp", ctor)

	defer func() {
		if recover() == nil {
			t.Error("registering the same id twice did not panic")
		}
	}()
	Register("temp", ctor)
}

// The header used to be checked and then ignored: the parser verified it began
// "SESSION UID" and indexed fields 0..8 regardless of what came after. A loginctl that
// inserted one column anywhere past UID would have parsed cleanly with every field from
// USER onward shifted by one — a session whose Class read as its TTY and whose username
// read as its seat — and nothing would have failed. Checking the header cannot prevent
// that, because the header is exactly what changed.
func TestParseSessionTableMapsColumnsByName(t *testing.T) {
	const header = "SESSION  UID USER   SEAT  TTY  CLASS LEADER IDLE DISTANCE SINCE"
	const row = "2      1000 rodney seat0 tty2 user   18344 no   7        Thu 2026-10-01"
	entries, err := parseSessionTable([]byte(header + "\n" + row + "\n1 session listed.\n"))
	if err != nil {
		t.Fatalf("parseSessionTable: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	// DISTANCE is a column this build has never heard of. It must occupy a slot so the
	// columns after it still line up, without being read as anything.
	if e.Session != "2" || e.UID != 1000 || e.User != "rodney" || e.Seat == nil || *e.Seat != "seat0" {
		t.Errorf("identity fields parsed wrong through an unknown column: %+v", e)
	}
	if e.TTY == nil || *e.TTY != "tty2" || e.Class != "user" || e.Leader != 18344 {
		t.Errorf("fields after the unknown column shifted: %+v", e)
	}
	// SINCE is multi-word ("Thu 2026-10-01 15:47:23 UTC") so only its first token is
	// captured. What matters here is that the column after the unknown one was read at
	// all, and read as SINCE rather than as something else.
	if e.Since == nil || *e.Since != "Thu" {
		t.Errorf("SINCE after the unknown column = %v, want its first token to have survived", e.Since)
	}
}

// Reordering is the other half: position must not matter at all. SINCE stays last
// here -- not because position matters for it, but because its value contains spaces
// and TestParseSessionTableRefusesSINCEBeforeTheLastColumn pins why that layout is
// refused rather than reordered.
func TestParseSessionTableToleratesReorderedColumns(t *testing.T) {
	in := "CLASS UID SESSION IDLE TTY SEAT USER LEADER SINCE\n" +
		"user 1000 2 no tty2 seat0 rodney 18344 -\n"
	entries, err := parseSessionTable([]byte(in))
	if err != nil {
		t.Fatalf("parseSessionTable: %v", err)
	}
	e := entries[0]
	if e.Session != "2" || e.UID != 1000 || e.User != "rodney" || e.Class != "user" || e.Leader != 18344 {
		t.Errorf("reordered header parsed wrong: %+v", e)
	}
	if e.Idle {
		t.Error("IDLE=no read as idle")
	}
}

// SINCE is the one column whose value contains a space, so strings.Fields splits it
// into two. With SINCE last the extra words fall off the end harmlessly. Anywhere
// else every column after it is shifted, and -- the part that makes this worth a
// guard -- each shifted value is individually valid, so the row parses cleanly and
// nothing downstream can see that CLASS now reads "no" and SEAT reads "user".
//
// This is the silent column misread the whole name-mapping design exists to prevent,
// and it is invisible to every check after it because the misread values are
// plausible. Refusing the layout is the only defence that does not require guessing
// how many words a timestamp happened to have.
func TestParseSessionTableRefusesSINCEBeforeTheLastColumn(t *testing.T) {
	in := "SESSION UID USER LEADER SINCE IDLE CLASS SEAT TTY\n" +
		"c1 1000 kid 1234 2026-10-01 15:47:23 no user seat0 tty1\n"
	_, err := parseSessionTable([]byte(in))
	if err == nil {
		t.Fatal("a header with SINCE mid-table was accepted; every column after it is shifted")
	}
	if !strings.Contains(err.Error(), "SINCE") {
		t.Errorf("error %q does not name the column at fault", err)
	}

	// The same columns, SINCE last, read correctly.
	ok := "SESSION UID USER LEADER IDLE CLASS SEAT TTY SINCE\n" +
		"c1 1000 kid 1234 no user seat0 tty1 2026-10-01 15:47:23\n"
	entries, err := parseSessionTable([]byte(ok))
	if err != nil {
		t.Fatalf("the same layout with SINCE last was refused: %v", err)
	}
	e := entries[0]
	if e.Class != "user" || e.Seat == nil || *e.Seat != "seat0" || e.TTY == nil || *e.TTY != "tty1" {
		t.Errorf("SINCE-last row read wrong: %+v", e)
	}

	// A table with no SINCE column at all is fine -- the guard is about its position,
	// not its presence.
	noSince := "SESSION UID USER LEADER IDLE CLASS SEAT TTY\n" +
		"c1 1000 kid 1234 no user seat0 tty1\n"
	if _, err := parseSessionTable([]byte(noSince)); err != nil {
		t.Errorf("a header without SINCE was refused: %v", err)
	}
}

// A row this code cannot read is a session that would go unenforced. Skipping it is a
// fail-open with no error anywhere, so it has to stop the sweep.
func TestParseSessionTableRejectsATruncatedRow(t *testing.T) {
	in := "SESSION UID USER SEAT LEADER CLASS TTY IDLE SINCE\n" +
		"2 1000 rodney seat0 18344 user tty2 no Thu\n" +
		"3 1001 kid seat0 19\n" // truncated: CLASS, TTY, IDLE, SINCE never arrive
	_, err := parseSessionTable([]byte(in))
	if err == nil {
		t.Fatal("a truncated row was accepted; that session would never be enforced")
	}
	if !strings.Contains(err.Error(), "3 1001 kid") {
		t.Errorf("error %q does not name the row it could not read", err)
	}
}

// loginctl's own trailer is not a session row, and must not be reported as one.
func TestParseSessionTableSkipsTheTrailer(t *testing.T) {
	in := "SESSION UID USER SEAT LEADER CLASS TTY IDLE SINCE\n" +
		"2 1000 rodney seat0 18344 user tty2 no Thu\n" +
		"\n" +
		"1 session listed.\n"
	entries, err := parseSessionTable([]byte(in))
	if err != nil {
		t.Fatalf("parseSessionTable: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("got %d entries, want 1 (the trailer is not a session)", len(entries))
	}
}

// A header-shaped line that is missing a column this code needs is a refusal, not a
// reason to fall through and guess at a later line.
func TestParseSessionTableRejectsAHeaderMissingARequiredColumn(t *testing.T) {
	_, err := parseSessionTable([]byte("SESSION UID USER SEAT TTY IDLE SINCE\n2 1000 rodney seat0 tty2 no Thu\n"))
	if err == nil {
		t.Fatal("a header with no LEADER or CLASS was accepted")
	}
	if !strings.Contains(err.Error(), "LEADER") || !strings.Contains(err.Error(), "CLASS") {
		t.Errorf("error %q does not name the missing columns", err)
	}
}

// A non-numeric UID is not "uid 0", which is root.
func TestParseSessionTableRejectsANonNumericUID(t *testing.T) {
	in := "SESSION UID USER SEAT LEADER CLASS TTY IDLE SINCE\n2 root rodney seat0 18344 user tty2 no Thu\n"
	if _, err := parseSessionTable([]byte(in)); err == nil {
		t.Fatal("UID=root parsed without an error; a wrong uid here aims the lock at the wrong account")
	}
}

// A row with more fields than the header describes has extra data whose column nobody
// declared. Reading it by position would shift nothing (the extra is trailing), but
// reading it at all means inventing a column, so the surplus is dropped.
func TestParseTableRowIgnoresFieldsBeyondTheHeader(t *testing.T) {
	in := "SESSION UID USER SEAT LEADER CLASS TTY IDLE SINCE\n" +
		"2 1000 rodney seat0 18344 user tty2 no Thu extra trailing\n"
	entries, err := parseSessionTable([]byte(in))
	if err != nil {
		t.Fatalf("parseSessionTable: %v", err)
	}
	e := entries[0]
	if e.Session != "2" || e.UID != 1000 || e.User != "rodney" || e.Class != "user" || e.Leader != 18344 {
		t.Errorf("trailing fields disturbed the declared columns: %+v", e)
	}
	if e.Since == nil || *e.Since != "Thu" {
		t.Errorf("SINCE = %v, want Thu", e.Since)
	}
}

// LEADER is a process id. A non-numeric one means the columns were read wrong or the
// output changed shape, and a leader parsed as 0 would be indistinguishable from a
// session with no leader process.
func TestParseSessionTableRejectsANonNumericLeader(t *testing.T) {
	in := "SESSION UID USER SEAT LEADER CLASS TTY IDLE SINCE\n2 1000 rodney seat0 leader user tty2 no Thu\n"
	if _, err := parseSessionTable([]byte(in)); err == nil {
		t.Fatal("LEADER=leader parsed without an error")
	} else if !strings.Contains(err.Error(), "LEADER") {
		t.Errorf("error %q does not name the column that failed", err)
	}
}
