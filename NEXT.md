# NEXT — issue-346 unit

State at push: branch `issue-346`, first base sync for PR #408, recorded as `Decision: H-0223`
(checkpoint `2520d94`, cut before the merge started); the feature this branch delivers is `H-0197`.
Everything below is this unit's hand-off plus every owed line from `main`'s hand-off (the
`issue-334` unit's, `origin/main:NEXT.md` at `7eb9632`) that this unit re-read as still true on the
merged tree. `make verify`'s and `factory/selftest.sh`'s verdicts are quoted in the pull-request body
from the run that ships these bytes, never from here (#1180). Coordinates marked **re-measured**
were opened on this merged tree; the rest are carried in their author's words and were not re-run.

## What this unit shipped

- The base sync for PR #408, `H-0223`: three conflicted files, resolved as a union that keeps both
  parents' intents, with `origin/main` at `7eb9632` and the merge base `6db0b13`.
- `factory/factory.sh` — the one semantic conflict. `main`'s #342 ask 5 split fetch (harness tree,
  then the main checkout only when they differ) and #331's hoist of the four self-report loops into
  `factory_self_report` are kept **verbatim**, and this branch's `factory_main_tree_check` call is
  re-seated after them. `git diff origin/main -- factory/factory.sh` is exactly those three lines and
  nothing else.
- `factory/common.sh` — an EOF double-append whose shared `return 0 }` tail the diff had aligned as
  context, so a marker-only splice would leave one function unclosed. `main`'s block keeps its
  absolute position (its own log cites `factory/common.sh:4084` **inside** that block, so putting
  ours first would break a published pointer); ours is appended last and is byte-identical to
  `HEAD:factory/common.sh`'s tail. `bash -n` passes.
- `factory/DECISIONS.md` — one citation-drift hunk and one tail seam. `H-0197` seats at its ascending
  position between `H-0196` and `H-0201`; the drifted digits come off the merged tree, digit-only,
  each repaired pointer opened again afterwards.
- `factory/selftest.sh` auto-merged with this branch's 325-line fixture block landing **below** every
  coordinate `main`'s new entries publish, so none of `main`'s `selftest.sh` anchors move.
- The feature itself is unchanged from the pre-sync head: `factory_main_tree_check` and
  `factory_main_tree_inspect` in `factory/common.sh`, called once per activation in `factory_body`,
  restoring only committed-and-missing tracked files and refusing every other dirt shape untouched.

## What this unit did not deliver

- **The check still reads one tree, and #342 ask 5 made that one tree narrower than the pass's own
  reach.** `factory_self_report` and the fetch now act on `$FACTORY_HARNESS` as well as the main
  checkout, and `factory_main_tree_check` is aimed only at `factory_main_checkout`. When the two
  differ, the harness tree is fetched and compared and never read for dirt. Not delivered because
  #346 measured the primary checkout and its legs pin that shape; widening it needs its own legs.
- `H-0197`'s own *What lost* stands: the three race arms (`late-clean`, `checkout-refused`, the two
  `RESTORE-INCOMPLETE` wordings) have journalled sentences and no planted leg.
- No live `MAIN-CHECKOUT-*` line exists yet in a real activation. Every verdict this branch ships was
  produced by fixtures, so the first real activation is still the first field read.

## What is still owed (all listed, none silent)

Items 4–19 are carried from `main`'s hand-off, in that file's order, with provenance named; items
1–3 are this unit's.

1. **#416 aims at this branch's shipped function and this unit did not re-run its measurement.**
   `factory/DECISIONS.md`'s restore arm restores a tracked-and-missing file every activation, so if
   the `.cortexkit` tool deletes `.cortexkit/.gitignore` again each time, the pass rewrites it each
   time — churn, not repair. Read #416 before treating `MAIN-CHECKOUT-RESTORED` as a clean verdict.
2. **A hand-off can cite a section that no ref ever held, and no leg sees it — measured at this
   tick.** `main`'s carried item 5 (below as item 8) points at `docs/factory.md`, *What a
   conflict-resolution commit is allowed to contain*. `grep -rn 'conflict-resolution commit' docs/`
   answers nothing outside `NEXT.md` itself, `grep -nE '^#+ .*[Cc]onflict' docs/factory.md` answers
   nothing, and `git log --all -S 'allowed to contain' -- docs/factory.md` answers **empty** — the
   section never existed on any ref, so this is not a rename. Mechanism: `scripts/check-next-md.sh`
   leg 2 (`scripts/check-next-md.sh:182` "for rel in NEXT.md …") tests only whether a **path** is
   missing; a path that exists with a phantom heading inside it passes. The carried rule's substance
   is still real — an unanchored-pointer sweep belongs on a branch that is not also resolving a
   conflict — but its only in-tree support is the AGENTS.md citation discipline, not that title.
   Repair shape: either write the section or re-point the sentence; do not carry the coordinate on.
3. **A control test can be vacuous in this family and say nothing.** Both refused splice orders were
   run here only after the union passed, so a mutant built from a tree that lacks the fixture being
   controlled reads green for the wrong reason — the general form of carried item 18.
4. **`H-0188` is owed a human answer, and it is the only thing that bounds `factory/log`.** The
   measured trajectory is in PR #401's body (20 MB / 100 files on 12 Nov, 36 MB / 172 at those
   bytes). Rotation and truncation are both deletions of the operator's records, a reserved class, so
   no unit may pick one; read the escalation comment on #334 before touching
   `factory/common.sh`'s footprint function again.
5. **Nothing reads whether a hand-off is TRUE.** `scripts/check-next-md.sh` reads the tree and never
   the API, so a well-formed hand-off about work `main` finished last week still passes. A leg that
   asks GitHub needs a token CI does not have; `scripts/check-denies.py` is the precedent for
   skipping loudly. Item 2 above is this gap's sibling: the gate cannot see a sentence whose target is
   a section rather than a file.
6. **The phantom-path leg reads three files, not every file.** `docs/factory.md`,
   `factory/selftest.sh` and `factory/next-decision-id.sh` each name the harness decision log with
   its trailing `s` missing, and `testing/capture/cdp.py` names a per-run capture manifest under the
   fixture runs directory — measured by the authoring unit; the leg declines all four because each is
   a true sentence about a refused or generated path. Widening the leg means naming why each of those
   four is not a phantom. (Carried; this unit re-measured none of the four.)
7. **A dropped owed line is still uncaught.** No check can tell a carried line from a deleted one,
   because it has no memory of the last hand-off. The measured instance is `#296`:
   `git log --all -S '#296' -- NEXT.md` answers the warning going in at `89c2298` and out at
   `af848f4`, a unit overwriting the file, not a merge resolution. So this survives as a rule, not a
   gate.
8. **539 decision-log pointers still carry no anchor, and a merge moves them invisibly.** The ratchet
   guards only anchored citations, so a bare `path:line` whose line still opens somewhere is invisible
   at rc 0. `H-0217`'s instrument, run by hand over the `issue-334` merge, found **24 bare tokens on
   19 rows across ten entries** (`H-0099` 1, `H-0112` 1, `H-0137` 1, `H-0139` 5, `H-0142` 2, `H-0145`
   1, `H-0146` 3, `H-0147` 1, `H-0150` 1, `H-0169` 3) pointing into `factory/` files where main's
   insertions have since landed. None were edited, deliberately:
   - three of the 19 stamp the tree their digits were taken on and must NOT be re-pointed — two rows
     of `H-0146` write `at 131b180` beside the number, and `H-0147`'s row says in terms "all three are
     coordinates in the tree the review was filed against";
   - `H-0169`'s three rows are the debt to take first: that entry states "the coordinates here are
     the merged tree's", so its own text makes the drift a defect rather than a stamp;
   - the repair shape is `H-0148`'s — a quote taken from the named line, never a paraphrase — and it
     belongs on a `DECISIONS.md`-touching branch whose tick is not also resolving a conflict (see item
     2 for why the citation that used to sit beside this rule cannot be trusted).
   `POINTER_DEBT` on this shipped tree is **539, re-measured** (`python3 scripts/check-decisions.py`:
   284 entries, 731 citations, 539 owing an anchor, 0 pointing where a reader cannot open). The
   instrument still has no gate leg. #381 owns the price this sync is one instance of.
9. **A tick's pre-flight fetch is unbudgeted.** `git fetch --prune origin` in this tree exceeded a
   120 s ceiling twice on 15 Nov and once on 16 Nov; the `issue-334` tick did not reproduce it (rc 0
   inside 120 s), and the class stands from #419 — one hung pre-flight fetch ate a whole 5400 s pass
   with zero journal lines. The unowned half: the allocator's own `REFRESH` advice tells a unit to
   fetch and nothing tells it to bound the fetch or say which fetch is hanging.
10. **The dispatch that invented a gate was not the only one.** Nothing in the tree enumerates the
    commands a dispatch may name, so a future sentence can invent a different tool — item 2 is the
    measured instance of exactly that, one level deeper. The three-file leg is the cheapest available
    net, not a proof none exist.
11. **Carried from the `issue-325` unit** (its merge is `main`'s head; this unit re-measured none of
    it — re-run before acting on any row):
    - the deleted-call hole: removing `_rule_probes_are_live(registry, ran, bad) from self_test()` is
      still green (D-0076's class). The promised closure is a tally of what the pass DID, handed to
      the verdict as a third argument; D-0081 said it did NOT ship that. Start from the class texts in
      `scripts/attack-capture-self-test.py`, not memory.
    - the switched-off derivation: deleting the PROBE-prefix trip count loses the verdict's tail on
      battery-only trips (measured, D-0081). Recorded trade — do not relitigate without a new
      measurement.
    - #325 itself stays open until its own test is met on the merged tree; read the issue body against
      `gh pr view` before deciding what the branch delivers, and keep #1256's closing-keyword
      adjacency rule off the issue number either way.
12. **This push spends the approval, and only a read-back says so.** A base-sync push dismisses the
    `APPROVED` (measured on #269: it dismisses reviews without touching any exit status). PR #408 read
    `reviewDecision=APPROVED` with `reviewRequests: []` at head `d25976d` before this push; ask
    `gh pr view 408 --json reviewDecision,reviews,mergeable,mergeStateStatus` after the push and
    before calling anything ready. The re-ask is the supervisor's poke, not a unit's.
13. **A checkpoint commit message once said "eight fresh entries" where the merge measured six**
    (`H-0183`, `H-0184`, `H-0185`, `H-0206`, `H-0209`, `H-0213` — the number `H-0217` carries and the
    numstat proves). `ed1954e` is an empty checkpoint and cannot be re-worded without a rewrite, which
    is denied here; the sentence lives in that pull request's body.
14. **A tool read can return a file that is not on disk — verify before you overwrite it.** Measured
    on 16 Nov in `wt/issue-334`: a `read` of `NEXT.md` returned a coherent 439-line hand-off in a
    format the repository has never committed, corroborated in the same turn by a `wc -l` reading 439,
    while the real file was 120 lines and byte-identical to `HEAD:NEXT.md`. No blob on any ref has
    that shape. The invented rows are the dangerous ones: the committed hand-off carried "#419, #424
    and #427 have merged" forward as a queue fact until type tests killed it. Instruments before any
    overwrite: `git diff --quiet HEAD -- NEXT.md && wc -c NEXT.md && sha256sum NEXT.md`, and read the
    type test (`gh pr view N --json headRefName`) rather than prose. Same class as #1180, on the read
    side. (Carried; not re-run here. `NEXT.md` in this tree came from `origin/main` and this unit
    proved it with the added-line census in the pull-request body before rewriting it.)
15. **`make verify` reads RED for a tick that exports its own dispatch env, and it is `main`'s defect.**
    Run as CI runs it this merge is green; run from a tick's own shell (which exports `FACTORY_LOG`,
    `FACTORY_TICK_UNIT`, `FACTORY_TICK_BRANCH`, `FACTORY_REVIEWER`, `FACTORY_REVIEWER_TEAM`,
    `FACTORY_SERVICE_UNIT`) it dies at `Makefile:111` with `2 failed`. The two legs are `a bare run
    journals nothing` and `control: one note can suppress the other`. Pinned coordinate, **re-measured
    on this merged tree**: the bare arm is `factory/selftest.sh:15213` `if [ "$unit" = '-' ]; then`
    (the hand-off this file replaces quoted `:14933`; this branch's sync moved it +280), the
    allocator's journalled write is behind `factory/next-decision-id.sh:546` `if [ -n
    "${FACTORY_TICK_UNIT:-}" ] && [ -n "${FACTORY_LOG:-}" ]; then` (unchanged). `grep -c
    'env -u FACTORY_TICK_UNIT' factory/selftest.sh` still answers **0** — #425 is unrepaired on this
    tree. Repair shape: the fixture pins its own truth with `env -u FACTORY_TICK_UNIT
    -u FACTORY_TICK_BRANCH` on both arms, plus the missing opposite leg. Workaround until then, and
    say so wherever the verdict is quoted: `env -u FACTORY_LOG -u FACTORY_TICK_UNIT
    -u FACTORY_TICK_BRANCH make verify`. Blast radius stands: every conflict tick meets this before
    it can push, and CI cannot see it.
16. **Census numbers belong to a stage, not to a commit.** A merge message once stated the *pre-entry
    candidate's* line and heading counts as the shipped blob's; the entry was appended inside the same
    commit. State census numbers with the stage they belong to, because the next unit reads a commit
    message as the shipped artifact.
17. **Five claims one unit made about its own work did not survive re-derivation — three numbers, one
    accusation and the wrong substitute offered in its place.** The accusation — that
    `TREE-CLEAN conflict:401 … cause=never-dirty` proved `factory_tree_dirt` cannot see an
    in-progress merge — is withdrawn: `git merge` writes `ORIG_HEAD` at start and its mtime was 116 s
    *after* the probe, the journal has 0 `TICK-START` lines in that window, and the reflog puts
    `reset: moving to HEAD` one second before the read, so the verdict was right. And the substitute
    ("no probe ever runs against a conflicted tree") was also false: `factory/selftest.sh:725` and
    `:726` assert a conflicted index for `[primary]` and `[worktree]`, `mm_repo_new`/`mm_merge` build
    one mid-merge, `mm_leg 'the detector names a mid-merge tree'` pins the word and the exit code, and
    a mutant retires an arm to prove the leg is load-bearing. One root: the mutation note at
    `factory/selftest.sh:277-280` says the *unmerged arm* is redundant **for the verdict**, and
    "redundant arm" is not "untested tree". A coverage claim comes from legs and their asserted
    substrings, never from a redundancy comment; a *why-was-this-logged* claim needs the clock of the
    thing it is compared against, and `ORIG_HEAD`'s mtime is free.
18. **A control tree must contain the legs it controls.** The pre-merge tip `d3d8782` carries ZERO
    `dpn_leg` legs even though its history contains `569ec56`, so a `#425` control run there is
    vacuous green. Before controlling any fixture-sensitive claim, grep the control tree for the
    fixture by name.
19. **`reap_leg_live` refuses a hold that is legitimately zero** (carried from the `issue-340`
    hand-off through `conflict:405`'s). Coordinate **re-measured on this merged tree**:
    `factory/selftest.sh:18109` "the live journal produced a deadline hold of 0s" — the same
    coordinate `H-0220` and `H-0222` published, because this branch's selftest block lands below
    it. Substance: a `oneshot` killed at its deadline writes `Main process exited`, `Failed with
    result 'timeout'` and `Failed to start` in the same instant, so the hold it measures IS zero, and
    the leg pins `deadline`-with-0 and `clean`-with-0 as one refusal. Repair: allow `deadline` with
    hold 0 when the work line and the terminal line belong to the same activation; keep the refusal on
    `clean`.

## Non-blocking review items carried forward (PR #402's review at 2026-11-15T21:57:01Z on head `5408b41`; the reviewer marked all three NOT merge blockers — fixing them spends the approval again)

1. **A bare `-` bullet slips the empty-marker leg.** `scripts/check-next-md.sh` requires a space or
   tab after the marker, so a lone `-` line matches neither branch, and the fixture plants `- ` *with*
   the space, so the suite cannot see the class. Measured by the reviewer: rc 0 for a shipped
   bare-dash bullet. Repair: widen the pattern and add a fixture.
2. **The scanned-file list is unpinned.** The leg reads the hand-off, the skill and `AGENTS.md`, but
   every phantom-path fixture plants only in the hand-off — a future PR could narrow the list with the
   suite green. Repair: one fixture that plants the phantom in `AGENTS.md`. Item 2 of the owed list is
   the same gap aimed at a section title rather than a path.
3. **The shape leg early-returns after its first refusal**, so a hand-off with two shape defects
   reports one per run. Accepted as a first cut; recorded so the next unit does not mistake the
   behavior for a bug.

## Hard rules for whoever picks this up

- Checkpoint before the decision, one effect commit with a `Decision:` trailer, and
  `factory/next-decision-id.sh factory/DECISIONS.md` re-run after a fresh `git fetch` before taking an
  id. A `REFRESH-BLIND` answer must be re-read after a fetch; the journal shows both answers, and this
  branch's `H-0197` is on refs, so it is not a gap to reuse.
- Run the gate with `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH` until #425 is
  repaired, and SAY so where you quote the verdict — the sanitization is a disclosed workaround, not a
  silent one.
- Before editing a file either log cites, run `python3 scripts/check-decisions.py --citations <path>`
  and fix what it lists in the same commit. Every `path:line` you write needs its quoted anchor on the
  same physical line or `POINTER_DEBT` rises and the gate refuses. Two ways to fail that: a quote
  wrapped onto the next physical line counts as bare, and citing a line number from a *mutated* tree
  is a pointer to nothing — quote refused orders, never cite them.
- When a citation's checker message lists two candidate lines, resolve by opening the blob the citing
  entry shipped at, and cite that act in the entry (`H-0220`'s method).
- An `Options:` line must not contain `|`. The checker splits on it, so a union spelled with pipes is
  cut into several options and `Chose:` then refuses the entry. And `Chose:` must quote an option
  verbatim.
- A decision-log union is keyed by id and mechanical: splice the ascending union, then prove it — the
  numstat against each parent blob, the added-line census, the marker count, the additive line census
  on every auto-merged file, one wrong order actually run so the refusal is measured and not asserted,
  and a parent-relative landing census of every citation for the drift the gate cannot read. Check
  first whether the fresh ids interleave: if they do, run BOTH whole-side orders. Where both sides
  appended at EOF, check whether the diff aligned a shared closer as context before deleting the
  markers. Restore mutants with `cp` from a backup, never `git checkout` (#2215).
- Mutants and probes run on copies under `/tmp/opencode`, never the gated tree (#1807). `rm -rf` is
  refused by the harness here, so use fresh unique directories.
- Overwrite `NEXT.md`, carrying every still-true line you did not write, and re-measure every live
  coordinate you carry — this unit re-measured all five coordinates `main`'s hand-off published and
  two had moved (`factory/selftest.sh:14933` → `:15213`, +280; `factory/selftest.sh:17070` →
  `:18109`, +1039) while three still opened exactly (`factory/common.sh:214`,
  `factory/next-decision-id.sh:546`, `factory/selftest.sh:725`). Deleting another
  unit's owed item because your branch did not write it is the failure this file exists to end; the
  shipped *narrative* of another unit's hand-off is not an owed item and stays on the branch whose
  blob carries it.
