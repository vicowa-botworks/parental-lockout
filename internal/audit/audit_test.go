package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// newWriter returns a writer on a private path with the log captured, so a test
// asserts on what the operator would have seen instead of on the daemon's stderr.
func newWriter(t *testing.T) (*Writer, string, *[]string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	var logs []string
	var mu sync.Mutex
	w := &Writer{
		Path: path,
		Keep: DefaultKeep,
		Now:  func() time.Time { return time.Date(2026, 10, 14, 23, 41, 7, 0, time.UTC) },
		Logf: func(format string, args ...any) {
			mu.Lock()
			defer mu.Unlock()
			logs = append(logs, fmt.Sprintf(format, args...))
		},
	}
	t.Cleanup(func() { w.Close() })
	return w, path, &logs
}

// readLines returns the file's contents as parsed records, asserting every line
// is whole. A torn JSON line is as useless as a missing one, and it is the shape
// a half-written append produces.
func readLines(t *testing.T, path string) []line {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(b)
	if s != "" && !strings.HasSuffix(s, "\n") {
		t.Fatalf("%s does not end in a newline; the next append would join two events", path)
	}
	var out []line
	for _, l := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		if l == "" {
			continue
		}
		var rec line
		if err := json.Unmarshal([]byte(l), &rec); err != nil {
			t.Fatalf("line %q is not JSON: %v", l, err)
		}
		out = append(out, rec)
	}
	return out
}

func TestWriteAppendsOneJSONLinePerRecord(t *testing.T) {
	w, path, _ := newWriter(t)

	if err := w.Write(Record{Kind: "lockout.entered", Username: "casey", RuleID: "bedtime"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Write(Record{Kind: "lock.asserted", Username: "casey", SessionID: "3"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got := readLines(t, path)
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2", len(got))
	}
	if got[0].Kind != "lockout.entered" || got[0].Username != "casey" || got[0].RuleID != "bedtime" {
		t.Errorf("first line = %+v", got[0])
	}
	// The timestamp is filled in when the caller has no clock, because a line
	// without one cannot answer the only question the trail gets asked.
	if got[0].At != "2026-10-14T23:41:07Z" {
		t.Errorf("at = %q, want the writer's clock", got[0].At)
	}
	if got[1].SessionID != "3" || got[1].RuleID != "" {
		t.Errorf("second line = %+v, want sessionId 3 and no ruleId", got[1])
	}
}

// A config mutation carries what changed. The distinction between "there was no
// previous value" and "the previous value was empty" is the difference between a
// parent adding a rule and clearing one, and a string cannot make it.
func TestOldAndNewCarryThePreviousValue(t *testing.T) {
	w, path, _ := newWriter(t)

	added, _ := json.Marshal(map[string]any{"start": "21:00"})
	if err := w.Write(Record{
		Kind: "config.changed", Username: "casey", Actor: "parent",
		Old: nil, New: added,
	}); err != nil {
		t.Fatal(err)
	}
	cleared := json.RawMessage(`null`)
	if err := w.Write(Record{
		Kind: "config.changed", Username: "casey", Actor: "parent",
		Old: added, New: cleared,
	}); err != nil {
		t.Fatal(err)
	}

	got := readLines(t, path)
	if got[0].Old != nil {
		t.Errorf("a new rule reports old = %s, want the field absent", got[0].Old)
	}
	if string(got[1].Old) != `{"start":"21:00"}` {
		t.Errorf("old = %s, want the previous rule as it was", got[1].Old)
	}
	if string(got[1].New) != "null" {
		t.Errorf("new = %s, want an explicit null: clearing a field is not the same as never setting it", got[1].New)
	}
}

func TestRotationShiftsGenerationsAndDropsTheOldest(t *testing.T) {
	w, path, logs := newWriter(t)
	w.MaxBytes = 200
	w.Keep = 2

	// Each record is ~120 bytes, so two records per file and three rotations.
	for i := 0; i < 9; i++ {
		if err := w.Write(Record{Kind: "test.filler", Detail: strings.Repeat("x", 100)}); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	for _, name := range []string{path, path + ".1", path + ".2"} {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("%s missing: %v", filepath.Base(name), err)
		}
	}
	if _, err := os.Stat(path + ".3"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s.3 exists; Keep=2 must not retain a third generation", filepath.Base(path))
	}
	// The current file is the one still being appended to, and it must be smaller
	// than the bound rather than having grown past it.
	if fi, err := os.Stat(path); err == nil && fi.Size() > 400 {
		t.Errorf("current log is %d bytes after rotating at 200", fi.Size())
	}
	for _, l := range *logs {
		if strings.Contains(l, "could not rotate") {
			t.Errorf("unexpected rotation failure: %s", l)
		}
	}
}

// Keep=1 is the smallest honest configuration: the file in front of you and the
// one before it. It must not become the default five, and it must not lose the
// generation it is replacing before the rename lands.
func TestKeepingOneGenerationStillShiftsItAside(t *testing.T) {
	w, path, _ := newWriter(t)
	w.MaxBytes = 100
	w.Keep = 1

	for i := 0; i < 4; i++ {
		if err := w.Write(Record{Kind: "test.filler", Detail: strings.Repeat("y", 90)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Errorf("%s.1 missing with Keep=1: %v", filepath.Base(path), err)
	}
	if _, err := os.Stat(path + ".2"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s.2 exists with Keep=1", filepath.Base(path))
	}
}

// The audit directory holds the trail. A child who can list it can time their
// attempts against it, so the mode is a requirement and not a preference.
func TestTheDirectoryIsCreatedPrivate(t *testing.T) {
	w, path, _ := newWriter(t)
	// A directory the writer did not make is checked, not corrected — see
	// prepareDir. This is the case where it makes one.
	path = filepath.Join(path, "deeper", "audit.jsonl")
	w.Path = path
	_ = path
	if err := w.Write(Record{Kind: "tick.skipped"}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o700 {
		t.Errorf("directory mode = %v, want 0700", fi.Mode().Perm())
	}
}

// PLAN §10's 0600 has to survive the umask. The mode passed to open is masked by
// it, so a daemon started under a permissive umask would otherwise create a trail
// every account on the machine can read, and nothing would say so.
// PLAN §10's 0600 has to survive the umask. The mode passed to open is masked by
// it, so a daemon started under a permissive — or, worse, a *restrictive* — umask
// would otherwise create a trail whose mode is an accident of how the service was
// started, and nothing would say so.
//
// Umask is process-wide, so this test cannot run beside others that open files.
func TestTheFileModeIsTightenedRegardlessOfUmask(t *testing.T) {
	t.Helper()
	// The directory is made first, under the normal umask: the mask below strips
	// the owner's own bits, which would make a freshly created directory one we
	// could not even enter, and that would test the directory rule rather than
	// this one.
	w, path, _ := newWriter(t)
	restore := forceUmask(t, 0o600)

	if err := w.Write(Record{Kind: "lock.asserted"}); err != nil {
		t.Fatalf("write under umask 600: %v", err)
	}
	restore()

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("audit file mode = %v, want 0600: the open's mode is a request, the descriptor's mode is the result", fi.Mode().Perm())
	}
}

// forceUmask sets the process umask and returns a function that puts it back.
// Returning the restorer rather than deferring it is deliberate: the caller needs
// the window to cover exactly the operation under test, and a defer that fires
// before it is the bug this helper replaced.
func forceUmask(t *testing.T, mask int) func() {
	t.Helper()
	prev := syscall.Umask(mask)
	restored := false
	restore := func() {
		if !restored {
			restored = true
			syscall.Umask(prev)
		}
	}
	t.Cleanup(restore)
	return restore
}

// The same rule one level up: the directory the trail lives in. Under a
// restrictive umask MkdirAll delivers nothing close to 0700 — umask 0777 turns
// 0700 into 0000 — so the mode set on the descriptor is the only thing that makes
// the directory private.
func TestACreatedDirectoryIsPrivateEvenUnderARestrictiveUmask(t *testing.T) {
	dir := t.TempDir()
	// 700 strips every bit of 0700, so MkdirAll alone delivers a directory nobody
	// can open. Only the mode set on the descriptor makes it usable.
	restore := forceUmask(t, 0o700)

	path := filepath.Join(dir, "plock", "audit.jsonl")
	w := &Writer{Path: path, Logf: func(string, ...any) {}}
	defer w.Close()
	if err := w.Write(Record{Kind: "lock.asserted"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	restore()

	fi, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o700 {
		t.Errorf("created directory mode = %v, want 0700", fi.Mode().Perm())
	}
}

func TestAWidelyReadableAuditFileIsRefused(t *testing.T) {
	w, path, logs := newWriter(t)
	if err := w.Write(Record{Kind: "lock.asserted"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Skipf("cannot chmod as this uid: %v", err)
	}

	err := w.Write(Record{Kind: "lockout.entered"})
	if err == nil {
		t.Fatal("wrote to a trail that every account can read")
	}
	if !strings.Contains(err.Error(), "0644") {
		t.Errorf("error %q does not name the mode that is wrong", err)
	}
	// Refusing is not the same as silently fixing: the operator has to know a
	// window existed.
	if !strings.Contains(strings.Join(*logs, "\n"), "NOT recorded") {
		t.Errorf("the refusal was not logged: %v", *logs)
	}
}

// A symlink at the audit path is the cheap version of this attack: the daemon is
// root, the path is predictable, and a link turns "append to the trail" into
// "append to whatever root can write".
func TestASymlinkAtTheAuditPathIsRefused(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "shadow")
	if err := os.WriteFile(victim, []byte("root:x:0:0:\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "audit.jsonl")
	if err := os.Symlink(victim, path); err != nil {
		t.Skipf("cannot symlink: %v", err)
	}

	var logs []string
	w := &Writer{Path: path, Keep: DefaultKeep, Logf: func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) }}
	defer w.Close()

	if err := w.Write(Record{Kind: "lock.asserted"}); err == nil {
		t.Fatal("wrote through a symlink")
	}
	b, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "root:x:0:0:\n" {
		t.Errorf("the linked file was modified: %q", b)
	}
}

// The fail-open shape. A FIFO at the audit path makes an ordinary open *wait for
// a writer that never arrives*, and this code runs inside the enforcement tick: a
// parked tick stops re-locking, so a child who can mkfifo turns enforcement off
// without touching the daemon. O_NONBLOCK turns that into an immediate error.
func TestAFIFOAtTheAuditPathFailsFastInsteadOfParkingTheTick(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("cannot mkfifo: %v", err)
	}

	w := &Writer{Path: path, Keep: DefaultKeep, Logf: func(string, ...any) {}}
	// Deliberately no `defer w.Close()`. If the open parks, it parks holding the
	// writer's mutex, and a Close that waits for that mutex turns this test into a
	// 900-second package timeout with a goroutine dump instead of the sentence
	// below. The leaked goroutine dies with the test binary.

	done := make(chan error, 1)
	go func() { done <- w.Write(Record{Kind: "lock.asserted"}) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("writing to a FIFO succeeded")
		}
		if !strings.Contains(err.Error(), "FIFO") {
			t.Errorf("error %q does not say what the path is", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Write parked on a FIFO: a tick parked here stops re-locking the child")
	}
}

// The descriptor is held between writes for speed, which means the name can be
// repointed while the writer keeps appending to the old inode. The next event has
// to notice.
func TestAPathReplacedUnderTheWriterIsCaught(t *testing.T) {
	w, path, _ := newWriter(t)
	if err := w.Write(Record{Kind: "lock.asserted"}); err != nil {
		t.Fatal(err)
	}

	// Move the real file away and leave a different one under the name.
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("planted\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := w.Write(Record{Kind: "lockout.entered"}); err == nil {
		t.Fatal("the writer kept appending to a file the path no longer names")
	}
	planted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(planted) != "planted\n" {
		t.Errorf("the planted file was appended to: %q", planted)
	}
}

// The policy the ticket exists to settle. A lost event is counted, and it is
// reported on every occurrence: a daemon that logs once and goes quiet is
// indistinguishable from one whose trail recovered.
func TestAFailedWriteIsCountedAndReportedEveryTime(t *testing.T) {
	// A directory where the file should be makes every open fail for a reason that
	// is not a race with the test itself.
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}

	var logs []string
	w := &Writer{Path: path, Logf: func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) }}
	defer w.Close()

	for i := 0; i < 5; i++ {
		if err := w.Write(Record{Kind: "lockout.entered"}); err == nil {
			t.Fatalf("write %d succeeded against a directory", i)
		}
	}

	dropped, consecutive := w.Stats()
	if dropped != 5 || consecutive != 5 {
		t.Errorf("Stats = (%d, %d), want (5, 5): five events never reached the file", dropped, consecutive)
	}
	// Every one of them said something. This is the assertion that separates the
	// policy from the tempting default.
	reported := 0
	for _, l := range logs {
		if strings.Contains(l, "NOT recorded") || strings.Contains(l, "STILL FAILING") {
			reported++
		}
	}
	if reported != 5 {
		t.Errorf("%d of 5 failures were logged; a trail that goes quiet reads as a trail that recovered", reported)
	}
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "STILL FAILING") || !strings.Contains(joined, "enforcement is running unrecorded") {
		t.Errorf("no escalation for a run of failures:\n%s", joined)
	}
}

// The escalation is one line per run, not one per event: a filesystem that stays
// broken must not fill the journal it is complaining about.
func TestEscalationSaysItIsContinuingOnceRatherThanEveryTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	var logs []string
	w := &Writer{Path: path, Logf: func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) }}
	defer w.Close()

	for i := 0; i < 50; i++ {
		w.Write(Record{Kind: "tick.skipped"})
	}
	escalations := 0
	for _, l := range logs {
		if strings.Contains(l, "STILL FAILING") {
			escalations++
		}
	}
	if escalations != 1 {
		t.Errorf("%d escalations over 50 failures, want 1 that says the run is continuing", escalations)
	}
}

// Recovery has to be as visible as the failure, or the last thing the operator
// read is a warning with no answer.
func TestTheTrailReportsWhenItRecovers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "audit.jsonl")

	var logs []string
	w := &Writer{Path: path, Logf: func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) }}
	defer w.Close()

	// The parent directory is a file, so the open fails; replacing it with a
	// directory is the recovery.
	if err := os.WriteFile(filepath.Join(dir, "sub"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(Record{Kind: "lockout.entered"}); err == nil {
		t.Fatal("expected the first write to fail")
	}
	if err := os.Remove(filepath.Join(dir, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(Record{Kind: "lockout.exited"}); err != nil {
		t.Fatalf("write after recovery: %v", err)
	}

	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "trail resumed") {
		t.Errorf("recovery was not reported:\n%s", joined)
	}
	if dropped, consecutive := w.Stats(); dropped != 1 || consecutive != 0 {
		t.Errorf("Stats after recovery = (%d, %d), want the drop kept and the failure run cleared", dropped, consecutive)
	}
	// The recovered line is in the file; the lost one is not, and that asymmetry
	// is what Stats exists to disclose.
	if got := readLines(t, path); len(got) != 1 || got[0].Kind != "lockout.exited" {
		t.Errorf("file holds %+v, want only the event written after recovery", got)
	}
}

// A Writer built without a log hook — the zero value, or any constructor that
// forgets — must still make noise. Losing the signal is the one outcome this
// package must not produce, and internal/enforce shipped exactly this hole in its
// constructor once already.
func TestADefaultWriterStillReportsItsFailures(t *testing.T) {
	var out strings.Builder
	// The default hook is the standard logger, so that is what a default Writer
	// has to reach. Restored before the test returns.
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
	if err := w.Write(Record{Kind: "lockout.entered"}); err == nil {
		t.Fatal("expected the write to fail")
	}
	if !strings.Contains(out.String(), "NOT recorded") {
		t.Errorf("a default Writer went silent: %q", out.String())
	}
}

// The engine emits from the tick and #10's handlers emit from requests, both into
// one stream. Interleaving inside a line would make the trail unparseable, which
// is as bad as losing it.
func TestConcurrentWritersProduceWholeLines(t *testing.T) {
	w, path, _ := newWriter(t)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				if err := w.Write(Record{
					Kind: "lock.asserted", Username: fmt.Sprintf("kid%d", i),
					Detail: strings.Repeat("z", 200),
				}); err != nil {
					t.Errorf("concurrent write: %v", err)
					return
				}
			}
		}(i)
	}
	wg.Wait()

	if got := readLines(t, path); len(got) != 200 {
		t.Errorf("got %d lines, want 200", len(got))
	}
}

// Rotation under concurrency. The invariant is not "every line survives" — past
// Keep generations are deleted on purpose, and a test that expected otherwise
// would be asking the writer to retain a trail its operator bounded. The
// invariant is that no write fails and no line is torn while the file underneath
// it is being renamed and reopened.
func TestRotationUnderConcurrencyNeverFailsOrTearsALine(t *testing.T) {
	w, path, logs := newWriter(t)
	w.MaxBytes = 512

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				if err := w.Write(Record{Kind: "lock.asserted", Detail: strings.Repeat("q", 120)}); err != nil {
					t.Errorf("write across a rotation: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	// Rotation really happened, or the test proved nothing.
	if _, err := os.Stat(path + ".5"); err != nil {
		t.Fatalf("no fifth generation after 160 writes at a 512-byte bound: rotation did not run")
	}
	// Every retained generation parses line by line. A write that landed in a
	// descriptor closed mid-rotation shows up here as a torn or missing newline.
	for _, name := range []string{path, path + ".1", path + ".2", path + ".3", path + ".4", path + ".5"} {
		if _, err := os.Stat(name); err != nil {
			continue
		}
		readLines(t, name)
	}
	for _, l := range *logs {
		if strings.Contains(l, "NOT recorded") || strings.Contains(l, "could not rotate") {
			t.Errorf("rotation reported a problem: %s", l)
		}
	}
	if dropped, fails := w.Stats(); dropped != 0 || fails != 0 {
		t.Errorf("Stats = (%d, %d), want (0, 0)", dropped, fails)
	}
}

// The rule's human name has to survive the trip to disk and back.
//
// Two separate losses are possible and this test now pins both. Dropping the field from
// the serialiser's field list compiles, and Record still holds the value in memory, so the
// loss is only visible in the file. Dropping it from decodeRecord's field list also
// compiles, and is invisible even in the file -- which is the one this test used to miss.
// Its last line used to parse with readLines, which unmarshals into `line`, the wire
// struct, so it asserted the wire shape twice and never once asked the reader for a
// Record. The comment below it claimed "nothing read it back until this test"; that claim
// was false, and issue #81 is the bug it let through.
//
// RuleID alone answers "which rule" with an opaque key; RuleName is the half a parent
// recognises, and the engine computes both for exactly this reason.
func TestRuleNameReachesTheTrail(t *testing.T) {
	w, path, _ := newWriter(t)

	if err := w.Write(Record{
		Kind:     "lockout.entered",
		Username: "casey",
		RuleID:   "r-7",
		RuleName: "School week",
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	raw := strings.TrimSpace(mustRead(t, path))
	// Asserted on the wire, not only on the parsed struct, so renaming the JSON key
	// fails here too: the reader and every other tool parse by that key.
	var onWire map[string]any
	if err := json.Unmarshal([]byte(raw), &onWire); err != nil {
		t.Fatalf("line is not JSON: %v\n%s", err, raw)
	}
	if got := onWire["ruleName"]; got != "School week" {
		t.Errorf("ruleName on the wire = %#v, want %q (line: %s)", got, "School week", raw)
	}
	if got := onWire["ruleId"]; got != "r-7" {
		t.Errorf("ruleId on the wire = %#v, want %q", got, "r-7")
	}

	got := readLines(t, path)
	if got[0].RuleName != "School week" {
		t.Errorf("parsed RuleName = %q, want %q", got[0].RuleName, "School week")
	}

	// And through the reader, which is the only path anything on the dashboard takes.
	// Without this half the test above is a check on the writer wearing a reader's name.
	tail, err := (&Reader{Path: path}).Tail(1)
	if err != nil {
		t.Fatalf("reading the trail back: %v", err)
	}
	if len(tail.Records) != 1 {
		t.Fatalf("the reader returned %d records, want 1", len(tail.Records))
	}
	if tail.Records[0].RuleName != "School week" {
		t.Errorf("RuleName read back through the reader = %q, want %q: the value is on disk, "+
			"so this is decodeRecord dropping it", tail.Records[0].RuleName, "School week")
	}
}

// An event with no rule must not grow an empty ruleName.
//
// The trail is read by a human grepping for a name, and "ruleName":"" on every
// daemon-wide line makes it impossible to tell "no rule" from "a rule whose name was
// lost" -- the omitempty distinction is the whole answer to that question.
func TestRuleNameOmittedWhenThereIsNoRule(t *testing.T) {
	w, path, _ := newWriter(t)

	if err := w.Write(Record{Kind: "enforcement.disabled", Detail: "operator stopped it"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	raw := strings.TrimSpace(mustRead(t, path))
	if strings.Contains(raw, "ruleName") {
		t.Errorf("a rule-less line carries ruleName: %s", raw)
	}
}

// mustRead returns the file's text. Two tests here need the bytes rather than the
// parsed records, because what they check is what is on the wire.
func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
