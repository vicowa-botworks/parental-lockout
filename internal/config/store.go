package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// Load reads and decodes the document at path, applies defaults, and refuses a
// version this build does not understand.
//
// Unknown JSON keys are rejected as well. A hand-edited document is a supported
// way to change this file, and a misspelled key that decodes to nothing is how a
// parent ends up believing they changed a setting they did not.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
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
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("config: parse %s: unexpected data after the JSON object", path)
	}

	if c.Version != CurrentVersion {
		return nil, fmt.Errorf("config: %s: %w: found %d, this build reads only %d",
			path, ErrUnknownVersion, c.Version, CurrentVersion)
	}

	c.applyDefaults()
	return &c, nil
}

// crashPointForTest, when set, runs after the temp file has been written and
// fsynced but before it is renamed into place, and aborts the save if it returns
// an error. That window is the only instant in which a crash could leave a
// partial configuration behind, so it is the instant the tests need to look at.
// It is nil outside tests.
var crashPointForTest func(tmpName, target string) error

// Save writes the document atomically at mode 0600.
//
// The order is the guarantee: write a temp file in the same directory, fsync
// it, rename it over the target, then fsync the directory. A reader therefore
// sees either the whole old file or the whole new one, and a machine that dies
// between the write and the rename leaves the old file intact with a stray
// temp file beside it — never a half-written configuration that the enforcement
// loop would act on.
//
// Save does not call Validate. Read-side code must keep working with whatever
// is on disk, so the gate belongs to whoever produced the change, not to the
// writer.
func (c *Config) Save(path string) error {
	if err := checkWritableTarget(path); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c.normalized(), "", "  ")
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
	if crashPointForTest != nil {
		if err := crashPointForTest(tmpName, path); err != nil {
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
func checkWritableTarget(path string) error {
	fi, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		// Creating the first file: there is nothing to protect yet.
		return nil
	}
	if err != nil {
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

// stamp is the pair of stat fields that change whenever the file's contents
// change. Size alone misses an equal-length edit; mtime alone can be too coarse
// on a filesystem with one-second timestamps.
type stamp struct {
	modTime time.Time
	size    int64
}

func statOf(path string) stamp {
	fi, err := os.Stat(path)
	if err != nil {
		return stamp{}
	}
	return stamp{modTime: fi.ModTime(), size: fi.Size()}
}

// Watch reports on the returned channel whenever the file at path changes, by
// polling its mtime and size — no new dependency, and portable to the inotify
// alternatives.
//
// It takes a done channel rather than returning only a subscription channel: a
// poller with no way to be stopped is a goroutine that outlives its purpose.
// The returned channel is closed when done is closed.
//
// Bursts are coalesced — the channel holds at most one pending signal, because
// a consumer reloads the whole document and does not need to know how many
// writes happened. A consumer that misses a signal still reloads on its own
// enforcement tick, so a coarse mtime degrades latency, never correctness.
func Watch(path string, interval time.Duration, done <-chan struct{}) <-chan struct{} {
	if interval <= 0 {
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
