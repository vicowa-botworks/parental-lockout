package audit

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// The documented constructor. A constructor no test calls is a constructor that
// can ship with the wrong defaults, and internal/enforce did exactly that with a
// nil log hook — so this test asserts the defaults are the documented ones and
// that the object it returns actually writes.
func TestNewReturnsAWriterThatWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")

	w := New(path)
	if w.Path != path {
		t.Errorf("Path = %q", w.Path)
	}
	if w.MaxBytes != DefaultMaxBytes {
		t.Errorf("MaxBytes = %d, want the documented %d", w.MaxBytes, DefaultMaxBytes)
	}
	if w.Keep != DefaultKeep {
		t.Errorf("Keep = %d, want the documented %d", w.Keep, DefaultKeep)
	}

	// Constructing must not touch the filesystem. `plock status` builds a writer
	// to read Stats, and it must not create /var/log/plock on a machine that has
	// never run the daemon.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("New created %s: a status query must not provision the log", path)
	}

	if err := w.Write(Record{Kind: "lock.asserted"}); err != nil {
		t.Fatalf("Write on a New writer: %v", err)
	}
	if got := readLines(t, path); len(got) != 1 {
		t.Errorf("got %d lines, want 1", len(got))
	}
	// A New writer has no Logf, and a failure still has to reach the journal.
	var out strings.Builder
	prev := log.Writer()
	log.SetOutput(&out)
	defer log.SetOutput(prev)
	bad := New(filepath.Join(path, "nested", "audit.jsonl")) // path's parent is a file
	if err := bad.Write(Record{Kind: "tick.skipped"}); err == nil {
		t.Fatal("expected the write to fail")
	}
	if !strings.Contains(out.String(), "NOT recorded") {
		t.Errorf("a New writer went silent: %q", out.String())
	}
}

// The zero value is what a caller gets from `&Writer{}`, and it must still be a
// writer with the documented bounds rather than one that never rotates or keeps
// nothing.
func TestTheZeroValueUsesTheDocumentedBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	w := &Writer{Path: path, Logf: func(string, ...any) {}}
	defer w.Close()

	if w.maxBytes() != DefaultMaxBytes {
		t.Errorf("maxBytes = %d, want %d", w.maxBytes(), DefaultMaxBytes)
	}
	if w.keep() != DefaultKeep {
		t.Errorf("keep = %d, want %d", w.keep(), DefaultKeep)
	}
	// Zero means "not set", not "retain nothing". A zero-value Writer is the most
	// likely way one gets built, and reading that as "discard the previous
	// generation on every rotation" would lose the trail with no line anywhere.
	w.Keep = 0
	if w.keep() != DefaultKeep {
		t.Errorf("keep with Keep=0 = %d, want the default: an unset field is not a request to lose the trail", w.keep())
	}
}

// A record that cannot be serialised will never reach the file whatever the
// filesystem does, so it counts as dropped for the same reason the parent reads
// the trail for: the line is missing and nothing says so.
func TestARecordThatCannotBeEncodedIsCountedAsDropped(t *testing.T) {
	w, _, logs := newWriter(t)

	err := w.Write(Record{Kind: "config.changed", Old: json.RawMessage("{ not json")})
	if err == nil {
		t.Fatal("an unencodable record reported success")
	}
	if dropped, consecutive := w.Stats(); dropped != 1 || consecutive != 1 {
		t.Errorf("Stats = (%d, %d), want (1, 1)", dropped, consecutive)
	}
	if !strings.Contains(strings.Join(*logs, "\n"), "encode") {
		t.Errorf("the failure does not say which stage lost the event: %v", *logs)
	}
}

// The audit path swapped for a FIFO *after* the writer had it open. The held
// descriptor still points at the real file, so only the name check notices — and
// "append to the trail" would otherwise keep succeeding against an inode nobody
// can open.
func TestAPathSwappedForAFIFOAfterTheFirstWriteIsCaught(t *testing.T) {
	w, path, _ := newWriter(t)
	if err := w.Write(Record{Kind: "lock.asserted"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("cannot mkfifo: %v", err)
	}

	if err := w.Write(Record{Kind: "lockout.entered"}); err == nil {
		t.Fatal("the writer accepted a path that is no longer the file it opened")
	}
}

// Rotation is allowed to fail — a read-only directory, a full disk — and the
// event already on disk must not be recounted as a dropped one. Conflating the
// two would report a gap in the trail that does not exist, which is the same
// credibility problem in the opposite direction.
func TestAFailedRotationKeepsTheEventAndSaysTheLogWillGrow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	var logs []string
	w := &Writer{Path: path, MaxBytes: 1, Keep: 2, Logf: func(f string, a ...any) { logs = append(logs, f) }}
	defer w.Close()

	if err := w.Write(Record{Kind: "lock.asserted", Detail: "x"}); err != nil {
		t.Fatalf("the event itself must succeed: %v", err)
	}

	// Make rotation impossible by taking the directory away from us.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Skipf("cannot chmod the directory: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	if err := w.Write(Record{Kind: "lock.asserted", Detail: "y"}); err != nil {
		t.Fatalf("a rotation failure must not fail the write: %v", err)
	}
	if dropped, _ := w.Stats(); dropped != 0 {
		t.Errorf("%d events dropped, want 0: the lines are in the file", dropped)
	}
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "keep growing past its size bound") {
		t.Errorf("the rotation failure was not reported as a growing log:\n%s", joined)
	}
}
