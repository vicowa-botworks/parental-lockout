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
	// A listing that was only partly readable is not a listing that says "this account has
	// no session" (#168). The sessions the daemon did read are lockable and are about to be
	// locked; answering 503 for the whole request was answering about a child whose session
	// was sitting in the same slice. Only a read with nothing in it is a request the page
	// cannot act on.
	if err != nil && len(sessions) == 0 {
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
	// The trail carries the gap as its own line. This log is the surface an operator reads
	// afterwards as evidence, and "locked 1" with no mention of the session that could not
	// be read is a record that quietly understates what was attempted (#168). It is a
	// separate event rather than a clause on the line above because that line's shape is
	// what the capture driver and the tests read.
	if err != nil {
		s.audit("lock.listing_incomplete", fmt.Sprintf("%s: %v", name, err))
	}

	byID := make(map[string]platform.Session, len(sessions))
	for _, sess := range sessions {
		byID[sess.ID] = sess
	}
	page := lockPage{Target: name, Result: res}
	if err != nil {
		page.Note = fmt.Sprintf("The session table was only partly readable, so the rows below are what "+
			"the daemon could see and not necessarily every session %s has: %v.", name, err)
	}
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
	// Note is a sentence about the listing itself, above the table. It is empty whenever
	// the read was complete, so the page's ordinary pixels do not move. It carries the one
	// fact a partial read makes uncertain: the rows below are what the daemon could see,
	// not necessarily everything the account holds (#168).
	Note string
}

// lockRow is one session's line, carrying the two facts the page adds over lockop's
// Outcome: which console the session is on, and whether this is the session the daemon
// itself sits in.
//
// The verdict reaches the page through the embedded Outcome's booleans, which lock.tmpl
// branches on to pick the span and its class. There is deliberately no string-returning
// State() beside these fields: the template never asked for one, so it would be a method
// nothing reads -- and a mutation pattern pointed at it survives every battery forever,
// because no test can observe a call that does not happen (#107).
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

// DetailText is the Detail cell's text with a sentence terminator on the end. The cell puts
// this next to the self-session note, and the note is a sentence the page wrote, so the
// seam needs a boundary rather than a space (#108).
//
// The terminator belongs to the rendering only. The audit line and `plock lock` print the raw
// Detail, deliberately: the trail is the machine-readable record, and a period added for a
// paragraph would put it out of step with the page for anyone who copies a sentence off the
// screen to go grep for it. Do not "fix" the audit side to match.
func (r lockRow) DetailText() string { return sentenceTerminated(r.Detail) }

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
