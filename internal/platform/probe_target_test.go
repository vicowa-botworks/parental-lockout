package platform

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// #25 item 4: the property-presence probe asked about entries[0], and on a real box
// entries[0] is frequently a class=manager row -- no seat, Type=unspecified, never a
// lock candidate. So the sentence "this loginctl does not report X" can be reached on
// the evidence of a session whose class is exactly why X is missing, while the class the
// daemon actually locks would have answered it.
//
// The fixture models the future loginctl the issue names: one that reports Type only for
// class=user. Manager-first is the order that used to decide the answer.
func TestThePropertyProbeAsksAboutALockableSession(t *testing.T) {
	r := serve(t, "list-sessions.manager-first.json", map[string]string{
		"4": "show-session-manager-notype.txt",
		"3": "show-session-desktop-typed.txt",
	})
	st := newLogind(t, r).Probe(context.Background())
	if len(st) != 1 || !st[0].Available {
		t.Fatalf("Probe = %+v, want logind available", st)
	}
	if strings.Contains(st[0].Detail, "does not report") {
		t.Errorf("Probe blamed a manager row for a property the desktop session reports: %q", st[0].Detail)
	}

	// The assertion the description promised and this test did not have. The text check
	// above does discriminate -- `serve` routes the id through the fixture, so asking "4"
	// produces the complaint and asking "3" does not -- but that is the fixture doing the
	// asserting. A Probe that asked about *both* rows would land the text assertion either
	// way, depending on which answer it happened to look at last. Naming the id is the only
	// form that says what the test is about.
	var asked []string
	for _, argv := range r.Argvs() {
		if sub(argv, 0) == "show-session" {
			asked = append(asked, sub(argv, 1))
		}
	}
	if len(asked) != 1 || asked[0] != "3" {
		t.Errorf("show-session was called for %v, want exactly [3] -- the class=user row. "+
			"Manager-first is the order this test exists to reject.", asked)
	}
}

// The control that keeps the test above from passing by asking nothing at all: with no
// lockable row in the listing, the probe still asks about the first row, so a genuinely
// missing property is still reported rather than silently dropped.
func TestThePropertyProbeStillReportsWhenNothingIsLockable(t *testing.T) {
	// A listing of manager rows only -- the shape a headless box actually has.
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if sub(argv, 0) == "list-sessions" {
			return []byte(`[{"session":"4","uid":1000,"user":"rodney","seat":null,"leader":6695,"class":"manager","tty":null,"idle":false,"since":null}]`), nil
		}
		return []byte("Name=rodney\nRemote=no\nState=active\nLockedHint=no\n"), nil
	}}

	st := newLogind(t, r).Probe(context.Background())
	if len(st) != 1 {
		t.Fatalf("Probe = %+v", st)
	}
	if !strings.Contains(st[0].Detail, "does not report: Type") {
		t.Errorf("Probe went quiet about a property nobody answered: %q", st[0].Detail)
	}
}

func str(s string) *string { return &s }

// The table the description named. It did not exist when the description was written,
// which is the failure mode this project has now been told about twice: a bullet claiming
// a test the next person cannot find with grep.
//
// The predicate is shared by two callers -- Sessions() filters with it, and probeTarget
// picks with it -- and a divergence between the two is invisible from either side: the
// listing would simply stop showing a row the probe asked about, or the probe would ask
// about a row the listing never returns. So the shapes are pinned here, once.
//
// `greeter` is not hypothetical: a display-manager greeter runs a class=greeter session
// with a real seat, and locking it would lock the login screen out from under whoever is
// about to log in. A class=user row with no seat is the other real one -- a remote or
// headless user session has no seat to lock a screen on.
func TestTheDesktopPredicateOnEveryRowShape(t *testing.T) {
	cases := []struct {
		name  string
		entry listEntry
		want  bool
	}{
		{"desktop session", listEntry{Session: "3", Class: "user", Seat: str("seat0")}, true},
		{"manager session, no seat", listEntry{Session: "4", Class: "manager", Seat: nil}, false},
		{"manager session, empty seat", listEntry{Session: "4", Class: "manager", Seat: str("")}, false},
		{"greeter with a real seat", listEntry{Session: "2", Class: "greeter", Seat: str("seat0")}, false},
		{"user class, no seat at all", listEntry{Session: "7", Class: "user", Seat: nil}, false},
		{"user class, seat present but empty", listEntry{Session: "7", Class: "user", Seat: str("")}, false},
		{"user class with a seat, no session id", listEntry{Session: "", Class: "user", Seat: str("seat0")}, false},
		{"lock-screen class", listEntry{Session: "9", Class: "lock-screen", Seat: str("seat0")}, false},
		{"empty row", listEntry{}, false},
	}
	for _, tc := range cases {
		if got := isDesktopEntry(tc.entry); got != tc.want {
			t.Errorf("isDesktopEntry(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The other half of "the two callers cannot diverge": the listing has to actually use the
// predicate, rather than carrying a second copy of the same triple that can drift.
func TestTheListingFiltersWithTheSamePredicate(t *testing.T) {
	// Every row the predicate rejects must be absent from Sessions(), and the one it
	// accepts must be present -- in a single call, so a filter that widened or narrowed
	// relative to the predicate shows up as a wrong count rather than a wrong row.
	rows := `[
		{"session":"3","uid":1000,"user":"casey","seat":"seat0","leader":6001,"class":"user","tty":null,"idle":false,"since":null},
		{"session":"4","uid":1000,"user":"casey","seat":null,"leader":6695,"class":"manager","tty":null,"idle":false,"since":null},
		{"session":"2","uid":111,"user":"lightdm","seat":"seat0","leader":5200,"class":"greeter","tty":null,"idle":false,"since":null},
		{"session":"7","uid":1001,"user":"rae","seat":null,"leader":7100,"class":"user","tty":null,"idle":false,"since":null}
	]`
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		switch sub(argv, 0) {
		case "list-sessions":
			return []byte(rows), nil
		case "show-session":
			if sub(argv, 1) != "3" {
				t.Errorf("Sessions() asked for details of session %q, which the predicate rejects", sub(argv, 1))
				return nil, errors.New("unexpected show-session target")
			}
			return []byte("Name=casey\nType=wayland\nState=active\nRemote=no\nLockedHint=no\n"), nil
		}
		t.Errorf("unexpected command from the backend: %v", argv)
		return nil, errors.New("unexpected command")
	}}

	sessions, err := newLogind(t, r).Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "3" {
		ids := make([]string, 0, len(sessions))
		for _, s := range sessions {
			ids = append(ids, s.ID)
		}
		t.Errorf("Sessions() returned %v, want exactly [3]: the listing and the predicate disagree", ids)
	}
}

// The fallback's own bug: "no lock candidate, ask about the first row" hands back "" when
// row zero has no session id, and the caller's `target != ""` check then skips the probe
// altogether. A JSON row with a missing `session` key parses without complaint, so this is
// reachable without anyone doing anything exotic.
//
// The symptom is the one this package keeps refusing: the probe goes quiet. Not "logind
// cannot report Type" -- nothing at all.
func TestTheProbeFallbackSkipsARowWithNoSessionID(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		switch sub(argv, 0) {
		case "list-sessions":
			// Row 0 has no session id; row 1 is the manager row that does. Neither is a
			// lock candidate, so this exercises the fallback rather than the predicate.
			return []byte(`[
				{"uid":1000,"user":"nobody","seat":null,"leader":1,"class":"user","tty":null,"idle":false,"since":null},
				{"session":"4","uid":1000,"user":"casey","seat":null,"leader":6695,"class":"manager","tty":null,"idle":false,"since":null}
			]`), nil
		case "show-session":
			return []byte("Name=casey\nRemote=no\nState=active\nLockedHint=no\n"), nil
		}
		t.Errorf("unexpected command from the backend: %v", argv)
		return nil, errors.New("unexpected command")
	}}

	st := newLogind(t, r).Probe(context.Background())
	if len(st) != 1 {
		t.Fatalf("Probe = %+v", st)
	}
	if !strings.Contains(st[0].Detail, "does not report: Type") {
		t.Errorf("the probe went quiet because the first row had no id: %q", st[0].Detail)
	}
	var asked []string
	for _, argv := range r.Argvs() {
		if sub(argv, 0) == "show-session" {
			asked = append(asked, sub(argv, 1))
		}
	}
	if len(asked) != 1 || asked[0] != "4" {
		t.Errorf("show-session was called for %v, want exactly [4] -- the first row that can be asked about", asked)
	}
}
