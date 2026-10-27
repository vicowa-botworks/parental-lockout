package enforce

// #168: the enforcement arm's half of a partly readable session listing.
//
// The loop used to `return rep` the moment Sessions() returned an error — before the loop
// that ticks sessions. Since Sessions() returns good rows alongside an error as a matter of
// ordinary design (a show-session that times out, a row that will not decode), one flaky
// session on a five-session box made that poll one in which nobody was locked, with a single
// audit row and a dashboard that still looked fine.
//
// These tests are the pin the issue asks for in this package rather than only in platform:
// the behaviour that has to survive is "the sessions that were read get locked", and it is
// the loop, not the parser, that decides it.

import (
	"errors"
	"testing"
)

func TestAPartialReadStillLocksTheSessionsItRead(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)

	// The first poll on this engine is also the one where a row fails. kid's session is in
	// the listing, so it must be locked even though the census is short.
	p.partialRead(errors.New("loginctl list-sessions: 1 of 2 session rows could not be read: row 1"))
	rep := tick(e)

	if len(rep.Errors) == 0 {
		t.Error("the shortfall was not reported on the tick")
	}
	mustCount(t, sink, KindPlatformError, 1)
	if got := p.lockCount("2"); got != 1 {
		t.Errorf("session 2 locked %d time(s), want 1: it was in the read, so it is inside a window "+
			"and the daemon has a duty to ask it (#168)", got)
	}
	// And the lockout is recorded as having begun, because it did.
	mustCount(t, sink, KindLockoutEntered, 1)
}

func TestAPartialReadDoesNotEndALockoutItCouldNotSee(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	tick(e)
	mustCount(t, sink, KindLockoutEntered, 1)

	// The next listing reads somebody else's session and loses kid's row. kid is still
	// sitting at a locked screen inside the same window.
	p.setSessions(desktop("7", "other"))
	p.partialRead(errors.New("session 2: loginctl show-session: exit status 1"))
	tick(e)

	// "Session is no longer listed" is a sentence about a logout. Written from a census that
	// could not read the row, it is a lie in the one record an operator reads as evidence —
	// and it drops the state that carries the override bookkeeping along with it.
	mustCount(t, sink, KindLockoutExited, 0)
	if got := e.State(); len(got) != 1 || got[0] != "2" {
		t.Errorf("State() = %v after a partial read, want the lockout on session 2 kept", got)
	}
}

func TestACompleteReadStillEndsALockout(t *testing.T) {
	// The guard above must be about partial reads and not a general refusal to notice a
	// logout, or a child who logged out mid-window would be recorded as locked out forever.
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	tick(e)
	mustCount(t, sink, KindLockoutEntered, 1)

	p.setSessions() // nobody is logged in, and the listing says so cleanly
	tick(e)

	mustCount(t, sink, KindLockoutExited, 1)
	if got := e.State(); len(got) != 0 {
		t.Errorf("State() = %v after a clean empty listing, want nothing enforcing", got)
	}
}

func TestATotalFailureStillStopsTheTick(t *testing.T) {
	// The other arm, so that "use what read" cannot quietly become "ignore errors". With no
	// rows at all there is nothing to tick, and the tick says so once.
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	p.setSessionsErr(errors.New("loginctl is not installed"))

	rep := tick(e)
	if len(rep.Errors) == 0 {
		t.Error("a listing that returned nothing reported no error")
	}
	mustCount(t, sink, KindPlatformError, 1)
	if got := p.lockCount("2"); got != 0 {
		t.Errorf("locked %d time(s) on a read that named no session", got)
	}
	mustCount(t, sink, KindLockoutEntered, 0)
}

// TestATotalFailureDoesNotEndALockout is the exit sweep. With no rows at all the tick has to
// stop where it is: falling through means every session already in e.state reads as "no longer
// listed", so a logind that died writes lockout.exited for a child sitting at a locked screen
// and drops the state carrying the override bookkeeping along with it. The tick still says
// nothing about a lockout beginning, which is what the test above this one already pins.
func TestATotalFailureDoesNotEndALockout(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	tick(e)
	mustCount(t, sink, KindLockoutEntered, 1)

	p.setSessionsErr(errors.New("loginctl: signal: killed"))
	tick(e)

	mustCount(t, sink, KindLockoutExited, 0)
	if got := e.State(); len(got) != 1 || got[0] != "2" {
		t.Errorf("State() = %v after the backend died mid-window, want the lockout on session 2 "+
			"kept: nothing in this tick proves anybody logged out (#168)", got)
	}
}
