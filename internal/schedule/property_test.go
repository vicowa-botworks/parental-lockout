package schedule

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// The table tests check the cases somebody thought of. This one checks the ones
// nobody did: it generates random schedules, walks them minute by minute for a
// quarter of a year, and holds the evaluator against a model written from the
// spec rather than from the code.
//
// The model is deliberately written differently — it compares wall-clock minutes
// and never constructs a window as an instant — so that the two agree by
// reasoning and not by sharing a bug. It hard-codes its own day-name table for
// the same reason: if weekday() is wrong, the model must not inherit it.
//
// Everything runs in UTC. A DST transition is where wall-clock minutes and real
// instants legitimately diverge, and that is what dst_test.go tests with exact
// instants; mixing it in here would only blur which failure is which.

var modelDayOrder = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// modelDays is the model's own copy of the day table. It repeats what
// config.DayNames says on purpose: if weekday() maps a name to the wrong day,
// the model must not inherit the mistake and agree with it.
var modelDays = map[string]time.Weekday{
	"mon": time.Monday,
	"tue": time.Tuesday,
	"wed": time.Wednesday,
	"thu": time.Thursday,
	"fri": time.Friday,
	"sat": time.Saturday,
	"sun": time.Sunday,
}

// modelActive is the specification, read literally: a rule covers t if t's
// weekday is listed and the minute of t falls inside the window that started
// today, or inside the morning half of the window that started yesterday.
func modelActive(rules []config.Rule, t time.Time) bool {
	for _, r := range rules {
		if modelRuleActive(r, t) {
			return true
		}
	}
	return false
}

func modelRuleActive(r config.Rule, t time.Time) bool {
	if !r.Enabled {
		return false
	}

	start, ok1 := parse(r.Start)
	end, ok2 := parse(r.End)
	if !ok1 || !ok2 || start == end {
		return false
	}

	cur := t.Hour()*60 + t.Minute()

	// The window that opened today. If it does not wrap, it is open between its
	// two times; if it wraps, today's half is the evening and runs to midnight.
	if listed(r, t.Weekday()) {
		if end > start {
			if cur >= start && cur < end {
				return true
			}
		} else if cur >= start {
			return true
		}
	}

	// The window that opened yesterday, which only reaches into today when it
	// wraps, and then only up to its end time.
	if listed(r, t.AddDate(0, 0, -1).Weekday()) && end < start && cur < end {
		return true
	}

	return false
}

func listed(r config.Rule, day time.Weekday) bool {
	for _, d := range r.Days {
		if modelDays[d] == day {
			return true
		}
	}
	return false
}

func parse(hhmm string) (int, bool) {
	if len(hhmm) != 5 || hhmm[2] != ':' {
		return 0, false
	}
	h := int(hhmm[0]-'0')*10 + int(hhmm[1]-'0')
	m := int(hhmm[3]-'0')*10 + int(hhmm[4]-'0')
	if h > 23 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// generate makes a schedule the way a parent would: a handful of rules on random
// days, some wrapping past midnight, none of them the degenerate start == end.
func generate(rng *rand.Rand) []config.Rule {
	n := 1 + rng.Intn(4)

	rules := make([]config.Rule, 0, n)
	for i := 0; i < n; i++ {
		var days []string
		// A slice, not the map: Go randomises map iteration, and a seed that
		// does not reproduce its own failure is worse than no seed at all.
		for _, name := range modelDayOrder {
			if rng.Intn(2) == 0 {
				days = append(days, name)
			}
		}
		if len(days) == 0 {
			days = []string{"wed"}
		}

		start := rng.Intn(24 * 60)
		end := rng.Intn(24 * 60)
		for end == start {
			end = rng.Intn(24 * 60)
		}

		rules = append(rules, config.Rule{
			ID:      fmt.Sprintf("r%d", i),
			Name:    fmt.Sprintf("rule %d", i),
			Days:    days,
			Start:   clock(start),
			End:     clock(end),
			Enabled: true,
		})
	}
	return rules
}

func clock(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

func TestPropertyAgreesWithTheSpec(t *testing.T) {
	const days = 90

	for seed := int64(1); seed <= 12; seed++ {
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			rules := generate(rng)

			p := &config.Profile{ID: "p", Timezone: "UTC", Rules: rules}
			start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).
				AddDate(0, 0, rng.Intn(200))

			// Walk every minute. A disagreement is reported with the schedule
			// that produced it, because a random failure nobody can reproduce is
			// not a finding.
			for i := 0; i < days*24*60; i++ {
				now := start.Add(time.Duration(i) * time.Minute)

				got := Evaluate(p, now).Active
				want := modelActive(rules, now)
				if got != want {
					t.Fatalf("disagreement at %s (%s)\n  Evaluate: %v\n  spec:     %v\n  schedule: %s",
						now.Format(time.RFC3339), now.Weekday(), got, want, describe(rules))
				}

				// Active flips only at a declared boundary: if the answer changes
				// across this minute, a rule must open or close exactly here.
				if next := modelActive(rules, now.Add(time.Minute)); want != next {
					if !isBoundary(rules, now.Add(time.Minute)) {
						t.Fatalf("Active changed at %s with no window boundary there\n  schedule: %s",
							now.Add(time.Minute).Format(time.RFC3339), describe(rules))
					}
				}
			}
		})
	}
}

// isBoundary reports whether some rule's window opens or closes at exactly t.
func isBoundary(rules []config.Rule, t time.Time) bool {
	if t.Second() != 0 || t.Nanosecond() != 0 {
		return false
	}
	cur := t.Hour()*60 + t.Minute()

	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		start, ok1 := parse(r.Start)
		end, ok2 := parse(r.End)
		if !ok1 || !ok2 || start == end {
			continue
		}

		// A boundary is a window edge landing on t: today's start, or today's end
		// for a window that started today, or yesterday's end for one that wrapped.
		if listed(r, t.Weekday()) && cur == start {
			return true
		}
		if listed(r, t.Weekday()) && cur == end && end > start {
			return true
		}
		if listed(r, t.AddDate(0, 0, -1).Weekday()) && cur == end && end < start {
			return true
		}
	}
	return false
}

func TestPropertyVerdictIsSelfConsistent(t *testing.T) {
	for seed := int64(1); seed <= 8; seed++ {
		rng := rand.New(rand.NewSource(seed + 500))
		rules := generate(rng)
		p := &config.Profile{ID: "p", Timezone: "UTC", Rules: rules}

		start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
		for i := 0; i < 30*24*60; i += 7 {
			now := start.Add(time.Duration(i) * time.Minute)
			v := Evaluate(p, now)

			if !v.Active {
				if v.Rule != nil || !v.StartedAt.IsZero() || !v.EndsAt.IsZero() {
					t.Fatalf("inactive verdict carries a window at %s: %+v", now.Format(time.RFC3339), v)
				}
				continue
			}

			if v.Rule == nil {
				t.Fatalf("active verdict with no rule at %s", now.Format(time.RFC3339))
			}
			if !v.Rule.Enabled {
				t.Fatalf("the disabled rule %s locked the account at %s", v.Rule.ID, now)
			}
			if v.StartedAt.After(now) || !v.EndsAt.After(now) {
				t.Fatalf("verdict at %s does not contain the instant: [%s, %s)",
					now.Format(time.RFC3339), v.StartedAt, v.EndsAt)
			}

			// The named rule must actually be the one that ends last: re-reading
			// the verdict at a minute before EndsAt has to still be active, and at
			// EndsAt the account is released unless a later rule takes over.
			if v2 := Evaluate(p, v.EndsAt.Add(-time.Minute)); !v2.Active {
				t.Fatalf("released a minute before EndsAt at %s: %+v", now.Format(time.RFC3339), v2)
			}
			if v2 := Evaluate(p, v.EndsAt); v2.Active && !v2.EndsAt.After(v.EndsAt) {
				t.Fatalf("still active at EndsAt with no later rule: %+v then %+v", v, v2)
			}
		}
	}
}

// NextStart is "the earliest window opening after now". The model states that
// directly: for each rule, each of the coming days that the rule lists, the
// window opens at the rule's start time; take the earliest one still ahead.
//
// This is the one place the model and the implementation reach for the same
// shape, because there is no other way to say "earliest opening". What keeps it
// honest is that Active is already pinned exhaustively by the minute walk above,
// and that every returned value is separately checked to be a real opening of
// some rule by modelRuleActive alone, below.
func modelNextStart(rules []config.Rule, now time.Time) time.Time {
	var best time.Time

	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		start, ok1 := parse(r.Start)
		end, ok2 := parse(r.End)
		if !ok1 || !ok2 || start == end {
			continue
		}

		for d := 0; d <= NextStartHorizon; d++ {
			day := now.AddDate(0, 0, d)
			if !listed(r, day.Weekday()) {
				continue
			}
			y, mo, dd := day.Date()
			open := time.Date(y, mo, dd, start/60, start%60, 0, 0, now.Location())
			if open.After(now) && (best.IsZero() || open.Before(best)) {
				best = open
			}
		}
	}
	return best
}

func TestPropertyNextStartIsTheFirstOpening(t *testing.T) {
	for seed := int64(1); seed <= 8; seed++ {
		rng := rand.New(rand.NewSource(seed + 900))
		rules := generate(rng)
		p := &config.Profile{ID: "p", Timezone: "UTC", Rules: rules}

		start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
		for i := 0; i < 30*24*60; i += 13 {
			now := start.Add(time.Duration(i) * time.Minute)
			v := Evaluate(p, now)

			if want := modelNextStart(rules, now); !v.NextStart.Equal(want) {
				t.Fatalf("NextStart at %s\n  got:  %s\n  want: %s\n  schedule: %s",
					now.Format(time.RFC3339), v.NextStart.Format(time.RFC3339),
					want.Format(time.RFC3339), describe(rules))
			}

			if v.NextStart.IsZero() {
				continue
			}

			// Independently: the instant it names must be some rule genuinely
			// opening — active at that minute, not active the minute before. This
			// uses only modelRuleActive, so a NextStart that was merely "a time
			// when something happened to be locked out" cannot pass it.
			opened := false
			for _, r := range rules {
				if modelRuleActive(r, v.NextStart) && !modelRuleActive(r, v.NextStart.Add(-time.Minute)) {
					opened = true
					break
				}
			}
			if !opened {
				t.Fatalf("NextStart %s is not an opening of any rule\n  from: %s\n  schedule: %s",
					v.NextStart.Format(time.RFC3339), now.Format(time.RFC3339), describe(rules))
			}

			if !v.NextStart.After(now) {
				t.Fatalf("NextStart %s is not after %s", v.NextStart, now)
			}
			if v.NextStart.Sub(now) > NextStartHorizon*24*time.Hour {
				t.Fatalf("NextStart %s is beyond the horizon from %s", v.NextStart, now)
			}
		}
	}
}

func describe(rules []config.Rule) string {
	out := ""
	for _, r := range rules {
		out += fmt.Sprintf("\n    %s %v %s-%s enabled=%v", r.ID, r.Days, r.Start, r.End, r.Enabled)
	}
	return out
}
