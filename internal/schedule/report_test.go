package schedule

import (
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// A profile whose timezone name does not resolve is evaluated in the machine's
// own zone. That is the right fallback — refusing to evaluate would drop every
// lockout in the profile — but it moves every window in the profile by whatever
// the two zones differ by, and it moves all of them at once.
//
// Validate rejects an unknown zone, so this is the hand-edit path: someone
// corrected the file directly, mistyped Berlin, and every rule in the profile went
// quiet without a word. ProfileErrors is where they hear about it.
func TestProfileErrorsReportsAnUnresolvableTimezone(t *testing.T) {
	p := profile("Europe/Berln", daily("21:00", "06:30"))

	errs := ProfileErrors(p)
	if len(errs) != 1 {
		t.Fatalf("ProfileErrors = %v, want the timezone reported", errs)
	}
	msg := errs[0].Error()
	if !strings.Contains(msg, "Europe/Berln") {
		t.Errorf("the error does not name the zone the parent typed: %q", msg)
	}
	if !strings.Contains(msg, "machine's own zone") {
		t.Errorf("the error does not say which zone is being used instead: %q", msg)
	}

	// The rule is still enforced, in the fallback zone. Reporting the zone must not
	// be allowed to switch the profile off, which would be the fail-open. Compared
	// against an explicitly empty timezone — the documented way to ask for the
	// machine's zone — so the assertion does not depend on what zone this test
	// machine happens to be in.
	fallback := &config.Profile{ID: p.ID, Timezone: "", Rules: p.Rules}
	from := at(time.UTC, 2026, 10, 12, 0, 0)
	fired := false
	for m := 0; m < 48*60; m++ {
		if Evaluate(p, from.Add(time.Duration(m)*time.Minute)).Active {
			fired = true
		}
		now := from.Add(time.Duration(m) * time.Minute)
		if got, want := Evaluate(p, now).Active, Evaluate(fallback, now).Active; got != want {
			t.Fatalf("%s: unresolvable zone says %v, the machine's zone says %v", now.Format(time.RFC3339), got, want)
		}
	}
	if !fired {
		t.Error("a profile with an unresolvable zone never enforced its rule in two days")
	}
}

// An empty timezone is the documented way to say "the machine's zone", so it is
// not a problem to report.
func TestProfileErrorsAcceptsAnEmptyTimezone(t *testing.T) {
	for _, tz := range []string{"", "UTC", "Europe/Berlin", "America/New_York"} {
		if errs := ProfileErrors(profile(tz, daily("21:00", "06:30"))); errs != nil {
			t.Errorf("ProfileErrors for timezone %q = %v, want nil", tz, errs)
		}
	}
}

// A rule that enforces nothing on every day of the year is reported, not silently
// skipped.
//
// The evaluator's own answer for the shape is "empty", which is right — reading
// the same time in both boxes as an overnight window would lock the child out
// around the clock. But a rule that never fires is indistinguishable, on screen,
// from a rule that fires and is being obeyed, and the parent has no way to find
// out. This is the case the skip-is-visible justification depends on.
func TestProfileErrorsReportsARuleThatEnforcesNothing(t *testing.T) {
	for _, tc := range []struct{ start, end string }{
		{"22:00", "22:00"},
		{"00:00", "00:00"},
		{"06:30", "06:30"},
	} {
		p := profile("UTC", rule("typo", []string{"mon"}, tc.start, tc.end))

		errs := ProfileErrors(p)
		if len(errs) != 1 {
			t.Fatalf("%s-%s: ProfileErrors = %v, want one problem", tc.start, tc.end, errs)
		}
		if msg := errs[0].Error(); !strings.Contains(msg, "enforces nothing") {
			t.Errorf("%s-%s: %q, want it to say the rule enforces nothing", tc.start, tc.end, msg)
		}

		// Reported and still skipped: the rule must not lock anyone out.
		if v := Evaluate(p, at(time.UTC, 2026, 10, 12, 12, 0)); v.Active {
			t.Errorf("%s-%s locked the account out on a Monday: %+v", tc.start, tc.end, v)
		}
	}
}

// A rule that is legal and merely emptied by the clock on one date is not
// reported. The parent typed two valid times; the calendar is what moved, and a
// warning every March would train everyone to ignore the list.
func TestProfileErrorsStaysQuietAboutARuleTheClockEmptied(t *testing.T) {
	transitionDay(t, zone(t, "Europe/Berlin"), 23*time.Hour)

	// Both ends inside the deleted hour: empty on that date, ordinary on every
	// other day of the year.
	p := profile("Europe/Berlin", daily("02:15", "02:45"))
	if errs := ProfileErrors(p); errs != nil {
		t.Errorf("ProfileErrors = %v, want nil: the rule is legal, the clock is not", errs)
	}
}

// A malformed rule and an unresolvable zone are different problems and both are
// reported, so a parent fixing the zone still sees the typo in the rule.
func TestProfileErrorsReportsBothProblems(t *testing.T) {
	p := &config.Profile{
		ID:       "kid",
		Timezone: "Mars/Olympus",
		Rules: []config.Rule{
			rule("good", []string{"mon"}, "09:00", "10:00"),
			rule("bad", []string{"Tues"}, "09:00", "10:00"),
		},
	}

	errs := ProfileErrors(p)
	if len(errs) != 2 {
		t.Fatalf("ProfileErrors = %v, want the bad rule and the zone", errs)
	}

	var sawRule, sawZone bool
	for _, err := range errs {
		switch {
		case strings.Contains(err.Error(), "Tues"):
			sawRule = true
		case strings.Contains(err.Error(), "Mars/Olympus"):
			sawZone = true
		}
	}
	if !sawRule || !sawZone {
		t.Errorf("reported %v, want one naming the bad day and one naming the zone", errs)
	}
}
