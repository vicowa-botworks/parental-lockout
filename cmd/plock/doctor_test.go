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
)

// fakeChain is the seam doctor reports through. The verdicts are supplied per test so
// the assertions say what doctor concluded, not what this machine has installed.
type fakeChain struct {
	names    []string
	unknown  []string
	statuses []platform.BackendStatus

	// sessions is the live session set the unenforced check sees. Empty by default, which
	// models "nobody is logged in" -- the neutral answer for the tests that are only about
	// backends, and deliberately not an error, because an error here would make every one of
	// those tests depend on how the join reports a failure.
	sessions    []platform.Session
	sessionsErr error
	lockErr     error
}

func (f *fakeChain) Names() []string   { return f.names }
func (f *fakeChain) Unknown() []string { return f.unknown }
func (f *fakeChain) Probe(context.Context) []platform.BackendStatus {
	return f.statuses
}
func (f *fakeChain) Sessions(context.Context) ([]platform.Session, error) {
	return f.sessions, f.sessionsErr
}

// lockErr is what --try reports. A var rather than a recorder because the tests for --try
// assert on what the operator is told, and only one lock happens per test.
func (f *fakeChain) Lock(context.Context, platform.Session) error { return f.lockErr }

// withChain installs a chain builder for one test and puts the real one back after.
func withChain(t *testing.T, cv chainView, err error) {
	t.Helper()
	saved := buildChain
	buildChain = func(config.Settings) (chainView, error) { return cv, err }
	t.Cleanup(func() { buildChain = saved })
}

// withDaemon states what the init system will answer. Without it every doctor test asserts
// on whether the machine running the test happens to have plock installed as a service,
// which is the shape of test that passes on the author's box and fails everywhere else.
func withDaemon(t *testing.T, state string, known bool, err error) {
	t.Helper()
	saved := systemdProbe
	systemdProbe = func() (string, bool, error) { return state, known, err }
	t.Cleanup(func() { systemdProbe = saved })
}

// writeConfig drops a minimal readable document so runDoctor gets past Load and the
// test is really exercising the report.
func writeConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDoctorReportsWhichBackendWouldBeUsed(t *testing.T) {
	withChain(t, &fakeChain{
		names: []string{"logind", "kde", "gnome"},
		statuses: []platform.BackendStatus{
			{Name: "logind", Available: false, Detail: "loginctl not found"},
			{Name: "kde", Available: true, Detail: "calls org.freedesktop.ScreenSaver.Lock"},
			{Name: "gnome", Available: true, Detail: "calls org.freedesktop.ScreenSaver.Lock"},
		},
	}, nil)
	withDaemon(t, "active", true, nil)

	code, out, errOut := runCmd(t, "doctor", "--config", writeConfig(t))
	if code != exitOK {
		t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, exitOK, out, errOut)
	}
	if !strings.Contains(out, "would try first: kde") {
		t.Errorf("did not name the first usable backend:\n%s", out)
	}
	// Configured order, not alphabetical: which backend is tried first is the whole
	// behaviour, and a report that re-sorts it describes a different machine.
	if i, j := strings.Index(out, "logind"), strings.Index(out, "kde"); i < 0 || j < 0 || i > j {
		t.Errorf("backends are not in configured order:\n%s", out)
	}
	if !strings.Contains(out, "unusable") || !strings.Contains(out, "loginctl not found") {
		t.Errorf("an unusable backend must be labelled and say why:\n%s", out)
	}
}

// A misspelled backend has to be named in the report body, not only in the stderr
// warning. Two reasons, both about how this output is actually used: the body is what
// gets pasted into a bug report, and stderr is the first thing a script redirects away.
//
// Both halves of the table matter. The common case is a typo sitting next to a backend
// that does resolve, so a line printed only when nothing at all works is a line that
// never fires in practice; and the all-broken case is the one a test would think to
// cover, which is why the assertion is the PHRASE and not merely the name — every
// configured backend is listed by name anyway, so a test looking for "nope" passes with
// the line deleted. Moving the print behind "something is usable" survived the whole
// suite once, on exactly that confusion.
func TestDoctorNamesUnknownBackendsInTheBody(t *testing.T) {
	cases := []struct {
		name     string
		names    []string
		unknown  []string
		statuses []platform.BackendStatus
		want     string
		wantCode int
	}{
		{
			name:    "alongside a backend that works",
			names:   []string{"kde", "kdeeen"},
			unknown: []string{"kdeeen"},
			statuses: []platform.BackendStatus{
				{Name: "kde", Available: true, Detail: "calls org.freedesktop.ScreenSaver.Lock"},
			},
			want:     `names kdeeen, which this binary does not recognise`,
			wantCode: exitOK,
		},
		{
			name:    "when nothing is usable",
			names:   []string{"nope"},
			unknown: []string{"nope"},
			statuses: []platform.BackendStatus{
				{Name: "nope", Available: false, Detail: "no such backend"},
			},
			want:     `names nope, which this binary does not recognise`,
			wantCode: exitFailure,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withChain(t, &fakeChain{names: tc.names, unknown: tc.unknown, statuses: tc.statuses}, nil)
			withDaemon(t, "active", true, nil)

			code, out, errOut := runCmd(t, "doctor", "--config", writeConfig(t))
			if code != tc.wantCode {
				t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tc.wantCode, out, errOut)
			}
			// stdout specifically: a test that accepts either stream is satisfied by
			// the stderr warning alone.
			if !strings.Contains(out, tc.want) {
				t.Errorf("the report body never says %q\nstdout:\n%s\nstderr:\n%s", tc.want, out, errOut)
			}
		})
	}
}

// "would try first" is only honest next to the sentence explaining that a backend is
// finished with once its lock is CONFIRMED. On its own the line predicts which backend
// will do the locking, which is the overclaim this wording replaced; and because
// deleting the explanation changes nothing any test was checking, an honest sentence
// here is one refactor away from being honest-looking output again.
func TestDoctorExplainsThatFirstIsNotTheBackendThatLocks(t *testing.T) {
	withChain(t, &fakeChain{
		names: []string{"logind", "kde"},
		statuses: []platform.BackendStatus{
			{Name: "logind", Available: true, Detail: "loginctl found"},
			{Name: "kde", Available: true, Detail: "calls org.freedesktop.ScreenSaver.Lock"},
		},
	}, nil)
	withDaemon(t, "active", true, nil)

	code, out, errOut := runCmd(t, "doctor", "--config", writeConfig(t))
	if code != exitOK {
		t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, exitOK, out, errOut)
	}
	for _, want := range []string{
		"would try first: logind",
		"only once its lock is confirmed",
		"may come from a later one",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the report is missing %q:\n%s", want, out)
		}
	}
}

// The failure this command exists for: nothing can lock, and the daemon would run
// forever enforcing nothing.
func TestDoctorFailsLoudlyWhenNothingCanLock(t *testing.T) {
	withChain(t, &fakeChain{
		names: []string{"logind", "kde"},
		statuses: []platform.BackendStatus{
			{Name: "logind", Available: false, Detail: "no loginctl"},
			{Name: "kde", Available: false, Detail: "no gdbus"},
		},
	}, nil)
	withDaemon(t, "active", true, nil)

	code, out, _ := runCmd(t, "doctor", "--config", writeConfig(t))
	if code == exitOK {
		t.Fatalf("exit 0 while no backend can lock; a script must be able to test for this\n%s", out)
	}
	if !strings.Contains(out, "would try: nothing") {
		t.Errorf("must say plainly that nothing can lock:\n%s", out)
	}
	if !strings.Contains(out, "enforced by nobody") {
		t.Errorf("must state the consequence, not just the fact:\n%s", out)
	}
}

func TestDoctorReportsUnknownBackendNames(t *testing.T) {
	withChain(t, &fakeChain{
		names:    []string{"logind"},
		unknown:  []string{"kde-x"},
		statuses: []platform.BackendStatus{{Name: "logind", Available: true, Detail: "ok"}},
	}, errors.New(`unknown lock backend "kde-x"`))
	withDaemon(t, "active", true, nil)

	code, out, errOut := runCmd(t, "doctor", "--config", writeConfig(t))
	if code != exitOK {
		t.Fatalf("exit %d, want %d — one bad name must not hide the report on the good ones\n%s\n%s",
			code, exitOK, out, errOut)
	}
	// The typo is reported, but the operator still gets the state of the backends that
	// do exist. That is why buildChain keeps the chain alongside its error.
	if !strings.Contains(errOut, "kde-x") {
		t.Errorf("the unknown name is not surfaced on stderr: %s", errOut)
	}
	if !strings.Contains(out, "would try first: logind") {
		t.Errorf("report lost the usable backend:\n%s", out)
	}
}

func TestDoctorReportsAConfigItCannotRead(t *testing.T) {
	// No chain installed at all: a config that will not load must stop every check that
	// would otherwise describe settings nobody wrote.
	//
	// It no longer stops the whole command. The original shape returned here, which left the
	// operator's actual next question -- "is the daemon even running?" -- unanswered, and the
	// two checks that answer it need no config at all. So the config-dependent checks are
	// reported as skipped rather than run against defaults, which is what this test is really
	// guarding against.
	withDaemon(t, "active", true, nil)
	path := filepath.Join(t.TempDir(), "absent.json")
	code, out, errOut := runCmd(t, "doctor", "--config", path)
	if code != exitFailure {
		t.Fatalf("exit %d, want %d", code, exitFailure)
	}
	if !strings.Contains(errOut, path) {
		t.Errorf("error does not name the file it failed on: %s", errOut)
	}
	// The checks that need no config still ran.
	for _, want := range []string{"daemon", "config.file"} {
		if !strings.Contains(out, want) {
			t.Errorf("a config failure suppressed the %s check, which does not need a config:\n%s", want, out)
		}
	}
	// And the ones that do are named as not checked, never as having passed.
	for _, want := range []string{"backends", "unenforced"} {
		line := checkLine(out, want)
		if !strings.Contains(line, string(verdictSkip)) {
			t.Errorf("%s reported %q, want it to say it was not checked:\n%s", want, line, out)
		}
	}
	// Nothing may claim success for a check that could not run.
	if strings.Contains(out, "nothing failed") {
		t.Errorf("a run whose config could not be read reported no failures:\n%s", out)
	}
}

// checkLine pulls the report line for one check name out of the text output.
func checkLine(out, name string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, " "+name+" ") {
			return line
		}
	}
	return ""
}

func TestDoctorRejectsBadArguments(t *testing.T) {
	for _, args := range [][]string{
		{"doctor", "--nonsense"},
		{"doctor", "extra"},
	} {
		code, _, errOut := runCmd(t, args...)
		if code != exitUsage {
			t.Errorf("plock %v: exit %d, want %d\n%s", args, code, exitUsage, errOut)
		}
		if !strings.Contains(errOut, "usage: plock doctor") {
			t.Errorf("plock %v: no usage on stderr:\n%s", args, errOut)
		}
	}
}

// The stub synopsis once promised flags this command did not have, and this test pinned
// their absence to keep the help from lying. Both flags have since landed, so the pin had
// to be turned around rather than deleted: the worry was never "--try must not exist", it
// was "the help and the FlagSet must agree". A lie runs both ways -- advertising a flag
// that does nothing, and hiding one that works -- so both directions are asserted now.
//
// The hiding direction is the one that bit for real: --json and --try were implemented,
// worked, and were absent from `plock doctor -h`, which no test noticed until the
// package-wide flag/documentation test was added.
func TestDoctorHelpIsTheRealUsage(t *testing.T) {
	code, out, _ := runCmd(t, "doctor", "-h")
	if code != exitOK {
		t.Fatalf("exit %d, want %d", code, exitOK)
	}
	if !strings.Contains(out, "usage: plock doctor") {
		t.Errorf("help text:\n%s", out)
	}

	// The real declared set, from the same AST walk the package-wide test uses. A fresh
	// flag.NewFlagSet here would declare nothing at all -- the flags are declared inside
	// runDoctor, on a FlagSet this test never sees -- and an empty set would make both
	// loops below pass by inspecting nothing.
	names := declaredFlags(t)["doctorUsage"]
	if len(names) == 0 {
		t.Fatal("no flags attributed to doctorUsage, so neither assertion below can mean anything")
	}
	declared := map[string]bool{}
	for _, n := range names {
		declared["--"+n] = true
	}

	// Every declared flag is advertised.
	for name := range declared {
		if !strings.Contains(out, name) {
			t.Errorf("doctor implements %s but its help does not mention it:\n%s", name, out)
		}
	}
	// Nothing is advertised that the command does not implement. The scan is limited to
	// the synopsis and the flag bullets, because the body legitimately names a flag while
	// explaining a rule about it ("--try cannot be combined with --json").
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "usage:") && !strings.HasPrefix(trimmed, "--") {
			continue
		}
		for _, tok := range strings.Fields(trimmed) {
			tok = strings.Trim(tok, "[]")
			if !strings.HasPrefix(tok, "--") {
				continue
			}
			if !declared[tok] {
				t.Errorf("doctor help advertises %s, which the command does not implement:\n%s", tok, out)
			}
		}
	}
}

// The seam that makes the report testable also hides the production wiring, so the
// wiring gets its own test. Without it, buildChain could be wrong in a way no test can
// see: every report test replaces it.
func TestBuildChainProductionWiring(t *testing.T) {
	t.Run("a real backend builds a chain with no error", func(t *testing.T) {
		cv, err := buildChain(config.Settings{LockBackends: []string{"logind"}})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if cv == nil {
			t.Fatal("nil chain for a backend that exists")
		}
		if got := cv.Names(); len(got) != 1 || got[0] != "logind" {
			t.Errorf("Names() = %v, want [logind]", got)
		}
	})

	// The whole reason doctor keeps the chain alongside the error: one bad name must
	// not cost the operator the report on the backends that do exist.
	t.Run("one bad name keeps the good backends", func(t *testing.T) {
		cv, err := buildChain(config.Settings{LockBackends: []string{"logind", "nope"}})
		if err == nil {
			t.Fatal("err = nil, want the unknown name reported")
		}
		if cv == nil {
			t.Fatal("chain dropped because of one bad name; doctor would then report nothing at all")
		}
	})

	// New never returns a nil chain, so this is the shape doctor actually sees when
	// every configured name is wrong.
	t.Run("every name wrong still yields a chain to report on", func(t *testing.T) {
		cv, err := buildChain(config.Settings{LockBackends: []string{"nope"}})
		if err == nil {
			t.Fatal("err = nil for a config with no usable backend")
		}
		if cv == nil {
			t.Fatal("nil chain; doctor would print nothing for the commonest misconfiguration")
		}
		if got := cv.Unknown(); len(got) != 1 || got[0] != "nope" {
			t.Errorf("Unknown() = %v, want [nope]", got)
		}
	})
}

// The commonest misconfiguration, end to end through the real builder: every name in
// lockBackends is wrong. The operator gets the typo named and a non-zero exit, not a
// silent "would try: nothing".
func TestDoctorReportsAConfigWhoseBackendsAllMiss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{"version":1,"settings":{"lockBackends":["nope","also-nope"]}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runCmd(t, "doctor", "--config", path)
	if code != exitFailure {
		t.Fatalf("exit %d, want %d\n%s", code, exitFailure, out)
	}
	if !strings.Contains(out, "would try: nothing") {
		t.Errorf("must say nothing can lock:\n%s", out)
	}
	// stdout, specifically. The stderr warning is not the report: it is the first
	// thing a script redirects away, and a test that accepts either stream is
	// satisfied by it alone — which is how the placement of this line went unpinned
	// once a change moved the print behind "nothing is usable".
	for _, name := range []string{"nope", "also-nope"} {
		if !strings.Contains(out, name) {
			t.Errorf("the bad name %q is not in the report body\nstdout:\n%s\nstderr:\n%s", name, out, errOut)
		}
	}
}

// A writer that fails is the only way to reach the error paths, and they matter because
// a silently-truncated report is the failure mode this command is meant to prevent.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("device full") }

func TestDoctorReportsAWriteFailure(t *testing.T) {
	withChain(t, &fakeChain{
		names:    []string{"logind"},
		statuses: []platform.BackendStatus{{Name: "logind", Available: true, Detail: "ok"}},
	}, nil)
	withDaemon(t, "active", true, nil)
	if code := runDoctor(failWriter{}, failWriter{}, []string{"--config", writeConfig(t)}); code != exitFailure {
		t.Errorf("exit %d, want %d when the report cannot be written", code, exitFailure)
	}

	var errOut strings.Builder
	if code := runDoctor(failWriter{}, &errOut, []string{"-h"}); code != exitFailure {
		t.Errorf("exit %d, want %d when help cannot be written", code, exitFailure)
	}
}
