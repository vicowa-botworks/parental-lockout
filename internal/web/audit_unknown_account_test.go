package web

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// #92: the account filter is free text, and a typo used to produce a complete, confident,
// closed answer. "None of the 1 most recent event is for Mallory. The trail holds nothing
// older than these" reads as *no, and here is proof nothing was missed* when the true
// answer is *you typed a name that does not exist*. These tests pin the existence note and,
// just as much, the three situations in which the page must stay silent about existence.

const unknownNote = "There is no account named"

// sentences is the page's prose with its markup removed. The banner wraps the account name
// in <strong>, so asserting on flat() bytes would pin the markup rather than the sentence --
// and re-tagging a banner would fail tests whose claim is still true. Escaping is pinned by
// this package's own escaping test; what these tests read is what an operator reads.
func sentences(body string) string {
	var out strings.Builder
	var inTag bool
	for _, r := range body {
		switch {
		case r == '<':
			inTag = true
		}
		if inTag {
			if r == '>' {
				inTag = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return flat(out.String())
}

// pageWithTrailAndAccounts is the audit page with both seams the note needs: the trail it
// reads and the listing that answers whether a name exists.
func pageWithTrailAndAccounts(t *testing.T, tail audit.Tail, listing *users.Listing, acctErr error) (*Server, *fakeTrail) {
	t.Helper()
	s, ft, _ := pageWithTrail(t, tail, nil)
	s.Accounts = func() (*users.Listing, error) { return listing, acctErr }
	return s, ft
}

func listingWith(names ...string) *users.Listing {
	l := &users.Listing{}
	for _, n := range names {
		l.Accounts = append(l.Accounts, users.Account{Username: n})
	}
	return l
}

// The typo, measured: the page used to answer it with a closed search and nothing else.
func TestATypoedAccountIsNamedAsATypo(t *testing.T) {
	tail := audit.Tail{Records: []audit.Record{
		{At: insideWindow, Kind: "lockout.entered", Username: "casey", RuleName: "weeknights"},
	}, FilesRead: 1}
	s, _ := pageWithTrailAndAccounts(t, tail, listingWith("casey", "parent"), nil)

	rec := authGet(t, s, "/audit?user=Mallory")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
	}
	body := sentences(rec.Body.String())
	if !strings.Contains(body, unknownNote+" Mallory") {
		t.Errorf("the page did not say the name is not an account:\n%s", body)
	}
	// The case-sensitivity hint is the actionable half: Mallory and mallory can be two
	// children, so the page must not fix the typo by matching loosely.
	if !strings.Contains(body, "case-sensitive") {
		t.Errorf("the note does not say names are case-sensitive:\n%s", body)
	}
	// Both sentences belong together. Dropping the count would trade one over-claim for
	// another: the operator still needs to know how far the search actually looked.
	if !strings.Contains(body, "1 most recent event is for") {
		t.Errorf("the bounded-search sentence disappeared:\n%s", body)
	}
}

// A name the listing knows, with nothing in the window. The honest answer is "no events for
// this account", and adding the existence note to it would send the operator
// off to look for an account that is sitting in front of them.
func TestAKnownAccountWithNoEventsIsNotCalledUnknown(t *testing.T) {
	tail := audit.Tail{Records: []audit.Record{
		{At: insideWindow, Kind: "lockout.entered", Username: "casey", RuleName: "weeknights"},
	}, FilesRead: 1}
	s, _ := pageWithTrailAndAccounts(t, tail, listingWith("casey", "alexis"), nil)

	body := sentences(authGet(t, s, "/audit?user=alexis").Body.String())
	if strings.Contains(body, unknownNote) {
		t.Errorf("the page called a listed account unknown:\n%s", body)
	}
	if !strings.Contains(body, "1 most recent event is for alexis") {
		t.Errorf("the bounded-search sentence is not the one for a known account:\n%s", body)
	}
}

// The three ways the listing fails to answer. Each must leave the page silent about
// existence: "the listing could not be read" is not evidence that nobody is called Mallory,
// and this is the same rule the trail seam carries -- a failed read is never an empty one.
func TestAnUnansweredListingKeepsThePageSilentAboutExistence(t *testing.T) {
	tail := audit.Tail{Records: []audit.Record{
		{At: insideWindow, Kind: "lockout.entered", Username: "casey", RuleName: "weeknights"},
	}, FilesRead: 1}

	cases := []struct {
		name    string
		listing *users.Listing
		err     error
		nilSeam bool
	}{
		{"the seam is unwired", nil, nil, true},
		{"the listing could not be read", nil, errors.New("cannot open /etc/passwd"), false},
		// A nil listing with a nil error is a source that answered nothing at all, and
		// readAccounts already says so; the note must not treat it as an empty account set.
		{"the source answered with nothing", nil, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := pageWithTrailAndAccounts(t, tail, tc.listing, tc.err)
			if tc.nilSeam {
				s.Accounts = nil
			}
			rec := authGet(t, s, "/audit?user=Mallory")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			body := sentences(rec.Body.String())
			if strings.Contains(body, unknownNote) {
				t.Errorf("a listing that did not answer produced an existence claim:\n%s", body)
			}
			// Silence about existence is not silence about the search: the bounded
			// sentence still has to be there, or the page has traded one gap for another.
			if !strings.Contains(body, "1 most recent event is for Mallory") {
				t.Errorf("the search sentence vanished along with the note:\n%s", body)
			}
		})
	}
}

// An account deleted last week still owns the events it made. The note is about the account
// listing now, so it belongs on the filtered-empty branch and never contradicts rows the
// operator is looking at.
func TestADeletedAccountsEventsAreNotContradicted(t *testing.T) {
	tail := audit.Tail{Records: []audit.Record{
		{At: insideWindow, Kind: "lockout.entered", Username: "robin", RuleName: "weeknights"},
	}, FilesRead: 1}
	s, _ := pageWithTrailAndAccounts(t, tail, listingWith("casey"), nil)

	rec := authGet(t, s, "/audit?user=robin")
	body := sentences(rec.Body.String())
	if !strings.Contains(body, "robin") || !strings.Contains(body, "lockout.entered") {
		t.Fatalf("the deleted account's own event is not on the page:\n%s", body)
	}
	if strings.Contains(body, unknownNote) {
		t.Errorf("the page denies an account that it is simultaneously showing events for:\n%s", body)
	}
}

// The hint earns its place on the case variant, and this is also the proof that the fix did
// not loosen the match: mallory in the listing does not make a filter for Mallory show
// mallory's events, it makes the page say the name typed is not an account.
func TestACaseVariantIsRefusedAndExplainedRatherThanMatched(t *testing.T) {
	tail := audit.Tail{Records: []audit.Record{
		{At: insideWindow, Kind: "lockout.entered", Username: "mallory", RuleName: "weeknights"},
	}, FilesRead: 1}
	s, _ := pageWithTrailAndAccounts(t, tail, listingWith("mallory"), nil)

	body := sentences(authGet(t, s, "/audit?user=Mallory").Body.String())
	if strings.Contains(body, "lockout.entered") {
		t.Errorf("the filter matched across case, which merges two children's lockouts:\n%s", body)
	}
	if !strings.Contains(body, unknownNote+" Mallory") {
		t.Errorf("a case variant is exactly the typo the hint exists for:\n%s", body)
	}
}

// The unfiltered page has no filter to comment on. Without this, an empty trail plus a
// failing listing would be enough to put an account-naming sentence on a page that never
// asked about an account.
func TestAnUnfilteredPageSaysNothingAboutAccounts(t *testing.T) {
	s, _ := pageWithTrailAndAccounts(t, audit.Tail{FilesRead: 0}, nil, errors.New("cannot open /etc/passwd"))
	rec := authGet(t, s, "/audit")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if body := sentences(rec.Body.String()); strings.Contains(body, unknownNote) {
		t.Errorf("an unfiltered page commented on an account:\n%s", body)
	}
}

// The rule, pinned where it is decided.
//
// These go straight at accountIsUnknown rather than through the page because audit.tmpl
// renders the note only inside its filtered-empty branch -- so a page-level test cannot see
// the handler's own rows-empty condition at all. That masking is how a guard survives being
// deleted: the template keeps producing the right sentence while the code underneath stops
// deciding it. Pinning the predicate is what keeps the two from drifting apart.
func TestTheRuleForWhenANoteIsOwed(t *testing.T) {
	matched := []audit.Record{{At: insideWindow, Kind: "lockout.entered", Username: "robin"}}
	none := []audit.Record{}
	listing := listingWith("casey", "parent")

	cases := []struct {
		name    string
		user    string
		rows    []audit.Record
		listing *users.Listing
		want    bool
	}{
		{"no filter", "", none, listing, false},
		{"filter matched, name not in the listing", "robin", matched, listing, false},
		{"filter matched and the name is listed", "casey", matched, listing, false},
		{"nothing matched and the name is listed", "parent", none, listing, false},
		{"nothing matched and the name is not listed", "Mallory", none, listing, true},
		{"the listing did not answer", "Mallory", none, nil, false},
		// An empty listing is an answer, and it is the answer "nobody is a local account
		// in the enforced range" -- the note is correct for it, which is why the case is
		// here rather than folded into the row above.
		{"the listing answered with no accounts", "Mallory", none, listingWith(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := accountIsUnknown(tc.user, tc.rows, tc.listing); got != tc.want {
				t.Errorf("accountIsUnknown(%q, %d rows) = %v, want %v", tc.user, len(tc.rows), got, tc.want)
			}
		})
	}
}

// The other half of the split: this is what the handler's own guard does, and without the
// pair below it a mutation that deletes the read entirely would look identical to the
// mutation that keeps it.
func TestTheListingIsReadOnlyWhenTheFilterFoundNothing(t *testing.T) {
	tail := audit.Tail{Records: []audit.Record{
		{At: insideWindow, Kind: "lockout.entered", Username: "robin", RuleName: "weeknights"},
	}, FilesRead: 1}

	t.Run("the filter found the account", func(t *testing.T) {
		s, _, _ := pageWithTrail(t, tail, nil)
		calls := 0
		s.Accounts = func() (*users.Listing, error) {
			calls++
			return nil, errors.New("the listing must not be opened for this request")
		}
		rec := authGet(t, s, "/audit?user=robin")
		if calls != 0 {
			t.Errorf("the account listing was opened %d times for a filter that matched", calls)
		}
		if !strings.Contains(sentences(rec.Body.String()), "lockout.entered") {
			t.Errorf("the matched row is not on the page:\n%s", sentences(rec.Body.String()))
		}
	})

	t.Run("the filter found nothing", func(t *testing.T) {
		s, _, _ := pageWithTrail(t, tail, nil)
		calls := 0
		s.Accounts = func() (*users.Listing, error) {
			calls++
			return listingWith("robin"), nil
		}
		authGet(t, s, "/audit?user=Mallory")
		if calls != 1 {
			t.Errorf("the account listing was opened %d times; the page needs it exactly once to answer", calls)
		}
	})
}

// #94's review finding, and it is the same failure this PR exists to close. The listing is a
// filter, not a census: parsePasswd drops every uid outside UID_MIN..UID_MAX and every
// nologin shell in silence, because a machine has hundreds of those and a note each would
// bury the real ones. So a name missing from the listing is evidence about the filter, not
// about the machine -- and `useradd -u 900` is a real child that no read at UID_MIN=1000 will
// ever return. The sentence that ran here said he does not exist and then pointed at the
// spelling, which is the one thing that makes an operator stop looking.
func TestTheNoteNamesTheFilterThatSaidNoRatherThanTheMachine(t *testing.T) {
	tail := audit.Tail{Records: []audit.Record{
		{At: insideWindow, Kind: "lockout.entered", Username: "casey", RuleName: "weeknights"},
	}, FilesRead: 1}
	listing := &users.Listing{UIDMin: 500, UIDMax: 1500}
	listing.Accounts = append(listing.Accounts, users.Account{Username: "casey"})
	s, _ := pageWithTrailAndAccounts(t, tail, listing, nil)

	body := sentences(authGet(t, s, "/audit?user=robert").Body.String())
	if !strings.Contains(body, "among the accounts plock can manage") {
		t.Errorf("the note speaks about the machine rather than the managed set:\n%s", body)
	}
	// The bounds are the operator's way out. Without them the narrowed sentence is still a
	// black box, and the only way to notice the filter was the wrong one is to see which
	// filter answered. They are the bounds actually applied, not the compiled defaults:
	// users.List reports what login.defs said, including the fallback it noted.
	if !strings.Contains(body, "uid 500-1500") {
		t.Errorf("the note does not name the uid bounds that were applied:\n%s", body)
	}
	// The claim the page is no longer allowed to make, pinned so a rewording cannot creep
	// back to it by way of "tidying the sentence up".
	if strings.Contains(body, "no local account") {
		t.Errorf("the page is asserting what accounts exist on the machine:\n%s", body)
	}
}

// A listing with no bounds is a hand-built one, and "uid 0-0" would be a range the page made
// up. The narrowed sentence is still exactly true without it, so the page says that and stops.
func TestAListingWithoutBoundsInventsNoRange(t *testing.T) {
	tail := audit.Tail{Records: []audit.Record{
		{At: insideWindow, Kind: "lockout.entered", Username: "casey", RuleName: "weeknights"},
	}, FilesRead: 1}
	s, _ := pageWithTrailAndAccounts(t, tail, listingWith("casey"), nil)

	body := sentences(authGet(t, s, "/audit?user=robert").Body.String())
	if !strings.Contains(body, unknownNote+" robert") {
		t.Fatalf("the note was withheld along with the bounds, so the typo is a closed search again:\n%s", body)
	}
	if strings.Contains(body, "uid 0-0") {
		t.Errorf("the page invented a uid range out of a zero value:\n%s", body)
	}
}

// A withheld note and an absent note look identical on the page, and this page's whole
// contract is that silence is a verdict. The trail's failed read gets a log line for the same
// reason; without one, "the page chose not to speak" and "the page could not look" cannot be
// told apart after the fact by anyone but the operator standing at the machine.
func TestAFailedAccountListingIsLoggedWhenTheNoteIsWithheld(t *testing.T) {
	tail := audit.Tail{Records: []audit.Record{
		{At: insideWindow, Kind: "lockout.entered", Username: "casey", RuleName: "weeknights"},
	}, FilesRead: 1}
	s, _ := pageWithTrailAndAccounts(t, tail, nil, errors.New("passwd unreadable"))
	sink := newLogSink()
	s.Logf = sink.log

	rec := authGet(t, s, "/audit?user=Mallory")
	if rec.Code != http.StatusOK {
		t.Fatalf("a listing failure must not turn the page into an error, got %d", rec.Code)
	}
	body := sentences(rec.Body.String())
	if strings.Contains(body, unknownNote) {
		t.Errorf("the page answered existence from a listing it could not read:\n%s", body)
	}
	var said bool
	for _, l := range sink.Lines() {
		if strings.Contains(l, "account listing unavailable") && strings.Contains(l, "passwd unreadable") {
			said = true
		}
	}
	if !said {
		t.Errorf("nothing was logged about the refused listing; lines: %q", sink.Lines())
	}

	// The control: a listing that answered must not log. Without this half, a page that
	// logs on every filtered request passes the test above just as well.
	clean, _ := pageWithTrailAndAccounts(t, tail, listingWith("casey"), nil)
	cleanSink := newLogSink()
	clean.Logf = cleanSink.log
	authGet(t, clean, "/audit?user=Mallory")
	for _, l := range cleanSink.Lines() {
		if strings.Contains(l, "account listing unavailable") {
			t.Errorf("a listing that answered was logged as though it had failed: %q", l)
		}
	}
}
