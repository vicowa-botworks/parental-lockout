package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
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
// The scope of this test is the `state-` prefix, which is the part of the vocabulary every reader can
// spell without being told where it lives. `TestEveryCensusNameIsSpelledAndDrawn` below takes the rest
// of the census -- `banner`, `flag`, `problem`, `refusal`, `override`, `exempt`, `override-expired`,
// `disabled` -- from its one home in `testing/capture/cdp.py`, and `pages.py`'s
// `every_state_class_the_pages_render_is_in_the_census` reads the same templates from the other side.
// The four composed row names are visible to none of these readers as a name: `dashboard.tmpl:33`
// writes `class="{{if .LockedOutNow}}held{{end}}{{if .CannotEvaluate}}unreadable{{end}}"` and
// `users.tmpl:43` the stale/unmanaged pair, so those names exist only as the text between two actions
// and the browser is the only reader that ever sees them resolved -- which is why `cdp.py` carries them
// by name in `ROW_STATE_CLASSES`. (This sentence used to claim the composed pairs were two `state-`
// words, `state-held` and `state-unknown`; `grep -n 'state-{{' internal/web/templates` answers nothing
// at all. The rows are the composed half, and #239 is the pass that noticed.)
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
//   - An at-rule's preamble (`@media (prefers-color-scheme: dark) {`) is not a selector, and no filter
//     keeps it out -- a fact about it does. A chunk is cut at its FIRST `{`, so the text read as a
//     selector is whatever precedes that brace, and a preamble carries no `.name` token. So an at-rule
//     contributes class names only through rules written after a `}`. That has one consequence, and this
//     stylesheet contains the case: the FIRST rule inside an at-rule body shares its chunk with the
//     preamble and is therefore read as the preamble. `app.css:239` opens a dark block whose first inner
//     rule is `tr.held` at :240, and that declaration is invisible here, while `tr.unreadable,
//     tr.stale` and `tr.unmanaged` later in the same block are seen (the block read on its own yields
//     `stale unmanaged unreadable` -- no `held`). Measured rather than inferred: `@media (x){ .a{} }` yields []
//     and `@media (x){ .a{} .b{} }` yields [b]. Nothing in this test turns on it today -- the four row
//     classes carry no `state-` prefix, so neither direction here asks about them, and `held` is in the
//     map anyway from its top-level rule at :227 -- but a state class drawn only inside a dark block,
//     written first, would be missed. What that costs is not symmetric, so it is stated per direction:
//     the first assertion ("a `state-` word a template says and the stylesheet never mentions") would
//     cry wolf, and the second ("a rule for a word nothing says") would stay quiet. If it ever matters,
//     the answer is to read the at-rule's body, not to widen the selector net -- and widening it is the
//     loose direction for the first assertion, which is the one worth keeping strict.
//
// There used to be an `if strings.Contains(sel, "@")` arm below, meant to drop that preamble. `sel` is
// the text up to the first `{`, so it can contain `@` and can never contain `{`; `LastIndex(sel, "{")`
// is therefore always -1 and the branch assigned `sel[0:]`, which is `sel`. It changed nothing, and the
// doc above described it working. It is gone, and this note is here so nobody writes it back.
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

// The two tuples `testing/capture/cdp.py` defines, as declared patterns rather than as strings built at
// call time: the `^` anchor is the whole difference between reading the census and reading the row list
// twice, and `TestTheCensusReaderReadsTheTupleItClaims` is the probe that keeps it.
var (
	stateTuple = regexp.MustCompile(`(?m)^STATE_CLASSES\s*=\s*\(([^)]*)\)`)
	rowTuple   = regexp.MustCompile(`(?m)^ROW_STATE_CLASSES\s*=\s*\(([^)]*)\)`)
	censusItem = regexp.MustCompile(`"([a-z][a-z0-9_-]*)"`)
)

// captureCensus reads the vocabulary out of the file that owns it.
//
// #199's finding was that the vocabulary had three homes and only the one being edited was ever read.
// The seam had the same defect twice over: this file kept its own copy of the half it could spell --
// the `state-` words, matched by prefix -- and `cdp.py` kept the rest, so every name that carries no
// prefix (`problem`, `override`, `override-expired`, `disabled`, `flag`, `banner`) was invisible to
// both directions of both guards. #239 is the pass that had to add two of them, which is the moment
// "invisible to the guard" stops being a description and starts being a bug: a new class could join the
// census with no rule in `app.css` and no template rendering it, and `go test` would have nothing to
// say about either.
//
// So the census is read from its one home instead of remembered here, the way `pages.py` reads the
// templates from theirs. What still is NOT covered, stated rather than left implied: a class name that
// is in neither the census nor the `state-` prefix is outside every guard in the repository except the
// browser's, and only when a capture runs -- `manifest.py check` is what forces a capture after any
// edit to `app.css`, which is the chain that closes it.
func captureCensus(t *testing.T) (words, rows []string) {
	t.Helper()
	path := filepath.Join("..", "..", "testing", "capture", "cdp.py")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the state census cannot be read from %s: %v. It lives there as `STATE_CLASSES`; a "+
			"vocabulary test that fell back to a list it invented would be asserting the invention, not "+
			"the vocabulary", path, err)
	}
	words = censusNames(stateTuple.FindStringSubmatch(string(b)))
	rows = censusNames(rowTuple.FindStringSubmatch(string(b)))
	// A reader that answered nothing would make every assertion below true, which is the failure this
	// repository has been burned by on every guard it has ever written (#211's reach check, #209's
	// rules that ran 41 of 42 times, #229's three green mutations). So the shape is asserted here, and
	// both names named are ones the guards are about: one from each tier of the vocabulary.
	if len(words) < 11 || len(rows) != 4 {
		t.Fatalf("read %d word names and %d row names out of %s, want at least 11 and exactly 4 (%v / %v):"+
			" the census is what this test measures, so a short read is a refusal, not a smaller claim",
			len(words), len(rows), path, words, rows)
	}
	for _, want := range []string{"state-held", "banner"} {
		if !hasName(words, want) {
			t.Errorf("cdp.STATE_CLASSES does not name %q, which every page in the set renders: the census "+
				"has been edited into something else and the assertions below are about that instead", want)
		}
	}
	if d := duplicates(words, rows); len(d) > 0 {
		t.Errorf("%s lists %v twice; a name counted in both tuples is a name the browser is asked about "+
			"twice and a coverage rule compares once", path, d)
	}
	return words, rows
}

func censusNames(m []string) []string {
	if m == nil {
		return nil
	}
	var out []string
	for _, q := range censusItem.FindAllStringSubmatch(m[1], -1) {
		out = append(out, q[1])
	}
	return out
}

func hasName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

func duplicates(lists ...[]string) []string {
	seen, dup := map[string]int{}, []string{}
	for _, l := range lists {
		for _, n := range l {
			seen[n]++
		}
	}
	for n, c := range seen {
		if c > 1 {
			dup = append(dup, n)
		}
	}
	sort.Strings(dup)
	return dup
}

func TestTheCensusReaderReadsTheTupleItClaims(t *testing.T) {
	src := "ROW_STATE_CLASSES = (\"held\", \"stale\")\n\n" +
		"STATE_CLASSES = (\"state-held\", \"banner\") + ROW_STATE_CLASSES\n"
	if got := censusNames(stateTuple.FindStringSubmatch(src)); !reflect.DeepEqual(got,
		[]string{"state-held", "banner"}) {
		t.Errorf("the census reader answered %v for STATE_CLASSES; without the `^` anchor it reads "+
			"ROW_STATE_CLASSES instead, and every name checked below would then be a row name", got)
	}
	if got := censusNames(rowTuple.FindStringSubmatch(src)); !reflect.DeepEqual(got,
		[]string{"held", "stale"}) {
		t.Errorf("the census reader answered %v for ROW_STATE_CLASSES", got)
	}
	// The `+ ROW_STATE_CLASSES` on the second line is the shape the real file has. A reader that ran to
	// the last `)` would claim `ROW_STATE_CLASSES` is a class name, and one that ignored the closing
	// paren would do the same from the other direction.
	if got := censusNames(stateTuple.FindStringSubmatch(src)); hasName(got, "ROW_STATE_CLASSES") {
		t.Error("the census reader read the tuple's own name out of the concatenation")
	}
}

// TestEveryCensusNameIsSpelledAndDrawn is both directions of the name seam, over the whole census
// rather than the `state-` half of it.
//
// It is the guard #239 needed and did not have. Adding `refusal` and `exempt` to the census adds two
// questions Chrome is asked on nine pages; before this test, adding a name there could not be refused
// by any Go assertion, because the only census this file could see was the one it could spell. The two
// directions are the two ways a census goes stale, and both were measured in #199: a name nobody
// renders means the browser is asked about a class that is never on a page (the guard passes on
// nothing), and a rendered name with no rule means a state word drawn in the voice of the prose around
// it (the guard refuses -- at the next capture, which is a minute too late for a branch).
func TestEveryCensusNameIsSpelledAndDrawn(t *testing.T) {
	words, rows := captureCensus(t)
	_, where := templateClassTokens(t)
	css, err := fs.ReadFile(Assets(), "app.css")
	if err != nil {
		t.Fatalf("app.css is not embedded: %v", err)
	}
	rules := stylesheetRules(string(css))

	for _, name := range append(append([]string{}, words...), rows...) {
		if where[name] == "" {
			t.Errorf("cdp.STATE_CLASSES measures .%s and no template puts that name on an element, so "+
				"every photographed page asks Chrome about a class that is never on it -- the census is "+
				"a census of the templates, and a rename in the markup is what this looks like (#199)",
				name)
			continue
		}
		if !rules[name] {
			t.Errorf("app.css has no rule for %q, which %s puts on a word. This is the #199 defect in the "+
				"half of the vocabulary that carries no `state-` prefix -- the half "+
				"TestEveryStateClassTheTemplatesNameIsDrawn cannot see and, until this test read the census "+
				"itself, could not see either: the class is in the markup and nothing turns it into "+
				"something a reader can see. Add the rule, or stop naming the state.", name, where[name])
		}
	}
	// The four composed row names are asserted like every other name now, and that is new: the reader
	// here used to cut attributes before it cut actions, which made `users.tmpl:43` and `dashboard.tmpl:33`
	// unreadable and left the four row classes out of `app.css` for exactly the reason pages.py's coverage
	// rule names -- "a reader that cannot see them excuses them, and the guard stays green on a stylesheet
	// that draws none of them". They are drawn as `tr.held` and friends, so the rule reader finds them by
	// the name the browser resolves.
	for _, name := range rows {
		if !hasName(words, name) {
			continue
		}
		t.Errorf("%q is in both cdp.STATE_CLASSES and cdp.ROW_STATE_CLASSES, so this test's two tiers are "+
			"not two tiers", name)
	}
}

// layoutHooks are the names the templates put on an element to say what the element IS, and what it is
// is not a state the daemon reached. Nothing in app.css reads any of them, which is the fact this table
// exists to keep honest.
//
// #239 produced this list by accident. Its first mutation probe renamed `class="refusal"` to
// `class="warned"` in one template and every Go assertion stayed green: the state-only guards did not
// know `warned`, the census did not either, and nothing anywhere asked whether a name the pages write is
// read by the stylesheet. That is #199's finding again in a different clothes -- "a class can be in the
// DOM, named in the stylesheet's prose, and do nothing at all" -- and app.css had already noticed one of
// these by hand: its `nav` block calls the shell's `class="inline-form"` "a class no rule in this file
// ever read, the same kind of unwired hook D-0012 refused". Prose that notices a hole is how the hole
// stays. So the vocabulary is closed from both ends now: every name the pages write is either drawn, or
// a state in the census (and therefore drawn by the test above), or declared here with the element it
// marks. A new name that is none of those is a refusal, and the two ways it can be wrong are the two
// ways this repository has been wrong before: a state with no rule, and a name nobody reads.
//
// What this does NOT claim: that a hook is wired to anything. `inline-form` is the counter-example the
// stylesheet already carries, and no assertion here can tell a deliberate layout hook from a dead one --
// only that the difference is now a decision somebody had to write down instead of an accident.
var layoutHooks = map[string]string{
	"assigned":       "the `Shared by` paragraph on /profiles",
	"create-profile": "the add-a-profile `section`, beside `profile`",
	"edit-form":      "every POST form on /profiles and /users (`<form>`, 12 sites)",
	"filters":        "the audit filter form (the one `GET` on /audit)",
	"inline-form":    "the sign-out `<form>` in the shell's nav (app.css names it as unwired)",
	"linklike":       "the sign-out `<button>` that is made to look like a link",
	"masthead":       "the shell's `<header>` (only `masthead__name` inside it is drawn)",
	"meta":           "the id-and-timezone `<p>` under a profile heading",
	"notes":          "the `aside` that explains how a page's list was produced (/audit, /users)",
	"opt":            "a `<label>` in the assign-profile cell",
	"profile":        "the `<section>` per profile on /profiles",
	"rule":           "the rule-name `<span>` in the status page's Why cell",
	"sessions":       "the session `<ul>` on the status page",
}

func TestEveryClassTheTemplatesWriteIsDrawnOrADeclaredHook(t *testing.T) {
	tokens, where := templateClassTokens(t)
	words, rows := captureCensus(t)
	css, err := fs.ReadFile(Assets(), "app.css")
	if err != nil {
		t.Fatalf("app.css is not embedded: %v", err)
	}
	rules := stylesheetRules(string(css))

	n := 0
	for tok, tmpl := range where {
		n++
		if rules[tok] || hasName(words, tok) || hasName(rows, tok) {
			continue
		}
		if _, ok := layoutHooks[tok]; ok {
			continue
		}
		t.Errorf("%s writes class=%q, which app.css has no rule for and nothing declares. Either it is a "+
			"state the daemon reports -- then it belongs in cdp.STATE_CLASSES, where Chrome is asked about "+
			"it on every photographed page, and it needs a rule (that is #199: `a class can be in the DOM, "+
			"named in the stylesheet's prose, and do nothing at all`) -- or it says what the element IS, "+
			"then name it in `layoutHooks` with the element it marks. Renaming a state class into a name "+
			"nothing reads is the case this rule was written for: mutation probe 1 renamed `refusal` to "+
			"`warned` and every other assertion in this file stayed green.", tmpl, tok)
	}
	if n < 20 {
		t.Fatalf("only %d class names are emitted by the templates, so the rule above examined almost "+
			"nothing: the reader is broken, not the pages", n)
	}
	for hook, what := range layoutHooks {
		if !tokens[hook] {
			t.Errorf("`layoutHooks` declares %q (%s) and no template writes that class any more. A stale "+
				"exemption is how a name becomes invisible to both guards at once -- the page stopped "+
				"saying it and the stylesheet never drew it, and this table is the only place either reader "+
				"would have noticed", hook, what)
		}
		if rules[hook] {
			t.Errorf("`layoutHooks` declares %q as a name app.css does not read, and it now has a rule for "+
				"it. Take it out of the table: the drawing is checked by the rules above, and a name in two "+
				"lists means neither is read", hook)
		}
		if hasName(words, hook) || hasName(rows, hook) {
			t.Errorf("%q is in the state census and in `layoutHooks`: one name cannot be both a state the "+
				"daemon reached and a word for what an element is. That is the defect #239 is about.", hook)
		}
	}
}

// classTokens reads the literal class names out of one template's text, with a label for where each
// name was first spelled (a file name at the call site, anything at the probe's).
//
// Three properties of this file's markup decide the shape of the reader, and the order of the two
// regexps is the first of them:
//
//   - Actions are cut out BEFORE the attributes are split, for the two reasons `pages.py` records at
//     `every_state_class_the_pages_render_is_in_the_census`, and they cut both ways. `users.tmpl:43`
//     writes `class="{{if .Stale}}stale{{end}}{{if not .Configured}}unmanaged{{end}}"`: cutting the
//     actions is what lets this reader see the four composed row names AT ALL -- a reader that insists on
//     a whole literal attribute calls that attribute unreadable and stops counting, which is exactly how
//     all four stayed out of `app.css` until pages.py's coverage rule said so -- and it is also what
//     keeps the words of the actions out of the census, because `not` sitting between two actions is Go
//     syntax, not a class (measured: this reader counted it as one before the cut went in). The other
//     reason is `page.tmpl:39`'s `class="page-{{template "pagename" .}}"`, which carries a quote INSIDE
//     the action: splitting attributes before cutting actions ends the attribute at that inner quote and
//     keeps half an action. What the cut leaves behind is the stub `page-`, and a prefix is not a name --
//     dropped below, by the same rule pages.py uses. `TestTheClassTokenReaderCutsActionsBeforeItSplits`
//     pins all three outcomes, which is why the reader is its own function: the only way to pin the order
//     of two regexps is to run the reader on the string that gets the order wrong.
//   - Tokens, not substrings. `override` is a substring of `override-expired`, so asking whether any
//     class attribute CONTAINS a name reports `override` as rendered on a page that stopped rendering
//     it -- the false green a vocabulary guard exists to refuse. Whole tokens only.
//   - Attributes, not prose. Unlike the `state-` reader in the first test, this one reads class
//     attributes, so a template comment that NAMES a class is not counted as one that emits it.
func classTokens(label, text string) (map[string]bool, map[string]string) {
	stripped := regexp.MustCompile(`\{\{.*?\}\}`).ReplaceAllString(text, " ")
	attr := regexp.MustCompile(`class="([^"]*)"`)
	tokens, where := map[string]bool{}, map[string]string{}
	for _, m := range attr.FindAllStringSubmatch(stripped, -1) {
		for _, tok := range strings.Fields(m[1]) {
			// A name built out of a value leaves its prefix behind once the action is cut
			// (`class="page-"`). A prefix is not a name, and no static reader can resolve it.
			if tok == "" || strings.HasSuffix(tok, "-") {
				continue
			}
			tokens[tok] = true
			if where[tok] == "" {
				where[tok] = label
			}
		}
	}
	return tokens, where
}

func TestTheClassTokenReaderCutsActionsBeforeItSplits(t *testing.T) {
	// Trap one, verbatim from users.tmpl:43. The two row names exist only BETWEEN the actions, so the cut
	// has to expose them -- a reader that cannot see them is the reader that let all four stay undrawn.
	// The words of the actions are not classes, and `not` is the one this reader actually read as one.
	row := `<tr class="{{if .Stale}}stale{{end}}{{if not .Configured}}unmanaged{{end}}">`
	gotRow, _ := classTokens("row", row)
	if !gotRow["stale"] || !gotRow["unmanaged"] || len(gotRow) != 2 {
		t.Errorf("the composed row yielded %v, want exactly `stale` and `unmanaged`: the actions have to "+
			"be cut before the attribute is split, or the four composed row names are invisible to every "+
			"guard in this file and to app.css beside them (that is how #199 shipped)", sortedKeys(gotRow))
	}
	if gotRow["not"] {
		t.Error("`not` is Go syntax between two actions, not a class: the cut removed the actions and " +
			"this reader still found the word, so it split the attribute before it cut")
	}
	// Trap two, verbatim from page.tmpl:39: the quote is INSIDE the action, so an attribute read that
	// has not cut the action stops at that inner quote and keeps half an action. What the cut leaves is
	// the prefix `page-`, and a prefix is not a name.
	body := `<body class="page-{{template "pagename" .}}">`
	if got, _ := classTokens("body", body); len(got) != 0 {
		t.Errorf("the composed body class yielded %v; `page-` is a prefix left by the cut, not a name, "+
			"and no rule in app.css reads it either", sortedKeys(got))
	}
	// And two real names have to survive all of this, or the assertions above pass on emptiness.
	got, where := classTokens("probe", `<p class="refusal" role="alert">no</p><span class="problem">x</span>`)
	if !got["refusal"] || !got["problem"] || len(got) != 2 || where["refusal"] != "probe" {
		t.Errorf("the reader did not read the two literal classes (got %v, attributed to %q): a reader "+
			"that returns nothing makes every rule built on it pass", sortedKeys(got), where["refusal"])
	}
	// The substring trap, on the two names that are actually confusable in this vocabulary.
	got, _ = classTokens("probe", `<span class="override-expired">x</span>`)
	if got["override"] {
		t.Error("the reader read `override` out of `override-expired`: the census would then report the " +
			"open hold as rendered on a page that stopped rendering it")
	}
}

// templateClassTokens runs that reader over every embedded template and keeps the file each name was
// first spelled in, for the messages that have to name a place.
func templateClassTokens(t *testing.T) (map[string]bool, map[string]string) {
	t.Helper()
	names, err := fs.Glob(pageFS, "templates/*.tmpl")
	if err != nil {
		t.Fatalf("the embedded templates cannot be listed: %v", err)
	}
	if len(names) < 5 {
		t.Fatalf("only %d templates are embedded; a census of the vocabulary is only an answer about "+
			"the pages if all of them are here", len(names))
	}
	tokens, where := map[string]bool{}, map[string]string{}
	for _, name := range names {
		b, err := fs.ReadFile(pageFS, name)
		if err != nil {
			t.Fatalf("the embedded template %q cannot be read: %v", name, err)
		}
		got, seen := classTokens(name, string(b))
		for tok := range got {
			tokens[tok] = true
			if where[tok] == "" {
				where[tok] = seen[tok]
			}
		}
	}
	if len(tokens) < 10 {
		t.Fatalf("read %d class tokens out of %d templates: the attribute reader is reading nothing, "+
			"which is the failure every rule built on it exists to catch", len(tokens), len(names))
	}
	if tokens["not"] {
		t.Fatalf("a template now spells a class `not`, or the action cut has stopped working; either way " +
			"the vocabulary census is about to inherit a word from the Go syntax of the markup")
	}
	return tokens, where
}

// classSite is one place a vocabulary class is written, with the two facts about its surroundings that
// decide what it means: the element it is on and the table cell it sits in.
type classSite struct {
	tmpl, name, tag, role, cell string
	inCell                      bool
}

var (
	stateTag  = regexp.MustCompile(`<(/?)(td|tr|p|span|aside|div|li)\b([^>]*)>`)
	classAttr = regexp.MustCompile(`class="([^"]*)"`)
	roleAttr  = regexp.MustCompile(`role="([^"]*)"`)
	labelAttr = regexp.MustCompile(`data-label="([^"]*)"`)
)

// classSites walks every template once, keeping the cell it is currently inside.
//
// A forward scan, not a search backwards from the class, because the backward form is the one
// `lock_now_test.go` documents reaching into the neighbouring cell and reporting a seam that is not
// there. The templates close every cell before opening the next, so "the nearest `<td` with no `</td`
// between it and here" is the cell the browser paints. `</tr>` closes a cell defensively: a template
// that dropped its `</td>` would otherwise make the rest of the row look cell-less rather than
// malformed, and the malformed shape is the one worth noticing.
//
// One limit: `[^>]*` is the attribute run, and it is only an attribute run while no template action
// contains a `>`. `grep -n '{{[^}]*>[^}]*}}' internal/web/templates` answers nothing at this head, so
// the reader is exact today; an action that ever needs a bare `>` would land OUTSIDE a cell rather than
// inside the wrong one, which is the loud direction for every rule below.
func classSites(t *testing.T, names map[string]bool) []classSite {
	t.Helper()
	names2, err := fs.Glob(pageFS, "templates/*.tmpl")
	if err != nil {
		t.Fatalf("the embedded templates cannot be listed: %v", err)
	}
	sort.Strings(names2)
	var out []classSite
	var cell string
	var inCell bool
	for _, tmpl := range names2 {
		b, err := fs.ReadFile(pageFS, tmpl)
		if err != nil {
			t.Fatalf("the embedded template %q cannot be read: %v", tmpl, err)
		}
		for _, m := range stateTag.FindAllStringSubmatch(string(b), -1) {
			closing, kind, attrs := m[1] == "/", m[2], m[3]
			switch {
			case kind == "td" && !closing:
				inCell, cell = true, firstMatch(labelAttr, attrs)
			case kind == "td" && closing, kind == "tr" && closing:
				inCell, cell = false, ""
			}
			for _, tok := range strings.Fields(firstMatch(classAttr, attrs)) {
				if !names[tok] {
					continue
				}
				out = append(out, classSite{
					tmpl: tmpl, name: tok, tag: kind, cell: cell, inCell: inCell,
					role: firstMatch(roleAttr, attrs),
				})
			}
		}
	}
	return out
}

func firstMatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return m[1]
}

// TestEveryStateClassCarriesOneMeaning is the split, written where a browser cannot reach it.
//
// #239's title is the whole claim: two classes carried two meanings each, and the guard that measures
// whether a mark is VISIBLE cannot see a mark on the WRONG thing. `cdp.require_state_legible` passed on
// every page before this split and passes on them now, because the marking was consistent both times --
// what it cannot ask is whether the sentence under the mark is the kind of sentence the mark is for.
// For the two classes that split, the answer lives in the shape of the markup they are written in, so
// the shape is what is asserted:
//
//	problem  -- the daemon's own words about one row (9 sites: 5 on /users, one each in the status
//	           page's Why cell and the lock page's Detail cell, two on the probe page's Detail cells).
//	           Always inside a cell; never announced. Out of a cell it is a refusal, a mode notice or a
//	           paragraph of help prose again -- three of the six jobs the issue counts, all three of
//	           them paragraphs, all three now wearing another name.
//	refusal  -- a write that did not happen, and the evaluator's sentence about a rule that can never
//	           fire (3 sites: the page-top refusal on /profiles, the profile's complaint, the settings
//	           refusal). Always `role="alert"`, always outside a cell: a refusal is a paragraph above
//	           or beside the form it refused, and it is read out to the operator by the same thing that
//	           reads the audit trail aloud.
//	override -- the hold the operator granted and nothing else. It lives in the status page's "Why"
//	           column, which is the only surface that grants a hold.
//	exempt   -- the session the daemon runs inside, which no schedule will lock (2 sites, the two
//	           result pages). It lives in a "Detail" cell, beside the `.problem` detail it must stay
//	           distinguishable from (#108's seam, and the reason both classes keep a voice of their own).
//
// Each of the four is asserted PRESENT as well as correct: a rule that read no site would pass, which
// is the failure this folder refuses on every guard it writes.
func TestEveryStateClassCarriesOneMeaning(t *testing.T) {
	tracked := map[string]bool{"problem": true, "refusal": true, "override": true, "exempt": true}
	sites := classSites(t, tracked)
	seen := map[string]int{}
	for _, s := range sites {
		seen[s.name]++
		switch s.name {
		case "problem":
			if !s.inCell {
				t.Errorf("%s puts `.problem` on a <%s> outside every table cell. The class means \"the "+
					"daemon's own words about this row\" and nothing else since #239: out of a cell it is "+
					"a refusal, a standing notice or help prose, and each of those has its own name now "+
					"(refusal, disabled, muted).", s.tmpl, s.tag)
			}
			if s.role != "" {
				t.Errorf("%s puts `.problem` on a <%s role=%q>. An announced sentence is the page talking "+
					"to the operator; `.problem` is the machine's words about one row, quoted. #239 moved "+
					"every announced sentence here to `.refusal`.", s.tmpl, s.tag, s.role)
			}
		case "refusal":
			if s.inCell {
				t.Errorf("%s puts `.refusal` inside the %q cell. A refusal is the answer to the form you "+
					"just posted, so it is a paragraph above or beside that form (#138's ordering); a word "+
					"about one row is `.problem`.", s.tmpl, s.cell)
			}
			if s.role != "alert" {
				t.Errorf("%s puts `.refusal` on a <%s> with role=%q. The class means \"the daemon refused "+
					"this\", which is exactly what role=\"alert\" announces; a refusal nobody is told about "+
					"is how a saved-but-refused rule reads as a saved one.", s.tmpl, s.tag, s.role)
			}
		case "override":
			if s.cell != "Why" {
				t.Errorf("%s puts `.override` (cell %q, in a cell: %v). The open hold is reported in the "+
					"status page's \"Why\" column, which is the column that explains why a schedule is not "+
					"holding; #239 gave the daemon's self-exemption its own name (`exempt`) rather than "+
					"leaving one class to mean a hold and the absence of one.", s.tmpl, s.cell, s.inCell)
			}
		case "exempt":
			if s.cell != "Detail" {
				t.Errorf("%s puts `.exempt` (cell %q, in a cell: %v). The self-session sentence belongs in "+
					"a session's Detail cell, beside the detail it has to stay readable against (#108).",
					s.tmpl, s.cell, s.inCell)
			}
		}
	}
	for _, name := range []string{"problem", "refusal", "override", "exempt"} {
		if seen[name] == 0 {
			t.Errorf("no `class=%q` site was read in any embedded template, so the rule above examined "+
				"nothing about it: the class was renamed, deleted, or written in a shape the reader here "+
				"does not parse -- all three are a silent retirement of this guard", name)
		}
	}
}
