package enforce

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// The tests in this file drive Tick directly: a fake clock, a fake platform, no
// sleeping. Each one names the behaviour it pins and, where the behaviour exists
// because of a measurement rather than a preference, the measurement.

func TestEnteringAWindowLocksAndAuditsTheEntryOnce(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)

	rep := tick(e)

	if got := p.lockCount("2"); got != 1 {
		t.Errorf("locked %d times on entry, want 1", got)
	}
	if rep.Asserted != 1 {
		t.Errorf("Report.Asserted = %d, want 1", rep.Asserted)
	}
	if rep.Sessions != 1 {
		t.Errorf("Report.Sessions = %d, want 1", rep.Sessions)
	}
	mustCount(t, sink, KindLockoutEntered, 1)
	mustCount(t, sink, KindLockAsserted, 1)

	// Payload, not just count: this is the line a parent reads to learn when the
	// child was locked out and by which rule.
	ev := sink.kindsOf(KindLockoutEntered)[0]
	if ev.Username != "kid" || ev.SessionID != "2" {
		t.Errorf("entry event names %s/%s, want kid/2", ev.Username, ev.SessionID)
	}
	if ev.RuleID != "r-curfew" || ev.RuleName != "School week" {
		t.Errorf("entry event names rule %q/%q, want r-curfew/School week", ev.RuleID, ev.RuleName)
	}
	if !ev.At.Equal(refInside) {
		t.Errorf("entry event stamped %s, want %s", ev.At, refInside)
	}
}

func TestADismissedLockIsReAssertedOnTheVeryNextTick(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)

	for i := 0; i < 3; i++ {
		tick(e)
	}

	// The product's whole purpose: a child who dismisses the lock screen gets it
	// again on the next tick, not at the next convenient moment.
	if got := p.lockCount("2"); got != 3 {
		t.Errorf("locked %d times over 3 ticks, want 3", got)
	}
	// ...while the audit trail still says one lockout started, not three.
	mustCount(t, sink, KindLockoutEntered, 1)
	mustCount(t, sink, KindLockAsserted, 3)
}

// TestASessionThatClaimsItIsLockedIsLockedAnyway is the #29 regression, and the
// single most important test in this package.
//
// #29 measured that SetLockedHint is authorized by uid alone — login1 defines 37
// polkit actions and none of them covers the hint — so a child can write
// LockedHint=true from their own session, with no sudo and no prompt, from one
// autostart file. The engine used to read that hint and skip the lock. The result
// was total and silent: no lock, no error, and an audit line that was true as far as
// the daemon knew.
//
// Reverting step 5 to "if s.Locked { leave it alone }" fails this test. That is the
// point of it.
func TestASessionThatClaimsItIsLockedIsLockedAnyway(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)

	forged := desktop("2", "kid")
	forged.Locked = true
	p.setSessions(forged)

	for i := 0; i < 4; i++ {
		tick(e)
	}

	if got := p.lockCount("2"); got != 4 {
		t.Errorf("locked %d times over 4 ticks while the session claimed it was locked, want 4: "+
			"a hint the controlled user can write must never decide when enforcement stops", got)
	}
	mustCount(t, sink, KindLockoutEntered, 1)
	mustCount(t, sink, KindLockAsserted, 4)
	if len(e.State()) != 1 {
		t.Errorf("State() = %v, want the session to be listed as enforced", e.State())
	}

	// The trail has to say the engine overrode the claim, so an operator reading
	// the file can see the forgery attempt rather than infer it.
	first := sink.kindsOf(KindLockAsserted)[0]
	if !strings.Contains(first.Detail, "reported itself locked") {
		t.Errorf("lock.asserted detail = %q, want it to name the untrusted hint", first.Detail)
	}
}

func TestExitingTheWindowStopsLockingAndAuditsTheExitOnce(t *testing.T) {
	e, p, sink, clock, _ := newEngine(t, kidDoc(), refInside)
	tick(e)

	// Wednesday midday: Tuesday's 20:00-06:30 window has closed.
	clock.Set(refOutside.AddDate(0, 0, 1))
	tick(e)
	tick(e)

	if got := p.lockCount("2"); got != 1 {
		t.Errorf("locked %d times, want the single lock from before the window closed", got)
	}
	mustCount(t, sink, KindLockoutExited, 1)
	mustCount(t, sink, KindLockoutEntered, 1)

	ev := sink.kindsOf(KindLockoutExited)[0]
	if ev.RuleID != "r-curfew" {
		t.Errorf("exit event names rule %q, want the rule that ended", ev.RuleID)
	}
	if ev.Username != "kid" {
		t.Errorf("exit event names %q, want kid", ev.Username)
	}
	if len(e.State()) != 0 {
		t.Errorf("State() = %v after the window closed, want empty", e.State())
	}
}

func TestAnActiveOverrideSuppressesLockingAndResumesWhenItExpires(t *testing.T) {
	doc := kidDoc()
	doc.Overrides = []config.Override{{
		Username: "kid", Until: refInside.Add(10 * time.Minute),
		Reason: "homework", By: "parent",
	}}
	e, p, sink, clock, _ := newEngine(t, doc, refInside)

	tick(e)
	tick(e)
	if got := p.totalLocks(); got != 0 {
		t.Errorf("locked %d times under an override, want 0", got)
	}
	mustCount(t, sink, KindLockoutEntered, 0)

	// The tick after the exemption runs out locks again, and says once that it did.
	clock.Add(11 * time.Minute)
	tick(e)
	if got := p.lockCount("2"); got != 1 {
		t.Errorf("locked %d times after the override expired, want 1", got)
	}
	mustCount(t, sink, KindOverrideExpired, 1)
	mustCount(t, sink, KindLockoutEntered, 1)

	tick(e)
	mustCount(t, sink, KindOverrideExpired, 1)
}

func TestAnOverrideGrantedMidLockoutStopsLockingWithoutClaimingTheWindowEnded(t *testing.T) {
	doc := kidDoc()
	e, p, sink, _, path := newEngine(t, doc, refInside)
	tick(e)

	doc.Overrides = []config.Override{{Username: "kid", Until: refInside.Add(time.Hour)}}
	writeDoc(t, path, doc)
	tick(e)

	if got := p.totalLocks(); got != 1 {
		t.Errorf("locked %d times, want the one lock from before the override", got)
	}
	// lockout.exited would say the window closed. It did not: an exemption started,
	// and the line that explains the gap is override.expired when it ends.
	mustCount(t, sink, KindLockoutExited, 0)
	mustCount(t, sink, KindOverrideExpired, 0)
}

func TestSessionsThatMustNeverBeLockedAreLeftAlone(t *testing.T) {
	doc := kidDoc()
	doc.Users = append(doc.Users, config.User{Username: "baby", ProfileID: "p-default", Enabled: false})
	e, p, sink, _, _ := newEngine(t, doc, refInside)

	remote := desktop("12", "kid")
	remote.Remote = true
	closing := desktop("13", "kid")
	closing.State = "closing"
	greeter := desktop("14", "kid")
	greeter.Class = "greeter"
	manager := desktop("15", "kid")
	manager.Seat = ""
	untyped := desktop("16", "kid")
	untyped.Type = "unspecified"

	p.setSessions(
		desktop("10", "ghost"), // not a managed account at all
		desktop("11", "baby"),  // managed, but disabled
		remote,                 // the admin's own SSH session
		closing,                // on its way out
		greeter,                // the login screen
		manager,                // a session with no console
		untyped,                // logind says "unspecified"
	)

	rep := tick(e)

	if got := p.totalLocks(); got != 0 {
		t.Errorf("locked %d sessions that must never be locked", got)
	}
	mustCount(t, sink, KindLockoutEntered, 0)
	if len(rep.Errors) != 0 {
		t.Errorf("tick reported %v, want no errors for sessions that are simply not ours to lock", rep.Errors)
	}
}

func TestAConfigEditMidWindowIsObeyedOnTheNextTick(t *testing.T) {
	doc := kidDoc()
	// A window that is shut at 22:30, so the first tick must do nothing.
	doc.Profiles[0].Rules = []config.Rule{{
		ID: "r-brief", Name: "Brief", Days: []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"},
		Start: "09:00", End: "10:00", Enabled: true,
	}}
	e, p, sink, _, path := newEngine(t, doc, refInside)

	tick(e)
	if got := p.totalLocks(); got != 0 {
		t.Fatalf("locked %d times outside the window, want 0", got)
	}

	// The parent edits the file while the child is sitting at the machine.
	doc.Profiles[0].Rules = []config.Rule{curfew()}
	writeDoc(t, path, doc)
	tick(e)

	if got := p.lockCount("2"); got != 1 {
		t.Errorf("locked %d times after the edit, want 1: a saved rule must not wait for a restart", got)
	}
	mustCount(t, sink, KindLockoutEntered, 1)
}

func TestAPlatformErrorKeepsTheLockoutAndIsRetried(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	tick(e)

	p.setSessionsErr(errors.New("logind is restarting"))
	rep := tick(e)
	if len(rep.Errors) == 0 {
		t.Error("a failed enumeration reported no error")
	}
	mustCount(t, sink, KindPlatformError, 1)
	// "Could not ask" is not evidence the window ended, so the lockout stands.
	if len(e.State()) != 1 {
		t.Errorf("State() = %v after a platform error, want the lockout kept", e.State())
	}
	mustCount(t, sink, KindLockoutExited, 0)

	p.setSessionsErr(nil)
	tick(e)
	// Recovery re-locks without pretending a second lockout began.
	if got := p.lockCount("2"); got != 2 {
		t.Errorf("locked %d times after recovery, want 2", got)
	}
	mustCount(t, sink, KindLockoutEntered, 1)
}

func TestASessionNothingCanLockIsEscalatedOnce(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	p.setLockErr("2", errors.New("no backend reached the desktop"))

	for i := 0; i < unenforceableAfter+3; i++ {
		tick(e)
	}

	// Every tick is reported, because "not right now" is worth seeing each time...
	mustCount(t, sink, KindPlatformError, unenforceableAfter+3)
	// ...but the verdict "this child is not being locked out" is said once, not
	// two hundred times a night.
	mustCount(t, sink, KindUnenforceable, 1)
	if got := p.lockCount("2"); got != unenforceableAfter+3 {
		t.Errorf("gave up after %d attempts, want it to keep asking", got)
	}
	// A lock that never happened is not a lockout, and the trail must not say it was.
	mustCount(t, sink, KindLockoutEntered, 0)
}

func TestAnUnconfirmedLockIsEnforcingButSaidOutLoud(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	p.setLockErr("2", fmt.Errorf("logind: %w", platform.ErrUnconfirmed))

	tick(e)
	rep := tick(e)

	if len(rep.Errors) == 0 {
		t.Error("an unconfirmed lock reported no error")
	}
	if got := p.lockCount("2"); got != 2 {
		t.Errorf("locked %d times, want 2: unconfirmed means keep re-arming", got)
	}
	// One line for the episode, not one per tick (#35). The per-tick verdict is still
	// in rep.Errors, which the check above reads; what changed is the audit file.
	mustCount(t, sink, KindLockUnconfirmed, 1)
	mustCount(t, sink, KindLockoutEntered, 1)
	// A desktop session that cannot confirm today has not been shown to be
	// unenforceable — the screensaver interface exposes no Locked property at all,
	// so the screen may be locked right now. Escalating that would be a false alarm.
	mustCount(t, sink, KindUnenforceable, 0)
	if len(e.State()) != 1 {
		t.Errorf("State() = %v, want the session treated as enforced", e.State())
	}
}

// TestARawVTIsEscalatedAsUnenforceableOnce covers the case platform.IsLockable
// deliberately refuses to filter out. A raw virtual terminal answers loginctl with
// exit 0 and nothing on it ever calls SetLockedHint, so ErrUnconfirmed is not a
// transient state there — it is permanent by construction. Dropping tty sessions
// from the listing was considered and rejected: the loop would then find nobody to
// enforce and go quiet while the child kept typing. So the verdict is reached here.
func TestARawVTIsEscalatedAsUnenforceableOnce(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	vt := desktop("2", "kid")
	vt.Type = "tty"
	p.setSessions(vt)
	p.setLockErr("2", platform.ErrUnconfirmed)

	for i := 0; i < 6; i++ {
		tick(e)
	}

	mustCount(t, sink, KindUnenforceable, 1)
	// The per-tick unconfirmed line is suppressed once the verdict is in: it would
	// say the same thing 200 times a night without adding that it never changes.
	mustCount(t, sink, KindLockUnconfirmed, 0)
	if got := p.lockCount("2"); got != 6 {
		t.Errorf("locked %d times, want 6: escalating is a report, not a reason to stop trying", got)
	}
}

func TestEnforcementOffIsANoOpAndTheTransitionIsAuditedOnce(t *testing.T) {
	doc := kidDoc()
	doc.Settings.Enforcement = false
	e, p, sink, _, path := newEngine(t, doc, refInside)

	tick(e)
	tick(e)
	if got := p.totalLocks(); got != 0 {
		t.Errorf("locked %d times with enforcement off, want 0", got)
	}
	if p.calls != 0 {
		t.Errorf("enumerated sessions %d times with enforcement off, want 0", p.calls)
	}
	mustCount(t, sink, KindEnforcementDisabled, 1)
	mustCount(t, sink, KindEnforcementEnabled, 0)

	doc.Settings.Enforcement = true
	writeDoc(t, path, doc)
	tick(e)
	mustCount(t, sink, KindEnforcementEnabled, 1)
	// Switching the daemon back on mid-bedtime re-audits the entry: a trail that
	// resumed locking without saying it started is a trail that misleads.
	mustCount(t, sink, KindLockoutEntered, 1)
	if got := p.totalLocks(); got != 1 {
		t.Errorf("locked %d times after switching back on, want 1", got)
	}
}

// TestSwitchingEnforcementOffAndOnReAuditsTheLockout is the half of that behaviour
// the test above cannot see.
//
// Turning the daemon off while nothing was ever enforced leaves the state map empty
// either way, so clearing it is invisible. The case that matters is a lockout that
// was running when the switch was thrown: the parent paused enforcement for a week,
// flips it back on at 22:00 on a Tuesday, and the audit file has to say the lockout
// started again then. If the old per-session state survives the pause, the engine
// believes it is still enforcing, emits nothing, and the trail shows a lockout that
// has been running continuously through the week it was not.
func TestSwitchingEnforcementOffAndOnReAuditsTheLockout(t *testing.T) {
	e, p, sink, _, path := newEngine(t, kidDoc(), refInside)

	doc := kidDoc()
	tick(e)
	mustCount(t, sink, KindLockoutEntered, 1)
	if len(e.State()) != 1 {
		t.Fatalf("State() = %v before the pause, want the session enforced", e.State())
	}

	doc.Settings.Enforcement = false
	writeDoc(t, path, doc)
	tick(e)
	if len(e.State()) != 0 {
		t.Errorf("State() = %v while enforcement is off, want nothing claimed", e.State())
	}
	mustCount(t, sink, KindEnforcementDisabled, 1)
	// Pausing is not a release: the window is still open, and lockout.exited would
	// say the schedule let the child go.
	mustCount(t, sink, KindLockoutExited, 0)

	doc.Settings.Enforcement = true
	writeDoc(t, path, doc)
	tick(e)

	mustCount(t, sink, KindLockoutEntered, 2)
	if got := p.lockCount("2"); got != 2 {
		t.Errorf("locked %d times across the pause, want one on each side of it", got)
	}
	if len(e.State()) != 1 {
		t.Errorf("State() = %v after switching back on, want the session enforced again", e.State())
	}
}

func TestAnUnreadableConfigKeepsEnforcingTheLastGoodDocument(t *testing.T) {
	e, p, sink, _, path := newEngine(t, kidDoc(), refInside)
	tick(e)

	if err := os.WriteFile(path, []byte("{ this is not json"), 0o600); err != nil {
		t.Fatalf("corrupt the config: %v", err)
	}

	rep := tick(e)
	if len(rep.Errors) == 0 {
		t.Error("an unreadable config reported no error")
	}
	mustCount(t, sink, KindConfigStale, 1)
	// The failure mode this pins: a half-written file must not free the child.
	if got := p.lockCount("2"); got != 2 {
		t.Errorf("locked %d times after the config became unreadable, want 2", got)
	}

	// It keeps being reported while it lasts — a parent who saved a broken edit has
	// to learn the edit did not land, on every tick until it does.
	tick(e)
	mustCount(t, sink, KindConfigStale, 2)
}

func TestAConfigThatHasNeverLoadedLocksNobody(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.json")
	p := &fakePlatform{sessions: []platform.Session{desktop("2", "kid")}}
	sink := &recSink{}
	e := New(path, p, sink)
	e.Clock = func() time.Time { return refInside }

	rep := tick(e)

	// Locking everyone is the other failure mode, and it is worse: it locks the
	// parent out of the machine and out of the UI that would fix the file. The
	// answer is to do nothing loudly, which is what this pins.
	if got := p.totalLocks(); got != 0 {
		t.Errorf("locked %d times with no config at all, want 0", got)
	}
	if len(rep.Errors) == 0 {
		t.Error("a config that has never loaded reported no error")
	}
	if rep.Enforcement {
		t.Error("Report.Enforcement is true for a tick that had no settings to read")
	}
}

func TestASessionThatStopsBeingListedAuditsItsExitOnce(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	tick(e)

	p.setSessions() // logind reports nobody: logged out, or closing
	tick(e)
	tick(e)

	mustCount(t, sink, KindLockoutExited, 1)
	ev := sink.kindsOf(KindLockoutExited)[0]
	// The Session is gone by the time this is emitted, so the name has to have been
	// kept from the tick before. Losing it makes the line unreadable.
	if ev.Username != "kid" {
		t.Errorf("exit event names %q, want kid", ev.Username)
	}
	if ev.RuleID != "r-curfew" {
		t.Errorf("exit event names rule %q, want the rule that was being enforced", ev.RuleID)
	}
}

func TestADegradedScheduleIsReportedWhenItChanges(t *testing.T) {
	doc := kidDoc()
	// A zone that does not exist: every rule in the profile is now being read in the
	// machine's own zone, which is the one compromise in the evaluator that changes
	// what every rule means rather than dropping one.
	doc.Profiles[0].Timezone = "Europe/Berlinx"
	e, _, sink, _, path := newEngine(t, doc, refInside)

	for i := 0; i < 3; i++ {
		tick(e)
	}
	mustCount(t, sink, KindScheduleDegraded, 1)
	got := sink.kindsOf(KindScheduleDegraded)[0]
	if !strings.Contains(got.Detail, "Europe/Berlinx") {
		t.Errorf("degraded detail = %q, want it to name the zone that failed", got.Detail)
	}
	if got.Username != "kid" {
		t.Errorf("degraded event names %q, want kid", got.Username)
	}

	// Fixing it must not emit a second line, and must not leave the engine
	// believing the profile is still degraded.
	doc.Profiles[0].Timezone = "UTC"
	writeDoc(t, path, doc)
	tick(e)
	mustCount(t, sink, KindScheduleDegraded, 1)

	// And a fresh breakage is reported again rather than swallowed as "already
	// reported once".
	doc.Profiles[0].Rules = []config.Rule{{ID: "r-bad", Name: "Bad", Days: []string{"nonsense"}, Start: "25:00", End: "26:00", Enabled: true}}
	writeDoc(t, path, doc)
	tick(e)
	mustCount(t, sink, KindScheduleDegraded, 2)
}

func TestAnEnabledAccountWithNoProfileIsReportedNotSkipped(t *testing.T) {
	doc := kidDoc()
	doc.Profiles = nil // the user still points at p-default
	e, p, sink, _, _ := newEngine(t, doc, refInside)

	rep := tick(e)
	if got := p.totalLocks(); got != 0 {
		t.Errorf("locked %d times with no profile to evaluate, want 0", got)
	}
	if len(rep.Errors) == 0 {
		t.Error("an enabled account with no profile reported no error")
	}
	// Repeated by design: this is a document the validator should have rejected, and
	// one line at startup is how it stays unfixed while a child goes unenforced.
	tick(e)
	mustCount(t, sink, KindPlatformError, 2)
}

func TestReportEventsAreTheEventsTheSinkReceived(t *testing.T) {
	e, _, sink, _, _ := newEngine(t, kidDoc(), refInside)

	rep := tick(e)
	sinkEvents := sink.all()

	if len(rep.Events) != len(sinkEvents) {
		t.Fatalf("Report has %d events, the sink got %d; they are documented as the same set",
			len(rep.Events), len(sinkEvents))
	}
	for i := range rep.Events {
		if rep.Events[i].Kind != sinkEvents[i].Kind {
			t.Errorf("event %d: Report says %s, sink got %s", i, rep.Events[i].Kind, sinkEvents[i].Kind)
		}
	}
}

// TestASecondLockoutReEscalates guards the per-lockout bookkeeping. A flag that is
// set once and never cleared means a child who was unenforceable last night is never
// reported again, and the whole point of the escalation is that it recurs until
// somebody fixes it.
func TestASecondLockoutReEscalates(t *testing.T) {
	e, p, sink, clock, _ := newEngine(t, kidDoc(), refInside)
	p.setLockErr("2", errors.New("no backend reached the desktop"))

	for i := 0; i < unenforceableAfter; i++ {
		tick(e)
	}
	mustCount(t, sink, KindUnenforceable, 1)

	// Out of the window, then back in the next night. The counter restarts with the
	// new lockout — "5 ticks in a row" is a statement about now, not about history —
	// so the second escalation lands on the fifth failed tick of the second window.
	clock.Set(refOutside.AddDate(0, 0, 1))
	tick(e)
	clock.Set(refInside.AddDate(0, 0, 1))
	for i := 0; i < unenforceableAfter; i++ {
		tick(e)
	}

	mustCount(t, sink, KindUnenforceable, 2)
}

// TestALockedGNOMEScreenDoesNotWriteAnAlarmEveryTick is #35.
//
// GNOME draws its lock screen inside gnome-shell, so there is no separate locker
// process and nothing corroborates a lock once it has taken: a GNOME-only chain
// confirms on the tick the hint moves and answers ErrUnconfirmed on every tick after.
// Before #35 that wrote "the child may still be at the desktop" every 30 seconds
// forever on a machine that is working correctly.
//
// The volume is the bug. A signal that fires on a healthy machine is a signal the
// operator learns to skip, and the one time it is true -- the screen really did come
// unlocked -- is then read as more of the same.
func TestALockedGNOMEScreenDoesNotWriteAnAlarmEveryTick(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)

	// Tick 1: the lock corroborates. This is the healthy GNOME tick.
	tick(e)
	mustCount(t, sink, KindLockAsserted, 1)
	mustCount(t, sink, KindLockUnconfirmed, 0)

	// Ticks 2-10: corroboration stops, which is GNOME's steady state.
	p.setLockErr("2", fmt.Errorf("gnome: %w", platform.ErrUnconfirmed))
	var last Report
	for i := 0; i < 9; i++ {
		last = tick(e)
	}

	mustCount(t, sink, KindLockUnconfirmed, 1)
	if got := p.lockCount("2"); got != 10 {
		t.Errorf("locked %d times, want 10: quieting the audit line must not quiet the asking", got)
	}
	// The live verdict is undiminished. Every tick still tells the caller the lock
	// could not be proven; only the file stops repeating it.
	if len(last.Errors) != 1 {
		t.Errorf("the tick reported %d errors, want the unconfirmed fault on every tick", len(last.Errors))
	}
}

// The episode boundary is what the alarm means, so a second one has to produce a
// second line. A rule that fired once per process would go silent for the rest of an
// uptime measured in weeks, which is a worse failure than the flooding it replaced.
func TestCorroborationReturningArmsTheAlarmAgain(t *testing.T) {
	e, p, sink, _, _ := newEngine(t, kidDoc(), refInside)
	p.setLockErr("2", fmt.Errorf("gnome: %w", platform.ErrUnconfirmed))

	tick(e)
	tick(e)
	mustCount(t, sink, KindLockUnconfirmed, 1)

	// Corroboration comes back: the hint moves, the lock confirms.
	p.setLockErr("2", nil)
	tick(e)
	mustCount(t, sink, KindLockAsserted, 1)

	// And goes away again. That is a new episode and the operator should hear about it.
	p.setLockErr("2", fmt.Errorf("gnome: %w", platform.ErrUnconfirmed))
	tick(e)
	tick(e)
	mustCount(t, sink, KindLockUnconfirmed, 2)
}

// A lockout that ends and starts again is a new episode too. The state entry survives
// the release so the exit event can name the rule that ended, which means the flag has
// to be cleared explicitly rather than relying on the entry being fresh.
func TestANewLockoutReportsUnconfirmedAgain(t *testing.T) {
	e, p, sink, clock, _ := newEngine(t, kidDoc(), refInside)
	p.setLockErr("2", fmt.Errorf("gnome: %w", platform.ErrUnconfirmed))

	tick(e)
	tick(e)
	mustCount(t, sink, KindLockUnconfirmed, 1)
	mustCount(t, sink, KindLockoutExited, 0)

	// Walk out of the window, then back into it.
	clock.Add(36 * time.Hour)
	tick(e)
	mustCount(t, sink, KindLockoutExited, 1)

	// Back to Wednesday 22:30, inside the 20:00-06:30 curfew.
	clock.Add(-12 * time.Hour)
	tick(e)
	mustCount(t, sink, KindLockoutEntered, 2)
	mustCount(t, sink, KindLockUnconfirmed, 2)
}

// An override does not delete the session's state entry -- the entry has to survive so
// override.expired can be audited when the exemption ends. That makes it the one release
// path where per-lockout bookkeeping can leak into the next lockout, and the alarm going
// missing is the failure: the parent granted an hour, the exemption ended, the screen
// stopped corroborating, and nothing says so.
func TestAnOverrideExpiringDoesNotCarryTheAlarmFlagIntoTheNextLockout(t *testing.T) {
	doc := kidDoc()
	e, p, sink, _, path := newEngine(t, doc, refInside)
	p.setLockErr("2", fmt.Errorf("gnome: %w", platform.ErrUnconfirmed))

	tick(e)
	tick(e)
	mustCount(t, sink, KindLockUnconfirmed, 1)

	doc.Overrides = []config.Override{{Username: "kid", Until: refInside.Add(time.Hour)}}
	writeDoc(t, path, doc)
	tick(e)

	// The exemption ends and the same window is still open.
	doc.Overrides = nil
	writeDoc(t, path, doc)
	tick(e)
	tick(e)

	mustCount(t, sink, KindOverrideExpired, 1)
	if got := p.lockCount("2"); got < 4 {
		t.Errorf("locked %d times, want the asking to resume after the override", got)
	}
	mustCount(t, sink, KindLockUnconfirmed, 2)
}
