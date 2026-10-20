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
    says so on its line (see verdict);
  - the snapshot can leave source OUT of the copy it runs against, and a battery on a
    tree that is missing files still reports every mutant caught -- so what the copy
    omits is the repository's own ignore rules plus four fixed names, decided by
    equality on the path being walked and never by a prefix match, because a directory
    that merely CONTAINS an ignored one still holds source (see snapshot_ignore). The
    same rules are what stop the copy dying outright: an ignored Chrome profile holds a
    socket, and copytree raises on it before one mutant runs.

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

    The predicate is equality on the path being walked, never a prefix match.
    `testing/capture` CONTAINS an ignored directory and must still be copied: dropping it
    would take source out of the snapshot, and the run would then report every mutant
    caught against a tree that is missing files. Nor may a name merely starting with an
    ignored path go with it -- `plock2` sits beside an ignored `plock` and is source.
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


# The self-test's own reach, pinned. A deleted rule is invisible to the rules that survive
# it, and the number the self-test prints is all a reader has to go on -- which is the
# failure scripts/check-mutations.py pins for itself, so this file pins it the same way.
# Both numbers go up when a rule is added, and that is the point of them.
EXPECTED_VERDICT_RULES = 22
EXPECTED_SNAPSHOT_RULES = 19


def self_test():
    """Check the rules a battery gets wrong without ever running one: the verdicts, and
    the snapshot it copies the tree into.

    The verdict cases are excerpts of real runs (main plus W25/W23 in internal/web, and a
    deliberate compile error), trimmed to the lines a verdict is decided from, so a
    rule cannot be written against an output shape go does not produce. The snapshot cases
    are a tree laid out like the repository and, at the end, a real copytree over a socket
    and a dead symlink -- the two things in a Chrome profile that made a battery die before
    it applied a mutant. There is no Go here on purpose: this must run in CI where a
    battery would cost an hour.
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
    snap_checked = 0
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

    # ---- the snapshot --------------------------------------------------------------
    # What a battery costs is why these are here: the run that found #117 died in copytree
    # before it had applied one mutant, and every check that runs earlier -- the checker,
    # `make verify`, CI's `mutation sets` job -- copies nothing at all, so each of them said
    # the tree was fine.
    #
    # Everything below needs git, because the fixtures are repositories: what gets skipped IS
    # the repository's answer. A missing git fails rather than skips -- a guard that stepped
    # around a missing binary reported "11 snapshot rules hold" while the one rule that
    # proves the fix never ran.
    snap_checked += 1
    if not shutil.which("git"):
        bad.append("git is not on PATH, so the snapshot rules have no repository to check "
                   "against and the fix this file exists for would go untested")

    def git_run(args, cwd):
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
            bad.append(f"git {' '.join(args)} could not run in {cwd} ({err}), so the "
                       "fixture below is not a repository and whatever it appears to check "
                       "is not what the snapshot does")
            return None
        if done.returncode != 0:
            bad.append(f"git {' '.join(args)} failed in {cwd} (exit {done.returncode}), so "
                       "the fixture below is not a repository and whatever it appears to "
                       "check is not what the snapshot does")
        return done.returncode

    def chrome_profile(where, name):
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
            bad.append(f"this platform will not build the profile in {name}/ ({err}), so "
                       "the copy rules below would be copying an ordinary directory")
        if not (os.path.islink(os.path.join(slot, "SingletonCookie"))
                and os.path.lexists(os.path.join(slot, "SingletonSocket"))):
            bad.append(f"the profile in {name}/ has no dead symlink and no socket, so the "
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

    def row(name, gone, want):
        """One table row's verdict, in a function so the canary under it can hand it an
        answer it has to refuse. The count below pins that a row ran; it cannot pin that this
        comparison still compares, and replacing the `if` with `pass` used to leave every row
        counting happily."""
        if gone != want:
            bad.append(f"snapshot {name}: skips {sorted(gone)}, want {sorted(want)}")

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
    for name, directory, names, want in rows:
        snap_checked += 1
        row(name, set(ignore_cb(directory, list(names))), want)

    snap_checked += 1
    if len(seen) != len(rows):
        bad.append(f"the snapshot table consulted the ignore callback {len(seen)} times for "
                   f"{len(rows)} rows: a row that answers from its own expectation is not a "
                   "rule about the callback")

    # The canary, and it is the row above that needs it rather than this line. Every row is
    # run again against an answer it must refuse; a comparison hollowed out to `pass` refuses
    # nothing and the count above, which counts rows rather than verdicts, never notices.
    # This is the mechanism scripts/check-mutations.py pins itself with.
    snap_checked += 1
    refused = 0
    for name, _directory, _names, want in rows:
        mark = len(bad)
        row(name, set(want) ^ {"__not_a_real_file_name__"}, want)
        if len(bad) > mark:
            refused += 1
        del bad[mark:]  # the canary's own complaint is not a finding about the tree
    if refused != len(rows):
        bad.append(f"the snapshot table refused {refused} wrong answers out of {len(rows)}: "
                   "a row that cannot fail is not a rule, and the count above cannot tell "
                   "one from a row that passed")

    # Which uncopyable path is fatal, over five real ones. Decided by kind rather than by
    # trying to read, because as root nothing is unreadable and a permission-based fixture
    # would be a rule that cannot fail.
    snap_checked += 1
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

    # The acting half. The rule above proves the sorting; nothing proved that a fatal sort
    # actually stops a run, because reaching it needs an unreadable file and nothing is
    # unreadable as root -- with the refusal deleted, the self-test printed "14 snapshot
    # rules hold". So `snapshot` takes its copy as an argument and the self-test hands it one
    # that fails on a named file. The fixture is an empty repository so the tracked check
    # below cannot be what raises: the reason for the refusal is asserted, not assumed.
    snap_checked += 1
    with tempfile.TemporaryDirectory() as work:
        src = os.path.join(work, "src")
        os.makedirs(os.path.join(src, "internal"))
        with open(os.path.join(src, "internal", "locked.go"), "w") as fh:
            fh.write("package web\n")
        git_run(["init", "-q"], src)

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

    # And the check that the copy HOLDS the repository -- acting, not sorting. A copy that
    # comes back without an error and without a tracked file is what a predicate bug, a file
    # that vanished mid-copy, and an unreadable symlink all look like from the outside, and
    # the sorting above cannot see any of them because nothing failed.
    snap_checked += 1
    with tempfile.TemporaryDirectory() as work:
        src = os.path.join(work, "src")
        os.makedirs(os.path.join(src, "internal"))
        for name in ("server.go", "keep.go"):
            with open(os.path.join(src, "internal", name), "w") as fh:
                fh.write("package web\n")
        git_run(["init", "-q"], src)
        git_run(["add", "internal/server.go", "internal/keep.go"], src)
        # Tracked AND deliberately not copied: `.cortexkit/.gitignore` is exactly this shape
        # in this repository, and it is the case that made the exemption necessary. Without
        # an assertion both ways the exemption is unpinned -- too wide silently drops source,
        # too narrow refuses a run that should have gone ahead.
        os.makedirs(os.path.join(src, ".cortexkit"), exist_ok=True)
        with open(os.path.join(src, ".cortexkit", ".gitignore"), "w") as fh:
            fh.write("state\n")
        git_run(["add", ".cortexkit/.gitignore"], src)

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

    # The end-to-end half, through `snapshot` rather than a hand-built callback: the
    # composition is the fix, and a rule that rebuilt the callback for itself stayed green
    # with `main` reverted to the four hardcoded names.
    #
    # `runs` is what the repository ignores and `other` is not. The first is the bug that
    # happened; the second is the guard the ticket asked for in as many words -- "a tree
    # containing an UNIGNORED directory with a socket in it" -- and it is the class rather
    # than the instance, because the next one will not be named runs/.
    snap_checked += 1
    with tempfile.TemporaryDirectory() as work:
        src = os.path.join(work, "src")
        os.makedirs(os.path.join(src, "internal", "web"))
        with open(os.path.join(src, "internal", "web", "server.go"), "w") as fh:
            fh.write("package web\n")
        for name in ("runs", "other"):
            chrome_profile(src, name)
        with open(os.path.join(src, ".gitignore"), "w") as fh:
            fh.write("runs/\n")
        git_run(["init", "-q"], src)
        # Tracked, so that the copy is checked against the repository rather than against
        # nothing: an empty tracked set makes `missing_source` pass for free. Only these two
        # are staged -- `git add -A` refuses to look at a socket, and the socket is the point.
        git_run(["add", "internal/web/server.go", "other/note.txt"], src)
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

    def git_tree(where, ignore=("runs/",)):
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
        git_run(["init", "-q"], where)
        return where

    # What git is asked for must be what gets skipped. The rule above gets its ignored paths
    # from `snapshot`, which asks git, so nothing so far would notice `ignored_paths` being
    # gutted into returning nothing -- and that is the other half of this bug, the half a
    # .gitignore that does not say `runs/` would hit.
    snap_checked += 1
    if shutil.which("git"):
        with tempfile.TemporaryDirectory() as work:
            got = ignored_paths(git_tree(work))
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
            quoted = ignored_paths(git_tree(work, ignore=("runs/", 'run"s/')))
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
            git_run(["init", "-q"], raw)
            if weird + "/" not in ignored_paths(raw):
                bad.append("an ignored path the locale cannot decode did not survive the "
                           f"round trip: {ignored_paths(raw)!r}")

    # The answer must be a function of the repository and of nothing else. `GIT_DIR` is
    # exported by hooks and by wrapper scripts that call `make verify`, `GIT_CONFIG_PARAMETERS`
    # is how `git -c k=v` reaches a child, and a machine-wide core.excludesFile is not part of
    # this tree. Any one of them can make a battery copy the wrong tree; the last can also
    # make the assertion above pass or fail on someone's dotfiles, and an assertion about what
    # is NOT ignored is the one that has to be able to say no.
    snap_checked += 1
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
            repo = git_tree(os.path.join(work, "repo"))
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

    # A tree git cannot answer for is snapshotted exactly as it always was. There are two
    # ways not to answer and the function has a branch for each: nothing to run, and
    # something that runs and says no. Both have to fall back rather than refuse, because a
    # missing git is no reason a battery cannot run -- which is survivable now because of the
    # error sorting above, not in spite of it.
    snap_checked += 1
    if ignored_paths(os.path.join(tempfile.gettempdir(), "no-such-tree-at-all")):
        bad.append("ignored_paths invented entries for a path that does not exist")

    snap_checked += 1
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
    # And the reach itself. Every rule above can pass perfectly while one of them is
    # missing from the file, so the count is compared against a number that has to be
    # edited on purpose. This check is deliberately NOT one of the counted rules: a pin
    # that counts itself moves the number it exists to hold still.
    if (checked, snap_checked) != (EXPECTED_VERDICT_RULES, EXPECTED_SNAPSHOT_RULES):
        bad.append(f"the self-test ran {checked} verdict rules and {snap_checked} snapshot "
                   f"rules, expected {EXPECTED_VERDICT_RULES} and "
                   f"{EXPECTED_SNAPSHOT_RULES}: a rule was deleted, or stopped counting")

    if bad:
        print("self-test FAILED:")
        for b in bad:
            print(f"  - {b}")
        return 1
    # Counted, not derived from len(cases): the number is a claim about how much was
    # tested, and a count that silently under-reports the rules below it is the same
    # species of error the rules themselves are here to prevent. The two kinds are counted
    # apart and named apart, because "N rules hold" said about two different mechanisms is
    # how a summary starts meaning less than the reader assumes.
    print(f"self-test: {checked} verdict rules and {snap_checked} snapshot rules hold")
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
        return self_test()

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
    # tracks plus anything untracked that it does not ignore, less the four names in
    # NEVER_COPIED wherever they appear -- see snapshot_ignore, and snapshot for what
    # happens to a path that cannot be copied at all.
    work = tempfile.mkdtemp(prefix="plock-mutate-")
    global ROOT
    ROOT = os.path.join(work, "repo")
    try:
        skipped = snapshot(SRC, ROOT)
    except SnapshotError as err:
        # The refusal is the last word this run gets, so it has to take the copy with it:
        # every retry on a tree with an unreadable file would otherwise leave another full
        # copy of the repository behind in TMPDIR.
        shutil.rmtree(work, ignore_errors=True)
        sys.exit(str(err))
    print(f"running against a copy of the tree in {ROOT}")
    for path in skipped:
        print(f"snapshot left out an uncopyable path that is not source: {path}")

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
