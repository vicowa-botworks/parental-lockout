package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"testing/fstest"
	"time"
)

// W10: custom TLS must refuse when *either* half is missing, not only when both are.
//
// With && instead of ||, a config that names a certificate and forgets the key passes
// the startup check and fails on the first handshake -- exactly the failure mode the
// "fail fast at startup" criterion exists to prevent.
func TestCustomTLSRefusesWhenOnlyOneHalfIsNamed(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "c.pem")
	key := filepath.Join(dir, "k.pem")
	for _, p := range []string{cert, key} {
		if err := os.WriteFile(p, []byte("not a key pair"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	now := func() time.Time { return time.Unix(1700000000, 0).UTC() }

	for name, pair := range map[string][2]string{
		"cert without a key": {cert, ""},
		"key without a cert": {"", key},
	} {
		_, err := LoadCustom(pair[0], pair[1], now)
		if err == nil {
			t.Errorf("%s: startup accepted a custom TLS config with one half missing", name)
			continue
		}
		// Which guard fired is the assertion. With && instead of || the error still
		// arrives -- tls.LoadX509KeyPair cannot read an empty path -- so asserting only
		// "an error happened" passes on the broken code and proves nothing. The startup
		// check is what must answer, and its sentence names the empty half.
		if !strings.Contains(err.Error(), "empty") {
			t.Errorf("%s: refused for the wrong reason (%v), so the emptiness check is not what stopped it", name, err)
		}
	}
}

// W16: an expired CSRF token must stop resolving at lookup, not only at put.
//
// put() sweeps expired entries, so a store that is only written to never shows the
// leak -- but a session that goes quiet keeps its token in the map, and lookup is the
// check that has to hold when nothing has written since. With the expiry test removed,
// a token minted at sign-in stays valid for the life of the process.
func TestAExpiredCSRFTokenStopsResolvingOnLookup(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	c := &csrfStore{tokens: map[string]csrfEntry{}, now: func() time.Time { return now }}
	c.put("session", "token", now.Add(time.Hour))

	if _, ok := c.lookup("session"); !ok {
		t.Fatal("a token inside its window did not resolve, so the assertion below proves nothing")
	}
	now = now.Add(2 * time.Hour)
	if _, ok := c.lookup("session"); ok {
		t.Error("an expired CSRF token still resolved, so it is valid for the life of the process")
	}
}

// W18: the logout cookie has to carry MaxAge=-1, not 0.
//
// The two look interchangeable and are not. MaxAge=0 means "emit no Max-Age
// attribute", so the browser falls back to Expires alone, and a cookie carrying only a
// past Expires is handled inconsistently across clients -- some keep the value until
// restart. MaxAge=-1 is the spelling that means delete now. Both attributes are set
// because a stale clock on the client can make either one alone fail to delete.
func TestLogoutAsksTheBrowserToDeleteTheCookie(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{})
	rec := httptest.NewRecorder()
	s.clearSession(rec)

	var found *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == sessionCookie {
			found = ck
		}
	}
	if found == nil {
		t.Fatal("logout set no session cookie at all")
	}
	if found.MaxAge >= 0 {
		t.Errorf("logout cookie Max-Age = %d, want a negative value: 0 means 'no attribute', not 'expire now'", found.MaxAge)
	}
	if !found.Expires.Before(time.Now()) {
		t.Errorf("logout cookie Expires = %v, want a time in the past", found.Expires)
	}
}

// W12: O_NOFOLLOW is the guard, not the mode check that happens to catch some cases.
//
// A symlink to a private 0700 directory passes every check that runs after the open: it
// is a directory, it is private, it is ours. Only refusing to walk the final symlink
// rejects it. Without the flag the daemon writes a private key into a directory the
// session user pointed it at, and the audit trail of who did that is a symlink the same
// user controls.
func TestAPrivateDirectoryBehindASymlinkIsRefused(t *testing.T) {
	real := t.TempDir()
	if err := os.Chmod(real, 0o700); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	link := filepath.Join(root, "tls")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	if err := ensurePrivateDir(link); err == nil {
		t.Error("a private directory reached through a symlink was accepted: only O_NOFOLLOW rejects it, and nothing else does")
	}
	// Control: the same directory addressed directly is accepted, so the refusal above
	// is about the symlink and not about the directory.
	if err := ensurePrivateDir(real); err != nil {
		t.Errorf("the real directory itself was refused (%v), so the assertion above proves nothing", err)
	}
}

// W40: the asset URL has to hash the bytes, not the name.
//
// The URL carries the hash precisely so the response can say "immutable, cache this
// forever". Fold the filename into the digest and two builds that differ only in content
// produce the same URL -- every browser that cached the old bytes keeps them, and the
// fix the operator just deployed never reaches the machine they deployed it to. The
// failure is silent and permanent until the name changes.
func TestTheAssetURLChangesWhenTheContentChanges(t *testing.T) {
	first := fstest.MapFS{"app.js": {Data: []byte("console.log(1)")}}
	second := fstest.MapFS{"app.js": {Data: []byte("console.log(2)")}}

	a, err := AssetURL(first, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	b, err := AssetURL(second, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Errorf("two different files named app.js share the URL %s, so the immutable header pins the old bytes forever", a)
	}

	// And the converse: the name must not be part of the digest, or a rename would
	// invalidate a cache entry for bytes nobody changed.
	renamed := fstest.MapFS{"bundle.js": {Data: []byte("console.log(1)")}}
	c, err := AssetURL(renamed, "bundle.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(c, strings.TrimPrefix(a, "/assets/")) && c == a {
		t.Errorf("identical bytes under a new name produced the same URL, so the name is in the digest")
	}
}

// W8: the mode argument is applied, not merely requested.
//
// os.CreateTemp already makes a 0600 file, so a caller that asks for 0600 cannot tell
// the difference -- which is why this survived. The distinction is real for any other
// mode, and the function is general: a caller that asks for a group-readable cert file
// and silently gets 0600 has a broken deployment, and a caller that asks for 0600 and
// gets whatever the umask allowed has an exposed key. Pinning it with a mode that is
// not 0600 is the only way to see the line work.
func TestWriteFilePrivateAppliesTheModeItWasGiven(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cert.pem")
	const want os.FileMode = 0o640

	if err := writeFilePrivate(path, []byte("bytes"), want); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != want {
		t.Errorf("the file is mode %04o, want %04o: the Chmod on the held descriptor is what makes the requested mode a fact rather than a request", got, want)
	}
}

// W6: a hostname the machine reports is not necessarily a legal DNS name.
//
// Safari rejects a certificate outright when a dNSName is not a valid label, so an
// over-eager SAN is not a warning to dismiss -- it is a LAN that cannot load the page,
// with a certificate file that looks fine. The guard is what keeps a machine named
// something like "living room" from taking the whole admin UI down.
func TestAnIllegalHostnameIsLeftOutOfTheCertificate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tls")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	store := &CertStore{
		Dir:       dir,
		Now:       func() time.Time { return time.Unix(1700000000, 0).UTC() },
		Hostnames: []string{"plock.local", "living room", "", "-bad-", "ok.example"},
	}
	cert, err := store.LoadOrMint()
	if err != nil {
		t.Fatalf("LoadOrMint: %v", err)
	}
	leaf := cert.Leaf
	if leaf == nil {
		t.Fatal("the minted certificate carries no leaf, so nothing below is checked")
	}

	got := map[string]bool{}
	for _, n := range leaf.DNSNames {
		got[n] = true
	}
	for _, bad := range []string{"living room", "-bad-", ""} {
		if got[bad] {
			t.Errorf("%q went into the certificate's SAN list; a client that validates dNSName rejects the whole certificate for it", bad)
		}
	}
	// Control: the legal names did get in, so the loop above is not passing because no
	// hostname was added at all.
	for _, good := range []string{"plock.local", "ok.example", "localhost"} {
		if !got[good] {
			t.Errorf("%q is a legal name and was left out, so the rejection above may be vacuous", good)
		}
	}
}

// W39: a drain that gives up has to be said out loud.
//
// Ignoring Shutdown's error makes the loudest kind of lie available here: requests were
// cut off mid-flight -- a parent's config save can be one of them -- and the daemon
// exits as if it had stopped cleanly. The only reason this was untested is that
// reaching it used to mean holding a request open for the full ten-second budget;
// DrainTimeout makes it a fifty-millisecond test.
func TestADrainThatGivesUpSaysSo(t *testing.T) {
	s, logged, _ := newTestServer(t, &stubAuth{})
	s.TLSMode = TLSOff
	s.DrainTimeout = 50 * time.Millisecond

	release := make(chan struct{})
	started := make(chan struct{})
	s.extraRoutes = func(mux *http.ServeMux) {
		mux.HandleFunc("GET /slow", func(http.ResponseWriter, *http.Request) {
			close(started)
			<-release
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()

	var addr string
	for i := 0; i < 200; i++ {
		if addr = s.Addr(); addr != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if addr == "" {
		t.Fatal("Serve never bound")
	}
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		cancel()
	})

	go func() {
		client := &http.Client{Timeout: 20 * time.Second}
		if resp, err := client.Get("http://" + addr + "/slow"); err == nil {
			resp.Body.Close()
		}
	}()
	<-started
	cancel()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Serve did not return after the drain expired")
	}
	close(release)

	if !anyContains(logged.Lines(), "shutdown drained with an error") {
		t.Errorf("the drain gave up and nothing said so; a request was cut off and the exit looked clean. logged: %q", logged.Lines())
	}
}
