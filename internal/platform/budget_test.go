package platform

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// The per-command bound in ExecRunner is not a bound on a lock. One Chain.Lock is many
// commands, and the two failure modes that produce the most of them are both real: a
// session bus that owns the name and never replies, and a desktop that accepts the lock
// and never writes the hint, which sends every backend through its own settle window.
// Measured at 17 and 36 subprocess calls respectively, so a 10s per-command guard
// permits minutes for a single session while the child sits at an unlocked screen and
// nothing else on the machine is enforced.
//
// These tests pin the operation-level bound instead.

// hungBackend is a desktop that owns the bus name and never answers: the call runs
// until the context it was given decides.
//
// The backstop matters for what this file is testing. A mutant that removes the budget
// hands out a context with no deadline at all, and a backend that waits only on
// ctx.Done() would then hang the suite rather than fail a test. Waiting on either turns
// "no bound" into a slow test that says why.
type hungBackend struct {
	name string

	mu          sync.Mutex
	calls       int
	noDeadline  int // calls that arrived with no bound at all
	backstopHit int
}

func (h *hungBackend) Name() string { return h.name }

func (h *hungBackend) Sessions(context.Context) ([]Session, error) { return nil, nil }

func (h *hungBackend) Lock(ctx context.Context, _ Session) error {
	_, bounded := ctx.Deadline()

	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		h.mu.Lock()
		h.backstopHit++
		h.mu.Unlock()
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls++
	if !bounded {
		h.noDeadline++
	}
	return context.DeadlineExceeded
}

func (h *hungBackend) Probe(context.Context) []BackendStatus { return nil }

func (h *hungBackend) counts() (calls, unbounded, backstops int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.calls, h.noDeadline, h.backstopHit
}

func TestChainLockBoundsTheWholeOperation(t *testing.T) {
	a := &hungBackend{name: "a"}
	b := &hungBackend{name: "b"}
	c := &Chain{backends: []Platform{a, b}, LockBudget: 60 * time.Millisecond}

	start := time.Now()
	err := c.Lock(context.Background(), desktopSession("7"))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("a lock nobody answered reported success")
	}
	// The arithmetic this replaces: two backends with nothing between them would each
	// have run to the backstop, so the call would have taken seconds rather than the
	// budget — and a tick that spends minutes on one session enforces nobody else.
	if elapsed > 2*time.Second {
		t.Errorf("one Chain.Lock took %v with a 60ms budget; if that is the whole chain's "+
			"cost then the bound is still per command", elapsed)
	}

	for _, h := range []*hungBackend{a, b} {
		calls, unbounded, backstops := h.counts()
		if calls == 0 {
			t.Errorf("backend %q was never asked", h.name)
			continue
		}
		if unbounded > 0 {
			t.Errorf("backend %q got %d call(s) with no deadline: Chain.Lock does not bound the operation",
				h.name, unbounded)
		}
		if backstops > 0 {
			t.Errorf("backend %q ran past every bound", h.name)
		}
	}
}

// The budget must never loosen a deadline the caller already set. Same rule as
// ExecRunner.CallTimeout: a caller with a careful budget cannot be quietly extended by
// a default it never asked for.
func TestChainLockBudgetOnlyTightensACallerDeadline(t *testing.T) {
	h := &hungBackend{name: "a"}
	c := &Chain{backends: []Platform{h}, LockBudget: time.Hour}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	start := time.Now()
	if err := c.Lock(ctx, desktopSession("7")); err == nil {
		t.Fatal("a lock nobody answered reported success")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %v; the 1h budget overrode the caller's 60ms deadline", elapsed)
	}
}

// acceptingBackend locks instantly and then cannot say whether it worked until its
// context gives up: the desktop that accepts the request and never writes the hint,
// which is the open question in docs/platforms.md §4.
type acceptingBackend struct {
	*Mock
}

func (a *acceptingBackend) LockConfirmed(ctx context.Context, _ Session) (bool, error) {
	select {
	case <-ctx.Done():
		return false, nil
	case <-time.After(3 * time.Second):
		return false, nil
	}
}

// Running out of the lock budget must not become a success, and it needs no new
// outcome: a backend that accepted the lock but could not be confirmed in time already
// belongs with "we asked and cannot prove it", which is the sentence that keeps the
// enforcement loop re-arming.
func TestChainLockBudgetExpiryIsNeverReportedAsSuccess(t *testing.T) {
	c := &Chain{
		backends:   []Platform{&acceptingBackend{Mock: &Mock{BackendName: "kde"}}},
		LockBudget: 40 * time.Millisecond,
	}

	start := time.Now()
	err := c.Lock(context.Background(), desktopSession("7"))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("an unconfirmed lock reported success")
	}
	if !errors.Is(err, ErrUnconfirmed) {
		t.Errorf("error = %v, want ErrUnconfirmed: the desktop did accept the request", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("the settle window outlived the lock budget (%v); the confirmation reads "+
			"must take the same context the chain bounded", elapsed)
	}
}

// The default has to be a real bound. A constant that drifts to zero would mean "no
// bound" while still looking configured, and the point of the field is that production
// never has to remember to set it.
func TestChainLockBudgetResolvesItsDefault(t *testing.T) {
	if DefaultLockBudget <= 0 {
		t.Errorf("DefaultLockBudget = %v; a zero default means no bound at all", DefaultLockBudget)
	}
	if got := (&Chain{}).lockBudget(); got != DefaultLockBudget {
		t.Errorf("zero LockBudget resolved to %v, want DefaultLockBudget", got)
	}
	if got := (&Chain{LockBudget: 2 * time.Second}).lockBudget(); got != 2*time.Second {
		t.Errorf("LockBudget = %v, want 2s", got)
	}
	// Under one default tick, or the bound stops meaning what it claims about a tick.
	if DefaultLockBudget >= 30*time.Second {
		t.Errorf("DefaultLockBudget = %v, which is not inside one 30s tick", DefaultLockBudget)
	}
}

// causeSpyBackend records why its context ended, which is the only way to see the
// cause the chain put on the deadline it created.
type causeSpyBackend struct {
	name string

	mu    sync.Mutex
	cause error
}

func (s *causeSpyBackend) Name() string { return s.name }

func (s *causeSpyBackend) Sessions(context.Context) ([]Session, error) { return nil, nil }

func (s *causeSpyBackend) Lock(ctx context.Context, _ Session) error {
	<-ctx.Done()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cause = context.Cause(ctx)
	return ctx.Err()
}

func (s *causeSpyBackend) Probe(context.Context) []BackendStatus { return nil }

func (s *causeSpyBackend) why() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cause
}

// The cause on the deadline is what lets a note coming back from a backend say the lock
// ran out of time instead of blaming a tool that was merely still waiting when the
// budget expired. ExecRunner reads the same distinction to choose between "no answer
// within 10s" and "the caller stopped waiting"; without a cause of its own the chain's
// budget would be reported as a hung binary, which is the wrong thing to go and fix.
func TestChainLockPutsItsOwnCauseOnTheDeadline(t *testing.T) {
	spy := &causeSpyBackend{name: "a"}
	c := &Chain{backends: []Platform{spy}, LockBudget: 30 * time.Millisecond}

	if err := c.Lock(context.Background(), desktopSession("7")); err == nil {
		t.Fatal("a lock nobody answered reported success")
	}
	if cause := spy.why(); !errors.Is(cause, errLockBudget) {
		t.Errorf("context.Cause after the budget expired = %v, want it to name the lock budget", cause)
	}
}
