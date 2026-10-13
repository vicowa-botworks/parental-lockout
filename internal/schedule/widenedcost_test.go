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
		if l := b.Sub(a); l != 24*time.Hour {
			out = append(out, dayChange{day: a, length: l})
		}
	}
	return out
}
