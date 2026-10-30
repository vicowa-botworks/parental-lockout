#!/usr/bin/env python3
"""The toolchain-free half of check-mutations.py, fast enough to belong in `make verify`.

Why this file exists (issue #189). PR #188 printed `verify: OK` on a tree whose CI then failed:

    mutation sets
      1 script encoding problem(s):
        scripts/mutate.py: line 463: open() names no encoding=, so it reads and writes in
        whatever locale the operator's shell happens to carry
      make: *** [Makefile:146: check-mutations] Error 1

Both guards that print that sentence live in `check-mutations.py`, and `verify` does not run
that target. CI runs it inside `mutation sets`, which spends about nine minutes applying and
compiling 827 mutants before it says anything about a locale default. The two halves of one
file's checks were split across two targets, and the local half could not see a violation in a
file the other half reads.

Measured on the tree that shipped the bug:

    the encoding pass alone, over 13 scripts        0.009 s
    `python3 scripts/check-mutations.py --self-test`  0.24 s
    `make check-mutations`, whole target            2 m 56 s

So the cheap half lives here, and this file is in `verify`. It **imports** the guards rather
than copying them: two copies of a check are two chances for the gates to disagree about what is
allowed, and the failure this exists to prevent is precisely one gate being blind to something
the other could see. `check-mutations` keeps everything that needs a Go toolchain -- applying
each mutant and compiling the package and its tests -- because that is the part worth a runner,
and the part that would make `verify` slow enough to skip.

The self-test in check-mutations.py pins that the calls below are still the calls it guards, for
the same reason it pins its own call sites: a deleted call leaves every other rule holding.
"""

import importlib.util
import os
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

USAGE = """usage: check-scripts.py [--help]

Runs the parts of the mutation gate that need no Go toolchain:

  * every Python tool under scripts/ read for an open() that names no encoding= (#154), and
    a subprocess call that decodes a child's output in the operator's locale (#151's tail)
  * every set file under scripts/mutations/ read for bytes a generator wrote rather than a
    person declared (#133, #151)

Exit is decided by check-mutations.exit_status, the same function the full pass uses, so the
two gates cannot disagree about what a problem costs. The apply-and-compile half stays in
`make check-mutations`; this is the half that belongs in `make verify`."""


def load_gate():
    """Import check-mutations.py by path.

    Its name has a hyphen -- it is the spelling every error message in this repository quotes --
    so it is not importable by module name, and renaming it to satisfy an import would rename it
    in the docs, the Makefile and CI.
    """
    path = os.path.join(ROOT, "scripts", "check-mutations.py")
    spec = importlib.util.spec_from_file_location("plock_check_mutations", path)
    if spec is None or spec.loader is None:
        raise SystemExit(f"check-scripts: cannot load {path} as a module")
    module = importlib.util.module_from_spec(spec)
    # Registering the name first: the module's own module-level `from mutset import ...` runs
    # during exec_module, and a dataclass or pickle path inside it looks the module up by name.
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def main(argv):
    if len(argv) > 1 and argv[1] in ("-h", "--help"):
        print(USAGE)
        return 0
    if len(argv) > 1:
        print(f"check-scripts: takes no arguments beyond --help; got {' '.join(argv[1:])!r}\n"
              f"It checks the tools and the set files, always all of them -- narrowing it to a "
              f"file is\nwhat made `check-mutations.py --help` open a file named '--help'.",
              file=sys.stderr)
        return 2

    gate = load_gate()

    # The same two calls the full pass makes, in the same order, with the same labels.
    script_problems = gate.script_open_problems(gate.script_files())

    dirn = os.path.join(ROOT, "scripts", "mutations")
    sets = sorted(os.path.join(dirn, f) for f in os.listdir(dirn) if f.endswith(".json"))
    file_problems = []
    for path in sets:
        label = os.path.relpath(path, ROOT)
        found, bad_bytes = gate.encoding_problems(label, path)
        file_problems.extend(found)
        if bad_bytes:
            print(f"{label}: skipped -- its bytes are not UTF-8, so there is nothing to check")

    if script_problems:
        print(f"\n{len(script_problems)} script encoding problem(s):")
        for why in script_problems:
            print(f"  {why}")
    if file_problems:
        print(f"\n{len(file_problems)} set-file encoding problem(s):")
        for why in file_problems:
            print(f"  {why}")

    # `bad` is 0 and the collision list is empty because both belong to the apply-and-compile
    # half. They are passed rather than defaulted: exit_status takes four lists on purpose, and
    # a default is how a call site that forgot one would stay green (see its docstring).
    status = gate.exit_status(0, file_problems, script_problems, [])
    if status:
        print("\nNothing here needed a compiler. `make check-mutations` is the half that does, "
              "and CI\nruns it as `mutation sets`; this pass is the part that should never have "
              "reached a runner.")
        return status
    print(f"check-scripts: {len(script_problems) + len(file_problems)} problems in "
          f"{len(gate.script_files())} tools and {len(sets)} set files")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
