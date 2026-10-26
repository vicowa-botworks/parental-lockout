package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/cli"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// The remaining doctor checks, each pinned by the situation it exists to distinguish.

// A poll interval is not a preference. Above a few minutes the child is not locked out for
// minutes after the window opens, and from the child's side that is indistinguishable from a
// lockout that is simply not happening.
func TestCheckPollIntervalBoundsTheWindow(t *testing.T) {
	for _, tc := range []struct {
		seconds int
		want    checkStatus
	}{
		{0, verdictFail},   // the daemon cannot use 0 as an interval
		{-1, verdictFail},  // nor a negative one
		{30, verdictOK},    // the documented default
		{300, verdictOK},   // the top of the allowed range, inclusive
		{301, verdictWarn}, // one past it must not read as fine
		{3600, verdictWarn},
	} {
		got := checkPollInterval(tc.seconds)
		if got.Status != tc.want {
			t.Errorf("pollSeconds %d: status %q, want %q (%s)", tc.seconds, got.Status, tc.want, got.Detail)
		}
	}
	// The bound has to be stated in the message. "poll interval is high" without the
	// number the operator has to type is a sentence that sends them to grep the source.
	got := checkPollInterval(3600)
	if !strings.Contains(got.Detail, "3600") {
		t.Errorf("the warning does not name the value it is warning about: %q", got.Detail)
	}
}

// TLS is the check that decides whether the admin password crosses the network in clear
// text, so its three expiry verdicts are each worth a test: an expired certificate makes the
// browser refuse the page and the daemon's log say nothing about why.
// pairOK stands in for web.LoadCustom on the rows whose subject is the certificate's validity
// window. The pair itself is pinned by TestDoctorAndStartUpCannotDisagreeAboutThePair, against
// the real loader and real files, because a fake cannot prove an identity with a loader it does
// not call.
func pairOK(string, string) error { return nil }

func TestCheckTLSExpiryWindows(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	loadValid := func(string) (time.Time, time.Time, error) {
		return now.Add(-30 * 24 * time.Hour), now.Add(60 * 24 * time.Hour), nil
	}

	// The control: a certificate with months left is fine, so the three cases below cannot
	// be a fixture that is always broken.
	if got := checkTLS("custom", "/tmp/x.pem", "/tmp/x.key", now, loadValid, pairOK); got.Status != verdictOK {
		t.Fatalf("control: a certificate valid for 60 more days was refused: %s", got.Detail)
	}

	for _, tc := range []struct {
		name string
		// offset from now applied to notAfter (or notBefore)
		notBefore, notAfter time.Time
		want                checkStatus
		wantText            string
	}{
		{
			name:      "expired yesterday",
			notBefore: now.Add(-60 * 24 * time.Hour),
			notAfter:  now.Add(-24 * time.Hour),
			want:      verdictFail,
			wantText:  "expired",
		},
		{
			name:      "not yet valid",
			notBefore: now.Add(24 * time.Hour),
			notAfter:  now.Add(60 * 24 * time.Hour),
			want:      verdictFail,
			wantText:  "not valid until",
		},
		{
			// Inside the 14-day warning window. This is the case a check written as
			// "expired or not" gets wrong by returning ok.
			name:      "expiring soon",
			notBefore: now.Add(-60 * 24 * time.Hour),
			notAfter:  now.Add(3 * 24 * time.Hour),
			want:      verdictWarn,
			wantText:  "expires",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nb, na := tc.notBefore, tc.notAfter
			load := func(string) (time.Time, time.Time, error) { return nb, na, nil }
			got := checkTLS("custom", "/tmp/x.pem", "/tmp/x.key", now, load, pairOK)
			if got.Status != tc.want {
				t.Errorf("status %q, want %q (%s)", got.Status, tc.want, got.Detail)
			}
			if !strings.Contains(got.Detail, tc.wantText) {
				t.Errorf("detail %q does not say %q", got.Detail, tc.wantText)
			}
			// The date is the actionable part: an operator renewing a certificate needs
			// to know which one and when it died.
			if !strings.Contains(got.Detail, "/tmp/x.pem") {
				t.Errorf("detail %q does not name the file", got.Detail)
			}
		})
	}
}

// "off" is a warning rather than a failure because it is a legitimate choice on a machine
// the operator has sectioned off -- but it has to be the loudest thing on the page, because
// it means the admin password crosses the network in clear text.
func TestCheckTLSWarnsLoudlyOnCleartext(t *testing.T) {
	got := checkTLS("off", "", "", time.Now(), func(string) (time.Time, time.Time, error) {
		t.Error("the certificate was loaded while tls was off")
		return time.Time{}, time.Time{}, nil
	}, pairOK)
	if got.Status != verdictWarn {
		t.Errorf("status %q, want a warning", got.Status)
	}
	for _, want := range []string{"clear text", "password"} {
		if !strings.Contains(got.Detail, want) {
			t.Errorf("the warning does not mention %q: %q", want, got.Detail)
		}
	}
}

// An unrecognised tls mode is a typo in the one setting that guards the credential, and the
// server would start with whatever the zero value implies.
func TestCheckTLSRefusesAnUnknownMode(t *testing.T) {
	got := checkTLS("ture", "/tmp/x.pem", "/tmp/x.key", time.Now(), func(string) (time.Time, time.Time, error) {
		return time.Time{}, time.Time{}, nil
	}, pairOK)
	if got.Status == verdictOK {
		t.Fatal(`settings.tls "ture" was accepted`)
	}
	if !strings.Contains(got.Detail, `"ture"`) {
		t.Errorf("the refusal does not quote the value: %q", got.Detail)
	}
	if !strings.Contains(got.Fix, "auto") {
		t.Errorf("the fix does not list the valid modes: %q", got.Fix)
	}
}

// custom with no certFile is the shape where the server has nothing to serve with.
func TestCheckTLSRefusesCustomWithNoCertificate(t *testing.T) {
	got := checkTLS("custom", "   ", "/tmp/x.key", time.Now(), func(string) (time.Time, time.Time, error) {
		t.Error("an empty certFile was opened anyway")
		return time.Time{}, time.Time{}, nil
	}, func(string, string) error {
		t.Error("an empty certFile was handed to the pair loader anyway")
		return nil
	})
	if got.Status == verdictOK {
		t.Fatal(`"custom" with an empty certFile was accepted`)
	}
}

// A certificate that cannot be read must be reported as unreadable, not as expired. The two
// have different fixes and only one of them is `certbot renew`.
func TestCheckTLSReportsAnUnreadableCertificate(t *testing.T) {
	got := checkTLS("custom", "/etc/plock/tls.pem", "/etc/plock/tls.key", time.Now(),
		func(string) (time.Time, time.Time, error) { return time.Time{}, time.Time{}, errors.New("no PEM data") },
		pairOK)
	if got.Status == verdictOK {
		t.Fatal("an unreadable certificate was reported as fine")
	}
	if !strings.Contains(got.Detail, "no PEM data") {
		t.Errorf("the underlying error is missing: %q", got.Detail)
	}
}

// The daemon check has three answers and they must not collapse into one another: "active",
// "not running", and "there is no systemd here" lead to three different actions.
func TestCheckDaemonRunningKeepsTheThreeAnswersApart(t *testing.T) {
	for _, tc := range []struct {
		name     string
		state    string
		known    bool
		err      error
		want     checkStatus
		wantText string
	}{
		{name: "active", state: "active", known: true, want: verdictOK, wantText: "active"},
		{name: "activating", state: "activating", known: true, want: verdictWarn, wantText: "activating"},
		{name: "inactive", state: "inactive", known: true, want: verdictFail, wantText: "inactive"},
		{name: "failed", state: "failed", known: true, want: verdictFail, wantText: "failed"},
		{
			// The one that must never be a failure: a box with a different supervisor
			// has no answer, and "the service is inactive" would be invented.
			name:     "no systemd",
			known:    false,
			want:     verdictSkip,
			wantText: "not checked",
		},
		{
			name:     "systemd unreachable",
			known:    true,
			err:      errors.New("dbus unavailable"),
			want:     verdictWarn,
			wantText: "could not ask",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, known, err := tc.state, tc.known, tc.err
			got := checkDaemonRunning(func() (string, bool, error) { return state, known, err })
			if got.Status != tc.want {
				t.Errorf("status %q, want %q (%s)", got.Status, tc.want, got.Detail)
			}
			if !strings.Contains(got.Detail, tc.wantText) {
				t.Errorf("detail %q does not say %q", got.Detail, tc.wantText)
			}
		})
	}
}

// runnerFor is a platform.Runner that answers with a fixed result, so the systemd probing
// can be exercised without a systemd.
type runnerFor struct {
	out []byte
	err error
}

func (r runnerFor) Run(context.Context, []string, []string, uint32) ([]byte, error) {
	return r.out, r.err
}

// A missing systemctl means "no systemd here", which is a skip. Every other error is a real
// failure to ask, and reporting it as "not running" invents the problem the tool was called
// in to look for.
func TestSystemdStateSeparatesNoSystemdFromCannotAsk(t *testing.T) {
	t.Run("nil runner", func(t *testing.T) {
		state, known, err := systemdState(nil)
		if known || err != nil || state != "" {
			t.Errorf("got %q/%v/%v, want \"\"/false/nil", state, known, err)
		}
	})

	t.Run("systemctl missing", func(t *testing.T) {
		// The error ExecRunner returns, not the stdlib's. lookupReason rewrites exec's
		// wording on purpose, so this fixture quoting `executable file not found in
		// $PATH` pinned a string production never produces -- and the real one fell
		// through to "could not ask systemd", warning about a broken bus on a machine
		// that has no init system at all.
		state, known, err := systemdState(runnerFor{
			err: fmt.Errorf("%s: %w", "systemctl", platform.ErrBinaryNotFound),
		})
		if known || err != nil || state != "" {
			t.Errorf("got %q/%v/%v, want \"\"/false/nil: a box with no systemd is not a box with a dead service",
				state, known, err)
		}
	})

	// The other direction of the same prose match. systemctl's bus errors quote errno in
	// lowercase often enough to matter, and reading one as "there is no init system here"
	// skips the check on a machine that has one and simply would not answer.
	t.Run("a bus error quoting ENOENT is still a failure to ask", func(t *testing.T) {
		state, known, err := systemdState(runnerFor{
			err: errors.New("systemctl exited with 1: Failed to connect to user scope bus: no such file or directory"),
		})
		if !known || err == nil || state != "" {
			t.Errorf("got %q/%v/%v, want known=true with the error", state, known, err)
		}
	})

	t.Run("cannot ask", func(t *testing.T) {
		state, known, err := systemdState(runnerFor{err: errors.New("connection refused")})
		if !known || err == nil {
			t.Errorf("got %q/%v/%v, want known=true with the error: this is a real failure, not an absent init system",
				state, known, err)
		}
	})

	// systemctl exits non-zero for a non-active state and prints the word anyway, so the
	// word wins over the exit status.
	t.Run("non-zero exit with a state word", func(t *testing.T) {
		state, known, err := systemdState(runnerFor{out: []byte("inactive\n"), err: errors.New("exit status 3")})
		if state != "inactive" || !known || err != nil {
			t.Errorf("got %q/%v/%v, want inactive/true/nil", state, known, err)
		}
	})

	t.Run("active", func(t *testing.T) {
		state, known, err := systemdState(runnerFor{out: []byte("active\n")})
		if state != "active" || !known || err != nil {
			t.Errorf("got %q/%v/%v, want active/true/nil", state, known, err)
		}
	})

	// Silence is not "inactive". systemctl printing nothing means the question was not
	// answered, and the error has to carry what was said so the operator can see why.
	t.Run("no state word", func(t *testing.T) {
		state, _, err := systemdState(runnerFor{out: []byte("")})
		if state != "" || err == nil {
			t.Fatalf("got %q/%v, want an error: an empty answer is not a state", state, err)
		}
		if !strings.Contains(err.Error(), "systemctl is-active said") {
			t.Errorf("the error does not quote the answer: %v", err)
		}
	})
}

func TestParseActiveStateIgnoresNoise(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"active\n", "active"},
		{"  activating  \n", "activating"},
		// systemd can print a warning line before the state on a degraded bus.
		{"Warning: The major version of systemd is 250\ninactive\n", "inactive"},
		{"", ""},
		{"something else entirely\n", ""},
		// A word that is not one of the six states must not be returned as a state.
		{"reloading\n", ""},
	} {
		if got := parseActiveState([]byte(tc.in)); got != tc.want {
			t.Errorf("parseActiveState(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// certValidity reads the window out of a PEM file. A file that is not PEM at all -- a text
// file, a DER blob, an empty file -- has to come back as an error, because the caller's
// alternative is a zero time window that reads as "expired in year 1".
func TestCertValidityRejectsANonCertificate(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct{ name, body string }{
		{"empty", ""},
		{"text", "this is not a certificate\n"},
		{"truncated pem", "-----BEGIN CERTIFICATE-----\nQUJD\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".pem")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			nb, na, err := certValidity(path)
			if err == nil {
				t.Fatalf("a %s file was accepted as a certificate (window %v..%v)", tc.name, nb, na)
			}
			// A zero window must never escape, whatever the error says.
			if !nb.IsZero() || !na.IsZero() {
				t.Errorf("an error was returned alongside a non-zero window: %v..%v", nb, na)
			}
		})
	}
}

// The dangling-profile list is the one part of doctor's output that is a list of names.
// Unordered, two runs on the same machine print different text -- Go randomises map
// iteration -- and any test or diff that compares doctor output becomes flaky for a reason
// that has nothing to do with the machine.
func TestDanglingProfilesAreListedInAStableOrder(t *testing.T) {
	names := []string{"zed", "anna", "mika", "boru", "yuki"}
	build := func() check {
		var accounts []unenforcedCandidate
		for _, n := range names {
			accounts = append(accounts, unenforcedCandidate{
				Username: n, Managed: true, DanglingProfile: "school",
			})
		}
		return checkUnenforced(time.Unix(1700000000, 0).UTC(), accounts)
	}

	var first string
	for i := 0; i < 8; i++ {
		got := build()
		if got.Status != verdictFail {
			t.Fatalf("run %d: %d accounts name a profile that does not exist and the check said %q (%s)",
				i, len(names), got.Status, got.Detail)
		}
		if first == "" {
			first = got.Detail
		}
		if got.Detail != first {
			t.Fatalf("run %d differs:\n first: %s\n  this: %s", i, first, got.Detail)
		}
	}

	// Sorted, not merely stable: an operator scanning the line to find their own child's
	// account needs an order they can predict.
	saw := map[string]int{}
	for _, n := range names {
		k := strings.Index(first, n)
		if k < 0 {
			t.Fatalf("%s is not named: %s", n, first)
		}
		saw[n] = k
	}
	sorted := append([]string(nil), names...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j-1] > sorted[j]; j-- {
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}
	for i := 1; i < len(sorted); i++ {
		if saw[sorted[i-1]] > saw[sorted[i]] {
			t.Errorf("%s is listed after %s; the list is not sorted: %s", sorted[i-1], sorted[i], first)
		}
	}
}

// The count in the message is what makes the list actionable, and it has to be the number of
// accounts named -- a check that says "3 account(s)" over five names teaches the operator to
// stop reading it.
func TestDanglingProfilesCountWhatTheyList(t *testing.T) {
	var accounts []unenforcedCandidate
	for _, n := range []string{"a1", "b2", "c3"} {
		accounts = append(accounts, unenforcedCandidate{Username: n, Managed: true, DanglingProfile: "p"})
	}
	got := checkUnenforced(time.Unix(1700000000, 0).UTC(), accounts)
	if !strings.HasPrefix(got.Detail, "3 enabled account(s)") {
		t.Errorf("the count is wrong: %s", got.Detail)
	}
	if !strings.Contains(got.Fix, "plock user assign") {
		t.Errorf("no remedy offered: %s", got.Fix)
	}
}

// Report.Field aligns to the widest label seen SO FAR, which is the contract the type
// documents: a report can be extended by a helper that does not know what else is on the
// page, and the price is that an early field is not pushed right by a later, longer one.
//
// Asserting a shared column instead would be asserting a contract the type deliberately
// does not offer -- and it is also the assertion that made CM4 look like a gap. CM4 deletes
// the `pad < len(label)` guard and survives, provably: Go's %-*s never truncates, so the
// rendered width is max(pad, len(label)) either way, and max(max(w,L), L) == max(w, L). The
// guard is defensive against a formatter that did truncate, not load-bearing against this
// one. It is recorded as equivalent in scripts/mutations/cmd.json rather than tested.
func TestFieldAlignsToTheWidestLabelSoFar(t *testing.T) {
	r := &cli.Report{}
	r.Field("name", "casey")
	r.Field("a much longer label", "v")
	out := r.String()

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), out)
	}
	// The second label is the widest one, so its own value sits one space after it.
	if want := "a much longer label v"; lines[1] != want {
		t.Errorf("line 2 = %q, want %q", lines[1], want)
	}
	// The first is padded to the width known at the time it was written -- "name" plus the
	// width the report had reached, which is its own length.
	if want := "name casey"; lines[0] != want {
		t.Errorf("line 1 = %q, want %q", lines[0], want)
	}

	// A long label early DOES push later fields right. That is the half of the contract
	// a caller can rely on, and it is what makes the report readable when the widest field
	// comes first. Asserted by column position rather than a literal run of spaces, so the
	// test says what is true instead of quoting the formatter.
	r2 := &cli.Report{}
	const wide = "widest label here"
	r2.Field(wide, "x")
	r2.Field("id", 7)
	lines2 := strings.Split(strings.TrimSuffix(r2.String(), "\n"), "\n")
	if len(lines2) != 2 {
		t.Fatalf("got %d lines, want 2:\n%q", len(lines2), r2.String())
	}
	firstCol := strings.Index(lines2[0], "x")
	secondCol := strings.Index(lines2[1], "7")
	if firstCol < 0 || secondCol < 0 {
		t.Fatalf("a value is missing:\n%q", r2.String())
	}
	if firstCol != secondCol {
		t.Errorf("values start at columns %d and %d; a later narrow field must line up with the widest label:\n%q",
			firstCol, secondCol, r2.String())
	}
	if want := len(wide) + 1; firstCol != want {
		t.Errorf("the column is %d, want %d (the widest label plus one space)", firstCol, want)
	}
}
