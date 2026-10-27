package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/schedule"
)

// The diagnostic half of `plock doctor`: the checks beyond "which backend would I use".
//
// The reason this file exists is the ticket's own framing — doctor is the answer to "it is
// not working". A parent who types that has already noticed the screen did not lock, and
// every check here is one of the reasons that happens which are invisible from the outside:
// the daemon is not running, the config says enforcement is off, the file is world-readable
// so the daemon refused it, the certificate expired, or the child is in a window right now
// with an unlocked screen.

// checkStatus is the verdict of one check.
type checkStatus string

const (
	verdictOK   checkStatus = "ok"
	verdictWarn checkStatus = "warn"
	verdictFail checkStatus = "fail"
	// verdictSkip means the check could not run here, not that it passed. The distinction is
	// in the report because "we did not look" read as "we looked and it was fine" is the
	// exact failure mode of a diagnostic tool.
	verdictSkip checkStatus = "skip"
)

// check is one named diagnostic.
type check struct {
	// Name is a stable JSON field name, documented in docs/cli.md. It is what --json
	// consumers branch on, so it is a key and not a sentence; the sentence is Detail.
	Name string `json:"name"`
	// Status is the verdict.
	Status checkStatus `json:"status"`
	// Detail says what was found, in the terms an operator would repeat.
	Detail string `json:"detail"`
	// Fix is what to do about it. Only set when there is something to do, and it is the
	// reason the tool is one command instead of a wiki: a parent at a terminal at 22:00
	// should not have to look up which of these is the problem and then what the answer is.
	Fix string `json:"fix,omitempty"`
}

func okCheck(name, detail string) check { return check{Name: name, Status: verdictOK, Detail: detail} }
func warnCheck(name, detail, fix string) check {
	return check{Name: name, Status: verdictWarn, Detail: detail, Fix: fix}
}
func failCheck(name, detail, fix string) check {
	return check{Name: name, Status: verdictFail, Detail: detail, Fix: fix}
}
func skipCheck(name, detail string) check {
	return check{Name: name, Status: verdictSkip, Detail: detail}
}

// report is the whole diagnostic run.
type report struct {
	// Checks in the order they ran: cheap and structural first, so a broken install reports
	// its cause before the per-user detail that depends on it.
	Checks []check `json:"checks"`
	// Failed names the checks that returned fail. It is duplicated out of Checks on purpose:
	// the ticket asks that --json name the failing check, and making every consumer re-filter
	// the array is how that requirement gets implemented differently in each consumer.
	Failed []string `json:"failed"`
	// Warned is the same for warn.
	Warned []string `json:"warned,omitempty"`
	// GeneratedAt is when the checks ran, so a saved report can be recognised as stale.
	GeneratedAt time.Time `json:"generatedAt"`
}

// add appends a check and keeps the failure lists in step with it.
//
// The lists are maintained here rather than derived at print time so the text and JSON
// renderings cannot disagree about what failed — the two paths have to agree, because a
// parent reading one and an operator scripting the other are the same incident.
func (r *report) add(cs ...check) {
	for _, c := range cs {
		r.Checks = append(r.Checks, c)
		switch c.Status {
		case verdictFail:
			r.Failed = append(r.Failed, c.Name)
		case verdictWarn:
			r.Warned = append(r.Warned, c.Name)
		}
	}
}

// writeJSON renders the report.
func (r *report) writeJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// writeTextErr renders the report for a human, and surfaces the write error.
//
// One line per check, prefixed by the verdict, with the fix indented under the failure that
// needs it. A check that passed still gets a line: "config: ok, mode 0600" is evidence, and
// a tool that only prints problems leaves the operator wondering whether it looked.
//
// The error has to come out of the LAST write, not the first: a partial report followed by
// exit 0 is the failure this command cannot have.
//
// There used to be a second, error-ignoring copy of this layout next to it. Nothing called
// it -- `doctor` has always gone through this function -- and coverage reported it at 0%
// rather than reporting it as absent, which is how a duplicated renderer survives: the two
// copies drift on the first edit to either, and the one a reader assumes is the real one is
// the one nobody runs.
func (r *report) writeTextErr(w io.Writer) error {
	for _, c := range r.Checks {
		if _, err := fmt.Fprintf(w, "%-6s %-22s %s\n", c.Status, c.Name, c.Detail); err != nil {
			return err
		}
		if c.Fix != "" {
			if _, err := fmt.Fprintf(w, "       fix: %s\n", c.Fix); err != nil {
				return err
			}
		}
	}
	if len(r.Failed) == 0 && len(r.Warned) == 0 {
		_, err := fmt.Fprintln(w, "\nnothing failed")
		return err
	}
	summary := fmt.Sprintf("\n%d failed, %d warned", len(r.Failed), len(r.Warned))
	if len(r.Failed) > 0 {
		summary += ": " + strings.Join(r.Failed, ", ")
	}
	_, err := fmt.Fprintln(w, summary)
	return err
}

// checkRoot reports whether the process has the privilege the daemon needs.
//
// Doctor is meant to be runnable by an administrator who is not root — that is the person
// diagnosing the machine — so a non-root run is not a failure of the machine. It is reported
// as a warning with what it invalidates, because several of the other checks silently answer
// "cannot tell" without it.
func checkRoot(uid func() int) check {
	if uid() == 0 {
		return okCheck("root", "running as root, so the daemon can lock any session")
	}
	return warnCheck("root",
		fmt.Sprintf("running as uid %d, not root", uid()),
		"run `sudo plock doctor` for the checks that need to read other users' sessions; the daemon itself must run as root")
}

// checkConfigPath reports the document's existence, ownership and mode.
//
// The mode matters more than it looks: the config holds the administrator password verifier
// and every child's schedule, and a group- or world-readable file is a child being able to
// read the schedule they are working around.
func checkConfigPath(path string, stat func(string) (os.FileInfo, error)) check {
	fi, err := stat(path)
	if err != nil {
		return failCheck("config.file",
			fmt.Sprintf("cannot read %s: %v", path, err),
			"create it with `plock init`, or point --config at the file the daemon uses")
	}
	if !fi.Mode().IsRegular() {
		return failCheck("config.file",
			fmt.Sprintf("%s is not a regular file (mode %s)", path, fi.Mode()),
			"the daemon refuses anything but a regular file at this path")
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		return failCheck("config.file",
			fmt.Sprintf("%s is mode %04o; the document holds the admin credential and every schedule", path, perm),
			fmt.Sprintf("run: chmod %03o %s", perm&0o700, path))
	}
	// Owner is reported rather than required. A file owned by root read by a non-root doctor
	// is a normal thing to do; a file owned by the child is not, and saying the uid out loud
	// is how that is spotted without the tool guessing.
	return okCheck("config.file", fmt.Sprintf("%s, mode %04o, uid %d", path, fi.Mode().Perm(), fileOwnerUID(fi)))
}

// checkConfigWritable reports whether the daemon and the CLI can save the document.
//
// A read-only mount or a full filesystem is otherwise discovered at the moment a parent
// clicks Save in the web UI and the form says nothing useful.
func checkConfigWritable(path string, access func(string, uint32) error) check {
	// W_OK on the file, not the directory: config.Save writes a temporary file in the
	// directory and renames it, so the directory has to be writable too. Both are checked
	// because they fail independently — a read-only bind mount of the file over a writable
	// directory is a real container layout.
	if err := access(path, 2); err != nil {
		return failCheck("config.writable",
			fmt.Sprintf("%s is not writable: %v", path, err),
			"the daemon cannot persist an override or a schedule change")
	}
	if err := access(filepath.Dir(path), 2); err != nil {
		return failCheck("config.writable",
			fmt.Sprintf("the directory holding %s is not writable: %v", filepath.Dir(path), err),
			"config.Save writes a temporary file beside the document and renames it over it")
	}
	return okCheck("config.writable", fmt.Sprintf("%s and its directory are writable", path))
}

// checkEnforcementSwitch reports the master switch.
//
// This is the check that answers most "it is not working" reports, and it is a warning
// rather than a failure because turning enforcement off is a legitimate thing to do — on
// a holiday, deliberately. What matters is that the tool says so instead of leaving the
// operator to find the one key in the file.
func checkEnforcementSwitch(enabled bool) check {
	if enabled {
		return okCheck("enforcement", "the master switch is on")
	}
	return warnCheck("enforcement",
		"settings.enforcement is false, so no account is locked out no matter what the schedules say",
		"set settings.enforcement true in the config, or `plock user enable <user>` if only one account was disabled")
}

// checkPollInterval reports the tick length.
//
// Bounded from above by the config's own ceiling, and reported here because a long interval
// is indistinguishable from a broken lockout from the child's side: the screen does lock,
// just not for several minutes after the window opens.
func checkPollInterval(seconds int) check {
	switch {
	case seconds <= 0:
		return failCheck("pollInterval",
			fmt.Sprintf("settings.pollSeconds is %d, which the daemon cannot use as an interval", seconds),
			"set settings.pollSeconds to 30")
	case seconds > 300:
		return warnCheck("pollInterval",
			fmt.Sprintf("settings.pollSeconds is %d, so a window opening is noticed up to %s late", seconds, (time.Duration(seconds)*time.Second).String()),
			"30 is the default; a longer interval is a deliberate trade of response for wakeups")
	default:
		return okCheck("pollInterval", fmt.Sprintf("every %ds", seconds))
	}
}

// checkTLS reports the transport the admin UI would come up on.
//
// An expired certificate is the failure mode that looks like a network problem: the daemon
// is up, the port answers, and the browser refuses to say why in terms a parent can act on.
//
// `pair` is the other half of the question, and the reason it is a parameter rather than an
// assumption: production hands it `web.LoadCustom`, the same function `web.New` calls at
// start-up, so doctor and start-up cannot give different answers about one document. Before #162
// this check took the certificate alone, which meant a config naming certificate A and key B —
// the exact mistake the settings form now refuses to save — reported `tls | ok` with a sentence
// byte-identical to a matching pair, while the daemon refused to boot on precisely that pair. Two
// independent answers to one question is the bug. The `cmd.json` rows that drop the key argument
// are what stop the signature quietly going back to one path.
func checkTLS(mode, certFile, keyFile string, now time.Time,
	load func(string) (time.Time, time.Time, error), pair func(cert, key string) error) check {
	switch mode {
	case "off":
		return warnCheck("tls",
			`settings.tls is "off", so the admin password and the session cookie cross the network in clear text`,
			`set settings.tls to "auto" to get a self-signed certificate, or "custom" with a real one`)
	case "", "auto":
		return okCheck("tls", `settings.tls is "auto"; a self-signed certificate is minted and renewed at startup`)
	case "custom":
	default:
		return failCheck("tls", fmt.Sprintf("settings.tls is %q, which the server does not recognise", mode),
			`use "auto", "off" or "custom"`)
	}
	if strings.TrimSpace(certFile) == "" {
		return failCheck("tls", `settings.tls is "custom" but settings.certFile is empty`,
			"point settings.certFile at the certificate, or use \"auto\"")
	}
	if strings.TrimSpace(keyFile) == "" {
		return failCheck("tls", `settings.tls is "custom" but settings.keyFile is empty`,
			`point settings.keyFile at the private key that goes with settings.certFile, or use "auto"`)
	}
	// The daemon's own question, asked with the daemon's own loader, and asked first: it is the
	// failure that stops the process. An operator whose admin UI never came up needs "these two are
	// not a pair", not a validity window read off one half of it while the other half is unopened.
	if err := pair(certFile, keyFile); err != nil {
		return failCheck("tls",
			fmt.Sprintf("the daemon will not start with %s and %s: %v", certFile, keyFile, err),
			`web.New refuses this pair at start-up, so the admin UI never comes up; doctor asks the `+
				`same loader rather than answering the question separately`)
	}
	notBefore, notAfter, err := load(certFile)
	if err != nil {
		return failCheck("tls", fmt.Sprintf("cannot read settings.certFile %s: %v", certFile, err),
			"the server refuses to start rather than serving a handshake it cannot complete")
	}
	switch {
	case now.Before(notBefore):
		return failCheck("tls",
			fmt.Sprintf("%s is not valid until %s", certFile, notBefore.Format(time.RFC3339)),
			"the handshake fails until then")
	case now.After(notAfter):
		return failCheck("tls",
			fmt.Sprintf("%s expired on %s", certFile, notAfter.Format(time.RFC3339)),
			"renew it; the browser will refuse the page and the daemon's log will not say why")
	case now.Sub(notAfter) > -14*24*time.Hour:
		return warnCheck("tls",
			fmt.Sprintf("%s expires on %s", certFile, notAfter.Format(time.RFC3339)),
			"renew it before then")
	default:
		return okCheck("tls", fmt.Sprintf("%s is valid until %s", certFile, notAfter.Format(time.RFC3339)))
	}
}

// checkDaemonRunning asks the init system about the service.
//
// It is a skip, not a failure, when the machine has no systemd to ask: the answer an
// operator needs is "I could not check", and reporting a failure here would send them to
// look for a broken service on a box where the daemon is running under a supervisor this
// check has never heard of.
func checkDaemonRunning(isActive func() (state string, known bool, err error)) check {
	state, known, err := isActive()
	if !known {
		return skipCheck("daemon", "no systemd to ask, so whether the daemon is running was not checked")
	}
	if err != nil {
		return warnCheck("daemon", fmt.Sprintf("could not ask systemd: %v", err),
			"check the service by hand")
	}
	switch state {
	case "active":
		return okCheck("daemon", "the plock service is active")
	case "activating":
		return warnCheck("daemon", "the plock service is activating", "re-run this in a few seconds")
	default:
		return failCheck("daemon", fmt.Sprintf("the plock service is %s", state),
			"start it: systemctl start plock && systemctl enable plock")
	}
}

// checkDaemonSlice asks the question every other check in this report quietly assumes:
// is the daemon running somewhere that lets it lock the account in front of it?
//
// The self-session rule (#67, docs/security.md §4) makes a daemon started from inside a
// login session decline that session's account for as long as it lives, and the CLI has no
// way to see that after the fact -- `status` cannot read the daemon's cgroup, so it stays
// silent (§8). Doctor is where "enforcement is not happening" gets investigated, so the
// deployment shape belongs here, and this is the one place the rule can be stated to someone
// who never reads the startup stderr.
//
// It reports the shape rather than the daemon's cgroup on purpose. With no service running
// there is no daemon to inspect, and the only case a human can fix by hand is the one where
// they are about to start one from a desktop terminal -- which is exactly where this command
// gets typed.
func checkDaemonSlice(isActive func() (state string, known bool, err error), self func() platform.SelfLocation, daemon func() daemonState) check {
	state, known, err := isActive()
	if !known {
		return skipCheck("daemon.slice",
			"not checked: there is no systemd to ask, so whether a daemon here runs inside a login session is unknown")
	}
	if err != nil {
		return warnCheck("daemon.slice", fmt.Sprintf("could not ask systemd whether the service is running: %v", err),
			"check by hand whether the daemon was started from a login session")
	}
	// A service the system manager is running sits outside every user's session, which is
	// the whole point of the unit in deploy/plock.service: it is the deployment where the
	// rule is inert and every account is enforced.
	//
	// That used to be concluded from `is-active` alone, which does not establish it. The
	// probe inherits SYSTEMD_ADDRESS on purpose, so a doctor run against somebody's user
	// manager sees that manager's unit as active — and would have asserted "outside every
	// login session, no account is declined" about a daemon sitting in one. The claim is
	// read off the running process now, from the same probe `status` marks its rows with,
	// so the two commands cannot answer from different sources.
	if state == "active" || state == "activating" {
		switch d := daemon(); {
		case !d.LocationKnown:
			return warnCheck("daemon.slice",
				"the service is active, but its cgroup could not be read, so this cannot say whether it declines any account",
				"read the cgroup of `systemctl show plock --property=MainPID` by hand")
		case d.Location.InUserSlice:
			return warnCheck("daemon.slice",
				fmt.Sprintf("the service is running inside uid %d's login session: that account is never locked, because "+
					"locking the screen behind the daemon means it can no longer be stopped", d.Location.UID),
				"install the unit in deploy/plock.service, which runs the daemon in system.slice where it declines nobody")
		default:
			return okCheck("daemon.slice",
				"the service is run by the system manager, outside every login session, so no account is declined")
		}
	}
	selfLoc := self()
	if !selfLoc.InUserSlice {
		return okCheck("daemon.slice",
			"this shell is outside every login session, so a daemon started from it declines no account")
	}
	return warnCheck("daemon.slice",
		fmt.Sprintf("no plock service is running, and this shell is inside uid %d's login session: a daemon started "+
			"from here will NOT lock that account, because locking the screen behind it means it can no longer be "+
			"stopped", selfLoc.UID),
		"install the unit in deploy/plock.service, which runs the daemon in system.slice where it declines nobody")
}

// unenforced is one account that should be locked out right now and is not.
type unenforced struct {
	Username string `json:"username"`
	Rule     string `json:"rule"`
	EndsAt   string `json:"endsAt"`
	Reason   string `json:"reason"`
}

// checkUnenforced is the ticket's "it is not working" signal.
//
// Everything else in doctor asks whether the machinery is present. This asks whether the
// machinery is achieving anything, which is the question actually being asked by whoever
// runs doctor: the schedules are configured, the daemon is running, and the child is
// nevertheless at the desktop.
//
// An account is reported when its schedule says it is inside a window, it has a session this
// build would lock, and that session is not locked.
func checkUnenforced(now time.Time, census sessionCensus) check {
	c := checkCensus(now, census.People)
	if census.Missing == nil {
		return c
	}
	// #168: this verdict is a claim about every managed account, and the census it was
	// computed from is missing rows. "No account is inside a window with an unlocked screen"
	// then means "no account the daemon could read", which is a different sentence — so the
	// gap goes on the detail line instead of staying in the daemon's log, where nobody
	// reading doctor's output would meet it. An account that could not be read has not been
	// found to be safe.
	if c.Status != verdictOK {
		c.Detail = fmt.Sprintf("%s (the session listing was also incomplete: %v)", c.Detail, census.Missing)
		return c
	}
	return warnCheck("unenforced",
		fmt.Sprintf("no account the daemon could read is inside a window with an unlocked screen, "+
			"and the listing was incomplete: %v", census.Missing),
		"an account whose session could not be read has not been checked at all; run `loginctl list-sessions` "+
			"and read the daemon's log before treating the line above as all of the machine")
}

// checkCensus is the verdict on the accounts a session census could place. It is separate
// from checkUnenforced because a census can be incomplete, and the same verdict computed
// over a partial census needs saying differently (#168).
func checkCensus(now time.Time, accounts []unenforcedCandidate) check {
	const name = "unenforced"
	var bad []unenforced
	var dangling []string
	for _, a := range accounts {
		if a.DanglingProfile != "" {
			dangling = append(dangling, a.Username)
			continue
		}
		if !a.InWindow {
			continue
		}
		switch {
		case !a.Managed:
			// Deliberately not enforced, so an unlocked screen is the configured behaviour.
			// Reported as a warning rather than silently skipped, because the operator has to
			// be able to tell "disabled on purpose" from "disabled by the bug I am looking for".
			continue
		case !a.HasSession:
			// No session to lock. Nothing is wrong; the child is simply not logged in.
			continue
		case a.Locked:
			continue
		case !a.Lockable:
			bad = append(bad, unenforced{
				Username: a.Username, Rule: a.Rule, EndsAt: a.EndsAt,
				Reason: "the session is not one this build can lock (no seat, remote, or not a desktop session)",
			})
		default:
			bad = append(bad, unenforced{
				Username: a.Username, Rule: a.Rule, EndsAt: a.EndsAt,
				Reason: "inside a window and the screen is unlocked",
			})
		}
	}
	sort.Strings(dangling)
	if len(dangling) > 0 {
		// Reported separately from the in-window case: these accounts are never locked out,
		// not just unlocked at this moment, and the fix is a reassignment rather than a
		// look at the daemon's log.
		return check{
			Name:   name,
			Status: verdictFail,
			Detail: fmt.Sprintf("%d enabled account(s) name a profile that does not exist and are "+
				"therefore never locked out: %s", len(dangling), strings.Join(dangling, ", ")),
			Fix: "assign them to a real profile: plock user assign <user> <profile>",
		}
	}
	if len(bad) > 0 {
		sort.Slice(bad, func(i, j int) bool { return bad[i].Username < bad[j].Username })
		// Each account carries its own reason, and the reason is rendered because the two
		// shapes have different fixes: "inside a window and unlocked" means go read the
		// daemon's log, while "not a session this build can lock" means the session itself is
		// the problem and no log will explain it. A summary that named only the accounts made
		// the distinction invisible -- the per-account Reason was computed and then dropped.
		lines := make([]string, 0, len(bad))
		for _, b := range bad {
			lines = append(lines, fmt.Sprintf("%s: %s", b.Username, b.Reason))
		}
		return check{
			Name:   name,
			Status: verdictFail,
			Detail: fmt.Sprintf("%d account(s) are in a lockout window and not locked: %s",
				len(bad), strings.Join(lines, "; ")),
			Fix: "look at the daemon's log for the lock attempt; `plock doctor --try <user>` locks one now and says which backend answered",
		}
	}
	return okCheck(name, "no account is inside a window with an unlocked screen")
}

// unenforcedCandidate is one account's state, assembled by the caller so the check is a pure
// function of data the tests can state.
type unenforcedCandidate struct {
	Username string
	Managed  bool
	// DanglingProfile is the profileID that does not resolve, or empty. An account enabled
	// for enforcement with no readable schedule is not locked out by anything, which is the
	// same outcome as a disabled account arrived at by accident instead of on purpose.
	DanglingProfile string
	InWindow        bool
	Rule            string
	EndsAt          string
	HasSession      bool
	Lockable        bool
	Locked          bool
}

// unenforcedCandidates joins three separate facts about each managed account: what its
// schedule says now, and what the machine says about its sessions.
//
// The join is here rather than inside checkUnenforced because each fact comes from a
// different place -- the evaluator, the config, and the session listing -- and a check that
// reached for all three could only be tested by arranging all three. As a pure function of
// this slice, the check's logic is testable on its own and the join is testable on its own.
//
// A session-listing failure is reported as "no session", not as a failure. The listing can
// be empty because the child is not logged in, and doctor cannot tell that apart from a
// backend that cannot see sessions; the `backends` check is where a broken backend shows up,
// and inventing an "unlocked" verdict from an error would be the tool manufacturing the
// alarm it was asked to investigate.
func unenforcedCandidates(cfg *config.Config, sessions sessionSource) sessionCensus {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// A listing that is only partly readable is used as far as it reaches (#168). This is
	// where the code used to write `live = nil`, which turned "four of five sessions read"
	// into "nobody is logged in": the account whose session could not be read was then
	// reported as having no session at all, the most reassuring wrong answer this tool can
	// give. The rows that read are kept and the shortfall travels with them.
	live, missing := sessions.Sessions(ctx)
	byUser := map[string]platform.Session{}
	for _, s := range live {
		// The first session wins when a uid owns several. A second desktop session for one
		// account is rare enough that averaging them would be inventing a policy; what must
		// not happen is the loop's last write deciding, because then the answer depends on
		// the order loginctl happened to print.
		if _, seen := byUser[s.Username]; !seen {
			byUser[s.Username] = s
		}
	}

	profiles := map[string]*config.Profile{}
	for i := range cfg.Profiles {
		profiles[cfg.Profiles[i].ID] = &cfg.Profiles[i]
	}

	now := time.Now()
	out := make([]unenforcedCandidate, 0, len(cfg.Users))
	for _, u := range cfg.Users {
		c := unenforcedCandidate{Username: u.Username, Managed: u.Enabled}
		if !u.Enabled {
			// A disabled account has no schedule to evaluate, and evaluating one anyway would
			// report a window that the daemon deliberately ignores.
			out = append(out, c)
			continue
		}
		prof, ok := profiles[u.ProfileID]
		if !ok {
			// A dangling assignment is its own finding, and it is a fail: the account is
			// configured to be enforced and has nothing to enforce. `plock user list` says the
			// same thing, and the two have to agree. It is reported through DanglingProfile
			// rather than folded into InWindow, because "no schedule at all" and "in a window"
			// are different sentences with different fixes.
			c.DanglingProfile = u.ProfileID
			out = append(out, c)
			continue
		}
		v := schedule.Evaluate(prof, now)
		c.InWindow = v.Active
		if v.Rule != nil {
			c.Rule = v.Rule.Name
			if c.Rule == "" {
				c.Rule = v.Rule.ID
			}
		}
		if !v.EndsAt.IsZero() {
			c.EndsAt = v.EndsAt.Format(time.RFC3339)
		}
		if s, seen := byUser[u.Username]; seen {
			c.HasSession = true
			c.Lockable = platform.IsLockable(s)
			c.Locked = s.Locked
		}
		out = append(out, c)
	}
	return sessionCensus{People: out, Missing: missing}
}

// sessionCensus is one pass over the session table: the accounts it could place in front of
// a screen, and the reason the table was not read completely.
//
// The error travels with the rows instead of replacing them (#168). A census that read four
// sessions out of five supports a narrower claim, not a blank one, and the two ways to be
// wrong about which one you have are "nobody is logged in" and "nobody needs locking".
type sessionCensus struct {
	People  []unenforcedCandidate
	Missing error
}

// runDoctorTry is `plock doctor --try <user>`: the one path that locks a real screen.
//
// It exists because "would the chain lock this machine?" is not answerable by probing. The
// probe says a backend is installed; only an actual lock says whether the desktop honours
// it, and on Plasma that is exactly the difference between logind (asked first, ignores the
// signal, exits 0) and the desktop backend (the one that lands). Without --try, doctor can
// report a healthy-looking chain on a machine that never locks.
//
// The answer is reported per backend rather than as a single success, because "which backend
// won" is the fact that tells you something about the machine.
func runDoctorTry(stdout, stderr io.Writer, chain chainView, user string, rep *report) int {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	sessions, err := chain.Sessions(ctx)
	if err != nil {
		// Say it, then use what came back (#168). --try is the command run when the question
		// is "will this screen actually lock", and a listing that read three sessions out of
		// four still names the one being asked about. Only a read with nothing in it ends the
		// command; the shortfall stays on stderr, so the exit status cannot be read as a clean
		// pass over a machine that was only partly seen.
		fmt.Fprintf(stderr, "plock doctor --try: %v\n", err)
		if len(sessions) == 0 {
			return exitFailure
		}
	}
	var target *platform.Session
	for i := range sessions {
		if sessions[i].Username == user {
			target = &sessions[i]
			break
		}
	}
	if target == nil {
		// Naming the account and saying nothing was locked is the whole contract here. A
		// typo'd username that fell through to "locked nobody, looks fine" would be worse than
		// an error.
		fmt.Fprintf(stderr, "plock doctor --try: no session found for %q; nothing was locked\n", user)
		return exitFailure
	}
	if !platform.IsLockable(*target) {
		fmt.Fprintf(stderr, "plock doctor --try: %q has a session this build cannot lock (%s)\n",
			user, sessionWhyUnlockable(*target))
		return exitFailure
	}

	fmt.Fprintf(stdout, "locking %q's session (%s) now\n", user, target.ID)
	if err := chain.Lock(ctx, *target); err != nil {
		// The distinction between "nobody confirmed" and "nobody was reached" is the whole
		// diagnostic value of this flag, so it is said out loud rather than folded into one
		// failure line.
		fmt.Fprintf(stderr, "plock doctor --try: %v\n", err)
		if errors.Is(err, platform.ErrUnconfirmed) {
			fmt.Fprintf(stderr,
				"\nEvery backend was asked and none could confirm the lock. That is the\n"+
					"\"the daemon is running and the screen never locks\" case: the commands run,\n"+
					"the desktop ignores them, and nothing on the machine reports it.\n")
		}
		return exitFailure
	}
	fmt.Fprintf(stdout, "locked: the screen is confirmed locked\n")
	return exitOK
}

// sessionWhyUnlockable says which rule excluded a session, in the order the rules are
// applied. A user told only "cannot be locked" has no way to tell a remote session from a
// greeter from a raw VT, and the three have different answers.
func sessionWhyUnlockable(s platform.Session) string {
	switch {
	case s.Remote:
		return "remote"
	case s.Seat == "":
		return "no seat"
	case s.Type == "":
		return "no session type"
	case s.Class != "" && s.Class != "user":
		return "class " + s.Class
	default:
		return "not a lockable desktop session"
	}
}
