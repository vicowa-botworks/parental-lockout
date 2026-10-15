package auth

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func newTestThrottle() (*Throttle, *fakeClock) {
	clock := newClock()
	t := NewThrottle()
	t.Now = clock.Now
	// Shrink the real numbers so the tests exercise the rules rather than waiting
	// on them. Every assertion names the value it is checking against, so a
	// shortened window cannot hide a broken rule.
	t.Window = 10 * time.Minute
	t.Limit = 10
	t.PenaltyStart = time.Second
	t.PenaltyCap = 30 * time.Second
	return t, clock
}

// spend makes n attempts, each of which fails, sitting out the escalating penalty
// in between. Without it the test is measuring the penalty rather than the
// allowance, which is the mistake the first version of these two tests made.
func spend(th *Throttle, clock *fakeClock, key string, n int) {
	for i := 0; i < n; i++ {
		th.Failure(key)
		ok, wait := th.Allow(key)
		if ok {
			continue
		}
		if wait > 0 {
			clock.Advance(wait)
		}
	}
}

func TestTenAttemptsThenBlocked(t *testing.T) {
	th, clock := newTestThrottle()

	for i := 0; i < 10; i++ {
		if ok, wait := th.Allow("1.2.3.4"); !ok {
			t.Fatalf("attempt %d was blocked (wait %v); the allowance is 10", i+1, wait)
		}
		th.Failure("1.2.3.4")
		clock.Advance(30 * time.Second) // longer than the cap, so the penalty never gates this loop
	}
	ok, wait := th.Allow("1.2.3.4")
	if ok {
		t.Fatal("the eleventh attempt in the window was allowed")
	}
	if wait <= 0 {
		t.Errorf("blocked with wait %v; the caller cannot render a Retry-After", wait)
	}
}

// The allowance is measured over a rolling window, not a fixed one. With a fixed
// counter an attacker spends ten attempts, waits for the boundary, and spends ten
// more immediately — twice the rate the limit describes.
func TestTheWindowRollsRatherThanResetting(t *testing.T) {
	th, clock := newTestThrottle()

	for i := 0; i < 10; i++ {
		th.Failure("host")
	}
	// One minute later the bucket is still full.
	clock.Advance(time.Minute)
	if ok, _ := th.Allow("host"); ok {
		t.Fatal("allowed after 1 minute of a 10 minute window")
	}
	// The first failure ages out, so exactly one attempt becomes available — not
	// the whole allowance.
	clock.Advance(9*time.Minute + time.Millisecond)
	if ok, _ := th.Allow("host"); !ok {
		t.Fatal("the oldest failure has left the window; one attempt should be available")
	}
	th.Failure("host")
	if ok, _ := th.Allow("host"); ok {
		t.Fatal("a second attempt was allowed when only one failure had aged out")
	}
}

func TestSuccessClearsTheHistory(t *testing.T) {
	th, clock := newTestThrottle()

	spend(th, clock, "host", 9)
	th.Success("host")
	// The parent has just proved who they are. Carrying the bucket forward would
	// punish the one person the throttle is meant to protect.
	for i := 0; i < 10; i++ {
		if ok, wait := th.Allow("host"); !ok {
			t.Fatalf("after a success, attempt %d blocked (wait %v)", i+1, wait)
		}
		th.Failure("host")
		clock.Advance(30 * time.Second)
	}
}

func TestThePenaltyGrowsAndCaps(t *testing.T) {
	th, clock := newTestThrottle()

	want := []time.Duration{
		time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second,
		16 * time.Second, 30 * time.Second, 30 * time.Second,
	}
	for i, expected := range want {
		th.Failure("host")
		ok, wait := th.Allow("host")
		if ok {
			t.Fatalf("failure %d: allowed immediately, want a %v penalty", i+1, expected)
		}
		if wait != expected {
			t.Errorf("failure %d: wait = %v, want %v", i+1, wait, expected)
		}
		// Sit out the penalty so the next failure is the next consecutive one.
		clock.Advance(wait)
	}
}

// An uncapped exponential penalty eventually overflows. A negative duration reads
// as "not blocked", so the arithmetic bug would switch the throttle off exactly
// when it has been working hardest — after hours of guessing.
func TestThePenaltyNeverOverflowsIntoBeingUnblocked(t *testing.T) {
	th, clock := newTestThrottle()

	for i := 0; i < 200; i++ {
		th.Failure("host")
		ok, wait := th.Allow("host")
		if ok {
			t.Fatalf("failure %d: allowed, after a penalty that should have saturated at the cap", i+1)
		}
		if wait <= 0 {
			t.Fatalf("failure %d: wait = %v; a non-positive wait reads as \"not blocked\" and turns the throttle off", i+1, wait)
		}
		if wait > th.PenaltyCap {
			t.Fatalf("failure %d: wait = %v, above the cap %v", i+1, wait, th.PenaltyCap)
		}
		clock.Advance(wait)
	}
}

// The penalty is a pure function of the consecutive count, and the two guards in
// it cannot be reached through Allow.
//
// Reaching a 64th consecutive failure needs 64 attempts inside the window, and the
// bucket refuses the 11th — so by the time the shift would overflow, the bucket is
// what is answering, and it answers with the same "wait 30s" the capped penalty
// would have produced. That makes the guards unobservable through the public API,
// which is not the same as unobservable: an overflowed duration is negative, and
// a negative wait reads as "not blocked" wherever it is compared. So the
// invariant is asserted on the function that owns it, for every count the counter
// can reach.
func TestThePenaltyIsAlwaysAUsableDuration(t *testing.T) {
	th, _ := newTestThrottle()
	for n := 1; n <= 100000; n++ {
		p := th.penalty(n)
		if p <= 0 {
			t.Fatalf("penalty(%d) = %v; a non-positive duration reads as not-blocked and switches the throttle off", n, p)
		}
		if p > th.PenaltyCap {
			t.Fatalf("penalty(%d) = %v, above the cap %v", n, p, th.PenaltyCap)
		}
	}
	// The escalation itself has to be real, or a cap that fires at n=2 would
	// satisfy everything above.
	if th.penalty(1) != time.Second || th.penalty(2) != 2*time.Second || th.penalty(3) != 4*time.Second {
		t.Errorf("penalty does not double: 1s->%v 2s->%v 3s->%v", th.penalty(1), th.penalty(2), th.penalty(3))
	}
	if th.penalty(6) != th.PenaltyCap {
		t.Errorf("penalty(6) = %v, want the cap %v", th.penalty(6), th.PenaltyCap)
	}
}

func TestEntriesExpireAndArePurged(t *testing.T) {
	th, clock := newTestThrottle()

	spend(th, clock, "host", 10)
	if ok, _ := th.Allow("host"); ok {
		t.Fatal("expected the host to be blocked")
	}
	clock.Advance(th.Window + time.Second)
	if ok, wait := th.Allow("host"); !ok {
		t.Errorf("still blocked a full window later (wait %v)", wait)
	}
	th.mu.Lock()
	n := len(th.keys)
	th.mu.Unlock()
	if n != 0 {
		t.Errorf("%d key entries survive a full window; they are meant to be purged on access", n)
	}
}

// The rule that keeps a flood from becoming a permanent lockout: the global
// cooldown is a fixed window from the moment it trips, and a failure arriving
// during it does not extend it.
func TestTheGlobalCooldownIsNotExtendedByAContinuingFlood(t *testing.T) {
	th, clock := newTestThrottle()
	th.GlobalLimit = 20
	th.GlobalCooldown = 5 * time.Minute

	for i := 0; i < 20; i++ {
		th.Failure(fmt.Sprintf("10.0.0.%d", i))
	}
	ok, wait := th.Allow("the-real-parent")
	if ok {
		t.Fatal("the global cooldown did not apply to an untouched key")
	}
	if wait <= 0 || wait > th.GlobalCooldown {
		t.Errorf("wait = %v, want at most %v", wait, th.GlobalCooldown)
	}

	before := wait

	// The flood keeps coming for the whole cooldown.
	for i := 0; i < 100; i++ {
		clock.Advance(time.Second)
		th.Failure(fmt.Sprintf("172.16.%d.%d", i/256, i%256))
		if ok, wait := th.Allow("the-real-parent"); ok {
			t.Fatal("the cooldown lifted early")
		} else if wait > before {
			t.Fatalf("the cooldown grew from %v to %v while the flood continued", before, wait)
		}
	}
	// The assertion that actually pins the rule, and the one the first version of
	// this test was missing. A re-arming cooldown does not make `wait` grow — it
	// stays at exactly one cooldown from the newest failure, so comparing against
	// the initial wait sees nothing. What it does do is move the *end*, so the
	// test has to measure from the trip: the flood has been running for 100 more
	// seconds, and a full cooldown after the trip must still be over.
	clock.Advance(th.GlobalCooldown - 100*time.Second)
	if ok, wait := th.Allow("the-real-parent"); !ok {
		t.Errorf("still locked out %v after the trip, though the cooldown is a fixed %v from that trip (wait %v)",
			th.GlobalCooldown, th.GlobalCooldown, wait)
	}
}

func TestTheGlobalLimitDoesNotFireOnOrdinaryFailures(t *testing.T) {
	th, _ := newTestThrottle()
	for i := 0; i < 19; i++ {
		th.Failure(fmt.Sprintf("10.0.0.%d", i))
	}
	if ok, wait := th.Allow("somebody-else"); !ok {
		t.Errorf("an untouched key was blocked by %d failures elsewhere (wait %v)", 19, wait)
	}
}

func TestResetClearsTheCooldown(t *testing.T) {
	th, _ := newTestThrottle()
	th.GlobalLimit = 2
	th.Failure("a")
	th.Failure("b")
	if ok, _ := th.Allow("c"); ok {
		t.Fatal("expected the global cooldown")
	}
	th.Reset()
	if ok, wait := th.Allow("c"); !ok {
		t.Errorf("Allow after Reset = (false, %v)", wait)
	}
}

func TestThrottleIsConcurrencySafe(t *testing.T) {
	th, _ := newTestThrottle()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("host-%d", i%3)
			for j := 0; j < 60; j++ {
				if ok, _ := th.Allow(key); ok {
					th.Failure(key)
				}
			}
			th.Success(key)
		}(i)
	}
	wg.Wait()
}

// Allow must answer without doing the expensive thing. This asserts the ordering
// the type documents: a blocked key is refused before any hashing would happen,
// so the throttle is also the KDF's denial-of-service guard.
func TestABlockedKeyIsRefusedWithoutAnyVerificationWork(t *testing.T) {
	th, clock := newTestThrottle()
	spend(th, clock, "host", 10)
	// There is nothing to assert about a callback here because Allow takes no
	// password at all — which is the property. What can be asserted is that the
	// handler has everything it needs to answer without one.
	ok, wait := th.Allow("host")
	if ok || wait <= 0 {
		t.Fatalf("Allow = (%v, %v), want a refusal with a usable retry time", ok, wait)
	}
}

// A zero-value Throttle must throttle, not panic.
//
// The panic is the interesting failure: it is not a refused login, it is the
// daemon going down, and a daemon that is down stops re-locking. Every knob here
// therefore has a default that is the documented policy rather than a zero.
func TestTheZeroValueIsAWorkingThrottle(t *testing.T) {
	th := &Throttle{}
	if ok, wait := th.Allow("host"); !ok {
		t.Errorf("a fresh zero-value throttle refused the first attempt (wait %v)", wait)
	}
	for i := 0; i < th.limit(); i++ {
		th.Failure("host")
	}
	if ok, wait := th.Allow("host"); ok || wait <= 0 {
		t.Errorf("zero-value throttle after %d failures = (ok %v, wait %v)", th.limit(), ok, wait)
	}
	if th.penalty(500) != th.penaltyCap() {
		t.Errorf("zero-value penalty = %v, want the cap", th.penalty(500))
	}
}
