#!/usr/bin/env python3
"""check-doc-labels.py -- every label name this repository documents is one it carries (#386).

Why this file exists. Six `gh issue create` recipes in `docs/issues-from-d-0058.md` and
`docs/issues-from-d-0065.md` named `guard`, `testing` and `ux`. Measured 19 Nov this repository
carries 18 labels and not one of those three is among them, so all six commands failed the same
way, with the name of the label that stopped them and nothing else:

    $ gh issue create --label guard,testing --title ... --body ...
    could not add label: 'guard' not found          exit 1, and no issue was created

The ticket calls that "a hard failure whose only signal is one stderr line, after a partial
attempt", and it is the class this loop keeps meeting: #317's body had to enumerate all 18 names
inline because nothing anywhere held them, and #437 is a supervisor queue reading one absent name
and reporting an empty result for a label that means nothing here. Nothing in `make verify` had
ever compared a documented name against the set, which is how three of them drifted.

What this checks. One sentence: a name written where `gh` would read it has to be a name the
repository carries. The census is every `--label <names>` token in every text file in the tree,
and the ground is `docs/labels.txt`, a checked-in copy of the live set -- because `gh label list`
needs a network and a token, and a gate that passes quietly when it cannot reach the API is worse
than no gate. `--live` is the answer to the staleness that trade-in buys: it re-reads the API and
refuses drift in either direction, and when it cannot reach the API it prints `SKIPPED` and exits
2, so "verified against the API" is never a phrase this file prints without having asked.

What this deliberately does NOT check, measured rather than assumed:

  * A name that is merely mentioned. `guard`, `testing` and `ux` appear in prose across `docs/`
    and in both decision logs -- H-0304 and the two pages this ticket edited name all three to
    explain the substitution -- and none of that prose is a command. The rule is therefore
    syntactic: only a token introduced by `--label` is a claim about the tracker. `--add-label`,
    `--remove-label` and `labels=` are counted and reported as inert, because those act on an
    issue that already exists and a wrong name there is a visible no-op rather than an issue that
    was never filed.
  * Whether a name is the RIGHT one. Re-pointing `guard` at `kind:chore` is a taxonomy claim with
    a reason in H-0304; this file can say the name exists and cannot say it is correct. A gate
    that refused a label it merely disliked gets widened away inside a week, which is H-0040's
    lesson as recorded at the top of `check-next-md.sh` on this repository's other gate.
  * A name that will be created later. There is no waiver file and no `--waive` flag, on purpose:
    the fix for a name that ought to exist is to create it and add one line to `docs/labels.txt`
    in the same commit, which is one step and leaves the ground file true. A waiver directory is
    how six recipes stay green, and #386 is what six recipes staying green cost.
  * Issue-template `labels:` frontmatter. Measured 19 Nov, `.github/` holds two workflow files and
    no template, so there is nothing here to read. If a template lands, its block is the same
    claim in another syntax and belongs on this scanner rather than in a second gate.
  * Its own fixtures. The exemption is by file identity -- `os.path.samefile` against this
    program -- and it is load-bearing: this docstring and every leg below quote `--label
    guard,testing`, because a gate that could not quote the defect it keeps would be a gate whose
    evidence nobody can re-run. `check-mutations.py` records the same trap for a line-based scan
    and solves it with `ast`; here there is no parse tree to hide in, so the exemption is by
    identity and leg 4 keeps it honest by refusing a COPY of this file, which proves no other
    file can borrow the exemption and that renaming this one does not buy a waiver.

Usage: check-doc-labels.py [--root DIR] [--ground FILE] [--paths F [F ...]] [--live] [--self-test]

Reads and changes nothing. Exit 0 clean, 1 a finding, 2 the ground could not be read or `--live`
could not reach the API -- both of which mean nothing was verified.
"""

import argparse
import contextlib
import io
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(HERE)
GROUND = "docs/labels.txt"
# The environment naming the binary `--live` asks, overridable for the reason check-denies.py
# overrides its own: the self-test has to exercise the live path with no network, so a fixture
# supplies a stand-in that answers the way `gh` does.
GH_ENV = "CHECK_DOC_LABELS_BIN"
LIVE_TIMEOUT = int(os.environ.get("CHECK_DOC_LABELS_TIMEOUT", "60"))

# `--label` and nothing else. The lookbehind is what makes that claim true rather than hopeful:
# without it, a token written after another dash would be read as a claim about the tracker.
# Both spellings `gh` takes are read (`--label a,b` and `--label=a,b`), and a quoted token is read
# whole so `--label "good first issue"` yields the two-word name this repository really carries --
# `\S+` alone would stop at the space and hand back `"good`, which is the shape of a scanner that
# refuses a correct recipe and teaches the reader to widen it.
LABEL_FLAG = re.compile(r"(?<![\w-])--label(?:[ \t]*=[ \t]*|[ \t]+)"
                        r"(\"[^\"]*\"|'[^']*'|\S+)")
# Counted, reported, never judged -- see the docstring.
INERT_FLAG = re.compile(r"(?<![\w-])--(?:add|remove)-label(?:[ \t]*=[ \t]*|[ \t]+)"
                        r"(\"[^\"]*\"|'[^']*'|\S+)"
                        r"|(?<![\w-])\blabels=(\"[^\"]*\"|'[^']*'|\S+)")
# A label name's character set, matched against the WHOLE of a comma-separated part and never
# searched inside it. A part made of anything else is a placeholder (`<names>`), a shell expansion
# (`$LABELS`) or the end of a markdown sentence, and none of those is a name anybody meant -- so
# none of them may be refused as one, and a substring search would refuse `guard` for the `u` in it.
NAME_PART = re.compile(r"[A-Za-z0-9_][A-Za-z0-9_.: -]*")

# A quoted token is one list, so `--label "good first issue"` reads as the two-word name this
# repository really carries instead of as three words that are not names.
QUOTED_LIST = re.compile(r"^([\"'])(.*)\1$")
QUOTE_CHARS = "\"'`"
TRAILING = ")]}>,.;:!?"

SKIP_DIRS = {".git", "bin", "dist", "node_modules", "__pycache__", ".factory", ".comfy",
             ".pytest_cache", ".mypy_cache", "runs"}
MAX_BYTES = 2 * 1024 * 1024


class Finding(Exception):
    """A problem with the check itself -- no ground, no API -- not a finding about the tree."""


def names_in(token):
    """The label names inside one `--label` token, or [] when the token names nothing.

    Comma-separated, because that is the one list form `gh` takes; each part is cleaned of the
    quotes and backticks a markdown author wraps it in and then matched WHOLE against the name
    character set, so a placeholder is not a name and the tail of a sentence is not a name either.
    """
    m = QUOTED_LIST.match(token)
    body = m.group(2) if m else token.rstrip(TRAILING).lstrip("`(")
    out = []
    for part in body.split(","):
        part = part.strip().strip(QUOTE_CHARS).strip()
        if part and NAME_PART.fullmatch(part):
            out.append(part)
    return out


def read_ground(root, ground_rel):
    """The set, in file order, plus every complaint about the file's own shape."""
    path = os.path.join(root, ground_rel)
    if not os.path.isfile(path):
        raise Finding("ground file %s is missing, so there is nothing to compare a documented "
                      "name against and nothing was verified" % ground_rel)
    try:
        with open(path, encoding="utf-8") as fh:
            raw = fh.read()
    except (OSError, UnicodeDecodeError) as exc:
        raise Finding("ground file %s cannot be read (%s) -- nothing was verified"
                      % (ground_rel, exc))
    names, problems, prev = [], [], None
    for n, line in enumerate(raw.splitlines(), 1):
        if line.startswith("#") or not line.strip():
            continue
        if line != line.strip():
            problems.append("%s:%d: a name line carries leading or trailing whitespace, so no "
                            "documented name can ever match it" % (ground_rel, n))
        if "\r" in line:
            problems.append("%s:%d: carries a carriage return, so no documented name will match "
                            "it" % (ground_rel, n))
            line = line.replace("\r", "")
        name = line.strip()
        if name in names:
            problems.append("%s:%d: '%s' is listed twice; the set is a set" % (ground_rel, n, name))
        elif prev is not None and name < prev:
            problems.append("%s:%d: '%s' is out of C order after '%s', so the refresh command's "
                            "output no longer round-trips this file" % (ground_rel, n, name, prev))
        names.append(name)
        prev = name
    if not names:
        problems.append("%s: holds no names at all, so every documented name is absent by "
                        "construction and a clean run here would mean nothing" % ground_rel)
    return names, problems


def fetch_live(root):
    """The live set from the API. Raises Finding when it cannot be had -- loudly, never empty."""
    # Read at call time, not at import: the self-test swaps this for a stand-in that answers like
    # `gh` so the live legs run with no network, and a value frozen at import would make every one
    # of those legs silently exercise the real binary instead. Measured on the first run of this
    # file's self-test, which is how that was found: the leg meant to prove agreement printed the
    # real `gh`'s complaint about a fixture directory that is not a repository.
    binary = os.environ.get("CHECK_DOC_LABELS_BIN", "gh")
    env = dict(os.environ)
    env["GH_PROMPT_DISABLED"] = "1"
    env["NO_COLOR"] = "1"
    try:
        proc = subprocess.run([binary, "label", "list", "--limit", "200", "--json", "name"],
                              capture_output=True, text=True, encoding="utf-8",
                              errors="replace", timeout=LIVE_TIMEOUT, cwd=root, env=env)
    except (OSError, subprocess.SubprocessError) as exc:
        raise Finding("%s could not be run (%s)" % (binary, exc))
    if proc.returncode != 0:
        raise Finding("%s label list exited %d: %s"
                      % (binary, proc.returncode, (proc.stderr or proc.stdout or "").strip()[:200]))
    try:
        rows = json.loads(proc.stdout)
    except (json.JSONDecodeError, ValueError) as exc:
        raise Finding("%s label list did not answer JSON (%s), so its answer is unread rather "
                      "than empty" % (binary, exc))
    if not isinstance(rows, list) or not rows:
        raise Finding("%s label list answered %s; a repository with no labels at all is not "
                      "believable from a call that can read 200 of them, so an empty answer is "
                      "treated as a failure to answer"
                      % (binary, "nothing" if not isinstance(rows, list) else "an empty list"))
    names = [row["name"].strip() for row in rows
             if isinstance(row, dict) and isinstance(row.get("name"), str) and row["name"].strip()]
    if not names:
        raise Finding("%s label list answered %d rows and not one of them carried a name"
                      % (binary, len(rows)))
    return names


def scan_file(path, ground, rel):
    """(findings, commands, names, inert) for one file."""
    try:
        with open(path, encoding="utf-8") as fh:
            text = fh.read()
    except (OSError, UnicodeDecodeError):
        return [], 0, 0, 0          # binary or unreadable: it documents no command
    findings, commands, seen, inert = [], 0, 0, 0
    for n, line in enumerate(text.splitlines(), 1):
        for token in LABEL_FLAG.findall(line):
            names = names_in(token)
            if not names:
                # `--label` with nothing name-shaped after it is prose ABOUT the flag, and prose is
                # what both decision logs and this file's own docstring are made of.
                continue
            commands += 1
            for name in names:
                seen += 1
                if name not in ground:
                    findings.append(
                        "%s:%d: '%s' is not a name this repository carries (%s). `gh` resolves "
                        "the name against the live set, so the command as documented exits 1 with "
                        "`could not add label: '%s' not found` and creates nothing."
                        % (rel, n, name, GROUND, name))
        for groups in INERT_FLAG.findall(line):
            token = next((g for g in groups if g), "")
            if names_in(token):
                inert += 1
    return findings, commands, seen, inert


def walk_documents(root):
    """Every file under *root* small enough and text-like enough to carry a documented command."""
    out = []
    for dirn, dirs, files in os.walk(root):
        dirs[:] = sorted(d for d in dirs if d not in SKIP_DIRS and not d.endswith(".egg-info"))
        for f in sorted(files):
            p = os.path.join(dirn, f)
            try:
                if os.path.getsize(p) <= MAX_BYTES:
                    out.append(p)
            except OSError:
                continue
    return out


def run(root, ground_rel, paths=None, live=False, stream=None):
    """The check. Returns (exit code, count fields)."""
    out = stream or sys.stdout
    ground, findings = read_ground(root, ground_rel)
    ground_set = set(ground)
    source = "the checked-in copy %s" % ground_rel

    # Asked before anything else, so that a run whose whole verdict is about the API still cannot
    # lose the step itself.
    makefile = os.path.join(root, "Makefile")
    if os.path.isfile(makefile):
        try:
            with open(makefile, encoding="utf-8") as fh:
                findings.extend(invocation_problems(fh.read()))
        except (OSError, UnicodeDecodeError) as exc:
            findings.append("Makefile: cannot be read (%s), so nothing can be claimed about whether "
                            "this gate is wired in at all" % exc)

    if live:
        try:
            live_names = fetch_live(root)
        except Finding as exc:
            print("check-doc-labels: SKIPPED -- %s. The census was judged against %s, which was "
                  "NOT verified against the API this run, so a name GitHub has since deleted would "
                  "still read clean here. Exit 2 means nothing was verified."
                  % (exc, ground_rel), file=sys.stderr)
            return 2, {}
        for name in sorted(set(ground) - set(live_names)):
            findings.append("%s: '%s' is in the ground file and the API does not carry it, so "
                            "every command this file calls clean on that name fails at the label "
                            "step" % (ground_rel, name))
        for name in sorted(set(live_names) - set(ground)):
            findings.append("%s: the API carries '%s' and the ground file does not, so a command "
                            "documented with that name reads absent. Refresh: gh label list "
                            "--limit 200 --json name --jq '.[].name' | LC_ALL=C sort"
                            % (ground_rel, name))
        if not findings:
            source = "the live API, re-read this run and identical to %s (%d names each side)" % (
                ground_rel, len(ground))
        ground_set = set(live_names) | ground_set

    files = [os.path.join(root, p) for p in paths] if paths else walk_documents(root)
    here = os.path.abspath(__file__)
    commands = names = inert = 0
    for path in files:
        rel = os.path.relpath(path, root)
        if os.path.abspath(path) == os.path.abspath(os.path.join(root, ground_rel)):
            continue                                  # the ground is data, not a recipe
        try:
            if os.path.samefile(path, here):
                continue                              # see the docstring; leg 4 keeps this honest
        except OSError:
            pass
        f, c, s, i = scan_file(path, ground_set, rel)
        findings.extend(f)
        commands += c
        names += s
        inert += i

    for line in findings:
        print("  " + line, file=out)
    print("check-doc-labels: commands=%d names=%d absent=%d inert=%d files=%d ground=%d "
          "verdict=%s -- ground is %s"
          % (commands, names, len(findings), inert, len(files), len(ground),
             "clean" if not findings else "refused", source), file=out)
    if inert:
        print("check-doc-labels: %d name(s) were named through `--add-label`, `--remove-label` or "
              "`labels=`, which act on an issue that already exists and are reported rather than "
              "judged" % inert, file=out)
    return (1 if findings else 0), {"commands": commands, "names": names, "absent": len(findings),
                                    "inert": inert, "files": len(files), "ground": len(ground)}


# ---------------------------------------------------------------------------------------------
# The guard on the wiring, copied in shape from `check-decisions.py` (#216) and `check-denies.py`
# (#237), which each refuse when their own `verify` step has been deleted. They do that because a
# green gate whose invocation is gone is indistinguishable from a passing gate, and #217 measured a
# guard satisfied by the very text that removed it: a filename substring is true of a comment, of a
# commented-out recipe line, and of an `echo` that names the script while running something else.
# The anchor is therefore a command CHUNK -- the text between `;` on a line make actually hands to a
# shell -- and #237's round seven added the second question, which is whether a red exit from that
# chunk can fail `verify` at all: errexit ignores every command of an AND-OR list but the last, so
# `A --self-test && B` prints nothing and ends 0 with A red.

MAKEFILE = os.path.join(DEFAULT_ROOT, "Makefile")
INVOCATION = re.compile(r"python3[ \t]+scripts/check-doc-labels\.py(?![^\s])")
SELFTEST_INVOCATION = re.compile(r"python3[ \t]+scripts/check-doc-labels\.py[ \t]+--self-test(?![^\s])")


def _status_reaches_shell(chunk, end):
    """Whether a non-zero exit from the invocation ending at `end` can end the recipe.

    `A --self-test && B;` under `set -e` prints nothing and exits 0 when A fails, so "the text is in
    the recipe" and "the recipe fails when this fails" are two claims and only the second is worth a
    gate. Anything after the invocation in the same chunk means something else decides the status.
    """
    tail, head = chunk[end:], chunk[:end]
    if "||" in tail or "&&" in tail or "|" in tail:
        return False
    if "|" in head:
        return False
    return True


def invocation_problems(recipe):
    """Given the text of a Makefile, say whether `verify` still runs this script and its self-test."""
    problems = []
    found = {"plain": False, "self-test": False}
    for raw in recipe.splitlines():
        if raw.lstrip().startswith("#"):
            # make hands this line to no shell at all, including whatever follows a `;` on it.
            continue
        for chunk in raw.split(";"):
            text = chunk.strip()
            if text.startswith("#"):
                continue
            kind = "self-test" if SELFTEST_INVOCATION.match(text) else None
            if kind is None and INVOCATION.match(text):
                kind = "plain"
            if kind is None:
                continue
            match = (SELFTEST_INVOCATION if kind == "self-test" else INVOCATION).match(text)
            if not _status_reaches_shell(text, match.end()):
                problems.append("Makefile: `%s` runs where its exit status cannot reach the shell -- a "
                                "command in an && or || list, or a pipeline, can fail and the recipe "
                                "still ends 0, so this step can never redden `verify`" % text)
                continue
            found[kind] = True
    if not found["plain"]:
        problems.append("Makefile: no recipe command runs `python3 scripts/check-doc-labels.py`, so "
                        "`verify` is not asked whether the documented recipes name labels this "
                        "repository carries -- which is the state #386 was in when it filed: six "
                        "recipes, three absent names, every gate green")
    if not found["self-test"]:
        problems.append("Makefile: no recipe command runs `python3 scripts/check-doc-labels.py "
                        "--self-test`, so the legs that prove this gate can refuse run only when "
                        "someone remembers they exist")
    return problems


# ---------------------------------------------------------------------------------------------
# The self-test. Every leg names the defect it keeps, and a leg that cannot fail is the defect #391
# names for a gate whose proof is vacuous: a gate that prints.

GROUND_TEXT = ("# comment line\naccessibility\narea:core\narea:ui\nbug\ndocumentation\nkind:chore\n")
# The same set with this repository's two real multi-word names in it, in C order -- which is why
# they sit before `kind:chore` and not after it. Leg 8's order rule refused the appended version.
GROUND_ORDERED = ("# comment line\naccessibility\narea:core\narea:ui\nbug\ndocumentation\n"
                  "good first issue\nhelp wanted\nkind:chore\n")
COUNT_KEYS = ("commands", "names", "absent", "inert", "files", "ground")


def _write(root, rel, text):
    path = os.path.join(root, rel)
    os.makedirs(os.path.dirname(path) or root, exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)
    return path


def _run(box, *args):
    """Run the check against *box* and hand back (rc, combined output, count fields)."""
    buf, err = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(buf), contextlib.redirect_stderr(err):
        rc = main(["--root", box] + list(args))
    text = buf.getvalue() + err.getvalue()
    fields = {}
    for word in text.replace("\n", " ").split():
        key, _, value = word.partition("=")
        if key in COUNT_KEYS and value.isdigit():
            fields[key] = int(value)
    return rc, text, fields


def self_test():
    """Every leg, each one a refusal that has to happen. Returns an exit code."""
    fails, boxes = [], []

    def expect(cond, label, detail=""):
        if not cond:
            fails.append("%s%s" % (label, (" -- " + detail) if detail else ""))

    def fixture(files):
        box = tempfile.mkdtemp(prefix="check-doc-labels-")
        boxes.append(box)
        for rel, text in files.items():
            _write(box, rel, text)
        return box

    # Leg 1 -- the defect exactly as #386 filed it: a documented name the set does not hold is
    # refused, each absent word is named, and the refusal says what the command really does.
    box = fixture({"docs/labels.txt": GROUND_TEXT,
                   "docs/recipe.md": "```bash\ngh issue create --label guard,testing --title \\\n"
                                     "  --body \"x\"\n```\n"})
    rc, out, f = _run(box)
    expect(rc == 1, "leg1-absent-passed: a recipe naming `guard` and `testing`, neither of which "
                    "the ground file holds, read clean -- %s" % out.strip()[-200:])
    expect("guard" in out and "testing" in out,
           "leg1-not-named: the refusal did not name the two words that are wrong")
    expect("exits 1" in out and "creates nothing" in out,
           "leg1-no-consequence: the refusal did not say what the documented command actually "
           "does, which is the half #386 was written about")
    expect(f.get("names") == 2 and f.get("absent") == 2,
           "leg1-count: the census counted %s name(s) and %s absent, not 2 and 2"
           % (f.get("names"), f.get("absent")))

    # Leg 2 -- a name the set carries passes, for the reason the reader thinks it does: the same
    # file with the absent words replaced by live ones is the only difference in the box.
    _write(box, "docs/recipe.md", "gh issue create --label bug,area:ui --title x --body y\n")
    rc, out, f = _run(box)
    expect(rc == 0, "leg2-live-name-refused: a name the ground file holds was refused, so the "
                    "comparison is not reading the ground file -- %s" % out.strip()[-200:])
    expect(f.get("names") == 2, "leg2-count: two names documented, %s counted" % f.get("names"))

    # Leg 3 -- the census is not vacuous, which is #391's complaint about a self-test that only
    # ever sees its own happy path: a tree that documents nothing must report zero, and say it in
    # the summary, so `verdict=clean` over an empty census cannot be mistaken for coverage.
    rc, out, f = _run(fixture({"docs/labels.txt": GROUND_TEXT,
                               "docs/quiet.md": "no commands here, only the word guard\n"}))
    expect(rc == 0 and f.get("commands") == 0 and f.get("names") == 0,
           "leg3-vacuous: a tree with no documented command did not report commands=0 names=0 -- %s"
           % out.strip()[-160:])
    expect("commands=0 names=0" in out,
           "leg3-not-printed: the counts are not in the summary line, so a silent-zero run stays "
           "invisible to the reader who has to trust it")

    # Leg 4 -- the exemption is by identity, not by content. A copy of this very file, placed where
    # a document can be, is refused: no other file can borrow the exemption, and renaming this one
    # buys nothing. Without this leg the exemption is a hole with the gate's own filename on it.
    with open(os.path.abspath(__file__), encoding="utf-8") as fh:
        myself = fh.read()
    rc, out, f = _run(fixture({"docs/labels.txt": GROUND_TEXT, "docs/copy-of-gate.py": myself}))
    expect(rc == 1 and "docs/copy-of-gate.py" in out and f.get("absent", 0) >= 2,
           "leg4-exemption-launderable: a copy of this file read clean, so the self-exemption "
           "follows the text and not the file -- %s" % out.strip()[-200:])
    rc, out, f = _run(DEFAULT_ROOT)
    expect(rc == 0 and f.get("absent") == 0,
           "leg4-exemption-not-applied: this file refused itself in its own tree -- %s"
           % out.strip()[-200:])

    # Leg 5 -- the census follows the name, not the directory: a command in a comment in factory/
    # is a recipe somebody will copy, exactly like one in docs/.
    rc, out, f = _run(fixture({"docs/labels.txt": GROUND_TEXT,
                               "factory/h.sh": "#  gh issue create --label notalabel --title x\n"}))
    expect(rc == 1 and "factory/h.sh" in out,
           "leg5-dir-blind: a name documented under factory/ was not censused -- %s"
           % out.strip()[-160:])

    # Leg 6 -- inert forms are reported and never judged. One of them is the sentence this ticket's
    # own two pages write, telling a filing human to add `bug` later.
    rc, out, f = _run(fixture({
        "docs/labels.txt": GROUND_TEXT,
        "docs/x.md": "run `gh issue edit 7 --add-label notalabel`, or "
                     "`gh issue edit 7 --remove-label alsomissing`, or `gh issue create "
                     "labels=missinghere`\nthe words guard and testing appear in prose only\n"
                     "the flag `--label` on its own names nothing\n"
                     "and a placeholder: --label <names> --title x\n"}))
    expect(rc == 0, "leg6-inert-refused: an inert mention was judged as a documented command -- %s"
                    % out.strip()[-200:])
    expect(f.get("inert", 0) >= 3, "leg6-inert-uncounted: three inert mentions in the fixture, %s "
                                   "counted -- an uncounted inert mention is an unwatched one"
                                   % f.get("inert"))
    expect(f.get("commands") == 0 and f.get("names") == 0,
           "leg6-placeholder-counted: a placeholder or a bare flag was read as a documented name "
           "(commands=%s names=%s) -- every decision log in this repository would then be a "
           "finding" % (f.get("commands"), f.get("names")))

    # Leg 7 -- a quoted multi-word name is one name. `good first issue` is in this repository's
    # live set; a scanner splitting on whitespace would refuse it as two absent names, and the
    # usual response to that is to widen the gate, which is how gates die. The ground below is
    # C-sorted with the two-word names in place, because leg 8's order rule caught this fixture
    # when they were appended out of order -- which is that leg working, on its author.
    box7 = fixture({"docs/labels.txt": GROUND_ORDERED,
                    "docs/a.md": "gh issue create --label \"good first issue\" --title x\n",
                    "docs/b.md": "gh issue create --label \"bogus two words\" --title x\n"})
    rc, out, f = _run(box7, "--paths", "docs/a.md")
    expect(rc == 0, "leg7-quoted-refused: a quoted name containing a space was refused -- %s"
                    % out.strip()[-200:])
    rc, out, f = _run(box7, "--paths", "docs/b.md")
    expect(rc == 1 and "bogus two words" in out,
           "leg7-quoted-blind: a quoted multi-word name that is absent was not named -- %s"
           % out.strip()[-160:])

    # Leg 8 -- the ground file's own shape. A duplicate, a stray space and an out-of-order line all
    # make the set lie, and the refresh command in its header is supposed to round-trip it.
    rc, out, f = _run(fixture({"docs/labels.txt": "# c\nbug\n  bug\ndocumentation\narea:core\n",
                               "docs/x.md": "nothing here\n"}))
    expect(rc == 1 and "twice" in out and "C order" in out,
           "leg8-ground-unchecked: a duplicate and an out-of-order ground file read clean -- %s"
           % out.strip()[-200:])

    # Leg 9 -- fail closed with no ground. A gate with nothing to compare against must not print
    # `clean`, which is the same shape as #386's other finding: a check that cannot see is a check
    # that reports success.
    rc, out, f = _run(fixture({"docs/x.md": "gh issue create --label bug --title x\n"}))
    expect(rc == 2, "leg9-no-ground: a tree without %s exited %d instead of refusing to pretend"
                    % (GROUND, rc))
    expect("nothing was verified" in out,
           "leg9-quiet: the missing-ground line did not say that nothing was verified")

    # Leg 10 -- the live path, all three answers, against a stand-in binary so the legs run with no
    # network at all. Agreement has to say so in words a reader can hold the gate to; drift names
    # both directions; an unreachable API is exit 2 and a `SKIPPED` line, never a clean run.
    box10 = fixture({"docs/labels.txt": GROUND_TEXT,
                     "docs/x.md": "gh issue create --label bug --title x\n"})
    fake = _write(box10, "fake-gh.sh", "#!/bin/sh\ncat \"$FAKE_LABELS_JSON\"\n")
    os.chmod(fake, 0o755)
    same = _write(box10, "same.json", json.dumps(
        [{"name": n} for n in GROUND_TEXT.split("\n") if n and not n.startswith("#")]))
    drift = _write(box10, "drift.json", json.dumps([{"name": "accessibility"},
                                                    {"name": "brand-new"}, {"name": "bug"}]))
    os.environ["CHECK_DOC_LABELS_BIN"] = fake
    try:
        os.environ["FAKE_LABELS_JSON"] = same
        rc, out, f = _run(box10, "--live")
        expect(rc == 0 and "re-read this run" in out,
               "leg10-live-agreement: agreeing with the API did not say so -- %s" % out.strip()[-200:])
        os.environ["FAKE_LABELS_JSON"] = drift
        rc, out, f = _run(box10, "--live")
        expect(rc == 1, "leg10-drift-passed: the ground file and the API disagree and the run "
                        "exited %d -- %s" % (rc, out.strip()[-200:]))
        expect("brand-new" in out, "leg10-drift-one-way: the name the API carries and the ground "
                                   "file does not was not named -- %s" % out.strip()[-240:])
        expect("'documentation'" in out or "'area:core'" in out,
               "leg10-drift-other-way: the name the ground file holds and the API does not was not "
               "named -- %s" % out.strip()[-240:])
        os.environ["CHECK_DOC_LABELS_BIN"] = os.path.join(box10, "no-such-gh")
        rc, out, f = _run(box10, "--live")
        expect(rc == 2, "leg10-unreachable-zero: an API that could not be reached exited %d; a "
                        "skipped live check is not a pass" % rc)
        expect("SKIPPED" in out and "NOT verified" in out,
               "leg10-unreachable-quiet: the unreachable line did not say what was not checked -- %s"
               % out.strip()[-200:])
    finally:
        os.environ.pop("CHECK_DOC_LABELS_BIN", None)
        os.environ.pop("FAKE_LABELS_JSON", None)
    rc, out, f = _run(box10)
    expect(rc == 0, "leg10-default-needs-network: the default run asked the API for something and "
                    "the ticket's rule is that the gate works offline -- %s" % out.strip()[-160:])

    # Leg 11 -- the wiring guard, both directions. A gate whose `verify` step has been deleted is
    # indistinguishable from a gate that passed, and #217 measured a guard satisfied by the text that
    # removed it, so this is asserted as command chunks against three Makefiles that delete it.
    if not os.path.isfile(MAKEFILE):
        expect(False, "leg11-no-makefile: %s is not readable from here, so the wiring guard asserts "
                      "nothing -- run this inside the repository" % MAKEFILE)
    else:
        with open(MAKEFILE, encoding="utf-8") as fh:
            real = fh.read()
        expect(not invocation_problems(real),
               "leg11-wiring: this repository's own Makefile no longer satisfies the guard that "
               "checks it does -- %s" % "; ".join(invocation_problems(real)))
        commented = "\n".join(("#\t" + line if "scripts/check-doc-labels.py" in line
                               and not line.lstrip().startswith("#") else line)
                              for line in real.splitlines())
        expect(commented != real, "leg11-comment-noop: the comment mutation found nothing to mutate, "
                                  "so it asserts nothing -- the step is not in the Makefile in the "
                                  "shape this leg documents")
        expect(bool(invocation_problems(commented)),
               "leg11-commented-green: commenting the step out left the guard green, so the step that "
               "catches this step being deleted is not real")
        dropped = "\n".join(ln for ln in real.splitlines()
                            if "check-doc-labels.py --self-test" not in ln)
        expect(dropped != real, "leg11-selftest-noop: this gate has no `--self-test` invocation in "
                                "`verify`, so the mutation below asserts nothing")
        # The absence has to be noticed even though `--self-test` still appears in `verify` for every
        # other gate -- which is exactly the false green check-denies.py records at round six.
        expect(bool(invocation_problems(dropped)),
               "leg11-selftest-dropped: deleting the --self-test invocation left the guard green, so "
               "nothing catches CI losing the only leg that proves this gate can refuse")
        # And putting it back the way it was NOT: `A --self-test && B`, the shape that passed four
        # reviews while a red self-test printed nothing and `verify` said OK.
        chained = real.replace("check-doc-labels.py --self-test; \\",
                               "check-doc-labels.py --self-test && python3 other-gate.py; \\")
        expect(chained != real, "leg11-chained-noop: the step is not in the `; \\[newline]` shape "
                                "this leg mutates, so the && case asserts nothing")
        expect(bool(invocation_problems(chained)),
               "leg11-chained: `A --self-test && B` read green to the guard, so a red self-test would "
               "hide behind errexit and the whole recipe would end 0")

    # Leg 12 -- the repository this file gates, which is not a fixture: the six recipes are the
    # reason it exists, and if a merge re-types an absent name this is the leg that says so.
    rc, out, f = _run(DEFAULT_ROOT)
    expect(rc == 0, "leg12-real-tree: the repository does not read clean -- %s" % out.strip()[-300:])
    expect(f.get("commands", 0) >= 6,
           "leg12-vacuous-on-real: the real tree documents %s labelled command(s); #386 counted "
           "six, so the census is not reaching docs/" % f.get("commands"))

    for box in boxes:
        shutil.rmtree(box, ignore_errors=True)
    if fails:
        print("check-doc-labels --self-test: %d leg(s) FAILED" % len(fails), file=sys.stderr)
        for f in fails:
            print("  self-test: " + f, file=sys.stderr)
        return 1
    print("check-doc-labels --self-test: 12 legs, every refusal demonstrated -- the absent name, "
          "the live name that passes, the empty census, the launderable exemption, the other "
          "directory, the inert forms, the quoted name, the lying ground file, the missing ground, "
          "the live path against a stand-in binary, this Makefile's own wiring with three ways to "
          "delete it, and the real tree")
    return 0


def main(argv=None):
    ap = argparse.ArgumentParser(prog="check-doc-labels.py",
                                 description="Refuse a documented label name the repository does "
                                             "not carry (#386).")
    ap.add_argument("--root", default=DEFAULT_ROOT, help="repository to check (default: this one)")
    ap.add_argument("--ground", default=GROUND, help="the file holding the set (default: %s)" % GROUND)
    ap.add_argument("--paths", nargs="+", default=None, help="check these files, not the whole tree")
    ap.add_argument("--live", action="store_true",
                    help="re-read the set from the API and refuse drift either way (exit 2 if the "
                         "API cannot be reached)")
    ap.add_argument("--self-test", action="store_true", help="prove every leg still refuses")
    args = ap.parse_args(argv)

    if args.self_test:
        return self_test()
    try:
        rc, _ = run(os.path.abspath(args.root), args.ground, args.paths, args.live)
    except Finding as exc:
        print("check-doc-labels: %s" % exc, file=sys.stderr)
        return 2
    return rc


if __name__ == "__main__":
    sys.exit(main())
