package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// statusFixture is the machine `status` sees: a fixed clock, a fixed config, and a fixed
// session list.
//
// The clock is the part that matters. A status test that lets the code call time.Now and
// computes its expectation from time.Now too asserts that a value equals itself, and it
// will pass on every schedule ever written while pinning nothing about any of them.
type statusFixture struct {
	now      time.Time
	sessions []platform.Session
	sessErr  error
	// what the chain resolved to. The report publishes both, so a fixture that cannot state
	// them leaves both fields untested -- which is how `backends` shipped as null.
	names   []string
	unknown []string
}

func withStatusMachine(t *testing.T, f statusFixture) string {
	t.Helper()

	prevNow := nowFunc
	nowFunc = func() time.Time { return f.now }
	t.Cleanup(func() { nowFunc = prevNow })

	prevChain := buildStatusChain
	buildStatusChain = func(config.Settings) (statusCommand, error) {
		return &fakeSessions{sessions: f.sessions, err: f.sessErr,
			names: f.names, unknown: f.unknown}, nil
	}
	t.Cleanup(func() { buildStatusChain = prevChain })

	return f.writeConfig(t)
}

type fakeSessions struct {
	sessions []platform.Session
	err      error
	// names and unknown are what the chain resolved to. The report publishes both, so the
	// fake has to be able to state them -- otherwise the fields stay untested and a null
	// `backends` in the JSON is invisible to the suite that exists to catch exactly that.
	names   []string
	unknown []string
}

func (f *fakeSessions) Sessions(context.Context) ([]platform.Session, error) {
	return f.sessions, f.err
}

func (f *fakeSessions) Names() []string   { return f.names }
func (f *fakeSessions) Unknown() []string { return f.unknown }

// statusConfig is a two-child house: one inside a curfew right now, one free with a window
// later tonight.
//
// The times are written against the fixture clock rather than the wall clock, so the
// fixture says what situation it models instead of depending on when it runs.
func (f statusFixture) writeConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// A curfew that covers the fixture instant: it opened an hour ago and closes in two.
	start := f.now.Add(-time.Hour).In(time.UTC).Format("15:04")
	end := f.now.Add(2 * time.Hour).In(time.UTC).Format("15:04")
	// A window that opens in three hours and closes at midnight.
	next := f.now.Add(3 * time.Hour).In(time.UTC).Format("15:04")

	body := fmt.Sprintf(`{
  "version": 1,
  "settings": {"enforcement": true, "pollSeconds": 30, "lockBackends": ["logind"]},
  "profiles": [
    {"id": "school-night", "name": "School night", "timezone": "UTC", "rules": [
      {"id": "curfew", "name": "Weeknight curfew", "days": ["mon","tue","wed","thu","fri","sat","sun"],
       "start": %q, "end": %q, "enabled": true},
      {"id": "late", "name": "Late window", "days": ["mon","tue","wed","thu","fri","sat","sun"],
       "start": %q, "end": "23:59", "enabled": true}
    ]},
    {"id": "broken", "name": "Broken", "timezone": "Mars/City", "rules": []}
  ],
  "users": [
    {"username": "casey", "profileID": "school-night", "enabled": true},
    {"username": "alex", "profileID": "school-night", "enabled": true},
    {"username": "robin", "profileID": "missing-profile", "enabled": true},
    {"username": "quiet", "profileID": "school-night", "enabled": false}
  ],
  "overrides": [
    {"username": "alex", "until": %q, "reason": "film night", "by": "parent"}
  ]
}`, start, end, next, f.now.Add(30*time.Minute).UTC().Format(time.RFC3339))

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	// The mtime is a reported field, so the fixture pins it rather than letting the
	// assertion tolerate whatever the filesystem happened to record.
	stamp := f.now.Add(-90 * time.Second)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	return path
}

// statusFixtureFor builds a fixture at a fixed instant with the two children landing where
// the report needs them.
func statusFixtureFor(t *testing.T) statusFixture {
	t.Helper()
	// A Wednesday afternoon. The day is chosen so the every-day rule above covers it and
	// the instant is not near a midnight boundary, where a two-hour arithmetic slip would
	// silently move the fixture into a different window.
	now := time.Date(2026, 3, 4, 15, 0, 0, 0, time.UTC)
	return statusFixture{
		now: now,
		sessions: []platform.Session{
			{ID: "2", Username: "casey", UID: 1000, Type: "x11", Seat: "seat0", State: "active", Class: "user"},
			{ID: "c1", Username: "casey", UID: 1000, Type: "manager", State: "active", Class: "manager"},
		},
	}
}

func TestStatusJSONNamesEveryFieldTheDocumentPromises(t *testing.T) {
	f := statusFixtureFor(t)
	path := withStatusMachine(t, f)

	code, out, errOut := runCmd(t, "status", "--config", path, "--json")
	if code != exitOK {
		t.Fatalf("exit %d, want %d\nstderr:\n%s", code, exitOK, errOut)
	}

	// Asserted field by field, against a decoded map rather than a struct, because the
	// contract is the KEY SET. A Go-side rename that leaves the json tag alone is fine;
	// a tag change that renames a key a script reads is a breaking change, and only the
	// map catches it.
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	for _, key := range []string{
		"enforcement", "configPath", "configModifiedAt", "pollSeconds",
		"backends", "users", "sessions",
	} {
		if _, ok := doc[key]; !ok {
			t.Errorf("--json document has no %q field: %s", key, out)
		}
	}
	if got := string(doc["enforcement"]); got != "true" {
		t.Errorf("enforcement = %s, want true", got)
	}
	if got := string(doc["pollSeconds"]); got != "30" {
		t.Errorf("pollSeconds = %s, want 30", got)
	}
	if got := string(doc["configPath"]); !strings.Contains(got, "config.json") {
		t.Errorf("configPath = %s", got)
	}
	// The mtime is what an operator uses to ask "did my edit land", so it has to be the
	// file's own time and not the moment the command ran.
	want := f.now.Add(-90 * time.Second).UTC().Format(time.RFC3339)
	if got := string(doc["configModifiedAt"]); !strings.Contains(got, want) {
		t.Errorf("configModifiedAt = %s, want the file's mtime %s", got, want)
	}
}

func TestStatusJSONPerUserFieldsAreStableAndComplete(t *testing.T) {
	f := statusFixtureFor(t)
	path := withStatusMachine(t, f)

	_, out, _ := runCmd(t, "status", "--config", path, "--json")
	var doc struct {
		Users []map[string]any `json:"users"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	byName := map[string]map[string]any{}
	for _, u := range doc.Users {
		byName[u["username"].(string)] = u
	}
	if len(byName) != 4 {
		t.Fatalf("got %d users, want 4: %s", len(byName), out)
	}

	// The child inside the window.
	casey := byName["casey"]
	if casey["lockout"] != true {
		t.Errorf("casey lockout = %v, want true", casey["lockout"])
	}
	if casey["ruleName"] != "Weeknight curfew" {
		t.Errorf("casey ruleName = %v, want the rule that is actually holding the account", casey["ruleName"])
	}
	// EndsAt is the instant a UI must render. Re-deriving it from the rule's clock text is
	// the bug the schedule package's own note warns about, so the report carries it and
	// this pins that it is an instant and not a wall time.
	if ends, _ := casey["endsAt"].(string); ends == "" {
		t.Error("casey has no endsAt while locked out")
	} else if _, err := time.Parse(time.RFC3339, ends); err != nil {
		t.Errorf("casey endsAt %q is not an RFC3339 instant: %v", ends, err)
	}

	// The child with an override.
	alex := byName["alex"]
	if alex["override"] != "film night" {
		t.Errorf("alex override = %v, want the reason the parent typed", alex["override"])
	}

	// The child whose profile does not resolve. "no rules" and "no profile" both read as
	// not locked out, and only one of them is a mistake in the config.
	robin := byName["robin"]
	deg, _ := robin["degraded"].([]any)
	if len(deg) == 0 {
		t.Errorf("robin's unresolvable profile is reported as nothing: %v", robin)
	} else if !strings.Contains(fmt.Sprint(deg[0]), "missing-profile") {
		t.Errorf("robin degraded = %v, want it to name the profile that is missing", deg)
	}

	// The disabled child.
	if byName["quiet"]["enabled"] != false {
		t.Error("a disabled user is reported enabled")
	}
	if byName["quiet"]["lockout"] == true {
		t.Error("a disabled user is reported locked out")
	}
}

func TestStatusJSONSessionsDistinguishALockableSessionFromAManager(t *testing.T) {
	f := statusFixtureFor(t)
	path := withStatusMachine(t, f)

	_, out, _ := runCmd(t, "status", "--config", path, "--json")
	var doc struct {
		Sessions []map[string]any `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(doc.Sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(doc.Sessions))
	}
	// The class=manager row is listed -- status reports what is there, it does not filter
	// the machine -- but it is not something the daemon would lock, and the report has to
	// say which is which or an operator reading "2 sessions" counts wrong.
	var lockable int
	for _, s := range doc.Sessions {
		if s["lockable"] == true {
			lockable++
			if s["session"] != "2" {
				t.Errorf("session %v is the lockable one, want the x11 session", s["session"])
			}
		}
	}
	if lockable != 1 {
		t.Errorf("%d of %d sessions reported lockable, want 1", lockable, len(doc.Sessions))
	}
}

func TestStatusReportsSessionsThatCannotBeSeenWithoutLosingTheSchedule(t *testing.T) {
	// The schedule verdicts come from the config and the clock. They are worth printing
	// even when logind is unreachable, and that combination is itself the finding.
	f := statusFixtureFor(t)
	f.sessErr = errors.New("session bus not reachable")
	path := withStatusMachine(t, f)

	code, out, errOut := runCmd(t, "status", "--config", path, "--json")
	if code != exitOK {
		t.Fatalf("exit %d, want %d\nstderr:\n%s", code, exitOK, errOut)
	}
	if !strings.Contains(out, `"lockout":true`) {
		t.Errorf("the schedule verdicts vanished with the session error:\n%s", out)
	}
	var doc struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, w := range doc.Warnings {
		if strings.Contains(w, "session bus not reachable") {
			found = true
		}
	}
	if !found {
		t.Errorf("the session failure is not in warnings %v", doc.Warnings)
	}
}

func TestStatusHumanReportLeadsWithTheAnswer(t *testing.T) {
	f := statusFixtureFor(t)
	path := withStatusMachine(t, f)

	code, out, _ := runCmd(t, "status", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d, want %d", code, exitOK)
	}
	first := strings.SplitN(strings.TrimLeft(out, "\n"), "\n", 2)[0]
	if !strings.HasPrefix(first, "enforcement") {
		t.Errorf("first line is %q, want the enforcement state: this command is typed by someone"+
			" who wants to know whether the daemon is doing its job", first)
	}
	if !strings.Contains(out, "locked out until") {
		t.Errorf("no lockout line:\n%s", out)
	}
	// The wait is stated in time-from-now as well as an instant, because "until 17:00"
	// makes the reader do arithmetic against a clock they have not looked at.
	if !strings.Contains(out, "from now)") {
		t.Errorf("the lockout end is not also given as a wait:\n%s", out)
	}
}

func TestStatusSaysWhatItCannotReportInsteadOfPrintingItBlank(t *testing.T) {
	// "last action" is in the ticket and has no source: the audit writer is append-only.
	// An empty field teaches the operator that empty means "nothing happened".
	f := statusFixtureFor(t)
	path := withStatusMachine(t, f)

	_, out, _ := runCmd(t, "status", "--config", path)
	if !strings.Contains(out, "last action") {
		t.Errorf("the report is silent about the field it cannot fill:\n%s", out)
	}
	if !strings.Contains(out, "write-only") {
		t.Errorf("it does not say why:\n%s", out)
	}
}

func TestStatusUnreadableConfigIsAFailureNotAnEmptyReport(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "absent.json")

	code, out, errOut := runCmd(t, "status", "--config", missing)
	if code != exitFailure {
		t.Errorf("exit %d, want %d: a config that will not load must not read as an empty"+
			" schedule", code, exitFailure)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing", out)
	}
	if !strings.Contains(errOut, "plock status") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestStatusUnreadableConfigWithJSONPutsTheErrorWhereTheParserLooks(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "absent.json")

	code, out, errOut := runCmd(t, "status", "--config", missing, "--json")
	if code != exitFailure {
		t.Fatalf("exit %d, want %d", code, exitFailure)
	}
	// The error is JSON on stderr and stdout is empty. The reasoning this test used to
	// record -- "a caller parsing stdout sees a success carrying an empty document" -- is
	// answered by the exit code, which is the primary signal and is non-zero, and by the
	// fact that an empty document is at least recognisably absent. The failure mode on the
	// other side is worse and silent: with the error on stdout, `plock status --json 2>&1 |
	// jq` hands jq a JSON document interleaved with a human line, and jq fails on input
	// that looks fine in either stream alone.
	//
	// #11's acceptance criteria also name stderr, and the repo had already drifted on the
	// strength of the argument below: status put JSON on stdout while every other command
	// put text on stderr, so neither was usable as a contract.
	var doc map[string]any
	if err := json.Unmarshal([]byte(errOut), &doc); err != nil {
		t.Fatalf("--json failure is not JSON on stderr: %v\n%q", err, errOut)
	}
	if doc["error"] == nil || doc["error"] == "" {
		t.Errorf("no error field in %v", doc)
	}
	if out != "" {
		t.Errorf("stdout carries a document on a failed run, so a merged 2>&1 stream is two documents: %q", out)
	}
}

func TestStatusRejectsAnUnexpectedArgumentAsAUsageError(t *testing.T) {
	f := statusFixtureFor(t)
	path := withStatusMachine(t, f)

	code, _, errOut := runCmd(t, "status", "--config", path, "extra")
	if code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "unexpected argument") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestStatusHelpIsNotAnError(t *testing.T) {
	code, out, errOut := runCmd(t, "status", "-h")
	if code != exitOK {
		t.Errorf("exit %d, want %d: asking for help is not a typo", code, exitOK)
	}
	if !strings.Contains(out, "usage: plock status") {
		t.Errorf("stdout = %q", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want nothing", errOut)
	}
}

// The command must never write. It is the command an operator runs while the daemon is
// running, and a status command that rewrote the config would race the daemon's own save.
func TestStatusDoesNotWriteTheConfig(t *testing.T) {
	f := statusFixtureFor(t)
	path := withStatusMachine(t, f)

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code, out, errOut := runCmd(t, "status", "--config", path); code != exitOK {
		t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, exitOK, out, errOut)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("status modified the config file")
	}
}
