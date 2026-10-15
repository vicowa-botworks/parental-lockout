package web

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// The two checks the web mutation battery found missing. Both are routing facts about
// where a line goes -- exactly the kind of fact that passes every test when it is wrong,
// because the line still exists somewhere.

func TestAServerBuiltWithoutAWarnSinkStillSaysTheWarning(t *testing.T) {
	// Options.Warn is optional. A Server assembled without one must still emit the
	// disabled-transport line through Logf rather than drop it, because that line is the
	// only thing standing between an operator and sending a password in the clear.
	logged := newLogSink()
	s, err := New(Options{
		StoreDir: t.TempDir(),
		TLSMode:  TLSOff,
		Listen:   "127.0.0.1:0",
		Logf:     logged.log,
	})
	if err != nil {
		t.Fatal(err)
	}
	s.warnf("TLS IS DISABLED")
	if !strings.Contains(strings.Join(logged.Lines(), "\n"), "TLS IS DISABLED") {
		t.Errorf("with no Warn sink the warning vanished: %q", logged.Lines())
	}
}

func TestTheDisabledTransportWarningGoesToTheWarnSinkAndNowhereElse(t *testing.T) {
	// The whole point of a separate sink is that stdout is the stream an operator redirects
	// away -- `plock serve > /dev/null`, or a unit file that sends stdout to the journal and
	// stderr somewhere they actually look. A warning that lands on the quiet stream is a
	// warning that is not delivered.
	//
	// Both halves are needed. Asserting only "it is in warn" passes when the line is written
	// to both streams; asserting only "it is not in logf" passes when it is dropped from
	// both.
	// Both streams go through the package's synchronized sink: Serve writes from its own
	// goroutine, and a plain bytes.Buffer here is a data race the -race run would report
	// (and did, on an earlier draft of this file).
	warned, logged := newLogSink(), newLogSink()
	s, err := New(Options{
		StoreDir: t.TempDir(),
		TLSMode:  TLSOff,
		Listen:   "127.0.0.1:0",
		Logf:     logged.log,
		Warn:     warned.log,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Serve is what emits the line in production; calling warnf by hand would test the
	// helper rather than the path that an operator actually runs.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		_ = s.Serve(ctx)
	}()

	if !waitFor(warned, "TLS IS DISABLED", 3*time.Second) {
		t.Fatalf("Serve never wrote the warning to the warn sink\nwarn: %q\nlog: %q", warned.Lines(), logged.Lines())
	}
	if !strings.Contains(strings.Join(warned.Lines(), "\n"), "clear text") {
		t.Errorf("the warning does not say what is at risk:\n%q", warned.Lines())
	}
	if got := strings.Join(logged.Lines(), "\n"); strings.Contains(got, "TLS IS DISABLED") {
		t.Errorf("the warning also went to the log sink, so the split is not real:\n%q", logged.Lines())
	}
	cancel()
	<-serveDone
}

func TestABrokenAssetBundleIsRefusedAtStartupNotSilentlyEmpty(t *testing.T) {
	// AssetMap hashes every embedded file to build its content-hashed URLs. If that fails
	// and the error is swallowed, the server starts, every /static/ request 404s, and the
	// operator gets an unstyled admin page with nothing in the log saying why. Refusing to
	// build the server is the only answer that reaches them.
	empty := fstest.MapFS{"app.js": &fstest.MapFile{Data: []byte("")}}
	_, err := New(Options{StoreDir: t.TempDir(), TLSMode: TLSOff, Listen: "127.0.0.1:0", Static: empty})
	if err == nil {
		t.Fatal("a bundle containing an empty asset built a server; every static URL for it is unreachable")
	}
	if !strings.Contains(err.Error(), "app.js") {
		t.Errorf("the error does not name the asset that failed: %v", err)
	}
}

func TestAGoodAssetBundleIsNotRefused(t *testing.T) {
	// The control. Without it the test above passes for any error at all, including one with
	// nothing to do with the bundle.
	good := fstest.MapFS{
		"app.js":  &fstest.MapFile{Data: []byte("console.log(1)")},
		"app.css": &fstest.MapFile{Data: []byte("body{}")},
	}
	if _, err := New(Options{StoreDir: t.TempDir(), TLSMode: TLSOff, Listen: "127.0.0.1:0", Static: good}); err != nil {
		t.Fatalf("a valid bundle was refused: %v", err)
	}
}

// waitFor polls b until it contains want, or the deadline passes.
//
// It waits on the line rather than sleeping, because a fixed sleep passes silently when the
// server returns early -- which is the exact failure this test is trying to catch.
func waitFor(b *logSink, want string, limit time.Duration) bool {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if strings.Contains(strings.Join(b.Lines(), "\n"), want) {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}
