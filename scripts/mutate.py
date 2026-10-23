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

Six ways a battery like this can report success while testing nothing, all six
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
import contextlib
import glob
import inspect
import io
import json
import os
import re
import shutil
import socket
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


# The snapshot's ignore rules, kept here with the other rules rather than next to the
# copytree call that uses them, for the reason the comment above the verdict rules gives:
# a rule that is only exercised by a full battery is a rule nobody sees fail. These two
# are pure, so the self-test below can feed them a tree instead of waiting an hour for
# one.

# Names that are never source. `bin` is go's build output, `.cortexkit` is tool state, and
# `__pycache__` is python's; the first three are directories at the root of this repository
# and nothing else, so they are matched at the root and nowhere else.
#
# They used to be matched by basename at any depth, and that is a fail-open with a body:
# a tracked `internal/platform/bin/runner.go` is source, the name says nothing about it, and
# the copy would drop it in silence -- no error, nothing in `skipped`, and a battery that
# reports every mutant caught against a tree missing a package. `.cortexkit/.gitignore` is
# tracked here today and was already going missing. `__pycache__` is the one name that
# legitimately appears at every depth.
NEVER_COPIED_ROOT = (".git", "bin", ".cortexkit")
NEVER_COPIED_ANYWHERE = ("__pycache__",)
NEVER_COPIED = NEVER_COPIED_ROOT + NEVER_COPIED_ANYWHERE


# The one git call the snapshot makes, spelled out here so the self-test can point it at
# something that is not git and check what happens when git cannot answer. The `-z` is not
# cosmetic: git C-quotes any path holding a byte above 0x7f, a double quote, or a control
# character, and a quoted `"testing/capture/runs-caf\303\251/"` matches nothing on disk --
# so the crash this whole function exists to prevent comes back for a directory with an
# accented letter in its name. NUL-separated output is never quoted.
LS_IGNORED = ("ls-files", "--others", "--ignored", "--exclude-standard", "--directory",
              "-z")

# What the answer must not depend on. `GIT_DIR` and friends are exported by hooks and by
# wrapper scripts that call `make verify`, and a machine-wide `core.excludesFile` is not
# part of this repository. Either one silently changes what a battery copies; the second
# one also makes the self-test's "this directory is NOT ignored" assertion depend on
# someone's dotfiles.
GIT_ISOLATED = ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_CEILING_DIRECTORIES",
                "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_COUNT",
                "GIT_CONFIG_PARAMETERS")


def git_env():
    """The environment for the snapshot's git call: the ambient one, minus everything that
    could make the answer a function of the machine rather than of the tree."""
    env = {k: v for k, v in os.environ.items() if k not in GIT_ISOLATED}
    # Both config files pointed at the void. The repository's own .gitignore files are read
    # from disk whatever the environment says, and those are the only rules the snapshot
    # wants to honour.
    env["GIT_CONFIG_GLOBAL"] = os.devnull
    env["GIT_CONFIG_SYSTEM"] = os.devnull
    return env


def ignored_paths(src, git="git"):
    """The paths `src` declares ignored, spelled as git spells them.

    Directories come back with a trailing slash and collapsed to the whole directory when
    everything under it is ignored (`testing/capture/runs/`); single files come back bare
    (`plock`). Empty means "nothing beyond NEVER_COPIED", which is also the answer when git
    cannot be asked: a tree with no git in it is then snapshotted exactly as it was before
    this function existed, because not being able to ask is no reason to refuse a run.
    What makes that survivable rather than fatal is the error sorting in `snapshot`, not luck.

    `git` is a command, and a list is a command too: the self-test has to hand this a stub
    that both writes an answer and refuses, which is the only way to pin the branch that
    throws an answer away.

    The output is decoded as the filesystem spells it, not as text: a path is bytes, and
    `text=True` would raise on a name the locale cannot decode, which is the same failure
    `-z` exists to avoid.
    """
    cmd = [git] if isinstance(git, str) else list(git)
    try:
        done = subprocess.run([*cmd, *LS_IGNORED], cwd=src, env=git_env(), timeout=60,
                              stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    except (OSError, subprocess.SubprocessError):
        # No git, or a git that would not answer in time: a hung git (a locked index, a
        # credential helper waiting on a prompt) must not hang the battery before it
        # prints anything either.
        return []
    if done.returncode != 0:
        return []
    return [os.fsdecode(p) for p in done.stdout.split(b"\0") if p]


def tracked_paths(src, git="git"):
    """Every path the repository tracks, or None when git cannot be asked.

    None rather than an empty list, because "nothing is tracked" and "I could not tell you"
    are different answers and only one of them is a reason to stop a run.
    """
    cmd = [git] if isinstance(git, str) else list(git)
    try:
        done = subprocess.run([*cmd, "ls-files", "-z"], cwd=src, env=git_env(),
                              timeout=60, stdout=subprocess.PIPE,
                              stderr=subprocess.DEVNULL)
    except (OSError, subprocess.SubprocessError):
        return None
    if done.returncode != 0:
        return None
    return [os.fsdecode(p) for p in done.stdout.split(b"\0") if p]


def snapshot_ignore(src, root=NEVER_COPIED_ROOT, anywhere=NEVER_COPIED_ANYWHERE,
                    ignored=()):
    """The `ignore` callback for one snapshot of `src`.

    Three things are left out of the copy: the names in `root`, but only at the root; the
    names in `anywhere`, at any depth; and any path the repository itself ignores.

    The second half is the bug this function exists for. `testing/capture/runs/` holds a
    Chrome profile, and a profile contains a socket -- `SingletonSocket`, which cannot be
    read, `ENXIO` -- and symlinks whose targets are gone by the time the copy runs --
    `SingletonCookie`, `SingletonLock`, `ENOENT`. `copytree` does not skip those: it
    collects the errors and raises once the walk is done, so a battery died in two seconds
    on a tree whose only crime was containing a photograph, before a single mutant ran.
    The directory is gitignored, so nothing in `git status`, in the pre-flight checker, or
    in CI showed the thing that did it.

    The predicate is equality on the path being walked, never a prefix match, at both places
    equality appears. `testing/capture` CONTAINS an ignored directory and must still be
    copied: dropping it would take source out of the snapshot, and the run would then report
    every mutant caught against a tree that is missing files. Nor may a name merely starting
    with an ignored path go with it -- `plock2` sits beside an ignored `plock` and is source.
    The other half is the short-circuit that skips a name already dropped: a prefix there
    keeps a name OUT of `gone`, so `.venv-old`, itself in `.gitignore` beside `.venv`, comes
    back into the copy. One direction loses source, the other brings a socket back.
    """
    base = shutil.ignore_patterns(*anywhere)
    paths = {p.rstrip("/") for p in ignored}

    def ignore(directory, names):
        gone = set(base(directory, names))
        rel = os.path.relpath(directory, src)
        at_root = rel == os.curdir
        for name in names:
            if name in gone:
                continue
            if at_root and name in root:
                gone.add(name)
                continue
            path = os.path.normpath(os.path.join(rel, name)).replace(os.sep, "/")
            if path in paths:
                gone.add(name)
        return gone

    return ignore


def missing_source(src, dst, tracked):
    """Tracked paths that were meant to arrive in `dst` and did not.

    The copy is the run's whole world, so "did the source get there" is the one question
    that must not be answered by hoping. Every name git tracks belongs in the copy unless
    the snapshot deliberately excludes it -- a root-level never-copied name, or a path under
    one -- and everything else that goes missing is the same failure: a battery reporting
    mutants caught against a tree that is not the repository. How it went missing (a
    predicate bug, a file that vanished mid-copy, a symlink whose target could not be read)
    is not worth distinguishing, because none of the three is survivable.
    """
    gone = []
    for path in tracked:
        parts = path.split("/")
        if parts[0] in NEVER_COPIED_ROOT or any(p in NEVER_COPIED_ANYWHERE
                                                for p in parts):
            continue
        if not os.path.lexists(os.path.join(dst, *parts)):
            gone.append(path)
    return gone


class SnapshotError(Exception):
    """Source that could not be copied. Fatal, where an uncopyable socket is not."""


def uncopyable_is_source(path):
    """Whether a path `copytree` could not copy could have been source.

    A socket, a FIFO and a device node are never source, and neither is a symlink whose
    target is gone -- those four are exactly what a Chrome profile leaves behind. A symlink
    whose target EXISTS is the opposite case: `copytree` follows it and copies the target, so
    a copy failure on one has taken a real file out of the run, and every symlink git can
    track is source. A regular file and a directory both might be source, and a battery that
    quietly copied a tree missing one would report every mutant caught over files that were
    not there.

    A function rather than a clause inside the error loop so the self-test can hand it real
    things and watch which way each sorts, including as root, where no file is unreadable
    and a permission-based fixture would check nothing at all.
    """
    if os.path.islink(path):
        return os.path.exists(path)
    return os.path.isdir(path) or os.path.isfile(path)


def note_uncopyable(path, err, skipped, fatal):
    """Sort one copytree failure into "skip it" or "stop the run".

    The lists come in from the caller so the self-test can pass two empty ones and see the
    sorting happen; a rule that could only observe a finished copy could not tell a skipped
    socket from a dropped source file.
    """
    if uncopyable_is_source(path):
        fatal.append(f"{path}: {err}")
    else:
        skipped.append(path)


def _copy_tree(src, dst, ignore):
    """The one copytree call, named so the self-test can hand `snapshot` a failure."""
    shutil.copytree(src, dst, ignore=ignore)


def snapshot(src, dst, copy=_copy_tree):
    """Copy `src` to `dst` as a battery needs it: the source, and nothing else.

    Both `main` and the self-test call this, and that is the point of it existing. The
    composition -- which ignore callback, over which ignore list -- IS the fix, and a
    self-test that exercised only the two pure functions stayed green with this call
    reverted to the four hardcoded names: the ticket's crash, at full green.

    The error sorting is what closes the class rather than one instance of it. A path that
    cannot be copied is skipped when it could not have been source anyway -- a socket, a
    FIFO, a device node, a symlink whose target is gone -- and refused when it might have
    been source, because a battery that quietly covers less than it claims is worse than one
    that crashes. `copytree` no longer takes an `onerror` (3.14 removed it), so the failures
    it collects are sorted afterwards, out of the (src, dst, why) triples it puts in
    `shutil.Error` -- the same list the ticket's traceback printed.

    Then the copy is asked whether it holds the repository. `copytree` can come back without
    an error and without a file: a path that vanished between the walk and the copy is not a
    file by the time the sorting re-stats it, and a predicate bug drops one in silence. The
    tracked set is the only answer that does not depend on how the loss happened.

    `copy` exists for the same reason the pure functions do. The fail-closed half is
    otherwise reachable only by making a file unreadable, which as root is not a thing that
    happens -- and with the refusal deleted, the self-test printed "14 snapshot rules hold".

    Returns the paths it skipped, so a caller can say so out loud.
    """
    skipped, fatal = [], []
    try:
        copy(src, dst, snapshot_ignore(src, ignored=ignored_paths(src)))
    except shutil.Error as err:
        for path, _dst, why in err.args[0]:
            note_uncopyable(path, why, skipped, fatal)
    if fatal:
        raise SnapshotError("the snapshot could not copy source:\n  "
                            + "\n  ".join(fatal))
    tracked = tracked_paths(src)
    if tracked is None:
        print("snapshot: git could not list the tracked files, so nothing checked that "
              "the copy holds the source")
    else:
        absent = missing_source(src, dst, tracked)
        if absent:
            shown = "\n  ".join(absent[:20])
            more = "" if len(absent) <= 20 else f"\n  ... and {len(absent) - 20} more"
            raise SnapshotError(f"the snapshot is missing {len(absent)} tracked files it "
                                f"should hold:\n  {shown}{more}")
    return skipped


def set_label(path, root=SRC):
    """How one set is named in the report.

    Repo-relative where the path is inside the repo. `basename` was the first version, and
    two sets sharing a basename -- `scripts/mutations/alpha.json` and a second
    `alpha.json` elsewhere -- then printed two identical lines for two different sets,
    which is the ambiguity the label exists to remove. A path outside the repo keeps its
    basename, so the self-test's fixtures stay readable.

    `root` is a parameter for that last sentence's own reason: the fixtures live under a
    TemporaryDirectory, which is outside SRC, so every self-test case took the basename arm
    and the relative half of this function was executed by nothing. A revert to
    `return os.path.basename(path)` -- undoing the fix above -- left the self-test green.
    Passing the fixture's own root lets a rule drive the relative half without writing
    inside the repo, which this file's own docstring forbids.
    """
    rel = os.path.relpath(path, root)
    return os.path.basename(path) if rel.startswith("..") else rel


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
        selected.append((tmp, f"{set_label(path)} "
                              f"[{','.join(m['id'] for m in keep)}]"))
    return selected, sorted(missing)


# The report's four columns. One definition because the TOTAL line, the exit code and the
# self-test's fixtures all have to agree on what a tally holds; a fifth column added to the
# print and not to this tuple is a column that silently never accumulates.
TOTAL_KEYS = ("caught", "survived", "invalid", "equivalent")


def run_selection(sets, run):
    """Run every set in ``sets``, returning ``(totals, ran_everything)``.

    ``run(path, label)`` is one set's battery: it answers with that set's tally dict, or with
    ``None`` for "this set could not be run at all". ``None`` stops the run rather than being
    summed past, because a battery that skipped a set has no honest ``TOTAL`` left to print;
    ``ran_everything`` is how ``main`` knows to exit 2 and say nothing.

    A function rather than a loop in ``main``, for the reason every other piece of this file
    is one. The loop is where the selection is CONSUMED, so it is where #118's symptom lives
    -- one set run, a complete-looking ``TOTAL``, exit 0 -- and auditing it as source text
    took four rounds and never closed: a call-site pin (round 1, defeated one line below),
    an assignment-shaped deny list (round 3, defeated by ``del sets[1:]``), and a region that
    ended at the loop header (round 4, defeated by a ``break`` in the body). A text audit can
    only deny the shapes someone thought to name, and the loop body is where the shapes that
    were not thought of go to hide. Run instead, against a fake runner, the same five edits
    are three failing assertions: a ``break`` or a ``continue`` leaves a set unrun, a ``pass``
    where the sum happens leaves the totals at zero, and a success returned where the stop
    signal belongs turns "nothing ran" into exit 0.
    """
    totals = {k: 0 for k in TOTAL_KEYS}
    for path, label in sets:
        tally = run(path, label)
        if tally is None:
            return totals, False
        for k in totals:
            totals[k] += tally[k]
    return totals, True


def rule(rules, bad, name, problems):
    """Count one self-test rule, and report whatever it found.

    The tally and the verdict leave the same call on purpose. Counting at the call site
    meant ``if False:`` in front of a check kept the pinned number exactly where it was --
    the tally counted calls, not assertions, which is the hole attack118.py measured. With
    the increment in here, the mechanical gut drops the count and the pinned number shouts.

    A rule is recorded as the pair (name, how many problems it saw) and the reach check
    refuses a list whose entries are not pairs, because a bare ``rules.append(name)`` used to
    keep the pinned number exactly where it was while deleting the assertion: the count only
    ever measured the length of the list. Hand-forging the pair is still possible, and so is
    replacing a call's arguments with a constant -- both are deliberate edits of the guard,
    which is a different threat from drift. The count is a deletion tripwire, not a coverage
    claim, and only running the attack suite catches the deliberate kind.
    """
    rules.append((name, len(problems)))
    for problem in problems:
        bad.append(f"{name}: {problem}")


def control(rules, bad, name, problems, why):
    """Count one control: a checker that has to refuse something on purpose.

    ``problems`` is what the checker had to say about a deliberately wrong answer. Empty
    means it did not notice, which is the failure -- a control nobody can turn off is a
    control that proves nothing about the thing it claims to test.
    """
    rule(rules, bad, name, [] if problems else [f"it said nothing, {why}"])


def run_rule(rules, bad, name, body):
    """Count one self-test rule that is a block of code rather than a pure checker.

    ``body(problems)`` builds whatever its fixture needs -- a repository, a socket, a dead
    symlink -- and appends to the list it is handed. The count and the problems leave the
    same call for the reason `rule()` exists: a body deleted from the file drops the pinned
    number, and a body that is still called has still run its comparisons.

    This is the shape for the snapshot rules because they are fixtures rather than
    assertions: each assembles a tree that cannot be copied and asks what ``snapshot`` did
    with it. The standard this file holds everywhere else is that a comparison nobody can
    falsify is a comparison that can be hollowed, so where a snapshot rule's comparison CAN
    be handed a wrong answer -- the ignore table, and the spy over it -- it is a pure checker
    with a control instead, and what is left here is the acting kind: a body that hands
    ``snapshot`` a copy which fails on a named file, and requires the refusal. Like `rule()`'s
    own count, this is a deletion tripwire, not a coverage claim.
    """
    problems = []
    body(problems)
    rule(rules, bad, name, problems)


def ignore_row_problems(gone, want):
    """What one row of the ignore table got wrong, as sentences. Empty means it skipped
    exactly what the table says that directory skips.

    A function rather than a comparison inside the loop for the reason every other checker
    here is one: the canary under the table has to be able to hand it an answer to refuse.
    #119 compared in the loop body and counted rows, so a row hollowed to ``pass`` refused
    nothing while the count stayed put -- its own canary worked around that by counting
    refusals, which is this checker's job done by hand.
    """
    if gone != want:
        return [f"skips {sorted(gone)}, want {sorted(want)}"]
    return []


def spy_problems(seen, rows):
    """Whether the ignore callback was still consulted, once per row. Empty means it was.

    The table pins what the callback answers; this pins that the table still asks it.
    Replacing the loop's call with the row's own expectation kept every row green, because
    nothing else was consulting the callback.
    """
    if len(seen) != len(rows):
        return [f"the callback was consulted {len(seen)} times for {len(rows)} rows: a row "
                f"that answers from its own expectation is not a rule about the callback"]
    return []


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


def set_label_problems(pairs, want):
    """What a set's report labels got wrong, as sentences. Empty means they are honest.

    Two independent claims, so a control can only trip for the one it is about:

    - every label distinct. Two sets printing one identical line is the ambiguity the label
      exists to remove, and it is what `basename` produced for `alpha/s.json` and
      `beta/s.json`;
    - where `want` is given, each ``(path, expected)`` in it is what that path was labelled.
      This is the half that pins the repo-relative answer, which no fixture path reaches on
      its own because the fixtures are outside the repo.

    A pure function of what it is handed, for the reason every other checker here is one: the
    self-test can then feed it a collapse and watch it refuse, instead of counting that a
    comparison was reached.
    """
    problems = []
    seen = {}
    for path, label in pairs:
        if label in seen:
            problems.append(f"{path!r} and {seen[label]!r} both report as {label!r}, so the "
                            f"run says which sets ran without saying which is which")
        else:
            seen[label] = path
    for path, expected in (want or []):
        got = dict(pairs).get(path)
        if got != expected:
            problems.append(f"{path!r} is labelled {got!r}, want the repo-relative {expected!r}")
    return problems


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
     lambda real, work: ([(real[0][0], "alpha.json [A1,B1]"), real[1]], [], WANT_AB, [])),
    ("if miss != want_missing:",
     lambda real, work: (real, [], WANT_AB, ["Z9"])),
    ("if got != ids:",
     lambda real, work: (_wide_selection(work), [], [("wide.json", ["A1"])], [])),
    ("if len(set(files)) != len(files) or not all(os.path.isfile(p) for p in files):",
     lambda real, work: (_same_file_twice(real), [],
                         [("alpha.json", ["A1"]), ("alpha.json", ["A1"])], [])),
)


def _wide_selection(work):
    """One set whose trimmed file holds an id nobody asked for, labelled honestly.

    Isolated on purpose: the label matches what the caller says the set will run, the ids
    reported missing are the ones the caller expects, and there is one distinct file, so
    only the file-content comparison can object. This is the comparison that keeps
    ``select_sets`` from writing the untrimmed document -- and with it gone the runner runs
    mutants nobody requested behind a label naming one of them, which is #118's symptom
    turned around.
    """
    path = os.path.join(work, "wide.json")
    with open(path, "w", encoding="utf-8") as fh:
        json.dump({"package": "./internal/x/",
                   "mutants": [{"id": i, "file": "x.go", "old": "a", "new": "b"}
                               for i in ("A1", "A2")]}, fh)
    return [(path, "wide.json [A1]")]


def _same_file_twice(real):
    """Two sets handed the same trimmed file, each labelled honestly.

    Nothing else in the checker notices: the labels match, the contents match what each set
    is said to run, and no id is missing. Only the one-distinct-file-per-set rule can say
    that one file is standing in for two runs.
    """
    path, label = real[0]
    return [(path, label), (path, label)]


def _tally(caught=0, survived=0, invalid=0, equivalent=0):
    """One set's report, with every column present.

    A tally missing a column is a KeyError inside the sum, so a fixture that cannot raise is
    not describing what a real set returns.
    """
    return {"caught": caught, "survived": survived, "invalid": invalid,
            "equivalent": equivalent}


def _fake_sets(n):
    """n sets to hand a runner, with no files behind them.

    run_selection never opens a path -- it hands each pair to the runner and adds up what
    comes back -- so the fixture is pairs of strings and the rule is about the loop and
    nothing else.
    """
    return [(f"/fixture/set{i}.json", f"set{i}.json [M{i}]") for i in range(n)]


def _drive(sets, tallies):
    """``run_selection`` over ``sets``, with a runner that answers ``tallies`` in order.

    Returns ``(totals, ran_everything, calls)``. The runner records what it was handed, so
    "the loop ran and threw the answer away" and "the loop skipped a set" are both visible
    from outside the function -- nothing here reads its internals, which is the point of
    extracting it: the same five edits that took four rounds of text audit to name are now
    ordinary failing observations.
    """
    calls = []
    answers = iter(tallies)

    def run(path, label):
        calls.append((path, label))
        return next(answers)

    totals, ran_all = run_selection(sets, run)
    return totals, ran_all, calls


def selection_run_problems(totals, ran_all, calls, want_totals, want_calls, want_ran):
    """What one ``run_selection`` drive got wrong, as sentences. Empty means it ran honestly.

    Three independent claims, so a control can only trip for the one it is about:

    - ``ran_all`` is what was asked. This is the stop signal: a set that could not be run at
      all has to reach ``main`` as "no honest TOTAL exists", and a run that reported success
      where the stop belongs is the reviewer's `return 0` -- nothing ran, nothing printed,
      exit 0.
    - every set was handed to the runner, in order. A ``break`` or a ``continue`` in the loop
      is #118 arriving one layer down, and the totals alone cannot show it: a tally that was
      never asked for is indistinguishable from one that was skipped unless the calls are
      compared.
    - the totals are the sum of every set's tally. A ``pass`` where the addition happens
      leaves the calls exactly right and every column at zero, which is a report that reads
      as a clean battery.

    A pure function of what the drive observed, for the reason every other checker here is
    one: the self-test can hand it a skipped set and watch it refuse, instead of counting
    that a comparison was reached.
    """
    problems = []
    if ran_all != want_ran:
        problems.append(f"the run reported ran_everything={ran_all} and the fixture asked "
                        f"for {want_ran}, so whether a set was skipped is not being told to "
                        f"the caller")
    if calls != want_calls:
        problems.append(f"the runner was handed {calls} for a selection of {want_calls}, so "
                        f"a set was skipped, repeated, or run out of order")
    if totals != want_totals:
        problems.append(f"the totals came out {totals}, want {want_totals} -- the sum of "
                        f"every set's tally, so a set ran and its answer was thrown away")
    return problems

# What main's --only block has to say to be honest. The self-test runs no battery, so the
# flag's shape is audited as text -- the way check-mutations.py pins its own summary line,
# for the same reason: a pinned call buys nothing if the next line throws its result away,
# and throwing the result away is exactly where #118 lived.
ONLY_CALL = "sets, missing = select_sets(sets, wanted, work)"
# The hand-off: the selection leaves main here and the running happens in run_selection.
# Pinned as a line because everything after it is glue, and everything before it is the
# selection itself -- which is what the deny list below protects.
ONLY_RUN = "totals, ran_all = run_selection(sets, run_set)"
ONLY_BLOCK_LINES = (
    # `if args.only:` is falsy for --only "", which then falls through to the else and runs
    # the ENTIRE battery behind a flag that asked for nothing.
    "if args.only is not None:",
    "if not wanted:",
    ONLY_CALL,
    # Nothing matched: refuse. Without this the run prints `TOTAL 0 caught, 0 survived`.
    "if not sets:",
    # The selection has to reach the runner exactly as the parser produced it. Without the
    # hand-off pinned, the block can compute a correct selection and drop it on the floor.
    ONLY_RUN,
)
# Every line the --only block is allowed to contain that touches the selection. Deny by
# default, because matching assignment syntax only catches a rebinding that LOOKS like one:
# round 3 restored the ticket's exact symptom with `del sets[1:]` and the guard stayed
# green, and `sets.pop()`, `sets[:] = sets[:1]`, `sets, missing = sets[:1], missing` and
# `missing.clear()` all throw the selection away without matching `^(sets|missing) = `.
# Anything in the block that names `sets` or `missing` and is not listed here is a problem,
# including shapes nobody has invented yet.
ONLY_BLOCK_HONEST = (
    ONLY_CALL,
    ONLY_RUN,
    "if not sets:",
    'sys.exit(f"none of {sorted(wanted)} is in {scope(args.sets)}")',
    'sys.exit(f"--only {args.only!r} names no mutant ids")',
    # The `else` arm of the --only branch. It names `sets`, so the audited region brings it
    # inside -- it is the no-flag path relabelling the full set list, not a mutation of a
    # selection. Listed rather than exempted by region, so a truncation hiding on the same
    # line (`sets[:1]` inside the comprehension) still trips the deny list.
    "sets = [(p, set_label(p)) for p in sets]",
    # Control flow the block is allowed to contain, listed rather than exempted. A `return 0`
    # one line above the hand-off prints no TOTAL and exits 0 on a run that never started --
    # strictly worse than #118 -- and it names neither `sets` nor `missing`, so the only way
    # to hold it is to treat an unlisted effect as a problem.
    "return 2",
    'sys.exit(str(err))',
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
    # The four shapes round 3 actually used to get past an auditor that matched assignment
    # syntax. Each is the ticket's symptom, and each is here so it stays the ticket's
    # symptom rather than a hypothetical.
    ("throws sets away with `del`, which is not an assignment",
     lambda s: s.replace(ONLY_CALL, ONLY_CALL + "\n            del sets[1:]", 1)),
    ("rebinds both results as a tuple, leaving the pinned call's text intact",
     lambda s: s.replace(ONLY_CALL, ONLY_CALL + "\n            sets, missing = sets[:1], missing", 1)),
    ("empties the missing list in place",
     lambda s: s.replace(ONLY_CALL, ONLY_CALL + "\n            missing.clear()", 1)),
    ("truncates the selection by slice assignment",
     lambda s: s.replace(ONLY_CALL, ONLY_CALL + "\n            sets[:] = sets[:1]", 1)),
    # Round 4's blocker: the audited region used to stop above the code that consumes the
    # selection, so anything placed in that gap was outside the audit entirely. These three
    # land in the gap that remains -- one line above the hand-off -- and are caught only
    # because the region runs to the end of the block. They are the region's own regression
    # test: narrowing it again fails here instead of quietly restoring #118.
    ("throws the selection away one line above the hand-off to the runner",
     lambda s: s.replace("        " + ONLY_RUN,
                         "        sets = sets[:1]\n        " + ONLY_RUN, 1)),
    ("swallows the unmatched ids one line above the hand-off to the runner",
     lambda s: s.replace("        " + ONLY_RUN,
                         "        missing = []\n        " + ONLY_RUN, 1)),
    ("drops the last set one line above the hand-off to the runner",
     lambda s: s.replace("        " + ONLY_RUN,
                         "        del sets[1:]\n        " + ONLY_RUN, 1)),
    # Control flow, which names neither `sets` nor `missing` and so was invisible to a deny
    # list keyed on those two words. Each of these is valid Python at this head and each was
    # measured to leave the self-test green before the predicate was widened: `return 0`
    # above the hand-off runs nothing, prints no TOTAL and exits 0, so a scripted
    # `mutate.py --only ... && ...` proceeds on a run that never started.
    ("exits 0 one line above the hand-off, so nothing runs and the shell sees success",
     lambda s: s.replace("        " + ONLY_RUN,
                         "        return 0\n        " + ONLY_RUN, 1)),
    ("exits the process 0 at the hand-off, reporting a complete run that never happened",
     lambda s: s.replace("        " + ONLY_RUN,
                         "        sys.exit(0)\n        " + ONLY_RUN, 1)),
    ("replaces the hand-off's effect with `pass`",
     lambda s: s.replace("        " + ONLY_RUN,
                         "        pass\n        " + ONLY_RUN, 1)),
    # What is deliberately NOT a control here: `break`, `continue`, and a `pass` standing in
    # for the sum. Those were the loop body's shapes, and the loop body no longer lives in
    # main -- it is run_selection, which the self-test executes against a fake runner, so
    # each of them fails an assertion in SELECTION_RUN_CONTROLS instead of a line this
    # auditor has to think to deny. A text control for a shape that cannot be written in
    # valid Python at this position would only inflate the count.
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
    # The region ends where the `try` does -- `    finally:` -- not at the branch and not at
    # the loop header. Round 4 ended it at `for path, label in sets:`, which left the loop
    # BODY outside the audit, and a `break` as the loop's last statement is #118 verbatim at
    # a green self-test. The loop has since moved into `run_selection`, which the self-test
    # RUNS against a fake runner rather than reads, so the text audit is back to what a text
    # audit is good at: the argument parsing and the glue that hands the selection over. It
    # still runs to the end of the block, because the lines after the hand-off are where a
    # `return 0` can land on a run that never started.
    #
    # The guard on the anchor is load-bearing: without it, editing the boundary itself widens
    # the region silently to the end of `main` and the report then complains about lines
    # nobody broke -- which sends the next reader hunting in the wrong place.
    end = "    finally:"
    if end not in rest:
        return ["the --only block no longer ends where the selection is handed to the "
                "runner, so the audited region cannot be located. Restore the boundary "
                "rather than trusting whatever this auditor then finds"]
    block = "if args.only" + rest.split(end)[0]
    problems = [f"the --only block no longer contains {line!r}"
                for line in ONLY_BLOCK_LINES if line not in block]
    # Deny by default. The pinned call is worth nothing if a later line throws what it
    # returned away, and that line does not have to look like an assignment to do it -- nor
    # does it have to name the selection at all. Control flow is the second way to throw a
    # selection away: `break` ends the run, `return 0` ends it quietly, `pass` deletes a sum,
    # and none of the three contains the word `sets`. So the unit being denied is a line's
    # EFFECT, and every admitted effect is in ONLY_BLOCK_HONEST by name.
    for raw in block.splitlines():
        code = raw.split("#")[0].strip()
        if not code or not (re.search(r"\b(?:sets|missing)\b", code)
                            or re.match(r"^(?:break|continue|return\b|raise\b|sys\.exit\("
                                        r"|pass\b|del\b)", code)):
            continue
        if code not in ONLY_BLOCK_HONEST:
            problems.append(f"the --only block does something unrecognised with the "
                            f"selection: {code!r} -- only the lines in ONLY_BLOCK_HONEST "
                            f"may touch it, so a dropped set or a swallowed id cannot hide "
                            f"in a shape this auditor has not seen before")
    if "if missing:" not in src:
        problems.append("main no longer reports a --only request it could only partly "
                        "answer")
    elif "return 1" not in src.split("if missing:")[-1]:
        problems.append("main's `if missing:` branch no longer exits non-zero, so a "
                        "scripted `mutate.py --only ... && ...` still proceeds")
    return problems


def case_problems(kind, detail, want, needle, absent):
    """What one captured ``go test`` case got wrong, as sentences. Empty means it agrees.

    A pure function of the verdict and what the case expected, so the self-test's loop body
    is a single ``rule()`` call and the count leaves the same place as the assertion.
    Asserting in the loop instead meant ``if kind != want:`` -> ``if False:`` kept the
    pinned number exactly where it was while emptying every case: the tally counted
    iterations, not assertions. With the comparison in here, hollowing it makes the control
    below report that the checker said nothing.
    """
    if kind != want:
        return [f"got {kind!r}, want {want!r}"]
    if needle and needle not in detail:
        return [f"detail {detail!r} does not mention {needle!r}"]
    if absent and absent in detail:
        return [f"detail {detail!r} says {absent!r}, which this case exists to rule out"]
    if kind not in VERDICT_TAG:
        return [f"kind {kind!r} has no report label"]
    return []


def note_problems(got, want):
    """What one ``equivalence_note`` answer got wrong. Which value counts as "marked" is
    the rule a typo in a set hits, so each spelling is named rather than averaged."""
    return [] if got == want else [f"returned {got!r}, want {want!r}"]


def tag_problems(kind, tag):
    """Whether one report label still fits the report's column.

    The tally is read back out of the report, which is what lets this change claim the
    output is unchanged. Narrowing a label to "CAUGHT" keeps every verdict assertion green
    and moves every line in the report, so the width is pinned rather than trusted.
    """
    return [] if len(tag) == 11 else [f"VERDICT_TAG[{kind!r}] = {tag!r} is {len(tag)} "
                                      f"wide, want 11 (the report's column)"]


def label_problems(kind):
    """Whether a verdict kind has a label to print, or would print `None` mid-line."""
    return [] if kind in VERDICT_TAG else [f"verdict kind {kind!r} has no report label"]


# Every comparison above, with an answer it has to reject. A control is what tells a deleted
# comparison from one that merely had nothing to say, and it is why the verdict half needs
# no source pins: `control()` reports "it said nothing" the moment a comparison stops
# firing, which is the same fact a substring pin gives and one the pin cannot give wrongly.
CASE_CONTROLS = (
    ("a case whose verdict disagrees with it",
     lambda: case_problems("caught", "by a test", "survived", "", None)),
    ("a case naming a test its detail does not mention",
     lambda: case_problems("caught", "by a test", "caught", "TestAbsent", None)),
    ("a case whose detail says the very thing it rules out",
     lambda: case_problems("caught", "by a test", "caught", "", "a test")),
    ("a verdict with no report label",
     lambda: case_problems("nonsense", "", "nonsense", "", None)),
)


class Problems(list):
    """A problem list that remembers how many problems it was handed.

    The self-test returns its problems, so the list was the only record that anything was
    wrong -- and `bad.clear()` on the way out therefore turned the whole run green with
    every failure discarded, while the pinned rule count stayed exactly where it was. The
    tally travels back as its own value now, and `self_test_verdict` compares the two: a
    list that has been emptied cannot lower the number it was handed.
    """

    def __init__(self):
        super().__init__()
        self.raised = 0

    def append(self, item):
        list.append(self, item)
        self.raised += 1


# How many verdict rules the self-test is expected to run: one per captured case, one per
# `equivalence_note` spelling, one per report label (width and existence), and one control
# per comparison in CASE_CONTROLS plus the three single-comparison controls. Pinned for the
# same reason the selection count is -- a deleted rule is invisible to the rules that
# survive -- and counted off the run rather than guessed.
EXPECTED_VERDICT_RULES = 35

# How many selection rules the self-test is expected to run, counted off the list the pin
# prints rather than off arithmetic: five --only cases and one "does this case assert
# anything" per case, the fixture's own shape, two controls at the selection level,
# a pin and a control per comparison in CHECKER_CONTROLS, the audit of main's --only block,
# one control per entry in ONLY_BLOCK_CONTROLS. Pinned for the same reason the verdict count
# is -- a deleted rule is invisible to the rules that survive.
EXPECTED_SELECT_RULES = 47

# How many snapshot rules the self-test is expected to run: #119's nineteen, counted the same
# way (one per table row, one per fixture that builds a tree and asks what the snapshot did
# with it, one control per comparison that can be handed a wrong answer). Re-derived from the
# run at this head, not carried over from either side of the merge: main printed
# `EXPECTED_SNAPSHOT_RULES = 19` beside a verdict count of 22, and this branch had already
# moved the verdict half to 35, so a number taken from either file would be a count that
# quietly means something else -- the defect #118 was opened about.
#
# The twentieth is the git-on-PATH precondition, which #119 counted. The twenty-first is one
# this branch's own attack battery bought: broadening the ignore loop's short-circuit to
# `startswith` answered all eight table rows exactly as honest code does, so the ported half
# reported nineteen rules holding while unable to see the mutation -- and on a tree with both
# `.venv` and `.venv-old` in .gitignore it copies the second one back.
EXPECTED_SNAPSHOT_RULES = 21


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

    bad = Problems()
    vrules = []
    rules = []
    srules = []
    for name, out, rc, note, want, needle, *rest in cases:
        absent = rest[0] if rest else None
        kind, detail = verdict(out, rc, note)
        rule(vrules, bad, f"verdict {name}",
             case_problems(kind, detail, want, needle, absent))

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
        rule(vrules, bad, f"equivalence_note {name}",
             note_problems(equivalence_note(entry), want))

    # The report is a fixed column, and "the tally is read from it" is the reason this
    # change claims the output is unchanged. Narrowing a label to "CAUGHT" keeps every
    # verdict assertion green and moves every line in the report, so the width is pinned
    # rather than trusted to the strings being left alone.
    for kind, tag in sorted(VERDICT_TAG.items()):
        rule(vrules, bad, f"report column width for {kind}", tag_problems(kind, tag))

    # Every label must be reachable, or a verdict could print `None` mid-line.
    for kind in ("caught", "survived", "invalid", "equivalent"):
        rule(vrules, bad, f"report label exists for {kind}", label_problems(kind))

    # The controls. Each one hands a checker an answer it must refuse; a checker that says
    # nothing about it is reported, which is the only way to tell a deleted comparison from
    # a clean run.
    for name, wrong in CASE_CONTROLS:
        control(vrules, bad, f"control: {name}", wrong(),
                "so a verdict case can be wrong and stay silent about it")
    control(vrules, bad, "control: a note answer that disagrees",
            note_problems("a note", None), "so a marked entry can return anything")
    control(vrules, bad, "control: a label too wide for the column",
            tag_problems("caught", "CAUGHT"), "so the report's column is not pinned")
    control(vrules, bad, "control: a verdict kind with no label",
            label_problems("nonsense"), "so a verdict could print None mid-line")

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
        # set_label's relative half. Every fixture lives under a TemporaryDirectory, which is
        # outside SRC, so before `root` was a parameter each case took the basename arm and
        # the relative half of the function was run by nothing -- `return os.path.basename(path)`
        # kept the self-test green at 35/31. These two drive it through the real function with
        # a root the fixtures are inside, and the control feeds the checker the collapse the
        # fix removed.
        inside = os.path.join(SRC, "scripts", "mutations", "x.json")
        rule(rules, bad, "a set inside the repo is labelled repo-relative",
             set_label_problems([(inside, set_label(inside, SRC)),
                             (alpha, set_label(alpha, os.path.dirname(alpha)))],
                            [(inside, os.path.join("scripts", "mutations", "x.json"))]))
        rule(rules, bad, "two sets sharing a basename get distinct labels",
             set_label_problems([(os.path.join(work, "alpha", "s.json"),
                              set_label(os.path.join(work, "alpha", "s.json"), work)),
                             (os.path.join(work, "beta", "s.json"),
                              set_label(os.path.join(work, "beta", "s.json"), work))],
                            None))
        control(rules, bad, "control: labels collapse to the basename",
                set_label_problems([(os.path.join(work, "alpha", "s.json"), "s.json"),
                                (os.path.join(work, "beta", "s.json"), "s.json")], None),
                "so two sets print one identical line again")
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
                    selection_problems(*wrong(real, work)) if len(real) == 2 else None,
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

    # The selection, consumed. Everything above decides WHICH sets a run agreed to run; this
    # decides whether it then ran them, which is where #118 actually lived. Driven rather
    # than read, because four rounds of auditing that loop as text never closed it -- each
    # round the guard was green with the symptom live, since a deny list can only refuse the
    # shapes someone thought to write down. Run against a fake runner, the shapes become
    # observations: a set nobody was handed, a tally nobody added, a stop nobody reported.
    three = _fake_sets(3)
    parts = [_tally(caught=1), _tally(survived=2), _tally(caught=3, invalid=1)]
    summed = _tally(caught=4, survived=2, invalid=1)
    got, ran_all, calls = _drive(three, parts)
    rule(rules, bad, "run_selection runs every set it is handed and sums every tally",
         selection_run_problems(got, ran_all, calls, summed, three, True))

    # A set that could not be run at all stops the run, and the caller is told. Without the
    # second half the battery prints a TOTAL over the sets that did run, which is the same
    # lie in a different font.
    got2, ran2, calls2 = _drive(three, [_tally(caught=1), None, _tally(caught=9)])
    rule(rules, bad, "run_selection stops when a set cannot be run, and says so",
         selection_run_problems(got2, ran2, calls2, _tally(caught=1), three[:2], False))

    # The degenerate selection. Nothing to run is not a failed run, and the totals have to be
    # the four zeros the report prints rather than whatever an uninitialised dict holds.
    got3, ran3, calls3 = _drive([], [])
    rule(rules, bad, "an empty selection runs nothing and is not a stopped run",
         selection_run_problems(got3, ran3, calls3, _tally(), [], True))

    # The controls: one broken claim each, the other two left honest on the same input, so a
    # control going green means that comparison is gone rather than that a neighbour is
    # covering for it. These are the reviewer's five measured edits, minus the two that can
    # no longer be written in main's block at all.
    control(rules, bad, "control: a set the loop never handed to the runner",
            selection_run_problems(summed, True, calls[:2], summed, three, True),
            "so a break or a continue in the loop is invisible again")
    control(rules, bad, "control: a tally that was never summed",
            selection_run_problems(_tally(), True, three, summed, three, True),
            "so a battery of zeroes reads as a clean run")
    control(rules, bad, "control: a stopped run reported as complete",
            selection_run_problems(got2, True, calls2, _tally(caught=1), three[:2], False),
            "so a run that could not start exits 0 and prints nothing")
    control(rules, bad, "control: a complete run reported as stopped",
            selection_run_problems(got, False, calls, summed, three, True),
            "so the stop signal is only compared in one direction")

    # ---- the snapshot --------------------------------------------------------------
    # What a battery costs is why these are here: the run that found #117 died in copytree
    # before it had applied one mutant, and every check that runs earlier -- the checker,
    # `make verify`, CI's `mutation sets` job -- copies nothing at all, so each of them said
    # the tree was fine.
    #
    # These are #119's rules, ported rather than re-derived: the bodies are its text, changed
    # only to take the problem list of the rule that runs them. A port that rewrote an
    # assertion would have to be re-reviewed line by line, and what this PR claims about them
    # is that they still COUNT -- that deleting one moves the pinned number, which is what
    # run_rule buys and what the two counts in the summary line now say apart.
    #
    # Everything below needs git, because the fixtures are repositories: what gets skipped IS
    # the repository's answer. A missing git fails rather than skips -- a guard that stepped
    # around a missing binary reported "11 snapshot rules hold" while the one rule that
    # proves the fix never ran.

    def git_run(args, cwd, problems):
        """A fixture's git call: isolated the way the real one is, and loud when it fails.

        Isolation matters here as much as it does in `ignored_paths` -- a wrapper or a hook
        that exports GIT_DIR makes `git init` fail somewhere else, and that turns a healthy
        tree into a red self-test with a message about the wrong thing.
        """
        try:
            done = subprocess.run(["git", *args], cwd=cwd, env=git_env(),
                                  stdout=subprocess.DEVNULL,
                                  stderr=subprocess.DEVNULL)
        except OSError as err:
            # No git at all. A traceback here is the same "looks like a Python bug" the
            # ticket complains about, and it buries the sentence above it.
            problems.append(f"git {' '.join(args)} could not run in {cwd} ({err}), so the "
                           "fixture below is not a repository and whatever it appears to check "
                       "is not what the snapshot does")
            return None
        if done.returncode != 0:
            problems.append(f"git {' '.join(args)} failed in {cwd} (exit {done.returncode}), so "
                           "the fixture below is not a repository and whatever it appears to "
                       "check is not what the snapshot does")
        return done.returncode

    def chrome_profile(where, name, problems):
        """A directory holding the two things of a Chrome profile that copytree cannot copy:
        a socket, and a symlink whose target is gone.

        Nothing here is swallowed. A profile that failed to materialise turns a copy rule
        into a copy of an ordinary directory, which passes whatever the code does -- the
        silent-pass shape, arriving through a platform instead of through a bug.
        """
        slot = os.path.join(where, name)
        os.makedirs(slot, exist_ok=True)
        with open(os.path.join(slot, "note.txt"), "w") as fh:
            fh.write("chrome wrote this\n")
        try:
            os.symlink("/no/such/target/anywhere", os.path.join(slot, "SingletonCookie"))
            listener = socket.socket(socket.AF_UNIX)
            listener.bind(os.path.join(slot, "SingletonSocket"))
            listener.close()
        except OSError as err:
            problems.append(f"this platform will not build the profile in {name}/ ({err}), so "
                           "the copy rules below would be copying an ordinary directory")
        if not (os.path.islink(os.path.join(slot, "SingletonCookie"))
                and os.path.lexists(os.path.join(slot, "SingletonSocket"))):
            problems.append(f"the profile in {name}/ has no dead symlink and no socket, so the "
                           "copy below proves nothing about a Chrome profile")
        return slot

    # The ignore callback, behind a spy. The table pins what it answers; the spy pins that
    # the table still asks it. Replacing the loop's call with the row's own expectation kept
    # the count and the run green, because nothing else was consulting the callback.
    real_cb = snapshot_ignore("/repo", ignored=("testing/capture/runs/", "plock"))
    seen = []

    def ignore_cb(directory, names):
        seen.append(directory)
        return real_cb(directory, names)

    rows = [
        ("the never-copied names go at the root", "/repo",
         [".git", "bin", ".cortexkit", "__pycache__", "go.mod"],
         {".git", "bin", ".cortexkit", "__pycache__"}),
        # `bin` at depth is a package like any other. Matching the root names by basename
        # anywhere is how a tracked internal/platform/bin/ left a battery's copy with nothing
        # raised and nothing printed, and this row is what keeps that fail-open shut.
        ("only __pycache__ is matched at any depth", "/repo/internal/web",
         ["__pycache__", "bin", "server.go"], {"__pycache__"}),
        ("an ignored directory goes", "/repo/testing/capture",
         ["cdp.py", "runs"], {"runs"}),
        ("the directory CONTAINING an ignored one does not", "/repo/testing",
         ["capture", "README.md"], set()),
        ("and neither does the root", "/repo",
         ["internal", "testing", "scripts"], set()),
        ("an ignored file goes", "/repo", ["plock", "go.mod"], {"plock"}),
        # Equality, never a prefix: `plock2` and `plock.go` are source sitting beside an
        # ignored `plock`, and a startswith() drops them just as quietly as the basename rule.
        ("a name merely STARTING with an ignored one stays", "/repo",
         ["plock", "plock2", "plock.go", "go.mod"], {"plock"}),
        ("a same-named file somewhere else stays", "/repo/internal/web",
         ["plock", "server.go"], set()),
    ]

    def git_tree(where, problems, ignore=("runs/",)):
        """A repository, not just a directory: git answers for tracked state, and an EMPTY
        ignored directory is not reported at all, because git has no notion of an empty
        directory. Both halves belong in the fixture or the rule passes for the wrong reason.
        `keep/` is never ignored and is what each rule says no about.
        """
        os.makedirs(where, exist_ok=True)
        with open(os.path.join(where, ".gitignore"), "w") as fh:
            fh.write("".join(f"{p}\n" for p in ignore))
        for name in ("keep",) + tuple(ignore):
            slot = os.path.join(where, name.rstrip("/"))
            os.makedirs(slot, exist_ok=True)
            with open(os.path.join(slot, "capture.png"), "w") as fh:
                fh.write("\n")
        git_run(["init", "-q"], where, problems)
        return where

    # The table, run through the callback rather than answered from its own expectation, and
    # the spy that pins the table still asks it. #119 compared inside the loop and counted
    # rows; the comparison is a checker now so the control under it can hand it an answer to
    # refuse, which is the only way to tell eight rows that passed from eight rows that
    # cannot fail.
    for name, directory, names, want in rows:
        rule(srules, bad, f"snapshot ignores {name}",
             ignore_row_problems(set(ignore_cb(directory, list(names))), want))
    rule(srules, bad, "the table consulted the ignore callback once per row",
         spy_problems(seen, rows))
    control(srules, bad, "control: a row that answers from its own expectation",
            spy_problems([], rows),
            "so the table is not being answered by the callback at all")
    control(srules, bad, "control: a snapshot row that cannot refuse a wrong answer",
            [p for _n, _d, _nm, want in rows
             for p in ignore_row_problems(set(want) ^ {"__not_a_real_file_name__"}, want)],
            "so eight rows counting happily is not eight rules")

    # The short-circuit that skips an already-dropped name is equality too, and the table
    # above cannot see it: broadening it to `startswith` answers all eight rows identically
    # (measured), because none of them walks two ignored names where one is a prefix of the
    # other. The product does have such pairs -- `.venv` and `.venv-old`, `plock` and a
    # `plock2` someone added to .gitignore -- and there the broadened test keeps the longer
    # name OUT of `gone`, so it is copied. That is #117 arriving again, a profile back in the
    # tree and a socket in the profile, from the branch of the loop nobody was aiming at.
    pair_cb = snapshot_ignore("/repo", ignored=("plock", "plock2"))
    rule(srules, bad, "snapshot ignores both of two ignored names where one starts with the other",
         ignore_row_problems(set(pair_cb("/repo", ["plock", "plock2", "go.mod"])),
                             {"plock", "plock2"}))

    def _snap01(bad):
        if not shutil.which("git"):
            bad.append("git is not on PATH, so the snapshot rules have no repository to check "
                       "against and the fix this file exists for would go untested")
    run_rule(srules, bad,
               "snapshot: git is on PATH, so the snapshot rules have a repository to answer from", _snap01)

    # Which uncopyable path is fatal, over five real ones. Decided by kind rather than by
    # trying to read, because as root nothing is unreadable and a permission-based fixture
    # would be a rule that cannot fail.

    def _snap02(bad):
        with tempfile.TemporaryDirectory() as work:
            regular = os.path.join(work, "server.go")
            with open(regular, "w") as fh:
                fh.write("package web\n")
            listener = socket.socket(socket.AF_UNIX)
            listener.bind(os.path.join(work, "SingletonSocket"))
            listener.close()
            os.symlink("/no/such/target/anywhere", os.path.join(work, "SingletonCookie"))
            # A link whose target exists is a tracked symlink: copytree follows it, so a copy
            # failure on one has taken a real file out of the run. The first version of this
            # sorting skipped every symlink, which made the whole branch fail open.
            os.symlink(regular, os.path.join(work, "policy.txt"))
            skipped, fatal = [], []
            note_uncopyable(regular, "EACCES", skipped, fatal)
            note_uncopyable(os.path.join(work, "SingletonSocket"), "ENXIO", skipped, fatal)
            note_uncopyable(os.path.join(work, "SingletonCookie"), "ENOENT", skipped, fatal)
            note_uncopyable(os.path.join(work, "policy.txt"), "EACCES", skipped, fatal)
            note_uncopyable(work, "EACCES", skipped, fatal)
            if len(fatal) != 3 or len(skipped) != 2:
                bad.append(f"an uncopyable path sorted into fatal={fatal} skipped={skipped}, "
                           "want the regular file, the live symlink and the directory fatal, "
                           "the socket and the dead symlink skipped")
    run_rule(srules, bad,
               "snapshot: an uncopyable path is sorted into fatal and skipped by what it is", _snap02)

    # The acting half. The rule above proves the sorting; nothing proved that a fatal sort
    # actually stops a run, because reaching it needs an unreadable file and nothing is
    # unreadable as root -- with the refusal deleted, the self-test printed "14 snapshot
    # rules hold". So `snapshot` takes its copy as an argument and the self-test hands it one
    # that fails on a named file. The fixture is an empty repository so the tracked check
    # below cannot be what raises: the reason for the refusal is asserted, not assumed.

    def _snap03(bad):
        with tempfile.TemporaryDirectory() as work:
            src = os.path.join(work, "src")
            os.makedirs(os.path.join(src, "internal"))
            with open(os.path.join(src, "internal", "locked.go"), "w") as fh:
                fh.write("package web\n")
            git_run(["init", "-q"], src, bad)

            def refusing_copy(s, d, ignore):
                raise shutil.Error([(os.path.join(s, "internal", "locked.go"),
                                     os.path.join(d, "locked.go"),
                                     "[Errno 13] Permission denied")])

            try:
                snapshot(src, os.path.join(work, "dst"), copy=refusing_copy)
                bad.append("a source file that could not be copied did not stop the snapshot: "
                           "the run would have gone on over a tree missing that file, and "
                           "reported every mutant caught against it")
            except SnapshotError as err:
                if "could not copy source" not in str(err):
                    bad.append(f"the snapshot refused for the wrong reason: {err}")

            def socket_copy(s, d, ignore):
                raise shutil.Error([(os.path.join(s, "SingletonSocket"),
                                     os.path.join(d, "SingletonSocket"),
                                     "[Errno 6] No such device or address")])

            try:
                left = snapshot(src, os.path.join(work, "dst2"), copy=socket_copy)
            except SnapshotError as err:
                bad.append(f"a socket stopped the snapshot: {err}")
                left = []
            if not any("SingletonSocket" in p for p in left):
                bad.append("the socket was neither copied nor reported, so nobody is told the "
                           f"tree being tested is smaller than the one on disk (got {left})")
    run_rule(srules, bad,
               "snapshot: a source file that could not be copied stops the snapshot, and a socket does not", _snap03)

    # And the check that the copy HOLDS the repository -- acting, not sorting. A copy that
    # comes back without an error and without a tracked file is what a predicate bug, a file
    # that vanished mid-copy, and an unreadable symlink all look like from the outside, and
    # the sorting above cannot see any of them because nothing failed.

    def _snap04(bad):
        with tempfile.TemporaryDirectory() as work:
            src = os.path.join(work, "src")
            os.makedirs(os.path.join(src, "internal"))
            for name in ("server.go", "keep.go"):
                with open(os.path.join(src, "internal", name), "w") as fh:
                    fh.write("package web\n")
            git_run(["init", "-q"], src, bad)
            git_run(["add", "internal/server.go", "internal/keep.go"], src, bad)
            # Tracked AND deliberately not copied: `.cortexkit/.gitignore` is exactly this shape
            # in this repository, and it is the case that made the exemption necessary. Without
            # an assertion both ways the exemption is unpinned -- too wide silently drops source,
            # too narrow refuses a run that should have gone ahead.
            os.makedirs(os.path.join(src, ".cortexkit"), exist_ok=True)
            with open(os.path.join(src, ".cortexkit", ".gitignore"), "w") as fh:
                fh.write("state\n")
            git_run(["add", ".cortexkit/.gitignore"], src, bad)

            def dropping_copy(s, d, ignore):
                shutil.copytree(s, d, ignore=ignore)
                os.remove(os.path.join(d, "internal", "keep.go"))

            try:
                snapshot(src, os.path.join(work, "dst"), copy=dropping_copy)
                bad.append("a snapshot missing a tracked file was accepted: the battery would "
                           "have run over a tree with that file gone and reported its mutants "
                           "caught against nothing")
            except SnapshotError as err:
                if "missing" not in str(err):
                    bad.append(f"the snapshot refused for the wrong reason: {err}")
                if ".cortexkit" in str(err):
                    bad.append("the snapshot refused over a file under a never-copied root name,"
                               f" which is excluded on purpose: {err}")
    run_rule(srules, bad,
               "snapshot: a copy that came back without a tracked file is refused", _snap04)

    # The end-to-end half, through `snapshot` rather than a hand-built callback: the
    # composition is the fix, and a rule that rebuilt the callback for itself stayed green
    # with `main` reverted to the four hardcoded names.
    #
    # `runs` is what the repository ignores and `other` is not. The first is the bug that
    # happened; the second is the guard the ticket asked for in as many words -- "a tree
    # containing an UNIGNORED directory with a socket in it" -- and it is the class rather
    # than the instance, because the next one will not be named runs/.

    def _snap05(bad):
        with tempfile.TemporaryDirectory() as work:
            src = os.path.join(work, "src")
            os.makedirs(os.path.join(src, "internal", "web"))
            with open(os.path.join(src, "internal", "web", "server.go"), "w") as fh:
                fh.write("package web\n")
            for name in ("runs", "other"):
                chrome_profile(src, name, bad)
            with open(os.path.join(src, ".gitignore"), "w") as fh:
                fh.write("runs/\n")
            git_run(["init", "-q"], src, bad)
            # Tracked, so that the copy is checked against the repository rather than against
            # nothing: an empty tracked set makes `missing_source` pass for free. Only these two
            # are staged -- `git add -A` refuses to look at a socket, and the socket is the point.
            git_run(["add", "internal/web/server.go", "other/note.txt"], src, bad)
            tracked = tracked_paths(src)
            if not tracked:
                bad.append("the fixture has no tracked files, so nothing checked that the copy "
                           "holds the source")
            dst = os.path.join(work, "dst")
            try:
                skipped = snapshot(src, dst)
            except (shutil.Error, SnapshotError) as err:
                skipped = []
                bad.append(f"a tree with sockets in it did not snapshot: {err}")
            if os.path.exists(os.path.join(dst, "runs")):
                bad.append("the ignored directory was copied anyway")
            for rel in ("internal/web/server.go", "other/note.txt"):
                if not os.path.isfile(os.path.join(dst, rel)):
                    bad.append(f"{rel} did not reach the snapshot, so the copy is not the tree")
            for junk in ("SingletonCookie", "SingletonSocket"):
                if os.path.lexists(os.path.join(dst, "other", junk)):
                    bad.append(f"the uncopyable {junk} was copied anyway")
                if not any(junk in p for p in skipped):
                    bad.append(f"the uncopyable {junk} was neither copied nor reported, so "
                               "nobody is told the tree being tested is smaller than the one on "
                               f"disk (reported: {skipped})")
    run_rule(srules, bad,
               "snapshot: a tree with sockets in it snapshots, keeps the source, and reports what it left out", _snap05)

    # What git is asked for must be what gets skipped. The rule above gets its ignored paths
    # from `snapshot`, which asks git, so nothing so far would notice `ignored_paths` being
    # gutted into returning nothing -- and that is the other half of this bug, the half a
    # .gitignore that does not say `runs/` would hit.

    def _snap06(bad):
        if shutil.which("git"):
            with tempfile.TemporaryDirectory() as work:
                got = ignored_paths(git_tree(work, bad))
                if "runs/" not in got:
                    bad.append("ignored_paths did not report the directory the repository "
                               f"ignores: {got}")
                if "keep/" in got or "keep/capture.png" in got:
                    bad.append("ignored_paths reported a directory the repository does not "
                               f"ignore: {got}")
                # The `-z` in LS_IGNORED, and it is not cosmetic. Without it git answers
                # `"run\"s/"` for a directory holding a quote, that answer matches nothing on
                # disk, and the crash this function exists to prevent comes back. A name with an
                # accent tests the same quoting but compares differently on a volume that
                # normalises unicode, so the fixture uses the quote itself.
                quoted = ignored_paths(git_tree(work, bad, ignore=("runs/", 'run"s/')))
                if 'run"s/' not in quoted or any(p.startswith('"') for p in quoted):
                    bad.append("git's answer came back C-quoted, so an ignored path holding a "
                               f"quote matches nothing on disk: {quoted}")
                # And a name the locale cannot decode at all, which is the same failure arriving
                # from the other direction: `os.fsdecode` is what keeps it a round trip instead
                # of a UnicodeDecodeError before a single mutant runs.
                raw = os.path.join(work, "raw")
                weird = os.fsdecode(b"prof\xedle")
                os.makedirs(os.path.join(raw, weird), exist_ok=True)
                with open(os.path.join(raw, ".gitignore"), "wb") as fh:
                    fh.write(b"prof\xedle/\n")
                git_run(["init", "-q"], raw, bad)
                if weird + "/" not in ignored_paths(raw):
                    bad.append("an ignored path the locale cannot decode did not survive the "
                               f"round trip: {ignored_paths(raw)!r}")
    run_rule(srules, bad,
               "snapshot: git's ignored answer is what gets skipped, quoting and locale included", _snap06)

    # The answer must be a function of the repository and of nothing else. `GIT_DIR` is
    # exported by hooks and by wrapper scripts that call `make verify`, `GIT_CONFIG_PARAMETERS`
    # is how `git -c k=v` reaches a child, and a machine-wide core.excludesFile is not part of
    # this tree. Any one of them can make a battery copy the wrong tree; the last can also
    # make the assertion above pass or fail on someone's dotfiles, and an assertion about what
    # is NOT ignored is the one that has to be able to say no.

    def _snap07(bad):
        if shutil.which("git"):
            with tempfile.TemporaryDirectory() as work:
                # A real config file pointing at a real excludes list, because that is the
                # machine-wide setting this has to be immune to; the list and the config cannot be
                # the same file, since one is git config and the other is a pattern list.
                patterns = os.path.join(work, "excludes")
                with open(patterns, "w") as fh:
                    fh.write("keep/\n")
                cfg = os.path.join(work, "config")
                with open(cfg, "w") as fh:
                    fh.write(f"[core]\n\texcludesFile = {patterns}\n")
                repo = git_tree(os.path.join(work, "repo"), bad)
                saved = dict(os.environ)
                try:
                    os.environ["GIT_DIR"] = os.path.join(work, "not-a-repo")
                    os.environ["GIT_WORK_TREE"] = work
                    os.environ["GIT_CONFIG_GLOBAL"] = cfg
                    os.environ["GIT_CONFIG_SYSTEM"] = cfg
                    os.environ["GIT_CONFIG_PARAMETERS"] = (
                        f"'core.excludesFile'='{patterns}'")
                    got = ignored_paths(repo)
                finally:
                    os.environ.clear()
                    os.environ.update(saved)
                if "runs/" not in got:
                    bad.append(f"an ambient GIT_DIR changed what the snapshot skips: {got}")
                if "keep/" in got:
                    bad.append(f"a machine-wide excludes setting changed what the snapshot "
                               f"skips: {got}")
    run_rule(srules, bad,
               "snapshot: an ambient GIT_DIR or a machine-wide excludes file does not change what is skipped", _snap07)

    # A tree git cannot answer for is snapshotted exactly as it always was. There are two
    # ways not to answer and the function has a branch for each: nothing to run, and
    # something that runs and says no. Both have to fall back rather than refuse, because a
    # missing git is no reason a battery cannot run -- which is survivable now because of the
    # error sorting above, not in spite of it.

    def _snap08(bad):
        if ignored_paths(os.path.join(tempfile.gettempdir(), "no-such-tree-at-all")):
            bad.append("ignored_paths invented entries for a path that does not exist")
    run_rule(srules, bad,
               "snapshot: a tree git cannot answer for is snapshotted the way it always was", _snap08)

    def _snap09(bad):
        with tempfile.TemporaryDirectory() as work:
            # A git that answers AND refuses. The first version of this rule handed over
            # `sys.executable`, whose stdout is empty, so the rule passed with the refusal branch
            # deleted: an answer worth refusing is the only way to pin the branch that throws one
            # away, and a source name is the answer that must not be believed.
            stub = os.path.join(work, "stub.py")
            with open(stub, "w") as fh:
                fh.write("import sys\n"
                         "sys.stdout.buffer.write(b'server.go\\0')\n"
                         "sys.exit(3)\n")
            if ignored_paths(work, git=[sys.executable, stub]):
                bad.append("ignored_paths trusted an answer from a command that exited non-zero")
    run_rule(srules, bad,
               "snapshot: an answer from a git that exited non-zero is not believed", _snap09)
    # The two helpers every rule above is counted through are themselves attack surface: a
    # `pass` where `rules.append(name)` is, or an `if False:` inside control(), keeps every
    # number intact while emptying the checks. So both are canaried through a throwaway
    # list. Deliberately NOT one of the counted rules, for the reason the count pin is not
    # either: a check that counts itself moves the number it exists to hold still.
    probe_rules, probe_bad = [], []
    rule(probe_rules, probe_bad, "canary", ["a problem"])
    control(probe_rules, probe_bad, "canary", [], "it said nothing")
    control(probe_rules, probe_bad, "canary", ["a problem"], "it said nothing")
    # run_rule is the same attack surface with one more step: a body that reports a problem
    # has to have it reported, and a body that reports none has to add nothing. Without the
    # second arm, `run_rule = lambda rules, bad, name, body: rules.append((name, 0))` counts
    # every snapshot rule as passing while calling none of them.
    run_rule(probe_rules, probe_bad, "canary", lambda problems: problems.append("a problem"))
    run_rule(probe_rules, probe_bad, "canary", lambda problems: None)
    if len(probe_rules) != 5 or len(probe_bad) != 3:
        bad.append(f"the rule/control/run_rule helpers are hollow: they counted "
                   f"{len(probe_rules)} and reported {len(probe_bad)}, want 5 and 3 -- a rule "
                   f"that cannot fail is not made honest by counting it")

    # Reported, not adjudicated: the reach check used to live here, and a bare `return`
    # inserted anywhere above it skipped every rule and still exited 0. What decides
    # pass or fail is self_test_verdict(), outside the code whose reach it measures.
    return vrules, rules, srules, bad, bad.raised


def guard(ran, bad_out, why, got, want):
    """Record one verdict guard: the count and the assertion are the same call.

    Split out for the reason `rule()` and `control()` are: a loop that appends the name by
    hand, or whose comparison was turned into `if False:`, has to be unable to produce a
    healthy count. Here the recorded value is derived from the comparison, so a guard that
    stopped noticing is a guard that stopped being able to say it ran.
    """
    ran.append((why, 0 if got == want else 1))
    if got != want:
        bad_out.append(f"{why}: self_test_verdict returned {got}, want {want}")


def verdict_guard_problems():
    """The controls on self_test_verdict itself, computed OUTSIDE the code they police.

    These began at the tail of self_test(), and that was wrong: an early
    ``return vrules, rules, bad, bad.raised`` placed just above them left the self-test
    green at 35/37, because both counted lists are already complete by then and the reach
    check cannot see a block that was skipped. Measured, not argued. It is the same reason
    the reach check lives in the caller: a check cannot police the code allowed to skip it.

    Not counted as rules -- they hold the numbers the pins hold, and a check that counts
    itself moves the number it exists to hold. They are counted SEPARATELY, and the caller
    pins that number: an early `return` here, or the loop deleted, is a check that stopped
    existing rather than a check that found nothing, and only a reach pin tells those apart.
    Returns ``(ran, problems)``.
    """
    ran, bad_out = [], []
    # The three comparisons in self_test_verdict that no rule above can reach, because they
    # judge the verdict rather than a case. Each was reversible to `if False:` with the
    # self-test green at 35/31 -- including the one that decides the exit code. They cannot be
    # `rule()`s: a rule counts itself, and these exist to hold the numbers the pins hold. So
    # they append to `bad` directly (which bumps `raised` with it, since the verdict compares
    # the two) and they are deliberately absent from both counted lists.
    #
    # self_test_verdict prints, so each call is muted -- a control that leaked its own
    # "self-test FAILED:" into an otherwise green run would be read as the failure it is
    # asserting about.
    with contextlib.redirect_stdout(io.StringIO()):
        for why, probe, want in (
                # An absent verdict is the whole self-test silently not running, so this is
                # the comparison with the most cost in it. Each entry is a thunk and the call
                # is made inside a try below: with the guard hollowed, self_test_verdict(None)
                # does not return 0, it dies unpacking None. A crash is loud, but "the guard
                # was deleted" is the sentence worth printing.
                ("an absent verdict is refused", lambda: self_test_verdict(None), 1),
                ("a non-tuple verdict is refused",
                 lambda: self_test_verdict(("a", "b")), 1),
                # The list emptied on the way out: the count says failures happened, the list
                # says none did. Without this the `clear()` revert prints a green summary.
                ("an emptied problem list is refused",
                 lambda: self_test_verdict(([(f"r{i}", 0) for i in range(EXPECTED_VERDICT_RULES)],
                                    [(f"s{i}", 0) for i in range(EXPECTED_SELECT_RULES)],
                                    [(f"n{i}", 0) for i in range(EXPECTED_SNAPSHOT_RULES)],
                                    Problems(), 2)), 1),
                # A pair appended by hand counts itself and asserts nothing.
                ("a hand-forged rule is refused",
                 lambda: self_test_verdict(([(f"r{i}", 0) for i in range(EXPECTED_VERDICT_RULES - 1)]
                                    + [("forged",)],
                                    [(f"s{i}", 0) for i in range(EXPECTED_SELECT_RULES)],
                                    [(f"n{i}", 0) for i in range(EXPECTED_SNAPSHOT_RULES)],
                                    Problems(), 0)), 1),
                # The shape this merge was exposed to: a report still carrying two counts
                # where three are asked for. #119's snapshot rules lived in the function this
                # PR restructured, so a resolution that dropped them would have printed a
                # confident pair over a fix nothing checks. This is that report, and it has to
                # be refused rather than read as an older, smaller, valid verdict.
                ("a verdict with two counts instead of three is refused",
                 lambda: self_test_verdict(([(f"r{i}", 0) for i in range(EXPECTED_VERDICT_RULES)],
                                    [(f"s{i}", 0) for i in range(EXPECTED_SELECT_RULES)],
                                    Problems(), 0)), 1),
                # And the honest report still has to pass, or the four above pass by always
                # saying no.
                ("a complete verdict still passes",
                 lambda: self_test_verdict(([(f"r{i}", 0) for i in range(EXPECTED_VERDICT_RULES)],
                                    [(f"s{i}", 0) for i in range(EXPECTED_SELECT_RULES)],
                                    [(f"n{i}", 0) for i in range(EXPECTED_SNAPSHOT_RULES)],
                                    Problems(), 0)), 0),
        ):
            try:
                got = probe()
            except Exception as exc:
                got = f"raised {type(exc).__name__}: {exc}"
            guard(ran, bad_out, why, got, want)
    return ran, bad_out


EXPECTED_VERDICT_GUARDS = 6


def verdict_guard_reach_problems(ran, problems):
    """Whether the verdict guards ran, and whether the helper that records them still works.

    Computed in the caller for the same reason the guards are: a check inside the code it
    polices can be skipped by a `return` placed above it.
    """
    out = list(problems)
    probe_ran, probe_bad = [], []
    guard(probe_ran, probe_bad, "canary", 1, 1)
    guard(probe_ran, probe_bad, "canary", 0, 1)
    if len(probe_ran) != 2 or len(probe_bad) != 1:
        out.append(f"the guard helper is hollow: it counted {len(probe_ran)} and reported "
                   f"{len(probe_bad)}, want 2 and 1 -- a guard that cannot fail is not made "
                   f"honest by being listed")
    if len(ran) != EXPECTED_VERDICT_GUARDS:
        out.append(f"the verdict guards ran {len(ran)} checks, expected "
                   f"{EXPECTED_VERDICT_GUARDS}: one was deleted, or the loop stopped "
                   f"reaching it. Ran: {ran}")
    elif any(flag for _, flag in ran):
        out.append(f"a verdict guard recorded a failure that never reached the problem "
                   f"list: {[n for n, f in ran if f]}")
    return out


def self_test_verdict(report, extra=()):
    """Turn what self_test() returned into an exit code.

    The reach check lives here rather than inside self_test() because a check cannot police
    the code that is allowed to skip it: a bare ``return`` in the middle of the self-test
    used to leave every rule unchecked and the process exiting 0, since the only thing that
    could notice the missing verdict was the line that never ran. An absent verdict is now
    its own failure, which is the only way to tell "nothing was checked" from "all of it
    held".

    The three counts are named apart in the summary because "N rules hold" said about three
    different mechanisms is how a summary starts meaning less than the reader assumes: the
    verdict rules read captured output, the selection rules run a pure function, and the
    snapshot rules build repositories on disk. A merge that carried one family's count and
    silently dropped another's -- which is what #119's snapshot rules would have done here,
    since this PR restructured the function they lived in -- would still have printed a
    confident pair.
    """
    if not isinstance(report, tuple) or len(report) != 5:
        print("self-test FAILED:")
        print(f"  - it returned {report!r} instead of a verdict, so nothing was checked "
              f"(a `return` inside self_test skips every rule below it)")
        return 1
    vrules, rules, srules, bad, raised = report
    # The problems and the count of problems arrive separately on purpose: the list is the
    # thing a `clear()` on the way out can empty, and the number is not.
    if len(bad) != raised:
        print("self-test FAILED:")
        print(f"  - it raised {raised} problems and returned {len(bad)}: the problem list "
              f"was emptied on the way out, so every failure it held was discarded")
        return 1
    # Each count is compared against a number that has to be edited on purpose: every rule
    # can pass perfectly while one of them is missing from the file. Neither pin is one of
    # the counted rules -- a pin that counts itself moves the number it exists to hold.
    # Every entry has to have come through `rule()`. A count is only a claim about
    # assertions if the assertions went through the thing that counts them.
    forged = [r for r in list(vrules) + list(rules) + list(srules)
              if not (isinstance(r, tuple) and len(r) == 2 and isinstance(r[1], int))]
    if forged:
        bad.append(f"{len(forged)} rule(s) were recorded without going through rule(): "
                   f"{forged[:3]} -- a rule appended by hand counts itself and asserts "
                   f"nothing")
    if len(vrules) != EXPECTED_VERDICT_RULES:
        bad = list(bad) + [f"the self-test ran {len(vrules)} verdict rules, expected "
                           f"{EXPECTED_VERDICT_RULES}: a rule was deleted, or stopped "
                           f"counting. Ran: {vrules}"]
    if len(rules) != EXPECTED_SELECT_RULES:
        bad = list(bad) + [f"the self-test ran {len(rules)} selection rules, expected "
                           f"{EXPECTED_SELECT_RULES}: a rule was deleted, or stopped "
                           f"counting. Ran: {rules}"]
    # The third count is the one this merge owed. #119 landed its snapshot rules inside the
    # function this PR restructured, so resolving the overlap toward this PR's shape without
    # carrying them would have left the fix present, the summary confident, and nothing
    # checking it -- which is the defect #118 was opened about, arriving through a merge.
    if len(srules) != EXPECTED_SNAPSHOT_RULES:
        bad = list(bad) + [f"the self-test ran {len(srules)} snapshot rules, expected "
                           f"{EXPECTED_SNAPSHOT_RULES}: a rule was deleted, or stopped "
                           f"counting. Ran: {srules}"]
    bad = list(bad) + [q for q in extra if q not in bad]
    if bad:
        print("self-test FAILED:")
        for b in bad:
            print(f"  - {b}")
        return 1
    print(f"self-test: {len(vrules)} verdict rules, {len(rules)} selection rules and "
          f"{len(srules)} snapshot rules hold")
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
        ran, guard_problems = verdict_guard_problems()
        return self_test_verdict(self_test(), verdict_guard_reach_problems(ran, guard_problems))

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
    # turn a real result into a compile failure. What arrives is what the repository
    # tracks plus anything untracked that it does not ignore, less the names in
    # NEVER_COPIED -- see snapshot_ignore, and snapshot for what happens to a path that
    # cannot be copied at all.
    work = tempfile.mkdtemp(prefix="plock-mutate-")
    global ROOT
    ROOT = os.path.join(work, "repo")
    # The try opens here rather than at the loop. A set file that fails validation makes
    # mutset.load call sys.exit, and an exit between the copy and the loop used to leave a
    # whole copy of the tree behind in /tmp. The snapshot's own refusal rides the same
    # finally: sys.exit inside the handler still runs it, so a retry on a tree with an
    # unreadable file cannot pile another full copy into TMPDIR.
    try:
        skipped = snapshot(SRC, ROOT)
        print(f"running against a copy of the tree in {ROOT}")
        for path in skipped:
            print(f"snapshot left out an uncopyable path that is not source: {path}")

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
            # set_label, not an inline relpath: the two arms of this branch label the same
            # file, and the inline copy printed `../../outside/platform.json` where the
            # --only arm printed `platform.json` for one set outside the repo.
            sets = [(p, set_label(p)) for p in sets]

        # Consumed by a function rather than a loop here, so the self-test can run this loop
        # against a fake runner and assert every set ran and every tally was summed -- see
        # run_selection's docstring for why that is the close and a text audit is not.
        totals, ran_all = run_selection(sets, run_set)
        if not ran_all:
            # A set could not be run at all, so there is no honest TOTAL to print: the
            # numbers would describe a run that did not happen.
            return 2
    except SnapshotError as err:
        # Source that could not be copied, or a copy found to be missing tracked files.
        # Both mean the run would report mutants caught against a tree that is not the
        # repository, so the refusal is the last word -- and it is fatal, where an
        # uncopyable socket is merely skipped.
        sys.exit(str(err))
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
              f"{'is' if len(missing) == 1 else 'are'} not in {scope(args.sets)}, so "
              f"{'it was' if len(missing) == 1 else 'they were'} never run")
        return 1
    return 0 if totals["survived"] == 0 and totals["invalid"] == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
