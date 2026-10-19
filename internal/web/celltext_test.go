package web

import "testing"

// TestTheTerminatorIsAddedOnceAndOnlyWhenNeeded pins the half of sentenceTerminated's
// promise that neither page test can see.
//
// Both seam tests ask one question only: does the Detail cell end in a terminator? A helper
// that appends one unconditionally answers yes on every input, including the details that
// already ended in one, and the page then reads "cannot confirm it.. This is the session".
// Measured: replacing the switch with an unconditional `return trimmed + "."` left the whole
// internal/web suite green, so the promise in the helper's own comment -- that a backend
// which later ends a sentence with a period must not gain a second one -- was pinned by
// nothing.
//
// The cases are the shapes the producers actually emit today (a bare clause from
// `err.Error()`, a parenthesised reason from lockop's notLockable) plus the shapes they
// could emit tomorrow without a code change in this package.
func TestTheTerminatorIsAddedOnceAndOnlyWhenNeeded(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"a bare clause gains one",
			"lock requested but not confirmed: command: accepted the request but cannot confirm it",
			"lock requested but not confirmed: command: accepted the request but cannot confirm it."},
		{"a parenthesised reason gains one after the bracket",
			"not a lockable desktop session (logind reported no class for it)",
			"not a lockable desktop session (logind reported no class for it)."},
		{"one that already ended gains nothing",
			"the screen went down.",
			"the screen went down."},
		{"a bang is a terminator too", "the backend said no!", "the backend said no!"},
		{"a question is a terminator too", "did the backend answer?", "did the backend answer?"},
		{"a terminator with trailing whitespace is left alone",
			"the screen went down. ",
			"the screen went down. "},
		{"a colon is not a terminator, so it gains a period",
			"two backends were tried:",
			"two backends were tried:."},
		{"an ellipsis is not one either, and is not ASCII",
			"the trail stops here…",
			"the trail stops here…."},
		{"empty stays empty", "", ""},
		{"whitespace only stays as it was", "   ", "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sentenceTerminated(tc.in); got != tc.want {
				t.Errorf("sentenceTerminated(%q) = %q, want %q", tc.in, got, tc.want)
			}
			// Applying it twice must not add a second period: the page calls it once per
			// render, but a helper that is not idempotent is one refactor away from "..".
			if twice := sentenceTerminated(sentenceTerminated(tc.in)); twice != tc.want {
				t.Errorf("applying the terminator twice gave %q, want %q", twice, tc.want)
			}
		})
	}
}
