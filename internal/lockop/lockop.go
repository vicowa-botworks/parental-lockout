// Package lockop is the one answer to "lock this account's session now".
//
// It existed as a loop inside `plock lock` until the admin UI needed the same action. The
// loop is short, but the three ways it can end are not obvious and they are the whole
// product: a backend that proves the screen is locked, a backend that asked and cannot
// prove anything, and nothing reaching the desktop at all. A second implementation would
// disagree with the first about which of those happened, and the operator would get one
// sentence from the terminal and the opposite one from the page.
//
// What is deliberately NOT here: whether an account is managed (that is the config
// document's answer, and the caller holds the document), and what the answer should be
// rendered as (a terminal report, an audit line, and a page sentence are three different
// renderings of the same Result).
package lockop

import (
	"context"
	"errors"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// Chain is the slice of a lock backend this package drives.
//
// It is an interface for the reason every other seam in this repository is one: the real
// implementation shells out to loginctl and gdbus, so a test that used it could only
// assert something about the machine it happened to run on. A fake here states which of
// the three answers the machine gave.
type Chain interface {
	Sessions(context.Context) ([]platform.Session, error)
	Lock(context.Context, platform.Session) error
}

// Outcome is one session's result.
//
// Locked and Unconfirmed are two fields rather than one three-valued one because the
// difference between them is the sentence the operator gets, and a zero value that means
// "nothing was locked and nothing was proven" is the honest reading of a session this
// package never asked a backend to lock.
type Outcome struct {
	Session  string
	Username string
	Locked   bool
	// Unconfirmed is the raw-VT and Plasma-ignores-the-signal case: the ask went through,
	// the exit status was zero, and nothing on the machine can attest that the screen is
	// dark. It is not a failure and it is not a success, and collapsing it into either
	// one is the lie this type exists to prevent.
	Unconfirmed bool
	Detail      string
}

// Result is what one lock request did, counted as well as itemised.
//
// The counts are not a convenience over Outcomes: the caller that renders a summary needs
// them, and a caller that recomputed them from Outcomes would be free to count them
// differently from the one that rendered the list.
type Result struct {
	Outcomes    []Outcome
	Locked      int
	Unconfirmed int
	// Refused counts sessions a backend cannot lock at all -- a class=manager session, a
	// greeter -- which is a different fact from "we asked and it did not take" and must
	// not be added to either of the other two.
	Refused int
}

// Asked reports whether any session was touched.
//
// It is the question the caller answers with an exit code or a status line: "there was
// nothing to lock" is not a failure, and "we asked for a lock and nothing answered" is.
func (r Result) Asked() bool { return len(r.Outcomes) > 0 }

// OwnedBy picks the sessions belonging to one account out of a listing.
//
// It is exported beside SessionsFor rather than folded into it because a caller that has
// already listed the sessions -- `plock lock` needs the whole table for --all and for the
// "no such account" check -- must not pay for a second loginctl call to learn the same
// thing, and a private helper would leave that caller writing its own loop and deciding
// the same question by hand.
func OwnedBy(sessions []platform.Session, username string) []platform.Session {
	var want []platform.Session
	for _, s := range sessions {
		if s.Username == username {
			want = append(want, s)
		}
	}
	return want
}

// SessionsFor returns the sessions that belong to one account.
//
// An empty result is not an error, because "this account is logged out" is a real state
// the caller has to be able to say out loud -- and it is a different sentence from "no
// such account", which only the config document can answer.
func SessionsFor(ctx context.Context, c Chain, username string) ([]platform.Session, error) {
	sessions, err := c.Sessions(ctx)
	if err != nil {
		return nil, err
	}
	return OwnedBy(sessions, username), nil
}

// notLockable says what about this session the rule will not take.
//
// The sentence is the one every other refusal in this repository uses, so the same fact reads
// the same way in the audit log, in doctor, and here. What it adds is the disqualifier, and
// only where one is readable off the record: naming the session's type instead -- which is
// what a first draft did -- told an operator that "x11" was why a greeter was refused, when
// the child's real desktop is x11 too.
//
// It deliberately falls back to the bare sentence for the conditions with no field to point at
// (no seat, a type of "unspecified"). Guessing which of them fired is how a refusal ends up
// explaining something nobody checked, and the platform rule that decided this stays in
// platform.isLockable rather than being restated here as a list of causes.
func notLockable(s platform.Session) string {
	const base = "not a lockable desktop session"
	switch {
	case s.Remote:
		return base + " (this one is remote, and locking it shuts the admin out)"
	case s.Class == "":
		return base + " (logind reported no class for it)"
	case s.Class != "user":
		return base + " (a " + s.Class + " session, not a desktop one)"
	default:
		return base
	}
}

// Now locks each session in turn and classifies what came back.
//
// It never returns an error for a session that simply failed to lock: one account's dead
// backend must not swallow the outcomes of the sessions after it, and a caller that got
// an error here would be tempted to report nothing at all. The only error is the one that
// stops the loop from making sense -- a chain that cannot list sessions -- and even that
// arrives as an empty Result rather than a half-counted one.
func Now(ctx context.Context, c Chain, sessions []platform.Session) Result {
	r := Result{Outcomes: make([]Outcome, 0, len(sessions))}
	for _, s := range sessions {
		if !platform.IsLockable(s) {
			// Counted, not silently skipped: a class=manager session is not something the
			// daemon can lock, and a report that says "locked 0 of 1" without saying why
			// reads as a broken backend.
			r.Refused++
			r.Outcomes = append(r.Outcomes, Outcome{
				Session:  s.ID,
				Username: s.Username,
				Detail:   notLockable(s),
			})
			continue
		}
		err := c.Lock(ctx, s)
		o := Outcome{Session: s.ID, Username: s.Username}
		switch {
		case err == nil:
			o.Locked = true
			r.Locked++
		case errors.Is(err, platform.ErrUnconfirmed):
			o.Unconfirmed = true
			o.Detail = err.Error()
			r.Unconfirmed++
		default:
			o.Detail = err.Error()
		}
		r.Outcomes = append(r.Outcomes, o)
	}
	return r
}
