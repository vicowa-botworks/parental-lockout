#!/usr/bin/env python3
"""Attack ``pages.py --self-test``: make it print a green verdict with a refusal contract broken.

``--self-test`` is the only gate that can see ``testing/capture/*.py`` at all. The mutants target
``.go`` and ``.tmpl``, nothing imports ``pages.py``, and shellcheck does not read Python -- which is
how a refusal helper that raised from inside its own caller's ``except`` handler shipped past every
check this repository has, stopping the capture loop after 1 page of 5 while still exiting 1. So 78
rules stand between the capture drivers and that history, and every one of those rules is itself
code that can be hollowed. #297 is the claim that nothing had tried: measured 11 Nov on
``origin/main`` at ``66c9162``, nine attacks were run by hand against the refusal contract and
**three printed** ``capture self-test: 76 refusal rules hold`` with the symptom live. ``D-0057``
gave the three clauses and re-ran the nine, and its own ``What lost:`` named what was still missing:
*the attacks themselves are still prose in this log rather than a runner in the tree*. This file is
the runner. The three that were green are rows below, expected to be caught, so they cannot reopen
without a red gate.

It is the sibling of ``scripts/attack-self-test.py``, which attacks ``scripts/mutate.py
--self-test``, and it inherits that file's whole hard-won shape rather than rediscovering it: copy
the module *and its siblings* into a box (a sandbox missing one import reports every earlier "catch"
as a catch when it was a ``ModuleNotFoundError``), assert every anchor matches exactly once, keep
``CAUGHT``/``GREEN``/``HOLE``/``INVALID`` distinct, pin the row count, refuse a run in which nothing
came back unseen, and keep a ``DOCUMENTED_HOLES`` list that fails the run both when a new hole
appears and when a listed hole stops being one.

What a sandbox has to hold, measured rather than guessed. ``testing/capture/*.py`` alone is not a
baseline: the census rule reads ``cdp.HERE/../../internal/web/templates``, so a box holding only the
capture tree fails one rule on ``No such file or directory: '<box>/internal/web/templates'`` and the
suite would then be measuring 78 rules against a baseline the guard could never meet -- every row a
false catch. So the box is ``testing/capture/*.py`` **plus** ``internal/web/templates/*.tmpl`` (72 KB,
no git, no browser, no fixture), and the baseline is re-run before every row would be trusted.

The child is run with ``-B``. Not for speed: bytecode written next to the copied modules would be
picked up by the box-cleanliness check below, which is the assertion that the summary line's promise
-- "nothing written outside a temp box" -- means what it says.

What is NOT attacked, and why that is a boundary rather than an oversight: ``cdp.discard_stage``, which
no rule reads any more (#220 made a refusal keep what it measured, and the rules read the drawer back).
The two single-frame drivers have no self-test of their own, but they have been *driven* by ``pages.py
--self-test`` since #296 -- ``single_shot`` and its two siblings call the driver's real ``main()`` with
the browser faked and the keep machinery real -- so a row that patches ``lock.py`` or ``testlock.py``
is now a row about code this guard can see, and #304 adds five. The version of this paragraph written
on 11 Nov said the opposite, and pre-dates those rules. ``manifest.py --self-test`` (44 rules) has the
same hole #297 describes and gets its own row set.

Cost: about four seconds, against 0.08 s for the self-test it attacks and ~3m9s for ``make verify``,
which runs this beside the thing it guards. A suite that starts no compiler and no browser has no
excuse for living in a job somebody can skip (#124).
"""

import hashlib
import os
import os.path as op
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = op.dirname(HERE)
CAPTURE = op.join(ROOT, "testing", "capture")
TEMPLATES = op.join(ROOT, "internal", "web", "templates")
PYTHON = sys.executable or "python3"

# The two sentences `self_test_verdict` ends with, and the third it ends with when a `return`
# truncated the run. `green` is the only one that means "the guard believes 78 rules hold"; a run
# that exits 0 without printing it has reported nothing, which is its own state (`silent`) and not a
# variant of green -- the whole content of #118.
GREEN_SUMMARY = re.compile(r"^capture self-test: \d+ refusal rules hold\b")
FAILED_SUMMARY = re.compile(r"^capture self-test: \d+ of \d+ rules failed$")
RAN_SUMMARY = re.compile(r"^capture self-test: \d+ rules ran, want \d+")

CAUGHT, HOLE, GREEN = "caught", "hole", "green"

# ---------------------------------------------------------------------------------------------
# The floor: every attack the current guard does not see, with what would close it. A row names the
# class it belongs to; several rows reach one class from different lines, and the class is the thing
# a fix closes. A NEW hole fails this run, and a listed hole that has stopped being a hole fails it
# too, so the list cannot rot in either direction.
#
# The classes below are five: two are the floor every suite of this shape has, and three are
# findings this suite made the first time it ran (11 Nov) -- each one a claim about the guard, filed
# as an issue so the queue can see it, and kept here as a row so the claim is re-measured on every
# push instead of remembered:
#
#   1. A counted rule cannot police its own call site -- the class #125 tracks from the other end.
#      The canary pair proves `rule()` is not hollow; nothing proves a *rule* is not.
#   2. A check cannot police its own deletion -- the same floor `attack-self-test.py` documents for
#      `mutate.py`'s `if bad:`. It shows up twice here, at the verdict and at the canary.
#   3-5. What the 78 rules do not look at: a crash that never happens, and the bytes of the frame
#      a refusal keeps.
# ---------------------------------------------------------------------------------------------
DOCUMENTED_HOLES = {
    "a hollowed rule is invisible to the count":
        "Every rule is registered by name and counted by the pin, so replacing a rule body with "
        "`return` leaves the count at 78 and the transcript green. Measured here, not assumed: all "
        "four refusal rules this suite empties came back unseen, so no rule's *content* is covered "
        "by a sibling of another -- the count pins how many rules ran and says nothing about what "
        "any of them checked. That is #297's title read literally. What closes it is the Go "
        "battery's design applied to Python: one probe per rule that breaks the thing that rule "
        "names and insists THAT rule goes red, so a hollowed rule retires a row instead of passing "
        "a gate.",
    "the canary pair's own comparison is unguarded":
        "`if counts != [1, 0] or len(probe_bad) != 1:` is the only sentence proving `rule()` is not "
        "hollow, and it sits one level above the pair it reads: switched off, the canaries are "
        "still registered, the counts are still computed, and nothing compares them. The same floor "
        "as the verdict's `if bad:` -- a check cannot police its own deletion -- so it is listed "
        "rather than fixed. Its presence is not unpinned, though: the four rows that empty the rule "
        "helper in four different ways are all caught, and they are caught *because* this "
        "comparison is live.",
    "the verdict's final decision is unguarded":
        "`if bad:` is where every problem becomes an exit code, the `len(ran) != EXPECTED_RULES` "
        "comparison is the reach check, and its `return 1` is the last link. Each is one edit above "
        "anything a rule compares: a rule that policed its own count check would have to hold the "
        "number the check exists to hold. This is the floor, so it is listed rather than fixed.",
    "a rule that throws is recorded as a passing rule":
        "`rule()`'s `except` arm is the only place a crashing rule becomes a problem. Emptying its one "
        "`problems.append` is invisible on a clean tree, and the crash never happens until the code "
        "under a rule breaks -- which is exactly when the silence matters. Measured with a real "
        "crash driven through both versions of the arm (`cdp.refused_dir` raising "
        "`RuntimeError('boom')`): arm intact, `capture self-test: 17 of 76 rules failed`, exit 1, "
        "each crashed rule named; arm emptied, the same crash prints `76 refusal rules hold`, exit 0, "
        "and `boom` appears in the transcript zero times. Those transcripts quote the count the suite "
        "had when the row was written -- the number is the suite's own, it moves with every rule any "
        "branch adds, and it reads 78 today. What closes it is a third canary: one "
        "registered like the other two, deliberately raising, asserting its own "
        "`f\"{why}: {type(exc).__name__}\"` sentence arrives. Not done here, because that is a change "
        "to the guard and #297 asked for the runner; filed as its own issue.",
    "the kept frame's bytes are never compared to the bytes the browser returned":
        "`_FakeSession.screenshot()` hands `cdp.capture_refused` a payload and the rules read the "
        "directory back -- names, counts, the note's sentences -- and nothing reads the file's "
        "contents. Writing an empty file at the announced path therefore keeps the transcript and "
        "the listing in perfect agreement about a frame with no picture in it. What closes it is "
        "one clause in `a_refused_run_says_where_its_frames_are`: the kept file's bytes equal the "
        "fake's payload. Filed as its own issue.",
}

HOLE_CLASS = {
    "the canary comparison is switched off": "the canary pair's own comparison is unguarded",
    "one refusal rule's body is emptied -- where its frames are":
        "a hollowed rule is invisible to the count",
    "one refusal rule's body is emptied -- frames kept apart per fixture":
        "a hollowed rule is invisible to the count",
    "one refusal rule's body is emptied -- the moved-out-of-the-queue claim":
        "a hollowed rule is invisible to the count",
    "one refusal rule's body is emptied -- one disposal per refusal":
        "a hollowed rule is invisible to the count",
    "the verdict's problem list is switched off":
        "the verdict's final decision is unguarded",
    "the reach check is switched off":
        "the verdict's final decision is unguarded",
    "the reach check runs and returns success":
        "the verdict's final decision is unguarded",
    "a rule that throws is silenced in the rule helper":
        "a rule that throws is recorded as a passing rule",
    "the kept frame is written as an empty file":
        "the kept frame's bytes are never compared to the bytes the browser returned",
}

# ---------------------------------------------------------------------------------------------
# The rows: (expect, target file, what it does, anchor, replacement).
#
# Anchors are copied from the file and matched exactly once -- a patch matching nothing is a no-op
# and a patch matching twice edits two places while reporting one, and in both cases the tally keeps
# counting while the guard stays where it was. They sit on statements, never on an aligned composite
# literal (a reformat retires the match silently -- the constraint list's own lesson), and never on
# `EXPECTED_RULES`, which a rebase moves legitimately.
#
# Rows 1-9 are the nine #297 was filed with, re-run. Rows 7-9 are the three that printed the green
# summary on `66c9162`; they are expected to be caught here, which is what makes them regression
# rows rather than notes.
# ---------------------------------------------------------------------------------------------
ROWS = [
    # ---- the nine from #297 / D-0057, each on its own copy of the tree (the gated tree is never
    #      itself the mutant -- #1807), each anchor asserted before its tally is believed ----
    (CAUGHT, "cdp.py", "keep_refused is hollowed to `return []`, so a refusal destroys what it measured",
     "def keep_refused(keep, note=None, out=None):",
     "def keep_refused(keep, note=None, out=None):\n    return []"),
    # Row 2's anchor retired once, on 11 Nov: #296 (D-0060) moved the refusal note onto the shared
    # `cdp.refusal_note` and the call rewrapped, and the run answered `INVALID … 0 anchor matches` with
    # exit 1 -- this suite refuses a retired anchor instead of counting it (unlike the mutation battery,
    # where a 0-match mutant is reported NOT APPLIED and the battery stays green, #1048). It is re-pointed
    # onto a one-line statement so reformatting the note cannot reach the anchor -- the reason `35f2a41`
    # gives for W214, and what retired the wrapped form on #296. It is not immunity to an edit of the
    # anchored line: a kwarg on `keep_refused` or a rename of `note` retires it too, loudly (#305).
    (CAUGHT, "pages.py", "the refusal branch goes back to `discard_stage()`",
     '            kept = cdp.keep_refused(keep, note=note, out=out)',
     '            cdp.discard_stage()\n            kept = None'),
    (CAUGHT, "cdp.py", "`refused_dir` stops naming the directory for the fixture, so two fixtures share one drawer",
     '    return op.join(RUNS, f"{mode}-refused")',
     '    return op.join(RUNS, "refused")'),
    (CAUGHT, "cdp.py", "the once-only clear of a refused directory becomes a clear on every call",
     "    if _REFUSED != real_keep:", "    if True:"),
    (CAUGHT, "cdp.py", "the keep-guard is switched off, so a keep directory outside `runs/` is accepted",
     "    if not inside or overlaps:", "    if False:"),
    (CAUGHT, "pages.py", "one refusal rule is unregistered, and the count is what notices",
     '    rule("a refused run says where its frames are", a_refused_run_says_where_its_frames_are)',
     "    pass  # the rule was unregistered"),
    (CAUGHT, "pages.py", "main()'s `N frame(s) this run measured are in <dir>` announcement is deleted",
     '                print(f"  {len(left)} frame(s) this run measured are in {keep}, with "\n'
     '                      f"{cdp.REFUSED_NOTE} beside them"\n'
     """                      + (f": {', '.join(left)}" if left else ""), file=sys.stderr, flush=True)""",
     ""),
    (CAUGHT, "pages.py", "the announcement says `0 frame(s)` while two frames sit in the kept directory",
     '                print(f"  {len(left)} frame(s) this run measured are in {keep}, with "',
     '                print(f"  {0} frame(s) this run measured are in {keep}, with "'),
    (CAUGHT, "cdp.py", "the pointer beside the refused frame is deleted from `capture_refused`",
     '    print(f"  what the page actually renders is at {path} -- this run refused to publish it, so "\n'
     '          "it is not evidence; read it with REFUSED.txt beside it", file=sys.stderr)',
     ""),

    # ---- the rest of the same contract, row by row ----
    (CAUGHT, "pages.py", "the `Nothing was written to <out>` sentence is deleted",
     '            print(f"Nothing was written to {out}.", file=sys.stderr, flush=True)',
     ""),
    (CAUGHT, "pages.py", "the retracting sentence about the FITS paths is deleted",
     '                print("  the paths the FITS lines above print are files this run did NOT write. "\n'
     '                      f"They are in {keep}, and they are not evidence: no manifest names them and "\n'
     '                      "nothing certifies them, which is where a refused page belongs.",\n'
     "                      file=sys.stderr, flush=True)",
     ""),
    (CAUGHT, "pages.py", "the announcement sends the reader to --out instead of the directory holding the frames",
     '                print(f"  {len(left)} frame(s) this run measured are in {keep}, with "',
     '                print(f"  {len(left)} frame(s) this run measured are in {out}, with "'),
    (CAUGHT, "pages.py", "the announcement keeps the count and drops the list of frames it kept",
     """                      + (f": {', '.join(left)}" if left else ""), file=sys.stderr, flush=True)""",
     '                      + "", file=sys.stderr, flush=True)'),
    (CAUGHT, "cdp.py", "the kept frame is announced without the words `not evidence`",
     '          "it is not evidence; read it with REFUSED.txt beside it", file=sys.stderr)',
     '          "read it with REFUSED.txt beside it", file=sys.stderr)'),
    (CAUGHT, "cdp.py", "`capture_refused` stops catching SystemExit, so a dead browser replaces the refusal it reports",
     "    except (Exception, SystemExit) as exc:  # noqa: BLE001 -- a diagnostic never outranks the refusal",
     "    except Exception as exc:  # noqa: BLE001"),
    (CAUGHT, "cdp.py", "the keep-guard's `--out` overlap arm is switched off",
     "    overlaps = real_out is not None and (real_keep == real_out",
     "    overlaps = False and (real_keep == real_out"),
    (CAUGHT, "cdp.py", "a kept frame stays in the publish queue because `_PENDING` is not emptied",
     "        moved.append(dest)\n    _PENDING.clear()",
     "        moved.append(dest)"),
    (CAUGHT, "cdp.py", "the stale clear empties nothing, so yesterday's frame is quoted as this run's",
     "        for name in stale:\n            os.remove(op.join(real_keep, name))",
     "        for name in stale:\n            pass"),
    (CAUGHT, "cdp.py", "the frame is hard-linked rather than moved, so the staged path still names it",
     "        os.replace(op.join(stage, name), dest)",
     "        os.link(op.join(stage, name), dest)"),
    (CAUGHT, "cdp.py", "REFUSED.txt is never written beside the kept frames",
     '    if note:\n'
     '        with open(op.join(real_keep, REFUSED_NOTE), "a") as fh:\n'
     "            fh.write(note)",
     "    if False:\n"
     '        with open(op.join(real_keep, REFUSED_NOTE), "a") as fh:\n'
     "            fh.write(note)"),
    (CAUGHT, "pages.py", "the refusal branch is switched off, so a refused run publishes and prints a fit count",
     "        if any(pages or untested for _, pages, untested in refused):", "        if False:"),
    (CAUGHT, "pages.py", "the success path stops publishing",
     "    cap.session.publish()", "    pass  # publish is skipped"),

    # ---- #304: one disposal per refusal, whichever handler reaches it, on both single-frame drivers.
    #      These are the first rows in this file that patch a driver rather than `cdp.py`/`pages.py`,
    #      which is possible because #296 put the drivers under `--self-test`'s nose; the rule they
    #      lean on is the one #304 adds, and each of the three is caught by THAT rule alone ----
    (CAUGHT, "cdp.py", "the one-disposal flag is never set, so a report that dies re-refuses the page",
     "        self.disposed = True", "        self.disposed = False"),
    # The second half of the guard, and the half the ticket did not ask for: the suppression sentence
    # has to say what is on disk. Forcing the note-exists branch makes the driver announce a
    # `REFUSED.txt` it never wrote -- the invention #304 is about, pointed the other way -- and only
    # the rule's `pre` cell can see it. Verified by attribution on 11 Nov: this row makes exactly one
    # rule fail, `capture self-test: 1 of 78 rules failed`, naming that rule and no other.
    (CAUGHT, "cdp.py", "the suppression sentence claims a note without looking for one",
     "            if op.isfile(note):", "            if True:"),
    # The other sentence the flag cannot tell you: an empty drawer. `capture_refused` reports a failed
    # photograph and returns None, so a refusal can leave no frame with the browser up, and the sentence
    # for that used to name the fixture either way -- measured 11 Nov, over a live browser. Forcing the
    # fixture branch on every empty drawer is the claim that retires.
    (CAUGHT, "cdp.py", "every empty drawer is explained as the fixture, browser or not",
     "                    if cap is None else", "                    if True else"),
    (CAUGHT, "lock.py", "the outer handler refuses directly instead of through the guard -- lock",
     "        disposal.refuse(None, cdp.refusal_reason(exc))",
     "        cdp.refuse_single_shot(None, FIXTURE, out, name, frame, cdp.refusal_reason(exc))"),
    (CAUGHT, "testlock.py", "the outer handler refuses directly instead of through the guard -- testlock",
     "        disposal.refuse(None, cdp.refusal_reason(exc))",
     "        cdp.refuse_single_shot(None, FIXTURE, out, name, frame, cdp.refusal_reason(exc))"),

    # ---- the plumbing that makes 78 rules mean 78 rules ----
    (CAUGHT, "pages.py", "the rule helper records zero problems for every rule",
     "        (ran_to if ran_to is not None else ran).append((why, len(problems)))",
     "        (ran_to if ran_to is not None else ran).append((why, 0))"),
    (CAUGHT, "pages.py", "the rule helper drops the problems it was handed",
     "        (bad_to if bad_to is not None else bad).extend(problems)",
     "        pass  # the problems are dropped"),
    (CAUGHT, "pages.py", "the rule helper never calls the rule body it is given",
     "            body(problems)", "            pass  # the body is never called"),
    (CAUGHT, "pages.py", "the canary pair is deleted, leaving nothing that proves the helper is not hollow",
     '    rule("canary", lambda problems: problems.append("a problem"), probe_ran, probe_bad)\n'
     '    rule("canary", lambda problems: None, probe_ran, probe_bad)',
     "    pass  # the canaries were deleted"),
    (HOLE, "pages.py", "the canary comparison is switched off",
     "    if counts != [1, 0] or len(probe_bad) != 1:", "    if False:"),
    (HOLE, "pages.py", "a rule that throws is silenced in the rule helper",
     '            problems.append(f"{why}: {type(exc).__name__}: {exc}")',
     "            pass  # a crashing rule says nothing"),
    (HOLE, "pages.py", "the verdict's problem list is switched off",
     "    if bad:\n        for p in bad:", "    if False:\n        for p in bad:"),
    (HOLE, "pages.py", "the reach check is switched off",
     "    if len(ran) != EXPECTED_RULES:", "    if False:"),
    (HOLE, "pages.py", "the reach check runs and returns success",
     '              "(a `return` inside self_test() skips every rule below it)", file=sys.stderr)\n'
     "        return 1",
     '              "(a `return` inside self_test() skips every rule below it)", file=sys.stderr)\n'
     "        return 0"),

    # ---- the class #297's title is about: a hollowed rule keeps the count at 78 ----
    (HOLE, "pages.py", "one refusal rule's body is emptied -- where its frames are",
     "    def a_refused_run_says_where_its_frames_are(problems):",
     "    def a_refused_run_says_where_its_frames_are(problems):\n        return"),
    (HOLE, "pages.py", "one refusal rule's body is emptied -- frames kept apart per fixture",
     "    def each_fixture_keeps_its_refused_frames_apart(problems):",
     "    def each_fixture_keeps_its_refused_frames_apart(problems):\n        return"),
    (HOLE, "pages.py", "one refusal rule's body is emptied -- the moved-out-of-the-queue claim",
     "    def a_kept_frame_is_moved_out_of_the_publish_queue(problems):",
     "    def a_kept_frame_is_moved_out_of_the_publish_queue(problems):\n        return"),
    # The rule #304 adds gets the same row as its siblings, because it inherits their floor: the count
    # went 77 -> 78 and a body emptied to `return` keeps it at 78. Listing it is the honest price of
    # adding a rule -- the class text claims every rule this suite empties came back unseen, and an
    # unlisted hollowing would make that sentence false in the one direction nothing else checks.
    (HOLE, "pages.py", "one refusal rule's body is emptied -- one disposal per refusal",
     "    def a_refusal_whose_report_raises_is_disposed_of_once(problems):",
     "    def a_refusal_whose_report_raises_is_disposed_of_once(problems):\n        return"),

    # ---- the bytes ----
    (HOLE, "cdp.py", "the kept frame is written as an empty file",
     '        with open(path, "wb") as fh:\n            fh.write(data)',
     '        with open(path, "wb") as fh:\n            fh.write(b"")'),

    # ---- honest edits: these must stay green, or the guard is matching text it should ignore, and
    #      a guard that is a tax gets switched off ----
    (GREEN, "pages.py", "a comment is added above the rule helper",
     "    def rule(why, body, ran_to=None, bad_to=None):",
     "    # The one place a rule is both counted and reported. The canary pair below proves it.\n"
     "    def rule(why, body, ran_to=None, bad_to=None):"),
    (GREEN, "pages.py", "the canary comparison's operands are swapped",
     "    if counts != [1, 0] or len(probe_bad) != 1:",
     "    if [1, 0] != counts or 1 != len(probe_bad):"),
    (GREEN, "pages.py", "a refusal rule's problem sentence is reworded",
     '            problems.append("the summary stopped saying that --out got nothing")',
     '            problems.append("the summary no longer says --out got nothing")'),
    (GREEN, "cdp.py", "the keep-guard's operands are swapped",
     "    if not inside or overlaps:", "    if overlaps or not inside:"),
    (GREEN, "cdp.py", "`keep_refused`'s summary line is reworded",
     '    """Move what this run has staged into `keep`; write `note` beside it; touch nothing at `--out`.',
     '    """Move this run\'s staged bytes into `keep`, write `note` beside them, and leave `--out` alone.'),
    (GREEN, "cdp.py", "a comment is added inside `capture_refused`'s success arm",
     '    print(f"  what the page actually renders is at {path} -- this run refused to publish it, so "',
     "    # The pointer an operator follows, and the disclaimer that keeps it out of the evidence set.\n"
     '    print(f"  what the page actually renders is at {path} -- this run refused to publish it, so "'),
]

# The suite's own count pin, on the pattern of `mutate.py`'s three and `attack-self-test.py`'s one.
# A row deleted from ROWS makes the tally smaller and every remaining row still reports its expected
# verdict, so the run prints a confident "40 attacks, 0 problems" over a guard that lost a test.
EXPECTED_ROWS = 47


def shape_problems():
    """Static faults in this file, checked before a single child is started.

    The row list, the class map and the hole list are joined by one string -- the row's ``what`` --
    and every one of those joins has a silent failure mode that leaves the run printing a confident
    verdict. A repeated ``what`` makes two rows answer to one class. A ``HOLE_CLASS`` key nothing
    names is a class that can never be seen again, and the run reports it as "DOCUMENTED HOLE NO
    LONGER SEEN" while pointing at the hole list instead of at the stale mapping. A HOLE row with no
    class falls back to its own name and reads as undocumented. All of these were written by hand
    within one sitting on the day this file was added -- one of them was, in fact, caught by the
    first draft of this function -- and none of them can be found by reading, which is the same
    argument that put a battery where a review comment used to be.
    """
    faults = []
    names = [row[2] for row in ROWS]
    for dup in sorted({n for n in names if names.count(n) > 1}):
        faults.append(f"two rows are both named {dup!r}: the class map keys on that name, so one "
                      f"of them is being described by the other's class")
    named = set(names)
    for stale in sorted(set(HOLE_CLASS) - named):
        faults.append(f"HOLE_CLASS names {stale!r}, which no row is: a class whose row was retired "
                      f"or renamed can never be seen again")
    for target in sorted({row[1] for row in ROWS}):
        if target not in capture_modules():
            faults.append(f"rows patch {target!r}, which is not in testing/capture/ -- the box "
                          f"would run an unpatched module and report the row as its verdict")
    for expect, _target, what, anchor, replacement in ROWS:
        if anchor == replacement:
            faults.append(f"{what!r} replaces the anchor with itself: the patch is a no-op")
        if expect == HOLE and what not in HOLE_CLASS:
            faults.append(f"{what!r} expects a hole and has no HOLE_CLASS entry, so it falls back "
                          f"to its own name and reports as undocumented")
        if expect != HOLE and what in HOLE_CLASS:
            faults.append(f"{what!r} is mapped to a hole class but expects {expect}: the class can "
                          f"never be reached, and its DOCUMENTED_HOLES entry will read as rotting")
    unreachable = sorted(set(DOCUMENTED_HOLES) - set(HOLE_CLASS.values()))
    for name in unreachable:
        faults.append(f"DOCUMENTED_HOLES lists {name!r} and no row maps to it, so nothing can ever "
                      f"re-measure it")
    for cls in sorted(set(HOLE_CLASS.values()) - set(DOCUMENTED_HOLES)):
        faults.append(f"rows are mapped to class {cls!r}, which DOCUMENTED_HOLES does not describe")
    return faults



def capture_modules():
    """Every module in ``testing/capture/``, so a box can never be missing a sibling.

    ``pages.py`` imports ``cdp`` and ``manifest`` off its own directory. Naming the three instead of
    globbing ``*.py`` is how ``attack-self-test.py`` once reported 42 catches that were all a missing
    import, and a box missing ``testlock.py`` would do the same thing here the day one rule imports
    it.
    """
    return sorted(p for p in os.listdir(CAPTURE) if p.endswith(".py"))


def template_files():
    """The ``.tmpl`` files the census rule reads, as paths relative to ``TEMPLATES``.

    The rule at ``pages.py:3326`` walks ``cdp.HERE/../../internal/web/templates`` in both directions:
    the census vocabulary has to appear in the templates and every class in the templates has to be
    in the census. A box without them fails that rule, which is the pre-flight's problem, not a
    catch: a suite measuring a guard against a baseline the guard could never meet reports every row
    as a catch.
    """
    found = []
    for base, _dirs, files in os.walk(TEMPLATES):
        found += [op.relpath(op.join(base, f), TEMPLATES) for f in files if f.endswith(".tmpl")]
    return sorted(found)


def sandbox():
    """A private copy of the capture tree, in the layout the driver expects, plus what it reads.

    The ``testing/capture`` level is not decoration: ``cdp.RUNS`` is ``dirname(__file__)/runs`` and
    the census rule resolves the templates two levels up from ``cdp.HERE``. Parked flat, both of
    those point at the scratch directory's parent, and the run measures a tree that does not exist.
    """
    box = tempfile.mkdtemp(prefix="plock-capture-attacks-")
    dst = op.join(box, "testing", "capture")
    os.makedirs(dst)
    for name in capture_modules():
        shutil.copyfile(op.join(CAPTURE, name), op.join(dst, name))
    tdst = op.join(box, "internal", "web", "templates")
    os.makedirs(tdst)
    for rel in template_files():
        os.makedirs(op.join(tdst, op.dirname(rel)), exist_ok=True)
        shutil.copyfile(op.join(TEMPLATES, rel), op.join(tdst, rel))
    return box


def box_files(box):
    """Every file in the box, relative. The baseline's cleanliness is asserted against this."""
    found = []
    for base, _dirs, files in os.walk(box):
        found += [op.relpath(op.join(base, f), box) for f in files]
    return set(found)


# Per-row ceiling. The average is ~0.1 s, so 60 s is several hundred times the work a row does, and
# the number that matters is the product with the row count: that is the longest `make verify` can
# ever spend here. Several rows edit loop control, so a patched driver that hangs is reachable.
TIMEOUT = 60


def run(box):
    """Run the self-test in `box` and classify what came back. Never raises.

    ``silent`` is its own state: a self-test that exits 0 without printing its summary has not
    reported that anything holds, it has reported nothing.
    """
    try:
        proc = subprocess.run([PYTHON, "-B", op.join(box, "testing", "capture", "pages.py"),
                               "--self-test"], capture_output=True, text=True, encoding="utf-8",
                              cwd=box, timeout=TIMEOUT)
    except subprocess.TimeoutExpired:
        # "Did not finish" and "did not stop" are different diagnoses, and this branch is also
        # reached by the baseline pre-flight, which has no patch to name. The caller prints the row.
        return "invalid", "", (f"the self-test did not finish in {TIMEOUT}s -- a hang, not a "
                               f"broken patch")
    except (OSError, subprocess.SubprocessError) as exc:
        return "invalid", "", f"the sandboxed self-test could not be run: {exc}"
    out, err = proc.stdout.strip(), proc.stderr
    last_err = [ln for ln in err.splitlines() if ln.strip()]
    last_err = last_err[-1] if last_err else ""
    if proc.returncode not in (0, 1) or "Traceback" in err or "SyntaxError" in err:
        # A patched module that does not compile proves nothing. The first version of the sibling
        # suite scored one of these as a catch, and the crash check could not see it because the run
        # raised before printing anything.
        return "invalid", out, err
    if proc.returncode == 0 and out and GREEN_SUMMARY.match(out.splitlines()[-1]):
        return "green", out, err
    if proc.returncode == 0:
        return "silent", out, err
    if proc.returncode == 1 and (FAILED_SUMMARY.match(last_err) or RAN_SUMMARY.match(last_err)):
        return "caught", out, err
    # Exit 1 that did not name itself a refusal: the harness's problem, not the guard's.
    return "invalid", out, err


def tree_state():
    """Digests of the files this suite reads. A run that changed one has broken its own rule."""
    state = {}
    for name in capture_modules():
        path = op.join(CAPTURE, name)
        state[name] = hashlib.sha256(open(path, "rb").read()).hexdigest()
    for rel in template_files():
        state[rel] = hashlib.sha256(open(op.join(TEMPLATES, rel), "rb").read()).hexdigest()
    return state


def verify_recipe(text):
    """The `verify:` target's recipe lines, and only those.

    Searched in the recipe rather than in the whole file on purpose: a mention in the prose above it
    is not a step, and the sentence explaining why the step exists must not be able to satisfy the
    assertion the step is supposed to satisfy.
    """
    lines = text.splitlines()
    try:
        start = next(i for i, line in enumerate(lines) if line.rstrip() == "verify:")
    except StopIteration:
        return ""
    out = []
    for line in lines[start + 1:]:
        if line and not line.startswith("\t"):
            break
        out.append(line)
    return "\n".join(out)


def wiring_problems():
    """Are both of this suite's `make verify` steps still in the Makefile this file ships in?

    The cheapest way to pass a check is to stop calling it, and nothing under `scripts/mutations/`
    names a file under `scripts/`, so no mutant can see a deleted step. `manifest.py` answered that
    in #144 by reading the Makefile it ships in, and does the same thing here -- with one addition
    this suite needs and that file does not: it is called from TWO steps, and they witness each
    other. The gate step runs it with no argument; the empty-filter loop names it in its `for runner`
    list. Each of the two assertions below is therefore reachable from the other, so deleting either
    step makes the surviving one refuse.
    """
    try:
        with open(op.join(ROOT, "Makefile"), encoding="utf-8") as fh:
            text = fh.read()
    except OSError as exc:
        return [f"the Makefile could not be read ({exc}), so nothing here can promise `make verify` "
                f"still runs this suite"]
    recipe = verify_recipe(text)
    problems = []
    if not re.search(r"python3 scripts/attack-capture-self-test\.py\b", recipe):
        problems.append("`make verify` no longer runs this suite. Forty-seven attacks nobody invokes "
                        "are a list of intentions, not a gate; if the step was deleted because this "
                        "file's verdict became inconvenient, the finding is in the verdict")
    if not re.search(r"for runner in [^\n]*attack-capture-self-test\.py", recipe):
        problems.append("`make verify` no longer passes this suite through the empty-filter loop, so "
                        "a filter matching nothing could exit 0 and the run would report a "
                        "confident verdict over nothing attempted. This loop is also the second "
                        "witness of the gate step above: delete it and that step goes red")
    return problems


def main():
    # `None` means no argument was given; `""` means one was given and it is empty. The sibling's
    # lesson, kept verbatim in shape: collapsing the two makes `""` mean "no filter", so the
    # Makefile step that passes "$FILTER" with FILTER unset runs the whole gate and reports green
    # while proving something else. An empty filter here matches NOTHING.
    only = sys.argv[1] if len(sys.argv) > 1 else None
    selected = only is not None
    faults = shape_problems() + wiring_problems()
    if faults:
        print("HARNESS: this file, or the gate that calls it, is malformed, so no verdict below "
              "would mean anything:")
        for fault in faults:
            print(f"  - {fault}")
        return 2
    originals = {name: open(op.join(CAPTURE, name), encoding="utf-8").read()
                 for name in capture_modules()}
    before = tree_state()

    box = sandbox()
    try:
        copied = box_files(box)
        state, out, err = run(box)
        if state != "green":
            print(f"HARNESS: the unpatched copy does not pass its own self-test ({state}).")
            print(f"  stdout: {out[:600]}")
            print(f"  stderr: {err[:600]}")
            print("Every attack below would be measured against a baseline the guard was never able "
                  "to meet, so none is run. The box holds testing/capture/*.py and "
                  "internal/web/templates/*.tmpl; if the self-test has started reading anything "
                  "else, the box has to hold that too.")
            return 2
        written = box_files(box) - copied
        if written:
            # The summary line's own promise -- "nothing written outside a temp box" -- checked on
            # the disk rather than believed. The box IS the temp box's neighbour: a driver that
            # writes into the tree it was copied into writes into the repository when it is not
            # copied, which is #222's hazard aimed at the gate instead of at the evidence.
            print("HARNESS: the self-test wrote into the tree it was copied into, so its summary "
                  f"line is false about its own writes: {sorted(written)[:6]}")
            return 2

        counts = {"caught": 0, "green": 0, "hole": 0, "invalid": 0, "wrong": 0}
        problems, seen_holes = [], []
        for expect, target, what, anchor, replacement in ROWS:
            if selected and (not only or only.lower() not in what.lower()):
                continue
            src = originals[target]
            hits = src.count(anchor)
            if hits != 1:
                counts["invalid"] += 1
                problems.append(f"anchor matched {hits} times in {target}, want exactly 1: {what}")
                print(f"INVALID  {what} ({hits} anchor matches in {target})")
                continue
            patched = src.replace(anchor, replacement, 1)
            if patched == src:
                counts["invalid"] += 1
                problems.append(f"the replacement is the text it replaces, so nothing was "
                                f"attacked: {what}")
                print(f"INVALID  {what} (the patch is a no-op)")
                continue
            with open(op.join(box, "testing", "capture", target), "w", encoding="utf-8") as fh:
                fh.write(patched)
            state, out, err = run(box)
            # Put the module back unpatched whether or not the row was interesting: two patched
            # files in one box is two changes, and a row's verdict has to belong to its own patch.
            with open(op.join(box, "testing", "capture", target), "w", encoding="utf-8") as fh:
                fh.write(src)
            if state == "invalid":
                counts["invalid"] += 1
                problems.append(f"the patched module did not answer usefully, so the row proves "
                                f"nothing: {what} -- {(err or out).strip()[:200]}")
                print(f"INVALID  {what}")
                continue
            unseen = state in ("green", "silent")
            if state == "caught" and expect == CAUGHT:
                counts["caught"] += 1
                print(f"CAUGHT   {what}")
            elif unseen and expect == HOLE:
                counts["hole"] += 1
                seen_holes.append(what)
                if HOLE_CLASS.get(what, what) not in DOCUMENTED_HOLES:
                    problems.append(f"undocumented hole: {what!r} -- the guard did not see this "
                                    f"and DOCUMENTED_HOLES does not list its class")
                print(f"HOLE     {what}"
                      + ("" if state == "green" else "   (silent: no verdict printed)"))
            elif state == "green" and expect == GREEN:
                counts["green"] += 1
                print(f"GREEN    {what}")
            elif unseen and expect == CAUGHT:
                counts["wrong"] += 1
                seen_holes.append(what)
                last = out.splitlines()[-1] if out else "<nothing at all>"
                problems.append(f"NEW HOLE {what} -- the self-test printed {last!r} with this live")
                print(f"NEW HOLE {what}")
            elif state == "caught" and expect == GREEN:
                counts["wrong"] += 1
                problems.append(f"OVER-STRICT {what} -- an honest edit turned the guard red, so it "
                                f"is matching text rather than behaviour: {out[:200]!r}")
                print(f"OVER-STRICT {what}")
            elif state == "caught" and expect == HOLE:
                counts["wrong"] += 1
                problems.append(f"HOLE CLOSED {what} -- the guard now refuses this, so retire the "
                                f"row and, if it was the last in its class, the class with it")
                print(f"HOLE CLOSED {what}")
            else:
                counts["wrong"] += 1
                problems.append(f"UNEXPECTED {what} -- expected {expect}, the patched self-test "
                                f"came back {state}: {out[:200]!r}")
                print(f"UNEXPECTED {what} (expected {expect}, got {state})")

        seen_classes = {HOLE_CLASS.get(w, w) for w in seen_holes}
        for name in DOCUMENTED_HOLES if not selected else ():
            if name not in seen_classes:
                problems.append(f"DOCUMENTED HOLE NO LONGER SEEN {name!r} -- either the guard got "
                                f"better, in which case remove the class and its rows, or the "
                                f"attack stopped attacking it")

        if tree_state() != before:
            problems.append("this suite modified the tree it was checking")

        ran = sum(counts.values())
        if selected:
            print(f"\nSELECTED RUN -- rows matching {only!r}, not a gate. The suite-wide pins (row "
                  f"count, hole coverage) are skipped because they describe the whole suite; run "
                  f"with no argument for the verdict CI applies.")
            if not ran:
                problems.append(f"no row matched {only!r}: a run that attempts nothing proves "
                                f"nothing")
        if not selected and len(ROWS) != EXPECTED_ROWS:
            problems.append(f"the suite ran {len(ROWS)} attacks, expected {EXPECTED_ROWS}: a row "
                            f"was deleted, or stopped being attempted. A smaller tally is not a "
                            f"smaller claim")
        if not selected and ran != len(ROWS):
            problems.append(f"{ran} rows were classified and {len(ROWS)} were attempted: a row was "
                            f"skipped, and a skipped row is a row that reports nothing")
        if not selected and not counts["hole"]:
            problems.append("no row came back unseen. Every expectation here is either 'the guard "
                            "refuses' or 'the guard does not see it', so a classifier stuck on "
                            "either answer satisfies one half by breaking the other -- a run with "
                            "no holes at all means the classifier is stuck, which is how the old "
                            "suite reported 42 catches it never made")
        print(f"\nTOTAL  {ran} attacks: {counts['caught']} caught, {counts['green']} honest edits "
              f"still green, {counts['hole']} holes, {counts['invalid']} invalid, "
              f"{counts['wrong']} unexpected")
        if seen_holes:
            print(f"HOLES ({len(seen_classes)} classes, {len(seen_holes)} rows) -- each one is a "
                  "finding about the guard:")
            for name in sorted(seen_classes):
                print(f"  o {name}\n      closes: {DOCUMENTED_HOLES.get(name, 'NOT DOCUMENTED')}")
        if problems:
            print(f"\nPROBLEMS ({len(problems)}):")
            for p in problems:
                print(f"  - {p}")
        print(f"\nVERDICT: {ran} attacks, {len(problems)} problem(s), {len(seen_classes)} hole "
              f"class(es), {len(seen_holes)} hole row(s)")
        return 1 if problems else 0
    finally:
        shutil.rmtree(box, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
