package web

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
)

// The audit page's own tests.
//
// The shape worth naming: every empty-state test here asserts which sentence the page
// chose, not merely that the table is empty. "Nothing happened", "nothing happened to
// this account", "there is no trail file yet" and "this page could not look" are four
// different answers, and a test that only checks for an absent row passes on all four.
// That is the same failure the dashboard had, where a failed session read rendered as an
// empty list and told a parent the house was quiet.

// fakeTrail answers with fixed state and records what the handler asked for, so the page
// size is pinned rather than assumed.
type fakeTrail struct {
	tail  audit.Tail
	err   error
	calls int
	wantN int
}

func (f *fakeTrail) Tail(n int) (audit.Tail, error) {
	f.calls++
	f.wantN = n
	return f.tail, f.err
}

// flat collapses the whitespace runs a browser collapses anyway. The templates wrap their
// prose so the source stays readable, so asserting on the raw bytes would pin line breaks
// instead of sentences -- and rewrapping a paragraph would fail tests that are still true.
func flat(s string) string { return strings.Join(strings.Fields(s), " ") }

// pageWithTrail is pageWithSeams plus the trail seam. It does not extend pageServer:
// that helper's five arguments are already what three seams cost, and every page test in
// the package calls it.
func pageWithTrail(t *testing.T, tail audit.Tail, err error) (*Server, *fakeTrail, *logSink) {
	t.Helper()
	s, logged := pageWithSeams(t)
	ft := &fakeTrail{tail: tail, err: err}
	s.Trail = ft
	return s, ft, logged
}

func trailFixture() []audit.Record {
	return []audit.Record{
		{At: insideWindow, Kind: "lock.asserted", Username: "casey",
			SessionID: "3", RuleID: "r-2f", RuleName: "weeknights", Detail: "session 3"},
		{At: insideWindow.Add(-time.Minute), Kind: "lockout.entered", Username: "casey",
			RuleID: "r-2f", RuleName: "weeknights"},
		{At: insideWindow.Add(-2 * time.Minute), Kind: "auth.login", Username: "parent",
			Actor: "parent"},
	}
}

// The page lists what happened, newest first, and keeps both halves of a rule.
func TestTheAuditPageListsEventsNewestFirst(t *testing.T) {
	s, ft, _ := pageWithTrail(t, audit.Tail{Records: trailFixture(), FilesRead: 1}, nil)
	rec := authGet(t, s, "/audit")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
	}
	body := flat(rec.Body.String())
	for _, want := range []string{"lock.asserted", "lockout.entered", "auth.login",
		"casey", "weeknights", "r-2f", "session 3"} {
		if !strings.Contains(body, want) {
			t.Errorf("the page never says %q:\n%s", want, body)
		}
	}
	// Newest first, by position rather than by presence: a trail rendered oldest-first
	// shows the same rows and passes every presence assertion above.
	if strings.Index(body, "lock.asserted") > strings.Index(body, "auth.login") {
		t.Error("the events are not newest first: the login from two minutes ago is " +
			"rendered before the lock that just happened")
	}
	// The page asks for the documented window. 200 is written as a literal on purpose:
	// asserting against auditWindow would pass no matter what the constant is changed to,
	// which is the "assertion that cannot fail" shape. The number is what the issue
	// promises an operator, so it belongs to the page's contract and not to the reader's.
	if ft.wantN != 200 {
		t.Errorf("the page asked for %d records, want the 200 the issue promises", ft.wantN)
	}
}

// A page that could not look must not report an empty trail.
//
// The row in TestEachUnwiredSeamNamesItself pins the log line; this pins the other half,
// which is what the operator actually reads: the refusal says the trail is not wired and
// never says the trail is empty.
func TestAnUnwiredTrailNeverClaimsTheTrailIsEmpty(t *testing.T) {
	s, _, logged := pageWithTrail(t, audit.Tail{}, nil)
	s.Trail = nil
	rec := authGet(t, s, "/audit")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503: an unwired seam is not a trail with nothing in it",
			rec.Code)
	}
	body := flat(rec.Body.String())
	if strings.Contains(body, "The trail is empty") || strings.Contains(body, "nothing has been") {
		t.Errorf("the page made a claim about what happened from a seam that cannot answer:\n%s", body)
	}
	if got := strings.Join(logged.Lines(), "\n"); !strings.Contains(got, "no audit trail is wired") {
		t.Errorf("the log never named the seam:\n%s", got)
	}
}

// A trail file that exists and holds nothing says the trail is empty.
func TestATrailWithNoRecordsSaysTheTrailIsEmpty(t *testing.T) {
	s, _, _ := pageWithTrail(t, audit.Tail{FilesRead: 1}, nil)
	body := flat(authGet(t, s, "/audit").Body.String())
	if !strings.Contains(body, "The trail is empty") {
		t.Errorf("an existing but empty trail did not say so:\n%s", body)
	}
	if strings.Contains(body, "No trail file exists") {
		t.Error("a trail file that exists was reported as missing; the operator would go " +
			"looking for a permissions problem that is not there")
	}
}

// A trail file that does not exist yet is a fresh machine, not an empty log and not a
// failed read.
func TestAMissingTrailFileSaysNothingHasBeenRecorded(t *testing.T) {
	// The reader's own contract: no generation 0 means a zero Tail and a nil error.
	s, _, _ := pageWithTrail(t, audit.Tail{}, nil)
	rec := authGet(t, s, "/audit")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: a machine that has not recorded anything is not broken",
			rec.Code)
	}
	body := flat(rec.Body.String())
	if !strings.Contains(body, "No trail file exists") {
		t.Errorf("a missing trail file did not say so:\n%s", body)
	}
	if strings.Contains(body, "The trail is empty") {
		t.Error("a missing file was reported as an empty one; those are different facts and " +
			"only one of them means the writer is working")
	}
}

// The filter matches accounts exactly.
//
// "cat" and "catie" is the whole reason this is not a substring match: the filter runs on
// a window that was already cut, so a merged pair is a wrong answer about who was held,
// on the one page a parent consults to find that out.
func TestTheAuditFilterMatchesAccountsExactly(t *testing.T) {
	rows := []audit.Record{
		{At: insideWindow, Kind: "lockout.entered", Username: "catie", RuleName: "weeknights"},
		{At: insideWindow.Add(-time.Minute), Kind: "lockout.entered", Username: "cat", RuleName: "school"},
	}
	s, _, _ := pageWithTrail(t, audit.Tail{Records: rows, FilesRead: 1}, nil)
	body := flat(authGet(t, s, "/audit?user=cat").Body.String())
	if !strings.Contains(body, "school") {
		t.Errorf("the matching account's own rule is missing:\n%s", body)
	}
	if strings.Contains(body, "weeknights") {
		t.Error("filtering to cat also showed catie's events; the filter is a claim about " +
			"who was held and a substring cannot make it")
	}
	// The count says what was filtered, so the page cannot read as the whole trail.
	if !strings.Contains(body, "1 of the 2 most recent") {
		t.Errorf("the filtered page never said it showed 1 of the 2 records it read:\n%s", body)
	}
}

// A filter that matched nothing says so about the window, not about the trail.
func TestAFilterThatMatchesNothingDoesNotClaimTheTrailIsEmpty(t *testing.T) {
	s, _, _ := pageWithTrail(t, audit.Tail{Records: trailFixture(), FilesRead: 1}, nil)
	body := flat(authGet(t, s, "/audit?user=mallory").Body.String())
	if !strings.Contains(body, "None of the 3 most recent") {
		t.Errorf("the page did not bound its own answer to the window it read:\n%s", body)
	}
	if strings.Contains(body, "The trail is empty") {
		t.Error("an account with no events in the window was reported as an empty trail; " +
			"the events may exist one generation older, which is where the operator would next look")
	}
}

// A generation that could not be read does not discard the records that were read.
func TestAPartialTrailShowsWhatWasReadAndSaysItIsIncomplete(t *testing.T) {
	s, _, logged := pageWithTrail(t,
		audit.Tail{Records: trailFixture(), FilesRead: 1},
		errors.New("audit: reading generation 1 (/var/log/plock/audit.jsonl.1): permission denied"))
	rec := authGet(t, s, "/audit")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: the records that were read are still worth showing",
			rec.Code)
	}
	body := flat(rec.Body.String())
	if !strings.Contains(body, "lock.asserted") {
		t.Errorf("the records that were readable were discarded:\n%s", body)
	}
	if !strings.Contains(body, "The trail is incomplete") {
		t.Errorf("a short page was presented as a complete one:\n%s", body)
	}
	if got := rec.Header().Get("X-Plock-Note"); got != "trail-incomplete" {
		t.Errorf("X-Plock-Note = %q, want %q: the dashboard's session note has a header so "+
			"a script can tell a short answer from an empty one, and this page needs it equally",
			got, "trail-incomplete")
	}
	got := strings.Join(logged.Lines(), "\n")
	if !strings.Contains(got, "audit trail incomplete") {
		t.Errorf("the log never carried the reason; the page tells the operator to read it:\n%s", got)
	}
	if strings.Contains(body, "permission denied") {
		t.Error("the underlying error text reached the page; it belongs in the log, and the " +
			"page's own sentence is what the operator needs")
	}
}

// Nothing that could be read is a page error, not a page.
func TestATrailThatCantBeReadAtAllIsAPageError(t *testing.T) {
	s, _, _ := pageWithTrail(t, audit.Tail{}, errors.New("audit: Reader.Path is empty"))
	rec := authGet(t, s, "/audit")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503: with no file read there is nothing to show honestly",
			rec.Code)
	}
	if strings.Contains(rec.Body.String(), "The trail is incomplete") {
		t.Error("a page that read nothing described itself as incomplete, which reads as " +
			"some events being present")
	}
}

// An account name is a string a local user chose, and it is rendered twice on this page:
// in the table and echoed back into the filter box.
func TestTheAuditPageEscapesAnAccountName(t *testing.T) {
	hostile := `<script>alert(1)</script>`
	rows := []audit.Record{{At: insideWindow, Kind: "lockout.entered", Username: hostile}}
	s, _, _ := pageWithTrail(t, audit.Tail{Records: rows, FilesRead: 1}, nil)
	body := flat(authGet(t, s, "/audit").Body.String())
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("an account name reached the page as live markup")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("the name was not escaped either, so this test cannot tell escaping from "+
			"omission:\n%s", body)
	}
}

// The page is subject to the same CSP as the dashboard, and it is the page most likely to
// gain a one-line script, because a trail looks like the kind of thing that wants a
// refresh timer.
func TestTheAuditPageHasNoInlineScriptOrStyleToBeBlockedByTheCSP(t *testing.T) {
	s, _, _ := pageWithTrail(t, audit.Tail{Records: trailFixture(), FilesRead: 1}, nil)
	rec := authGet(t, s, "/audit")
	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("no Content-Security-Policy header on a page that names who was locked out")
	}
	if strings.Contains(csp, "unsafe-inline") {
		t.Errorf("the CSP allows inline script, so the markup check below proves nothing: %q", csp)
	}
	body := flat(rec.Body.String())
	for _, bad := range []string{" onclick=", " onload=", " onerror=", " style="} {
		if strings.Contains(strings.ToLower(body), bad) {
			t.Errorf("the page carries %q, which the CSP blocks: the control works in a test "+
				"and fails in a browser", bad)
		}
	}
}

// What the read cost, because a trail that is losing lines otherwise looks clean.
func TestTheAuditPageReportsWhatTheReadSkipped(t *testing.T) {
	s, _, _ := pageWithTrail(t, audit.Tail{
		Records: trailFixture(), Skipped: 2, FilesRead: 3}, nil)
	body := flat(authGet(t, s, "/audit").Body.String())
	if !strings.Contains(body, "2 lines were present but not readable") {
		t.Errorf("unreadable lines were not reported:\n%s", body)
	}
	if !strings.Contains(body, "3 trail files were read") {
		t.Errorf("the page never said three generations contributed:\n%s", body)
	}
}

// One skipped line reads as one line, not as a grammar error in the page.
func TestTheAuditPageSaysOneLine(t *testing.T) {
	s, _, _ := pageWithTrail(t, audit.Tail{Records: trailFixture(), Skipped: 1, FilesRead: 1}, nil)
	body := flat(authGet(t, s, "/audit").Body.String())
	if !strings.Contains(body, "1 line was present but not readable") {
		t.Errorf("the singular is wrong:\n%s", body)
	}
	if !strings.Contains(body, "1 trail file was read") {
		t.Errorf("the singular is wrong:\n%s", body)
	}
}

// The filter box keeps what was typed, escaped, and offers a way back.
//
// The hostile value rides in the query rather than the fixture because the two render
// paths are different: the table prints a name the daemon recorded, and the box prints a
// name the request supplied. Only the second one is attacker-controlled by the URL, which
// is the half that a filter form newly introduces.
func TestTheFilterBoxEchoesTheAccountItWasGiven(t *testing.T) {
	hostile := `<a href="x">`
	s, _, _ := pageWithTrail(t, audit.Tail{Records: trailFixture(), FilesRead: 1}, nil)
	body := flat(authGet(t, s, "/audit?user="+url.QueryEscape(hostile)).Body.String())
	if strings.Contains(body, `<a href="x">`) {
		t.Error("a filter value reached the page as live markup")
	}
	if !strings.Contains(body, "&lt;a href=") {
		t.Errorf("the value was not echoed escaped, so the test cannot tell escaping from "+
			"an absent value:\n%s", body)
	}
	// The way back has to be a link, not a cleared box: a reset button would show every
	// event while still displaying the account name in the field.
	if !strings.Contains(body, `href="/audit"`) {
		t.Errorf("a filtered page offers no way back to the unfiltered one:\n%s", body)
	}
}
