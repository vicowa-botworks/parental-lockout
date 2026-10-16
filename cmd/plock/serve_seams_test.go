package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// The pages refuse rather than render an empty table when a seam is missing, which is
// the right behaviour for a request and the wrong behaviour for a build: an unwired seam
// is a 503 on the real daemon and a completely green suite, because every page test wires
// its own seams by hand.
//
// This is the test that asks the assembled server whether it can see the machine. It is
// the only place the answer is the production one.
func TestServeWiresEverySeamThePagesRead(t *testing.T) {
	dir := t.TempDir()
	path := writeServeConfig(t, dir)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	chain, _ := platform.New(cfg.Settings, platform.ExecRunner{})

	srv, err := serveServer(cfg, chain, users.Source{}, platform.SelfLocation{}, path, filepath.Join(dir, "state"), context.Background(), os.Stdout, os.Stderr)
	if err != nil {
		t.Fatalf("serveServer: %v", err)
	}
	// Named rather than looped over: a table of closures would report "a seam is nil"
	// and leave the reader to work out which page just stopped working.
	if srv.Config == nil {
		t.Error("serve wired no configuration source; the dashboard answers 503 on every request")
	}
	if srv.Lockables == nil {
		t.Error("serve wired no session source; the dashboard cannot say who is logged in")
	}
	if srv.Accounts == nil {
		t.Error("serve wired no account source; the accounts page answers 503 on every request")
	}
}

// The configuration source has to read the file the operator named with --config, and
// has to re-read it. Both halves fail silently in the same direction: a closure over the
// config loaded at startup answers every question with the state at process start, and a
// closure pointed at the default path reads a different machine's file -- reads it, and
// once the write path lands, writes it.
func TestTheServedConfigSourceReReadsTheFileTheOperatorNamed(t *testing.T) {
	dir := t.TempDir()
	path := writeServeConfig(t, dir)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	srv, err := serveServer(cfg, nil, users.Source{}, platform.SelfLocation{}, path, filepath.Join(dir, "state"), context.Background(), os.Stdout, os.Stderr)
	if err != nil {
		t.Fatalf("serveServer: %v", err)
	}

	first, err := srv.Config()
	if err != nil {
		t.Fatalf("the configuration source failed on its first read: %v", err)
	}
	if len(first.Profiles) == 0 {
		t.Fatal("the configuration read through the seam has no profiles; it is not the file under test")
	}

	// Rewrite the file the way `plock profile add` would, then read again.
	doc := *first
	doc.Profiles = append(append([]config.Profile{}, doc.Profiles...), config.Profile{
		ID: "added-later", Name: "Added after startup", Timezone: "UTC",
	})
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	second, err := srv.Config()
	if err != nil {
		t.Fatalf("the configuration source failed on its second read: %v", err)
	}
	for _, p := range second.Profiles {
		if p.ID == "added-later" {
			return
		}
	}
	ids := []string{}
	for _, p := range second.Profiles {
		ids = append(ids, p.ID)
	}
	t.Errorf("the served configuration is the snapshot taken at startup; a profile added after it is invisible (profiles: %v)", ids)
}

// fixtureSessions is a session source a test owns. *platform.Chain satisfies the same
// interface, so this exercises the real wiring path; what it does not do is inherit the
// host's session table, which is the thing that made the assertion below decidable.
type fixtureSessions struct {
	got context.Context
	out []platform.Session
	err error
}

func (f *fixtureSessions) Sessions(ctx context.Context) ([]platform.Session, error) {
	f.got = ctx
	return f.out, f.err
}

// The Lockables seam is wired AND it answers, and the ctx it passes down is the serve
// context.
//
// The old assertion here was `srv.Lockables == nil`, which a hollowed-out body satisfies
// perfectly well: W62 replaced the closure's body with `return nil, nil` and cmd/plock,
// internal/web and the whole web battery stayed green. On a real machine that is a
// dashboard that says nobody is logged in -- the fail-open direction, because "no
// sessions" looks like a quiet house rather than a broken daemon, and the pages' own
// "no seam" refusal never fires because the seam is present and simply says nothing.
//
// A fixture rather than the host is what makes this decidable. Asking the running
// machine for its sessions can only ever assert that something came back, and on a
// headless CI runner the honest answer is nothing at all -- which would have made the
// assertion vacuous on exactly the machines that need it red.
func TestTheServedSessionSourceIsTheOneThatWasGiven(t *testing.T) {
	dir := t.TempDir()
	path := writeServeConfig(t, dir)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := &fixtureSessions{out: []platform.Session{
		{ID: "2", Class: "user", UID: 1500, Seat: "seat0", Type: "x11", State: "active"},
		{ID: "c1", Class: "greeter", UID: 1500, Seat: "seat0", Type: "unspecified", State: "active"},
	}}
	srv, err := serveServer(cfg, src, users.Source{}, platform.SelfLocation{}, path, filepath.Join(dir, "state"), ctx, os.Stdout, os.Stderr)
	if err != nil {
		t.Fatalf("serveServer: %v", err)
	}
	if srv.Lockables == nil {
		t.Fatal("serve wired no session source")
	}

	got, err := srv.Lockables()
	if err != nil {
		t.Fatalf("the session source errored: %v", err)
	}
	ids := make([]string, 0, len(got))
	for _, s := range got {
		ids = append(ids, s.ID)
	}
	if len(got) != 2 || ids[0] != "2" || ids[1] != "c1" {
		t.Errorf("the served session source returned %v, want the fixture's 2 and c1; a seam that "+
			"returns nothing renders a dashboard that says nobody is logged in", ids)
	}
	// The context has to be the one serve was handed, or a shutdown leaves the session
	// listing running with no way to be cancelled. Comparing the two contexts' errors
	// while both are live proves nothing -- both are nil -- so the serve context is
	// cancelled first and the question becomes whether the one the seam received dies
	// with it. A seam handed a fresh context.Background() survives the cancellation,
	// which is the defect.
	cancel()
	if src.got == nil {
		t.Error("the session source was called with no context at all")
	} else if err := src.got.Err(); err == nil {
		t.Error("the seam passed a context that outlived the serve context; a session query " +
			"already in flight at shutdown has nothing that can cancel it, so the drain waits on it")
	}
}
