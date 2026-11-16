# NEXT — issue-334 unit

State at push: branch `issue-334`, checkpoint `990ec86`; the effect commit carries `Decision: H-0215`
— the second base-sync union for PR #401 (the first is `H-0205`; the feature itself is `H-0187` and the
escalation is `H-0188`). Everything below is this unit's hand-off plus every owed line the `issue-337`
hand-off carried that this unit re-read as still true on the merged tree. `make verify`'s verdict is
quoted in the pull-request body from its read-back, never from here (#1180).

## What this unit shipped

- `factory_log_footprint` in `factory/common.sh` and its one call site in `factory/factory.sh` `--status`:
  the journal line now also reports bytes, file count and the oldest journal under `$FACTORY_LOG`, from
  one `find -maxdepth 1 -type f`, cross-checked byte-for-byte against `du -sb`. It reads no journal
  content and deletes, rotates, truncates and prunes nothing.
- `factory/selftest.sh` legs for the footprint, the wiring, and the three adversarial rounds the pull
  request lists (a comment-call must not count as wiring; a nested log dir must not be summed; the
  journal-less arm needs its own plant).
- `H-0188`, an escalation, not a fix: the two ways to bound `factory/log` both delete the operator's
  audit trail, which is a reserved class, so the entry carries `Chose: escalated` and the menu is on the
  issue. Nothing here ships a deletion.
- Two base-sync merges for PR #401, recorded as `H-0205` and `H-0215`: the conflict is the decision-log
  double-append, never code. `H-0215` is the second, and it is a double double-append — `main` published
  `H-0207`/`H-0208` past this branch's `H-0205`, so there are two seams and neither side is a whole tail.
  A third can land before the merge — `main`'s log moves under every open branch while #381 is unshipped.

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
   `testing/capture/cdp.py` names a per-run capture manifest under the fixture runs directory — measured,
   and all four are true sentences about a path that is refused or generated, which is why the leg
   declines them. Widening the leg means naming why each of those four is not a phantom, and an
   instruction in a page outside the three is still unchecked.
4. **A dropped owed line is still uncaught.** The contract says carry the previous unit's items forward;
   no check can tell a carried line from a deleted one, because it has no memory of the last hand-off.
   The measured instance is `#296`: `git log --all -S '#296' -- NEXT.md` answers the warning going in at
   `89c2298` and out at `af848f4`, a unit overwriting the file, not a merge resolution. So this survives
   as a rule, not a gate.
5. **539 decision-log pointers still carry no anchor, and a merge moves them invisibly.** Measured here
   rather than feared: this base sync moved 62 numbers in entries both sides share — `main` re-pointed
   them, and the union takes `main`'s lines — and `scripts/check-decisions.py` cannot read one of them,
   because an unanchored citation has no expected text. The instrument that did answer in seconds, and
   that nothing ships: for every unanchored pointer resolving into a file a merge touched, compare the
   line at the pointed number against each parent; this sync was 217 of 217 identical to `main`, which is
   how it proved it authored zero drift. That census is a gate leg waiting for someone to write it, and
   it needs no API.
6. **A tick's pre-flight fetch is unbudgeted.** `git fetch --prune origin` in this tree hit the harness's
   120 s ceiling on the first attempt and returned rc 0 when re-run with a 600 s bound. #419 already owns
   the class (one hung pre-flight fetch ate a whole 5400 s pass with zero journal lines); the unowned half
   is that the allocator's own `REFRESH` advice tells a unit to fetch and nothing tells it to bound the
   fetch or say which fetch is hanging.
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
8. **This push spends the approval, and only a read-back says so.** `H-0215` records that the push
   dismisses the `APPROVED` read back at `b046c44` (measured on #269: a base-sync push dismisses reviews
   without touching any exit status). Ask `gh pr view 401 --json reviewDecision,reviews,mergeable,mergeStateStatus`
   after the push and before calling anything ready; the re-ask is the supervisor's poke, not a unit's.

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
  numstat against each parent blob, the added-lines census, the marker count, the leg census, and one
  wrong order actually run so the refusal is measured and not asserted. Restore mutants with `cp` from a
  backup, never `git checkout`.
- Mutants and probes run on `git archive` copies under `/tmp/opencode`, never the gated tree (#1807).
  `rm -rf` is refused by the harness here, so use fresh unique directories.
- Overwrite this file, carrying every still-true line you did not write. Deleting another unit's owed
  item because your branch did not write it is the failure this file exists to end.
