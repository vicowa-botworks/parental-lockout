package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return b
}

// serve wires a fake runner to the captured fixtures: the listing answers with the
// real --json=short output, and each session id answers with its real show-session
// block.
func serve(t *testing.T, listing string, details map[string]string) *fakeRunner {
	t.Helper()
	return &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if len(argv) >= 2 && argv[1] == "list-sessions" {
			return fixture(t, listing), nil
		}
		if len(argv) >= 3 && argv[1] == "show-session" {
			name, ok := details[argv[2]]
			if !ok {
				// The real binary's behaviour for a session that is gone, so a test
				// that asks for an id it was never listed gets what production gets.
				return nil, errors.New("loginctl show-session " + argv[2] + " exited with 1: Failed to get path for session '" + argv[2] + "': No session '" + argv[2] + "' known")
			}
			return fixture(t, name), nil
		}
		t.Errorf("unexpected command from the backend: %v", argv)
		return nil, errors.New("unexpected command")
	}}
}

func newLogind(t *testing.T, r Runner) *Chain {
	t.Helper()
	p, err := New(configSettings("logind"), r)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

// The captured listing is the whole point of this test: it is what a live systemd
// 259 printed, including the null seat on the manager session that a plain string
// field would have failed to decode.
func TestLogindSessionsFromCapturedListing(t *testing.T) {
	r := serve(t, "list-sessions.json", map[string]string{
		"3": "show-session-desktop.txt",
		"4": "show-session-manager.txt",
	})
	p := newLogind(t, r)

	got, err := p.Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d session(s) %v, want only the desktop session — the manager session shares the uid and has nothing to lock",
			len(got), got)
	}
	s := got[0]
	if s.ID != "3" || s.Username != "rodney" || s.UID != 1000 || s.Type != "x11" || s.State != "active" {
		t.Errorf("session = %+v, want id 3, rodney, uid 1000, x11, active", s)
	}
	if s.Remote {
		t.Errorf("Remote = true, want false (Remote=no in the fixture)")
	}
	if !s.Locked {
		t.Errorf("Locked = false, want true (LockedHint=yes in the fixture)")
	}

	// The listing call is the one the ticket got wrong, so pin it exactly: --json=short
	// is what works, and --output=json is accepted, ignored, and returns a table.
	wantArgv(t, r.Argvs(), [][]string{
		{"loginctl", "list-sessions", "--json=short"},
		{"loginctl", "show-session", "3", "-p", "Name", "-p", "Type", "-p", "State", "-p", "Remote", "-p", "UID", "-p", "LockedHint"},
	})
}

// The manager session must be dropped before the per-session call, not after: it is
// visible in the listing as class=manager with a null seat, and asking logind about it
// is a wasted call every 30 seconds forever.
func TestLogindDoesNotQueryTheManagerSession(t *testing.T) {
	r := serve(t, "list-sessions.json", map[string]string{
		"3": "show-session-desktop.txt",
		"4": "show-session-manager.txt",
	})
	if _, err := newLogind(t, r).Sessions(context.Background()); err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	for _, c := range r.Calls() {
		if len(c.argv) >= 3 && c.argv[1] == "show-session" && c.argv[2] == "4" {
			t.Errorf("asked logind about the manager session: %v", c.argv)
		}
	}
}

// The table fallback is tested against the table a real loginctl printed when asked
// for JSON, because that is the output a lenient parser has to cope with.
func TestLogindFallsBackToTheTable(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if len(argv) >= 3 && argv[2] == "--json=short" {
			// An older build that rejects the flag outright.
			return nil, errors.New("loginctl list-sessions --json=short exited with 1: Unknown option --json=short")
		}
		if len(argv) >= 2 && argv[1] == "list-sessions" {
			return fixture(t, "list-sessions.table"), nil
		}
		if len(argv) >= 3 && argv[1] == "show-session" {
			return fixture(t, "show-session-desktop.txt"), nil
		}
		t.Errorf("unexpected command: %v", argv)
		return nil, errors.New("unexpected")
	}}

	got, err := newLogind(t, r).Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(got) != 1 || got[0].ID != "3" || got[0].Type != "x11" {
		t.Fatalf("table fallback gave %v, want the one desktop session", got)
	}
	// The manager row's seat is "-" in the table and null in JSON; both must arrive as
	// absent, or the filter behaves differently depending on which parser ran.
	if got[0].Username != "rodney" || got[0].UID != 1000 {
		t.Errorf("table row parsed as %+v, want rodney uid 1000", got[0])
	}
	wantArgv(t, r.Argvs(), [][]string{
		{"loginctl", "list-sessions", "--json=short"},
		{"loginctl", "list-sessions"},
		{"loginctl", "show-session", "3", "-p", "Name", "-p", "Type", "-p", "State", "-p", "Remote", "-p", "UID", "-p", "LockedHint"},
	})
}

// A loginctl that accepts --json=short and prints a table anyway is the failure mode
// the real 259 demonstrates with --output=json. It must be read, not re-run.
func TestLogindReadsATableEvenWhenTheFlagSucceeds(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if len(argv) >= 2 && argv[1] == "list-sessions" {
			return fixture(t, "list-sessions.table"), nil
		}
		if len(argv) >= 3 && argv[1] == "show-session" {
			return fixture(t, "show-session-desktop.txt"), nil
		}
		t.Errorf("unexpected command: %v", argv)
		return nil, errors.New("unexpected")
	}}
	got, err := newLogind(t, r).Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %v, want one session", got)
	}
	// Exactly one listing call: the table was recognised in place rather than
	// triggering a second command.
	if n := len(r.Calls()); n != 2 {
		t.Errorf("%d command(s), want 2 (one listing, one detail): %v", n, r.Argvs())
	}
}

// LockedHint absent is the documented "best effort" case, and it must not be an
// error: an unknown property is not an error to logind, which prints nothing at all.
func TestLogindMissingLockedHintIsNotAnError(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		switch {
		case len(argv) >= 2 && argv[1] == "list-sessions":
			return []byte(`[{"session":"7","uid":1001,"user":"kid","seat":"seat0","leader":9,"class":"user","tty":"tty3","idle":false,"since":null}]`), nil
		case len(argv) >= 2 && argv[1] == "show-session":
			// No LockedHint line at all, and the remaining properties in an order
			// that is neither requested nor alphabetical — both are real.
			return []byte("Name=kid\nRemote=no\nState=active\nType=wayland\n"), nil
		}
		t.Errorf("unexpected command: %v", argv)
		return nil, errors.New("unexpected")
	}}

	got, err := newLogind(t, r).Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions returned an error for an absent LockedHint: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %v, want one session", got)
	}
	if got[0].Locked {
		t.Errorf("Locked = true with no LockedHint property")
	}
	if got[0].Type != "wayland" || got[0].State != "active" || got[0].Username != "kid" {
		t.Errorf("properties read positionally: %+v", got[0])
	}
}

// Synthetic rather than captured: this machine has no remote session and nothing in
// closing state, so the filter cannot be pinned from a fixture.
func TestLogindExcludesRemoteAndClosing(t *testing.T) {
	listing := `[` +
		`{"session":"1","uid":1000,"user":"parent","seat":"seat0","leader":1,"class":"user","tty":null,"idle":false,"since":null},` +
		`{"session":"2","uid":1000,"user":"parent","seat":null,"leader":2,"class":"user","tty":null,"idle":false,"since":null},` +
		`{"session":"3","uid":1001,"user":"kid","seat":"seat0","leader":3,"class":"user","tty":"tty2","idle":false,"since":null},` +
		`{"session":"4","uid":1001,"user":"kid","seat":"seat0","leader":4,"class":"user","tty":"tty2","idle":false,"since":null}` +
		`]`
	type sess struct{ typ, state, remote string }
	details := map[string]sess{
		"1": {"x11", "active", "no"},      // seat0, local -> kept
		"2": {"x11", "active", "yes"},     // remote ssh -> dropped
		"3": {"wayland", "active", "no"},  // kept
		"4": {"wayland", "closing", "no"}, // logging out -> dropped
	}

	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if len(argv) >= 2 && argv[1] == "list-sessions" {
			return []byte(listing), nil
		}
		if len(argv) >= 3 && argv[1] == "show-session" {
			id := argv[2]
			d, ok := details[id]
			if !ok {
				t.Errorf("asked about an id that was not listed: %s", id)
				return nil, errors.New("no session known")
			}
			return []byte("Name=x\nRemote=" + d.remote + "\nType=" + d.typ + "\nState=" + d.state + "\nLockedHint=no\n"), nil
		}
		t.Errorf("unexpected command: %v", argv)
		return nil, errors.New("unexpected")
	}}

	got, err := newLogind(t, r).Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	var ids []string
	for _, s := range got {
		if s.Remote {
			t.Errorf("session %s is remote and still in the list — locking it would cut off the admin", s.ID)
		}
		if s.State == "closing" {
			t.Errorf("session %s is closing and still in the list", s.ID)
		}
		ids = append(ids, s.ID)
	}
	if want := []string{"1", "3"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("kept %v, want %v", ids, want)
	}
}

// A session that disappears between the listing and the detail call is ordinary —
// someone logged out mid-poll — and must not cost the caller the other sessions.
func TestLogindVanishedSessionDoesNotSinkTheSweep(t *testing.T) {
	listing := `[` +
		`{"session":"gone","uid":1001,"user":"kid","seat":"seat0","leader":1,"class":"user","tty":"tty2","idle":false,"since":null},` +
		`{"session":"here","uid":1001,"user":"kid","seat":"seat0","leader":2,"class":"user","tty":"tty2","idle":false,"since":null}` +
		`]`
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if len(argv) >= 2 && argv[1] == "list-sessions" {
			return []byte(listing), nil
		}
		if len(argv) < 3 {
			t.Errorf("unexpected command: %v", argv)
			return nil, errors.New("unexpected")
		}
		if argv[2] == "gone" {
			return nil, errors.New("loginctl show-session gone exited with 1: Failed to get path for session 'gone': No session 'gone' known")
		}
		return []byte("Name=kid\nRemote=no\nType=x11\nState=active\nLockedHint=no\n"), nil
	}}

	got, err := newLogind(t, r).Sessions(context.Background())
	if err != nil {
		t.Fatalf("Sessions: %v — a vanished session must not be reported as a failure", err)
	}
	if len(got) != 1 || got[0].ID != "here" {
		t.Fatalf("got %v, want just the session that is still there", got)
	}
}

// A per-session failure that is NOT "no session known" is a real problem and has to
// reach the caller, while still leaving the sessions that did answer.
func TestLogindReportsAnUnexpectedSessionFailure(t *testing.T) {
	listing := `[` +
		`{"session":"bad","uid":1001,"user":"kid","seat":"seat0","leader":1,"class":"user","tty":"tty2","idle":false,"since":null},` +
		`{"session":"ok","uid":1001,"user":"kid","seat":"seat0","leader":2,"class":"user","tty":"tty2","idle":false,"since":null}` +
		`]`
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if len(argv) >= 2 && argv[1] == "list-sessions" {
			return []byte(listing), nil
		}
		if len(argv) < 3 {
			t.Errorf("unexpected command: %v", argv)
			return nil, errors.New("unexpected")
		}
		if argv[2] == "bad" {
			return nil, errors.New("loginctl show-session bad exited with 1: Connection reset by peer")
		}
		return []byte("Name=kid\nRemote=no\nType=x11\nState=active\nLockedHint=no\n"), nil
	}}

	got, err := newLogind(t, r).Sessions(context.Background())
	if err == nil {
		t.Fatal("a connection failure was swallowed")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Errorf("error %q does not name the session that failed", err)
	}
	if len(got) != 1 || got[0].ID != "ok" {
		t.Errorf("got %v, want the one session that answered to still be returned", got)
	}
}

func TestLogindLockPinsArgvAndRefusesWhatItMustNotLock(t *testing.T) {
	ctx := context.Background()
	// The lock call, then the confirmation read. LockedHint=yes is what makes Lock
	// return nil: without a confirmation the chain reports ErrUnconfirmed rather than
	// claiming a lock it cannot stand behind.
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if argv[1] == "show-session" {
			return []byte("LockedHint=yes\n"), nil
		}
		return nil, nil
	}}
	p := newLogind(t, r)

	if err := p.Lock(ctx, desktopSession("3")); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	wantArgv(t, r.Argvs(), [][]string{
		{"loginctl", "lock-session", "3"},
		{"loginctl", "show-session", "3", "-p", "LockedHint"},
	})
	if c := r.Calls()[0]; c.uid != 0 || c.env != nil {
		t.Errorf("lock ran as uid %d with env %v, want uid 0 (root) and the inherited environment", c.uid, c.env)
	}

	// Every non-lockable shape has to be refused before anything is run.
	for name, s := range map[string]Session{
		"remote":      {ID: "4", Class: "user", Seat: "seat0", Type: "x11", State: "active", Remote: true},
		"closing":     {ID: "5", Class: "user", Seat: "seat0", Type: "x11", State: "closing"},
		"greeter":     {ID: "5b", Class: "greeter", Seat: "seat0", Type: "x11", State: "active"},
		"no seat":     {ID: "5c", Class: "user", Type: "x11", State: "active"},
		"no type":     {ID: "6", Class: "user", Seat: "seat0", State: "active"},
		"unspecified": {ID: "7", Class: "user", Seat: "seat0", Type: "unspecified", State: "active"},
	} {
		before := len(r.Calls())
		if err := p.Lock(ctx, s); err == nil {
			t.Errorf("Lock(%s) succeeded, want refusal", name)
		}
		if len(r.Calls()) != before {
			t.Errorf("Lock(%s) ran a command before refusing: %v", name, r.Argvs()[before:])
		}
	}

	// A session id that would be read as an option must never reach loginctl, even
	// though it could only have arrived from logind itself.
	for _, id := range []string{"--help", "-p", "--", "2/tty", "a b", ""} {
		s := Session{ID: id, Class: "user", Seat: "seat0", Type: "x11", State: "active"}
		before := len(r.Calls())
		if err := p.Lock(ctx, s); err == nil {
			t.Errorf("Lock with id %q succeeded, want refusal", id)
		}
		if len(r.Calls()) != before {
			t.Errorf("Lock with id %q reached loginctl: %v", id, r.Argvs()[before:])
		}
	}
}

// loginctl exits 0 on Plasma while Plasma ignores the signal. The backend has to report
// that as "not confirmed" rather than success, or the chain stops at logind and the KDE
// backend in #6 is never reached on the platform it exists for.
func TestLogindLockIsNotSuccessUntilLocked(t *testing.T) {
	ctx := context.Background()
	for _, hint := range []string{"no", ""} {
		r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
			if argv[1] == "show-session" {
				if hint == "" {
					return []byte("Type=x11\n"), nil // property absent entirely
				}
				return []byte("LockedHint=" + hint + "\n"), nil
			}
			return nil, nil
		}}
		err := newLogind(t, r).Lock(ctx, desktopSession("3"))
		if err == nil {
			t.Errorf("LockedHint=%q: Lock reported success with the session still unlocked", hint)
			continue
		}
		if !errors.Is(err, ErrUnconfirmed) {
			t.Errorf("LockedHint=%q: err = %v, want it to match ErrUnconfirmed", hint, err)
		}
		if !strings.Contains(err.Error(), "lock-session") && !strings.Contains(err.Error(), "unlocked") {
			t.Errorf("error %q does not say what was attempted", err)
		}
	}
}

// A confirmation read that fails is not a success either. "Could not check" and
// "checked and it is unlocked" are different sentences, and both have to keep the chain
// moving.
func TestLogindConfirmationFailureIsNotSuccess(t *testing.T) {
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		if argv[1] == "show-session" {
			return nil, errors.New("Connection reset by peer")
		}
		return nil, nil
	}}
	err := newLogind(t, r).Lock(context.Background(), desktopSession("3"))
	if err == nil {
		t.Fatal("a failed confirmation reported success")
	}
	if !strings.Contains(err.Error(), "Connection reset by peer") {
		t.Errorf("error %q loses the reason the check failed", err)
	}
}

// Probe is what `plock doctor` will show, and the ticket is explicit that it must not
// lock anything. A probe that locked would lock every session on the machine the
// moment an operator asked "is this working?".
func TestLogindProbeDoesNotLock(t *testing.T) {
	r := serve(t, "list-sessions.json", map[string]string{"3": "show-session-desktop.txt"})
	status := newLogind(t, r).Probe(context.Background())

	if len(status) != 1 {
		t.Fatalf("Probe gave %d statuses, want 1: %+v", len(status), status)
	}
	if !status[0].Available {
		t.Errorf("Probe reports unavailable against a working loginctl: %+v", status[0])
	}
	if status[0].Name != "logind" {
		t.Errorf("Probe named %q, want logind", status[0].Name)
	}
	for _, argv := range r.Argvs() {
		for _, a := range argv {
			if a == "lock-session" {
				t.Errorf("Probe ran a lock: %v", argv)
			}
		}
	}
}

// A machine with no loginctl must produce a sentence an operator can act on, not a
// wrapped stack of exec internals.
func TestLogindWithoutLoginctl(t *testing.T) {
	// The message the real ExecRunner produces for a missing binary, so this tests
	// the backend's handling of it rather than an invented shape.
	r := &fakeRunner{respond: func(argv []string) ([]byte, error) {
		return nil, fmt.Errorf("%s: %w", argv[0], ErrBinaryNotFound)
	}}

	p := newLogind(t, r)
	if _, err := p.Sessions(context.Background()); err == nil {
		t.Fatal("Sessions succeeded with no loginctl")
	} else if !errors.Is(err, ErrBinaryNotFound) {
		t.Errorf("Sessions error %q does not wrap ErrBinaryNotFound, so the CLI cannot say \"install systemd\"", err)
	}

	st := p.Probe(context.Background())
	if len(st) != 1 || st[0].Available {
		t.Errorf("Probe reports %+v, want one unavailable status", st)
	}
	if !strings.Contains(st[0].Detail, "loginctl") {
		t.Errorf("Probe detail %q does not name the missing binary", st[0].Detail)
	}
}

// LockConfirmed queries by id, so it has to apply the same refusal as Lock. A "--help"
// reaching show-session would make loginctl print its usage on stdout, and a usage line
// parsed for LockedHint is a nonsense answer to a security question.
func TestLockConfirmedRefusesAnImplausibleSessionID(t *testing.T) {
	r := &fakeRunner{}
	// newLogind hands back the Chain New builds, so the backend has to be taken out of
	// it. That unwrap is itself the assertion: if logind ever stopped implementing
	// Confirmer the chain would silently downgrade it to "cannot confirm".
	conf, ok := backendAt(t, newLogind(t, r), 0).(Confirmer)
	if !ok {
		t.Fatal("logind does not implement Confirmer; the chain would never be able to stand behind its own locks")
	}
	yes, err := conf.LockConfirmed(context.Background(), desktopSession("--help"))
	if err == nil {
		t.Fatal("LockConfirmed queried an option-shaped session id")
	}
	if yes {
		t.Error("an implausible id reported the session as locked")
	}
	if len(r.Calls()) != 0 {
		t.Errorf("loginctl ran before the id was refused: %v", r.Argvs())
	}
}

// seatString flattens the listing's nullable seat. Both shapes have to be right: a nil
// seat is "no console", and a real seat must survive the flattening rather than become
// the empty string that means the opposite.
func TestSeatString(t *testing.T) {
	if got := seatString(nil); got != "" {
		t.Errorf("seatString(nil) = %q, want the empty string", got)
	}
	seat := "seat0"
	if got := seatString(&seat); got != "seat0" {
		t.Errorf("seatString(&seat0) = %q, want seat0", got)
	}
}
