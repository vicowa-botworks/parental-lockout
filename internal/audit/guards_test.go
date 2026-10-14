package audit

import (
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
	if !strings.Contains(err.Error(), "owned by another account") {
		t.Errorf("error %q does not name ownership", err)
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
	if fi, serr := os.Stat(target); serr == nil && fi.Mode().Perm() == 0o700 {
		t.Errorf("the linked directory was chmodded to 0700: %v", fi.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(target, "audit.jsonl")); !os.IsNotExist(err) {
		t.Error("the trail was written through the linked directory")
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
