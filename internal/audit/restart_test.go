package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTheTrailSurvivesARestartAndRotationWithoutLosingARetainedLine is ticket #13's
// acceptance criterion — "audit log survives daemon restart and rotates without losing
// a line" — written down instead of left as a manual step.
//
// Every other rotation test in this package drives one Writer through one run. A daemon
// is not one run: it restarts, and the next process finds a file that already carries
// bytes. Two things go wrong exactly there. A writer that opens with O_TRUNC erases the
// history it was asked to keep; and a writer that starts its size accounting at zero
// rotates late, so the file outgrows the bound the operator configured. The second is
// pinned on its own by TestRotationCountsTheBytesAlreadyInTheFile. The first has never
// been pinned across a process boundary: no test here has ever built a second Writer on a
// trail a different Writer already wrote to. The nearest thing is
// TestFailuresInTheMiddleOfAFileDoNotMoveTheBound, where one Writer drops its descriptor
// and reopens it — the same process, still holding its own size, which is a different
// question from a fresh process that remembers nothing.
//
// The assertions go through the reader and the retained files rather than a byte count,
// because that is what "no line lost" has to mean: an operator asking for the last N
// events gets those events, whole, newest first, with no hole where a generation used to
// be.
func TestTheTrailSurvivesARestartAndRotationWithoutLosingARetainedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plock.log")

	const (
		maxBytes = 200
		keep     = 20
		before   = 5 // events written by the process that then exits
		total    = 11
	)
	name := func(i int) string { return fmt.Sprintf("kid%02d", i) }

	// Two things about these numbers are load-bearing and neither is obvious.
	//
	// The window is wider than the run on purpose. With Keep large enough to hold every
	// generation this run produces, a missing record can only mean the trail lost it;
	// retaining two generations would also pass, and would prove nothing, because
	// dropping the oldest is the policy rather than a defect.
	//
	// `before` leaves the live file holding one unrotated record at the moment the
	// process exits. A restart that lands where the live file happens to be empty makes
	// a truncating reopen undetectable — measured, before this was tuned: with six
	// records the live file closed at 0 bytes, O_TRUNC erased nothing, and the test went
	// green against the very bug it exists to catch.
	write := func(w *Writer, i int) {
		if err := w.Write(Record{
			Kind:     "lock.asserted",
			Username: name(i),
			Detail:   strings.Repeat("x", 100),
		}); err != nil {
			t.Fatalf("event %d: %v", i, err)
		}
	}

	first := New(path)
	first.MaxBytes, first.Keep, first.Now = maxBytes, keep, now
	for i := 0; i < before; i++ {
		write(first, i)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("the daemon stopping: %v", err)
	}

	// The restart. A Writer built the way the unit starts one, on the same path, with
	// nothing carried over from the process that just exited — no inherited descriptor,
	// no remembered size.
	second := New(path)
	second.MaxBytes, second.Keep, second.Now = maxBytes, keep, now
	for i := before; i < total; i++ {
		write(second, i)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("the daemon stopping again: %v", err)
	}

	got, err := (&Reader{Path: path, Keep: keep}).Tail(total)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}

	// The vacuity guard. Without a rotation this is a plain append test wearing the
	// criterion's name, so the run has to show rotation happened.
	if got.FilesRead < 2 {
		t.Fatalf("only %d generation(s) were read; the criterion is about rotation, so "+
			"this test would pass without ever having exercised it", got.FilesRead)
	}
	// A line the reader cannot parse is as good as lost, and a half-written append is
	// what a restart mid-write produces.
	if got.Skipped != 0 {
		t.Errorf("Skipped = %d; the reader found lines it could not parse: %v",
			got.Skipped, usernames(got.Records))
	}
	if len(got.Records) != total {
		t.Fatalf("got %d records (%v), want all %d: the trail lost events across the "+
			"restart or the rotation", len(got.Records), usernames(got.Records), total)
	}
	// Newest first, and contiguous. The contiguity is the point: a truncating reopen
	// does not remove the newest event or leave the file short at the boundary, it
	// deletes one record from the middle of the retained trail (measured: kid04 simply
	// absent from an otherwise complete page), which no count of bytes and no check on
	// the newest line would notice.
	for i, rec := range got.Records {
		if want := name(total - 1 - i); rec.Username != want {
			t.Errorf("record %d is %q, want %q; the retained trail has a hole, which is a "+
				"lost line (whole page: %v)", i, rec.Username, want, usernames(got.Records))
		}
	}

	// The bound, which is the other half of "rotates". One generation may overshoot
	// maxBytes by the record that triggered the rotation — the check runs after the
	// write, so 368 bytes against a 200 bound is correct — but never by a whole extra
	// bound. A size counter starting at zero buys exactly that: measured at 552 bytes,
	// which is the per-run bound wearing a per-file bound's clothes.
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, e := range ents {
		fi, err := e.Info()
		if err != nil {
			t.Fatalf("stat %s: %v", e.Name(), err)
		}
		if fi.Size() > 2*maxBytes {
			t.Errorf("%s holds %d bytes against a %d bound; a restart must not add another "+
				"whole bound before the first rotation", e.Name(), fi.Size(), maxBytes)
		}
	}
}
