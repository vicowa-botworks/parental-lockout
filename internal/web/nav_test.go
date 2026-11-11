package web

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
)

// #204's half of #199: the navigation did not say which page you were on.
//
// The report measured every link on all five photographed pages and found the same computed
// style on each of them, the link for the page you are standing on included -- `rgb(0,0,238)`,
// weight 400, underlined, no border, transparent background. The cause was two absences, one per
// layer: `app.css` had no selector for `nav`, `nav a` or `a` at all, and `page.tmpl` emitted the
// links with no attribute for such a rule to hook onto.
//
// These guards are about the attribute, which is the half that can be settled in Go: it is
// rendered by the server, so the DOM is fully determined by the bytes the handler wrote and no
// browser is needed to ask what is in it. What the attribute is *worth* -- that the marked link
// actually looks different from its siblings, and that none of them is still the browser's own
// link blue -- is a computed-style question, and that half is refused by the capture driver on all
// nine pages at both viewports: `cdp.require_nav_marked`, `cdp.require_nav_in_palette` and
// `cdp.require_nav_distinguishable`. Splitting it this way
// is the same division D-0013 made for the #202 controls: the grep pins presence, the capture
// measures effect, and neither one is asked to answer the other's question.

// navEntry is one link in the shell's navigation, as the rendered bytes state it.
type navEntry struct {
	href   string
	marked bool
}

// navEntries reads the shell's <nav> block out of a rendered page.
//
// The element is matched as exactly "<nav>" because that is what the shell emits and what
// TestTheMastheadLinksAreSeparatedInTheRenderedBytes already depends on. Adding an attribute to
// the element would make both guards report "no nav block" -- which is the honest failure, not a
// silent one: a guard that quietly found nothing here would be a guard that passes on a page with
// no navigation at all.
func navEntries(t *testing.T, body string) []navEntry {
	t.Helper()
	nav, ok := between(body, "<nav>", "</nav>")
	if !ok {
		t.Fatalf("the page renders no <nav> block, so the navigation cannot be asked anything")
	}
	var out []navEntry
	for _, after := range strings.Split(nav, "<a ")[1:] {
		open, _, _ := strings.Cut(after, ">")
		// The space that separated the tag name from its first attribute went into the split, so
		// it goes back on: without it `href` is not preceded by a space and the first attribute
		// of every anchor reads as absent. Measured -- the loop then found an href on exactly one
		// entry, the one that had `aria-current` in front of it.
		out = append(out, navEntry{
			href:   attribute(" "+open, "href"),
			marked: strings.Contains(open, `aria-current="page"`),
		})
	}
	if len(out) == 0 {
		t.Fatal("the <nav> block holds no links; a navigation with no entries marks nothing and " +
			"is not evidence that marking works")
	}
	return out
}

// attribute reads one attribute out of an opening tag's text. The leading space is part of the
// search so that `data-href` cannot answer for `href`.
func attribute(open, name string) string {
	i := strings.Index(open, " "+name+`="`)
	if i < 0 {
		return ""
	}
	rest := open[i+len(name)+3:]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// markedHrefs is the answer to "which entries does the shell think are current", for a shell
// built around one path and a fixed list of entries.
func markedHrefs(path string, entries []string) []string {
	d := pageData{Path: path}
	var out []string
	for _, e := range entries {
		if d.IsCurrent(e) {
			out = append(out, e)
		}
	}
	return out
}

// The navigation entries the shell actually emits, read from the shell rather than restated.
//
// This is not a list of what to remember: it is the same five strings the template writes, and
// the guard below asserts the rendered page carries exactly these -- so a sixth entry added to
// page.tmpl without being added here fails the equality, and an entry renamed in one place and
// not the other fails it too. A guard that instead read the entries out of the page and checked
// each against itself could only ever agree with the page, which is the failure
// route_coverage_test.go names in its own header.
//
// The promise was written before the guard kept it. What the guard compared until #229's review
// measured it was `shellNav ⊆ seen`, never the count, so the sixth entry this comment says fails
// left the whole package green -- `/login` "Help" is a live GET route, so even the route guard
// approves it. The equality is asserted on both sides and on the count now; the sentence above is
// the claim, and that arm is the check.
var shellNav = []string{"/", "/users", "/audit", "/profiles", "/settings"}

// Exactly one entry is marked, it is the entry for the page that rendered, and that is true on
// every page the shell renders.
func TestTheNavigationMarksTheEntryForThePageItIsOn(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	// The fourth seam, for the same reason route_coverage_test.go adds it: without a trail
	// /audit answers 503, and a page that never rendered is a page this guard cannot ask what
	// its navigation says.
	s.Trail = &fakeTrail{tail: audit.Tail{Records: trailFixture(), FilesRead: 1}}

	for _, want := range shellNav {
		body := authGet(t, s, want).Body.String()
		entries := navEntries(t, body)

		var marked []string
		seen := map[string]bool{}
		for _, e := range entries {
			seen[e.href] = true
			if e.marked {
				marked = append(marked, e.href)
			}
		}
		if len(marked) != 1 {
			t.Errorf("%s: %d navigation entries carry aria-current=\"page\", want exactly 1 (%v)",
				want, len(marked), marked)
			continue
		}
		if marked[0] != want {
			t.Errorf("%s: the marked entry is %q, want %q", want, marked[0], want)
		}
		// The whole row has to be there for the one mark to mean "this one, not the others", and
		// NOTHING else may be there. A nav that rendered a single link passes both assertions above,
		// and so does a sixth entry: measured for #229's review, `<a href="/login">Help</a>` added to
		// the row is a live GET route (so TestEveryNavigationEntryIsALiveGETRoute approves it), the
		// one mark is still on the right href, and the whole package stayed green. A row that grows in
		// one place while its guards keep approving the old shape is the failure this file's own
		// header names, so the comparison below is an equality -- which is what shellNav promises.
		var missing, extra []string
		for _, href := range shellNav {
			if !seen[href] {
				missing = append(missing, href)
			}
		}
		for _, e := range entries {
			if !shellOffers(e.href) {
				extra = append(extra, e.href)
			}
		}
		if len(missing) > 0 || len(extra) > 0 || len(entries) != len(shellNav) {
			t.Errorf("%s: the navigation is not the row the shell declares -- %d entries rendered "+
				"against %d declared, missing %v, not in the declaration %v. Marking one entry is the "+
				"claim \"this one, not the others\" only while the row is the one these guards were "+
				"written against", want, len(entries), len(shellNav), missing, extra)
		}
	}
}

// shellOffers is the declaration read as a question: is this href one of the entries the shell is
// written to emit? Read off `shellNav` rather than a second map so the two cannot drift.
func shellOffers(href string) bool {
	for _, e := range shellNav {
		if e == href {
			return true
		}
	}
	return false
}

// The rule behind the attribute, including the two shapes a rendered page cannot produce.
//
// The rendered loop above can only ask about pages that exist. A path nobody registered, and a
// shell that was never told a path at all, are the two cases where the rule has to refuse, and
// neither is reachable through a handler -- which is why they are here rather than in the test
// above.
func TestTheCurrentEntryIsTheSectionThePathIsIn(t *testing.T) {
	for _, tc := range []struct {
		path string
		want []string
	}{
		{"/", []string{"/"}},
		{"/users", []string{"/users"}},
		// A trailing slash is not a second page, and it must not become a page with no entry.
		{"/users/", []string{"/users"}},
		// The two POST result pages are photographed artifacts at a path the navigation has no
		// entry for. They are inside a section, and the section is what the row marks: on
		// /users/casey/lock-now the accounts entry is current. cdp.require_nav_marked asserts the
		// same thing on the DOM of lock.png and testlock.png.
		{"/users/casey/lock-now", []string{"/users"}},
		{"/users/casey/test-lock", []string{"/users"}},
		{"/audit", []string{"/audit"}},
		{"/settings/tls", []string{"/settings"}},
		// A path that merely starts with a section's name is not in that section. This is why the
		// rule splits the path instead of testing a prefix: a prefix test would also make "/" a
		// prefix of every path and mark two entries at once.
		{"/usersfoo", nil},
		{"/nope", nil},
		// No path is not the root. Marking the site's own home entry because the shell was never
		// told where it is would be readAccounts' nil listing rendering "no accounts" in the
		// header instead of the body.
		{"", nil},
	} {
		got := markedHrefs(tc.path, shellNav)
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%q marks %v, want %v", tc.path, got, tc.want)
		}
	}
}

// Every entry in the navigation goes somewhere the server actually answers.
//
// The nav is five hand-written hrefs in page.tmpl, and the router is a table in server.go that
// this package can already enumerate out of its own source. Without this, the way an entry goes
// wrong is a rename on one side: the page still renders, the link still looks marked, and the
// reader gets a 405 or a redirect on a click they had no reason to expect.
func TestEveryNavigationEntryIsALiveGETRoute(t *testing.T) {
	body := authGet(t, pageServer(t, pageConfig(), nil, nil, nil, nil), "/").Body.String()
	served := registeredGETPaths(t)
	for _, e := range navEntries(t, body) {
		if e.href == "" {
			t.Error("a navigation entry has no href, so it is not a link")
			continue
		}
		if strings.HasPrefix(e.href, "http://") || strings.HasPrefix(e.href, "https://") {
			t.Errorf("navigation entry %q is an absolute URL; the shell links within the site only",
				e.href)
			continue
		}
		if !served[e.href] {
			t.Errorf("the navigation links to %s and no GET route answers that exact path", e.href)
		}
	}
}

// registeredGETPaths is every literal path the router answers to a GET.
//
// A pattern with a wildcard in it ("GET /static/{file}") is not an answer to "does this exact
// path serve", so it is left out rather than normalised into a guess. The root is the one
// wildcard that is: "GET /{$}" is the router's spelling of "/" and nothing else.
func registeredGETPaths(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, d := range registeredRoutes(t) {
		if methodOf(d.pattern) != "GET" {
			continue
		}
		_, pat, _ := strings.Cut(d.pattern, " ")
		if strings.ContainsRune(pat, '{') && pat != "/{$}" {
			continue
		}
		// The root is registered as "GET /{$}", whose path is "/" once the "{$}" terminal is
		// cut. Cutting the whole "/{$}" instead leaves "" and the root never answers.
		out[strings.TrimSuffix(pat, "{$}")] = true
	}
	if len(out) == 0 {
		t.Fatal("the route scan found no GET routes, so the guard above cannot fail and is " +
			"not a guard -- see route_coverage_test.go on why that matters")
	}
	return out
}

// The header offers the root exactly once.
//
// #204 found two anchors with `href="/"` in the header of the page the root renders -- the site
// name and the entry for the page -- differing only in weight, and wrote: "One of them is not a
// navigation target, and nothing says which." The fix was to stop the site name being a route;
// this is the guard that keeps it from becoming one again, because the way it comes back is a
// well-meaning `<h1><a href="/">` that looks like a wordmark convention and quietly doubles the
// tab order on one page out of five.
//
// Asserted on the header's bytes rather than on the navigation's: the duplicate lived outside the
// <nav> block, so a guard that only reads the entries cannot see it.
//
// All five shell pages, including `/audit`. It was the one the list skipped, which reads as
// deliberate even though it is not -- the page 503s without an audit trail, and a page that never
// rendered has a header this guard never asks about. So the seam the navigation guard above already
// carries is set here too, and the fifth page costs one line rather than a question in every review
// after this one.
func TestTheHeaderRoutesToTheRootExactlyOnce(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	// Without a trail /audit answers 503 and renders no header at all, which the arm below refuses
	// on rather than skipping -- the same fourth seam route_coverage_test.go adds for the same reason.
	s.Trail = &fakeTrail{tail: audit.Tail{Records: trailFixture(), FilesRead: 1}}
	for _, path := range []string{"/", "/users", "/audit", "/profiles", "/settings"} {
		body := authGet(t, s, path).Body.String()
		header, ok := between(body, "<header", "</header>")
		if !ok {
			t.Fatalf("%s renders no <header>, so this guard is checking nothing", path)
		}
		if n := strings.Count(header, `href="/"`); n != 1 {
			t.Errorf("%s: the header has %d links to the root, want 1 -- the navigation's own. "+
				"A second one is a second tab stop on the way to a page the row already names: %s",
				path, n, strings.Join(strings.Fields(header), " "))
		}
		// The site's name is still in the header, and it is still not a route. What element holds
		// it changed after this guard was written: #219 (D-0016) moved the <h1> to the page's own
		// name and demoted the product's to a labelled line, which is
		// TestEveryPageNamesItselfInTheHeading's business and not this one. So this asks where the
		// words are rather than what element they sit in: once, in the header, and not inside the
		// anchor above.
		if n := strings.Count(header, "parental lockout"); n != 1 {
			t.Errorf("%s: the header says the site's name %d times, want 1 -- the header still has "+
				"to say what this software is, and it has one line to do it in: %s", path, n,
				strings.Join(strings.Fields(header), " "))
		}
		// And the surviving route to the root is the navigation's own entry, not a wordmark that
		// picked its href back up: the anchor has to close before the product's name begins.
		at := strings.Index(header, `href="/"`)
		if at < 0 {
			t.Fatalf("%s: the header has no link to the root at all, so the count above proves "+
				"nothing: %s", path, strings.Join(strings.Fields(header), " "))
		}
		end := strings.Index(header[at:], "</a>")
		if end < 0 {
			t.Fatalf("%s: the link to the root is never closed, so this guard cannot say what is "+
				"inside it: %s", path, strings.Join(strings.Fields(header), " "))
		}
		if strings.Contains(header[at:at+end], "parental lockout") {
			t.Errorf("%s: the header's only link to the root carries the site's name, so the "+
				"wordmark is a second tab stop on the way to a page the row already names: %s", path,
				strings.Join(strings.Fields(header), " "))
		}
	}
}

// The stylesheet reaches the marked entry, and reaches it last.
//
// A grep pins presence, not effect -- the same caveat the #202 pins in narrow_width_test.go carry,
// and stated here because the strings below would survive being moved into a selector that never
// matches, or losing a semicolon. The effect is measured in the browser: `cdp.require_nav_marked`,
// `cdp.require_nav_in_palette` and `cdp.require_nav_distinguishable` ask Chrome, on every
// photographed page at both viewports, whether the marked link differs from every sibling on at
// least one of the properties a reader can see and whether any link is still the browser's own
// blue.
//
// The ordering assertion is the one a reformat would break. `nav a:hover` has to come BEFORE the
// current-page rule: both name `text-decoration`, they have the same specificity once the
// attribute selector is counted, and the later rule wins -- which is why hovering the page you are
// on does not underline it while hovering a page you are not on still says it is a link. Someone
// who moves the block into alphabetical order, or lets a formatter sort it, quietly puts the
// underline back on the entry that is supposed to read as a location.
//
// Which is why each needle below carries its opening brace, and why each is counted before an offset
// is taken on it. See the comment in the body for what the two needles used to be and what that cost.
func TestTheStylesheetMarksTheCurrentEntryLast(t *testing.T) {
	css, err := fs.ReadFile(Assets(), "app.css")
	if err != nil {
		t.Fatalf("app.css is not embedded: %v", err)
	}
	text := string(css)
	// Each needle carries the opening brace, and each is counted before any offset is trusted.
	//
	// The brace is what makes a needle name a RULE rather than a mention of one: until #229's review
	// measured it, these two needles were `"nav a:hover"` and `` `nav a[aria-current="page"]` ``, each
	// of which occurs twice in this file -- once in the sentence above the block that names it (at 233
	// and 236) and once as the rule (at 263 and 282). `Index` answers with the first, so both offsets
	// the guard compared were positions in prose, ordered as the prose was written and not as the file
	// was: deleting either rule left its mention behind, moving the current-page rule above the hover
	// rule left the comment's order intact, and all three arms measured green against all three
	// mutations. (`rowAt` was already brace-anchored, and that asymmetry was the tell.)
	//
	// The count is the general half of the lesson, and it is asserted for all three needles: a brace is
	// only the reason THIS file's prose happens to be distinguishable from its rules, and a comment
	// that ever spells a selector out with its brace would mute the ordering guard again in exactly the
	// way the prose did. An offset is only evidence when the thing it points at is unique.
	for _, want := range []struct {
		what   string
		needle string
	}{
		{"the `nav a` rule that takes the row out of the user-agent stylesheet", "nav a {"},
		{"the `nav a:hover` rule", "nav a:hover {"},
		{"the rule for the current page's entry", `nav a[aria-current="page"] {`},
	} {
		switch c := strings.Count(text, want.needle); c {
		case 0:
			t.Errorf("app.css has no %s: #204 measured every link at the browser's own "+
				"rgb(0, 0, 238) with an underline, on all five photographed pages, because the "+
				"file had no selector for `nav`, `nav a` or `a` at all", want.what)
		case 1:
			// The only count an `Index` below can be trusted on.
		default:
			t.Errorf("app.css names %s %d times, want exactly 1: the ordering comparison reads the "+
				"byte offset of the FIRST mention, so with two of them it compares two mentions and "+
				"never reads the rules -- which is how this guard passed on a stylesheet whose "+
				"current-page rule was deleted, whose hover rule was deleted, and whose two rules "+
				"were swapped (#229). Either name the selector in prose without its brace, or teach "+
				"the guard to compare every occurrence: %q", want.what, c, want.needle)
		}
	}
	// Only the two rules the cascade orders are indexed. `nav a` is the least specific of the three
	// -- two elements, against an attribute or pseudo-class on each of the others -- so its position
	// in the file decides nothing, and an offset nobody compares is an offset nobody should trust.
	hoverAt := strings.Index(text, "nav a:hover {")
	currentAt := strings.Index(text, `nav a[aria-current="page"] {`)
	if hoverAt >= 0 && currentAt >= 0 && hoverAt > currentAt {
		t.Errorf("app.css puts `nav a:hover` at byte %d, after the current-page rule at %d: the "+
			"two rules both name text-decoration at the same specificity, so the hover rule now "+
			"wins and hovering the page you are on underlines it like any destination",
			hoverAt, currentAt)
	}
}
