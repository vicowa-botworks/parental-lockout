package dashboard

import (
	"fmt"
	"strings"
	"time"
)

// banner is the one-sentence statement of what will happen and when.
//
// It is a sentence rather than a table because it is the part a parent reads on a
// phone from another room, and it is composed here rather than in a template because
// the sentence has to be true about the machine, not merely about the fields. The
// order of the cases below is the order of what matters: a switched-off daemon makes
// every other sentence a lie, so it is answered first and nothing else is mentioned
// alongside it.
func (v View) banner() string {
	if !v.EnforcementOn {
		// Named first and alone. Every other banner in this function would tell the
		// parent a time at which something happens, and with the switch off nothing
		// happens at any time.
		if v.LockedOut > 0 || v.NotEnforcing > 0 {
			return "Enforcement is switched off, so nothing is being locked out and nothing will be, whatever the schedule says."
		}
		return "Enforcement is switched off, so nothing is being locked out."
	}

	if held := v.heldRows(); len(held) > 0 {
		return v.heldBanner(held)
	}

	// Nothing is being held. Say when that changes, because "nobody is locked out" on
	// its own reads as an all-clear at exactly the moment a parent is deciding whether
	// to hand the machine over.
	if next, who, ok := v.nextWindow(); ok {
		if v.NotEnforcing > 0 {
			// Both halves, not the earlier one alone. The upcoming window is the more
			// urgent fact, but on its own it is an all-clear about the machine as a
			// whole -- "nobody is locked out right now; casey is back at 06:30" says
			// nothing about the second row sitting scheduled and unenforced, which is
			// precisely the thing this sentence must not hide.
			return fmt.Sprintf("Nobody is locked out right now; %s is locked out again at %s. Meanwhile %s — an override, a profile this page cannot read, or the daemon declining to lock its own session.",
				who, wallClock(next), unenforcedClause(v.NotEnforcing))
		}
		return fmt.Sprintf("Nobody is locked out right now; %s is locked out again at %s.", who, wallClock(next))
	}

	if v.NotEnforcing > 0 {
		// NotEnforcing mixes two things that need saying differently, so the sentence
		// names both rather than picking one: a profile that could not be read, and a
		// schedule that is being held open by an override. Claiming "no window is
		// scheduled" would be the fail-open reading of a broken profile, and claiming
		// "something is wrong" would alarm a parent who unlocked the child on purpose
		// ten minutes ago.
		return fmt.Sprintf("Nobody is locked out right now, but %s — an override, a profile this page cannot read, or the daemon declining to lock its own session.", unenforcedClause(v.NotEnforcing))
	}

	if len(v.Rows) == 0 {
		return "No accounts are managed yet, so nothing will be locked out."
	}
	return "Nobody is locked out right now, and no window is scheduled in the next week."
}

// heldBanner states who is held, by what, and until when.
//
// The rule name is in the sentence on purpose. A parent who is told only "locked out
// until 06:30" cannot tell whether that is the school-night rule they meant or the
// weekend rule they forgot to switch off, and the second is the complaint that
// arrives by text message at seven in the morning.
func (v View) heldBanner(held []Row) string {
	first := held[0]
	var b strings.Builder
	fmt.Fprintf(&b, "%s is locked out now", first.Username)
	if first.RuleName != "" {
		fmt.Fprintf(&b, " by %q", first.RuleName)
	}
	if !first.EndsAt.IsZero() {
		fmt.Fprintf(&b, " until %s", wallClock(first.EndsAt))
	}
	if len(held) > 1 {
		fmt.Fprintf(&b, ", and %d other accounts with it", len(held)-1)
	}
	return b.String() + "."
}

// heldRows are the accounts actually being held, in the order the view shows them.
func (v View) heldRows() []Row {
	var out []Row
	for _, r := range v.Rows {
		if r.LockedOutNow {
			out = append(out, r)
		}
	}
	return out
}

// nextWindow is the earliest upcoming lockout across every row, with the account that
// owns it.
//
// Rows are already sorted by username, so a tie between two accounts at the same
// instant names the same one on every render.
func (v View) nextWindow() (time.Time, string, bool) {
	var when time.Time
	who := ""
	for _, r := range v.Rows {
		if r.NextLockAt.IsZero() || !r.Enabled {
			continue
		}
		if who == "" || r.NextLockAt.Before(when) {
			when, who = r.NextLockAt, r.Username
		}
	}
	return when, who, who != ""
}

// wallClock renders an instant the way the profile's own zone would say it.
//
// The instant carries the location it was computed in, which is the profile's, so
// this is the parent's clock rather than the daemon's. Formatting in time.Local
// would print a different minute for a child on a different zone, and the whole point
// of a per-profile timezone is that the two are not the same machine.
func wallClock(t time.Time) string {
	if t.IsZero() {
		// "00:04" for "there is no next window" is not a blank, it is a wrong answer,
		// and it is the wrong answer a reader can least easily check.
		return ""
	}
	return t.Format("15:04")
}

// unenforcedClause is the subject and verb of the sentence above, agreeing with the
// count it carries.
//
// Assembled here rather than interpolated with a fixed "is" because the plural would
// print "3 accounts is scheduled to be", and a banner with a grammar mistake in it is
// a banner a parent stops trusting.
func unenforcedClause(n int) string {
	if n == 1 {
		return "one account is scheduled to be locked out and is not"
	}
	return fmt.Sprintf("%d accounts are scheduled to be locked out and are not", n)
}
