package web

// The pages are read as tables on a desktop and as stacked blocks on a phone, and the
// only thing that tells a stacked cell what it is holding is its data-label. Nothing in
// Go renders that differently, so an ordinary page test cannot notice it missing -- the
// failure only exists in a browser, at one width, on a column somebody adds later.
//
// These tests are the part of the 360 px acceptance criterion that can be pinned without
// a browser. The measurement itself (documentElement.scrollWidth at a 360 px viewport)
// was taken in headless Chrome and is recorded in the PR description; what is checked
// here is the markup contract that measurement depends on.
//
// The check lives in rowsTableComplaints rather than inline in the test, and the last test
// in this file runs it against a hand-built bad table. That is not decoration: a page
// fixture is by definition a page that is currently correct, so an assertion deleted from
// an inline loop would leave the suite green, and the only thing that would notice is the
// next unlabelled column.

import (
	"io/fs"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
)

var (
	tdTagRe     = regexp.MustCompile(`(?s)<td\b[^>]*>`)
	colHeadRe   = regexp.MustCompile(`(?s)<th scope="col"[^>]*>\s*([^<]*?)\s*</th>`)
	dataLabelRe = regexp.MustCompile(`data-label="([^"]*)"`)
	rowRe       = regexp.MustCompile(`(?s)<tr\b.*?</tr>`)
	// A row's own header, matched only at the start of the row's cells -- a row header
	// somewhere in the middle would be a malformed table, not a label.
	rowHeadRe = regexp.MustCompile(`(?s)^\s*<th\b[^>]*scope="row"`)
)

// notAPage records every authenticated GET route this guard deliberately does not visit,
// with the reason. It exists so that an omission has to be written down somewhere a
// reviewer sees: a route that is neither a page nor listed here fails the test.
var notAPage = map[string]string{}

// narrowPages is every page the server serves to a signed-in caller, derived from the
// route registrations rather than written out by hand.
//
// The list this replaces named three pages. The audit page -- the fourth, with a
// five-column table -- had already shipped, so the guard's own promise ("the failure only
// exists on a column somebody adds later") was false about whole pages, not just columns.
// Deriving the list is what makes adding a page an act the guard notices.
func narrowPages(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, d := range registeredRoutes(t) {
		method, pattern, _ := strings.Cut(d.pattern, " ")
		if method != "GET" {
			continue
		}
		if _, open := reachableWithoutASession[d.pattern]; open {
			continue // the login page and the health check hold no tables to label
		}
		if why, skip := notAPage[d.pattern]; skip {
			t.Logf("%s is not visited as a page: %s", d.pattern, why)
			continue
		}
		path := pattern
		if path == "/{$}" {
			path = "/"
		}
		for {
			brace := strings.IndexByte(path, '{')
			if brace < 0 {
				break
			}
			end := strings.IndexByte(path[brace:], '}')
			if end < 0 {
				t.Fatalf("%s (%s): an unbalanced path parameter, so no request can be built for it",
					d.pattern, d.where)
			}
			param := path[brace : brace+end+1]
			value, ok := pathValues[param]
			if !ok {
				// Skipping it would report the page as covered while never asking it
				// anything, which is the failure this whole file exists to refuse.
				t.Errorf("%s (%s): no fixture value for path parameter %s, so this page would be skipped",
					d.pattern, d.where, param)
				break
			}
			path = path[:brace] + value + path[brace+end+1:]
		}
		out = append(out, path)
	}
	// The floor is the guard's own control: if the scan ever reads nothing, an empty loop
	// is a green test that checked no page at all.
	if len(out) < 4 {
		t.Fatalf("the route scan yielded %d pages (%v); the server serves at least the dashboard, "+
			"users, profiles and audit, so the scan has gone blind", len(out), out)
	}
	sort.Strings(out)
	return out
}

// rowsTableComplaints reports every way one rendered page breaks the stacked-cell contract,
// and how many cells it actually examined.
//
// The count is part of the answer on purpose. "No complaints" and "nothing to complain
// about" are different facts, and a caller that cannot tell them apart reports an empty
// fixture as a pass.
//
// Position, not membership: a cell labelled with a column that exists somewhere else on the
// page passes a "is this a header?" check and still shows the operator the wrong word at
// phone width. A cell that spans the table is excluded on purpose -- it is a sentence about
// the table ("No accounts are managed yet"), not a value under a column, and giving it a
// column label would be a lie.
func rowsTableComplaints(path, body string) (complaints []string, cells int) {
	if !strings.Contains(body, `<table class="rows"`) {
		return nil, 0 // a page with no table has no cells to label
	}
	// Per table, not per page. A page may carry more than one rows table -- the backend
	// probe prints one for the backends and one for the account's sessions, and they do not
	// share a single column -- and collecting the headers page-wide pairs the second table's
	// cells against the first table's columns. That reads as a broken page when the page is
	// fine, and, in the other direction, lets a genuinely mislabelled cell in a later table
	// line up with an earlier column that happens to carry the same word.
	for _, tbl := range rowsTables(body) {
		c, n := oneTableComplaints(path, tbl)
		complaints = append(complaints, c...)
		cells += n
	}
	if cells == 0 {
		complaints = append(complaints,
			path+" renders a rows table with no ordinary cells, so nothing was checked")
	}
	return complaints, cells
}

// rowsTables cuts a rendered page into its rows tables, in document order.
func rowsTables(body string) []string {
	var out []string
	rest := body
	for {
		i := strings.Index(rest, `<table class="rows"`)
		if i < 0 {
			return out
		}
		rest = rest[i:]
		j := strings.Index(rest, "</table>")
		if j < 0 {
			return append(out, rest)
		}
		out = append(out, rest[:j])
		rest = rest[j:]
	}
}

// oneTableComplaints applies the contract to a single table's markup.
func oneTableComplaints(path, tbl string) (complaints []string, cells int) {
	// The columns in the order the page prints them, which is the order a stacked cell has
	// to be labelled against.
	var heads []string
	for _, m := range colHeadRe.FindAllStringSubmatch(tbl, -1) {
		heads = append(heads, m[1])
	}
	if len(heads) == 0 {
		return []string{path + ": a rows table with no column headers, so a label cannot be checked against anything"}, 0
	}

	for _, row := range rowRe.FindAllString(tbl, -1) {
		if strings.Contains(row, `scope="col"`) || strings.Contains(row, "colspan") {
			continue // the header row, or a sentence about an empty table
		}
		tags := tdTagRe.FindAllString(row, -1)
		if len(tags) == 0 {
			continue
		}
		// A row that opens with its own row header has already named the first column in
		// that header, so its cells line up with the columns after it.
		offset := 0
		if rowHeadRe.MatchString(row[strings.Index(row, ">")+1:]) {
			offset = 1
		}
		for i, tag := range tags {
			cells++
			if offset+i >= len(heads) {
				complaints = append(complaints,
					path+": a row carries more cells than the table has columns "+strings.Join(heads, ", ")+": "+tag)
				break
			}
			want := heads[offset+i]
			label := dataLabelRe.FindStringSubmatch(tag)
			if label == nil {
				complaints = append(complaints, path+": the "+want+
					" cell carries no data-label, so at phone width it renders as an unlabelled paragraph: "+tag)
				continue
			}
			if label[1] != want {
				complaints = append(complaints, path+": data-label "+strconv.Quote(label[1])+
					" is not this cell's own column "+strconv.Quote(want)+
					"; the desktop and phone readings of the same value then disagree, and only one of them is on screen at a time: "+tag)
			}
		}
	}
	return complaints, cells
}

// Every page the server serves to a signed-in caller, at the width a phone holds it.
func TestEveryCellInARowsTableCarriesItsNarrowLabel(t *testing.T) {
	// The trail seam is wired because the audit page renders its table only when a trail
	// answered: with the seam unwired the page is a refusal in prose, the loop below finds
	// no cells, and the page is silently absent from a test that names every page.
	s, _, _ := pageWithTrail(t, audit.Tail{Records: trailFixture(), FilesRead: 1}, nil)

	checked := 0
	pages := narrowPages(t)
	for _, path := range pages {
		rec := authGet(t, s, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %s", path, rec.Body.String())
		}
		got, cells := rowsTableComplaints(path, rec.Body.String())
		for _, c := range got {
			t.Error(c)
		}
		if cells > 0 {
			checked++
		}
	}
	// The count is what stops this test passing by checking less. A hand-written page list
	// that quietly leaves one page out, or a fixture whose rows stop rendering, both leave
	// every remaining assertion green.
	if checked < 4 {
		t.Errorf("only %d of the %v pages had a single cell checked; the dashboard, users, profiles "+
			"and audit pages all render tables, so this test has stopped measuring them", checked, pages)
	}
}

// The bad-table fixture is why the assertions above are known to be able to fail.
//
// Every real page in the suite is a page that is currently correct, so a comparison deleted
// from rowsTableComplaints cannot be caught by them. This hands the same function a table
// with each defect written out, and a correct one to prove it is not simply reporting
// everything.
func TestTheLabelCheckRejectsAHandBuiltTable(t *testing.T) {
	const good = `<table class="rows">
<thead><tr><th scope="col">When</th><th scope="col">Event</th><th scope="col">Detail</th></tr></thead>
<tbody>
<tr><th scope="row">casey</th><td data-label="Event">lock.asserted</td><td data-label="Detail">session 2</td></tr>
<tr><td colspan="3">a sentence about the table</td></tr>
</tbody></table>`

	got, cells := rowsTableComplaints("/probe", good)
	if len(got) != 0 || cells != 2 {
		t.Errorf("the correct table was rejected (%d cells, %v); a check that reports every table "+
			"is not a check", cells, got)
	}

	const bad = `<table class="rows">
<thead><tr><th scope="col">When</th><th scope="col">Event</th><th scope="col">Detail</th></tr></thead>
<tbody>
<tr><td>nothing here names the column</td><td data-label="When">a label from the wrong column</td><td data-label="Detail">fine</td><td data-label="Detail">one cell too many</td></tr>
</tbody></table>`

	got, cells = rowsTableComplaints("/probe", bad)
	if cells != 4 {
		t.Errorf("the bad table reported %d cells, want 4 -- the loop is not walking the row", cells)
	}
	for _, want := range []string{
		"carries no data-label",         // the first cell: no label at all
		"is not this cell's own column", // the second: a label copied from a neighbour
		"more cells than the table has", // the fourth: a column nobody declared
	} {
		if !anyContains(got, want) {
			t.Errorf("the bad table did not produce a complaint containing %q; it produced %v", want, got)
		}
	}

	const empty = `<table class="rows">
<thead><tr><th scope="col">When</th><th scope="col">Event</th></tr></thead>
<tbody><tr><td colspan="2">nothing to show</td></tr></tbody></table>`
	if got, cells := rowsTableComplaints("/probe", empty); cells != 0 || !anyContains(got, "no ordinary cells") {
		t.Errorf("a table with no rows was reported as %d cells and %v; an empty fixture has to read "+
			"as nothing measured, not as a pass", cells, got)
	}
}

// Two tables on one page are checked against their own columns.
//
// The guard used to collect its headers page-wide, which a single-table page cannot reveal:
// the backend probe prints one table for the backends and one for the account's sessions, and
// the second table's cells were being paired against the first table's columns. That fails a
// correct page, and in the other direction it lets a mislabelled cell in a later table pass
// because an earlier column happens to carry the same word. Both directions are asserted here,
// because a change that only makes the good page pass is indistinguishable from a change that
// stopped checking.
func TestTwoTablesOnOnePageAreCheckedAgainstTheirOwnColumns(t *testing.T) {
	const good = `<table class="rows">
<thead><tr><th scope="col">Backend</th><th scope="col">Can lock</th><th scope="col">Detail</th></tr></thead>
<tbody><tr><th scope="row">kde</th><td data-label="Can lock">yes</td><td data-label="Detail">gdbus found</td></tr></tbody>
</table>
<table class="rows">
<thead><tr><th scope="col">Session</th><th scope="col">Where</th><th scope="col">Would lock</th></tr></thead>
<tbody><tr><th scope="row">2</th><td data-label="Where">seat0 · x11</td><td data-label="Would lock">yes</td></tr></tbody>
</table>`
	if got, cells := rowsTableComplaints("/probe", good); len(got) != 0 || cells != 4 {
		t.Errorf("a correct two-table page was rejected (%d cells, %v); each table has to be "+
			"measured against its own headers", cells, got)
	}

	// The same page with one label copied from the FIRST table into the second. "Detail" is
	// a real column on this page -- just not in this table -- which is exactly the case a
	// page-wide header list cannot see.
	bad := strings.Replace(good, `<td data-label="Where">seat0 · x11</td>`,
		`<td data-label="Detail">seat0 · x11</td>`, 1)
	got, cells := rowsTableComplaints("/probe", bad)
	if cells != 4 {
		t.Errorf("the two-table page reported %d cells, want 4 -- one of the two tables was "+
			"never walked", cells)
	}
	if !anyContains(got, `data-label "Detail" is not this cell's own column "Where"`) {
		t.Errorf("a label copied from the other table was not named; the guard produced %v", got)
	}
}

// The labels are only worth having if the stylesheet actually prints them.
//
// Deleting the media block would leave every page test green: the attributes stay in the
// markup, the desktop layout is untouched, and the phone layout silently loses the text
// that makes a stacked cell readable.
func TestTheStylesheetPrintsTheNarrowLabels(t *testing.T) {
	css, err := fs.ReadFile(Assets(), "app.css")
	if err != nil {
		t.Fatalf("app.css is not embedded: %v", err)
	}
	text := string(css)
	for _, want := range []string{
		// The breakpoint is pinned to its value, not merely to the existence of a
		// media query: a number small enough to never match a phone keeps every one of
		// these strings in the file and leaves the desktop layout on a 360 px screen.
		// 40rem is 640 px, which is comfortably past the widest phone in portrait.
		"@media (max-width: 40rem)",
		`content: attr(data-label)`,
		`table.rows td[colspan]::before`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("app.css no longer contains %q -- the stacked phone layout depends on it", want)
		}
	}
}
