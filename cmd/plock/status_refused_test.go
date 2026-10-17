package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// systemctl installed and refusing is the fourth answer, and the one the three-state model
// had no place for. Printing "not running" for it asserts a deployment fact the probe did
// not establish -- and marks no session declined while doing it, so the session rows read
// lockable/unlocked with no reason attached. That is the exact failure the row-level
// sentence was added to prevent, arriving through the other input.
func TestStatusSaysSystemdRefusedRatherThanNotRunning(t *testing.T) {
	f := statusFixtureFor(t)
	f.sessions = twoAccounts()
	f.daemon = &daemonState{SystemdKnown: true, AskFailed: true,
		Err: errors.New("systemctl exited with 1: System has not been booted with systemd as init system")}
	path := withStatusMachine(t, f)

	code, out, _ := runCmd(t, "status", "--config", path, "--json")
	if code != exitOK {
		t.Fatalf("exit %d, want %d", code, exitOK)
	}
	var doc struct {
		Daemon map[string]any `json:"daemon"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if doc.Daemon["askError"] == nil || doc.Daemon["askError"] == "" {
		t.Errorf("the JSON hides why the ask failed: %v", doc.Daemon)
	}
	if doc.Daemon["systemd"] != true {
		t.Errorf("systemd = %v, want true: systemctl was found and answered, so \"no init system\" would be the wrong sentence",
			doc.Daemon["systemd"])
	}

	_, text, _ := runCmd(t, "status", "--config", path)
	if !strings.Contains(text, "refused to answer") {
		t.Errorf("the text report does not say systemd refused:\n%s", text)
	}
	if !strings.Contains(text, "not been booted") {
		t.Errorf("the text report drops systemctl's own reason, which is the diagnosis:\n%s", text)
	}
	// The two sentences this distinction exists for. Each is a claim about the
	// deployment that a refusal does not license.
	if strings.Contains(text, "\n  daemon      not running") {
		t.Errorf("a refused ask printed as \"not running\":\n%s", text)
	}
	if strings.Contains(text, "no init system") {
		t.Errorf("a refused ask printed as \"no init system\", which is the other wrong answer:\n%s", text)
	}
}

// The location came from the kernel, so a refusal does not invalidate it -- but a report
// that prints only the half that worked invites the operator to read the rest the same
// way. The warning names which source each half came from.
func TestStatusKeepsARefusalsLocationAndSaysWhereItCameFrom(t *testing.T) {
	f := statusFixtureFor(t)
	f.sessions = twoAccounts()
	f.daemon = &daemonState{SystemdKnown: true, AskFailed: true, Running: true, LocationKnown: true,
		Location: platform.SelfLocation{UID: 1000, InUserSlice: true},
		Err:      errors.New("systemctl exited with 1: Failed to connect to bus")}
	path := withStatusMachine(t, f)

	_, out, _ := runCmd(t, "status", "--config", path, "--json")
	var doc struct {
		Warnings []string       `json:"warnings"`
		Daemon   map[string]any `json:"daemon"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if doc.Daemon["running"] != true {
		t.Errorf("the pid's own cgroup said running and the report does not: %v", doc.Daemon)
	}
	var warned bool
	for _, w := range doc.Warnings {
		if strings.Contains(w, "refused to answer") && strings.Contains(w, "/proc") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("no warning saying the location came from /proc rather than systemd:\n%v", doc.Warnings)
	}
}
