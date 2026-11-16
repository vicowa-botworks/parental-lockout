# NEXT — issue-340 unit

State at push: branch `issue-340`, checkpoint `496dfb3`; the merge commit carries `Decision: H-0218`
-- the second base-sync union on this branch, fifth of this shape (`H-0190`, `H-0207`, `H-0208`,
`H-0214`, now `H-0218`) and the first of them with TWO seams.
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
- The `H-0218` base-sync merge of `origin/main`, the second on this branch: still one conflicted file,
  but TWO seams -- `H-0192` seats inside `main`'s fresh run above `H-0195`, and `H-0214` seats inside it
  between `H-0213` and `H-0216`, because a fresh entry goes at its ascending position and not at the end.
  The merge moved four coordinates `main`'s own freshest entry had published, each by exactly 72 lines,
  and all four were re-pointed from the merged tree; both whole-tail orders were run and both were
  refused. Shape, refusals, the +72 proof and the heading's blank-line census are in `H-0218`.
- `make verify` on the merged tree, twice: the first run ends `714 passed, 2 failed` and the second
  `715 passed, 1 failed`, and `make` exits 2 on both. The standing failure is `reap_leg_live`, and it is
  NOT this merge -- the leg fails on `origin/main` in a clean worktree on this box too, control in
  `H-0218`'s `Effect:` and in owed item 8 below. Both narrower gates are
  green on these bytes: `python3 scripts/check-decisions.py` and `bash scripts/check-next-md.sh`, rc 0.
  The leg the first run also lost, `unit_call_sites`, is measured four ways in `H-0218` rather than
  declared a flake, and it stays open as owed item 9 below.
- The push of both onto the existing PR #404, and the read-back of `mergeable`, `mergeStateStatus`,
  `reviewDecision` and `headRefOid` after it. The push spends the APPROVED review the branch carried --
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
   than trusting git's silence on auto-merged files. The third pass on this branch, `H-0218`, adds two
   more sweeps to that set: test every pointer that points INTO a decision log against the merged tree
   before committing, because a sync can move coordinates the other side published only hours earlier, and
   census the blank line above every `## H-` heading, because an interleaved splice leaves one heading
   glued to the entry above it and no shipped instrument reads blank lines at all.
 7. **Three stale coordinates in `H-0169`'s row, found by this branch's own sweep and left alone on
   purpose.** They send a reader to factory/factory.sh lines 1112 to 1114 for "the stamping loop" and
   1241 to 1247 for "the ranking loop"; on today's tree the stamping loop is at line 1489 and the ranking
   read at line 1632. All six of that entry's pointers into that file are bare, so no gate reads them,
   they were already off in both parents, and this merge moved none of them -- `H-0218` names why no
   numbers were written for them inside a merge commit. Owed to #374 and #381.
  8. **`reap_leg_live` refuses a hold that is legitimately zero.** Its one substantive pairing is
     `deadline` implies a hold above 0 (in `factory/selftest.sh` at `:17062`, "the live journal produced a
     deadline hold of 0s"), but a `oneshot` killed at its deadline writes `Main process exited`, `Failed
     with result 'timeout'` and `Failed to start` in the same instant, so the hold it measures IS zero.
     Measured 16 Nov on this box: `factory_reap_read` over the live journal answers `0  deadline  2531  1`
     at rc 0, and this leg is the only failure of the 716-leg suite on `origin/main` as well as on this
     branch -- the merge is not the cause. `H-0124` withheld `reaped in 0s` for a CLEAN reaping with no
     start point at all; a deadline kill whose lines share an instant is another shape, and the leg pins
     the two as one. Repair: allow `deadline` with hold 0 when the work line and the terminal line belong
     to the same activation, and keep the refusal where it bites, on `clean`.
  9. **A leg that could not read its file reports a missing clause.** `status_arm` -- the suite's own
     reader of the `--status)` arm, `factory/selftest.sh` at `:11908` -- runs the file through `awk` and
     sends its stderr to `/dev/null`, and no caller reads that read's exit status, so `unit_call_sites`
     refuses with "--status has no matched arm" for any failure of the read and not only for an absent
     arm. The comment above the function (lines 11905 to 11907) already names the class -- "a scan that
     cannot see the thing it claims to check" -- and this is that defect on the read rather than the scan.
     Control measured 16 Nov: a stub `awk` exiting 1 on `PATH` reproduces that refusal byte for byte.
     The leg is then measured in four trees the same day, and the CI verdict is not among them: green on
     `origin/main` (`bash factory/selftest.sh` -> `714 passed, 1 failed`, only `reap_leg_live` red), green
     in two of the three merged-tree runs, red in the first merged-tree `make verify`, and red at
     `499e0ae` -- the head this sync was taken against -- with a DIFFERENT message, "has no unreadable
     arm", because `status_arm` extracts zero lines from that revision's `factory/factory.sh` (682 passed,
     2 failed there). The API reports the checks on `499e0ae` green, so read the CI job's own log before
     concluding anything: either the leg skips on the runner or the runner does not run `make verify`.
     The merged-tree loss still has no cause -- the legs either side of it, which read the same arm the
     same way, passed in that run, the journal holds no OOM or kill record for the window, and a second
     suite was running in the same tree and finished two minutes after `make` did. Repair: check the read's
     status and print "the arm could not be read" when it fails, so a transient never reads as a code
     defect, and settle the CI question with the job log rather than with this box.

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
