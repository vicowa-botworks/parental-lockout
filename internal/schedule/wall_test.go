package schedule

import (
	"testing"
	"time"
)

// The ticket's DST criterion is "01:30 happens twice". Every other DST test in
// this package uses a window that straddles the repeated hour, which passes with
// the endpoint landing on either pass — so none of them could tell the two passes
// apart, and the fail-open they were written to guard against stayed open.
//
// These put an endpoint exactly on the ambiguous minute.

// A curfew that ends at a clock reading the fall-back night repeats.
//
// The parent wrote one end time and the clock shows it twice. Releasing on the
// first pass hands the child back the screen in the middle of the night the rule
// was written for, in the hour the clock itself added — so the window runs to the
// last instant that reads the written time, and the child goes when the clock
// moves past it, exactly as on a day with no change.
func TestWallAmbiguousEndHoldsToTheLastPass(t *testing.T) {
	for _, z := range ambiguousZones {
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)
			day, ok := fallBackDay(loc)
			if !ok {
				t.Skipf("%s has no fall-back day in 2026", z)
			}
			first, second := repeatedHour(t, loc, day)
			endClock := second.Format("15:04")

			// The night before the change, running into it.
			// Every weekday, because on a zone whose change lands on midnight the
			// window ending at the repeated reading starts on the day before, and
			// on one whose change lands at eleven at night it does not.
			p := profile(z, daily("22:00", endClock))

			v := Evaluate(p, second.Add(-time.Minute))
			if !v.Active {
				t.Fatalf("released a minute before the second %s on %s: the clock still reads %s and the rule said %s until %s",
					endClock, day.Format("2006-01-02"), endClock, "22:00", endClock)
			}
			if !v.EndsAt.Equal(second) {
				t.Errorf("EndsAt = %s, want %s (the second %s)",
					v.EndsAt.Format(time.RFC3339), second.Format(time.RFC3339), endClock)
			}
			if Evaluate(p, second).Active {
				t.Errorf("still holding at the second %s, which is the end and the end is exclusive", endClock)
			}

			// The first pass must not be mistaken for the end.
			if v := Evaluate(p, first); !v.Active {
				t.Errorf("released at the first %s: EndsAt = %s, want %s",
					endClock, v.EndsAt.Format(time.RFC3339), second.Format(time.RFC3339))
			}
		})
	}
}

// The mirror case: a window that starts at a repeated reading opens on the first
// pass, so the child is held for both of them. Anything else would free them for
// the hour the clock repeated.
func TestWallAmbiguousStartOpensAtTheFirstPass(t *testing.T) {
	for _, z := range ambiguousZones {
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)
			day, ok := fallBackDay(loc)
			if !ok {
				t.Skipf("%s has no fall-back day in 2026", z)
			}
			first, second := repeatedHour(t, loc, day)
			startClock := first.Format("15:04")

			// Three hours past the second pass, read off the wall: an end that is
			// unambiguously after the window opened. A zone whose repeated hour is
			// the last one of the day has no room for that, and forcing one would
			// turn the rule into an overnight window that never closes during the
			// minutes this test looks at.
			later := second.Add(3 * time.Hour)
			if _, _, d := later.In(loc).Date(); d != day.Day() {
				t.Skipf("%s repeats %s in its last hour of the day, leaving no room for a window that opens there and closes the same day",
					z, startClock)
			}
			p := profile(z, daily(startClock, later.In(loc).Format("15:04")))

			if v := Evaluate(p, first); !v.Active {
				t.Errorf("not held at the first %s: %+v", startClock, v)
			}
			if v := Evaluate(p, first.Add(-time.Minute)); v.Active {
				t.Errorf("held a minute before the window opens at %s", startClock)
			}
			if v := Evaluate(p, second); !v.Active {
				t.Errorf("released at the second %s: the window opened on the first pass and runs to the end of the day", startClock)
			}
			if v := Evaluate(p, first); !v.StartedAt.Equal(first) {
				t.Errorf("StartedAt = %s, want %s", v.StartedAt.Format(time.RFC3339), first.Format(time.RFC3339))
			}
		})
	}
}

// A window that starts inside the hour a spring-forward deletes.
//
// The written time never arrives, so the window opens at the jump: the first
// instant at which the clock is past what the parent wrote. time.Date instead
// shifts the time by the size of the gap, forward in Europe and backward in North
// America, which opens the Berlin window half an hour late — a fail-open — and the
// New York one an hour early.
func TestWallGapStartOpensAtTheJump(t *testing.T) {
	for _, z := range []string{"Europe/Berlin", "America/New_York", "Australia/Sydney", "Pacific/Chatham", "Australia/Lord_Howe"} {
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)
			day, ok := springForwardDay(loc)
			if !ok {
				t.Skipf("%s has no spring-forward day in 2026", z)
			}
			gap := deletedMinutes(loc, day)
			if len(gap) == 0 {
				t.Skipf("%s deletes no minutes on %s", z, day.Format("2006-01-02"))
			}
			jump := firstInstantAfterGap(loc, day, gap)
			middle := gap[len(gap)/2]

			p := profile(z, daily(clock(middle), "23:00"))

			if v := Evaluate(p, jump); !v.Active {
				t.Errorf("not held at the jump %s, though the written %s has been passed: %+v",
					jump.Format(time.RFC3339), clock(middle), v)
			}
			if v := Evaluate(p, jump.Add(-time.Minute)); v.Active {
				t.Errorf("held a minute before the jump: the window written to open at %s opened early", clock(middle))
			}
			if v := Evaluate(p, jump); !v.StartedAt.Equal(jump) {
				t.Errorf("StartedAt = %s, want the jump at %s",
					v.StartedAt.Format(time.RFC3339), jump.Format(time.RFC3339))
			}
		})
	}
}

// A window that ends inside the deleted hour holds to the jump. The clock never
// reaches the written end, so the moment it stops showing the minute before the
// gap is the moment the parent meant.
func TestWallGapEndHoldsToTheJump(t *testing.T) {
	for _, z := range []string{"Europe/Berlin", "America/New_York", "Australia/Sydney", "Pacific/Chatham"} {
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)
			day, ok := springForwardDay(loc)
			if !ok {
				t.Skipf("%s has no spring-forward day in 2026", z)
			}
			gap := deletedMinutes(loc, day)
			if len(gap) == 0 || gap[0] == 0 {
				t.Skipf("%s gives no room before the gap on %s", z, day.Format("2006-01-02"))
			}
			jump := firstInstantAfterGap(loc, day, gap)
			middle := gap[len(gap)/2]

			p := profile(z, daily("00:30", clock(middle)))

			if v := Evaluate(p, jump.Add(-time.Minute)); !v.Active {
				t.Errorf("released before the jump, though the clock has not reached %s: %+v", clock(middle), v)
			}
			if v := Evaluate(p, jump); v.Active {
				t.Errorf("still holding at the jump: EndsAt = %s, want %s",
					v.EndsAt.Format(time.RFC3339), jump.Format(time.RFC3339))
			}
		})
	}
}

// TestWallNeverFreesAMinuteTheParentWrote is the fail-closed property itself,
// stated without reference to how endpoints are resolved: no instant whose own wall
// clock reads a time inside a window the parent wrote may be free. Nothing else in
// the suite states it that way -- the other DST tests each check one endpoint, and
// an endpoint test passes whenever the endpoint lands on the pass that happens to
// agree with it.
//
// Every zone here has a clock change in 2026, and the set is chosen for the ones
// that do not behave like Europe — a half-hour shift, a forty-five minute shift,
// the southern hemisphere, a zone whose change lands on midnight, and a zone whose
// change is two hours wide.
//
// It is written as a subtest per zone on purpose. The first version of this test
// looped over the zones in the body and called t.Skipf when one had no clock
// change, which aborts the whole test rather than that zone: the first zone in the
// list with a calendar the helpers could not see skipped everything after it, and
// CI reported a pass with zero minutes checked. A skip has to be scoped to the
// zone it describes, and the test has to fail -- not skip -- if no zone ended up
// contributing anything.
func TestWallNeverFreesAMinuteTheParentWrote(t *testing.T) {
	shapes := [][2]string{
		{"00:30", "03:30"}, // straight through a night-time change
		{"01:00", "02:00"}, // inside the hours a change touches
		{"02:00", "04:00"}, // starting on the boundary
		{"02:15", "02:45"}, // entirely inside a deleted hour
		{"20:00", "23:30"}, // an evening, away from the change
	}

	// Minutes asserted across all zones. Guards the test itself: a helper that
	// quietly stops finding transitions would otherwise turn this into a no-op
	// that passes.
	var checked int

	for _, z := range ambiguousZones {
		if neverShifts[z] {
			// Not a skip: these zones are asserted below to have no clock change at
			// all, so there is no minute this test could check in them.
			continue
		}
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)
			days := changedDays(loc)
			if len(days) == 0 {
				t.Fatalf("%s changes its clocks in none of 2026. This list claims every "+
					"zone in it does; a zone that stopped shifting has to be moved to "+
					"neverShifts, where the claim is checked, rather than skipped.", z)
			}

			// Minutes this zone alone put on the table. A zone that contributes none
			// passes without having checked anything, and the aggregate guard below
			// cannot see it: seventeen zones passing on zero minutes each still
			// looks like a suite that works.
			var mine int

			for _, shape := range shapes {
				p := profile(z, daily(shape[0], shape[1]))

				for _, dc := range days {
					for _, now := range minutesOn(loc, dc.day) {
						reading := now.In(loc).Format("15:04")
						if reading < shape[0] || reading >= shape[1] {
							continue
						}
						checked++
						mine++
						if !Evaluate(p, now).Active {
							t.Fatalf("%s %s (%s day): free at %s, whose clock reads %s, inside the written %s-%s",
								z, dc.day.Format("2006-01-02"), dc.length, now.Format(time.RFC3339),
								reading, shape[0], shape[1])
						}
					}
				}
			}

			if mine == 0 {
				t.Errorf("%s has %d clock-change days and contributed no minute inside any "+
					"of the %d window shapes: the day finder sees a change this test cannot "+
					"reach, so the zone is on the list for nothing", z, len(days), len(shapes))
			}
		})
	}

	if checked == 0 {
		t.Errorf("no zone contributed a single minute to check: the transition helpers "+
			"find nothing in %d zones, so this test proves nothing", len(ambiguousZones))
	}
}

// ambiguousZones covers the clock changes that are not the European hour: half
// hour shifts, forty-five minute shifts, the southern hemisphere, and the zones
// that change at midnight.
var ambiguousZones = []string{
	"Europe/Berlin",
	"Europe/London",
	"Atlantic/Azores",
	"America/New_York",
	"America/Chicago",
	"America/Denver",
	"America/Los_Angeles",
	"America/Havana",
	"America/St_Johns",
	"America/Santiago",
	"Asia/Beirut",
	"Asia/Jerusalem",
	"Asia/Tehran",
	"Australia/Sydney",
	"Australia/Brisbane",
	"Australia/Lord_Howe",
	"Pacific/Chatham",
	"Pacific/Auckland",
	"Antarctica/Troll",
}

// neverShifts are the zones above that do not change their clocks at all, so a
// test about clock changes has nothing to check in them.
//
// They are named rather than skipped because a skip is how this suite lost a whole
// test once already: a zone that quietly stops shifting is indistinguishable, in CI,
// from a helper that quietly stops finding shifts. Both directions are therefore
// asserted -- a zone listed here must have no clock change, and a zone in the list
// that is not here must have one.
var neverShifts = map[string]bool{
	// Iran stopped shifting clocks; tzdata has no future transitions for it.
	"Asia/Tehran": true,
	// Queensland has never observed DST.
	"Australia/Brisbane": true,
}

func TestTheZoneListsSayWhatTheyMean(t *testing.T) {
	for _, z := range ambiguousZones {
		loc := zone(t, z)
		changed := len(changedDays(loc))
		switch {
		case neverShifts[z] && changed != 0:
			t.Errorf("%s is in neverShifts but has %d clock changes in 2026; it now "+
				"belongs in the tests that skip nothing", z, changed)
		case !neverShifts[z] && changed == 0:
			t.Errorf("%s has no clock change in 2026; move it to neverShifts so the "+
				"claim is checked instead of skipped", z)
		}
	}
}

// fallBackDay and springForwardDay return a 2026 day that got longer or shorter
// than twenty-four hours, if the zone has one.
//
// The test is on the length, not on it being 25 or 23 hours. Antarctica/Troll
// moves its clock by two hours in each direction, so a predicate that asks for a
// one-hour day reports that Troll has no clock change at all -- and every test
// that skips when it finds no change then skips a zone the suite lists on purpose.
// A shift of two hours is exactly the case these tests are for, so it must not be
// invisible.
//
// Unlike transitionDay they do not fail the test, because a zone's calendar is not
// something the suite should insist on.
func fallBackDay(loc *time.Location) (time.Time, bool) {
	return dayLongerThan(loc, 24*time.Hour)
}

func springForwardDay(loc *time.Location) (time.Time, bool) {
	return dayShorterThan(loc, 24*time.Hour)
}

func dayLongerThan(loc *time.Location, want time.Duration) (time.Time, bool) {
	for _, d := range changedDays(loc) {
		if d.length > want {
			return d.day, true
		}
	}
	return time.Time{}, false
}

func dayShorterThan(loc *time.Location, want time.Duration) (time.Time, bool) {
	for _, d := range changedDays(loc) {
		if d.length < want {
			return d.day, true
		}
	}
	return time.Time{}, false
}

// dayChange is a 2026 local day that was not twenty-four hours long, and by how
// much it differed.
type dayChange struct {
	day    time.Time
	length time.Duration
}

// changedDays returns every 2026 day in the zone whose length is not twenty-four
// hours, of whatever width the clock move was.
func changedDays(loc *time.Location) []dayChange {
	var out []dayChange
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	for i := 0; i < 365; i++ {
		a, b := base.AddDate(0, 0, i), base.AddDate(0, 0, i+1)
		if l := b.Sub(a); l != 24*time.Hour {
			out = append(out, dayChange{day: a, length: l})
		}
	}
	return out
}

// minutesOn returns every minute of the given local day, in order.
//
// The day is walked from its noon, which always exists, rather than from UTC
// midnight: a local day in the southern hemisphere can start sixteen hours before
// its UTC midnight, and a scan anchored there misses the morning — which on a
// transition day is exactly the part where the clock moved.
func minutesOn(loc *time.Location, day time.Time) []time.Time {
	y, mo, d := day.Date()
	noon := time.Date(y, mo, d, 12, 0, 0, 0, loc)
	var out []time.Time
	for i := -15 * 60; i <= 15*60; i++ {
		t := noon.Add(time.Duration(i) * time.Minute)
		if _, _, td := t.In(loc).Date(); td == d && t.Month() == mo && t.Year() == y {
			out = append(out, t)
		}
	}
	return out
}

// deletedMinutes lists the minutes past local midnight that no instant on the day
// shows, because the clock jumped over them.
func deletedMinutes(loc *time.Location, day time.Time) []int {
	y, mo, d := day.Date()
	var out []int
	for m := 0; m < 24*60; m++ {
		if !readsAs(time.Date(y, mo, d, m/60, m%60, 0, 0, loc), loc, y, mo, d, m) {
			out = append(out, m)
		}
	}
	return out
}

// firstInstantAfterGap is the instant the clock jumps to: the first moment on the
// day whose reading is at or past the deleted run.
func firstInstantAfterGap(loc *time.Location, day time.Time, gap []int) time.Time {
	after := gap[len(gap)-1] + 1
	for _, now := range minutesOn(loc, day) {
		h, mm, _ := now.In(loc).Clock()
		if h*60+mm >= after {
			return now
		}
	}
	return time.Time{}
}

// wallInstant at the primitive, including the input windowOn never produces.
//
// Pacific/Apia skipped 2011-12-30 entirely: the calendar went from Friday to
// Sunday when the date line moved. No instant on that day reads any time at all,
// so the resolver has neither a match nor a jump to bisect. windowOn cannot ask
// for such a day — it is always given a day that an instant is standing in — but
// the primitive is exported to the package's own future, and the honest answer is
// that it says nothing useful rather than crashing or inventing a window.
func TestWallInstantOnADayThatDoesNotExist(t *testing.T) {
	apia := zone(t, "Pacific/Apia")

	// Samoa moved across the date line on 2011-12-30: the clocks went from
	// 23:59:59 -1000 Friday straight to 00:00:00 +1400 Sunday, so the whole day is
	// missing and the offset moves by twenty-four hours at once.
	//
	// The resolver normalises the date through noon before it looks for anything,
	// and noon on a day that does not exist reads as the next day that does. So the
	// answer is the same wall time on the day the clock actually shows, and it is
	// never an instant on the skipped day -- which is the only answer that can be
	// enforced. A window asked for on 2011-12-30 in Apia is enforced at 2011-12-31
	// 12:00, and a caller that wanted the impossible gets the nearest thing to it
	// rather than a silent nothing.
	got := wallInstant(apia, 2011, 12, 30, 12*60, false)
	if y, mo, d := got.In(apia).Date(); y == 2011 && mo == 12 && d == 30 {
		t.Errorf("%s reads as a time on the day the calendar skipped", got.Format(time.RFC3339))
	}
	if want := time.Date(2011, 12, 31, 12, 0, 0, 0, apia); !got.Equal(want) {
		t.Errorf("wallInstant = %s, want %s (12:00 on the day the clock shows)",
			got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
	if last := wallInstant(apia, 2011, 12, 30, 12*60, true); !last.Equal(got) {
		t.Errorf("with nothing to choose between, earliest %s and latest %s disagree", got, last)
	}

	// The same day asked for as the 30th of a month with 31 days must not be
	// confused with the normalisation above: the 32nd is a date that overflows, and
	// it has to land on the 1st, not on the 31st it would otherwise be read as.
	if got := wallInstant(apia, 2011, 12, 31, 12*60, false); got.In(apia).Day() != 31 {
		t.Errorf("the 31st in a 31-day month resolved to the %d", got.In(apia).Day())
	}
}

// An ordinary minute in an ordinary zone must come back as exactly what
// time.Date gives, or the resolver has bought DST correctness with a drift on the
// 363 days a year that do not need it.
func TestWallInstantAgreesWithTheCalendarOnAnOrdinaryMinute(t *testing.T) {
	for _, z := range []string{"UTC", "Europe/Berlin", "America/New_York", "Asia/Kolkata", "Pacific/Kiritimati"} {
		loc := zone(t, z)
		for _, day := range []time.Time{
			time.Date(2026, 6, 15, 12, 0, 0, 0, loc),
			time.Date(2026, 2, 28, 12, 0, 0, 0, loc),
		} {
			y, mo, d := day.Date()
			for _, min := range []int{0, 1, 60, 750, 1439} {
				want := time.Date(y, mo, d, min/60, min%60, 0, 0, loc)
				for _, last := range []bool{false, true} {
					if got := wallInstant(loc, y, mo, d, min, last); !got.Equal(want) {
						t.Errorf("%s %s %s: got %s, want %s",
							z, day.Format("2006-01-02"), clock(min), got.Format(time.RFC3339), want.Format(time.RFC3339))
					}
				}
			}
		}
	}
}
