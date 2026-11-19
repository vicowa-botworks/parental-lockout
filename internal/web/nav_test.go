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

// TestTheShellCarriesASkipLinkFirstAndLandingOnThisPage is the other half of #204's delivery:
// the skip link.
//
// The masthead sits above every page's content: five navigation links and a sign-out button,
// all focusable. With nothing past them, a keyboard reader walked the entire chrome of the
// site on Tab to reach the page they were already on -- the bullet #204 listed as
// undelivered, which the template's own comment apologised for until #387 deleted the
// apology together with the defect.
//
// #387 names the false pass this guard exists to refuse: `href="#main"` pointing at a
// <main> with no id reads as a skip link to every grep that asks "does the anchor exist"
// and goes nowhere in a browser. So the contract is stated as RESOLUTION, never as
// spelling -- the id is read off the element and the fragment off the link, and the two
// are compared, so neither can be satisfied by coincidence of a literal:
//
//   - the content region exists, exactly once, and carries an id -- the shell emits one
//     skip target per page and there is exactly one thing to skip to;
//   - exactly one link on the page targets that id -- a second one is a second Tab stop
//     on the way to the content, the thing the first one exists to save;
//   - that link precedes <nav> in the rendered bytes, and no focusable element precedes
//     it -- "first in the tab order" is document order for sequential focus navigation,
//     so a link after the masthead is the defect back with the spelling right;
//   - the link's own text is not empty -- an invisible link with no words is announced
//     as nothing and skips nothing for anybody.
//
// The login page stays out of the loop on the same reading as
// TestTheShellCarriesNavigationThatMarksThePageYouAreOn: the login document is
// loginPageTmpl in handlers.go, its own inline template with no {{template "page"}} render,
// and there is no <nav> block in its bytes for any of this to attach to. What the capture
// side measures -- that Chrome
// focuses the link at all, which `display: none` would silently prevent -- stays the
// driver's half, and this is the cheap half that runs on the rendered bytes, per page, on
// every `go test`.
func TestTheShellCarriesASkipLinkFirstAndLandingOnThisPage(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	// The same seam the navigation guard carries: without a trail the /audit page answers
	// 503 and renders no document to ask about.
	s.Trail = &fakeTrail{tail: audit.Tail{Records: trailFixture(), FilesRead: 1}}

	for _, path := range []string{"/", "/users", "/audit", "/profiles", "/settings"} {
		body := authGet(t, s, path).Body.String()

		// The target: the content region, its id, and their singleness. Reading the id
		// off the element rather than matching a literal is what keeps this about the
		// contract: an id renamed on both ends in the same edit still passes, and an
		// `<a href="#main">` aimed at a bare <main> still does not.
		if n := strings.Count(body, "<main"); n != 1 {
			t.Errorf("%s: the page carries %d <main> element(s); the shell renders the content "+
				"region exactly once, and #387's skip link targets it (#387)", path, n)
			continue
		}
		mainTag := body[strings.Index(body, "<main"):]
		mainTag, _, _ = strings.Cut(mainTag, ">")
		target := attribute(mainTag, "id")
		if target == "" {
			t.Errorf("%s: <main> carries no id, so no fragment can land on it: a skip link here "+
				"is decoration -- #387's named false pass is an href nothing resolves, and it "+
				"reads fine to any grep that only asks whether the anchor exists (#387)", path)
			continue
		}
		// The id must be UNIQUE, not merely present: getElementById answers with the
		// FIRST element bearing it, so a duplicate `id` standing earlier in the document
		// shadows the content region -- Tab-and-Enter lands on a box that still contains
		// the masthead, and every property the arms above check is individually fine.
		// The capture leg sees the same document from the browser as ttag != MAIN.
		if n := strings.Count(body, `id="`+target+`"`); n != 1 {
			t.Errorf("%s: %d elements carry id %q in the rendered bytes, so the fragment lands "+
				"on the earliest one, not necessarily the content region -- a duplicate id is "+
				"#387's false pass with a shadow standing in front (#387)", path, n, target)
		}

		// The anchors, scanned left to right with their real byte offsets — the way
		// navEntries reads the nav (opening tags and href attributes, no regex), plus
		// the one thing navEntries does not need: position. Position is the whole
		// question here, and an offset that is not an offset is how this guard nearly
		// shipped wrong: a tail-derived "offset" that equals len(body) minus the
		// REMAINDER reads low for the first anchor and high for the last, so the
		// first anchor in the bytes was reported at byte 3218 of a 3385-byte page.
		// A guard asserting "first" must know where first IS.
		var targeting, dangling []string
		skipAt, skipText := -1, ""
		for _, a := range anchors(body) {
			href := attribute(" "+a.open, "href")
			switch {
			case href == "#"+target:
				targeting = append(targeting, href)
				text, _, _ := strings.Cut(a.inner, "</a>")
				skipText = strings.TrimSpace(text)
				if skipAt == -1 {
					skipAt = a.at
				}
			case strings.HasPrefix(href, "#"):
				dangling = append(dangling, href)
			}
		}
		if len(targeting) != 1 {
			t.Errorf("%s: %d link(s) target the content region's id %q (%v), and the page carries "+
				"%d other in-page fragment(s) (%v): the shell emits exactly one skip link, which is "+
				"a first Tab stop, not a second one to walk past (#387)",
				path, len(targeting), target, targeting, len(dangling), dangling)
			continue
		}
		if skipText == "" {
			t.Errorf("%s: the skip link carries no text, so it is announced as nothing and nobody "+
				"is told what it skips (#387)", path)
		}
		if navIdx := strings.Index(body, "<nav"); navIdx < 0 {
			t.Errorf("%s: the page renders no <nav> block; the navigation guard above owns that "+
				"failure, and without it there is no masthead to establish that the skip link "+
				"comes first", path)
		} else if skipAt > navIdx {
			t.Errorf("%s: the skip link sits at byte %d and the navigation opens at byte %d, so the "+
				"link comes AFTER the masthead in document order -- and document order IS the tab "+
				"order here, which is the defect back with the spelling right (#387)",
				path, skipAt, navIdx)
		}

		// Nothing focusable before it, measured with the same left-to-right care.
		// <input type="hidden"> is skipped because it is not focusable, and that is
		// not a detail: the masthead's own sign-out form carries one, so a scan that
		// counted hidden inputs would report the skip link second on every page --
		// and a scan that instead took the FIRST and LAST byte of each token with
		// strings.Index would call the last form on the page the first element in
		// it. Both misreads were made writing this guard and are named here so the
		// next reader does not make a third.
		if first, what := firstFocusable(body); first != skipAt {
			t.Errorf("%s: something focusable (%s) sits at byte %d while the skip link is at "+
				"byte %d, so the skip link is not the first Tab stop on the page (#387)",
				path, what, first, skipAt)
		}
	}
}

// anchor is one opening <a> tag in a rendered page, with the byte offset the ordering
// assertions are made of.
// nextAnchorStart finds the next opening anchor tag at or after i: the two bytes `<a`
// followed by ASCII whitespace or `>`. The old literal "<a " required the space and the
// browser does not: `<a\nhref="#main">` is a legal anchor, a real tab stop, and exactly
// the element this file's guards are about -- a scanner that misses it lets the cheapest
// byte in HTML hide a focusable element from a byte-level tab-order assertion. `<abbr`
// is a different word and `</a` is the same word backwards; neither may match.
func nextAnchorStart(body string, i int) int {
	for ; i >= 0 && i < len(body); i++ {
		if body[i] != '<' || i+2 >= len(body) || body[i+1] != 'a' {
			continue
		}
		switch body[i+2] {
		case ' ', '\t', '\n', '\r', '>':
			return i
		}
	}
	return -1
}

type anchor struct {
	at    int    // byte offset of the "<a" itself
	open  string // the tag between "<a" and its closing ">", quote-aware
	inner string // the bytes after the opening tag: the anchor's text, among other things
}

// anchors scans every opening <a> tag out of a rendered page, left to right.
//
// A quote-aware scanner, not a Cut: an attribute value may hold a literal ">" (Go's escaper
// writes "&gt;", but a value may also hold a bare one inside quotes and parsers must
// tolerate it), and the tag ends at the first ">" OUTSIDE quotes. The inner text runs to
// the first "</a" after the tag, or to the next opening anchor -- whichever comes first --
// because this shell emits no nested anchors and an unclosed one is a defect the caller's
// counts will surface, not something the scanner should silently swallow.
func anchors(body string) []anchor {
	var out []anchor
	for i := 0; ; {
		start := nextAnchorStart(body, i)
		if start < 0 {
			break
		}
		tag := body[start+2:]
		var quote byte
		end := -1
		for k := 0; k < len(tag); k++ {
			switch c := tag[k]; {
			case quote != 0:
				if c == quote {
					quote = 0
				}
			case c == '"' || c == '\'':
				quote = c
			case c == '>':
				end = k
			}
			if end >= 0 {
				break
			}
		}
		if end < 0 {
			// An opening tag never closed: report what was scanned and stop, so the
			// caller sees a malformed page as missing links, not as a hang.
			break
		}
		inner := tag[end+1:]
		if cut := strings.Index(inner, "</a"); cut >= 0 && cut <= len(inner) {
			inner = inner[:cut]
		}
		if next := nextAnchorStart(inner, 0); next >= 0 {
			inner = inner[:next]
		}
		out = append(out, anchor{at: start, open: tag[:end], inner: inner})
		i = start + 2 + end + 1
	}
	return out
}

// firstFocusable answers which focusable element a browser reaches first in document
// order, and its token, for the byte-level tab-order assertion. Hidden inputs are not
// focusable and the masthead sign-out form carries one; the shell's only focusable
// elements are anchors, buttons, and the form controls below.
func firstFocusable(body string) (int, string) {
	best, what := len(body), "nothing"
	if i := nextAnchorStart(body, 0); i >= 0 && i < best {
		best, what = i, "<a"
	}
	for _, tok := range []string{"<button", "<select", "<textarea"} {
		if i := strings.Index(body, tok); i >= 0 && i < best {
			best, what = i, tok
		}
	}
	for i := 0; ; {
		j := strings.Index(body[i:], "<input")
		if j < 0 {
			break
		}
		at := i + j
		tag := body[at:]
		if k := strings.Index(tag, ">"); k >= 0 {
			if !strings.Contains(tag[:k], `type="hidden"`) && at < best {
				best, what = at, "<input"
			}
		}
		i = at + len("<input")
	}
	return best, what
}

// TestTheAnchorScannerReadsEveryLegalSpelling pins the tokenizer itself, not the page:
// the template happens to write `<a ` today, which is exactly why a "<a "-only scanner
// can pass every page test while being blind to what the browser would focus. The
// browser treats `<a` followed by space, `\n`, `\t`, `\r` or `>` as one tag opener; the
// scanner must too, and it must keep saying no to `<abbr` and to `</a`.
func TestTheAnchorScannerReadsEveryLegalSpelling(t *testing.T) {
	body := `<p abbr="x">text</p>` + "\n<a\nhref=\"#a\">one</a>" +
		"<a\thref=\"#b\">two</a><a>c</a>" + "</a><abbr>not</abbr>"
	wantText := []string{"one", "two", "c"}
	got := anchors(body)
	if len(got) != len(wantText) {
		t.Fatalf("the scanner read %d anchors from a page carrying %d: at=%v",
			len(got), len(wantText), func() []int {
				var at []int
				for _, a := range got {
					at = append(at, a.at)
				}
				return at
			}())
	}
	for i, a := range got {
		text, _, _ := strings.Cut(a.inner, "</a>")
		if strings.TrimSpace(text) != wantText[i] {
			t.Errorf("anchor %d carries text %q, want %q -- the scanner mis-sliced the document",
				i, strings.TrimSpace(text), wantText[i])
		}
	}
	if i := nextAnchorStart("</a><abbr>", 0); i >= 0 {
		t.Errorf("byte %d of %q is not an anchor opening; </a> and <abbr must never match", i, "</a><abbr>")
	}
	if i := nextAnchorStart(body, 0); i != strings.Index(body, "<a\n") {
		t.Errorf("the first anchor was reported at %d, want the newline-spelled one at %d",
			i, strings.Index(body, "<a\n"))
	}
	if at, what := firstFocusable(body); at != strings.Index(body, "<a\n") || what != "<a" {
		t.Errorf("firstFocusable answered (%d,%q), want the newline-spelled anchor at %d",
			at, what, strings.Index(body, "<a\n"))
	}
}
