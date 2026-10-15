package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// Both of these exist because a check can be written, documented, unit-tested in isolation,
// and never registered -- and nothing in the suite notices. `checkConfigWritable` had three
// passing tests against it while `plock doctor` never printed a `config.writable` line, and
// `StatusReport.Backends` was documented as the configured order while the field sat
// unassigned and marshalled as null.
//
// So these do not test the checks. They test that the *command* runs them, which is the only
// claim docs/cli.md actually makes.

// TestDoctorRunsTheWritabilityCheck pins the wiring, not the verdict.
//
// The unit tests in doctor_config_test.go already prove the check distinguishes a writable
// pair from a read-only file and a read-only directory. What they could not see is that
// runDoctor never called it, so a green `plock doctor` read as "the daemon can save this"
// on a machine where it cannot -- on a read-only mount, or a full filesystem, discovered at
// the moment a parent clicks Save.
func TestDoctorRunsTheWritabilityCheck(t *testing.T) {
	withChain(t, &fakeChain{
		names:    []string{"logind"},
		statuses: []platform.BackendStatus{{Name: "logind", Available: true, Detail: "ok"}},
	}, nil)
	withDaemon(t, "active", true, nil)

	path := writeConfig(t)
	code, out, errOut := runCmd(t, "doctor", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d, want %d\n%s\n%s", code, exitOK, out, errOut)
	}
	if !strings.Contains(out, "config.writable") {
		t.Errorf("doctor never ran the writability check, so its report cannot answer "+
			"\"can the daemon save the config\" even though docs/cli.md promises the line:\n%s", out)
	}
}

// TestDoctorJSONCarriesTheWritabilityCheck is the same claim in the machine grammar.
//
// --json is what a monitoring job reads, and its check names are the published contract, so
// a check missing from the text output and a check missing from the array are two separate
// wiring failures.
func TestDoctorJSONCarriesTheWritabilityCheck(t *testing.T) {
	withChain(t, &fakeChain{
		names:    []string{"logind"},
		statuses: []platform.BackendStatus{{Name: "logind", Available: true, Detail: "ok"}},
	}, nil)
	withDaemon(t, "active", true, nil)

	code, out, errOut := runCmd(t, "doctor", "--json", "--config", writeConfig(t))
	if code != exitOK {
		t.Fatalf("exit %d, want %d\n%s\n%s", code, exitOK, out, errOut)
	}
	var rep struct {
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("the --json document does not decode: %v\n%s", err, out)
	}
	for _, c := range rep.Checks {
		if c.Name == "config.writable" {
			return
		}
	}
	t.Errorf("config.writable is absent from the --json checks, so a script reading the "+
		"published field list cannot tell \"writable\" from \"never asked\":\n%s", out)
}

// TestStatusReportsTheBackendsItResolved is the same class of failure in `status`.
//
// The field was declared, documented, and never assigned. `"backends":null` is worse than an
// absent key: it reads as "this machine offers nothing", which is a claim about the desktop
// rather than the honest one, which is that the command did not ask. The data was already
// reachable -- the same chain doctor reads answers Names() and Unknown().
func TestStatusReportsTheBackendsItResolved(t *testing.T) {
	path := withStatusMachine(t, statusFixture{
		now:      time.Date(2026, 3, 14, 20, 0, 0, 0, time.UTC),
		names:    []string{"logind", "kde"},
		unknown:  []string{"wayland-x"},
		sessions: nil,
	})

	code, out, errOut := runCmd(t, "status", "--json", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d, want %d\n%s\n%s", code, exitOK, out, errOut)
	}
	var rep StatusReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("the --json document does not decode: %v\n%s", err, out)
	}
	if rep.Backends == nil {
		t.Errorf("backends marshalled as null -- the field is declared and documented but "+
			"nothing assigns it, so a script sees \"no backends\" where the truth is \"not "+
			"asked\":\n%s", out)
	}
	if got := strings.Join(rep.Backends, ","); got != "logind,kde" {
		t.Errorf("backends = %q, want the configured order logind,kde", got)
	}
	if got := strings.Join(rep.UnknownBackends, ","); got != "wayland-x" {
		t.Errorf("unknownBackends = %q, want wayland-x -- a typo in lockBackends has to "+
			"survive into the report or the operator never learns the name resolved to nothing", got)
	}
}

// TestStatusTextNamesTheBackendsOrSaysThereAreNone covers the human rendering.
//
// `backends` printed with nothing after it is the exact shape this command shipped with, so
// the empty case has to be a sentence rather than a blank.
func TestStatusTextNamesTheBackendsOrSaysThereAreNone(t *testing.T) {
	path := withStatusMachine(t, statusFixture{
		now:   time.Date(2026, 3, 14, 20, 0, 0, 0, time.UTC),
		names: []string{"logind", "kde"},
	})
	_, out, _ := runCmd(t, "status", "--config", path)
	if !strings.Contains(out, "logind -> kde") {
		t.Errorf("the text report does not show the resolution order:\n%s", out)
	}

	empty := withStatusMachine(t, statusFixture{now: time.Date(2026, 3, 14, 20, 0, 0, 0, time.UTC), names: nil})
	_, out, _ = runCmd(t, "status", "--config", empty)
	// Asserted on the backends line itself. A whole-output `Contains(out, "none")` is satisfied
	// by the unrelated "none — the config has no users" line a few rows below, so it passes on
	// a bare `backends` label -- which is the exact bug, reported as covered.
	line := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "backends") {
			line = strings.TrimSpace(l)
		}
	}
	if line == "" {
		t.Errorf("no backends line in the report:\n%s", out)
	} else if !strings.Contains(line, "none") {
		t.Errorf("an empty backend list rendered as %q -- a bare label is indistinguishable "+
			"from the never-assigned field this replaces; the line has to say there are none", line)
	}
}
