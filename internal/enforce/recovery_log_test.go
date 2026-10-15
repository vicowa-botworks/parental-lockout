package enforce

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// logCapture collects the engine's human-readable diagnostics. The engine's Logf is a plain
// func(string, ...any), so a test that wants to assert on what the operator would read has to
// format the same way the daemon does rather than capturing the standard logger.
type logCapture struct {
	mu    sync.Mutex
	lines []string
}

func (c *logCapture) printf(format string, args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, strings.TrimSpace(fmt.Sprintf(format, args...)))
}

func (c *logCapture) count(sub string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	var n int
	for _, l := range c.lines {
		if strings.Contains(l, sub) {
			n++
		}
	}
	return n
}

func (c *logCapture) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.lines...)
}

// TestAHealthyConfigNeverReportsRecovery is what the real-hardware run prompted.
//
// The end-to-end transcript's daemon log carried "enforce: config readable again" on a machine
// whose config had been readable since before the daemon started, with no stale period to end.
// The line marks the END of a degraded stretch -- its own comment says the operator saw
// KindConfigStale and should see it stop -- so printing it when there was no stale period puts a
// false statement in the one place the operator looks to learn whether anything is wrong, and it
// repeats on every tick for the life of the process.
func TestAHealthyConfigNeverReportsRecovery(t *testing.T) {
	e, _, _, _, _ := newEngine(t, kidDoc(), time.Unix(1700000000, 0))
	capture := &logCapture{}
	e.Logf = capture.printf

	for i := 0; i < 5; i++ {
		if _, err := e.reload(e.Clock(), &Report{}); err != nil {
			t.Fatalf("reload %d: %v", i, err)
		}
	}
	if got := capture.count("readable again"); got != 0 {
		t.Errorf("five clean reloads reported recovery %d times; the line belongs to the end of "+
			"a stale period and this config was readable the whole time\nlog: %v", got, capture.all())
	}
}
