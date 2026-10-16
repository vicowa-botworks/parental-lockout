package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// wednesdayNoon is a plain instant with no DST interest: the tests here are about
// which switch decides the answer, not about the calendar.
var wednesdayNoon = time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)

// insideWindow is two hours after the school-night rule opens.
var insideWindow = time.Date(2026, 3, 18, 23, 0, 0, 0, time.UTC)

func schoolNights() config.Profile {
	return config.Profile{
		ID:       "school",
		Name:     "School nights",
		Timezone: "UTC",
		Rules: []config.Rule{{
			ID: "r1", Name: "Sun–Thu 21:00 → 06:30",
			Days:  []string{"sun", "mon", "tue", "wed", "thu"},
			Start: "21:00", End: "06:30", Enabled: true,
		}},
	}
}

func cfg(users ...config.User) *config.Config {
	c := config.Default()
	c.Profiles = []config.Profile{schoolNights()}
	c.Users = users
	return c
}

func casey(enabled bool) config.User {
	return config.User{Username: "casey", ProfileID: "school", Enabled: enabled}
}

func row(t *testing.T, v View, name string) Row {
	t.Helper()
	for _, r := range v.Rows {
		if r.Username == name {
			return r
		}
	}
	t.Fatalf("no row for %q", name)
	return Row{}
}

// The premise of every other test here, checked rather than assumed: inside the
// window with everything on, the account really is held.
func TestInsideTheWindowTheAccountIsHeld(t *testing.T) {
	v := Build(cfg(casey(true)), nil, platform.SelfLocation{}, insideWindow)
	r := row(t, v, "casey")
	if !r.LockedOutNow {
		t.Fatalf("an account inside its window with enforcement on is not held: %+v", r)
	}
	if r.RuleName == "" || r.EndsAt.IsZero() {
		t.Errorf("the row does not say what is holding it: %+v", r)
	}
	if v.LockedOut != 1 {
		t.Errorf("LockedOut = %d, want 1", v.LockedOut)
	}
}

// The end time is the evaluator's, not one re-derived from the rule's clock text.
//
// This is asserted rather than incidentally true because the two agree on an ordinary
// day and disagree mid-DST-fold, where the wall clock can read before the written
// start while the account is still held. A template that recomputed the end from
// "06:30" would announce a release that has not happened.
func TestEndsAtIsTheVerdictsOwnEnd(t *testing.T) {
	v := Build(cfg(casey(true)), nil, platform.SelfLocation{}, insideWindow)
	r := row(t, v, "casey")
	want := time.Date(2026, 3, 19, 6, 30, 0, 0, time.UTC)
	if !r.EndsAt.Equal(want) {
		t.Errorf("EndsAt = %s, want %s", r.EndsAt, want)
	}
	if !strings.Contains(v.Banner, "06:30") {
		t.Errorf("the banner does not carry the verdict's end time: %q", v.Banner)
	}
}

// The master switch off is the state where reading Verdict.Active directly does the
// most damage: the schedule says held, nothing is doing it, and the screen would say
// the child is sitting at a locked screen.
func TestMasterSwitchOffIsNotReportedAsLockedOut(t *testing.T) {
	c := cfg(casey(true))
	c.Settings.Enforcement = false
	v := Build(c, nil, platform.SelfLocation{}, insideWindow)
	r := row(t, v, "casey")
	if r.LockedOutNow {
		t.Error("the daemon will not touch this account, so it is not locked out by us")
	}
	if !r.ScheduledButNotEnforcing {
		t.Error("the schedule does say held, and that has to be visible rather than shown as an all-clear")
	}
	if v.LockedOut != 0 {
		t.Errorf("LockedOut = %d with the switch off, want 0", v.LockedOut)
	}
	if v.NotEnforcing != 1 {
		t.Errorf("NotEnforcing = %d, want 1", v.NotEnforcing)
	}
}

// The banner with the switch off says only that. Anything else in the sentence would
// be a time at which something happens, and with the switch off nothing happens at
// any time.
func TestBannerWithEnforcementOffSaysOnlyThat(t *testing.T) {
	c := cfg(casey(true))
	c.Settings.Enforcement = false
	v := Build(c, nil, platform.SelfLocation{}, insideWindow)
	if !strings.Contains(v.Banner, "switched off") {
		t.Errorf("banner = %q", v.Banner)
	}
	if strings.Contains(v.Banner, "06:30") || strings.Contains(v.Banner, "21:00") {
		t.Errorf("the banner quotes a time while nothing is enforcing: %q", v.Banner)
	}
}

// An open override suppresses enforcement, so the account is not held even though the
// window covers now.
func TestAnOpenOverrideReleasesTheRow(t *testing.T) {
	c := cfg(casey(true))
	c.Overrides = []config.Override{{
		Username: "casey", Until: insideWindow.Add(30 * time.Minute),
		Reason: "film night", By: "admin",
	}}
	v := Build(c, nil, platform.SelfLocation{}, insideWindow)
	r := row(t, v, "casey")
	if r.LockedOutNow {
		t.Error("an override is open, so the daemon is deliberately not holding this account")
	}
	if !r.OverrideActive || r.OverrideReason != "film night" {
		t.Errorf("the override is not on the row: %+v", r)
	}
	if !r.ScheduledButNotEnforcing {
		t.Error("the window still covers now; the row must not read as an all-clear")
	}
}

// An expired override is shown as expired rather than dropped.
//
// The engine stops honouring it the instant it passes, and a parent who then sees
// nothing at all cannot tell "nobody ever unlocked this child" from "the unlock I made
// ran out twenty minutes ago" -- which is the difference between making another one
// and going to look at the schedule.
func TestAnExpiredOverrideIsShownAsExpired(t *testing.T) {
	c := cfg(casey(true))
	c.Overrides = []config.Override{{
		Username: "casey", Until: insideWindow.Add(-time.Minute), Reason: "film night",
	}}
	v := Build(c, nil, platform.SelfLocation{}, insideWindow)
	r := row(t, v, "casey")
	if !r.OverrideExpired {
		t.Error("the override ran out and the row says nothing about it")
	}
	if r.OverrideActive {
		t.Error("an expired override does not suppress enforcement")
	}
	if !r.LockedOutNow {
		t.Error("enforcement resumes the moment the override expires")
	}
}

// A managed account whose profile is gone is the row a template gets wrong.
//
// There is no schedule, so nothing is locked out -- and "not locked out" is exactly
// the wrong thing to tell a parent whose configuration has stopped meaning what they
// set. The row has to refuse to answer rather than answer no.
func TestAMissingProfileIsNotReportedAsNotLockedOut(t *testing.T) {
	c := cfg(config.User{Username: "casey", ProfileID: "weekend", Enabled: true})
	v := Build(c, nil, platform.SelfLocation{}, insideWindow)
	r := row(t, v, "casey")
	if !r.CannotEvaluate {
		t.Error("the row must say it could not be decided")
	}
	if len(r.Problems) == 0 {
		t.Fatal("a row that could not be evaluated has to say why")
	}
	if !strings.Contains(r.Problems[0], "weekend") {
		t.Errorf("the problem does not name the missing profile: %q", r.Problems[0])
	}
	if r.LockedOutNow {
		t.Error("nothing is holding this account, and claiming otherwise is its own lie")
	}
	if v.NotEnforcing != 1 {
		t.Errorf("an unreadable row is not coverage: NotEnforcing = %d", v.NotEnforcing)
	}
}

// A profile with a bad timezone still enforces -- the evaluator widens rather than
// dropping -- but the verdict rests on a fallback and the parent is told.
func TestADegradedProfileIsReportedAlongsideItsVerdict(t *testing.T) {
	c := cfg(casey(true))
	c.Profiles[0].Timezone = "Not/AZone"
	v := Build(c, nil, platform.SelfLocation{}, insideWindow)
	r := row(t, v, "casey")
	if len(r.Problems) == 0 {
		t.Fatal("a profile read in the wrong zone is a problem worth naming")
	}
	if !strings.Contains(strings.ToLower(r.Problems[0]), "zone") {
		t.Errorf("the problem does not name the timezone: %q", r.Problems[0])
	}
}

// The account's own switch is a different fix from the daemon-wide one, so the row
// keeps them apart.
func TestADisabledAccountIsNotHeld(t *testing.T) {
	v := Build(cfg(casey(false)), nil, platform.SelfLocation{}, insideWindow)
	r := row(t, v, "casey")
	if r.LockedOutNow {
		t.Error("the account is switched off")
	}
	if r.ScheduledButNotEnforcing {
		t.Error("this row is switched off deliberately; it is not a schedule being silently unenforced")
	}
}

// Sessions belong to the row of the account that owns them, and a session for an
// unmanaged account is not shown at all.
func TestSessionsAreGroupedOntoTheirOwnRow(t *testing.T) {
	sessions := []platform.Session{
		{ID: "2", Username: "casey", Seat: "seat0", Type: "wayland", Locked: true},
		{ID: "3", Username: "casey", Seat: "", Type: "tty"},
		{ID: "4", Username: "root", Seat: "seat1", Type: "x11"},
	}
	v := Build(cfg(casey(true)), sessions, platform.SelfLocation{}, insideWindow)
	r := row(t, v, "casey")
	if len(r.Sessions) != 2 {
		t.Fatalf("casey's row carries %d sessions, want the 2 that are theirs", len(r.Sessions))
	}
	for _, s := range r.Sessions {
		if s.Username != "casey" {
			t.Errorf("a session belonging to %q landed on casey's row", s.Username)
		}
	}
	if !r.Sessions[0].Locked {
		t.Error("the lock state the backend reported is not carried through")
	}
}

// Outside the window the row shows when the next one opens, which is the question a
// parent actually asks.
func TestOutsideTheWindowTheRowShowsTheNextOpening(t *testing.T) {
	v := Build(cfg(casey(true)), nil, platform.SelfLocation{}, wednesdayNoon)
	r := row(t, v, "casey")
	if r.LockedOutNow {
		t.Fatal("noon is outside a 21:00 window")
	}
	if r.NextLockAt.IsZero() {
		t.Fatal("no next window on a row whose profile has one")
	}
	if want := time.Date(2026, 3, 18, 21, 0, 0, 0, time.UTC); !r.NextLockAt.Equal(want) {
		t.Errorf("NextLockAt = %s, want %s", r.NextLockAt, want)
	}
	if !strings.Contains(v.Banner, "21:00") {
		t.Errorf("the banner does not say when the next lockout begins: %q", v.Banner)
	}
	if !strings.Contains(v.Banner, "casey") {
		t.Errorf("the banner does not say who: %q", v.Banner)
	}
}

// The held banner names the rule, not just the time.
//
// "Locked out until 06:30" cannot be told apart from the weekend rule the parent
// forgot to switch off, and that is the complaint that arrives by text at seven in
// the morning.
func TestHeldBannerNamesTheRule(t *testing.T) {
	v := Build(cfg(casey(true)), nil, platform.SelfLocation{}, insideWindow)
	// The rule's own name, not the profile's: a profile can hold several rules and the
	// parent needs the one that is actually holding the child.
	if !strings.Contains(v.Banner, "Sun\u2013Thu 21:00 \u2192 06:30") {
		t.Errorf("the banner does not name the responsible rule: %q", v.Banner)
	}
	if !strings.Contains(v.Banner, "casey") {
		t.Errorf("the banner does not say who is held: %q", v.Banner)
	}
}

// Rows are ordered by username so the page is stable between loads.
func TestRowsAreOrderedByUsername(t *testing.T) {
	v := Build(cfg(casey(true),
		config.User{Username: "zed", ProfileID: "school", Enabled: true},
		config.User{Username: "amy", ProfileID: "school", Enabled: true}), nil, platform.SelfLocation{}, wednesdayNoon)
	got := []string{}
	for _, r := range v.Rows {
		got = append(got, r.Username)
	}
	want := []string{"amy", "casey", "zed"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rows = %v, want %v", got, want)
		}
	}
}

// A nil configuration is not an empty dashboard. It is the state where the daemon has
// nothing to enforce, and the page has to say so rather than render a blank table
// that looks like a quiet evening.
func TestNilConfigSaysNothingIsEnforced(t *testing.T) {
	v := Build(nil, nil, platform.SelfLocation{}, insideWindow)
	if len(v.Rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(v.Rows))
	}
	if !strings.Contains(v.Banner, "no configuration") {
		t.Errorf("banner = %q", v.Banner)
	}
}

// No managed accounts at all is a different sentence from accounts with nothing
// scheduled: the first is a setup step missing, the second is a normal afternoon.
func TestNoManagedAccountsIsItsOwnSentence(t *testing.T) {
	v := Build(cfg(), nil, platform.SelfLocation{}, insideWindow)
	if !strings.Contains(v.Banner, "No accounts are managed") {
		t.Errorf("banner = %q", v.Banner)
	}
}

// An inactive verdict contributes no end time.
//
// EndsAt on an inactive verdict bounds a window that is not running, and a countdown
// on a row that is not locked out is a clock counting down to nothing.
func TestAnInactiveRowCarriesNoEndsAt(t *testing.T) {
	v := Build(cfg(casey(true)), nil, platform.SelfLocation{}, wednesdayNoon)
	r := row(t, v, "casey")
	if !r.EndsAt.IsZero() {
		t.Errorf("an inactive row shows an end time: %s", r.EndsAt)
	}
	if r.RuleName != "" {
		t.Errorf("an inactive row names a responsible rule: %q", r.RuleName)
	}
}

// A successful read is knowledge, even when it found nobody.
//
// SessionsKnown is what separates "nobody has a desktop session" from "we could not
// look", and the web layer renders the two differently. Build is the thing that claims
// knowledge, so Build is where it gets pinned -- otherwise the flag is only observable
// through a page test in another package, and a dashboard test that builds a View
// directly never learns the difference exists.
func TestABuildThatWasGivenSessionsSaysItKnowsThem(t *testing.T) {
	if v := Build(cfg(casey(true)), nil, platform.SelfLocation{}, insideWindow); !v.SessionsKnown {
		t.Error("Build reported an empty session list as a read that failed; a machine nobody is " +
			"logged into would be described as a broken session backend")
	}
	sessions := []platform.Session{{Username: "casey", Type: "wayland"}}
	if v := Build(cfg(casey(true)), sessions, platform.SelfLocation{}, insideWindow); !v.SessionsKnown {
		t.Error("a read that returned sessions is not marked as knowledge")
	}
}

// The other side is the caller's to say, and only the caller's: a nil session list from
// a source that worked is not the same as a source that failed, so Build must not guess.
func TestBuildNeverInventsAFailedRead(t *testing.T) {
	v := Build(cfg(casey(true)), nil, platform.SelfLocation{}, insideWindow)
	if v.Rows[0].Sessions != nil && len(v.Rows[0].Sessions) != 0 {
		t.Error("an empty session list produced sessions")
	}
}
