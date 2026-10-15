package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// This file pins the one gate that stands between a malformed config and a write.
//
// config.Save deliberately does not validate: the store writes what it is handed, because
// validation is the producer's job. That makes doInit's cfg.Validate() the only place in the
// production path where a document is checked before it reaches the disk — and a gate nothing
// exercises is a gate that can be deleted without any test noticing.

// A config can load perfectly well and still be invalid: Load decodes and applies defaults,
// Validate is what decides whether the result means anything. pollSeconds is the clean example
// — an explicit 1 is preserved by Load (defaults are for absent keys, not wrong ones) and
// rejected by Validate, because a 1-second poll multiplies into a tick interval the enforcement
// loop was not designed for.
//
// So the reachable shape is real: an operator hand-edits /etc/plock/config.json, gets one value
// outside the legal range, and then runs plock init --force to reset the admin credential.
// Without the gate, init rewrites the file with the out-of-range value still in it and the
// daemon starts enforcing on a schedule nobody chose.
func TestInitRefusesToWriteAConfigThatLoadsButIsInvalid(t *testing.T) {
	cfgPath, state := initPaths(t)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}

	// Loads cleanly, fails validation. Written by hand, which is exactly how it gets invalid.
	const handEdited = `{"version":1,` +
		`"admin":{"username":"parent","password":"an-existing-verifier"},` +
		`"settings":{"pollSeconds":1,"listen":"0.0.0.0:8443","tls":"auto",` +
		`"lockBackends":["logind"]}}`
	if err := os.WriteFile(cfgPath, []byte(handEdited), 0o600); err != nil {
		t.Fatal(err)
	}

	// The premise, asserted rather than assumed: if Load ever starts rejecting this document,
	// the test below would be exercising the read path and the gate would go untested again.
	if _, err := config.Load(cfgPath); err != nil {
		t.Fatalf("the fixture must be loadable for this test to reach the gate: %v", err)
	}

	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state, "--force")
	if code == exitOK {
		t.Fatal("init wrote a config that fails validation")
	}
	if !strings.Contains(stderr, "invalid") {
		t.Errorf("refusal should say the config is invalid, got:\n%s", stderr)
	}

	now, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(now) != handEdited {
		t.Errorf("the invalid config was rewritten anyway:\n%s", now)
	}
}

// The other half: the gate must not fire on a config that is merely *unusual*. A legal document
// with an explicit pollSeconds is the common case, and a gate that cannot tell "invalid" from
// "not what I would have written" would make init refuse every real machine.
func TestInitWritesAConfigThatIsValidAtTheBoundary(t *testing.T) {
	cfgPath, state := initPaths(t)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	// pollSeconds 5 is the documented minimum, not below it.
	const legal = `{"version":1,` +
		`"admin":{"username":"parent","password":"an-existing-verifier"},` +
		`"settings":{"pollSeconds":5,"listen":"0.0.0.0:8443","tls":"auto",` +
		`"lockBackends":["logind"]}}`
	if err := os.WriteFile(cfgPath, []byte(legal), 0o600); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state, "--force")
	if code != exitOK {
		t.Fatalf("init refused a valid config (exit %d):\n%s", code, stderr)
	}
}
