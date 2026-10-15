package platform

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// The rule these pin: the desktop backend runs what the resolution returned.
//
// The attack the resolution closes is not that gdbus is missing. It is that a bare name in argv
// is resolved against the PATH handed to the child, and the child is the account being locked
// out. Put a script called gdbus in a directory that account can write and that sits on the
// daemon's PATH, and the child chooses what the daemon executes -- including a binary that prints
// "()" and exits 0 without touching a bus. That is precisely the reply the void-reply check is
// looking for, so the backend reports success, the chain stops, and the trail records a lock that
// never happened.

func TestDesktopLockRunsTheResolvedProgramNotAName(t *testing.T) {
	stubGdbusPath(t, stubGdbus)

	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if isProgram(argv, "gdbus") {
			return []byte("()\n"), nil
		}
		return nil, nil
	}}
	b, err := newRegistered(t, "kde", r)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Lock(context.Background(), desktopSession("2")); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	calls := r.Calls()
	if len(calls) == 0 {
		t.Fatal("nothing was run")
	}
	if calls[0].argv[0] != stubGdbus {
		t.Errorf("argv[0] = %q, want the resolved path %q", calls[0].argv[0], stubGdbus)
	}
	// The failure this closes, stated as an assertion rather than left to the line above:
	// a bare name is what lets the child decide. Checked separately so a regression that
	// resolves to the right *file* but still passes a bare name is still caught.
	if calls[0].argv[0] == gdbusProgram {
		t.Errorf("argv[0] is the bare name %q, so exec resolves it against the child's PATH", gdbusProgram)
	}
	if !filepath.IsAbs(calls[0].argv[0]) {
		t.Errorf("argv[0] = %q is not absolute; a relative path is still resolved against PATH", calls[0].argv[0])
	}
}

// The environment the child runs with still carries PATH, because dropping it would break the
// very resolution we just pinned. So the protection cannot be "the child has no PATH" and this
// test says so out loud: the defense is that the path was chosen before the child was involved.
func TestDesktopLockStillPassesPathToTheChild(t *testing.T) {
	stubGdbusPath(t, stubGdbus)

	var env []string
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		return []byte("()\n"), nil
	}}
	b, err := newRegistered(t, "kde", r)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Lock(context.Background(), desktopSession("2")); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	for _, c := range r.Calls() {
		if isProgram(c.argv, "gdbus") {
			env = c.env
			break
		}
	}
	if env == nil {
		t.Fatal("no gdbus call was recorded")
	}
	if !hasEnv(env, "PATH=") {
		t.Error("PATH was dropped from the child's environment; gdbus needs it to be resolvable at all")
	}
}

// When the tool cannot be found, Lock must say that. Building an argv around a path the
// resolution never produced would hand the runner an empty argv[0], and an empty program name is
// a different, vaguer failure than "gdbus is not runnable".
func TestDesktopLockRefusesWhenTheToolCannotBeResolved(t *testing.T) {
	stubGdbusMissing(t)

	r := &fakeRunner{}
	b, err := newRegistered(t, "kde", r)
	if err != nil {
		t.Fatal(err)
	}
	err = b.Lock(context.Background(), desktopSession("2"))
	if err == nil {
		t.Fatal("Lock reported success with no gdbus to run")
	}
	if !strings.Contains(err.Error(), gdbusProgram) {
		t.Errorf("the refusal should name the tool it could not find, got: %v", err)
	}
	if got := len(r.Calls()); got != 0 {
		t.Errorf("%d subprocess(es) were run despite the tool not resolving: %v", got, r.Argvs())
	}
}

// Resolution happens once per Lock, not once per bus name. The kde backend tries several
// service names in order, and re-resolving between them would mean the second attempt could
// exec a different binary than the first.
func TestDesktopLockResolvesOnceAcrossEveryBusName(t *testing.T) {
	stubGdbusPath(t, stubGdbus)

	var resolutions int
	prev := lookPath
	lookPath = func(name string) (string, error) {
		resolutions++
		return prev(name)
	}
	t.Cleanup(func() { lookPath = prev })

	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if isProgram(argv, "gdbus") {
			return []byte("()\n"), nil
		}
		return nil, nil
	}}
	b, err := newRegistered(t, "kde", r)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Lock(context.Background(), desktopSession("2")); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if resolutions != 1 {
		t.Errorf("gdbus was resolved %d time(s), want 1", resolutions)
	}
}

func hasEnv(env []string, prefix string) bool {
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}
