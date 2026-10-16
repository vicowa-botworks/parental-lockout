package web

import (
	"net/http"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
)

// GET /audit -- the trail, newest first, filterable by account.
//
// The page reads through the Trail seam rather than opening the file itself for the same
// reason the other pages read through theirs: only the daemon's own process knows where
// the trail lives and whether it can be opened, and a handler that opened paths directly
// could only be tested by arranging a filesystem. The CI runner has no /var/log/plock, so
// such a test would either be skipped or would be asserting facts about the runner -- and
// a test that passes because of what the host occurs to contain also passes when the
// feature is broken.

// auditWindow is the page size, not a limit on the trail. The reader walks rotated
// generations until it has this many records, so a quiet house shows fewer than this and
// a busy one shows exactly this many and says how many files that took.
const auditWindow = 200

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Query().Get("user")
	tail, err := s.readTrail(auditWindow)
	if err != nil && tail.FilesRead == 0 {
		// Nothing could be read at all, so there is no page to render honestly.
		s.pageError(w, r, "the audit trail could not be read", err)
		return
	}
	if err != nil {
		// The reader's contract is that it hands back whatever it did read along with the
		// reason the page is short, so the page shows both. Dropping the records would
		// turn a half-readable trail into an empty one, which is the one reading of a
		// failed read that is definitely wrong.
		s.logf("web: %s audit trail incomplete: %v", requestID(r), err)
		w.Header().Set("X-Plock-Note", "trail-incomplete")
	}
	rows := tail.Records
	if user != "" {
		rows = auditRowsFor(rows, user)
	}

	// Named rather than written into the literal below on purpose. A composite literal's
	// lines are aligned by gofmt, so a pattern matching one of them retires itself the
	// moment an unrelated field with a longer name is added to the struct -- which has
	// already happened to this package's mutants twice. A statement's text does not move.
	//
	// The three are the page's whole honesty story, and each is a different claim:
	// scanned is what the read returned before filtering, so a filter that matched nothing
	// can bound its own answer instead of speaking about the whole trail; noTrail is the
	// file not existing yet, which is a fresh machine rather than an empty log or a failed
	// read; partial is a generation that could not be opened, so the rows are real and are
	// not everything.
	scanned := len(tail.Records)
	noTrail := tail.FilesRead == 0
	partial := err != nil

	s.render(w, r, "audit", auditPage{
		Rows:      rows,
		User:      user,
		Scanned:   scanned,
		Skipped:   tail.Skipped,
		FilesRead: tail.FilesRead,
		Partial:   partial,
		NoTrail:   noTrail,
	})
}

// auditRowsFor keeps the records naming this account.
//
// Exact match, not a prefix or a substring. The filter runs on a set that was already cut
// to the newest auditWindow records, so a loose match would quietly merge two accounts --
// "cat" with "catie" -- on a page whose entire job is saying who was held and when.
func auditRowsFor(rows []audit.Record, user string) []audit.Record {
	out := make([]audit.Record, 0, len(rows))
	for _, rec := range rows {
		if rec.Username == user {
			out = append(out, rec)
		}
	}
	return out
}

// auditPage is what audit.tmpl is given.
//
// The three empty-state flags are separate because "nothing happened", "nothing happened
// to this account" and "this page could not look" are three different answers, and the
// one thing a trail page must not do is render a read it could not perform as a quiet
// house. The dashboard made the same correction for the session list.
type auditPage struct {
	Rows []audit.Record

	// User is the filter as typed, empty when the page is unfiltered. It is echoed back
	// into the form so a filtered page can be edited rather than retyped, and it is
	// rendered through html/template like everything else: an account name is a string a
	// local user chose.
	User string

	// Scanned counts what the read returned before filtering. Without it a filter that
	// matched nothing can only say "no events", which is a claim about the whole trail
	// made from a window of auditWindow records.
	Scanned int

	Skipped   int
	FilesRead int

	// Partial is a generation that could not be opened. The records above are real, and
	// they are not all there is.
	Partial bool

	// NoTrail is the trail file not existing yet.
	NoTrail bool
}
