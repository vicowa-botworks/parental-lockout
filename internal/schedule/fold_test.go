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
