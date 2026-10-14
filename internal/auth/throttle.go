package auth

import (
	"sort"
	"sync"
	"time"
)

// Throttle limits how fast somebody may guess the admin password.
//
// It protects two things at once, and they need different shapes. The first is
// the obvious one: 600 000 iterations makes an *offline* crack expensive, but a
// network attacker who can try as often as they like is not offline, and each
// attempt also costs the daemon a quarter of a second of a core — so without a
// limit, the login endpoint is simultaneously a guessing oracle and a cheap
// denial of service against the only interface that turns enforcement back on.
//
// The second is the parent on the other side of that. A shared connection means
// the household's one public IP may be carrying a stranger's flood, so the global
// limit has a fixed end that a continuing flood cannot extend. Without that rule
// the response to a sustained attack is "the real parent is locked out for as long
// as the attack lasts", which is the attack working.
//
// Call Allow before verifying, not after. The point is to not do the work.
type Throttle struct {
	// Limit is how many attempts one key may make per Window.
	Limit int
	// Window is the rolling period the allowance is measured over.
	Window time.Duration
	// PenaltyStart is the delay after the first failure; it doubles per
	// consecutive failure up to PenaltyCap.
	PenaltyStart time.Duration
	// PenaltyCap bounds the per-key penalty. Uncapped, a day of background
	// guessing would leave a delay measured in hours, and the parent would conclude
	// the machine is broken rather than that they were locked out.
	PenaltyCap time.Duration
	// GlobalLimit is how many failures across every key in Window trigger a global
	// cooldown. It is far above anything one household produces, so it fires on a
	// flood and not on a fumbling parent.
	GlobalLimit int
	// GlobalCooldown is how long every key is held when the global limit trips.
	// It is a ceiling, not a duration to be re-armed: see the note on the type.
	GlobalCooldown time.Duration

	mu   sync.Mutex
	keys map[string]*keyState
	// global counts failures across every key, as timestamps so the window rolls.
	global      []time.Time
	globalUntil time.Time
	now         func() time.Time
}

type keyState struct {
	failures     []time.Time
	consecutive  int
	blockedUntil time.Time
}

// NewThrottle returns a throttle with the documented defaults: ten attempts per
// ten minutes per key, a penalty starting at one second and capping at thirty, and
// a global cooldown of five minutes that a continuing flood cannot extend.
func NewThrottle() *Throttle {
	return &Throttle{
		Limit:          10,
		Window:         10 * time.Minute,
		PenaltyStart:   time.Second,
		PenaltyCap:     30 * time.Second,
		GlobalLimit:    200,
		GlobalCooldown: 5 * time.Minute,
		keys:           map[string]*keyState{},
		now:            time.Now,
	}
}

// Allow reports whether an attempt may proceed now, and how long to wait if it
// may not.
//
// The returned duration is advisory — it tells the handler what to put in
// Retry-After, and it is what a test asserts on. The handler must not sleep it:
// holding an HTTP connection open for thirty seconds per guess is a slower
// denial of service than the one the throttle exists to prevent.
func (t *Throttle) Allow(key string) (bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	at := t.clock()
	t.purgeLocked(at)

	// Both deadlines are tested for "is set" before they are tested for "is in the
	// future". The zero time is in the past, so `!at.Before(zero)` is true, and a
	// bare comparison turns "no cooldown has ever been set" into "every key on
	// earth is blocked" — with a wait computed against year 1, which comes out as
	// the most negative duration there is. A handler that renders that as
	// Retry-After says "try again immediately", so the bug is not even a reliable
	// lockout: it is a throttle that reports itself working.
	if blocked, wait := t.blockedFor(at); blocked {
		return false, wait
	}
	ks := t.keys[key]
	if ks == nil {
		return true, 0
	}
	if !ks.blockedUntil.IsZero() && at.Before(ks.blockedUntil) {
		return false, ks.blockedUntil.Sub(at)
	}
	// The bucket is the timestamps themselves rather than a counter with a reset
	// timer: "ten per ten minutes" then means ten in any rolling ten minutes, so
	// an attacker cannot spend the allowance twice in quick succession by waiting
	// for a boundary that a fixed-window counter would have reset.
	cutoff := at.Add(-t.window())
	ks.failures = keepAfter(ks.failures, cutoff)
	if len(ks.failures) >= t.limit() {
		// The allowance comes back one attempt at a time as each failure ages out,
		// so the wait is until the oldest one leaves the window.
		return false, ks.failures[0].Add(t.window()).Sub(at)
	}
	return true, 0
}

// blockedFor reports whether the global cooldown currently covers this instant.
//
// Split out so the zero-time rule has one home. The same rule applies to the
// per-key deadline in Allow, and the two must not drift: the failure mode of
// getting it wrong is that the throttle silently stops throttling.
func (t *Throttle) blockedFor(at time.Time) (bool, time.Duration) {
	if t.globalUntil.IsZero() {
		return false, 0
	}
	if at.Before(t.globalUntil) {
		return true, t.globalUntil.Sub(at)
	}
	// Past its end. Clearing it here means a later Failure starts from "not
	// already in cooldown", which is what makes the cooldown non-extensible.
	t.globalUntil = time.Time{}
	return false, 0
}

// Failure records a wrong password for a key and applies the escalating penalty.
func (t *Throttle) Failure(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	at := t.clock()
	t.purgeLocked(at)

	if t.keys == nil {
		t.keys = map[string]*keyState{}
	}
	ks := t.keys[key]
	if ks == nil {
		ks = &keyState{}
		t.keys[key] = ks
	}
	ks.failures = append(keepAfter(ks.failures, at.Add(-t.window())), at)
	ks.consecutive++
	ks.blockedUntil = at.Add(t.penalty(ks.consecutive))

	t.global = append(keepAfter(t.global, at.Add(-t.window())), at)
	if len(t.global) >= t.globalLimit() && !at.Before(t.globalUntil) {
		// The condition is "not already in cooldown", not "trip again". A flood
		// that keeps arriving would otherwise re-arm the cooldown on every failure
		// and the global limit would become a permanent lockout — which is the
		// failure mode the global limit exists to avoid, so the trip is a fixed
		// window from the moment it fires and no later.
		t.globalUntil = at.Add(t.cooldown())
	}
}

// Success clears a key's history.
//
// Deleting the entry rather than zeroing it is what makes "the parent signed in,
// so the parent is not being throttled" true immediately: a correct password also
// proves the guessing has stopped, and carrying a penalty forward past a
// successful login punishes the one person whose identity the penalty was meant to
// protect.
func (t *Throttle) Success(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.keys, key)
}

// Reset clears everything, including any global cooldown. It is for tests and for
// an operator who has established the flood has stopped.
func (t *Throttle) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.keys = map[string]*keyState{}
	t.global = nil
	t.globalUntil = time.Time{}
}

// penalty is the escalating delay for the n-th consecutive failure: 1s, 2s, 4s,
// capped.
//
// The shift is guarded rather than allowed to run off the end of an int64. A
// long-running daemon with a persistent guesser reaches consecutive counts where
// 1<<n overflows, and an overflowed duration can come out negative — which
// Allow reads as "not blocked", so the arithmetic bug would turn off the throttle
// precisely when it has been working hardest.
func (t *Throttle) penalty(n int) time.Duration {
	start := t.penaltyStart()
	cap := t.penaltyCap()
	if n <= 1 {
		return start
	}
	shift := n - 1
	if shift >= 31 {
		return cap
	}
	p := start << shift
	if p <= 0 || p > cap {
		// p <= 0 is the overflow case described above.
		return cap
	}
	return p
}

// purgeLocked drops keys that can no longer affect a decision.
//
// It runs on access rather than on a timer, for the same reason the session store
// does: a goroutine needs a lifecycle and misses the window where the process was
// not running it. The scan is bounded by the number of distinct keys, which is
// bounded in turn by the throttle doing its job — and a flood of distinct keys is
// exactly what trips the global cooldown.
func (t *Throttle) purgeLocked(at time.Time) {
	cutoff := at.Add(-t.window())
	for k, ks := range t.keys {
		ks.failures = keepAfter(ks.failures, cutoff)
		if len(ks.failures) == 0 && !at.Before(ks.blockedUntil) {
			delete(t.keys, k)
		}
	}
	t.global = keepAfter(t.global, cutoff)
}

// keepAfter trims a sorted timestamp slice to its last window.
func keepAfter(ts []time.Time, cutoff time.Time) []time.Time {
	i := sort.Search(len(ts), func(i int) bool { return ts[i].After(cutoff) })
	if i == 0 {
		return ts
	}
	return ts[i:]
}

// clock is the time source, defaulting to the real one.
//
// A zero-value Throttle must work. The alternative is a nil func call, and a panic
// in the login path is not a refused login: it is the daemon going down, which for
// this product means enforcement stops being re-asserted until systemd notices.
// The same reasoning is why internal/enforce's Logf and internal/audit's Logf
// default rather than staying nil.
func (t *Throttle) clock() time.Time {
	if t.now != nil {
		return t.now()
	}
	return time.Now()
}

func (t *Throttle) window() time.Duration {
	if t.Window > 0 {
		return t.Window
	}
	return 10 * time.Minute
}

func (t *Throttle) limit() int {
	if t.Limit > 0 {
		return t.Limit
	}
	return 10
}

func (t *Throttle) penaltyStart() time.Duration {
	if t.PenaltyStart > 0 {
		return t.PenaltyStart
	}
	return time.Second
}

func (t *Throttle) penaltyCap() time.Duration {
	if t.PenaltyCap > 0 {
		return t.PenaltyCap
	}
	return 30 * time.Second
}

func (t *Throttle) globalLimit() int {
	if t.GlobalLimit > 0 {
		return t.GlobalLimit
	}
	return 200
}

func (t *Throttle) cooldown() time.Duration {
	if t.GlobalCooldown > 0 {
		return t.GlobalCooldown
	}
	return 5 * time.Minute
}
