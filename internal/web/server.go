// Package web is the HTTPS server the parent reaches from the LAN.
//
// Two things shape every decision in here. The first is that this server runs as
// root and its only job is to hand out the ability to turn lockouts off, so an
// unauthenticated request is the expected case rather than the exceptional one.
// The second is that its audience is one parent on a home network, not an
// operations team: a security control that makes the tool annoying gets routed
// around, so each control here has to be worth the friction it costs.
package web

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/auth"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// Server is the daemon's admin endpoint.
//
// It is built once at startup and then only read, so the request path takes no
// locks. Everything mutable lives behind the seams it points at (Sessions,
// Throttle), which are safe on their own.
// defaultDrainTimeout is the budget for requests already in flight when shutdown begins.
//
// Ten seconds, and no longer: systemd's default stop timeout is 90, and a daemon that
// spends all of it holding a socket looks hung in the journal.
const defaultDrainTimeout = 10 * time.Second

type Server struct {
	// Auth, Sessions and Throttle are the three pieces from internal/auth. Auth
	// may be nil, which means "no account has been created yet": every protected
	// route then answers with the same page as a wrong password, and the log says
	// what to run. A nil Authenticator is not a programming error, it is the state
	// of a machine before `plock init`.
	// Auth is internal/auth's own interface, not a local lookalike: it already
	// separates "you got it wrong" from "this machine cannot authenticate
	// anybody", which is the distinction the login handler has to make.
	Auth     auth.Authenticator
	Sessions *auth.Sessions
	Throttle *auth.Throttle

	// Listen is the host:port to bind.
	Listen string

	// DrainTimeout is how long shutdown waits for requests already in flight before it
	// gives up and closes them. Zero means defaultDrainTimeout.
	DrainTimeout time.Duration

	// TLSMode is "auto", "off" or "custom". CertFile and KeyFile are the
	// operator-supplied pair for "custom"; Store owns the minted pair for "auto".
	TLSMode  string
	Store    *CertStore
	CertFile string
	KeyFile  string

	// Audit receives security-relevant events (a rejected CSRF token, a login).
	// It is a function rather than an interface because there is exactly one
	// implementation and the tests want to append to a slice.
	Audit func(event string, detail string)

	// Logf is the structured request log. A nil Logf means io.Discard.
	Logf func(format string, args ...any)

	// warn is Options.Warn: the sink for the one line that must survive a stdout
	// redirect. Unexported because it is resolved once, in New, and a caller that
	// reassigns it after startup would be moving the warning out from under the only
	// code path that emits it.
	warn func(format string, args ...any)

	// Now defaults to time.Now.
	Now func() time.Time

	// Static is the asset filesystem. When nil the server serves no assets, which
	// is what the tests that only exercise routing want.
	Static fs.FS

	// Config, Lockables and Accounts are the three things outside this package that
	// the content pages read. They are functions for the reason every other seam
	// here is one: the answers come from the host -- a file, a D-Bus call, a
	// password file -- and a handler that reached for those directly could only be
	// tested on a machine that happens to have them.
	//
	// A nil seam is not a silent empty page. readConfig and friends turn it into an
	// error the page says out loud, because "no accounts" and "I could not look" are
	// different claims and the first one is a reassurance the daemon has not earned.
	//
	// Accounts returns the whole Listing rather than its []Account, because the notes
	// are the half a parent needs. A seam that carried only the rows would render a
	// page where a child is simply absent -- no UID_MIN line, no "declares no home",
	// no "declares no login shell" -- and this package exists so that absence is
	// explained rather than implied.
	Config    func() (*config.Config, error)
	Lockables func() ([]platform.Session, error)
	Accounts  func() (*users.Listing, error)

	// csrf holds the per-session token this package owns.
	csrf *csrfStore

	// assets maps a hashed URL to the file behind it, built once because the
	// embedded tree cannot change while the process runs.
	assets map[string]string

	// extraRoutes puts a route on the real router, and exactly three tests use it — all
	// of them the shutdown drain, which needs a handler that is still running at the
	// moment the context is cancelled.
	//
	// It is unexported on purpose. The alternative, a test standing up its own
	// http.Server around s.Handler(), would not go through the listener wrap, the
	// listener close ordering, or the Shutdown call that Serve installs — and those are
	// the things the drain tests exist to pin. The seam is one field on an unexported
	// path; the alternative is not testing the shutdown path at all.
	extraRoutes func(mux *http.ServeMux)

	mu       sync.Mutex
	listener net.Listener
	minted   *tls.Config

	// started is closed once Serve has attempted to bind — on failure as well as on
	// success, because a caller waiting on a bind that will never happen is the exact
	// deadlock this field's comment used to promise away. startEr carries what it found.
	//
	// startEr is written before close(started) and never again, so a read that happens
	// after observing the close is ordered by the close itself and needs no lock.
	started   chan struct{}
	startEr   error
	startOnce sync.Once
}

// sessionCookie is the one cookie this server sets.
//
// The name says what it holds because a browser's cookie inspector is where an
// operator ends up when they are debugging "it keeps logging me out", and a cookie
// called `sid` tells them nothing about which of the daemon's two tokens they are
// looking at.
const sessionCookie = "plock_session"

// Options configures New.
type Options struct {
	Listen   string
	TLSMode  string
	CertFile string
	KeyFile  string
	StoreDir string

	// Logf is set here rather than on the returned Server because New is where the
	// transport is resolved, and the warning it emits for tls: "off" is the single
	// most important line the daemon writes at startup. A caller that assigned the
	// sink after New would lose exactly that line.
	Logf func(format string, args ...any)

	// Warn is where the disabled-transport warning goes, separately from Logf, because
	// it is the one line the operator must not miss and a daemon's stdout is routinely
	// redirected while its stderr is what the journal flags. Defaulting to Logf keeps
	// every caller that does not care about the split working unchanged.
	Warn func(format string, args ...any)

	// Static is the asset filesystem. Nil means the embedded bundle.
	//
	// It is on Options rather than only on Server because the asset map is built in New,
	// and a check that can only be reached by assembling a Server by hand is a check no
	// caller of the documented constructor can ever trip -- which is also a check no test
	// can write without duplicating the constructor.
	Static fs.FS

	// Config, Lockables and Accounts are the content pages' view of the host. They
	// are on Options for the same reason Static is: a caller that assembles a Server
	// by hand is not using the documented constructor, and a seam only reachable
	// that way is a seam no test of the real path can set.
	Config    func() (*config.Config, error)
	Lockables func() ([]platform.Session, error)
	Accounts  func() (*users.Listing, error)
}

// New builds a Server and resolves its transport, failing on anything the first
// request would trip over.
//
// The certificate is loaded here rather than in Serve for that reason: a daemon
// that starts, reports healthy, and then cannot complete a handshake looks like a
// network problem, and the operator spends the evening debugging the wrong thing.
func New(opts Options) (*Server, error) {
	s := &Server{
		Logf:      opts.Logf,
		warn:      opts.Warn,
		Static:    opts.Static,
		Config:    opts.Config,
		Lockables: opts.Lockables,
		Accounts:  opts.Accounts,
		Listen:    opts.Listen,
		TLSMode:   opts.TLSMode,
		CertFile:  opts.CertFile,
		KeyFile:   opts.KeyFile,
		csrf:      newCSRFStore(),
		started:   make(chan struct{}),
	}
	if s.Listen == "" {
		return nil, errors.New("web: nothing to listen on")
	}
	// Defaulted rather than required. A Server with no session store or no
	// throttle is not a programming error the way a nil listener would be: both
	// have a correct default, and a constructor that panics or errors on a
	// zero-value dependency is one more way to build a daemon that fails at
	// startup rather than at the thing it was asked to do.
	if s.Static == nil {
		s.Static = Assets()
	}
	// A broken asset tree is a startup failure, not a shrug. Swallowing this left
	// s.assets nil, every /static/ request 404ing, and the operator looking at the
	// unstyled page that embedding the assets exists to prevent -- with nothing in the
	// log saying the bundle was never readable.
	m, err := AssetMap(s.Static)
	if err != nil {
		return nil, fmt.Errorf("web: %w", err)
	}
	s.assets = m

	if s.Sessions == nil {
		s.Sessions = auth.NewSessions()
	}
	if s.Throttle == nil {
		s.Throttle = auth.NewThrottle()
	}

	switch s.TLSMode {
	case TLSOff:
		// Loud on every start, by design. A quiet plain-HTTP mode is how a
		// password-ending-up-on-the-LAN configuration survives a security review.
		s.warnf("web: TLS IS DISABLED. The admin password and the session cookie cross the network in clear text. " +
			"This is only defensible on a machine you reach over a localhost port-forward; set settings.tls back to \"auto\" for anything else.")
	case TLSCustom:
		cert, err := LoadCustom(s.CertFile, s.KeyFile, s.now)
		if err != nil {
			return nil, fmt.Errorf("web: %w", err)
		}
		s.minted = &tls.Config{Certificates: []tls.Certificate{cert}}
	case TLSAuto, "":
		s.TLSMode = TLSAuto
		if s.Store == nil {
			if opts.StoreDir == "" {
				return nil, errors.New("web: tls is \"auto\" but no certificate directory was given")
			}
			s.Store = &CertStore{Dir: opts.StoreDir, Logf: s.Logf}
		}
		cfg, err := s.Store.TLSConfig()
		if err != nil {
			return nil, fmt.Errorf("web: %w", err)
		}
		s.minted = cfg
	default:
		// config.Validate rejects anything else, so reaching here means the two
		// lists drifted. Refusing to start is the only safe reading of an unknown
		// transport mode: guessing "off" would silently downgrade.
		return nil, fmt.Errorf("web: unknown tls mode %q (expected auto, off or custom)", s.TLSMode)
	}
	// The pages are parsed here so that a broken template stops the daemon instead of
	// surfacing as a permanent 500 on whichever page a parent happens to open first.
	if err := parsePagesAtStartup(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// warnf writes the one line that must survive a stdout redirect. Falling back to Logf,
// and then to the package logger, means a Server built without Options.Warn still says it.
func (s *Server) warnf(format string, args ...any) {
	if s.warn != nil {
		s.warn(format, args...)
		return
	}
	s.logf(format, args...)
}

func (s *Server) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

func (s *Server) audit(event, detail string) {
	if s.Audit != nil {
		s.Audit(event, detail)
	}
}

// TLSConfigForTests exposes the resolved transport so a test can assert which mode
// won without connecting to it.
func (s *Server) tlsConfig() *tls.Config {
	if s.TLSMode == TLSOff {
		return nil
	}
	// A Server built without going through New has no certificate yet. Defaulting
	// rather than dereferencing nil: the transport policy is this function's to own
	// and it should not depend on which constructor ran.
	base := s.minted
	if base == nil {
		base = &tls.Config{}
	}
	cfg := base.Clone()
	// TLS 1.2 minimum: every browser the parent will use supports 1.3, but a
	// phone's captive-portal-style webview on an old Android is a real client on a
	// home network, and refusing it means the parent cannot unlock from the sofa.
	cfg.MinVersion = tls.VersionTLS12
	// No client certificates: there is exactly one admin and they authenticate
	// with a password. Requiring a client cert would be a second thing to install
	// on every device, which is the definition of a control that gets routed
	// around.
	return cfg
}

// Serve binds, then serves until ctx is cancelled, and drains in-flight requests
// on the way out.
//
// The order matters. The listener is closed first, so a new connection is refused
// while the requests already running get their deadline; closing the listener
// after Shutdown would let a client connect into a server that is no longer
// accepting anything.
func (s *Server) Serve(ctx context.Context) error {
	ln, err := s.listen()
	s.markStarted(err)
	if err != nil {
		return err
	}

	// The transport is put on the wire here, and nowhere else. tls.go mints a
	// certificate, checks the directory and key modes and works out the rotation
	// window; tlsConfig() assembles the policy. None of that means anything until a
	// listener speaks it, and until this line none of it did: Serve handed the raw TCP
	// socket to http.Server, so a daemon configured `auto` -- the default -- answered
	// plain HTTP on a port it had just logged as https://.
	//
	// That is worse than TLS being off, because TLS being off is said out loud. The
	// `TLS IS DISABLED` warning fires only for TLSOff, so under `auto` the wire was
	// identical and the warning never printed: the admin password and a Secure cookie
	// crossing the LAN in clear text under a log line claiming otherwise. And the
	// cookie being Secure made login impossible in a browser, because a browser drops
	// a Secure cookie on an http:// origin -- the infinite /login redirect this file's
	// own comment warns about, reached from the other side.
	//
	// A test on the tls.Config struct could not see any of this, and one existed. The
	// assertion has to be on the socket.
	if cfg := s.tlsConfig(); cfg != nil {
		ln = tls.NewListener(ln, cfg)
	}

	httpSrv := &http.Server{
		Handler: s.Handler(),
		// A slow-loris body has no business holding a worker on a home server, and
		// the whole admin UI is a handful of small forms.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		// 64 KiB. Far past the largest form this UI posts, and small enough that a
		// flood of connections cannot exhaust memory on headers nobody reads. The
		// net/http default is 1 MiB, which is four times more header than a login
		// page can use and four times more to hold per connection.
		MaxHeaderBytes: 1 << 16,
		// net/http's own complaints -- a TLS handshake that failed, a request with a
		// malformed request line, a connection closed mid-body -- go here. Left unset
		// they go to the log package's default logger, which on a daemon means the
		// journal via stderr and on a test means nowhere. On a daemon reachable from
		// the LAN, a burst of failed handshakes is the single most interesting thing
		// this server can report, and it is exactly the thing that would be missing
		// from the configured sink.
		ErrorLog: log.New(&logWriter{fn: s.logf}, "", 0),
	}
	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	// 10 seconds, and no longer: systemd's default stop timeout is 90, and a
	// daemon that spends it all holding a socket looks hung in the journal.
	//
	// DrainTimeout overrides it. It is a real knob, not a test-only one -- an operator
	// who raises systemd's TimeoutStopSec because their handlers do slow config writes
	// should be able to spend more of it draining -- and without it the only way to
	// observe a drain that gives up is to hold a request open for ten seconds.
	drainFor := s.DrainTimeout
	if drainFor <= 0 {
		drainFor = defaultDrainTimeout
	}
	s.logf("web: shutting down, up to %s for requests in flight", drainFor)
	drain, cancel := context.WithTimeout(context.Background(), drainFor)
	defer cancel()
	if err := httpSrv.Shutdown(drain); err != nil {
		// Saying so is the whole behaviour. A drain that gave up means requests were cut
		// off mid-flight, and a daemon that exits 0 after that looks like a clean stop --
		// the operator's next clue would be a half-written config and no explanation.
		s.logf("web: shutdown drained with an error: %v", err)
	}
	return ctx.Err()
}

// listen binds the socket and reports the address in the log.
//
// Binding is separate from serving so Start can return the real port: `:0` is how
// the tests avoid colliding, and a log line that prints the configured string
// rather than the bound address is useless in that case — and in the operator's,
// where an address already in use is the common first failure.
func (s *Server) listen() (net.Listener, error) {
	ln, err := net.Listen("tcp", s.Listen)
	if err != nil {
		return nil, fmt.Errorf("web: listen on %s: %w", s.Listen, err)
	}
	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	scheme := "https"
	if s.TLSMode == TLSOff {
		scheme = "http"
	}
	s.logf("web: listening on %s://%s", scheme, ln.Addr())
	return ln, nil
}

// Addr is the bound address, or "" before Serve has bound.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// startedCh returns the bind signal, creating it on first use.
//
// Lazy rather than left to New: a Server assembled as a struct literal — which every
// in-package test does, and which the zero-value rule here requires to keep working — has
// no channel, and closing a nil channel panics. Making the constructor the only place the
// value exists is the same mistake this codebase has already made three times (a nil Logf
// in enforce.New, a nil clock in Throttle, max == 0 in Sessions).
func (s *Server) startedCh() chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started == nil {
		s.started = make(chan struct{})
	}
	return s.started
}

// markStarted releases whoever is waiting on the bind, exactly once. Serving the same
// Server twice must not close the channel twice, and the first answer is the one they
// were waiting for.
func (s *Server) markStarted(err error) {
	ch := s.startedCh()
	s.startOnce.Do(func() {
		s.startEr = err
		close(ch)
	})
}

// WaitReady blocks until Serve has tried to bind the socket and reports what it found.
//
// A caller that asked for port 0 needs this. Addr() is "" until the socket exists, so
// without it the only way to learn the port is to poll — which is what this package's own
// tests did, 500 times, until the field that advertised a better answer turned out to be
// wired to nothing.
//
// It returns on a bind FAILURE too. A caller that blocks until the port is real and never
// gets one is worse off than the caller who polled.
func (s *Server) WaitReady(ctx context.Context) error {
	ch := s.startedCh()
	select {
	case <-ch:
		return s.startEr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Handler returns the full middleware chain over the router.
//
// Tests drive this with httptest rather than going through a socket, which keeps
// them about routing and headers rather than about TLS.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public, in the order they are matched.
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /login", s.handleLoginPage)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("POST /logout", s.handleLogout)

	if s.extraRoutes != nil {
		s.extraRoutes(mux)
	}

	// Everything below the auth gate.
	mux.Handle("GET /{$}", s.requireAuth(http.HandlerFunc(s.handleDashboard)))
	mux.Handle("GET /users", s.requireAuth(http.HandlerFunc(s.handleUsers)))
	mux.HandleFunc("GET /static/{file}", s.handleStatic)

	chain := s.recoverMiddleware(
		s.requestLog(
			s.securityHeaders(
				s.csrfMiddleware(mux),
			),
		),
	)
	return chain
}

// handleHealthz answers 200 and nothing else.
//
// It is unauthenticated and carries no information, deliberately: the systemd unit
// and any external monitor need a port check, and a page that reports config state
// to an anonymous caller is an inventory list for anybody on the LAN.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok\n")
}

// handleStatic serves the embedded assets with an immutable cache header.
//
// The URL carries a content hash, so "never revalidate" is honest. Without the
// hash in the path this would be a cache header that outlives every fix, which is
// how a CSS change appears not to deploy.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if s.Static == nil {
		http.NotFound(w, r)
		return
	}
	requested := r.PathValue("file")
	if requested == "" || strings.Contains(requested, "..") || strings.ContainsRune(requested, 0) {
		http.NotFound(w, r)
		return
	}
	// The URL carries a content hash, so the only names that resolve are the ones
	// this process computed at startup. An unknown hash is a 404 rather than a
	// fallback to the unhashed name: serving app.css for app.<wrong>.css would make
	// the immutable cache header a way to pin a stale file forever.
	name, ok := s.assets["/static/"+requested]
	if !ok {
		http.NotFound(w, r)
		return
	}
	b, err := fs.ReadFile(s.Static, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType(name))
	// Every name that resolves here is hashed by construction, so the long cache is
	// unconditional rather than conditional on re-deriving the property.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(b)
}

func contentType(name string) string {
	switch {
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".js"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

// logWriter adapts the daemon's Logf to the io.Writer that log.Logger wants, so
// net/http's own diagnostics land in the same sink as everything else this server says.
//
// The trailing newline is dropped rather than passed through: Logf is a format function
// and the logger hands it a line that already ends in one, so forwarding it verbatim
// writes a blank line after every message.
type logWriter struct{ fn func(string, ...any) }

func (w *logWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")
	if msg != "" {
		w.fn("web: http: %s", msg)
	}
	return len(p), nil
}
