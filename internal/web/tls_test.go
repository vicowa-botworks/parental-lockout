package web

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// A clock the tests move, and a store pointed at a temp directory.
//
// Every property of a certificate store worth asserting is about time — reused,
// expired, near expiry — so the clock is a field rather than something the tests
// work around.
func newStore(t *testing.T) (*CertStore, *time.Time, *[]string) {
	t.Helper()
	at := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	// A pointer, not the slice: the closure appends after this returns, and a slice
	// value handed back now would keep pointing at the empty backing array.
	var logged []string
	s := &CertStore{
		// A path that does not exist yet. t.TempDir() itself is 0775 here, so
		// pointing the store at it would make the private-directory guard refuse,
		// and the test would be asserting the guard rather than the store.
		Dir: filepath.Join(t.TempDir(), "tls"),
		Now: func() time.Time { return at },
		Logf: func(format string, args ...any) {
			logged = append(logged, strings.TrimSpace(fmt.Sprintf(format, args...)))
		},
		// A fixed interface set, so "the SANs include the LAN address" is a claim
		// about the code rather than about which NIC the machine running the test
		// happens to have.
		Interfaces: func() ([]net.Interface, error) {
			return []net.Interface{
				{Index: 1, Name: "lo", Flags: net.FlagUp | net.FlagLoopback},
				{Index: 2, Name: "enp3s0", Flags: net.FlagUp},
			}, nil
		},
		Addrs: func(ifi net.Interface) ([]net.Addr, error) {
			switch ifi.Name {
			case "lo":
				return []net.Addr{&net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)}}, nil
			case "enp3s0":
				return []net.Addr{
					&net.IPNet{IP: net.IPv4(192, 168, 1, 42), Mask: net.CIDRMask(24, 32)},
					&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
				}, nil
			}
			return nil, nil
		},
	}
	return s, &at, &logged
}

func TestFirstStartMintsACertificate(t *testing.T) {
	s, _, _ := newStore(t)
	cert, err := s.LoadOrMint()
	if err != nil {
		t.Fatal(err)
	}
	leaf := cert.Leaf
	if leaf == nil {
		t.Fatal("the returned certificate has no parsed leaf")
	}

	if leaf.SignatureAlgorithm != x509.ECDSAWithSHA256 {
		t.Errorf("signature algorithm = %v, want ECDSA-SHA256", leaf.SignatureAlgorithm)
	}
	if _, ok := leaf.PublicKey.(*ecdsa.PublicKey); !ok {
		t.Errorf("public key is %T, want an ECDSA key", leaf.PublicKey)
	}
	if got := leaf.NotAfter.Sub(leaf.NotBefore); got < (certValidityDays-1)*24*time.Hour {
		t.Errorf("validity = %v, want about %d days", got, certValidityDays)
	}

	// The key must not be world-readable, and the directory must not be either.
	keyPath := filepath.Join(s.Dir, keyFileName)
	fi, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("key mode = %v, want 0600", fi.Mode().Perm())
	}
	if di, err := os.Stat(s.Dir); err != nil {
		t.Fatal(err)
	} else if di.Mode().Perm()&^0o700 != 0 {
		t.Errorf("directory mode = %v, want no bits above 0700", di.Mode().Perm())
	}
}

// The SANs are what makes the browser warning appear once or on every hostname the
// parent ever types.
func TestTheCertificateCoversLocalhostAndTheLANAddress(t *testing.T) {
	s, _, _ := newStore(t)
	cert, err := s.LoadOrMint()
	if err != nil {
		t.Fatal(err)
	}
	leaf := cert.Leaf

	wantIPs := []string{"127.0.0.1", "::1", "192.168.1.42"}
	got := map[string]bool{}
	for _, ip := range leaf.IPAddresses {
		got[ip.String()] = true
	}
	for _, want := range wantIPs {
		if !got[want] {
			t.Errorf("%s is not in the certificate's IP SANs: %v", want, leaf.IPAddresses)
		}
	}
	// A link-local address is not a name anybody dials, and an IPv6 link-local in a
	// SAN is at best noise.
	for ip := range got {
		if strings.HasPrefix(ip, "fe80") {
			t.Errorf("the link-local address %s should not be in the SANs", ip)
		}
	}
	if len(leaf.DNSNames) == 0 {
		t.Error("the certificate has no DNS names; https://localhost would warn")
	}
	var hasLocalhost bool
	for _, n := range leaf.DNSNames {
		if n == "localhost" {
			hasLocalhost = true
		}
		if strings.Contains(n, "_") || strings.HasPrefix(n, "-") || strings.HasSuffix(n, "-") {
			t.Errorf("%q is not a legal DNS label; Safari rejects a certificate containing it", n)
		}
	}
	if !hasLocalhost {
		t.Errorf("localhost is not in the DNS SANs: %v", leaf.DNSNames)
	}
}

// Re-minting on every start would change the fingerprint the parent pinned, so
// they would re-trust the certificate each time the daemon restarted — which is
// exactly how a warning becomes a reflex they ignore.
func TestSecondStartReusesTheSameCertificate(t *testing.T) {
	s, _, _ := newStore(t)
	first, err := s.LoadOrMint()
	if err != nil {
		t.Fatal(err)
	}
	firstDER := first.Certificate[0]
	firstKey, err := os.ReadFile(filepath.Join(s.Dir, keyFileName))
	if err != nil {
		t.Fatal(err)
	}

	second, err := s.LoadOrMint()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstDER, second.Certificate[0]) {
		t.Fatal("the certificate changed between two starts with the clock unmoved")
	}
	secondKey, err := os.ReadFile(filepath.Join(s.Dir, keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(secondKey) != string(firstKey) {
		t.Error("the private key was rewritten on a start that did not need a new certificate")
	}
}

func TestACertificateNearExpiryIsReplaced(t *testing.T) {
	s, at, logged := newStore(t)
	first, err := s.LoadOrMint()
	if err != nil {
		t.Fatal(err)
	}

	// Park the clock inside the rotation window but not past expiry.
	*at = first.Leaf.NotAfter.Add(-rotateWhenUnder + time.Hour)
	second, err := s.LoadOrMint()
	if err != nil {
		t.Fatal(err)
	}
	if second.Leaf.SerialNumber.Cmp(first.Leaf.SerialNumber) == 0 {
		t.Error("a certificate inside the rotation window was reused")
	}
	if second.Leaf.NotAfter.Before(first.Leaf.NotAfter) {
		t.Errorf("the replacement expires earlier: %v before %v", second.Leaf.NotAfter, first.Leaf.NotAfter)
	}
	if !anyContains(*logged, "rotation window") {
		t.Errorf("nothing said why the certificate was replaced; logged: %q", *logged)
	}
}

func TestAnExpiredCertificateIsReplaced(t *testing.T) {
	s, at, logged := newStore(t)
	first, err := s.LoadOrMint()
	if err != nil {
		t.Fatal(err)
	}
	*at = first.Leaf.NotAfter.Add(time.Minute)

	second, err := s.LoadOrMint()
	if err != nil {
		t.Fatalf("an expired certificate stopped the daemon starting: %v", err)
	}
	if second.Leaf.SerialNumber.Cmp(first.Leaf.SerialNumber) == 0 {
		t.Error("the expired certificate was served again")
	}
	if !anyContains(*logged, "expired") {
		t.Errorf("nothing said the certificate had expired; logged: %q", *logged)
	}
}

// A corrupt file is replaced rather than fatal.
//
// The alternative is a daemon that will not start because of one bad file, and a
// daemon that will not start does not lock anybody out — which is the failure this
// whole product exists to prevent.
func TestACorruptCertificateIsReplacedAndSaysSo(t *testing.T) {
	s, _, logged := newStore(t)
	if _, err := s.LoadOrMint(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir, certFileName), []byte("not a certificate\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cert, err := s.LoadOrMint()
	if err != nil {
		t.Fatalf("a corrupt certificate stopped the daemon starting: %v", err)
	}
	if cert.Leaf == nil {
		t.Fatal("no certificate came back")
	}
	if !anyContains(*logged, "replacing it") {
		t.Errorf("the store replaced the file without saying so; logged: %q", *logged)
	}
}

// A key that does not match the certificate must not be served: the handshake
// would fail on the first request rather than at startup.
func TestAMismatchedKeyPairIsReplaced(t *testing.T) {
	s, _, logged := newStore(t)
	if _, err := s.LoadOrMint(); err != nil {
		t.Fatal(err)
	}
	// Overwrite the key with a different valid EC key.
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalECPrivateKey(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir, keyFileName), pemBlock("EC PRIVATE KEY", der), 0o600); err != nil {
		t.Fatal(err)
	}

	cert, err := s.LoadOrMint()
	if err != nil {
		t.Fatalf("a mismatched pair stopped the daemon starting: %v", err)
	}
	if cert.Leaf == nil {
		t.Fatal("no certificate came back")
	}
	if !anyContains(*logged, "replacing it") {
		t.Errorf("the store did not report the mismatch; logged: %q", *logged)
	}
}

// A directory the operator did not intend is refused rather than used.
func TestAPrivateKeyIsNotStoredInAnOpenDirectory(t *testing.T) {
	s, _, _ := newStore(t)
	dir := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s.Dir = dir

	if _, err := s.LoadOrMint(); err == nil {
		t.Fatal("a private key was written into a 0755 directory")
	} else if !strings.Contains(err.Error(), "more open than 0700") {
		t.Errorf("the refusal does not name the mode: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, keyFileName)); !os.IsNotExist(err) {
		t.Error("the key was written before the directory was checked")
	}
}

func TestASymlinkedCertificateDirectoryIsRefused(t *testing.T) {
	s, _, _ := newStore(t)
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "tls")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("cannot symlink: %v", err)
	}
	s.Dir = link

	if _, err := s.LoadOrMint(); err == nil {
		t.Fatal("the store wrote a private key through a symlink")
	}
}

// tls: custom must fail at startup, not on the first handshake.
func TestCustomTLSRejectsAMissingOrExpiredCertificate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tls")
	at := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	if _, err := LoadCustom(filepath.Join(dir, "nope.crt"), filepath.Join(dir, "nope.key"), func() time.Time { return at }); err == nil {
		t.Error("a missing certificate was accepted")
	}
	if _, err := LoadCustom("", "key", func() time.Time { return at }); err == nil {
		t.Error("an empty certFile was accepted")
	}

	// A real, expired pair.
	store, _, _ := newStore(t)
	store.Dir = dir
	cert, err := store.LoadOrMint()
	if err != nil {
		t.Fatal(err)
	}
	expired := cert.Leaf.NotAfter.Add(time.Hour)
	if _, err := LoadCustom(filepath.Join(dir, certFileName), filepath.Join(dir, keyFileName), func() time.Time { return expired }); err == nil {
		t.Error("an expired certificate was accepted at startup")
	} else if !strings.Contains(err.Error(), "expired") {
		t.Errorf("the refusal does not say expired: %v", err)
	}

	// The same pair, while it is still valid, is accepted.
	// The same pair, while it is still inside its validity, is accepted. NotBefore
	// is five minutes behind the minting clock, so "a little after minting" is the
	// valid point to test rather than a time before it.
	if _, err := LoadCustom(filepath.Join(dir, certFileName), filepath.Join(dir, keyFileName), func() time.Time { return cert.Leaf.NotBefore.Add(time.Minute) }); err != nil {
		t.Errorf("a valid custom pair was refused: %v", err)
	}
}

// The modes in this package and the ones config validates must not drift: a
// settings.tls value that validates and then hits the default branch would be a
// daemon that silently serves the wrong transport.
func TestTLSModesMatchConfig(t *testing.T) {
	for _, mode := range []string{TLSAuto, TLSOff, TLSCustom} {
		cfg := configWithTLS(t, mode)
		if err := cfg.Validate(); err != nil {
			t.Errorf("config rejects %q, which this package implements: %v", mode, err)
		}
	}
}

// The certificate must actually work as a server certificate, not merely parse.
func TestTheMintedCertificateCompletesAHandshake(t *testing.T) {
	s, _, _ := newStore(t)
	tlsConfig, err := s.TLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	type result struct {
		state tls.ConnectionState
		err   error
	}
	dialed := make(chan result, 1)
	go func() {
		conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         "localhost",
		})
		if err != nil {
			dialed <- result{err: err}
			return
		}
		st := conn.ConnectionState()
		conn.Close()
		dialed <- result{state: st}
	}()

	// Accept one connection and complete the handshake. A certificate the stack
	// refuses to serve would surface here rather than in a parse test.
	conn, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	server := tls.Server(conn, tlsConfig)
	if err := server.Handshake(); err != nil {
		t.Fatalf("handshake failed with the minted certificate: %v", err)
	}
	server.Close()

	got := <-dialed
	if got.err != nil {
		t.Fatalf("the client refused the minted certificate: %v", got.err)
	}
	// The client is the side that sees the chain; a server's PeerCertificates is
	// empty unless it asked for a client certificate.
	if len(got.state.PeerCertificates) != 1 {
		t.Fatalf("the client saw %d certificates, want 1", len(got.state.PeerCertificates))
	}
	if !got.state.PeerCertificates[0].IsCA {
		t.Error("the minted certificate is not its own issuer")
	}
}

// configWithTLS builds the smallest document that passes validation with the
// given settings.tls value.
func configWithTLS(t *testing.T, mode string) *config.Config {
	t.Helper()
	c := &config.Config{
		Version: config.CurrentVersion,
		Settings: config.Settings{Listen: "127.0.0.1:8443", TLS: mode, PollSeconds: 30,
			LockBackends: []string{"logind"}},
		Admin: config.Admin{Username: "parent", Password: "pbkdf2-sha256$100000$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
	}
	if mode == TLSCustom {
		c.Settings.CertFile = "/etc/plock/tls.crt"
		c.Settings.KeyFile = "/etc/plock/tls.key"
	}
	return c
}

func pemBlock(typ string, der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der})
}

func anyContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
