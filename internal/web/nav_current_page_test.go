package web

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
)

// #204: the navigation has to say which page you are looking at, in the bytes and not only on
// the screen.
//
// Measured before the fix, at a 360 px viewport, on all five photographed pages: every nav link
// computed identically -- `rgb(0, 0, 238)`, weight 400, underline, no border -- including the link
// for the page being rendered, because app.css had no selector for `nav a` and the whole row was
// user-agent default. The stylesheet half is in app.css; this file guards the half a stylesheet
// cannot supply. `aria-current` is on the DOM rather than simulated by a body-class selector
// because the attribute is the part a screen reader announces, and #204's finding is that the
// current page was unmarked for a parent and for assistive technology at the same time.
//
// The count is the load-bearing assertion, not the value. Zero means the navigation says nothing
// about where you are, which is the defect; two means it is confidently wrong about a page, which
// is worse than the defect and is exactly what a hand-maintained "which page am I" field produces
// when a route is renamed. `.Page` comes from `render`, one line from the route name, so neither
// direction is reachable by forgetting to update a second list.
func TestEachPageMarksExactlyOneNavLinkAsCurrent(t *testing.T) {
	for _, page := range navPages {
		body := authGet(t, navServer(t), "/"+page.name).Body.String()
		marked := currentNavLinks(t, body, page.name)
		if len(marked) != 1 {
			t.Errorf("/%s marks %d nav links as the current page (%v): zero is #204 and two is a "+
				"navigation that lies about where the parent is", page.name, len(marked), marked)
			continue
		}
		if marked[0] != page.href {
			t.Errorf("/%s marks %q as current, want %q -- the link and the route disagree, so the "+
				"stylesheet is bolding the wrong word", page.name, marked[0], page.href)
		}
	}
}

// TestTheNavLinkVocabularyIsTheTemplatesVocabulary pins the two names a page goes by.
//
// `aria-current` is chosen from `.Page`, the *route* vocabulary (`dashboard`), while the body class
// is rendered from the template's own `pagename` definition (`status` on the same page -- the one
// place the two vocabularies part). Two tables that name the same page differently are the shape
// this repo refuses: the forgotten row keeps rendering, and only the affordance that reads one of
// them goes quietly wrong. So the pairing is asserted on every page instead of assumed, and adding
// a page means adding one row here, where the failure names both halves.
func TestTheNavLinkVocabularyIsTheTemplatesVocabulary(t *testing.T) {
	for _, page := range navPages {
		body := authGet(t, navServer(t), "/"+page.name).Body.String()
		class, ok := bodyPageClass(body)
		if !ok {
			t.Errorf("/%s renders no page-<name> body class, so nothing pins its pagename", page.path)
			continue
		}
		if class != page.pagename {
			t.Errorf("%s: the shell marks the nav from route %q while the template calls itself %q "+
				"(body class page-%s); the pairing in navPages is what keeps those honest",
				page.name, page.href, class, class)
		}
	}
}

// TestAResultPageMarksNoNavLinkAsCurrent states the deliberate gap rather than leaving it to be
// discovered as a bug.
//
// The POST result pages (`/users/<name>/lock-now`, `/users/<name>/test-lock`) are not in the
// navigation, so no link is theirs to mark. The alternative -- bolding "Accounts" while a parent
// reads a lock verdict -- claims they are looking at a page they are not, which is the same class
// of lie as the unmarked current page, pointed the other way. If a later design decides a result
// page belongs under Accounts, this test is the one to change, and app.css already marks whatever
// carries the attribute.
func TestAResultPageMarksNoNavLinkAsCurrent(t *testing.T) {
	f := newLockFixture(t)
	rec := authPost(t, f.s, "/users/casey/lock-now", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("lock-now rendered %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	nav, ok := between(body, "<nav>", "</nav>")
	if !ok {
		t.Fatalf("the result page renders no <nav> block, so this guard is checking nothing")
	}
	if marked := strings.Count(nav, `aria-current="page"`); marked != 0 {
		t.Errorf("a page that is not in the navigation marks %d links as current: %s", marked, nav)
	}
}

// navPages is the pairing the two tests above read: the URL as the driver and a parent type it,
// the nav href that stands for it, and the name the page's own template claims.
var navPages = []struct {
	name     string
	href     string
	pagename string
}{
	{"", "/", "status"},
	{"users", "/users", "users"},
	{"audit", "/audit", "audit"},
	{"profiles", "/profiles", "profiles"},
	{"settings", "/settings", "settings"},
}

// navServer builds a server that can answer every page in navPages.
//
// /audit needs one seam filled: with no trail behind it the page renders its refusal rather than
// the shell, and a guard that reads <nav> out of a refusal page is reading nothing. (This is why
// #204's own measurement lists five pages while TestTheMastheadLinksAreSeparatedInTheRenderedBytes
// walks four -- that test never asked the audit page for a masthead.) An empty trail is enough
// here: the navigation is the shell's, so an account with no events still has one, and inventing
// records to test a link would put this guard in the audit page's business.
func navServer(t *testing.T) *Server {
	t.Helper()
	s, _, _ := pageWithTrail(t, audit.Tail{FilesRead: 0}, nil)
	return s
}

var markedLink = regexp.MustCompile(`<a href="([^"]+)"[^>]*aria-current="page"`)

// currentNavLinks returns the href of every nav link carrying aria-current="page".
//
// Scoped to the <nav> block on purpose: the masthead's <h1> links to `/` as well, and #204 records
// that the two `/` links differ only in weight. The title is not a navigation target, so it is not
// this attribute's subject, and a test that counted it would either fail on the dashboard or have
// to mark the title -- which would put a second link on the page you are on and make the styling
// question the issue asked about impossible to answer.
func currentNavLinks(t *testing.T, body, name string) []string {
	t.Helper()
	nav, ok := between(body, "<nav>", "</nav>")
	if !ok {
		t.Errorf("/%s renders no <nav> block, so there is no navigation to mark", name)
		return nil
	}
	var hrefs []string
	for _, m := range markedLink.FindAllStringSubmatch(nav, -1) {
		hrefs = append(hrefs, m[1])
	}
	return hrefs
}

var pageClass = regexp.MustCompile(`(?s)<body[^>]*class="[^"]*\bpage-([a-z-]+)\b`)

func bodyPageClass(body string) (string, bool) {
	m := pageClass.FindStringSubmatch(body)
	if m == nil {
		return "", false
	}
	return m[1], true
}
