#!/usr/bin/env python3
"""Attack ``pages.py --self-test``: make it print a green verdict with a refusal contract broken.

``--self-test`` is the only gate that can see ``testing/capture/*.py`` at all. The mutants target
``.go`` and ``.tmpl``, nothing imports ``pages.py``, and shellcheck does not read Python -- which is
how a refusal helper that raised from inside its own caller's ``except`` handler shipped past every
check this repository has, stopping the capture loop after 1 page of 5 while still exiting 1. So 81
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
suite would then be measuring 81 rules against a baseline the guard could never meet -- every row a
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
on 11 Nov said the opposite, and pre-dates those rules. #317 is what ``D-0065``'s ``What lost:``
forfeited rather than widen -- "the sweep driver, which has its own three handlers, is not driven by
this rule yet" -- and it is the expensive row: one dead ``print`` at the sweep's first refused page
took every page after it, and both fixture arms, which have a single handler, printed nothing at all
and exited 1. Four rows below attack the four guards that fix installs, and two more empty the rules
that grade the arms #304 could not reach. ``manifest.py --self-test`` (54 rules, the number its own
verdict line prints) has the same hole #297 describes and has NO row set here or in
``attack-self-test.py`` -- corrected 13 Nov, because the sentence this paragraph carried until then
claimed otherwise and the reader who trusted it would have looked for a guard that does not exist.
What that costs is worth naming now that #302 put something load-bearing behind it: the receipt writer
(``cdp.write_receipt``, and ``Session.publish``'s call to it) is attacked by nothing in this file, and
its only pin is ``manifest.py --self-test``'s
``the_receipt_the_driver_writes_is_the_one_record_reads``, which drives the real
``cdp.Session.publish`` and reads the result back with the real ``record``. Hollow the writer and that
suite goes red (measured 13 Nov: ``the publish put the frame on disk and wrote no receipt beside it``);
this harness, which runs ``pages.py --self-test``, stays green, because the page suite reads a log line
about the publish and not the file.

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
# truncated the run. `green` is the only one that means "the guard believes 81 rules hold"; a run
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
# The classes below are four: the floor every guard of this shape has, which now shows up at three
# sites, and one claim about what the 81 rules do not look at. Each is re-measured by its rows on
# every push rather than remembered:
#
#   1. A check cannot police its own deletion -- the floor `attack-self-test.py` documents for
#      `mutate.py`'s `if bad:`. Three sites: the canary pair's comparison, the verdict's `if bad:`,
#      and the probe harness's own arms. The last is the largest, because what it guards is the
#      guard for all 81 rules.
#   2. What the rules do not look at: the bytes of the frame a refusal keeps.
#
# Two classes ran with these and are gone. `rule()`'s crash arm is witnessed by a canary that raises
# on purpose (#323, `D-0072`). And #322's -- "a hollowed rule is invisible to the count", the class
# this file was written to describe, whose seven rows all printed `HOLE` -- is closed by `pages.py`'s
# probe pass: `_probe_every_rule` breaks the contract each rule names and requires THAT rule to go
# red, so those seven rows read CAUGHT now and the class retires with them. What did not close is the
# floor under the repair: every arm of the harness is one `if`, and deleting the
# `_rule_probes_are_live` call is silent. That is class 1 at a new site, listed rather than fixed.
# ---------------------------------------------------------------------------------------------
DOCUMENTED_HOLES = {
    "a probe-harness arm's own comparison is unguarded":
        "The probe pass is what retired #322's hole, so the pass needs a witness, and the only "
        "witnesses available in this language are more `if`s in the same file. Measured at the head "
        "these rows were written, each of these printed `capture self-test: 81 refusal rules hold` "
        "at exit 0 with the break live: delete the `_rule_probes_are_live(registry, ran, bad)` call "
        "from `self_test()`; switch off `if probed != len(registry)`, `if not reported`, `if arms != 4`, "
        "`if shapes != 4`, `if not applied`, `if not restored`, or arm 4's table-coverage witness; or satisfy a "
        "witness with a constant instead of driving the function it claims to drive. Same floor as "
        "the verdict's `if bad:` and the canary pair, and the same answer: listed. What would close "
        "the first of them -- the only one that costs the whole guard -- is a probe tally the verdict "
        "is HANDED as a third argument rather than a list `self_test()` chooses whether to fill; "
        "nothing smaller does, because every channel out of `self_test()` is a value it decides to "
        "put there. The mutations the arms DO catch are rows above: hollowing `_apply_probe`, making "
        "an undo not undo, dropping a verdict shape, and stopping `rule()` registering its rules all "
        "come back CAUGHT.",
    "the canary pair's own comparison is unguarded":
        "`if counts != [1, 0] or len(probe_bad) != 1:` is the only sentence proving `rule()` is not "
        "hollow, and it sits one level above the pair it reads: switched off, the canaries are "
        "still registered, the counts are still computed, and nothing compares them. The same floor "
        "as the verdict's `if bad:` -- a check cannot police its own deletion -- so it is listed "
        "rather than fixed. Its presence is not unpinned, though: the four rows that empty the rule "
        "helper in four different ways are all caught, and they are caught *because* this "
        "comparison is live.",
    "the verdict's final decision is unguarded":
        "`if bad:` is where every problem becomes an exit code, and it is the floor for one reason "
        "the other two arms of that function are not: a finding raised *because* `if bad:` stopped "
        "printing has nowhere to be printed but `if bad:`. #322's probe harness pinned the other two "
        "-- `self_test_verdict` is a different function from the one whose findings it reports, so "
        "arm 2 can ask it four shapes, and switching the reach check off or turning its `return 1` "
        "into `return 0` is CAUGHT now -- which is why this class is one row where it used to be "
        "three. What closes the last one is the same thing named above: a probe tally handed to the "
        "verdict rather than a pair of lists that can be handed to it empty.",
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
    "the verdict's problem list is switched off":
        "the verdict's final decision is unguarded",
    "the probe pass is deleted from the self-test":
        "a probe-harness arm's own comparison is unguarded",
    "the coverage count is switched off":
        "a probe-harness arm's own comparison is unguarded",
    "the reports-versus-raises witness is switched off":
        "a probe-harness arm's own comparison is unguarded",
    "the arms count is switched off":
        "a probe-harness arm's own comparison is unguarded",
    "the shapes count is switched off":
        "a probe-harness arm's own comparison is unguarded",
    "the applied witness is switched off":
        "a probe-harness arm's own comparison is unguarded",
    "the restore witness is switched off":
        "a probe-harness arm's own comparison is unguarded",
    "the table-coverage witness is switched off":
        "a probe-harness arm's own comparison is unguarded",
    "the verdict witness answers without asking":
        "a probe-harness arm's own comparison is unguarded",
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
     '    operator_print(f"  what the page actually renders is at {path} -- this run refused to publish "\n'
     '                   "it, so it is not evidence; read it with REFUSED.txt beside it")',
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
     '                   "it, so it is not evidence; read it with REFUSED.txt beside it")',
     '                   "it, read it with REFUSED.txt beside it")'),
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
     # The call gained its fixture argument in #302 -- `publish(fixture)` is what writes the receipt
     # `manifest.py record` reads -- and the needle is the call, so the needle moved with it. The
     # four leading spaces are the tail of the call's real eight-space indent: the row patches a
     # substring, which is why an argument added to the call breaks the anchor rather than shifting
     # it. Measured: with the old needle the gate said `anchor matched 0 times in pages.py, want
     # exactly 1`, which is a PROBLEM, not a pass -- an attack that cannot be applied tests nothing.
     "    cap.session.publish(mode)", "    pass  # publish is skipped"),

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

    # ---- the plumbing that makes 81 rules mean 81 rules ----
    (CAUGHT, "pages.py", "the rule helper records zero problems for every rule",
     "        (ran_to if ran_to is not None else ran).append((why, len(problems)))",
     "        (ran_to if ran_to is not None else ran).append((why, 0))"),
    (CAUGHT, "pages.py", "the rule helper drops the problems it was handed",
     "        (bad_to if bad_to is not None else bad).extend(problems)",
     "        pass  # the problems are dropped"),
    # Row re-pointed 14 Nov by #322, which added a second `body(problems)` -- the probe pass calls a
    # rule's body a second time on purpose -- so the bare statement matched twice and the suite
    # refused the row (`INVALID`, exit 1) instead of counting a patch that would have edited two
    # places while reporting one. The `except` comment is the half that says WHICH call: `rule()`'s
    # own arm reads "a rule that throws has not passed", the probe's reads "an exception is an answer
    # here". Anchoring on the pair means the row cannot silently start patching the probe instead.
    (CAUGHT, "pages.py", "the rule helper never calls the rule body it is given",
     "            body(problems)\n        except Exception as exc:  # a rule that throws has not "
     "passed",
     "            pass  # the body is never called\n        except Exception as exc:  # a rule that "
     "throws has not passed"),
    (CAUGHT, "pages.py", "the canary pair is deleted, leaving nothing that proves the helper is not hollow",
     '    rule("canary", lambda problems: problems.append("a problem"), probe_ran, probe_bad)\n'
     '    rule("canary", lambda problems: None, probe_ran, probe_bad)',
     "    pass  # the canaries were deleted"),
    (HOLE, "pages.py", "the canary comparison is switched off",
     "    if counts != [1, 0] or len(probe_bad) != 1:", "    if False:"),
    # Retired on 13 Nov, from HOLE to CAUGHT: `pages.py` now registers a third canary that raises on
    # purpose, so the arm speaks on every run and this patch turns the guard red with the arm's own
    # sentence named (#323, `D-0072`). The row stays attacked rather than deleted -- the arm can be
    # emptied again, and a guard is only as good as the run that re-measures it.
    (CAUGHT, "pages.py", "a rule that throws is silenced in the rule helper",
     '            problems.append(f"{why}: {type(exc).__name__}: {exc}")',
     "            pass  # a crashing rule says nothing"),
    (HOLE, "pages.py", "the verdict's problem list is switched off",
     "    if bad:\n        for p in bad:", "    if False:\n        for p in bad:"),
    (CAUGHT, "pages.py", "the reach check is switched off",
     "    if len(ran) != EXPECTED_RULES:", "    if False:"),
    (CAUGHT, "pages.py", "the reach check runs and returns success",
     '              "(a `return` inside self_test() skips every rule below it)", file=sys.stderr)\n'
     "        return 1",
     '              "(a `return` inside self_test() skips every rule below it)", file=sys.stderr)\n'
     "        return 0"),

    # ---- #297's title, and the class #322 filed about it, closed by `pages.py`'s probe pass. These
    #      seven bodies are still emptied here on purpose: with the pass live each one reddens the
    #      run, because `_probe_every_rule` breaks that rule's own contract and insists THAT rule
    #      report. They stay rows rather than being deleted -- if a table entry is dropped, or the
    #      pass is switched off, the rule goes back to being hollowable in silence and these rows come
    #      back as NEW HOLE instead of the count line quietly staying true.
    (CAUGHT, "pages.py", "one refusal rule's body is emptied -- where its frames are",
     "    def a_refused_run_says_where_its_frames_are(problems):",
     "    def a_refused_run_says_where_its_frames_are(problems):\n        return"),
    (CAUGHT, "pages.py", "one refusal rule's body is emptied -- frames kept apart per fixture",
     "    def each_fixture_keeps_its_refused_frames_apart(problems):",
     "    def each_fixture_keeps_its_refused_frames_apart(problems):\n        return"),
    (CAUGHT, "pages.py", "one refusal rule's body is emptied -- the moved-out-of-the-queue claim",
     "    def a_kept_frame_is_moved_out_of_the_publish_queue(problems):",
     "    def a_kept_frame_is_moved_out_of_the_publish_queue(problems):\n        return"),
    # Every rule added to this suite inherits the same floor, so every rule added owes a row: the
    # count went 77 -> 78 for #304's disposal rule, 78 -> 80 for #317's two rules for the arms #304
    # could not reach, and 80 -> 81 for #302's fixture-on-the-publish rule; a body emptied to `return`
    # keeps it at whatever the pin says, either way. Listing each is the honest price of adding one --
    # the class text claims every rule this suite empties came back unseen, and an unlisted hollowing
    # would make that sentence false in the one direction nothing else checks.
    (CAUGHT, "pages.py", "one refusal rule's body is emptied -- one disposal per refusal",
     "    def a_refusal_whose_report_cannot_be_written_still_finishes_its_disposal(problems):",
     "    def a_refusal_whose_report_cannot_be_written_still_finishes_its_disposal(problems):\n"
     "        return"),
    (CAUGHT, "pages.py", "one refusal rule's body is emptied -- the fixture arm's disposal",
     "    def a_refusal_in_a_fixture_still_disposes_of_itself_once(problems):",
     "    def a_refusal_in_a_fixture_still_disposes_of_itself_once(problems):\n        return"),
    (CAUGHT, "pages.py", "one refusal rule's body is emptied -- the sweep's dead keeping",
     "    def a_page_whose_keeping_died_does_not_stop_the_sweep(problems):",
     "    def a_page_whose_keeping_died_does_not_stop_the_sweep(problems):\n        return"),
    (CAUGHT, "pages.py", "one refusal rule's body is emptied -- the fixture named to the publish",
     "    def a_clean_run_names_its_fixture_to_the_publish(problems):",
     "    def a_clean_run_names_its_fixture_to_the_publish(problems):\n        return"),

    # ---- the bytes ----
    (HOLE, "cdp.py", "the kept frame is written as an empty file",
     '        with open(path, "wb") as fh:\n            fh.write(data)',
     '        with open(path, "wb") as fh:\n            fh.write(b"")'),

    # ---- #317: a refusal's report and its keeping must not be able to leave the driver ----
    #
    # Four rows, one per guard the fix installs, because the defect was three different escapes in
    # three different places and each one exits 1 exactly like a refusal. The first takes a sentence
    # off the stream it is spoken on and puts it back on the other, so a bare `print` there is the
    # pre-fix tree, not a stylistic variant: measured at `e16ab68`, one dead `print` on the sweep's
    # first refused page cost 11 of its 12 measurements and both fixture arms printed nothing at all.
    (CAUGHT, "cdp.py", "the refusal headline goes back to a bare print, so a dead stderr aborts the "
                       "disposal it is reporting",
     '    operator_print(f"REFUSED: {exc}")',
     '    print(f"REFUSED: {exc}", file=sys.stderr)'),
    (CAUGHT, "cdp.py", "the disposal's wrap catches only what a print raises, so an OSError from the "
                       "keep escapes the driver",
     "        except (Exception, SystemExit) as exc:  # noqa: BLE001 -- a dying disposal is not a "
     "second refusal",
     "        except (TypeError, SystemExit) as exc:  # noqa: BLE001"),
    (CAUGHT, "pages.py", "the sweep's per-page keeping catches only what a print raises, so one dead "
                         "keep ends the run",
     "    except (Exception, SystemExit) as died:  # noqa: BLE001 -- one page's keeping is not the run",
     "    except (TypeError, SystemExit) as died:  # noqa: BLE001"),
    (CAUGHT, "pages.py", "the fit count is never told a disposal died, so it prints a numerator over "
                         "frames that are not there",
     "                missing = unreported.get(width) or []",
     "                missing = []  # the count is kept from admitting it"),

    # ---- #322: the probe pass, on itself. Nine CAUGHT rows for the machinery, nine HOLE rows for
    #      the arms. The CAUGHT rows break the pass by breaking what it does -- ignore the table, aim
    #      every rule at the same harmless break, hollow `_apply_probe`, make an undo not undo, stop
    #      `rule()` registering, drop a verdict shape -- and each turns the run red. The HOLE rows
    #      break the checks that watch the pass, which is the floor: see the class text. They are
    #      listed rather than deleted because a floor nobody re-measures is indistinguishable from a
    #      floor that was never there.
    (CAUGHT, "pages.py", "the probe table is ignored, so no rule is broken at all",
     "        spec = RULE_PROBES.get(why)", "        spec = None"),
    (CAUGHT, "pages.py", "every rule is probed with the same unrelated break",
     "        spec = RULE_PROBES.get(why)", '        spec = ("hollow", "cdp.capture_out")'),
    (CAUGHT, "pages.py", "the probe loop runs over nothing",
     "    for why, body in registry:", "    for why, body in []:"),
    (CAUGHT, "pages.py", "a rule that answered its own break is not counted as probed",
     "        probed += 1", "        probed += 0"),
    (CAUGHT, "pages.py", "`_apply_probe` is hollowed to a no-op",
     '    kind = spec[0]\n    if kind in ("hollow", "refuse"):',
     '    return (lambda: None, "nothing")\n    kind = spec[0]\n'
     '    if kind in ("hollow", "refuse"):'),
    (CAUGHT, "pages.py", "a probe's undo does not put the guard back",
     '        return (lambda: setattr(owner, name, original)), f"{kind} {spec[1]}"',
     '        return (lambda: None), f"{kind} {spec[1]}"'),
    (CAUGHT, "pages.py", "`rule()` stops registering its rules, so the table names 81 strangers",
     "            registry.append((why, body))", "            pass"),
    # Measured 14 Nov: this one went red on its own, which is why the raise-only degradation is a
    # caught row and not a listed floor. `if not reported:` is the arm that names it, and the arm is
    # itself one `if` -- so the degradation is witnessed while the witness stays switchable, and both
    # halves are rows here rather than one of them being a claim in a comment.
    (CAUGHT, "pages.py", "no rule is ever counted as having reported a problem",
     "        if problems:\n            reported += 1", "        if False:\n            reported += 1"),
    (CAUGHT, "pages.py", "a shape is dropped from the verdict witness",
     '            ("one harness finding and 81 clean rules", full, [f"{PROBE}: a"], 1),', ""),
    (HOLE, "pages.py", "the probe pass is deleted from the self-test",
     "    _rule_probes_are_live(registry, ran, bad)", "    pass"),
    (HOLE, "pages.py", "the coverage count is switched off",
     "    if probed != len(registry):", "    if False:"),
    (HOLE, "pages.py", "the reports-versus-raises witness is switched off",
     "    if not reported:", "    if False:"),
    (HOLE, "pages.py", "the arms count is switched off", "    if arms != 4:", "    if False:"),
    (HOLE, "pages.py", "the shapes count is switched off", "    if shapes != 4:", "    if False:"),
    (HOLE, "pages.py", "the applied witness is switched off", "    if not applied:", "    if False:"),
    (HOLE, "pages.py", "the restore witness is switched off", "    if not restored:", "    if False:"),
    (HOLE, "pages.py", "the table-coverage witness is switched off",
     '    if not any(p.startswith(f"{PROBE}:") for p in witness):', "    if False:"),
    (HOLE, "pages.py", "the verdict witness answers without asking",
     "        with contextlib.redirect_stderr(io.StringIO()), "
     "contextlib.redirect_stdout(io.StringIO()):\n"
     "            code = self_test_verdict(list(ran_shape), list(bad_shape))",
     "        code = want"),

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
     '    operator_print(f"  what the page actually renders is at {path} -- this run refused to publish "',
     "    # The pointer an operator follows, and the disclaimer that keeps it out of the evidence set.\n"
     '    operator_print(f"  what the page actually renders is at {path} -- this run refused to publish "'),
]

# The suite's own count pin, on the pattern of `mutate.py`'s three and `attack-self-test.py`'s one.
# A row deleted from ROWS makes the tally smaller and every remaining row still reports its expected
# verdict, so the run prints a confident "40 attacks, 0 problems" over a guard that lost a test.
# 54 -> 72 on 14 Nov: the nine rows for #322's probe machinery and the nine for its arms.
EXPECTED_ROWS = 72


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
        problems.append("`make verify` no longer runs this suite. "
                        f"{EXPECTED_ROWS} attacks nobody invokes are a list of intentions, not a "
                        "gate; if the step was deleted because this file's verdict became "
                        "inconvenient, the finding is in the verdict")
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
