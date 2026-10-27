package platform

// #168: one unreadable row costs that row, not the machine.
//
// #167 fixed the field that failed on the development host. This file pins the shape of the
// failure, which outlives any one field: the listing is decoded row by row, a row that cannot
// be read is reported rather than silently subtracted, and the rows that did read reach the
// caller instead of being thrown away with it. Every test here fails if the array goes back
// to decoding in one call, or if a partial read starts being treated as no read.

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// serveInline is serve() with the bytes stated in the test instead of read from testdata.
// A partial listing is a fixture that must not be committed: testdata is for captures, and
// inventing a capture would be exactly the mistake the README records — the one committed
// listing carried `"since":null` in both rows and so was the only sample of the field that
// could not fail.
func serveInline(listing string, details map[string]string) *fakeRunner {
	return &fakeRunner{respond: func(argv []string) ([]byte, error) {
		switch sub(argv, 0) {
		case "list-sessions":
			return []byte(listing), nil
		case "show-session":
			body, ok := details[sub(argv, 1)]
			if !ok {
				return nil, errors.New("loginctl show-session " + sub(argv, 1) + ": No session known")
			}
			return []byte(body), nil
		}
		return nil, errors.New("unexpected command")
	}}
}

// rowJSON spells one listing row. The bad one carries a uid that is not a number, which is
// the same class of surprise as #167's integer `since`: a field arriving as something the
// struct cannot hold.
func rowJSON(id string, uid string) string {
	return `{"session":"` + id + `","uid":` + uid + `,"user":"kid","seat":"seat0","leader":6654,` +
		`"class":"user","tty":"tty2","idle":false,"since":null}`
}

const desktopProps = "Name=kid\nType=x11\nState=active\nRemote=no\nLockedHint=no\n"

func TestOneBadRowCostsThatRowAndNothingElse(t *testing.T) {
	listing := "[" + strings.Join([]string{
		rowJSON("3", "1001"),
		rowJSON("4", `"not-a-number"`),
		rowJSON("5", "1002"),
	}, ",") + "]"

	entries, err := parseSessionJSON([]byte(listing))
	if err == nil {
		t.Fatal("a row with a uid that is not a number parsed without complaint")
	}
	if len(entries) != 2 {
		t.Fatalf("got %d rows, want the 2 that parsed", len(entries))
	}
	if entries[0].Session != "3" || entries[1].Session != "5" {
		t.Errorf("kept sessions %q and %q, want 3 and 5", entries[0].Session, entries[1].Session)
	}
	// The loss is stated, not subtracted: "2 rows" alone would read as a machine with two
	// sessions on it.
	for _, want := range []string{"1 of 3", "row 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %s", err, want)
		}
	}
}

func TestAPartialListingIsNotReParsedAsATable(t *testing.T) {
	// The arm this test exists for. A JSON decode error is list()'s cue to fall back to the
	// table parser, and falling back over JSON bytes always fails — which is how one bad row
	// used to come out the other end as "unrecognised output" with the good rows discarded.
	listing := "[" + strings.Join([]string{
		rowJSON("3", "1001"),
		rowJSON("4", `"not-a-number"`),
	}, ",") + "]"
	r := serveInline(listing, nil)

	entries, err := (&logind{r: r}).list(context.Background())
	if len(entries) != 1 || entries[0].Session != "3" {
		t.Fatalf("the rows that read were discarded: %+v (err %v)", entries, err)
	}
	if err == nil {
		t.Error("the dropped row went unreported")
	}
	if n := len(r.Calls()); n != 1 {
		t.Errorf("%d command(s) run, want 1: a partial listing must not spend the table fallback\n%v",
			n, r.Argvs())
	}
	if strings.Contains(err.Error(), "unrecognised output") {
		t.Errorf("a readable listing was reported as unrecognised: %v", err)
	}
}

func TestAPartialListingStillYieldsTheSessionsItRead(t *testing.T) {
	// The property the enforcement loop actually needs: four sessions and an error is a
	// poll in which four children can still be locked.
	listing := "[" + strings.Join([]string{
		rowJSON("3", "1001"),
		rowJSON("4", `"not-a-number"`),
		rowJSON("5", "1002"),
	}, ",") + "]"
	r := serveInline(listing, map[string]string{
		"3": desktopProps,
		"5": desktopProps,
	})

	got, err := newLogind(t, r).Sessions(context.Background())
	if err == nil {
		t.Fatal("Sessions reported a clean read over a listing with a row it could not parse")
	}
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want the 2 that read: %v", len(got), got)
	}
	if got[0].ID != "3" || got[1].ID != "5" {
		t.Errorf("sessions = %s and %s, want 3 and 5", got[0].ID, got[1].ID)
	}
	if !strings.Contains(err.Error(), "row 1") {
		t.Errorf("the error does not identify the dropped row: %v", err)
	}
}

func TestAListingWithNothingReadableIsNotSilentlyEmpty(t *testing.T) {
	// The other arm, so that "use what read" cannot become "ignore errors". With no rows the
	// caller has nothing to act on and must be told.
	listing := "[" + rowJSON("3", `"nope"`) + "," + rowJSON("4", `"nope"`) + "]"
	r := serveInline(listing, nil)

	got, err := newLogind(t, r).Sessions(context.Background())
	if err == nil {
		t.Fatal("an entirely unreadable listing returned no error")
	}
	if len(got) != 0 {
		t.Fatalf("got %d sessions from a listing that parsed no rows", len(got))
	}
	if !strings.Contains(err.Error(), "2 of 2") {
		t.Errorf("the error does not say the whole listing was lost: %v", err)
	}
}

func TestATableListingStillFallsBack(t *testing.T) {
	// The fallback #167's fix depends on is the one for output that is not JSON at all, and
	// the row-by-row decode must not swallow it. This is what a build that accepts
	// --json=short and ignores it produces.
	table := "SESSION  UID USER SEAT  LEADER CLASS TTY  IDLE SINCE\n" +
		"3 1001 kid  seat0 6654  user tty2 no   -\n"
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if sub(argv, 0) == "list-sessions" {
			if hasFlag(argv, "--json=short") {
				return []byte(table), nil
			}
			return []byte(table), nil
		}
		return nil, errors.New("unexpected command")
	}}

	entries, err := (&logind{r: r}).list(context.Background())
	if err != nil {
		t.Fatalf("a plain table listing did not parse: %v", err)
	}
	if len(entries) != 1 || entries[0].Session != "3" {
		t.Fatalf("the table fallback stopped working: %+v", entries)
	}
}
