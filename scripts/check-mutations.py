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

import inspect
import os
import subprocess
import sys
import tempfile

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

# How many rules self_test must run. Pinned because a deleted rule is invisible to every rule
# that survives it -- see the assertion in self_test for what that does and does not catch.
EXPECTED_RULES = 38

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

    The generator is fixed, so why keep the gate: the hunk in #133 came from neither writer. It
    came from an ad-hoc `python3 -c` append, which is what the obvious one-liner does by default
    when a mutant is being added at the end of a long session. A convention cannot stop that; a
    refusal can.
    """
    try:
        with open(path, encoding="utf-8") as fh:
            text = fh.read()
    except UnicodeDecodeError as exc:
        return [f"{label} is not valid UTF-8 ({exc}); a set file declares its encoding by being "
                f"UTF-8, not by whatever locale generated it last"]
    out = []
    for offset, esc in json_escapes(text):
        line = text.count("\n", 0, offset) + 1
        # The remedy says "edit this file", not "regenerate it". A first version told the reader
        # to re-run the generator with ensure_ascii=False, and on this tree that deletes CI21 --
        # `gen-cli-mutations.py` emits 20 mutants and the committed set holds 21, because CI21 was
        # appended by hand. Advice that fixes an encoding hunk by silently dropping a mutant is
        # the worse trade, and the gate cannot see the mutant go.
        #
        # The sentence names #150 because it is a claim about today's counts, and nothing pins
        # it: when #150 lands and the generator owns the whole file, this becomes a confidently
        # wrong instruction. The number is the thing to re-check there, so the number is where
        # the ticket is.
        out.append(f"{label}:{line}: {esc} is a JSON escape for a character the file could hold "
                   f"literally -- edit it to the character itself. Do not re-run "
                   f"gen-cli-mutations.py to fix this: it writes fewer mutants than the committed "
                   f"file holds (see #150).")
    return out


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

        got = encoding_case('{"description": "x",\n'
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

        got = encoding_case('{"description": "x",\n'
                            '  "note": "held — until the clock jumps",\n}\n')
        rule("the same note written with the character itself is accepted",
             "" if not got else f"refused a literal em dash: {got}")

        # The file bytes here are `"old": "\\u00e9"` -- two backslashes -- which JSON decodes to a
        # literal backslash followed by `u`. That is what a pattern matching a Go string escape
        # holds, and it is the case a naive grep for `\u` reports as a defect.
        got = encoding_case('{"description": "x",\n  "old": "\\\\u00e9",\n}\n')
        rule("a Go string escape in a pattern is not read as a JSON escape",
             "" if not got else f"refused a backslash followed by u: {got}")

    # What a detected problem is worth at the end of the run. These four are the difference
    # between "the checks noticed" and "the run failed", and until now only the second one was
    # ever claimed by a sentence in main. Each half is its own rule because each arrives through
    # its own list; collapsing `or` to one operand is the gutting that used to survive.
    def status_report(got, want):
        return None if got == want else f"exit {got}, want {want}"

    rule("a clean pass exits 0", status_report(exit_status(0, []), 0))
    rule("a mutant that does not apply exits 1", status_report(exit_status(3, []), 1))
    rule("a set-file encoding problem alone exits 1",
         status_report(exit_status(0, ["cli.json: JSON escape"]), 1))
    rule_fires("control: the exit-status check reports a wrong code",
               status_report(exit_status(0, ["cli.json: JSON escape"]), 0))

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


def exit_status(bad, file_problems):
    """The exit code a finished pass deserves, as a number rather than as a branch in main.

    `if bad or file_problems:` is where a detected problem becomes a failing run, and it was the
    last link in a chain pinned end to end everywhere except there. Nine rules prove the checks
    notice something; nothing proved that noticing is what makes the run fail, so dropping
    `or file_problems` left every rule holding and printed the summary line over an escaped set
    file. Pulled out because a branch in main cannot be handed a case and a function can.

    Both halves are separate on purpose. A mutant that does not apply and a file that was
    generated instead of declared arrive through different lists and are reported by different
    sentences, and either one alone has to be enough to fail the run.
    """
    return 1 if (bad or file_problems) else 0


def main(argv):
    if argv[1:] == ["--self-test"]:
        return self_test()
    paths = argv[1:]
    if not paths:
        dirn = os.path.join(ROOT, "scripts", "mutations")
        paths = sorted(os.path.join(dirn, f) for f in os.listdir(dirn) if f.endswith(".json"))

    total = bad = 0
    named = []
    file_problems = []
    for path in paths:
        label = os.path.relpath(path, ROOT)
        # Before `load`, not after it. `mutset.load` opens with encoding="utf-8" and catches
        # ValueError -- which UnicodeDecodeError subclasses -- and then calls sys.exit, so a file
        # whose bytes were not UTF-8 died there printing `not valid JSON: 'utf-8' codec can't
        # decode byte 0xff` and the sentence written for exactly that case never printed. It also
        # took the other twelve sets down with it: there is nothing to check in a file that cannot
        # be read, but there is plenty to check in the files after it.
        found = encoding_problems(label, path)
        # Deliberately not added to `bad`. A mutant that does not apply and a file that was
        # written by a tool instead of declared are different failures: the closing line counts
        # "N of M mutants would run without testing anything", and folding an encoding defect
        # into that number would attribute a file-level problem to mutants that are fine.
        file_problems.extend(found)
        if any(" is not valid UTF-8 " in w for w in found):
            print(f"{label}: skipped -- its bytes are not UTF-8, so there is nothing to check")
            continue
        doc = load(path)
        named.append((label, doc))
        problems = check(doc)
        total += len(doc["mutants"])
        bad += len(problems)
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

    if file_problems:
        print(f"\n{len(file_problems)} set-file encoding problem(s):")
        for why in file_problems:
            print(f"  {why}")

    status = exit_status(bad, file_problems)
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
