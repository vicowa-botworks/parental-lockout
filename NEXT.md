# NEXT — issue-334 unit

State at push: branch `issue-334`, third base-sync merge `ff771d5` carrying `Decision: H-0217`
(checkpoint `ed1954e`) — the third union for PR #401 (the first two are `H-0205` and `H-0215`; the
feature itself is `H-0187`, the escalation is `H-0188`). Everything below is this unit's hand-off plus
every owed line the earlier `issue-334` hand-offs carried that this unit re-read as still true on the
merged tree. `make verify`'s verdict is quoted in the pull-request body from its read-back, never from
here (#1180).

## What this unit shipped

- The third base sync for PR #401, recorded as `H-0217`: two whole-entry seams (ours' `H-0205` vs
  main's `H-0206` mid-file; ours' `H-0215` vs main's `H-0209`+`H-0213` at the tail), spliced as the
  ascending-id union, proved additive against both parent blobs, zero markers. One digit repair, in
  this branch's own `H-0215`: its `selftest.sh` coordinate slid +1395 (main's exact net to that file)
  from 12193 to 13588; null and aim mutants both named the entry at rc 1 and were restored by `cp`.
  Splice-authored pointer drift over all 368 resolved citations: zero — this branch's 173-line
  `factory/selftest.sh` insertion lands at line 16394, below every cited coordinate.
- The feature and the escalation are unchanged from the first sync: `factory_log_footprint` in
  `factory/common.sh` with its one call site in `factory/factory.sh --status` — bytes, file count and
  oldest journal under `$FACTORY_LOG`, cross-checked against `du -sb`; it reads no journal content and
  deletes, rotates, truncates and prunes nothing. `H-0188` is an escalation, not a fix: both ways to
  bound `factory/log` delete the operator's audit trail, a reserved class, so the menu is on the issue
  and nothing here ships a deletion.
- A fourth sync can land before this merge — `main`'s log moves under every open branch while #381 is
  unshipped; that is item 5's census and #381's price, re-paid by this tick as by its two predecessors.

## What is still owed (all listed, none silent)

1. **`H-0188` is owed a human answer, and it is the only thing that bounds `factory/log`.** The measured
   trajectory is in the pull-request body (20 MB / 100 files on 12 Nov, 36 MB / 172 at these bytes).
   Rotation and truncation are both deletions of the operator's records, so no unit may pick one; read
   the escalation comment on #334 before touching `factory/common.sh`'s footprint function again.
2. **Nothing here reads whether a hand-off is TRUE.** `scripts/check-next-md.sh` reads the tree and never
   the API, so a well-formed hand-off about work `main` finished last week still passes and still costs a
   merge to notice. A leg that asks GitHub needs a token that CI does not have — `scripts/check-denies.py`
   is the precedent for skipping loudly, which is the honest shape for that leg.
3. **The phantom-path leg reads three files, not every file.** `docs/factory.md`, `factory/selftest.sh`
   and `factory/next-decision-id.sh` each name the harness decision log with its trailing `s` missing, and
   `testing/capture/cdp.py` names a per-run capture manifest under the fixture runs directory — measured
   by the authoring unit, and all four are true sentences about a path that is refused or generated,
   which is why the leg declines them. Widening the leg means naming why each of those four is not a
   phantom, and an instruction in a page outside the three is still unchecked.
4. **A dropped owed line is still uncaught.** The contract says carry the previous unit's items forward;
   no check can tell a carried line from a deleted one, because it has no memory of the last hand-off.
   The measured instance is `#296`: `git log --all -S '#296' -- NEXT.md` answers the warning going in at
   `89c2298` and out at `af848f4`, a unit overwriting the file, not a merge resolution. So this survives
   as a rule, not a gate.
5. **539 decision-log pointers still carry no anchor, and a merge moves them invisibly.** Measured here
   for the third sync running rather than feared: the union takes `main`'s re-pointings of shared rows
   (63 this window), and `scripts/check-decisions.py` cannot read one unanchored citation, because an
   unanchored citation has no expected text. The instrument that answered in seconds again, and that
   nothing ships: for every citation resolving into a file, compare the line at the pointed number
   against each parent; this sync was zero drift against `main` on all 368 resolved rows except the
   pre-existing dead `README.md:588`/`:725`/`:1005` family — dead on the base too, a 349-line README —
   which is how it proved it authored none. That census is a gate leg waiting for someone to write it,
   and it needs no API.
6. **A tick's pre-flight fetch is unbudgeted.** `git fetch --prune origin` in this tree again exceeded a
   120 s ceiling (re-measured 16 Nov as part of the same chained call that also ran `gh pr view`; it
   returned rc 0 when re-run alone with 300 s). #419 already owns the class (one hung pre-flight fetch
   ate a whole 5400 s pass with zero journal lines); the unowned half is that the allocator's own
   `REFRESH` advice tells a unit to fetch and nothing tells it to bound the fetch or say which fetch is
   hanging.
7. **Carried forward from the `issue-325` unit** (its merge is `main`'s head; this unit re-measured none
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
8. **This push spends the approval, and only a read-back says so.** `H-0217` records that the push
   dismisses the `APPROVED` read back at `ca8f541` (measured on #269: a base-sync push dismisses reviews
   without touching any exit status). Ask `gh pr view 401 --json reviewDecision,reviews,mergeable,mergeStateStatus`
   after the push and before calling anything ready; the re-ask is the supervisor's poke, not a unit's.
9. **The checkpoint commit message for this sync says "eight fresh entries" where the merge measured
   six** (`H-0183`, `H-0184`, `H-0185`, `H-0206`, `H-0209`, `H-0213` — the number `H-0217` carries and
   the numstat proves). `ed1954e` is an empty checkpoint and cannot be re-worded without a rewrite,
   which is denied here; the sentence lives in the pull-request body instead, so a reader of the log
   message learns which number the evidence supports.

## Non-blocking review items carried forward (PR #402's review at 2026-11-15T21:57:01Z on head `5408b41`; the reviewer marked all three NOT merge blockers — fix on a later head, they will spend the approval again)

1. **A bare `-` bullet slips the empty-marker leg.** `scripts/check-next-md.sh` requires a space or tab
   after the marker, so a lone `-` line matches neither branch, and the fixture plants `- ` *with* the
   space, so the suite cannot see the class. Measured by the reviewer: rc 0 for a shipped bare-dash
   bullet. Repair: widen the pattern and add a fixture.
2. **The scanned-file list is unpinned.** The leg reads the hand-off, the skill and `AGENTS.md`, but every
   phantom-path fixture plants only in the hand-off — a future PR could narrow the list with the suite
   green. Repair: one fixture that plants the phantom in `AGENTS.md`.
3. **The shape leg early-returns after its first refusal**, so a hand-off with two shape defects reports
   one per run. Accepted as a first cut; recorded so the next unit does not mistake the behavior for a bug.

## Hard rules for whoever picks this up

- Checkpoint before the decision, one effect commit with a `Decision:` trailer, and
  `factory/next-decision-id.sh factory/DECISIONS.md` re-run after a fresh `git fetch` before taking an id.
- Before editing a file either log cites, run `python3 scripts/check-decisions.py --citations` with that
  path and fix what it lists in the same commit. Every `path:line` you write needs its quoted anchor on
  the same physical line or `POINTER_DEBT` rises and the gate refuses.
- A decision-log union is keyed by id and mechanical: splice the ascending union, then prove it — the
  numstat against each parent blob, the added-lines census, the marker count, the additive line census
  on every auto-merged code file, one wrong order actually run so the refusal is measured and not
  asserted, and a parent-relative landing census of every citation for the drift the gate cannot read.
  Restore mutants with `cp` from a backup, never `git checkout` (#2215).
- Mutants and probes run on `git archive` copies under `/tmp/opencode`, never the gated tree (#1807).
  `rm -rf` is refused by the harness here, so use fresh unique directories.
- `Chose:` must quote an option verbatim or `check-decisions.py` refuses the entry (measured 16 Nov:
  the paraphrased `Chose:` line cost one gate run).
- Overwrite this file, carrying every still-true line you did not write. Deleting another unit's owed
  item because your branch did not write it is the failure this file exists to end.
