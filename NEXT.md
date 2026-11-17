# NEXT — issue-334 unit (PR #431, dispatched `conflict:431`)

State at push: branch `issue-334`, PR #431, the **seventh** base sync in this family and the first on this
branch, recorded as `Decision: H-0237` (checkpoint `0da656f`, an empty commit cut before the merge
started). It takes `origin/main` `cf00b44` — PR #427, the `H-0235` merge — on merge base `6e069eb`, and
`git merge-tree --write-tree --name-only refs/heads/issue-334 cf00b44` named **two** conflicted paths:
`factory/DECISIONS.md` and `factory/selftest.sh`, each a both-append at one shared seam. The pre-merge
read of the artifact was `OPEN`, non-draft, `headRefOid 8b01175`, `mergeable CONFLICTING`,
`mergeStateStatus DIRTY`, `gh pr view 431 --json commits` = **9**; `reviewDecision` read empty, so the
approval is the supervisor's read and this unit asked no one. This is the sync the hand-off this unit
replaced called owed the moment the sixth landed (its item 20).

`H-0237` is journalled: `2026-11-17T06:34:06Z DECISION-ID H-0237 unit=conflict:431 branch=issue-334
(allocator, factory/DECISIONS.md, REFRESH 51s union)` in the operator's `factory/log/supervisor.log`. The
id this sync did **not** take is the half that matters: `H-0236` is in neither decision log at this head
(`grep -c '^## H-0236'` answers **0** against `HEAD` and **0** against `origin/main`) and answers **1**
against each of `refs/heads/issue-341` and `refs/remotes/origin/issue-341`, with two allocator grants for
`unit=unclaimed:341` at `05:49:36Z` and `06:03:08Z`. That is item 24's sibling-branch class, re-measured on
this box at this tick rather than inherited: a sync whose union read were `HEAD` plus `origin/main` would
have taken `H-0236` and collided the moment `#414`'s successor landed.

Everything below is this unit's hand-off plus every still-true line of the one it replaces. Trunk's copy
(`origin/main` at `cf00b44`) arrived **whole**: outside the head block, the two sections this unit owns,
and the rows added below, this file is byte-identical to it — every trunk row removed by this write is
inside trunk's own rows 1..151, its head, ship narrative and did-not-deliver block — so its 901 added rows
are carried, not re-summarised, and its 51 owed items are carried unchanged and unrenumbered. The previous
unit's ship narrative and its "For the sixth sync specifically" disclosure are superseded as narratives,
which is what this file has done since `H-0233`; they read at `git show cf00b44:NEXT.md`, and re-measurement
row 7 below names where each of their live facts now lives. Coordinates marked **re-measured** were opened
on this merged tree; the rest are carried in their authors' words and were not re-run. `make verify`'s and
`factory/selftest.sh`'s verdicts are quoted in the pull-request body from the run that ships these bytes,
naming the exact `FACTORY_*` sanitization in force, never from here (#1180).

## What this unit shipped

- **The seventh base sync, for PR #431** — `Decision: H-0237`, checkpoint `0da656f` cut before the first
  write, one merge commit taking `origin/main` `cf00b44` into this branch at `8b01175`. Two conflicted
  paths, both a both-append at one seam, both resolved as a union that keeps both parents:
  `factory/DECISIONS.md` by ascending id order, `factory/selftest.sh` by putting trunk's `#348`
  worktree-census legs and this branch's `#334` retention-contract legs side by side ahead of the shared
  closer.
- **Proof the union lost nothing, measured on the shipped bytes and re-runnable.** Per-path added-line
  census — every row either side added against merge base `6e069eb`, required present in the merged bytes:
  `docs/factory.md` ours +99 / theirs +0, `factory/DECISIONS.md` +28 / +189 raw (+153 folded), `factory/selftest.sh`
  +317 / +219, `AGENTS.md` 0 / +6, `NEXT.md` 0 / +901, `factory/common.sh` 0 / +171, `factory/factory.sh` 0 / +12,
  `scripts/check-decisions.py` 0 / +10 raw (+9 folded) — with **0 rows of trunk's absent, raw or
  folded**. Ours come back
  **9 absent raw, 2 absent folded**, and those 9 are exactly the log rows this sync re-pointed: 7 differ
  only in digits, 2 also carry new anchor text. Structure: `factory/DECISIONS.md` 12300 → **12632** rows
  (**12479** at the pure union; +153 = the blank separator plus the 152-row entry), `^## ` **224 / 226 / 230 / 233** and `^## H-` **223 /
  225 / 229 / 232** across base / ours / trunk / merged; `6e069eb`'s log is a **strict prefix** of
  `8b01175`'s (this branch edited **0** shared rows), so keeping trunk's shared region cannot drop an edit
  of ours; and merged's first **12451** rows are trunk's whole file verbatim except row **3113**, which
  differs only in digits (the `H-0096` re-point below). `factory/selftest.sh` 21191 → **21727**, proved by
  whole-list equality `merged == base[:21173] + trunk[21173:21392] + ours[21173:]`, one verdict rather than
  two (`^reached_verdict=1$` occurs exactly **1** time in base, ours, trunk and merged; the
  `reached_verdict=` marker **4** times in all four), `bash -n` clean.
- **27 drifted anchors repaired by row identity, and the offset method measured out rather than argued.**
  The pure union needs no memory to rebuild — `6e069eb` is a prefix of `8b01175`, so it *is* trunk's file
  plus those 28 appended rows — and the gate's own `classify` over it reads **852 citations, 364 owing an
  anchor, 0 unresolvable, 27 anchored pointers whose quoted row is no longer at the cited row**. Shipped:
  **0** broken, `364` owing an anchor in this log **unchanged**, and the two-log read at **865 citations,
  546 owing an anchor, 0 unopenable** with `POINTER_DEBT` held at 546 (never lowered; this entry's two
  citations both anchored). What the arithmetic can and cannot do, measured on those 27: **23 sit at exactly
  their file's uniform trunk growth** (`+171` ×8 in `factory/common.sh`, `+219` ×15 in
  `factory/selftest.sh`), and the **4 that do not are exactly the 4 whose quoted fragment occurs on more
  than one row** — `"$d/earned-tail.sh"` on 5 rows (3 pointers) and `"$d/earned-window.sh"` on 2 rows (1) —
  and on all 4 the offset row *contains* the fragment, one row above the refusal the entry measured:
  `factory/selftest.sh:21638`, `:21647` and `:21664` are the `dpn_mut`/`grep -qF` guards, while the entry
  means the `control failed:` rows `:21639` and `:21648`. So a shared fragment is not a rare wart here: it
  is the whole difference between an offset that lands and an offset that passes on the wrong statement,
  23/23 and 4/4 — item 52.
- **Item 35's bare-pointer census, re-run at this head** (**re-measured**, method unchanged): **546** bare
  citations, **533** whose carrying row is verbatim at `6e069eb` so the number written is base's number,
  **521** still landing on base's row with digits folded, **3** that do not. One is repaired here:
  `H-0096`'s `scripts/check-decisions.py:679-682`, which trunk's 9-row growth onto that file dropped onto a
  **blank** row, re-pointed by unique 4-row block identity to `:688-691` (that block occurs exactly once).
  Two stay wrong and are the pair item 35 already names — `H-0099`'s and `H-0107`'s `factory/common.sh:1607`,
  whose base row folds to a bare `#` and now has **192** candidate rows folding the same way (their count
  was 189), so no identity method can choose. The leg that runs this on every merge is still unwritten.
- **A probe hazard this unit paid for and named:** four numbers reached this ticket through `;`-joined
  probes and none survived a single-command re-run — `0e1b0c6` (`git cat-file -t`: *Not a valid object
  name*), a "pushed head" `350e688` that is `chore: merge origin/main into issue-361 …` on another branch, a
  "merge commit" `4e19c12` that is this branch's own `test(factory): pin the two journal readers a bound
  must not disarm (#334)`, and NEXT.md rows supposedly citing a trunk sha that is in **neither** copy of
  this file. The rule is in the hard rules now: one command per call, and read the name of the ref you
  asked for before quoting its value.
- **Withheld, as the queue says:** nothing merged, published, labelled or deleted; no reviewer asked
  (`reviewDecision` empty at both reads, and the reviewer ask is the supervisor's poke); no machine-level
  config touched; `factory/HALT` absent and `plock` untracked in base, both parents and the merged tree.

## What this unit did not deliver

- **Nothing on the merged tree was re-run for behaviour.** The added-line census, the heading counts, the
  whole-list equality and `bash -n` are presence and parse checks. Trunk's `H-0232`..`H-0235` legs and this
  branch's own `#334` retention legs were not executed by this unit; `make verify` and
  `make check-mutations` on these bytes are the first runs and their verdicts ship in the pull-request body.
- **Trunk's fresh entries are carried as their author's numbers.** `H-0233`, `H-0234` and `H-0235` —
  including `H-0233`'s 822-versus-774 verdict count — were not re-derived here. Re-measure before acting on
  one.
- **It repaired pointers; it did not gate them.** The bare-pointer instrument is still a script someone ran
  by hand (items 5, 35 and 51), the shared-fragment rule this sync's own evidence asks for is unwritten and
  costs 59 citations to land the strict version (item 52), and `scripts/check-next-md.sh` still tests a
  cited path for existence and never for its row (item 51).
- **The two carried limitations stand unchanged.** `factory_main_tree_check` still reads one tree (#342 ask
  5 made it narrower than the pass's own reach) and the census is still report-only in fact: `H-0197`'s
  *What lost* (three race arms, no planted leg) and `H-0225`'s (the prune arm has never run live) are both
  still true, and item 17 is still the live instance.
- **It pushed only this branch's own ref.** It ran no fixture that appends to the operator's
  `factory/log/supervisor.log`; what this tick wrote there is the allocator's `DECISION-ID` rows, which is
  what the allocator is for.

## Queue state the fifth tick read (16 Nov, ~23:55Z — stale as of the sixth sync)

- `gh pr list --state open --limit 500 --json number,headRefName,reviewDecision,mergeable,mergeStateStatus,isDraft`
  → **3** open PRs, **0** drafts: #410 `APPROVED`/`CONFLICTING`/`DIRTY`, #413 no review decision and
  `MERGEABLE`/`BLOCKED`, #414 `CHANGES_REQUESTED`/`CONFLICTING`/`DIRTY`. `gh issue list --state open
  --limit 500 --json number` → **54** open issues. Both counts came from raw JSON counted in python,
  not a `-q` template: a bare generator inside an interpolated string drops rows at rc 0 (#1180,
  measured 14 Nov). The previous hand-off counted 4 open with all four conflicting; #411 merged
  since — that is what this sync takes — and #413 is the one survivor that now reads mergeable.
- `main`'s tip is `1c385b7` (PR #411, `issue-355`: `D-0084`, `D-0085`, `H-0202`, `H-0229`, and
  `H-0230` is held by `refs/heads/issue-361`/`refs/remotes/origin/issue-361` at `350e688`, re-read
  over all 72 non-main refs — which is why this entry is `H-0231`).
- This branch's own state before the write: #410 `OPEN`/`APPROVED`/`CONFLICTING`/`DIRTY` at head
  `1bc9a0e` (this sync's checkpoint), 9 commits, merge base `5aaa9a7`, `reviewRequests` **empty** —
  so the reviewer ask is the supervisor's poke and this unit posts no blocker. **Re-measured:** its
  `statusCheckRollup` now returns **5** rows, all `COMPLETED`/`SUCCESS` (`verify`, `shellcheck`,
  `mutation sets`, `packages (amd64)`, `packages (arm64)`); the previous tick saw **zero** rows, so a
  rollup does recompute once a head exists to recompute on — read it again before concluding it is
  GitHub not having recomputed. The push spends that approval (item 9).

## The census's first live run — measured, read-only, at the fifth tick

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

   Their instance, carried because it is a second sighting of the same ceiling: in the `issue-355`
   tree `git fetch --prune origin` hit the tool's own 120 s ceiling once and returned rc 0 in well
   under a second on the retry, moving no ref — a fast fetch is not evidence the ceiling was wrong.
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
   `docs/factory.md:651` "### The approval a push voids" — **re-stamped by this sync**: the row
   carried `:652`, an off-by-one that is false on both parents of this merge as well (the heading is
   at 651 on `HEAD`, on `origin/main` and on the merged tree, where the file is byte-identical to
   both), so it is a pre-existing pointer error, not a drift this merge introduced: a base-sync push
   turns `APPROVED` into a
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

   Their instance, carried: the `issue-355` unit found **both** proposed causes unsupported on its
   own tree — the carried `FACTORY_*`-inheritance cause was not reproduced there, and neither was
   the review's substitute — so the question stays open even though this branch measured a real
   RED on its own shell. Do not close #425 on this branch's measurement alone.
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

   Their id-gap census, carried and **re-measured on this merged tree**: 224 `## H-` rows,
   `H-0001`..`H-0229`, missing `H-0031`, `H-0032`, `H-0057`, `H-0122`, `H-0203`. `H-0225` is no
   longer a gap — this branch carried it, so the merge resolved half of their finding — and `H-0203`
   is the only recent one: it has a `## H-0203` heading on both `refs/heads/issue-361` and
   `refs/remotes/origin/issue-361`, whose tip is now `350e688`, not the `63bcf84` they measured, so
   the branch has moved since their read. The other four are older rows named in prose (`H-0057`
   fourteen times) that never had a heading. Both facts still show the allocator's blind half in
   concrete form: a ref-held id is invisible to a clone that has not refreshed.
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

29. **A repair helper that re-reads its own path inside `open(path, "w")` truncates the log.**
   Measured here: the first version did exactly that and both decision logs were written down to one
   line. Nothing was lost only because the union had been staged first, so the index blob was a whole
   restore point (`git checkout-index -f -- <log>` returned both byte-identically). No leg covers it.
   Rule to carry: stage the resolution, then edit; and read once, write once.

   This unit kept both restore points the rule asks for: the spliced log was `git add`ed
   before any later pass edited it, and a `cp` backup was taken before the pointer repair ran,
   so the census in item 35 had a whole pre-repair log to compare against. The repair pass
   itself reads once and writes once, from an in-memory list.

30. **Gate-blind pointers are empty at this head, not by design.** All 34 citations of
   `factory/DECISIONS.md` happened to be anchored. The next bare `path:line` under the splice moves
   invisibly, and 539 bare pointers across the two logs are already outside the checker's reach. #428
   and #381 own that class.

   **Re-measured on this merged tree, and the premise no longer holds:** 51 citations now aim
   at `factory/DECISIONS.md` — 31 anchored, 11 bare and non-inheriting, 9 inheriting their file
   or ranged. That is 20 gate-blind pointers into the very file this family splices, not an
   empty set, and item 35 is the count of what that blindness cost at this head.

31. **`origin/main`'s `DECISIONS.md` at `5aaa9a7` ends without a final newline** (**re-measured**: last
   byte `.`), so the next EOF append on any branch lands against that line; this merge happened to
   restore the newline because ours' blob ended with one. Nothing in the tree requires a log to end
   with a newline, so nothing stops the next drop.

   **Re-measured here:** both logs end with a newline on this merged tree (`tail -c1` reads
   `\n` at the end of `factory/DECISIONS.md` and of `DECISIONS.md`), because this branch's blobs
   ended with one. That is luck, not a rule — nothing in the tree requires it yet, so the
   next EOF append still lands against whatever the last unit's editor happened to do.

32. #355's own claims are untested against main's `serve`-stop rewrite: `D-0085`'s single-clock join
    and `H-0202`'s shape-guard were measured on this branch's own head. The gate is green over the
    merged tree, which is the only new evidence.

33. Two open branches still re-conflict on these same two logs at every merge (#428, #381): this
    sync's whole conflict surface is the two decision-log tails and nothing else. Do not quote a
    count of "syncs this hour" for it — a review challenged the phrase and no timestamped census of
    merge units exists in this tree to back any such number.

34. **Two reviews on this pull request, at 14:33:30Z and 14:47:41Z, name artifacts this repository has never contained, and every one of their load-bearing claims was checked before anything was changed here.** Absent across all refs (`git log --all` finds 0 commits adding them): the two python test files the 14:33 review names, test_all_day_boundary and
       test_doctor_schedule_window, both said to live under the testing tree, plus the symbols `allDayNotStarted` and `stillLocked`, the report strings `all day, not started` / `still ends at` / `still locked until`, and any `test_*.py` under `testing/` (the tree holds `README.md`, `capture/`, `e2e/`), so `python3 -m unittest discover -s testing` collects 0 tests, not 34. The line they cite, `cmd/plock/doctor_checks.go:458`, is `if !a.InWindow {` here and `\t\t\tcontinue` at `4fbb051`; `doctor.go:141` is the `--try`/`--json` refusal, and the only other clock reads in that file are the report stamp at 155 and the TLS check at 186, neither of which evaluates a schedule. Two sentences quoted as this body's have never been in this body. Commits `d0c94d4` and `4118646` do not exist here. What the reviews asked for already ships on this branch: the fixture says `allDay` with both stamps at `00:00:00Z` and `scripts/check-fixture-window.sh` refuses the `00:00`/`23:59` pair (`4fbb051`), and the guard runs at `Makefile:152`, BEFORE vet/build/race. If a future unit is told to fix the all-day union, ask for the ref first — the second review's own instrument confirmed this sync independently (clean merge, ids unique and ascending, both gates rc 0, its full run 3365 passed 0 failed), so the reviewer can read this tree; it is the findings, not the tree, that are elsewhere.

35. **A bare pointer into a file the merge shifted moves invisibly; a census against the
    base blob counts them.** Measured on these bytes: 382 bare `path:line` citations whose carrying
    physical row is verbatim at `5aaa9a7` — so the number written is base's number — and 369 of them
    still land on the same row with digits folded. **4 do not**, all aimed at `factory/common.sh`
    because main grew it: `H-0099`'s `:1607`, `H-0107`'s `:1649` and `:1607`, `H-0148`'s `:2614`. Two
    placed by unique row identity and were repaired (`:1649`→`:1820`, `:2614`→`:2785`); two are
    unplaceable, because base's row there folds to a bare `#` and the merged `factory/common.sh`
    holds 189 copies, so no identity method can choose and they stay wrong — a reader following
    `H-0099`'s `common.sh:1607` lands on `dirt_rc=`, not on the line the entry means. The method is
    a python pass comparing `git show <base>:<file>` with the working file, digits folded, scoped to
    citations whose carrying row is verbatim at the base blob. It needs no anchor, which is exactly
    why the anchor rule cannot do it, and it is cheap: 382 rows in a second. A leg that runs it on
    every merge is owed next to item 5's; until then this is a floor, because a bare citation whose
    carrying row either parent edited is excluded here as provenance-unclear (5 such rows at this
    head before the repairs, 8 after, since re-pointing a number makes its carrying row newer than
    base).

36. **Carried from `main`'s hand-off, row 3, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- one leg flaked once and was never named:
   **One leg flaked once and was never named.** The first `make verify` at `fec929f` reported
   `682 passed, 1 failed` while the new leg printed `ok`; the same tree under output capture and two
   later full runs reported 0 failures. The FAIL line was never captured, so the leg is unidentified.
   Anyone who sees a single-leg red inside `make verify` that standalone does not reproduce should
   capture to a file (`make verify > log 2>&1`) before re-running.

37. **Carried from `main`'s hand-off, row 5, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- the suite's tally does not count every line it prints:
   **The suite's tally does not count every line it prints.** **Re-measured on three runs:** 822
   printed verdict lines against 774 counted at this head clean, 822 against 775 with the env intact,
   821 against 774 at `main`. The delta is control lines that print `ok` without incrementing. No leg
   is missing — the label sets are identical across runs — but a reader who compares tallies across
   environments will think a leg appeared or vanished. Pin the tally, or count printed labels.

38. **Carried from `main`'s hand-off, row 6, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- `make verify` at `67c05e3` with the env unset was never run:
   **`make verify` at `67c05e3` with the env unset is not yet run.** Run it before calling the head
   verified; quote what it printed, not what it should have said.

39. **Carried from `main`'s hand-off, row 7, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- the journal is still polluted, and the #414 run added to it while proving so:
   **The journal is still polluted, and this unit added to it while proving so.** The 604 `LOG-MOVED`
   lines the entry counts are still there, and the control run at `main` appended three more
   (9458→9461, timestamped 01:57:01Z/01:57:03Z, naming that run's own deleted `/tmp` fixture root),
   because unpatched trees still do the write. Tidying the file is the operator's call — it is the
   operator's record, and truncation is a deletion.

40. **Carried from `main`'s hand-off, row 8, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- the reviewer re-ask is owed and is not a unit's verb:
   **The reviewer re-ask is owed and is not a unit's verb.** `reviewRequests` is empty on #414 at this
   head, so the ask has not gone out since the push; the poke is the supervisor's (`factory_request_review`
   in `factory/factory.sh`, addressed to `FACTORY_REVIEWER_TEAM`), and `gh pr edit --add-reviewer` exits
   1 on this account's retired `projectCards` field. This unit wrote nothing to GitHub beyond its own
   branch and pull-request text.

41. **Carried from `main`'s hand-off, row 11, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- the instrument that repairs merge-moved pointers still has no gate leg:
   **The instrument that repairs merge-moved pointers still has no gate leg** (#381 owns the price).
   `scripts/check-decisions.py` refuses drifted *anchored* citations but only locates them, so a repair
   must resolve by parent-blob row identity. This unit's five re-stamps were done by hand, opened one
   by one on the merged tree.

42. **Carried from `main`'s hand-off, row 20, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- bare pointers are the debt:
   **Bare pointers are the debt**: repair them with a quoted anchor from the named line (`H-0148`'s
   shape), and take `H-0169`'s three rows first because its own text makes the drift a defect.

43. **Carried from `main`'s hand-off, row 21, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- a checkpoint commit message claimed a count the merge did not measure:
   A checkpoint commit message once said "eight fresh entries" where the merge measured six;
   `ed1954e` is empty and cannot be re-worded without a rewrite, which is denied here.

44. **Carried from `main`'s hand-off, row 24, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- five claims one unit made about its own work did not survive re-derivation:
   **Five claims one unit made about its own work did not survive re-derivation**; the `TREE-CLEAN
   … cause=never-dirty` accusation was withdrawn and its substitute was also false. A coverage claim
   comes from legs and their asserted substrings, never from a redundancy comment.

45. **Carried from `main`'s hand-off, row 28, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- a sync that does not re-derive the trunk's fresh entries leaves them unverified:
   **This unit did not re-derive any of `main`'s fresh entries' measurements**, including the 126-line
   reseating cascade and the ambiguity count that came with it.

46. **Carried from `main`'s hand-off, row 36, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- the merged id sequence's gaps, and what they mean:
   The merged id sequence still shows `H-0203` and `H-0225` as gaps and both are explained by refs, not
   lost entries; a clone that has not fetched `issue-348`/`issue-361` sees two unexplained gaps. That
   is the allocator's `REFRESH-BLIND` risk in concrete form.

47. **Carried from `main`'s hand-off, row 37, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- reviews naming artifacts the repository never contained, on another pull request:
   **Two reviews on #411 named artifacts this repository has never contained** (phantom python test
   files under the testing tree, symbols and report strings no ref holds, commits unknown to this
   clone) and every load-bearing claim was checked before anything was changed there. Check the
   artifact exists before acting on a review that names one.

48. **Carried from `main`'s hand-off, row 38, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- `git log --grep='Decision: H-0204'` no longer finds the fixture's work:
   **`git log --grep='Decision: H-0204'` no longer finds this fixture's work.** `0924bff` keeps
   `Decision: H-0204` in its message and `2af486e` keeps `pre-H-0204` in its subject — rewriting a
   pushed branch is the denied force-push — and `main`'s own `H-0204` is a different decision (a base
   sync erratum). The mapping is in `H-0232`'s `Id:` line and the merge commit alone.

49. **Carried from `main`'s hand-off, row 39, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- isolation is asserted over writes, not over every read:
   **Isolation is asserted over writes, not over every read.** The leg guards the dispatch path; the
   other ask sites are not census-enforced, and the direct `journalctl --user -n 1` probe in
   `reap_leg_live` names no unit at all (**re-measured**: it is the only direct `journalctl` call in
   `factory/selftest.sh` that asks without `-u` — every named-unit ask comes from the shipped code the
   fixtures invoke, not from the script).

50. **Carried from `main`'s hand-off, row 40, in its author's words** (`origin/main` `6e069eb`, the `fix/journal-test-isolation` unit, `Decision: H-0232`; "this unit" there is that unit, and "row k" is their numbering, not this file's) -- an entry's measured counts are stamped, not live:
   **The entry's measured counts are stamped, not live**: "6 dispatch asks", "604 lines out of 7006",
   "26 pointers into `factory/common.sh`", "683 passed" belong to the tree they were taken on. The
   merged tree's equivalent numbers are in this file's header and in the merge commit message.

51. **A `path:line` pointer inside this file is tested for the file, never for the row — and the
    off-by-one that survived two hand-offs is the proof.** The sixth sync found two false coordinates
    in its own input, neither created by the merge: row 1490 of `factory/common.sh` naming
    `factory_worktree_lines`, which sits at `factory/common.sh:1547` "factory_worktree_lines() { # [main-checkout]",
    and the row numbered 652 of `docs/factory.md` naming the heading that is at
    `docs/factory.md:651` "### The approval a push voids" — false on `HEAD`, on `origin/main` and on
    this branch alike, so it was inherited twice. No gate can see either class: the anchor rule reads
    only the two decision logs, and the hand-off leg at `scripts/check-next-md.sh:179` "leg 2: phantom paths"
    asks whether a named file exists (`scripts/check-next-md.sh:189` "[ -e "$root/$p" ] || printf", the existence test the leg makes in full),
    never whether the numbered row still holds what the entry quotes. The owed
    change is a leg that opens the file, reads the named row and compares it against the quoted
    fragment already in the sentence — the same predicate `scripts/check-decisions.py` runs on the
    logs, pointed at this file. Until then every pointer in a hand-off is a promise the next sync has
    to re-derive by hand, which is what this sync spent its turns doing.

52. **A shared anchor fragment defeats an offset repair *silently* — the anchor rule passes on the wrong
    row.** Measured on this merge, not argued: of the **27** anchored pointers the pure union shipped with
    their quoted row moved, the **23** whose fragment occurs on exactly one row of the target file all sat at
    their file's uniform trunk growth (`+171` in `factory/common.sh`, `+219` in `factory/selftest.sh`), and
    the **4** that did not are exactly the 4 whose fragment occurs on more than one row — `"$d/earned-tail.sh"`
    on 5 rows, `"$d/earned-window.sh"` on 2. For all 4 the arithmetic row *passes* the anchor test while
    naming the wrong statement: `scripts`-level guards `factory/selftest.sh:21638`, `:21647`, `:21664`
    (`dpn_mut` and `grep -qF` calls that also mention the fragment) instead of the `control failed:` refusal
    the entry measured, `factory/selftest.sh:21639` and `:21648`, each `grep -c` answering **1**. Both shipped
    pointers are aimed at the refusal rows now. Two ways to land this, and the cost of each is measured at
    this head so nobody has to find it later. **(a) The cheap merge-scope leg:** when a merge adds rows to a
    file that a pointer cites, refuse the merge unless every pointer whose cited row lost its anchor is
    re-aimed at a row where the fragment is *unique*. Refuses **0** citations on these bytes and would have
    refused **27** at the pure union, i.e. exactly this sync's payload, and it never touches the debt number.
    **(b) The strict rule:** refuse any anchored citation whose fragment occurs on more than one row of the
    target file. That refuses **59 of the 261** anchored citations this head ships (38 of them pointing back
    into `factory/DECISIONS.md` itself) and is therefore a sweep of its own, not a base-sync change. Either
    way do not lower `POINTER_DEBT` to land it — the 546 bare pointers would re-classify and the ratchet may
    only fall, so the change carries its own measured before/after number in the same commit the way `H-0220`
    did. The predicate `check-decisions.py` already computes is the one to change: `classify` builds
    `now = [i + 1 for i, l in enumerate(body) if want in norm(l)]` and refuses only when the cited row is
    absent from `now`, so a fragment occurring 5 times is a pass today.

## Re-measurements the sixth sync took against the rows above

These four are `main`'s rows 1, 2, 4 and 12. They are placed here rather than written into
the author's own sentence, because a carried row that has been contradicted stays true to its
author and gets the contradiction beside it — deleting or quietly editing another unit's
measurement is how a queue loses the fact that someone checked.

1. **`main`'s row 1, on this merged tree (carried as item 36's subject).** The #414 unit
   reproduced #425's fixture defect at `origin/main` `db79d41` and at its own head: with the
   tick's `FACTORY_*` names inherited, `bash factory/selftest.sh` reads two legs red —
   `a bare run journals nothing` and `control: one note can suppress the other` — and green
   with them unset. Its stated mechanism is `dpn_alloc_run`'s bare arm forwarding
   `FACTORY_LOG` and nothing else, so the inherited `FACTORY_TICK_UNIT` survives into the
   plant-free run. **That remains the only reproduction; this sync did not reproduce it.** What
   this sync ran on the merged bytes is the sanitized half only: `make verify` rc 0 (`verify: OK`,
   1m55s, which contains `bash factory/selftest.sh`) and `make check-mutations` rc 0 (847 mutants,
   3m25s), both under `env -u FACTORY_LOG -u FACTORY_REVIEWER -u FACTORY_REVIEWER_TEAM
   -u FACTORY_TICK_BRANCH -u FACTORY_SERVICE_UNIT -u FACTORY_TICK_UNIT`, and the operator's journal
   read 9563 lines before the first run and 9563 after the second. The inherited-name leg was
   started once and killed before it reached the suite — `factory/selftest.sh` under the tick's
   identity, SIGTERM then a sweep, no survivor and the scratch tree untouched — and it was not
   re-run, because on this box that leg's write target is
   `FACTORY_LOG=/home/rodney/prj/parental-lockout/factory/log` and its unit identity is
   `FACTORY_SERVICE_UNIT=parental-lockout-factory.service`, the operator's live journal and live
   unit; item 39 is the row that says a leg may not add rows to a file it does not own, and reaching
   for a live unit is host-level work whoever owns the unit makes, not a dispatched branch. The row
   this correction replaces claimed both forms had been re-run and their verdicts quoted in the
   pull-request body; only the sanitized verdicts are quoted there, and the false sentence was
   written by this sync before the runs existed.
2. **`main`'s row 2 refutes this file's item 12 as written, and item 12 stands as its author
   wrote it.** Item 12 says `make verify` reads RED for any shell that exports its own
   dispatch env; the #414 unit measured `make verify` GREEN with the names intact at
   `fec929f` (`683 passed, 0 failed`, `verify: OK`) and found the two reds appear in the
   standalone suite, not the gate. It also refuted the review's substitute cause: `grep -rln
   FACTORY_STATE_DIR` returned 0 files, so neither theory may extend a sanitization list.
   Do not close #425 on any of these measurements alone — item 12's own closing paragraph
   already says that about its own run.
3. **`main`'s row 4 is this file's item 28, same defect, sharper instrument.** Their leg is
   `the timer's scripts are executable`, which reads `git ls-files -s`, so a conflicted path
   with three stage rows prints `not 100755 in the index`. Their rule is in the hard rules
   below: resolve the index before running any gate. This sync staged every resolution before
   its first gate run and says so where it quotes the verdict.
4. **`main`'s row 12's counts, and this tree's.** Their row reports 539 gate-blind bare
   pointers "by luck, not design"; this file's item 30 reports 34, and item 5 reports 546
   unanchored. Both were true on their own trees. What this sync re-measured is in the pull
   request body: the pointer census of the merged logs, and the count of citations the drift
   sweep had to move to stay true.

## Re-measurements the seventh sync took against the rows above

Nothing in this section edits another unit's sentence. Each row names the row it measured, says what this
merged tree answers, and leaves the author's words standing where they are.

1. **Item 24, on this merged tree (re-measured).** Their claim is that a sibling branch in flight burns an
   id `origin/main` cannot see. At this head `H-0236` is exactly that: absent from both decision logs
   (`grep -c '^## H-0236'` → **0** against `HEAD`, **0** against `origin/main`) and present once on each of
   `refs/heads/issue-341` and `refs/remotes/origin/issue-341`, with allocator grants journalled for
   `unit=unclaimed:341` at `05:49:36Z` and `06:03:08Z`. This unit's own read took `H-0237` because
   `factory/next-decision-id.sh` unions `refs/heads` and `refs/remotes`; the item stands, sharpened to: a
   read over `HEAD` + `origin/main` only is not an allocator read.
2. **Item 35, on this merged tree (re-measured).** Their census at `5aaa9a7` was 382 bare citations, 369
   landing, 4 not. At `6e069eb` against this tree the same pass over both logs answers **546 bare, 533 with
   the carrying row verbatim at base, 521 landing, 3 not**. The 3: the two `factory/common.sh:1607` rows they
   already report unplaceable (still unplaceable — 192 rows fold to a bare `#` now, against their 189), plus
   a **new** one this merge created, `H-0096`'s `scripts/check-decisions.py:679-682`, now a blank row,
   repaired here to `:688-691` by unique block identity. Their "it is cheap: 382 rows in a second" holds at
   546 rows in a few seconds.
3. **Item 5, on this merged tree (re-measured).** Still **546**, and the number is load-bearing in a new way:
   this sync added 2 citations and both carry anchors, so the ratchet passed with the debt unmoved. A merge
   that adds citations normally raises the number, and the temptation the ratchet exists to refuse is
   lowering it to keep the line green.
4. **Item 31, on this merged tree (re-measured).** Their finding was that `origin/main`'s
   `factory/DECISIONS.md` at `5aaa9a7` ends **without** a final newline. At `cf00b44` it ends **with** one,
   and that trailing blank is the separator the union needed: merged's first **12451** rows are trunk's whole
   file verbatim, with one exception (row 3113, re-pointed by digits only). A sync that assumed their shape
   and appended with no blank would have welded `## H-0234` onto trunk's last row and the parser would have
   swallowed it into the entry above.
5. **`H-0235`'s own hand-off list, discharged.** Its entry says the next sync over a trunk that grows either
   file must re-point its pointers. This sync did: of the 27 drifted anchors **14 are `H-0235`'s and 13 are
   `H-0234`'s**, and the gate reads **0** broken anchors at these bytes. Nothing in that pair needs
   re-pointing again on this tree.
6. **Item 22, on this merged tree (re-measured, same direction).** Their claim is that the citation gate
   reads rc 0 for a whole-side order that deletes a parent's delivery. Confirmed here with the strongest
   instrument available: taking trunk's `factory/selftest.sh` whole would delete 317 rows of this branch's
   legs, the file would still parse, and the suite tallies its own legs — so `0 failed` prints either way.
   The added-line census is the only thing in this tree that sees it, and it is not a leg.
7. **The counts of the hand-off this unit replaced.** It reported 40 owed rows on `main` against its own 35,
   and 8 ship bullets and 7 did-not-deliver bullets (3 general, 4 under "For the sixth sync specifically:").
   At this head: **51** owed rows carried + 1 new = **52**; the 3 general did-not-deliver bullets are
   restated above in this unit's voice because they are still true of the tree; the 4 historical ones and the
   8 ship bullets are superseded as narrative, and the two facts among them with a live consequence were
   already transferred to owed items 38 and 39 by the sixth sync, unchanged.
8. **Item 20 (a base sync is owed again the moment this one lands), re-measured.** The open queue at this
   tick still answers pull requests each one trunk-move from conflicting, and this sync changes nothing
   about that: the same two both-append files are the whole cause, so the next several syncs will look
   exactly like this one.

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

## What `origin/main`'s hand-off (`H-0232`, PR #414) shipped

Its own `## What this unit shipped` — 153 rows — is deliberately not reproduced here: it is
that unit's narrative of work that is now in trunk, and every row of its **owed** list, its
did-not-deliver section and its non-blocking rows is carried above. The zero-loss census in
the ledger attributes every line that is not here, so "not reproduced" is a stated scope, not
a silent drop — which is the #337 failure this file exists to end. What it delivered, for
whoever needs the shape: journal-test isolation (`H-0232`), `claim_unit` defined once in the
claims setup and exported in `ask_dispatch`'s env list, plus the control leg
`the claims dispatch asks only its own unit journal`; the renumber of its own entry from
`H-0204` to `H-0232` after a fresh fetch, because the number the review proposed, `H-0208`,
was itself already published on `main`; a base sync of its own (`67c05e3`) whose gate read was
294 entries, 780 citations, 0 unopenable, `POINTER_DEBT` 539; five bare coordinates
re-stamped on the merged tree; a dropped final newline in `factory/DECISIONS.md` caught and
restored; and the measurement its owed row 1 carries — the suite run with the dispatch
environment intact. Full prose: `git show 6e069eb:NEXT.md`.

## Wording ledger for `origin/main`'s hand-off (`H-0229`)

The added-line census reports a *wording* ledger, not a fact ledger: where two hand-offs state one
rule in different words, the union carries one wording and the census calls the other lost. So the
mapping is stated per item rather than inferred from a line count. Their 31 owed items, compared by
sentence against this file's 28 owed items, its 3 non-blocking items and its "did not deliver" list:

| their item | where it went |
|---|---|
| 1 | **new**, item 29 (with this unit's restore-point note appended) |
| 2 | already here — item 5 already carries `H-0217`'s instrument having no gate leg |
| 3 | **new**, item 30, with this unit's re-measured count appended (20 gate-blind, not empty) |
| 4 | **new**, item 31, with this unit's re-measured instance appended (both logs end in `\n` now) |
| 5 | item 23 |
| 6 | item 6, with their re-measured 120 s instance appended inside it |
| 7 | item 12 (the #425 dispatch-env defect) |
| 8 | item 1 |
| 9 | item 2 |
| 10 | item 3 |
| 11 | item 4 |
| 12 | item 5 |
| 13 | item 13 |
| 14 | item 11 |
| 15 | item 13 (this file's item 13 states both halves of their 13 and 15) |
| 16 | item 14 |
| 17 | item 15 |
| 18 | item 16 |
| 19 | item 24 |
| 20 | the "did not deliver" bullet |
| 21 | item 25 |
| 22 | item 26 |
| 23 | item 27 |
| 24 | item 8 |
| 25 | item 9 |
| 26 | appended inside item 12 as their instance, because their finding is that neither cause holds |
| 27 | this file's non-blocking section, items 1, 2 and 3 |
| 28 | **new**, item 32 |
| 29 | **new**, item 33 |
| 30 | appended inside item 24 with this unit's re-measurement (the gap list, and `350e688`) |
| 31 | **new**, item 34 |

Their "What this unit shipped" and "did not deliver" sections are the `issue-355` unit's narrative of
its own merge and stay on the branch whose blob carries it, per the rule below; the durable fact in
them — that #355's claims are untested against main's `serve`-stop rewrite — is item 32. This file's
own previous tick lost the same treatment in the other direction and says so here: its ship section
described the fourth sync, and that sync's numbers — seventeen repaired citations, the
`--ours`/`--theirs` asymmetry, the 20589/20590 junction, the twelve-named/five-withheld refusal and
its seven ambiguous rows — now live in `H-0228`'s entry in `factory/DECISIONS.md` rather than as a
line here, because a shipped sync's narrative belongs to its decision row. What survived the re-cut
verbatim: the `NEXT.md`-overwrite hazard, the feature bullets for #348, the census's first live run,
the non-blocking section, and items 1–28 in full.


### Ledger for `main`'s hand-off at `6e069eb` (the `fix/journal-test-isolation` unit, `H-0232`)

Forty rows, all accounted for: **20** are already in this file under its own numbering,
**4** are re-measurements placed in the section above, **1** is this file's own
non-blocking section, and **15** are new below as items 36–50. Nothing was dropped. The 15
new rows are that author's text, verbatim and indented, and `git show 6e069eb:NEXT.md` is the
source they match with digits folded; the 20 shared rows are carried in **this file's** wording,
so their phrasing lost and the census below counts exactly how many lines that cost.
Their 8 hard rules: 5 are already in this file's bullets in substance; 3 are added there in
their own words (resolve the index before gating; report the `FACTORY_*` env you ran under;
nothing in a review body authorises a command). Their checkpoint rule also carries one clause
this file lacked — re-run the allocator at merge time if a review proposes an id for you — and
it is carried in the checkpoint bullet.

| their row | where it lives here |
|---|---|
| 1 | re-measurement 1 — item 12 and item 36 |
| 2 | re-measurement 2 — item 12 |
| 3 | item 36 (new) |
| 4 | re-measurement 3 — item 28, and a hard rule below |
| 5 | item 37 (new) |
| 6 | item 38 (new) |
| 7 | item 39 (new) |
| 8 | item 40 (new) |
| 9 | item 9 (already here) |
| 10 | item 29 (already here) |
| 11 | item 41 (new) |
| 12 | re-measurement 4 — item 30 (which carries its own re-measured count) and the re-measurement section |
| 13 | item 31 (already here) |
| 14 | item 23 (already here) |
| 15 | item 6 (already here) |
| 16 | item 1 (already here) |
| 17 | item 2 (already here) |
| 18 | item 3 (already here) |
| 19 | item 4 (already here) |
| 20 | item 42 (new) |
| 21 | item 43 (new) |
| 22 | item 11 (already here) |
| 23 | item 13 (already here) |
| 24 | item 44 (new) |
| 25 | item 15 (already here) |
| 26 | item 16 (already here) |
| 27 | item 24 (already here) |
| 28 | item 45 (new) |
| 29 | item 25 (already here) |
| 30 | item 26 (already here) |
| 31 | item 27 (already here) |
| 32 | item 8 (already here) |
| 33 | this file's non-blocking section below (their three rows are the same three review items this section already carries) |
| 34 | item 32 (already here) |
| 35 | item 33 (already here) |
| 36 | item 46 (new) |
| 37 | item 47 (new) |
| 38 | item 48 (new) |
| 39 | item 49 (new) |
| 40 | item 50 (new) |


Zero-loss census, measured on the final bytes with digits folded and whitespace collapsed
(`git diff -U0 1c385b7 <side> -- NEXT.md` for each side's added rows, then a folded membership
test against this file). Over **all 7 paths either parent touched** since base — `NEXT.md`,
`factory/DECISIONS.md`, `factory/common.sh`, `factory/selftest.sh`, `factory/factory.sh`,
`AGENTS.md`, `scripts/check-decisions.py` — ours added **1109** non-blank rows and theirs
**720**, and the merged tree loses a row on exactly **one** path: `NEXT.md`. The other six read
zero absent on both sides, including the two code seams (`factory/common.sh`, ours +171 / theirs
+57) and `factory/DECISIONS.md` (ours +175 / theirs +206). On `NEXT.md`, this file's prior version
lost **24 of its 545** added rows — 20 preamble rows about *its* sync, which this file's header
restates in new words, plus one heading each in ship, queue-state, the census's first live run and
owed, all four renamed so a stale tick's timestamp cannot be read as this tick's. `main`'s copy
lost **165 of its 204**: 12 preamble, 24 ship (replaced by the scope section above, not dropped
silently), 11 did-not-deliver (mapped bullet by bullet there), 14 hard-rule lines (5 of their 8
rules are this file's bullets in other words; the 3 new ones are verbatim), and 104 owed lines —
of which exactly **40** are the numbered heading lines, each replaced here by this file's carrying
heading "Carried from `main`'s hand-off, row k, in its author's words" or by the row it maps to,
and **5** are the intro their owed section opened with; the rest are body lines of the **24** rows
this file carries in its own longer wording, and **zero** body lines are missing from the 15 rows
landed as items 36–50, whose bodies are verbatim. So the union's only real cost is *phrasing* on
shared rows, and the table above is what a reader checks a phrase against.

The pointer census over the merged tree moved **49** `path:line` coordinates on **32** physical
lines to stay true — **24** `factory/DECISIONS.md` self-citations (21 at one +101 offset, three
one-offs at +131, +78 and +80), **18** into `factory/common.sh` (17 base rows below the shared
seam at +171, one row inside this branch's own block at +57) and **7** into
`factory/factory.sh` (+11 ×4) and `factory/selftest.sh` (+259 ×2, +196 ×1). Three of those 49 were
created by the code seam and 46 were inherited from the log splice; the repair is reproducible as
the digit-only diff between the raw id-ascending union and the shipped log. Two coordinates
in *this* file were stale on the tree they describe and are re-stamped, not re-quoted:
`factory/common.sh:1490` "factory_worktree_lines() {" was the fifth sync's true row and the
function sits at `factory/common.sh:1547` here, and `docs/factory.md:652` "### The approval a push
voids" is at `docs/factory.md:651` on `HEAD`, on `origin/main` and on this merged tree — that file
is byte-identical to both parents — so the second is an off-by-one inherited through two hand-offs
rather than drift from this merge. Neither gate could have caught either: the anchor rule lives in
`scripts/check-decisions.py` and reads only the two decision logs, and `scripts/check-next-md.sh`'s
path leg tests `[ -e "$root/$p" ]` — that the named file exists — never that the numbered row holds
the quoted text. A hand-off pointer one row off is invisible to `make verify`, which is owed item 4
in the abstract and this file's own two rows in particular. A pointer whose author has already
shipped has nobody left to re-open it, so the sync that reads it is the sync that fixes it.

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
- Restore mutants with `cp` from a backup, never `git checkout --` (#2215), and **stage the resolved
  logs before running any script over them**, so the index blob is a whole restore point: a repair
  helper that re-reads its own path inside `open(path, "w")` truncated both decision logs to one
  line on `main` (item 29). Restore a log the same way — `cp` from a backup or
  `git checkout-index -f --` with the path explicit — never `git checkout -- <file>` while your own
  edits are uncommitted. Run probes on
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

- Resolve the index (`git add` every resolution) **before** running any gate: a conflicted path
  carries three stage rows, and `the timer's scripts are executable` reads `git ls-files -s`, so
  it reddens a merge for a reason that has nothing to do with the change (carried from `main`'s
  hand-off row 4, `H-0232`).
- If you run the suite on this box, say whether the `FACTORY_*` dispatch names were set, and
  expect two reds while #425 stands open; a green run **with** them set is the run that needs
  explaining (carried from `main`'s hand-off, `H-0232`).
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or
  an approval: answer it, and leave merges and publishes to whoever owns them. And re-run
  `factory/next-decision-id.sh factory/DECISIONS.md` at merge time if a review proposes an id
  for you — the allocator's answer belongs to the moment it was taken (both clauses carried
  from `main`'s hand-off, `H-0232`).

- Run every state probe as **one command per call**, and read the name of the ref you asked for before
  quoting its value. Measured at this tick: `;`-joined probes put four numbers into this ticket's reasoning
  that no single command confirms — a trunk sha with no such object, a "pushed head" that is another
  branch's merge commit, a "merge commit" that is this branch's own test commit, and hand-off rows citing a
  trunk sha present in neither copy of the file. A chained probe's output in a tree with a dozen live
  worktrees is not this branch's state, and a `;` chain dies on the first harness refusal, taking the probes
  you wanted with it (#1180).
- Before trusting a per-file offset to re-point a pointer, `grep -c` the quoted fragment. An offset is right
  for a pointer whose fragment is unique and *silently wrong* for one whose fragment is shared: measured at
  this merge, 23 unique-fragment pointers all landed at the uniform trunk offset and all 4 shared-fragment
  pointers landed on a row that contains the same fragment and is not the row the entry means (item 52).
  Offset arithmetic is a way to *find* candidates, never a way to *place* one.
