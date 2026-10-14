package config

import (
	"encoding/json"
	"strings"
	"testing"
)

// #16: a rule is two HH:MM boxes, so 00:00–23:59 is 1439 minutes and the last
// minute of every day is free. allDay is the way to say there is no minute free.
//
// These tests are the storage half. The evaluator half — that the window really
// covers the minute the boxes miss, and that it still ends — is
// internal/schedule/allday_test.go.

func TestRuleDecodesAllDay(t *testing.T) {
	for _, tc := range []struct {
		name string
		json string
		want bool
	}{
		{"declared", `{"id":"a","days":["mon"],"start":"","end":"","allDay":true}`, true},
		// The absence is not a shadow case and deliberately has no *bool. Every
		// other optional boolean in this file defaults toward the lockout, because
		// a rule is written to restrict a child. allDay widens a window rather than
		// gating one, so defaulting it on would lock every account around the clock
		// the moment a rule was saved without it — including one that spells out
		// 21:00 to 06:30 and means exactly that.
		{"omitted", `{"id":"b","days":["mon"],"start":"21:00","end":"06:30"}`, false},
		{"explicitly off", `{"id":"c","days":["mon"],"start":"21:00","end":"06:30","allDay":false}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var r Rule
			if err := json.Unmarshal([]byte(tc.json), &r); err != nil {
				t.Fatalf("decode %s: %v", tc.json, err)
			}
			if r.AllDay != tc.want {
				t.Errorf("AllDay = %v, want %v", r.AllDay, tc.want)
			}
		})
	}
}

// A wrong type is refused rather than read as false. `"allDay":"yes"` is the kind
// of thing a hand edit produces, and a rule that silently loses its all-day
// window releases the child at 23:59 while the parent believes otherwise.
func TestRuleRejectsANonBooleanAllDay(t *testing.T) {
	for _, tc := range []struct {
		value   string
		wantErr bool
	}{
		{`"yes"`, true},
		{`1`, true},
		// null is the one the decoder allows: it leaves a bool at its zero value,
		// so the rule becomes an ordinary one. That is the safe direction — it
		// releases a minute rather than locking a day.
		{`null`, false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			var r Rule
			err := json.Unmarshal([]byte(`{"id":"a","days":["mon"],"allDay":`+tc.value+`}`), &r)
			if tc.wantErr && err == nil {
				t.Fatalf("decoded allDay:%s with no error; AllDay = %v", tc.value, r.AllDay)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("allDay:%s: %v", tc.value, err)
			}
		})
	}
}

// allDay has to be a field the decoder knows. The document decoder refuses unknown
// keys — that is the typo guard for the whole file — so a rule using the flag
// before the struct carried it would have failed to load at all.
func TestAllDayIsAKnownKey(t *testing.T) {
	c := sample()
	c.Profiles[0].Rules = []Rule{{ID: "always", Name: "All day", Days: []string{"mon"}, AllDay: true, Enabled: true}}

	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"allDay":true`) {
		t.Fatalf("marshalled config does not carry the flag: %s", raw)
	}

	var back Config
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("the config this package wrote does not load: %v", err)
	}
	if !back.Profiles[0].Rules[0].AllDay {
		t.Error("AllDay did not survive a marshal/unmarshal round trip")
	}
}

// The same through the file, where the failure would be silent in practice: a
// flag the writer drops is a rule that stops being all-day on the next save.
func TestAllDaySurvivesAFileRoundTrip(t *testing.T) {
	path := t.TempDir() + "/config.json"
	c := sample()
	c.Profiles[0].Rules = []Rule{
		{ID: "always", Name: "All day", Days: []string{"mon", "tue"}, AllDay: true, Enabled: true},
		{ID: "bedtime", Name: "Bedtime", Days: []string{"wed"}, Start: "21:00", End: "06:30", Enabled: true},
	}
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := back.Profiles[0].Rules; len(got) != 2 || !got[0].AllDay || got[1].AllDay {
		t.Fatalf("rules after a round trip = %+v", got)
	}
}

// The validation half. An all-day rule is not asked for valid clock ends, because
// the evaluator never reads them.
//
// The alternative — insisting the times stay well-formed — would make a checkbox
// change which of the form's own fields are legal, and would refuse a document
// that enforces exactly what its author meant.
func TestValidateAcceptsAnAllDayRuleWithAnyClock(t *testing.T) {
	for _, tc := range []struct{ name, start, end string }{
		{"cleared", "", ""},
		{"left behind by the form", "20:00", "06:00"},
		{"the shape Validate rejects without the flag", "08:00", "08:00"},
		{"not times at all", "bedtime", "25:00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			c.Profiles[0].Rules = []Rule{{
				ID: "always", Days: []string{"mon"},
				Start: tc.start, End: tc.end, AllDay: true, Enabled: true,
			}}
			if err := c.Validate(); err != nil {
				t.Errorf("Validate: %v", err)
			}
		})
	}
}

// The rejection stays for the rule that did not ask for all day, and its message
// now points at the flag instead of recommending a window that is a minute short.
func TestValidateEmptyWindowStillRejectedAndPointsAtAllDay(t *testing.T) {
	c := valid()
	c.Profiles[0].Rules = []Rule{{ID: "typo", Days: []string{"mon"}, Start: "08:00", End: "08:00", Enabled: true}}

	err := c.Validate()
	if err == nil {
		t.Fatal("Validate accepted a window that ends where it starts")
	}
	msg := err.Error()
	if !strings.Contains(msg, "allDay") {
		t.Errorf("error %q does not name the flag that means all day", msg)
	}
	// The old advice is the bug being fixed, so a test that it does not come back.
	if strings.Contains(msg, "23:59") && !strings.Contains(msg, "minute short") {
		t.Errorf("error %q still recommends 00:00 to 23:59 as if it were all day", msg)
	}
	// And it must still not claim the shape locks the account.
	if strings.Contains(msg, "locks the account") || strings.Contains(msg, "all day rule, and note") {
		t.Errorf("error %q overstates what an empty window does", msg)
	}
}
