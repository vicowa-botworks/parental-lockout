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

// Constructor builds a backend over a Runner. Every backend takes the runner rather
// than calling exec itself, so the tests that pin argv and env are the same tests
// that exercise the backend.
type Constructor func(r Runner) (Platform, error)

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
		p, err := ctor(r)
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
// trip. The loop calls this at most once per user per 30 seconds, so the wait costs
// nothing worth naming.
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
func (c *Chain) Lock(ctx context.Context, s Session) error {
	if !isLockable(s) {
		return fmt.Errorf("refusing to lock %s: not a lockable desktop session", s.Describe())
	}
	if len(c.backends) == 0 {
		return fmt.Errorf("no lock backend available (config names %v, this binary knows %s)",
			c.unknown, strings.Join(Registered(), ", "))
	}

	var notes []error // everything worth saying, in backend order
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
