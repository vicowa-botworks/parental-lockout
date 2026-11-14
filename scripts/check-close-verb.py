#!/usr/bin/env python3
"""Read a pull-request body and say which of its issue references will actually close something.

Why this exists (#353, 13 Nov). A closing keyword written inside a code span closes nothing, and
every reader of the same sentence disagrees about it. Measured over all 176 merged pull requests of
this repository (`/tmp/opencode/p353/closer_audit_all.py`, same classifier as this file): a verb in
plain text fired **92 of 94** times within two seconds of the merge, with nothing at all in the
three-to-120-second bucket; a verb inside inline code fired **0 of 2**, and both of its issues were
closed by hand 6086 s and 14472 s after the merge that had claimed them. The two specimens are PR
#308 (`Closes #304` in backticks) and PR #338 (`Closes #301.` in backticks, on a line that says the
keyword is deliberate). Meanwhile `factory/triage.sh`'s `CLAIM_RE` takes a backtick as one of its
filler characters, so the harness believes a backticked verb is a claim, reports the issue as named,
and re-offers it once the merge turns out to have delivered nothing to it.

So there are three things a body can say that look alike and are not alike, and the only useful
report names them apart:

    fires     a keyword in plain text naming an issue. GitHub acts at merge.
    negation  a keyword in plain text on a line that also says "not". It still fires: GitHub's
              parser has no notion of negation (#1256, measured -- `**This does not close #10.**`
              closed #10 the second the merge landed).
    inert     a keyword only ever inside inline code or a fenced block. It closes nothing, and it
              still reads as a claim to `CLAIM_RE`. This is the class that costs a unit.
    neutral   `Refs #10` and `Part of #10`. Never closers, reported so a reader can see the whole
              reference set rather than only the dangerous part of it.

Exit status: 0 when nothing is inert, 1 when something is, 2 when the body could not be read. It is
deliberately not an error for a body to carry a firing closer -- a finished unit is supposed to ship
one. It is an error to ship one the parser will never see.

Usage:
    scripts/check-close-verb.py PATH [PATH...]     check bodies
    scripts/check-close-verb.py --self-test        the four classes and the strip order
"""

import re
import sys

# GitHub's own keyword set. `refs` and `part of` are NOT in it and must never be reported as closers,
# which is exactly why this file does not borrow `CLAIM_RE` from triage.sh: that pattern is a claim
# matcher, and a claim is a wider thing than a closer.
CLOSER = re.compile(r'\b(?:clos(?:e|es|ed|ing)|fix(?:e[sd])?|resolv(?:e|es|ed|ing))\b[^#\n]{0,20}#([1-9][0-9]*)',
                    re.IGNORECASE)
NEUTRAL = re.compile(r'\b(?:refs?|part of)\b[^#\n]{0,20}#([1-9][0-9]*)', re.IGNORECASE)
NEGATION = re.compile(r"\b(?:not|never|no|n't|without|cannot|neither)\b", re.IGNORECASE)
FENCE = re.compile(r'```.*?```', re.S)
INLINE = re.compile(r'`[^`\n]*`')


def classify(body):
    """Return {firing: {n: line}, negated: {n: line}, inert: {n: line}, neutral: {n: line}}.

    Spans are stripped in one order and it matters: fenced blocks first, because a fence can hold
    backticks, then inline spans. Reversed, a backtick pair inside a fence reads as an inline span
    and the fence's contents leak into the plain-text set.
    """
    out = {"firing": {}, "negated": {}, "inert": {}, "neutral": {}}
    fenced = FENCE.findall(body)
    no_fence = FENCE.sub(" \x00 ", body)
    inline = INLINE.findall(no_fence)
    plain = INLINE.sub(" \x01 ", no_fence)
    # Plain text, line by line, so a negation can be attributed to the line that carries the verb and
    # not to the body as a whole.
    for line in plain.splitlines():
        for n in CLOSER.findall(line):
            if NEGATION.search(line) and n not in out["firing"]:
                out["negated"].setdefault(n, line.strip())
            else:
                out["firing"][n] = line.strip()
                out["negated"].pop(n, None)
    for span in inline + fenced:
        for n in CLOSER.findall(span):
            if n not in out["firing"] and n not in out["negated"]:
                out["inert"].setdefault(n, " ".join(span.split())[:120])
    for n in NEUTRAL.findall(body):
        if n not in out["firing"] and n not in out["negated"] and n not in out["inert"]:
            out["neutral"].setdefault(n, "")
    return out


def report(path, body):
    c = classify(body)
    parts = []
    if c["firing"]:
        parts.append("closes %s" % ", ".join("#" + n for n in sorted(c["firing"], key=int)))
    if c["negated"]:
        parts.append("CLOSES DESPITE THE NEGATION %s" % ", ".join("#" + n for n in sorted(c["negated"], key=int)))
    if c["inert"]:
        parts.append("inert %s" % ", ".join("#" + n for n in sorted(c["inert"], key=int)))
    if c["neutral"]:
        parts.append("neutral %s" % ", ".join("#" + n for n in sorted(c["neutral"], key=int)))
    if not parts:
        print("%s: no issue reference of any kind" % path)
    else:
        print("%s: %s" % (path, "; ".join(parts)))
    for n in sorted(c["negated"], key=int):
        print("    #%s is a keyword in plain text on a line that says not, and it fires anyway: %s"
              % (n, c["negated"][n][:140]))
    for n in sorted(c["inert"], key=int):
        print("    #%s is inside a code span, so nothing will close it: %s" % (n, c["inert"][n][:140]))
    return 1 if c["inert"] or c["negated"] else 0


GOOD = {
    "firing": "Body says it is finished.\n\nCloses #313 — the whole repair.\n",
    "neutral": "Part of #10 only; the umbrella is not finished.\n\nRefs #11.\n",
    "mixed": "Fixes #5 and mentions `Refs #7` in passing.\n",
    "span-safe": "The page models the failing form: `Closes #99`, and quoting it is the point.\n",
}
BAD = {
    "inert": "The keyword is here on purpose. `Closes #301.` — that is all.\n",
    "fenced": "```text\nCloses #42\n```\n",
    "negated": "**This does not close #10.**\n",
    "fence-leak": "```text\n`Refs #3` and then Closes #44\n```\nplain text names `#7`\n",
}


def self_test():
    fails = []

    def expect(name, body, want_class, want_numbers):
        got = classify(body)
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
    expect("inert", BAD["inert"], "inert", ["301"])
    expect("fenced", BAD["fenced"], "inert", ["42"])
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
    if fails:
        for f in fails:
            print("check-close-verb --self-test: %s" % f)
        return 1
    n = len(GOOD) + len(BAD) + 1
    print("check-close-verb: %d fixtures, every class where the parser says it is" % n)
    return 0


def main(argv):
    if len(argv) == 2 and argv[1] == "--self-test":
        return self_test()
    if len(argv) < 2:
        print(__doc__.strip().splitlines()[-3])
        return 2
    rc = 0
    for path in argv[1:]:
        try:
            with open(path, encoding="utf-8") as fh:
                body = fh.read()
        except OSError as exc:
            print("check-close-verb: %s could not be read: %s" % (path, exc))
            return 2
        rc = max(rc, report(path, body))
    if rc == 1:
        print("check-close-verb: a keyword inside a code span closes nothing, and plain text on a "
              "line that says `not` fires anyway. Put the verb where GitHub can see it, or write "
              "`Refs #N` and say what is unfinished.")
    return rc


if __name__ == "__main__":
    sys.exit(main(sys.argv))
