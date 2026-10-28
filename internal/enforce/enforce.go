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
	//
	// Cadence: once per unconfirmed episode, not per tick. The engine re-asserts the
	// lock every poll while a window stays open, so a per-tick line meant a machine that
	// can never confirm its lock — a GNOME-only configuration, per #35 — wrote the same
	// audit line every 30 seconds for as long as the curfew ran, and a trail that floods
	// is a trail nobody reads. It is emitted on the transition into unconfirmed and
	// cleared on the first confirmed tick, so a lock that flaps reports each episode.
	// Report.Errors and platform.Lock are unchanged: the live verdict is still per-tick,
	// and the daemon still re-arms every tick. Only the audit line deduplicates.
	KindLockUnconfirmed = "lock.unconfirmed"

	// KindLockReleased is an observed transition: a session that reported itself
	// LOCKED on one poll reports itself unlocked on the next, inside a window whose
	// lock had been confirmed. The screen came off between two ticks.
	//
	// It exists because the containment for #177 is invisible without it. The session
	// bus exports org.gnome.ScreenSaver.SetActive, served by a process running as the
	// controlled user's own uid, and #177 measured SetActive(false) dismissing a
	// password-gated lock shield with no password at all (the wrong password is
	// rejected; the right one unlocks; SetActive(false) needs neither). The loop
	// already absorbs that, because it re-asserts every tick whatever the session says
	// about itself (#29), so the child gets at most one poll of the desktop. But an
	// absorbed attack and a quiet night then write the same line — lock.asserted,
	// every 30 seconds, forever. The parent's question is not "is it locked now", it is
	// "is someone fighting it", and only a count answers that.
	//
	// Cadence: once per observed transition. This is NOT the flood KindLockUnconfirmed
	// deduplicates. That one fires on every poll of a perfectly healthy GNOME machine,
	// because the machine can never corroborate its own lock; this one needs someone
	// to actually take the screen off, so the worst case is one line per poll and only
	// while someone is genuinely fighting the loop — which is the case an operator most
	// wants to see, and the running count in the detail says how long it has been
	// going. A screen left unlocked is reported once, not every poll: the transition is
	// consumed, and the next line needs the hint to go up and come down again.
	//
	// It never gates anything. The release is inferred from LockedHint, which the
	// child owns (#29), so it is evidence of tampering and nothing else: the tick
	// calls platform.Lock whether or not this fires. Refusing to lock on the strength
	// of a release would be the exact fail-open #29 killed.
	KindLockReleased = "lock.released"

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

	// cfgErr is why cfg is being reused: the last read failed, so enforcement continues on
	// the previous document. The per-tick KindConfigStale is built from the load error on the
	// tick that sees it, not from this field; what the field carries is the fact that a stale
	// period is open, which is what makes a later good read a recovery worth logging.
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

	// Self says whether this process is running inside someone's per-user slice, and
	// whose. See platform.SelfLocation and platform.UserSliceUID for what the signal
	// is and why the cgroup is the only one of the candidates a controlled user cannot
	// write.
	//
	// The caller resolves it; New deliberately does not. Reading /proc inside the
	// constructor would make every engine on this machine report "inside uid 1000's
	// slice", and every enforcement test would then depend on which kind of host runs
	// it -- green on a desktop, silently different on a headless runner, which is the
	// class of defect this repo has already been burned by twice.
	//
	// The zero value is the supported deployment and the safe one: a systemd system
	// unit lives under system.slice, so the rule never applies. A fixture that leaves
	// this unset therefore gets exactly what a production daemon gets.
	Self platform.SelfLocation

	// selfSkipped holds the session ids the daemon has already announced it will not
	// lock, because they are its own.
	//
	// It is keyed separately from state rather than stored inside it for a specific
	// reason: both release paths reset the per-lockout "already told you" flag, and
	// this condition does not end with the lockout. Storing the flag in state would
	// mean one audit line per tick for a deployment mistake that will never fix itself
	// -- the exact volume #35 exists to prevent.
	selfSkipped map[string]bool

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

	// unconfirmedReported is true once KindLockUnconfirmed has fired for the current
	// run of unconfirmed ticks, and is cleared by any tick that does confirm.
	//
	// This is the transition rule #35 asks for. A GNOME-only configuration confirms on
	// the tick the hint moves and reports ErrUnconfirmed on every tick after, because
	// GNOME draws its lock screen inside gnome-shell and nothing corroborates a lock
	// once it has taken. Before this, a correctly locked GNOME screen wrote an
	// operator-actionable "the child may still be at the desktop" line every 30 seconds
	// forever -- and a signal that fires on a healthy machine every 30 seconds is a
	// signal the operator learns to skip, which is how the one case that IS actionable
	// gets missed.
	//
	// Enforcement is untouched: the loop still calls platform.Lock every tick, and
	// Report.Errors still carries the fault every tick, so the live verdict is unchanged.
	// What dedupes is the audit file.
	unconfirmedReported bool

	// confirmedAndSeenLocked is true once this lockout has both CONFIRMED a lock and
	// seen the session report itself locked. It is what makes a release reportable:
	// the release rule needs an observed transition from locked to unlocked, not the
	// absence of a locked report.
	//
	// The distinction is the whole difference between a tamper alarm and a false one.
	// A confirmation does not require the hint — the locker-identity corroborator
	// proves the lock without it — so on a desktop that never writes LockedHint a
	// level test ("it says unlocked") would fire on every poll of a perfectly healthy
	// machine, which is the alarm-on-a-quiet-box failure #35 exists to prevent. A
	// session that never said locked has no evidence of a release, and says nothing.
	confirmedAndSeenLocked bool

	// releases counts the KindLockReleased lines this lockout has produced, so the
	// detail can say how many times the screen came back unlocked and not merely that
	// it did once. Cleared with the rest of the lockout's bookkeeping.
	releases int

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
		// A partial read is not an empty one. The rows that came back are children who
		// are inside a window right now, and returning here is what made one flaky
		// show-session cost the whole box a poll in which nobody was locked — with a
		// single audit row, and a dashboard that still looked fine (#168). The state
		// decision above stays as it is either way; this is a different decision, and
		// the tick is the product.
		if len(sessions) == 0 {
			return rep
		}
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
	//
	// The sweep is only sound on a complete census. After a partial read an id missing
	// from `seen` means either "logged out" or "the row we could not read" (#168), and
	// guessing wrong writes a lockout.exited for a child still sitting at a locked
	// screen while dropping the state that carries their override bookkeeping. Skipping
	// it once costs a deferred blip; running it on a partial census invents an exit.
	if err != nil {
		return rep
	}

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

	// Saying that enforcement started, and at what period, is the difference between
	// a daemon that is enforcing and a daemon that merely came up. Before this there
	// was no line at all: Run logged only its exceptions, so a build wired without
	// the loop was indistinguishable in the journal from one running quietly and
	// correctly. The period belongs in the sentence because an operator watching a
	// screen that will not lock needs to know whether they are waiting 5s or 5m.
	//
	// Which is exactly why it cannot simply read e.interval() here. Nothing has
	// published pollSeconds yet -- the first reload happens inside the first tick -- so
	// interval() answers the compiled default and a config that says 5 announces 30s.
	// Measured on the dev loop: `pollSeconds: 5` printed "every 30s" for the whole run.
	// The number is therefore read from the same file the tick will read, here on the
	// goroutine that owns the log line, before the tick starts and while nothing else
	// can race with it.
	period := e.startupPeriod()
	if period < 0 {
		// Unreadable, so the honest sentence is that the period is not known yet. The
		// default would be a guess presented as a measurement, and the first tick is
		// about to report the same load failure anyway.
		e.logf("enforce: running, period not yet known (config unreadable), config %s", e.Path)
	} else {
		e.logf("enforce: running, every %v, config %s", period, e.Path)
	}

	// The announced period has to be the period actually run at, and it was not: the
	// first timer was armed from interval() before any reload had published
	// pollSeconds, so a config asking for 5s sat 30s before its second tick. Measured on
	// the dev loop -- two lock calls 30s apart on a machine whose config said 5 and whose
	// startup line, after the previous fix, correctly said 5s. Announcing one number and
	// running another is worse than announcing nothing, because the operator now has a
	// figure to trust and it is still wrong.
	//
	// Publishing here is safe: pollSeconds is an atomic whose only other writer is
	// reload, which will store the same number from the same file on the first tick. An
	// unreadable config leaves it alone, so interval() keeps the documented default and
	// the line above already said the period is not known.
	if period > 0 {
		e.pollSeconds.Store(int64(period / time.Second))
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

// startupPeriod reads the tick period straight from the config for the startup line.
//
// It returns a negative duration when the file cannot be read, which the caller words as
// "not yet known" rather than falling back to the default: the default is what interval()
// already reports, and the whole defect this exists to fix was a default being printed as
// though it were the configured value.
//
// Safe to call from Run before startTick because nothing else has the config yet. Once the
// tick is running, pollSeconds is the only channel and this must not be called again.
func (e *Engine) startupPeriod() time.Duration {
	cfg, err := config.Load(e.Path)
	if err != nil {
		return -1
	}
	// No zero check here, and none is missing: config's decode step replaces an absent
	// pollSeconds with DefaultPollSeconds (config.go, "if s.PollSeconds == 0"), so a
	// loaded document never carries 0. The guard this replaces was written on the
	// assumption that Load hands back the raw JSON, and a mutation battery proved it
	// unreachable. interval() keeps its own > 0 test for the opposite reason -- the
	// published atomic really is 0 before the first reload.
	return time.Duration(cfg.Settings.PollSeconds) * time.Second
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
//  5. the daemon's own login session   → refuse, and say why once
//  6. otherwise                        → lock, and audit it, on EVERY tick
//
// Step 5 is the only step that declines to lock for a reason other than the policy
// saying not to, and it is inert in the supported deployment; see the comment where it
// is issued.
//
// There is no "the session says it is already locked" step between 4 and 6, and
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

	// 5. A session inside the daemon's own user slice is one it must not lock.
	//
	// The hazard is not the child's screen. It is the operator's: a daemon that locks
	// the session it is running in cannot be stopped, because stopping it needs a
	// keyboard and the keyboard is behind the lock it just put up. The override path is
	// no help either — it needs the web UI or the CLI, and both need the admin to be at
	// the machine that is now locked.
	//
	// This is a deliberate, stated fail-open, and it is worth being exact about what it
	// opens. It fires only when the daemon is running inside a per-user slice, which a
	// systemd system unit never is; in the supported deployment the rule is inert and
	// enforcement is unchanged for every account including the admin's own. Where it
	// does fire is a daemon started from a desktop — `make run-dev`, or someone
	// running `plock serve` by hand to see what it does — and there the alternative is
	// not "the child gets locked", it is "the daemon locked its own operator out and
	// must be rebooted to be stopped".
	//
	// Why the uid comparison is enough here, when #67 is right that uid alone does not
	// decide the policy question. This is not asking "is this the child's session"; it
	// is asking "would locking this session lock the screen I am sitting behind". When
	// an admin and a child genuinely share one account they share a uid, and no signal
	// available to a root daemon distinguishes the two sessions either — so the rule
	// declines, and says so in the audit trail rather than going quiet.
	//
	// The signal is the kernel's own record of this process's cgroup, which is what
	// makes it pass the test #29 applies to LockedHint: a controlled user cannot place
	// a root service in their slice, and cannot move one that is already running.
	// Process ancestry — the comparison Session.Leader's comment proposes — was
	// measured on a live Plasma session and matches only the user@.service "manager"
	// row, because a systemd user manager reparents GUI processes onto itself. A guard
	// built on it would never fire on the session actually on the screen, and a guard
	// that cannot fire reads as a guard that passed.
	if e.Self.Declines(s.UID) {
		// release(), not releaseState(), on purpose — but be exact about what that
		// buys, because it is defence and not a live path. No state can exist here in
		// the supported flow: Self is resolved once before Run and never changes, so a
		// session that reaches this line has never been locked by this process and has
		// no lockout to exit. release() is the call that keeps state and audit
		// consistent if that ever stops being true, and releaseState() would delete the
		// state while leaving its "entered" line unanswered.
		e.release(rep, s, now, "this is the session the daemon is running in")
		if e.selfSkipped == nil {
			e.selfSkipped = map[string]bool{}
		}
		if !e.selfSkipped[s.ID] {
			e.selfSkipped[s.ID] = true
			e.emit(rep, Event{At: now, Kind: KindUnenforceable, Username: s.Username,
				SessionID: s.ID, RuleID: ruleID, RuleName: ruleName,
				Detail: fmt.Sprintf(
					"this session belongs to uid %d, whose login session the daemon itself is "+
						"running in, so locking it would lock the screen the daemon is stopped "+
						"from: this account is not being locked out. Run plock as a system "+
						"service (deploy/plock.service) to enforce this account", e.Self.UID)})
		}
		return
	}

	// 6. Lock it. Every tick, while the window is open, whatever the session says
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

	// An observed transition: this session reported itself LOCKED on the previous
	// poll, having had a lock confirmed, and it now reports itself unlocked. Something
	// between the two polls took the screen off. On GNOME one SetActive(false) call
	// from the child's own session bus does exactly that (#177), and SetLockedHint is
	// writable the same way (#29), so the expected author of this line is the child.
	//
	// Reported, then locked again. The re-assert below is the containment and it is
	// not conditional on anything here: this decides what the trail says, never
	// whether the screen goes back on.
	rel := e.ensure(s.ID)
	if rel.confirmedAndSeenLocked && !hinted {
		rel.releases++
		e.emit(rep, Event{At: now, Kind: KindLockReleased, Username: s.Username,
			SessionID: s.ID, RuleID: ruleID, RuleName: ruleName,
			Detail: releasedDetail(rel.releases)})
		// The transition is consumed. Until a confirmed poll sees the screen locked
		// again there is no NEXT transition to report, so a child who unlocks once and
		// walks away writes one line, and one who unlocks every poll writes one line
		// per poll.
		rel.confirmedAndSeenLocked = false
	}

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

		// A confirmed tick ends any run of unconfirmed ticks, so the next one is a
		// transition and gets its line again. Without this the alarm would fire once
		// per process rather than once per episode, and an episode that starts hours
		// after a healthy stretch is exactly the one worth reading.
		//
		// confirmedAndSeenLocked is the other half of the same idea: it records that
		// this lock is both confirmed and VISIBLE in the hint, which is what lets the
		// next tick notice the hint going away. The confirmation alone is not enough
		// and the hint alone is not ours to believe.
		st := e.ensure(s.ID)
		st.unconfirmedReported = false
		if hinted {
			st.confirmedAndSeenLocked = true
		}

	case errors.Is(err, platform.ErrUnconfirmed):
		// The request reached a backend; nobody could prove the screen locked.
		// Treat it as enforcing — we asked, and the next tick asks again — but say
		// so, because this is the one outcome where the child may still be at the
		// desktop.
		st := e.markEnforcing(rep, s, now, ruleID, ruleName, "lock requested but not confirmed")
		// confirmedAndSeenLocked is deliberately NOT cleared here. "Could not
		// corroborate this poll" is not "the screen came off", and a session that has
		// already shown us the hint go up is still worth believing when the hint later
		// goes down — especially on a GNOME-only configuration, which corroborates on
		// the tick the hint moves and almost never again afterwards (#35).
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
		} else if !st.unconfirmedReported {
			// First unconfirmed tick of this episode. Later ones are silent in the
			// audit file and loud in Report.Errors; see the field's note on #35.
			st.unconfirmedReported = true
			e.emit(rep, Event{At: now, Kind: KindLockUnconfirmed, Username: s.Username,
				SessionID: s.ID, RuleID: ruleID, RuleName: ruleName,
				Detail: unconfirmedDetail(hinted)})
		}

	default:
		// Nothing reached the desktop. Keep the state as it was: if we were
		// enforcing, we still believe we should be, and the next tick tries again.
		//
		// confirmedAndSeenLocked is deliberately left alone for the same reason. A
		// failed call is the absence of information, not evidence the screen came off:
		// the lock we confirmed last tick is very likely still up. Clearing it here
		// would hide a real dismissal behind a gdbus blip.
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

// releasedDetail words a lock.released line. The count is the payload: a parent
// deciding whether to act needs to know this is the third time tonight or the
// thirtieth, and the audit file is the only place that knows.
//
// Both readings are named because the trail cannot tell them apart: the signal this
// is inferred from is one the session user can write, so "the screen came off" and
// "someone wrote the hint" are the same line here. What they have in common is that
// neither happens on a machine nobody is touching.
func releasedDetail(n int) string {
	if n == 1 {
		return "the session reported the screen locked on one poll and unlocked on the next, after a confirmed lock; locked again now. Only the session user can write that signal, so this is either the screen coming off or someone writing the hint, and neither happens on a machine nobody is touching"
	}
	return fmt.Sprintf("the screen has come back unlocked %d times since this lockout began, each time after a confirmed lock; locked again now. Only the session user can write the signal this is inferred from, so this is either someone dismissing the lock or someone writing the hint, and neither happens on a machine nobody is touching", n)
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
		// The line closes a stale period, so it belongs to the transition out of one, not to
		// every successful read. Keying it off "not the first load" printed it on every tick
		// for the life of the process: the live Plasma run showed it in the daemon log on a
		// machine whose config had been readable since before the daemon started, with no
		// KindConfigStale to answer. A diagnostic that lies on a healthy machine is worse than
		// no diagnostic, because the operator learns to skim past it.
		recovered := e.cfgErr != nil
		e.cfg, e.cfgErr = cfg, nil
		// Published for Run, which must not read e.cfg from another goroutine. A
		// parent who changes pollSeconds sees the period move on the next tick
		// without a restart, which is the same reason the file is re-read at all.
		e.pollSeconds.Store(int64(cfg.Settings.PollSeconds))
		if recovered {
			e.logf("enforce: config readable again")
		}
		return cfg, nil
	}

	if e.cfg == nil {
		return nil, fmt.Errorf("load config %s: %w", e.Path, err)
	}

	// What makes the next good read a recovery rather than an ordinary tick. Without this
	// assignment the field is written only as nil, so the transition above can never fire and
	// the recovery line the field exists to gate would be lost with the bug that printed it.
	e.cfgErr = err
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
		// The release bookkeeping is per-lockout too. A child who dismissed the lock
		// all of last night must not leave the daemon still believing a lock was
		// confirmed when tonight's window opens, or tonight's first tick would read as
		// a dismissal of yesterday's lock.
		//
		// No mutant covers this pair, and the reason is F6's: the delete below drops
		// the whole entry unless an override is holding, and an override that is holding
		// means no lock has been attempted since it was granted -- which is the only way
		// to set either field. So no input reaches the next lockout with the entry kept
		// and the flag set. It stays as insurance against a change to that delete's
		// condition; the path that can leak is the override one, and F44 owns it.
		st.confirmedAndSeenLocked = false
		st.releases = 0
		// Per-lockout bookkeeping starts again the next time this session is inside a
		// window. Leaving unenforceableReported set would mean a child who was
		// unenforceable last night is never escalated again, because the flag says
		// "already told you" for a lockout that has since ended.
		//
		// No unconfirmedReported reset here, and the reason is not what this comment used
		// to claim. It said the override branch returns before the window check, so this
		// path is unreachable with an override held -- backwards. The window check is step
		// 3 and the override branch is step 4, so an account whose override is holding and
		// whose window has closed arrives here, with overrideActive still true.
		//
		// The conclusion holds anyway, for a different reason: the flag can never be set
		// while an override holds. Only the lock-attempt path sets it, and every tick that
		// runs under an override returns through releaseState before reaching a lock, and
		// releaseState clears the flag at the moment the exemption starts. So a flag set
		// before the override was granted is gone by the first exempt tick, and this path
		// can only ever see it false.
		//
		// When no override is holding, the delete below drops the whole entry and the next
		// lockout starts from a fresh one.
		//
		// The reset below sits flush against the `if` on purpose. A mutation pattern that
		// spans a comment breaks every time someone improves the prose -- which has now
		// happened to this mutant twice -- and the guard it stands for is worth more than
		// the sentence next to it.
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
		st.confirmedAndSeenLocked = false
		st.releases = 0
		// Same reason as the exit path, and sharper here: an override means the child
		// is handed their desktop back on purpose. If confirmedAndSeenLocked survived
		// the exemption, the tick after the override expires would report the parent's
		// own grant as the child dismissing a lock. This is the one release path where
		// the entry survives, so it is the one place that bookkeeping can leak.
		// The only reset of unconfirmedReported besides the clear-on-confirm at the top of
		// the lock path. It is load-bearing: this is the one place the state entry survives
		// a release, so it is the only place "already told you" can be carried into the next
		// lockout. TestAnOverrideExpiringDoesNotCarryTheAlarmFlagIntoTheNextLockout fails
		// if this line goes.
		st.unconfirmedReported = false
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
