package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// The write seam is the first thing on this daemon that changes the operator's file, so it
// gets the two assertions the read seams already had -- it is wired, and it is pointed at the
// path that was named.
//
// The second half is the dangerous one. A write closure that closes over the config loaded at
// startup, or over defaultConfigPath, does not fail loudly: it succeeds, and it writes a
// different file than the one the operator is editing. With --config that means the daemon
// silently maintains /etc/plock/config.json while the operator edits a test file, and the
// first real write rolls back everything in the real document.

func TestServeWiresAConfigWritePath(t *testing.T) {
	dir := t.TempDir()
	path := writeServeConfig(t, dir)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	chain, _ := platform.New(cfg.Settings, platform.ExecRunner{})

	srv, err := serveServer(cfg, chain, users.Source{}, platform.SelfLocation{}, path,
		filepath.Join(dir, "state"), context.Background(), os.Stdout, os.Stderr)
	if err != nil {
		t.Fatalf("serveServer: %v", err)
	}
	if srv.Mutate == nil {
		t.Fatal("serve wired no config write path; every override answers 503 on the real daemon")
	}
}

// The write seam has to land in the file --config named, and it has to re-read it first.
//
// Both halves are asserted by putting a marker in the file that the running process has no
// reason to know about: a closure over the startup document would drop it, and a closure
// pointed at the default path would leave the marker file untouched.
func TestTheServedWritePathWritesTheFileTheOperatorNamed(t *testing.T) {
	dir := t.TempDir()
	path := writeServeConfig(t, dir)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	chain, _ := platform.New(cfg.Settings, platform.ExecRunner{})

	srv, err := serveServer(cfg, chain, users.Source{}, platform.SelfLocation{}, path,
		filepath.Join(dir, "state"), context.Background(), os.Stdout, os.Stderr)
	if err != nil {
		t.Fatalf("serveServer: %v", err)
	}
	if srv.Mutate == nil {
		t.Fatal("no write path is wired")
	}

	// A concurrent writer the daemon knows nothing about: `plock` on the command line, or an
	// operator's editor. Whatever the seam reads has to include it.
	other := *cfg
	other.Profiles = append(append([]config.Profile(nil), cfg.Profiles...),
		config.Profile{ID: "weekend", Name: "Weekend", Timezone: "UTC"})
	if err := other.Save(path); err != nil {
		t.Fatalf("seeding the file: %v", err)
	}

	if err := srv.Mutate(func(c *config.Config) error {
		c.AddOverride(config.Override{Username: "casey", Reason: "written through the seam"})
		return nil
	}); err != nil {
		t.Fatalf("Mutate: %v", err)
	}

	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	body := string(onDisk)
	if !strings.Contains(body, "written through the seam") {
		t.Errorf("the mutation never reached the file the operator named (%s):\n%s", path, body)
	}
	// The profile added after serveServer was called is still there. A seam that closed over
	// the startup document would rewrite the file without it, which is a silent rollback of
	// somebody else's change.
	if !strings.Contains(body, "weekend") {
		t.Error("the write rolled back a change made to the file after the daemon started")
	}
}

// A mutation that refuses must leave the file alone.
//
// The handler's refusals run inside the mutation, so "nothing was written" is what makes the
// audit line's claim true. Saving a document that fn rejected would put the daemon's config in
// a state nobody approved.
func TestARefusedMutationSavesNothing(t *testing.T) {
	dir := t.TempDir()
	path := writeServeConfig(t, dir)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	chain, _ := platform.New(cfg.Settings, platform.ExecRunner{})

	srv, err := serveServer(cfg, chain, users.Source{}, platform.SelfLocation{}, path,
		filepath.Join(dir, "state"), context.Background(), os.Stdout, os.Stderr)
	if err != nil {
		t.Fatalf("serveServer: %v", err)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the file: %v", err)
	}
	if err := srv.Mutate(func(*config.Config) error {
		return errors.New("the handler refused this override")
	}); err == nil {
		t.Fatal("Mutate reported success although the mutation refused")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-reading the file: %v", err)
	}
	if string(after) != string(before) {
		t.Error("a refused mutation still changed the file")
	}
}
