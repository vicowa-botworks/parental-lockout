package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// TestCheckDaemonSlice covers the one check whose input is the deployment rather than the
// config.
//
// The self-session rule (#67) is invisible from the CLI: `status` cannot read the daemon's
// cgroup, so a machine where an account will never be locked looks identical to one where it
// is enforced (docs/security.md §8). Doctor is where that question gets asked, and the two
// directions it can get wrong are opposite: warning about a declined account on a box running
// the shipped system unit teaches the operator to ignore the check, and staying silent on a
// hand-started daemon is how the account goes unenforced with nobody saying so.
func TestCheckDaemonSlice(t *testing.T) {
	in := platform.SelfLocation{UID: 4123, InUserSlice: true}
	out := platform.SelfLocation{}

	for _, tc := range []struct {
		name     string
		state    string
		known    bool
		err      error
		self     platform.SelfLocation
		want     checkStatus
		wantText string
		wantFix  string
	}{
		{
			// The case that must stay quiet even though the operator running doctor is
			// sitting in a login session: the service is the system manager's, so it is
			// outside every session and declines nobody.
			name:  "service active, operator in a login session",
			state: "active", known: true, self: in,
			want:     verdictOK,
			wantText: "system manager",
		},
		{
			name:     "service activating",
			state:    "activating",
			known:    true,
			self:     in,
			want:     verdictOK,
			wantText: "system manager",
		},
		{
			// The case the check exists for.
			name:     "no service, operator in a login session",
			state:    "inactive",
			known:    true,
			self:     in,
			want:     verdictWarn,
			wantText: "uid 4123",
			wantFix:  "deploy/plock.service",
		},
		{
			name:     "no service, operator outside any session",
			state:    "inactive",
			known:    true,
			self:     out,
			want:     verdictOK,
			wantText: "outside every login session",
		},
		{
			name:  "no systemd to ask",
			known: false,
			self:  in,
			want:  verdictSkip,
			// A skip must not name an account: with no answer there is no declined uid, and
			// the warning's whole value is that it is specific.
			wantText: "unknown",
		},
		{
			name:     "systemd unreachable",
			known:    true,
			err:      errors.New("dbus unavailable"),
			self:     in,
			want:     verdictWarn,
			wantText: "could not ask",
			wantFix:  "by hand",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, known, err := tc.state, tc.known, tc.err
			self := tc.self
			got := checkDaemonSlice(
				func() (string, bool, error) { return state, known, err },
				func() platform.SelfLocation { return self })

			if got.Name != "daemon.slice" {
				t.Errorf("check name %q, want daemon.slice; the name is what --json scripts match", got.Name)
			}
			if got.Status != tc.want {
				t.Errorf("status %q, want %q (%s)", got.Status, tc.want, got.Detail)
			}
			if !strings.Contains(got.Detail, tc.wantText) {
				t.Errorf("detail %q does not say %q", got.Detail, tc.wantText)
			}
			if tc.wantFix != "" && !strings.Contains(got.Fix, tc.wantFix) {
				t.Errorf("fix %q does not point at %q", got.Fix, tc.wantFix)
			}
		})
	}
}

// TestDoctorRunsTheDaemonSliceCheck pins the wiring rather than the verdict, and it exists
// because the unit test above cannot see this failure at all.
//
// A check that is written but never added to the report is invisible: the suite stays green,
// `plock doctor` prints a clean bill of health, and the operator investigating "enforcement
// is not happening" never learns that the daemon they are about to start from this terminal
// will decline the account in front of them. docs/cli.md promises the line, so a missing line
// is a broken promise in the tool whose whole job is keeping promises about the deployment.
func TestDoctorRunsTheDaemonSliceCheck(t *testing.T) {
	withChain(t, &fakeChain{
		names:    []string{"logind"},
		statuses: []platform.BackendStatus{{Name: "logind", Available: true, Detail: "ok"}},
	}, nil)
	withDaemon(t, "active", true, nil)
	installDoctorSelfLocation(t, platform.SelfLocation{UID: 1000, InUserSlice: true})

	path := writeConfig(t)
	code, out, errOut := runCmd(t, "doctor", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d, want %d\n%s\n%s", code, exitOK, out, errOut)
	}
	if !strings.Contains(out, "daemon.slice") {
		t.Errorf("doctor never ran the deployment-shape check, so its report cannot answer \"would a "+
			"daemon here decline an account\" even though docs/cli.md promises the line:\n%s", out)
	}
}

// TestDoctorJSONCarriesTheDaemonSliceCheck is the same claim in the machine grammar: --json is
// what gets pasted into a bug report, and the check names are the published contract.
func TestDoctorJSONCarriesTheDaemonSliceCheck(t *testing.T) {
	withChain(t, &fakeChain{
		names:    []string{"logind"},
		statuses: []platform.BackendStatus{{Name: "logind", Available: true, Detail: "ok"}},
	}, nil)
	withDaemon(t, "active", true, nil)
	installDoctorSelfLocation(t, platform.SelfLocation{UID: 1000, InUserSlice: true})

	path := writeConfig(t)
	code, out, errOut := runCmd(t, "doctor", "--config", path, "--json")
	if code != exitOK {
		t.Fatalf("exit %d, want %d\n%s\n%s", code, exitOK, out, errOut)
	}
	var doc struct {
		Checks []struct {
			Name string `json:"name"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	var names []string
	for _, c := range doc.Checks {
		names = append(names, c.Name)
	}
	if !strings.Contains(strings.Join(names, ","), "daemon.slice") {
		t.Errorf("the JSON report has no daemon.slice check; names = %v", names)
	}
}

// installDoctorSelfLocation states, for one test, whose login session the shell running doctor
// is inside. Without it the answer comes from /proc and the check's verdict depends on whether
// the test machine has a desktop.
func installDoctorSelfLocation(t *testing.T, self platform.SelfLocation) {
	t.Helper()
	prev := doctorSelfLocation
	doctorSelfLocation = func() platform.SelfLocation { return self }
	t.Cleanup(func() { doctorSelfLocation = prev })
}

// TestCheckDaemonSliceNamesTheAccountNotTheShell pins the specific half of the warning. A
// detail that only says "a daemon started here may decline an account" is not actionable: the
// operator needs the uid, because that is what they compare against the account they expected
// to be enforced.
func TestCheckDaemonSliceNamesTheAccountNotTheShell(t *testing.T) {
	got := checkDaemonSlice(
		func() (string, bool, error) { return "inactive", true, nil },
		func() platform.SelfLocation { return platform.SelfLocation{UID: 1000, InUserSlice: true} })

	if !strings.Contains(got.Detail, "1000") {
		t.Errorf("the warning does not name the uid it is declining: %q", got.Detail)
	}
	if !strings.Contains(got.Detail, "NOT lock") {
		t.Errorf("the warning must state the consequence in the same breath as the cause, got %q", got.Detail)
	}
}
