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

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// fakeLock is a chain that answers each of the three ways, per session id.
type fakeLock struct {
	sessions []platform.Session
	sessErr  error
	// answers maps a session id to what Lock returns for it. A missing key means the
	// backend confirmed the lock.
	answers map[string]error
	locked  []string
}

func (f *fakeLock) Sessions(context.Context) ([]platform.Session, error) {
	return f.sessions, f.sessErr
}

func (f *fakeLock) Lock(_ context.Context, s platform.Session) error {
	f.locked = append(f.locked, s.ID)
	return f.answers[s.ID]
}

func withLockChain(t *testing.T, f *fakeLock) string {
	t.Helper()
	prev := buildLockChain
	buildLockChain = func(config.Settings) (lockChain, error) { return f, nil }
	t.Cleanup(func() { buildLockChain = prev })
	return lockConfig(t, "casey", "alex")
}

// lockConfig writes a config with the named users and no overrides, and returns its path
// so the test can assert afterwards that nothing was written.
func lockConfig(t *testing.T, users ...string) string {
	t.Helper()
	list := make([]string, len(users))
	for i, u := range users {
		list[i] = fmt.Sprintf(`{"username": %q, "profileID": "p", "enabled": true}`, u)
	}
	body := fmt.Sprintf(`{
  "version": 1,
  "settings": {"enforcement": true, "pollSeconds": 30, "lockBackends": ["logind"]},
  "profiles": [{"id": "p", "name": "P", "timezone": "UTC", "rules": []}],
  "users": [%s]
}`, strings.Join(list, ","))
	path := lockConfigDir(t, body)
	return path
}

func lockConfigDir(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func desktopSession(id, user string) platform.Session {
	return platform.Session{
		ID: id, Username: user, UID: 1000, Type: "x11", Seat: "seat0",
		State: "active", Class: "user",
	}
}

func managerSession(id, user string) platform.Session {
	return platform.Session{ID: id, Username: user, UID: 1000, Type: "manager", State: "active", Class: "manager"}
}

func TestLockReportsAConfirmedLock(t *testing.T) {
	f := &fakeLock{sessions: []platform.Session{desktopSession("2", "casey")}}
	path := withLockChain(t, f)

	code, out, errOut := runCmd(t, "lock", "casey", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d, want %d\n%s\n%s", code, exitOK, out, errOut)
	}
	if len(f.locked) != 1 || f.locked[0] != "2" {
		t.Errorf("locked %v, want the one desktop session", f.locked)
	}
	if !strings.Contains(out, "locked") {
		t.Errorf("no locked line:\n%s", out)
	}
}

func TestLockKeepsUnconfirmedApartFromFailure(t *testing.T) {
	// Three outcomes, three sentences. Collapsing "asked and cannot prove" into "failed"
	// would put a permanent red X on a machine whose screen is in fact locked, which trains
	// the operator to ignore the one signal they have.
	f := &fakeLock{
		sessions: []platform.Session{desktopSession("2", "casey")},
		answers:  map[string]error{"2": fmt.Errorf("%w: logind exited 0", platform.ErrUnconfirmed)},
	}
	path := withLockChain(t, f)

	code, out, errOut := runCmd(t, "lock", "casey", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d, want 0: unconfirmed is not an operational failure\n%s\n%s", code, out, errOut)
	}
	if !strings.Contains(out, "unconfirmed") {
		t.Errorf("the word unconfirmed never appears:\n%s", out)
	}
	if strings.Contains(out, "failed") {
		t.Errorf("an unconfirmed lock is also reported as failed:\n%s", out)
	}
	// The explanation has to travel with the verdict, or the operator has to go read
	// whether a raw VT can ever confirm.
	if !strings.Contains(out, "SetLockedHint") {
		t.Errorf("the report does not say why a machine can be locked and unconfirmed:\n%s", out)
	}
}

// TestLockLocksWhatAPartialListingRead is #168 on the command line. The census that read
// four sessions out of five still names the account being asked about, and answering
// `sessions_failed` for the whole command was answering about a session sitting in the same
// slice -- while the one line the operator needs, "the machine was only partly seen", went
// unmentioned.
func TestLockLocksWhatAPartialListingRead(t *testing.T) {
	f := &fakeLock{
		sessions: []platform.Session{desktopSession("2", "casey")},
		sessErr:  errors.New("session 7: loginctl show-session: timed out"),
	}
	path := withLockChain(t, f)

	code, out, errOut := runCmd(t, "lock", "casey", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d, want %d: the session that read was lockable\n%s\n%s", code, exitOK, out, errOut)
	}
	if len(f.locked) != 1 || f.locked[0] != "2" {
		t.Errorf("locked %v, want the session the listing did read", f.locked)
	}
	if !strings.Contains(errOut, "timed out") {
		t.Errorf("the shortfall was never said: %s", errOut)
	}
	// stderr, so --json stays one document on stdout.
	if strings.Contains(out, "timed out") {
		t.Errorf("the shortfall leaked onto the report stream: %s", out)
	}
}

// TestLockDoesNotReportALogoutForASessionItCouldNotRead is the other arm of #168: the account
// whose row is the one that failed must not come back as "has no session on this machine".
// That sentence is a logout, and a census that lost the row has no evidence for it.
func TestLockDoesNotReportALogoutForASessionItCouldNotRead(t *testing.T) {
	f := &fakeLock{
		sessions: []platform.Session{desktopSession("2", "alex")},
		sessErr:  errors.New("session 7: loginctl show-session: timed out"),
	}
	path := withLockChain(t, f)

	code, out, errOut := runCmd(t, "lock", "casey", "--config", path)
	if code == exitOK {
		t.Fatalf("an account the census could not place was reported as handled:\n%s", out)
	}
	if len(f.locked) != 0 {
		t.Errorf("locked %v for an account with no session in the read", f.locked)
	}
	if strings.Contains(errOut, "has no session on this machine") {
		t.Errorf("a lost row was reported as a logout: %s", errOut)
	}
	if !strings.Contains(errOut, "could not read") {
		t.Errorf("the refusal does not say the machine was only partly seen: %s", errOut)
	}
}

// TestLockStillRefusesAListingThatReadNothing keeps the guard from becoming "ignore errors":
// with no rows there is nothing to lock and the command says so.
func TestLockStillRefusesAListingThatReadNothing(t *testing.T) {
	f := &fakeLock{sessErr: errors.New("loginctl: signal: killed")}
	path := withLockChain(t, f)

	code, _, errOut := runCmd(t, "lock", "casey", "--config", path, "--json")
	if code == exitOK {
		t.Fatal("a listing that read nothing exited successfully")
	}
	// cli.Fail writes the failure document to stderr in both modes, which is what keeps
	// stdout a single machine-readable stream on the success path.
	if !strings.Contains(errOut, "sessions_failed") {
		t.Errorf("the failure does not name the session read: %s", errOut)
	}
}

func TestLockReportsAHardFailureAsAFailure(t *testing.T) {
	// Nothing reached the desktop at all. That is a different sentence from unconfirmed and
	// it is the one that exits non-zero.
	f := &fakeLock{
		sessions: []platform.Session{desktopSession("2", "casey")},
		answers:  map[string]error{"2": errors.New("dbus: connection refused")},
	}
	path := withLockChain(t, f)

	code, out, errOut := runCmd(t, "lock", "casey", "--config", path)
	if code != exitFailure {
		t.Errorf("exit %d, want %d\n%s\n%s", code, exitFailure, out, errOut)
	}
	if !strings.Contains(out, "failed") {
		t.Errorf("the failure is not reported:\n%s", out)
	}
}

func TestLockJSONCarriesAllThreeCounts(t *testing.T) {
	f := &fakeLock{
		sessions: []platform.Session{
			desktopSession("2", "casey"),
			desktopSession("3", "casey"),
			managerSession("c1", "casey"),
		},
		answers: map[string]error{"3": fmt.Errorf("%w", platform.ErrUnconfirmed)},
	}
	path := withLockChain(t, f)

	code, out, errOut := runCmd(t, "lock", "casey", "--config", path, "--json")
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	var doc struct {
		Locked      int `json:"locked"`
		Unconfirmed int `json:"unconfirmed"`
		Refused     int `json:"refused"`
		Sessions    []struct {
			Session     string `json:"session"`
			Locked      bool   `json:"locked"`
			Unconfirmed bool   `json:"unconfirmed"`
			Detail      string `json:"detail"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if doc.Locked != 1 || doc.Unconfirmed != 1 || doc.Refused != 1 {
		t.Errorf("counts = %d/%d/%d, want 1 locked, 1 unconfirmed, 1 refused: %s",
			doc.Locked, doc.Unconfirmed, doc.Refused, out)
	}
	// The manager session is counted and explained, not silently skipped: "locked 0 of 1"
	// with no reason reads as a broken backend.
	var explained bool
	for _, s := range doc.Sessions {
		if s.Session == "c1" && strings.Contains(s.Detail, "not a lockable") {
			explained = true
		}
	}
	if !explained {
		t.Errorf("the non-lockable session is not explained: %+v", doc.Sessions)
	}
}

func TestLockDoesNotTouchASessionItCannotLock(t *testing.T) {
	// The chain's own guard is the last line of defence; the command must not even ask.
	f := &fakeLock{sessions: []platform.Session{managerSession("c1", "casey")}}
	path := withLockChain(t, f)

	code, out, errOut := runCmd(t, "lock", "casey", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d, want 0: there was nothing to lock, which is not a failure\n%s\n%s",
			code, out, errOut)
	}
	if len(f.locked) != 0 {
		t.Errorf("Lock was called on %v, which is not a lockable session", f.locked)
	}
}

func TestLockAllLocksEveryUserAndNamesNobody(t *testing.T) {
	f := &fakeLock{sessions: []platform.Session{
		desktopSession("2", "casey"), desktopSession("4", "alex"),
	}}
	path := withLockChain(t, f)

	code, out, errOut := runCmd(t, "lock", "--all", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	if len(f.locked) != 2 {
		t.Errorf("locked %v, want both sessions", f.locked)
	}
	if !strings.Contains(out, "all users") {
		t.Errorf("the report does not say it was asked for everybody:\n%s", out)
	}
}

func TestLockRefusesANameAndAllTogether(t *testing.T) {
	// Two different questions, and answering both silently would lock accounts the
	// operator did not name.
	f := &fakeLock{sessions: []platform.Session{desktopSession("2", "casey"), desktopSession("4", "alex")}}
	path := withLockChain(t, f)

	code, _, errOut := runCmd(t, "lock", "casey", "--all", "--config", path)
	if code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "not both") {
		t.Errorf("stderr = %q", errOut)
	}
	if len(f.locked) != 0 {
		t.Errorf("it locked %v anyway", f.locked)
	}
}

func TestLockTellsAnUnknownUserApartFromAChildWhoIsLoggedOut(t *testing.T) {
	f := &fakeLock{sessions: []platform.Session{desktopSession("2", "casey")}}
	path := withLockChain(t, f)

	// Not a managed user at all: the name is wrong.
	code, _, errOut := runCmd(t, "lock", "nobody", "--config", path)
	if code != exitFailure {
		t.Errorf("exit %d, want %d", code, exitFailure)
	}
	if !strings.Contains(errOut, `no managed user "nobody"`) {
		t.Errorf("stderr = %q", errOut)
	}

	// A managed user with no session: the name is right and the child is not at a keyboard.
	// Both look like "nothing to lock", and the operator needs to know which.
	code, _, errOut = runCmd(t, "lock", "alex", "--config", path)
	if code != exitFailure {
		t.Errorf("exit %d, want %d", code, exitFailure)
	}
	if !strings.Contains(errOut, `"alex" has no session`) {
		t.Errorf("stderr = %q, want it to say the account is real and simply not logged in", errOut)
	}
}

func TestLockWritesNothingToTheConfig(t *testing.T) {
	// `plock lock` is one action, not a change of policy. If it wrote, it would race the
	// daemon's own save and would leave the account free once the window closed.
	f := &fakeLock{sessions: []platform.Session{desktopSession("2", "casey")}}
	path := withLockChain(t, f)

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code, out, errOut := runCmd(t, "lock", "casey", "--config", path); code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("lock modified the config")
	}
}

func TestLockReportsAConfigItCannotRead(t *testing.T) {
	f := &fakeLock{}
	prev := buildLockChain
	buildLockChain = func(config.Settings) (lockChain, error) { return f, nil }
	t.Cleanup(func() { buildLockChain = prev })

	code, _, errOut := runCmd(t, "lock", "casey", "--config", filepath.Join(t.TempDir(), "absent.json"))
	if code != exitFailure {
		t.Errorf("exit %d, want %d", code, exitFailure)
	}
	if !strings.Contains(errOut, "plock lock") {
		t.Errorf("stderr = %q", errOut)
	}
	if len(f.locked) != 0 {
		t.Error("it locked something without a config")
	}
}

func TestLockNeedsAUsername(t *testing.T) {
	f := &fakeLock{}
	path := withLockChain(t, f)
	code, _, errOut := runCmd(t, "lock", "--config", path)
	if code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "username is required") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestLockHelpIsNotAnError(t *testing.T) {
	code, out, errOut := runCmd(t, "lock", "-h")
	if code != exitOK {
		t.Errorf("exit %d, want %d", code, exitOK)
	}
	if !strings.Contains(out, "usage: plock lock") {
		t.Errorf("stdout = %q", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q", errOut)
	}
}
