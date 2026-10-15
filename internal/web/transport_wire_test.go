package web

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/auth"
)

// The transport this package exists to land, asserted on the socket.
//
// Every other TLS test here drives s.Handler() through httptest, which never touches the
// listener, and TestTheTransportRefusesAnythingOlderThanTLS12 asserts on the *tls.Config
// struct. Both passed while Serve was handing the raw TCP socket to http.Server -- so 487
// lines of certificate minting, mode checks and rotation were dead at runtime, the daemon
// answered plain HTTP on a port it had just logged as https://, and the "TLS IS DISABLED"
// warning stayed silent because it only fires for TLSOff.
//
// The consequence that makes this a functional bug and not only a security one: the
// session cookie is Secure whenever TLSMode is not off, so a browser drops it on an
// http:// origin and login can never complete.
func TestTheBoundPortSpeaksTLS(t *testing.T) {
	s, logged, _ := startRealServer(t, TLSAuto)
	addr := s.Addr()

	// 1. A real TLS handshake against the bound address succeeds.
	cli := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	resp, err := cli.Get("https://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("https request to the bound port failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("https /healthz = %d, want 200", resp.StatusCode)
	}
	if len(body) == 0 {
		t.Error("/healthz returned an empty body over TLS")
	}

	// 2. A plain HTTP request to the same port is not served. Go's TLS listener answers
	//    it with 400 "Client sent an HTTP request to an HTTPS server" rather than a
	//    connection error, so the assertion is "never a real response", not "never a
	//    reply" -- and the sentence is checked because a 400 from somewhere else would be
	//    a different story. This is the half that was broken, and the half a tls.Config
	//    assertion cannot see.
	plain := &http.Client{Timeout: 3 * time.Second}
	resp2, err := plain.Get("http://" + addr + "/healthz")
	if err != nil {
		// A refused connection is also a correct outcome.
		t.Logf("plain HTTP was refused at the connection level: %v", err)
	} else {
		b, _ := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		if resp2.StatusCode == http.StatusOK {
			t.Errorf("plain HTTP was served on a port the daemon logs as https: %q",
				strings.TrimSpace(string(b)))
		}
		if !strings.Contains(string(b), "HTTPS") {
			t.Errorf("plain HTTP got %d without the TLS-mismatch sentence, so this test cannot tell a refusal from an unrelated failure: %q",
				resp2.StatusCode, strings.TrimSpace(string(b)))
		}
	}

	// 3. The log line and the wire agree.
	if !strings.Contains(joinLines(logged), "https://") {
		t.Errorf("the daemon did not log an https URL, so the line and the wire disagree: %q",
			joinLines(logged))
	}
}

// The control. Without it the test above could be passing because nothing is served at
// all -- which is exactly the shape of the bug it was written to catch.
func TestTheBoundPortSpeaksPlainHTTPWhenTLSIsOff(t *testing.T) {
	s, logged, _ := startRealServer(t, TLSOff)

	plain := &http.Client{Timeout: 5 * time.Second}
	resp, err := plain.Get("http://" + s.Addr() + "/healthz")
	if err != nil {
		t.Fatalf("plain http request failed on a server configured with tls off: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("http /healthz = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(joinLines(logged), "http://") {
		t.Errorf("tls is off and the daemon still did not say what it serves: %q", joinLines(logged))
	}
	if strings.Contains(joinLines(logged), "https://") {
		t.Errorf("tls is off and the daemon logged an https URL: %q", joinLines(logged))
	}
}

// startRealServer starts a real server on a loopback port through New, because New is the
// only path that resolves a transport -- the struct literal the other tests use has no
// certificate to present, so it cannot prove anything about the wire.
func startRealServer(t *testing.T, mode string) (*Server, *logSink, *logSink) {
	t.Helper()
	logged, audited := newLogSink(), newLogSink()
	// t.TempDir() is 0775 on this box, and the certificate store refuses a directory
	// looser than 0700 -- correctly, so the fixture has to make the directory it was
	// promised rather than hope the runner's umask agrees.
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod %s: %v", dir, err)
	}
	s, err := New(Options{
		Listen:   "127.0.0.1:0",
		TLSMode:  mode,
		StoreDir: dir,
		Logf:     logged.log,
	})
	if err != nil {
		t.Fatalf("New(%s): %v", mode, err)
	}
	s.Auth = &stubAuth{username: "parent", password: "correct horse"}
	s.Sessions = auth.NewSessions()
	s.Throttle = &auth.Throttle{}
	s.Audit = audited.event

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()

	// Wait for the bind rather than sleeping: the port is chosen by the kernel, and a
	// test that races it fails on a URL of "https://" with no clue why.
	for i := 0; i < 500 && s.Addr() == ""; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if s.Addr() == "" {
		cancel()
		t.Fatal("Serve never bound")
	}
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("Serve did not return after its context was cancelled")
		}
	})
	return s, logged, audited
}

// joinLines flattens a sink for substring assertions. It is local rather than a method on
// logSink because that helper is shared by every other test in the package and a substring
// check is only ever wanted here.
func joinLines(l *logSink) string { return strings.Join(l.Lines(), "\n") }
