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
  * a `path:line` citation whose quoted anchor is not on the line it names, a new citation with no
    anchor at all, and a pointer whose name belongs to more than one file

An entry is `## D-0007 · title` followed by `- Field: value` lines, and the six field names are the
contract this file and `.opencode/skills/factory/SKILL.md` share.

The citation rule exists because a coordinate is a promise that a reader can go and look, and a merge
can take the ground out from under it. Merging `origin/main` into `issue-310` moved `AGENTS.md:38` from
the escalation instruction to a checkpoint bullet without editing the entry that cited it, and that
false pointer survived a month because every rule here was about the entry's shape. It reads the tree,
not the history, on purpose: `factory/selftest.sh`'s `gate_reads_no_refs` leg refuses this file any git
subcommand but `cat-file`, because a verdict that changes between a branch head and a merge commit
reads as flaky and gets re-run until it agrees. See the block above `POINTER_DEBT` for what that
leaves checkable and what it leaves counted instead.

The self-test is in `make verify` next to this check, and the first assertion is that `verify` still
invokes this script: the cheapest way to pass a gate is to stop calling it, and nothing in the
mutation battery names a file under `scripts/` (all rows checked -- see the note above `verify:` in
the Makefile), so a deleted step here would otherwise stay green forever.

Exit 0 clean · 1 the log has problems · 2 usage.
"""

import collections
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LOGS = (os.path.join(ROOT, "DECISIONS.md"), os.path.join(ROOT, "factory/DECISIONS.md"))
LOG = LOGS[0]
MAKEFILE = os.path.join(ROOT, "Makefile")

USAGE = """usage: check-decisions.py [--self-test] [--log PATH] [--citations]

Checks every entry in DECISIONS.md has the fields that make it arguable later, and that the log is
still being invoked by `make verify`. Takes no positional arguments: a filename in a shell variable
that expanded to nothing is a silent change of subject, which is what `--log` is for.

--log PATH checks that one file: the same entry rules, the same checkpoint
existence rule, and the Makefile invocation rule whenever this checkout has a
Makefile. Narrowing the file is not narrowing the rules -- the old wording
promised checkpoint existence here while the code skipped it, which is how a
fabricated checkpoint survived four passes of exactly this invocation. A `--log` run over one file still
asks the duplicate-id rule, but against that file alone -- that is the only thing about `--log` that
narrows. It also reports `path:line` citations for that file but does not judge them against
POINTER_DEBT, which is a count over the pair; the closing line says which of the two it did.

--citations prints what the citation rule found instead of refusing on it: the unanchored
count by the file pointed at, the citations a reader cannot open, and the anchored ones whose
text has moved. It reads and changes nothing, and it is how the debt number is re-derived.

--citations PATH lists every citation pointing at that path instead, with the log line that cites
it. Ask before editing a file that other people's decisions quote: a merge that moves a line has to
fix those pointers in the same commit, and nothing else in this repository can tell you which they
are. `--citations AGENTS.md` is the ordinary first call.

A citation is `path:NNN` in backticks. It is judged against the tree being gated -- no git, no
revision, the same answer in CI as at the merge -- and it is checked when it carries an anchor: a
quoted fragment of at least 16 characters with nothing between it and the closing backtick.

    `AGENTS.md:43` "write the entry with `Chose: escalated` and `Awaiting:` naming what is owed"

Then the fragment has to be on line 43 of AGENTS.md. No window: a pointer wrong by three lines still
lands on the wrong sentence, which is the bug #318 was written about. A citation with no anchor cannot
be checked, so it is counted, and the count is recorded in this file as POINTER_DEBT. That number may
only fall: a new unanchored citation is refused, and so is a run that measured below it, because an
unlowered number hides the next broken pointer in the slack.
"""

FIELDS = ("Question", "Options", "Chose", "Why", "Checkpoint")
ENTRY = re.compile(r"^## ([DH]-(\d{4}))\s*(?:·|\||-)\s*(.+?)\s*$")
FIELD = re.compile(r"^\s*[-*]\s*([A-Za-z][A-Za-z ]*):\s*(.*)$")
SHA = re.compile(r"^[0-9a-f]{7,40}$")
ID_NUM = re.compile(r"[DH]-(\d{4})$")
# Two shapes an id takes inside a refusal's own prose. A *citation* names an entry (`D-0082 came
# before it`), an example shows a heading to write -- always printed between backticks, and always read
# here exactly as printed (`## H-0001 · …`). They owe the reader different things: a citation has to
# resolve in the log being read, an example has to be a heading `ENTRY` accepts. Telling them apart is
# what lets one rule hold both messages to their obligation, and taking the example as printed is what
# gives the second half teeth -- a check that rebuilds the heading from the id it extracted would be
# matching its own string and could never fail.
ID_TOKEN = re.compile(r"\b[DH]-\d{4}\b")
EXAMPLE_HEADING = re.compile(r"`(#+\s*[DH]-\d+[^`]*)`")


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
    prev_id = None

    if not entries:
        # Both id spaces are offered because this arm cannot know which one the file writes: it
        # returns before a single entry has been parsed, `--log` accepts any path, and `parse` imposes
        # no prefix on it -- both logs match the same `ENTRY`. Guessing one from the path or from a
        # caller's label is the assumption H-0091 refused to make about the out-of-order predecessor's
        # prefix, and measured there it is wrong for a file holding both spaces. So the example names
        # what the grammar accepts: a lone `D-` told whoever pointed this at an empty harness log to
        # grep for a heading that file can never hold (#268).
        return ["no entry found: the file exists and nothing in it matched `## D-0001 · …` or "
                "`## H-0001 · …`, which is what a log that has stopped being written looks like"]

    for e in entries:
        at = "%s:%d" % (e["label"], e["line"])
        if e["id"] in seen:
            problems.append("%s: id %s is already used at %s:%d; 'revert to before %s' would name "
                            "two commits" % (at, e["id"], e["label"], seen[e["id"]], e["id"]))
        seen[e["id"]] = e["line"]

        # The predecessor is printed exactly as it was parsed, never rebuilt from its number. This
        # formatter used to write `"D-%04d" % prev_num`: the number comes from whichever log is being
        # read (`e["num"]`), so re-attaching a `D-` means the harness log can only ever be told the
        # wrong name. Measured 10 Nov in /home/rodney/prj/wt/issue-263 while resolving PR #264's
        # `factory/DECISIONS.md` conflict -- both sides appended at the same offset, so `H-0079`
        # landed after `H-0082` and the refusal read:
        #   factory/DECISIONS.md:2159: H-0079 is out of order (D-0082 came before it)
        # `D-0082` exists in neither log (`DECISIONS.md` carries 39 entries and stops at `D-0052`), so
        # the sentence a person reads while the gate is red sends them looking for an entry that is
        # not there. Number and id now move together and cannot name two different decisions.
        if e["num"] < prev_num:
            problems.append("%s: %s is out of order (%s came before it); the log is a history and a "
                            "history that jumps around cannot be bisected"
                            % (at, e["id"], prev_id))
        if e["num"] > prev_num:
            prev_num, prev_id = e["num"], e["id"]

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
    # The one that survived the guard until 6 Nov: the whole step commented out at column 0, with the
    # invocation sitting after a `;` on the same line. `make verify` here runs nothing, and the file
    # still contains the command as plain text for any guard that reads lines instead of chunks.
    "commented-line": ('\t@set -e; \\\n'
                       '#\techo "== decision log"; python3 scripts/check-decisions.py --self-test; \\\n'
                       '#\techo "== decision log again"; python3 scripts/check-decisions.py; \\\n'
                       '\techo "== shellcheck"; $(MAKE) --no-print-directory check-shell; \\\n'),
}


def invocation_problems(recipe):
    """Given the text of a Makefile, say whether `verify` still runs this script and its self-test.

    A filename substring is the wrong assertion: it is satisfied by a comment, by a commented-out
    recipe line, and by an `echo` that names the script while running something else. So the anchor is
    a *command chunk* -- the text between `;` on a recipe line -- that starts with the interpreter and
    names `scripts/check-decisions.py` as one whole word, and the word boundary bars a longer filename
    that merely starts the same way. The shapes in `RECIPE_BAD` are asserted against this function in
    `--self-test`, so a later edit that weakens the match has to delete those cases as well.

    A comment is tested at the *line*, before the split, not at the chunk. Reported by review on #217
    at 869564a: with the step commented as `#\techo "== decision log"; python3
    scripts/check-decisions.py; ` followed by a line-continuation backslash, make hands that line
    to no shell at all, but splitting on `;` first
    put the invocation in chunk two where a `chunk.startswith("#")` test could not see it -- so both
    this gate and its own self-test returned 0 while `make verify` ran neither command. The premise the
    docstring used to carry, that a chunk beginning with `#` cannot match, is what the shell refuted.
    """
    problems = []
    tails = []
    for raw in recipe.splitlines():
        if raw.lstrip().startswith("#"):
            # make never hands this line to a shell; nothing on it runs, including whatever follows a `;`.
            continue
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


def read_tree(rel):
    """A cited file, as the tree being gated holds it. None is "absent or unreadable".

    Relative to this checkout, because that is what a citation means: `triage.sh:122` is a promise
    about the file in front of the reader, not about a revision they would have to name. Read fresh
    from disk rather than from git, so a file a unit has edited but not committed is the file the
    citations are judged against -- the failure this rule exists to catch is a merge moving a line,
    and the unit that moves it has to be told before the commit, not after the merge.
    """
    try:
        with open(os.path.join(ROOT, rel.replace("/", os.sep)), encoding="utf-8",
                  errors="replace") as fh:
            return fh.read()
    except OSError:
        return None


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
    # The same arm reached from the other log. This fixture is the whole reason the bug shipped: every
    # case in `BAD` is a `D-` log, where the single `D-` example in the empty-log refusal happens to be
    # right, and the needle was the phrase "no entry found" -- printed whichever example the sentence
    # carries, so the arm looked covered while its only id was wrong for half its readers (#268).
    "no-entries-harness": "# Harness decisions\n\nNothing here yet.\n",
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
    # The same arm reached from the other id space, which is the case that did not exist and so the
    # bug that survived: every other fixture here is a `D-` log, where a hardcoded `D-` in the
    # message happens to be right. `GOOD.replace("D-", "H-")` keeps the two fixtures identical apart
    # from the prefix -- the only thing that differs -- and the numbers are the measured pair from
    # PR #264's conflict (`H-0079` after `H-0082`).
    "out-of-order-harness": "## H-0082 · Later\n\n- Question: q?\n- Options: a | b\n- Chose: a\n"
                            "- Why: because.\n- Checkpoint: abc1234\n" + GOOD.replace("D-", "H-"),
    # One file holding both id spaces. Reachable by design: `--log` takes any path and `parse` imposes
    # no prefix on a file (both logs match the same `ENTRY`), and a merge that took both sides into one
    # file produces exactly this. It is the fixture that separates the two candidate fixes -- printing
    # the predecessor as parsed names `D-0082`, which is what is really above the entry, while
    # re-deriving the prefix from the offending entry names `H-0082`, an id in neither log. Measured
    # 10 Nov: with the re-derived prefix the two fixtures above both still pass, so without this row
    # that variant is green and one edit away.
    "out-of-order-mixed": "## D-0082 · Later\n\n- Question: q?\n- Options: a | b\n- Chose: a\n"
                          "- Why: because.\n- Checkpoint: abc1234\n"
                          "## H-0079 · Earlier\n\n- Question: q?\n- Options: a | b\n- Chose: a\n"
                          "- Why: because.\n- Checkpoint: abc1234\n",
}


def git_checkpoint_resolver(shas):
    """Which of these objects exist in this repository, and are commits. One batch call, not one per entry.

    The gate used to stop at "is this 7-40 hex characters", which is a shape test, not an existence test:
    written minutes after H-0021..H-0023 were committed, this session typed an invented 40-hex checkpoint
    into three entries and the checker reported 45 entries clean. A checkpoint is the one field whose whole
    job is to be typed into `git revert`, so a plausible one that resolves to nothing is worse than a
    missing one -- it reads like a rollback exists.
    """
    if not shas:
        return {}
    import subprocess
    try:
        out = subprocess.run(["git", "cat-file", "--batch-check=%(objectname) %(objecttype)"],
                             input="\n".join(shas) + "\n", capture_output=True, text=True,
                             encoding="utf-8", errors="replace", cwd=ROOT, timeout=30)
    except (OSError, subprocess.SubprocessError):
        # None means "could not ask", and it has to be distinguishable from {}. This used to return
        # an empty mapping, which `checkpoint_problems` read with `resolve.get(cp, True)` -- so no
        # git on PATH, or a wedged `cat-file` past its timeout, produced a clean gate in which the
        # existence rule was never asked. Found on #217 round four by the same reflex that found the
        # rest of them: ask what the checker does when it cannot do it. A refusal is already correct
        # when git is present and the tree is not a repository (a `.git`-less copy produced 49
        # refusals), so the only hole left was the one where nothing answered at all.
        return None
    # One line per input, in order: `<sha> commit` for a commit, `<obj> missing` for anything else.
    lines = out.stdout.splitlines()
    return {s: (i < len(lines) and lines[i].endswith(" commit")) for i, s in enumerate(shas)}


def checkpoint_problems(entries, resolve):
    """That every named checkpoint is a commit the resolver can see.

    `resolve` is injected so `--self-test` can assert both answers: the same sample must refuse when told
    the object is absent and pass when told it is present. It may be a mapping or a one-argument callable. Without that the rule would be a git call
    nobody can prove still fires -- the failure mode this script keeps being written about.
    """
    problems = []
    named = [e for e in entries
             if e["fields"].get("Checkpoint", "").strip() not in ("", "n/a")
             and SHA.match(e["fields"].get("Checkpoint", "").strip())]
    if not named:
        # Nothing to ask, so nothing to claim either way. `main` only builds a resolver when some
        # entry names a checkpoint, so this is the self-test's artificial case -- and it is the
        # boundary of the rule below: a log with no checkpoints must not be refused for a git
        # problem it does not have.
        return []
    if resolve is None:
        # The resolver could not ask git. Worded like `verify_invocation`'s unreadable-Makefile case,
        # because it is the same claim being withheld: an unevaluated rule is not a passed rule.
        return ["%s: git could not be asked whether the named checkpoints exist, so nothing can be "
                "claimed about them -- the existence rule did not run. (A missing git, a wedged "
                "`git cat-file`, or a tree this is not a repository of: fix that before trusting a "
                "clean run of this gate.)" % label_of(entries[0])]
    for e in entries:
        cp = e["fields"].get("Checkpoint", "").strip()
        if not cp or cp == "n/a" or not SHA.match(cp):
            continue
        # The default used to be True -- "the resolver did not mention it, so it must exist" -- which
        # is the same fail-open one level down: a mapping that does not cover a sha is an unanswered
        # question, not a voucher. `main` builds the mapping over every checkpoint it found, so an
        # uncovered one means the entry appeared after the batch, and the honest answer is no.
        known = resolve(cp) if callable(resolve) else resolve.get(cp, False)
        if not known:
            problems.append("%s: %s names checkpoint %s, which is not a commit in this repository -- "
                            "`git revert` cannot be pointed at it, so the entry is not reversible the way "
                            "it claims" % (label_of(e), e["id"], cp))
    return problems


def label_of(e):
    return "%s:%d" % (e["label"], e["line"])


# ---------------------------------------------------------------------------------------------
# `path:NNN` citations (issue #318)
# ---------------------------------------------------------------------------------------------
#
# Why this is here. A decision-log entry is the only reason an unattended judgement call is
# reversible, and almost every entry leans on a coordinate to say what it measured: `triage.sh:122`,
# `pages.py:368`, `AGENTS.md:38`. A coordinate is a promise that a reader can go and look. Merging
# `origin/main` into `issue-310` broke two of them without touching a word of the prose that made
# them, because the other side of the merge added lines to the files they point at. `H-0139` cited
# `AGENTS.md:38` for the escalation instruction; on the merged tree line 38 is a checkpoint bullet.
# Nothing in `make verify` could see it, and the log carried a false pointer for a month before
# somebody followed it by accident. The census that found this produced a SECOND finding, and the
# second one was wrong: it reported that `factory/DECISIONS.md:3178` and `:3259` -- cited at
# `39dd88e54:factory/DECISIONS.md:3497`, where the entry says they "all resolve" -- were false the day
# they were written, because the file at that revision holds 2621 lines. It was matching a
# fully-qualified citation by BASENAME and taking the first candidate, `DECISIONS.md`, which is a
# different file with 2621 lines. The factory log holds 3531 lines there, both coordinates land on the
# tally sentence and the `reviewRequests` Effect they name, and they still land today. So the audit was
# the only false thing in the room -- and it produced that finding inside the session sent to fix false
# findings. Which is why resolution is exact-path first, a basename second, and refused when two files
# answer: a wrong path is not an unreadable one, it is a fabricated verdict.
#
# What this rule is NOT, and why. The obvious design resolves each citation at the revision that
# wrote it, which is where the entry's measurement actually lives. It needs `git blame`, and `blame`
# is a walk down `HEAD`, so the verdict would differ between the branch head CI checks and the merge
# commit the supervisor merges, and again in a shallow clone -- `factory/selftest.sh`'s
# `gate_reads_no_refs` leg exists to refuse exactly that (H-0056, #253): a verdict that changes
# between two identical checkouts reads as flaky and gets re-run until it agrees. So this rule asks
# git nothing at all -- not `blame`, not `ls-files`, not a rev -- and reads only the two logs and the
# files they cite, in the tree being gated. `gate_reads_no_refs` witnesses that from the outside on
# every `make verify`.
#
# What it IS: the citation carries its own anchor, and the anchor is checked against the tree.
#
#     `AGENTS.md:43` "write the entry with `Chose: escalated` and `Awaiting:` naming what is owed"
#
# A citation followed by a quoted fragment of at least 16 characters must find that fragment inside
# the cited lines -- no window, no fuzzy match: a pointer that is right by three lines still lands
# on the wrong sentence, which is the bug. A citation with no anchor cannot be checked at all, so it
# is counted instead of judged, and the count is `POINTER_DEBT` below. That number may only fall.
# It is the grandfathering that option 2 of #318 needs and the thing a silent exemption would not
# be: a merge that moves an unanchored pointer leaves the count alone, while a unit that writes one
# new unanchored citation, or one anchored citation whose sentence is not there, is refused.
#
# The path half is measured, not assumed. `pr:209`, `unclaimed:263` and `address_review:240` appear
# in these logs and are not citations -- they are a triage `class:PR` pair, and `STATE_PROPS:1040`
# is a symbol in a file named earlier in the sentence. Refusing every `word:digit` span would have
# been 15 false refusals in the first run. So a span is a citation only when its path half names a
# file that exists in this tree, which is also what makes the `.git` exclusion load-bearing: it is
# done by NAME below, because `"/.git" in "./.github/workflows"` is a true statement and the first
# census of #318 used it and reported `ci.yml:62` as a dead citation.

# The citations a reader cannot follow in this tree: the file is absent, the line is past it, the
# name belongs to more than one file, or there is no anchor to check. `scripts/check-decisions.py
# --citations` prints them grouped, `--citations PATH` answers it for one file, and a run whose count is
# above this number is refused; a run below it is refused too, because an unlowered number is a ratchet
# with the pawl filed off -- the next broken pointer hides inside the slack.
#
# 535, re-measured on 14 Nov 2026 on the SECOND merge of `origin/main` into `issue-318` (`8437eea`, 22
# commits) by `python3 scripts/check-decisions.py --citations | tail -1`. 517 + 18: every one of the 18 is
# inside `H-0163`..`H-0169`, the five harness entries `main` published while #367 was in review, and none
# of them was written by this unit. The merge also moved four anchored pointers in this branch's own
# `H-0162` -- `factory/selftest.sh:11568`->`:11823`, `:11577`->`:11832`, `docs/factory.md:1737`->`:1904`,
# `:1773`->`:1940` -- and the anchor rule named all four with their new lines, which is the first time
# this rule caught its own author across a merge. The previous measurement was 517 on 13 Nov, AFTER the
# 54 ambiguous pointers were given their full paths -- 37 on this branch's side, and 17 that arrived
# with `main` while this was in review (10 of them a bare `README.md`, which five files here answer to).
# All 535 were written before this rule existed in the tree -- the rule is only on this branch -- and 4 of
# them are the paragraph-length continuations `scan_citations` documents it cannot place. None of them is
# refused today; every one is visible, and none may grow except by a merge that re-measures. Paying one
# down means editing this line in the same commit.
POINTER_DEBT = 535

# How many entries the two logs held when that number was measured, and therefore the smallest corpus
# the number is a statement about. The ratchet is judged only at or above this size. Re-measured with the
# debt on 13 Nov 2026: 214 entries across the two logs on the merge of `origin/main` into `issue-318`.
# Re-measured again on 14 Nov with the debt at 535: 222 entries, the eight new ones arriving with `main`.
# The floor deliberately stays at the older, lower figure: the logs are append-only, so a tree smaller
# than 214 is a fixture or a fork rather than a checkout one commit behind, and every entry added between
# 214 and 222 keeps being judged instead of quietly escaping. Raising this number is the one edit that
# makes the ratchet judge LESS, which is why it does not move with a merge.
#
# This is not a rounding convenience; it is the rule not lying to a tree it did not measure.
# `factory/selftest.sh`'s `gate_verdict_off_refs` leg copies this script into a synthetic repository --
# two logs, one entry each -- to prove the verdict is a reading of files and not of the ref store, and the
# first full run after this rule landed answered that leg with `the decision logs now hold 0 citations
# that owe an anchor, below the recorded debt of 378: pay it down`. A constant from this repository's logs
# refused a repository that has no such logs, in a checkout where the thing under test was a different
# rule entirely. Below the floor the number is reported as not judged rather than applied.
#
# What that costs: a run whose logs hold FEWER entries than this escapes the ratchet, so deleting entries
# from a log is the one way to move the floor. It is visible in a diff, the logs are append-only, and a
# unit that deletes an entry to pass a citation check has told you what it did in the only place that
# records it. The other half -- an anchored citation whose text is not there -- is refused at any size.
POINTER_DEBT_AT = 214


CITATION = re.compile(r"`([^\s`]*?):(\d{1,5}(?:\s*-\s*\d{1,5})?)`")
# The anchor: a double-quoted fragment that follows the citation with nothing between it and the
# closing backtick but spaces and an apostrophe -- up to three characters of them, so `` `x.md:9` "…" ``
# and `` `x.md:9'"…` `` both read. What it does NOT admit is the `s` of a possessive: the corpus shape is
# `` `README.md:588`'s "…" ``, four characters of gap, and this class declines it. That is measured, not
# oversight -- see `NEAR_QUOTE` below for what widening would and would not buy. Anything further away is
# another sentence's quotation and belongs to a different claim. Sixteen characters is the shortest fragment
# that cannot be an accident: `the`, `verify` and `Chose: a` match half the tree, and an anchor that
# matches anything is a check that cannot fail.
ANCHOR_AFTER = re.compile(r"^[ ']{0,3}\"([^\"]{16,})\"")
MIN_ANCHOR = 16
# A quotation that is close to the coordinate but not flush against it, or flush and too short. The rule
# declines both, and a refusal that only says "has no anchor" sends the author to look for a rule they
# cannot see: they wrote a quotation, and the sentence that declines it never mentions the separator or
# the length. This regex exists so the refusal can name what it declined. It is NOT a second anchor rule:
# nothing here is judged by it, and widening `ANCHOR_AFTER` to what it matches was measured and refused
# (H-0170: three pointers in other units' entries change class for a 0.6% dent in a debt of 535).
NEAR_QUOTE = re.compile(r"^(.{0,6})\"([^\"]+)\"")
# Refusals name at most this many offenders; the count is in the headline either way.
MAX_LISTED = 12


def norm(text):
    """Whitespace-collapsed, for comparing a hand-wrapped quote against a wrapped source line."""
    return " ".join(text.split())


def tree_files(root=None):
    """Every file in the tree being gated, repo-relative, forward-slashed.

    `os.walk`, not `git ls-files`: the rule must answer the same way in CI, in the review loop and at
    the merge, and the gate reads no refs (see `gate_reads_no_refs` in `factory/selftest.sh`). An
    untracked file a unit has just written is therefore visible here, which is the right answer -- it
    is the file the citation will point at in the commit being prepared.
    """
    root = root or ROOT
    paths = []
    try:
        for dirpath, dirnames, filenames in os.walk(root):
            # `.git` by NAME, never by substring -- see the census note above.
            dirnames[:] = sorted(d for d in dirnames if d != ".git")
            for name in filenames:
                paths.append(os.path.relpath(os.path.join(dirpath, name), root).replace(os.sep, "/"))
    except OSError as exc:
        return None, "the tree cannot be walked (%s), so no citation was resolved and the rule did " \
                     "not run" % exc
    return sorted(paths), None


def tree_index(paths):
    """What a written path means, in two tables that are consulted in a fixed order.

    `exact` is keyed by repo-relative path, `base` by basename. A written path with a `/` in it -- or
    leading `./` -- is looked up in `exact` ONLY, which is what makes `./DECISIONS.md` mean the root
    log rather than "one of the two logs, you pick". Measured on the first run against the real logs:
    the repair wrote `./DECISIONS.md:1847`, a single-key index then stripped the `./` and answered with
    both logs, and the rule refused the very spelling its own message recommends -- the ambiguity it
    exists to prevent, reproduced inside the fix.

    A file at the repository root is its own basename, and appending it under both keys makes it
    "ambiguous with itself": every citation to `Makefile`, `README.md` and `AGENTS.md` would be refused
    for naming two files, both of them the one file. Measured on that same first run, where
    `notes.md:9` came back as `names 2 files (notes.md, notes.md)`. The keys are sets, so one file has
    one answer whatever it is called.
    """
    exact = {}
    base = {}
    for p in set(paths):
        exact.setdefault(p, set()).add(p)
        base.setdefault(os.path.basename(p), set()).add(p)
    return {"exact": {k: sorted(v) for k, v in exact.items()},
            "base": {k: sorted(v) for k, v in base.items()}}


def resolve_citation(written, index):
    """Which files a written path could mean, in the order a reader would check them."""
    if not written:
        return None
    if written.startswith("./") or "/" in written:
        return index["exact"].get(written[2:] if written.startswith("./") else written)
    return index["base"].get(written)


def entry_of(entries, line):
    """The entry a citation sits under, for the refusal to name."""
    owner = None
    for e in entries:
        if e["line"] > line:
            break
        owner = e["id"]
    return owner or "the log's own preamble"


def scan_citations(text, entries, index):
    """Every `path:NNN` citation in a log, as a dict the checker can judge.

    Three shapes are in these logs and they are not the same claim:

      * `` `triage.sh:122` `` -- a file, by full path or by a basename nothing else owns.
      * `` `:661-662` `` -- the same file again ("`README.md:588` and the table at `:661-662`"), so it
        inherits the last file resolved on the same line and never the last line read: a continuation
        is a property of its sentence, and inheriting across sentences would silently re-aim a
        pointer at a different file.
      * `` `pr:209` ``, `` `STATE_PROPS:1040` `` -- a triage `class:PR` pair and a symbol. Not
        citations. They fall out because their path half names no file in this tree.

    Known limit, measured rather than discovered later. "The same line" is a physical line, and an entry
    in these logs is markdown wrapped at ~110 columns, so one physical line is often a whole paragraph
    naming several files. Four continuations in `factory/DECISIONS.md` are aimed by a file that is not
    the one the prose means: `H-0110` writes "`emit()` is defined TWICE in `factory/selftest.sh`, at
    `:2766` ... and `:3731`" on a line whose earlier citation is `scripts/check-decisions.py:161`, and
    `H-0111` writes the PROCESS/CLAIM shim plants as `:2853` and `:3833` on a line whose earlier
    citation is `factory/triage.sh:600`. All four land in `unanchored`, where they cost nothing today --
    and all four would be refused as past-EOF by the length check #318's option 3 proposes, which is a
    fourth reason that option is out: it refuses four true sentences to catch zero false ones. The
    repair is a rule about sentences, not lines, and it is filed as its own issue rather than
    improvised here; a bare `:NNN` inside a wrapped paragraph is the one shape this rule admits it
    cannot place.
    """
    found = []
    for n, line in enumerate(text.splitlines(), 1):
        last = None
        for m in CITATION.finditer(line):
            written, span = m.group(1), m.group(2)
            continued = False
            if not written and last:
                written, continued = last, True
            anchor = None
            near = None
            am = ANCHOR_AFTER.match(line[m.end():])
            if am:
                anchor = am.group(1)
            else:
                nm = NEAR_QUOTE.match(line[m.end():])
                if nm:
                    # (the gap itself, length of the quote) -- what the refusal has to name when it
                    # declines this one for having no anchor at all. The gap is kept as text, not as a
                    # width, because `` `x.md:9`'s "…" `` is three characters wide and the third of them
                    # is what the rule declines; a width alone cannot tell the two refusals apart.
                    near = (nm.group(1), len(nm.group(2)))
            bounds = [int(x) for x in span.replace(" ", "").split("-")]
            low, high = bounds[0], bounds[-1]
            malformed = high < low or low < 1
            cand = resolve_citation(written, index)
            if continued and cand and len(cand) > 1:
                cand = None                    # a continuation inherits a resolved file, not a guess
            if cand:
                kind = "ambiguous" if len(cand) > 1 else ("bad-range" if malformed else "file")
                resolved = cand[0] if kind == "file" else None
                if kind == "file":
                    last = resolved
            elif written and revision_qualified(written):
                # `origin/main:factory/selftest.sh:4120` and `refs/heads/sibling:11` are in these logs.
                # They name the revision they were read at, which is the other half of the same promise
                # an anchor makes, so they owe no anchor -- and they cannot be checked here, because
                # checking them means reading another revision and this rule reads no git at all. The
                # pinned spelling is a sha; a branch name is the loose one, and `--citations` counts
                # them so the loose half stays visible.
                kind, resolved, cand = "revision-qualified", None, []
            elif written and "/" in written:
                # A path with a directory in it is a path even when nothing answers to it: the file was
                # renamed, or the entry misremembered where it lived. A bare `pr:209` is not -- that
                # shape is a triage `class:PR` pair, and it is common in both logs.
                kind = "bad-range" if malformed else "file-missing"
                resolved, cand = written, [written]
            else:
                kind, resolved, cand = "not-a-path", None, []
            found.append({"written": written, "kind": kind, "path": resolved,
                          "cands": cand, "low": low, "high": high, "anchor": anchor,
                          "near": near,
                          "line": n, "entry": entry_of(entries, n),
                          "continued": continued})
    return found


def revision_qualified(written):
    """Does this coordinate name the revision it was read at (`origin/main:file`, `refs/heads/x`,
    `3044b80:file`)? Such a citation states which tree to open, so it cannot silently drift.

    It is exempt from the anchor rule for the same reason an anchored one is checked: the reader has
    something to verify against. It is not checked *here*, because here means this tree and no other,
    and this rule reads no git (`gate_reads_no_refs`). `--citations` counts them separately: a branch
    name is the loose spelling of this form and a sha is the pinned one.
    """
    if ":/" in written or written.startswith(("refs/", "origin/", "upstream/", "heads/", "tags/", "HEAD")):
        return True
    return bool(re.fullmatch(r"[0-9a-f]{7,40}", written.split(":")[0]))


def debt_applies(entries, floor=POINTER_DEBT_AT):
    """Is the corpus in this tree the one `POINTER_DEBT` is a number about?

    Kept out as a function so `--self-test` can pin both sides of the boundary: a synthetic repository
    with two entries is not, and a tree that has grown since the measurement is. See the note above
    `POINTER_DEBT_AT` for the leg that measured this.
    """
    return entries is not None and entries >= floor


def no_anchor_note(near):
    """What the 'has no anchor' refusal adds when a quotation is sitting right there.

    The rule declines two shapes silently: a quotation separated from the closing backtick by more than
    three characters -- the possessive `s ` of `` `README.md:588`'s "…" `` is the ordinary one -- and a
    quotation flush against it but shorter than `MIN_ANCHOR`. Both leave the author with a sentence that
    says their citation has no anchor when they wrote one, and neither the separator nor the floor is
    named. Measured on the two logs: 3 citations carry a quotation of 16+ characters within six
    characters of the coordinate that the class declines, and 2 carry one of 12 or 13.
    """
    if not near:
        return ""
    gap, length = near
    if len(gap) > 3 or set(gap) - set(" '"):
        return (" -- there IS a quotation %d characters long sitting %d characters after the closing "
                "backtick, and the gap is `%s`: this rule reads only spaces and a lone apostrophe in that "
                "gap, so that quotation checks nothing. Move it tight against the coordinate; if the gap "
                "is a possessive, drop the possessive and put the coordinate after the sentence."
                % (length, len(gap), gap))
    return (" -- there IS a quotation right after the closing backtick but it is only %d characters, and "
            "the floor is %d: a fragment that short matches half the tree and a check that cannot fail is "
            "not a check -- quote the sentence, not a word" % (length, MIN_ANCHOR))


def stray_note(cands):
    """What the ambiguity refusal adds when one of the candidates may not belong to the repository.

    The tree walk is the filesystem, not `git ls-files` (H-0162: the gate reads no refs, so CI, the
    review loop and the merge commit answer identically). That is what makes an untracked directory able
    to ambiguous-ify a bare name -- a vendored checkout, a scratch tree, an ignored agent-state dir -- and
    the pointers this hits were written by other units, years of log history that no edit in this commit
    caused. Say so, and hand over the one read that separates the two, or the reader's only available
    repair is to re-path entries they did not touch.
    """
    return (" -- nothing here can tell which of these is the stray, because the gate reads the tree and "
            "not git. If one is untracked or ignored it is not the logs' defect and the repair is to "
            "remove it, not to re-path other units' entries: `git ls-files --others --ignored "
            "--exclude-standard` names it. A pointer that names %d files is still ambiguous either way, "
            "so the full path is owed whichever tree this is" % len(cands))


def classify(found, read):
    """The three buckets a citation falls into, plus the count that owes an anchor.

    `read(path)` returns the file's text or None. Split out of `citation_problems` so `--citations`
    reports exactly what the gate refuses on, rather than a second implementation that can drift from
    it -- the same reason `check-scripts.py` imports the encoding guard instead of copying it.

    Only `unanchored` feeds the ratchet. A pointer to a file that is absent, or to a name five files
    answer to, is not an unverified promise, it is a pointer that does not work today, and that is
    refused on its own and must be zero -- folding it into the number would have hidden the 17 pointers
    one merge from `main` delivered inside a baseline of 500 while the headline stayed green (measured
    on this branch's own merge of `origin/main`: 17 unopenable citations arrived in other units'
    entries, 10 of them a bare `README.md`, which five files in this tree answer to).

    That separation is why `citation_problems` refuses to JUDGE the number while either of the other two
    buckets is non-empty (#367 review #1). One untracked `vendor/AGENTS.md` moved 12 pointers out of
    `unanchored` and into `unresolvable` and the gate printed "below the recorded debt of 517 -- pay it
    down by lowering POINTER_DEBT to 505": the same tree content both invented a defect in five other
    units' entries and ordered the one repair that permanently disarms the ratchet. The number stays a
    statement about the logs, and the must-be-zero check keeps its own teeth.
    """
    unanchored = []
    unresolvable = []
    broken = []
    cache = {}

    def lines_of(path):
        if path not in cache:
            body = read(path)
            cache[path] = body.splitlines() if body is not None else None
        return cache[path]

    for c in found:
        if c["kind"] in ("not-a-path", "revision-qualified"):
            continue
        at = "%s:%d (%s)" % (c.get("label", "?"), c["line"], c["entry"])
        cite = "`%s:%s`" % (c["written"], c["low"] if c["high"] == c["low"]
                            else "%d-%d" % (c["low"], c["high"]))
        if c["kind"] == "ambiguous":
            unresolvable.append("%s %s names %d files (%s) -- a pointer the reader has to guess at is "
                                "not a pointer; write the path you mean, or `./%s` for one at the root%s"
                                % (at, cite, len(c["cands"]), ", ".join(sorted(c["cands"])),
                                   os.path.basename(c["written"]), stray_note(c["cands"])))
            continue
        body = lines_of(c["path"])
        want = norm(c["anchor"]) if c["anchor"] is not None else None
        if body is None:
            row = broken if c["anchor"] else unresolvable
            row.append("%s %s cites %s, which is not in the tree being gated -- %s"
                       % (at, cite, c["path"],
                          "an anchored citation promises the reader a line and there is no file to "
                          "read it in" if c["anchor"] else "the file was renamed, deleted, or never "
                          "lived where the entry said; write the path a reader can open today"))
            continue
        if want is None:
            unanchored.append("%s %s -> %s has no anchor, so nothing here can tell whether line %d "
                              "still says what the entry measured%s"
                              % (at, cite, c["path"], c["low"], no_anchor_note(c.get("near"))))
            continue
        now = [i + 1 for i, l in enumerate(body) if want in norm(l)]
        if c["low"] > len(body):
            broken.append("%s %s cites line %d of %s, which has %d lines, anchored to %r -- %s"
                          % (at, cite, c["low"], c["path"], len(body), c["anchor"][:48],
                             "it is at line %s" % ", ".join(str(x) for x in now[:6]) if now
                             else "it is nowhere in the file"))
            continue
        if want in norm(" ".join(body[c["low"] - 1:c["high"]])):
            continue
        where = ("it is at line %s now, not %d -- the lines above it moved, and a pointer that is "
                 "wrong by one line still lands on the wrong sentence"
                 % (", ".join(str(x) for x in now[:6]), c["low"])) if now else \
                ("it is nowhere in %s -- the sentence was deleted, or the entry never quoted it "
                 "faithfully; line %d reads: %s"
                 % (c["path"], c["low"], norm(body[c["low"] - 1])[:96]))
        broken.append("%s %s in %s is anchored to %r and %s"
                      % (at, cite, c["path"], c["anchor"][:48], where))
    return unanchored, unresolvable, broken


CitationVerdict = collections.namedtuple(
    "CitationVerdict",
    # `judged` is the answer to "did the ratchet actually compare this number?", and `unjudged_because`
    # is the sentence that says why it did not. Both exist because the headline used to claim "the first
    # number is the recorded debt, which may only fall" on a run where the number had just been moved by
    # an untracked directory -- a true sentence about a false measurement. One classifier, one verdict,
    # so the headline cannot disagree with the refusal list above it.
    "problems measured unanchored unresolvable broken judged unjudged_because")


def citation_problems(found, read, debt=POINTER_DEBT, judge_debt=True):
    """Problems, and a `CitationVerdict` carrying the measured count of citations that still owe an anchor.

    `read(path)` is injected so `--self-test` can assert every arm without a filesystem: the moved
    anchor, the deleted one, the path that is two files, the range written backwards, a missing file
    anchored and unanchored, a citation that names its own revision, the ratchet in both directions, and
    the ratchet NOT running when a pointer cannot be opened. `judge_debt=False` is the `--log` case,
    where the corpus the number describes is not the corpus being read.
    """
    problems = []
    for c in found:
        if c["kind"] == "bad-range":
            problems.append("%s:%d (%s) `%s:%d-%d` cites no lines: the range runs backwards or starts "
                            "before the first line of the file, so there is nothing for a reader to "
                            "open" % (c.get("label", "?"), c["line"], c["entry"], c["written"],
                                      c["low"], c["high"]))
    unanchored, unresolvable, broken = classify(found, read)
    for group, head in ((broken, "anchored citation(s) in the decision logs point at text that is not "
                                 "there"),
                        (unresolvable, "citation(s) in the decision logs a reader cannot open at all")):
        if not group:
            continue
        problems.append("%d %s:" % (len(group), head))
        problems.extend("    " + g for g in group[:MAX_LISTED])
        if len(group) > MAX_LISTED:
            problems.append("    ... and %d more; `python3 scripts/check-decisions.py --citations` "
                            "lists all of them" % (len(group) - MAX_LISTED))
    measured = len(unanchored)
    unopenable = len(unresolvable) + len(broken)
    if not judge_debt:
        return CitationVerdict(problems, measured, unanchored, unresolvable, broken,
                               False, "--log narrows the corpus the number is a count of")
    if debt is None:
        problems.append("the citation-debt number could not be read, so the rule did not run")
        return CitationVerdict(problems, measured, unanchored, unresolvable, broken,
                               False, "the recorded debt could not be read")
    if unopenable:
        # The ratchet is a statement about the logs, and this number is not one: every pointer in the two
        # lists above left `unanchored` without the logs changing, and a stray directory can do it to a
        # dozen at a time. Measured at `bc1c08c` with one untracked `vendor/AGENTS.md`: 12 pointers moved
        # and the gate ordered "lower POINTER_DEBT to 505" -- the one repair that permanently disarms it,
        # issued to a reader under a green-gate deadline (#367 review #1). The must-be-zero refusals above
        # still stand on their own; only the comparison with the constant waits.
        problems.append("%d citation(s) above are not in the %d the ratchet guards: a pointer a reader "
                        "cannot open is a defect, not a grandfathered promise. The number is therefore "
                        "NOT judged this run, and that is not an invitation to edit POINTER_DEBT -- an "
                        "untracked or ignored directory moves pointers out of the debt and into that list, "
                        "so a number measured against a tree the repository does not have says nothing "
                        "about the logs. Fix the list, make `git status --short --ignored` empty of "
                        "anything the logs did not write, and the comparison comes back on the next run."
                        % (unopenable, measured))
        return CitationVerdict(problems, measured, unanchored, unresolvable, broken,
                               False, "%d citation(s) a reader cannot open" % unopenable)
    if measured > debt:
        problems.append("the decision logs hold %d `path:line` citations that owe an anchor, and the "
                        "recorded debt is %d: the count went UP. A citation you add today has to carry "
                        "one -- a quoted fragment of at least %d characters immediately after the "
                        "closing backtick, the text you actually read at that line -- and nothing may "
                        "be added to this number. If yours is not in the list, one of these is already "
                        "yours to pay down. A merge from `main` raises this number with nothing written "
                        "by you: `--citations` re-derives it, and raising POINTER_DEBT to the new "
                        "measurement is legal only beside the entry that records the measurement, "
                        "because the one-line diff is the whole audit trail."
                        % (measured, debt, MIN_ANCHOR))
        problems.extend("    " + u for u in unanchored[:MAX_LISTED])
    elif measured < debt:
        problems.append("the decision logs now hold %d citations that owe an anchor, below the recorded "
                        "debt of %d -- pay it down by lowering POINTER_DEBT to %d in this same commit. "
                        "An unlowered number is a ratchet with the pawl filed off: the next unanchored "
                        "citation hides in the slack." % (measured, debt, measured))
    return CitationVerdict(problems, measured, unanchored, unresolvable, broken, True, None)


def citation_report(found, read, only=None, verdict=None):
    """The `--citations` listing: what the debt is made of, by the file it points at.

    With `only`, it answers the other question instead -- the one to ask BEFORE editing a file: every
    citation pointing at that path, with the log line that cites it and the number it names. A merge
    that moves a line has to find the pointers to fix in the same commit, and the by-file counts above
    cannot tell a reader which ones. Without this, the question was answerable only by grepping the
    logs for a basename and reading 378 lines of prose.

    `verdict` is the one the gate already computed; passing it is what makes the last line of this report
    and the refusal's headline the same number by construction rather than by two runs of `classify()`
    agreeing (#367 review, non-blocking: three calls to one classifier was three chances to drift).
    """
    if only:
        want = only[2:] if only.startswith("./") else only
        rows = []
        for c in found:
            if c["kind"] == "not-a-path":
                continue
            if c["path"] == want or os.path.basename(c["written"] or "") == os.path.basename(want):
                rows.append("%s:%d (%s) `%s:%s` -> %s %s"
                            % (c.get("label", "?"), c["line"], c["entry"], c["written"],
                               c["low"] if c["high"] == c["low"] else "%d-%d" % (c["low"], c["high"]),
                               c["path"] or "(unresolved)",
                               "anchored" if c["anchor"] else "UNANCHORED"))
        print("citations pointing at %s (%d):" % (want, len(rows)))
        for r in sorted(rows, key=lambda s: s.split("`")[0]):
            print("  " + r)
        if rows:
            print("  -- editing that file moves these. Fix them in the same commit, or the log keeps "
                  "pointing at whatever line arrived where the number used to be.")
        return []

    if verdict is None:
        # A report asked for on its own has no verdict to borrow; classifying here is the one copy of the
        # rule, not a second implementation of it.
        unanchored, unresolvable, broken = classify(found, read)
        note = ""
    else:
        unanchored, unresolvable, broken = verdict.unanchored, verdict.unresolvable, verdict.broken
        note = "" if verdict.judged else " (NOT judged this run: %s)" % verdict.unjudged_because
    counts = {}
    for c in found:
        if c["kind"] in ("file", "file-missing") and c["anchor"] is None:
            counts[c["path"]] = counts.get(c["path"], 0) + 1
    out = ["citations that owe an anchor, by the file they point at (%d):" % sum(counts.values())]
    for path in sorted(counts, key=lambda p: (-counts[p], p)):
        out.append("  %3d  %s" % (counts[path], path))
    out.append("citations a reader cannot open (%d):" % len(unresolvable))
    out.extend("  " + u for u in unresolvable)
    out.append("anchored citations whose text has moved or gone (%d):" % len(broken))
    out.extend("  " + b for b in broken)
    out.append("citations that name their own revision, exempt from both (%d):"
               % len([c for c in found if c["kind"] == "revision-qualified"]))
    out.append("debt this gate ratchets on: %d%s" % (len(unanchored), note))
    return out


def cross_log_problems(where_seen):
    """That no decision id is claimed by two logs.

    The rule H-0021 leaves behind: product decisions are `D-` in `DECISIONS.md`, harness decisions are
    `H-` in `factory/DECISIONS.md`, and a citation is only worth anything while it resolves to one entry.
    `where_seen` maps an id to the logs that carry it, so this is testable without touching either file --
    which matters, because a rule that lives inline in `main` has no case, and review on #217 measured
    that deleting the loop entirely left `--self-test` green.

    The clean case is asserted too, and it is the whole point of the prefix split: `D-0012` and `H-0012`
    are two different decisions in two different logs and must not be reported as a collision.
    """
    problems = []
    for ident in sorted(where_seen):
        if len(where_seen[ident]) > 1:
            problems.append(
                "%s is used in %s -- one id, %d decisions; a citation to it does not resolve. "
                "Renumber the later entry in its own log." %
                (ident, " and ".join(sorted(where_seen[ident])), len(where_seen[ident])))
    return problems


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

    # An empty log has nothing parsed to tell it which id space it writes, so the example it offers is
    # a claim about the grammar rather than about the file -- and it used to be the same `D-` claim for
    # every log, which is false of the harness log (#268). The needle here used to be "no entry found",
    # a phrase the message prints whichever example sits in it: the rule was covered, the id was not.
    # Each empty log is now answered with the heading its own space writes.
    for name, want in (("no-entries", "## D-0001 · …"), ("no-entries-harness", "## H-0001 · …")):
        counts[0] += 1
        got = check(parse(BAD[name], name))
        if not got:
            fails.append("%s: expected a problem, got none -- the rule is not being applied" % name)
        elif not any(want in p for p in got):
            fails.append("%s: the refusal does not offer `%s`, the heading this log writes -- %s"
                         % (name, want, got))

    # The rule that row is one instance of, stated once for every refusal this file prints: an id in
    # the sentence has to be something the reader can resolve. A citation has to be an entry in the log
    # being read -- an invented one sends a person to grep for an entry that is not there; an example
    # has to be a heading `ENTRY` parses -- an example the parser does not match keeps the log refused
    # after they do what it says. Iterating `BAD` instead of naming an arm is what makes it the same
    # rule for both messages that name an id in prose, and it costs no edit when a fixture is added:
    # the out-of-order refusal is held to it exactly as this one is.
    for name, text in sorted(BAD.items()):
        counts[0] += 1
        parsed = parse(text, name)
        known = set(e["id"] for e in parsed)
        for p in check(parsed):
            snippets = EXAMPLE_HEADING.findall(p)
            # A citation is whatever id survives removing every printed example, so a message cannot
            # exempt its own invention by writing it as a heading.
            elsewhere = p
            for snippet in snippets:
                elsewhere = elsewhere.replace(snippet, " ")
            for invented in sorted(set(ID_TOKEN.findall(elsewhere))):
                if invented in known:
                    continue
                fails.append("%s: the refusal cites %s, which is not an entry in the log it is reading"
                             " -- a reader sent to grep for it finds nothing: %s" % (name, invented, p))
            for snippet in sorted(set(snippets)):
                # `…` stands for the title, so the example is completed before it is asked whether
                # ENTRY would take it -- but from the printed bytes, not from an id lifted out of them.
                # The digits, the prefix and the separator are the thing under test, and a check that
                # rebuilt the heading itself could never fail.
                if not ENTRY.match(snippet.rstrip("…").rstrip() + " Example"):
                    fails.append("%s: the refusal offers %r as the heading to write, which this file's "
                                 "own ENTRY does not parse -- %s" % (name, snippet, p))

    expect_fail(BAD["missing-field"], "missing-field", "missing Options")
    expect_fail(BAD["one-option"], "one-option", "one option")
    expect_fail(BAD["chose-unknown"], "chose-unknown", "not among the recorded options")
    expect_fail(BAD["escalated-no-await"], "escalated-no-await", "without `Awaiting:`")
    expect_fail(BAD["bad-sha"], "bad-sha", "not a commit sha")
    expect_fail(BAD["checkpoint-na"], "checkpoint-na", "no checkpoint")
    expect_fail(BAD["duplicate"], "duplicate", "already used")
    # Out of order, in every id-space arrangement -- and asserting the *id* named, not just the phrase.
    # The case used to be a single `expect_fail(..., "out of order")`, a needle the buggy message also
    # printed: the arm was covered and the wrong name still shipped, because the only fixture reaching
    # it was a `D-` log. Each row now has to name its own predecessor; the two single-space logs must
    # not cite the other space at all, and the mixed log has no such assertion to make (both prefixes
    # are legitimately in that file) so its whole weight is on which id the parenthetical names.
    for name, want, foreign in (("out-of-order", "D-0009 came before it", r"\bH-\d"),
                                ("out-of-order-harness", "H-0082 came before it", r"\bD-\d"),
                                ("out-of-order-mixed", "D-0082 came before it", None)):
        counts[0] += 1
        got = [p for p in check(parse(BAD[name], name)) if "out of order" in p]
        if not got:
            fails.append("%s: expected a problem, got none -- the rule is not being applied" % name)
        elif not any(want in p for p in got):
            fails.append("%s: the refusal does not name %s -- %s" % (name, want, got))
        elif foreign and any(re.search(foreign, p) for p in got):
            fails.append("%s: the refusal cites an id from the other log's id space, which is an id "
                         "nothing in this file resolves -- %s" % (name, got))

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

    # The cross-log id rule, asserted both ways. A collision must be named, and the D-/H- pair that the
    # split exists to allow must not be. Deleting `cross_log_problems`' loop would otherwise leave this
    # script advertising a check in SKILL.md that nothing runs.
    counts[0] += 1
    collided = cross_log_problems({"D-0012": ["DECISIONS.md", "factory/DECISIONS.md"],
                                   "H-0001": ["factory/DECISIONS.md"]})
    if not any("one id, 2 decisions" in p for p in collided):
        fails.append("cross-log-collision: an id used in both logs passed the rule -- %s" % collided)
    # A checkpoint that is well-shaped and does not exist. The shape test alone let an invented sha
    # through in this very session, so both answers are asserted against the injected resolver.
    counts[0] += 1
    absent = checkpoint_problems(parse(GOOD, "sample"), resolve=lambda s: False)
    if not any("not a commit in this repository" in p for p in absent):
        fails.append("checkpoint-absent: a checkpoint the repository does not have passed the rule -- %s" % absent)
    counts[1] += 1
    present = checkpoint_problems(parse(GOOD, "sample"), resolve=lambda s: True)
    if present:
        fails.append("checkpoint-present: a checkpoint the resolver vouches for was refused -- %s" % present)
    # And the third answer: nobody answered. `git_checkpoint_resolver` returns None when the call
    # raised or timed out, and the rule used to read that as "present" via `resolve.get(cp, True)` --
    # a clean gate in which the existence rule had never run. Asserted as its own case so deleting
    # the arm turns this self-test red rather than quietly returning the count to what it was.
    counts[0] += 1
    unknown = checkpoint_problems(parse(GOOD, "sample"), resolve=None)
    if not any("could not be asked" in p for p in unknown):
        fails.append("checkpoint-unknown: a resolver that could not ask produced no refusal, so the "
                     "rule passes when it has not run -- %s" % unknown)
    # The default, not just the sentinel. `checkpoint_problems` asks `resolve.get(cp, False)`, and
    # `None` and `{}` are different failures with opposite consequences: `None` is "git did not answer",
    # `{}` is "git answered that this object is nothing" -- and a resolver that built its mapping over
    # the shas it *validated*, rather than the shas it *found*, hands back exactly that `{}`. With the
    # default flipped to True the entry above still refuses (a callable returning False answers) and this
    # case is the only thing left that notices, so it is asserted here rather than reasoned about.
    # Measured on #217 round five: flipping the default left the self-test green at rc 0 until this row
    # was added, at which point it named `names checkpoint 2f4c1ab9, which is not a commit`.
    counts[0] += 1
    empty = checkpoint_problems(parse(GOOD, "sample"), resolve={})
    if not any("not a commit in this repository" in p for p in empty):
        fails.append("checkpoint-empty-map: an empty resolver answered the existence rule with a pass "
                     "-- %s" % empty)
    counts[1] += 1
    no_checkpoints = checkpoint_problems(parse("## D-0001 · Title\n\n- Question: q\n- Checkpoint: n/a\n",
                                              "sample"), resolve=None)
    if no_checkpoints:
        fails.append("checkpoint-unknown-nothing-to-ask: a log with no named checkpoints was refused "
                     "for a git problem it does not have -- %s" % no_checkpoints)

    counts[1] += 1
    separated = cross_log_problems({"D-0012": ["DECISIONS.md"], "H-0012": ["factory/DECISIONS.md"]})
    if separated:
        fails.append("cross-log-clean: `D-0012` and `H-0012` are different decisions in different logs "
                     "and the rule refused them -- %s" % separated)

    # ---------------------------------------------------------------------------------------------
    # `path:line` citations (#318). A fixture tree of the exact shape the rule has to survive:
    #   * two files that answer to the basename `SKILL.md`, which is the ambiguity #318 counted;
    #   * a directory whose name begins with `.git` that must NOT be skipped -- `"/.git" in
    #     "./.github/workflows"` is true, and the first census of #318 wrote that and reported a
    #     living citation in ci.yml as dead;
    #   * a file three lines long, to cite past the end of.
    # Nothing here touches git or the filesystem, because the rule itself must not either.
    CITES_TREE = {
        ".github/workflows/ci.yml": "name: CI\non: push\njobs:\n  verify:\n    runs-on: ubuntu-latest\n",
        "docs/factory.md": "one\ntwo\nthe escalation label is printed by exactly one script\nfour\n",
        "SKILL.md": "the root skill file is the one a bare name must not silently choose\n",
        "skills/a/SKILL.md": "aaa\n",
        "skills/b/SKILL.md": "the second skill file's first line is long enough\n",
        "notes.md": "first\nsecond\nthe third line is long enough to anchor a citation to\n",
    }
    CITES_INDEX = tree_index(sorted(CITES_TREE))
    read_fixture = CITES_TREE.get

    def cites(body):
        found = scan_citations(body, parse(GOOD, "log"), CITES_INDEX)
        for c in found:
            c["label"] = "log"
        return found

    def cite_case(body, name, needle, debt=0, judge=True):
        counts[0] += 1
        verdict = citation_problems(cites(body), read_fixture, debt=debt, judge_debt=judge)
        probs = verdict.problems
        if not probs:
            fails.append("%s: an unresolvable citation passed -- the rule is not being applied" % name)
        elif not any(needle in p for p in probs):
            fails.append("%s: expected a refusal containing %r, got %s" % (name, needle, probs))

    def cite_ok(body, name, debt=0):
        counts[1] += 1
        verdict = citation_problems(cites(body), read_fixture, debt=debt)
        if verdict.problems:
            fails.append("%s: expected clean, got %s" % (name, verdict.problems))
        return verdict.measured

    # The good case, and it is asserted at debt 0: a clean citation must also measure clean, or the
    # ratchet below is judging a number nothing produced.
    cite_ok('See `docs/factory.md:3` "the escalation label is printed by exactly one script".\n',
            "anchor-exact")
    cite_ok('Lines `docs/factory.md:2-3` "the escalation label is printed by exactly one script".\n',
            "anchor-in-range")
    # The ticket's own caveat, as a case: `.github` is a directory whose name starts with the string
    # `.git`, and skipping by substring would drop every citation into `.github/` from the tree.
    cite_ok('`.github/workflows/ci.yml:5` "runs-on: ubuntu-latest" is the runner.\n',
            "dot-dir-not-skipped")
    cite_case('`docs/factory.md:2` "the escalation label is printed by exactly one script".\n',
              "anchor-moved", "it is at line 3 now, not 2")
    cite_case('`docs/factory.md:3` "a sentence nothing in this file ever wrote".\n',
              "anchor-absent", "nowhere in docs/factory.md")
    cite_case('`docs/factory.md:2-3` "a sentence nothing in this file ever wrote".\n',
              "anchor-outside-range", "nowhere in docs/factory.md")
    cite_case('`notes.md:9` "the third line is long enough to anchor a citation to".\n',
              "anchor-past-eof", "which has 3 lines")
    cite_case('`notes.md:1-0` is not a range a reader can open.\n', "range-backwards", "cites no lines")
    cite_case('Both `skills/a/SKILL.md:1` and `SKILL.md:1` are cited here.\n',
              "ambiguous-basename", "names 3 files")
    # The disambiguation this rule tells you to write, asserted to work: `./` is root-relative and is
    # looked up ONLY as a path, never by basename. It is in the message above, and the first run of the
    # real repair wrote `./DECISIONS.md:1847` and was refused by the rule for it -- advice that does not
    # work is worse than no advice, because the reader follows it and then cannot tell what is wrong.
    cite_ok('`./SKILL.md:1` "a bare name must not silently choose" is the root file.\n',
            "dot-slash-is-root-relative")
    cite_case('`./SKILL.md:1` "the second skill file\'s first line".\n',
              "dot-slash-not-basename", "nowhere in SKILL.md")
    cite_case('`gone/file.md:2` "the third line is long enough to anchor a citation".\n',
              "anchored-file-gone", "not in the tree being gated")
    # The same absent file with no anchor is not "unverified", it is a pointer that does not work
    # today. It is refused outright rather than added to the debt, or 41 of them could sit inside a
    # baseline of 375 with the headline green.
    cite_case('`gone/file.md:2` names a file this tree does not have.\n',
              "unanchored-file-gone", "cannot open at all")
    # A citation that names its own revision states which tree to open, which is the promise the
    # anchor exists to make. It is exempt -- and exempt means "not counted", asserted at debt 0: an
    # exempt citation that still raised the number would be refused, and the exemption would be a
    # sentence in a docstring rather than a property of the rule.
    cite_ok('`origin/main:factory/selftest.sh:4120` and `refs/heads/sibling:11` both name a revision.\n',
            "revision-qualified-exempt")
    # The ratchet, both ways. Over: a new pointer arrived with nothing to check it against.
    cite_case("`notes.md:2` says something.\n", "debt-rose", "the count went UP")
    # Under: somebody paid one down and left the number claiming the old figure.
    cite_case('`docs/factory.md:3` "the escalation label is printed by exactly one script".\n',
              "debt-fell", "lowering POINTER_DEBT to 0", debt=4)
    # And the narrowing that makes the number mean what it says: `--log` reads one file, and the
    # number describes the pair, so it is not judged -- yet the anchor rule above still runs there.
    counts[1] += 1
    narrowed = citation_problems(cites("`notes.md:2` says something.\n"), read_fixture,
                                 debt=0, judge_debt=False)
    if narrowed.problems:
        fails.append("debt-not-judged-under-log: --log was refused on a number that counts the other "
                     "log too -- %s" % narrowed.problems)
    if narrowed.measured != 1:
        fails.append("debt-not-judged-under-log: the unanchored citation was not even counted -- %r"
                     % (narrowed,))
    # The floor. `POINTER_DEBT` is a number about a corpus, and a tree with fewer entries in its logs
    # than that corpus held is a different repository. This one is measured, not designed:
    # `gate_verdict_off_refs` in `factory/selftest.sh` copies this script into a two-entry fixture to
    # prove the verdict reads files and not refs, and the first full run refused that fixture with "the
    # decision logs now hold 0 citations that owe an anchor, below the recorded debt of 378".
    counts[1] += 1
    if not debt_applies(POINTER_DEBT_AT) or debt_applies(POINTER_DEBT_AT - 1) \
            or debt_applies(None):
        fails.append("debt-floor: the boundary between this corpus and a foreign one moved -- %d judged,"
                     " %d not, None %r" % (POINTER_DEBT_AT, POINTER_DEBT_AT - 1, debt_applies(None)))
    # And the floor does not silence the part that is about a pointer rather than a count.
    counts[0] += 1
    foreign = citation_problems(cites('`docs/factory.md:2` "a sentence nothing in this file ever '
                                      'wrote".\n'), read_fixture, debt=0, judge_debt=False)
    if not any("nowhere in docs/factory.md" in p for p in foreign.problems):
        fails.append("foreign-corpus-still-checks: below the floor the anchor rule went quiet too -- %s"
                     % foreign.problems)

    # What is NOT a citation. `pr:209` and `unclaimed:263` are a triage `class:PR` pair that appears
    # in both logs; `STATE_PROPS:1040` is a symbol. Refusing every `word:digit` span would have been
    # fifteen false refusals in the first run of this rule, so a span counts only when its path half
    # names a file in the tree -- and that is why the `.git` exclusion above is done by name.
    counts[1] += 1
    prose = cites("The supervisor dispatched `pr:209` and `unclaimed:263` at 06:30 on `2026-11-10T23:51`.\n")
    if [c for c in prose if c["kind"] != "not-a-path"]:
        fails.append("not-a-citation: class:PR and clock spans were read as citations -- %s"
                     % [c for c in prose if c["kind"] != "not-a-path"])
    # A continuation ("`README.md:588` and the table at `:661-662`") inherits the file named earlier in
    # the SAME LINE. Inheriting across lines would silently re-aim a pointer at whatever file the
    # previous sentence happened to name.
    counts[0] += 2
    cont = cites('`notes.md:3` "the third line is long enough" and the row at `:1` agree.\n')
    tail = [c for c in cont if c["continued"]]
    if len(tail) != 1 or tail[0]["path"] != "notes.md":
        fails.append("continuation: `:1` did not inherit notes.md from its own line -- %s" % cont)
    orphan = cites("and the row at `:1` agrees, in a sentence naming no file.\n")
    if [c for c in orphan if c["kind"] != "not-a-path"]:
        fails.append("continuation-orphan: a bare `:1` with no file on its line resolved to something "
                     "anyway -- %s" % orphan)
    # A short quote is not an anchor: `notes` and `Chose: a` match half the tree, and an anchor that
    # matches anything is a check that cannot fail. It falls into the debt instead, which the ratchet
    # then refuses -- the loophole closes itself.
    counts[0] += 1
    short = citation_problems(cites('`notes.md:3` "the third line" is all it quoted.\n'),
                              read_fixture, debt=0)
    if short.measured != 1 or not any("went UP" in p for p in short.problems):
        fails.append("short-anchor: a %d-character quote was taken for an anchor -- %r" % (MIN_ANCHOR, short))
    # ... and the refusal says which of its two floors it applied. Measured on the two logs: 3 citations
    # carry a quotation of 16+ characters within six characters of the coordinate that the separator class
    # declines, and 2 carry one of 12 or 13 characters. A message that only says "has no anchor" sends all
    # five to look for a rule that is not there (#367 review, non-blocking).
    counts[0] += 2
    for shape, needle, name in (
            ('`notes.md:3`\'s "the third line is long enough to anchor a citation to".\n',
             "the gap is `'s `", "declined-separator-names-the-gap"),
            ('`notes.md:3`, "the third line is long enough to anchor a citation to".\n',
             "characters long sitting 2 characters after", "declined-comma-names-the-gap")):
        got = citation_problems(cites(shape), read_fixture, debt=0)
        if not any(needle in q for q in got.problems):
            fails.append("%s: a quotation the separator class declines was refused with no hint of the "
                         "separator -- %s" % (name, got.problems))
        if not any("the count went UP" in q for q in got.problems):
            fails.append("%s: the declined quotation was not counted either -- %s" % (name, got.problems))
    # The separator note has to stay silent when there is no quotation in sight, or every one of the 535
    # old pointers would grow a sentence about a quote it never had.
    counts[1] += 1
    naked = citation_problems(cites("`notes.md:2` says something.\n"), read_fixture, debt=0)
    if any("there IS a quotation" in q for q in naked.problems):
        fails.append("declined-separator-phantom: a citation with no quotation nearby was told about one "
                     "-- %s" % naked.problems)

    # ---------------------------------------------------------------------------------------------
    # #367 review #1: an untracked directory must not be able to order the ratchet lowered.
    #
    # The fixture's two `SKILL.md` basenames are the reviewer's `vendor/AGENTS.md` in miniature: one extra
    # file in the tree makes a bare pointer ambiguous, which moves it OUT of `unanchored`. Before this
    # commit that lowered the measured number and the gate answered with "pay it down by lowering
    # POINTER_DEBT", i.e. the tree content both invented the defect and disarmed the guard.
    counts[0] += 3
    stray = cites("Both `skills/a/SKILL.md:1` and `SKILL.md:1` are cited here.\n")
    moved = citation_problems(stray, read_fixture, debt=4)
    if any("lowering POINTER_DEBT" in q for q in moved.problems):
        fails.append("stray-tree-orders-the-ratchet-lowered: 3 pointers left the debt because a file "
                     "appeared, and the gate told the reader to disarm it -- %s" % moved.problems)
    if moved.judged is not False or moved.unjudged_because is None:
        fails.append("stray-tree-not-flagged: the headline has no reason to print instead of \"may only "
                     "fall\" -- %r" % (moved,))
    if not any("cannot open at all" in q for q in moved.problems):
        fails.append("stray-tree-quiet: the must-be-zero refusal vanished with the number -- %s"
                     % moved.problems)
    # The same stray tree, and the ratchet must stay shut on the way UP too: an unanchored pointer
    # arriving beside 3 unopenable ones is still refused, but by the list, not by the comparison.
    rose = citation_problems(cites("Both `skills/a/SKILL.md:1` and `SKILL.md:1` are cited here, and "
                                   "`notes.md:2` says something.\n"), read_fixture, debt=0)
    if any("went UP" in q for q in rose.problems):
        fails.append("stray-tree-still-ratchets-up: the number was compared with the constant while 3 "
                     "pointers could not be opened -- %s" % rose.problems)
    if not any("cannot open at all" in q for q in rose.problems):
        fails.append("stray-tree-lost-the-new-pointer: the unanchored citation was not refused either -- %s"
                     % rose.problems)
    # The control, which is the whole point: with nothing unopenable, both arms of the ratchet still bite.
    # Suppressing the comparison must not become a way to switch the rule off.
    counts[1] += 2
    for body, needle, name in (
            ("`notes.md:2` says something.\n", "the count went UP", "ratchet-up-still-bites"),
            ('`docs/factory.md:3` "the escalation label is printed by exactly one script".\n',
             "lowering POINTER_DEBT to 0", "ratchet-down-still-bites")):
        clean = citation_problems(cites(body), read_fixture, debt=4 if "factory" in body else 0)
        if not any(needle in q for q in clean.problems):
            fails.append("%s: the unopenable guard silenced the ratchet for a tree that has no unopenable "
                         "pointer -- %s" % (name, clean.problems))
        if clean.judged is not True or clean.unjudged_because is not None:
            fails.append("%s: a clean tree reported itself as not judged -- %r" % (name, clean))
    # And `--citations` says so on the line the reader actually reads.
    counts[0] += 1
    report = [l for l in citation_report(stray, read_fixture, verdict=moved)
              if l.startswith("debt this gate ratchets on")]
    if not report or "NOT judged this run" not in report[0]:
        fails.append("report-headline-quiet: --citations printed the moved number as if it were the debt "
                     "-- %s" % report)
    counts[1] += 1
    clean_report = [l for l in citation_report(cites("`notes.md:2` says something.\n"), read_fixture,
                                               verdict=citation_problems(
                                                   cites("`notes.md:2` says something.\n"),
                                                   read_fixture, debt=1))
                    if l.startswith("debt this gate ratchets on")]
    if not clean_report or "NOT judged" in clean_report[0]:
        fails.append("report-headline-phantom: a judged run called itself unjudged -- %s" % clean_report)
    # The ambiguity refusal has to hand over the read that separates "the logs are wrong" from "a stray
    # directory is in the tree", or the only repair a reader can see is re-pathing other units' entries.
    counts[0] += 1
    if not any("git ls-files --others --ignored" in q for q in moved.problems):
        fails.append("stray-note-missing: the ambiguity refusal orders work on files it cannot tell apart "
                     "-- %s" % moved.problems)

    if fails:
        print("check-decisions --self-test FAILED:")
        for f in fails:
            print("  %s" % f)
        return 1
    print("check-decisions --self-test: %d cases that must fail, %d that must pass, the two "
          "invocations in `verify`, four Makefiles that delete this gate, the cross-log id rule, "
          "checkpoint existence, the out-of-order refusal in both logs and in a file holding both, and "
          "the `path:line` citation rule (anchor exact, moved, absent, past EOF, ambiguous, absent "
          "file, revision-qualified, class:PR prose, inherited ranges, the ratchet both ways, the ratchet "
          "held shut while a pointer cannot be opened, and the refusal that names which anchor floor it "
          "applied), each asserted both ways" % (counts[0], counts[1]))
    return 0


def main(argv):
    if len(argv) > 1 and argv[1] in ("-h", "--help"):
        print(USAGE)
        return 0
    if len(argv) > 1 and argv[1] == "--self-test":
        return self_test()
    report = len(argv) > 1 and argv[1] == "--citations"
    report_only = argv[2] if report and len(argv) > 2 else None
    if report and report_only not in (None, "--help", "-h") and report_only.startswith("-"):
        print("check-decisions: --citations takes at most one path, not %r\n%s" % (report_only, USAGE),
              file=sys.stderr)
        return 2

    path = LOG
    if len(argv) > 2 and argv[1] == "--log":
        if len(argv) < 3 or not argv[2]:
            print("check-decisions: --log wants a path; an empty one is a change of subject, not a "
                  "default", file=sys.stderr)
            return 2
        path = argv[2]
    elif len(argv) > 1 and argv[1] not in ("--citations",):
        print("check-decisions: unknown argument %r\n%s" % (argv[1], USAGE), file=sys.stderr)
        return 2

    paths = [path] if path != LOG else list(LOGS)
    # The debt is a number about THIS pair of logs. Narrowing the file narrows everything the rules
    # can see, and `POINTER_DEBT` describes the pair -- counting one log against it would refuse every
    # `--log` run with a number that means nothing. So the debt is not judged here, and the line below
    # says so rather than going quiet about it: `--log` used to skip two rules silently, which is what
    # a fabricated checkpoint survived (#229).
    judge_debt = path == LOG
    tree, walk_problem = tree_files()
    index = tree_index(tree or [])
    problems = []
    total = 0
    found = []
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
        # Both of these used to sit behind `if path == LOG`, and `LOG` is this script's own default
        # path -- so `--log SOMETHING` was precisely the invocation that skipped them. Measured 8 Nov
        # while checking #229's review: the same script, the same file,
        #   --log DECISIONS.md -> 3 problems, the fabricated checkpoint absent
        #   (no argument)       -> 5 problems, "D-0034 names checkpoint e2dbd46e..., not a commit"
        # A fabricated checkpoint is the rule an agent walking one branch's log most needs, and the
        # usage text promised it in this mode. There is no reason for either rule to depend on which
        # path the caller named: a checkpoint resolves against git whatever the file is called, and
        # the Makefile guard asks about a file in this checkout.
        for e in entries:
            where_seen.setdefault(e["id"], []).append(label)
        shas = sorted({e["fields"].get("Checkpoint", "").strip() for e in entries
                       if SHA.match(e["fields"].get("Checkpoint", "").strip())})
        problems += checkpoint_problems(entries, git_checkpoint_resolver(shas))
        cites = scan_citations(text, entries, index)
        for c in cites:
            c["label"] = label
        found += cites

    if walk_problem:
        # Fail closed, in the shape of every other unreadable input here: an unevaluated rule is not a
        # passed rule.
        problems.append("citations: %s" % walk_problem)
        judged, debt_note, measured = False, None, 0
        verdict = CitationVerdict([], 0, [], [], [], False, "the tree cannot be walked")
    else:
        # `POINTER_DEBT` counts the citations in THIS pair of logs. A tree whose logs are smaller than
        # the corpus that number was measured on is a different repository -- a fixture, a fork, an old
        # checkout -- and refusing it with a foreign constant reports a defect that does not exist.
        judged = judge_debt and debt_applies(total)
        verdict = citation_problems(found, read_tree, debt=POINTER_DEBT, judge_debt=judged)
        problems += verdict.problems
        measured = verdict.measured
        # One classifier, one verdict, and the headline reads the verdict's own fields. It used to call
        # `classify()` a second time for these two counts, which is how a run could print "the recorded
        # debt, which may only fall" beside a number the tree had just moved (#367 review #1).
        debt_note = ("not judged here: these logs hold %d entries and the recorded debt was measured "
                     "against %d, so the number is not a statement about this tree"
                     % (total, POINTER_DEBT_AT)) if judge_debt and not judged else None
        if judged and not verdict.judged:
            debt_note = ("not judged this run: %s, so the anchor count is a statement about this tree and "
                         "not about the logs -- fix the list above and the comparison comes back"
                         % verdict.unjudged_because)

    if report:
        for line in citation_report(found, read_tree, only=report_only, verdict=verdict):
            print(line)
        return 0

    if os.path.exists(os.path.join(ROOT, "Makefile")):
        problems += verify_invocation()
        # One id in two logs is not a duplicate to dedupe: it is a citation that points at two
        # decisions. Measured 6 Nov, when merging origin/main into the harness branch: `main` carried
        # D-0011..D-0015 as product decisions (a control's typeface, `:has()` reaching a tick box's
        # words) while the harness log had D-0011..D-0015 as harness decisions (the tick's `$PWD`, the
        # worktree cut, the deny keying), because a tick numbers from the highest id it can *see*, and
        # a tree cut before this file existed sees none of it. Both files now have their own id space,
        # so the only thing left to enforce is that they do not meet.
        problems += cross_log_problems(where_seen)

    if problems:
        print("\n%d decision-log problem(s):" % len(problems))
        for p in problems:
            print("  %s" % p)
        return 1

    print("check-decisions: %d entries across %d log(s), every one with its question, options, "
          "choice, reason and checkpoint" % (total, len(paths)))
    if judge_debt and debt_note:
        print("check-decisions: %d `path:line` citations checked for anchors, and the recorded debt is "
              "%s" % (len([c for c in found if c["kind"] != "not-a-path"]), debt_note))
    elif judge_debt:
        print("check-decisions: %d `path:line` citations resolve against this tree, %d of them owing "
              "an anchor and %d pointing somewhere a reader cannot open -- %s"
              % (len([c for c in found if c["kind"] != "not-a-path"]), measured,
                 len(verdict.unresolvable) + len(verdict.broken),
                 "the first number is the recorded debt, which may only fall" if verdict.judged
                 else "the ratchet was NOT judged this run: %s" % verdict.unjudged_because))
    else:
        print("check-decisions: %d `path:line` citations checked for anchors; the debt number was not "
              "judged, because --log narrows the corpus it is a count of"
              % len([c for c in found if c["kind"] != "not-a-path"]))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
