# NEXT — issue-355 unit (PR #411, issue #355)

State at push: branch `issue-355`, first base sync for PR #411, recorded as `Decision: H-0229`
(checkpoint `ca5e47f1`, cut before the merge started). The merge takes `origin/main` `5aaa9a7` with
merge base `6db0b13`, which is this branch's own `HEAD~5`. The feature this branch delivers is
`D-0085` plus `H-0202`: the unenforced doctor join now answers to one clock, the acceptance fixture
says `allDay`, and `scripts/check-fixture-window.sh` refuses the `00:00`/`23:59` pair. Everything
below is this unit's hand-off plus every owed line from `main`'s hand-off (the `issue-346` unit's,
`H-0226`) that this unit re-read as still true and carries in reduced wording — **none dropped**; the
replaced file is on `main`'s side of this merge at 297 lines if a row needs its full prose.
Coordinates are quoted as their authors published them unless marked **re-measured**, which means
opened on this merged tree. `make verify`'s verdict is quoted in the pull-request body from the run
that ships these bytes, never here (#1180).

## What this unit shipped

- The first base sync for PR #411, `H-0229`: **two** conflicted files, `DECISIONS.md` and
  `factory/DECISIONS.md`, one hunk each, both pure tail double-appends (this branch wrote no line
  above the append point in either log), resolved as one ascending-id union — `D-0084` then `D-0085`,
  and `H-0201`, this branch's `H-0202`, main's `H-0204` onward. `H-0202` seats strictly **inside**
  main's fresh run, which is `H-0209`'s shape rather than `H-0226`'s.
- Proof the union lost nothing, measured not asserted: zero-loss census over all 22 paths either
  parent touched since `6db0b13` — 6 added lines are not byte-identical in the merged tree and **0**
  are absent with digits folded, so every difference is a coordinate.
- Nine coordinates re-pointed (8 in this log, 1 in `DECISIONS.md`), each opened at its own number on
  `origin/main`'s blob first, then found again here by row identity, then asserted digits-only. Both
  whole-side orders were run against the parent blobs: ours-first is a descending sequence the gate
  refuses and main-whole deletes `H-0202` and `D-0085`; the union deletes neither.
- The census the gate cannot do, because it opens only anchored citations: all 34 citations of
  `factory/DECISIONS.md` in the two logs, chased parent→merged row by row — 26 above the splice still
  true, 8 below it moved, all 8 repaired, **0** left silently mis-aimed. `POINTER_DEBT` is 539 and
  stays 539 (`python3 scripts/check-decisions.py`: 291 entries, 760 citations, 0 pointing somewhere a
  reader cannot open, **re-measured** on these bytes).
- The `Makefile` step this branch adds came through the auto-merge at `Makefile:152` "fixture window
  guard" **re-measured** — main's own step landed above it, which is what moved `D-0085`'s pointer.

## What this unit did not deliver

- No behaviour was re-derived for the merged tree. `make verify` is the only verdict this merge runs
  over the combined `cmd/plock` tree; the sync proves the two logs and fourteen auto-merged paths
  lost no lines, which is a presence claim, not a claim that the doctor join still answers to one
  clock after main's `serve`-stop rewrite landed next door.
- None of main's 40 fresh entries (`H-0183`..`H-0226`) was re-measured. Eight of their published
  coordinates were rewritten by this hand, digit-only, each opened on a parent first.
- The seven `factory/DECISIONS.md` headings with no blank line above, and the missing final newline on
  `origin/main`'s `DECISIONS.md` at `5aaa9a7`, were left as found — see the owed list.

## What is still owed (all listed, none silent)

Items 1–7 are this sync's; items 8–30 are carried from the `issue-346` unit's hand-off (`H-0226`) and
its own carried lineage, in that file's order, condensed but not dropped.

1. **A repair helper that re-reads its own path inside `open(path, "w")` truncates the log.**
   Measured here: the first version did exactly that and both decision logs were written down to one
   line. Nothing was lost only because the union had been staged first, so the index blob was a whole
   restore point (`git checkout-index -f -- <log>` returned both byte-identically). No leg covers it.
   Rule to carry: stage the resolution, then edit; and read once, write once.
2. **The instrument that repairs merge-moved pointers still has no gate leg.** Carried — `H-0217`'s
   census found 24 bare tokens for the `issue-334` merge and 20 drifted anchored citations for
   `H-0226`; this sync is a third instance (9). `check-decisions.py` refuses every drifted *anchored*
   citation but only *locates* them — 3 of this sync's 9 shared one phrase with five candidate rows —
   so any leg must resolve by parent-blob row identity. #381 owns the price.
3. **Gate-blind pointers are empty at this head, not by design.** All 34 citations of
   `factory/DECISIONS.md` happened to be anchored. The next bare `path:line` under the splice moves
   invisibly, and 539 bare pointers across the two logs are already outside the checker's reach. #428
   and #381 own that class.
4. **`origin/main`'s `DECISIONS.md` at `5aaa9a7` ends without a final newline** (**re-measured**: last
   byte `.`), so the next EOF append on any branch lands against that line; this merge happened to
   restore the newline because ours' blob ended with one. Nothing in the tree requires a log to end
   with a newline, so nothing stops the next drop.
5. **Seven `## H-` headings in `factory/DECISIONS.md` have no blank line above them in BOTH parents**,
   so they are prose to a renderer — rows 2315, 3737, 4907, 6936, 8670, 8998, 9015 (**re-measured**:
   unchanged at 7 here, none authored by this splice). A fix edits seven other units' rows and
   cascades pointers, which is why it was left.
6. **A tick's pre-flight fetch is unbudgeted** (carried, #419). **One instance re-measured here:**
   `git fetch --prune origin` in this tree hit the tool's own 120 s ceiling once and returned rc 0 in
   well under a second on the retry, moving no ref. A fast fetch is not evidence the ceiling was wrong.
7. **`make verify` reads RED from a tick's own shell until #425 is repaired** (carried; see item 26).
   This unit ran `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make verify` and says
   so where the verdict is quoted.
8. `H-0188` is owed a human answer and is the only thing that bounds `factory/log` (20 MB / 100 files
   on 12 Nov, 36 MB / 172 later); rotation and truncation are both deletions of the operator's records,
   a reserved class. Read the escalation comment on #334 before touching the footprint function again.
9. Nothing reads whether a hand-off is TRUE — `scripts/check-next-md.sh` reads the tree and never the
   API, so a well-formed hand-off about finished work still passes.
10. The phantom-path leg reads three files, not every file (four named paths are declined as
    true sentences about refused or generated paths; the authoring unit measured them, this one did
    not re-run it).
11. A dropped owed line is still uncaught — no check has memory of the last hand-off; the measured
    instance is `#296`, in and out across two unit overwrites, not a merge.
12. 539 decision-log pointers carry no anchor and a merge moves them invisibly; `H-0217`'s instrument
    found 24 bare tokens on 19 rows across ten entries at the `issue-334` merge, none edited
    deliberately (three stamp the tree their digits were taken on and must NOT be re-pointed;
    `H-0169`'s three rows are the debt to take first because its own text makes the drift a defect).
    Repair shape is `H-0148`'s: a quote taken from the named line, never a paraphrase.
13. A checkpoint commit message once said "eight fresh entries" where the merge measured six
    (`H-0183`, `H-0184`, `H-0185`, `H-0206`, `H-0209`, `H-0213`); `ed1954e` is empty and cannot be
    re-worded without a rewrite, which is denied here.
14. A tool read can return a file that is not on disk — verify before you overwrite it (measured on
    16 Nov in `wt/issue-334`: a read returned a coherent 439-line hand-off no ref ever held, while the
    real file was 120 lines). Instruments before any overwrite: `git diff --quiet HEAD -- NEXT.md`,
    `wc -c`, `sha256sum`. Same class as #1180, on the read side. **This unit's own read before
    overwriting:** `HEAD:NEXT.md` is main's 297-line `H-0226` hand-off and `git status --short` after
    the merge listed `NEXT.md` as modified by the merge alone — this branch never touched the file
    (`git diff --name-only 6db0b13..HEAD -- NEXT.md` is empty), so nothing this branch wrote is lost.
15. Census numbers belong to a stage, not to a commit — state them with the stage they belong to;
    `H-0226` names its 217-entry candidate and 218-entry shipped blob separately, and this file's
    numbers say which side of the entry they were taken on.
16. Five claims one unit made about its own work did not survive re-derivation — the accusation that
    `TREE-CLEAN … cause=never-dirty` proved the dirt probe blind to a mid-merge tree is **withdrawn**
    (`ORIG_HEAD`'s mtime is free and the probe was right), and the substitute ("no probe ever runs
    against a conflicted tree") was also false: two legs assert a conflicted index for `[primary]` and
    `[worktree]` and a mutant retires an arm to prove they are load-bearing. A coverage claim comes
    from legs and their asserted substrings, never from a redundancy comment.
17. A control tree must contain the legs it controls — grep the control tree for the fixture by name
    before trusting a green control run.
18. `reap_leg_live` refuses a hold that is legitimately zero: `factory/selftest.sh:18570` (carried
    coordinate, **not re-measured here** — this merge added no lines to that file above it, but the
    number is `H-0220`'s, not this unit's). A `oneshot` killed at its deadline writes its terminal
    lines in one instant, so the hold it measures IS zero.
19. `H-0225` is burned by a branch whose pull request is not in `main` yet (`refs/heads/issue-348` and
    `refs/remotes/origin/issue-348`); `H-0203` is held the same way. A sibling in flight consumes an
    id `origin/main` does not show — re-run the allocator after a fresh fetch, never guess.
20. This unit did not re-derive any of main's fresh entries' measurements, including `H-0224`'s claim
    that a 126-line reseating cascade moved 46 more pointers and made 8 quotations ambiguous.
21. #416 aims at `H-0197`'s shipped function and was not re-run here: the main-checkout restore arm
    rewrites `.cortexkit/.gitignore` every activation if the tool that owns it deletes it again.
22. A hand-off can cite a section that no ref ever held and no leg sees it — the phantom *heading*
    inside a real file passes the phantom-*path* leg. Substance carried: an unanchored-pointer sweep
    belongs on a branch that is not also resolving a conflict.
23. A control test can be vacuous in this family and say nothing — both refused splice orders were run
    here only after the union passed.
24. Carried from the `issue-325` unit, none re-measured here: the deleted-call hole (removing
    `_rule_probes_are_live(...) from self_test()` is still green — D-0076's class, D-0081 said the
    closure did NOT ship); the switched-off derivation (deleting the PROBE-prefix trip count loses the
    verdict's tail on battery-only trips — a recorded trade); and #325 itself stays open until its own
    test is met on the merged tree.
25. **This push spends the approval, and only a read-back says so.** PR #411 read
    `reviewDecision=APPROVED` with `reviewRequests: []` at head `d8aa547` before this push; ask
    `gh pr view 411 --json reviewDecision,reviews,mergeable,mergeStateStatus` after the push and
    before calling anything ready. The re-ask is the supervisor's poke, not a unit's.
26. `make verify` reads RED for a tick that exports its own dispatch env (`FACTORY_LOG`,
    `FACTORY_TICK_UNIT`, `FACTORY_TICK_BRANCH`), and it is `main`'s defect (#425). Workaround until
    repaired, and say so wherever the verdict is quoted:
    `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make verify`.
27. **Three non-blocking review items from PR #402's review, carried because they are still true of the
    shipped gate**: a bare `-` bullet slips the empty-marker leg; the scanned-file list is unpinned
    (a future PR could narrow it with the suite green); the shape leg early-returns after its first
    refusal so a hand-off with two defects reports one per run. None is a merge blocker.
28. #355's own claims are untested against main's `serve`-stop rewrite: `D-0085`'s single-clock join
    and `H-0202`'s shape-guard were measured on this branch's own head. The gate is green over the
    merged tree, which is the only new evidence.
29. Two open branches still re-conflict on these same two logs at every merge (#428, #381): this is
    the third sync this hour whose whole conflict surface is a decision-log tail.
30. `H-0203` and `H-0225` remain gaps in the merged id sequence with no pull request in `main`;
    `docs/factory.md` requires a gap to be explicable, and these two are explained only by refs on
    this machine. A unit on another clone cannot see why.

## Hard rules for whoever picks this up

- Checkpoint before the decision, one effect commit carrying its `Decision:` trailer, and
  `factory/next-decision-id.sh factory/DECISIONS.md` re-run after a fresh `git fetch` before taking an
  id. Read the `REFRESH` / `REFRESH-BLIND` / `PENDING` note, not only the id.
- Repair a merge's drifted citations by **row identity, never by a delta**: find the carrying row in
  the parent blob whose copy it is, open the cited row there, find it again here, move the number,
  assert the row is byte-identical with digits stripped, then re-open every anchor.
- Before editing a file either log cites, run `python3 scripts/check-decisions.py --citations <path>`
  and fix what it lists in the same commit. Every `path:line` you write needs a quoted anchor of ≥16
  characters on the SAME physical line or `POINTER_DEBT` rises and the gate refuses — this unit wrote
  two bare ones (a heading coordinate and a stale `Makefile` number) and the gate named the rise before
  anything shipped.
- An `Options:` line uses `|` only as the separator and `Chose:` quotes one option verbatim.
- Stage the resolved logs before running any repair script over them, so the index blob is a whole
  restore point (owed item 1). Restore with `cp` from a backup or `git checkout-index -f`, never
  `git checkout -- <file>` while your own edits are uncommitted (#2215).
- A decision-log union is keyed by id and mechanical: splice the ascending union, then prove it — the
  added-line census over every path either parent touched, the marker count, blank-line hygiene at
  both junctions, both whole-side orders actually run, and a parent-relative landing census of every
  citation for the drift the gate cannot read.
