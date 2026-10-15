package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/cli"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// The negative cases elsewhere in this file prove a check refuses something. These prove it
// accepts the thing it is supposed to accept, which is the half a check can get wrong in the
// direction that makes the operator ignore it.

// writeSelfSigned lays down a real certificate so the reader has something genuine to
// succeed on.
//
// A fixture made of text that is not a certificate only ever exercises the failure path, and
// a check whose failure path is the only path ever taken will pass while refusing every real
// certificate -- which is exactly how CO3 survived: `pem.Decode` pointed at an empty slice
// makes every certificate "hold no PEM block", and every test in the package was already
// asserting that a non-certificate produces that message.
func writeSelfSigned(t *testing.T, dir string, notBefore, notAfter time.Time) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "plock-test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		DNSNames:     []string{"plock.local"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "tls.pem")
	body := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A real certificate has to produce a real window. This is the positive that makes every
// expiry assertion in doctor_more_test.go mean something: without it, "expired" and "not a
// certificate" are the same message and the check could be failing for the wrong reason on
// every input.
func TestCertValidityReadsARealCertificate(t *testing.T) {
	notBefore := time.Now().Add(-48 * time.Hour).UTC().Truncate(time.Second)
	notAfter := time.Now().Add(30 * 24 * time.Hour).UTC().Truncate(time.Second)
	path := writeSelfSigned(t, t.TempDir(), notBefore, notAfter)

	gotBefore, gotAfter, err := certValidity(path)
	if err != nil {
		t.Fatalf("a valid certificate was refused: %v", err)
	}
	if !gotBefore.Equal(notBefore) {
		t.Errorf("notBefore = %v, want %v", gotBefore, notBefore)
	}
	if !gotAfter.Equal(notAfter) {
		t.Errorf("notAfter = %v, want %v", gotAfter, notAfter)
	}
}

// And the whole chain: doctor's tls check pointed at a real file on disk, not at an injected
// loader. The injected loader is a test double the test itself wrote, so it cannot catch the
// real reader returning nothing.
func TestCheckTLSOnARealCertificateFile(t *testing.T) {
	now := time.Now().UTC()
	path := writeSelfSigned(t, t.TempDir(), now.Add(-time.Hour), now.Add(60*24*time.Hour))

	got := checkTLS("custom", path, now, certValidity)
	if got.Status != verdictOK {
		t.Fatalf("a valid certificate on disk was reported as %q: %s", got.Status, got.Detail)
	}
	if !strings.Contains(got.Detail, path) {
		t.Errorf("the report does not name the file it read: %s", got.Detail)
	}

	// The same file, read at a time after it expires, must now fail. Same bytes, different
	// clock: this is the check deciding, not the reader refusing.
	expired := checkTLS("custom", path, now.Add(120*24*time.Hour), certValidity)
	if expired.Status != verdictFail {
		t.Errorf("reading the same certificate 120 days later gave %q: %s", expired.Status, expired.Detail)
	}
	if !strings.Contains(expired.Detail, "expired") {
		t.Errorf("the refusal does not say expired: %s", expired.Detail)
	}
}

// config.loads is the first line of doctor and the one everything else is conditioned on.
// Reported as "skip" on a machine where the config loaded fine, it reads to an operator
// scanning the page as "we did not look", and the whole report loses its foundation.
func TestDoctorReportsConfigLoadsAsOK(t *testing.T) {
	dir := t.TempDir()
	path := seed(t)
	_ = dir
	withChain(t, &fakeChain{names: []string{"logind"},
		statuses: []platform.BackendStatus{{Name: "logind", Available: true}}}, nil)
	withDaemon(t, "active", true, nil)

	code, out, errOut := runCmd(t, "doctor", "--config", path, "--json")
	if code == exitFailure {
		t.Fatalf("doctor failed on a healthy document: %s\n%s", out, errOut)
	}
	got := mustCheck(t, out, "config.loads")
	if got.Status != verdictOK {
		t.Errorf("config.loads on a good document is %q, want ok: %s", got.Status, got.Detail)
	}
	// The detail has to carry the path and the version: this is the line that says which
	// file was actually read, and --config mistakes are common enough that naming it is the
	// whole diagnostic.
	if !strings.Contains(got.Detail, path) {
		t.Errorf("the report does not name the file it loaded: %s", got.Detail)
	}
	if !strings.Contains(got.Detail, "version 1") {
		t.Errorf("the report does not name the version: %s", got.Detail)
	}
}

// A document that does not parse must be a failure, not a skip, and it has to carry a remedy.
func TestDoctorReportsABrokenConfigAsFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{ this is not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	withChain(t, &fakeChain{names: []string{"logind"},
		statuses: []platform.BackendStatus{{Name: "logind", Available: true}}}, nil)
	withDaemon(t, "active", true, nil)

	_, out, _ := runCmd(t, "doctor", "--config", path, "--json")
	got := mustCheck(t, out, "config.loads")
	if got.Status != verdictFail {
		t.Errorf("an unparseable config gave %q, want fail: %s", got.Status, got.Detail)
	}
	if got.Fix == "" {
		t.Error("an unparseable config failed without saying what to do about it")
	}
}

// mustCheck decodes a --json doctor report and returns one check by name, failing the test
// if it is absent. Absent is a failure rather than a nil because a check that stopped being
// emitted is indistinguishable from one that passed, from the outside.
func mustCheck(t *testing.T, out, name string) check {
	t.Helper()
	var rep report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("--json is not decodable: %v\n%s", err, out)
	}
	for _, c := range rep.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("the report has no %q check:\n%s", name, out)
	return check{}
}

// The edit gate. `plock profile add` and `plock user assign` refuse a document that would
// leave the daemon unable to start -- but only the parts they author. Disabling the gate
// entirely (CX5) lets a half-written profile reach the file, and the daemon then refuses the
// whole document on the next tick: every child goes unenforced because one profile was
// misnamed.
func TestManageRefusesAnEditThatWouldBreakTheDocument(t *testing.T) {
	dir := t.TempDir()
	path := seed(t)
	_ = dir
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	// The control: the document as loaded passes its own gate, so the rejection below is
	// about the edit and not about the gate refusing everything.
	if err := cfg.ValidateForEdit(); err != nil {
		t.Fatalf("the unmodified document fails its own edit gate: %v", err)
	}

	// A user pointing at a profile that does not exist. The type accepts it, the UI shows the
	// account as "managed", and the daemon has no schedule to enforce -- so the child is
	// never locked out while every screen says they should be.
	cfg.UpsertUser(config.User{Username: "robin", ProfileID: "no-such-profile", Enabled: true})
	if err := cfg.ValidateForEdit(); err == nil {
		t.Fatal("a user naming a profile that does not exist passed the edit gate; the daemon would refuse the whole document")
	} else if !strings.Contains(err.Error(), "no-such-profile") {
		t.Errorf("the refusal does not name the missing profile: %v", err)
	}

	// And the same defect seen from the other side: a user pointing at a profile that is not
	// there. This is the state that silently stops locking one child out.
	other, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	other.UpsertProfile(config.Profile{
		ID: "trips", Name: "Travel days", Timezone: "UTC",
		Rules: []config.Rule{{ID: "daytrip", Name: "Day trip", Start: "09:00", End: "11:00",
			Days: []string{"sat"}, Enabled: true}},
	})
	if err := other.ValidateForEdit(); err != nil {
		t.Fatalf("a well-formed extra profile was refused: %v", err)
	}
}

// The gate is only worth having if it stops the write. `saveEdited` is the one path every
// `plock profile` and `plock user` mutation goes through, and the assertion that matters is
// not that it printed a complaint but that the file on disk still says what it said before.
//
// Disabling the gate (CX5) is the fail-open this repo keeps tripping over: the command exits
// 0, prints its cheerful confirmation, and the daemon on the next tick refuses a document it
// cannot load -- at which point every child in the file goes unenforced and the operator's
// only clue is a log line.
func TestSaveEditedRefusesAndWritesNothing(t *testing.T) {
	path := seed(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	// The defect: an account pointing at a profile that does not exist.
	cfg.UpsertUser(config.User{Username: "robin", ProfileID: "no-such-profile", Enabled: true})

	var out, errOut bytes.Buffer
	code := saveEdited(cfg, path, &out, &errOut, "user assign", func(r *cli.Report) {
		r.Field("assigned", "robin")
	})
	if code != cli.Failure {
		t.Fatalf("saveEdited reported success for a document the daemon will not load\n%s", out.String())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("the document was modified despite the refusal:\nbefore: %s\n after: %s", before, after)
	}
	// The message has to name the command and say it refused, because this is the moment the
	// operator needs to know the change did not happen.
	if msg := errOut.String(); !strings.Contains(msg, "refusing to write") || !strings.Contains(msg, "user assign") {
		t.Errorf("stderr does not say which command refused: %q", msg)
	}
	// And stdout stays clean: a half-written confirmation on stdout is what a script parses.
	if out.Len() != 0 {
		t.Errorf("stdout carries a confirmation for a refused write: %q", out.String())
	}
}

// The control for the test above: a good edit through the same function does write, so the
// refusal cannot be saveEdited refusing everything.
func TestSaveEditedWritesAGoodEdit(t *testing.T) {
	path := seed(t)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.UpsertUser(config.User{Username: "robin", ProfileID: "school", Enabled: true})

	var out, errOut bytes.Buffer
	if code := saveEdited(cfg, path, &out, &errOut, "user assign", func(r *cli.Report) {
		r.Field("assigned", "robin")
	}); code == cli.Failure {
		t.Fatalf("a valid edit was refused: %s", errOut.String())
	}
	after, err := config.Load(path)
	if err != nil {
		t.Fatalf("the written document does not load: %v", err)
	}
	if _, ok := after.ProfileFor("robin"); !ok {
		t.Errorf("robin is not in the saved document")
	}
	if !strings.Contains(out.String(), "robin") {
		t.Errorf("no confirmation on stdout: %q", out.String())
	}
}

// The split itself has to hold in the other direction too: the edit gate must NOT demand the
// admin credential, because `plock profile add` on a document that has not been initialised
// yet is a legitimate thing to want to do, and `plock init` is the command that owns that
// requirement.
func TestTheEditGateDoesNotDemandTheAdminCredential(t *testing.T) {
	cfg := &config.Config{Version: config.CurrentVersion,
		Settings: config.Settings{PollSeconds: 30, TLS: "auto", Enforcement: true,
			LockBackends: []string{"logind"}, Listen: "127.0.0.1:8443"},
		Profiles: []config.Profile{{ID: "school", Name: "School nights", Timezone: "UTC",
			Rules: []config.Rule{{ID: "week", Name: "Week", Days: []string{"mon"},
				Start: "20:00", End: "06:00", Enabled: true}}}},
	}
	if err := cfg.ValidateForEdit(); err != nil {
		t.Errorf("the edit gate demanded something beyond the edit itself: %v", err)
	}
	// And the full gate still demands it -- otherwise this test would be proving that the
	// credential requirement was simply deleted.
	if err := cfg.Validate(); err == nil {
		t.Error("Validate stopped requiring the admin credential")
	} else if !strings.Contains(err.Error(), "admin") {
		t.Errorf("Validate refused for a reason unrelated to the credential: %v", err)
	}
}
