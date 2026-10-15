package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Reader reads back what Writer writes.
//
// It exists because a trail nothing can read is, from the operator's side, a trail that
// does not exist. `plock status` is meant to answer "what did the daemon last do to this
// account" and #10's audit view is the same question with a bigger page, and neither could
// be built: the writer was the whole package.
//
// The reader is a separate type rather than methods on Writer because the two have
// opposite failure instincts. A writer that cannot write must say so loudly on every event
// and keep the daemon running; a reader that cannot read must not invent a trail, and must
// distinguish "there are no records" from "I could not look" — a dashboard that shows an
// empty audit list because the file was unreadable is worse than one that shows an error,
// because an empty audit list is what a clean machine looks like.
type Reader struct {
	// Path is the current audit file, the same one Writer is given.
	Path string

	// Keep is how many rotated generations may be read, newest first: Path, then
	// Path.1 … Path.(Keep-1). Zero means DefaultKeep, and it is resolved by a method
	// rather than in a constructor so a struct literal cannot silently read nothing.
	Keep int

	// Logf receives the non-fatal findings: a record that would not decode, a
	// generation that could not be opened. Nil means log.Printf, because a reader
	// that drops its own warnings is how a half-readable trail looks clean.
	Logf func(format string, args ...any)
}

// Tail returns up to n of the most recent records, newest first.
//
// Three properties that are not obvious from the signature:
//
//  1. It reads backwards from the end of each generation in blocks, so asking for ten
//     records out of an 8 MiB file costs one block, not 8 MiB. The audit file is
//     deliberately large enough to fill a screen many times over.
//
//  2. It crosses rotation. A page of 50 that stops at the end of the current file would
//     show the operator nothing happened for as long as it takes to rotate — up to 8 MiB
//     of events — and "nothing happened" is exactly the wrong thing for a tool about
//     lockouts to imply.
//
//  3. A torn final line is dropped rather than failing the read. The writer can be killed
//     between writing half a record and the newline, and the correct answer is the other
//     499 records plus a count of what could not be read. Refusing to show anything because
//     the last line is half-written is a reader that turns one lost record into a lost
//     trail, and it is also the state a machine that crashed mid-lockout is left in — the
//     one moment somebody is most likely to ask.
func (r *Reader) Tail(n int) (Tail, error) {
	if n <= 0 {
		return Tail{}, fmt.Errorf("audit: Tail wants a positive count, got %d", n)
	}
	if r.Path == "" {
		return Tail{}, errors.New("audit: Reader.Path is empty")
	}

	out := Tail{}
	// Generation 0 is the live file; .1 is the one before it, and so on. The writer
	// renames upward, so the numbering is already newest-first.
	for gen := 0; gen < r.keep() && len(out.Records) < n; gen++ {
		path := r.Path
		if gen > 0 {
			path = fmt.Sprintf("%s.%d", r.Path, gen)
		}
		want := n - len(out.Records)
		got, err := r.tailFile(path, want, gen == 0)
		switch {
		case errors.Is(err, os.ErrNotExist):
			// No older generation. Generations appear only as the log grows, so
			// stopping here is the normal end of the trail, not a failure.
			return out, nil
		case err != nil:
			// A generation we cannot open is not a generation with no records in it.
			// Returning what we have with the error attached is the only honest shape:
			// the caller gets the records it can show and the reason the page is short.
			return out, fmt.Errorf("audit: reading generation %d (%s): %w", gen, path, err)
		}
		out.Records = append(out.Records, got.records...)
		out.Skipped += got.skipped
		out.FilesRead++
	}
	return out, nil
}

// Tail is a page of records plus what it cost to assemble.
type Tail struct {
	Records []Record

	// Skipped counts lines that were present but not readable as records. It is a
	// number rather than a dropped-on-the-floor log line because a caller rendering a
	// table has to say "49 shown, 2 unreadable" or it is reporting a clean trail.
	Skipped int

	// FilesRead is how many generations contributed, so a caller can tell a short
	// page that means "the trail is short" from one that means "rotation is close".
	FilesRead int
}

// tailResult is one generation's contribution.
type tailResult struct {
	records []Record
	skipped int
}

// tailFile returns up to n newest records from one file, newest first.
//
// isCurrent matters for one thing: only the live file can be caught mid-append by a
// concurrent writer, and only there does a final line without a newline mean "a record is
// being written right now". In a rotated generation the same shape means the daemon died
// with a half-written record. Both are dropped; the wording of the warning differs, and
// the difference is the difference between a live system and a crash.
func (r *Reader) tailFile(path string, n int, isCurrent bool) (tailResult, error) {
	f, err := openRead(path)
	if err != nil {
		return tailResult{}, err
	}
	defer f.Close()

	// The shape of the file actually opened, checked on the held descriptor. A check
	// before the open leaves the window the guard exists to close: the audit directory
	// is root-owned, but "root-owned" describes the directory, not what a link inside it
	// points at.
	fi, err := f.Stat()
	if err != nil {
		return tailResult{}, fmt.Errorf("stat: %w", err)
	}
	if !fi.Mode().IsRegular() {
		return tailResult{}, fmt.Errorf("%s is not a regular file (mode %v)", path, fi.Mode())
	}

	size := fi.Size()
	if size == 0 {
		return tailResult{}, nil
	}

	res := tailResult{}
	// buf holds the not-yet-split bytes to the left of the block we just read. A record
	// straddling a block boundary is only decodable once its left half arrives, which is
	// why the carry is prepended rather than dropped.
	var carry []byte
	// firstPartial is the fragment left at EOF on the first block: the torn tail, if any.
	var firstPartial []byte
	first := true
	// end is the offset the next block read stops at, moving backwards.
	end := size

	for end > 0 && len(res.records) < n {
		start := end - int64(readBlockBytes)
		if start < 0 {
			start = 0
		}
		blk := make([]byte, end-start)
		if _, err := f.ReadAt(blk, start); err != nil && !errors.Is(err, io.EOF) {
			return tailResult{}, fmt.Errorf("read at %d: %w", start, err)
		}

		// The bytes are blk + carry, and the first newline inside that is the start of
		// the first *complete* record this iteration can yield.
		data := blk
		if len(carry) > 0 {
			data = append(append([]byte(nil), blk...), carry...)
		}

		lines := splitNewestFirst(data, start == 0)
		if first {
			firstPartial = lines.tornTail
			first = false
		}
		carry = lines.carry

		for _, line := range lines.complete {
			if len(res.records) >= n {
				// Stopping mid-file is normal: the page is full. What is not normal
				// would be reporting the page as the whole trail, and FilesRead plus
				// the caller's own count keep that from being inferred.
				break
			}
			rec, err := decodeRecord(line)
			if err != nil {
				res.skipped++
				r.logf("audit: %s: skipping an unreadable record (%v): %.120q", path, err, line)
				continue
			}
			res.records = append(res.records, rec)
		}

		if start == 0 {
			break
		}
		end = start
	}

	// A file that does not end in a newline has a torn last record. splitNewestFirst
	// already dropped it as a partial on the first iteration; here it is counted, because
	// "49 shown" and "49 shown, 1 torn" are different claims about the same page.
	//
	// The two cases are the same shape and different news. On the live file a torn tail is
	// a writer caught mid-append, which is normal and self-healing. In a rotated
	// generation the file is closed and immutable, so a torn tail there means the daemon
	// died with a half-written record -- and the record it died writing is, more often than
	// not, the one the operator came to read.
	if len(firstPartial) > 0 {
		res.skipped++
		if isCurrent {
			r.logf("audit: %s ends mid-record (%d bytes); a write is in flight or was interrupted", path, len(firstPartial))
		} else {
			r.logf("audit: rotated generation %s ends mid-record (%d bytes); the daemon died writing it", path, len(firstPartial))
		}
	}
	return res, nil
}

// readBlockBytes is how much is read per backwards step. It is a multiple of a typical
// record (a few hundred bytes) so a page of 50 usually costs one read, and small enough
// that a tail of 5 out of 8 MiB still costs one read.
const readBlockBytes = 64 << 10

// lineSet is the result of cutting a block at newlines.
type lineSet struct {
	// complete are whole lines, newest first.
	complete [][]byte
	// carry is the fragment at the LEFT of the block with no newline before it inside
	// this block: the right half of a record whose left half is still unread. It is
	// prepended to the next block read, not dropped.
	carry []byte
	// tornTail is the fragment at the RIGHT of the block with no newline after it. It is
	// never a record and never carried; it exists so the caller can count what was lost.
	tornTail []byte
}

// splitNewestFirst cuts data into whole lines newest-first.
//
// atStart says whether data reaches byte 0 of the file. It is what tells a straddling
// record apart from the file's first record: away from byte 0 the leftmost fragment has a
// left half somewhere still unread, so it must be carried; at byte 0 there is nothing
// further left to arrive, so it is a record in its own right.
//
// The terminator is a newline, not a record separator. That distinction is the whole
// function: "a\nb\n" is two records, and "a\nb" is one record and a torn one. Treating the
// newline as a separator instead drops the first record of every file (the loop runs out
// before emitting it) and hands back the torn tail as if it were complete -- both of which
// this function did before this comment existed, and neither of which fails loudly.
func splitNewestFirst(data []byte, atStart bool) lineSet {
	var out lineSet
	rest := data

	// A trailing segment with nothing after it was not finished being written.
	if len(rest) > 0 && rest[len(rest)-1] != '\n' {
		i := bytes.LastIndexByte(rest, '\n')
		if i < 0 {
			// Not one newline in the block: everything here is one unfinished record,
			// and there is no complete line to emit.
			out.tornTail = rest
			return out
		}
		out.tornTail = rest[i+1:]
		// Keep the newline: it is the terminator of the line before the torn fragment,
		// and dropping it here would make that line look like the leftmost fragment.
		rest = rest[:i+1]
	}

	// The invariant for the loop below: `rest` is empty or ends with a newline. Every
	// line this loop emits is `rest` minus its own terminator, and the next iteration's
	// `rest` is everything up to and including the newline that terminated the line
	// before it. Getting this wrong by one byte does not crash anything -- it eats the
	// last character of every record and then reports them as unreadable.
	for len(rest) > 0 {
		body := rest[:len(rest)-1] // drop this line's terminator
		i := bytes.LastIndexByte(body, '\n')
		if i < 0 {
			// body is the leftmost thing in the block. Whose it is depends on whether
			// the block reaches byte 0.
			if atStart {
				out.complete = append(out.complete, body)
			} else {
				// Carried WITH its terminator. Dropping the newline here is the subtle
				// bug: the next block's data would then be the record minus its own
				// terminator, which reads as a torn tail at EOF and gets discarded --
				// so exactly one record, the one straddling each boundary, vanishes with
				// nothing counted as skipped.
				out.carry = rest
			}
			return out
		}
		out.complete = append(out.complete, body[i+1:])
		rest = rest[:i+1]
	}
	return out
}

// decodeRecord turns one line into a Record.
func decodeRecord(raw []byte) (Record, error) {
	var wire line
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Record{}, err
	}
	at, err := time.Parse(time.RFC3339Nano, wire.At)
	if err != nil {
		return Record{}, fmt.Errorf("unreadable timestamp %q: %w", wire.At, err)
	}
	return Record{
		At:        at,
		Kind:      wire.Kind,
		Username:  wire.Username,
		SessionID: wire.SessionID,
		RuleID:    wire.RuleID,
		Detail:    wire.Detail,
		Actor:     wire.Actor,
		Old:       wire.Old,
		New:       wire.New,
	}, nil
}

// keep resolves the generation bound. Zero means DefaultKeep rather than "read none":
// the zero value of a Reader is what a caller gets by writing `&Reader{Path: p}`, and a
// reader that reads nothing in that state is the dangerous default this package has
// already had to remove twice.
func (r *Reader) keep() int {
	if r.Keep <= 0 {
		return DefaultKeep
	}
	return r.Keep
}

func (r *Reader) logf(format string, args ...any) {
	if r.Logf != nil {
		r.Logf(format, args...)
		return
	}
	// The default is the real logger, not silence -- the same rule Writer.logf records and
	// for the same reason: a reader that drops its own warnings makes a half-readable
	// trail look clean.
	log.Printf(format, args...)
}

// NewestUser returns the most recent record naming user, searching back through the
// trail. It is the question `plock status` was written to ask and could not: "what did the
// daemon last do to this account".
//
// It stops at the first hit rather than materialising a page, and the bound is a count of
// records examined rather than a duration, because a machine under a burst of
// platform.error events can write a generation in minutes and a time bound would make the
// answer depend on how healthy the desktop is.
func (r *Reader) NewestUser(user string, scan int) (Record, bool, error) {
	if user == "" {
		return Record{}, false, errors.New("audit: NewestUser needs a username")
	}
	if scan <= 0 {
		scan = defaultScanRecords
	}
	// Records are newest first, so a page is enough to search in order. The page grows
	// with the scan bound rather than being looped, because paging a backwards reader
	// means re-reading the bytes it already skipped.
	tail, err := r.Tail(scan)
	for _, rec := range tail.Records {
		if rec.Username == user {
			return rec, true, err
		}
	}
	return Record{}, false, err
}

// defaultScanRecords bounds a search when the caller has no opinion. Ten thousand records
// is several generations of a busy machine and still one backwards pass of a few hundred
// blocks.
const defaultScanRecords = 10_000

// Generations reports the audit files that exist, newest first, and the total bytes they
// hold. It is for `plock doctor` and the web UI's "how much disk is the trail using",
// which cannot be answered from Writer.Stats because that only knows the current run.
func (r *Reader) Generations() ([]Generation, int64, error) {
	if r.Path == "" {
		return nil, 0, errors.New("audit: Reader.Path is empty")
	}
	var out []Generation
	var total int64
	for gen := 0; gen < r.keep(); gen++ {
		path := r.Path
		if gen > 0 {
			path = fmt.Sprintf("%s.%d", r.Path, gen)
		}
		// Lstat, not Stat: a link at a generation path is the finding, not the size of
		// whatever it points at.
		fi, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return out, total, fmt.Errorf("audit: stat %s: %w", path, err)
		}
		g := Generation{Path: path, Index: gen, Size: fi.Size(), Regular: fi.Mode().IsRegular()}
		if !g.Regular {
			// Reported, not sized: a non-regular generation is a tamper finding and
			// its size is somebody else's file.
			g.Size = 0
		}
		out = append(out, g)
		total += g.Size
	}
	return out, total, nil
}

// Generation is one audit file on disk.
type Generation struct {
	Path  string
	Index int // 0 is the live file
	Size  int64
	// Regular is false for anything that is not a plain file. The reader refuses those,
	// and the caller has to be able to say why the number is missing.
	Regular bool
}

// Dir returns the directory holding the trail. Exposed so a diagnostic can report the
// directory's mode without each caller re-deriving filepath.Dir and getting the policy
// subtly different.
func (r *Reader) Dir() string { return filepath.Dir(r.Path) }
