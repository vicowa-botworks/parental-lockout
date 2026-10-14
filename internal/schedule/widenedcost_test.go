package schedule

import (
	"testing"
	"time"
)

// The widened day takes the LAST reading of its closing midnight, and that choice is
// stated in schedule.go with numbers. This pins them, because a comment that cites a
// figure nothing measures is worse than no comment: it reads like evidence and rots
// silently the day the database changes.
//
// Two claims, both measured over every canonical zone the host provides:
//
//   - the fail-open direction never happens. Taking the earliest end instead would
//     release minutes whose clock still reads the enforced day. No canonical zone does
//     this in 2026-2031; only the right/* leap-second variants reach the shape, and
//     TestWidenedDayEndsAtTheLastReadingOfTheNextMidnight pins those.
//   - the cost of the choice is bounded. Where the end moves at all, it moves by at
//     most an hour, and that hour reads the FOLLOWING day -- which is why the choice
//     is safe to make in the holding direction even in a canonical zone.
func TestWidenedDayCostIsBoundedAndNeverFailsOpen(t *testing.T) {
	const years = 6 // 2026..2031, the span the comment claims
	zones := tzZones(t)
	if len(zones) == 0 {
		t.Fatal("no zones to measure; the bounds cannot be pinned")
	}

	var scanned, moved, freedAnywhere int
	var worstCost time.Duration
	var movedZones, freedZones []string
	for _, z := range zones {
		loc, err := time.LoadLocation(z)
		if err != nil {
			// tzZones lists names a config can meaningfully name. One that will not
			// load is a broken database, not a zone to step over.
			t.Fatalf("zone %s listed but will not load: %v", z, err)
		}
		scanned++
		for yr := 2026; yr < 2026+years; yr++ {
			// Only a day adjacent to an offset change can have an ambiguous next
			// midnight, so the change days plus their neighbour are the whole search.
			for _, dc := range changedDaysIn(loc, yr) {
				for off := -1; off <= 1; off++ {
					d := dc.day.AddDate(0, 0, off)
					y, mo, dd := d.Date()
					first := wallInstant(loc, y, mo, dd+1, 0, false)
					last := wallInstant(loc, y, mo, dd+1, 0, true)
					if last.Equal(first) {
						continue // the midnight is unique; the flag cannot matter
					}
					moved++
					if cost := last.Sub(first); cost > worstCost {
						worstCost = cost
					}
					if len(movedZones) < 40 {
						movedZones = append(movedZones, z+" "+d.In(loc).Format("2006-01-02"))
					}
					// Minutes the earlier end would release that still read day dd.
					for c := first; c.Before(last); c = c.Add(time.Minute) {
						if cy, cm, cd := c.In(loc).Date(); cy == y && cm == mo && cd == dd {
							freedAnywhere++
							if len(freedZones) < 10 {
								freedZones = append(freedZones, z+" "+d.In(loc).Format("2006-01-02"))
							}
							break
						}
					}
				}
			}
		}
	}

	// Unreachable today: len(zones) == 0 fatals above, and every zone either loads and
	// increments scanned or fatals. Kept as a tripwire, because the only edit that makes
	// it reachable is the tempting one — turning the load failure into a `continue` so a
	// broken database stops failing the suite. That edit would otherwise turn this into a
	// test that measures nothing and reports green.
	if scanned == 0 {
		t.Fatal("no zone loaded; nothing was measured")
	}
	if freedAnywhere != 0 {
		t.Errorf("%d canonical zone-days FREE a minute of the enforced day when the widened window takes the earliest end (%v) — the fail-open the latest end exists to prevent has arrived in canonical data, and the comment in schedule.go is now wrong",
			freedAnywhere, freedZones)
	}
	// The comment says the canonical cost is 60 minutes. If a future database moves a
	// canonical clock by more than an hour, that sentence has to be re-measured, so an
	// increase is a failure rather than a stale comment.
	if worstCost > time.Hour {
		t.Errorf("the widened end moves by %s in canonical data; schedule.go states the cost as 60 minutes", worstCost)
	}
	t.Logf("canonical zones scanned=%d zone-days whose widened end moves=%d worst cost=%v freed-minutes=%d",
		scanned, moved, worstCost.Round(time.Second), freedAnywhere)
	if moved > 0 {
		t.Logf("zones whose widened end moves: %v", movedZones)
	} else {
		// Not a failure: it means the flag cannot matter in canonical data on this
		// database, which is the safe direction. But it must be said, because "the
		// cost is 60 minutes in Havana" would then be a claim this host cannot check.
		t.Logf("no canonical zone-day moves here: the 60-minute cost is only reachable on a database with Havana/Azores")
	}
}

// changedDaysIn returns every day of the given year in the zone whose length is not
// twenty-four hours. changedDays is the same scan pinned to 2026.
func changedDaysIn(loc *time.Location, yr int) []dayChange {
	var out []dayChange
	base := time.Date(yr, 1, 1, 0, 0, 0, 0, loc)
	for i := 0; i < 366; i++ {
		a, b := base.AddDate(0, 0, i), base.AddDate(0, 0, i+1)
		// 366 iterations are needed so a leap year's Dec 31 is examined at all — with
		// 365 the last day is never the `a` of any pair. But on a 365-day year the extra
		// iteration lands on Jan 1 of yr+1, and a zone whose New Year transition falls
		// on that instant would be counted twice: once here under yr, once when the
		// caller reaches yr+1. Double-counting inflates `moved` and could report the
		// same zone twice in `movedZones`. No zone on the current database changes its
		// offset on Jan 1, so this is latent — but "latent" is only true of the database
		// this was measured on, and the count is the thing the comment cites.
		if a.Year() != yr {
			continue
		}
		if l := b.Sub(a); l != 24*time.Hour {
			out = append(out, dayChange{day: a, length: l})
		}
	}
	return out
}

// changedDaysIn's own contract: every day it returns belongs to the year it was asked
// about.
//
// The leak is not hypothetical. Measured on this host's database, Africa/Sao_Tome changes
// its offset on 2018-01-01 (a 23-hour day) and on 2019-01-01 (25 hours), and 2017 is a
// 365-day year — so the helper's 366th iteration lands on 2018-01-01 and reports it as a
// 2017 change day. The caller loops years in order, so it then counts that same zone-day
// a second time when it reaches 2018, inflating `moved` and possibly naming the zone twice
// in `movedZones`. The cost figures this file exists to pin are what get wrong.
//
// Asserted as a property of the helper rather than left to the figures, because the
// figures only go wrong on a database that happens to hold such a zone inside the scanned
// span — and the span is chosen for the claim being pinned, not for this one.
func TestChangedDaysInNeverReportsAnotherYearsDay(t *testing.T) {
	probes := []string{"Africa/Sao_Tome", "America/Lima", "America/Managua", "Africa/Bissau", "America/Danmarkshavn"}
	const lo, hi = 2014, 2022
	shape, checked := 0, 0
	for _, z := range probes {
		loc, err := time.LoadLocation(z)
		if err != nil {
			t.Fatalf("%s: %v", z, err)
		}
		for yr := lo; yr <= hi; yr++ {
			for _, dc := range changedDaysIn(loc, yr) {
				checked++
				d := dc.day.In(loc)
				if d.Month() == time.January && d.Day() == 1 {
					shape++
				}
				if d.Year() != yr {
					t.Errorf("%s: changedDaysIn(%d) returned %s, a day in %d — the caller counts that zone-day twice",
						z, yr, d.Format("2006-01-02"), d.Year())
				}
			}
		}
	}
	// A New Year change is the only input that can expose the leak, so a run without one
	// has proved nothing and must not be allowed to look like a pass.
	if shape == 0 {
		t.Fatalf("none of the %d probe zones had a New Year offset change in %d-%d (%d change days checked): the leak this pins cannot be reached on this database",
			len(probes), lo, hi, checked)
	}
	t.Logf("%d New Year change days among %d change days checked", shape, checked)
}
