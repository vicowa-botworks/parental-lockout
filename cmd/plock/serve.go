package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/vicowa-botworks/parental-lockout/internal/auth"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/web"
)

// serveUsage is what `plock serve -h` prints.
const serveUsage = `usage: plock serve [--config PATH]

Run the daemon: the enforcement loop and the admin UI.

  --config PATH   the config file (default /etc/plock/config.json)
  --state-dir DIR where the self-signed certificate is kept
                  (default /var/lib/plock)

Signals: SIGINT and SIGTERM both shut the daemon down. Requests in flight get
ten seconds; the enforcement loop stops between ticks and will not start a new
lock request it cannot finish.
`

// serve is the daemon's entry point.
//
// The shape is: one context, two long-running things under it, and one place that
// decides when it is over. A single context is not tidiness — it is what makes
// "stop" mean stop. If the web server and the enforcement loop each held their own,
// a SIGTERM would leave one of them running, and the one still running would be
// locking a screen while systemd counts down to SIGKILL.
func runServe(stdout, stderr io.Writer, args []string) int {
	return runServeContext(context.Background(), stdout, stderr, args)
}

func runServeContext(ctx context.Context, stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(stderr, serveUsage) }
	path := fs.String("config", defaultConfigPath, "config file")
	stateDir := fs.String("state-dir", "/var/lib/plock", "directory for the certificate and state")
	if err := fs.Parse(args); err != nil {
		// flag has already written the complaint, or Usage has written the help.
		// Either way this is a usage error, and exit code 2 is how the shell tells
		// that apart from a failure to do the work.
		return 2
	}

	// The config is read once here and then watched by the engine. Reading it up
	// front is what makes a broken file a startup failure with a line number rather
	// than a daemon that comes up and enforces nothing.
	cfg, err := config.Load(*path)
	if err != nil {
		fmt.Fprintf(stderr, "plock serve: %v\n", err)
		return 1
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, "plock serve: %v\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	chain, chainErr := platform.New(cfg.Settings, platform.ExecRunner{})
	if chainErr != nil {
		// A partial chain is usable: `doctor` exists precisely because some backends
		// are missing on most machines. The error is printed and the daemon starts
		// with what it has, because refusing to start on a machine with no GNOME
		// would make the product unusable on the one it is for.
		fmt.Fprintf(stderr, "plock serve: %v\n", chainErr)
	}

	// Saying which backends answered is the whole value of starting with a partial
	// chain: on a machine where nothing was detected, the daemon comes up, serves
	// the UI, and locks nothing — and without this line the operator has no way to
	// know that until the first night the screen stays unlocked.
	if chain != nil {
		for _, st := range chain.Probe(ctx) {
			if st.Available {
				fmt.Fprintf(stdout, "plock serve: backend %s is available\n", st.Name)
				continue
			}
			fmt.Fprintf(stderr, "plock serve: backend %s is not usable: %s\n", st.Name, st.Detail)
		}
	}

	srv, err := web.New(web.Options{
		Listen:   cfg.Settings.Listen,
		TLSMode:  cfg.Settings.TLS,
		CertFile: cfg.Settings.CertFile,
		KeyFile:  cfg.Settings.KeyFile,
		StoreDir: *stateDir + "/tls",
		// Logf is part of Options rather than a field set afterwards, and that is
		// load-bearing: New is where the transport is resolved, and the "TLS is
		// disabled" warning it emits is the loudest line the daemon writes. Set
		// after the fact, that one line would go to the default logger and the
		// operator's redirect would miss it.
		Logf: func(format string, args ...any) { fmt.Fprintf(stdout, format+"\n", args...) },
		// The disabled-transport warning goes to stderr, everywhere else goes to
		// stdout. A daemon's stdout is routinely redirected to a file nobody reads;
		// stderr is the stream the journal marks and the operator's pager stops on.
		// This is the one line that must not be skippable, and it is the one line
		// whose absence means the admin password is on the LAN in clear text.
		Warn: func(format string, args ...any) { fmt.Fprintf(stderr, format+"\n", args...) },
	})
	if err != nil {
		fmt.Fprintf(stderr, "plock serve: %v\n", err)
		return 1
	}
	srv.Auth = adminAuthenticator(cfg)
	srv.Audit = func(event, detail string) { fmt.Fprintf(stdout, "audit: %s %s\n", event, detail) }

	// The enforcement loop joins `runners` when #7 lands on this branch. Until it
	// does, serve runs the admin UI and nothing else, and says so out loud rather
	// than pretending: a daemon that logs "enforcement running" while no loop
	// exists is the failure mode this whole project treats as the worst one
	// available.
	runners := []func(context.Context) error{
		srv.Serve,
	}
	fmt.Fprintln(stdout, "plock serve: the enforcement loop is not wired into this build; the admin UI is served and nothing is being locked out")

	if err := runAll(ctx, runners); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(stderr, "plock serve: %v\n", err)
		return 1
	}
	return 0
}

// runAll starts every runner and returns when the first one finishes.
//
// Cancelling the shared context on the way out is the point: one runner failing
// must take the others with it. A web server that keeps serving after the
// enforcement loop died is a machine whose parent can look at a dashboard that
// says everything is fine.
func runAll(ctx context.Context, runners []func(context.Context) error) error {
	if len(runners) == 0 {
		return errors.New("nothing to run")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, len(runners))
	for _, r := range runners {
		go func(fn func(context.Context) error) { errs <- fn(ctx) }(r)
	}
	// The first result decides the exit status, and the context is cancelled by the
	// deferred cancel above so the others stop rather than orphaning.
	return <-errs
}

// adminAuthenticator turns the stored admin record into something the web server
// can check a password against.
//
// The verifier is read from the config rather than from a separate credential
// file because there is exactly one source of truth for the daemon's state, and a
// second place a credential lives is a second place for the CLI and the web UI to
// disagree.
//
// An empty verifier is passed through rather than skipped: PasswordAuthenticator
// answers ErrNoCredential for it, which is the one answer that lets the login page
// tell the operator to run `plock init` instead of reporting a wrong password.
func adminAuthenticator(cfg *config.Config) auth.Authenticator {
	return auth.PasswordAuthenticator{
		Username: cfg.Admin.Username,
		Verifier: cfg.Admin.Password,
	}
}
