package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// tlsModes are the settings the web server knows how to act on: generate a
// self-signed certificate, serve plain HTTP, or use operator-supplied paths.
var tlsModes = []string{"auto", "off", "custom"}

// MinPollSeconds is the shortest tick the daemon will accept.
//
// Exported because deploy/run-dev.sh has to refuse a too-small PLOCK_DEV_POLL before it
// writes a dev root, and a second copy of this number in a shell script is how the two
// come to disagree. The script's comment names this constant; TestTheDevScriptsPollFloor
// is what makes that naming more than a hope.
const MinPollSeconds = 5

// MaxPollSeconds is a day. Anything longer is a mistake rather than a policy:
// the enforcement loop would not notice a schedule change for over that long.
const MaxPollSeconds = 86400

// ErrInvalid marks a refusal produced by Validate rather than by the filesystem, so a
// caller can tell "what you asked for is not a valid document" from "the disk said no".
// The two need opposite answers on the web UI: one is a 4xx that names what to change, the
// other is a 503 that says try again. Without a sentinel the only way to tell them apart is
// to match message text, which this repo's own rule forbids.
var ErrInvalid = errors.New("configuration is not valid")

// Validate reports every problem it finds, not just the first. The web UI shows
// this list to a parent who has just submitted a form, and making them fix one
// error per round trip is a poor way to spend an evening.
//
// Each message names the field it is about, because the UI has nothing else to
// point at.
//
// Save does not call this. The daemon has to keep enforcing whatever is already
// on disk even if a hand edit made it unsound, so refusing to write is the
// responsibility of whoever produced the change.
func (c *Config) Validate() error { return c.validate(false) }

// ValidateForEdit is Validate without the administrator credential.
//
// The credential is the only requirement in the document that a profile or user edit does
// not author, and it is the one that a machine can legitimately lack: the credential exists
// to log in to the web UI, and everything in this file is what you do *before* the web UI
// is reachable, over SSH, on a headless box.
//
// Requiring it on an edit is a fail-open. A parent who has not yet run `plock init` on a
// fresh install would be refused when adding their child, and a child who cannot be added
// is not enforced. The command that refuses to write because of a field it never touched is
// the same mistake as `plock unlock` refusing over the admin credential, and it is the third
// instance of that shape in this package.
//
// Validate remains the whole-document check, and `plock init` still insists on the
// credential: setting up the machine is where that requirement belongs.
func (c *Config) ValidateForEdit() error { return c.validate(true) }

func (c *Config) validate(skipAdmin bool) error {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	if c.Version != CurrentVersion {
		add("version: got %d, this build writes %d", c.Version, CurrentVersion)
	}

	if !skipAdmin {
		if strings.TrimSpace(c.Admin.Username) == "" {
			add("admin.username: an administrator account is required")
		}
		// An empty verifier is the one ambiguity that decides what the login form
		// accepts, so it is reported rather than left for ticket #10 to discover at
		// runtime: every login would fail, with nothing on disk to check against.
		if strings.TrimSpace(c.Admin.Password) == "" {
			add("admin.password: no password verifier is stored, so the web UI cannot authenticate anyone")
		}
	}

	if strings.TrimSpace(c.Settings.Listen) == "" {
		add("settings.listen: must not be empty")
	}
	if c.Settings.PollSeconds < 5 {
		add("settings.pollSeconds: %d is below the minimum of %d", c.Settings.PollSeconds, MinPollSeconds)
	} else if c.Settings.PollSeconds > MaxPollSeconds {
		// Bounded from above because the consumer multiplies it into a
		// time.Duration, and an unbounded value wraps to a negative interval.
		add("settings.pollSeconds: %d is above the maximum of %d", c.Settings.PollSeconds, MaxPollSeconds)
	}
	if !slices.Contains(tlsModes, c.Settings.TLS) {
		add("settings.tls: %q is not one of %s", c.Settings.TLS, strings.Join(tlsModes, ", "))
	}
	if c.Settings.TLS == "custom" && (c.Settings.CertFile == "" || c.Settings.KeyFile == "") {
		add("settings.certFile and settings.keyFile: both are required when tls is \"custom\"")
	}
	if len(c.Settings.LockBackends) == 0 {
		add("settings.lockBackends: at least one lock backend is required")
	}

	// The command backend is the one configured backend that runs a program this
	// binary does not ship, so its two fields are checked here rather than only at
	// construction. A document that names "command" without a path would otherwise
	// load clean, pass doctor, and then fail on the first tick -- and a *relative*
	// path is worse than a missing one, because it looks specific: it would be
	// resolved against whatever working directory the daemon happened to start in,
	// which under systemd is not a directory anyone chose. #53 refused a bare name
	// for the shipped lock binaries for the same reason; this is that rule applied
	// to the binary an operator names.
	if wants := slices.Contains(c.Settings.LockBackends, "command"); wants {
		path := strings.TrimSpace(c.Settings.LockCommandPath)
		switch {
		case path == "":
			add("settings.lockCommandPath: required when lockBackends names \"command\"")
		case !strings.HasPrefix(path, "/"):
			add("settings.lockCommandPath: %q must be an absolute path; a relative or bare name "+
				"is resolved against the daemon's working directory, which is not a thing an "+
				"operator controls", c.Settings.LockCommandPath)
		}
	} else if c.Settings.LockCommandPath != "" || len(c.Settings.LockCommandArgv) > 0 {
		// Configured but never used is its own bug: the operator believes a command
		// backend is in the chain, and doctor will report the three backends that are.
		add("settings.lockCommandPath: set but lockBackends does not name \"command\", so nothing " +
			"will ever run it")
	}

	profileIDs := make(map[string]bool, len(c.Profiles))
	for i, p := range c.Profiles {
		label := fmt.Sprintf("profiles[%d]", i)
		if p.Name != "" {
			label = fmt.Sprintf("profiles[%d] (%s)", i, p.Name)
		}

		if strings.TrimSpace(p.ID) == "" {
			add("%s.id: must not be empty", label)
		} else if profileIDs[p.ID] {
			add("profiles: id %q is used more than once", p.ID)
		} else {
			profileIDs[p.ID] = true
		}

		// LoadLocation("") answers UTC rather than failing, so an unset
		// timezone would silently evaluate every rule in the machine's zone.
		if p.Timezone == "" {
			add("%s.timezone: must not be empty", label)
		} else if _, err := time.LoadLocation(p.Timezone); err != nil {
			add("%s.timezone: %q is not a usable timezone: %v", label, p.Timezone, err)
		}

		ruleIDs := make(map[string]bool, len(p.Rules))
		for j, r := range p.Rules {
			rl := fmt.Sprintf("%s.rules[%d]", label, j)
			if r.Name != "" {
				rl = fmt.Sprintf("%s.rules[%d] (%s)", label, j, r.Name)
			}

			if strings.TrimSpace(r.ID) == "" {
				add("%s.id: must not be empty", rl)
			} else if ruleIDs[r.ID] {
				add("%s: rule id %q is used more than once in %s", rl, r.ID, label)
			} else {
				ruleIDs[r.ID] = true
			}

			// Both halves of the clock go through ParseClock, the same function the
			// evaluator uses, so the two can never disagree about what is a legal
			// time. Matching the pattern here instead would agree only for as long
			// as nobody touches the one it borrows.
			//
			// An all-day rule is not asked. Its clock ends are not consulted by the
			// evaluator at all, so rejecting "25:00" here would refuse a document
			// that enforces exactly what its author meant, and demanding the times
			// be cleared first would force the form (#10) to know that a checkbox
			// changes which of its own fields are legal.
			if !r.AllDay {
				if _, ok := ParseClock(r.Start); !ok {
					add("%s.start: %q is not HH:MM on a 24-hour clock", rl, r.Start)
				}
				if _, ok := ParseClock(r.End); !ok {
					add("%s.end: %q is not HH:MM on a 24-hour clock", rl, r.End)
				}
			}
			// A window that ends where it starts is empty: the evaluator finds no
			// instant inside it. That is what a parent who typed the same time
			// into both boxes means — "no window" — but it is also the one typo
			// in this file that silently enforces nothing, so it is rejected at
			// save time rather than left to be discovered on a night the child
			// had the run of the machine.
			//
			// The message must not claim this shape locks the account. It does
			// not: an empty window locks out nobody, and telling a parent it
			// locks out everybody would let them approve the typo on purpose.
			// It points at allDay instead, which is the rule they actually meant
			// when they wrote "all day" — and unlike 00:00 to 23:59 it really is
			// all day, because that window is a minute short.
			if r.Start == r.End && !r.AllDay {
				if _, ok := ParseClock(r.Start); ok {
					add("%s.start: %s is also the end time, so the window is empty and enforces nothing; for a rule with no minute free set allDay instead — 00:00 to 23:59 is a minute short",
						rl, r.Start)
				}
			}

			if len(r.Days) == 0 {
				// The consequence is stated because it is the opposite of what it
				// looks like. An empty day list reads as "no day", but the evaluator
				// resolves an unreadable part of a rule toward the lockout, so a rule
				// with no days is enforced on all seven.
				add("%s.days: at least one day is required; as written the rule names no day, and the daemon would enforce it on every day", rl)
			}
			seen := make(map[string]bool, len(r.Days))
			for _, d := range r.Days {
				switch {
				case !IsDay(d):
					add("%s.days: %q is not a weekday name (%s)", rl, d, strings.Join(DayNames, ", "))
				case seen[d]:
					add("%s.days: %q is listed more than once", rl, d)
				default:
					seen[d] = true
				}
			}
		}
	}

	usernames := make(map[string]bool, len(c.Users))
	for i, u := range c.Users {
		label := fmt.Sprintf("users[%d]", i)
		if strings.TrimSpace(u.Username) == "" {
			add("%s.username: must not be empty", label)
		} else if usernames[u.Username] {
			add("users: account %q is listed more than once", u.Username)
		} else {
			usernames[u.Username] = true
		}

		if !profileIDs[u.ProfileID] {
			add("%s.profileID: %q does not name an existing profile", label, u.ProfileID)
		}
	}

	// One override per account is what AddOverride guarantees, but a hand-edited
	// document can hold several, and then the winner is array order. This
	// package's position on a value that silently loses is already on the record
	// with duplicate JSON keys: it is an error, not a precedence rule.
	overrideAccounts := make(map[string]bool, len(c.Overrides))
	for i, o := range c.Overrides {
		label := fmt.Sprintf("overrides[%d]", i)
		if !usernames[o.Username] {
			add("%s.username: %q does not name a configured account, so this override would do nothing",
				label, o.Username)
		} else if overrideAccounts[o.Username] {
			add("overrides: account %q has more than one override; only the first can ever apply", o.Username)
		} else {
			overrideAccounts[o.Username] = true
		}

		if o.Until.IsZero() {
			add("%s.until: must be a date and time", label)
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("%w: %d problem(s): %s", ErrInvalid, len(problems), strings.Join(problems, "; "))
	}
	return nil
}

// ErrProfileNameTaken is the refusal every write path gives when a profile would end up
// labelled the way another one already is.
//
// One sentence rather than three, because the web form and `plock profile edit --name` refusing
// the same thing in different words is how an operator learns that only one of the two paths is
// real. It deliberately does not say "this page": the CLI refuses it too.
var ErrProfileNameTaken = errors.New("another profile on this machine already has that name, " +
	"and profiles are listed by name -- two alike could not be told apart")

// ProfileNameTaken reports whether name is already the label that a profile other than
// exceptID answers to.
//
// It lives next to Validate and is deliberately not inside it. Validate runs on every write,
// so refusing a duplicate there would make a machine that already holds two same-named
// profiles -- possible today through `plock profile add --copy-from`, and possible on any
// config written before any check existed -- fail every subsequent write, including ones that
// have nothing to do with names. That is the wrong shape for a labelling rule: it would stop
// enforcement-adjacent edits over a cosmetic collision, and a config that cannot be written is
// a config whose lockout cannot be lifted. The write paths that can create the pair refuse with
// this predicate, and the profiles page reports a pair that already exists instead.
//
// name is compared as it is printed, so trimmed -- on both sides, and inside this function
// rather than at the call sites. A rule whose callers have to remember to trim is a rule that
// quietly differs by caller, which is the drift the predicate exists to remove. A profile whose name is empty is headed with
// its id, which is the only case in which an id counts as a taken label: refusing an id
// unconditionally would be a rule bigger than its reason, because a profile that does have a
// name never shows its id as a heading, so renaming onto that id disambiguates something.
//
// exceptID is the profile being renamed, so that submitting a form without changing the field
// is not refused as a collision with itself. The create paths pass "".
func ProfileNameTaken(profiles []Profile, name, exceptID string) bool {
	candidate := strings.TrimSpace(name)
	for _, p := range profiles {
		if p.ID == exceptID {
			continue
		}
		label := strings.TrimSpace(p.Name)
		if label == candidate || (label == "" && p.ID == candidate) {
			return true
		}
	}
	return false
}

// AmbiguousProfileNames reports which profiles the profiles page cannot tell apart, keyed by
// profile id.
//
// It answers a state this repo can already be in rather than one it can now reach: a config
// written before the write paths checked, or one edited by hand, can hold two profiles with one
// name. Such a config still loads and still enforces -- refusing it would trade a labelling
// problem for a lockout nobody can lift -- so the pair has to be visible where it hurts, which
// is the page that heads both of them with the same words and offers two delete confirmations
// that name the same thing.
//
// The label is the one the page prints: the trimmed name, or the id when the name is empty.
func AmbiguousProfileNames(profiles []Profile) map[string]bool {
	byLabel := make(map[string][]string, len(profiles))
	for _, p := range profiles {
		label := strings.TrimSpace(p.Name)
		if label == "" {
			label = p.ID
		}
		byLabel[label] = append(byLabel[label], p.ID)
	}
	ambiguous := map[string]bool{}
	for _, ids := range byLabel {
		if len(ids) < 2 {
			continue
		}
		for _, id := range ids {
			ambiguous[id] = true
		}
	}
	return ambiguous
}
