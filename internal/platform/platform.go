// Package platform is the seam between the enforcement loop and whatever desktop
// the machine is actually running.
//
// Everything the daemon does to a session — enumerate it, ask whether it is locked,
// lock it — goes through Platform, so the enforcement loop is testable against a
// fake and no test in this repository ever executes a real binary.
//
// Two rules hold across every file here, and both come from the daemon running as
// root against other people's sessions:
//
//   - Commands are argv slices, never shell strings. A username is data. The one
//     place a session's identity reaches a command is as a session id, and it is
//     passed as its own argv element, so nothing about it can be reinterpreted.
//   - Locking is only ever aimed at a desktop session. A remote session is never
//     locked (locking an SSH session would cut off the admin), and neither is a
//     session that has no seat or no session type — see isLockable.
package platform

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// Session is one desktop session on this machine, as far as the daemon needs to
// know.
//
// Class and Seat are carried rather than used only to build the list, so that the
// "never lock the wrong thing" rule can be re-checked at lock time by a caller that
// fabricated a Session by hand — the CLI in #11 will have a session id and a username,
// and nothing else.
type Session struct {
	ID       string // logind session id, e.g. "2"
	Username string
	UID      uint32
	Class    string // user | greeter | lock | manager
	Seat     string // seat0; empty for a session with no console
	Type     string // x11 | wayland | tty | mir
	State    string // active | online | closing
	Remote   bool
	Locked   bool // best effort: logind's LockedHint

	// Leader is the pid of the session's leader process and TTY its controlling
	// terminal. Both come from the listing, and both exist for one question that
	// nothing else can answer: "is this the session I am running in?" A daemon that
	// locks its own session locks itself out of ever unlocking it, and the uid policy
	// alone does not catch that when the admin and the child share an account. The
	// caller compares Leader against its own process ancestry; this package does not
	// decide the policy.
	Leader int
	TTY    string
}

// Describe is what an audit line or a status page shows. It never carries a secret,
// and it is stable enough to assert against in a test.
func (s Session) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:%s(%s)", s.Username, s.ID, s.Type)
	if s.Remote {
		b.WriteString(" remote")
	}
	if s.Locked {
		b.WriteString(" locked")
	}
	return b.String()
}

// BackendStatus is one backend's answer to "could you do your job right now".
type BackendStatus struct {
	Name      string
	Available bool
	// Detail is for a human: the binary that is missing, the error that came back.
	// It is never parsed.
	Detail string
}

// Locker is the half a backend that can only lock has to implement. The KDE and
// GNOME backends are lockers without a way to enumerate sessions, which is why
// Sessions and Lock are separable.
//
// Lock asks for the lock. Returning nil means "the request was accepted", which is
// NOT the same as "the screen is locked", and the chain therefore does not treat it
// that way — see Confirmer.
type Locker interface {
	Lock(ctx context.Context, s Session) error
}

// Confirmer is implemented by a backend that can check whether its own lock actually
// took effect.
//
// This exists because the target platform may defeat a plain "first success wins"
// chain. docs/PLAN.md §7 records that Plasma "historically" ignores logind's Lock
// signal while `loginctl lock-session` still exits 0 — a design premise carried forward
// from that row, and NOT yet measured on a live Plasma session, because measuring it
// means locking a desktop someone is using (docs/platforms.md §5 has the recipe).
//
// The chain's shape does not depend on which way that turns out. A chain that returned
// nil on logind's zero exit would stop there and never reach the desktop backend,
// reporting a lock that never happened if the premise holds — and on a raw VT the
// premise is known to hold (docs/PLAN.md §7: logind only emits the signal, and nothing
// on a VT ever calls SetLockedHint). So a backend's success counts only when it can be
// confirmed, and a backend that cannot confirm does not end the chain. A backend that
// cannot verify must not be treated as confirmed: "we asked and cannot prove it" is a
// distinct, reported outcome, never a silent success.
type Confirmer interface {
	// LockConfirmed reports whether this backend's lock is now in effect for s.
	// It must be cheap and must not lock anything.
	LockConfirmed(ctx context.Context, s Session) (bool, error)
}

// Corroborator is a Confirmer that can say whether its own yes rests on something
// besides logind's LockedHint.
//
// The distinction exists because the hint is not a private signal. Session.SetLockedHint
// is authorized by uid alone — org.freedesktop.login1.policy defines 37 actions,
// including org.freedesktop.login1.lock-sessions at auth_admin_keep, and none of them
// covers the hint, because logind assumes only a desktop writes it. #29 measured a
// session user flipping a genuinely locked session's hint to no and back with a plain
// gdbus call: no sudo, no polkit prompt, no logind event to notice.
//
// So a yes that rests on the hint alone means "the session says it is locked", which is
// a statement by the party being locked out. A backend that can add evidence that party
// did not put there says so here, and Chain.Lock weights the two accordingly.
type Corroborator interface {
	Confirmer
	// Corroborated reports whether a yes from LockConfirmed stands on something other
	// than the hint. It is a property of the backend, not of the moment: it must not
	// lock anything, query anything, or vary between calls.
	Corroborated() bool
}

// ErrUnconfirmed is what Chain.Lock returns when at least one backend accepted the
// request but none could confirm the screen is locked.
//
// It is a distinct sentinel rather than a plain error because the enforcement loop has
// to treat it differently from a failure: the request went out, so the next poll
// should re-lock and stay quiet, whereas a hard failure means nothing reached the
// desktop and an operator may need to know. errors.Is finds it through the joined
// chain.
var ErrUnconfirmed = errors.New("lock requested but not confirmed")

// unconfirmedError carries what each backend said alongside the sentinel, so the
// message an operator reads names the backends rather than just "not confirmed".
type unconfirmedError struct{ reasons []error }

func (e *unconfirmedError) Error() string {
	parts := make([]string, 0, len(e.reasons))
	for _, r := range e.reasons {
		parts = append(parts, r.Error())
	}
	return "lock requested but not confirmed: " + strings.Join(parts, "; ")
}

// Is makes errors.Is(err, ErrUnconfirmed) work for this wrapper, so callers do not
// have to know the concrete type exists.
func (e *unconfirmedError) Is(target error) bool { return target == ErrUnconfirmed }

func (e *unconfirmedError) Unwrap() []error { return e.reasons }

// LockConfirmed asks every backend whether a lock is currently in effect, and reports
// true if any of them says so.
//
// The chain needs to answer this because it is the chain the enforcement loop holds,
// and the loop's whole job is re-locking a screen that got unlocked. Without it the
// loop could only ever ask by locking again.
//
// "any" rather than "all": backends observe different things, and one that cannot see
// the session must not veto another that can see it locked. The failure direction is
// preserved — if every backend says unlocked or errors, this says unlocked.
func (c *Chain) LockConfirmed(ctx context.Context, s Session) (bool, error) {
	var errs []error
	for _, b := range c.backends {
		conf, ok := b.(Confirmer)
		if !ok {
			continue
		}
		yes, err := conf.LockConfirmed(ctx, s)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", b.Name(), err))
			continue
		}
		if yes {
			return true, nil
		}
	}
	if len(errs) > 0 {
		// Nothing could establish that the screen is locked. That is reported as
		// unlocked with the reasons attached, never as true on a guess.
		return false, joinErrs(errs...)
	}
	return false, nil
}

// Platform is a source of sessions and a way to lock them.
type Platform interface {
	Name() string
	Sessions(ctx context.Context) ([]Session, error)
	Locker
	Probe(ctx context.Context) []BackendStatus
}

// isLockable is the whole "never lock the wrong thing" rule in one place.
//
// Remote is the loud one: the admin's own session is usually the remote one, and a
// daemon that locks it locks itself out of the machine. The others are quieter.
//
// Class must be "user". A greeter session is the login screen, and locking it leaves
// an uninputtable wall where the child's account was never even chosen; a manager
// session has no desktop to lock at all. Both share a uid with a real session, so a
// uid-based caller can easily aim at the wrong one. Requiring the field to be present
// and equal to "user" also means a hand-built Session that never came from a listing
// is refused rather than trusted.
//
// A session with no seat is not on any console, and a session with no type (logind
// reports "unspecified" for the manager session) is the same thing seen from the other
// side.
//
// State=closing is excluded because the session is on its way out: locking it races
// the teardown for no benefit. Everything else, including "online", stays lockable —
// excluding a state this code has not observed would silently stop locking a real
// child session, which is the fail-open direction.
func isLockable(s Session) bool {
	if s.Remote {
		return false
	}
	if s.State == "closing" {
		return false
	}
	if s.Class != "user" {
		return false
	}
	if s.Seat == "" {
		return false
	}
	if s.Type == "" || s.Type == "unspecified" {
		return false
	}
	return true
}

// IsLockable is the exported view of the rule above, for the one caller outside this
// package that needs it: the enforcement loop asks before it evaluates a schedule,
// because a session it will never lock must not be reported as an enforceable one
// either — a greeter would otherwise show up in the audit log as a child who is not
// being locked out.
//
// The rule itself stays in isLockable, in one place, and stays unexported so that
// deciding what counts as a desktop remains this package's call. This is a door, not a
// second copy: a mutant in the rule below is still caught by this package's own tests.
func IsLockable(s Session) bool { return isLockable(s) }

// Constructor builds a backend over a Runner. Every backend takes the runner rather
// than calling exec itself, so the tests that pin argv and env are the same tests
// that exercise the backend.
//
// It also takes the settings, because one registered backend -- "command" -- runs a
// program the operator names rather than one this binary ships. Passing the whole
// Settings rather than a purpose-built struct is deliberate: a narrower bag would
// have to be assembled field by field at every New call site, and a field added to
// Settings would be silently absent from it. The backends that do not need any of it
// ignore the parameter, which is what the two shipped ones already do with the runner
// they do not use.
type Constructor func(r Runner, s config.Settings) (Platform, error)

var registry = map[string]Constructor{}

// Register makes a backend available under an id, which is the name a config's
// lockBackends list uses. It panics on a duplicate or an empty id, both of which
// are a programming error caught at init time.
func Register(id string, ctor Constructor) {
	if id == "" {
		panic("platform: Register with an empty id")
	}
	if _, dup := registry[id]; dup {
		panic("platform: backend " + id + " registered twice")
	}
	registry[id] = ctor
}

// Registered lists the backend ids this binary knows, sorted.
func Registered() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Chain is the configured backends tried in order.
type Chain struct {
	backends []Platform
	// unknown holds the ids named in the config that this binary has no backend
	// for. They are reported rather than dropped: a parent who writes "kde" into a
	// build without the KDE backend has to be told the lock will not happen, not
	// silently handed a shorter chain.
	unknown []string

	// LockBudget bounds one Lock call end to end. Zero means DefaultLockBudget.
	//
	// It is a field rather than a constant because the bound is the thing a test has
	// to be able to shrink: proving that a hung backend stops eating the chain means
	// making a call that outlives the budget, and 25s is a long time to wait for a
	// unit test.
	LockBudget time.Duration
}

// DefaultLockBudget bounds one Chain.Lock from the first backend to the last.
//
// The per-command guard in ExecRunner is not enough on its own. One Lock is many
// commands: measured at 18 subprocess calls with a session bus that owns the name and
// never replies, and 36 when the desktop accepts the lock but never writes the hint,
// because every backend then runs its own settle window. Ten seconds apiece is 2m50s
// to 6m for a single session of a single child, which is minutes of a 30s tick during
// which nothing else is enforced and the machine in front of the child stays unlocked.
//
// 25s is under one default tick, and against a healthy chain — two or three calls at
// 0.01s each — it is roughly a thousand times the cost, so it fires only when
// something is already not answering. Running out of it is not a silent outcome: the
// remaining backends are never asked, the call returns an error or ErrUnconfirmed, and
// the enforcement loop keeps the lockout and asks again on the next tick.
//
// The arithmetic that is left, stated rather than claimed: a tick that locks N
// sessions can still take N times this budget in the pathological case. That is what
// the engine's overlap guard is for — it skips the tick rather than stacking them and
// reports that enforcement is landing late.
const DefaultLockBudget = 25 * time.Second

// errLockBudget is the cause put on the deadline Chain.Lock creates, so that a note
// coming back from a backend says the lock ran out of time instead of blaming a tool
// that was merely still waiting when the budget expired.
var errLockBudget = errors.New("platform: lock budget expired")

// lockBudget resolves the configured bound.
func (c *Chain) lockBudget() time.Duration {
	if c.LockBudget != 0 {
		return c.LockBudget
	}
	return DefaultLockBudget
}

// New builds the chain named by the configuration, in the configured order.
//
// The returned errors are not fatal and the returned Chain is usable: a chain with
// one working backend and one unknown name still locks. What the errors mean is that
// the chain is shorter than whoever wrote the config believes it is.
func New(s config.Settings, r Runner) (*Chain, error) {
	var c Chain
	var errs []error
	for _, id := range s.LockBackends {
		ctor, ok := registry[id]
		if !ok {
			c.unknown = append(c.unknown, id)
			errs = append(errs, fmt.Errorf("lockBackends: no backend named %q (this binary knows %s)",
				id, strings.Join(Registered(), ", ")))
			continue
		}
		p, err := ctor(r, s)
		if err != nil {
			errs = append(errs, fmt.Errorf("lockBackends: %s: %w", id, err))
			continue
		}
		c.backends = append(c.backends, p)
	}
	return &c, joinErrs(errs...)
}

// Unknown lists configured backend ids this binary could not build.
func (c *Chain) Unknown() []string { return append([]string(nil), c.unknown...) }

// Names lists the backends actually in the chain, in order.
func (c *Chain) Names() []string {
	out := make([]string, 0, len(c.backends))
	for _, b := range c.backends {
		out = append(out, b.Name())
	}
	return out
}

func (c *Chain) Name() string { return "chain(" + strings.Join(c.Names(), " ") + ")" }

// Sessions asks every backend and merges by session id.
//
// Merging rather than first-answer-wins matters once more than one backend can
// enumerate: two backends reporting the same session must not produce two entries,
// because the enforcement loop would lock it twice and log two audit events for one
// action. The first backend to report a session wins, so the order in the config
// decides whose view of Locked is the one that gets used.
//
// An empty result with a nil error is normal and means "nobody is on a lockable
// desktop session right now" — which is the correct answer at 3am and on a machine
// where the child has not logged in. It is deliberately not an error: making the
// healthy idle state an error would have the loop log a failure every 30 seconds and
// train everyone to ignore it. A caller that needs to tell "nobody home" apart from
// "the backend is broken" asks Probe, which reports whether the backend can be talked
// to at all.
func (c *Chain) Sessions(ctx context.Context) ([]Session, error) {
	seen := map[string]bool{}
	var out []Session
	var errs []error
	for _, b := range c.backends {
		ss, err := b.Sessions(ctx)
		// Merge whatever came back before deciding what to do about the error. A
		// backend is allowed to return results alongside an error — logind does, when
		// one session vanished or failed to answer while the rest did not — and
		// dropping those results here would mean a partially broken backend stops
		// enforcement for every session on the machine.
		for _, s := range ss {
			if seen[s.ID] {
				continue
			}
			seen[s.ID] = true
			out = append(out, s)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", b.Name(), err))
		}
	}
	if len(out) == 0 && len(errs) > 0 {
		return nil, joinErrs(errs...)
	}
	// A backend that failed while another produced sessions is reported alongside
	// the results rather than hidden: the loop should still enforce, but "the KDE
	// backend cannot list sessions" is not something to swallow.
	return out, joinErrs(errs...)
}

// confirmSettle bounds how long Chain.Lock waits for a lock to become visible after a
// backend accepted the request.
//
// logind's lock-session returns as soon as the Lock signal is queued on D-Bus; it is
// the desktop environment that later calls SetLockedHint, and LockedHint is the only
// evidence this package can read. Asking whether the lock took the instant the request
// returns therefore races the desktop, and loses on any machine where it takes a few
// milliseconds to react: the screen does get locked, and the chain reports success as
// failure, moves on to the next backend, and locks it a second time.
//
// Ten reads a tenth of a second apart is a second, far past any real signal round
// trip, and the loop asks at most once per user per 30 seconds.
//
// That second is the healthy cost, and only the healthy cost. Each read is a command,
// and a command that meets a service which owns the name and never replies costs up to
// DefaultCallTimeout, so an unbounded-in-the-aggregate window on a hung bus is ten
// reads times ten seconds per backend. The bound on that is not here: it is
// DefaultLockBudget around the whole Chain.Lock, which is also what stops the settle
// loop, because every read takes the same context and a cancelled one reports
// unlocked at once rather than after the remaining polls.
const (
	confirmReads = 10
	confirmPoll  = 100 * time.Millisecond
)

// settleWait parks the caller between reads. It is a var so the retry loop can be
// tested without spending a second per case — the same reason Runner exists for
// commands. TestMain swaps it for an instant one; sleepCtx itself is pinned directly.
var settleWait = sleepCtx

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// confirmWithSettle asks a backend whether the lock took, giving an asynchronous lock
// a bounded number of reads to become visible before answering "no".
//
// Only a negative is retried. A yes and an error both return immediately: a backend
// that cannot be queried will not start answering between two polls, and retrying an
// error would hold up the rest of the chain for no information.
//
// A context cancelled mid-window reports unlocked rather than an error. The chain is
// about to hand the same context to the next backend, and "we asked and cannot prove
// it" is the honest verdict whether the deadline or the desktop was the reason.
func confirmWithSettle(ctx context.Context, conf Confirmer, s Session) (bool, error) {
	yes, err := conf.LockConfirmed(ctx, s)
	if err != nil || yes {
		return yes, err
	}
	for i := 0; i < confirmReads; i++ {
		if !settleWait(ctx, confirmPoll) {
			return false, nil
		}
		yes, err = conf.LockConfirmed(ctx, s)
		if err != nil || yes {
			return yes, err
		}
	}
	return false, nil
}

// corroborated reports whether this confirmer stands behind its yes with evidence the
// controlled user did not supply.
//
// The type assertion alone is not the question. Both desktop flavours implement
// Corroborator on the same type, so `_, ok := conf.(Corroborator)` is true for gnome
// even though gnome has nothing to corroborate with — and treating it as a corroborator
// is the #29 attack arriving through a gnome-only config: a hint the child set from its
// own account confirms the lock, the chain stops, and the screen never locks. The answer
// the method returns is the whole difference between the two flavours, so the answer is
// what gets asked.
func corroborated(conf Confirmer) bool {
	c, ok := conf.(Corroborator)
	return ok && c.Corroborated()
}

// lockState reads the lock state from the backend that reports the raw hint, for the
// question Chain.Lock asks: what was true before we asked anybody?
//
// It deliberately prefers a confirmer that does NOT corroborate. Asking a corroborating
// backend instead would ask the wrong question — "was the lock already proved?" rather
// than "was the forgeable signal already set?" — and a chain configured with the desktop
// backend ahead of logind would then read false on a spoofed machine, because the
// corroborator correctly says "not locked", and logind's bare-hint yes would be taken at
// face value. That is the attack, arriving through the configuration order.
//
// When every confirmer corroborates there is no uncorroborated signal left to discount,
// and the fallback answer is never used: such a backend returns nil before the value is
// consulted.
// lockState asks the chain what it believes is true right now.
//
// Three answers, not two. `err` is the verdict-level failure -- nothing in this chain
// could tell us anything -- and only that should discount a later uncorroborated yes.
// `notes` carries the confirmers that faulted on the way but did not decide the answer;
// they are not the verdict and they are not nothing, so they go to the caller to be
// reported rather than being collected and dropped.
func (c *Chain) lockState(ctx context.Context, s Session) (bool, error, []error) {
	var fallback Confirmer
	var fallbackName string
	// Every uncorroborated confirmer, not just the first. See the loop below.
	var plain []Confirmer
	var plainNames []string
	for i, b := range c.backends {
		conf, ok := b.(Confirmer)
		if !ok {
			continue
		}
		if !corroborated(conf) {
			plain = append(plain, conf)
			plainNames = append(plainNames, c.backends[i].Name())
			continue
		}
		if fallback == nil {
			fallback, fallbackName = conf, b.Name()
		}
	}
	// A read that fails is that confirmer's problem, not the operation's. Returning on
	// the first error would set preErr for the whole Chain.Lock and discount every later
	// uncorroborated yes, including one from a second confirmer that answered for itself.
	// Only the first confirmer that can answer at all decides, though: every uncorroborated
	// one reads the same LockedHint, so asking the rest after a clean answer buys a
	// disagreement the real desktop cannot produce.
	var errs []error
	for i, conf := range plain {
		yes, err := conf.LockConfirmed(ctx, s)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", plainNames[i], err))
			continue
		}
		return yes, nil, errs
	}
	if fallback == nil {
		if len(errs) > 0 {
			return false, errors.Join(errs...), nil
		}
		return false, fmt.Errorf("no backend in this chain can confirm a lock"), nil
	}
	yes, err := fallback.LockConfirmed(ctx, s)
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", fallbackName, err))
		return false, errors.Join(errs...), nil
	}
	// The fallback answered, so nothing above decided the verdict -- but a confirmer
	// that faulted on the way is still worth saying. Dropping it made the comment on
	// this function a promise the code did not keep: the fault was collected and then
	// discarded whenever some later backend happened to answer cleanly. It goes back to
	// the caller as notes now, which is where Chain.Lock already puts everything worth
	// reporting that is not the verdict.
	return yes, nil, errs
}

// Lock asks each backend in configured order until one confirms the lock took effect.
//
// It refuses a session that is not lockable before asking anyone, so the rule is
// enforced by the type every caller already has to go through rather than by each
// backend remembering to check.
//
// A backend that returns an error is a failure and the chain moves on. A backend that
// returns nil is only done if it can confirm: otherwise the chain keeps going, because
// an unconfirmed success is exactly what logind produces on KDE, and stopping there
// would leave the screen unlocked while the audit log said otherwise.
//
// If every backend was tried and none confirmed, the result is ErrUnconfirmed rather
// than nil. Nothing here returns nil without evidence.
//
// The whole call is bounded by LockBudget, which is what keeps a service that has
// stopped answering from turning one lock into minutes. Running out of it needs no new
// outcome, because the two shapes it produces are already the right sentences: a
// backend that had accepted the lock but could not be confirmed in time lands in
// ErrUnconfirmed, and one that never got asked lands in the joined errors. Both keep
// the enforcement loop re-arming.
func (c *Chain) Lock(ctx context.Context, s Session) error {
	if !isLockable(s) {
		return fmt.Errorf("refusing to lock %s: not a lockable desktop session", s.Describe())
	}
	if len(c.backends) == 0 {
		return fmt.Errorf("no lock backend available (config names %v, this binary knows %s)",
			c.unknown, strings.Join(Registered(), ", "))
	}

	// Applied here rather than at the call sites so it cannot be forgotten, the same
	// reason the per-command bound lives in ExecRunner. A caller's earlier deadline
	// still wins, because WithTimeout takes the sooner of the two.
	ctx, cancel := context.WithTimeoutCause(ctx, c.lockBudget(), errLockBudget)
	// The locker-process answer is memoized for exactly this operation's lifetime, and
	// this is the only place it can be attached: it has to outlive every confirmation
	// inside the operation and no further, and a cache on the backend would be shared by
	// concurrent operations and survive the tick that produced it.
	ctx = withLockerCache(ctx)
	defer cancel()

	// Read the lock state once, before anyone is asked.
	//
	// Without this the chain is defeated by construction, and by the very mechanism
	// built to make it trustworthy. The hint is already true — one autostart file
	// running SetLockedHint true at login does it, as the controlled user, with no
	// privilege (#29). logind's Lock is then accepted, its confirmation reads the hint
	// back, sees true, and the chain stops: the desktop backend is never reached, the
	// screen never locks, and the audit log records a lock that happened. The
	// confirmation chain was supposed to be the thing that survives a child dismissing
	// the lock screen, and a hint pre-set from the child's own account switches it off.
	//
	// Knowing what was true beforehand turns that into an argument the daemon can see
	// through: a hint that was already true cannot be the consequence of a request that
	// had not been made yet. So a yes from a backend that cannot corroborate it counts
	// only if the state changed under us.
	//
	// One read for the whole operation, not one per backend: the question is what held
	// when we started, and every backend here confirms off the same property.
	wasLocked, preErr, preNotes := c.lockState(ctx, s)

	// Everything the pre-read learned but could not make a verdict, in front: it is the
	// earliest thing worth saying, and a confirmer that faults before anyone is asked is
	// the one a reader needs first to understand the rest.
	notes := preNotes // everything worth saying, in backend order
	var failures int  // backends that returned an error rather than accepting
	for _, b := range c.backends {
		if err := b.Lock(ctx, s); err != nil {
			failures++
			notes = append(notes, fmt.Errorf("%s: %w", b.Name(), err))
			continue
		}
		conf, ok := b.(Confirmer)
		if !ok {
			// Success we cannot stand behind. Keep trying the rest of the chain and
			// say so in the outcome.
			notes = append(notes, fmt.Errorf("%s: accepted the request but cannot confirm it", b.Name()))
			continue
		}
		yes, err := confirmWithSettle(ctx, conf, s)
		if err != nil {
			notes = append(notes, fmt.Errorf("%s: could not confirm: %w", b.Name(), err))
			continue
		}
		if yes {
			// A backend that stands behind its yes with evidence the controlled user
			// did not supply has proved the lock whatever was true beforehand, and the
			// steady state of a locked machine depends on that: the loop re-asserts
			// every tick, so from the second tick on the hint is always already true.
			// Discounting a corroborated yes would report unconfirmed forever on a
			// genuinely locked screen, and an operator trained to ignore that signal
			// has no signal left.
			if corroborated(conf) {
				return nil
			}
			if wasLocked {
				notes = append(notes, fmt.Errorf(
					"%s: reports the session locked, but it already reported locked before we asked. "+
						"LockedHint is writable by the session user, so that is not evidence this lock took",
					b.Name()))
				continue
			}
			if preErr != nil {
				// The same reading, and the safe one: if the state beforehand could not
				// be established, the change cannot be claimed as ours. A backend whose
				// pre-read fails will usually fail its post-read too, since they are the
				// same call, so this costs little and closes the gap where they differ.
				notes = append(notes, fmt.Errorf(
					"%s: reports the session locked, but the state before the request could not be read "+
						"(%w), so this cannot be told apart from a hint that was already true",
					b.Name(), preErr))
				continue
			}
			return nil
		}
		notes = append(notes, fmt.Errorf("%s: accepted the request but the session is still unlocked", b.Name()))
	}
	if failures == len(c.backends) {
		// Nothing ever reached the desktop. That is a different report from "we asked
		// and cannot prove it": here the operator may need to intervene, and the loop
		// should not file it under the same heading.
		return joinErrs(notes...)
	}
	return &unconfirmedError{reasons: notes}
}

// Probe asks every backend whether it is usable. It never locks anything.
func (c *Chain) Probe(ctx context.Context) []BackendStatus {
	var out []BackendStatus
	for _, b := range c.backends {
		out = append(out, b.Probe(ctx)...)
	}
	for _, id := range c.unknown {
		out = append(out, BackendStatus{Name: id, Detail: "no such backend in this binary"})
	}
	return out
}

// joinErrs is errors.Join with the nil-noise removed: joining nothing gives nil,
// and a slice with one error returns that error unwrapped so %w keeps working
// through it.
func joinErrs(errs ...error) error {
	var kept []error
	for _, e := range errs {
		if e != nil {
			kept = append(kept, e)
		}
	}
	switch len(kept) {
	case 0:
		return nil
	case 1:
		return kept[0]
	default:
		return errors.Join(kept...)
	}
}
