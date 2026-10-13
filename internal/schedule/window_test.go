package schedule

import (
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
			rs, errs := compile([]config.Rule{
				{ID: "r", Days: allDays, Start: tc.start, End: tc.end, Enabled: true},
			})
			if errs != nil {
				t.Fatalf("compile reported %v for a rule Validate accepts", errs)
			}

			w, ok := rs[0].windowOn(tc.day, tc.zone)
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
