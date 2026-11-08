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

    the encoding pass alone, over 5 tools and 13 set files   0.009 s
    `python3 scripts/check-mutations.py --self-test`           0.24 s
    `make check-mutations`, whole target                     2 m 56 s

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
import re
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


# --------------------------------------------------------------------------- command-shape guard

# A value variable in command position runs its *value* as a program. Measured on `b5fd0ca`:
# `if ! "$DRY_RUN"; then` printed `bash: 0: command not found` on every real pass and took the sweep
# arm, so `--dry-run` deleted the files a dry run exists to spare, and `"$DRY_RUN" || systemctl --user
# restart` did the same to the timer restart. Neither `shellcheck` 0.10.0 nor `bash -n` can see a
# variable's value -- the parse is legal, the value is the bug -- so once #217 made the shellcheck step
# real, this is the only thing in the repository that can catch the class.
#
# The rule is deliberately narrower than "no $VAR in command position", because that one is false here:
# `"$PLOCK_BIN" version` and `"$PLOCK" serve` are the idiom for running a binary whose path the caller
# chose, and the first draft of this guard refused six of them -- `testing/capture/fixture.sh:117`,
# `testing/e2e/e2e.sh:188`, `testing/e2e/forge-hint.sh:156` -- which is how a guard gets widened to
# death. What is refused is a variable that only ever holds `0` or `1`, anywhere in the shipped scripts:
# such a name is a flag, and a flag run as a command is always a defect.
COMMAND_WORD_BEFORE = {"", "!", "&&", "||", "|", ";", "then", "else", "do", "if", "while", "until",
                       "(", "{", "time"}
ASSIGN = re.compile(r'^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)=(.*)$')
DEFAULT_ONLY = re.compile(r'^\s*:\s*"\$\{([A-Za-z_][A-Za-z0-9_]*):-[01]\}"\s*$')
VAR_USE = re.compile(r'(?:"\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?"|\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?)')


def _assignment(line):
    """(name, kind) for one line, kind in ('flag', 'other'), or None when it assigns nothing.

    Both flag idioms count: `DRY_RUN=0` and `DRY_RUN="${DRY_RUN:-0}"` and the bare `: "${X:-1}"`.
    Reading only the literal form is what would have classed the harness's own flag as a path.
    """
    cut = "" if line.lstrip().startswith("#") else line.split("#", 1)[0]
    if not cut.strip():
        return None
    m = ASSIGN.match(cut)
    if m:
        name, value = m.group(1), m.group(2).strip()
        if value in ("0", "1", '"0"', '"1"'):
            return name, "flag"
        if re.match(r'^"?\$\{%s:-[01]\}"?$' % re.escape(name), value):
            return name, "flag"
        return name, "other"
    m = DEFAULT_ONLY.match(cut)
    return (m.group(1), "flag") if m else None


def flag_variables(texts):
    """Names assigned only `0` or `1` across every shipped script, with one file that does it.

    Every assignment in every file is read, not just the first: a name that holds `0` here and a path
    there is not a flag, and guessing from one line is how a guard starts refusing working code.
    """
    flags, other = {}, set()
    for label, text in texts:
        for number, line in enumerate(text.splitlines(), 1):
            found = _assignment(line)
            if not found:
                continue
            name, kind = found
            if kind == "flag":
                flags.setdefault(name, "%s:%d" % (label, number))
            else:
                other.add(name)
    return {name: where for name, where in flags.items() if name not in other}


def shape_problems(label, text, flags=None):
    """Every flag-valued `$VAR` a line runs as a command, as `file:line` findings.

    `flags=None` means derive them from this one file, which is what the self-test wants; the pass over
    the tree passes the set built across all files, so a name that is a flag everywhere is refused and a
    name that holds something else somewhere is not refused at all.
    """
    flags = flag_variables([(label, text)]) if flags is None else flags
    problems = []
    for number, raw in enumerate(text.splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        for match in VAR_USE.finditer(line):
            name = match.group(1) or match.group(2)
            if name not in flags:
                continue
            head = line[:match.start()].strip()
            tokens = head.replace(";", " ; ").replace("(", " ( ").split()
            before = tokens[-1] if tokens else ""
            if before in COMMAND_WORD_BEFORE:
                problems.append(
                    "%s:%d: `%s` runs $%s as a program, and $%s only ever holds 0 or 1 here (assigned "
                    "at %s) -- the value is the bug, not the parse: shellcheck and `bash -n` both pass "
                    "this. Say `[ \"$%s\" = \"1\" ]`"
                    % (label, number, match.group(0), name, name, flags[name], name))
    return problems


def shell_files():
    """The shell scripts this repository ships."""
    found = []
    for top in ("factory", "scripts", "testing"):
        base = os.path.join(ROOT, top)
        if not os.path.isdir(base):
            continue
        for dirn, _dirs, files in os.walk(base):
            for name in sorted(files):
                if name.endswith(".sh"):
                    found.append(os.path.join(dirn, name))
    return sorted(found)


SHAPE_BAD = [
    ("sweep.sh", 'DRY_RUN=0\nif ! "$DRY_RUN"; then\n\tfind "$FACTORY_RUN" -delete\nfi\n'),
    ("restart.sh", 'DRY_RUN=1\n\t"$DRY_RUN" || systemctl_user restart "$NAME.timer"\n'),
]
SHAPE_GOOD = [
    ("sweep.sh", 'DRY_RUN=0\nif [ "$DRY_RUN" != "1" ]; then\n\tfind "$FACTORY_RUN" -delete\nfi\n'),
    ("path.sh", 'PLOCK_BIN=./plock\n"$PLOCK_BIN" version\nPLOCK=./plock\n"$PLOCK" serve &\n'),
    ("arg.sh", 'DRY_RUN=0\nx="$DRY_RUN"\nif [ "$DRY_RUN" != "1" ]; then :; fi\n'),
]


def shape_self_test():
    """Both directions, asserted. A guard nobody can redden is a comment, and a guard that refuses
    working code gets widened away -- `path.sh` is the case that matters most, because the first draft
    of this guard refused six real call sites of exactly that shape."""
    fails = []
    for label, text in SHAPE_BAD:
        if not shape_problems(label, text):
            fails.append("the guard accepted %s, the exact line that shipped on b5fd0ca and made "
                         "--dry-run delete files" % label)
    for label, text in SHAPE_GOOD:
        problems = shape_problems(label, text)
        if problems:
            fails.append("the guard refused %s: %s" % (label, problems[0]))
    mixed = [("a.sh", "DRY_RUN=0\n"), ("b.sh", 'DRY_RUN=$(/bin/echo 1)\n')]
    if flag_variables(mixed):
        fails.append("a name that holds something other than 0/1 in one file was still classed as a "
                     "flag, so the set is per-line rather than per-tree")
    return fails


def main_shape(argv, texts):
    """`--self-test`: assert the guard bites, then run it over the shipped scripts."""
    if "--self-test" not in argv:
        return None
    fails = shape_self_test()
    if fails:
        print("check-scripts --self-test FAILED:", file=sys.stderr)
        for why in fails:
            print("  " + why, file=sys.stderr)
        return 1
    flags = flag_variables(texts)
    problems = []
    for label, text in texts:
        problems += shape_problems(label, text, flags)
    if problems:
        print("\n%d command-position problem(s):" % len(problems))
        for why in problems:
            print("  " + why)
        return 1
    print("check-scripts --self-test: the flag-in-command-position guard bites on both shapes that "
          "shipped, stays green on `\"$PLOCK_BIN\" version`, and sees %d flag variable(s) across %d "
          "shipped scripts" % (len(flags), len(texts)))
    return 0


def main(argv):
    if len(argv) > 1 and argv[1] in ("-h", "--help"):
        print(USAGE)
        return 0
    texts = [(os.path.relpath(path, ROOT), open(path, encoding="utf-8").read())
             for path in shell_files()]
    shape = main_shape(argv, texts)
    if shape is not None:
        return shape
    if len(argv) > 1:
        print(f"check-scripts: takes no arguments beyond --help; got {' '.join(argv[1:])!r}\n"
              f"It checks the tools and the set files, always all of them -- narrowing it to a "
              f"file is\nwhat made `check-mutations.py --help` open a file named '--help'.",
              file=sys.stderr)
        return 2

    tree_problems = []
    flags = flag_variables(texts)
    for label, text in texts:
        tree_problems += shape_problems(label, text, flags)

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
    if tree_problems:
        print(f"\n{len(tree_problems)} command-position problem(s):")
        for why in tree_problems:
            print(f"  {why}")
    if file_problems:
        print(f"\n{len(file_problems)} set-file encoding problem(s):")
        for why in file_problems:
            print(f"  {why}")

    # `bad` is 0 and the collision list is empty because both belong to the apply-and-compile
    # half. They are passed rather than defaulted: exit_status takes four lists on purpose, and
    # a default is how a call site that forgot one would stay green (see its docstring).
    status = gate.exit_status(0, file_problems, script_problems + tree_problems, [])
    if status:
        print("\nNothing here needed a compiler. `make check-mutations` is the half that does, "
              "and CI\nruns it as `mutation sets`; this pass is the part that should never have "
              "reached a runner.")
        return status
    print(f"check-scripts: {len(script_problems) + len(file_problems) + len(tree_problems)} problems in "
          f"{len(gate.script_files())} tools and {len(sets)} set files, and {len(texts)} shipped shell "
          f"scripts for command position")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
