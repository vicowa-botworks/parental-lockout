package web

import (
	"net/http"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
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
// a busy one shows exactly this many and says how many files that took. The handler asks
// the reader for one more than this; see handleAudit for why the extra record is the
// difference between a guess and a fact.
const auditWindow = 200

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Query().Get("user")
	// One record more than the page shows, deliberately. Tail returns min(n, total) with
	// a nil error, so asking for exactly the window makes "the trail is longer than this"
	// and "the trail is exactly this long" the same result -- and the page has to answer
	// those two differently. The 201st record's existence is the evidence, and it is the
	// only evidence that costs nothing.
	tail, err := s.readTrail(auditWindow + 1)
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
	// The window is full, so the read stopped for a reason and not because the trail ran
	// out. This is the page's whole honesty story about how far back the trail goes, and
	// the probe above is what makes it exact rather than merely likely: a page holding
	// more than it may show is proof there is at least one older record, while a page at
	// or under the limit has read the trail to its end. Inferring it from a full window
	// instead -- the shape this had -- cannot tell those apart at exactly the limit, and
	// at that one length it tells the operator to search a generation that does not exist.
	truncated := len(rows) > auditWindow
	if truncated {
		rows = rows[:auditWindow]
	}

	// Named rather than written into the literal below on purpose. A composite literal's
	// lines are aligned by gofmt, so a pattern matching one of them retires itself the
	// moment an unrelated field with a longer name is added to the struct -- which has
	// already happened to this package's mutants twice. A statement's text does not move.
	//
	// The three are the page's other honesty claims, and each is different:
	// scanned is what the read returned before filtering, so a filter that matched nothing
	// can bound its own answer instead of speaking about the whole trail; noTrail is the
	// file not existing yet, which is a fresh machine rather than an empty log or a failed
	// read; partial is a generation that could not be opened, so the rows are real and are
	// not everything.
	//
	// scanned is counted after the cut and before the filter on purpose. Counting the
	// probe would have the page say "the filter only sees the most recent 201", a number
	// it never showed; counting after the filter would have it report the matches as if
	// they were the searched set.
	scanned := len(rows)
	noTrail := tail.FilesRead == 0
	partial := err != nil
	if user != "" {
		rows = auditRowsFor(rows, user)
	}

	// A filter that matched nothing has two causes and the banner below only names one.
	// "None of these events are for Mallory" is read as "this child was not locked out",
	// when the true answer may be "nobody on this machine is called Mallory" -- the
	// difference is whether the operator stops looking. Existence is a separate question
	// from what the trail holds, so it is asked of the account listing rather than
	// inferred from the rows, and it is only ever said when the listing actually
	// answered: readAccounts treats an unwired seam, an error, and a nil listing with a
	// nil error all as "no answer", and no answer keeps the page silent here rather than
	// letting a failed read become a confident "no such account".
	//
	// Restricted to the filtered-empty branch on purpose. When rows are on screen the
	// trail itself disagrees with the listing -- a deleted account's events outlive the
	// account -- and the page must not contradict what the operator is looking at.
	// Two halves, deliberately. This guard decides whether the account listing is read at
	// all: a page that already found the account has no business opening /etc/passwd, and
	// a listing that fails must not be able to change an answer it was not asked for. The
	// rule itself lives in accountIsUnknown, where it can be pinned directly -- left here,
	// the same condition is invisible to every test, because audit.tmpl only renders the
	// note inside its filtered-empty branch and would mask any change to it.
	userUnknown := false
	if user != "" && len(rows) == 0 {
		if listing, lerr := s.readAccounts(); lerr == nil {
			userUnknown = accountIsUnknown(user, rows, listing)
		}
	}

	page := auditPage{
		Rows:      rows,
		User:      user,
		Scanned:   scanned,
		Skipped:   tail.Skipped,
		FilesRead: tail.FilesRead,
		Partial:   partial,
		NoTrail:   noTrail,
		Truncated: truncated,
	}
	// Assigned after the literal rather than added to it. gofmt aligns the values in a
	// composite literal, so a longer field name retires any mutation pattern that matches
	// one of its lines -- which has already happened to this package's mutants twice. A
	// statement's text does not move.
	page.UserUnknown = userUnknown

	s.render(w, r, "audit", page)
}

// accountIsUnknown is the rule behind the note, stated apart from the page so it can be
// pinned without going through a template that would otherwise be the only thing enforcing
// it.
//
// Three conditions, each ruling out a different wrong sentence:
//
//   - no filter, nothing to comment on;
//   - rows on screen -- the trail itself disagrees with the listing, because an account
//     deleted last week still owns the events it made, and the page must not contradict
//     what the operator is looking at;
//   - no listing, no answer. readAccounts already turns an unwired seam, an error, and a
//     nil listing with a nil error into "no answer"; a nil here is the same silence.
//
// Existence is asked of the listing, never inferred from the rows: "no events for this
// name" and "no such account" are different facts and the operator needs to know which one
// the page actually checked.
func accountIsUnknown(user string, rows []audit.Record, listing *users.Listing) bool {
	if user == "" || len(rows) > 0 || listing == nil {
		return false
	}
	return !accountNamed(listing, user)
}

// accountNamed says whether the listing carries this exact account name.
//
// Exact, case-sensitive, no trimming. Linux account names are case-sensitive and Mallory
// and mallory can be two children, so a loose match here would tell an operator that a
// name they typed does not exist while a differently-cased account does -- which is worse
// than the typo dead-end this exists to clear. The page's job is to say what the listing
// said, not to guess what was meant.
func accountNamed(listing *users.Listing, user string) bool {
	for _, a := range listing.Accounts {
		if a.Username == user {
			return true
		}
	}
	return false
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

	// Truncated is the read having stopped with a record still behind it, which is the only
	// thing that entitles the page to say older events may exist beyond what it showed. Its
	// absence means the trail was read to the end, and telling an operator to look further
	// back than the trail goes is its own kind of wrong answer -- one that sends them to
	// check a generation that does not exist while the account they asked about really has
	// no events. It is decided by the probe record, not by the page being full.
	Truncated bool

	// UserUnknown is the filtered-empty page having checked, and found no account by that
	// exact name on this machine right now. It says nothing about the trail: the events of
	// an account deleted last week are still its events. It is set only when the account
	// listing answered, and only when no row matched, so it can never contradict rows the
	// operator is looking at, and a listing that could not be read leaves the page silent
	// about existence rather than inventing a second answer.
	UserUnknown bool
}
