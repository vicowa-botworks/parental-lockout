# NEXT — issue-348 unit (PR #410, dispatched `conflict:410`)

State at push: branch `issue-348`, base-sync merge carrying `Decision: H-0225` (checkpoint
`674cb95`), on top of this branch's own feature work for #348 (`H-0200`, the worktree census).
Everything below is this unit's hand-off plus every owed line from `origin/main:NEXT.md` — the
`issue-334` hand-off that #401 shipped — that this unit re-read as still true on the merged tree.
`make verify`'s verdict is quoted in the pull-request body from its read-back, never from here
(#1180).

## What this unit shipped

- The base sync for PR #410, recorded as `H-0225`. It is the first sync in this family whose **code**
  conflict sits inside a function both sides rewrote: main's #331 hoisted the three self-report loops
  out of `factory_body` and #342 split its origin fetch, while this branch appended the census loop to
  the block main deleted. The resolution keeps main's `factory_body` and grafts the census behind the
  pass's **last** fetch — the census's deadness verdict reads `origin/main`, so it must run after every
  fetch the pass makes. `factory/factory.sh:1546` "done < <(factory_worktree_lines" is the graft.
- Both whole-side splice orders were **run** as `/tmp/opencode` mutants and both refused at rc 1: the
  tail append after `H-0224` answers `H-0200 is out of order (H-0224 came before it)`; the insert before
  main's `H-0183` answers the same four times (`H-0195`, `H-0196`, `H-0198`, `H-0199`). Only the
  ascending interleave is legal, so the "both sides just appended" shape is not available here.
- 23 drifted decision-log pointers repaired, taken from the merged tree, each only after its quotation
  was opened at the named line; four had ambiguous quotations and were resolved by requiring both the
  file's measured offset and the citing entry's quoted string to hold at the candidate. `POINTER_DEBT`
  546 held (546 of 741 citations owing an anchor, 0 unopenable).
- The feature #348 asked for, unchanged by the sync: `factory_worktree_lines` in
  `factory/common.sh:1490` "factory_worktree_lines() {" — a census that **reports** dead worktrees
  (`WORKTREE-ORPHAN <path> head=<sha> reason=merged-and-clean|outside-trees`), prunes git's admin rows
  in the same pass (`WORKTREE-PRUNABLE`/`WORKTREE-PRUNED`, `DRY-RUN` under `DRY_RUN=1`), and says
  `WORKTREE-CENSUS-FAILED` rather than being silent; plus its legs in `factory/selftest.sh`, including
  the control that dirties the planted tree and requires the line to be absent.
- Nothing was removed. `git worktree remove` is refused to this session (the permission-refusal class
  #1180 names) and forced removal of a directory someone may be standing in is the reserved act #348
  itself declines.

## Queue state this tick read (16 Nov, ~20:50Z)

- `gh pr list --state open --limit 500 --json number` → **5** open PRs: #408, #410, #411, #413, #414.
  `gh issue list --state open --limit 500 --json number` → **56** open issues. Drafts **0**. `#414`
  reads `CHANGES_REQUESTED`; the other four read `APPROVED`. `#408` and this `#410` are the only two
  `CONFLICTING`/`DIRTY`; `#411`, `#413`, `#414` read `UNKNOWN`/`UNKNOWN` (GitHub has not recomputed).
  Both counts carry `--limit 500`: gh's default page is 30, and an unbounded-looking count is a page
  (#1180). This tick counted the raw JSON in python rather than a `-q` template, because a bare
  generator inside an interpolated string drops rows at rc 0 — also #1180, measured 14 Nov.
- `main`'s tip is `580d722` (PR #409, `issue-347`, merged 20:41:40Z). Between the `issue-334`
  hand-off (tip `6ca61c3`, ten open PRs) and this tick, **four** pull requests merged — #405 at
  16:20:56Z, #401 at 17:50:51Z, #406 at 19:11:37Z, #409 at 20:41:40Z — read from
  `gh pr list --state all --json number,state,mergedAt`, not from prose. #401 merging is why main's
  hand-off is the one this file replaces.
- This branch's own state before the write: #410 `APPROVED`/`CONFLICTING`/`DIRTY` at head `f2ca7e1`,
  merge base `6db0b13`, base tip `580d722`. The push spends that approval (item 9 below).

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
   #334 is still `OPEN`. Rotation and truncation are both deletions of the operator's records, so no
   unit may pick one; read the escalation comment on #334 before touching `factory/common.sh`'s
   footprint function again.
2. **Nothing here reads whether a hand-off is TRUE.** `scripts/check-next-md.sh` reads the tree and
   never the API, so a well-formed hand-off about work `main` finished last week still passes. A leg
   that asks GitHub needs a token CI does not have; `scripts/check-denies.py` is the precedent for
   skipping loudly.
3. **The phantom-path leg reads three files, not every file** (`docs/factory.md`,
   `factory/selftest.sh`, `factory/next-decision-id.sh`, plus `testing/capture/cdp.py`'s generated
   manifest). Carried, not re-measured — **but this tick did measure a phantom of the adjacent kind, in
   the hand-off itself**: the `issue-334` item 5 cited *`docs/factory.md`, "What a conflict-resolution
   commit is allowed to contain"*. No such heading exists on the merged tree
   (`grep -n '^#\{2,4\} ' docs/factory.md | grep -i conflict` → nothing; the only "conflict-resolution"
   hit is prose at `docs/factory.md:658` inside `### The approval a push voids`). The rule it meant is
   the `H-0143`/`H-0144` lineage in `factory/DECISIONS.md` — a published coordinate is re-pointed, never
   rewritten. #337 is the open issue for a contract naming a gate that does not exist; this row is one
   more instance of it, in prose rather than a dispatch.
4. **A dropped owed line is still uncaught** — no check has memory of the last hand-off. The measured
   instance is #296 (`git log --all -S '#296' -- NEXT.md`: in at `89c2298`, out at `af848f4`). This
   tick's evidence for the rule working is item 10 below: a carried coordinate that had moved 1500 lines.
5. **546 decision-log pointers still carry no anchor, and a merge moves them invisibly.** The
   `issue-334` hand-off said 539; the merged tree measures 546 (`python3 scripts/check-decisions.py`'s
   closing line, against `POINTER_DEBT` at `scripts/check-decisions.py:595`), and the ratchet only
   guards anchored citations, so a bare `path:line` whose line still opens somewhere is invisible at
   rc 0. `H-0217`'s instrument still has no gate leg; PR #422 is the open branch that reads the merged
   tree instead of the working one, and #381 owns the re-conflict price this sync is one instance of.
6. **A tick's pre-flight fetch is unbudgeted.** #419 stands: one hung `git fetch --prune origin` ate a
   whole 5400 s pass with zero journal lines, and the allocator's own `REFRESH` advice tells a unit to
   fetch without bounding it. This tick's fetch returned inside the 120 s default, so no new overrun.
7. **The dispatch that invented the NEXT.md gate was not the only one** — nothing in the tree enumerates
   the commands a dispatch may name, so a future sentence can invent a different tool.
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
    repair it by weakening either leg.
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
19. **`make verify` never touches the mutant sets — CI's `mutation sets` job is the only gate that does,
    and a base sync is exactly what stalenesses them.** `Makefile:48` "kept OUT of verify for the opposite
    reason" states the split as a cost decision: the checker compiles the tree once per mutant and pays
    minutes. But a sync whose whole content is moved code lines is what moves a mutant patch off its
    anchor, so a sync unit runs `make check-mutations` before pushing rather than letting CI find it.
    Measured at head `e8ab8b9`: the local run answers `all 847 mutants apply and compile`, with its own
    caveat printed beside it — `this does not run the suite, so a mutant that survives every test is
    invisible here (make mutate)` — and CI's job on the same head passed in 16m19s. Two mechanics worth
    knowing before you start it: it applies each mutant to the **working tree** and restores between runs,
    so `git status` reads one to three dirty files while it runs and nothing may be committed during it
    (the same hazard #2215 names for the negative-control helper), and its summary is a per-file tally over
    `scripts/mutations/` — 13 files, `audit` 59 through `web` 292, summing to the `all 847` it prints. That
    847 is the mutant **set** population and is not the number any `make verify` line reports; quoting one
    as the other is how a set count gets inflated in prose.

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
   in `AGENTS.md`.
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
  and the union splits one side's tail.
- Restore mutants with `cp` from a backup, never `git checkout --` (#2215), and run probes on
  `git archive` copies or files under `/tmp/opencode`, never the gated tree. `rm -rf` is refused here, so
  use fresh unique directory names.
- Run the gate as CI runs it: `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make
  verify` until #425 is repaired, and say so wherever the verdict is quoted.
- **A base sync also runs `make check-mutations` before it pushes.** `verify` never touches the mutant
  sets — `Makefile:48` "kept OUT of verify for the opposite reason" states that as a cost decision — and CI
  is the only thing that checks them, so a sync whose content is moved code lines can move a patch off its
  anchor and read green locally. Nothing is committed while that run is in flight: it applies each mutant to
  the working tree and restores between runs, so `git status` reads one to three dirty files on purpose
  (#2215 is the same hazard). Measured at head `e8ab8b9`: the local run prints `all 847 mutants apply and
  compile`, the 847 being the per-file tally's own sum (13 files, 59 at the low end, 292 at the high), and
  CI's job on the same head passed in 16m19s.
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
- Overwrite this file, carrying every still-true line you did not write — and re-measure the live
  coordinates in it before you cite them (item 10).
