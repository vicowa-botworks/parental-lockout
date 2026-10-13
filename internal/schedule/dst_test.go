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
func TestDSTWindowTheClockReverses(t *testing.T) {
	for _, z := range []string{"Europe/Berlin", "America/New_York"} {
		t.Run(z, func(t *testing.T) {
			loc := zone(t, z)
			day := transitionDay(t, loc, 23*time.Hour)
			y, mo, d := day.Date()

			// The first wall-clock minute whose successor resolves earlier.
			startMin, endMin := -1, -1
			prev := time.Date(y, mo, d, 0, 0, 0, 0, loc)
			for m := 1; m < 24*60; m++ {
				cur := time.Date(y, mo, d, m/60, m%60, 0, 0, loc)
				if cur.Before(prev) {
					startMin, endMin = m-1, m
					break
				}
				prev = cur
			}
			if startMin < 0 {
				t.Skipf("%s has no reversed minute on its spring-forward day", z)
			}

			p := profile(z, daily(clock(startMin), clock(endMin)))

			// The rule is well formed; the clock made it empty. It must not lock
			// anyone out, and it must not be reported as a malformed rule, which
			// would tell the parent their schedule has a typo when the calendar
			// is what moved.
			if v := Evaluate(p, day); v.Active {
				t.Errorf("a window the clock reversed still locked the account out: %+v", v)
			}
			if blocks, held := walk(p, day, 23*time.Hour, time.Minute); blocks != 0 || held != 0 {
				t.Errorf("a reversed window held for %v in %d blocks, want nothing", held, blocks)
			}
			if errs := ProfileErrors(p); errs != nil {
				t.Errorf("ProfileErrors = %v, want nil: the rule is legal, the clock is not", errs)
			}

			// And the next ordinary day must be untouched: one impossible minute
			// cannot be allowed to switch the rule off for good.
			next := day.AddDate(0, 0, 1)
			if blocks, held := walk(p, next, 24*time.Hour, time.Minute); blocks != 1 || held != time.Minute {
				t.Errorf("the day after: %d blocks held %v, want 1 block held 1m", blocks, held)
			}
		})
	}
}

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
