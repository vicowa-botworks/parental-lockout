package enforce

import (
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// ownSession puts the engine inside uid 1000's login session. The desktop() fixture's
// sessions carry uid 1001, so a test that wants the daemon and the target to share a
// uid has to say which uid the session has; that is what withUID is for.
func withUID(s platform.Session, uid uint32) platform.Session {
	s.UID = uid
	return s
}

// twoKids is kidDoc plus a second managed account, so a test can show the rule
// declining for one account while still locking another. A rule that skipped every
// session would pass a test that only has one session.
func twoKids() *config.Config {
	doc := kidDoc()
	doc.Profiles = append(doc.Profiles, config.Profile{
		ID: "p-second", Name: "Second", Timezone: "UTC",
		Rules: []config.Rule{curfew()},
	})
	doc.Users = append(doc.Users, config.User{
		Username: "other", ProfileID: "p-second", Enabled: true,
	})
	return doc
}

// The daemon must not lock the session it is running in. The reason is not politeness:
// stopping the daemon needs a keyboard, and the keyboard is behind the lock it just put
// up. The override path does not help, because granting one needs the web UI or the CLI
// on the machine that is now locked.
func TestTheDaemonsOwnSessionIsNotLocked(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	e.Self.UID = 1001
	e.Self.InUserSlice = true

	tick(e)

	if got := p.totalLocks(); got != 0 {
		t.Errorf("locked %d time(s) a session the daemon is itself running in; the daemon "+
			"would have locked the only screen it can be stopped from", got)
	}
	mustCount(t, sink, KindLockoutEntered, 0)

	// Declining is not the same as being quiet about declining. A session that is not
	// being enforced has to say so, or the operator's only evidence is a screen that
	// stays unlocked.
	mustCount(t, sink, KindUnenforceable, 1)
	ev := sink.kindsOf(KindUnenforceable)[0]
	if !strings.Contains(ev.Detail, "running in") {
		t.Errorf("detail = %q; want it to name the reason the session was skipped", ev.Detail)
	}
	if !strings.Contains(ev.Detail, "deploy/plock.service") {
		t.Errorf("detail = %q; want it to name the fix, not just the refusal -- an operator "+
			"who reads 'not locking your own session' with no next step cannot act on it", ev.Detail)
	}
	if ev.Username != "kid" || ev.SessionID != "2" {
		t.Errorf("event names %q / session %q; want kid / 2", ev.Username, ev.SessionID)
	}
}

// The rule fires on the PAIR. SelfInUserSlice alone must not skip anything, because a
// systemd system unit -- the supported deployment -- has no user slice, and a daemon
// that skipped uid 0 or uid 1000 there would enforce nothing at all while looking
// healthy.
func TestTheOwnSessionRuleNeedsBothHalvesOfThePair(t *testing.T) {
	e, p, _, _, _ := newEngine(t, kidDoc(), refInside)
	e.Self.UID = 1001
	// SelfInUserSlice left false: what a system unit resolves to.
	tick(e)

	if got := p.totalLocks(); got != 1 {
		t.Errorf("locked %d time(s) with SelfInUserSlice false, want 1. A uid match alone "+
			"must never skip a session: in the supported deployment the daemon is root and "+
			"shares no slice with anybody, so this is the only thing keeping enforcement on",
			got)
	}
}

// The skip is per session, not global. Without a second account in the fixture, a rule
// that returned early for every session would pass the test above.
func TestOnlyTheDaemonsOwnUidIsSkipped(t *testing.T) {
	doc := twoKids()
	e, p, sink, _, _ := newEngine(t, doc, refInside)
	p.setSessions(
		withUID(desktop("2", "kid"), 1001),
		withUID(desktop("5", "other"), 1002),
	)
	e.Self.UID = 1001
	e.Self.InUserSlice = true

	tick(e)

	if got := p.totalLocks(); got != 1 {
		t.Errorf("locked %d session(s), want exactly 1 -- the other account's session", got)
	}
	// Asserted per session rather than in total, so "locked the wrong one" cannot be
	// mistaken for "locked the right one".
	if got := p.lockCount("2"); got != 0 {
		t.Errorf("the daemon's own session was locked %d time(s), want 0", got)
	}
	if got := p.lockCount("5"); got != 1 {
		t.Errorf("the other account's session was locked %d time(s), want 1 -- a rule that "+
			"skipped every session would pass the single-session test above", got)
	}
	mustCount(t, sink, KindUnenforceable, 1)
	if ev := sink.kindsOf(KindUnenforceable)[0]; ev.Username != "kid" {
		t.Errorf("the skip was reported for %q; want kid, whose uid is the daemon's own", ev.Username)
	}
}

// A deployment mistake that will never fix itself by itself must not produce one audit
// line every 30 seconds forever. This is the volume rule #35 exists for, and it is the
// reason the bookkeeping is keyed apart from the per-lockout state: both release paths
// reset the per-lockout flag, so reusing it here would re-announce on every tick.
func TestTheOwnSessionIsAnnouncedOnceNotEveryTick(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	e.Self.UID = 1001
	e.Self.InUserSlice = true

	for i := 0; i < 5; i++ {
		tick(e)
	}

	if got := p.totalLocks(); got != 0 {
		t.Errorf("locked %d time(s) across 5 ticks, want 0", got)
	}
	mustCount(t, sink, KindUnenforceable, 1)
}

// The window closing still has to look like a window closing, and the window opening
// must not announce a lockout the daemon then refuses to carry out.
func TestTheDaemonsOwnSessionNeverEntersALockout(t *testing.T) {
	e, _, sink, clock, _ := newEngine(t, kidDoc(), refOutside)
	e.Self.UID = 1001
	e.Self.InUserSlice = true

	tick(e) // outside the window
	clock.Add(11 * time.Hour)
	tick(e) // inside it

	mustCount(t, sink, KindLockoutEntered, 0)
	mustCount(t, sink, KindLockoutExited, 0)
	mustCount(t, sink, KindUnenforceable, 1)
}

// A new session id is a new thing to announce. The set is keyed by session, not by uid,
// so a child who logs out and back in gets one line for the new session rather than
// silence inherited from the old one.
func TestANewSessionOfTheOwnUidIsAnnouncedAgain(t *testing.T) {
	doc := twoKids()
	e, p, sink, _, _ := newEngine(t, doc, refInside)
	p.setSessions(withUID(desktop("2", "kid"), 1001))
	e.Self.UID = 1001
	e.Self.InUserSlice = true

	tick(e)
	p.setSessions(withUID(desktop("9", "kid"), 1001))
	tick(e)

	mustCount(t, sink, KindUnenforceable, 2)
}
