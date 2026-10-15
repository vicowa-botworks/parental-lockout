package web

import (
	"testing"
	"testing/fstest"
)

// The asset map has to arrive through the constructor, because the constructor is the only
// place the embedded bundle is ever read.
//
// The existing asset test builds its Server through the page fixture, which assigns `assets`
// directly -- so it asserts what assetURL does with a map and never that New hands one over.
// W44 replaced `s.assets = m` with a drop and the whole suite stayed green: New still
// succeeded, the fixture still had its map, and the daemon that ships would 404 every
// /static/ request. That is the unstyled page embedding the assets exists to prevent, and
// the log says nothing when it happens.
//
// So this goes through New with nothing assigned afterwards, and asks the question the
// operator's browser asks.
func TestNewHandsTheAssetMapToTheServer(t *testing.T) {
	s, err := New(Options{
		StoreDir: t.TempDir(),
		TLSMode:  TLSOff,
		Listen:   "127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// The default Static is the embedded bundle, so a real name must resolve. If this
	// comes back empty, every page renders with no stylesheet and no script.
	if got := s.assetURL("app.css"); got == "" {
		t.Fatal("New built a server with no asset map; every /static/ request would 404")
	}
	// And the map has to be the bundle's, not merely non-empty: the hashed URL is what
	// makes the immutable cache header honest.
	if got := s.assetURL("definitely-not-bundled.css"); got != "" {
		t.Errorf("an unbundled name resolved to %q; the map is not the bundle New was given", got)
	}
}

// The other half: a caller that supplies its own tree gets that tree, not the embedded one.
// Without this, defaulting `Static` before the map is built could silently ignore a
// caller's asset filesystem and the test above would still pass.
func TestNewUsesTheAssetTreeItWasGiven(t *testing.T) {
	s, err := New(Options{
		StoreDir: t.TempDir(),
		TLSMode:  TLSOff,
		Listen:   "127.0.0.1:0",
		Static:   fstest.MapFS{"own.css": {Data: []byte("body{}")}},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.assetURL("own.css") == "" {
		t.Error("the supplied asset tree was ignored; the server serves the embedded bundle instead")
	}
	if s.assetURL("app.css") != "" {
		t.Error("the embedded bundle leaked into a server that was given its own tree")
	}
}
