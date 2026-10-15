package web

import (
	"strings"
	"testing"
)

// assetURL's empty answer is the one the shell template has to cope with, so it is a
// contract and not an accident: a name that is not in the bundle must produce "", which
// the template renders as no link at all. Returning the bare name instead would emit
// href="app.css" -- a request that 404s and looks like a broken daemon, which is the
// exact thing embedding the assets was done to prevent.
func TestAnAssetThatIsNotInTheBundleHasNoURL(t *testing.T) {
	s, _ := pageWithSeams(t)
	if got := s.assetURL("app.css"); got == "" {
		t.Fatal("a bundled asset resolved to no URL; every page would render unstyled")
	} else if !strings.HasPrefix(got, "/static/") {
		t.Errorf("asset URL = %q, want a /static/ path", got)
	}
	if got := s.assetURL("not-bundled.css"); got != "" {
		t.Errorf("an unknown asset resolved to %q, want the empty answer the template skips on", got)
	}
}
