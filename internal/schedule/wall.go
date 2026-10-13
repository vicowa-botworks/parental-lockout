package schedule

import (
	"sort"

	// The tz database, compiled into the binary.
	//
	// A profile's timezone is not decoration: read in the wrong zone, a
	// school-hours rule locks out the wrong hours and says nothing while it does.
	// Without an embedded database, LoadLocation fails on any host with no
	// /usr/share/zoneinfo — the slim and distroless images a root daemon actually
	// gets packaged into — and then every profile on the machine falls back to the
	// machine's own zone at once. A typo in one profile's zone is a bad hour for
	// one child; a missing database is that for all of them, silently.
	//
	// It costs a few hundred kilobytes. Go still prefers the host's own database
	// when it has one, so an installed system keeps tracking its tzdata updates.
	"time"
	_ "time/tzdata"
)

// A rule's endpoints are wall-clock times: "21:00" means whatever instant the
// clock in the child's room reads as 21:00. On almost every day that is exactly
// one instant and time.Date finds it. Twice a year, in most of the world, it is
// not one instant, and time.Date's answer is wrong in the direction that matters.
//
// A clock going back repeats an hour, so "01:30" names two instants an hour
// apart. time.Date returns the first, which means a window that ends at 01:30
// releases the child while the clock still reads 01:30 for the second time. In
// America/New_York on 2026-11-01 a curfew written 22:00-01:00 released at the
// first 01:00 and left the child free for a full hour inside a window the parent
// wrote.
//
// A clock jumping forward deletes an hour, so "02:30" names no instant at all.
// time.Date shifts it by the size of the gap — forward in Europe, backward in
// North America — so one config file opens a window half an hour late in Berlin
// and an hour early in New York. Late is the fail-open.
//
// Both resolve toward holding the child, which is the direction this product
// takes every ambiguity: a start takes the earliest instant that reads as the
// written time, an end takes the last, and a time that does not exist resolves to
// the first instant after it. None of these invents a lockout longer than the
// parent wrote by more than the clock itself made ambiguous, and none of them
// hands back a minute the parent did not ask to keep.
//
// wallInstant is where that decision lives; windowOn is the only caller.

// wallInstant returns the instant in loc on the given calendar day whose wall
// clock reads min minutes past midnight, choosing the earliest such instant, or
// the latest one when last is set.
//
// day may overflow the month — day 32 of January is 1 February — the same way
// time.Date allows.
func wallInstant(loc *time.Location, y int, mo time.Month, day, min int, last bool) time.Time {
	// Normalise the date through noon: midnight is itself the thing that can go
	// missing, and a midnight-anchored normalisation would shift the whole day in
	// the four zones whose clock change lands on it.
	noon := time.Date(y, mo, day, 12, 0, 0, 0, loc)
	y, mo, day = noon.Date()

	h, mm := min/60, min%60
	t := time.Date(y, mo, day, h, mm, 0, 0, loc)

	// Almost every instant on almost every day has exactly one reading: if the
	// offset is the same two hours either side, the wall clock is a one-to-one
	// map here and time.Date's answer is the only instant there is. Checking this
	// first keeps the slow path off the 363 days a year that do not need it.
	if readsAs(t, loc, y, mo, day, min) &&
		offsetAt(t.Add(-2*time.Hour), loc) == offsetAt(t.Add(2*time.Hour), loc) {
		return t
	}

	// Otherwise collect every instant that reads as the written time. The offsets
	// a day can be in are the ones in effect around it, so sampling the offsets
	// and converting back through each of them finds every candidate; a repeated
	// hour yields two, and nothing yields more than two in any zone in the tz
	// database.
	var matches, candidates []time.Time
	for _, off := range offsetsAround(t, loc) {
		c := time.Date(y, mo, day, h, mm, 0, 0, time.UTC).Add(-off)
		candidates = append(candidates, c)
		if readsAs(c, loc, y, mo, day, min) {
			matches = append(matches, c)
		}
	}

	var best time.Time
	for _, c := range matches {
		if best.IsZero() || (last && c.After(best)) || (!last && c.Before(best)) {
			best = c
		}
	}
	if !best.IsZero() {
		return best
	}

	// Nothing reads as the written time, so the clock jumped over it. The
	// candidates bracket the jump — one sits on each side of it, at the distance
	// the clock moved — so bisecting between them finds the jump itself to the
	// second. That instant is the first moment at which the clock is past what the
	// parent wrote, which is the answer that starts holding the child soonest.
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Before(candidates[j]) })
	lo, hi := candidates[0], candidates[len(candidates)-1]
	for hi.Sub(lo) > time.Second {
		mid := lo.Add(hi.Sub(lo) / 2)
		if offsetAt(mid, loc) == offsetAt(lo, loc) {
			lo = mid
		} else {
			hi = mid
		}
	}
	// Every change in the tz database falls on a whole second, so truncating the
	// upper bound lands on the change exactly. Without this, two endpoints
	// resolving to the same jump from different written times land a fraction
	// apart, and a window that should be empty comes out a fraction wide.
	return hi.Truncate(time.Second)
}

// readsAs reports whether the instant t shows the given calendar day and clock
// reading in loc.
//
// Every instant this is asked about is within a day of the requested date, so the
// day and the clock reading together already pin the year: nothing can be a year
// away and still claim the same month, day and minute. The year is checked anyway
// because the alternative is a wrong window that only shows up in a zone nobody
// thought to test, and the comparison costs nothing.
func readsAs(t time.Time, loc *time.Location, y int, mo time.Month, day, min int) bool {
	ly, lmo, ld := t.In(loc).Date()
	lh, lm, _ := t.In(loc).Clock()
	return ly == y && lmo == mo && ld == day && lh*60+lm == min
}

// offsetAt is the zone offset in effect at an instant.
func offsetAt(t time.Time, loc *time.Location) time.Duration {
	_, seconds := t.In(loc).Zone()
	return time.Duration(seconds) * time.Second
}

// offsetsAround samples the offsets in effect around t. A day can hold at most
// two transitions in any zone in the tz database, and a transition moves the
// clock by at most two hours, so sampling every twelve hours across three days
// cannot miss one that matters to an instant at the centre.
func offsetsAround(t time.Time, loc *time.Location) []time.Duration {
	var out []time.Duration
	for h := -36; h <= 36; h += 12 {
		o := offsetAt(t.Add(time.Duration(h)*time.Hour), loc)
		seen := false
		for _, have := range out {
			if have == o {
				seen = true
				break
			}
		}
		if !seen {
			out = append(out, o)
		}
	}
	return out
}
