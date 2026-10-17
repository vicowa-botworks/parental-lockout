package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// `plock status` runs as the operator, so the only honest answer to "why will the daemon
// not lock this session?" comes from the daemon's cgroup, not the shell's. These pin the
// three states apart, because each one supports a different sentence and the failure
// mode of collapsing them is a report that reassures on no evidence.

// twoAccounts is one lockable session for the account the daemon lives in and one for a
// neighbour. Both are lockable, so the only thing separating the rows is the rule.
func twoAccounts() []platform.Session {
	return []platform.Session{
		{ID: "2", Username: "casey", UID: 1000, Type: "x11", Seat: "seat0", State: "active", Class: "user"},
		{ID: "3", Username: "alee", UID: 1001, Type: "x11", Seat: "seat1", State: "active", Class: "user"},
	}
}

func TestStatusMarksTheSessionTheDaemonRunsIn(t *testing.T) {
	f := statusFixtureFor(t)
	f.sessions = twoAccounts()
	f.daemon = &daemonState{
		SystemdKnown: true, Running: true, LocationKnown: true,
		Location: platform.SelfLocation{UID: 1000, InUserSlice: true},
	}
	path := withStatusMachine(t, f)

	_, out, _ := runCmd(t, "status", "--config", path, "--json")
	var doc struct {
		Sessions []map[string]any `json:"sessions"`
		Daemon   map[string]any   `json:"daemon"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(doc.Sessions) != 2 {
		t.Fatalf("got %d sessions, want 2:\n%s", len(doc.Sessions), out)
	}
	for _, s := range doc.Sessions {
		declined, _ := s["declinedByDaemon"].(bool)
		if want := s["uid"] == float64(1000); declined != want {
			t.Errorf("session %v uid %v: declinedByDaemon = %v, want %v\n%s",
				s["session"], s["uid"], declined, want, out)
		}
		// lockable stays true: the session could be locked, and the report's job is to
		// say it will not be, not to hide the distinction by folding the two.
		if s["lockable"] != true {
			t.Errorf("session %v stopped being lockable: %v", s["session"], s)
		}
	}
	if doc.Daemon == nil {
		t.Fatalf("no daemon object:\n%s", out)
	}
	if doc.Daemon["running"] != true || doc.Daemon["inUserSlice"] != true {
		t.Errorf("daemon = %v, want running inside a user slice", doc.Daemon)
	}
	if doc.Daemon["uid"] != float64(1000) {
		t.Errorf("daemon uid = %v, want 1000", doc.Daemon["uid"])
	}
}

// The text is what a person reads before they reach for --json, and the row that reads
// "lockable / unlocked" is the row that looks like a daemon failure. The reason has to
// arrive on that row.
func TestStatusTextPutsTheDeclineReasonOnTheRowItExplains(t *testing.T) {
	f := statusFixtureFor(t)
	f.sessions = twoAccounts()
	f.daemon = &daemonState{
		SystemdKnown: true, Running: true, LocationKnown: true,
		Location: platform.SelfLocation{UID: 1000, InUserSlice: true},
	}
	path := withStatusMachine(t, f)

	_, out, _ := runCmd(t, "status", "--config", path)
	lines := strings.Split(out, "\n")
	var marked, unmarked int
	for _, l := range lines {
		if !strings.Contains(l, "uid 1000 ") && !strings.Contains(l, "uid 1001 ") {
			continue
		}
		if strings.Contains(l, "declined") {
			marked++
			if !strings.Contains(l, "uid 1000") {
				t.Errorf("the declined row is the wrong one: %q", l)
			}
		} else {
			unmarked++
		}
	}
	if marked != 1 || unmarked != 1 {
		t.Errorf("marked %d rows and left %d, want 1 and 1\n%s", marked, unmarked, out)
	}
	// The header line names the account, because "declined" without "by what" is the
	// same silence with more syllables.
	if !strings.Contains(out, "uid 1000's login session") {
		t.Errorf("no daemon line naming the declined account:\n%s", out)
	}
}

// A daemon that is running but unreadable must not render as a daemon that declines
// nobody. That rendering is the reassurance with nothing behind it.
func TestStatusSaysItCannotTellRatherThanDecliningNobody(t *testing.T) {
	f := statusFixtureFor(t)
	f.sessions = twoAccounts()
	f.daemon = &daemonState{SystemdKnown: true, Running: true, LocationKnown: false}
	path := withStatusMachine(t, f)

	code, out, _ := runCmd(t, "status", "--config", path, "--json")
	if code != exitOK {
		t.Fatalf("exit %d, want %d", code, exitOK)
	}
	if strings.Contains(out, "declinedByDaemon") {
		t.Errorf("a session was marked declined on a location the probe could not read:\n%s", out)
	}
	var doc struct {
		Warnings []string       `json:"warnings"`
		Daemon   map[string]any `json:"daemon"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var warned bool
	for _, w := range doc.Warnings {
		if strings.Contains(w, "cgroup could not be read") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("no warning about the unreadable slice:\n%v", doc.Warnings)
	}
	if doc.Daemon["running"] != true || doc.Daemon["locationKnown"] != false {
		t.Errorf("daemon = %v, want running with locationKnown false", doc.Daemon)
	}

	_, text, _ := runCmd(t, "status", "--config", path)
	if !strings.Contains(text, "slice unknown") {
		t.Errorf("the text report does not say the slice is unknown:\n%s", text)
	}
	if strings.Contains(text, "no session is declined") {
		t.Errorf("the text report asserts nothing is declined, on no evidence:\n%s", text)
	}
}

// The known-ness half of the decline rule, on its own.
//
// `askDaemonLocation` cannot build this state today: it leaves Location zero whenever
// the read fails, and a zero location declines nobody anyway. The guard is there because
// the rule is not answered by that one producer -- daemonProbe is a seam, and the whole
// point of the seam is that a second way to learn the answer can arrive (systemd's
// ControlGroup property, a socket to the daemon). When it does, the safety property must
// live in the rule rather than in the producer's discipline, because the failure it
// prevents is a confident decline mark on an unverified read.
func TestStatusMarksNothingWhenTheDaemonLocationIsUnverified(t *testing.T) {
	f := statusFixtureFor(t)
	f.sessions = twoAccounts()
	f.daemon = &daemonState{
		SystemdKnown: true, Running: true, LocationKnown: false,
		Location: platform.SelfLocation{UID: 1000, InUserSlice: true},
	}
	path := withStatusMachine(t, f)

	code, out, _ := runCmd(t, "status", "--config", path, "--json")
	if code != exitOK {
		t.Fatalf("exit %d, want %d", code, exitOK)
	}
	if strings.Contains(out, "declinedByDaemon") {
		t.Errorf("a session was marked declined from a location the report calls unverified:\n%s", out)
	}
	_, text, _ := runCmd(t, "status", "--config", path)
	if strings.Contains(text, "declined:") {
		t.Errorf("a row carried a decline reason the report says it cannot establish:\n%s", text)
	}
}

// "No init system" and "not running" are different findings, and the second is the one a
// reader acts on. Saying "not running" where systemctl could not be executed is the tool
// inventing the problem it was called in to look for.
func TestStatusSeparatesNoSystemdFromNotRunning(t *testing.T) {
	f := statusFixtureFor(t)
	f.daemon = &daemonState{SystemdKnown: false}
	path := withStatusMachine(t, f)

	_, text, _ := runCmd(t, "status", "--config", path)
	if !strings.Contains(text, "no init system") {
		t.Errorf("no line saying systemd could not be reached:\n%s", text)
	}
	if strings.Contains(text, "daemon                         not running") {
		t.Errorf("said \"not running\" without an init system to have said so:\n%s", text)
	}

	_, out, _ := runCmd(t, "status", "--config", path, "--json")
	if !strings.Contains(out, `"systemd":false`) {
		t.Errorf("the JSON does not separate the two states:\n%s", out)
	}
}

// The case an operator most needs, and the one the column used to leave blank: nothing
// is running, so nothing is declining anyone, and the reason nothing locked is here.
func TestStatusSaysTheDaemonIsNotRunning(t *testing.T) {
	f := statusFixtureFor(t)
	f.sessions = twoAccounts()
	// The fixture's default: systemd answered, MainPID 0.
	path := withStatusMachine(t, f)

	_, text, _ := runCmd(t, "status", "--config", path)
	if !strings.Contains(text, "not running") {
		t.Errorf("no \"not running\" line:\n%s", text)
	}
	if strings.Contains(text, "declined") {
		t.Errorf("a session was marked declined with no daemon running:\n%s", text)
	}

	_, out, _ := runCmd(t, "status", "--config", path, "--json")
	if !strings.Contains(out, `"running":false`) {
		t.Errorf("daemon.running is not false:\n%s", out)
	}
}

// A system unit is the supported deployment: it declines nobody, and that is an answer
// rather than a gap, so it gets said plainly.
func TestStatusSaysASystemUnitDeclinesNoSession(t *testing.T) {
	f := statusFixtureFor(t)
	f.sessions = twoAccounts()
	f.daemon = &daemonState{SystemdKnown: true, Running: true, LocationKnown: true}
	path := withStatusMachine(t, f)

	_, text, _ := runCmd(t, "status", "--config", path)
	if !strings.Contains(text, "no session is declined") {
		t.Errorf("a system unit's report does not say it declines nobody:\n%s", text)
	}
	if strings.Contains(text, "declined:") {
		t.Errorf("a system unit marked a session declined:\n%s", text)
	}
}

// The report's own contract: the daemon object is part of the published key set the
// moment it ships, and it is absent (not all-false) when nothing probed.
func TestStatusJSONCarriesTheDaemonObject(t *testing.T) {
	f := statusFixtureFor(t)
	path := withStatusMachine(t, f)

	_, out, _ := runCmd(t, "status", "--config", path, "--json")
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	raw, ok := doc["daemon"]
	if !ok {
		t.Fatalf("--json document has no \"daemon\" field: %s", out)
	}
	var d map[string]json.RawMessage
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("daemon is not an object: %s", raw)
	}
	for _, key := range []string{"systemd", "running", "inUserSlice", "locationKnown"} {
		if _, ok := d[key]; !ok {
			t.Errorf("daemon has no %q key: %s", key, raw)
		}
	}
}
