package enforce

import (
	"os"
	"strings"
	"testing"
	"time"
)

// The startup line is the only place the daemon states its tick period, and its stated
// purpose is to tell an operator watching a screen that will not lock whether they are
// waiting 5s or 5m. It used to print e.interval() before anything had published
// pollSeconds -- the first reload happens inside the first tick -- so it printed the
// compiled default whatever the config said. Measured on the dev loop with
// `settings.pollSeconds: 5`: the log said "every 30s" for the whole run.
//
// A number in a startup line is a claim the operator acts on, so the assertion is the
// configured number and, just as importantly, NOT the default. Asserting only "contains
// 7s" would pass on a build that printed both.
func TestTheStartupLineReportsTheConfiguredPeriod(t *testing.T) {
	doc := kidDoc()
	doc.Settings.PollSeconds = 7
	e, _, _, _, _ := newEngine(t, doc, refInside)
	log := &logCapture{}
	e.Logf = log.printf
	// No Interval override: the point of this test is the period the config chooses, and
	// e.Interval is the test seam that outranks it.
	e.WatchInterval = -1

	startRun(t, e)
	waitFor(t, "the startup line", func() bool { return log.count("enforce: running") > 0 })

	got := ""
	for _, l := range log.all() {
		if strings.Contains(l, "enforce: running") {
			got = l
			break
		}
	}
	if !strings.Contains(got, "every 7s") {
		t.Errorf("startup line = %q, want it to report the configured 7s period", got)
	}
	if strings.Contains(got, "30s") {
		t.Errorf("startup line = %q; it is reporting the compiled default, which is the defect "+
			"this test exists to keep out -- the operator would wait half a minute for a tick "+
			"that was due in seven seconds", got)
	}
}

// The same line with the default in force must still be truthful, or the fix would just
// have moved the lie to the other branch.
func TestTheStartupLineReportsTheDefaultWhenTheConfigOmitsIt(t *testing.T) {
	doc := kidDoc()
	doc.Settings.PollSeconds = 0
	e, _, _, _, _ := newEngine(t, doc, refInside)
	log := &logCapture{}
	e.Logf = log.printf
	e.WatchInterval = -1

	startRun(t, e)
	waitFor(t, "the startup line", func() bool { return log.count("enforce: running") > 0 })

	if log.count("every 30s") == 0 {
		t.Errorf("a config with no pollSeconds should report the documented default; got %v", log.all())
	}
}

// And when the config cannot be read at all, the line must not present a default as a
// measurement. The first tick is about to report the load failure; the startup line's job
// is not to contradict it with a number nobody configured.
func TestTheStartupLineSaysSoWhenTheConfigIsUnreadable(t *testing.T) {
	doc := kidDoc()
	e, _, _, _, path := newEngine(t, doc, refInside)
	log := &logCapture{}
	e.Logf = log.printf
	e.WatchInterval = -1

	// Removed after the engine was built, so the path is exactly the "readable when the
	// daemon was configured, gone by the time it started" shape an operator actually hits.
	if err := os.Remove(path); err != nil {
		t.Fatalf("removing the config: %v", err)
	}

	startRun(t, e)
	waitFor(t, "the startup line", func() bool { return log.count("enforce: running") > 0 })

	if log.count("period not yet known") == 0 {
		t.Errorf("an unreadable config still produced a period; got %v", log.all())
	}
	for _, l := range log.all() {
		if strings.Contains(l, "enforce: running") && strings.Contains(l, "every ") {
			t.Errorf("the startup line invented a period for a config it could not read: %q", l)
		}
	}
}

// startupPeriod is what the line reads, and its negative return is the whole contract of
// the unreadable branch. Pinned directly because the log-line tests above could be
// satisfied by a different mechanism while this one quietly returned 0.
func TestStartupPeriodDistinguishesUnreadableFromZero(t *testing.T) {
	doc := kidDoc()
	doc.Settings.PollSeconds = 11
	e, _, _, _, path := newEngine(t, doc, refInside)

	if got := e.startupPeriod(); got != 11*time.Second {
		t.Errorf("startupPeriod() = %v, want 11s", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("removing the config: %v", err)
	}
	if got := e.startupPeriod(); got >= 0 {
		t.Errorf("startupPeriod() = %v for an unreadable config, want a negative value so the "+
			"caller says so instead of quoting a default", got)
	}
}
