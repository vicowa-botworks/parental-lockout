package web

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The state vocabulary has three homes, and this test is the seam between two of them.
//
// A page reports a state by putting a class on a word (`internal/web/templates/*.tmpl`); the
// stylesheet turns that class into something a reader can see (`internal/web/static/app.css`); and
// `testing/capture/cdp.py`'s `STATE_CLASSES` is the list the browser is actually asked about, on
// every photographed page, by `require_state_legible`. #199 is what happens when the three drift:
// the markup said `state-unknown` and `state-gap` and the stylesheet had never heard of either, so
// "no desktop session", "session list unavailable" and "scheduled, not enforced" were ordinary prose
// next to a red bold "locked out" -- and nothing in the tree noticed, because the only file that
// knew the vocabulary was the one being edited.
//
// What lives here is the static half of the seam: the two names a rename moves. The effect is not
// checked here, and cannot be -- a grep of app.css can read that a class is *mentioned* and cannot
// read that anything is *drawn*, which is the exact difference #229's review made about the
// navigation pins one comment later in the same file. The effect is measured in Chrome, at both
// viewports, by the capture driver.
//
// The scope of this test is the `state-` prefix, which is the part of the vocabulary the pages spell
// literally. `banner`, `flag`, `problem`, `override` and `disabled` are the other half; their seam is
// `every_state_class_the_pages_render_is_in_the_census` in `testing/capture/pages.py`, which reads
// the same templates from the other side, and the two composed cells (`state-held`, `state-unknown`,
// written as `class="state-{{if …}}held{{else}}unknown{{end}}"`) are visible to neither reader as a
// name -- the browser is the only one that ever sees them resolved.
func TestEveryStateClassTheTemplatesNameIsDrawn(t *testing.T) {
	names, err := fs.Glob(pageFS, "templates/*.tmpl")
	if err != nil {
		t.Fatalf("the embedded templates cannot be listed: %v", err)
	}
	if len(names) < 5 {
		t.Fatalf("only %d templates are embedded; the vocabulary census below is only an answer "+
			"about the pages if all of them are here", len(names))
	}

	// Every `state-<name>` written out in a template. The composed cells do not match this pattern
	// and are reported above rather than silently absent.
	emitted := map[string]string{} // class name -> the template that spells it
	stateName := regexp.MustCompile(`state-[a-z][a-z-]*`)
	for _, name := range names {
		b, err := fs.ReadFile(pageFS, name)
		if err != nil {
			t.Fatalf("the embedded template %q cannot be read: %v", name, err)
		}
		for _, m := range stateName.FindAllString(string(b), -1) {
			emitted[m] = name
		}
	}
	// The census has to be about something. A template that lost its only `state-` word would leave
	// this test asserting nothing, which is the failure mode this folder names in every guard it
	// writes.
	if len(emitted) < 2 {
		t.Fatalf("the templates name %d state classes (%v); the pages report at least a held and a "+
			"not-managed state, so the census found the wrong files", len(emitted), emitted)
	}

	css, err := fs.ReadFile(Assets(), "app.css")
	if err != nil {
		t.Fatalf("app.css is not embedded: %v", err)
	}
	rules := stylesheetRules(string(css))

	for class, where := range emitted {
		if !rules[class] {
			t.Errorf("app.css has no rule for %q, which %s puts on a word: this is the #199 defect "+
				"-- the class is in the markup and nothing turns it into something a reader can see, "+
				"so the state it names is drawn in exactly the voice of the prose around it. Add the "+
				"rule, or stop naming the state.", class, where)
		}
	}

	// The other direction: the stylesheet may not invent a state word the pages never say. A rule
	// for a class nothing renders is not harmless -- it is the reason a vocabulary census goes
	// stale, and a stale census is how the next #199 hides.
	for class := range rules {
		if !strings.HasPrefix(class, "state-") || emitted[class] != "" {
			continue
		}
		if class == "state-free" {
			continue // the tripwire below, which knows this one by name
		}
		t.Errorf("app.css styles %q and no template emits it; the browser cannot be asked about a "+
			"class that is never on the page, so either the pages have stopped reporting this state "+
			"(delete the rule and the word) or a template needs to say it", class)
	}

	// `state-free` is the one state the stylesheet draws and no page says. It predates the capture
	// folder, `cdp.STATE_CLASSES` leaves it out on purpose, and the browser cannot be asked about a
	// class that is never on a page. Both halves of that are asserted rather than remembered, because
	// each half going stale on its own is how the census starts lying: a template that begins to emit
	// the word has to put it in the census, and a stylesheet that stops drawing it should lose the
	// name.
	if styled, said := rules["state-free"], emitted["state-free"] != ""; styled == said {
		t.Errorf(`the "free" state is styled=%v and emitted=%v; exactly one of those is the fact `+
			`this repository holds. If a page renders it now, add it to cdp.STATE_CLASSES so Chrome is `+
			`asked about it; if app.css stopped drawing it, drop the rule and the sentence in `+
			`cdp.py that excuses it`, styled, said)
	}
}

// stylesheetRules returns every class name app.css writes a rule for, by its selector.
//
// Only a selector counts: the file's own prose names classes at length -- including the four this
// test is about -- and a substring search would read a comment as a rule. That is the same mistake
// #229's review caught on `internal/web/nav_test.go`, where two of the needles were landing in the
// comment above the block they named and the guard reported green against a stylesheet with the rules
// deleted.
//
// Two properties of this file's actual text decide the shape of the reader:
//
//   - A rule's selector is everything between the previous `}` (or the start of the file) and its
//     `{`, and the declaration blocks in this file contain no braces -- so one pass splits the file.
//   - An at-rule's preamble (`@media (prefers-color-scheme: dark) {`) is not a selector, and this
//     file's dark block re-declares the palette tokens rather than classes. Dropping what precedes a
//     chunk's last `{` means an at-rule contributes no class names, which is right for `:root` and
//     means a class styled ONLY inside an at-rule would not be seen. Every state class in this file is
//     styled in both schemes, so the census does not lean on that; if one day it does not, the answer
//     is to read the at-rule's body, not to widen the selector net.
//
// One limit stated rather than hidden: the comma split is textual, so a class written inside a
// functional pseudo-class argument -- this file has `label:has(> input:is([type="radio"],
// [type="checkbox"]))` at :439 -- would be claimed as a rule for it. No class is written that way
// today, and the error direction is the mild one: it can only make the census read *more* classes as
// styled, never fewer, so it can excuse a rule that exists and cannot hide one that is missing.
func stylesheetRules(css string) map[string]bool {
	out := map[string]bool{}
	css = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	class := regexp.MustCompile(`\.[A-Za-z][A-Za-z0-9_-]*`)
	for _, chunk := range strings.Split(css, "}") {
		i := strings.Index(chunk, "{")
		if i < 0 {
			continue
		}
		sel := chunk[:i]
		if strings.Contains(sel, "@") {
			sel = sel[strings.LastIndex(sel, "{")+1:]
		}
		for _, part := range strings.Split(sel, ",") {
			for _, m := range class.FindAllString(part, -1) {
				out[m[1:]] = true
			}
		}
	}
	return out
}

// The rule-set is what the guard reads, so the guard's own reader is worth pinning: a selector
// parser that finds nothing makes every assertion above vacuous, and "vacuous" is the one failure
// this repository has been burned by repeatedly (#211's reach check, #209's rules that ran 41
// of 42 times, #229's three green mutations).
func TestTheStylesheetRuleReaderReadsTheRulesItClaims(t *testing.T) {
	css, err := fs.ReadFile(Assets(), "app.css")
	if err != nil {
		t.Fatalf("app.css is not embedded: %v", err)
	}
	rules := stylesheetRules(string(css))
	// Six names, each from a different shape of rule in this file, and none of them one of the
	// assertions above: `rows` is a class qualified by an element, `state-held` is the pre-existing
	// state, `masthead__name` carries the double underscore a class-name reader has to allow,
	// `inline` is reached through `form.inline`, `request-id` through a hyphenated name, and `held`
	// is the row rule added by this branch -- an element-qualified selector whose class is also the
	// name of a colour token, so a reader that matched declarations rather than selectors would find
	// it for the wrong reason.
	for _, want := range []string{"rows", "state-held", "masthead__name", "inline", "request-id",
		"held"} {
		if !rules[want] {
			t.Errorf("stylesheetRules() does not see %q, which app.css does have a rule for; every "+
				"assertion in the census above would then pass against a stylesheet it cannot read",
				want)
		}
	}
	if len(rules) < 12 {
		t.Errorf("stylesheetRules() found %d classes: %v", len(rules), sortedKeys(rules))
	}
	// The prose must stay out of it. This is the mutation the comment above promises to survive, so
	// it gets its own probe: the file's prose names `state-free`, and so does a rule.
	prose := stylesheetRules("/* .from-the-comment { } */\n.from-the-rule { color: red; }")
	if prose["from-the-comment"] {
		t.Error("stylesheetRules() read a class out of a comment")
	}
	if !prose["from-the-rule"] {
		t.Error("stylesheetRules() does not read a rule it should")
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
