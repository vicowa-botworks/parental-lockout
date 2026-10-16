package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
)

// The filtered-empty banner has three honest endings, and the reviewer's probe caught the
// page giving the wrong one for the common case: a three-record trail was told it might
// have "older events beyond this window" when the window was never full and nothing was
// unreadable. The trail had simply run out, and there is no older generation to go look in.

// A short trail that was read to the end must not send the operator looking backwards.
func TestAnExhaustedTrailDoesNotPromiseOlderEvents(t *testing.T) {
	s, _, _ := pageWithTrail(t, audit.Tail{Records: trailFixture(), FilesRead: 1}, nil)
	body := flat(authGet(t, s, "/audit?user=mallory").Body.String())
	if strings.Contains(body, "may exist beyond this window") {
		t.Errorf("a trail of 3 records was described as if it had been cut off at %d:\n%s",
			auditWindow, body)
	}
	if !strings.Contains(body, "The trail holds nothing older than these") {
		t.Errorf("the page did not say the trail ends here, which is the answer that ends "+"the search rather than extending it:\n%s", body)
	}
}

// A full window is the one case that entitles the page to say older events may exist.
func TestAFullWindowSaysTheReadStoppedEarly(t *testing.T) {
	full := make([]audit.Record, auditWindow)
	for i := range full {
		full[i] = audit.Record{
			At:        insideWindow.Add(-time.Duration(i) * time.Minute),
			Kind:      "lock.asserted",
			Username:  "someone-else",
			SessionID: fmt.Sprintf("%d", i),
		}
	}
	s, _, _ := pageWithTrail(t, audit.Tail{Records: full, FilesRead: 2}, nil)
	rec := authGet(t, s, "/audit?user=mallory")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := flat(rec.Body.String())
	if !strings.Contains(body, "may exist beyond this window") {
		t.Errorf("a page holding exactly the whole window was reported as if it had read "+"everything, so an operator would conclude the account was never held:\n%s", body)
	}
	if strings.Contains(body, "The trail holds nothing older than these") {
		t.Error("a truncated read claimed the trail was exhausted; the older generations are " + "where the events the operator is looking for would be")
	}
}

// A short page caused by an unreadable generation is neither of the above: nothing older is
// in what could be read, but the trail is not exhausted either.
func TestAShortPageFromAnUnreadableGenerationClaimsNeitherEnd(t *testing.T) {
	s, _, _ := pageWithTrail(t,
		audit.Tail{Records: trailFixture(), FilesRead: 1},
		errors.New("audit: reading generation 1 (/var/log/plock/audit.jsonl.1): permission denied"))
	body := flat(authGet(t, s, "/audit?user=mallory").Body.String())
	if strings.Contains(body, "The trail holds nothing older than these") {
		t.Errorf("a trail with an unreadable generation was reported as complete:\n%s", body)
	}
	if !strings.Contains(body, "Nothing older is in what could be read") {
		t.Errorf("the page did not say its shortness came from a generation it could not open:\n%s", body)
	}
}
