package auth

import (
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClock is the whole reason the expiry rules are testable: a 12-hour window
// cannot be waited out, and a test that asserted on "some time in the future"
// would pass whether the window was 12 hours or 12 nanoseconds.
type fakeClock struct {
	mu sync.Mutex
	at time.Time
}

func newClock() *fakeClock {
	return &fakeClock{at: time.Date(2026, 10, 14, 23, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

func newTestSessions(t *testing.T) (*Sessions, *fakeClock) {
	t.Helper()
	clock := newClock()
	s := NewSessions().WithTTL(time.Hour).WithMax(4)
	s.now = clock.Now
	return s, clock
}

func TestStartLookupRevoke(t *testing.T) {
	s, _ := newTestSessions(t)

	sess, err := s.Start("parent")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if sess.Username != "parent" || sess.Token == "" {
		t.Fatalf("Start = %+v", sess)
	}
	if sess.Token == sess.Username {
		t.Fatal("the token is the username")
	}
	got, ok := s.Lookup(sess.Token)
	if !ok || got.Username != "parent" {
		t.Fatalf("Lookup = (%+v, %v)", got, ok)
	}

	s.Revoke(sess.Token)
	if _, ok := s.Lookup(sess.Token); ok {
		t.Fatal("a revoked token still resolves")
	}
}

// A token is 256 bits of crypto/rand, so "forged" means guessed. The store must
// not resolve anything it did not issue, and must not distinguish between "never
// existed" and "expired" in a way a caller could turn into an oracle.
func TestAForgedTokenIsAMiss(t *testing.T) {
	s, _ := newTestSessions(t)
	issued, _ := s.Start("parent")

	for _, forged := range []string{
		"",
		" ",
		issued.Token[:len(issued.Token)-1],
		issued.Token + "A",
		strings.Repeat("A", len(issued.Token)),
		"null",
		"*",
	} {
		if _, ok := s.Lookup(forged); ok {
			t.Errorf("Lookup(%q) resolved a token that was never issued", forged)
		}
	}
}

func TestExpiryIsEnforcedOnLookup(t *testing.T) {
	s, clock := newTestSessions(t)
	sess, _ := s.Start("parent")

	clock.Advance(time.Hour - time.Millisecond)
	if _, ok := s.Lookup(sess.Token); !ok {
		t.Fatal("a session inside its window was refused")
	}
	// The lookup above slid the window, so an hour after *that* it is still live.
	clock.Advance(time.Hour - time.Millisecond)
	if _, ok := s.Lookup(sess.Token); !ok {
		t.Fatal("a session used within its window must not expire")
	}
	clock.Advance(time.Hour)
	if _, ok := s.Lookup(sess.Token); ok {
		t.Fatal("an expired session still resolved")
	}
	// Count() is what the status command reports, and it must not lean on Lookup
	// having already deleted the entry: the expiry sweep inside Count is the only
	// thing that runs when nobody presents a token.
	if n := s.Count(); n != 0 {
		t.Errorf("Count = %d after the only session expired, want 0", n)
	}
}

// An expired session must stop being counted even if nobody ever presents the
// token again.
//
// The test above cannot catch this on its own: its Lookup deletes the entry on the
// way to failing, so Count is left with an empty map whether or not the sweep
// works. This one goes straight to Count.
func TestExpiredSessionsAreSweptWithoutBeingLookedUp(t *testing.T) {
	s, clock := newTestSessions(t)
	for i := 0; i < 3; i++ {
		if _, err := s.Start("parent"); err != nil {
			t.Fatal(err)
		}
		clock.Advance(time.Minute)
	}
	if n := s.Count(); n != 3 {
		t.Fatalf("Count = %d, want the 3 live sessions", n)
	}
	clock.Advance(time.Hour)
	if n := s.Count(); n != 0 {
		t.Errorf("Count = %d with every session expired and none looked up, want 0", n)
	}
}

// Sliding expiry has to be visible as a change in the deadline, not merely as
// "still valid": a store that returned true but never moved ExpiresAt would pass
// every test above and still log a parent out mid-task.
func TestLookupSlidesTheDeadline(t *testing.T) {
	s, clock := newTestSessions(t)
	sess, _ := s.Start("parent")
	first := sess.ExpiresAt

	clock.Advance(30 * time.Minute)
	got, ok := s.Lookup(sess.Token)
	if !ok {
		t.Fatal("lookup inside the window")
	}
	if !got.ExpiresAt.After(first) {
		t.Errorf("ExpiresAt did not move: %v then %v", first, got.ExpiresAt)
	}
	// And the store agrees, not just the returned copy.
	clock.Advance(45 * time.Minute)
	if _, ok := s.Lookup(sess.Token); !ok {
		t.Error("the store did not record the slid deadline")
	}
}

func TestRevokeAllEndsEverything(t *testing.T) {
	s, _ := newTestSessions(t)
	var tokens []string
	for i := 0; i < 3; i++ {
		sess, err := s.Start("parent")
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, sess.Token)
	}
	s.RevokeAll()
	for _, tok := range tokens {
		if _, ok := s.Lookup(tok); ok {
			t.Error("a session survived RevokeAll")
		}
	}
	if n := s.Count(); n != 0 {
		t.Errorf("Count = %d after RevokeAll", n)
	}
}

// RevokeAll is what a password change calls. If it missed even one session, the
// old password would still be usable by whoever knows it — which is the whole
// reason the password was changed.
func TestRevokeAllCatchesASessionStartedJustBeforeIt(t *testing.T) {
	s, _ := newTestSessions(t)
	last, _ := s.Start("parent")
	s.RevokeAll()
	if _, ok := s.Lookup(last.Token); ok {
		t.Error("the most recent session survived RevokeAll")
	}
}

func TestTheCapEvictsTheLeastRecentlyUsed(t *testing.T) {
	s, clock := newTestSessions(t) // max 4

	first, _ := s.Start("parent")
	var others []Session
	for i := 0; i < 3; i++ {
		clock.Advance(time.Minute)
		sess, err := s.Start("parent")
		if err != nil {
			t.Fatal(err)
		}
		others = append(others, sess)
	}
	// Touch `first` last so it is the freshest of the four. The order is the test:
	// a store that evicted by creation time rather than by deadline would drop this
	// one, and the parent reading the screen would be the one logged out.
	clock.Advance(time.Minute)
	if _, ok := s.Lookup(first.Token); !ok {
		t.Fatal("first expired")
	}
	if n := s.Count(); n != 4 {
		t.Fatalf("Count = %d, want 4", n)
	}

	// One over the cap. The eviction must take the session nobody has used, not
	// the one the parent is typing into.
	clock.Advance(time.Minute)
	if _, ok := s.Lookup(first.Token); !ok {
		t.Fatal("first expired before the eviction")
	}
	newest, err := s.Start("parent")
	if err != nil {
		t.Fatal(err)
	}
	if n := s.Count(); n != 4 {
		t.Errorf("Count = %d after exceeding the cap of 4, want 4", n)
	}
	if _, ok := s.Lookup(newest.Token); !ok {
		t.Error("the session just started is not live")
	}
	if _, ok := s.Lookup(first.Token); !ok {
		t.Error("the most recently used session was evicted instead of the stalest")
	}
	// The oldest by deadline is the one that goes.
	if _, ok := s.Lookup(others[0].Token); ok {
		t.Error("the stalest session survived; something else was evicted")
	}
}

func TestStartRefusesAnEmptyUsername(t *testing.T) {
	s, _ := newTestSessions(t)
	if _, err := s.Start(""); err == nil {
		t.Error("Start(\"\") succeeded: an anonymous session would resolve for anybody")
	}
}

// A session token from a predictable source is a session the child of the house
// can mint. The store must fail rather than fall back.
func TestStartFailsWhenTheEntropySourceFails(t *testing.T) {
	s, _ := newTestSessions(t)
	s.randBytes = func([]byte) error { return errors.New("out of entropy") }

	sess, err := s.Start("parent")
	if !errors.Is(err, ErrNoEntropy) {
		t.Errorf("Start = (%+v, %v), want ErrNoEntropy", sess, err)
	}
	if sess.Token != "" {
		t.Errorf("a session was returned alongside the failure: %q", sess.Token)
	}
	if n := s.Count(); n != 0 {
		t.Errorf("Count = %d after a failed Start", n)
	}
}

// The web server looks a token up on every request while another handler may be
// revoking them all.
func TestConcurrentUseIsSafe(t *testing.T) {
	s, _ := newTestSessions(t)
	s.WithMax(64)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				sess, err := s.Start("parent")
				if err != nil {
					t.Errorf("Start: %v", err)
					return
				}
				s.Lookup(sess.Token)
				s.Revoke(sess.Token)
				s.Count()
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 40; j++ {
			s.RevokeAll()
		}
	}()
	wg.Wait()
}

// The zero-value store must work for the same reason the zero-value throttle must:
// a panic in the login path takes the daemon down, and a daemon that is down stops
// re-locking the children.
func TestTheZeroValueIsAWorkingStore(t *testing.T) {
	s := &Sessions{}
	sess, err := s.Start("parent")
	if err != nil {
		t.Fatalf("Start on a zero-value store: %v", err)
	}
	if len(sess.Token) == 0 {
		t.Error("no token")
	}
	if _, ok := s.Lookup(sess.Token); !ok {
		t.Error("a session started on the zero-value store cannot be looked up")
	}
	if s.window() != DefaultSessionTTL || s.limit() != DefaultMaxSessions {
		t.Errorf("zero-value store: ttl %v, max %d; want the documented defaults", s.window(), s.limit())
	}
	// The cap has to be enforced, not merely readable: against a zero cap the
	// eviction loop never terminates, so this hangs rather than fails.
	full := &Sessions{}
	for i := 0; i < DefaultMaxSessions+3; i++ {
		if _, err := full.Start("parent"); err != nil {
			t.Fatalf("start %d: %v", i, err)
		}
	}
	if n := full.Count(); n != DefaultMaxSessions {
		t.Errorf("zero-value store holds %d sessions, want the cap of %d enforced", n, DefaultMaxSessions)
	}
}

// A session token is 256 bits.
//
// Asserted on the decoded length rather than the string length, because the
// encoding is base64 and "looks long" is not a security property: a token of 8
// bytes is 11 characters and still looks like a token, and 64 bits of bearer
// credential on a LAN is guessable.
func TestATokenIs256Bits(t *testing.T) {
	s, _ := newTestSessions(t)
	sess, err := s.Start("parent")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(sess.Token)
	if err != nil {
		t.Fatalf("the token is not raw-url base64: %v", err)
	}
	if len(raw) != 32 {
		t.Errorf("token is %d bytes, want 32", len(raw))
	}
}
