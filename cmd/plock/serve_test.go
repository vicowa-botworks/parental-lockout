package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer a test can read while the daemon is still writing.
//
// The commands under test write from several goroutines (the request log comes from
// the server, the startup lines from the command itself), and a plain
// bytes.Buffer read concurrently is both a race and a test that passes or fails
// depending on how the scheduler felt.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// waitFor polls until the buffer contains want, so a test asserts that a line
// eventually appears rather than that it appeared before an arbitrary sleep.
func waitFor(t *testing.T, b *syncBuffer, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(b.String(), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("did not see %q within %s; output so far:\n%s", want, timeout, b.String())
}

// A config the daemon will accept, written where serve expects to find one.
func writeServeConfig(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "config.json")
	doc := `{
	  "version": 1,
	  "settings": {"listen": "127.0.0.1:0", "tls": "off", "pollSeconds": 30,
	               "enforcement": true, "lockBackends": ["logind"]},
	  "admin": {"username": "parent",
	            "password": "pbkdf2-sha256$100000$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
	  "profiles": [{"id": "p", "name": "P", "timezone": "UTC", "rules": []}],
	  "users": []
	}`
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// startServe runs the daemon against a temp config and returns it once the line
// the test is waiting for appears.
//
// Every test that starts the daemon goes through here so none of them decides the
// daemon is "up" by sleeping.
// startServe returns both streams. The second one matters: the disabled-transport
// warning is deliberately written to stderr, so a test that reads only stdout would
// "prove" the warning works by asserting on a stream it was never written to.
func startServe(t *testing.T, extra ...string) (*syncBuffer, *syncBuffer, context.CancelFunc, <-chan int) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := writeServeConfig(t, dir)
	var out, errOut syncBuffer

	ctx, cancel := context.WithCancel(context.Background())
	code := make(chan int, 1)
	go func() {
		code <- runServeContext(ctx, &out, io.MultiWriter(&errOut), []string{"--config", cfgPath, "--state-dir", dir})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-code:
		case <-time.After(15 * time.Second):
		}
	})
	return &out, &errOut, cancel, code
}

func TestServeRejectsABrokenConfigAtStartup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut syncBuffer
	code := runServeContext(context.Background(), &out, &errOut, []string{"--config", path})
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if errOut.String() == "" {
		t.Error("a broken config produced no message on stderr")
	}
}

// A config that parses but is not usable must fail here too.
//
// The distinction matters because the two look the same from the outside and are
// nothing alike to fix, and because a daemon that starts on an invalid document
// then enforces whatever partial reading it made of it.
func TestServeRejectsAnInvalidConfigAtStartup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	doc := `{"version": 1, "settings": {"listen": "", "tls": "maybe", "pollSeconds": 30}}`
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut syncBuffer
	if code := runServeContext(context.Background(), &out, &errOut, []string{"--config", path}); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestServeUsageErrorIsExitTwo(t *testing.T) {
	var out, errOut syncBuffer
	if code := runServeContext(context.Background(), &out, &errOut, []string{"--nonsense"}); code != 2 {
		t.Errorf("exit code = %d, want 2 (a usage error is not an operational failure)", code)
	}
}

// One runner failing must stop the others.
//
// This is the property the shared context exists for. A web server that keeps
// serving after the enforcement loop died is a dashboard that says everything is
// fine on a machine where nothing is being locked out — which is the failure this
// project treats as worse than a crash, because a crash is at least visible.
func TestOneRunnerFailingCancelsTheOthers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	died := errors.New("the enforcement loop stopped")
	started := make(chan struct{})
	stopped := make(chan struct{})

	err := runAll(ctx, []func(context.Context) error{
		func(context.Context) error { return died },
		func(other context.Context) error {
			close(started)
			<-other.Done()
			close(stopped)
			return other.Err()
		},
	})

	if !errors.Is(err, died) {
		t.Errorf("runAll returned %v, want the runner's own error", err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("the second runner never started")
	}
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("the surviving runner was left running after the other one died")
	}
}

// A shutdown cancels the context every runner is holding.
//
// The web server's own drain is tested in internal/web; what is tested here is the
// part that belongs to the command: that the signal handler's context is the one
// the runners hold, because the enforcement loop is not a socket and will only stop
// when the context says so.
func TestServeStopsWhenTheContextIsCancelled(t *testing.T) {
	out, _, cancel, code := startServe(t)
	waitFor(t, out, "listening on", 10*time.Second)

	cancel()
	select {
	case got := <-code:
		if got != 0 {
			t.Errorf("a clean shutdown returned exit code %d, want 0", got)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("serve did not return after the context was cancelled")
	}
}

// The daemon must not claim it is enforcing when it is not.
//
// This is the one line in serve that a reader would skip, and it is the line that
// decides whether an operator on a half-built daemon knows they are on one.
func TestServeSaysOutLoudThatTheEnforcementLoopIsAbsent(t *testing.T) {
	out, _, _, _ := startServe(t)
	waitFor(t, out, "enforcement loop is not wired", 10*time.Second)
}

// tls: off is a loud warning on every start, not a quiet mode.
//
// It is asserted on the writers the caller passed rather than on the process's streams,
// because the failure mode this guards is the warning going somewhere the operator's
// redirect does not catch it. It is asserted on stderr in particular: stdout is the
// stream a daemon's operator redirects, and a warning that lands there is a warning that
// gets read by nobody.
func TestServeWarnsEveryTimeTLSIsOff(t *testing.T) {
	_, errOut, _, _ := startServe(t)
	waitFor(t, errOut, "TLS IS DISABLED", 10*time.Second)
}

// The other half of the last test. "The warning is on stderr" is only useful if it is not
// also being buried in the stream that gets redirected -- and a change that sent every
// line to both streams would satisfy the test above while proving nothing about where the
// operator looks.
func TestTheTLSSWarningIsNotOnStdout(t *testing.T) {
	out, errOut, _, _ := startServe(t)
	waitFor(t, errOut, "TLS IS DISABLED", 10*time.Second)
	// The daemon writes its startup banner to stdout, so by the time the warning has
	// appeared there is something to compare against; an empty stdout would make this
	// pass for the wrong reason.
	if !strings.Contains(out.String(), "plock serve") {
		t.Errorf("stdout is empty, so this test cannot tell a split from a silence: %q", out.String())
	}
	if strings.Contains(out.String(), "TLS IS DISABLED") {
		t.Errorf("the warning is on stdout, the stream an operator redirects away: %q", out.String())
	}

}

// The warning has to arrive through the sink the caller configured, not the default
// logger — New is where it is emitted, so a Logf assigned afterwards would miss it.
//
// The default logger stamps every line with a date, so the stamp's absence is what
// proves the line came through the caller's writer.
func TestTheTLSSWarningReachesTheConfiguredSink(t *testing.T) {
	_, errOut, _, _ := startServe(t)
	waitFor(t, errOut, "TLS IS DISABLED", 10*time.Second)

	for _, line := range strings.Split(errOut.String(), "\n") {
		if !strings.Contains(line, "TLS IS DISABLED") {
			continue
		}
		// log.Printf's prefix is "2006/01/02 15:04:05 ".
		if len(line) > 10 && line[0] == '2' && line[4] == '-' && line[7] == '-' {
			t.Errorf("the warning went to the default logger, so an operator's redirect would miss it: %q", line)
		}
	}
}
