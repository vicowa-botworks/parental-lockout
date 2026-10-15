package platform

import (
	"context"
	"fmt"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// The two failure shapes in docs/platforms.md §4 are quoted as subprocess counts, and
// those numbers are the entire argument for `DefaultLockBudget`. They were measured by
// hand once, which means a change to the chain can make them wrong in silence — and a
// doc that overstates or understates the cost of a stuck session is the same class of
// defect as a comment that promises a budget the code does not implement.
//
// So the measurement lives here.

// shapeRunner is the session bus and loginctl of a machine where the lock never lands,
// counting every command the chain issues.
type shapeRunner struct {
	// hangGdbus models a session bus that owns org.kde.screensaver and never replies.
	// False models a desktop that accepts the lock and never writes the hint.
	hangGdbus bool
	// callTimeout is what this fake waits before giving up on a hung bus. The real
	// ExecRunner applies DefaultCallTimeout itself, and a fake *is* the runner, so the
	// bound has to be modelled here or a hung bus hangs forever.
	callTimeout time.Duration

	mu     sync.Mutex
	counts map[string]int
}

func (r *shapeRunner) Run(ctx context.Context, argv []string, _ []string, _ uint32) ([]byte, error) {
	// Keyed by basename on purpose. The backend resolves gdbus through PATH and execs the
	// absolute path it found, so argv[0] is /usr/bin/gdbus on a real host and whatever the
	// fixture's PATH yields here. The quantity this test exists to pin is how many times a
	// Chain.Lock invokes the tool, which does not depend on how the invocation is spelled.
	prog := path.Base(argv[0])
	r.mu.Lock()
	if r.counts == nil {
		r.counts = map[string]int{}
	}
	r.counts[prog]++
	r.mu.Unlock()

	switch prog {
	case "gdbus":
		if r.hangGdbus {
			tctx, cancel := context.WithTimeout(ctx, r.callTimeout)
			defer cancel()
			<-tctx.Done()
			return nil, fmt.Errorf("%s: %w", argv[2], context.DeadlineExceeded)
		}
		return []byte("()\n"), nil
	case "loginctl":
		if sub(argv, 0) == "show-session" {
			// The hint never moves: this is the shape where the desktop never writes
			// it, which is the open question in docs/platforms.md §4.
			return []byte("LockedHint=no\n"), nil
		}
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected program %q", prog)
}

func (r *shapeRunner) snapshot() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]int, len(r.counts))
	for k, v := range r.counts {
		out[k] = v
	}
	return out
}

// measureShape runs one Chain.Lock over the shipped default chain and returns what it
// cost.
//
// The two bounds are deliberately moved out of the way. The count being documented is how
// many commands the chain *would* issue, so the operation budget is widened rather than
// allowed to truncate the walk, and the per-command timeout is shrunk so a bus that never
// answers costs a millisecond instead of ten seconds. Neither changes how many commands
// there are; they only decide whether the measurement finishes before the reviewer's
// coffee does.
func measureShape(t *testing.T, hangGdbus bool) map[string]int {
	t.Helper()
	r := &shapeRunner{hangGdbus: hangGdbus, callTimeout: time.Millisecond}
	c, err := New(config.Settings{LockBackends: []string{"logind", "kde", "gnome"}}, r)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.LockBudget = time.Hour

	if err := c.Lock(context.Background(), desktopSession("3")); err == nil {
		t.Fatal("a lock nobody ever confirmed reported success")
	}
	return r.snapshot()
}

func sumShape(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

// The counts are asserted, not printed. A test that only reports them would let the docs
// drift exactly as far as nobody happened to notice.
//
// The pre-read Chain.Lock now takes before asking anybody (see the #29 discounting in
// Lock) is one loginctl per operation, and it is inside these numbers.
func TestDocumentedCallCountsPerChainLock(t *testing.T) {
	for _, tc := range []struct {
		name      string
		hangGdbus bool
		wantGdbus int
		wantTotal int
	}{
		{"bus owns the name and never answers", true, 5, 18},
		{"desktop accepts the lock and never writes the hint", false, 2, 37},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := measureShape(t, tc.hangGdbus)
			total := sumShape(got)
			if got["gdbus"] != tc.wantGdbus {
				t.Errorf("gdbus called %d time(s), want %d — docs/platforms.md §4 says %d+%d",
					got["gdbus"], tc.wantGdbus, tc.wantGdbus, tc.wantTotal-tc.wantGdbus)
			}
			if total != tc.wantTotal {
				t.Errorf("one Chain.Lock issued %d subprocess calls, want %d — docs/platforms.md §4 "+
					"quotes %d (%d gdbus + %d loginctl). Update the table with the measurement, not the other way round.",
					total, tc.wantTotal, tc.wantTotal, tc.wantGdbus, tc.wantTotal-tc.wantGdbus)
			}
			if other := total - got["gdbus"] - got["loginctl"]; other != 0 {
				t.Errorf("%d call(s) went to a program other than gdbus or loginctl: %v", other, got)
			}
		})
	}
}
