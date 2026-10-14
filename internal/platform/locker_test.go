package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// procFixture builds a directory that walks like /proc, so the locker check can be
// pinned against the exact process tree measured on the target without the test suite
// having to lock a screen. Entries are written from the measured shape: cmdline is
// NUL-terminated, status is the keyed text /proc actually serves.
type procFixture struct {
	t    *testing.T
	root string
}

func newProcFixture(t *testing.T) *procFixture {
	t.Helper()
	return &procFixture{t: t, root: t.TempDir()}
}

// add writes one process. argv is joined with NULs and given the trailing NUL a real
// cmdline carries.
func (f *procFixture) add(pid, ppid int, uid uint32, argv ...string) *procFixture {
	f.t.Helper()
	dir := filepath.Join(f.root, fmt.Sprint(pid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatalf("mkdir %s: %v", dir, err)
	}
	cmdline := strings.Repeat("", 0)
	for _, a := range argv {
		cmdline += a + "\x00"
	}
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644); err != nil {
		f.t.Fatalf("write cmdline: %v", err)
	}
	// Name is deliberately the 15-char truncated form, because that is what the real
	// /proc serves and it is the trap this check has to avoid stepping in.
	status := fmt.Sprintf("Name:\t%s\nUid:\t%d\t%d\t%d\t%d\nPPid:\t%d\n",
		truncate15(baseOf(argv[0])), uid, uid, uid, uid, ppid)
	if err := os.WriteFile(filepath.Join(dir, "status"), []byte(status), 0o644); err != nil {
		f.t.Fatalf("write status: %v", err)
	}
	return f
}

// use points the package at this fixture for the duration of the test.
func (f *procFixture) use() {
	old := procRoot
	procRoot = f.root
	f.t.Cleanup(func() { procRoot = old })
}

// kernelThread writes a process with an empty cmdline, which is what /proc serves for a
// kernel thread.
func (f *procFixture) kernelThread(pid, ppid int) *procFixture {
	f.t.Helper()
	dir := filepath.Join(f.root, fmt.Sprint(pid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), nil, 0o644); err != nil {
		f.t.Fatalf("write cmdline: %v", err)
	}
	status := fmt.Sprintf("Name:\tkthreadd\nUid:\t0\t0\t0\t0\nPPid:\t%d\n", ppid)
	if err := os.WriteFile(filepath.Join(dir, "status"), []byte(status), 0o644); err != nil {
		f.t.Fatalf("write status: %v", err)
	}
	return f
}

func truncate15(s string) string {
	if len(s) > 15 {
		return s[:15]
	}
	return s
}

func baseOf(s string) string {
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		return s[i+1:]
	}
	return s
}

const (
	measuredLocker    = "/usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet"
	measuredKsmserver = "/usr/bin/ksmserver"
)

// The tree measured on the target with the session genuinely locked:
//
//	2224790 7619 1000 /usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet --graceTime 5000 --ksldfd 56
//	7619    6695 1000 /usr/bin/ksmserver
//
// This is the one shape that must corroborate a lock.
func TestLockerRunningAcceptsTheMeasuredTree(t *testing.T) {
	newProcFixture(t).
		add(6695, 1, 1000, "/usr/lib/systemd/systemd --user").
		add(7619, 6695, 1000, measuredKsmserver).
		add(2224790, 7619, 1000, measuredLocker, "--graceTime", "5000", "--ksldfd", "56").
		use()

	yes, err := lockerRunning(context.Background(), 1000)
	if err != nil {
		t.Fatalf("lockerRunning: %v", err)
	}
	if !yes {
		t.Error("the measured locked tree did not corroborate a lock")
	}
}

// A hint with no locker behind it is the spoof, and it is also just "the session said
// so". This is the case the corroborator exists for.
func TestLockerRunningRejectsAHintWithNoLockerBehindIt(t *testing.T) {
	newProcFixture(t).
		add(6695, 1, 1000, "/usr/lib/systemd/systemd --user").
		add(7619, 6695, 1000, measuredKsmserver).
		add(300, 7619, 1000, "/usr/bin/plasmashell").
		use()

	yes, err := lockerRunning(context.Background(), 1000)
	if err != nil {
		t.Fatalf("lockerRunning: %v", err)
	}
	if yes {
		t.Error("a session with no locker process corroborated a lock")
	}
}

// The parent check is not decoration. A locker the session user started themselves —
// from a shell, or from an autostart file — is exactly as easy to produce as the hint
// was, and must not corroborate anything.
func TestLockerRunningRejectsALockerThatIsNotKsmserversChild(t *testing.T) {
	for _, parent := range []struct {
		pid  int
		argv string
		why  string
	}{
		{4000, "/usr/bin/bash", "started from a shell"},
		{4001, "/usr/bin/systemctl", "started as a user unit"},
		{4002, "/home/kid/.config/autostart.sh", "started at login"},
		{4003, measuredLocker, "parented to another copy of itself"},
	} {
		t.Run(parent.why, func(t *testing.T) {
			newProcFixture(t).
				add(7619, 6695, 1000, measuredKsmserver).
				add(parent.pid, 3000, 1000, parent.argv).
				add(5000, parent.pid, 1000, measuredLocker, "--graceTime", "5000").
				use()

			yes, err := lockerRunning(context.Background(), 1000)
			if err != nil {
				t.Fatalf("lockerRunning: %v", err)
			}
			if yes {
				t.Errorf("a locker %s corroborated a lock", parent.why)
			}
		})
	}
}

// The locker belongs to the session it locks. Without the uid check, another user's
// locked session — or a root-owned one — would stand in as evidence for this one.
func TestLockerRunningRejectsAnotherUsersLocker(t *testing.T) {
	newProcFixture(t).
		add(7619, 6695, 1001, measuredKsmserver).
		add(2224790, 7619, 1001, measuredLocker).
		use()

	yes, err := lockerRunning(context.Background(), 1000)
	if err != nil {
		t.Fatalf("lockerRunning: %v", err)
	}
	if yes {
		t.Error("uid 1001's locked session corroborated uid 1000's lock")
	}
}

// /proc's comm field is truncated at 15 characters, so the locker is "kscreenlocker_g"
// there and in ps output. Matching on comm would mean matching on the truncated form,
// which is a prefix collision waiting to happen; matching on cmdline is exact. This
// pins which one the code reads by giving it a tree where the two disagree.
func TestLockerRunningMatchesOnCmdlineNotTheTruncatedName(t *testing.T) {
	f := newProcFixture(t)
	f.add(7619, 6695, 1000, measuredKsmserver)
	f.add(2224790, 7619, 1000, measuredLocker)
	f.use()

	// Sanity: the fixture really does carry the truncated name, or this test proves
	// nothing about the trap.
	raw, err := os.ReadFile(filepath.Join(f.root, "2224790", "status"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Name:\tkscreenlocker_g\n") {
		t.Errorf("fixture status is %q, want the 15-char truncated name", raw)
	}

	yes, err := lockerRunning(context.Background(), 1000)
	if err != nil || !yes {
		t.Fatalf("lockerRunning = %v, %v; want the full argv0 to match", yes, err)
	}

	// And a process whose *name* is the truncated locker but whose argv is something
	// else is not the locker. Alone, it must corroborate nothing — that is what a
	// prefix match would get wrong.
	imp := newProcFixture(t)
	imp.add(7619, 6695, 1000, measuredKsmserver)
	imp.add(9000, 7619, 1000, "/usr/bin/kscreenlocker_g")
	imp.use()
	yes, err = lockerRunning(context.Background(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if yes {
		t.Error("a program merely *named* like the truncated locker corroborated a lock")
	}
}

// /proc/<pid>/cmdline ends every argument with a NUL, so a naive split leaves an empty
// field on the end of every argv. Nothing here reads argv[1], so a check that only ever
// looks at argv[0] would not notice — which is exactly why it gets asserted.
func TestProcArgvDropsTheTrailingNulField(t *testing.T) {
	f := newProcFixture(t)
	f.add(1234, 1, 1000, measuredLocker, "--graceTime", "5000")
	f.use()

	argv, err := procArgv("1234")
	if err != nil {
		t.Fatalf("procArgv: %v", err)
	}
	want := []string{measuredLocker, "--graceTime", "5000"}
	if len(argv) != len(want) {
		t.Fatalf("argv = %q (%d fields), want %d: the trailing NUL was left in", argv, len(argv), len(want))
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Fatalf("argv = %q, want %q", argv, want)
		}
	}
}

// The uid that owns a locker's D-Bus connection is the effective one, and it is the one
// a setuid helper differs on. Reading the real uid instead would accept a locker that is
// not the session's.
func TestLockerRunningMatchesTheEffectiveUidNotTheRealOne(t *testing.T) {
	f := newProcFixture(t)
	f.add(7619, 6695, 1000, measuredKsmserver)
	f.add(2224790, 7619, 1000, measuredLocker)
	dir := filepath.Join(f.root, "2224790")
	raw, err := os.ReadFile(filepath.Join(dir, "status"))
	if err != nil {
		t.Fatal(err)
	}
	// real 1000, effective 1001.
	if err := os.WriteFile(filepath.Join(dir, "status"),
		[]byte(strings.Replace(string(raw), "Uid:\t1000\t1000\t1000\t1000", "Uid:\t1000\t1001\t1000\t1000", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	f.use()

	yes, err := lockerRunning(context.Background(), 1001)
	if err != nil || !yes {
		t.Fatalf("lockerRunning for the effective uid = %v, %v; want it to count", yes, err)
	}
	// And the real uid is not the session's.
	if yes, err := lockerRunning(context.Background(), 1000); err != nil || yes {
		t.Errorf("lockerRunning for the real uid = %v, %v; want it not to count", yes, err)
	}
}

func TestLockerRunningEmptyTreeIsNothingNotAnError(t *testing.T) {
	newProcFixture(t).use()

	yes, err := lockerRunning(context.Background(), 1000)
	if err != nil {
		t.Fatalf("lockerRunning: %v", err)
	}
	if yes {
		t.Error("an empty /proc corroborated a lock")
	}
}

// Kernel threads and non-numeric entries are the normal noise in /proc. Neither is a
// fault, and a walk that treated one as a failure would report every lock unconfirmed on
// a healthy machine.
func TestLockerRunningWalkesPastOrdinaryProcNoise(t *testing.T) {
	f := newProcFixture(t)
	f.kernelThread(1, 0)
	f.kernelThread(2, 0)
	f.add(7619, 6695, 1000, measuredKsmserver)
	f.add(2224790, 7619, 1000, measuredLocker)
	f.use()
	if err := os.WriteFile(filepath.Join(f.root, "self"), []byte("not a pid"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A directory that looks like a pid and vanished before it could be read.
	if err := os.MkdirAll(filepath.Join(f.root, "999999"), 0o755); err != nil {
		t.Fatal(err)
	}

	yes, err := lockerRunning(context.Background(), 1000)
	if err != nil {
		t.Fatalf("lockerRunning: %v", err)
	}
	if !yes {
		t.Error("ordinary /proc noise stopped the walk before the locker")
	}
}

// "Cannot look" must not arrive in the same shape as "looked and found nothing": the
// caller files the second as unconfirmed and the first as a fault worth naming.
func TestLockerRunningErrorsWhenProcCannotBeRead(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-proc")
	old := procRoot
	procRoot = missing
	t.Cleanup(func() { procRoot = old })

	yes, err := lockerRunning(context.Background(), 1000)
	if err == nil {
		t.Fatal("an unreadable procRoot reported " + fmt.Sprint(yes) + " instead of faulting")
	}
	if yes {
		t.Error("a failed lookup corroborated a lock")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q does not name the path it could not open", err)
	}
}

func TestLockerRunningHonoursACancelledContext(t *testing.T) {
	newProcFixture(t).add(7619, 6695, 1000, measuredKsmserver).use()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := lockerRunning(ctx, 1000); !errors.Is(err, context.Canceled) {
		t.Errorf("lockerRunning after cancel = %v, want context.Canceled", err)
	}
}

// The status parser reads by key, not by position, because a command name may contain
// spaces and parentheses. A fixture whose Name line carries both is the only way to say
// that without a process whose name does.
func TestProcParentSurvivesANameWithSpacesAndParens(t *testing.T) {
	f := newProcFixture(t)
	dir := filepath.Join(f.root, "4242")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(measuredLocker+"\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	status := "Name:\tkscreenlocker_g\nUid:\t1000\t1000\t1000\t1000\nGid:\t1000\t1000\t1000\t1000\n" +
		"Groups:\t27 100\nPPid:\t7619\nTracerPid:\t0\nFDSize:\t64\nVmPeak:\t   12345 kB\n"
	if err := os.WriteFile(filepath.Join(dir, "status"), []byte(status), 0o644); err != nil {
		t.Fatal(err)
	}
	f.use()

	ppid, uid, err := procParent("4242")
	if err != nil {
		t.Fatalf("procParent: %v", err)
	}
	if ppid != 7619 || uid != 1000 {
		t.Errorf("ppid=%d uid=%d, want 7619 and 1000", ppid, uid)
	}
}

func TestProcParentRejectsWhatItCannotTrust(t *testing.T) {
	for name, body := range map[string]string{
		"no PPid":          "Name:\tx\nUid:\t1000\t1000\t1000\t1000\n",
		"no Uid":           "Name:\tx\nPPid:\t7619\n",
		"PPid not numeric": "PPid:\tnope\nUid:\t1000\t1000\t1000\t1000\n",
		"Uid truncated":    "Uid:\t1000\n",
		"Uid not numeric":  "Uid:\tkid\tkid\tkid\tkid\n",
	} {
		t.Run(name, func(t *testing.T) {
			f := newProcFixture(t)
			dir := filepath.Join(f.root, "77")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "status"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			f.use()

			if _, _, err := procParent("77"); err == nil {
				t.Errorf("accepted a status it cannot read: %q", body)
			}
		})
	}
}
