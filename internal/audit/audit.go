// Package audit writes the append-only JSONL trail required by docs/PLAN.md §10:
// every mutation of the product's state, who caused it, when, and what changed.
//
// The trail exists for the parent, not the machine. Its whole value is that a
// line saying "Casey was locked out at 23:41" is present when the parent asks,
// and absent lines are indistinguishable from a night nothing happened. So the
// governing rule here is the one in the ticket this package closes: a write that
// fails must never suppress enforcement, and must never be absorbed quietly
// either. Enforcement continues and the failure is counted, logged on every
// occurrence, and reported through Stats so `plock status` can say the trail is
// incomplete.
//
// The seam stays in internal/enforce, which owns the event vocabulary: this
// package exposes Write(Record) and nothing about engine events, so there is no
// second copy of that type to drift out of step with the first. Wiring the two
// together is cmd/plock's job, because it is the one package that may import both.
package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultPath and the defaults below are the values PLAN §10 implies but does not
// name. Rotation size and retention are left unspecified there, so they are
// stated here rather than discovered at runtime by whoever first fills a disk.
const (
	DefaultPath = "/var/log/plock/audit.jsonl"

	// DefaultMaxBytes is per file, not per day: a burst of platform.error events on
	// a machine whose desktop has died is the shape that grows the log, and it
	// grows at the tick rate, not the clock rate. 8 MiB is roughly 60k lines, which
	// is hours of that at 30-second ticks — long enough to see when it started.
	DefaultMaxBytes = 8 << 20

	// DefaultKeep is how many rotated files survive. Five generations at 8 MiB is
	// 40 MiB worst case, which is a rounding error on the disk it sits on and long
	// enough that "when did this start" is answerable after a weekend.
	DefaultKeep = 5

	// dirMode is 0700, not 0755: the directory holds the trail, and a trail a
	// controlled user can list is a trail they can time their attempts against.
	dirMode os.FileMode = 0o700
	// fileMode is PLAN §10 verbatim.
	fileMode os.FileMode = 0o600
)

// Record is one line of the trail.
//
// The field set is the engine's Event plus the two things a config mutation
// needs and a tick does not: who acted, and what the value was before. They are
// pointers rather than strings because "the parent cleared a field" and "there
// was no old value" are different claims, and an empty string cannot tell them
// apart.
type Record struct {
	At        time.Time `json:"at"`
	Kind      string    `json:"kind"`
	Username  string    `json:"username,omitempty"`
	SessionID string    `json:"sessionId,omitempty"`
	RuleID    string    `json:"ruleId,omitempty"`
	// RuleName is the schedule window's human name ("School week"). RuleID survives a
	// rename and RuleName is what an operator recognises, so a trail carrying only the
	// id answers "which rule" to nobody. The comment above says this field set is the
	// engine's Event plus two -- and RuleName is in Event, so without this field,
	// wiring the engine to the writer silently discards the readable half.
	RuleName string `json:"ruleName,omitempty"`
	Detail   string `json:"detail,omitempty"`

	// Actor is who caused a mutation, as opposed to who it was about. An engine
	// event has no actor: nobody did anything, a window opened.
	Actor string `json:"actor,omitempty"`
	// Old and New carry a config change. They are raw JSON so a rule can be shown
	// as the object it was rather than as a type name.
	Old json.RawMessage `json:"old,omitempty"`
	New json.RawMessage `json:"new,omitempty"`
}

// line is the shape actually serialised. Splitting it from Record keeps the
// monotonic clock and the JSON nil-vs-empty distinction out of Record's API.
type line struct {
	At        string          `json:"at"`
	Kind      string          `json:"kind"`
	Username  string          `json:"username,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	RuleID    string          `json:"ruleId,omitempty"`
	RuleName  string          `json:"ruleName,omitempty"`
	Detail    string          `json:"detail,omitempty"`
	Actor     string          `json:"actor,omitempty"`
	Old       json.RawMessage `json:"old,omitempty"`
	New       json.RawMessage `json:"new,omitempty"`
}

// Writer appends Records to one JSONL file, rotating it by size.
//
// It is safe for concurrent use: the engine emits from the tick and #10's admin
// server emits from request handlers, and both must land in one stream, because
// the story a parent reads is "the parent set the rule, then the child was
// locked" and two streams cannot tell it.
type Writer struct {
	// Path is the audit file. Its directory is created if absent.
	Path string
	// MaxBytes is the rotation threshold. Zero means DefaultMaxBytes.
	MaxBytes int64
	// Keep is how many rotated files to retain. Zero means DefaultKeep.
	//
	// There is deliberately no "keep nothing" setting reachable by leaving this
	// field alone. A zero value is the most common way a Writer gets built — by
	// someone who wrote &Writer{Path: p} and meant the defaults — and the reading
	// that makes that mean "discard the previous generation on every rotation"
	// loses the trail quietly, which is the failure this package exists to prevent.
	// Somebody who genuinely wants one file says Keep: 1.
	Keep int
	// Logf receives every failure, and every escalation. It defaults to
	// log.Printf; a Writer built by the daemon with no hook set must not lose the
	// one signal that its trail is broken.
	Logf func(format string, args ...any)
	// Now defaults to time.Now. A field so a test can pin the timestamp on a line
	// rather than assert on a range.
	Now func() time.Time

	mu sync.Mutex
	// f is held open between writes. Reopening per event would be safer against a
	// swapped path and slower by an order of magnitude at the tick rate; the
	// compromise is to re-verify the descriptor on every write (see verify) so a
	// swap is caught on the next event rather than never.
	f    *os.File
	size int64

	// failures counts write errors since the last success, and dropped counts
	// every event that never reached the file. Both are what Stats reports and
	// what makes "the trail is incomplete" a fact an operator can see rather than
	// infer from a missing line.
	failures int
	dropped  int64
	// escalated says the loud wording has already been emitted for this run of
	// failures. It is reset by a success, so a trail that breaks again is loud
	// again; what it prevents is N identical lines for N events with nothing to
	// say the run is continuing.
	escalated bool
}

// ErrNotRootOwned reports an audit file owned by an account other than the one
// writing it.
//
// PLAN §10 requires it, and the reason it is worth refusing rather than fixing is
// that ownership is the only thing standing between a controlled user and a trail
// they curated. Chowning it to the writer would silently accept a file that had
// been theirs, including one with lines already in it.
var ErrNotRootOwned = errors.New("audit file is not owned by the account writing it")

// ErrNotRegular reports an audit path that is not a regular file.
var ErrNotRegular = errors.New("not a regular file")

// errOwnershipUnsupported reports a platform that cannot say who owns a file. It
// is an error rather than a nil so the guard fails closed: an audit writer that
// cannot check ownership does not get to write a trail that claims to be root's.
var errOwnershipUnsupported = errors.New("cannot determine file ownership on this platform")

// New returns a Writer on the default path with the default policy. The file is
// not opened until the first write, so constructing one never fails and never
// creates /var/log/plock on a machine that is only running `plock status`.
func New(path string) *Writer {
	return &Writer{Path: path, MaxBytes: DefaultMaxBytes, Keep: DefaultKeep}
}

// Stats reports how many events were lost and how many consecutive writes have
// failed. It is what `plock status` reads to say the trail is incomplete.
func (w *Writer) Stats() (dropped int64, consecutiveFailures int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dropped, w.failures
}

// Write appends one Record.
//
// The error it returns is information, not a decision: the caller is the
// enforcement loop, and a lockout that has begun does not stop because the log
// is full. Every caller should pass the error along — the engine files it as a
// degraded report — and none should let it change what happens to the child.
//
// The mutex is held across the open and the append, so a filesystem that stalls
// stalls every writer, including the admin server's. That is a deliberate
// trade: releasing the lock between opening and writing would let two events
// interleave across a rotation, and a torn line is as unreadable as a lost one.
// What makes the stall bounded is O_NONBLOCK — without it, a FIFO at the audit
// path parks the open forever by design, which is not a slow filesystem but a
// child turning enforcement off. The mutant that removes that flag is caught by
// a test whose whole job is that the call returns within two seconds.
func (w *Writer) Write(r Record) error {
	blob, err := w.marshal(r)
	if err != nil {
		// A record we cannot serialise is a record that will never be written,
		// whatever the filesystem does, so it counts as dropped for the same
		// reason: the line is missing and a parent reading the trail cannot know.
		w.noteFailure(nil, "encode", err)
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.open(); err != nil {
		w.noteFailure(nil, "open", err)
		return err
	}
	if err := w.verify(); err != nil {
		w.noteFailure(w.f, "verify", err)
		return err
	}

	n, err := w.f.Write(blob)
	w.size += int64(n)
	if err != nil {
		// A short write is the same event as an error, and the size must not claim
		// bytes that are not there: it would push the rotation threshold out and
		// let the file grow past the bound that justifies the retention.
		w.noteFailure(w.f, "write", err)
		return err
	}

	if w.failures != 0 {
		// The trail is whole again from here. Saying so is what turns the earlier
		// escalation from an open question into a closed one.
		w.logf("audit: trail resumed at %s after %d dropped event(s)", w.Path, w.dropped)
		w.failures = 0
		w.escalated = false
	}

	if w.maxBytes() > 0 && w.size >= w.maxBytes() {
		if err := w.rotate(); err != nil {
			// The event is already on disk, so this is not a dropped event and
			// must not be counted as one. It is a file that will keep growing,
			// which is a different complaint and gets its own line.
			w.logf("audit: could not rotate %s: %v (the log will keep growing past its size bound)", w.Path, err)
		}
	}
	return nil
}

// Close releases the held descriptor.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

func (w *Writer) marshal(r Record) ([]byte, error) {
	at := r.At
	if at.IsZero() {
		at = w.now()
	}
	l := line{
		At: at.Format(time.RFC3339Nano), Kind: r.Kind, Username: r.Username,
		SessionID: r.SessionID, RuleID: r.RuleID, RuleName: r.RuleName,
		Detail: r.Detail, Actor: r.Actor,
		Old: r.Old, New: r.New,
	}
	blob, err := json.Marshal(l)
	if err != nil {
		return nil, err
	}
	return append(blob, '\n'), nil
}

// Open forces the lazy open to happen now.
//
// Write opens on demand, which is right for a caller that just wants to append. A
// daemon starting up is not that caller: it wants a bad audit path to be a startup
// failure with a message, not the first dropped event three hours later. The trail
// being unwritable is discovered either way -- Open only decides whether it is
// discovered by the operator who can act on it or by whoever reads the gap next
// month.
func (w *Writer) Open() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.open()
}

// open gets a descriptor on the audit file, creating the directory and the file
// with their final modes.
//
// The directory is created before the file because the file's mode is set by the
// open itself: there is no window in which the trail exists and is world-readable,
// which a create-then-chmod would open on a filesystem where the umask is not what
// we assumed.
func (w *Writer) open() error {
	if w.f != nil {
		return nil
	}
	dir := filepath.Dir(w.Path)
	if err := w.prepareDir(dir); err != nil {
		return err
	}

	f, err := openAppend(w.Path, fileMode)
	if err != nil {
		return err
	}

	// The file is judged before it is changed.
	//
	// The mode is set on the descriptor, which is the only safe place to set it
	// (see tightenMode), and that made it the one thing this function did before it
	// knew what it had opened. The order was: open, fchmod 0600, and only in the
	// caller's verify() ask whether the thing behind the descriptor is a file at
	// all. So every refusal at open was also a write: point settings.auditPath at
	// /dev/null from a root unit and the refusal arrives a microsecond after
	// /dev/null has become crw-------, which is not a broken daemon but a machine
	// where nothing can open /dev/null any more — including the next process
	// anything tries to run. Point it at another account's regular file and the
	// daemon has chmodded a file it just declined to touch: the same arbitrary-chmod
	// primitive the descriptor was chosen to avoid, reached through the descriptor
	// instead of through the path.
	//
	// internal/config already does this in the right order — checkRegular on the
	// held descriptor, then tightenMode — and this package is the one holding the
	// root privileges, so it is the one that cannot afford the other order.
	//
	// Both verdicts verify() makes about the *identity* of the file are made here
	// now, because both are made about something the chmod would invalidate: the
	// shape, and who owns it. The rest of verify() stays where it is — the mode and
	// the name have to be re-checked on every write regardless, and a file that was
	// legitimately accepted once has already been legitimately tightened.
	fi, statErr := statFile(f)
	if statErr != nil {
		// Nothing is chmodded when the descriptor could not be identified: "I cannot
		// tell what this is" is not a licence to change it.
		//
		// What happens next depends on why the stat failed, and the two cases do not look
		// alike. What is pinned and what is only observed, precisely — because a block that
		// says "nothing here is tested" invites someone to delete the assertion that is
		// quietly holding two of its three claims up:
		//
		//   The same-write rotation and the post-rotation re-arm are both pinned, by the
		//   assertion in TestAnUnmeasurableTrailIsTreatedAsFullNotEmpty. Deferring the
		//   rotation reads "resolved to 470 bytes against a 400-byte bound"; not re-arming
		//   after rotate() zeroes the counter reads "resolved to 0 bytes". Both measured.
		//
		//   What has no pin is writes two and later: every shipped test that holds the seam
		//   failing writes exactly one record. The multi-write behaviour was measured in
		//   #186 by a probe that was then deleted.
		//
		//   The genuine-fstat case is not reachable from a test at all — the seam below is
		//   the only injection point — so everything said about it is observation.
		//
		//   Through the seam below — what the tests exercise — the bound holds and the
		//   trail shards one record per generation: rotate() ends in open(), so this arm
		//   runs again after each rotation and re-arms. Measured at the head that wrote
		//   this: three writes against a 400-byte bound leave plock.log empty and
		//   plock.log.1, .2 and .3 holding one record each, and print the line below on
		//   every write. Bounded for disk, and it never recovers on its own.
		//
		//   "Bounded" is not "kept", and for this file the difference is the point. rotate()
		//   deletes past Keep, so the sharding is bounded by the retention: eight writes with
		//   the default Keep of 5 leave the five most recent records on disk and the other
		//   three nowhere (measured at this head: the count and the byte total both plateau
		//   at five after the fifth write). Seam-only, so not the daemon's failure mode — but
		//   a trail whose job is that a parent can read what happened should be described
		//   with that clause next to the reassuring word, not without it.
		//
		//   The rotation lands on the write that prints this line, not on the one after
		//   it: Write() opens, appends, and compares w.size against the bound in the same
		//   call, so the record that trips the arm is the record that renames the file.
		//   That is why the sentence below promises no rotation at all — an earlier draft
		//   said "rotating on the next write", which was true of nothing: late by one
		//   write on the seam path, and never on the genuine one.
		//
		//   On a genuine fstat failure the operator sees neither state. verify() calls
		//   w.f.Stat() directly (it does not read this seam), so the same failure refuses
		//   every write as a dropped event: nothing rotates, nothing shards, and the
		//   counter this arm just set is never consulted again.
		//
		// So what this arm buys is the honesty of the counter and of the sentence printed
		// beside it — pinned by A62 — not a change in on-disk layout for the failure it
		// exists for.
		//
		// maxBytes(), not MaxBytes: MaxBytes is the *configured* threshold and zero
		// means "use the default", which is exactly what the daemon does — serve.go
		// builds &audit.Writer{Path: …, Logf: …} and sets nothing. Reading the field
		// there stored 0, and 0 is the permissive end: "measured as empty", the very
		// thing this arm exists to refuse, with the log line promising a rotation that
		// is then a full DefaultMaxBytes away.
		w.logf("audit: cannot stat %s to measure the existing trail: %v; treating the trail as full", w.Path, statErr)
		w.size = w.maxBytes()
		w.f = f
		return nil
	}
	if !fi.Mode().IsRegular() {
		f.Close()
		return fmt.Errorf("refusing to append the trail to %s: %w", w.Path, ErrNotRegular)
	}
	mine, err := ownerIsWriter(fi)
	if err != nil {
		f.Close()
		return err
	}
	if !mine {
		f.Close()
		return fmt.Errorf("%w: %s is owned by another account", ErrNotRootOwned, w.Path)
	}

	// The mode in the open is masked by the umask, so it is a request rather than a
	// result. Setting it on the descriptor is what makes PLAN §10's 0600 a fact
	// about the file instead of a fact about how the daemon happened to be started.
	if err := tightenMode(f, fileMode); err != nil {
		f.Close()
		return fmt.Errorf("chmod %s: %w", w.Path, err)
	}
	w.f = f
	// The size comes from the file, not from zero.
	//
	// The file is opened O_APPEND, so new bytes land after whatever is already there
	// while a counter started at zero decides when to rotate. Setting it to 0 therefore
	// does not mis-report a number, it moves the bound: every restart adds up to another
	// MaxBytes before the first rotation fires, so "five generations at 8 MiB is 40 MiB
	// worst case" is a per-run bound wearing a per-file bound's clothes.
	//
	// The shape that grows the log is the shape that restarts the daemon. KindPlatformError
	// is emitted per tick by the engine, and a machine whose desktop has died is also a
	// machine whose supervisor is restarting it. Disk full is then the failure that makes
	// this package's own failure mode permanent: the trail stops being written on the one
	// machine that most needs it.
	//
	// KindLockUnconfirmed used to be named here as well. It is now emitted once per
	// unconfirmed episode rather than per tick (#35), so it no longer contributes to the
	// growth this bound is sized against — which does not relax the bound. The per-tick
	// producer is still there in KindPlatformError, and a bound that only holds while the
	// engine behaves as expected is not a bound.
	w.size = fi.Size()
	return nil
}

// statFile measures a file the writer has just opened.
//
// It is a variable for the same reason geteuid is one a few files over: the branch it feeds
// is a guard against the OS refusing to measure a file it just agreed to open, and there is
// no portable way for an unprivileged test to arrange that on purpose. A guard that has
// never fired on any machine is a guard nobody has checked, and the direction it fails in is
// the direction that matters — treating an unmeasurable trail as empty moves the rotation
// bound back to zero and lets the file grow past what the operator configured.
var statFile = func(f *os.File) (os.FileInfo, error) { return f.Stat() }

// prepareDir makes sure the directory holding the trail exists and is private.
//
// It creates the directory when it is missing and sets its mode then, because a
// directory this package just made is this package to set. When the directory
// already exists it *checks and refuses* rather than correcting.
//
// Two reasons, and the second is the one that bites. The first is honesty: a
// directory somebody else made carries somebody else's intent, and silently
// rewriting it to 0700 hides that a change happened rather than reporting it.
// The second is that chmod by path is not a local operation. The daemon is root,
// and the path comes from a config file, so "chmod the directory of the audit
// path" is really "chmod an arbitrary directory as root": point settings at
// /var/log/plock.log and the previous version of this function set /var/log to
// 0700, which takes the box's whole logging down with it. Point it at /dev/null
// and it takes /dev down. Refusing is the only safe verb on a directory we did
// not create.
func (w *Writer) prepareDir(dir string) error {
	li, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dir, dirMode); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
		// MkdirAll honours the umask, so the mode we asked for is a request and the
		// directory it produced may be anything from 0700 down to nothing at all.
		// Setting it on the descriptor is the only way 0700 becomes a result.
		if err := w.setModeOnNewDir(dir, dirMode); err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}
	if li.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink: refusing to write the trail through a directory this process did not create", dir)
	}
	if !li.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	// World-writable, not group-writable. A directory anybody can write into is one
	// anybody can rename the trail out of, and the mode check is worth nothing
	// beyond that. Group membership is a legitimate way to let a log-reading
	// account see the directory — /var/log itself is root:adm 0775 on a good number
	// of installs — and refusing it here would make the daemon the odd one out on
	// machines that work.
	if li.Mode().Perm()&0o002 != 0 {
		return fmt.Errorf("%s is mode %04o: a directory anybody can write into is one they can move the trail out of", dir, li.Mode().Perm())
	}
	mine, err := ownerIsWriter(li)
	if err != nil {
		return err
	}
	if !mine {
		return fmt.Errorf("%s is owned by another account: refusing to append the trail to a directory somebody else controls", dir)
	}
	return nil
}

// setModeOnNewDir corrects the mode of a directory this process just created.
//
// It works on the descriptor of the parent rather than on the new directory
// itself, because a umask strict enough to make this matter leaves a directory the
// creator cannot open — see setDirMode.
//
// One limit worth naming before it becomes a surprise: MkdirAll may have created
// intermediate levels as well as the target, and only the target's mode is corrected
// here — the ancestors keep the umask-masked mode. That is unreachable for DefaultPath
// (one level under /var/log, whose parent always exists) and for any --state-dir whose
// parent exists, so it is not a live defect today. If a nested audit path ever becomes
// supported, cmd/plock/dir_unix.go is the model: it walks one component at a time so a
// hostile umask cannot make an intermediate directory unusable before the leaf's chmod
// runs, and it checks only the target rather than policing every ancestor with the
// leaf's ceiling.
func (w *Writer) setModeOnNewDir(dir string, mode os.FileMode) error {
	parent := filepath.Dir(dir)
	pf, err := openDir(parent)
	if err != nil {
		// The parent is where the audit path lives, so a parent this process cannot
		// read is a trail it cannot keep.
		return fmt.Errorf("open %s: %w", parent, err)
	}
	defer pf.Close()
	if err := setDirMode(pf, filepath.Base(dir), mode); err != nil {
		return fmt.Errorf("chmod %s: %w", dir, err)
	}
	return nil
}

// verify re-checks the descriptor on every write.
//
// The file is held open for speed, which means a path swap after the first write
// would otherwise be invisible: the descriptor keeps pointing at the inode we
// opened, and the name keeps pointing at whatever replaced it. Checking the name
// against the inode catches that, and checking the mode and owner catches the
// slower attacks — a file replaced by a hardlink, or chmodded by something with
// the right to do both.
//
// It is cheap: one stat on a path we already hold, per event, at one event per
// tick.
func (w *Writer) verify() error {
	fi, err := w.f.Stat()
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return ErrNotRegular
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%s is mode %04o: the trail must not be readable or writable by anyone but root", w.Path, fi.Mode().Perm())
	}
	mine, err := ownerIsWriter(fi)
	if err != nil {
		return err
	}
	if !mine {
		return fmt.Errorf("%w: %s is owned by another account", ErrNotRootOwned, w.Path)
	}

	// The name has to still be the file we are holding. A hardlink is the case the
	// descriptor alone cannot see: it is the same inode reached a second way, and
	// writing to it through our descriptor then changes the file the name points
	// at, which is not the file we agreed to write.
	li, err := os.Lstat(w.Path)
	if err != nil {
		return err
	}
	// The symlink question is asked first, before the identity comparison, even
	// though a link would fail that too. A symlink is not one of the ways a file
	// goes missing — it is a deliberate act against a root daemon — and an operator
	// reading the journal needs the sentence that says so rather than the generic
	// "no longer refers to the file this writer opened", which also covers a
	// rotation, a backup tool, and a disk full mid-rename.
	if li.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is now a symlink: something repointed the trail while this writer held it open", w.Path)
	}
	if !os.SameFile(fi, li) {
		return fmt.Errorf("%s no longer refers to the file this writer opened", w.Path)
	}
	return nil
}

// rotate moves the current file aside and opens a new one.
//
// The order matters. Renaming the name away and then creating a fresh file means
// there is a moment when the name does not exist, but never a moment when a line
// is written to the wrong file. The reverse — creating first — would let a
// concurrent writer append to the file we are about to retire.
func (w *Writer) rotate() error {
	if err := w.f.Close(); err != nil {
		// The descriptor is still unusable either way, so carry on and let the
		// reopen below report a real problem if there is one.
		w.logf("audit: closing %s during rotation: %v", w.Path, err)
	}
	w.f = nil
	w.size = 0

	oldest := fmt.Sprintf("%s.%d", w.Path, w.keep())
	if err := os.Remove(oldest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for i := w.keep() - 1; i >= 1; i-- {
		from := fmt.Sprintf("%s.%d", w.Path, i)
		if err := os.Rename(from, fmt.Sprintf("%s.%d", w.Path, i+1)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Rename(w.Path, w.Path+".1"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return w.open()
}

// noteFailure records a lost event and says so.
//
// Every failure is logged, not the first one. A daemon that logs the first
// failure and goes quiet is indistinguishable from one whose trail recovered, and
// the operator finds out at the worst possible moment. After the third the wording
// changes to say the run is continuing and how long it has been going, and that
// escalated line is emitted once per run rather than once per event so a
// filesystem that stays broken does not fill the journal it is complaining about.
func (w *Writer) noteFailure(f *os.File, op string, err error) {
	w.dropped++
	w.failures++
	if f != nil {
		// The descriptor may be pointing at a file we must not keep writing to, and
		// the next write would reopen it anyway. Dropping it here is what makes
		// "the next event re-verifies" true rather than aspirational.
		f.Close()
		w.f = nil
		// w.size is deliberately left alone. open() re-measures from the file when the
		// next event reopens it, and zeroing it here used to throw the accounting away
		// for the rest of the run: a stretch of failures mid-file would silently double
		// or triple how far past MaxBytes the trail grew.
	}
	switch {
	case w.failures == 1:
		w.logf("audit: %s failed, event NOT recorded (%s): %v", w.Path, op, err)
	case w.failures == 3 && !w.escalated:
		w.escalated = true
		w.logf("audit: STILL FAILING after %d events, %d total dropped (%s): %v — the trail is incomplete and enforcement is running unrecorded",
			w.failures, w.dropped, op, err)
	default:
		w.logf("audit: event NOT recorded (%s): %v", op, err)
	}
}

func (w *Writer) logf(format string, args ...any) {
	if w.Logf != nil {
		w.Logf(format, args...)
		return
	}
	// The default is the real logger, not silence. A Writer built as a zero value
	// or by a constructor that forgets the hook must not lose the one signal that
	// the trail is broken — that is the failure this package exists to report, and
	// reporting it is the only way it reaches anyone. internal/enforce had the
	// same hole in its constructor and the same fix.
	log.Printf(format, args...)
}

func (w *Writer) now() time.Time {
	if w.Now == nil {
		return time.Now()
	}
	return w.Now()
}

func (w *Writer) maxBytes() int64 {
	if w.MaxBytes != 0 {
		return w.MaxBytes
	}
	return DefaultMaxBytes
}

func (w *Writer) keep() int {
	if w.Keep > 0 {
		return w.Keep
	}
	return DefaultKeep
}
