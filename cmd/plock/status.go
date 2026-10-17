package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/cli"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/schedule"
)

const statusUsage = `usage: plock status [--json] [--config PATH]

Reports what the daemon would do right now: whether enforcement is on, which lock
backends this machine offers, and for every managed user whether they are inside a
lockout window, which rule says so, when it ends, and when the next one opens.

It reads the same config file the daemon and the web UI read, and it changes nothing.
`

// statusCommand is the seam `status` needs from the machine, so the report can be tested
// against a fixed set of sessions instead of whatever logins happen to exist on the box
// running the test. Same reason chainView exists for doctor.
type statusCommand interface {
	Sessions(context.Context) ([]platform.Session, error)
	// Names and Unknown are here because the report promises both: docs/cli.md documents
	// `backends` as the configured order and `unknownBackends` as the names that resolved to
	// nothing, and a field that is never assigned marshals as null -- which is worse than
	// absent, because `"backends":null` reads as "the machine has none" rather than "this
	// command did not ask". *platform.Chain already answers both for doctor's seam, so this
	// costs no new code in platform, only the two lines that read it.
	Names() []string
	Unknown() []string
}

// buildStatusChain is the production wiring, behind a var for the same reason buildChain
// is one.
var buildStatusChain = func(s config.Settings) (statusCommand, error) {
	return platform.New(s, platform.ExecRunner{})
}

// statConfigPath is the file-info seam: the report quotes the config's mtime, and a test
// must be able to say what that mtime is.
var statConfigPath = os.Stat

// nowFunc is the clock. It is a var rather than a time.Now call so "ends at" can be
// pinned; a status test that computes its expectation from the same time.Now as the code
// under test asserts that a value equals itself.
var nowFunc = time.Now

// StatusReport is the --json document.
//
// Field names are the contract docs/cli.md publishes, so they are written out with explicit
// json tags rather than left to field-name inference: a later rename of a Go field must not
// silently rename a JSON key that a script reads.
//
// Every per-user fact is a string or a bool or an RFC3339 instant, never a Go duration.
// time.Duration marshals as an integer of nanoseconds, which is a value nobody can read off
// a terminal and which changes meaning if the unit is ever restated.
type StatusReport struct {
	Enforcement      bool            `json:"enforcement"`
	ConfigPath       string          `json:"configPath"`
	ConfigModifiedAt string          `json:"configModifiedAt"`
	PollSeconds      int             `json:"pollSeconds"`
	Backends         []string        `json:"backends"`
	UnknownBackends  []string        `json:"unknownBackends,omitempty"`
	Users            []StatusUser    `json:"users"`
	Sessions         []StatusSession `json:"sessions"`
	// Daemon is where the enforcement engine is actually running, which is what makes
	// a session's declinedByDaemon mean anything. It is a pointer because the probe
	// can be absent rather than answered: a report assembled without touching the
	// machine (every buildStatus test) has no daemon to report, and `null` says that,
	// where an all-false struct would claim a daemon that is not running.
	Daemon          *StatusDaemon `json:"daemon,omitempty"`
	Warnings        []string      `json:"warnings,omitempty"`
	UnsupportedHere []string      `json:"-"`
}

// StatusDaemon is the running engine's own location, as far as the self-session rule
// cares.
//
// The three states are deliberately distinguishable, because the sentence each one
// supports is different and collapsing any pair makes the report assert something the
// probe did not establish:
//
//   - Running false — no engine, so nothing is being declined, and an operator who was
//     wondering why nothing locked has their answer;
//   - Running true, LocationKnown true — the engine declines exactly the sessions
//     Declined marks;
//   - Running true, LocationKnown false — the engine is running and this command cannot
//     tell whose slice it is in, so no session is marked and the report says so instead
//     of implying "declines nobody".
type StatusDaemon struct {
	// Systemd is false when systemctl could not be executed at all: "no init system
	// here" is not the same claim as "the service is not running".
	Systemd bool `json:"systemd"`
	Running bool `json:"running"`
	// InUserSlice and UID are the engine's location: whose login session it is inside,
	// if it is inside anybody's. Zero values mean "outside anybody's", which is a real
	// answer (a system unit) and not a missing one — that is what LocationKnown is for.
	InUserSlice   bool   `json:"inUserSlice"`
	UID           uint32 `json:"uid,omitempty"`
	LocationKnown bool   `json:"locationKnown"`
}

// StatusUser is one managed user's verdict.
type StatusUser struct {
	Username      string `json:"username"`
	Enabled       bool   `json:"enabled"`
	Profile       string `json:"profile"`
	Lockout       bool   `json:"lockout"`
	Rule          string `json:"rule,omitempty"`
	RuleName      string `json:"ruleName,omitempty"`
	StartedAt     string `json:"startedAt,omitempty"`
	EndsAt        string `json:"endsAt,omitempty"`
	NextStart     string `json:"nextStart,omitempty"`
	Override      string `json:"override,omitempty"`
	OverrideUntil string `json:"overrideUntil,omitempty"`
	// Degraded carries what the evaluator could not read cleanly. It is surfaced here
	// because a parent reading `lockout: true` is entitled to know the schedule behind it
	// was widened rather than honoured.
	Degraded []string `json:"degraded,omitempty"`
}

// StatusSession is one live desktop session.
type StatusSession struct {
	Session  string `json:"session"`
	Username string `json:"username"`
	UID      uint32 `json:"uid"`
	Type     string `json:"type"`
	Seat     string `json:"seat,omitempty"`
	Locked   bool   `json:"locked"`
	Lockable bool   `json:"lockable"`
	// Declined is the reason a lockable session will not be locked: the engine runs
	// inside this session's own user slice. Only ever true when the report's daemon
	// location is known; an unknown location marks nothing rather than guessing, and
	// says so in warnings.
	Declined bool `json:"declinedByDaemon,omitempty"`
}

func runStatus(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	// Help is an answer and a bad flag is a complaint, so they go to different streams.
	// Same shape as doctor's flag set, for the same reason.
	var flagOut bytes.Buffer
	fs.SetOutput(&flagOut)
	path := fs.String("config", defaultConfigPath, "path to the config document")
	asJSON := fs.Bool("json", false, "machine-readable output on stdout")
	fs.Usage = func() { flagOut.WriteString(statusUsage) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if werr := writeAll(stdout, flagOut.String()); werr != nil {
				return cli.Failure
			}
			return cli.OK
		}
		fmt.Fprintf(stderr, "plock status: %v\n%s", err, flagOut.String())
		return cli.Usage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "plock status: unexpected argument %q\n\n", fs.Arg(0))
		writeAll(stderr, statusUsage)
		return cli.Usage
	}

	cfg, err := config.Load(*path)
	if err != nil {
		// A config that will not load is reported, never defaulted. Status answering from
		// guessed settings would tell an operator their child is not locked out when the
		// rules they wrote were never read.
		return statusFail(stdout, stderr, *asJSON, "config_unreadable", err)
	}

	info, err := statConfigPath(*path)
	if err != nil {
		// Load just succeeded, so this is a race or a permission change rather than a
		// missing file. The mtime is a fact about the file, and a report that cannot
		// establish it says so instead of printing the zero time, which reads as 1970.
		return statusFail(stdout, stderr, *asJSON, "config_unstatable", err)
	}

	now := nowFunc()
	rep := buildStatus(cfg, *path, info, now)

	// Where the engine itself is running, probed once and passed down. A session's
	// declined mark means nothing without it: this command runs as the operator, and
	// the operator's own slice answers a different question and gets the opposite
	// answer in both directions (an operator at a desktop with a system-unit daemon
	// would mark every one of their own sessions declined; an operator in SSH with a
	// desktop-started daemon would mark none).
	daemon := daemonProbe()
	rep.Daemon = &StatusDaemon{
		Systemd: daemon.SystemdKnown, Running: daemon.Running,
		InUserSlice: daemon.Location.InUserSlice, UID: daemon.Location.UID,
		LocationKnown: daemon.LocationKnown,
	}
	if daemon.Running && !daemon.LocationKnown {
		// The one case where saying nothing would itself be a claim: the sessions
		// below are then unmarked, and an unmarked list reads as "the engine declines
		// nobody" — the reassurance, with no evidence behind it.
		rep.Warnings = append(rep.Warnings, "daemon slice: the engine is running but its cgroup could not be read, "+
			"so no session is marked as declined")
	}

	var sessErr string
	chain, chainErr := buildStatusChain(cfg.Settings)
	if chainErr != nil {
		// A chain that could not be fully built still reports what it did build, which is
		// what doctor does for the same reason: the operator needs to know the config
		// named something that does not exist.
		rep.Warnings = append(rep.Warnings, chainErr.Error())
	}
	if chain != nil {
		// Assigned here rather than in buildStatus, because buildStatus deliberately never
		// touches the machine and these two fields are what the chain resolved to. An empty
		// slice, not nil: the JSON contract is an array, and a reader doing `.backends | length`
		// should not get a type error on a machine with no backends configured.
		rep.Backends = chain.Names()
		if rep.Backends == nil {
			rep.Backends = []string{}
		}
		rep.UnknownBackends = chain.Unknown()
		sessions, err := chain.Sessions(context.Background())
		if err != nil {
			// Sessions are a section of the report, not a precondition for it. The
			// schedule verdicts come from the config and the clock and are worth printing
			// even when logind is unreachable — that combination is itself the finding.
			sessErr = err.Error()
			rep.Warnings = append(rep.Warnings, "live sessions: "+err.Error())
		}
		rep.Sessions = statusSessions(sessions, daemon)
	}

	if *asJSON {
		if err := cli.JSON(stdout, rep); err != nil {
			fmt.Fprintf(stderr, "plock status: %v\n", err)
			return cli.Failure
		}
		return cli.OK
	}

	if err := writeStatus(stdout, rep, now); err != nil {
		fmt.Fprintf(stderr, "plock status: %v\n", err)
		return cli.Failure
	}
	if sessErr != "" {
		// The human report already carries the warning line; exiting non-zero too would
		// make `plock status && ...` fail on a machine whose schedule is fine but whose
		// desktop is not reachable, which is not the question that shell asks.
		_ = sessErr
	}
	return cli.OK
}

// statusFail reports a failure in whichever grammar the caller asked for.
//
// It defers to cli.Fail rather than owning a policy of its own: the contract is that a
// --json failure is JSON on stderr with stdout left empty, and the moment one command
// decides stdout is more convenient for it, the contract is per-command again.
func statusFail(_, stderr io.Writer, asJSON bool, code string, err error) int {
	return cli.Fail(stderr, asJSON, "plock status", code, err)
}

// buildStatus turns a config into the report, without touching the machine.
func buildStatus(cfg *config.Config, path string, info os.FileInfo, now time.Time) StatusReport {
	rep := StatusReport{
		Enforcement:      cfg.Settings.Enforcement,
		ConfigPath:       path,
		ConfigModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
		PollSeconds:      cfg.Settings.PollSeconds,
		Users:            []StatusUser{},
		Sessions:         []StatusSession{},
	}

	users := append([]config.User(nil), cfg.Users...)
	sort.Slice(users, func(i, j int) bool { return users[i].Username < users[j].Username })

	for _, u := range users {
		su := StatusUser{Username: u.Username, Enabled: u.Enabled, Profile: u.ProfileID}
		prof, ok := cfg.ProfileFor(u.Username)
		if !ok {
			// A user whose profile id resolves to nothing is the config's problem, and it
			// has to be visible here: "no profile" and "a profile with no rules" both look
			// like "not locked out", and only one of them is a mistake.
			su.Degraded = []string{fmt.Sprintf("no profile named %q", u.ProfileID)}
			rep.Users = append(rep.Users, su)
			continue
		}

		v := schedule.Evaluate(&prof, now)
		su.Lockout = v.Active
		// The evaluator's own words, not the error values: a parent reads this line, and
		// "unknown timezone "Mars/City", using UTC" is a sentence while an error's type
		// name is not.
		for _, e := range schedule.ProfileErrors(&prof) {
			su.Degraded = append(su.Degraded, e.Error())
		}
		if v.Rule != nil {
			su.Rule = v.Rule.ID
			su.RuleName = v.Rule.Name
			su.StartedAt = v.StartedAt.Format(time.RFC3339)
			su.EndsAt = v.EndsAt.Format(time.RFC3339)
		}
		if !v.NextStart.IsZero() {
			su.NextStart = v.NextStart.Format(time.RFC3339)
		}
		if ov, ok := cfg.ActiveOverride(u.Username, now); ok {
			su.Override = ov.Reason
			su.OverrideUntil = ov.Until.Format(time.RFC3339)
		}
		rep.Users = append(rep.Users, su)
	}
	return rep
}

// statusSessions renders live sessions, sorted so two runs on the same machine produce the
// same bytes.
//
// The daemon state arrives as an argument rather than being probed here, so that the
// declined marks are a function of data a test can state. Probing inside this function
// would make every session test depend on whether the machine running it happens to have
// plock installed, which is the mistake this command's tests were written to avoid.
func statusSessions(in []platform.Session, daemon daemonState) []StatusSession {
	out := make([]StatusSession, 0, len(in))
	for _, s := range in {
		// The same predicate the engine and the dashboard answer from, so the three
		// surfaces cannot drift into disagreeing about a session. The known-ness half
		// is not redundant with the producer leaving Location zero: this rule is
		// answered through a seam, and the safety property belongs in the rule, not in
		// the discipline of whoever fills the struct in.
		//
		// A statement rather than a field in the literal below, because a composite
		// literal's text moves when gofmt realigns it for an unrelated field.
		declined := daemon.LocationKnown && daemon.Location.Declines(s.UID)
		out = append(out, StatusSession{
			Session: s.ID, Username: s.Username, UID: s.UID, Type: s.Type,
			Seat: s.Seat, Locked: s.Locked, Lockable: platform.IsLockable(s),
			// The reason a lockable session will not be locked even while its profile is
			// in a lockout window: the engine runs inside this session's own user slice,
			// and locking it would take away the only screen the daemon can be stopped
			// from. Separate from lockable rather than folded into it, because "could be
			// locked" and "will be locked" are different questions, and a reader of
			// `lockable: true, lockout: true` with no lock arriving needs the second.
			Declined: declined,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Session < out[j].Session })
	return out
}

// writeStatus renders the human report.
//
// The answer comes first. `plock status` is typed by someone who wants to know whether the
// daemon is doing its job, and a report that reaches that line after a preamble has taught
// them to skip to the bottom.
func writeStatus(w io.Writer, rep StatusReport, now time.Time) error {
	r := &cli.Report{}
	state := "off"
	if rep.Enforcement {
		state = "on"
	}
	r.Field("enforcement", state)
	r.Field("config", rep.ConfigPath)
	r.Field("modified", rep.ConfigModifiedAt)
	r.Field("poll", fmt.Sprintf("%ds", rep.PollSeconds))
	// The engine's own location, in the report rather than only behind the sessions it
	// decides. Every wording here names what the probe established and stops there:
	// "not running" is only said when systemd answered, and "no session is declined"
	// is only said when the slice was actually read.
	if d := rep.Daemon; d != nil {
		switch {
		case !d.Systemd:
			r.Field("daemon", "unknown — no init system reachable, so this cannot say whether the engine is running")
		case !d.Running:
			r.Field("daemon", "not running")
		case !d.LocationKnown:
			r.Field("daemon", "running, slice unknown — see warnings")
		case d.InUserSlice:
			r.Field("daemon", fmt.Sprintf("running inside uid %d's login session — that account's sessions are never locked", d.UID))
		default:
			r.Field("daemon", "running in a system slice — no session is declined")
		}
	}
	// An empty list is a finding, not a blank. `backends` with nothing after it is the shape
	// this command shipped with -- the field was never assigned -- so the line has to be able
	// to tell "nothing is configured" apart from "this command did not look".
	if len(rep.Backends) == 0 {
		r.Field("backends", "none — settings.lockBackends is empty, so nothing can be locked")
	} else {
		r.Field("backends", strings.Join(rep.Backends, " -> "))
	}
	if len(rep.UnknownBackends) > 0 {
		r.Field("unknown", strings.Join(rep.UnknownBackends, ", "))
	}

	r.Section("managed users")
	if len(rep.Users) == 0 {
		r.Line("  none — the config has no users, so nothing is enforced")
	}
	for _, u := range rep.Users {
		r.Line("  %s", u.Username)
		if !u.Enabled {
			r.Line("    disabled — never locked out")
		} else if u.Lockout {
			r.Line("    locked out until %s (%s from now)", u.EndsAt, cli.Duration(endInstant(u.EndsAt, now).Sub(now)))
			if u.RuleName != "" {
				r.Line("      rule %s", u.RuleName)
			}
		} else if u.NextStart != "" {
			r.Line("    free — next window %s (%s)", u.NextStart, cli.Duration(endInstant(u.NextStart, now).Sub(now)))
		} else {
			r.Line("    free — no window in the next %d days", schedule.NextStartHorizon)
		}
		if u.Override != "" {
			r.Line("    override until %s — %s", u.OverrideUntil, u.Override)
		}
		for _, d := range u.Degraded {
			r.Line("    WARNING %s", d)
		}
	}

	r.Section("live desktop sessions")
	if len(rep.Sessions) == 0 {
		r.Line("  none visible")
	}
	for _, s := range rep.Sessions {
		lock := "unlocked"
		if s.Locked {
			lock = "locked"
		}
		// The declined reason goes on the row rather than in a footnote, because the
		// row is what reads as "lockable / unlocked, so the daemon is failing": the
		// sentence has to arrive in the same breath as the row it explains.
		declined := ""
		if s.Declined {
			declined = " — declined: the daemon runs in this session's own slice, so locking it would stop the daemon"
		}
		r.Line("  %s %s uid %d %s (%s)%s", s.Session, s.Username, s.UID, s.Type, lock, declined)
	}

	// The last action is deliberately absent, and said so. The audit writer is
	// append-only — there is no reader — so "last action" has no source, and a status
	// command that printed an empty field would teach the operator that an empty field
	// means "nothing happened" rather than "this build cannot tell you".
	r.Section("not reported here")
	r.Line("  last action — the audit log is write-only in this build; see %s", auditPathHint)

	for _, wn := range rep.Warnings {
		r.Line("WARNING %s", wn)
	}
	return r.Write(w)
}

// auditPathHint is where the trail lands by default. It is a const rather than read from
// config because the audit path is not a config field yet, and a status line that reads
// the wrong setting would point the operator at the wrong file.
const auditPathHint = "/var/log/plock/audit.jsonl"

// endInstant parses an RFC3339 field back into a time for the "from now" arithmetic.
//
// The report struct holds strings because strings are what --json publishes; recomputing
// the duration from the same instant the human line displays is what keeps the two halves
// of the sentence agreeing. A zero or unparseable value yields the zero time, which
// cli.Duration renders as "0s" — wrong, but wrong in the loud direction, and the field is
// only ever populated from a value that formatted successfully.
func endInstant(value string, now time.Time) time.Time {
	if value == "" {
		return now
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return now
	}
	return t
}
