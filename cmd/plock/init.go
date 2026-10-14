package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/auth"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

const initUsage = `Usage: plock init [--username NAME] (--password PASS | --password-stdin) [--force]

Create or replace the single administrator account, non-interactively.

  --username NAME        the login name (required)
  --password PASS        the password, on the command line
  --password-stdin       read the password from standard input instead
  --force                replace an existing administrator account
  --config PATH          config file (default /etc/plock/config.json)
  --state-dir PATH       state directory (default /var/lib/plock)

The password is stored only as a PBKDF2-SHA256 verifier and is never written to
the config in any recoverable form. --password-stdin is the form to use in a
script: a password in argv is readable by any user on the machine for as long as
the process runs, and lands in the shell history.

A password shorter than 12 characters is refused. This account unlocks every
lockout on the machine over the local network, so it is not a password to reuse
from somewhere less important.

Re-running without --force on a machine that already has an administrator is an
error, so a provisioning script that runs twice cannot silently reset the
password out from under the parent who set it.
`

// minPasswordLen is the floor the CLI enforces.
//
// The KDF cost is what makes an offline crack expensive; length is what makes the
// password not the first thing a guesser tries. Sixty thousand iterations cannot
// save "password123456", and this account is the master switch for every child on
// the LAN.
const minPasswordLen = 12

// defaultStateDir is where the daemon keeps state that is not configuration: the
// TLS material #9 bootstraps, and anything later that is generated rather than
// chosen. It is a separate directory from /etc/plock because the two have
// different owners in an operator's head — one is edited, the other is replaced.
const defaultStateDir = "/var/lib/plock"

// initFlags is the parsed flag set, kept as a struct so the behaviour is testable
// without a process boundary.
type initFlags struct {
	username      string
	password      string
	passwordStdin bool
	force         bool
	configPath    string
	stateDir      string
}

// parseInitArgs builds the flag set. It is separate from runInit so the behaviour
// can be driven directly, with a chosen stdin, rather than only through a process.
func parseInitArgs(args []string) (*initFlags, error) {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	// flag sends the usage line and the parse error to the same sink; the caller
	// decides which stream each belongs on, so the buffer is handed back untouched.
	fs.SetOutput(&flagErrSink{})

	f := &initFlags{}
	fs.StringVar(&f.username, "username", "", "administrator login name")
	fs.StringVar(&f.password, "password", "", "administrator password")
	fs.BoolVar(&f.passwordStdin, "password-stdin", false, "read the password from standard input")
	fs.BoolVar(&f.force, "force", false, "replace an existing administrator account")
	fs.StringVar(&f.configPath, "config", defaultConfigPath, "path to the config file")
	fs.StringVar(&f.stateDir, "state-dir", defaultStateDir, "path to the state directory")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	return f, nil
}

// flagErrSink discards flag's own writing. runInit re-emits the parse error on
// stderr with the usage, so letting flag write to os.Stderr would put a second
// copy of the complaint somewhere the caller cannot capture.
type flagErrSink struct{}

func (flagErrSink) Write(p []byte) (int, error) { return len(p), nil }

func runInit(stdout, stderr io.Writer, args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		if err := writeAll(stdout, initUsage); err != nil {
			return exitFailure
		}
		return exitOK
	}
	f, err := parseInitArgs(args)
	if err != nil {
		return initUsageError(stderr, fmt.Sprintf("plock init: %v", err))
	}
	return doInit(f, stdout, stderr, os.Stdin)
}

// doInit is the whole of `plock init` with its inputs named.
//
// stdin is a parameter rather than os.Stdin because the --password-stdin path is
// the one a provisioning script uses, and it is the one path here where "what got
// read" has to be pinned by a test: a trailing newline from a heredoc is not part
// of anybody's password, and a password with a trailing space is.
func doInit(f *initFlags, stdout, stderr io.Writer, stdin io.Reader) int {
	switch {
	case strings.TrimSpace(f.username) == "":
		return initUsageError(stderr, "plock init: --username is required")
	case f.password == "" && !f.passwordStdin:
		return initUsageError(stderr,
			"plock init: a password is required: pass --password or --password-stdin")
	case f.password != "" && f.passwordStdin:
		// Refusing the combination is not fussiness. If both were accepted, which
		// one wins would be decided by the order of two ifs in this file, and a
		// provisioning script that sets both would get a password nobody can
		// reproduce from reading the script.
		return initUsageError(stderr,
			"plock init: --password and --password-stdin are mutually exclusive")
	}

	password := f.password
	if f.passwordStdin {
		// ReadLine rather than io.ReadAll: the password is one line, and reading to
		// EOF would swallow whatever the caller piped after it and treat the
		// concatenation as a secret.
		b, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return initFailure(stderr, fmt.Sprintf("plock init: reading the password: %v", err))
		}
		password = strings.TrimRight(b, "\r\n")
	}

	if len(password) < minPasswordLen {
		// The length is reported, the password is not. An echo of the value would
		// put the secret in the journal for a mistake made while typing it.
		return initUsageError(stderr, fmt.Sprintf(
			"plock init: the password is %d characters; at least %d are required\n"+
				"This account unlocks every lockout on the machine over the local network.",
			len(password), minPasswordLen))
	}

	// The verifier is produced before anything is written, so a failure in the KDF
	// leaves the existing credential exactly as it was rather than half-applied.
	verifier, err := auth.Hasher{}.Hash(password)
	// Dropping the last reference to the plaintext as soon as the KDF has consumed it.
	//
	// This is not a scrub. The bufio buffer behind the string is still live and Go may
	// have copied the bytes on assignment, so zeroing this variable removes one
	// reference and no bytes. Getting the stronger property would mean reading into a
	// fixed array and zeroing that, which the password-prompt path does not do; the
	// claim here is only "we do not hold it any longer than the KDF needs".
	password = ""
	if err != nil {
		return initFailure(stderr, fmt.Sprintf("plock init: hashing the password: %v", err))
	}

	cfg, err := loadOrStart(f.configPath)
	if err != nil {
		return initFailure(stderr, fmt.Sprintf("plock init: %v", err))
	}
	if cfg.Admin.Username != "" && !f.force {
		// This sentence used to promise "Every existing admin session ends when you do."
		// Nothing ends them. Sessions live in the daemon's memory, plock init writes a
		// config file and exits, and there is no config-watch, no SIGHUP handler, and no
		// consumer of Admin.UpdatedAt anywhere in the tree. The engine re-reads the config
		// every tick, so the *verifier* is picked up within 30 seconds — but a token minted
		// before the rotation keeps resolving against the in-memory store for its full
		// sliding 12-hour window, refreshed on every request.
		//
		// That is printed at precisely the moment a parent is most likely to be rotating the
		// password because they believe a session is compromised, and it tells them the thing
		// that did not happen. Saying less is the fix until the daemon has a consumer for it;
		// see the ticket referenced in the PR.
		return initFailure(stderr, fmt.Sprintf(
			"plock init: %s already has an administrator %q.\n"+
				"Re-run with --force to replace it. The new password is picked up within one\n"+
				"engine tick, but sessions already signed in stay valid until the daemon is\n"+
				"restarted — run systemctl restart plock to end them now.",
			f.configPath, cfg.Admin.Username))
	}

	cfg.Admin.Username = strings.TrimSpace(f.username)
	cfg.Admin.Password = verifier
	cfg.Admin.UpdatedAt = time.Now().UTC()

	if err := cfg.Validate(); err != nil {
		return initFailure(stderr, fmt.Sprintf("plock init: the resulting config is invalid: %v", err))
	}
	if err := ensurePrivateDir(f.stateDir); err != nil {
		return initFailure(stderr, fmt.Sprintf("plock init: %v", err))
	}
	// The store writes atomically, which means it needs its directory to already
	// exist: it creates a temporary file beside the target and renames it over, so
	// a missing directory is a failed write rather than a created one. On a fresh
	// machine that directory is this command's job — the ticket's "create
	// /etc/plock with the right modes" is literally this line, and without it
	// `plock init` cannot be the first thing anybody runs.
	if dir := filepath.Dir(f.configPath); dir != "" && dir != "." {
		if err := ensureDirUsable(dir, 0o700); err != nil {
			return initFailure(stderr, fmt.Sprintf("plock init: %v", err))
		}
	}
	if err := cfg.Save(f.configPath); err != nil {
		return initFailure(stderr, fmt.Sprintf("plock init: writing %s: %v", f.configPath, err))
	}

	// The URL is the thing the parent does next, and the port is a default they may
	// never have seen written down anywhere else. Printing it is what makes a
	// headless provisioning run finish with something actionable rather than with
	// "ok".
	if err := writeAll(stdout, fmt.Sprintf(
		"Administrator %q set in %s\nOpen https://%s/ to sign in.\n",
		cfg.Admin.Username, f.configPath, hostAndPort(cfg.Settings.Listen))); err != nil {
		return exitFailure
	}
	return exitOK
}

// loadOrStart reads the config, or starts a fresh document when the file does not
// exist yet.
//
// Only "not there" is treated as fresh. A config that exists and cannot be parsed
// is an error: overwriting it would silently delete every user, profile and rule
// on the machine because somebody typed the wrong password twice, and the store
// refuses a malformed file for exactly this reason.
func loadOrStart(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err == nil {
		return cfg, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return config.Default(), nil
	}
	// A missing parent directory is the other shape of "not there yet" on a first
	// run, and Save creates it.
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && errors.Is(pathErr.Err, os.ErrNotExist) {
		return config.Default(), nil
	}
	return nil, fmt.Errorf("reading %s: %w", path, err)
}

// ensurePrivateDir creates the state directory at 0700.
//
// The mode is set on a descriptor for the directory rather than by chmod on the
// path, and an existing directory is checked rather than corrected: see the note
// on internal/audit's prepareDir, which is the same rule learned the same way. A
// root CLI that chmods a directory it did not create is an arbitrary-chmod
// primitive pointed at whatever --state-dir says.
func ensurePrivateDir(path string) error {
	return ensureDirPrivate(path, 0o700)
}

// hostAndPort turns a listen address into something that belongs in a URL.
//
// A listen address of "0.0.0.0:8443" is a bind address, not a host you can point a
// browser at, and printing it as one is how a parent types it into the address bar
// and gets a connection error on a machine that is working perfectly.
func hostAndPort(listen string) string {
	host, port, found := strings.Cut(listen, ":")
	if !found {
		return listen
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		// The daemon is on this machine and the parent is standing in front of it.
		host = "localhost"
	}
	return host + ":" + port
}

func initUsageError(stderr io.Writer, msg string) int {
	if err := writeAll(stderr, msg+"\n\n", initUsage); err != nil {
		return exitFailure
	}
	return exitUsage
}

func initFailure(stderr io.Writer, msg string) int {
	if err := writeAll(stderr, msg+"\n"); err != nil {
		return exitFailure
	}
	return exitFailure
}
