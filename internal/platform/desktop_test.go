package platform

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
)

func TestSessionEnvTargetsTheSessionsBus(t *testing.T) {
	// The daemon's own environment, including the things a root daemon is likely to
	// have that must never reach a user's session.
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/0")
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/run/user/0/bus")
	t.Setenv("HOME", "/root")
	t.Setenv("LD_PRELOAD", "/evil/lib.so")
	t.Setenv("PYTHONPATH", "/opt/admin")
	t.Setenv("PATH", "/usr/sbin:/usr/bin")
	t.Setenv("LANG", "en_GB.UTF-8")
	t.Setenv("LC_NUMERIC", "de_DE")
	t.Setenv("DISPLAY", ":99")

	env := sessionEnv(1001)
	got := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		got[k] = v
	}

	if want := "/run/user/1001"; got["XDG_RUNTIME_DIR"] != want {
		t.Errorf("XDG_RUNTIME_DIR = %q, want %q — the daemon's is root's and a call that used it fails as a broken desktop", got["XDG_RUNTIME_DIR"], want)
	}
	if want := "unix:path=/run/user/1001/bus"; got["DBUS_SESSION_BUS_ADDRESS"] != want {
		t.Errorf("DBUS_SESSION_BUS_ADDRESS = %q, want %q", got["DBUS_SESSION_BUS_ADDRESS"], want)
	}

	// The allowlist, in both directions. Keeping the two presentation variables is
	// asserted because dropping them changes what a failing command logs; dropping the
	// rest is asserted because that is the security property.
	if got["PATH"] != "/usr/sbin:/usr/bin" {
		t.Errorf("PATH = %q, want it passed through", got["PATH"])
	}
	if got["LANG"] != "en_GB.UTF-8" {
		t.Errorf("LANG = %q, want it passed through", got["LANG"])
	}
	if got["LC_NUMERIC"] != "de_DE" {
		t.Errorf("LC_NUMERIC = %q, want the LC_* family passed through", got["LC_NUMERIC"])
	}
	for _, banned := range []string{"LD_PRELOAD", "PYTHONPATH", "DISPLAY"} {
		if v, ok := got[banned]; ok {
			t.Errorf("%s=%q leaked from the daemon's environment into a session command", banned, v)
		}
	}
	// HOME is set, but to empty: inheriting /root is the failure, and a tool that
	// writes to HOME should write nowhere rather than into a root home.
	if v, ok := got["HOME"]; !ok {
		t.Error("HOME is unset rather than explicitly emptied; an unset HOME still leaves whatever the exec default is")
	} else if v != "" {
		t.Errorf("HOME = %q, want it emptied", v)
	}

	// The precondition for the leak assertions, checked against the environment
	// itself rather than through sessionEnv: a variable the daemon does not actually
	// have cannot demonstrate that nothing leaked, and a guard that measures the
	// precondition with the function under test turns a broken filter into a passing
	// test.
	for _, must := range []string{"LD_PRELOAD", "PYTHONPATH", "DISPLAY", "PATH"} {
		if os.Getenv(must) == "" {
			t.Fatalf("%s is not set in the test environment, so its absence from the result proves nothing", must)
		}
	}
}

func TestSessionEnvIsBuiltFromScratch(t *testing.T) {
	// Nothing in the returned environment may be a slice element the daemon owns.
	// sessionEnv appends to a literal, so this holds by construction — but the day it
	// starts from os.Environ() the appends would write into the daemon's own array, and
	// this is the cheap pin that notices.
	t.Setenv("LANG", "C")
	first := sessionEnv(1001)
	before := append([]string(nil), first...)
	second := sessionEnv(2002)
	if len(second) == 0 {
		t.Fatal("second environment empty")
	}
	for i := range first {
		if first[i] != before[i] {
			t.Fatalf("building a second environment changed the first at %d: %q -> %q", i, before[i], first[i])
		}
	}
}

func TestCheckSessionTargetRefusesUID0(t *testing.T) {
	s := desktopSession("2")
	s.UID = 0
	err := checkSessionTarget(s)
	if err == nil {
		t.Fatal("uid 0 accepted; a privilege drop that targets uid 0 drops nothing and runs the lock as root")
	}
	if !strings.Contains(err.Error(), "uid 0") {
		t.Errorf("error does not name uid 0: %v", err)
	}

	noID := desktopSession("")
	if err := checkSessionTarget(noID); err == nil {
		t.Error("a session with no id was accepted; the id is the whole argument saying which screen to lock")
	}

	if err := checkSessionTarget(desktopSession("2")); err != nil {
		t.Errorf("a normal session refused: %v", err)
	}
}

// lockArgv is the command the backend must produce, spelled out rather than derived
// from the code so a change to the flags has to be made here too.
func lockArgv(dest string) []string {
	return []string{
		"gdbus", "call", "--session",
		"--dest", dest,
		"--object-path", "/ScreenSaver",
		"--method", "org.freedesktop.ScreenSaver.Lock",
	}
}

func TestDesktopLockRunsGdbusAsTheSessionUser(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if argv[0] == "gdbus" {
			return []byte("()\n"), nil
		}
		return nil, nil
	}}
	b, err := newRegistered(t, "kde", r)
	if err != nil {
		t.Fatal(err)
	}
	s := desktopSession("2")
	if err := b.Lock(context.Background(), s); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	calls := r.Calls()
	if len(calls) != 1 {
		t.Fatalf("%d command(s) run, want 1: %v", len(calls), r.Argvs())
	}
	wantArgv(t, [][]string{calls[0].argv}, [][]string{lockArgv("org.kde.screensaver")})

	if calls[0].uid != s.UID {
		t.Errorf("ran as uid %d, want the session uid %d", calls[0].uid, s.UID)
	}
	if len(calls[0].env) == 0 {
		t.Fatal("ran with an empty (inheriting) environment; the daemon's bus is not the session's")
	}
	if !containsEnv(calls[0].env, "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus") {
		t.Errorf("env lacks the session bus address: %v", calls[0].env)
	}
}

func TestDesktopLockRefusesUID0(t *testing.T) {
	r := &fakeRunner{}
	b, err := newRegistered(t, "kde", r)
	if err != nil {
		t.Fatal(err)
	}
	s := desktopSession("2")
	s.UID = 0
	if err := b.Lock(context.Background(), s); err == nil {
		t.Fatal("Lock succeeded for a uid-0 target")
	}
	if len(r.Calls()) != 0 {
		t.Errorf("ran %d command(s) after refusing the target", len(r.Calls()))
	}
}

// A zero exit is not a lock. This is the qdbus failure mode generalised: on Ubuntu
// /usr/bin/qdbus is qtchooser and prints an error to stdout with exit 0.
func TestDesktopLockRequiresTheVoidReply(t *testing.T) {
	for _, tc := range []struct {
		name string
		out  string
	}{
		{"empty output", ""},
		{"a value reply, not a void one", "(true,)"},
		{"qtchooser's failure text on a zero exit", "qdbus: could not find a Qt installation of ''"},
		{"whitespace only", "   \n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
				return []byte(tc.out), nil
			}}
			b, err := newRegistered(t, "kde", r)
			if err != nil {
				t.Fatal(err)
			}
			err = b.Lock(context.Background(), desktopSession("2"))
			if err == nil {
				t.Fatalf("Lock reported success on reply %q with exit 0 — this is the fail-open the reply check exists for", tc.out)
			}
			if !strings.Contains(err.Error(), "expected the void reply") {
				t.Errorf("error does not name the reply check: %v", err)
			}
			// Every configured name was tried, and none of them turns a bad reply
			// into a success.
			if got := len(r.Calls()); got != 3 {
				t.Errorf("%d command(s) run, want 3 (every configured bus name)", got)
			}
		})
	}
}

func TestDesktopLockTriesBusNamesInOrderAndStopsAtTheFirstYes(t *testing.T) {
	var seen []string
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		dest := argv[4]
		seen = append(seen, dest)
		if dest == "org.kde.screensaver" {
			return nil, errors.New("NameHasNoOwner")
		}
		return []byte("()"), nil
	}}
	b, err := newRegistered(t, "kde", r)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Lock(context.Background(), desktopSession("2")); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if len(seen) != 2 || seen[0] != "org.kde.screensaver" || seen[1] != "org.freedesktop.ScreenSaver" {
		t.Errorf("tried %v, want the first two names in configured order and no third", seen)
	}
}

// Confirmation must not use any screensaver signal. GetActive() was measured returning
// (true,) on this desktop while it was unlocked and in active use, so a backend that
// confirmed with it would report every lock as confirmed forever.
//
// The locker fixture is what lets the hint confirm at all on KDE: the hint alone is the
// controlled user's own property (#29), so a bare yes is no longer enough here.
func TestDesktopConfirmUsesLockedHintAndNeverAsksTheScreensaver(t *testing.T) {
	newProcFixture(t).
		add(7619, 6695, desktopUID, measuredKsmserver).
		add(2224790, 7619, desktopUID, measuredLocker).
		use()
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if argv[0] == "loginctl" {
			return []byte("LockedHint=yes\n"), nil
		}
		return []byte("()"), nil
	}}
	b, err := newRegistered(t, "kde", r)
	if err != nil {
		t.Fatal(err)
	}
	conf, ok := b.(Confirmer)
	if !ok {
		t.Fatal("the desktop backend does not implement Confirmer; it cannot end a chain")
	}

	yes, err := conf.LockConfirmed(context.Background(), desktopSession("2"))
	if err != nil {
		t.Fatalf("LockConfirmed: %v", err)
	}
	if !yes {
		t.Error("LockedHint=yes with the locker running did not confirm")
	}

	for _, c := range r.Calls() {
		joined := strings.Join(c.argv, " ")
		if strings.Contains(joined, "GetActive") {
			t.Errorf("confirmation asked the screensaver GetActive, which is true on an unlocked desktop: %v", c.argv)
		}
		if c.argv[0] == "gdbus" {
			t.Errorf("confirmation ran a gdbus command (%v); confirming must not call anything that could lock", c.argv)
		}
	}
	// The one thing it should have done.
	if len(r.Calls()) != 1 || r.Calls()[0].argv[0] != "loginctl" {
		t.Errorf("confirmation did not read LockedHint through loginctl: %v", r.Argvs())
	}
}

func TestDesktopConfirmRefusesAnUnidentifiedSession(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		return []byte("LockedHint=yes\n"), nil
	}}
	b, err := newRegistered(t, "kde", r)
	if err != nil {
		t.Fatal(err)
	}
	conf := b.(Confirmer)
	if yes, err := conf.LockConfirmed(context.Background(), desktopSession("")); err == nil {
		t.Errorf("confirmed an id-less session as locked=%v without an error", yes)
	}
	if len(r.Calls()) != 0 {
		t.Errorf("queried logind %d time(s) for a session with no id", len(r.Calls()))
	}
}

func TestDesktopProbeSaysItEnumeratesNothing(t *testing.T) {
	b, err := newRegistered(t, "kde", &fakeRunner{})
	if err != nil {
		t.Fatal(err)
	}
	statuses := b.Probe(context.Background())
	if len(statuses) != 1 {
		t.Fatalf("%d status lines, want 1", len(statuses))
	}
	if statuses[0].Name != "kde" {
		t.Errorf("status names backend %q", statuses[0].Name)
	}
	// Whether the tool is present decides availability, and the box running the tests
	// may or may not have gdbus. Either answer must be stated, not left implicit.
	if statuses[0].Available {
		if !strings.Contains(strings.ToLower(statuses[0].Detail), "no sessions of its own") {
			t.Errorf("an available desktop backend must say it finds no sessions of its own, or a chain configured without logind looks complete: %q", statuses[0].Detail)
		}
		if !strings.Contains(statuses[0].Detail, "LockedHint") {
			t.Errorf("an available desktop backend must say where confirmation comes from: %q", statuses[0].Detail)
		}
		// "usable" is the word an operator acts on, and on a box with no desktop it is
		// a lie: gdbus being installed says nothing about a screensaver being reachable,
		// because Probe has no user session to ask. The detail has to carry that limit.
		if !strings.Contains(statuses[0].Detail, "not that a lock would land") {
			t.Errorf("an available desktop backend must not imply a lock would succeed here: %q", statuses[0].Detail)
		}
	} else if !strings.Contains(statuses[0].Detail, gdbusProgram) {
		t.Errorf("an unavailable desktop backend must name the missing tool: %q", statuses[0].Detail)
	}
}

// A chain configured with only a desktop backend finds no sessions. That is the shape a
// misconfigured install has, and it must be visible rather than quiet.
func TestDesktopOnlyChainFindsNoSessions(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) { return []byte("()"), nil }}
	b, err := newRegistered(t, "kde", r)
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := b.Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("the desktop backend reported %d session(s); it has no way to enumerate them", len(sessions))
	}
}

func containsEnv(env []string, want string) bool {
	for _, kv := range env {
		if kv == want {
			return true
		}
	}
	return false
}

// newRegistered builds a backend through the registry rather than by naming its struct,
// so that a test which asserts "kde does X" is asserting it of the backend a config
// selecting "kde" actually gets. A name that is not registered is a fatal error, not a
// nil the test then dereferences.
func newRegistered(t *testing.T, id string, r Runner) (Platform, error) {
	t.Helper()
	ctor, ok := registry[id]
	if !ok {
		t.Fatalf("no backend registered as %q (registered: %s)", id, strings.Join(registeredIDs(), ", "))
	}
	return ctor(r)
}

// registeredIDs lists the backend ids a config could name. Used in failure messages so
// a typo in a test points at what is actually available.
func registeredIDs() []string {
	var out []string
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// The gnome backend is the same mechanism with a different name preference, and it is a
// separate config-selectable id. Asserted through the registry so a config saying
// "gnome" is checked against what a config saying "gnome" gets.
func TestGnomeBackendPrefersTheGnomeServiceName(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) { return []byte("()"), nil }}
	b, err := newRegistered(t, "gnome", r)
	if err != nil {
		t.Fatal(err)
	}
	if b.Name() != "gnome" {
		t.Errorf("Name() = %q, want gnome — the audit log records whichever Name the chain got", b.Name())
	}
	if err := b.Lock(context.Background(), desktopSession("7")); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	wantArgv(t, r.Argvs(), [][]string{lockArgv("org.gnome.ScreenSaver")})

	// The fallback list must still include the neutral name: a GNOME release that
	// moved the interface would otherwise have nowhere to fall back to.
	b2, err := newRegistered(t, "gnome", &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if argv[4] == "org.gnome.ScreenSaver" {
			return nil, errors.New("NameHasNoOwner")
		}
		return []byte("()"), nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := b2.Lock(context.Background(), desktopSession("7")); err != nil {
		t.Fatalf("Lock after the first name failed: %v", err)
	}
	if got := b2.(*desktop).dests; len(got) < 2 || got[1] != "org.freedesktop.ScreenSaver" {
		t.Errorf("gnome fallback list = %v, want the neutral freedesktop name second", got)
	}
}

// Both Probe answers, through the lookup seam. Which one a given host produces is not
// something a test may leave to chance: the unavailable branch is the text an operator
// reads when nothing ever locked.
func TestProbeReportsBothToolPresentAndToolMissing(t *testing.T) {
	saved := lookPath
	defer func() { lookPath = saved }()

	t.Run("gdbus missing", func(t *testing.T) {
		lookPath = func(string) (string, error) { return "", errors.New("executable file not found") }
		b, err := newRegistered(t, "kde", &fakeRunner{})
		if err != nil {
			t.Fatal(err)
		}
		st := b.Probe(context.Background())
		if st[0].Available {
			t.Error("Probe reported the backend available with gdbus missing; it would fail every lock and the chain would keep trying it")
		}
		if !strings.Contains(st[0].Detail, gdbusProgram) {
			t.Errorf("unavailable status must name the missing tool: %q", st[0].Detail)
		}
	})

	t.Run("gdbus present", func(t *testing.T) {
		lookPath = func(string) (string, error) { return "/usr/bin/gdbus", nil }
		b, err := newRegistered(t, "kde", &fakeRunner{})
		if err != nil {
			t.Fatal(err)
		}
		st := b.Probe(context.Background())
		if !st[0].Available {
			t.Errorf("Probe reported the backend unavailable with gdbus present: %q", st[0].Detail)
		}
	})
}

// The negative half. Without it, a backend that confirmed unconditionally would pass
// every other assertion here: "LockedHint=yes confirms" is only half of what a
// confirmer has to get right, and the half that matters is not confirming when the
// screen is up — a false "locked" stops the chain from ever re-locking.
func TestDesktopConfirmReportsUnlockedWhenHintSaysNo(t *testing.T) {
	for _, hint := range []string{"LockedHint=no\n", "LockedHint=false\n", ""} {
		r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
			if argv[0] == "loginctl" {
				return []byte(hint), nil
			}
			return []byte("()"), nil
		}}
		b, err := newRegistered(t, "kde", r)
		if err != nil {
			t.Fatal(err)
		}
		yes, err := b.(Confirmer).LockConfirmed(context.Background(), desktopSession("2"))
		if err != nil {
			t.Fatalf("hint %q: %v", hint, err)
		}
		if yes {
			t.Errorf("hint %q confirmed a lock; an unconditional yes means the chain never re-locks", hint)
		}
	}
}

// A backend that cannot be reached must not be reported as "unlocked" with no error:
// the chain distinguishes "the screen is up" from "we could not ask", and the
// enforcement loop acts differently on each.
func TestDesktopConfirmPropagatesALogindFailure(t *testing.T) {
	boom := errors.New("dbus: connection reset")
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if argv[0] == "loginctl" {
			return nil, boom
		}
		return []byte("()"), nil
	}}
	b, err := newRegistered(t, "kde", r)
	if err != nil {
		t.Fatal(err)
	}
	yes, err := b.(Confirmer).LockConfirmed(context.Background(), desktopSession("2"))
	if err == nil {
		t.Fatal("a failed LockedHint read reported no error")
	}
	if yes {
		t.Error("reported locked on a failed read")
	}
	if !errors.Is(err, boom) {
		t.Errorf("the cause is not wrapped through: %v", err)
	}
}

// The shipped default chain is [logind, kde, gnome], and kde and gnome are the same
// interface under different preferred names. When confirmation never arrives — the
// open question in docs/platforms.md §4 — both run, and they end up at the same
// locker.
//
// kde asks for its preferred name, org.kde.screensaver, which ksmserver owns. gnome
// prefers org.gnome.ScreenSaver, which on a KDE machine nothing owns, so it falls
// through to its second name, org.freedesktop.ScreenSaver — and ksmserver owns that
// one too (measured: it serves the interface under org.kde.screensaver,
// org.freedesktop.ScreenSaver and org.kde.ksmserver, and org.gnome.ScreenSaver is not
// among them). One desktop, two lock requests per poll.
//
// ScreenSaver.Lock is idempotent, so the effect is nil; what is not nil is the audit
// trail, which will show two accepted locks every 30 seconds. Pinning that here keeps
// it a known property instead of a surprise, and makes any future "optimisation" that
// skips a backend change this test on purpose.
func TestChainWithKdeAndGNOMEAsksTheSameLockerTwice(t *testing.T) {
	var mu sync.Mutex
	var accepted []string

	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if len(argv) > 0 && argv[0] == "loginctl" {
			// The confirmer's view: the desktop never reports the hint, which is the
			// whole reason both backends get asked.
			return []byte("LockedHint=no\n"), nil
		}
		switch dest := dbusDest(argv); dest {
		case "org.gnome.ScreenSaver":
			return nil, errors.New("Error: GDBus.Error:org.freedesktop.DBus.Error.ServiceUnknown")
		default:
			mu.Lock()
			accepted = append(accepted, dest)
			mu.Unlock()
			return []byte("()\n"), nil
		}
	}}

	kde, err := newRegistered(t, "kde", r)
	if err != nil {
		t.Fatal(err)
	}
	gnome, err := newRegistered(t, "gnome", r)
	if err != nil {
		t.Fatal(err)
	}
	c := &Chain{backends: []Platform{kde, gnome}}

	err = c.Lock(context.Background(), desktopSession("2"))
	if !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("Lock error = %v, want ErrUnconfirmed: both backends asked, neither could prove the lock took", err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"org.kde.screensaver", "org.freedesktop.ScreenSaver"}
	if len(accepted) != len(want) {
		t.Fatalf("lock requests = %v, want %v", accepted, want)
	}
	for i := range want {
		if accepted[i] != want[i] {
			t.Errorf("request %d went to %s, want %s", i, accepted[i], want[i])
		}
	}
}

// dbusDest pulls the --dest value out of a gdbus argv.
func dbusDest(argv []string) string {
	for i, a := range argv {
		if a == "--dest" && i+1 < len(argv) {
			return argv[i+1]
		}
	}
	return ""
}
