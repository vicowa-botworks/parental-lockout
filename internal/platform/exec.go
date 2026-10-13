package platform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner is the only way this package starts a process.
//
// It is an interface rather than a function call for one reason: the assertions that
// matter here are about the exact argv and env a lock produces. Getting those wrong
// is not a crash, it is a daemon that quietly does nothing, or worse, aims a lock at
// the wrong session. A fake runner can pin those byte for byte; a real one cannot be
// run in a test without locking someone's screen.
type Runner interface {
	// Run executes argv (argv[0] is the program, the rest its arguments — never a
	// shell string, and never a command line that a username has been interpolated
	// into) and returns its standard output.
	//
	// env is the complete environment as KEY=VALUE strings. A nil env inherits the
	// daemon's own environment; a backend that runs something inside another user's
	// desktop session must build the environment itself, because the daemon's bus
	// address is not that user's.
	//
	// uid selects the account the process runs as. 0 means "stay as we are", which
	// for the daemon is root. Anything else drops privileges before exec.
	Run(ctx context.Context, argv []string, env []string, uid uint32) ([]byte, error)
}

// ErrBinaryNotFound means the program named by argv[0] is not there.
//
// It is a distinct error rather than a wrapped exec.ErrNotFound because the caller's
// job is to tell an operator "loginctl is not installed", and that sentence should
// not depend on the shape of the standard library's error text. errors.Is still
// works against exec.ErrNotFound.
var ErrBinaryNotFound = errors.New("not found in PATH")

// ExecRunner runs commands for real.
type ExecRunner struct{}

// Run starts argv and returns its stdout.
//
// A non-zero exit is an error carrying the program's own stderr, because "loginctl
// lock-session failed" without the reason is a bug report nobody can act on. A
// missing binary is reported as ErrBinaryNotFound naming the program, not as the
// exec package's error chain.
func (ExecRunner) Run(ctx context.Context, argv []string, env []string, uid uint32) ([]byte, error) {
	if len(argv) == 0 || argv[0] == "" {
		return nil, errors.New("platform: Run called with no program to execute")
	}
	// A single element containing spaces is the tell that someone built a command
	// line instead of an argv. exec would then look for a file literally named
	// "loginctl list-sessions", fail with a confusing path, and the mistake would be
	// found in production.
	if strings.ContainsAny(argv[0], " \t\n") {
		return nil, fmt.Errorf("platform: %q looks like a command line, not a program; pass the arguments as separate elements", argv[0])
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if env != nil {
		cmd.Env = env
	} else {
		cmd.Env = os.Environ()
	}
	// WaitDelay bounds how long Wait holds on after the child is killed, so a
	// grandchild that survives and keeps the stdout pipe open cannot park the caller
	// past the context deadline.
	//
	// The kill itself is CommandContext's own default cancellation, which is
	// exactly cmd.Process.Kill() — there is no cmd.Cancel line here because
	// re-stating the default only implies it is doing something the default is not.
	//
	// What this does NOT do is guarantee the whole process tree dies. CommandContext's
	// cancellation reaches the direct child only; a grandchild it spawned keeps
	// running, and WaitDelay only stops us from waiting for it. A backend that shells
	// out to something with helpers of its own has to put the child in its own process
	// group and kill the group — this seam does not do that on its behalf.
	cmd.WaitDelay = 5 * time.Second

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if uid != 0 {
		if err := dropPrivileges(cmd, uid); err != nil {
			return nil, err
		}
	}

	// Output() returns whatever was written even when it also returns an error, so the
	// stdout buffer is kept on the failure paths below. A program that printed a D-Bus
	// error object or a usage line before exiting non-zero has said the useful thing
	// there, and dropping it leaves the operator with "exited with 1".
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}

	// Name the program and say what is wrong, in one sentence an operator can act
	// on. The exec package's text for a missing file is "exec: \"x\": executable
	// file not found in $PATH", which is accurate but buries the name.
	if errors.Is(err, exec.ErrNotFound) {
		return out, fmt.Errorf("%s: %w", argv[0], ErrBinaryNotFound)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			// Nothing on stderr: the exec error's own text is all there is, and it
			// carries the exit status.
			msg = exitErr.Error()
		}
		return out, fmt.Errorf("%s exited with %d: %s", argv[0], exitErr.ExitCode(), msg)
	}
	return out, fmt.Errorf("%s: %w", argv[0], err)
}
