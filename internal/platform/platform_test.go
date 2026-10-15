package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

func configSettings(ids ...string) config.Settings {
	return config.Settings{LockBackends: ids}
}

func TestNewBuildsTheChainInConfiguredOrder(t *testing.T) {
	r := &fakeRunner{}
	c, err := New(configSettings("logind"), r)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := c.Names(); len(got) != 1 || got[0] != "logind" {
		t.Errorf("chain = %v, want [logind]", got)
	}
	if len(c.Unknown()) != 0 {
		t.Errorf("Unknown = %v, want none", c.Unknown())
	}
	// kde and gnome landed in #6, so the default config's three names all resolve.
	// The unknown-name path is still what this block is for, so it is exercised with
	// names nothing registers rather than with names that used to be missing.
	c2, err := New(configSettings("logind", "plasma", "kde "), r)
	if err == nil {
		t.Fatal("New reported no error for backends this binary does not have")
	}
	if got := c2.Names(); len(got) != 1 || got[0] != "logind" {
		t.Errorf("chain = %v, want the one backend that exists", got)
	}
	// Config order, not sorted: the list is what somebody wrote, and echoing it back
	// in the order they wrote it is what makes the message readable.
	if got := c2.Unknown(); len(got) != 2 || got[0] != "plasma" || got[1] != "kde " {
		t.Errorf("Unknown = %v, want [plasma \"kde \"] in config order", got)
	}
	// The error has to name both the offending id and what is available, or the
	// operator has no way to find the typo.
	msg := err.Error()
	for _, want := range []string{"kde", "gnome", "logind"} {
		if !strings.Contains(msg, want) {
			t.Errorf("New error %q does not mention %q", msg, want)
		}
	}
	// And the chain is still usable: a shorter chain is not a broken one.
	if st := c2.Probe(context.Background()); len(st) != 3 {
		t.Errorf("Probe on a partial chain gave %+v, want the backend plus the two unknown names", st)
	}
}

// A config that names nothing usable must say so at lock time in terms an operator
// can act on, rather than returning nil from a chain with no members.
func TestChainWithNoBackendsRefusesLoudly(t *testing.T) {
	c, err := New(configSettings("nonexistent"), &fakeRunner{})
	if err == nil {
		t.Fatal("New accepted a backend that does not exist")
	}
	err = c.Lock(context.Background(), desktopSession("3"))
	if err == nil {
		t.Fatal("Lock succeeded with no backend in the chain")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error %q does not name the configured id", err)
	}
}

func TestChainSessionsMergesByID(t *testing.T) {
	// Two backends that both enumerate. Today only logind does, but the merge rule is
	// what stops the enforcement loop locking one session twice and writing two audit
	// events for one action.
	a := &Mock{BackendName: "a", Listed: []Session{
		{ID: "3", Username: "kid", Type: "x11", State: "active", Locked: true},
	}}
	b := &Mock{BackendName: "b", Listed: []Session{
		{ID: "3", Username: "kid", Type: "x11", State: "active", Locked: false},
		{ID: "4", Username: "other", Type: "wayland", State: "active"},
	}}
	c := &Chain{backends: []Platform{a, b}}

	got, err := c.Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d sessions %v, want 2 — session 3 is reported by both", len(got), got)
	}
	// First backend wins, so the config order decides whose Locked is used.
	if got[0].ID != "3" || !got[0].Locked {
		t.Errorf("session 3 came from %q, want the first backend's view (Locked=true)", got[0].Username)
	}
}

// A backend that cannot list must not hide the sessions another backend found, but
// the failure has to be visible: "the KDE backend is broken" is not something to
// swallow on the way to enforcing.
func TestChainSessionsReportsAPartialFailure(t *testing.T) {
	good := &Mock{BackendName: "good", Listed: []Session{{ID: "3", Type: "x11", State: "active"}}}
	bad := &Mock{BackendName: "bad", ListErr: errors.New("boom")}
	c := &Chain{backends: []Platform{bad, good}}

	got, err := c.Sessions(context.Background())
	if len(got) != 1 {
		t.Fatalf("got %v, want the session the working backend returned", got)
	}
	if err == nil {
		t.Fatal("the failing backend was swallowed")
	}
	if !strings.Contains(err.Error(), "bad") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("error %q does not name the backend and its cause", err)
	}
}

func TestChainLockTriesInOrderAndStopsAtFirstConfirmation(t *testing.T) {
	s := desktopSession("3")
	// The first backend locks AND can prove it, which is the only thing that ends the
	// chain.
	// "Prove" is load-bearing now. A backend that only relays LockedHint says nothing
	// this call can be credited with, because the hint is the controlled user's own
	// property (#29) — so the backend that ends the chain is one that corroborates.
	first := &corroborating{Mock: &Mock{BackendName: "first"}, yes: true}
	second := &confirming{Mock: &Mock{BackendName: "second"}, yes: true}
	c := &Chain{backends: []Platform{first, second}}

	if err := c.Lock(context.Background(), s); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if first.LockCount() != 1 || second.LockCount() != 0 {
		t.Errorf("calls: first=%d second=%d, want the first confirmation to stop the chain", first.LockCount(), second.LockCount())
	}
}

func TestChainLockFallsThroughOnFailure(t *testing.T) {
	s := desktopSession("3")
	first := &confirming{Mock: &Mock{BackendName: "first", LockErr: errors.New("plasma ignored us")}}
	second := &confirming{Mock: &Mock{BackendName: "second"}, yes: true}
	c := &Chain{backends: []Platform{first, second}}

	if err := c.Lock(context.Background(), s); err != nil {
		t.Fatalf("Lock: %v — the second backend should have succeeded", err)
	}
	if first.LockCount() != 1 || second.LockCount() != 1 {
		t.Errorf("calls: first=%d second=%d, want both", first.LockCount(), second.LockCount())
	}
}

// When every backend fails, the error has to carry every reason. This is the exact
// situation the KDE backend exists for, and "nothing worked" without the reasons is
// undiagnosable from a log line.
func TestChainLockReportsEveryFailure(t *testing.T) {
	s := desktopSession("3")
	a := &Mock{BackendName: "logind", LockErr: errors.New("permission denied")}
	b := &Mock{BackendName: "kde", LockErr: errors.New("no session bus")}
	c := &Chain{backends: []Platform{a, b}}

	err := c.Lock(context.Background(), s)
	if err == nil {
		t.Fatal("Lock succeeded with every backend failing")
	}
	for _, want := range []string{"logind", "permission denied", "kde", "no session bus"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q", err, want)
		}
	}
}

// The chain refuses a non-lockable session before asking any backend, so the rule
// lives on the type every caller must go through rather than on each backend's
// memory.
func TestChainNeverLocksWhatItMustNot(t *testing.T) {
	m := &corroborating{Mock: &Mock{}, yes: true}
	c := &Chain{backends: []Platform{m}}
	for name, s := range map[string]Session{
		"remote ssh":  {ID: "1", Class: "user", Seat: "seat0", Type: "x11", State: "active", Remote: true},
		"closing":     {ID: "2", Class: "user", Seat: "seat0", Type: "x11", State: "closing"},
		"manager":     {ID: "3", Class: "manager", Type: "unspecified", State: "active"},
		"greeter":     {ID: "3b", Class: "greeter", Seat: "seat0", Type: "x11", State: "active"},
		"no seat":     {ID: "3c", Class: "user", Type: "x11", State: "active"},
		"no class":    {ID: "3d", Seat: "seat0", Type: "x11", State: "active"},
		"no type":     {ID: "4", Class: "user", Seat: "seat0", State: "active"},
		"tty session": {ID: "5", Class: "user", Seat: "seat0", Type: "tty", State: "active"}, // lockable: a VT is still a session
	} {
		err := c.Lock(context.Background(), s)
		if name == "tty session" {
			if err != nil {
				t.Errorf("Lock(%s): %v — a tty session is lockable", name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("Lock(%s) succeeded, want refusal", name)
		}
	}
	if n := m.LockCount(); n != 1 {
		t.Errorf("%d lock call(s) reached the backend, want only the tty session", n)
	}
}

func TestChainProbeIncludesUnknownNames(t *testing.T) {
	m := &Mock{BackendName: "logind"}
	c := &Chain{backends: []Platform{m}, unknown: []string{"kde"}}
	st := c.Probe(context.Background())
	if len(st) != 2 {
		t.Fatalf("Probe gave %+v, want the backend plus the unknown name", st)
	}
	if st[1].Name != "kde" || st[1].Available {
		t.Errorf("unknown backend reported as %+v, want unavailable", st[1])
	}
}

func TestIsLockable(t *testing.T) {
	base := Session{ID: "1", Class: "user", Seat: "seat0", Type: "x11", State: "active"}
	for _, tc := range []struct {
		name string
		s    Session
		want bool
	}{
		{"plain", base, true},
		{"tty", Session{Class: "user", Seat: "seat0", Type: "tty", State: "active"}, true},
		{"wayland", Session{Class: "user", Seat: "seat0", Type: "wayland", State: "active"}, true},
		{"remote", Session{Class: "user", Seat: "seat0", Type: "x11", State: "active", Remote: true}, false},
		{"closing", Session{Class: "user", Seat: "seat0", Type: "x11", State: "closing"}, false},
		{"empty type", Session{Class: "user", Seat: "seat0", State: "active"}, false},
		{"unspecified type", Session{Class: "user", Seat: "seat0", Type: "unspecified", State: "active"}, false},
		// A greeter session is the login screen. Locking it leaves an uninputtable wall
		// where the child's account was never chosen, and it can share a uid with a real
		// session, so a uid-aimed caller can easily hit it.
		{"greeter", Session{Class: "greeter", Seat: "seat0", Type: "x11", State: "active"}, false},
		{"lock screen", Session{Class: "lock", Seat: "seat0", Type: "x11", State: "active"}, false},
		{"manager", Session{Class: "manager", Type: "unspecified", State: "active"}, false},
		// Observed on the reviewer's box (systemd 259.5-0ubuntu3.4) and named by no
		// ticket: class=manager-early. The class check is an allowlist, so an
		// unrecognised class is refused rather than waved through — which is the only
		// reason a value nobody predicted here is handled correctly. Pinned so the
		// day someone widens this to a denylist, this case fails loudly.
		{"manager-early", Session{Class: "manager-early", Seat: "seat0", Type: "x11", State: "active"}, false},
		// No seat means not on any console.
		{"no seat", Session{Class: "user", Type: "x11", State: "active"}, false},
		// The seat requirement is doing more work than the fixtures show: on the
		// reviewer's box the SSH sessions are class=user with no seat, and loginctl
		// reports no Remote=yes anywhere. So the seat check, not the Remote check, is
		// what keeps a remote admin from being locked on that machine. Both are kept
		// because which one fires depends on the build.
		{"ssh session, class user, no seat", Session{Class: "user", Type: "x11", State: "active", Remote: false}, false},
		// A Session that never came from a listing has no class. Refusing it is the
		// point: the rule is "came from a desktop session listing", and that is only
		// checkable if the field has to be there.
		{"no class", Session{Seat: "seat0", Type: "x11", State: "active"}, false},
		// An online-but-not-active session is still a session someone could walk up
		// to, so it stays lockable.
		{"online", Session{Class: "user", Seat: "seat0", Type: "x11", State: "online"}, true},
	} {
		if got := isLockable(tc.s); got != tc.want {
			t.Errorf("%s: isLockable = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestRegisteredIsSortedAndComplete(t *testing.T) {
	got := Registered()
	if len(got) == 0 {
		t.Fatal("no backends registered; logind's init did not run")
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Fatalf("Registered is not sorted: %v", got)
		}
	}
	// The mock must stay out of the registry: a config that could select it would
	// report success while locking nothing.
	for _, id := range got {
		if id == "mock" {
			t.Error("the mock backend is registered — a config could then silently enforce nothing")
		}
	}
}

// Only the empty id is exercised. The duplicate case panics for the same reason and
// is equally a programming error, but testing it would leave a test-only name in the
// package-level registry for every other test in this package to trip over, and
// there is no unregister to put it back.
func TestRegisterRejectsAnEmptyID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Register with an empty id did not panic")
		}
	}()
	Register("", func(r Runner, _ config.Settings) (Platform, error) { return nil, nil })
}

func TestMockSessionsReturnsACopy(t *testing.T) {
	m := &Mock{Listed: []Session{{ID: "3", Type: "x11", State: "active"}}}
	got, err := m.Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	got[0].ID = "tampered"
	again, _ := m.Sessions(context.Background())
	if again[0].ID != "3" {
		t.Errorf("the script was rewritten by the caller: %v", again[0].ID)
	}
}

// The mock is what every enforcement test in #7 will assert through, so its own
// bookkeeping has to be pinned here rather than discovered from the other side of a
// failing loop test.
func TestMockBookkeeping(t *testing.T) {
	ctx := context.Background()
	s := Session{ID: "3", Username: "kid", Type: "x11", State: "active"}
	m := &Mock{Listed: []Session{s}}

	if err := m.Lock(ctx, s); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := m.Lock(ctx, Session{ID: "4", Type: "x11", State: "active"}); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if got := m.LockedIDs(); len(got) != 2 || got[0] != "3" || got[1] != "4" {
		t.Errorf("LockedIDs = %v, want [3 4] in call order", got)
	}
	if m.LockCount() != 2 {
		t.Errorf("LockCount = %d, want 2", m.LockCount())
	}
	if len(m.Locked()) != 2 || m.Locked()[0].Username != "kid" {
		t.Errorf("Locked = %v", m.Locked())
	}

	// A failed lock is still an attempt, and a fallback chain has to be able to count
	// the attempts rather than only the successes.
	failing := &Mock{LockErr: errors.New("nope")}
	if err := failing.Lock(ctx, s); err == nil {
		t.Error("a failing mock reported success")
	}
	if failing.LockCount() != 1 {
		t.Errorf("a failed lock was not counted: %d", failing.LockCount())
	}

	// Per-session failure, which is how a chain test says "backend A cannot do THIS
	// session".
	per := &Mock{LockErrBySession: map[string]error{"3": errors.New("only 3 fails")}}
	if err := per.Lock(ctx, s); err == nil {
		t.Error("the scripted per-session failure did not fire")
	}
	if err := per.Lock(ctx, Session{ID: "9", Type: "x11", State: "active"}); err != nil {
		t.Errorf("an unscripted session failed: %v", err)
	}

	// Reset clears the record and keeps the script.
	m.Reset()
	if m.LockCount() != 0 || len(m.Locked()) != 0 {
		t.Error("Reset left the record behind")
	}
	if got, err := m.Sessions(ctx); err != nil || len(got) != 1 {
		t.Errorf("Reset cleared the script: %v %v", got, err)
	}

	// Scripted probe, and Probe hands back a copy so a caller cannot rewrite the
	// script through the slice it was given.
	m.SetProbe(BackendStatus{Name: "mock", Available: false, Detail: "down"})
	st := m.Probe(ctx)
	if len(st) != 1 || st[0].Available || st[0].Detail != "down" {
		t.Errorf("Probe = %+v, want the scripted unavailable status", st)
	}
	st[0].Detail = "tampered"
	if again := m.Probe(ctx); again[0].Detail != "down" {
		t.Errorf("the probe script was rewritten by the caller: %q", again[0].Detail)
	}
	if def := (&Mock{}).Probe(ctx); len(def) != 1 || !def[0].Available {
		t.Errorf("default Probe = %+v, want one available status", def)
	}
	if n := (&Mock{}).Name(); n != "mock" {
		t.Errorf("default Name = %q, want mock", n)
	}
	if n := (&Mock{BackendName: "kde"}).Name(); n != "kde" {
		t.Errorf("Name = %q, want kde", n)
	}
}

// The failure this whole distinction exists for: logind accepts the request, Plasma
// ignores it, and a chain that treats "no error" as "locked" stops right there. On the
// target platform that means the KDE backend is never reached and the screen never
// locks, while the daemon records a successful lock every 30 seconds.
func TestChainDoesNotStopAtASuccessItCannotConfirm(t *testing.T) {
	s := desktopSession("3")
	logind := &confirming{Mock: &Mock{BackendName: "logind"}, yes: false}
	kde := &confirming{Mock: &Mock{BackendName: "kde"}, yes: true}
	c := &Chain{backends: []Platform{logind, kde}}

	if err := c.Lock(context.Background(), s); err != nil {
		t.Fatalf("Lock: %v, want the chain to fall through to the backend that confirms", err)
	}
	if logind.LockCount() != 1 || kde.LockCount() != 1 {
		t.Errorf("calls: logind=%d kde=%d, want both", logind.LockCount(), kde.LockCount())
	}
}

// A backend that cannot confirm at all must not end the chain either. If it did, adding
// a confirming backend behind a non-confirming one would change nothing, and the
// difference between the two would be invisible to whoever configures the chain.
func TestChainDoesNotStopAtABackendThatCannotConfirm(t *testing.T) {
	s := desktopSession("3")
	// Mock deliberately does not implement Confirmer.
	first := &Mock{BackendName: "legacy"}
	second := &corroborating{Mock: &Mock{BackendName: "kde"}, yes: true}
	c := &Chain{backends: []Platform{first, second}}

	if err := c.Lock(context.Background(), s); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if second.LockCount() != 1 {
		t.Errorf("the confirming backend was never tried: it is the only one that can prove anything")
	}
}

// When every backend asks and none can prove it, the caller has to be able to tell that
// apart from a lock. The loop keeps re-arming on this, so it must not read as success.
func TestChainReportsUnconfirmedWhenNothingConfirms(t *testing.T) {
	s := desktopSession("3")
	a := &confirming{Mock: &Mock{BackendName: "logind"}, yes: false}
	b := &Mock{BackendName: "legacy"}
	c := &Chain{backends: []Platform{a, b}}

	err := c.Lock(context.Background(), s)
	if err == nil {
		t.Fatal("Lock returned nil with no backend able to confirm the lock")
	}
	if !errors.Is(err, ErrUnconfirmed) {
		t.Errorf("err = %v, want it to match ErrUnconfirmed", err)
	}
	// Both reasons have to survive, or the operator cannot tell which backend to fix.
	for _, want := range []string{"logind", "legacy", "still unlocked", "cannot confirm"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q", err, want)
		}
	}
}

// A chain where every backend hard-failed is not "unconfirmed": nothing ever reached the
// desktop, which is a different problem and needs a different report.
func TestChainHardFailureIsNotReportedAsUnconfirmed(t *testing.T) {
	s := desktopSession("3")
	a := &Mock{BackendName: "logind", LockErr: errors.New("permission denied")}
	b := &Mock{BackendName: "kde", LockErr: errors.New("no session bus")}
	c := &Chain{backends: []Platform{a, b}}

	err := c.Lock(context.Background(), s)
	if err == nil {
		t.Fatal("Lock returned nil with every backend failing")
	}
	if errors.Is(err, ErrUnconfirmed) {
		t.Errorf("a total hard failure was reported as ErrUnconfirmed: %v", err)
	}
	for _, want := range []string{"logind", "permission denied", "kde", "no session bus"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q", err, want)
		}
	}
}

// A mixed chain — one hard failure, one unconfirmed ask — is still the unconfirmed
// class, because something did reach the desktop, and the hard failure must not be lost.
func TestChainMixedFailureReportsUnconfirmedAndKeepsBothReasons(t *testing.T) {
	s := desktopSession("3")
	a := &Mock{BackendName: "logind", LockErr: fmt.Errorf("after the deadline: %w", context.DeadlineExceeded)}
	b := &confirming{Mock: &Mock{BackendName: "kde"}, yes: false}
	c := &Chain{backends: []Platform{a, b}}

	err := c.Lock(context.Background(), s)
	if !errors.Is(err, ErrUnconfirmed) {
		t.Errorf("err = %v, want it to match ErrUnconfirmed", err)
	}
	for _, want := range []string{"after the deadline", "kde"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q", err, want)
		}
	}
	// The per-backend reasons are joined, and a caller has to be able to reach through
	// the join to a cause it cares about — the loop distinguishes "timed out" from
	// "refused" without matching on message text.
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false; the joined reasons are opaque: %v", err)
	}
}

// A confirmation that errors is not a confirmation, and must not be reported as one.
func TestChainKeepsGoingWhenConfirmationErrors(t *testing.T) {
	s := desktopSession("3")
	a := &confirming{Mock: &Mock{BackendName: "logind"}, err: errors.New("Connection reset by peer")}
	b := &corroborating{Mock: &Mock{BackendName: "kde"}, yes: true}
	c := &Chain{backends: []Platform{a, b}}

	if err := c.Lock(context.Background(), s); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if b.LockCount() != 1 {
		t.Error("a failed confirmation stopped the chain")
	}
}

// The enforcement loop holds a Chain, and its job is to notice a screen that got
// unlocked. If the chain could not answer "is it locked", the loop's only move would be
// to lock again — so the composite has to answer the question its parts answer.
func TestChainAnswersWhetherALockIsInEffect(t *testing.T) {
	s := desktopSession("3")

	yes, err := (&Chain{backends: []Platform{
		&confirming{Mock: &Mock{BackendName: "logind"}, yes: false},
		&confirming{Mock: &Mock{BackendName: "kde"}, yes: true},
	}}).LockConfirmed(context.Background(), s)
	if !yes || err != nil {
		t.Errorf("LockConfirmed = %v, %v; want true when one backend sees the lock", yes, err)
	}

	// Nobody sees a lock: reported as unlocked, not as an error and not as true.
	yes, err = (&Chain{backends: []Platform{
		&confirming{Mock: &Mock{BackendName: "logind"}, yes: false},
		&confirming{Mock: &Mock{BackendName: "kde"}, yes: false},
	}}).LockConfirmed(context.Background(), s)
	if yes {
		t.Error("LockConfirmed reported a lock that no backend could see")
	}
	if err != nil {
		t.Errorf("an ordinary unlocked answer came back as an error: %v", err)
	}

	// A backend that cannot see the session must not veto one that can.
	yes, err = (&Chain{backends: []Platform{
		&confirming{Mock: &Mock{BackendName: "blind"}, err: errors.New("Failed to connect to bus")},
		&confirming{Mock: &Mock{BackendName: "kde"}, yes: true},
	}}).LockConfirmed(context.Background(), s)
	if !yes {
		t.Errorf("a failing backend vetoed a backend that saw the lock (err=%v)", err)
	}

	// Nothing could establish the state. That is unlocked, with the reasons kept.
	yes, err = (&Chain{backends: []Platform{
		&confirming{Mock: &Mock{BackendName: "blind"}, err: errors.New("Failed to connect to bus")},
	}}).LockConfirmed(context.Background(), s)
	if yes {
		t.Error("LockConfirmed reported a lock on nothing but failures")
	}
	if err == nil || !strings.Contains(err.Error(), "Failed to connect to bus") {
		t.Errorf("the reasons the check failed were lost: %v", err)
	}

	// A backend with no confirmation ability is skipped, not counted as "unlocked".
	yes, err = (&Chain{backends: []Platform{&Mock{BackendName: "legacy"}}}).
		LockConfirmed(context.Background(), s)
	if yes || err != nil {
		t.Errorf("LockConfirmed = %v, %v with a non-confirming backend", yes, err)
	}
}
