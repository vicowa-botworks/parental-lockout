#!/usr/bin/env python3
"""Read a pull-request body and say which of its issue references will actually close something.

Why this exists (#353, 13 Nov; the pattern rewritten 14 Nov after review). A closing keyword written
inside a code span closes nothing, and every reader of the same sentence disagrees about it. Measured
over all 180 merged pull requests of this repository, read 14 Nov 2026, with the classifier this file
ships (`/tmp/opencode/p357/audit_closers.py`, which imports `CLOSER` from here rather than re-typing
it, and takes its population from `gh pr list --json number,body,mergedAt` plus
`gh issue list --json number,state,closedAt`): a verb in plain text naming an issue fired **97 of
100** times within two seconds of the merge, with nothing at all in the three-to-120-second bucket; a
verb inside inline code and nowhere else fired **0 of 2**, and both of its issues were closed by hand
6086 s and 14472 s after the merge that had claimed them. The two specimens are PR #308
(`Closes #304` in backticks) and PR #338 (`Closes #301.` in backticks, on a line that says the keyword
is deliberate). Meanwhile `factory/triage.sh`'s `CLAIM_RE` takes a backtick as one of its filler
characters, so the harness believes a backticked verb is a claim, reports the issue as named, and
re-offers it once the merge turns out to have delivered nothing to it.

The pattern is GitHub's keyword set and nothing else, and it requires the verb to be ADJACENT to the
reference. Both halves are measurements, not taste. The set is `close closes closed fix fixes fixed
resolve resolves resolved` -- there is no `-ing` form, so `Closing #77` and `Resolving #78` are not
requests; they were matched until 14 Nov, while `Fixing #79` was not, because the stems had been
copied from the claim matcher rather than from the keyword list. The adjacency is what GitHub
documents, and a filler window invents claims: the same 180 bodies scanned with up to 20 characters of
prose allowed between verb and `#N` answer **115** plain-text matches where **100** exist -- 107 naming
a real issue plus 8 naming a pull request or nothing at all. Fifteen of them are invented: 7 name real
issues no sentence asked about (#291, still open behind the line quoted in `BAD` below; #228, closed
29398 s BEFORE the merge that named it; #168, #304, #25, #232, #10), 7 name pull requests, which no
closer can act on, and 1 names #1180, a cross-repository citation that is not an artifact of this
repository. Six of the invented pairs ship as fixtures in `BAD` below and `--self-test` runs every one.
A wider pattern is not a safer pattern here: `factory/factory.sh`'s `factory_verify_closure` writes the
verdict of this same scan onto the live ticket, so an invented closer writes a false sentence on
somebody's issue.

So there are five things a body can say that look alike and are not alike, and the only useful report
names them apart:

    fires     a keyword in plain text naming an issue. GitHub acts at merge.
    negation  a keyword in plain text on a line that also says "not". It still fires: GitHub's
              parser has no notion of negation (#1256, measured -- `**This does not close #10.**`
              closed #10 the second the merge landed).
    inert     a keyword only ever inside inline code or a fenced block. It closes nothing, and it
              still reads as a claim to `CLAIM_RE`. This is the class that costs a unit.
    neutral   `Refs #10` and `Part of #10`. Never closers, reported so a reader can see the whole
              reference set rather than only the dangerous part of it.
    hazard    an inflection that is NOT in the documented set -- `Closing #77`, `Resolving #78` --
              standing next to the number in plain text. Refused, and refused on purpose without
              knowing whether GitHub acts on it. What is known: the published set has no `-ing` form,
              and all 180 merged bodies of this repository contain ONE adjacent `-ing` specimen (PR
              #47, whose prose puts a `-ing` verb on #46), so the corpus cannot separate the two
              readings -- one observation, and it is confounded by the same body also carrying a
              documented keyword on the same number. The two sides of this tool answer that
              uncertainty in opposite directions, which is not a contradiction but the point: the READ
              side -- `factory/factory.sh`'s `factory_verify_closure`, whose verdict is written onto a
              live ticket -- keeps the documented nine, because an invented closer there writes a false
              sentence on somebody's artifact (H-0164). The WRITE side, this file, refuses the
              superset, because the cost of refusing is one reworded sentence and the cost of being
              wrong the other way is an unplanned close. #1256's rule is the same rule and predates
              this paragraph: never put any inflection of the verb next to a number in text GitHub
              parses, not even to name or negate the hazard.

And text GitHub parses is not only the pull-request body. A commit message carrying the same adjacency
closes the same issue the moment its commit reaches the default branch, so
`scripts/check-commit-verbs.sh` runs this file over the branch's own messages and `make verify` runs
that. The measurement that bought it is this repository's own: `git log --format=%B origin/main..HEAD`
over the commits of PR #357 on 14 Nov answered two adjacent `-ing` forms in `8c9999b`'s message, in the
sentence that was warning about them -- the #245 shape from #1256, one layer down, in a place no check
looked. Both of its numbers were already-closed artifacts that day, which is luck about two numbers and
not a property of the shape.

Exit status: 0 when nothing is inert, refused or negated, 1 when any of those is true, 2 when the body
could not be read. `negated` is in the failing set because a verb on a line saying `not` fires anyway,
which is the one class a reader is most likely to wave through. It is deliberately not an error for a
body to carry a firing closer -- a finished unit is supposed to ship one. It is an error to ship one
the parser will never see, or one whose keyword the parser does not have.

Usage:
    scripts/check-close-verb.py PATH [PATH...]     check bodies (`-` reads a body from stdin)
    scripts/check-close-verb.py --self-test        the five classes and the strip order
    scripts/check-commit-verbs.sh [BASE]           every commit message between BASE and HEAD
"""

import re
import sys

USAGE = ("usage: scripts/check-close-verb.py [--self-test] [--span-blind] PATH [PATH...]\n"
         "       `-` as a PATH reads the body from stdin")

# GitHub's documented keyword set -- `close closes closed fix fixes fixed resolve resolves resolved`.
# No `-ing` form exists in it, and the verb has to touch the reference: `[:, \t]*` is the separator
# GitHub's own prose allows ("Closes #1", "Closes: #1", "close, #1"). Not `\s`, because `\s` matches a
# newline and this file, `factory/factory.sh` and the audit all read line by line -- a window that
# reaches across a line break is a window again, which is the defect this line replaced.
# `refs` and `part of` are NOT in the keyword set and must never be reported as closers, which is
# exactly why this file does not borrow `CLAIM_RE` from triage.sh: that pattern is a claim matcher,
# and a claim is a wider thing than a closer. `factory/selftest.sh` runs the two shipped spellings --
# this one and `CLOSER_RE` in `factory/factory.sh` -- over one table of lines and refuses a verdict
# they disagree on, because the drift between those two is what this rewrite was.
CLOSER = re.compile(r'\b(?:clos(?:e|es|ed)|fix(?:e[sd])?|resolv(?:e|es|ed))[:, \t]*#([1-9][0-9]*)',
                    re.IGNORECASE)
# The documented nine, as words, so the fifth class can name what is NOT one of them. Kept as a set of
# literals rather than derived from `CLOSER`'s source: a pattern is not a list, and this file's own
# history is a stem list copied from the wrong place.
KEYWORDS = frozenset("close closes closed fix fixes fixed resolve resolves resolved".split())
# Every inflection of the three stems that stands next to a number. `CLOSER` decides what GitHub is
# asked to do; this decides what a writer may not write. Read `classify` for why those are two
# questions, and why only the difference between them is refused.
HAZARD = re.compile(r'\b((?:clos|fix|resolv)[a-z]*)[:, \t]*#([1-9][0-9]*)', re.IGNORECASE)
NEUTRAL = re.compile(r'\b(?:refs?|part of)\b[^#\n]{0,20}#([1-9][0-9]*)', re.IGNORECASE)
NEGATION = re.compile(r"\b(?:not|never|no|n't|without|cannot|neither)\b", re.IGNORECASE)
# An unclosed fence runs to the end of the body, which is what GitHub renders, so `\Z` is an arm of
# the alternation and not a fallback: without it the contents of a forgetfully-opened block land in
# the plain-text set and get reported as a closer that will never fire.
FENCE = re.compile(r'```.*?(?:```|\Z)', re.S)
INLINE = re.compile(r'`[^`\n]*`')


def classify(body, spans=True):
    """Return {firing: {n: line}, negated: {n: line}, inert: {n: line}, neutral: {n: line},
    hazard: {n: line}}.

    Spans are stripped in one order and it matters: fenced blocks first, because a fence can hold
    backticks, then inline spans. Reversed, a backtick pair inside a fence reads as an inline span
    and the fence's contents leak into the plain-text set.

    `spans=False` reads every byte as plain text, and exists for commit messages. That a code span
    is inert is a MEASUREMENT, not a property of backticks: measured 14 Nov over this
    repository's merged bodies, a keyword inside inline code and nowhere else fired 0 of 2, so a
    body is read with spans. Nothing has ever measured what GitHub does with a backticked verb in
    a COMMIT MESSAGE, and the specimen that says this switch exists is one with its verbs inside
    backticks -- so the honest read there is the conservative one: no span is a shelter until it
    has been measured, and a quoted verb in a message is read as a claim. That direction costs a
    reworded sentence; the other one costs an unplanned close.
    """
    out = {"firing": {}, "negated": {}, "inert": {}, "neutral": {}, "hazard": {}}
    if spans:
        fenced = FENCE.findall(body)
        no_fence = FENCE.sub(" \x00 ", body)
        inline = INLINE.findall(no_fence)
        plain = INLINE.sub(" \x01 ", no_fence)
    else:
        fenced, inline, plain = [], [], body
    # Plain text, line by line, so a negation can be attributed to the line that carries the verb and
    # not to the body as a whole.
    for line in plain.splitlines():
        for n in CLOSER.findall(line):
            if NEGATION.search(line) and n not in out["firing"]:
                out["negated"].setdefault(n, line.strip())
            else:
                out["firing"][n] = line.strip()
                out["negated"].pop(n, None)
        # The fifth class, on the same stripped line as the first two: an inflection outside the
        # documented set next to a number. A number that already fires is not also reported here -- the
        # firing report is the stronger one, and a doubled verdict on one number reads as two claims.
        for word, n in HAZARD.findall(line):
            if word.lower() not in KEYWORDS and n not in out["firing"] and n not in out["negated"]:
                out["hazard"].setdefault(n, line.strip())
    for span in inline + fenced:
        for n in CLOSER.findall(span):
            if n not in out["firing"] and n not in out["negated"]:
                out["inert"].setdefault(n, " ".join(span.split())[:120])
    for n in NEUTRAL.findall(body):
        if n not in out["firing"] and n not in out["negated"] and n not in out["inert"] \
                and n not in out["hazard"]:
            out["neutral"].setdefault(n, "")
    return out


def report(path, body, spans=True):
    c = classify(body, spans)
    parts = []
    if c["firing"]:
        parts.append("closes %s" % ", ".join("#" + n for n in sorted(c["firing"], key=int)))
    if c["negated"]:
        parts.append("CLOSES DESPITE THE NEGATION %s" % ", ".join("#" + n for n in sorted(c["negated"], key=int)))
    if c["inert"]:
        parts.append("inert %s" % ", ".join("#" + n for n in sorted(c["inert"], key=int)))
    if c["hazard"]:
        parts.append("REFUSED, NOT A KEYWORD %s"
                     % ", ".join("#" + n for n in sorted(c["hazard"], key=int)))
    if c["neutral"]:
        parts.append("neutral %s" % ", ".join("#" + n for n in sorted(c["neutral"], key=int)))
    if not parts:
        # Accurate about its own reach. A bare `#123` with no keyword beside it is a real reference and
        # this tool does not see it: it reads keywords, not links, and a sentence that claimed to have
        # found no reference "of any kind" would be citing a scan that never looked for one.
        print("%s: no closing keyword and no Refs/Part of line; a bare `#N` here is a reference this "
              "tool does not look at" % path)
    else:
        print("%s: %s" % (path, "; ".join(parts)))
    for n in sorted(c["negated"], key=int):
        print("    #%s is a keyword in plain text on a line that says not, and it fires anyway: %s"
              % (n, c["negated"][n][:140]))
    for n in sorted(c["inert"], key=int):
        print("    #%s is inside a code span, so nothing will close it: %s" % (n, c["inert"][n][:140]))
    for n in sorted(c["hazard"], key=int):
        # The line is stored, not the word, so the verb is re-read from it -- and re-read for THIS
        # number: `Closing #77 and Resolving #78` is one line carrying two refusals, and reporting the
        # first verb against both numbers is the misattribution this file exists to refuse. With a
        # fallback, because a report that raises while explaining a refusal is a gate that disappears
        # on its own terms.
        verb = next((m.group(0) for m in HAZARD.finditer(c["hazard"][n]) if m.group(2) == n), None)
        print("    #%s is named by %s, an inflection GitHub does not document: whether it fires is "
              "unmeasured here, so it is refused -- write the documented keyword, or write `Refs` and "
              "say what is unfinished" % (n, ("`%s`" % verb)[:56] if verb else "a bare verb"))
    return 1 if c["inert"] or c["negated"] or c["hazard"] else 0


GOOD = {
    "firing": "Body says it is finished.\n\nCloses #313 — the whole repair.\n",
    "neutral": "Part of #10 only; the umbrella is not finished.\n\nRefs #11.\n",
    "mixed": "Fixes #5 and mentions `Refs #7` in passing.\n",
    "span-safe": "The page models the failing form: `Closes #99`, and quoting it is the point.\n",
    # GitHub's own multi-issue spelling, and the two separator forms the keyword set allows.
    "multi": "Resolves #10, resolves #12, and closes: #13.\n",
    # No `-ing` form is a GitHub keyword, and the adjacency rule is what keeps a verb about one
    # artifact from claiming the next number it can see.
    "no-ing": "Closing #77 and resolving #78, while Fixing #79 was never a keyword either.\n",
}
BAD = {
    "inert": "The keyword is here on purpose. `Closes #301.` — that is all.\n",
    "fenced": "```text\nCloses #42\n```\n",
    # The same block with its closing fence forgotten, which GitHub renders as a block to the end.
    "fence-open": "```text\nCloses #43\n",
    "negated": "**This does not close #10.**\n",
    "fence-leak": "```text\n`Refs #3` and then Closes #44\n```\nplain text names `#7`\n",
    # The six lines this repository actually merged that a 20-character filler window turned into
    # closing requests. Each is quoted exactly as merged; none of them closes what the window claims.
    "window-refs": "pull request #292 has the fix. Refs #291 for context only.\n",
    "window-crossrepo": "needs the boundary resolved by hand, and #1180 is why\n",
    "window-scope": "The fix is **#304**'s scope, not this.\n",
    "window-criterion": "Refs #167 rather than closing it: criterion 2 is #168's.\n",
    "window-passing": "Also fixes item 1 of #25 in passing.\n",
    "window-half": "Closes the write half of #10's remaining criteria\n",
}

# The fifth class, and the two readings of the same bytes. Neither fixture is a coinage: the first is
# PR #47's opening sentence as it merged (that body also carries a documented keyword on the same
# number, which is exactly why the corpus cannot settle whether an `-ing` form fires -- one specimen,
# and it is confounded); the second is the sentence commit `8c9999b` used to warn about this hazard,
# found by the scan this file now runs over commit messages.
HAZ = {
    "corpus-47": "Adds the audit **reader** — `Tail`, `NewestUser`, `Generations` — closing #46.\n",
    # Two verbs inside backticks, and a third separated from its number by a line break: the adjacency
    # the separator class allows is the adjacency the guard refuses, and the one a line break breaks is
    # genuinely broken -- no reading of these bytes reports #79.
    "commit-357": ("`fix(?:e[sd])?` has no `-ing` arm, so `Closing #77` and `Resolving #78` scanned\n"
                   "as closers and `Fixing\n#79` was never a keyword either.\n"),
    "suppressed": "Closes #12, and Closing #12 in the same breath.\n",
}


def self_test():
    fails = []

    def expect(name, body, want_class, want_numbers, spans=True):
        got = classify(body, spans)
        have = set(got[want_class])
        if have != set(want_numbers):
            fails.append("%s: %s holds %s, expected %s" % (name, want_class, sorted(have), sorted(want_numbers)))

    expect("firing", GOOD["firing"], "firing", ["313"])
    expect("firing", GOOD["firing"], "inert", [])
    expect("neutral", GOOD["neutral"], "neutral", ["10", "11"])
    expect("neutral", GOOD["neutral"], "firing", [])
    expect("mixed", GOOD["mixed"], "firing", ["5"])
    expect("mixed", GOOD["mixed"], "inert", [])
    expect("span-safe", GOOD["span-safe"], "inert", ["99"])
    expect("span-safe", GOOD["span-safe"], "firing", [])
    # Every number on the line is a closer, not just the first one, and both separator spellings take.
    expect("multi", GOOD["multi"], "firing", ["10", "12", "13"])
    expect("multi", GOOD["multi"], "neutral", [])
    # The three `-ing` spellings, and the one that only the claim matcher would have taken.
    expect("no-ing", GOOD["no-ing"], "firing", [])
    expect("no-ing", GOOD["no-ing"], "neutral", [])
    expect("inert", BAD["inert"], "inert", ["301"])
    expect("fenced", BAD["fenced"], "inert", ["42"])
    expect("fence-open", BAD["fence-open"], "inert", ["43"])
    expect("fence-open", BAD["fence-open"], "firing", [])
    expect("negated", BAD["negated"], "negated", ["10"])
    expect("negated", BAD["negated"], "firing", [])
    # The strip order, pinned. Inside the fence there is a neutral ref in backticks and a keyword in
    # plain text; if inline spans were stripped first the fence's contents would leak and #44 would
    # be reported as firing, which is the wrong answer in the dangerous direction.
    expect("fence-leak", BAD["fence-leak"], "inert", ["44"])
    expect("fence-leak", BAD["fence-leak"], "firing", [])
    # A body that names an issue both ways fires, and the inert report must not shadow it.
    expect("both", "Closes #8 in running text, and `Closes #8` again in a span.\n", "firing", ["8"])
    expect("both", "Closes #8 in running text, and `Closes #8` again in a span.\n", "inert", [])
    # The window, refused. Each merged line below used to be reported as a closer for a number the
    # sentence was not asking about; the honest verdict for every one of them is that nothing fires
    # and, where the sentence says `Refs`, that a neutral reference exists.
    expect("window-refs", BAD["window-refs"], "firing", [])
    expect("window-refs", BAD["window-refs"], "neutral", ["291"])
    expect("window-crossrepo", BAD["window-crossrepo"], "firing", [])
    expect("window-scope", BAD["window-scope"], "firing", [])
    expect("window-scope", BAD["window-scope"], "negated", [])
    expect("window-criterion", BAD["window-criterion"], "firing", [])
    expect("window-criterion", BAD["window-criterion"], "negated", [])
    expect("window-criterion", BAD["window-criterion"], "neutral", ["167"])
    expect("window-passing", BAD["window-passing"], "firing", [])
    expect("window-half", BAD["window-half"], "firing", [])
    # The fifth class. The corpus line is refused where the documented set sees no claim at all -- and
    # `firing` stays empty for it, because refusing a form is not the same as claiming GitHub acts on
    # it. Those are two questions asked by two patterns, and this pins that they stay apart.
    expect("corpus-47", HAZ["corpus-47"], "hazard", ["46"])
    expect("corpus-47", HAZ["corpus-47"], "firing", [])
    # Inside a body, backticks are a measured shelter, so a message quoted into one says nothing.
    expect("commit-357", HAZ["commit-357"], "hazard", [])
    expect("commit-357", HAZ["commit-357"], "inert", [])
    # Read span-blind, which is how a commit message is read, the same bytes carry two refusals -- and
    # the number behind the line break is still not a claim.
    expect("commit-357", HAZ["commit-357"], "hazard", ["77", "78"], spans=False)
    expect("commit-357", HAZ["commit-357"], "firing", [], spans=False)
    # And a keyword inside a span, read span-blind, is reported as the claim it might be: the inert
    # verdict is a measurement, and where nobody has made one the tool declines to hand it out.
    expect("span-blind-inert", BAD["inert"], "firing", ["301"], spans=False)
    expect("span-blind-inert", BAD["inert"], "inert", [], spans=False)
    # One number, both spellings: the firing report is the stronger verdict and is not doubled.
    expect("suppressed", HAZ["suppressed"], "firing", ["12"])
    expect("suppressed", HAZ["suppressed"], "hazard", [])
    if fails:
        for f in fails:
            print("check-close-verb --self-test: %s" % f)
        return 1
    n = len(GOOD) + len(BAD) + len(HAZ) + 1
    print("check-close-verb: %d fixtures, every class where the parser says it is" % n)
    return 0


def main(argv):
    if "--self-test" in argv:
        return self_test()
    # `--span-blind` is the commit-message reading; see `classify`. A flag on the tool rather than
    # a second entry point, so there stays one classifier and one place the two readings differ.
    spans = "--span-blind" not in argv
    paths = [a for a in argv[1:] if a != "--span-blind"]
    if not paths:
        print(USAGE)
        return 2
    rc = 0
    for path in paths:
        try:
            if path == "-":
                # A body from a pipe, which is how `scripts/check-commit-verbs.sh` hands over a
                # branch's messages: `git log --format=%B` writes them and this file decides.
                body = sys.stdin.read()
            else:
                with open(path, encoding="utf-8") as fh:
                    body = fh.read()
        except OSError as exc:
            print("check-close-verb: %s could not be read: %s" % (path, exc))
            return 2
        rc = max(rc, report(path, body, spans))
    if rc == 1:
        print("check-close-verb: a keyword inside a code span closes nothing, plain text on a "
              "line that says `not` fires anyway, and an inflection GitHub does not document is "
              "refused because nobody has measured what it does. Write the documented keyword "
              "where GitHub can see it, or write `Refs` and say what is unfinished.")
    return rc


if __name__ == "__main__":
    sys.exit(main(sys.argv))
