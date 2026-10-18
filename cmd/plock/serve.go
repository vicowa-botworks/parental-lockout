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
	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/enforce"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
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

// selfLocation is this process's login-session membership: whose screen would go dark if
// this process locked a session.
//
// It is one function on purpose. The daemon decides whether to decline an account with it,
// and doctor reports the deployment shape that makes the rule bite with it; two callers
// reading the same fact through two implementations is how a diagnostic ends up explaining
// a rule it no longer agrees with.
func selfLocation() platform.SelfLocation {
	uid, inside := platform.ReadSelfUserSlice()
	return platform.SelfLocation{UID: uid, InUserSlice: inside}
}

// serveSelfLocation is how the daemon learns which user slice it is running in, behind
// a var for the same reason serveRunner is one: the answer comes from /proc, so without
// the seam the only way to test the self-session rule is to start the daemon from
// inside a login session, which is true here and false on a headless runner. A test
// that inherits that answer passes for the wrong reason on one of the two.
var serveSelfLocation func() platform.SelfLocation = selfLocation

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
	// The position is part of the guarantee, not a convenience: this is the last point at
	// which the daemon can refuse without leaving something behind. Below it sit web.New,
	// which starts warning about a transport for a daemon that will never serve, and
	// trail.Open(), which creates the audit directory and the file. A refusal that opens a
	// trail leaves a 0-byte audit.jsonl on a machine that never enforced anything -- an
	// artifact in the one place the operator looks to find out what happened. The sentence
	// used to live up by the probe loop and describe a placement the code did not have;
	// TestARefusalLeavesNoAuditTrailBehind is what keeps it true now.
	if usable == 0 {
		fmt.Fprintf(stderr, "plock serve: no usable lock backend, so there is nothing to enforce; refusing to start\n")
		return 1
	}
	// Which login session, if any, this process is inside. Resolved once, here, and
	// handed to both halves of the daemon: the enforcement loop that decides whether to
	// lock, and the dashboard that says what is being locked. One value to both is what
	// keeps them from disagreeing about a child in front of an unlocked screen.
	//
	// It is resolved here rather than inside enforce.New so that a test of the
	// enforcement loop never depends on the kind of host running it, and it is resolved
	// after the no-usable-backend refusal above on purpose: a daemon that refuses to
	// start has declined to lock nobody, and printing this warning there would attach a
	// second, unrelated problem to a clean refusal.
	//
	// It goes on stderr with the other things that are wrong about this start, and it
	// does not stop the daemon: a system unit never trips it, and a hand-started daemon
	// still enforces every other account.
	self := serveSelfLocation()
	if self.InUserSlice {
		fmt.Fprintf(stderr, "plock serve: this process is running inside uid %d's login "+
			"session, so sessions of that uid will NOT be locked: locking the screen this "+
			"daemon runs behind means it can no longer be stopped. Install the unit in "+
			"deploy/plock.service to enforce that account\n", self.UID)
	}
	srv, err := serveServer(cfg, chain, users.Source{}, self, *path, *stateDir, *auditPath, ctx, stdout, stderr)
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

	srv.Audit = func(event, detail string) {
		if err := trail.Write(audit.Record{Kind: event, Detail: detail, Actor: "admin"}); err != nil {
			// The mutation already happened; failing the request over the bookkeeping
			// would tell the parent their change did not take when it did.
			fmt.Fprintf(stderr, "plock serve: audit write for %s: %v\n", event, err)
		}
	}

	engine := serveEngine(*path, chain, trail, self, logf(stdout))

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

// sessionSource is the slice of the chain that needs reading by name rather than by type:
// serve's Lockables seam and doctor's unenforced check both take it, so it is declared once
// here and shared rather than restated per file.
//
// *platform.Chain satisfies it, and a test supplies its own because platform.New builds a
// chain from a config and a Runner and cannot be handed a fake backend: without this, the
// only thing a wiring test could assert about the seam is that it is not nil, and a
// hollowed-out body satisfies that. The same argument holds for doctor's join, which wants
// to state a set of sessions instead of asserting on whether the test machine has loginctl.
type sessionSource interface {
	Sessions(context.Context) ([]platform.Session, error)
}

// sessionLocker is the other half of the chain: the half that puts a screen to sleep.
//
// It is an optional interface rather than a tenth parameter to serveServer for the same
// reason the chain treats a backend's Confirmer as optional. The callers that hand in a
// listing -- doctor's unenforced check, and every test that only wants to state a set of
// sessions -- keep compiling and simply get no lock path, while the one caller that hands
// in the real chain gets the lock path without eight call sites having to remember another
// positional argument next to three adjacent strings.
//
// What it must never grow into is a decision. Capability is what an assertion may answer
// here -- "does this object lock screens at all" -- and not a verdict about whether a lock
// took; that question is answered by calling into the object, which is the distinction
// #1018 was written for.
type sessionLocker interface {
	Lock(ctx context.Context, sess platform.Session) error
}

// serveEngine assembles the enforcement loop the way serveServer assembles the admin
// server, and it exists for one line in the middle of it.
//
// engine.Self is what makes the daemon decline to lock the login session it runs inside
// (#67). Assembled inline, that assignment was invisible to every test: the loop's own
// tests set the field themselves, and the one assembly test looked at the dashboard's copy
// of the same value. Deleting the line left the suite green and reintroduced the exact bug
// #67 was filed for — the daemon locking the account it is running behind, so it can no
// longer be stopped — while the page cheerfully reported that account as declined.
//
// A value the daemon resolves once, at startup, is only observable if the assembly is a
// function a test can call and inspect. That is the whole reason this takes self as an
// argument instead of reading it, and the whole reason it returns the engine.
func serveEngine(cfgPath string, chain platform.Platform, trail *audit.Writer, self platform.SelfLocation, log func(string, ...any)) *enforce.Engine {
	engine := enforce.New(cfgPath, chain, auditSink{w: trail})
	engine.Logf = log
	engine.Self = self
	return engine
}

// serveServer assembles the admin server the way the real daemon does it, so that a test
// can ask the assembled object whether it can see the machine. Wiring the seams anywhere
// else means the only place they are wired is a test fixture, and an unwired seam is a 503
// on the real daemon beside a green suite.
//
// src and sessions are parameters rather than calls made inside the seams for the same
// reason the runner is one: /etc/passwd and the live session table are facts about the
// host, and a wiring test that reads the host can only assert that something came back.
//
// Audit is deliberately not set here: it has to write to the shared audit trail, which this
// function neither creates nor sees, and setting a stdout stub here would quietly win over
// the real hook by ordering.
//
// Auth is not set either, and for a sharper reason. The web layer reads the admin
// credential through Config on every attempt, so a value frozen here would be a second
// credential the daemon consults and the operator did not rotate — which is issue #37
// itself. Server.Auth exists only for a caller with no Config seam at all, and serve always
// has one.
func serveServer(cfg *config.Config, sessions sessionSource, src users.Source, self platform.SelfLocation, cfgPath, stateDir, auditPath string, ctx context.Context, stdout, stderr io.Writer) (*web.Server, error) {
	opts := web.Options{
		Listen:   cfg.Settings.Listen,
		TLSMode:  cfg.Settings.TLS,
		CertFile: cfg.Settings.CertFile,
		KeyFile:  cfg.Settings.KeyFile,
		StoreDir: stateDir + "/tls",
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
		// The three seams the content pages read. Wiring them here is the difference
		// between a page that works and a page that works in tests: the handlers refuse
		// rather than render an empty table, so an unwired seam is a 503 on the real
		// daemon and a green suite. Every one of them re-reads on each call, because a
		// dashboard that shows the state at process start is a dashboard that lies by
		// the afternoon.
		Config: func() (*config.Config, error) { return config.Load(cfgPath) },
		Lockables: func() ([]platform.Session, error) {
			return sessions.Sessions(ctx)
		},
		Accounts: func() (*users.Listing, error) {
			// The whole Listing, notes included. Dropping them here would put the
			// explanation back out of reach on the one path a parent actually uses:
			// "which files did you read, and what did you have to guess" is the answer
			// to a child who is missing from the table.
			return users.List(src)
		},
		// The write half of the Config seam. It re-reads inside the mutation rather than
		// closing over a document captured here: the server holds a mutex around the call,
		// but nothing stops `plock` on the command line from writing the file between two
		// web requests, and a stale document loaded at startup would quietly roll that edit
		// back. Reading last is the only ordering that loses the smaller change.
		//
		// fn's error is returned without saving. A refusal from the handler -- an unknown
		// account, a length outside the set -- must not leave a half-applied document on
		// disk, and "we did not write anything" is the one guarantee the audit line makes.
		Mutate: func(fn func(*config.Config) error) error {
			c, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			if err := fn(c); err != nil {
				return err
			}
			return c.Save(cfgPath)
		},
	}
	// Assigned as a statement rather than as a field in the literal above, and that is
	// load-bearing: a composite literal's fields are aligned by gofmt to its longest
	// key, so a future key longer than Lockables rewrites every other line in it --
	// including the line a mutation pattern is anchored on, which then matches nothing
	// and retires itself while the suite stays green. A statement does not move.
	//
	// Both halves are asserted, and that phrase used to be a claim rather than a fact:
	// TestServeHandsTheDashboardTheSameLocationItHandsTheEngine checked srv.Self only, so
	// deleting the engine's assignment left the suite green (issue #72). It now calls
	// serveEngine as well and compares the two, because the two failure directions are
	// opposite and each one is a lie on its own surface: the dashboard announcing an
	// account as locked out while the engine declines to lock it, or the engine locking the
	// login session it runs inside while the page says that account was declined. Only the
	// dashboard is in front of the parent; only the engine can stop the daemon.
	opts.Self = self

	// The trail is read from the same path the writer in runServe was given, so the page
	// and the file the daemon appends to are the same file: a reader pointed at
	// audit.DefaultPath is not nil, it is confident and wrong, and on a machine started
	// with --audit-path it opens a file nobody writes and reports an empty trail about a
	// full one. Assigned as a statement for the reason above -- and the reader re-opens
	// per request rather than being built once, so a rotation between two page loads is
	// visible on the second one instead of being missed by stale size accounting.
	opts.Trail = &audit.Reader{Path: auditPath, Logf: func(format string, args ...any) {
		fmt.Fprintf(stderr, format+"\n", args...)
	}}

	// The lock path, for the dashboard's "lock now" control, taken from the same object the
	// listing above came from. That identity is the whole point: serve builds one chain from
	// the config's backend list and hands it to the enforcement loop and to this server, so
	// the page cannot be asking a different set of backends than the loop is enforcing with.
	// Two chains would be two answers to "did the screen go down", and the operator would
	// read the wrong one.
	//
	// A statement rather than a field in the literal, for the gofmt reason documented above
	// -- and because it is conditional: a caller that handed in a listing which cannot lock
	// screens leaves the seam nil, and the page then says this daemon has no lock path
	// instead of reporting a lock it never asked for. See sessionLocker.
	if lk, ok := sessions.(sessionLocker); ok {
		opts.Lock = lk.Lock
	}

	srv, err := web.New(opts)
	if err != nil {
		return nil, err
	}
	return srv, nil
}
