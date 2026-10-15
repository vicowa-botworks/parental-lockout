package dashboard

// The dashboard has to answer the same question the enforcement loop answers: is this
// account actually being locked? It used to answer it from the schedule alone, which
// was true up to the moment the engine gained a reason to decline — after which the
// page could announce "casey is locked out until 06:30" about a child sitting at an
// unlocked screen. These tests are the half that keeps the two in step.

import (
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

func session(username string, uid uint32) platform.Session {
	return platform.Session{Username: username, UID: uid, Type: "wayland", Class: "user"}
}

// The premise, checked rather than assumed: the same configuration with the daemon
// outside anybody's session does hold the account. Without this the test below could
// pass because the row is never held for any reason at all.
func TestOutsideAnySessionTheAccountIsStillHeld(t *testing.T) {
	sessions := []platform.Session{session("casey", 1000)}
	v := Build(cfg(casey(true)), sessions, platform.SelfLocation{}, insideWindow)
	r := row(t, v, "casey")
	if !r.LockedOutNow {
		t.Fatalf("a session the daemon is not inside is not held: %+v", r)
	}
	if v.NotEnforcing != 0 {
		t.Errorf("NotEnforcing = %d, want 0 for an account being enforced", v.NotEnforcing)
	}
}

func TestADeclinedAccountIsNotShownAsLockedOut(t *testing.T) {
	sessions := []platform.Session{session("casey", 1000)}
	self := platform.SelfLocation{UID: 1000, InUserSlice: true}

	v := Build(cfg(casey(true)), sessions, self, insideWindow)
	r := row(t, v, "casey")

	if r.LockedOutNow {
		t.Errorf("the page says casey is locked out, and the engine is declining to lock "+
			"this session: %+v", r)
	}
	if !r.ScheduledButNotEnforcing {
		t.Errorf("an active window the daemon will not act on is not counted as scheduled "+
			"and unenforced: %+v", r)
	}
	if v.LockedOut != 0 {
		t.Errorf("LockedOut = %d, want 0", v.LockedOut)
	}
	if v.NotEnforcing != 1 {
		t.Errorf("NotEnforcing = %d, want 1", v.NotEnforcing)
	}

	// The count alone is not enough: "scheduled and not enforced" has three causes and
	// the fix for this one is not something a parent can guess.
	var explains bool
	for _, p := range r.Problems {
		if strings.Contains(p, "deploy/plock.service") {
			explains = true
		}
	}
	if !explains {
		t.Errorf("the row does not name the fix; problems = %q", r.Problems)
	}
}

// The engine locks session by session, so a second session elsewhere really is being
// held. Marking the account unenforced on the strength of one declined session would
// raise a false alarm about a child who is in fact locked out.
func TestOneDeclinedSessionAmongSeveralDoesNotMarkTheAccountUnenforced(t *testing.T) {
	sessions := []platform.Session{session("casey", 1000), session("casey", 1001)}
	self := platform.SelfLocation{UID: 1000, InUserSlice: true}

	v := Build(cfg(casey(true)), sessions, self, insideWindow)
	r := row(t, v, "casey")
	if !r.LockedOutNow {
		t.Errorf("casey has a session the daemon will lock, and the page does not say so: %+v", r)
	}
	if v.NotEnforcing != 0 {
		t.Errorf("NotEnforcing = %d, want 0 while any session is still being locked", v.NotEnforcing)
	}
}

// An account with nobody logged in is not "declined" — there is nothing to decline.
// Folding that case in would mark every logged-out account unenforced and bury the
// rows that mean something is actually wrong.
func TestAnAccountWithNoSessionIsNotReportedAsDeclined(t *testing.T) {
	self := platform.SelfLocation{UID: 1000, InUserSlice: true}
	v := Build(cfg(casey(true)), nil, self, insideWindow)
	r := row(t, v, "casey")
	if r.ScheduledButNotEnforcing {
		t.Errorf("an account with no session is reported as declined: %+v", r)
	}
	if !r.LockedOutNow {
		t.Errorf("an account with no session inside its window stopped being held; that is "+
			"a different change than the decline rule: %+v", r)
	}
}

// The banner is the sentence a parent reads on a phone from another room, and it names
// its causes. A decline counted in NotEnforcing but absent from the list of causes is
// a sentence that explains the number wrongly.
func TestTheBannerNamesTheDeclineAmongItsCauses(t *testing.T) {
	sessions := []platform.Session{session("casey", 1000)}
	self := platform.SelfLocation{UID: 1000, InUserSlice: true}

	// Inside the window, which is the case the page gets wrong without this rule: the
	// schedule says held, the engine says no, and the banner is what a parent trusts.
	v := Build(cfg(casey(true)), sessions, self, insideWindow)
	if v.NotEnforcing == 0 {
		t.Fatalf("the decline did not reach NotEnforcing; the banner cannot name it: %+v", v)
	}
	if !strings.Contains(v.Banner, "declining to lock its own session") {
		t.Errorf("the banner counts the decline but does not name it as a cause:\n%s", v.Banner)
	}
}
