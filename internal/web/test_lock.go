package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/vicowa-botworks/parental-lockout/internal/lockop"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// handleTestLock is the operator's "could this machine lock a screen at all?" button.
//
// It is the web surface for the question `plock doctor` already answers, and it exists
// because the person looking at a dashboard row that says "locked out now" is usually not
// root and not at a terminal. The capability is not new: `platform.Chain.Probe` asks every
// configured backend whether it could do its job right now, and doctor renders that report.
// What is new is that the answer is one click from the row that raised the doubt.
//
// What it does NOT do is lock anything. That is the whole difference between this route and
// `POST /users/{name}/lock-now`, and the reason both exist: lock-now is the operator telling
// a screen to go down, this is the operator asking the machine whether it could. A probe that
// locked the child's screen as a side effect of being pressed could not be pressed while the
// child is working, which is exactly when the question gets asked. The page says so in its
// own banner, and `TestTheProbeAsksTheBackendsAndNeverLocks` pins it by failing if the lock
// seam is touched at all.
//
// It is a POST for the same reason the write routes are: the CSRF gate covers it, and a
// request that reaches into the host -- even to read -- does not belong on a URL a link can
// point at. It is per-account rather than a page-wide `/test-lock` because the useful answer
// is about the row the operator is looking at: which backends are reachable, and whether this
// account currently holds a session one of them could lock.
func (s *Server) handleTestLock(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		s.pageRefuse(w, r, http.StatusBadRequest, "a username is required")
		return
	}
	if s.Probe == nil {
		// The assembled-wrong case, said before anything is read. A server without the
		// probe seam is not a machine with no reachable backends, and rendering an empty
		// table would turn a wiring mistake into a statement about the operator's desktop.
		s.pageError(w, r, "the backend probe is not available on this daemon", errNoProbePath)
		return
	}

	cfg, err := s.readConfig()
	if err != nil {
		s.pageError(w, r, "the configuration could not be read", err)
		return
	}
	// The same refusal lock-now gives, in the same words: a name that is not in the
	// document is not a machine question at all, and answering it with a backend table
	// would imply the probe had something to probe for.
	if !knownAccount(cfg, name) {
		s.pageRefuse(w, r, http.StatusNotFound, "no managed account named "+name)
		return
	}

	// The probe itself. It is taken first and kept whole: the backends' answers are the
	// question that was asked, and nothing below is allowed to veto them.
	statuses := s.Probe(r.Context())
	reachable := 0
	for _, st := range statuses {
		if st.Available {
			reachable++
		}
	}

	page := probePage{Target: name, Statuses: statuses, Reachable: reachable}

	// The session half is context, not the answer, so it is read after the probe and its
	// failure is confined to the one line that depends on it. A failed listing is never
	// rendered as "this account has no session" -- the same rule the audit page and the
	// assignment route hold -- and it is not a 503 either: the backends were reached and
	// their answers are already in hand, so refusing the whole page here would throw away
	// the answer to get at the one that failed.
	chain := serverChain{s: s}
	sessions, serr := lockop.SessionsFor(r.Context(), chain, name)
	switch {
	case serr != nil:
		page.SessionsFailed = true
	case len(sessions) == 0:
		page.NoSessions = true
	default:
		for _, sess := range sessions {
			row := probeSession{Session: sess.ID, Seat: sess.Seat, Type: sess.Type}
			row.Lockable = platform.IsLockable(sess)
			if !row.Lockable {
				// The reason comes from the same function that decided it, so the page
				// cannot explain a refusal the rule did not make.
				row.Detail = lockop.NotLockable(sess)
			}
			if s.Self.Declines(sess.UID) {
				row.SelfSession = true
			}
			page.Sessions = append(page.Sessions, row)
		}
	}

	// Counts, never a verdict, and the same shape as the lock line so one grep finds both
	// halves of the story. A probe is an action in the sense that matters here: the
	// operator asked the machine a question, and the answer they were given belongs in the
	// trail next to the lock attempts it explains.
	s.audit("lock.probed", fmt.Sprintf("%s: %d of %d backends reachable, %s",
		name, reachable, len(statuses), sessionName(s, r)))

	s.render(w, r, "testlock", page)
}

// errNoProbePath is the assembled-wrong case for the probe seam, kept as a sentinel so the
// log line names it and a test can assert on the sentence rather than on a status code.
var errNoProbePath = errors.New("no backend-probe seam is wired on this server")

// probePage is the view model for one backend probe.
type probePage struct {
	Target    string
	Statuses  []platform.BackendStatus
	Reachable int
	// Sessions is the account's own sessions, and SessionsFailed / NoSessions are the two
	// reasons it can be empty. Three fields because "nobody is logged in" and "we could not
	// look" are different facts and the operator's next action differs for each.
	Sessions       []probeSession
	NoSessions     bool
	SessionsFailed bool
}

// probeSession is one of the account's sessions, as far as the probe cares: where it is and
// whether any backend could lock it.
type probeSession struct {
	Session     string
	Seat        string
	Type        string
	Lockable    bool
	Detail      string
	SelfSession bool
}

// Total is the denominator the banner reads out. It is a method rather than a field so the
// number on the page cannot disagree with the table under it.
func (p probePage) Total() int { return len(p.Statuses) }

// AnyLockable says whether at least one of this account's sessions is something a backend
// could take. It is the sentence the operator is really asking for -- "would a lock reach
// this child?" -- and it is separate from the backend count, because a machine with three
// healthy backends and a child logged out still answers "nothing would be locked today".
func (p probePage) AnyLockable() bool {
	for _, s := range p.Sessions {
		if s.Lockable {
			return true
		}
	}
	return false
}
