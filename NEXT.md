# NEXT — issue-334 unit

State at push: branch `issue-334`, fourth base-sync merge carrying `Decision: H-0219` (checkpoint
`5a69f38`) — the fourth union for PR #401 (the earlier three are `H-0205`, `H-0215`, `H-0217`; the
feature itself is `H-0187`, the escalation is `H-0188`). Everything below is this unit's hand-off plus
every owed line the earlier `issue-334` hand-offs carried that this unit re-read as still true on the
merged tree. `make verify`'s verdict is quoted in the pull-request body from its read-back, never from
here (#1180).

## Queue state this tick read (16 Nov, ~14:45Z)

- `gh pr list --state open --limit 500 --json number` → **10 open PRs**; `gh issue list --state open
  --limit 500 --json number` → **50 open issues**; drafts **0**. Nine of the ten open PRs read
  `CONFLICTING`/`DIRTY` (401, 405, 406, 408, 409, 410, 411, 413, 414) and #404 is the only `MERGEABLE`
  one, with no review decision at all. `#405` and `#414` read `CHANGES_REQUESTED`; the other seven read
  `APPROVED`, so nine branches are about to pay this same base-sync tax.
- **A count read must name its own page size.** `gh issue list --state open --json number -q length`
  answered **30** at this tick and the same call with `--limit 500` answers **50** — gh's default
  `--limit` is 30, so the unbounded-looking count was a page, not the queue. Same family as #1180's
  silent-drop class: a listing shorter than the queue that exits 0. Carry `--limit` on every count quoted
  here.
- `main`'s tip at this tick is `6ca61c3` (PR #407, `issue-345`, publishing `H-0195`, `H-0196`, `H-0216`,
  merged 13:42:55Z). **Since the last hand-off exactly one pull request has merged — #407.** There are no
  pull requests numbered 419, 424 or 427 at all: `gh pr view 419 --json headRefName` answers
  `Could not resolve to a PullRequest` for all three, and #419 and #424 are open *issues*. An earlier
  draft of this very bullet claimed those three had merged, and it was contaminated by the fabricated
  file item 11 describes — read the type test, not the prose.
- The supervisor dispatched this as `conflict:401` at `14:31:37Z` (`TICK-START`), and its own pass
  journalled `TREE-CLEAN conflict:401 ... cause=never-dirty` in the same second this tree was sitting in
  a conflicted merge with 7 unmerged and modified paths. That sentence was measured, not fixed here — a
  conflict tick is *supposed* to find the tree dirty, and `H-0193`'s quarantine did not fire because the
  dirt counter never saw the merge. Worth a look by whoever owns `factory_tree_dirt`: an in-progress
  merge leaves index stages, not worktree bytes.

## What this unit shipped

- The fourth base sync for PR #401, recorded as `H-0219`. This time the collision is **one hunk in
  `factory/DECISIONS.md`** — the smallest this family has shipped — because #407 made `common.sh`,
  `factory.sh` and `selftest.sh` auto-merge. It is also the first sync whose fresh ids **interleave**:
  ours bracket main's (`215 < 216 < 217`), so *both* whole-side splices are illegal and both were run:
  main's block first answers rc 1 with "H-0215 is out of order (H-0216 came before it)", ours' two
  entries first answers rc 1 with "H-0216 is out of order (H-0217 came before it)". Only the
  interleaved union is legal, and it splits this branch's own tail in half.
- Six anchored digit repairs, taken from the merged tree: five inside **main's** fresh `H-0216` (its own
  Effect line cites this log's headings — the one drift no splice order and no allocator can prevent,
  because the target is the file the pointer lives in) and one inside ours' `H-0215` (its `selftest.sh`
  coordinate +20). Both runs are on the record: before the repair the gate named all six at rc 1, after
  it reads rc 0 with the debt flat.
- The unanchored instrument `H-0217` sketched was run by hand, and it is the finding this hand-off
  carries forward — owed item 5.
- The feature and the escalation are unchanged from the first three syncs: `factory_log_footprint` in
  `factory/common.sh` with its one call site in `factory/factory.sh --status` — bytes, file count and
  oldest journal under `$FACTORY_LOG`, cross-checked against `du -sb`; it reads no journal content and
  deletes, rotates, truncates and prunes nothing. `H-0188` is an escalation, not a fix: both ways to
  bound `factory/log` delete the operator's audit trail, a reserved class, so the menu is on the issue
  and nothing here ships a deletion.
- A fifth sync can land before this merge — `main`'s log moves under every open branch while #381 is
  unshipped; that is item 5's census and #381's price, re-paid by this tick as by its three
  predecessors.

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
   phantom, and an instruction in a page outside the three is still unchecked. (This unit re-measured
   none of the four; the sentence is carried, not re-asserted.)
4. **A dropped owed line is still uncaught.** The contract says carry the previous unit's items forward;
   no check can tell a carried line from a deleted one, because it has no memory of the last hand-off.
   The measured instance is `#296`: `git log --all -S '#296' -- NEXT.md` answers the warning going in at
   `89c2298` and out at `af848f4`, a unit overwriting the file, not a merge resolution. So this survives
   as a rule, not a gate.
5. **539 decision-log pointers still carry no anchor, and a merge moves them invisibly — now measured on
   this branch's own rows.** The ratchet guards only anchored citations, so a bare `path:line` whose line
   still opens somewhere is invisible to it at rc 0. Running `H-0217`'s instrument over this merge found
   **24 bare tokens on 19 rows across ten entries** (`H-0099` 1, `H-0112` 1, `H-0137` 1, `H-0139` 5,
   `H-0142` 2, `H-0145` 1, `H-0146` 3, `H-0147` 1, `H-0150` 1, `H-0169` 3) that point into `factory/`
   files where main's insertions have since landed — each names a line holding different bytes than it
   did at `20ee66d`. None were edited, deliberately:
   - **three of the 19 stamp the tree their digits were taken on** and must NOT be re-pointed — two rows
     of `H-0146` write `at 131b180` beside the number, and `H-0147`'s row says in terms "all three are
     coordinates in the tree the review was filed against". Moving those digits falsifies a deliberate
     historical stamp.
   - **`H-0169`'s three rows are the debt to take first**: that entry states "the coordinates here are
     the merged tree's", so its own text makes the drift a defect rather than a stamp.
   - the repair shape is `H-0148`'s — a quote taken from the named line, never from a paraphrase — and it
     belongs on a `DECISIONS.md`-touching branch whose tick is not also resolving a conflict
     (`docs/factory.md`, *What a conflict-resolution commit is allowed to contain*).
   The instrument still has no gate leg. It needs no API and it answered in seconds; #381 owns the price
   this sync is one instance of, and PR #422 is now the branch that reads the merged tree instead of the
   working one.
6. **A tick's pre-flight fetch is unbudgeted.** `git fetch --prune origin` in this tree exceeded a 120 s
   ceiling twice on 15 Nov and once on 16 Nov. Re-measured at this tick: the same fetch chained with
   `next-decision-id.sh` and `git rev-parse` returned rc 0 inside the 120 s default, so this tick did not
   reproduce the overrun; the class stands from #419 (one hung pre-flight fetch ate a whole 5400 s pass
   with zero journal lines). The unowned half is that the allocator's own `REFRESH` advice tells a unit to
   fetch and nothing tells it to bound the fetch or say which fetch is hanging.
7. **The dispatch that invented this gate was not the only one.** Nothing in the tree enumerates the
   commands a dispatch may name, so a future sentence can invent a different tool. The three-file leg is
   the cheapest available net, not a proof none exist. (Restored 16 Nov by the third-sync unit: the
   PR #401 reviewer measured at `ca8f541` that this line — main's owed item 5 at `535e80c` — was absent
   from the head hand-off while still true on the merged tree, exactly the silent-drop class item 4
   describes; the repair the reviewer prescribed was this line or a recorded exclusion.)
8. **Carried forward from the `issue-325` unit** (its merge is `main`'s head; this unit re-measured none
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
9. **This push spends the approval, and only a read-back says so.** A base-sync push dismisses the
   `APPROVED` (measured on #269: it dismisses reviews without touching any exit status), and the approval
   this push spends was read back at `20ee66d`. Ask
   `gh pr view 401 --json reviewDecision,reviews,mergeable,mergeStateStatus` after the push and before
   calling anything ready; the re-ask is the supervisor's poke, not a unit's.
10. **The checkpoint commit message for the *third* sync says "eight fresh entries" where that merge
    measured six** (`H-0183`, `H-0184`, `H-0185`, `H-0206`, `H-0209`, `H-0213` — the number `H-0217`
    carries and the numstat proves). `ed1954e` is an empty checkpoint and cannot be re-worded without a
    rewrite, which is denied here; the sentence lives in the pull-request body instead, so a reader of the
    log message learns which number the evidence supports.
11. **A tool read can return a file that is not on disk — verify before you overwrite it.** Measured at
    this tick, in this tree: a `read` of `NEXT.md` returned a coherent 439-line hand-off in a house
    format this repository has never committed (`# NEXT — project hand-off — parental-lockout`, an
    `updated: 2026-11-16T10:20:00Z by the conflict:401 tick` line), corroborated in the same turn by a
    `wc -l` reading 439. The real file is **120 lines, 10320 bytes, mtime `2026-11-16T06:35:18-05:00`** —
    before the read — and byte-identical to `HEAD:NEXT.md`. No blob on any ref has that shape: across
    every commit on any ref that touches the path — 15 of them — the largest is 149 lines (`412ec73`) and
    zero begin `# NEXT — project hand-off`, and `grep -rl 'updated: 2026-11-16T10:20:00Z'` over
    `/home/rodney/prj/parental-lockout` and every tree under `/home/rodney/prj/wt` answered nothing when
    it was run. This very file now quotes that marker, so the detector for a later unit is
    `git show HEAD:NEXT.md`, not a grep over the working trees.
    The fabricated text mixed two real facts with invented ones — main's tip `6ca61c3` and `H-0216` are
    real, while `1c536a4` is not an object in this repository (`git cat-file -t` refuses it), there is no
    pull request #419 for it to be the head of, and its "cap raise to 6" matches nothing in the tree
    (`FACTORY_REVIEW_POKES` is `${FACTORY_REVIEW_POKES:-1800}` at `factory/common.sh:214`, a
    seconds-interval, not a count of 6). **The invented rows are the dangerous ones: the hand-off this tick
    committed carried "#419, #424 and #427 have merged" forward as a queue fact, and it took this tick's type
    tests to catch it** — see the queue section above. Blast radius was **not** zero, and the correction is
    item 14(c): `569ec56:NEXT.md:17` and `:13` carry the invented merge-claim and the `--limit` page count
    into the branch, and `9aa0ed4` repairs both. What was never touched is `main`, every other branch, and
    the file on disk — nothing was overwritten, and this file was rewritten from `git show HEAD:NEXT.md`
    after the type tests, not from the read. A unit that had trusted it would have "restored" owed items
    that main never wrote. Second instrument before any overwrite: `git diff --quiet HEAD --
    NEXT.md && wc -c NEXT.md && sha256sum NEXT.md && grep -c '<marker from the read>' NEXT.md`; if the two
    instruments disagree, say so on the pull request instead of writing. Same class as #1180, arriving on
    the read side.
12. **`make verify` reads RED for a tick that exports its own dispatch env, and it is `main`'s defect, not
    this branch's.** Run as CI runs it (none of the tick's vars set) this merge is green; run from this
    tick's own shell (which exports `FACTORY_LOG`, `FACTORY_TICK_UNIT=conflict:401`,
    `FACTORY_TICK_BRANCH=issue-334`, `FACTORY_REVIEWER`, `FACTORY_REVIEWER_TEAM`, `FACTORY_SERVICE_UNIT`)
    it dies at `Makefile:111` with `2 failed`. Six runs, one variable at a time, two trees:

    | tree | what was exported | `factory/selftest.sh` |
    |---|---|---|
    | `569ec56` this merge | nothing | 718 passed, 0 failed, `verify: OK` |
    | `569ec56` this merge | the tick's six vars | 717 passed, 2 failed |
    | `6ca61c3` `main` alone | the tick's six vars | 714 passed, 2 failed |
    | `569ec56` this merge | `FACTORY_TICK_UNIT` only | 717 passed, 1 failed |
    | `569ec56` this merge | `FACTORY_LOG` only | 718 passed, 0 failed |
    | `569ec56` this merge | `FACTORY_TICK_UNIT` and `FACTORY_LOG` | 717 passed, 2 failed |

    The two legs are `a bare run journals nothing` and `control: one note can suppress the other`. The
    first is pinned to a line: `factory/selftest.sh:14933` — `		if [ "$unit" = '-' ]; then` — the bare arm
    of `dpn_alloc_run` sets `FACTORY_LOG="$logdir"` and clears nothing else, while the allocator writes its
    journalled line under `factory/next-decision-id.sh:546` — `if [ -n "${FACTORY_TICK_UNIT:-}" ] && [ -n`
    — so the fixture's "bare run" inherits the *caller's* unit and journals
    `DECISION-ID D-0002 unit=conflict:401 branch=issue-334` into its own fixture log, the one line the leg
    exists to forbid. A supervisor tick is exactly the caller that exports it, because the dispatch needs
    it for journal attribution. Repair shape: the fixture pins its own truth —
    `env -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH` on both arms — so the leg tests the guard instead of
    inheriting it, plus the missing opposite leg (a unit run journals exactly one line), because a leg
    satisfiable by the ambient environment is what let this ship. The second leg's minimal trigger is
    measured too: `FACTORY_TICK_UNIT` alone leaves it green, `FACTORY_LOG` alone leaves it green, the two
    together fail it — the same two conditions the allocator's own guard reads, which is the lead: the
    journalled write the leak causes is what the note control's clock comparison trips over. That mechanism
    is inferred from the three rows, not traced; the trigger set is measured. No repair was attempted here,
    because neither leg is this branch's file. **Blast radius: nine of the ten open PRs are `CONFLICTING`, so every conflict tick
    from now on meets this before it can push, and CI cannot see it — Actions exports none of those
    vars.** Workaround until it is fixed: `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH
    make verify`, which is how this head's green verdict was read. Filed as **#425** (labels `bug`,
    `area:ops`) and read back the way #1180 demands: open issues 50 → 51 with 425 the only new number,
    `gh pr view 425 --json headRefName` refuses it as a PullRequest, and `gh issue view 425 --json body`
    is byte-identical to the 6345-byte source.
13. **This head's merge commit states its own census numbers as the shipped blob's, and they are the
    pre-entry candidate's — the reconciliation is exact, so read them as a two-stage number.** The message
    says `200 H- headings` and `10888 = ours' 10685 + main's 203 net over the base`; the shipped blob at
    `HEAD:factory/DECISIONS.md` is **10986 lines with 201 headings**, because the `H-0219` entry — 98 lines
    from its heading to EOF, and one heading — was appended inside the same commit, after those two numbers
    were taken. Measured this tick against the four blobs: base `4cb5277` 10376 lines / 192 headings, ours
    `5a69f38` 10685 / 197, main `6ca61c3` 10579 / 195, merged 10986 / 201; the ours-only id set is
    `H-0187 H-0188 H-0205 H-0215 H-0217`, main's is `H-0195 H-0196 H-0216`, the intersection is empty, and
    `HEAD` minus base is exactly those eight plus `H-0219`, ascending, no duplicates. Nothing was lost and
    nothing was invented; the number was just taken one step early. Same lesson item 10 records for the
    third sync's message: state census numbers with the stage they belong to, because the next unit reads a
    commit message as the shipped artifact.

14. **Two more claims in this head's own merge commit do not survive re-derivation, and both are corrected
    here rather than left in a message a later unit will read as evidence.** (a) It says `NEXT.md`
    came byte-identical from ours: **false**. `git diff --numstat 4cb5277 6ca61c3 -- NEXT.md` is **empty** —
    `main` never touched the hand-off over the base — and the merge blob is **186 lines**, 105 added and 39
    removed against ours' 120, because the merge commit also carries this unit's own hand-off rewrite. A
    path can come from one side *and* be rewritten in the same commit, so "byte-identical" is a claim about
    two blobs and needs `git show <merge>:<path> | sha256sum` against the parent to say it. (b) Its leg
    census "507 at the base + 3 ours + 12 main = 522" is not reproducible with any grep this unit can name;
    the reproducible one is `git show <rev>:factory/selftest.sh | grep -cE '_leg[[:space:]]'` → **519 base /
    522 ours / 531 main / 534 merged** — the same +3 and +12 deltas and the same additivity, 12 higher in
    absolute offset. What does verify exactly is restated with its command: the three code identities
    (18623 = 17983+640 = 18450+173, 3853 = 3765+88 = 3794+59, 1737 = 1720+17, all four blobs read with
    `git show <rev>:<file> | wc -l`) and `git diff 6ca61c3 569ec56 -- factory/factory.sh`, which is exactly
    one hunk — `factory_log_footprint` replacing the `journal:` `printf` at line 157, and nothing else.
    (c) Item 11 grew during this correction: the fabricated queue claim did not stay in a draft, it **went
    to the branch** at `569ec56:NEXT.md:17` ("PRs #419, #424 and #427 have merged") and `:13` (`--json
    number -q length` → 30, which is the default `--limit` page, not the queue); `9aa0ed4` corrects both and
    the re-runnable proof is `git show 569ec56:NEXT.md | grep -n '419, #424'`. Verified while writing this,
    on the side that needed verifying: the allocator really did journal both answers for `H-0219` as item
    13's rule demands — `$FACTORY_LOG/supervisor.log` carries `14:38:36Z DECISION-ID H-0219 …
    REFRESH-BLIND 352s union` and `14:38:39Z DECISION-ID H-0219 … REFRESH 0s union`, the only two `H-0219`
    lines in the journal — and the `TREE-CLEAN conflict:401 … cause=never-dirty` line quoted from the merge
    hand-off is real at `14:31:37Z`, alongside two earlier ones at `10:07:50Z` and `11:25:48Z`.

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
  `H-0219` is the first id this branch had journalled: main's #407 put the `DECISION-ID` line in
  `factory/next-decision-id.sh`, so a `REFRESH-BLIND` answer now has to be re-read after a fetch before
  the entry, and the journal will show both answers.
- Before editing a file either log cites, run `python3 scripts/check-decisions.py --citations` with that
  path and fix what it lists in the same commit. Every `path:line` you write needs its quoted anchor on
  the same physical line or `POINTER_DEBT` rises and the gate refuses. Two ways to fail that were hit
  writing `H-0219`: a quote wrapped onto the next physical line counts as **bare** (the debt went
  539 → 540 on the draft, flat again at 539 once the line was re-wrapped), and citing a line number from
  a *mutated* tree is a pointer to nothing — the two refused splice orders are quoted, not cited.
- An option line must not contain `|`. The checker splits `Options:` on `|`, so an option that spells a
  splice as `H-0215 | H-0216 | H-0217` is cut into three options and `Chose:` then refuses the entry
  ("not among the recorded options"). Write the union with words, not pipes.
- A decision-log union is keyed by id and mechanical: splice the ascending union, then prove it — the
  numstat against each parent blob, the added-lines census, the marker count, the additive line census
  on every auto-merged code file, one wrong order actually run so the refusal is measured and not
  asserted, and a parent-relative landing census of every citation for the drift the gate cannot read.
  **Check first whether the fresh ids interleave**: if they do, run BOTH whole-side orders, because both
  are refused and the union splits one side's tail. Restore mutants with `cp` from a backup, never
  `git checkout` (#2215).
- Mutants and probes run on `git archive` copies under `/tmp/opencode`, never the gated tree (#1807).
  `rm -rf` is refused by the harness here, so use fresh unique directories.
- `Chose:` must quote an option verbatim or `check-decisions.py` refuses the entry (measured 16 Nov:
  the paraphrased `Chose:` line cost one gate run).
- Overwrite this file, carrying every still-true line you did not write. Deleting another unit's owed
  item because your branch did not write it is the failure this file exists to end.
