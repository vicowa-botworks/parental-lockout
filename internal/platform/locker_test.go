package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

// add writes one process whose exe link resolves to a binary nobody but root could
// have supplied, which is the shape the real desktop presents.
//
// The binary is a real file under the fixture's bin/ directory and the exe link points
// at it, because /proc/<pid>/exe is a symlink the kernel maintains and the check reads
// it with Readlink and then Stats the target. A dangling link would be an honest way to
// model a deleted binary, and that has its own case below.
func (f *procFixture) add(pid, ppid int, uid uint32, argv ...string) *procFixture {
	f.t.Helper()
	f.addWithExe(pid, ppid, uid, baseOf(argv[0]), argv...)
	return f
}

// addWithExe writes one process and names the binary its exe link resolves to.
//
// argv[0] and the resolved binary are separate arguments on purpose: the whole point of
// the check is that they can disagree, and a fixture that could only express them
// together could not express the attack.
func (f *procFixture) addWithExe(pid, ppid int, uid uint32, exeProgram string, argv ...string) *procFixture {
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
	if exeProgram != "" {
		f.linkExe(dir, exeProgram, 0o755)
	}
	return f
}

// addWithNoExe writes a process with no exe link yet, for a case that wants to install
// a particular one afterwards.
func (f *procFixture) addWithNoExe(pid, ppid int, uid uint32, argv ...string) *procFixture {
	f.t.Helper()
	f.addWithExe(pid, ppid, uid, "", argv...)
	return f
}

// linkDeletedExe gives a process the exe link the kernel serves for a binary that was
// unlinked while running: the path with " (deleted)" on the end, pointing at nothing.
func (f *procFixture) linkDeletedExe(dir, program string) {
	f.t.Helper()
	path := filepath.Join(f.binDir(), program+" (deleted)")
	if err := os.Symlink(path, filepath.Join(dir, "exe")); err != nil {
		f.t.Fatalf("link deleted exe: %v", err)
	}
}

// binDir is where the fixture keeps the binaries its exe links point at.
func (f *procFixture) binDir() string {
	f.t.Helper()
	dir := filepath.Join(f.root, "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatalf("mkdir %s: %v", dir, err)
	}
	return dir
}

// linkExe gives a process directory an exe link resolving to a binary called program.
//
// The binary is created 0755 and owned by whoever runs the tests, which is not
// fixtureUID — so it reads as a binary the session user cannot have written, exactly as
// a root-owned file under /usr does on a real machine.
func (f *procFixture) linkExe(dir, program string, mode os.FileMode) {
	f.t.Helper()
	path := filepath.Join(f.binDir(), program)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), mode); err != nil {
			f.t.Fatalf("write %s: %v", path, err)
		}
	}
	if err := os.Chmod(path, mode); err != nil {
		f.t.Fatalf("chmod %s: %v", path, err)
	}
	// The fixture cannot express the situation this package's rule is about if the
	// binary's owner and the session's uid are the same person: every tree would be
	// rejected as forgeable, including the honest ones, and the only way to make the
	// tests go green would be to weaken the rule. Say that out loud instead of leaving
	// the next reader to discover it as four red assertions.
	if owner, ok := fileOwnerUID(path); ok && owner == desktopUID {
		f.t.Fatalf("fixture binary %s is owned by uid %d, which is the session uid this tree is checked against; the honest tree would be rejected for the wrong reason. desktopUID is derived from os.Getuid() and must not collide with the user running the suite", path, owner)
	}
	if err := os.Symlink(path, filepath.Join(dir, "exe")); err != nil {
		f.t.Fatalf("link exe: %v", err)
	}
}

// fileOwnerUID reports the uid that owns path, and whether that could be determined.
func fileOwnerUID(path string) (uint32, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return st.Uid, true
}

// forge writes a process whose argv[0] claims to be one program while its exe resolves
// to something else. This is the impostor: argv[0] is whatever the caller passed to
// execve, so the name is free and the resolved path is not.
func (f *procFixture) forge(pid, ppid int, uid uint32, claimedArgv0, realBinary string) *procFixture {
	f.t.Helper()
	f.addWithExe(pid, ppid, uid, realBinary, claimedArgv0)
	return f
}

// writableBinary makes the fixture's copy of program writable by anybody, modelling a
// binary the session user could have put their own bytes into.
func (f *procFixture) writableBinary(program string) {
	f.t.Helper()
	path := filepath.Join(f.binDir(), program)
	if err := os.Chmod(path, 0o777); err != nil {
		f.t.Fatalf("chmod %s: %v", path, err)
	}
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

// fixtureUID is the uid the fixture's session belongs to.
//
// It is deliberately not the uid running the tests. The identity rule rejects a binary
// the session user could have written, and every file a test creates is owned by the
// test's own uid — so a fixture that used os.Getuid() would have its own honest trees
// rejected, and the way to make them pass would be to weaken the rule. Picking a uid
// nobody here owns keeps the fixture on the same side of the check as the real desktop.
const fixtureUID = uint32(60000)

// The tree measured on the target with the session genuinely locked:
//
//	2224790 7619 1000 /usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet --graceTime 5000 --ksldfd 56
//	7619    6695 1000 /usr/bin/ksmserver
//
// This is the one shape that must corroborate a lock.
func TestLockerRunningAcceptsTheMeasuredTree(t *testing.T) {
	newProcFixture(t).
		add(6695, 1, fixtureUID, "/usr/lib/systemd/systemd --user").
		add(7619, 6695, fixtureUID, measuredKsmserver).
		add(2224790, 7619, fixtureUID, measuredLocker, "--graceTime", "5000", "--ksldfd", "56").
		use()

	yes, err := lockerRunning(context.Background(), fixtureUID)
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
		add(6695, 1, fixtureUID, "/usr/lib/systemd/systemd --user").
		add(7619, 6695, fixtureUID, measuredKsmserver).
		add(300, 7619, fixtureUID, "/usr/bin/plasmashell").
		use()

	yes, err := lockerRunning(context.Background(), fixtureUID)
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
				add(7619, 6695, fixtureUID, measuredKsmserver).
				add(parent.pid, 3000, 1000, parent.argv).
				add(5000, parent.pid, 1000, measuredLocker, "--graceTime", "5000").
				use()

			yes, err := lockerRunning(context.Background(), fixtureUID)
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

	yes, err := lockerRunning(context.Background(), fixtureUID)
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
	f.add(7619, 6695, fixtureUID, measuredKsmserver)
	f.add(2224790, 7619, fixtureUID, measuredLocker)
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

	yes, err := lockerRunning(context.Background(), fixtureUID)
	if err != nil || !yes {
		t.Fatalf("lockerRunning = %v, %v; want the full argv0 to match", yes, err)
	}

	// And a process whose *name* is the truncated locker but whose argv is something
	// else is not the locker. Alone, it must corroborate nothing — that is what a
	// prefix match would get wrong.
	imp := newProcFixture(t)
	imp.add(7619, 6695, fixtureUID, measuredKsmserver)
	imp.add(9000, 7619, fixtureUID, "/usr/bin/kscreenlocker_g")
	imp.use()
	yes, err = lockerRunning(context.Background(), fixtureUID)
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
	f.add(1234, 1, fixtureUID, measuredLocker, "--graceTime", "5000")
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
	f.add(7619, 6695, fixtureUID, measuredKsmserver)
	f.add(2224790, 7619, fixtureUID, measuredLocker)
	dir := filepath.Join(f.root, "2224790")
	raw, err := os.ReadFile(filepath.Join(dir, "status"))
	if err != nil {
		t.Fatal(err)
	}
	// A real/effective pair where only the effective uid is the session's, so the
	// distinction is the only thing that can decide the answer. Both are derived rather
	// than written down: a literal here eventually equals some runner's uid, and the
	// writability rule then rejects the tree for a reason this test is not about.
	realUID := desktopUID + 7
	if err := os.WriteFile(filepath.Join(dir, "status"),
		[]byte(strings.Replace(string(raw),
			fmt.Sprintf("Uid:\t%d\t%d\t%d\t%d", fixtureUID, fixtureUID, fixtureUID, fixtureUID),
			fmt.Sprintf("Uid:\t%d\t%d\t%d\t%d", realUID, desktopUID, realUID, realUID), 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	f.use()

	yes, err := lockerRunning(context.Background(), desktopUID)
	if err != nil || !yes {
		t.Fatalf("lockerRunning for the effective uid = %v, %v; want it to count", yes, err)
	}
	// And the real uid is not the session's.
	if yes, err := lockerRunning(context.Background(), realUID); err != nil || yes {
		t.Errorf("lockerRunning for the real uid = %v, %v; want it not to count", yes, err)
	}
}

func TestLockerRunningEmptyTreeIsNothingNotAnError(t *testing.T) {
	newProcFixture(t).use()

	yes, err := lockerRunning(context.Background(), fixtureUID)
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
	f.add(7619, 6695, fixtureUID, measuredKsmserver)
	f.add(2224790, 7619, fixtureUID, measuredLocker)
	f.use()
	if err := os.WriteFile(filepath.Join(f.root, "self"), []byte("not a pid"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A directory that looks like a pid and vanished before it could be read.
	if err := os.MkdirAll(filepath.Join(f.root, "999999"), 0o755); err != nil {
		t.Fatal(err)
	}

	yes, err := lockerRunning(context.Background(), fixtureUID)
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

	yes, err := lockerRunning(context.Background(), fixtureUID)
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
	newProcFixture(t).add(7619, 6695, fixtureUID, measuredKsmserver).use()

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

// The file's mode is not the whole question, and this is the tree that shows it.
//
// A root-owned 0755 locker inside a directory the session user's group can write -- a
// shared prefix installed root:staff 0775, exactly the non-canonical layout the
// "don't pin a canonical path" decision has to tolerate -- is a locker whose bytes are
// theirs: unlink and recreate, and the name and the mode both still read honest. Checking
// only the file passed this tree while the comment claimed the rule was about authorship.
func TestALockerInAWriteableDirectoryIsRejectedEvenThoughTheBinaryIsNot(t *testing.T) {
	f := newProcFixture(t).
		add(6695, 1, fixtureUID, "/usr/lib/systemd/systemd --user").
		add(7619, 6695, fixtureUID, measuredKsmserver).
		add(2224790, 7619, fixtureUID, measuredLocker, "--graceTime", "5000", "--ksldfd", "56")
	// The binary itself stays root-style: 0755, owned by whoever runs the suite, which
	// is not the session uid -- the self-check in linkExe guarantees that half.
	if err := os.Chmod(f.binDir(), 0o775); err != nil {
		t.Fatal(err)
	}
	f.use()

	yes, err := lockerRunning(context.Background(), fixtureUID)
	if err != nil {
		t.Fatal(err)
	}
	if yes {
		t.Error("a locker sitting in a group-writable directory corroborated a lock: whoever owns the directory owns the bytes, and nothing about the file says so")
	}

	// Control: the same tree with the directory back to 0755 corroborates, so the
	// rejection above is about the directory and not about something else in the fixture.
	if err := os.Chmod(f.binDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if yes, err := lockerRunning(context.Background(), fixtureUID); err != nil || !yes {
		t.Errorf("with the directory at 0755 the same tree gave %v, %v; want it to corroborate, so the assertion above proves nothing", yes, err)
	}
}
