#!/usr/bin/env python3
# shellcheck shell=bash ignore=0
"""split-decisions.py -- one batch of #488 phase 3: file a slice of a decision monolith out of it.

WHAT THIS IS. `DECISIONS.md` and `factory/DECISIONS.md` are append-only logs that reached 5,651 and
19,253 lines, and the split (#429, H-0263) says an entry is one file. Phase 1 (#442) taught the gate
to read both layouts; phase 2 (#456) taught the allocator to emit filed ids; phase 3 moves the 437
entries still inside the two blobs. This script does ONE batch: it takes the oldest entries (up to
`--count N`, or everything through `--through <id>`) out of a monolith, writes each one to
`<dir>/<id>.md` byte for byte, rewrites the monolith from the preamble plus what remains, and re-aims
every `path:line` citation in the union that pointed into the log whose rows it just moved.

WHY THE CITATIONS TRAVEL WITH THE ENTRIES. A pointer in these logs is a COORDINATE, and the gate
refuses one whose anchor is not on the row it names. Deleting an entry from the middle of a log moves
every row below it, so a batch that only moved files would silently mis-aim every pointer under the
cut -- including pointers aimed at entries that stayed. So the transform is one row map, applied to
the whole union in the same commit as the move:

    old row -> (new file, new row)

  * a row inside a moved entry becomes that entry's own file, at `row - heading + 1`;
  * a row that stays keeps the monolith and loses the lines removed above it;
  * the preamble is above every cut, so its rows do not move.

Every citation aimed at the log is rewritten from that map, continuations (`:12339`) included -- and a
continuation has to have its path WRITTEN OUT once its target can live in a different file, because a
bare `:NNN` inherits its file from its own sentence and that sentence still names the monolith.

WHY A MOVED ENTRY CANNOT ROT A POINTER QUIETLY. Anchored citations verify themselves: the rewrite
keeps the quoted anchor byte-identical and the gate re-checks it against the row the coordinate now
names, so wrong arithmetic in the map is a RED GATE, not a drift. Unanchored citations (the
`POINTER_DEBT` class) are the ones nothing checks; they ride the same map, and that is the argument
recorded in `factory/DECISIONS/H-0383.md` -- the map is one function over rows, so the anchored
pointers that re-verify are the evidence for the ~40 that cannot.

WHAT IT REFUSES, each as one machine-readable line: a target that is not one of the two monoliths
(`not-a-monolith`), an entry file that already exists (`already-filed` -- phase 3 never overwrites a
filed entry, that is the duplicate #442 counts), a row that is in no entry and below the first heading
(`gap-rows`, `overlapping-blocks`: a shape this map cannot answer for), a citation whose cited row is
past the end of the log it names (`row-past-eof`, counted and left alone rather than given a number
nobody read), a range whose ends land in two files (`range-split` -- that pointer wants an author, not
arithmetic), and a scanner whose citation rows no longer line up with the text (`scanner-diverged`),
the same assertion `merge-decisions.py` makes before it renumbers anyone's pointer.

The gate is IMPORTED for the entry model and the citation scanner -- one home, the same argument as
`merge-decisions.py`'s docstring -- and this script never commits and never pushes.

USAGE
    scripts/split-decisions.py --root DIR --log factory/DECISIONS.md
                               [--count 40 | --through H-0040] [--dry-run]
    scripts/split-decisions.py --self-test
"""

import argparse
import importlib.util
import os
import re
import shutil
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))

# The gate, by path beside this file, not by module name: `check-decisions` is not importable (the
# hyphen is load-bearing for the CLI), and `dont_write_bytecode` keeps this from leaving a stray .pyc
# in the tree whose logs it is moving -- both reasons are `merge-decisions.py`'s, lines 80-92.
_spec = importlib.util.spec_from_file_location("check_decisions_gate",
                                               os.path.join(HERE, "check-decisions.py"))
sys.dont_write_bytecode = True
if _spec is None or _spec.loader is None:
    print("SPLIT-REFUSED reason=no-gate %s" % os.path.join(HERE, "check-decisions.py"))
    sys.exit(2)
gate = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(gate)


class Refuse(Exception):
    """One refusal: the fixed reason token, then the detail the caller prints."""

    def __init__(self, reason, detail):
        super().__init__("%s %s" % (reason, detail))
        self.reason = reason
        self.detail = detail


def read_at(root, rel):
    with open(os.path.join(root, rel.replace("/", os.sep)), encoding="utf-8") as fh:
        return fh.read()


def blocks_of(text, rel):
    """A monolith as `(row count, [entry blocks])`, every row accounted for.

    A block runs from its heading to the line before the next heading, and the last one to the end of
    the file. `gate.parse` is the only reader of what a heading is: a line the gate will not accept as
    a heading is not a block here either, and the row-coverage checks below refuse the batch rather
    than move text nobody agreed is an entry.
    """
    entries = gate.parse(text, rel)
    if not entries:
        raise Refuse("no-entries", "%s carries no entry heading -- there is nothing to file" % rel)
    lines = text.splitlines()
    heads = [(e["line"], e["id"]) for e in entries]
    out = []
    for i, (row, ident) in enumerate(heads):
        end = heads[i + 1][0] - 1 if i + 1 < len(heads) else len(lines)
        out.append({"id": ident, "start": row, "end": end, "file": None})
    covered = set(range(1, heads[0][0]))
    for b in out:
        span = set(range(b["start"], b["end"] + 1))
        if covered & span:
            raise Refuse("overlapping-blocks", "%s: rows %s sit in two entry blocks"
                         % (rel, sorted(covered & span)[:5]))
        covered |= span
    gaps = sorted(set(range(1, len(lines) + 1)) - covered)
    if gaps:
        raise Refuse("gap-rows", "%s: rows %s are in no entry and below the first heading, so no row "
                                 "map can answer for them" % (rel, gaps[:8]))
    return len(lines), out


def row_map(total_lines, entry_blocks, first_head, chosen):
    """`old row -> (new file or None, new row)` for one monolith, with `chosen` filed out of it."""
    mapping = {r: (None, r) for r in range(1, total_lines + 1)}
    shift = 0
    for b in entry_blocks:
        rows = range(b["start"], b["end"] + 1)
        if b["id"] in chosen:
            for r in rows:
                mapping[r] = (b["file"], r - b["start"] + 1)
            shift += b["end"] - b["start"] + 1
        else:
            for r in rows:
                mapping[r] = (None, r - shift)
    return mapping


def spans_of(text, found):
    """Per citation `(path span, digits span)`, aligned row for row with the gate's scanner.

    `scan_citations` answers dicts with no offsets and a rewrite needs them, so the scanner's own skip
    rule is replayed over `gate.CITATION` matches and every row is asserted to agree on
    (written, low, high) -- the contract `merge-decisions.py`'s `citation_spans` states at its lines
    445-470. If the gate ever grows a filter this replay does not carry, this returns None and nothing
    is rewritten; it never renumbers a pointer the scanner did not mean.
    """
    out = []
    it = gate.CITATION.finditer(text)
    for c in found:
        m = next(it, None)
        while m is not None and "\n" in m.group(0):
            m = next(it, None)
        if m is None:
            return None
        parts = m.group(2).replace(" ", "").split("-")
        if (m.group(1) != c["written"] or int(parts[0]) != c["low"] or int(parts[-1]) != c["high"]):
            # A continuation is written bare and resolved upstream, so only its digits can be compared.
            if not c["continued"] or (int(parts[0]) != c["low"] or int(parts[-1]) != c["high"]):
                return None
        out.append((m.span(1), m.span(2)))
    return out


def rewrite_citations(text, rel, index, target, mapping, total_lines):
    """Re-aim every citation in `text` aimed at `target`. -> (new text, count, notes)."""
    entries = gate.parse(text, rel)
    found = gate.scan_citations(text, entries, index)
    spans = spans_of(text, found)
    if spans is None:
        raise Refuse("scanner-diverged", "%s: the gate's citation rows no longer line up with this "
                                         "text -- the split and the gate disagree about what a "
                                         "pointer is, so nothing is rewritten" % rel)
    edits, notes = [], []
    for c, (psp, dsp) in zip(found, spans):
        if c["kind"] in ("not-a-path", "revision-qualified"):
            continue
        if c["path"] != target:
            continue
        low, high = c["low"], c["high"]
        if low > total_lines or high > total_lines:
            notes.append("row-past-eof %s:%d (in %s, cited from %s:%s) names a row past the log's %d "
                         "lines -- nothing judged it before and nothing can map it now, so it is left "
                         "as written and counted"
                         % (c["written"] or ":", low, c["entry"], rel, c["line"], total_lines))
            continue
        lo, hi = mapping[low], mapping[high]
        if lo[0] != hi[0] or (high - low) != (hi[1] - lo[1]):
            raise Refuse("range-split", "%s:%d-%d (in %s, cited from %s:%s) spans rows that land in "
                                        "%s and %s -- one pointer cannot aim at two files, so its "
                                        "author re-aims it"
                         % (c["written"] or ":", low, high, c["entry"], rel, c["line"],
                            lo[0] or target, hi[0] or target))
        digits = str(lo[1]) if lo[1] == hi[1] else "%d-%d" % (lo[1], hi[1])
        replacement = "%s:%s" % (lo[0] or target, digits)
        start = psp[0] if psp[1] > psp[0] else dsp[0] - 1
        if text[dsp[0] - 1] != ":":
            raise Refuse("scanner-diverged", "%s: no colon before the digits at row %d"
                         % (rel, c["line"]))
        edits.append((start, dsp[1], replacement))
    for start, stop, replacement in reversed(edits):
        text = text[:start] + replacement + text[stop:]
    return text, len(edits), notes


def split(root, log_rel, chosen_ids, dry_run=False):
    """File `chosen_ids` out of `log_rel`, re-aiming the union. -> the report the caller prints."""
    if log_rel not in gate.LOG_REL:
        raise Refuse("not-a-monolith", "%s is not one of %s; a filed log has nothing to split"
                     % (log_rel, " and ".join(gate.LOG_REL)))
    stem = log_rel[:-len(".md")]
    if stem not in gate.ENTRY_DIRS:
        raise Refuse("not-a-monolith", "%s would file into %s, which is not one of %s"
                     % (log_rel, stem, " or ".join(gate.ENTRY_DIRS)))
    original = read_at(root, log_rel)
    lines = original.splitlines(keepends=True)
    total, entry_blocks = blocks_of(original, log_rel)
    ids = [b["id"] for b in entry_blocks]
    missing = [i for i in chosen_ids if i not in ids]
    if missing:
        raise Refuse("unknown-id", "%s: %s is not an entry in this log" % (log_rel, ", ".join(missing)))
    for b in entry_blocks:
        b["file"] = "%s/%s.md" % (stem, b["id"])
    chosen = set(chosen_ids)
    clash = [b["file"] for b in entry_blocks
             if b["id"] in chosen and os.path.exists(os.path.join(root, b["file"].replace("/", os.sep)))]
    if clash:
        raise Refuse("already-filed", "%s already exists -- the split never overwrites a filed entry; "
                                      "that is the duplicate #442 counts, not a batch to re-run"
                     % ", ".join(clash[:4]))

    # Every block's bytes, taken once; the reconstruction below proves the partition is exact before a
    # single file is written, because a split that lost a line of prose is worse than no split.
    for b in entry_blocks:
        b["body"] = "".join(lines[b["start"] - 1:b["end"]])
    head = entry_blocks[0]["start"]
    preamble = "".join(lines[:head - 1])
    if preamble + "".join(b["body"] for b in entry_blocks) != original:
        raise Refuse("byte-loss", "%s: preamble plus the entry blocks does not reproduce the log it "
                                  "read, so the batch would move a lossy split" % log_rel)

    mapping = row_map(total, entry_blocks, head, chosen)
    new_monolith = preamble + "".join(b["body"] for b in entry_blocks if b["id"] not in chosen)
    written = [(b["file"], b["body"] if b["body"].endswith("\n") else b["body"] + "\n")
               for b in entry_blocks if b["id"] in chosen]

    report = {"log": log_rel, "moved": [b["id"] for b in entry_blocks if b["id"] in chosen],
              "before_lines": total, "after_lines": len(new_monolith.splitlines()),
              "rewritten": [], "notes": [], "planned_files": [f for f, _ in written]}

    # The union is re-aimed against the tree as it will be: the new monolith and the new entry files
    # are the files a reader will open, so they go into the index (and the read) before anything else.
    planned = {log_rel: new_monolith}
    planned.update(dict(written))
    index_paths = set(gate.tree_files(root)[0] or []) | set(planned)
    index = gate.tree_index(sorted(index_paths))

    def body_at(path, rel):
        if rel in planned:
            return planned[rel]
        try:
            with open(path, encoding="utf-8") as fh:
                return fh.read()
        except (OSError, UnicodeDecodeError):
            return None

    # The union, as the reader of the next commit will see it: `log_paths` walks the DIRECTORY, so the
    # files this batch is about to create are not in it yet -- and a moved entry's own body is exactly
    # where the pointers that need re-aiming live. Hence the planned files are walked as well.
    walk = list(gate.log_paths(root)[0])
    seen = {os.path.relpath(p, root).replace(os.sep, "/") for p in walk}
    for rel in planned:
        if rel not in seen:
            walk.append(os.path.join(root, rel.replace("/", os.sep)))
    for path in walk:
        rel = os.path.relpath(path, root).replace(os.sep, "/")
        text = body_at(path, rel)
        if text is None:
            continue
        new_text, count, notes = rewrite_citations(text, rel, index, log_rel, mapping, total)
        report["notes"] += notes
        if count:
            report["rewritten"].append([rel, count])
            planned[rel] = new_text

    if not dry_run:
        # `planned` holds exactly the files this batch changes -- the rewritten monolith, the new entry
        # files (whose own pointers were re-aimed by the pass above), and any other log that carried a
        # coordinate into the log that moved. Nothing else on disk is touched.
        for rel, body in sorted(planned.items()):
            path = os.path.join(root, rel.replace("/", os.sep))
            parent = os.path.dirname(path)
            if parent and not os.path.isdir(parent):
                os.makedirs(parent, exist_ok=True)
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(body)
    return report


def select(entry_ids, count, through):
    """Which ids this batch takes: the oldest `count`, or everything through `through`."""
    if count is not None and through:
        raise Refuse("bad-args", "--count and --through are two ways to say one thing")
    if through:
        if not re.fullmatch(r"[DH]-\d{4}", through):
            raise Refuse("bad-args", "--through wants an id like H-0040, got %r" % through)
        if through not in entry_ids:
            raise Refuse("unknown-id", "%s is not an entry in this log, so the batch would stop "
                                       "somewhere a reader cannot check" % through)
        return entry_ids[:entry_ids.index(through) + 1]
    return entry_ids[:count if count is not None else 40]


# ---------------------------------------------------------------------------------------------
# self test: a fixture repository, and every citation shape that has to come out the other side
# ---------------------------------------------------------------------------------------------

# The fixture is written with `@…@` where a coordinate goes, because a hand-counted row number in a
# fixture is the same mistake this whole script exists to stop: the assertion would be about the
# fixture's arithmetic, not the tool's. `_fixture` counts the rows and fills them in.
FIXTURE_PRE = ("# Decision log\n\n"
               "Preamble citing a moved row: `factory/DECISIONS.md:@c1@` \"- Chose: one\".\n\n")
FIXTURE_E1 = ("## H-0001 · the first one\n\n"
              "- Chose: one\n- Decision: H-0001\n\n"
              "  It cites itself at `factory/DECISIONS.md:@c1@` \"- Chose: one\", and the next entry at\n"
              "  `factory/DECISIONS.md:@c2@` \"- Chose: two\", and again bare at `:@d2@` \"- Decision: H-0002\".\n\n")
FIXTURE_E2 = ("## H-0002 · the second one\n\n"
              "- Chose: two\n- Decision: H-0002\n\n"
              "  A range `factory/DECISIONS.md:@c1@-@d1@` over the entry before it, and a pinned\n"
              "  `3c09d6b:factory/DECISIONS.md:@c1@` that states its own tree.\n\n")
FIXTURE_E3 = ("## H-0003 · the third one\n\n"
              "- Chose: three\n- Decision: H-0003\n\n"
              "  It stays, and still means the preamble: `factory/DECISIONS.md:3` \"Preamble citing\".\n\n")
FIXTURE_D1 = ("# Decision log\n\n"
              "## D-0001 · product side\n\n- Chose: yes\n- Decision: D-0001\n\n"
              "  Points across the logs at `factory/DECISIONS.md:@c2@` \"- Chose: two\".\n\n")


def _write(root, rel, text):
    d = os.path.dirname(os.path.join(root, rel.replace("/", os.sep)))
    if d:
        os.makedirs(d, exist_ok=True)
    with open(os.path.join(root, rel.replace("/", os.sep)), "w", encoding="utf-8") as fh:
        fh.write(text)


def fixture_text():
    """The harness log with its coordinates filled in from the rows the anchors are really on."""
    raw = FIXTURE_PRE + FIXTURE_E1 + FIXTURE_E2 + FIXTURE_E3
    rows = {}
    for key, anchor in (("c1", "- Chose: one"), ("d1", "- Decision: H-0001"),
                        ("c2", "- Chose: two"), ("d2", "- Decision: H-0002")):
        # a prefix match, because every anchor here is ALSO quoted as an anchor somewhere above it: a
        # substring search would find the quotation and aim the fixture's arithmetic at row 3
        rows[key] = next(i + 1 for i, row in enumerate(raw.splitlines()) if row.startswith(anchor))
    for key, row in rows.items():
        raw = raw.replace("@%s@" % key, str(row))
    return raw, rows


def self_test():
    """Every shape a pointer in the real logs takes, and where it has to land.

    The fixture is the smallest tree holding all of them at once: a pointer into a moved entry from the
    preamble, from a moved sibling, from a staying entry and from the OTHER log; a bare continuation
    whose file changes underneath it; a range spanning two rows of one moved entry; a revision-qualified
    pointer that must NOT move; a pointer to the preamble, which is above every cut; and two refusals
    that must fire -- a filed entry that already exists, and a range whose ends land in two files.
    """
    fails = []
    checks = [0]

    def expect(cond, msg):
        checks[0] += 1
        if not cond:
            fails.append(msg)

    root = tempfile.mkdtemp(prefix="split-selftest-")
    try:
        raw, rows = fixture_text()
        _write(root, "factory/DECISIONS.md", raw)
        _write(root, "DECISIONS.md", FIXTURE_D1.replace("@c2@", str(rows["c2"])))
        rep = split(root, "factory/DECISIONS.md", ["H-0001", "H-0002"])
        expect(rep["moved"] == ["H-0001", "H-0002"], "the batch moved %s" % rep["moved"])

        f1 = read_at(root, "factory/DECISIONS/H-0001.md")
        f2 = read_at(root, "factory/DECISIONS/H-0002.md")
        left = read_at(root, "factory/DECISIONS.md")
        prod = read_at(root, "DECISIONS.md")
        # where each moved row landed: `row - heading + 1`, the map's whole arithmetic
        h1 = next(i + 1 for i, r in enumerate(raw.splitlines()) if r.startswith("## H-0001"))
        h2 = next(i + 1 for i, r in enumerate(raw.splitlines()) if r.startswith("## H-0002"))
        c1, d1 = rows["c1"] - h1 + 1, rows["d1"] - h1 + 1
        c2, d2 = rows["c2"] - h2 + 1, rows["d2"] - h2 + 1

        # a filed entry is its own block, heading first, and none of its neighbour
        expect(f1.startswith("## H-0001 · the first one"), "H-0001.md does not start at its heading")
        expect("## H-0002" not in f1 and "## H-0003" not in f1, "H-0001.md swallowed a neighbour")
        expect(f2.startswith("## H-0002 · the second one"), "H-0002.md does not start at its heading")
        # the monolith kept its preamble and the entry that stays, and lost the two it moved
        expect(left.startswith("# Decision log\n\nPreamble citing a moved row"),
               "the monolith lost its preamble: %r" % left[:60])
        expect("## H-0001" not in left and "## H-0002" not in left, "the monolith kept a moved entry")
        expect("## H-0003" in left, "the monolith lost the entry that stays")
        # every pointer into a moved entry now names that entry's file at the row inside it
        expect("`factory/DECISIONS/H-0001.md:%d` \"- Chose: one\"" % c1 in left.splitlines()[2],
               "a preamble pointer into a moved entry was not re-aimed: %r" % left.splitlines()[2])
        expect("`factory/DECISIONS/H-0001.md:%d` \"- Chose: one\"" % c1 in f1,
               "H-0001's self-citation was not re-aimed at its own file: %r" % f1)
        expect("`factory/DECISIONS/H-0002.md:%d` \"- Chose: two\"" % c2 in f1,
               "H-0001's pointer into H-0002 was not re-aimed at that file: %r" % f1)
        expect("`factory/DECISIONS/H-0002.md:%d` \"- Decision: H-0002\"" % d2 in f1,
               "the bare continuation was not expanded into a written path: %r" % f1)
        expect("`factory/DECISIONS/H-0001.md:%d-%d`" % (c1, d1) in f2,
               "a range over two rows of one moved entry was not re-aimed as a range: %r" % f2)
        expect("`factory/DECISIONS/H-0002.md:%d` \"- Chose: two\"" % c2 in prod,
               "a pointer from the other log did not follow the entry: %r" % prod)
        # a revision-qualified pointer states its own tree, so the split leaves it alone
        expect("`3c09d6b:factory/DECISIONS.md:%d`" % rows["c1"] in f2,
               "a revision-qualified citation was rewritten: %r" % f2)
        # a pointer to the preamble is untouched: the preamble is above every cut
        expect("`factory/DECISIONS.md:3` \"Preamble citing\"" in left,
               "a pointer to the preamble drifted: the preamble is above every cut")
        # and no bare monolith coordinate survives inside a moved entry -- the only row that may still
        # name the monolith is the pinned one, and it opens with a sha, not with a backtick
        expect("`factory/DECISIONS.md:" not in f1 and "`:" not in f1,
               "a stale coordinate survives inside a moved entry: %r" % f1)

        # refusal 1: an entry the monolith still carries while its file already exists
        _write(root, "factory/DECISIONS/H-0003.md", "## H-0003 · the third one\n")
        try:
            split(root, "factory/DECISIONS.md", ["H-0003"])
            fails.append("filing an entry whose file exists did not refuse")
        except Refuse as r:
            checks[0] += 1
            expect(r.reason == "already-filed", "that refused with reason=%s" % r.reason)
        os.remove(os.path.join(root, "factory", "DECISIONS", "H-0003.md"))
        # refusal 2: a range whose two ends land in different files after the map. Rows are counted
        # here, in the ten lines that say what they are: row 3 is the preamble, row 8 is inside the
        # entry being filed, so `3-8` is one pointer aimed at two files.
        seg = ["# Decision log", "", "Preamble line.", "",
               "## H-0004 · spans the cut", "", "- Chose: four", "- Decision: H-0004", "",
               None, ""]
        seg[seg.index(None)] = "  Straddles: `factory/DECISIONS.md:3-8` \"- Decision: H-0004\"."
        _write(root, "factory/DECISIONS.md", "\n".join(seg) + "\n" + FIXTURE_E3)
        try:
            split(root, "factory/DECISIONS.md", ["H-0004"])
            fails.append("a range whose ends split across two files did not refuse")
        except Refuse as r:
            checks[0] += 1
            expect(r.reason == "range-split", "that refused with reason=%s" % r.reason)
    finally:
        shutil.rmtree(root, ignore_errors=True)
    return fails, checks[0]


def main(argv):
    ap = argparse.ArgumentParser(prog="split-decisions.py",
                                 description="File one batch of a decision monolith into per-entry files.")
    ap.add_argument("--root", default=os.getcwd(), help="the tree to split (default: cwd)")
    ap.add_argument("--log", choices=list(gate.LOG_REL))
    ap.add_argument("--count", type=int, help="how many of the oldest entries this batch takes")
    ap.add_argument("--through", help="file everything up to and including this id")
    ap.add_argument("--dry-run", action="store_true", help="report the batch, write nothing")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args(argv)
    if a.self_test:
        fails, n = self_test()
        for f in fails:
            print("FAIL", f)
        print("self-test: %d assertion(s), %d failure(s)" % (n, len(fails)))
        return 1 if fails else 0
    if not a.log:
        ap.error("--log is one of " + ", ".join(gate.LOG_REL))
    root = os.path.abspath(a.root)
    try:
        chosen = select([e["id"] for e in gate.parse(read_at(root, a.log), a.log)], a.count, a.through)
        rep = split(root, a.log, chosen, dry_run=a.dry_run)
    except Refuse as r:
        print("SPLIT-REFUSED reason=%s %s" % (r.reason, r.detail))
        return 1
    print("%s %s: %d entries filed out (%d lines -> %d), range %s..%s"
          % ("DRY" if a.dry_run else "SPLIT", rep["log"], len(rep["moved"]),
             rep["before_lines"], rep["after_lines"], rep["moved"][0], rep["moved"][-1]))
    for rel, cnt in rep["rewritten"]:
        print("  re-aimed %-40s %d citation(s)" % (rel, cnt))
    for note in rep["notes"]:
        print("  NOTE %s" % note)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
