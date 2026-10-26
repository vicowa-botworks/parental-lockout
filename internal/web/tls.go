package web

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// The three values settings.tls accepts, mirroring config.tlsModes. They are
// restated here rather than imported because config validates the string and this
// package acts on it; the test that keeps them in step is TestTLSModesMatchConfig.
const (
	TLSAuto   = "auto"
	TLSOff    = "off"
	TLSCustom = "custom"
)

const (
	// certValidityDays is 825, the number browsers settled on as the maximum
	// lifetime for a certificate they will accept. A self-signed cert the parent
	// has to re-trust every 90 days is a cert they will learn to click through,
	// which is the outcome we are trying to avoid.
	certValidityDays = 825

	// rotateWhenUnder is how much life triggers a re-mint. Thirty days is long
	// enough that a daemon restarted daily re-mints once a year, and short enough
	// that a daemon left running for two years is not serving an expired chain.
	rotateWhenUnder = 30 * 24 * time.Hour

	certFileName = "plock.crt"
	keyFileName  = "plock.key"
)

// CertStore owns the daemon's own certificate.
//
// It is a struct with a Clock rather than a pair of functions because every
// interesting property here is about time — "is this cert still valid", "is it
// near expiry", "did it just get minted" — and a store that reads time.Now
// directly can only be tested by waiting for it.
type CertStore struct {
	// Dir holds plock.crt and plock.key. It is created 0700 if missing.
	Dir string

	// Logf receives the one line per start that says what the server decided. A
	// nil Logf means io.Discard rather than a nil-interface panic: the store is
	// constructed in tests far more often than it is served.
	Logf func(format string, args ...any)

	// Now defaults to time.Now.
	Now func() time.Time

	// Hostnames, when non-empty, replaces interface-derived names. It exists for
	// tests and for an operator who knows the name the LAN actually uses; the
	// daemon's own guess is what runs by default.
	Hostnames []string

	// Interfaces defaults to net.Interfaces. SANs come from the machine's real
	// addresses, and a test cannot arrange to have a particular NIC.
	Interfaces func() ([]net.Interface, error)

	// Addrs defaults to the unicast addresses of Interfaces.
	Addrs func(net.Interface) ([]net.Addr, error)
}

func (s *CertStore) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *CertStore) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
	}
}

// TLSConfig returns the certificate the server should present.
//
// The returned tls.Config pins nothing else: cipher suites and the minimum
// version are set by the caller (Server.TLSConfig) so that one place owns the
// transport policy and this one owns the key material.
func (s *CertStore) TLSConfig() (*tls.Config, error) {
	cert, err := s.LoadOrMint()
	if err != nil {
		return nil, err
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

// LoadOrMint returns the stored certificate, minting or refreshing it as needed.
//
// Three cases, in this order:
//
//  1. A usable cert on disk is reused byte for byte. Re-minting on every start
//     would change the fingerprint the parent pinned, so they would have to
//     re-trust the certificate each time the daemon restarted — which is how a
//     warning gets trained into a reflex they ignore.
//  2. A cert that is expired, unparseable, or inside rotateWhenUnder of its
//     expiry is replaced.
//  3. No cert at all is minted.
//
// A cert that exists but cannot be read is replaced rather than fatal, with the
// reason logged: the alternative is a daemon that will not start because of a
// corrupt file, and a daemon that will not start does not lock anybody out.
func (s *CertStore) LoadOrMint() (tls.Certificate, error) {
	if s.Dir == "" {
		return tls.Certificate{}, errors.New("certificate store needs a directory")
	}
	if err := ensurePrivateDir(s.Dir); err != nil {
		return tls.Certificate{}, err
	}

	certPath := filepath.Join(s.Dir, certFileName)
	keyPath := filepath.Join(s.Dir, keyFileName)

	existing, err := tls.LoadX509KeyPair(certPath, keyPath)
	switch {
	case err == nil:
		need, _, verifyErr := s.needReplace(existing)
		if !need {
			s.logf("web: using the certificate at %s (expires %s)", certPath,
				notAfter(&existing).Format(time.RFC3339))
			return existing, nil
		}
		// verifyErr is the loud case: the pair is not merely old, it is wrong, and
		// "wrong" is what an operator needs to see before they conclude the daemon
		// is silently broken.
		if verifyErr != nil {
			s.logf("web: %s is unusable (%v); replacing it", certPath, verifyErr)
		} else {
			s.logf("web: %s expires %s, inside the %s rotation window; replacing it",
				certPath, notAfter(&existing).Format(time.RFC3339), rotateWhenUnder)
		}
	case os.IsNotExist(err):
		// Nothing to say; case 3 is the ordinary first start.
	default:
		// LoadX509KeyPair reports a parse failure as a plain error, so this is the
		// corrupt-file branch. Fall through and overwrite.
		s.logf("web: could not read the certificate pair in %s (%v); replacing it", s.Dir, err)
	}

	cert, err := s.mint(certPath, keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	s.logf("web: listening on https and serving a self-signed certificate at %s. "+
		"The browser will warn once; trusting this file stops it: %s", certPath, certPath)
	return cert, nil
}

// needReplace decides whether a loaded pair should be regenerated.
//
// The bool-only return would be enough to drive the caller, but not enough to
// write a log line an operator can act on, so the reason travels too.
func (s *CertStore) needReplace(cert tls.Certificate) (bool, string, error) {
	leaf := cert.Leaf
	if leaf == nil {
		// tls.LoadX509KeyPair does not populate Leaf on older paths; parse it from
		// the DER we already hold rather than treating it as a failure.
		parsed, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return true, "the certificate cannot be parsed", err
		}
		leaf = parsed
		cert.Leaf = leaf
	}

	now := s.now()
	if now.Before(leaf.NotBefore) {
		return true, "the certificate is not valid yet", fmt.Errorf("not valid before %s (clock skew?)", leaf.NotBefore.Format(time.RFC3339))
	}
	if !now.Before(leaf.NotAfter) {
		return true, "the certificate has expired", fmt.Errorf("expired %s", leaf.NotAfter.Format(time.RFC3339))
	}
	if remaining := leaf.NotAfter.Sub(now); remaining < rotateWhenUnder {
		return true, "inside the rotation window", nil
	}
	// A key that does not match the certificate is rejected by tls.X509KeyPair on
	// load, so there is nothing left to check here: the pair either loads or it
	// does not.
	return false, "", nil
}

func notAfter(c *tls.Certificate) time.Time {
	if c.Leaf != nil {
		return c.Leaf.NotAfter
	}
	if len(c.Certificate) == 0 {
		return time.Time{}
	}
	if leaf, err := x509.ParseCertificate(c.Certificate[0]); err == nil {
		return leaf.NotAfter
	}
	return time.Time{}
}

// mint writes a fresh key and self-signed certificate, then loads them back.
//
// Loading the written files rather than returning what was built in memory is
// deliberate: it means the object the server serves is the one on disk, so a
// mismatch between the two (a truncated write, a mode that made the key
// unreadable) fails at startup instead of on the first TLS handshake.
func (s *CertStore) mint(certPath, keyPath string) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate the certificate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate a serial number: %w", err)
	}

	hosts, ips, err := s.subjectAltNames()
	if err != nil {
		return tls.Certificate{}, err
	}

	names := append([]string(nil), hosts...)
	if len(names) == 0 {
		names = []string{"localhost"}
	}
	cn := names[0]

	now := s.now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     now.Add(certValidityDays * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		// IsCA is on because this cert is its own issuer, and browsers only accept
		// a self-signed leaf that can sign itself. BasicConstraintsValid is what
		// makes the flag mean anything: without it the constraint is ignored.
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              hosts,
		IPAddresses:           ips,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create the certificate: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("encode the certificate key: %w", err)
	}

	// The key goes down first. A cert with no key is a startup failure the next
	// time round (case 3 re-mints); a key with no cert is inert.
	if err := writeFilePrivate(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return tls.Certificate{}, fmt.Errorf("write %s: %w", keyPath, err)
	}
	if err := writeFilePrivate(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		return tls.Certificate{}, fmt.Errorf("write %s: %w", certPath, err)
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("read back the certificate just written: %w", err)
	}
	if _, err := x509.ParseCertificate(cert.Certificate[0]); err != nil {
		return tls.Certificate{}, fmt.Errorf("the certificate just written does not parse: %w", err)
	}
	return cert, nil
}

// subjectAltNames are the names a browser might present this machine under.
//
// Loopback is included because `https://localhost:8443` on the machine itself is
// how the parent usually reaches it. The machine's real hostname is what the LAN
// uses. A name that is not a legal DNS label is dropped rather than written: one
// malformed SAN makes the whole certificate rejected, so a machine whose hostname
// is `build-7.local.` or an empty string must still get a working cert.
func (s *CertStore) subjectAltNames() (hosts []string, ips []net.IP, err error) {
	seenHost := map[string]bool{}
	addHost := func(h string) {
		h = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
		if h == "" || !isDNSName(h) || seenHost[h] {
			return
		}
		seenHost[h] = true
		hosts = append(hosts, h)
	}

	addHost("localhost")
	if s.Hostnames != nil {
		for _, h := range s.Hostnames {
			addHost(h)
		}
	} else if hn, err := os.Hostname(); err == nil {
		addHost(hn)
		// The short form is what most LAN resolvers actually answer to.
		if i := strings.IndexByte(hn, '.'); i > 0 {
			addHost(hn[:i])
		}
	}

	seenIP := map[string]bool{}
	addIP := func(ip net.IP) {
		if ip == nil || ip.IsUnspecified() || seenIP[ip.String()] {
			return
		}
		seenIP[ip.String()] = true
		ips = append(ips, ip)
	}
	// Loopback first: it is the one address that is always there and always
	// correct, so a failure below never costs the operator the local UI.
	addIP(net.IPv4(127, 0, 0, 1))
	addIP(net.IPv6loopback)

	ifs, err := s.ifaces()
	if err != nil {
		// A machine whose interfaces cannot be listed still gets a loopback-only
		// cert rather than no server at all. The log line says so, because the
		// operator will otherwise wonder why the LAN address is missing.
		s.logf("web: could not list network interfaces (%v); the certificate will only cover localhost", err)
		return hosts, ips, nil
	}
	for _, ifi := range ifs {
		if ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := s.addrs(ifi)
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ip, ok := addrIP(a)
			if !ok || !ip.IsGlobalUnicast() {
				// Link-local and multicast are not names anybody dials, and an IPv6
				// link-local in a SAN is at best useless.
				continue
			}
			addIP(ip)
		}
	}
	sort.Strings(hosts)
	return hosts, ips, nil
}

func (s *CertStore) ifaces() ([]net.Interface, error) {
	if s.Interfaces != nil {
		return s.Interfaces()
	}
	return net.Interfaces()
}

func (s *CertStore) addrs(ifi net.Interface) ([]net.Addr, error) {
	if s.Addrs != nil {
		return s.Addrs(ifi)
	}
	return ifi.Addrs()
}

// addrIP turns an interface address into a bare IP, dropping the CIDR mask and
// the IPv6 zone.
func addrIP(a net.Addr) (net.IP, bool) {
	switch v := a.(type) {
	case *net.IPNet:
		return v.IP, true
	case *net.IPAddr:
		return v.IP, true
	}
	return nil, false
}

// isDNSName reports whether s is a single label or a dotted name that x509 will
// accept, without importing a validator that disagrees with the encoder.
//
// Underscores are rejected on purpose: Go's asn1 encoder will happily write them
// and Safari will reject the certificate for it.
func isDNSName(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			alnum := c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
			if !alnum && c != '-' {
				return false
			}
			// A label may not begin or end with a hyphen (RFC 952).
			if c == '-' && (i == 0 || i == len(label)-1) {
				return false
			}
		}
	}
	return true
}

// maxCertFileBytes is the ceiling on a file this package will read as a certificate or a key.
//
// A certificate, its chain, and an EC key are a few kilobytes; a megabyte is already two
// orders of magnitude past a real pair. The ceiling is not tidiness: the path arrives from a
// configuration file and, since #99's TLS form, from a web request, and an unbounded read of
// whatever it names is a daemon that never finishes starting. Measured on this box before the
// ceiling existed, `tls.LoadX509KeyPair("/dev/zero", "/dev/zero")` does not return — it
// allocates until the runtime kills the process with
// `runtime: out of memory: cannot allocate 8388608-byte block`.
const maxCertFileBytes = 1 << 20

// readCertFile reads a certificate or key file the way the rest of this daemon reads a path
// it was handed: open first, then ask the descriptor what it opened.
//
// Two shapes have to be refused before any bytes are read, and each was measured rather than
// reasoned about:
//
//   - A named pipe with no writer parks a plain open in the kernel forever — LoadCustom on a
//     FIFO was still parked after 10 s with a heap of 0 MB. O_NONBLOCK makes that open return
//     instead of blocking, which is the same reason ensurePrivateDir passes it (#879); the
//     shape check below is what then refuses the pipe. On Windows the constant is 0 and the
//     open can still block, which this package accepts for the reason dir_other.go gives: the
//     daemon only ever runs on Linux.
//   - The mode is read from the held descriptor, not from the path. A stat on the path
//     describes a file that may already have been swapped for something else by the time the
//     bytes are read (#880), and the size limit is only honest about the object it measured.
//
// The limit is applied twice on purpose: once to the size the directory entry claims, which
// refuses a large file before reading it, and once to what was actually read, which refuses
// something that grew while it was being read.
func readCertFile(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is %s, not a regular file", path, shapeOf(info.Mode()))
	}
	if info.Size() > maxCertFileBytes {
		return nil, fmt.Errorf("%s is %d bytes, and a certificate file larger than %d is not a certificate file",
			path, info.Size(), maxCertFileBytes)
	}
	buf, err := io.ReadAll(io.LimitReader(f, maxCertFileBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(buf)) > maxCertFileBytes {
		return nil, fmt.Errorf("%s grew past %d bytes while it was being read; a certificate file does not do that",
			path, maxCertFileBytes)
	}
	return buf, nil
}

// shapeOf names what a path actually is, for the refusal above. The operator has to be able
// to tell "you pointed me at a directory" from "you pointed me at a socket" without guessing
// which one a mode string means.
//
// Nouns only, because the caller owns the clause: readCertFile appends ", not a regular file",
// so a case here that repeats it prints the same eight words twice. That is how a device reads
// as "a device, not a file, not a regular file" — a sentence nobody wrote and no test would
// have caught, because the assertion only looked for the wrapper's half.
//
// The character-device case has to come before the block one. Go defines ModeCharDevice as
// ModeDevice OR its own bit, so every character device matches both tests, and whichever is
// written first is the only one that can ever run. Measured here: /dev/zero stats as
// Dcrw-rw-rw- with ModeDevice and ModeCharDevice both set.
func shapeOf(mode os.FileMode) string {
	switch {
	case mode.IsDir():
		return "a directory"
	case mode&os.ModeNamedPipe != 0:
		return "a named pipe with nobody writing to it"
	case mode&os.ModeSocket != 0:
		return "a socket"
	case mode&os.ModeCharDevice != 0:
		return "a character device"
	case mode&os.ModeDevice != 0:
		return "a device"
	default:
		// Not a directory, pipe, socket or device, and not a regular file either — the mode
		// bits carry something this switch has no noun for. Naming it honestly is the point:
		// the old default repeated the caller's clause and read as a stutter.
		return "something the mode bits do not name"
	}
}

// LoadCustom reads an operator-supplied pair and refuses to start on anything the
// first handshake would trip over.
//
// Failing here rather than on the first request is the whole point: a daemon that
// starts, reports healthy, and then cannot serve a page looks like a network
// problem, and the operator debugs the wrong thing.
//
// It is also the check the settings form runs before it saves, so that a path typed into the
// page cannot produce a document the next start refuses to boot on. That is why the read
// goes through readCertFile rather than tls.LoadX509KeyPair: the form calls this same
// function, so what the page accepts is what the daemon does, and the two cannot drift into
// agreeing about a pair that hangs one of them.
func LoadCustom(certFile, keyFile string, now func() time.Time) (tls.Certificate, error) {
	if certFile == "" || keyFile == "" {
		return tls.Certificate{}, errors.New("tls is \"custom\" but certFile or keyFile is empty")
	}
	certPEM, err := readCertFile(certFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load %s and %s: %w", certFile, keyFile, err)
	}
	keyPEM, err := readCertFile(keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load %s and %s: %w", certFile, keyFile, err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load %s and %s: %w", certFile, keyFile, err)
	}
	leaf := cert.Leaf
	if leaf == nil {
		leaf, err = x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("%s does not contain a parseable certificate: %w", certFile, err)
		}
		cert.Leaf = leaf
	}
	at := time.Now()
	if now != nil {
		at = now()
	}
	if at.Before(leaf.NotBefore) {
		return tls.Certificate{}, fmt.Errorf("%s is not valid before %s; the clock is wrong or the certificate is early", certFile, leaf.NotBefore.Format(time.RFC3339))
	}
	if !at.Before(leaf.NotAfter) {
		return tls.Certificate{}, fmt.Errorf("%s expired on %s", certFile, leaf.NotAfter.Format(time.RFC3339))
	}
	return cert, nil
}

// writeFilePrivate writes bytes with the given mode, atomically.
//
// The temp file is created with O_EXCL in the same directory and renamed over the
// target, so a reader never sees a half-written certificate. The mode is applied
// with Fchmod on the descriptor we opened, not chmod on the path: a root daemon
// that chmods a path it did not open is an arbitrary-chmod primitive the moment a
// symlink appears in that directory.
func writeFilePrivate(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".plock-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	committed = true
	return nil
}
