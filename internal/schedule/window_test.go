package schedule

import (
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// windowOn's whole contract is one line: it never hands back a window you cannot
// be inside.
//
// Evaluate's containment test happens to make a zero-length or reversed window
// match nothing even without windowOn's own guard, so this cannot be reached
// through the public API — which is exactly why it is pinned here, at the
// primitive. The guard is what makes emptiness a property of the window rather
// than an accident of how the containment comparison occurs to be written; a
// later change to that comparison must not be able to turn "empty" into
// "everything" without a test saying so.
func TestWindowOnNeverReturnsAWindowYouCannotBeInside(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	cases := []struct {
		name       string
		zone       *time.Location
		day        time.Time
		start, end string
		wantWindow bool
	}{
		{
			name: "an ordinary window",
			zone: time.UTC, day: monday(), start: "09:00", end: "10:00",
			wantWindow: true,
		},
		{
			name: "one minute",
			zone: time.UTC, day: monday(), start: "09:00", end: "09:01",
			wantWindow: true,
		},
		{
			name: "across midnight",
			zone: time.UTC, day: monday(), start: "23:00", end: "01:00",
			wantWindow: true,
		},
		{
			// The shape a parent makes by typing the same time twice.
			name: "zero length",
			zone: time.UTC, day: monday(), start: "08:00", end: "08:00",
		},
		{
			// The shape the calendar makes: 02:59 does not exist on this day and
			// resolves to 03:59, after the 03:00 the window ends at.
			name: "reversed by a spring-forward",
			zone: berlin, day: time.Date(2026, 3, 29, 0, 0, 0, 0, berlin),
			start: "02:59", end: "03:00",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, ok := config.ParseClock(tc.start)
			if !ok {
				t.Fatalf("ParseClock(%q) failed", tc.start)
			}
			end, ok := config.ParseClock(tc.end)
			if !ok {
				t.Fatalf("ParseClock(%q) failed", tc.end)
			}

			var everyDay [7]bool
			for i := range everyDay {
				everyDay[i] = true
			}
			r := compiled{startMin: start, endMin: end, days: everyDay}

			w, ok := r.windowOn(tc.day, tc.zone)
			if !ok {
				if tc.wantWindow {
					t.Fatalf("windowOn refused to build %s-%s on %s", tc.start, tc.end, tc.day)
				}
				return
			}
			if !tc.wantWindow {
				t.Fatalf("windowOn returned [%s, %s) for an empty window", w.start, w.end)
			}

			// Anything it does hand back has to be a real interval with a real
			// instant inside it, or the guard above is being satisfied by
			// something that is not a window.
			if !w.end.After(w.start) {
				t.Fatalf("returned a window whose end is not after its start: [%s, %s)", w.start, w.end)
			}
			if inside := w.end.Add(-time.Nanosecond); inside.Before(w.start) {
				t.Fatalf("returned a window with no instant inside it: [%s, %s)", w.start, w.end)
			}
		})
	}
}

// The same property over every window shape: whatever the guard decides, it must
// never hand back something you cannot be inside.
//
// Every minute of the day against every minute of the day, on the two Berlin days
// where the clock makes this interesting and one ordinary UTC day. The compiled
// rule is built directly rather than through compile, because the property under
// test is windowOn's and compile only allocates; a 1440x1440 grid through it
// would make this the slowest test in the repository for no extra coverage.
func TestWindowOnIsSoundForEveryShape(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	var everyDay [7]bool
	for i := range everyDay {
		everyDay[i] = true
	}

	type dayIn struct {
		loc *time.Location
		day time.Time
	}
	days := []dayIn{
		{berlin, time.Date(2026, 3, 29, 0, 0, 0, 0, berlin)},  // spring forward
		{berlin, time.Date(2026, 10, 25, 0, 0, 0, 0, berlin)}, // fall back
		{time.UTC, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)},
	}

	for _, d := range days {
		for start := 0; start < 24*60; start++ {
			for end := 0; end < 24*60; end++ {
				r := compiled{startMin: start, endMin: end, days: everyDay}

				w, ok := r.windowOn(d.day, d.loc)
				if !ok {
					continue
				}
				if !w.end.After(w.start) {
					t.Fatalf("%s-%s on %s in %s: windowOn returned [%s, %s), which contains nothing",
						clock(start), clock(end), d.day.Format("2006-01-02"), d.loc, w.start, w.end)
				}
			}
		}
	}
}

func monday() time.Time {
	return time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC) // a Monday
}

// A window that crosses midnight on the last day of a month.
//
// The end is computed as "the day after the start" by adding one to the day of
// the month, which for the 31st of a 31-day month is the 32nd -- a date that does
// not exist and that the calendar silently rolls into the next month. That is the
// answer wanted here, but only if the roll happens once: a resolver that anchors
// the day before rolling it can land on the 1st of the next month at the wrong
// hour, or on the 31st again, and either way the overnight window closes hours
// early with the child asleep and the screen unlocked.
//
// The three boundaries that a day-of-month plus-one can get wrong are a 31-day
// month, a 30-day month, and December.
func TestWindowOnCrossesMonthAndYearBoundaries(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")
	p := profile("Europe/Berlin", daily("22:00", "06:00"))

	cases := []struct {
		name    string
		y       int
		mo      time.Month
		d       int
		hourMin int
		want    bool
	}{
		{"May 31 to June 1, inside", 2026, time.June, 1, 2 * 60, true},
		{"May 31 to June 1, after the end", 2026, time.June, 1, 7 * 60, false},
		{"April 30 to May 1, inside", 2026, time.May, 1, 2 * 60, true},
		{"December 31 to January 1, inside", 2026, time.January, 1, 2 * 60, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The day the window is standing in, not the day it started on: the
			// evaluator is asked about now, and it is the one that has to walk back
			// to yesterday across the boundary.
			now := time.Date(tc.y, tc.mo, tc.d, tc.hourMin/60, tc.hourMin%60, 0, 0, berlin)
			if got := Evaluate(p, now).Active; got != tc.want {
				t.Errorf("%s: Evaluate(%s) = %v, want %v",
					tc.name, now.Format(time.RFC3339), got, tc.want)
			}
		})
	}

	// December 31 2026 is a Thursday, so the overnight window that opens that
	// evening closes on New Year's Day. Check the closing instant itself, in the
	// zone, because a month roll that lands a day out still looks right an hour
	// either side of it.
	nye := profile("Europe/Berlin", rule("nye", []string{"thu"}, "22:00", "06:00"))
	before := time.Date(2027, time.January, 1, 5, 59, 0, 0, berlin)
	after := time.Date(2027, time.January, 1, 6, 0, 0, 0, berlin)
	if v := Evaluate(nye, before); !v.Active {
		t.Errorf("the New Year window had already closed at %s: %+v", before.Format(time.RFC3339), v)
	}
	if v := Evaluate(nye, after); v.Active {
		t.Errorf("the New Year window was still holding at its own end %s", after.Format(time.RFC3339))
	}
	if v := Evaluate(nye, before); v.EndsAt.Day() != 1 || v.EndsAt.Month() != time.January {
		t.Errorf("the window that started Thursday 31 December ends on %s, want 1 January",
			v.EndsAt.Format("2006-01-02"))
	}
}

// An overnight window whose end lands in the hour the clock repeats.
//
// This is the month-boundary code path -- the end is computed as "the day after
// the start" -- meeting the path where a written time names two instants. The
// window starts on one date and ends on the next, so the end is resolved from a
// day-of-month that had to be rolled forward, and it is resolved into an hour that
// arrives twice.
//
// It has to hold to the second pass. Resolving the end to the first one releases
// the child an hour early, in the middle of the night the rule was written for,
// and the parent sees a schedule that says 02:30 while the machine unlocked at the
// first 02:30 the clock showed.
func TestWindowOnOvernightEndInRepeatedHour(t *testing.T) {
	for _, z := range []string{"Europe/Berlin", "America/New_York", "America/Havana", "Australia/Sydney"} {
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)
			day, ok := fallBackDay(loc)
			if !ok {
				t.Skipf("%s has no fall-back day in 2026", z)
			}
			first, second := repeatedHour(t, loc, day)
			endClock := second.Format("15:04")

			// The window opens on the evening before the change, so its end is the
			// day that had to be rolled forward.
			start := day.AddDate(0, 0, -1)
			if _, _, d := first.In(loc).Date(); d == start.Day() {
				t.Skipf("%s repeats %s on the night the change begins, not the morning after", z, endClock)
			}
			// The config's own day spelling, not time.Weekday's: a name the
			// validator rejects would silently drop the rule and make every
			// assertion below pass by finding nothing.
			dayName := strings.ToLower(start.In(loc).Weekday().String()[:3])
			if !config.IsDay(dayName) {
				t.Fatalf("%q is not a day the config accepts", dayName)
			}
			p := profile(z, rule("overnight", []string{dayName}, "21:00", endClock))
			if errs := ProfileErrors(p); errs != nil {
				t.Fatalf("the rule is not being enforced: %v", errs)
			}

			if v := Evaluate(p, second.Add(-time.Minute)); !v.Active {
				t.Errorf("released at %s, a minute before the window ends at the second %s (%s)",
					second.Add(-time.Minute).In(loc).Format(time.RFC3339), endClock, second.Format(time.RFC3339))
			} else if !v.EndsAt.Equal(second) {
				t.Errorf("EndsAt = %s, want %s (the second %s)",
					v.EndsAt.Format(time.RFC3339), second.Format(time.RFC3339), endClock)
			}
			if Evaluate(p, second).Active {
				t.Errorf("still holding at %s, the second %s, which is the end",
					second.In(loc).Format(time.RFC3339), endClock)
			}
			// The first pass is inside the window, not at its end.
			if v := Evaluate(p, first.Add(time.Minute)); !v.Active {
				t.Errorf("released at the first %s (%s): EndsAt = %s, want %s",
					first.In(loc).Format(time.RFC3339), first.Format(time.RFC3339),
					v.EndsAt.Format(time.RFC3339), second.Format(time.RFC3339))
			}
		})
	}
}
