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
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

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

const versionUsage = `Usage: plock version

Print the version, commit and build date.
`

// planned records the intended flag surface of commands that are not
// implemented yet, so `plock <cmd> -h` is accurate before the code exists.
// Every entry here is also listed in globalUsage; a test pins the two together
// so neither can quietly drift away from the other.
var planned = map[string]string{
	"serve":  "[--config PATH]",
	"status": "[--json]",
	"lock":   "<user> [--all]",
	"unlock": "<user> --minutes N --reason TEXT",
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		if err := writeAll(stderr, globalUsage); err != nil {
			return exitFailure
		}
		return exitUsage
	}

	command, rest := args[0], args[1:]
	switch command {
	case "help", "-h", "-help", "--help":
		return runHelp(stdout, stderr, rest)
	case "version":
		return runVersion(stdout, stderr, rest)
	case "doctor":
		return runDoctor(stdout, stderr, rest)
	case "init":
		return runInit(stdout, stderr, rest)
	case "serve":
		return runServe(stdout, stderr, rest)
	case "status":
		return runStatus(stdout, stderr, rest)
	default:
		if synopsis, ok := planned[command]; ok {
			return notImplemented(stdout, stderr, command, synopsis, rest)
		}
		if err := writeAll(stderr,
			fmt.Sprintf("plock: unknown command %q\n\n", command), globalUsage); err != nil {
			return exitFailure
		}
		return exitUsage
	}
}

// runHelp implements `plock help [command]`. Help goes to stdout; asking about
// something that does not exist is a usage error, because printing the overview
// anyway is how a typo turns into a mystery.
func runHelp(stdout, stderr io.Writer, args []string) int {
	// `plock help`, `plock help help` and `plock help -h` all mean "show me the
	// overview" — there is nothing more specific to say about help itself.
	if len(args) == 0 || isHelpFlag(args[0]) || args[0] == "help" {
		if err := writeAll(stdout, globalUsage); err != nil {
			return exitFailure
		}
		return exitOK
	}

	topic := args[0]
	switch {
	case topic == "version":
		if err := writeAll(stdout, versionUsage); err != nil {
			return exitFailure
		}
		return exitOK
	case topic == "doctor":
		if err := writeAll(stdout, doctorUsage); err != nil {
			return exitFailure
		}
		return exitOK
	case topic == "init":
		if err := writeAll(stdout, initUsage); err != nil {
			return exitFailure
		}
		return exitOK
	default:
		if synopsis, ok := planned[topic]; ok {
			if err := writeAll(stdout, stubUsage(topic, synopsis)); err != nil {
				return exitFailure
			}
			return exitOK
		}
	}

	if err := writeAll(stderr,
		fmt.Sprintf("plock: no help for unknown command %q\n\n", topic), globalUsage); err != nil {
		return exitFailure
	}
	return exitUsage
}

// runVersion prints the build identity. It takes no arguments, so a typo is an
// error rather than a silently ignored positional.
func runVersion(stdout, stderr io.Writer, args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		if err := writeAll(stdout, versionUsage); err != nil {
			return exitFailure
		}
		return exitOK
	}

	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, versionUsage) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if fs.NArg() > 0 {
		if err := writeAll(stderr,
			fmt.Sprintf("plock version: unexpected argument %q\n\n", fs.Arg(0)),
			versionUsage); err != nil {
			return exitFailure
		}
		return exitUsage
	}

	if _, err := fmt.Fprintln(stdout, version.String()); err != nil {
		fmt.Fprintf(stderr, "plock version: %v\n", err)
		return exitFailure
	}
	return exitOK
}

// notImplemented is the honest placeholder for a command that has a documented
// surface but no behaviour yet. It exits 1, not 0: a script must never read a
// no-op as success.
//
// Help is recognised only in flag position zero. Scanning the whole argv would
// let a *value* that happens to read "-h" — `plock lock --reason -h`, or
// anything after a `--` terminator — turn a command that did nothing into exit
// code 0.
func notImplemented(stdout, stderr io.Writer, command, synopsis string, args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		if err := writeAll(stdout, stubUsage(command, synopsis)); err != nil {
			return exitFailure
		}
		return exitOK
	}
	fmt.Fprintf(stderr, "plock %s: not implemented yet\n", command)
	return exitFailure
}

func stubUsage(command, synopsis string) string {
	return fmt.Sprintf("Usage: plock %s %s\n\n%s is not implemented yet.\n", command, synopsis, command)
}

// isHelpFlag accepts all three spellings Go's own flag package accepts, so the
// CLI does not disagree with itself about how to ask for help.
func isHelpFlag(arg string) bool {
	switch arg {
	case "-h", "-help", "--help":
		return true
	default:
		return false
	}
}

// writeAll reports a write failure so a broken pipe or a full device cannot be
// reported to the caller as success.
func writeAll(w io.Writer, parts ...string) error {
	for _, p := range parts {
		if _, err := io.WriteString(w, p); err != nil {
			return err
		}
	}
	return nil
}
