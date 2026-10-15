package platform

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// The rule these pin: the desktop backend resolves gdbus once per Lock and hands the runner
// an absolute path, so every bus name in the fallback list is reached through the same file.
//
// What this is NOT, and the earlier version of this file said otherwise: it is not a defence
// against the session user choosing which binary runs. Resolution happens in the daemon's
// process, from the daemon's os.Getenv("PATH"), when exec.CommandContext is constructed --
// before cmd.Env is assigned -- so the PATH handed to the child is never consulted for it.
// Measured with two copies of one tool and the repo's own ordering, cmd.Path is the daemon's
// copy and the daemon's copy is what runs.
//
// The reason to resolve here is specific to this backend and has nothing to do with the
// child: `kde` walks several bus names in order, and a lookup per attempt would be several
// answers to one question, with the second attempt free to exec a different file than the
// first. The exposure that does exist -- a directory the session user can write, on the
// daemon's own PATH -- is not closed by any of this, is identical for loginctl, and is
// written up in docs/security.md as the open decision it is.

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
	// Checked separately from the equality above so that a regression which resolves to
	// the right file but still passes the bare name along is caught by the assertion that
	// states the real consequence: the lookup would then happen inside exec.CommandContext,
	// once per bus name, giving each attempt its own answer.
	if calls[0].argv[0] == gdbusProgram {
		t.Errorf("argv[0] is the bare name %q, so each bus name in the fallback list resolves separately", gdbusProgram)
	}
	if !filepath.IsAbs(calls[0].argv[0]) {
		t.Errorf("argv[0] = %q is not absolute, so it is not the file this backend resolved", calls[0].argv[0])
	}
}

// The child's environment still carries PATH. It has to: gdbus is a real program that looks
// things up at run time, and stripping PATH would break the tool rather than stop an attack.
// Worth pinning because "we resolved the path ourselves" tempts a reader into concluding the
// child's PATH is now load-bearing and should be removed. It is not load-bearing for
// resolution at all -- that happened in the daemon -- which is the whole point.
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
