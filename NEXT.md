# NEXT — conflict:405 unit (PR #405, branch `issue-341`)

State at push: branch `issue-341`, checkpoint `d3d8782` (`chore(checkpoint): pre-H-0220`, empty);
the merge commit carries `Decision: H-0220` — the second base sync on this branch and the third of
this shape after `H-0210`'s lineage. Everything below is the `issue-340` hand-off (`412ec73`) carried
forward, with its two live coordinates re-measured on this merged tree, plus this unit's own
shipped/owed sections. `make verify`'s verdict is quoted in the pull-request body from its read-back,
never from here (#1180).

## What this unit shipped

- The `H-0220` base-sync merge of `origin/main` (`0b5e5da`, the `#404` merge): `factory/DECISIONS.md`
  conflicted with git's alignment scattering our two fresh entries across `main`'s run, and
  `factory/selftest.sh` conflicted in TWO fixture-wiring hunks (not coordinate drift — both sides
  extended the same stub-runner env list). The log resolved as the ascending union with `H-0193`
  seated inside `main`'s run and `H-0210` immediately below `H-0209`; both refused whole-orders were
  run and refused (`6` and `11` gate problems, exit 1). The two selftest hunks each resolved as the
  word union of one env list (`PROC_TICK_FAILS` + `PROC_DIRTY` + `PROC_TICK_ALLOC`), keeping both
  arms' semantics. Thirteen drifted coordinates were re-pointed by the checker's own list — thirteen
  unambiguous, one (`selftest.sh:3278` → `3285`) chosen by opening the blob `H-0216` shipped at
  (`8802950^`, where that line is the `proc_mutate` hit-line, not the `proc_mutate_factory` twin at
  `6044`) — and the ratchet stayed at `539` with `0` pointers a reader cannot open.
- `make verify` on these bytes, and the #425 environment disclosure: the gate is red for ANY tick
  that exports its own `FACTORY_TICK_UNIT` on a tree carrying `#407`'s journalling fixture, which
  this merge now does. The full run that ships these bytes ran with `FACTORY_TICK_UNIT` and
  `FACTORY_TICK_BRANCH` unset (the CI shape, green by #425's own table) and the verdict there is
  what the body quotes; the bare-env differential is measured in `H-0220`'s `Effect:`.

## What this unit did NOT ship

- Nothing was merged: merges belong to the supervisor. Nothing was approved or re-requested: reviewer
  asks belong to the supervisor's poke (#2027).
- `#425` is NOT fixed by this merge and this unit did not attempt its repair — see owed item 0.
- `#311` stays where it stood: the escalation label name still resolves to none of the repository's
  labels; `H-0192` guarantees only that the name reaches the tick from the reader, never baked.

## What is still owed (all listed, none silent)

0. **`#425` gates every CONFLICTING sync branch now, including this one.** `factory/selftest.sh`'s
   journalling bare arm (`env FACTORY_LOG="$logdir" bash "$script"`) inherits the caller's
   `FACTORY_TICK_UNIT`/`FACTORY_TICK_BRANCH`, and `factory/next-decision-id.sh` journals under
   precisely that pair, so the two legs the issue names go red for every tick that exports its own
   unit. Differential measured 16 Nov on this merged tree, same hour, same host: bare env with the full
   supervisor export set (`FACTORY_LOG`, `FACTORY_TICK_UNIT`, `FACTORY_TICK_BRANCH`) reads
   **722 passed, 2 failed**, exit 1 -- the two #425 legs verbatim; the same tree with the two tick
   vars unset reads **723 passed, 0 failed**, exit 0. Repair is `#425`'s pinned
   pin (clear the two vars inside the bare arm); do not "fix" it by weakening the legs.
1. **A control tree must contain the legs it controls.** Measured the hard way in this unit: the
   pre-merge tip `d3d8782` carries ZERO `dpn_leg` legs (`grep -c dpn_leg factory/selftest.sh` -> 0,
   and `a bare run journals nothing` absent) even though its history contains `569ec56`,
   `b7c975b` and `8802950` — ancestry does not mean content, at least one sync there did not carry
   `#407`'s fixture. A `#425` control run on that tip is vacuous green. Before controlling any
   fixture-sensitive claim, grep the control tree for the fixture by name.
2. **`reap_leg_live` refuses a hold that is legitimately zero** (carried from the `issue-340`
   hand-off; coordinate re-measured on this merged tree: the sentence is at
   `factory/selftest.sh:17070` "the live journal produced a deadline hold of 0s", was `:17062`).
   Its substance is unchanged and this unit did not re-run its measurement: a `oneshot` killed at
   its deadline writes `Main process exited`, `Failed with result 'timeout'` and `Failed to start`
   in the same instant, so the hold it measures IS zero, and the leg pins `deadline`-with-0 and
   `clean`-with-0 as one refusal. Repair: allow `deadline` with hold 0 when the work line and the
   terminal line belong to the same activation; keep the refusal on `clean`.
3. **A leg that could not read its file reports a missing clause** (carried; coordinate
   re-measured: `status_arm() {` is at `factory/selftest.sh:11915`, was `:11908`). `unit_call_sites`
   refuses with "no matched arm" for any failure of the `awk` read whose stderr is thrown away and
   whose exit no caller checks. The four-tree measurement and the stub-`awk` control are in the
   `issue-340` hand-off at `412ec73`. Repair: check the read's status and print "the arm could not
   be read"; settle the CI question from the job log, not this box.
4. **Nothing here reads whether a hand-off is TRUE** (carried verbatim in substance from
   `issue-340` item 1): `check-next-md.sh` reads the tree and never the API, so a well-formed
   hand-off about work `main` finished still passes.
5. **The phantom-path leg reads three files, not every file** (carried, `issue-340` item 2,
   re-verified this run: the gate still scans `NEXT.md`, the skill and `AGENTS.md` only).
6. **A dropped owed line is still uncaught** (carried, `issue-340` item 3 — no check tells a
   carried line from a deleted one; rule, not gate).
7. **Carried from the `issue-325` unit** (carried, `issue-340` item 4 — the deleted-call hole, the
   switched-off derivation, `#325`'s own test): re-run before acting on any row.
8. **The dispatch that invented `check-next-md.sh` was not the only one** (carried, `issue-340`
   item 5).
9. **While `#381` is unshipped, every merge re-conflicts every open branch on the decision logs**
   (carried, `issue-340` item 6 — the census shapes are listed there and were all four reused here,
   plus the pointer-into-log sweep and the heading blank-line census; the blank-line census caught
   nothing new this time because the splice script kept each `## H-` block's leading blank as one
   unit, which is now the only splice style that has never produced the glued-heading defect).
10. **Three stale coordinates in `H-0169`'s row** (carried, `issue-340` item 7): left alone on
    purpose, owed to `#374` and `#381`; this merge moved none of them.
11. **Non-blocking review items from PR #402** (carried unchanged — the bare-dash leg bullet, the
    unpinned `leg_paths` list, and the `leg_shape` early-return; all three live in
    `scripts/check-next-md.sh`, which this merge did not touch; full text at `412ec73`).

## Hard rules for whoever picks this up

- Checkpoint before the decision, one effect commit with a `Decision:` trailer, and
  `factory/next-decision-id.sh factory/DECISIONS.md` re-run after a fresh `git fetch` before taking
  an id. `H-0220` itself was re-read after this merge's fetch and confirmed free against the fresh
  union; the `H-0219` gap is the allocator's to skip, not yours to reuse.
- Run the gate with `env -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH` until `#425` is repaired, and
  SAY so where you quote the verdict — the sanitization is a disclosed workaround, not a silent one.
- Before editing a file either log cites, run `python3 scripts/check-decisions.py --citations <path>`
  and fix what it lists in the same commit. Every `path:line` you write needs its quoted anchor on
  the same physical line or `POINTER_DEBT` rises and the gate refuses.
- A `Chose:` line that is not verbatim one of the recorded options is refused; the citation for a
  moved pointer must be written only at its NEW coordinate.
- When a citation's checker message lists two candidate lines, resolve by opening the blob the
  citing entry shipped at, and cite that act in the entry — that is how `H-0220` chose `3285` over
  `6044`.
- Mutants and probes run on copies under `/tmp/opencode`, never the gated tree (#1807); restore
  measured files from the staged index (`git show :path`), not from memory.
- `rm -rf` is refused by the harness here; use fresh unique directories.
- Overwrite `NEXT.md`, carrying every still-true line you did not write, and re-measure every live
  coordinate you carry: this unit found two that had moved by 7 and 8 lines.
