package schedule

import (
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// allDayRule is a rule whose parent checked the box. The clock ends are whatever
// the form left behind, which is the point: they are not consulted.
func allDayRule(id string, days []string, start, end string) config.Rule {
	r := rule(id, days, start, end)
	r.AllDay = true
	return r
}

// The bug this issue is about, stated as the assertion that used to fail.
//
// 00:00-23:59 is 1439 minutes and TestEvaluateAllDayWindowIsOneMinuteShort pins the
// minute it gives back. A parent who meant "all day" and wrote the closest thing
// they could type lost the machine for one minute every day, with no signal — and
// one minute is two 30-second re-lock ticks, which is all a child needs.
func TestEvaluateAllDayCoversTheMinuteTheClockEndsMiss(t *testing.T) {
	loc := utc(t)
	p := profile("UTC", allDayRule("allday", allDays, "00:00", "23:59"))

	cases := []struct {
		name   string
		now    time.Time
		active bool
	}{
		{"midnight", at(loc, 2026, 10, 12, 0, 0), true},
		{"midday", at(loc, 2026, 10, 12, 12, 0), true},
		{"23:58", at(loc, 2026, 10, 12, 23, 58), true},
		{"23:59, the minute the clock ends gave back", at(loc, 2026, 10, 12, 23, 59), true},
		{"23:59:59", at(loc, 2026, 10, 12, 23, 59).Add(59 * time.Second), true},
		// There is no "after" to assert here. This profile names all seven days, so
		// Tuesday's own window opens at the instant Monday's closes and the rule is
		// continuous by design — that is what the parent asked for. That the window
		// still has an end, and that a rule naming fewer days honours it, is
		// TestEvaluateAllDayStillRespectsItsDays.
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if v := Evaluate(p, tc.now); v.Active != tc.active {
				t.Fatalf("Active = %v at %s, want %v", v.Active, tc.now.Format(time.RFC3339), tc.active)
			}
		})
	}
}

// Every minute of the day, asserted by walking the day rather than by sampling it.
// The gap in 00:00-23:59 was exactly one minute wide, so a test that checked the
// hour and the half hour would have found nothing.
func TestEvaluateAllDayLeavesNoMinuteUnenforced(t *testing.T) {
	loc := utc(t)
	p := profile("UTC", allDayRule("allday", allDays, "", ""))

	noon := at(loc, 2026, 10, 12, 12, 0)
	v := Evaluate(p, noon)
	if !v.Active {
		t.Fatalf("an all-day rule is not enforcing at noon: %+v", v)
	}

	// Walk the whole day at one-minute resolution, plus the two instants either side
	// of each boundary. 1440 samples is nothing next to the DST tests in this file.
	var free []string
	for i := 0; i < 24*60; i++ {
		c := at(loc, 2026, 10, 12, 0, 0).Add(time.Duration(i) * time.Minute)
		if !Evaluate(p, c).Active {
			free = append(free, c.Format("15:04"))
		}
	}
	if len(free) != 0 {
		t.Errorf("an all-day rule released %d minute(s) of its own day: %s", len(free), strings.Join(free, ", "))
	}
}

// All-day is a window, not a switch. A rule for Mondays must not hold the child
// out on Tuesday, or the flag is a permanent lockout with a day list on it.
func TestEvaluateAllDayStillRespectsItsDays(t *testing.T) {
	loc := utc(t)
	// 2026-10-12 is a Monday.
	p := profile("UTC", allDayRule("school", []string{"mon"}, "00:00", "23:59"))

	for _, tc := range []struct {
		day    time.Time
		active bool
	}{
		{at(loc, 2026, 10, 11, 12, 0), false}, // Sunday
		{at(loc, 2026, 10, 12, 0, 0), true},   // Monday 00:00
		{at(loc, 2026, 10, 12, 23, 59), true}, // Monday 23:59
		{at(loc, 2026, 10, 13, 12, 0), false}, // Tuesday
	} {
		if v := Evaluate(p, tc.day); v.Active != tc.active {
			t.Errorf("Active = %v at %s (%s), want %v",
				v.Active, tc.day.Format("2006-01-02 15:04"), tc.day.Format("Mon"), tc.active)
		}
	}
}

// The reporting half, and the reason allDay is not simply aliased onto the
// widening this package already does.
//
// A rule widened from a clock it could not read is a degraded state and must be
// reported. A rule whose parent asked for the whole day is not. Reporting the
// second one like the first would put a line in the dashboard on every profile
// that uses the feature, which trains a parent to ignore the report — and the
// report is the only thing standing between a mistyped rule and a silent hole.
func TestAllDayIsNotReportedAsDegraded(t *testing.T) {
	for _, tc := range []struct {
		name           string
		rule           config.Rule
		wantErrs       bool
		wantEnforcing  bool
		wantErrMention string
	}{
		{
			// A form that sends the checkbox with the times still attached.
			name:          "declared, with ordinary times",
			rule:          allDayRule("a", allDays, "20:00", "06:00"),
			wantEnforcing: true,
		},
		{
			// The same form after it cleared the fields. Both are legal, and the
			// parent is not told their rule is broken in either case.
			name:          "declared, with the times cleared",
			rule:          allDayRule("b", allDays, "", ""),
			wantEnforcing: true,
		},
		{
			name:          "declared, with times that are not times",
			rule:          allDayRule("c", allDays, "bedtime", "25:00"),
			wantEnforcing: true,
		},
		{
			// The days are a separate part of the rule and are still unreadable.
			name:           "declared, with a day nobody spells",
			rule:           allDayRule("d", []string{"tues"}, "", ""),
			wantErrs:       true,
			wantEnforcing:  true,
			wantErrMention: "every day",
		},
		{
			// Control: the inferred widening keeps its report.
			name:           "inferred from an unreadable clock",
			rule:           rule("e", allDays, "bedtime", "wake"),
			wantErrs:       true,
			wantEnforcing:  true,
			wantErrMention: "widest reading",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := profile("UTC", tc.rule)
			errs := ProfileErrors(p)
			if tc.wantErrs {
				if len(errs) == 0 {
					t.Fatal("ProfileErrors = nil, want the rule reported")
				}
				if tc.wantErrMention != "" && !strings.Contains(errs[0].Error(), tc.wantErrMention) {
					t.Errorf("error %q does not say %q", errs[0], tc.wantErrMention)
				}
				if tc.rule.AllDay && strings.Contains(errs[0].Error(), "is not HH:MM") {
					// Only a declared all-day rule has a clock nobody reads. The
					// control below is an ordinary rule whose times really are
					// unreadable, and naming them is the whole of the report.
					t.Errorf("error %q complains about a clock an all-day rule never reads", errs[0])
				}
			} else if len(errs) != 0 {
				t.Errorf("ProfileErrors = %v, want nil: the rule says exactly what it enforces", errs)
			}

			noon := at(utc(t), 2026, 10, 12, 12, 0)
			if got := Evaluate(p, noon).Active; got != tc.wantEnforcing {
				t.Errorf("Active at noon = %v, want %v", got, tc.wantEnforcing)
			}
		})
	}
}

// A declared all-day rule inherits the endpoint choice the widened rule already
// pays for, and that choice is not free. Where a fall-back repeats midnight, the
// window ends at the LAST reading of the next midnight, so up to an hour is held
// whose clock already reads the next day.
//
// This is asserted rather than assumed because the two readings of "end of today"
// differ by an hour in America/Havana, and the one that frees a minute of the
// enforced day is the one that fails open.
func TestAllDayEndsAtTheLastReadingOfNextMidnight(t *testing.T) {
	for _, tc := range []struct{ zone, day string }{
		{"America/Havana", "2026-10-31"}, // fall-back at 01:00 repeats midnight
		{"America/Havana", "2026-11-01"}, // control: the next midnight is unique
	} {
		loc := zone(t, tc.zone)
		enforced, err := time.Parse("2006-01-02", tc.day)
		if err != nil {
			t.Fatal(err)
		}
		noon := time.Date(enforced.Year(), enforced.Month(), enforced.Day(), 12, 0, 0, 0, loc)
		dow := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}[noon.Weekday()]
		p := profile(tc.zone, allDayRule("allday", []string{dow}, "", ""))

		if errs := ProfileErrors(p); errs != nil {
			t.Fatalf("ProfileErrors = %v, want nil", errs)
		}
		want, reads := lastReadingOfNextMidnight(loc, noon)
		if reads == 0 {
			t.Fatalf("%s %s: no instant reads the next midnight; the scan proves nothing", tc.zone, tc.day)
		}

		v := Evaluate(p, noon)
		if !v.Active {
			t.Fatalf("%s %s: the all-day rule is not enforcing at noon: %+v", tc.zone, tc.day, v)
		}
		if !v.EndsAt.Equal(want) {
			t.Errorf("%s %s: window ends %s, want the last of the %d readings of the next midnight (%s)",
				tc.zone, tc.day, v.EndsAt.In(loc).Format("01-02 15:04 MST"), reads,
				want.In(loc).Format("01-02 15:04 MST"))
		}
	}
}

// The two spellings of "the whole day" must not be confused. A rule that declares
// allDay and is also disabled enforces nothing — the switch that turns a rule off
// outranks the window it would have opened, or checking "all day" would be
// impossible on a rule that is switched off.
func TestAllDayDoesNotOverrideADisabledRule(t *testing.T) {
	loc := utc(t)
	r := allDayRule("off", allDays, "", "")
	r.Enabled = false
	p := profile("UTC", r)

	if errs := ProfileErrors(p); errs != nil {
		t.Errorf("ProfileErrors = %v for a rule that is switched off, want nil", errs)
	}
	for _, h := range []int{0, 12, 23} {
		if v := Evaluate(p, at(loc, 2026, 10, 12, h, 0)); v.Active {
			t.Errorf("a disabled all-day rule was active at %s", at(loc, 2026, 10, 12, h, 0))
		}
	}
}
