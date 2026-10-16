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

// trailOfLength is a newest-first trail of n records, with the two ends named so a test
// can say which end got cut. The reader returns newest first, so index 0 is the newest
// event and index n-1 is the oldest one the read returned.
func trailOfLength(n int) []audit.Record {
	out := make([]audit.Record, n)
	for i := range out {
		detail := fmt.Sprintf("event-%d", i)
		switch i {
		case 0:
			detail = "newest-record"
		case n - 1:
			detail = "oldest-record"
		}
		out[i] = audit.Record{
			At:        insideWindow.Add(-time.Duration(i) * time.Minute),
			Kind:      "lock.asserted",
			Username:  "someone-else",
			SessionID: fmt.Sprintf("%d", i),
			Detail:    detail,
		}
	}
	return out
}

// One record more than the window is the only shape that entitles the page to say older
// events may exist: the extra record is proof the read stopped with something behind it.
func TestARecordBehindTheWindowSaysTheReadStoppedEarly(t *testing.T) {
	s, _, _ := pageWithTrail(t, audit.Tail{Records: trailOfLength(auditWindow + 1), FilesRead: 2}, nil)
	rec := authGet(t, s, "/audit?user=mallory")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := flat(rec.Body.String())
	if !strings.Contains(body, "may exist beyond this window") {
		t.Errorf("a read that stopped with a record behind it was reported as if it had read "+"everything, so an operator would conclude the account was never held:\n%s", body)
	}
	if strings.Contains(body, "The trail holds nothing older than these") {
		t.Error("a truncated read claimed the trail was exhausted; the older events are " + "where the record the operator is looking for would be")
	}

	// The rows themselves, on an unfiltered page: a filtered-empty page renders no table,
	// so the cut can only be observed where rows are actually shown.
	rows := flat(authGet(t, s, "/audit").Body.String())
	// The extra record is evidence, not content: showing it would make the page one row
	// longer than it says it is.
	if strings.Contains(rows, "oldest-record") {
		t.Errorf("the probe record was rendered as an event; the page shows %d rows and no more:\n%s",
			auditWindow, rows)
	}
	if !strings.Contains(rows, "newest-record") {
		t.Error("cutting the extra record off the wrong end dropped the newest event, which is " +
			"the one row an operator reads first")
	}
	// The caption carries both numbers, and the probe must not leak into either: 200 shown
	// out of 200 searched, not 200 of 201.
	if !strings.Contains(rows, "200 of the 200 most recent events") {
		t.Errorf("the caption did not report the window it showed and the set it filtered: %q\n%s",
			"200 of the 200 most recent events", rows)
	}
}

// Exactly the window is the case the page used to guess at. The trail has been read to its
// end -- there is no record behind it -- so the page must close the search rather than
// point at a generation that does not exist.
func TestATrailOfExactlyTheWindowIsExhaustedNotTruncated(t *testing.T) {
	s, _, _ := pageWithTrail(t, audit.Tail{Records: trailOfLength(auditWindow), FilesRead: 1}, nil)
	body := flat(authGet(t, s, "/audit?user=mallory").Body.String())
	if strings.Contains(body, "may exist beyond this window") {
		t.Errorf("a trail of exactly %d records was described as cut off, sending the operator to "+"search older events that do not exist while being told the filter only saw the "+"most recent %d -- which was all there ever was:\n%s", auditWindow, auditWindow, body)
	}
	if !strings.Contains(body, "The trail holds nothing older than these") {
		t.Errorf("the page did not close the search, which is the answer that ends it:\n%s", body)
	}
	rows := flat(authGet(t, s, "/audit").Body.String())
	if !strings.Contains(rows, "oldest-record") {
		t.Errorf("a trail that fits the window exactly had a row cut from it:\n%s", rows)
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
