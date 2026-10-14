package platform

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The fixture in locker_test.go can describe a process tree, and a description is a
// claim: the fixture says "this pid's argv[0] is ksmserver" and the check reads that
// claim out of a file the fixture wrote. That is structurally unable to catch the attack
// the identity rule exists to stop, because the attack *is* a false claim.
//
// So this file runs the check against the real /proc, with a tree built the way an
// attacker builds one: two execve calls, argv[0] chosen freely, no privilege, nothing
// KDE-related anywhere.

// forgedEnv tells TestMain this process is part of a forged tree rather than a test run.
const forgedEnv = "PLOCK_FORGED_TREE"

// runForgedParent is the impostor compositor. It re-execs this same test binary under a
// chosen argv[0], spawns a child under another chosen argv[0], and reports the child's
// pid so the test can look it up.
func runForgedParent() {
	if os.Getenv(forgedEnv) != "child" {
		return
	}
	// The grandchild is /bin/sleep wearing the locker's name. Its real binary is
	// whatever sleep actually is, which is exactly the point: the name is free and the
	// resolved path is not.
	argv0 := os.Getenv("PLOCK_FORGED_ARGV0")
	if argv0 == "" {
		argv0 = "kscreenlocker_greet"
	}
	cmd := exec.Command("/bin/sleep", "30")
	cmd.Args = []string{argv0, "30"}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "forged parent: %v\n", err)
		os.Exit(1)
	}
	// The test needs the pid, and stdout is the only channel it owns.
	fmt.Printf("CHILD %d\n", cmd.Process.Pid)
	_ = cmd.Wait()
}

// TestTheIdentityCheckRejectsAForgeableProcessTree is the case the fixture cannot
// express: a parent and child whose names are both lies, against the real /proc.
//
// The reviewer built this tree and ran the pre-fix function against it, and the KDE
// backend reported a corroborated lock. Two execve calls as the session user — no
// ptrace, no relationship to a compositor, nothing KDE running — was enough, because
// argv[0] is whatever the caller passed to execve.
func TestTheIdentityCheckRejectsAForgeableProcessTree(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: the uid filter would accept the forged tree for other reasons")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	parent := exec.Command(self, "-test.run=TestNothingMatchesThisPattern")
	// argv[0] is the claim. The kernel records the real binary separately, and that is
	// the difference this check is supposed to see.
	parent.Args = []string{"/usr/bin/ksmserver", "-test.run=TestNothingMatchesThisPattern"}
	parent.Env = append(os.Environ(), forgedEnv+"=child")
	stdout, err := parent.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.Start(); err != nil {
		t.Fatalf("start the impostor compositor: %v", err)
	}
	t.Cleanup(func() {
		if parent.Process != nil {
			parent.Process.Kill()
			parent.Wait()
		}
	})

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("the impostor compositor never reported its child: %v", err)
	}
	fields := strings.Fields(line)
	if len(fields) != 2 || fields[0] != "CHILD" {
		t.Fatalf("unexpected reply from the impostor: %q", line)
	}
	childPID, err := strconv.Atoi(fields[1])
	if err != nil {
		t.Fatal(err)
	}

	// Give /proc a moment to publish the child, then confirm the tree really is what it
	// claims before asserting the check rejects it. Without this the test could pass
	// because the tree was never there.
	deadline := time.Now().Add(3 * time.Second)
	var claimed bool
	for time.Now().Before(deadline) {
		if argv, err := procArgv(strconv.Itoa(childPID)); err == nil && len(argv) > 0 &&
			filepathBase(argv[0]) == "kscreenlocker_greet" {
			claimed = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !claimed {
		t.Fatal("the forged tree did not appear in /proc, so this test measured nothing")
	}

	// The positive control: the same walk, on the same machine, resolves a binary it
	// should. If Readlink were simply failing, the rejection below would be vacuous.
	if _, err := procExe(procRoot, strconv.Itoa(os.Getpid())); err != nil {
		t.Fatalf("the real /proc cannot resolve our own binary (%v), so a rejection here proves nothing", err)
	}

	uid := uint32(os.Geteuid())
	yes, err := lockerRunning(context.Background(), uid)
	if err != nil {
		t.Fatalf("lockerRunning: %v", err)
	}
	if yes {
		t.Error("a tree built from two argv[0] values corroborated a lock: the impostor still only needs to run code as the session user")
	}
}

// TestTheIdentityCheckRejectsABinaryTheSessionUserCanWrite pins the second half of the
// rule. The canonical path is not what makes the real locker trustworthy — the path
// differs by distribution — it is that the session user cannot put their own bytes in it.
func TestTheIdentityCheckRejectsABinaryTheSessionUserCanWrite(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/kscreenlocker_greet"
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o777); err != nil {
		t.Fatal(err)
	}
	uid := uint32(os.Geteuid())
	if !binaryWritableBy(path, uid) {
		t.Error("a binary owned by and writable by the session user was treated as trustworthy")
	}

	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	// Still owned by them. Ownership alone is the disqualifier: they chose the bytes.
	if !binaryWritableBy(path, uid) {
		t.Error("a binary owned by the session user was treated as trustworthy")
	}

	// A file that cannot be read at all is not trusted either: "cannot tell" is answered
	// on the side of not corroborating.
	if !binaryWritableBy(dir+"/does-not-exist", uid) {
		t.Error("an unresolvable binary was treated as trustworthy")
	}
}

// TestAForgeableFixtureTreeIsRejected is the same attack expressed in the fixture, so it
// runs on any machine. It is not a substitute for the case above — a fixture is still a
// claim — but it pins which of the two halves refuses, and it is the one that can run in
// a container with a masked /proc.
func TestAForgeableFixtureTreeIsRejected(t *testing.T) {
	f := newProcFixture(t)
	f.add(7619, 6695, fixtureUID, "/usr/bin/ksmserver")
	// The locker's argv[0] says kscreenlocker_greet; its exe resolves to /bin/sleep.
	f.forge(2224790, 7619, fixtureUID, "/usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet --graceTime 5000", "sleep")
	f.use()

	yes, err := lockerRunning(context.Background(), fixtureUID)
	if err != nil {
		t.Fatal(err)
	}
	if yes {
		t.Error("a locker whose resolved binary is sleep corroborated a lock")
	}
}

// TestALockerTheSessionUserCouldHaveWrittenIsRejected pins the writability half in the
// fixture, where the file's mode can be set to something a real install would never have.
func TestALockerTheSessionUserCouldHaveWrittenIsRejected(t *testing.T) {
	f := newProcFixture(t)
	f.add(7619, 6695, fixtureUID, "/usr/bin/ksmserver")
	f.add(2224790, 7619, fixtureUID, "/usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet")
	f.writableBinary("kscreenlocker_greet")
	f.use()

	yes, err := lockerRunning(context.Background(), fixtureUID)
	if err != nil {
		t.Fatal(err)
	}
	if yes {
		t.Error("a locker binary anybody could write corroborated a lock")
	}
}

// TestALockerWhoseBinaryWasDeletedIsRejected: the kernel marks such a link, and the bytes
// at that path are gone, so whatever is there now was put there by somebody else.
func TestALockerWhoseBinaryWasDeletedIsRejected(t *testing.T) {
	f := newProcFixture(t)
	f.add(7619, 6695, fixtureUID, "/usr/bin/ksmserver")
	f.addWithNoExe(2224790, 7619, fixtureUID, "/usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet")
	f.linkDeletedExe(filepath.Join(f.root, "2224790"), "kscreenlocker_greet")
	f.use()

	yes, err := lockerRunning(context.Background(), fixtureUID)
	if err != nil {
		t.Fatal(err)
	}
	if yes {
		t.Error("a locker whose binary no longer exists corroborated a lock")
	}
}

func filepathBase(s string) string {
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		return s[i+1:]
	}
	return s
}
