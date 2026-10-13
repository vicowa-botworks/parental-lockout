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

func TestPropsUint(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want uint32
		ok   bool
	}{
		{"1000", 1000, true},
		{"0", 0, true},
		{"", 0, false},
		{"abc", 0, false},
		{"-1", 0, false},
		{"99999999999999999999", 0, false}, // overflows uint32
	} {
		got, ok := (props{"k": tc.in}).uint("k")
		if got != tc.want || ok != tc.ok {
			t.Errorf("uint(%q) = %d,%v want %d,%v", tc.in, got, ok, tc.want, tc.ok)
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
	// Header but no rows: refuse, because "no sessions" and "cannot read the output"
	// have to stay distinguishable.
	if _, err := parseSessionTable([]byte("SESSION  UID USER SEAT LEADER CLASS TTY IDLE SINCE\n\n2 sessions listed.\n")); err == nil {
		t.Error("a header with no rows parsed successfully")
	}
	// A row whose LEADER is not a number is not a session row.
	if _, err := parseSessionTable([]byte("SESSION UID USER SEAT LEADER CLASS TTY IDLE SINCE\nx y z - q user - no -\n")); err == nil {
		t.Error("a row with a non-numeric leader parsed successfully")
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
		if argv[1] == "list-sessions" {
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
		if argv[1] == "list-sessions" {
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
		if len(argv) >= 3 && argv[2] == "--json=short" {
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
	err := newLogind(t, r).Lock(context.Background(), Session{ID: "3", Type: "x11", State: "active"})
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

// The listing's uid is 0 for a session logind could not resolve, and show-session
// carries the real one.
func TestSessionsTakesTheUIDFromShowWhenTheListingHasNone(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if argv[1] == "list-sessions" {
			return []byte(`[{"session":"9","uid":0,"user":"kid","seat":"seat0","leader":5,"class":"user","tty":null,"idle":false,"since":null}]`), nil
		}
		return []byte("Type=x11\nState=active\nRemote=no\nUID=1005\n"), nil
	}}
	got, err := newLogind(t, r).Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(got) != 1 || got[0].UID != 1005 {
		t.Fatalf("got %v, want uid 1005 from show-session", got)
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
		if len(argv) >= 3 && argv[2] == "--json=short" {
			return nil, errors.New("Unknown option --json=short")
		}
		return []byte("total nonsense\n"), nil
	}}
	_, err := (&logind{r: r}).list(context.Background())
	if err == nil {
		t.Fatal("unreadable output parsed successfully")
	}
	for _, want := range []string{"Unknown option", "no SESSION UID header"} {
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
