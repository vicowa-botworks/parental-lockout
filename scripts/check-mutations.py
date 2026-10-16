#!/usr/bin/env python3
"""Pre-flight every mutation set: does each mutant still apply, and does it still compile?

This exists because a mutation battery reports a *tally*, and two of its failure modes
look like a pass:

  - **NOT APPLIED.** The mutant's `old` no longer matches the tree, so nothing was
    changed, the suite stayed green, and the line still printed. The check that mutant
    represents is no longer tested by anything.
  - **INVALID (did not compile, so nothing was tested).** The replacement names something
    that no longer exists. Same outcome: a line in the tally, no check behind it.

Both arrived the same way, three separate times on this repository: a rename or a
restructure in a source file silently retired the mutants pointed at it, and `make verify`
stayed green throughout. The battery is the only thing that notices, and it notices only
if somebody reads every line rather than the total.

Run it before a battery, and after any change to a file a set covers:

    python3 scripts/check-mutations.py            # every set
    python3 scripts/check-mutations.py scripts/mutations/platform.json

It is not in `make verify` on purpose: it compiles the tree once per mutant, which is
seconds per set and would make the gate something people skip. `make mutate` is already
the slow target; this is the fast one that tells you the slow one is worth running.
"""

import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
GO = os.environ.get("GO", "go")

# Shape rules come from mutset, the same module the battery loads. Two validators that
# disagree are how a set ends up green at this gate and refused at the battery.
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from mutset import duplicate_claims, load, validate  # noqa: E402


def compiles(pkg):
    """Compile the package *and its tests*, running nothing.

    `go build` is not enough: a mutant that deletes a helper can leave the package
    building while every test file fails to compile, and the battery would report that as
    INVALID rather than as the missing coverage it is. `-run ^$` compiles the test binary
    and selects no test, so this stays fast.
    """
    return subprocess.run(
        # Split for the same reason mutate.py splits: a package field may name several
        # packages, and an unsplit one fails to resolve in a way that reads as a
        # compile failure rather than as the checker being unable to ask.
        # -timeout matched with mutate.py:82 on purpose. `-run ^$` selects no test but
        # still runs the binary, so a mutated package whose init hangs would hang this
        # process too -- and this now runs as a required check, where the default is
        # GitHub's six-hour job limit rather than a human noticing and pressing ctrl-C.
        [GO, "test", "-run", "^$", "-count=1", "-timeout", "900s", *pkg.split()],
        capture_output=True,
        text=True,
        cwd=ROOT,
    )


def check(doc):
    """Return a list of (mutant_id, problem) for one already-validated set."""
    default_pkg = doc.get("package")
    problems = []
    # Cache file contents, and restore them all on the way out even if we are interrupted.
    original = {}
    touched = []
    try:
        for m in doc["mutants"]:
            mid, rel, old, new = m["id"], m["file"], m["old"], m["new"]
            # A mutant may ask for its `old` to appear N times, and the runner replaces every
            # occurrence. Both halves of that have to be honoured here or the gate is testing
            # a tree the battery never runs: a count>1 mutant used to be rejected for matching
            # twice, and if it had been accepted the gate would have compiled a one-occurrence
            # mutation while the battery applied all of them.
            want = m.get("count", 1)
            if rel not in original:
                try:
                    with open(os.path.join(ROOT, rel), encoding="utf-8") as fh:
                        original[rel] = fh.read()
                except OSError as exc:
                    problems.append((mid, f"{rel} cannot be read: {exc}"))
                    continue
            src = original[rel]
            hits = src.count(old)
            if hits != want:
                problems.append((mid, f"`old` matches {hits} places in {rel}, expected {want}: this "
                                      f"mutant does not apply, so the behaviour it was written for "
                                      f"is unchecked"))
                continue

            mutated = src.replace(old, new)
            if mutated == src:
                problems.append((mid, "replacement is identical to the original: a no-op mutant"))
                continue

            pkg = m.get("package", default_pkg)

            abs_path = os.path.join(ROOT, rel)
            if rel not in touched:
                touched.append(rel)
            with open(abs_path, "w", encoding="utf-8") as fh:
                fh.write(mutated)
            rc = compiles(pkg)
            if rc.returncode != 0:
                first = (rc.stderr or rc.stdout).strip().splitlines()
                problems.append((mid, "does not compile: " + (first[0] if first else "no output")))
    finally:
        for rel in touched:
            with open(os.path.join(ROOT, rel), "w", encoding="utf-8") as fh:
                fh.write(original[rel])
    return problems


def self_test():
    """Check the shape rules against synthetic sets, without a toolchain or the tree.

    Why this lives here rather than in a test file: the rules in mutset are the reason a
    green CI tick means something, and nothing else in the repository exercises them. A
    battery needs Go and minutes; these assertions need neither, so they can run on the same
    path that checks the sets and fail for the same reason the gate would.

    Every assertion goes through one of the three helpers below, and they share a counter.
    The count printed at the end is a claim about how much was tested, so it is counted
    rather than derived from the length of one of the lists -- an earlier version of this
    printed `len(cases) + 2` and went stale the moment a case was added, which is the exact
    species of error this whole change is about.

    The controls (the `expect_accepts` and the `want=False` calls) are what make the
    rejections evidence: a validator that rejects everything would pass every "refused"
    assertion in here.
    """
    good = {"id": "T1", "file": "x.go", "old": "a", "new": "b"}
    problems = []
    checked = 0

    def expect_rejects(name, doc, want):
        nonlocal checked
        checked += 1
        found = validate(doc)
        if not any(want in p for p in found):
            problems.append(f"{name}: expected a problem containing {want!r}, got {found or 'none'}")

    def expect_accepts(name, doc):
        nonlocal checked
        checked += 1
        if validate(doc):
            problems.append(f"{name}: control set was rejected: {validate(doc)}")

    def expect_claims(name, docs, want):
        nonlocal checked
        checked += 1
        got = bool(duplicate_claims(docs))
        if got != want:
            problems.append(f"{name}: expected {'a duplicate report' if want else 'silence'}, "
                            f"got {got}")

    expect_rejects("one id used twice in a set",
                   {"package": "./x/", "mutants": [good, dict(good, id="T1", new="c")]},
                   "is used twice in this set")
    expect_rejects("a rationale filed under a key nothing reads",
                   {"package": "./x/", "mutants": [dict(good, _why="proved it in a scratch file")]},
                   "is read by nothing")
    expect_rejects("a mutant no package would be run against",
                   {"mutants": [good]}, "no 'package' on the mutant or the set")
    expect_rejects("an `equivalent` that says the opposite of what it means",
                   {"package": "./x/", "mutants": [dict(good, equivalent="false")]},
                   "which is TRUTHY")
    expect_rejects("a count that is not a count",
                   {"package": "./x/", "mutants": [dict(good, count="2")]},
                   "must be an integer of 1 or more")
    expect_rejects("a required field that is not a string",
                   {"package": "./x/", "mutants": [dict(good, old="")]},
                   "'old' must be a non-empty string")

    # An empty `new` is a DELETION and 19 mutants in these sets are one, so the validator
    # must not treat it as missing -- the same mistake would have rejected a valid set.
    expect_accepts("an empty `new`, which is a deletion mutant",
                   {"package": "./x/", "mutants": [dict(good, new="")]})
    # Three sets keep the equivalence PROOF in `equivalent`, so a string is a valid claim.
    expect_accepts("`equivalent` holding the proof as prose",
                   {"package": "./x/", "mutants": [dict(good, equivalent="proved in a scratch file")]})
    expect_accepts("a control set with every rule satisfied",
                   {"package": "./x/", "mutants": [good, dict(good, id="T2", count=2, equivalent=True)]})

    expect_claims("the same mutation in two sets",
                  [("a.json", {"mutants": [good]}), ("b.json", {"mutants": [good]})], True)
    expect_claims("two different mutations of one line",
                  [("a.json", {"mutants": [good]}),
                   ("b.json", {"mutants": [dict(good, new="a different bug")]})], False)

    # A mutant's claim includes WHICH package it asks, so containment is the test and two
    # spellings of a package list are not two claims. This is the real shape: a set added for
    # one package re-deriving a mutant another set already runs against that package and more.
    narrow = {"package": "./internal/cli/", "mutants": [good]}
    wider = {"package": "./cmd/", "mutants": [dict(good, package="./internal/cli/ ./cmd/plock/")]}
    expect_claims("a mutant whose packages are a subset of another's",
                  [("cli.json", narrow), ("cmd.json", wider)], True)
    expect_claims("control: the same mutation against two disjoint packages",
                  [("cli.json", narrow),
                   ("other.json", {"package": "./internal/version/", "mutants": [good]})], False)

    if problems:
        print("self-test FAILED:")
        for p in problems:
            print(f"  {p}")
        return 1
    print(f"self-test: {checked} shape rules hold")
    return 0


def main(argv):
    if argv[1:] == ["--self-test"]:
        return self_test()
    paths = argv[1:]
    if not paths:
        dirn = os.path.join(ROOT, "scripts", "mutations")
        paths = sorted(os.path.join(dirn, f) for f in os.listdir(dirn) if f.endswith(".json"))

    total = bad = 0
    named = []
    for path in paths:
        doc = load(path)
        named.append((os.path.relpath(path, ROOT), doc))
        problems = check(doc)
        total += len(doc["mutants"])
        bad += len(problems)
        label = os.path.relpath(path, ROOT)
        if problems:
            print(f"{label}: {len(problems)} of {len(doc['mutants'])} mutants are not testing anything")
            for mid, why in problems:
                print(f"  {mid}: {why}")
        else:
            print(f"{label}: all {len(doc['mutants'])} mutants apply and compile")

    # One claim counted twice is a tally that overstates what the battery proved, and it is
    # invisible from inside a single set -- which is how a mutant added to guard a shared
    # helper can duplicate one that already exists in another file's set. Checked across
    # every set this run was given.
    dupes = duplicate_claims(named)
    bad += len(dupes)
    if dupes:
        print(f"\n{len(dupes)} duplicated claim(s) across these sets:")
        for why in dupes:
            print(f"  {why}")

    if bad:
        print(f"\n{bad} of {total} mutants would run without testing anything.")
        print("Re-point them onto the code that replaced theirs -- updating `old` but not")
        print("`new` is how a mutant ends up failing to compile, which the battery prints as")
        print("a line and checks nothing.")
        return 1
    print(f"\nall {total} mutants verified")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
