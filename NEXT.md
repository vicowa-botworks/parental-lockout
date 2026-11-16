# NEXT — issue-346 unit

State at push: branch `issue-346`, **second** base sync for PR #408, recorded as `Decision: H-0226`
(checkpoint `760e42c`, cut before the merge started); the first sync is `Decision: H-0223` (checkpoint
`2520d94`); the feature this branch delivers is `H-0197`. This merge takes `origin/main` `580d722` with
the merge base `7eb9632` — the first sync already took that commit, so main's only advance is PR #409
(`issue-347`: `H-0198`, `H-0199`, `H-0224`). Everything below is this unit's hand-off plus every owed
line from the hand-off this file replaces (the same unit's first sync) and from `main`'s hand-off
lineage (the `issue-334` unit's, `origin/main:NEXT.md` at `7eb9632`) that this unit re-read as still
true on the merged tree. `make verify`'s and `factory/selftest.sh`'s verdicts are quoted in the
pull-request body from the run that ships these bytes, never from here (#1180). Coordinates marked
**re-measured** were opened on this merged tree; the rest are carried in their authors' words and were
not re-run.

## What this unit shipped

- The second base sync for PR #408, `H-0226`: **one** conflicted file, `factory/DECISIONS.md`, in seven
  hunks — five pure coordinate drift, one mid-file seam, one tail seam — resolved as a union that keeps
  both parents, on `origin/main` `580d722` with the merge base `7eb9632`.
- Why the five drift hunks exist at all, which is the finding worth carrying: they are this branch's own
  first-sync re-anchors meeting main's re-anchors of the same sentences. A sync that repairs pointers
  *creates* the next sync's conflicts, and no check says so.
- The seam order: `H-0197` at its ascending place, main's `H-0198` and `H-0199` after it, then ours'
  `H-0222` rows, `H-0223`, main's `H-0224`, this log's new entry last. Ids unique and ascending across
  all 217 headings in this log before the new entry, 218 after.
- `H-0222`'s shared rows were decided by evidence, not preference: all four pointers inside them open on
  `HEAD`'s tree and not on main's, so ours is the newer measurement — and the sweep moved them again.
- Twenty drifted coordinates repaired on twelve rows, all same-length, every repaired row byte-identical
  with digits stripped. Thirteen point into `factory/factory.sh`, `factory/selftest.sh` and
  `factory/common.sh` and were moved where the quoted anchor has exactly one occurrence; seven are this
  log's self-citations, re-pointed by opening the cited row in the parent blob that holds the carrying
  row verbatim and finding that row again here (4 to `origin/main`, 3 to `HEAD`).
- Control run before repairing, so the cause was pinned rather than assumed: **all twenty inherited
  coordinates open on at least one parent's own tree** (4 on main's, 9 on `HEAD`'s), so the drift is
  this merge's doing and none of them was a pre-existing lie.
- Both whole-side orders run outside the gated tree and refused on measurement, not asserted: ours
  deletes 129 rows main added and loses `H-0198`, `H-0199`, `H-0224`; theirs deletes 76 rows this branch
  added and loses `H-0197`, `H-0223`; the union deletes neither.
- `NEXT.md` — rewritten by this unit (overwriting, per the contract). `main` added no line to it since
  the base, so the added-line census reports 0 lines lost in either direction.
- The feature itself is unchanged from the first sync's head: `factory_main_tree_check` and
  `factory_main_tree_inspect` in `factory/common.sh`, called once per activation in `factory_body`.
  `git diff origin/main -- factory/factory.sh` is still exactly those three added lines.

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
- This unit proved the merged parents' work for **presence**, not behaviour: `H-0223`'s fixture block and
  `H-0223`'s three-line `factory/factory.sh` delta came through the auto-merge and were re-measured in
  the added-line census only. Nothing from the first sync was re-run for behaviour, and no claim of
  main's fresh entries (`H-0198`, `H-0199`, `H-0224`) was re-derived here.

## What is still owed (all listed, none silent)

Items 1–4 are this sync's; items 5–7 are the same unit's first sync; items 8–23 are carried from
`main`'s hand-off lineage, in that file's order, with provenance named.

1. **The instrument that repairs merge-moved pointers still has no gate leg, and a second sync of one
   pull request re-pays the whole sweep by hand.** `H-0217`'s parent-relative landing census found 24
   bare tokens for the `issue-334` merge and 20 drifted anchored citations for this one. Every branch
   that touches the log and syncs twice pays it twice; #381 owns the price. What a gate leg would have
   to do is now measured, not guessed: `check-decisions.py` **does** refuse every drifted anchored
   citation — re-run in a scratch tree from the conflicted backup it refused the resolved-but-unrepaired
   log at rc 1 with exactly 20 named, and reads rc 0 on the shipped bytes — but it only *locates* them:
   7 of those 20 refusals print two, three or four candidate lines (three of them the shared phrase
   `our two entries at`, which sits on four rows of this log), so the repair must pick the row by
   identity against the parent blob. A leg that repaired them would need that parent-relative step, and
   #381 is the issue to say so.
2. **Seven `## H-` headings in `factory/DECISIONS.md` have no blank line above them, in BOTH parents, so
   they are prose to a renderer.** Measured rows on this merged tree: 2315, 3737, 4907, 6936, 8670, 8998
   and 9015 — each of them quoted heading text inside wrapped prose, present at the same numbers in
   `HEAD` and `origin/main`, and 8 in each parent (the eighth is a seam junction this merge fixed).
   `scripts/check-decisions.py` reads the raw lines and so cannot see the defect; a fix edits seven other
   units' rows and would itself cascade pointers, which is why this unit left it.
3. **`H-0225` is burned by a branch whose pull request is not in `main` yet** — `refs/heads/issue-348`
   and `refs/remotes/origin/issue-348` both hold it, which is why this sync took `H-0226`. A sibling in
   flight therefore consumes an id that `origin/main` does not show; the next unit must re-run
   `factory/next-decision-id.sh factory/DECISIONS.md` after a fresh fetch and never guess the next
   number.
4. **This unit did not re-derive any of main's fresh entries' measurements.** `H-0198`, `H-0199` and
   `H-0224` — including `H-0224`'s claim that a 126-line reseating cascade moved 46 more pointers and
   made 8 quotations ambiguous — are carried as their author's numbers. Re-measure before acting on one.
5. **#416 aims at this branch's shipped function and this unit did not re-run its measurement.**
   `factory/DECISIONS.md`'s restore arm restores a tracked-and-missing file every activation, so if
   the `.cortexkit` tool deletes `.cortexkit/.gitignore` again each time, the pass rewrites it each
   time — churn, not repair. Read #416 before treating `MAIN-CHECKOUT-RESTORED` as a clean verdict.
6. **A hand-off can cite a section that no ref ever held, and no leg sees it — re-measured on this
   merged tree.** `main`'s carried item (below as item 12) points at `docs/factory.md`,
   *What a conflict-resolution commit is allowed to contain*. `grep -rn 'conflict-resolution commit'
   docs/` answers **rc 1, no hits**, `git log --all -S 'allowed to contain' -- docs/factory.md` answers
   **0 lines** — the section never existed on any ref, so this is not a rename. Mechanism:
   `scripts/check-next-md.sh` leg 2 (`scripts/check-next-md.sh:182` "for rel in NEXT.md …", still
   182) tests only whether a **path** is missing; a path that exists with a phantom heading inside it
   passes. The carried rule's substance is still real — an unanchored-pointer sweep belongs on a branch
   that is not also resolving a conflict — but its only in-tree support is the AGENTS.md citation
   discipline, not that title. Repair shape: either write the section or re-point the sentence; do not
   carry the coordinate on.
7. **A control test can be vacuous in this family and say nothing.** Both refused splice orders were run
   here only after the union passed, so a mutant built from a tree that lacks the fixture being
   controlled reads green for the wrong reason — the general form of carried item 22.
8. **`H-0188` is owed a human answer, and it is the only thing that bounds `factory/log`.** The
   measured trajectory is in PR #401's body (20 MB / 100 files on 12 Nov, 36 MB / 172 at those
   bytes). Rotation and truncation are both deletions of the operator's records, a reserved class, so
   no unit may pick one; read the escalation comment on #334 before touching
   `factory/common.sh`'s footprint function again.
9. **Nothing reads whether a hand-off is TRUE.** `scripts/check-next-md.sh` reads the tree and never
   the API, so a well-formed hand-off about work `main` finished last week still passes. A leg that
   asks GitHub needs a token CI does not have; `scripts/check-denies.py` is the precedent for
   skipping loudly. Item 6 above is this gap's sibling: the gate cannot see a sentence whose target is
   a section rather than a file.
10. **The phantom-path leg reads three files, not every file.** `docs/factory.md`,
    `factory/selftest.sh` and `factory/next-decision-id.sh` each name the harness decision log with
    its trailing `s` missing, and `testing/capture/cdp.py` names a per-run capture manifest under the
    fixture runs directory — measured by the authoring unit; the leg declines all four because each is
    a true sentence about a refused or generated path. Widening the leg means naming why each of those
    four is not a phantom. (Carried; this unit re-measured none of the four.)
11. **A dropped owed line is still uncaught.** No check can tell a carried line from a deleted one,
    because it has no memory of the last hand-off. The measured instance is `#296`:
    `git log --all -S '#296' -- NEXT.md` answers the warning going in at `89c2298` and out at
    `af848f4`, a unit overwriting the file, not a merge resolution. So this survives as a rule, not a
    gate.
12. **539 decision-log pointers still carry no anchor, and a merge moves them invisibly.** The ratchet
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
      6 for why the citation that used to sit beside this rule cannot be trusted).
    `POINTER_DEBT` on this shipped tree is **539, re-measured** (`python3 scripts/check-decisions.py`:
    288 entries, 748 citations, 539 owing an anchor, 0 pointing where a reader cannot open). The
    instrument still has no gate leg — see item 1. #381 owns the price this sync is two instances of.
13. **A tick's pre-flight fetch is unbudgeted.** `git fetch --prune origin` in this tree exceeded a
    120 s ceiling twice on 15 Nov and once on 16 Nov; the `issue-334` tick did not reproduce it (rc 0
    inside 120 s), and the class stands from #419 — one hung pre-flight fetch ate a whole 5400 s pass
    with zero journal lines. The unowned half: the allocator's own `REFRESH` advice tells a unit to
    fetch and nothing tells it to bound the fetch or say which fetch is hanging. **One instance
    re-measured here:** this unit's own `git fetch --prune origin` after the merge took **0.572 s** at
    rc 0 — the class is intermittent, so a fast fetch is not evidence the ceiling was wrong.
14. **The dispatch that invented a gate was not the only one.** Nothing in the tree enumerates the
    commands a dispatch may name, so a future sentence can invent a different tool — item 6 is the
    measured instance of exactly that, one level deeper. The three-file leg is the cheapest available
    net, not a proof none exist.
15. **Carried from the `issue-325` unit** (that unit's merge is in `main`; this unit re-measured none of
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
16. **This push spends the approval, and only a read-back says so.** A base-sync push dismisses the
    `APPROVED` (measured on #269: it dismisses reviews without touching any exit status). PR #408 read
    `reviewDecision=APPROVED` with `reviewRequests: []` at head `0c54ab7` before this push; ask
    `gh pr view 408 --json reviewDecision,reviews,mergeable,mergeStateStatus` after the push and before
    calling anything ready. The re-ask is the supervisor's poke, not a unit's.
17. **A checkpoint commit message once said "eight fresh entries" where the merge measured six**
    (`H-0183`, `H-0184`, `H-0185`, `H-0206`, `H-0209`, `H-0213` — the number `H-0217` carries and the
    numstat proves). `ed1954e` is an empty checkpoint and cannot be re-worded without a rewrite, which
    is denied here; the sentence lives in that pull request's body.
18. **A tool read can return a file that is not on disk — verify before you overwrite it.** Measured
    on 16 Nov in `wt/issue-334`: a `read` of `NEXT.md` returned a coherent 439-line hand-off in a
    format the repository has never committed, corroborated in the same turn by a `wc -l` reading 439,
    while the real file was 120 lines and byte-identical to `HEAD:NEXT.md`. No blob on any ref has
    that shape. The invented rows are the dangerous ones: the committed hand-off carried "#419, #424
    and #427 have merged" forward as a queue fact until type tests killed it. Instruments before any
    overwrite: `git diff --quiet HEAD -- NEXT.md && wc -c NEXT.md && sha256sum NEXT.md`, and read the
    type test (`gh pr view N --json headRefName`) rather than prose. Same class as #1180, on the read
    side. (Carried; the read-side hazard itself was not reproduced here. What this unit did measure
    before overwriting: `git show HEAD:NEXT.md` is 243 lines / 20708 bytes / `sha256 989a2e4c…`, and
    `git status --short` after the merge listed five modified paths with **`NEXT.md` among neither the
    modified nor the staged** -- so the bytes this unit read were `HEAD`'s, and `main` added no line to
    the file since the base, so nothing its lineage wrote is lost by this rewrite.)
19. **`make verify` reads RED for a tick that exports its own dispatch env, and it is `main`'s defect.**
    Run as CI runs it this merge is green; run from a tick's own shell (which exports `FACTORY_LOG`,
    `FACTORY_TICK_UNIT`, `FACTORY_TICK_BRANCH`, `FACTORY_REVIEWER`, `FACTORY_REVIEWER_TEAM`,
    `FACTORY_SERVICE_UNIT` — **all six are exported in the shell that wrote this file**) it dies at
    `Makefile:111` with `2 failed` (the verify recipe's `@set -e` line, **re-measured**). The two legs
    are `a bare run journals nothing` and `control: one note can suppress the other`. Pinned
    coordinates, **re-measured on this merged tree**: the bare arm is `factory/selftest.sh:15674`
    `if [ "$unit" = '-' ]; then` (the hand-off this file replaces quoted `:15213`; main's selftest
    block landed above it, +461), the allocator's journalled write is behind
    `factory/next-decision-id.sh:546` `if [ -n "${FACTORY_TICK_UNIT:-}" ] && [ -n
    "${FACTORY_LOG:-}" ]; then` (unchanged). `grep -c 'env -u FACTORY_TICK_UNIT' factory/selftest.sh`
    still answers **0** — #425 is unrepaired on this tree. Repair shape: the fixture pins its own truth
    with `env -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH` on both arms, plus the missing opposite leg.
    Workaround until then, and say so wherever the verdict is quoted: `env -u FACTORY_LOG
    -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make verify`. Blast radius stands: every conflict tick
    meets this before it can push, and CI cannot see it.
20. **Census numbers belong to a stage, not to a commit.** A merge message once stated the *pre-entry
    candidate's* line and heading counts as the shipped blob's; the entry was appended inside the same
    commit. State census numbers with the stage they belong to, because the next unit reads a commit
    message as the shipped artifact. (`H-0226` therefore names its 217-entry candidate blob and its
    218-entry shipped blob separately, and its numstat was re-run after the entry went in.)
21. **Five claims one unit made about its own work did not survive re-derivation — three numbers, one
    accusation and the wrong substitute offered in its place.** The accusation — that
    `TREE-CLEAN conflict:401 … cause=never-dirty` proved `factory_tree_dirt` cannot see an in-progress
    merge — is withdrawn: `git merge` writes `ORIG_HEAD` at start and its mtime was 116 s *after* the
    probe, the journal has 0 `TICK-START` lines in that window, and the reflog puts `reset: moving to
    HEAD` one second before the read, so the verdict was right. And the substitute ("no probe ever runs
    against a conflicted tree") was also false: `factory/selftest.sh:725` and `:726` assert a
    conflicted index for `[primary]` and `[worktree]` (**both still open on this merged tree**),
    `mm_repo_new`/`mm_merge` build one mid-merge, `mm_leg 'the detector names a mid-merge tree'` pins
    the word and the exit code, and a mutant retires an arm to prove the leg is load-bearing. One root:
    the mutation note at `factory/selftest.sh:277-280` says the *unmerged arm* is redundant **for the
    verdict**, and "redundant arm" is not "untested tree". A coverage claim comes from legs and their
    asserted substrings, never from a redundancy comment; a *why-was-this-logged* claim needs the clock
    of the thing it is compared against, and `ORIG_HEAD`'s mtime is free.
22. **A control tree must contain the legs it controls.** The pre-merge tip `d3d8782` carries ZERO
    `dpn_leg` legs even though its history contains `569ec56`, so a `#425` control run there is
    vacuous green. Before controlling any fixture-sensitive claim, grep the control tree for the
    fixture by name.
23. **`reap_leg_live` refuses a hold that is legitimately zero** (carried from the `issue-340`
    hand-off through `conflict:405`'s). Coordinate **re-measured on this merged tree**:
    `factory/selftest.sh:18570` "the live journal produced a deadline hold of 0s" — the same coordinate
    `H-0220`, `H-0222` and `H-0223` published, because main's selftest block landed above it this time
    (+461; this branch's own block is below it and did not move it). Substance: a `oneshot` killed at
    its deadline writes `Main process exited`, `Failed with result 'timeout'` and `Failed to start` in
    the same instant, so the hold it measures IS zero, and the leg pins `deadline`-with-0 and
    `clean`-with-0 as one refusal. Repair: allow `deadline` with hold 0 when the work line and the
    terminal line belong to the same activation; keep the refusal on `clean`.

## Non-blocking review items carried forward (PR #402's review at 2026-11-15T21:57:01Z on head `5408b41`; the reviewer marked all three NOT merge blockers — fixing them spends the approval again)

1. **A bare `-` bullet slips the empty-marker leg.** `scripts/check-next-md.sh` requires a space or
   tab after the marker, so a lone `-` line matches neither branch, and the fixture plants `- ` *with*
   the space, so the suite cannot see the class. Measured by the reviewer: rc 0 for a shipped
   bare-dash bullet. Repair: widen the pattern and add a fixture.
2. **The scanned-file list is unpinned.** The leg reads the hand-off, the skill and `AGENTS.md`, but
   every phantom-path fixture plants only in the hand-off — a future PR could narrow the list with the
   suite green. Repair: one fixture that plants the phantom in `AGENTS.md`. Item 6 of the owed list is
   the same gap aimed at a section title rather than a path.
3. **The shape leg early-returns after its first refusal**, so a hand-off with two shape defects
   reports one per run. Accepted as a first cut; recorded so the next unit does not mistake the
   behavior for a bug.

## Hard rules for whoever picks this up

- Checkpoint before the decision, one effect commit with a `Decision:` trailer, and
  `factory/next-decision-id.sh factory/DECISIONS.md` re-run after a fresh `git fetch` before taking an
  id. A `REFRESH-BLIND` answer must be re-read after a fetch; the journal shows both answers, and this
  branch's `H-0197` is on refs, so it is not a gap to reuse. `H-0225` is held by an unmerged sibling —
  see owed item 3.
- Run the gate with `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH` until #425 is
  repaired, and SAY so where you quote the verdict — the sanitization is a disclosed workaround, not a
  silent one.
- Before editing a file either log cites, run `python3 scripts/check-decisions.py --citations <path>`
  and fix what it lists in the same commit. Every `path:line` you write needs its quoted anchor on the
  same physical line or `POINTER_DEBT` rises and the gate refuses. Two ways to fail that: a quote
  wrapped onto the next physical line counts as bare, and citing a line number from a *mutated* tree
  is a pointer to nothing — quote refused orders, never cite them.
- Repair a merge's drifted citations by **row identity, never by a delta**: find the carrying row
  verbatim in the parent blob whose copy it is, open the cited row there, then find that cited row
  again in the merged file and move the number to it. Assert each repair is digits-only (identical
  with digits stripped) and re-open every anchor afterwards. Both of this family's alternatives —
  picking a side, or adding a measured offset — silently keep one parent's stale numbers.
- When a citation's checker message lists two candidate lines, resolve by opening the blob the citing
  entry shipped at, and cite that act in the entry (`H-0220`'s method).
- An `Options:` line must not contain `|`. The checker splits on it, so a union spelled with pipes is
  cut into several options and `Chose:` then refuses the entry. And `Chose:` must quote an option
  verbatim.
- A decision-log union is keyed by id and mechanical: splice the ascending union, then prove it — the
  numstat against each parent blob, the added-line census, the marker count, the additive line census
  on every auto-merged file, **both** whole-side orders actually run so the refusal is measured and not
  asserted, and a parent-relative landing census of every citation for the drift the gate cannot read.
  Check first whether the fresh ids interleave: if they do, run both orders. Where both sides appended
  at EOF, check whether the diff aligned a shared closer as context before deleting the markers. Check
  a shared entry's digit rows by opening its pointers on each parent — the newer side wins, and the
  sweep re-moves it anyway. Restore mutants with `cp` from a backup, never `git checkout` (#2215).
- Mutants and probes run on copies under `/tmp/opencode`, never the gated tree (#1807). `rm -rf` is
  refused by the harness here, so use fresh unique directories.
- Overwrite `NEXT.md`, carrying every still-true line you did not write, and re-measure every live
  coordinate you carry — this unit re-measured the seven the replaced file published and **two had
  moved** (`factory/selftest.sh:15213` → `:15674`, +461; `factory/selftest.sh:18109` → `:18570`, +461)
  while five still opened exactly (`scripts/check-next-md.sh:182`, `factory/next-decision-id.sh:546`,
  `factory/common.sh:214`, `factory/selftest.sh:725`, `Makefile:111`). Deleting another unit's owed
  item because your branch did not write it is the failure this file exists to end; the shipped
  *narrative* of another unit's hand-off is not an owed item and stays on the branch whose blob carries
  it.
