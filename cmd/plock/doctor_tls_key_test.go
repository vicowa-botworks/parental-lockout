package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/web"
)

// writePair mints a self-signed certificate and writes its private key beside it, returning both
// paths.
//
// writeSelfSigned, the helper already in this package, throws the key away — which is exactly the
// half #162 is about: with no key on disk there was nothing for the check to be wrong about, and no
// test could ask. Every row this file adds needs the key to exist as a separate file, because the
// failure being pinned is the two of them not going together.
func writePair(t *testing.T, dir string, notBefore, notAfter time.Time) (string, string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "plock-doctor-test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		DNSNames:     []string{"plock.local"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	derKey, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath,
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: derKey}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

// TestDoctorAndStartUpCannotDisagreeAboutThePair is the property #162 asks for, stated as an
// equality rather than as a list of messages: for every shape of pair on disk, doctor's `tls`
// verdict is OK exactly when web.LoadCustom — the function web.New calls at start-up — accepts the
// same two paths.
//
// Both sides read the same files in the same process, so a row where they differ is a drift between
// a diagnostic and the thing it diagnoses. That drift is the bug this issue was filed for: the
// report said `tls | ok` about a pair the daemon would not boot on, and the operator was left
// debugging a transport that the tool had just certified.
func TestDoctorAndStartUpCannotDisagreeAboutThePair(t *testing.T) {
	now := time.Now().UTC()
	dir := t.TempDir()
	certA, keyA := writePair(t, filepath.Join(dir, "a"), now.Add(-time.Hour), now.Add(30*24*time.Hour))
	certB, keyB := writePair(t, filepath.Join(dir, "b"), now.Add(-time.Hour), now.Add(30*24*time.Hour))
	gone := filepath.Join(dir, "gone", "key.pem")

	for _, tc := range []struct {
		name    string
		cert    string
		key     string
		wantOK  bool
		wantTxt string
	}{
		{name: "a matching pair", cert: certA, key: keyA, wantOK: true},
		{name: "certificate A with key B", cert: certA, key: keyB, wantTxt: "does not match"},
		{name: "certificate B with key A", cert: certB, key: keyA, wantTxt: "does not match"},
		{name: "a key that is not there", cert: certA, key: gone, wantTxt: "no such file"},
		{name: "a certificate where the key should be", cert: certA, key: certA, wantTxt: "private key"},
		{name: "a key that is a directory", cert: certA, key: dir, wantTxt: "not a regular file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, startErr := web.LoadCustom(tc.cert, tc.key, time.Now)
			got := checkTLS("custom", tc.cert, tc.key, now, certValidity, daemonPair)

			if (got.Status == verdictOK) != (startErr == nil) {
				t.Errorf("doctor and start-up disagree about one document: checkTLS gave %q (%s) while "+
					"web.LoadCustom gave err=%v. One of them is answering the operator a different "+
					"question from the one the daemon asks.", got.Status, got.Detail, startErr)
			}
			if tc.wantOK && got.Status != verdictOK {
				t.Errorf("control: a pair the daemon accepts was reported as %q: %s", got.Status, got.Detail)
			}
			// The loader's own sentence has to survive into the report. Doctor is a root-only CLI
			// on the machine itself, so this is the one place the reason is allowed to be said —
			// and the reason is useless if it stayed inside the function that found it.
			if tc.wantTxt != "" && !strings.Contains(got.Detail, tc.wantTxt) {
				t.Errorf("the refusal does not carry the loader's reason %q: %s", tc.wantTxt, got.Detail)
			}
		})
	}
}

// TestTheKeyHalfIsNamedWhenTheTwoFilesAreNotAPair: the sentence an operator reads has to name both
// files. A refusal naming only the certificate sends them to renew a certificate that was never the
// problem — which is the mistake #161's form now refuses to save, and the one this check used to
// answer with `tls | ok`.
func TestTheKeyHalfIsNamedWhenTheTwoFilesAreNotAPair(t *testing.T) {
	now := time.Now().UTC()
	dir := t.TempDir()
	certA, _ := writePair(t, filepath.Join(dir, "a"), now.Add(-time.Hour), now.Add(30*24*time.Hour))
	_, keyB := writePair(t, filepath.Join(dir, "b"), now.Add(-time.Hour), now.Add(30*24*time.Hour))

	got := checkTLS("custom", certA, keyB, now, certValidity, daemonPair)
	if got.Status != verdictFail {
		t.Fatalf("a mismatched pair gave %q: %s", got.Status, got.Detail)
	}
	for _, want := range []string{certA, keyB, "not start"} {
		if !strings.Contains(got.Detail, want) {
			t.Errorf("the refusal does not name %q: %s", want, got.Detail)
		}
	}
	// The fix has to say who is refusing, because the operator's next question is "then why did
	// doctor say ok" — and the answer is that the same code that will not boot is what answered.
	if !strings.Contains(got.Fix, "web.New") {
		t.Errorf("the fix does not name the code that refuses this pair: %s", got.Fix)
	}
}

// TestTheReportHandsTheKeyToTheTLSCheck is the wiring pin. Everything else in this file calls
// checkTLS directly, and #162 was not a wrong check — it was a correct one that was never handed
// the key. A test that stops at the function cannot see a call site that drops an argument, so this
// one drives the command the operator drives and reads the line they read.
func TestTheReportHandsTheKeyToTheTLSCheck(t *testing.T) {
	now := time.Now().UTC()
	dir := t.TempDir()
	certA, keyA := writePair(t, filepath.Join(dir, "a"), now.Add(-time.Hour), now.Add(30*24*time.Hour))
	_, keyB := writePair(t, filepath.Join(dir, "b"), now.Add(-time.Hour), now.Add(30*24*time.Hour))

	run := func(t *testing.T, keyFile string) string {
		t.Helper()
		path := filepath.Join(dir, "config-"+filepath.Base(filepath.Dir(keyFile))+".json")
		body := fmt.Sprintf(`{"version":1,"settings":{"tls":"custom","certFile":%q,"keyFile":%q}}`,
			certA, keyFile)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		// The exit code is not asserted: on a box with no plock service running it is a failure for
		// reasons that have nothing to do with the transport. The line is the claim here.
		var out, errOut strings.Builder
		if code := runDoctor(&out, &errOut, []string{"--config", path}); code < 0 {
			t.Errorf("doctor returned %d, which is not a status it may return", code)
		}
		return out.String()
	}

	// The control first: the matching pair has to reach the same "valid until" sentence it reached
	// before this check grew a second argument, or every failure below could be a fixture nobody
	// can satisfy.
	matching := run(t, keyA)
	if !strings.Contains(matching, "is valid until") {
		t.Errorf("the matching pair did not get the ok sentence:\n%s", matching)
	}
	if strings.Contains(matching, "will not start") {
		t.Errorf("the matching pair was refused:\n%s", matching)
	}

	mismatched := run(t, keyB)
	if !strings.Contains(mismatched, "will not start with") {
		t.Errorf("the report did not refuse certificate A with key B:\n%s", mismatched)
	}
	for _, want := range []string{certA, keyB} {
		if !strings.Contains(mismatched, want) {
			t.Errorf("the refusal does not name %s:\n%s", want, mismatched)
		}
	}
}

// An empty keyFile is the same shape as the empty certFile the check already refuses: the server
// has nothing to serve with. It is refused before either file is touched, so the report cannot be
// produced by half-reading a pair the document never named.
func TestAnEmptyKeyFileIsRefusedBeforeEitherFileIsOpened(t *testing.T) {
	got := checkTLS("custom", "/etc/plock/tls.pem", "   ", time.Now(),
		func(string) (time.Time, time.Time, error) {
			t.Error("the certificate was opened although the key was empty")
			return time.Time{}, time.Time{}, nil
		},
		func(string, string) error {
			t.Error("the pair loader was called although the key was empty")
			return nil
		})
	if got.Status != verdictFail {
		t.Fatalf("an empty settings.keyFile gave %q: %s", got.Status, got.Detail)
	}
	if !strings.Contains(got.Detail, "keyFile") {
		t.Errorf("the refusal does not name settings.keyFile: %s", got.Detail)
	}
	if !strings.Contains(got.Fix, "settings.keyFile") {
		t.Errorf("the fix does not say which setting to point at the key: %s", got.Fix)
	}
}

// "auto" mints its own certificate at start-up and "off" serves none, so neither configured path is
// anyone's business. Asserted because both settings are routinely left empty in those modes: a
// check that asked about the pair anyway would fail every honest "auto" config on the box.
func TestAutoAndOffNeverAskAboutTheConfiguredPaths(t *testing.T) {
	for _, mode := range []string{"auto", "", "off"} {
		got := checkTLS(mode, "", "", time.Now(),
			func(string) (time.Time, time.Time, error) {
				t.Errorf("%q: the certificate was read", mode)
				return time.Time{}, time.Time{}, nil
			},
			func(string, string) error {
				t.Errorf("%q: the pair loader was called", mode)
				return nil
			})
		if mode == "off" {
			if got.Status != verdictWarn {
				t.Errorf("%q: status %q, want the cleartext warning (%s)", mode, got.Status, got.Detail)
			}
			continue
		}
		if got.Status != verdictOK {
			t.Errorf("%q: status %q, want ok (%s)", mode, got.Status, got.Detail)
		}
	}
}
