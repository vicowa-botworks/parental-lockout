package web

// The Lockout cell is two pieces of prose -- the state and, when the lockout ends at a
// known instant, the clock -- and the template comment between them was written with trim
// markers on both sides, which deleted the only whitespace that separated them. The result
// on the page was "locked outuntil 06:30".
//
// Nothing noticed, because every existing assertion asks whether the page contains
// "locked out" and whether it contains "06:30", and both are true of the joined text. This
// is the cell a parent reads to answer "when can they go back?", so the two halves have to
// be readable as two things.
//
// The check is a function rather than an inline assertion for the same reason the narrow
// width check is one: the dashboard fixture is a page that is currently correct, so the
// only way to show the comparison can fail is to hand it a cell that is not.

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

var (
	lockoutCellRe = regexp.MustCompile(`(?s)<td data-label="Lockout">(.*?)</td>`)
	tagRe         = regexp.MustCompile(`<[^>]*>`)
	// "locked out" with a non-space character glued to it: the join the trim markers made.
	joinedWordRe = regexp.MustCompile(`locked out[^\s]`)
)

// lockoutCellComplaint reads the dashboard's Lockout cell the way a browser does -- markup
// dropped, runs of whitespace collapsed -- and says what is wrong with it, or returns "" if
// nothing is.
func lockoutCellComplaint(body string) string {
	cell := lockoutCellRe.FindStringSubmatch(body)
	if cell == nil {
		return "the dashboard has no Lockout cell, so there is nothing to read"
	}
	text := strings.Join(strings.Fields(tagRe.ReplaceAllString(cell[1], "")), " ")
	// The rule is about the join, not about a deadline: a held account with no known end
	// renders the state alone, and that is correct. What is never correct is the state
	// running straight into whatever follows it.
	if joinedWordRe.MatchString(text) {
		return "the Lockout cell reads " + quote(text) + "; the state and whatever follows it have " +
			"to be two words apart, because the sentence is the answer to \"when can they go back?\""
	}
	return ""
}

func quote(s string) string { return `"` + s + `"` }

func TestTheLockoutCellSeparatesItsStateFromItsDeadline(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	rec := authGet(t, s, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if got := lockoutCellComplaint(rec.Body.String()); got != "" {
		t.Error(got)
	}
}

// The control that says the comparison above can fail.
//
// Written as the markup the template produced before it was fixed, so the assertion is
// answering the question "would this have caught it" and not merely "is the page fine now".
func TestTheLockoutCheckRejectsAJoinedCell(t *testing.T) {
	joined := `<td data-label="Lockout"><span class="state-held">locked out</span><time>06:30</time></td>`
	if got := lockoutCellComplaint(joined); got == "" {
		t.Error("a cell that renders \"locked outuntil 06:30\" was accepted, so the check above " +
			"is not checking the thing its name says")
	}

	// And the other direction: a cell in another state has no deadline to separate, so the
	// check must not demand one.
	other := `<td data-label="Lockout">not now; next at 21:00</td>`
	if got := lockoutCellComplaint(other); got != "" {
		t.Errorf("a cell that is not held was rejected: %s", got)
	}
}
