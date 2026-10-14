package enforce

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// This file is the test rig: a platform that records what it was asked to do, a
// sink that records what was said, a clock nobody has to wait for, and a config
// fixture that reads like a parent's file rather than a fixture.
//
// Every one of them is safe for concurrent use, because the -race criterion in #7
// is about the loop and the reloader running together. A fake with an unguarded
// field would either flake or, worse, make a real race in the engine invisible by
// hiding inside the harness.

// refNoon and refNight are the two instants the fixtures are built around: the same
// UTC day, twelve hours apart, one inside the curfew and one outside it. Both are
// fixed rather than "now" so a test that runs at 03:00 local says the same thing as
// one that runs at 15:00.
var (
	refOutside = time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)  // Tuesday midday
	refInside  = time.Date(2026, 3, 3, 22, 30, 0, 0, time.UTC) // Tuesday night, curfew open
)

// curfew is the rule every fixture uses: every day, 20:00 to 06:30 the next morning.
// It crosses midnight on purpose — that is the shape a real bedtime rule has, and a
// fixture that only tested same-day windows would leave the interesting half of the
// evaluator unexercised.
func curfew() config.Rule {
	return config.Rule{
		ID: "r-curfew", Name: "School week",
		Days:  []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"},
		Start: "20:00", End: "06:30", Enabled: true,
	}
}

// kidDoc is the document the tests start from: enforcement on, one profile, one
// managed account, and a rule that is open at refInside and closed at refOutside.
func kidDoc() *config.Config {
	return &config.Config{
		Version:  config.CurrentVersion,
		Settings: config.Settings{PollSeconds: 30, Enforcement: true},
		Profiles: []config.Profile{{
			ID: "p-default", Name: "Default", Timezone: "UTC",
			Rules: []config.Rule{curfew()},
		}},
		Users: []config.User{{Username: "kid", ProfileID: "p-default", Enabled: true}},
	}
}

// writeDoc marshals a document to path. It goes through the real encoder rather
// than a JSON literal so that a change to a config tag breaks these tests loudly
// instead of leaving them asserting against a document the loader no longer reads.
func writeDoc(t *testing.T, path string, c *config.Config) {
	t.Helper()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// newEngine builds an engine over a temp config file, a fake platform and a
// recording sink, wired to a fake clock parked at at.
func newEngine(t *testing.T, doc *config.Config, at time.Time) (*Engine, *fakePlatform, *recSink, *fakeClock, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	writeDoc(t, path, doc)

	p := &fakePlatform{sessions: []platform.Session{desktop("2", "kid")}}
	sink := &recSink{}
	clock := &fakeClock{t: at}

	e := New(path, p, sink)
	e.Clock = clock.Now
	return e, p, sink, clock, path
}

// desktop is a session the platform would agree to lock: class user, a seat, a real
// session type, not remote, not closing. Tests build through it rather than spelling
// the fields out, because platform.IsLockable refuses a bare Session{Type:...} and a
// fixture that is not lockable tests the wrong branch.
func desktop(id, user string) platform.Session {
	return platform.Session{
		ID: id, Username: user, UID: 1001,
		Class: "user", Seat: "seat0", Type: "wayland", State: "active",
	}
}

// fakePlatform is a platform.Platform that records every request.
type fakePlatform struct {
	mu sync.Mutex

	sessions    []platform.Session
	sessionsErr error

	// lockErr is per session, because the interesting failures are "this session
	// cannot be locked" rather than "the platform is down".
	lockErr map[string]error

	// block, when set, holds every Lock call until it is closed. It exists for the
	// overlap test, which needs a tick that is provably still running.
	block chan struct{}

	// slow makes every Lock take this long. The race test needs a tick that is
	// still holding the engine's state at the moment another goroutine reads it;
	// a tick that finishes in nanoseconds is a race that never happens, and a
	// guard that is never observed racing is a guard nobody has tested.
	slow time.Duration

	// entered is closed on the first Lock call so a test can wait for a tick to be
	// demonstrably inside the platform rather than sleeping.
	entered chan struct{}
	once    sync.Once

	locks []string
	calls int
}

func (f *fakePlatform) Name() string { return "fake" }

func (f *fakePlatform) Sessions(context.Context) ([]platform.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.sessionsErr != nil {
		return nil, f.sessionsErr
	}
	return append([]platform.Session(nil), f.sessions...), nil
}

func (f *fakePlatform) Lock(ctx context.Context, s platform.Session) error {
	f.mu.Lock()
	f.locks = append(f.locks, s.ID)
	block := f.block
	slow := f.slow
	err := f.lockErr[s.ID]
	entered := f.entered
	f.mu.Unlock()

	if entered != nil {
		f.once.Do(func() { close(entered) })
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return fmt.Errorf("fake platform: still blocked after 5s")
		}
	}
	if slow > 0 {
		select {
		case <-time.After(slow):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

func (f *fakePlatform) Probe(context.Context) []platform.BackendStatus { return nil }

func (f *fakePlatform) lockCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, got := range f.locks {
		if got == id {
			n++
		}
	}
	return n
}

func (f *fakePlatform) totalLocks() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.locks)
}

func (f *fakePlatform) setSessions(s ...platform.Session) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions = s
}

func (f *fakePlatform) setSessionsErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessionsErr = err
}

func (f *fakePlatform) setLockErr(id string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lockErr == nil {
		f.lockErr = map[string]error{}
	}
	f.lockErr[id] = err
}

// gateEntered arms the signal that closes the first time Lock is called, so a test
// can wait for a tick to be demonstrably inside the platform instead of sleeping and
// hoping.
func (f *fakePlatform) gateEntered() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entered = make(chan struct{})
}

// recSink is a Sink that keeps everything.
type recSink struct {
	mu     sync.Mutex
	events []Event
}

func (s *recSink) Emit(ev Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
}

func (s *recSink) all() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Event(nil), s.events...)
}

// countOf counts the recorded events of one kind. The acceptance criteria ask for
// events "asserted by count and by payload", so this is the count half and kindsOf
// is the shape of a whole run.
func (s *recSink) countOf(kind string) int {
	n := 0
	for _, ev := range s.all() {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

func (s *recSink) kindsOf(kind string) []Event {
	var out []Event
	for _, ev := range s.all() {
		if ev.Kind == kind {
			out = append(out, ev)
		}
	}
	return out
}

func (s *recSink) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = nil
}

// fakeClock is a clock a test drives.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// tick is shorthand for a tick whose report is checked for nothing but returned.
func tick(e *Engine) Report { return e.Tick(context.Background()) }

// mustEvent fails the test unless exactly n events of kind were recorded.
func mustCount(t *testing.T, s *recSink, kind string, n int) {
	t.Helper()
	if got := s.countOf(kind); got != n {
		t.Errorf("%s emitted %d time(s), want %d; events: %s", kind, got, n, kindsList(s.all()))
	}
}

func kindsList(evs []Event) string {
	out := ""
	for _, ev := range evs {
		out += fmt.Sprintf("[%s %s %s] ", ev.Kind, ev.Username, ev.Detail)
	}
	if out == "" {
		return "(none)"
	}
	return out
}
