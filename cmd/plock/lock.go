package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/vicowa-botworks/parental-lockout/internal/cli"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/lockop"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

const lockUsage = `usage: plock lock <user> [--all] [--json] [--config PATH]

Locks <user>'s desktop session now, ignoring the schedule. Nothing is written to the
config: this is one action, not a change of policy, so the account returns to whatever
its schedule says the moment the daemon's next tick runs.

--all locks every managed user who has a lockable session, and is what you want after
changing a backend, when the question is "does locking work at all here".

A machine can answer in three ways, and they are three different sentences:
  locked        a backend confirmed the screen is locked
  unconfirmed   the lock was asked for and nothing could prove it took
  failed        nothing reached the desktop at all
`

// lockOutcome is one session's result. It is a type rather than an error so the three
// shapes stay distinguishable in --json: collapsing them means a script cannot tell "the
// screen is locked and the desktop cannot prove it" from "no backend exists".
//
// It is the JSON face of lockop.Outcome. The field names are this command's contract with
// scripts and must not drift; the classification behind them is lockop's.
type lockOutcome struct {
	Session  string `json:"session"`
	Username string `json:"username"`
	Locked   bool   `json:"locked"`
	// Unconfirmed is the raw-VT and Plasma-ignores-the-signal case: the ask went through,
	// the exit status was zero, and nothing on the machine can attest that the screen is
	// dark. It is not a failure and it is not a success.
	Unconfirmed bool   `json:"unconfirmed,omitempty"`
	Detail      string `json:"detail,omitempty"`
}

// lockChain is the seam `lock` drives, so the command can be tested against a chain that
// answers each of the three ways without a desktop.
type lockChain interface {
	Sessions(context.Context) ([]platform.Session, error)
	Lock(context.Context, platform.Session) error
}

// lockSelfLocation is the same read the daemon and doctor make, behind its own seam so a
// test can state whose session the operator is typing in. The answer comes from /proc, and a
// test that inherits it passes on a desktop and means something else on a runner.
var lockSelfLocation func() platform.SelfLocation = selfLocation

var buildLockChain = func(s config.Settings) (lockChain, error) {
	return platform.New(s, platform.ExecRunner{})
}

func runLock(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("lock", flag.ContinueOnError)
	var flagOut bytes.Buffer
	fs.SetOutput(&flagOut)
	path := fs.String("config", defaultConfigPath, "path to the config document")
	all := fs.Bool("all", false, "lock every managed user with a lockable session")
	asJSON := fs.Bool("json", false, "machine-readable output on stdout")
	fs.Usage = func() { flagOut.WriteString(lockUsage) }

	// See cli.TakeFirst: the username comes first because that is the shape a human
	// copies, and Go's flag parser would otherwise stop before reaching --all.
	target, flagArgs, hasTarget := cli.TakeFirst(fs, args)
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if werr := writeAll(stdout, flagOut.String()); werr != nil {
				return cli.Failure
			}
			return cli.OK
		}
		fmt.Fprintf(stderr, "plock lock: %v\n%s", err, flagOut.String())
		return cli.Usage
	}
	// --all is consulted before the username is demanded: `plock lock --all` is the whole
	// command, and asking for a name after it was already given is the complaint that makes
	// someone think --all was not a real flag.
	if extra := fs.Args(); len(extra) > 0 {
		fmt.Fprintf(stderr, "plock lock: unexpected argument %q", extra[0])
		if target != "" {
			fmt.Fprintf(stderr, " after username %q", target)
		}
		fmt.Fprint(stderr, "\n\n")
		writeAll(stderr, lockUsage)
		return cli.Usage
	}
	if target == "" && !*all {
		fmt.Fprint(stderr, "plock lock: a username is required (or --all)\n\n")
		writeAll(stderr, lockUsage)
		return cli.Usage
	}
	if hasTarget && *all {
		// Naming someone and saying "everyone" is two different questions, and answering
		// both silently would lock accounts the operator did not name.
		fmt.Fprint(stderr, "plock lock: name one user or pass --all, not both\n")
		return cli.Usage
	}

	cfg, err := config.Load(*path)
	if err != nil {
		return opFail(stderr, *asJSON, "plock lock", codeConfigUnreadable, err)
	}

	chain, chainErr := buildLockChain(cfg.Settings)
	if chainErr != nil {
		// A chain that is missing a backend still locks with the ones it has, which is what
		// doctor and status do for the same reason. The warning goes to stderr rather than
		// into the report so --json stays exactly one document.
		fmt.Fprintf(stderr, "plock lock: warning: %v\n", chainErr)
	}
	if chain == nil {
		return opFail(stderr, *asJSON, "plock lock", codeNoBackend,
			errors.New("no lock backend could be built"))
	}

	sessions, err := chain.Sessions(context.Background())
	if err != nil {
		// A listing that came back short is used as far as it reaches (#168). Whoever types
		// this command is asking about one account, and a census that read four sessions out
		// of five still names it — refusing here was refusing about a session sitting in the
		// same slice. The warning goes to stderr so `--json` stays exactly one document, and
		// the exit status stays the lock's rather than the census's. Only a read with nothing
		// in it ends the command at this point.
		fmt.Fprintf(stderr, "plock lock: warning: %v\n", err)
		if len(sessions) == 0 {
			return opFail(stderr, *asJSON, "plock lock", codeSessionsFailed, err)
		}
	}

	want := sessions
	if !*all {
		want = lockop.OwnedBy(sessions, target)
		// An unknown username is refused rather than reported as "no sessions", because
		// "no sessions" is also what a child who is logged out looks like, and the
		// operator needs to know which of the two they are looking at.
		if len(want) == 0 {
			if _, ok := cfg.UserFor(target); !ok {
				return opFail(stderr, *asJSON, "plock lock", codeNoSuchUser,
					fmt.Errorf("no managed user %q", target))
			}
			// The same rule one step further along: a census that lost a row cannot
			// establish that nobody is logged in (#168). "has no session on this machine"
			// about the one session the listing could not read is the reassuring answer,
			// and it is not this command's to give — the operator has to be told the
			// machine was only partly seen.
			if err != nil {
				return opFail(stderr, *asJSON, "plock lock", codeSessionsFailed,
					fmt.Errorf("%q may hold a session the listing could not read: %w", target, err))
			}
			return opFail(stderr, *asJSON, "plock lock", codeNoLockableSession,
				fmt.Errorf("%q has no session on this machine", target))
		}
	}

	// --all is documented as a smoke test, and it is the command someone types from the
	// desktop they are about to lose. The daemon declines its own login session (#67) and the
	// CLI deliberately does not: a one-shot lock is not a lockout, because the operator types
	// their password and is back (docs/security.md §4). So this says so and carries on.
	// Refusing would break the smoke test for the person who means it; saying nothing is how
	// someone discovers the rule by losing their session mid-command.
	if *all {
		if self := lockSelfLocation(); self.InUserSlice {
			for _, s := range want {
				if s.UID == self.UID && platform.IsLockable(s) {
					fmt.Fprintf(stderr, "plock lock: --all includes your own session (%s), so your "+
						"screen is about to go dark; the enforcement loop would decline this one, and "+
						"the difference is that a lock you asked for once is one you can undo\n",
						s.Username)
					break
				}
			}
		}
	}

	// The classification is lockop's, not this file's. `plock lock` and the admin UI's
	// "lock now" button drive the same chain and have to say the same sentence about what
	// came back; a loop restated here would be free to disagree with the page.
	res := lockop.Now(context.Background(), chain, want)
	outcomes := make([]lockOutcome, 0, len(res.Outcomes))
	for _, o := range res.Outcomes {
		outcomes = append(outcomes, lockOutcome{
			Session:     o.Session,
			Username:    o.Username,
			Locked:      o.Locked,
			Unconfirmed: o.Unconfirmed,
			Detail:      o.Detail,
		})
	}
	locked, unconfirmed, refused := res.Locked, res.Unconfirmed, res.Refused

	if *asJSON {
		return writeLockJSON(stdout, stderr, outcomes, locked, unconfirmed, refused, *all)
	}
	return writeLockText(stdout, stderr, outcomes, locked, unconfirmed, refused, target, *all)
}

func writeLockJSON(stdout, stderr io.Writer, outcomes []lockOutcome, locked, unconfirmed, refused int, all bool) int {
	target := "all"
	if !all {
		if len(outcomes) > 0 {
			target = outcomes[0].Username
		} else {
			target = ""
		}
	}
	doc := struct {
		Target      string        `json:"target"`
		Locked      int           `json:"locked"`
		Unconfirmed int           `json:"unconfirmed"`
		Refused     int           `json:"refused"`
		Outcomes    []lockOutcome `json:"sessions"`
	}{Target: target, Locked: locked, Unconfirmed: unconfirmed, Refused: refused, Outcomes: outcomes}
	if err := cli.JSON(stdout, doc); err != nil {
		fmt.Fprintf(stderr, "plock lock: %v\n", err)
		return cli.Failure
	}
	return lockExitCode(locked, unconfirmed, refused)
}

func writeLockText(stdout, stderr io.Writer, outcomes []lockOutcome, locked, unconfirmed, refused int, target string, all bool) int {
	r := &cli.Report{}
	label := target
	if all {
		label = "all users"
	}
	r.Field("target", label)
	r.Field("locked", locked)
	if unconfirmed > 0 {
		r.Field("unconfirmed", unconfirmed)
	}
	if refused > 0 {
		r.Field("not lockable", refused)
	}
	for _, o := range outcomes {
		state := "failed"
		switch {
		case o.Locked:
			state = "locked"
		case o.Unconfirmed:
			state = "unconfirmed"
		}
		if o.Detail != "" {
			r.Line("  %s %s — %s", o.Session, state, o.Detail)
			continue
		}
		r.Line("  %s %s", o.Session, state)
	}
	if unconfirmed > 0 {
		// The sentence this command exists to make available. A zero exit status here
		// would be a lie and a non-zero one would be an alarm; the operator has to be able
		// to tell "the screen is dark and nothing can prove it" apart from "the screen is
		// not dark", and only the wording can do that.
		r.Line("\nunconfirmed means the lock was asked for and no backend could prove the")
		r.Line("screen locked. On a raw virtual terminal this is permanent: nothing on a VT")
		r.Line("ever calls SetLockedHint. Re-run `plock doctor` to see which backend answered.")
	}
	if err := r.Write(stdout); err != nil {
		fmt.Fprintf(stderr, "plock lock: %v\n", err)
		return cli.Failure
	}
	return lockExitCode(locked, unconfirmed, refused)
}

// lockExitCode maps the three outcomes onto the exit codes.
//
// Unconfirmed is NOT a failure. It is the ordinary answer on a raw VT and on a Plasma that
// accepted the signal without reporting it, and making it exit non-zero would put a
// permanent red X on a machine whose screen is in fact locked — which trains the operator
// to ignore the one signal they have. It is a failure to have asked and gotten nothing at
// all, and that is the case that exits 1.
func lockExitCode(locked, unconfirmed, refused int) int {
	if locked > 0 || unconfirmed > 0 {
		return cli.OK
	}
	if refused > 0 {
		// Nothing was lockable. The command did what it could and the answer is "there was
		// nothing to lock", which is not an operational failure.
		return cli.OK
	}
	return cli.Failure
}
