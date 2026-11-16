# NEXT — issue-348 unit (PR #410, dispatched `conflict:410`)

State at push: branch `issue-348`, **fourth** base sync for PR #410, recorded as `Decision: H-0228`
(checkpoint `fa96ba4`, cut before the merge started); the third sync is `Decision: H-0225` (checkpoint
`674cb95`) and the prose round that answered the review is `Decision: H-0227` (checkpoint `ac0879b`).
This merge takes `origin/main` `5aaa9a7` — PR #408, the `issue-346` merge, carrying `H-0197`, `H-0223`
and `H-0226` — on merge base `580d722`; it is the sync this branch's own item 20 named as owed.
Everything below is this unit's hand-off plus every owed line from the file this unit overwrote **and**
from `main`'s hand-off (`origin/main:NEXT.md`, the `issue-346` unit's) that this unit re-read as still
true on the merged tree. Six owed items are new to this file -- five carried from `main`, one measured
here -- and five hard rules, four of them this branch's copy never having carried them — deleting another unit's owed item because your branch did not write it is
the failure this file exists to end. `make verify`'s and `factory/selftest.sh`'s verdicts are quoted in
the pull-request body from the run that ships these bytes, never from here (#1180). Coordinates marked
**re-measured** were opened on this merged tree; the rest are carried in their authors' words and were
not re-run.

## What this unit shipped

- The fourth base sync for PR #410, `H-0228`: FOUR conflicted files — `NEXT.md`,
  `factory/DECISIONS.md`, `factory/factory.sh`, `factory/selftest.sh` — resolved as a union that keeps
  both parents, on `origin/main` `5aaa9a7` with merge base `580d722`. It is the first sync in this
  family where **both sides appended a whole self-test section at the same anchor**: base line 20590
  takes main's `mtj_*`/`proc_leg` block (325 lines) and this branch's `wtc_*` block (219) in front of
  the shared tally, so the file now carries both and `bash -n` parses it.
- The code seam: main's rewritten `factory_body` and its `MAIN-CHECKOUT` pass are kept whole and the
  census loop is grafted behind the pass's **last** fetch — `factory/factory.sh:1549`
  "done < <(factory_worktree_lines" is the graft, **re-measured** — because the census's deadness
  verdict reads `origin/main`, so it must run after every fetch the pass makes. No leg asserts an order
  between the two sections' log lines; each greps its own prefix out of the journal after the pass.
- Both whole-side orders were **run**, in shared clones under `/tmp/opencode` and never in the gated
  tree, and the finding is an asymmetry: `--theirs` refuses at rc 1 with 15 problems naming 20 drifted
  anchored citations, while `--ours` reads **rc 0** — 288 entries, 750 citations, nothing to refuse —
  and destroys 84 `factory/DECISIONS.md`, 244 `factory/selftest.sh`, 3 `factory/factory.sh` and 268
  `NEXT.md` digit-normalized lines plus `H-0197`, `H-0223` and `H-0226`. The citation gate is blind to
  that order; only the added-line census sees it (item 22).
- Seventeen drifted anchored citations repaired by row identity against the parent blobs, each
digits-only and re-opened afterwards. The gate prints twelve refusals and withholds five behind
`... and 5 more`, and **7 of those 12 named two to five candidate lines**; each ambiguous one was
decided by opening the blob the citing entry shipped, so the ambiguity count is a floor off a
truncated refusal and not a total: `H-0216`'s `factory/DECISIONS.md:11575` "our two entries
  at" resolves to that row because `origin/main`'s own line 11474 is the same row of the same entry,
  not a self-citation. `POINTER_DEBT` **re-measured** at `scripts/check-decisions.py:595`
  "POINTER_DEBT = 546": 292 entries, 774 citations, 546 owing an anchor, 0 unopenable. Stages, as item 13
  insists: the log splices at 221 ids and 12183 lines and ships at 222 ids and 12193 lines once
  `H-0228` is in it, and the numbers in the bullet above are the shipped ones.
- One drift was **this unit's own, not the merge's**: the first splice dropped the blank line main's
  `H-0226` junction added, landing main's section header on 20589 where main publishes it at 20590. The
  citation gate would have "repaired" that by re-pointing a pointer main never moved; the additive line
  census against both parents caught it, and the seam was rebuilt from the parent blobs instead.
- `NEXT.md` — rewritten by this unit (overwriting, per the contract). This time the overwrite had to
  **add**: main's hand-off held owed items and hard rules this branch's copy never carried, so they are
  in below as items 22–27 rather than deleted with the file.
- The feature #348 asked for, unchanged by the sync: `factory_worktree_lines` in
  `factory/common.sh:1490` "factory_worktree_lines() {" — **re-measured** — a census that **reports**
  dead worktrees (`WORKTREE-ORPHAN <path> head=<sha> reason=merged-and-clean|outside-trees`), prunes
  git's admin rows in the same pass (`WORKTREE-PRUNABLE`/`WORKTREE-PRUNED`, `DRY-RUN` under
  `DRY_RUN=1`), and says `WORKTREE-CENSUS-FAILED` rather than being silent; plus its legs in
  `factory/selftest.sh`, including the control that dirties the planted tree and requires the line to
  be absent.
- Nothing was removed. `git worktree remove` is refused to this session (the permission-refusal class
  #1180 names) and forced removal of a directory someone may be standing in is the reserved act #348
  itself declines.

## What this unit did not deliver

- **Nothing on the merged tree was re-run for behaviour.** Both fixture blocks came through as bytes:
  `bash -n` parses `factory/selftest.sh`, the additive census holds for every conflicted file, and
  `python3 scripts/check-decisions.py` reads rc 0. main's `H-0223` legs, `H-0226`'s measurements and
  this branch's own census legs were not executed by this unit — `make verify` and `make
  check-mutations` on these bytes are the first runs, and their verdict ships in the pull-request body.
- **main's fresh entries are carried as their author's numbers.** `H-0197`, `H-0223` and `H-0226` —
  including `H-0226`'s claim that its own sweep repaired twenty coordinates on twelve rows — were not
  re-derived here. Re-measure before acting on one.
- **The two carried limitations stand unchanged.** `factory_main_tree_check` still reads one tree
  (#342 ask 5 made it narrower than the pass's own reach), and the census is still report-only in fact:
  `H-0197`'s *What lost* (three race arms, no planted leg) and `H-0225`'s (the prune arm has never run
  live) are both still true, and item 17 below is the live instance.

## Queue state this tick read (16 Nov, ~21:45Z)

- `gh pr list --state open --limit 500 --json number,reviewDecision,mergeable,mergeStateStatus,isDraft`
  → **4** open PRs: #410, #411, #413, #414. `gh issue list --state open --limit 500 --json number` →
  **55** open issues. Drafts **0**. `#414` reads `CHANGES_REQUESTED`; the other three read `APPROVED`.
  **All four read `CONFLICTING`/`DIRTY`** — the previous hand-off counted 5 open with 2 conflicting, so
  #408 merged since and every survivor is now behind `main`. Both counts carry `--limit 500` (gh's
  default page is 30) and both were counted from raw JSON in python, not a `-q` template, because a bare
  generator inside an interpolated string drops rows at rc 0 (#1180, measured 14 Nov).
- `main`'s tip is `5aaa9a7` (PR #408, `issue-346`: `H-0197`, `H-0223`, `H-0226`). This branch's item 20
  predicted this sync with `git merge-tree --write-tree HEAD origin/main` at `13afad2` — exit 1, the
  same four files — and this tick's merge conflicted on exactly those four.
- This branch's own state before the write: #410 `OPEN`/`APPROVED`/`CONFLICTING`/`DIRTY` at head
  `5881fce`, 7 commits, merge base `580d722`. Its `statusCheckRollup` returns **zero rows** while
  `gh run list --branch issue-348` shows its three CI runs all `success` — so the rollup is GitHub not
  having recomputed, not a red pipeline. Read the runs, not the rollup, before concluding anything about
  this branch's checks. The push spends that approval (item 9).

## The census's first live run — measured, read-only, at this tick

`DRY_RUN=1 FACTORY_TREES=/home/rodney/prj/wt bash -c '. ./factory/common.sh; factory_worktree_lines
/home/rodney/prj/parental-lockout'`, output at `/tmp/opencode/census-live.txt`: **16 `WORKTREE-ORPHAN`
rows, 0 `WORKTREE-PRUNABLE`, 0 `WORKTREE-CENSUS-FAILED`** — 5 `reason=merged-and-clean` inside
`$FACTORY_TREES` and 11 `reason=outside-trees` under `/tmp/opencode`. Four pairs share a head
(`404-ctrl-main`/`main-ctrl`, `base-332`/`wt-main-392`, `h401-a`/`h401-b`, and the three
`t328/adv/n[123]/wt` on one head), which is `AGENTS.md`'s "two of them on the same commit" at six-weeks'
remove. Two facts a later unit should not have to rediscover:

- **`#348`'s named orphans are gone from this box.** `/home/rodney/prj/.wt-armcheck` and
  `/tmp/opencode/p302/pre` both answer `No such file or directory` at this tick, so the issue's live
  example is closed by removal someone else performed, not by this branch. The mechanism it describes
  stands and is now quantified by the 16 rows above.
- Zero `WORKTREE-PRUNABLE` rows means the first **live** pass (no `DRY_RUN`) has nothing to prune on
  this box: it is report-only in fact today, not only by design. Re-run the dry preview before believing
  that about any later day.

## What is still owed (all of it, carried forward; none of it silent)

1. **`H-0188` still owes a human answer, and it is the only thing that bounds `factory/log`.** Issue
   #334 is still `OPEN`, and the measured trajectory the `issue-346` hand-off carries (not re-measured
   here) is in PR #401's body: 20 MB / 100 files on 12 Nov, 36 MB / 172 at those bytes. Rotation and
   truncation are both deletions of the operator's records, a reserved class, so no unit may pick one; read the escalation comment on #334 before touching `factory/common.sh`'s
   footprint function again.
2. **Nothing here reads whether a hand-off is TRUE.** `scripts/check-next-md.sh` reads the tree and
   never the API, so a well-formed hand-off about work `main` finished last week still passes. A leg
   that asks GitHub needs a token CI does not have; `scripts/check-denies.py` is the precedent for
   skipping loudly. Item 26 below is this gap's sibling, carried from the `issue-346` hand-off: the gate
   cannot see a sentence whose target is a **section** rather than a file.
3. **The phantom-path leg reads three files, not every file** (`docs/factory.md`,
   `factory/selftest.sh`, `factory/next-decision-id.sh`, plus `testing/capture/cdp.py`'s generated
   manifest). Carried, not re-measured — **but this tick did measure a phantom of the adjacent kind, in
   the hand-off itself**: the `issue-334` item 5 cited *`docs/factory.md`, "What a conflict-resolution
   commit is allowed to contain"*. No such heading exists on the merged tree
   (`grep -n '^#\{2,4\} ' docs/factory.md | grep -i conflict` → nothing; the only "conflict-resolution"
   hit is prose at `docs/factory.md:658` inside `### The approval a push voids`). The rule it meant is
   the `H-0143`/`H-0144` lineage in `factory/DECISIONS.md` — a published coordinate is re-pointed, never
   rewritten. #337 is the open issue for a contract naming a gate that does not exist; this row is one
   more instance of it, in prose rather than a dispatch. Widening the leg means naming why each of those
   four is not a phantom -- the `issue-346` unit counted the same four and declined all four on exactly
   that ground, which is the work the leg needs before it can be widened.
4. **A dropped owed line is still uncaught** — no check has memory of the last hand-off. The measured
   instance is #296 (`git log --all -S '#296' -- NEXT.md`: in at `89c2298`, out at `af848f4`) -- a unit
   overwriting the file, not a merge resolution, which is why the rule binds a unit and not a tick. This
   tick's evidence for the rule working is item 10 below: a carried coordinate that had moved 1500 lines.
5. **546 decision-log pointers still carry no anchor, and a merge moves them invisibly.** The
   `issue-334` hand-off said 539; the merged tree measures 546 (`python3 scripts/check-decisions.py`'s
   closing line, against `POINTER_DEBT` at `scripts/check-decisions.py:595`), and the ratchet only
   guards anchored citations, so a bare `path:line` whose line still opens somewhere is invisible at
   rc 0. `H-0217`'s instrument still has no gate leg; PR #422 is the open branch that reads the merged
   tree instead of the working one, and #381 owns the re-conflict price this sync is one instance of.
   The constraint on that sweep, measured by the `issue-346` unit over the `issue-334` merge (24 bare
   tokens on 19 rows across ten entries, left untouched deliberately): three of those rows stamp the tree
   their digits were taken on and must **not** be re-pointed, so a bare token needs a provenance verdict
   before it moves, not just a delta.
6. **A tick's pre-flight fetch is unbudgeted.** #419 stands: one hung `git fetch --prune origin` ate a
   whole 5400 s pass with zero journal lines, and the allocator's own `REFRESH` advice tells a unit to
   fetch without bounding it. This tick's fetch returned inside the 120 s default, so no new overrun; the
   `issue-346` unit measured the other arm of the same class, its own post-merge fetch at 0.572 s at rc 0,
   so one fast fetch is evidence of intermittence and never of a repair.
7. **The dispatch that invented the NEXT.md gate was not the only one** — nothing in the tree enumerates
   the commands a dispatch may name, so a future sentence can invent a different tool. The three-file leg
   is the cheapest available net, not a proof that none exist.
8. **Carried from the `issue-325` unit** (not re-measured here; re-run before acting on any row): the
   deleted-call hole in `scripts/attack-capture-self-test.py` (removing
   `_rule_probes_are_live(registry, ran, bad) from self_test()` is still green, D-0076's class, and
   D-0081 says the tally-of-what-ran closure did NOT ship); the switched-off PROBE-prefix derivation
   (a recorded trade, D-0081 — do not relitigate without a new measurement); and #325 itself stays open
   until its own test is met on the merged tree, with #1256's closing-keyword adjacency rule respected
   either way.
9. **This push spends the approval, and only a read-back says so.** Measured on #269 and restated at
   `docs/factory.md:652` "### The approval a push voids": a base-sync push turns `APPROVED` into a
   `DISMISSED` row without touching any exit status. Ask
   `gh pr view 410 --json reviewDecision,mergeable,mergeStateStatus` after the push; the re-ask is the
   supervisor's poke (`factory_request_review`), not a unit's.
10. **Re-measure every live coordinate you carry — this tick's worst moved by 1500 lines.** The
    `issue-334` hand-off cited `factory/selftest.sh:17070` for `reap_leg_live`'s refusal; on this merged
    tree the sentence is at `factory/selftest.sh:18570` "the live journal produced a deadline hold of 0s".
    Same pass, others: the `dpn_alloc_run` bare arm `14933 → 15674`, `mm_repo_new` `3585 → 3950`,
    `mm_merge` `3615 → 3980`, the mid-merge detector leg `3676 → 4041`, the quarantine round trip
    `3767 → 4132`. Unchanged and re-checked: `factory/next-decision-id.sh:546`
    `if [ -n "${FACTORY_TICK_UNIT:-}" ] && [ -n`, `factory/common.sh:214`
    `FACTORY_REVIEW_POKES=${FACTORY_REVIEW_POKES:-1800}`, `Makefile:111` (the `verify:` recipe line).
    The cause is arithmetic: main took +4285 `selftest.sh` lines between the two hand-offs.
11. **A tool read can return a file that is not on disk — verify before you overwrite it.** Measured on
    the `issue-334` tick: a `read` of `NEXT.md` returned a coherent 439-line hand-off in a format this
    repository has never committed, corroborated by a `wc -l` in the same turn, while the real file was
    120 lines and byte-identical to `HEAD:NEXT.md`. Its invented rows ("#419, #424 and #427 have merged")
    went to the branch at `569ec56:NEXT.md:17` before a type test caught them; `9aa0ed4` corrects it.
    Instruments before any overwrite: `git show HEAD:NEXT.md`, then `git diff --quiet HEAD -- NEXT.md`,
    `wc -c`, `sha256sum`, and `grep -c '<marker from the read>'`. Same class as #1180, arriving on the
    read side. The type test #2027 asks for is `gh pr list --head <branch> --state all --json
    number,state`, and this tick ran both reads: it answers `410 OPEN` at `f2ca7e1`, and
    `gh pr view 410 --json ...,headRefOid` answered `f2ca7e1`/`APPROVED`/`CONFLICTING`/`DIRTY` — the read
    the checkpoint commit message quotes.
12. **`make verify` reads RED for any shell that exports its own dispatch env, and it is `main`'s defect
    (#425, still open).** This tick's shell exports six `FACTORY_*` vars including
    `FACTORY_TICK_UNIT=conflict:410`, so the head's verdict was read as CI runs it:
    `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make verify`, and that sanitization
    is disclosed wherever the verdict is quoted, not hidden. The two legs that trip are `a bare run
    journals nothing` and `control: one note can suppress the other`; the leak is the journalling guard at
    `factory/next-decision-id.sh:546` inheriting the caller's unit into a fixture's "bare run". Do not
    repair it by weakening either leg. Still unrepaired on this merged tree, **re-measured**:
    `grep -c 'env -u FACTORY_TICK_UNIT' factory/selftest.sh` answers 0, and the two legs' fixture arms are
    the coordinates the `issue-346` unit pinned at `factory/selftest.sh:15674` and
    `factory/selftest.sh:18570` (+461 on the tree it measured, both re-opened here).
13. **State a census number with the stage it belongs to.** Two hand-offs now carry this lesson: the
    third sync's checkpoint message said "eight fresh entries" where six measured (`ed1954e`, an empty
    commit that cannot be re-worded), and the fourth's merge message stated the pre-entry blob's totals as
    the shipped blob's. This unit's numbers are labelled the same way they are in `H-0225`: `DECISIONS.md`
    9397+101+2576=12074 is the **union of the two parents**, and the shipped blob is 12083 lines with 217
    headings because `H-0225`'s own 9 lines ride the same commit.
14. **A claim about coverage comes from the legs and their asserted substrings, never from a comment
    about redundancy; a claim about *why* something was logged needs the clock of what it is compared
    against.** Both halves are the `issue-334` unit's withdrawn accusation about `factory_tree_dirt` (the
    detector does probe a conflicted tree — `factory/selftest.sh:725` and `:726`, `mm_repo_new`,
    `mm_leg 'the detector names a mid-merge tree'` — and `ORIG_HEAD`'s mtime is free). Full narrative in
    `git log -p NEXT.md` at `569ec56`/`9aa0ed4`; the rule is what travels.
15. **A control tree must contain the legs it controls.** Ancestry is not content: `d3d8782` carries
    `569ec56` in its history and zero `dpn_leg` legs. Grep the control tree for the fixture by name before
    any differential run against it.
16. **`reap_leg_live` refuses a hold that is legitimately zero** (carried from `issue-340` through
    `conflict:405`; coordinate re-measured in item 10). A `oneshot` killed at its deadline writes its
    terminal lines in the same instant, so the hold it measures IS zero and the leg pins `deadline`-with-0
    and `clean`-with-0 as one refusal. Repair: allow `deadline` with hold 0 when the work line and the
    terminal line belong to the same activation; keep the refusal on `clean`.
17. **New, and this branch's own:** the census has never run with the prune arm live, and its legs only
    ever plant fixture repositories. The 16-row live preview above is the only read against real state,
    and it was taken with `DRY_RUN=1`. A unit that wants the prune exercised for real should watch the
    next supervisor pass's journal for `WORKTREE-PRUNED`/`WORKTREE-PRUNE-FAILED` rather than aiming the
    function at someone else's tree.
18. **New:** #348's report names trees that are *another unit's live scratch* (the three
    `t328/adv/n*/wt`, `s406/wt-merge`). That is the intended behaviour — reported, never removed — but it
    means the line count in a supervisor pass is a count of other people's work, not a to-do list. Anyone
    tempted to act on a `WORKTREE-ORPHAN` row reads that tree's owner first.
19. **`make verify` never *applies* the mutant sets — the apply-and-compile half lives only in
    `make check-mutations`, which CI runs as `mutation sets` — and a base sync is exactly what
    stalenesses them.** `Makefile:48` "kept OUT of verify for the opposite reason" states the split as a
    cost decision: the checker compiles the tree once per mutant and pays minutes. Read the split
    narrowly, because the set files are not absent from `verify`: its `==> script and set-file encoding
    guards` step (`Makefile:116`) runs `scripts/check-scripts.py`, whose own docstring calls that file
    "the toolchain-free half of check-mutations.py" and whose pass opens every file under
    `scripts/mutations/` for its bytes. Measured in a `git archive` copy of `13afad2`: one appended
    `0xff` byte in `scripts/mutations/version.json` stops `make verify` at that step with
    `1 set-file encoding problem(s)` (rc 2), while `check-mutations.py --self-test` stays green at 99
    rules over the same bytes. So what a green local run cannot see is a stale **anchor**, not a bad
    byte — and a sync whose whole content is moved code lines is exactly what moves a mutant patch off
    its anchor, so a sync unit runs `make check-mutations` before pushing rather than letting CI find
    it. Measured at head `e8ab8b9`: the local run answers `all 847 mutants apply and compile`, with its
    own caveat printed beside it — `this does not run the suite, so a mutant that survives every test is
    invisible here (make mutate)` — and CI's `mutation sets` job on that head ran 20:57:16Z→21:13:35Z,
    16m19s, success; the same job at `13afad2` ran 21:15:24Z→21:27:51Z, 12m27s, success. Two mechanics
    worth knowing before you start it: it applies each mutant to the **working tree** and restores
    between runs, so `git status` reads one to three dirty files while it runs and nothing may be
    committed during it (the same hazard #2215 names for the negative-control helper). What it prints is
    two different things, and item 14 of this file is the reason to keep them apart: a **per-set tally**,
    one line per set file (`scripts/check-mutations.py:1855`) — 13 files at this head, from
    `version.json`'s 2 to `web.json`'s 292 — and one closing **summary**, `summary_line()`
    (`scripts/check-mutations.py:553`), whose `all 847` is that tally's sum. Neither is the number any
    `make verify` line reports, because verify's set-file line counts *files*, not mutants (`0 problems
    in 10 tools and 13 set files`); quoting one as the other is how a set count gets inflated in prose.
20. **A base sync is owed again the moment this one lands, and the whole open queue is in that state.**
    This tick's merge took `main` `5aaa9a7`; the queue read above shows all four open pull requests
    `CONFLICTING`/`DIRTY` against that tip, so the queue is four conflict ticks, not one. Do not carry
    any of these numbers: re-measure `git merge-tree --write-tree HEAD origin/main` after a fresh fetch
    and quote its exit code. Whoever takes the next sync of *this* branch owes `make check-mutations`
    before the push for the reason item 19 states, and owes the added-line census on
    `factory/selftest.sh` in particular, because the anchor at base line 20590 is now the place two
    families of fixture sections both append at (item 22).
21. **A stale count in a Makefile comment, found while verifying item 19 and deliberately not repaired
    here.** `Makefile:74` "all 773 rows checked" prices a claim that is still true at this head — parsing
    all 13 set files, 0 of the 847 mutants name `scripts/` in their `file`, `old` or `new` field — but 773
    is how many rows the sets held when that sentence was written, not how many they hold now. That is the
    family the open issue about prose figures stated "at this head" with no gate able to read them
    describes, and it is left for the unit that owns the file, because a Makefile line moves decision-log
    citations this round does not own.

22. **The citation gate reads rc 0 for the whole-side order that deletes a parent's delivery, and no
    leg runs the census that sees it.** Measured this tick in two shared clones: taking `origin/main`
    for the four conflicted files refuses at rc 1 with 20 named drifted citations, while taking this
    branch's side reads rc 0 — 288 entries, 750 citations, every anchor open — because a self-consistent
    log has nothing to complain about, and that same order deletes 599 digit-normalized lines of main's
    work -- 84 `factory/DECISIONS.md`, 244 `factory/selftest.sh`, 3 `factory/factory.sh`, 268 `NEXT.md` --
    plus three of its decision entries. The only instrument that sees it is the added-line census
    run by hand against both parents. `H-0217`'s parent-relative instrument (#381) is the home for it: a
    gate leg that compares a merge result against BOTH parent blobs, not only the log's own anchors.
    Item 1 above is the same instrument's other half and #381 owns both. The `issue-346` unit measured the
    complementary arm, and the two together are the specification a leg needs: the gate **does** refuse a
    resolved-but-unrepaired log -- re-run in a scratch tree from the conflicted backup it refused at rc 1
    naming exactly 20 drifted citations -- and reads rc 0 on the shipped bytes; and 7 of those 20 refusals
    print two, three or four candidate lines, three of them the shared phrase `our two entries at`, which
    sits on four rows of the log. So a gate leg needs the parent-relative row-identity step to repair, and
    the added-line census to notice the order that never raised a hand.
23. **Seven `## H-` headings in `factory/DECISIONS.md` have no blank line above them, so they are prose
    to a renderer — carried from the `issue-346` hand-off and re-measured on this merged tree.** Rows
    2315, 3737, 4907, 6936, 8670, 8998 and 9015, the same seven main measured; 8 in base and 8 in each
    parent, the eighth being the `H-0201` junction that main fixed and this merge re-took under
    `H-0200`. `scripts/check-decisions.py` reads raw lines and cannot see the defect. A fix edits seven
    other units' rows and would cascade pointers of its own, which is why this unit left it.
24. **A sibling branch in flight burns a decision id that `origin/main` cannot see — re-measured
    twice now.** `H-0225` and `H-0227` sat on `refs/heads/issue-348` while `main` had neither,
    which is why the `issue-346` sync took `H-0226`; this tick read the allocator three times and
    it answered `H-0228` on all three -- `REFRESH` 30 s before the merge, `REFRESH-BLIND` at 424 s
    mid-merge, `REFRESH` 0 s after `git fetch --prune origin` -- and **each read journalled its own
    `DECISION-ID H-0228` line, the blind one included**, so a stale read is not inert: it stamps
    the journal as though it were a verdict. Its own advisory is the rule to follow -- "so H-0228
    is free only as far as this clone can see. Refresh, then re-read before the entry is committed"
    -- so the read an entry rests on is the post-fetch one. A unit that guesses the next number, or
    that trusts a blind read, collides with a sibling.
25. **#416 aims at the main-checkout restore arm and this unit did not re-run its measurement** (carried
    from the `issue-346` hand-off). The restore arm restores a tracked-and-missing file every
    activation, so if the tool that deletes `.cortexkit/.gitignore` runs again each time, the pass
    rewrites it each time — churn, not repair. Read #416 before treating `MAIN-CHECKOUT-RESTORED` as a
    clean verdict.
26. **A hand-off can cite a section that no ref ever held, and no leg sees it — re-measured on this
    merged tree** (carried from the `issue-346` hand-off). Its item 12 points at `docs/factory.md`,
    *What a conflict-resolution commit is allowed to contain*: `grep -rn 'conflict-resolution commit'
    docs/` answers **rc 1, no hits** and `git log --all -S 'allowed to contain' -- docs/factory.md`
    answers **0 lines**, so the section never existed anywhere — not a rename. Mechanism:
    `scripts/check-next-md.sh:182` `for rel in NEXT.md .opencode/skills/factory/SKILL.md AGENTS.md; do`
    (**re-measured**) tests whether a **path** is missing; a path that exists with a phantom heading
    inside it passes. The carried rule's substance is real; its in-tree support is the AGENTS.md citation
    discipline, not that title. Either write the section or re-point the sentence — do not carry the
    coordinate on.
27. **A control test can be vacuous in this family and say nothing** (carried from the `issue-346`
    hand-off). A mutant built from a tree that lacks the fixture being controlled reads green for the
    wrong reason — the general form of item 15. This tick's guard was ordering: run the union first,
    then the mutants, then check that the mutant actually contains the leg it claims to control.
28. **`make verify` reads RED on a resolved-but-unstaged merge, and the reason it prints is false.**
    Measured this tick: with the four conflicted files resolved in the working tree and not yet staged,
    `factory/selftest.sh`'s `the timer's scripts are executable` leg refused with
    `not 100755 in the index: factory.sh(100755 100755 100755) selftest.sh(...)`, while
    `git ls-files -s` shows every stage entry of both files at 100755. Mechanism:
    `factory/selftest.sh:17917` `index_exec_problems() { # <repo> <name>... -> the names git does not
    hold at 100755` collects one mode per **index entry**, and an unmerged path has three (stages 1/2/3),
    so `factory/selftest.sh:17921` `[ "$mode" = "100755" ] || bad="$bad $f(${mode:-untracked})"` compares
    a three-line string and refuses a mode that is correct. Every conflict tick meets this before it can
    stage, and the refusal names no stage, so it reads like a real permissions defect. Repair shape:
    dedupe the collected modes and refuse only a distinct value that is not 100755, or skip paths git
    reports unmerged and say which were skipped. This unit changed no leg -- the workaround is to stage
    the resolutions and re-run, which is what the verdict quoted in the pull-request body was taken on.

## Non-blocking review items carried forward (PR #402's review at 2026-11-15T21:57:01Z on head `5408b41`; the reviewer marked all three NOT merge blockers; #402 merged 06:32Z and none of the three has been taken)

1. **A bare `-` bullet slips the empty-marker leg.** `scripts/check-next-md.sh` requires a space or tab
   after the marker, so a lone `-` line matches neither branch, and the fixture plants `- ` *with* the
   space. Measured by the reviewer: rc 0 for a shipped bare-dash bullet. **Re-measured on this merged
   tree, still open:** the pattern is `scripts/check-next-md.sh:159`
   `if (line ~ /^[ \t]*[-*][ \t]/ || line ~`, and the only empty-marker fixture writes the marker with a
   space (`scripts/check-next-md.sh:376`). Repair: widen the pattern and add a bare-dash fixture.
2. **The scanned-file list is unpinned.** The phantom-path leg reads the hand-off, the skill and
   `AGENTS.md`, but every fixture plants only in the hand-off — a future PR could narrow the list with
   the suite green. **Re-measured:** the list is `scripts/check-next-md.sh:182`
   `for rel in NEXT.md .opencode/skills/factory/SKILL.md AGENTS.md; do` and the sole phantom fixture
   appends to `$SELFTEST_BOX/NEXT.md` at `scripts/check-next-md.sh:381`. Repair: one fixture that plants
   in `AGENTS.md`. Item 26 of the owed list is the same gap aimed at a section title rather than a path.
3. **The shape leg early-returns after its first refusal**, so a hand-off with two shape defects reports
   one per run. **Re-measured:** both legs end their refusal block with
   `scripts/check-next-md.sh:172` `		return 0` and the same at the phantom leg's close. Accepted as a
   first cut; recorded so the next unit does not mistake the behaviour for a bug.

## Hard rules for whoever picks this up

- Checkpoint before the judgement call, then one effect commit carrying the `Decision:` trailer — and
  `factory/next-decision-id.sh factory/DECISIONS.md` re-run after a fresh `git fetch` before taking an
  id. Both answers get journalled (`REFRESH-BLIND`, then `REFRESH`); a blind answer must not be the one
  you use, and a gap the allocator skips stays the allocator's to skip. **Except on a branch whose base
  predates #407:** the journalling arm is not in that tree — `git show f2ca7e1:factory/next-decision-id.sh
  | grep -c 'DECISION-ID'` answers 0, and `grep -rh 'unit=conflict:410'` over `$FACTORY_LOG` yields zero
  `DECISION-ID` lines for this tick — so on such a branch the read survives only in the tick transcript
  (`/home/rodney/prj/parental-lockout/factory/log/20261116T204146Z.conflict:410.log:252`) and quoting the transcript line is the honest
  provenance, not an invented journal line. The merged tree carries the arm.
- `Chose:` must quote an option **verbatim** or `check-decisions.py` refuses the entry — re-measured at
  this tick: `H-0225`'s paraphrased `Chose:` cost exactly one gate run. And an option line must not
  contain `|`, because the checker splits `Options:` on it.
- Every `path:line` you write needs its quoted anchor **on the same physical line**, and only at the new
  coordinate, or `POINTER_DEBT` rises and the gate refuses. Verified at this head: five anchored
  citations added, 741 read, 546 owing an anchor, 0 unopenable.
- A decision-log union is keyed by id and mechanical: splice the ascending union, then prove it — numstat
  against each parent blob, the added-lines census, the marker count, the additive line census on every
  auto-merged file (`common.sh` 3181+171+1093=4445, `selftest.sh` 16322+219+4285=20826, `factory.sh`
  1683+11+153=1847, `docs/factory.md` 2128+0+150=2278, `AGENTS.md` 72+6+0=78), one wrong order actually
  run so the refusal is measured, and a parent-relative citation census for the drift the gate cannot
  read. Check first whether the fresh ids **interleave**: if they do, both whole-side orders are refused
  and the union splits one side's tail. Run **both** orders and read the added-line census afterwards,
  because the gate refuses only one of them (item 22). Where both sides appended at the same anchor,
  check whether the diff aligned a shared closer as context before deleting the markers, and check
  whether the base already had a defect at the junction that one parent fixed — keep the fix, do not
  re-point the pointer it protects. Check a shared entry's digit rows by opening its pointers on each
  parent: the newer side wins, and the sweep re-moves it anyway.
- Restore mutants with `cp` from a backup, never `git checkout --` (#2215), and run probes on
  `git archive` copies or files under `/tmp/opencode`, never the gated tree. `rm -rf` is refused here, so
  use fresh unique directory names.
- Run the gate as CI runs it: `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make
  verify` until #425 is repaired, and say so wherever the verdict is quoted.
- **A base sync also runs `make check-mutations` before it pushes.** `verify` never *applies* the mutant
  sets — `Makefile:48` "kept OUT of verify for the opposite reason" states that as a cost decision, and
  verify's `check-scripts.py` step reads their bytes for encoding and nothing more (measured: one
  non-UTF-8 byte appended to a set file stops `make verify` at `==> script and set-file encoding
  guards`, rc 2) — so an anchor a sync moved off is caught only by `make check-mutations`, which CI runs
  as `mutation sets`, and the sync can read green locally. Nothing is committed while that run is in
  flight: it applies each mutant to the working tree and restores between runs, so `git status` reads one
  to three dirty files on purpose (#2215 is the same hazard). Measured at head `e8ab8b9`: the local run
  prints `all 847 mutants apply and compile`, the 847 being that per-set tally's own sum (13 files at
  this head, from `version.json`'s 2 to `web.json`'s 292), and CI's job on that head passed in 16m19s.
- Never commit the built `plock` binary; `git add -A` has swept it four times. Check
  `git ls-files | grep -x plock` before every `git add -A`, and read the pushed file list afterwards.
- No `gh pr edit` on this account (it dies on the retired `projectCards` field and writes nothing, and it
  is what kills `--add-reviewer`): the working verbs are `gh issue edit N --body-file`,
  `gh issue comment N --body-file`, `gh pr comment N --body-file`. Reading a body back is raw JSON plus a
  byte-diff, not a `-q` projection (#1180).
- Nothing that ships may put an inflection of "close" next to an issue number it does not finish (#1256),
  and the check runs against the **read-back**, not the draft.
- Merging, publishing, labelling, dependency changes and deletions are withheld from a unit; the
  supervisor merges what it re-reads as approved, mergeable, clean and green. A unit that thinks a
  reviewer ask is going to the wrong person says so on the pull request and names
  `FACTORY_REVIEWER_TEAM`, and leaves the timer's environment alone.
- Before editing a file either log cites, run `python3 scripts/check-decisions.py --citations <path>`
  and fix what it lists in the same commit. Two ways to fail the anchor rule: a quote wrapped onto the
  next physical line counts as bare, and citing a line number from a *mutated* tree is a pointer to
  nothing — quote refused orders, never cite them.
- Repair a merge's drifted citations by **row identity, never by a delta**: find the carrying row
  verbatim in the parent blob whose copy it is, open the cited row there, then find that cited row again
  in the merged file and move the number to it. Assert each repair is digits-only (identical with digits
  stripped) and re-open every anchor afterwards. Both alternatives in this family — picking a side, or
  adding a measured offset — silently keep one parent's stale numbers. When the checker lists two
  candidate lines, resolve by opening the blob the citing entry shipped at, and cite that act in the
  entry (`H-0220`'s method).
- When both hand-offs state the same rule in different words the union carries **one** wording, and
the added-line census then reports the other as lost -- a wording ledger, not a fact ledger. Read it per
item: this unit compared all 23 of main's owed items against this branch's 21 by sentence and restored
the six facts that were genuinely main's alone (the `H-0188` footprint trajectory, `#296`'s "a unit
overwriting the file, not a merge resolution", the must-not-re-point three, the 0.572 s fetch arm, the
section-vs-file sibling, the rc-1-with-20-named control). Calling the census clean on a rewritten
hand-off is a false verdict; quoting "262 lines lost" without naming which are facts is the same
failure wearing a number.
- A mutant needs the repository to be judged honestly: build it with `git clone --shared` and copy the
  tracked files over the clone, because a bare file copy has no object database and `check-decisions.py`
  then spends its whole refusal on `checkpoint ... is not a commit in this repository` — measured this
  tick, 285 such lines drowning a verdict. `rm -rf` is refused here, so fresh unique directory names.
- Overwrite this file carrying every still-true line you did not write, **including the ones your branch
  never authored**: this tick found six owed items and four hard rules in `main`'s copy that this
  branch's copy had never carried. The shipped *narrative* of another unit's hand-off is not an owed
  item and stays on the branch whose blob carries it.
  coordinates in it before you cite them (item 10).
