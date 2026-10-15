package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// CommandID is the registry name of the configured-command backend.
const CommandID = "command"

// commandBackend is a registered backend that runs a program the operator names.
//
// It exists so a developer can run the whole daemon — enforcement loop included — on a
// machine with no graphical session and no root, which is the loop #12 asks for and which
// the three shipped backends cannot provide because all three need a real logind session
// on a real seat.
//
// It is not the same thing as Mock, and the difference is the reason this one is
// registered and Mock is not. Mock is a scripted object that reports success; handing it
// a registry id would let a config point lockBackends at something that locks nothing
// while answering "done", which is the failure shape this daemon cannot afford because it
// looks like it is working. This backend really execs a program and returns that
// program's exit status as its own answer. "Does nothing" is therefore not a hidden
// property of a magic identifier — it is an absolute path in a root-owned 0600 file,
// visible in `plock doctor`, unchangeable by anyone who cannot already write the policy.
// That is the same trust boundary docs/security.md §2 puts the whole config behind.
//
// What it deliberately does not do:
//
//   - It does not resolve a bare name through PATH. The path must be absolute, refused at
//     construction as well as in Validate, because this runs inside a root daemon and #53
//     established that a PATH lookup on the lock path is an attack surface
//     (resolveProgram, docs/security.md §8). Validate is the message an operator sees;
//     this check is the one that holds when a Settings value was built by hand rather
//     than read from a file.
//   - It does not implement Confirmer. Nothing about a subprocess exiting 0 says the
//     screen is locked, and inventing a second signal to corroborate one would be exactly
//     the rubber stamp #29 warns about. So a chain using this backend reports
//     lock.unconfirmed in the audit trail. On a developer's own machine that is the
//     honest reading, and it is the reading the trail should give.
//   - It does not list sessions itself. A backend that lists nothing makes the
//     enforcement loop find no sessions and enforce nothing while every component reports
//     health — the fail-open direction this repo resolves against everywhere else. So
//     listing is delegated to logind, which is the part a dev box does have.
type commandBackend struct {
	path string
	argv []string

	// lister answers Sessions. It is a field rather than a call to &logind{r} so a test
	// can supply sessions without a loginctl on PATH; the constructor wires the real one,
	// which is the same seam desktop.go uses for the hint.
	lister sessionLister
	// exec is the call site, for the same reason: a test must be able to assert the
	// exact argv without a process being spawned.
	exec func(ctx context.Context, path string, argv []string) error
}

var _ Platform = (*commandBackend)(nil)

// sessionLister is the half of Platform this backend delegates. Named rather than written
// inline so the delegation is a stated contract: the command backend contributes the lock
// and logind contributes the listing, and a test can hand either half to the other.
type sessionLister interface {
	Sessions(ctx context.Context) ([]Session, error)
}

func init() {
	Register(CommandID, func(r Runner, s config.Settings) (Platform, error) {
		if r == nil {
			return nil, errors.New("command: no runner")
		}
		b, err := newCommand(s, &logind{r: r})
		if err != nil {
			return nil, err
		}
		b.exec = func(ctx context.Context, path string, argv []string) error {
			// Runner.Run takes one argv with the program at index 0: never a shell
			// string, so a path containing a space stays one argument and nothing in
			// the config can be reinterpreted as shell syntax.
			//
			// env is nil and uid is 0 -- "inherit, stay as we are" -- deliberately.
			// The other backends drop to the session uid because they must act inside
			// another user's desktop bus; this backend has no such requirement, and
			// asking for a uid change is a privileged operation, so dropping would
			// make the dev loop fail with EPERM for a reason that has nothing to do
			// with what it is testing. Whatever the daemon runs as, the script runs as.
			_, err := r.Run(ctx, append([]string{path}, argv...), nil, 0)
			return err
		}
		return b, nil
	})
}

// newCommand builds the backend over the settings. Split out from the registration so the
// refusals are testable without going through the registry.
func newCommand(s config.Settings, lister sessionLister) (*commandBackend, error) {
	path := strings.TrimSpace(s.LockCommandPath)
	switch {
	case path == "":
		// Reaching here means lockBackends named "command" with no path. Validate
		// refuses that document, so this error is what a hand-built Settings produces
		// -- and it must be an error rather than a backend that locks nothing.
		return nil, errors.New("command: settings.lockCommandPath is required when lockBackends names \"command\"")
	case !strings.HasPrefix(path, "/"):
		// The same rule as Validate, enforced here because New is reachable with a
		// Settings nobody validated. A relative path would resolve against the daemon's
		// working directory, which under systemd is "/" and is not a thing the person
		// who wrote the config chose.
		return nil, fmt.Errorf("command: settings.lockCommandPath %q must be an absolute path; a bare or relative name is refused rather than looked up in PATH", path)
	}
	if lister == nil {
		return nil, errors.New("command: no session lister")
	}
	return &commandBackend{path: path, argv: append([]string(nil), s.LockCommandArgv...), lister: lister}, nil
}

func (b *commandBackend) Name() string { return CommandID }

// Sessions delegates to logind. See the type comment for why this is not allowed to come
// back empty on its own.
func (b *commandBackend) Sessions(ctx context.Context) ([]Session, error) {
	return b.lister.Sessions(ctx)
}

// Lock runs the configured program for one session.
//
// Substitution: every %SESSION%, %USER%, %UID%, %SEAT% and %TTY% in the configured argv
// is replaced with that field from the session, so a script can put the identity where
// its own argument parser wants it. A token whose field is empty substitutes to empty
// rather than dropping the argument — a script that asked for the seat should get an
// empty seat and notice, not a shifted argv that silently means something else.
//
// When the argv names no token at all, the session id is appended as one final argument.
// The reason is the failure this is guarding: a command backend configured with no way to
// know which session it was asked about can only ever lock whatever it guessed, and a
// developer's script that locks the wrong thing looks identical to one that worked.
// Appending costs a script that ignores its arguments nothing.
func (b *commandBackend) Lock(ctx context.Context, s Session) error {
	argv, substituted := b.expand(s)
	if !substituted {
		argv = append(argv, s.ID)
	}
	if b.exec == nil {
		return fmt.Errorf("command: no runner configured for %s", b.path)
	}
	if err := b.exec(ctx, b.path, argv); err != nil {
		// The program's own words are the diagnosis, so they are kept rather than
		// replaced with "the lock failed". errors.Is still works against the runner's
		// sentinel causes.
		return fmt.Errorf("command: %s: %w", b.path, err)
	}
	return nil
}

// expand substitutes the session's fields into the configured argv, reporting whether any
// token appeared.
func (b *commandBackend) expand(s Session) ([]string, bool) {
	fields := map[string]string{
		"%SESSION%": s.ID,
		"%USER%":    s.Username,
		"%UID%":     fmt.Sprintf("%d", s.UID),
		"%SEAT%":    s.Seat,
		"%TTY%":     s.TTY,
	}
	out := make([]string, 0, len(b.argv)+1)
	found := false
	for _, a := range b.argv {
		for token, value := range fields {
			if strings.Contains(a, token) {
				found = true
				a = strings.ReplaceAll(a, token, value)
			}
		}
		out = append(out, a)
	}
	return out, found
}

// Probe answers the lock-capable question: is the configured program there and runnable.
//
// It must not run the program. `plock doctor` is what an operator reaches for when they
// suspect locking is broken, and a probe that executed the lock command would lock their
// screen at the moment they asked whether locking works. So this is a stat plus an
// executability check, and the listing question is reported separately underneath it.
//
// The order is the point (#25 item 4): a probe that answered from the listing would
// report the backend available on a machine where every lock is about to be refused, and
// the reverse — reporting unavailable because loginctl is missing — would hide that the
// program itself is fine, which is the half the operator can actually fix by installing
// something. Both are reported; availability is decided by the lock half.
func (b *commandBackend) Probe(ctx context.Context) []BackendStatus {
	status := BackendStatus{Name: b.Name()}

	switch err := executableFile(b.path); {
	case err != nil:
		status.Detail = fmt.Sprintf("%s: %v", b.path, err)
		return []BackendStatus{status}
	}

	status.Available = true
	status.Detail = fmt.Sprintf("%s (configured command; not a production backend)", b.path)

	// The listing half, reported underneath rather than folded in. A machine where
	// logind cannot answer will lock nothing this backend is asked to lock, and that
	// belongs in the same line the operator is already reading.
	if _, err := b.lister.Sessions(ctx); err != nil {
		status.Detail = fmt.Sprintf("%s; sessions unavailable: %v", status.Detail, err)
	}
	return []BackendStatus{status}
}

// executableFile reports whether path is a file the daemon can execute.
//
// A directory passes os.Stat and passes the mode check on a typical umask, and exec of a
// directory fails with EACCES on some kernels and EISDIR on others — so "is a directory"
// is said here instead of being left to whatever the exec returns at 3 a.m. A symlink is
// followed deliberately: the operator owns the config, and refusing a symlink would only
// push them toward putting the real binary in a worse place.
func executableFile(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// The program is named, matching how ExecRunner reports the same thing
			// (exec.go: "%w: %s: %s"). os.ErrNotExist is deliberately not wrapped: this
			// package's vocabulary for "the binary is not there" is ErrBinaryNotFound,
			// and a caller that had to check both would be a caller guessing.
			return fmt.Errorf("%w: %s", ErrBinaryNotFound, path)
		}
		return err
	}
	if fi.IsDir() {
		return errors.New("is a directory, not a program")
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", fi.Mode())
	}
	// Any execute bit is enough to try: the daemon runs as root, for whom the mode bits
	// are advisory rather than binding, and requiring the owner bit specifically would
	// refuse a 0750 program owned by a different uid that root can still run.
	if fi.Mode().Perm()&0o111 == 0 {
		return errors.New("not executable by anyone")
	}
	return nil
}
