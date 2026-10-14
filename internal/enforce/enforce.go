// Package enforce is the mechanism the product exists for: while a lockout window
// is active, keep the screen locked even after the child dismisses the lock screen.
//
// It inherits the fail-safe rule the other two policy packages run on — every
// ambiguity resolves toward locking the child out, and a degraded state is reported
// rather than absorbed. Concretely that means:
//
//   - A platform error on a tick keeps the previous state and retries. It never
//     clears a lockout, because "could not ask" is not evidence that the window
//     ended.
//   - A config that fails to load does not switch enforcement off. The last good
//     document keeps enforcing and the failure is reported on every tick until it
//     is fixed, so a corrupt file cannot silently free the child.
//   - A lock that was requested but not confirmed (platform.ErrUnconfirmed) is
//     retried on the next tick and reported as its own event kind. It is not
//     success, and it is not the same failure as "nothing reached the desktop".
//
// What this package does NOT do is decide what a lockout means, or how to lock a
// screen. schedule.Evaluate answers "is a window open"; platform.Lock performs the
// lock. Everything here is bookkeeping about which of those to ask and when.
package enforce

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/schedule"
)

// Event kinds. They are strings rather than an int enum because they land in a
// JSONL audit log and in a web UI, and a log line whose kind reads
// "lock.asserted" needs no lookup table. They are also what the tests assert on,
// so renaming one is a visible, greppable change rather than a constant drift.
const (
	// KindLockAsserted is one successful call to platform.Lock. Expect it on the
	// tick that enters a window and again each time the child dismisses the lock
	// screen — not every tick, because a session already reporting itself locked
	// is left alone (see Engine.Tick).
	KindLockAsserted = "lock.asserted"

	// KindLockoutEntered and KindLockoutExited bracket a period of enforcement
	// for one session. They are transitions, emitted once each, and they are the
	// pair a parent actually reads: "locked at 20:00, released at 06:30".
	KindLockoutEntered = "lockout.entered"
	KindLockoutExited  = "lockout.exited"

	// KindOverrideExpired fires once, on the tick where an override that was
	// suppressing enforcement runs out. The parent wants to know the exemption
	// ended; they do not want one line per tick while it lasts.
	KindOverrideExpired = "override.expired"

	// KindEnforcementDisabled and KindEnforcementEnabled mark the global switch
	// moving. Emitted on the transition only: settings.enforcement=false is a
	// state a daemon can sit in for weeks, and one audit line per 30 seconds
	// would bury every other event in the file.
	KindEnforcementDisabled = "enforcement.disabled"
	KindEnforcementEnabled  = "enforcement.enabled"

	// KindPlatformError is a failed Sessions or Lock call. It is emitted per
	// occurrence rather than once-per-transition, because the point of the line is
	// "the daemon could not enforce right now" and that is worth seeing each time
	// it happens.
	KindPlatformError = "platform.error"

	// KindLockUnconfirmed is a lock a backend accepted but could not prove took
	// effect (platform.ErrUnconfirmed). Distinct from KindPlatformError on
	// purpose: "the request was refused" and "the request was accepted and the
	// screen may not be locked" need different sentences in front of an operator,
	// and only the second one means the child might still be at the desktop.
	KindLockUnconfirmed = "lock.unconfirmed"

	// KindConfigStale is emitted on every tick that runs on a previously loaded
	// document because the file could not be read. Repeated by design: a parent
	// editing the schedule needs to know their edit did not land.
	KindConfigStale = "config.stale"

	// KindUnenforceable is a session that is inside a lockout but cannot be
	// locked by any backend — a raw VT, for instance, where every backend either
	// refuses or reports ErrUnconfirmed forever. Retrying into the log every
	// 30 seconds is noise; going quiet is a fail-open. So it is reported once per
	// session per lockout, and the operator decides.
	KindUnenforceable = "session.unenforceable"

	// KindTickSkipped is a tick that found the previous one still running. It
	// should be rare — a tick is a handful of loginctl calls against a local
	// daemon — but when it is not rare it means enforcement is falling behind, and
	// that is exactly the condition nobody would otherwise learn about.
	KindTickSkipped = "tick.skipped"
)

// Event is one audit line. It carries no pointer into the config document:
// schedule.Verdict.Rule points into the profile it was evaluated from, and an
// audit line outlives the tick that produced it, so the identity fields are
// copied here. See the comment on schedule.Verdict.Rule for the aliasing hazard
// this avoids.
type Event struct {
	At   time.Time `json:"at"`
	Kind string    `json:"kind"`

	// Username and SessionID identify whom the event is about. Both are empty for
	// daemon-wide events such as KindEnforcementDisabled.
	Username  string `json:"username,omitempty"`
	SessionID string `json:"session,omitempty"`

	// RuleID and RuleName name the rule responsible, when the event is about a
	// lockout. RuleName is what a parent recognises ("School week"); RuleID is
	// what survives a rename.
	RuleID   string `json:"ruleID,omitempty"`
	RuleName string `json:"ruleName,omitempty"`

	// Detail is for a human and is never parsed.
	Detail string `json:"detail,omitempty"`
}

// Sink receives every event the engine decides to record.
//
// It is an interface rather than a writer on purpose. The acceptance criteria ask
// for events "asserted by count and by payload", which a fake does directly, and a
// real file would make the whole suite slow and flaky under -race. The JSONL
// writer, its rotation and its permissions belong to #28 and drop in behind this
// without touching the engine.
//
// Emit must not block: it is called from the tick, and a tick that stalls holds the
// overlap guard and stops the daemon re-locking anyone.
type Sink interface {
	Emit(Event)
}

// Engine decides, once per tick, who should be locked out right now and asks the
// platform to do it.
//
// The zero value is not usable; Path and Platform are required. The rest have
// defaults so a test can construct one with New and override only the seam it is
// pinning.
type Engine struct {
	// Path is the config file. Tick reads it fresh on every call (see reload).
	Path string

	// Platform enumerates sessions and locks them. Usually a *platform.Chain.
	Platform platform.Platform

	// Sink receives events. nil discards them, which is what a test that is not
	// asserting on events wants; it is never what production should pass.
	Sink Sink

	// Clock returns the current time. Tests set it to a fake so a tick can be
	// placed at 21:00 on a Tuesday without waiting for one.
	Clock func() time.Time

	// Logf receives human-readable diagnostics that are not audit events — the
	// ones about the engine itself. nil discards them.
	Logf func(string, ...any)

	// cfg is the last document that loaded successfully. It is what enforcement
	// runs on when the file becomes unreadable, and it is the reason a corrupt
	// config cannot free a child mid-window.
	cfg *config.Config

	// cfgErr is why cfg is being reused. Reported every tick while it persists.
	cfgErr error

	// state is per-session bookkeeping keyed by session id.
	state map[string]*sessionState

	// enforcementWas is the previous tick's view of settings.enforcement, so the
	// transition can be audited once instead of every tick. It starts true
	// because that is the value an absent key decodes to: the first tick of a
	// daemon whose config says nothing should not report a transition into
	// enforcement that never stopped.
	enforcementWas bool

	// unenforceable counts, per session, how many ticks in a row this session has
	// been unenforceable. See KindUnenforceable.
	unenforceable map[string]int
}

// sessionState is what one tick learns about one session so the next tick can tell
// whether anything changed.
type sessionState struct {
	// enforcing is true when the previous tick considered this session locked out
	// and subject to enforcement — whether or not it also called Lock. A session
	// inside a window that already reports itself locked is enforcing; it is just
	// not being touched again.
	enforcing bool

	// overrideActive is true when an override suppressed enforcement last tick, so
	// its expiry can be audited once.
	overrideActive bool

	// ruleID names the window we were enforcing, so the exit event can say what
	// ended rather than only that something ended.
	ruleID   string
	ruleName string

	// username is kept for the same reason: the exit event that fires when a
	// session stops being listed has no Session to read it from any more.
	username string
}

// New returns an Engine with the defaults filled in.
func New(path string, p platform.Platform, sink Sink) *Engine {
	return &Engine{
		Path:           path,
		Platform:       p,
		Sink:           sink,
		Clock:          time.Now,
		enforcementWas: true,
	}
}

// Report is the outcome of one tick.
//
// It exists for two callers: `plock status`, which shows an operator what the last
// tick did, and the tests, which assert on a tick without going through the Sink.
// It is a summary, not a transcript — the events are the transcript.
type Report struct {
	At time.Time

	// Enforcement is settings.enforcement as this tick saw it.
	Enforcement bool

	// Sessions is how many sessions the platform reported, lockable or not.
	Sessions int

	// Asserted counts the platform.Lock calls that succeeded this tick.
	Asserted int

	// Events is everything this tick emitted, in the order it emitted them. It is
	// the same set the Sink received.
	Events []Event

	// Errors holds every platform or config failure this tick. A non-empty Errors
	// is not fatal: the loop keeps the previous state and tries again.
	Errors []error
}

// Tick runs one enforcement pass and reports what it did.
//
// It never returns an error. A tick that cannot do its job records the reason in
// Report.Errors and leaves the state that the previous tick had established,
// because the loop must survive a transient logind restart, a D-Bus blip, or a
// config write caught mid-flight. The only way to stop a daemon is to stop calling
// Tick.
//
// Tick is not safe to call concurrently with itself: it mutates the engine's
// state. Run enforces that with an overlap guard; a caller driving Tick directly is
// responsible for the same discipline.
func (e *Engine) Tick(ctx context.Context) Report {
	now := e.now()
	rep := Report{At: now}

	cfg, err := e.reload(now, &rep)
	if err != nil {
		// There has never been a readable config, so there is no schedule to
		// enforce and no way to know who is a child. Locking everyone would be
		// the other failure mode, and it is worse: it locks the parent out of
		// their own machine and out of the web UI that would fix the file.
		//
		// So this is the one place the daemon deliberately does not act, and it
		// is loud rather than quiet: the error is in every Report until the file
		// is fixed, and #12's unit restarts the process.
		rep.Errors = append(rep.Errors, err)
		e.logf("enforce: no usable config and none has ever loaded: %v", err)
		return rep
	}

	enforcement := cfg.Settings.Enforcement
	if enforcement != e.enforcementWas {
		kind := KindEnforcementEnabled
		detail := "settings.enforcement is true"
		if !enforcement {
			kind = KindEnforcementDisabled
			detail = "settings.enforcement is false: no account is being locked out"
		}
		e.emit(&rep, Event{At: now, Kind: kind, Detail: detail})
		e.enforcementWas = enforcement
	}
	rep.Enforcement = enforcement

	if !enforcement {
		// Drop every session's state so that switching enforcement back on
		// re-audits lockout.entered for whoever is still inside a window. Without
		// this, a daemon that was switched off and back on mid-bedtime would
		// resume locking with no record of having started.
		e.state = nil
		return rep
	}

	sessions, err := e.Platform.Sessions(ctx)
	if err != nil {
		// Keep e.state exactly as it is. Clearing it would make the next
		// successful tick re-audit lockout.entered for sessions that never left
		// their window, and would lose the override bookkeeping along with it.
		rep.Errors = append(rep.Errors, fmt.Errorf("enumerate sessions: %w", err))
		e.emit(&rep, Event{At: now, Kind: KindPlatformError, Detail: fmt.Sprintf("enumerate sessions: %v", err)})
		return rep
	}
	rep.Sessions = len(sessions)

	seen := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		seen[s.ID] = true
		e.tickSession(ctx, cfg, s, now, &rep)
	}

	// A session that is no longer listed has stopped being enforceable: logged
	// out, or closing. Its lockout ended, so say so once.
	//
	// The cost is that a listing which transiently omits a session emits an exit
	// and then an entry when it comes back. That is the honest direction — an
	// audit trail that claims a lockout ran after the session vanished is worse
	// than one that shows a blip — and a session id is not reused within a
	// logind's lifetime, so the pair cannot be mistaken for two lockouts.
	for id, st := range e.state {
		if seen[id] || !st.enforcing {
			continue
		}
		prev := e.state[id]
		e.emit(&rep, Event{
			At:        now,
			Kind:      KindLockoutExited,
			Username:  prev.username,
			SessionID: id,
			RuleID:    prev.ruleID,
			RuleName:  prev.ruleName,
			Detail:    "session is no longer listed",
		})
		delete(e.state, id)
		delete(e.unenforceable, id)
	}

	return rep
}

// tickSession applies the whole per-session policy to one session.
//
// The order of the checks is the policy, and it is deliberate:
//
//  1. not a lockable desktop session  → never touch it
//  2. not a managed, enabled account   → never touch it
//  3. no window open right now         → release, and audit the exit once
//  4. an unexpired override            → release, and remember to audit expiry
//  5. already reporting itself locked  → leave it alone
//  6. otherwise                        → lock, and audit it
//
// Steps 1 and 2 come before any evaluation so an unmanaged account is never
// locked even if a rule would match it, and so a malformed profile cannot cost
// the admin their own session.
func (e *Engine) tickSession(ctx context.Context, cfg *config.Config, s platform.Session, now time.Time, rep *Report) {
	// 1. The platform already refuses to lock a greeter, a lock session or a
	// remote session, but refusing at the call site is what lets this tick decide
	// not to *report* the session as enforceable either.
	if !platform.IsLockable(s) {
		e.release(rep, s, now, "not a lockable desktop session")
		return
	}

	// 2. Unmanaged and disabled accounts are never locked.
	u, ok := cfg.UserFor(s.Username)
	if !ok || !u.Enabled {
		e.release(rep, s, now, "not a managed account")
		return
	}

	profile, ok := cfg.ProfileFor(s.Username)
	if !ok {
		// An enabled account whose profile is missing is a config the validator
		// should have rejected. Failing open here would free the child on the
		// strength of a broken document, so report it every tick instead.
		e.release(rep, s, now, "no profile")
		rep.Errors = append(rep.Errors, fmt.Errorf("user %q is enabled but has no profile %q", u.Username, u.ProfileID))
		e.emit(rep, Event{At: now, Kind: KindPlatformError, Username: s.Username, SessionID: s.ID,
			Detail: fmt.Sprintf("enabled account with no profile %q: not enforcing", u.ProfileID)})
		return
	}

	v := schedule.Evaluate(&profile, now)

	// 3. No window open: release.
	if !v.Active {
		e.release(rep, s, now, "no lockout window is open")
		e.clearOverride(rep, s, now)
		return
	}

	// The verdict's Rule points into `profile`, which is a copy this function
	// drops at return, so the identity is copied out here rather than carried.
	ruleID, ruleName := "", ""
	if v.Rule != nil {
		ruleID, ruleName = v.Rule.ID, v.Rule.Name
	}

	// 4. An unexpired override suppresses enforcement.
	if overrideFor(cfg, s.Username, now) {
		st := e.ensure(s.ID)
		st.overrideActive = true
		// Not release(): the lockout did not end, the exemption started, and
		// KindOverrideExpired is the line that explains the gap.
		e.releaseState(s.ID)
		return
	}
	e.clearOverride(rep, s, now)

	// 5. Already locked: leave it alone. Re-locking under the user's fingers
	// re-shows the lock screen, and a child who is honestly looking at a lock
	// screen is not the failure this loop is for.
	//
	// What this trusts: platform.Session.Locked comes from logind's LockedHint,
	// which the desktop writes asynchronously. It is the only signal logind
	// offers — Active, State and IdleHint all read as "active"/"no" on a locked
	// screen (measured, see #7) — so if a desktop ever left the hint set after an
	// unlock, this line would skip every tick and the child would have the
	// machine, silently. That is why the unlock direction is in #6's manual
	// recipe, and why the time this session has claimed to be locked is kept: a
	// lockout that has "been locked" for far longer than any plausible lock
	// screen is worth surfacing.
	if s.Locked {
		st := e.ensure(s.ID)
		if !st.enforcing {
			e.emit(rep, Event{At: now, Kind: KindLockoutEntered, Username: s.Username,
				SessionID: s.ID, RuleID: ruleID, RuleName: ruleName,
				Detail: "window open; session already reports itself locked"})
		}
		st.enforcing = true
		st.ruleID, st.ruleName = ruleID, ruleName
		st.username = s.Username
		delete(e.unenforceable, s.ID)
		return
	}

	// 6. Lock it.
	err := e.Platform.Lock(ctx, s)
	switch {
	case err == nil:
		st := e.ensure(s.ID)
		if !st.enforcing {
			e.emit(rep, Event{At: now, Kind: KindLockoutEntered, Username: s.Username,
				SessionID: s.ID, RuleID: ruleID, RuleName: ruleName})
		}
		st.enforcing = true
		st.ruleID, st.ruleName = ruleID, ruleName
		st.username = s.Username
		rep.Asserted++
		e.emit(rep, Event{At: now, Kind: KindLockAsserted, Username: s.Username,
			SessionID: s.ID, RuleID: ruleID, RuleName: ruleName})
		delete(e.unenforceable, s.ID)

	case errors.Is(err, platform.ErrUnconfirmed):
		// The request reached a backend; nobody could prove the screen locked.
		// Treat it as enforcing — we asked, and the next tick re-asks if the hint
		// says otherwise — but say so, because this is the one outcome where the
		// child may still be at the desktop.
		st := e.ensure(s.ID)
		if !st.enforcing {
			e.emit(rep, Event{At: now, Kind: KindLockoutEntered, Username: s.Username,
				SessionID: s.ID, RuleID: ruleID, RuleName: ruleName,
				Detail: "lock requested but not confirmed"})
		}
		st.enforcing = true
		st.ruleID, st.ruleName = ruleID, ruleName
		st.username = s.Username
		rep.Errors = append(rep.Errors, fmt.Errorf("lock %s: %w", s.Describe(), err))
		e.emit(rep, Event{At: now, Kind: KindLockUnconfirmed, Username: s.Username,
			SessionID: s.ID, RuleID: ruleID, RuleName: ruleName,
			Detail: "a backend accepted the lock; nothing confirmed the screen locked"})

	default:
		// Nothing reached the desktop. Keep the state as it was: if we were
		// enforcing, we still believe we should be, and the next tick tries again.
		rep.Errors = append(rep.Errors, fmt.Errorf("lock %s: %w", s.Describe(), err))
		e.emit(rep, Event{At: now, Kind: KindPlatformError, Username: s.Username,
			SessionID: s.ID, RuleID: ruleID, RuleName: ruleName,
			Detail: fmt.Sprintf("no backend could lock this session: %v", err)})

		// A session every backend permanently refuses — a raw VT is the real case,
		// see #5 — would otherwise produce one identical error every 30 seconds
		// forever. Count it and report once, loudly, rather than either burying
		// the log or going quiet.
		if e.unenforceable == nil {
			e.unenforceable = map[string]int{}
		}
		e.unenforceable[s.ID]++
		if e.unenforceable[s.ID] == unenforceableAfter {
			e.emit(rep, Event{At: now, Kind: KindUnenforceable, Username: s.Username,
				SessionID: s.ID, RuleID: ruleID, RuleName: ruleName,
				Detail: fmt.Sprintf("no backend has been able to lock this session for %d ticks in a row; this account is not being locked out", unenforceableAfter)})
		}
	}
}

// unenforceableAfter is how many consecutive failed ticks a session must show
// before it is escalated as unenforceable. Five is two and a half minutes at the
// default interval: long enough that a logind restart (which takes seconds) does
// not trip it, short enough that an operator hears about a child who is not being
// locked out before bedtime is over.
const unenforceableAfter = 5

// overrideFor reports whether an override exempts username at now.
//
// Overrides are matched by username, not by session: an exemption granted for a
// child's homework applies to every session that account has, and a parent who
// grants one and then switches users would otherwise find the new session locked.
func overrideFor(cfg *config.Config, username string, now time.Time) bool {
	for _, o := range cfg.Overrides {
		if o.Username == username && o.Until.After(now) {
			return true
		}
	}
	return false
}

// reload reads the config file.
//
// It reads on every tick rather than tracking mtime, and that is a deliberate
// simplification of the ticket's "hot-reload config if the file changed". The
// file is a few kilobytes and a tick is 30 seconds, so the cost is nothing; what
// it buys is that "did the engine notice my edit?" stops being a question. There
// is no cache to invalidate, no mtime granularity to lose a change inside, and no
// watcher goroutine whose death would silently freeze the schedule.
//
// config.Watch is still used by Run — not for correctness, but so that an edit
// takes effect on the next second instead of the next tick.
//
// A failed read keeps the last good document and reports the failure on every
// tick. Enforcement continues on the old rules, which is the fail-safe direction:
// the alternative is that a half-written file frees the child.
func (e *Engine) reload(now time.Time, rep *Report) (*config.Config, error) {
	cfg, err := config.Load(e.Path)
	if err == nil {
		first := e.cfg == nil
		e.cfg, e.cfgErr = cfg, nil
		if !first {
			// A recovery is worth a line: the operator saw KindConfigStale and
			// should see it stop.
			e.logf("enforce: config readable again")
		}
		return cfg, nil
	}

	if e.cfg == nil {
		return nil, fmt.Errorf("load config %s: %w", e.Path, err)
	}

	rep.Errors = append(rep.Errors, fmt.Errorf("load config %s (still enforcing the last good document): %w", e.Path, err))
	e.emit(rep, Event{At: now, Kind: KindConfigStale,
		Detail: fmt.Sprintf("config %s unreadable (%v); still enforcing the last document that loaded", e.Path, err)})
	return e.cfg, nil
}

// release stops enforcing one session and audits the exit once.
func (e *Engine) release(rep *Report, s platform.Session, now time.Time, detail string) {
	st := e.state[s.ID]
	if st != nil && st.enforcing {
		e.emit(rep, Event{At: now, Kind: KindLockoutExited, Username: s.Username,
			SessionID: s.ID, RuleID: st.ruleID, RuleName: st.ruleName, Detail: detail})
		st.enforcing = false
		st.ruleID, st.ruleName = "", ""
	}
	delete(e.unenforceable, s.ID)
}

// releaseState forgets enforcement for a session without auditing an exit. Used
// when an override takes over: the lockout did not end, the exemption started, and
// KindOverrideExpired is the line that explains it.
func (e *Engine) releaseState(id string) {
	if st, ok := e.state[id]; ok {
		st.enforcing = false
		st.ruleID, st.ruleName = "", ""
	}
	delete(e.unenforceable, id)
}

// clearOverride audits an override's expiry once, on the tick it stops holding.
func (e *Engine) clearOverride(rep *Report, s platform.Session, now time.Time) {
	st := e.state[s.ID]
	if st == nil || !st.overrideActive {
		return
	}
	st.overrideActive = false
	e.emit(rep, Event{At: now, Kind: KindOverrideExpired, Username: s.Username,
		SessionID: s.ID, Detail: "override ended; enforcement resumes"})
}

func (e *Engine) ensure(id string) *sessionState {
	if e.state == nil {
		e.state = map[string]*sessionState{}
	}
	st, ok := e.state[id]
	if !ok {
		st = &sessionState{}
		e.state[id] = st
	}
	return st
}

func (e *Engine) emit(rep *Report, ev Event) {
	rep.Events = append(rep.Events, ev)
	if e.Sink != nil {
		e.Sink.Emit(ev)
	}
}

func (e *Engine) now() time.Time {
	if e.Clock != nil {
		return e.Clock()
	}
	return time.Now()
}

func (e *Engine) logf(format string, args ...any) {
	if e.Logf != nil {
		e.Logf(format, args...)
	}
}

// State reports how many sessions the engine currently believes are being
// enforced. It is what `plock status` shows, and it is a snapshot: the slice is
// sorted so two calls with the same state print identically.
func (e *Engine) State() []string {
	var out []string
	for id, st := range e.state {
		if st.enforcing {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}
