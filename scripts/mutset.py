#!/usr/bin/env python3
"""One load/validate path for the mutation sets, shared by the battery and the pre-flight.

Both scripts read the same JSON files, and until now each had its own idea of what a valid
set is. That drift is not cosmetic, because the two scripts fail in opposite directions:

  - ``check-mutations.py`` required ``mutants`` only. ``mutate.py`` requires ``package``
    *and* ``mutants``. A set that names a package on every mutant but none at the top level
    therefore passes the gate with ``all N mutants apply and compile`` and then refuses to
    run at the battery -- a green tick that checked nothing, which is precisely the failure
    the gate was added to catch.
  - ``mutate.py`` honours ``count`` (a mutant whose ``old`` legitimately appears N times)
    and replaces every occurrence. The gate never read ``count`` and replaced only the
    first, so it rejected a valid multi-occurrence mutant and, had it allowed one, would
    have compiled a different tree than the battery runs.
  - Neither checked whether two mutants in one set share an ``id``. They do not have to be
    the same bug: the runner prints both under one label and reports a tally that counts
    entries, not claims.

So the rules live here once. If a set loads here, both scripts agree it is runnable, and
if it does not, both refuse it with the same sentence.
"""

import json
import sys

# Every key the runner and the gate actually read on a mutant. A key outside this list is a
# key nothing prints, and the two ways that happens are both silent:
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

# Top-level prose keys are deliberately NOT policed. The sets use three spellings for the
# header sentence (`_about`, `note`, `description`) and none of them harms anything: the
# runner ignores all of them, and unlike a mutant-level `_why` they are not posing as a
# rationale the battery was supposed to print. The one top-level mistake that does matter
# -- `packge` where `package` was meant -- is already caught, because every mutant then
# fails the package-resolvability rule below.
DOC_KEYS = {"_about", "note", "description"}


def load(path):
    """Parse one set and refuse it, in the same words, whichever script is asking."""
    try:
        with open(path, encoding="utf-8") as fh:
            doc = json.load(fh)
    except OSError as exc:
        sys.exit(f"{path}: {exc}")
    except ValueError as exc:
        sys.exit(f"{path}: not valid JSON: {exc}")
    problems = validate(doc)
    if problems:
        sys.exit(f"{path}:\n" + "\n".join(f"  {p}" for p in problems))
    return doc


def validate(doc):
    """Return a list of human-readable problems with a set's shape. Empty means runnable."""
    if not isinstance(doc, dict):
        return ["expected an object with 'package' and 'mutants'"]

    problems = []
    mutants = doc.get("mutants")
    if not isinstance(mutants, list) or not mutants:
        problems.append("'mutants' must be a non-empty list")
        return problems
    default_pkg = doc.get("package")
    if default_pkg is not None and not isinstance(default_pkg, str):
        problems.append("'package' must be a string")
        default_pkg = None

    seen = {}
    for i, m in enumerate(mutants):
        where = m.get("id") if isinstance(m, dict) and m.get("id") else f"mutant #{i}"
        if not isinstance(m, dict):
            problems.append(f"{where}: expected an object")
            continue

        for key in sorted(set(m) - KNOWN_KEYS):
            problems.append(
                f"{where}: key {key!r} is read by nothing: whatever it says is in the file "
                "but never printed, and a near-miss spelling of a real key would silently "
                "disable that key"
            )

        for key in ("id", "file", "old"):
            val = m.get(key)
            if not isinstance(val, str) or not val:
                problems.append(f"{where}: {key!r} must be a non-empty string")

        # `old` must be non-empty for a reason the runner makes concrete: str.count("") is
        # len+1, so an empty pattern "applies" everywhere and mutates nothing while the
        # tally reads clean. `new` is the opposite case -- an empty `new` is a DELETION,
        # which is one of the most useful mutations there is (drop the guard, drop the log
        # line), and 19 mutants in these sets are exactly that. Requiring it non-empty
        # would reject a valid set, which is the failure this module exists to prevent.
        if not isinstance(m.get("new"), str):
            problems.append(f"{where}: 'new' must be a string (empty is a deletion)")

        if "count" in m:
            count = m["count"]
            if isinstance(count, bool) or not isinstance(count, int) or count < 1:
                problems.append(f"{where}: 'count' must be an integer of 1 or more, not {count!r}")

        # The runner asks `if m.get("equivalent")`, so anything non-empty marks the mutant
        # equivalent -- and three sets use that to keep the PROOF next to the claim, which
        # is exactly what a reviewer needs when deciding whether to believe an equivalence.
        # A bool is the common case; a string is the proof. What must not happen is a string
        # that says the opposite: "false" is truthy, so it would file a live mutant under
        # equivalent and the tally would read clean.
        if "equivalent" in m:
            eq = m["equivalent"]
            if isinstance(eq, str):
                if not eq.strip():
                    problems.append(f"{where}: 'equivalent' is an empty string, so the runner "
                                    "treats it as not equivalent -- say so by leaving the key out")
                elif eq.strip().lower() in {"false", "no", "0", "none", "never"}:
                    problems.append(f"{where}: 'equivalent' is the string {eq!r}, which is TRUTHY, "
                                    "so the runner files this mutant as equivalent while the prose "
                                    "says it is not -- delete the key, or write the proof")
            elif not isinstance(eq, bool):
                problems.append(f"{where}: 'equivalent' must be true, or a string holding the "
                                "proof that it is")

        # A mutant may name its own package (a set can span packages) or inherit the set's.
        # Neither being present is a defect in the tree; having neither is, because there is
        # then nothing to compile or test.
        pkg = m.get("package", default_pkg)
        if not pkg or not isinstance(pkg, str):
            problems.append(f"{where}: no 'package' on the mutant or the set, so there is "
                            "nothing to run against it")

        mid = m.get("id")
        if isinstance(mid, str) and mid:
            if mid in seen:
                problems.append(
                    f"{where}: id {mid!r} is used twice in this set (also on mutant #{seen[mid]}). "
                    "The runner applies both and prints them under one label, so the tally counts "
                    "two entries for what a reader will take for one claim. If they are the same "
                    "bug, drop one; if they are different bugs, give the second its own id."
                )
            else:
                seen[mid] = i

    return problems


def packages_of(m, default):
    """The package set a mutant is run against, as a frozenset.

    A mutant may name several packages at once -- `run_set` splits the string on spaces, so
    one mutant can ask the same question of two suites -- which is why this returns a set
    rather than a string. Comparing the raw strings would call `./internal/cli/` and
    `./internal/cli/ ./cmd/plock/` different claims, when the second one asks everything the
    first does and more.
    """
    raw = m.get("package", default) or ""
    return frozenset(raw.split())


def duplicate_claims(named_docs):
    """Mutants whose claim is already made by another mutant. ``named_docs`` is [(label, doc)].

    Ids are unique per set file, not globally -- ``A1`` in audit.json and ``A1`` in auth.json
    are different mutants and that is the convention. What is not a convention is one
    (file, old, new) being claimed twice: the tally then reports two entries for one, and
    whoever breaks that line gets the same mutant reported from two places with two different
    notes to reconcile.

    It spans files on purpose -- two sets guarding one shared helper is exactly how the pair
    goes unnoticed, and a set added for one package routinely re-derives a mutant another
    package already has.

    A mutant's claim is "the suite for package P notices mutation M", so the package SET is
    part of the claim, and the test is containment rather than equality: if another mutant
    mutates the same line and runs every package this one does, this one cannot add a result.
    Disjoint package sets are genuinely two claims and are left alone.
    """
    entries = []
    for label, doc in named_docs:
        default = doc.get("package")
        for m in doc.get("mutants", []):
            if not isinstance(m, dict):
                continue
            pkgs = packages_of(m, default)
            entries.append((label, m.get("id"), m.get("file"), m.get("old"),
                            m.get("new"), pkgs))

    problems = []
    for i, (label, mid, rel, old, new, pkgs) in enumerate(entries):
        for j, (label2, mid2, rel2, old2, new2, pkgs2) in enumerate(entries):
            if i == j or (rel, old, new) != (rel2, old2, new2):
                continue
            if not pkgs <= pkgs2:
                continue  # disjoint, or a claim the other does not cover
            if pkgs == pkgs2:
                if i < j:  # report an identical pair once, not twice
                    problems.append(
                        f"{rel}: {label}:{mid} and {label2}:{mid2} are the same mutation, run "
                        f"against the same package(s), so the tally counts 2 claims for one. "
                        "Keep the one whose note explains the bug, or narrow the other to a "
                        "different mutation of that line."
                    )
            else:
                problems.append(
                    f"{rel}: {label}:{mid} runs {sorted(pkgs)} and {label2}:{mid2} runs the same "
                    f"mutation of the same line against a SUPERSET of those packages "
                    f"({sorted(pkgs2)}), so the first can never report anything the second does "
                    f"not already cover -- drop it, or point it at a different mutation."
                )
    return problems
