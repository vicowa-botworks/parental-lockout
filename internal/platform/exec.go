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
	//
	// Implementations must bound the call: a backend's caller is a 30s loop that has
	// to keep running whether or not the thing it asked ever answers. ExecRunner
	// documents the default budget.
	Run(ctx context.Context, argv []string, env []string, uid uint32) ([]byte, error)
}

// ErrBinaryNotFound means the program named by argv[0] is not there.
//
// It is a distinct error rather than a wrapped exec.ErrNotFound because the caller's
// job is to tell an operator "loginctl is not installed", and that sentence should
// not depend on the shape of the standard library's error text. errors.Is still
// works against exec.ErrNotFound.
var ErrBinaryNotFound = errors.New("not found in PATH")

// DefaultCallTimeout bounds a single external command.
//
// Every call this daemon makes is a short CLI round-trip, but nothing about that is
// enforced by the tools: `gdbus call` has no default timeout, so a service that owns
// the bus name and never replies leaves the child waiting forever — and ExecRunner
// only ever killed on the caller's context. With no deadline anywhere in non-test
// code, a hung bus wedged whatever asked: the 30s enforcement loop would simply stop
// enforcing, and say nothing. Silence is this daemon's characteristic failure mode,
// not a crash.
//
// 10s is generous against what these calls actually cost (loginctl show-session and
// gdbus call both return well under a second on the target box) and still lets the
// whole default chain run inside one 30s tick.
const DefaultCallTimeout = 10 * time.Second

// ExecRunner runs commands for real.
type ExecRunner struct {
	// CallTimeout bounds each command. Zero means DefaultCallTimeout.
	//
	// The bound only ever tightens a caller's deadline, never loosens it, so a call
	// site cannot accidentally become unbounded by forgetting to set something. A
	// caller that genuinely needs a longer budget — a future bulk or slow operation —
	// builds its own ExecRunner with a larger value rather than passing a context
	// that quietly disables the guard.
	CallTimeout time.Duration
}

func (r ExecRunner) timeout() time.Duration {
	if r.CallTimeout != 0 {
		return r.CallTimeout
	}
	return DefaultCallTimeout
}

// Run starts argv and returns its stdout.
//
// A non-zero exit is an error carrying the program's own stderr, because "loginctl
// lock-session failed" without the reason is a bug report nobody can act on. A
// missing binary is reported as ErrBinaryNotFound naming the program, not as the
// exec package's error chain.
//
// The call is bounded by CallTimeout, so a command that never returns cannot wedge
// the caller; see DefaultCallTimeout for why the guard lives here rather than in the
// backends.
func (r ExecRunner) Run(ctx context.Context, argv []string, env []string, uid uint32) ([]byte, error) {
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

	// Bound the call before handing the context to the child. Doing it here rather
	// than in each backend means the guard cannot be forgotten: every command this
	// package runs, now and later, and every caller including `plock doctor`, inherits
	// it. A caller's own earlier deadline still wins, because context.WithTimeout
	// takes the sooner of the two.
	callTimeout := r.timeout()
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

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

	// A call that ran out of time is its own answer and must not read as a crash. The
	// killed child reports "signal: killed" with nothing on stderr, which an operator
	// reads as "this tool is broken"; what actually happened is that the thing it
	// called never answered, which on a desktop usually means a service that owns the
	// bus name and is not replying. That distinction is the whole reason the bound
	// exists, so it has to survive into the error text.
	if ctx.Err() != nil {
		return out, fmt.Errorf("%s: no answer within %s, gave up: %w", argv[0], callTimeout, ctx.Err())
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
