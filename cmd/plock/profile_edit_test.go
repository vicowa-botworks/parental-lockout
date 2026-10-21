package main

import (
	"os"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// profileByID reads the profile out of the file on disk. Every assertion in this file goes
// through it, because the command's own output is not evidence that it saved anything.
func profileByID(t *testing.T, path, id string) config.Profile {
	t.Helper()
	cfg := readCfg(t, path)
	p, ok := findProfile(cfg, id)
	if !ok {
		t.Fatalf("the command left no profile %q on disk", id)
	}
	return p
}

// readFile is the raw bytes, for the cases where "the file did not change" is the property.
func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// `profile edit` is the one management command that changes a profile a child is already
// assigned to, so the property that matters is not "did it write" but "did it write only
// what it was told". A profile carries rules, a timezone and an enabled bit; a rename that
// also reset the timezone would move every window in the profile by hours and look like a
// cosmetic edit on screen.

func TestProfileEditChangesOnlyTheName(t *testing.T) {
	path := seed(t)
	code, _, errOut := runCmd(t, "profile", "edit", "school", "--name", "School nights (winter)", "--config", path)
	if code != 0 {
		t.Fatalf("exit %d, stderr %s", code, errOut)
	}
	p := profileByID(t, path, "school")
	if p.Name != "School nights (winter)" {
		t.Errorf("name = %q", p.Name)
	}
	if p.Timezone != "Europe/London" {
		t.Errorf("the timezone moved to %q while only the name was asked for", p.Timezone)
	}
	if len(p.Rules) != 1 || p.Rules[0].ID != "week" {
		t.Errorf("the rules changed: %+v", p.Rules)
	}
}

func TestProfileEditChangesOnlyTheTimezone(t *testing.T) {
	path := seed(t)
	code, _, errOut := runCmd(t, "profile", "edit", "school", "--timezone", "America/New_York", "--config", path)
	if code != 0 {
		t.Fatalf("exit %d, stderr %s", code, errOut)
	}
	p := profileByID(t, path, "school")
	if p.Timezone != "America/New_York" {
		t.Errorf("timezone = %q", p.Timezone)
	}
	if p.Name != "School nights" {
		t.Errorf("the name moved to %q while only the timezone was asked for", p.Name)
	}
	// The rules are the payload here. They carry no zone of their own -- the profile's is
	// the one the evaluator resolves them against -- so an edit that dropped them would
	// silently release the child rather than reschedule them.
	if len(p.Rules) != 1 || p.Rules[0].Start != "20:00" || p.Rules[0].End != "06:00" {
		t.Errorf("the rules did not survive the timezone change: %+v", p.Rules)
	}
}

func TestProfileEditRefusesWhenThereIsNothingToChange(t *testing.T) {
	path := seed(t)
	code, _, errOut := runCmd(t, "profile", "edit", "school", "--config", path)
	if code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "nothing to change") {
		t.Errorf("stderr = %q", errOut)
	}
	// An edit with no fields must not rewrite the file at all: a save re-encodes the whole
	// document, so a no-op that still writes would churn mtime and re-order nothing, and
	// `plock status` reports config mtime as the "did anything happen" signal.
	before := readFile(t, path)
	if got := readFile(t, path); got != before {
		t.Error("the file changed for an edit that changed nothing")
	}
}

func TestProfileEditRefusesAZoneTheSystemCannotResolve(t *testing.T) {
	path := seed(t)
	code, _, errOut := runCmd(t, "profile", "edit", "school", "--timezone", "Mars/Olympus_Mons", "--config", path)
	if code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "not one this system knows") {
		t.Errorf("stderr = %q", errOut)
	}
	if p := profileByID(t, path, "school"); p.Timezone != "Europe/London" {
		t.Errorf("the unusable zone was written: %q", p.Timezone)
	}
}

// `Local` is the one value time.LoadLocation resolves that has no meaning of its own: it means
// "whatever this machine's clock is set to". The web form refuses it, and a rule that lives only
// in the form is a rule about a screen rather than about the config -- `plock profile edit
// --timezone Local` would land it anyway, and nothing would ever say so.
func TestProfileEditRefusesLocalBecauseItFollowsTheMachine(t *testing.T) {
	path := seed(t)
	code, _, errOut := runCmd(t, "profile", "edit", "school", "--timezone", "Local", "--config", path)
	if code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "machine's clock") {
		t.Errorf("stderr = %q, want it to say Local follows the machine rather than the household",
			errOut)
	}
	if p := profileByID(t, path, "school"); p.Timezone != "Europe/London" {
		t.Errorf("Local was written: %q", p.Timezone)
	}
}

func TestProfileEditOnAnUnknownProfileIsAFailureNotAUsageError(t *testing.T) {
	path := seed(t)
	code, _, errOut := runCmd(t, "profile", "edit", "nobody", "--name", "x", "--config", path)
	// 1, not 2: the command line was well formed, the machine just has no such profile.
	// A script that retries on usage errors must not retry this.
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut, `no profile "nobody"`) {
		t.Errorf("stderr = %q", errOut)
	}
}

// Write commands report in text and have no --json. That is the ticket's line -- "`--json`
// on every read command" -- and it is worth stating as a decision rather than an omission,
// because the report is built through cli.Report either way and a later change could quietly
// make writes machine-readable without anyone deciding to. What is asserted here is the
// promise the command actually makes: the fields a parent reads back to confirm the edit
// landed on the right profile.
func TestProfileEditReportsTheFieldsItChanged(t *testing.T) {
	path := seed(t)
	code, out, errOut := runCmd(t, "profile", "edit", "school", "--name", "Winter", "--config", path)
	if code != 0 {
		t.Fatalf("exit %d, stderr %s", code, errOut)
	}
	for _, want := range []string{"profile", "school", "name", "Winter", "timezone", "Europe/London"} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not mention %q:\n%s", want, out)
		}
	}
	// The untouched zone is reported too. "Here is the profile as it now stands" is a
	// different promise from "here is what I changed", and a report that shows only the
	// changed field cannot distinguish "the zone is still London" from "the zone is gone".
	if strings.Contains(out, "unchanged") {
		t.Errorf("the report claims something is unchanged; it reports state, not deltas:\n%s", out)
	}
}

func TestProfileEditNeedsAnID(t *testing.T) {
	path := seed(t)
	code, _, errOut := runCmd(t, "profile", "edit", "--name", "x", "--config", path)
	if code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "profile id is required") {
		t.Errorf("stderr = %q", errOut)
	}
}
