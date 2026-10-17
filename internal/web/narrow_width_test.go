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

import (
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

var (
	tdTagRe     = regexp.MustCompile(`(?s)<td\b[^>]*>`)
	colHeadRe   = regexp.MustCompile(`(?s)<th scope="col"[^>]*>\s*([^<]*?)\s*</th>`)
	dataLabelRe = regexp.MustCompile(`data-label="([^"]*)"`)
)

// Every cell that reports one field of a record has to name that field.
//
// A cell that spans the table is excluded on purpose: it is a sentence about the table
// ("No accounts are managed yet"), not a value under a column, and giving it a column
// label would be a lie.
func TestEveryCellInARowsTableCarriesItsNarrowLabel(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)

	for _, path := range []string{"/", "/users", "/profiles"} {
		rec := authGet(t, s, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %s", path, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, `<table class="rows"`) {
			t.Fatalf("%s: no rows table on the page, so the loop below would prove nothing", path)
		}

		heads := map[string]bool{}
		for _, m := range colHeadRe.FindAllStringSubmatch(body, -1) {
			heads[m[1]] = true
		}
		if len(heads) == 0 {
			t.Fatalf("%s: no column headers found, so the label check below has nothing to match against", path)
		}

		for _, tag := range tdTagRe.FindAllString(body, -1) {
			if strings.Contains(tag, "colspan") {
				continue
			}
			label := dataLabelRe.FindStringSubmatch(tag)
			if label == nil {
				t.Errorf("%s: a cell carries no data-label, so at phone width it renders as an "+
					"unlabelled paragraph: %s", path, tag)
				continue
			}
			// The label has to be the column's own name. A cell labelled with something
			// else is worse than no label: the desktop reading and the phone reading of
			// the same value disagree, and only one of them is on screen at a time.
			if !heads[label[1]] {
				t.Errorf("%s: data-label %q is not one of the page's column headers %v: %s",
					path, label[1], keysOf(heads), tag)
			}
		}
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

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
