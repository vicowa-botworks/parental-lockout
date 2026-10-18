package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
	"github.com/vicowa-botworks/parental-lockout/internal/web"
)

// The dashboard's lock path is wired AND it reaches the object that listed the sessions.
//
// The W62 lesson from serve_seams_test.go applies here word for word: asserting
// `srv.Lock == nil` is satisfied by a closure that answers nothing, and the defect it would
// hide is a button that reports a lock it never asked for -- the one failure this feature
// exists not to commit. So the seam is called, and the fixture has to have been reached.
//
// The identity half is the reason the seam is taken from the session source rather than
// built again inside serve. One chain goes to the enforcement loop and to this server; a
// page that locked through a second platform.New would be asking a different set of backends
// about the same screen, and the operator would read whichever answer the page happened to
// get. That is the same disagreement #1018 was written about, arriving through assembly
// rather than through a type assertion.

// fixtureChain is a session source that can also lock, and it records both calls, so "the
// page locked through the object serve was handed" is an observation rather than an
// assumption about how the assembly was written.
type fixtureChain struct {
	fixtureSessions
	locked  []string
	lockErr error
}

func (f *fixtureChain) Lock(_ context.Context, s platform.Session) error {
	f.locked = append(f.locked, s.ID)
	return f.lockErr
}

func serveWithSource(t *testing.T, dir string, src sessionSource) *web.Server {
	t.Helper()
	path := writeServeConfig(t, dir)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	srv, err := serveServer(cfg, src, users.Source{}, platform.SelfLocation{}, path,
		filepath.Join(dir, "state"), filepath.Join(dir, "trail.jsonl"), ctx, os.Stdout, os.Stderr)
	if err != nil {
		t.Fatalf("serveServer: %v", err)
	}
	return srv
}

func TestTheServedLockPathLocksTheObjectThatListedSessions(t *testing.T) {
	dir := t.TempDir()
	src := &fixtureChain{
		fixtureSessions: fixtureSessions{out: []platform.Session{
			{ID: "2", Class: "user", UID: 1500, Seat: "seat0", Type: "x11", State: "active"},
		}},
	}
	srv := serveWithSource(t, dir, src)

	if srv.Lock == nil {
		t.Fatal("serve wired no lock path; the dashboard's lock-now button would answer that " +
			"this daemon cannot lock screens, on a daemon that can")
	}
	if _, err := srv.Lockables(); err != nil {
		t.Fatalf("the listing seam errored: %v", err)
	}
	if err := srv.Lock(context.Background(), platform.Session{ID: "2"}); err != nil {
		t.Fatalf("the lock seam errored: %v", err)
	}
	if len(src.locked) != 1 || src.locked[0] != "2" {
		t.Errorf("the served lock path reached %v; the page asks the chain serve was handed, "+
			"and an empty record means the seam closes over something else", src.locked)
	}
}

// TestTheServedLockPathPassesTheChainsAnswerThrough. The three outcomes are distinguished by
// the error that comes back -- nil, ErrUnconfirmed, or a hard error -- so a serve layer that
// swallowed or replaced it would collapse them silently, and the page would say "not locked"
// about a lock that was asked for and could not be proved.
func TestTheServedLockPathPassesTheChainsAnswerThrough(t *testing.T) {
	dir := t.TempDir()
	src := &fixtureChain{
		fixtureSessions: fixtureSessions{out: []platform.Session{{ID: "2"}}},
		lockErr:         platform.ErrUnconfirmed,
	}
	srv := serveWithSource(t, dir, src)

	err := srv.Lock(context.Background(), platform.Session{ID: "2"})
	if !errors.Is(err, platform.ErrUnconfirmed) {
		t.Errorf("the chain's unconfirmed answer arrived as %v; the page reads that as a "+
			"hard failure and tells the operator to fix a backend that did its job", err)
	}
}

// TestAServerWhoseSourceCannotLockHasNoLockPath. doctor and every test that only wants to
// state a set of sessions hand serve a listing that cannot lock. The seam stays nil rather
// than becoming a no-op that reports success, because the page then says this daemon has no
// lock path instead of reporting a lock nobody asked for.
func TestAServerWhoseSourceCannotLockHasNoLockPath(t *testing.T) {
	dir := t.TempDir()
	srv := serveWithSource(t, dir, &fixtureSessions{out: []platform.Session{{ID: "2"}}})

	if srv.Lock != nil {
		t.Error("serve invented a lock path for a source that cannot lock; a button that " +
			"answers 200 without reaching a desktop is the failure this route exists to avoid")
	}
}
