package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The transport form (#99, third slice), tested the way the repo tests a write path: an accepted
// submit is asserted against the stored document, and a refused one against the absence of both a
// write and an audit line.
//
// This form has one property the other two do not, and most of the file is about it. `tls`,
// `certFile` and `keyFile` are read once, by `web.New`, when the daemon starts. Every other
// setting on this page is wrong until it is corrected; one of these, saved wrongly, means the
// daemon does not come back at all — including the page you would have used to correct it. So the
// refusals here are not only "did the handler say no", but "did the handler say no about the pair
// the daemon would actually have tried to load", and "did it say no without describing the
// filesystem to a caller who is not root".

// setTLS puts the document's transport in a known position before a submit.
func (f *settingsFixture) setTLS(mode, cert, key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.doc.Settings.TLS = mode
	f.doc.Settings.CertFile = cert
	f.doc.Settings.KeyFile = key
}

// validPair mints a certificate and key into dir through the same store the daemon uses, and
// returns the two paths. Minting rather than embedding a fixture certificate means the pair cannot
// expire and start failing the suite in three years.
func validPair(t *testing.T, dir string) (string, string) {
	t.Helper()
	store, _, _ := newStore(t)
	// A path the store creates itself, not t.TempDir() directly: the store refuses to write a
	// private key into a directory that is more open than 0700, and a fresh temp dir is 0775.
	store.Dir = filepath.Join(dir, "tls")
	if _, err := store.LoadOrMint(); err != nil {
		t.Fatalf("mint: %v", err)
	}
	return filepath.Join(store.Dir, certFileName), filepath.Join(store.Dir, keyFileName)
}

// radioChecked reports whether the named radio control is rendered as selected.
//
// It looks inside the tag rather than searching the whole page because three of the page's
// controls can carry `checked` at once (the enforcement pair and this trio), and a substring
// test on the body would pass on whichever one happens to be lit.
func radioChecked(t *testing.T, body, name, value string) bool {
	t.Helper()
	marker := `name="` + name + `" value="` + value + `"`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("the page has no radio %s=%q at all:\n%s", name, value, body)
	}
	end := strings.Index(body[i:], ">")
	if end < 0 {
		t.Fatalf("the radio %s=%q tag is unterminated", name, value)
	}
	return strings.Contains(body[i:i+end], "checked")
}

func TestTheTransportFormShowsTheStoredTransport(t *testing.T) {
	f := newSettingsFixture(t)
	f.setTLS("custom", "/etc/plock/cert.pem", "/etc/plock/key.pem")

	body := authGet(t, f.s, "/settings").Body.String()
	if !radioChecked(t, body, "tls", "custom") {
		t.Error("the stored mode is not the checked one, so the page answers a question it was not asked")
	}
	if radioChecked(t, body, "tls", "off") || radioChecked(t, body, "tls", "auto") {
		t.Error("more than one transport position is checked; the page cannot hold two answers")
	}
	// The paths come back as stored, because the operator's next question is "is that the file
	// I think it is", and a box that shows nothing forces them to open the config to answer it.
	for _, want := range []string{`name="certFile" value="/etc/plock/cert.pem"`,
		`name="keyFile" value="/etc/plock/key.pem"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not carry %q.\n%s", want, body)
		}
	}
}

func TestTheTransportFormSaysWhatEachPositionMeansAndWhenItApplies(t *testing.T) {
	f := newSettingsFixture(t)
	body := authGet(t, f.s, "/settings").Body.String()
	// "Saved" beside a transport change is the sentence an operator reads as "the wire changed".
	// It did not: the transport was chosen at start-up. The page has to say so before the submit,
	// not only in the confirmation after it, because the confirmation is only read by somebody who
	// already believes the change took effect.
	for _, want := range []string{
		"next time the daemon starts",
		// The fail-open clause: #99's criterion is that a control which stops protecting the
		// session names what stops happening.
		"cross the network in clear text",
		// The other direction, so "automatic" is not read as a certificate from an authority.
		"not signed by an authority",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the transport form never says %q, so the control is offered without what it "+
				"does.\n%s", want, body)
		}
	}
}

func TestACustomPairTheDaemonCanLoadIsSavedAndAudited(t *testing.T) {
	dir := t.TempDir()
	cert, key := validPair(t, dir)
	f := newSettingsFixture(t)
	f.setTLS("auto", "", "")

	rec := authPost(t, f.s, "/settings/tls",
		"tls=custom&certFile="+cert+"&keyFile="+key)
	if rec.Code != http.StatusOK {
		t.Fatalf("a loadable pair answered %d, want 200:\n%s", rec.Code, rec.Body.String())
	}
	got := f.settings()
	if got.TLS != "custom" || got.CertFile != cert || got.KeyFile != key {
		t.Errorf("the document holds tls=%q cert=%q key=%q, want custom and the two paths",
			got.TLS, got.CertFile, got.KeyFile)
	}
	lines := f.settingsAuditLines()
	if len(lines) != 1 {
		t.Fatalf("an accepted transport change wrote %v, want exactly one line", lines)
	}
	// Both ends of each change and the who, in the same kind as the switch and the interval:
	// "what changed on this machine" has to stay one grep.
	for _, want := range []string{`tls "auto" to custom`, "certFile \"\" to " + strconv.Quote(cert),
		"keyFile \"\" to " + strconv.Quote(key), "by parent"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("the audit line %q does not say %q", lines[0], want)
		}
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Saved: tls") {
		t.Errorf("the page does not confirm what it saved.\n%s", body)
	}
	if !strings.Contains(body, "loaded just now") {
		t.Errorf("the confirmation does not say the pair was checked against the start-up rule, "+
			"which is the whole reason this submit was accepted.\n%s", body)
	}
}

func TestATransportSubmitThatChangesNothingSaysSoAndWritesNothing(t *testing.T) {
	f := newSettingsFixture(t)
	f.setTLS("off", "", "")

	rec := authPost(t, f.s, "/settings/tls", "tls=off&certFile=&keyFile=")
	if rec.Code != http.StatusOK {
		t.Fatalf("a no-op submit answered %d, want 200:\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Nothing changed, so nothing was written.") {
		t.Errorf("a no-op submit did not say so.\n%s", rec.Body.String())
	}
	if n := f.savesCount(); n != 0 {
		t.Errorf("a no-op submit saved %d times; the config file was renamed for nothing", n)
	}
	if lines := f.settingsAuditLines(); len(lines) != 0 {
		t.Errorf("a no-op submit wrote %v; the trail is for things that happened", lines)
	}
}

// TestAnUnknownOrMissingModeIsRefused covers the two ways the trio arrives wrong: not at all, and
// with a value the server has no branch for. Both are refusals rather than defaults for the
// reason the enforcement pair refuses a missing field (#873) — the page always submits one of the
// three, so a request that does not is a truncated body or a hand-built POST, and guessing which
// transport the operator meant is the fail-open kind of guessing.
func TestAnUnknownOrMissingModeIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name, form, want string
	}{
		{"absent", "certFile=&keyFile=", "settings.tls was not submitted"},
		{"https", "tls=https&certFile=&keyFile=", "settings.tls must be one of auto, off, custom"},
		// The quotes arrive as &#34; because the page escapes the echoed value, which is the
		// behaviour being asserted as much as the sentence is: whatever a caller puts in this
		// field is printed as text.
		{"wrong case", "tls=Auto&certFile=&keyFile=", `not &#34;Auto&#34;`},
		{"a mode config does not know", "tls=acme&certFile=&keyFile=", `not &#34;acme&#34;`},
		{"empty", "tls=&certFile=&keyFile=", "settings.tls was not submitted"},
	} {
		f := newSettingsFixture(t)
		f.setTLS("auto", "", "")

		rec := authPost(t, f.s, "/settings/tls", tc.form)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: answered %d, want 422: the operator caused this and the server cannot "+
				"retry it", tc.name, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, tc.want) {
			t.Errorf("%s: the refusal does not say %q.\n%s", tc.name, tc.want, body)
		}
		if !strings.Contains(body, `action="/settings/tls"`) {
			t.Errorf("%s: the refusal is not the form; the operator has to navigate back to "+
				"correct it.\n%s", tc.name, body)
		}
		if got := f.settings(); got.TLS != "auto" {
			t.Errorf("%s: a refused submit moved the stored mode to %q", tc.name, got.TLS)
		}
		if n := f.savesCount(); n != 0 {
			t.Errorf("%s: a refused submit saved %d times", tc.name, n)
		}
		if lines := f.settingsAuditLines(); len(lines) != 0 {
			t.Errorf("%s: a refused submit wrote %v", tc.name, lines)
		}
	}
}

func TestCustomWithAnEmptyPathIsRefusedNamingTheBox(t *testing.T) {
	// config's own check says "certFile or keyFile is empty" because it is written to cover
	// either. The operator holding the form needs the box, which is the only thing this handler
	// knows that the document-level sentence cannot.
	for _, tc := range []struct {
		form, want, notWant string
	}{
		{"tls=custom&certFile=&keyFile=", "settings.certFile and settings.keyFile", ""},
		{"tls=custom&certFile=/x.pem&keyFile=", "settings.keyFile", ""},
		{"tls=custom&certFile=&keyFile=/x.key", "settings.certFile", "settings.keyFile"},
	} {
		f := newSettingsFixture(t)
		f.setTLS("auto", "", "")

		rec := authPost(t, f.s, "/settings/tls", tc.form)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%q answered %d, want 422", tc.form, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, tc.want) {
			t.Errorf("%q was refused without naming %q.\n%s", tc.form, tc.want, body)
		}
		if tc.notWant != "" && strings.Contains(body, tc.notWant) {
			t.Errorf("%q named %q as well, so the operator is told to fill a box they already "+
				"filled.\n%s", tc.form, tc.notWant, body)
		}
		if got := f.settings(); got.TLS != "auto" {
			t.Errorf("%q changed the stored mode to %q", tc.form, got.TLS)
		}
		if lines := f.settingsAuditLines(); len(lines) != 0 {
			t.Errorf("%q was refused and still wrote %v", tc.form, lines)
		}
	}
}

// TestAPairTheDaemonCouldNotLoadIsRefusedWithoutDescribingTheFilesystem is the reason this form
// calls LoadCustom instead of checking the paths itself, and the reason LoadCustom reads files
// through a bounded, shape-checked open.
//
// Every case below is a path an operator can type into a browser form. Before the bound existed
// each was measured against the plain tls.LoadX509KeyPair: /dev/zero never returns and dies in
// the runtime's allocator, a named pipe with no writer parks in open forever, and the rest come
// back with a sentence about the file. The first two are why the read is bounded; all of them are
// why the sentence is not shown.
//
// The wall clock is part of the assertion for the two shapes that hang: a regression that removes
// the bound does not fail this test with a wrong answer, it fails it by never finishing, so the
// cases are timed and the budget is stated rather than left to the suite's timeout.
func TestAPairTheDaemonCouldNotLoadIsRefusedWithoutDescribingTheFilesystem(t *testing.T) {
	dir := t.TempDir()
	_, otherKey := validPair(t, filepath.Join(dir, "other"))
	cert, key := validPair(t, dir)
	notPEM := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(notPEM, []byte("this is not a certificate\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	type badPair struct{ name, cert, key string }
	cases := []badPair{
		{name: "missing", cert: filepath.Join(dir, "nope.pem"), key: key},
		{name: "a directory", cert: dir, key: key},
		{name: "not PEM", cert: notPEM, key: key},
		{name: "a mismatched pair", cert: cert, key: otherKey},
	}
	if _, err := os.Stat("/dev/zero"); err == nil {
		// The device is the case the bound exists for, and it is platform-specific, so the suite
		// stays honest about which half it actually ran rather than passing everywhere on one box.
		cases = append(cases, badPair{name: "an endless device", cert: "/dev/zero", key: "/dev/zero"})
	}

	for _, tc := range cases {
		f := newSettingsFixture(t)
		f.setTLS("auto", "", "")
		start := time.Now()

		rec := authPost(t, f.s, "/settings/tls", "tls=custom&certFile="+tc.cert+"&keyFile="+tc.key)
		if elapsed := time.Since(start); elapsed > 10*time.Second {
			t.Errorf("%s: the refusal took %s. The read is supposed to be bounded, which is the "+
				"only reason a typed path cannot park this request or exhaust the daemon.", tc.name, elapsed)
		}
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: answered %d, want 422", tc.name, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "could not be loaded") {
			t.Errorf("%s: the refusal does not say the pair could not be loaded.\n%s", tc.name, body)
		}
		if !strings.Contains(body, "plock doctor") {
			t.Errorf("%s: the refusal does not tell the operator where the reason is.\n%s", tc.name, body)
		}
		// The reason itself stays out of the page. This handler runs as root, so every distinct
		// sentence is a different fact about a path a signed-in-but-unprivileged caller can ask
		// about: existence, readability, and whether the first bytes are a PEM block.
		for _, leak := range []string{"no such file", "permission denied", "PEM", "is a directory",
			"not a regular file", "private key", "mismatch", "x509", "expired", "certificate file larger"} {
			if strings.Contains(body, leak) {
				t.Errorf("%s: the refusal leaks %q, which is a fact about the filesystem that "+
					"belongs in the daemon's log and not in a page.\n%s", tc.name, leak, body)
			}
		}
		if got := f.settings(); got.TLS != "auto" || got.CertFile != "" || got.KeyFile != "" {
			t.Errorf("%s: a refused submit stored tls=%q cert=%q key=%q",
				tc.name, got.TLS, got.CertFile, got.KeyFile)
		}
		if n := f.savesCount(); n != 0 {
			t.Errorf("%s: a refused submit saved %d times", tc.name, n)
		}
		if lines := f.settingsAuditLines(); len(lines) != 0 {
			t.Errorf("%s: a refused submit wrote %v", tc.name, lines)
		}
	}
}

func TestTheTypedPathsComeBackButTheIntervalDoesNot(t *testing.T) {
	f := newSettingsFixture(t)
	f.set(true, 45)
	f.setTLS("auto", "", "")

	rec := authPost(t, f.s, "/settings/tls", "tls=custom&certFile=/nope/a.pem&keyFile=/nope/b.key")
	body := rec.Body.String()
	// The boxes this submit carried have to hold what was typed: a refusal about a path is
	// useless without the path still in front of the operator.
	for _, want := range []string{`value="/nope/a.pem"`, `value="/nope/b.key"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the refusal does not hold %q.\n%s", want, body)
		}
	}
	// The interval box keeps the stored number. This submit never carried it, and a box showing
	// something the daemon is not running would be a second lie on a page that is already refusing.
	if !strings.Contains(body, `value="45"`) {
		t.Errorf("the interval box no longer holds the stored 45.\n%s", body)
	}
}

func TestAForgedTransportFormIsRefusedWithoutTouchingTheSeam(t *testing.T) {
	f := newSettingsFixture(t)
	f.setTLS("auto", "", "")
	resp, _ := login(t, f.s, "parent", "hunter2")
	c := cookieOf(t, resp)

	rec := postWithoutToken(t, f, c, "/settings/tls", "tls=off&certFile=&keyFile=")
	if rec.Code != http.StatusForbidden {
		t.Errorf("a forged token answered %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got := f.settings(); got.TLS != "auto" {
		t.Errorf("a forged form moved the transport to %q", got.TLS)
	}
	if lines := f.settingsAuditLines(); len(lines) != 0 {
		t.Errorf("a forged form wrote %v", lines)
	}
}

// TestTheRunningTransportIsNotMovedByASave pins the sentence the confirmation prints.
//
// The claim "this takes effect the next time the daemon starts" is the one an operator is most
// likely to assume backwards, because they are reading the page over a connection that does not
// change when the setting does. So the test asserts both halves at once: the document moved, and
// the transport serving this process did not.
func TestTheRunningTransportIsNotMovedByASave(t *testing.T) {
	f := newSettingsFixture(t)
	f.setTLS("auto", "", "")
	if f.s.TLSMode != TLSAuto {
		t.Fatalf("the fixture's resolved transport is %q; this test is about a running auto server",
			f.s.TLSMode)
	}
	before := f.s.tlsConfig()
	if before == nil {
		t.Fatal("a server on auto resolved to no transport at all")
	}

	rec := authPost(t, f.s, "/settings/tls", "tls=off&certFile=&keyFile=")
	if rec.Code != http.StatusOK {
		t.Fatalf("tls=off answered %d:\n%s", rec.Code, rec.Body.String())
	}
	if got := f.settings(); got.TLS != "off" {
		t.Fatalf("the document did not move to off: %q", got.TLS)
	}
	if f.s.TLSMode != TLSAuto || f.s.tlsConfig() == nil {
		t.Error("saving tls=off changed the transport this process is serving on; the confirmation " +
			"page says it waits for a restart, and a page that lies about that is worse than no form")
	}
	if before.MinVersion != f.s.tlsConfig().MinVersion {
		t.Error("the running transport policy changed under a request handler")
	}
	if !strings.Contains(rec.Body.String(), "next time the daemon starts") {
		t.Errorf("the confirmation does not say the change waits for a restart.\n%s", rec.Body.String())
	}
}

// TestTheFormAcceptsExactlyTheModesTheDocumentAccepts is the two-directional version of
// TestTLSModesMatchConfig, which only asks whether config accepts what this package implements.
//
// Both directions matter here. A mode config accepts and the form refuses is a setting the UI
// cannot reach; a mode the form accepts and config refuses is a save that Validate would have
// rejected, which would mean the form is the weaker gate. The empty string is deliberately out:
// the document resolves an absent key to auto, while the form refuses an absent field, and that
// asymmetry is the point of tlsModeWord rather than an oversight.
func TestTheFormAcceptsExactlyTheModesTheDocumentAccepts(t *testing.T) {
	for _, raw := range []string{"auto", "off", "custom", "Auto", "AUTO", "acme", "https",
		"generate", "none", "tls", "on", " true", "custom "} {
		mode, refusal := tlsModeWord(strings.TrimSpace(raw))
		cfg := pageConfig()
		cfg.Settings.TLS = mode
		// Filled paths, so the equivalence below is about the mode and not about config's separate
		// rule that "custom" needs somewhere to read the certificate from.
		cfg.Settings.CertFile, cfg.Settings.KeyFile = "/etc/plock/cert.pem", "/etc/plock/key.pem"
		docOK := cfg.Validate() == nil
		if refusal == "" && !docOK {
			t.Errorf("the form accepts %q, which the document then refuses to save: the form is the "+
				"weaker gate", mode)
		}
		if refusal != "" && strings.TrimSpace(raw) != "" {
			probe := pageConfig()
			probe.Settings.TLS = strings.TrimSpace(raw)
			if probe.Validate() == nil {
				t.Errorf("the form refuses %q, which the document accepts: a mode the UI cannot set",
					strings.TrimSpace(raw))
			}
		}
	}
}

// TestLoadCustomRefusesANonFileWithoutReadingIt pins the two guards in readCertFile directly, so
// the reason the form's refusal is generic is checked where the reading happens.
func TestLoadCustomRefusesANonFileWithoutReadingIt(t *testing.T) {
	dir := t.TempDir()
	_, key := validPair(t, dir)
	at := time.Now()

	big := filepath.Join(dir, "big.crt")
	fh, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := fh.Truncate(maxCertFileBytes + 1); err != nil {
		t.Fatal(err)
	}
	fh.Close()

	for _, tc := range []struct{ name, cert, want string }{
		{"a directory", dir, "not a regular file"},
		{"too big", big, "larger than"},
	} {
		start := time.Now()
		_, err := LoadCustom(tc.cert, key, func() time.Time { return at })
		if err == nil {
			t.Errorf("%s: LoadCustom accepted %s as a certificate", tc.name, tc.cert)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: the error does not say %q: %v", tc.name, tc.want, err)
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("%s: took %s; the guard is supposed to refuse before reading", tc.name, elapsed)
		}
	}
}

// TestLoadCustomRefusesADeviceWithoutExhaustingTheProcess is the /dev/zero case, run against the
// function rather than the page so the failure names the read and not the handler.
func TestLoadCustomRefusesADeviceWithoutExhaustingTheProcess(t *testing.T) {
	if _, err := os.Stat("/dev/zero"); err != nil {
		t.Skip("/dev/zero is not on this platform")
	}
	dir := t.TempDir()
	validPair(t, dir)

	done := make(chan error, 1)
	go func() {
		_, err := LoadCustom("/dev/zero", "/dev/zero", nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("LoadCustom loaded /dev/zero as a certificate")
		}
		if !strings.Contains(err.Error(), "not a regular file") {
			t.Errorf("the refusal does not name the shape: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Error("LoadCustom is still reading /dev/zero after 10s. An unbounded read of a path " +
			"typed into a form is a daemon that allocates until the runtime kills it.")
	}
	if _, err := LoadCustom(filepath.Join(dir, "tls", certFileName),
		filepath.Join(dir, "tls", keyFileName), nil); err != nil {
		t.Errorf("the real pair no longer loads after a refused read of a device: %v", err)
	}
}

// TestTheTransportFormHasNothingTheCSPWouldBlock is the page-side half of #99's criteria. The
// generic guard already enumerates every template; this one names the form specifically, because
// the thing most likely to appear in a new form is a script that shows the path boxes only when
// "custom" is selected.
func TestTheTransportFormHasNothingTheCSPWouldBlock(t *testing.T) {
	f := newSettingsFixture(t)
	body := authGet(t, f.s, "/settings").Body.String()
	start := strings.Index(body, `action="/settings/tls"`)
	if start < 0 {
		t.Fatal("the page has no transport form")
	}
	end := strings.Index(body[start:], "</form>")
	form := body[start : start+end]
	for _, banned := range []string{"<script", "onclick", "onchange", "style="} {
		if strings.Contains(form, banned) {
			t.Errorf("the transport form contains %q, which the page's CSP blocks; the disclosure it "+
				"is trying to implement has to work without it", banned)
		}
	}
	if !strings.Contains(form, `name="csrf"`) {
		t.Error("the transport form carries no CSRF token")
	}
}

// postWithoutToken posts a real session's cookie with a token the server never minted.
func postWithoutToken(t *testing.T, f *settingsFixture, c *http.Cookie, path, form string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", path, strings.NewReader(form+"&csrf=not-a-token"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(c)
	req.RemoteAddr = "192.0.2.71:1"
	f.s.Handler().ServeHTTP(rec, req)
	return rec
}
