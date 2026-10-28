package enforce

// lock.released: the tamper-evidence line for #177.
//
// #177 measured that org.gnome.ScreenSaver.SetActive is exported to the controlled
// user on their own session bus and dismisses a password-gated lock with no password.
// The loop already absorbs that — it re-asserts every poll whatever the session says
// (#29) — so what was missing was not containment but visibility: an absorbed
// dismissal and a quiet night both write lock.asserted, identically, forever.
//
// Every test here pins one of two things: the line appears when the evidence is there,
// and it does NOT appear when it isn't. None of them pins whether the engine locks,
// because that is the #29 invariant and it is pinned by its own test — a signal the
// child owns may add a line to the audit file and nothing else.

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// errDesktopDown stands in for a backend that could not be reached at all — the branch
// that is neither a confirmation nor a dismissal.
var errDesktopDown = errors.New("fake: gdbus could not reach the session bus")

// locked returns the fixture session reporting itself locked: the state the desktop
// writes when the screen goes on, and, per #29, the state the child can write.
func locked(s platform.Session) platform.Session {
	s.Locked = true
	return s
}

func TestAConfirmedLockThatComesBackIsReportedOnce(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)

	// Poll 1: confirmed, but the hint has not moved yet. There is nothing to compare
	// against, and inventing a release here is the false alarm this rule avoids.
	tick(e)
	mustCount(t, sink, KindLockReleased, 0)

	// Poll 2: the desktop has written the hint, so the lock is now both confirmed and
	// seen. Still nothing to report.
	p.setSessions(locked(desktop("2", "kid")))
	tick(e)
	mustCount(t, sink, KindLockReleased, 0)

	// Poll 3: the hint is gone. That is the transition, and it is the line.
	p.setSessions(desktop("2", "kid"))
	tick(e)
	mustCount(t, sink, KindLockReleased, 1)

	// Poll 4: still unlocked, still one line. The transition was consumed; a screen
	// left unlocked is one event, not an event per poll.
	tick(e)
	mustCount(t, sink, KindLockReleased, 1)

	// And a NEW transition is a new line, because the first one is three hours stale by
	// the time the second happens.
	p.setSessions(locked(desktop("2", "kid")))
	tick(e)
	p.setSessions(desktop("2", "kid"))
	tick(e)
	mustCount(t, sink, KindLockReleased, 2)
}

// The false-alarm pin, and the reason the rule is a transition rather than a level. A
// locker that corroborates a lock without ever writing LockedHint is a healthy machine:
// if "the hint says unlocked" were enough on its own, every poll of every such machine
// would accuse someone of dismissing the lock.
func TestAReleaseIsNeverReportedForASessionThatNeverReportedLocked(t *testing.T) {
	e, _, sink, _, _ := newEngine(t, kidDoc(), refInside)

	for i := 0; i < 12; i++ {
		tick(e)
	}
	mustCount(t, sink, KindLockReleased, 0)
	mustCount(t, sink, KindLockAsserted, 12)
}

// The #29 invariant, restated for this feature: finding a dismissal must never become a
// reason to stop locking, or the tamper report is itself the fail-open.
func TestReportingAReleaseNeverChangesWhetherTheLockIsAsserted(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)

	p.setSessions(locked(desktop("2", "kid")))
	tick(e)
	p.setSessions(desktop("2", "kid"))
	for i := 0; i < 5; i++ {
		tick(e)
	}

	mustCount(t, sink, KindLockReleased, 1)
	mustCount(t, sink, KindLockAsserted, 6)
	if got := p.lockCount("2"); got != 6 {
		t.Errorf("platform.Lock called %d times over 6 polls, want 6: a release may add an audit line and must never skip a lock", got)
	}
}

// A child who dismisses as fast as the daemon re-asserts is the case the count exists
// for. Each dismissal is its own line — the poll interval is what bounds the rate, not
// a dedupe — and each one says how many this lockout has seen, because "once tonight"
// and "nineteen times tonight" are different problems and the trail is the only place
// that knows which it was.
func TestEveryDismissalIsReportedAndCarriesItsCount(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)

	for i := 1; i <= 12; i++ {
		p.setSessions(locked(desktop("2", "kid")))
		tick(e)
		p.setSessions(desktop("2", "kid"))
		tick(e)
		mustCount(t, sink, KindLockReleased, i)
	}

	last := sink.kindsOf(KindLockReleased)
	if got := last[len(last)-1].Detail; !strings.Contains(got, "12 times") {
		t.Errorf("the twelfth release says %q, want the running count in it", got)
	}
	if got := p.lockCount("2"); got != 24 {
		t.Errorf("locked %d times over 24 polls, want 24", got)
	}
}

// An override hands the child their desktop back on purpose. If the flag survived the
// exemption, the first poll after it expired would report the parent's own grant as the
// child dismissing a lock — the one false accusation this line cannot afford, because
// it is the line a parent acts on.
func TestAnOverrideDoesNotReportTheParentsOwnGrantAsADismissal(t *testing.T) {
	doc := kidDoc()
	e, p, sink, _, path := newEngine(t, doc, refInside)

	p.setSessions(locked(desktop("2", "kid")))
	tick(e)
	mustCount(t, sink, KindLockReleased, 0)

	doc.Overrides = []config.Override{{Username: "kid", Until: refInside.Add(time.Hour)}}
	writeDoc(t, path, doc)
	p.setSessions(desktop("2", "kid"))
	tick(e)
	mustCount(t, sink, KindLockReleased, 0)

	doc.Overrides = nil
	writeDoc(t, path, doc)
	before := p.lockCount("2")
	tick(e)
	mustCount(t, sink, KindOverrideExpired, 1)
	mustCount(t, sink, KindLockReleased, 0)
	if p.lockCount("2") != before+1 {
		t.Errorf("locked %d times after the exemption ended, want one more: the asking has to resume", p.lockCount("2"))
	}
}

// Yesterday's lockout is not evidence about tonight's. Without the reset on exit, the
// first poll of a new window — screen unlocked, because the child was legitimately
// released when the curfew ended — would open tonight's trail by accusing them of a
// dismissal.
func TestANewLockoutDoesNotReportLastNightsLockAsADismissal(t *testing.T) {
	e, p, sink, clock, _ := newEngine(t, kidDoc(), refInside)

	p.setSessions(locked(desktop("2", "kid")))
	tick(e)
	mustCount(t, sink, KindLockReleased, 0)

	// Out of the window: the lockout ends legitimately and the screen comes off.
	clock.Add(36 * time.Hour)
	p.setSessions(desktop("2", "kid"))
	tick(e)
	mustCount(t, sink, KindLockoutExited, 1)
	mustCount(t, sink, KindLockReleased, 0)

	// Back inside the same curfew, unlocked, exactly as it should be on the first tick.
	clock.Add(-12 * time.Hour)
	tick(e)
	mustCount(t, sink, KindLockoutEntered, 2)
	mustCount(t, sink, KindLockReleased, 0)
}

// A platform error is the absence of information, not evidence the screen came off.
// Clearing the flag on that path would hide a real dismissal behind a gdbus blip, so
// the evidence has to survive the outage.
func TestAPlatformErrorDoesNotEraseTheEvidenceOfALock(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)

	p.setSessions(locked(desktop("2", "kid")))
	tick(e)
	mustCount(t, sink, KindLockReleased, 0)

	// The desktop stops answering. The screen still reports locked, and nothing is
	// reportable while that is true.
	p.setLockErr("2", errDesktopDown)
	tick(e)
	mustCount(t, sink, KindPlatformError, 1)
	mustCount(t, sink, KindLockReleased, 0)

	// It answers again and now says unlocked. The evidence of the earlier lock outlived
	// the outage, so the dismissal is still reported.
	p.setLockErr("2", nil)
	p.setSessions(desktop("2", "kid"))
	tick(e)
	mustCount(t, sink, KindLockReleased, 1)
}
