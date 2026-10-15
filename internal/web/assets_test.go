package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The assets have to be in the binary, not on disk next to it.
//
// The install is one copied binary and one config file. A directory of assets
// beside it is a second thing install.sh can fail to place, and the symptom is an
// admin UI that looks broken — which is the worst possible thing for a tool whose
// value depends on being trusted.
func TestTheAssetsAreEmbedded(t *testing.T) {
	a := Assets()
	for _, name := range []string{"app.css", "app.js"} {
		b, err := fs.ReadFile(a, name)
		if err != nil {
			t.Errorf("%s is not in the binary: %v", name, err)
			continue
		}
		if len(b) == 0 {
			t.Errorf("%s is embedded but empty", name)
		}
	}
}

func TestAssetURLCarriesAContentHash(t *testing.T) {
	a := Assets()
	url, err := AssetURL(a, "app.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "/static/app.") || !strings.HasSuffix(url, ".css") {
		t.Errorf("AssetURL = %q, want /static/app.<hash>.css", url)
	}
	if !isHashedName(url) {
		t.Errorf("%q does not look like a hashed name", url)
	}

	// The hash is over the bytes, so the same content always maps to the same URL
	// and different content never does.
	other, err := AssetURL(a, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	if other == url {
		t.Error("two different files produced the same URL")
	}
}

// The immutable cache header is only honest because the name changes with the
// content. Serving app.css under a URL that does not change would pin a stale
// stylesheet on every parent's phone for a year.
func TestAnUnknownHashIsNotFoundRatherThanServedUnhashed(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{})
	s.Static = Assets()
	m, err := AssetMap(s.Static)
	if err != nil {
		t.Fatal(err)
	}
	s.assets = m

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/static/app.deadbeefdeadbeef.css", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404: falling back to app.css would make the immutable header a way to pin a stale file", rec.Code)
	}
}

func TestAHashedAssetIsServedWithALongCacheAndNoSniffing(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{})
	s.Static = Assets()
	m, _ := AssetMap(s.Static)
	s.assets = m

	var url string
	for u := range m {
		if strings.HasSuffix(u, ".css") {
			url = u
		}
	}
	if url == "" {
		t.Fatal("no css asset in the map")
	}

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q, want immutable", cc)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

// The CSP is default-src 'self', which rejects an inline script. A script tag in
// the page would work in development and fail the moment the headers are on, which
// is the worst time to find out.
func TestTheJavaScriptHasNoInlineCounterpartInThePage(t *testing.T) {
	a := Assets()
	js, err := fs.ReadFile(a, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	// A handler attribute in the JS string would be injected into the DOM and
	// blocked by the same rule.
	for _, bad := range []string{"onclick=", "onload=", "eval(", "document.write("} {
		if strings.Contains(string(js), bad) {
			t.Errorf("app.js contains %q, which the CSP rejects or which is unsafe", bad)
		}
	}
}

func TestAssetMapCoversEveryFile(t *testing.T) {
	a := Assets()
	m, err := AssetMap(a)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	fs.WalkDir(a, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			count++
		}
		return nil
	})
	if len(m) != count {
		t.Errorf("the map has %d entries for %d files", len(m), count)
	}
}

func isHashedName(url string) bool {
	base := url[strings.LastIndexByte(url, '/')+1:]
	stem := strings.TrimSuffix(base, ".css")
	stem = strings.TrimSuffix(stem, ".js")
	i := strings.LastIndexByte(stem, '.')
	if i < 0 {
		return false
	}
	hash := stem[i+1:]
	if len(hash) != 16 {
		return false
	}
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
