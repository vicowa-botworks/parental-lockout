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

import ast
import contextlib
import inspect
import io
import json
import os
import shutil
import signal
import subprocess
import sys
import tempfile
import time

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
GO = os.environ.get("GO", "go")

# Words that claim a test killed something. This checker never runs a test, so none of them
# may appear in anything it prints about the sets it checked (#110). Substring matches on
# purpose: `verif` has to catch verified, verifies and unverified alike.
#
# Deliberately absent: test, check, pass, valid. The scan reads main's whole source, where
# those are this tool's own vocabulary -- `--self-test`, `check(doc)` -- so including them
# would make the file object to being itself. Each stem here gets its own control in
# self_test, generated from this tuple, so dropping one drops the rule count.
CLAIM_STEMS = ("verif", "confirm", "proven", "guarantee", "certain", "cover", "kill", "caught")

# What the closing line has to say about its own limits. Forbidding claim verbs is only half
# a pin: a sentence can avoid every banned word and still say nothing about what was never
# measured. Requiring the disclaimer means deleting it is an explicit edit rather than an
# absence nobody noticed.
DISCLAIMER_FRAGMENTS = ("apply", "compile", "does not run the suite", "survives every test",
                        "invisible", "make mutate")

# The one line that prints the tally, pinned by equality in the self-test. Re-spelling the
# call is allowed; it just has to be re-spelled here too, so the guard cannot be routed
# around by editing the call site and leaving the checks behind. The double backslash is
# deliberate: getsource returns source text, where the escape is two characters.
SUMMARY_CALL = 'print(f"\\n{summary_line(total)}")'

# The two lines that connect a detected set-file problem to the run's verdict, pinned the same
# way as the summary call. `exit_status` has four rules of its own and `encoding_problems` has
# three, and none of them can read the call site: handing `exit_status` one list prints the
# refusal and exits 0, and deleting the collection silences the refusal as well -- both measured
# with every other rule holding. These two lines are also exactly where a merge lands, since
# this file has been conflict-resolved on them more than once, so moving one is meant to be a
# deliberate edit in two rather than a resolution nobody re-read.
STATUS_CALL = ("status = exit_status(bad, file_problems, script_problems, "
               "collisions)")
COLLECT_CALL = "file_problems.extend(found)"

# The line that runs the tool scan. Pinned because it is the whole check and it is one statement:
# a merge that takes the side without it leaves every rule in self_test holding, the sets checked,
# the summary printed, and nothing at all looking at scripts/. The two lines above pin where a
# problem goes once it is found; this one pins that anyone looks.
SCRIPT_SCAN_CALL = "script_problems = script_open_problems(script_files())"

# The two calls scripts/check-scripts.py makes back into this file. Pinned from here because that
# file is the half of this gate `make verify` runs (#189), and a deleted call over there is
# invisible from in here: every rule in this self-test would still hold, `make verify` would
# still print OK, and the guard the pre-push gate exists to run would be running nothing. The
# same shape as SCRIPT_SCAN_CALL, one file over -- which is exactly why it needs its own pin.
SCRIPTS_TOOL_CALL = "script_problems = gate.script_open_problems(gate.script_files())"
SCRIPTS_SET_CALL = "found, bad_bytes = gate.encoding_problems(label, path)"

# How many rules self_test must run. Pinned because a deleted rule is invisible to every rule
# that survives it -- see the assertion in self_test for what that does and does not catch.
EXPECTED_RULES = 95

# What `--help` prints. A string rather than a docstring because the docstring is this file's
# own explanation of the gate and would run to two screens here.
USAGE = """usage: check-mutations.py [--self-test | --restore | SET_FILE ...]

With no arguments, check every set under scripts/mutations/: that each mutant still applies,
and that the package and its tests still compile with it applied. Nothing is run, so a mutant
that survives every test is invisible here -- that is `make mutate`.

  --self-test   check the rules this file depends on; no toolchain, milliseconds
  --restore     undo a mutation a dead run left applied (see .mutation-journal.json)
  --help        this

The half of this that needs no Go toolchain -- the encoding guards over scripts/ and the set
files -- runs in `make verify` as scripts/check-scripts.py, which imports the guards from here
rather than copying them."""

# Shape rules come from mutset, the same module the battery loads. Two validators that
# disagree are how a set ends up green at this gate and refused at the battery.
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from mutset import (GRANDFATHERED_IDS, duplicate_claims, id_collisions, load,  # noqa: E402
                     remaining_collisions, validate)


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
        # Named, not inherited. This decodes `go test`'s output, and this repository's own failure
        # messages carry em dashes; under a locale that is not UTF-8 the decode raises inside
        # subprocess and the checker reports a compile failure it did not observe.
        encoding="utf-8",
        cwd=ROOT,
    )


def json_escapes(text):
    """Yield (offset, escape) for every JSON \\uXXXX escape in raw file text.

    This has to run on the bytes of the file and not on the decoded document, and that is the
    whole reason #148 needs its own check rather than riding along in `check()`. Once
    `json.loads` has run, `"\\u2014"` and `"—"` are the same string: the escape is a property of
    the file, and it is gone by the time any decoded-doc rule can look at it.

    Backslashes are consumed in pairs, so `\\u00e9` -- a literal backslash followed by `u`, which
    is what a mutant's `old` holds when it targets a Go string escape -- is not reported. A check
    that fired on those would be refusing correct files, and the self-test pins that case.
    """
    i, n = 0, len(text)
    while i < n:
        if text[i] == "\\":
            nxt = text[i + 1] if i + 1 < n else ""
            if nxt == "u" and i + 6 <= n and all(c in "0123456789abcdefABCDEF"
                                                 for c in text[i + 2:i + 6]):
                yield i, text[i:i + 6]
            i += 2
            continue
        i += 1


def encoding_problems(label, path):
    """Refuse a committed set file that was written by a tool rather than declared.

    Two defaults do this, and both were live in `gen-cli-mutations.py` until #148: `json.dump`
    with `ensure_ascii=True` rewrites every em dash and arrow in a note as `\\u2014` / `\\u2192`,
    and `open()` without `encoding=` writes in the operator's locale. The first mints a diff hunk
    on a mutant nobody changed -- #133 carried exactly one into review on a line of
    `enforce.json` its own PR did not touch -- and the second means two machines write different
    bytes from the same generator, which fails silently in the opposite direction.

    The generator that carried those defaults is gone -- #150 deleted it, because running it
    silently dropped the hand-added CI21 -- so why keep the gate: the hunk in #133 came from
    neither writer anyway. It came from an ad-hoc `python3 -c` append, which is what the obvious
    one-liner does by default when a mutant is being added at the end of a long session. A
    convention cannot stop that; a refusal can. Deleting the tool removes one way to write a bad
    set file and leaves every other way exactly as open, which is the whole argument for the
    refusal living in the gate rather than in the writer.

    It answers with a second value, `bad_bytes`, because main has to decide something from this
    result and a decision made by re-reading the sentence is a decision coupled to its wording.
    The first version asked `any(" is not valid UTF-8 " in w for w in found)`, which held only
    while the message below kept that exact phrase and nothing on either side was pinned: reword
    the message and the run went back to dying inside `load` on the second set of thirteen,
    losing the other eleven checks and the sentence written for the case. Failing closed either
    way is why that was a note and not a breakage; a flag is why it does not have to be either.
    """
    try:
        with open(path, encoding="utf-8") as fh:
            text = fh.read()
    except UnicodeDecodeError as exc:
        return [f"{label} is not valid UTF-8 ({exc}); a set file declares its encoding by being "
                f"UTF-8, not by whatever locale generated it last"], True
    out = []
    for offset, esc in json_escapes(text):
        line = text.count("\n", 0, offset) + 1
        # The remedy says "edit this file", not "regenerate it". A first version told the reader
        # to re-run the generator with ensure_ascii=False, and on this tree that deleted CI21 --
        # `gen-cli-mutations.py` emitted 20 mutants while the committed set held 21, because CI21
        # was appended by hand. Advice that fixes an encoding hunk by silently dropping a mutant
        # is the worse trade, and the gate cannot see the mutant go. #150 settled it by deleting
        # the generator, so the sentence below no longer has to warn against a specific tool:
        # there is no writer to re-run, and the sets are hand-maintained by declaration.
        out.append(f"{label}:{line}: {esc} is a JSON escape for a character the file could hold "
                   f"literally -- edit it to the character itself. There is no generator to "
                   f"re-run: set files are maintained by hand (#150).")
    return out, False


JOURNAL_NAME = ".plock-mutant-journal.json"

# The pre-run contents of every file this process has mutated, kept in memory as well as on
# disk so a single journal spans every set the run was handed.
_JOURNAL = {}


def journal_path():
    return os.path.join(ROOT, JOURNAL_NAME)


def journal_load():
    """The undo journal as a dict, or None when there is no journal.

    A journal that exists but cannot be parsed is returned with an `unreadable` key rather
    than as None: it is still the marker that a run died here, and answering "clean tree" to
    a tree with an unreadable marker is the failure this whole file exists to prevent (#169).
    """
    try:
        with open(journal_path(), encoding="utf-8") as fh:
            return json.load(fh)
    except FileNotFoundError:
        return None
    except (OSError, ValueError) as exc:
        return {"unreadable": str(exc), "files": {}}


def journal_flush():
    """Write the journal out, atomically, before the next byte of any mutant lands.

    `finally` is the current crash insurance and it does not run on SIGKILL, which is exactly
    what a harness with a tool timeout sends. The measured case: a sweep killed at 300 s left
    W205's `new` sitting in `internal/web/templates/testlock.tmpl`, and the next sweep reported
    W205 as `matches 0 places` -- which reads as "someone retired this mutant in a merge" and
    invites a re-point of a mutant that was never broken (#169).
    """
    tmp = journal_path() + ".tmp"
    body = {"pid": os.getpid(),
            "undo": "python3 scripts/check-mutations.py --restore",
            "files": _JOURNAL}
    with open(tmp, "w", encoding="utf-8") as fh:
        json.dump(body, fh)
    os.replace(tmp, journal_path())


def journal_record(rel, text):
    """Claim `rel` before mutating it. The first claim is the one that counts: it holds the
    contents from before this run touched the file, not from before this mutant."""
    if rel in _JOURNAL:
        return
    _JOURNAL[rel] = text
    journal_flush()


def journal_clear():
    try:
        os.remove(journal_path())
    except FileNotFoundError:
        pass


def journal_restore(journal):
    """Write every file the journal holds back the way it was, then drop the journal."""
    done = []
    for rel, text in sorted(journal.get("files", {}).items()):
        with open(os.path.join(ROOT, rel), "w", encoding="utf-8") as fh:
            fh.write(text)
        done.append(rel)
    journal_clear()
    return done


def install_undo_signals():
    """Put the tree back on a signal a process is allowed to catch.

    SIGTERM and SIGINT cover the two ways a person or a harness stops a sweep politely: Ctrl-C
    in a terminal, and the SIGTERM a supervisor sends before it escalates. Neither is SIGKILL --
    no handler runs there, which is what the journal on disk is for (#169). This is the half
    that leaves `git status` clean; the journal is the half that leaves the next run honest.

    `os._exit` is deliberate. A normal raise would unwind into `check`'s finally, which restores
    from its own cache and then reports -- mid-signal, with the tree already put back.
    """

    def undo(signum, _frame):
        try:
            # Shaped like the on-disk document, because that is what `journal_restore` reads.
            # Handing it the bare {path: text} map restores nothing and still deletes the
            # journal -- which is worse than doing nothing at all.
            journal_restore({"files": dict(_JOURNAL)})
        finally:
            os._exit(128 + signum)

    for name in ("SIGTERM", "SIGINT"):
        sig = getattr(signal, name, None)
        if sig is not None:
            signal.signal(sig, undo)


def dirty_paths():
    """The paths `git status --porcelain` reports, or None when git cannot answer.

    None is not "clean". The sentence this feeds has to be able to say it could not look.
    """
    try:
        out = subprocess.run(["git", "-C", ROOT, "status", "--porcelain"],
                             capture_output=True, text=True, encoding="utf-8", timeout=20)
    except (OSError, subprocess.SubprocessError):
        return None
    if out.returncode != 0:
        return None
    return [line for line in out.stdout.splitlines() if line.strip()]


def stale_corpse_note(problems, dirty):
    """The sentence a `matches 0 places` report needs and does not have (#169).

    That report is the one a person acts on, and today it cannot tell a mutant somebody
    retired in a merge from a mutant a killed run left applied in the tree. `problems` is the
    run's (id, why) list; `dirty` is `dirty_paths()` from before the first write, or None.
    Returns None when there is nothing to say.
    """
    if not any("matches 0 places" in why for _, why in problems):
        return None
    if dirty:
        return (f"{len(dirty)} path(s) were already modified when this sweep started, so a "
                f"`matches 0 places` line may be describing a mutation a killed run left "
                f"behind rather than a mutant that was retired. Check `git diff` on the named "
                f"file before re-pointing anything; `python3 scripts/check-mutations.py "
                f"--restore` undoes a journal.")
    if dirty is None:
        return ("this run could not ask git whether the tree was clean, so a `matches 0 places` "
                "line cannot be told apart from a mutation a killed run left behind. "
                "`git status --porcelain` before re-pointing anything.")
    return None


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
            # Claimed before the write, not in the `finally`: the finally is the undo, and the
            # journal is the proof of what the undo is for. A run killed by SIGKILL never gets
            # to it (#169).
            journal_record(rel, original[rel])
            with open(abs_path, "w", encoding="utf-8") as fh:
                fh.write(mutated)
            rc = compiles(pkg)
            # Restore this file before asking about the next mutant. Each write is
            # `original.replace(old, new, 1)`, so only one edit per file is ever live --
            # but leaving it on disk means a later mutant of a DIFFERENT file in the same
            # package is compiled against this mutant's edit too, and a package build
            # fails on the first file that does not parse. One non-compiling mutant then
            # reports every mutant after it as broken: this was observed as 10 reported
            # failures for a single orphaned variable, nine of them pointing at mutants
            # that were fine. The `finally` below stays as crash insurance.
            with open(abs_path, "w", encoding="utf-8") as fh:
                fh.write(original[rel])
            if rc.returncode != 0:
                lines = (rc.stderr or rc.stdout).strip().splitlines()
                # The first line is `# <pkg> [<pkg>.test]` -- a header, not a reason. The
                # reason is below it, and dropping it sends the reader to reproduce the
                # compile by hand to find out what the mutant actually broke.
                reasons = lines[1:4] if len(lines) > 1 else lines
                problems.append((mid, "does not compile: " + "; ".join(reasons or ["no output"])))
    finally:
        for rel in touched:
            with open(os.path.join(ROOT, rel), "w", encoding="utf-8") as fh:
                fh.write(original[rel])
    return problems


def summary_line(total):
    """The closing line, worded to survive being quoted on its own.

    This is the only line anyone carries away: a per-set line two lines above it already
    said "apply and compile" and still got read as a battery result, because the summary
    said "verified" (#110). So the ceiling has to travel *inside* the sentence rather than
    sit beside it -- a caveat on its own line is dropped by exactly the reader the per-set
    line was dropped by.

    What this checker does is apply each mutant and compile the tree. It never runs a test,
    so it cannot see a surviving mutant: `W176` applied, compiled, and then passed
    `go test ./...` on main at 567a76f, and this script had nothing to say about it. The
    mutant was eventually deleted by 53be75d together with the dead `State()` it mutated,
    precisely because no test could kill it -- so the evidence outlives the pattern, and a
    green `mutation sets` job whose output ends in a word like "verified" is still read as
    "the battery passed" by everyone who does not open the workflow. That reading is the
    failure this line is the last chance to prevent.
    """
    return (f"all {total} mutants apply and compile -- this does not run the suite, so a "
            f"mutant that survives every test is invisible here (make mutate)")


def script_files():
    """Every Python tool under scripts/.

    These are the programs that read this repository's files and write them back -- the battery
    restores a source through the same kind of call it read it with -- so a locale default in one
    of them is a tool handing back bytes that are not the ones it took. That is #154: the same
    default #151 refused in a set file, one layer down, in the tool that writes the set file.
    """
    dirn = os.path.join(ROOT, "scripts")
    return sorted(os.path.join(dirn, f) for f in os.listdir(dirn) if f.endswith(".py"))


# A child process's output is bytes with the same problem: `text=True` decodes them with
# `locale.getpreferredencoding(False)`, and this repository's own test output carries em dashes and
# arrows. Measured on the pre-fix tree, the first thing to die under `PYTHONUTF8=0
# PYTHONCOERCECLOCALE=0 LC_ALL=C` was not one of the four `open()` sites #154 names -- it was
# `subprocess.run` inside mutate.py, in `subprocess._translate_newlines`, on byte 0xe2 of a `go
# test` line. So the gate covers both surfaces or it does not cover the failure.
TEXT_SUBPROCESS_FUNCS = ("run", "Popen", "check_output", "check_call", "call", "getoutput",
                         "getstatusoutput")


def _textual_subprocess(node):
    """True when a subprocess call asks for decoded output and does not say what to decode with."""
    flags = {}
    for kw in node.keywords:
        if kw.arg in ("text", "universal_newlines", "encoding"):
            flags[kw.arg] = kw.value
    if "encoding" in flags:
        return False
    for name in ("text", "universal_newlines"):
        node_value = flags.get(name)
        if isinstance(node_value, ast.Constant) and node_value.value is True:
            return True
    return False


def unencoded_opens(text):
    """Complaint lines for every text-mode `open()` in *text* that names no encoding.

    `ast` rather than a scan of lines, and that choice is load-bearing for more than tidiness.
    This file's own docstring contains the phrase "open() without `encoding=`", and mutate.py's
    comments name the same call; a line-based scan refuses the checker for describing the defect
    it exists to find, which is how a gate like this gets loosened within a week. A call node
    cannot be a sentence and cannot be a comment.

    Binary mode is exempt because it has no encoding to name. The mode is read off the call --
    second positional argument or `mode=` -- and never guessed from the filename, because
    `capture.png` opened as text is precisely the mistake a path-based rule would bless.

    A file that does not parse is reported rather than skipped: every tool here is expected to
    run, so a SyntaxError is a file whose encoding defaults cannot be formed an opinion about,
    and silence is how a scan goes blind one file at a time.

    The surface is `open` and `io.open` only. `Path.read_text` / `Path.write_text` carry the same
    locale default and nothing in scripts/ uses them today; if one does, it belongs on this list
    rather than on a rule that quietly keeps passing it.
    """
    try:
        tree = ast.parse(text)
    except SyntaxError as exc:
        return [f"line {exc.lineno or '?'} does not parse ({exc.msg}), so its open() calls "
                f"cannot be read at all"]
    out = []
    for node in ast.walk(tree):
        fn = node.func if isinstance(node, ast.Call) else None
        # An Attribute is included because `io.open` is builtins.open with a module in front of
        # it; the default is the same, so the requirement is the same.
        name = fn.id if isinstance(fn, ast.Name) else (fn.attr if isinstance(fn, ast.Attribute) else None)
        if name == "open":
            pass
        elif (name in TEXT_SUBPROCESS_FUNCS and isinstance(fn, ast.Attribute)
              and isinstance(fn.value, ast.Name) and fn.value.id == "subprocess"):
            # Qualified, not merely named: mutate.py has its own `run` helpers, and a rule that
            # flagged every `run(..., text=True)` in sight would be refused as noise the first
            # time it fired on one.
            if _textual_subprocess(node):
                out.append(f"line {node.lineno}: subprocess.{name} decodes a child's output with "
                           f"text=True and names no encoding=, so a `go test` line carrying an em "
                           f"dash is decoded with the operator's locale")
            continue
        else:
            continue
        if any(kw.arg == "encoding" for kw in node.keywords):
            continue
        mode = None
        if len(node.args) > 1 and isinstance(node.args[1], ast.Constant):
            mode = node.args[1].value
        for kw in node.keywords:
            if kw.arg == "mode" and isinstance(kw.value, ast.Constant):
                mode = kw.value.value
        if isinstance(mode, str) and "b" in mode:
            continue
        out.append(f"line {node.lineno}: open() names no encoding=, so it reads and writes in "
                   f"whatever locale the operator's shell happens to carry")
    return out


def script_open_problems(paths):
    """`unencoded_opens` over the tools, each problem labelled with the file it lives in."""
    out = []
    for path in paths:
        label = os.path.relpath(path, ROOT)
        try:
            with open(path, encoding="utf-8") as fh:
                text = fh.read()
        except OSError as exc:
            out.append(f"{label}: cannot be read ({exc}); the tools that check this repository "
                       f"are expected to be readable")
            continue
        out.extend(f"{label}: {c}" for c in unencoded_opens(text))
    return out


def self_test():
    """Check the rules this file depends on, without a toolchain or the tree.

    Why this lives here rather than in a test file: the rules in mutset are the reason a
    green CI tick means something, and nothing else in the repository exercises them. A
    battery needs Go and minutes; these assertions need neither, so they can run on the same
    path that checks the sets and fail for the same reason the gate would.

    Every assertion goes through one of the helpers below and shares its counter, with three
    exceptions that cannot: the unreadable-source path, which only fires on a run that has
    already failed; the recorder canary, which must not count itself or it would inflate the
    tally it exists to protect; and the pinned tally itself, which would change the number it
    compares if it counted itself. Two groups of rules live here: rules over synthetic sets,
    which is what the tool validates, and rules about the closing line, which is a claim the
    tool makes. The tally says "rules" rather than "shape rules" because calling a wording
    assertion a shape rule would overstate what was
    tested -- the same species of error as the summary line this file was rewritten for.
    The count is a claim about how much was tested, so it is counted
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

    def expect_collisions(name, docs, want, grandfathered=GRANDFATHERED_IDS):
        nonlocal checked
        checked += 1
        got = bool(id_collisions(docs, grandfathered=grandfathered))
        if got != want:
            problems.append(f"{name}: expected {'a collision report' if want else 'silence'}, "
                            f"got {got}")

    # #142's ratchet, both directions. The pair is the whole rule: a checker that reports every
    # collision fails the second case, and one that reports none fails the first.
    two_sets = [("a.json", {"package": "./x/", "mutants": [dict(good, id="FRESH")]}),
                ("b.json", {"package": "./y/", "mutants": [dict(good, id="FRESH")]})]
    expect_collisions("a name two sets both claim, and nobody collided before", two_sets, True)
    expect_collisions("a name two sets both claim, already grandfathered",
                      [("a.json", {"package": "./x/", "mutants": [dict(good, id="A1")]}),
                       ("b.json", {"package": "./y/", "mutants": [dict(good, id="A1")]})], False)
    expect_collisions("one name in one set",
                      [("a.json", {"package": "./x/", "mutants": [dict(good, id="FRESH")]})],
                      False)
    expect_collisions("a grandfathered name, with the ceiling emptied", two_sets, True,
                      grandfathered=frozenset())
    if remaining_collisions(two_sets, grandfathered=frozenset({"FRESH"})) != ["FRESH"]:
        problems.append("remaining_collisions did not report the grandfathered id still colliding")
    if remaining_collisions(two_sets, grandfathered=frozenset()):
        problems.append("remaining_collisions counted a collision the ceiling does not cover")

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
                   {"package": "./x/", "expected_mutants": 1, "mutants": [dict(good, new="")]})
    # Three sets keep the equivalence PROOF in `equivalent`, so a string is a valid claim.
    expect_accepts("`equivalent` holding the proof as prose",
                   {"package": "./x/", "expected_mutants": 1,
                    "mutants": [dict(good, equivalent="proved in a scratch file")]})
    expect_accepts("a control set with every rule satisfied",
                   {"package": "./x/", "expected_mutants": 2,
                    "mutants": [good, dict(good, id="T2", count=2, equivalent=True)]})

    # #157: the one failure the rest of this module cannot see. Everything above validates what
    # is IN a file, so a mutant can leave a set by any route at all -- a bad merge, a hand edit, a
    # generator, a `git checkout` of the wrong path -- and `check-mutations.py` prints one fewer
    # and stays green. The count is a field in the set file, one line above the list it counts, so
    # a merge that deletes a mutant deletes its neighbour and the refusal names the file.
    expect_rejects("a set that lost a mutant is refused",
                   {"package": "./x/", "expected_mutants": 2, "mutants": [good]},
                   "a mutant was deleted")
    expect_rejects("a set that gained one without raising the number is refused",
                   {"package": "./x/", "expected_mutants": 1,
                    "mutants": [good, dict(good, id="T2")]},
                   "a mutant was added without")
    expect_rejects("a set with no count at all is refused",
                   {"package": "./x/", "mutants": [good]}, "is missing")
    expect_rejects("a count that is not a number is refused",
                   {"package": "./x/", "expected_mutants": "2", "mutants": [good]},
                   "must be an integer")
    # `True == 1` in Python, so a boolean count would compare equal to a one-mutant set and the
    # gate would read a typo as a satisfied claim.
    expect_rejects("a count written as a boolean is refused",
                   {"package": "./x/", "expected_mutants": True, "mutants": [good]},
                   "must be an integer")
    expect_accepts("a set whose count matches its list",
                   {"package": "./x/", "expected_mutants": 2,
                    "mutants": [good, dict(good, id="T2")]})

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

    # The closing line is pinned by equality with a literal written here, not by a denylist
    # of verbs. A denylist only ever contains the words someone thought of: a summary reading
    # "all N mutants apply and compile, and every one is confirmed covered" passes a ban on
    # verif/pass/battery while claiming exactly the property #110 was filed about. Equality
    # turns re-wording from a slip in one place into a deliberate edit in two.
    pinned = ("all 697 mutants apply and compile -- this does not run the suite, so a "
              "mutant that survives every test is invisible here (make mutate)")

    def rule(name, complaint, into=None, count=True):
        nonlocal checked
        if count:
            checked += 1
        if complaint:
            (problems if into is None else into).append(f"{name}: {complaint}")

    def rule_fires(name, complaint, into=None, count=True):
        """A control: hand a check something provably wrong and require a complaint back.

        Without one, a check whose comparison was hollowed out still increments the counter
        and the tally keeps reporting full strength. The first draft of this guard did exactly
        that -- it printed "19 rules hold" with four rules that could no longer fail.
        """
        nonlocal checked
        if count:
            checked += 1
        if not complaint:
            (problems if into is None else into).append(
                f"{name}: reported nothing, so the check it guards cannot fail")

    # The recorders are the layer every other rule reports through, so they get the treatment
    # they hand out. Writing to a throwaway list proves `rule` records a complaint, that
    # `rule_fires` records its absence, and that `rule_fires` stays quiet when the check did
    # report. One token changed in either body -- `if complaint:` to `if False and ...` -- and
    # every count survives while all of it goes silent; this is the only thing that notices.
    canary = []
    rule("canary", "boom", into=canary, count=False)
    rule_fires("canary", None, into=canary, count=False)
    rule_fires("canary", "boom", into=canary, count=False)
    if len(canary) != 2:
        problems.append(f"the rule recorder is not recording: the canary wrote "
                        f"{len(canary)} of the 2 entries it should have")

    def wording_diff(got, want):
        return None if got == want else f"got {got!r}, want {want!r}"

    def claim_report(line, stems):
        hit = [w for w in stems if w in line.lower()]
        return f"claims {hit}, which this checker never measured" if hit else None

    def disclaimer_report(line, fragments):
        missing = [w for w in fragments if w not in line.lower()]
        return f"omits {missing}" if missing else None

    def source_report(body):
        """A claim verb anywhere in main, and whether its last print is the pinned call.

        Every line is scanned, not only lines containing `print(`, and the summary line is not
        exempt. An earlier version skipped any line naming summary_line, which made the one
        line that actually prints the tally the single line the guard could not read: appending
        ` -- and every one is verified` to it, or calling `.replace()` on the returned sentence
        to turn "invisible here" into a claim, shipped the ticket's exact failure at full green.
        That append is also precisely the merge-conflict resolution this scan exists to catch.

        The last print is compared to the literal call rather than to a substring, so a tally
        re-spelled as `{total:>4}`, a renamed local, a second print after the summary, or a
        `sys.stdout.write` of the old sentence all land on a rule instead of walking past one.
        Changing the call's spelling is then a deliberate edit in two, same as the wording pin.
        """
        hits = [f"line {number} {complaint}"
                for number, line in enumerate(body.splitlines(), 1)
                for complaint in [claim_report(line, CLAIM_STEMS)] if complaint]
        printed = [ln for ln in body.splitlines() if "print(" in ln]
        if not printed:
            hits.append("main prints nothing at all")
        elif printed[-1].strip() != SUMMARY_CALL:
            hits.append(f"main's last print is {printed[-1].strip()!r}, not {SUMMARY_CALL!r}")
        return "; ".join(hits) or None

    rule("the summary line is the pinned wording", wording_diff(summary_line(697), pinned))
    # Equality against a single fixture cannot see a function that ignores its argument and
    # returns a constant -- and a constant here would print 697 after checking 2, the same
    # species of error as the one this file was rewritten for. Second number, same literal.
    rule("the tally comes from its argument",
         wording_diff(summary_line(3), pinned.replace("697", "3")))
    rule_fires("control: the wording check reports a mismatch",
               wording_diff("all 697 mutants verified", pinned))
    # A pin is only as honest as the string pinned, and that cuts both ways: the literal must
    # claim nothing, and it must still say the thing that makes it honest. Deleting the
    # disclaimer -- a sentence saying nothing false and nothing true -- is then an explicit
    # edit rather than an absence nobody noticed.
    rule("the pinned wording claims nothing", claim_report(pinned, CLAIM_STEMS))
    rule("the pinned wording still carries its disclaimer",
         disclaimer_report(pinned, DISCLAIMER_FRAGMENTS))
    rule_fires("control: the disclaimer check finds it missing",
               disclaimer_report("all 697 mutants apply and compile", DISCLAIMER_FRAGMENTS))
    # One control per stem, generated from the tuple itself. A fixture that exercises only
    # `verif` leaves the other seven silently deletable; looping means narrowing CLAIM_STEMS
    # drops the rule count and trips the tally below.
    for stem in CLAIM_STEMS:
        rule_fires(f"control: the claim check finds {stem!r}",
                   claim_report(f"all mutants {stem} by the suite", CLAIM_STEMS))

    # Pinning the function is not pinning the print. Revert main's closing print to the old
    # literal and leave summary_line correct, and every rule above still passes while the
    # script ships the overclaim and summary_line rots into dead code.
    try:
        body = inspect.getsource(main)
    except OSError as err:
        # Deliberately not a counted rule: it only fires on a path that has already failed,
        # and letting it increment would make the pinned tally wrong on that path too.
        problems.append(f"cannot read main's source to pin the call site: {err}")
        body = ""
    rule("main's summary print is pinned and claim-free", source_report(body))
    rule_fires("control: the source scan finds a claim appended to the summary line",
               source_report(f'    print(f"\\n{{summary_line(total)}} -- and every one is verified")'
                             f'\n    return 0'))
    rule_fires("control: the source scan finds the old tally re-spelled",
               source_report('    _ = summary_line(total)\n'
                             '    print(f"\\nall {total:>4} mutants verified")\n'))
    rule_fires("control: the source scan finds a print after the summary",
               source_report(f'    {SUMMARY_CALL}\n    print("done")\n'))

    # #148's check reads the file, not the decoded document, so it is the one rule here that has
    # to put bytes on disk. The three together are what make it evidence rather than a grep: the
    # escape is refused, the same character written literally is accepted, and a literal
    # backslash-u -- what a pattern targeting a Go string escape actually contains -- is not
    # mistaken for one. Without the third, a check hollowed into "refuse any backslash" passes
    # the first two and rejects correct set files.
    with tempfile.TemporaryDirectory() as tmp:
        def encoding_case(body):
            path = os.path.join(tmp, "set.json")
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(body)
            return encoding_problems("set.json", path)

        def raw_case(data):
            # Bytes rather than text. The case main exists to survive is a file that cannot be
            # read as UTF-8 at all, and `encoding_case` cannot put those bytes on disk: opening
            # the file to write it is the thing that fails.
            path = os.path.join(tmp, "raw.json")
            with open(path, "wb") as fh:
                fh.write(data)
            return encoding_problems("raw.json", path)

        got, bad_bytes = encoding_case(
            '{"description": "x",\n'
            '  "note": "held \\u2014 until the clock jumps",\n}\n')
        # `rule_fires` takes the complaint the guarded check produced and fails when that is
        # empty, so the escape case hands it `got` directly. A first pass inverted this and the
        # self-test reported "the check it guards cannot fail" about a check that had just
        # refused correctly -- which is at least the right direction to fail in. A second pass
        # wrapped it as `"; ".join(got) or "reported nothing"` to make the message read better,
        # which made the rule unable to fail: the fallback supplies the complaint the check
        # failed to produce, so the empty branch was unreachable and gutting the scanner left
        # the tally at full strength. Measured: `if False and nxt == "u"` printed
        # `34 rules hold`. A complaint the check did not make is not evidence the check made one.
        rule_fires("a JSON-escaped em dash in a set file is refused", "; ".join(got))

        got, bad_bytes = encoding_case(
            '{"description": "x",\n'
            '  "note": "held — until the clock jumps",\n}\n')
        rule("the same note written with the character itself is accepted",
             "" if not got else f"refused a literal em dash: {got}")
        # The flag is only useful if it distinguishes. This file decodes, so main must go on and
        # check its mutants; a flag stuck at True skips every set and prints nothing about it.
        # Paired with the raw case below, which is the same assertion the other way round: each
        # one is the control the other cannot be hollowed into.
        rule("a file that decodes is not reported unreadable",
             None if not bad_bytes else "a readable set file was reported unreadable, so main would "
                                    "skip it and check nothing")

        # The file bytes here are `"old": "\\u00e9"` -- two backslashes -- which JSON decodes to a
        # literal backslash followed by `u`. That is what a pattern matching a Go string escape
        # holds, and it is the case a naive grep for `\u` reports as a defect.
        got, bad_bytes = encoding_case('{"description": "x",\n  "old": "\\\\u00e9",\n}\n')
        rule("a Go string escape in a pattern is not read as a JSON escape",
             "" if not got else f"refused a backslash followed by u: {got}")

        # 0xFF is not a UTF-8 byte in any position, so this is the file the branch in main takes:
        # say so once, in a sentence of our own, and leave the JSON parser alone. Before #151 the
        # run reached `load` instead and died there with `not valid JSON: 'utf-8' codec can't
        # decode byte 0xff`, taking the remaining sets down with the one it could not read.
        got, bad_bytes = raw_case(b'{"description": "x",\n  "note": "\xff",\n}\n')
        rule("bytes that are not UTF-8 are reported unreadable",
             None if bad_bytes else "a set file whose bytes do not decode was not reported "
                                        "unreadable, so main would hand it to the JSON parser "
                                        "and lose every set after it")
        rule_fires("the unreadable file still gets a sentence of its own", "; ".join(got))

    # What a detected problem is worth at the end of the run. These four are the difference
    # between "the checks noticed" and "the run failed", and until now only the second one was
    # ever claimed by a sentence in main. Each half is its own rule because each arrives through
    # its own list; collapsing `or` to one operand is the gutting that used to survive.
    def status_report(got, want):
        return None if got == want else f"exit {got}, want {want}"

    rule("a clean pass exits 0", status_report(exit_status(0, [], [], []), 0))
    rule("a mutant that does not apply exits 1", status_report(exit_status(3, [], [], []), 1))
    # #142: a shared id fails the run without joining the count that says mutants are untested.
    rule("a new colliding mutant id exits 1", status_report(exit_status(0, [], [], ["D9"]), 1))

    def folded_into_the_tally(source):
        return None if "bad += len(collisions)" not in source else (
            "main adds the collision count to `bad`, which feeds the closing sentence 'N of M "
            "mutants would run without testing anything'. Both mutants in a colliding pair run "
            "and test what they say; the defect is in what a citation of their name means (#142), "
            "and counting them there sends the reader to re-point two working mutants.")

    rule("a colliding id is not folded into the untested-mutant tally",
         folded_into_the_tally(body))
    rule_fires("control: a main that folds the collision into the tally is caught",
               folded_into_the_tally(body + chr(10) + "    bad += len(collisions)"))
    rule("a set-file encoding problem alone exits 1",
         status_report(exit_status(0, ["cli.json: JSON escape"], [], []), 1))
    rule_fires("control: the exit-status check reports a wrong code",
               status_report(exit_status(0, ["cli.json: JSON escape"], [], []), 0))

    # Pinning a function is not pinning the line that calls it, and this branch's own claim --
    # that the encoding refusal cannot be deleted -- is a claim about main, not about
    # `exit_status`. Both guttings below were measured on this head before anything read those
    # two lines: handing `exit_status` one list printed the refusal and exited 0, deleting the
    # collection printed nothing and exited 0, and the tally said every rule held either way.
    # Putting them here makes the measurement a rule instead of a review comment, and makes a
    # merge that resolves across these lines break something on purpose. Same idiom as
    # SUMMARY_CALL above: the text is compared by equality, so re-spelling the call is a
    # deliberate edit in two rather than a silent one.
    def call_site_report(source, call):
        return None if call in source else f"main does not contain {call!r}"

    rule("main hands both lists to exit_status", call_site_report(body, STATUS_CALL))
    rule_fires("control: the call-site scan finds exit_status handed one list",
               call_site_report(body.replace(STATUS_CALL, "status = exit_status(bad, [])"),
                                STATUS_CALL))
    rule("main still collects what encoding_problems reported",
         call_site_report(body, COLLECT_CALL))
    rule_fires("control: the call-site scan finds the collection deleted",
               call_site_report(body.replace(COLLECT_CALL, ""), COLLECT_CALL))

    # And now the reason the two pins above are not enough. A text pin says a string is present;
    # it cannot say the value it names reaches the process. Each gutting below keeps both pinned
    # lines byte-for-byte, was run against a set file carrying a \u2014, and ships the refusal
    # with exit 0 anyway: `found = []` one line above the pinned `extend`; `return status` turned
    # into `return 0`; `return status` deleted; `if status:` turned into `if False:`; and the
    # merge that keeps both pinned lines while taking main's tail from the side that never met
    # `exit_status`. That last one is not hypothetical -- it is what a conflict resolution on
    # these very lines produces, because the assignment and the collection are in the hunk and
    # look load-bearing while the tail comes from the other side. Then `status` is computed and
    # dropped, "45 rules hold" prints, CI is green, and the escape ships.
    #
    # So drive main and read the status back. The fixture's bytes are not UTF-8, so main takes
    # the skip branch before `load`, which leaves this branch's own chain -- collection,
    # `exit_status`, the return -- as the only thing that can make the run nonzero. It costs no
    # compile: nothing here reaches `check`, and the self-test stays at 0.05 s.
    def verdict_report(data, want, sentence=None):
        with tempfile.TemporaryDirectory() as box:
            path = os.path.join(box, "set.json")
            with open(path, "wb") as fh:
                fh.write(data)
            out = io.StringIO()
            try:
                with contextlib.redirect_stdout(out):
                    rc = main(["check-mutations.py", path])
            except SystemExit as exc:
                return ("main exited from under the caller (%s), so the sets after this one "
                        "were never checked" % exc)
            # The sentence check goes first because on the escape arm it is the whole claim:
            # a decodable fixture cannot be an empty set (`load` refuses one) and a fixture whose
            # mutant applies would cost a compile, so its `bad` is 1 whatever the escape does and
            # the exit code says nothing about this PR. The pair reads: the skip arm pins that the
            # run is nonzero, this arm pins that the refusal is actually printed.
            if sentence is not None and sentence not in out.getvalue():
                return f"main exited {rc} without printing {sentence!r}"
            return None if rc == want else "main returned %r, want %r" % (rc, want)

    UNDECODABLE = (b'{"description": "x", "expected_mutants": 1, "mutants": [{"id": "A1", "file": "a.go",\n'
                   b'  "old": "\xff", "new": "y"}], "package": "./x/"}\n')
    rule("a set file that cannot be decoded costs the process its exit status",
         verdict_report(UNDECODABLE, 1))
    # The control has to exercise the comparison, not the `SystemExit` arm: a harness whose last
    # line became `return None` still reports the exit it was handed, and a `mutants: []` fixture
    # -- the obvious second choice -- passes on exactly that hollowing, because an empty set is a
    # clean set and main returns 0 for it whether or not the harness can read it. So the control
    # is the same bytes with a promise that is knowingly wrong.
    rule_fires("control: the harness reports a status that is not the one it was promised",
               verdict_report(UNDECODABLE, 0))

    # The arm above is the one a `\u2014` never arrives through. That fixture is not UTF-8 on
    # purpose -- it is what forces the skip branch and leaves the chain observable -- but it also
    # means `main` never walks the half of the loop body this PR exists for, and the pinned
    # `file_problems.extend(found)` cannot see that either: wrap it in `if bad_bytes:` and the
    # string survives verbatim, one indent deeper. Worse, that shape is a merge and not a mutant.
    # `if bad_bytes: ... continue` sits five lines below the collection, inside the hunk this
    # branch wrote, and taking the "skip early" side across those two lines moves one statement
    # that already reads as conditional-adjacent -- the pin holds, the behavioural rule holds
    # because the skip arm still collects, and the refusal is gone: nothing printed, exit 0,
    # CI green, on the branch whose headline claim is that the refusal cannot be deleted.
    #
    # So drive the escape arm as well, and assert the sentence rather than the status. Both
    # fixtures name a file that does not exist, so `check` reports it and nothing compiles: the
    # self-test stays in milliseconds and needs no toolchain.
    ESCAPED = (b'{"description": "x \\u2014 y", "expected_mutants": 1,\n'
               b'  "mutants": [{"id": "A1", "file": "self-test-missing.go",\n'
               b'  "old": "q", "new": "y"}], "package": "./x/"}\n')
    # Same bytes with the character written the way the file should hold it. A clean set prints
    # no refusal, so a sentence check hollowed into always-passing has nothing to report here --
    # which is the only reason this fixture is a control and not a second claim.
    CLEAN = ESCAPED.replace(b'\\u2014', "—".encode("utf-8"))
    rule("an escaped set file is refused out loud, not only by the exit code",
         verdict_report(ESCAPED, 1, "set-file encoding problem"))
    rule_fires("control: the harness reports a refusal sentence that was never printed",
               verdict_report(CLEAN, 1, "set-file encoding problem"))

    # The wiring, not just the function. `mutate.py` reaches these files only through `load`, so a
    # rule that stops at `validate` would leave the battery free to run a set the gate refused --
    # the same split this module was written for, one field later. Both directions are driven on a
    # real file, and the pair is completed by the control below the drives.
    ONE_MUTANT = ('{"package": "./x/", "mutants": [{"id": "T1", "file": "x.go", '
                  '"old": "a", "new": "b"}]}')

    def load_says(text, want):
        """Drive `mutset.load` on a real temp file. `want=None` asks for a clean load."""
        with tempfile.TemporaryDirectory() as box:
            path = os.path.join(box, "set.json")
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(text)
            try:
                loaded = load(path)
            except SystemExit as exc:
                if want is None:
                    return f"load() refused a set it was promised: {exc}"
                return None if want in str(exc) else f"load() refused for the wrong reason: {exc}"
            if want is None:
                return None
            return f"load() accepted a set holding {len(loaded['mutants'])} mutants"

    rule("load() refuses a short set, so the battery cannot run one either",
         load_says(ONE_MUTANT.replace("{", '{"expected_mutants": 2,', 1),
                   "a mutant was deleted"))
    rule("load() still loads a set whose count matches",
         load_says(ONE_MUTANT.replace("{", '{"expected_mutants": 1,', 1), None))
    rule_fires("control: the load harness reports a set it never refused",
               load_says(ONE_MUTANT.replace("{", '{"expected_mutants": 1,', 1),
                         "a mutant was deleted"))

    # #154: the same default, one layer down. The rules above refuse a `\u2014` in a committed set
    # file; these refuse the tool that would write it. The fixtures are source text, so nothing
    # here touches disk until the drive at the end -- and the pair that decides whether this check
    # is worth having is the fourth one, not the first.
    def open_report(body):
        return "; ".join(unencoded_opens(body))

    rule_fires("a text-mode open() with no encoding= is refused",
               open_report('with open(os.path.join(ROOT, f)) as fh:\n    orig = fh.read()\n'))
    rule("the same call with its encoding named is accepted",
         None if not open_report('with open(p, "w", encoding="utf-8") as fh:\n    fh.write(x)\n')
         else "refused a call that names encoding=: "
              + open_report('with open(p, "w", encoding="utf-8") as fh:\n    fh.write(x)\n'))

    # Binary has no encoding to name, and the mode is read off the call rather than off the
    # filename. `capture.png` opened as text is the mistake a path-shaped rule would bless, and it
    # is a line in mutate.py today.
    rule("a binary open is accepted, by its mode and not by its name",
         None if not open_report('with open(path, "rb") as fh:\n    a = fh.read()\n'
                                 'with open(path, mode="wb") as fh:\n    b = fh.write(y)\n')
         else "refused a binary open: "
              + open_report('with open(path, "rb") as fh:\n    a = fh.read()\n'
                            'with open(path, mode="wb") as fh:\n    b = fh.write(y)\n'))

    # The false-positive control, and the one that decides whether this gate survives its own
    # first annoyance: this very file says `open()` without `encoding=` in prose, and mutate.py
    # names the call in comments. A scan that reads lines would fail the checker for describing
    # the defect it looks for -- and the fix for that is always to widen the scan's exception
    # list until it reports nothing.
    rule("the word open() in a docstring or a comment is not a call",
         None if not open_report('"""It opens a file: open() without encoding= is the bug."""\n'
                                 '# open(p) here is prose, not a call\n')
         else "refused prose: "
              + open_report('"""It opens a file: open() without encoding= is the bug."""\n'
                            '# open(p) here is prose, not a call\n'))

    # Per call, not per file. A tool that gets one open right and one wrong is the common case --
    # mutate.py had sixteen of one and none of the other -- and a check that reports the file as
    # guilty names the wrong thing to fix. Both halves: the bad one is still found, and the good
    # one is not swept into it.
    MIXED = ('with open(p, encoding="utf-8") as fh:\n    a = fh.read()\n'
             'with open(q, "w") as gh:\n    b = gh.write(a)\n')
    rule_fires("control: the named open does not excuse the unnamed one", open_report(MIXED))
    rule("and only the unnamed open is reported",
         None if len(unencoded_opens(MIXED)) == 1
         else f"reported {len(unencoded_opens(MIXED))} problems for a file with one")

    # The subprocess arm, which is where the measured crash actually was. The pair that keeps this
    # honest is the third: mutate.py has helpers named `run` of its own, and a rule that flagged
    # those would be widened away within a review cycle.
    rule_fires("a text=True subprocess.run with no encoding= is refused",
               open_report('r = subprocess.run(cmd, capture_output=True, text=True)\n'))
    rule("the same call with its encoding named is accepted",
         None if not open_report('r = subprocess.run(cmd, capture_output=True, text=True,\n'
                                 '                 encoding="utf-8")\n')
         else "refused a subprocess call that names encoding=: "
              + open_report('r = subprocess.run(cmd, capture_output=True, text=True,\n'
                            '                 encoding="utf-8")\n'))
    rule("a subprocess call that keeps its output as bytes is accepted",
         None if not open_report('r = subprocess.run(cmd, capture_output=True)\n'
                                 'r = subprocess.run(cmd, text=False)\n')
         else "refused a byte-mode subprocess call: "
              + open_report('r = subprocess.run(cmd, capture_output=True)\n'
                            'r = subprocess.run(cmd, text=False)\n'))
    rule("the older spelling is caught too",
         None if open_report('r = subprocess.run(cmd, universal_newlines=True)\n')
         else "a universal_newlines run with no encoding= walked past the check")
    rule("a helper of ours that happens to be named run is not subprocess",
         None if not open_report('r = run(cmd, text=True)\nr = go.run(cmd, text=True)\n')
         else "refused a call that is not subprocess: "
              + open_report('r = run(cmd, text=True)\nr = go.run(cmd, text=True)\n'))

    # Fail closed on a file the scanner cannot read. Skipping it is the quiet way this check stops
    # checking: one broken tool at a time, at full green.
    rule_fires("a tool that does not parse is reported rather than skipped",
               open_report('def broken(:\n'))

    # Whether a reported problem reaches the process. The pin first, then the drive, because the
    # pin cannot tell a collected list from one printed and dropped -- the lesson #151's tail
    # learned on the lines directly above.
    rule("main actually runs the tool scan",
         None if SCRIPT_SCAN_CALL in body else f"main does not contain {SCRIPT_SCAN_CALL!r}")
    rule_fires("control: the call-site scan finds the scan deleted",
               None if SCRIPT_SCAN_CALL in body.replace(SCRIPT_SCAN_CALL, "script_problems = []")
               else "the scan no longer sees the line it guards")

    # The other file that runs these guards. `make verify` reaches the encoding pass through
    # scripts/check-scripts.py (#189), which imports the functions from here rather than copying
    # them -- so the copy cannot drift, but the *call* can disappear, and from inside this file
    # that disappearance is silent: the full pass still scans scripts/, still prints its own
    # summary, and `make verify` never runs any of it. Read the same way main's body is read, and
    # fail closed the same way: a check-scripts.py that cannot be read is a missing gate, not a
    # skipped one.
    try:
        with open(os.path.join(ROOT, "scripts", "check-scripts.py"), encoding="utf-8") as fh:
            scripts_body = fh.read()
    except OSError as err:
        problems.append(f"cannot read scripts/check-scripts.py to pin the pre-push gate: {err}")
        scripts_body = ""
    rule("check-scripts.py runs the tool scan through this file",
         None if SCRIPTS_TOOL_CALL in scripts_body
         else f"scripts/check-scripts.py no longer contains {SCRIPTS_TOOL_CALL!r}, so `make "
              f"verify` is not running the guard it was added to run")
    rule("check-scripts.py runs the set-file scan through this file",
         None if SCRIPTS_SET_CALL in scripts_body
         else f"scripts/check-scripts.py no longer contains {SCRIPTS_SET_CALL!r}")
    rule_fires("control: the pin finds the pre-push tool scan deleted",
               None if SCRIPTS_TOOL_CALL in scripts_body.replace(SCRIPTS_TOOL_CALL, "pass")
               else "the pin no longer sees the call it guards")

    def scripts_drive(tool_body, want, sentence=None):
        """Run main over a tree whose scripts/ holds one tool, with no set files at all.

        No sets is the point. With nothing to check, `bad` is 0 and the scan is the only thing
        that can make the run nonzero; hand it a fixture set instead and `bad` carries a number
        whatever the scan does, which is the trap the set-file drive above documents at length.
        """
        global ROOT
        saved = ROOT
        with tempfile.TemporaryDirectory() as box:
            os.makedirs(os.path.join(box, "scripts", "mutations"))
            with open(os.path.join(box, "scripts", "tool.py"), "w", encoding="utf-8") as fh:
                fh.write(tool_body)
            ROOT = box
            out = io.StringIO()
            try:
                with contextlib.redirect_stdout(out):
                    rc = main(["check-mutations.py"])
            except SystemExit as exc:
                return ("main exited from under the caller (%s) instead of returning a status, so "
                        "nothing after it ran" % exc)
            finally:
                ROOT = saved
            if sentence is not None and sentence not in out.getvalue():
                return f"main exited {rc} without printing {sentence!r}"
            return None if rc == want else "main returned %r, want %r" % (rc, want)

    BAD_TOOL = 'import os\nwith open(os.path.join(ROOT, "f")) as fh:\n    x = fh.read()\n'
    CLEAN_TOOL = BAD_TOOL.replace('ROOT, "f")) ', 'ROOT, "f"), encoding="utf-8") ')
    rule("a tool that opens a repo file with no encoding costs the run its exit status, out loud",
         scripts_drive(BAD_TOOL, 1, "script encoding problem"))
    rule("a tool that names its encoding leaves the run at 0", scripts_drive(CLEAN_TOOL, 0))
    rule_fires("control: the drive reports a status that is not the one it was promised",
               scripts_drive(BAD_TOOL, 0, "script encoding problem"))

    # ---- #169: the undo journal -------------------------------------------------------
    # `finally` is this tool's crash insurance and it does not run on SIGKILL, which is what a
    # harness kills a long sweep with. The measured cost: a sweep killed at a 300 s timeout left
    # W205's `new` sitting in internal/web/templates/testlock.tmpl, and the next sweep reported
    # W205 as `matches 0 places` -- a healthy mutant reported as retired, with "re-point it" as
    # the advice. These rules drive a mutant that really lands, so `compiles` is stubbed and the
    # tree is a box; the journal, not the compiler, is what is under test.
    GOOD_SET = ('{"package": "./x/", "expected_mutants": 1, "mutants": '
                '[{"id": "J1", "file": "x.go", "old": "aaa", "new": "bbb"}]}')
    MISS_SET = GOOD_SET.replace('"old": "aaa"', '"old": "zzz"')

    class _RC:
        def __init__(self, code):
            self.returncode, self.stdout, self.stderr = code, "", ""

    def journal_drive(set_text, want_rc, want=None, absent=None, pre=None, dirty=None,
                      argv=None, keeps_journal=False):
        """Drive `main` over a tree whose mutant really lands, then judge what it left behind.

        `pre(box)` runs before the sweep and is how a journal gets planted -- the state a killed
        run leaves is not reproducible by running one and killing it inside a self-test that has
        to stay in milliseconds. `dirty` replaces `git status`, so the corpse note is driven
        without a repository behind the box.
        """
        global ROOT, compiles, dirty_paths
        saved = (ROOT, compiles, dirty_paths)
        with tempfile.TemporaryDirectory() as box:
            os.makedirs(os.path.join(box, "scripts", "mutations"))
            with open(os.path.join(box, "x.go"), "w", encoding="utf-8") as fh:
                fh.write("aaa\n")
            with open(os.path.join(box, "scripts", "mutations", "s.json"), "w",
                      encoding="utf-8") as fh:
                fh.write(set_text)
            if pre:
                pre(box)
            ROOT = box
            compiles = lambda pkg: _RC(0)          # noqa: E731
            dirty_paths = lambda: dirty            # noqa: E731
            out = io.StringIO()
            try:
                with contextlib.redirect_stdout(out):
                    rc = main(argv or ["check-mutations.py"])
            finally:
                ROOT, compiles, dirty_paths = saved
            text = out.getvalue()
            if rc != want_rc:
                return f"the run returned {rc}, want {want_rc}: {text.strip()[:160]}"
            if want and want not in text:
                return f"the run did not print {want!r}"
            if absent and absent in text:
                return f"the run printed {absent!r}, which it was promised not to"
            if os.path.exists(os.path.join(box, JOURNAL_NAME)) and not keeps_journal:
                return (f"{JOURNAL_NAME} survived a finished run, so the next sweep would refuse "
                        f"to read this tree")
            if open(os.path.join(box, "x.go"), encoding="utf-8").read() != "aaa\n":
                return "x.go was left mutated by a run that finished"
            return None

    def journal_planted(box):
        with open(os.path.join(box, JOURNAL_NAME), "w", encoding="utf-8") as fh:
            json.dump({"pid": 1, "files": {"x.go": "zzz\n"}}, fh)

    def journal_garbage(box):
        with open(os.path.join(box, JOURNAL_NAME), "w", encoding="utf-8") as fh:
            fh.write("{not json at all")

    def journal_mutated(box):
        """The corpse: the file is mid-mutation and the journal says what it was."""
        with open(os.path.join(box, "x.go"), "w", encoding="utf-8") as fh:
            fh.write("bbb\n")
        journal_planted(box)
        with open(os.path.join(box, JOURNAL_NAME), "w", encoding="utf-8") as fh:
            json.dump({"pid": 1, "files": {"x.go": "aaa\n"}}, fh)

    def journal_recorded_before_the_write():
        """The claim the whole fix rests on: the record exists BEFORE the mutant lands.

        A journal written in the `finally` would satisfy every other rule here and be worthless,
        because the `finally` is the thing SIGKILL skips. So the stub compiler reads the journal
        and the tree while the mutant is live and says what it found.
        """
        global ROOT, compiles, dirty_paths
        saved = (ROOT, compiles, dirty_paths)
        with tempfile.TemporaryDirectory() as box:
            os.makedirs(os.path.join(box, "scripts", "mutations"))
            with open(os.path.join(box, "x.go"), "w", encoding="utf-8") as fh:
                fh.write("aaa\n")
            with open(os.path.join(box, "scripts", "mutations", "s.json"), "w",
                      encoding="utf-8") as fh:
                fh.write(GOOD_SET)
            found = {}

            def probe(pkg):
                j = journal_load() or {}
                found["exists"] = j.get("files") is not None
                found["original"] = j.get("files", {}).get("x.go")
                found["undo"] = j.get("undo")
                with open(os.path.join(ROOT, "x.go"), encoding="utf-8") as fh:
                    found["live"] = fh.read()
                return _RC(0)

            ROOT, compiles, dirty_paths = box, probe, (lambda: [])
            out = io.StringIO()
            try:
                with contextlib.redirect_stdout(out):
                    main(["check-mutations.py"])
            finally:
                ROOT, compiles, dirty_paths = saved
            if not found.get("exists"):
                return "no journal existed while the mutant was applied -- it is written too late"
            if found.get("original") != "aaa\n":
                return f"the journal holds {found.get('original')!r}, want the pre-run contents"
            if found.get("live") != "bbb\n":
                return "the mutant never reached the tree, so nothing here was ever at risk"
            if not found.get("undo"):
                return "the journal does not say how to undo it"
            return None

    rule("the journal is on disk while the mutant is, holding what to put back",
         journal_recorded_before_the_write())
    rule("a sweep that finishes leaves neither a journal nor a mutation behind",
         journal_drive(GOOD_SET, 0, "apply and compile"))
    rule("a sweep refuses to report on a tree a killed run left mutated",
         journal_drive(GOOD_SET, 1, "--restore", absent="apply and compile",
                       pre=journal_planted, keeps_journal=True))
    rule("a journal that cannot be parsed still stops the run",
         journal_drive(GOOD_SET, 1, "cannot be read", pre=journal_garbage,
                       keeps_journal=True))
    rule("--restore replays the journal, puts the file back, and drops the journal",
         journal_drive(GOOD_SET, 0, "restored 1 file(s)", pre=journal_mutated,
                       argv=["check-mutations.py", "--restore"]))
    rule("a `matches 0 places` line on a dirty tree says it may be a corpse",
         journal_drive(MISS_SET, 1, "left behind", dirty=[" M x.go"]))
    rule("a tree git cannot answer for says it could not look",
         journal_drive(MISS_SET, 1, "could not ask git"))
    rule_fires("control: a clean tree is not told its mutant may be a corpse",
               journal_drive(MISS_SET, 1, "left behind", dirty=[]))

    def killed_mid_sweep(sig):
        """Stop a real sweep with `sig` and report what the tree looks like afterwards.

        The child runs this script for real, in its own box, with `GO` pointed at a stub that
        sleeps -- so the sweep is genuinely holding a mutant when the signal lands and nothing
        about the timing depends on how fast a machine compiles. `git init` is in the box because
        the promise under test is the one an operator actually checks: `git status --porcelain`
        empty (#169's fourth criterion, measured rather than argued).
        """
        box = tempfile.mkdtemp(prefix="plock-kill-")
        try:
            os.makedirs(os.path.join(box, "scripts", "mutations"))
            for name in ("check-mutations.py", "mutset.py"):
                shutil.copyfile(os.path.join(ROOT, "scripts", name),
                                os.path.join(box, "scripts", name))
            tool = os.path.join(box, "slow-go")
            with open(tool, "w", encoding="utf-8") as fh:
                fh.write("#!/bin/sh\nsleep 30\n")
            os.chmod(tool, 0o755)
            with open(os.path.join(box, "x.go"), "w", encoding="utf-8") as fh:
                fh.write("aaa\n")
            with open(os.path.join(box, "scripts", "mutations", "s.json"), "w",
                      encoding="utf-8") as fh:
                fh.write(GOOD_SET)
            for args in (["init", "-q"], ["add", "-A"], ["commit", "-q", "-m", "x"]):
                subprocess.run(["git", "-C", box, "-c", "user.email=t@t.invalid",
                                "-c", "user.name=t"] + args, capture_output=True, check=True)
            # No __pycache__: the box is a git repository and the assertion below is
            # `git status --porcelain`, so an import artefact is a false failure.
            env = dict(os.environ, GO=tool, PYTHONDONTWRITEBYTECODE="1")
            script = os.path.join(box, "scripts", "check-mutations.py")
            child = subprocess.Popen([sys.executable, script], cwd=box, env=env,
                                     stdout=subprocess.DEVNULL, stderr=subprocess.STDOUT)
            journal = os.path.join(box, JOURNAL_NAME)
            for _ in range(250):
                if os.path.exists(journal):
                    break
                time.sleep(0.02)
            else:
                child.kill()
                child.wait()
                return "the sweep never wrote a journal, so there was nothing to interrupt"
            child.send_signal(sig)
            child.wait(timeout=30)
            with open(os.path.join(box, "x.go"), encoding="utf-8") as fh:
                live = fh.read()
            dirty = subprocess.run(["git", "-C", box, "status", "--porcelain"],
                                   capture_output=True, text=True,
                                   encoding="utf-8").stdout.strip()
            if sig == signal.SIGKILL:
                if not os.path.exists(journal):
                    return ("SIGKILL left no journal: the next sweep would read this mutated "
                            "tree and report a healthy mutant as retired (#169)")
                again = subprocess.run([sys.executable, script], cwd=box, env=env,
                                       capture_output=True, text=True, encoding="utf-8",
                                       timeout=120)
                if again.returncode == 0 or "--restore" not in again.stdout:
                    return ("the sweep after a SIGKILL reported on the mutated tree instead of "
                            f"refusing (exit {again.returncode})")
                return None
            if live != "aaa\n" or dirty:
                return (f"SIGTERM left the tree mutated (x.go={live!r}, git status {dirty!r}); "
                        f"the handler is the half that keeps `git status` clean")
            if os.path.exists(journal):
                return ("SIGTERM restored the tree but left the journal, so the next run refuses "
                        "a clean tree")
            return None
        except (OSError, subprocess.SubprocessError) as exc:
            return f"the kill drive could not run: {exc}"
        finally:
            shutil.rmtree(box, ignore_errors=True)

    rule("a sweep stopped by SIGTERM puts the tree back, byte for byte",
         killed_mid_sweep(signal.SIGTERM))
    rule("a sweep stopped by SIGKILL leaves a journal, and the next sweep refuses",
         killed_mid_sweep(signal.SIGKILL))

    # A deleted rule is invisible to every rule that survives it, so the tally is pinned. This
    # is not the error the docstring above warns about: that one *derived* the count from a
    # list and went stale when a case was added. What it catches is deletion by removal --
    # deletion by hollowing is caught by the canary and the controls, not here, and a count is
    # no defence against a rule that was kept and gutted. It is also the one assertion that
    # cannot go through a helper: doing so would change the number it compares.
    if checked != EXPECTED_RULES:
        problems.append(f"expected {EXPECTED_RULES} rules, ran {checked} "
                        "-- one was deleted, or never reached")

    if problems:
        print("self-test FAILED:")
        for p in problems:
            print(f"  {p}")
        return 1
    print(f"self-test: {checked} rules hold")
    return 0


def exit_status(bad, file_problems, script_problems, id_problems):
    """The exit code a finished pass deserves, as a number rather than as a branch in main.

    `if bad or file_problems:` is where a detected problem becomes a failing run, and it was the
    last link in a chain pinned end to end everywhere except there. Nine rules prove the checks
    notice something; nothing proved that noticing is what makes the run fail, so dropping
    `or file_problems` left every rule holding and printed the summary line over an escaped set
    file. Pulled out because a branch in main cannot be handed a case and a function can.

    All four are separate on purpose. A mutant that does not apply, a set file that was generated
    instead of declared, a tool that reads a repo file in the operator's locale, and a mutant id
    two sets have started to share arrive through four lists and are reported by four sentences;
    folding one into another's count attributes somebody else's defect to it, which is the same
    argument that keeps `bad` and `file_problems` apart above. Either one alone has to be enough
    to fail the run.

    The collision list is not folded into `bad` for a reason a reader would notice: `bad` feeds
    the closing sentence "N of M mutants would run without testing anything", and a shared id is
    not that. Both mutants in the colliding pair run and test exactly what they say; the defect
    is in what a citation of their name means (#142). Counting it there would send the reader to
    re-point two working mutants.

    No argument has a default. A default is how a call site that forgot one of these lists would
    keep compiling -- and compiling is what a merge resolution needs in order to look fine.
    """
    return 1 if (bad or file_problems or script_problems or id_problems) else 0


def main(argv):
    if argv[1:] == ["--self-test"]:
        return self_test()
    # Before the argument loop. `--help` used to fall through to `paths = argv[1:]` and be opened
    # as a set file, so the tool that reports encoding problems died with a FileNotFoundError
    # traceback naming `encoding_problems` -- sending the reader to the wrong function for a
    # missing usage line (found by CI on #188).
    if any(arg in ("-h", "--help") for arg in argv[1:]):
        print(USAGE)
        return 0
    if any(arg == "--scripts" for arg in argv[1:]):
        print("`--scripts` is not a flag of this tool; the cheap half is its own file:\n"
              "  python3 scripts/check-scripts.py        (0.05 s, no Go; this runs it too)")
        return 2
    if argv[1:] == ["--restore"]:
        journal = journal_load()
        if journal is None:
            print(f"nothing to undo: no {JOURNAL_NAME} here")
            return 0
        if "unreadable" in journal:
            print(f"{JOURNAL_NAME} exists but cannot be read ({journal['unreadable']}), so "
                  f"nothing was restored. Read `git status --porcelain` and `git diff`, undo the "
                  f"mutation by hand, then remove the journal.")
            return 1
        done = journal_restore(journal)
        print(f"restored {len(done)} file(s) a dead run left behind: "
              + (", ".join(done) if done else "the journal named none"))
        return 0
    paths = argv[1:]
    # Whether this run sees every set, which is the only case in which "N collisions are left" is
    # a fact about the repository rather than about the two files on the command line.
    whole_board = not paths
    if not paths:
        dirn = os.path.join(ROOT, "scripts", "mutations")
        paths = sorted(os.path.join(dirn, f) for f in os.listdir(dirn) if f.endswith(".json"))

    # Before anything is read or written. A run that died with a mutant applied leaves the tree
    # mid-mutation, and reporting on that tree produces the most confident wrong answer this
    # tool can give: a healthy mutant reported as retired (#169).
    stale = journal_load()
    if stale is not None:
        if "unreadable" in stale:
            print(f"{JOURNAL_NAME} exists and cannot be read ({stale['unreadable']}), so this "
                  f"run cannot tell what a dead run left applied here -- which is exactly the "
                  f"tree it must not report on.")
        else:
            print(f"{JOURNAL_NAME} exists: a previous check-mutations.py run died with a mutant "
                  f"applied (pid {stale.get('pid', 'unknown')}, "
                  f"{len(stale.get('files', {}))} file(s) recorded).")
        print("Nothing below is trustworthy on a tree that is still mutated. Run "
              "`python3 scripts/check-mutations.py --restore`, or undo it by hand and delete "
              "the journal.")
        return 1
    # Captured here, before the first write, because after the first write every path this
    # reports is one this process touched.
    dirty = dirty_paths()
    # A second call in the same process (the self-test drives `main` repeatedly) must not carry
    # the previous run's record forward: the journal on disk is per-run, and so is this.
    _JOURNAL.clear()
    # Catchable signals only, and only because they are the ones that arrive: Ctrl-C and the
    # SIGTERM a supervisor sends before it escalates. The uncatchable one is answered by the
    # journal, not by a handler (#169).
    install_undo_signals()

    # Once, before the sets: this looks at the tools that write the files below, and which sets
    # this run was handed has nothing to do with what they open.
    script_problems = script_open_problems(script_files())

    total = bad = 0
    named = []
    file_problems = []
    seen_problems = []
    try:
        for path in paths:
            label = os.path.relpath(path, ROOT)
            # Before `load`, not after it. `mutset.load` opens with encoding="utf-8" and catches
            # ValueError -- which UnicodeDecodeError subclasses -- and then calls sys.exit, so a
            # file whose bytes were not UTF-8 died there printing `not valid JSON: 'utf-8' codec
            # can't decode byte 0xff` and the sentence written for exactly that case never
            # printed. It also took the other twelve sets down with it: there is nothing to check
            # in a file that cannot be read, but there is plenty to check in the files after it.
            found, bad_bytes = encoding_problems(label, path)
            # Deliberately not added to `bad`. A mutant that does not apply and a file that was
            # written by a tool instead of declared are different failures: the closing line
            # counts "N of M mutants would run without testing anything", and folding an
            # encoding defect into that number would attribute a file-level problem to mutants
            # that are fine.
            file_problems.extend(found)
            # The flag the check returns, not a search for a phrase inside the sentences it just
            # built. Matching its prose worked only while the wording held, and when it stopped
            # holding this branch stopped being taken: the run went back to dying in `load` two
            # sets into thirteen, which is the failure the sentence above exists to prevent.
            if bad_bytes:
                print(f"{label}: skipped -- its bytes are not UTF-8, so there is nothing to check")
                continue
            doc = load(path)
            named.append((label, doc))
            problems = check(doc)
            seen_problems.extend(problems)
            total += len(doc["mutants"])
            bad += len(problems)
            if problems:
                print(f"{label}: {len(problems)} of {len(doc['mutants'])} mutants are not testing anything")
                for mid, why in problems:
                    print(f"  {mid}: {why}")
            else:
                print(f"{label}: all {len(doc['mutants'])} mutants apply and compile")
    finally:
        # Every file `check` touched was restored by its own finally; this drops the record that
        # said so. It never runs when the process is stopped by a signal it cannot catch, which
        # is why the journal is written BEFORE the first write rather than here (#169) -- this
        # is the tidy path, not the safety net.
        journal_clear()

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

    # A different failure from one claim made twice, and it lives one level up: two sets each
    # making their own claim under one name. That is the convention for 76 ids already, so the
    # ratchet refuses only a name nobody has collided before (#142) -- a hard rename would
    # invalidate every note and review reply that cites one, which is a worse ambiguity than the
    # one it removes.
    collisions = id_collisions(named)
    if collisions:
        print(f"\n{len(collisions)} new colliding mutant id(s) across these sets (#142):")
        for why in collisions:
            print(f"  {why}")

    if script_problems:
        print(f"\n{len(script_problems)} script encoding problem(s):")
        for why in script_problems:
            print(f"  {why}")
    if file_problems:
        print(f"\n{len(file_problems)} set-file encoding problem(s):")
        for why in file_problems:
            print(f"  {why}")

    # Said after the per-set lines because it is about how to read them. A `matches 0 places`
    # report is the one line in this output a person acts on -- it tells them to re-point a
    # mutant -- and it cannot itself tell a retired mutant from a mutation a dead run left in
    # the tree (#169). Informational, and deliberately not part of the exit status: a dirty tree
    # is the normal state of a working copy, and failing on it would train people to ignore this.
    note = stale_corpse_note(seen_problems, dirty)
    if note:
        print(f"\nnote: {note}")

    # The ratchet's other direction. Retiring a collision is worth saying out loud, because the
    # constant in mutset.py only comes down if someone notices it can, and a ceiling nobody
    # tightens is a ceiling that quietly becomes the new floor. Informational, like the note
    # above: the run is not allowed to fail because it got better.
    still = remaining_collisions(named)
    if whole_board and len(still) < len(GRANDFATHERED_IDS):
        print(f"\nnote: {len(still)} of the {len(GRANDFATHERED_IDS)} grandfathered id collisions "
              f"are still here, so mutset.GRANDFATHERED_IDS can come down to {len(still)}.")

    status = exit_status(bad, file_problems, script_problems, collisions)
    if status:
        if bad:
            print(f"\n{bad} of {total} mutants would run without testing anything.")
            print("Re-point them onto the code that replaced theirs -- updating `old` but not")
            print("`new` is how a mutant ends up failing to compile, which the battery prints as")
            print("a line and checks nothing.")
        return status
    print(f"\n{summary_line(total)}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
