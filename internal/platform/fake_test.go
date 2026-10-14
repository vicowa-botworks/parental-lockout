package platform

import (
	"context"
	"reflect"
	"sync"
	"testing"
)

// call is one recorded Run.
type call struct {
	argv []string
	env  []string
	uid  uint32
}

// fakeRunner is the only way these tests see what a backend asked for.
//
// The ticket's rule is that no test executes a real binary, and the reason is not
// tidiness: the assertions that matter are the exact argv and env a lock produces,
// and the only way to check them against a real loginctl is to actually lock a
// screen.
type fakeRunner struct {
	mu      sync.Mutex
	calls   []call
	respond func(argv []string) ([]byte, error)
}

func (f *fakeRunner) Run(ctx context.Context, argv []string, env []string, uid uint32) ([]byte, error) {
	f.mu.Lock()
	// Copy what was asked for. The caller owns the slice it passed in, and a backend
	// that reuses a scratch argv would otherwise rewrite its own history — the same
	// sharing bug the config package's edit helpers had, arriving from the other side.
	rec := call{
		argv: append([]string(nil), argv...),
		uid:  uid,
	}
	if env != nil {
		rec.env = append([]string(nil), env...)
	}
	f.calls = append(f.calls, rec)
	f.mu.Unlock()

	if f.respond == nil {
		return nil, nil
	}
	return f.respond(argv)
}

func (f *fakeRunner) Calls() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]call(nil), f.calls...)
}

// Argvs is what an argv assertion usually wants: the commands, in order.
func (f *fakeRunner) Argvs() [][]string {
	var out [][]string
	for _, c := range f.Calls() {
		out = append(out, c.argv)
	}
	return out
}

// wantArgv compares the recorded commands against want and reports the first
// difference with both sides printed in full, because a wrong argv is usually one
// element off in a way that a diff of quoted slices hides.
func wantArgv(t testing.TB, got, want [][]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%d command(s) run, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
		return
	}
	for i := range got {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Errorf("command %d:\n got %v\nwant %v", i, got[i], want[i])
		}
	}
}

// desktopSession is the shape logind reports for a real desktop session. Tests build
// through it rather than spelling the fields out, because IsLockable requires Class and
// Seat and a bare Session{Type: "x11"} is now correctly refused — a session that never
// came from a listing is not something to hand to lock-session.
func desktopSession(id string) Session {
	return Session{
		ID: id, Username: "kid", UID: 1001,
		Class: "user", Seat: "seat0", Type: "x11", State: "active",
	}
}

// confirming makes a Mock also satisfy Confirmer.
//
// Mock itself deliberately does NOT implement Confirmer. That is not an oversight: the
// chain has a branch for "a backend accepted the request and cannot prove the screen is
// locked", and if the shared test double always confirmed, that branch would never be
// exercised by anything.
type confirming struct {
	*Mock
	yes bool
	err error
}

func (c confirming) LockConfirmed(context.Context, Session) (bool, error) { return c.yes, c.err }

// backendAt takes one backend out of the chain New builds, for tests that need to
// exercise a backend's optional interfaces. The chain is what callers hold, so this is
// the honest route to the thing underneath it.
func backendAt(t *testing.T, c *Chain, i int) Platform {
	t.Helper()
	if i >= len(c.backends) {
		t.Fatalf("chain has %d backend(s), wanted index %d", len(c.backends), i)
	}
	return c.backends[i]
}
