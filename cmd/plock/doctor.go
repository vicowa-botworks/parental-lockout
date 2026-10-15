package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// defaultConfigPath is where the daemon keeps its document. It is spelled here rather
// than imported because internal/config does not own a default today — Load takes a
// path — and inventing the constant in two places is how the two drift. Ticket #8
// (plock init) is where the path becomes a single exported fact; this is the seam it
// replaces.
const defaultConfigPath = "/etc/plock/config.json"

const doctorUsage = `usage: plock doctor [--json] [--try <user>] [--config PATH]

Reports, for each configured lock backend: whether it is usable on this machine, why
not if it is not, and which one a lock attempt would be offered to first. Also checks
that the process is root, that the config is readable, writable and 0600, that the poll
interval is sane, that the TLS certificate is valid, that the daemon is running, and --
the check that answers "it is not working" -- whether any managed user is inside a
lockout window with a session that is not locked.

Reads only. Without --try it never locks a screen and never writes the config.

  --json         the report as one JSON document on stdout; a failure is reported as
                 JSON on stderr and stdout stays empty
  --try <user>   actually lock that user and report which backend won. This is the one
                 flag that changes the machine, so it cannot be combined with --json:
                 a script must never be able to lock a screen as a side effect of
                 asking a question
  --config PATH  config file (default /etc/plock/config.json)

Exits 0 when nothing failed, 1 when a check failed -- and --json names which in its
"failed" array.
`

// chainView is what doctor needs from the chain: the configured order, the per-backend
// verdicts, and the names that resolved to nothing.
//
// An interface rather than a *platform.Chain so the report can be tested against a
// fixed set of verdicts. Without that seam every assertion about "would try first:
// logind" is a claim about whether the machine running the test happens to have
// loginctl and gdbus installed, which is how a test ends up passing for the wrong
// reason on one box and failing for no reason on another.
// systemdProbe asks the init system about the service. A var so a test can state the
// service's state instead of asserting on whether the machine running the test happens to
// have plock installed -- which is how a diagnostic's own test ends up passing on the
// author's box and failing on every other one.
var systemdProbe = func() (string, bool, error) { return systemdState(platform.ExecRunner{}) }

type chainView interface {
	Names() []string
	Unknown() []string
	Probe(context.Context) []platform.BackendStatus
	// Sessions is here because the unenforced check needs the live session set, and a
	// diagnostic that could not see sessions could not answer the question it exists for.
	Sessions(context.Context) ([]platform.Session, error)
	Lock(context.Context, platform.Session) error
}

// buildChain is the production wiring, behind a var for the same reason.
//
// platform.New always hands back a chain, alongside an error naming whatever it could
// not build, so there is no "no chain" case to branch on here. That is the behaviour
// doctor wants: a config whose only entry is a typo still gets a report — the unknown
// name, and "would try: nothing" — rather than an error line and no information.
var buildChain = func(s config.Settings) (chainView, error) {
	return platform.New(s, platform.ExecRunner{})
}

// runDoctor implements `plock doctor`.
//
// The question it answers is the one an operator has after "the child's screen never
// locked": is the backend even here? Every failure mode this daemon has that is not a
// schedule bug ends in a lock that silently does not happen, so the answer has to be
// legible without reading source, and it has to be legible for the configured order
// rather than the registry's alphabetical one — which backend is tried first is the
// whole of the behaviour.
func runDoctor(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	// flag sends the usage line and the parse error to the same sink, but they belong
	// on different streams: help is an answer (stdout), a bad flag is a complaint
	// (stderr). Capturing the output lets each go where it belongs without losing
	// flag's own message about which argument was wrong.
	var flagOut bytes.Buffer
	fs.SetOutput(&flagOut)
	path := fs.String("config", defaultConfigPath, "path to the config document")
	jsonOut := fs.Bool("json", false, "emit the checks as JSON, with stable field names")
	// --try is the only path in this command that changes anything, and it is opt-in by
	// name: a diagnostic that locked the screen as a side effect of being run could not be
	// run at all on a machine someone is using.
	tryUser := fs.String("try", "", "lock this account's session now and report which backend answered")
	fs.Usage = func() { flagOut.WriteString(doctorUsage) }
	if err := fs.Parse(args); err != nil {
		// flag treats -h as an error, but asking for help is not one. Without this
		// `plock doctor -h` exits 2 like a typo, which is the behaviour the command
		// surface test exists to catch.
		if errors.Is(err, flag.ErrHelp) {
			if werr := writeAll(stdout, flagOut.String()); werr != nil {
				return exitFailure
			}
			return exitOK
		}
		fmt.Fprintf(stderr, "plock doctor: %v\n%s", err, flagOut.String())
		return exitUsage
	}
	if *tryUser != "" && *jsonOut {
		// --try writes to the screen and --json is what a script reads. Letting both run
		// means a monitoring job could lock a child's screen on a schedule, which is not what
		// anyone wiring a health check expects from a command called doctor.
		fmt.Fprintf(stderr, "plock doctor: --try locks a real session and cannot be combined with --json\n")
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "plock doctor: unexpected argument %q\n\n", fs.Arg(0))
		writeAll(stderr, doctorUsage)
		return exitUsage
	}

	// The report is assembled from checks rather than printed as it goes, so that a
	// failure in one check cannot stop the others from running. A config that will not
	// load used to end the command there -- which is the wrong shape for a diagnostic:
	// the operator's very next question ("is the daemon even running?") was unanswered,
	// and the two checks that would have answered it need no config at all.
	rep := &report{GeneratedAt: time.Now()}
	rep.add(checkRoot(currentUID))
	rep.add(checkConfigPath(*path, osStat))

	cfg, cfgErr := config.Load(*path)
	if cfgErr != nil {
		// Reported rather than defaulted: running the rest of the checks against guessed
		// settings would tell the operator the backends are fine when the settings they
		// actually wrote were never read.
		// The error also goes to stderr, naming the file, because `plock doctor 2>&1 | tail`
		// and a script that only reads stderr both need the cause, and because the exit code
		// alone says nothing about which of the checks is the reason.
		fmt.Fprintf(stderr, "plock doctor: %v\n", cfgErr)
		rep.add(failCheck("config.loads", fmt.Sprintf("%v", cfgErr),
			"fix the document; every check below needs to read it first"))
		rep.add(skipCheck("backends", "not checked: the config could not be read"),
			skipCheck("unenforced", "not checked: the config could not be read"))
		return finishDoctor(rep, stdout, stderr, *jsonOut)
	}
	rep.add(okCheck("config.loads", fmt.Sprintf("%s parses as version %d", *path, cfg.Version)))

	rep.add(checkEnforcementSwitch(cfg.Settings.Enforcement))
	rep.add(checkPollInterval(cfg.Settings.PollSeconds))
	rep.add(checkTLS(cfg.Settings.TLS, cfg.Settings.CertFile, time.Now(), certValidity))
	rep.add(checkDaemonRunning(systemdProbe))

	chain, chainErr := buildChain(cfg.Settings)
	if chainErr != nil {
		fmt.Fprintf(stderr, "plock doctor: warning: %v\n", chainErr)
	}

	// The backend verdict becomes one check among several rather than the whole command.
	// Its detail line keeps the summary an operator reads first; the full per-backend table
	// is a note, because "logind usable, kde usable" is the verdict and the reasons belong
	// underneath it.
	statuses := chain.Probe(context.Background())
	first := ""
	for _, st := range statuses {
		if st.Available {
			first = st.Name
			break
		}
	}
	if first == "" {
		rep.add(failCheck("backends",
			"no configured backend can lock this machine, so a lockout period would be enforced by nobody",
			"see the table below; `plock doctor` lists what each backend needs"))
	} else {
		rep.add(okCheck("backends",
			fmt.Sprintf("%d of %d configured backends are usable; a lock is offered to %s first",
				countUsable(statuses), len(statuses), first)))
	}
	// Printed whether or not something is usable: the report body is what people paste
	// into bug reports, and a misspelled backend must be visible there even on a
	// machine that happens to have a working one later in the list.
	if unknown := chain.Unknown(); len(unknown) > 0 {
		rep.add(warnCheck("backends.unknown",
			fmt.Sprintf("settings.lockBackends names %s, which this binary does not recognise",
				strings.Join(unknown, ", ")),
			"fix the spelling, or install the backend that provides it"))
	}

	rep.add(checkUnenforced(time.Now(), unenforcedCandidates(cfg, chain)))

	if *tryUser != "" {
		return runDoctorTry(stdout, stderr, chain, *tryUser, rep)
	}

	if *jsonOut {
		return finishDoctor(rep, stdout, stderr, true)
	}

	// The backend table is printed above the check lines in text mode. It is the detail
	// behind the `backends` check and the part people paste into a bug report, so it goes
	// first rather than being flattened into one line.
	var out bytes.Buffer
	fmt.Fprintf(&out, "lock backends, in the order they are tried (%s)\n\n",
		strings.Join(chain.Names(), " -> "))
	for _, st := range statuses {
		// "unusable" rather than "unavailable" because the line an operator reads
		// first should say what happens: an unusable backend is skipped at lock time.
		verdict := "unusable"
		if st.Available {
			verdict = "usable"
		}
		fmt.Fprintf(&out, "  %-8s %-8s %s\n", st.Name, verdict, st.Detail)
	}

	// Which backend a lock would actually go to is NOT answerable from here, and
	// claiming otherwise would repeat the mistake this command exists to catch.
	// Chain.Lock tries every usable backend and stops at the first one whose lock it
	// can CONFIRM, so the first usable backend is only "the one that gets asked
	// first". Printing "would use: logind" stated a conclusion the chain does not
	// reach: logind is asked first, and whether it finishes depends on a confirmation
	// this command never performs — doctor reads, it does not lock.
	//
	// (The design premise is that on KDE logind's Lock is ignored and the desktop
	// backend is the one that lands. That is recorded in docs/PLAN.md §7 as something
	// Plasma does "historically" and has NOT been measured on a real Plasma session,
	// because measuring it means locking a live desktop. docs/platforms.md §5 carries
	// the recipe. So the honest answer here is the order, not the outcome.)
	//
	// Saying "nothing" out loud is still the point of the command: a machine where no
	// backend is usable otherwise looks exactly like a machine where nothing needed
	// locking.
	fmt.Fprintln(&out)
	if first == "" {
		fmt.Fprintln(&out, "would try: nothing - no configured backend can lock this machine,")
		fmt.Fprintln(&out, "           so a lockout period would be enforced by nobody.")
	} else {
		fmt.Fprintf(&out, "would try first: %s\n", first)
		fmt.Fprintln(&out, "           A backend is finished with only once its lock is confirmed, so")
		fmt.Fprintln(&out, "           the lock that lands may come from a later one in the list.")
	}
	fmt.Fprintln(&out)
	if err := writeAll(stdout, out.String()); err != nil {
		fmt.Fprintf(stderr, "plock doctor: %v\n", err)
		return exitFailure
	}
	return finishDoctor(rep, stdout, stderr, false)
}

// countUsable counts the backends a lock would actually be offered to.
func countUsable(statuses []platform.BackendStatus) int {
	n := 0
	for _, st := range statuses {
		if st.Available {
			n++
		}
	}
	return n
}

// finishDoctor writes the report and turns its verdict into an exit code.
//
// The write error is checked and returned as a failure on purpose. For a command whose only
// job is to be trusted, "reported nothing" must never look like "reported that everything is
// fine" -- a closed pipe, `| head`, or a full disk would otherwise print half a report and
// exit 0.
func finishDoctor(rep *report, stdout, stderr io.Writer, asJSON bool) int {
	var err error
	if asJSON {
		err = rep.writeJSON(stdout)
	} else {
		err = rep.writeTextErr(stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "plock doctor: %v\n", err)
		return exitFailure
	}
	// The ticket's contract: non-zero when something failed, and --json names which.
	if len(rep.Failed) > 0 {
		return exitFailure
	}
	return exitOK
}
