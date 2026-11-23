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

import bisect
import collections
import errno
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
# Repo-relative, because the directories below are named the same way and a caller that points the
# reader at a fixture tree (`--self-test`, and `factory/selftest.sh`'s behaviour legs) has to be able
# to join all four to a different root. `LOGS` stays absolute for every existing caller.
LOG_REL = ("DECISIONS.md", "factory/DECISIONS.md")
LOGS = tuple(os.path.join(ROOT, p) for p in LOG_REL)
LOG = LOGS[0]
MAKEFILE = os.path.join(ROOT, "Makefile")

# Phase 1 of one-file-per-entry (#429). The directories an entry can live in, beside the two monoliths.
#
# Both are optional *by design*: phase 2 is what starts writing them and phase 3 is what moves the
# existing entries, so until then a tree with no `DECISIONS/` is the normal tree. A reader that refused
# an absent directory would refuse every branch in flight, which is the opposite of what a reader change
# is for -- #429's premise is that the two layouts have to validate at once so no branch is invalidated
# when the writer changes. What is NOT optional is what a directory file must be: one entry, whose id is
# its filename (`layout_problems`), and an id no other file in the union already carries
# (`cross_log_problems`, which now sees directory entries as its own locations).
#
# The closing line still says `N entries across M log(s)` deliberately. Fifty-two entries in the two
# logs quote it with the number filled in, on 71 lines as of this commit -- `H-0065` to `H-0226` and
# `D-0059` to `D-0076` -- because it is the receipt each of those runs shipped with. Rewording it makes
# every one of them describe a line no gate ever printed -- the hazard H-0192 recorded when a
# correction rewrote a line another entry was quoting.
ENTRY_DIRS = ("DECISIONS", "factory/DECISIONS")
ENTRY_ID = re.compile(r"^[DH]-\d{4}$")

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

With no `--log` the run reads the union: the two monoliths, then any file filed in `DECISIONS/` or
`factory/DECISIONS/` -- one decision per file, named by its id, so `H-0220.md` holds `## H-0220 · …`.
Both directories are optional: until the existing entries are split out (#429 phase 3) an absent one is
zero entries, never an error, because a reader that refused a tree that has not migrated would refuse
every branch in flight. A file inside one is refused unless it holds exactly one entry and its filename
is that entry's id -- the filename is what turns two branches claiming one id into a git add/add
collision instead of a green merge that leaves both headings in place (#381). An id carried by a
directory file and by a monolith is a duplicate and is refused; a `--log` run cannot see that, since it
reads one file. The out-of-order rule stays what it has always been -- a comparison of headings inside
one file -- because in this layout the sequence is a filename, and filenames have no order a merge
respects.

--citations prints what the citation rule found instead of refusing on it: the unanchored
count by the file pointed at, the citations a reader cannot open, the anchored ones whose
text has moved, since #440 the ones whose anchor is satisfied by more rows than the citation
promised, and since #495 the ones whose quotation stops inside a token of the row it quotes.
It reads and changes nothing, and it is how the debt number is re-derived.

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

Being on the line answers only *is the text there*. The fragment also has to single that line out --
#440, an anchor satisfied by rows the citation did not promise verifies nothing -- and it has to stop
at a token boundary of the line it quotes -- #495, a quotation cut inside a word stays green when the
rest of the word is renamed, which is how five rows of `factory/DECISIONS.md` came to point at a
default that had been changed under them. Both are refused outright and neither is counted in
POINTER_DEBT: the row already carries a quotation, and finishing it is the whole repair.

A provenance pair is a commit token glued to a parenthesised pull-request reference: `` `cf00b44`
(PR #410) ``. #433 measured ``cf00b44`` (PR #427, `H-0235`) ride two review rounds while one `git
log` refuted it, and neither number was openable by any rule. Each pair now resolves in this clone
and is held to the commit's own message: the merge subject must name one of the asserted numbers,
or an asserted number must appear in the message. A correction quoting a false pair binds nothing
and is not refused; the sha has to sit on the parenthesis for the row to be making the claim.
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
        # Positional, and deliberately so -- but positional *within this file*. A directory of
        # one-entry files has no order a merge honours: git sorts nothing, and two branches adding
        # `H-0231.md` and `H-0232.md` land in either order depending on the reader. Sequence in that
        # layout is the allocator's job (it hands out the next number and journals the claim), so the
        # comparison a merge can violate is only ever the one between headings sharing a file (#429).
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


def layout_problems(label, entries):
    """That a file inside an entry directory is one entry, and that its id is its filename.

    Only files under `ENTRY_DIRS` are judged; a monolith takes none of this, because its whole shape is
    the opposite -- many headings in one file, which is the thing being migrated away from.

    Why the filename carries the id at all: it is the only move that makes two branches claiming one id
    collide *in git*. #381's experiment put a `merge=union` driver on the monolith and a same-id append
    merged green, leaving two `## H-0009` headings in one file -- and a merge here is permanent history
    (`allow_squash_merge=false`, `allow_rebase_merge=false`), so the union hides the one alarm the log
    exists to sound. Add/add on the same path is refused at the merge instead.

    The heading stays inside the file, so `parse` needs no second code path and a reader can still grep
    an id and land on it. Which is also why a disagreement between the two names is refused rather than
    reconciled: `H-0220.md` holding `## H-0221` would answer a citation to `H-0220` with a different
    decision, and answer it *silently*, because every reader that finds the file by filename and every
    reader that finds it by heading would then be reading two different entries.
    """
    problems = []
    parent = os.path.dirname(label.replace(os.sep, "/"))
    if parent not in ENTRY_DIRS:
        return problems

    name = os.path.basename(label)
    stem = name[:-3] if name.endswith(".md") else name
    at = "%s:1" % label

    if not ENTRY_ID.match(stem):
        problems.append("%s: %r is not an entry id. A file in %s/ is named by the decision it records -- "
                        "`## H-0001 · …` becomes `H-0001.md` -- and a name git can add/add against is the "
                        "whole reason for the layout" % (at, name, parent))
        return problems
    if len(entries) != 1:
        problems.append("%s: %d entry headings in one file under %s/. The filename is the id, so %d "
                        "headings alias %d decisions onto one path, and 'revert to before the entry' "
                        "names one file for all of them"
                        % (at, len(entries), parent, len(entries), len(entries)))
        return problems
    if entries[0]["id"] != stem:
        problems.append("%s: the heading says %s and the filename says %s. A citation read by heading and "
                        "a file read by name would land on two different decisions, and nothing else "
                        "here can see the pair" % (at, entries[0]["id"], stem))
    return problems


# The two `verify` recipe lines this file insists on, in the Makefile's own shape: `echo "..."` and
# the command share a line, so the guard below reads command chunks rather than whole lines. Each is a
# constant of its own so `RECIPE_BAD` can delete exactly ONE of them: the two clauses differ only by
# their argument, and #391 measured what a guard that cannot hold them apart fails to see.
SELFTEST_RECIPE = '\techo "== decision-log contract"; python3 scripts/check-decisions.py --self-test; \\\n'
GATE_RECIPE = '\techo "== decision log"; python3 scripts/check-decisions.py; \\\n'
RECIPE_GOOD = ('\t@set -e; \\\n' + SELFTEST_RECIPE + GATE_RECIPE +
               '\techo "== shellcheck"; $(MAKE) --no-print-directory check-shell; \\\n')

# The two sentences this guard prints, named here because #391's hole was a test that could only ask
# whether *something* was refused: the gate clause deleted, the self-test clause left behind, `verify`
# running no decision check at all, and the guard reading clean. `RECIPE_BAD` now asserts which shape
# earns which sentence, so the two assertions cannot quietly collapse back into one.
GATE_SENTENCE = ("Makefile: no recipe command runs `python3 scripts/check-decisions.py` without "
                 "`--self-test`; a gate whose invocation has been deleted, commented out, renamed, or "
                 "answered only by the self-test clause is a suggestion")
SELFTEST_SENTENCE = ("Makefile: nothing runs `python3 scripts/check-decisions.py --self-test`; the "
                     "assertions below then run only when someone remembers they exist")

# Seven shapes that each cost `verify` a step this file needs -- six by taking the step out, the
# seventh by piping it into `cat` so its exit status dies before the shell sees it. Every one is a
# `make verify` that runs less than it did and still reports success, and every one keeps the string
# "check-decisions.py" somewhere in the file, which is what a substring guard reads. The first four
# were reported by review on #217
# (5 Nov) as a guard satisfied by the text that removes it, and they are the shapes a person reaches
# for -- comment a step out, rename the file it calls, or replace the step with one that only mentions
# it. The last two are #391's pair, reported 14 Nov on the tree at `0673832` and re-measured on this
# branch's checkpoint in a copy outside the gated tree before anything was written here: the gate
# clause's command is a literal prefix of the self-test clause's, so a match that stops at the
# filename reads the self-test line as the gate line, and the bare gate step came off `verify` in one
# line with no red anywhere -- rc 0 and neither sentence, while the same probe with the SELFTEST clause
# removed refused with its own sentence.
#
# Each case names the sentences it must earn, and a shape earns a sentence exactly when the assertion
# that sentence states is false. Naming them, rather than insisting that SOME refusal happened, is the
# point of the pair: `("gate",)` is a Makefile whose self-test still runs, so a guard that answered
# with the self-test sentence -- or with both, the degenerate shape of one shared "is the command in
# the file" test -- has not told the reader which step is gone. `("gate", "self-test")` is a file that
# runs neither, which is what the first four are.
RECIPE_BAD = {
    "commented": ('\t@set -e; \\\n'
                  '\techo "== decision-log contract"; \n'
                  '#\tpython3 scripts/check-decisions.py --self-test; \\\n'
                  '#\tpython3 scripts/check-decisions.py; \\\n', ("gate", "self-test")),
    "renamed": (RECIPE_GOOD.replace("check-decisions.py", "check-decisions-OLD.py"),
                ("gate", "self-test")),
    "mentioned": ('\t@set -e; \\\n'
                  '\techo "== decision log (check-decisions.py)"; $(MAKE) --no-print-directory quiet; \\\n',
                  ("gate", "self-test")),
    # The one that survived the guard until 6 Nov: the whole step commented out at column 0, with the
    # invocation sitting after a `;` on the same line. `make verify` here runs nothing, and the file
    # still contains the command as plain text for any guard that reads lines instead of chunks.
    "commented-line": ('\t@set -e; \\\n'
                       '#\techo "== decision log"; python3 scripts/check-decisions.py --self-test; \\\n'
                       '#\techo "== decision log again"; python3 scripts/check-decisions.py; \\\n'
                       '\techo "== shellcheck"; $(MAKE) --no-print-directory check-shell; \\\n',
                       ("gate", "self-test")),
    # The two halves of #391, one clause each. Each file still contains BOTH invocations as text and
    # runs exactly one of them, so exactly one assertion is false and exactly one sentence is owed.
    # Asserted as a pair because the hole was one-directional on purpose: the self-test question was
    # already asked, and only the gate question could be answered by the very line that removed it.
    "gate-deleted": (RECIPE_GOOD.replace(GATE_RECIPE, ""), ("gate",)),
    "self-test-deleted": (RECIPE_GOOD.replace(SELFTEST_RECIPE, ""), ("self-test",)),
    # The seventh is #472's B1, measured on `8f7cd14`: nothing is deleted, the command still runs,
    # both patterns still match the chunk -- and `| cat` decides the chunk's exit status, so a gate
    # that fails ends the recipe 0 and `verify` never knows. The same shape is refused in words by
    # the arm `check-denies.py` and `check-doc-labels.py` carry, which this file's guard is "copied
    # in shape from". The owed set names the refusal that arm prints as well as the gate's, so a
    # guard that counted the piped chunk as the step (or printed only the old sentence) is refused.
    # The entry `H-0311` names the review that measured all of this, and refuses both fakes: a
    # guard that counted the piped chunk as the step, or printed only the old sentence.
    "gate-piped": (RECIPE_GOOD.replace(
        GATE_RECIPE,
        '\techo "== decision log"; python3 scripts/check-decisions.py | cat; \\\n'),
        ("gate", "status")),
}


# The two invocations the guard is asked about, as two patterns rather than one. `INVOCATION` still
# matches the self-test line by design -- the word boundary bars a longer FILENAME, not a longer
# argument, and a chunk that opens with the bare command IS that command with an argument -- so the
# caller has to look at which pattern answered before it counts anything. Trying `SELFTEST_INVOCATION`
# first is how `check-denies.py` and `check-doc-labels.py` already order the same pair.
INVOCATION = re.compile(r"python3[ \t]+scripts/check-decisions\.py(?![^\s])")
SELFTEST_INVOCATION = re.compile(r"python3[ \t]+scripts/check-decisions\.py[ \t]+--self-test(?![^\s])")

# The third sentence this guard prints, named beside the other two because it refuses a third thing:
# the invocation is present, matched, and about to be counted -- and cannot fail the recipe. A gate
# piped into `cat` is the "is a suggestion" GATE_SENTENCE describes, arrived at by a different road,
# and the reader is owed which repair fits which shape.
STATUS_SENTENCE = ("Makefile: `%s` runs where its exit status cannot reach the shell -- a "
                   "command in an && or || list, or a pipeline, can fail and the recipe "
                   "still ends 0, so this step can never redden `verify`")


def _status_reaches_shell(chunk, end):
    """Whether a non-zero exit from the invocation ending at `end` can end the recipe.

    Ported from `check-denies.py`, which carries the same function and measured on `b5fd0ca` that
    `A --self-test && B;` under `set -e` prints nothing and exits 0 when A fails, because errexit
    ignores every command of an AND-OR list but the last. So "the text is in the recipe" and "the
    recipe fails when this fails" are two different claims, and only the second one is worth a
    gate. Anything after the invocation in the same chunk -- `&&`, `||`, a pipe -- means something
    else decides the status; anything before it means the invocation is last, which is the position
    that counts. #472's review measured this guard shipping without the arm: the piped gate matched,
    was counted as the step, and read clean -- while `make` on `false | cat; \\` under `set -e`
    printed `recipe still ends 0` at rc 0.
    """
    tail, head = chunk[end:], chunk[:end]
    if "||" in tail or "&&" in tail or "|" in tail:
        return False
    if "|" in head:
        return False
    return True


def invocation_problems(recipe):
    """Given the text of a Makefile, say whether `verify` still runs this script and its self-test
    -- and whether a failure of either could actually fail `verify`.

    A filename substring is the wrong assertion: it is satisfied by a comment, by a commented-out
    recipe line, and by an `echo` that names the script while running something else. So the anchor is
    a *command chunk* -- the text between `;` on a recipe line -- that starts with the interpreter and
    names `scripts/check-decisions.py` as one whole word, and the word boundary bars a longer filename
    that merely starts the same way. The shapes in `RECIPE_BAD` are asserted against this function in
    `--self-test`, so a later edit that weakens the match has to delete those cases as well.

    Each chunk is counted for its OWN question, which is the half #391 measured missing. The gate
    clause runs the same command as the self-test clause and differs from it only by the argument that
    follows, so a guard that matched a chunk and then asked "did we see the command?" let the
    `--self-test` line answer for the gate: deleting the bare gate step from `verify` refused with
    neither sentence, while deleting the self-test step refused with its own. The gate question is now
    "a chunk that runs this script and is not the self-test", so each deletion answers with its own
    sentence and a Makefile missing both says both.

    A comment is tested at the *line*, before the split, not at the chunk. Reported by review on #217
    at 869564a: with the step commented as `#\techo "== decision log"; python3
    scripts/check-decisions.py; ` followed by a line-continuation backslash, make hands that line
    to no shell at all, but splitting on `;` first
    put the invocation in chunk two where a `chunk.startswith("#")` test could not see it -- so both
    this gate and its own self-test returned 0 while `make verify` ran neither command. The premise the
    docstring used to carry, that a chunk beginning with `#` cannot match, is what the shell refuted.

    A matched chunk must also be in a position to fail the recipe: #472's review measured that a
    gate piped into `cat` -- `python3 scripts/check-decisions.py | cat; \\` -- ran, matched, and read
    clean, while `make` reported rc 0 after its failure, because the last command of a pipeline owns
    its status. `_status_reaches_shell` is the same arm `check-denies.py` and `check-doc-labels.py`
    already carry; a refusal that names a chunk it cannot redden is a gate in name only, so such a
    chunk counts toward neither owed sentence and earns `STATUS_SENTENCE` instead.
    """
    problems = []
    found = {"gate": False, "self-test": False}
    for raw in recipe.splitlines():
        if raw.lstrip().startswith("#"):
            # make never hands this line to a shell; nothing on it runs, including whatever follows a `;`.
            continue
        for chunk in raw.split(";"):
            chunk = chunk.strip()
            if chunk.startswith("#"):
                continue
            match = None
            if SELFTEST_INVOCATION.match(chunk):
                match, kind = SELFTEST_INVOCATION.match(chunk), "self-test"
            elif INVOCATION.match(chunk):
                # The gate clause, and anything else that runs this script for real: this is the chunk
                # whose exit status is the decision gate. #391's shape is a file where this arm never
                # fires while the arm above does, and that used to read as both assertions holding.
                match, kind = INVOCATION.match(chunk), "gate"
            if match is None:
                continue
            if not _status_reaches_shell(chunk, match.end()):
                # #472's B1: text that runs where its status cannot reach the shell is decoration,
                # not a step, so it counts toward neither owed sentence -- and it is refused by name,
                # because "deleted" and "decorated" are different repairs for a reader to make.
                problems.append(STATUS_SENTENCE % chunk)
                continue
            found[kind] = True
    if not found["gate"]:
        problems.append(GATE_SENTENCE)
    if not found["self-test"]:
        problems.append(SELFTEST_SENTENCE)
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


def _nested_markdown(directory, prefix):
    """Every `*.md` below `directory`, named from the repository root. Refuses a layout the reader can't judge.

    Deliberately `os.walk` and deliberately exhaustive: the failure being guarded is an entry that
    exists on disk and is validated by nothing, and a census that stops at the first level would report
    the tidy case while the deep one sits there unread. The name is built from `prefix`, the entry
    directory plus this subdirectory, rather than from a path relative to `directory` -- a refusal that
    names `deep/H-0009.md` tells a reader to move a file they cannot find from the repository root, and
    the instruction they can act on is the one that says `DECISIONS/deep/H-0009.md`.
    """
    deep = []
    for dirpath, dirnames, filenames in os.walk(directory):
        dirnames.sort()
        for name in sorted(filenames):
            if name.endswith(".md"):
                deep.append("%s/%s" % (prefix, os.path.relpath(os.path.join(dirpath, name),
                                                               directory).replace(os.sep, "/")))
    return deep


def entry_dir_files(root=None):
    """The entry files inside `ENTRY_DIRS`, repo-relative, as `(paths, problem)`.

    `os.scandir`, not `git ls-files`, for the reason `tree_files` gives: the gate answers the tree in
    front of it -- identically in CI, in the review loop and at the merge, reading no refs -- so a file
    a unit has just written is a file the gate has to see before it is committed anywhere.

    An absent directory is not a problem and says so by returning nothing: phase 1 lands while every
    entry is still in a monolith, and refusing that tree would refuse the repository (#429). A
    directory that cannot be *opened* for any other reason is a problem, in the shape `tree_files` uses
    -- an unevaluated rule is not a passed rule, and ENOENT is the only errno here that means "there are
    no entries yet". Sorting is not decoration: the log's order is a reading order, and entries added by
    one run should print in id order rather than in whatever order the directory returns them.

    Only `*.md` at the top level is taken. A subdirectory inside `DECISIONS/` is a layout this reader
    has no rule for -- there is no id that names a path segment -- and a file the reader silently skips
    is an entry that validates nowhere, which is the exact failure #429 phase 3 must not arrive at
    quietly. `factory/selftest.sh`'s `decisions_unseen_entry_file` leg puts that shape in front of the
    reader so it cannot become invisible by accident.
    """
    root = root or ROOT
    paths, nested, problem = [], [], None
    for rel in ENTRY_DIRS:
        directory = os.path.join(root, rel)
        try:
            with os.scandir(directory) as handle:
                found = sorted(handle, key=lambda e: e.name)
        except FileNotFoundError:
            continue
        except OSError as exc:
            if exc.errno == errno.ENOENT:
                continue
            if problem is None:
                problem = ("%s cannot be listed (%s), so any entry filed there was not read and the "
                           "duplicate-id rule did not run over it" % (directory, exc))
            continue
        for entry in found:
            try:
                is_dir = entry.is_dir()
            except OSError as exc:
                # a dangling link, or a tree that moved mid-read: not "no entries"
                if problem is None:
                    problem = ("%s could not be examined (%s), so the reader cannot say what entries "
                               "it holds" % (entry.path, exc))
                continue
            if is_dir:
                nested += _nested_markdown(entry.path, "%s/%s" % (rel, entry.name))
            elif entry.name.endswith(".md"):
                paths.append("%s/%s" % (rel, entry.name))
    if nested and problem is None:
        problem = ("an entry file cannot live below %s/: %s. No id names a path segment, so both the "
                   "filename rule and the duplicate-id rule would be skipped for it -- move it up "
                   "beside the others"
                   % (" or ".join(ENTRY_DIRS), ", ".join(sorted(nested)[:5])))
    return paths, problem


def log_paths(root=None):
    """Every file the union reads, absolute, in reading order, as `(paths, problem)`.

    The two monoliths first -- that is the order this file has always read in, and the closing tally
    line prints the count -- then the entry files. `main` calls it only for the default invocation:
    `--log PATH` narrows to exactly one file, which is the one thing about `--log` that narrows (its
    USAGE text promises the duplicate-id rule runs against that file alone).
    """
    root = root or ROOT
    extra, problem = entry_dir_files(root)
    return [os.path.join(root, p) for p in LOG_REL + tuple(extra)], problem


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
# `sha` (PR #N) provenance pairs (issue #433)
# ---------------------------------------------------------------------------------------------

# A backticked-or-bare commit token glued -- whitespace only, and it may wrap a line -- to a
# parenthesised pull-request reference. The window is literally `\s*`, so it spans a blank line as
# well as a wrap: a sha at the foot of one paragraph and a `(PR #N` at the head of the next ARE bound.
# That is the wide edge of "glue is the claim", stated here rather than discovered later -- #498's
# review noted `H-0351` described it as "wraps", which under-states it by a paragraph. Narrowing it to
# `[ \t]*\n?[ \t]*` would refuse nothing today and buy nothing; the corpus answer is in `H-0355`. The binding is the claim: "`<sha>` (PR #N)" says the named
# commit IS the landing of the named pull request. Glued is deliberate: #433's own specimen of a
# misleading attach (`620bcb1` (PR #403 merged: ...)` -- the base's landing was #402, #403 merged
# PAST it) binds because it is glued, and a looser window would re-aim at whatever `PR #N` the
# sentence happens to contain next, which is a guess about intent this file has refused to make
# since `scan_citations` inherited within the sentence and no further. A bare run of hex has to
# stand alone on both sides so no word tail (`issue-303`) or longer object name half-matches;
# `(PR #341/#432)` asserts both numbers, and any one of them satisfying the commit satisfies the
# row -- the multi-number spelling names one landing from two angles (the pull and the issue
# branch it landed), and requiring all of them would refuse `H-0239`'s honest "`883b5e8`
# (PR #341/#432)" on a technicality.
PROV_PAIR = re.compile(
    r"(?:`([0-9a-f]{7,40})`|(?<![0-9A-Za-z./_-])([0-9a-f]{7,40})(?![0-9A-Za-z_-]))"
    r"\s*\(\s*PR\s+#(?P<nums>\d{1,5}(?:\s*[/,]\s*#\d{1,5})*)")
PROV_MERGE = re.compile(r"^Merge pull request #(\d+)")


def parse_pair_batch(shas, raw):
    """The records of one `git cat-file --batch` reply, framed in BYTES, keyed by the sha asked for.

    Split out of `git_pair_message_reader` so both of its arms are pinned by `--self-test` on
    synthetic batch bytes -- toolchain-free, git-free, the same fixture pattern the rule already uses
    -- which leaves the subprocess call as the only line of the reader a self-test cannot reach.

    `None` means the stream cannot be trusted and the whole batch goes unanswered with it, exactly as
    in the reader: a desynced stream cannot answer any later row either, and a silent short map would
    be a fail-open default one level down.

    The framing is done in bytes because the number a header announces IS a byte count, and a
    character-decoded body is shorter than that whenever the message is not ASCII: measured on this
    repository's own history, `1c385b7` announces 1364 and delivers 1360 characters -- its quoted
    pull-request title carries an en dash and an em dash, three bytes and one character each, so a
    parser that slices characters by that number comes up four short and answers `None` for a batch
    git replied to perfectly (H-0355, the review of #498, which
    ran the *true* pair `1c385b7` (PR #411) through the shipped gate and got rc 1, "the provenance
    rule did not run", about a merge whose subject reads `Merge pull request #411`). The header's name
    field is checked against the token that was asked for for the same reason in the other direction:
    without it, a stream that desynced re-parses three bytes into the next header and the row gets
    graded on bytes belonging to its neighbour with no signal at all. Records arrive in request order
    -- `--batch` answers one per input line forever, and ordering is the only key that works: the row
    asserts the ABBREVIATED token (`cf00b44`) while the header carries the full name, so a map keyed
    on the printed sha answers every abbreviated lookup with "unreadable" (measured on an earlier
    shape of this reader when it keyed on `%H`: all fourteen pairs in the shipped log refused "could
    not be read" at once). Parsing by SIZE rather than by scanning for a terminator keeps the stream
    in sync even when message text looks like a header.
    """
    messages = {}
    rest = raw
    for asked in shas:
        nl, sep, rest = rest.partition(b"\n")
        if not sep:
            return None
        fields = nl.split(b" ")
        if len(fields) == 2 and fields[1] == b"missing":
            messages[asked] = None
            continue
        if len(fields) != 3 or fields[1] != b"commit" or not fields[2].isdigit():
            return None
        if not fields[0].decode("ascii", "replace").lower().startswith(asked.lower()):
            return None
        size = int(fields[2])
        # `+ 1` and the trailing 0x0A: git puts a newline after each record's bytes, and a record that
        # does not end where its own header said it would is a desync, not a short read to ignore.
        if len(rest) < size + 1 or rest[size] != 0x0A:
            return None
        content, rest = rest[:size], rest[size + 1:]
        messages[asked] = (content.split(b"\n\n", 1)[1].decode("utf-8", "replace")
                           if b"\n\n" in content else "")
    return messages


def git_pair_message_reader(shas):
    """The full commit message of each named commit, in one batch call.

    `None` means the question could not be asked -- no git, a wedged `git cat-file`, a timeout --
    and is distinguishable from `{}` exactly as in `git_checkpoint_resolver`: an unevaluated rule
    is not a passed rule. Only shas the existence resolver already called commits are asked for, so
    a bad object should be impossible here; one that vanishes between the two calls, or answers
    with a type that is not commit, lands as `None` under its own key and is refused, not skipped.
    The subcommand is `cat-file --batch` on purpose: `the gate reads no branch refs`
    (`factory/selftest.sh`, #253) refuses any gate that asks git for anything but object lookups,
    because a verdict that changes with the last fetch reads as flaky and gets re-run until it
    agrees -- and `git log`, even `--no-walk`, is that first measurement's failure shape, caught
    here by that leg on the first `make verify` of this rule.

    The reply is read as BYTES, not text: `text=True, errors="replace"` would hand the framer a
    string whose length is a character count while every header it must trust counts bytes, and that
    is the desync `parse_pair_batch` exists to refuse. Byte mode also keeps
    `scripts/check-mutations.py`'s `unencoded_opens` leg quiet, which grades opens without an
    encoding -- binary is exempt there, and a `text=True` open is a claim about an encoding this
    reader deliberately does not make.
    """
    if not shas:
        return {}
    import subprocess
    try:
        out = subprocess.run(
            ["git", "cat-file", "--batch"],
            input=("\n".join(shas) + "\n").encode("utf-8"),
            capture_output=True, cwd=ROOT, timeout=30)
    except (OSError, subprocess.SubprocessError):
        return None
    if out.returncode != 0:
        return None
    return parse_pair_batch(shas, out.stdout)


def provenance_pair_problems(text, entries, resolve, message_of):
    """That every `` `sha` (PR #N) `` pair names a commit whose own message agrees it is PR #N's landing.

    Neither class of number #433 measured entered any rule this script had: a pull-request number
    is not a `path:line` citation and not a `Checkpoint:` field, so `H-0237` shipped
    ``cf00b44`` (PR #427, `H-0235`) through two review rounds with rc 0 while `git log -1
    --format='%s' cf00b44` -- one command, available at that same head -- answered `Merge pull
    request #410`. Both injectables are the `checkpoint_problems` pattern, so the self-test asserts
    the refusal, the pass and both blind answers without git: `resolve` maps sha to "is a commit"
    (mapping or callable), `message_of` maps sha to the full message; either being `None` means git
    could not be asked, and the rule reports that it did not run rather than passing silently.
    """
    pairs = list(PROV_PAIR.finditer(text))
    if not pairs:
        # Nothing to ask, so nothing to claim -- the same boundary `checkpoint_problems` draws:
        # a log with no pairs must not be refused for a git problem it does not have.
        return []
    # `label_of` carries a line number; the pairs bring their own, so the label is the file half.
    label = entries[0]["label"] if entries else "<log>"
    if resolve is None:
        return ["%s: git could not be asked whether the commits named in `(<PR #…` provenance pairs "
                "exist, so nothing can be claimed about them -- the provenance rule did not run. "
                "(A missing git, a wedged `git cat-file`, or a tree this is not a repository of.)"
                % label]
    if message_of is None:
        return ["%s: the commits named in `(<PR #…` provenance pairs exist, but git could not be "
                "asked for their messages, so nothing can be claimed about the pairs -- the "
                "provenance rule did not run." % label]

    def get(mapping, sha):
        return mapping(sha) if callable(mapping) else mapping.get(sha)

    problems = []
    for m in pairs:
        sha = m.group(1) or m.group(2)
        nums = re.findall(r"\d+", m.group("nums"))
        line = text.count("\n", 0, m.start()) + 1
        where = "%s:%d" % (label, line)
        entry = entry_of(entries, line)
        if not get(resolve, sha):
            problems.append(
                "%s: entry %s asserts `%s` (PR #%s), but %s is not a commit in this repository -- "
                "a provenance pair nobody can open is the claim with the receipt missing, which is "
                "what #433 prices" % (where, entry, sha, "/".join(nums), sha))
            continue
        msg = get(message_of, sha)
        if not msg:
            problems.append(
                "%s: entry %s asserts `%s` (PR #%s), but the commit's message could not be read, so "
                "the pair is unverified and unverified is not passed"
                % (where, entry, sha, "/".join(nums)))
            continue
        subject = msg.splitlines()[0] if msg else ""
        mm = PROV_MERGE.match(subject)
        if mm and mm.group(1) in nums:
            continue
        if any(re.search(r"(?<!\d)%s(?!\d)" % n, msg) for n in nums):
            continue
        problems.append(
            "%s: entry %s asserts `%s` (PR #%s), but the commit's own message says otherwise: %r. "
            "`git log -1 --format='%%s' %s` was one command at the head that shipped it and still "
            "is -- #433 measured `(PR #427)` ride two review rounds for the merge that is PR #410's "
            "landing; re-aim the number, or attach the parenthetical to what it really describes"
            % (where, entry, sha, "/".join(nums), subject, sha))
    return problems


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
# 539, re-measured on 14 Nov 2026 after the #367 review repairs, by `--citations | tail -1`. 535 + 4, and
# all four are the SAME defect being paid for rather than four new promises: they are the `:NNN`
# continuations that re-resolved their parent's basename, got two candidates back, and fell out as
# `not-a-path` -- checked by nothing, so they were in neither the debt nor the refusal. Making a citation
# that names no file a defect (H-0171) is what surfaces them, and the rise is recorded beside the entry
# that measures it, which is the only direction this constant may move up.
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
# Re-measured on 15 Nov with the debt at 546: 248 entries across the two logs. The seven that arrived
# since 539 are the fresh coordinates `factory/DECISIONS.md`'s H-0200 writes about its own effect --
# three of them carry their quote and are anchored, four name a line and no text -- counted by
# `--citations` and recorded here in the same commit as the entry, per the AGENTS.md rule that a merge
# which grows the logs re-measures rather than trims. This very line was the hazard it names: writing
# it slid five older pointers in `scripts/check-decisions.py`, repaired in the same commit -- three by
# digit (H-0171 x2, H-0172, quotes verified at the new lines) and two by revision pin (H-0176, H-0177
# re-read as `35cd0ea:scripts/check-decisions.py:586`, because the 539 they measured no longer exists
# in the file and a pin, unlike a re-date, states which tree they measured).
POINTER_DEBT = 589  # citations that owe an anchor; may only fall (H-0361 (issue #440; numbered H-0353 until the yield H-0362) re-measured 583 -> 589 beside the entry that records it, a legal +6 adoption measured at the head that ships this line: the #440 continuation repair -- a revision-qualified coordinate now fills its file into the continuation store when the path half resolves uniquely, where before a pinned row handed nothing to the bare `:NNN` rows after it and left them `not-a-path`, checked by nothing -- adopted 6 rows, all named: rows 8404 (the `factory/factory.sh` `:919` of `H-0167`), 8405 (`:632` and `:665`, same entry) and 13800 (`factory/selftest.sh` `:402`, `:725` and `:1304`, `H-0253`). No claim is new: each row already wrote the coordinate, `factory/factory.sh` and `factory/selftest.sh` are untouched at this head, and a row that was invisible is now guarded. The 123-indeterminate repair itself moved no debt: lengthened rows stayed anchored and pinned rows were never in the debt number -- the indeterminate bucket was a refused list, not the counted one; H-0308 (issue-389 review round 1) pays back the 4 that H-0307 booked as a rise, 587 -> 583, because that rise was recorded against an enforcement the shipped gate did not provide: the review of #471 at 2026-11-19T14:57:56Z ran `--citations` at 83a428e and found all four of H-0307's coordinates in the UNANCHORED bucket -- log rows 15489, 15490, 15531 and 15614, pointing at 2907, 2908, factory/selftest.sh 16162 and 15255 -- while H-0307's `Effect:` and this comment both called them anchored on the text they quote. Cause: `ANCHOR_AFTER` reads a double-quoted fragment flush against the coordinate's closing backtick, up to three spaces or apostrophes of gap, and `(`H-0093`) says` was sitting in that gap. Measured both directions on probe copies before anything was repaired: editing 2907's 54 to 55 on the shipped bytes answered rc 0 and silent -- the refusal the entry promised did not fire -- and the same edit beside one flush anchor answered rc 1 naming the row. The four now carry flush fragments on their own physical lines, so the ratcheted number is 583 on the tree that ships this line and the histogram `--citations` prints is `factory/selftest.sh` 49 and `factory/DECISIONS.md` 19: the 50 and 22 of the 587 reading, less the four. What that 587 clause itself claimed for the pair was `factory/selftest.sh` 50 and `factory/DECISIONS.md` 21, and 0 rows in the unanchored shape -- 22 is what the tree at 83a428e prints, and 4 rows had arrived in exactly the shape it denied, so both numbers are corrected here rather than inherited; the rise H-0307 recorded was legal beside its entry, and the word `anchored` in that clause was not true of the tree it was written on; H-0284's third base sync onto `b433f01` re-measured 583 on the merged pair with this entry out of the log and 583 again with it in -- 1141 citations before, 1147 after, the entry's six all anchored on their own physical line -- so the number trunk composed for trunk's corpus is verified true of the merged corpus, which is the claim the auto-merge of this file made without being asked; H-0282's second base sync onto `feb3b16` re-measured 583 on the merged corpus and the fall from 584 is row-composed, not inferred: the --citations histograms of the HEAD pair and the merged pair differ by exactly one open row -- `factory/DECISIONS.md:8469` (H-0169) `docs/factory.md:205` UNANCHORED here, re-aimed by trunk to `docs/factory.md:204` anchored there -- so the merged corpus counts that row exempt, and 584 - 1 = 583 with no other row moving across the boundary; the revision-qualified bucket grew 85 -> 89 by the union's four new revision-naming rows, none of which was ever open; H-0279's sync re-measured 584 on the merged pair and the fall from 586 is composed, not inferred: 582 open rows carried in, +2 from the rows H-0139 and H-0146 became when the #374 class refused their trunk-re-quoted present tense and they were past-tensed with the date -- the first refusals the shipped gate filed against this repository's own oldest entry; the 27 anchored drifts that merge re-aimed by the gate's named line moved no debt because they were anchored throughout; H-0268's union merge re-measured 586 on the merged pair, and the fall from this branch's 588 is composed, not inferred: this branch's logs run through the merged scanner answer 586 unanchored + 2 wrapped, and the 2 are the newline-cut anchors #369's bucket separates from the ratchet -- both live rows trunk fixed, so the merged corpus answers wrapped=0 and the 2 stop being debt without any promise being unmade; the open-row multiset diff of the same two corpora, re-measured at 12ddbc4 by H-0270 with this head's own scanner over both pairs of logs, holds 32 pure citing-line renames (14 at +2, 18 at +123) and 1 addition -- open rows 175 -> 176, the added row is main's own H-0256 Follow-up citing a bare :12339, 0 open rows lost -- and the unanchored-debt diff of the same pair is 74 rows each side over 586 on each: 5 pure renames all +2 (H-0200's :3187 row 10695->10697 and its two factory.sh rows 10697->10699, H-0210's selftest.sh:263/265 rows 11325->11327) plus 69 rows whose coordinates this merge re-aimed, entry-balanced, 0 lost 0 gained; two of the four pairs this comment formerly named, 10643->10645 and 10656->10658, name no open row in either corpus -- they are two of those re-aims, H-0200's rows at factory/common.sh 1435->1468 and 1554->1587; H-0254's row-by-row read of the review corrections of H-0253 said +11 citations and NO debt move; H-0255's multiset diff of the same two trees answers +12 -- the missed row was H-0254's own quotation of this constant: 2 self-qualify (1 revision-qualified, 1 anchored on its own quoted line), 10 land in the open shape the classifier does not count; H-0250: 546 -> 588, the 52 newly-aimed continuation rows this rule adopts, measured +52 at 3d8ff711 and every one listed in factory/DECISIONS.md)

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
# Does the left half name a FILE? An extension is the only marker that survives both spellings:
# `triage.sh`, `./DECISIONS.md`, `internal/web/handlers.go` all answer to it, and `pr`, `unclaimed`,
# `STATE_PROPS`, `D-0033`, `verify` and `H-0153` all refuse it. It is what separates "a path whose file
# has gone" from "a triage class, a symbol, an entry id" now that a bare name counts as a path too
# (#367 review #2): `gonee.md:3` is a promise about a file, and the promise is false when no file answers
# to it, wherever the entry meant it to live.
FILENAME_SHAPE = re.compile(r"[\w.+-]*[A-Za-z0-9][.][A-Za-z][A-Za-z0-9]*$")
# The anchor: a double-quoted fragment that follows the citation with nothing between it and the
# closing backtick but spaces and an apostrophe -- up to three characters of them, so `` `x.md:9` "…" ``
# and `` `x.md:9'"…` `` both read. What it does NOT admit is the `s` of a possessive: the corpus shape is
# `` `README.md:588`'s "…" ``, four characters of gap, and this class declines it. That is measured, not
# oversight -- see `NEAR_QUOTE` below for what widening would and would not buy. Anything further away is
# another sentence's quotation and belongs to a different claim. Sixteen characters is the shortest fragment
# that cannot be an accident: `the`, `verify` and `Chose: a` match half the tree, and an anchor that
# matches anything is a check that cannot fail.
# Where a citation's sentence ends, for the continuation rule in `scan_citations`. Measured on the two
# logs rather than designed (#365): the break set is a sentence-final `.` followed by whitespace or the
# end of the text, plus a markdown block marker opening a line -- bullet, blockquote, enumeration,
# heading. Against the physical-line rule, on the sync-merge tree `adddacc` counting occurrences, this
# aims 52 wrapped continuations the line rule left invisible (95 aimed -> 137, 199 -> 157 unplaced, of
# 294 bare rows) and drops the aim of 10 rows that reached their file only by riding an earlier
# sentence's citation -- among them the four the issue named (`H-0110`'s `:2766` and `:3731`,
# `H-0111`'s `:2853` and `:3833`). No row is re-aimed at a different file. Keyed by
# (file, line, low, high) the dropped 10 are 9: one coordinate sits twice on a single log line and
# only the row aimed at that file's own reader falls there, the pair's twin keeps its aim under both
# rules. The dropped rows sit in the open shape `scan_citations` documents -- their sentence names the
# intended file as a bare backticked path with no coordinate, or names no file at all -- and admitting
# that bare path was refused on the row it mis-aims: `:760` of `D-0038` lands on
# `docs/screenshots/360/lock.png`, the nearest path, not the `manifest.py` the sentence runs on about.
# Wider terminators were refused on their own rows too: `;` un-aims 17 for 36 of its 52 gains;
# adding ` -- ` un-aims 16 for 42, including the ticket's own two-coordinate shape.
# Narrower means honest orphans; wider means rows read as checked while aimed at the wrong file.
SENTENCE_BREAK = re.compile(r"\.(?=[ \t\n]|$)|(?<=\n)[ \t]*(?:[-*#>]|\d+\.)(?= )")

ANCHOR_AFTER = re.compile(r"^[ ']{0,3}\"([^\"]{16,})\"")
MIN_ANCHOR = 16
# A quotation that is close to the coordinate but not flush against it, or flush and too short. The rule
# declines both, and a refusal that only says "has no anchor" sends the author to look for a rule they
# cannot see: they wrote a quotation, and the sentence that declines it never mentions the separator or
# the length. This regex exists so the refusal can name what it declined. It is NOT a second anchor rule:
# nothing here is judged by it, and widening `ANCHOR_AFTER` to what it matches was measured and refused
# (H-0170: three pointers in other units' entries change class for a 0.6% dent in a debt of 535).
NEAR_QUOTE = re.compile(r"^(.{0,6})\"([^\"]+)\"")
# The #369 exit, now a stated refusal rather than a silent one. These logs are markdown wrapped at
# ~110 columns, so a faithful anchor outlives its physical line in two shapes: the fragment OPENS
# in anchor position and the closing quote is on the next line (`ANCHOR_DANGLING` matches the tail,
# unclosed), or the whole fragment starts the next line (`ANCHOR_AFTER` aimed at the following line
# matches it). Neither is an anchor -- the rule reads one physical line -- but both used to leave the
# row as unanchored debt with no refusal naming the newline, which is what the writers of H-0171 hit
# twice and the `H-0179` merge entry hit five times. `scan_citations` flags both so `classify` can
# refuse by name; a tail that ends inside an open code span is neither shape (a command wrapped
# across lines is shell, not a cut quotation -- the `H-0142` row is that, and it stays debt).
ANCHOR_DANGLING = re.compile(r"^[ ']{0,3}\"[^\"]*$")
# The CURRENT-STATE citation (#374): a pointer whose sentence asserts where the lines ARE now --
# "`AGENTS.md:46` now reads", "the function is at `factory/factory.sh:245`", "the rule sits at `:437`".
# This is the one shape the anchor rule left unguarded: the row reads as a statement about the tree
# being gated, the next merge to the cited file makes it false, and every gate stays green while a
# reader follows the number to the wrong sentence. Measured 18 Nov: `H-0139`'s corrective bullet,
# written 11 Nov against `5be86e5`, said the `Chose: escalated` instruction "is at `AGENTS.md:46`";
# today that sentence sits at `AGENTS.md:51-52`, `make verify` reads green over the gap. Past-tense
# coordinates age honestly -- "was at `:713` on `9a3b0aa`" -- they name the tree they measured, so
# they join the counted debt like any other unanchored pointer instead of rotting in a must-be-zero
# bucket. Present-tense ones must carry the anchor like a new citation or be reworded into the past
# with the date they were read; that is what lets the counted number FALL, which #374's title says
# the ratchet never obliged it to do.
# A verb marks THIS row only when it stands ADJACENT to the coordinate, not merely near it -- the
# verb and one small connective, at either side of the citation's backticks. Two orders exist in the
# corpus and in #374's own quotations: "`X:38` now reads \"- Checkpoint...\"" (verb after, quoted
# text after that) and "the `needs-human` sentence quoted above is at `AGENTS.md:46`" (verb before).
# A bare verb-regex over the coordinate's line plus one was written FIRST and measured against the
# two logs: it mis-sorted exactly the rows the issue warned prose-matching "will mis-sort" on --
# "`docs/factory.md` carrying" pulled by "row now reading" three clauses away (H-0116), "citations
# would point at a gap `main` says is permanent" (H-0105). Adjacency keeps every row the issue
# names as the class, and the self-test pins both mis-sort controls against the shipped matcher.
# Every verb group opens with a word boundary: the first false positive of the shipped matcher was
# this entry's own sentence "one bare at `file:3113`", where "bare at" carries "are at" as a
# substring -- and "this at" carries "is at". A guardless alternation refuses English.
_NOW_AT = r"\b(?:now at|is at|are at|sits at|lives at|points at)"
_NOW_SAY = r"\b(?:now reads|still (?:holds|reads|names))"
_NOW_MV = r"\b(?:re-?pointed to|moved to)"
# the object a marker must reach: a line coordinate (`:713`, bare 941, `**`:308`**`, `**941**`) or a
# quoted/backticked text fragment ("look:", `factory/triage.sh:336`).
_NOW_OBJ = r"(?:\*\*)?(?:`[^`]+`|\*{1,2}:?[0-9]{2,4}(?:-[0-9]{2,4})?\*{1,2}|[:][0-9]{2,4}|[0-9]{2,4}(?:-[0-9]{2,4})?)(?:\*\*)?"
_NOW_TXT = r"(?:\*\*)?(?:\u201c[^\u201d]{2,}\u201d|\"[^\"]{2,}\"|`[^`]{2,}`)"
# forward, read from the closing backtick: verb then its object. The at-family may appear bare
# ("is", "are") because the coordinate itself carries the "at" the verb needs: "...quoted above is
# at `AGENTS.md:46`" has the full verb BEFORE the earlier coordinate and a bare predicate here.
NOW_FWD = re.compile(r"\s*\*{0,2}\s*(?:"
                     + _NOW_AT + r"\s*\*{0,2}\s*" + _NOW_OBJ
                     + r"|" + _NOW_SAY + r"\s*\*{0,2}\s*" + _NOW_TXT
                     + r"|" + _NOW_MV + r"\s*\*{0,2}\s*" + _NOW_OBJ
                     + r"|,?\s*(?:still|then|now)\s+(?:at|reads?)\s*\*{0,2}\s*" + _NOW_OBJ
                     + r"|\n\s*(?:" + _NOW_AT + r"|" + _NOW_SAY + "|" + _NOW_MV
                     + r"|\b(?:is|are)\s+at)\b)")
# reverse: the verb ends the text before the opening backtick. The bare predicate is the reverse of
# the forward `is at`: "the function is at `factory/factory.sh:245`" states THIS tree's line. A bare
# "at" before a coordinate ("read at `manifest.py:507`") is deliberately NOT a marker -- measured on
# the corpus, twelve rows of it are revision-anchored readings ("`manifest.py:507` at `d7a5149`"),
# where the preposition attaches the observation to a named head and the row ages the moment the
# head is named. Only the verb with its subject ("is at") claims the gated tree.
NOW_REV = re.compile(r"(?:" + _NOW_AT + r"|" + _NOW_SAY + r"|" + _NOW_MV + r")[\s*.,;:]*\Z")
# the wrapped verb: H-0139's bullet puts the verb on the line AFTER the coordinate
# ("`AGENTS.md:38`\n now reads \"- Checkpoint...\""), so the forward read may cross ONE newline.
# Refusals name at most this many offenders; the count is in the headline either way.
MAX_LISTED = 12

# A diff tally: an `Effect:` that records a merge's read-back as `adds 429 (nine headings)`, the shape
# `H-0093` shipped and #389 found stale. Two rows carry it, both in that one entry, and both went false
# inside the merge they describe -- re-run from that merge's own parents on 19 Nov at `ab9ad8d` the pair
# answers 112 lines / 2 headings and 504 lines / 10 headings, one heading and one entry's prose apiece
# more than the 54 and 429 written. The figures are not repaired: `H-0101`'s reason still holds, that an
# edit inside the entry asserting "deletes 0 content lines" turns that 0 into a 1 and refutes the row it
# lands in. What #389 asks for instead is that a stale tally is NAMED by a later entry, so this file
# counts the claims and `--self-test` pins the comparison, and neither one refuses. Refusing is not a
# stricter option, it is a different and worse one: re-measuring a historical diff is a history read, and
# `gate_reads_no_refs` (`factory/selftest.sh`) denies this file any git verb but `cat-file` precisely
# because a verdict that moves between two identical checkouts gets re-run until it agrees (#253); and a
# gate that blocked on drift would make every merge that grows the log a blocked merge, which is #318's
# moved-pointer lesson read from the number side.
DIFF_TALLY = re.compile(r"\badds (\d+) \(([a-z0-9,]+) headings?\)")
# The heading count as these logs write it: `H-0093` spells both figures out, and a row of a decision
# log is prose. A word outside this table leaves the claim's second figure None, which `tally_verdict`
# counts and never compares -- guessing a number the prose did not write is the invention this whole
# file refuses.
_TALLY_NUM = {"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7,
              "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12}


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
        inherits the last file resolved in ITS OWN SENTENCE and never the last line read: a continuation
        is a property of its sentence, and inheriting across a finished sentence -- or across a whole
        wrapped paragraph that named other files -- silently re-aims a pointer at a different file.
        The sentence is delimited by ``SENTENCE_BREAK``, which #365 measured; before that the boundary
        was the physical line, and an entry in these logs is markdown wrapped at ~110 columns, so a
        line was often several sentences naming several files.
      * `` `gonee.md:3` `` -- a bare name WITH an extension is a path, and a file that is not in the tree
        is refused, not skipped: the anchor or the coordinate promised a reader something to open.
      * `` `pr:209` ``, `` `STATE_PROPS:1040` `` -- a triage `class:PR` pair and a symbol. Not
        citations. They fall out because their left half carries no extension, so it names no file in
        this tree and could not be made to without reading git for what used to be tracked.

    The shape that stays open, because the alternative was measured to be worse: a bare `:NNN` whose
    sentence names no file inherits nothing and is not refused -- 175 of the 312 bare rows are that
    shape at `e66dc85d`, 157 of 294 under this rule at `adddacc`, the sync-merge tree the boundary
    was measured on, and refusing them would refuse true sentences that merely wrap. The cost of the
    boundary, counted in occurrences at `adddacc`: 10 rows the line rule aimed -- only by riding an
    earlier sentence's citation, the mis-aim this rule exists to stop -- fell to the open shape,
    among them the four the issue names. No row's aim moved to a different file; the dropped
    sentences name the file bare with no coordinate or name no file at all (see
    ``SENTENCE_BREAK``). What the boundary bought in the same census: 52 wrapped continuations the
    line rule left invisible became aimed and counted -- 137 aimed in all at `adddacc`, and 137 again
    at `e66dc85d`, where the 18 bare rows this rule's own log prose added there landed in the open
    shape themselves.
    """
    found = []
    lines = text.splitlines()
    ends = []
    off = 0
    for line in lines:
        off += len(line) + 1
        ends.append(off)
    breaks = [m.end() for m in SENTENCE_BREAK.finditer(text)]
    store = {}
    for m in CITATION.finditer(text):
        if "\n" in m.group(0):
            # A range whose dash swallowed a line break is not one coordinate. The per-line scan this
            # function replaced never saw such a span, and neither does a reader; the `\s*` in the
            # range would otherwise let the whole-text scan invent it.
            continue
        n = bisect.bisect_right(ends, m.start()) + 1
        line_start = 0 if n == 1 else ends[n - 2]
        rest = lines[n - 1][m.end() - line_start:]
        written, span = m.group(1), m.group(2)
        sid = bisect.bisect_left(breaks, m.start())
        last = store.get(sid)
        continued = False
        if not written and last:
            written, continued = last, True
        anchor = None
        near = None
        wrapped = None
        am = ANCHOR_AFTER.match(rest)
        if am:
            anchor = am.group(1)
        else:
            # #369: the two newline shapes, flagged here and refused by name in `classify`. An
            # odd number of backticks in the tail means this coordinate sits inside an open code
            # span, and a quotation past it is wrapped SHELL (the `H-0142` row's `gh issue edit`
            # command), not a cut anchor -- that row keeps the debt exit it was measured with.
            if rest.count("`") % 2 == 0:
                nx = ANCHOR_AFTER.match(lines[n]) if n < len(lines) else None
                if nx:
                    wrapped = ("next-line", len(nx.group(1)))
                elif ANCHOR_DANGLING.match(rest):
                    wrapped = ("dangling-open", 0)
            nm = NEAR_QUOTE.match(rest)
            if nm:
                # (the gap itself, length of the quote) -- what the refusal has to name when it
                # declines this one for having no anchor at all. The gap is kept as text, not as a
                # width, because `` `x.md:9`'s "…" `` is three characters wide and the third of them
                # is what the rule declines; a width alone cannot tell the two refusals apart.
                near = (nm.group(1), len(nm.group(2)))
        bounds = [int(x) for x in span.replace(" ", "").split("-")]
        low, high = bounds[0], bounds[-1]
        malformed = high < low or low < 1
        # #374, adjacency: the forward read starts at this citation's closing backtick and may
        # cross ONE newline (the logs wrap at ~110 columns and H-0139's bullet opens the next
        # line with "now reads"); the reverse read ends at its opening backtick. A verb anywhere
        # else on the physical line belongs to a different coordinate's sentence. The reads run
        # on the PHYSICAL line pair, not the sentence: the claim is about the line the reader
        # finds the coordinate on, and #365's sentence boundary governs which file a bare `:NNN`
        # inherits, not which verb counts as current-state.
        fwd = rest
        if n < len(lines):
            fwd += "\n" + lines[n]
        wm = NOW_FWD.match(fwd) or NOW_REV.search(lines[n - 1][:m.start() - line_start])
        if continued:
            # A continuation inherits the FILE, not the spelling. The store holds a RESOLVED path;
            # re-resolving it by NAME can answer two candidates, and the no-guess guard below then
            # turned the continuation into "no file at all" -- which is how `./DECISIONS.md:2706` and
            # `:2986` left the continuation checked by nothing, because `DECISIONS.md` is also
            # `factory/DECISIONS.md` (#367 review #2, found by measuring the reviewer's own repro:
            # 4 rows in the two logs, all of them a `./`-prefixed parent whose basename is ambiguous
            # with a nested copy).
            written, cand = last, [last]
        else:
            cand = resolve_citation(written, index)
        if cand:
            kind = "ambiguous" if len(cand) > 1 else ("bad-range" if malformed else "file")
            resolved = cand[0] if kind == "file" else None
            if kind == "file":
                store[sid] = resolved
        elif written and revision_qualified(written):
            # `origin/main:factory/selftest.sh:4120` and `refs/heads/sibling:11` are in these logs.
            # They name the revision they were read at, which is the other half of the same promise
            # an anchor makes, so they owe no anchor -- and they cannot be checked here, because
            # checking them means reading another revision and this rule reads no git at all. The
            # pinned spelling is a sha; a branch name is the loose one, and `--citations` counts
            # them so the loose half stays visible.
            kind, resolved, cand = "revision-qualified", None, []
            # (#440) The left half still NAMES a file, and a bare `:NNN` later in the same sentence
            # must inherit it. Before a pin the same coordinate supplied that inheritance as a
            # `file` row; orphaning it is measured, not theorized -- the row that pinned
            # `factory/selftest.sh:10466` turned the `:263` and `:265` continuations on the next
            # row into `not-a-path`, checked by nothing. Fill the store when the path half resolves
            # uniquely; never guess a continuation's file any more than this rule guesses anywhere
            # else, so an ambiguous or missing half leaves the store as it was.
            parts = written.split(":", 1)
            half = parts[1] if len(parts) > 1 else ""
            if "/" in half or FILENAME_SHAPE.match(half):
                rc_ = resolve_citation(half, index)
                if rc_ and len(rc_) == 1:
                    store[sid] = rc_[0]
        elif written and ("/" in written or FILENAME_SHAPE.match(written)):
            # A path with a directory in it is a path even when nothing answers to it: the file was
            # renamed, or the entry misremembered where it lived. So is a bare name that carries an
            # extension -- `gonee.md:3` promised a reader a line in a file, and before #367 that
            # promise was checked by nothing whenever the file was gone, because this arm could only
            # be reached by a written value containing `/`. What stays exempt is a left half with no
            # extension: `pr:209` (a triage `class:PR` pair, common in both logs), `STATE_PROPS:1040`
            # (a symbol), `D-0033:236` and `H-0153:45` (an entry id and a line of prose about one),
            # `verify:1` and `make:2`. The cost is honest and measured: an ILLUSTRATIVE coordinate in
            # prose -- `x.md:9` written to explain a shape -- is refused too, and has to be spelled
            # without the colon or aimed at a real file. That is one edit in this log
            # today. The read that would separate the two for free is `git ls-files`, which says
            # whether a name was ever tracked; this rule reads no git at all (`gate_reads_no_refs`),
            # deliberately.
            kind = "bad-range" if malformed else "file-missing"
            resolved, cand = written, [written]
        else:
            kind, resolved, cand = "not-a-path", None, []
        found.append({"written": written, "kind": kind, "path": resolved,
                      "cands": cand, "low": low, "high": high, "anchor": anchor,
                      "near": near, "wrapped": wrapped, "now": wm.group(0) if wm else None,
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
    """The buckets a citation falls into, plus the count that owes an anchor.

    `read(path)` returns the file's text or None. Split out of `citation_problems` so `--citations`
    reports exactly what the gate refuses on, rather than a second implementation that can drift from
    it -- the same reason `check-scripts.py` imports the encoding guard instead of copying it.

    `wrapped` is the #369 bucket: a citation whose quoted anchor is present but cut by the line break,
    so the rule could not read it and the writer was never told. It must be zero like `unresolvable`
    and `broken`, and it is deliberately NOT folded into `unanchored`: those rows carry no quotation at
    all and are grandfathered into the ratchet number, while these carry one and refuse. Left in
    `unanchored` the same bytes both inflate the debt and pass, which is the defect #369 was filed on.

    `nowstate` is the #374 bucket and its argument is the same, one level up. A row whose sentence says
    the lines ARE at the coordinate ("now reads", "is at", "sits at") is an assertion about the tree
    being gated, not about the tree that was measured; left in `unanchored` it joins a number that only
    forbids RISING, so it stays counted forever and goes false in silence -- 49 rows measured 18 Nov,
    `make verify` green over every one. It must be zero, it is NOT in the debt number either way (a
    row refused at write time never becomes grandfathered slack), and the repair is the anchor or a
    past tense with the date, never a re-date of the coordinate: `H-0139`'s rule is that the number
    the entry measured stays as measured, and "was at `:46` on 11 Nov" keeps that promise while
    "is at `:52`" silently rewrites the record every merge.

    `indeterminate` is the #440 bucket and it refuses the accept branch's blind spot. The accept
    test asks whether the fragment is at the cited row -- a true answer, and no longer enough: a
    fragment that also sits on other rows cannot distinguish the promised row from them, so the
    entry says "one row" while the anchor answers several, and a scripted re-aim has no guard left
    (#440 measured 284 coordinates re-aimed green with three landed in the wrong entry, two of
    them 8,763 rows off inside a different heading's `Effect:` row). It must be zero like the
    others and it is not folded into `unanchored` either way -- the row carries a quotation and a
    lengthening is the whole repair, so it must refuse, not ratchet. Two matches are excluded from
    the rival count: rows inside the cited range, because the promise covers the range, and the
    citing row when the citation points from a file into that same file, because a row quotes its
    own anchor by construction -- counting that match makes every same-file cross-row pointer
    indeterminate with no repair that removes it (measured: 155 rows naive, 127 after the two
    exclusions; the naive count would have refused the gate's own corpus with nothing to fix).

    `midtoken` is the #495 bucket and it refuses the other blind spot of the same accept test. "The
    fragment is at the cited row" is true of a fragment that stops in the middle of a word, and such
    a quote is equally satisfied by a line that no longer says what the entry measured. Measured on
    `431b24b`, which re-aimed six rows of this log at the escalation-label default while changing
    both that line's number and its value: one row carries the whole new line as its quotation, five
    were shortened to its first 53 characters and now quote `${FACTORY_ESCALATION_LABEL:-` -- a quote
    ending inside the word `question`, on `factory/common.sh:297`, the line this file itself names as
    the single place that default is written. The five read green the moment they were written and
    read green on the tree that ships this paragraph, because containment measures what is inside the
    quotation marks and nothing measured what sits outside them: rename that default to anything and
    the quoted fragment stays true of the row while the row stops saying it. That is #374's `cannot
    fail` at the edge the accept branch never tested -- the same edge #440 widened the accept branch
    for at the other end: what the fragment matches, and now how much of the row it covers. It must
    be zero like the others and it is not folded into `unanchored` either way: the row carries a
    quotation and lengthening it is the whole repair, so it refuses, not ratchets. The boundary is a
    token run, not an arbitrary text run; `ANCHOR_TOKEN_RUN`, kept beside `anchor_cut` at the foot of
    this file, carries the measured reasons for the marks inside it and for the two that are not.

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
    wrapped = []
    nowstate = []
    indeterminate = []
    midtoken = []
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
        if c.get("wrapped") and c["anchor"] is None:
            # #369: the anchor exists and the newline is exactly why the rule cannot read it, so the
            # refusal says the newline -- the silence this row used to leave was the ticket's whole
            # complaint ("nothing says the newline is why").
            how, qlen = c["wrapped"]
            if how == "next-line":
                wrapped.append("%s %s -> %s is counted as having no anchor, but the quotation is there: "
                               "it starts on the NEXT physical line, %d characters. The anchor rule reads "
                               "only the coordinate's own line, so the NEWLINE is what cut this anchor, "
                               "not a missing quotation -- and until #369 a row like this one moved the "
                               "debt number with no error at all. Put the whole fragment on the "
                               "coordinate's own line, flush within three spaces of the closing backtick; "
                               "if that quotation belongs to the sentence below rather than to this "
                               "coordinate, re-wrap so the next line does not open with one"
                               % (at, cite, c["path"], qlen))
            else:
                wrapped.append("%s %s -> %s is counted as having no anchor, but the quotation opens "
                               "right after the closing backtick and never closes on this line -- the "
                               "NEWLINE cut it, and the anchor rule reads one physical line. Put the "
                               "whole fragment on the coordinate's own line; if you meant no anchor "
                               "here, take the opening quotation mark off the coordinate's tail"
                               % (at, cite, c["path"]))
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
            if c.get("now"):
                eof = (" and line %d is past the end of a file that has %d lines" % (c["low"], len(body))) \
                    if c["low"] > len(body) else ""
                nowstate.append("%s %s -> %s is a CURRENT-STATE citation: the sentence says %r%s, so "
                                "the row asserts where the text is in the tree being gated, and the next "
                                "merge to %s makes it false with every gate green -- that silence is "
                                "exactly what #374 was filed on. Either put the measured sentence beside "
                                "the coordinate -- `` `%s:%s` \"...\" `` with at least %d quoted "
                                "characters on the SAME physical line -- or reword the sentence into the "
                                "past with the date it was read (\"was at `` `%s:%s` `` on 12 Nov\"), "
                                "which keeps the number as measured and joins the counted debt like any "
                                "other unanchored pointer"
                                % (at, cite, c["path"], c["now"], eof, c["path"],
                                   c["written"] or "file", c["low"], MIN_ANCHOR,
                                   c["written"] or "file", c["low"]))
            else:
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
        region = norm(" ".join(body[c["low"] - 1:c["high"]]))
        if want in region:
            # #495: the same accept test, at its other edge. Containment says the fragment is on the
            # cited text and says nothing about where the quotation stops, and a fragment that stops
            # inside a token stays true when the rest of that token is renamed -- `cannot fail` in the
            # shape this branch never measured. The occurrence set is read off the same joined region
            # the accept test just matched against, so the two rules cannot disagree about where the
            # fragment sits; `anchor_cut` is conservative exactly as #440 is, refusing only when every
            # match of the fragment is cut.
            cut = anchor_cut(want, region)
            if cut is not None:
                where, dropped = cut
                c["cut"] = cut
                midtoken.append("%s %s -> %s is anchored to %r and the fragment IS at the cited row, "
                                "but the quotation stops inside a token: %s, so the rest of the word "
                                "can be renamed and this pointer stays green over a line that no "
                                "longer says it -- #374's `cannot fail`, and not a coordinate (the class "
                                "#495 was filed on is five rows of this log quoting "
                                "`${FACTORY_ESCALATION_LABEL:-`, 53 characters ending inside the word "
                                "`question`, after `431b24b` shortened them). Write the quotation "
                                "through to the end of the token "
                                "and re-check the longer fragment: it must still match only the "
                                "promised row, which is #440's rule and the reason not to pad the "
                                "quote with the next line"
                                % (at, cite, c["path"], c["anchor"][:48],
                                   ("the cited text carries %r more of the same word immediately after "
                                    "the quote" % dropped) if where == "end" else
                                   ("the cited text carries %r more of the same word immediately "
                                    "before the quote" % dropped)))
                continue
            # #440: "the fragment is at the cited row" answers *is it there*, never *is it only
            # there*, and a fragment that also sits elsewhere cannot single out the row the entry
            # promised -- that is how a scripted re-aim aimed three pointers into another entry's
            # `Effect:` row over 8,000 rows away and the gate read green. The match set is already
            # computed above; the accept branch used to discard it. Two kinds of match are not
            # rivals: the cited range, whose promise covers every row inside it, and the citing
            # row of a same-file citation -- a log row quotes its own anchor by construction, so
            # counting that self-match would grade every cross-row same-file pointer in the logs
            # indeterminate with no repair available, one sentence or not (H-0361).
            rivals = [n for n in now
                      if not (c["low"] <= n <= c["high"])
                      and not (c.get("label") == c["path"] and n == c["line"])]
            if rivals:
                more = " and %d further rows" % (len(rivals) - 6) if len(rivals) > 6 else ""
                indeterminate.append(
                    "%s %s -> %s is anchored to %r and the fragment IS at the cited row, but it "
                    "also sits on %s%s -- this check cannot tell the row the entry promised from "
                    "those, so the pointer says one row while the anchor answers %d. Lengthen the "
                    "quotation to a fragment only the promised row carries: the fix is one "
                    "sentence, and an anchor that answers several rows is refused here at any "
                    "size, because a citation a mover cannot tell apart from its lookalikes is "
                    "the defect #440 was filed on"
                    % (at, cite, c["path"], c["anchor"][:48],
                       ", ".join(str(x) for x in rivals[:6]), more, len(rivals) + 1))
            continue
        where = ("it is at line %s now, not %d -- the lines above it moved, and a pointer that is "
                 "wrong by one line still lands on the wrong sentence"
                 % (", ".join(str(x) for x in now[:6]), c["low"])) if now else \
                ("it is nowhere in %s -- the sentence was deleted, or the entry never quoted it "
                 "faithfully; line %d reads: %s"
                 % (c["path"], c["low"], norm(body[c["low"] - 1])[:96]))
        broken.append("%s %s in %s is anchored to %r and %s"
                      % (at, cite, c["path"], c["anchor"][:48], where))
    return unanchored, unresolvable, broken, wrapped, nowstate, indeterminate, midtoken


CitationVerdict = collections.namedtuple(
    "CitationVerdict",
    # `judged` is the answer to "did the ratchet actually compare this number?", and `unjudged_because`
    # is the sentence that says why it did not. Both exist because the headline used to claim "the first
    # number is the recorded debt, which may only fall" on a run where the number had just been moved by
    # an untracked directory -- a true sentence about a false measurement. One classifier, one verdict,
    # so the headline cannot disagree with the refusal list above it. `wrapped` (#369) is on the verdict
    # for the same reason: the headline and `--citations` must read the same four buckets. `nowstate`
    # (#374) joins them for the same reason again: a fifth bucket that only the report knew about would
    # let the headline count as debt exactly the rows the report refuses. `indeterminate` (#440) joins
    # for the third time on the same argument: the refusal list, the `--citations` report and the
    # headline must be three reads of one verdict, not three chances to disagree. `midtoken` (#495)
    # joins for the fourth time on the same argument, and its absence would be worse than a drift in a
    # count: a cut quotation is the one refused shape that reads as a satisfied anchor, so a headline
    # that did not carry it would call the run clean while the report listed pointers that stay green
    # over lines that no longer say them.
    "problems measured unanchored unresolvable broken wrapped nowstate indeterminate midtoken judged "
    "unjudged_because")


def citation_problems(found, read, debt=POINTER_DEBT, judge_debt=True):
    """Problems, and a `CitationVerdict` carrying the measured count of citations that still owe an anchor.

    `read(path)` is injected so `--self-test` can assert every arm without a filesystem: the moved
    anchor, the deleted one, the path that is two files, the range written backwards, a missing file
    anchored and unanchored, a citation that names its own revision, the ratchet in both directions, and
    the ratchet NOT running when a pointer cannot be opened, both newline-cut anchors (#369: the
    fragment on the next line, the quotation unclosed on this one) refused rather than counted, and the
    #440 anchor that answers more rows than the citation promised (both polarities, the same-file
    self-match exclusion, the cited range, and the ratchet held while one exists), and the #495 anchor
    that stops inside a token of the row it quotes (the cut end, the cut beginning, a cut inside a
    dotted name or a path staying outside the class, a range edge, the conservative many-match rule,
    and the ratchet held while one exists).
    `judge_debt=False` is the `--log` case, where the corpus the number describes is not the corpus
    being read.
    """
    problems = []
    for c in found:
        if c["kind"] == "bad-range":
            problems.append("%s:%d (%s) `%s:%d-%d` cites no lines: the range runs backwards or starts "
                            "before the first line of the file, so there is nothing for a reader to "
                            "open" % (c.get("label", "?"), c["line"], c["entry"], c["written"],
                                      c["low"], c["high"]))
    unanchored, unresolvable, broken, wrapped, nowstate, indeterminate, midtoken = \
        classify(found, read)
    for group, head in ((broken, "anchored citation(s) in the decision logs point at text that is not "
                                 "there"),
                        (unresolvable, "citation(s) in the decision logs a reader cannot open at all"),
                        (wrapped, "citation(s) in the decision logs whose quoted anchor sits on the far "
                                  "side of a newline -- the anchor is there, and the rule could not read "
                                  "it, which #369 refuses instead of counting as debt"),
                        (nowstate, "citation(s) in the decision logs whose sentence says the text IS at "
                                   "the coordinate in the tree being gated -- a claim the next merge "
                                   "silently falsifies, which #374 refuses instead of counting as debt"),
                        (indeterminate, "citation(s) in the decision logs whose anchor is satisfied by "
                                        "more rows than the citation promised -- the text is at the "
                                        "row, and elsewhere too, which #440 refuses instead of "
                                        "accepting as checked"),
                        (midtoken, "citation(s) in the decision logs whose quotation stops inside a "
                                   "token of the cited row -- the text is at the row and the quote "
                                   "carries part of one of its words, which #495 refuses instead of "
                                   "accepting as checked")):
        if not group:
            continue
        problems.append("%d %s:" % (len(group), head))
        problems.extend("    " + g for g in group[:MAX_LISTED])
        if len(group) > MAX_LISTED:
            problems.append("    ... and %d more; `python3 scripts/check-decisions.py --citations` "
                            "lists all of them" % (len(group) - MAX_LISTED))
    measured = len(unanchored)
    unopenable = (len(unresolvable) + len(broken) + len(wrapped) + len(nowstate)
                  + len(indeterminate) + len(midtoken))
    if not judge_debt:
        return CitationVerdict(problems, measured, unanchored, unresolvable, broken, wrapped,
                               nowstate, indeterminate, midtoken, False,
                               "--log narrows the corpus the number is a count of")
    if debt is None:
        problems.append("the citation-debt number could not be read, so the rule did not run")
        return CitationVerdict(problems, measured, unanchored, unresolvable, broken, wrapped,
                               nowstate, indeterminate, midtoken, False, "the recorded debt could not be read")
    if unopenable:
        # The ratchet is a statement about the logs, and this number is not one: every pointer in the two
        # lists above left `unanchored` without the logs changing, and a stray directory can do it to a
        # dozen at a time. Measured at `bc1c08c` with one untracked `vendor/AGENTS.md`: 12 pointers moved
        # and the gate ordered "lower POINTER_DEBT to 505" -- the one repair that permanently disarms it,
        # issued to a reader under a green-gate deadline (#367 review #1). The must-be-zero refusals above
        # still stand on their own; only the comparison with the constant waits.
        problems.append("%d citation(s) above are not in the %d the ratchet guards: a pointer a reader "
                        "cannot open, one #374 refuses as a current-state claim, one #440 refuses "
                        "because its anchor answers more rows than the citation promised, or one #495 "
                        "refuses because the quotation stops inside a token of the row it quotes, is a "
                        "defect, "
                        "not a "
                        "grandfathered promise -- and while rows sit in those lists this number is not "
                        "the number the constant is a statement about. The number is therefore "
                        "NOT judged this run, and that is not an invitation to edit POINTER_DEBT -- an "
                        "untracked or ignored directory moves pointers out of the debt and into that list, "
                        "so a number measured against a tree the repository does not have says nothing "
                        "about the logs. Fix the list, make `git status --short --ignored` empty of "
                        "anything the logs did not write, and the comparison comes back on the next run."
                        % (unopenable, measured))
        return CitationVerdict(problems, measured, unanchored, unresolvable, broken, wrapped,
                               nowstate, indeterminate, midtoken, False,
                               "%d citation(s) in a must-be-zero bucket" % unopenable)
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
    return CitationVerdict(problems, measured, unanchored, unresolvable, broken, wrapped,
                           nowstate, indeterminate, midtoken, True, None)


TallyVerdict = collections.namedtuple(
    "TallyVerdict", "reports mismatched re_measured unverifiable")


def tally_line(count):
    """The clean-run sentence about diff tallies, or None when the logs hold none.

    A function so `--self-test` can assert what a reader of a green run actually reads. Three things in
    it are load-bearing and one mutant each: the count, the words `none re-measured`, and the two issue
    numbers -- #253 for why the gate cannot know, #389 for the convention that stands in place of
    knowing. A later edit that dropped `none re-measured` would leave a sentence that reads like a
    verdict about diffs this file has never looked at, which is the invention `H-0237` and `#1180` exist
    to refuse.
    """
    if not count:
        return None
    return ("check-decisions: %d `Effect:` diff tallies counted in the two logs, none re-measured -- "
            "this gate reads no history (#253), so a tally a later merge outgrew is named by that "
            "entry and never edited (#389)" % count)


def tally_claims(text, entries):
    """Every `Effect:` diff tally in one log, as (line, entry, lines added, headings, as written).

    Scanned over the whole text rather than the `Effect:` field alone: the claim a reader follows is the
    figure and its paren, and the dated annotation `H-0096` and `H-0101` set quotes that same figure --
    an annotation quoting `adds 429 (nine headings)` is a second claim on the same diff and belongs in
    the same count, or the count of receipts silently falls every time one is named.
    """
    claims = []
    for m in DIFF_TALLY.finditer(text):
        word = m.group(2)
        line = text[:m.start()].count("\n") + 1
        claims.append({"line": line,
                       "entry": entry_of(entries, line),
                       "adds": int(m.group(1)),
                       "headings": int(word) if word.isdigit() else _TALLY_NUM.get(word),
                       "written": m.group(0)})
    return claims


def measure_no_history(claim):
    """What the shipped gate can answer about any historical diff: nothing, and say so.

    `measure` is injected for the same reason `citation_problems` injects `read(path)` and
    `checkpoint_problems` injects `resolve` -- the arm that decides "stale" is asserted against a
    fixture diff in `--self-test`, while the shipped call hands back None for every claim. Re-running
    `git diff <parent> <merge>` is a refs-and-history read that #253's `gate_reads_no_refs` leg denies
    this file, and a count is the honest shipped verdict: a number that changed between two identical
    checkouts with no commit between them is the flakiness that leg was written to refuse.
    """
    return None


def tally_verdict(claims, measure):
    """Report -- never refuse -- each tally whose diff answers a different figure today.

    `measure(claim)` answers `(lines_added, headings)` for the diff the claim quotes, or None when it
    cannot be re-measured. `--self-test` injects a fixture whose diff answers 112 lines against a claim
    of `adds 54 (one heading)` -- #389's own pair, measured at `ab9ad8d` -- and the shipped gate passes
    `measure_no_history`, so its `reports` is empty by construction and only the count reaches the
    headline. Nothing here returns problems, and that is the rule, not an omission: a tally a later
    merge outgrew is annotated by that later entry (#1180's dated correction beside the record, which is
    what `H-0096` and `H-0101` did to their own stale tallies) and blocking a merge over it is the
    #318 failure re-imported as a number.
    """
    reports, re_measured, mismatched, unverifiable = [], 0, 0, 0
    for c in claims:
        if c["headings"] is None:
            unverifiable += 1
            continue
        got = measure(c)
        if got is None:
            unverifiable += 1
            continue
        re_measured += 1
        if got[0] != c["adds"] or got[1] != c["headings"]:
            mismatched += 1
            reports.append("%s:%d (%s) quotes `%s` and that diff answers %d lines and %d headings -- "
                           "the merge it describes grew the log past its own receipt. The written "
                           "figures stay as measured, because editing them is the deletion that refutes "
                           "the row; #389's convention is that a later entry names the drift."
                           % (c.get("label", "?"), c["line"], c["entry"], c["written"], got[0],
                              got[1]))
    return TallyVerdict(reports, mismatched, re_measured, unverifiable)


# The files that TEACH this rule are the one place a false pointer is invisible by construction: the rule
# reads the two decision logs and nothing else, so a worked example six lines off is copied by every unit
# that opens it. Measured at `4fcf9a4` -- the checkpoint before this code -- `.opencode/skills/factory/SKILL.md`
# taught `AGENTS.md` at line 46 for a sentence that sits at line 52, and wrote a bare `DECISIONS.md:2221-2231`
# that this gate refuses outright inside a log because two files answer to that name.
TEACHES_RULE = (".opencode/skills/factory/SKILL.md", "AGENTS.md")


def teaches_problems(read, index, paths=TEACHES_RULE):
    """Refuse a pointer in the instruction that teaches the rule, and nothing else about it.

    Same classifier as the logs, deliberately -- a second copy of the anchor logic is the bug H-0170
    removed from the headline. What differs is what is owed: an entry in a decision log makes a promise
    forever, so an unanchored one is counted against the ratchet; a guide is edited as a unit and its
    pointers are re-read by this command, so what it owes is that every coordinate it tells a unit to
    copy resolves today, and an unanchored one is not a defect. That is why `unanchored` is dropped here
    and the two must-be-zero lists are not -- plus, since #369, `wrapped`: a guide that demonstrates the
    anchor rule with a quotation the newline cut in half demonstrates the very exit the rule now refuses,
    and every unit that copies it writes debt it was never told about. And since #440, `indeterminate`
    is refused here too, deliberately inherited rather than filtered: the guide is where an ambiguous
    anchor does the most damage, because it is the shape every unit copies -- #440 itself traced the
    boilerplate `Effect:` anchors that matched dozens of rows to pointers re-aimed by script under a
    gate that graded them as good, and the sharpest specimen was a guide line quoting the very row the
    rule uses as its example. And since #495, `midtoken`, for the same reason in its opposite shape: a
    guide that demonstrates the anchor with a quotation stopped inside a word teaches the cut, and a
    unit copying it writes a pointer that cannot fail -- #495's five rows are what a corpus looks like
    when nobody grades that. `docs/factory.md` is NOT in
    `TEACHES_RULE` and that is a measured choice, not an oversight: it is the reference, and it carries
    the shapes the rule refuses on purpose (`gonee.md:3`, `pr:209`, a `README.md:588` past a 349-line
    EOF) inside the table that explains them, so pointing this pass at it refuses the guide's own
    examples.
    """
    found = []
    for path in paths:
        text = read(path)
        if text is None:
            continue          # a fixture tree, a fork without the skill: absent is not false
        for c in scan_citations(text, [], index):
            c["label"], c["entry"] = path, "the guide"
            found.append(c)
    _, unresolvable, broken, wrapped, _nowstate, indeterminate, midtoken = classify(found, read)
    return ["the file that teaches this rule has a pointer a reader cannot follow: " + r
            for r in unresolvable + broken + wrapped + indeterminate + midtoken]


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
                               "anchored, QUOTE STOPS INSIDE A TOKEN (#495)" if c.get("cut") else "anchored"
                               if c["anchor"] else
                               ("ANCHOR CUT BY NEWLINE" if c.get("wrapped") else
                                 ("CURRENT-STATE (#374)" if c.get("now") else "UNANCHORED"))))
        # Returned, not printed: the census branch below returns its lines and `main` prints both, and a
        # function with two output channels is one the self-test cannot assert on (H-0171 found this the
        # hard way -- the leg that proves `--citations gonee.md` still finds a pointer whose file is
        # already gone was asserting on `[]`).
        out = ["citations pointing at %s (%d):" % (want, len(rows))]
        for r in sorted(rows, key=lambda s: s.split("`")[0]):
            out.append("  " + r)
        if rows:
            out.append("  -- editing that file moves these. Fix them in the same commit, or the log "
                       "keeps pointing at whatever line arrived where the number used to be.")
        return out

    if verdict is None:
        # A report asked for on its own has no verdict to borrow; classifying here is the one copy of the
        # rule, not a second implementation of it.
        unanchored, unresolvable, broken, wrapped, nowstate, indeterminate, midtoken = \
            classify(found, read)
        note = ""
    else:
        unanchored, unresolvable, broken = verdict.unanchored, verdict.unresolvable, verdict.broken
        wrapped = verdict.wrapped
        nowstate = verdict.nowstate
        indeterminate = verdict.indeterminate
        midtoken = verdict.midtoken
        note = "" if verdict.judged else " (NOT judged this run: %s)" % verdict.unjudged_because
    counts = {}
    for c in found:
        if (c["kind"] in ("file", "file-missing") and c["anchor"] is None and not c.get("wrapped")
                and not c.get("now")):
            counts[c["path"]] = counts.get(c["path"], 0) + 1
    out = ["citations that owe an anchor, by the file they point at (%d):" % sum(counts.values())]
    for path in sorted(counts, key=lambda p: (-counts[p], p)):
        out.append("  %3d  %s" % (counts[path], path))
    out.append("citations a reader cannot open (%d):" % len(unresolvable))
    out.extend("  " + u for u in unresolvable)
    out.append("anchored citations whose text has moved or gone (%d):" % len(broken))
    out.extend("  " + b for b in broken)
    out.append("citations whose anchor the newline cut (#369) (%d):" % len(wrapped))
    out.extend("  " + w for w in wrapped)
    out.append("current-state citations (#374): sentence says the text IS there today (%d):"
               % len(nowstate))
    out.extend("  " + s for s in nowstate)
    out.append("citations whose anchor answers more rows than the citation promised (#440) (%d):"
               % len(indeterminate))
    out.extend("  " + i for i in indeterminate)
    out.append("citations whose quotation stops inside a token of the cited row (#495) (%d):"
               % len(midtoken))
    out.extend("  " + m for m in midtoken)
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
            where = sorted(where_seen[ident])
            # Which verb applies depends on the layout the second copy arrived in. Inside a monolith the
            # fix is to renumber and re-sort; a filed entry has no neighbours to sort past -- it has to
            # take an id nothing has allocated, which is the allocator's job and not an edit here.
            filed = [w for w in where if os.path.dirname(w.replace(os.sep, "/")) in ENTRY_DIRS]
            advice = ("Renumber the later entry in its own log." if not filed else
                      "One of them keeps the id; the other takes a fresh one from "
                      "`factory/next-decision-id.sh` and, if it stays filed, is renamed to match.")
            problems.append(
                "%s is used in %s -- one id, %d decisions; a citation to it does not resolve. %s" %
                (ident, " and ".join(where), len(where), advice))
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
    if any(GATE_SENTENCE in p for p in invocation):
        fails.append("verify-invocation: `make verify` no longer calls check-decisions.py for its "
                     "decision gate -- %s" % invocation)
    if any(SELFTEST_SENTENCE in p for p in invocation):
        fails.append("verify-invocation: nothing runs `check-decisions.py --self-test` -- %s"
                     % invocation)

    # And the guard on the guard. Asserting only against the live Makefile proves the Makefile is
    # fine today; it says nothing about whether this function would notice it stop being fine, which
    # is the whole job. The first four cases below are Makefiles in which `make verify` runs no
    # decision gate at all while still containing the words "check-decisions.py", so a match weakened
    # back to a substring turns one of these into a pass and this run refuses. The last two are
    # #391's pair, and they are what naming the sentences is for: each deletes ONE clause and leaves
    # the other running, so a guard that cannot tell the two assertions apart passes both of them --
    # which is exactly how the bare gate step stayed deletable after the self-test leg went in.
    counts[1] += 1
    if invocation_problems(RECIPE_GOOD):
        fails.append("recipe-good: the Makefile's own shape is refused by the guard -- %s"
                     % invocation_problems(RECIPE_GOOD))
    for name, (recipe, expect) in sorted(RECIPE_BAD.items()):
        counts[0] += 1
        if recipe == RECIPE_GOOD:
            # A mutant whose edit found nothing to mutate asserts nothing while printing a green row,
            # the failure `scripts/mutate.py` refuses outright for the same reason. `gate-deleted` and
            # `self-test-deleted` are `str.replace` over a constant, so the day `verify`'s recipe
            # changes shape they silently become RECIPE_GOOD again and every one of them passes.
            fails.append("recipe-%s: the case's own edit changed nothing, so it asserts nothing -- the "
                         "Makefile no longer holds the invocation shape this case documents" % name)
            continue
        says = invocation_problems(recipe)
        # `says`, not `found`: the dict `invocation_problems` builds under that name answers
        # "did a step run", and a list of refusations in the same reader's view under the same
        # name is how a later edit reads the wrong one. `status` is named by fragment, not by the
        # constant, because STATUS_SENTENCE is a `%s` template by then.
        earned = set(n for n, s in (("gate", GATE_SENTENCE), ("self-test", SELFTEST_SENTENCE),
                                    ("status", "runs where its exit status cannot reach the shell"))
                     if any(s in p for p in says))
        if not says:
            fails.append("recipe-%s: a Makefile that runs no decision gate passed the guard; a "
                         "substring is not an invocation" % name)
        elif earned != set(expect):
            # The reason is asserted, not just the refusal. Before #391 the four cases above were the
            # whole table and all four asked for the gate sentence, because all four lack the command
            # outright; nothing asked what a file that keeps the self-test and loses the gate owes. A
            # guard that answers every question with one sentence, or answers the gate question from
            # the self-test line, passes that table and leaves the gate step deletable. The seventh
            # case owes `status` as well as `gate`, which is what pins #472's arm: a guard that
            # counted the piped chunk as the step says nothing, and one that only missed the step
            # says `gate` alone.
            fails.append("recipe-%s: owes %s, says %s -- %s"
                         % (name, " and ".join(sorted(expect)) or "nothing",
                            " and ".join(sorted(earned)) or "nothing", says))

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

    # The `sha` (PR #N) provenance pairs (#433): the git token the citation rules never opened. The
    # message table is a fixture, not git -- the rule has to be provable both ways without a history,
    # the way `checkpoint_problems` is, and #433's shipped pair `cf00b44` (PR #427, `H-0235`) is
    # recreated here rather than trusted to a probe: the drift case below IS that row.
    PROV_FIXTURE = ("## H-0100 · Pair\n\n- Question: q?\n- Options: a | b\n- Chose: a\n"
                    "- Why: the merge is `origin/main` `1111111` (PR #410, `H-0105`) into the branch.\n"
                    "- Checkpoint: abc1234\n")
    PROV_MSGS = {
        "1111111": "Merge pull request #410 from vicowa-botworks/issue-348\n\nfeat: body text\n",
        "2222222": "Merge pull request #432 from vicowa-botworks/issue-341\n\nfeat: x\n",
        "3333333": "fix: a landing that was squashed, not merged\n\nmentions #499 in passing\n",
    }

    def pairs(text, resolve=lambda s: True, messages=PROV_MSGS):
        return provenance_pair_problems(text, parse(text, "log"), resolve, messages)

    counts[1] += 1
    if pairs(PROV_FIXTURE):
        fails.append("pair-agrees: a `sha` (PR #N) pair whose commit subject names PR #N was refused "
                     "-- %s" % pairs(PROV_FIXTURE))
    counts[0] += 1
    drift = pairs(PROV_FIXTURE.replace("(PR #410,", "(PR #427,"))
    if not any("PR #427" in p and "Merge pull request #410" in p for p in drift):
        fails.append("pair-drift: #433's shipped shape -- PR #427 asserted beside the commit whose "
                     "subject names #410 -- passed the provenance rule; the refusal must name both "
                     "numbers -- %s" % drift)
    counts[0] += 1
    dead = pairs(PROV_FIXTURE, resolve=lambda s: False)
    if not any("not a commit in this repository" in p for p in dead):
        fails.append("pair-dead-object: an asserted commit that resolves to nothing passed the "
                     "provenance rule -- %s" % dead)
    counts[1] += 1
    if pairs(PROV_FIXTURE.replace("`1111111` (PR #410", "`2222222` (PR #341/#432")):
        fails.append("pair-multi-number: `sha` (PR #341/#432) beside the commit that is #432's "
                     "landing was refused -- any one asserted number agreeing is the multi-number "
                     "spelling's meaning")
    counts[1] += 1
    if pairs(PROV_FIXTURE.replace("`1111111` (PR #410", "`3333333` (PR #499")):
        fails.append("pair-nonmerge-in-message: a non-merge commit whose message carries the "
                     "asserted number was refused -- that is the rule's own second clause")
    counts[0] += 1
    noNum = pairs(PROV_FIXTURE.replace("`1111111` (PR #410", "`3333333` (PR #500"))
    if not any("PR #500" in p for p in noNum):
        fails.append("pair-nonmerge-absent: a non-merge commit whose message carries none of the "
                     "asserted numbers passed -- %s" % noNum)
    counts[1] += 1
    # The correction shape H-0237's bullet actually ships: a quoted `(PR #427, `H-0235`)` with the
    # sha NOT glued to it. A gate that refused a correction would be refused back, and the row
    # would be edited into a lie to make the gate go quiet.
    quoted = ("## H-0237 · Correction\n\n- Why: it read `(PR #427, `H-0235`)` beside `origin/main` "
              "`cf00b44`, and both tokens were recalled rather than read.\n")
    if pairs(quoted):
        fails.append("pair-quote-unbound: a correction quoting a false pair, with the sha not glued "
                     "to the reference, was refused -- the binding is the claim, and a quotation "
                     "makes none -- %s" % pairs(quoted))
    counts[0] += 1
    wrapped = pairs(PROV_FIXTURE.replace("`1111111` (PR #410", "`1111111`\n  (PR #427"))
    if not any("PR #427" in p for p in wrapped):
        fails.append("pair-line-wrap: an entry wrapped at ~110 columns is how these logs ship, and "
                     "a pair split across the wrap escaped the rule -- %s" % wrapped)
    counts[0] += 1
    blind = pairs(PROV_FIXTURE, resolve=None)
    if not any("did not run" in p for p in blind):
        fails.append("pair-git-blind-existence: no resolver answer produced no refusal -- %s" % blind)
    counts[0] += 1
    blind2 = pairs(PROV_FIXTURE, messages=None)
    if not any("did not run" in p for p in blind2):
        fails.append("pair-git-blind-message: an unreadable message batch produced no refusal -- %s"
                     % blind2)
    counts[0] += 1
    halfblind = pairs(PROV_FIXTURE, messages={})
    if not any("could not be read" in p for p in halfblind):
        fails.append("pair-message-missing: a mapping that omits an existing commit answered the "
                     "message rule with a pass -- the fail-open default one level down -- %s"
                     % halfblind)
    counts[1] += 1
    # The reader's own bytes (H-0355, the review of #498). The rows above inject a message TABLE, so
    # none of them ever reaches the framer; the framer is where the shipped defect lived -- it framed
    # a char-decoded stream by the byte counts a `cat-file --batch` header announces, and so answered
    # `None` for a batch git had answered perfectly, on any message that is not ASCII. Pinned here on
    # synthetic records: toolchain-free, git-free, the subprocess leg the seam leaves behind is the
    # only line of the reader `--self-test` cannot reach.
    def batch(sha, message, announce=None, name=None):
        body = message.encode("utf-8")
        head = "%s commit %d\n" % (name or sha, len(body) if announce is None else announce)
        return head.encode("ascii") + body + b"\n"

    EM = "Merge pull request #411 from x\n\na merge body carrying an em dash — inside it\n"
    ASCII_MSG = "Merge pull request #410 from x\n\nplain ascii body\n"
    # The control the fix is owed before anything is believed about it: a fixture that is ASCII in
    # both units pins nothing about byte framing, so assert the differential the defect needs.
    if len(EM) == len(EM.encode("utf-8")):
        fails.append("pair-batch-control: the em-dash fixture is ASCII, so it cannot distinguish a "
                     "byte-framed parser from a char-framed one -- the plant has no effect and the "
                     "rows beside it prove nothing")
    counts[0] += 1
    # …and the differential aimed the other way: the char-framed shape has to be REFUTED on these
    # bytes, or the row below passes for a reason that has nothing to do with the fix.
    char_body = batch("1111111", EM).decode("utf-8").split("\n", 1)[1]
    if len(char_body) >= int(batch("1111111", EM).split(b"\n", 1)[0].split(b" ")[2]):
        fails.append("pair-batch-control-char: the fixture's char-decoded body is not shorter than "
                     "its announced byte size, so the char-framed reader would have parsed it and "
                     "the multibyte row below is not a regression test -- %d chars" % len(char_body))
    counts[1] += 1
    got = parse_pair_batch(["1111111"], batch("1111111", EM))
    if not isinstance(got, dict) or got.get("1111111") != EM.split("\n\n", 1)[1]:
        fails.append("pair-batch-multibyte: a commit message whose bytes outnumber its characters was "
                     "not decoded -- that is the shape `1c385b7` measures 1364 bytes against 1361 "
                     "characters, which the shipped char-framed reader answered None for -- %r" % (got,))
    counts[1] += 1
    # Position independence: the shipped reader only reddened when the multibyte record sorted LAST.
    two = batch("1111111", ASCII_MSG) + batch("2222222", EM)
    both = parse_pair_batch(["1111111", "2222222"], two)
    if not isinstance(both, dict) or both.get("2222222") != EM.split("\n\n", 1)[1]:
        fails.append("pair-batch-order: a multibyte record last in the batch desynced the framer -- "
                     "the position arithmetic that made `make verify` red on one corpus order and "
                     "green on the reverse -- %r" % (both,))
    counts[0] += 1
    if parse_pair_batch(["1111111"], batch("1111111", EM, announce=len(EM.encode()) - 2)) is not None:
        fails.append("pair-batch-short-record: a record whose bytes do not end where its own header "
                     "said they would was answered with a message instead of a refusal")
    counts[0] += 1
    if parse_pair_batch(["1111111"], batch("1111111", EM, name="9999999f")) is not None:
        fails.append("pair-batch-desync-name: a header naming an object other than the one asked for "
                     "was answered with its message -- a desynced stream re-aligning three bytes into "
                     "the next header and grading a row on its neighbour's bytes, silently")
    counts[1] += 1
    gone = parse_pair_batch(["1111111"], b"1111111 missing\n")
    # Keyed AND `None`, not merely absent: a framer that skips the record hands the row map a missing
    # key, which the rule below the reader also refuses -- but then this row would pass on the skip,
    # and the shape it names (the object is unreadable, and the reader says so under its own key)
    # would be unpinned. Measured: the skip is invisible to `.get(k) is not None`.
    if not isinstance(gone, dict) or "1111111" not in gone or gone["1111111"] is not None:
        fails.append("pair-batch-missing-record: `<sha> missing` was not read as that commit being "
                     "unreadable under its own key, which is how a vanished object stops being a "
                     "refusal -- %r" % (gone,))
    counts[1] += 1
    if pairs("## H-0100 · No pair\n\n- Why: nothing git here.\n", resolve=None, messages=None):
        fails.append("pair-nothing-to-ask: a log with no pairs was refused for a git problem it "
                     "does not have -- the same boundary `checkpoint_problems` draws")

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
        # #440: one fragment on two rows; the same shape pointed at from the target file's OWN row
        # (the self-match a log citation carries by construction), with and without a third
        # lookalike -- the exclusion has a control leg, and the control is the shape #440 died on.
        "dupes.md": "line one\n"
                    "the anchor appears on two rows of this file\n"
                    "the anchor appears on two rows of this file, once here and again below\n"
                    "last line\n",
        "selfcite.md": "lockout is re-asserted on every open window of the tick\n"
                       'see `selfcite.md:1` "lockout is re-asserted on every open window" quotes '
                       "its own anchor\n"
                       "nothing here repeats that fragment\n",
        "selfcite-dup.md": "lockout is re-asserted on every open window of the tick\n"
                           'see `selfcite-dup.md:1` "lockout is re-asserted on every open window" '
                           "but a third row repeats it\n"
                           "lockout is re-asserted on every open window shows up here too\n",
        # #495: the boundary, in both directions. Rows 1 and 2 are one quote's two fates -- the
        # fragment is a whole word on the first and the head of a longer word on the second, so the
        # leg pair proves the fixture has both preconditions and not only the one a refusal needs.
        # Row 3 is the class #495 was filed on, verbatim. Rows 4 and 6 are the two marks the token
        # alphabet leaves out on purpose, and their legs are what the cost of putting them back is
        # measured against. Row 5 is the cut at the quotation's beginning. The two files after this
        # one are the conservative arm and its control: a fragment that matches a row twice, once
        # whole and once cut, against the same fragment matching nothing but cuts.
        "tokens.md": "read the escalation label from common.sh and print it\n"
                     "read the escalation labels from common.sh and print them\n"
                     "FACTORY_ESCALATION_LABEL=${FACTORY_ESCALATION_LABEL:-question}\n"
                     "the boundary of the add/delete rename is where a move is not a copy\n"
                     "cleanup runs find . -name 'triage-stderr.*' -delete at the tick's foot\n"
                     "the gate script is check-decisions.py at the row's end\n",
        # The conservative arm needs a fragment that matches twice WITHOUT the other row of the same
        # file carrying it, or #440 answers first and the arm is never reached -- so each arm gets its
        # own file, and the pair is the arm and its control.
        "tokens-mixed.md": "count the word tokens here, then count the word token there\n",
        "tokens-cut-twice.md": "count the word tokens here, and count the word tokens there too\n",
        # #495's own corpus leg: the log cites the one line that writes a default, and quotes the head
        # of the substitution on it. The file is named as the logs name it, so the leg scans the same
        # shape the five rows are.
        "factory/common.sh": "# the shipped default, and the one line that writes it\n"
                             "FACTORY_ESCALATION_LABEL=${FACTORY_ESCALATION_LABEL:-question}\n",
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
    # #440: the anchor must SINGLE OUT the cited row, and it must be told apart from the matches
    # that are not rivals -- the cited range (the promise covers every row in it) and the
    # citing row of a same-file citation (a row quotes its own anchor by construction; counting it
    # makes every cross-row pointer in the logs indeterminate with no repair that removes it).
    cite_case('`dupes.md:2` "the anchor appears on two rows of this file".\n',
              "anchor-duplicated-refused", "also sits on 3")
    cite_ok('`dupes.md:3` "anchor appears on two rows of this file, once here and again below".\n',
            "anchor-unique-ok")
    cite_ok('Lines `dupes.md:2-3` "the anchor appears on two rows of this file".\n',
            "anchor-dupes-inside-range")
    # the self-match exclusion, constructed rather than body-wrapped so the citation's label IS
    # the target file -- every other leg cites across files, where the exclusion must NOT apply
    # (the `anchor-duplicated-refused` leg above is that control).
    counts[1] += 1
    self_f = scan_citations(read_fixture("selfcite.md"), [], CITES_INDEX)
    for c in self_f:
        c["label"] = "selfcite.md"
    self_v = citation_problems(self_f, read_fixture, debt=0)
    if len(self_f) != 1 or self_v.problems:
        fails.append("self-row-is-not-a-rival: a same-file citation quoting its own anchor from "
                     "inside the target %s -- the exclusion or the scan broke"
                     % ("was refused" if self_v.problems else "was not scanned"))
    counts[0] += 1
    selfd_f = scan_citations(read_fixture("selfcite-dup.md"), [], CITES_INDEX)
    for c in selfd_f:
        c["label"] = "selfcite-dup.md"
    selfd_v = citation_problems(selfd_f, read_fixture, debt=0)
    if not selfd_v.indeterminate:
        fails.append("self-row-exclusion-does-not-swallow-rivals: a same-file citation whose "
                     "fragment also sits on a third row passed as checked -- the one exclusion "
                     "ate the rule it lives inside")
    # and the bucket refuses, it does not ratchet: no new debt, and the ratchet stays shut while
    # one exists, the #367-review-#1 discipline every must-be-zero bucket obeys.
    counts[0] += 1
    held = citation_problems(cites('`dupes.md:2` "the anchor appears on two rows of this file".\n'),
                             read_fixture, debt=0)
    if held.judged or not any("NOT judged this run" in p for p in held.problems):
        fails.append("indeterminate-holds-ratchet: with one indeterminate the run still compared "
                     "the debt -- %s" % held.problems)
    if held.measured:
        fails.append("indeterminate-is-not-debt: an ambiguous anchor entered the debt (%d) instead "
                     "of the refusal list -- ratcheting it would teach lowering the constant to "
                     "hide ambiguity" % held.measured)
    # #495: the quotation must cover a WHOLE token of the row, not merely sit inside it. The fixture
    # carries both fates of one fragment and the legs read that BEFORE they trust any refusal: a corpus
    # of nothing but cuts would pass every `cite_case` below with a predicate that fires on everything,
    # so the precondition is taken off `anchor_cut` on the two rows directly, and every refusal is
    # answered by a clean leg on the row it is aimed at.
    tok = read_fixture("tokens.md").splitlines()
    counts[1] += 2
    if anchor_cut("read the escalation label", tok[0]) is not None:
        fails.append("anchor-cut-fixture-not-whole: `tokens.md:1` is the leg's control and the "
                     "predicate already calls the fragment a cut there -- the row does not carry it as "
                     "a whole word, so no leg below can tell the rule from a stub")
    if anchor_cut("read the escalation label", tok[1]) is None:
        fails.append("anchor-cut-fixture-not-cut: `tokens.md:2` is meant to carry the fragment as the "
                     "head of a longer word and the predicate calls it whole -- every cut leg below "
                     "would then be asserting nothing")
    cite_case('`tokens.md:2` "read the escalation label".\n', "anchor-cut-end-refused",
              "carries 's' more of the same word immediately after the quote")
    cite_ok('`tokens.md:2` "read the escalation labels from common.sh".\n', "anchor-cut-end-repaired")
    # The class #495 was filed on, verbatim, and the repair it needs: the whole token, no further.
    cite_case('`tokens.md:3` "FACTORY_ESCALATION_LABEL=${FACTORY_ESCALATION_LABEL:-".\n',
              "anchor-cut-substitution", "carries 'question}' more of the same word")
    cite_ok('`tokens.md:3` "FACTORY_ESCALATION_LABEL=${FACTORY_ESCALATION_LABEL:-question}".\n',
            "anchor-cut-substitution-repaired")
    # The cut at the other end of the quotation: the row carries `-name`, the quote opens on `name`.
    cite_case('`tokens.md:5` "name \'triage-stderr.*\' ".\n', "anchor-cut-start-refused",
              "carries '-' more of the same word immediately before the quote")
    cite_ok('`tokens.md:5` "-name \'triage-stderr.*\'".\n', "anchor-cut-start-repaired")
    # The two marks the token alphabet leaves out, priced by the legs that pass because of it: a quote
    # that stops at a dotted name's dot, and one that stops at a slash between two words.
    cite_ok('`tokens.md:6` "the gate script is check-decisions".\n', "anchor-cut-dot-not-a-token")
    cite_ok('`tokens.md:4` "the boundary of the add".\n', "anchor-cut-slash-not-a-token")
    # A quotation that ends where the row ends is whole: the row's own edge is a boundary, which is
    # what keeps the rule off every sentence-ending anchor in the logs.
    cite_ok('`tokens.md:2` "from common.sh and print them".\n', "anchor-cut-row-edge-is-boundary")
    # And a range's joined seam is a boundary too, so the rule cannot manufacture a cut at the place
    # `classify` itself put a space.
    cite_ok('Lines `tokens.md:1-2` "and print it read the escalation labels".\n',
            "anchor-cut-join-seam-is-boundary")
    # The conservative arm, with its control: one whole match anywhere on the cited text is enough to
    # pass, because the lengthening that would fix the other match is advice this check cannot give;
    # the same fragment with nothing but matches cut is refused.
    cite_ok('`tokens-mixed.md:1` "count the word token".\n', "anchor-cut-one-whole-match-passes")
    cite_case('`tokens-cut-twice.md:1` "count the word token".\n', "anchor-cut-all-matches-cut",
              "carries 's' more of the same word immediately after the quote")
    # and the bucket refuses rather than ratchets, the #367-review-#1 discipline again: the cut row
    # enters no number, and the ratchet stays shut while one exists.
    counts[0] += 1
    cut_held = citation_problems(cites('`tokens.md:3` '
                                       '"FACTORY_ESCALATION_LABEL=${FACTORY_ESCALATION_LABEL:-".\n'),
                                 read_fixture, debt=0)
    if cut_held.judged or not any("NOT judged this run" in p for p in cut_held.problems):
        fails.append("midtoken-holds-ratchet: with one cut quotation the run still compared the debt "
                     "-- %s" % cut_held.problems)
    if cut_held.measured:
        fails.append("midtoken-is-not-debt: a quotation stopped inside a token entered the debt (%d) "
                     "instead of the refusal list -- ratcheting it would teach lowering the constant "
                     "to hide a cut" % cut_held.measured)
    # The report has to name the class in the one listing a person reads before editing the cited
    # file, and a cut row carries an anchor, so the row cannot be allowed to read `anchored`.
    counts[0] += 1
    # `main` classifies before it reports, and the listing reads the marks `classify` leaves on the
    # rows; the leg follows that order rather than handing the report unclassified rows, which is what
    # it would do if anything ever called it first.
    cut_body = ('`tokens.md:3` "FACTORY_ESCALATION_LABEL=${FACTORY_ESCALATION_LABEL:-" is the row.\n')
    cut_found = cites(cut_body)
    citation_problems(cut_found, read_fixture, debt=0)
    cut_rows = [l for l in citation_report(cut_found, read_fixture, only="tokens.md")
                if "-> tokens.md" in l]
    if len(cut_rows) != 1 or "#495" not in cut_rows[0]:
        fails.append("report-names-cut: `--citations tokens.md` read %s over a quotation the gate "
                     "just refused as cut -- the screen a person consults before re-aiming a pointer "
                     "cannot be the one place it looks clean" % cut_rows)
    # The filed class itself, as the logs write it: three rows citing the one line that names the
    # shipped default, each quoting the head of the substitution, beside the one row that carries the
    # whole of it. Reverting the predicate empties the first list, and this is the leg that notices --
    # `factory/DECISIONS.md` cannot be the only test of the rule that reads it.
    counts[0] += 1
    filed = cites('one line at `factory/common.sh:2` "FACTORY_ESCALATION_LABEL=${FACTORY_ESCALATION_LABEL:-"'
                  ' names a label this repository has, and the one home at `factory/common.sh:2` '
                  '"FACTORY_ESCALATION_LABEL=${FACTORY_ESCALATION_LABEL:-" is that same line, while the '
                  'wrong word `factory/common.sh:2` "FACTORY_ESCALATION_LABEL=${FACTORY_ESCALATION_LABEL:-"'
                  ' used to read needs-human; the fourth row, `factory/common.sh:2` '
                  '"FACTORY_ESCALATION_LABEL=${FACTORY_ESCALATION_LABEL:-question}", carries all of it;\n')
    filed_v = citation_problems(filed, read_fixture, debt=0)
    if len(filed_v.midtoken) != 3:
        fails.append("filed-class-is-refused: the shape #495 was filed on answered %d cut quotation(s) "
                     "where the logs carry three -- either the predicate is off or this leg has drifted "
                     "away from the rows it reproduces: %s" % (len(filed_v.midtoken), filed_v.midtoken))
    if filed_v.measured or len(filed) != 4:
        fails.append("filed-class-is-not-debt: the four rows of the filed class did not scan as four "
                     "anchored citations (%d scanned, %d in the debt) -- the leg would pass a rule that "
                     "counts cuts as debt" % (len(filed), filed_v.measured))
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
    # #440: the pinned half still NAMES the file, so a bare `:NNN` later in the sentence must
    # inherit it. Before the pin, `notes.md:2` passed its file to a `:3` continuation; pin it to
    # `deadbeef:notes.md:2` and the continuation silently became `not-a-path`, checked by nothing
    # -- measured on the repaired corpus, where the row pinning `factory/selftest.sh:10466`
    # orphaned the `:263`/`:265` pair on the next row. Both polarities: the right anchor passes,
    # and a wrong one is STILL refused (proving the continuation inherited a file and was checked;
    # `not-a-path` would pass either way, which is the hole).
    cite_ok('`deadbeef:notes.md:2` names that row at its revision; its `:3` '
            '"the third line is long enough to anchor a citation to" inherits the file.\n',
            "pinned-half-carries-the-file")
    cite_case('`deadbeef:notes.md:2` names that row at its revision; its `:1` "the third line is '
              'long enough to anchor a citation to" lands off it.\n',
              "pinned-half-continuation-checked", "it is at line 3 now, not 1")
    # #369: a newline-cut anchor is a stated refusal, not silent debt. The rule reads only the
    # coordinate's physical line, and these logs wrap at ~110 columns, so a faithful fragment most often
    # lands past a newline -- and until this entry the row left the rule as unanchored debt with nothing
    # saying the newline was why (the writers of `H-0171` hit it twice and read it as the ratchet rising;
    # the `H-0179` merge entry shipped five rows this classifier now names). Both cuts refuse by name,
    # neither row feeds the count, and the refusal must hold the ratchet: with a cut pointer on the
    # books the number is NOT judged, or a tree full of them would order its own disarmament.
    counts[0] += 2
    cut = citation_problems(cites('`docs/factory.md:3` ends this line and the quotation does not.\n'
                                  '"the escalation label is printed by exactly one script" opens the next.\n'),
                            read_fixture, debt=1)
    if not any("NEXT physical line" in p and "NEWLINE" in p for p in cut.problems):
        fails.append("anchor-next-line-refused: a quotation opening the line below passed unrefused, or "
                     "was refused without naming the newline -- %s" % cut.problems)
    if cut.measured != 0:
        fails.append("anchor-next-line-refused: the cut row was counted as debt instead of refused -- "
                     "measured %d, wrapped %d" % (cut.measured, len(cut.wrapped)))
    if cut.judged or "NOT judged" not in " ".join(cut.problems):
        fails.append("anchor-next-line-refused: the ratchet judged a number while a pointer sat "
                     "uncuttable on the line below -- %s" % cut.problems)
    dangling = citation_problems(cites('`docs/factory.md:3` "the escalation label is printed by exactly '
                                       'one script\nand the closing quote sits a newline below the rule.\n'),
                                 read_fixture, debt=1)
    if not any("NEWLINE cut it" in p for p in dangling.problems):
        fails.append("anchor-cut-midquote-refused: a quotation opened in anchor position and unclosed at "
                     "end of line passed unrefused, or was refused without naming the newline -- %s"
                     % dangling.problems)
    if dangling.measured != 0 or not dangling.wrapped:
        fails.append("anchor-cut-midquote-refused: the cut row was counted as debt instead of refused -- "
                     "measured %d, wrapped %d" % (dangling.measured, len(dangling.wrapped)))
    # And the two shapes that must NOT be accused. A quotation that merely sits two lines away was
    # measured and kept as debt in #367 -- widening the class reclassified other units' true pointers --
    # and a coordinate whose tail is an OPEN code span carries wrapped SHELL, not a cut quotation: the
    # `H-0142` row's `gh issue edit` command is that shape, and it stays debt, refused by nothing.
    counts[1] += 2
    below = citation_problems(cites('`docs/factory.md:3` "the escalation label is printed by exactly one '
                                    'script" is anchored here.\nAnd a later sentence quotes:\n'
                                    '"another line entirely".\n'), read_fixture, debt=0)
    if below.problems or below.wrapped:
        fails.append("quote-below-not-cut-anchor: an anchored citation gained a phantom cut-anchor "
                     "refusal from a quote two lines down -- %s" % below.problems)
    span = citation_problems(cites('the wrapped command: `docs/factory.md:3` -- `gh issue edit "$ISSUE" '
                                   '--add-label\n"$(factory/escalation-label.sh)"` is shell inside a '
                                   'code span, not a cut anchor.\n'), read_fixture, debt=1)
    if span.wrapped or span.measured != 1:
        fails.append("code-span-wrap-stays-debt: a coordinate inside an open code span was refused as "
                     "cut, or not counted as debt -- wrapped %s, measured %d"
                     % (span.wrapped, span.measured))
    # The CURRENT-STATE citation (#374): the sentence says the text IS at the coordinate in the tree
    # being gated, so the row is an assertion the next merge falsifies with every gate green -- the
    # silence H-0139's corrective bullet lived in for a week. Four exits and the ratchet hold: refuse
    # the marker, pass the anchored one, fall back to counted debt for a past tense, keep the marker
    # out of the debt number either way, and hold the ratchet shut while any refused row exists.
    counts[0] += 3
    now_row = citation_problems(cites('the function is at `docs/factory.md:3` and readers follow it.\n'),
                                read_fixture, debt=0, judge_debt=False)
    if not now_row.nowstate:
        fails.append("now-state-refused: `is at` beside a coordinate was not refused as current-state "
                     "-- %s" % now_row.problems)
    if now_row.measured:
        fails.append("now-state-not-debt: a refused current-state row was still counted in the "
                     "ratchet number -- %d" % now_row.measured)
    if not any("CURRENT-STATE" in p for p in now_row.problems):
        fails.append("now-state-names-itself: the refusal did not name the class -- %s"
                     % now_row.problems)
    counts[0] += 1
    wrapped_verb = citation_problems(cites('`notes.md:2`\n  now reads the one-line rule, which is\n'
                                           'what the bullet measured.\n'),
                                     read_fixture, debt=0, judge_debt=False)
    if not wrapped_verb.nowstate:
        fails.append("now-state-window: the verb sat on the line AFTER the coordinate -- the exact "
                     "H-0139 bullet shape, wrapped logs put it there -- and the row passed -- %s"
                     % wrapped_verb.problems)
    counts[1] += 1
    anchored_now = citation_problems(
        cites('the rule is at `docs/factory.md:3` "the escalation label is printed by exactly one '
              'script", so this row checks.\n'), read_fixture, debt=0, judge_debt=False)
    if anchored_now.nowstate or anchored_now.problems:
        fails.append("now-state-anchored-passes: an anchored current-state row was refused -- %s / %s"
                     % (anchored_now.problems, anchored_now.nowstate))
    counts[1] += 1
    past_now = citation_problems(
        cites('the function was at `notes.md:2` on 12 Nov and nobody re-read it.\n'),
        read_fixture, debt=0, judge_debt=False)
    if past_now.nowstate or past_now.measured != 1:
        fails.append("now-state-past-is-debt: a past-tense coordinate left the counted debt or was "
                     "refused -- nowstate %d, measured %d"
                     % (len(past_now.nowstate), past_now.measured))
    counts[0] += 1
    held = citation_problems(cites('the read is at `notes.md:2` today.\n'), read_fixture, debt=0)
    if held.judged or not any("NOT judged this run" in p for p in held.problems):
        fails.append("now-state-holds-ratchet: the number was judged while a must-be-zero bucket was "
                     "non-empty -- judged %s, %s" % (held.judged, held.problems))
    counts[1] += 1
    # The two mis-sorts #374 said prose-matching "will mis-sort", pinned silent. Both rows were
    # caught by the first implementation -- a verb-regex over the coordinate's line plus one -- and
    # both are sentences about something OTHER than a line position: a verb three clauses away, and
    # a preposition attaching the read to a named revision. A control that started firing would
    # refuse true sentences, which is the failure mode that retires a gate.
    distant = citation_problems(
        cites('the classifier table\u2019s row `docs/factory.md:3` is long, and the `Effect:` now '
              'reads a census of its own making.\n'), read_fixture, debt=0, judge_debt=False)
    if distant.nowstate:
        fails.append("now-state-missort-distant: a verb three clauses from the coordinate refused a "
                     "row about table length -- %s" % distant.nowstate)
    if distant.measured != 1:
        fails.append("now-state-missort-distant: the unanchored row left the counted debt -- %d"
                     % distant.measured)
    counts[1] += 1
    atrev = citation_problems(
        cites('the read `notes.md:2` at `d7a5149` was true there.\n'),
        read_fixture, debt=0, judge_debt=False)
    if atrev.nowstate:
        fails.append("now-state-missort-revision: a read attached to a named head was refused as a "
                     "claim about the gated tree -- %s" % atrev.nowstate)
    counts[1] += 1
    plural = citation_problems(
        cites('and all 28 surviving mentions in `notes.md:2` are inside this record now.\n'),
        read_fixture, debt=0, judge_debt=False)
    if plural.nowstate:
        fails.append("now-state-missort-predicate: \"are inside\", not \"are at\", refused a row "
                     "about where mentions are recorded -- %s" % plural.nowstate)
    # the boundary witness: this unit's own entry text "one bare at `file:3113`" fired the shipped
    # matcher at first, because "bare at" carries "are at" as a substring ("this at" carries
    # "is at"). The guard is `\b` on every verb group; this leg refuses a re-loosening.
    substring = citation_problems(
        cites('11 numbers the checkpoint left stale and one bare at `notes.md:2` stay as debt.\n'),
        read_fixture, debt=0, judge_debt=False)
    if substring.nowstate:
        fails.append("now-state-missort-substring: \"bare at\" contains \"are at\", and the refusal "
                     "of that English is how a gate gets retired -- %s" % substring.nowstate)
    counts[1] += 1
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
    # the SAME SENTENCE. Inheriting across a finished sentence would silently re-aim a pointer at
    # whatever file the previous sentence happened to name -- which, in logs wrapped at ~110 columns,
    # is what the physical-line rule did for a whole paragraph at a time (#365).
    counts[0] += 2
    cont = cites('`notes.md:3` "the third line is long enough" and the row at `:1` agree.\n')
    tail = [c for c in cont if c["continued"]]
    if len(tail) != 1 or tail[0]["path"] != "notes.md":
        fails.append("continuation: `:1` did not inherit notes.md from its own sentence -- %s" % cont)
    orphan = cites("and the row at `:1` agrees, in a sentence naming no file.\n")
    if [c for c in orphan if c["kind"] != "not-a-path"]:
        fails.append("continuation-orphan: a bare `:1` with no file in its sentence resolved to something "
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
    # #367 review #2: a citation that names no file was skipped instead of refused, and one of the
    # shapes that got away was a `:NNN` continuation.
    #
    # `last` holds a RESOLVED path, and the continuation re-resolved it by name. For `./SKILL.md:1`
    # -- a root file whose basename three files answer to -- that returned three candidates, the
    # no-guess guard turned them into "no file", and the continuation fell out as `not-a-path`: a
    # coordinate in a decision log checked by nothing. The fixture already carries the ambiguity, so
    # the case below is the bug as written, not a new file invented to hold it.
    counts[0] += 3
    counts[1] += 1
    ROOT_SKILL = "the root skill file is the one a bare name must not silently choose"
    cite_case('`./SKILL.md:1` "%s", again at `:2` "%s".\n' % (ROOT_SKILL, ROOT_SKILL),
              "continuation-of-ambiguous-parent-is-checked", "which has 1 lines")
    cite_ok('`./SKILL.md:1` "%s", again at `:1` "%s".\n' % (ROOT_SKILL, ROOT_SKILL),
            "continuation-of-ambiguous-parent-resolves")
    # A bare name with an extension is a promise about a file. Before #367 the absent-file arm could
    # only be reached by a written value containing `/`, so this exact body -- the reviewer's own
    # repro, verbatim -- produced zero problems and left the debt untouched.
    cite_case('`gonee.md:3` "the third line is long enough to anchor a citation".\n',
              "bare-name-gone-anchored", "there is no file to read it in")
    cite_case('`gonee.md:3` is where it said so.\n',
              "bare-name-gone-unanchored", "never lived where the entry said")
    # The exemption has to survive the widening, and these are shapes both logs are full of. None
    # carries an extension, so no tree can ever answer to them and refusing them would refuse the
    # prose that reports triage output, quotes a symbol, or names an entry id and a line about it.
    for shapes, name in (
            ("`pr:209`, `unclaimed:263` and `STATE_PROPS:1040` are triage classes and a symbol.\n",
             "class-pair-and-symbol-stay-exempt"),
            ("`D-0033:236` is the id of `H-0153:45` in prose, and `verify:1` and `make:2` are a verb "
             "and a tool.\n", "id-prose-and-verb-stay-exempt")):
        verdict = citation_problems(cites(shapes), read_fixture, debt=0)
        if verdict.problems or verdict.measured:
            fails.append("%s: a citation-shaped token with no extension was refused, or counted as "
                         "debt -- %s" % (name, verdict.problems))
        else:
            counts[1] += 1
    # A bare `:NNN` with no file named earlier in its own sentence names no file at all, and is still
    # NOT refused: that is the documented open shape of the inheritance rule (#365 -- the sentence,
    # never the last line read, and never a paragraph that wrapped). Measured at `e66dc85d` by that
    # tree's own scanner over its own two logs (H-0258; the 294/137/157 this comment carried were
    # that rule over `adddacc`'s logs, whose own scanner answers 294/95/199): 312 rows carry a bare
    # `:NNN`, 137 inherit a file from their own sentence, 175 name no antecedent -- `H-0153`'s
    # "158 rows" predates both the extension rule and this one and does not re-measure. Refusing
    # the 175 refuses 175 true sentences that merely wrap.
    # Pinned as a must-pass so a widening cannot swallow it by accident.
    counts[1] += 1
    verdict = citation_problems(cites("`:19` continues nothing on this line and stays a limit.\n"),
                                read_fixture, debt=0)
    if verdict.problems or verdict.measured:
        fails.append("orphan-continuation-still-a-limit: the sentence-limit became a defect -- %s"
                     % verdict.problems)
    # ---------------------------------------------------------------------------------------------
    # #367 review #3: the files that teach the rule were the one place a false pointer could not be seen,
    # because the rule read only the two logs. Five legs refuse a bad pointer in a guide, four hold the
    # exemptions open -- the one that matters is the unanchored coordinate, which is a debt in a log and
    # is NOT a defect in a guide.
    counts[0] += 5
    counts[1] += 3

    def teach(body):
        read = dict(CITES_TREE, **{"guide.md": body}).get
        return teaches_problems(read, CITES_INDEX, paths=("guide.md",))

    for body, name, needle in (
            ('read `SKILL.md:1` by its bare name here.\n', "guide-ambiguous-basename",
             "names 3 files"),
            ('read `docs/factory.md:2` "the escalation label is printed by exactly one script".\n',
             "guide-anchor-moved", "it is at line 3 now, not 2"),
            ('read `gonee.md:3` for it.\n', "guide-gone-file", "not in the tree being gated"),
            # #440: the guide is where an ambiguous anchor does the most damage -- it is the shape
            # every unit copies, and the boilerplate anchors #440 traces were copied exactly that
            # way under a gate that graded them good. Inherited, not filtered.
            ('read `dupes.md:2` "the anchor appears on two rows of this file".\n',
             "guide-ambiguous-anchor", "also sits on 3"),
            # #369: a guide that shows the anchor rule with a quotation the newline cut demonstrates
            # the very exit the rule now refuses, and every unit copying it writes silent debt.
            ('read `docs/factory.md:3` and quote it on the line below instead:\n'
             '"the escalation label is printed by exactly one script".\n',
             "guide-wrapped-anchor", "NEXT physical line"),
            # #495: and the guide is where a quotation stopped inside a word does its damage -- the
            # boilerplate #495 found was copied from a guide that quoted a substitution's head, and a
            # unit copying it writes a pointer that cannot fail. Inherited, not filtered.
            ('read `tokens.md:3` "FACTORY_ESCALATION_LABEL=${FACTORY_ESCALATION_LABEL:-" for it.\n',
             "guide-cut-anchor", "stops inside a token")):
        probs = teach(body)
        if not any(needle in q for q in probs):
            fails.append("%s: expected a refusal containing %r, got %s" % (name, needle, probs))
    for body, name in (
            ('read `docs/factory.md:3` "the escalation label is printed by exactly one script".\n',
             "guide-anchor-exact"),
            ('read `docs/factory.md:3` and that is the whole promise a guide owes.\n',
             "guide-may-go-unanchored"),
            ('`pr:209` and an orphan `:19` are still not citations here either.\n',
             "guide-shapes-stay-exempt")):
        probs = teach(body)
        if probs:
            fails.append("%s: a guide was refused for a shape the rule does not owe it -- %s"
                         % (name, probs))
    counts[1] += 1
    if teaches_problems(CITES_TREE.get, CITES_INDEX, paths=("no-such-guide.md",)):
        fails.append("guide-absent-is-not-false: a fixture tree with no skill file is refused for a "
                     "file it does not have")

    # The pin on a wrapped anchor is inverted. The rule reads the coordinate's own physical line, so a
    # quotation cut by the newline left the citation in the debt instead of checked: a number moved and
    # nothing was refused. This same fixture used to assert `measured == 1` -- that silence is exactly
    # what #369 was filed on (the writers of H-0171 hit it twice and read it as the ratchet rising, and
    # these logs wrap at ~110 columns, which is exactly where a 20-character fragment wants to go). The
    # sentence that used to pin the silence now pins its refusal, and the unwrapped same sentence is
    # still checked rather than counted.
    counts[0] += 1
    counts[1] += 1
    body = ('`docs/factory.md:3` "the escalation label is printed by\n'
            '  exactly one script" continues onto the next line.\n')
    verdict = citation_problems(cites(body), read_fixture, debt=1)
    if not verdict.wrapped or verdict.measured != 0:
        fails.append("wrapped-anchor-refused: the newline-cut quotation %s -- measured %s, %s"
                     % ("is still silent debt" if not verdict.problems else "was refused but still counted",
                        verdict.measured, verdict.problems))
    body = ('`docs/factory.md:3` "the escalation label is printed by exactly one script" stays on its '
            'own line.\n')
    verdict = citation_problems(cites(body), read_fixture, debt=0)
    if verdict.measured != 0 or verdict.problems:
        fails.append("same-line-anchor-is-checked: the unwrapped form of that sentence should be "
                     "checked, not counted -- %s" % verdict.problems)

    # #365: the continuation boundary is the SENTENCE, not the physical line, and a paragraph of these
    # logs is several wrapped sentences naming several files. Four legs, each the shape the line rule
    # got wrong or the sentence rule could get wrong.
    # 1. Wrapped continuation: the file sits on the earlier physical line of one sentence. The line
    #    rule saw no file on the `:1` line and left the row invisible -- not debt, not checked.
    counts[1] += 1
    cont = cites('`notes.md:3` "the third line is long enough to anchor a citation to" wraps\n'
                 'onto the next line, and the row at `:1` agrees.\n')
    tail = [c for c in cont if c["continued"]]
    if len(tail) != 1 or tail[0]["path"] != "notes.md":
        fails.append("continuation-wrapped-is-aimed: a `:1` one wrapped line inside its sentence %s "
                     "-- %s" % ("did not inherit notes.md" if not tail else "inherited "
                                + str(tail[0]["path"]), cont))
    # 2. Two files in one sentence: the NEAREST antecedent wins, not the first, and not the file
    #    another sentence named -- the store is per sentence, so a global-leak bug shows here.
    counts[1] += 1
    cont = cites('`notes.md:3` "the third line is long enough to anchor a citation to" and '
                 '`docs/factory.md:3` "the escalation label is printed by exactly one script", and '
                 'again at `:1`.\n')
    tail = [c for c in cont if c["continued"]]
    if len(tail) != 1 or tail[0]["path"] != "docs/factory.md":
        fails.append("continuation-nearest-antecedent: `:1` after two files in one sentence inherited "
                     "%s, not docs/factory.md -- %s" % (tail[0]["path"] if tail else "nothing", cont))
    # 3. A period ends a sentence even on one physical line, where the line rule happily inherited.
    counts[1] += 1
    cut = cites('`docs/factory.md:3` "the escalation label is printed by exactly one script". The row '
                'at `:2` agrees.\n')
    if [c for c in cut if c["continued"]]:
        fails.append("continuation-period-cuts: `:2` in a new sentence inherited the previous "
                     "sentence's file -- %s" % cut)
    # 4. A markdown bullet marker opening a line is a boundary even with no period before it -- these
    # logs are bullet lists, and the line rule crossed the bullet only by accident (the previous line
    # ending the file clause). Remove the bullet terminator and `:1` inherits across the list.
    counts[1] += 1
    cut = cites('- `notes.md:3` "the third line is long enough to anchor a citation to"\n'
                '- and the row at `:1` agrees.\n')
    if [c for c in cut if c["continued"]]:
        fails.append("continuation-bullet-cuts: `:1` in its own bullet inherited the previous bullet's "
                     "file -- %s" % cut)

    # ... and the affordance still answers the question a deletion asks -- what points at this file --
    # when the answer is a file that is already gone.
    counts[0] += 1
    gone_rows = [l for l in citation_report(cites("`gonee.md:3` is where it said so.\n"),
                                           read_fixture, only="gonee.md")
                 if "-> gonee.md" in l]
    if len(gone_rows) != 1:
        fails.append("gone-file-census: --citations gonee.md could not find the dead pointer it just "
                     "refused -- %s" % gone_rows)

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

    # One-file-per-entry (#429 phase 1). `layout_problems` and `entry_dir_files` are both pure functions
    # of a label or a root for exactly this reason: the shapes are asserted as text and as a generated
    # tree, never as an editable fixture file -- a fixture a person can edit is a fixture that quietly
    # stops asserting. The end-to-end pair (the shipped reader over a fixture repository, and what git
    # does when two branches file the same id) lives in `factory/selftest.sh`, because it needs a tree.
    counts[1] += 1
    filed = ("## H-0042 · An entry filed as its own file\n\n- Question: does the filename carry the id?\n"
             "- Options: it does | the heading does\n- Chose: it does, and the heading repeats it\n"
             "- Why: two branches claiming one id then collide in git.\n- Checkpoint: n/a\n")
    if layout_problems("DECISIONS/H-0042.md", parse(filed, "DECISIONS/H-0042.md")):
        fails.append("filed-clean: a filed entry whose filename is its id was refused -- %s"
                     % layout_problems("DECISIONS/H-0042.md", parse(filed, "DECISIONS/H-0042.md")))
    counts[0] += 1
    got = layout_problems("DECISIONS/H-0042.md", parse(filed.replace("H-0042", "H-0043"), "DECISIONS/H-0042.md"))
    if not any("the filename says H-0042" in g for g in got):
        fails.append("filed-id-disagrees: %r is what a heading that contradicts its filename earned; "
                     "a reader following the name and a reader following the heading would read two "
                     "different decisions" % (got,))

    counts[0] += 1
    two = filed + "\n" + filed.replace("H-0042", "H-0043")
    got = layout_problems("DECISIONS/H-0042.md", parse(two, "DECISIONS/H-0042.md"))
    if not any("2 entry headings" in g for g in got):
        fails.append("filed-two-entries: two decisions in one filed file read %r; the filename is one id, "
                     "so the pair would answer one citation with two decisions" % (got,))

    counts[0] += 1
    got = layout_problems("factory/DECISIONS/notes.md", parse(filed, "factory/DECISIONS/notes.md"))
    if not any("is not an entry id" in g for g in got):
        fails.append("filed-not-an-id: a file named nothing-a-reader-can-resolve read %r" % (got,))

    # The rule is layout-scoped, and the control proves it: the monoliths hold hundreds of headings each
    # and must answer nothing here, or every run since before this phase would be red.
    counts[1] += 1
    if layout_problems("factory/DECISIONS.md", parse(two, "factory/DECISIONS.md")):
        fails.append("monolith-unaffected: the filename rule was applied to a monolith -- %s"
                     % layout_problems("factory/DECISIONS.md", parse(two, "factory/DECISIONS.md")))

    # The duplicate the layout is designed to make loud: one id, one filed file and one monolith. The
    # message has to name both places and offer the verb that fits a filed entry (a fresh id from the
    # allocator), while the monolith-only collision keeps the advice it has always given.
    counts[0] += 1
    got = cross_log_problems({"H-0042": ["DECISIONS/H-0042.md", "factory/DECISIONS.md"]})
    if not (any("DECISIONS/H-0042.md" in g and "factory/DECISIONS.md" in g for g in got)
            and any("next-decision-id.sh" in g for g in got)):
        fails.append("filed-duplicate: a cross-layout duplicate read %r; it has to name both files and "
                     "offer the allocator, which is the only thing that can give a filed entry a new id"
                     % (got,))
    counts[1] += 1
    got = cross_log_problems({"H-0042": ["DECISIONS.md", "factory/DECISIONS.md"]})
    if not any("Renumber the later entry in its own log." in g for g in got):
        fails.append("monolith-duplicate-advice: the monolith collision lost the sentence that tells a "
                     "person to renumber in place -- %s" % (got,))

    # What the reader walks out of a tree. `log_paths` and `entry_dir_files` take a root for this case:
    # the tree is built here and thrown away, so nothing in the repository can be edited into agreeing.
    counts[0] += 1
    counts[1] += 1
    import shutil
    import tempfile
    sandbox = tempfile.mkdtemp(prefix="plock-layout-")
    try:
        os.makedirs(os.path.join(sandbox, "factory"))
        paths, problem = log_paths(sandbox)
        if [os.path.relpath(x, sandbox).replace(os.sep, "/") for x in paths] != list(LOG_REL) or problem:
            fails.append("absent-dirs: a tree with no `DECISIONS/` read %r / %r -- phase 1 lands while "
                         "every entry is still in a monolith, so this is the repository's own shape and "
                         "must not be an error" % (paths, problem))
        filed_dir = os.path.join(sandbox, "DECISIONS")
        os.makedirs(os.path.join(filed_dir, "deep"))
        for name in ("H-0002.md", "H-0001.md", "notes.txt"):
            with open(os.path.join(filed_dir, name), "w", encoding="utf-8") as fh:
                fh.write("x")
        with open(os.path.join(filed_dir, "deep", "H-0003.md"), "w", encoding="utf-8") as fh:
            fh.write("x")
        paths, problem = entry_dir_files(sandbox)
        if paths != ["DECISIONS/H-0001.md", "DECISIONS/H-0002.md"]:
            fails.append("dir-scan: `%s/` read %r -- expected the two `.md` files at the top level, "
                         "sorted, and nothing else" % ("DECISIONS", paths))
        if not (problem and "deep/H-0003.md" in problem):
            fails.append("nested-entry-unread: an entry filed below a directory was accepted silently "
                         "(%r); the reader would skip it and no rule would ever run over it" % (problem,))
    finally:
        shutil.rmtree(sandbox, ignore_errors=True)

    # Issue #389: an `Effect:` that quotes a diff tally -- `adds 429 (nine headings)` -- records what the
    # diff answered the day the entry was written, and the merge it describes then grows the log past its
    # own receipt. `H-0093`'s two did exactly that inside their own merge: 54 went to 112 and 429 to 504,
    # one heading and one entry's prose apiece, and no later entry named it -- the unevenness the ticket
    # was filed on, since `H-0096` and `H-0101` each annotate the stale tallies they left alone. The
    # shipped gate only COUNTS (`measure_no_history`: re-running a historical diff is the refs-and-history
    # read `gate_reads_no_refs` denies this file), so the arm that decides "stale" is asserted here against
    # an injected fixture diff, and the report is asserted NOT to be a refusal -- a gate that blocked on
    # drift would make every merge that grows the log a blocked merge, #318's lesson read from the number
    # side. Must-trip cases here are cases the REPORT must fire, counted with the refusals.
    def tally_fixture(text, name):
        parsed = parse(text, name)
        claims = tally_claims(text, parsed)
        for c in claims:
            c["label"] = name
        return claims

    STALE_TALLY = ("## H-0404 · A merge keeps both sides and reads the diff back\n\n"
                   "- Question: does the resolution keep the other side's lines?\n"
                   "- Options: read the diff back | declare it kept\n"
                   "- Chose: read the diff back\n"
                   "- Why: a receipt is a read, not a declaration.\n"
                   "- Checkpoint: 2f4c1ab9\n"
                   "- Effect: the diff deletes 0 content lines and adds 54 (one heading), and each side\n"
                   "  loses none of the other's text.\n")

    counts[1] += 1
    stale_claims = tally_fixture(STALE_TALLY, "factory/DECISIONS.md")
    if len(stale_claims) != 1 or stale_claims[0]["adds"] != 54 or stale_claims[0]["headings"] != 1 \
            or stale_claims[0]["entry"] != "H-0404":
        fails.append("tally-claim-read: the receipt above read %r -- `adds N (M headings)` is the shape "
                     "H-0093 shipped and #389 measured, and a claim not found here is a report that can "
                     "never fire" % (stale_claims,))

    counts[0] += 1
    drifted = tally_verdict(stale_claims, lambda c: (112, 2))
    if drifted.mismatched != 1 or not drifted.reports:
        fails.append("tally-stale-reported: a claim of `adds 54 (one heading)` measured against the diff "
                     "that answers 112 lines / 2 headings -- #389's own pair -- reported %r"
                     % (drifted.reports,))
    elif not any("54" in r and "112" in r and "2 headings" in r and "H-0404" in r
                 for r in drifted.reports):
        fails.append("tally-report-names-both: the report does not hold the written figure and the "
                     "measured one beside the entry that wrote it -- %s" % drifted.reports)

    counts[1] += 1
    exact = tally_verdict(stale_claims, lambda c: (54, 1))
    if exact.reports or exact.mismatched or exact.re_measured != 1:
        fails.append("tally-exact-not-reported: the same claim measured against the diff it quotes was "
                     "reported stale -- a report that fires on a true receipt is a report a reader "
                     "learns to ignore: %s" % exact.reports)

    # Both figures are in the receipt, so either one drifting is drift: `H-0093`'s rows moved on lines
    # AND headings at once, and a comparison that read only the numstat would have called that clean.
    counts[0] += 1
    half = tally_verdict(stale_claims, lambda c: (54, 2))
    if half.mismatched != 1:
        fails.append("tally-headings-compared: a claim of (one heading) against a diff answering 2 "
                     "headings was not reported, and the heading count is the half the prose claims")

    # The shipped arm, which is the only arm `make verify` ever runs: nothing re-measured, nothing
    # reported, everything counted. This is the assertion that keeps the count a count -- a later edit
    # that reached for a real diff would fail HERE before it ever blocked a merge.
    counts[1] += 1
    shipped = tally_verdict(stale_claims, measure_no_history)
    if shipped.reports or shipped.re_measured or shipped.unverifiable != 1:
        fails.append("tally-shipped-counts-only: the shipped measure re-measured or reported, which is "
                     "the history read #253 denies this gate -- %r" % (shipped,))

    # A heading word outside `_TALLY_NUM` is counted and never compared: guessing the number the prose
    # did not write would let the report invent its own second figure.
    counts[1] += 1
    wordless = tally_fixture(STALE_TALLY.replace("adds 54 (one heading)", "adds 54 (seventeen headings)"),
                             "factory/DECISIONS.md")
    if len(wordless) != 1 or wordless[0]["headings"] is not None:
        fails.append("tally-unknown-word: `seventeen` is outside the word table and the claim was still "
                     "read as a number -- %r" % (wordless,))
    if tally_verdict(wordless, lambda c: (54, 17)).reports:
        fails.append("tally-unknown-word-compared: a claim whose heading word could not be read was "
                     "compared against a number the entry never wrote")

    # And the report is not a decision-log problem: the entry above is well-formed, and neither refusal
    # path sees its number. This is the assertion that keeps #389 a convention and not a gate.
    counts[1] += 1
    shape = check(parse(STALE_TALLY, "factory/DECISIONS.md"))
    if shape:
        fails.append("tally-not-a-problem: an entry carrying a re-measurably stale tally was refused as "
                     "a malformed decision -- %s" % shape)

    # The sentence a green run prints, since the count is the shipped behaviour and a reader cannot tell
    # a count from a verdict by its shape.
    counts[1] += 1
    if tally_line(0) is not None:
        fails.append("tally-line-empty: logs holding no diff tally still printed a tally line -- %r"
                     % tally_line(0))
    counts[1] += 1
    said = tally_line(2)
    if not (said and said.startswith("check-decisions: 2 ") and "none re-measured" in said
            and "#253" in said and "#389" in said):
        fails.append("tally-line-text: the clean-run sentence stopped carrying the count, or the words "
                     "`none re-measured`, or the two issue numbers that make it a count and not a "
                     "verdict about diffs this file never looked at -- %r" % (said,))

    # ---------------------------------------------------------------------------------------------
    # #440 rule legs over the REAL corpus: walk the logs that exist and re-derive the invariant the
    # bucket is supposed to hold -- no anchored citation is accepted while its fragment answers a row
    # outside the cited range and outside the citing row of a same-file pointer. The module's own
    # scan_citations/parse/norm do the work, so the verdict is the gate's read, not a second
    # implementation that can drift from it (the `check-scripts.py` import argument). The tally is
    # deliberately NOT pinned: the corpus grows ~27 citations every four hours, and a leg naming
    # today's number is red before the branch merges. In the two-entry fixture repo selftest.sh runs
    # `--self-test` in, these loops simply find little or nothing, and an empty corpus is an empty
    # truth, not a failure.
    counts[1] += 1
    real_paths, real_walk = log_paths(ROOT)
    real_tree = tree_files(ROOT)[0]
    if real_walk is None and real_tree is not None:
        real_index = tree_index(real_tree)
        for real_log in real_paths:
            real_text, text_problem = read(real_log)
            if text_problem or real_text is None:
                continue
            real_rel = os.path.relpath(real_log, ROOT)
            for c in scan_citations(real_text, parse(real_text, real_rel), real_index):
                if c["kind"] in ("not-a-path", "revision-qualified", "ambiguous") or c.get("wrapped"):
                    continue
                c["label"] = real_rel
                if c["anchor"] is None:
                    continue
                body = read_tree(c["path"])
                if body is None:
                    continue
                rows = body.splitlines()
                want = norm(c["anchor"])
                if c["low"] > len(rows) or want not in norm(" ".join(rows[c["low"] - 1:c["high"]])):
                    continue  # the gate refuses these under other buckets; they are not accepts
                for i, row in enumerate(rows):
                    if want in norm(row) and not (c["low"] <= i + 1 <= c["high"]) \
                            and not (c.get("label") == c["path"] and c["line"] == i + 1):
                        fails.append(
                            "real-corpus-anchor-singular: `%s:%s` in %s matched row %d outside the "
                            "cited range -- the accept branch let an ambiguous anchor through again "
                            "(#440)" % (c["written"], c["low"], real_rel, i + 1))

    if fails:
        print("check-decisions --self-test FAILED:")
        for f in fails:
            print("  %s" % f)
        return 1
    print("check-decisions --self-test: %d cases that must fail, %d that must pass, the two "
          "invocations in `verify`, seven Makefiles each owing a named sentence, the cross-log id rule, "
          "checkpoint existence, the out-of-order refusal in both logs and in a file holding both, the "
          "`sha` (PR #N) provenance pair (agreement and drift, the multi-number landing, the non-merge "
          "whose message carries the number and the one that does not, the dead object, the unreadable "
          "message, the correction quotation that binds nothing, the pair a line-wrap splits, both "
          "git-blind answers, and the reader's own batch framing on synthetic bytes (the fixture's "
          "byte-versus-character differential in both directions, a multibyte record, that record last "
          "in the batch, a record that does not end where its header said, a header naming somebody "
          "else's object, and `<sha> missing`)), the `path:line` citation rule (anchor exact, moved, absent, past EOF, ambiguous, absent "
          "file, revision-qualified, class:PR prose and the extension line, inherited ranges and the "
          "continuation that inherits a file, the ratchet both ways, the ratchet held shut while a "
          "pointer cannot be opened, the refusal that names which anchor floor it applied, and the "
          "anchor that stops inside a token of the cited row (#495: the cut at each end with the "
          "whole-word repair beside it, the fixture's two preconditions read off the predicate "
          "itself, the dotted-name and slash controls the token alphabet buys, the row edge and the "
          "joined range seam as boundaries, the conservative many-match arm beside its control, the "
          "report naming a cut row instead of calling it anchored, and the ratchet held while one "
          "exists)), the "
          "one-file-per-entry layout (filename and heading agreeing or disagreeing, two decisions in one "
          "file, a file named by nothing, the monolith answering none of it, the cross-layout duplicate "
          "naming both places, and a tree with no entry directory at all), and the `Effect:` diff-tally "
          "count (a receipt recognised, drift reported with both figures beside each other, agreement "
          "not reported, a heading word outside the table counted rather than guessed, the shipped arm "
          "counting without measuring, and the sentence a green run prints), each asserted both ways"
          % (counts[0], counts[1]))
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

    # `--log PATH` reads exactly one file, and that is the only thing about `--log` that narrows: the
    # union -- the two monoliths plus whatever is filed in `DECISIONS/` and `factory/DECISIONS/` -- is
    # the default invocation's subject. A `--log` run therefore cannot see a cross-layout duplicate, and
    # its USAGE text says so instead of being quietly contradicted here.
    if path != LOG:
        paths, dirs_problem = [path], None
    else:
        paths, dirs_problem = log_paths(ROOT)
    # The debt is a number about THIS pair of logs. Narrowing the file narrows everything the rules
    # can see, and `POINTER_DEBT` describes the pair -- counting one log against it would refuse every
    # `--log` run with a number that means nothing. So the debt is not judged here, and the line below
    # says so rather than going quiet about it: `--log` used to skip two rules silently, which is what
    # a fabricated checkpoint survived (#229).
    judge_debt = path == LOG
    tree, walk_problem = tree_files()
    index = tree_index(tree or [])
    problems = []
    if dirs_problem:
        # Fail closed in the shape of every other unreadable input here: a directory the reader could
        # not list is an unevaluated duplicate rule, not a passed one.
        problems.append("entry directories: %s" % dirs_problem)
    total = 0
    found = []
    tally = []
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
        # One entry whose id is its filename, for the files filed under `DECISIONS/`. A monolith answers
        # nothing here: many headings in one file is the shape being migrated away from, and this is the
        # rule that only exists because of the filename.
        problems += layout_problems(label, entries)
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
        # #433: the same git question the checkpoint rule asks, aimed at the OTHER git token the
        # rules used to skip -- a commit glued to a `(PR #N` reference inside prose. The existence
        # resolver is reused whole (it answers "is a commit", which is exactly the pair's first
        # demand, including the fail-closed `None`); the message reader is then asked only for
        # shas that resolved, so a dead one is refused by the resolver's sentence and never
        # reaches a `git log` that would fail the whole batch for it.
        pair_shas = sorted({(pm.group(1) or pm.group(2)) for pm in PROV_PAIR.finditer(text)})
        pair_exist = git_checkpoint_resolver(pair_shas)
        pair_live = sorted(s for s in pair_shas if pair_exist and pair_exist.get(s))
        problems += provenance_pair_problems(text, entries, pair_exist,
                                             git_pair_message_reader(pair_live))
        # Everything above this line judges the list `parse` returned, so the first rule that has to
        # look at the raw bytes is the one that asks whether that list is the whole log.
        problems += invisible_heading_problems(text, label)
        cites = scan_citations(text, entries, index)
        for c in cites:
            c["label"] = label
        found += cites
        claims = tally_claims(text, entries)
        for c in claims:
            c["label"] = label
        tally += claims

    if walk_problem:
        # Fail closed, in the shape of every other unreadable input here: an unevaluated rule is not a
        # passed rule.
        problems.append("citations: %s" % walk_problem)
        judged, debt_note, measured = False, None, 0
        verdict = CitationVerdict([], 0, [], [], [], [], [], [], False, "the tree cannot be walked")
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

    if not walk_problem and path == LOG:
        # Not under `--log`: that invocation narrows the corpus deliberately, and a run aimed at one file
        # is not a verdict on the repository's instructions either.
        problems += teaches_problems(read_tree, index)

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
        print("check-decisions: %d `path:line` citations read out of the two logs, %d of them owing "
              "an anchor and %d refused outright -- a reader cannot open them, the newline cut their "
              "anchor, #440's rule says an anchor answering several rows verifies nothing, or #495's "
              "says a quotation stopped inside a word -- %s"
              % (len([c for c in found if c["kind"] != "not-a-path"]), measured,
                 len(verdict.unresolvable) + len(verdict.broken) + len(verdict.wrapped)
                 + len(verdict.nowstate) + len(verdict.indeterminate) + len(verdict.midtoken),
                 "the first number is the recorded debt, which may only fall" if verdict.judged
                 else "the ratchet was NOT judged this run: %s" % verdict.unjudged_because))
    else:
        print("check-decisions: %d `path:line` citations checked for anchors; the debt number was not "
              "judged, because --log narrows the corpus it is a count of"
              % len([c for c in found if c["kind"] != "not-a-path"]))
    if path == LOG:
        # #389's count, and a count is all it is: the gate reads no history, so it can say how many
        # merge receipts the logs hold and not one word about whether any of them still lands. Printed
        # beside the debt line because both are numbers a reader follows into a historical claim, and
        # both are annotated forward rather than repaired in place.
        line = tally_line(len(tally))
        if line:
            print(line)
        # The id-citation census, over the same corpus the entries were read from -- `where_seen` is the
        # union of every heading this run parsed, monoliths and filed entries alike, which is exactly the
        # set a reader following an id expects to land in. A second pass over the texts rather than a
        # clause in the loop above, because the loop sits above lines the two logs quote by number.
        dangling = scan_id_mentions([(os.path.relpath(p, ROOT), read(p)[0]) for p in paths],
                                    set(where_seen))
        line = dangling_line(dangling)
        if line:
            print(line)
    return 0


# The one regex in this file that is deliberately WIDER than `ENTRY`, and therefore the only way this
# gate can see an entry it has no power to read. Kept beside the rule it serves, below `main`, because
# the two logs quote this file by line: `--citations scripts/check-decisions.py` lists every one, and
# the three anchored rows nearest this end of the file -- the `--log` rationale comment and
# the `layout_problems` call row -- sit ABOVE the seam where a per-file rule can still be
# called from, so the call went below them and this definition goes below all of them. Anything inserted
# above a quoted row moves a published coordinate; this seam moves the two lines of the `__main__`
# guard, which nothing quotes.
HEADING_LIKE = re.compile(r"^(#{1,6})\s+([DH]-\d{4})\b")

# How many unresolved ids the clean-run sentence names before it says `and N more`. Thirty because the
# census this shipped under found twenty-three, and a line that holds the whole set is a line a reader
# can act on without re-deriving the census by hand.
DANGLING_SHOWN = 30


def invisible_heading_problems(text, label):
    """Every heading-shaped line the entry parser cannot read: an entry no other rule in this file sees.

    `parse` finds an entry with `ENTRY`, and every other rule here walks the list `parse` returns --
    the required fields, the `Options` count, the `Chose:`-against-`Options` agreement, checkpoint
    existence, id reuse and ordering, the filed-file layout rule, the cross-log duplicate. A line that
    puts an id where a heading belongs, in a shape `ENTRY` does not accept, is therefore never refused
    as malformed. It is INVISIBLE: the run containing it prints a clean headline one entry short of the
    truth, and the citations that name that id read to whoever follows them as dangling -- which is how
    #393 found the one that was genuinely dangling, and found this shape on the same pass.

    Measured 19 Nov taking the census that issue asked for, on `origin/main` at `25d2904`: the two logs
    answer `382 entries`, and 383 lines in them start with `## ` and an id. The extra line separates its
    id from its title with an em-dash where `ENTRY` wants `·`, `|` or `-`. The entry was not thin --
    `Question`, `Options`, `Chose`, `Why`, `Checkpoint`, `Effect`, `Confidence`, `Revisit`, `Scope` and
    `Status` are all in it and its checkpoint resolves -- and `make verify` had been green over it for
    two days with none of the rules above ever handed its bytes. Making it visible was one character of
    punctuation plus the bullets its fields were already using, and it then refused twice on its own
    field shapes before it passed: which is what a record owes, and exactly what invisibility withheld.

    The boundary, measured on the same tree: this reads a line that BEGINS with one-to-six `#`, a space
    and an id, so a counterexample printed at the start of a fenced line would refuse here too. Neither
    log holds one -- every non-`##` heading level shape in both logs counts zero, and the fenced copies
    of log rows in this repository are in `docs/factory.md`, which is not a log. Quote a malformed shape
    inside a sentence rather than as a line of its own; `self_test` already refuses to print an example
    heading `ENTRY` cannot read, which is the same obligation from the writing side.
    """
    problems = []
    for n, line in enumerate(text.splitlines(), 1):
        shaped = HEADING_LIKE.match(line)
        if shaped and not ENTRY.match(line):
            problems.append(
                "%s:%d: %s is a heading-shaped line the entry parser cannot read, so this gate counts "
                "no entry there and runs no rule on the record under it -- not its fields, not its "
                "checkpoint, not id reuse or ordering -- while `--citations` and a reader both see an id "
                "that resolves to nothing. `ENTRY` wants the id, then one of these separators, then a "
                "title: `## D-0001 · the question that was asked`. Expect the newly visible entry to "
                "refuse on its own fields once it parses; that refusal is the point of this one"
                % (label, n, shaped.group(2)))
    return problems


def scan_id_mentions(pairs, known):
    """Every `D-NNNN`/`H-NNNN` in the logs' text that names no entry in `known`, with where it sits.

    A heading DEFINES an id; a mention anywhere else CITES it. `HEADING_LIKE` is the test for the first,
    so a heading line never reports itself, and everything else carrying an id token is a promise that a
    reader can arrive at a record -- which is what `DECISIONS.md`'s preamble says ids exist for ("Ids are
    shared across the repository rather than per branch, because `git log --grep 'Decision: D-0011'` has
    to name exactly one commit"). A mention whose id is in `known` is that promise kept and is dropped;
    one whose id is in no log is what #393 was filed about.

    Two exemptions, both the issue's own:

    * a token the previous character introduces with `#` -- `backup-tip:factory/DECISIONS.md#H-0122` --
      names the ref it lives on, so the reader has an address even though the log does not hold it;
    * a file this run could not read is not a file with no citations. `read()` reports the failure and
      the caller refuses on it; a census that read it as empty would be a count of nothing.

    Deliberately NOT exempt: an id in a `Checkpoint:` or `Effect:` field. Those are prose a reader
    follows, and `Decision: H-NNNN` in a commit message is the same promise one layer out.
    """
    mentions = []
    for label, text in pairs:
        if text is None:
            continue
        for n, line in enumerate(text.splitlines(), 1):
            if HEADING_LIKE.match(line):
                continue
            for m in ID_TOKEN.finditer(line):
                if m.group(0) in known:
                    continue
                if m.start() and line[m.start() - 1] == "#":
                    continue
                mentions.append({"id": m.group(0), "label": label, "line": n,
                                 "text": line.strip()})
    return mentions


def dangling_line(mentions):
    """The clean-run sentence about id citations that resolve to nothing, or None when there are none.

    Report-only, and the sentence says so: #393 asks for this class "as a report-only list first and say
    how many rows it finds", and its own census is the reason. Measured 19 Nov on the tree this entry was
    written against: the count this function prints is not single figures, so a refusal turned every
    branch red on the day it shipped -- including the branch whose job is to land the check -- which is
    the opposite of what a ratchet is for. `POINTER_DEBT` was raised the same way for `path:line` rows
    and its comment still carries the number it opened at.

    The ids are printed, not just counted, because the set is small enough to act on and a count without
    the names sends a reader to re-derive the census with a script of their own, which is how #393 had to
    argue its case. Capped, because the line is one line: the cap is stated rather than silent, so a
    reader who sees `…` knows there are more and how many more.
    """
    if not mentions:
        return None
    ids = sorted({m["id"] for m in mentions})
    shown = ", ".join(ids[:DANGLING_SHOWN])
    if len(ids) > DANGLING_SHOWN:
        shown += " and %d more" % (len(ids) - DANGLING_SHOWN)
    return ("check-decisions: %d id citation(s) in the two logs name %d id(s) no entry in either log "
            "carries (%s) -- a reader sent to grep for them lands in a hole. Reported, not refused: "
            "#393 asks for the class as a report first, because these rows predate the rule and a gate "
            "that refuses every branch on the day it ships cannot be the branch that fixes them"
            % (len(mentions), len(ids), shown))


# The alphabet of a token, for the mid-quotation test in `anchor_cut` below. Word characters, and the
# four marks that live INSIDE the shell words and `${VAR:-default}` substitutions these logs quote --
# the class #495 was filed on is exactly a substitution whose value was deleted, leaving the quote
# standing inside the word it used to name.
#
# `.` and `/` are deliberately absent, and both absences are measured rather than inherited. A `.`
# rule is refused on this tree's own corpus: 575 anchors are accepted today and 19 of them stop
# immediately before a period -- the row ends `%s:%s` "…both of them the one file"` and the next row
# opens `. Measured on that same fi` -- faithful quotations of a whole sentence that simply do not
# want the sentence's own period inside the quotation marks. A rule for them is a different rule,
# about sentence boundaries, and it has to cost those 19 before it lands. A `/` rule costs nothing
# today (0 of the 575 anchors stop before a slash) and would refuse, the day one is written, a
# quotation like the `add/delete` boundary `H-0114` quotes -- a `-` between two words is a word
# character here and a path separator there, and no rule reads the difference. So the class is token
# runs, not arbitrary text runs, and a cut inside a dotted name or a path stays outside it. Widening
# this set is a decision with a measured price, `--self-test`'s `anchor-cut-*` legs are the seam the
# price gets paid at, and the price is re-measurable with the tool that priced it: run `classify()`
# over the union and count the accepted anchors whose match is followed by a candidate character.
ANCHOR_TOKEN_RUN = frozenset("abcdefghijklmnopqrstuvwxyz"
                             "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
                             "0123456789_-$@{}")


def anchor_cut(want, region):
    """How a quotation the containment test accepts stops inside a token of the cited text.

    Returns `(where, dropped)`, or None when the quote is a whole token: `where` is `end` or `start`,
    and `dropped` is the run of token characters the quotation left on the row -- the text a reader
    has to add to make the quote the whole word the row carries. None means every place the fragment
    matches the cited text begins and ends on a boundary the row agrees with, and that is what makes
    the quote a coordinate: change the word the quote stops in front of and the citation goes red
    instead of staying green over a line that no longer says it (#495, where `431b24b` did exactly
    that to five rows of the log this gate reads).

    `region` is the same `norm()`ed text the accept test matched against -- one line, or a range joined
    with `" ".join` -- so the two rules read one string and cannot disagree about where the fragment
    sits. A row's own edges count as boundaries: `norm()` leaves a match at either end facing a space
    or nothing, never a token character.

    Conservative on the same ground #440 states it: a fragment that matches the cited text more than
    once is refused only when EVERY match is cut. One whole match means the row does carry the quote
    as a complete word, and the lengthening that would fix the remaining match is advice this check
    cannot give -- so the row passes, and the sharper rule about what a fragment answers stays #440's.

    The leftover run is walked to its end rather than reported one character deep because the return
    value becomes the message: `dropped` names what the writer has to add, and a five-character
    leftover quoted as one character is a repair that leaves the pointer cut in a new place.
    """
    if not want:
        return None
    head, tail = want[0] in ANCHOR_TOKEN_RUN, want[-1] in ANCHOR_TOKEN_RUN
    if not head and not tail:
        return None
    cuts, i = [], region.find(want)
    while i != -1:
        after, before = region[i + len(want):i + len(want) + 1], region[i - 1:i]
        if tail and after in ANCHOR_TOKEN_RUN:
            run, j = after, i + len(want) + 1
            while j < len(region) and region[j] in ANCHOR_TOKEN_RUN:
                run, j = run + region[j], j + 1
            cuts.append(("end", run))
        elif head and before in ANCHOR_TOKEN_RUN:
            run, j = before, i - 2
            while j >= 0 and region[j] in ANCHOR_TOKEN_RUN:
                run, j = region[j] + run, j - 1
            cuts.append(("start", run))
        else:
            return None                     # one whole match: the conservative rule, above
        i = region.find(want, i + 1)
    return cuts[0] if cuts else None


if __name__ == "__main__":
    sys.exit(main(sys.argv))
