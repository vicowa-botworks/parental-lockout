// Package dashboard turns the stored configuration into what the dashboard renders.
//
// It is a package rather than a handful of template functions because the answers it
// gives are decisions, not formatting. "Is Casey locked out right now?" has four ways
// to be wrong, and three of them come from reading a field directly: the schedule
// says yes while the master switch is off; the schedule says yes while an override is
// holding it open; the user names a profile that is not in the document; the clock
// text is re-read to compute an end time the evaluator already refused to state
// plainly. Each of those is a sentence shown to a parent that is not true about the
// machine, and a template is the worst place in the program to discover it.
package dashboard

import (
	"fmt"
	"sort"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/schedule"
)

// Row is one managed account.
//
// The zero value is not a meaningful row: an absent LockedOutNow means "not locked
// out", which is a claim about the machine. Rows are only ever produced by Build,
// which either evaluates or says in Problems why it could not.
type Row struct {
	Username string

	// ProfileName is what the parent named the profile; ProfileID is what the config
	// keys on. Both are carried because the screen links by id and reads out the name.
	ProfileID   string
	ProfileName string

	// Enabled is the account's own switch, distinct from the master switch.
	Enabled bool

	// LockedOutNow is the answer the banner is built from, and it is deliberately
	// narrower than Verdict.Active: it is true only when the account is actually being
	// held by this daemon right now.
	//
	// A verdict that is active while the master switch is off, or while an override is
	// open, is a schedule that is not being enforced. Showing those as locked out
	// tells a parent the child is sitting in front of a locked screen when the screen
	// is unlocked and the daemon will not touch it.
	LockedOutNow bool

	// RuleName is the rule responsible, and EndsAt when its window closes.
	//
	// EndsAt is copied out of the verdict and never re-derived. Mid-DST-fold the wall
	// clock can read before the written start while the account is still held, so a
	// template that recomputed the end from the rule's clock text would announce a
	// release that has not happened.
	RuleName string
	EndsAt   time.Time

	// NextLockAt is the next window opening, which can fall before EndsAt when a
	// second rule opens inside the running window. See schedule.Verdict.NextStart.
	NextLockAt time.Time

	// EndsAtClock and NextLockAtClock are EndsAt and NextLockAt written as a wall
	// time in the profile's own zone, which is what a parent actually reads.
	//
	// They are strings on the row rather than something the template formats,
	// because formatting is a decision and the template has no business making it:
	// each instant carries the location it was evaluated in, so formatting it in the
	// daemon's zone would print a different minute for a child on another zone --
	// wrong by the offset, which is the kind of wrong that leaves a parent convinced
	// the daemon let a lockout go early. Empty when there is no such instant, since
	// a zero time formats as 00:00 and midnight is a claim rather than a blank.
	EndsAtClock     string
	NextLockAtClock string

	// Override is the standing unlock, if any.
	OverrideActive  bool
	OverrideUntil   time.Time
	OverrideReason  string
	OverrideBy      string
	OverrideExpired bool

	// ScheduledButNotEnforcing marks the state the master switch creates: the schedule
	// says this account should be locked out now and nothing is doing it.
	//
	// It is a field rather than a absence-of-LockedOutNow because the two have to be
	// rendered differently, and because a summary that counts it as "not locked out"
	// is reporting an off switch as an all-clear.
	ScheduledButNotEnforcing bool

	// CannotEvaluate marks a row the dashboard could not decide, and it is the reason
	// Problems exists: an unevaluated account must never be shown as a clean "not
	// locked out".
	CannotEvaluate bool
	Problems       []string

	// Sessions are this account's live sessions, with the lock state the backends
	// reported.
	Sessions []platform.Session
}

// View is one dashboard render.
type View struct {
	Rows []Row

	// EnforcementOn is the master switch, shown at the top because it changes the
	// meaning of every row below it.
	EnforcementOn bool
	PollSeconds   int

	// LockedOut counts the rows actually being held.
	LockedOut int

	// SessionsKnown says whether the session list in these rows came from a read that
	// worked.
	//
	// Its zero value is false -- "we could not look" -- on purpose. An empty list and a
	// failed read are different facts, and the one that is safe to guess wrong about is
	// the one that makes the page claim less than it knows. Rendering a failed read as
	// an empty list tells a parent nobody has a desktop session, which is a positive
	// claim about the machine that the failed read did not establish; it is also the
	// identical rendering to a daemon whose session backend is broken, which is the
	// thing a parent needs to be told.
	SessionsKnown bool

	// NotEnforcing counts rows that are scheduled to be held and are not — the master
	// switch, or an override, or a profile that could not be read, or the daemon
	// declining to lock the session it is running inside.
	NotEnforcing int

	// Banner is the one-sentence statement of what will happen and when.
	Banner string

	// Now is the instant the view was computed at, so a relative time rendered later
	// can say what it was relative to.
	Now time.Time
}

// Build computes the dashboard from a configuration, the live sessions, and an
// instant.
//
// now is a parameter rather than time.Now for the reason every clock in this project
// takes one: the answers here are all about a boundary, and a test that cannot place
// the instant on the far side of it cannot pin the behaviour.
//
// cfg is read, never mutated, and the rows copy what they need out of it. The
// config store is copy-on-write, so a View taken from a document keeps naming the
// rules that document had.
//
// self is where the daemon is running, and it is a parameter for the same reason the
// engine takes one: the answer comes from the host's /proc, and a view that read it
// itself would render differently on a desktop and on a headless runner. Its zero
// value is the supported deployment, in which nothing is declined.
//
// It has to be here at all because the enforcement loop declines to lock the session
// it is running inside. Without it this page announces "casey is locked out until
// 20:00" about a child sitting at an unlocked screen, which is the fail-open reading
// of a row and the one this page exists to prevent.
func Build(cfg *config.Config, sessions []platform.Session, self platform.SelfLocation, now time.Time) View {
	v := View{Now: now, SessionsKnown: true}
	// A caller that could not read the sessions says so by clearing this after the
	// call; an empty list that arrived normally is knowledge, not ignorance, and must
	// not be rendered as "unavailable".
	if cfg == nil {
		v.Banner = "Nothing is being enforced: there is no configuration to enforce."
		return v
	}
	v.EnforcementOn = cfg.Settings.Enforcement
	v.PollSeconds = cfg.Settings.PollSeconds

	byName := make(map[string][]platform.Session, len(sessions))
	for i := range sessions {
		s := sessions[i]
		byName[s.Username] = append(byName[s.Username], s)
	}

	profiles := make(map[string]*config.Profile, len(cfg.Profiles))
	for i := range cfg.Profiles {
		profiles[cfg.Profiles[i].ID] = &cfg.Profiles[i]
	}

	overrides := make(map[string]*config.Override, len(cfg.Overrides))
	for i := range cfg.Overrides {
		overrides[cfg.Overrides[i].Username] = &cfg.Overrides[i]
	}

	rows := make([]Row, 0, len(cfg.Users))
	for _, u := range cfg.Users {
		r := Row{Username: u.Username, ProfileID: u.ProfileID, Enabled: u.Enabled}
		if s := byName[u.Username]; s != nil {
			r.Sessions = s
		}

		applyOverride(&r, overrides[u.Username], now)

		p, ok := profiles[u.ProfileID]
		if !ok {
			// The account is managed and the thing that says when is gone. That is
			// not "not locked out"; it is a configuration that cannot be honoured,
			// and the difference is the whole content of the row.
			r.CannotEvaluate = true
			r.Problems = append(r.Problems,
				fmt.Sprintf("no profile %q in this configuration, so nothing decides when this account is locked out", u.ProfileID))
			rows = append(rows, r)
			continue
		}
		r.ProfileName = p.Name

		if errs := schedule.ProfileErrors(p); len(errs) > 0 {
			// Reported, not repaired. The evaluator still enforces the widest
			// reading it can, so the row keeps its verdict; the parent is told the
			// verdict rests on a fallback.
			for _, e := range errs {
				r.Problems = append(r.Problems, e.Error())
			}
		}

		verdict := schedule.Evaluate(p, now)
		r.RuleName, r.EndsAt, r.NextLockAt = verdictRule(verdict)
		r.EndsAtClock = wallClock(r.EndsAt)
		r.NextLockAtClock = wallClock(r.NextLockAt)

		switch {
		case !r.Enabled:
			// The account's own switch. Distinct from the master switch because the
			// fix is different: one is a checkbox on this row, the other is the
			// daemon-wide kill switch at the top of the page.
		case r.OverrideActive:
			r.ScheduledButNotEnforcing = verdict.Active
		case !v.EnforcementOn:
			r.ScheduledButNotEnforcing = verdict.Active
		case declined(self, r.Sessions):
			// The engine will not lock this account, so the schedule is not being
			// enforced whatever the verdict says. Reported on the row and not only in
			// the count, because the fix — run plock as a system service — is not
			// something a parent can infer from "not locked out".
			r.ScheduledButNotEnforcing = verdict.Active
			if verdict.Active {
				r.Problems = append(r.Problems,
					"this account owns the login session the daemon is running in, so locking it "+
						"would lock the screen the daemon is stopped from: it is not being locked "+
						"out. Run plock as a system service (deploy/plock.service) to enforce it")
			}
		default:
			r.LockedOutNow = verdict.Active
		}
		rows = append(rows, r)
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Username < rows[j].Username })
	v.Rows = rows
	for _, r := range rows {
		if r.LockedOutNow {
			v.LockedOut++
		}
		if r.ScheduledButNotEnforcing || r.CannotEvaluate {
			v.NotEnforcing++
		}
	}
	v.Banner = v.banner()
	return v
}

// declined reports whether every session this account has on the machine is one the
// daemon will not lock.
//
// All of them, not any: the engine locks session by session, so an account with a
// second session somewhere else is still being held and saying otherwise would be a
// false alarm about a child who is in fact locked out.
//
// An account with no session is not declined. There is nothing to decline, which is
// the case Build already reports from the schedule alone, and folding it in here
// would mark every logged-out account as unenforced.
func declined(self platform.SelfLocation, sessions []platform.Session) bool {
	if len(sessions) == 0 {
		return false
	}
	for i := range sessions {
		if !self.Declines(sessions[i].UID) {
			return false
		}
	}
	return true
}

// applyOverride records the standing unlock, if there is one.
//
// An expired override is reported as expired rather than dropped. The engine stops
// honouring it the moment it passes, and a parent who sees nothing at all cannot tell
// "no override was ever made" from "the one I made ran out", which is the difference
// between unlocking the child again and going to look at the schedule.
func applyOverride(r *Row, o *config.Override, now time.Time) {
	if o == nil {
		return
	}
	r.OverrideReason = o.Reason
	r.OverrideBy = o.By
	r.OverrideUntil = o.Until
	if o.Until.After(now) {
		r.OverrideActive = true
		return
	}
	r.OverrideExpired = true
}

// verdictRule copies the parts of a verdict a row shows.
//
// An inactive verdict contributes nothing, including its times: EndsAt on an inactive
// verdict bounds a window that is not running, and rendering it would put a countdown
// on a row that is not locked out.
func verdictRule(v schedule.Verdict) (string, time.Time, time.Time) {
	if !v.Active {
		return "", time.Time{}, v.NextStart
	}
	name := ""
	if v.Rule != nil {
		name = v.Rule.Name
	}
	return name, v.EndsAt, v.NextStart
}
