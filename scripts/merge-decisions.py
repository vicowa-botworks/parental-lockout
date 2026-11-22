#!/usr/bin/env python3
# shellcheck shell=bash ignore=0
"""merge-decisions.py -- the machine union of the decision logs on a conflicted merge (#381).

The logs are append-only and keyed by id, so the union of two sides is a set operation
the machine can do exactly. It has not been the machine's to do: every merge re-conflicted
every open branch by hand, and the tally that made this a unit of work was 35 conflict ticks
and 14.16 hours, about 30% of the tick budget (`issue #381`, measured over the queue's own
journal). The tickets that lost real content to hand resolution are #360 and #403; the ones
that burned a whole tick on nothing but the junction are #291 (24 minutes) and #334 (48).

WHAT THIS IS. Called inside a conflicted `git merge` in a tree, this script unions the two
decision-log files (`DECISIONS.md`, `factory/DECISIONS.md`) and stages the result -- it does
not commit and it does not push. Those stay with the caller (`factory_resolve_conflict` for
the supervisor; plain `git commit` for a person), because a machine that resolves AND commits
leaves a person no place to look before the history exists.

THE CONTRACT:

  Accept -- an entry one side added and the other lacks; an entry both added with identical
  bytes; an entry both edited with identical bytes; an entry only one side edited (the other
  left it as base had it -- that is what a base sync and a pointer re-aim look like); two
  edits of one entry whose whole difference is DIGITS INSIDE `path:line` COORDINATES -- one
  reading consistently larger (the drifted line coordinate, the shape of 15 of #435's 16
  hunks), or both readings moved the SAME way from base by different amounts, which is one
  seam re-aimed twice by two branches, kept per row at the larger reading because the re-aim
  pass below anchor-verifies the survivor anyway. A digit that differs OUTSIDE any coordinate
  is prose -- a budget, a constant, a sha -- and a changed claim is nobody's sort to settle.
  And a drifted `path:line` citation is re-aimed to the line its anchor text is UNIQUELY on
  in the merged tree.

  Refuse -- everything else, with one machine-readable line naming why. A vocabulary of
  fixed `reason=` tokens, so the journal counts by cause and a model tick is dispatched at
  exactly the residue: a same-id edit that differs by more than coordinate digits (`id-collision`
  -- two claims about one decision, whose resolution belongs to the author), an entry deleted on
  one side but not the other (`dropped-entry` -- the silent loss this component exists to make
  impossible), a side whose log is not strictly ascending (`not-ascending`), a non-log file
  conflicted alongside the logs (`other-conflict` -- the union is atomic: it never stages half
  a resolution), a clean planned path with no stage-0 index entry to restore it from
  (`index-drift`), and a merged tree the shipped gate refuses (`arbiter-red`, `anchor-debt`).
  A refusal costs nothing anywhere it touched: conflicted paths get their conflict regenerated,
  and a CLEAN log the clean-file fixup re-aimed gets back the stage-0 bytes the merge itself
  had staged, in the index and in the worktree -- so the model (or the person the docstring
  above invites to `git commit` by hand) arrives at exactly the state the failed merge left.

WHY NOT `.gitattributes` union merge or `git merge-file --union`: they do a set operation over
LINES, not entries. #362 measured the result on the tail seam -- a duplicated heading and
interleaved fields, a log that reads as two decisions and argues neither. Line union also
keeps BOTH copies of a re-aimed pointer, which is a citation pointing at whatever the reader
finds first. This script treats an entry as the indivisible unit of claim, the only grain at
which "did we lose a decision?" has an answer.

WHY THE ARBITER IS THE SHIPPED GATE: `scripts/check-decisions.py` is the one home for what a
valid log is -- entries, checkpoints, citations, the `POINTER_DEBT` ratchet. Reimplementing
any of it here would be a second home, and second homes are what these decision logs are
full of. The gate is IMPORTED for its parser and citation scanner (pure functions over text)
and run as a SUBPROCESS over the merged tree for the verdict, so the union cannot pass a
check the merge commit's own `make verify` will fail. A union that cannot be certified is
refused, not trusted: the merge goes to a model, and the worst case is the status quo, never
a wrong pointer.

Exit status: 0 everything unioned and staged (or, with --plan-only, would have been); 1
refused, nothing written -- every path a refusal had already written is put back first,
clean fixed-up logs to their stage-0 entry, so exit 1 means nothing moved anywhere; 2
usage, or not a merge in progress. The last is deliberately distinct -- a caller must not
journal "no merge here" as a verdict on log content it never looked at.
"""

import argparse
import importlib.util
import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))

# The gate, by path beside this file, not by module name: `check-decisions` is not importable
# (the hyphen is load-bearing for the CLI), and pinning the file next door is what makes "one
# home for the entry model" a fact a reader can check rather than a hope about sys.path.
_spec = importlib.util.spec_from_file_location("check_decisions_gate",
                                               os.path.join(HERE, "check-decisions.py"))
# The import must not write `__pycache__` beside the gate: the union runs inside a working
# tree, and a stray .pyc is untracked dirt in the tree the merge belongs to. Measured live:
# the #381 selftest box caught a conflict tick quarantining
# `scripts/__pycache__/check-decisions.cpython-314.pyc` -- the union dirtying the very tree
# it was called to heal. Real trees ignore __pycache__ so the quarantine is fixture-visible,
# but the dirt is real either way, and `dont_write_bytecode` covers both worlds at once.
sys.dont_write_bytecode = True
if _spec is None or _spec.loader is None:
    print("UNION-REFUSED reason=no-gate %s" % os.path.join(HERE, "check-decisions.py"))
    sys.exit(2)
gate = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(gate)

LOGS = gate.LOG_REL  # ("DECISIONS.md", "factory/DECISIONS.md") -- the union's whole world
SERIES = {"DECISIONS.md": "D", "factory/DECISIONS.md": "H"}


class Refuse(Exception):
    """One refusal: the fixed reason token, then the detail a journal line carries."""

    def __init__(self, reason, detail):
        super().__init__("%s %s" % (reason, detail))
        self.reason = reason
        self.detail = detail


def git(tree, *args):
    """git in <tree>, or a Refuse that says git refused. argv always, never a shell."""
    proc = subprocess.run(["git", "-C", tree, *args], capture_output=True)
    if proc.returncode != 0:
        raise Refuse("git-refused", "git %s exited %d: %s" % (
            " ".join(args), proc.returncode,
            proc.stderr.decode("utf-8", "replace").strip()[:180]))
    return proc.stdout.decode("utf-8", "replace")


def blob(tree, sha):
    """One stage's bytes, read the gate's own way: `git cat-file`, never a working-file guess."""
    return git(tree, "cat-file", "blob", sha)


def unmerged_paths(tree):
    """Every path with an index stage, in stable order."""
    paths = []
    for line in git(tree, "ls-files", "-u").splitlines():
        path = line.split("\t", 1)[1] if "\t" in line else ""
        if path and path not in paths:
            paths.append(path)
    return paths


def stages(tree, path):
    """{stage: text} (1, 2, 3) for one conflicted path; a deleted side is an absent key."""
    got = {}
    for line in git(tree, "ls-files", "-u", "--", path).splitlines():
        meta, _, p = line.partition("\t")
        parts = meta.split()
        if len(parts) >= 3 and p == path and parts[2] in ("1", "2", "3"):
            got[parts[2]] = blob(tree, parts[1])
    return got


# ------------------------------------------------------------------ entries, as byte blocks

HEADING_SPLIT = re.compile(r"(?m)^(?=## [DH]-\d{4}\s*(?:·|\||-))")


def split_log(text, label):
    """(preamble, [(id, num, raw)]) preserving every byte of the text in its parts.

    The gate's `parse` reads FIELDS; the union competes over the entry as a byte block, and it
    must cut at the SAME grain the gate calls an entry -- so the heading regex is the gate's
    `ENTRY` re-anchored here rather than a second spelling, and `parse` still adjudicates what
    a valid entry is at the arbiter. A union that cut at a different boundary than the gate
    reads would accept exactly at the seam the gate refuses.
    """
    parts = HEADING_SPLIT.split(text)
    blocks = []
    for body in parts[1:]:
        m = gate.ENTRY.match(body.split("\n", 1)[0])
        if m is None:
            # HEADING_SPLIT matched a line ENTRY rejects -- impossible while both regexes say
            # what they say today, and cheap to refuse in case they ever part company.
            raise Refuse("heading-shape", "%s holds a heading the gate's parser rejects: %r"
                         % (label, body.split(chr(10), 1)[0][:80]))
        blocks.append((m.group(1), int(m.group(2)), body))
    return parts[0], blocks


def side_map(label, text):
    """One side's log as a checked view: (preamble, {id: raw}, [ids in file order])."""
    preamble, blocks = split_log(text, label)
    raw = {}
    order = []
    for ident, num, body in blocks:
        if ident in raw:
            raise Refuse("duplicate-id", "%s repeats %s on one side -- the allocator's one rule, "
                         "broken before the merge even began" % (label, ident))
        expected = SERIES.get(label)
        if expected and not ident.startswith(expected + "-"):
            raise Refuse("wrong-series", "%s holds %s -- this log takes %s- ids only, and an id "
                         "filed in the wrong log is a pointer two decisions answer to"
                         % (label, ident, expected))
        raw[ident] = body
        order.append((num, ident))
    if any(order[i][0] >= order[i + 1][0] for i in range(len(order) - 1)):
        raise Refuse("not-ascending", "%s is not strictly ascending by id; an append-only log "
                     "that reordered its history is not a merge input, it is an incident" % label)
    return preamble, raw, [ident for _, ident in order]


def _coord_spans(text):
    """Character spans of the `path:line` coordinate fields in one text -- by the GATE's own
    `CITATION` regex, the same grammar `citation_spans` replays, carrying the scanner's own
    skip rule (a range whose dash swallowed a newline is not a coordinate). A digit run that
    sits outside every span is a PROSE claim -- a measured budget, a ratchet constant, a sha
    -- and the #460 review is why this file refuses to accept one: a rule that decided
    "about 30% of every tick budget" by which side wrote the bigger number decided someone's
    claim, not a line coordinate."""
    return [m.span(2) for m in gate.CITATION.finditer(text) if "\n" not in m.group(0)]


def _digit_runs(text):
    """[(value, is_coordinate)] for every digit run of one text, in order."""
    spans = _coord_spans(text)
    return [(int(m.group(0)), any(s <= m.start() and m.end() <= e for s, e in spans))
            for m in re.finditer(r"\d+", text)]


def coord_pair_diff(ours, theirs):
    """How do two raw blocks of one same-id entry, known to differ, differ in their digits?

      'skeleton'  -- not digit twins: words differ, run counts differ, or a run is a
                     coordinate on one side and prose on the other. `id-collision`.
      'prose'     -- skeletons twin, but a digit OUTSIDE every coordinate differs: a number
                     quoted in running text is a claim that moved. `id-collision`.
      'up'/'down' -- every coordinate run that differs moves the same way between the sides:
                     the drifted line coordinate, the shape of 15 of #435's 16 hunks, whose
                     consistently larger reading is the later truth.
      'mixed'     -- coordinate runs disagree on direction: only the base can arbitrate --
                     two branches re-aiming one seam the same way by different amounts is
                     drift (three_way takes the larger reading per row), anything else is
                     sorting someone's claims.
      'same'      -- no coordinate moved either; unreachable behind the caller's byte check.
    """
    if re.sub(r"\d+", "", ours) != re.sub(r"\d+", "", theirs):
        return "skeleton"
    ro, rt = _digit_runs(ours), _digit_runs(theirs)
    if len(ro) != len(rt):
        return "skeleton"
    gt = lt = 0
    for (a, ia), (b, ib) in zip(ro, rt):
        if ia != ib:
            return "skeleton"
        if not ia:
            if a != b:
                return "prose"
        elif a > b:
            gt += 1
        elif a < b:
            lt += 1
    if gt and lt:
        return "mixed"
    if gt:
        return "up"
    if lt:
        return "down"
    return "same"


def same_direction(base, ours, theirs):
    """Did both sides move every COORDINATE the same way from base (or leave it alone)?

    A prose digit that moved on either side answers False -- a claim changed, and the drift
    rules do not read claims. A classification mismatch answers False for the mirror
    reason: nothing here may decide what a coordinate IS.
    """
    if re.sub(r"\d+", "", base) != re.sub(r"\d+", "", ours):
        return False  # base's own skeleton differs: this is not one file shifting

    rb, ro, rt = _digit_runs(base), _digit_runs(ours), _digit_runs(theirs)
    if not (len(rb) == len(ro) == len(rt)):
        return False
    seen = set()
    for (b, ib), (o, io), (t, it) in zip(rb, ro, rt):
        if not ib == io == it:
            return False
        if not io:
            if o != b or t != b:
                return False
            continue
        for delta in (o - b, t - b):
            if delta:
                seen.add(delta > 0)
    return len(seen) <= 1  # one direction, or nobody moved a coordinate at all


def merge_coord_max(ours, theirs):
    """One text per row from two coordinate twins (the caller proved twin skeletons, equal
    prose runs, equal classification): the LARGER reading per coordinate, prose digits
    taken verbatim. An estimate's record, never a verdict -- repoint anchor-verifies every
    anchored survivor row against the merged tree before the union stages anything, and a
    row with no anchor was never re-aimable by this file anyway.
    """
    mo, mt = list(re.finditer(r"\d+", ours)), list(re.finditer(r"\d+", theirs))
    spans = _coord_spans(ours)
    out, pos = [], 0
    for a, b in zip(mo, mt):
        out.append(ours[pos:a.start()])
        in_coord = any(s <= a.start() and a.end() <= e for s, e in spans)
        out.append(str(max(int(a.group(0)), int(b.group(0)))) if in_coord else a.group(0))
        pos = a.end()
    out.append(ours[pos:])
    return "".join(out)


def three_way(label, ident, base_raw, ours_raw, theirs_raw, stats):
    """The survivor of one entry, under the accept rules; Refuse when the survivor is a choice.

    Base==ours (theirs edited alone) and base==theirs (ours edited alone) are ACCEPTED by
    design, not refused: that shape is a published entry whose pointer one branch re-aimed
    while the other left it alone -- a base sync, the single most common thing a trunk merge
    does to these logs. What no side may do silently is LOSE an entry (`dropped-entry`, keyed
    by presence, checked before this function runs) or contradict a published CLAIM
    (`id-collision`: words differ, or digits outside a coordinate differ -- coordinates are
    what drifts; prose numbers are claims).
    """
    if ours_raw == theirs_raw:
        # Byte-identical resolutions, usually git splitting one identical edit into two hunks.
        return ours_raw
    if base_raw and ours_raw == base_raw:
        stats["theirs"] += 1
        return theirs_raw
    if base_raw and theirs_raw == base_raw:
        stats["ours"] += 1
        return ours_raw
    verdict = coord_pair_diff(ours_raw, theirs_raw)
    if verdict == "same":  # unreachable behind the byte check above; the safe reading.
        return ours_raw
    if verdict in ("up", "down"):
        if base_raw and not same_direction(base_raw, ours_raw, theirs_raw):
            # Every coordinate moved one way BETWEEN the sides, but the sides disagree about
            # which way they each moved FROM base: one branch read the anchor higher than
            # base and the other lower. Merging cannot both add and remove lines above one
            # point, so at most one side saw the truth, and nothing keys which. (Anchored
            # rows still get re-verified by repoint after the survivor is chosen -- the
            # direction rule gates entry to the union; the anchor settles the final one.)
            raise Refuse("id-collision", "%s %s is digits-only on both sides but moved in "
                         "OPPOSITE directions from base -- one branch says the lines moved "
                         "up, the other down, and a merge is one tree; whose reading survives "
                         "is the author's call, not a sort" % (label, ident))
        stats["drift"] += 1
        return ours_raw if verdict == "up" else theirs_raw
    if verdict == "mixed" and base_raw and same_direction(base_raw, ours_raw, theirs_raw):
        # The sides disagree about the MAGNITUDE, not the direction: both re-aimed the same
        # seam the same way, each against its own tree, and the merged tree grew past both
        # readings. Measured live at this branch's own merge of origin/main@b6b8a55: ours
        # re-aimed one published entry's coordinate 2366 -> 2376, trunk re-aimed the same
        # coordinate 2366 -> 2384 -- one seam moved twice, not two claims colliding, and a
        # sort between the sides' numbers was refusing arithmetic. The larger reading per
        # row keeps each side's legitimate re-aim; repoint is the verifier, not this line.
        stats["drift"] += 1
        return merge_coord_max(ours_raw, theirs_raw)
    if verdict == "mixed":
        raise Refuse("id-collision", "%s %s moves coordinates in OPPOSITE directions with no "
                     "base agreement to read as one file growing%s -- whose reading survives "
                     "is the author's call, not a sort"
                     % (label, ident,
                        "" if base_raw else " (no base holds this entry to compare against)"))
    if verdict == "prose":
        raise Refuse("id-collision", "%s %s differs between the sides in a digit OUTSIDE any "
                     "`path:line` coordinate -- a budget, a constant, a sha: prose numbers "
                     "are claims, and whose claim survives is the author's call, not a sort"
                     % (label, ident))
    raise Refuse("id-collision", "%s %s differs on both sides by more than coordinate "
                 "digits -- two claims about one decision, and whose claim survives is the "
                 "author's call, not a sort" % (label, ident))


def union_log(label, base_text, ours_text, theirs_text, stats):
    """One log file, three stages, to merged text -- or Refuse. Writes nothing, touches no git."""
    bp, b_blocks = split_log(base_text, label) if base_text else ("", [])
    b_raw = {i: body for i, _, body in b_blocks}
    b_ids = [i for i, _, _ in b_blocks]
    op, o_raw, o_ids = side_map(label, ours_text)
    tp, t_raw, t_ids = side_map(label, theirs_text)

    preamble = op if op == tp else (tp if op == bp else (op if tp == bp else None))
    if preamble is None:
        raise Refuse("preamble-edit", "%s: both sides rewrote the text above the first entry and "
                     "the two preambles differ; nothing keys those lines, so nothing here may "
                     "choose between them" % label)

    # A base entry that vanished from ONE side is the silent loss this whole component exists
    # to make impossible -- the rebase-accident shape. A decision both sides dropped together
    # is a retraction both branches carry, and stays gone.
    for ident in b_ids:
        if (ident in o_raw) != (ident in t_raw):
            side = "ours" if ident in o_raw else "theirs"
            raise Refuse("dropped-entry", "%s %s is in base and on %s but vanished from the "
                         "other side -- a published decision that disappears from one side of "
                         "a merge is a rebase accident until a person says otherwise"
                         % (label, ident, side))

    merged_raw = {}
    for ident in b_ids:
        if ident not in o_raw:
            continue  # dropped on BOTH sides: a retraction both carry; keep it gone
        merged_raw[ident] = three_way(label, ident, b_raw[ident], o_raw[ident], t_raw[ident], stats)
    for ident in o_ids:
        if ident in merged_raw:
            continue
        if ident in t_raw:
            # One id both sides CREATED: the allocator's collision unless the whole difference
            # is digits (H-0288) or bytes.
            merged_raw[ident] = three_way(label, ident, "", o_raw[ident], t_raw[ident], stats)
        else:
            merged_raw[ident] = o_raw[ident]
            stats["ours"] += 1
    for ident in t_ids:
        if ident not in merged_raw:
            merged_raw[ident] = t_raw[ident]
            stats["theirs"] += 1

    # The log's one invariant: strictly ascending by id, which is what "append-only" MEANS once
    # two sides have appended concurrently. Seating every new id after every smaller id is the
    # ticket's "seat each side's entries at the tail's correct position" -- the correct tail
    # position for an id is relative to ids, not to append order.
    order = sorted(merged_raw, key=lambda i: int(gate.ID_NUM.match(i).group(1)))
    merged = preamble + "".join(merged_raw[i] for i in order)
    if not merged.endswith("\n"):
        merged += "\n"
    # Re-read the union's own output at the gate's grain -- the cheap half of the arbiter, in
    # the right vocabulary: a reassembly that mangled a heading refuses as a union bug, not as
    # someone else's content conflict.
    recount = [b[0] for b in split_log(merged, label)[1]]
    if recount != order:
        raise Refuse("reassembly", "%s: the unioned text does not re-read as the entries the "
                     "union wrote -- a parser disagreement, not a content conflict" % label)
    return merged


# ------------------------------------------------------------------ anchor re-aim

REAIM_ROUNDS = 3


def read_planned(planned, tree):
    """`read(path)` over the merged tree: the planned logs where they are planned, disk elsewhere."""
    def read(rel):
        if rel in planned:
            return planned[rel]
        try:
            with open(os.path.join(tree, rel.replace("/", os.sep)), encoding="utf-8") as f:
                return f.read()
        except (OSError, UnicodeDecodeError):
            return None
    return read


def citation_spans(text, found):
    """Character spans of the coordinate digits in `text`, aligned row-for-row to `found`.

    `scan_citations` answers dicts with no offsets and a rewrite needs them, so the scanner's
    own skip rule (a range whose dash swallowed a newline is not a coordinate) is replayed
    over `CITATION` matches and EVERY row is asserted to agree with its match on
    (written, low, high). That agreement is the whole safety of the shortcut: if the scanner
    ever grows a filter this replay does not carry, the assertion fails and the union refuses
    with `scanner-diverged` -- it never silently renumbers someone's pointer.
    """
    spans = []
    it = gate.CITATION.finditer(text)
    for c in found:
        m = next(it, None)
        while m is not None and "\n" in m.group(0):
            m = next(it, None)
        if m is None:
            return spans, False
        parts = m.group(2).replace(" ", "").split("-")
        if (m.group(1) != c["written"] or int(parts[0]) != c["low"]
                or int(parts[-1]) != c["high"]):
            # A continuation is written bare in the text and resolved upstream by the scanner;
            # its `written` is a resolved path, so only the digits can be compared there.
            if not c["continued"] or (int(parts[0]) != c["low"] or int(parts[-1]) != c["high"]):
                return spans, False
        spans.append(m.span(2))
    return spans, True


def repoint(label, text, read, index, stats):
    """Re-aim drifted `path:line` coordinates in one log. -> (text, hard_rows).

    The rule is the ticket's: re-aim only when the anchor is UNIQUELY on one line of the
    merged tree -- which is exactly the shape a drift leaves, since a drift moves text, it
    does not duplicate it. Everything else joins `hard`, the rows that refuse the union: the
    anchor on two lines, on none, cut by a newline, a current-state verb with no anchor, a
    file the merged tree does not hold. The ticket's tie-break governs every one of them: a
    wrong pointer may be a wrong CLAIM, and refusing with the conflict in place costs the
    merge a model tick; inventing a coordinate could cost the log a decision.

    Rows the shipped gate grandfathered (`unanchored`, no current-state verb) are left
    untouched and uncounted here; the ratchet pre-check counts them below and the subprocess
    arbiter judges them. Re-aiming converges: a re-numbered coordinate cannot move an anchor
    (no line is added or removed), so round 2 confirms and stops; the cap is paranoia with a
    reason attached, not magic.
    """
    for _round in range(REAIM_ROUNDS):
        entries = gate.parse(text, label)
        found = gate.scan_citations(text, entries, index)
        spans, aligned = citation_spans(text, found)
        if not aligned:
            raise Refuse("scanner-diverged", "%s: the gate's citation rows no longer line up "
                         "with the citations in this text -- the union and the gate disagree "
                         "about what a pointer is, so the union defers to a model" % label)
        rewrites = []
        hard = []
        for c, span in zip(found, spans):
            if c["kind"] in ("not-a-path", "revision-qualified"):
                continue
            if c["kind"] in ("ambiguous", "bad-range", "file-missing"):
                # Every one of these the shipped gate also refuses on its own account; naming
                # it here puts the sentence beside the merge that made it.
                hard.append("%s:%d (%s) `%s:%s` is %s -- a pointer the reader cannot open is "
                            "not a drifted pointer; no re-numbering fixes a wrong file"
                            % (c["written"] or ":", c["low"], c["entry"],
                               c["written"] or ":", c["low"], c["kind"]))
                continue
            if c.get("wrapped"):
                hard.append("%s:%d (%s) has its anchor across a newline -- the gate refuses "
                            "that shape and re-wrapping prose is a model's edit, not a "
                            "re-numbering" % (c["written"] or ":", c["low"], c["entry"]))
                continue
            body = read(c["path"])
            if body is None:
                hard.append("%s:%d (%s) cites %s, which the merged tree does not hold"
                            % (c["written"] or ":", c["low"], c["entry"], c["path"]))
                continue
            lines = body.splitlines()
            want = gate.norm(c["anchor"]) if c["anchor"] is not None else None
            if want is None:
                if c.get("now"):
                    hard.append("%s:%d (%s) is a current-state citation with no anchor -- the "
                                "gate refuses the bucket and the repair is prose"
                                % (c["written"] or ":", c["low"], c["entry"]))
                continue
            length = c["high"] - c["low"] + 1
            if 1 <= c["low"] <= len(lines) and \
                    want in gate.norm(" ".join(lines[c["low"] - 1:c["low"] - 1 + length])):
                continue  # true in the merged tree already -- nothing drifted on this row
            starts = [s for s in range(1, len(lines) - length + 2)
                      if want in gate.norm(" ".join(lines[s - 1:s - 1 + length]))]
            if len(starts) == 1:
                rewrites.append((span, c["low"], starts[0]))
            else:
                hard.append("%s:%d (%s) anchors to %r which sits on %d line(s) of %s after the "
                            "merge -- re-aim needs a unique home, and guessing one is exactly "
                            "the wrong-claim risk the ticket closes the door on"
                            % (c["written"] or ":", c["low"], c["entry"], c["anchor"][:48],
                               len(starts), c["path"]))
        if hard:
            return text, hard
        if not rewrites:
            return text, []
        for span, low, new_low in sorted(rewrites, key=lambda r: r[0][0], reverse=True):
            delta = new_low - low
            # `span` IS the coordinate's digit field (citation_spans took m.span(2)), so the
            # whole field shifts by one delta: a range's `high` moves with its `low` because
            # the anchor window keeps its length. Descending order keeps earlier spans valid.
            text = (text[:span[0]]
                    + re.sub(r"\d+", lambda mo: str(int(mo.group(0)) + delta),
                             text[span[0]:span[1]])
                    + text[span[1]:])
            stats["repointed"] += 1
    raise Refuse("repoint-loop", "anchor re-aim did not converge in %d rounds" % REAIM_ROUNDS)


# ------------------------------------------------------------------ the arbiter

def arbiter(tree):
    """The shipped gate over the merged tree, as a subprocess: one home for validity.

    Run from the TREE's copy of the gate -- the tree being merged carries the newest
    expectation -- so the union cannot pass a check the merge commit's own `make verify` will
    fail. The whole verdict comes back: entries, checkpoints, cross-log ids, teaches, the
    ratchet. Its exit is the verdict; the union never re-derives it.
    """
    script = os.path.join(tree, "scripts", "check-decisions.py")
    if not os.path.isfile(script):
        # Not the tree's own gate means no verdict at all, and a union that cannot be checked
        # is refused, not trusted.
        raise Refuse("no-gate", "scripts/check-decisions.py is not in the tree being merged, "
                     "so nothing can certify this union; the merge belongs to a model")
    proc = subprocess.run([sys.executable or "python3", script], cwd=tree, capture_output=True,
                          env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"))
    if proc.returncode != 0:
        out = proc.stdout.decode("utf-8", "replace") + proc.stderr.decode("utf-8", "replace")
        first = [ln.strip() for ln in out.splitlines() if ln.strip()][:3]
        raise Refuse("arbiter-red", "the shipped gate refused the merged tree (first: %s) -- "
                     "the conflicted bytes are back in place" % " / ".join(first)[:300])


# ------------------------------------------------------------------ the pass

def plan(tree):
    """Compute the whole resolution without touching a byte. -> (planned {path: text}, fields)."""
    paths = unmerged_paths(tree)
    if not paths:
        raise Refuse("nothing-unmerged", "the index holds no conflicted path -- this runs "
                     "between a failing `git merge` and its abort, not before either")
    logs = [p for p in paths if p in LOGS]
    if not logs:
        raise Refuse("no-log-conflict", "neither decision log is conflicted here (%s) -- a "
                     "merge with no append-only file in it holds no judgement this machine "
                     "owns" % ", ".join(sorted(paths)[:4]))
    others = sorted(set(paths) - set(logs))
    if others:
        # Atomic by choice: staging the logs while a script stays conflicted hands the next
        # `git commit` a half-resolved merge -- the opposite of "the model gets the residue it
        # was promised".
        raise Refuse("other-conflict", "the logs union cleanly but %s also conflicts -- "
                     "nothing is staged until the whole file set is machine-resolvable"
                     % ", ".join(others))
    stats = {"ours": 0, "theirs": 0, "drift": 0, "repointed": 0}
    planned = {}
    merged_entries = 0
    for path in logs:
        got = stages(tree, path)
        missing = [side for side in ("2", "3") if side not in got]
        if missing:
            names = " and ".join({"2": "ours", "3": "theirs"}[s] for s in missing)
            raise Refuse("side-deleted", "%s is deleted on %s -- a log that vanishes from one "
                         "side is not an append conflict, and `git` already offered a choice no "
                         "machine should take silently" % (path, names))
        before = merged_entries
        merged = union_log(path, got.get("1", ""), got["2"], got["3"], stats)
        merged_entries = before + len(gate.parse(merged, path))
        planned[path] = merged

    # Re-aim runs over BOTH logs, not only the conflicted ones: a trunk merge shifts line
    # numbers in files that merge CLEANLY, and the drifted pointers in a clean log are exactly
    # as broken in the merged tree as the ones in a conflicted log. A clean log that gets
    # touched becomes part of the resolution (staged with it) -- the ticket's "clean-file
    # fixup", and the reason the backstop cannot be red on a machine-resolved merge.
    paths_in_tree, problem = gate.tree_files(tree)
    if problem:
        raise Refuse("unreadable", problem)
    index = gate.tree_index(paths_in_tree)
    read = read_planned(planned, tree)
    hard = []
    for path in LOGS:
        text = planned.get(path)
        if text is None:
            text = read(path)
            if text is None:
                continue
        new_text, rows = repoint(path, text, read, index, stats)
        if rows:
            hard += rows
        elif new_text != text:
            planned[path] = new_text
        read = read_planned(planned, tree)  # later logs' citations into an earlier fixed log
    if hard:
        raise Refuse("anchor-debt", "%d citation(s) cannot be re-aimed uniquely after the merge "
                     "(first: %s) -- refusing with the conflict in place is the ticket's "
                     "tie-break: a wrong pointer may be a wrong claim"
                     % (len(hard), hard[0][:300]))

    # The ratchet, counted here ONLY to name it in the refusal's own vocabulary -- the
    # subprocess arbiter still decides. A refusal that says "the debt rose" is actionable;
    # `arbiter-red` with the gate's sentence buried in captured stdout is not.
    unanchored = 0
    total_entries = 0
    for path in LOGS:
        text = planned.get(path) or read(path)
        if text is None:
            continue
        entries = gate.parse(text, path)
        total_entries += len(entries)
        unanchored += len(gate.classify(gate.scan_citations(text, entries, index), read)[0])
    if gate.debt_applies(total_entries) and unanchored > gate.POINTER_DEBT:
        raise Refuse("anchor-debt", "the merged pair carries %d unanchored citation(s) over the "
                     "recorded debt of %d -- lowering POINTER_DEBT is a model's commit, never "
                     "this script's" % (unanchored, gate.POINTER_DEBT))
    fields = "entries=%d ours=%d theirs=%d drift=%d repointed=%d files=%d" % (
        merged_entries, stats["ours"], stats["theirs"], stats["drift"], stats["repointed"],
        len(logs))
    return planned, fields


def restore_point(tree, path):
    """The cacheinfo spec of one clean path's stage-0 index entry, or None if it is
    conflicted, or Refuse if it is neither. A clean path is restored from the stage-0
    entry the merge itself staged -- captured BEFORE `git add` overwrites it, because
    `checkout -m` on a path with no stages is a silent no-op (measured twice: the #460
    review, and the G8 leg's control, which caught the refused run's re-aim WRITTEN AND
    STAGED in the clean log). A planned path that is neither conflicted nor a single
    stage-0 entry has no bytes any rollback could honestly put back, so it is refused
    before anything is written."""
    if git(tree, "ls-files", "-u", "--", path).strip():
        return None
    stage0 = [ln for ln in git(tree, "ls-files", "--stage", "--", path).splitlines()
              if ln.split("\t", 1)[-1] == path]
    if len(stage0) != 1:
        raise Refuse("index-drift", "%s is neither conflicted nor one stage-0 index entry -- "
                     "a planned path whose pre-merge bytes no rollback could restore is "
                     "refused before anything is written" % path)
    meta = stage0[0].split("\t", 1)[0].split()
    return "%s,%s,%s" % (meta[0], meta[1], path)


def run(tree, plan_only=False):
    """Plan, then -- and only then -- write, stage, and certify. Raises Refuse otherwise."""
    planned, fields = plan(tree)
    if plan_only:
        return fields
    restore = {path: restore_point(tree, path) for path in planned}
    for path, text in planned.items():
        with open(os.path.join(tree, path.replace("/", os.sep)), "w", encoding="utf-8",
                  newline="") as f:
            f.write(text)
        git(tree, "add", "--", path)
    try:
        arbiter(tree)
    except Refuse as refuse:
        # Back to exactly the failed-merge state, every planned path, as if this script had
        # never run. `checkout -m` is the verb that regenerates a conflict -- and it reaches
        # only paths whose stages the index still holds. The clean paths a fixup re-aimed
        # have no stages to regenerate: their restore point is the stage-0 entry snapshotted
        # above, put back in the INDEX first (`update-index --cacheinfo`) so the `checkout`
        # that follows restores the worktree to the merge's own bytes, not to ours.
        for path in planned:
            if restore[path] is not None:
                subprocess.run(["git", "-C", tree, "update-index", "--cacheinfo",
                                restore[path]], capture_output=True)
                subprocess.run(["git", "-C", tree, "checkout", "--", path], capture_output=True)
            else:
                subprocess.run(["git", "-C", tree, "checkout", "-m", "--", path],
                               capture_output=True)
        raise refuse
    return fields


def main(argv):
    parser = argparse.ArgumentParser(prog="merge-decisions.py",
                                     description="machine union of the decision logs on a "
                                                 "conflicted merge (#381)")
    parser.add_argument("--tree", default=".", help="the tree holding the conflicted merge")
    parser.add_argument("--plan-only", action="store_true",
                        help="compute and report; write nothing (the supervisor's dry run)")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args(argv[1:])
    if args.self_test:
        return self_test()
    tree = os.path.abspath(args.tree)
    if not os.path.exists(os.path.join(tree, ".git")):
        print("UNION-REFUSED reason=not-a-repo %s is not a working tree" % tree)
        return 2
    probe = subprocess.run(["git", "-C", tree, "rev-parse", "-q", "--verify", "MERGE_HEAD"],
                           capture_output=True)
    if probe.returncode != 0:
        print("UNION-REFUSED reason=not-merging %s holds no merge in progress -- a verdict "
              "about the tree, not the logs, and it exits 2 for exactly that reason" % tree)
        return 2
    try:
        fields = run(tree, plan_only=args.plan_only)
    except Refuse as refuse:
        print("UNION-REFUSED reason=%s %s" % (refuse.reason, refuse.detail))
        return 1
    print("UNION %s" % fields)
    return 0


# ------------------------------------------------------------------ self-test

def _sh(*args, cwd=None):
    subprocess.run(args, cwd=cwd, check=True, capture_output=True)


def _sh_rc(*args, cwd=None):
    return subprocess.run(args, cwd=cwd, capture_output=True).returncode


def _entry(ident, title, checkpoint, why=None):
    return ("## %s · %s\n\n- Question: what does %s decide?\n- Options: this | that | neither\n"
            "- Chose: this\n- Why: %s\n- Checkpoint: %s\n\n"
            % (ident, title, ident, why or "because the fixture says so, in one clause",
               checkpoint))


def self_test():
    """Legs in two families, split by what git can be TRUSTED to produce.

    Git legs drive real conflicted merges in throwaway repos, but only in the shapes this
    file's own micro-probes measured as reliably conflicted (both sides appending full
    entries at the tail; the same line edited on both sides; notes.md rewrites). An earlier
    draft also bet legs on "an edit here plus an append there must overlap" -- the probe
    falsified that (edit-last-line vs append-EOF and edit-mid-block vs append-EOF both merge
    CLEAN), and a leg that silently stops being a conflict tests nothing at all. The
    adjudication rules therefore live in UNIT legs that call `union_log` and `repoint` over
    text directly: deterministic, and the whole accept/refuse vocabulary is exercised without
    xdiff roulette. Nothing is committed into this repository (the #339 rule).

    Every git leg keeps its control: the merge really conflicted before the union ran, and
    every refusal leg asserts the conflicted bytes are still there -- the promise that the
    model gets the state it was shown.
    """
    state = {"ok": 0, "bad": 0}

    def check(label, cond, why=""):
        if cond:
            state["ok"] += 1
            print("  ok    %s" % label)
        else:
            state["bad"] += 1
            print("  FAIL  %s%s" % (label, (" -- " + why) if why else ""))

    PRE = "# Product decisions\n\n"

    def uni(label, base, ours, theirs):
        """union_log with a fresh stats dict -> ('ok', text, stats) or ('refuse', reason, detail)."""
        stats = {"ours": 0, "theirs": 0, "drift": 0, "repointed": 0}
        try:
            text = union_log(label, base, ours, theirs, stats)
        except Refuse as refuse:
            return "refuse", refuse.reason, refuse.detail
        return "ok", text, stats

    def e(ident, title, cp, body=None):
        return _entry(ident, title, cp) if body is None else (
            "## %s · %s\n\n- Question: what does %s decide?\n- Options: this | that | neither\n"
            "- Chose: this\n- Why: %s\n- Checkpoint: %s\n\n" % (ident, title, ident, body, cp))

    # ---- unit legs: the accept/refuse vocabulary, straight against the rules ----------------
    u1 = e("D-0001", "base", "c1")
    # one-sided edit of a published entry (base sync): the accept that a both-sides-only
    # rule would refuse -- the most common thing a trunk merge does to these logs.
    kind, got, extra = uni("DECISIONS.md", PRE + u1,
                           PRE + e("D-0001", "base", "c1", "ours re-aimed a pointer here"),
                           PRE + u1)
    check("unit: a one-sided edit of a published entry is ACCEPTED (base-sync shape)",
          kind == "ok" and "ours re-aimed a pointer here" in got, str(extra))
    # identical edit both sides: accept, exactly once.
    both = PRE + e("D-0001", "base", "c1", "both typed this same sentence")
    kind, got, extra = uni("DECISIONS.md", PRE + u1, both, both)
    check("unit: two identical edits of one entry accept, present exactly once",
          kind == "ok" and got.count("both typed this same sentence") == 1, str(extra))
    # words collide: two claims about one decision.
    kind, got, extra = uni("DECISIONS.md", PRE + u1,
                           PRE + e("D-0001", "base", "c1", "ours rewrote the Why on purpose"),
                           PRE + e("D-0001", "base", "c1", "theirs rewrote the Why otherwise"))
    check("unit: same-id edits differing by WORDS refuse with id-collision",
          kind == "refuse" and got == "id-collision", str(extra))
    # drift: the whole difference is digits and one side is consistently larger.
    drift_base = PRE + e("D-0001", "base", "c1", "the gate lives at `x.py:3` now")
    kind, got, extra = uni("DECISIONS.md", drift_base,
                           PRE + e("D-0001", "base", "c1", "the gate lives at `x.py:5` now"),
                           PRE + e("D-0001", "base", "c1", "the gate lives at `x.py:7` now"))
    check("unit: two drifted re-aims of one pointer accept, the LARGER coordinate wins",
          kind == "ok" and "`x.py:7`" in got and extra["drift"] == 1, str(extra))
    # drift with mixed directions is not drift: two different edits wearing digits.
    kind, got, extra = uni("DECISIONS.md", drift_base,
                           PRE + e("D-0001", "base", "c1", "the gate lives at `x.py:2` now"),
                           PRE + e("D-0001", "base", "c1", "the gate lives at `x.py:9` now"))
    check("unit: mixed-direction digit edits refuse -- drift moves one way or it is a claim",
          kind == "refuse" and got == "id-collision", str(extra))
    # #460 finding 2, on the reviewer's own fixture: a digit outside any coordinate is a
    # PROSE claim. The skeleton twins and only the number moves; size must not adjudicate.
    prose_base = PRE + e("D-0001", "base", "c1", "the ceiling is 2700 s of every tick")
    kind, got, extra = uni("DECISIONS.md", prose_base,
                           PRE + e("D-0001", "base", "c1", "the ceiling is 2701 s of every tick"),
                           PRE + e("D-0001", "base", "c1", "the ceiling is 4200 s of every tick"))
    check("unit: digits differing OUTSIDE coordinates refuse -- a budget is a claim, not a "
          "coordinate (the pre-fix rule handed the entry to the larger number)",
          kind == "refuse" and got == "id-collision", str(extra))
    # The same seam re-aimed twice, once per branch (measured live at this branch's merge of
    # origin/main@b6b8a55): both sides moved the same direction from base, by different
    # amounts. Pre-fix this refused as id-collision on the live merge; the per-row larger
    # reading keeps each side's legitimate re-aim, and repoint arbitrates against the tree.
    seam = ("- Why: the row sits at `a.py:%d` and the block at `b.md:%d` per their anchors.\n")
    seam_base = PRE + ("## D-0001 · seam fixture\n\n- Question: where does the seam sit?\n"
                       "- Options: re-aim | refuse\n- Chose: re-aim\n" + seam % (3, 40)
                       + "- Checkpoint: c1\n\n")
    seam_ours = PRE + ("## D-0001 · seam fixture\n\n- Question: where does the seam sit?\n"
                       "- Options: re-aim | refuse\n- Chose: re-aim\n" + seam % (5, 40)
                       + "- Checkpoint: c1\n\n")
    seam_theirs = PRE + ("## D-0001 · seam fixture\n\n- Question: where does the seam sit?\n"
                         "- Options: re-aim | refuse\n- Chose: re-aim\n" + seam % (3, 45)
                         + "- Checkpoint: c1\n\n")
    kind, got, extra = uni("DECISIONS.md", seam_base, seam_ours, seam_theirs)
    check("unit: both branches re-aiming the same seam the SAME way by different amounts "
          "accept, each side's larger reading kept per row",
          kind == "ok" and "`a.py:5`" in got and "`b.md:45`" in got and extra["drift"] == 1,
          str(got)[:200])
    # ...and without a base to compare against, an opposite-direction pair is unprovable.
    kind, got, extra = uni("DECISIONS.md", PRE,
                           PRE + e("D-0002", "minted both", "c1", "see `p.py:9` and `q.py:20` now"),
                           PRE + e("D-0002", "minted both", "c1", "see `p.py:3` and `q.py:30` now"))
    check("unit: opposite-direction coordinate edits with NO base refuse -- nothing proves "
          "one file grew, so the author decides", kind == "refuse" and got == "id-collision",
          str(extra))
    # dropped on one side vs retraction on both.
    kind, got, extra = uni("DECISIONS.md", PRE + u1, PRE + u1 + e("D-0002", "ours", "c1"),
                           PRE + e("D-0003", "theirs dropped base", "c1"))
    check("unit: an entry vanished from ONE side refuses with dropped-entry (silent-loss guard)",
          kind == "refuse" and got == "dropped-entry", str(extra))
    kind, got, extra = uni("DECISIONS.md", PRE + u1, PRE + e("D-0002", "ours", "c1"),
                           PRE + e("D-0003", "theirs", "c1"))
    check("unit: an entry BOTH sides dropped stays gone (the retraction shape) and the rest "
          "unions", kind == "ok" and "D-0001" not in got and "D-0002" in got and "D-0003" in got,
          str(extra))
    # both sides minted the same id: identical bytes or nothing.
    mint = e("D-0002", "both minted this", "c1")
    kind, got, extra = uni("DECISIONS.md", PRE, PRE + mint + e("D-0003", "ours", "c1"),
                           PRE + mint + e("D-0004", "theirs", "c1"))
    check("unit: one id both sides created with identical bytes accepts exactly one copy",
          kind == "ok" and got.count("## D-0002") == 1, str(extra))
    kind, got, extra = uni("DECISIONS.md", PRE,
                           PRE + e("D-0002", "ours minted", "c1"),
                           PRE + e("D-0002", "theirs minted", "c1"))
    check("unit: one id both sides created with different words refuses -- two decisions, one "
          "name", kind == "refuse" and got == "id-collision", str(extra))
    # the one invariant of the merged file: strictly ascending. Ours appended 0004, theirs
    # appended 0003 -- the correct tail seats ids, not append order.
    kind, got, extra = uni("DECISIONS.md", PRE + u1, PRE + u1 + e("D-0004", "ours", "c1"),
                           PRE + u1 + e("D-0003", "theirs", "c1"))
    check("unit: concurrent appends seat by id, not by which branch appended first",
          kind == "ok" and [i for i in ("D-0001", "D-0003", "D-0004") if "## " + i in got]
          == ["D-0001", "D-0003", "D-0004"]
          and got.index("D-0003") < got.index("D-0004"), str(extra))
    # a side that reordered its history is an incident, not a merge input.
    kind, got, extra = uni("DECISIONS.md", PRE,
                           PRE + e("D-0003", "written first", "c1") + e("D-0002", "written second", "c1"),
                           PRE + e("D-0004", "theirs", "c1"))
    check("unit: descending ids on a side refuse with not-ascending (never sorted)",
          kind == "refuse" and got == "not-ascending", str(extra))
    kind, got, extra = uni("DECISIONS.md", PRE, PRE + e("D-0002", "once", "c1")
                           + e("D-0002", "twice", "c1"), PRE + e("D-0003", "theirs", "c1"))
    check("unit: a side repeating an id refuses with duplicate-id",
          kind == "refuse" and got == "duplicate-id", str(extra))
    kind, got, extra = uni("DECISIONS.md", PRE,
                           PRE + e("H-0001", "harness entry in the product log", "c1"),
                           PRE + e("D-0003", "theirs", "c1"))
    check("unit: an H- id filed into DECISIONS.md refuses with wrong-series",
          kind == "refuse" and got == "wrong-series", str(extra))
    # preambles: keyed by nothing, so a divergent pair is nobody's to choose.
    kind, got, extra = uni("DECISIONS.md", PRE + u1,
                           "# Product decisions, restyled by ours\n\n" + u1,
                           "# Product decisions, restyled by theirs\n\n" + u1)
    check("unit: both sides rewriting the preamble refuses -- no key, no sort, no claim wins",
          kind == "refuse" and got == "preamble-edit", str(extra))
    kind, got, extra = uni("DECISIONS.md", PRE + u1,
                           "# Product decisions, restyled by ours\n\n" + u1, PRE + u1)
    check("unit: a one-sided preamble edit accepts (same rule the entries follow)",
          kind == "ok" and got.startswith("# Product decisions, restyled by ours"), str(extra))

    # ---- repoint legs: H-0289, against a real mini-tree --------------------------------------
    tree2 = tempfile.mkdtemp(prefix="merge-decisions-repoint-")
    with open(os.path.join(tree2, "tool.py"), "w", encoding="utf-8") as f:
        f.write("unrelated first\nalpha here uniquely\nthird line\n")
    paths2, prob2 = gate.tree_files(tree2)
    index2 = gate.tree_index(paths2)
    read2 = read_planned({}, tree2)

    def rep(text):
        st = {"ours": 0, "theirs": 0, "drift": 0, "repointed": 0}
        try:
            new_text, hard = repoint("DECISIONS.md", text, read2, index2, st)
        except Refuse as refuse:
            return "refuse", refuse.reason, st
        # A non-empty `hard` is what plan() turns into a refusal; the legs read the rows.
        return ("hard", " || ".join(hard), st) if hard else ("ok", new_text, st)

    cite = ("- Why: the tool handles it; see `tool.py:%d` \"alpha here uniquely\" "
            "for the case.\n")
    ent = ("## D-0009 · repoint fixture\n\n- Question: does drift get re-aimed?\n"
           "- Options: re-aim | refuse\n- Chose: re-aim\n")
    kind, got, st = rep(PRE + ent + cite % 1 + "- Checkpoint: c1\n\n")
    check("unit: a citation whose anchor is UNIQUELY on another line is re-aimed there",
          kind == "ok" and "`tool.py:2`" in got and st["repointed"] == 1, str(got)[:120])
    kind, got, st = rep(PRE + ent + cite % 2 + "- Checkpoint: c1\n\n")
    check("unit: a citation already true in the merged tree is left alone, silently",
          kind == "ok" and "`tool.py:2`" in got and st["repointed"] == 0, str(got)[:120])
    with open(os.path.join(tree2, "tool.py"), "w", encoding="utf-8") as f:
        f.write("unrelated first\nalpha same repeated\nsomething else entirely "
                "here\nalpha same repeated\n")
    paths2, _ = gate.tree_files(tree2)
    index2 = gate.tree_index(paths2)
    read2 = read_planned({}, tree2)
    kind, got, st = rep(PRE + ent + ("- Why: see `tool.py:1` \"alpha same repeated\" for the "
                                     "case.\n- Checkpoint: c1\n\n"))
    check("unit: an anchor on TWO lines refuses rather than guess -- the wrong pointer is a "
          "wrong claim", kind == "hard" and "tool.py:1" in got and st["repointed"] == 0
          and "sits on 2 line(s)" in got, str(got)[:160])
    kind, got, st = rep(PRE + ent + ("- Why: see `gone-file.md:4` \"gone nowhere at all\" for the "
                                     "case.\n- Checkpoint: c1\n\n"))
    check("unit: a citation into a file the merged tree does not hold refuses rather than "
          "re-number", kind == "hard" and "gone-file.md:4" in got and st["repointed"] == 0
          and "cannot open" in got, str(got)[:160])
    shutil.rmtree(tree2, ignore_errors=True)

    # ---- git legs: only the shapes measured (above) as reliably conflicted --------------------
    work = tempfile.mkdtemp(prefix="merge-decisions-selftest-")
    repo = os.path.join(work, "repo")
    os.makedirs(os.path.join(repo, "scripts"))
    shutil.copy(os.path.join(HERE, "check-decisions.py"), os.path.join(repo, "scripts/"))
    shutil.copy(os.path.join(HERE, "merge-decisions.py"), os.path.join(repo, "scripts/"))
    _sh("git", "init", "-q", "-b", "main", repo)
    _sh("git", "-C", repo, "config", "user.email", "fixture@invalid")
    _sh("git", "-C", repo, "config", "user.name", "fixture")
    os.makedirs(os.path.join(repo, "factory"))
    # tool.py is the clean-file fixup's target: H-0001's citation anchors on its FIRST line,
    # so a leg that shifts tool.py on one side leaves the CLEAN-merged harness log holding a
    # row the repoint pass must re-aim -- the shape whose rollback the #460 review caught
    # leaking. The anchor must stay uniquely on line 1 for every other leg's arbiter run.
    with open(os.path.join(repo, "tool.py"), "w", encoding="utf-8") as f:
        f.write("alpha here uniquely\nsecond line\nthird line\n")
    with open(os.path.join(repo, "notes.md"), "w", encoding="utf-8") as f:
        f.write("filler one\nfiller two\nfiller three\n")
    _sh("git", "-C", repo, "add", "-A")
    _sh("git", "-C", repo, "commit", "-qm", "fixture: scripts and a file to collide")
    checkpoint = subprocess.check_output(["git", "-C", repo, "rev-parse", "HEAD"],
                                         encoding="utf-8", text=True).strip()
    # The shipped gate refuses a log with no entry in it ("a log that has stopped being
    # written"), so the arbiter legs need the harness log to hold one -- and every entry's
    # Checkpoint must name a commit that already exists when the entry's own commit lands,
    # so the logs arrive one commit AFTER the sha they cite, and that first commit is root.
    with open(os.path.join(repo, "DECISIONS.md"), "w", encoding="utf-8") as f:
        f.write("# Product decisions\n\n")
    # H-0001's Why cites tool.py's first line with its anchor: true in every leg's tree
    # except the clean-fixup leg, where it is the drifted row the plan must re-aim (and,
    # after a refusal, must have put back).
    h0001 = ("## H-0001 · fixture harness entry\n\n- Question: what does H-0001 decide?\n"
             "- Options: this | that | neither\n- Chose: this\n"
             "- Why: the tool handles it; see `tool.py:1` \"alpha here uniquely\" for the "
             "case.\n- Checkpoint: %s\n\n" % checkpoint)
    with open(os.path.join(repo, "factory", "DECISIONS.md"), "w", encoding="utf-8") as f:
        f.write("# Harness decisions\n\n" + h0001)
    _sh("git", "-C", repo, "add", "-A")
    _sh("git", "-C", repo, "commit", "-qm", "fixture: two seeded logs")
    root = subprocess.check_output(["git", "-C", repo, "rev-parse", "HEAD"], encoding="utf-8", text=True).strip()

    def merge_leg(name, ours_edit, theirs_edit):
        """Each leg is its own three-way world off the fixture's ROOT commit (main never
        moves, or every later leg inherits the last leg's entries). -> (ours tree, merge rc)."""
        base = os.path.join(work, name + ".base")
        trunk = "trunk-" + name
        _sh("git", "-C", repo, "worktree", "add", "-q", "-b", trunk, base, root)
        with open(os.path.join(base, "DECISIONS.md"), "a", encoding="utf-8") as f:
            f.write(_entry("D-0001", "the base entry", checkpoint))
        _sh("git", "-C", base, "add", "-A")
        _sh("git", "-C", base, "commit", "-qm", name + ": base entry")
        base_sha = subprocess.check_output(["git", "-C", base, "rev-parse", "HEAD"],
                                           encoding="utf-8", text=True).strip()
        ours = os.path.join(work, name + ".ours")
        _sh("git", "-C", repo, "worktree", "add", "-q", "-b", "ours-" + name, ours, base_sha)
        ours_edit(ours)
        _sh("git", "-C", ours, "add", "-A")
        _sh("git", "-C", ours, "commit", "-qm", name + ": ours")
        theirs_edit(base)
        _sh("git", "-C", base, "add", "-A")
        _sh("git", "-C", base, "commit", "-qm", name + ": theirs")
        rc = _sh_rc("git", "-C", ours, "merge", "--no-edit", trunk)
        return ours, rc

    def log(path):
        with open(os.path.join(path, "DECISIONS.md"), encoding="utf-8") as f:
            return f.read()

    def appends(text):
        def edit(path):
            with open(os.path.join(path, "DECISIONS.md"), "a", encoding="utf-8") as f:
                f.write(text)
        return edit

    def union(ours, plan_only=False):
        proc = subprocess.run([sys.executable, os.path.join(HERE, "merge-decisions.py"),
                               "--tree", ours] + (["--plan-only"] if plan_only else []),
                              capture_output=True, encoding="utf-8", text=True)
        return proc.returncode, (proc.stdout + proc.stderr).strip()

    def conflicted(path):
        try:
            return "<<<<<<<" in log(path)
        except OSError:
            return False

    def same_line(words):
        """Both sides replacing the SAME base line: the micro-probe's shape C, reliably
        conflicted -- used when a leg needs git's hands on a published entry."""
        def edit(path):
            body = log(path)  # read BEFORE opening for write; the reverse truncates to empty
            with open(os.path.join(path, "DECISIONS.md"), "w", encoding="utf-8") as f:
                f.write(body.replace("- Why: because the fixture says so, in one clause",
                                     "- Why: " + words))
        return edit

    try:
        # G1 -- the tail-seam append, the ticket's own shape, end to end through the CLI:
        #     control (git conflicted), accept, fields, staged-but-not-committed, and the
        #     shipped gate green over the merged tree (the arbiter ran and said nothing).
        tree, rc = merge_leg("appends", appends(_entry("D-0002", "ours appended", checkpoint)),
                             appends(_entry("D-0003", "theirs appended", checkpoint)))
        check("git: both-side appends really conflict at the tail seam (control)", rc != 0)
        rcu, out = union(tree)
        body = log(tree)
        unmerged_left = subprocess.run(["git", "-C", tree, "ls-files", "-u"],
                                       capture_output=True, encoding="utf-8", text=True).stdout.strip()
        check("git: both appends accepted, both ids present in the merged log",
              rcu == 0 and "## D-0002" in body and "## D-0003" in body, out)
        check("git: the accept line carries the fields the ticket names",
              all(k in out for k in ("entries=", "ours=", "theirs=", "drift=", "repointed=",
                                     "files=")), out)
        check("git: staged but NOT committed -- the merge stays open for the caller to commit",
              not unmerged_left and
              _sh_rc("git", "-C", tree, "rev-parse", "-q", "--verify", "MERGE_HEAD") == 0)
        # the arbiter's own voice, straight: the gate this script certifies with says the
        # merged tree is clean (without it, G1 would pass with a red gate behind the UNION line)
        arb = subprocess.run([sys.executable, os.path.join(tree, "scripts", "check-decisions.py")],
                             cwd=tree, capture_output=True, encoding="utf-8", text=True)
        check("git: the shipped gate says the merged tree is clean, on its own account",
              arb.returncode == 0, (arb.stdout + arb.stderr)[:220])

        # G2 -- same-line collision through the CLI: refusal AND the conflicted bytes intact.
        tree, rc = merge_leg("collide", same_line("ours rewrote the Why on purpose"),
                             same_line("theirs rewrote the Why differently"))
        check("git: the same-line shape really conflicted (control)", rc != 0)
        rcu, out = union(tree)
        check("git: same-id edits differing by WORDS come back as id-collision through the CLI",
              rcu == 1 and "reason=id-collision" in out, out)
        check("git: refusal leaves the conflicted bytes exactly where the model will find them",
              conflicted(tree))

        # G3 -- atomicity: a non-log conflict refuses the whole file set, logs stay conflicted.
        tree, rc = merge_leg("other",
                             lambda p: (appends(_entry("D-0002", "ours", checkpoint))(p),
                                        _write(os.path.join(p, "notes.md"), "ours notes\n")),
                             lambda p: (appends(_entry("D-0003", "theirs", checkpoint))(p),
                                        _write(os.path.join(p, "notes.md"), "theirs notes\n")))
        rcu, out = union(tree)
        check("git: a non-log conflict refuses the union atomically, naming the file",
              rcu == 1 and "reason=other-conflict" in out and "notes.md" in out, out)
        check("git: and the logs stay conflicted too -- no half resolution was staged",
              conflicted(tree))

        # G4 -- --plan-only: the same verdict, zero bytes written; the run then makes it true.
        tree, rc = merge_leg("dry", appends(_entry("D-0002", "ours", checkpoint)),
                             appends(_entry("D-0003", "theirs", checkpoint)))
        rc0, out0 = union(tree, plan_only=True)
        check("git: --plan-only reports the accept line and writes nothing",
              rc0 == 0 and out0.startswith("UNION ") and conflicted(tree), out0)
        rc1, out1 = union(tree)
        check("git: --plan-only and the real run agree", rc1 == 0 and not conflicted(tree)
              and out0.split("repointed=")[0] == out1.split("repointed=")[0],
              "%s | %s" % (out0, out1))

        # G5 -- arbiter-red: a well-formed union whose tree the shipped gate refuses (a
        #       missing Why) refuses with the gate's sentence, and regenerates the conflict.
        tree, rc = merge_leg("red",
                             appends("## D-0002 · no Why field\n\n- Question: why?\n"
                                     "- Options: a | b\n- Chose: a\n- Checkpoint: %s\n\n"
                                     % checkpoint),
                             appends(_entry("D-0003", "fine", checkpoint)))
        rcu, out = union(tree)
        check("git: a merged tree the shipped gate refuses comes back as arbiter-red",
              rcu == 1 and "reason=arbiter-red" in out, out)
        check("git: the refusal regenerated the conflict -- the model gets the merge it was "
              "promised", conflicted(tree))

        # G8 -- the clean-file fixup and the rollback it used to skip (the #460 review): the
        # plan re-aims the CLEAN harness log's drifted row (tool.py shifted on ours only, the
        # harness log merged clean), the shipped gate refuses the merged whole (the Why-less
        # entry), and exit 1 must mean nothing written for the CLEAN log too -- worktree AND
        # index back to the merge's own stage-0 bytes. Without the restore_point rollback the
        # re-aim stays written and STAGED (`checkout -m` on a stage-less path is a no-op), so
        # a person who hand-finishes the merge commits an uncertified pointer rewrite.
        tree, rc = merge_leg("cleanfix",
                             lambda p: (appends("## D-0002 · no Why field\n\n- Question: why?\n"
                                                "- Options: a | b\n- Chose: a\n- Checkpoint: %s\n\n"
                                                % checkpoint)(p),
                                        _write(os.path.join(p, "tool.py"),
                                               "header ours\nalpha here uniquely\nsecond line\n"
                                               "third line\n")),
                             appends(_entry("D-0003", "theirs appended", checkpoint)))

        def harness_log():
            with open(os.path.join(tree, "factory", "DECISIONS.md"), encoding="utf-8") as f:
                return f.read()

        def harness_stage():
            got = subprocess.run(["git", "-C", tree, "ls-files", "--stage", "--",
                                  "factory/DECISIONS.md"], capture_output=True,
                                 encoding="utf-8", text=True).stdout
            return [ln.split()[1] for ln in got.splitlines()
                    if ln.split("\t", 1)[-1] == "factory/DECISIONS.md"]

        check("git: the fixture clean-merged a harness log holding the drifted row, stage 0, "
              "anchor moved to line 2 (precondition -- without this the leg proves nothing)",
              rc != 0 and "<<<<<<<" not in harness_log() and "`tool.py:1`" in harness_log()
              and len(harness_stage()) == 1
              and harness_log().count("`tool.py:1`") == 1
              and open(os.path.join(tree, "tool.py"), encoding="utf-8").read()
              .splitlines()[1] == "alpha here uniquely")
        pre_sha = harness_stage()[0]
        rcu, out = union(tree)
        post_stage = harness_stage()
        index_body = ""
        if post_stage:
            index_body = subprocess.run(["git", "-C", tree, "cat-file", "blob", post_stage[0]],
                                        capture_output=True, encoding="utf-8",
                                        text=True).stdout
        check("git: a clean fixup plus a refused arbiter comes back as arbiter-red",
              rcu == 1 and "reason=arbiter-red" in out, out)
        check("git: the refusal put the CLEAN fixed-up log back in the worktree -- the "
              "re-aimed row is gone from the text",
              "`tool.py:2`" not in harness_log() and "`tool.py:1`" in harness_log())
        check("git: and in the INDEX -- the staged blob is the merge's own stage-0 entry "
              "again, not the re-aim (the #460 leak was staged, not just written)",
              post_stage == [pre_sha] and "`tool.py:2`" not in index_body,
              "post_stage=%s pre=%s" % (post_stage, pre_sha))
        check("git: and the conflicted log still regenerates beside it -- the model gets "
              "both files exactly as the failed merge left them", conflicted(tree))
        rcu2, out2 = union(tree)
        check("git: the refused tree replans to the same refusal -- the rollback restored "
              "every byte this run had touched, fixup included",
              rcu2 == 1 and "reason=arbiter-red" in out2, out2)

        # G6 -- a log deleted on one side: git's modify/delete is not a union.
        def rm_log(path):
            os.remove(os.path.join(path, "DECISIONS.md"))
        tree, rc = merge_leg("dellog", appends(_entry("D-0002", "ours", checkpoint)), rm_log)
        rcu, out = union(tree)
        check("git: a log deleted on one side refuses with side-deleted",
              rcu == 1 and "reason=side-deleted" in out, out)

        # G7 -- exit-code vocabulary the supervisor switches on.
        clean = os.path.join(work, "clean")
        _sh("git", "-C", repo, "worktree", "add", "-q", "-b", "clean-branch", clean, root)
        proc = subprocess.run([sys.executable, os.path.join(HERE, "merge-decisions.py"),
                               "--tree", clean], capture_output=True, encoding="utf-8", text=True)
        check("git: no merge in progress is exit 2 with reason=not-merging -- never exit 1, "
              "which the supervisor would journal as a content verdict",
              proc.returncode == 2 and "reason=not-merging" in proc.stdout, proc.stdout)

        tree, rc = merge_leg("nolog",
                             lambda p: _write(os.path.join(p, "notes.md"), "ours\n"),
                             lambda p: _write(os.path.join(p, "notes.md"), "theirs\n"))
        rcu, out = union(tree)
        check("git: a conflict holding neither log: exit 1, reason=no-log-conflict",
              rcu == 1 and "reason=no-log-conflict" in out, out)
    finally:
        subprocess.run(["git", "-C", repo, "worktree", "prune"], capture_output=True)
        shutil.rmtree(work, ignore_errors=True)
    print("merge-decisions --self-test: %d ok, %d failed" % (state["ok"], state["bad"]))
    return 1 if state["bad"] else 0


def _write(path, text):
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
