package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

const (
	// tokenBytes is 256 bits. A session token is a bearer credential over the LAN,
	// so it needs to be unguessable rather than merely unlikely-to-collide, and it
	// is the one value here an attacker gets to submit repeatedly.
	tokenBytes = 32

	// DefaultSessionTTL is a sliding window: it restarts on each use, so a parent
	// working in the admin UI is not interrupted, while a token copied off a
	// shared machine stops working twelve hours after it was last touched rather
	// than twelve hours after it was minted.
	DefaultSessionTTL = 12 * time.Hour

	// DefaultMaxSessions bounds how many tokens are live at once. The admin is one
	// person, so thirty-two is already generous; the point is that a stolen or
	// replayed token cannot accumulate indefinitely, and that RevokeAll is a
	// single operation with a bounded cost.
	DefaultMaxSessions = 32
)

// ErrNoEntropy reports crypto/rand refusing to produce bytes.
//
// It is not worked around. A session token from a predictable source is a session
// the child of the house can mint, and the alternative to failing is a login that
// succeeds and then hands control of every lockout to whoever guesses next.
var ErrNoEntropy = errors.New("system random source unavailable")

// Session is a live login.
type Session struct {
	Token     string
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Sessions is the store of live admin logins.
//
// It is safe for concurrent use: the web server looks a token up on every request
// while another handler may be revoking them all because the password just
// changed.
//
// Expiry is enforced on access, not by a background goroutine. A ticker would be
// a second thing to get wrong — it would need its own lifecycle, it would not run
// while the process is stopped, and a gap in it would silently extend sessions.
// Checking on read means a token is never honoured after its deadline no matter
// what else is happening, and an idle store costs nothing.
type Sessions struct {
	mu sync.Mutex

	byToken map[string]*Session
	max     int
	ttl     time.Duration

	// now and randBytes are seams for the two things here that a test cannot
	// otherwise control: the clock, and the case where the entropy source fails.
	now       func() time.Time
	randBytes func([]byte) error
}

// NewSessions returns a store with the documented defaults.
func NewSessions() *Sessions {
	return &Sessions{
		byToken:   map[string]*Session{},
		max:       DefaultMaxSessions,
		ttl:       DefaultSessionTTL,
		now:       time.Now,
		randBytes: func(b []byte) error { _, err := rand.Read(b); return err },
	}
}

// WithTTL and WithMax return the store with one bound changed, for tests and for
// a deployment that wants a shorter window. Zero leaves the default.
func (s *Sessions) WithTTL(d time.Duration) *Sessions {
	if d > 0 {
		s.ttl = d
	}
	return s
}

func (s *Sessions) WithMax(n int) *Sessions {
	if n > 0 {
		s.max = n
	}
	return s
}

// Start mints a session for an authenticated username.
func (s *Sessions) Start(username string) (Session, error) {
	if username == "" {
		return Session{}, errors.New("session needs a username")
	}
	raw := make([]byte, tokenBytes)
	if err := s.readRand(raw); err != nil {
		return Session{}, fmt.Errorf("%w: %v", ErrNoEntropy, err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byToken == nil {
		s.byToken = map[string]*Session{}
	}
	s.purgeLocked()
	// At capacity, the token that would have expired soonest goes. With a sliding
	// window that is the one least recently used, so the eviction is "the machine
	// nobody has touched in the longest time" rather than an arbitrary pick — and
	// it means the cap cannot be used to lock the parent out by filling the store
	// first: whatever is already there is always fresher than the new login.
	for len(s.byToken) >= s.limit() {
		s.evictSoonestToExpireLocked()
	}
	at := s.clock()
	sess := &Session{
		Token: token, Username: username,
		CreatedAt: at, ExpiresAt: at.Add(s.window()),
	}
	s.byToken[token] = sess
	return *sess, nil
}

// Lookup resolves a token and slides its expiry.
//
// The map lookup is not constant time and that is fine here, unlike a password
// comparison: a Go map's cost depends on which bucket the hash lands in, not on
// how much of the key matched, so there is no partial-match signal to measure.
// Timing only matters where the secret is short enough to search incrementally,
// and 256 random bits are not.
func (s *Sessions) Lookup(token string) (Session, bool) {
	if token == "" {
		return Session{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.byToken[token]
	if !ok {
		return Session{}, false
	}
	at := s.clock()
	if !at.Before(sess.ExpiresAt) {
		// Expired. Deleting it here rather than trusting a purge pass is what
		// makes "never honoured after its deadline" true.
		delete(s.byToken, token)
		return Session{}, false
	}
	sess.ExpiresAt = at.Add(s.window())
	return *sess, true
}

// Revoke ends one session.
func (s *Sessions) Revoke(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byToken, token)
}

// RevokeAll ends every session. It is what a password change calls: the old
// password may still be known to somebody, and the only thing that makes a
// password change effective against a session already in progress is ending the
// sessions.
func (s *Sessions) RevokeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byToken = map[string]*Session{}
}

// Count reports how many sessions are live, not counting any that have expired.
func (s *Sessions) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byToken == nil {
		s.byToken = map[string]*Session{}
	}
	s.purgeLocked()
	return len(s.byToken)
}

// limit and window are the two bounds, defaulted.
//
// The defaulting is not cosmetic. `for len(s.byToken) >= s.max` against a zero
// max is an infinite loop, not a refusal — so a zero-value store does not fail,
// it hangs the request thread that tried to log in. Reading the field directly was
// a bug that no amount of "always use NewSessions" discipline makes safe.
func (s *Sessions) limit() int {
	if s.max > 0 {
		return s.max
	}
	return DefaultMaxSessions
}

func (s *Sessions) window() time.Duration {
	if s.ttl > 0 {
		return s.ttl
	}
	return DefaultSessionTTL
}

// readRand fills b from the entropy source, defaulting to crypto/rand.
//
// The zero value has to work: see the note on clock. A nil func here would panic
// on the first login, and a panic in this path is the daemon going down rather
// than a login being refused.
func (s *Sessions) readRand(b []byte) error {
	if s.randBytes != nil {
		return s.randBytes(b)
	}
	_, err := rand.Read(b)
	return err
}

// clock is the time source, defaulting to the real one. See the note on
// Throttle.clock: a zero value must work, because the alternative is a panic in
// the path that authenticates the only person who can turn enforcement off.
func (s *Sessions) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// purgeLocked drops everything past its deadline. Called on every mutation, which
// is what keeps a store nobody is using from holding expired tokens forever
// without needing a goroutine to notice.
func (s *Sessions) purgeLocked() {
	at := s.clock()
	for t, sess := range s.byToken {
		if !at.Before(sess.ExpiresAt) {
			delete(s.byToken, t)
		}
	}
}

// evictSoonestToExpireLocked removes one session, choosing the one a user would
// least expect to lose.
//
// Sorting the whole store to find one entry is O(n log n) at n=32, which is
// nothing; the reason to write it this way rather than keeping a heap is that a
// heap has to be maintained on every sliding refresh, and a stale heap would
// evict the wrong session — the failure being that an active admin gets logged
// out while an idle one survives.
func (s *Sessions) evictSoonestToExpireLocked() {
	if len(s.byToken) == 0 {
		return
	}
	type entry struct {
		token   string
		expires time.Time
	}
	all := make([]entry, 0, len(s.byToken))
	for t, sess := range s.byToken {
		all = append(all, entry{t, sess.ExpiresAt})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].expires.Equal(all[j].expires) {
			// A tie needs a deterministic winner or the eviction depends on map
			// iteration order, and a test cannot say which session was dropped.
			return all[i].token < all[j].token
		}
		return all[i].expires.Before(all[j].expires)
	})
	delete(s.byToken, all[0].token)
}
