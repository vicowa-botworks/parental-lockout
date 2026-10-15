package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func now() time.Time { return time.Unix(1700000000, 0).UTC() }

// The bound is a property of the file, not of the process.
//
// Every test in this package used to build its Writer on a fresh t.TempDir(), so no test
// ever opened a file that already had bytes in it — and open() set w.size = 0
// unconditionally. The file is opened O_APPEND, so the bytes landed after what was there
// while the counter that decides rotation started from zero. Measured with MaxBytes 400
// and a restart after two records, the file reached 559 bytes against a 400-byte bound.
//
// That is not a rounding error. DefaultMaxBytes was a per-run bound, so every restart added
// up to another MaxBytes before the first rotation, and the "five generations at 8 MiB is
// 40 MiB worst case" arithmetic behind DefaultKeep did not hold. The shape that grows the
// log is the shape that restarts the daemon: the engine emits KindPlatformError and
// KindLockUnconfirmed per tick, and a machine whose desktop has died is a machine whose
// supervisor is restarting it.
func TestRotationCountsTheBytesAlreadyInTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	const bound = 400

	// A first run writes some records and exits. This is the restart.
	first := &Writer{Path: path, MaxBytes: bound, Keep: 2}
	for i := 0; i < 2; i++ {
		if err := first.Write(Record{Kind: "run1", Detail: "before the restart", At: now()}); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if before.Size() == 0 || before.Size() >= bound {
		t.Fatalf("the fixture wrote %d bytes; it has to leave a file that is part-full but under the bound", before.Size())
	}

	// A second run keeps writing. Rotation must fire at the bound measured from the file's
	// existing length, not from the second run's first byte.
	//
	// "And then it rotates" is not the assertion — the buggy version rotates too, just late.
	// What the bound promises is that the file never grows past it by more than the one
	// record that crossed it, since that record is already committed when the size is
	// checked. Measuring the peak against the bound is what tells the two apart.
	second := &Writer{Path: path, MaxBytes: bound, Keep: 2}
	one := recordBytes(t)
	peak := before.Size()
	rotated := false
	for i := 0; i < 40; i++ {
		if err := second.Write(Record{Kind: "run2", Detail: "after the restart", At: now()}); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Size() < peak {
			rotated = true
			break
		}
		peak = fi.Size()
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if !rotated {
		t.Fatalf("40 records after a restart never rotated; the file peaked at %d bytes against a %d-byte bound", peak, bound)
	}
	if peak > bound+one {
		t.Errorf("the file reached %d bytes against a %d-byte bound, and one record is %d bytes: MaxBytes is counted from this process's first write rather than from the file's end",
			peak, bound, one)
	}
}

// The same reset was reachable without a restart at all.
//
// noteFailure dropped the descriptor and zeroed the size; the next write re-appended to the
// same file and recomputed zero. So a stretch of failures in the middle of a file threw the
// accounting away for the rest of the run — the bound silently became "MaxBytes from the
// last error".
func TestAFailedWriteDoesNotThrowAwayTheSizeAccounting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	const bound = 600

	w := &Writer{Path: path, MaxBytes: bound, Keep: 2}
	for i := 0; i < 3; i++ {
		if err := w.Write(Record{Kind: "warm", Detail: "filling the file", At: now()}); err != nil {
			t.Fatal(err)
		}
	}
	// Force the writer to drop its descriptor, which is what any write failure does.
	if err := w.f.Close(); err != nil {
		t.Fatal(err)
	}
	w.f = nil
	w.noteFailure(nil, "write", os.ErrClosed)

	// The file is still the same file with the same bytes. Reopening must know that.
	if err := w.Write(Record{Kind: "after", Detail: "the failure", At: now()}); err != nil {
		t.Fatal(err)
	}
	if w.size == 0 {
		t.Fatal("the size was reset to zero while the file kept its contents, so the bound moved")
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if w.size != fi.Size() {
		t.Errorf("the writer thinks the file is %d bytes and it is %d", w.size, fi.Size())
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

// recordBytes measures how big one record actually is, so the bound can be asserted with
// the tolerance the implementation genuinely has — one record, because a record is written
// before the size is compared against MaxBytes — instead of a magic number somebody
// guessed. It is measured, not written down, so it cannot go stale when a field is added
// to Record.
func recordBytes(t *testing.T) int64 {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "probe.jsonl")
	w := &Writer{Path: path, MaxBytes: 1 << 20, Keep: 1}
	if err := w.Write(Record{Kind: "run2", Detail: "after the restart", At: now()}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() == 0 {
		t.Fatal("a record wrote no bytes")
	}
	return fi.Size()
}
