package schedule

import (
	"testing"
	"time"
)

// instantsReading is an oracle: it finds every instant within two days of the
// target whose clock in loc reads the given calendar day and minute, by looking at
// the clock and nothing else.
//
// It exists because a precondition checked through the resolver cannot catch the
// resolver. The first version of these tests asked wallInstant whether 23:00
// arrived twice; with the unsound probe in place it answered "once", the test
// concluded the zone no longer behaved as described, and skipped — green while the
// curfew lost three hours. Anything that decides whether a test runs has to be
// derived from outside the code the test is about.
func instantsReading(t *testing.T, loc *time.Location, y int, mo time.Month, day, min int) []time.Time {
	t.Helper()
	// Two days either side: the widest gap between two readings of one wall time in
	// the tz database is 26h, so anything further out cannot read the same clock.
	centre := time.Date(y, mo, day, min/60, min%60, 0, 0, time.UTC)
	var out []time.Time
	for m := -48 * 60; m <= 48*60; m++ {
		c := centre.Add(time.Duration(m) * time.Minute)
		if readsTheClock(c, loc, y, mo, day, min) {
			out = append(out, c)
		}
	}
	return out
}

// Antarctica/Casey put its clocks back three hours every March from 2010 to 2023,
// which makes every wall time in the repeated stretch arrive twice — and three
// hours is wider than the hour anyone thinks about when they think about DST.
//
// It is the reason the fast path's probe is measured in the database's own bounds
// rather than in "DST moves an hour". With a two-hour probe the guard believed the
// reading was unique, and a window that starts in the repeated stretch got the
// later of its two readings: the rule below opened at the second 23:00, so the
// account sat unlocked through the first three hours of its own curfew.
//
// Casey stopped observing after March 2023, so this is not reachable from a live
// poll today. It is reachable from any date the evaluator is asked about that
// predates that — and the guard could not know that, because the bound it was
// using was a guess about the present.
func TestCaseyThreeHourFoldKeepsTheWholeWindow(t *testing.T) {
	loc := zone(t, "Antarctica/Casey")

	// 2021-03-13 is a Saturday, and 23:00 arrives twice on it.
	readings := instantsReading(t, loc, 2021, 3, 13, 23*60)
	if len(readings) != 2 {
		t.Fatalf("23:00 on 2021-03-13 in Casey reads %d times, want 2: %v",
			len(readings), formatAll(readings))
	}
	first, second := readings[0], readings[1]
	if gap := second.Sub(first); gap != 3*time.Hour {
		t.Fatalf("the two readings of 23:00 are %v apart, want 3h", gap)
	}

	p := profile("Antarctica/Casey", rule("sat", []string{"sat"}, "23:00", "23:59"))
	if errs := ProfileErrors(p); errs != nil {
		t.Fatalf("the rule is not being enforced: %v", errs)
	}

	// The resolver, both directions.
	if got := wallInstant(loc, 2021, 3, 13, 23*60, false); !got.Equal(first) {
		t.Errorf("earliest 23:00 = %s, want %s: a window starting here opens %v late",
			got.UTC().Format(time.RFC3339), first.Format(time.RFC3339), got.Sub(first))
	}
	if got := wallInstant(loc, 2021, 3, 13, 23*60, true); !got.Equal(second) {
		t.Errorf("latest 23:00 = %s, want %s", got.UTC().Format(time.RFC3339), second.Format(time.RFC3339))
	}

	// The product: locked from the first reading, not the second.
	if v := Evaluate(p, first); !v.Active {
		t.Errorf("not locked at the first 23:00 (%s): the child has the machine for the first three hours of the curfew",
			first.In(loc).Format("2006-01-02 15:04:05 MST"))
	} else if !v.StartedAt.Equal(first) {
		t.Errorf("StartedAt = %s, want %s", v.StartedAt.UTC().Format(time.RFC3339), first.Format(time.RFC3339))
	}

	// Between the two passes the clock reads *before* the written start: 22:59 on
	// the +08 side, three hours after the window opened. Releasing there is the
	// tempting correction — it agrees with the wall clock and contradicts the
	// policy. The window opened at the first 23:00 and closes at the last reading of
	// its end, so the fold is held. This is the instant the PR text originally got
	// wrong, and it is here so the text cannot drift again.
	middle := first.Add(3*time.Hour - time.Minute) // 14:59Z, whose clock reads 22:59 +08
	if v := Evaluate(p, middle); !v.Active {
		t.Errorf("released at %s, whose clock reads %s, three hours into a window that "+
			"opened at %s: the fold is held, not handed back",
			middle.UTC().Format(time.RFC3339), middle.In(loc).Format("15:04 MST"),
			first.UTC().Format(time.RFC3339))
	}

	// The window holds across the fold and closes at 23:59 on the +08 side, the
	// last reading the parent's words name — and the end is exclusive, so the
	// release lands on 23:59 itself rather than a minute later.
	end := instantsReading(t, loc, 2021, 3, 13, 23*60+59)
	if len(end) == 0 {
		t.Fatalf("23:59 reads no instants at all")
	}
	closesAt := end[len(end)-1]
	if v := Evaluate(p, closesAt.Add(-time.Minute)); !v.Active {
		t.Errorf("released a minute before the end at %s", closesAt.In(loc).Format("15:04 MST"))
	} else if !v.EndsAt.Equal(closesAt) {
		t.Errorf("EndsAt = %s, want %s", v.EndsAt.UTC().Format(time.RFC3339), closesAt.Format(time.RFC3339))
	}
	if v := Evaluate(p, closesAt); v.Active {
		t.Errorf("still holding at the end %s: %+v", closesAt.In(loc).Format("15:04 MST"), v)
	}
}

// A clock change bigger than a day is not hypothetical: Pacific/Kwajalein jumped
// twenty-three hours and Antarctica/Vostok moved seven hours in 1994. A window
// that starts on one side of one of those and ends on the other is the worst case
// the resolver has, and it is the one a probe sized in hours silently fails.
func TestVostokSevenHourChangeKeepsTheWholeWindow(t *testing.T) {
	loc := zone(t, "Antarctica/Vostok")

	readings := instantsReading(t, loc, 1994, 1, 31, 22*60)
	if len(readings) != 2 {
		t.Fatalf("22:00 on 1994-01-31 in Vostok reads %d times, want 2: %v",
			len(readings), formatAll(readings))
	}
	first, second := readings[0], readings[1]
	if second.Sub(first) < 6*time.Hour {
		t.Fatalf("the two readings of 22:00 are %v apart, want at least 6h", second.Sub(first))
	}

	p := profile("Antarctica/Vostok", rule("mon", []string{"mon"}, "22:00", "23:30"))
	if errs := ProfileErrors(p); errs != nil {
		t.Fatalf("the rule is not being enforced: %v", errs)
	}

	// 22:59 on the far side of the change is still inside the rule's own evening;
	// resolving the start to the near side opens the window hours late and leaves
	// the account unlocked through the whole of it.
	far := first.Add(59 * time.Minute)
	if v := Evaluate(p, far); !v.Active {
		t.Errorf("not locked at %s (%s): the window opened %v late",
			far.In(loc).Format("2006-01-02 15:04:05 MST"), far.Format(time.RFC3339), second.Sub(first))
	}
	if v := Evaluate(p, far); !v.StartedAt.Equal(first) {
		t.Errorf("StartedAt = %s, want the first 22:00 at %s",
			v.StartedAt.UTC().Format(time.RFC3339), first.Format(time.RFC3339))
	}
}

func formatAll(ts []time.Time) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.UTC().Format(time.RFC3339))
	}
	return out
}

// A fold lengthens a window that overlaps it, by exactly the fold's width.
//
// The parent wrote two clock times and the clock shows the ones inside the
// repeated stretch twice. The window opens at the first of them and closes at the
// last of the second, so the account is held for longer than the arithmetic on the
// two boxes suggests: a one-hour rule on Antarctica/Casey in 2021 holds for four.
//
// That direction is the only safe one. Cutting the window at the first pass instead
// would hand the screen back in the middle of the repeated hour, which is the
// fail-open this file exists to keep shut. What it costs is a child locked out for
// an hour the parent did not count, and the dashboard shows that honestly as long
// as it renders EndsAt as the instant it is rather than re-deriving it from the
// rule's text.
//
// The fold is located from the clock and its width asserted, not assumed: a tzdata
// edit that moves one of these turns the test red instead of quietly narrowing it.
func TestAFoldLengthensAWindowByItsOwnWidth(t *testing.T) {
	for _, tc := range []struct {
		zone     string
		year     int
		wantFold time.Duration
	}{
		{"Antarctica/Casey", 2021, 3 * time.Hour},
		{"Antarctica/Vostok", 1994, 7 * time.Hour},
		{"Antarctica/Troll", 2026, 2 * time.Hour},
		{"Europe/Berlin", 2026, time.Hour},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			loc := zone(t, tc.zone)
			day, fold := foldDay(t, loc, tc.year)
			if fold != tc.wantFold {
				t.Fatalf("%s %d: the fold on %s is %s wide, not %s -- this test's "+
					"expectations are written against a clock that has moved",
					tc.zone, tc.year, day.Format("2006-01-02"), fold, tc.wantFold)
			}

			first := firstRepeatedMinute(t, loc, day)
			start, end := hhmm(first), hhmm(first+60)
			p := profile(tc.zone, daily(start, end))

			probe := time.Date(day.Year(), day.Month(), day.Day(),
				first/60, first%60, 0, 0, loc).Add(time.Minute)
			v := Evaluate(p, probe)
			if !v.Active {
				t.Fatalf("free at %s, one minute into the written %s-%s on a %s fold",
					probe.Format(time.RFC3339), start, end, fold)
			}

			held := v.EndsAt.Sub(v.StartedAt)
			if want := time.Hour + fold; held != want {
				t.Errorf("the written hour holds for %s, want %s (the hour plus the %s fold)",
					held, want, fold)
			}
			if held < time.Hour {
				t.Errorf("held %s is shorter than the written hour: a fold must never "+
					"give back time the parent wrote", held)
			}

			// The release is the last instant that still reads the written end,
			// found by looking at the clock and nothing else. On a fold exactly one
			// hour wide the end minute sits just outside the repeated stretch and
			// has only one reading; the claim is the same either way.
			passes := instantsReading(t, loc, day.Year(), day.Month(), day.Day(), first+60)
			if len(passes) == 0 {
				t.Fatalf("nothing on %s in %s reads %s, so this zone no longer behaves "+
					"the way the window above assumes", end, day.Format("2006-01-02"), tc.zone)
			}
			if last := passes[len(passes)-1]; !v.EndsAt.Equal(last) {
				t.Errorf("released at %s, want the last instant reading %s, %s",
					v.EndsAt.Format(time.RFC3339), end, last.Format(time.RFC3339))
			}
		})
	}
}

// foldDay returns the day in the given year whose clock went backwards, and by how
// much. It reads the length of each local day and nothing else, so it cannot agree
// with a broken resolver and hide the case it is meant to set up.
func foldDay(t *testing.T, loc *time.Location, year int) (time.Time, time.Duration) {
	t.Helper()
	base := time.Date(year, 1, 1, 0, 0, 0, 0, loc)
	for i := 0; i < 366; i++ {
		a, b := base.AddDate(0, 0, i), base.AddDate(0, 0, i+1)
		if l := b.Sub(a); l > 24*time.Hour {
			return a, l - 24*time.Hour
		}
	}
	t.Fatalf("%s has no fold in %d", loc, year)
	return time.Time{}, 0
}

// firstRepeatedMinute is the first minute past local midnight that the clock shows
// twice on the day, found by counting instants that read it.
func firstRepeatedMinute(t *testing.T, loc *time.Location, day time.Time) int {
	t.Helper()
	y, mo, d := day.Date()
	for min := 0; min < 24*60; min++ {
		if len(instantsReading(t, loc, y, mo, d, min)) == 2 {
			return min
		}
	}
	t.Fatalf("no minute on %s in %s is shown twice", day.Format("2006-01-02"), loc)
	return -1
}

// hhmm writes minutes past midnight the way a parent types them.
func hhmm(min int) string {
	return time.Date(0, 1, 1, min/60, min%60, 0, 0, time.UTC).Format("15:04")
}
