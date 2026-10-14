package platform

import (
	"context"
	"sync"
)

// Mock is a scripted Platform for tests.
//
// It is deliberately not registered under an id. A backend named "mock" that the
// registry would happily build from a config is a way for lockBackends to end up
// pointing at something that reports success and locks nothing — the exact shape of
// failure this daemon cannot afford, because it looks like it is working. Tests
// construct it directly and hand it to whatever takes a Platform.
type Mock struct {
	// BackendName is what Name reports. Empty means "mock".
	BackendName string

	// Listed is what Sessions returns, verbatim, before any filtering. Mock applies
	// no IsLockable filter: a test that wants to know the loop ignores a remote
	// session should have to say so, not get the answer from the fixture.
	Listed []Session
	// ListErr is returned instead of listing anything.
	ListErr error

	// LockErr is returned by every Lock call when set. LockErrBySession overrides it
	// for one session, which is what a fallback-chain test needs: backend A fails on
	// this session, backend B succeeds.
	LockErr          error
	LockErrBySession map[string]error

	mu     sync.Mutex
	locked []Session
	locks  int
	probe  []BackendStatus
}

var _ Platform = (*Mock)(nil)

func (m *Mock) Name() string {
	if m.BackendName == "" {
		return "mock"
	}
	return m.BackendName
}

func (m *Mock) Sessions(ctx context.Context) ([]Session, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}
	// A copy, so a caller that sorts or truncates what it got back cannot rewrite the
	// script the next call is supposed to return.
	return append([]Session(nil), m.Listed...), nil
}

func (m *Mock) Lock(ctx context.Context, s Session) error {
	m.mu.Lock()
	m.locks++
	m.locked = append(m.locked, s)
	m.mu.Unlock()

	if err, ok := m.LockErrBySession[s.ID]; ok {
		return err
	}
	if m.LockErr != nil {
		return m.LockErr
	}
	return nil
}

func (m *Mock) Probe(ctx context.Context) []BackendStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.probe != nil {
		return append([]BackendStatus(nil), m.probe...)
	}
	return []BackendStatus{{Name: m.Name(), Available: true, Detail: "mock"}}
}

// SetProbe scripts what Probe reports, for a test that needs a backend to look
// unavailable without making it fail at lock time.
func (m *Mock) SetProbe(s ...BackendStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.probe = append([]BackendStatus(nil), s...)
}

// Locked lists the sessions Lock was called with, in call order.
func (m *Mock) Locked() []Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Session(nil), m.locked...)
}

// LockedIDs is the same thing reduced to what an assertion usually wants.
func (m *Mock) LockedIDs() []string {
	ss := m.Locked()
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.ID)
	}
	return out
}

// LockCount counts every call, including the ones that returned an error — a
// fallback chain that tries and fails three times should read as three attempts, not
// one.
func (m *Mock) LockCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.locks
}

// Reset clears the record without touching the script.
func (m *Mock) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.locked = nil
	m.locks = 0
}
