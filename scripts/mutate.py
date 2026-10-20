#!/usr/bin/env python3
"""Mutation battery: revert one safety property at a time and insist the suite notices.

A green suite says the code passes the tests. It does not say the tests would fail
if the code were wrong, and for this product that difference is the whole point -- a
lockout that silently stops locking out is exactly the failure a test can bless. So
each entry in scripts/mutations/*.json puts one known bug back into the source and
the run checks that something goes red.

Usage:

    scripts/mutate.py                      # every set under scripts/mutations/
    scripts/mutate.py scripts/mutations/schedule.json
    scripts/mutate.py --only S1,W3         # a subset, for working on one finding
    scripts/mutate.py --self-test           # the verdict rules only: no Go, no battery

Five ways a battery like this can report success while testing nothing, all five
handled here:

  - a mutation whose pattern is not in the file tests nothing and looks like a pass,
    so the pattern's occurrence count is checked against the entry before anything
    runs, and a mismatch is INVALID;
  - a mutation that breaks the BUILD makes every later mutant "fail", so the run
    reports everything caught while proving nothing -- a build failure is therefore
    INVALID, never CAUGHT;
  - a baseline that is already red makes every mutant look caught for the wrong
    reason, so the run refuses to start unless the unmutated tree is green;
  - a pattern that matches somewhere other than where it was meant to land mutates
    the wrong line and can pass for the wrong reason, so every entry states how many
    occurrences it expects and the run checks the count;
  - a package that STOPS RUNNING on the test timeout exits non-zero like any caught
    mutant, so it reads as a guard holding when nothing asserted anything -- a stall
    with no failing test is therefore INVALID too, and a stall after real failures
    says so on its line (see verdict).

The source is restored between every mutant, from the copy taken at startup. That is
not tidiness: restoring only the file just mutated lets mutant N run on top of
mutant N-1, and then neither the catches nor the survivors mean anything.

The run happens in a throwaway copy of the repository, never in the working tree.
Two reasons, both learned the hard way. An interrupted run used to leave the tree
mutated. The second is subtler: the runner snapshots only the files it mutates, so
everything else in the package -- the tests, above all -- stays live. Edit a test
file while a battery is running and every mutant after that moment fails to compile
and is reported INVALID, which reads like a broken mutant set and is really a broken
workflow. One run reported 53 INVALID out of 55 that way. Copy the tree and there is
nothing to interfere with, and nothing to dirty.

An entry may be marked "equivalent" when the mutation cannot be distinguished from
correct behaviour by any input -- a probe wider than the proof needs, say. Those are
reported separately and never counted as a pass, and each carries a note saying why.
A survivor that is not marked is a real gap in the suite: the fix is a better test,
not a new note.

Exit status is 0 only when nothing survived and nothing failed to apply.
"""

import argparse
import glob
import inspect
import json
import os
import re
import shutil
import subprocess
import tempfile
import sys

# The set format is validated in one place, shared with the pre-flight, so a set the
# gate accepts can never be a set the battery refuses to load.
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from mutset import load  # noqa: E402

SRC = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
# The Makefile takes GO ?= go; honouring it here keeps `make mutate GO=...` testing
# the compiler it names rather than whatever else is on PATH.
GO = os.environ.get("GO", "go")
ROOT = SRC  # main() points this at the throwaway copy


def run_suite(pkg):
    # A package field may name more than one package, space-separated. A mutant in a
    # shared helper is often pinned by tests in the helper's own package AND by the
    # command-level test that reproduces what the user saw, and only running both proves
    # which. Passing the field through unsplit handed go a single argument containing a
    # space, which fails to resolve -- and a suite that cannot run looks exactly like a
    # suite where every test failed, so every mutant of that helper would read as caught.
    return subprocess.run(
        [GO, "test", *pkg.split(), "-count=1", "-timeout", "900s"],
        capture_output=True,
        text=True,
        cwd=ROOT,
    )


def failing_tests(out):
    names = []
    for line in out.splitlines():
        line = line.strip()
        if line.startswith("--- FAIL:"):
            names.append(line.split("FAIL:")[1].split()[0])
    return names


def run_set(path, label):
    doc = load(path)
    pkg, mutants = doc.get("package"), doc["mutants"]
    # A mutant's tests live wherever its behaviour is observable, which is not always
    # the set's default package: a cmd/plock mutant is invisible to the platform suite.
    # Without this, a mutant whose tests never ran was reported as SURVIVED and looked
    # like a gap in the tests rather than a gap in the runner.
    pkgs = sorted({m.get("package", pkg) for m in mutants})
    files = sorted({m["file"] for m in mutants})
    for f in files:
        if not os.path.isfile(os.path.join(ROOT, f)):
            sys.exit(f"{label}: {f} is not in the tree -- the set is stale")
    orig = {f: open(os.path.join(ROOT, f)).read() for f in files}

    def restore():
        for f in files:
            with open(os.path.join(ROOT, f), "w") as fh:
                fh.write(orig[f])

    print(f"\n{label}  ({len(mutants)} mutants, {', '.join(pkgs)})")

    for p in pkgs:
        base = run_suite(p)
        if base.returncode != 0:
            restore()
            print(f"baseline is RED in {p} -- refusing to run, every mutant would look caught:\n"
                  + base.stdout + base.stderr)
            return None
        print(f"baseline green in {p}")

    tally = {"caught": 0, "survived": 0, "invalid": 0, "equivalent": 0}

    for m in mutants:
        src = m["file"]
        mpkg = m.get("package", pkg)
        label = f"{m['id']}  {m['old'][:52]!r}"
        want = m.get("count", 1)
        hits = orig[src].count(m["old"])
        if hits != want:
            print(f"NOT APPLIED  {label} -- {hits} occurrences, expected {want}")
            tally["invalid"] += 1
            continue

        restore()
        mutated = orig[src].replace(m["old"], m["new"])
        if mutated == orig[src]:
            print(f"NOT APPLIED  {label} -- replacement is identical")
            tally["invalid"] += 1
            continue

        with open(os.path.join(ROOT, src), "w") as fh:
            fh.write(mutated)
        r = run_suite(mpkg)
        out = r.stdout + r.stderr

        note = equivalence_note(m)
        kind, detail = verdict(out, r.returncode, note)
        print(f"{VERDICT_TAG[kind]}{label}{detail}")
        tally[kind] += 1

    restore()
    print(f"{tally['caught']} caught, {tally['survived']} survived, "
          f"{tally['equivalent']} equivalent, {tally['invalid']} invalid, "
          f"{len(mutants)} total")
    return tally


# Everything below decides a verdict, and it is kept together, below the loop that
# calls it, for one reason: the rules are the thing that must not drift, so they get
# one neighbourhood and one self-test rather than being scattered next to the print
# they govern.

# What `go test -timeout` prints when the package stops running. The exact prefix
# matters: it is the only thing that distinguishes "the suite rejected this mutant"
# from "the suite never finished", and the two must not share a verdict.
STALL = "panic: test timed out after"

# The report's column labels, spelled once so verdict() returns a word and the
# printer emits the alignment it always emitted.
VERDICT_TAG = {
    "caught": "CAUGHT     ",
    "survived": "SURVIVED   ",
    "invalid": "INVALID    ",
    "equivalent": "EQUIVALENT ",
}


def equivalence_note(m):
    """The equivalence note for a set entry, or None when the entry is not marked.

    None is the ONLY value that means "not marked", so a marked entry yields a string
    even when its note is missing, empty, or written as JSON null. Falling through to
    None would report a green run as SURVIVED, which reads as a gap in the tests rather
    than as the typo in the set that it is.

    `m.get("note", "")` is not enough for that: it defaults only when the key is
    ABSENT, and `"equivalent": true, "note": null` arrives as None with the key present.
    """
    if not m.get("equivalent"):
        return None
    return m.get("note") or ""


def verdict(out, rc, note=None):
    """Decide one mutant's verdict from the suite's output. Returns (kind, detail).

    Every way a run can mean nothing is decided here, in one order, so a new failure
    mode has one place to be added and the self-test one place to cover.

    A build failure is INVALID, never CAUGHT: nothing ran. A package that stops
    running on the test timeout is the same shape one step later -- if no test
    reported a failure before it stopped, nothing was asserted either, so it is
    INVALID on the same grounds. When tests did fail first the catch is real, but
    the line says the package also stopped, because such a run pays the whole
    -timeout budget and the log must not read like an ordinary catch.

    A non-zero exit that is neither of those is still a catch even when no test is
    named: a mutant that panics the suite was noticed by it. Only the timeout is a
    way for a package to stop without having noticed anything.
    """
    if "[build failed]" in out or "[setup failed]" in out:
        return "invalid", " (did not compile, so nothing was tested)"
    if rc == 0:
        if note is not None:
            return "equivalent", f" -- {note or 'no note'}"
        return "survived", ""
    names = failing_tests(out)
    if STALL in out:
        if not names:
            return ("invalid", " (the package stopped running on the test timeout "
                               "with no test reporting a failure, so nothing was "
                               "asserted)")
        return ("caught", f" by {', '.join(names[:2])} -- the package then stopped "
                          "running on the test timeout")
    return "caught", f" by {', '.join(names[:2]) or 'a test'}"


def select_sets(paths, wanted, work):
    """Trim every set holding a wanted id down to those ids, one trimmed file per set.

    Returns ``(selected, missing)``: ``selected`` is ``[(path, label)]`` in the order the
    sets were given, ``missing`` the requested ids no set holds.

    One file per matching set, and no early ``break``. The first version wrote a single
    ``subset.json`` and stopped at the first set that matched, so ``--only W1,W48`` across
    platform.json and web.json ran one mutant, printed ``TOTAL  1 caught, 0 survived``,
    and exited 0: a partial run reporting the whole answer, which is the one error this
    runner exists not to make (#118). The label carries only the ids that set contributes,
    so two sets never both claim the full request.
    """
    selected, missing, n = [], set(wanted), 0
    for path in paths:
        doc = load(path)
        keep = [m for m in doc["mutants"] if m["id"] in wanted]
        if not keep:
            continue
        for m in keep:
            missing.discard(m["id"])
        doc["mutants"] = keep
        n += 1
        tmp = os.path.join(work, f"subset-{n}.json")
        with open(tmp, "w", encoding="utf-8") as fh:
            json.dump(doc, fh, indent=1)
        selected.append((tmp, f"{os.path.basename(path)} "
                              f"[{','.join(m['id'] for m in keep)}]"))
    return selected, sorted(missing)


def rule(rules, bad, name, problems):
    """Count one self-test rule, and report whatever it found.

    The tally and the verdict leave the same call on purpose. Counting at the call site
    meant ``if False:`` in front of a check kept the pinned number exactly where it was --
    the tally counted calls, not assertions, which is the hole attack118.py measured. With
    the increment in here, the mechanical gut drops the count and the pinned number shouts.

    What the count still cannot see is a call whose arguments were replaced by ``True``:
    that is a deliberate edit, and only running the attack suite catches it. The count is a
    deletion tripwire, not a coverage claim.
    """
    rules.append(name)
    for problem in problems:
        bad.append(f"{name}: {problem}")


def control(rules, bad, name, problems, why):
    """Count one control: a checker that has to refuse something on purpose.

    ``problems`` is what the checker had to say about a deliberately wrong answer. Empty
    means it did not notice, which is the failure -- a control nobody can turn off is a
    control that proves nothing about the thing it claims to test.
    """
    rule(rules, bad, name, [] if problems else [f"it said nothing, {why}"])


# The --only cases, as data: what was asked for, which fixture pair to ask it of, what a
# correct answer selects, and what it reports missing. The rules are generated from this
# tuple, so dropping a case drops the rule count with it.
SELECT_CASES = (
    ("A1,B1 across two sets", "ab", {"A1", "B1"},
     [("alpha.json", ["A1"]), ("beta.json", ["B1"])], [],
     "the ticket: a request spanning two sets has to run both"),
    ("B1 in one set", "ab", {"B1"}, [("beta.json", ["B1"])], [],
     "the other half of the fix: not running every set regardless, since a --only as slow "
     "as a full battery stops being used"),
    ("A1,Z9 with one unknown id", "ab", {"A1", "Z9"}, [("alpha.json", ["A1"])], ["Z9"],
     "an id in no set has to be named, not left out of a run that looks like it complied"),
    ("DUP held by two sets", "dup", {"DUP"},
     [("dupa.json", ["DUP"]), ("dupb.json", ["DUP"])], [],
     "an id two sets both hold runs in both, and neither set claims the other's"),
    ("Z9 in no set", "ab", {"Z9"}, [], ["Z9"],
     "the refusal: an empty selection is what makes main's branch reachable"),
)


def scope(paths):
    """Where an id was looked for, in words a reader can act on.

    Naming the whole directory is wrong whenever positional set arguments narrow the run:
    the id may well live in a file this run was never pointed at, and "not in any set" is
    the sentence that makes someone delete a working request.
    """
    if not paths:
        return "any set under scripts/mutations/"
    return "the sets this run read (" + ", ".join(os.path.basename(p) for p in paths) + ")"


def selection_problems(sel, miss, want, want_missing):
    """What one ``--only`` selection got wrong, as sentences. Empty means it did what was asked.

    ``sel`` is what the selection agreed to run (``[(path, label)]``) and ``miss`` the ids
    no set holds; ``want`` is ``[(set basename, [ids that set must run]), ...]`` in the
    order the sets were given, and ``want_missing`` the ids that should be reported.

    It is a pure check on its arguments rather than a call into ``select_sets``, so the
    self-test can hand it a selection that is wrong on purpose -- a checker nobody can
    falsify is a checker that can be hollowed. The label is checked whole, not just its
    filename: the ticket's symptom line was a label naming every requested id while the set
    ran one of them, so "the right sets" is only half of what a reader reads. Checking the
    label covers which sets ran as well, which is why there is no separate filename
    comparison -- a second one would be redundant, and a redundant comparison is a control
    that proves nothing about the comparison it claims to test.
    """
    problems = []
    got_labels = [lab for _, lab in sel]
    want_labels = [f"{name} [{','.join(ids)}]" for name, ids in want]
    if got_labels != want_labels:
        problems.append(f"selected {got_labels}, want {want_labels}")
    if miss != want_missing:
        problems.append(f"reported {miss} as unmatched, want {want_missing}")
    for (path, lab), (_, ids) in zip(sel, want):
        got = [m["id"] for m in load(path)["mutants"]]
        if got != ids:
            problems.append(f"{lab} holds {got}, want only {ids}")
    files = [p for p, _ in sel]
    if len(set(files)) != len(files) or not all(os.path.isfile(p) for p in files):
        problems.append(f"the trimmed files are not one distinct file per set: {files}")
    return problems


# What the two-set fixture in the self-test owes: one mutant from each set, in order.
WANT_AB = [("alpha.json", ["A1"]), ("beta.json", ["B1"])]

# The comparisons a --only selection is decided by, each paired with a selection that is
# wrong about exactly that thing. Both halves come from this tuple, so emptying it -- the
# cheapest way to make a pin vacuous -- drops the rule count along with the coverage.
CHECKER_CONTROLS = (
    ("if got_labels != want_labels:",
     lambda real: ([(real[0][0], "alpha.json [A1,B1]"), real[1]], [], WANT_AB, [])),
    ("if miss != want_missing:",
     lambda real: (real, [], WANT_AB, ["Z9"])),
)

# What main's --only block has to say to be honest. The self-test runs no battery, so the
# flag's shape is audited as text -- the way check-mutations.py pins its own summary line,
# for the same reason: a pinned call buys nothing if the next line throws its result away,
# and throwing the result away is exactly where #118 lived.
ONLY_CALL = "sets, missing = select_sets(sets, wanted, work)"
ONLY_BLOCK_LINES = (
    # `if args.only:` is falsy for --only "", which then falls through to the else and runs
    # the ENTIRE battery behind a flag that asked for nothing.
    "if args.only is not None:",
    "if not wanted:",
    ONLY_CALL,
    # Nothing matched: refuse. Without this the run prints `TOTAL 0 caught, 0 survived`.
    "if not sets:",
)
# Each control hands the auditor a main with that one break in it, and has to be caught.
ONLY_BLOCK_CONTROLS = (
    ("throws the selection away one line after the pinned call, which is #118 itself",
     lambda s: s.replace(ONLY_CALL, ONLY_CALL + "\n            sets = sets[:1]", 1)),
    ("swallows the ids that matched nothing",
     lambda s: s.replace(ONLY_CALL, ONLY_CALL + "\n            missing = []", 1)),
    ("runs the whole battery when --only names no ids",
     lambda s: s.replace("if args.only is not None:", "if args.only:", 1)),
    ("reports an empty success when nothing matched",
     lambda s: s.replace("            if not sets:", "            if False:", 1)),
    ("exits 0 on a request it could only partly answer",
     lambda s: s.replace("        return 1\n", "        return 0\n", 1)),
)


def only_block_problems(src):
    """What ``main``'s ``--only`` handling gets wrong, as sentences. Empty means it is honest.

    Takes the source as an argument rather than reading it, so the self-test can hand it a
    ``main`` with each break in it on purpose. A checker nobody can falsify is a checker
    that can be hollowed.
    """
    _before, found, rest = src.partition("if args.only")
    if not found:
        return ["main has no `if args.only` block, so the flag's whole shape is gone and "
                "nothing below applies"]
    # partition, not split: the marker has to stay in the block, because the block is then
    # checked for the very lines that open it.
    block = "if args.only" + rest.split("\n        else:")[0]
    problems = [f"the --only block no longer contains {line!r}"
                for line in ONLY_BLOCK_LINES if line not in block]
    # The pinned call is worth nothing if a later line re-assigns what it returned. This is
    # the shape a substring pin cannot see, and it is the shape of the original bug.
    for stmt in re.findall(r"^[ \t]*(?:sets|missing)[ \t]*=[^=].*$", block, re.M):
        problems.append(f"the --only block reassigns outside the pinned call: "
                        f"{stmt.strip()!r} -- a dropped set or a swallowed id hides there")
    if "if missing:" not in src:
        problems.append("main no longer reports a --only request it could only partly "
                        "answer")
    elif "return 1" not in src.split("if missing:")[-1]:
        problems.append("main's `if missing:` branch no longer exits non-zero, so a "
                        "scripted `mutate.py --only ... && ...` still proceeds")
    return problems


# How many verdict rules the self-test is expected to run, pinned for the same reason the
# selection count is. Counted off the run rather than guessed: the number is a claim about
# how much was tested.
EXPECTED_VERDICT_RULES = 22

# How many selection rules the self-test is expected to run, counted off the list the pin
# prints rather than off arithmetic: five --only cases and one "does this case assert
# anything" per case, the fixture's own shape, two controls at the selection level, a pin
# and a control per comparison in CHECKER_CONTROLS, the audit of main's --only block, and
# one control per entry in ONLY_BLOCK_CONTROLS. Pinned for the same reason the verdict count
# is -- a deleted rule is invisible to the rules that survive.
EXPECTED_SELECT_RULES = 23


def self_test():
    """Check the verdict rules against captured `go test` output.

    The cases are excerpts of real runs (main plus W25/W23 in internal/web, and a
    deliberate compile error), trimmed to the lines a verdict is decided from, so a
    rule cannot be written against an output shape go does not produce. There is no
    Go here on purpose: this must run in CI where a battery would cost an hour.
    """
    green = "ok  \tgithub.com/vicowa-botworks/parental-lockout/internal/web\t13.202s\n"
    caught = ("--- FAIL: TestAMachineWithNoCredentialSaysThatRatherThanWrongPassword (0.00s)\n"
              "FAIL\nFAIL\tgithub.com/vicowa-botworks/parental-lockout/internal/web\t12.9s\n"
              "exit status 1\n")
    build = ("# github.com/vicowa-botworks/parental-lockout/internal/web "
             "[github.com/vicowa-botworks/parental-lockout/internal.web.test]\n"
             "internal/web/handlers.go:210:23: undefined: notDefinedAnywhere\n"
             "FAIL\tgithub.com/vicowa-botworks/parental-lockout/internal/web\t0.412s\n"
             "[build failed]\n")
    stalled = ("panic: test timed out after 15m0s\n"
               "\trunning tests:\n"
               "\t\tTestTwoOverlappingMutationsDoNotLoseOne (15m0s)\n\n"
               "goroutine 10 [running]:\nFAIL\tgithub.com/vicowa-botworks/parental-lockout/internal/web\t900.412s\n")
    stalled_after_failures = ("--- FAIL: TestTheDashboardOverrideFormGrantsAsRendered (0.00s)\n"
                              "--- FAIL: TestTheDashboardWindowButtonEndsAtTheWindow (0.00s)\n"
                              "panic: test timed out after 40s\n"
                              "\trunning tests:\n"
                              "\t\tTestTwoOverlappingMutationsDoNotLoseOne (40s)\n"
                              "FAIL\tgithub.com/vicowa-botworks/parental-lockout/internal/web\t40.2s\n")
    # A link failure is go's other way to produce a non-zero run with no test in it:
    # `[setup failed]` instead of `[build failed]`. Both mean nothing ran, and the rule
    # tests for two markers, so a fixture for one of them pins half the guard.
    setup = ("# github.com/vicowa-botworks/parental-lockout/internal/web\n"
             "internal/web/audit.go:12:2: package plock/missing is not in std\n"
             "FAIL\tgithub.com/vicowa-botworks/parental-lockout/internal/web [setup failed]\n")
    # A panic that is not a timeout. This is the fixture that pins STALL's exactness:
    # loosening the prefix to "panic:" turns this run into a stall report for a hang that
    # does not exist, and the suite would still be green without it.
    plain_panic = ("panic: runtime error: index out of range [3] with length 2\n"
                   "FAIL\tgithub.com/vicowa-botworks/parental-lockout/internal/web\t4.2s\n")
    # Non-zero, no stall, no `--- FAIL:` line, and no panic either: the package noticed
    # something without naming what. This is the fallback row on its own, so it stays
    # covered even if the panic shape above stops being the way go reports one.
    unnamed = ("FAIL\tgithub.com/vicowa-botworks/parental-lockout/internal/web\t4.2s\n"
               "exit status 2\n")

    cases = [
        # (name, output, exit status, equivalent note, expected kind, must contain,
        #  optional: must NOT contain)
        ("a green suite survives", green, 0, None, "survived", ""),
        ("a failing test catches", caught, 1, None, "caught",
         "TestAMachineWithNoCredentialSaysThatRatherThanWrongPassword"),
        ("a build failure is not a catch", build, 1, None, "invalid",
         "did not compile"),
        ("a stall with no failing test is not a catch", stalled, 1, None, "invalid",
         "nothing was asserted"),
        ("a stall after failures is still a catch", stalled_after_failures, 1, None,
         "caught", "TestTheDashboardOverrideFormGrantsAsRendered"),
        ("but it says the package stopped", stalled_after_failures, 1, None, "caught",
         "stopped running on the test timeout"),
        ("an equivalent marked entry reports as such", green, 0, "probe is wider",
         "equivalent", "probe is wider"),
        ("a marked entry with no note still says so", green, 0, "", "equivalent",
         "no note"),
        # The control: the stall rule must not swallow a genuine catch.
        ("a stall does not outrun a real failure", stalled_after_failures, 1, None,
         "caught", ""),
        # The other direction of the same control: a panic that is not a timeout is a
        # mutant the suite noticed, and it stays a catch only while STALL is matched as
        # precisely as it is written.
        ("a panic that is not a timeout is still a catch", plain_panic, 2, None,
         "caught", "by a test", "timed out"),
        ("a setup failure is not a catch either", setup, 1, None, "invalid",
         "did not compile"),
        # The fallback row: caught with no test named. Without a case here the "a test"
        # wording could be deleted and nothing would notice.
        ("a non-zero run naming no test is still a catch", unnamed, 1, None,
         "caught", "by a test"),
        # A marked entry whose note is JSON null must reach verdict() as a string, or it
        # reports SURVIVED on a green run. equivalence_note() is what makes that true, and
        # this is the end-to-end half of the claim.
        ("a marked entry with a null note is still equivalent", green, 0,
         equivalence_note({"equivalent": True, "note": None}), "equivalent", "no note"),
    ]

    bad = []
    checked = 0
    rules = []
    for name, out, rc, note, want, needle, *rest in cases:
        checked += 1
        absent = rest[0] if rest else None
        kind, detail = verdict(out, rc, note)
        if kind != want:
            bad.append(f"{name}: got {kind!r}, want {want!r}")
        elif needle and needle not in detail:
            bad.append(f"{name}: detail {detail!r} does not mention {needle!r}")
        elif absent and absent in detail:
            bad.append(f"{name}: detail {detail!r} says {absent!r}, which this case "
                       f"exists to rule out")
        if kind not in VERDICT_TAG:
            bad.append(f"{name}: kind {kind!r} has no report label")

    # Which value counts as "marked" is a rule of its own, and it is the one a typo in a
    # set hits. Checked here rather than left to the case above, so each spelling is named.
    for name, entry, want in [
        ("unmarked", {"id": "X"}, None),
        ("marked with a note", {"equivalent": True, "note": "probe is wider"},
         "probe is wider"),
        ("marked with no note key", {"equivalent": True}, ""),
        ("marked with a null note", {"equivalent": True, "note": None}, ""),
        ("marked with an empty note", {"equivalent": True, "note": ""}, ""),
        ("marked by a prose proof", {"equivalent": "proved in a scratch file",
                                     "note": None}, ""),
        ("explicitly not marked", {"equivalent": False, "note": "stale"}, None),
    ]:
        checked += 1
        got = equivalence_note(entry)
        if got != want:
            bad.append(f"equivalence_note {name}: got {got!r}, want {want!r}")

    # The report is a fixed column, and "the tally is read from it" is the reason this
    # change claims the output is unchanged. Narrowing a label to "CAUGHT" keeps every
    # verdict assertion green and moves every line in the report, so the width is pinned
    # rather than trusted to the strings being left alone.
    checked += 1
    for kind, tag in sorted(VERDICT_TAG.items()):
        if len(tag) != 11:
            bad.append(f"VERDICT_TAG[{kind!r}] = {tag!r} is {len(tag)} wide, want 11 "
                       f"(the report's column)")

    # Every label must be reachable, or a verdict could print `None` mid-line.
    checked += 1
    for kind in ("caught", "survived", "invalid", "equivalent"):
        if kind not in VERDICT_TAG:
            bad.append(f"verdict kind {kind!r} has no report label")

    # Which mutants a partial run agrees to run. These are checked against real set files
    # written to a temp dir rather than a mock, because the bug (#118) was in which FILES
    # got written: one `subset.json` and a `break` meant a request spanning two sets ran
    # the first one and printed a TOTAL that read like the whole answer.
    with tempfile.TemporaryDirectory() as td:
        def write_set(name, ids):
            p = os.path.join(td, name)
            with open(p, "w", encoding="utf-8") as fh:
                json.dump({"package": "./internal/x/",
                           "mutants": [{"id": i, "file": "x.go", "old": "a", "new": "b"}
                                       for i in ids]}, fh)
            return p

        alpha = write_set("alpha.json", ["A1", "A2"])
        beta = write_set("beta.json", ["B1"])
        # An id is unique per SET file, not globally -- `W1` is three different mutants in
        # platform.json, schedule.json and web.json -- so one requested id can be owed by
        # several sets, and a "fix" that deduped by id would run one of three and call it
        # complete.
        dup_a = write_set("dupa.json", ["DUP"])
        dup_b = write_set("dupb.json", ["DUP", "OTHER"])
        work = os.path.join(td, "work")
        os.mkdir(work)

        fixtures = {"ab": [alpha, beta], "dup": [dup_a, dup_b]}
        for name, pair, wanted, want, want_missing, why in SELECT_CASES:
            got, miss = select_sets(fixtures[pair], wanted, work)
            rule(rules, bad, f"--only {name}",
                 selection_problems(got, miss, want, want_missing))
            # A case that expects no sets AND no missing ids can never fail, so it is not a
            # rule -- it is a counter that keeps the pinned number honest about nothing.
            rule(rules, bad, f"--only {name} asserts something",
                 [] if (want or want_missing)
                 else [f"expects nothing in either direction, so {why} is untested"])

        # The controls: a working checker has to refuse each of these. Both are generated
        # from what they check, so a checker that stops noticing is reported rather than
        # merely counted.
        real, _ = select_sets([alpha, beta], {"A1", "B1"}, work)
        rule(rules, bad, "the two-set fixture holds two sets",
             [] if len(real) == 2 else
             [f"select_sets answered {real} for a request spanning two sets"])
        control(rules, bad, "control: a dropped set",
                selection_problems(real[:1], [], WANT_AB, []) if len(real) == 2 else None,
                "so which sets ran is not being checked")
        # And one at the select_sets level. Everything above asserts on what select_sets
        # returned, so a loop that computed it and threw the answer away would keep every
        # rule green; this one feeds the checker select_sets' OWN short answer.
        one, one_miss = select_sets([alpha, beta], {"A1"}, work)
        control(rules, bad, "control: select_sets' own one-set answer",
                selection_problems(one, one_miss, WANT_AB, []),
                "so the rules above assert nothing about what select_sets returns")

        # The comparisons, pinned in the source and then fed the selection only they
        # reject. The pin alone is a substring test; the control alone cannot tell a
        # deleted comparison from one that never had anything to say. Both rules come from
        # the same tuple, so emptying it drops the count instead of quietly emptying the
        # coverage.
        try:
            checker = inspect.getsource(selection_problems)
        except OSError:
            checker = ""
            bad.append("could not read selection_problems' source, so neither comparison "
                       "in it is checked")
        for line, wrong in CHECKER_CONTROLS:
            rule(rules, bad, f"pin: {line}",
                 [] if line in checker else
                 [f"selection_problems no longer contains {line!r}, so a --only run can "
                  f"overstate it in silence"])
            control(rules, bad, f"control: {line}",
                    selection_problems(*wrong(real)) if len(real) == 2 else None,
                    f"so {line!r} is present and doing nothing")

        # main's --only block, audited as text: the bug was in main throwing away all but
        # the first set, and the self-test runs no battery, so the flag's shape is the only
        # thing CI can look at. Audited through a function that TAKES the source, so each
        # break below can be handed to it on purpose -- a pin that cannot be falsified is a
        # pin that one inserted line defeats, which is how round 1's call-site pin failed.
        try:
            src = inspect.getsource(main)
        except OSError:
            src = ""
            bad.append("could not read main's source, so the --only flag's shape is "
                       "unchecked")
        rule(rules, bad, "main's --only handling",
             only_block_problems(src) if src else ["main's source could not be read"])
        for name, patch in ONLY_BLOCK_CONTROLS:
            control(rules, bad, f"control: a main that {name}",
                    only_block_problems(patch(src)) if src else None,
                    "so nothing stops it")

    # The two helpers every rule above is counted through are themselves attack surface: a
    # `pass` where `rules.append(name)` is, or an `if False:` inside control(), keeps every
    # number intact while emptying the checks. So both are canaried through a throwaway
    # list. Deliberately NOT one of the counted rules, for the reason the count pin is not
    # either: a check that counts itself moves the number it exists to hold still.
    probe_rules, probe_bad = [], []
    rule(probe_rules, probe_bad, "canary", ["a problem"])
    control(probe_rules, probe_bad, "canary", [], "it said nothing")
    control(probe_rules, probe_bad, "canary", ["a problem"], "it said nothing")
    if len(probe_rules) != 3 or len(probe_bad) != 2:
        bad.append(f"the rule/control helpers are hollow: they counted {len(probe_rules)} "
                   f"and reported {len(probe_bad)}, want 3 and 2 -- a rule that cannot fail "
                   f"is not made honest by counting it")

    # Reported, not adjudicated: the reach check used to live here, and a bare `return`
    # inserted anywhere above it skipped every rule and still exited 0. What decides
    # pass or fail is self_test_verdict(), outside the code whose reach it measures.
    return checked, rules, bad


def self_test_verdict(report):
    """Turn what self_test() returned into an exit code.

    The reach check lives here rather than inside self_test() because a check cannot police
    the code that is allowed to skip it: a bare ``return`` in the middle of the self-test
    used to leave every rule unchecked and the process exiting 0, since the only thing that
    could notice the missing verdict was the line that never ran. An absent verdict is now
    its own failure, which is the only way to tell "nothing was checked" from "all of it
    held".

    The two counts are named apart in the summary because "N rules hold" said about two
    different mechanisms is how a summary starts meaning less than the reader assumes.
    """
    if not isinstance(report, tuple) or len(report) != 3:
        print("self-test FAILED:")
        print(f"  - it returned {report!r} instead of a verdict, so nothing was checked "
              f"(a `return` inside self_test skips every rule below it)")
        return 1
    checked, rules, bad = report
    # Each count is compared against a number that has to be edited on purpose: every rule
    # can pass perfectly while one of them is missing from the file. Neither pin is one of
    # the counted rules -- a pin that counts itself moves the number it exists to hold.
    if checked != EXPECTED_VERDICT_RULES:
        bad = list(bad) + [f"the self-test ran {checked} verdict rules, expected "
                           f"{EXPECTED_VERDICT_RULES}: a rule was deleted, or stopped "
                           f"counting"]
    if len(rules) != EXPECTED_SELECT_RULES:
        bad = list(bad) + [f"the self-test ran {len(rules)} selection rules, expected "
                           f"{EXPECTED_SELECT_RULES}: a rule was deleted, or stopped "
                           f"counting. Ran: {rules}"]
    if bad:
        print("self-test FAILED:")
        for b in bad:
            print(f"  - {b}")
        return 1
    print(f"self-test: {checked} verdict rules and {len(rules)} selection rules hold")
    return 0


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("sets", nargs="*", help="mutation JSON files")
    ap.add_argument("--only", metavar="IDS",
                    help="comma-separated mutant ids to run, e.g. S1,W3")
    ap.add_argument("--self-test", action="store_true",
                    help="check the verdict rules against captured go test output "
                         "and exit; needs no Go and runs no battery")
    args = ap.parse_args()

    # Before the Go probe and before any tree is copied: the rules being checked are
    # about interpreting output, so the check must run on a box with no toolchain
    # and must never be able to mutate anything.
    if args.self_test:
        return self_test_verdict(self_test())

    try:
        probe = subprocess.run([GO, "version"], capture_output=True)
    except FileNotFoundError:
        probe = None
    if probe is None or probe.returncode != 0:
        sys.exit(f"{GO} is not runnable; put it on PATH or set GO= (the Makefile uses GO ?= go)")

    sets = [os.path.abspath(p) for p in
            (args.sets or sorted(glob.glob(os.path.join(SRC, "scripts/mutations/*.json"))))]
    if not sets:
        sys.exit("no mutation sets found under scripts/mutations/")
    for p in sets:
        if not os.path.isfile(p):
            sys.exit(f"no such mutation set: {p}")

    # Everything below runs against this copy. The working tree is never written to,
    # so an interrupted run cannot leave a mutant behind and a concurrent edit cannot
    # turn a real result into a compile failure.
    work = tempfile.mkdtemp(prefix="plock-mutate-")
    global ROOT
    ROOT = os.path.join(work, "repo")
    # The try opens here rather than at the loop. A set file that fails validation makes
    # mutset.load call sys.exit, and an exit between the copy and the loop used to leave a
    # whole copy of the tree behind in /tmp.
    try:
        shutil.copytree(SRC, ROOT, ignore=shutil.ignore_patterns(
            ".git", "bin", ".cortexkit", "__pycache__"))
        print(f"running against a copy of the tree in {ROOT}")

        # --only writes a trimmed copy of the set rather than filtering inside run_set,
        # so what the run reports is exactly what the file on disk said -- one trimmed
        # file per set that matches, because a run that answered part of the request must
        # not print a TOTAL for the whole of it (#118).
        missing = []
        if args.only is not None:
            wanted = {s.strip() for s in args.only.split(",") if s.strip()}
            if not wanted:
                sys.exit(f"--only {args.only!r} names no mutant ids")
            sets, missing = select_sets(sets, wanted, work)
            if not sets:
                sys.exit(f"none of {sorted(wanted)} is in {scope(args.sets)}")
        else:
            sets = [(p, os.path.relpath(p, SRC)) for p in sets]

        totals = {"caught": 0, "survived": 0, "invalid": 0, "equivalent": 0}
        for path, label in sets:
            tally = run_set(path, label)
            if tally is None:
                return 2
            for k in totals:
                totals[k] += tally[k]
    finally:
        shutil.rmtree(work, ignore_errors=True)

    print(f"\nTOTAL  {totals['caught']} caught, {totals['survived']} survived, "
          f"{totals['equivalent']} equivalent, {totals['invalid']} invalid")
    if missing:
        # On stdout, under the TOTAL, because the report is what a reader trusts: a run
        # that answered part of the request has to say so where the answer is, and it must
        # not exit 0 for a scripted `mutate.py --only ... && ...` to mistake for the check
        # having happened (#118).
        print(f"INCOMPLETE  {', '.join(missing)} "
              f"{'is' if len(missing) == 1 else 'are'} in {scope(args.sets)}, so "
              f"{'it was' if len(missing) == 1 else 'they were'} never run")
        return 1
    return 0 if totals["survived"] == 0 and totals["invalid"] == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
