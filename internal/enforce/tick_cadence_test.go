package enforce

import (
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// The startup line and the first timer must agree, and for one review round they did not.
// The line was fixed to read the config, but the loop still armed its first timer from
// interval() before any reload had published pollSeconds -- so a config asking for a 1s
// tick announced 1s and then sat 30s before its second tick. Measured on the dev loop as
// two lock calls exactly 30s apart on a machine whose config said 5.
//
// Announcing a number and running a different one is worse than announcing nothing: the
// operator has a figure to trust and it is wrong. So this asserts the cadence itself, not
// the sentence about it -- a test on the log line alone passed while the bug was live,
// which is how it survived the previous fix.
func TestTheFirstTickWaitsTheConfiguredPeriod(t *testing.T) {
	doc := kidDoc()
	// 1s, below MinPollSeconds. Load does not run Validate -- the gate belongs to
	// whoever produced the document -- so a test may use a tick short enough to observe
	// twice inside a second and a half.
	doc.Settings.PollSeconds = 1
	e, p, _, _, _ := newEngine(t, doc, refInside)
	// No Interval override: e.Interval is the seam that outranks the config, and the
	// whole point is the period the config chooses.
	e.WatchInterval = -1

	startRun(t, e)
	// Two ticks at 1s, with room for a slow machine. Before the fix the second tick was
	// 30s out, so this deadline is nowhere near tight enough to flake and far past the
	// point where the bug is unmissable.
	waitFor(t, "a second tick at the configured period", func() bool { return p.totalLocks() >= 2 })

	if n := p.totalLocks(); n < 2 {
		t.Errorf("only %d tick(s) ran in the wait: the first timer is not using the configured "+
			"1s period, so the daemon announces one cadence and keeps another", n)
	}
}

// The same thing seen from the other side: with no pollSeconds in the document, the first
// timer must be the documented default rather than something shorter, or the fix would
// pass by arming a zero timer that fires immediately.
func TestTheFirstTickWaitsTheDefaultWhenNothingIsConfigured(t *testing.T) {
	doc := kidDoc()
	doc.Settings.PollSeconds = 0
	e, p, _, _, _ := newEngine(t, doc, refInside)
	e.WatchInterval = -1

	startRun(t, e)
	// The first tick is immediate, so exactly one is expected; a second would mean the
	// timer armed at 0 and the loop is spinning.
	waitFor(t, "the first tick", func() bool { return p.totalLocks() >= 1 })
	time.Sleep(400 * time.Millisecond)
	if n := p.totalLocks(); n != 1 {
		t.Errorf("%d ticks in 400ms with no pollSeconds configured; the first timer should be the "+
			"30s default, not zero -- a zero timer makes the loop spin and re-lock as fast as it can", n)
	}
}

// interval() is the only thing standing between a bad published number and
// time.NewTimer, and NewTimer fires immediately for a duration that is not positive. So
// the guard that matters is interval()'s own `s > 0`, not the `period > 0` at the publish
// site: with the publish guard removed, a negative sentinel still lands on the atomic and
// interval() is what refuses it. Asserted directly, because a loop-level test cannot see
// this -- the second guard hides the first, which is the masking pattern that has cost
// this repo batteries before.
func TestIntervalRefusesANonPositivePublishedPeriod(t *testing.T) {
	doc := kidDoc()
	doc.Settings.PollSeconds = 9
	e, _, _, _, _ := newEngine(t, doc, refInside)

	// New() does not read the config -- only reload publishes pollSeconds -- so the
	// published value has to be put there the way Run would put it. Asserting 9s before
	// this would only prove the atomic starts at zero.
	e.pollSeconds.Store(9)
	if got := e.interval(); got != 9*time.Second {
		t.Fatalf("interval() = %v after a 9s publish, want 9s", got)
	}
	for _, bad := range []int64{0, -1, -30} {
		e.pollSeconds.Store(bad)
		if got := e.interval(); got != config.DefaultPollSeconds*time.Second {
			t.Errorf("interval() = %v with pollSeconds %d; a non-positive published value must fall "+
				"back to the documented default, because time.NewTimer fires immediately for a "+
				"non-positive duration and the enforcement loop would spin", got, bad)
		}
	}
}

// The startup line and the first timer agree because the same number is published before
// the loop starts. Pinned as a property rather than left to the two code paths happening
// to match: whatever the line announces is what interval() returns from the first
// iteration onward.
func TestTheAnnouncedPeriodIsThePublishedPeriod(t *testing.T) {
	doc := kidDoc()
	doc.Settings.PollSeconds = 11
	e, _, _, _, _ := newEngine(t, doc, refInside)

	announced := e.startupPeriod()
	e.pollSeconds.Store(int64(announced / time.Second))
	if got := e.interval(); got != announced {
		t.Errorf("the line would announce %v while the loop runs at %v; announcing one cadence "+
			"and keeping another is the defect this pins", announced, got)
	}
}
