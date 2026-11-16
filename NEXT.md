# NEXT — issue-337 unit

State at push: branch `issue-337`, checkpoint `ae1047e`; the effect commit carries `Decision: H-0207`
-- the second base-sync union for PR #402 (the first is `H-0190`, the hand-off gate is `H-0189`).
Everything below is the hand-off `H-0189` shipped, plus the follow-ups PR #402's 15 Nov review raised.
`make verify`'s verdict is quoted in the pull-request body from its read-back, never from here (#1180).
This file survived a third base-sync union (PR #403 into `issue-331`, `H-0208`) byte-unchanged; a union
that rewrites the hand-off destroys the carried items it exists to protect, so it is not a union.

## What this unit shipped

- `scripts/check-next-md.sh` — the gate the contract only claimed existed. Four legs: `NEXT.md`'s shape,
  a phantom-path leg over the three instruction files, the skill still naming the file and the gate, and
  both clauses present in `verify:`. Self-test: 18 cases — 17 fixtures over a copied tree (15 refusals,
  2 null variants accepted) and the witness that refuses when this file's own `make verify` clause is
  deleted (#391's hole).
- `.opencode/skills/factory/SKILL.md` — Step 6 now names `NEXT.md`, its four required sections, the run
  command, and what the gate does not cover.
- `Makefile` — `==> hand-off contract` and `==> hand-off file` inside `verify:` (clauses only: the
  comment block above `verify:` carries five `Makefile:` pointers, so it grew no prose).
- `NEXT.md` — this file, rewritten with the previous unit's owed items carried forward.
- Two base-sync merges for PR #402, recorded as `H-0190` and `H-0207`: the conflict is the decision-log
  double-append, never code, and each is resolved as one ascending-id union byte-exact on each side.
  A third can land before the merge — main's log moves under every open branch while #381 is unshipped.

## What is still owed (all listed, none silent)

1. **Nothing here reads whether a hand-off is TRUE.** `check-next-md.sh` reads the tree and never the
   API, so a well-formed hand-off about work `main` finished last week still passes and still costs a
   merge to notice. A leg that asks GitHub needs a token that CI does not have — `check-denies.py` is
   the precedent for skipping loudly, which is the honest shape for that leg if someone wants it.
2. **The phantom-path leg reads three files, not every file.** `docs/factory.md`, `factory/selftest.sh`
   and `factory/next-decision-id.sh` each name the harness decision log with its trailing `s` missing, and
   `testing/capture/cdp.py` names a per-run capture manifest under the fixture runs directory -- measured,
   and all four are true sentences about a path that is refused or generated, which is why the leg
   declines them. `NEXT.md` therefore describes them in words: this unit's own first draft spelled the
   missing-`s` log as a path and the leg refused this file for it, correctly. Widening the leg to a fourth
   file means naming why each of those four is not a phantom, and an instruction in a page outside the
   three is still unchecked.
3. **A dropped owed line is still uncaught.** The contract says carry the previous unit's items forward;
   no check can tell a carried line from a deleted one, because it has no memory of the last `NEXT.md`. The
   measured instance: `git log --all -S '#296' -- NEXT.md` answers two commits — the warning went in at
   `89c2298` and out at `af848f4`, a unit overwriting the file with its own hand-off (58 lines to 42), not
   a merge resolution as the report says. So this survives as a rule, not a gate.
4. **Carried forward from the `issue-325` unit** (its merge is `main`'s head; this unit re-measured none
   of it — re-run before acting on any row):
   - the deleted-call hole: removing `_rule_probes_are_live(registry, ran, bad) from self_test()` is
     still green (D-0076's class). The promised closure is a tally of what the pass DID, handed to the
     verdict as a third argument; D-0081 said it did NOT ship that. Start from the class texts in
     `scripts/attack-capture-self-test.py`, not memory.
   - the switched-off derivation: deleting the PROBE-prefix trip count loses the verdict's tail on
     battery-only trips (measured, D-0081). Recorded trade — do not relitigate without a new measurement.
   - #325 itself stays open until its own test is met on the merged tree; read the issue body against
     `gh pr view` before deciding what the branch delivers, and keep the closing-keyword adjacency rule
     of #1256 off the issue number either way.
5. **The dispatch that invented this gate was not the only one.** Nothing in the tree enumerates the
   commands a dispatch may name, so a future sentence can invent a different tool. The three-file leg is
   the cheapest available net, not a proof none exist.

## Non-blocking review items carried forward (PR #402, review at 2026-11-15T21:57:01Z on head `5408b41`; the reviewer marked all three NOT merge blockers — fix on a later head, they will spend the approval again)

1. **A bare `-` bullet slips the empty-marker leg.** `scripts/check-next-md.sh:157` requires
   `[-*][ \t]` after the marker, so a lone `-` line matches neither branch, and the fixture at
   `:372-377` plants `- ` *with* the space, so the suite cannot see the class. Measured by the
   reviewer: rc 0 for a shipped bare-dash bullet. Repair: widen to `[-*]([ \t]|$)` and add a fixture.
2. **The scanned-file list is unpinned.** `leg_paths` at `:182` reads `NEXT.md`, the skill and
   `AGENTS.md`, but every phantom-path fixture plants only in `NEXT.md` — a future PR could narrow the
   list with the suite green. The reviewer's probes show the list binds today. Repair: one fixture that
   plants the phantom in `AGENTS.md`.
3. **`leg_shape` early-returns after the first shape refusal** (`:119`/`:123`/`:128`/`:143`), so a
   hand-off with two shape defects reports one per run. Accepted as a first cut; recorded so the next
   unit does not mistake the behavior for a bug.

## Hard rules for whoever picks this up

- Checkpoint before the decision, one effect commit with a `Decision:` trailer, and
  `factory/next-decision-id.sh factory/DECISIONS.md` re-run after a fresh `git fetch` before taking an id.
- Before editing a file either log cites, run `python3 scripts/check-decisions.py --citations <path>` and
  fix what it lists in the same commit. Every `path:line` you write needs its quoted anchor on the same
  physical line or `POINTER_DEBT` rises and the gate refuses.
- Mutants and probes run on `git archive` copies under `/tmp/opencode`, never the gated tree (#1807).
- `rm -rf` is refused by the harness here; use fresh unique directories.
- Overwrite `NEXT.md`, carrying every still-true line you did not write. Deleting another unit's owed
  item because your branch did not write it is the failure this file exists to end.
