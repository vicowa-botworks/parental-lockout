package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// settingsPlatform is a desktop the tick can consult without touching this machine. It reports no
// sessions at all, which is the point: the only thing under test is the report's own enforcement
// field, and a fixture that also produced sessions would let a lock-path regression masquerade as
// a settings result.
type settingsPlatform struct{}

func (settingsPlatform) Name() string { return "fixture" }

func (settingsPlatform) Sessions(context.Context) ([]platform.Session, error) { return nil, nil }

func (settingsPlatform) Lock(context.Context, platform.Session) error { return nil }

func (settingsPlatform) Probe(context.Context) []platform.BackendStatus { return nil }

// Issue #99's third acceptance criterion is that the master switch is written through the *same
// config seam the enforcement loop reads*, and that a test proves the loop sees it rather than
// proving the file changed.
//
// Those are different claims and the difference is the whole reason this file exists. A test that
// writes the config and re-reads it establishes only that the filesystem works; the daemon could
// still be enforcing from a document it loaded at startup and never drops. The failure this is
// written against is #59, where every page shipped green because each package's own tests passed
// about an object no production path ever assembled.

// TestTheServedSettingsPageIsGatedNotMissing is the assembly half.
//
// It asks the assembled server, not a hand-built web.Options, what an unauthenticated request to
// the two new routes gets. The answer has to be the auth gate's 303, because a route nobody
// registered answers 404 -- and 404 is exactly what #59 shipped, at full green, behind links the
// operator could see.
func TestTheServedSettingsPageIsGatedNotMissing(t *testing.T) {
	dir := t.TempDir()
	path := writeServeConfig(t, dir)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	srv, err := serveServer(cfg, nil, users.Source{}, platform.SelfLocation{}, path,
		filepath.Join(dir, "state"), filepath.Join(dir, "trail.jsonl"),
		context.Background(), os.Stdout, os.Stderr)
	if err != nil {
		t.Fatalf("serveServer: %v", err)
	}

	// GET only, and the omission is the finding rather than an oversight. csrfMiddleware wraps the
	// mux rather than sitting in each handler, so a tokenless POST is answered by the gate before
	// the mux is ever consulted: measured, POST /settings answers 303 to /login with the
	// registration deleted and 303 with it present. A POST assertion here would therefore pass
	// against a daemon that cannot serve the route at all. TestTheSettingsRoutesAreRegistered in
	// the web package is what pins the POST registration, and the refusal path is pinned there by
	// the route-coverage guard.
	for _, want := range []struct{ method, target string }{
		{"GET", "/settings"},
	} {
		req := httptest.NewRequest(want.method, want.target, nil)
		req.Header.Set("Accept", "text/html")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
			t.Errorf("%s %s answered %d Location=%q, want 303 to /login: a 404 would mean the "+
				"route is not in the mux the daemon serves, which is #59's defect arriving as a "+
				"link that goes nowhere", want.method, want.target, rec.Code,
				rec.Header().Get("Location"))
		}
	}
}

// TestTheSettingsSeamIsTheOneTheEnforcementLoopReads is the load-bearing half.
//
// It writes through `srv.Mutate` -- the seam serveServer handed the web server, which is what
// POST /settings reaches through mutateConfig -- and then runs one tick of the engine serveEngine
// assembled over the same config path. The tick's own report is the assertion, not the file: a
// loop that kept enforcing from a document it loaded at startup would leave the file perfectly
// correct and the child still locked out, and the file test would pass.
func TestTheSettingsSeamIsTheOneTheEnforcementLoopReads(t *testing.T) {
	dir := t.TempDir()
	path := writeServeConfig(t, dir)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.Settings.Enforcement {
		t.Fatal("the fixture starts with enforcement off, so the assertion below proves nothing")
	}

	trail := &audit.Writer{Path: filepath.Join(dir, "trail.jsonl")}
	srv, err := serveServer(cfg, nil, users.Source{}, platform.SelfLocation{}, path,
		filepath.Join(dir, "state"), filepath.Join(dir, "trail.jsonl"),
		context.Background(), os.Stdout, os.Stderr)
	if err != nil {
		t.Fatalf("serveServer: %v", err)
	}
	// The seam has to exist before it can be the right one. An absent Mutate makes the web
	// server's own guard answer 503, so a test that skipped this check would be asserting that
	// nothing happened after nothing was wired.
	if srv.Mutate == nil {
		t.Fatal("the assembled server has no config write seam, so /settings could not save " +
			"anything on the real daemon")
	}

	// The same two fields the page writes, through the seam the page writes them with.
	if err := srv.Mutate(func(c *config.Config) error {
		c.Settings.Enforcement = false
		c.Settings.PollSeconds = 60
		return nil
	}); err != nil {
		t.Fatalf("the served write seam failed: %v", err)
	}

	eng := serveEngine(path, settingsPlatform{}, trail, platform.SelfLocation{}, func(string, ...any) {})
	rep := eng.Tick(context.Background())
	if rep.Enforcement {
		t.Errorf("the enforcement loop's own tick still reports enforcement on after the served " +
			"seam switched it off; the file may well be correct, which is why the file is not what " +
			"is asserted here: a loop reading a stale document keeps locking children out beside a " +
			"page that says it stopped")
	}

	// And back the other way, because the claim is that the loop re-reads on every tick rather
	// than once. A loop that noticed the switch going off but cached the document afterwards would
	// pass the assertion above and then keep enforcing off forever after the parent switched it
	// back on -- which is the direction that silently stops protecting the child.
	if err := srv.Mutate(func(c *config.Config) error {
		c.Settings.Enforcement = true
		return nil
	}); err != nil {
		t.Fatalf("the served write seam failed on the second write: %v", err)
	}
	if rep := eng.Tick(context.Background()); !rep.Enforcement {
		t.Errorf("the enforcement loop still reports enforcement off after the served seam " +
			"switched it back on; the loop is holding a document it loaded once, so a switch " +
			"flipped from the page would take a daemon restart to take effect")
	}

	// The file last, and as the record of both writes rather than as the proof: the assertions
	// above are about what the loop saw, which is the claim the criterion makes.
	fresh, err := config.Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !fresh.Settings.Enforcement || fresh.Settings.PollSeconds != 60 {
		t.Errorf("the document on disk holds %+v, want the interval from the first write and the "+
			"switch back on from the second", fresh.Settings)
	}
}
