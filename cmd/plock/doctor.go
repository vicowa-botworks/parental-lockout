package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// defaultConfigPath is where the daemon keeps its document. It is spelled here rather
// than imported because internal/config does not own a default today — Load takes a
// path — and inventing the constant in two places is how the two drift. Ticket #8
// (plock init) is where the path becomes a single exported fact; this is the seam it
// replaces.
const defaultConfigPath = "/etc/plock/config.json"

const doctorUsage = `usage: plock doctor [--config PATH]

Reports, for each configured lock backend: whether it is usable on this machine, why
not if it is not, and which one a lock attempt would be offered to first.

Reads only. It never locks a screen and never writes the config.
`

// chainView is what doctor needs from the chain: the configured order, the per-backend
// verdicts, and the names that resolved to nothing.
//
// An interface rather than a *platform.Chain so the report can be tested against a
// fixed set of verdicts. Without that seam every assertion about "would use: logind"
// is a claim about whether the machine running the test happens to have loginctl and
// gdbus installed, which is how a test ends up passing for the wrong reason on one box
// and failing for no reason on another.
type chainView interface {
	Names() []string
	Unknown() []string
	Probe(context.Context) []platform.BackendStatus
}

// buildChain is the production wiring, behind a var for the same reason.
//
// platform.New always hands back a chain, alongside an error naming whatever it could
// not build, so there is no "no chain" case to branch on here. That is the behaviour
// doctor wants: a config whose only entry is a typo still gets a report — the unknown
// name, and "would use: nothing" — rather than an error line and no information.
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
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "plock doctor: unexpected argument %q\n\n", fs.Arg(0))
		writeAll(stderr, doctorUsage)
		return exitUsage
	}

	cfg, err := config.Load(*path)
	if err != nil {
		// A config that will not load is reported rather than defaulted: running
		// doctor against guessed settings would tell the operator the backends are
		// fine when the settings they actually wrote were never read.
		fmt.Fprintf(stderr, "plock doctor: %v\n", err)
		return exitFailure
	}

	chain, chainErr := buildChain(cfg.Settings)
	if chainErr != nil {
		fmt.Fprintf(stderr, "plock doctor: warning: %v\n", chainErr)
	}

	statuses := chain.Probe(context.Background())

	// The report is built in full and written once, with the error checked. Writing it
	// with a series of Fprintf calls would discard every error: `plock doctor | head`,
	// a closed pipe, or a full disk would then print half a report — or none of it —
	// and exit 0. For a command whose only job is to be trusted, "reported nothing"
	// must never look like "reported that everything is fine".
	var report bytes.Buffer
	fmt.Fprintf(&report, "lock backends, in the order they are tried (%s)\n\n",
		strings.Join(chain.Names(), " -> "))
	for _, st := range statuses {
		// "unusable" rather than "unavailable" because the line an operator reads
		// first should say what happens: an unusable backend is skipped at lock time.
		verdict := "unusable"
		if st.Available {
			verdict = "usable"
		}
		fmt.Fprintf(&report, "  %-8s %-8s %s\n", st.Name, verdict, st.Detail)
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
	first := ""
	for _, st := range statuses {
		if st.Available {
			first = st.Name
			break
		}
	}
	fmt.Fprintln(&report)
	if first == "" {
		fmt.Fprintln(&report, "would try: nothing - no configured backend can lock this machine,")
		fmt.Fprintln(&report, "           so a lockout period would be enforced by nobody.")
	} else {
		fmt.Fprintf(&report, "would try first: %s\n", first)
		fmt.Fprintln(&report, "           A backend is finished with only once its lock is confirmed, so")
		fmt.Fprintln(&report, "           the lock that lands may come from a later one in the list.")
	}
	// Printed whether or not something is usable: the report body is what people paste
	// into bug reports, and a misspelled backend must be visible there even on a
	// machine that happens to have a working one later in the list.
	if unknown := chain.Unknown(); len(unknown) > 0 {
		fmt.Fprintf(&report, "not recognised in this binary: %s\n", strings.Join(unknown, ", "))
	}

	if err := writeAll(stdout, report.String()); err != nil {
		fmt.Fprintf(stderr, "plock doctor: %v\n", err)
		return exitFailure
	}
	if first == "" {
		return exitFailure
	}
	return exitOK
}
