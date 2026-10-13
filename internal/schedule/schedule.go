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
// The second is that a rule nobody can read cannot be enforced as written, and a
// daemon that refuses to load over one malformed rule stops locking out every
// child on the machine. So an unreadable rule is enforced at the widest reading of
// the parts that could be read — days that are not weekday names become every day,
// a clock that does not parse becomes the whole day — and reported by
// ProfileErrors.
//
// It is never dropped. Dropping it was the earlier policy and it is the wrong way
// round: a rule that says "tues" instead of "tue" then kept the days that parsed
// and gave up on the one the parent actually mistyped, which is a hole in the
// schedule that nothing shows. Widening produces the opposite error — a lockout on
// a day the parent did not intend, which they notice the first time it happens and
// fix. Between a mistake the parent notices and one the child enjoys, the package
// takes the first one, here as everywhere else.
//
// What is never reinterpreted is a rule that reads cleanly. Those are evaluated in
// wall-clock terms and never by adding durations to a day, because a window
// mis-parsed by an hour locks or unlocks at the wrong minute and says nothing while
// it does it.
//
// One consequence of wall-clock resolution is worth stating where the code is read:
// a window that overlaps a fall-back is held for longer than the two clock times
// suggest, by exactly the width of the fold, because it opens at the first reading
// of its start and closes at the last reading of its end. That is the safe
// direction, and it is pinned by TestAFoldLengthensAWindowByItsOwnWidth. Anything
// showing a parent when a lockout ends has to render EndsAt as the instant it is,
// not re-derive it from the rule's text.
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
	// It points into the profile that was evaluated, so it is the caller's own
	// document rather than a copy. That is safe under one rule the config store
	// enforces and this package depends on: a loaded document is never mutated in
	// place. Every edit in internal/config is copy-on-write, so a profile handed
	// to Evaluate keeps the bytes it had at the moment it was read, and a verdict
	// taken from it keeps naming the rule that was actually responsible.
	//
	// Anything that breaks that rule breaks this too, silently and in the
	// fail-open direction: a Rule pointer whose Enabled has been flipped to false
	// describes a lockout that is not happening. So copy ID and Name if a verdict
	// has to outlive the document, and do not mutate a profile in place anywhere.
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

// windowLookback is how many candidate start dates Evaluate tests: today, and the
// two days before it. See the comment at the walk for why two days back is both
// necessary and enough.
const windowLookback = 3

// NextStartHorizon is how many days ahead NextStart looks, counting today as the
// first of them.
//
// Eight: seven weekdays to cover a rule whatever its day set, plus one so that a
// rule on today's weekday is still found after today's window has closed.
// Neither can be dropped — TestNextStartHorizonIsSufficient walks every pairing
// of start day and rule day.
//
// Eight is a proof about the calendar, not about the clock, and the clock can eat
// a day: a zone that deletes a date visits seven distinct days in eight steps.
// Measured over every zone, that costs a NextStart answer only in historical
// zones whose clock changes fell inside the scan window — Africa/Cairo and
// Algeria among them — and never once across 39.8M verdicts in 2010–2035.
//
// The direction matters more than the count. NextStart is what a dashboard shows
// as "locked again at…"; getting it wrong shows a parent the wrong time. It never
// releases a child early, because Evaluate decides lockouts from the windows
// themselves and not from this number.
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

	// A window can cover now only if it started today, yesterday, or the day
	// before. Testing the yesterday window is what makes 02:00 fall inside a Sunday
	// 21:00-06:30 rule; the third day is what makes a skipped calendar day work.
	//
	// A window's end is written as a wall time on the start's date or the next
	// one, so on an unbroken calendar two days is the whole story. But a date can
	// be deleted: when Pacific/Apia crossed the dateline in 2011, Friday became
	// Sunday and 2011-12-30 never existed. A window that opened on the Thursday
	// and was written to close on the Friday closes on the Saturday instead, and
	// now inside it is two local dates past the day it started on. Walking back
	// only one released the account for the twenty-two hours in between, with the
	// rule still on the books.
	//
	// Two is enough, and it is the same measurement as the fast path's: the
	// biggest forward clock move anywhere in the database is 24h, which deletes at
	// most one date, so a window's own extra day plus one deleted day is the
	// furthest a live window can sit behind today. TestNoWindowIsEverFurtherBack
	// walks every zone looking for one that sits further.
	for _, r := range rules {
		for _, day := range [windowLookback]time.Time{
			today.AddDate(0, 0, -2), today.AddDate(0, 0, -1), today,
		} {
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
		for i := 0; i < NextStartHorizon; i++ {
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
//
// It also reports a profile whose timezone name cannot be resolved, because
// Location's fallback to the machine's own zone is the one thing in this package
// that changes what every rule means rather than dropping one. A Berlin family on
// a UTC-hosted machine loses the first hours of every overnight rule with nothing
// on screen unless something says so.
func ProfileErrors(p *config.Profile) []error {
	if p == nil {
		return nil
	}
	_, errs := compile(p.Rules)

	// An empty Timezone is the documented way to say "the machine's zone", so it
	// is not a problem. Anything else that will not resolve is.
	if p.Timezone != "" {
		if _, err := time.LoadLocation(p.Timezone); err != nil {
			errs = append(errs, fmt.Errorf(
				"schedule: timezone %q is not a known zone (%v); every rule in this profile is being read in the machine's own zone instead",
				p.Timezone, err))
		}
	}
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
// midnight and its days as a lookup, so that testing it against the many candidate
// days of one evaluation costs no parsing.
//
// It is built per Evaluate and dropped when that returns — there is no cache
// across calls. At one call per user per 30s poll that is free; if #6 ever makes
// Evaluate hot enough to matter, the compiled form is what moves onto the Profile.
type compiled struct {
	def      *config.Rule
	startMin int
	endMin   int
	days     [7]bool

	// allDay is a rule whose clock could not be read at all. It holds for the
	// whole local day rather than nothing; see compile for why the unreadable part
	// of a rule widens instead of disappearing.
	allDay bool
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
		badDays := len(r.Days) == 0
		for _, d := range r.Days {
			wd, ok := weekday(d)
			if !ok {
				bad = append(bad, fmt.Sprintf("day %q is not a weekday name", d))
				badDays = true
				continue
			}
			days[wd] = true
		}
		if badDays && len(r.Days) == 0 {
			bad = append(bad, "no days are listed")
		}

		// A rule the daemon cannot read as written is still a rule the parent
		// wrote, and the package's rule is that ambiguity resolves toward locking
		// the child out. Dropping the rule is the fail-open reading, and it is the
		// failure that goes unnoticed: a "school nights" rule that says "tues"
		// instead of "tue" keeps the days that parsed and silently loses the one
		// the parent actually mistyped.
		//
		// So each unreadable part takes the widest value consistent with the parts
		// that could be read:
		//
		//   days unreadable -- a name that is not a weekday, or none listed
		//       the window applies every day
		//   clock unreadable -- a start or end that is not HH:MM
		//       the rule holds for the whole day
		//
		// Both stay loud: the rule is reported either way, and the dashboard (#10)
		// is where the parent reads it. A lockout the parent notices and fixes beats
		// a hole the child enjoys.
		//
		// The one shape that stays empty is a rule that is readable in full and
		// whose start and end are the same time. That is not ambiguity, it is the
		// documented zero-length window, and widening it would let one typo lock the
		// child out around the clock. When the days are unreadable too there is no
		// readable part left for that reading to protect, and the widest one wins —
		// including the window, because an empty window on every day enforces nothing
		// while the report below claims the rule was widened.
		empty := okStart && okEnd && start == end
		allDay := !okStart || !okEnd || (empty && badDays)
		if badDays {
			days = everyDay()
		}

		// The same time in both boxes. Validate rejects it, but Load keeps a
		// document it cannot validate, and windowOn's own answer — empty, rather
		// than the 24 hours an overnight reading would give — means the rule
		// enforces nothing on every day of the year. That is exactly the kind of
		// silence this list exists to break: a parent looking at a schedule that
		// does nothing.
		//
		// Only the always-empty shape is reported. A window can also come back
		// empty on one date because the clock deleted the hour it lived in, and
		// saying so would mean scanning the year for every rule on every reload.
		// See issue #16.
		zeroLength := empty && !badDays
		if zeroLength {
			bad = append(bad, fmt.Sprintf("start and end are both %s", r.Start))
		} else if allDay || badDays {
			bad = append(bad, "enforcing the widest reading instead: "+enforcedAs(allDay, badDays))
		}

		if len(bad) > 0 {
			if zeroLength {
				errs = append(errs, fmt.Errorf("schedule: rule %s enforces nothing: %s, so the rule is not enforced",
					label, joinProblems(bad)))
			} else {
				errs = append(errs, fmt.Errorf("schedule: rule %s is not readable (%s)",
					label, joinProblems(bad)))
			}
		}
		if zeroLength {
			continue
		}

		out = append(out, compiled{def: r, startMin: start, endMin: end, days: days, allDay: allDay})
	}

	return out, errs
}

// everyDay is the day set of a rule whose days could not be read: it applies
// whenever its window is open.
func everyDay() (d [7]bool) {
	for i := range d {
		d[i] = true
	}
	return d
}

// enforcedAs says out loud what a rule that could not be read is being held to,
// because the report is the only place a parent finds out.
func enforcedAs(allDay, badDays bool) string {
	switch {
	case allDay && badDays:
		return "the whole day, every day"
	case allDay:
		return "the whole day, on the days that were readable"
	default:
		return "the window on every day"
	}
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

	y, mo, d := day.Date()

	// A rule whose clock could not be read holds for the whole local day: from the
	// first instant that reads this day to the LAST instant that reads the next.
	//
	// The asymmetry is deliberate and it is the whole point of the end being `true`.
	// Where a fall-back crosses midnight, the clock shows the next day's 00:00 and
	// then jumps back to 23:00 of the enforced day, so the first reading of the next
	// midnight closes the window while the clock still reads the day the rule was
	// written for. Measured on the leap-second variants (right/Egypt 2026-10-29,
	// right/Asia/Beirut 2026-10-24, right/America/Santiago 2026-04-04): the earliest
	// end frees 59 minutes of the enforced day; the latest end holds them and costs
	// one minute held whose clock already reads the next day. Both ends of every
	// other window in this file take the same side of an ambiguity for the same
	// reason. No canonical zone is affected in 2026-2031 — the shape needs the
	// leap-second offset to move the change off exact midnight — but the daemon
	// accepts a right/* timezone name without complaint, and this package does not
	// fail open on the shapes it can reach.
	if r.allDay {
		return window{
			rule:  r.def,
			start: wallInstant(loc, y, mo, d, 0, false),
			end:   wallInstant(loc, y, mo, d+1, 0, true),
		}, true
	}

	start := wallInstant(loc, y, mo, d, r.startMin, false)

	// A window that ends before it starts crosses midnight and still belongs to
	// the day it started on: Sunday 21:00-06:30 runs into Monday morning, and the
	// rule that says "sun" is the one responsible for Monday at 02:00.
	endDay := d
	if r.endMin < r.startMin {
		endDay = d + 1
	}
	end := wallInstant(loc, y, mo, endDay, r.endMin, true)

	// A window with no instant inside it is empty, and two things arrive here.
	//
	// end == start is the one a parent writes by accident, by typing the same
	// time into both boxes. Validate rejects the shape, but Load keeps a document
	// it cannot validate — a hand edit has to keep enforcing — so the evaluator
	// needs its own answer. Reading it as overnight instead would be the
	// fail-open in reverse: one typo would silently never let the child use the
	// machine at all. compile reports this shape, because it enforces nothing on
	// any day of the year.
	//
	// end < start is the one nobody writes and the one only a calendar makes. A
	// rule whose whole window falls inside an hour the clock jumped over resolves
	// both ends to the same instant — the jump — and a rule that ends inside the
	// gap while starting before it can resolve its end behind its start. Berlin's
	// 2026-03-29 deletes 02:00-03:00, so 01:30-02:30 comes back as 01:30 to 03:00
	// minus the hour that is not there.
	//
	// Reporting it as empty is the honest answer: there is no instant inside such
	// a window on that day. Normalising it would invent a lockout the parent never
	// wrote, on one night a year, in one timezone. Unlike the shape above this one
	// is empty on a single date rather than always, so compile cannot report it
	// without scanning the year; see issue #16.
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
//
// That last claim is an assumption about the tz database rather than a property of
// time arithmetic, and it is the reason the anchor is not simply `t`: a zone whose
// clock change landed on noon would shift the anchor a day and change the weekday
// read back out of it. No zone in the database observes a change at noon today,
// and the midnight zones -- America/Havana, America/Santiago, Atlantic/Azores --
// are tested by name, so if a future tzdata release ever moves one to midday this
// is the comment to come back to.
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
