package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// unlockConfig is a config with one managed user, written to a fresh directory so the
// command's own writes can be inspected afterwards.
func unlockConfig(t *testing.T, users ...string) string {
	t.Helper()
	if len(users) == 0 {
		users = []string{"casey"}
	}
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

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// withUnlockClock pins the clock so "until" is a fact the test states rather than a
// number both sides compute from time.Now.
func withUnlockClock(t *testing.T, now time.Time) {
	t.Helper()
	prev := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = prev })
}

func TestUnlockWritesTheOverrideTheOperatorAskedFor(t *testing.T) {
	now := time.Date(2026, 3, 4, 15, 0, 0, 0, time.UTC)
	withUnlockClock(t, now)
	path := unlockConfig(t)

	code, out, errOut := runCmd(t, "unlock", "casey", "--minutes", "45",
		"--reason", "film night", "--by", "pat", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, exitOK, out, errOut)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Overrides) != 1 {
		t.Fatalf("%d overrides, want 1", len(cfg.Overrides))
	}
	o := cfg.Overrides[0]
	if o.Username != "casey" {
		t.Errorf("username = %q", o.Username)
	}
	if !o.Until.Equal(now.Add(45 * time.Minute)) {
		t.Errorf("until = %v, want %v", o.Until, now.Add(45*time.Minute))
	}
	if o.Reason != "film night" || o.By != "pat" {
		t.Errorf("reason/by = %q/%q, want the reason and the name given", o.Reason, o.By)
	}
	// The confirmation has to name the account and the instant, because the operator is
	// deciding whether that is the right time, and silence after a write is
	// indistinguishable from a write that went to a different --config.
	for _, want := range []string{"casey", "film night", "pat"} {
		if !strings.Contains(out, want) {
			t.Errorf("confirmation does not echo %q:\n%s", want, out)
		}
	}
}

func TestUnlockReplacesAnExistingOverrideRatherThanStacking(t *testing.T) {
	now := time.Date(2026, 3, 4, 15, 0, 0, 0, time.UTC)
	withUnlockClock(t, now)
	path := unlockConfig(t)

	for _, mins := range []string{"10", "90"} {
		code, out, errOut := runCmd(t, "unlock", "casey", "--minutes", mins,
			"--reason", "changed my mind", "--by", "pat", "--config", path)
		if code != exitOK {
			t.Fatalf("minutes=%s exit %d\n%s\n%s", mins, code, out, errOut)
		}
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	// Two grants for one child is not "the longer one wins" — ActiveOverride returns the
	// first match, so a stale 10-minute entry ahead of a fresh 90-minute one would release
	// the account early while the file appeared to hold the later time.
	if len(cfg.Overrides) != 1 {
		t.Fatalf("%d overrides for one user, want 1: %v", len(cfg.Overrides), cfg.Overrides)
	}
	if !cfg.Overrides[0].Until.Equal(now.Add(90 * time.Minute)) {
		t.Errorf("until = %v, want the second grant", cfg.Overrides[0].Until)
	}
}

func TestUnlockRefusesEveryShapeOfMissingMinutes(t *testing.T) {
	now := time.Date(2026, 3, 4, 15, 0, 0, 0, time.UTC)
	withUnlockClock(t, now)

	for _, tc := range []struct {
		args []string
		want string
		why  string
	}{
		{[]string{}, "--minutes is required", "an override with no length has no end"},
		{[]string{"--minutes", "0"}, "at least 1", "zero minutes is a way of writing nothing"},
		{[]string{"--minutes", "-5"}, "at least 1", "a negative grant is not a thing"},
		{[]string{"--minutes", "1441"}, "more than a day", "a typo of 14400 must not become ten days of no enforcement"},
	} {
		path := unlockConfig(t)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		args := append([]string{"casey", "--reason", "x", "--config", path}, tc.args...)
		code, out, errOut := runCmd(t, append([]string{"unlock"}, args...)...)
		if code != exitUsage {
			t.Errorf("%v: exit %d, want %d (%s)\nstderr: %s", tc.args, code, exitUsage, tc.why, errOut)
		}
		if out != "" {
			t.Errorf("%v: stdout = %q, want nothing", tc.args, out)
		}
		if !strings.Contains(errOut, tc.want) {
			t.Errorf("%v: stderr = %q, want it to contain %q", tc.args, errOut, tc.want)
		}
		// A refused command must not have touched the file. The refusal happens before the
		// config is opened, so this is also the assertion that it happens before it.
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Errorf("%v modified the config despite refusing", tc.args)
		}
	}
}

func TestUnlockRefusesABlankReasonIncludingWhitespace(t *testing.T) {
	// `--reason " "` is what a shell produces when the operator means to type a reason and
	// does not. An override is the one config entry that REMOVES enforcement, and the
	// reason is the only thing that will explain it to anyone reading the file later.
	for _, reason := range []string{"", "   ", "\t"} {
		path := unlockConfig(t)
		code, _, errOut := runCmd(t, "unlock", "casey", "--minutes", "30",
			"--reason", reason, "--config", path)
		if code != exitUsage {
			t.Errorf("reason %q: exit %d, want %d", reason, code, exitUsage)
		}
		if !strings.Contains(errOut, "--reason is required") {
			t.Errorf("reason %q: stderr = %q", reason, errOut)
		}
		cfg, err := config.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Overrides) != 0 {
			t.Errorf("reason %q wrote an override anyway", reason)
		}
	}
}

func TestUnlockRefusesAUserThatIsNotManaged(t *testing.T) {
	// An override for a name that is not managed is not harmless. It sits in the file, it
	// is granted by someone who believed they were unlocking a child, and if that username
	// is created next week it is already active with a stale reason attached.
	now := time.Date(2026, 3, 4, 15, 0, 0, 0, time.UTC)
	withUnlockClock(t, now)
	path := unlockConfig(t)

	code, _, errOut := runCmd(t, "unlock", "nobody", "--minutes", "30",
		"--reason", "wrong box", "--config", path)
	if code != exitFailure {
		t.Errorf("exit %d, want %d", code, exitFailure)
	}
	if !strings.Contains(errOut, `no managed user "nobody"`) {
		t.Errorf("stderr = %q", errOut)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Overrides) != 0 {
		t.Errorf("an override was written for an unmanaged user: %v", cfg.Overrides)
	}
}

func TestUnlockNeedsExactlyOneUsername(t *testing.T) {
	path := unlockConfig(t)
	for _, tc := range []struct {
		args []string
		why  string
	}{
		{[]string{"--minutes", "5", "--reason", "x"}, "a username is required"},
		// Two usernames names an ambiguous target, and the message says which token it
		// is blaming and offers the other reason a line like this happens: an unquoted
		// --reason film night arrives as a second positional.
		{[]string{"a", "b", "--minutes", "5", "--reason", "x"}, `unexpected argument "b"`},
	} {
		code, _, errOut := runCmd(t, append([]string{"unlock", "--config", path}, tc.args...)...)
		if code != exitUsage {
			t.Errorf("%v: exit %d, want %d", tc.args, code, exitUsage)
		}
		if !strings.Contains(errOut, tc.why) {
			t.Errorf("%v: stderr = %q, want %q", tc.args, errOut, tc.why)
		}
	}
}

func TestUnlockKeepsTheConfigPrivate(t *testing.T) {
	// The override is written through config.Save, which writes 0600 atomically. This is
	// asserted here rather than trusted because the CLI is a NEW caller of that path, and
	// "the file used to stay private" is not the same claim as "every writer keeps it
	// private".
	now := time.Date(2026, 3, 4, 15, 0, 0, 0, time.UTC)
	withUnlockClock(t, now)
	path := unlockConfig(t)

	code, out, errOut := runCmd(t, "unlock", "casey", "--minutes", "30",
		"--reason", "dinner", "--by", "pat", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("config mode is %o after an unlock, want 600", info.Mode().Perm())
	}
	// The atomic write leaves a temp file beside the target if it dies mid-write; a
	// successful one must not leave one behind for the next Load to find.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("%d files in the config directory after a clean save, want 1: %v", len(entries), names)
	}
}

func TestOverrideListShowsAnExpiredGrantAndSaysSo(t *testing.T) {
	now := time.Date(2026, 3, 4, 15, 0, 0, 0, time.UTC)
	withUnlockClock(t, now)
	path := unlockConfig(t)

	if code, out, errOut := runCmd(t, "unlock", "casey", "--minutes", "60",
		"--reason", "active one", "--by", "pat", "--config", path); code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	// The grant is aged out by moving the clock forward rather than by writing a second
	// config with a past `until`: that would test the reader's tolerance for a value the
	// writer would never produce.
	withUnlockClock(t, now.Add(2*time.Hour))

	code, out, errOut := runCmd(t, "override", "list", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, exitOK, out, errOut)
	}
	// "why is this account free" is a question about the file, not only about right now,
	// so an expired override is listed and labelled rather than hidden.
	if !strings.Contains(out, "expired") {
		t.Errorf("an expired override is not labelled:\n%s", out)
	}
	if !strings.Contains(out, "active one") {
		t.Errorf("the reason is not shown:\n%s", out)
	}
}

func TestOverrideListJSONCarriesActiveAsABool(t *testing.T) {
	now := time.Date(2026, 3, 4, 15, 0, 0, 0, time.UTC)
	withUnlockClock(t, now)
	path := unlockConfig(t)

	if code, _, errOut := runCmd(t, "unlock", "casey", "--minutes", "60",
		"--reason", "film", "--by", "pat", "--config", path); code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}

	code, out, errOut := runCmd(t, "override", "list", "--config", path, "--json")
	if code != exitOK {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	var doc struct {
		Overrides []map[string]any `json:"overrides"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(doc.Overrides) != 1 {
		t.Fatalf("%d overrides: %s", len(doc.Overrides), out)
	}
	if active, ok := doc.Overrides[0]["active"].(bool); !ok || !active {
		t.Errorf("active = %#v, want the JSON boolean true", doc.Overrides[0]["active"])
	}
	for _, key := range []string{"username", "until", "reason", "by", "active"} {
		if _, ok := doc.Overrides[0][key]; !ok {
			t.Errorf("no %q field in %v", key, doc.Overrides[0])
		}
	}
}

func TestOverrideClearRemovesItAndSaysWhichAccount(t *testing.T) {
	now := time.Date(2026, 3, 4, 15, 0, 0, 0, time.UTC)
	withUnlockClock(t, now)
	path := unlockConfig(t)

	if code, _, errOut := runCmd(t, "unlock", "casey", "--minutes", "60",
		"--reason", "film", "--by", "pat", "--config", path); code != exitOK {
		t.Fatalf("setup: %s", errOut)
	}

	code, out, errOut := runCmd(t, "override", "clear", "casey", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, exitOK, out, errOut)
	}
	if !strings.Contains(out, "casey") {
		t.Errorf("the confirmation does not name the account:\n%s", out)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Overrides) != 0 {
		t.Errorf("override survived the clear: %v", cfg.Overrides)
	}
}

func TestOverrideClearOnNothingIsNotAFailure(t *testing.T) {
	// `plock override clear kid` in a teardown script must not abort the script because the
	// override had already expired on its own. Same end state, so same exit code — and the
	// complaint still goes to stderr, so a human sees that nothing was removed.
	path := unlockConfig(t)
	code, out, errOut := runCmd(t, "override", "clear", "casey", "--config", path)
	if code != exitOK {
		t.Errorf("exit %d, want 0: an already-absent override is the state the caller wanted", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing", out)
	}
	if !strings.Contains(errOut, "no override") {
		t.Errorf("stderr = %q, want it to say nothing was removed", errOut)
	}
}

func TestOverrideRejectsAnUnknownSubcommand(t *testing.T) {
	path := unlockConfig(t)
	code, _, errOut := runCmd(t, "override", "frobnicate", "--config", path)
	if code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "unknown subcommand") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestUnlockHelpIsNotAnError(t *testing.T) {
	code, out, errOut := runCmd(t, "unlock", "-h")
	if code != exitOK {
		t.Errorf("exit %d, want %d", code, exitOK)
	}
	if !strings.Contains(out, "usage: plock unlock") {
		t.Errorf("stdout = %q", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q", errOut)
	}
}
