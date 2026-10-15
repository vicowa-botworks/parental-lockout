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

Four ways a battery like this can report success while testing nothing, all four
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
    occurrences it expects and the run checks the count.

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
import json
import os
import shutil
import subprocess
import tempfile
import sys

SRC = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
# The Makefile takes GO ?= go; honouring it here keeps `make mutate GO=...` testing
# the compiler it names rather than whatever else is on PATH.
GO = os.environ.get("GO", "go")
ROOT = SRC  # main() points this at the throwaway copy


def load(path):
    with open(path) as fh:
        doc = json.load(fh)
    if "package" not in doc or "mutants" not in doc:
        sys.exit(f"{path}: expected an object with 'package' and 'mutants'")
    return doc


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
    pkg, mutants = doc["package"], doc["mutants"]
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

        if "[build failed]" in out or "[setup failed]" in out:
            print(f"INVALID    {label} (did not compile, so nothing was tested)")
            tally["invalid"] += 1
        elif r.returncode != 0:
            names = failing_tests(out)
            print(f"CAUGHT     {label} by {', '.join(names[:2]) or 'a test'}")
            tally["caught"] += 1
        elif m.get("equivalent"):
            print(f"EQUIVALENT {label} -- {m.get('note', 'no note')}")
            tally["equivalent"] += 1
        else:
            print(f"SURVIVED   {label}")
            tally["survived"] += 1

    restore()
    print(f"{tally['caught']} caught, {tally['survived']} survived, "
          f"{tally['equivalent']} equivalent, {tally['invalid']} invalid, "
          f"{len(mutants)} total")
    return tally


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("sets", nargs="*", help="mutation JSON files")
    ap.add_argument("--only", metavar="IDS",
                    help="comma-separated mutant ids to run, e.g. S1,W3")
    args = ap.parse_args()

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
    shutil.copytree(SRC, ROOT, ignore=shutil.ignore_patterns(
        ".git", "bin", ".cortexkit", "__pycache__"))
    print(f"running against a copy of the tree in {ROOT}")

    # --only writes a trimmed copy of the set rather than filtering inside run_set,
    # so what the run reports is exactly what the file on disk said.
    tmp = None
    if args.only:
        wanted = {s.strip() for s in args.only.split(",") if s.strip()}
        for path in sets:
            doc = load(path)
            keep = [m for m in doc["mutants"] if m["id"] in wanted]
            if not keep:
                continue
            doc["mutants"] = keep
            tmp = os.path.join(work, "subset.json")
            with open(tmp, "w") as fh:
                json.dump(doc, fh, indent=1)
            sets = [(tmp, f"{os.path.basename(path)} [{args.only}]")]
            break
        else:
            shutil.rmtree(work, ignore_errors=True)
            sys.exit(f"none of {sorted(wanted)} is in any set under scripts/mutations/")
    else:
        sets = [(p, os.path.relpath(p, SRC)) for p in sets]

    try:
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
    return 0 if totals["survived"] == 0 and totals["invalid"] == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
