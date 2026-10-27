package platform

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestALiveSystemd259ListingDecodes is the bug report kept as a test.
//
// list-sessions.since-number.json is what this development host printed on 27 Oct under
// systemd 259.9-0ubuntu0.1: five rows, four of them class=user on seat0, and `"since":
// 855186289603` on every row the listing reports as idle. `Since` was a *string, so the
// decode returned an error, and because list() decodes the whole array in one call, one
// field on one row cost the daemon every session on the machine (#167).
//
// The fixture that has "captured from a live systemd 259" written in the testdata README
// carries `"since":null` in both of its rows. That is why nothing saw this: the only
// sample of the field in the whole suite was the one value that cannot fail.
func TestALiveSystemd259ListingDecodes(t *testing.T) {
	entries, err := parseSessionJSON(fixture(t, "list-sessions.since-number.json"))
	if err != nil {
		t.Fatalf("the live 259.9 listing does not decode: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("decoded %d rows, want the 5 the capture carries", len(entries))
	}

	want := map[string]string{
		"468": "855186289603",
		"469": "<nil>", // the manager session: logind prints null
		"472": "855186289153",
		"567": "855186291473",
		"570": "<nil>", // idle=false, and the capture prints null for it
	}
	for _, e := range entries {
		got := "<nil>"
		if e.Since != nil {
			got = string(*e.Since)
		}
		if got != want[e.Session] {
			t.Errorf("session %s: since = %q, want %q", e.Session, got, want[e.Session])
		}
	}
}

// TestTheLiveListingYieldsTheLockableSessions walks the path the failure was actually
// observed on — the dashboard's "session list unavailable" and a 503 on the lock button —
// because "a struct decodes" is not the property anyone needed. The property is that a
// daemon on a machine with four lockable sessions reports four lockable sessions.
func TestTheLiveListingYieldsTheLockableSessions(t *testing.T) {
	r := serve(t, "list-sessions.since-number.json", map[string]string{
		"468": "show-session-468.txt",
		"472": "show-session-472.txt",
		"567": "show-session-567.txt",
		"570": "show-session-570.txt",
	})

	got, err := newLogind(t, r).Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	ids := make([]string, 0, len(got))
	for _, s := range got {
		ids = append(ids, s.ID)
	}
	// 469 is the manager row: same uid, no seat, nothing to lock.
	if want := []string{"468", "472", "567", "570"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("sessions = %v, want %v", ids, want)
	}
}

// TestTheSinceFieldTakesEveryShapeLogindHasPrinted pins the type against everything the
// field has been measured sending, plus the shapes it has not — because the failure this
// type exists to end was a value refusing to decode, and the fix must not be "accept
// whatever arrives".
func TestTheSinceFieldTakesEveryShapeLogindHasPrinted(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string // "<nil>" means the pointer stayed nil
		wantErr bool
	}{
		{"integer", `{"session":"1","since":855186289603}`, "855186289603", false},
		{"negative integer", `{"session":"1","since":-5}`, "-5", false},
		{"wall clock", `{"session":"1","since":"Thu 2026-10-01 15:47:23 UTC"}`, "Thu 2026-10-01 15:47:23 UTC", false},
		{"text needing unquoting", `{"session":"1","since":"a\"b"}`, `a"b`, false},
		{"empty text", `{"session":"1","since":""}`, "", false},
		{"null", `{"session":"1","since":null}`, "<nil>", false},
		{"absent", `{"session":"1"}`, "<nil>", false},
		{"object", `{"session":"1","since":{"a":1}}`, "", true},
		{"array", `{"session":"1","since":[1]}`, "", true},
		{"bare true", `{"session":"1","since":true}`, "", true},
	}
	for _, tc := range tests {
		var e listEntry
		err := json.Unmarshal([]byte(tc.in), &e)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: decoded without an error; a shape nobody has measured must not parse into a row", tc.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		got := "<nil>"
		if e.Since != nil {
			got = string(*e.Since)
		}
		if got != tc.want {
			t.Errorf("%s: since = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestANullReachesTheTypeAsTheEmptyText is the same claim one level down, and it is the
// reason UnmarshalJSON carries a null arm the listing never uses.
//
// Through the struct the arm cannot run: encoding/json sets a pointer to nil for a null
// without consulting an Unmarshaler, which the table above pins. It is there because the
// day the field stops being a pointer — or the day encoding/json changes — a null would
// otherwise arrive at the refusal, and a refusal on one row is the whole listing lost.
// The catastrophic direction gets the defensive arm even when the current caller cannot
// reach it, as long as a test can.
func TestANullReachesTheTypeAsTheEmptyText(t *testing.T) {
	var s since
	if err := json.Unmarshal([]byte("null"), &s); err != nil {
		t.Fatalf("null into a since value: %v", err)
	}
	if s != "" {
		t.Errorf("null decoded to %q, want the empty text", s)
	}
}

// TestTheTablePathAndTheJSONPathAgreeOnAnAbsentSince keeps the promise tableField's
// comment makes about the two parsers producing the same shape. A "-" in the table and a
// null in the JSON are one fact — logind did not say — and they have to arrive identically.
//
// They cannot agree about a PRESENT value, and that is left alone on purpose: the table
// prints a wall clock and 259.9's JSON prints an integer, so normalising one into the
// other would be this package claiming the two spellings mean the same thing.
func TestTheTablePathAndTheJSONPathAgreeOnAnAbsentSince(t *testing.T) {
	tbl, err := parseSessionTable([]byte("SESSION  UID USER   SEAT  LEADER CLASS TTY  IDLE SINCE\n" +
		"3 1000 rodney seat0 6654 user tty2 no -\n"))
	if err != nil {
		t.Fatalf("parseSessionTable: %v", err)
	}
	js, err := parseSessionJSON([]byte(`[{"session":"3","uid":1000,"user":"rodney","seat":"seat0",` +
		`"leader":6654,"class":"user","tty":"tty2","idle":false,"since":null}]`))
	if err != nil {
		t.Fatalf("parseSessionJSON: %v", err)
	}
	if len(tbl) != 1 || len(js) != 1 {
		t.Fatalf("parsed %d table rows and %d json rows, want one each", len(tbl), len(js))
	}
	if tbl[0].Since != nil || js[0].Since != nil {
		t.Errorf("absent SINCE: table gave %+v, json gave %+v; both must be nil", tbl[0].Since, js[0].Since)
	}
}

// TestAnUnreadableListingSaysWhyTheJSONFailed is the diagnosability half of #167.
//
// The message this box produced was `loginctl list-sessions: unrecognised output:
// [{"session":"468",…` — it blamed the listing and threw the decode error away, so the
// only way to find out what had actually failed was to run the parser by hand. The
// fallback arm already states the principle two tests away: both failures belong in the
// message.
//
// #168 changed what this case reports, and improved it: a listing whose rows are all
// unreadable is no longer handed to the table parser, so the message is the row-level one
// rather than "unrecognised output … (json: …; table: no column header)". What it has to
// keep doing is naming the field and the value, which is the whole point of the test.
func TestAnUnreadableListingSaysWhyTheJSONFailed(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		return []byte(`[{"session":"3","uid":1000,"user":"rodney","seat":"seat0","leader":6654,` +
			`"class":"user","tty":"tty2","idle":false,"since":{"nope":1}}]`), nil
	}}

	_, err := (&logind{r: r}).list(context.Background())
	if err == nil {
		t.Fatal("a listing whose since is an object parsed successfully")
	}
	for _, want := range []string{"neither text nor a number", "row 0", "1 of 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q", err, want)
		}
	}
}
