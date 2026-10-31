package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
)

// The survivors this file closes all share one shape: a test that asserted the outcome
// without asserting the channel it arrived on. A warning that goes to the wrong sink, a
// generation reported as normal, and a refusal that names the wrong thing all produce the
// right verdict for the wrong reason.

// A generation that is not a regular file is a tamper finding, and its size is somebody
// else's file. Reporting it as a normal generation with that size both hides the finding and
// inflates "how much disk is the trail using".
//
// Two shapes, because the whole argument about this fixture is about the shape's size, and the
// two shapes answer differently. A FIFO, a socket and a character device all report
// st_size == 0, so on those a mutant that adds the size to the trail's total is
// indistinguishable from the guard; only a shape with a real size makes "the size of somebody
// else's file is being counted" observable. A symlink's st_size is the length of its target
// string, so it is non-zero on every filesystem. A directory's is whatever the filesystem
// happens to say -- 4096 on ext4, 40 on tmpfs, 0 on btrfs -- which is precisely how A49 walked
// free on the common Fedora/openSUSE/Arch layout (issue #185). So the symlink carries the
// guard everywhere, and the directory stays because it is the shape the guard was written
// against, with its premise checked rather than assumed.
func TestGenerationsReportsANonRegularGenerationAsTheFinding(t *testing.T) {
	// Long on purpose: the link's st_size IS this string's length, so a short target would
	// make the assertion below depend on how much the live trail happens to hold.
	const target = "a-generation-that-was-replaced-and-its-name-is-the-size-we-are-counting"
	for _, shape := range []struct {
		name  string
		place func(t *testing.T, gen1 string) string
	}{
		{"a symlink", func(t *testing.T, gen1 string) string {
			// Generations() Lstats rather than Stats, so the link is the finding and its
			// target is irrelevant -- it does not have to exist.
			if err := os.Symlink(target, gen1); err != nil {
				t.Fatalf("cannot place a symlink at a generation path here: %v", err)
			}
			return fmt.Sprintf("a symlink to %q", target)
		}},
		{"a directory", func(t *testing.T, gen1 string) string {
			if err := os.Mkdir(gen1, 0o700); err != nil {
				t.Fatalf("cannot place a directory at a generation path here: %v", err)
			}
			return "an empty directory"
		}},
	} {
		t.Run(shape.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeTrail(t, dir, 1)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// Generation .1 replaced by something that is not a plain file.
			gen1 := path + ".1"
			if err := os.Remove(gen1); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			what := shape.place(t, gen1)

			// The premise, checked rather than assumed: everything below measures nothing if
			// the shape reports no size, so say that out loud instead of passing for the
			// wrong reason -- the exact failure this test had on btrfs.
			//
			// For the symlink arm this cannot fire on Linux: st_size is the target's length
			// and an empty target is refused (measured: os.Symlink("", …) → ENOENT, and the
			// shortest possible target reports 1). It is kept for both arms on purpose. A
			// premise check written once, around the fixture seam, is what stops the next
			// shape being added without it; one written only where it can currently fail is
			// how the directory got here.
			fi, err := os.Lstat(gen1)
			if err != nil {
				t.Fatal(err)
			}
			if fi.Size() == 0 {
				t.Skipf("%s reports size 0 on this filesystem, so nothing below would be measured", what)
			}

			gens, total, err := (&Reader{Path: path}).Generations()
			if err != nil {
				t.Fatalf("Generations: %v", err)
			}
			if len(gens) != 2 {
				t.Fatalf("got %d generations, want 2: %v", len(gens), gens)
			}
			if !gens[0].Regular {
				t.Errorf("generation 0 is the live file and was reported as non-regular")
			}
			if gens[1].Regular {
				t.Errorf("generation 1 is %s and was reported as a normal log file", what)
			}
			// The size of somebody else's file is not the trail's bytes. Counting it would
			// make the answer to "how big is my audit log" depend on what the path was
			// replaced with.
			if gens[1].Size != 0 {
				t.Errorf("a non-regular generation contributed %d bytes to the trail's total", gens[1].Size)
			}
			if total != int64(len(body)) {
				t.Errorf("total = %d, want %d (the live file only)", total, len(body))
			}
		})
	}
}

// The configured sink must actually receive the warning. The zero-value fallback has its own
// test; a caller that installs a sink to capture the log has to be able to trust that
// installing it does not SILENCE the thing it was installed to see.
func TestAConfiguredSinkReceivesTheSkippedRecordWarning(t *testing.T) {
	dir := t.TempDir()
	path := writeTrail(t, dir, 1)
	if err := os.WriteFile(path, []byte("this is not a record\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var seen []string
	r := &Reader{Path: path, Logf: func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, fmt.Sprintf(format, args...))
	}}

	got, err := r.Tail(5)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if got.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", got.Skipped)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) == 0 {
		t.Fatal("the configured sink received nothing, so installing a sink silences the reader")
	}
	if !strings.Contains(seen[0], "unreadable record") {
		t.Errorf("the sink got %q, want it to name what was skipped", seen[0])
	}
}

// A FIFO at the audit path has to be refused, and the refusal has to say what it refused.
//
// The guard that fires is the shape check on the held descriptor, NOT a translation of
// ENXIO: O_NONBLOCK makes the open succeed, so there is no errno to translate. An earlier
// version of this test asserted the word "FIFO" and passed, because the message it matched
// came from a different branch than the one it claimed to prove.
func TestAFIFORefusalNamesTheShape(t *testing.T) {
	dir := t.TempDir()
	pipe := filepath.Join(dir, "plock.log")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Skipf("cannot create a FIFO here: %v", err)
	}

	_, err := (&Reader{Path: pipe}).Tail(5)
	if err == nil {
		t.Fatal("a FIFO at the audit path was read without error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "not a regular file") {
		t.Errorf("the refusal does not name the shape: %q", msg)
	}
	// The mode is in the message so the operator can see what was found without a second
	// command: `prw-------` is the tell that something is listening on the path.
	if !strings.Contains(msg, "p") || !strings.Contains(msg, pipe) {
		t.Errorf("the refusal does not name the path that was refused: %q", msg)
	}
	// And it must not be the bare syscall string.
	if strings.Contains(msg, "no such device or address") {
		t.Errorf("the raw errno reached the operator untranslated: %q", msg)
	}
}

// The symlink refusal has the same requirement, and it is the one that matters most: the
// reader is a disclosure primitive, so the operator needs to know a link was refused rather
// than that the trail is missing.
func TestASymlinkRefusalNamesTheLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "elsewhere.log")
	if err := os.WriteFile(target, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "plock.log")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	_, err := (&Reader{Path: link}).Tail(5)
	if err == nil {
		t.Fatal("a symlinked trail was read without error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "symlink") {
		t.Errorf("the refusal does not name the link: %q", msg)
	}
	if !strings.Contains(msg, link) {
		t.Errorf("the refusal does not name the path that was refused: %q", msg)
	}
}

// The writer's own bound, from the other side: a file it cannot measure.
//
// Treating an unmeasurable file as empty moves the rotation bound in the unsafe direction,
// and it is silent unless the operator is told. The branch is unreachable without a seam --
// see statFile -- so the test injects the refusal rather than leaving the guard unexercised.
//
// It runs against two constructions on purpose. The configured one is what a caller who
// thought about rotation writes; the other is what the daemon actually builds -- serve.go
// constructs &audit.Writer{Path: …, Logf: …} and leaves MaxBytes at zero, which means
// "the default". A test that only ever sets MaxBytes cannot tell `w.size = w.maxBytes()`
// from `w.size = w.MaxBytes`, and the second one stores 0 on every real deployment.
func TestAnUnmeasurableTrailIsTreatedAsFullNotEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plock.log")

	var mu sync.Mutex
	var seen []string
	restore := statFile
	statFile = func(f *os.File) (os.FileInfo, error) {
		return nil, fmt.Errorf("injected: cannot measure")
	}
	t.Cleanup(func() { statFile = restore })

	for _, tc := range []struct {
		name string
		// build makes the writer the way its caller would, without knowing which
		// bound the unmeasurable arm is going to read.
		build func(logf func(string, ...any)) *Writer
		want  int64
	}{
		{
			name: "the bound the operator configured",
			build: func(logf func(string, ...any)) *Writer {
				w := New(path)
				w.MaxBytes = 400
				w.Logf = logf
				return w
			},
			want: 400,
		},
		{
			// The regression. This is the only Writer the daemon builds, and against
			// it the field reads zero -- the permissive end, "measured as empty".
			name: "the bound the daemon actually builds",
			build: func(logf func(string, ...any)) *Writer {
				return &Writer{Path: path, Logf: logf}
			},
			want: DefaultMaxBytes,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mu.Lock()
			seen = nil // each subtest judges its own warning, not the previous one's
			mu.Unlock()
			w := tc.build(func(format string, args ...any) {
				mu.Lock()
				defer mu.Unlock()
				seen = append(seen, fmt.Sprintf(format, args...))
			})
			w.Now = now
			t.Cleanup(func() { w.Close() })

			if err := w.Write(Record{Kind: "lock.asserted", Username: "kid"}); err != nil {
				t.Fatal(err)
			}
			// The conservative end: the bound is already reached, so the NEXT write
			// rotates rather than appending past what the operator configured. Zero
			// here would mean "start over", and a trail that cannot be measured would
			// then never rotate at all.
			if w.size != tc.want {
				t.Errorf("an unmeasurable trail resolved to %d bytes against a %d-byte bound; it must resolve to the bound", w.size, tc.want)
			}
			mu.Lock()
			defer mu.Unlock()
			if len(seen) == 0 {
				t.Fatal("the writer guessed at the size without saying so")
			}
			if !strings.Contains(seen[0], "cannot stat") {
				t.Errorf("the warning does not say measurement failed: %q", seen[0])
			}
		})
	}
}

// noteFailure must not re-zero the running size, and the branch is inside the `f != nil`
// guard: a failure that still holds a descriptor is the one that drops it. An earlier version
// of this test called noteFailure with a nil file and therefore proved nothing about the
// line it claimed to cover.
func TestFailuresInTheMiddleOfAFileDoNotMoveTheBound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plock.log")
	w := New(path)
	w.Now = now
	// The bound is deliberately far above what this test writes. An earlier version used
	// 400 bytes and the third record tripped rotation, which resets the size on its own --
	// so the assertion compared 0 against 0 and could not have failed for any mutation.
	w.MaxBytes = 100_000
	t.Cleanup(func() { w.Close() })

	for i := 0; i < 3; i++ {
		if err := w.Write(Record{Kind: "lock.asserted", Username: "kid", Detail: strings.Repeat("y", 60)}); err != nil {
			t.Fatal(err)
		}
	}
	before := w.size
	held := w.f
	if held == nil {
		t.Fatal("the writer holds no descriptor, so the branch under test cannot be reached")
	}
	if before == 0 {
		t.Fatal("the writer has recorded three events and counts no bytes; the assertion below would be vacuous")
	}

	// A failure that drops a live descriptor. If it also resets the size, the bound silently
	// restarts from zero and the file never rotates when it should.
	w.noteFailure(held, "write", fmt.Errorf("injected"))
	if w.size != before {
		t.Errorf("noteFailure moved the running size %d -> %d; the bound must survive a failure", before, w.size)
	}

	// The next write reopens and re-measures from the file, which is the mechanism the
	// comment at the guard records. It has to land on the file's real length, not on
	// whatever the failure left in the counter.
	for i := 0; i < 2; i++ {
		if err := w.Write(Record{Kind: "lock.asserted", Username: "kid", Detail: strings.Repeat("y", 60)}); err != nil {
			t.Fatal(err)
		}
	}
	if w.size <= before {
		t.Errorf("the running size did not advance past the injected failure (%d -> %d)", before, w.size)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// The counter and the file have to agree, because the counter is what decides rotation.
	// A counter that lost the bytes already in the file is the bug in its observable form.
	if w.size < fi.Size() {
		t.Errorf("the writer counts %d bytes against %d actually on disk", w.size, fi.Size())
	}
}
