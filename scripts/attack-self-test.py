#!/usr/bin/env python3
"""Attack ``mutate.py --self-test``: make it print a green verdict with the symptom live.

``--self-test`` is the only thing standing between ``scripts/mutate.py`` and its own history. It
counts rules, compares captured output, and refuses a hollowed checker -- and every one of those
mechanisms is itself code that can be hollowed. A reviewer approving a guard is approving a claim
about the guard, and that claim is evidence only if something has tried to break it. This file is
that attempt, run on every push.

It is the successor to ``/tmp/opencode/attack118.py``, which defended #118 through 42 attacks and
four documented holes and then evaporated with the scratch directory that held it. The tally quoted
in #118's thread could not be re-run by the reviewer who asked for it, or by anyone afterwards;
issue #124 is that eviction. The suite's own history is why it belongs in the tree rather than in
a claim: it once copied only the file under test, so every run died on ``ModuleNotFoundError: No
module named 'mutset'`` and **every "caught" it had ever reported had been a missing import**;
separately, eight of its patches were no-ops (``X = (`` to ``X = () or (`` leaves the tuple
intact). Both were found by running the suite against a broken version of itself, which is the
only way either is ever found.

What counts as an attack
------------------------

An attack has to leave the runner looking like a runner. Deleting the line that decides the exit
code is not an attack -- the answer to "what if you delete the guard?" is "then there is no
guard", and that needs no experiment. Every row below instead makes the runner's *content* wrong
while its shape stays intact: a comparison whose operand order is honest but whose answer is
fixed, a checker that returns the right shape and the wrong verdict, a fixture that observes what
it was told rather than what happened. Rows that only remove a check are kept where they teach
something -- as the floor, below.

Verdicts, and the difference between them is the design:

  CAUGHT       the patched self-test refused, in words. The guard is load-bearing.
  GREEN        an honest edit (a comment, a reworded docstring, swapped operands) that the guard
               left alone. As load-bearing as a catch: a guard that turns red on a comment is
               matching text, and a guard that is a tax gets switched off. A suite in which
               everything is caught is testing nothing.
  HOLE         the patched self-test printed its confident green summary with the symptom live.
               A finding about the guard, not about the attack.
  INVALID      the patch did not produce a runnable module -- a traceback, an unexpected exit, or
               an anchor that did not match exactly once. An attack that does not compile proves
               nothing, and the first version of this suite scored one as a catch: row 39's
               replacement never closed a paren, and the crash check could not see the failure
               because the run raised before printing anything.

Two properties are enforced rather than described. **Every anchor must occur exactly once** -- a
patch matching nothing is a no-op and a patch matching twice edits two places while reporting one,
and in both cases the tally keeps counting while the guard stays where it was. **A row expected to
stay green fails the run if the guard turns red on it.**

The floor
---------

``DOCUMENTED_HOLES`` names every attack the current guard does not see, with what would close it.
A hole is listed rather than quietly dropped, which is the only way the next reader knows the
tally has a floor; a NEW hole fails this run, and a listed hole that has stopped being a hole also
fails it, so the list cannot rot in either direction. Every entry is the same class, and #125
tracks it from the other end: **a counted rule cannot police its own call site.** A pin can be
switched off, an auditor replaced by a constant, a control handed an answer it never computed --
each of those is an edit one level above the thing the rules compare, and a rule that policed its
own call would move the number it exists to hold.

Cost: about two seconds. The self-test itself takes 0.06 s, so the whole suite is cheaper than one
``go test`` package and belongs in ``make verify`` beside the thing it guards, not in a job
somebody can skip.
"""

import hashlib
import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
PYTHON = sys.executable or "python3"

GREEN_SUMMARY = re.compile(r"^self-test: \d+ verdict rules, \d+ selection rules and "
                           r"\d+ snapshot rules hold$")

CAUGHT, HOLE, GREEN = "caught", "hole", "green"

# What would close each hole, in one sentence each. The point of writing them down is that they
# are cheap: five of the seven are one more probe or one more control, and the other two are the
# irreducible part of the design. A row names the class it belongs to, because several attacks
# reach the same class from different lines and the class is what gets closed.
DOCUMENTED_HOLES = {
    "a control is handed an answer it never computed":
        "control() takes its answer as an argument, so a call site can hand it a constant and it "
        "will report a refusal it never computed; the same call site can append the pair by hand "
        "and skip the checker entirely. Closes by having control() call the checker itself rather "
        "than receive its result -- which is what #119's controls already do, so what is left are "
        "the older call sites.",
    "an auditor's own guards are uncovered":
        "three of only_block_problems' guards -- no block at all, an unlocatable region, a main "
        "that never reports unmatched ids -- have no control that feeds the auditor a main "
        "matching them, so each branch is uncovered and can be deleted with nothing noticing. "
        "Closes by adding those three mains to ONLY_BLOCK_CONTROLS.",
    "a count pin can be switched off":
        "a pin is invisible until a rule goes missing, and verdict_guard_problems probes five "
        "shapes of verdict -- absent, non-tuple, emptied, forged, two-counts -- and not the "
        "count-mismatch shape the pins exist for. Closes by one more probe that hands "
        "self_test_verdict a report one rule short of its pin.",
    "the canary comparison can be switched off":
        "the same shape as the pins one level down: probe_rules and probe_bad keep being filled "
        "and the comparison that reads them can be switched off with nothing noticing. Closes by "
        "a probe over the canary itself, which is the reach argument applied to the reach check.",
    "the emptied-list guard cannot see the clear it names":
        "`len(bad) != raised` is documented as catching the list being emptied on the way out, "
        "and Problems.clear() zeroes the counter in the same breath, so the two move together and "
        "the comparison holds. It catches a list emptied WITHOUT its count, which is a different "
        "bug nobody writes. Closes by capturing the count before the rules run and comparing the "
        "list against that, rather than against a field the same clear() resets.",
    "the verdict's final decision is unguarded":
        "`if bad:` is where every problem becomes an exit code, `bad.raised` is the number the "
        "emptied-list guard compares, and main's call of self_test_verdict is the last link. Each "
        "can be replaced by a constant one level above anything a rule compares. This is the "
        "floor: a check cannot police its own deletion, so it is listed rather than fixed.",
    "a git that exited non-zero is believed by tracked_paths":
        "the snapshot rules probe the non-zero-exit refusal for ignored_paths and not for "
        "tracked_paths, so `return None` there can be deleted and the run goes on treating a "
        "failed `git ls-files` as 'nothing is tracked' -- which is a snapshot that silently omits "
        "the source. Closes by one more fixture, on the pattern of the ignored_paths one.",
}

# ---------------------------------------------------------------------------------------------
# The attacks: (expect, what it does, anchor, replacement).
#
# Anchors are copied from the file and matched exactly once. They sit on statements, never on an
# aligned composite literal (a gofmt realignment retires the match silently -- the lesson in the
# repo's own constraint list) and never on the three EXPECTED_* numbers, which a rebase moves
# legitimately and an anchor that breaks on a legitimate change is a row that stopped attacking
# while still being counted.
# ---------------------------------------------------------------------------------------------
ATTACKS = [
    # ---- the counters: `rule`, `control`, `run_rule`, and the canary that guards all three ----
    (CAUGHT, "rule() stops reporting the problems it was handed",
     "    for problem in problems:", "    for problem in []:"),
    (CAUGHT, "rule() stops counting, so a deleted rule is not a smaller number",
     "    rules.append((name, len(problems)))", "    pass"),
    (CAUGHT, "control() is satisfied by a checker that said nothing",
     '    rule(rules, bad, name, [] if problems else [f"it said nothing, {why}"])',
     "    rule(rules, bad, name, [])"),
    (CAUGHT, "control() passes whatever it is handed, including silence",
     '    rule(rules, bad, name, [] if problems else [f"it said nothing, {why}"])',
     '    rule(rules, bad, name, problems or ["nothing at all"])'),
    (CAUGHT, "run_rule() never calls the body it is given",
     "    body(problems)", "    pass"),
    (HOLE, "the canary comparison is switched off",
     "    if len(probe_rules) != 5 or len(probe_bad) != 3:", "    if False:"),
    (CAUGHT, "the canary keeps its count and loses its assertions",
     '    rule(probe_rules, probe_bad, "canary", ["a problem"])',
     '    rule(probe_rules, probe_bad, "canary", [])'),

    # ---- the verdict gate ----
    (HOLE, "the list and its counter cleared together are not noticed",
     "    return vrules, rules, srules, bad, bad.raised",
     "    bad.clear()\n    return vrules, rules, srules, bad, bad.raised"),
    (CAUGHT, "a hand-forged rule pair is no longer refused",
     "    if forged:", "    if False:"),
    (HOLE, "the verdict count pin is switched off",
     "    if len(vrules) != EXPECTED_VERDICT_RULES:", "    if False:"),
    (HOLE, "the auditor's 'no block at all' guard is switched off",
     "    if not found:", "    if False:"),
    (HOLE, "the verdict prints green whatever the list holds",
     "    if bad:", "    if False:"),
    (CAUGHT, "a set that could not run contributes nothing and is reported as run",
     "        if tally is None:\n            return totals, False",
     "        if tally is None:\n            tally = {k: 0 for k in totals}"),

    # ---- deletions: the pins exist for these, and only for these ----
    # Two-line anchors where a rule is a call plus its checker argument, because a one-line
    # deletion leaves an orphaned continuation and the row would be scored INVALID -- which is
    # exactly what the old suite's row 39 did, and it was scored as a catch.
    (CAUGHT, "one verdict rule is deleted",
     '    ("a case whose verdict disagrees with it",\n'
     '     lambda: case_problems("caught", "by a test", "survived", "", None)),', ""),
    (CAUGHT, "one selection rule is deleted",
     '        rule(rules, bad, "main\'s --only handling",\n'
     '             only_block_problems(src) if src else ["main\'s source could not be read"])',
     "        pass  # the rule was deleted"),
    (CAUGHT, "one snapshot rule is deleted",
     '    rule(srules, bad, "the table consulted the ignore callback once per row",\n'
     '         spy_problems(seen, rows))', "    pass  # the rule was deleted"),

    # ---- the --only auditor: the text audit #118 was written for ----
    (CAUGHT, "the deny list is switched off, so any effect on the selection is honest",
     "        if code not in ONLY_BLOCK_HONEST:", "        if False:"),
    (CAUGHT, "the deny list denies nothing",
     "ONLY_BLOCK_HONEST = (", "ONLY_BLOCK_HONEST = () and ("),
    (CAUGHT, "a main whose `if missing:` branch exits 0 is accepted",
     '    elif "return 1" not in src.split("if missing:")[-1]:', "    elif False:"),
    (CAUGHT, "an auditor replaced by a constant is caught by its own controls",
     "def only_block_problems(src):", "def only_block_problems(src):\n    return []"),
    (HOLE, "the auditor's 'region cannot be located' guard is switched off",
     "    if end not in rest:", "    if False:"),
    (HOLE, "the auditor's 'never reports unmatched ids' guard is switched off",
     '    if "if missing:" not in src:', "    if False:"),

    # ---- the selection, consumed ----
    (CAUGHT, "each set's tally is thrown away",
     "            totals[k] += tally[k]", "            pass"),
    (CAUGHT, "only the first set in the selection is run",
     "    for path, label in sets:", "    for path, label in sets[:1]:"),
    (CAUGHT, "the driven-selection fixture cannot see a skipped set",
     "    if ran_all != want_ran:", "    if False:"),
    (CAUGHT, "the driven-selection fixture cannot see a set run twice",
     "    if calls != want_calls:", "    if False:"),
    (CAUGHT, "the driven-selection fixture cannot see a sum thrown away",
     "    if totals != want_totals:", "    if False:"),

    # ---- the verdict half: the comparisons over captured `go test` output ----
    (CAUGHT, "a verdict kind with no report label is believed",
     "    return [] if kind in VERDICT_TAG else", "    return [] if True else"),
    (CAUGHT, "the failing-test names a run extracts are dropped on the floor",
     '            names.append(line.split("FAIL:")[1].split()[0])', "            pass"),

    # ---- the snapshot half: #119's fix, and the rules that hold it ----
    (CAUGHT, "the ignore loop's short-circuit becomes a prefix test",
     "            if name in gone:", "            if any(name.startswith(g) for g in gone):"),
    (CAUGHT, "a copy that came back without a tracked file is accepted",
     "    gone = []\n    for path in tracked:", "    return []\n    for path in tracked:"),
    (HOLE, "a git that exited non-zero is believed by tracked_paths",
     "    if done.returncode != 0:\n        return None", "    if False:\n        return None"),

    # ---- honest edits: these must stay green, or the guard is matching text it should ignore ----
    (GREEN, "a comment is added inside self_test()",
     "    return vrules, rules, srules, bad, bad.raised",
     "    # nothing below this line is counted; the reach check is in the caller\n"
     "    return vrules, rules, srules, bad, bad.raised"),
    (GREEN, "a docstring is reworded",
     "    \"\"\"Count one self-test rule, and report whatever it found.",
     "    \"\"\"Count one rule, and report every problem it found."),
    (GREEN, "a comment is added above the rule counter",
     "def rule(rules, bad, name, problems):",
     "# The one place a rule is both counted and reported. See self_test_verdict for the pins.\n"
     "def rule(rules, bad, name, problems):"),
    (GREEN, "a comparison's operands are swapped",
     "    if len(vrules) != EXPECTED_VERDICT_RULES:",
     "    if EXPECTED_VERDICT_RULES != len(vrules):"),

    # ---- the floor: call-site guts, listed in DOCUMENTED_HOLES with what would close each ----
    (HOLE, "a control is handed a problem it never found",
     "                    only_block_problems(patch(src)) if src else None,",
     '                    ["a problem"] if src else None,'),
    (HOLE, "the problem count is reported as the length of the list",
     "    return vrules, rules, srules, bad, bad.raised",
     "    return vrules, rules, srules, bad, len(bad)"),
    (HOLE, "everything below the unpacking is skipped, quietly",
     "    vrules, rules, srules, bad, raised = report",
     "    vrules, rules, srules, bad, raised = report\n    return 0"),
    (HOLE, "a rule pair is forged by hand instead of going through rule()",
     '        rule(rules, bad, "main\'s --only handling",\n'
     '             only_block_problems(src) if src else ["main\'s source could not be read"])',
     '        rules.append(("main\'s --only handling", 0))'),
]


# Which class each hole row belongs to. Several rows reach one class from different lines, and the
# class -- not the row -- is the thing a fix closes, so what is owed is keyed by class.
HOLE_CLASS = {
    "a control is handed a problem it never found":
        "a control is handed an answer it never computed",
    "a rule pair is forged by hand instead of going through rule()":
        "a control is handed an answer it never computed",
    "the auditor's 'no block at all' guard is switched off":
        "an auditor's own guards are uncovered",
    "the auditor's 'region cannot be located' guard is switched off":
        "an auditor's own guards are uncovered",
    "the auditor's 'never reports unmatched ids' guard is switched off":
        "an auditor's own guards are uncovered",
    "the verdict count pin is switched off": "a count pin can be switched off",
    "the canary comparison is switched off": "the canary comparison can be switched off",
    "the verdict prints green whatever the list holds": "the verdict's final decision is unguarded",
    "the problem count is reported as the length of the list":
        "the verdict's final decision is unguarded",
    "everything below the unpacking is skipped, quietly":
        "the verdict's final decision is unguarded",
    "a git that exited non-zero is believed by tracked_paths":
        "a git that exited non-zero is believed by tracked_paths",
    "the list and its counter cleared together are not noticed":
        "the emptied-list guard cannot see the clear it names",
}


# The suite's own count pin, on the pattern of mutate.py's three. A row deleted from ATTACKS makes
# the tally smaller and every remaining row still reports its expected verdict, so the run prints a
# confident "39 attacks, 0 problems" over a guard that lost a test. It is pinned to the number the
# run prints, and the number is re-derived from a run rather than computed by arithmetic -- the
# same rule the repo already applies to EXPECTED_VERDICT_RULES, where a hand-derived count was once
# off by one and the suite went red for the wrong reason.
EXPECTED_ROWS = 40


def siblings():
    """Every module in `scripts/`, so a sandbox can never be missing one.

    `mutate.py` puts its own directory on `sys.path` and imports `mutset`. When the old suite
    copied only the file under test, every run died on the import and every "caught" it had
    reported up to then had been a missing module. Copying the siblings is the fix; copying `*.py`
    rather than a named list is what keeps the fix from going stale the day a second module
    appears.
    """
    return sorted(p for p in os.listdir(HERE)
                  if p.endswith(".py") and not p.startswith("attack-self-test"))


def sandbox():
    """A private copy of the runner and its siblings, in the layout the runner expects.

    The `scripts/` level is not decoration. `SRC` is `dirname(dirname(__file__))`, so a copy parked
    directly in the scratch directory makes `SRC` that directory's parent -- and the selection
    fixtures build their expected labels with `set_label`, whose basename arm only fires when the
    fixture's `TemporaryDirectory` is outside `SRC`. Measured: parked flat, three selection rules
    failed on `tmpgh97hahz/alpha.json` where they wanted `alpha.json`, and the pre-flight refused
    to run a single attack. A suite that measured a guard against a baseline the guard could never
    pass would report every one of its rows as a catch.
    """
    box = tempfile.mkdtemp(prefix="plock-attacks-")
    os.makedirs(os.path.join(box, "scripts"))
    for name in siblings():
        shutil.copyfile(os.path.join(HERE, name), os.path.join(box, "scripts", name))
    return box


# Per-attack ceiling. The average is 0.15 s, so 60 s is ~400x the work a row actually does, and
# the number that matters is the product: a patched runner that hangs is reachable -- several rows
# edit loop control -- and the ceiling multiplied by the row count is the longest `make verify` can
# ever take on this file. At 300 s that product was 40 x 300 s, about two hours, on the gate whose
# entire argument for existing is that it costs seconds.
TIMEOUT = 60


def run(box):
    """Run the self-test in `box` and classify what came back. Never raises.

    `silent` is its own state and not a variant of green: a self-test that exits 0 without
    printing its summary has not reported that anything holds, it has reported nothing, and the
    difference is the whole content of #118.
    """
    try:
        proc = subprocess.run([PYTHON, os.path.join(box, "scripts", "mutate.py"), "--self-test"],
                              capture_output=True, text=True, cwd=box, timeout=TIMEOUT)
    except subprocess.TimeoutExpired:
        # Its own state of the message, because "did not run" and "did not stop" are different
        # diagnoses. It names no patch on purpose: this branch is also reached by the baseline
        # pre-flight, which runs the unpatched copy, and there a hang means mutate.py loops on its
        # own -- so "the patched self-test ... its patch" printed a suspect that does not exist,
        # directly under "HARNESS: the unpatched copy does not pass its own self-test". The caller
        # prints the row name, so this stays a claim about what happened, not about who caused it.
        return "invalid", "", (f"the self-test did not finish in {TIMEOUT}s -- a hang, not a "
                               f"broken patch")
    except (OSError, subprocess.SubprocessError) as exc:
        return "invalid", "", f"the sandboxed self-test could not be run: {exc}"
    out, err = proc.stdout.strip(), proc.stderr
    if "Traceback" in err or proc.returncode not in (0, 1):
        return "invalid", out, err
    lines = out.splitlines()
    if proc.returncode == 0 and lines and GREEN_SUMMARY.match(lines[-1]):
        return "green", out, err
    if proc.returncode == 0:
        return "silent", out, err
    if proc.returncode == 1 and out.startswith("self-test FAILED"):
        return "caught", out, err
    # Exit 1 that did not say why: a refusal the suite cannot vouch for is the harness's problem,
    # not the guard's.
    return "invalid", out, err


def main():
    # `None` means no argument was given; `""` means an argument was given and it is empty. The
    # distinction is the whole point and the first version of this line collapsed it to `""`,
    # which made the falsy form mean "no filter" -- so `attack-self-test.py ""` ran the entire
    # gate. That is honest output for the wrong claim: the Makefile step that invokes this with
    # an argument is named `empty selection refuses`, and a run of all 40 rows reports green
    # while proving something else entirely. The obvious future edit -- passing "$${FILTER}" with
    # FILTER unset -- lands there by accident.
    #
    # So an empty filter matches NOTHING (see the loop below), which is what a filter that empty
    # means: nothing was asked for, nothing ran, the run refuses. `mutate.py` answers the same
    # question with `args.only is not None` for the same reason; this is the third copy of that
    # rule, and it is now named in all three places rather than implied.
    only = sys.argv[1] if len(sys.argv) > 1 else None
    # One name for the rule, so the fifth copy of it cannot be written as `if not only` and
    # silently mean something else. Every suite-wide claim below is gated on this, and the
    # gate is what makes a selected run assert only what it can see.
    selected = only is not None
    # The suite-wide pins describe the whole suite, so they cannot be asserted over a subset: a
    # filtered run would fire `ran != len(ATTACKS)` and every class whose rows were filtered out,
    # which measured on this tree turned `attack-self-test.py canary` -- two rows, both answering
    # correctly -- into exit 1 with seven problems naming classes nobody touched. A contributor
    # following a red CI run to this file would conclude the guard had regressed. So a selected run
    # says what it is, and asserts only what it can see.
    with open(os.path.join(HERE, "mutate.py"), encoding="utf-8") as fh:
        original = fh.read()
    before = {name: hashlib.sha256(open(os.path.join(HERE, name), "rb").read()).hexdigest()
              for name in siblings()}

    box = sandbox()
    try:
        state, out, err = run(box)
        if state != "green":
            print(f"HARNESS: the unpatched copy does not pass its own self-test ({state}).")
            print(f"  stdout: {out[:600]}")
            print(f"  stderr: {err[:600]}")
            print("Every attack below would be measured against a baseline the guard was never "
                  "able to meet, so none is run. Fix the sandbox or the runner first.")
            return 2

        counts = {"caught": 0, "green": 0, "hole": 0, "invalid": 0, "wrong": 0}
        problems, seen_holes = [], []
        for expect, what, anchor, replacement in ATTACKS:
            # `not only` here is deliberate and is the other half of the fix: an empty filter
            # must not match every row the way `"" in anything` does. With only=None the filter
            # is off and every row runs; with only="" no row is selected and the `if not ran`
            # guard below refuses the run.
            if selected and (not only or only.lower() not in what.lower()):
                continue
            hits = original.count(anchor)
            if hits != 1:
                counts["invalid"] += 1
                problems.append(f"anchor matched {hits} times in mutate.py, want exactly 1: "
                                f"{what}")
                print(f"INVALID  {what} ({hits} anchor matches)")
                continue
            patched = original.replace(anchor, replacement, 1)
            if patched == original:
                counts["invalid"] += 1
                problems.append(f"the replacement is the text it replaces, so nothing was "
                                f"attacked: {what}")
                print(f"INVALID  {what} (the patch is a no-op)")
                continue
            with open(os.path.join(box, "scripts", "mutate.py"), "w", encoding="utf-8") as fh:
                fh.write(patched)
            state, out, err = run(box)
            if state == "invalid":
                counts["invalid"] += 1
                # "does not run" asserted something this branch does not know. It fires on a
                # traceback, on an unexpected exit code, and on a timeout -- and on a timeout the
                # module ran perfectly well and simply never stopped, which is the exact
                # conflation the message inside run() exists to keep apart. "did not answer
                # usefully" claims only what is true for all three; the text after the dash is
                # what names which one it was.
                problems.append(f"the patched module did not answer usefully, so the row proves "
                                f"nothing: {what} -- {(err or out).strip()[:200]}")
                print(f"INVALID  {what}")
                continue
            # `state` is what the patched self-test did; `expect` is what this row claims the
            # guard will do about it. A row expecting a hole is satisfied by exactly the states
            # that mean "the guard did not see it": a green summary, or a green exit that printed
            # no verdict at all.
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
                problems.append(f"OVER-STRICT {what} -- an honest edit turned the guard red, so "
                                f"it is matching text rather than behaviour: {out[:200]!r}")
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
        # Gated like the four pins below it. This one hides in a `for` header, which is how a
        # sweep for `if not only` misses it -- and an ungated copy is not a silent miss: a
        # selected run reports one phantom problem per class its filter left out, measured at
        # 7 for the empty filter on this tree.
        for name in DOCUMENTED_HOLES if not selected else ():
            if name not in seen_classes:
                problems.append(f"DOCUMENTED HOLE NO LONGER SEEN {name!r} -- either the guard got "
                                f"better, in which case remove the class and its rows, or the "
                                f"attack stopped attacking it")

        after = {name: hashlib.sha256(open(os.path.join(HERE, name), "rb").read()).hexdigest()
                 for name in siblings()}
        if after != before:
            problems.append("this suite modified the tree it was checking")

        ran = sum(counts.values())
        if selected:
            print(f"\nSELECTED RUN -- rows matching {only!r}, not a gate. The suite-wide pins "
                  f"(row count, hole coverage) are skipped because they describe the whole suite; "
                  f"run with no argument for the verdict CI applies.")
            # Scoping the pins out is what makes this clause necessary. With nothing suite-wide
            # left to run, a filter that matches nothing -- the commonest way to attempt no rows,
            # because it is a typo -- reports "0 problem(s)" next to exit 0, and the banner's
            # disclaimer is not what a reader scans. `ran == 0` is a claim about the selection, so
            # it belongs here and not beside the pins. Note the direction of travel: before the
            # pins were scoped, a typo'd filter failed loudly for the wrong reason; now it could
            # pass for no reason at all, which is the worse of the two.
            if not ran:
                problems.append(f"no row matched {only!r}: a run that attempts nothing proves "
                                f"nothing")
        if not selected and len(ATTACKS) != EXPECTED_ROWS:
            problems.append(f"the suite ran {len(ATTACKS)} attacks, expected {EXPECTED_ROWS}: a "
                            f"row was deleted, or stopped being attempted. A smaller tally is "
                            f"not a smaller claim")
        if not selected and ran != len(ATTACKS):
            problems.append(f"{ran} rows were classified and {len(ATTACKS)} were attempted: a row "
                            f"was skipped, and a skipped row is a row that reports nothing")
        if not selected and not counts["hole"]:
            problems.append("no row came back unseen. Every expectation in this suite is either "
                            "'the guard refuses' or 'the guard does not see it', so a classifier "
                            "stuck on either answer satisfies one half by breaking the other -- a "
                            "run with no holes at all means the classifier is stuck, which is "
                            "exactly how the old suite reported 42 catches it never made")
        print(f"\nTOTAL  {ran} attacks: {counts['caught']} caught, {counts['green']} honest "
              f"edits still green, {counts['hole']} holes, {counts['invalid']} invalid, "
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
        print(f"\nVERDICT: {ran} attacks, {len(problems)} problem(s), "
              f"{len(seen_classes)} hole class(es), {len(seen_holes)} hole row(s)")
        return 1 if problems else 0
    finally:
        shutil.rmtree(box, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
