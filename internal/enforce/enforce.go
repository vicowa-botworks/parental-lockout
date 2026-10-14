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
//   - A session that reports itself locked is locked anyway. logind's LockedHint is
//     writable by the session user with no polkit action behind it (#29, measured on
//     live hardware), so it can never be the reason enforcement stops.
//
// What this package does NOT do is decide what a lockout means, or how to lock a
// screen. schedule.Evaluate answers "is a window open"; platform.Lock performs the
// lock. Everything here is bookkeeping about which of those to ask and when.
package enforce

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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
	// KindLockAsserted is one successful call to platform.Lock. Expect it on EVERY
	// tick a window stays open, not only on the tick that enters it: the child may
	// have dismissed the lock screen in between, and a session that reports itself
	// already locked is locked again anyway, because that report is something the
	// child can forge (see the comment at step 5 of tickSession, and #29).
	//
	// That makes this the highest-volume kind in the log — two lines a minute per
	// enforced session — which is the price of a trail that proves enforcement was
	// continuous rather than asserting it happened. Size rotation is #28's job.
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

	// KindUnenforceable is a session that is inside a lockout but cannot be locked
	// by any backend — a raw VT, for instance, where every backend either refuses or
	// reports ErrUnconfirmed forever. Retrying into the log every 30 seconds is
	// noise; going quiet is a fail-open. So it is reported once per session per
	// lockout, and the operator decides.
	KindUnenforceable = "session.unenforceable"

	// KindScheduleDegraded is a profile the evaluator could read but not obey: a
	// timezone name that does not resolve, so every rule is being read in the
	// machine's own zone, or a rule whose window can never open. It is emitted when
	// the set of complaints CHANGES, not every tick — a parent has to fix it once,
	// and a profile can sit degraded for weeks.
	//
	// It exists because schedule.ProfileErrors would otherwise have no consumer. A
	// parent who sets "Europe/Berlin" on a UTC-hosted machine and sees the child
	// released at 04:00 instead of 02:00 has no way to know the schedule was never
	// the one they wrote, and every other line in the log will read as if it was.
	KindScheduleDegraded = "schedule.degraded"

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

	// Logf receives human-readable diagnostics that are not audit events — the ones
	// about the engine itself: a tick that fell behind, a config that went stale and
	// then recovered. New defaults it to log.Printf, so the journal gets them without
	// the caller remembering; nil is the explicit way to ask for quiet.
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
	// shown a hard lock failure. See KindUnenforceable.
	unenforceable map[string]int

	// degraded holds the last schedule-complaint signature per session, so
	// KindScheduleDegraded fires when the set changes rather than every tick. It is
	// keyed separately from state rather than stored inside it because a degraded
	// profile outside any window still has to be reported, and creating enforcement
	// state for a session that is not being enforced would be a lie.
	degraded map[string]string

	// Interval, when non-zero, fixes the tick period and ignores
	// settings.pollSeconds. Tests set it to a few milliseconds so Run can be
	// exercised without waiting thirty seconds for a ticker that does not need to
	// exist to prove the policy.
	Interval time.Duration

	// WatchInterval is how often config.Watch re-stats the config file. Zero means
	// one second: an edit lands on the next tick anyway, so this only buys latency.
	WatchInterval time.Duration

	// mu is the overlap guard. One tick at a time is a property of the engine, not
	// of Run, because the caller that would otherwise race the loop is #10's
	// "apply now" button — which cannot know that a loop is running somewhere.
	// Tick runs under it; TryTick takes it with TryLock.
	mu sync.Mutex

	// skipped counts the ticks Run refused because one was already running. It is
	// atomic because Skipped is read from the status path while the loop ticks.
	skipped atomic.Int64

	// pollSeconds mirrors settings.pollSeconds out of the last successful reload so
	// Run can retime itself without reading e.cfg off the loop goroutine. e.cfg
	// belongs to the tick, and reading it from both is a race the race detector is
	// right to complain about.
	pollSeconds atomic.Int64
}

// sessionState is what one tick learns about one session so the next tick can tell
// whether anything changed.
type sessionState struct {
	// enforcing is true when the previous tick considered this session locked out
	// and subject to enforcement. It is a statement about the policy, not about the
	// screen: the tick that sets it also called Lock, whatever the session claimed
	// about being locked already (#29).
	enforcing bool

	// overrideActive is true when an override suppressed enforcement last tick, so
	// its expiry can be audited once.
	overrideActive bool

	// unenforceableReported is true once KindUnenforceable has fired for this
	// lockout. It is a flag rather than a count because the two ways to reach it
	// differ: a session every backend refuses escalates after unenforceableAfter
	// ticks, and a session that can never confirm escalates on the first, so a
	// shared counter would have to be reset differently by each path.
	unenforceableReported bool

	// profileErrs is the schedule complaint set this session was last reported with,
	// so KindScheduleDegraded fires when the set changes rather than every tick. The
	// empty string means nothing was wrong last time.
	profileErrs string

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
		Path:     path,
		Platform: p,
		Sink:     sink,
		Clock:    time.Now,
		// Diagnostics go to the standard logger unless the caller replaces it.
		// Without this, an engine built here and left unconfigured drops the three
		// loudest things it has to say — a tick that fell behind, a config that went
		// stale, a schedule that stopped being readable — and silence is this
		// daemon's characteristic failure mode rather than its crash. The journal
		// capture in #12 gets them for free; a caller with its own logger assigns
		// Logf, and one that wants quiet assigns nil.
		Logf:           log.Printf,
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
// state. Run and TryTick both take the engine's guard first, and a caller driving
// Tick directly is responsible for the same discipline.
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
		// A shutdown between sessions ends the pass rather than continuing through
		// the list. Every remaining session would be locked with a dead context,
		// and the platform reports that as an unconfirmed lock — so a routine
		// `systemctl stop` would write one alarming audit line per child on the
		// machine, on its way out.
		if ctx.Err() != nil {
			return rep
		}
		seen[s.ID] = true
		e.tickSession(ctx, cfg, s, now, &rep)
	}

	// A session that is no longer listed has stopped being enforceable: logged
	// out, or closing. Its lockout ended, so say so once.
	//
	// It is then forgotten completely, whether or not it was enforcing. Keeping the
	// bookkeeping of sessions that have come and gone would grow the map for the
	// lifetime of the daemon — logind mints a new id for every login and never
	// reuses one — and a stale entry buys nothing, because a session id is not
	// reused within a logind's lifetime.
	//
	// The cost is that a listing which transiently omits a session emits an exit
	// and then an entry when it comes back. That is the honest direction — an
	// audit trail that claims a lockout ran after the session vanished is worse
	// than one that shows a blip.
	for id, st := range e.state {
		if seen[id] {
			continue
		}
		if st.enforcing {
			e.emit(&rep, Event{
				At:        now,
				Kind:      KindLockoutExited,
				Username:  st.username,
				SessionID: id,
				RuleID:    st.ruleID,
				RuleName:  st.ruleName,
				Detail:    "session is no longer listed",
			})
		}
		delete(e.state, id)
		delete(e.unenforceable, id)
		delete(e.degraded, id)
	}

	return rep
}

// Run drives Tick from a ticker until ctx is cancelled, and returns ctx.Err().
//
// Three things belong here rather than in Tick:
//
//   - The period. settings.pollSeconds, taken from the config the tick just read,
//     so editing the interval takes effect without a restart.
//   - The overlap guard. A tick that outlives its period is skipped and counted,
//     never stacked: two ticks locking the same session at once would double every
//     audit line and race the engine's own state.
//   - Latency. config.Watch pokes the loop when the file changes, so an edit lands
//     within a second instead of within a tick. It is not load-bearing — Tick
//     re-reads the file regardless — so a watcher that dies costs latency and
//     nothing else, which is why nothing here watches the watcher.
//
// The first tick runs immediately, before any waiting: a daemon restarted at 21:03
// during a curfew must lock on start, not hand back 30 seconds first.
//
// Ticks run on their own goroutine so that a platform call which ignores its
// context cannot park the loop and make it deaf to cancellation. That is also what
// makes the overlap guard necessary rather than theoretical.
func (e *Engine) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var wg sync.WaitGroup
	defer func() {
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(shutdownGrace):
			// The tick's context is already cancelled, so a tick still running now
			// is stuck somewhere that does not answer to cancellation. Waiting
			// forever would let a hung D-Bus turn every restart into a SIGKILL, so
			// this gives up and says so — the goroutine is left to finish, and the
			// process exiting takes it with it.
			e.logf("enforce: a tick was still running %v after shutdown; not waiting for it", shutdownGrace)
		}
	}()

	watchDone := make(chan struct{})
	defer close(watchDone)
	// A negative WatchInterval opts out of watching entirely. config.Watch spawns a
	// goroutine that stats the path forever, and a test that drives Run with its own
	// short Interval does not want a file poller in the race report.
	var changes <-chan struct{}
	if e.WatchInterval >= 0 {
		changes = config.Watch(e.Path, e.watchInterval(), watchDone)
	}

	e.startTick(ctx, &wg)

	for {
		timer := time.NewTimer(e.interval())
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			e.startTick(ctx, &wg)
		case <-changes:
			e.startTick(ctx, &wg)
		}
	}
}

// shutdownGrace bounds how long Run waits, on the way out, for a tick still in
// flight. Five seconds is well past a healthy tick — a handful of loginctl calls
// against a local daemon — and well short of systemd's default stop timeout, so the
// daemon exits on its own and the journal keeps the explanation.
const shutdownGrace = 5 * time.Second

// startTick runs one tick unless one is already running, in which case it records
// the skip. The guard is taken here rather than inside the goroutine because a tick
// that has been refused must not even begin: two ticks reading and writing
// e.state at once is the failure, not two ticks finishing out of order.
func (e *Engine) startTick(ctx context.Context, wg *sync.WaitGroup) {
	if !e.mu.TryLock() {
		e.noteSkipped()
		return
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer e.mu.Unlock()
		e.Tick(ctx)
	}()
}

// noteSkipped records a refused tick as both a counter and an event.
//
// The counter alone would be enough for a status page and useless for everything
// else: a daemon that quietly falls behind every tick leaves no trace in the audit
// log, and "enforcement stopped being timely three hours ago" is precisely the fact
// an operator finds out by other means.
func (e *Engine) noteSkipped() {
	n := e.skipped.Add(1)
	e.logf("enforce: tick skipped, previous one still running (total %d)", n)
	if e.Sink != nil {
		e.Sink.Emit(Event{At: e.now(), Kind: KindTickSkipped,
			Detail: fmt.Sprintf("a tick was still running when this one was due; %d skipped so far. Enforcement is taking longer than the tick interval, so locks are landing late.", n)})
	}
}

// TryTick runs one guarded enforcement pass, reporting whether it ran.
//
// This is the entry point for a caller outside the loop — #10's "apply now" — that
// wants the engine's own overlap rule instead of inventing a second one. It never
// waits: a tick is in flight, the caller is told no, and that in-flight tick is
// already doing the same work with the same config.
//
// A refused TryTick is not counted in Skipped, which means specifically "the timer
// fired and enforcement was behind".
func (e *Engine) TryTick(ctx context.Context) (Report, bool) {
	if !e.mu.TryLock() {
		return Report{}, false
	}
	defer e.mu.Unlock()
	return e.Tick(ctx), true
}

// Skipped is how many timer ticks have been refused because one was still running.
// Any number that keeps growing means the tick interval is shorter than a tick.
func (e *Engine) Skipped() int64 { return e.skipped.Load() }

// interval is the current tick period: the test override if set, else whatever the
// last successful reload said, else the documented default. It reads the published
// number rather than e.cfg because e.cfg belongs to the tick goroutine.
func (e *Engine) interval() time.Duration {
	if e.Interval > 0 {
		return e.Interval
	}
	if s := e.pollSeconds.Load(); s > 0 {
		return time.Duration(s) * time.Second
	}
	return config.DefaultPollSeconds * time.Second
}

func (e *Engine) watchInterval() time.Duration {
	if e.WatchInterval > 0 {
		return e.WatchInterval
	}
	return time.Second
}

// tickSession applies the whole per-session policy to one session.
//
// The order of the checks is the policy, and it is deliberate:
//
//  1. not a lockable desktop session  → never touch it
//  2. not a managed, enabled account   → never touch it
//  3. no window open right now         → release, and audit the exit once
//  4. an unexpired override            → release, and remember to audit expiry
//  5. otherwise                        → lock, and audit it, on EVERY tick
//
// There is no "the session says it is already locked" step between 4 and 5, and
// there must never be one again; see the comment where the lock is issued.
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

	// A schedule the evaluator could read but not obey is said out loud before
	// anything is decided from it, because the verdict below is computed from rules
	// that may not be the ones the parent wrote.
	e.reportDegraded(rep, s, &profile, now)

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

	// 5. Lock it. Every tick, while the window is open, whatever the session says
	// about itself.
	//
	// There used to be a step here that skipped the lock when s.Locked was true, on
	// the reasoning that re-locking under someone's fingers is rude and a child
	// honestly looking at a lock screen is not the failure this loop exists for.
	// #29 measured why that reasoning is fatal. platform.Session.Locked is logind's
	// LockedHint, and SetLockedHint is authorized by uid alone — there is no polkit
	// action for it among login1's 37, because desktops are supposed to be the only
	// thing writing it. So this runs from the child's own account, no sudo, no
	// prompt:
	//
	//	gdbus call --system --dest org.freedesktop.login1 \
	//	  --object-path /org/freedesktop/login1/session/_33 \
	//	  --method org.freedesktop.login1.Session.SetLockedHint true
	//
	// One autostart file with that line and the old engine believed the session was
	// locked on every tick forever: no lock, no error, and an audit trail reading
	// "window open; session already reports itself locked" — accurate as far as the
	// daemon knew, false in fact. A signal the controlled user can write must never
	// decide when enforcement may stop.
	//
	// So the hint is now wording, never a gate. The cost of being wrong in this
	// direction is that ScreenSaver.Lock is called again on a screen that is already
	// locked, which #6 measured to be idempotent: a repaint. The cost of being wrong
	// in the other direction is the product.
	hinted := s.Locked
	err := e.Platform.Lock(ctx, s)
	if err != nil && ctx.Err() != nil {
		// The machine is going down, and this is not a lock outcome.
		//
		// Chain.Lock cannot tell "the desktop would not confirm" from "we were
		// cancelled while asking": a cancelled settle window answers "not locked",
		// which arrives here as ErrUnconfirmed — the one verdict whose rule is
		// "keep asking every tick". Applying that rule during shutdown makes the
		// daemon's last act an audit line claiming the child may still be at the
		// desktop, on a machine that is powering off.
		//
		// Unconfirmed is the signal an operator is meant to act on. Emitting a
		// false one on every restart is how it stops being believed, so the tick
		// says nothing and leaves the recorded state for the next process to
		// re-assert.
		return
	}
	switch {
	case err == nil:
		e.markEnforcing(rep, s, now, ruleID, ruleName, "")
		rep.Asserted++
		e.emit(rep, Event{At: now, Kind: KindLockAsserted, Username: s.Username,
			SessionID: s.ID, RuleID: ruleID, RuleName: ruleName,
			Detail: assertedDetail(hinted)})

	case errors.Is(err, platform.ErrUnconfirmed):
		// The request reached a backend; nobody could prove the screen locked.
		// Treat it as enforcing — we asked, and the next tick asks again — but say
		// so, because this is the one outcome where the child may still be at the
		// desktop.
		st := e.markEnforcing(rep, s, now, ruleID, ruleName, "lock requested but not confirmed")
		rep.Errors = append(rep.Errors, fmt.Errorf("lock %s: %w", s.Describe(), err))

		// A raw virtual terminal is a different statement from "unconfirmed
		// today". Nothing on a VT ever calls SetLockedHint and loginctl still
		// exits 0, so this returns ErrUnconfirmed on tick 1 and on tick 10,000
		// (measured, #5). platform.IsLockable keeps such a session in the listing
		// on purpose — refusing it would make Sessions() report nobody for that
		// child and the loop would go quiet while the child kept typing — so the
		// verdict that this account is not being locked out has to be reached here,
		// in the loop, rather than in the filter that decides what to look at.
		//
		// A wayland or x11 session is NOT in this category. Its desktop may have
		// locked the screen without ever writing the hint — the screensaver
		// interface exposes no Locked property at all (measured, #6) — so
		// persistent unconfirmed on a real desktop is a question to keep asking,
		// not an answer.
		if canNeverConfirm(s) {
			if !st.unenforceableReported {
				st.unenforceableReported = true
				e.emit(rep, Event{At: now, Kind: KindUnenforceable, Username: s.Username,
					SessionID: s.ID, RuleID: ruleID, RuleName: ruleName,
					Detail: "this session type has no desktop to lock, so no backend will ever be able to confirm a lock on it; this account is not being locked out"})
			}
		} else {
			e.emit(rep, Event{At: now, Kind: KindLockUnconfirmed, Username: s.Username,
				SessionID: s.ID, RuleID: ruleID, RuleName: ruleName,
				Detail: unconfirmedDetail(hinted)})
		}

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
		if e.unenforceable[s.ID] >= unenforceableAfter {
			if st := e.ensure(s.ID); !st.unenforceableReported {
				st.unenforceableReported = true
				e.emit(rep, Event{At: now, Kind: KindUnenforceable, Username: s.Username,
					SessionID: s.ID, RuleID: ruleID, RuleName: ruleName,
					Detail: fmt.Sprintf("no backend has been able to lock this session for %d ticks in a row; this account is not being locked out", unenforceableAfter)})
			}
		}
	}
}

// assertedDetail words a lock.asserted line. The hint is named when it was set so
// the trail says out loud that the engine re-locked a session that claimed it did
// not need it — the line a parent reads to learn someone was writing LockedHint.
func assertedDetail(hinted bool) string {
	if hinted {
		return "re-asserted although the session reported itself locked; that report is not evidence the screen is locked"
	}
	return ""
}

// unconfirmedDetail words a lock.unconfirmed line.
func unconfirmedDetail(hinted bool) string {
	if hinted {
		return "a backend accepted the lock and the session already claimed to be locked; neither proves the screen is locked"
	}
	return "a backend accepted the lock; nothing confirmed the screen locked"
}

// canNeverConfirm reports whether a session is one no backend will ever be able to
// confirm, as opposed to merely unconfirmed on this tick. See the call site.
func canNeverConfirm(s platform.Session) bool { return s.Type == "tty" }

// markEnforcing records that this session is inside an enforced lockout and returns
// its state, emitting lockout.entered on the tick that starts one.
//
// Every branch that asked for a lock goes through here — confirmed and unconfirmed —
// so "we believe this account should be locked" cannot be set by one outcome and
// forgotten by another.
func (e *Engine) markEnforcing(rep *Report, s platform.Session, now time.Time, ruleID, ruleName, enterDetail string) *sessionState {
	st := e.ensure(s.ID)
	if !st.enforcing {
		e.emit(rep, Event{At: now, Kind: KindLockoutEntered, Username: s.Username,
			SessionID: s.ID, RuleID: ruleID, RuleName: ruleName, Detail: enterDetail})
	}
	st.enforcing = true
	st.ruleID, st.ruleName = ruleID, ruleName
	st.username = s.Username
	delete(e.unenforceable, s.ID)
	return st
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
		// Published for Run, which must not read e.cfg from another goroutine. A
		// parent who changes pollSeconds sees the period move on the next tick
		// without a restart, which is the same reason the file is re-read at all.
		e.pollSeconds.Store(int64(cfg.Settings.PollSeconds))
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
	if st := e.state[s.ID]; st != nil {
		if st.enforcing {
			e.emit(rep, Event{At: now, Kind: KindLockoutExited, Username: s.Username,
				SessionID: s.ID, RuleID: st.ruleID, RuleName: st.ruleName, Detail: detail})
		}
		st.enforcing = false
		st.ruleID, st.ruleName = "", ""
		// Per-lockout bookkeeping starts again the next time this session is inside
		// a window. Leaving unenforceableReported set would mean a child who was
		// unenforceable last night is never escalated again, because the flag says
		// "already told you" for a lockout that has since ended.
		st.unenforceableReported = false
		if !st.overrideActive {
			delete(e.state, s.ID)
		}
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
		st.unenforceableReported = false
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

// reportDegraded surfaces a profile the evaluator had to compromise on.
//
// It reports when the complaint SET changes, in either direction: a parent who
// fixes the timezone should see it stop, and a profile that silently gained a rule
// that can never fire should be visible the tick it was saved. Reporting every tick
// would bury the lock lines; reporting only the first time would leave a parent who
// missed it with no second chance while the daemon enforced a schedule nobody chose.
func (e *Engine) reportDegraded(rep *Report, s platform.Session, p *config.Profile, now time.Time) {
	errs := schedule.ProfileErrors(p)
	msgs := make([]string, 0, len(errs))
	for _, err := range errs {
		msgs = append(msgs, err.Error())
	}
	sig := strings.Join(msgs, "\n")

	if e.degraded[s.ID] == sig {
		return
	}
	if e.degraded == nil {
		e.degraded = map[string]string{}
	}
	e.degraded[s.ID] = sig

	if sig == "" {
		e.logf("enforce: %s: schedule readable again", s.Username)
		return
	}
	e.emit(rep, Event{At: now, Kind: KindScheduleDegraded, Username: s.Username,
		SessionID: s.ID, Detail: sig})
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

// State reports which sessions are inside a lockout the engine is enforcing. It is
// what `plock status` and the dashboard show, and it is a snapshot: the slice is
// sorted so two calls with the same state print identically.
//
// Read it as "this account is in a lockout we are acting on", not "this screen is
// locked now". Those differ in both directions, and the difference is deliberate: a
// session whose very first lock failed is absent, because nothing has been enforced,
// while a session that was locked an hour ago and whose platform has since gone quiet
// stays listed, because the window is still open and the failure is reported as an
// error rather than as a release. Anything that wants the second question has to ask
// the platform, and #29 says what that answer is worth on its own.
//
// It takes the same guard a tick runs under, so a status read cannot observe half of
// a tick's bookkeeping. That means it waits for a tick in flight, which is the right
// trade for a screen whose whole job is to be truthful about the last tick.
func (e *Engine) State() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []string
	for id, st := range e.state {
		if st.enforcing {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}
