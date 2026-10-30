package audit

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// The ownership guard only ever fires for a file owned by somebody else, and an
// unprivileged test cannot create one — so on CI the rule had never run. That is
// worth a seam: the alternative is a tamper check that ships having never fired
// anywhere.
func TestAAuditFileOwnedByAnotherAccountIsRefusedAtAnyUmask(t *testing.T) {
	w, _, _ := newWriter(t)
	if err := w.Write(Record{Kind: "lock.asserted"}); err != nil {
		t.Fatal(err)
	}

	// Pretend the process is running as a different account than the one that owns
	// the file. Nothing about the file changes; only the answer to "who are we".
	prev := geteuid
	geteuid = func() uint32 { return prev() + 1000 }
	t.Cleanup(func() { geteuid = prev })

	err := w.Write(Record{Kind: "lockout.entered"})
	if err == nil {
		t.Fatal("wrote to a trail the writing account does not own")
	}
	if !strings.Contains(err.Error(), "owned by another account") {
		t.Errorf("error %q does not name ownership as the reason", err)
	}
}

// The audit path aimed at /dev/null is not hypothetical — it is what a
// misconfigured unit file looks like — and it is the quietest possible failure:
// every event "succeeds", nothing is recorded anywhere, and the trail reads as a
// machine where nothing ever happened.
//
// This is also the only way to get a descriptor on a non-regular file, since a
// FIFO cannot be opened for append at all.
func TestTheNullDeviceIsRefusedAsAnAuditPath(t *testing.T) {
	if _, err := os.Stat("/dev/null"); err != nil {
		t.Skip("no /dev/null")
	}
	if os.Geteuid() != 0 {
		// /dev is root's, so an unprivileged writer refuses the directory before it
		// ever gets to ask what kind of file /dev/null is. The refusal is correct and
		// is asserted by TestADirectoryOwnedByAnotherAccountIsRefused; this one needs
		// the run to be root to reach the shape check itself.
		t.Skip("reaching /dev/null's shape needs a root run; the directory guard fires first otherwise")
	}
	// The mode is read before the writer is pointed at the node, and put back if the
	// run finds it changed. This test is the only place the real device is reached,
	// and a root run that leaves /dev/null at 0600 does not merely fail — it stops
	// every later process on the machine from opening /dev/null, including the shell
	// the next test runs in. Restoring is the difference between a red test and a
	// bricked runner, and the assertion below is what makes the red test happen.
	before := filePerm(t, "/dev/null")
	t.Cleanup(func() {
		if now := filePerm(t, "/dev/null"); now != before {
			if err := os.Chmod("/dev/null", before); err != nil {
				t.Errorf("the run changed /dev/null from %04o to %04o and it could not be put back: %v", before, now, err)
			}
		}
	})

	var logs []string
	w := &Writer{Path: "/dev/null", Logf: func(f string, a ...any) { logs = append(logs, f) }}
	defer w.Close()

	err := w.Write(Record{Kind: "lockout.entered"})
	if err == nil {
		t.Fatal("the trail was accepted on /dev/null: every event would be discarded and reported as written")
	}
	if !strings.Contains(err.Error(), "regular file") {
		t.Errorf("error %q does not say the path is not a regular file", err)
	}
	if dropped, _ := w.Stats(); dropped != 1 {
		t.Errorf("%d events dropped, want 1: an event that went to /dev/null is a lost event", dropped)
	}
	if now := filePerm(t, "/dev/null"); now != before {
		t.Errorf("refusing /dev/null chmodded it first: mode went %04o to %04o. The shape check runs after the fchmod, so a misconfigured unit file takes the machine's null device down with it",
			before, now)
	}
}

// filePerm reads a path's permission bits for the guards that must prove they left
// the file alone. It is used on ordinary files and on /dev/null alike, so the name says
// what it reads rather than what the first caller passed it.
func filePerm(t *testing.T, path string) os.FileMode {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return fi.Mode().Perm()
}

// A refusal must not leave a mark.
//
// The writer opens the audit path, sets its mode on the held descriptor, and only
// then decides whether the file is one it may append to. That order means every
// refusal at open is also a chmod: the daemon reaches into a file it has just
// decided is not its trail and rewrites its permissions.
//
// On /dev/null that is the machine's null device going to 0600 (the test above,
// root only, and the reason it restores the mode before failing). The same defect
// on a regular file is an arbitrary-chmod primitive with a refusal printed
// afterwards: a root daemon pointed at another account's file tightens it to 0600
// and then declines to write it. The ownership verdict is supplied through the
// statFile seam because an unprivileged run cannot create a file it does not own —
// but the mode asserted at the end is read off the real inode, so it measures
// whether the chmod ran before the verdict rather than restating the fake.
func TestARefusedTrailIsNotChmoddedOnTheWayToRefusingIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plock.log")
	const before os.FileMode = 0o640
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	// WriteFile masks the mode with the umask; the point of this fixture is a file
	// whose mode is a fact, so it is set explicitly rather than requested.
	if err := os.Chmod(path, before); err != nil {
		t.Fatal(err)
	}

	restore := statFile
	statFile = func(f *os.File) (os.FileInfo, error) {
		fi, err := f.Stat()
		if err != nil {
			return nil, err
		}
		return otherOwner{fi}, nil
	}
	t.Cleanup(func() { statFile = restore })

	var logs []string
	w := &Writer{Path: path, Logf: func(f string, a ...any) { logs = append(logs, f) }}
	defer w.Close()

	err := w.Write(Record{Kind: "lockout.entered"})
	if err == nil {
		t.Fatal("appended to a trail the writing account does not own")
	}
	if !strings.Contains(err.Error(), "owned by another account") {
		t.Errorf("error %q does not name ownership as the reason", err)
	}
	if now := filePerm(t, path); now != before {
		t.Errorf("the writer refused the file and chmodded it anyway: mode went %04o to %04o. Deciding not to use a file must not rewrite it",
			before, now)
	}
}

// otherOwner restates a real FileInfo as one belonging to a different account.
// See the note on charDevice for why the verdict needs a seam and the mode does
// not.
type otherOwner struct{ fs.FileInfo }

func (o otherOwner) Sys() any {
	st, ok := o.FileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return o.FileInfo.Sys()
	}
	copied := *st
	copied.Uid = st.Uid + 1000
	return &copied
}

// charDevice restates a real FileInfo as the shape the writer must refuse.
//
// It exists because the shapes that matter — /dev/null, /dev/zero, a block device —
// are all behind a directory an unprivileged test cannot pass the ownership guard
// in, and mknod needs privileges CI does not have. statFile is already a seam for
// exactly this kind of unreachable branch (see its definition); this drives the
// shape check through it with a real descriptor on a real file, so the assertion
// about that file's mode is a measurement and not a restatement of the fake.
type charDevice struct{ fs.FileInfo }

func (charDevice) Mode() fs.FileMode { return os.ModeDevice | os.ModeCharDevice | 0o666 }

// The same ordering, on the shape half: the node is refused, so the node must be
// the node the writer found.
//
// The mode asserted here is the real file's, read from the real filesystem after
// the refusal. The fake only changes what the writer is told about the descriptor;
// if the chmod still runs before the verdict, it lands on the real inode and this
// reads it back.
func TestANonRegularShapeIsRefusedBeforeTheModeIsTouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plock.log")
	const before os.FileMode = 0o644
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, before); err != nil {
		t.Fatal(err)
	}

	restore := statFile
	statFile = func(f *os.File) (os.FileInfo, error) {
		fi, err := f.Stat()
		if err != nil {
			return nil, err
		}
		return charDevice{fi}, nil
	}
	t.Cleanup(func() { statFile = restore })

	var logs []string
	w := &Writer{Path: path, Logf: func(f string, a ...any) { logs = append(logs, f) }}
	defer w.Close()

	err := w.Write(Record{Kind: "lockout.entered"})
	if err == nil {
		t.Fatal("the trail was accepted on a descriptor reported as a character device")
	}
	if !strings.Contains(err.Error(), "regular file") {
		t.Errorf("error %q does not say the path is not a regular file", err)
	}
	if dropped, _ := w.Stats(); dropped != 1 {
		t.Errorf("%d events dropped, want 1: a refused event is a lost event", dropped)
	}
	if now := filePerm(t, path); now != before {
		t.Errorf("the writer refused the shape and chmodded it first: mode went %04o to %04o", before, now)
	}
}

// A symlink planted at the name *after* the first write. The descriptor still
// points at the real file, so this is caught by the name check — but the reason
// has to be reported as the symlink it is, because "no longer refers to the file
// this writer opened" describes a dozen causes and an operator reading the journal
// needs the one that means somebody did this on purpose.
func TestAPathReplacedByASymlinkAfterTheFirstWriteIsNamedAsASymlink(t *testing.T) {
	w, path, _ := newWriter(t)
	if err := w.Write(Record{Kind: "lock.asserted"}); err != nil {
		t.Fatal(err)
	}

	other := filepath.Join(filepath.Dir(path), "elsewhere.jsonl")
	if err := os.WriteFile(other, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, path); err != nil {
		t.Skipf("cannot symlink: %v", err)
	}

	err := w.Write(Record{Kind: "lockout.entered"})
	if err == nil {
		t.Fatal("the writer accepted a path that had become a symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error %q does not name the symlink; the operator cannot tell an attack from a stray rename", err)
	}
	if b, err := os.ReadFile(other); err == nil && string(b) != "" {
		t.Errorf("the linked file was written to: %q", b)
	}
}

// The first-write symlink has to be refused by the open flag, not by the later
// name comparison. The outcome looks the same and the difference is not: without
// O_NOFOLLOW the daemon has already opened the target for append before anything
// complains, and opening is where the damage to a device or a queue happens.
func TestASymlinkAtTheAuditPathIsRefusedByTheOpenItself(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.jsonl")
	if err := os.WriteFile(target, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "audit.jsonl")
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("cannot symlink: %v", err)
	}

	var logs []string
	w := &Writer{Path: path, Logf: func(f string, a ...any) { logs = append(logs, f) }}
	defer w.Close()

	err := w.Write(Record{Kind: "lock.asserted"})
	if err == nil {
		t.Fatal("wrote through a symlink")
	}
	if !strings.Contains(err.Error(), "refusing to write the trail through it") {
		t.Errorf("error %q came from the after-the-fact name check, not from O_NOFOLLOW:\n"+
			"the open itself must never follow the link", err)
	}
}

// The shape check on the held descriptor, reached without root.
//
// A FIFO cannot be opened for append, so the only way to end up holding a
// non-regular descriptor is a path that was regular when we opened it and a
// descriptor we inherited — which is what a future refactor that reuses a
// descriptor would do. Opening read-only with O_NONBLOCK is the one way an
// unprivileged test can produce that state on purpose.
func TestANonRegularDescriptorIsRefusedEvenWhenTheNameLooksRight(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("cannot mkfifo: %v", err)
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Skipf("cannot open a FIFO read-only: %v", err)
	}

	w := &Writer{Path: path, Logf: func(string, ...any) {}}
	w.f = f // the state the guard exists for: a descriptor that is not a file
	if err := w.Write(Record{Kind: "lock.asserted"}); err == nil {
		t.Fatal("wrote through a descriptor on a FIFO")
	} else if !strings.Contains(err.Error(), "regular file") {
		t.Errorf("error %q does not name the shape of the descriptor", err)
	}
}

// The directory guard, reachable at any uid: a directory the writing account does
// not own is one somebody else can move the trail out of, and the daemon must say
// so instead of appending into it.
func TestADirectoryOwnedByAnotherAccountIsRefused(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "notmine")
	if err := os.Mkdir(other, 0o700); err != nil {
		t.Fatal(err)
	}
	prev := geteuid
	geteuid = func() uint32 { return prev() + 1000 }
	t.Cleanup(func() { geteuid = prev })

	w := &Writer{Path: filepath.Join(other, "audit.jsonl"), Logf: func(string, ...any) {}}
	defer w.Close()
	err := w.Write(Record{Kind: "lock.asserted"})
	if err == nil {
		t.Fatal("appended the trail into a directory another account controls")
	}
	// The file-level guard would refuse this fixture too, for a related but
	// different reason. Naming the directory is what pins *this* guard: without it
	// the writer appends into a directory somebody else owns and only the file
	// check, which fires later and covers less, notices anything.
	if !strings.Contains(err.Error(), "directory somebody else controls") {
		t.Errorf("error %q does not name the directory guard", err)
	}
}

// A world-writable directory holding the trail is a trail anybody can rename out
// of the way between two events.
func TestAWorldWritableAuditDirectoryIsRefused(t *testing.T) {
	dir := t.TempDir()
	loose := filepath.Join(dir, "loose")
	// Mkdir's mode is masked by the umask, so the mode has to be set explicitly or
	// the test is quietly checking a directory that was never world-writable.
	if err := os.Mkdir(loose, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(loose, 0o707); err != nil {
		t.Fatal(err)
	}
	w := &Writer{Path: filepath.Join(loose, "audit.jsonl"), Logf: func(string, ...any) {}}
	defer w.Close()
	err := w.Write(Record{Kind: "lock.asserted"})
	if err == nil {
		t.Fatal("accepted a trail directory anybody can write into")
	}
	if !strings.Contains(err.Error(), "0707") {
		t.Errorf("error %q does not name the mode", err)
	}
}

// A symlink where the audit directory should be is refused rather than followed.
// This is the arbitrary-chmod primitive this package used to contain: a root
// daemon that chmods a directory it did not create will chmod whatever a config
// file points it at.
func TestASymlinkedAuditDirectoryIsRefusedNotChmodded(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot symlink: %v", err)
	}

	w := &Writer{Path: filepath.Join(link, "audit.jsonl"), Logf: func(string, ...any) {}}
	defer w.Close()
	err := w.Write(Record{Kind: "lock.asserted"})
	if err == nil {
		t.Fatal("wrote through a symlinked audit directory")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error %q does not name the symlink; the IsDir check below fires for a link too, and an operator cannot tell a link from a plain file by that message", err)
	}
	if fi, serr := os.Stat(target); serr == nil && fi.Mode().Perm() == 0o700 {
		t.Errorf("the linked directory was chmodded to 0700: %v", fi.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(target, "audit.jsonl")); !os.IsNotExist(err) {
		t.Error("the trail was written through the linked directory")
	}
}

// An audit path whose parent is a plain file is a misconfiguration, and the two
// ways to refuse it read very differently in a journal: "not a directory" points
// at the config line, while the errno from opening under a non-directory ("not a
// directory" again, but from the syscall, or "no such file or directory" on some
// filesystems) points at a missing file that is not missing.
func TestAParentThatIsAPlainFileIsReportedAsThat(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "notadir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	w := &Writer{Path: filepath.Join(blocker, "audit.jsonl"), Logf: func(string, ...any) {}}
	defer w.Close()

	err := w.Write(Record{Kind: "lock.asserted"})
	if err == nil {
		t.Fatal("wrote an audit path whose parent is a file")
	}
	// The errno from opening under a non-directory says "not a directory" as well,
	// so the words alone do not prove the guard ran. What distinguishes them is
	// which path is named: the guard names the parent the config got wrong, the
	// errno names a file that is not missing.
	if !strings.Contains(err.Error(), blocker+" is not a directory") {
		t.Errorf("error %q does not name %s as the thing that is not a directory", err, blocker)
	}
}

// Dropping the descriptor after a refused write is what makes "the next event
// re-verifies" true rather than aspirational — and the case that proves it is not
// the refusal, it is the recovery afterwards. A writer that keeps a descriptor it
// has already refused can never adopt the file the path names once the path is
// fixed, so an operator who repairs the permissions finds the trail still broken
// and no error that explains why.
func TestTheWriterAdoptsThePathAgainAfterARefusalIsRepaired(t *testing.T) {
	w, path, _ := newWriter(t)
	if err := w.Write(Record{Kind: "lock.asserted"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Skipf("cannot chmod: %v", err)
	}
	if err := w.Write(Record{Kind: "lock.asserted"}); err == nil {
		t.Fatal("expected the loose mode to be refused")
	}

	// Repair it the way an operator would: replace the file with a correct one.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(Record{Kind: "lockout.exited"}); err != nil {
		t.Fatalf("the writer never recovered after the path was replaced: %v", err)
	}
	if got := readLines(t, path); len(got) != 1 || got[0].Kind != "lockout.exited" {
		t.Errorf("file holds %+v, want the post-repair event in the file the path names", got)
	}
}

// A Writer whose log hook is the default must still reach the journal. Asserted
// separately from New so the rule is pinned even for `&Writer{}`.
func TestTheDefaultLogHookReachesTheStandardLogger(t *testing.T) {
	var out strings.Builder
	prev := log.Writer()
	log.SetOutput(&out)
	defer log.SetOutput(prev)

	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	w := &Writer{Path: path}
	defer w.Close()
	if err := w.Write(Record{Kind: "tick.skipped"}); err == nil {
		t.Fatal("expected the write to fail")
	}
	if out.Len() == 0 {
		t.Error("a Writer with no hook produced no journal line")
	}
}
