package schedule

import (
	"testing"
	"time"
)

// Some zones have days that never happened. Pacific/Apia crossed the dateline on
// 2011-12-31 and jumped from Friday to Sunday; Pacific/Kiritimati did the same in
// 1995; Pacific/Kwajalein did it in 1993. A rule written against one of those
// dates has to land somewhere sensible, and the place it lands is the day after
// the gap.
//
// The evaluator's answer must also still be *found*. A window that opens the
// evening before the gap and is written to close on the deleted date closes two
// local dates after it opened, and the walk that looks for the window holding now
// only looked back one day. The account sat unlocked for the twenty-two hours in
// the middle while the rule was still on the books — fail-open, on the one path
// where the calendar itself moves the goalposts.
//
// The oracle here is the clock: it finds the instants that read a given local date
// and time by looking at nothing but the clock, so a bug in the evaluator's own
// date arithmetic cannot talk this test out of running.
func TestWindowSurvivesADeletedCalendarDay(t *testing.T) {
	zones := dateDeletingZones(t)
	if len(zones) == 0 {
		t.Skip("no zone in this tz database deletes a calendar day")
	}
	t.Logf("%d zones have crossed the dateline: %v", len(zones), zones)

	for _, z := range zones {
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)
			gap, ok := deletedDate(loc)
			if !ok {
				t.Errorf("%s jumps the clock by a day but has no deleted date between 1990 and 2020; "+
					"widen the search rather than letting this case go untested", z)
				return
			}
			dayBefore := gap.AddDate(0, 0, -1)

			// A window that opens the evening before the gap and is written to close
			// on the gap itself: the shape that has to survive it.
			p := profile(z, rule("overnight",
				[]string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}, "22:00", "21:59"))
			if errs := ProfileErrors(p); errs != nil {
				t.Fatalf("the rule is not being enforced: %v", errs)
			}

			start, ok := earliestReading(loc, dayBefore, 22*60)
			if !ok {
				t.Fatalf("nothing reads 22:00 on %s", dayBefore.Format("2006-01-02"))
			}
			// "The next day" is the next day that exists.
			endDate := nextExistingDate(loc, dayBefore)
			if !endDate.After(dayBefore) {
				t.Fatalf("the day after %s reads as %s", dayBefore.Format("2006-01-02"), endDate.Format("2006-01-02"))
			}
			end, ok := latestReading(loc, endDate, 21*60+59)
			if !ok {
				t.Fatalf("nothing reads 21:59 on %s", endDate.Format("2006-01-02"))
			}
			if !end.After(start) {
				t.Fatalf("window end %s is not after its start %s", end.Format(time.RFC3339), start.Format(time.RFC3339))
			}

			// The whole window, minute by minute, plus an hour either side.
			//
			// Membership is tested against the union of every window that could be
			// standing, not just the one this zone's gap produces. The rule fires on
			// every day, so the window that opened two evenings earlier can still be
			// holding; a model that looks at one window reports the hour before it
			// opens as a disagreement when the evaluator is in fact correctly inside
			// its neighbour.
			windows := nearbyWindows(loc, gap, 22*60, 21*60+59)
			if len(windows) == 0 {
				t.Fatalf("no windows could be modelled around %s", gap.Format("2006-01-02"))
			}

			var firstMiss time.Time
			missed := 0
			for m := -60; m <= int(end.Sub(start)/time.Minute)+60; m++ {
				now := start.Add(time.Duration(m) * time.Minute)
				if got := Evaluate(p, now).Active; got != inside(windows, now) {
					if missed == 0 {
						firstMiss = now
					}
					missed++
				}
			}
			if missed > 0 {
				t.Errorf("%d minutes disagree with the clock over %s..%s; first at %s (local %s): the window opened %s and closes %s",
					missed, start.Format(time.RFC3339), end.Format(time.RFC3339),
					firstMiss.Format(time.RFC3339), firstMiss.In(loc).Format("2006-01-02 15:04:05 MST Mon"),
					start.Format(time.RFC3339), end.Format(time.RFC3339))
			}

			// The specific failure, spelled out: the middle of the window is inside
			// it, and it sits two local dates past the day the window opened — which
			// is exactly how far the walk has to look.
			middle := start.Add(end.Sub(start) / 2)
			if !Evaluate(p, middle).Active {
				t.Errorf("not locked at the middle of the window, %s (local %s)",
					middle.Format(time.RFC3339), middle.In(loc).Format("2006-01-02 15:04:05 MST"))
			}
			if span := localDatesBetween(loc, start, end); span < 2 {
				t.Errorf("this zone's deleted date only spans %d local dates, so the case is not being exercised", span)
			}
		})
	}
}

// localDatesBetween is how many civil dates apart two instants are in loc — the
// distance the walk has to cover, counted on the calendar rather than in hours.
func localDatesBetween(loc *time.Location, a, b time.Time) int {
	ay, amo, ad := a.In(loc).Date()
	by, bmo, bd := b.In(loc).Date()
	da := time.Date(ay, amo, ad, 0, 0, 0, 0, time.UTC)
	db := time.Date(by, bmo, bd, 0, 0, 0, 0, time.UTC)
	return int(db.Sub(da).Hours() / 24)
}

type span struct{ start, end time.Time }

// nearbyWindows models the windows a daily startMin-endMin rule puts around date,
// from the clock alone: each window opens at the earliest instant reading
// startMin on its day and closes at the latest instant reading endMin on the next
// day that exists.
func nearbyWindows(loc *time.Location, around time.Time, startMin, endMin int) []span {
	var out []span
	for back := 4; back >= -1; back-- {
		d := around.AddDate(0, 0, -back)
		st, ok := earliestReading(loc, d, startMin)
		if !ok {
			continue
		}
		en, ok := latestReading(loc, nextExistingDate(loc, d), endMin)
		if !ok || !en.After(st) {
			continue
		}
		out = append(out, span{st, en})
	}
	return out
}

func inside(ws []span, t time.Time) bool {
	for _, w := range ws {
		if !t.Before(w.start) && t.Before(w.end) {
			return true
		}
	}
	return false
}

// dateDeletingZones finds every zone whose clock has ever jumped forward by a day
// or more, which is what deletes a calendar date.
//
// The zones are discovered rather than listed. The five that have done it are
// Pacific history — Apia, Kiritimati, Kwajalein, Fakaofo, Kanton crossing the
// dateline — and a future tzdata could add another with no reason to remind
// anyone that a test was written against a list of names.
func dateDeletingZones(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, z := range tzZones(t) {
		loc, err := time.LoadLocation(z)
		if err != nil {
			continue
		}
		start := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
		prev := offsetAt(start, loc)
		hit := false
		for h := 24; h <= 24*366*130 && !hit; h += 24 {
			tt := start.Add(time.Duration(h) * time.Hour)
			o := offsetAt(tt, loc)
			if o == prev {
				continue
			}
			if o-prev >= 24*time.Hour {
				hit = true
			}
			prev = o
		}
		if hit {
			out = append(out, z)
		}
	}
	return out
}

// deletedDate returns a local date in loc that no instant ever reads, searching
// 1970 to 2035.
//
// The candidates are enumerated as civil dates in UTC and then asked of the zone.
// Deriving them by converting a UTC day into loc does not work: the conversion
// skips the missing date itself, so the search walks straight past the very thing
// it is looking for and reports that no zone has one.
func deletedDate(loc *time.Location) (time.Time, bool) {
	for d := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC); d.Year() <= 2035; d = d.AddDate(0, 0, 1) {
		y, mo, day := d.Date()
		found := false
		for h := -48; h <= 48 && !found; h += 6 {
			tt := time.Date(y, mo, day, 12, 0, 0, 0, time.UTC).Add(time.Duration(h) * time.Hour)
			ty, tmo, td := tt.In(loc).Date()
			if ty == y && tmo == mo && td == day {
				found = true
			}
		}
		if !found {
			return time.Date(y, mo, day, 12, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}

// nextExistingDate returns the first local date strictly after d that some instant
// actually reads.
func nextExistingDate(loc *time.Location, d time.Time) time.Time {
	for next := d.AddDate(0, 0, 1); ; next = next.AddDate(0, 0, 1) {
		y, mo, day := next.Date()
		for h := -48; h <= 48; h += 6 {
			tt := time.Date(y, mo, day, 12, 0, 0, 0, time.UTC).Add(time.Duration(h) * time.Hour)
			ty, tmo, td := tt.In(loc).Date()
			if ty == y && tmo == mo && td == day {
				return time.Date(y, mo, day, 12, 0, 0, 0, time.UTC)
			}
		}
	}
}

// earliestReading and latestReading find the first and last instants whose clock in
// loc reads the given local date and minute, by scanning the clock itself.
func earliestReading(loc *time.Location, date time.Time, min int) (time.Time, bool) {
	y, mo, day := date.Date()
	centre := time.Date(y, mo, day, min/60, min%60, 0, 0, time.UTC)
	var best time.Time
	for m := -48 * 60; m <= 48*60; m++ {
		c := centre.Add(time.Duration(m) * time.Minute)
		if readsTheClock(c, loc, y, mo, day, min) && (best.IsZero() || c.Before(best)) {
			best = c
		}
	}
	return best, !best.IsZero()
}

func latestReading(loc *time.Location, date time.Time, min int) (time.Time, bool) {
	y, mo, day := date.Date()
	centre := time.Date(y, mo, day, min/60, min%60, 0, 0, time.UTC)
	var best time.Time
	for m := -48 * 60; m <= 48*60; m++ {
		c := centre.Add(time.Duration(m) * time.Minute)
		if readsTheClock(c, loc, y, mo, day, min) && (best.IsZero() || c.After(best)) {
			best = c
		}
	}
	return best, !best.IsZero()
}

func readsTheClock(t time.Time, loc *time.Location, y int, mo time.Month, day, min int) bool {
	ty, tmo, td := t.In(loc).Date()
	th, tm, _ := t.In(loc).Clock()
	return ty == y && tmo == mo && td == day && th*60+tm == min
}
