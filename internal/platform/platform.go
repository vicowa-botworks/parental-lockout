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

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// Session is one desktop session on this machine, as far as the daemon needs to
// know.
type Session struct {
	ID       string // logind session id, e.g. "2"
	Username string
	UID      uint32
	Type     string // x11 | wayland | tty | mir
	State    string // active | online | closing
	Remote   bool
	Locked   bool // best effort: logind's LockedHint
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
type Locker interface {
	Lock(ctx context.Context, s Session) error
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
// daemon that locks it locks itself out of the machine. The other two are quieter.
// A session with no seat is not on any console — on a live systemd this is the
// class=manager session that shares a uid with the desktop session and has no
// desktop to lock. A session with no type (logind reports "unspecified" for it) is
// the same thing seen from the other side.
//
// State=closing is excluded because the session is on its way out: locking it races
// the teardown for no benefit.
func isLockable(s Session) bool {
	if s.Remote {
		return false
	}
	if s.State == "closing" {
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

// Lock tries each backend in configured order; the first success wins.
//
// It refuses a session that is not lockable before asking anyone, so the rule is
// enforced by the type every caller already has to go through rather than by each
// backend remembering to check.
func (c *Chain) Lock(ctx context.Context, s Session) error {
	if !isLockable(s) {
		return fmt.Errorf("refusing to lock %s: not a lockable desktop session", s.Describe())
	}
	if len(c.backends) == 0 {
		return fmt.Errorf("no lock backend available (config names %v, this binary knows %s)",
			c.unknown, strings.Join(Registered(), ", "))
	}
	var errs []error
	for _, b := range c.backends {
		if err := b.Lock(ctx, s); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", b.Name(), err))
			continue
		}
		return nil
	}
	return joinErrs(errs...)
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
