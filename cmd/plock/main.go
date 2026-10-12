// Command plock enforces parental lockout periods on Linux desktop sessions.
//
// It runs as a root-owned daemon that keeps a user's session locked during
// configured lockout windows, re-asserting the lock on every poll tick, and
// serves the parent-facing admin UI over HTTPS on the local network.
//
// The command surface is declared here in full from the first release so that
// `plock -h` documents where the tool is going; commands whose behaviour has
// not landed yet report that plainly and exit non-zero rather than pretending.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/vicowa-botworks/parental-lockout/internal/version"
)

// Exit codes shared by every subcommand: 0 success, 1 operational failure,
// 2 usage error. Anything else would make scripting the CLI a guessing game.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

const globalUsage = `plock - parental lockout for Linux desktop sessions

Usage:
  plock <command> [flags]

Commands:
  serve     run the enforcement daemon and the admin web UI
  init      create or replace the single admin account
  status    show enforcement state and per-user lockout status
  lock      lock a user's session immediately, ignoring the schedule
  unlock    grant a temporary override that suspends enforcement
  doctor    diagnose whether locking actually works on this machine
  version   print version information

Run "plock <command> -h" for the flags a command accepts.
`

// planned records the intended flag surface of commands that are not
// implemented yet, so `plock <cmd> -h` is accurate before the code exists.
var planned = map[string]string{
	"serve":  "[--config PATH]",
	"init":   "[--username NAME] (--password PASS | --password-stdin) [--force]",
	"status": "[--json]",
	"lock":   "<user> [--all]",
	"unlock": "<user> --minutes N --reason TEXT",
	"doctor": "[--json] [--try USER]",
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, globalUsage)
		return exitUsage
	}

	command, rest := args[0], args[1:]
	switch command {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, globalUsage)
		return exitOK
	case "version":
		return runVersion(stdout, stderr, rest)
	default:
		if _, ok := planned[command]; ok {
			return notImplemented(stdout, stderr, command, rest)
		}
		fmt.Fprintf(stderr, "plock: unknown command %q\n\n%s", command, globalUsage)
		return exitUsage
	}
}

// runVersion prints the build identity. It accepts only -h so that a typo in a
// flag is an error rather than a silently ignored argument.
func runVersion(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, "Usage: plock version\n\nPrint the version, commit and build date.\n")
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitUsage
	}
	fmt.Fprintln(stdout, version.String())
	return exitOK
}

// notImplemented is the honest placeholder for a command that has a documented
// surface but no behaviour yet. It exits 1, not 0: a script must never read a
// no-op as success.
func notImplemented(stdout, stderr io.Writer, command string, args []string) int {
	if slices.Contains(args, "-h") || slices.Contains(args, "--help") {
		fmt.Fprintf(stderr, "Usage: plock %s %s\n\n%s is not implemented yet.\n",
			command, planned[command], command)
		return exitOK
	}
	fmt.Fprintf(stderr, "plock %s: not implemented yet\n", command)
	return exitFailure
}
