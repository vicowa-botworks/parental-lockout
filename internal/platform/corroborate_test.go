package platform

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// #29, measured: Session.SetLockedHint is authorized by uid alone, so a controlled user
// can hold LockedHint true from a file in ~/.config/autostart. The whole point of the
// confirmation chain was to survive a child dismissing the lock screen, and a hint
// pre-set from the child's own account switched it off — logind's Lock is a no-op on
// Plasma that still exits 0, its confirmation read the hint back, saw true, and the chain
// stopped before the backend that actually locks was ever asked.
//
// These tests are the two halves of the answer: a hint that was already true is not
// evidence about this request, and a backend that can corroborate is still believed.

// The attack, end to end, on the chain shape the default config builds.
func TestAPreSetHintDoesNotStopTheChainBeforeTheBackendThatLocks(t *testing.T) {
	// logind answers "locked" to everything, which is what a spoofed hint looks like,
	// and it cannot corroborate that — it only relays the property.
	spoofed := &confirming{Mock: &Mock{BackendName: "logind"}, yes: true}
	// The desktop backend is the one that really locks, and it reports honestly: the
	// hint is true but no locker is running, so nothing is locked.
	desktop := &corroborating{Mock: &Mock{BackendName: "kde"}, yes: false}

	c := &Chain{backends: []Platform{spoofed, desktop}}
	err := c.Lock(context.Background(), desktopSession("3"))

	// The desktop had to be asked. This single assertion is the difference between the
	// screen locking and the child using the machine.
	if desktop.LockCount() != 1 {
		t.Errorf("the desktop backend was asked %d time(s), want 1: stopping at the hint is the bug", desktop.LockCount())
	}
	if !errors.Is(err, ErrUnconfirmed) {
		t.Errorf("Lock = %v, want ErrUnconfirmed: nothing here can be stood behind", err)
	}
	if err == nil || !strings.Contains(err.Error(), "before we asked") {
		t.Errorf("error %q does not say that the hint was already set when we asked", err)
	}
}

// A chain with no corroborator anywhere must never report success off the hint alone.
// This is the shape of a machine configured with logind only, and it is also the worst
// case of the rule: honest, and permanently unconfirmed.
func TestAHintAloneNeverEndsAChainThatCannotCorroborate(t *testing.T) {
	only := &confirming{Mock: &Mock{BackendName: "logind"}, yes: true}
	c := &Chain{backends: []Platform{only}}

	if err := c.Lock(context.Background(), desktopSession("3")); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("Lock = %v, want ErrUnconfirmed", err)
	}
}

// The other half, and the reason the rule above is not simply "never trust the hint":
// the enforcement loop re-asserts every tick, so from the second tick onward a genuinely
// locked machine always reads "already locked". Discounting a corroborated yes there
// would report unconfirmed forever on a locked screen, and an operator trained to ignore
// that signal has no signal left.
func TestALockedMachineStillConfirmsQuietlyOnEveryReAssert(t *testing.T) {
	hint := &confirming{Mock: &Mock{BackendName: "logind"}, yes: true}
	desktop := &corroborating{Mock: &Mock{BackendName: "kde"}, yes: true}

	c := &Chain{backends: []Platform{hint, desktop}}
	for tick := 1; tick <= 3; tick++ {
		if err := c.Lock(context.Background(), desktopSession("3")); err != nil {
			t.Fatalf("tick %d: Lock = %v, want nil: the locker is running, so this lock is proved", tick, err)
		}
	}
	if desktop.LockCount() != 3 {
		t.Errorf("the desktop was asked %d time(s) over 3 ticks, want 3: re-asserting is the point", desktop.LockCount())
	}
}

// Chain.lockState has to ask the backend that reports the raw hint, not the first one it
// finds. A chain configured [kde logind] would otherwise ask the corroborator, correctly
// hear "not locked" (no locker), and then take logind's bare-hint yes at face value —
// the attack, arriving through the configuration order rather than the code.
func TestTheBeforeStateIsReadFromTheUncorroboratedBackendWhateverTheOrder(t *testing.T) {
	// Honest desktop: hint true, no locker, so it says unlocked and can prove that it
	// cannot prove a lock.
	desktop := &corroborating{Mock: &Mock{BackendName: "kde"}, yes: false}
	spoofed := &confirming{Mock: &Mock{BackendName: "logind"}, yes: true}

	c := &Chain{backends: []Platform{desktop, spoofed}}
	err := c.Lock(context.Background(), desktopSession("3"))

	if !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("Lock = %v, want ErrUnconfirmed", err)
	}
	if !strings.Contains(err.Error(), "before we asked") {
		t.Errorf("logind's yes was credited rather than discounted; the before-state came from the wrong backend: %v", err)
	}
}

// A lock that genuinely changed the state is evidence even with no corroborator, or the
// first lock on a healthy machine could never be reported at all.
func TestAStateThatChangedIsStillEvidence(t *testing.T) {
	// Unlocked before the request, locked after it: the transition is the daemon's own.
	flip := &flipConfirmer{Mock: &Mock{BackendName: "logind"}, trueFrom: 2}
	c := &Chain{backends: []Platform{flip}}

	if err := c.Lock(context.Background(), desktopSession("3")); err != nil {
		t.Fatalf("Lock = %v, want nil: the hint went from no to yes under us", err)
	}
	if flip.reads < 2 {
		t.Errorf("confirmation read %d time(s), want the before-read and at least one after", flip.reads)
	}
}

// If the state beforehand could not be read, a yes cannot be credited to this request.
// The safe reading is the same one the rest of the package takes: keep trying.
func TestChainLockDiscountsAYesWhoseBeforeStateIsUnreadable(t *testing.T) {
	boom := errors.New("bus gone")
	// Errors on the pre-read only; the settle reads then answer "locked", which is the
	// shape that has to be discounted rather than believed.
	f := &flipConfirmer{Mock: &Mock{BackendName: "logind"}, errOn: 1, err: boom, trueFrom: 2}
	c := &Chain{backends: []Platform{f}}

	err := c.Lock(context.Background(), desktopSession("3"))
	if !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("Lock = %v, want ErrUnconfirmed", err)
	}
	if !strings.Contains(err.Error(), "could not be read") {
		t.Errorf("error %q does not say the before-state was unreadable", err)
	}
	if !errors.Is(err, boom) {
		t.Errorf("error %q loses the reason the state could not be read", err)
	}
}

// A chain with nothing that can confirm at all has to say so, rather than reporting the
// before-state as "unlocked" and then crediting a bare yes.
func TestChainWithNoConfirmerSaysItCannotReadTheState(t *testing.T) {
	m := &Mock{BackendName: "legacy"}
	c := &Chain{backends: []Platform{m}}

	_, err := c.lockState(context.Background(), desktopSession("3"))
	if err == nil || !strings.Contains(err.Error(), "no backend") {
		t.Errorf("lockState = %v, want a sentence naming that nothing here can confirm", err)
	}
}

// The desktop backend is where #29 asked for the second signal, and it is flavour
// specific: KDE spawns a locker, GNOME draws its lock screen inside gnome-shell.
func TestDesktopConfirmationNeedsTheLockerOnKdeAndNotOnGnome(t *testing.T) {
	hintYes := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if argv[0] == "loginctl" {
			return []byte("LockedHint=yes\n"), nil
		}
		return []byte("()"), nil
	}}

	t.Run("kde with no locker", func(t *testing.T) {
		newProcFixture(t).add(7619, 6695, desktopUID, measuredKsmserver).use()
		b, err := newRegistered(t, "kde", hintYes)
		if err != nil {
			t.Fatal(err)
		}
		yes, err := b.(Confirmer).LockConfirmed(context.Background(), desktopSession("3"))
		if err != nil {
			t.Fatalf("LockConfirmed: %v", err)
		}
		if yes {
			t.Error("the hint alone corroborated a KDE lock")
		}
		if _, ok := b.(Corroborator); !ok {
			t.Error("the kde backend does not implement Corroborator, so Chain.Lock cannot tell its yes from a bare hint")
		}
	})

	t.Run("kde with the locker", func(t *testing.T) {
		newProcFixture(t).
			add(7619, 6695, desktopUID, measuredKsmserver).
			add(2224790, 7619, desktopUID, measuredLocker).use()
		b, err := newRegistered(t, "kde", hintYes)
		if err != nil {
			t.Fatal(err)
		}
		yes, err := b.(Confirmer).LockConfirmed(context.Background(), desktopSession("3"))
		if err != nil || !yes {
			t.Fatalf("LockConfirmed = %v, %v; want the hint and the locker to confirm together", yes, err)
		}
	})

	// GNOME has no corroborating signal yet, so it says so rather than pretending. The
	// point of Corroborated() being false is that Chain.Lock then discounts its yes
	// whenever the hint was already set — the honest consequence of #29, and the reason
	// the flavour is explicit instead of accidental.
	t.Run("gnome", func(t *testing.T) {
		newProcFixture(t).use()
		b, err := newRegistered(t, "gnome", hintYes)
		if err != nil {
			t.Fatal(err)
		}
		yes, err := b.(Confirmer).LockConfirmed(context.Background(), desktopSession("3"))
		if err != nil || !yes {
			t.Fatalf("LockConfirmed = %v, %v; gnome confirms on the hint, as measured", yes, err)
		}
		if cor, ok := b.(Corroborator); ok && cor.Corroborated() {
			t.Error("gnome claims to corroborate; nothing about GNOME's lock has been measured here")
		}
	})
}

// A corroborator whose second signal cannot be consulted is not a corroborator that said
// "unlocked". The hint said locked and the check failed; that is a fault, and it has to
// arrive as one.
func TestDesktopConfirmationReportsAFailedCorroborationAsAFault(t *testing.T) {
	old := procRoot
	procRoot = "/proc/this/path/does/not/exist"
	t.Cleanup(func() { procRoot = old })

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

	yes, err := b.(Confirmer).LockConfirmed(context.Background(), desktopSession("3"))
	if err == nil {
		t.Fatal("an unconsultable corroborator reported a verdict instead of a fault")
	}
	if yes {
		t.Error("a failed corroboration confirmed the lock")
	}
	if !strings.Contains(err.Error(), "could not be checked") {
		t.Errorf("error %q does not say the locker could not be checked", err)
	}
}

// Probe is what an operator reads when a lock never happens, so it has to say whether a
// confirmation from this backend is worth anything.
func TestProbeSaysWhetherConfirmationIsCorroborated(t *testing.T) {
	for _, tc := range []struct{ id, want string }{
		{"kde", "corroborated by a live kscreenlocker_greet"},
		{"gnome", "uncorroborated"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			b, err := newRegistered(t, tc.id, &fakeRunner{})
			if err != nil {
				t.Fatal(err)
			}
			var detail string
			for _, st := range b.Probe(context.Background()) {
				detail += st.Detail
			}
			if !strings.Contains(detail, tc.want) {
				t.Errorf("the %s backend's Probe does not say %q:\n%s", tc.id, tc.want, detail)
			}
		})
	}
}
