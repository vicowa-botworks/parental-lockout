package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// The ticket's acceptance criteria for doctor are specific: it must exit non-zero when a
// managed user is in a window with an unlocked session, its --json must name the failing
// check, and it must never lock anything unless --try was passed explicitly. Those three are
// what this file pins.

func TestCheckUnenforced(t *testing.T) {
	in := unenforcedCandidate{Username: "casey", Managed: true, InWindow: true,
		Rule: "School nights", EndsAt: "2026-10-15T06:00:00Z", HasSession: true, Lockable: true}

	cases := []struct {
		name     string
		in       unenforcedCandidate
		wantFail bool
		wantText string
	}{
		{
			name:     "in a window, unlocked",
			in:       in,
			wantFail: true,
			wantText: "inside a window and the screen is unlocked",
		},
		{
			// The negative that makes the positive mean anything.
			name: "in a window, locked",
			in: func() unenforcedCandidate {
				c := in
				c.Locked = true
				return c
			}(),
		},
		{
			name: "not in a window",
			in: func() unenforcedCandidate {
				c := in
				c.InWindow = false
				return c
			}(),
		},
		{
			// Disabled on purpose is not the bug someone is looking for, and saying so is how
			// they stop looking.
			name: "disabled on purpose",
			in: func() unenforcedCandidate {
				c := in
				c.Managed = false
				return c
			}(),
		},
		{
			name: "not logged in",
			in: func() unenforcedCandidate {
				c := in
				c.HasSession = false
				return c
			}(),
		},
		{
			// Still reported. A session that cannot be locked while a window is open is the
			// child being unenforced, whatever the reason is.
			name: "in a window, session not lockable",
			in: func() unenforcedCandidate {
				c := in
				c.Lockable = false
				return c
			}(),
			wantFail: true,
			wantText: "not one this build can lock",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkUnenforced(time.Now(), []unenforcedCandidate{tc.in})
			if tc.wantFail {
				if got.Status != verdictFail {
					t.Fatalf("status = %s, want fail: %s", got.Status, got.Detail)
				}
				if !strings.Contains(got.Detail, tc.wantText) {
					t.Errorf("detail = %q, want it to say %q", got.Detail, tc.wantText)
				}
				if got.Fix == "" {
					t.Error("a failure with no fix leaves the operator at a terminal with nothing to do")
				}
				return
			}
			if got.Status == verdictFail {
				t.Errorf("status = fail, want it not to fail: %s", got.Detail)
			}
		})
	}
}

func TestCheckUnenforcedNamesEveryAffectedAccount(t *testing.T) {
	// One account is an incident; five is a misconfiguration, and the operator needs the list
	// to tell which. Sorted, so the output is stable enough to diff.
	mk := func(u string) unenforcedCandidate {
		return unenforcedCandidate{Username: u, Managed: true, InWindow: true,
			HasSession: true, Lockable: true}
	}
	got := checkUnenforced(time.Now(), []unenforcedCandidate{mk("zoe"), mk("casey"), mk("alex")})
	if got.Status != verdictFail {
		t.Fatalf("status = %s", got.Status)
	}
	// Sorted, so two runs of doctor on the same machine produce output that can be diffed.
	for _, want := range []string{"3 account(s)", "alex", "casey", "zoe"} {
		if !strings.Contains(got.Detail, want) {
			t.Errorf("detail = %q, want it to contain %q", got.Detail, want)
		}
	}
	if i := strings.Index(got.Detail, "alex"); i > strings.Index(got.Detail, "zoe") {
		t.Errorf("the accounts are not sorted, so the report cannot be diffed between runs:\n%s", got.Detail)
	}
}

func TestCheckUnenforcedReportsADanglingProfileAsNeverLocked(t *testing.T) {
	// An enabled account whose profileID does not resolve is locked out by nothing, forever.
	// That is the same outcome as disabling the account, reached by accident, and it has to be
	// louder than a momentary unlocked screen because no later tick will fix it.
	got := checkUnenforced(time.Now(), []unenforcedCandidate{
		{Username: "casey", Managed: true, DanglingProfile: "school"},
	})
	if got.Status != verdictFail {
		t.Fatalf("status = %s, want fail: %s", got.Status, got.Detail)
	}
	if !strings.Contains(got.Detail, "never locked out") {
		t.Errorf("detail does not state the consequence: %q", got.Detail)
	}
	if !strings.Contains(got.Fix, "plock user assign") {
		t.Errorf("fix does not name the command: %q", got.Fix)
	}
}

func TestDoctorExitsNonZeroWhenAnAccountIsUnlockedInAWindow(t *testing.T) {
	// The acceptance criterion, end to end through the command rather than the function.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body := `{
  "version": 1,
  "settings": {"enforcement": true, "pollSeconds": 30, "lockBackends": ["logind"]},
  "profiles": [{"id": "school", "name": "School nights", "timezone": "UTC",
    "rules": [{"id": "all", "days": ["mon","tue","wed","thu","fri","sat","sun"],
               "start": "00:00", "end": "23:59", "enabled": true}]}],
  "users": [{"username": "casey", "profileID": "school", "enabled": true}]
}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	withChain(t, &fakeChain{
		names:    []string{"logind"},
		statuses: []platform.BackendStatus{{Name: "logind", Available: true, Detail: "ok"}},
		// A real desktop session, unlocked, while the always-on window is open.
		sessions: []platform.Session{{
			ID: "2", Username: "casey", UID: 1001, Class: "user", Seat: "seat0",
			Type: "wayland", State: "active", Locked: false,
		}},
	}, nil)
	withDaemon(t, "active", true, nil)

	code, out, errOut := runCmd(t, "doctor", "--config", path)
	if code != exitFailure {
		t.Fatalf("exit %d, want %d: an account in a window with an unlocked screen is the\n"+
			"signal the ticket asks doctor to raise\nstdout:\n%s\nstderr:\n%s", code, exitFailure, out, errOut)
	}
	if !strings.Contains(out, "casey") {
		t.Errorf("the report does not name the affected account:\n%s", out)
	}
	if !strings.Contains(out, "unenforced") {
		t.Errorf("the report does not name the failing check:\n%s", out)
	}
}

func TestDoctorJSONNamesTheFailingCheck(t *testing.T) {
	// The other half of the acceptance criterion: --json names which check failed, so a
	// script can branch on it instead of reading prose.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "settings": {"enforcement": false, "pollSeconds": 30, "lockBackends": ["logind"]},
  "profiles": [], "users": []
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	withChain(t, &fakeChain{names: []string{"logind"},
		statuses: []platform.BackendStatus{{Name: "logind", Available: true}}}, nil)
	withDaemon(t, "inactive", true, nil)

	code, out, errOut := runCmd(t, "doctor", "--config", path, "--json")
	if code != exitFailure {
		t.Fatalf("exit %d, want %d\n%s\n%s", code, exitFailure, out, errOut)
	}

	var rep report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("--json is not the shape the text report claims: %v\n%s", err, out)
	}
	if !strings.Contains(strings.Join(rep.Failed, ","), "daemon") {
		t.Errorf("failed = %q, want it to name the inactive service", rep.Failed)
	}
	// The disabled master switch is a warning, not a failure: turning it off is a thing a
	// parent does on purpose, and a monitoring job should not page over a holiday.
	if !strings.Contains(strings.Join(rep.Warned, ","), "enforcement") {
		t.Errorf("warned = %q, want it to name the master switch", rep.Warned)
	}
	// Every check is present in the array, not only the ones that failed. "We did not look"
	// and "we looked and it was fine" both have to be distinguishable from silence.
	if len(rep.Checks) < 6 {
		t.Errorf("only %d checks in the report:\n%s", len(rep.Checks), out)
	}
	for _, c := range rep.Checks {
		if c.Name == "" || c.Status == "" {
			t.Errorf("a check has no name or status: %+v", c)
		}
		if c.Status == verdictFail && c.Fix == "" {
			t.Errorf("check %q failed without saying what to do about it", c.Name)
		}
	}
	if rep.GeneratedAt.IsZero() {
		t.Error("generatedAt is missing, so a saved report cannot be recognised as stale")
	}
	// stderr still carries nothing but errors on a successful run of the checks themselves.
	if strings.Contains(errOut, "no account") {
		t.Errorf("--json wrote report content to stderr: %s", errOut)
	}
}

func TestDoctorJSONAndTextAgreeOnWhatFailed(t *testing.T) {
	// The two renderings are separate code paths over the same report. A parent reading one
	// and an operator scripting the other are looking at the same incident, so the two cannot
	// be allowed to disagree about what failed.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"settings":{"pollSeconds":30}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	withChain(t, &fakeChain{names: []string{"logind"},
		statuses: []platform.BackendStatus{{Name: "logind", Available: false, Detail: "no loginctl"}}}, nil)
	withDaemon(t, "failed", true, nil)

	jsonCode, jsonOut, _ := runCmd(t, "doctor", "--config", path, "--json")
	textCode, textOut, _ := runCmd(t, "doctor", "--config", path)
	if jsonCode != textCode {
		t.Errorf("exit codes differ: json %d, text %d", jsonCode, textCode)
	}
	var rep report
	if err := json.Unmarshal([]byte(jsonOut), &rep); err != nil {
		t.Fatal(err)
	}
	for _, name := range rep.Failed {
		if !strings.Contains(textOut, "fail") || !strings.Contains(textOut, name) {
			t.Errorf("the text report does not show the failure %q that --json reports:\n%s", name, textOut)
		}
	}
}

func TestDoctorNeverLocksWithoutTry(t *testing.T) {
	// The safety criterion. Doctor is run to find out what is wrong on a machine someone is
	// using, and a diagnostic that locked the screen as a side effect of being run could not
	// be run at all.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"settings":{"pollSeconds":30}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// The chain has to have a lockable session in it. An empty fake makes "locked nothing"
	// true no matter what the command does, and a safety assertion that cannot fail is worse
	// than none because it reads as coverage.
	fake := &countingChain{sessions: []platform.Session{{
		ID: "2", Username: "casey", Class: "user", Seat: "seat0", Type: "wayland", State: "active",
	}}}
	withChain(t, fake, nil)
	withDaemon(t, "active", true, nil)

	if code, out, errOut := runCmd(t, "doctor", "--config", path); code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	if fake.locks != 0 {
		t.Errorf("plain `plock doctor` locked %d session(s); it reads, it does not lock", fake.locks)
	}
	// And the control that makes the assertion above mean anything: this fake *would* have
	// recorded a lock had one been attempted.
	if err := fake.Lock(t.Context(), fake.sessions[0]); err != nil {
		t.Fatalf("the fake is not counting: %v", err)
	}
	if fake.locks != 1 {
		t.Errorf("the fake does not count locks, so the assertion above proves nothing")
	}
}

func TestDoctorTryLocksAndSaysWhichBackendAnswered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"settings":{"pollSeconds":30}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &countingChain{sessions: []platform.Session{{
		ID: "2", Username: "casey", Class: "user", Seat: "seat0", Type: "wayland", State: "active",
	}}}
	withChain(t, fake, nil)
	withDaemon(t, "active", true, nil)

	code, out, errOut := runCmd(t, "doctor", "--config", path, "--try", "casey")
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	if fake.locks != 1 {
		t.Errorf("--try locked %d sessions, want 1", fake.locks)
	}
	if !strings.Contains(out, "locked") {
		t.Errorf("the success is not reported:\n%s", out)
	}
}

func TestDoctorTryOnAnUnknownAccountLocksNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"settings":{"pollSeconds":30}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &countingChain{sessions: []platform.Session{{
		ID: "2", Username: "casey", Class: "user", Seat: "seat0", Type: "wayland",
	}}}
	withChain(t, fake, nil)
	withDaemon(t, "active", true, nil)

	code, _, errOut := runCmd(t, "doctor", "--config", path, "--try", "typo")
	if code != exitFailure {
		t.Errorf("exit %d, want %d", code, exitFailure)
	}
	if fake.locks != 0 {
		t.Errorf("a typo'd account locked %d session(s)", fake.locks)
	}
	// The message has to say nothing was locked. "no session found" alone leaves the operator
	// wondering whether something else went ahead.
	if !strings.Contains(errOut, "nothing was locked") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestDoctorTryReportsUnconfirmedSeparatelyFromUnreachable(t *testing.T) {
	// The whole reason --try exists. "Every backend was asked and none confirmed" is the
	// Plasma case -- the commands run, the desktop ignores them, nothing reports it -- and it
	// is a different sentence from "nothing was reached at all".
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"settings":{"pollSeconds":30}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &countingChain{
		sessions: []platform.Session{{
			ID: "2", Username: "casey", Class: "user", Seat: "seat0", Type: "wayland",
		}},
		lockErr: platform.ErrUnconfirmed,
	}
	withChain(t, fake, nil)
	withDaemon(t, "active", true, nil)

	code, out, errOut := runCmd(t, "doctor", "--config", path, "--try", "casey")
	if code != exitFailure {
		t.Fatalf("exit %d, want %d\n%s", code, exitFailure, out)
	}
	combined := out + errOut
	if !strings.Contains(combined, "daemon is running and the screen never locks") {
		t.Errorf("the unconfirmed case is not explained as the incident it is:\n%s", combined)
	}

	// And a hard error must not borrow that sentence.
	fake.lockErr = errors.New("loginctl: exit status 1")
	code, out, errOut = runCmd(t, "doctor", "--config", path, "--try", "casey")
	if code != exitFailure {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(out+errOut, "daemon is running and the screen never locks") {
		t.Errorf("a hard failure was reported as the unconfirmed case:\n%s%s", out, errOut)
	}
}

func TestDoctorTryRefusesToRunUnderJSON(t *testing.T) {
	// --json is what a monitoring job reads. Letting it also lock a real screen means a health
	// check could lock a child's screen on a schedule.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"settings":{"pollSeconds":30}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &countingChain{}
	withChain(t, fake, nil)
	withDaemon(t, "active", true, nil)

	code, _, errOut := runCmd(t, "doctor", "--config", path, "--json", "--try", "casey")
	if code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "--try") {
		t.Errorf("the refusal does not name the flag: %q", errOut)
	}
	if fake.locks != 0 {
		t.Errorf("the combination locked %d session(s) before refusing", fake.locks)
	}
}

// countingChain records lock calls so "never locks without --try" is an assertion rather than
// an absence of evidence.
type countingChain struct {
	sessions    []platform.Session
	locks       int
	lockErr     error
	sessionsErr error
}

func (c *countingChain) Names() []string   { return []string{"logind"} }
func (c *countingChain) Unknown() []string { return nil }
func (c *countingChain) Probe(context.Context) []platform.BackendStatus {
	return []platform.BackendStatus{{Name: "logind", Available: true, Detail: "ok"}}
}
func (c *countingChain) Sessions(context.Context) ([]platform.Session, error) {
	return c.sessions, c.sessionsErr
}
func (c *countingChain) Lock(context.Context, platform.Session) error {
	c.locks++
	return c.lockErr
}
