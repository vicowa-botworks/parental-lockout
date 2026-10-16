package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The reason field's two rules, both of which exist because an override is the one config
// entry that REMOVES enforcement.
//
// The audit line is what settles a dispute at the kitchen table, and both of these failures
// produce a line that settles nothing: a blank one ends in a colon with no sentence after it,
// and a megabyte-long one is unreadable on the phone the parent is holding. Neither is an
// attack -- the writer is an authenticated admin -- so both are guardrails, and a guardrail
// that refuses without saying what it wants is worse than no guardrail.

func TestAWhitespaceOnlyReasonIsRefused(t *testing.T) {
	f := newOverrideFixture(t)

	// `--reason " "` from a shell and a form field someone typed a single space into are the
	// same request, and both used to store a reason that renders as nothing.
	rec := authPost(t, f.s, "/users/casey/override", "minutes=15&reason=+")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "needs a reason") {
		t.Errorf("the refusal does not say what it wants: %q", rec.Body.String())
	}
	if got := f.overrides(); len(got) != 0 {
		t.Errorf("a whitespace-only reason was stored anyway: %+v", got)
	}
}

func TestAnAllWhitespaceReasonOfAnyKindIsRefused(t *testing.T) {
	f := newOverrideFixture(t)
	// Tabs and newlines survive a textarea, which is what a long reason is typed into.
	for _, blank := range []string{" ", "   ", "\t", "\n", " \t \n "} {
		rec := authPost(t, f.s, "/users/casey/override",
			"minutes=15&reason="+url.QueryEscape(blank))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("reason %q: status = %d, want 422", blank, rec.Code)
		}
	}
	if got := f.overrides(); len(got) != 0 {
		t.Errorf("blank reasons were stored: %+v", got)
	}
}

// TestTheReasonLimitIsNamedInItsOwnRefusal is the whole point of bounding the field at all:
// an operator who is refused without being told the limit has no way to find out what to
// change, and the form gives them no hint either.
func TestTheReasonLimitIsNamedInItsOwnRefusal(t *testing.T) {
	f := newOverrideFixture(t)

	long := strings.Repeat("a", maxOverrideReason+1)
	rec := authPost(t, f.s, "/users/casey/override",
		"minutes=15&reason="+url.QueryEscape(long))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, fmt.Sprint(maxOverrideReason)) {
		t.Errorf("the refusal does not name the limit (%d), so the operator cannot act on it: %q",
			maxOverrideReason, body)
	}
	if got := f.overrides(); len(got) != 0 {
		t.Errorf("an over-long reason was stored anyway: %+v", got)
	}
}

// TestAReasonOfExactlyTheLimitIsAccepted pins the boundary in the direction that matters.
// A guard that refuses at the limit it advertises contradicts its own message, and the
// operator has no way to discover the real number.
func TestAReasonOfExactlyTheLimitIsAccepted(t *testing.T) {
	f := newOverrideFixture(t)

	rec := authPost(t, f.s, "/users/casey/override",
		"minutes=15&reason="+url.QueryEscape(strings.Repeat("a", maxOverrideReason)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("a reason of exactly %d characters = %d, want 303; body %q",
			maxOverrideReason, rec.Code, rec.Body.String())
	}
	got := f.overrides()
	if len(got) != 1 || len(got[0].Reason) != maxOverrideReason {
		t.Errorf("overrides = %+v, want one with a %d-character reason", got, maxOverrideReason)
	}
}

// TestTheStoredReasonIsTrimmed keeps the stored value and the audit line agreeing with what
// the operator wrote. Trimming only the check and not the value would store " hi " and put
// that in the trail, so the two halves have to move together.
func TestTheStoredReasonIsTrimmed(t *testing.T) {
	f := newOverrideFixture(t)

	rec := authPost(t, f.s, "/users/casey/override",
		"minutes=15&reason="+url.QueryEscape("  homework at Sam's  "))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body %q", rec.Code, rec.Body.String())
	}
	got := f.overrides()
	if len(got) != 1 {
		t.Fatalf("overrides = %+v, want one", got)
	}
	if got[0].Reason != "homework at Sam's" {
		t.Errorf("stored reason = %q, want the trimmed text", got[0].Reason)
	}
	// The audit line is the durable copy, so it carries the trimmed reason too.
	joined := strings.Join(f.audited.Lines(), "\n")
	if !strings.Contains(joined, "homework at Sam's") {
		t.Errorf("the audit trail does not carry the reason: %q", joined)
	}
	if strings.Contains(joined, "  homework") {
		t.Errorf("the audit trail carries the untrimmed reason: %q", joined)
	}
}
