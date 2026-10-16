package audit

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// writeTrail lays down a trail of n records with predictable usernames, using the real
// Writer rather than hand-written JSONL. A reader tested against a fixture the writer
// never produces is a reader that passes against a file format that does not exist.
func writeTrail(t *testing.T, dir string, n int) string {
	t.Helper()
	path := filepath.Join(dir, "plock.log")
	w := New(path)
	w.Now = now
	t.Cleanup(func() { w.Close() })
	for i := 0; i < n; i++ {
		if err := w.Write(Record{
			Kind:     "lock.asserted",
			Username: fmt.Sprintf("kid%02d", i),
			Detail:   fmt.Sprintf("event %d", i),
		}); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return path
}

func TestTailReturnsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	path := writeTrail(t, dir, 5)

	got, err := (&Reader{Path: path}).Tail(3)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(got.Records) != 3 {
		t.Fatalf("got %d records, want 3", len(got.Records))
	}
	// kid04 was written last, so it is the newest.
	want := []string{"kid04", "kid03", "kid02"}
	for i, u := range want {
		if got.Records[i].Username != u {
			t.Errorf("record %d is %q, want %q", i, got.Records[i].Username, u)
		}
	}
	if got.Skipped != 0 {
		t.Errorf("Skipped = %d on a clean trail; a clean trail must report nothing unreadable", got.Skipped)
	}
	if got.FilesRead != 1 {
		t.Errorf("FilesRead = %d, want 1", got.FilesRead)
	}
}

// Asking for more than exists is the ordinary case -- a fresh machine has three records
// and the dashboard asks for fifty. It must answer with three, not with an error that the
// UI renders as "audit unavailable".
func TestTailAskingForMoreThanExistsIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	path := writeTrail(t, dir, 2)

	got, err := (&Reader{Path: path}).Tail(50)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(got.Records) != 2 {
		t.Fatalf("got %d records, want the 2 that exist", len(got.Records))
	}
}

// The field round-trip. A reader that drops a field is worse than one that fails: the
// table renders, the columns are just quietly empty, and `sessionId` empty reads as
// "this was not about a session" rather than "the reader lost it".
func TestTailRoundTripsEveryField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plock.log")
	w := New(path)
	at := time.Unix(1700000123, 456).UTC()
	w.Now = func() time.Time { return at }
	if err := w.Write(Record{
		Kind:      "config.change",
		Username:  "kid",
		SessionID: "2",
		RuleID:    "schoolnight",
		RuleName:  "School week",
		Detail:    "changed",
		Actor:     "web:admin",
		Old:       json.RawMessage(`{"enabled":true}`),
		New:       json.RawMessage(`{"enabled":false}`),
	}); err != nil {
		t.Fatal(err)
	}
	w.Close()

	got, err := (&Reader{Path: path}).Tail(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 1 {
		t.Fatalf("got %d records", len(got.Records))
	}
	r := got.Records[0]

	// Every field of Record has to come back, and this loop is what makes that claim
	// instead of the list below. The list is the one that failed: it was written when
	// Record had nine fields, stayed green when a tenth was added, and stayed green while
	// decodeRecord dropped it -- because a field the fixture never sets cannot be observed
	// missing. Reflecting over the struct means the next field added to Record has to be
	// put in the fixture on purpose, and a field decodeRecord forgets to copy fails here
	// even though it compiles.
	rv := reflect.ValueOf(r)
	for i := 0; i < rv.NumField(); i++ {
		if rv.Field(i).IsZero() {
			t.Errorf("%s came back zero: either this fixture does not set it or decodeRecord "+
				"does not copy it, and a field dropped there is a value that reaches the disk "+
				"and never comes back", reflect.TypeOf(r).Field(i).Name)
		}
	}

	for _, c := range []struct {
		name, got, want string
	}{
		{"Kind", r.Kind, "config.change"},
		{"Username", r.Username, "kid"},
		{"SessionID", r.SessionID, "2"},
		{"RuleID", r.RuleID, "schoolnight"},
		{"RuleName", r.RuleName, "School week"},
		{"Detail", r.Detail, "changed"},
		{"Actor", r.Actor, "web:admin"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if !r.At.Equal(at) {
		t.Errorf("At = %v, want %v", r.At, at)
	}
	if string(r.Old) != `{"enabled":true}` {
		t.Errorf("Old = %s", r.Old)
	}
	if string(r.New) != `{"enabled":false}` {
		t.Errorf("New = %s", r.New)
	}
}

// A page that stops at the end of the current file would tell the operator nothing
// happened for as long as it takes to rotate -- up to DefaultMaxBytes of events. "Nothing
// happened" is the wrong thing for a lockout tool to imply.
func TestTailCrossesRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plock.log")

	// Three generations by hand, newest first: the live file, then .1, then .2.
	for gen, names := range [][]string{
		{"live-a", "live-b"},
		{"gen1-a", "gen1-b"},
		{"gen2-a", "gen2-b"},
	} {
		p := path
		if gen > 0 {
			p = fmt.Sprintf("%s.%d", path, gen)
		}
		var sb strings.Builder
		for _, n := range names {
			b, err := json.Marshal(line{
				At: now().Format(time.RFC3339Nano), Kind: "lock.asserted", Username: n,
			})
			if err != nil {
				t.Fatal(err)
			}
			sb.Write(b)
			sb.WriteByte('\n')
		}
		if err := os.WriteFile(p, []byte(sb.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := (&Reader{Path: path}).Tail(5)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	want := []string{"live-b", "live-a", "gen1-b", "gen1-a", "gen2-b"}
	if len(got.Records) != len(want) {
		t.Fatalf("got %d records %v, want %d", len(got.Records), usernames(got.Records), len(want))
	}
	for i, u := range want {
		if got.Records[i].Username != u {
			t.Errorf("record %d is %q, want %q (whole page: %v)",
				i, got.Records[i].Username, u, usernames(got.Records))
		}
	}
	if got.FilesRead != 3 {
		t.Errorf("FilesRead = %d, want 3", got.FilesRead)
	}
}

// Keep bounds how far back the reader looks. It must agree with the writer's retention or
// the reader promises generations the writer has already deleted.
func TestTailStopsAtTheConfiguredGenerationBound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plock.log")
	for gen := 0; gen < 4; gen++ {
		p := path
		if gen > 0 {
			p = fmt.Sprintf("%s.%d", path, gen)
		}
		body, err := json.Marshal(line{At: now().Format(time.RFC3339Nano), Kind: "lock.asserted", Username: fmt.Sprintf("g%d", gen)})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, append(body, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := (&Reader{Path: path, Keep: 2}).Tail(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 2 {
		t.Fatalf("Keep:2 read %v, want the two newest generations only", usernames(got.Records))
	}
	if got.Records[0].Username != "g0" || got.Records[1].Username != "g1" {
		t.Errorf("read %v, want g0 then g1", usernames(got.Records))
	}
}

// The writer can be killed between writing half a record and the newline. The correct
// answer is the rest of the page plus a count, not an empty page.
func TestATornFinalLineIsDroppedAndCounted(t *testing.T) {
	dir := t.TempDir()
	path := writeTrail(t, dir, 3)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	// Half a record: valid JSON with no newline, which is what a kill mid-append leaves.
	if _, err := f.WriteString(`{"at":"2025-01-01T00:00:00Z","kind":"lock","username":"torn"`); err != nil {
		t.Fatal(err)
	}
	f.Close()

	got, err := (&Reader{Path: path}).Tail(10)
	if err != nil {
		t.Fatalf("a torn tail must not fail the read: %v", err)
	}
	if len(got.Records) != 3 {
		t.Fatalf("got %v, want the 3 complete records", usernames(got.Records))
	}
	for _, r := range got.Records {
		if r.Username == "torn" {
			t.Error("the torn record was returned as if it were complete")
		}
	}
	if got.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1: the page must say it lost something", got.Skipped)
	}
}

// A line that is present but not a record -- corrupt, or written by a newer version with a
// different shape -- is counted, not silently absent.
func TestAnUnreadableRecordIsCountedNotHidden(t *testing.T) {
	dir := t.TempDir()
	path := writeTrail(t, dir, 2)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("this is not json at all\n")
	f.WriteString(`{"at":"not-a-timestamp","kind":"lock","username":"kid99"}` + "\n")
	f.Close()

	got, err := (&Reader{Path: path}).Tail(10)
	if err != nil {
		t.Fatalf("one bad line must not fail the read: %v", err)
	}
	if got.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2 (a non-JSON line and a line with an unreadable timestamp)", got.Skipped)
	}
	// A timestamp that cannot be parsed is a record whose time is unknown. Returning it
	// with a zero At would render as 1970 and read as "a very old event", which is a
	// fabrication.
	for _, r := range got.Records {
		if r.Username == "kid99" {
			t.Error("a record with an unparseable timestamp was returned")
		}
	}
}

// A record straddling the 64 KiB block boundary must still decode. This is the case a
// backwards reader gets wrong by dropping the left half, and it is invisible unless the
// file is bigger than one block.
func TestARecordStraddlingABlockBoundaryStillDecodes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plock.log")

	// Fill past one block with fixed-size records, then place a marker whose bytes land
	// across the boundary. Detail is padded so the marker's start is deterministic.
	var sb strings.Builder
	n := 0
	for sb.Len() < readBlockBytes+readBlockBytes/2 {
		rec, err := json.Marshal(line{
			At: now().Format(time.RFC3339Nano), Kind: "lock.asserted",
			Username: fmt.Sprintf("pad%04d", n),
			Detail:   strings.Repeat("x", 200),
		})
		if err != nil {
			t.Fatal(err)
		}
		sb.Write(rec)
		sb.WriteByte('\n')
		n++
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	// Ask for enough records to walk across at least one boundary.
	got, err := (&Reader{Path: path}).Tail(n)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != n {
		t.Fatalf("read %d of %d records; a shortfall means a straddling record was dropped (Skipped=%d)",
			len(got.Records), n, got.Skipped)
	}
	if got.Skipped != 0 {
		t.Errorf("Skipped = %d on a clean file; a straddling record was treated as unreadable", got.Skipped)
	}
	// Every padding record must be intact, not merely counted.
	for i, r := range got.Records {
		if len(r.Detail) != 200 {
			t.Fatalf("record %d has a %d-byte detail, want 200: it was reassembled wrongly", i, len(r.Detail))
		}
	}
}

func TestAMissingTrailIsEmptyAndNotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nothing-here.log")
	got, err := (&Reader{Path: path}).Tail(10)
	if err != nil {
		t.Fatalf("a machine that has not locked anyone out yet is not an error: %v", err)
	}
	if len(got.Records) != 0 || got.FilesRead != 0 {
		t.Errorf("got %d records from %d files, want 0 from 0", len(got.Records), got.FilesRead)
	}
}

// The zero value is what `&Reader{Path: p}` produces. If Keep resolved to 0 there, the
// reader would read nothing and report an empty trail -- the dangerous default this
// package has already had to remove twice.
func TestAReaderWithNoKeepReadsTheDefaultGenerations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plock.log")
	body, err := json.Marshal(line{At: now().Format(time.RFC3339Nano), Kind: "lock.asserted", Username: "only"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := (&Reader{Path: path}).Tail(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 1 {
		t.Fatalf("a zero-value Reader read %d records, want 1", len(got.Records))
	}
}

// The reader is a disclosure primitive: a root daemon asked to show the trail will read
// whatever a link in that directory points at, and the result goes to whoever holds the
// terminal. The writer's O_NOFOLLOW guard would be worth little if the reader skipped it.
func TestAReaderRefusesASymlinkedTrail(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "shadow")
	if err := os.WriteFile(secret, []byte("root:x:0:0::/root:/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "plock.log")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}

	got, err := (&Reader{Path: link}).Tail(5)
	if err == nil {
		t.Fatalf("a symlinked trail was read; got %v", usernames(got.Records))
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("refused for the wrong reason: %v", err)
	}
	if len(got.Records) != 0 {
		t.Error("records were returned alongside the refusal")
	}
}

// The same FIFO that parks the enforcement tick would park whatever rendered the audit
// view. A reader that blocks is a denial of service on the thing that explains outages.
func TestAReaderRefusesAFIFO(t *testing.T) {
	dir := t.TempDir()
	pipe := filepath.Join(dir, "plock.log")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Skipf("cannot create a FIFO here: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := (&Reader{Path: pipe}).Tail(5)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("a FIFO at the audit path was read without error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the reader blocked on a FIFO: the audit view would hang, not fail")
	}
}

// A non-regular file that survives the open is caught on the held descriptor. A check
// before the open leaves exactly the window the guard exists to close.
//
// /dev/null is the reachable case and it needs no root: it is a character device that
// opens read-only for anybody, and `Path: "/dev/null"` is a real misconfiguration rather
// than a thought experiment -- the writer's own directory guard already names /dev/null as
// the path that once made it chmod /dev. An earlier version of this test hardlinked
// /dev/null into the fixture directory and SKIPPED on every machine ("invalid cross-device
// link"), so the guard it claimed to cover had never been seen to fire.
func TestAReaderRefusesADeviceNodeAtTheAuditPath(t *testing.T) {
	const devNull = "/dev/null"
	fi, err := os.Stat(devNull)
	if err != nil || fi.Mode().IsRegular() {
		t.Skipf("%s is not usable as a non-regular path here", devNull)
	}

	got, err := (&Reader{Path: devNull}).Tail(5)
	if err == nil {
		t.Fatalf("a character device was accepted as a trail; got %d records", len(got.Records))
	}
	if !strings.Contains(err.Error(), "regular") {
		t.Errorf("refused for the wrong reason: %v", err)
	}
	if len(got.Records) != 0 {
		t.Error("records were returned alongside the refusal")
	}
}

func TestTailRejectsANonPositiveCount(t *testing.T) {
	dir := t.TempDir()
	path := writeTrail(t, dir, 1)
	for _, n := range []int{0, -1} {
		if _, err := (&Reader{Path: path}).Tail(n); err == nil {
			t.Errorf("Tail(%d) succeeded; a zero count would render as an empty audit list", n)
		}
	}
}

func TestAnEmptyPathIsRefused(t *testing.T) {
	if _, err := (&Reader{}).Tail(5); err == nil {
		t.Error("an empty Path was accepted; it would read the file named \"\"")
	}
	if _, _, err := (&Reader{}).Generations(); err == nil {
		t.Error("Generations accepted an empty Path")
	}
}

func TestNewestUserFindsTheMostRecentMention(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plock.log")
	w := New(path)
	w.Now = now
	for _, u := range []string{"anna", "bob", "anna", "cara"} {
		if err := w.Write(Record{Kind: "lock.asserted", Username: u, Detail: "d-" + u}); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	rec, found, err := (&Reader{Path: path}).NewestUser("anna", 100)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("anna is in the trail")
	}
	// Written third of four, so it is the newest anna.
	if rec.Detail != "d-anna" {
		t.Errorf("Detail = %q", rec.Detail)
	}
	if _, found, err := (&Reader{Path: path}).NewestUser("zoe", 100); found || err != nil {
		t.Errorf("zoe: found=%v err=%v, want false and no error", found, err)
	}
}

// An empty username must not match the records that have no username -- those are
// daemon-level events, and reporting one as "the last thing done to <nobody>" is the kind
// of answer that makes a diagnostic lie.
func TestNewestUserRefusesAnEmptyUsername(t *testing.T) {
	dir := t.TempDir()
	path := writeTrail(t, dir, 1)
	if _, found, err := (&Reader{Path: path}).NewestUser("", 10); err == nil {
		t.Errorf("an empty username matched (found=%v); it would match the records with no username", found)
	}
}

func TestGenerationsReportsWhatExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plock.log")
	sizes := map[int]int{0: 100, 1: 200, 2: 50}
	for gen, size := range sizes {
		p := path
		if gen > 0 {
			p = fmt.Sprintf("%s.%d", path, gen)
		}
		if err := os.WriteFile(p, make([]byte, size), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	gens, total, err := (&Reader{Path: path}).Generations()
	if err != nil {
		t.Fatal(err)
	}
	if len(gens) != 3 {
		t.Fatalf("got %d generations, want 3", len(gens))
	}
	if total != 350 {
		t.Errorf("total = %d, want 350", total)
	}
	for _, g := range gens {
		if !g.Regular {
			t.Errorf("generation %d reported as not regular", g.Index)
		}
		if want := sizes[g.Index]; int(g.Size) != want {
			t.Errorf("generation %d is %d bytes, want %d", g.Index, g.Size, want)
		}
	}
}

// A gap in the numbering is the end of the trail. The writer only ever creates generations
// upward, so a missing .1 with a present .2 means something moved the files, and reading
// past the gap would report a trail with a hole in it as if it were contiguous.
func TestGenerationsStopAtAGap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plock.log")
	for _, gen := range []int{0, 2} {
		p := path
		if gen > 0 {
			p = fmt.Sprintf("%s.%d", path, gen)
		}
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gens, _, err := (&Reader{Path: path}).Generations()
	if err != nil {
		t.Fatal(err)
	}
	if len(gens) != 1 {
		t.Errorf("got %d generations across a missing .1, want 1", len(gens))
	}
}

// The default log sink must be a real logger. A Reader whose Logf defaults to nil drops the
// one signal that the trail is only partly readable -- the same hole this package and
// internal/enforce already had to close in their constructors.
func TestAReaderWithNoLogfStillWarns(t *testing.T) {
	dir := t.TempDir()
	path := writeTrail(t, dir, 1)
	if err := os.WriteFile(path, []byte("this is not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf syncBuffer
	// The package logs through log.Printf, so redirecting the standard logger is the
	// whole test. A capture helper that swaps os.Stderr would miss it if the logger
	// cached its writer, which is why the logger itself is redirected.
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	got, err := (&Reader{Path: path}).Tail(5)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if got.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", got.Skipped)
	}
	text := buf.String()
	if !strings.Contains(text, "unreadable record") {
		t.Errorf("a zero-value Reader logged nothing about the record it skipped; got %q", text)
	}
}

// syncBuffer is a log sink that is safe to read while the code under test may still be
// writing to it.
type syncBuffer struct {
	mu   sync.Mutex
	body []byte
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.body = append(b.body, p...)
	return len(p), nil
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.body)
}

func usernames(rs []Record) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Username)
	}
	return out
}
