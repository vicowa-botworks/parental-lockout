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
)

// Server is the daemon's admin endpoint.
//
// It is built once at startup and then only read, so the request path takes no
// locks. Everything mutable lives behind the seams it points at (Sessions,
// Throttle), which are safe on their own.
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

	// Now defaults to time.Now.
	Now func() time.Time

	// Static is the asset filesystem. When nil the server serves no assets, which
	// is what the tests that only exercise routing want.
	Static fs.FS

	// csrf holds the per-session token this package owns.
	csrf *csrfStore

	// assets maps a hashed URL to the file behind it, built once because the
	// embedded tree cannot change while the process runs.
	assets map[string]string

	// extraRoutes lets a test put a route on the real router. It exists for the
	// shutdown drain, which needs a handler that is still running at the moment the
	// context is cancelled — and a route that only exists in a test's own handler
	// would not be going through the chain that Serve installs, which is the thing
	// under test.
	extraRoutes func(mux *http.ServeMux)

	mu       sync.Mutex
	srv      *http.Server
	listener net.Listener
	minted   *tls.Config

	// started is closed once the listener is bound, so a caller can wait for the
	// port to be real rather than sleeping.
	started chan struct{}
	startEr error
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
}

// New builds a Server and resolves its transport, failing on anything the first
// request would trip over.
//
// The certificate is loaded here rather than in Serve for that reason: a daemon
// that starts, reports healthy, and then cannot complete a handshake looks like a
// network problem, and the operator spends the evening debugging the wrong thing.
func New(opts Options) (*Server, error) {
	s := &Server{
		Logf:     opts.Logf,
		Listen:   opts.Listen,
		TLSMode:  opts.TLSMode,
		CertFile: opts.CertFile,
		KeyFile:  opts.KeyFile,
		csrf:     newCSRFStore(),
		started:  make(chan struct{}),
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
	if m, err := AssetMap(s.Static); err == nil {
		s.assets = m
	}

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
		s.logf("web: TLS IS DISABLED. The admin password and the session cookie cross the network in clear text. " +
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
	return s, nil
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
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
	if err != nil {
		return err
	}

	httpSrv := &http.Server{
		Handler: s.Handler(),
		// A slow-loris body has no business holding a worker on a home server, and
		// the whole admin UI is a handful of small forms.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		// 1 MiB is far past the largest form this UI posts and small enough that a
		// flood of connections cannot exhaust memory on bodies nobody reads.
		MaxHeaderBytes: 1 << 16,
	}
	s.mu.Lock()
	s.srv = httpSrv
	s.mu.Unlock()

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
	s.logf("web: shutting down, up to 10s for requests in flight")
	drain, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(drain); err != nil {
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
	mux.Handle("GET /{$}", s.requireAuth(http.HandlerFunc(s.handleIndex)))
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

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, "<!doctype html><title>plock</title><p>enforcement daemon</p>\n")
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
