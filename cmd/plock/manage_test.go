package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// seed writes a config with one profile and one user and returns its path.
func seed(t *testing.T) string {
	t.Helper()
	return writeCfg(t, `{
  "version": 1,
  "settings": {"enforcement": true, "pollSeconds": 30, "lockBackends": ["logind"]},
  "profiles": [
    {"id": "school", "name": "School nights", "timezone": "Europe/London",
     "rules": [{"id": "week", "name": "Week", "days": ["mon","tue","wed","thu","fri"],
                "start": "20:00", "end": "06:00", "enabled": true}]}
  ],
  "users": [{"username": "casey", "profileID": "school", "enabled": true}]
}`)
}

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// readCfg decodes what the command actually left on disk. Asserting on the file rather
// than on stdout is the only way to know the command wrote what it printed.
func readCfg(t *testing.T, path string) *config.Config {
	t.Helper()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("the command left an unreadable config: %v", err)
	}
	return cfg
}

func TestProfileListCountsAssignments(t *testing.T) {
	path := seed(t)
	code, out, errOut := runCmd(t, "profile", "list", "--config", path, "--json")
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	var doc struct {
		Profiles []struct {
			ID       string `json:"id"`
			Rules    int    `json:"rules"`
			Assigned int    `json:"assigned"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(doc.Profiles) != 1 {
		t.Fatalf("profiles = %d, want 1: %s", len(doc.Profiles), out)
	}
	// The assignment count is on the listing because it is the number that decides whether
	// `profile rm` can succeed. Learning it by attempting the removal is the wrong order.
	if doc.Profiles[0].Rules != 1 || doc.Profiles[0].Assigned != 1 {
		t.Errorf("rules/assigned = %d/%d, want 1/1", doc.Profiles[0].Rules, doc.Profiles[0].Assigned)
	}
}

func TestProfileShowRendersTheWindowAndTheAllDayCase(t *testing.T) {
	path := seed(t)
	code, out, errOut := runCmd(t, "profile", "show", "school", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	for _, want := range []string{"Europe/London", "20:00", "06:00", "week"} {
		if !strings.Contains(out, want) {
			t.Errorf("show omits %s:\n%s", want, out)
		}
	}

	// A profile with no rules never locks anybody out, and the listing has to say so rather
	// than showing an empty section someone reads as "not yet filled in".
	empty := writeCfg(t, strings.Replace(`{
  "version": 1,
  "settings": {"enforcement": true, "pollSeconds": 30, "lockBackends": ["logind"]},
  "profiles": [{"id": "school", "name": "S", "timezone": "UTC", "rules": []}],
  "users": []
}`, "school", "bare", 1))
	code, out, _ = runCmd(t, "profile", "show", "bare", "--config", empty)
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "never locks anybody out") {
		t.Errorf("an empty profile is not called out:\n%s", out)
	}
}

func TestProfileAddRefusesToOverwrite(t *testing.T) {
	path := seed(t)
	before := readCfg(t, path)

	code, _, errOut := runCmd(t, "profile", "add", "school", "--name", "Rewritten", "--config", path)
	if code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "already exists") {
		t.Errorf("stderr = %q", errOut)
	}
	// Silently replacing a profile replaces every child's schedule.
	if got := readCfg(t, path); len(got.Profiles) != len(before.Profiles) || got.Profiles[0].Name != before.Profiles[0].Name {
		t.Error("the existing profile was changed")
	}
}

func TestProfileAddCopiesRulesAndTheirTimezoneTogether(t *testing.T) {
	path := seed(t)
	code, out, errOut := runCmd(t, "profile", "add", "weekend", "--copy-from", "school", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	cfg := readCfg(t, path)
	var p *config.Profile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ID == "weekend" {
			p = &cfg.Profiles[i]
		}
	}
	if p == nil {
		t.Fatal("no weekend profile was written")
	}
	if len(p.Rules) != 1 {
		t.Fatalf("copied %d rules, want 1", len(p.Rules))
	}
	// A rule is a wall-clock time, so its meaning depends on the zone. Copying the rules but
	// not the zone would shift every window by the offset while the rule text still looked
	// identical to the source.
	if p.Timezone != "Europe/London" {
		t.Errorf("timezone = %q, want the source's Europe/London", p.Timezone)
	}
	if p.Name == "weekend" && p.Rules[0].ID != "week" {
		t.Error("rules did not come across")
	}
}

func TestProfileAddRefusesAZoneTheSystemCannotResolve(t *testing.T) {
	path := seed(t)
	code, _, errOut := runCmd(t, "profile", "add", "x", "--timezone", "Mars/Olympus_Mons", "--config", path)
	if code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "not one this system knows") {
		t.Errorf("stderr = %q", errOut)
	}
	// The evaluator falls back to a default zone and reports itself degraded, which is right
	// for a daemon that must keep enforcing and wrong for a command with a human standing
	// there. A silently-wrong zone moves every window by hours and the schedule still looks
	// correct on screen.
	if _, err := config.Load(path); err != nil {
		t.Fatal(err)
	}
	cfg := readCfg(t, path)
	for _, p := range cfg.Profiles {
		if p.ID == "x" {
			t.Error("the profile was written despite the unusable zone")
		}
	}
}

func TestProfileRmRefusesWhileUsersAreAssigned(t *testing.T) {
	// The safety rule of the whole management surface. A user whose profileID resolves to
	// nothing has no rules, and no rules means no lockout: deleting a profile that children
	// are assigned to releases them silently, on the tick the daemon next reads the file.
	path := seed(t)

	code, _, errOut := runCmd(t, "profile", "rm", "school", "--config", path)
	if code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "casey") {
		t.Errorf("the refusal does not name the affected users: %q", errOut)
	}
	if !strings.Contains(errOut, "no lockout") {
		t.Errorf("the refusal does not say what would happen: %q", errOut)
	}
	cfg := readCfg(t, path)
	if len(cfg.Profiles) != 1 {
		t.Error("the profile was removed anyway")
	}
}

func TestProfileRmSucceedsOnceNobodyIsAssigned(t *testing.T) {
	path := seed(t)
	// The documented way round: stop managing the user, then remove the profile. There is no
	// --force, because the config layer refuses an assignment still pointing at the profile
	// and a flag that promised past that would either lie or silently unmanage a child.
	if code, out, errOut := runCmd(t, "user", "rm", "casey", "--config", path); code != exitOK {
		t.Fatalf("user rm: exit %d\n%s\n%s", code, out, errOut)
	}
	if code, out, errOut := runCmd(t, "profile", "rm", "school", "--config", path); code != exitOK {
		t.Fatalf("profile rm: exit %d\n%s\n%s", code, out, errOut)
	}
	if cfg := readCfg(t, path); len(cfg.Profiles) != 0 {
		t.Errorf("profiles left = %d, want 0", len(cfg.Profiles))
	}
}

func TestProfileRmHasNoForceFlag(t *testing.T) {
	// Pinned so the flag cannot come back. A --force the writer ignores is worse than no
	// flag: the operator believes they overrode something.
	path := seed(t)
	code, _, errOut := runCmd(t, "profile", "rm", "school", "--force", "--config", path)
	if code != exitUsage {
		t.Errorf("exit %d, want %d: --force is not a flag this command has", code, exitUsage)
	}
	if !strings.Contains(errOut, "force") {
		t.Errorf("stderr = %q, want the unknown flag named", errOut)
	}
}

func TestUserAssignRefusesAProfileThatDoesNotExist(t *testing.T) {
	// The same silent release as deleting the profile from under them, and easier to type.
	path := seed(t)
	code, _, errOut := runCmd(t, "user", "assign", "casey", "typoed", "--config", path)
	if code != exitFailure {
		t.Errorf("exit %d, want %d", code, exitFailure)
	}
	if !strings.Contains(errOut, `no profile "typoed"`) {
		t.Errorf("stderr = %q", errOut)
	}
	cfg := readCfg(t, path)
	if cfg.Users[0].ProfileID != "school" {
		t.Errorf("the assignment was changed to %q", cfg.Users[0].ProfileID)
	}
}

func TestUserAssignCreatesAnEnforcedUser(t *testing.T) {
	path := seed(t)
	code, out, errOut := runCmd(t, "user", "assign", "alex", "school", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	cfg := readCfg(t, path)
	var u *config.User
	for i := range cfg.Users {
		if cfg.Users[i].Username == "alex" {
			u = &cfg.Users[i]
		}
	}
	if u == nil {
		t.Fatal("alex was not added")
	}
	// A brand new managed user starts enforced. Defaulting to unmanaged would mean the first
	// `user assign` on a fresh install did nothing at all.
	if !u.Enabled {
		t.Error("a new managed user was created with enforcement off")
	}
}

func TestUserDisableSaysWhatItMeans(t *testing.T) {
	path := seed(t)
	code, out, errOut := runCmd(t, "user", "disable", "casey", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	cfg := readCfg(t, path)
	if cfg.Users[0].Enabled {
		t.Error("the user is still enabled")
	}
	// Disabling is the one CLI action that stops enforcement for a child without touching the
	// schedule. Echoing `enabled false` is not enough for that.
	if !strings.Contains(out, "no longer locked out") {
		t.Errorf("the confirmation does not say what it did:\n%s", out)
	}

	// And the listing has to keep saying it, so a later `status` cannot read as configured.
	if _, out, _ := runCmd(t, "user", "list", "--config", path); !strings.Contains(out, "never locked out") {
		t.Errorf("user list does not flag the disabled account:\n%s", out)
	}
}

func TestUserRmStopsManagingTheAccountNotDeletingIt(t *testing.T) {
	path := seed(t)
	code, out, errOut := runCmd(t, "user", "rm", "casey", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	cfg := readCfg(t, path)
	if len(cfg.Users) != 0 {
		t.Errorf("users left = %d, want 0", len(cfg.Users))
	}
	// The profile survives: removing a user must not cascade into deleting a schedule other
	// children may share.
	if len(cfg.Profiles) != 1 {
		t.Errorf("profiles left = %d, want the schedule untouched", len(cfg.Profiles))
	}
	if !strings.Contains(out, "account itself is untouched") {
		t.Errorf("the confirmation does not say the OS account is untouched:\n%s", out)
	}
}

func TestRuleAddNeedsBothEndpoints(t *testing.T) {
	path := seed(t)
	// One endpoint is the shape the evaluator has to widen to the whole day, which is the
	// opposite of what a half-filled flag line means.
	for _, args := range [][]string{
		{"--start", "20:00"},
		{"--end", "06:00"},
		{},
	} {
		code, _, errOut := runCmd(t, append([]string{"profile", "rule", "add", "school", "r2"}, args...)...)
		if code != exitUsage {
			t.Errorf("%v: exit %d, want %d", args, code, exitUsage)
		}
		if !strings.Contains(errOut, "both are required") && !strings.Contains(errOut, "both required") {
			t.Errorf("%v: stderr = %q", args, errOut)
		}
	}
	cfg := readCfg(t, path)
	if len(cfg.Profiles[0].Rules) != 1 {
		t.Error("a rule was written despite the incomplete window")
	}
}

func TestRuleAddWritesAWholeWindow(t *testing.T) {
	path := seed(t)
	code, out, errOut := runCmd(t, "profile", "rule", "add", "school", "sat",
		"--days", "sat,sun", "--start", "09:00", "--end", "21:00", "--name", "Weekend", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	cfg := readCfg(t, path)
	rules := cfg.Profiles[0].Rules
	if len(rules) != 2 {
		t.Fatalf("rules = %d, want 2", len(rules))
	}
	r := rules[1]
	if r.ID != "sat" || r.Start != "09:00" || r.End != "21:00" || !r.Enabled {
		t.Errorf("rule = %+v", r)
	}
	if len(r.Days) != 2 || r.Days[0] != "sat" || r.Days[1] != "sun" {
		t.Errorf("days = %v, want [sat sun]", r.Days)
	}
}

func TestRuleAddAcceptsLongWeekdayNames(t *testing.T) {
	path := seed(t)
	code, _, errOut := runCmd(t, "profile", "rule", "add", "school", "r",
		"--days", "Monday, FRIDAY ", "--start", "09:00", "--end", "10:00", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	cfg := readCfg(t, path)
	got := cfg.Profiles[0].Rules[1].Days
	if len(got) != 2 || got[0] != "mon" || got[1] != "fri" {
		t.Errorf("days = %v, want [mon fri]", got)
	}
}

func TestRuleAddRefusesAnUnknownWeekday(t *testing.T) {
	path := seed(t)
	code, _, errOut := runCmd(t, "profile", "rule", "add", "school", "r",
		"--days", "mon,funday", "--start", "09:00", "--end", "10:00", "--config", path)
	if code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "funday") {
		t.Errorf("stderr does not name the bad weekday: %q", errOut)
	}
}

func TestRuleAddRefusesADuplicateRuleID(t *testing.T) {
	path := seed(t)
	code, _, errOut := runCmd(t, "profile", "rule", "add", "school", "week",
		"--start", "09:00", "--end", "10:00", "--config", path)
	if code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, `already has a rule "week"`) {
		t.Errorf("stderr = %q", errOut)
	}
	// A rule id is what `rule rm` and `rule disable` take, so replacing one silently means
	// those commands later act on a rule the operator never saw.
	cfg := readCfg(t, path)
	if cfg.Profiles[0].Rules[0].Start != "20:00" {
		t.Error("the existing rule was replaced")
	}
}

func TestRuleDisableAndEnableFlipOneRule(t *testing.T) {
	path := seed(t)
	code, out, errOut := runCmd(t, "profile", "rule", "disable", "school", "week", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	cfg := readCfg(t, path)
	if cfg.Profiles[0].Rules[0].Enabled {
		t.Error("the rule is still enabled")
	}
	// Only that rule. Disabling by profile would be a different command.
	if code, _, errOut := runCmd(t, "profile", "rule", "enable", "school", "week", "--config", path); code != exitOK {
		t.Fatalf("re-enable: exit %d: %s", code, errOut)
	}
	if cfg = readCfg(t, path); !cfg.Profiles[0].Rules[0].Enabled {
		t.Error("re-enabling did not take")
	}
}

func TestRuleRmNamesAMissingRule(t *testing.T) {
	path := seed(t)
	code, _, errOut := runCmd(t, "profile", "rule", "rm", "school", "nope", "--config", path)
	if code != exitFailure {
		t.Errorf("exit %d, want %d", code, exitFailure)
	}
	if !strings.Contains(errOut, "no rule") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestManageCommandsTakeFlagsAfterThePositional(t *testing.T) {
	// The documented form. Go's flag package stops at the first non-flag argument, so
	// without cli.TakeFirst every one of these parses as positionals and zero flags.
	path := seed(t)
	code, out, errOut := runCmd(t, "profile", "add", "p2", "--name", "Second", "--timezone", "UTC", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	cfg := readCfg(t, path)
	for _, p := range cfg.Profiles {
		if p.ID == "p2" {
			if p.Name != "Second" {
				t.Errorf("name = %q, want Second (the flag was not parsed)", p.Name)
			}
			return
		}
	}
	t.Error("p2 was not written")
}

func TestManageRefusesAConfigItCannotRead(t *testing.T) {
	// Writing on top of a document this build cannot read would replace the operator's bytes
	// with one assembled from defaults, and the daemon would enforce something nobody
	// configured.
	gone := filepath.Join(t.TempDir(), "absent.json")
	for _, args := range [][]string{
		{"profile", "list"}, {"profile", "add", "x"}, {"user", "list"},
		{"user", "assign", "a", "b"}, {"profile", "rm", "x"},
	} {
		code, _, errOut := runCmd(t, append(args, "--config", gone)...)
		if code != exitFailure {
			t.Errorf("%v: exit %d, want %d", args, code, exitFailure)
		}
		if errOut == "" {
			t.Errorf("%v: nothing on stderr", args)
		}
	}
}

func TestManageSubcommandsRejectUnknownVerbs(t *testing.T) {
	path := seed(t)
	for _, args := range [][]string{{"profile", "frobnicate"}, {"user", "teleport"}, {"profile", "rule", "frobnicate"}} {
		code, _, errOut := runCmd(t, append(args, "--config", path)...)
		if code != exitUsage {
			t.Errorf("%v: exit %d, want %d", args, code, exitUsage)
		}
		if !strings.Contains(errOut, "unknown subcommand") {
			t.Errorf("%v: stderr = %q", args, errOut)
		}
	}
}

func TestManageHelpIsNotAnError(t *testing.T) {
	for _, args := range [][]string{{"profile", "-h"}, {"user", "--help"}, {"profile", "rule", "-h"}} {
		code, out, errOut := runCmd(t, args...)
		if code != exitOK {
			t.Errorf("%v: exit %d, want %d\n%s", args, code, exitOK, errOut)
		}
		if !strings.Contains(out, "usage: plock") {
			t.Errorf("%v: stdout = %q", args, out)
		}
		if errOut != "" {
			t.Errorf("%v: stderr = %q", args, errOut)
		}
	}
}

func TestManageWritesAreAtomicEnoughToReadBack(t *testing.T) {
	// Every write goes through config.Save, which is the discipline the daemon depends on: a
	// half-written config is a config the daemon cannot load, and a daemon that cannot load
	// its document enforces nothing.
	path := seed(t)
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("u%d", i)
		if code, out, errOut := runCmd(t, "user", "assign", name, "school", "--config", path); code != exitOK {
			t.Fatalf("%s: exit %d\n%s\n%s", name, code, out, errOut)
		}
	}
	cfg := readCfg(t, path)
	if len(cfg.Users) != 13 {
		t.Errorf("users = %d, want 13 (one per assign plus the seeded casey)", len(cfg.Users))
	}
}
