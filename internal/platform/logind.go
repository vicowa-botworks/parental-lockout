package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"strings"
)

// logind is the systemd-logind backend: the one that works on GNOME and most other
// desktops, and the only source of session identity and lock state on this platform.
//
// Every command here was captured from a live systemd 259 on the target platform
// rather than written from the manual, because two of the three details that matter
// are not what the documentation implies:
//
//   - `loginctl list-sessions --output=json` is silently ignored. It prints the
//     plain table and exits 0. The flag that works is --json=short. A backend built
//     on the wrong flag does not fail; it feeds a table to a JSON parser every 30
//     seconds and enforces nothing.
//   - the listing carries no Remote and no LockedHint, so lock state and the
//     remote flag can only come from a per-session show-session call. One call per
//     session is the right trade at a 30-second poll for a handful of sessions.
//   - show-session prints Key=value lines in an order that is neither the requested
//     order nor alphabetical, so they are parsed into a map.
type logind struct{ r Runner }

func init() {
	Register("logind", func(r Runner, _ config.Settings) (Platform, error) {
		if r == nil {
			return nil, fmt.Errorf("logind: no runner")
		}
		return &logind{r: r}, nil
	})
}

func (l *logind) Name() string { return "logind" }

// listEntry is one row of `loginctl list-sessions --json=short`.
//
// Seat and TTY are pointers because logind prints real nulls for them — a session
// with no seat is exactly the manager session the filter has to drop, so "absent"
// has to be distinguishable from "empty string" only insofar as both are dropped.
// They are pointers so the JSON decodes at all: unmarshalling null into a string is
// an error, and an error here would mean seeing no sessions at all.
type listEntry struct {
	Session string  `json:"session"`
	UID     uint32  `json:"uid"`
	User    string  `json:"user"`
	Seat    *string `json:"seat"`
	Leader  int     `json:"leader"`
	Class   string  `json:"class"`
	TTY     *string `json:"tty"`
	Idle    bool    `json:"idle"`
	Since   *string `json:"since"`
}

// Sessions lists the desktop sessions worth locking.
//
// The filter is applied here rather than left to the caller, because every caller of
// this is the enforcement loop, and the loop's default has to be "do not touch the
// admin's SSH session". A caller that genuinely wants the full picture reads it from
// Probe or from the CLI's doctor output.
func (l *logind) Sessions(ctx context.Context) ([]Session, error) {
	entries, err := l.list(ctx)
	if err != nil {
		return nil, err
	}

	var out []Session
	var errs []error
	for _, e := range entries {
		// Cheap pre-filter from the listing alone, so a machine with a dozen
		// non-desktop sessions does not make a dozen pointless calls. class=user
		// with a seat is the desktop session; everything else (manager, greeter)
		// has nothing to lock.
		if !isDesktopEntry(e) {
			continue
		}

		props, err := l.show(ctx, e.Session, sessionProps...)
		if err != nil {
			// A session that disappeared between the listing and the detail call is
			// normal — someone logged out mid-poll. It is not a reason to give up on
			// the sessions that are still there.
			if isUnknownSession(err) {
				continue
			}
			errs = append(errs, fmt.Errorf("session %s: %w", e.Session, err))
			continue
		}

		s := Session{
			ID:       e.Session,
			Username: props.get("Name"),
			UID:      e.UID,
			Class:    e.Class,
			Seat:     seatString(e.Seat),
			Type:     props.get("Type"),
			State:    props.get("State"),
			Remote:   props.flag("Remote"),
			Locked:   props.flag("LockedHint"),
		}
		// The listing's user is the account that owns the session, and the listing is
		// the only place the uid comes from: show-session has no UID property to give
		// (see sessionProps). A uid of 0 here means logind could not resolve the
		// account, which is not fatal — profiles are keyed by username, and the
		// username is a required column that cannot be missing.
		if s.Username == "" {
			s.Username = e.User
		}
		s.Leader = e.Leader
		s.TTY = seatString(e.TTY)
		if !isLockable(s) {
			continue
		}
		out = append(out, s)
	}
	return out, joinErrs(errs...)
}

// Lock asks logind to lock one session. It runs as the daemon (root), which is the
// whole point: logind lets the manager lock any session, so no downgrade and no
// access to the user's bus is needed.
func (l *logind) Lock(ctx context.Context, s Session) error {
	if !isLockable(s) {
		return fmt.Errorf("logind: refusing to lock %s: not a lockable desktop session", s.Describe())
	}
	// The id came from logind itself, but it is about to be an argument to a command
	// that parses leading dashes as options. A session id that starts with one is
	// either a bug upstream or a hostile /run, and either way it must not reach
	// loginctl as an option.
	if !validSessionID(s.ID) {
		return fmt.Errorf("logind: refusing to lock session %q: not a plausible session id", s.ID)
	}
	_, err := l.r.Run(ctx, []string{"loginctl", noAsk, "lock-session", s.ID}, nil, 0)
	if err != nil {
		return fmt.Errorf("logind: %w", err)
	}
	return nil
}

// Probe checks that logind can be talked to. It runs the read-only listing and
// nothing else — in particular it never locks.
// LockConfirmed re-reads LockedHint to see whether the lock took effect.
//
// It has to re-query rather than look at Session.Locked, because that field was read
// before the lock was requested — using it would report the state the daemon was
// trying to change. LockedHint is set by the desktop environment when it locks, so it
// is the one signal that distinguishes "loginctl accepted my request" from "the screen
// is locked", which on Plasma is exactly the difference that matters.
//
// It is a second call per lock attempt. At one lock per session per 30 seconds that is
// cheap, and being wrong here means believing a child's screen is locked when it is
// not.
func (l *logind) LockConfirmed(ctx context.Context, s Session) (bool, error) {
	// No id check here on purpose. show() is the only route to loginctl for a query and
	// it refuses implausible ids itself, so a second guard in front of it can never
	// fire — and a branch nothing can reach is a branch nobody ever tests, which is
	// where a wrong stops being visible.
	props, err := l.show(ctx, s.ID, "LockedHint")
	if err != nil {
		return false, err
	}
	return props.flag("LockedHint"), nil
}

func (l *logind) Probe(ctx context.Context) []BackendStatus {
	entries, err := l.list(ctx)
	if err != nil {
		return []BackendStatus{{Name: l.Name(), Detail: err.Error()}}
	}
	status := BackendStatus{
		Name:      l.Name(),
		Available: true,
		Detail:    fmt.Sprintf("%d session(s) listed", len(entries)),
	}
	// Say which properties this loginctl did not answer, because on the wire absence
	// is invisible: logind returns exit 0 with no line for a property it does not
	// know, so an unimplemented property cannot be told apart from an unset one. Two
	// of them change what the daemon can actually do:
	//
	//   LockedHint absent -- nothing can ever confirm a lock, so no backend can end
	//       the chain and every one of them is called on every 30-second poll.
	//   Remote absent -- a remote session is kept out only by the seat requirement,
	//       which happens to work today but is not the check that was designed.
	//
	// This is a read-only show-session on one session, never a lock. It is skipped
	// when there is nothing to ask about, and a session that vanishes between the
	// listing and this call is not a reason to stop reporting the probe.
	// Ask about a session the daemon would actually try to lock. entries[0] is often a
	// class=manager row -- no seat, Type=unspecified, never a lock candidate -- and on
	// some boxes it is the only row besides a greeter. Blaming a property's absence on
	// the first row therefore reports "this loginctl does not report X" on the evidence
	// of a session whose class may be exactly why X is missing, while the class the
	// daemon does lock would have answered it. A machine with no lockable session still
	// gets asked about, so the report does not go quiet.
	if target := probeTarget(entries); target != "" {
		if p, err := l.show(ctx, target, sessionProps...); err == nil {
			var absent []string
			for _, k := range sessionProps {
				if _, ok := p[k]; !ok {
					absent = append(absent, k)
				}
			}
			if len(absent) > 0 {
				status.Detail = fmt.Sprintf("%s; this loginctl does not report: %s",
					status.Detail, strings.Join(absent, ", "))
			}
		}
	}
	return []BackendStatus{status}
}

// probeTarget names the session the property-presence probe should ask about: the first
// row that is a lock candidate, or the first row at all when none is.
//
// The "or the first row at all" half is not a courtesy. A loginctl that lists only
// manager and greeter rows still has an opinion about whether LockedHint exists, and
// returning "" here would turn a real finding into no finding -- the failure direction
// this package keeps refusing.
func probeTarget(entries []listEntry) string {
	for _, e := range entries {
		if isDesktopEntry(e) {
			return e.Session
		}
	}
	// Nothing is a lock candidate. Ask about any row that can be asked about anyway: the
	// question the probe answers is about what this loginctl reports, not about the
	// session it happens to ask, and a headless box with only manager rows still has to
	// be told whether Type is reportable at all.
	//
	// "any row" is deliberately not "the first row". A JSON row with a missing or empty
	// `session` key parses without complaint, and returning it means the caller's
	// `target != ""` check skips the probe entirely — the silence this package keeps
	// refusing, arriving through a fallback that looked like a safety net.
	for _, e := range entries {
		if e.Session != "" {
			return e.Session
		}
	}
	return ""
}

// isDesktopEntry is the cheap pre-filter that decides which listed sessions are worth a
// detail call: class=user with a seat is the desktop session, and everything else
// (manager, greeter) has nothing to lock.
//
// It is a function rather than an inline condition because the property-presence probe
// has to agree with it. Asking a non-candidate what the daemon would have seen is how a
// manager row ends up testifying about a property only class=user carries.
func isDesktopEntry(e listEntry) bool {
	return e.Class == "user" && e.Seat != nil && *e.Seat != "" && e.Session != ""
}

// list gets the session rows, preferring JSON and falling back to the table.
//
// Two paths reach the table, and both are real: an older loginctl that rejects
// --json=short outright, and a loginctl that accepts the flag and ignores it — which
// is what 259 does to --output=json, and the reason the fixture for the fallback is a
// captured table rather than an invented one.
func (l *logind) list(ctx context.Context) ([]listEntry, error) {
	out, err := l.r.Run(ctx, []string{"loginctl", noAsk, "list-sessions", "--json=short"}, nil, 0)
	if err == nil {
		if entries, jsonErr := parseSessionJSON(out); jsonErr == nil {
			return entries, nil
		}
		// The flag was accepted and ignored, or the output is truncated. Fall through
		// and read what came back as a table before spending another call.
		if entries, tableErr := parseSessionTable(out); tableErr == nil {
			return entries, nil
		}
		return nil, fmt.Errorf("loginctl list-sessions: unrecognised output: %s", firstLine(out))
	}

	// The flag itself was refused: ask without it.
	out2, err2 := l.r.Run(ctx, []string{"loginctl", noAsk, "list-sessions"}, nil, 0)
	if err2 != nil {
		// Report the first error, since that is the command whose failure means
		// something; the fallback failing too is expected on a machine with no
		// loginctl at all.
		return nil, err
	}
	entries, tableErr := parseSessionTable(out2)
	if tableErr != nil {
		return nil, fmt.Errorf("loginctl list-sessions: %w (json attempt failed: %v)", tableErr, err)
	}
	return entries, nil
}

// noAsk is on every loginctl invocation. A polkit challenge that cannot be answered
// non-interactively does not fail fast by default — it prompts, and a daemon with no
// terminal attached to it waits. `plock doctor` run by a normal user is the realistic
// case: the lock action is auth_admin_keep, so doctor would hang instead of saying
// "you are not root". With this flag the same situation returns an error immediately.
const noAsk = "--no-ask-password"

// sessionProps are the per-session properties the enforcement decision needs.
//
// They are named explicitly rather than taken from a bare `show-session`, because a
// bare call prints every property and a future systemd adding one would change what
// the code reads without changing anything in this file. Naming them also lets the
// argv test assert the exact set: an unknown property is not an error from loginctl,
// so a typo here would look exactly like a property that was never set.
//
// There is deliberately no "UID" here. org.freedesktop.login1.Session has no UID
// property — the uid lives on the User object, not the session — and logind answers an
// unknown property with exit 0 and no line at all, so asking for it produces nothing
// and no error. Verified against the live binary:
//
//	$ loginctl show-session 3 -p Name -p Type -p State -p Remote -p UID -p LockedHint
//	Name=rodney / Remote=no / Type=x11 / State=active / LockedHint=yes
//
// No UID line. The uid comes from the listing, which always carries it as a column.
var sessionProps = []string{"Name", "Type", "State", "Remote", "LockedHint"}

// show asks for one session's properties.
func (l *logind) show(ctx context.Context, id string, want ...string) (props, error) {
	if !validSessionID(id) {
		return nil, fmt.Errorf("loginctl show-session: refusing to query session %q: not a plausible session id", id)
	}
	argv := []string{"loginctl", noAsk, "show-session", id}
	for _, p := range want {
		argv = append(argv, "-p", p)
	}
	out, err := l.r.Run(ctx, argv, nil, 0)
	if err != nil {
		return nil, err
	}
	return parseProps(out), nil
}

// props is a parsed show-session block.
//
// A map rather than a positional read because the real binary does not preserve the
// order the properties were asked for: asking for LockedHint then TTY returns TTY
// first. Anything read by index would silently read the wrong field, and the two
// fields most likely to be swapped are exactly the booleans that decide whether to
// lock.
type props map[string]string

func parseProps(out []byte) props {
	p := props{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		p[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return p
}

func (p props) get(key string) string { return p[key] }

// flag reads a yes|no property. Anything that is not an obvious truth reads false,
// which for LockedHint is the safe direction for the property's own meaning (it says
// "not locked", and the loop re-locks every 30 seconds regardless), and for Remote
// would be the unsafe one — so Remote is only ever true when logind says so in words
// it actually uses.
func (p props) flag(key string) bool {
	switch strings.ToLower(p[key]) {
	case "yes", "true", "1":
		return true
	}
	return false
}

// isUnknownSession recognises the one per-session failure that is expected.
//
// Matching on text is not satisfying, but logind gives nothing else: an unknown
// session exits 1 with "Failed to get path for session '99999': No session '99999'
// known" and no distinct status code. Sessions disappear between the listing and the
// detail call on any machine where anyone logs out during a poll, so treating that as
// a hard error would make the enforcement loop report a failure several times a day
// and, worse, could drop the sessions that were still there.
func isUnknownSession(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no session") && strings.Contains(msg, "known")
}

// validSessionID keeps a session id from becoming an option.
//
// logind's own ids are short alphanumerics ("2", "c1", "13"). Anything with a leading
// dash, a slash, or whitespace did not come from a normal listing and must not be
// handed to loginctl as an argument, because loginctl would read a leading dash as a
// flag.
func validSessionID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.':
			// Allowed inside an id, but never first — see below.
		default:
			return false
		}
	}
	return id[0] != '-'
}

func parseSessionJSON(out []byte) ([]listEntry, error) {
	var entries []listEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// parseSessionTable reads the plain listing.
//
// The columns are SESSION UID USER SEAT LEADER CLASS TTY IDLE SINCE, with "-" for an
// absent value. It is positional, which is only safe because the header is checked
// first: a build that reorders the columns would then fail loudly instead of putting
// the class in the username field, which is the kind of mistake that shows up as
// "nobody ever gets locked".

// tableStr reads a column that is always present, mapping logind's "-" placeholder
// to the empty string the JSON path would have produced for an absent value.
func tableStr(s string) string {
	if s == "-" {
		return ""
	}
	return s
}

// tableField reads a column that can be absent, keeping the pointer semantics the
// JSON path uses so the two parsers produce the same shape: a seat logind prints as
// "-" and a seat logind prints as null have to arrive identically, because the
// filter that drops the manager session reads this field.
func tableField(s string) *string {
	if s == "-" || s == "" {
		return nil
	}
	return &s
}

func firstLine(out []byte) string {
	s := strings.TrimSpace(string(out))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	// Cut on runes. A 120-byte slice can land in the middle of a UTF-8 sequence, and
	// this string goes into an error message that ends up in a log and possibly on a
	// status page.
	if r := []rune(s); len(r) > 120 {
		s = string(r[:120]) + "..."
	}
	if s == "" {
		return "(empty output)"
	}
	return s
}

// seatString flattens the listing's nullable seat for the Session struct, where the
// empty string already means "no console".
func seatString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
