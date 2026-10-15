package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// The seam is wired AND it answers. Non-nil is not the property.
//
// W75 replaced the body of serve's Accounts closure with `return nil, nil` and every test
// stayed green: the wiring test checks `srv.Accounts != nil`, and a closure that returns
// nothing is a perfectly good non-nil value. The result on a real machine is an accounts
// page with an empty table on it -- the page that tells a parent which children the daemon
// can see -- and the pages' own "no seam" refusal never fires, because the seam is there
// and simply says nobody lives here.
//
// The assertion has to be against a fixture rather than the host. Asking /etc/passwd
// whether any account is in range is the mistake that made a "backend is available" line
// pass on every desktop and fail on every CI runner, and here it would be worse: a runner
// with no in-range account would make the assertion vacuous rather than red.
func TestTheServedAccountSourceIsTheOneThatWasGiven(t *testing.T) {
	dir := t.TempDir()
	passwd := filepath.Join(dir, "passwd")
	defs := filepath.Join(dir, "login.defs")
	// One in-range account and one below UID_MIN. If the seam ignored its source the
	// answer would be the host's, which is not this pair; if it were hollow the answer
	// would be empty, which is not `ada`.
	if err := os.WriteFile(passwd, []byte(
		"root:x:0:0:root:/root:/bin/sh\n"+
			"ada:x:1500:1500:Ada:/home/ada:/bin/bash\n"+
			"svc:x:900:900:service:/nonexistent:/usr/sbin/nologin\n"), 0o600); err != nil {
		t.Fatalf("write passwd: %v", err)
	}
	if err := os.WriteFile(defs, []byte("UID_MIN 1000\nUID_MAX 60000\n"), 0o600); err != nil {
		t.Fatalf("write login.defs: %v", err)
	}
	src := users.Source{
		PasswdPath:    passwd,
		LoginDefsPath: defs,
		// Homes are a host fact too. The fixture says they exist so the account is
		// classified on its uid alone, which is what this test is about.
		HomeExists: func(string) bool { return true },
	}

	cfgPath := writeServeConfig(t, dir)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	chain, _ := platform.New(cfg.Settings, platform.ExecRunner{})

	srv, err := serveServer(cfg, chain, src, cfgPath, filepath.Join(dir, "state"),
		context.Background(), os.Stdout, os.Stderr)
	if err != nil {
		t.Fatalf("serveServer: %v", err)
	}
	if srv.Accounts == nil {
		t.Fatal("serve wired no account source")
	}

	got, err := srv.Accounts()
	if err != nil {
		t.Fatalf("the account source errored: %v", err)
	}
	if got == nil {
		// The seam now answers with the whole Listing, and a nil one is refused by
		// readAccounts rather than rendered -- so this is the shape that would have
		// reached /users as a 503 on the real daemon.
		t.Fatal("the served account source answered with no listing")
	}
	names := map[string]bool{}
	for _, a := range got.Accounts {
		names[a.Username] = true
	}
	if !names["ada"] {
		t.Errorf("the served account source returned %v, want the fixture's ada in it; a seam that "+
			"returns nothing renders an empty accounts table on a machine with children on it", got)
	}
	if names["root"] || names["svc"] {
		t.Errorf("the served account source returned system accounts %v; the bounds come from the "+
			"source it was given, not from the daemon's assumptions", got)
	}
}
