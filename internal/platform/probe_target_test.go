package platform

import (
	"context"
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
