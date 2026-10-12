package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrUnknownVersion reports a document written by a schema this build does not
// understand. It is a hard refusal rather than a best-effort read: acting on a
// partially understood schedule could lock a child out at the wrong time, or
// fail to lock them out at all.
var ErrUnknownVersion = errors.New("unsupported config schema version")

// errOwnershipUnsupported is returned on platforms where a file's owner cannot
// be determined, so the tamper guard fails closed instead of waving the check
// through.
var errOwnershipUnsupported = errors.New("this platform cannot report a file's owner")

// errNotRegular reports a config path that is a device, socket or directory
// rather than a file.
var errNotRegular = errors.New("not a regular file")

// ErrRefusesSymlink reports a config path that is a symlink. It is exported
// because a caller may reasonably want to tell "someone replaced the file with a
// link" apart from "the file is missing".
var ErrRefusesSymlink = errors.New("refusing to follow a symlinked config path")

// staleTempAge is how old a leftover save file has to be before Load reaps it.
// A Save takes milliseconds, so anything older than this belongs to a process
// that no longer exists.
const staleTempAge = time.Hour

// Load reads and decodes the document at path, applies defaults, and refuses a
// version this build does not understand.
//
// It also repairs two things it finds, because both are worse than the state
// that caused them: a credential file that group or other can read is tightened
// to 0600, and a save file stranded by a crash is deleted. Both are
// best-effort — Load is called by the enforcement loop, and refusing to enforce
// because a cleanup failed would be the wrong trade.
func Load(path string) (*Config, error) {
	f, err := openReadable(path)
	if err != nil {
		return nil, fmt.Errorf("config: open %s: %w", path, err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()

	var c Config
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	// The document is one JSON object; anything after it means two documents in
	// one file, which is how a hand edit goes quietly wrong. This is the one
	// place that check lives.
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("config: parse %s: unexpected data after the JSON object", path)
	}

	if c.Version != CurrentVersion {
		return nil, fmt.Errorf("config: %s: %w: found %d, this build reads only %d",
			path, ErrUnknownVersion, c.Version, CurrentVersion)
	}

	c.applyDefaults()
	tightenMode(f)
	reapStaleTempFiles(path, time.Now())
	return &c, nil
}

// openReadable opens the configuration file for reading, refusing a symlink or
// anything that is not a regular file.
//
// The shape check happens before the open, not after it: opening a FIFO for
// reading blocks until someone writes to it, which would park the enforcement
// loop in a call that never returns. O_NOFOLLOW then closes the gap between
// looking at the path and opening it, so a link swapped in during that window
// fails instead of being followed.
//
// This is the read half of the tamper guard, and it deliberately checks less
// than the write half. Refusing to load stops enforcement, which is the failure
// this product exists to prevent, and a child who can plant a file in /etc/plock
// can already write the configuration directly. What the read path must never do
// is follow a link or open a device: that hands the most privileged process in
// the product a way to read — and, until this check existed, to chmod — a file
// outside the directory it was given.
func openReadable(path string) (*os.File, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, &os.PathError{Op: "open", Path: path, Err: ErrRefusesSymlink}
	}
	if !fi.Mode().IsRegular() {
		return nil, &os.PathError{Op: "open", Path: path, Err: errNotRegular}
	}
	return openNoFollow(path)
}

// tightenMode removes group and other permissions from the credential file.
//
// Fixing it on sight beats refusing: the alternative to reading this file is
// enforcing nothing, and a child whose machine stopped being locked out because
// someone ran `chmod a+r` is the outcome this package exists to prevent.
//
// It works on the descriptor Load already holds, never on the path. Chmodding by
// path follows a symlink and races with whoever made it: the file that gets
// tightened would not necessarily be the file that was read.
func tightenMode(f *os.File) {
	fi, err := f.Stat()
	if err != nil {
		return
	}
	perm := fi.Mode().Perm()
	if perm&0o077 == 0 {
		return
	}
	fchmodFile(f, perm&^0o077)
}

// reapStaleTempFiles deletes save files a crashed Save left behind.
//
// They are mode 0600 copies of the whole document, password verifier included,
// so leaving one in /etc/plock forever is not acceptable. The age floor is what
// keeps this safe: a Save that is still in flight must not lose its work file.
func reapStaleTempFiles(path string, now time.Time) {
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix := "." + filepath.Base(path) + ".tmp"

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil || now.Sub(info.ModTime()) < staleTempAge {
			continue
		}
		os.Remove(filepath.Join(dir, entry.Name()))
	}
}

// crashPointForTest, when set, runs after the temp file has been written and
// fsynced but before it is renamed into place, and aborts the save if it returns
// an error. That window is the only instant in which a crash could leave a
// partial configuration behind, so it is the instant the tests need to look at.
// It is nil outside tests.
//
// The mutex is not decoration: Save reads it on every call, so a test that sets
// it while another test saves — which is what t.Parallel invites — would
// otherwise be a data race in the middle of the production path.
var (
	crashMu           sync.Mutex
	crashPointForTest func(tmpName, target string) error
)

func setCrashPoint(hook func(tmpName, target string) error) {
	crashMu.Lock()
	crashPointForTest = hook
	crashMu.Unlock()
}

func takeCrashPoint() func(tmpName, target string) error {
	crashMu.Lock()
	defer crashMu.Unlock()
	return crashPointForTest
}

// Save writes the document atomically at mode 0600.
//
// The order is the guarantee: write a temp file in the same directory, fsync
// it, rename it over the target, then fsync the directory. A reader therefore
// sees either the whole old file or the whole new one, and a machine that dies
// between the write and the rename leaves the old file intact with a stray
// temp file beside it — never a half-written configuration that the enforcement
// loop would act on. Load reaps the stray.
//
// Save does not call Validate. Read-side code must keep working with whatever
// is on disk, so the gate belongs to whoever produced the change, not to the
// writer. It does refuse a version it would not be able to read back: writing a
// document that the next start cannot load would take the daemon down at the
// worst possible moment.
func (c *Config) Save(path string) error {
	out := c.normalized()
	if out.Version != CurrentVersion {
		return fmt.Errorf("config: refusing to write version %d to %s: this build reads only version %d",
			out.Version, path, CurrentVersion)
	}
	if err := checkWritableTarget(path); err != nil {
		return err
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("config: encode: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp")
	if err != nil {
		return fmt.Errorf("config: create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	// Until the rename succeeds there is a temp file to clean up. After it,
	// tmpName is cleared and the deferred removal is a no-op.
	defer func() {
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}()

	if err := writeAndSync(tmp, data); err != nil {
		return fmt.Errorf("config: write %s: %w", tmpName, err)
	}
	if hook := takeCrashPoint(); hook != nil {
		if err := hook(tmpName, path); err != nil {
			return err
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("config: rename %s to %s: %w", tmpName, path, err)
	}
	tmpName = ""

	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("config: open directory %s to sync: %w", dir, err)
	}
	if err := d.Sync(); err != nil {
		d.Close()
		return fmt.Errorf("config: sync directory %s: %w", dir, err)
	}
	if err := d.Close(); err != nil {
		return fmt.Errorf("config: close directory %s: %w", dir, err)
	}
	return nil
}

// writeAndSync puts data on disk and flushes it before the file handle goes
// away. Chmod is explicit rather than relying on CreateTemp's 0600, because the
// mode of this file is a security property, not a preference.
func writeAndSync(f *os.File, data []byte) error {
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// checkWritableTarget is the tamper guard. A child who somehow obtains write
// access to the configuration directory must not be able to redirect the write
// elsewhere, or have the daemon overwrite a file they own.
//
// Known limit, stated rather than papered over: the guard inspects the target,
// not the directory above it. If /etc/plock is itself a symlink to
// attacker-writable space, the file is created there. Reaching that needs write
// access one level above the configuration directory, where an attacker already
// holds strictly better options, so the guard stops at the check that pays for
// itself.
func checkWritableTarget(path string) error {
	fi, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		// Creating the first file: there is nothing to protect yet.
		return nil
	}
	if err != nil {
		// EACCES, ELOOP, anything else. Not being able to look is not a pass.
		return fmt.Errorf("config: stat %s: %w", path, err)
	}
	return checkTarget(path, fi, os.Geteuid())
}

// checkTarget is the guard with its inputs handed to it, so the ownership rule
// can be tested without a second user account on the machine.
func checkTarget(path string, fi os.FileInfo, euid int) error {
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("config: refusing to write through %s, which is a symlink", path)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("config: refusing to write %s, which is not a regular file", path)
	}

	uid, err := fileUID(fi)
	if err != nil {
		return fmt.Errorf("config: cannot check the owner of %s: %w", path, err)
	}
	if uid != euid {
		return fmt.Errorf("config: refusing to overwrite %s: owned by uid %d, this process is uid %d",
			path, uid, euid)
	}
	return nil
}

// stamp is what changes whenever the file's contents change. Each field alone
// has a hole: size misses an equal-length edit, mtime is coarse on some
// filesystems and is preserved by cp -p and rsync --mtime, and an inode number
// alone misses an in-place write. Together they catch a replacement by any of
// the tools a parent might reach for.
type stamp struct {
	modTime time.Time
	size    int64
	ino     uint64
}

func statOf(path string) stamp {
	fi, err := os.Stat(path)
	if err != nil {
		return stamp{}
	}
	return stamp{modTime: fi.ModTime(), size: fi.Size(), ino: fileIno(fi)}
}

// Watch reports on the returned channel whenever the file at path changes, by
// polling its stat fields — no new dependency, and portable to the inotify
// alternatives.
//
// It takes a done channel rather than returning only a subscription channel: a
// poller with no way to be stopped is a goroutine that outlives its purpose.
// The returned channel is closed when done is closed. A nil done never stops,
// which is what the daemon wants — it watches for the life of the process.
//
// Bursts are coalesced — the channel holds at most one pending signal, because
// a consumer reloads the whole document and does not need to know how many
// writes happened. A consumer that misses a signal still reloads on its own
// enforcement tick, so a coarse mtime degrades latency, never correctness.
func Watch(path string, interval time.Duration, done <-chan struct{}) <-chan struct{} {
	if interval <= 0 {
		// A zero interval would panic inside time.NewTicker, in a goroutine
		// nobody can recover from.
		interval = time.Second
	}
	changes := make(chan struct{}, 1)
	// The baseline is taken here, before Watch returns, so the first comparison
	// is against the state the caller saw when it subscribed. Taking it inside
	// the goroutine would race with a write that happens immediately after
	// subscribing, and a file that appears in that window would never be
	// reported at all.
	last := statOf(path)

	go func() {
		defer close(changes)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				now := statOf(path)
				if now == last {
					continue
				}
				last = now
				select {
				case changes <- struct{}{}:
				default:
				}
			}
		}
	}()

	return changes
}
