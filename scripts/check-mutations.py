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

import json
import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
GO = os.environ.get("GO", "go")


def load(path):
    with open(path) as fh:
        doc = json.load(fh)
    if "mutants" not in doc:
        sys.exit(f"{path}: expected an object with a 'mutants' list")
    return doc


def compiles(pkg):
    """Compile the package *and its tests*, running nothing.

    `go build` is not enough: a mutant that deletes a helper can leave the package
    building while every test file fails to compile, and the battery would report that as
    INVALID rather than as the missing coverage it is. `-run ^$` compiles the test binary
    and selects no test, so this stays fast.
    """
    return subprocess.run(
        [GO, "test", "-run", "^$", "-count=1", pkg],
        capture_output=True,
        text=True,
        cwd=ROOT,
    )


# Every key the runner and this checker actually read. A key outside this list is a key
# nothing prints, and the two ways that happens are both silent:
#
#   - a rationale written under a name the runner does not know (`_why`, `why`, `reason`)
#     never reaches the battery's output, so the equivalence claim a reviewer needs to
#     audit is in the file and nowhere a human will look;
#   - a typo in a key that DOES matter (`equivalant`, `packge`) drops the mutant into the
#     default path -- an equivalent mutant starts reporting as a survivor, or a mutant
#     aimed at another package runs against the wrong one.
#
# Both look like a working set. `make verify` stays green and the battery prints a tally.
KNOWN_KEYS = {"id", "file", "old", "new", "package", "count", "equivalent", "note"}


def check(path):
    """Return a list of (mutant_id, problem) for one set."""
    doc = load(path)
    default_pkg = doc.get("package")
    problems = []
    # Cache file contents, and restore them all on the way out even if we are interrupted.
    original = {}
    touched = []
    try:
        for m in doc["mutants"]:
            mid, rel, old, new = m["id"], m["file"], m["old"], m["new"]
            for key in sorted(set(m) - KNOWN_KEYS):
                problems.append((mid, f"key {key!r} is read by nothing: whatever it says is "
                                      f"in the file but never printed, and a near-miss spelling "
                                      f"of a real key would silently disable that key"))
            if rel not in original:
                try:
                    with open(os.path.join(ROOT, rel), encoding="utf-8") as fh:
                        original[rel] = fh.read()
                except OSError as exc:
                    problems.append((mid, f"{rel} cannot be read: {exc}"))
                    continue
            src = original[rel]
            hits = src.count(old)
            if hits == 0:
                problems.append((mid, f"`old` matches nothing in {rel}: this mutant tests "
                                      f"nothing, and the behaviour it was written for is unchecked"))
                continue
            if hits > 1:
                problems.append((mid, f"`old` matches {hits} places in {rel}: which one gets mutated "
                                      f"is whichever the runner happens to reach first"))
                continue

            mutated = src.replace(old, new, 1)
            if mutated == src:
                problems.append((mid, "replacement is identical to the original: a no-op mutant"))
                continue

            # A mutant may name its own package (a set can span packages) or inherit the
            # set's. Neither being present is a defect in the tree, so say which is missing
            # rather than crashing on it.
            pkg = m.get("package", default_pkg)
            if not pkg:
                problems.append((mid, "no 'package' on the mutant or the set, so nothing can be compiled"))
                continue

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


def main(argv):
    paths = argv[1:]
    if not paths:
        dirn = os.path.join(ROOT, "scripts", "mutations")
        paths = sorted(os.path.join(dirn, f) for f in os.listdir(dirn) if f.endswith(".json"))

    total = bad = 0
    for path in paths:
        doc = load(path)
        problems = check(path)
        total += len(doc["mutants"])
        bad += len(problems)
        label = os.path.relpath(path, ROOT)
        if problems:
            print(f"{label}: {len(problems)} of {len(doc['mutants'])} mutants are not testing anything")
            for mid, why in problems:
                print(f"  {mid}: {why}")
        else:
            print(f"{label}: all {len(doc['mutants'])} mutants apply and compile")

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
