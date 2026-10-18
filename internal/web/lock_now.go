package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/vicowa-botworks/parental-lockout/internal/lockop"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// handleLockNow is the operator's "lock this account now" button.
//
// It is the same action `plock lock <user>` performs, driven through the same chain the
// enforcement loop enforces with (serve wires both from one platform.New), and it shares
// the classification with the CLI through lockop rather than restating it here. That
// sharing is the point. A second loop would be free to call an unconfirmed lock a success,
// and the operator would then read one sentence in the terminal and the opposite one on
// the page they are actually looking at.
//
// What it does NOT do is write anything to the config. This is one action, not a change of
// policy: the account returns to whatever its schedule says the moment the daemon's next
// tick runs, and the loop re-asserts the lock on every tick of an open window anyway
// (#1001), so a manual lock can neither hold an account past its window nor release it
// early.
//
// The answer is rendered rather than redirected to. The verdict came from the chain call
// this request made, and the two ways to carry it across a redirect are both worse: a query
// parameter is text the operator can type, so the dashboard would render a claim the server
// never made, and a server-side flash store is state that outlives the request and goes
// stale on the next tick while still looking like an answer. Rendering the page the call
// produced is the only version where the sentence and the event are the same request. A
// reload asks again, which for this action is harmless: locking a locked screen is a
// repaint (#1001), not a second lockout.
func (s *Server) handleLockNow(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		s.pageRefuse(w, r, http.StatusBadRequest, "a username is required")
		return
	}
	if s.Lock == nil {
		// An unwired lock path is not "the lock failed" and not "no sessions": it means
		// this server was assembled without the thing that locks screens. It is 503 rather
		// than a refusal because the server is the broken half, and it is said before
		// anything is read so a page never reports "0 of 1 locked" for a daemon that never
		// asked.
		s.pageError(w, r, "locking is not available on this daemon", errNoLockPath)
		return
	}

	cfg, err := s.readConfig()
	if err != nil {
		s.pageError(w, r, "the configuration could not be read", err)
		return
	}
	// The same refusal the override routes give, in the same words. "No such account" and
	// "that account has no session" are different facts, and a parent trying to lock a
	// child out needs to know which one they are looking at: the first means the name is
	// wrong, the second means the child is already logged out.
	if !knownAccount(cfg, name) {
		s.pageRefuse(w, r, http.StatusNotFound, "no managed account named "+name)
		return
	}

	chain := serverChain{s: s}
	sessions, err := lockop.SessionsFor(r.Context(), chain, name)
	if err != nil {
		s.pageError(w, r, "the session table could not be read", err)
		return
	}
	if len(sessions) == 0 {
		s.pageRefuse(w, r, http.StatusConflict, name+" has no session on this machine")
		return
	}

	res := lockop.Now(r.Context(), chain, sessions)

	// Counts, never a verdict. "locked 0, unconfirmed 1" is the trail's record of a lock
	// that may well have taken; writing "locked" here would be the page asserting
	// something the machine could not prove, and the audit log is the surface an operator
	// reads afterwards as evidence.
	s.audit("lock.requested", fmt.Sprintf("%s: locked %d, unconfirmed %d, refused %d by %s",
		name, res.Locked, res.Unconfirmed, res.Refused, sessionName(s, r)))

	byID := make(map[string]platform.Session, len(sessions))
	for _, sess := range sessions {
		byID[sess.ID] = sess
	}
	page := lockPage{Target: name, Result: res}
	for _, o := range res.Outcomes {
		row := lockRow{Outcome: o}
		// The page names the session by more than its logind id, and the listing is the
		// only place that knows the seat and the type. lockop deliberately does not carry
		// them: its Result is the same record whether it is rendered as a terminal report,
		// an audit line or a page, and only the page has columns.
		if sess, ok := byID[o.Session]; ok {
			row.Seat = sess.Seat
			row.Type = sess.Type
			if s.Self.Declines(sess.UID) {
				// The one case where the page has to explain the difference between the two
				// ways this daemon locks a screen. The enforcement loop declines the session it
				// runs inside (#67), and a lock the operator asked for is not that lock
				// (docs/security.md §4), so this account is both "the schedule will never hold
				// it" and "just locked, because you pressed the button". Saying only one of the
				// two leaves the operator to resolve the contradiction themselves.
				row.SelfSession = true
			}
		}
		page.Rows = append(page.Rows, row)
	}
	s.render(w, r, "lock", page)
}

// errNoLockPath is the assembled-wrong case, kept as a sentinel so the log line names it
// and a test can assert on the sentence rather than on a status code alone.
var errNoLockPath = errors.New("no lock seam is wired on this server")

// lockPage is the view model for the result of one lock request.
type lockPage struct {
	Target string
	Result lockop.Result
	Rows   []lockRow
}

// lockRow is one session's line, carrying the two facts the page adds over lockop's
// Outcome: which console the session is on, and whether this is the session the daemon
// itself sits in.
type lockRow struct {
	lockop.Outcome
	// Seat and Type come from the listing, not from the lock. They are on the page because
	// an account can hold a greeter row and a desktop row with the same uid, and the
	// operator reading "not a lockable desktop session" needs to see which row said it.
	Seat string
	Type string
	// SelfSession is the account whose login session the daemon runs inside. It is a
	// separate sentence rather than part of the verdict because the verdict is about the
	// screen and this is about the schedule: the loop will never hold this account, and
	// the operator just locked it.
	SelfSession bool
}

// State names the verdict in the page's words.
//
// Three strings, not a boolean: "unconfirmed" is the answer a raw virtual terminal gives
// forever, and a page that rendered it as either success or failure would teach the
// operator to distrust the only signal the machine has.
func (r lockRow) State() string {
	switch {
	case r.Locked:
		return "locked"
	case r.Unconfirmed:
		return "unconfirmed"
	default:
		return "not locked"
	}
}

// serverChain adapts the server's seams to lockop.Chain.
//
// It exists so the lock path reads sessions through readSessions the way every other page
// does -- which is what turns an unwired seam into an error rather than an empty list --
// and locks through the one seam serve wired, rather than building a second chain that
// might hold different backends than the loop is enforcing with.
type serverChain struct{ s *Server }

// Sessions ignores ctx because the server's seam does: the session table is read from the
// host here and now, and there is no cancellable operation underneath it to pass through.
func (c serverChain) Sessions(ctx context.Context) ([]platform.Session, error) {
	return c.s.readSessions()
}

func (c serverChain) Lock(ctx context.Context, sess platform.Session) error {
	if c.s.Lock == nil {
		// Unreachable from the handler, which refuses before reading anything. It stays
		// because the chain outlives that check's scope -- and because the alternative
		// reading of a nil seam is a nil-pointer panic inside the loop, mid-request, after
		// some sessions were already locked.
		return errNoLockPath
	}
	return c.s.Lock(ctx, sess)
}
