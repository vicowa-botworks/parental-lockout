package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func sample() *Config {
	until := time.Date(2026, 10, 12, 20, 40, 0, 0, time.UTC)
	return &Config{
		Version: CurrentVersion,
		Admin:   Admin{Username: "parent", Password: "pbkdf2-sha256$600000$abc$def", UpdatedAt: until},
		Settings: Settings{
			Listen:       "0.0.0.0:8443",
			TLS:          "auto",
			PollSeconds:  30,
			Enforcement:  true,
			LockBackends: []string{"logind", "kde", "gnome"},
		},
		Profiles: []Profile{{
			ID:       "schoolnight",
			Name:     "School night",
			Timezone: "Europe/Berlin",
			Rules: []Rule{
				{ID: "r1", Name: "Bedtime", Days: []string{"mon", "tue", "wed", "thu", "sun"}, Start: "21:00", End: "06:30", Enabled: true},
				{ID: "r2", Name: "School hours", Days: []string{"mon", "tue", "wed", "thu", "fri"}, Start: "08:15", End: "13:30", Enabled: true},
			},
		}},
		Users:     []User{{Username: "kiddo", ProfileID: "schoolnight", Enabled: true}},
		Overrides: []Override{{Username: "kiddo", Until: until, Reason: "homework with teacher", By: "parent"}},
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("seeding %s: %v", path, err)
	}
}

// The acceptance criterion is that Save then Load is the identity. Byte
// stability is the strictest honest form of that claim: whatever Save produced
// is exactly what Load hands back to the next Save.
func TestSaveLoadRoundTrip(t *testing.T) {
	cases := map[string]*Config{
		"empty":            {},
		"defaults":         Default(),
		"populated":        sample(),
		"needs defaults":   {Admin: Admin{Username: "p"}, Profiles: []Profile{{ID: "x", Timezone: "UTC"}}},
		"unicode and \"\"": {Admin: Admin{Username: `älïcë "quoted" ✓`}},
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")

			if err := in.Save(path); err != nil {
				t.Fatalf("Save: %v", err)
			}
			first, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}

			back, err := Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if want := in.normalized(); !reflect.DeepEqual(back, want) {
				t.Errorf("Load(Save(c)) != c.normalized():\n got %+v\nwant %+v", back, want)
			}

			if err := back.Save(path); err != nil {
				t.Fatalf("second Save: %v", err)
			}
			second, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if string(first) != string(second) {
				t.Errorf("a document changed on a second save:\nfirst:\n%s\nsecond:\n%s", first, second)
			}
		})
	}
}

func TestSaveCreatesMode0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	// A permissive umask must not leak into this file's mode: it holds the
	// administrator's password verifier.
	if err := sample().Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode after save = %o, want 600", perm)
	}

	// Saving over a file that someone loosened must tighten it again.
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	if err := sample().Save(path); err != nil {
		t.Fatalf("Save over: %v", err)
	}
	fi, err = os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode after re-save = %o, want 600", perm)
	}
}

func TestSaveRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "shadow")
	write(t, target, "do not clobber me")
	link := filepath.Join(dir, "config.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	err := sample().Save(link)
	if err == nil {
		t.Fatal("Save through a symlink succeeded, want refusal")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error = %v, want it to say why (a symlink)", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "do not clobber me" {
		t.Errorf("the symlink target was written through: %q", got)
	}
}

func TestSaveRefusesNonRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	if err := sample().Save(path); err == nil {
		t.Fatal("Save onto a directory succeeded, want refusal")
	}
}

// The failure that matters is the one after the temp file exists: a save that
// dies between the fsync and the rename must not leave a copy of the whole
// document, password verifier included, sitting in the config directory.
func TestFailedSaveLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	aborted := errors.New("aborted")
	setCrashPoint(func(string, string) error { return aborted })
	err := sample().Save(path)
	defer setCrashPoint(nil)
	if !errors.Is(err, aborted) {
		t.Fatalf("Save error = %v, want the injected abort", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("temp file %q left behind after a failed save", e.Name())
		}
	}
}

// A save that never got as far as creating a temp file must not create one
// either, and must not touch the target.
func TestRefusedSaveCreatesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "config.json")
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	if err := sample().Save(link); err == nil {
		t.Fatal("Save through a symlink succeeded")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("the guard fired too late: temp file %q was created first", e.Name())
		}
	}
}

// Load is the reaper for a temp file a killed process could not clean up.
func TestLoadReapsStaleTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := Default().Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	stale := filepath.Join(dir, ".config.json.tmp123456")
	fresh := filepath.Join(dir, ".config.json.tmp999999")
	for _, name := range []string{stale, fresh} {
		if err := os.WriteFile(name, []byte("a stranded copy of the document"), 0o600); err != nil {
			t.Fatalf("seeding %s: %v", name, err)
		}
	}
	old := time.Now().Add(-2 * staleTempAge)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	// Something unrelated to the save protocol is none of Load's business. It is
	// old on purpose: the name is the only thing that tells the two apart.
	unrelated := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(unrelated, []byte("keep"), 0o600); err != nil {
		t.Fatalf("seeding %s: %v", unrelated, err)
	}
	if err := os.Chtimes(unrelated, old, old); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	if _, err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Error("the stranded temp file survived Load")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("a save in flight lost its work file: %v", err)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Errorf("Load deleted an unrelated file: %v", err)
	}
}

// The file holds the administrator's password verifier, so a mode that lets the
// child's account read it is repaired on sight rather than enforced around.
func TestLoadTightensALooseMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := sample().Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	if _, err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode after Load = %o, want 600", perm)
	}
}

// The crash criterion: a save interrupted at its most dangerous instant must
// leave the old document whole. The interruption is injected between the fsync
// of the temp file and the rename, which is the only window in which a partial
// file could ever be observed.
func TestSaveInterruptedLeavesOldFileWhole(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	old := Default()
	old.Admin.Username = "original"
	if err := old.Save(path); err != nil {
		t.Fatalf("seeding Save: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	crash := errors.New("simulated power loss")
	setCrashPoint(func(tmpName, target string) error {
		if target != path {
			t.Errorf("crash point saw target %q, want %q", target, path)
		}
		// At the instant of the crash the live file is still the old one...
		live, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("reading the live file at the crash point: %v", err)
		}
		if string(live) != string(before) {
			t.Error("the live file was replaced before the rename")
		}
		// ...and the incoming file is already complete, so a crash here loses
		// the new save but cannot produce a half-written document.
		staged, err := os.ReadFile(tmpName)
		if err != nil {
			t.Fatalf("reading the staged file at the crash point: %v", err)
		}
		if !strings.Contains(string(staged), "replaced") {
			t.Errorf("the staged file is not the complete new document: %q", staged)
		}
		if fi, err := os.Stat(tmpName); err != nil {
			t.Errorf("Stat staged: %v", err)
		} else if perm := fi.Mode().Perm(); perm != 0o600 {
			t.Errorf("staged file mode = %o, want 600", perm)
		}
		if filepath.Dir(tmpName) != dir {
			t.Errorf("staged file is in %q, want the same directory as the target (%q)", filepath.Dir(tmpName), dir)
		}
		return crash
	})
	defer setCrashPoint(nil)

	newCfg := sample()
	newCfg.Admin.Username = "replaced"
	err = newCfg.Save(path)
	if !errors.Is(err, crash) {
		t.Fatalf("Save error = %v, want the simulated crash", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after crash: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("the live document changed across a failed save:\nbefore:\n%s\nafter:\n%s", before, after)
	}

	back, err := Load(path)
	if err != nil {
		t.Fatalf("Load after crash: %v", err)
	}
	if back.Admin.Username != "original" {
		t.Errorf("admin after crash = %q, want %q", back.Admin.Username, "original")
	}
}

func TestLoadRefusesUnknownVersion(t *testing.T) {
	for _, version := range []string{"999", "0", "2"} {
		t.Run(version, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			body := fmt.Sprintf(`{"version": %s, "admin": {"username": "parent"}}`, version)
			write(t, path, body)

			_, err := Load(path)
			if err == nil {
				t.Fatal("Load accepted a document it cannot understand")
			}
			if !errors.Is(err, ErrUnknownVersion) {
				t.Errorf("error = %v, want it to wrap ErrUnknownVersion", err)
			}
			if !strings.Contains(err.Error(), "version") {
				t.Errorf("error = %v, want it to name the version", err)
			}

			// Refusing must not "repair" the file: silently rewriting a newer
			// document would drop the fields this build does not know about.
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if string(got) != body {
				t.Errorf("the rejected file was modified:\n%s", got)
			}
		})
	}
}

func TestLoadRejectsUnreadableDocuments(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"unknown key": {
			body: `{"version":1,"profilles":[]}`,
			want: "profilles",
		},
		"trailing data": {
			body: `{"version":1} {"version":1}`,
			want: "after the JSON object",
		},
		"truncated": {
			body: `{"version":1,"admin":{`,
			want: "parse",
		},
		"not json": {
			body: `this is not json`,
			want: "parse",
		},
		"version of the wrong type": {
			body: `{"version":"one"}`,
			want: "parse",
		},
		// Strictness has to survive the custom unmarshalers, or the one section
		// a parent hand-edits is the one section that stops reporting typos.
		"unknown key inside settings": {
			body: `{"version":1,"settings":{"pollSecond":9}}`,
			want: "pollSecond",
		},
		"unknown key inside admin": {
			body: `{"version":1,"admin":{"user":"parent"}}`,
			want: `"user"`,
		},
		"unknown key inside a rule": {
			body: `{"version":1,"profiles":[{"id":"p","timezone":"UTC","rules":[{"id":"r","dayz":["mon"],"start":"21:00","end":"06:00"}]}]}`,
			want: "dayz",
		},
		"settings of the wrong type": {
			body: `{"version":1,"settings":[1,2]}`,
			want: "parse",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			write(t, path, tc.body)

			_, err := Load(path)
			if err == nil {
				t.Fatalf("Load accepted %q", tc.body)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// Enforcement is the master switch for the whole program. Every way a document
// can fail to say "off" has to mean "on", because a parent who never
// deliberately turned enforcement off must not discover it was off all week.
func TestEnforcementDefaultsToOn(t *testing.T) {
	cases := map[string]string{
		"no settings object":           `{"version":1,"admin":{"username":"parent"}}`,
		"empty settings":               `{"version":1,"settings":{}}`,
		"settings without enforcement": `{"version":1,"settings":{"pollSeconds":45}}`,
		"explicit true":                `{"version":1,"settings":{"enforcement":true}}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			write(t, path, body)

			c, err := Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !c.Settings.Enforcement {
				t.Error("Enforcement = false, want the default of true")
			}
		})
	}
}

func TestLoadAppliesDefaultsToAnAbsentSettingsObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	write(t, path, `{"version":1}`)

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := defaultSettings(); !reflect.DeepEqual(c.Settings, want) {
		t.Errorf("Settings = %+v, want %+v", c.Settings, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.json")

	_, err := Load(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error = %v, want it to wrap os.ErrNotExist", err)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	write(t, path, `{"version":1,"admin":{"username":"parent"}}`)

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Literals, not the constants: a default asserted against the constant it
	// comes from would survive the constant being changed to anything.
	if c.Settings.PollSeconds != 30 {
		t.Errorf("PollSeconds = %d, want 30", c.Settings.PollSeconds)
	}
	if c.Settings.Listen != "0.0.0.0:8443" {
		t.Errorf("Listen = %q, want 0.0.0.0:8443", c.Settings.Listen)
	}
	if c.Settings.TLS != "auto" {
		t.Errorf("TLS = %q, want auto", c.Settings.TLS)
	}
	if want := []string{"logind", "kde", "gnome"}; !reflect.DeepEqual(c.Settings.LockBackends, want) {
		t.Errorf("LockBackends = %v, want %v", c.Settings.LockBackends, want)
	}
	// Enforcement is a zero value in JSON, so a document that omits it must not
	// silently mean "off".
	if !c.Settings.Enforcement {
		t.Error("Enforcement = false, want the default of true")
	}
}

// A partial settings object still has to pick up the settings it left out.
// Load's applyDefaults is the only thing that does this for a present-but-
// incomplete object, so it is not covered by the no-settings-object case above.
func TestLoadAppliesDefaultsToAPartialSettingsObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	write(t, path, `{"version":1,"settings":{"pollSeconds":45}}`)

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Settings.PollSeconds != 45 {
		t.Errorf("PollSeconds = %d, want the explicit 45", c.Settings.PollSeconds)
	}
	if c.Settings.Listen != "0.0.0.0:8443" {
		t.Errorf("Listen = %q, want the default", c.Settings.Listen)
	}
	if c.Settings.TLS != "auto" {
		t.Errorf("TLS = %q, want the default", c.Settings.TLS)
	}
	if len(c.Settings.LockBackends) != 3 {
		t.Errorf("LockBackends = %v, want the default three", c.Settings.LockBackends)
	}
}

func TestLoadKeepsExplicitFalse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	write(t, path, `{"version":1,"settings":{"enforcement":false,"pollSeconds":7}}`)

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Settings.Enforcement {
		t.Error("Enforcement = true, want the explicit false preserved")
	}
	if c.Settings.PollSeconds != 7 {
		t.Errorf("PollSeconds = %d, want 7", c.Settings.PollSeconds)
	}
}

// The enforcement loop reads the document on every poll while a parent saves
// from the web UI. Rename is atomic, so a reader must never see a torn file —
// and this is the test the race detector is worth running for.
func TestConcurrentLoadDuringSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := sample().Save(path); err != nil {
		t.Fatalf("seeding Save: %v", err)
	}

	const writers = 4
	const rounds = 25

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				c, err := Load(path)
				if err != nil {
					t.Errorf("Load during concurrent save: %v", err)
					return
				}
				if c.Version != CurrentVersion {
					t.Errorf("loaded version %d mid-save", c.Version)
					return
				}
			}
		}()
	}

	for i := range rounds {
		c := sample()
		c.Admin.Username = fmt.Sprintf("parent-%d", i)
		if err := c.Save(path); err != nil {
			t.Fatalf("Save round %d: %v", i, err)
		}
	}
	close(stop)
	wg.Wait()
}

// A read-only /etc/plock, a missing directory, a full disk: the operator has to
// be told the save failed rather than watching the UI report success while the
// old document keeps being enforced.
func TestSaveReportsAnUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions, so this test would prove nothing")
	}

	t.Run("missing parent directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "absent", "config.json")
		if err := sample().Save(path); err == nil {
			t.Fatal("Save into a nonexistent directory succeeded")
		}
	})

	t.Run("read-only directory", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.json")
		seed := Default()
		seed.Admin.Username = "original"
		if err := seed.Save(path); err != nil {
			t.Fatalf("seeding Save: %v", err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}

		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatalf("Chmod: %v", err)
		}
		t.Cleanup(func() { os.Chmod(dir, 0o700) })

		if err := sample().Save(path); err == nil {
			t.Fatal("Save into a read-only directory succeeded")
		}

		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile after the failed save: %v", err)
		}
		if string(after) != string(before) {
			t.Error("the failed save changed the live document")
		}
	})
}

// Save must not write a document this build could not read back: a daemon that
// starts, saves, and then cannot load its own file is dead on arrival.
func TestSaveRefusesAVersionItCannotRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := Default().Save(path); err != nil {
		t.Fatalf("seeding Save: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	c := sample()
	c.Version = 7
	err = c.Save(path)
	if err == nil {
		t.Fatal("Save wrote a version this build would refuse to load")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("error = %v, want it to name the version", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(after) != string(before) {
		t.Error("the refused save changed the live document")
	}
}

// A path the guard cannot inspect is a refusal, not a pass. This is the branch
// that decides whether an EACCES or an ELOOP fails open.
func TestSaveRefusesAPathItCannotStat(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions, so this test would prove nothing")
	}
	dir := t.TempDir()
	hidden := filepath.Join(dir, "hidden")
	if err := os.Mkdir(hidden, 0o700); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.Chmod(hidden, 0); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(hidden, 0o700) })

	err := sample().Save(filepath.Join(hidden, "config.json"))
	if err == nil {
		t.Fatal("Save into a directory it cannot see succeeded")
	}
	// The point is that the guard refused because it could not look, not that
	// some later step happened to fail too.
	if !strings.Contains(err.Error(), "stat") {
		t.Errorf("error = %v, want the guard's own refusal to write what it cannot inspect", err)
	}
}

// Two documents in one file is how a hand edit goes quietly wrong: the second
// one's fields would be dropped without a word.
func TestLoadRejectsTrailingDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	write(t, path, `{"version":1}`+"\n"+`{"version":1}`)

	if _, err := Load(path); err == nil {
		t.Fatal("Load accepted two documents in one file")
	}
}

func TestWatchReportsChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := Default().Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	done := make(chan struct{})
	defer close(done)
	changes := Watch(path, 10*time.Millisecond, done)

	// A save can land inside the same clock tick as the seed, so force the pair
	// of fields Watch actually looks at to differ.
	time.Sleep(30 * time.Millisecond)
	c := sample()
	c.Admin.Username = "changed"
	if err := c.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	select {
	case <-changes:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not report a save")
	}

	select {
	case <-changes:
		t.Error("Watch reported the same change twice")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestWatchReportsAFileAppearing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	done := make(chan struct{})
	defer close(done)
	changes := Watch(path, 10*time.Millisecond, done)

	if err := Default().Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	select {
	case <-changes:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not report the file appearing")
	}
}

// cp -p, tar -x and rsync --mtime all produce a file with the original size and
// the original timestamp. Size and mtime alone would call that "no change", and
// the daemon would go on enforcing a schedule that is no longer on disk.
func TestWatchNoticesAReplacedFileWithIdenticalSizeAndMtime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	stamp := time.Now().Add(-time.Hour)
	if err := os.WriteFile(path, []byte("aaaaaaaaaa"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	done := make(chan struct{})
	defer close(done)
	changes := Watch(path, 10*time.Millisecond, done)

	// Same length, same mtime, different inode: only the inode moves.
	incoming := filepath.Join(dir, "incoming")
	if err := os.WriteFile(incoming, []byte("bbbbbbbbbb"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Chtimes(incoming, stamp, stamp); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	if err := os.Rename(incoming, path); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	select {
	case <-changes:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch missed a replacement whose size and mtime were unchanged")
	}
}

// The channel holds one pending signal, so a burst of edits costs the consumer
// one reload. That only works because the channel is buffered: with an
// unbuffered one the non-blocking send drops nearly every signal in a burst and
// nothing is ever delivered.
func TestWatchCoalescesABurstIntoOneSignal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	done := make(chan struct{})
	defer close(done)
	changes := Watch(path, 10*time.Millisecond, done)

	for i := range 5 {
		c := sample()
		c.Admin.Username = fmt.Sprintf("parent-%d", i)
		if err := c.Save(path); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
		time.Sleep(15 * time.Millisecond)
	}

	select {
	case <-changes:
	case <-time.After(2 * time.Second):
		t.Fatal("a burst of five saves produced no signal at all")
	}
	select {
	case <-changes:
		t.Error("a burst queued more than one reload")
	case <-time.After(150 * time.Millisecond):
	}
}

// A zero interval would panic inside time.NewTicker, in a goroutine with nobody
// above it to recover.
func TestWatchTreatsAZeroIntervalAsTheDefault(t *testing.T) {
	done := make(chan struct{})
	changes := Watch(filepath.Join(t.TempDir(), "absent.json"), 0, done)

	close(done)
	select {
	case _, ok := <-changes:
		if ok {
			t.Error("Watch delivered a signal after done was closed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch panicked or hung on a zero interval")
	}
}

func TestWatchStopsWhenDone(t *testing.T) {
	done := make(chan struct{})
	changes := Watch(filepath.Join(t.TempDir(), "absent.json"), 10*time.Millisecond, done)

	close(done)
	select {
	case _, ok := <-changes:
		if ok {
			t.Error("Watch delivered a signal after done was closed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not close its channel when done was closed")
	}
}

// The read path is the half a child can aim at. Replacing the configuration with
// a link makes every reader follow it, and Load used to tighten the mode of what
// it read by path — which turned "fix the credential file's permissions" into
// "chmod any root-owned file on the machine". /usr/bin/sudo goes 0755 to 0700
// with one ln -s, from the process with the most privileges in the product.
func TestLoadRefusesASymlinkedConfig(t *testing.T) {
	dir := t.TempDir()

	// The link has to point at a document that would actually load, or the test
	// passes because decoding failed rather than because the link was refused.
	secret := filepath.Join(dir, "shadow")
	if err := sample().Save(secret); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.Chmod(secret, 0o644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	path := filepath.Join(dir, "config.json")
	if err := os.Symlink(secret, path); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load through a symlink succeeded, want refusal")
	}
	if !errors.Is(err, ErrRefusesSymlink) {
		t.Errorf("error = %v, want it to wrap ErrRefusesSymlink", err)
	}

	fi, err := os.Stat(secret)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o644 {
		t.Errorf("the link target's mode is %o after a refused Load, want 644 — it was chmodded through the link", perm)
	}
}

// A directory is openable but not readable as a file, and a FIFO blocks the open
// until someone writes to it. Both are refused before the open, so neither can
// park the enforcement loop in a call that never returns.
func TestLoadRefusesADirectory(t *testing.T) {
	path := t.TempDir()

	if _, err := Load(path); err == nil {
		t.Fatal("Load of a directory succeeded, want refusal")
	} else if !strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("error = %v, want it to say the path is not a regular file", err)
	}
}

// A missing file has to stay distinguishable from a refused one: the daemon
// treats "nothing configured yet" differently from "someone tampered".
func TestLoadStillReportsAMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error = %v, want it to wrap os.ErrNotExist", err)
	}
}

// The mode is tightened on the descriptor Load holds, never on the path. This is
// a second guard rather than a restatement of the first: whoever is quick enough
// to swap a link in after the open finds the file they aimed at left alone, even
// if the check at the open were ever bypassed.
func TestTightenModeIgnoresAPathSwappedAfterTheOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := sample().Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	f, err := openReadable(path)
	if err != nil {
		t.Fatalf("openReadable: %v", err)
	}
	defer f.Close()

	shadow := filepath.Join(dir, "shadow")
	if err := sample().Save(shadow); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.Chmod(shadow, 0o644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := os.Symlink(shadow, path); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	tightenMode(f)

	shadowFi, err := os.Stat(shadow)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := shadowFi.Mode().Perm(); perm != 0o644 {
		t.Errorf("the file swapped in behind the path was chmodded to %o, want 644 untouched", perm)
	}

	held, err := f.Stat()
	if err != nil {
		t.Fatalf("f.Stat: %v", err)
	}
	if perm := held.Mode().Perm(); perm != 0o600 {
		t.Errorf("the file the descriptor points at is %o, want 600 — tightening did not happen where it should have", perm)
	}
}
