// Package schedule answers one question: is this account locked out right now,
// and until when.
//
// It is pure. There is no I/O, no clock and no global state — every input is a
// parameter and every output is a value, which is the only reason the whole of it
// can be tested against a table. The daemon calls it once per user per poll and
// does exactly what it says.
//
// The model is lockout periods, not time budgets. A rule is a wall-clock window
// on a set of weekdays; nothing accumulates, nothing is spent, and nothing has to
// be remembered between one call and the next. That is what makes the evaluator
// idempotent, and what lets the daemon re-derive the truth from the config file
// every thirty seconds instead of maintaining state a child could corrupt.
//
// Two rules govern the edge cases here, and they pull against each other.
//
// The first is the one the config package already holds to: when something is
// genuinely ambiguous, resolve it toward the lockout, because the failure this
// product cannot survive is a child who was not locked out.
//
// The second is that a rule nobody can read cannot be enforced either, and a
// daemon that refuses to load over one malformed rule stops locking out every
// child on the machine. So an unreadable rule is skipped and reported by
// ProfileErrors. That is the one place the evaluator does not fail toward
// locking, and it is only tolerable because it is visible: whoever shows this
// list is showing a parent a rule they believe is working.
//
// What is never skipped is a rule that reads cleanly. Those are evaluated in
// wall-clock terms and never by adding durations to a day, because a window
// mis-parsed by an hour locks or unlocks at the wrong minute and says nothing
// while it does it.
package schedule

import (
	"fmt"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// Verdict is the answer to "is this account locked out right now?".
//
// Its zero value means "not locked out, and nothing is scheduled", which is what
// a nil or empty profile has to produce: a profile we know nothing about is a
// profile with no rules.
type Verdict struct {
	// Active is true when at least one enabled rule covers now.
	Active bool

	// Rule is the rule responsible for the lockout: among the rules active at
	// now, the one whose window ends last, because that is the one that decides
	// how long the child stays locked out. It is nil when Active is false.
	//
	// It points into the profile that was evaluated. It is only good for as long
	// as that profile is not modified, so copy the two fields you need — ID and
	// Name — before keeping a verdict across a config reload.
	Rule *config.Rule

	// StartedAt and EndsAt bound Rule's window. StartedAt is inclusive and
	// EndsAt is exclusive: a rule that ends at 06:30 has released the account by
	// 06:30:00, not at 06:31. Both are in the profile's location.
	StartedAt time.Time
	EndsAt    time.Time

	// NextStart is the earliest window opening strictly after now, whether or not
	// Active is true. It answers "when does the next lockout begin", which is
	// what the dashboard shows a parent.
	//
	// Note what that means while a lockout is running: NextStart can fall before
	// EndsAt, because a second rule can open in the middle of the window that is
	// currently holding the account. It is the next opening, not the next lockout
	// after this one finishes. Zero when nothing opens within the next
	// NextStartHorizon days.
	NextStart time.Time
}

// NextStartHorizon is how far ahead NextStart looks, in days.
//
// Eight: seven weekdays to cover every rule whatever its day set, plus one so
// that a rule on the same weekday as today is still found rather than being
// missed as "already passed".
const NextStartHorizon = 8

// Evaluate reports whether p locks its accounts out at now.
//
// now is used as an instant; the profile's location decides what day and what
// wall-clock time it is. Passing time.Now() is the normal case.
func Evaluate(p *config.Profile, now time.Time) Verdict {
	var v Verdict
	if p == nil {
		return v
	}

	rules, _ := compile(p.Rules)
	if len(rules) == 0 {
		return v
	}

	loc := Location(p.Timezone)
	today := anchor(loc, now)

	// A window can cover now only if it started today or yesterday. Every window
	// is shorter than 24 hours and belongs to the day it started on, so anything
	// that opened two days ago closed before midnight last night. Testing the
	// yesterday window is what makes 02:00 fall inside a Sunday 21:00-06:30 rule.
	for _, r := range rules {
		for _, day := range [2]time.Time{today.AddDate(0, 0, -1), today} {
			w, ok := r.windowOn(day, loc)
			if !ok || now.Before(w.start) || !now.Before(w.end) {
				continue
			}
			// The latest end wins, because the account stays locked while any
			// rule is active and EndsAt has to be when the last one lets go. On a
			// tie the rule that comes first in the profile is named, so the
			// verdict does not depend on map or slice order between two rules
			// that happen to release together.
			if !v.Active || w.end.After(v.EndsAt) {
				v.Active = true
				v.Rule = w.rule
				v.StartedAt = w.start
				v.EndsAt = w.end
			}
		}
	}

	for _, r := range rules {
		for i := 0; i <= NextStartHorizon; i++ {
			w, ok := r.windowOn(today.AddDate(0, 0, i), loc)
			if !ok || !w.start.After(now) {
				continue
			}
			if v.NextStart.IsZero() || w.start.Before(v.NextStart) {
				v.NextStart = w.start
			}
		}
	}

	return v
}

// ProfileErrors reports the rules in p that Evaluate will skip, and why.
//
// It exists because skipping is silent by design: a daemon that refuses to start
// over one malformed rule stops locking out every child on the machine, which is
// worse than locking out none of them. So the evaluator carries on and this is
// how the web UI and `plock doctor` find out that a rule is not doing anything.
//
// The rules it reports are the ones a parent believes are enforcing. That makes
// this list worth showing prominently rather than logging at debug level.
func ProfileErrors(p *config.Profile) []error {
	if p == nil {
		return nil
	}
	_, errs := compile(p.Rules)
	return errs
}

// window is one concrete instance of a rule: the rule's wall-clock times pinned
// to one starting date, as instants in the profile's location.
type window struct {
	rule  *config.Rule
	start time.Time
	end   time.Time
}

// compiled is a rule that has been read once: its times as minutes past
// midnight and its days as a lookup, so that evaluating it a hundred times a
// minute costs no parsing.
type compiled struct {
	def      *config.Rule
	startMin int
	endMin   int
	days     [7]bool
}

// compile turns rules into something the evaluator can use, collecting the rules
// it could not read instead of failing on them.
func compile(rules []config.Rule) ([]compiled, []error) {
	var (
		out  []compiled
		errs []error
	)

	for i := range rules {
		r := &rules[i]
		if !r.Enabled {
			continue
		}

		label := r.ID
		if label == "" {
			label = fmt.Sprintf("rules[%d]", i)
		}

		start, okStart := config.ParseClock(r.Start)
		end, okEnd := config.ParseClock(r.End)

		var bad []string
		if !okStart {
			bad = append(bad, fmt.Sprintf("start %q is not HH:MM on a 24-hour clock", r.Start))
		}
		if !okEnd {
			bad = append(bad, fmt.Sprintf("end %q is not HH:MM on a 24-hour clock", r.End))
		}

		var days [7]bool
		for _, d := range r.Days {
			wd, ok := weekday(d)
			if !ok {
				bad = append(bad, fmt.Sprintf("day %q is not a weekday name", d))
				continue
			}
			days[wd] = true
		}
		if len(r.Days) == 0 {
			bad = append(bad, "no days are listed, so the rule can never match")
		}

		if len(bad) > 0 {
			errs = append(errs, fmt.Errorf("schedule: rule %s is not enforced: %s",
				label, joinProblems(bad)))
			continue
		}

		out = append(out, compiled{def: r, startMin: start, endMin: end, days: days})
	}

	return out, errs
}

// windowOn pins the rule to the given starting date, which must be a moment on
// the intended local day — anchor is the way to make one.
//
// It reports false when the rule does not apply to that day or when the window
// that day is empty. It never fails: a rule that reached compiled has already
// been read successfully.
func (r compiled) windowOn(day time.Time, loc *time.Location) (window, bool) {
	if !r.days[day.Weekday()] {
		return window{}, false
	}

	// end == start is a zero-length window. Validate rejects it, but Load keeps a
	// document it cannot validate — a hand edit has to keep enforcing — so the
	// evaluator needs its own answer, and "this window is empty" is the only one
	// that does not invent a 24-hour lockout from two boxes that say the same
	// thing. Reading it as overnight instead would be the fail-open in reverse:
	// a typo would silently never let the child use the machine at all.
	if r.endMin == r.startMin {
		return window{}, false
	}

	y, mo, d := day.Date()
	start := time.Date(y, mo, d, r.startMin/60, r.startMin%60, 0, 0, loc)

	// A window that ends before it starts crosses midnight and still belongs to
	// the day it started on: Sunday 21:00-06:30 runs into Monday morning, and the
	// rule that says "sun" is the one responsible for Monday at 02:00.
	endDay := d
	if r.endMin < r.startMin {
		endDay = d + 1
	}
	end := time.Date(y, mo, endDay, r.endMin/60, r.endMin%60, 0, 0, loc)

	// Both ends are wall-clock, so a transition can move one of them — and
	// time.Date is not monotonic in its wall-clock argument across a
	// spring-forward gap: the times inside the gap resolve into the shifted hour
	// and land after real times that are earlier on the wall. In Berlin on
	// 2026-03-29, 02:59 resolves to 03:59 while 03:00 resolves to 03:00, so the
	// legal, Validate-approved rule 02:59-03:00 comes back with its end before
	// its start.
	//
	// There is no instant inside such a window, so it is reported as empty. The
	// alternative — normalising it somehow — would invent a lockout the parent
	// never wrote, on one night a year, in one timezone.
	if !end.After(start) {
		return window{}, false
	}

	return window{rule: r.def, start: start, end: end}, true
}

// anchor is noon on the local day that t falls on.
//
// Only the year, month and day are ever read back out of it. Noon is the point
// that matters: DST transitions sit in the small hours, and a transition can
// shift or skip a midnight-based value in a way that lands it on the wrong
// calendar day. Noon is never in the skipped hour, so adding days to an anchor
// always lands on the intended day.
func anchor(loc *time.Location, t time.Time) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 12, 0, 0, 0, loc)
}

// weekday maps a documented day name to a time.Weekday.
//
// It is derived from config.DayNames rather than repeating the list, so the
// evaluator and the validator cannot end up disagreeing about which names exist.
// DayNames starts at Monday, and time.Weekday starts at Sunday, hence the shift.
func weekday(name string) (time.Weekday, bool) {
	for i, n := range config.DayNames {
		if n == name {
			return time.Weekday((i + 1) % 7), true
		}
	}
	return 0, false
}

// Location resolves a profile's timezone name, falling back to the machine's
// local zone.
//
// The fallback is the same one config applies to an empty Timezone, so a profile
// that never named a zone evaluates in the zone the operator's clock says. An
// unrecognised name gets the same treatment rather than an error: there is no
// timezone that is safe to guess, and refusing to evaluate would drop every
// lockout in the profile. Validate rejects unknown zone names, so this is the
// hand-edit path, and ProfileErrors is where a parent hears about it.
func Location(name string) *time.Location {
	if name == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.Local
	}
	return loc
}

func joinProblems(problems []string) string {
	out := problems[0]
	for _, p := range problems[1:] {
		out += "; " + p
	}
	return out
}
