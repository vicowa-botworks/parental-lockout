package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
	"github.com/vicowa-botworks/parental-lockout/internal/auth"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// insideWindow is inside the school-night rule the fixtures below define, so the
// dashboard has something to report.
var insideWindow = time.Date(2026, 3, 18, 23, 0, 0, 0, time.UTC)

// pageVerifier is one production-legal admin credential, minted once for the package.
//
// Production-legal is not a detail: the authenticator checks the stored cost at the
// trust boundary, so a TestIterations fixture is refused there with a 500 -- correctly.
// A page fixture that cannot log in is not testing the page. The cost is one ~100 ms
// hash for the whole package rather than one per fixture, because every page test logs
// in and none of them should pay for the KDF.
var pageVerifier = func() string {
	v, err := auth.Hasher{Iterations: auth.MinIterations}.Hash("hunter2")
	if err != nil {
		panic("the test verifier could not be minted: " + err.Error())
	}
	return v
}()

func pageConfig() *config.Config {
	c := config.Default()
	// A deployable config has an admin in it. This stopped being optional when the
	// login path started reading its credential through the same Config seam the pages
	// use: a fixture that stubbed Config for page data was silently also saying "this
	// machine has no administrator", and every login then failed for a reason the test
	// had nothing to do with.
	c.Admin = config.Admin{Username: "parent", Password: pageVerifier}
	c.Profiles = []config.Profile{{
		ID: "school", Name: "School nights", Timezone: "UTC",
		Rules: []config.Rule{{ID: "r1", Name: "weeknights",
			Days:  []string{"sun", "mon", "tue", "wed", "thu"},
			Start: "21:00", End: "06:30", Enabled: true}},
	}}
	c.Users = []config.User{{Username: "casey", ProfileID: "school", Enabled: true}}
	return c
}

// authGet performs a real login and returns the body of an authenticated GET, so
// none of these tests hand-assemble state the server never issues.
func authGet(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	resp, _ := login(t, s, "parent", "hunter2")
	req := httptest.NewRequest("GET", path, nil)
	// A browser's Accept. Without it the server answers the unauthenticated case as
	// JSON -- which is right for a programmatic caller -- and the test would be
	// asserting a status code no parent's browser ever sees.
	req.Header.Set("Accept", "text/html")
	for _, c := range resp.Cookies() {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// pageServer is a server with all three content seams wired to fixed state.
func pageServer(t *testing.T, cfg *config.Config, sessions []platform.Session, accounts []users.Account, cfgErr, acctErr error) *Server {
	t.Helper()
	s, _, _ := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	// newTestServer assembles a Server by hand rather than through New, so the asset
	// index the shell links the stylesheet from does not exist yet. Building it here
	// is the same call New makes; without it every page renders unstyled and the
	// stylesheet test would be asserting nothing.
	m, err := AssetMap(Assets())
	if err != nil {
		t.Fatalf("the embedded assets are not readable: %v", err)
	}
	s.assets = m
	s.Now = func() time.Time { return insideWindow }
	if cfgErr != nil {
		s.Config = func() (*config.Config, error) { return nil, cfgErr }
	} else {
		s.Config = func() (*config.Config, error) { return cfg, nil }
	}
	s.Lockables = func() ([]platform.Session, error) { return sessions, nil }
	if acctErr != nil {
		s.Accounts = func() (*users.Listing, error) { return nil, acctErr }
	} else {
		s.Accounts = func() (*users.Listing, error) {
			return &users.Listing{Accounts: accounts, UIDMin: users.DefaultUIDMin, UIDMax: users.DefaultUIDMax}, nil
		}
	}
	return s
}

// Nothing outside the login page is readable without a session.
func TestTheContentPagesRequireASession(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	for _, path := range []string{"/", "/users", "/audit"} {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Accept", "text/html")
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		if got := rec.Result().StatusCode; got != http.StatusSeeOther && got != http.StatusFound {
			t.Errorf("GET %s without a session = %d, want a redirect to the login page", path, got)
		}
	}
}

// The dashboard names the account and says what is holding it.
func TestTheDashboardNamesTheAccountAndTheRule(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	rec := authGet(t, s, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"casey", "locked out", "weeknights", "06:30"} {
		if !strings.Contains(body, want) {
			t.Errorf("the page never says %q:\n%s", want, body)
		}
	}
}

// A username is attacker-influenced text.
//
// The password file is root-owned, but a machine can carry accounts created by
// somebody else, and an account name is the one field a child with a shell on the box
// can plausibly control. The page must render it as a name.
func TestAnAccountNameIsRenderedAsTextNotMarkup(t *testing.T) {
	c := pageConfig()
	c.Users[0].Username = `<script>alert(1)</script>`
	s := pageServer(t, c, nil, nil, nil, nil)
	body := authGet(t, s, "/").Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("the account name was written into the document as markup")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("the account name is not in the page at all, escaped or otherwise")
	}
}

// The response carries a CSP with no 'unsafe-inline', and the page contains no
// inline handler or style attribute to be blocked by it.
//
// Both halves matter. The header alone means a stray onclick silently stops working
// in a browser while every test still passes; the markup alone means the control is
// absent.
func TestThePageHasNoInlineScriptOrStyleToBeBlockedByTheCSP(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)

	// Every page, not just the dashboard. A page added later with a style="" attribute
	// works in every test that only looks at "/" and fails only in a browser.
	//
	// The list is the guard's ceiling and it is worth stating out loud: this loop can only
	// see what a GET renders. A page that exists only as the response to a POST is not
	// reachable from here no matter what path is added, so such a page has to call
	// cspComplaints on the body it obtained by posting -- which internal/web/lock_now_test.go
	// does for the lock result page. Adding a POST-only page without that sibling call is
	// how this guard goes blind while staying green.
	for _, path := range []string{"/", "/users", "/profiles", "/settings", "/login"} {
		rec := authGet(t, s, path)
		for _, complaint := range cspComplaints(rec.Header().Get("Content-Security-Policy"), rec.Body.String()) {
			t.Errorf("%s: %s", path, complaint)
		}
	}
}

// The masthead's links have to be separated in the rendered bytes, not just in the template.
//
// This is #108's failure arriving through the navigation instead of a table cell: a Go template
// action trims the whitespace on the side its dashes point at, so a comment written between two
// inline elements deletes the space that separated them. The template still reads fine and the
// page still passes the CSP guard, the route guard and every width test that looks at overflow
// rather than at word boundaries -- the only thing that showed it was the 360 px photograph,
// where the nav rendered "ProfilesSettings". An assertion on the bytes is what keeps the next
// comment inserted between two links from doing it again.
func TestTheMastheadLinksAreSeparatedInTheRenderedBytes(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)

	for _, path := range []string{"/", "/users", "/profiles", "/settings"} {
		body := authGet(t, s, path).Body.String()
		nav, ok := between(body, "<nav>", "</nav>")
		if !ok {
			t.Errorf("%s renders no <nav> block, so this guard is checking nothing", path)
			continue
		}
		// /login is deliberately not in this list: it is the pre-auth page and has no masthead by
		// design, so asking it for one would only teach the next reader that the guard is wrong
		// about the login page. The four above are every page the shell renders.
		//
		// The list is what stops an empty nav passing: a page whose links all vanished would
		// certainly contain no glued pair.
		for _, label := range []string{"Status", "Accounts", "Audit", "Profiles", "Settings"} {
			if !strings.Contains(nav, ">"+label+"</a>") {
				t.Errorf("%s: the masthead never links to %s", path, label)
			}
		}
		for _, glued := range []string{"</a><a", "</a><button"} {
			if i := strings.Index(nav, glued); i >= 0 {
				lo := max(0, i-40)
				t.Errorf("%s: the masthead emits %q with nothing between the two elements, so they "+
					"render as one word: ...%s...", path, glued, nav[lo:min(len(nav), i+60)])
			}
		}
	}
}

// between returns the text between the first occurrence of open and the next close.
func between(body, open, close string) (string, bool) {
	i := strings.Index(body, open)
	if i < 0 {
		return "", false
	}
	rest := body[i+len(open):]
	j := strings.Index(rest, close)
	if j < 0 {
		return "", false
	}
	return rest[:j], true
}

// mastheadLink is one anchor in the shell's navigation, as the rendered bytes declare it.
type mastheadLink struct {
	href    string
	current string // the aria-current value, "" when the attribute is absent
}

// mastheadLinks reads the navigation out of one rendered page.
//
// Bytes rather than a DOM, in the file that already asserts on the masthead's bytes: the
// claim below is about what the template emits, and the same choice #204's own measurement
// made when it counted `aria-current=` in the rendered files rather than in a browser. What
// the bytes cannot say -- whether the mark is actually visible -- is the half the capture
// driver measures through Chrome instead (testing/capture/cdp.py, require_nav_marked).
func mastheadLinks(t *testing.T, what, body string) []mastheadLink {
	t.Helper()
	nav, ok := between(body, "<nav>", "</nav>")
	if !ok {
		t.Fatalf("%s: the page renders no <nav> block, so nothing below can be claimed about it", what)
	}
	var links []mastheadLink
	for _, tag := range mastheadATag.FindAllString(nav, -1) {
		href, cur := "", ""
		if m := mastheadHref.FindStringSubmatch(tag); m != nil {
			href = m[1]
		}
		if m := mastheadAria.FindStringSubmatch(tag); m != nil {
			cur = m[1]
		}
		links = append(links, mastheadLink{href: href, current: cur})
	}
	return links
}

var (
	mastheadATag = regexp.MustCompile(`<a\b[^>]*>`)
	mastheadHref = regexp.MustCompile(`href="([^"]*)"`)
	mastheadAria = regexp.MustCompile(`aria-current="([^"]*)"`)
)

// TestTheShellMarksTheLinkForThePageItRenders is #204's first criterion, on the bytes.
//
// The navigation is rendered by the shared shell, so there is exactly one place that can
// know which page is being written, and exactly one way for it to go wrong on every page at
// once: the five strings compared in templates/page.tmpl are the names handlers pass to
// render(), and a rename on either side leaves a row of five unmarked links that looks
// correct to everyone looking at it. A screen reader announcing no current page is not a
// visible symptom, which is why the guard is a test and not a screenshot.
//
// Three things are refused here, and the third is the one a naive test misses:
//
//   - no link marked, the shipped bug;
//   - the wrong link marked, which is worse than none, because it reads as an answer;
//   - more than one element on the page marked. The masthead's h1 is also a link to /, so
//     a selector keyed on the href rather than on the page name would light up two elements
//     on the dashboard and #204's criterion says at most one element may be so marked -- the
//     count is therefore taken over the whole body, not over the navigation.
func TestTheShellMarksTheLinkForThePageItRenders(t *testing.T) {
	// One server per page, because the helpers differ: /audit is the one content page
	// pageServer cannot render -- it wires no trail seam, so the page answers its failure
	// page instead, and a failure page has no navigation to mark.
	for _, page := range []struct {
		path, name string
		trail      bool
	}{
		{"/", "dashboard", false},
		{"/users", "users", false},
		{"/profiles", "profiles", false},
		{"/settings", "settings", false},
		{"/audit", "audit", true},
	} {
		var s *Server
		if page.trail {
			s, _, _ = pageWithTrail(t, audit.Tail{Records: trailFixture(), FilesRead: 1}, nil)
		} else {
			s = pageServer(t, pageConfig(), nil, nil, nil, nil)
		}
		rec := authGet(t, s, page.path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s (%s): answered %d, so there is no page here to mark: %s",
				page.path, page.name, rec.Code, rec.Body.String())
			continue
		}
		body := rec.Body.String()
		links := mastheadLinks(t, page.path, body)
		// A page whose navigation vanished would contain no unmarked link and no wrong one,
		// so the count is the control for everything below it.
		if len(links) != 5 {
			t.Errorf("%s: the masthead renders %d links, want 5", page.path, len(links))
			continue
		}
		var marked []mastheadLink
		for _, l := range links {
			if l.current != "" {
				marked = append(marked, l)
			}
		}
		if len(marked) != 1 {
			t.Errorf("%s (%s): %d of the five navigation links carry aria-current, want exactly 1 "+
				"-- an unmarked row is the defect #204 opened, and two is a navigation with two answers",
				page.path, page.name, len(marked))
			continue
		}
		if marked[0].href != page.path {
			t.Errorf("%s (%s): the marked link is %q, so the page claims to be somewhere else",
				page.path, page.name, marked[0].href)
		}
		if marked[0].current != "page" {
			t.Errorf("%s: aria-current=%q; `page` is the value that means \"this link is the current "+
				"page\" -- `true` means the same thing to no assistive technology in a navigation",
				page.path, marked[0].current)
		}
		if n := strings.Count(body, "aria-current"); n != 1 {
			t.Errorf("%s: %d elements in the page carry aria-current, want 1; the masthead's title is "+
				"also a link to /, so a mark keyed on the href rather than on the page marks two",
				page.path, n)
		}
	}
}

// TestTheResultPagesMarkNothing is the other half of #204's criterion: at most one element.
//
// Both result pages are rendered through the same shell, so they carry the same navigation --
// and neither of them is one of the five pages the navigation contains. Marking "Accounts"
// from /users/casey/lock-now would tell a reader who cannot see the page that they are on
// /users, which is a confident wrong answer rather than an honest absence. So the absence has
// to be pinned here: without this test the cheapest way to satisfy the test above on the two
// POST pages is a comparison that matches on a path prefix, and the five GET tests stay green.
//
// The link count is asserted too, for the reason it is asserted above: zero marks on a page
// with no navigation would pass this test while failing #204 outright.
func TestTheResultPagesMarkNothing(t *testing.T) {
	for _, page := range []struct {
		desc string
		body func(t *testing.T) string
	}{
		{"the lock result page", func(t *testing.T) string {
			return authPost(t, newLockFixture(t).s, "/users/casey/lock-now", "").Body.String()
		}},
		{"the backend probe result page", func(t *testing.T) string {
			return authPost(t, newProbeFixture(t).s, "/users/casey/test-lock", "").Body.String()
		}},
	} {
		body := page.body(t)
		links := mastheadLinks(t, page.desc, body)
		if len(links) != 5 {
			t.Errorf("%s renders %d navigation links, want 5 -- the shell renders its navigation here, "+
				"and a page with no navigation is not the absence this test is recording", page.desc, len(links))
			continue
		}
		if n := strings.Count(body, "aria-current"); n != 0 {
			t.Errorf("%s carries aria-current %d time(s): this page is not one of the five the "+
				"navigation links to, so marking a link here claims the reader is somewhere they are not",
				page.desc, n)
		}
	}
}

// cspComplaints checks one rendered page against the CSP contract, from both halves.
//
// The header alone means a stray onclick silently stops working in a browser while every
// test still passes; the markup alone means the control is absent. It is a function rather
// than inline code because the page-by-page loop above can only reach pages a GET renders,
// and a POST-only page has to check the body it posted for.
func cspComplaints(csp, body string) (bad []string) {
	if csp == "" {
		return []string{"no Content-Security-Policy header on a page that shows who is locked out"}
	}
	if strings.Contains(csp, "unsafe-inline") {
		bad = append(bad, fmt.Sprintf("the CSP allows inline script, so the markup check below proves nothing: %q", csp))
	}
	for _, marker := range []string{" onclick=", " onload=", " onerror=", " style="} {
		if strings.Contains(strings.ToLower(body), marker) {
			bad = append(bad, fmt.Sprintf("the page carries %q, which the CSP blocks: the control works in a test and fails in a browser", marker))
		}
	}
	return bad
}

// A page that cannot read the configuration says so.
//
// An empty table would be a different claim: it reads as "nothing is managed", which
// is the most reassuring wrong answer this interface can produce, and it is exactly
// what a parent sees in the minute when the config file has actually gone missing.
func TestADashboardThatCannotReadTheConfigSaysSo(t *testing.T) {
	s, _ := pageWithSeams(t)
	rec := authGetAfterBreaking(t, s, func(s *Server) {
		s.Config = func() (*config.Config, error) {
			return nil, errors.New("open /etc/plock/config.json: no such file")
		}
	}, "/")
	if rec.Code == http.StatusOK {
		t.Error("the page rendered as though nothing were wrong")
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "configuration") {
		t.Errorf("the refusal does not say what failed: %q", rec.Body.String())
	}
}

// The same for the accounts page, where the missing read is the password file.
func TestTheUsersPageRefusesWhenTheAccountsCannotBeRead(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, errors.New("open /etc/passwd: permission denied"))
	rec := authGet(t, s, "/users")
	if rec.Code == http.StatusOK {
		t.Error("an unreadable account list rendered as an empty one")
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "account") {
		t.Errorf("the refusal does not say what failed: %q", rec.Body.String())
	}
}

// A missing seam is refused the same way a failing read is.
//
// The zero value of `Config` is nil, and a handler that treated nil as "no accounts"
// would let a daemon built without its wiring advertise an empty household forever.
func TestAContentSeamThatWasNeverWiredIsRefused(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	rec := authGet(t, s, "/")
	if rec.Code == http.StatusOK {
		t.Error("a server with no configuration source rendered a dashboard anyway")
	}
}

// The session column is the one thing that can be missing without taking the page
// down: which child is held, and until when, does not depend on who is logged in.
//
// What it must not do is say "no desktop session". That sentence is a claim that nobody
// has a session, and a failed read establishes nothing of the kind -- it is the same
// rendering the page gives a machine where nobody is logged in, so a broken session
// backend looks like a quiet house. The page has to distinguish "nobody is logged in"
// from "we could not look", and the two cases are pinned separately below.
func TestTheDashboardStillRendersWhenTheSessionListIsGone(t *testing.T) {
	c := pageConfig()
	s := pageServer(t, c, nil, nil, nil, nil)
	s.Lockables = func() ([]platform.Session, error) { return nil, errors.New("dbus: transport not connected") }
	rec := authGet(t, s, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("the whole page went down for the session column: %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "casey") {
		t.Error("the accounts vanished with the sessions")
	}
	if !strings.Contains(body, "session list unavailable") {
		t.Errorf("a failed session read is not said as a failure; the body says %q instead, which is "+
			"a claim about who is logged in that the failed read did not establish", "no desktop session")
	}
	if strings.Contains(body, "no desktop session") {
		t.Error("the failed read rendered as an empty session list; a parent cannot tell a broken " +
			"session backend from a machine nobody is logged into")
	}
}

// The other half of the same column: a read that worked and found nobody. This is the
// case "no desktop session" is true for, and it must keep that wording -- otherwise the
// new failure sentence is paid for by making the honest case vague.
func TestAnEmptySessionListStillSaysNobodyIsLoggedIn(t *testing.T) {
	rec := authGet(t, pageServer(t, pageConfig(), nil, nil, nil, nil), "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "no desktop session") {
		t.Error("a successful read that found no sessions does not say so")
	}
	if strings.Contains(body, "session list unavailable") {
		t.Error("a read that worked is reported as a failure")
	}
}

// Sessions are shown with the state the backend reported.
func TestTheSessionColumnCarriesTheLockState(t *testing.T) {
	s := pageServer(t, pageConfig(),
		[]platform.Session{{ID: "2", Username: "casey", Seat: "seat0", Type: "wayland", Locked: true}},
		nil, nil, nil)
	body := authGet(t, s, "/").Body.String()
	if !strings.Contains(body, "wayland") || !strings.Contains(body, "locked") {
		t.Errorf("the session column does not show what the backend reported:\n%s", body)
	}
}

// The accounts page shows the gap the parent has not been told about: a human account
// on the machine that nothing enforces.
func TestTheUsersPageShowsTheUnmanagedCount(t *testing.T) {
	accounts := []users.Account{
		{Username: "casey", UID: 1000, Home: "/home/casey", Shell: "/bin/bash"},
		{Username: "bruce", UID: 1001, Home: "/home/bruce", Shell: "/bin/bash"},
	}
	s := pageServer(t, pageConfig(), nil, accounts, nil, nil)
	body := authGet(t, s, "/users").Body.String()
	if !strings.Contains(body, "bruce") {
		t.Error("an account on the machine is missing from the accounts page")
	}
	if !strings.Contains(body, "1") || !strings.Contains(strings.ToLower(body), "not managed") {
		t.Errorf("the page does not count the unmanaged account:\n%s", body)
	}
}

// The profile dropdown shows the name the parent typed, not the id.
func TestTheUsersPageNamesProfilesTheWayTheParentDid(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	body := authGet(t, s, "/users").Body.String()
	if !strings.Contains(body, "School nights") {
		t.Errorf("the page shows the profile id rather than its name:\n%s", body)
	}
}

// Every page has a template, and every template has a route.
//
// The failure this pins is adding a handler and forgetting the .tmpl: the route then
// answers 500 with a line in the log, which is a bug report that takes an afternoon
// to read as a missing file.
func TestEveryContentRouteHasATemplate(t *testing.T) {
	got := strings.Join(pageNames(), ",")
	// "audit" belongs here for the same reason the others do: a handler without a
	// .tmpl answers 500 on the real daemon and passes every handler test.
	for _, want := range []string{"dashboard", "users", "audit"} {
		if !strings.Contains(got, want) {
			t.Errorf("no template named %q is embedded; found %q", want, got)
		}
	}
}

// The stylesheet is linked by its hashed URL.
//
// A plain /static/app.css would be served with the page's no-store, so the CSS would
// be re-fetched on every page view -- or, if it were cached instead, a fix would not
// appear on a parent's phone until the browser felt like revalidating. That second
// one is the change that gets reported as "you did not deploy it".
func TestTheStylesheetIsLinkedByItsHashedURL(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	body := authGet(t, s, "/").Body.String()
	if !strings.Contains(body, "/static/app.") || strings.Contains(body, "/static/app.css\"") {
		t.Errorf("the stylesheet is not linked by its content hash:\n%s", body)
	}
}

// The page is not cacheable.
func TestTheContentPagesAreNotCacheable(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	for _, path := range []string{"/", "/users", "/audit"} {
		if got := authGet(t, s, path).Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("GET %s Cache-Control = %q, want no-store", path, got)
		}
	}
}

// A handler that names a template nobody embedded is a programming error, and the
// useful answer is a 500 plus a log line naming the page -- not a panic that the
// recovery middleware turns into an "internal error" with nothing saying which
// handler was missing what.
func TestRenderingAPageThatWasNeverEmbeddedSaysWhichOne(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	s.render(rec, httptest.NewRequest("GET", "/nope", nil), "nonexistent", struct{}{})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// A profile with no name is possible in the document, and a dropdown that renders an
// empty entry is indistinguishable from a broken control.
func TestAnUnnamedProfileIsStillNamedSomething(t *testing.T) {
	c := pageConfig()
	c.Profiles[0].Name = ""
	s := pageServer(t, c, nil, nil, nil, nil)
	body := authGet(t, s, "/users").Body.String()
	if strings.Contains(body, "<option value=\"school\"></option>") || strings.Contains(body, "school</span>\n") {
		t.Error("the profile rendered as an empty label")
	}
	// The id is the fallback, so the row still says which profile is attached.
	if !strings.Contains(body, "school") {
		t.Errorf("an unnamed profile lost its identity entirely:\n%s", body)
	}
}

// The listing's notes have to arrive on the page, which is the reason the accounts seam
// carries a *users.Listing rather than its []Account.
//
// Before the seam was widened, internal/users could write as many notes as it liked and
// none of them could reach a browser: the seam's signature had nowhere to put them. A
// child excluded by a UID_MIN the operator never meant was simply absent from the table,
// which is the exact failure the package's own doc comment says it exists to prevent.
func TestTheAccountsPageRendersTheListingsNotes(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	s.Accounts = func() (*users.Listing, error) {
		return &users.Listing{
			Accounts: []users.Account{{Username: "casey", UID: 1000, GID: 1000, Home: "/home/casey", Shell: "/bin/bash"}},
			UIDMin:   1000,
			UIDMax:   60000,
			Notes:    []string{"the file sets no UID_MIN; assuming 1000, which hides any account below it"},
		}, nil
	}
	rec := authGet(t, s, "/users")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /users = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "the file sets no UID_MIN") {
		t.Error("the listing's note never reached the page; the seam is dropping it somewhere")
	}
	// The bounds are part of the same explanation: "where is my kid" is answered by the
	// numbers that were applied.
	if !strings.Contains(body, "1000") || !strings.Contains(body, "60000") {
		t.Error("the uid range that was actually applied is not shown")
	}
}

// A source that answers with no listing at all is a failure, not an empty machine.
func TestAnAccountSourceThatAnswersNothingIsRefused(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	s.Accounts = func() (*users.Listing, error) { return nil, nil }
	rec := authGet(t, s, "/users")
	if rec.Code == http.StatusOK {
		t.Fatal("a nil listing rendered as \"no accounts\", which is a claim about the machine that " +
			"a source returning nothing did not establish")
	}
}
