package enforce

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestTheRecoveryLineMarksARealRecovery is the other half of the guard. Silencing the false line
// is one edit away from deleting the useful one, and the useful one is the only signal an
// operator gets that a stretch of enforcement ran on stale rules and has now stopped.
//
// The order matters: the document has to load successfully once first. A first-ever read that
// fails takes the "nothing to fall back to" path and returns an error without opening a stale
// period, so a test that breaks the file before the first load can never reach the transition it
// is trying to observe.
func TestTheRecoveryLineMarksARealRecovery(t *testing.T) {
	e, _, _, _, path := newEngine(t, kidDoc(), time.Unix(1700000000, 0))
	capture := &logCapture{}
	e.Logf = capture.printf

	dir := filepath.Dir(path)
	if _, err := e.reload(e.Clock(), &Report{}); err != nil {
		t.Fatalf("first load: %v", err)
	}
	if got := capture.count("readable again"); got != 0 {
		t.Fatalf("the first load already reported recovery (%d); nothing was stale", got)
	}

	// Open the stale period: the document is there but cannot be read.
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Skipf("cannot make the config directory unreadable here: %v", err)
	}
	restored := false
	t.Cleanup(func() {
		if !restored {
			os.Chmod(dir, 0o700)
		}
	})

	rep := &Report{}
	if _, err := e.reload(e.Clock(), rep); err != nil {
		t.Fatalf("a reload with a good document held must fall back, not fail: %v", err)
	}
	if len(rep.Errors) == 0 {
		t.Fatal("the unreadable config was not reported; the stale period never opened")
	}
	if got := capture.count("readable again"); got != 0 {
		t.Fatalf("recovery reported while still stale (%d)", got)
	}

	// Close it.
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("restore the directory: %v", err)
	}
	restored = true
	if _, err := e.reload(e.Clock(), &Report{}); err != nil {
		t.Fatalf("reload after the file became readable: %v", err)
	}

	if got := capture.count("readable again"); got != 1 {
		t.Errorf("a genuine recovery reported %d recovery lines, want exactly 1\nlog: %v",
			got, capture.all())
	}
}
