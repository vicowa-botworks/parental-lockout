# NEXT — issue-377 unit (PR for `issue-377`: the pass now compares the generator's `Environment=` name-set against the installed unit and prints `UNIT-ENV-STALE` per missing name, or `UNIT-ENV-UNKNOWN` when either side cannot be read; the comparison lives inside the readable arm of `factory_unit_lines`; four selftest legs each carry their control; `H-0287`; the install half of #377 is an operator act and is NOT done by this unit; merging belongs to the supervisor)

State read before this file wrote a byte, by this tick, raw JSON and never a `-q` template: this
checkout is `/home/rodney/prj/wt/issue-377` on branch `issue-377`, base `origin/main` at `51c87d3`
(the #455 merge, agreed by `git ls-remote` at fetch time), merged in as `d6f7b59` before any decision
file was written, so the `H-0287` tail seats behind trunk's `H-0286` with no junction. The allocator
was asked after that fetch and again immediately before the entry (`H-0287` both times, journalled
`REFRESH` union, no `PENDING`, no `REFRESH-BLIND`), and `## H-0287` sat on no ref or tree this clone
could name at read time; re-read before honoring this id on any successor's authority. `git status
--short --ignored` in this tree holds only the files this unit edited, so the `check-decisions`
ratchet was judged, not exempted. No live unit was read, rewritten or reloaded here, and no
`systemctl` call was made: the install is the operator's (#377 half 1, named below and never acted on).

## What this dispatch pushed

- `factory/common.sh`: `factory_generator_env_names` (sorted NAMES the generator's template writes;
  rc 1 only for a file that cannot be read, so the readable-but-empty generator is the empty set and
  not a failed read) and `factory_unit_env_lines` (one `UNIT-ENV-STALE unit=… name=…` sentence per
  name the install lacks, `UNIT-ENV-UNKNOWN` for either unreadable side, names compared never
  values, which is what keeps #241's documented empty-value override from being nagged). The call
  sits inside the readable arm of `factory_unit_lines`, after `UNIT-AT`/`UNIT-DIVERGED`, not above
  the `case`: the uninstalled, foreign and unreadable arms already own their sentences, and a
  comparison against an absent, hand-written or unreadable unit would print invented drift.
  `factory_unit_lines` grew a third argument — the generator path — defaulting to this file's own
  tree, which is the #260 rule that the tree running carries the newest expectation.
- `factory/selftest.sh`: four `claim_leg` legs after `unit_records_its_own_name` — drift names every
  missing name (with the empty-value fixture and the arm-deletion control rewriting the sentence
  out of a copy of `common.sh` and re-running the same fixture in a fresh `bash`); match-is-silent
  plus the discriminating rewrite (one name added to the generator flips silence to exactly one
  sentence naming it) plus the empty-generator case plus the `#Environment=` comment case;
  unreadable-generator prints `UNKNOWN`, never a match, with its own arm-deletion control and a
  readable-generator flip-back; and the three other arms print no set verdict at all, gated by the
  hoist control that moves the call above the `case` in a copy and requires the UNINSTALLED case to
  start speaking. Every control premise-checks its own rewrite before believing it (the H-0184
  shape), and a control that answers nothing while its premise held prints the fixture-is-inert
  message rather than passing.
- `docs/factory.md`: a paragraph beside the #260 unit reads quoting the shipped sentence, the
  warning-not-halt polarity, the names-not-values rule and the operator-act fix.
- `factory/DECISIONS.md`: `H-0287` with four lost options each carrying its reason, checkpoint
  `6e3e0e0`; and the 147 drifted anchored citations the shipped gate refused (measured by its own
  first scan on this tree: 147 rows across 114 lines in 49 entries, all of them moved by this
  unit's own insertions) re-aimed at the line the gate's own refusal named, `check-decisions` green
  on these bytes with `POINTER_DEBT` unmoved at 583 and 0 refused (the count is the same number;
  what moved is which line each citation names). One slip, disclosed: the first repair helper
  replaced every occurrence of a coordinate on a row, and `factory/DECISIONS.md:12503` (H-0235's
  Effect row) carries the same coordinate twice — seven of its eight `selftest.sh` pointers
  collapsed onto one line number — caught by the shipped gate on the next run and the row rebuilt
  from `git show HEAD:` with the gate's per-anchor verdicts as the identity, not an offset.
- The reproduction that motivated the shape, run against the shipped code before the legs existed:
  a three-`Environment=` unit planted against the real 5-line generator answered exactly the two
  names #377 measured missing (`FACTORY_REVIEWER_TEAM`, `FACTORY_SERVICE_UNIT`) and nothing else;
  the empty-value override answered no sentence. Both are legs now.

## What this push spent

- One decision id (`H-0287`), one checkpoint (`6e3e0e0`), one base sync (`d6f7b59`, trunk
  `51c87d3`, clean, zero conflicts, zero content movement).
- No cloud spend, no GitHub write before this file's own pull-request step, no live systemd object.
- 147 citation re-aims (a repair, not a change of promise: every one keeps its own anchor text and
  moves only the number, and the gate re-opened each one by reading the anchor at the new line —
  that read is what `check-decisions` rc 0 means here).

## What is still owed (this tick's rows first, then every carried hand-off row, none silent)

1. Review on this unit's PR: the poke (`factory_request_review`, `FACTORY_REVIEWER_TEAM`) owns the
   first ask; a unit never asks. Whether the merge moved the review state is a post-push read
   quoted in the body, not a claim of this file.
2. #377 half 1 is NOT delivered by this branch and cannot be: the live installed unit on this
   machine still predates the two generator lines, and re-running `factory/install-timer.sh`
   rewrites and reloads the unit that drives the queue and re-derives `PATH` from the caller's
   environment — an act for whoever maintains the timer, never a tick's (`systemctl` is in the
   denial list; `H-0287` carries the split). After this lands, the pass itself will print
   `UNIT-ENV-STALE` twice a day naming those two names: that journal noise is the feature working
   against the un-reinstalled install, and the quiet fix is the operator's reinstall, not silencing
   the read.
3. If this branch is base-synced by a later conflict tick: the `H-0287` Effect quotes `check-decisions`
   output as measured on THESE bytes (`51c87d3` + this unit); a union that moves the lines my own
   citations re-aimed must re-aim by anchor, never by offset — the slip disclosed above is exactly
   what an offset repair does to a row that cites one coordinate twice.
4. The allocator journalled `H-0287` to this unit; if the entry never lands, the journal holds an id
   no union can retire — the standing class the carried rows below name, unfixed and un-papered.
5. Carried from the `H-0286` head above this Record, none of it touched by this tick: its rows 1-8
   stand as written there (the #455 review row, the two `#425` hazard rows and their `H-0252`
   `Follow-up:` claim, the #311/#334 operator waits, the two harness defects (a) the exported
   `FACTORY_TICK_UNIT`/`FACTORY_LOG` allocator legs — which this unit re-measured the same way as
   its own gate run, under `env -u` of the six exported `FACTORY_*` names, and (b) the
   journalled-id-invisible-to-union class — the census-by-anchor lesson, and the `dash_ctl`
   locale row).

## What this head pruned, and how to get it back

| Dropped | Where it is whole | Why it is a record and not an instruction |
|---|---|---|
| the issue-311 `H-0286` dispatch head's level-1 heading (line 1 of `git show 51c87d3:NEXT.md`) | that blob | demoted to a Record heading below; its state read and all four of its record sections — pushed, spent, owed, pruned — are carried whole beneath it, so "this dispatch" names one unit |

Everything else the file held is carried forward unedited, including that Record's owed section and
every instruction section below the Records. No owed line was dropped: the census is the one-direction
line-multiset diff of `git show 51c87d3:NEXT.md` against this file (lines of the parent absent here;
this head's own added lines are not the census's subject), and it found exactly one — the demoted
heading itself. `python3 scripts/check-close-verb.py NEXT.md` still rc 1 on the two carried `#425`
hazard rows, disclosed, unchanged: this unit edits no line another unit's claim rests on.


## Record — the issue-311 unit (PR #455 conflict tick: the base sync onto `292b4d9` seats this branch's `H-0285` below trunk's `H-0283`/`H-0284` by entry id at a one-junction tail, this head replaces the issue-375 `H-0284` head, the hazard-row pointers are re-aimed at the moved hand-off, `POINTER_DEBT` re-measured on the merged corpus, `H-0286`; merging belongs to the supervisor)

State read before this file wrote a byte, by this conflict tick, one read per call, raw JSON and
never a `-q` template: **#455** `OPEN` (raw `url` says `/pull/455`), `reviewDecision` empty,
`mergeable` **CONFLICTING**, `mergeStateStatus` **DIRTY**, head `68a3862` matching this checkout,
`baseRefName` `main`. Both type reads ran in the same breath before any write and both name the
pull — `gh pr view 455 --json headRefName` answers `issue-311-ask-shape`, a field an issue cannot
supply, and `gh issue view 455 --json url` answers `/pull/455` — because a bare number read is not a
type test (#2027). `git fetch --prune origin` ran first and `origin/main` is `292b4d9`; the merge
base was `b433f01`, so trunk had moved by the #454 merge since this branch cut. The allocator,
fetched first, answers **`H-0286`** (`REFRESH 8s union`, no `PENDING`, no `REFRESH-BLIND`), journalled
to this unit, and `## H-0286` sits in no entry on any ref or tree this clone can name. Re-read all
of it before acting: a number here belongs to the clock it was taken on.

## What this dispatch pushed

- `c67c41a` (the pre-`H-0286` checkpoint, the rollback target) and the merge commit carrying
  `Decision: H-0286`: the base sync of `origin/main` at `292b4d9` into `issue-311-ask-shape`.
  `git merge` counted ONE conflicted path, `factory/DECISIONS.md`, one tail junction — this branch's
  `H-0285` meeting trunk's `H-0283` + `H-0284` directly below the shared `H-0282` — resolved as the
  ascending-id union with trunk's two entries seated first and ours last, the seat
  `scripts/check-decisions.py`'s ordering rule leaves for one id. The shipped gate read rc 0 over
  that union BEFORE this head and this entry existed: 1153 citations, 583 owing, 0 refused — zero
  drift, because each side appended and only one side edited each seam row. The three auto-merged
  paths were censused anyway, being resolutions git made without being asked: `NEXT.md`,
  `docs/factory.md` and `scripts/check-decisions.py` all arrive from trunk alone (this branch
  touched none of the three), and the `POINTER_DEBT` constant trunk carries reads 583, the value
  the merged corpus confirms.
- This branch's own deliverable — `H-0285`, the one-answerable-comment escalation shape and the two
  stale marker-name rows in `.opencode/skills/factory/SKILL.md` — shipped before this tick at
  `68a3862` and is untouched by the merge; the `SKILL.md` line count is unchanged at 625, so the
  pointers aimed into that file kept their ground through the sync too.
- This head, plus the re-aim of every `NEXT.md` coordinate the insert moved (row 2).

## What this push spent

- Nothing this file asserts before the push. `reviewDecision` read **empty** on #455 at `68a3862`
  before this file wrote a byte — there was no verdict for the head move to spend — and the post-push
  read lives in the pull-request body. The supervisor's poke (`factory_request_review`,
  `FACTORY_REVIEWER_TEAM`) owns the ask either way; it is never a unit's verb.
- Gate, `check-next-md` and `make verify` verdicts are quoted in the pull-request body from the runs
  on the bytes this file ships, never here.
- Nothing else: no comment posted, no issue touched, nothing merged. The two logs and this page carry
  only what the merge of `292b4d9` put there plus this unit's own entry and head.

## What is still owed (this tick's rows first, then every carried hand-off row, none silent)

1. Review on #455: it never held one, and the poke owns the first ask; whether the merge moved the
   review state is a post-push read quoted in the body, not a claim of this file.
2. The two `#425` hazard rows in the instruction sections below still sit on the keyword and still
   refuse `python3 scripts/check-close-verb.py NEXT.md` with rc 1, unchanged by this merge; this
   head moved them off the `H-0284` coordinates, and every pointer naming them in
   `factory/DECISIONS.md` was re-aimed at the line the shipped gate's refusal named — the move is
   recorded in the `H-0286` entry. This tick again declined the two-word fix, for the reason the
   Records below gave: `H-0252`'s `Follow-up:` row asserts each parent's copy refuses at those exact
   bytes, and editing them retires an openable claim this merge did not pay for.
3. The allocator journalled `H-0286` to this unit; if the entry never lands, the journal holds an id
   no union can retire — the standing class below, which this tick did not fix and did not paper
   over.
4. #311 stays open for a human word, carried from `H-0285`'s `Follow-up:` — the rename shipped in
   #441 and this branch paid the prose that contradicted it, but closing a ticket whose operator
   answer arrived in chat is not a unit's call. #334's next sub-question (journal bytes and
   generations) was posted in the `Question:` shape at 21:06Z and waits on the same operator.
5. Two harness defects, carried untouched because both are outside a conflict tick's remit: (a) the
   allocator-journaling selftest legs inherit `FACTORY_TICK_UNIT` and `FACTORY_LOG` from a
   dispatching shell and redden `a bare run journals nothing` in the exported tick environment; the
   fixtures must scrub the two names. (b) The allocator's union reads entries on refs only, so a
   `DECISION-ID` line journalled to a sibling that never landed an entry is invisible to it and no
   `PENDING` fires; the cure stays the journal's abandoned line, not a bigger union, and the two
   `H-0277` rows still stand as claims nothing can retire but a landed entry.
6. Carried census lesson, met again in prose rather than pointers: a base-sync union must adjudicate
   on a citation's **anchor**, not on the cited line's bytes — a row the other side *rewrote* can be
   true at the number it names, and a line-comparing sweep would "repair" it onto a different row.
   A census that compares only what each side *added* is also blind to a pre-base row whose wording
   changed. Ask both questions and report both numbers.
7. `the streaming test is what answers`, the `dash_ctl` leg of `factory/selftest.sh`, is
   locale-dependent and goes red in any scrubbed environment (`LC_ALL=C` rc 0 inside the bound,
   `LC_ALL=C.UTF-8` the rc 124 the leg wants); the pay is the harness's and is measured in full at
   row 7 of the `H-0283` Record below.
8. The carried instruction sections below this tick's Records (issue-366 rows 1-7, the issue-364
   undelivered list and its 56 inherited numbered items, PR #402's three carried review items, the
   hard rules) all stand as written there; this tick touched none of them.

## What this conflict tick pruned, and how to get it back

| Dropped | Where it is whole | Why it is a record and not an instruction |
|---|---|---|
| the issue-375 `H-0284` dispatch head's level-1 heading (line 1 of `git show 292b4d9:NEXT.md`) | that blob | demoted to a Record heading; its state read and all four of its record sections — pushed, spent, owed, pruned — are carried whole below it, so "this dispatch" names one unit |

Everything else either parent had is carried forward unedited, including that Record's owed section
and every instruction section below the Records. A dropped owed line is the worse failure, so the
census behind this table is the raw one — every line of either parent missing from this file — and
it found exactly one: the demoted heading itself.

## Record — the issue-375 third conflict tick (PR #454, base sync onto `b433f01` at head `bcb16c3`; the pass is `H-0284`; its state read is its own clock's)

State read before this file wrote a byte, by this conflict tick, one read per call, raw JSON and
never a `-q` template: **#454** `OPEN` (raw `url` says `/pull/454`), `reviewDecision` **APPROVED**,
`mergeable` **CONFLICTING**, `mergeStateStatus` **DIRTY**, head `bcb16c3` matching this checkout,
`baseRefName` `main`, `isDraft` false. Both type reads ran in the same breath before any write and
both name the pull — `gh pr view 454 --json headRefName` answers `issue-375`, a field an issue cannot
supply, and `gh issue view 454 --json url` answers `/pull/454` — because a bare number read is not a
type test (#2027). `gh pr checks 454` answered 5 pass and 0 fail: `mutation sets`,
`packages (amd64)`, `packages (arm64)`, `shellcheck`, `verify`. `git fetch --prune origin` succeeded
first try and `origin/main` is `b433f01`, the merge of #453, agreed by `git ls-remote origin HEAD` —
so trunk moved again about two hours after the head below, and this pull is on its third base sync.
The allocator, fetched first, answers **`H-0284`** (`REFRESH 26s union`, no `PENDING`, no
`REFRESH-BLIND`), and the taken-set was read the way #451's review taught: `## H-0284` in no entry on
any ref or tree this clone can name, the only `DECISION-ID` rows for `H-0284` are this tick's two, and
the two digits this branch does not carry are settled — `H-0282` is landed on
`refs/remotes/origin/main` through the #453 merge, `H-0283` is landed here. Two slips, both mine,
both disclosed here rather than in a footnote: the first two times I asked the allocator I ran
`bash factory/next-decision-id.sh` with no argument, which is the question about the *other* log and
answered `D-0086` (a third, with `--self-test`, answers `D-0099 selftest-ok`); the flag's own text
names the shape — `--logs PATHS   logs to scan (default: DECISIONS.md when given no argument, all
logs on --all)` — the id this entry carries comes from the explicit `--logs factory/DECISIONS.md`
read, and nothing was written from the bare calls. Re-read all of it before acting: a number here
belongs to the clock it was taken on.

## What this dispatch pushed

- `bba2e6b` (the pre-`H-0284` checkpoint, the rollback target — its subject names `H-0284`, the id
  the allocator printed when that commit was cut) and the merge commit carrying `Decision: H-0284`:
  the base sync of `origin/main` at `b433f01` into `issue-375`. `git merge-tree --write-tree` counted
  five conflicted paths before anything was touched, and every one is resolved in that one commit. An
  auto-merge is a resolution too, so all five were censused against the merge base `feb3b16` after
  the fact, and the census classes every line either parent has that the merged tree lacks:
  - `scripts/check-decisions.py` auto-merged. Trunk's scanner refactor replaces 16 lines the base and
    this branch share — `classify` returning its buckets, the `wrapped` count threaded through
    `CitationVerdict`, the headline reading the same four buckets as `--citations` — and this branch
    had touched this file none of its own, so trunk's scanner ships whole. The same auto-merge took
    trunk's `POINTER_DEBT`, and that is the one auto-merge in a base sync that cannot be stamped: the
    constant is a claim about the corpus, base `feb3b16` read 585, this branch left it there, trunk
    lowered it to 583, so git had one side to take and took the lower one while this tree also carries
    this branch's `H-0281` and `H-0283`. The merged corpus was then re-measured with this entry in the
    file and the composition is on the constant's own line.
  - `.opencode/skills/factory/SKILL.md` auto-merged; trunk's #383 rows on scrubbing dispatch text
    replace one shared line, and that line's lesson is carried by trunk's rewrite of it.
  - `docs/factory.md` is where the ticket's own work lands, and its census is nine lines. All nine
    are trunk's number-bearing wordings of sentences this branch's fix de-numbers — `hold 565
    backticked`, `at this head it reads 539 owing an anchor`, `** 539 of those citations predate`,
    `the four above 535`, `that number is POINTER_DEBT … (re-measured at this head: --citations
    answers debt this gate ratchets on: 535)` — and the merged page carries this branch's wording at
    all nine, which is what #375 was filed for: the page quotes no copy of the constant in prose, and
    the count's only reading is the gate's last line. Trunk's insert in the same region — the
    paragraph that teaches the #374 class, "A sentence that says a line IS there today is itself a
    pointer, and it rots the same way" — was a *different* paragraph and ships beside it, so the merge
    is the first tree where both are true of the same page: the count is a gate output, and a sentence
    naming where lines are today is a pointer that rots.
  - `factory/DECISIONS.md`, four regions, none of them a coin flip. (1) `H-0252`'s `Follow-up:` rows —
    the digit-fold collision #454's first sync met, one round later: each parent had re-aimed one
    identical sentence at the hand-off IT rewrote, 825/929 against 650/754; the merged hand-off is the
    ground and both digits were re-derived from its lines after this file's head was final. (2) the
    tail junction, where this branch's `H-0281` and `H-0283` met trunk's `H-0279` and `H-0282`: union
    by entry id, ascending, with the `H-0280` `Effect:`/`Follow-up:` rows that sat inside that one
    hunk on both sides shown digit-fold identical — three rows, 0 differing — before the union was
    cut, so no seam sentence was preferred over the other's. (3) `H-0252`'s `Refs:` row, a pure
    diff-context artifact: the identical row stands on both sides of that hunk, so ours is kept and
    the duplicate retired rather than shipped twice. (4) seventeen rows in the middle of the log that
    auto-merged to trunk's past tense — `is at` → `was at`, `now reads` → `read on 13 Nov`, `sits at`
    → `was at` — which are trunk's #374 corrections to the present-tense coordinate prose this branch
    carried, and the shipped gate refusing 0 rows outright is the evidence that trunk's side is the
    side that reads. Two further replaced lines are coordinates trunk re-aimed to a single row where
    ours named a range.
  - `NEXT.md`, rewritten at the top, both superseded dispatch heads demoted to Records.
- Ten re-aimed `NEXT.md` pointers in `factory/DECISIONS.md` — `H-0252`'s two, `H-0280`'s two,
  `H-0281`'s two, `H-0282`'s two and `H-0283`'s two — converge on the same two rows of the one merged
  hand-off. Two of the ten arrived from trunk with `H-0282` naming trunk's digits; where a row's digits
  were also a historical claim, the digits that union shipped are kept in the sentence beside the
  pointer rather than overwritten.
- The `NEXT.md` union: this head, two Record demotions (this branch's `H-0283`, trunk's `H-0282`),
  trunk's `H-0279` record — which trunk's own copy had wedged into the middle of another unit's state
  read, and which is seated here as a Record of its own — and the record stack and instruction
  sections below them. Census on this file, digits and whitespace folded: 179 rows this branch added
    over the base, 1 absent; 143 trunk added, 5 absent. The raw census — every line of either parent
  missing from the merged page, added or inherited — answers 7, and all 7 are headings itemised in the
  prune table below, because a fold blind to digits cannot see a re-worded heading. That is not a
  quibble: the added-only census, the one every previous sync on this pull quotes, is structurally
  blind to the four Record headings trunk's copy reworded, since those rows predate the base and only
  their wording moved. Nothing else either parent had is missing, in either direction, and trunk's two
  prune-ledger rows for the issue-429 record — which ours' copy of that record does not carry — are
  grafted into it below under their own provenance heading rather than adjudicated away.

## What this push spent

- Nothing this file asserts before the push. `reviewDecision` read **APPROVED** on #454 at head
  `bcb16c3` before this file wrote a byte, and a base sync moves the head, so whether it survived is
  a post-push read and it lives in the pull-request body. A `REVIEW_REQUIRED` or `DISMISSED` read
  there does not contradict this line, and owed row 1 holds what is owed either way.
- Gate, `check-next-md` and `make verify` verdicts are quoted in the pull-request body from the runs
  on the bytes this file ships, never here.
- Nothing else: no comment posted, no issue touched, nothing merged. The two logs and this page carry
  only what the merge of `b433f01` put there plus this unit's own entry.

## What is still owed (this tick's rows first, then every carried hand-off row, none silent)

1. Re-review of #454: the push spends whatever verdict stood at `bcb16c3`, and the supervisor's poke
   (`factory_request_review`, `FACTORY_REVIEWER_TEAM`) owns the ask — never a unit's verb, because
   `gh pr edit --add-reviewer` exits 1 on the retired `projectCards` field and writes nothing.
2. The two `#425` hazard rows in the instruction sections below still sit on the keyword and still
   refuse `python3 scripts/check-close-verb.py NEXT.md` with rc 1, unchanged by this merge; their
   merged coordinates are held by `H-0252`'s `Follow-up:` row. This tick again declined the two-word
   fix, for the reason the two Records below gave: that row asserts each parent's copy refuses at
   those exact bytes, and editing them here retires an openable claim this merge did not pay for.
3. The allocator journalled `H-0284` to this unit; if the entry never lands, the journal holds an id
   no union can retire — the standing class in row 5(b), which this tick did not fix and did not
   paper over.
4. #375's own deliverable is on these bytes and is owed only the review in row 1: the page quotes no
   copy of the constant, the count's only reading is `python3 scripts/check-decisions.py --citations`
   answering `debt this gate ratchets on:`, and trunk's #374 paragraph now sits beside that rewrite in
   the same page — which is what the ticket asked for and what neither parent could show alone.
5. Two harness defects, both carried untouched because both are outside a conflict tick's remit.
   (a) The allocator-journaling selftest legs inherit `FACTORY_TICK_UNIT` and `FACTORY_LOG` from the
   dispatching shell — every tick-dispatched `make verify` carries them — so the fixture's bare
   allocator leg journals where a bare run must not and reddens `a bare run journals nothing` beside
   its control leg; the fixtures must scrub the two names before launching. Measured both ways on one
   tree: 2 FAIL exported, 0 FAIL and `verify: OK` under `env -u` over the same bytes. (b) The
   allocator's union reads entries on refs only, so a `DECISION-ID` line journalled to a sibling that
   never landed an entry is invisible to it and no `PENDING` fires; the cure stays the journal's
   abandoned line, not a bigger union, and the two `H-0277` rows still stand as claims nothing can
   retire but a landed entry.
6. Carried, and met again by this tick in prose rather than pointers: a base-sync union that automates
   its census must adjudicate on a citation's **anchor**, not on the cited line's bytes — a row the
   other side *rewrote* can be true at the number it names, and a line-comparing sweep would "repair"
   it onto a different row and silently re-aim a pointer into a sentence it never described. The same
   trap has a second jaw: a census that compares only what each side *added* is blind to a row that
   predates the base and had its wording changed, which is where four of this merge's seven drops
   lived. Ask both questions and report both numbers.
7. `the streaming test is what answers`, the `dash_ctl` leg of `factory/selftest.sh`, is
   locale-dependent and goes red in any scrubbed environment — including the `env -i` run row 5 tells
   the next tick to reach for: `LC_ALL=C` finishes inside the leg's wall-clock bound and answers rc 0
   where `LC_ALL=C.UTF-8` answers the rc 124 the leg wants, so its documented escape hatch opens on a
   locale axis rather than the speed axis it was written for. Measured in full, with three timed runs
   and the fixture's byte count, at row 7 of the `H-0283` Record below. The pay is the harness's —
   pin the measured locale across both halves of the leg, or assert a byte-counted cost instead of a
   wall-clock bound — and a conflict tick does not edit `factory/selftest.sh`, so it is named here
   and paid nowhere.
8. The carried instruction sections below this tick's Records (issue-366 rows 1-7, the issue-364
   undelivered list and its 56 inherited numbered items, PR #402's three carried review items, the
   hard rules) all stand as written there; this tick touched none of them.

## What this conflict tick pruned, and how to get it back

| Dropped | Where it is whole | Why it is a record and not an instruction |
|---|---|---|
| the issue-375 `H-0283` dispatch head's level-1 heading (line 1 of `git show bcb16c3:NEXT.md`) | that blob | demoted to a Record heading; its state read and all four of its record sections — pushed, spent, owed, pruned — are carried whole below it, so "this dispatch" names one unit |
| the issue-374 `H-0282` dispatch head's level-1 heading (line 1 of `git show b433f01:NEXT.md`) | that blob | the same demotion on trunk's side, its four record sections carried whole |
| trunk's wording of the shared issue-373 `H-0280` record's heading (line 104 of `git show b433f01:NEXT.md`) | that blob | one heading cannot ship twice; both parents wrote the same record with a different parenthetical and this file carries ours, which names the pull (`#452`) and the pass (`H-0280`) beside the head it synced onto |
| trunk's wording of the shared issue-371 second-tick record's heading (line 393 of `git show b433f01:NEXT.md`) | that blob | same collision, same repair: ours names the merge (`c0de3a4`) and the pass (`H-0276`), and this one predates the base, so the added-only census never saw it |
| trunk's wording of the issue-371-unit, issue-365 `#444`-sync and issue-429 record headings (lines 454, 456 and 458 of `git show b433f01:NEXT.md`) | that blob | three more of the same collision and the same repair: each of ours' three names its pass (`H-0276`, `H-0271`, `3bb1f08`+`3d7c2f7`) where trunk's adds only a blob a reader can open anyway |

Everything else either parent had is carried forward unedited, including both parents' owed sections
and every instruction section below the Records. A dropped owed line is the worse failure, which is
why the census that found these seven is the raw one — every line of either parent missing from this
file, not merely the lines digits-folding called new — and why the two prune-ledger rows trunk's copy
of the issue-429 record carries, which ours' copy does not, are grafted into that Record below under
their own provenance heading instead of being adjudicated away.

## Record — the issue-375 second conflict tick (PR #454, base sync onto `feb3b16` at head `bcb16c3`; the pass is `H-0283`; its state read is its own clock's)

State read by this tick before this file wrote a byte, one field-set per call, raw JSON and never a
`-q` template: **#454** `OPEN`, base `main`, `reviewDecision` **APPROVED**, `mergeable`
**CONFLICTING**, `mergeStateStatus` **DIRTY**, head `5d1e09f` matching this checkout, and
`gh pr checks 454` reporting every row green. Both type reads named the pull before anything was
written: `gh issue view 454 --json url` answers the `/pull/454` url and `gh pr view 454 --json
headRefName` answers `issue-375`, which only a pull request can supply. `origin/main` is `feb3b16`
— the merge of #452 — which moves both decision logs and lands `H-0273`, `H-0278` and `H-0280`.
The allocator, re-run after a fresh `git fetch --prune origin`, answers **`H-0283`**, journalled
`19:48:40Z DECISION-ID H-0283 unit=conflict:454 (REFRESH 11s union)` with no `PENDING`; it answered
`H-0282` on this unit's two earlier reads (`19:42:56Z REFRESH 27s`, `19:46:38Z REFRESH 1s`) because
the sibling that holds `H-0282` had not committed it yet — `refs/heads/issue-374` at `c8c07f6`,
committed `19:47:52Z`, unpushed. That is why this entry is `H-0283` and the checkpoint subject says
`pre-H-0282`: `9161f64` was named for the id that stood when it was made, and a pushed branch is
not re-worded here.
`python3 scripts/check-decisions.py` on the merged bytes answers **343 entries / 1126
citations read / 585 owing an anchor / 0 refused, rc 0**, and `--citations` ends
`debt this gate ratchets on: 585` — the constant both parents stood on, unmoved, because every
citation this entry adds carries an anchor or a revision and none joins the unanchored set.
`bash scripts/check-next-md.sh` answers ok on all three legs. `make verify` ran both ways on these
bytes and **no red leg is a byte written here**: under `env -i` it ends `860 passed, 1 failed` and in
this tick's own exported environment `860 passed, 2 failed`, both `make: *** [Makefile:111: verify]
Error 1`. The two that go red only in the exported environment are #425's allocator-journaling
fixtures, cause-and-control on trunk's own bytes in the `H-0281` record above. The one that goes red
only under `env -i` is `the streaming test is what answers`, whose wall-clock bound this box's locale
turns off — measured both ways, and owed as row 7 below, because pinning a locale in
`factory/selftest.sh` is not a conflict tick's file. The two numbers are quoted in the pull-request
body from the runs whose logs are named there, never here: a page that printed its own gate's verdict
would be quoting a run of other bytes.
Re-read all of it before acting: a number here belongs to the clock it was taken on.

## What this dispatch pushed

- `9161f64`, the checkpoint and this sync's rollback target, and one merge commit carrying
  `Decision: H-0283` — the merge of `origin/main` at `feb3b16` into `issue-375`. This unit pushes
  and never merges; #454 lands on `main` only by the supervisor's merge of what it re-reads as
  approved, mergeable, clean and green.
- `docs/factory.md` and `scripts/check-decisions.py`, both auto-merged with no conflict: the page's
  citation rows and `POINTER_DEBT = 585` are byte-identical to `origin/main` here, so #375's
  pointers and #373's ride the same constant, which is what let the seam below be adjudicated on
  row identity instead of on a guess about which side pinned 586.
- The `factory/DECISIONS.md` union: trunk's `H-0273`, `H-0278` and `H-0280` seated by entry id
  between this branch's `H-0275` and `H-0281`, and `H-0252`'s `Follow-up` row — the one seam whose
  only difference between the parents was digits — taken as an alignment, then re-aimed. 273
  headings on the shipped bytes including this unit's own, unique and ascending; zero parent-added
  rows absent against either parent with digits and whitespace folded.
- The `NEXT.md` union: this head, two Record demotions, three carried records from trunk, this
  branch's delivery-unit record, and the shared issue-371 hand-off below them — a
  1038-line body byte-identical in both parents (`diff` empty), with its one two-wording
  heading itemised in the prune table.
- Six re-aimed `NEXT.md` pointers in `factory/DECISIONS.md` — `H-0252`'s two, `H-0280`'s two and
  `H-0281`'s two — which converge: all six name the same two rows of the one merged hand-off.

## What this push spent

- Nothing this file can assert before the push. `reviewDecision` read **APPROVED** on #454 at head
  `5d1e09f` before this file wrote a byte, and a base sync moves the head, so whether it survived is
  a post-push read and it lives in the pull-request body -- journalled nowhere, because a unit writes
  no line in the supervisor's journal and `factory.sh` has no review marker to ask for. A
  `REVIEW_REQUIRED` read there does not contradict `HEAD~1`'s `APPROVED`; owed row 1 has the read.
- Gate and `check-next-md` verdicts are quoted in the pull-request body from the runs on the bytes
  this file ships, including `make verify`'s two numbers under the two environments quoted in the
  state read above.
- Nothing else: no comment posted, no issue touched, nothing merged. The two logs and the page carry
  only what the merge of `feb3b16` put there plus this unit's own entry.

## What is still owed (this tick's rows, then everything the carried hand-offs state)

1. Nothing on #375's side is left open except the review, and the post-push read carries what this
   file refused to predict: `hermesvicowa`'s row, filed against `5d1e09f6`, reads **DISMISSED**,
   `reviewDecision` empty and `mergeStateStatus` **BLOCKED** at head `95ddb50`. The re-review ask is
   owed and unpaid -- the supervisor's poke (`factory_request_review`, `FACTORY_REVIEWER_TEAM`), never
   a unit's verb: `gh pr edit --add-reviewer` exits 1 on `projectCards` and writes nothing.
2. #375 owes #373 nothing new on the citation class: `docs/factory.md` auto-merged clean here, so
   `python3 scripts/check-decisions.py --citations docs/factory.md` still answers the rows trunk
   measured. Re-run it if another page edit lands first — a conflict on that paragraph is the only
   way those pointers break.
3. `H-0169`'s remaining bare rows point at the scripts and stay bare; the instrument the issue-429
   hand-off owes (its item 55) covers them. The docs row, the one this ticket proved dead, is paid.

4. Two harness defects the previous sync measured and did not touch, both outside a conflict tick's
   remit, carried here verbatim from the Record below so neither goes quiet while the branch is open.
   (a) the allocator-journaling selftest legs inherit `FACTORY_TICK_UNIT` and `FACTORY_LOG` from the
   dispatching shell -- every tick-dispatched `make verify` carries them (this tick's `env` read
   names both) -- so the fixture's bare allocator leg journals where a bare run must not and
   reddens `a bare run journals nothing` beside its control leg; the fixtures must scrub the two
   names before launching. Measured both ways on one tree: 2 FAIL exported, 0 FAIL and
   `verify: OK` under `env -u` over the same bytes.
   (b) the allocator's union reads entries on refs only, so a `DECISION-ID` line journalled to a
   sibling that never landed an entry is invisible to it and no `PENDING` fires: `H-0277`
   journalled 16:09:10Z to `issue-371`, absent from every ref and tree this clone can see, re-read
   0s after a fresh full fetch answering the same id with no PENDING. `H-0278` is the yield this
   bought; the journal also has no abandoned line, so the two `H-0277` rows this tick journalled
   and will not land stand as claims nothing can retire but a landed entry.
5. Row 4(b), re-measured by this sync, now with a control that separates the two cases it confuses:
   `H-0277`'s two journal rows are still journalled-unlanded and this branch will never land them,
   while `H-0279` — the digit this branch's union answered *past* on its last sync — is landed on
   `origin/issue-374`. So one journal holds two rows of a class a reader cannot grade: an id issued
   and never landed, next to an id the union was right to skip. The cure stays the journal's
   abandoned line (4(b)), not a bigger union.
6. This tick's own addition to #381's cure, measured rather than argued: a base-sync union that
   automates the drift census must adjudicate on a citation's **anchor**, not on the cited line's
   bytes. Two of the six whole-line mismatches counted above were rows the other side had *rewritten*
   — one coordinate moved inside them — and both were already true at the number they name. A
   line-comparing sweep would "repair" those onto a different row and silently re-aim a pointer into
   a sentence it never described.
 7. `the streaming test is what answers` (`factory/selftest.sh:17815`, the leg `dash_ctl`) is
    locale-dependent and goes red in any scrubbed environment -- including the `env -i` run row 4
    tells the next tick to reach for. Its control asks the superseded whole-value substitution to
    *still be running* when `timeout 1` fires (`rc_ctl` must be 124), and that substitution is
    multibyte work: on one box, on the leg's own fixture rebuilt outside the selftest (1,300
    four-EM-DASH lines, 137,835 bytes), `LC_ALL=C` answers **rc 0** -- it finishes inside the bound --
    where `LC_ALL=C.UTF-8` and `LC_ALL=en_US.UTF-8` both answer **rc 124**, the verdict the leg wants;
    three timed runs under `en_US.UTF-8` measure 16.77 s, 16.76 s, 17.33 s against the 14.66 s its own
    comment records. `env -i` carries no `LANG` and no `LC_*`, so bash falls back to the POSIX locale,
    the substitution goes byte-wise, and the leg's own documented escape hatch opens --
    `factory/selftest.sh:17799` "the bound has gone slack on some faster machine and the first
    assertion has stopped meaning anything; the leg says so rather than going quietly green" -- on a
    locale axis rather than the speed axis it was written for. Nothing about the tree: the leg sources
    `$HERE/common.sh` and writes its own fixture, and the only files this merge moves are `NEXT.md`,
    `docs/factory.md`, `factory/DECISIONS.md` and `scripts/check-decisions.py`. The pay is the
    harness's -- pin the measured locale across both halves of `dash_ctl`, or assert a byte-counted
    cost instead of a wall-clock bound -- and a conflict tick does not edit `factory/selftest.sh`, so
    it is named here and paid nowhere.

## What this conflict tick pruned, and how to get it back

| what this file dropped | where the bytes are | why |
|---|---|---|
| the issue-375 `H-0281` dispatch head's level-1 heading (`git show 5d1e09f:NEXT.md` line 1) | that blob | demoted to a Record heading; its state-read paragraph and both of its record sections are carried whole below it, so "this dispatch" names one unit |
| the issue-373 `H-0280` dispatch head's level-1 heading (`git show origin/main:NEXT.md` line 1) | that blob | the same demotion on trunk's side; its state read and all four of its record sections — pushed, spent, owed, pruned — are carried whole below a Record heading, so the added-line census against trunk reports this one line and nothing else |
| trunk's wording of the shared issue-371 record's heading (`## Record - the issue-371 second conflict tick (PR #450, base sync against `956f8fc`; …)`, line 226 of `git show origin/main:NEXT.md`) | that blob | one heading cannot ship twice; both parents wrote the same record with a different parenthetical and this file carries ours, which names the merge (`c0de3a4`) and the pass (`H-0276`) while trunk's adds only the base it synced against (`956f8fc`) |

Everything else is carried forward unedited, including both parents' records, both parents' owed
sections and the whole shared issue-371 hand-off. A dropped owed line is the worse failure, and all
three drops above are headings.

## Record — the issue-374 #453 final sync (PR #453, base sync onto `feb3b16` at head `c8c07f6`; the pass is `H-0282`; its state read is its own clock's, and the pull it names is #453)

State read before this file wrote a byte, by this conflict tick, one read per call, raw JSON and
never a `-q` template: **#453** `OPEN`, base `main`, `headRefOid` `a880e51` matching this checkout,
`mergeable` **CONFLICTING**, `mergeStateStatus` **DIRTY**, `reviewDecision` **APPROVED** — the one
row `hermesvicowa`, `APPROVED`, filed against `a880e51` at `2026-11-18T19:10:07Z` with two
non-blocking prose findings — and all five `statusCheckRollup` rows `COMPLETED` (`verify`,
`shellcheck`, `mutation sets`, both packages). `origin/main` is `feb3b16` — PR #452 merged, carrying
`H-0273`, `H-0278` and `H-0280 —` read from the `git fetch --prune origin` this tick ran before the
allocator. The allocator answers **`H-0282`** on a fetched `REFRESH 1s` union, no `PENDING`; `supervisor.log` also shows
`H-0282` journalled 18:38:13Z to `unit=conflict:454 branch=issue-375`, whose remote head `5d1e09f` held no
such heading at read time — journalled claims stay inert until an entry lands (`D-0086`, seven-plus lines
since 16 Nov); #442: journaling is not consuming. Re-read all before acting: a number is clock-scoped.

## What this dispatch pushed

- One merge commit (its sha is in the ref this push carries, never in this line, which is written
  before the commit exists) carrying `Decision: H-0282`, preceded by its pre-entry checkpoint `cb1d0ff` (the
  rollback target): `git merge origin/main` over three conflicted paths — four hunks in
  `factory/DECISIONS.md` (two junctions unioned by ascending entry id: `H-0273` seated before
  `H-0274` between the shared `H-0272` and `H-0276`, and the tail junction seating `H-0278`,
  `H-0279`, `H-0280` in id order above a blank line restored before the `H-0280` heading;
  `H-0271`'s `Why:` seam row adjudicated by opening the merged `scripts/check-decisions.py:771` —
  this side's prose-anchor form survives any digit the constant ships, trunk's `956f8fc` pin's
  parenthetical names the 585 that branch landed, which is a sentence about that merge and not
  about this tree; `H-0252`'s `Follow-up:` `NEXT.md` coordinate pair re-aimed after this file
  settled, by the line the shipped gate names, never by an offset), one hunk in
  `scripts/check-decisions.py` (`POINTER_DEBT`: this side's 584 taken as the lower side of the
  ratchet, then re-measured on the merged corpus at 583, the composition beside it), and one in
  this file (both units had rewritten the top; resolved as this top with both superseded dispatch
  heads demoted to Records below, state reads intact, every owed row they carried repeated in the
  live owed section).
- The gate then refused 4 anchor rows on these bytes and every refusal was paid by re-aiming onto
  the line each refusal named: the `H-0252` and `H-0280` `Follow-up` pairs settled onto those lines.
- The added-line census (digits and ranges folded, multiset diff against the merge base, over the
  paths either parent touched): five line-classes replaced by design — the two dispatch h1s renamed to Record headings, the one `H-0271` seam `Why:` row adjudicated against the merged constant, the two superseded `POINTER_DEBT` comment lines folded into the one 583 row — and zero lines lost that were not replaced on purpose
- `python3 scripts/check-decisions.py` rc 0 (343 entries, 1132 citations, 583 debt,
  0 in every must-be-zero bucket) and `bash scripts/check-next-md.sh` OK on the bytes this file
  ships; `make verify` and `--self-test` verdicts ship in the pull-request body from the runs on
  the commit.
- No merge, no label, no reviewer ask, no issue write; the queue belongs to the supervisor, and
  the reviewer re-ask after a push is its poke, not a unit's verb.

## What this push spent

- #453 carried **APPROVED** at `a880e51` — read before this merge began, quoted above, not lifted
  from a dump. This push moves the head, so it spends that approval: the row goes `DISMISSED` on a
  fast-forward base-sync push with no exit-status sign and no comment (measured as a class on
  #269), and the post-push read in the pull-request body carries the verdict as it then stands.
  What is owed the moment the push lands is a re-review ask, which goes out through the
  supervisor's poke — `factory_request_review` is the only verb that addresses
  `FACTORY_REVIEWER_TEAM`, because `gh pr edit --add-reviewer` dies on this account's retired
  `projectCards` field and writes nothing.
- The reviewer's two non-blocking findings ride this branch, and this tick pays them in a separate
  commit after the merge, not inside it: the `docs/factory.md` example row whose second cell uses
  backslash-escaped backticks inside a single-backtick span (CommonMark does not escape inside a
  code span; the shipped repair is the double-backtick form the cell's own first example uses),
  and the demoted `H-0279` record's line naming `#435` for the class the gate enforces, which the
  review measured as belonging to `#374` — corrected in place there, with this row as the witness,
  because the record's own opening already narrates the referent reassignment and only that one
  number was false.

## What is still owed (this tick's rows first, then every carried hand-off row, none silent)

1. Re-review of #453: the push spends the standing approval as a base sync does; the supervisor's
   poke owns the ask, and nothing in this tree re-asks it.
2. The allocator journalled `H-0282` to this unit; if the entry never lands, the journal holds an
   id no union can retire (carried shape of the issue-373 row 3(b) below — this tick's own
   `DECISION-ID` line joins that class the moment it is written).
3. Carried from the issue-373 hand-off (its live top, now `H-0280`'s Record): `H-0169`'s remaining
   bare rows point at the scripts and stay bare; the instrument the issue-429 hand-off owes (its
   item 55) covers them. The docs row, the one that ticket proved dead, is paid.
4. Carried, both still untouched here (outside a conflict tick's remit): (a) the
   allocator-journaling selftest legs inherit `FACTORY_TICK_UNIT` and `FACTORY_LOG` from the
   dispatching shell, so the fixture's bare allocator leg journals where a bare run must not and
   reddens `a bare run journals nothing` beside its control leg — the fixtures must scrub the two
   names before launching (measured both ways on one tree: 2 FAIL exported, 0 FAIL and
   `verify: OK` under `env -u` over the same bytes); (b) the allocator's union reads entries on
   refs only, so a `DECISION-ID` line journalled to a sibling that never landed an entry is
   invisible to it and no `PENDING` fires — `H-0277` journalled 16:09:10Z to `issue-371`, absent
   from every ref and tree this clone can see; the cure stays the journal's abandoned line, not a
   bigger union, and the two `H-0277` rows stand as claims nothing can retire but a landed entry.
5. Carried: the journal now also holds this tick's `H-0282` line beside the `H-0279`/`H-0277`
   pair — an id issued and (until this push lands) not yet landed; a reader cannot grade the two
   cases from the journal alone.
6. Carried, the census lesson: a base-sync union that automates the drift census must adjudicate
   on a citation's **anchor**, not on the cited line's bytes — a row the other side *rewrote* can
   be true at its own number, and a line-comparing sweep would "repair" it onto a different row
   and silently re-aim a pointer into a sentence it never described.
7. The carried instruction sections below this tick's Records (issue-366 rows 1-7, the issue-364
   undelivered list and its 56 inherited numbered items, PR #402's three carried review items, the
   hard rules) all stand as written there; this tick touched none of them.

## What this conflict tick pruned, and how to get it back

| Dropped | Where it is whole | Why it is a record and not an instruction |
|---|---|---|
| the two `# NEXT` level-1 heading lines of the superseded dispatch heads (this branch's `H-0279` top and trunk's `H-0280` top), renamed to `## Record` lines | the two Record headings immediately below, bodies carried verbatim | one hand-off per branch — the gate refuses a second level-1, and the live hand-off is this file's top now |

Everything else is carried forward unedited; nothing else was dropped. A dropped owed line is the
worse failure — which is why the live owed section above restates every owed line either head
carried, and the Records keep their state reads whole for their own clocks.

## Record — the issue-375 conflict tick (PR #454, base sync onto `c0de3a4` at head `5d1e09f`; the pass is `H-0281`; its state read is its own clock's)

State read before this file wrote a byte, by this conflict tick, one read per call, raw JSON and
never a `-q` template: **#454** `OPEN`, `reviewDecision` empty (no review has ever been filed on
this branch), `mergeable` **CONFLICTING**, `mergeStateStatus` **DIRTY**, remote head `70b3604`
matching this checkout's pre-checkpoint head, checks green. `origin/main` is at `c0de3a4`. The
allocator, fetched first, answered **`H-0281`** (journal line `18:14:29Z DECISION-ID H-0281
unit=conflict:454 (REFRESH 0s union)`); re-run it before honoring this id on any successor's
authority. Re-read all of it before acting: a number here belongs to the clock it was taken on.

## What this dispatch pushed

- `c7cca17` (the pre-`H-0281` checkpoint, the rollback target) and the merge commit carrying
  `Decision: H-0281`: `git merge origin/main` over 2 conflict regions in `factory/DECISIONS.md`
  — one `Follow-up` row whose two `NEXT.md:` coordinates each parent had re-aimed to its own
  tree's numbers (digit-only: `389`/`493` on this branch, `349`/`453` on trunk) adjudicated as
  one row at the merged hand-off's measured lines, and one tail junction where this branch's
  `H-0275` seats between trunk's `H-0272` and `H-0276` because
  `scripts/check-decisions.py:236` "%s: %s is out of order (%s came before it); the log is a
  history" leaves exactly one seat per id. `docs/factory.md` is untouched by trunk since the
  base, so this branch's `H-0275` pass rides whole; `factory/selftest.sh` is untouched by this
  branch, so trunk's version rides whole.
- `NEXT.md` rewritten at the top for this tick with both superseded dispatch heads demoted to
  Records, newest first; trunk's record line "the previous dispatch head renamed to a Record so
  'this dispatch' never means three units at once" has aged into four — that is what Records
  are for, and no carried section was edited.
- No merge, no label, no reviewer ask, no issue write; the queue belongs to the supervisor, and
  the reviewer ask is its poke (`factory_request_review`), not a unit's verb.

## What this push spent

- Nothing spent on review: `reviewDecision` read empty before this push, so there was no
  approval for the push to spend; the supervisor's poke does the ask. The post-push read is
  quoted in the pull-request body.
- Gate and `check-next-md` verdicts are quoted in the pull-request body from the runs on the
  bytes this file ships.
- `make verify` on these bytes: **860 passed, 2 failed**, and both red legs are owed item 4's
  tracked `#425` — allocator legs that read the supervisor's exported `FACTORY_TICK_UNIT`,
  deterministic, this page holding the A–H matrix that named the scrub. This tick re-derived the
  fingerprint on a tree the matrix had not run: trunk's own `c0de3a4` bytes under the same tick
  environment read 860 passed / 2 failed with the identical two legs, and scrubbing the export
  reads green on both trees — so this merge regressed nothing and the red is a property of the
  environment a tick runs in, not of the bytes. One price of `#425` this tick pays visibly: legs
  journalling through the inherited `FACTORY_LOG` wrote roughly twenty fixture `DECISION-ID`
  rows into this tick's own journal (`factory/log/20261118T181224Z.conflict:454.log`) — inert to
  the end-of-tick comparison, which reads headings and asks whether an allocation was journalled,
  never the reverse — and they are disclosed here, not edited out: that journal is the
  supervisor's record, not this unit's.


## Record — the issue-373 conflict tick (PR #452, base sync onto `c0de3a4` at head `ab00909`; the pass is `H-0280`; its state read is its own clock's)

State read by this tick, one field-set per call, raw JSON and never a `-q` template -- and read twice,
because this file's own closing rule says a number belongs to its clock: the opening read, taken before
the `c855b11` checkpoint at `17:43:17Z`, answered `reviewDecision` the **empty string**; the re-read
before this push, at `17:51:36Z`, answers **`APPROVED`** on the same head `ab00909`, and nothing this
unit did can approve a pull request -- the remote head has not moved all session. So a verdict arrived
while the merge was in flight, and the second read is the one that ships: **#452** `OPEN`, base `main`,
`reviewDecision` **`APPROVED`** -- a standing approval which this push spends, so the re-review ask is
owed and the supervisor's poke owns it -- `mergeable` **`CONFLICTING`**, `mergeStateStatus` **`DIRTY`**,
head `ab00909` matching this checkout, five `statusCheckRollup` rows every one `COMPLETED`/`SUCCESS`.
Both type reads named the pull before anything was written, per the rule that a bare number read is not
a type test. `origin/main` is `c0de3a4` -- PR #450 merged, carrying `H-0272` and `H-0276` -- agreed by
`git ls-remote origin HEAD` and by the fetched ref, after one `git fetch` died at SSH-22 with a connect
timeout and the identical retry succeeded (#417's shape, so a refused fetch is a retry candidate, never
a verdict). The allocator answers **`H-0280`**, journalled to this unit with no `PENDING`: `H-0279` is
landed on `refs/remotes/origin/issue-374` and nothing elsewhere journals 280. Re-read all of it before
acting: a number here belongs to the clock it was taken on, and this file has the two clocks to prove it.

## What this dispatch pushed

- `c855b11` (the pre-`H-0280` checkpoint, the rollback target) and the merge commit carrying
  `Decision: H-0280`. `git merge-tree --write-tree` counted the damage before anything was touched:
  two files, three hunks — this file's head, one coordinate pair inside `H-0252`'s Follow-up row, and
  this log's tail junction. The junction unions four entries by id (`H-0272`, `H-0273`, `H-0276`,
  `H-0278`); its seam rows (`H-0271`'s `Why:`, `Effect:` and `Checkpoint:`) are taken from **this**
  side, because the merged scanner carries this branch's `POINTER_DEBT` constant at 585 and only
  ours' revision-pinned form of that constant's citation is a true sentence about it — main's bare
  form quotes 586 at a line the merged tree reads 585. That is the shape #451's review refused,
  adjudicated here by opening the line rather than by preferring a side.
- The `H-0252` Follow-up coordinate pair was re-aimed *after* this head was written, because the head
  is what moves the file it cites; the gate named the landing lines and both digits were checked
  against them, never inferred from an offset.
- The census the gate cannot do, re-run on the bytes this file ships: of the 1117 citations the gate
  counts, 1028 are file-shaped and every one of them whose target is in the tree was compared
  line-for-line against the blob of the side that wrote its row. 1017 identical, 9 landing past EOF at both
  authors identically (pre-existing, untouched), and 2 whole-line mismatches (`H-0231` and `H-0252`, both
  naming one `factory/DECISIONS.md` `Effect:` row) shown to be a *rewrite*, not a move — that row's only
  change is one `factory/selftest.sh` coordinate main's own `H-0276` pass moved by the eleven lines its
  guard adds, with both anchors verified on the merged line. Result: **0 bare (unanchored) rows drifted
  by this merge**, which is the class the gate is blind to because it never opens an unanchored pointer.
- Two self-citations re-seated onto the row each one cites, by the rule `H-0276` set when the same
  anchor came to sit on three rows: the cited row is what the sentence asks a reader to open, so the
  citation moves there rather than staying on the first or last occurrence. `H-0271` onto `H-0268`'s
  `Why:` row, `H-0273` onto `H-0264`'s `Effect:` row.
- This `NEXT.md`, rewritten at the top; both superseded dispatch heads demoted to Records below with
  their state reads intact, every owed row they carried repeated verbatim in the live owed section
  above, and main's prune of the issue-365 `#444`-sync record kept exactly as main wrote it (21 base
  lines gone by design; its own recovery heading is the `## Record — the issue-365 #444-sync dispatch`
  line below this file's Records, which points at `git show a8bcb5a:NEXT.md`).
- Gate and hand-off verdicts on the bytes this file ships, quoted from those runs in the
  pull-request body: 340 entries, 1117 citations, 585 owing an anchor, 0 refused; `scripts/check-next-md.sh` OK.

## What this push spent

- #452 carries **`APPROVED`**, read at `17:51:36Z` and not lifted from a dump, where this tick's opening
  read had answered the empty string at `ab00909` -- the verdict arrived mid-merge, and the later read is
  the live one. This push spends it: a base sync moves the head and the verdict row goes `DISMISSED`,
  with no exit-status sign and no comment (measured as a class on #269). So a re-review ask is owed the
  moment the push lands, and it goes out through the supervisor's poke -- `factory_request_review` is the
  only verb here that addresses `FACTORY_REVIEWER_TEAM`, because `gh pr edit --add-reviewer` dies on this
  account's retired `projectCards` field and writes nothing.

## What is still owed (issue-373 rows, then everything the carried hand-offs state)

1. Nothing on #373's side is left open except the review: this push spent the `APPROVED` that stood at
   `ab00909`, as a base sync does, and the supervisor's poke owns the re-review ask. If the merge lands
   after another page edit, re-run `python3 scripts/check-decisions.py --citations docs/factory.md` and expect
   59 rows plus whatever the other edit added — the neutral rewrite means a conflict on this paragraph is
   the only way this unit's pointers break.
2. `H-0169`'s remaining bare rows point at the scripts and stay bare; the instrument the issue-429
   hand-off owes (its item 55) covers them. The docs row, the one this ticket proved dead, is paid.
3. Two harness defects the previous sync measured and did not touch, both outside a conflict tick's
   remit, carried here verbatim from the Record below so neither goes quiet while the branch is open.
   (a) the allocator-journaling selftest legs inherit `FACTORY_TICK_UNIT` and `FACTORY_LOG` from the
   dispatching shell -- every tick-dispatched `make verify` carries them (this tick's `env` read
   names both) -- so the fixture's bare allocator leg journals where a bare run must not and
   reddens `a bare run journals nothing` beside its control leg; the fixtures must scrub the two
   names before launching. Measured both ways on one tree: 2 FAIL exported, 0 FAIL and
   `verify: OK` under `env -u` over the same bytes.
   (b) the allocator's union reads entries on refs only, so a `DECISION-ID` line journalled to a
   sibling that never landed an entry is invisible to it and no `PENDING` fires: `H-0277`
   journalled 16:09:10Z to `issue-371`, absent from every ref and tree this clone can see, re-read
   0s after a fresh full fetch answering the same id with no PENDING. `H-0278` is the yield this
   bought; the journal also has no abandoned line, so the two `H-0277` rows this tick journalled
   and will not land stand as claims nothing can retire but a landed entry.
4. Row 3(b), re-measured by this sync, now with a control that separates the two cases it confuses:
   `H-0277`'s two journal rows are still journalled-unlanded and this branch will never land them,
   while `H-0279` — the digit this branch's union answered *past* on its last sync — is landed on
   `origin/issue-374`. So one journal holds two rows of a class a reader cannot grade: an id issued
   and never landed, next to an id the union was right to skip. The cure stays the journal's
   abandoned line (3(b)), not a bigger union.
5. This tick's own addition to #381's cure, measured rather than argued: a base-sync union that
   automates the drift census must adjudicate on a citation's **anchor**, not on the cited line's
   bytes. Two of the six whole-line mismatches counted above were rows the other side had *rewritten*
   — one coordinate moved inside them — and both were already true at the number they name. A
   line-comparing sweep would "repair" those onto a different row and silently re-aim a pointer into
   a sentence it never described.

## What this conflict tick pruned, and how to get it back

| Dropped | Where it is whole | Why it is a record and not an instruction |
|---|---|---|
| the fourth dispatch's `# NEXT` level-1 heading and everything under it down to its pruned table (lines 1-83 of `git show c855b11:NEXT.md`) | that blob | its state read's clock expired the moment #450 merged as `c0de3a4` and its pushed list describes commits this merge now sits on top of; its three owed rows are carried verbatim into the live owed section above, so nothing is dropped but the clock |
| the issue-371 second conflict tick's `# NEXT` level-1 heading (line 1 of `git show origin/main:NEXT.md`) | that blob | the unit merged, so the file's live hand-off is this branch's; its state read and both of its record sections are carried below under a Record heading so that "this dispatch" names one unit |
| this branch's `## Record - the issue-365 unit` heading (line 115 of `git show c855b11:NEXT.md`), which sat above content main pruned away | that blob, and main's own recovery heading below | main pruned the same record to a one-line pointer and its prune table carries the reason; keeping ours would leave a heading over nothing, which is the shape that hides a lost section |

Everything else is carried forward unedited. A dropped owed line is the worse failure - the three
drops above are headings and expired state reads, and the only owed lines they restated are itemised
as carried above.

## Record — the issue-374 #453 base sync onto `c0de3a4` (H-0279 dispatch at head `a880e51`; its state read is its own clock’s, and its "#435" naming is the dispatch ticket’s referent — the delivered pull is #453)


State read before this file wrote a byte, by this conflict tick, one read per call, raw JSON and
never a `-q` template: **#453** `OPEN` (raw `url` says `/pull/453`,
and `headRefName` says `issue-374` — both type reads run before any write, because the dispatch
ticket named this delivery "#435" and #435 is the merged issue-368 pull: the referent had been
reassigned before dispatch, the named class of the inherited row 81), `reviewDecision` **empty**,
remote head `243ec7b` matching this checkout, `mergeable` **CONFLICTING**, `mergeStateStatus`
**DIRTY**. `origin/main` was at `c0de3a4` (PRs #444/#450/#452/#453 merged,
carrying `H-0270`–`H-0278`). The allocator, fetched first and re-run at 0s REFRESH after the merge,
answers **`H-0279`** with no `PENDING` and no `REFRESH-BLIND`; re-run it before honoring this id on
any successor's authority. Re-read all of it before acting: a number here belongs to the clock it
was taken on.

## What this dispatch pushed

- One merge commit (its sha is in the ref this pushes, and in the pull-request body's read-back)
  carrying `Decision: H-0279`, preceded by its pre-entry checkpoint `fdc0417` (the rollback
  target): `git merge origin/main` over three conflicted paths — 14 hunks in
  `factory/DECISIONS.md` (12 digit folds, the `POINTER_DEBT` `Effect:` row retired to trunk's
  revision-pinned spelling by `H-0271`'s precedent, the tail junction seating `H-0274`), one hunk
  in `docs/factory.md`'s shape table resolved as the union (both classes, #365's boundary and
  #374's current-state refusal), and one in `scripts/check-decisions.py` where trunk's #365/#369
  rewrite of `scan_citations` had landed *inside the body* #374's matcher was inserted into: trunk's
  scan body taken whole, the adjacency matcher and fifth bucket grafted back onto trunk's loop
  variables, the `now` key beside `near`/`wrapped` in the result dict. `--self-test` answers 78
  must-fail / 47 must-pass rc 0 — the union of both parents' legs, the proof both rule families
  fire on one body.
- The gate then refused 29 on these bytes and every refusal was paid: 27 anchored drifts re-aimed
  onto the line each refusal named (one triple-cite adjudicated by context where two lines carry the
  quote, one 4-line range collapsed onto the single line the quote moved to), and the two rows
  trunk had re-aimed into the present tense (`H-0139`, `H-0146`) past-tensed with the date per the
  shipped gate's own offered repair — the first #374-class refusals this repository shipped against
  its own log, and they join the counted debt as past coordinates.
- The added-line census (digits and ranges folded, multiset diff against the merge base, all six
  paths either parent touched): 0 parent-added lines absent except the three disclosed in `H-0279`
  — the superseded `Effect:`-row rewrite, the 11 `NEXT.md` rows this file's own contract prunes, and
  two census artifacts (a fold that strips leading tabs, a key pair split across dict lines) whose
  "absent" readings were refuted by opening the named lines. Both logs unique and ascending after
  the union: 268 `## H-` before this entry, `H-0274` seated between `H-0272` and `H-0276`.
- `POINTER_DEBT` lowered 586 to 584 with the fall's composition beside it in the same commit; the
  27 re-aims moved no debt (anchored throughout) and this tick's entry carries only anchored or
  revision-pinned citations. `python3 scripts/check-decisions.py` rc 0 (339 entries, 1109 citations,
  584 debt, 0 in every must-be-zero bucket) on the bytes this file ships; `make verify` and
  `make check-mutations` verdicts ship in the pull-request body from the runs on the commit.
- This `NEXT.md`, rewritten at the top; every instruction section below carried forward unedited,
  the superseded dispatch head renamed to a Record so "this dispatch" never means three units at
  once.
- No merge, no label, no reviewer ask, no issue write; the queue belongs to the supervisor, and
  the reviewer re-ask after a push is its poke, not a unit's verb.

## What this push spent

- The push moves nothing that was reviewed — #453 carried no review on head `243ec7b` at the read
  above — so what it owes is a first review, which is the supervisor's poke
  (`factory_request_review`), not a unit's ask. The post-push read is quoted in the pull-request body.
- #374's whole point is that the gate refuses present-tense coordinate prose (corrected per the review of #453: #435 is the merged issue-368 exec-bit pull) — and the first two
  rows it refused on this merge were this repository's own, in `H-0139` and `H-0146`, repaired by
  the tense change the entry itself teaches. A branch that ships a rule and then violates it two
  entries later is the reason the rule is a gate and not a convention.

## Record - the issue-373 fourth dispatch (PR #452, base sync against `956f8fc` at head `a1b1b96`; its state read is its own clock's)

State read before this file wrote a byte, by this conflict tick, one read per call, raw JSON and
never a `-q` template: **#452** `OPEN`, `reviewDecision` **APPROVED**, `mergeable` **CONFLICTING**,
`mergeStateStatus` **DIRTY**, head `dbbb23a` matching this checkout, checks green. `origin/main` was
at `956f8fc` (PR #444 merged, carrying `H-0270`/`H-0271`). The allocator answered **`H-0277`** at
16:28Z, but a re-read after a fresh fetch found the supervisor journal had already issued `H-0277`
to `issue-371` (16:09:10Z) with no entry landed anywhere - so this unit yields the id and this entry
is written `H-0278` (`H-0169` the precedent). Re-read all of it before acting: a number here belongs
to the clock it was taken on.
## What this dispatch pushed

- `4b432f5` (the pre-`H-0277` checkpoint, the rollback target - its subject names the id the
  allocator printed when it was cut, this entry explains why the yield cannot reach it) and
  the merge commit carrying `Decision: H-0278`: three files conflicted. `factory/DECISIONS.md` resolved
  by entry id - tail union
  seating `H-0270`, `H-0271` under this branch's `H-0273`, the two seam rows adjudicated on the merged
  tree (the `H-0264` constant row keeps this branch's revision pin `0a2c093:scripts/check-decisions.py:770`
  over main's `30aa557:...:595` - both openable at their own revisions, ours is the one `H-0273` paid
  for); `scripts/check-decisions.py` keeps main's 586 ledger line, re-measured against the merged
  corpus in `H-0278`; `NEXT.md` rewritten here as the union of both hand-offs. The content pass
  re-aimed the four pointers this union moved - the two `H-0252` `NEXT.md` anchors (302->436 and
  406->540, each twice moved by this file's own growth above them, the shipped numbers final), the
  `H-0273` log self-pointer (14137->14172, disambiguated against the two lines now
  carrying the quote), and `H-0271`'s `POINTER_DEBT = 586` constant citation, revision-pinned to
  `956f8fc:` where that line is true - and measured the whole bare ledger for drift: all 585 open
  rows compared line-for-line against the tree their author shipped, `0` drifted, none new, the one
  departing row the `H-0169` anchor payment above. The gate on these bytes reads `335 entries /
  1102 citations / 585 owing an anchor / 0 refused`, exit 0, debt constant `585`; `make verify` is
  green here after the two selftest legs were traced to this tick's own exported
  `FACTORY_TICK_UNIT`/`FACTORY_LOG` (fixtures inheriting them journal where a bare run must not -
  the env, not the tree: green in the same env cleared of those two names).
- This `NEXT.md`, rewritten at the top; both superseded dispatch heads demoted to Records, every
  instruction section below carried forward unedited, the issue-373 unit's two owed rows carried
  verbatim into the owed section below so the union drops no owed line, and two harness defects this
  tick measured appended there.
- No merge, no label, no reviewer ask, no issue write; the queue belongs to the supervisor.

## What this push spent

- #452 read `reviewDecision APPROVED` at head `dbbb23a` before this merge began; the push into the pull
  request spends that approval as a matter of course (measured on #269: the row flips `DISMISSED` on a
  fast-forward conflict push with no exit-status sign). What the push owes is a re-review, which is the
  supervisor's poke, not a unit's verb.
- Gate and hand-off verdicts are quoted in the pull-request body from the runs on the bytes this file
  ships.

## What is still owed (issue-373 rows, then everything the carried hand-offs state)

1. Nothing on #373's side is left open except the review: the push spends any approval as a matter of
   course and the supervisor's poke owns the re-review ask. If the merge lands after another page
   edit, re-run `python3 scripts/check-decisions.py --citations docs/factory.md` and expect 59 rows
   plus whatever the other edit added — the neutral rewrite means a conflict on this paragraph is the
   only way this unit's pointers break.
2. `H-0169`'s remaining bare rows point at the scripts and stay bare; the instrument the issue-429
   hand-off owes (its item 55) covers them. The docs row, the one this ticket proved dead, is paid.
3. Two harness defects this tick measured and did not touch, both outside a conflict tick's remit.
   (a) the allocator-journaling selftest legs inherit `FACTORY_TICK_UNIT` and `FACTORY_LOG` from the
   dispatching shell -- every tick-dispatched `make verify` carries them (this tick's `env` read
   names both) -- so the fixture's bare allocator leg journals where a bare run must not and
   reddens `a bare run journals nothing` beside its control leg; the fixtures must scrub the two
   names before launching. Measured both ways on one tree: 2 FAIL exported, 0 FAIL and
   `verify: OK` under `env -u` over the same bytes.
   (b) the allocator's union reads entries on refs only, so a `DECISION-ID` line journalled to a
   sibling that never landed an entry is invisible to it and no `PENDING` fires: `H-0277`
   journalled 16:09:10Z to `issue-371`, absent from every ref and tree this clone can see, re-read
   0s after a fresh full fetch answering the same id with no PENDING. `H-0278` is the yield this
   bought; the journal also has no abandoned line, so the two `H-0277` rows this tick journalled
   and will not land stand as claims nothing can retire but a landed entry.


## What this conflict tick pruned, and how to get it back

| Dropped | Where it is whole | Why it is a record and not an instruction |
|---|---|---|
| the issue-373 third dispatch's leading state read and its `## What is still owed (issue-373 rows...)` intro and carry-forward note (lines 3-8 and 39-50 of `git show dbbb23a:NEXT.md`) | that blob | that read is the clock at `dbbb23a`, before this sync; both numbered owed rows are carried verbatim into the live owed section above, so the prune hides no debt |
| the issue-365 conflict tick's `# NEXT` intro and its leading state read (lines 1-9 of `git show 956f8fc:NEXT.md`) | that blob | #444 merged as `956f8fc`; its mergeable/approval claims died with the merge, its durable rows are carried under its Record below |

Everything else is carried forward unedited. A dropped owed line is the worse failure - the two drops
above are state reads whose clocks expired, and the only owed lines they restated are itemised as
carried above.


## Record — the issue-375 delivery unit (PR #454, the `H-0275` pass; its state read is its own clock's)

State read before this file wrote a byte, by this unit, one read per call: branch head at the
checkpoint is `942396e` (the pre-`H-0275` checkpoint, the rollback target); the allocator, fetched
first, answered **`H-0275`** (journal lines 15:35:00Z and 15:36:36Z, both `REFRESH 0s union`); no
pull request existed for `issue-375` before this unit pushed (`gh pr list --head issue-375
--state all` answered `[]`). `python3 scripts/check-decisions.py` on the shipped bytes answers
**335 entries / 1093 citations read / 586 owing an anchor / 0 refused, rc 0**, and
`--citations` ends `debt this gate ratchets on: 586` — the constant unmoved. Re-read all of it
before acting: a number here belongs to the clock it was taken on.

## What this dispatch pushed

- `942396e` (the pre-`H-0275` checkpoint, the rollback target) and one effect commit carrying
  `Decision: H-0275`: three blocks of `docs/factory.md` rewritten to carry no figure the gate
  prints — the opening tally and the ratchet paragraph become pointers ("the closing line reads
  how many were read and how many owe"), the paragraph gains the rule "Never copy its readings
  into prose" naming #375 as the measurement, and the past-EOF sentence stops claiming a count it
  never read. 9 lines changed 9 lines, file 2706 before and after, so every coordinate aimed at
  the page keeps its ground; every figure no command prints stays exactly as #444 pinned it.
- `H-0275` at the log's tail, with two defects repaired before the commit rather than shipped: the
  generated entry's `Chose:` restated option 1 in its own words and the gate refused it
  (`not among the recorded options`, rc 1) until `Chose:` became the option's verbatim text; and
  its opening pinned `9f7a8d8:docs/factory.md:1936`, which at that revision is the possessive-`s`
  sentence — the true figures sit at 1906/1962/1966/1983, verified with `git show` line by line.
- This `NEXT.md`, rewritten at the top; every instruction section below carried forward unedited,
  and the previous dispatch head renamed to a Record so "this dispatch" never means two units.
- No merge, no label, no reviewer ask, no issue write; the queue belongs to the supervisor, and
  the reviewer ask is its poke (`factory_request_review`), not a unit's verb.

## What this push spent

- Nothing spent on review: no pull request for `issue-375` existed before this push, so the first
  review of this branch begins with these bytes. The supervisor's poke does the ask.
- Gate, mutations and selftest verdicts are quoted in the pull-request body from the runs on the
  bytes this file ships.

## What this unit observed and could not close

- **The `H-0275` entry appeared on disk, complete, before this session wrote it.** The tree's
  `factory/DECISIONS.md` gained the full nine-line entry at 15:35:14–15:37:11Z while this unit's
  own writes had touched only `docs/factory.md` (three edits, verified `git diff` 9/9 lines); the
  `TICK-START` journal line and both `DECISION-ID` lines are this unit's own allocator reads — the
  log `20261118T153132Z.unclaimed:375.log` tails *this session's* transcript, so no second agent
  is running this ticket. The unit cannot name the writer from here, and it is recorded rather
  than explained because the failure class is #1180's: an `Effect:` on disk claimed "the whole
  gate reads green on these bytes" **before any gate ran** — a prophecy written as a receipt. What
  makes it safe is what #1180 prescribes: every claim was then re-derived (the gate refused the
  entry twice: rc 1 on `Chose:`, then the mis-pinned coordinate found by `git show`), the gate was
  actually run to rc 0, and the shipped entry quotes only figures re-read after the writes. A
  successor that finds its own work on disk already written must run the same ladder — read-back,
  gate, coordinate-by-coordinate opening — and must not treat the file's existence as its
  verification. Whoever can reach the journal's writer for 15:35–15:37Z should diff what it wrote
  against this commit: they differ in exactly the three repairs above.
- #375's own numbers drifted again while this unit worked (the issue says 565→1021; the pre-entry
  tree answered 1091); that is the third independent proof the figures were never maintainable as
  prose, and `H-0275`'s `Why:` carries it.


## Record - the issue-373 third dispatch (PR #452, pushed `a1b1b96` + `dbbb23a`; its state read is its own clock's)

## What this dispatch pushed

- `docs/factory.md` line count **unchanged at 2696**: the release paragraph rewritten inside its own
  eleven lines (base 118-128, new 118-128), so none of the 56 openable pointers into the page — 27
  anchored, 29 bare — moved. The clause the round-2 review asked for is in: the queue reads **only the
  label** (`is_parked()` is a jq test of the label map, `factory/triage.sh:845`) and the comment is
  durable for the human and for the next dispatched tick. No label name spelled on the page — the
  `esc_one_home` leg in `factory/selftest.sh` still holds.
- `factory/DECISIONS.md`: `H-0273` and this hand-off. Two in-place repairs to other units' rows:
  `H-0169`'s docs pointer was **already dead in the base** — its fragment sits at line 204 of
  `docs/factory.md` at `0a2c093`, not the cited 205 — re-aimed and anchored; `H-0264`'s row quoting
  the old constant was revision-pinned to `0a2c093:scripts/check-decisions.py:770`, verified openable
  there.
- `scripts/check-decisions.py`: `POINTER_DEBT` 544 → 543 (the `H-0169` payment), ledger comment
  extended in place, file still 2271 lines.
- No merge, no label write, no reviewer ask, no issue write; the queue belongs to the supervisor.

## What this dispatch verified, on the shipped bytes

- `python3 scripts/check-decisions.py` → 328 entries, 1032 citations, **543 owing an anchor against a
  recorded 543**, 0 refused, exit 0. `--citations docs/factory.md` → 59 rows, `H-0169` anchored.
  Full-census exempt list 71 → 72 (this entry's pin).
- `bash scripts/check-next-md.sh` → OK (its verdict is quoted in the pull-request body, the #337
  contract).
- `make verify` → passes on these bytes; its raw tail is quoted in the pull-request body. Run under
  `env -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH -u FACTORY_LOG` because this session's harness
  exports the tick names (the leak #449 is about — see the carried-forward section below; it is real,
  this dispatch hit it too).


## Record — the issue-371 second conflict tick (PR #450, merged as `c0de3a4`; its state read and its pushed/spent record are its own clock's; the pass itself is `H-0276`)

State read before this file wrote a byte, by this conflict tick, one read per call, raw JSON and
never a `-q` template: **#450** `OPEN`, `reviewDecision` **APPROVED**, `mergeable`
**CONFLICTING**, `mergeStateStatus` **DIRTY**, remote head `45a76ca` matching this checkout, checks
green. `origin/main` was at `956f8fc` (PR #444 merged, carrying `H-0270`/`H-0271`). The allocator,
fetched first, answers **`H-0276`** with a `REFRESH` note and no `PENDING`; re-run it before
honoring this id on any successor's authority. Re-read all of it before acting: a number here
belongs to the clock it was taken on.

## What this dispatch pushed

- `09d1615` (the pre-`H-0276` checkpoint, the rollback target) and the merge commit carrying
  `Decision: H-0276`: `git merge origin/main` over 4 conflict hunks in `factory/DECISIONS.md`
  (3 where the sides differ only in digits — `H-0168`, `H-0177` and `H-0234` rows re-quoted at each
  tree's numbers — adjudicated by taking the trunk side and letting the gate speak, 1 tail union
  seating this branch's `H-0272` above trunk's `H-0270`/`H-0271` in id order); every rejected
  `path:line` digit then re-aimed by the gate's own named line — 14 pointers, 12 of them the
  uniform `+11` this branch's `selftest.sh` growth imposes, one 235→236 in `check-decisions.py`,
  one self-citation re-seated onto the cited row rather than the row quoting it. `NEXT.md`,
  `DECISIONS.md`, `docs/factory.md` and `scripts/check-decisions.py` auto-merged.
- The zero-loss census before the push: 0 parent-added lines absent from the merged tree across
  all 6 paths either parent touched since `0a2c093`, digits and ranges folded; both logs unique
  and ascending (266 `H-`, 71 `D-`).
- This `NEXT.md`, rewritten at the top; every instruction section below carried forward unedited,
  and the superseded dispatch head renamed to a Record so "this dispatch" never means three units
  at once.
- No merge, no label, no reviewer ask, no issue write; the queue belongs to the supervisor, and
  the reviewer re-ask after a push is its poke, not a unit's verb.

## What this push spent

- The push spends #450's `APPROVED` as a matter of course (measured on #269: a conflict-resolution
  push moves APPROVED→DISMISSED with no exit status reporting it); what the push owes is a
  re-review, which is the supervisor's poke. The post-push read is quoted in the pull-request body.
- Gate, `check-decisions` and `check-next-md` verdicts are quoted in the pull-request body from the
  runs on the bytes this file ships.

## Record — the issue-365 #444-sync dispatch (PR #444, merged `956f8fc`; its state read and its pushed/spent record are whole at `git show a8bcb5a:NEXT.md` lines 1-31; the pass itself is `H-0271`)



## Record — the issue-365 round-5 dispatch (PR #444, pushed `86ec41c` + `30119d7`; its state read is its own clock's)


## What this dispatch pushed

- `86ec41c` (the pre-`H-0270` checkpoint, the rollback target) and one effect commit `30119d7`
  carrying `Decision: H-0270`: the six-rows sentence — shipped three times, the `POINTER_DEBT`
  comment, `H-0268`'s `Effect:` and the pull-request body — is replaced at the head that carries it
  with the diff `12ddbc4` measures: open rows 175 -> 176, 32 pure citing-line renames (14 at +2,
  18 at +123) and one addition (main's own `H-0256` Follow-up citing a bare :12339), 0 open rows
  lost; the unanchored-debt diff is 74 rows each side over 586 on each — 5 pure renames, all +2,
  plus 69 coordinate re-aims, entry-balanced. Two of the four line pairs the old sentence named are
  debt re-aims, not open renames, and the entry says so. The round's three non-blocking carries went
  in with it: the message-vs-entry 26/27 disclosure, the docstring's `adddacc` pair qualified "under
  this rule" per `H-0258`, and `H-0253`'s Question labeled to the sync-merge tree.
- On these bytes: `python3 scripts/check-decisions.py` answers 331 / 1067 / 586 / 0 refused rc 0,
  `--self-test` answers 68 must-fail and 37 must-pass rc 0; `make verify`'s verdict is the pull
  request body's, from the run on the bytes this file ships.
- This `NEXT.md`, rewritten at the top; every instruction section below carried forward unedited,
  and the previous dispatch's push record renamed to its owner so "this dispatch" never means two
  units at once.
- No merge, no label, no reviewer ask, no issue write; the queue belongs to the supervisor, and the
  reviewer re-ask after a push is its poke, not a unit's verb.

## Record — the issue-429 unit (PR #451, merged `0a2c093` as `3bb1f08`+`3d7c2f7`; its state read above its name is that unit's own and belongs to the clock it was taken on)


## What this dispatch pushed

- `3d7c2f7` — `Merge origin/main (26e8c6d, past fe21023) into issue-429`: the 44 conflict hunks in
  `factory/DECISIONS.md` resolved as the id-ascending union, then every `path:line` digit in both logs
  re-derived by content identity against the merged files (the review's named procedure, `fe21023`'s
  digits-not-sentences). Also carries the review's second blocking point: the pair at line 3342 of
  `DECISIONS.md` whose second coordinate was stale-at-true — the checkpoint docstring moved, the
  citation now reads 646. Proof the review demanded, measured on the tree the pass last touched:
  `python3 scripts/check-decisions.py` answers `0 refused` with `POINTER_DEBT` **544 unchanged**.
- `H-0269` and its effect commit: the entry records the pass, the fixed-point discipline (a
  cosmetic blank-row delete moved 84 anchored pointers out and had to be followed by the gate, not
  by faith), and the #449 reproduction below.
- `factory/selftest.sh`: the stray double blank between the six new `gen_leg`s and
  `gen_leg 'the allocator never fetches'` deleted (review non-blocking item 2).
- This `NEXT.md`, rewritten at the top; every section below is carried forward unedited.
- The pull-request body: drift figures re-derived on the merged tree (review non-blocking item 1),
  and the `DECISIONS/H-0042.md` example question raised on the PR (non-blocking item 3 — whether the
  #429 sketch's `D`↔`DECISIONS/` pairing should bind is a design call the sketch owns, not this sync).
- No merge, no label, no reviewer ask, no issue write; the queue belongs to the supervisor.

## What this push spent, and what it did not fix

- The push spends any approval on #451 as a matter of course — and at the read above the review
  stands at CHANGES_REQUESTED, so what the push owes is a re-review, which is the supervisor's poke,
  not this unit's ask.
- #449 (`FACTORY_TICK_*` inherited by the journalling fixtures) reproduced live this dispatch:
  `make verify` launched from this session's environment failed exactly its two named legs, because
  the dispatch exports `FACTORY_TICK_UNIT`/`FACTORY_TICK_BRANCH`. The suite's own bare-run control
  caught it — that is the control doing its job — and the gate runs here with those six variables
  unset until #449 lands its repair. Anyone re-running `make verify` on this branch: same six.
- The 20 ambiguous unanchored pointers the pass refused to guess stay stale (recorded in `H-0269`'s
  `What lost:`); they are unread by any gate and unchanged in kind from the 544.
- Phase 2–4 of #429 stay open and are the body's `Not in this PR` section, unchanged.

## Carried forward from the issue-370 unit (PR #446), verbatim

- `bf51a58` (the pre-`H-0267` checkpoint, the rollback target) and one effect commit carrying
  `Decision: H-0267`: the `verify` job gains a step that installs tzdata `2026e-0ubuntu0.26.04.1` and
  its `tzdata-legacy` sibling, both verified against the Ubuntu archive's published sha256, replacing
  `/usr/share/zoneinfo` before `make verify`. That is the repair for six `internal/schedule` failures
  refusing the runner's partial zone database; `factory/DECISIONS.md` `H-0267` carries the measured
  pair (base-only deb rc 1 with the six failures verbatim; the pair rc 0 in 2.605s, both on this
  26.04 box) and the evidence that pinning `ubuntu-24.04` would only defer the same red —
  noble-updates' 2026e base deb ships no `Enderbury` either. The same commit re-aims `H-0266`'s
  `ci.yml` pointer, which this step's 58 inserted lines moved.
- The eight SC2329 infos under shellcheck 0.11.0 stay unfixed — that is `H-0266`'s own `What lost:`. They
  sit on fixture functions `factory/selftest.sh` invokes indirectly by name, and the pin at 0.10.0 is what
  keeps them out of the verdict; the day this repository moves the pin, those eight rows are the next
  decision. Do not "fix" the pin by deleting the digest.
- The tzdata pair is pinned to release `2026e`, the one the 26.04 image itself ships. When this repository
  chooses to follow tzdata, both digests and the version move together — `H-0267`'s `What lost:` row is
  where the next unit learns the pair is a standing obligation, not a one-time fetch.
- A box whose PATH shellcheck is 0.11.x reads `make verify` RED on these bytes with exactly those eight
  rows. That is disclosed and known, not a regression you introduced: point `SHELLCHECK` at a 0.10.0
  binary (the one the `shellcheck` job fetches) and the leg prints `check-shell: 0.10.0 over 33 scripts`
  at rc 0.
- #370 item 3 — may a red trunk refuse a `MERGE`? — stays open and is a human call: carried row 57
  below; the shipped read is report-only, and neither `H-0266` nor `H-0267` touched that question.
- Their dispatch's pruning table is whole at `git show 3594696:NEXT.md`; #446 merged, so its rows are
  history now, not instructions.

## What this dispatch pruned, and how to get it back

| Dropped | Where it is whole | Why it is a record and not an instruction |
|---|---|---|
| the #444-sync dispatch's leading state read, its `## What this dispatch pushed` and `## What this push spent` (lines 1-31 of `git show a8bcb5a:NEXT.md`) | that blob | #444 merged as `956f8fc`; its conflict pass is recorded in `H-0271`, its merge commit and this branch's sync that landed on top of it |
| that dispatch's leading state-read paragraph, replaced here by this tick's read | this file's top | a read of a queue that has moved: #444 is merged and this tick's read is the live one |

Everything else is carried forward unedited.

## What the issue-365 #444-sync dispatch pruned, and how to get it back

| Dropped | Where it is whole | Why it is a record and not an instruction |
|---|---|---|
| the round-5 dispatch's leading state read and its intro (lines 1-13 of `git show a8bcb5a:NEXT.md`) | that blob | #444 was still open when that tick wrote it but that read's queue state was the fifth round's; its own read sat above it |
| the issue-429 unit's leading state read and its `# NEXT` intro (lines 35-45 of the conflicted merge, whole at `git show 0a2c093:NEXT.md`) | that blob | #451 has merged; its mergeable/approval claims are stale, its durable rows are carried under its Record above |
| the H-0268 sync dispatch's state read and its `## What this dispatch pushed` (lines 1-26 of `git show 12ddbc4:NEXT.md`) | that blob | that push landed as `12ddbc4` and is recorded in `H-0268`, the pull-request body and the fifth-round review read; the debt composition it stated is itself now re-measured by `H-0270` |
| the issue-370 top block's dated state read (lines 3-11 of `git show 26e8c6d:NEXT.md`) | that blob | a read of a queue that has moved: #446 has merged since |
| its `## What this dispatch pushed` and `## What this push spent` bullets that speak of #446 as live (lines 13-46 of the same blob) | same blob | #446 merged; the mergeable/approval claims are stale, and the durable ones are folded into the carried section above |
| its pruning table (lines 48-57 of the same blob) | same blob | #446's drops are history now, itemised above in one line |

Everything else is carried forward unedited. None of it is an owed line this branch owes; the two
open ledger questions the previous top restated — #370 item 3, the red-trunk `MERGE` human call
(carried as row 57 below), and the eight SC2329 infos due when the shellcheck pin moves — stay
stated in the carried prune section below and are named here too, because a dropped owed line is the
worse failure. One operational note the dropped head carried travels by reference, not by edit: port
22 to `ssh.github.com` still timed out on that dispatch's clock, so its fetch used the
`GIT_SSH_COMMAND` 443 incantation in `docs/factory.md` — load-bearing, not folklore, and owed a
re-measurement on whoever's clock fetches next.

## What the H-0268 sync dispatch pruned, and how to get it back

| Dropped | Where it is whole | Why it is a record and not an instruction |
|---|---|---|
| the issue-370 unit’s leading state read and its `## What this dispatch pushed` / `## What this push spent` sections (lines 1-47 of `git show e3587b9:NEXT.md`) | that blob | #446 reads `MERGED` in the read at `git show 12ddbc4:NEXT.md`, which this file's top has itself since pruned: its pushes landed, its approval question died with the merge, and its `verify` red was fixed by the pins its own entries record |

Everything else is carried forward unedited. A dropped owed line is the worse failure — so the drop
is itemised above, none of it is an owed line this branch owes, and the two open ledger questions it
named (#370 item 3, the red-trunk `MERGE` human call, carried as row 57 below; the eight SC2329 infos
due when the shellcheck pin moves) are stated here in one breath so the prune cannot hide them.

## What the issue-374 lineage pruned at this record, carried from trunk's copy

| Dropped | Where it is whole | Why it is a record and not an instruction |
|---|---|---|
| the issue-371 second-tick top block, its pushed/spent sections and its two prune tables (lines 1-169 of `git show c0de3a4:NEXT.md`) | that blob | #450 merged as `c0de3a4`'s ancestry; its conflict pass is recorded in `H-0276`, and the digits its tables name were re-derived by this tick's own gate run on the merged tree |
| this branch's own superseded top — the issue-429 unit's head and state read, 11 added rows | `git show 243ec7b:NEXT.md` | #451 merged; that read's queue state is four merges old, and every durable row in it was carried into the sections below long before this tick |

Everything else is carried forward unedited. A dropped owed line is the worse failure — none of the
two drops above is an owed line; the owed rows begin at the inherited list below, and the two
standing ledger questions (item 57's red-trunk `MERGE` human call, the eight SC2329 infos due when
the shellcheck pin moves) still stand there.

Provenance: lines 460-470 of `git show b433f01:NEXT.md`, verbatim — the section trunk's copy of the
issue-429 record used to itemise its own pruning. Ours' copy of the same record carries a different
table, and the two are not wordings of one another, so both ship. "this branch" in the second row
is the issue-374 lineage, not this one.
## What the verdict makes visible, and this unit could not touch

- `docs/factory.md`'s summary of this token — the paragraph that calls it "the token this log argues
  about most" — stops at "`H-0159` reads the same token both ways" and sends the reader to `H-0162`'s
  Effect. Its own sentence, that the root reading is past EOF, is true; the framing is unfinished.
  Whoever edits that file next ends the sentence with the verdict. `H-0251`'s `Follow-up:` row carries
  this, and no issue was opened, because #366 is the right scope.
- Four other rows carry the same token on the root reading and are untouched: `H-0152`'s, `H-0161`'s,
  `H-0167`'s `8437eea:README.md:588` census row and `H-0177`'s Effect row. None of them becomes a defect
  by this verdict — on the root reading they are all past EOF and say so — and none is repaired here:
  `H-0143`/`H-0144`, the objection `H-0159` itself files, decides a measurement is not rewritten in
  place. If a future unit re-aims them, it re-aims to the file the prose means, and says so in its own
  entry.
- #318's length-check gate stays unmade and `POINTER_DEBT` sits at 544 — the issue-366 unit recorded 546
  after its ten citations, and the pair now measures 544 at `bf51a58` whether or not a later unit paid two
  down; this unit added three more citations, every one anchored to live text it was measured against, so
  the number did not move and the exempt count (71) did not either.

## What the issue-366 unit pruned, and where its two dropped hand-offs are recovered verbatim

Two `## What this unit shipped` sections are dropped by this file, both of them dated reads of a pull
request that has since merged, and both recoverable whole: the clause-5 hand-off at
`git show 3c09d6b:NEXT.md`, lines 13 to 30, and the census hand-off — the six-clock tool, the
`esc_one_home` marker-name repair, and that dispatch's own hand-run census — at
`git show 38a1fe4:NEXT.md`, lines 35 to 67. Everything from `## What is still owed` downward is carried
forward from **that second file**, unedited, and that is what brings owed items **5-7** and the
`Three things a stranger will break here` section into this branch's copy: the head this branch wrote at
`2dd751a` carried the four-item owed list and the `Two things` section out of `3c09d6b`, and `H-0249`'s
unit added three owed items, one stranger-warning about a branch left red by its own last commit, and a
rewritten probe bullet now that the census has a tracked script — all three taken here rather than
rewritten here. Also carried: the 56 numbered inherited rows, PR #402's three non-blocking review items,
`Hard rules for whoever picks this up`, and that unit's own prune list, the section further down with
the near-identical heading, which is the only record of what it dropped from the 1,174-line file it
inherited. A dropped owed line is the worse failure, and item 4 of the inherited list says so.

## What is still owed (issue-366’s rows 1-7, carried forward; none of it silent)

1. **The number, the clock and the zone for the bound, from a maintainer.** Ask it on the ticket, in one
   line, and read the answers back out of the API rather than from this page. `retention: option A` does
   not exist as a format any more — the option letter died with the sixth unit's post — so a reply that
   names days only is *not* an answer, and clause 5 is the rule that says a bound must refuse it rather
   than default it.
2. **The bytes and generations, once 1 is answered.** `H-0188` withheld them; four things read the journal
   by name and one of them is the busy guard that authorises removing a worktree, so clause 1 (rotate at a
   tick boundary after the release line) and clause 2 (never window the earned-id scan) are not optional
   decoration on whoever implements it. Clause 3 says keep the newest bytes, never `head -c`; clause 4
   says the footprint line must show what a prune took.
3. **A park for a waiting escalation is #437's, not this unit's.** Until that lands, expect #334 to be
   dispatched again and again; if you are that dispatch, read the ticket's comments first — the last three
   units each found something the previous one did not, and none of them chose the number.
4. **The red you will see on your first `make verify` is already ticketed: #425, and its title is right
   about the variable.** Not this unit's doing — the untouched base tree read red in this dispatch too. Six
   `FACTORY_*` exports are inherited from a dispatch, among them `FACTORY_TICK_UNIT=unclaimed:334` and
   `FACTORY_TICK_BRANCH=issue-334` (read `env | grep ^FACTORY` where you are standing and count them), and a
   fixture whose question is "a bare run journals nothing" cannot answer it with a unit name in its
   environment. Five runs of `factory/selftest.sh` alone on one tree, each ~90 s:

   | run | environment | legs failed | result |
   |---|---|---|---|
   | A | inherited minus `FACTORY_TICK_UNIT` and `FACTORY_TICK_BRANCH` | — | 828 passed, **0 failed** |
   | B | inherited minus all six `FACTORY_*` | — | 828 passed, **0 failed** |
   | C | inherited as-is | both | 827 passed, **2 failed** |
   | D | inherited as-is, repeated | both | 827 passed, **2 failed** |
   | G | inherited minus `FACTORY_TICK_UNIT` **only** | — | 828 passed, **0 failed** |
   | H | inherited minus `FACTORY_TICK_BRANCH` **only** | both | 827 passed, **2 failed** |

   G against H says `FACTORY_TICK_UNIT` alone is sufficient and `FACTORY_TICK_BRANCH` contributes nothing;
   C against D says it is deterministic, not flaky — the same two legs, `control: one note can suppress the
   other` and `a bare run journals nothing`, every time. So #425's title is exact and the repair is to scrub
   `FACTORY_TICK_UNIT` in the journalling fixture's bare arm the way other legs scrub `PATH`. This unit did
   **not** touch the harness on this docs branch, and it did not assume #425's diagnosis was correct until
   run G tested it. Rule out **#371** (`selftest.sh` printing `FAIL` where it means `SKIP` when `$TMPDIR`
   resolves inside a repository) before blaming your tree for any *other* red leg: `echo $TMPDIR`.

   Own this too, because it is the reason the row exists: the first version of it reported "830 passed / 0
   failed" and "829 passed / 1 failed", called the pair the cause, and described a leg as intermittent. The
   logs say 828, 828, 827-with-two, 827-with-two, 828, 827-with-two. The numbers were read off a
   half-remembered grep of a running job instead of the finished file, and the two conclusions built on them
   were both wrong. A claim about a run is a claim about a file — `grep -o '[0-9]* passed, [0-9]* failed'`
   and `grep '  FAIL  '` on the completed log, before the sentence.

   And correct the record on this hand-off: an earlier draft named #374, which is the citation-ratchet
   ticket, because the number was recalled instead of read. #425 is what the measurement is about. A
   referent named from memory is the same class of defect as a coordinate that was never opened.


5. **The census tool's own two gaps, both stated in its output and neither repaired here.** Row **E**
   cannot be priced at a pinned `--now` — `find -mtime` reads the real clock and takes no reference time,
   so the tool refuses (exit 2) rather than print a row off two clocks; a future change that wants E at a
   `--now` has to set the fixture's mtimes to that instant, which is exactly what `rc_live_fixture` does
   for the live-fault leg. And `--bytes` prices the **transcript** set only: `journal:` prints
   `supervisor.log`'s bytes and line count so an operator can see what a cap would do, but the
   keep-newest-B-bytes arithmetic and the generation/rotation-point rules are owed item 2's, and the
   script says `journal line: 1 of 7 rows` for that reason. Anyone who implements 1 and 2 should extend
   this script rather than write a second instrument, or the two will disagree in production the way the
   six clocks disagree in the fixture.
6. **A `bash -x` self-test is refused, and the refusal is the only thing that keeps that leg honest.**
   If a future unit finds the guard inconvenient, the fix is not to widen it: a traced function returns
   its trace on stdout (`rc_row` and `rc_find_cut` are command substitutions), so the legs read rows that
   are not rows — measured at 13 failed legs and 67 of 79 row assertions, all of them naming the leading
   `+ local tz=…` line as a row whose first field was not one of A–F. Run `bash factory/retention-census.sh
   --self-test`, or `bash -B`/`set -u`, both of which stay allowed and stay green.
7. **The park is still #437's, and #311's merged PR did not create the marker.** Re-measured at
   `00:08:17Z`: 18 labels, none of them the name `factory/escalation-label.sh` prints; `blocked` on
   exactly 334 and 311; triage's own stderr says nothing is parked because that marker name is not a
   label in this repository, so no issue can be parked at all -- #311. (That stderr sentence spells the
   name out; this file may not -- `esc_one_home` keeps the name in `factory/common.sh` alone.)
   Creating the label is a human verb and no unit here ran it; until it happens, expect this
   ticket to be dispatched again, and read the ticket's eleven comments before measuring anything — the
   last five units each found something the previous one missed, and none of them chose the number.

## Three things a stranger will break here

- **Green is a property of a tree and its leg set, never of a branch.** The census dispatch ran the suite
  six times green and *then* committed a NEXT.md that spelled the escalation marker's name three times —
  while the leg that scans NEXT.md for it (`esc_one_home` at `factory/selftest.sh:9745` `esc_one_home [root] -- the name is spelled once, and that once is common.sh`)
  was already live in its own base: `git show 64ec65e:factory/selftest.sh` carries the `claim_leg`.
  So the branch tip was red from the minute that commit landed, nothing re-gated it because the branch
  was never pushed, and the next dispatch's first `make verify` met it. Re-run the full gate after the
  last file your last commit touches.
- **Line numbers in these logs are live coordinates, and an insertion moves them.** `make verify` re-opens
  every `path:line` in both decision logs and exits 1 naming the true line; this unit's own edit tripped it
  twice inside ten minutes, once by two lines and once by three. So add to the *end* of the section you are
  editing, then re-aim: `python3 scripts/check-decisions.py --citations docs/factory.md` lists every
  pointer aimed at a page before you touch it, and `POINTER_DEBT` may only fall.
- **The probe in clause 5 is still documentation and still fails quietly; the tool is what refuses.** The
  block prints six zeroes at exit 0 wherever the journal directory is not — measured again on an empty
  directory at this head, every one of the six arms printing `0` — and the probe is not tracked, so it
  exists only where someone typed it. `factory/retention-census.sh` is the tracked half and it exits 3 on
  the same directory naming the glob it found nothing behind. `total` is printed by both for exactly that
  reason: six zeroes beside a nonzero `total` is the finding, six zeroes beside `total 0` is your own
  working directory.

## What the issue-366 unit pruned out of the hand-off it inherited, and how to get it back

`NEXT.md` had been accreting since 12 Nov: `origin/main`'s copy is 1,174 lines, and `3d3c65c`'s owner was
the issue-364 conflict unit whose PR #436 has merged. This unit keeps the four sections that are still
instructions -- the 56-item owed list, that unit's `What this unit did not deliver`, PR #402's three
non-blocking review items, and `Hard rules for whoever picks this up` -- all of them below this line, with
two headings renamed so a reader knows whose rows they are. It drops the sections that are dated
measurements of a queue that no longer exists, each recoverable verbatim:

| Dropped | Lines in `3d3c65c:NEXT.md` | Why it is a record and not an instruction |
|---|---|---|
| `What this unit shipped` (issue-364) | 19-53 | PR #436 merged at `3d3c65c`; its verdicts are in the merge and the pull request |
| `Queue state the fifth tick read` and `The census's first live run` | 70-107 | self-labelled "stale as of the sixth sync" |
| the sixth-to-eleventh sync re-measurements | 603-828 | each one a read of a coordinate `main` has since moved |
| the `H-0232` / `H-0229` hand-off summaries and wording ledger | 848-1015 | shipped by the PRs they name |
| `What the #362 unit shipped` | 1152-1174 | PR #434 merged |

Recover any of it with `git show 3d3c65c:NEXT.md`. The reason for pruning rather than appending is #436,
which existed for one thing only: this file conflicts on every trunk merge, and 1,174 lines of it were
46 conflict regions of digit drift. A dropped owed line is the worse failure and item 4 of the inherited
list says so -- so the prune is itemised here rather than silent, and nothing in the four kept sections
was edited except the two headings named above.

## What the issue-364 unit did not deliver (its PR #436 merged; these rows are still its owner's successor's, i.e. the next unit's)

- **No behaviour was re-derived for the merged tree beyond parse and presence.** `bash -n` and the
  greps above are presence claims. The suite run that matters is `make verify`'s own, quoted in the
  pull-request comment from the run that ships these bytes.
- **The review this branch had is spent by the push and nobody has been re-asked yet.**
  `gh pr view 436 --json reviewDecision,reviews,mergeable,mergeStateStatus` after the push is the read
  that decides what the body may claim; the supervisor re-asks on its own poke (`factory_request_review`,
  `FACTORY_REVIEWER_TEAM`), and asking is not this unit's verb.
- **Owed items 54–56 below are untouched by this sync.** The unparsed `H-0242` heading was re-confirmed
  alive (the gate parses 313 entries against 314 raw `^## [DH]-` headings across the two logs — exactly
  one heading, `H-0242`'s U+2014, invisible); the 27 bare pointers and the cross-tree legs were not
  re-measured and are carried as written.
- **Nothing was merged, no label was written, no reviewer was asked, no machine-level configuration was
  touched, and only this branch's own ref was pushed.**


## Owed items inherited from the issue-364 hand-off -- 56 numbered rows, unrenumbered, numbered there and referred to by number below

*Two rows are superseded by this unit's read and one carries a word a reader should not copy. Row 1 quotes
"20 MB / 100 files on 12 Nov, 36 MB / 172": at 2026-11-17T19:37:42Z the same directory holds **227
transcripts and 58,792,819 B** plus a 1,907,580 B live journal, and clause 5 is the rule that any number
quoted from it must name its clock. And the row about #425 says "Do not close #425 on this branch's
measurement alone" -- verbatim, on purpose: that keyword next to a number is exactly the shape that actions
an issue from a pull-request body, comment or commit message, and it survives here because a file's
contents are not one of those surfaces. Copy it into a body and it becomes a closer.*

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
   hit is prose at `docs/factory.md:665` inside `### The approval a push voids`). The rule it meant is
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
   `docs/factory.md:658` "### The approval a push voids" — **re-stamped by this sync**: the row
   carried `:652`, an off-by-one that is false on both parents of this merge as well (the heading is
   at 651 on `HEAD`, on `origin/main` and on the merged tree, where the file is byte-identical to
   both), so it is a pre-existing pointer error, not a drift this merge introduced: a base-sync push
   turns `APPROVED` into a
   `DISMISSED` row without touching any exit status. Ask
   `gh pr view 410 --json reviewDecision,mergeable,mergeStateStatus` after the push; the re-ask is the
   supervisor's poke (`factory_request_review`), not a unit's.
10. **Re-measure every live coordinate you carry — this tick's worst moved by 1500 lines.** The
    `issue-334` hand-off cited `factory/selftest.sh:17163` for `reap_leg_live`'s refusal; on this merged
    tree the sentence is at `factory/selftest.sh:19803` "the live journal produced a deadline hold of 0s".
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
    the coordinates the `issue-346` unit pinned at `factory/selftest.sh:15767` and
    `factory/selftest.sh:18663` (+461 on the tree it measured, both re-opened here).

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
    `factory/selftest.sh:18010` `index_exec_problems() { # <repo> <name>... -> the names git does not
    hold at 100755` collects one mode per **index entry**, and an unmerged path has three (stages 1/2/3),
    so `factory/selftest.sh:18014` `[ "$mode" = "100755" ] || bad="$bad $f(${mode:-untracked})"` compares
    a three-line string and refuses a mode that is correct. Every conflict tick meets this before it can
    stage, and the refusal names no stage, so it reads like a real permissions defect. Repair shape:
    dedupe the collected modes and refuse only a distinct value that is not 100755, or skip paths git
    reports unmerged and say which were skipped. This unit changed no leg -- the workaround is to stage
    the resolutions and re-run, which is what the verdict quoted in the pull-request body was taken on.
    **Update (this sync, 2026-11-18): the repair shipped as `H-0243` (#368) --
    `factory/selftest.sh:18979` "index_exec_problems() { # <repo>" now reads stage 0 alone and an
    unmerged path prints `(unmerged at stage ...)` instead of a three-line mode string, so the
    mechanism above no longer exists and its permissions misframing with it; the leg still refuses
    while the index is unmerged, so the stage-then-rerun workaround stands. The two coordinates in
    the paragraph above do not open on `origin/main` or on this merged tree -- a NEXT.md digit is
    read by no gate (#375), which is why this bullet's own numbers rotted silently behind its fix.**

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
    `H-0099`'s `common.sh:1640` lands on `dirt_rc=`, not on the line the entry means. The method is
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
    `factory_worktree_lines`, which sits at `factory/common.sh:1580` "factory_worktree_lines() { # [main-checkout]",
    and the row numbered 652 of `docs/factory.md` naming the heading that is at
    `docs/factory.md:658` "### The approval a push voids" — false on `HEAD`, on `origin/main` and on
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
    naming the wrong statement: `scripts`-level guards `factory/selftest.sh:21731`, `:21647`, `:21664`
    (`dpn_mut` and `grep -qF` calls that also mention the fragment) instead of the `control failed:` refusal
    the entry measured, `factory/selftest.sh:21732` and `:21648`, each `grep -c` answering **1**. Both shipped
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
53. **The bare-pointer debt this tree carries, who created it, and the instrument that can say so.** A bare
    `path:line` pointer is one with no anchor quote after it: 546 of the log's 876 citations are bare and
    `check-decisions.py` reads none of them, so they go wrong in silence. This tree has **32** landing on a
    row that differs from the merge base `cf00b44`. Three sweeps of the same tree give three numbers and the
    difference *is* the instrument, so all three are on the record: one reference tree for every pointer
    reports **46** — it judges pointers trunk wrote by our files and charges the merge for damage it did not
    do; a per-entry reference reports **31**; attributing each pointer against **both** parents' tips reports
    **32**, and only that shape answers the question a repairer actually has — all 32 read true at `670df88`,
    all 32 false at `883b5e8`, 0 false at both, so the debt arrived with trunk's growth, it is not this
    branch's and the union created none of it. Of the 32, **14** are coordinates this sync re-aimed, which no
    base-referenced sweep can call right; **18** (13 into `factory/common.sh`, 5 into `factory/selftest.sh`)
    are untouched and wrong; **0** land on a blank row. To pay it down: `python3 scripts/check-decisions.py
    --citations` lists every pointer aimed at a path and marks the checked ones `anchored`, so the bare ones
    are the rows printed without that word; for each, read the coordinate's row in the tree that *wrote* the
    entry — `git show cf00b44:<path>` for an entry already at the base, `git show 670df88:<path>` for one this
    branch wrote, `git show 883b5e8:<path>` for one trunk wrote — then re-aim the coordinate to the merged
    file's row whose pointed-at line plus three context rows on each side match that text uniquely. A delta is
    not a shortcut: `H-0241` measured six distinct growth deltas across three files in one sync, and an offset
    applied file-wide re-points pointers onto copies of their quoted text while their anchors keep passing.


54. **Make `scripts/check-decisions.py` read an entry whose separator it does not know.** Trunk's
    `H-0242` heading separates id from title with U+2014, and the shipped `ENTRY` pattern admits only the
    middle dot, the pipe and the hyphen, so `factory/DECISIONS.md` carries **239** `^## [DH]-<four digits>`
    rows and the shipped parser yields **238** entries. `H-0242` is the only heading either log the parser
    skips — the root log's 71 rows all parse — so neither the duplicate-id rule nor the ordering rule can
    look at it, and a second `H-0242` could land on trunk and be reported clean. Falsified twice against the
    shipped checker on a copy: replace that one character with a middle dot and the file is read as **239**
    entries and then refused at the row (`H-0242 is missing Question, Options, Chose, Why, Checkpoint`);
    leave it and the entry's bare `Question:` lines with no leading dash are a second, independent reason
    `FIELD` declines the block. So widening the heading grammar alone moves the blindness rather than ending
    it, and the repair is the checker's, not a rewrite of somebody else's committed entry — decide whether
    the separator set widens, whether the authoring side gains a refusal that fires before a merge has to
    adjudicate a heading it cannot parse, or both.
55. **The 27 bare pointers this sync displaced, and the sweep that can hand them to a human.** The census
    reads every unanchored `path:line` in `factory/DECISIONS.md` — **364** resolve to a file — and asks what
    sits at the named row in the merged tree, in this branch's copy and in trunk's: **337** are the same row
    in all three, **27** name a row this branch's tip did not have, **0** differ from both parents. The 27
    are **23** into `factory/selftest.sh` and **4** into `factory/factory.sh`; they sit in **8** entries
    (`H-0112` 3, `H-0130` 2, `H-0136` 1, `H-0139` 6, `H-0142` 2, `H-0145` 1, `H-0169` 9, `H-0200` 3), and
    **7** of those 8 entries are byte-identical to the merge base — the eighth, `H-0169`, carries this
    branch's copy. So this is not text either parent edited: it is old unanchored coordinates whose targets
    moved under the union's numbering, and whether any of the 27 still reads true cannot be answered from
    the tree, because a bare pointer carries no quoted text and every one of them names a row of ordinary
    code (`\t}`, `\t\treturn 1`, a comment fragment) rather than a blank or an out-of-range row. Same dead
    end owed item 53 describes, with a number and a per-entry address; `make verify` reads none of them. The
    instrument is `H-0245`'s `What lost:`, and the work is to make it emit the 27 rows for a person to
    re-point by reading the code — the only tool that works where there is no anchor.
56. **Give the selftest legs that exercise a merged line the merge's own shape.** Two legs assert on the
    *file* rather than on the *ref*, so both fail on a correct tree in the middle of a merge — measured on
    this tree, unsanitized run first, before anything was staged: `the timer's scripts are executable`
    compares `git ls-files -s -- factory/selftest.sh` against `100755` and a mid-merge index answers **three**
    stage rows (stages 1/2/3) where a clean tree answers one, so the leg's `[ "$mode" = "100755" ]` is handed
    a three-line value and refuses. The mode is not the cause and was falsified rather than assumed:
    `git ls-tree` holds `factory/selftest.sh` at `100755` in the base and in both parents, and the leg's own
    shell re-run says one row of `100755` passes where three rows of `100755` do not — it is the count of
    index rows, so the leg fails on any conflicted file and on nothing else about the bytes; and the allocator legs
    (`a bare run journals nothing`, `control: one note can suppress the other`) read `FACTORY_LOG`, so a
    journal inherited from the tick supplies a `PENDING` heading the fixtures do not plant — `H-0230`
    sanitizes the environment and this round re-measured that the leak is still live, **818 passed
    3 failed** where the same tree with `FACTORY_TICK_UNIT`, `FACTORY_TICK_BRANCH` and `FACTORY_LOG` unset
    passes. Two repairs, and they are different: the mode leg should ask the index for the mode of the stage
    zero row, or the leg should compare against the ref it is describing, so that a leg stops depending on
    whether some other process happens to be mid-merge in the same tree; and the legs that read a journal
    should plant their own root (as the `mktemp -d` legs do) or name the sanitization in their own label, so
    a leak says so at the leg instead of looking like a broken allocator.


57. **The trunk verdict is read and said; what it may *do* is the open question.** #370 landed the read —
    `factory_trunk_ci_lines` in `factory/common.sh` journals one `TRUNK-CI` line per activation (red
    trunks add a `TRUNK-RED` line) and nothing else. Whether a red trunk should refuse a `MERGE`, halt
    the loop, or stay report-only is **not decided** and is deliberately still #370 item 3: a halt is the
    shape `H-0056` refused and #341 measured the cost of, and a merge-gate change is a class of decision
    this harness does not make unattended. Whoever takes it: start from the measurement that opened the
    ticket (main carried a push-workflow FAILURE for 1h49m04s on 16 Nov — run `44110925937` — with 36
    `DELIVERED` rows and one `MERGED` journaled over it), and remember the read itself already answers
    the *seeing* half — the live trunk is green at filing (three newest push runs all `success` as of
    18 Nov, the 16 Nov red cleared by its own re-run), so the gate question is about the next red, not
    this one. The bad-conclusion vocabulary is deliberately the shared `JQ_BAD_CONCLUSIONS` list beside
    `JQ_CHECKS`; a proposal that diverges the two vocabularies has to say why.
58. **The two allocator legs are still leak-riding, and a session env can carry the leak for a whole
    suite run.** `control: one note can suppress the other` and `a bare run journals nothing` failed
    18 Nov in the `issue-370` unit tree exactly as NEXT row 56 measures — the journal inherited from
    `FACTORY_LOG`/`FACTORY_TICK_UNIT`/`FACTORY_TICK_BRANCH` supplies a `DECISION-ID` line with the
    *working branch's* unit and branch in it (`unit=unclaimed:370 branch=issue-370`), which the fixture
    never planted. Nothing in #370's diff touches the allocator; the same two rows fail identically on
    HEAD in a clean worktree under the same session env. The row-56 repair (plant the journal root, or
    sanitize and say so in the label) is still owed, and until it lands, a unit running the suite from a
    shell that sourced the factory env must unset the three vars and say which run it quotes.
59. **The 98 unanchored pointers this sync refused to place, and the sweep that can name them.** The
   `H-0271` content pass (its entry in `factory/DECISIONS.md`) lists per-entry addresses for every
   unqualified `path:line` whose landing was ambiguous or absent on the merged tree: 85 mid-run, 98
   against the shipped bytes (the 13-row delta is rows the pass itself moved, which no re-run can
   re-place). 15 of them cite content that no longer matches even their author blob -- stale at
   source, carried untouched. The instrument owed is the one `H-0245`'s `What lost:` shape wants:
   emit these rows with the author-blob read beside the merged row so a person re-points them by
   reading code, the only method that works where there is no anchor. `make verify` reads none of
   them; row 53 and row 55 are the same debt at other heads, this row is this head's census.
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

- **Read the subject of a sha before naming its pull request, and re-read a row's number after
  anything appends above it.** Neither is a `path:line` citation, so no gate opens either one:
  `H-0237` shipped `(PR #427, `H-0235`)` for the merge that is PR #410 and trunk `H-0233`, and its
  self-coordinate `this entry at 12481` was true at `d8467a7` and false at `4d81c15` because that
  commit appended one line above it — both of them rode through `check-decisions.py` at rc 0 and
  through two reviews. The commands are `git log -1 --format='%s' <sha>`, which prints
  `Merge pull request #N …`, and `grep -n '^## H-<id>' factory/DECISIONS.md`, and they belong at the
  head you are shipping, not the one you drafted on. Filed as #433.
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
- Ascend by entry id, or do not merge. A `factory/DECISIONS.md` conflict resolves as a union keyed by id in
  ascending order; a whole side is a deletion the gate cannot see and a concatenation is a log out of order
  the gate refuses (`H-0241`, whose `--self-test` fixtures `out-of-order` and `out-of-order-harness` are the
  assertion). Re-aim every pointer the merge moves by row identity -- the pointed-at row plus three context
  rows on each side, matched from the tree where the pointer was true -- never by an offset: this sync's
  offsets were six distinct values across three files and a wrong one keeps its anchor passing while naming
  the wrong statement.
- Anchor every citation you add. `POINTER_DEBT` may only fall, and a merge raises it with nothing written by
  you; `--citations` re-derives the number and raising the constant is legal only beside the entry that
  records the measurement.
- Read the review after the push, not before. `gh pr view 431 --json reviewDecision,reviews,mergeable,mergeStateStatus`
  after the push is what the body may claim, and `gh pr checks 431` at rc 8 means pending, not failed.
