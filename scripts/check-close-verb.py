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

The same bytes answer a second question (#457, 21 Nov): does the body name the ticket the branch says it
delivers. An `issue-<N>` branch states its ticket in its name, and until #457 nothing read the two
together. The specimen is PR #454, re-measured 21 Nov from its merged body: `#375` occurs zero times --
the substring `375` appears three times, twice as the branch string `issue-375` and once inside
`selftest.sh:3756`, and no keyword names anything -- so GitHub registered no closing reference,
`factory/triage.sh` counted the merged pull as claiming nothing (correctly, by its design), and the
ticket was re-dispatched eight minutes after the merge landed. So on a branch whose name claims a
number, naming that number with NOTHING is refused: a documented keyword, a `Refs`/`Part of` line, or
any classification this file already reports counts as naming it, because a bare `#N` is a link and
registers with no queue -- the fact the no-keyword report has always recorded for the writer, now made
load-bearing. The branch is read from `git rev-parse --abbrev-ref HEAD` in the caller's directory
(which is where the delivery contract says to run this check: in the worktree); `--branch NAME`
overrides what it finds, `--branch ''` silences the leg, and the leg stays silent for `--span-blind`
reads, because commit messages are a whole branch's bytes, not one body's delivery claim.

The same body answers a third question (#497, 22 Nov): does it CLAIM to be the slice that finishes
the ticket. A ticket delivered in slices has every slice genuinely partial at the moment its body is
written, so `Refs #N` is the correct verb on all of them and the closer is owed by nobody -- measured
at trunk `eef3210` on 21 Nov: 11 of 11 open issues sitting in the `delivered` queue had merged
bodies naming their own ticket with `Refs` and nothing else, the oldest row 6d01h old. No artifact
recorded which slice completed the umbrella, so a finished ticket and a half-delivered one reached
`triage.sh` in one indistinguishable state, and the row may not close anything itself: "What this
arm must never grow is a close" (`factory/factory.sh`, the delivered-report block), and `Refs` stays
out of `CLOSER_RE` for #1256's reason. The missing artifact is the AUTHOR's statement, so the body
may now carry one: `Last slice of #N.` The phrase is deliberately not a GitHub keyword -- it fires
nothing on its own; it is a statement to this checker and to the next reader (a `--self-test` leg
pins that neither shipped closer pattern matches it). Where the statement is present this file
demands the closer: a declared number with no firing or negated keyword is refused. Where it is
absent the file stays silent, because the undeclared row stays honestly `delivered` and a genuinely
partial slice must never be pushed toward a false closer -- a closer on a half-delivered umbrella is
#1256 pointed the other way. Like the branch leg, the arm reads bodies only: a commit message is not
the author's statement about a pull request, and `--span-blind` reads stay declaration-silent in the
report. Unlike the branch leg, its numbers count as naming the branch claim -- one number, one
verdict line, and the declaration's message is the sharper diagnosis.

Exit status: 0 when nothing is inert, refused or negated, no last-slice declaration stands without
its keyword, and a claiming branch is named back, 1 when any of those is true, 2 when the body could
not be read. `negated` is in the failing set because a verb on a line saying `not` fires anyway,
which is the one class a reader is most likely to wave through. It is deliberately not an error for a
body to carry a firing closer -- a finished unit is supposed to ship one. It is an error to ship one
the parser will never see, or one whose keyword the parser does not have.

Usage:
    scripts/check-close-verb.py PATH [PATH...]     check bodies (`-` reads a body from stdin)
    scripts/check-close-verb.py --self-test        the five classes, the strip order, the branch leg
    scripts/check-close-verb.py --branch NAME      what the branch leg reads instead of HEAD
    scripts/check-commit-verbs.sh [BASE]           every commit message between BASE and HEAD
"""

import contextlib
import io
import re
import subprocess
import sys

USAGE = ("usage: scripts/check-close-verb.py [--self-test] [--span-blind] [--branch NAME] "
         "PATH [PATH...]\n"
         "       `-` as a PATH reads the body from stdin; `--branch ''` silences the branch leg")

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
# The last-slice declaration (#497): `Last slice of #N.`, the author's statement that this pull
# finishes the ticket. The phrase carries no GitHub keyword -- `CLOSER` matches nothing in it, and
# `CLOSER_RE` in `factory/factory.sh` is NOT widened, because the read side must never grow a closer
# it did not measure (#1256, H-0164) -- the declaration acts at write time, through this file alone.
# Adjacency is `CLOSER`'s own contract: a filler window here would invent declarations the sentence
# never made, the exact defect the 20-character window bought in #353.
LAST_SLICE = re.compile(r'\blast slice of[:, \t]*#([1-9][0-9]*)', re.IGNORECASE)
# An unclosed fence runs to the end of the body, which is what GitHub renders, so `\Z` is an arm of
# the alternation and not a fallback: without it the contents of a forgetfully-opened block land in
# the plain-text set and get reported as a closer that will never fire.
FENCE = re.compile(r'```.*?(?:```|\Z)', re.S)
INLINE = re.compile(r'`[^`\n]*`')
# The number a branch name claims: `issue-<N>`, bounded on both sides. The left bound keeps
# `issues-9` from claiming 9, which a substring-only match would take -- and `nonissue-5` besides;
# the right one keeps `issue-4570` claiming 4570 and not 457. Measured 21 Nov over `git branch -a`
# of this repository: `issue-<digits>` is the only shape carrying a bare issue number
# (`backup/audit-page-pre-rebase`, `docs/roadmap-h-0109`, `rescue/pre-h0234-reword`, `main`,
# `master` claim none -- the roadmap name holds a decision id, and the `issue-` prefix is what
# keeps it from being read as an issue).
BRANCH_ISSUE = re.compile(r'(?:^|[/._-])issue-([1-9][0-9]*)(?=$|[/.-])')


def branch_numbers(branch):
    """The issue numbers a branch name claims, ascending: `wip/issue-204-nav` claims 204."""
    return sorted(set(BRANCH_ISSUE.findall(branch or "")), key=int)


def unnamed_branch_numbers(c, branch):
    """Every number the branch claims that the classification names with nothing. A number in ANY
    class counts as named -- firing, negated (it fires anyway), inert, hazard, declared: this leg
    refuses silence about the branch's own number, not correctness about it, and a wrong claim is
    the other legs' verdict. A declared number is named, and the declaration leg (not this one)
    judges whether the closer arrived with it."""
    named = set()
    for cls in c.values():
        named.update(cls)
    return [n for n in branch_numbers(branch) if n not in named]


def current_branch():
    """The caller's branch via `git rev-parse --abbrev-ref HEAD`; the empty string whenever that
    question has no answer -- no git binary, no repository, detached HEAD, timeout. A leg that
    cannot see a branch says nothing: the rule exists for a unit standing in its worktree running
    the Step 5 check there, and refusing from a tarball or a detached HEAD would make a guard a
    tax, and #353's history is what guards become when they are taxes."""
    try:
        out = subprocess.run(["git", "rev-parse", "--abbrev-ref", "HEAD"],
                             stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True,
                             encoding="utf-8", errors="replace", timeout=5)
    except (OSError, subprocess.TimeoutExpired):
        return ""
    name = out.stdout.strip() if out.returncode == 0 else ""
    return "" if name == "HEAD" else name


def classify(body, spans=True):
    """Return {firing: {n: line}, negated: {n: line}, inert: {n: line}, neutral: {n: line},
    hazard: {n: line}, declared: {n: line}}.

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
    out = {"firing": {}, "negated": {}, "inert": {}, "neutral": {}, "hazard": {}, "declared": {}}
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
        # The last-slice declaration (#497), on the same stripped plain text that decides `firing`:
        # a declaration inside a code span quotes the form, it does not make it -- one strip order,
        # one shelter, the same measured one #353 bought. A negation BEFORE the phrase flips it
        # ("not the last slice of #10" states that no closer is owed); a negation in a LATER clause
        # ("Last slice of #10; the umbrella is not finished") belongs to that clause and leaves the
        # declaration standing. The line-wide NEGATION `firing` uses cannot tell those apart, so
        # this leg reads the negation's POSITION, not its presence.
        for m in LAST_SLICE.finditer(line):
            if NEGATION.search(line[:m.start()]):
                continue
            out["declared"].setdefault(m.group(1), line.strip())
    for span in inline + fenced:
        for n in CLOSER.findall(span):
            if n not in out["firing"] and n not in out["negated"]:
                out["inert"].setdefault(n, " ".join(span.split())[:120])
    for n in NEUTRAL.findall(body):
        if n not in out["firing"] and n not in out["negated"] and n not in out["inert"] \
                and n not in out["hazard"]:
            out["neutral"].setdefault(n, "")
    return out


def report(path, body, spans=True, branch=""):
    c = classify(body, spans)
    missing = unnamed_branch_numbers(c, branch)
    # The declaration leg (#497) is a BODY statement, so it reads only the body mode: a commit
    # message is not the author's claim about a pull request's place in a ticket, and GitHub acts
    # on no phrase there anyway -- so the leg is silent on the span-blind read, exactly like the
    # branch leg. A declared number answered by a firing or negated keyword is satisfied (a negated
    # verb fires anyway, which is #1256's own lesson); numbers whose only verb is inert or hazard
    # are refused by their own legs, whose advice -- write the keyword in running text -- already
    # reaches this author, and a doubled verdict line on one number reads as two claims.
    declared_unmet = []
    if spans:
        declared_unmet = [n for n in c["declared"]
                          if n not in c["firing"] and n not in c["negated"]
                          and n not in c["inert"] and n not in c["hazard"]]
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
    if missing:
        parts.append("REFUSED, branch %s claims %s, named with nothing"
                     % (branch, ", ".join("#" + n for n in missing)))
    if declared_unmet:
        parts.append("REFUSED, declared last slice with no keyword touching the number %s"
                     % ", ".join("#" + n for n in sorted(declared_unmet, key=int)))
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
    for n in missing:
        print("    the branch claims #%s and this body names it with no keyword and no `Refs`/`Part "
              "of` line; a bare `#N` is a link and registers with no queue. #454's body named #375 "
              "with nothing and the ticket was worked again eight minutes after the merge -- write "
              "`Refs #%s`, or the documented keyword touching the number when the ticket really is "
              "finished" % (n, n))
    for n in sorted(declared_unmet, key=int):
        # No closer vocabulary adjacent to a number even here (#1256 reaches this file's own stdout):
        # the advice names the fix as "the keyword touching the number", never the keyword itself.
        print("    #%s is declared the last slice of its ticket, and no documented keyword touches "
              "it in plain text: if this slice finishes the ticket, put the keyword beside the "
              "number where GitHub can see it; if it does not, drop the declaration -- an "
              "undeclared row stays honestly delivered: %s" % (n, c["declared"][n][:140]))
    return 1 if c["inert"] or c["negated"] or c["hazard"] or missing or declared_unmet else 0


GOOD = {
    "firing": "Body says it is finished.\n\nCloses #313 — the whole repair.\n",
    "neutral": "Part of #10 only; the umbrella is not finished.\n\nRefs #11.\n",
    # The declared last slice shipping the closer it declares (#497): the healthy final slice of a
    # multi-slice ticket -- the author's statement and the keyword touching the same number.
    "declared-good": "The final piece of the umbrella work.\n\nLast slice of #313.\n\n"
                     "Closes #313 and Closes #336.\n",
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
    # The declaration that promises more than the body delivers (#497): the author states the
    # ticket is finished and GitHub is asked for nothing -- the shape that left 11 delivered
    # issues open behind honest `Refs` bodies at trunk `eef3210`, one statement short.
    "declared-bare": "Delivers the last piece.\n\nLast slice of #10.\n",
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
    # The declaration (#497): the author's statement, its own class, judged on the same stripped
    # plain text that decides `firing`. The healthy slice states it and ships the keyword touching
    # the same number; the bare one states it and asks GitHub for nothing.
    expect("declared-good", GOOD["declared-good"], "declared", ["313"])
    expect("declared-good", GOOD["declared-good"], "firing", ["313", "336"])
    expect("declared-good", GOOD["declared-good"], "neutral", [])
    expect("declared-bare", BAD["declared-bare"], "declared", ["10"])
    expect("declared-bare", BAD["declared-bare"], "firing", [])
    # Negation POSITION decides a declaration, where `firing` reads only presence: a negation
    # before the phrase states that no closer is owed; a negation in a later clause belongs to
    # that clause and leaves the statement standing.
    expect("not-declared", "This is not the last slice of #10; the umbrella stays open.\n",
           "declared", [])
    expect("later-clause-neg", "Last slice of #10; the umbrella is not finished.\n",
           "declared", ["10"])
    # A quoted declaration states nothing -- one shelter, measured the same way as #353's keyword
    # shelter and decided by the same strip order -- and the phrase is pinned to fire no shipped
    # closer pattern: the read side does not grow, whatever a body says (#1256, H-0164).
    expect("span-declared", "The skill teaches `Last slice of #99` and closes nothing.\n",
           "declared", [])
    if CLOSER.search("Last slice of #99.") or CLOSER.search("last slice of: #99"):
        fails.append("declared-notcloser: a shipped closer pattern matched the declaration phrase")
    # Span-blind (a commit message) still CLASSIFIES the phrase -- message bytes get the
    # conservative read like every other verb does -- and it is the report that stays silent on
    # the leg, pinned below at the exit code.
    expect("spanblind-declared", "Last slice of #99.\n", "declared", ["99"], spans=False)
    # The branch leg (#457): what a branch name claims.
    def branch_expect(name, branch, want):
        got = branch_numbers(branch)
        if got != sorted(set(want), key=int):
            fails.append("branch-%s: %r parses %s, expected %s"
                         % (name, branch, got, sorted(set(want), key=int)))

    branch_expect("bare", "issue-457", ["457"])
    branch_expect("prefixed", "wip/issue-204-nav-preserved", ["204"])
    branch_expect("remote-prefixed", "origin/issue-12", ["12"])
    branch_expect("dotted", "issue-9.backport", ["9"])
    branch_expect("two", "issue-12-plus-issue-345", ["12", "345"])
    branch_expect("trunk", "main", [])
    branch_expect("plural-trap", "issues-9", [])
    branch_expect("non-issue-trap", "nonissue-5", [])
    branch_expect("zero-trap", "issue-0", [])
    branch_expect("long-number", "issue-4570", ["4570"])
    branch_expect("decision-not-issue", "docs/roadmap-h-0109", [])
    branch_expect("empty", "", [])

    # And what counts as naming the claim back. Every class the parser reports counts -- the leg
    # refuses silence about the branch's own number, not correctness about it -- while silence and
    # a bare link do not, and a healthy body aimed at a DIFFERENT number still leaves the branch's
    # number unnamed (the #454 shape exactly: a body that spoke a lot and named nothing).
    def misses_expect(name, body, branch, want):
        got = unnamed_branch_numbers(classify(body), branch)
        if got != list(want):
            fails.append("branch-%s: body against %r misses %s, expected %s"
                         % (name, branch, got, list(want)))

    misses_expect("firing", GOOD["firing"], "issue-313", [])
    misses_expect("neutral", GOOD["neutral"], "issue-10", [])
    misses_expect("negated-names", BAD["negated"], "issue-10", [])
    misses_expect("inert-names", BAD["inert"], "issue-301", [])
    misses_expect("hazard-names", HAZ["corpus-47"], "issue-46", [])
    # A declared number names the branch claim too (#497): this leg refuses silence, and the
    # declaration leg is what judges whether the closer arrived with the statement.
    misses_expect("declared-names", BAD["declared-bare"], "issue-10", [])
    misses_expect("silent", "Nothing names any ticket here.\n", "issue-457", ["457"])
    misses_expect("bare-link", "See #457 for the finding.\n", "issue-457", ["457"])
    misses_expect("wrong-number", "Refs #457.\n", "issue-458", ["458"])
    misses_expect("multi-branch", "Refs #12 and Refs #345.\n", "issue-12-plus-issue-345", [])
    misses_expect("trunk-body", "Nothing names any ticket here.\n", "main", [])

    # The refusal exists at the exit code, not only in the prose. `want_not` guards the verdict's own
    # wording: #1256's rule applies to this file's stdout too -- no closer vocabulary adjacent to a
    # number in text a scanning tool may one day read.
    def report_expect(name, body, branch, want_rc, want_in="", want_not="", spans=True):
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            got_rc = report(name, body, spans, branch)
        out = buf.getvalue()
        if got_rc != want_rc:
            fails.append("report-%s: rc %d, expected %d (first line: %r)"
                         % (name, got_rc, want_rc, out.splitlines()[:1]))
        if want_in and want_in not in out:
            fails.append("report-%s: verdict lacks %r" % (name, want_in))
        if want_not and want_not in out:
            fails.append("report-%s: verdict carries %r it must not" % (name, want_not))

    report_expect("silent-on-issue-branch", "Nothing names any ticket here.\n", "issue-457", 1,
                  want_in="REFUSED, branch issue-457 claims #457", want_not="closes #")
    report_expect("named-on-issue-branch", "Refs #457.\n", "issue-457", 0)
    report_expect("silent-on-trunk", "Nothing names any ticket here.\n", "main", 0)
    report_expect("bare-link-refused", "See #457 for the finding.\n", "issue-457", 1,
                  want_in="REFUSED, branch issue-457 claims #457", want_not="closes #")
    # The declaration arm (#497), pinned at the exit code. The bare statement is refused because it
    # promises a finished ticket and asks GitHub for nothing; the statement with its keyword passes;
    # a negated statement passes; a quoted statement passes; and silence passes -- an undeclared
    # row stays honestly delivered, because a genuinely partial slice must never be pushed toward
    # a false closer.
    report_expect("declared-bare-refused", BAD["declared-bare"], "", 1,
                  want_in="REFUSED, declared last slice", want_not="closes #")
    report_expect("declared-good-passes", GOOD["declared-good"], "", 0)
    report_expect("not-declared-passes",
                  "This is not the last slice of #10; the umbrella stays open.\n", "", 0)
    report_expect("span-declared-passes", "The skill teaches `Last slice of #99`.\n", "", 0)
    report_expect("later-clause-neg-refused",
                  "Last slice of #10; the umbrella is not finished.\n", "", 1,
                  want_in="REFUSED, declared last slice")
    # Span-blind silence: a commit message carrying the phrase is not the author's statement about
    # a pull request, so the report declines the leg there -- the branch leg's contract exactly.
    report_expect("spanblind-silent", "Last slice of #99.\n", "", 0, spans=False)
    # One number, one verdict: the declaration names the branch claim, so the refusal that speaks
    # for #10 is the declaration's and the branch leg stays quiet about the same number.
    report_expect("declared-names-branch", BAD["declared-bare"], "issue-10", 1,
                  want_in="REFUSED, declared last slice", want_not="REFUSED, branch")
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
    branch = None
    paths = []
    args = argv[1:]
    i = 0
    while i < len(args):
        a = args[i]
        if a == "--branch":
            if i + 1 >= len(args):
                print("check-close-verb: --branch needs a value (write `--branch ''` to silence "
                      "the leg explicitly)")
                return 2
            branch = args[i + 1]
            i += 2
        elif a.startswith("--branch="):
            branch = a.split("=", 1)[1]
            i += 1
        elif a == "--span-blind":
            i += 1
        else:
            paths.append(a)
            i += 1
    if not paths:
        print(USAGE)
        return 2
    # The leg reads the caller's HEAD only in body mode and only when not told otherwise: a
    # message pipe is a whole branch's bytes, not one delivery claim, and the caller over a pipe
    # has already said what it means by choosing --span-blind.
    if branch is None:
        branch = current_branch() if spans else ""
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
        rc = max(rc, report(path, body, spans, branch))
    if rc == 1:
        print("check-close-verb: a keyword inside a code span closes nothing, plain text on a "
              "line that says `not` fires anyway, an inflection GitHub does not document is "
              "refused because nobody has measured what it does, a branch named `issue-<N>` "
              "whose body names N with nothing re-offers its ticket after the merge, and a "
              "declared `Last slice of #N` with no keyword touching the number claims a closer "
              "nobody wrote. Write the documented keyword where GitHub can see it, or write "
              "`Refs` and say what is unfinished.")
    return rc


if __name__ == "__main__":
    sys.exit(main(sys.argv))
