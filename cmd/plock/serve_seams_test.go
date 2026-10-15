package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
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

	srv, err := serveServer(cfg, chain, path, filepath.Join(dir, "state"), context.Background(), os.Stdout, os.Stderr)
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
	srv, err := serveServer(cfg, nil, path, filepath.Join(dir, "state"), context.Background(), os.Stdout, os.Stderr)
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
