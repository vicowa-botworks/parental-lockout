package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// The one heading on a page names that page (#219).
//
// The shell held the product's name in its only <h1>, so the first heading of every page in the
// site said the same two words: "parental lockout", on five pages, with nothing else in the
// heading layer to tell them apart. A reader who cannot use the tab strip -- because the page has
// already loaded, which is every moment after the page has loaded -- had no way to answer "which
// page am I" that the document offered in markup. The decision (D-0016) put the page's own name
// in the <h1> and demoted the product's name to a labelled paragraph in the header.
//
// The heading is asserted against the page's own <title> rather than against a list of expected
// strings, and that choice is the guard. A list pins the pages it knows about, is silent about the
// sixth page, and passes on a page whose tab strip and heading both changed to something nobody
// chose. What #219 was about is the *relationship* -- one definition, rendered in both places, so
// they cannot disagree -- and comparing one against the other is the only assertion that names it.
func TestEveryPageNamesItselfInTheHeading(t *testing.T) {
	// A page that did not answer 200 has no heading to check, and reporting "0 <h1> elements" for
	// a 302 or a 503 would blame the template for a fixture. That is not hypothetical: /audit
	// answers 503 on a server with no trail seam wired, and /login answers 302 to / to a caller
	// who is already signed in. Both are the wrong verdict for the wrong reason.
	check := func(path string, rec *httptest.ResponseRecorder) {
		t.Helper()
		if rec.Code != http.StatusOK {
			t.Errorf("%s answered %d instead of a page, so the heading was never rendered: %q",
				path, rec.Code, rec.Body.String())
			return
		}
		for _, complaint := range headingComplaints(rec.Body.String()) {
			t.Errorf("%s: %s", path, complaint)
		}
	}

	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	// The pages a signed-in GET renders through the shell. The two result pages are not here
	// because a GET cannot reach them, which is the same ceiling
	// TestThePageHasNoInlineScriptOrStyleToBeBlockedByTheCSP states for the CSP guard: a loop over
	// GET-able paths goes blind on exactly the pages nobody thought to add. Their sibling call is
	// TestTheResultPagesNameThemselvesInTheHeading, below.
	for _, path := range []string{"/", "/users", "/profiles", "/settings"} {
		check(path, authGet(t, s, path))
	}

	// /audit needs the trail, not the account list: pageServer leaves s.Trail at whatever
	// newTestServer stubbed, and the handler answers 503 without one.
	trail, _, _ := pageWithTrail(t, audit.Tail{Records: trailFixture(), FilesRead: 1}, nil)
	check("/audit", authGet(t, trail, "/audit"))

	// /login is the one page in the site that is not the shell, and it had the product's name in
	// its <h1> just like the rest of them -- so it is inside the rule. It has to be fetched by a
	// caller with no session: handleLoginPage redirects a signed-in visitor to / (#219 is a claim
	// about headings, not about that redirect).
	req := httptest.NewRequest("GET", "/login", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	check("/login", rec)
}

// TestTheResultPagesNameThemselvesInTheHeading is the POST half of the rule above.
//
// /users/<name>/lock-now and /users/<name>/test-lock exist only as the answer to a POST, so the
// loop above cannot see them however its list is written. They are rendered through the same
// shell, so the same claim is being made about them -- and the shell is one template, so the only
// thing that could make them differ is that nobody looked.
func TestTheResultPagesNameThemselvesInTheHeading(t *testing.T) {
	for _, page := range []struct {
		path string
		srv  func(*testing.T) *Server
	}{
		{"/users/casey/lock-now", func(t *testing.T) *Server {
			f := newLockFixture(t)
			f.setLockErr(platform.ErrUnconfirmed)
			return f.s
		}},
		{"/users/casey/test-lock", func(t *testing.T) *Server {
			return newProbeFixture(t).s
		}},
	} {
		rec := authPost(t, page.srv(t), page.path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s answered %d", page.path, rec.Code)
		}
		body := rec.Body.String()
		// The same guard against checking nothing that the CSP sibling has: without the table this
		// is a refusal, and a refusal carries no shell for the heading to be wrong in.
		if !strings.Contains(body, `<table class="rows"`) {
			t.Fatalf("%s rendered no rows table, so nothing was checked: %s", page.path, body)
		}
		for _, complaint := range headingComplaints(body) {
			t.Errorf("%s: %s", page.path, complaint)
		}
	}
}

var (
	h1Block = regexp.MustCompile(`(?s)<h1[^>]*>(.*?)</h1>`)
	// Every heading element, for the "the brand survived as a heading" case: the check below has
	// to look at what is left once the document's headings are set aside, or a brand line put back
	// into an <h2> would still count as visible.
	headingBlock = regexp.MustCompile(`(?s)<h[1-6][^>]*>.*?</h[1-6]>`)
	titleBlock   = regexp.MustCompile(`(?s)<title>(.*?)</title>`)
)

// headingComplaints checks one rendered page against the rule #219 decided on.
//
// Four clauses, each a different way the change comes undone while the page still looks right:
//
//   - not exactly one <h1>. Zero is the defect itself. Two is one edit away in a shell that is
//     shared by every page, and a page with two headings answers "which page am I" twice.
//   - the <h1> and the tab strip disagreeing, in either direction. That is the state a reader
//     cannot resolve from inside the page, which is what the issue is for.
//   - the <h1> naming the product. The defect, back under a different element.
//   - the product's name nowhere in the visible document. Demotion is not deletion: the masthead
//     still has to say what this software is, and the only other place it said so is the half of
//     <title> a screen reader stops announcing once the page has loaded.
func headingComplaints(body string) (bad []string) {
	blocks := h1Block.FindAllStringSubmatch(body, -1)
	if len(blocks) != 1 {
		return append(bad, fmt.Sprintf(
			"the page carries %d <h1> element(s); #219 decided there is exactly one and it names "+
				"the page, so %d is either the defect back or a second heading nobody decided on",
			len(blocks), len(blocks)))
	}
	got := strings.TrimSpace(blocks[0][1])

	titles := titleBlock.FindStringSubmatch(body)
	if titles == nil {
		return append(bad, "the page has no <title>, so there is nothing here to check the heading against")
	}
	want := pageNameFromTitle(titles[1])

	// The order is the diagnosis. Empty and product-name are both mismatches, so if the mismatch
	// clause ran first they could never say which of the two failures this is -- and "the defect is
	// back in the heading" is the one answer a reader of the failure needs first.
	switch {
	case got == "":
		bad = append(bad, "the page's <h1> is empty")
	case isProductName(got):
		bad = append(bad, fmt.Sprintf(
			"the <h1> names the product (%q), which is #219's defect: every page in the site then "+
				"opens with the same two words and the heading says nothing about which one this is", got))
	case !strings.EqualFold(got, want):
		bad = append(bad, fmt.Sprintf(
			"the <h1> reads %q and the tab strip reads %q: #219 wants the same words in both, so a "+
				"reader who is already inside the page can still answer \"which page am I\"", got, want))
	}

	// The brand, still visible. <title> is removed with the headings so this cannot be satisfied
	// by the tab strip alone -- the one place the product's name is guaranteed to appear is the
	// place nobody reads after load, and that is exactly the place #219 says is not enough.
	visible := headingBlock.ReplaceAllString(titleBlock.ReplaceAllString(body, ""), "")
	if !strings.Contains(strings.ToLower(visible), "parental lockout") {
		bad = append(bad, "the product's name appears nowhere outside <title> and the headings: the "+
			"masthead still has to say what this software is, and demotion is not deletion")
	}
	return bad
}

// pageNameFromTitle is the page's name as the tab strip states it: everything before the brand
// separator. The shell writes " · " and the login page writes the entity for the same character,
// because the login document predates the shell and has no template to share with it.
func pageNameFromTitle(title string) string {
	for _, sep := range []string{" &middot; ", " · ", " &bull; ", " - "} {
		if i := strings.Index(title, sep); i >= 0 {
			return strings.TrimSpace(title[:i])
		}
	}
	return strings.TrimSpace(title)
}

// isProductName is the two spellings this repository has ever used for the product's name on a
// page: the shell's words and the login page's title suffix.
func isProductName(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "parental lockout", "plock":
		return true
	}
	return false
}
