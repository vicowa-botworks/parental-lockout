#!/usr/bin/env python3
"""Attack ``manifest.py --self-test``: make it print a green verdict with an evidence contract broken.

``manifest.py check`` is the gate that decides whether a committed screenshot is still evidence, and
``manifest.py --self-test`` (54 rules, the number its own verdict line prints) is the only thing that
checks that gate. Its sibling suites proved on their own guards that the rules are the attack surface
(#297 measured three hollowed clauses printing ``capture self-test: 76 refusal rules hold``;
``attack-self-test.py`` measured 42 phantom catches on ``mutate.py``), and the capture suite's
docstring has said since 13 Nov that this module "has NO row set here or in ``attack-self-test.py``".
This file is the row set that sentence asked for, and it says which half of that sentence survives.

It is the third of the family and inherits the shape the other two paid for rather than
rediscovering it: copy the module *and its siblings* into a box, assert every anchor matches exactly
once, keep ``CAUGHT``/``GREEN``/``HOLE``/``INVALID`` distinct, pin the row count, refuse a run in
which nothing came back unseen, and keep a ``DOCUMENTED_HOLES`` list that fails the run both when a
new hole appears and when a listed hole stops being one.

What a sandbox has to hold, measured rather than guessed. ``testing/capture/*.py`` alone is not a
baseline: ``manifest.py`` reads ``ROOT/Makefile`` at import to pin its own invocation, so a box
without the Makefile answers ``capture manifest self-test FAILED: ... the Makefile could not be
read`` with exit 1 and the pre-flight stops the run (correctly -- an attack measured against a
baseline the guard could never meet reports every row as a catch). And the census rule reads
``cdp.HERE/../../internal/web/templates``, so the templates come too. The box is therefore
``testing/capture/*.py`` **plus** ``internal/web/templates/*.tmpl`` **plus** ``Makefile`` -- no git,
no browser, no fixture, no writes outside the box.

The child is run with ``-B``, and the baseline is re-run before any row is trusted.

Expectations below are measurements, not wishes. Every one was run against this head with the patch
live in a copy of the tree, and the classification is what the patched self-test actually printed.
That matters for the rows this suite files as HOLE: the capture suite turned three of its floors
into CAUGHT rows by giving its guard a canary (#323, ``D-0072``); ``manifest.py`` has no canary
yet, so its rule helper, reach pin and verdict arm are switchable today, and saying so in a comment
would be exactly the prose-without-a-runner that #297 was about. Each hole row names its closes.

Cost: about four seconds, against the ~1 s self-test it attacks and the ~3m9s ``make verify`` that
runs this beside the thing it guards.

And the boundary, named because #326 asked for three surfaces and two of them moved: the single-frame
drivers (``lock.py``, ``testlock.py``) are *driven* by ``pages.py --self-test`` since #296 and
attacked by rows in ``attack-capture-self-test.py`` since #304 -- this file does not duplicate that
coverage. What is NOT covered anywhere, measured 14 Nov rather than assumed: deleting a
``cdp.require_*`` CALL from a driver's capture path leaves every gate green, because the rules and
probes grade what the helpers do when called, not whether the path calls them. Two probes, each in a
copy of the tree, each followed by both suites: ``cdp.require_records`` and
``cdp.require_labelled_cells`` deleted from ``pages.py``'s shoot loop, and
``cdp.require_labelled_cells`` deleted from ``testlock.capture()`` -- every time ``pages.py
--self-test`` printed ``capture self-test: 81 refusal rules hold`` and ``manifest.py --self-test``
printed ``capture manifest self-test: 54 rules hold``. That is a finding about
the other two suites, filed as its own issue (#326's comment carries the measurements), not a row
this suite could pass or fail.
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

# The two sentences self_test() ends with -- `capture manifest self-test: 54 rules hold ...` on
# success and `capture manifest self-test FAILED:` before its list -- both on stdout, unlike the
# capture suite's, whose failures go to stderr. Matching the exact heads keeps "refused" and "did
# not run" apart, which is the same distinction #302 insists on for the manifests themselves.
GREEN_SUMMARY = re.compile(r"^capture manifest self-test: \d+ rules hold")
FAILED_SUMMARY = re.compile(r"^capture manifest self-test FAILED:")

CAUGHT, GREEN, HOLE = "caught", "green", "hole"

# Rows are (expect, target, what, anchor, replacement). `what` names the attack and keys the hole
# class map; `anchor` must match the live module exactly once, and `shape_problems` checks that
# before a single child starts, so a row whose code moved says so instead of scoring a patch that
# was never applied.

ROWS = [
    # ---- the production contracts, one row per clause the self-test is named for ----
    # Each patch switches a refusal off without touching the rule that claims to watch it. These
    # came back CAUGHT as measured: the rules drive the real `check`/`record` over fake-git boxes
    # and compare sentences, so the refusal going dark reddens the run by itself.
    (CAUGHT, "manifest.py", "a rule's reach count never sees the rule, so the pin cannot miss it",
     "        ran.append(name)",
     "        pass  # the reach counter never sees the rule"),
    (CAUGHT, "manifest.py", "one rule is deleted from the run, and the reach pin names the loss",
     '    rule("a manifest recorded at another viewport is refused",\n'
     '         says("not 360", drive(wrong_width)))',
     "    pass  # the rule was deleted"),
    (CAUGHT, "manifest.py", "a replaced PNG's identity comparison is switched off",
     "        if want != got:",
     "        if False:"),
    (CAUGHT, "manifest.py", "the viewport comparison is switched off",
     '    if doc.get("viewport") != ARTIFACT_WIDTH:',
     "    if False:"),
    (CAUGHT, "manifest.py", "the shallow-clone guard is switched off",
     "    if history_is_shallow(run=run):",
     "    if False:"),
    (CAUGHT, "manifest.py", "the orphan-PNG sweep iterates over nothing",
     "    for name in on_disk:\n        if name not in seen:",
     "    for name in []:\n        if name not in seen:"),
    (CAUGHT, "manifest.py", "the driver-omission check is switched off",
     '        if spec.artifact + ".png" not in seen:',
     "        if False:"),
    (CAUGHT, "manifest.py", "the uncommitted-source check is switched off",
     "        if live:",
     "        if False:"),
    (CAUGHT, "manifest.py", "the no-visible-commit fetch guard is switched off",
     "        if not current:",
     "        if False:"),
    (CAUGHT, "manifest.py", "the scripts-on refusal is switched off",
     '            elif scripts == "on":',
     '            elif False:'),
    (CAUGHT, "manifest.py", "the claim-versus-bytes comparison is switched off",
     "                elif claim_sha != got:",
     "                elif False:"),
    (CAUGHT, "manifest.py", "the missing-capture-block refusal is switched off",
     '        if doc.get("schema") == SCHEMA and not cap:',
     "        if False:"),
    (CAUGHT, "manifest.py", "record's unclaimed-frame gate is switched off",
     "    if replaced or unclaimed or uncertified:",
     "    if False:"),
    (CAUGHT, "manifest.py", "the empty-directory refusal returns no problem",
     '        return [f"{directory} holds no screenshots and no manifest."]',
     "        return []"),
    (CAUGHT, "manifest.py", "the Makefile pin accepts every Makefile",
     "def verify_clause_problems(text):",
     "def verify_clause_problems(text):\n    return []  # every Makefile reads as wired"),
    (CAUGHT, "manifest.py", "every capture receipt is made invisible",
     "def read_receipt(path, fixture):",
     "def read_receipt(path, fixture):\n    return None  # no run ever photographed anything"),
    (CAUGHT, "manifest.py", "strictly_behind can never prove a frame stands behind its page",
     "def strictly_behind(older, newer, run=git):",
     "def strictly_behind(older, newer, run=git):\n    return False"),
    (CAUGHT, "manifest.py", "a verified claim's commit is dropped before it dates the frame",
     "                else:\n                    claim_rev = claim_head",
     '                else:\n                    claim_rev = ""  # the claim is kept but cannot date'),
    (CAUGHT, "manifest.py", "the drive harness answers every rule with no problems, running nothing",
     "    def drive(setup=None, run=None, pages=DRIVER, verb=\"check\"):",
     "    def drive(setup=None, run=None, pages=DRIVER, verb=\"check\"):\n        return []"),
    (CAUGHT, "manifest.py", "says() stops looking for the sentence it is named for",
     "    def says(name, problems):\n"
     '        return None if any(name in p for p in problems) else f"no problem named {name!r}"',
     "    def says(name, problems):\n        return None  # every tree says whatever it is asked for"),

    # ---- the harness's own machinery, caught ----
    # `drive()` and `says()` returning nothing are CAUGHT because most rules read their absence as
    # a complaint -- measured, not assumed: `drive-hollow` printed FAILED over the says rules, and
    # `says-hollow` printed FAILED over the rules that expect a refusal and would otherwise be
    # vacuous. A hollow that CRASHES (`drive` returning None) is INVALID by design: it proves
    # nothing, so it is not a row.

    # ---- honest edits: these must stay green, or the guard is matching text rather than behaviour,
    #      and a guard that is a tax gets switched off ----
    (GREEN, "manifest.py", "a comment is added above the rule helper",
     "    def rule(name, complaint):",
     "    # The one place a rule is both counted and reported.\n    def rule(name, complaint):"),
    (GREEN, "manifest.py", "the reach pin's operands are swapped",
     "    if len(ran) != EXPECTED:",
     "    if EXPECTED != len(ran):"),
    (GREEN, "manifest.py", "the identity sentence is reworded",
     'f"{name} is not the file this manifest recorded ({got[:12]}... on "',
     'f"{name} is not the file the manifest recorded ({got[:12]}... on "'),
    (GREEN, "manifest.py", "a comment is added inside check()",
     '    """Every way a committed photograph can stop being evidence, as a list of sentences."""',
     '    """Every way a committed photograph can stop being evidence, as a list of sentences."""\n'
     "    # Each sentence below names the repair, because an operator reads it, not a robot."),

    # ---- the floors, listed rather than deleted ----
    # Each of these seven patches was measured green WITH the patch live: the guard prints
    # `54 rules hold` with the named clause hollow. The capture suite's rule helper, reach pin and
    # verdict arm were once the same three floors; #323 turned its helper and verdict into CANARY
    # controls and moved the rows to CAUGHT. manifest.py has no canary yet, so these rows stay
    # HOLE, and each class names the shape that would close it. A floor nobody re-measures is
    # indistinguishable from a floor that was never there.
    (HOLE, "manifest.py", "the rule helper's reporting arm is switched off",
     "    def rule(name, complaint):\n        ran.append(name)\n        if complaint:",
     "    def rule(name, complaint):\n        ran.append(name)\n        if False:"),
    (HOLE, "manifest.py", "the reach pin is switched off",
     "    if len(ran) != EXPECTED:",
     "    if False:"),
    (HOLE, "manifest.py", "the verdict's problem list is switched off",
     '    if bad:\n        print("capture manifest self-test FAILED:")',
     '    if False:\n        print("capture manifest self-test FAILED:")'),
    (HOLE, "manifest.py", "the box's ROOT restore is switched off",
     "        finally:\n            ROOT = saved",
     "        finally:\n            pass"),
    (HOLE, "manifest.py", "the committed-PNGs-without-manifest arm is switched off and the "
                          "empty-directory sentence still answers",
     "    if doc is None:\n        if on_disk:",
     "    if doc is None:\n        if False:"),
    (HOLE, "manifest.py", "the missing-PNG-file arm is switched off",
     "        if not os.path.exists(path):",
     "        if False:"),
    (HOLE, "manifest.py", "the missing-source arm is switched off",
     "        missing = [s for s in sources if not os.path.exists(os.path.join(ROOT, s))]\n"
     "        if missing:\n"
     '            problems.append(f"{page}\'s manifest names {missing[0]}, which does not exist. '
     'A page "',
     "        missing = [s for s in sources if not os.path.exists(os.path.join(ROOT, s))]\n"
     "        if False:\n"
     '            problems.append(f"{page}\'s manifest names {missing[0]}, which does not exist. '
     'A page "'),
]

# Every HOLE row must name a class; every class must be described below. A row that falls back to
# its own name reports as an undocumented hole on purpose: the class text is the finding, and a
# row-level name is a patch description.
HOLE_CLASS = {
    "the rule helper's reporting arm is switched off":
        "the rule helper's reporting arm is unguarded",
    "the reach pin is switched off":
        "the reach pin is one switchable `if`",
    "the verdict's problem list is switched off":
        "the verdict's final decision is unguarded",
    "the box's ROOT restore is switched off":
        "the harness's ROOT restore is switchable",
    "the committed-PNGs-without-manifest arm is switched off and the empty-directory sentence "
    "still answers":
        "the two no-manifest refusals are told apart by no rule",
    "the missing-PNG-file arm is switched off":
        "the missing-PNG-file arm is unchecked",
    "the missing-source arm is switched off":
        "the missing-source arm is unchecked",
}

DOCUMENTED_HOLES = {
    "the verdict's final decision is unguarded":
        "closes the way #323 closed pages.py's (D-0072): a planted complaint pushed through a "
        "shadow list by a canary rule, so an emptied verdict is a failed run rather than a quiet "
        "one. Until then this row re-measures it on every run.",
    "the rule helper's reporting arm is unguarded":
        "closes with the same canary pair: give rule() the ran_to/bad_to shadow arguments pages.py "
        "has and assert the pair's counts are [1, 0]; the row moves to CAUGHT with it.",
    "the reach pin is one switchable `if`":
        "closes by registering the rules as pages.py does, so the pin compares two counts the run "
        "produces instead of one count and a literal; switching the pin off then strands the "
        "registration witness.",
    "the harness's ROOT restore is switchable":
        "closes with a rule that asserts ROOT still names this checkout after the first drive() -- "
        "the restore is observable from the next rule's box.",
    "the two no-manifest refusals are told apart by no rule":
        "closes by making the two rules seek sentences only their own arm speaks: the PNG set "
        "seeks 'committed screenshot(s)' (both sentences contain 'no manifest' today, measured) and "
        "the empty directory keeps 'no screenshots'. The row moves to CAUGHT when the seeks "
        "discriminate.",
    "the missing-PNG-file arm is unchecked":
        "closes with a rule that drives a manifest entry naming an uncommitted PNG and seeks "
        "'no such file is committed' -- the arm exists in check() and no rule reaches it.",
    "the missing-source arm is unchecked":
        "closes with a rule that drives a manifest entry naming a deleted source and seeks "
        "'which does not exist' -- the arm exists in check() and no rule reaches it.",
}

# The suite's own count pin, on the pattern of the two siblings: a row deleted from ROWS makes the
# tally smaller while every remaining row still reports its expected verdict, so the run would print
# a confident verdict over a guard that lost a test. 31 at first run, every expectation measured
# against this head (20 caught, 4 honest edits green, 7 holes).
EXPECTED_ROWS = 31


def shape_problems():
    """Static faults in this file, checked before a single child is started.

    The sibling suites each caught one of these by hand on the day they were written -- a repeated
    `what` making two rows answer to one class, a HOLE_CLASS key no row names, a target module that
    is not in the box. Same argument as theirs: none of it can be found by reading, so it is a
    battery.
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

    Same reason as the capture suite's copy of this function: manifest.py imports its siblings off
    its own directory, and a box missing one reports every earlier "catch" as a catch when it was a
    ModuleNotFoundError.
    """
    return sorted(p for p in os.listdir(CAPTURE) if p.endswith(".py"))


def template_files():
    """The ``.tmpl`` files the census rule reads, as paths relative to ``TEMPLATES``."""
    found = []
    for base, _dirs, files in os.walk(TEMPLATES):
        found += [op.relpath(op.join(base, f), TEMPLATES) for f in files if f.endswith(".tmpl")]
    return sorted(found)


def sandbox():
    """A private copy of the capture tree plus the templates and the Makefile, in the layout ROOT
    expects.

    The Makefile is not decoration and was not guessed in: manifest.py reads ``ROOT/Makefile`` at
    import for its own pin rule, and a box without it fails the pre-flight with "the Makefile could
    not be read" -- a baseline the guard could never meet, which would make every row a false
    catch. The Makefile is copied, so the pin rule reads a real one; the row engine never patches
    it (rows patch capture modules only, which shape_problems enforces).
    """
    box = tempfile.mkdtemp(prefix="plock-manifest-attacks-")
    dst = op.join(box, "testing", "capture")
    os.makedirs(dst)
    for name in capture_modules():
        shutil.copyfile(op.join(CAPTURE, name), op.join(dst, name))
    tdst = op.join(box, "internal", "web", "templates")
    os.makedirs(tdst)
    for rel in template_files():
        os.makedirs(op.join(tdst, op.dirname(rel)), exist_ok=True)
        shutil.copyfile(op.join(TEMPLATES, rel), op.join(tdst, rel))
    shutil.copyfile(op.join(ROOT, "Makefile"), op.join(box, "Makefile"))
    return box


def box_files(box):
    """Every file in the box, relative. The baseline's cleanliness is asserted against this."""
    found = []
    for base, _dirs, files in os.walk(box):
        found += [op.relpath(op.join(base, f), box) for f in files]
    return set(found)


# Per-row ceiling. The measured average row here is ~0.15 s; 60 s is several hundred times the work
# a row does. Several rows edit loop control, so a patched self-test that hangs is reachable.
TIMEOUT = 60


def run(box):
    """Run the manifest self-test in `box` and classify what came back. Never raises.

    ``silent`` is its own state: a self-test that exits 0 without printing its summary has reported
    nothing, which is not the same as having reported success.
    """
    try:
        proc = subprocess.run([PYTHON, "-B", op.join(box, "testing", "capture", "manifest.py"),
                               "--self-test"], capture_output=True, text=True, encoding="utf-8",
                              cwd=box, timeout=TIMEOUT)
    except subprocess.TimeoutExpired:
        return "invalid", "", (f"the self-test did not finish in {TIMEOUT}s -- a hang, not a "
                               f"broken patch")
    except (OSError, subprocess.SubprocessError) as exc:
        return "invalid", "", f"the sandboxed self-test could not be run: {exc}"
    out, err = proc.stdout.strip(), proc.stderr
    if proc.returncode not in (0, 1) or "Traceback" in err or "SyntaxError" in err:
        return "invalid", out, err
    if proc.returncode == 0 and out and GREEN_SUMMARY.match(out.splitlines()[-1]):
        return "green", out, err
    if proc.returncode == 0:
        return "silent", out, err
    if proc.returncode == 1 and FAILED_SUMMARY.search(out):
        return "caught", out, err
    return "invalid", out, err


def tree_state():
    """Digests of the files this suite reads. A run that changed one has broken its own rule."""
    state = {}
    for name in capture_modules():
        path = op.join(CAPTURE, name)
        state[name] = hashlib.sha256(open(path, "rb").read()).hexdigest()
    for rel in template_files():
        state[rel] = hashlib.sha256(open(op.join(TEMPLATES, rel), "rb").read()).hexdigest()
    state["Makefile"] = hashlib.sha256(open(op.join(ROOT, "Makefile"), "rb").read()).hexdigest()
    return state


def verify_recipe(text):
    """The `verify:` target's recipe lines, and only those.

    Searched in the recipe rather than the whole file, for the siblings' reason: a mention in the
    prose above the target is not a step, and the sentence explaining a step must not be able to
    satisfy the assertion the step is supposed to satisfy.
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
    names a file under `scripts/`, so no mutant can see a deleted step. Both siblings answer this
    the same way, and this suite needs their addition: it is called from TWO steps that witness
    each other -- the gate step runs it with no argument, the empty-filter loop names it in the
    `for runner` list -- so deleting either step makes the surviving assertion refuse.
    """
    try:
        with open(op.join(ROOT, "Makefile"), encoding="utf-8") as fh:
            text = fh.read()
    except OSError as exc:
        return [f"the Makefile could not be read ({exc}), so nothing here can promise `make verify` "
                f"still runs this suite"]
    recipe = verify_recipe(text)
    problems = []
    if not re.search(r"python3 scripts/attack-manifest-self-test\.py\b", recipe):
        problems.append("`make verify` no longer runs this suite. "
                        f"{EXPECTED_ROWS} attacks nobody invokes are a list of intentions, not a "
                        "gate; if the step was deleted because this file's verdict became "
                        "inconvenient, the finding is in the verdict")
    if not re.search(r"for runner in [^\n]*attack-manifest-self-test\.py", recipe):
        problems.append("`make verify` no longer passes this suite through the empty-filter loop, so "
                        "a filter matching nothing could exit 0 and the run would report a "
                        "confident verdict over nothing attempted. This loop is also the second "
                        "witness of the gate step above: delete it and that step goes red")
    return problems


def main():
    # `None` means no argument was given; `""` means one was given and it is empty -- the siblings'
    # lesson kept in shape: collapsing the two makes `""` mean "no filter", so the Makefile step
    # that passes "$FILTER" with FILTER unset runs the whole gate and reports green while proving
    # something else. An empty filter here matches NOTHING.
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
                  "to meet, so none is run. The box holds testing/capture/*.py, "
                  "internal/web/templates/*.tmpl and Makefile; if the self-test has started reading "
                  "anything else, the box has to hold that too.")
            return 2
        written = box_files(box) - copied
        if written:
            # The summary line's own promise -- "nothing written outside a temp box" -- checked on
            # the disk rather than believed, as in the sibling: a driver that writes into the tree
            # it was copied into writes into the repository when it is not copied.
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
