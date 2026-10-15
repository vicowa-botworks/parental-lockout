package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// scriptFile makes an executable file the backend is allowed to be pointed at, and returns
// its absolute path. Absolute is the whole rule, so every fixture has to be one.
func scriptFile(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return p
}

// listerFake supplies sessions without a loginctl on PATH.
type listerFake struct {
	sessions []Session
	err      error
	calls    int
}

func (l *listerFake) Sessions(context.Context) ([]Session, error) {
	l.calls++
	return l.sessions, l.err
}

// execFake records the resolved program and argv the backend asked to run.
type execFake struct {
	runs []struct {
		path string
		argv []string
	}
	err error
}

func (e *execFake) exec(_ context.Context, path string, argv []string) error {
	e.runs = append(e.runs, struct {
		path string
		argv []string
	}{path, append([]string(nil), argv...)})
	return e.err
}

// The backend must be reachable by the name a config writes, or the whole ticket is
// plumbing nobody can select.
func TestCommandBackendIsRegisteredUnderItsConfigName(t *testing.T) {
	if _, ok := registry[CommandID]; !ok {
		t.Fatalf("no backend registered as %q; registered: %s", CommandID, strings.Join(registeredIDs(), ", "))
	}
	b, err := newRegisteredWith(t, CommandID, &fakeRunner{}, config.Settings{LockCommandPath: scriptFile(t, "lock")})
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if got := b.Name(); got != CommandID {
		t.Errorf("Name() = %q, want %q -- doctor and the audit trail print this", got, CommandID)
	}
}

// A bare name is refused rather than resolved. This is the rule #53 established for the
// shipped lock binaries, applied to the binary an operator names, and it is asserted here
// as well as in config.Validate because New is reachable with a Settings nobody validated.
func TestACommandPathThatIsNotAbsoluteIsRefused(t *testing.T) {
	for _, path := range []string{"locktest.sh", "./locktest.sh", "../bin/lock", "bin/lock"} {
		_, err := newCommand(config.Settings{LockCommandPath: path}, &listerFake{})
		if err == nil {
			t.Errorf("newCommand accepted %q; it would be resolved against the daemon's working directory", path)
			continue
		}
		if !strings.Contains(err.Error(), "absolute") {
			t.Errorf("newCommand(%q) said %v; the refusal must say the path has to be absolute", path, err)
		}
		if !strings.Contains(err.Error(), "PATH") {
			t.Errorf("newCommand(%q) said %v; the refusal must say it will not fall back to PATH, so the operator does not go and export one", path, err)
		}
	}
}

// An absent path is the other half: lockBackends named the backend, so "no path" cannot
// quietly mean "a backend that locks nothing".
func TestACommandBackendWithNoPathIsAnErrorNotANoOp(t *testing.T) {
	for _, path := range []string{"", "   "} {
		b, err := newCommand(config.Settings{LockCommandPath: path}, &listerFake{})
		if err == nil {
			t.Errorf("newCommand(path=%q) returned %v with no error; a chain would carry a backend that locks nothing", path, b)
			continue
		}
		if !strings.Contains(err.Error(), "lockCommandPath") {
			t.Errorf("the refusal does not name the field to fix: %v", err)
		}
		// The two refusals have to be told apart. Both messages name the same field, so
		// asserting only the field lets the absolute-path case answer for the empty one --
		// which is what happened when this branch was mutated, and it is the masking
		// pattern the audit battery hit repeatedly. An operator whose path is missing does
		// not need "must be an absolute path"; they need "you did not give me one".
		if strings.Contains(err.Error(), "absolute") {
			t.Errorf("an empty path was refused with the absolute-path message, which tells an "+
				"operator to fix a path they never wrote: %v", err)
		}
	}
}

// The path is trimmed for the check but the checked value is what runs, so a path with
// trailing whitespace is refused as empty rather than exec'd with the space kept.
func TestTheCommandPathIsTrimmedBeforeItIsJudged(t *testing.T) {
	good := scriptFile(t, "lock")
	b, err := newCommand(config.Settings{LockCommandPath: "  " + good + " "}, &listerFake{})
	if err != nil {
		t.Fatalf("a padded absolute path should be usable: %v", err)
	}
	if b.path != good {
		t.Errorf("path = %q, want the padding removed so exec sees exactly the file", b.path)
	}
}

// Lock must really exec, with the program at argv[0] and no shell in between.
func TestLockRunsTheConfiguredProgramAsOneArgv(t *testing.T) {
	path := scriptFile(t, "plock-locktest")
	ef := &execFake{}
	b, err := newCommand(config.Settings{
		LockCommandPath: path,
		LockCommandArgv: []string{"--now"},
	}, &listerFake{})
	if err != nil {
		t.Fatalf("newCommand: %v", err)
	}
	b.exec = ef.exec

	if err := b.Lock(context.Background(), desktopSession("2")); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if len(ef.runs) != 1 {
		t.Fatalf("%d runs, want exactly 1", len(ef.runs))
	}
	run := ef.runs[0]
	if run.path != path {
		t.Errorf("program = %q, want the configured absolute path %q", run.path, path)
	}
	// No token appeared, so the session id is appended. See the Lock comment.
	want := []string{"--now", "2"}
	if strings.Join(run.argv, "|") != strings.Join(want, "|") {
		t.Errorf("argv = %v, want %v", run.argv, want)
	}
}

// Each token substitutes, and a token in the middle of an argument works too -- an
// operator writing --session=%SESSION% is not doing anything exotic.
func TestEverySessionFieldSubstitutes(t *testing.T) {
	s := desktopSession("2")
	s.TTY = "tty2"
	for _, tc := range []struct{ token, want string }{
		{"%SESSION%", "2"},
		{"%USER%", "kid"},
		{"%UID%", fmt.Sprintf("%d", desktopUID)},
		{"%SEAT%", "seat0"},
		{"%TTY%", "tty2"},
	} {
		ef := &execFake{}
		b, err := newCommand(config.Settings{
			LockCommandPath: scriptFile(t, "lock"),
			LockCommandArgv: []string{"--x=" + tc.token},
		}, &listerFake{})
		if err != nil {
			t.Fatalf("newCommand: %v", err)
		}
		b.exec = ef.exec
		if err := b.Lock(context.Background(), s); err != nil {
			t.Fatalf("Lock: %v", err)
		}
		if got := strings.Join(ef.runs[0].argv, " "); got != "--x="+tc.want {
			t.Errorf("%s substituted to %q, want --x=%s", tc.token, got, tc.want)
		}
	}
}

// A token whose field is empty becomes an empty argument rather than a dropped one. The
// argument count is part of what a script parses, so silently removing one would shift
// every later argument into a different slot.
func TestAnEmptyFieldSubstitutesToEmptyAndKeepsTheArgument(t *testing.T) {
	s := desktopSession("2")
	s.Seat = ""
	ef := &execFake{}
	b, _ := newCommand(config.Settings{
		LockCommandPath: scriptFile(t, "lock"),
		LockCommandArgv: []string{"--seat", "%SEAT%", "--user", "%USER%"},
	}, &listerFake{})
	b.exec = ef.exec
	if err := b.Lock(context.Background(), s); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	got := strings.Join(ef.runs[0].argv, "|")
	if want := "--seat||--user|kid"; got != want {
		t.Errorf("argv = %q, want %q -- the empty seat must stay as an empty argument", got, want)
	}
}

// When the argv does name a token, nothing is appended: the operator said where the
// identity goes, and a second copy would be an argument the script never asked for.
func TestNothingIsAppendedWhenATokenWasUsed(t *testing.T) {
	ef := &execFake{}
	b, _ := newCommand(config.Settings{
		LockCommandPath: scriptFile(t, "lock"),
		LockCommandArgv: []string{"--session", "%SESSION%"},
	}, &listerFake{})
	b.exec = ef.exec
	if err := b.Lock(context.Background(), desktopSession("2")); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if got := strings.Join(ef.runs[0].argv, "|"); got != "--session|2" {
		t.Errorf("argv = %q, want --session|2 with nothing appended", got)
	}
}

// A non-zero exit is the backend's answer, with the program named. This is the property
// that separates the backend from Mock: it can fail.
func TestAFailedProgramIsReportedWithItsPath(t *testing.T) {
	path := scriptFile(t, "lock")
	boom := errors.New("exit status 3")
	ef := &execFake{err: boom}
	b, _ := newCommand(config.Settings{LockCommandPath: path}, &listerFake{})
	b.exec = ef.exec

	err := b.Lock(context.Background(), desktopSession("2"))
	if err == nil {
		t.Fatal("Lock reported success for a program that failed")
	}
	if !errors.Is(err, boom) {
		t.Errorf("the runner's error is not reachable with errors.Is: %v", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("the failure does not name the program that failed: %v", err)
	}
}

// Sessions delegates. A backend that listed nothing would make the enforcement loop find
// no sessions and enforce nothing while every component reported health.
func TestSessionsDelegatesToLogind(t *testing.T) {
	want := []Session{desktopSession("2")}
	lf := &listerFake{sessions: want}
	b, _ := newCommand(config.Settings{LockCommandPath: scriptFile(t, "lock")}, lf)
	b.exec = (&execFake{}).exec

	got, err := b.Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(got) != 1 || got[0].ID != "2" {
		t.Errorf("Sessions = %v, want the lister's one session", got)
	}
	if lf.calls != 1 {
		t.Errorf("the lister was called %d times, want 1", lf.calls)
	}
}

// The lister's failure must come back as a failure. Swallowing it would look exactly like
// "nobody is logged in", which is the fail-open reading.
func TestAListingFailureIsNotReportedAsNoSessions(t *testing.T) {
	boom := errors.New("loginctl not found")
	b, _ := newCommand(config.Settings{LockCommandPath: scriptFile(t, "lock")}, &listerFake{err: boom})

	got, err := b.Sessions(context.Background())
	if err == nil {
		t.Fatalf("Sessions returned %v with no error; the loop would read that as an empty house", got)
	}
	if !errors.Is(err, boom) {
		t.Errorf("errors.Is cannot reach the cause: %v", err)
	}
}

// The registration wires the real logind as the lister, not a nil that only the tests
// would notice.
func TestTheRegisteredConstructorWiresLogindAsTheLister(t *testing.T) {
	b, err := newRegisteredWith(t, CommandID, &fakeRunner{}, config.Settings{LockCommandPath: scriptFile(t, "lock")})
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	cb := b.(*commandBackend)
	if cb.lister == nil {
		t.Fatal("the registered backend has no session lister; Sessions would panic on the first tick")
	}
	if _, ok := cb.lister.(*logind); !ok {
		t.Errorf("lister is %T, want *logind", cb.lister)
	}
	if cb.exec == nil {
		t.Fatal("the registered backend has no exec; every Lock would fail")
	}
}

// The registration must refuse the same things the constructor does, or a config can build
// a broken backend through the door production actually uses.
func TestTheRegisteredConstructorRefusesABareName(t *testing.T) {
	_, err := newRegisteredWith(t, CommandID, &fakeRunner{}, config.Settings{LockCommandPath: "locktest.sh"})
	if err == nil {
		t.Fatal("the registry accepted a bare program name")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Errorf("refusal does not explain the rule: %v", err)
	}
}

// Probe must not run the program. `plock doctor` is what an operator runs when they
// suspect locking is broken, and a probe that executed the lock command would lock their
// screen at the moment they asked whether locking works.
func TestProbeNeverRunsTheProgram(t *testing.T) {
	ef := &execFake{}
	b, _ := newCommand(config.Settings{LockCommandPath: scriptFile(t, "lock")}, &listerFake{})
	b.exec = ef.exec

	if got := b.Probe(context.Background()); len(got) == 0 {
		t.Fatal("Probe reported nothing")
	}
	if len(ef.runs) != 0 {
		t.Errorf("Probe ran the program %d time(s); a probe must never lock anything", len(ef.runs))
	}
}

func TestProbeAvailabilityFollowsTheProgramNotTheListing(t *testing.T) {
	t.Run("a missing program is unavailable", func(t *testing.T) {
		b, _ := newCommand(config.Settings{
			LockCommandPath: filepath.Join(t.TempDir(), "not-here"),
		}, &listerFake{sessions: []Session{desktopSession("2")}})
		b.exec = (&execFake{}).exec
		st := b.Probe(context.Background())[0]
		if st.Available {
			t.Error("Probe reported available for a program that does not exist")
		}
		if !strings.Contains(st.Detail, "not-here") {
			t.Errorf("the detail does not name the missing file: %q", st.Detail)
		}
	})

	// The reverse direction, and the one #25 item 4 is about: the listing being broken
	// must not be reported as the program being broken, because those are different
	// things to fix. Availability follows the lock half; the listing is said underneath.
	t.Run("a broken listing does not blame the program", func(t *testing.T) {
		path := scriptFile(t, "lock")
		b, _ := newCommand(config.Settings{LockCommandPath: path},
			&listerFake{err: errors.New("no bus")})
		b.exec = (&execFake{}).exec
		st := b.Probe(context.Background())[0]
		if !st.Available {
			t.Errorf("Probe reported unavailable because the listing failed; the program is fine: %q", st.Detail)
		}
		if !strings.Contains(st.Detail, "sessions unavailable") {
			t.Errorf("the listing failure is not reported: %q", st.Detail)
		}
		if !strings.Contains(st.Detail, "no bus") {
			t.Errorf("the listing failure lost its reason: %q", st.Detail)
		}
	})

	t.Run("a directory is not a program", func(t *testing.T) {
		b, _ := newCommand(config.Settings{LockCommandPath: t.TempDir()}, &listerFake{})
		b.exec = (&execFake{}).exec
		st := b.Probe(context.Background())[0]
		if st.Available {
			t.Error("Probe reported a directory available")
		}
		// The exact message, not the word "directory". Go names a subtest's temp
		// directory after the subtest, so this fixture's own path contains the string
		// "directory" and a substring check on it passes no matter what the code says --
		// the assertion was vacuous and a mutation of the IsDir branch proved it.
		if !strings.Contains(st.Detail, "is a directory, not a program") {
			t.Errorf("the detail does not say what is wrong with the path: %q", st.Detail)
		}
	})

	t.Run("a non-executable file is not a program", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "not-executable")
		if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		b, _ := newCommand(config.Settings{LockCommandPath: p}, &listerFake{})
		b.exec = (&execFake{}).exec
		if b.Probe(context.Background())[0].Available {
			t.Error("Probe reported a 0644 file available")
		}
	})
}

// executableFile is what decides availability, and its missing-file case has to stay
// classifiable: ErrBinaryNotFound is the sentinel doctor uses to say "install loginctl"
// rather than "something went wrong", and Probe flattens the error into a Detail string,
// so errors.Is can only be checked here, at the place that still has an error.
func TestTheMissingProgramKeepsTheSentinel(t *testing.T) {
	err := executableFile(filepath.Join(t.TempDir(), "absent"))
	if err == nil {
		t.Fatal("executableFile accepted a file that does not exist")
	}
	if !errors.Is(err, ErrBinaryNotFound) {
		t.Errorf("errors.Is cannot reach ErrBinaryNotFound, so a missing program is no longer "+
			"reportable as a missing program: %v", err)
	}
	// The program is named, the way ExecRunner reports the same failure. A sentinel
	// alone says "something was not found" on a machine with several configured
	// programs. os.ErrNotExist is deliberately NOT asserted: this package's vocabulary
	// for a missing binary is ErrBinaryNotFound, and ExecRunner wraps only that.
	if !strings.Contains(err.Error(), "absent") {
		t.Errorf("the refusal does not name the program that is missing: %v", err)
	}
}

// The detail must say this is not a production backend. The field is documented as a hook
// for tests and a developer's loop, and `plock doctor` is where an operator would
// otherwise conclude a command backend is a normal thing to be running.
func TestProbeSaysThisIsNotAProductionBackend(t *testing.T) {
	b, _ := newCommand(config.Settings{LockCommandPath: scriptFile(t, "lock")}, &listerFake{})
	b.exec = (&execFake{}).exec
	st := b.Probe(context.Background())[0]
	if !strings.Contains(st.Detail, "not a production backend") {
		t.Errorf("Probe detail = %q; it must say the configured command is not a production backend", st.Detail)
	}
}

// The whole security argument rests on this: the backend must not confirm. A subprocess
// exiting 0 says nothing about the screen being locked, and a Confirmer here would put a
// rubber stamp where #29 requires a second signal.
func TestTheCommandBackendIsNotAConfirmer(t *testing.T) {
	b, err := newRegisteredWith(t, CommandID, &fakeRunner{}, config.Settings{LockCommandPath: scriptFile(t, "lock")})
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if c, ok := b.(Confirmer); ok {
		yes, _ := c.LockConfirmed(context.Background(), desktopSession("2"))
		t.Errorf("the command backend implements Confirmer and answered %v; a program exiting 0 is not evidence the screen locked", yes)
	}
	if c, ok := b.(Corroborator); ok {
		t.Errorf("the command backend implements Corroborator (%v); it has no second signal to corroborate with", c.Corroborated())
	}
}

// And the consequence of the above, stated as behaviour rather than left to inference: a
// chain of only this backend reports ErrUnconfirmed, so the audit trail says so instead of
// reporting a confirmed lock.
func TestACommandOnlyChainReportsUnconfirmed(t *testing.T) {
	path := scriptFile(t, "lock")
	b, err := newRegisteredWith(t, CommandID, &fakeRunner{}, config.Settings{LockCommandPath: path})
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	b.(*commandBackend).exec = (&execFake{}).exec

	c := &Chain{backends: []Platform{b}}
	err = c.Lock(context.Background(), desktopSession("2"))
	if !errors.Is(err, ErrUnconfirmed) {
		t.Errorf("Chain.Lock = %v, want ErrUnconfirmed -- the trail must not read as a confirmed lock", err)
	}
}
