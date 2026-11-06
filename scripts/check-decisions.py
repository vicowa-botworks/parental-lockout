#!/usr/bin/env python3
"""The decision-log gate. Fast enough to belong in `make verify`, and in it for that reason.

Why this file exists. The delivery supervisor runs sessions with no human in the loop, and the deal
that makes that tolerable is written in three parts: take a checkpoint before a judgement call,
record what was decided and what else was on the table, and make the rollback one command. The first
and third parts are git and need no guard. The second is a markdown file, and a markdown file that
nobody checks is how a log becomes a list of conclusions -- which is exactly the thing that is
useless six weeks later when someone asks what the alternative was and why it lost.

So this gate checks the shape of an entry, never its judgement. It cannot tell you a decision was
wrong. It refuses the entries that cannot be argued with later:

  * missing a field -- "Chose: the simple one" is not a record
  * one option -- a decision with no alternative is a description of what happened
  * a `Chose:` that is not one of the listed options, unless it is `escalated`
  * `escalated` without `Awaiting:`, which is a unit that stopped and did not say what it is waiting
    for -- the failure mode where a ticket goes quiet and nobody knows it is blocked on a person
  * a `Checkpoint:` that is not a commit, except on an escalated entry, which changed nothing
  * a reused or out-of-order id, which makes "roll back to before D-0012" ambiguous

An entry is `## D-0007 · title` followed by `- Field: value` lines, and the six field names are the
contract this file and `.opencode/skills/factory/SKILL.md` share.

The self-test is in `make verify` next to this check, and the first assertion is that `verify` still
invokes this script: the cheapest way to pass a gate is to stop calling it, and nothing in the
mutation battery names a file under `scripts/` (all rows checked -- see the note above `verify:` in
the Makefile), so a deleted step here would otherwise stay green forever.

Exit 0 clean · 1 the log has problems · 2 usage.
"""

import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LOGS = (os.path.join(ROOT, "DECISIONS.md"), os.path.join(ROOT, "factory/DECISIONS.md"))
LOG = LOGS[0]
MAKEFILE = os.path.join(ROOT, "Makefile")

USAGE = """usage: check-decisions.py [--self-test] [--log PATH]

Checks every entry in DECISIONS.md has the fields that make it arguable later, and that the log is
still being invoked by `make verify`. Takes no positional arguments: a filename in a shell variable
that expanded to nothing is a silent change of subject, which is what `--log` is for."""

FIELDS = ("Question", "Options", "Chose", "Why", "Checkpoint")
ENTRY = re.compile(r"^## ([DH]-(\d{4}))\s*(?:·|\||-)\s*(.+?)\s*$")
FIELD = re.compile(r"^\s*[-*]\s*([A-Za-z][A-Za-z ]*):\s*(.*)$")
SHA = re.compile(r"^[0-9a-f]{7,40}$")
ID_NUM = re.compile(r"[DH]-(\d{4})$")


def parse(text, label="<log>"):
    """Every entry in the log, as (id, number, title, {field: value}, line-no).

    A field continues onto the next line, and only onto the next line. This repository's prose is
    hand-wrapped near 88 columns and so is a real `Options:` line, and a field parser that reads one
    line and stops does not fail on a wrapped entry -- it silently stores half of it. That is the
    worst shape a gate can have: the check then compares a `Chose:` against a truncated option list
    and reports the *entry* as the problem. Measured on the first run of this script against the log
    it ships with, four of five entries came back as "chose something not among the options".

    So a continuation is a more-indented line with no field name on it, and only while the previous
    line was that field or another continuation. A blank line closes it, which is what keeps an
    entry's own prose paragraph from being swallowed into the last field.
    """
    entries = []
    current = None
    pending = None
    for n, line in enumerate(text.splitlines(), 1):
        m = ENTRY.match(line)
        if m:
            if current is not None:
                entries.append(current)
            current = {"id": m.group(1), "num": int(m.group(2)), "title": m.group(3),
                       "fields": {}, "line": n, "label": label}
            pending = None
            continue
        if current is None:
            continue
        if not line.strip():
            pending = None
            continue
        fm = FIELD.match(line)
        if fm:
            name = fm.group(1).strip()
            value = fm.group(2).strip()
            if name in FIELDS + ("Awaiting", "Effect"):
                if name in current["fields"]:
                    current["fields"].setdefault("_dup", []).append(name)
                current["fields"][name] = value
                pending = name
            else:
                pending = None
            continue
        if pending is not None and line[:1].isspace():
            current["fields"][pending] = "%s %s" % (current["fields"][pending], line.strip())
            continue
        pending = None
    if current is not None:
        entries.append(current)
    return entries


def check(entries):
    """Problems as strings. Empty means the log is usable as a record."""
    problems = []
    seen = {}
    prev_num = 0

    if not entries:
        return ["no entry found: the file exists and nothing in it matched `## D-0001 · …`, which "
                "is what a log that has stopped being written looks like"]

    for e in entries:
        at = "%s:%d" % (e["label"], e["line"])
        if e["id"] in seen:
            problems.append("%s: id %s is already used at %s:%d; 'revert to before %s' would name "
                            "two commits" % (at, e["id"], e["label"], seen[e["id"]], e["id"]))
        seen[e["id"]] = e["line"]

        if e["num"] < prev_num:
            problems.append("%s: %s is out of order (%s came before it); the log is a history and a "
                            "history that jumps around cannot be bisected"
                            % (at, e["id"], "D-%04d" % prev_num))
        prev_num = max(prev_num, e["num"])

        f = e["fields"]
        missing = [k for k in FIELDS if not f.get(k)]
        if missing:
            problems.append("%s: %s is missing %s -- an entry without these is a conclusion, not a "
                            "record" % (at, e["id"], ", ".join(missing)))

        for name, value in f.items():
            if name.startswith("_"):
                continue
            if not value:
                problems.append("%s: %s has an empty `%s:` field" % (at, e["id"], name))
        if f.get("_dup"):
            problems.append("%s: %s repeats %s; the last one wins and the first one is silently the "
                            "one nobody reads" % (at, e["id"], ", ".join(sorted(set(f["_dup"])))))

        options = [o.strip() for o in re.split(r"\s*\|\s*", f.get("Options", "")) if o.strip()]
        if f.get("Options") and len(options) < 2:
            problems.append("%s: %s lists one option -- a decision with no alternative on the record "
                            "is a description of what happened, not a decision" % (at, e["id"]))

        chose = f.get("Chose", "").strip()
        escalated = chose.lower().startswith("escalat")
        if chose and not escalated and options and chose not in options:
            problems.append("%s: %s chose %r, which is not among the recorded options (%s) -- either "
                            "the option was not written down or the entry was edited afterwards"
                            % (at, e["id"], chose, "; ".join(options)))
        if escalated:
            if not f.get("Awaiting"):
                problems.append("%s: %s escalated without `Awaiting:` -- a unit that stopped and did "
                                "not say what it is waiting for is indistinguishable from one that "
                                "died" % (at, e["id"]))
        elif chose and not escalated and f.get("Awaiting", "").strip() not in ("", "-"):
            problems.append("%s: %s has `Awaiting:` but did not escalate; delete the field or set "
                            "`Chose: escalated`" % (at, e["id"]))

        cp = f.get("Checkpoint", "").strip()
        if cp and cp != "n/a" and not SHA.match(cp):
            problems.append("%s: %s checkpoint %r is not a commit sha (7-40 hex) or `n/a`, so it is "
                            "not something `git revert` can be pointed at" % (at, e["id"], cp))
        if cp == "n/a" and not escalated:
            problems.append("%s: %s has no checkpoint and did not escalate; the checkpoint is the "
                            "rollback target and it is taken before the decision, never after"
                            % (at, e["id"]))
    return problems


# The two `verify` recipe lines this file insists on, in the Makefile's own shape: `echo "..."` and
# the command share a line, so the guard below reads command chunks rather than whole lines.
RECIPE_GOOD = ('\t@set -e; \\\n'
               '\techo "== decision-log contract"; python3 scripts/check-decisions.py --self-test; \\\n'
               '\techo "== decision log"; python3 scripts/check-decisions.py; \\\n'
               '\techo "== shellcheck"; $(MAKE) --no-print-directory check-shell; \\\n')

# Three edits that all delete the gate. Each is a `make verify` that runs nothing and reports success,
# and each keeps the string "check-decisions.py" somewhere in the file, which is what a substring
# guard reads. Reported by review on #217 (5 Nov) as a guard satisfied by the text that removes it; the
# shapes here are the ones a person reaches for -- comment a step out, rename the file it calls, or
# replace the step with one that only mentions it.
RECIPE_BAD = {
    "commented": ('\t@set -e; \\\n'
                  '\techo "== decision-log contract"; \n'
                  '#\tpython3 scripts/check-decisions.py --self-test; \\\n'
                  '#\tpython3 scripts/check-decisions.py; \\\n'),
    "renamed": RECIPE_GOOD.replace("check-decisions.py", "check-decisions-OLD.py"),
    "mentioned": ('\t@set -e; \\\n'
                  '\techo "== decision log (check-decisions.py)"; $(MAKE) --no-print-directory quiet; \\\n'),
}


def invocation_problems(recipe):
    """Given the text of a Makefile, say whether `verify` still runs this script and its self-test.

    A filename substring is the wrong assertion: it is satisfied by a comment, by a commented-out
    recipe line, and by an `echo` that names the script while running something else. So the anchor is
    a *command chunk* -- the text between `;` on a recipe line -- that starts with the interpreter and
    names `scripts/check-decisions.py` as one whole word. A chunk beginning with `#` is a comment and
    cannot match, and the word boundary bars a longer filename that merely starts the same way. The
    three shapes in `RECIPE_BAD` are asserted against this function in `--self-test`, so a later edit
    that weakens the match has to delete those cases as well.
    """
    problems = []
    tails = []
    for raw in recipe.splitlines():
        for chunk in raw.split(";"):
            chunk = chunk.strip()
            if chunk.startswith("#"):
                continue
            found = re.match(r"python3[ \t]+scripts/check-decisions\.py(?![^\s])(.*)$", chunk)
            if found:
                tails.append(found.group(1))
    if not tails:
        problems.append("Makefile: no recipe command runs `python3 scripts/check-decisions.py`; a gate "
                        "whose invocation has been deleted, commented out, or renamed is a suggestion")
        return problems
    if not any("--self-test" in tail for tail in tails):
        problems.append("Makefile: nothing runs `python3 scripts/check-decisions.py --self-test`; the "
                        "assertions below then run only when someone remembers they exist")
    return problems


def verify_invocation():
    """That `make verify` still calls this script, and calls its self-test.

    Reading the Makefile rather than trusting memory: the failure this guards is the deleted line,
    and the deleted line is exactly what a later edit that "simplifies the gate" produces.
    """
    try:
        with open(MAKEFILE, encoding="utf-8") as fh:
            recipe = fh.read()
    except OSError as exc:
        return ["Makefile: cannot be read (%s), so nothing can be claimed about the gate" % exc]
    return invocation_problems(recipe)


def read(path):
    try:
        with open(path, encoding="utf-8") as fh:
            return fh.read(), None
    except OSError as exc:
        return None, "%s cannot be read (%s); the log either does not exist or cannot be opened, " \
                     "and a run that reads nothing must not report that as clean" % (path, exc)


GOOD = """
## D-0001 · Keep the harness in the repository

- Question: where does the delivery loop live?
- Options: in this repository | in the shared skills repository
- Chose: in this repository
- Why: the loop is only trustworthy where its own gate runs; a shared skill cannot pin this
  repository's `make verify`.
- Checkpoint: 2f4c1ab9
- Effect: factory/factory.sh

## D-0002 · Dependency question

- Question: may a tick add a third-party module?
- Options: yes, if it is justified | escalated to a maintainer
- Chose: escalated to a maintainer
- Why: the zero-dependency rule is the user's decision, recorded as a project rule with a gate.
- Checkpoint: n/a
- Awaiting: a maintainer answer on the issue
"""

WRAPPED = """## D-0001 · A wrapped entry

- Question: does a field continue onto the next line, when the prose is hand-wrapped the way this
  repository wraps it?
- Options: fold the continuation into the field | read one line per field and stop
- Chose: fold the continuation into the field
- Why: a parser that stops at the newline does not fail on a wrapped entry, it stores half of it and
  then blames the entry.
- Checkpoint: 2f4c1ab9
"""

PROSE = WRAPPED + "\nA paragraph after a blank line, which is not part of `Checkpoint:`.\n"

BAD = {
    "no-entries": "# Decisions\n\nNothing here yet.\n",
    "missing-field": "## D-0001 · X\n\n- Question: q?\n- Chose: a\n",
    "one-option": "## D-0001 · X\n\n- Question: q?\n- Options: only this\n- Chose: only this\n"
                  "- Why: because.\n- Checkpoint: abc1234\n",
    "chose-unknown": "## D-0001 · X\n\n- Question: q?\n- Options: a | b\n- Chose: c\n"
                     "- Why: because.\n- Checkpoint: abc1234\n",
    "escalated-no-await": "## D-0001 · X\n\n- Question: q?\n- Options: a | escalated to a maintainer"
                          "\n- Chose: escalated to a maintainer\n- Why: not mine.\n- Checkpoint: n/a\n",
    "bad-sha": "## D-0001 · X\n\n- Question: q?\n- Options: a | b\n- Chose: a\n"
               "- Why: because.\n- Checkpoint: HEAD~1\n",
    "checkpoint-na": "## D-0001 · X\n\n- Question: q?\n- Options: a | b\n- Chose: b\n"
                     "- Why: because.\n- Checkpoint: n/a\n",
    "duplicate": GOOD + "\n## D-0001 · Again\n\n- Question: q?\n- Options: a | b\n- Chose: a\n"
                        "- Why: because.\n- Checkpoint: abc1234\n",
    "out-of-order": "## D-0009 · Later\n\n- Question: q?\n- Options: a | b\n- Chose: a\n"
                    "- Why: because.\n- Checkpoint: abc1234\n" + GOOD,
}


def self_test():
    """Every rule above has a case that trips it, and the good sample trips none of them.

    The cases are in-memory rather than in fixtures for the same reason `mutate.py --self-test`
    keeps its rows inline: a fixture a person can edit is a fixture that quietly stops asserting.
    """
    fails = []
    counts = [0, 0]

    def expect_ok(text, name):
        counts[1] += 1
        problems = check(parse(text, name))
        if problems:
            fails.append("%s: expected clean, got %s" % (name, problems))

    def expect_fail(text, name, needle):
        counts[0] += 1
        problems = check(parse(text, name))
        if not problems:
            fails.append("%s: expected a problem, got none -- the rule is not being applied" % name)
        elif not any(needle in p for p in problems):
            fails.append("%s: expected a problem containing %r, got %s" % (name, needle, problems))

    expect_ok(GOOD, "good")
    expect_ok(WRAPPED, "wrapped")

    # A continuation is a *more-indented line with no field name*, and it ends at a blank line. If it
    # did not, the paragraph after an entry would be folded into `Checkpoint:` and the sha test below
    # would be reporting the entry's own prose as a malformed commit.
    counts[1] += 1
    folded = parse(PROSE, "prose")
    tail = folded[-1]["fields"].get("Checkpoint", "") if folded else ""
    if tail != "2f4c1ab9":
        fails.append("prose-after-blank-line: `Checkpoint:` reads %r, so the paragraph after the "
                     "entry was folded into it" % tail)

    expect_fail(BAD["no-entries"], "no-entries", "no entry found")
    expect_fail(BAD["missing-field"], "missing-field", "missing Options")
    expect_fail(BAD["one-option"], "one-option", "one option")
    expect_fail(BAD["chose-unknown"], "chose-unknown", "not among the recorded options")
    expect_fail(BAD["escalated-no-await"], "escalated-no-await", "without `Awaiting:`")
    expect_fail(BAD["bad-sha"], "bad-sha", "not a commit sha")
    expect_fail(BAD["checkpoint-na"], "checkpoint-na", "no checkpoint")
    expect_fail(BAD["duplicate"], "duplicate", "already used")
    expect_fail(BAD["out-of-order"], "out-of-order", "out of order")

    # The self-pinning pair: this file's own invocations, asserted the way the screenshot manifest
    # asserts its own. Deleting either line from `verify` makes this run refuse.
    counts[1] += 2
    invocation = verify_invocation()
    if any("no recipe command runs" in p for p in invocation):
        fails.append("verify-invocation: `make verify` no longer calls check-decisions.py -- %s"
                     % invocation)
    if any("nothing runs" in p for p in invocation):
        fails.append("verify-invocation: nothing runs `check-decisions.py --self-test` -- %s"
                     % invocation)

    # And the guard on the guard. Asserting only against the live Makefile proves the Makefile is
    # fine today; it says nothing about whether this function would notice it stop being fine, which
    # is the whole job. Each case below is a Makefile in which `make verify` runs no decision gate at
    # all while still containing the words "check-decisions.py", so a match weakened back to a
    # substring turns one of these into a pass and this run refuses.
    counts[1] += 1
    if invocation_problems(RECIPE_GOOD):
        fails.append("recipe-good: the Makefile's own shape is refused by the guard -- %s"
                     % invocation_problems(RECIPE_GOOD))
    for name, recipe in sorted(RECIPE_BAD.items()):
        counts[0] += 1
        found = invocation_problems(recipe)
        if not found:
            fails.append("recipe-%s: a Makefile that runs no decision gate passed the guard; a "
                         "substring is not an invocation" % name)
        elif not any("no recipe command runs" in p for p in found):
            # The reason is asserted, not just the refusal. All three Makefiles lack the *command*,
            # so "nothing runs --self-test" as the only complaint would mean the guard matched
            # something in a file that runs nothing -- which is the substring behaviour this case
            # exists to catch.
            fails.append("recipe-%s: refused for the wrong reason -- %s" % (name, found))

    if fails:
        print("check-decisions --self-test FAILED:")
        for f in fails:
            print("  %s" % f)
        return 1
    print("check-decisions --self-test: %d cases that must fail, %d that must pass, the two "
          "invocations in `verify`, and three Makefiles that delete this gate" % (counts[0], counts[1]))
    return 0


def main(argv):
    if len(argv) > 1 and argv[1] in ("-h", "--help"):
        print(USAGE)
        return 0
    if len(argv) > 1 and argv[1] == "--self-test":
        return self_test()

    path = LOG
    if len(argv) > 2 and argv[1] == "--log":
        if len(argv) < 3 or not argv[2]:
            print("check-decisions: --log wants a path; an empty one is a change of subject, not a "
                  "default", file=sys.stderr)
            return 2
        path = argv[2]
    elif len(argv) > 1:
        print("check-decisions: unknown argument %r\n%s" % (argv[1], USAGE), file=sys.stderr)
        return 2

    paths = [path] if path != LOG else list(LOGS)
    problems = []
    total = 0
    where_seen = {}
    for p in paths:
        text, problem = read(p)
        if problem:
            print(problem, file=sys.stderr)
            return 1
        label = os.path.relpath(p, ROOT)
        entries = parse(text, label)
        total += len(entries)
        problems += check(entries)
        if path == LOG:
            for e in entries:
                where_seen.setdefault(e["id"], []).append(label)

    if path == LOG:
        problems += verify_invocation()
        # One id in two logs is not a duplicate to dedupe: it is a citation that points at two
        # decisions. Measured 6 Nov, when merging origin/main into the harness branch: `main` carried
        # D-0011..D-0015 as product decisions (a control's typeface, `:has()` reaching a tick box's
        # words) while the harness log had D-0011..D-0015 as harness decisions (the tick's `$PWD`, the
        # worktree cut, the deny keying), because a tick numbers from the highest id it can *see*, and
        # a tree cut before this file existed sees none of it. Both files now have their own id space,
        # so the only thing left to enforce is that they do not meet.
        for ident in sorted(where_seen):
            if len(where_seen[ident]) > 1:
                problems.append(
                    "%s is used in %s -- one id, %d decisions; a citation to it does not resolve. "
                    "Renumber the later entry in its own log." %
                    (ident, " and ".join(sorted(where_seen[ident])), len(where_seen[ident])))

    if problems:
        print("\n%d decision-log problem(s):" % len(problems))
        for p in problems:
            print("  %s" % p)
        return 1

    print("check-decisions: %d entries across %d log(s), every one with its question, options, "
          "choice, reason and checkpoint" % (total, len(paths)))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
