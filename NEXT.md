# NEXT — issue-340 unit

State at push: branch `issue-340`, checkpoint `5b8219a`; the merge commit carries `Decision: H-0214`
-- the base-sync union for PR #404 (fourth of this shape: `H-0190`, `H-0207`, `H-0208`, now `H-0214`).
Everything below is the `issue-337` hand-off carried forward, re-checked against this merged tree where
a check was cheap, plus this unit's own shipped/owed sections. `make verify`'s verdict is quoted in the
pull-request body from its read-back, never from here (#1180).

## What this unit shipped

- The `H-0192` fix (effect commit `63b7437`, decision in `factory/DECISIONS.md`): the `address_review`
  arm's transport-failure branch no longer interpolates the escalation label's VALUE into the tick
  instruction; the suite gained the render-based discriminator (`ask_prompt_render`,
  `ask_prompt_carries_no_name`) and the re-planting control.
- The `H-0214` base-sync merge of `origin/main` into this branch: one conflicted file
  (`factory/DECISIONS.md`, one tail double-append), resolved as the interleaved ascending-id union with
  `H-0192` seated between `H-0191` and `H-0201`; one pointer re-anchored by the merge
  (`H-0192`'s `factory/factory.sh` pointer, 1031 to 1111), erratum in the merge commit.
- A push of both onto the existing PR #404. The push spends the APPROVED review the branch carried --
  read `reviewDecision` back before believing anything about it.

## What this unit did NOT ship

- Nothing was merged: merges belong to the supervisor.
- #311 stays where it stood: the escalation label name still resolves to none of the repository's
  eighteen labels; `H-0192` guarantees only that the name reaches the tick from the reader, never baked.

## What is still owed (all listed, none silent)

1. **Nothing here reads whether a hand-off is TRUE.** `check-next-md.sh` reads the tree and never the
   API (re-verified on this merged tree: zero `gh`/API reads in the file), so a well-formed hand-off
   about work `main` finished last week still passes and still costs a merge to notice. A leg that asks
   GitHub needs a token that CI does not have -- `check-denies.py` is the precedent for skipping
   loudly, which is the honest shape for that leg if someone wants it.
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
   measured instance: `git log --all -S '#296' -- NEXT.md` answers two commits -- the warning went in at
   `89c2298` and out at `af848f4`, a unit overwriting the file with its own hand-off (58 lines to 42), not
   a merge resolution as the report says. So this survives as a rule, not a gate.
4. **Carried forward from the `issue-325` unit** (its merge is `main`'s head; this unit re-measured none
   of it -- re-run before acting on any row):
   - the deleted-call hole: removing `_rule_probes_are_live(registry, ran, bad) from self_test()` is
     still green (D-0076's class). The promised closure is a tally of what the pass DID, handed to the
     verdict as a third argument; D-0081 said it did NOT ship that. Start from the class texts in
     `scripts/attack-capture-self-test.py`, not memory.
   - the switched-off derivation: deleting the PROBE-prefix trip count loses the verdict's tail on
     battery-only trips (measured, D-0081). Recorded trade -- do not relitigate without a new measurement.
   - #325 itself stays open until its own test is met on the merged tree; read the issue body against
     `gh pr view` before deciding what the branch delivers, and keep the closing-keyword adjacency rule
     of #1256 off the issue number either way.
5. **The dispatch that invented this gate was not the only one.** Nothing in the tree enumerates the
   commands a dispatch may name, so a future sentence can invent a different tool. The three-file leg is
   the cheapest available net, not a proof none exist.
6. **While #381 is unshipped, every merge re-conflicts every open branch on the decision logs.** This
   branch has now been through it twice (`63b7437`'s sync lineage and `H-0214`); the census script shapes
   used in that entry (id union against both parents, per-entry byte identity, digit-normalized line
   multisets over every merged file, leg counts) are the four that caught everything -- reuse them rather
   than trusting git's silence on auto-merged files.

## Non-blocking review items carried forward (PR #402, review at 2026-11-15T21:57:01Z on head `5408b41`; the reviewer marked all three NOT merge blockers -- fix on a later head, they will spend the approval again; all three re-verified still-true on this merged tree, whose `check-next-md.sh` is still the single-commit `07bcea8` version)

1. **A bare `-` bullet slips the empty-marker leg.** `scripts/check-next-md.sh:157` requires
   `[-*][ \t]` after the marker, so a lone `-` line matches neither branch, and the fixture at
   `:372-377` plants `- ` *with* the space, so the suite cannot see the class. Measured by the
   reviewer: rc 0 for a shipped bare-dash bullet. Repair: widen to `[-*]([ \t]|$)` and add a fixture.
2. **The scanned-file list is unpinned.** `leg_paths` at `:182` reads `NEXT.md`, the skill and
   `AGENTS.md`, but every phantom-path fixture plants only in `NEXT.md` -- a future PR could narrow the
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
- A `Chose:` line that is not verbatim one of the recorded options is refused; the citation for a moved
  pointer must be written only at its NEW coordinate -- restating the old coordinate mints a second,
  unanchored pointer and the ratchet bills it (`H-0214` tripped both).
- Mutants and probes run on `git archive` copies under `/tmp/opencode`, never the gated tree (#1807).
- `rm -rf` is refused by the harness here; use fresh unique directories.
- Overwrite `NEXT.md`, carrying every still-true line you did not write. Deleting another unit's owed
  item because your branch did not write it is the failure this file exists to end.
