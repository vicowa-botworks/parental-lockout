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

func TestSaveLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := sample().Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// A symlink target fails after the temp file exists, which is the failure
	// path most likely to strand one.
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	_ = sample().Save(link)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "tmp") {
			t.Errorf("temp file %q left behind", e.Name())
		}
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
	crashPointForTest = func(tmpName, target string) error {
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
	}
	defer func() { crashPointForTest = nil }()

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
	if c.Settings.PollSeconds != DefaultPollSeconds {
		t.Errorf("PollSeconds = %d, want %d", c.Settings.PollSeconds, DefaultPollSeconds)
	}
	if c.Settings.Listen != DefaultListen {
		t.Errorf("Listen = %q, want %q", c.Settings.Listen, DefaultListen)
	}
	if c.Settings.TLS != DefaultTLS {
		t.Errorf("TLS = %q, want %q", c.Settings.TLS, DefaultTLS)
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
