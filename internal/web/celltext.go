package web

import "strings"

// sentenceTerminated returns text with a '.' on the end unless it already ends in a
// sentence terminator.
//
// The Detail cell can hold two sentences from two different sources: the verdict's own
// detail, and the self-session note the page adds underneath it. They are separate spans
// and the only thing between them is a space, so when the first does not end in
// punctuation the browser renders
//
//	...accepted the request but cannot confirm it This is the session the daemon runs inside.
//
// -- two sentences joined by nothing but a capital letter (#108). A CSS margin would not
// help: the text still copies out as one run-on, and this is the cell an operator pastes
// into a bug report as evidence.
//
// The terminator is added here rather than by each backend because the page cannot know
// what a backend will produce. Every detail this daemon emits today ends bare -- lockop's
// notLockable ends in ")" and its error details end in a clause, "...cannot confirm it" --
// but a backend that later ends a sentence with a period must not gain a second one, so
// the decision is made from the text rather than from a list of known shapes. That is also
// why it is not done in the template: `{{if .Detail}}. {{end}}` would print ".." the first
// time a detail arrives already terminated, and the template cannot see the text it is
// separating.
//
// Only . ! ? count as terminators, because that is exactly the contract the page's test
// asserts. A detail ending in ":" therefore gains a period as well; that reads oddly and
// says nothing false, which is the cheaper failure than a cell whose seam no test can
// check.
func sentenceTerminated(text string) string {
	trimmed := strings.TrimRight(text, " \t\r\n")
	if trimmed == "" {
		return text
	}
	runes := []rune(trimmed)
	switch runes[len(runes)-1] {
	case '.', '!', '?':
		return text
	}
	return trimmed + "."
}
