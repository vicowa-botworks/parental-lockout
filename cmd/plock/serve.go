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

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
	"github.com/vicowa-botworks/parental-lockout/internal/auth"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/enforce"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/web"
)

// serveUsage is what `plock serve -h` prints.
const serveUsage = `usage: plock serve [--config PATH]

Run the daemon: the enforcement loop and the admin UI.

  --config PATH     the config file (default /etc/plock/config.json)
  --state-dir DIR   where the self-signed certificate is kept
                    (default /var/lib/plock)
  --audit-path PATH where the audit trail is written
                    (default /var/log/plock/audit.jsonl)

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
// serveRunner is how the daemon reaches the outside world, behind a var so a test
// can hand it a loginctl that fails.
//
// The guard below decides whether this machine can lock anybody out by asking the
// backends, and the backends ask a subprocess. Without the seam the only way to test
// the refusal is to install a broken loginctl on the host, which produces a test that
// passes here and asserts nothing on the next machine.
var serveRunner platform.Runner = platform.ExecRunner{}

func runServe(stdout, stderr io.Writer, args []string) int {
	return runServeContext(context.Background(), stdout, stderr, args)
}

func runServeContext(ctx context.Context, stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(stderr, serveUsage) }
	path := fs.String("config", defaultConfigPath, "config file")
	stateDir := fs.String("state-dir", "/var/lib/plock", "directory for the certificate and state")
	auditPath := fs.String("audit-path", audit.DefaultPath, "where the audit trail is written")
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

	// The one line that is always printed, on the stream the operator redirects.
	//
	// It exists for two reasons. The first is the one this package has already been
	// bitten by: a process that starts and a process that does not look identical in
	// a log that records only exceptions, so "did the daemon even come up" was
	// answerable only by watching a screen stay locked. The second is narrower and
	// the reason the config path is in the text: --config pointing at the wrong file
	// is the one mistake that reads as success, because the wrong document still
	// loads, still validates, and still gets written back. Naming the file that was
	// opened turns that into a line a reader catches on the first start.
	//
	// It goes before the backend probe deliberately. A start that later refuses is
	// still a start that was attempted, and the path is the thing being questioned in
	// that case too.
	fmt.Fprintf(stdout, "plock serve: starting, config %s\n", *path)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	chain, chainErr := platform.New(cfg.Settings, serveRunner)
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
	// The probe answers two questions and both are asked here, from one pass.
	//
	// The first is the report: which backends answered. The second is the guard, and
	// the reason they are the same loop is that the guard needs exactly the predicate
	// the report computes. Counting chain.Names() instead -- which is what this did
	// -- asks a different question: Names() lists backends the registry could
	// *construct*, and a constructor only fails on a nil runner or an unknown name.
	// So the guard fired on a typo in lockBackends and stayed silent on the machine
	// where loginctl exists and exits 1, which is the machine the guard exists for and
	// the machine every CI runner is. The daemon came up, announced that enforcement
	// was running, served the admin UI, and could lock nobody.
	//
	// Refusing here also happens before the audit trail is opened and before the
	// listener is bound, so a refusal leaves nothing behind to unwind.
	usable := 0
	if chain != nil {
		for _, st := range chain.Probe(ctx) {
			if st.Available {
				usable++
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
	// The trail is one writer shared by both halves of the daemon. Two writers on one
	// path would rotate against each other, and the engine's events and the admin's
	// mutations have to interleave in one file for the trail to answer "what happened
	// after the parent changed the rule".
	trail := &audit.Writer{Path: *auditPath, Logf: logf(stdout)}
	if err := trail.Open(); err != nil {
		// Refusing to start is the only honest option. A daemon that serves the UI
		// and locks nobody is bad enough; one that also cannot say it did anything
		// is unreviewable.
		fmt.Fprintf(stderr, "plock serve: cannot open the audit trail at %s: %v\n", *auditPath, err)
		return 1
	}
	defer func() {
		if err := trail.Close(); err != nil {
			fmt.Fprintf(stderr, "plock serve: closing the audit trail: %v\n", err)
		}
	}()

	srv.Auth = adminAuthenticator(cfg)
	srv.Audit = func(event, detail string) {
		if err := trail.Write(audit.Record{Kind: event, Detail: detail, Actor: "admin"}); err != nil {
			// The mutation already happened; failing the request over the bookkeeping
			// would tell the parent their change did not take when it did.
			fmt.Fprintf(stderr, "plock serve: audit write for %s: %v\n", event, err)
		}
	}

	// The condition is "nothing answered the probe", not "no backend was configured"
	// and not "the chain is nil". platform.New always returns a Chain, so a nil check
	// here could never fire; and an empty Names() list is a config typo, which is the
	// least dangerous of the three because the operator gets an error line naming the
	// unknown backend. What has to be refused is a machine where every configured
	// backend is present and broken: that is the one that looks healthy from outside.
	//
	// A partial chain stays allowed, and that is what the per-backend lines above are
	// for -- refusing to start on a machine with no GNOME would make the product
	// unusable on the one it is for.
	if usable == 0 {
		fmt.Fprintf(stderr, "plock serve: no usable lock backend, so there is nothing to enforce; refusing to start\n")
		return 1
	}
	engine := enforce.New(*path, chain, auditSink{w: trail})
	engine.Logf = logf(stdout)

	runners := []func(context.Context) error{
		srv.Serve,
		engine.Run,
	}

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

// logf sends a daemon diagnostic to stdout. It is a function rather than an inline
// closure because the web server, the engine, and the audit writer each take a hook
// of this shape, and all three must land on the same stream or a reader cannot put
// the lines back into order.
func logf(w io.Writer) func(string, ...any) {
	return func(format string, args ...any) { fmt.Fprintf(w, format+"\n", args...) }
}

// auditSink adapts the engine's Sink to the audit writer.
//
// The mapping is field for field -- Record is the engine's Event plus the two a
// config mutation needs -- with one exception worth stating: Emit must not block,
// because it runs inside the tick and a stalled tick holds the overlap guard and
// stops the daemon re-locking anyone. The writer's own open is guarded by its mutex
// and its writes are appends, so the only way this blocks is a filesystem that
// blocks, which is a machine-level failure no in-process timeout would survive
// either.
type auditSink struct{ w *audit.Writer }

func (a auditSink) Emit(e enforce.Event) {
	if err := a.w.Write(audit.Record{
		At:        e.At,
		Kind:      e.Kind,
		Username:  e.Username,
		SessionID: e.SessionID,
		RuleID:    e.RuleID,
		RuleName:  e.RuleName,
		Detail:    e.Detail,
	}); err != nil {
		// The writer has already reported this through its own Logf and counted it
		// as dropped. Nothing the tick could do with the error would help the child
		// who is not locked out, so it is not turned into a tick failure.
		_ = err
	}
}
