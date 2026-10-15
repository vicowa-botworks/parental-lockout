package web

import (
	"context"
	"crypto/tls"
	"errors"
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

	// The port is chosen by the kernel, so the caller has to wait for the bind rather
	// than guess it. WaitReady is what that wait is for; this helper used to poll Addr()
	// 500 times, which is the shape of caller the field's comment promised nobody would
	// have to write.
	wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
	defer wcancel()
	if err := s.WaitReady(wctx); err != nil {
		cancel()
		t.Fatalf("Serve failed to bind: %v", err)
	}
	if s.Addr() == "" {
		cancel()
		t.Fatal("WaitReady returned no error and Addr is still empty")
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

// waitAddr blocks until Serve has bound and returns the address it bound to.
//
// Four tests used to carry their own 200-iteration poll of Addr() for this. The poll was
// not merely noisy: it reported "Serve never bound" when the real answer was a bind error
// the test threw away, so a misconfigured listen address and a server that had not started
// yet looked the same. WaitReady carries the error, and says so.
func waitAddr(t *testing.T, s *Server, ctx context.Context) string {
	t.Helper()
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := s.WaitReady(wctx); err != nil {
		t.Fatalf("Serve failed to bind: %v", err)
	}
	a := s.Addr()
	if a == "" {
		t.Fatal("WaitReady reported no bind error and Addr() is still empty")
	}
	return a
}

// The contract the field's comment advertised, tested. `started` was declared with "so a
// caller can wait for the port to be real rather than sleeping" and nothing ever closed
// it, so a caller that believed the comment blocked forever.
//
// The failure half is the one that matters. A channel closed only on success turns every
// misconfigured address into a hang, and a hang in a daemon's startup path looks exactly
// like a daemon that is still starting.
func TestWaitReadyReleasesItsCallerWhenTheBindFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod %s: %v", dir, err)
	}
	// Port 1 on loopback is reserved and never bindable by an unprivileged process, so
	// this is a real bind failure rather than a mocked one.
	s, err := New(Options{
		Listen:   "127.0.0.1:1",
		TLSMode:  TLSOff,
		StoreDir: dir,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Serve is what binds, so it has to be running for the waiter to be released.
	serveErr := make(chan error, 1)
	go func() { serveErr <- s.Serve(ctx) }()

	if err := s.WaitReady(ctx); err == nil {
		t.Fatal("WaitReady reported no error for a bind that cannot have succeeded")
	} else if errors.Is(err, ctx.Err()) || errors.Is(err, context.DeadlineExceeded) {
		// Without this half the test passes when `started` is never closed at all: the
		// caller gives up after 5s, gets a deadline error, and a test named for the bind
		// reports that a bind error arrived.
		t.Fatalf("WaitReady timed out rather than reporting the bind failure: %v", err)
	} else if !strings.Contains(err.Error(), "listen") {
		t.Errorf("WaitReady reported %v, which is not the bind failure", err)
	}
	if a := s.Addr(); a != "" {
		t.Errorf("the bind failed and Addr() still reports %q", a)
	}
	// Serve must return the same failure rather than blocking or swallowing it.
	select {
	case err := <-serveErr:
		if err == nil {
			t.Error("Serve returned no error although the bind failed")
		}
	case <-time.After(5 * time.Second):
		t.Error("Serve did not return after its bind failed")
	}
}

// The success half, and the reason the poll in startRealServer is gone rather than merely
// moved: WaitReady has to answer promptly, not eventually.
func TestWaitReadyAnswersOnceThePortIsReal(t *testing.T) {
	s, _, _ := startRealServer(t, TLSOff)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.WaitReady(ctx); err != nil {
		t.Fatalf("WaitReady on a server that is serving: %v", err)
	}
	if s.Addr() == "" {
		t.Error("WaitReady returned and Addr() is still empty, so the two disagree about the bind")
	}
}

// The third answer WaitReady can give: nobody bound anything while we waited.
//
// The bind-failure test above cannot reach this branch -- a failed bind still closes the
// channel, so the select takes the first case. Without a test for the timeout path,
// `return ctx.Err()` can become `return nil` and every caller keeps believing the port is
// live: a daemon that never bound would report itself ready to whatever supervises it.
func TestWaitReadyReportsTheWaitItGaveUpOn(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod %s: %v", dir, err)
	}
	s, err := New(Options{Listen: "127.0.0.1:0", TLSMode: TLSOff, StoreDir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Serve is deliberately never called, so the bind signal never arrives.
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	err = s.WaitReady(ctx)
	if err == nil {
		t.Fatal("WaitReady reported success for a server that was never asked to bind")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("WaitReady reported %v, want the deadline it waited on", err)
	}
}
