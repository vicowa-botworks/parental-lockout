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

// TestProfileAddCopiesRulesAndTheirTimezoneTogether pins what travels from a copied profile and
// what does not.
//
// It used to run without --name, which is the behaviour #122 removed: --copy-from carried the
// source's display name onto the copy, so the command produced exactly the pair the web rename
// form refuses -- two profiles with one name, which the profiles page cannot tell apart. The
// copy now has to be named, so the test names it, and the assertion that the name is the one
// supplied is the half that keeps the two properties from being confused: the rules and the zone
// come across because a rule is a wall-clock time, and the name does not come across because a
// name is how the operator finds the profile afterwards.
func TestProfileAddCopiesRulesAndTheirTimezoneTogether(t *testing.T) {
	path := seed(t)
	code, out, errOut := runCmd(t, "profile", "add", "weekend", "--copy-from", "school",
		"--name", "Weekend rules", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	cfg := readCfg(t, path)
	var p, src *config.Profile
	for i := range cfg.Profiles {
		switch cfg.Profiles[i].ID {
		case "weekend":
			p = &cfg.Profiles[i]
		case "school":
			src = &cfg.Profiles[i]
		}
	}
	if p == nil {
		t.Fatal("no weekend profile was written")
	}
	if src == nil {
		t.Fatal("the source profile disappeared")
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
	if p.Rules[0].ID != "week" {
		t.Errorf("copied rule ids = %q, want the source's", p.Rules[0].ID)
	}
	if p.Name != "Weekend rules" {
		t.Errorf("name = %q, want the one --name supplied", p.Name)
	}
	if p.Name == src.Name {
		t.Errorf("the copy shares the source's name %q, which is the pair this command must not make",
			src.Name)
	}
}

// TestProfileAddRefusesToCopyANameThatIsAlreadyUsed is the symptom in #122, in one command.
//
// `--copy-from` with no `--name` used to carry the source's display name onto the copy and exit
// 0. The result is two profiles the profiles page renders with one heading, one table caption
// and two delete confirmations naming the same thing -- the exact state POST /profiles/{id}/rename
// was written to refuse. The refusal was only ever on the form, and the config is writable from
// three places.
func TestProfileAddRefusesToCopyANameThatIsAlreadyUsed(t *testing.T) {
	path := seed(t)
	before := readCfg(t, path)

	code, _, errOut := runCmd(t, "profile", "add", "weekend", "--copy-from", "school", "--config", path)
	if code != exitFailure {
		t.Fatalf("exit %d, want %d: copying a name that is in use must not succeed\n%s",
			code, exitFailure, errOut)
	}
	if !strings.Contains(errOut, "already has that name") {
		t.Errorf("stderr = %q, want it to name the collision", errOut)
	}
	// Refused means nothing written, not "written and complained about".
	if got := readCfg(t, path); len(got.Profiles) != len(before.Profiles) {
		t.Errorf("the config gained a profile anyway: %d, want %d",
			len(got.Profiles), len(before.Profiles))
	}
}

// TestProfileAddAcceptsACopyGivenItsOwnName is the other half of the refusal: the command is
// not being made unusable, only made to ask. --name is the answer the error message gives, so
// the message has to be able to work.
func TestProfileAddAcceptsACopyGivenItsOwnName(t *testing.T) {
	path := seed(t)
	code, out, errOut := runCmd(t, "profile", "add", "weekend", "--copy-from", "school",
		"--name", "Weekends", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	cfg := readCfg(t, path)
	for _, p := range cfg.Profiles {
		if p.ID == "weekend" && p.Name != "Weekends" {
			t.Errorf("name = %q, want the one --name supplied", p.Name)
		}
	}
}

// TestProfileAddRefusesAnExplicitNameThatIsAlreadyUsed covers the path where the operator types
// the colliding name themselves. The rule is about the resulting config, not about which flag
// the value came from, so --name is refused exactly as --copy-from is.
func TestProfileAddRefusesAnExplicitNameThatIsAlreadyUsed(t *testing.T) {
	path := seed(t)
	code, _, errOut := runCmd(t, "profile", "add", "weekend", "--name", "School nights",
		"--config", path)
	if code != exitFailure {
		t.Fatalf("exit %d, want %d\n%s", code, exitFailure, errOut)
	}
	if !strings.Contains(errOut, "already has that name") {
		t.Errorf("stderr = %q, want it to name the collision", errOut)
	}
}

// TestProfileEditRefusesANameAnotherProfileHas is the second CLI write path. Before this it
// checked nothing, so `plock profile edit school --name "Kids"` on a machine with a profile
// called Kids renamed straight into the ambiguous pair.
func TestProfileEditRefusesANameAnotherProfileHas(t *testing.T) {
	path := writeCfg(t, `{
  "version": 1,
  "settings": {"enforcement": true, "pollSeconds": 30, "lockBackends": ["logind"]},
  "profiles": [
    {"id": "school", "name": "School nights", "timezone": "Europe/London"},
    {"id": "weekend", "name": "Weekends", "timezone": "UTC"}
  ],
  "users": [{"username": "casey", "profileID": "school", "enabled": true}]
}`)
	before := readCfg(t, path)

	code, _, errOut := runCmd(t, "profile", "edit", "school", "--name", "Weekends", "--config", path)
	if code != exitFailure {
		t.Fatalf("exit %d, want %d\n%s", code, exitFailure, errOut)
	}
	// The same sentence the web form answers 422 with, not a CLI paraphrase of it: two
	// wordings for one refusal is how an operator decides only one path is real.
	if !strings.Contains(errOut, "already has that name") {
		t.Errorf("stderr = %q, want the shared refusal sentence", errOut)
	}
	if got := readCfg(t, path); got.Profiles[0].Name != before.Profiles[0].Name {
		t.Errorf("the name was changed anyway, to %q", got.Profiles[0].Name)
	}
}

// TestProfileEditAllowsRepeatingTheProfilesOwnName keeps the rule the size of its reason. The
// refusal is about two profiles sharing a label, so re-stating a profile's own name -- which is
// what a form does when only the other field was edited -- has to stay allowed.
func TestProfileEditAllowsRepeatingTheProfilesOwnName(t *testing.T) {
	path := writeCfg(t, `{
  "version": 1,
  "settings": {"enforcement": true, "pollSeconds": 30, "lockBackends": ["logind"]},
  "profiles": [
    {"id": "school", "name": "School nights", "timezone": "Europe/London"},
    {"id": "weekend", "name": "Weekends", "timezone": "UTC"}
  ],
  "users": [{"username": "casey", "profileID": "school", "enabled": true}]
}`)
	code, out, errOut := runCmd(t, "profile", "edit", "school", "--name", "School nights",
		"--timezone", "Europe/Paris", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	cfg := readCfg(t, path)
	if cfg.Profiles[0].Name != "School nights" {
		t.Errorf("name = %q, want it unchanged", cfg.Profiles[0].Name)
	}
	if cfg.Profiles[0].Timezone != "Europe/Paris" {
		t.Errorf("timezone = %q, want the change to have gone through", cfg.Profiles[0].Timezone)
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
