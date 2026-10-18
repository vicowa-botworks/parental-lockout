package main

import (
	"context"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// The dashboard's backend probe is wired, and it reaches the object the lock path uses.
//
// Same shape as the lock assertion beside it, for the same reason: `srv.Probe != nil` is
// satisfied by a closure that answers nothing, and the defect that hides is a page reporting a
// capability nobody was asked about. So the seam is called, and the fixture has to have been
// reached.
//
// The identity half is the load-bearing one. A probe built from a second platform.New inside
// serve would report on a set of backends that need not be the one the enforcement loop
// enforces with, and the operator would read a healthy table for a machine whose real lock path
// is broken -- the disagreement #1018 is written about, arriving through assembly rather than
// through a type assertion.

// fixtureProbe is a session source that can also lock and also report on its backends, and it
// records every call, so "the page asked the object serve was handed" is an observation.
type fixtureProbe struct {
	fixtureSessions
	probed   int
	locked   []string
	statuses []platform.BackendStatus
}

func (f *fixtureProbe) Lock(_ context.Context, s platform.Session) error {
	f.locked = append(f.locked, s.ID)
	return nil
}

func (f *fixtureProbe) Probe(context.Context) []platform.BackendStatus {
	f.probed++
	return f.statuses
}

func TestTheServedProbeAsksTheObjectThatListedSessions(t *testing.T) {
	dir := t.TempDir()
	src := &fixtureProbe{
		fixtureSessions: fixtureSessions{out: []platform.Session{
			{ID: "2", Class: "user", UID: 1500, Seat: "seat0", Type: "x11", State: "active"},
		}},
		statuses: []platform.BackendStatus{{Name: "kde", Available: true, Detail: "gdbus found"}},
	}
	srv := serveWithSource(t, dir, src)

	if srv.Probe == nil {
		t.Fatal("the served server has no backend-probe seam, so /users/{name}/test-lock " +
			"answers that this daemon has no probe path")
	}
	got := srv.Probe(context.Background())
	if src.probed != 1 {
		t.Fatalf("the served probe seam answered from nowhere: the fixture was called %d times", src.probed)
	}
	if len(got) != 1 || got[0].Name != "kde" || !got[0].Available {
		t.Errorf("the served probe returned %+v, want the fixture's own table", got)
	}
	// The lock seam is wired from the same object, and the probe must not have used it.
	if srv.Lock == nil {
		t.Error("the served server has no lock seam, so the dashboard's Lock now is dead")
	}
	if len(src.locked) != 0 {
		t.Errorf("installing the probe seam locked sessions %v", src.locked)
	}
}

// TestTheServedProbeIsOptional. A session source that can list but cannot probe is a legal
// object, and serve must hand the server a nil seam rather than a probe that invents an answer.
// The page then says the daemon has no probe path, which is true, instead of rendering an empty
// table, which is not.
func TestTheServedProbeIsOptional(t *testing.T) {
	dir := t.TempDir()
	src := &fixtureChain{
		fixtureSessions: fixtureSessions{out: []platform.Session{
			{ID: "2", Class: "user", UID: 1500, Seat: "seat0", Type: "x11", State: "active"},
		}},
	}
	srv := serveWithSource(t, dir, src)
	if srv.Probe != nil {
		t.Error("a session source with no Probe method was handed a probe seam anyway")
	}
	if srv.Lock == nil {
		t.Error("asserting the probe seam is absent must not cost the server its lock seam")
	}
}
