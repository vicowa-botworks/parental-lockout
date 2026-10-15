package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/user"
	"strings"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/cli"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

const unlockUsage = `usage: plock unlock <user> --minutes N --reason TEXT [--by NAME] [--config PATH]

Grants <user> temporary freedom from every lockout window until N minutes from now,
replacing any override they already hold. This is the same action the web UI's "unlock"
button performs, on the same field, so the two cannot disagree about what an override is.

--minutes is required and must be between 1 and 1440.
--reason is required and must not be blank.

The account returns to its schedule on its own; there is nothing to undo. Use
` + "`plock override clear <user>`" + ` to end an override early.
`

const overrideUsage = `usage: plock override list [--json] [--config PATH]
       plock override clear <user> [--config PATH]

list shows every override in the config, active or not, with who granted it and why.
clear removes one. An expired override is shown by list and removed by clear, because
"why is this account free" is a question about the file, not only about right now.
`

// maxOverrideMinutes is one day.
//
// The bound is not there to stop a parent being generous, it is there to stop a typo.
// `--minutes 14400` typed for 1440 is ten days of no enforcement, and the config file is
// the enforcement: nothing else will ever revisit the number. A grant longer than a day is
// a change to the profile, and a profile change is reviewed by whoever edits the profile —
// an override is the fast path, and the fast path is where nobody looks.
const maxOverrideMinutes = 1440

func runUnlock(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("unlock", flag.ContinueOnError)
	var flagOut bytes.Buffer
	fs.SetOutput(&flagOut)
	path := fs.String("config", defaultConfigPath, "path to the config document")
	minutes := fs.Int("minutes", 0, "how long the account stays free of lockout, from now")
	reason := fs.String("reason", "", "why the account is being freed (required)")
	by := fs.String("by", "", "who is granting it (defaults to the invoking user)")
	fs.Usage = func() { flagOut.WriteString(unlockUsage) }

	// The username is taken out before parsing. Go's flag package stops at the first
	// non-flag argument, so `plock unlock casey --minutes 30 --reason x` -- the shape this
	// command documents and the shape every human copies from a manual -- would otherwise
	// parse as five positionals and no flags, and report that --minutes was missing.
	name, flagArgs, _ := cli.TakeFirst(fs, args)
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if werr := writeAll(stdout, flagOut.String()); werr != nil {
				return cli.Failure
			}
			return cli.OK
		}
		fmt.Fprintf(stderr, "plock unlock: %v\n%s", err, flagOut.String())
		return cli.Usage
	}
	// Anything still positional after TakeFirst is a second username, or a word that
	// spilled out of an unquoted --reason. Both are refusals, and the second one matters:
	// silently keeping the first word of "film night" would record a different reason than
	// the one typed.
	if extra := fs.Args(); name == "" || len(extra) > 0 {
		switch {
		case name == "" && len(extra) == 0:
			fmt.Fprint(stderr, "plock unlock: a username is required\n\n")
		// There used to be a middle case here, "expected one username, got N", for a name
		// that was never adopted while words were left over. TakeFirst now adopts the
		// first token after `--`, so that combination cannot be produced: every argument
		// shape either yields a name or leaves nothing behind. It is deleted rather than
		// left as a branch that can only be reached by a bug in TakeFirst -- a refusal you
		// cannot reproduce is a refusal you cannot test, and its count was wrong anyway.
		default:
			fmt.Fprintf(stderr, "plock unlock: unexpected argument %q after username %q\n"+
				"(if --reason has a space in it, quote the whole value)\n\n", extra[0], name)
		}
		writeAll(stderr, unlockUsage)
		return cli.Usage
	}

	// --minutes is checked before anything is read. A run that is going to be refused
	// should be refused without touching the file: the operator learns the command was
	// wrong, not that it was wrong after it had already opened the config the daemon is
	// reading.
	if *minutes <= 0 {
		fmt.Fprintf(stderr, "plock unlock: --minutes is required and must be at least 1, got %d\n", *minutes)
		return cli.Usage
	}
	if *minutes > maxOverrideMinutes {
		fmt.Fprintf(stderr, "plock unlock: --minutes %d is more than a day (%d).\n"+
			"An override that long is a change to the profile, not a fast grant — edit the\n"+
			"profile so the change is visible where the schedule is decided.\n",
			*minutes, maxOverrideMinutes)
		return cli.Usage
	}
	if strings.TrimSpace(*reason) == "" {
		// Blank is refused, and so is whitespace, which is what a shell --reason " "
		// produces. An override is the one config entry that REMOVES enforcement, and the
		// reason is the only thing that will explain it later to anyone reading the audit
		// trail or this file.
		fmt.Fprint(stderr, "plock unlock: --reason is required and must not be blank\n")
		return cli.Usage
	}

	who := *by
	if who == "" {
		who = invokingUser()
	}

	cfg, err := config.Load(*path)
	if err != nil {
		fmt.Fprintf(stderr, "plock unlock: %v\n", err)
		return cli.Failure
	}

	// The user has to exist. An override for a name that is not managed is not harmless:
	// it sits in the file, it is granted by someone who believed they were unlocking a
	// child, and if that username is created next week it is already active with a stale
	// reason attached.
	if _, ok := cfg.UserFor(name); !ok {
		fmt.Fprintf(stderr, "plock unlock: no managed user %q\n", name)
		return cli.Failure
	}

	until := nowFunc().Add(time.Duration(*minutes) * time.Minute)
	cfg.AddOverride(config.Override{
		Username: name,
		Until:    until,
		Reason:   *reason,
		By:       who,
	})

	// The gate belongs to whoever produced the change -- Save deliberately does not
	// validate, because read-side code has to keep working with whatever is on disk. But
	// the gate is scoped to the change, not to the document.
	//
	// cfg.Validate() checks the whole file, including the admin credential, and a machine
	// that has never run `plock init` has no admin. Gating on it would mean a parent
	// cannot grant a 20-minute override on a headless box until they have also set up web
	// login -- and the error would name a field this command did not touch and could not
	// fix. Load already tolerates the rest of the file, and Save already refuses a version
	// it could not read back. What is ours to check is the entry we added, and it is
	// checked above: the user exists, the length is bounded, the reason is not blank.
	if err := cfg.Save(*path); err != nil {
		fmt.Fprintf(stderr, "plock unlock: %v\n", err)
		return cli.Failure
	}

	r := &cli.Report{}
	r.Field("user", name)
	r.Field("free until", until.Local().Format(time.RFC3339))
	r.Field("minutes", *minutes)
	r.Field("by", who)
	r.Field("reason", *reason)
	r.Line("\nThe enforcement loop picks this up on its next tick (within %ds), and the\naccount returns to its schedule on its own at the time above.",
		cfg.Settings.PollSeconds)
	if err := r.Write(stdout); err != nil {
		fmt.Fprintf(stderr, "plock unlock: %v\n", err)
		return cli.Failure
	}
	return cli.OK
}

func runOverride(stdout, stderr io.Writer, args []string) int {
	if len(args) == 0 {
		writeAll(stderr, overrideUsage)
		return cli.Usage
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return runOverrideList(stdout, stderr, rest)
	case "clear":
		return runOverrideClear(stdout, stderr, rest)
	case "-h", "--help", "help":
		return runHelp(stdout, stderr, []string{"override"})
	default:
		fmt.Fprintf(stderr, "plock override: unknown subcommand %q\n\n%s", sub, overrideUsage)
		return cli.Usage
	}
}

func runOverrideList(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("override list", flag.ContinueOnError)
	var flagOut bytes.Buffer
	fs.SetOutput(&flagOut)
	path := fs.String("config", defaultConfigPath, "path to the config document")
	asJSON := fs.Bool("json", false, "machine-readable output on stdout")
	fs.Usage = func() { flagOut.WriteString(overrideUsage) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if werr := writeAll(stdout, flagOut.String()); werr != nil {
				return cli.Failure
			}
			return cli.OK
		}
		fmt.Fprintf(stderr, "plock override list: %v\n%s", err, flagOut.String())
		return cli.Usage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "plock override list: unexpected argument %q\n\n", fs.Arg(0))
		return cli.Usage
	}

	cfg, err := config.Load(*path)
	if err != nil {
		return opFail(stderr, *asJSON, "plock override list", codeConfigUnreadable, err)
	}

	now := nowFunc()
	type row struct {
		Username string `json:"username"`
		Until    string `json:"until"`
		Reason   string `json:"reason"`
		By       string `json:"by"`
		Active   bool   `json:"active"`
	}
	rows := make([]row, 0, len(cfg.Overrides))
	for _, o := range cfg.Overrides {
		rows = append(rows, row{
			Username: o.Username,
			Until:    o.Until.Format(time.RFC3339),
			Reason:   o.Reason,
			By:       o.By,
			Active:   o.Until.After(now),
		})
	}

	if *asJSON {
		if err := cli.JSON(stdout, map[string]any{"overrides": rows}); err != nil {
			return opFail(stderr, true, "plock override list", codeOutputFailed, err)
		}
		return cli.OK
	}

	r := &cli.Report{}
	if len(rows) == 0 {
		r.Line("no overrides")
		return mustWrite(r, stdout, stderr, "override list")
	}
	for _, x := range rows {
		state := "expired"
		if x.Active {
			state = "active"
		}
		r.Line("%s  until %s  (%s)  by %s — %s", x.Username, x.Until, state, x.By, x.Reason)
	}
	return mustWrite(r, stdout, stderr, "override list")
}

func runOverrideClear(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("override clear", flag.ContinueOnError)
	var flagOut bytes.Buffer
	fs.SetOutput(&flagOut)
	path := fs.String("config", defaultConfigPath, "path to the config document")
	fs.Usage = func() { flagOut.WriteString(overrideUsage) }
	name, flagArgs, _ := cli.TakeFirst(fs, args)
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if werr := writeAll(stdout, flagOut.String()); werr != nil {
				return cli.Failure
			}
			return cli.OK
		}
		fmt.Fprintf(stderr, "plock override clear: %v\n%s", err, flagOut.String())
		return cli.Usage
	}
	if extra := fs.Args(); name == "" || len(extra) > 0 {
		if name == "" {
			fmt.Fprint(stderr, "plock override clear: exactly one username is required\n\n")
		} else {
			// Naming the extra argument rather than printing name and extras as a list of
			// usernames: the usual way to land here is a flag written after a `--`
			// separator, and "--config" is not anybody's username.
			fmt.Fprintf(stderr, "plock override clear: unexpected argument %q after username %q\n\n",
				extra[0], name)
		}
		writeAll(stderr, overrideUsage)
		return cli.Usage
	}

	cfg, err := config.Load(*path)
	if err != nil {
		fmt.Fprintf(stderr, "plock override clear: %v\n", err)
		return cli.Failure
	}
	if !cfg.RemoveOverride(name) {
		// Not found is reported on stderr and still exits 0: nothing the operator asked
		// for happened, but nothing went wrong either. `plock override clear kid` in a
		// teardown script must not abort the script because the override had already
		// expired on its own -- that is the same end state.
		fmt.Fprintf(stderr, "plock override clear: no override for %q\n", name)
		return cli.OK
	}
	// Removing an entry cannot make the document less valid than it was, so there is
	// nothing to gate here that Save does not already gate.
	if err := cfg.Save(*path); err != nil {
		fmt.Fprintf(stderr, "plock override clear: %v\n", err)
		return cli.Failure
	}
	// Confirmed on stdout, and named: the operator needs to see which account the file
	// changed for, and silence after a write is indistinguishable from a write that went
	// to a different config path.
	r := &cli.Report{}
	r.Field("cleared", name)
	r.Field("config", *path)
	return mustWrite(r, stdout, stderr, "override clear")
}

// mustWrite writes a report and maps a write failure onto the exit codes.
func mustWrite(r *cli.Report, stdout, stderr io.Writer, what string) int {
	if err := r.Write(stdout); err != nil {
		fmt.Fprintf(stderr, "plock %s: %v\n", what, err)
		return cli.Failure
	}
	return cli.OK
}

// invokingUser names whoever ran the command, for the override's `by` field.
//
// It is a var so the field can be pinned in a test; a recorded grant whose author depends
// on whose terminal the test suite ran from is not a fact worth asserting.
//
// The answer is best-effort and that is honest: the daemon's web UI records the admin who
// logged in, and a terminal records whatever the OS says the process owner is. When even
// that is unavailable the field says "unknown" rather than being left empty, because an
// empty author and an absent author read differently in the file.
var invokingUser = func() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "unknown"
}
