package web

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/auth"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// #37: rotating the admin credential has to end the sessions that were minted under the
// old one, and has to make the new password work. Both halves or neither -- revoking
// live sessions while the daemon still accepts the stolen password is theatre.
//
// The credential is a row in a file another process writes, so "changed" can only mean
// "differs from what I last read". What it must not mean is "the timestamp moved": the
// write path rewrites the whole document, and a save that touched nothing else would
// log the operator out of their own session mid-edit.

// rotatable is a config source whose admin verifier a test can swap, guarded because
// the watcher reads it on its own goroutine.
type rotatable struct {
	mu       sync.Mutex
	cfg      *config.Config
	readErr  error
	readHits int
}

func (r *rotatable) get() (*config.Config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.readHits++
	if r.readErr != nil {
		return nil, r.readErr
	}
	return r.cfg, nil
}

func (r *rotatable) setVerifier(v string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *r.cfg
	c.Admin.Password = v
	r.cfg = &c
}

func (r *rotatable) hits() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.readHits
}

func rotationServer(t *testing.T) (*Server, *rotatable, *logSink) {
	t.Helper()
	s, logged, _ := newTestServer(t, nil)
	r := &rotatable{cfg: pageConfig()}
	s.Config = r.get
	// Fast enough that a test is not a nap, and the interval is the thing under test
	// rather than the scheduler.
	s.CredentialPoll = 10 * time.Millisecond
	return s, r, logged
}

// serveFor runs the server on a real listener and returns a request function, because
// the watcher only starts inside Serve.
func serveFor(t *testing.T, s *Server) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan struct{})
	go func() { defer close(done); _ = s.Serve(ctx) }()
	select {
	case <-s.startedCh():
	case <-time.After(5 * time.Second):
		t.Fatal("Serve never reported a bind")
	}
	if err := s.startEr; err != nil {
		t.Fatalf("the server did not start: %v", err)
	}
	t.Cleanup(func() { cancel(); <-done })
}

func mintVerifier(t *testing.T, password string) string {
	t.Helper()
	v, err := auth.Hasher{Iterations: auth.MinIterations}.Hash(password)
	if err != nil {
		t.Fatalf("verifier: %v", err)
	}
	return v
}

func tokenOf(t *testing.T, s *Server, password string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", strings.NewReader("username=parent&password="+password))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != 303 {
		t.Fatalf("login with that password did not succeed: %d %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c.Value
		}
	}
	t.Fatal("login set no session cookie")
	return ""
}

// The headline property.
func TestARotatedCredentialEndsSessionsMintedUnderTheOldOne(t *testing.T) {
	s, r, logged := rotationServer(t)
	serveFor(t, s)

	token := tokenOf(t, s, "hunter2")
	if _, ok := s.Sessions.Lookup(token); !ok {
		t.Fatal("the session did not exist before the rotation; the rest of this test proves nothing")
	}

	r.setVerifier(mintVerifier(t, "rotated-away"))

	// Waiting on the log line, not on the session disappearing.
	//
	// The watcher revokes and then logs, so the session can be gone a moment before the
	// line exists -- waiting on the effect and then asserting the record would fail
	// intermittently on a runner that won the race, which is the counter-before-event
	// shape in the other direction. The record is the last thing to happen, so waiting
	// for it bounds both halves.
	if !eventually(func() bool {
		return strings.Contains(strings.Join(logged.Lines(), "\n"), "admin credential changed")
	}, 3*time.Second) {
		t.Fatalf("the revocation happened silently; an operator in the middle of an incident needs to see it:\n%s",
			strings.Join(logged.Lines(), "\n"))
	}
	if _, ok := s.Sessions.Lookup(token); ok {
		t.Error("the credential was rotated and the session signed in under the old one is still valid")
	}
}

// The other half, and the one that is easy to skip: revoking live sessions means nothing
// if the daemon still authenticates the password it was built with at startup.
func TestTheRotatedPasswordWorksAndTheOldOneStopsWorking(t *testing.T) {
	s, r, _ := rotationServer(t)

	tokenOf(t, s, "hunter2") // the old password works before the rotation
	r.setVerifier(mintVerifier(t, "brand-new"))

	// The new password is checked first on purpose. A refused old password records a
	// failure against this client's throttle bucket, and the test penalty is an hour --
	// so asserting the refusal first would make the "the new one works" half fail with a
	// 429 and read as a broken rotation rather than a full bucket.
	tokenOf(t, s, "brand-new") // and with no restart

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", strings.NewReader("username=parent&password=hunter2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code == 303 {
		t.Error("the old password still logs in after the credential was rotated")
	}
}

// The precedence rule, and the shape that makes it matter: a server handed BOTH a frozen
// authenticator and a configuration source. That is exactly what `plock serve` did until
// the dead `srv.Auth = adminAuthenticator(cfg)` assignment was removed -- the config read
// once at startup and handed over, while the file kept moving underneath it -- so "the
// handed value wins" is not a hypothetical, it is the bug that was shipped: the daemon kept
// accepting the password it booted with while the file said otherwise, and the revocation
// watcher happily logged that it ended sessions.
//
// The configuration is the daemon's only source of truth for the credential, so it wins.
// A handed Authenticator is what a server with no configuration source falls back to,
// not an override.
func TestAHandedAuthenticatorDoesNotOutrankTheConfiguration(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	r := &rotatable{cfg: pageConfig()}
	s.Config = r.get
	if s.Auth == nil {
		t.Fatal("the fixture has no handed authenticator; the precedence it is testing cannot be exercised")
	}

	// The config says one password; the handed stub says another. Only the config's may
	// work, and the stub records whether it was consulted at all.
	r.setVerifier(mintVerifier(t, "from-the-file"))
	tokenOf(t, s, "from-the-file")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", strings.NewReader("username=parent&password=hunter2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code == 303 {
		t.Error("the handed authenticator answered a login the configuration does not permit")
	}
}

// The fallback half: with no configuration source, the handed authenticator is the only
// answer, and it is used.
func TestAHandedAuthenticatorIsUsedWhenThereIsNoConfiguration(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	if s.Config != nil {
		t.Fatal("the fixture must have no configuration source for the fallback to be the thing under test")
	}
	tokenOf(t, s, "hunter2")
}

// The issue's explicit warning: a rewrite that leaves the password identical must not
// log everybody out, or a UI save that rewrites the whole document would revoke the
// operator's own session mid-edit.
func TestAConfigRewriteThatKeepsThePasswordKeepsTheSessions(t *testing.T) {
	s, r, logged := rotationServer(t)
	serveFor(t, s)

	token := tokenOf(t, s, "hunter2")
	// Same verifier, brand new document -- what a save of an untouched profile produces.
	r.setVerifier(pageConfig().Admin.Password)

	// Long enough for several ticks, decided by read count rather than wall time so a
	// loaded runner cannot make this pass by never having polled at all.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && r.hits() < 3 {
		time.Sleep(5 * time.Millisecond)
	}
	if r.hits() < 3 {
		t.Fatalf("the watcher polled %d times; the assertion never ran", r.hits())
	}
	if _, ok := s.Sessions.Lookup(token); !ok {
		got := strings.Join(logged.Lines(), "\n")
		t.Errorf("an unchanged credential logged the operator out:\n%s", got)
	}
	if got := strings.Join(logged.Lines(), "\n"); strings.Contains(got, "admin credential changed") {
		t.Errorf("the watcher claimed a rotation that did not happen:\n%s", got)
	}
}

// The watcher's first read is a baseline, not a rotation. Revoking at startup would end
// every session on every daemon restart -- a self-inflicted outage that reads as hygiene.
func TestStartingTheWatcherDoesNotRevokeAnything(t *testing.T) {
	s, _, logged := rotationServer(t)
	resp, _ := login(t, s, "parent", "hunter2")
	before := s.Sessions.Count()
	if before == 0 {
		t.Fatal("no session existed to protect")
	}

	ctx, cancel := context.WithCancel(context.Background())
	go s.watchCredential(ctx, 5*time.Millisecond)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if s.Sessions.Count() != before {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()

	if got := s.Sessions.Count(); got != before {
		t.Errorf("sessions = %d after starting the watcher, want %d; the first read is a baseline", got, before)
	}
	if got := strings.Join(logged.Lines(), "\n"); strings.Contains(got, "admin credential changed") {
		t.Errorf("startup was reported as a rotation:\n%s", got)
	}
	_ = resp
}

// A config that cannot be read right now is not evidence the credential changed. The
// file is replaced rather than truncated, so a read can fail because the watcher arrived
// between the unlink and the rename.
func TestAConfigReadFailureDoesNotLogAnybodyOut(t *testing.T) {
	s, r, logged := rotationServer(t)
	serveFor(t, s)

	token := tokenOf(t, s, "hunter2")
	r.mu.Lock()
	r.readErr = errors.New("stale file handle")
	r.mu.Unlock()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if strings.Contains(strings.Join(logged.Lines(), "\n"), "could not be re-read") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, ok := s.Sessions.Lookup(token); !ok {
		t.Error("a transient config read error ended a live session")
	}
	if got := strings.Join(logged.Lines(), "\n"); !strings.Contains(got, "could not be re-read") {
		t.Errorf("the read failure was swallowed:\n%s", got)
	}
}

// A server with no configuration source has no rotated credential to notice, so no
// watcher runs and no line is written. Starting one anyway would log an error every five
// seconds forever and train the operator to ignore the line that means something.
func TestNoConfigSourceMeansNoWatcher(t *testing.T) {
	s, logged, _ := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	s.CredentialPoll = 5 * time.Millisecond
	serveFor(t, s)

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := strings.Join(logged.Lines(), "\n"); strings.Contains(got, "admin credential could not be re-read") {
		t.Errorf("a server with no configuration source complained about not being able to read it:\n%s", got)
	}
}

// eventually polls until cond holds or the budget is spent. Every watcher test needs it and
// none of them should be guessing a sleep length: too short is a flake, too long is a
// slow suite that still passes for the wrong reason.
func eventually(cond func() bool, budget time.Duration) bool {
	deadline := time.Now().Add(budget)
	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(2 * time.Millisecond)
	}
}
