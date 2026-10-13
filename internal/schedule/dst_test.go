package schedule

import (
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// DST is where a schedule written in wall-clock time and evaluated in instants
// goes wrong, and both directions fail silently: a fall-back night enforced
// twice, or a spring-forward lockout that evaporates with the hour it was meant
// to cover.
//
// The rule this package holds to is that a parent writes wall-clock times and
// gets wall-clock behaviour. A window's real duration therefore changes across a
// transition — that is the correct answer, not a bug — so these tests assert the
// changed durations explicitly instead of reaching for 24-hour arithmetic.
//
// Nothing here hard-codes a transition date or an ambiguous hour. Berlin falls
// back at 03:00->02:00 and New York at 02:00->01:00, so the repeated hour is a
// different hour in each, and a test that assumed one would quietly assert
// nothing in the other. The helpers below find both from the zone database.

func zone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q): %v — is the tzdata package installed?", name, err)
	}
	return loc
}

// daily is a rule that applies every weekday, so a transition day is never
// skipped for want of a matching name.
func daily(start, end string) config.Rule {
	return rule("daily", allDays, start, end)
}

// transitionDay returns the day in 2026 whose midnight-to-midnight length is
// want — a 23-hour day for a spring forward, 25 for a fall back.
func transitionDay(t *testing.T, loc *time.Location, want time.Duration) time.Time {
	t.Helper()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	for i := 0; i < 365; i++ {
		a := base.AddDate(0, 0, i)
		b := base.AddDate(0, 0, i+1)
		if b.Sub(a) == want {
			return a
		}
	}
	t.Fatalf("no %v day in %s in 2026", want, loc)
	return time.Time{}
}

// repeatedHour returns two instants on the given day that show the same local
// wall clock — the hour the clock change runs through twice.
func repeatedHour(t *testing.T, loc *time.Location, day time.Time) (first, second time.Time) {
	t.Helper()

	seen := make(map[string]time.Time, 26*60)
	for i := day; i.Before(day.Add(26 * time.Hour)); i = i.Add(time.Minute) {
		key := i.Format("15:04")
		if prev, ok := seen[key]; ok {
			return prev, i
		}
		seen[key] = i
	}
	t.Fatalf("no wall-clock minute repeats on %s in %s", day.Format("2006-01-02"), loc)
	return time.Time{}, time.Time{}
}

// walk samples the verdict across a span and reports how many separate locked
// blocks there were and how long the account was held in total.
//
// The block count is the shape of the answer, and it is what catches the two DST
// failures a duration check alone waves through. A window enforced twice in one
// night adds up to a plausible total; a window split by the clock change looks
// like one long block unless you are counting where the blocks start.
func walk(p *config.Profile, from time.Time, span, step time.Duration) (blocks int, held time.Duration) {
	was := false
	for i := from; i.Before(from.Add(span)); i = i.Add(step) {
		now := Evaluate(p, i).Active
		if !now {
			was = false
			continue
		}
		if !was {
			blocks++
		}
		// The opening sample counts toward the time held as well as toward the
		// block count: it is a real minute behind the lock, and leaving it out
		// makes every window read one minute shorter than it is.
		held += step
		was = true
	}
	return blocks, held
}

func TestDSTSpringForward(t *testing.T) {
	for _, z := range []string{"Europe/Berlin", "America/New_York"} {
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)
			day := transitionDay(t, loc, 23*time.Hour)
			y, mo, d := day.Date()

			p := profile(z, daily("01:00", "03:00"))

			// 01:00 exists, 02:00 does not, 03:00 exists. The window is the one
			// real hour between them: the lost hour is genuinely gone, and a
			// lockout that stretched to make it whole would be holding the child
			// an hour past what the parent wrote.
			v := Evaluate(p, day.Add(90*time.Minute))
			if !v.Active {
				t.Fatalf("not locked out 90 minutes into a 01:00-03:00 window on %s: %+v",
					day.Format("2006-01-02"), v)
			}
			if got := v.EndsAt.Sub(v.StartedAt); got != time.Hour {
				t.Errorf("window is %v, want 1h", got)
			}
			if want := time.Date(y, mo, d, 1, 0, 0, 0, loc); !v.StartedAt.Equal(want) {
				t.Errorf("StartedAt = %s, want 01:00 local", v.StartedAt)
			}
			if want := time.Date(y, mo, d, 3, 0, 0, 0, loc); !v.EndsAt.Equal(want) {
				t.Errorf("EndsAt = %s, want 03:00 local", v.EndsAt)
			}

			// One block, not two: the window must not be enforced on both sides
			// of the jump.
			blocks, held := walk(p, day, 23*time.Hour, time.Minute)
			if blocks != 1 {
				t.Errorf("%d locked blocks on the spring-forward day, want 1", blocks)
			}
			if held != time.Hour {
				t.Errorf("held for %v across the day, want 1h", held)
			}
		})
	}
}

func TestDSTFallBack(t *testing.T) {
	for _, z := range []string{"Europe/Berlin", "America/New_York"} {
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)
			day := transitionDay(t, loc, 25*time.Hour)

			p := profile(z, daily("00:30", "02:30"))

			// The clock goes back across the middle of this window, so the
			// repeated hour is inside it twice. Both passes have to hold the
			// account: releasing at the end of the first pass would hand the
			// child back the screen in the middle of a lockout the parent wrote
			// once.
			first, second := repeatedHour(t, loc, day)
			if first.Format("15:04") != second.Format("15:04") {
				t.Fatalf("repeatedHour returned two different clocks: %s and %s", first, second)
			}
			for _, now := range []time.Time{first, second} {
				if v := Evaluate(p, now); !v.Active {
					name, _ := now.Zone()
					t.Errorf("released at %s (%s), inside 00:30-02:30 on the fall-back day",
						now.Format(time.RFC3339), name)
				}
			}

			// One block covering the whole longer night. Two blocks would mean
			// the window was instantiated per wall-clock pass, which is the
			// doubled window; a shorter one means it was cut at the first pass.
			v := Evaluate(p, first)
			if got := v.EndsAt.Sub(v.StartedAt); got != 3*time.Hour {
				t.Errorf("window is %v, want 3h — the night is genuinely longer, held once", got)
			}

			blocks, held := walk(p, day, 25*time.Hour, time.Minute)
			if blocks != 1 {
				t.Errorf("%d locked blocks on the fall-back day, want 1", blocks)
			}
			if held != 3*time.Hour {
				t.Errorf("held for %v across the day, want 3h", held)
			}
		})
	}
}

// An all-day rule takes the length of the day it is written on: 22h59m of real
// lockout on a 23-hour day, 24h59m on a 25-hour one. The odd minutes are the
// point — they are what a duration-based implementation gets wrong by an hour.
func TestDSTAllDayRuleAcrossTransition(t *testing.T) {
	for _, z := range []string{"Europe/Berlin", "America/New_York"} {
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)
			p := profile(z, daily("00:00", "23:59"))

			short := transitionDay(t, loc, 23*time.Hour)
			if blocks, held := walk(p, short, 23*time.Hour, time.Minute); blocks != 1 || held != 22*time.Hour+59*time.Minute {
				t.Errorf("spring-forward day: %d blocks held %v, want 1 block held 22h59m", blocks, held)
			}

			long := transitionDay(t, loc, 25*time.Hour)
			if blocks, held := walk(p, long, 25*time.Hour, time.Minute); blocks != 1 || held != 24*time.Hour+59*time.Minute {
				t.Errorf("fall-back day: %d blocks held %v, want 1 block held 24h59m", blocks, held)
			}
		})
	}
}

// The widened all-day rule (an unreadable clock) has to end at the LAST instant
// that reads the next midnight, not the first.
//
// Where a fall-back crosses midnight the clock shows the next day's 00:00 and then
// jumps back, so an end taken from the first reading closes the window while the
// clock still reads the day the rule was written for. Two checks, because the two
// halves need different databases:
//
//   - the endpoint itself, in America/Havana, where a fall-back at 01:00 repeats
//     midnight in a canonical zone every host has. This is the half that pins the
//     flag, and it runs everywhere;
//   - the consequence, walking every minute of the enforced day. Only the right/*
//     leap-second variants actually free a minute this way (the 27-second offset
//     moves the change off exact midnight), so each zone declares whether it CAN fail
//     that walk. A zone that can fail but no longer has an ambiguous walked day is an
//     error rather than a pass: the day list is otherwise free to rot into minutes that
//     cannot fail, which is how this subtest passed for three of its zones while the
//     bug was live. See the walk for why the day before each change day is in the list.
//
// lastReadingOfNextMidnight scans the clock directly for every minute-boundary that
// reads the enforced day's successor at 00:00, returning the last such instant and how
// many there were. A count above one is what "the next midnight is ambiguous" means
// here, and it is the only honest way to ask: building this from wallInstant would have
// the guard ask the function under test whether the ambiguity is there, which reports
// "no ambiguity" precisely when the resolver is what is broken.
func lastReadingOfNextMidnight(loc *time.Location, enforced time.Time) (time.Time, int) {
	ny, nmo, nd := enforced.AddDate(0, 0, 1).Date()
	var last time.Time
	var n int
	for i := -720; i <= 2160; i++ {
		c := enforced.Add(time.Duration(i) * time.Minute)
		if c.In(loc).Format("15:04") != "00:00" {
			continue
		}
		if cy, cmo, cd := c.In(loc).Date(); cy != ny || cmo != nmo || cd != nd {
			continue
		}
		n++
		last = c
	}
	return last, n
}

func TestWidenedDayEndsAtTheLastReadingOfTheNextMidnight(t *testing.T) {
	t.Run("endpoint is the last reading", func(t *testing.T) {
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
			p := profile(tc.zone, rule("widen", []string{dow}, "9am", "23:00"))

			want, reads := lastReadingOfNextMidnight(loc, noon)
			if reads == 0 {
				t.Fatalf("%s %s: no instant reads the next midnight; the scan proves nothing",
					tc.zone, tc.day)
			}

			v := Evaluate(p, noon)
			if !v.Active {
				t.Fatalf("%s %s: the widened rule is not enforcing at noon: %+v", tc.zone, tc.day, v)
			}
			if !v.EndsAt.Equal(want) {
				extra := ""
				if reads > 1 {
					extra = " — an end taken from the FIRST reading releases while the clock still reads the enforced day"
				}
				t.Errorf("%s %s: window ends %s, want the last of the %d readings of the next midnight (%s)%s",
					tc.zone, tc.day, v.EndsAt.In(loc).Format("01-02 15:04 MST"), reads,
					want.In(loc).Format("01-02 15:04 MST"), extra)
			}
		}
	})

	t.Run("no minute of the enforced day is free", func(t *testing.T) {
		// Each zone declares whether it CAN fail this check. Measured on 2026 tzdata,
		// the zones with an ambiguous next midnight are Havana (10-31, canonical) and
		// the three right/* variants; canonical Santiago, Beirut and Berlin have no
		// ambiguous midnight at all, because their fall-back does not land on or
		// across midnight.
		//
		// Saying which is which is the point. Requiring every zone to be able to fail
		// would fail the controls forever; requiring nothing lets a candidate rot into
		// a control -- after a tzdata edit, say -- and report green while checking
		// nothing. So a candidate with no ambiguous walked day is an error, and a
		// control is walked anyway as the honest baseline.
		type walkZone struct {
			name       string
			mustAmbigu bool // true: must have at least one ambiguous walked day
		}
		zones := []walkZone{
			{"America/Havana", true},
			{"America/Santiago", false},
			{"Asia/Beirut", false},
			{"Europe/Berlin", false},
		}
		if _, err := time.LoadLocation("right/Egypt"); err == nil {
			// The variants that can actually reach the shape. The daemon accepts a
			// right/* timezone name with no complaint, so this is not a hypothetical.
			zones = append(zones,
				walkZone{"right/Egypt", true},
				walkZone{"right/Asia/Beirut", true},
				walkZone{"right/America/Santiago", true})
		} else {
			t.Logf("no leap-second variants on this host: the canonical controls below are the only walk")
		}

		for _, zc := range zones {
			z, mustHaveAmbiguity := zc.name, zc.mustAmbigu
			t.Run(z, func(t *testing.T) {
				loc := zone(t, z)
				// Every 2026 day whose length is not 24h, the day BEFORE each of
				// those, and an ordinary day so a zone with no change still asserts
				// something.
				//
				// The day before is not padding. Where a fall-back lands on midnight,
				// the 25-hour length belongs to the NEXT day, so the day the rule
				// actually enforces is the ordinary-looking one before the change --
				// and a walk over only the change days checks minutes that cannot
				// fail. That is the same trap the suite already has a rule for on the
				// zone list, applied to the day list: a subtest that passes while
				// checking nothing is invisible to an aggregate guard.
				seen := map[[3]int]bool{}
				var days []time.Time
				add := func(d time.Time) {
					y, mo, dd := d.Date()
					k := [3]int{y, int(mo), dd}
					if seen[k] {
						return
					}
					seen[k] = true
					days = append(days, d)
				}
				add(time.Date(2026, 6, 15, 12, 0, 0, 0, loc))
				for _, dc := range changedDays(loc) {
					add(dc.day)
					add(dc.day.AddDate(0, 0, -1))
				}

				var walked, freed, ambiguousDays int
				for _, day := range days {
					y, mo, d := day.Date()
					noon := time.Date(y, mo, d, 12, 0, 0, 0, loc)
					// Does this day's next midnight have more than one reading? If
					// not, no answer the evaluator gives on this day can be wrong in
					// the way the subtest is looking for, and the day contributes
					// coverage it will not report.
					if _, reads := lastReadingOfNextMidnight(loc, noon); reads > 1 {
						ambiguousDays++
					}
					dow := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}[noon.Weekday()]
					p := profile(z, rule("widen", []string{dow}, "9am", "23:00"))
					for _, now := range minutesOn(loc, noon) {
						walked++
						if !Evaluate(p, now).Active {
							freed++
						}
					}
				}
				if walked == 0 {
					t.Fatalf("%s: walked no minutes; the subtest proves nothing", z)
				}
				// The stronger half of the guard, and the reason the day list above is
				// not simply longer: a candidate zone whose walked days are all
				// unambiguous has checked nothing, however many minutes it walked, so
				// it must be reported rather than counted green.
				if mustHaveAmbiguity && ambiguousDays == 0 {
					t.Errorf("%s: none of the %d walked days has an ambiguous next midnight, so no minute here could fail — the day list is shaped favourably and this subtest is not testing the widening",
						z, len(days))
				}
				if freed != 0 {
					t.Errorf("%s: %d of %d minutes whose clock reads the enforced day are FREE — the widened rule released them",
						z, freed, walked)
				}
			})
		}
	})
}

// A lockout that spans the transition night is the common real case: a school
// night rule running straight through 02:00.
func TestDSTOvernightWindowAcrossTransition(t *testing.T) {
	loc := zone(t, "Europe/Berlin")

	// The night of 28/29 March 2026 loses an hour, so 22:00 to 06:00 is seven
	// real hours rather than eight.
	p := profile("Europe/Berlin", daily("22:00", "06:00"))

	start := time.Date(2026, 3, 28, 22, 0, 0, 0, loc)
	v := Evaluate(p, start.Add(time.Hour))
	if !v.Active {
		t.Fatalf("not locked out on the transition night: %+v", v)
	}
	if got := v.EndsAt.Sub(v.StartedAt); got != 7*time.Hour {
		t.Errorf("window is %v, want 7h — the night that loses an hour is shorter in real time", got)
	}
	if want := time.Date(2026, 3, 29, 6, 0, 0, 0, loc); !v.EndsAt.Equal(want) {
		t.Errorf("EndsAt = %s, want 06:00 local on the 29th", v.EndsAt)
	}

	// Still locked out at 03:00 on the morning of the change — the instant a
	// duration-based implementation releases early.
	if v := Evaluate(p, time.Date(2026, 3, 29, 3, 0, 0, 0, loc)); !v.Active {
		t.Errorf("released at 03:00 on the spring-forward morning: %+v", v)
	}

	// The night before is an ordinary eight hours, so the rule has not been
	// stretched across the whole week by one transition.
	if v := Evaluate(p, time.Date(2026, 3, 27, 23, 0, 0, 0, loc)); v.EndsAt.Sub(v.StartedAt) != 8*time.Hour {
		t.Errorf("the ordinary night before is %v, want 8h", v.EndsAt.Sub(v.StartedAt))
	}
}

// A window whose ends both fall in the skipped hour has no interval to enforce.
// It must not panic and must not invent a lockout.
func TestDSTWindowEntirelyInsideTheSkippedHour(t *testing.T) {
	for _, z := range []string{"Europe/Berlin", "America/New_York"} {
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)
			day := transitionDay(t, loc, 23*time.Hour)

			p := profile(z, daily("02:00", "02:30"))

			blocks, held := walk(p, day, 23*time.Hour, time.Minute)
			if held > 30*time.Minute {
				t.Errorf("a 30-minute rule held for %v on the day its hour does not exist", held)
			}
			if blocks > 1 {
				t.Errorf("%d blocks, want at most 1", blocks)
			}

			// Whatever happens on the skipped day, the next ordinary day must be
			// unaffected: one impossible day cannot be allowed to swallow the
			// rule.
			next := day.AddDate(0, 0, 1)
			if _, heldNext := walk(p, next, 24*time.Hour, time.Minute); heldNext != 30*time.Minute {
				t.Errorf("the day after holds for %v, want 30m", heldNext)
			}
		})
	}
}

// time.Date is not monotonic in its wall-clock argument across a spring-forward
// gap. On the night the clock jumps, the times inside the gap are resolved into
// the shifted hour, and they land after real times that are earlier on the wall:
// in Berlin on 2026-03-29, 02:59 resolves to 03:59 while 03:00 resolves to 03:00.
//
// So a rule that Validate is happy with — two legal HH:MM values, end after
// start — can come back from the clock with its end before its start. That is
// what windowOn's last guard is for, and this finds the pair from the zone
// database rather than hard-coding one, because Berlin and New York produce it at
// different minutes.
// A window that lands in the hour a spring-forward deletes.
//
// The clock goes 01:59 straight to 03:00, so a written time inside the gap names
// no instant. Two shapes follow, and they get different answers.
//
// A window with both ends inside the gap has no instant inside it at all: both
// ends resolve to the jump itself and the window is empty. It must not lock anyone
// out, and it must not be reported as a malformed rule — the parent typed two
// legal times and the calendar is what moved.
//
// A window that starts before the gap and ends inside it holds until the jump.
// "Until 02:00" on a night when 02:00 never arrives means until the clock stops
// showing 01:59, which is the same instant. Releasing a minute earlier would be
// the fail-open.
func TestDSTWindowInsideTheDeletedHour(t *testing.T) {
	for _, z := range []string{"Europe/Berlin", "America/New_York", "America/Havana"} {
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)
			day := transitionDay(t, loc, 23*time.Hour)
			y, mo, d := day.Date()

			// The deleted hour: the first written minute that does not read back,
			// and the last one that does not.
			first, last := -1, -1
			for m := 0; m < 24*60; m++ {
				if !readsAs(time.Date(y, mo, d, m/60, m%60, 0, 0, loc), loc, y, mo, d, m) {
					if first < 0 {
						first = m
					}
					last = m
				}
			}
			if first < 0 {
				t.Skipf("%s deletes no minutes on %s", z, day.Format("2006-01-02"))
			}

			// Both ends inside: nothing to enforce.
			inside := profile(z, daily(clock(first), clock(last)))
			if blocks, held := walk(inside, day, 23*time.Hour, time.Minute); blocks != 0 || held != 0 {
				t.Errorf("a window entirely inside the deleted %s-%s held for %v in %d blocks, want nothing",
					clock(first), clock(last), held, blocks)
			}
			if errs := ProfileErrors(inside); errs != nil {
				t.Errorf("ProfileErrors = %v, want nil: the rule is legal, the clock is not", errs)
			}

			// One end inside: hold to the jump. The last minute before the gap is a
			// real minute, and the window runs to the instant the clock stops
			// showing it.
			if first == 0 {
				t.Skipf("%s deletes the day from its first minute", z)
			}
			straddles := profile(z, daily(clock(first-1), clock(first)))
			if blocks, held := walk(straddles, day, 23*time.Hour, time.Minute); blocks != 1 || held != time.Minute {
				t.Errorf("the window %s-%s across the jump: %d blocks held %v, want 1 block held 1m",
					clock(first-1), clock(first), blocks, held)
			}

			// And the next ordinary day must be untouched: one impossible hour
			// cannot be allowed to switch the rule off for good.
			next := day.AddDate(0, 0, 1)
			if blocks, held := walk(straddles, next, 24*time.Hour, time.Minute); blocks != 1 || held != time.Minute {
				t.Errorf("the day after: %d blocks held %v, want 1 block held 1m", blocks, held)
			}
		})
	}
}

// A profile in a zone with no DST at all is the common case for a family that
// never changes its clocks, and it must not be disturbed by what the machine's
// own zone does.
func TestDSTZoneWithoutTransitions(t *testing.T) {
	loc := zone(t, "Asia/Kolkata")
	p := profile("Asia/Kolkata", daily("21:00", "06:00"))

	// Noon to noon, so the overnight window is seen as the single block it is
	// rather than as the tail of last night plus the start of tonight.
	noon := time.Date(2026, 3, 29, 12, 0, 0, 0, loc)
	if blocks, held := walk(p, noon, 24*time.Hour, time.Minute); blocks != 1 || held != 9*time.Hour {
		t.Errorf("%d blocks held %v, want 1 block held 9h", blocks, held)
	}

	// The same instant evaluated through a profile in Berlin and one in Kolkata
	// gives different answers, and both are right: the schedule belongs to the
	// child's wall clock, not the machine's.
	berlin := profile("Europe/Berlin", daily("21:00", "06:00"))
	now := time.Date(2026, 3, 29, 0, 30, 0, 0, loc) // 19:00 UTC, 20:00 CET in Berlin

	if !Evaluate(p, now).Active {
		t.Error("the Kolkata profile released at 00:30 local, inside 21:00-06:00")
	}
	if v := Evaluate(berlin, now); v.Active {
		t.Errorf("the Berlin profile was active at %s, outside 21:00-06:00", v.StartedAt)
	}
}

// Some zones change their clocks at midnight rather than in the small hours, and
// in those a calendar day genuinely has no midnight: in America/Havana on
// 2026-03-08, time.Date(..., 0, 0, ...) resolves to 23:00 on the 7th.
//
// That is a fail-open, not a curiosity. Anchor the day walk on midnight and the
// transition Sunday evaluates as a Saturday, so a Sunday lockout rule does not
// fire on exactly one day a year — the day nobody thinks to test. Anchoring on
// noon is what prevents it: no zone in the tz database moves its clock within
// twelve hours of noon, so noon always belongs to the day it was built from.
//
// The list below is every zone in the installed tz database where a calendar
// day's midnight resolves to a different calendar day. The test finds the day by
// asking the database rather than trusting this comment, and skips a zone that
// no longer has the property.
func TestDSTZoneWhoseMidnightIsOnTheWrongDay(t *testing.T) {
	for _, z := range []string{"America/Havana", "America/Santiago", "Atlantic/Azores", "Chile/Continental"} {
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)

			var target time.Time
			for i := 0; i < 366*3 && target.IsZero(); i++ {
				// Enumerated through UTC: walking days in the zone under test
				// would be shifted by the very effect being looked for, and the
				// search would find nothing.
				u := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i)
				y, mo, d := u.Date()
				if mt, mm, md := time.Date(y, mo, d, 0, 0, 0, 0, loc).Date(); mt != y || mm != mo || md != d {
					target = time.Date(y, mo, d, 21, 0, 0, 0, loc)
				}
			}
			if target.IsZero() {
				t.Skipf("%s has no missing midnight in this tz database", z)
			}

			p := profile(z, rule("once", []string{dayName(target.Weekday())}, "20:00", "23:00"))

			// The evening exists; only the midnight around it does not. The rule
			// has to fire on the day it names.
			v := Evaluate(p, target)
			if !v.Active {
				t.Fatalf("the %s rule did not fire at 21:00 on %s, the day whose midnight is missing: %+v",
					target.Weekday(), target.Format("2006-01-02"), v)
			}
			if want := time.Date(target.Year(), target.Month(), target.Day(), 23, 0, 0, 0, loc); !v.EndsAt.Equal(want) {
				t.Errorf("EndsAt = %s, want 23:00 on %s", v.EndsAt, target.Format("2006-01-02"))
			}

			// And the day walk has to reach that day at all: from a week earlier
			// the dashboard must be able to see the coming lockout, rather than
			// reporting nothing because the walk stepped over the missing day.
			opens := time.Date(target.Year(), target.Month(), target.Day(), 20, 0, 0, 0, loc)
			weekBefore := time.Date(target.Year(), target.Month(), target.Day(), 12, 0, 0, 0, loc).AddDate(0, 0, -5)
			if v := Evaluate(p, weekBefore); !v.NextStart.Equal(opens) {
				t.Errorf("NextStart from %s = %s, want %s",
					weekBefore.Format("2006-01-02"), v.NextStart.Format(time.RFC3339), opens.Format(time.RFC3339))
			}
		})
	}
}

// dayName is the documented three-letter name for a weekday, read out of the same
// table the evaluator reads rather than a parallel one.
func dayName(w time.Weekday) string {
	return config.DayNames[(int(w)+6)%7]
}

// The noon anchor is not only about the day being evaluated. Every overnight
// window is found by walking back to yesterday, and in a zone whose clock change
// lands on midnight, walking back from the small hours lands on the missing
// midnight and resolves to the day before that.
//
// In America/Havana, from 00:00 to 00:59 on the Monday after the change, an
// anchor built from the instant itself puts "yesterday" on a Saturday. A
// Sunday-night lockout would then release the child at midnight instead of
// holding them to 06:00 — in the small hours, which is precisely when a lockout
// is there to hold. Anchoring on noon makes the walk land on a time that always
// exists.
func TestDSTYesterdayWalkAcrossAMidnightTransition(t *testing.T) {
	for _, z := range []string{"America/Havana", "America/Santiago", "Atlantic/Azores", "Chile/Continental"} {
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)

			var changed time.Time
			for i := 0; i < 366*3 && changed.IsZero(); i++ {
				u := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i)
				y, mo, d := u.Date()
				if mt, mm, md := time.Date(y, mo, d, 0, 0, 0, 0, loc).Date(); mt != y || mm != mo || md != d {
					changed = time.Date(y, mo, d, 12, 0, 0, 0, loc)
				}
			}
			if changed.IsZero() {
				t.Skipf("%s has no missing midnight in this tz database", z)
			}

			// A rule on the day of the change, running through the night after it.
			p := profile(z, rule("weekend", []string{dayName(changed.Weekday())}, "22:00", "06:00"))

			// The small hours of the following day. Built from that day's date
			// rather than by adding hours to the change day: 00:30 on the change
			// day is itself one of the times that does not exist, and would
			// resolve to the evening before.
			next := changed.AddDate(0, 0, 1)
			earlyHours := time.Date(next.Year(), next.Month(), next.Day(), 0, 30, 0, 0, loc)

			v := Evaluate(p, earlyHours)
			if !v.Active {
				t.Fatalf("released at %02d:%02d on the %s after the change, inside the %s-night window: %+v",
					earlyHours.Hour(), earlyHours.Minute(), earlyHours.Weekday(),
					changed.Weekday(), v)
			}
			if want := time.Date(changed.Year(), changed.Month(), changed.Day(), 22, 0, 0, 0, loc); !v.StartedAt.Equal(want) {
				t.Errorf("StartedAt = %s, want 22:00 on %s", v.StartedAt, changed.Format("2006-01-02"))
			}
			if want := time.Date(earlyHours.Year(), earlyHours.Month(), earlyHours.Day(), 6, 0, 0, 0, loc); !v.EndsAt.Equal(want) {
				t.Errorf("EndsAt = %s, want 06:00 on %s", v.EndsAt, earlyHours.Format("2006-01-02"))
			}
		})
	}
}
