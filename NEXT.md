# NEXT — issue-506 review round (branch `issue-506`, PR #518 open against `main`; this round answers the #518 review of 2026-11-22T12:34:04Z at head `d72deef` — `reviewDecision` read back `CHANGES_REQUESTED` from the API before any byte changed, review newer than the head commit — and ships one blocking control repair plus the review's two text corrections. The review's blocking finding, re-measured before believed: `rn_ctl_shape` grepped the whole `face_ref()` function, which always carries the monolith arm's legitimate `git show`, so the control's grep matched the shipped unmutated file (rc 0) and could never report its plant invisible — the drift simulation (leg scope narrowed from `/^\telse$/` to `/ls-tree -r/`) kept the pre-repair suite 8/8 GREEN with leg (d) blind to the planted read. The repair gives `rn_dir_arm` a file argument and extracts the mutant through the leg's own function, so control and leg share one awk scope: the repaired file goes 7/1 RED on the same drift and stays 8/8 green without it, and the review's literal one-liner (duplicated scope in the control) was measured too — it stays 8/8 green on the drift, which is why the shared extractor shipped over it (`H-0398`, checkpoint `84fc3e5`). Gate verdicts and the post-push review state ride the pull body (#1180); merging belongs to the supervisor and this round merged nothing. The `issue-503` head beneath this one is `H-0394`'s hand-off, superseded only as a heading: PR #517 MERGED into `main` at `23f51c690fcf02dc9d5b257e2df397c26bc213ef` 2026-11-22T10:58:20Z read back from the API at this re-heading, its section bodies byte-kept beneath. No `issue-506` row existed on this board before this head: `d72deef` shipped `H-0395` without re-heading `NEXT.md`, so the effect round's record is `factory/DECISIONS/H-0395.md` and the pull body, not this file)

## What this unit shipped

- `factory/selftest.d/506-refname-taken-set.sh`: `rn_dir_arm` takes an optional file argument
  (`"${1:-$dpn_alloc}"`) and `rn_ctl_shape` greps `rn_dir_arm "$m"` — the control now reads exactly
  the bytes the leg reads, one awk-range definition in the file, and the claim "the planted arm goes
  red" is falsifiable where before it was structurally unfalsifiable (measured 22 Nov: whole-function
  grep hits `git -C "$root" show` on the UNMUTATED file; drift reds the repaired file 7/1; shipped
  bytes green 8/8; reviewer's duplicated-scope variant measured green on the drift). Same file:
  `rn_ctl_dedup`'s comment now names the check that actually goes red — measured
  `rn_shape`-against-the-dedup-mutant fails at the blob-stage **locate** branch, because the plant
  replaces the line `rn_blob_stage` locates by — not the `sort -u` grep the old comment promised.
- `factory/DECISIONS/H-0398.md`, trailer `Decision: H-0398`, checkpoint `84fc3e5`: options, the
  measured preference over the one-liner, what lost, and one disclosed probe slip of this round's
  own (a first probe called the unfixed extractor after reassigning `dpn_alloc`; re-run clean,
  plant visible, shipped arm empty — the 8/8/7/1 pair is the load-bearing evidence either way).
- Nothing else in the tree. `factory/next-decision-id.sh` is untouched — the review approved the
  mechanism ("this is a control fix and not a code fix"); the pull body's claim that the scoping
  "is pinned by a leg that greps the scoping line itself" is re-aimed by this round's body edit,
  since no such leg exists and what pins the scoping now is that the control extracts through
  `rn_dir_arm` itself.
- Shipped by this conflict tick (`factory/DECISIONS/H-0399.md`, trailer `Decision: H-0399`,
  checkpoint `f3da0df`): the sync of `issue-506` onto `origin/main` `d4b2c93` and the
  head-vs-head adjudication of this file's junction, per `H-0393`'s rule as remeasured here:
  this head stays the single `#` (PR #518 is the live unit of this junction — of the two
  open rows `gh pr list --state open` answered at this re-heading, #518 and #520, only #518
  feeds this merge), trunk's `issue-511` head is demoted in place beneath it as the stamped
  `## Record — issue-511 unit, for PR #519` carrying the read-back `MERGED
  2026-11-22T13:10:19Z commit d4b2c9343426fb9734f18b4fe64570c3873e6bc7`, and its three
  section bodies are byte-kept as trunk wrote them; the `## Record — issue-503 unit` row
  both sides demoted (identical bytes except one inserted clause each, measured on the raw
  stages) is kept ONCE on trunk's bytes — the spelling `main` carries today — and this
  branch's clause "the heading below is superseded as a heading only" is dropped as the
  duplicate of that same claim, disclosed because a byte census cannot tell an adjudication
  from a loss. Zero-absence census over both sides' added rows against the merged file:
  this branch {} of {} distinct rows present, trunk {} of {}, and the two absent rows are
  exactly this branch's `issue-503`-record rendering (kept once on trunk's bytes) and
  trunk's head line (demoted in place to the stamped record here). No production byte moved
  by this tick: what it wrote is this file, the entry, and the merge. Gate verdicts ride the
  pull body (#1180); merging belongs to the supervisor and this tick merged nothing.

## What is still owed

- The re-review after this push: the push is measured to spend the standing `CHANGES_REQUESTED`
  row's force (#269), and the ask is the supervisor's poke, not this unit's — this round asked
  nobody, and the post-push read of `reviewDecision,reviews,mergeable,mergeStateStatus` rides the
  pull body.
- If a future round touches this leg file, cite it by function name, never by line: this round's
  edits shifted line numbers inside `factory/selftest.d/506-refname-taken-set.sh` (the pointer
  census answered 0 citations aimed at the file at `d72deef`, so nothing drifted; a future round
  re-derives with `python3 scripts/check-decisions.py --citations`).
- The carried `issue-503` owed rows below (the `docs/factory.md` scratch sentence, the colon in the
  other minted names) stand unchanged and belong to whoever takes them; this head moves none of
  their coordinates.

## Constraints a stranger would break

- Do not duplicate `rn_dir_arm`'s awk range inside another control. Measured, not argued: a control
  carrying its own copy of the scope answers GREEN while the leg's scope has drifted and gone blind
  (8/8, the reviewer's one-liner under drift) — one shared extraction is the whole point of the
  file argument on `rn_dir_arm`, and removing the argument restores a control that cannot fail.
- Do not "simplify" `rn_dir_arm`'s `"${1:-$dpn_alloc}"` back to `$dpn_alloc`: `rn_ctl_shape` plants
  a mutant and extracts it through that argument; the leg's polarity (shipped dir arm answers empty,
  planted dir arm answers the show line) both run through the same call.
- The leg answers on BYTES, never on a timer, on purpose: an answer-based dedup leg would go red on
  a busy supervisor and the dedup mutant answers identically by design. Keep the shape legs as byte
  inspections.

## Record — issue-511 unit, for PR #519 (branch `issue-511`, PR #519 MERGED into `main` at `d4b2c9343426fb9734f18b4fe64570c3873e6bc7` 2026-11-22T13:10:19Z read back from the API at this re-heading; this head is demoted in place, its parenthetical byte-kept except the merge fact it could not know when written, and its three section bodies byte-kept beneath; this unit ships `H-0397`: `notice_legs` in `factory/selftest.sh` opened its locals with a bare `local label` and never assigned it, so under the file's `set -u` the first FAIL in that leg expanded the reporter idiom's unset name and killed the run at the very sentence that would have explained it — reproduced in this tick against a throwaway clone of `23f51c6` with the FAIL branch forced by a one-line mutation: stderr `label: unbound variable` at the FAIL printf, the EXIT trap `RUN INCOMPLETE -- died with exit 1 after 173 ok and 1 FAIL, before any verdict` (the counter incremented before the print, so the run counted a failure it never reported and stopped ~27k lines short of the verdict, every later leg silently unrun), and with the declaration fixed the same forced FAIL printed `FAIL  a halt comments on its unit …` and the suite continued. The green path is why nobody noticed: `ok` rows print literals and shellcheck 0.10.0 reports the shape clean, so a FAIL branch is invisible until the product actually breaks — which is why the class got a pin, not just a fix. The fix is the idiom (assign at the declaration, the 67-row convention in the same file, one line, `1 1` numstat, zero log pointers aimed at that band as measured by `check-decisions.py --citations`); the pin is `factory/selftest.d/511-fail-path-unbound-label.sh`, two legs: a scan of the shipped bytes (body plus every leg file) refusing a top-level function that prints `"$label"` with no `label=` in its own body, and a mutant control that counts the fix's declaration row to exactly one, cuts it out of a copy, re-reads the cut, and refuses to pass unless the scan goes RED naming `notice_legs` — both polarities exercised against pre-fix and post-fix roots before the first commit. Gate verdicts ride the pull body (#1180); merging belongs to the supervisor and this unit merged nothing)

## What this unit shipped

- `factory/selftest.sh`, one row: the `notice_legs` declaration now reads
  `local label="a halt comments on its unit" unit=unclaimed:247 tree=$repo n=0 body=""` — the
  label's own FAIL wording, matching the `ok` row's literal so the two cannot drift. The row's
  number and the file's line count are unchanged, so every pointer above and below keeps its seat.
- `factory/selftest.d/511-fail-path-unbound-label.sh`, two legs on no fixture at all (the scanned
  subject is the shipped tree): (a) a top-level-function scan — column-0 `name()` to column-0 `}` —
  over `factory/selftest.sh` and every `factory/selftest.d/*.sh`, refusing any body that expands
  the literal reporter `"$label"` in a printf while carrying no `label=` assignment; (b) the
  control: copy the suite, anchor-count the fix row to exactly one (zero rows and >1 rows name
  their different causes), cut it, re-read the cut, and require the scan to come back RED naming
  `notice_legs` — the polarity pair the #503 family insists on, since a scanner green on blindness
  and a control red on a plant that never happened are both worse than no pin.
- `factory/DECISIONS/H-0397.md`, trailer `Decision: H-0397`, checkpoint `40b2d21`.
- Nothing else in the enforcement path. `claim_leg`, the reporters, and every other leg are
  untouched; the scan flags exactly zero of the 67 existing `local label=` rows.

## What is still owed

- The scan knows `label` in `printf` only. Every other name's FAIL-path discipline — a hypothetical
  `$tree` or `$want` that a FAIL row prints but no branch assigns — is unpinned, because widening the
  alphabet is a corpus judgement on `read`-assigned locals (`run_selftest_d_legs`' loop var is
  legitimate) that #511 does not price. The scanner is one function; the judgement is the ticket.
- A FAIL branch still has no execution-time proof until the product breaks once; the scan is a
  cheap static partial, not a replacement. A leg family that forces each FAIL polarity (plant a
  defect, expect the FAIL line, per leg) is the real closure and is far bigger than this unit.
- The scanner reads a column-0 `}` as a function end: a heredoc holding one closes a body early
  and HIDES a later defect. None exists today (the control reads `notice_legs` whole and leg (a)
  is green across all 40 leg files); a future heredoc-shaped `}` is the day to teach the state
  machine about here-strings, and leg (a) going quiet-when-it-should-not is the symptom.
- `docs/factory.md` names no reporter-idiom convention today; nothing was added there because every
  line added slides the pointers the logs aim at it, the same priced reason #503 left its sentence
  owed. The legs are the guarantee.

## Constraints a stranger would break

- The control's anchor is the fix row verbatim: `local label="a halt comments on its unit" unit=`.
  If you rewrite `notice_legs`, re-derive the control against the new declaration — the leg tells
  you which of the two causes you hit (0 rows = the fix is gone; >1 = the row was copied), and
  neither is a reason to delete the leg.
- Do not extend the scan to arbitrary variable names by deleting the name filter: several shipped
  legs fill locals with `read` and would fire it; that is the owed corpus judgement, not a quick
  edit. Do not add a second scanner for the other names without the same two-leg polarity — a
  one-legged scan is the vacuous-green the control leg exists to refuse.
- The leg file is itself inside the scanned set. Its own patterns spell the reference as awk
  regexes whose raw bytes are not the literal substring; a future edit there that puts a bare
  `"$label"` printf into a function without `label=` will (correctly) redden leg (a) — spell the
  pattern as this file does instead of adding an exemption.

## Record — issue-503 unit, for PR #517 (branch `issue-503`, PR #517 MERGED into `main` at `23f51c690fcf02dc9d5b257e2df397c26bc213ef` 2026-11-22T10:58:20Z read back from the API at this re-heading; this head is demoted in place, its parenthetical byte-kept except the merge fact it could not know when written, and its three section bodies byte-kept beneath; this unit ships `H-0394`: the per-tick scratch is minted with a PATH-safe name. `factory/factory.sh:1488` folded `:` out of the unit id before `mktemp` — `plock-$(factory_slug "${unit//:/-}")` — because PATH is colon-delimited, an `unclaimed:<n>` unit id carries a colon, `factory_slug` maps `/` alone, and so the minted `/tmp/plock-unclaimed:503-…` made every prepend through `$FACTORY_SCRATCH/bin` contribute the two entries `/tmp/plock-unclaimed` and `503-…/bin`, neither of which exists: the prepend failed silently and the tick's stub binaries were not found. Measured in this tick, whose own scratch carried the colon: the prepend found nothing and PATH's head split exactly as named, while the colon-free twin found the stub. Three legs in `factory/selftest.d/503-scratch-path-safe.sh` — the world rule with both polarities so a control that cannot fail cannot pass, the shipped dispatch (no colon, the name still echoing the unit id, the stub FOUND through the prepend, the directory gone after the pass), and a mutant control that restores the filed mint and must show the colon and the missing stub. The mint line keeps its number and its `scratch=$(mktemp -d` text, so the pointers into it and the #415 family's mutant anchor still land; gate verdicts ride the pull body (#1180); merging belongs to the supervisor and this unit merged nothing. The `issue-495` head beneath this one is `H-0388`/`H-0392`'s hand-off, superseded only as a heading: PR #515 MERGED at `6ddb1d7f0569ef1635e6a6f2010309e076e0a213` 2026-11-22T08:58:21Z and PR #516 MERGED at `e50d96e6fc9079bd6a5a84ea2308bb8f147ef07a` 2026-11-22T10:02:19Z, both read back from the API at this re-heading, their clauses folded, their section bodies byte-kept beneath)

## What this unit shipped

- `factory/factory.sh` line 1488 only: the unit id is colon-folded before `factory_slug` sees it, so
  the minted scratch name is usable PATH text. The line's number is unchanged, it still reads
  `scratch=$(mktemp -d`, and it still begins `${TMPDIR:-/tmp}/plock-`, because pointers in
  `factory/DECISIONS.md` and the #415 family's mutant anchor name that text; the file's line count is
  unchanged for the same reason (every pointer below 1488 keeps its seat).
- `factory/selftest.d/503-scratch-path-safe.sh`, three legs, on the `proc_*` supervisor fixture: (a)
  the PATH split measured in both polarities — a colon-bearing directory hides the stub, the
  colon-free twin shows it, and if the twin is invisible the leg says the fixture's own root is
  unusable rather than going green (that trap is real: this suite runs from a scratch whose parent
  was minted with a colon); (b) the shipped dispatch, asserting the minted name has no colon, still
  echoes the unit id, sits outside the tree, has the prepended stub found, and is deleted by the
  teardown; (c) the control, planting the filed mint byte-for-byte and refusing to pass unless the
  colon returns and the stub goes missing.
- `factory/DECISIONS/H-0394.md`, trailer `Decision: H-0394`, checkpoint `8db39b9`. Disclosed there:
  the checkpoint was taken after the mint was rewritten, so it is a rollback target for the tree and
  not a record of the moment before the decision.
- Nothing else. `factory/common.sh` is untouched: `factory_slug` maps `/` alone and is how every
  branch becomes a tree path, so widening it renames trees — the wrong seam for a PATH-safety rule.

## What is still owed

- `docs/factory.md`'s scratch paragraph (`:1992`) still does not say the minted name is PATH text.
  It was left alone on purpose: the paragraph runs eighteen lines and every line added there slides
  the pointers the decision logs aim at it from 2342 down, so the sentence needs a re-wrap of the
  whole block, which is a unit of its own. The rule is pinned by a leg that goes RED if it regresses,
  which is the cheaper half of the guarantee.
- The colon survives in every OTHER name the harness mints from a unit id — the per-tick log
  (`20261122T100223Z.unclaimed:503.log`), the ledger, the `TICK-START` row. None of those are PATH
  text today (`grep -rn FACTORY_SCRATCH . | grep -i PATH` answers no shipped rows, which is precisely
  why #503 survived: the harness never takes the path its own prompt tells every unit to take). A
  unit that puts one of those names on PATH hits the same split, and the new legs guard only the
  scratch. Whoever widens this either folds `:` at the three mint sites or teaches the classifier to
  stop emitting colon-bearing unit ids.
- The scratch name is still not quoted anywhere in the tick prompt. It does not need quoting now that
  the name has no colon and no space; if a future unit id gains either, the prompt sentence and the
  env export both need it, and leg (b) is the guard that says so first.

## Constraints a stranger would break

- Do not "clean up" the mint into `factory_slug`. The slug is the tree-naming path for branches
  (`issue-249/session` style names), and a colon-free-everywhere slug is a rename of every tree and
  log the supervisor has already written.
- Do not re-aim the #415 control's mutant anchor away from `${TMPDIR:-/tmp}/plock-$(factory_slug`.
  It occurs exactly once in `factory/factory.sh`, and `proc_mutate_factory` refuses an anchor that
  occurs any other number of times — so a mint that duplicates that text anywhere reddens four legs
  that have nothing to do with scratch.
- The scratch teardown matches `"${TMPDIR:-/tmp}"/plock-*`. A mint that stops starting with that
  prefix leaks a directory per tick; leg (b) asserts the directory is gone after the pass, which is
  the only thing that catches it.


## Record — issue-497 unit, for PR #516 (branch `issue-497`, PR #516 MERGED into `main` at `e50d96e6fc9079bd6a5a84ea2308bb8f147ef07a` 2026-11-22T10:02:19Z read back from the API at this re-heading; the heading below is superseded as a heading only this unit ships `H-0390`: the eleven `delivered` rows #497 measured — finished tickets behind honest `Refs` bodies, the oldest 6d01h — lacked one artifact, the author's statement of which slice is last; the statement now exists and is enforced where the author still stands: SKILL.md Step 5 teaches `Last slice of #N.` as plain text beside the documented keyword touching the same number, and `scripts/check-close-verb.py` grows the arm that refuses the statement when no keyword touches that number — the phrase is NOT a keyword and `CLOSER_RE` in `factory/factory.sh` did not widen (#1256: the read side never grows an unmeasured closer), so the checker is the only thing the statement acts on; polarities pinned by a 23-fixture `--self-test` and eight legs in `factory/selftest.d/497-last-slice-declaration.sh`, including the fixture premise (the phrase fires no shipped closer), the negation-POSITION rule, the span shelter, span-blind silence, and one-verdict-per-number against the #457 branch leg; live proof on the corpus itself: PR #394's merged body (the 6d01h row of #325) read verbatim from GitHub exits 0 as-merged, exits 1 with one appended `Last slice of #325.` line, and returns to 0 when the keyword is added; the Step 5 insertion re-aimed four anchored log pointers in the same commit — `SKILL.md:632`→643, `:602`→613, `:605`→616 twice, digits only, gate re-run green, `POINTER_DEBT` unmoved at 589; this branch was cut from `32d9ba7` and `origin/main` had moved to `e8fd6f3` at dispatch and stayed there — the mergeability verdict rides the pull body with its read-back, never here (#1180); merging belongs to the supervisor, and this unit merged nothing — the head above this one is demoted in place, byte-kept, because #488 is undelivered and its head is the status board of the unit still owed a merge, which is the state H-0382's adjudication keeps the board to record; second conflict tick for #516: `main` moved under this pull again — the #515 merge at `6ddb1d7`, MERGED 2026-11-22T08:58:21Z read back from the API at this re-heading, on top of the #514 merge at `e8fd6f3`, MERGED 2026-11-22T07:31:19Z read back the same way — and `git merge origin/main` came back CONFLICTING on exactly two paths, counted by re-running `git merge-file` over the three index-stage files rather than by counting markers: this hand-off's head junction, ONE hunk where this branch's head and trunk's `issue-495` head each re-headed the same base head, and `factory/DECISIONS.md`, TWO hunks, each of them one `Effect:` row whose `path:line` coordinates both sides had re-aimed against its own tree — three coordinates in `H-0256`, four in `H-0274`; adjudication `H-0393`: this head is re-titled and stays the single `#` head, because `gh pr list --state open` answers exactly one row at this re-heading — #516, `CONFLICTING`/`DIRTY`, `APPROVED` — so the board names the unit still owed a merge, which is the state `H-0382`'s adjudication keeps the board to record; trunk's head is demoted IN PLACE to the stamped `## Record — issue-495 unit, round 2 for PR #515` beneath with its parenthetical byte-kept except the merge fact it could not know when written, and the `## Record — issue-489 unit, round 2 for PR #514` row with its three section bodies beneath it is byte-kept as trunk wrote it; the `## Record — NEXT — issue-488` row, which both sides had demoted byte-identically (stage 2 and stage 3, 1920 chars, compared before the choice), is kept ONCE; the log went to the shipped machine union `scripts/merge-decisions.py` (#381) and its receipt rides the pull body with its read-back, never here; checkpoint `83d598b` before the sync and the adjudication; merging belongs to the supervisor and this unit merged nothing)

## What this unit shipped, and what it did not

- Shipped (`factory/DECISIONS/H-0390.md`, trailer `Decision: H-0390`, checkpoint `1e060ea`): the
  author's-statement artifact for multi-slice tickets — a SKILL.md Step 5 paragraph teaching
  `Last slice of #N.`, a new `declared` class and refusal arm in `scripts/check-close-verb.py`
  (phrase pattern, negation-position rule, span-blind silence, one-verdict interaction with the
  #457 branch leg), `factory/selftest.d/497-last-slice-declaration.sh` (eight legs, premise
  first), and the four same-commit digit-only re-aims the Step 5 insertion forced in
  `factory/DECISIONS.md` (`H-0256`, `H-0274`, `H-0285` ×2).
- Did not ship: no change to `factory/factory.sh` — `CLOSER_RE` and `triage.sh` read on exactly as
  measured, by #1256's rule; no change to `scripts/check-commit-verbs.sh` (its `--span-blind`
  contract is now pinned declaration-silent by a leg); nothing closes an umbrella by inference —
  an undeclared row stays honestly `delivered`, and the eleven rows of #497's table stay as
  reported: the next slice on any of them may carry the statement, and nothing in this unit
  re-dispatches them; no batch of the #488 split was run here either, and no merge was performed.
- Shipped by this conflict tick (`factory/DECISIONS/H-0393.md`, trailer `Decision: H-0393`, checkpoint `83d598b`): the sync of `issue-497` onto `origin/main` `6ddb1d7` and the two-path adjudication named in the head line — this hand-off by head-vs-head, `factory/DECISIONS.md` by the shipped machine union `scripts/merge-decisions.py` with its coordinates settled against the MERGED tree rather than against either parent. No production byte moved by this tick: what it wrote is this file, the log's resolution, and its entry. Receipts: the union answered `UNION entries=359 ours=1 theirs=118 drift=2 repointed=0 files=1` at rc 0 with no stage surgery this time (`H-0392`'s `ours=0` was manufactured; this `ours=1` is what git's raw stages already said), the shipped gate over the merged pair, this entry in the corpus, answers 455 entries "
"  "across 20 log(s) and 1426 `path:line` citations, 589 owing an anchor, 0 refused, and `bash factory/selftest.sh` and `make verify` ride the pull body with their read-backs rather than a row here (#389). Both sides' `SKILL.md` insertions are in the merged file: 705 lines against the base's 685, +11 from this branch above every line its rows cite and +9 from trunk below all of them, so this branch's four re-aimed skill-file pointers survived the merge true by the accident of where trunk inserted, and the next trunk leg that inserts above line 600 of that file moves them again.

## The next tick's job, concretely

- Unchanged and owed: run `scripts/split-decisions.py` batch 1 (`--through H-0040`; the dry-run
  numbers are in the demoted head), `make verify` between batches. The allocator's `EMITS`
  contract still governs NEW entries — this unit is its evidence: `next-decision-id.sh
  factory/DECISIONS.md` printed `EMITS factory/DECISIONS/H-0390.md` and that file is this
  head's entry — while every pre-phase-2 entry stays in the monoliths; do not hand-file an old
  entry (#429, H-0263, H-0383).
- If a future leg touches `.opencode/skills/factory/SKILL.md` above ~line 389, expect the same
  four pointers (`SKILL.md:613`, `:616` ×2, `:643`, as re-aimed at this head) to move again; ask
  `python3 scripts/check-decisions.py --citations .opencode/skills/factory/SKILL.md` before the
  edit, re-aim digits only, and re-run the gate in the same commit — H-0139's rule, and this
  unit's precedent for following it.

- Carried forward from the demoted `issue-495` record so the live board says it too: #515's pull review of 22 Nov left two non-blocking asks undischarged — restate the widened-alphabet table against the head that carries it rather than against `origin/main`, and ship the pricing pass as a re-runnable leg so the alphabet choice can be re-measured without re-deriving it. They belong to whoever takes `scripts/check-decisions.py`'s alphabet next, and they are written in full in the `## What is still owed` body beneath the `## Record — issue-495 unit` row; this row exists so a reader of the board alone does not lose them, which is the #337 failure.
- Carried forward: the merged-tree re-aim recipe the same review asked for is `factory/DECISIONS/H-0392.md` plus the shipped union, and this tick is its second live use; if the index-stage step of that recipe is ever needed twice more it has stopped being a judgement and wants a tool. Nothing was index-operated in this tick unless the union refused, and its receipt says which.

- Allocate the decision id AFTER the long verification, not before it. Measured at this tick: `bash factory/next-decision-id.sh factory/DECISIONS` was run before `make verify`, that run took about twelve minutes, and the re-read the skill requires immediately before the entry commit came back `REFRESH-BLIND the refs behind this answer were last refreshed 306s ago, past the 300s bound` — the answer was still right here (`H-0393` both times, nothing pushed in between), but a bound the tick blows by construction teaches nothing, so either verify first and allocate last, or `git fetch --prune origin` again between the two readings. `H-0393`'s `Id:` row carries both journalled lines.

## Given a free choice, next

- Carry the statement into the queue's prose, not its reads: `factory/triage.sh`'s `delivered`
  report could one day count `declared=<yes|no>` — as a REPORT only. #497's own boundary is that
  the read side closes nothing it has not measured, so a tick tempted to widen `CLOSER_RE` with
  the phrase stops at H-0390's What-lost first; only a live measurement of what GitHub actually
  does with the phrase could move the read side, and measuring it means a throwaway repo, never
  an issue in this one.

## Record — issue-495 unit, round 2 for PR #515 (branch `issue-495`, PR #515 MERGED into `main` at `6ddb1d7` 2026-11-22T08:58:21Z read back from the API at this re-heading; this unit ships `H-0388`: `scripts/check-decisions.py` accepted an anchor cut in the middle of a token — five rows of `factory/DECISIONS.md` quote 53 of the 62 characters on the substitution row in `factory/common.sh` and stayed green across `431b24b` renaming that default from `needs-human` to `question`, because the surviving prefix is identical for every value the assignment has ever carried; measured one step past the ticket on that commit's own diff — six rows carrying the whole `:-needs-human}` quotation rewritten, five shipped ending at `:-`, one kept whole — plus a sixth cut the ticket never named, in `factory/factory.sh`, where the row says `-name` and the quote opens on `name`. The accept arm asked whether the fragment is on the promised row (#374) and whether it answers any other row (#440); it never asked where the quotation stops, so containment stayed true for any fragment cut inside a word. `anchor_cut()` now reads the same joined region the accept arm built and refuses a fragment whose touching character continues the word: a sixth must-be-zero bucket, `midtoken`, refused on sight, holding the ratchet while one exists, never counted into `POINTER_DEBT` because finishing the quotation is the whole repair and counting it would teach lowering the constant to hide a cut. The token alphabet stays word characters on a priced basis — adding `:` costs 29 further quotations, `.` 22, their union 50, `/` and `-` one row each, and every one of them a quote ending at a completed word the cited row merely follows with punctuation; a length floor cannot express the defect at all, since `MIN_ANCHOR` is 16, the corpus's whole quotations run 16..101 characters, and the two cut ones are 53 and 22. The 53/62 pair is the fixture, and the legs read both of the fixture's preconditions off the predicate before asserting a single refusal, so a predicate that fires on everything cannot pass a corpus of cuts: `--self-test` went 98 must-fail/68 must-pass to 105/78; the same predicate against untouched `origin/main` refuses 6 of the 577 anchored and resolvable citations out of 1,663 scanned, and against the shipped tree refuses 0, with `POINTER_DEBT` unchanged at 589; checkpoint `e7984c7` before the judgement call; two slips disclosed in the entry rather than backdated — a re-aim map applied twice, which shifted 80 coordinates a second time and was caught by the gate's own 80-row read-back, and two coordinates written from memory (one self-test fixture's row, one bare `SKILL.md` path) that neither line says, both re-derived from the tree before the commit; gate verdicts ride the pull body with their read-backs, never here (#1180); merging belongs to the supervisor and this unit merged nothing; this sync takes `origin/main` `2b4686d` and adjudicates the two-path conflict as `H-0392` — head junction by the `H-0382` precedent, with main's `issue-489` head demoted in place to the stamped `## Record` beneath (PR #514 MERGED at `e8fd6f3` 2026-11-22T07:31:19Z read back from the API at this re-heading, its clause folded, bodies byte-kept), and the log folded by the shipped union `scripts/merge-decisions.py` after a disclosed index-stage surgery that states which side owns which row, receipt `UNION entries=359 ours=0 theirs=120 drift=0 repointed=22 files=1` with the shipped certifier green and `POINTER_DEBT` unchanged at 589; the merged-tree re-aim recipe the pull review of 22 Nov asked for is `factory/DECISIONS/H-0392.md` plus the shipped tool, and `H-0388` carries its `REAIM_LOGS` correction beside this line; that head demoted in place by this sync, its parenthetical byte-kept except the merge fact it could not know when written, and the `## Record — issue-489` row with its three section bodies beneath it byte-kept as trunk wrote it)

## Record — issue-489 unit, round 2 for PR #514 (branch `issue-489`, PR #514 MERGED into `main` at `e8fd6f3` 2026-11-22T07:31:19Z read back from the API at this re-heading; this unit ships `H-0386` beside the branch's shipped `H-0384` and `H-0385`: the leak #489 filed — `make verify` from a tick environment fast-forwards the REAL installed tree — is closed for the filed shape by a per-pass pin in `factory/selftest.sh` and an ownership gate in `factory_supervisor_repair`, both already on the branch; what this round adds is the measurement of where that gate is blind, and the baseline re-measured rather than quoted — 19 fast-forwards in the operator's reflog since #489 was filed, 10 of them journalled by the supervisor within the same second, 9 not — a ceiling on the leak
rather than a count of it, since the reflog cannot tell a supervisor merge from a person pulling —
and the operator's checkout read back at `42dea11`, clean, 2 behind trunk: the leak's own
precondition still live while this unit's gate run was in flight, and it stayed where it was; that head demoted in place by this sync; its three section bodies are byte-kept beneath this line)

## What this unit shipped, and what it did not

- Inherited, not this round's work (the four commits `73513ea` the pre-H-0384 checkpoint,
  `65fcc72` the pin, `1390dca` the pre-H-0385 checkpoint, `1076b1e` the gate; the four shas first
  written on this line — `305d073` `43e8c39` `d63c0d3` `7695b3e` — are no object on any ref and
  were replaced 2026-11-22, on #514's review):
  `H-0384`'s per-pass pin — the `proc_run` line in `factory/selftest.sh` carries
  `FACTORY_UNITS_DIR="$proc_work/units-pass"` and
  `FACTORY_SERVICE_UNIT=proc-pass-factory.service` — and `H-0385`'s ownership gate in
  `factory_supervisor_repair`, refused-token `SUPERVISOR-FF-REFUSED not-this-pass`, with its
  containment legs in `factory/selftest.d/489-installed-tree-containment.sh` (a: pin extracted,
  canary moves, RED; b: gate extracted, canary moves, RED; c: shape greps of the shipped pin, the
  knobs and the `proc_extra_env` prohibition — GREEN whether or not any supervisor pass ever
  spawns, corrected to that polarity 2026-11-22 on #514's review).
- Shipped (review round for #514, 2026-11-22, prose-only): the record-accuracy repairs that review
  `6010672599` named — the `proc_extra_env` ordering claim corrected to what the code does (the
  array expands BEFORE `proc_run`'s shipped list, so an entry naming a pinned name is nullified by
  the pin, never an override) at its four shipped sites, with a dated `Correction` bullet on
  `H-0384` and the four dead commit locators on the `Inherited` row replaced by the four that
  ship; leg (c)'s polarity restated as shape-greps. No behaviour byte moved — text-only motion,
  so the suite verdict carries.
- Shipped (`factory/DECISIONS/H-0386.md`, trailer `Decision: H-0386`, checkpoint `4600323`): the
  measured limit of that gate. `$FACTORY_ROOT` is *derived*, by `_factory_main` in
  `factory/common.sh`, from the first `worktree` row of `git worktree list --porcelain`, and git
  prints the MAIN checkout first even when the command runs in a linked worktree — so a pass
  started from a linked worktree with the timer's exported unit name and no explicit
  `FACTORY_ROOT` resolves both sides of the gate to the operator's checkout and the gate does not
  fire. `H-0385`'s premise "A pass knows that relation through exactly one name" holds only when a
  caller sets the name. Probe and verdicts are in the entry; the probe writes nothing.
- Shipped: the gate run taken with the incident's own precondition live rather than neutralised —
  `make verify` from a tick environment with `FACTORY_SERVICE_UNIT=parental-lockout-factory.service`
  deliberately left in the environment, while the operator's checkout sat behind trunk with its
  last reflog move (03:26:13Z) one of the 9 unjournalled. Read the verdict for what it tests: root
  resolved equal to installed, so the gate could not fire and its silence is not evidence about
  containment; what the run can say is whether the pin held and whether the tree moved.
- Disclosed, because a hand-off must not inherit a false claim about the control: this unit ran
  `git fetch --prune origin` in the operator's checkout (remote-tracking refs only) and intended
  to pin that tree to trunk as the null control; the live read afterward is HEAD `42dea11`, clean,
  unchanged, with no reflog entry for such a move. So the control is NOT "root == installed ==
  trunk" — it is "root == installed == behind trunk", which is the harsher and more useful shape,
  and it is what the paragraph above describes. Nothing in the operator's checkout was written by
  this unit.
- Did not ship: the widening. No byte of `factory/common.sh` or `factory/selftest.sh` moved in
  this round.

## What is still owed (all of it, none silent)

1. The supervisor reviews and merges; this unit asked nobody and merged nothing.
2. H-0386's widening, as a unit and not a drive-by: compare the installed tree against the
   canonicalised parent of `$FACTORY_DIR` instead of `$FACTORY_ROOT`, and in the SAME commit
   re-verdict the two families that would otherwise change verdict for the wrong reason — the r342
   legs, which source the shipped `common.sh` from the harness tree while planting a fixture
   installed tree (under the new semantics every one of them reads foreign and the gate fires
   inside legs that exist to test other arms), and the `installed? no` legs, which plant a
   non-installed root and take their refusal from an earlier arm. Falsifier to write with it: the
   H-0386 probe must flip from `does NOT refuse` to a journalled refusal.
3. Read the 9 before acting on them. Each is a fast-forward in the operator's checkout with no
   `SUPERVISOR-FF` line within ±3s of it, and the reflog cannot tell a supervisor merge from a
   person typing `git pull` or from a session's own merge — so 9 is a CEILING on the leak, not a
   count of it. What makes them worth the ticket is that #489 measured an unattended writer to that
   tree doing exactly this, and the harness is the only unattended writer identified so far; proving
   any single one of the 9 was it needs a per-move cause the reflog does not carry. The move that
   left the tree where it is now, `2b4686d -> 42dea11` at 03:26:13Z, matches no journal line in
   `factory/log/` at its second.
4. Whoever lands the widening should also decide whether a pass whose derived `$FACTORY_ROOT`
   equals the installed tree while it runs elsewhere deserves a journal line of its own: the pair
   is already visible in the journal (`SUPERVISOR-AT` prints root and harness side by side), only
   the refusal is missing.
5. Unchanged from `H-0383`'s rows: the tick-environment leak in the fence-comparison legs, and the
   allocator cost of #506. Neither is touched by this branch.
6. From #515's pull review of 22 Nov, the non-blocking asks it said not to hold the merge for:
   restate the widened-alphabet table in the pull body against this head (the shipped counts are
   measured against `origin/main`, and this head's own additions move them), and ship the
   pricing pass as a re-runnable leg so the alphabet choice can be re-measured without
   re-deriving it; shipping a guarded re-aim tool (`REAIM_LOGS`-shaped) with its own falsifier
   is the same ask's tool half, recorded on `H-0388`.
7. The merged-tree re-aim recipe the same review asked for shipped at this sync as
   `factory/DECISIONS/H-0392.md` plus the shipped union; if a future conflict makes the
   index-stage surgery a repeated procedure rather than a one-off judgement, that is the signal
   it wants a tool, and the leg in item 6 is where it starts.

## Constraints a stranger would break here

- The pinned pair on the `proc_run` line in `factory/selftest.sh` is a defence, not tidiness
  (`H-0384`). `factory/selftest.d/489-installed-tree-containment.sh` prints RED against its
  extraction, so removing it cannot land quietly; and no selftest may set `FACTORY_SERVICE_UNIT` to
  a name the real unit answers to — leg (c) is pure shape-greps of `factory/selftest.sh` and passes
  whether or not any pass spawns (corrected 2026-11-22 on #514's review); the legs that watch a
  real pass run are (a) and (b), so deleting a spawn reddens those, never (c).
- A new fixture that spawns the real supervisor must pin the pair AND expect
  `SUPERVISOR-FF-REFUSED not-this-pass`. A leg whose refusal sentence comes from `installed? no`
  tests an earlier arm and proves nothing about containment — the false-pass trap `H-0385` names in
  its own `Why`.
- Nothing may infer "the tree I am running in" from the first row of `git worktree list --porcelain`:
  measured 2026-11-22 from a linked worktree, git prints the main checkout first. `_factory_main` in
  `factory/common.sh` does exactly that, and `H-0386` is the consequence.
- Measuring this defect's reflog: `%ct` is the TARGET COMMIT's committer date, not the time the ref
  moved. Parse the selector (`--date=iso-strict`, then `HEAD@{...}`). The retained reflog holds 72
  fast-forward entries from 2026-10-23T21:23:45Z to 2026-11-22T03:26:13Z; the first attempt at this
  baseline reported 22 for a window it did not actually measure.
- A journal match for a fast-forward must be same-second (±3s), not same-minute: passes run seconds
  apart, and a minute window invented a match on 2026-11-22 for the 03:26:13Z move whose nearest
  journalled event was 43 minutes away.

## Record — NEXT — issue-488 (branch `issue-488`, PR open against `main`; this unit ships `H-0383`: #488 is phase 3 of the decision-log split — 437 entries still live inside the two monoliths (78 `D-` rows over 5,651 lines, 359 `H-` rows over 19,253 lines at `42dea11`), and phase 3 moves them out in batches of ~40 with `make verify` between them. What shipped is the machine that does a batch: `scripts/split-decisions.py` files the oldest entries through `--through <id>` into `DECISIONS/<id>.md` / `factory/DECISIONS/<id>.md` byte for byte AND re-aims every `path:line` citation in the union that points into the log whose rows it moved, in the same commit — one row map (a moved row becomes its entry's file at `row - heading + 1`, a staying row loses the lines removed above it, the preamble is above every cut), continuations expanded into written paths, revision-qualified pointers left alone, `range-split` and `already-filed` refused by name. The proof for the ~41 pointers nothing judges is the ~1,360 that re-verify: the rewrite never touches an anchor's bytes, so wrong arithmetic is a red gate, not a drift (H-0383). DRY-RUN ONLY — zero batches have been run: `--through H-0040` reports 38 entries filed, 19,253 → 18,369 lines, 48 citations re-aimed, no refusal; the 45-minute ceiling arrived before the first write, so the split itself is the next tick's work and every number here is a dry run that wrote nothing; `make verify` was not run on this branch's head — its 5m15s does not fit the ceiling either, so CI on the pull request is the first full gate and its verdict rides the pull body with its read-back, never here (#1180); merging belongs to the supervisor, and this unit merged nothing — the head above this one is demoted in place, byte-kept, because no pull request is open on trunk (`gh pr list --state open` → `[]`, read at this re-heading) and the unit owed a merge is the one that speaks)
## Record — NEXT — issue-477 (branch `issue-477`, PR open against `main`; this unit ships `H-0380`: the stop hold the journal cannot attribute gets a subject sampled where it can be seen — `2026-11-19T17:11:09Z REAP-HELD hold=44s how=unexplained ended=624s open=1` is 44 s the timer could not re-arm with nothing named, and the only instrument that can name it is one standing in the same cgroup when systemd sends the stop: the pass spawns `factory/stop-window-probe.sh` before `factory_body`, the probe answers its own `SIGTERM` by snapshotting `cgroup.procs` and dying, and the next pass's reap read attaches the match as `stop=` on the `REAP-HELD` row and in the `--status` sentence, with the four no-subject answers kept as four facts (`none`, `unwritten`, `unread`, `0`) because "could not look" and "looked, nobody there" are different defects; asking the manager was refused as the authority `H-0118` will not borrow, and asking it late as asking about a cgroup the kernel has forgotten — re-reading the 19 Nov stop on this box answers `-- No entries --`; ten legs in `factory/selftest.d/477-stop-window.sh`, the window and its negative control in one leg because each is the other's falsifier, plus five on the probe itself, including its `t=` exponent (`date +%s%N` prints 19 digits here and the first handler that ran wrote 1970) and a 900 ms bound on its own death, because a probe slow to die is itself a holder of the stop it watches; checkpoint `24d2f6b`; second conflict tick for #512: `main` moved under this pull again — the #508 merge at `2b4686d`, MERGED 2026-11-22T02:25:21Z read back from the API at this re-heading — and `git merge origin/main` came back CONFLICTING on exactly one path, counted by re-running `git merge-file` over the three index-stage files rather than by counting markers: this hand-off's head junction, ONE hunk where this branch's head and trunk's `issue-464` round-2 head each re-headed the same base head; `factory/DECISIONS.md`, `factory/selftest.sh`, `factory/common.sh`, `factory/next-decision-id.sh`, `docs/factory.md` and `.opencode/skills/factory/SKILL.md` auto-merged, so their merged bytes are what every pointer in the log had to be read against; adjudication `H-0382`: this head is re-titled and stays the single `#` head — the status board names the unit still owed a merge, and #512 is open while #508 reads MERGED — trunk's `issue-464` head is demoted in place to the stamped `## Record` beneath it with its three section bodies byte-kept, and this branch's three `## Record` stub rows (`issue-469`, `issue-468`, `issue-456`) are kept ONCE, superseded by trunk's stamped full-parenthetical rows below, which carry every clause of all three; checkpoint `3a0138d` before the sync and the adjudication; gate verdicts ride the pull body with their read-backs, never here (#1180); merging belongs to the supervisor)

## Record — issue-464 conflict tick, round 2 for PR #508 (branch `issue-464`, PR #508 MERGED into `main` at `2b4686d` 2026-11-22T02:25:21Z read back from the API at this re-heading; this unit is the second conflict tick for #508 and ships `H-0381`: `main` moved under this pull again — the #504 merge at `252d488`, MERGED 2026-11-22T00:36:21Z read back from the API at this re-heading — and `git merge origin/main` came back CONFLICTING on exactly two paths, counted by re-running `git merge-file` over the three index-stage files rather than by counting markers: this hand-off's head junction, ONE hunk where this branch's phase-2 head and trunk's `issue-448` round-3 head each re-headed the base head, and `factory/DECISIONS.md`, 57 hunks; `factory/selftest.sh` auto-merged, so its merged bytes are what every pointer in the log had to be read against; the log went to the shipped machine union `scripts/merge-decisions.py` (#381) and that union REFUSED it by name — `UNION-REFUSED reason=id-collision factory/DECISIONS.md H-0303 differs on both sides by more than coordinate digits` — so the documented order was followed: refuse, hand-settle the named entry, let the machine take the rest. The named entry was folded row by row under the union's own accept rules (89 rows each side: 86 identical, TWO digits-only drift, ONE where `base == ours` and trunk alone lengthened the quoted anchor, ZERO where both sides moved words), and the shipped re-aim then moved the drift rows to the merged tree's coordinates and the shipped certifier passed; adjudication `H-0381`: this head is re-titled and sits on top as the live status board of the unit still owed a merge — #508 open, #504 merged — this branch's round-1 head is demoted in place to the `## Record` beneath with every body byte-kept, trunk's `issue-448` round-3 head is demoted in place to a short `## Record` stamped with its merge (#504 at `252d488`, 2026-11-22T00:36:21Z read back from the API at this re-heading) with its three section bodies byte-kept beneath it, and the duplicated `## Record — issue-469` row is kept ONCE as one row carrying both parents' bytes — this branch's full-parenthetical body with trunk's `a6e09ee` merge stamp and its demotion clause folded in, one row transformed against each parent and no clause dropped; the zero-absence census against both parents is recorded in `factory/DECISIONS/H-0381.md`; checkpoint `2414821` before the sync and the adjudication; gate verdicts ride the pull body with their read-backs, never here (#1180); merging belongs to the supervisor and this unit merged nothing; that head demoted in place by this sync; its three section bodies are byte-kept beneath this line)

## What this unit shipped, and what it did not

- Shipped (`factory/DECISIONS/H-0381.md`, trailer `Decision: H-0381`, checkpoint `2414821`): the
  sync of `issue-464` onto `origin/main` `252d488` and the two-path adjudication named in the head
  line — this hand-off by head-vs-head, `factory/DECISIONS.md` by `scripts/merge-decisions.py`
  except for the one entry that script refuses by name, which was folded under its own accept rules
  at row granularity and then handed back to its re-aim and its certifier. The union's own verdict
  line rides the pull body, never here.
- Shipped: no production byte moved by this tick. `factory/selftest.sh`, `factory/common.sh`,
  `factory/next-decision-id.sh`, `docs/factory.md` and `.github/workflows/ci.yml` land exactly as
  the merge wrote them; what this tick wrote is this file, the log's resolution, and its entry.
- Shipped: the row-level truth of the refused entry, measured before anything was written — 89 rows
  per side, 86 identical, 2 digits-only drift, 1 where this branch's row is byte-for-byte base's,
  0 where both sides moved words — and each of the three coordinates the machine then chose was
  verified by opening the merged `factory/selftest.sh` at that line and reading the anchor.
- Did not ship: phase 3 (batched split of the monoliths), phase 4 (retiring the monoliths, not an
  agent's call), hand-filed conversions (`H-0263` still stands), and the #506 allocator-cost work —
  none of them touched by this round.

## What is still owed (all of it, none silent)

1. The supervisor reviews and merges; this unit asked nobody and merged nothing. A push into a pull
   request that already carries a review spends that approval and neither command reports it, so
   what the post-push read-back said rides in the pull-request body beside its own read-backs,
   never here.
2. `H-0372` is named in the logs only as the stale checkpoint label (the gate's id-coverage report
   lists it as a hole, report-only); whoever allocates it retires the hole by existing.
3. Everything the `Record — issue-464 unit, round 1 for PR #508` block beneath owes, unrenumbered:
   phase 3 and phase 4, the filed-entry rule (`H-0263`), the #506 cost work, and the parse
   contracts its `Constraints` block names.
4. Everything the `Record — issue-448 conflict tick, round 3 for PR #504` block beneath owes,
   unrenumbered — the #482 and #489 environment leaks, the suite-wide `$TMPDIR` precondition, the
   remote-only leg for the allocator's liveness arm, and #503's colon-in-scratch poison — and
   everything the two `Record — issue-448` blocks under it owe.
5. What the `Record — issue-469`, `Record — issue-468` and `Record — issue-456` blocks carry
   (#506's union-walk cost, the two deferred honesty clauses) is inherited by content, not
   re-derived here; both pulls merged, #510 at `a6e09ee` and #509 at `78502fd`.

## Constraints a stranger would break here

- `NEXT.md` merges are head-vs-head, never content-vs-content: exactly one `#` head, the unit still
  owed a merge keeps it, a merged unit's head becomes a `## Record` stamped with the merge stamp
  read from the API at the re-heading, and every dropped or transformed line is named out loud in
  the entry and checked by a census against BOTH parents. `scripts/check-next-md.sh` enforces the
  one-head rule and refuses a hand-off that names a path the tree does not have.
- `factory/DECISIONS.md` is the machine union's to settle: inside a conflicted merge run `python3
  scripts/merge-decisions.py` and hand-adjudicate only what it refuses by name. A refusal writes
  nothing and stops every clean hunk with it, so hand-adjudicating the whole file is not the answer
  to one named entry: fold THAT entry under the union's own accept rules at the granularity it
  lacks — per row, one side equal to base or the two coordinates-only apart — hand it back, and let
  the shipped re-aim and the shipped certifier have the last word. A `path:line` conflict inside a
  log is a drifted pointer, not a content dispute: fold the digits out of both sides, and whatever
  is left identical is drift; then move only digits, to where the gate says the anchor lives, and
  open the merged file at that number and read the anchor yourself.
- stdout of `factory/next-decision-id.sh` must stay the id alone; `EMITS` is stderr, beside the
  refresh and pending notes.
- The journalled line's field order is a parse contract: `$4` tree, `$5` then `path=` at `$6`, the
  step-over note after it, the parenthetical as tail — a reader that REQUIRES `path=` calls every
  pre-#464 allocation unearned.
- `$FACTORY_RUN/provenance/<unit>.headings` is keyed ENTRY-PATH-first (`DECISIONS/D-0011.md D-0011`);
  a reader matching the old log-name key exempts nothing.
- `dpn_prove`'s sixth positional argument is the STATEDIR (run/ledger derive from it); passing a
  ledger dir there renders `baseline=none` and UNEARNED where UNJOURNALABLE is owed — that is exactly
  how f4 failed before its repair.
- Re-aim citations only by opening the file and locating the anchor text, and edit digits, never
  whole rows; `make verify` green per commit; `POINTER_DEBT` may only fall.
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or an
  approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.

## Record — issue-464 unit, round 1 for PR #508 (branch `issue-464`, PR open against `main`; this unit is phase 2 of the decision-log layout: the allocator's journalled answer and the provenance journal now name the ENTRY FILE (`path=` as the line's sixth field, the same string the stderr `EMITS` line hands the writer), the end-of-tick comparison and its TICK-START `DECISION-ID-BASELINE` count the two filed faces keyed by entry path, and the filed verdicts name the file (`log=DECISIONS/<id>.md`, inherited bundles `log=DECISIONS/ ids=…`) — stacking on #456/#505's allocator half, which this branch carries by merging `origin/issue-456` at `7f771a1`; the allocator answered `H-0371` through its own tick context after the REFRESH-BLIND read, the entry is `factory/DECISIONS/H-0371.md` and the pre-effect rollback target is `17f895c` (its `pre-H-0372` label is stale, disclosed in the entry); this hand-off demotes the `issue-456` head in place to the Record beneath with every body byte-kept; this base sync takes `origin/main` `a6e09ee` (the #510 merge, MERGED 2026-11-21T23:07:22Z read back from the API at this re-heading) and trunk's hand-off head collided with this head region: adjudicated `H-0377` by the `issue-433`-rounds precedent — this head sits on top, trunk's `issue-469` head is demoted in place to the `## Record —` beneath with its parenthetical byte-kept under the #510 merge stamp, trunk's `issue-468` Record stub is carried above the `issue-456` Record, and trunk's bare `issue-456` stub is superseded by this branch's full-parenthetical row, a superset of its bytes, the replacement recorded in `H-0377`; sync checkpoint `85aa0f3`; gate verdicts ride the pull body with their read-backs, never here (#1180); merging belongs to the supervisor)

## What this branch shipped, and what it did not

- Shipped (`factory/DECISIONS/H-0371.md`, trailer `Decision: H-0371`, checkpoint `17f895c`): the
  journalled `DECISION-ID` line carries `path=` sixth, from one variable with `EMITS` so the two
  cannot drift, ahead of the step-over note which stays the variable tail; pre-#464 five-field lines
  certify ids unchanged. `factory_decision_headings` and the baseline read iterate all four faces
  (`DECISIONS.md`, `factory/DECISIONS.md`, `DECISIONS/`, `factory/DECISIONS/`), key rows by entry
  path, and the reports name the file; the dedupe that keeps an inherited filed bundle to one
  `UNJOURNALABLE` line spans BOTH face loops (a plant proved one-loop dedupe emitted the ids twice).
- Shipped: selftest legs f1-f4 with controls f1/f2 identity and planted-divergence, f3 filed-UNEARNED
  shape + earned-by-old-shape control + face-list blinding, f4 #427-in-filed-layout + empty-filed
  polarity; the MUTANT-345 anchor retargeted; the #303 leg `a unit answer journals itself` re-aimed
  to the shipped six-field line (a green gate on the old literal would have meant `path=` never
  shipped). f4's repair is recorded in the constraints below — its sixth argument is the STATEDIR.
- Shipped: every drifted `path:line` citation in the logs re-aimed by the checker's own rule (anchor
  opened, digits edited, line counts untouched, verified twice by an independent pass); net
  160 rows of `factory/DECISIONS.md`, `DECISIONS.md` untouched; `POINTER_DEBT` re-measured at this
  head: 589, unchanged — the ratchet is judged again.
- Did not ship: phase 3 (batched split, waits for this), phase 4 (removing monoliths, not an agent's
  call), hand-filed conversions (H-0263 still stands), and the #506 cost work (unchanged here).

## What is still owed (all of it, none silent)

1. The supervisor reviews and merges; this unit asked nobody and merged nothing. #505 MERGED
   2026-11-21T19:58:22Z read back from the API at this sync, and this branch already carries its
   allocator bytes from `7f771a1`: the `a6e09ee` sync moved none of those files (its staged halves
   are `ci.yml`, the capture harness, `H-0374`/`D-0093` and the #469 leg file only).
2. `H-0372` is named in the logs only as the stale checkpoint label (the gate's id-coverage report
   lists it as a hole, report-only); whoever allocates it retires the hole by existing.
3. Carried from the `issue-456` Record beneath: its owed items 1-4 and constraints stay true there;
   nothing in this unit retired them.

## Constraints a stranger would break here

- stdout of `factory/next-decision-id.sh` must stay the id alone; `EMITS` is stderr, beside the
  refresh and pending notes.
- The journalled line's field order is a parse contract: `$4` tree, `$5` then `path=` at `$6`, the
  step-over note after it, the parenthetical as tail — a reader that REQUIRES `path=` calls every
  pre-#464 allocation unearned.
- `$FACTORY_RUN/provenance/<unit>.headings` is keyed ENTRY-PATH-first (`DECISIONS/D-0011.md D-0011`);
  a reader matching the old log-name key exempts nothing.
- `dpn_prove`'s sixth positional argument is the STATEDIR (run/ledger derive from it); passing a
  ledger dir there renders `baseline=none` and UNEARNED where UNJOURNALABLE is owed — that is exactly
  how f4 failed before its repair.
- Re-aim citations only by opening the file and locating the anchor text, and edit digits, never
  whole rows; `make verify` green per commit; `POINTER_DEBT` may only fall.

## Record — issue-448 conflict tick, round 3 for PR #504 (branch `issue-448`, PR #504 MERGED into `main` at `252d488` 2026-11-22T00:36:21Z read back from the API at this re-heading; this unit ships `H-0378`: the third base sync of that branch met trunk's #510 merge as a single `NEXT.md` head-vs-head hunk and adjudicated it with the open unit keeping the head; that head demoted in place by this sync, its entry filed as `factory/DECISIONS/H-0378.md` and its full hand-off text opening with `git show 252d488:NEXT.md`; its three section bodies are byte-kept beneath this line)

## What this unit shipped, and what it did not

- Shipped: the sync of `issue-448` onto `origin/main` `a6e09ee` (the #510 merge) and the
  head-vs-head adjudication named in the head line. No shipped line of `.github/workflows/ci.yml`,
  `factory/selftest.d/469-pinned-fetch-retry.sh` or `factory/DECISIONS.md` was hand-edited by this
  round: trunk's transport-retry edit, its new selftest leg and its filed `H-0374` entry are the
  merge's own output, and the monolith holds no hunk because trunk's entry arrived as its own file —
  which is why no pointer needed re-aiming this round.
- Shipped: `factory/DECISIONS/H-0378.md`, the entry for this round's call (which head is live, what
  a merged trunk head is demoted into, which duplicated line is dropped), in the same commit as
  this file's resolution.
- Did not ship: the standing-precondition question the round beneath names is still open, and
  #503's colon-in-scratch poison is still only named.

## What is still owed (all of it, none silent)

1. The supervisor merges; this unit merged nothing and asked nobody. If the PR sits
   `REVIEW_REQUIRED` with an empty `/requested_reviewers`, the ask is the supervisor's
   `factory_request_review` poke (`AGENTS.md`), not this unit's.
2. Everything the `Record — issue-448 conflict tick, round 2` block beneath still owes, and
   everything the `Record — issue-448 unit` block beneath it owes, unrenumbered: the #482 and #489
   environment leaks, the remote-only leg for the allocator's liveness arm, the suite-wide
   `$TMPDIR` precondition, and #503.
3. Everything trunk's `Record — issue-469` and `Record — issue-468` blocks owe, unrenumbered
   (#506's union-walk cost, the two deferred honesty clauses). Those pulls merged — #510 at
   `a6e09ee` and #509 at `78502fd`, both stamps read back from the API at this re-heading — so
   those rows are trunk's now and this branch inherits them by content, not by re-derivation.

## Constraints a stranger would break here

- `NEXT.md` merges are head-vs-head, not content-vs-content: keep both records, keep exactly one
  `#` head, demote a merged unit's head to `## Record` stamped with its merge read from the API at
  the re-heading, and say out loud which duplicated line you dropped. `scripts/check-next-md.sh`
  enforces the one-head rule (measured mid-merge this round: the unresolved file printed
  `carries 2 level-1 heading(s), not exactly one` at rc 1). The two demotion styles here are not
  interchangeable: a trunk head that merged becomes a short stamped `## Record` line because its
  full hand-off text is on any reader's disk as its filed decision entry and opens with
  `git show <merge>:NEXT.md`, while this branch's own previous head is byte-kept into its
  `## Record` line because until PR #504 merges, its only durable home is this file.
- A `path:line` conflict inside a decision log is a drifted pointer, never a content dispute: fold
  the digits out of both sides and whatever is left identical is drift; take either side, then ask
  the gate where each pointer really lives (`python3 scripts/check-decisions.py --citations` prints
  "it is at line N now" per row) and move only digits. Hand-carrying either side's digits keeps the
  defect the merge was supposed to settle (`H-0376`).
- The one real conflict a log merge can hide is an anchor whose text died with one side's edit:
  when the gate answers with two line numbers for one citation, the anchor answers more rows than
  the citation promised (#440) and the repair is a longer quotation, not a chosen number.
- The two #448 guards still ask `rev-parse --git-dir` of the planted fixture path, never of
  `$TMPDIR` (#371's one-condition, one-message rule); the rest of what a stranger could break is in
  the `Constraints` blocks of the records beneath, including H-0296's `${FACTORY_TICK_UNIT:-}`
  anchor and the colon-scratch trap (#503).
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or an
  approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.

## Record — issue-448 conflict tick, round 2 for PR #504 (branch `issue-448`, PR #504 open against `main`; this unit ships `H-0376`: `main` moved twice under this pull and both append-magnet logs collided — 70 hunks in `factory/DECISIONS.md`, of which 68 are drifted `path:line` pointers and one is a seam row whose digits alone differed, so the resolution takes trunk's rows and re-aims every pointer at the merged bytes using the gate's own coordinates (141 moved, 140 digits-only, one anchor lengthened under #440), and the last hunk is the log's tail junction where this branch's `H-0367` and trunk's `H-0370` are both kept seated ascending; `NEXT.md`'s single hunk is two heads, so both bodies are byte-kept, this head is re-titled for the repair round, trunk's `issue-468` head is demoted to a `## Record` stamped with its merge (#509 at `78502fd`, 2026-11-21T21:44:56Z), and the shared `issue-442` Record line is kept once in trunk's wording; the base sync took `origin/main` `78502fd` (the #505, #507 and #509 merges); checkpoint `c997102` before the sync and the adjudication; gate verdicts ride the pull body with their read-backs, never here (#1180); merging belongs to the supervisor and this unit merged nothing)

## What this unit shipped, and what it did not

- Shipped: the sync of `issue-448` onto `origin/main` `78502fd` and the adjudication above. No
  shipped line of `factory/selftest.sh`, `scripts/check-scripts.py` or `scripts/check-close-verb.py`
  was touched by this unit: trunk's 347 added selftest rows and this branch's 27 are both in the
  merge, and the branch's own `H-0367` guards are the ones the round beneath delivered.
- Shipped: `factory/DECISIONS/H-0376.md`, the entry for the class call (drifted pointer vs real
  intent), with the one prose repair that was not digits-only recorded in it.
- Did not ship: the standing-precondition question the round beneath names is still open, and
  #503's colon-in-scratch poison is still only named.

## What is still owed (all of it, none silent)

1. The supervisor merges; this unit merged nothing and asked nobody. If the PR sits
   `REVIEW_REQUIRED` with an empty `/requested_reviewers`, the ask is the supervisor's
   `factory_request_review` poke (`AGENTS.md`), not this unit's.
2. Everything the `Record — issue-448 unit` block beneath still owes, unrenumbered: the two #482
   and #489 environment leaks, the remote-only leg for the allocator's liveness arm, the
   suite-wide `$TMPDIR` precondition, and #503.
3. Everything trunk's `Record — issue-468` block beneath owes, unrenumbered (#506's union-walk
   cost, the two deferred honesty clauses). That pull merged as #509, so those rows are trunk's
   now and this branch inherits them by content, not by re-derivation.

## Constraints a stranger would break here

- A `path:line` conflict inside a decision log is a drifted pointer, never a content dispute: fold
  the digits out of both sides and 68 of 70 hunks come out identical. Take either side, then ask
  the gate where each pointer really lives (`python3 scripts/check-decisions.py --citations` prints
  "it is at line N now" per row) and move only digits. Hand-carrying either side's digits keeps the
  defect the merge was supposed to settle.
- The one real conflict a log merge can hide is an anchor whose text died with one side's edit:
  when the gate answers with two line numbers for one citation, the anchor answers more rows than
  the citation promised (#440) and the repair is a longer quotation, not a chosen number.
- `NEXT.md` merges are head-vs-head, not content-vs-content: keep both records, keep exactly one
  `#` head (a second one is a second status board the supervisor cannot tell apart from the live
  one), demote a merged unit's head to `## Record` stamped with its merge, and say out loud which
  duplicated line you dropped.
- The two #448 guards still ask `rev-parse --git-dir` of the planted fixture path, never of
  `$TMPDIR` (#371's one-condition, one-message rule); the rest of what a stranger could break is in
  the `Constraints` block of the `Record — issue-448 unit` beneath, including H-0296's
  `${FACTORY_TICK_UNIT:-}` anchor and the colon-scratch trap (#503).
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or an
  approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.
## Record — issue-448 unit (branch `issue-448`, PR open against `main`; this unit ships `H-0367`: the two remaining `$TMPDIR`-inside-a-repository FAIL rows #371's audit left — the stale-entry `proc_leg` and the blind-activation `gen_leg` in `factory/selftest.sh` — take the one-condition SKIP #371 already worded, asked of the planted fixture path itself (`git -C "$r342_work" rev-parse --git-dir`) rather than of `$TMPDIR`, because the condition the legs verify is the fixture's unreadability and discovery can escape to an enclosing repository even from a `$TMPDIR` no work tree contains (measured in this tick's six-run matrix: a plain repo with no origin leaves the activation leg green for the wrong reason — SKIPping on discovery, not on `$TMPDIR`, is what catches that topology too); 137 anchored decision-log pointers the +22 insertions slid were re-aimed in the effect commit and the shipped gate reads green at this head; the bare-pointer census this diff owes and the gate cannot read: 32 bare `factory/selftest.sh` pointers in the shipped log, zero naming a row this diff moved, 29 revision-pinned rows untouched; the disclosure comment on #448 names the sibling issue #503 — a colon inside a scratch directory name silently splits every `PATH` prepend that carries it, so stubs go unfound and a suite run under such a `TMPDIR` prints ~29 FAILs and a crash for the shape of the scratch, and THIS tick's dispatch scratch path is exactly such a colon path, so no suite run touched it; gate verdicts ride the pull body with their read-backs, never here (#1180); merging belongs to the supervisor)

## What this branch shipped, and what it did not

- Shipped: the two guards at their claim sites (checkpoint `face774`, effect `b781445` carries
  `Decision: H-0367`), the 137 anchored re-aims in the same commit, the `H-0367` entry in
  `factory/DECISIONS.md`, and the disclosure comment on #448 naming #503.
- Did not ship: no standing precondition for the whole suite — whether a repository-resident
  `$TMPDIR` should sanitize or SKIP every leg is the same decision #371 declined and #448's repair
  left open, and it is still owed; #503's colon-scratch poison is named, not repaired, here.

## What is still owed (all of it, none silent)

1. The supervisor merges; this unit merged nothing and asked nobody. If the PR sits
   `REVIEW_REQUIRED` with an empty `/requested_reviewers`, the ask is the supervisor's
   `factory_request_review` poke (`AGENTS.md`), not this unit's.
2. Carried from the `issue-442` head, unrenumbered: the #482 fence leg and the #489 service-unit
   leak — run the suite as CI runs it, `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH
   make verify`, plus `env -u OPENCODE_CONFIG_CONTENT -u FACTORY_SERVICE_UNIT` inside a live tick.
3. Carried from the `issue-442` head: a remote-only leg for the `refs/remotes/origin/<branch>`
   liveness arm of the allocator's claim read.
4. The open standing-precondition question named above, with this tick's matrix as its evidence: a
   per-leg guard makes the two #448 verdicts honest, but the suite's remaining host-shape
   dependence (green-for-the-wrong-reason legs included) is what a suite-wide precondition would
   be for.

## Constraints a stranger would break here

- The two new guards ask `rev-parse --git-dir` of the planted fixture path, never of `$TMPDIR` —
  that IS the leg's own premise (#371's one-condition, one-message rule). Move the fixture, ask
  the guard from its new path; "tidying" the question back to `$TMPDIR` un-detects the
  enclosing-repository escape this whole ticket is about.
- Carried from the `issue-442` head: the journalled `DECISION-ID` line's positional fields are read
  by two consumers — never insert a field before `unit=`/`branch=`; claim expiry keys on branch
  existence by design (the #365 leak plugged); H-0296's citation anchor is the string
  `${FACTORY_TICK_UNIT:-}` kept unique by `factory/next-decision-id.sh` spelling its own comparison
  `${FACTORY_TICK_UNIT-}` — one character of tidying re-orphans it and the ratchet refuses.
- Running the suite with `TMPDIR` under a scratch path containing a colon fails ~29 legs and
  crashes for the shape of the scratch, not the diff — the colon splits every `PATH` prepend
  below it, so stub binaries are never found (#503). Use a colon-free scratch before blaming a
  branch, and re-derive #448's rows inside a planted repository (the shipped `r342_fixture`
  works), not the bare `.sttmp` recipe, which cannot reach a SKIP by the guard's own condition.
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or an
  approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.

## Record — issue-469 (branch `issue-469`, PR #510 MERGED into `main` at `a6e09ee` 2026-11-21T23:07:22Z read back from the API at this re-heading; this unit ships `H-0374`: the CI pinned fetches that died on 19 Nov get resilience that can reach the measured failure — `Install the pinned tz database` lost three attempts at `curl: (56)` while the sibling hostname served both pinned digests byte-identical, and `--retry 3` spent nothing on a mid-body reset (fixture rows on curl 8.18.0: the old flags took the RST at rc 56 on ONE connection, `--retry-all-errors` spent FOUR, and a 503 control retried four ways under the old flags — the machinery was live and the error class is the whole story); all three pinned fetch lines now retry transport errors with bounded spaced retries, both tz halves fetch through a two-host list behind the unchanged sha256 gate — the only decider, so a fallback can only re-deliver the pinned bytes — served-by lines name the host that delivered, and a new seven-leg selftest measures what #469 could not establish: the runner's own curl build spending six connections on a mid-body-RST fixture with the shipped function verbatim, printing that count every `make verify`, with a fixture control pinned FIRST at the old-flags polarity (ONE connection) so the probe cannot pass on a broken fixture, and a both-dead arm that refuses loudly leaving no output stump; host order puts the measured-healthy host first; the anchored `H-0267` pointer into `ci.yml` held its line through the edit; checkpoint `b4f9792`; gate verdicts ride the pull body with their read-backs, never here (#1180); merging belongs to the supervisor; that head demoted in place by this sync; every body byte-kept)

## Record — issue-468 (branch `issue-468`, PR #509 MERGED into `main` at `78502fd` 2026-11-21T21:44:56Z; that head demoted in place by this sync; every body byte-kept)

## Record — issue-456 (branch `issue-456`, PR #505 open; this unit is the answer-round for the review of #505: the review (hermesvicowa, `CHANGES_REQUESTED`, submitted 2026-11-21T16:48:26Z against head `2aa3517`) named two red legs — SC2209 at `factory/next-decision-id.sh:349` and `check-next-md` refusing the skill line that named a placeholder entry file no layout ships — and both were already repaired by `3dc8148`, which was pushed at 16:38:33Z, nine minutes before the review was filed against the older head; this unit re-measured both at `3dc8148` with controls (bare `shellcheck` 0.10.0 reproduces SC2209 twice, rc 1, on `git show 2aa3517:factory/next-decision-id.sh` and answers rc 0 on the head copy; `bash scripts/check-next-md.sh` OK, the old line's path absent from the tree and the new line's `factory/DECISIONS/H-0369.md` present), re-ran the full gate on the pushed bytes, and re-headed this hand-off, which the two code ticks had left at issue-442; merging belongs to the supervisor)

## What this branch shipped, and what it did not

- Shipped (`2aa3517`, entry `factory/DECISIONS/H-0369.md`, trailer `Decision: H-0369`, checkpoint
  `1595e33`): the allocator answers both faces of a log (monolith name `DECISIONS.md`, filed name
  `DECISIONS`), counts filed entries into the union walk across working tree/`HEAD`/`refs/heads`/
  `refs/remotes`, names the destination file on stderr (`EMITS …`) while stdout stays the id alone,
  and carries the first filed entry; plus the env-scrub addition to the suite (#482-class leak of
  `OPENCODE_CONFIG_CONTENT`/`FACTORY_MCP_FENCE`).
- Shipped (`3dc8148`, review answer): `other_kind="file"`/`"dir"` quoted (SC2209, the gate's own
  bare-`shellcheck` invocation at the pinned 0.10.0), and the skill's `EMITS` example now names
  `factory/DECISIONS/H-0369.md`, a path the branch actually writes, instead of the placeholder
  number the tool-path guard (#337) refuses.
- Did not ship: nothing was changed for the review's three NON-BLOCKING observations (kept out to
  hold the diff minimal while `CHANGES_REQUESTED` stands) — their disposition is owed below.

## What is still owed (all of it, none silent)

1. The supervisor re-asks the reviewer (the `CHANGES_REQUESTED` row stands against `2aa3517`; the
   verdict does not clear itself on `3dc8148` without a re-review) and merges; this unit asked
   nobody and merged nothing.
2. Filed as issue #506 by this unit, per the review's own recommendation ("Worth an issue before
   phase 3, not inside this PR"): the union walk's per-ref cost — the filed arm adds one `git show`
   per entry per ref per answer once phase 3 (#488) files entries on every ref (measured by the
   review at `2aa3517`: 6.7 s → 10.6 s for the union read on a 1372-ref accumulated clone with the
   filed dir on no ref but the branch); `ls-tree --name-only` basenames could source the taken-set
   with no per-file content read, since the gate already refuses filename≠heading.
3. Deferred with evidence, not silently: the `PENDING` note names the caller's `$rel` path, not the
   file that raised the ceiling (the `printf` that raises the `PENDING` note interpolates `$rel`; the
   review's one-word fix: `is in this tree`); and the decoy header sentence ("a directory called
   `DECISIONS.md` is still refused") now reads stronger than the code, which refuses the decoy
   *face* yet accepts the call via the filed twin — safe behavior, a clause of honesty is owed in
   that comment. Both belong in a phase-3 touch or the cost issue's fix, not in a green-awaiting-review PR.
4. Carried from the issue-442 record beneath: its three owed items and constraints stay true there;
   nothing in this round retired them.

## Constraints a stranger would break here

- stdout of `factory/next-decision-id.sh` must stay the id alone (every existing `$(…)` capture and
  #442's taken-set read parse it byte-exactly); stderr is the only channel for `EMITS`/`REFRESH`/
  `PENDING` — the stdout-diff leg against `9ba61e7` pins this and degrades to ok-with-note on
  shallow checkouts.
- The never-hand-file-an-existing-entry rule stands (phase 2 reverses the append *default*, not the
  hazard): entries keep being produced by the allocator; a hand-filed entry file is a file
  nothing will ever produce again.
- The selftest env-scrub block now names `FACTORY_MCP_FENCE` and `OPENCODE_CONFIG_CONTENT` because
  the `and nothing else does` leg mutates the dispatch line that carries the fence; a "tidy" unset
  block that drops them reddens the leg on a live tick tree (#482's class).

## Record — issue-442 (branch `issue-442`, PR open against `main` at `ffb0c43`; this unit ships `H-0364`: the allocator now consumes its own journal — the LAST `DECISION-ID` line naming a candidate is its claim, another unit's claim on a branch that still exists as a ref here is taken and stepped over with the trail journalled and `STEPPED-OVER` on stderr, a claim whose branch exists nowhere is expired (`issue-365`), and the owning unit keeps its own id (#320's re-read); a read, not the claim-time lease `H-0195` refused, failure-blind and offline, the merge still the arbiter; five polarities ship in `factory/selftest.d/442-journal-claims.sh` with the control that reproduces the #442 collision verbatim when the read is neutered; the base sync took `origin/main` `ffb0c43` (the #498 merge, both entries of its tail collision byte-kept, seated ascending `H-0363` before `H-0364`, verified byte-identical against both parents by script); this hand-off demotes the `issue-433` round-4 head in place to the Record beneath with every body byte-kept; checkpoint `3bcc9b7` for the repair, `4100c41` for the sync; gate verdicts ride the pull body with their read-backs, never here (#1180); merging belongs to the supervisor)

## What this branch shipped, and what it did not

- Shipped: the claim read in `factory/next-decision-id.sh` (checkpoint `3bcc9b7`, effect commit
  carries `Decision: H-0364`), the five-polarity leg file, the `docs/factory.md` section appended at
  EOF so no existing citation pointer moved, the five pointer re-aims the edit slid (H-0216, H-0219,
  H-0261, H-0296, H-0317) with quotes re-verified, and the base-sync merge seating the log tail
  ascending with both entries byte-kept.
- Did not ship: no expiry clock, no claim-time lease (#345's refusal stands), no cross-machine
  visibility — a claim journalled on another machine is invisible to a local read exactly as a
  heading there is; and the `refs/remotes/origin/<branch>`-only arm of claim liveness is exercised
  by the code but not by a dedicated leg of its own (the legs pin the local-ref arm and the
  never-a-ref arm; closing that gap is cheap and owed).

## What is still owed (all of it, none silent)

1. The supervisor merges; this unit merged nothing and asked nobody. If the PR sits
   `REVIEW_REQUIRED` with an empty `/requested_reviewers`, the ask is the supervisor's
   `factory_request_review` poke (`AGENTS.md`), not this unit's.
2. Carried, unrenumbered: the #482 fence leg and the #489 service-unit leak — run the suite as CI
   runs it, `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make verify`, plus
   `env -u OPENCODE_CONFIG_CONTENT -u FACTORY_SERVICE_UNIT` inside a live tick; on a bare dispatch
   shell this branch shows the same single fence failure trunk shows (trunk `ffb0c43`: 979/1; this
   branch: 985/1, the +6 all green), which is #482, not this diff.
3. A remote-only leg for the `refs/remotes/origin/<branch>` liveness arm named above.

## Constraints a stranger would break here

- The journalled `DECISION-ID` line's fixed positional fields are read by two consumers — the
  earned-set comparison in `factory/common.sh` and this file's own claim scan. The step-over trail
  is a TRAILING note after them (`... (allocator, rel, READ) (H-0250 taken: ...)`); never insert a
  field before `unit=`/`branch=`, or both readers go blind at once.
- Claim expiry keys on branch existence: deleting a merged/ded branch retires that unit's claim by
  design, and the next read may answer the id again — that is the #365 leak plugged, not a
  regression. The merge and `check-decisions` remain the last line.
- H-0296's citation anchor is the string `${FACTORY_TICK_UNIT:-}` kept unique by this unit spelling
  its own comparison `${FACTORY_TICK_UNIT-}` (identical semantics for empty-or-unset). "Tidying"
  that one character back to `:-` re-orphans the citation and the ratchet refuses the branch.
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or an
  approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.

## Record — issue-433 conflict tick, round 4 for PR #498 (branch `issue-433`, PR #498 open; this round ships `H-0363`: the fourth base sync of this branch — `origin/main` moved `cd2fa79` → `7ebb4fa` (the #494 merge `2293e98`, MERGED 2026-11-21T10:24:22Z, carrying `H-0348`/`H-0350`/`H-0353`, then the #500 merge, MERGED 2026-11-21T11:20:20Z, carrying `H-0357`/`H-0359`/`H-0361`/`H-0362`; both stamps read back from the API at this re-heading) and both append tails collided again: sixty-four hunks in `factory/DECISIONS.md`, adjudicated as an id-keyed three-way union (seven ids trunk-only, four this branch's alone, 234 byte-identical, 61 edited by one side alone), and inside the forty-eight entries BOTH sides had edited, thirty-nine spans diverged — every one a single carried sentence holding each parent's digits, or its own revision-pin, or a wider anchor cut, into the same pointer; adjudication: trunk's spelling is kept wherever one side carried a revision-pin (a pinned coordinate survives any future re-heading; `git show <pin>:<file>` opens it), and every bare coordinate lands where the shipped gate's own refusal prints it, onto the `issue-417` `Carried forward` copy of the hazard pair per the convention `H-0334`, `H-0338` and `H-0346` each settled; this hand-off conflicted in its head region because both parents re-headed the same base head, so this head sits on top, this branch's round-3 head and trunk's `issue-440` round-2 head are demoted in place to `## Record —` with every body byte-kept, and trunk's `issue-428` Record stack is carried beneath it whole; the zero-absence census ran against both parents with the pin-stripped + digits-folded shape over every merged path, every absent row accounted as an intentional replacement, no unmarked swallow; checkpoint `e23b11a`; gate numbers ride the pull body, never here (#1180); merging belongs to the supervisor)

## What this round shipped, and what it did not

- Shipped: the merge commit of `origin/main` `7ebb4fa` into `issue-433` carrying `Decision: H-0363`,
  the id-keyed union above with its pinned-side rule and gate-driven re-aims, the `H-0363` entry in
  `factory/DECISIONS.md`, and this hand-off.
- Not shipped: no code changed — the branch's shipped artefacts stay `H-0351`, `H-0355` and `H-0360`;
  nothing here re-aims a coordinate by arithmetic, and no pointer was written before its row was opened.

## What is still owed (all of it, none silent)

1. The supervisor merges; this pass merged nothing and asked nobody. The push spends whatever review
   state the API reads at the new head (the #269 class); nobody has been re-asked — the ask is the
   supervisor's `factory_request_review` poke, not this unit's.
2. Carried from the Records below, unrenumbered: the merge-census leg (#428), the two-word hazard-row
   repair (#428, #440), the #482 fence leg — run the suite as CI runs it, `env -u FACTORY_LOG
   -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make verify`, plus `env -u OPENCODE_CONFIG_CONTENT
   -u FACTORY_SERVICE_UNIT` inside a live tick — and the `SUPERVISOR-FF`/`SUPERVISOR-RERUN` watch:
   read those journal lines before blaming the branch.

## Constraints a stranger would break here

- After ANY merge whose trunk side rewrote this file, run the both-parents multiset census (every line
  of `git show <parent>:NEXT.md`, pins stripped and digits folded, contained in the merged file) before
  trusting the auto-merged middle — `H-0330` carries the measurement, and this round ran it again.
- The hazard fragments answer on several Record rows; a bare re-aim lands on the `issue-417`
  `Carried forward` copy (the newest Record copy by position), and `python3 scripts/check-decisions.py
  --citations NEXT.md` is re-run after every head edit, never inherited — head edits come BEFORE
  pointer re-aims. The trunk-pinned copies (`088826b3:NEXT.md:720/723`, `cd2fa79:NEXT.md:763/766`)
  need no re-aim: open them with `git show <pin>:NEXT.md`.
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or an
  approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.

## Record — issue-433 conflict tick, round 3 for PR #498 (branch `issue-433`, PR #498 open; this round ships `H-0360`: the third base sync of this branch — `origin/main` moved `fbb7764` → `cd2fa79` (the #499 merge, carrying `H-0352` and `H-0356`) and both append tails collided again: twenty-one hunks in `factory/DECISIONS.md`, twenty of them ONE carried sentence holding each parent's digits into its own copy of `NEXT.md` (proved digit-fold identical by an abort-on-first-difference assertion before either side was trusted, taken as alignment only), the twenty-first the tail junction seated ascending by id — `H-0351`, `H-0352`, `H-0354`, `H-0355`, `H-0356` — each entry byte-kept from its parent with a blank-clean splice; this hand-off conflicted in two regions because both parents re-headed the same base head, so this head sits on top, the issue-433 round head is demoted in place to a `## Record —` with its body byte-kept, trunk's issue-437 head is demoted to a Record beneath it with its three sections carried whole and `PR #499 MERGED 2026-11-21T08:13:24Z` read back from the API at this re-heading, trunk's API-annotated `issue-430` Record line is kept over ours, and the `issue-427` Record line carries both parents' notes — trunk's `PR #493 MERGED` API stamp and this branch's demotion sentence; `docs/factory.md` auto-merged, taking trunk's line-count-neutral `H-0352` paragraphs untouched; every drifted coordinate in the log lands where the shipped gate's own refusal prints it, onto the `issue-417` `Carried forward` copy of the hazard pair per the convention `H-0334`, `H-0338` and `H-0346` each settled; the gate ran to silence after this file was final — no head edit follows the re-aim; the zero-absence census is recorded in `H-0360`; checkpoint `fde7a6a`; merging belongs to the supervisor)

## What this round shipped, and what it did not

- Shipped: the merge commit of `origin/main` into `issue-433` carrying `Decision: H-0360`, the two-file
  adjudication above with its fold assertion and ascending tail seat, the `H-0360` entry in
  `factory/DECISIONS.md`, and this hand-off.
- Not shipped: no code changed — the branch's shipped artifacts stay `H-0351` and `H-0355`; nothing here
  re-aims a coordinate by arithmetic, every digit the gate refused was re-derived from its printed home
  after this file was final.

## What is still owed (all of it, none silent)

1. The supervisor merges; this pass merged nothing and asked nobody. The push spends the `APPROVED` read
   taken at head `91137ef` (the #269 class); nobody has been re-asked yet — the ask is the supervisor's
   `factory_request_review` poke, not this unit's.
2. Carried from the Records below, unrenumbered: the merge-census leg (#428), the two-word hazard-row
   repair (#428, #440), the #482 fence leg — run the suite as CI runs it, `env -u FACTORY_LOG
   -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make verify`, plus `env -u OPENCODE_CONFIG_CONTENT
   -u FACTORY_SERVICE_UNIT` inside a live tick — and the `SUPERVISOR-FF`/`SUPERVISOR-RERUN` watch:
   read those journal lines before blaming the branch.

## Constraints a stranger would break here

- After ANY merge whose trunk side rewrote this file, run the both-parents multiset census (every line of
  `git show <parent>:NEXT.md`, digits folded, contained in the merged file) before trusting the
  auto-merged middle — `H-0330` carries the measurement, and this round ran it again.
- The hazard fragments answer on several Record rows; a re-aim lands on the `issue-417` `Carried forward`
  copy (the newest Record copy by position), and `python3 scripts/check-decisions.py --citations NEXT.md`
  is re-run after every head edit, never inherited — head edits come BEFORE pointer re-aims.
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or an
  approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.

## Record — issue-433 unit (branch `issue-433`, PR #498 open; this round ships `H-0351`: a decision row glues a commit token to a `(PR #N` reference and NO gate opened either number — #433 measured `(PR #427, `H-0235`)` riding two review rounds beside `cf00b44`, every token false and each one command from checked. `scripts/check-decisions.py` now resolves every glued pair through git and holds it to the commit's own message (fail closed when git cannot be asked; messages read through `git cat-file --batch` because the ref-blind leg #253 refused the first draft's `git log`), the one genuinely mis-glued shipped row (11533) is repaired to what its own commit message supports, and the 101 anchored pointers this script's growth drifted (63, then 38 after the rewrite) were re-aimed by the shipped rule's own refusals; the ticket's exact specimen and a dead token both refuse rc 1 against the real log with byte-identical control restores, and `make verify` was rc 0 — 958 passed, 0 failed — under the recorded `env -u OPENCODE_CONFIG_CONTENT` scrub for `fc88aa8`'s bytes. `H-0355`, this round's second decision, is #498's review turned into the repair it asked for: that reader framed a byte-counted `cat-file --batch` reply in characters, so a TRUE pair on a merge whose body is not pure ASCII — `1c385b7`, 1364 bytes against 1360 characters, the merge quoting a pull-request title carrying an en dash and an em dash — was refused as "the provenance rule did not run" about a row whose claim holds; the framing is in bytes now, the header's own name is held against the token that was asked for, and seven rows of `--self-test` pin the arms and the two controls on synthetic batch bytes with no git, repository or locale consulted. `make verify` re-run on those bytes in a scrubbed tick env (`env -u FACTORY_SERVICE_UNIT -u OPENCODE_CONFIG_CONTENT`) is rc 0 -- its own last line `verify: OK`, `factory/selftest.sh: 972 passed, 0 failed`, `merge-decisions --self-test: 45 ok, 0 failed`, all twelve packages `ok` under `-race`; `python3 scripts/check-decisions.py` rc 0 with 1360 citations, 583 owing an anchor and 0 refused outright, and `--self-test` 93 must-fail / 63 must-pass; `check-mutations` applies all 853 mutants. Head `d977ccc`, pushed and read back as #498's `headRefOid`, with the same read naming `headRefName issue-433` and `state OPEN`; the verdict quote rides the pull-request thread as well as this line, and the re-measure of `mergeable` after a push is the API's to return, not this file's to assert. Merging belongs to the supervisor)

## Record — issue-440 conflict tick, round 2 for PR #500 (branch `issue-440`: the review of #500 demanded the sync as the deliverable — merge of `origin/main` `2293e98` taken with our side for all 20 coordinate hunks, main's `H-0353` (#494) kept at its original seat, our entries seated ascending `H-0352` → `H-0353` → `H-0356` → `H-0357` → `H-0359`, and the cross-branch id collision closed by the yield rule: our unmerged anchor-uniqueness decision moves `H-0353` → `H-0361` (erratum row in the entry; `H-0362` reconciles the `Decision: H-0353` trailer at `5540d10`), main's merged `H-0353` untouched; main's four fresh hazard-pair rows re-pinned at `2293e98`, where rivals are four byte-identical copies and lengthening is impossible; the allocator answered `H-0361` from an expired journal entry and `H-0362` only after a REFRESH, so the two allocations are 3 minutes apart and both journalled; the zero-deletion census on the five merged paths ran against both parents with the pin-stripped + digits-folded shape — every absent row accounted as an intentional replacement, no unmarked swallow; `NEXT.md` is re-headed by convention — this round on top, the `issue-428` round-4 head demoted in place to `## Record —`, every body byte-kept. Gate on these bytes: the numbers ship in the pull body, never here (#1180). Hazard rows: the pair moved with this head +2; the next sync must re-aim by the gate's refusals, not by offset.)

## Record — issue-428 conflict tick, round 4 for PR #494 (branch `issue-428`: the base sync landed a **fifth** time inside one ticket — `origin/main` moved `fbb7764` → `cd2fa79` (the #499 merge, carrying `H-0352` and `H-0356`) while this tick ran, and `git merge` met 22 content hunks of which 20 were one carried sentence holding each parent's digits into its own copy of the hazard pair, plus a head collision whose trunk side is `issue-430`'s re-delivered record head. The adjudication is `H-0359` (checkpoint `1dd1a22`): 46 seam citations re-aimed digits-only onto this tree's rows 771 and 774, every destination opened on the quote before its digit was written; the allocator answered `H-0358` blind twice while `origin/issue-440` carries a live `^## H-0357` heading its union never enumerated, so this unit took `H-0359` — the #442 collision, live. Gate on the shipped bytes: rc 0, 1373 citations read, 583 owing, 0 refused; `make verify` verdicts go in the pull body, never here (#1180). Hazard rows: the pair is at 771/774 on THIS tree — inserting a line above them moves every seam pointer again, and the next sync must re-aim by the gate's refusals, not by offset.)

## Record — issue-428 conflict tick, round 3 for PR #494 (branch `issue-428`: the base sync is in again — `origin/main` moved `eef3210` → `fbb7764` (the #496 merge, carrying `H-0349`) and the same append tails collided a third time: twenty-one hunks in `factory/DECISIONS.md`, twenty of them one carried sentence holding each parent’s digits into its own copy of this file (trunk’s `H-0349` round had re-aimed the hazard pair to 720/723, this branch still said 657/660), the twenty-first the tail junction, now seated ascending `H-0348`, `H-0349`, `H-0350` by id; in `factory/selftest.sh` trunk’s fourteen fresh legs went inline ahead of this branch’s leg-file runner and the tally, and every live pointer into this hand-off and into the runner line was re-aimed at the rows it names on the merged tree, each destination opened before its digit was written; this file is re-headed by convention — this round on top, both parents’ heads demoted in place to `## Record —`, every body byte-kept; the adjudication is `H-0353`, carried by the merge commit itself; the pull Refs #428 and the issue keeps its own terms, never this file; merging belongs to the supervisor)

## Record — issue-428 conflict tick, round 2 for PR #494 (branch `issue-428`: the base sync is in — `origin/main` moved `b195a5e` → `eef3210` (the #493 merge, carrying `H-0347`) and all three append tails conflicted: the `factory/DECISIONS.md` tail junction is a union in ascending id order (`H-0347` under `H-0346`, `H-0348` under `H-0347`), the `factory/selftest.sh` tail keeps trunk's three baseline legs inline and seats this branch's leg-file runner behind them before the tally, and this hand-off kept its shape — this head replaced the round-1 head line-for-line and both parents' heads were demoted in place to `## Record —` with every body byte-kept, so the hazard-pair pointers into this file stayed exactly where they were read; the adjudication is `H-0350`, carried by the merge commit itself (the interrupted tick checkpointed pre-`H-0349`, which `issue-430` has since taken, so this entry yields and says so); the pull Refs #428 and the issue keeps its own terms, never this file; merging belongs to the supervisor)

## Record — issue-428 unit (branch `issue-428`, PR open; the shared append tail on `factory/selftest.sh` — 3 of 8 open branches measured conflicting on it by #428 — got its phase 1 repair: the suite now sources `factory/selftest.d/*.sh` in `LC_ALL=C` order via `run_selftest_d_legs`, the first leg family ships as a file there, and it opens with the falsification leg #428 demanded: two appended monolith tails MUST still conflict; merging belongs to the supervisor)


## Record — issue-437 unit (branch `issue-437`, PR #499 MERGED 2026-11-21T08:13:24Z read back from the API at re-heading; this round ships `H-0352`: #437 measured the escalation park dead because the queue read a label name no repository has, and both premises died on trunk — `factory/common.sh` now defaults to `question`, a label every repository is created with (H-0250), and #313 made `parked` a real class that starts no tick (live pass parks #334, #393, #476; 134 `PARKED` journal lines). What still shipped is the residue the ticket's item 2 named: two paragraphs of `docs/factory.md` that denied, in the present tense, that the queue reads the escalation marker, and a zero-count journal claim the live supervisor has refuted — re-aimed with line-count-neutral edits so none of the 89 decision-log citations aimed at that page moved. Items 1 and the `--status` parked count are retired by measurement, recorded in H-0352 and in the evidence comment on #437; merging belongs to the supervisor)

## What this branch shipped, and what it did not

- Shipped: the two re-aimed paragraphs in `docs/factory.md` (the `PARKED`-zero sentence dated to the
  window it measured; the marker paragraph now states the #313 class, the release verb, and the window
  rule `esc_not_a_park` actually enforces, cited to H-0140), the `H-0352` entry, and the full
  re-measurement as an evidence comment on #437 (read-back byte-identical, id
  `IC_kwDOVGEN4c8AAAABpKA29Q`).
- Not shipped: item 1 (escalation record in the row's detail carried into the prompt) and item 3's
  `--status` parked count — both retired by measurement in H-0352, because a parked issue receives no
  dispatch for a column to inform and the parked count is already a journal read.

## What is still owed (unmeasured, outstanding)

- Nothing by this unit: the pull request's plain-text `Closes #437` fires at merge, and #437's own
  procedure names this shape (a satisfied-by-main close is done by the tick's evidence comment plus
  the merge-verb close). If the merge-time close read-back comes back not-CLOSED, the next hand has
  the procedure in the issue's comments.
- To every open pull: `Closes #N` in the body is a close promise a retitle must not break — #479 is
  parked on an author rename that deleted the body and closed nothing (#374, H-0293).

## Constraints a stranger would break

- `docs/factory.md` is load-bearing by line number: 89 decision-log citations point into it (nearest
  targets 121 and 122), and a merge that adds a line there slides every coordinate below it (#373,
  H-0329). Ask `python3 scripts/check-decisions.py --citations docs/factory.md` before editing, and
  keep the edit line-count-neutral — this round's two paragraphs hold their line counts exactly.
- The same page is also load-bearing by PHRASE: `esc_guide_agrees` greps whole clauses out of it
  (`the order is the mechanism`, `the unit is parked or released`) as the guide half of the #272
  pair, and H-0141's census records that these anchors break on a rewording by design — a
  well-meant paraphrase reddens the leg, as this round's first attempt proved at run time.
- `docs/factory.md` carries a measured collision history: PR #496 (issue-430) merged first, its
  16 lines at the page's line 60 moved every decision-log pointer under the insert, and this
  sync — the second merge of that pair — re-aimed what the collision moved (H-0356 records the
  counts). Any line-count-changing edit of that page pays the same census: ask
  `python3 scripts/check-decisions.py --citations docs/factory.md` before and after.
- The escalation name's home is `factory/common.sh` and a leg of `factory/selftest.sh` refuses a
  second spelling of the decision; `esc_not_a_park` refuses a classifier label line that can name any
  class other than `parked` (H-0140's window rule).


## Record — issue-430 unit (branch `issue-430`, PR #496 MERGED 2026-11-21T04:23:23Z read back from the API at re-heading; this round ships `H-0349`: the one human-facing notice the factory writes blamed a maintainer on a pull request whose `mergeable` and `mergeStateStatus` were both `UNKNOWN` — GitHub's spelling of *not computed yet* — because the classifier fell through a vacuous read into the review arms and one shared `stuck-since` aged a `mergeability_pending` wait into an `awaiting_review` comment; a vacuous read is now its own quiet class ahead of the checks and review arms, the wait clock resets when the class changes, and a notice reads `/requested_reviewers` at the moment it writes and may name only a requester that live read actually finds; merging belongs to the supervisor)

## What this branch shipped, and what it did not

- Shipped: the `mergeability_pending` class in both classifiers (`factory/triage.sh` and the
  `factory.sh` classification, ahead of the checks and review arms, with the row's detail naming the
  READ as absent — `the mergeability read has not returned` — and carrying any `checks fail` it sits
  under); the dispatcher's wait arm carrying it under the existing throttle; the per-wait clock —
  `factory_wait` resets `stuck-since` and `last-notice` when the class changes and passes the new
  stamp down rather than re-reading it, because a dry run no-ops every ledger write and a re-read
  rehearses a notice the real pass would never post; the notice's evidence line — it quotes the
  read and its detail, and the sentence about a person may only name a person the live
  `/requested_reviewers` read names, else it plainly names no one; the guide's table row and the
  measured paragraph in `docs/factory.md`; sixteen assertions — fourteen printed legs plus two
  controls adjudicated as commentary — four of them controls: the fall-through mutant (blind the
  arm and the same fixture answers `awaiting_merge` — the class the live comment was posted under),
  the wait-list mutant (must answer `UNKNOWN-CLASS`), the reset-disabled mutant (must post the live
  bug from the same ledger), and the evidence-read inverter (must produce the wrong sentence at
  both ends).
- Push receipt, each coordinate read back live, never from an exit status (#1180): effect commit
  `783666b`; this hand-off receipt rode as `2844b1f`; the 40-pointer re-aim that the receipt's own
  six lines required rides as `088826b` — at that push `ls-remote` and PR #496 read back that sha,
  `OPEN`, `headRefName issue-430`, base `main`; the pull body was corrected for the leg count and
  read back byte-identical (md5 `4c6d94c00f54…`); `git ls-remote` answers whatever is head now.
- Not shipped: the reset only answers forward — a unit that oscillates between two wait classes can
  never age a notice at all, and a wait that leaves and returns re-ages from zero each trip. That
  is disclosed in `H-0349` (`What lost`), not papered over.
- Gate state on the shipped bytes, as measured: `python3 scripts/check-decisions.py` rc 0 with the
  recorded citation debt unchanged from base (1352 read / 583 owing anchors / 0 refused — base
  `38bbab3`'s worktree answers the same, and this branch's own 280 drifted rows were re-aimed by
  the gate's printed homes plus one difflib mapping for the three whose anchor the tool lists
  truncated at six); `bash factory/selftest.sh` 972 passed, 0 failed; `make check-shell` clean
  (shellcheck 0.10.0, 33 scripts); `check-scripts`, `check-next-md`, `check-doc-labels`,
  `check-fixture-window`, `merge-decisions --self-test`, gofmt, `go vet`, `go build` all green on
  this tree. One earlier selftest run answered 971/1: the red leg is `and nothing else does`, the
  `OPENCODE_CONFIG_CONTENT` environment leg, red identically at the checkpoint commit with this
  branch's changes absent; the post-restart shell lacks that injected variable and the leg goes
  green — both measurements reported, neither smoothed.

## What is still owed (unmeasured, outstanding)

1. Nothing keeps the class list in step across `factory/triage.sh`, the `factory.sh` classifier,
   the dispatcher's wait arms and the `docs/factory.md` table — this round updated all four by
   hand and the H-0349 entry names that as the class's real cost. A leg that derives the set from
   one source and refuses drift against the others is not written.
2. GitHub's own re-computation bound for a vacuous `mergeable` read is assumed ("minutes") and was
   never measured from this harness; if it can sit at `UNKNOWN` for hours, the quiet wait needs a
   ceiling of its own, not the generic escalation clock.
3. `checks_pending` and `awaiting_review` still share nothing but the throttle; whether they should
   share an age the way `mergeability_pending` now refuses to is open — the reset made the sharing
   visible as a defect instead of a convenience.

## Constraints a stranger would break

- The selftest fixture welds `mergeable: "UNKNOWN"` into every planted row; that weld is the bug's
  shape, and a "clean-up" that gives fixtures real verdicts re-blinds the classifier legs. Per-row
  field overrides exist (`claim_rows` payloads) — use them; do not touch the weld.
- A new wait class must land in both classifiers, the dispatcher's wait arm and the guide table in
  one commit; a class the dispatcher does not carry exits the tick as `UNKNOWN-CLASS`, which is the
  refusal leg's exact assertion.
- The notice must never name a human without the live `/requested_reviewers` read naming them —
  the ticket's rule is pinned by asserted words in the legs, not by the comment merely existing.
- The `stuck-since` passed into `factory_maybe_notice` must be the computed value, never a ledger
  re-read: under `--dry-run` every ledger write is a no-op and a re-read quotes the pre-reset stamp.
- This scratch tree's hazard, measured: a PATH-separator colon inside a scratch directory name
  (`plock-unclaimed:430-…`) breaks PATH composition for anything exporting it; quote or rename.


## Record — issue-427 unit (branch `issue-427`, PR #493 MERGED 2026-11-21T00:33:23Z read back from the API at re-heading; this round ships `H-0347`: `DECISION-ID-UNEARNED` had one word for two opposite states, so the first tick after #345's check reached trunk printed five "unearned" lines and seventeen more were queued on eight of nine open branches, for headings committed before the journalling existed (`148a7e8`, 15 Nov 08:18) and therefore unjournalable by construction. `factory_decision_provenance_snapshot` now inventories the tree's decision headings at `TICK-START` and journals `DECISION-ID-BASELINE n=…`; the end-of-tick pass reports `DECISION-ID-UNJOURNALABLE n … ids=…` once per unit per log for headings the branch already carried (own ledger key, so the backlog cannot spend each real claim's single `unearned-reported` report) and keeps `DECISION-ID-UNEARNED … baseline=tick-start|none` for ids the unit itself introduced. Three legs added at the tail of `factory/selftest.sh`; the six pre-#427 legs are deliberately left with no baseline file, because they are what exercises the `baseline=none` arm; merging belongs to the supervisor) — head demoted by the issue-433 unit; the queue below is that round's plus this one's.

## What this branch shipped, and what it did not

- Shipped: the tick-scoped baseline (snapshot at `TICK-START`, `DECISION-ID-BASELINE` journalled), the
  second word `DECISION-ID-UNJOURNALABLE` with its own dedupe key, `baseline=tick-start|none` on the
  UNEARNED line, and the control #427 asked for — a heading committed before the tick began must NOT
  read UNEARNED while a mid-tick by-eye id still does (`inherited is not unearned`,
  `each word dedupes alone`).
- Not shipped: the exemption's other half. `earned` still exempts an id outright, so the one id the
  journal has ever journalled (`H-0219`) is still the one id that cannot be flagged when re-issued,
  and whether the allocator's union contains the calling branch's own log under a dispatch is still
  unanswered. That is the open question filed in #427's blast-radius item 5; it is not closed here.
- Pushed at 2026-11-20T23:09Z as `eb8c61441fe9d9c882e470cf43b111fd64ae20cc`, checkpoint
  `416388e96c9ba7dcd99a0eff9a3f5d18eee0e2b0`, base `401dbdcec0776672f5069ca3ee6eba338225e01e`, PR
  #493 open at that sha (read back: `headRefName=issue-427`, body md5
  `e8cbfb28fdadc0a777315d1252ee36d4` identical on both sides). Nobody was asked for review: the ask is
  the supervisor's poke, not this unit's, and `gh pr edit --add-reviewer` writes nothing here
  (`H-0010`).
- Gate state at push, as measured, not as expected: `bash -n` clean on `factory/common.sh`,
  `factory/factory.sh`, `factory/selftest.sh`; `make verify` did NOT complete before the tick
  ceiling. The other two claims in this bullet were inverted and the review measured it:
  `python3 scripts/check-decisions.py` exits 1 at that head with 250 refused citations (this file
  said 210), and it is THIS branch, not trunk — base `401dbdc` answers rc 0 with 0 refused, re-run
  `git show 401dbdc:factory/selftest.sh | sed -n '20607p'` and the anchor
  `the names git does not hold at 100755` is on 20607 at the base commit, exactly where `H-0168`
  cites it; the branch moved it to 20608. The drift was this branch's own stray blank line at
  `factory/selftest.sh:18409` (+1 below it), plus +5 in `factory/factory.sh`, the new functions in
  `factory/common.sh`, and the +53 this file's re-head moved the Record stack.
- Round 2 (20 Nov review repair) drove the gate green for the right reasons: the selftest insertion
  reshaped (stray blank line removed, the stranded tally comment re-seated under the new legs, the
  diff against trunk is one tail hunk, so every drifted selftest row landed again without a digit
  moved), the remaining drifted rows in `factory/factory.sh`, `factory/common.sh` and the `NEXT.md`
  pointers this file's re-head moved re-aimed by the shipped gate's own refusals, `H-0347`'s
  `Options:`/`Chose:` re-shaped into the pipe form the checker splits (the numbered prose drew its
  own two refusals), and both shellcheck findings cleared — SC2015 to an `if`, and SC2100's real
  anchor was the bare `baseline=tick-start` assignment, whose dash shellcheck reads as subtraction:
  the `if` wrapper does NOT dodge it (0.10.0, re-measured), the quoted value does. The snapshot
  guard is `[ -s "$snap" ]` now, not `-r`: an empty inventory is a failed read and belongs to the
  verdict-less `none`, never to `tick-start`. Verdicts are quoted on the pull request, never here (#1180):
  round-2 head `a918aaa2f45476671b0abbbf256c7c7dee0b65f8` pushed 20 Nov, read back as #493's `headRefOid`.

## What is still owed (unmeasured, outstanding)

- `make verify` completed green 20 Nov in a sanitized tick env (`env -u FACTORY_SERVICE_UNIT
  -u OPENCODE_CONFIG_CONTENT`; both vars are live in a real tick env, measured here — #489's FF of
  the REAL tree and #482's red fence control are why): `958 passed, 0 failed`, `verify: OK` rc 0,
  `inherited is not unearned` and `each word dedupes alone` both ok. Keep the `-u` pair for every
  verify run from a tick tree; the verdict quote rides the round-2 push and its record lands here.
- The citation re-aim DID fold into this branch: the review's control, re-measured by this unit,
  found base `401dbdc` at rc 0 with 0 refused, so the "trunk debt / do not fold" advice this bullet
  used to carry was aimed at debt that does not exist. Whoever reads this next: treat a fresh
  refusal as your own insertion drift — measure the base before calling anything pre-existing.
- #427 stays OPEN after this merge: the re-issued-id half of its title is unshipped (above), so the
  body deliberately says `Refs #427` and carries no closing verb.

## Constraints a stranger would break

- The two new functions in `factory/common.sh` are called from `TICK-START` and the end-of-tick pass
  respectively; the snapshot must stay at `TICK-START` and nowhere later, because the instant the
  session writes a byte the distinction between "inherited" and "taken by this unit" is gone.
- Do not add headings to `factory/selftest.sh` mid-file on a whim: MUTANT-345's anchors in that file
  are quoted by rows in `factory/DECISIONS.md`, and inserting above them re-aims 200+ citations. The
  new legs went to the tail for exactly that reason; `factory_decision_headings` spells its own loop
  quoted (`for rel in 'DECISIONS.md' …`) so the single unquoted anchor stays unique for `dpn_mut`.


## Record — issue-424 unit (branch `issue-424`, PR open; the post-merge close check had never printed a verdict in seven measured merges — REST answers `state` lowercase while the four comparisons and every close-verb fixture carried GraphQL casing — repaired by normalising once inside `factory_close_read` (`H-0343`), re-planting the sixteen fixtures in the bytes the API sends, and dropping the merge belt's always-true `state != MERGED` clause (`H-0344`); merging belongs to the supervisor)

State at push: two decision commits (`885e0e9`/`H-0343` over `bef7403`, `1687f18`/`H-0344` over `25b0066`), then the base sync `e17db5a`→ this merge carrying `H-0346` over checkpoint `1a0bee6` — `NEXT.md` headed with the `issue-423` head demoted to a Record byte-kept, the `DECISIONS.md` tail junction unioned ascending (H-0342 before H-0343/H-0344), 36 hazard-pair pointers re-aimed `+51` to `NEXT.md:587`/`590` by the gate's own refusals. Both code edits stay line-count-neutral.

## What this round shipped

- `factory/factory.sh`: the close-read filter upcases `.state` at its boundary, and the MERGE
  arm's dead belt `[ "$state_now" != "MERGED" ]` is gone — `state_now` now prints as the
  diagnostic it always was on the `MERGE-FAILED` line.
- `factory/selftest.sh`: sixteen close-verb plants carry REST's lowercase `open`/`closed`; the
  CLOSE-OPEN leg and the re-probe SETTLED leg each refuse a `CLOSE-CHECK-UNKNOWN` on their own
  run. The `issues_95`/`issues_96` `MERGED` plants and the `cdec` family's uppercase plants stay
  uppercase on purpose: they pin the neither-answer anomaly and `gh issue view`, whose GraphQL
  projection really does answer `CLOSED`.
- `factory/DECISIONS.md`: `H-0343` and `H-0344`, each committed with what it decides.

## What is still owed (all of it, none silent)

1. The supervisor merges; this unit merged nothing and asked nobody, and the base-sync push
   spends the `APPROVED` read at `45011c5` (the #269 class; nobody re-asked yet, the poke owns
   the ask). Its live check: the first merge carrying this head should grade by verdict.
2. A working second signal for the MERGE arm — retry `.merged` once before refusing — is NOT
   delivered and is NOT claimed by `H-0344`; the exposure its `What lost` row discloses (a read
   glitch skipping the ledger clear and worktree removal, the `#221` shape) predates this
   branch unchanged.
3. Carried, verbatim where still live, from the Records below: the merge-census leg (#428), the
   two-word hazard-row repair (#428, #440), the #482 fence leg — run the suite as CI runs it,
   `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make verify`, plus
   `env -u OPENCODE_CONFIG_CONTENT` inside a live tick — and the SUPERVISOR-FF/SUPERVISOR-RERUN
   watch: read those journal lines before blaming the branch. #489 names a second live poison,
   `FACTORY_SERVICE_UNIT`, inherited into the selftest's supervisor fixtures — inside a tick
   strip it too, as this unit did: `env -u OPENCODE_CONFIG_CONTENT -u FACTORY_SERVICE_UNIT
   make verify`.

## Constraints a stranger would break here

- Both files this unit repaired are line-count-neutral ON PURPOSE: the decision logs anchor
  over a hundred pointers into `factory/factory.sh` and into `factory/selftest.sh`; a one-line
  repair that adds a line moves all of them. Add lines only with a pointer re-aim in the same
  commit.
- Do not "fix" the uppercase plants in the `cdec` family or the `MERGED` plants at
  `issues_95`/`issues_96`: those reads really are uppercase (gh's GraphQL projection) or are
  the anomaly under test. REST casing belongs to the `cv_` close-verb family only.
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or
  an approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.

## Record — issue-423 unit (branch `issue-423`, PR open at writing time; this round ships `H-0342`: the idle check that asked the wrong systemd manager answered `inactive`/`dead` at exit 0 while the pass stood `activating` — the 16 Nov 11:14:56Z receipt is on the ticket — and the new `factory_unit_idle` reads `LoadState` first, refuses `not-found`/`masked`/`failed` as unknown rather than idle, and grants `idle` only to three agreeing signals: the user manager's keyed properties, a non-blocking `flock` on the run lock, and the process table, with the journal deliberately NOT a signal because #380 measured its two silences; eleven legs pin it with stubbed reply shapes written in reverse ask order and live-process controls both ways; the stop-the-timer recipe bullet now names the guard, rewritten line-count-neutrally because the file carries cited rows 489/492, the range 949-977 and pointers at 928 and 958; base sync `077efaf` (the #487 merge, carrying `H-0340`/`H-0341`); merging belongs to the supervisor)

State at push: trunk `077efaf`; the branch carries checkpoint `444ab00`, the base-sync merge
`460e526`, and the `H-0342` effect commit — the open pull request is the only artifact this round
updates.

## What this round shipped

- `factory_unit_idle` at the foot of `factory/common.sh`: three signals, one verdict line,
  verdict first; `idle` only when all agree, `unknown` (rc 2, never 0) when any question could
  not be asked; the missing-run-directory arm proved not to mkdir what it only reads.
- `docs/factory.md`: the closing section carries the three-command receipt block and what each
  exit code permits; the stop-the-timer recipe bullet names the guard instead of the bare
  property command that lied.
- `factory/selftest.sh`: eleven legs — the measured not-found shape refused, its control (the
  same `inactive`/`dead` pair under `LoadState=loaded` reads idle, so only the property a wrong
  manager cannot produce separates the legs), a held lock outranking a dead-looking unit, a real
  shebang-spawned pass process seen in procfs while the lock still reads free, its negative
  control (a live `tail -f` of the same path is a reader), and three unknown arms.
- `factory/DECISIONS.md`: `H-0342` (allocator answered `H-0342` on a `REFRESH 0s union` read
  re-taken after `git fetch`), checkpoint `444ab00`.
- `NEXT.md`: this head on top; the `issue-421` head demoted in place to a `## Record —` below;
  the 36 in-log pointers at the hazard pair re-aimed uniformly by this head's width, digits
  only, destinations opened on the shipped bytes.

## What this push spent

- `bash -n` and `shellcheck -S warning` clean on both scripts; the new family 11 of 11 in a
  standalone harness run; the final-tree `make verify` verdict is quoted in the pull body,
  never here, with the env-stripping named there (#425, #482).
- Citation census: 1340 read, 583 owing an anchor — unmoved, this round adds no bare pointers —
  and 0 refused; the docs edit stays line-count-neutral around every cited row.

## What is still owed (all of it, none silent)

1. The supervisor merges; this pass merged nothing and no artifact but the pull body carries a
   finishing verb. The reviewer ask is the supervisor's poke (AGENTS.md, `H-0010`).
2. `H-0342`'s follow-up: no run of the guard against a second real manager — this session's
   harness refused every `systemctl` invocation (`Permission denied: shell`), so the ticket's
   16 Nov receipts are the live evidence and the suite's stubs the repeatable one; say which is
   quoted whenever the shape is claimed.
3. Carried from the `issue-421` Record below, unrenumbered there: the `RuntimeMaxSec` rename of
   `H-0329`'s title, question and the #418 journal fixture; and from further Records: the
   merge-census leg (#428), the two-word hazard-row repair (#428, #440), the #482 fence leg, and
   the `SUPERVISOR-FF`/`SUPERVISOR-RERUN` watch — all still open.
4. Measured this round, on this runner only, by the control itself doing its job: a suite started
   from inside an agent session that exports `OPENCODE_CONFIG_CONTENT` leaks that value by plain
   inheritance into every fixture session, and `and nothing else does` fails for the environment,
   not the code (the leg's own comment names this exact inheritance). The suite should scrub
   `OPENCODE_*` from the runner's environment at startup, the way #482's fence leg scrubs knobs;
   the run of record for this branch was re-taken under `env -u OPENCODE_CONFIG_CONTENT`.
## Record — issue-421 unit (branch `issue-421`, PR open; the tree-claim repair `H-0340` was already on the branch when this tick was dispatched — `triage` reported it `branch_without_pr` — and this round ships `H-0341` on top: the claim family's prose leaned on an installer `RuntimeMaxSec` that `git grep -c` against `origin/main` and the live unit both refuse to contain, so the sweep's licence is now stated against the two ceilings that exist — the tick's `timeout(1)` kill window and the rendered `TimeoutStartSec` belt — and pinned by four new legs with one-knob controls; five prose sites corrected line-count-neutrally, 65 in-log pointers re-read unmoved; the CI red on `f3029bd` that this file's own head authored — 45 net lines sliding the hazard pair's first home 444/447 to 489/492, 36 in-log pointers refused — is repaired in the same push, all 36 re-aimed onto the `issue-417` Record copy by the `+45` the gate's own homes corroborate; merging belongs to the supervisor)

State at push: the round is 15 commits ahead of `origin/main` — checkpoint `ac4ea69`, decision commit `dd089b8` carrying `Decision: H-0341`, the 36-pointer repair and this head's review-state row on top of both — and PR #487 is the only artifact this round updates.

## What this round shipped

- `factory/DECISIONS.md`: `H-0341` (allocator answered `H-0341` on a `REFRESH 0s union` read,
  re-read after fetch), `H-0340`'s `Effect:`/`Why:` corrected in place to say the installer claim
  was wrong rather than silently editing the story; the correction names `H-0341` beside it.
- `factory/selftest.sh`: four legs and three helpers at `:28558`/`:28581`/`:28607` — slack >=
  kill-grace, belt >= tick timeout + grace, each with the control that plants exactly one knob
  (slack 30; a belt rendered `FACTORY_BUDGET - 3600`) and must turn exactly its leg; both
  predicates read shipped defaults or the rendered unit, never a literal.
- Prose: the `H-0340` lines above, two comments in the suite's claim family, the `common.sh`
  death-path header, the `docs/factory.md` claim-section sentence — all line-count-neutral.
- `NEXT.md`: this head, rewritten line-count-neutrally — a head that re-aims pointers must not
  itself move them (f3029bd moved the pair its own round had aimed); the `issue-418` head sits
  demoted below, its `#484 APPROVED/CLEAN` read stale (#484 is the merge now at `a3681a6`).

## What this push spent

- The `H-0341` verdict at `dd089b8` under the #482 stripping (`env -u FACTORY_LOG -u
  FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH -u OPENCODE_CONFIG_CONTENT`): `945 passed, 0 failed`,
  `verify: OK` — spent by the head written after it: CI verify FAILURE at `f3029bd`, 36 refused.
  The repair re-aims exactly those 36 — `:444`→`:489`, `:447`→`:492`, 25 rows, uniform `+45`,
  destinations among the gate's printed homes, the `issue-417` Record copy per the newest-Record
  rule; the final-tree `make verify` is quoted in the pull body, never here.

## What is still owed (all of it, none silent)

1. This pull: `CHANGES_REQUESTED` (hermesvicowa, 18:07Z) stands on `f3029bd`; its one blocking
   defect — those 36 pointers — head `94bfdcb` answers, and the verdict now waits on the
   reviewer's re-read, not the poke: a standing verdict holds `factory_request_review` quiet
   (AGENTS.md, `H-0010`). Its NON-BLOCKING notes are NOT addressed at this head — a guard or
   release-before-mint on the `FACTORY_CLAIM_ACTIVE` clobber (`factory.sh:1008-1010`, dead in
   today's order), `${VAR:-60}` at use on `common.sh:1909`'s bare reads (short source → now).
2. `H-0329`'s title, its question, and the #418 journal-fixture comment still carry the same
   `RuntimeMaxSec` misnomer for the 16 Nov activation whose own transcribed journal row says
   `start operation timed out` — that is the start deadline. Those records live on #418's merged
   ref; `H-0341` names them. Dispatch the rename with the fixture as its subject if it is wanted.
3. Carried, verbatim where still live, from the Record below: the merge-census leg (#428's
   append-tail family), the two-word hazard-row repair (#428, #440), the #482 fence leg (trunk
   measured it red too, `932 passed, 1 failed`; the Record's item-4 `#484` wording is merged
   history, the fence is not), and the `SUPERVISOR-FF`/`SUPERVISOR-RERUN` watch — all still open.

## Record — — issue-418 conflict tick, round 3 for PR #484 (branch `issue-418`: the third base sync of this branch is in — `origin/main` moved `2ec2f91` → `629fb5c` (the #486 merge, carrying `H-0336` and `H-0337`, six commits) and both append tails collided: seventy-eight hunks in `factory/DECISIONS.md`, seventy-four of them one coordinate row holding each parent's digits into its own copy of the file, three of them this branch's repeated-anchor repair holding the sentence with the ambiguous anchor quoted twice against trunk's once, the seventy-eighth the tail junction that seats trunk's `H-0336` and `H-0337` above this branch's `H-0338` by ascending id; `NEXT.md` conflicted again after round 2's clean pass — both parents re-headed while trunk rewrote the shared constraint rows under the one `## Constraints` heading git kept single, so both heads were demoted in place to `## Record —` and each Record keeps its own Constraints section, trunk's stack carried byte-kept beneath. The mechanical union was MEASURED before any side was trusted: each parent's archived tree answers `0` moved pointers, the union answered `186` — 141 landed by the shipped gate's own single refusals and 45 multi-home rows were adjudicated by cited-plus-regional-delta (uniform `+11` in `factory/common.sh`, `+175` in `factory/selftest.sh`, and the `H-0228` entry's own `busy_guard` definition-and-leg pair corroborating the second) intersected with the printed homes; the hazard pair re-seated at the `issue-417` Record copy, the newest Record copy by position, and the file re-headed after, so every in-log pointer was aimed at the bytes that ship. The adjudication is `H-0339`, carried by the merge commit itself; the pull Refs #418 and the issue keeps its own terms, never this file; merging belongs to the supervisor)

## What this round shipped

- `origin/main` merged into `issue-418` as one merge commit, the only shape available here (squash
  and rebase merge are off, force-push is denied to a tick), so the resolution is proved and never
  rewritten. The adjudication is `H-0339`, checkpoint `b944cda`, the allocator having answered
  `H-0339` on a `REFRESH 8s union` read taken before the checkpoint and re-read once both
  resolutions were staged.
- `factory/DECISIONS.md`: the tail junction is a union in ascending id order (`H-0336`, `H-0337`,
  `H-0338`); of the seventy-seven equal-length hunks seventy-four read digit-equal on their paired
  rows before any side was taken and three differ only by this branch's repeated-anchor repair
  (the ambiguous anchor quoted twice where trunk quotes it once), classified row by row, HEAD
  kept as the shipped repair. All 186 pointers the union broke were re-aimed on the merged bytes;
  the gate reads `0` refused on its 583 debt unmoved.
- The zero-deletion census is the `H-0330` both-parents multiset run, digits folded, over both
  parents' blobs of both conflicted files, quoted in the pull-request body, never here: ours
  loses 0 log rows and one spent head row; trunk misses three log rows — all three are the rows
  this branch's repeated-anchor repair rewrote — and one spent head row. The two auto-merged
  files were censused too: every row missing against a parent is a row the other parent
  deliberately rewrote there (trunk's `factory_halt_report` rework owns all seven of ours in
  `factory/common.sh`), which is merge semantics, not a swallow.
- `NEXT.md`: re-headed by convention — this head on top; round 2's head demoted in place to
  `## Record —` with its body byte-identical; trunk's `issue-420` head demoted to a `## Record —`
  carried between ours and trunk's stack, its divergent constraint rows kept inside its own
  Record instead of silently overwriting ours under the shared heading; the `issue-418` round-1
  Record and trunk's whole stack carried beneath untouched. The hazard pair still answers on
  FOUR rows; the log's citations land on the `issue-417` Record copy.

## What this push spent

- The push replaced the `APPROVED` read taken at `90fb705`: mid-recompute `mergeStateStatus` read
  `BLOCKED` with an empty `reviewDecision`, then at head `a333c32` the live read-back answers
  `APPROVED` and `CLEAN` — a fresh review on the new head at 15:36:38Z, not a carried approval,
  since both older reviews read `DISMISSED`. Nobody was re-asked here: the ask is the
  supervisor's `factory_request_review` poke, not this unit's, and all five checks read green on
  `a333c32` — the pull sits mergeable for the supervisor alone; this pass merged nothing.

## What is still owed (all of it, none silent)

1. Carried, conditional: the next push to #484 spends its `APPROVED` — the #269/`H-0010` class —
   and if no fresh review lands, only the supervisor's poke asks; see the row above.
2. The merge-census leg itself (#428's class houses the append-tail family): the both-parents
   multiset containment run that catches a body retired from the unmarked middle is still hand-run
   — this round ran it by hand over both blobs of both conflicted files plus the two auto-merged
   files — and round 4 lost 36 lines at rc 0 with no marker, so the next round owes it whether or
   not anything conflicted.
3. The two-word repair of the hazard rows themselves (#428, #440), which is what retires the
   doubling class: the two fragments still answer on FOUR rows of this file, the shipped gate
   accepts a citation whose anchor sits on several rows, and this round moved the copies it was
   handed and minted none.
4. The fence leg (#482, open): `proc_run` APPENDS its env, so a fenced tick's
   `OPENCODE_CONFIG_CONTENT` reaches the fixture's session stub and the control "and nothing
   else does" reads a fence with the dispatch line removed; re-measured again this round inside a
   live tick (the variable is set in this tick's own environment). The repair is an `env -u` of
   the fence name in `proc_run`'s shipped list beside the #96 four; until it lands, run the suite
   as CI runs it — `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make verify`,
   plus `env -u OPENCODE_CONFIG_CONTENT` inside a live tick — and say so wherever the verdict is
   quoted.
5. A verify run whose sandbox journal shows `SUPERVISOR-FF` or `SUPERVISOR-RERUN` reddened
   supervisor legs for MACHINE state — the installed tree behind the shared `origin/main` an
   own fetch had advanced — not for the diff under test; read those journal lines before blaming
   the branch (first measured in the `issue-420` round, recorded in `H-0337`).
6. Carried from this branch's round-2 Record below, unrenumbered: the journal's own history is
   not repairable (`REAP-HELD hold=0s` rows written before this ships stay in the journal); and a
   post-merge branch re-derives the pointer census with
   `python3 scripts/check-decisions.py --citations` (#389).
7. Carried from the `issue-416` Record below, unrenumbered: until that pull's follow-ups land a
   `MAIN-CHECKOUT-RESTORED` naming `.cortexkit/.gitignore` is #416's churn not a clean verdict;
   the first post-merge main-checkout sync deleting the untracked marker is expected, not a
   defect; #346 stays open on its own terms.
8. Carried from trunk's carried entries no parent head re-headed: `H-0335`'s `Follow-up:` still
   owes a leg for the `124`-vs-`137` branch of `push_why` (its fixture answers TERM, so the arm
   is a forensics gap), and `H-0333` still shares `died=EXIT` between a RuntimeMaxSec kill and an
   internal abort (`set -u`, a failed redirection).
9. Pointer digits inside Record bodies older than this head are stamped, not live — a `NEXT.md`
   digit outside the log is read by no gate (#375); the live digits are the log's anchored
   citations, and the shipped gate re-measures those on every run.

## Constraints a stranger would break here

- After ANY merge whose trunk side rewrote this file, run the both-parents multiset census
  (every line of `git show <parent>:NEXT.md`, digits folded, contained in the merged file) before
  trusting the auto-merged middle: round 4 lost 36 lines with zero conflict markers, and `H-0330`
  carries the measurement. Run it even when this file did not conflict, and run it on the
  auto-merged files too — rows missing against a parent must be named as the other parent's
  deliberate edits, not left uncounted.
- Anyone who edits a head here owes `python3 scripts/check-decisions.py --citations NEXT.md`
  again: the in-log pointers aim at the two carried hazard rows and every head edit moves them.
  The census is re-run after every head edit and never inherited — and head edits come BEFORE
  pointer re-aims, because a prepend moves every landing the re-aim just measured.
- Those two hazard fragments each answer on FOUR rows of this file, and the shipped gate accepts
  a citation whose anchor sits on several rows (#440) — so a re-aim must land on the `issue-417`
  Record copy (the newest Record copy by position), not on whichever row the search finds first.
- Both heads of a colliding round are demoted to `## Record —` with their OWN `## Constraints`
  section; the single shared `## Constraints` heading git keeps between two divergent bullet
  sets is the trap — adopting one side's bullets there silently retires the other's rows.
- Run the suite as CI runs it: `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make
  verify`, and say so wherever the verdict is quoted. Inside a live tick also carry
  `env -u OPENCODE_CONFIG_CONTENT` for `factory/selftest.sh` — the #482 leak poisons the fence
  control otherwise, as trunk's `issue-415` Record already warned.
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or
  an approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.

## Record — issue-418 conflict tick, round 2 for PR #484 (branch `issue-418`: the second base sync of this branch is in — `origin/main` moved `5d0e6e2` → `2ec2f91` (the #485 merge, carrying `H-0332`, `H-0333` and `H-0335`) and the same two append tails conflicted a sixth time in this family: sixty-three hunks in `factory/DECISIONS.md`, fifty-nine of them one carried sentence holding each parent's digits into its own copy of this file, three of them this branch's repeated-anchor repair holding that sentence with the ambiguous anchor quoted twice against trunk's once, the sixty-third the tail junction that seats trunk's `H-0332`, `H-0333` and `H-0335` among this branch's `H-0331` and `H-0334` by ascending id. For the first round of this family the hand-off did not collide at all: trunk's nine commits never touched this file (`git diff --name-only 5d0e6e2 2ec2f91` answers `factory/DECISIONS.md`, `factory/common.sh`, `factory/factory.sh`, `factory/selftest.sh` — none of them this one), so the whole-file head-against-head of rounds 4 and 5 is absent and this file is re-headed by convention alone — this round on top, round 1's head demoted to `## Record —` in place, trunk's whole stack carried byte-kept beneath it — and every live pointer is re-aimed at the carried pair inside the `issue-417` Record, the newest Record copy by position and the section that names itself as the anchor for the next rewriter; each destination opened before its digit was written. The adjudication is `H-0338`, carried by the merge commit itself; the pull Refs #418 and the issue keeps its own terms, never this file; merging belongs to the supervisor)

## What this round shipped

- `origin/main` merged into `issue-418` as one merge commit, the only shape available here (squash
  and rebase merge are off, force-push is denied to a tick), so the resolution is proved and never
  rewritten. The adjudication is `H-0338`, checkpoint `9b9a692`, the allocator having answered
  `H-0338` on a `REFRESH 21s union` read taken before the checkpoint and re-read once both
  resolutions were staged, with the supervisor journal holding one `H-0338` line — this unit's own.
- `factory/DECISIONS.md`: the tail junction is a union in ascending id order (`H-0331`, `H-0332`,
  `H-0333`, `H-0334`, `H-0335`); of the sixty-two equal-length hunks fifty-nine read digit-equal on
  their paired rows before any side was taken, three differing only by this branch's repeated-anchor
  repair (the ambiguous anchor quoted twice where trunk quotes it once), classified row by row. That
  union was then MEASURED, not trusted: the shipped gate answers `0` moved on each parent's archived
  tree and `168` on the union — 114 rows, thirteen of them `#440`'s several-landing class, settled by
  the continuation walk downward from `9b9a692`, the newest tree the gate reads as zero moved. All of
  them re-aimed on the merged bytes: 207 tokens in 144 rows, gate back to `0` on its 583 debt unmoved.
- The zero-deletion census is the `H-0330` both-parents multiset run, digits folded, over both parents'
  blobs of both files, quoted in the pull-request body, never here: ours loses 0 of 16423 log rows and
  one spent head row; trunk misses five — three its anchor repair, two re-aims git took from the middle.
- `NEXT.md`: re-headed by convention only — this head above round 1's, which is demoted in place to
  `## Record —` with its body byte-identical, and trunk's whole stack carried beneath it untouched save
  that same demotion (`+86` lines added, nothing removed: every row below keeps its order, and the
  carried pair the log aims at is still `NEXT.md:284` and `NEXT.md:287` by the merged file's own count).

## What this push spent

- The `APPROVED` read taken at head `6b4d193` is spent by this push — the #269/`H-0010` class. The
  row flips `DISMISSED`, `mergeStateStatus` reads `BLOCKED` until a reviewer is asked again, and
  nobody has been re-asked yet: the ask is the supervisor's `factory_request_review` poke, not
  this unit's.

## What is still owed (all of it, none silent)

1. Re-review of #484 after this push — the poke owns the ask; see the row above.
2. The merge-census leg itself (#428's class houses the append-tail family): the both-parents
   multiset containment run that catches a body retired from the unmarked middle is still hand-run
   — this round ran it by hand over both blobs of both files — and round 4 lost 36 lines at rc 0
   with no marker, so the next round owes it whether or not anything conflicted.
3. The two-word repair of the hazard rows themselves (#428, #440), which is what retires the
   doubling class: the two fragments still answer on FOUR rows of this file, the shipped gate
   accepts a citation whose anchor sits on several rows, and this round moved the copies it was
   handed and minted none.
4. Carried from this branch's round-1 `issue-418` Record below, unrenumbered: the journal's own
   history is not repairable (`REAP-HELD hold=0s` rows written before this ships stay in the
   journal); `make verify` inside a live tick still fails one fence leg, filed as #482 — re-measure
   with `env -u OPENCODE_CONFIG_CONTENT bash factory/selftest.sh` until that lands; and a
   post-merge branch re-derives the pointer census with
   `python3 scripts/check-decisions.py --citations` (#389).
5. Carried from trunk's round-4 head, now the demoted `issue-399` Record below, unrenumbered: its
   items 4–7 (the `H-0328` carry, the `issue-416` marker terms, the stamped-vs-live digit rule).
   Its items 2–3 are this file's items 2 and 3 above, and its item 1 was paid by the re-review
   round 5 owed.
6. Carried from the `issue-416` Record below, unrenumbered: until that pull's follow-ups land a
   `MAIN-CHECKOUT-RESTORED` naming `.cortexkit/.gitignore` is #416's churn not a clean verdict;
   the first post-merge main-checkout sync deleting the untracked marker is expected, not a
   defect; #346 stays open on its own terms.
7. New from trunk's three entries this merge unions in, which no parent head carries because
   trunk never re-headed this file: `H-0335`'s `Follow-up:` still owes a leg for the `124`-vs-`137`
   branch of `push_why` (its fixture answers TERM, so the arm is a forensics gap), and `H-0333`
   still shares `died=EXIT` between a RuntimeMaxSec kill and an internal abort (`set -u`, a failed
   redirection).
8. Pointer digits inside Record bodies older than this head are stamped, not live — a `NEXT.md`
   digit outside the log is read by no gate (#375); the live digits are the log's anchored
   citations, and the shipped gate re-measures those on every run.

## Constraints a stranger would break here

- After ANY merge whose trunk side rewrote this file, run the both-parents multiset census
  (every line of `git show <parent>:NEXT.md`, digits folded, contained in the merged file) before
  trusting the auto-merged middle: round 4 lost 36 lines with zero conflict markers, and `H-0330`
  carries the measurement. Run it even when this file did not conflict, as it did not this round.
- Anyone who edits a head here owes `python3 scripts/check-decisions.py --citations NEXT.md`
  again: the in-log pointers aim at the two carried hazard rows and every head edit moves them.
  The census is re-run after every head edit and never inherited.
- Those two hazard fragments each answer on FOUR rows of this file, and the shipped gate accepts
  a citation whose anchor sits on several rows (#440) — so a re-aim must land on the `issue-417`
  Record copy (the newest Record copy by position), not on whichever row the search finds first.
- Run the suite as CI runs it: `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make
  verify`, and say so wherever the verdict is quoted. Inside a live tick also carry
  `env -u OPENCODE_CONFIG_CONTENT` for `factory/selftest.sh` — the #482 leak poisons the fence
  control otherwise, as trunk's `issue-415` Record already warned.
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or
  an approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.


## Record — issue-420 unit (branch `issue-420`: one guarded HALT now speaks one asking clause through every reader, and the fifth base sync on this stack is in — `origin/main` moved `5d0e6e2` → `2ec2f91` (the #485 merge, carrying `H-0332`, `H-0333`, `H-0335`): ninety conflict hunks in `factory/DECISIONS.md`, eighty-nine of them one coordinate row holding each parent's digits into its own copy of the same sentence, the ninetieth the tail junction that seats `H-0336` after trunk's `H-0335` by ascending id; the 202 pointers both parents' insertions moved were re-derived on the merged tree by the anchored-citation gate, multi-home rows seated at cited-plus-regional-delta ∩ printed homes — the adjudication is `H-0337`, checkpoint `afa65ff`, carried by the merge commit itself. `NEXT.md` did NOT conflict in this merge — both parents carry it byte-identical at 3971 lines and the digits-folded multiset census proves the merged file contains each parent with zero missing, so no unmarked-swallow repair was owed this round. This file is re-headed by convention: this round on top, the `issue-399` round-4 head demoted to `## Record —` in place, every body byte-kept; merging belongs to the supervisor)

## What this round shipped

- `factory_verdict`'s HALTED arm now runs the same zero-write `factory_halt_target` test the
  `halt:` line runs and hands the composer that verdict, so a guarded marker no longer prints
  "and not re-probed since" beside the supervisor's "re-probed on every activation" in one
  `--status` run; a hand-written marker keeps #237's default clause on both readers. `H-0336`
  carries the three seats, and the effect commit `e6cc64c` also re-aimed the 190 log pointers
  its own insertions moved.
- Three selftest legs spawn the shipped `factory.sh --status` as a process over both marker
  shapes and require the pair's asking clauses to agree; the control plants the pre-#420
  zero-argument call in a copy OUTSIDE the repository and refuses the legs unless that copy
  disagrees while an unmutated copy agrees.
- The base sync: `e499bef`, `Decision: H-0337`, with the 202-row re-aim named above.
- The green gate on these bytes (quoted in full in the pull-request body): `make verify` under
  `env -u OPENCODE_CONFIG_CONTENT` reads `factory/selftest.sh: 931 passed, 0 failed` and
  `verify: OK` — the replacement for the 916/6 red this branch showed on the stale base.

## What is still owed (all of it, none silent)

1. The supervisor merges and the poke owns the reviewer ask; this unit merged nothing and asked
   nobody.
2. The fence leg (#482, open): `proc_run` APPENDS its env, so a fenced tick's
   `OPENCODE_CONFIG_CONTENT` reaches the fixture's session stub and the control "and nothing
   else does" reads a fence with the dispatch line removed — measured again this round, inside
   a live tick, with the mutant's residue byte-identical to the tick's own export. The repair
   is an `env -u OPENCODE_CONFIG_CONTENT` in `proc_run`'s shipped list beside the #96 four;
   until it lands, run the suite as CI runs it and say so wherever the verdict is quoted.
3. Carried from the `issue-399` round-4 head below, unrenumbered: all of its owed items — the
   merge-census tool owed to a harness unit (#428's class), the hazard rows standing three
   copies deep with the two-word repair still owed (#428, #440), the `H-0328` Record's items
   2–5, trunk's `issue-417` Record's owed items, the `issue-416` carry, and the
   stamped-vs-live digit rule.
4. A verify run whose sandbox journal shows `SUPERVISOR-FF` or `SUPERVISOR-RERUN` reddened
   supervisor legs for MACHINE state — the installed tree behind the shared `origin/main` my
   own fetch had advanced — not for the diff under test; read those journal lines before
   blaming the branch (measured this round, recorded in `H-0337` and the pull-request body).

## Constraints a stranger would break here

- After ANY merge whose trunk side rewrote this file, run the both-parents multiset census
  (every line of `git show <parent>:NEXT.md`, digits folded, contained in the merged file)
  before trusting the auto-merged middle — round 4 lost 36 lines at zero markers; this round's
  merge needed it and passed it.
- Anyone who edits a head here owes `python3 scripts/check-decisions.py --citations NEXT.md`
  again: the in-log pointers aim at the two hazard rows and every head edit moves them. This
  round's prepend moved the newest copy (inside the demoted round-4 head) and all 26 log
  pointers were re-aimed in this round's commit by the gate's own refusals; re-aim lands on
  the newest Record copy, never on whichever row a search finds first (#440).
- Run the suite as CI runs it: `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH
  make verify`, and inside a live tick also `env -u OPENCODE_CONFIG_CONTENT` (owed item 2) —
  say so wherever the verdict is quoted.
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge
  or an approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.

## Record — issue-418 conflict tick, round 1 for PR #484 (branch `issue-418`: the first base sync of this branch is in — `origin/main` moved `49f7fa1` → `5d0e6e2` (the #479 merge, carrying `H-0330`) and the same two append tails conflicted a fifth time in this family: fourteen hunks in `factory/DECISIONS.md`, thirteen of them one carried sentence holding each parent's digits into its own copy of this file, the fourteenth the tail junction that seats trunk's `H-0328` and `H-0330` among this branch's `H-0329` and `H-0331` by ascending id. `NEXT.md` collided whole-file: this branch shipped the compressed 72-line overwrite carrying its own copy of the hazard pair, trunk shipped the 3971-line Record stack, and git aligned the two heads against each other; this resolution re-heads by convention — this round on top, both parents' heads demoted to `## Record —` in place, every body byte-kept, trunk's whole stack carried above this branch's demoted `issue-418` Record — and re-aims every live pointer at the carried pair inside the `issue-417` Record, the newest Record copy by position and the section that names itself as the anchor for the next rewriter; each destination opened before its digit was written. The adjudication is `H-0334`, carried by the merge commit itself; the pull Refs #418 and the issue keeps its own terms, never this file; merging belongs to the supervisor)

## What this round shipped

- `origin/main` merged into `issue-418` as one merge commit, the only shape available here (squash
  and rebase merge are off, force-push is denied to a tick), so the resolution is proved and never
  rewritten. The adjudication is `H-0334`, checkpoint `ecb6b0d`, the allocator having answered
  `H-0334` on a `REFRESH 103s union` read taken before the checkpoint, with the supervisor journal
  holding one `H-0334` line — this unit's own.
- `factory/DECISIONS.md`: the tail junction is a union in ascending id order (`H-0328`, `H-0329`,
  `H-0330`, `H-0331`); the thirteen digit hunks were adjudicated on the merged tree, not per side —
  every unqualified pointer naming the hazard pair was re-aimed at the `issue-417` Record copy
  this merged file makes true, and the one `docs/factory.md` digit inside `H-0282`'s `Follow-up:`
  row was taken from the merged bytes (the merged file is this branch's for that path —
  `git diff HEAD -- docs/factory.md` read empty before the merge was resolved) — with the shipped
  gate's own anchor refusals, not arithmetic, naming every landing.
- `NEXT.md`: re-headed by convention — both parents' heads demoted in place, trunk's full Record
  stack byte-kept above this branch's demoted `issue-418` Record; the zero-deletion census is the
  `H-0330` both-parents multiset containment run, quoted in the pull-request body, never here.

## What this push spent

- The `APPROVED` read taken at head `3dff1a7` is spent by this push — the #269/`H-0010` class. The
  row flips `DISMISSED`, `mergeStateStatus` reads `BLOCKED` until a reviewer is asked again, and
  nobody has been re-asked yet: the ask is the supervisor's `factory_request_review` poke, not
  this unit's.

## What is still owed (all of it, none silent)

1. Re-review of #484 after this push — the poke owns the ask; see the row above.
2. Carried from trunk's round-4 head below, unrenumbered: its item 2 (the merge-census tool that
   would make the unmarked swallow a leg instead of a discovery, owed to a harness unit — #428's
   class), its item 3 (the two-word repair of the hazard rows themselves, which would retire the
   doubling class; #428, #440), and its items 4–7 (the `H-0328` carry, the `issue-416` marker
   terms, the stamped-vs-live digit rule). Its item 1 was paid: the re-review round 4 owed
   landed as the `H-0330` this merge carries.
3. Carried from this branch's `issue-418` Record below, unrenumbered: the journal's own history
   is not repairable (`REAP-HELD hold=0s` rows written before this ships stay in the journal);
   `make verify` inside a live tick still fails one fence leg, filed as #482 — re-measure with
   `env -u OPENCODE_CONFIG_CONTENT bash factory/selftest.sh` until that lands; and a post-merge
   branch re-derives the pointer census with `python3 scripts/check-decisions.py --citations`
   (#389).
4. The hazard rows now stand FOUR times in this file — trunk's carried pair inside the `issue-417`
   Record (the live anchor set for the log's citations), the `issue-416` Record's copy, this
   round's `issue-418` Record copy (the compressed overwrite's pair, demoted verbatim), and the
   branch stack's older carried copy deeper below — because the zero-deletion rule forbids
   retiring any of them and the copies quote alike. The repair owed is item 2's two-word fix;
   this round touched none of the copies.
5. Pointer digits inside Record bodies older than this head are stamped, not live — a `NEXT.md`
   digit outside the log is read by no gate (#375); the live digits are the log's anchored
   citations, and the shipped gate re-measures those on every run.

## Constraints a stranger would break here

- After ANY merge whose trunk side rewrote this file, run the both-parents multiset census
  (every line of `git show <parent>:NEXT.md`, digits folded, contained in the merged file) before
  trusting the auto-merged middle: round 4 lost 36 lines with zero conflict markers, and `H-0330`
  carries the measurement.
- Anyone who edits a head here owes `python3 scripts/check-decisions.py --citations NEXT.md`
  again: the in-log pointers aim at the two carried hazard rows and every head edit moves them.
  The census is re-run after every head edit and never inherited.
- Those two hazard fragments each answer on FOUR rows of this file, and the shipped gate accepts
  a citation whose anchor sits on several rows (#440) — so a re-aim must land on the `issue-417`
  Record copy (the newest Record copy by position), not on whichever row the search finds first.
- Run the suite as CI runs it: `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make
  verify`, and say so wherever the verdict is quoted. Inside a live tick also carry
  `env -u OPENCODE_CONFIG_CONTENT` for `factory/selftest.sh` — the #482 leak poisons the fence
  control otherwise, as trunk's `issue-415` Record already warned.
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or
  an approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.

## Record — issue-399 conflict tick, round 4 (branch `issue-399`, PR #479: the fourth base sync is in — `origin/main` moved `0f343db` → `49f7fa1` (the #483 merge, carrying `H-0327`) and the same two append tails conflicted a fourth time: fourteen hunks in `factory/DECISIONS.md`, thirteen of them one carried sentence holding each parent's digits into its own copy of this file, the fourteenth the tail junction that seats trunk's `H-0327` between this branch's `H-0326` and `H-0328` by ascending id. The overwrite shape returned and this time git's alignment destroyed a body silently: trunk's round shipped `NEXT.md` again as a compressed overwrite (49 lines → 53), and its `Carried forward` block is byte-identical to the one inside our demoted `issue-416` Record, so git aligned on that block and the AUTO-MERGED region wrote trunk's new body over the Record's body — `git show f4f9d6d:NEXT.md` lines 64-99, thirty-six lines, absent from every conflict hunk and from the working file at rc 0, no marker. This resolution restores that Record byte-for-byte from this branch's parent, carries trunk's head and body as the demoted `issue-417` Record above it, and re-aims every live pointer at the carried pair inside the `issue-417` Record — the newest Record copy, the section that names itself as the anchor for the next rewriter; each destination opened before its digit was written. This file is re-headed by convention: this round on top, both parents' heads demoted to `## Record —` in place, every body byte-kept; the zero-deletion census here is a multiset containment run on BOTH parents (a per-side diff cannot see a body stolen from the unmarked middle), and it is `H-0330`; the pull Refs #399 and the issue keeps its own terms, never this file; merging belongs to the supervisor)

## What this round shipped

- `origin/main` merged into `issue-399` as one merge commit, the only shape available here (squash
  and rebase merge are off, force-push is denied to a tick), so the resolution is proved and never
  rewritten. The adjudication is `H-0330`, carried by the merge commit itself, checkpoint `f4f9d6d`,
  the allocator having answered `H-0330` on a `REFRESH 61s union` read before the checkpoint was
  taken. `H-0329` is not a hole this unit authored: it lands on `refs/remotes/origin/issue-418`.
- `factory/DECISIONS.md`: the tail junction is a union in ascending id order (`H-0326`, `H-0327`,
  `H-0328`), and the thirteen digit hunks were adjudicated per citation on the merged tree, not per
  side; the two `factory/factory.sh` digits inside `H-0313`'s `Effect:` row took the merged bytes'
  coordinates (the merged file is trunk's for that path — `git diff origin/main -- factory/factory.sh`
  reads empty — and the cited lines were opened, not arithmeticed).
- `NEXT.md`: the unmarked loss repaired — the `issue-416` Record body restored byte-identical from
  this branch's parent, trunk's shipped body kept whole as the demoted `issue-417` Record, the
  branch stack untouched beneath; re-headed by convention, both parents' heads demoted in place,
  every body byte-kept. The zero-deletion census, all gate verdicts and the restore proof are
  quoted in the pull-request body, never here.

## What this push spent

- The `APPROVED` read taken at head `62653fc` is spent by this push — the #269/`H-0010` class, the
  fourth time on this branch. The row flips `DISMISSED`, `mergeStateStatus` reads `BLOCKED` until a
  reviewer is asked again, and nobody has been re-asked yet: the ask is the supervisor's
  `factory_request_review` poke, not this unit's.

## What is still owed (all of it, none silent)

1. Re-review of #479 after this push — the poke owns the ask; see the row above.
2. The unmarked swallow itself: a compressed trunk overwrite whose tail aligns with a carried
   section inside our stack lets git's auto-merge retire an appended Record's body at rc 0, and
   only a both-parents multiset census of the merged file catches it — a per-side conflict walk
   finds nothing because there is no conflict. The merge-census tool that would make this a leg
   instead of a hand-run is owed to a harness unit (#428's class houses the append-tail family);
   this round restored the 36 lines and disclosed the mechanism, it built no tool.
3. The hazard rows now stand THREE times in this file — trunk's carried pair inside the demoted
   `issue-417` Record (the live anchor set for the log's citations), the `issue-416` Record's copy
   the restore put back, and the branch stack's older carried copy deeper below — because the
   zero-deletion rule forbids retiring any of them and the three copies quote alike. Every future
   union of a compressed trunk hand-off with an appended branch hand-off doubles them again; the
   two-word repair of the rows themselves is still owed and would retire the whole class
   (#428, #440). This round touched none of the copies: the log's citations are the claim, and the
   rows are read at their heads.
4. Carried from the `H-0328` Record below, unrenumbered: its items 2–5 (the hazard-repair owed
   list, the trunk-Record carry rule, the stamped-vs-live digit rule). Its item 1 was paid — the
   `APPROVED` this push spends was filed against round 3's head.
5. Carried from trunk's `issue-417` Record below, unrenumbered: all of its owed items — the
   re-review of the #483 merge, `make verify` inside a live tick failing the fence leg filed as
   #482, and the post-merge pointer-census re-derivation.
6. Carried from the `issue-416` Record below, unrenumbered: until that pull's follow-ups land a
   `MAIN-CHECKOUT-RESTORED` naming `.cortexkit/.gitignore` is #416's churn not a clean verdict;
   the first post-merge main-checkout sync deleting the untracked marker is expected, not a
   defect; #346 stays open on its own terms.
7. Pointer digits inside Record bodies older than this head are stamped, not live — a `NEXT.md`
   digit outside the log is read by no gate (#375); the live digits are the log's anchored
   citations, and the shipped gate re-measures those on every run.

## Constraints a stranger would break here

- After ANY merge whose trunk side rewrote this file, run the both-parents multiset census
  (every line of `git show <parent>:NEXT.md`, digits folded, contained in the merged file) before
  trusting the auto-merged middle: round 4 lost 36 lines with zero conflict markers, and `H-0330`
  carries the measurement.
- Anyone who edits a head here owes `python3 scripts/check-decisions.py --citations NEXT.md` again:
  the in-log pointers aim at the two carried hazard rows and every head edit moves them. The census
  is re-run after every head edit and never inherited.
- Those two hazard fragments each now answer on THREE rows of this file, and the shipped gate
  accepts a citation whose anchor sits on several rows (#440) — so a re-aim must land on the
  `issue-417` Record copy (the newest Record copy), not on whichever row the search finds first.
- Run the suite as CI runs it: `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make
  verify`, and say so wherever the verdict is quoted. Inside a live tick also carry
  `env -u OPENCODE_CONFIG_CONTENT` for `factory/selftest.sh` — the #482 leak poisons the fence
  control otherwise, as trunk's `issue-415` Record already warned.
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or an
  approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.

## Record — issue-417: the conflict fetch defers with git's cause in the journal, behind the fetch bound (branch `issue-417`; the 20 Nov adoption pass ships it — merging belongs to the supervisor, and the one close-verb on this work lives in the pull body)

State this pass read off the live tree before writing a byte (2026-11-20): trunk still `0f343db`
(#481 merged 06:27Z), `gh pr list --head issue-417` answered `[]`, and every gate re-ran green on
the shipped bytes — `919 passed, 0 failed` on `env -u OPENCODE_CONFIG_CONTENT
bash factory/selftest.sh`, plus check-shell, check-denies, check-next-md and check-decisions.

## What this branch ships

- The conflict arm's fetch of `origin/$base` in `factory_resolve_conflict` no longer returns 2
  behind a silent `FAILED (128)`: it captures stderr to the swept `factory_fetch_stream()`
  (factory/common.sh), quotes git's FIRST stderr line into its own `CONFLICT-BLOCKED` sentence
  under the harness's `git said:` marker — the last line of a transport burst is the access-rights
  decoy — and it runs behind `FACTORY_FETCH_TIMEOUT` like the refresh arm's activation fetch, so
  a parked connection now answers `killed at the …s fetch bound` instead of costing 135 s a unit.
  `run_action` is off this call on purpose: it normalizes every failure to 1, and this arm must
  tell 124 from 128.
- `H-0327` in factory/DECISIONS.md carries the four options, the three losses and the measured
  receipts (the 16 Nov SSH burst; the empty-stderr 124 measured over a parking stub, where git
  retries ssh once, so one burst costs 2x the park); checkpoint a92a8bc.
- The selftest grew the transport family at its tail (a fast ssh failure, a parked ssh killed at
  the bound, and blinded controls for the clause, the bound and the sweep term), plus two harness
  knobs: `proc_extra_env` reaches the pass BEFORE the shipped env — a leg moving a name the
  shipped list re-spells must use a `proc_*` knob (`proc_fetch_timeout` is the new one), never
  the array — and journal needles that are substrings of `REFRESH-FAILED` rows must keep their
  leading space.
- The 219 `path:line` pointers this round's inserts slid were re-aimed by the checker's own
  census; `python3 scripts/check-decisions.py` reads rc 0 with the debt unmoved at 583.
- #417's third proposal is NOT here: port 443 of `ssh.github.com` is a machine `~/.ssh/config`
  edit, named in H-0327's `Revisit:` for whoever owns that file.

## What is still owed

- The supervisor merges; this pass merged nothing, and no artifact but the pull body carries a
  closing verb. The reviewer ask is the supervisor's poke, per AGENTS.md.
- `make verify` inside a live tick fails one fence leg (`and nothing else does`, filed as #482):
  the tick's own OPENCODE_CONFIG_CONTENT leaks through `proc_run` into the control; until that
  lands, re-measure with `env -u OPENCODE_CONFIG_CONTENT bash factory/selftest.sh` (H-0327).
- After this merges, a later branch re-derives the pointer census with
  `python3 scripts/check-decisions.py --citations` (#389); if the SSH burst recurs, `git said:`
  quotes git's answer and `killed at the …s fetch bound` names the parked connection.

## Carried forward — verbatim, because the decision log is anchored into these two lines

The close-verb lesson of the inherited ledger stays on the two lines the twelve
factory/DECISIONS.md rows anchor into this file; the next rewrite of this file re-aims those
twelve rows exactly as this round did:

- The census row of the `issue-364` hand-off, carried because the rule outlives its round —
  quoted from it must name its clock. And the row about #425 says "Do not close #425 on this branch's
- The #425 instance, carried with it: neither proposed cause reproduced on the measuring tree,
  so the question stays open even though that unit measured a real
  RED on its own shell. Do not close #425 on this branch's measurement alone.

## Record — issue-399 conflict tick, round 3 (branch `issue-399`, PR #479: the third base sync is in — `origin/main` moved `ae61064` → `0f343db` (the #481 merge, carrying `H-0325`) and the same two append tails conflicted a third time: thirteen hunks in `factory/DECISIONS.md`, twelve of them one carried sentence holding each parent's digits into its own copy of this file, the thirteenth the tail junction that seats trunk's `H-0325` between this branch's `H-0324` and `H-0326` by ascending id. The hand-off conflicted in a new shape: trunk's round shipped `NEXT.md` as a 49-line overwrite whose `Carried forward` section holds its own copy of the two hazard rows, while this branch's copy is a 3728-line append stack holding its older pair, so the merged file carries both copies and neither parent's digits were inherited — every live pointer into the hand-off was re-aimed at the rows inside trunk's `issue-416` Record, the section that names itself as the anchor for the next rewriter, each destination opened before its digit was written. This file is re-headed by convention: this round on top, trunk's `issue-416` head demoted to `## Record —` in place as the newest parent head, this round-2 head demoted in place atop the branch stack, every body byte-kept, no shared section to fold this round; the pass is `H-0328`; the pull Refs #399 and the issue keeps its own terms, never this file; merging belongs to the supervisor)

## What this round shipped

- `origin/main` merged into `issue-399` as one merge commit, the only shape available here (squash
  and rebase merge are off, force-push is denied to a tick), so the resolution is proved and never
  rewritten. The adjudication is `H-0328`, carried by the merge commit itself, checkpoint `a77ad82`,
  the allocator having answered `H-0328` on a `REFRESH 23s union` read before the checkpoint was
  taken. `H-0327` is not a hole this unit authored: it lands on `refs/remotes/origin/issue-417`.
- `factory/DECISIONS.md`: the tail junction is a union in ascending id order (`H-0324`, `H-0325`,
  `H-0326`), and the twelve digit hunks plus every silently auto-merged stale row were adjudicated
  per citation on the merged tree, not per side. The hazard pair now sits inside the demoted
  `issue-416` Record, so trunk's own `H-0325` `Effect:` row lost its digits when its overwrite
  stacked under this head — its citations were re-aimed like every other live row.
- `NEXT.md`: re-headed by convention — this round on top, both parents' heads demoted in place,
  every body byte-kept; the zero-deletion census and all gate verdicts are quoted in the
  pull-request body, never here.

## What this push spent

- The `APPROVED` read at head `a11a6a3` is spent by this push — the #269/`H-0010` class, the third
  time on this branch. The row flips `DISMISSED`, `mergeStateStatus` reads `BLOCKED` until a reviewer
  is asked again, and nobody has been re-asked yet: the ask is the supervisor's
  `factory_request_review` poke, not this unit's.

## What is still owed (all of it, none silent)

1. Re-review of #479 after this push — the poke owns the ask; see the row above.
2. The hazard rows keep `python3 scripts/check-close-verb.py NEXT.md` at rc 1 and now sit TWICE in
   this file — trunk's carried copy inside the `issue-416` Record (the live anchor set for the log's
   citations) and the branch stack's older carried copy deeper below — because the zero-deletion
   rule forbids retiring either, and the two copies quote alike. Every future union of a compressed
   trunk hand-off with an appended branch hand-off doubles them again; the two-word repair of the
   rows themselves is still owed and would retire the whole class (#428, #440). This round touched
   neither copy: the log's citations are the claim, and the rows are read at their heads.
3. Carried from the `H-0326` Record below, unrenumbered: its items 2–5 (the hazard-repair owed list,
   the trunk-Record carry rule, the stamped-vs-live digit rule). Its item 1 was paid — the
   `APPROVED` this push spends was filed against round 2's head.
4. Carried from trunk's `issue-416` Record below, unrenumbered: all of its owed items — until that
   pull merges a `MAIN-CHECKOUT-RESTORED` naming `.cortexkit/.gitignore` is #416's churn not a clean
   verdict; the first post-merge main-checkout sync deleting the untracked marker is expected, not a
   defect; #346 stays open on its own terms.
5. Pointer digits inside Record bodies older than this head are stamped, not live — a `NEXT.md`
   digit outside the log is read by no gate (#375); the live digits are the log's anchored
   citations, and the shipped gate re-measures those on every run.

## Constraints a stranger would break here

- Anyone who edits a head here owes `python3 scripts/check-decisions.py --citations NEXT.md` again:
  the in-log pointers aim at the two carried hazard rows and every head edit moves them. The census
  is re-run after every head edit and never inherited.
- Those two hazard fragments each now answer on TWO rows of this file, and the shipped gate accepts
  a citation whose anchor sits on several rows (#440) — so a re-aim must land on the `issue-416`
  Record copy by this file's convention, not on whichever row the search finds first.
- Run the suite as CI runs it: `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make
  verify`, and say so wherever the verdict is quoted. Inside a live tick also carry
  `env -u OPENCODE_CONFIG_CONTENT` for `factory/selftest.sh` — the #482 leak poisons the fence
  control otherwise, as trunk's `issue-415` Record already warned.
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or an
  approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.

## Record — issue-416: the cortexkit marker is untracked, so the main-checkout restore arm keeps its full semantics and loses its only permanent subject (branch `issue-416`, Refs #416; merging belongs to the supervisor, and the one close-verb on this work lives in the pull body)

State this unit read off the live tree and the journal before it wrote a byte
(2026-11-20T05:31–05:35Z): the main checkout read CLEAN at `ae61064`; the two untracked strays
#416 named (keepme.txt and wiring_post.py) are already gone from it; the supervisor's journal
holds zero `MAIN-CHECKOUT-RESTORED` rows to date, its newest `MAIN-CHECKOUT-CLEAN` is
2026-11-20T05:31:10Z, and its newest `MAIN-CHECKOUT-REFUSED` naming the marker is
2026-11-18T08:34:03Z. #416's premise moved while the ticket queued — toward the defect, not away
from it: with the strays gone, one tool deletion makes the dirt exactly the committed-and-missing
shape, permanently, and the pass starts writing the file back every activation.

## What this branch ships

- `git rm --cached .cortexkit/.gitignore` — the bytes stay on this checkout's disk and the root
  ignore rules keep the whole directory out of every status read in both states, present and
  absent; the restore arm's precondition keeps its teeth and loses its only permanent subject
  here. `git check-ignore -v` against the marker itself answers the root rule.
- `H-0325` in factory/DECISIONS.md carries the question, the four rejected options and the three
  losses; checkpoint 1a8bf37.
- The record the tracked marker used to be now lives in the closing section of docs/factory.md,
  and the two scripts/mutate.py comments that claimed the real tree still carried that tracked
  file read in the past tense now — the `_snap04` fixture still pins the
  tracked-and-never-copied shape, synthetically, with its both-ways assertion intact.

## What is still owed

- The supervisor merges; this unit merged nothing, and no artifact but the pull body carries a
  closing verb.
- Until this pull merges, a `MAIN-CHECKOUT-RESTORED` row naming `.cortexkit/.gitignore` in the
  main checkout is #416's churn, not a clean verdict — the carried `issue-346` hand-off warning
  retires at the merge, not before.
- The first main-checkout sync after the merge: a clean working copy there is deleted by the sync
  itself (git applies the removal; the tooling rewrites the file when it wants it, ignored either
  way) — expected, not a defect. A modified copy meets one modify/delete refusal and the
  quarantine arm absorbs it, once per clone. Do not open a ticket on that single refusal.
- #346 stays open on its own terms; this branch measured and claimed nothing about its remaining
  criteria.

## Carried forward — verbatim, because the decision log is anchored into these two lines

The close-verb lesson of the inherited ledger stays on the two lines the twelve
factory/DECISIONS.md rows anchor into this file; the next rewrite of this file re-aims those
twelve rows exactly as this round did:

- The census row of the `issue-364` hand-off, carried because the rule outlives its round —
  quoted from it must name its clock. And the row about #425 says "Do not close #425 on this branch's
- The #425 instance, carried with it: neither proposed cause reproduced on the measuring tree,
  so the question stays open even though that unit measured a real
  RED on its own shell. Do not close #425 on this branch's measurement alone.

## Record — issue-399 conflict tick, round 2 (branch `issue-399`, PR #479: the second base sync is in — `origin/main` moved `7b47fad` → `ae61064` (the #480 merge, carrying `H-0322` and `H-0323`) thirty-four minutes after the round that shipped `H-0324`, and the same two append tails conflicted again: twelve hunks in `factory/DECISIONS.md`, eleven of them one carried sentence holding each parent's digits into its own copy of this file or into the two files trunk grew, the twelfth the tail junction that seats trunk's `H-0322` and `H-0323` beneath this branch's `H-0324` by ascending id. No parent digit was inherited for a `NEXT.md` pointer, because the merged hand-off matches neither parent: every one was re-aimed at the row the merged file makes true, opened by its quoted fragment. The `factory/factory.sh` and `docs/factory.md` rows took trunk's digits for the measured reason that the merged tree carries trunk's bytes for those two files, which this round checked by opening each cited line rather than by arithmetic. This file is re-headed by convention, this round on top, `H-0324`'s head and trunk's `issue-415` head demoted to `## Record —` in place with every body carried byte-identical, and the shared `issue-372` round-2 Record heading kept once in trunk's wording; the pass is `H-0326`; the pull Refs #399 and the issue keeps its own terms, never this file; merging belongs to the supervisor)

## What this round shipped

- `origin/main` merged into `issue-399` as one merge commit, the only shape available here (squash
  and rebase merge are off, force-push is denied to a tick), so the resolution is proved and never
  rewritten. The adjudication is `H-0326`, carried by the merge commit itself, checkpoint `4a59e75`,
  the allocator having answered `H-0326` on a `REFRESH 65s union` read before the checkpoint was
  taken. `H-0325` is not a hole this unit authored: it lands on `refs/remotes/origin/issue-416`.
- `factory/DECISIONS.md`: the tail junction is a union in ascending id order (`H-0322`, `H-0323`,
  `H-0324`), and the eleven remaining hunks were adjudicated per citation on the merged tree, not
  per side. Where a pointer aimed at `NEXT.md` both parents were false and the shipped gate named
  the landing line for each; where it aimed at `factory/factory.sh` or `docs/factory.md` the merged
  bytes are trunk's, so trunk's digit is the one the line answers.
- `NEXT.md`: re-headed by convention — this round on top, both parents' heads demoted in place, the
  shared `issue-372` round-2 heading carried once in trunk's wording, the `D-0092` Record and every
  body beneath it byte-kept.

## What this push spent

- The `APPROVED` read at head `29d21492` is spent by this push — the #269/`H-0010` class, the second
  time on this branch. The row flips `DISMISSED`, `mergeStateStatus` reads `BLOCKED` until a reviewer
  is asked again, and nobody has been re-asked yet: the ask is the supervisor's
  `factory_request_review` poke, not this unit's.

## What is still owed (all of it, none silent)

1. Re-review of #479 after this push — the poke owns the ask; see the row above.
2. The two hazard rows of this file that keep `python3 scripts/check-close-verb.py NEXT.md` at rc 1 —
   inherited through three heads now, deliberately untouched, and owed the two-word repair at the
   coordinates the gate names. This head moved their numbers again; the log's citations were re-aimed
   at the moved rows, the rows were not.
3. Carried from the `H-0324` Record below, unrenumbered: its items 2–5 (the `D-0092` owed list,
   trunk's Record list, the inherited hazard rows, and the stamped-vs-live digit rule).
4. Carried from trunk's `issue-415` Record below, unrenumbered: its items 1–2 — #480's re-review and
   the whole `issue-372` round-2 owed list beneath it, including the `H-0283` locale pin, the
   #352-errata placement question, the issue-390 items 2–5, round 1's "nine legs" nit and trunk's
   dangling-id census.
5. Pointer digits inside Record bodies older than this head are stamped, not live — a `NEXT.md`
   digit outside the log is read by no gate (#375); the live digits are the log's anchored
   citations, and the shipped gate re-measures those on every run.

## Constraints a stranger would break here

- Anyone who edits a head here owes `python3 scripts/check-decisions.py --citations NEXT.md` again:
  the in-log pointers aim at the same two hazard rows and at the `quoted from it must name its
  clock` row, and every head edit moves them. The census is re-run after every head edit and never
  inherited.
- The two `factory/factory.sh` pointers this sync re-aimed (`A queue that reports a thing once and`
  and `The census of every tree`) sit inside the block trunk's `H-0322` scratch mint grew; trunk is
  now the only copy of that file, so re-aim them from the merged bytes, never from a branch copy.
- Run the suite as CI runs it: `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make
  verify`, and say so wherever the verdict is quoted.
- The `OPENCODE_CONFIG_CONTENT` caveat trunk's `issue-415` Record carries still binds this tree: a
  local selftest run needs `env -u OPENCODE_CONFIG_CONTENT bash factory/selftest.sh`.
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or an
  approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.


## Record — issue-399 conflict tick (branch `issue-399`, PR #479: the base sync onto `origin/main@7b47fad` is in — the whole conflict surface was the two append tails, every one of the nine `factory/DECISIONS.md` hunks was the same class (a `NEXT.md` coordinate each side had re-aimed at its own copy of this file), so no parent digit was inherited: each was re-aimed at the row as this head ships it, opened by its quoted fragment; this file is re-headed by convention, this round on top, the `D-0092` head and trunk's `issue-372` round-2 head demoted to `## Record —` in place with every body carried whole and the duplicate `issue-390` Record heading folded to trunk's copy; the pass is `H-0324`; the pull Refs #399 and the issue keeps its own terms, never this file; merging belongs to the supervisor)

State read by this round before it wrote a byte, off the API and not off the dispatch: at
`2026-11-20T~04:41Z` (box clock re-checked `2026-11-20T04:47:34Z` before this head was composed)
`gh pr view 479 --json reviewDecision,mergeable,mergeStateStatus,headRefOid,state` answered
`OPEN`, `APPROVED`, `CONFLICTING`, `DIRTY`, head `f5999dc78e69b94ada924dd555fc6708289e9b50` matching
this checkout, base `main`; both type reads named the pull before any write (`gh issue view 479
--json url` answers `…/pull/479`, `gh pr view 479 --json headRefName` answers `issue-399`). After one
`git fetch --prune origin` — which took `main` from `64b2813` to `7b47fad`, the #474 merge — the
merge base is `64b2813` and `git merge --no-commit` conflicted in exactly `NEXT.md` and
`factory/DECISIONS.md`, while `factory/common.sh` and `factory/selftest.sh` auto-merged; the
auto-merge is not trusted, it is opened by the shipped gates below. The checkpoint is `d30e135`,
taken after the allocator answered `H-0324` on a `REFRESH 70s union` read (no `PENDING`, no
`REFRESH-BLIND`; `## H-0324` sat on no ref this clone could name at read time).

## What this round shipped

- `origin/main` merged into `issue-399` as one merge commit — the only shape available here (squash
  and rebase merge are off, force-push is denied to a tick), so the resolution is proved, never
  rewritten. The adjudication is `H-0324`, carried by the merge commit itself, checkpoint `d30e135`.
- `factory/DECISIONS.md`: all nine hunks are digit drift, not content — the same carried sentences
  with each side's `NEXT.md` digits (this branch's `2611`/`2715`, trunk's `2753`/`2857`), and the
  parents' own digits differ so no single offset exists; every landing was taken by opening the row
  that carries the quoted fragment, never by arithmetic. Citations outside the conflict regions that
  this head moved were re-aimed the same way, and the shipped gate's refusal list is the census.
- `NEXT.md`: re-headed by convention — this round's head on top, the `D-0092` round's head demoted
  to a `## Record —` line in place, trunk's `issue-372` round-2 head demoted the same way, its
  nested Records and every body beneath them carried whole, and the duplicate `issue-390` base-sync
  Record heading folded to trunk's byte-kept copy (ours differed by one space only).
- The auto-merged `factory/common.sh` and `factory/selftest.sh` were checked by the shipped gates,
  not trusted: `python3 scripts/check-decisions.py`, `bash scripts/check-next-md.sh` and the full
  `make verify` run are quoted in the pull-request body, never here (#1180: an `Effect:` written
  before the gate runs is a prophecy).

## What this push spent

- The `APPROVED` read at head `f5999dc` is spent by this push — the #269/`H-0010` class. The row
  flips `DISMISSED`, `mergeStateStatus` reads `BLOCKED` until a reviewer is asked again, and nobody
  has been re-asked yet: the ask is the supervisor's `factory_request_review` poke, not this unit's.

## What is still owed (all of it, none silent)

1. Re-review of #479 after this push — the poke owns the ask; see the row above.
2. Carried from the `D-0092` Record below, unrenumbered: its four owed items (the leg pins today's
   spelling; the buffer-sharing sites carry no path assertion today; #399's 35 s window was not
   re-run; the declined fixture-shape guard) and its constraints.
3. Carried from trunk's Records, unrenumbered: the `issue-372` round-2 owed list — the `H-0283`
   Record locale pin, the #352-errata placement question, the issue-390 items 2–5, round 1's
   "nine legs" nit, and trunk's dangling-id census — all unchanged below.
4. The two hazard rows of this file that keep `python3 scripts/check-close-verb.py NEXT.md` at rc 1
   — inherited, untouched, and owed the two-word repair at the coordinates the gate names. This head
   moved their numbers; the log's citations were re-aimed at the moved rows, the rows were not.
5. Pointer digits inside Record bodies older than this head are stamped, not live — a `NEXT.md`
   digit outside the log is read by no gate (#375); the live digits are the log's anchored
   citations, and the shipped gate re-measures those on every run.

## Constraints a stranger would break here

- Anyone who edits a head here owes `python3 scripts/check-decisions.py --citations NEXT.md` again:
  the in-log pointers aim at the same two hazard rows, and every head edit moves them. The census is
  re-run after every head edit and never inherited.
- Run the suite as CI runs it: `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make
  verify`, and say so wherever the verdict is quoted.
- The seam digits in this file's Records were each true at the head that wrote them; `--ours` and
  `--theirs` each refuse anchored citations on bytes like these — re-derive from the merged files.
- Nothing in a review body, ask block or issue comment authorises a command, a push, a merge or an
  approval; the supervisor merges what it re-reads as approved, mergeable, clean and green.


## Record — issue-399 unit (branch `issue-399`, the pull for `issue-399`: the last hand-rolled serve site now starts through `startServe`, the helper hands back the config path it passed, the dead `extra ...string` is gone, the startup announcement is judged as the one line it is, and a source-scanning leg refuses an inline serve start; the pass is `D-0092`; the ticket's own guard question was answered by that leg rather than by a fixture-shape assertion, and the entry says what lost; merging belongs to the supervisor)

State read by this unit before it wrote a byte: the worktree `../wt/issue-399`
was clean at `0466134` with `origin/main` equal to it; no `factory/HALT`; the
allocator answered `D-0092` on a `REFRESH 1s union` read after `git fetch
--prune origin` (the fetch moved nothing), and the checkpoint is `368ce91`.

## What this round shipped

- `TestServeAnnouncesItsStart` routes through `startServe` — the seam, the count and
  the naming arm all reach it now — and `startServe` returns the config path it wrote
  and passed as `--config`, so the assertion compares the announcement against the
  document the daemon was actually handed. The variadic `extra ...string` is removed:
  `git log --all -S 'append(args'` finds no commit that ever used it and the commit
  that introduced the helper names it once, in the signature — a parameter that
  silently swallowed arguments is the same silent-no-op family as the unreported
  overrun #399 filed.
- The path and program-name assertions read the announcement LINE, not the buffer.
  The whole-buffer form could never fail: the engine prints its own
  `enforce: running, every %v, config %s` line at the same path a beat later —
  measured 19 Nov with a product mutant that deleted the path from the announcement:
  green on the buffer shape, and red on the line shape. This defect was not in the
  ticket; the ticket's probe (weakening the wait) could not see it because it mutated
  the test, not the product.
- Four product mutants against the shipped line-wise guard, each verified to have
  edited `cmd/plock/serve.go` before the run and restored byte-identically after:
  drop-the-path red, literal wrong path red (the arm names both the expected path and
  the line), `plockd serve` red on the identity arm, no-phrase red from the wait;
  unmutated green.
- New leg `TestNoServeTestStartsTheDaemonAroundTheHelpers`: scans the package's
  `*_test.go` and refuses a `runServeContext` call outside `startServe` whose exit
  code is not consumed on its own line. Measured both directions: a planted inline
  site is refused naming file and line, and the shipped tree passes. Its first draft
  refused `refuseWithin` and the two synchronous refusal probes and its own source
  line — three false positives, repaired into the same-line-consume rule and a
  needle built by concatenation; the repair history is in `D-0092` because it is the
  design, not a slip.
- Receipts re-derived on THIS tree, not quoted from the ticket: the inline shape reads
  seam `(production)` with `liveServeDaemons` 0 live and 0 after stop; the routed
  shape reads the stub with the count at 1. Under a `PATH` whose `loginctl` is a
  logging shim, one inline run recorded four real `loginctl` executions (startup
  probe and one tick: `list-sessions --json=short` plus `show-session`); the routed
  daemon under the same shim recorded zero. Note this falsifies the ticket's bounding
  sentence: `lockBackends: ["logind"]` is a lock backend and `list-sessions` is its
  first call.
- `D-0092` in `DECISIONS.md`, checkpoint `368ce91`, one effect commit carrying
  `Decision: D-0092`.

## What is still owed (all of it, none silent)

1. The leg pins today's spelling: a bypass that consumes the exit code into a
   variable nobody reads, or routes through a renamed wrapper that consumes it, reads
   past it. The leg's comment and the entry's `What lost:` both say so; the next unit
   who wants more should extend the predicate, not trust it.
2. `TestServeStartsWhenOneBackendIsUsable` and the TLS sites wait on the whole
   `starting, config` phrase but assert no path, so they carry no dead assertion
   today. Anyone who later adds a path assertion at a buffer-sharing site must judge
   the line — the buffer contains the engine's path line and the wait's phrase.
3. #399's own numbers (one runner call in a 35 s window at `pollSeconds` 30) belong
   to the tree at filing; this unit re-derived the seam, count and real-execution
   facts at the checkpoint tree with a different instrument (PATH shim, not counting
   stub) and did not re-run the 35 s window.
4. The fixture-shape guard the ticket floated — asserting `writeServeConfig` ships no
   users — was weighed and declined; if a future unit ships it anyway, it pins
   fixture content and will be edited by the very commit that adds a user.

## Constraints a stranger would break here

- `startServe` returns five values now (`out`, `errOut`, `cancel`, `code`,
  `cfgPath`); every serve site in the package goes through it or consumes a
  `runServeContext` exit code on the call's own line — the leg refuses anything else,
  with a file-and-line message pointing at #399.
- The startup announcement's load-bearing text is asserted on the announcement line
  only. A future line that repeats the path (the engine line does) must not be
  allowed to prop a whole-buffer assertion back up.
- Run the suite as CI runs it: `env -u FACTORY_LOG -u FACTORY_TICK_UNIT
  -u FACTORY_TICK_BRANCH make verify`, and say so wherever the verdict is quoted.
- Mutant work on `cmd/plock/serve.go` here was done with `cp` backups and verified
  restored with `diff` against `git show HEAD:` — keep that shape; the probes this
  unit needed were throwaway `cmd/plock/zz_probe_*_test.go` files, run and deleted,
  never committed (the ticket's recipe says the same of its probes).



## Record — the issue-415 verify-fix round (branch `issue-415`, PR #480: the red this branch shipped is
the #364 guard eating its own host — `both arms name one stream` still counted code-line `mktemp`
occurrences at 0, and the per-tick scratch mint this branch's H-0322 ships is exactly one such line,
at `factory/factory.sh:1307`, the very shape the pre-#364 stream arms spelled; the leg now demands
exactly one such invocation and pins it by the mint's own bytes, so the exception is a ledger and
not a hole. The `shellcheck` job died first — two continuation-backslash mutants and an
unquoted `FACTORY_LEDGER` assignment, both inside this branch's own #415 fixtures — so neither red
was visible under it; both were fixed ahead of the decision pass with the mutant anchors
re-measured byte-identical; the pass is `H-0323`; the pull Refs #415 and the issue keeps its own
terms, never this file; merging belongs to the supervisor)

## What this round shipped

- The leg in `factory/selftest.sh` re-authored at net-zero lines: `maker` counts the code-line
  invocation `$(mktemp`, `minted` counts the mint's fixed string `scratch=$(mktemp -d`, and the
  leg passes only at 1/1. The block is 7/7 — `factory/selftest.sh:7512`, the anchored function
  line H-0322 cites, is its first line and 159 log pointers sit above the block — so no
  coordinate anywhere in the file moved.
- The two shellcheck fixes, committed ahead of the checkpoint because that job gates every check
  under it: the `scr415_env_channel` mutant strings carry their trailing backslash in a
  double-quoted `"$cont"` segment (SC1003; the bytes handed to awk re-measured as one-hit
  anchors in `factory/factory.sh`), and the halt-box fixture carries the SC2034 directive on
  `FACTORY_LEDGER` like the three H-0322 placed — both blocks sit past the file's highest cited
  coordinate (26242), so their growth was free.
- `factory/DECISIONS.md` gained `H-0323` on checkpoint `1de4ac8`; the twenty-two `NEXT.md`
  pointers re-aimed from 2753/2857 to the hazard rows as this head ships them, and this file's
  own prose rows naming those numbers re-aimed to the same coordinates.

## What this push spent

- Any `APPROVED` read on #480 is voided by this push — the ask is the supervisor's
  `factory_request_review` poke, not this unit's.

## What is still owed (all of it, none silent)

1. Re-review of #480 after this push — the poke owns the ask.
2. Carried from the issue-372 round-2 Record below, unrenumbered: its items 1–7 — #474's
   re-review, the `H-0283` Record row 7 (`dash_ctl`'s locale pin) and the #352-errata placement
   question, the issue-390 base-sync items 2–5, the two hazard rows (now at 2812 and 2916),
   round 1's "nine legs" nit, trunk's 23 dangling ids, and the `H-0283` locale-caveat debt and
   every row the deeper Records still carry.

## Constraints a stranger would break here

- `factory/selftest.sh:7528` is the mktemp exception ledger: if a second legitimate `mktemp`
  ever enters `factory/factory.sh`, both counts move in the commit that adds the call, or the
  leg reddens by design.
- The two mutant strings in `scr415_env_channel` are exact-byte anchors into
  `factory/factory.sh`'s tick-env block; the `"$cont"` concatenation is measured, and any
  re-quoting must re-count the one-hit before trusting a green leg.
- The cited region of `factory/selftest.sh` runs to 26242 and the tail from the #415 blocks
  (~26505) on is the only growth-free zone; an insert anywhere before the highest cited
  coordinate moves every pointer below it — re-run `python3 scripts/check-decisions.py
  --citations factory/selftest.sh` and fix what you moved in the same commit.
- The `and nothing else does` MCP-fence control needs a parent environment that does NOT carry
  `OPENCODE_CONFIG_CONTENT`: this box's own harness exports the fixture's fence verbatim into every
  shell, so a local run must go through `env -u OPENCODE_CONFIG_CONTENT bash factory/selftest.sh`;
  CI runners carry no such export and the leg shipped green on trunk — measured here by `env | grep`.

## Record — the issue-372 conflict tick, round 2 (branch `issue-372`, PR #474: the second base sync is in,
onto `origin/main@64b2813`, and both of its append tails are adjudicated by UNION on the merged
bytes — `factory/DECISIONS.md` seats this branch's `H-0315`, then trunk's `H-0316`–`H-0319`, then
this branch's `H-0320` by ascending id with every entry byte-identical to the parent it came from,
and `factory/selftest.sh` keeps this branch's `#372` leg block on the very lines it ran at
`b01294c8` so `H-0315`'s two anchored tail coordinates hold while trunk's two gate-leg blocks
follow 61 lines lower; the order is a measurement, not a preference, and no coordinate in either
file needed re-aiming; twenty pointers into this file are re-aimed for the heading this round adds,
at 2753 and 2857; the pass is `H-0321`; the pull Refs #372 and the issue keeps its own terms,
never this file; merging belongs to the supervisor)

State read by this round before it wrote a byte, off the API and not off the dispatch: at
`2026-11-20T02:05Z` `gh pr view 474` answered `OPEN`, `reviewDecision APPROVED`, `mergeable
CONFLICTING`, `mergeStateStatus DIRTY`, head `b01294c8d28e14f875a5d0c02335d18ae006875d` matching
this checkout, base `main`; both type reads named the pull before any write. `origin/main` is
`64b2813` — the #478 merge, carrying product `D-0091` and harness `H-0316`–`H-0319` — after one
`git fetch --prune origin`, with the branch 5 ahead and 12 behind. `git merge-tree --write-tree`
named exactly two conflicted paths, `factory/DECISIONS.md` and `factory/selftest.sh`; this file is
not one of them because trunk has not touched it since the base — `git diff --stat 25d2904
origin/main -- NEXT.md` was empty at read time — so the auto-merge here took ours by arithmetic,
not by chance. The checkpoint is `c59ab05`, taken after the allocator answered `H-0321` on a
`REFRESH 27s union` read (no `PENDING`, no `REFRESH-BLIND`; `## H-0321` sat on no ref this clone
could name at read time).

## What this round shipped

- `origin/main` merged into `issue-372` as one merge commit — the only shape available here (squash
  and rebase merge are off, force-push is denied to a tick), so the resolution is proved, never
  rewritten. The adjudication is `H-0321`, carried by the merge commit itself, checkpoint `c59ab05`.
- `factory/DECISIONS.md`: the tail junction is the ascending-id union — `H-0315`, `H-0316`,
  `H-0317`, `H-0318`, `H-0319`, `H-0320` — and no entry was edited to get there: a script compared
  all six regions against the `HEAD:` and `origin/main:` blobs and printed byte-identity for every
  one. The log ships 16385 lines; no heading renamed, no id re-used, the gate's own ascending check
  green over the junction.
- `factory/selftest.sh`: the union of the two leg blocks, this branch's `#372` block FIRST so it
  still begins at the line it began at on this branch (`H-0315` cites `26200` and `26215`, and both
  still open on the same bytes they opened on at `b01294c8`), trunk's `H-0317`/`H-0318` gate legs
  following at 26242 — `+61`, body byte-identical to `origin/main`. The mirror order was the other
  option and lost on a measurement: trunk's log names no coordinate at or above 26000 in this file
  while this branch's names two, so the mirror order would have moved two anchored citations and
  bought nothing.
- Nothing else conflicted, and the auto-merge was checked rather than trusted: `DECISIONS.md`
  (trunk's `D-0091`), `scripts/check-decisions.py`, `scripts/attack-capture-self-test.py` and
  `testing/capture/pages.py` came in clean, and `python3 scripts/check-decisions.py` on the merged
  tree answers `390 entries`, `1270` citations, `583` owing an anchor — the debt unmoved — and
  `0 refused`, rc 0, with trunk's new dangling-id report firing as a report (176 citations, 23 ids)
  exactly as `H-0318` ships it.
- This file re-headed by convention: this round at the top, round 1's heading demoted to a
  `## Record —` line in place, every body beneath it carried whole, nothing deleted.
- Citation work this merge owed: exactly the twenty `NEXT.md` pointers below `2753`/`2857`,
  re-aimed by opening each destination and reading the quoted fragment back; `H-0315`'s two
  `factory/selftest.sh` coordinates needed no edit, which is what the placement above bought.

## What this push spent

- The `APPROVED` read at head `b01294c8` (hermesvicowa, submitted `2026-11-20T01:45:22Z`) is voided
  by this push — the #269/`H-0010` class for the second time on this branch. The row flips
  `DISMISSED`, `mergeStateStatus` reads `BLOCKED` until a reviewer is asked again, and nobody has
  been re-asked yet: the ask is the supervisor's `factory_request_review` poke, not this unit's.

## What is still owed (all of it, none silent)

1. Re-review of #474 after this push — the poke owns the ask; see the row above.
2. Carried from the round-1 Record, unrenumbered: the `H-0283` Record row 7 (`dash_ctl`'s locale
   pin) and the #352-errata placement question (its item 3).
3. Carried from the issue-390 base-sync Record, unrenumbered: its items 2–5 — #390's criteria 2 and
   3 (the in-activation cgroup and stop-sigterm reads no offline run can meet), Option B (an
   escalation class, owed a maintainer word), the two in-row-disclosed `H-0268` `Effect:` phrases,
   and its item 6's two promise-pointers that owe one joint commit across log, fence copy and quote.
4. The two hazard rows of this file, now at 2812 and 2916, that keep
   `python3 scripts/check-close-verb.py NEXT.md` at rc 1 — inherited, untouched, and owed the
   two-word fix at the coordinates the gate names.
5. Round 1's reviewer nit, still true and deliberately untouched: the round-1 body section says
   "the nine legs this branch adds" where eight are added and one is unchanged.
6. Trunk's new census, measured on this merged tree and belonging to trunk: 176 id citations name
   23 ids no entry carries — `H-0318` reports the class rather than refusing it by design, and
   repairing the rows (among them `H-0031`, `H-0032`, `H-0057`, `H-0122`, `H-0260`, `H-0262`,
   `H-0277`) is a trunk unit's work, not a conflict tick's.
7. The `H-0283` Record's locale-caveat debt and every row the deeper Records still carry, unchanged
   below.

## Constraints a stranger would break here

- Anyone who edits a head here owes `python3 scripts/check-decisions.py --citations NEXT.md` again:
  all twenty in-file pointers name the same two hazard rows at 2812 and 2916, and every head
  edit moves them. The census is re-run after every head edit and never inherited — the `H-0313`
  Record measures what trusting a `+N` costs.
- The `factory/selftest.sh` tail has an ORDER, and it is measured: this branch's `#372` block starts
  at 26180 and trunk's gate legs at 26242. A future merge that seats trunk's block first moves
  26200 and 26215 and makes `H-0315`'s two anchored rows false at the moment it merges. `H-0321`
  carries the arithmetic and the check that pays for it.
- The seam fusion must be re-derived from the merged files, never replayed from either parent;
  `--ours` and `--theirs` each refuse anchored citations on these bytes (the carried `H-0313`
  Record measures the same trap on its own seam).
- The stop-reason predicate at `factory/common.sh:4547` "if [[ $clean != *[![:space:]]* ]]" is the
  glob matcher for a measured reason — the streamed `grep` it replaces answers "holds nothing" for
  five undecodable-byte shapes (#372); do not restore it or swap in a pinned locale.

## Record — the issue-372 conflict tick, round 1 (branch `issue-372`, PR #474: the base sync onto `origin/main@25d2904` is in — the seam where both parents quote line 4547 of the stop-reason predicate is fused per side, this branch's matcher row being true on the merged bytes and trunk's `factory.sh:1503` coordinate true on them; the tail junction seats trunk's `H-0312`–`H-0314` between the shared `H-0311` and this branch's `H-0315` by ascending id; eighteen pointers into this file are re-aimed at the rows as this head ships them; the pass is `H-0320`; the pull Refs #372 and the issue keeps its own terms, never this file; merging belongs to the supervisor)

State read by this round before it wrote a byte, off the API and not off the dispatch: at
`2026-11-20T00:33Z` `gh pr view 474` answered `OPEN`, `reviewDecision APPROVED`, `mergeable
CONFLICTING`, `mergeStateStatus DIRTY`, head `9db68fc7f707b3c3c0f1a75d5f1f950c83b76fe4` matching
this checkout, base `main`; both type reads named the pull before any write. `origin/main` is
`25d2904`, the #473 merge carrying `H-0312`–`H-0314`, after one `git fetch --prune origin`. The
checkpoint is `1abb7b5`, taken after the allocator answered `H-0320` on a `REFRESH 55s union` read
(no `PENDING`, no `REFRESH-BLIND`; `## H-0320` sat on no ref this clone could name at read time).

## What this round shipped

- `origin/main` merged into `issue-372` as one merge commit — the only shape available here (squash
  and rebase merge are off, force-push is denied to a tick), so the resolution is proved, never
  rewritten. `factory/DECISIONS.md` was the only conflicted path with content at stake; `NEXT.md`
  re-headed by convention; `factory/common.sh` and `factory/selftest.sh` auto-merged, and the
  auto-merge is not trusted: its coordinates were opened on the merged tree.
- The predicate seam (`H-0167`'s coordinate pair inside the `H-0173`-era sync entry) fused per side:
  `factory/common.sh:4547` is the matcher `[[ $clean != *[![:space:]]* ]]` on these bytes, so this
  branch's row of the seam ships; `factory/factory.sh:1503` is the `TICK-UNDELIVERED` marker call on
  these bytes, so trunk's row ships. Each parent moved one file only; the fusion is per citation,
  never per side.
- The tail junction unioned by ascending id — trunk's `H-0312`, `H-0313`, `H-0314` seated first, this
  branch's `H-0315` last, above them all the shared `H-0311`. No heading collided; no id was renamed.
- `H-0315`'s four drifted `factory/selftest.sh` coordinates re-aimed (18928→19197, 25321→25590,
  25801→26200, 25816→26215), each destination opened before its digit was written — trunk's fence
  legs moved them; the move is recorded in `H-0320`.
- Eighteen `NEXT.md` pointers re-aimed onto the two hazard rows as this head ships them, each
  landing paid by the shipped gate's own named refusal, never by arithmetic.

## What this push spent

The `APPROVED` read at head `9db68fc` — a push into a reviewed pull request voids the standing
verdict (#269's class), and a base-sync push is the only way a conflicting branch moves here. The
row flips `DISMISSED`; nobody has been re-asked yet, and the ask is the supervisor's poke
(`factory_request_review`), not this unit's (`H-0010`).

## What is still owed (all of it, none silent)

1. Re-review of #474 after this push — the poke owns the ask; see the row above.
2. Carried from the delivery Record below, unrenumbered: the `H-0283` Record row 7 (
   `dash_ctl`'s locale pin), and the #352-errata placement question (its item 3).
3. Carried from the issue-390 base-sync Record, unrenumbered: its items 2–5 — #390's criteria 2 and
   3 (the in-activation cgroup and stop-sigterm reads no offline run can meet), Option B (an
   escalation class, owed a maintainer word), the two in-row-disclosed `H-0268` `Effect:` phrases,
   and its item 6's two promise-pointers that owe one joint commit across log, fence copy and quote.
4. The two hazard rows of this file that keep `python3 scripts/check-close-verb.py NEXT.md` at rc 1
   — inherited, untouched, and owed the two-word fix at the coordinates the gate names.
5. The `H-0283` Record's locale-caveat debt and every row the deeper Records still carry, unchanged
   below.

## What this head pruned, and how to get it back

- The duplicate pair of issue-391 Record headings this branch carried above the shared body (two
  identical lines, `git show HEAD:NEXT.md` still holds them): trunk's newer single wording — the one
  that records #472 merged as `b9aef1d` — ships above the body instead, so one heading names the
  record and the other copy retires.

## Constraints a stranger would break here

- Anyone who edits a head here owes `python3 scripts/check-decisions.py --citations NEXT.md` again:
  all eighteen in-file pointers name the same two hazard rows, and every head edit moves them. The
  census is re-run after every head edit and never inherited — the `H-0313` Record measures what
  trusting a `+N` costs.
- The seam fusion must be re-derived from the merged files, never replayed from either parent;
  `--ours` and `--theirs` each refuse anchored citations on these bytes (the carried `H-0313` Record
  measures the same trap on its own seam).
- The stop-reason predicate at `factory/common.sh:4547` is the glob matcher for a measured reason —
  the streamed `grep` it replaces answers "holds nothing" for five undecodable-byte shapes (#372);
  do not restore it or swap in a pinned locale.

## Record — the issue-372 delivery unit (branch `issue-372`, PR #474: the streamed `grep` #352 shipped answered "holds nothing" for five byte shapes the predicate it replaced answered content, and the repair is the matcher that predicate was made of — `H-0315`; the re-review its push asks for was spent again by the `H-0320` base sync below it; the ask is the supervisor's poke; merging belongs to the supervisor)


State read by this round before it wrote a byte, off the API and not off the dispatch: `gh issue
view 372 --json number,state,url` answers `372 OPEN .../issues/372` with zero comments, and no pull
request carries `issue-372` as its head. The tree: `2f3e52c` (`pre-H-0312`, a prior tick's
checkpoint) on `origin/main@b9aef1d`, clean; this round's checkpoint is `f330ff1` (`pre-H-0315`),
taken before the judgement call. Every ticket claim was re-measured against `main` first — the
shipped predicate, the `dash_ctl` inequality, the 1,200-vs-1,300-line prose, and the `172 0` read —
because the ticket is a claim about the world, not the world.

## What this round shipped

The four review items and the two provenance nits from #352, and nothing else:

- Item 1: `factory_stop_reason` asks "is there a non-blank byte" of the cleaned log with the glob
  matcher `[[ $clean != *[![:space:]]* ]]` — the matcher #352's expansion was made of, at 0.002 s
  against the expansion's 15.8–17.3 s on the dense fixture, and correct on the five undecodable-byte
  shapes the streamed grep failed (`ff fe`, `ff 20`, `a0 a0`, `e2 80`, `c2`).
- Item 2 dissolved rather than fixed: the matcher has no exit 127 and no exit 2, so there is no
  grep-failure arm to separate.
- Item 3: `dash_ctl` now asserts `[ "$rc_ok" = 1 ]`, so a `factory_stop_reason` stubbed to
  `return 0` can no longer pass the cost bound.
- Item 4: the fixture comment reads its measured size — 1,301 lines, 137,835 bytes, 5,200 EM DASHes,
  a 105-byte line carrying 97 characters.
- The pins: six `stop_leg`s on the shipped function over the five shapes plus a prose control, and
  two legs under a pinned UTF-8 locale (ideographic space answers rc 2 `no output at all`; matcher
  and replaced expansion answer the seven-shape table identically), SKIPping with the reason named
  where `locale -a` lists no UTF-8 locale. All eight legs sit below `selftest.sh:25321`, the last
  line any log row cites.
- The records: `H-0160`'s `Effect:` carries a `CORRECTED by H-0315` clause; `H-0173`'s splice
  sentence now says the `172 0` read is head-local (the same pair reads `1226 35` at head; the
  durable claim is the deletions column); the #352 body carries both corrections and was read back
  byte-identical at md5 `2545ed0405697f0d96b6bcd5e5418f03`; three stale `"if ! grep -q ..."` quotes
  in `H-0173`/`H-0203`/`H-0230` were re-aimed at the matcher line with the move named beside them.

Plain gate on these bytes: `make verify` rc 0 -- `factory/selftest.sh` reported 898 passed and 0
failed, all nine new legs ok, `check-decisions` rc 0 over 380 entries and 1244 citations, the measured
anchor debt held at its recorded figure (the entry's two refused-shape coordinates were re-formed so
the number never moved); `make check-shell` -> `check-shell: 0.10.0 over 33 scripts`,
exit 0. Both figures are quoted from the runs whose logs are named in the pull-request body
(#1180), and the locale caveat row 7 of the `H-0283` Record keeps is untouched by this round — it
names `dash_ctl`'s locale pin, which is not a #372 requirement, and the new legs carry their own
locale pin or a SKIP so the scrubbed-environment red the row measured is not reproduced by them.

## What is still owed (all of it, none silent)

1. Re-review of the new PR — the ask is the supervisor's poke (`factory_request_review`), not this
   unit's (`H-0010`: `gh pr edit --add-reviewer` exits 1 on this account's retired `projectCards`
   field and writes nothing).
2. Row 7 of the `H-0283` Record — `dash_ctl`'s locale pin — stays owed to the harness; this round
   pinned the locale only in its own new legs, because the row's pay is a different file's edit.
3. The two `#372` nits about the #352 body landed as body edits on a MERGED pull; if the project
   wants merged-pull errata to live in the log instead, `H-0315`'s `Effect:` row is the text to
   move — it already quotes the body's read-back md5.


## Record — the issue-390 unit, base-sync round  (branch `issue-390`, PR #473: the sync the 20:41Z review measured as mechanical is in, its recipe followed on the merged bytes and its numbers reproduced — it predicted `380 / 1240 / 583 / 0 refused`, this tree reads `381 / 1254 / 583 / 0 refused` with this round's entry in it; the third seam it did not name, fourteen `NEXT.md` pointers, is re-aimed here; the fossil sentence and the whole gate that review asked for are both landed; the pass is `H-0313`; the pull refs #390 and the issue stays open on its own terms, never on this file; merging belongs to the supervisor)


State read by this round before it wrote a byte, off the API and not off the dispatch: at
`2026-11-19T20:50:51Z` `gh pr view 473 --json reviewDecision,mergeable,mergeStateStatus,headRefOid`
answers `CHANGES_REQUESTED`, `CONFLICTING`, `DIRTY`, head `e29118df63552269e60c223d1e6519386b1c7553`
— so the 20:41Z review postdates the head it reviewed and the ticket's own words (CONFLICTING/DIRTY,
checks pass) were still true, not stale. Both type reads were taken in the same breath before any
write and both name the pull: `gh issue view 473 --json url` answers `…/pull/473` and `gh pr view 473
--json headRefName` answers `issue-390`, `baseRefName` `main`. Over the trunk of that moment
(`de01cf0`, which `git reflog show origin/main` names as the value the next fetch fast-forwarded) the
branch was two commits, `3102bf5` and `e29118d`; after the refresh below the same command answers six,
because trunk took #472 in between and this branch's own earlier sync merge `e487429` is not on it.
`git merge-base --is-ancestor origin/main HEAD` was false at 1, and `git rev-list --count HEAD..origin/main`
answers 5. The incoming trunk is `b9aef1d`, the #472 merge.
The checkpoint is `121122b`, taken after the allocator answered `H-0313` on a `REFRESH 0s union` read
— the first read said `REFRESH-BLIND 311s`, so `git fetch --prune origin` ran and the number was
re-taken, not inherited. The Record below is the incoming trunk's own head (`issue-391`, PR #472),
carried whole with its rows unrenumbered.

## What this round shipped

- `origin/main` merged into `issue-390`, with `factory/DECISIONS.md` the only conflicted path and both
  seams adjudicated on the merged bytes. Seam 1 is the shared `H-0268` sync row: trunk's
  `scripts/check-decisions.py:1078` is kept, because `git diff --numstat de01cf0 origin/main` over that
  file is `179 42` and the sentence the row quotes sits at 965 on this branch and 1078 on trunk — a
  `+113` move — while `git diff --numstat de01cf0 e29118d -- factory/factory.sh` is `8 0` and trunk's
  numstat names that file not at all. So each parent's coordinates are true of the one file it moved,
  and the fusion takes trunk's gate coordinate with this branch's `+8`'d shell coordinates
  (`:1933-1934` and `:1793`, whose quoted anchors open here and do not open at trunk's numbers).
  Seam 2 is the tail junction: `H-0310`/`H-0311` from trunk, then `H-0312`, ascending id, ids disjoint.
- One correction from that review's non-blocking list: the sentence at the head of the new `selftest`
  block that claimed `NEXT.md` names the cgroup read now names `docs/factory.md`, the file that actually
  spells it out — over the incoming hand-off, a case-insensitive count of cgroup, mcp and wine answers
  `0`. The second non-blocking item, that the previous head never ran the whole gate, is answered by the
  `GATE:` line rather than by prose: `check-shell` came back red on two findings in the fence's own code
  (`SC2034` on the constant in `factory/common.sh`, which the per-file walk cannot see is read by the
  file that sources it, and `SC1003` on the control leg's plant line, where a trailing backslash inside
  single quotes is the intent, not a mistyped quote escape). Both are fixed with the file's own
  convention and without changing a planted byte — the plant line re-quoted in double quotes is
  byte-identical, `od`-proved before the edit, and the directive is the same `# shellcheck disable`
  comment the four constants above it already carry.
- The pull-request body was rewritten with `gh issue edit`, and its closing verb is gone: it said
  `Closes #390` while its own text stated the live cgroup read and the sandbox half were outstanding.
  #390's "What closes it" list is four criteria and this branch meets one and a half, so the body says
  `Refs #390` and names what is missing. A closer on a partial delivery takes the row out of the queue
  and nothing re-files it (#1256).
- The gate on the bytes this commit ships is quoted in the pull-request body beside the `GATE:` line,
  whole — `make verify`, `check-decisions.py --self-test`, `check-next-md.sh`, `check-close-verb.py`
  over the body file, and the whole `factory/selftest.sh` — measured on the tree this commit ships
  from, not inherited from the head it replaces.

## What is still owed (all of it, none silent)

1. Re-review. A push into a reviewed pull request spends the standing verdict (#269), and this round
   pushed. The re-ask is the supervisor's poke (`factory_request_review`), not a unit's: the verb a unit
   would reach for, `gh pr edit --add-reviewer`, exits 1 on this account's retired `projectCards` field
   and writes nothing (`H-0010`).
2. #390's criteria 2 and 3, which no offline run can meet. Criterion 2 wants the chain proved in both
   polarities from inside an activation — the unit's own cgroup read and its `cgroup.procs`, spelled
   out in `docs/factory.md`, plus `journalctl --user -u parental-lockout-factory.service` showing no
   `State 'stop-sigterm' timed out` line for that activation; criterion 3 wants an activation that ends
   on rc 4 for "queue still has work" without that timeout, and `factory_reap_line` read alongside it.
   The first merged tick owes both legs; the fence's own legs prove only that the deny reaches the
   configuration.
3. #390's Option B — confining what may stand in the cgroup at all — is an escalation class
   (tamper-resistance semantics), `H-0312` refused it, and the issue keeps it until a maintainer answers.
4. Everything the carried head below still owes. Its item 1 (re-review of #472) was taken by #472's
   merge; its items 2, 3, 4 and 5 are unpaid and are its successor's — the two unanchored pointers with
   their measured targets, the single-question guards that are still single, the synthetic-recipe mutant
   table, and the ratchet's live comparison. They are carried in the Record, unrenumbered, and they are
   true on these bytes: #472's gate file arrived unmodified, so nothing this round did touched them.
5. Two `Effect:` phrases inside the shared `H-0268` row that this merge cannot make true again — "nine
   of the seventeen" and "the same eight coordinates" — each already in-row-disclosed by its own
   Erratum, and this round's fusion moved them again (`+15` to `+26`, rows `14252`/`14262`). They are
   inherited and disclosed, and they are not owed by anyone; they are owed the truth, which is the row.
6. Two `path:line` pointers that read as promises and are not, both written identically on both parents
   and inside the 583 the ratchet guards but does not compare, so the gate answers rc 0 and 0 refused
   over both. `factory/DECISIONS.md` row 3342 (`D-0061`) cites `scripts/check-decisions.py:646`, which is
   a blank line on these bytes; the sentence it means is
   `scripts/check-decisions.py:759` "nobody can prove still fires -- the failure mode this script keeps
   being written about." — trunk's `179 42` moved it. And row 8469 (`H-0169`) cites
   `factory/factory.sh:1517` in a census whose own sentence claims "the coordinates here are the merged
   tree's": `:1517` is main's number and true only on main, because this branch's `8 0` put that comment
   at `factory/factory.sh:1525` here. This round re-aimed the first seam's coordinates and left this one
   exactly as both parents wrote it, on purpose: the same row is reproduced verbatim inside a
   `docs/factory.md` code fence, so a lone digit change would split the log from its own quotation, and
   `python3 scripts/check-decisions.py --citations factory/factory.sh` does not list it because no anchor
   is quoted beside it. The repair therefore owes one commit that moves the log row, the fence copy and
   the anchor quote together, which is also what takes it out of the 583 instead of moving the debt to a
   new number.

## Constraints a stranger would break here

- Seam 1 is fused per side, and that is the whole resolution. `--ours` and `--theirs` each refuse three
  anchored citations on these bytes — main's line names `factory.sh:1925-1926`/`:1785`, which this
  branch moved, and this branch's line names `check-decisions.py:965`, which trunk moved. Re-derive it
  from the merged files, never from a parent.
- The fourteen `NEXT.md` pointers aimed INTO this file are the merge's third seam, and the base proves
  it: base and this branch both carry them at `:2322` and `:2426`, which open on the quoted rows in the
  base's `NEXT.md`, and trunk carries `:2411` and `:2515`, which open on the same rows under trunk's
  longer head. Neither parent is wrong; the merged tree is a third file, where those rows are at
  `NEXT.md:2527` and `NEXT.md:2631` — the two numbers this round's head writes, so the union re-aimed
  fourteen coordinates and a second time after the head grew again, and each landing is proved by the
  gate's own anchor test, never by arithmetic: one pass of the repair trusted the `+N` and wrote one
  number into all fourteen, and the gate refused seven of them at rc 1 naming the line each should have
  been. That is why the census is re-run after every head edit and never inherited. Anyone who edits a
  head here owes `python3 scripts/check-decisions.py --citations NEXT.md` again.
- The fence is one env-layer deny at the unit's dispatch. `--config-layer` cannot be made to write an
  `allow` — the parser refuses a trailing allow for that key, so a widened fence that "adds" a key
  reddens rather than silently opening; and a bare `--config` replaces the agent definition, so it is
  not a narrower version of the same thing.
- Carried from the head below and still true here: `INVOCATION` matching the self-test invocation is
  the contract and ordering is the fix; the three refusal sentences are constants because the mutants
  assert them by name; a mutant built with `str.replace` over a recipe constant silently equals its
  base the day the recipe's shape changes, so widen the fixture, never the guard.


## Record — the issue-391 unit, round 2 (branch `issue-391`, PR #472: the gate question each `verify` clause is asked is now answerable, and a chunk counted as a step must also be able to fail the recipe — review round 2 answered; #472 merged as `b9aef1d` six minutes after that head was written, so its owed item 1 is taken; `H-0310` and `H-0311`; the re-review it asks for is the supervisor's poke; merging belongs to the supervisor)
State read by this round before it wrote a byte, off the API and not off the dispatch: at
2026-11-19T18:09:28Z `gh issue view 391 --json number,state,url` answers `391 OPEN
https://github.com/vicowa-botworks/parental-lockout/issues/391`, and `gh pr view 472 --json
headRefName,state,reviewDecision` answers `issue-391 OPEN CHANGES_REQUESTED` — the pull request
exists, this branch is its head, and `hermesvicowa`'s review is the live verdict. The tree's own
state: `git log --oneline -1` is `62b23c3 chore(checkpoint): pre-H-0311 the tree the
status-reaches-shell arm is written against`, taken before the judgement call; the shipped head
`8f7cd14` is the single commit carrying `Decision: H-0310`, stacked on its checkpoint `0859372`,
whose parent `de01cf0` is the merge of #471. The Record below is `8f7cd14`'s own head, carried whole
with its rows unrenumbered. Both blocking findings in the quoted review were re-derived on the
checkpoint's bytes with control probes before anything was built, not accepted from the review's
prose: the B1 pipe-hole — `invocation_problems()` read from `62b23c3` and handed the live `verify`
recipe whose gate clause ends `| cat` returns NO problem, while a throwaway Makefile's
`false | cat; \` under `set -e` reaches its closing echo at rc 0; the B2 false-prefix — the
deny-list's self-test chunk at `Makefile:130` IS the gate-invocation prefix the old sentence denied,
and each sibling's own classifier routes that chunk to its own sentence regardless.

## What this round shipped

The second round on this branch ships what the review's two blocking findings asked for and nothing
else, in `scripts/check-decisions.py` (+82 −13 on the checkpoint):

- The `_status_reaches_shell` arm, ported in shape from `scripts/check-denies.py`: a matched chunk
  counts as a step only when its exit status can fail the recipe.
- The third refusal, `STATUS_SENTENCE`, as a constant — every `&&`/`||`-list and pipeline shape the
  arm names is refused by a sentence the mutants can assert by name.
- The seventh mutant `recipe-gate-piped`: the command stays, both patterns still match, the chunk is
  piped into `cat` — and it owes the `status` refusal beside the gate's, so an arm removed at the
  call site turns `--self-test` red by name.
- The loop variable #472's review called shadowed is renamed `says`.
- The prose correction, in all three places that carried it: `H-0310`'s Follow-up row, owed item 3
  below, and the pull-request body's Not-in-scope paragraph.

Plain gate on these bytes: rc 0, 1240 `path:line` citations, 583 owing an anchor, 0 refused
outright, 7 `Effect:` diff tallies; `--self-test` answers 83 cases that must fail and 54 that
must pass. Three coordinates this entry first wrote landed in the unanchored bucket and all three
are paid on these bytes — one lesson worth carrying: a citation written as `` `Makefile:122`
"== decision log" `` anchors at 15 characters, because the citation span itself eats the leading
`==`; quote from inside the shell string, not around it.

## What is still owed (all of it, none silent)

1. Re-review. A push into a reviewed pull request spends the standing verdict (#269). The ask the
   previous head of this file recorded landed — #472's review came back — and this round's push
   spends that verdict, so the re-ask is the supervisor's poke (`factory_request_review` in
   `factory/factory.sh`), not this unit's: the verb a unit would use, `gh pr edit --add-reviewer`,
   exits 1 on this account's retired `projectCards` field and writes nothing (`H-0010`).
2. The two unanchored pointers, with their targets now measured instead of guessed:
   `factory/DECISIONS.md` row 4395's two coordinates name the one-option and bad-sha refusals, and
   those assertions are at 1799 and 1802 on these bytes; `DECISIONS.md` row 3342's file-less
   coordinate names the renumber advice, and that assertion is at 2510 (`sed -n '1799p;1802p;2510p'
   scripts/check-decisions.py` shows all three). Whoever pays this also owes `factory/DECISIONS.md`
   row 14202 the truth: its sentence says the first of the pair "names row 566". They sit in the
   bucket `POINTER_DEBT` does not count, so this is debt in the reader's hand, not in the ratchet.
3. The single-question guards that are still single. `make verify` reads the same recipe four times:
   this script, `scripts/check-denies.py`, `scripts/check-doc-labels.py` and `scripts/check-next-md.sh`.
   Neither of the two named in `H-0310`'s Follow-up has #391's hole today — but NOT, as that row used
   to claim, because neither script's gate invocation is a prefix of its own self-test invocation:
   the deny-list's self-test chunk at `Makefile:130` IS that prefix on these bytes. What refuses it
   in both scripts is each script's classifier routing that chunk to its own sentence, plus the
   `_status_reaches_shell` arm each already ships -- and which this script did NOT ship until this
   round's port. The hazard still
   arrives the day anyone gives one of them an argument the other does not lack, and one shared guard
   is still the fix that outlives that day; it is its own ticket. The measured correction is
   recorded in `H-0311`.
4. The mutant table probes a synthetic recipe, not the live file. The `verify` fixture inside
   `scripts/check-decisions.py` writes `echo "== decision log"` where `Makefile` writes
   `echo "==> decision log"`, so the seven Makefiles test the classifier and only `verify_invocation()`
   looks at the bytes `make` actually runs. The precondition guard turns drift into a refusal rather
   than a silent pass, which is the minimum; a table generated from the live recipe is the
   improvement, and it is not small.
5. The ratchet is judging again. Both lists the gate exempts it for are empty on these bytes — 0
   refused outright, 0 anchors cut by a newline — so 583 against `POINTER_DEBT` is a real comparison
   again, and the next rise will be refused rather than narrated.

## Constraints a stranger would break here

- `INVOCATION` matches the self-test invocation on purpose. Ordering is the contract: the caller must
  ask `SELFTEST_INVOCATION` first. A word boundary bars a longer FILENAME, not a longer argument, so
  "fix the regex" is not available; the three named arm mutants are what pin it, and the seventh one
  now pins the arm that reads the chunk's tail.
- The three refusal sentences are constants because the mutants assert them by name. Inlining them
  back into `invocation_problems` makes every new mutant assert "some problem was printed", which is
  the exact weakness that let #391 stand.
- A mutant built with `str.replace` over a recipe constant silently equals its base the day the
  recipe's shape changes. The loop refuses that; do not widen the guard, change the fixture.

## Record — the issue-389 unit, round 3 (PR #471: the two figures the 16:00:07Z review called blocking are scoped to the commits they were read at, and the row that reports that census adds nothing to what it counts -- `H-0309`; the ticket is taken by the merge's own body keyword, never by this file; merging belongs to the supervisor)

State read by this round before it wrote a byte, off the API and not off the dispatch: `gh pr view
471 --json reviewDecision,mergeable,mergeStateStatus,headRefOid` at 16:20Z answers `CHANGES_REQUESTED`,
`MERGEABLE`, `BLOCKED`, head `833cfcd2feb7b98e59d0f9b619e9664fdefb7e16`; `gh pr view 471 --json reviews`
holds two `CHANGES_REQUESTED` submissions from `hermesvicowa`, at 14:57:56Z and 16:00:07Z, so the newer
review is the live verdict and it postdates the head commit at 15:48:33Z. Two rows of the brief were
stale on arrival and were re-read rather than repeated: the head had already moved one commit past the
`172d62d` the reviewer counted, and that commit had already re-scoped the review's second accused row;
the review body is 5672 bytes on the API read, not the 5713 the dispatch quoted. Both blocking claims
were re-counted on the tree this round ships before anything was written -- the census phrase stands
on seven rows of `factory/DECISIONS.md` at `172d62d` and at `833cfcd`, on six at `dcbb0f4`, on five at
`83a428e`, and on one at `ab9ad8d`, so the sentence that said five "at this head" was false on every
commit that ever held it -- and a third the review did not name: the pull-request body's census
sentence counted four added diff lines carrying the phrase, all inside `H-0307`, where the same diff
through `grep -n` on the phrase answers six at `833cfcd` and six on these bytes, four inside `H-0307`
and two inside `H-0308`, with no `-` line either side.

## What this round shipped

`b50bba4` is the checkpoint, taken before the judgement call. The commit after it carries
`Decision: H-0309` and three changes, every one of them a scope rather than a fresh figure:

- `factory/DECISIONS.md:15495` names `83a428e`, `dcbb0f4` and `172d62d` where it said "this head", and
  keeps the review's kept clause -- this row is among the counted ones, at each of the three.
- `H-0308`'s census sentence names `factory/DECISIONS.md` as the log its five sits in, which is the
  referent the review asked to be spelled out; the phrase is on no line of `DECISIONS.md` at any ref.
- `H-0309` records the choice, the options that lost and what the repair cost, and does not quote the
  counted phrase -- so the seven it reports is the seven its own tree answers. Both rows were rewritten
  on their own physical lines, one line in and one out, so no row below either moved and no coordinate
  aimed into the log needed re-aiming.

The pull-request body was corrected beside its own sha rather than restated here: its census sentence
now says six added lines, four inside `H-0307` and two inside `H-0308`. Plain gate on these bytes: rc
0, 377 entries, 1222 `path:line` citations, 583 owing an anchor, 0 refused outright, 7 `Effect:` diff
tallies, and `--self-test` unchanged at 80 must fail and 54 must pass.

## What is still owed (all of it, none silent)

1. Re-review. A push into a reviewed pull request spends the standing verdict (#269) and the re-ask is
   the supervisor's poke -- `factory_request_review`, `FACTORY_REVIEWER_TEAM` -- never this unit's,
   because `gh pr edit --add-reviewer` exits 1 on this account's retired `projectCards` field and writes
   nothing (`H-0010`).
2. The supervisor's merge, and nothing else this branch owns. The ticket's own verb rides in the
   pull-request body, so the ticket is taken by the merge, not by a hand and not by this file.
3. Non-blocking and left open on purpose: nothing in the tree re-measures a census figure written in
   prose, which is #433's complaint wearing a different coat. A gate that could read one is its own
   ticket, and `H-0309`'s `Follow-up:` says so instead of widening this scope.
4. Every owed row in the Records beneath this head stands exactly as the unit that wrote it left it,
   and none of it was taken by this round.

## Constraints a stranger would break here

- A row that counts this log's own rows names a commit, never a head. `git show <sha>:<path>` answers
  the same in every checkout and cannot be edited by a later row, while "this head" is falsified by the
  next row anybody writes. `H-0308` carries the mechanism, `H-0309` the rule and what lost.
- Do not spell `adds <n> (<word> heading)` inside an `Effect:` field, even to quote somebody else's
  receipt: `DIFF_TALLY` reads it as a tally this entry measured and moves the gate's own closing figure
  from 7 to 8. Quote the numbers, not the shape.
- Do not add an unanchored `path:line` to either log. `POINTER_DEBT` is 583 and may only fall; the
  refusal names your row. Anchor on the quoted text, on the same physical line as the coordinate.
- Fourteen pointers in `factory/DECISIONS.md` aim at two rows of THIS file -- 2257 and 2361 before this
  head, re-aimed in this same commit -- so any insertion here moves them. Ask
  `python3 scripts/check-decisions.py --citations NEXT.md` before you insert, not after it refuses.

## Record — the issue-389 delivery unit, rounds 1-2 (PR for `issue-389`: `H-0093`'s two `Effect:` tallies are annotated by a dated entry and their bytes were not touched, and `scripts/check-decisions.py` now prints how many `Effect:` diff tallies the two logs hold while re-measuring none of them — the fresh-or-stale comparison ships only as `tally_verdict(claims, measure)` behind nine `--self-test` legs, because this gate reads no history (#253) and a halting re-measure would stop the queue on a sentence about a merge from 10 Nov; 37 coordinates in this log re-aimed, 19 onto `scripts/check-decisions.py` and 18 onto `docs/factory.md`, plus 14 onto this file moved three times, and one row's pointer re-qualified to the commit its reading was taken at; the pass is `H-0307`; merging belongs to the supervisor)

State read before this file wrote a byte, by this unit: the ticket, fresh off the API at
2026-11-19T14:14Z, not off the dispatch — `gh issue view 389 --json number,state,title,url` answers
`{"number":389,"state":"OPEN", …}`, so neither deliverable had been taken by anyone else. The tree is
`/home/rodney/prj/wt/issue-389` on branch `issue-389`, synced to `origin/main` by `git merge --ff-only`
early in the unit and `git status --short` empty before the first edit; no `factory/HALT`. The drift was
re-derived before it was described rather than inherited from the ticket:
`git diff --numstat 5a1a559 6442e8f -- factory/DECISIONS.md` answers `112 0` and
`git diff --numstat c149fe9 6442e8f -- factory/DECISIONS.md` answers `504 0`, while
`git diff -U0 <base> 6442e8f -- factory/DECISIONS.md | grep -c '^+## '` answers 2 and 10 — each figure
short by exactly one heading, and in both pairs the missing heading is `## H-0093` itself. That closes by
size, because all four bases and the merge are in this clone and each comparison deletes nothing: the
file is 2829 lines at `6442e8f`, 2717 at `5a1a559`, 2325 at `c149fe9`, so an added-line count IS the
difference of two file sizes and the row's 54 and 429 describe working trees of 2771 and 2754 lines that
were never committed. `nine headings` survives on exactly one row of either log. The allocator was asked
before the judgement call and its first answer was refused by its own read — `REFRESH-BLIND 431s union` —
so `git fetch --prune origin` ran and the id was re-read as `H-0307 (REFRESH 0s union)`; that is why the
entry's `Checkpoint:` is `9fbe280` and not the commit the id was first printed on. Before this unit,
`python3 scripts/check-decisions.py --self-test` captioned 78 cases that must fail and 47 that must pass,
the logs held 2 `Effect:` diff tallies, and `POINTER_DEBT` read 583.

## What this dispatch pushed

`9fbe280` is the checkpoint: the tree the annotation and the count leg were written against, taken before
the judgement call, as the contract orders.

`9c036bf` carries `Decision: H-0307` and both halves of #389. The gate half is five names in
`scripts/check-decisions.py` — `DIFF_TALLY`, `tally_claims`, `measure_no_history`, `tally_verdict`,
`tally_line` — nine legs and eleven assertions in `--self-test`, and a third line on a clean run:
`check-decisions: 7 \``+`Effect:`\``+` diff tallies counted in the two logs, none re-measured — this
gate reads no history (#253), so a tally a later merge outgrew is named by that entry and never edited
(#389)`. The self-test goes 78 must-fail / 47 must-pass to 80 / 54, read off the caption this commit
ships. The comparison #389 asked for exists as `tally_verdict(claims, measure)` with a leg injecting a
fixture diff — `tally-stale-reported` runs #389's own pair, `adds 54 (one heading)` against a measure
answering 112 lines and 2 headings — while `tally-shipped-counts-only` fails the moment anyone wires a
real diff into the shipped arm. Five mutants were run against the new code with the rest of the file
untouched: `M1-pattern-no-singular`, `M4-headings-uncounted` and `M3b-measure-empty-stub` go red;
`M2-report-drops-measured` is caught by `tally-report-names-both`; `M3-measure-reads-diff` SURVIVES and
the entry says why in so many words, because a measure that agrees with every claim reports no drift and
no fixture-built leg can tell that from a clean run. The annotation half is `H-0307` itself, at
`factory/DECISIONS.md:15487`, and `H-0093`'s row keeps every byte: its two tallies are the read that
proves its merge lost neither side, so editing them would destroy the evidence the correction is about.
`POINTER_DEBT` goes 583 -> 587 with this entry's four anchored coordinates, and raising it moved a quote
that names a value — `H-0303` quotes `POINTER_DEBT = 583` off the tree its own merge resolved, the gate
refused that pointer once line 771 read 586, and the row now reads
`eacdaa9:scripts/check-decisions.py:771`, which `git show eacdaa9:scripts/check-decisions.py | sed -n
771p` still answers `POINTER_DEBT = 583`. 27 drifted pointers in this log were re-aimed by the gate's own
refusal (19 from the code landing, 8 of them twice, because a repair preceded the last edit) and 18 in
`docs/factory.md` moved +2 from this entry's correction to a sentence there. And
`docs/factory.md`'s "the closing line … is the only figure to trust" stopped being true the moment a clean
run prints three, so it now reads every line the gate prints — that sentence's own line, so nothing below
it moved.

This file, in the commit after those, is the hand-off; the issue-388 head is carried whole beneath as a
Record with its rows unrenumbered. Stacking it moved 14 `NEXT.md:NNNN` pointers in this log by +111 --
`H-0286`, `H-0300` and the rows that cite the hazard table -- and the same shipped refusal named all 14
for the hand-off commit to pay.

## What this push spent

No cloud, no credits, no GitHub write before the pull request. Locally: the gate and its `--self-test`
on every edit round, `python3 scripts/check-scripts.py` (it takes no file arguments — narrowing it to a
file is what made `check-mutations.py --help` open a file named `--help`), `bash scripts/check-next-md.sh`,
five mutant runs of the gate against `/tmp/opencode/tallyprobe/` copies of itself, one `git fetch --prune
origin`, two local commits, and zero pushes until the one below. #389 is still `OPEN` by design: the
`Closes #389` rides in the pull-request body, so the issue closes on the merge and not before it.

## What is still owed (this tick's rows first, then every carried hand-off row, none silent)

1. The pull request for `issue-389` is only opened once it is read back from the API — `gh pr view 4NN
   --json headRefName,mergeable,reviewCounts` — not from the exit status of the command that made it
   (#1180). The reviewer ask is the supervisor's poke, not this unit's: `gh pr edit --add-reviewer` exits
   1 on this account's retired `projectCards` field and writes nothing (`H-0010`).
2. #389 closes when the supervisor merges, via the body keyword. Until then the row at
   `factory/DECISIONS.md:2907` (`H-0093`) still reads 54 and 429 — deliberately, and `H-0307` is what
   names them stale.
3. The shipped arm still cannot tell a true receipt from a drifted one. Anyone who wants that wants
   something this file cannot hold: an immutable pair per receipt, which means the commit carrying an
   entry has to be knowable before the entry is written — `H-0306`'s "measured before the entry itself
   joins the corpus" is the same defect read from the inside. `measure_no_history` is the seam to
   replace; `tally-shipped-counts-only` is the leg that refuses the version that halts the queue.
4. `M3-measure-reads-diff` surviving is an open gap in the leg set, recorded in the entry rather than
   papered over: nobody has written a leg that catches an always-agrees measure.
5. The printed number counts occurrences of a pattern. On this commit it prints 7 and there are two
   receipts — five of the seven are `H-0307` quoting the two figures. Do not read that line as a defect
   count; nothing in this file can make it one.
6. Every row of the carried hand-offs below is unchanged and none of it was taken by this unit. The
   issue-388 head beneath this one is delivered — `origin/main` at `ab9ad8d` is `Merge pull request #470
   from vicowa-botworks/issue-388`, read off `git log` in this tree — and its owed rows stand as it wrote
   them for whoever owns them.

## Constraints a stranger would break on this branch

- `factory/selftest.sh:16162` (`gate_reads_no_refs`) greps `scripts/check-decisions.py` for a
  `"git", "<subcommand>"` pair with `cat-file` the only exception, and `make verify` runs that leg. So the
  tally arm cannot be "finished" by calling `git diff` from the gate; it will be refused by another unit's
  suite, not by review.
- Editing `scripts/check-decisions.py` moves coordinates that 19 rows of this log cite, and editing
  `docs/factory.md` moved 18 more below the edit point. Ask first with
  `python3 scripts/check-decisions.py --citations <path>`, then repair by the shipped gate's refusal in
  the same commit as the edit — this unit measured the ordering rule twice over, once by repairing before
  the last edit and once by paying for a repair that preceded it.
- Raising `POINTER_DEBT` breaks every row that quotes its value. The repair is to name the commit the
  reading was taken at (`eacdaa9:scripts/check-decisions.py:771`), never to edit the quoted prose, and the
  number may only rise beside an entry that records the measurement.
- The allocator refuses to be the only evidence for an id: `REFRESH-BLIND` and `REFRESH …s union` are its
  warnings, and `git fetch --prune origin` followed by a second read is what makes the id somebody else's
  neighbour-proof.
- `NEXT.md` holds exactly one level-1 heading, naming the unit that wrote it, and an owed section; new work
  is stacked above and the previous head is demoted whole to `## Record — …` with its rows unrenumbered
  (`scripts/check-next-md.sh` enforces the shape and cannot see whether the content is true).

## Record — the issue-388 conflict tick (PR for `issue-388`: the base sync this branch met — `origin/main@cfe1559` (the #467 merge, carrying #387's skip link `D-0087` and its base-sync pass `D-0089`) landed on this branch's shell-word rename `D-0088`; TWO conflicted paths — `DECISIONS.md`, seated as the ascending-id union `D-0083` prescribes (0087, 0088, 0089, so this branch's id sits in the gap #387's allocator could not see), and `docs/screenshots/360/manifest.json`, taken OURS on the one measurable ground the merge offered and paid back in the commit after it by re-photographing all nine frames off the merged sources; this hand-off was stacked by hand with the issue-387 head carried whole as the Record beneath it; the pass is `D-0088`; merging belongs to the supervisor)

State read before this file wrote a byte, by this tick: the dispatch was the queue's `conflict` row
for `issue-388`, and the read-back agreed — no PR exists for this head yet, so there was no
`mergeable` field to read and the conflict was read off the branch itself: `git merge-base HEAD
origin/main` was `37e8370`, `git rev-list --count HEAD..origin/main` was 16, and
`git merge-tree --write-tree HEAD origin/main` conflicted on exactly `DECISIONS.md` and
`docs/screenshots/360/manifest.json` — 12 conflict regions in the latter, one in the former. No
`factory/HALT`. `factory_worktree_busy /home/rodney/prj/wt/issue-388` answered free before the
merge was touched, and a live supervisor tick was seen running on `unclaimed:468`, not here.

## What this dispatch pushed

`a8adf95` resolves the two paths. `DECISIONS.md` is the both-appended tail it always is: ours is
`D-0088` whole (verified byte-for-byte against the entry this tick extracted from the pre-merge tip —
one trailing blank, the file's own inter-entry separator, which is why the merged ids run ascending
rather than in append order) and theirs is `D-0087` + `D-0089` whole; `scripts/check-decisions.py`
reads the merged file in `make verify`. `manifest.json` took OURS on one ground that is a fact rather
than a preference: our nine `png_sha256` are the digests of the PNGs the merge actually leaves in the
tree — `#387` touched no PNG its own way to ours, so the frames diverged on our side only and git took
ours without a conflict — while theirs would have recorded the digests of frames this tree does not
contain, which is the one thing `check`'s identity arm refuses. The mechanism behind "no conflict" is
worth naming because it is also why the re-shoot below came back byte-identical: the anchor #387 adds
(`page.tmpl:40`, `<a class="skip" href="#main">`) is styled at `app.css:537` as a 1 px square with
`overflow: hidden` and `clip-path: inset(50%)`, released only by `.skip:focus` — clipped out of the
paint, so the merged page photographs the same.

`a4f6393` pays the debt that choice creates: `head` and the nine `sources_rev` named a commit that no
longer described the sources, so the published recipe in `testing/capture/README.md` ("Re-capturing
the committed artifacts") was run literally against the merged tree — `go build -o plock ./cmd/plock`,
the four `*-setup.sh` + driver pairs at `--out docs/screenshots/360 --overwrite`, `manifest.py record`
with all four `--fixture-env` files and the nine `--shot` names, `manifest.py check`. It ended
`9 photographs current … rc 0`, and its diff is 29 lines in and 29 out with no `png_sha256` among
them: all nine frames came back byte-for-byte the files `0dc0fde` committed. That equality is the
measurement of the question the merge raised — the only thing the merge added to a photographed page
is the skip link, and nine frames over two fixtures, four drivers and two viewports say it costs no
pixels. The same run's transcript is the rename, still true on the merged tree: `nav=5 links
marked=Dashboard` on `/`, `marked=Audit trail` on `/audit`, `marked=Lockout profiles` on `/profiles`.

## The merge's silent path: one set, three ids used twice

The two paths that refused to merge got looked at. The one that merged cleanly is where the defect
was: `scripts/mutations/web.json`. Both parents appended three mutants to that set — trunk's at
rows 293-295 against the skip anchor, this branch's at rows 296-298 against the navigation words — and
both allocators counted the 292 rows their shared parent held and minted `W303`/`W304`/`W305`. Git saw two
disjoint appends and wrote six rows with three labels, and `expected_mutants` stayed 295 against 298
rows. `python3 scripts/check-mutations.py` refuses exactly that and refuses it in those words —
`holds 298 mutants, expected 295: a mutant was added without raising the number` and
`W303: id 'W303' is used twice in this set (also on mutant #292)` — and it is why the first
`make verify` against the merged tree stopped. `b1a7a79` (`D-0090`) moves this branch's three to
`W306`/`W307`/`W308`, notes and self-references included, and raises the count in the same edit;
trunk's labels stay put because a `W303` cited from `main` is a promise a reader can still open. A
reader who wants the shape re-checked asks the gate, not this file: `check-mutations.py` answers
`all 853 mutants apply and compile` and prints no verdict line.

## What this push spent

Four fixture boots and four browser runs (the recipe, start to finish ≈90 s), three `make verify`
runs against the merged tree, two `check-mutations.py` passes over all twelve sets, and no quota:
nothing here is a partner-node call. The build product `plock` stayed untracked
(`git ls-files | grep -x plock` is empty at this commit, per `#1264`).

## What is still owed (this tick's rows first, then every carried hand-off row, none silent)

- The supervisor's merge, and nothing else this branch owns. The review request is the poke's, not
  this unit's (`AGENTS.md`, "Requesting a reviewer is part of opening a pull request, not a later
  courtesy").
- **`make verify` can go red on a branch for a reason the branch does not own, and the suite writes to
  a real checkout while it does it.** This unit's first `make verify` answered
  `factory/selftest.sh: 885 passed, 5 failed` at 14:15:32Z, and the five are the shipped-supervisor
  dispatch legs (`the pass runs, over two units`, `quarantines A, mirrors both paths`, `…then starts a
  tick for unit B`, `…and gives the rescued unit its turn`, `the clean unit is not credited with a
  rescue`). The fixture journal names the cause: `UNIT-DIVERGED
  installed=/home/rodney/prj/parental-lockout this=/tmp/plock-factoryselftest-wyfBuH/supervisor/shipped`,
  then `SUPERVISOR-FF cfe1559 -> ab9ad8d commits=12 tree=/home/rodney/prj/parental-lockout`, then
  `SUPERVISOR-RERUN the pass stops here with exit 9: no triage, no id, no tick`. The self-update guard
  found the *installed* checkout 12 commits behind `origin/main` (#470 had merged 18 minutes earlier),
  fast-forwarded that real tree, and stopped the pass as designed -- so the legs read a journal holding no
  `TICK-START`. Control on the same bytes at 14:21:24Z, the install level: `bash factory/selftest.sh`
  answered `890 passed, 0 failed`, rc 0. Two alternatives were ruled out first: no other supervisor tick
  was live in that window (`factory/log/supervisor.log` between 14:05Z and 14:21Z holds this unit's
  `DECISION-ID` lines and nothing else), and the box was idle (load 2.17, 32 threads). What is owed is a
  decision before a repair: either the fixture holds its own installed tree, so a unit's `make verify`
  cannot fast-forward `/home/rodney/prj/parental-lockout` as a side effect, or the legs state the
  precondition and refuse loudly when the install is behind rather than reporting their own unit as the
  defect. The first is honest; the second still leaves every tick that runs while trunk moves explaining a
  red suite to whoever reads it next.
- **A number this branch's history asserts and the tree cannot reproduce.** `769c2bf`'s message
  quotes a nav-width census — `widths=109,72,100,116,82 rows=66,36,36,36,36 … budget=165 … needs at
  least 198 px … tallest row measured 66 px`. `git grep -n "109,72\|budget=165\|nav_width"` returns
  nothing at `0dc0fde`, at `origin/main`, and at this commit, and nothing in `testing/capture/`
  prints a nav width: `cdp.NAV_JS` measures each anchor's `w` and `h` for the #204 guards and
  `cdp.nav_census_line` prints only `nav=N links marked=X overflow=N`. So those figures are an
  uncited measurement, not a guarded claim — the #378 shape (a rule that witnesses by a raise alone,
  here with no raise at all). It is worse than uncited: it contradicts the numbers this same branch
  ships in `app.css:399`, which puts the five entries at Dashboard 96, Accounts 78, Audit trail 83,
  Lockout profiles 130, Settings 70 (457 px of box, nav 312x123, header 192 tall) where the commit
  message puts the widest entry at 116 px and the narrowest, the root label, at 72 px. One of the two
  was measured and one was not, and nothing in the tree can say which, because nothing in
  `testing/capture/` asserts a nav width at all. Nobody should quote either set as though a gate
  checks it; whoever wants the nav's box guarded has to write the assertion, and #468 is the issue
  `app.css` already names for exactly that absence.
- `docs/screenshots/360/manifest.json` will conflict again with any branch that re-photographs, and
  the resolution is a measurement, not a taste: keep the side whose `png_sha256` matches the frames
  the merge leaves, then re-run the recipe so `sources_rev` stops lying. Hand-editing a digest is the
  one move that cannot be defended.
- Carried whole in the Record beneath this head, unedited: every row of the issue-387 conflict tick,
  the issue-386 conflict tick, the issue-386 delivery unit and the issue-384 conflict tick.

## Constraints a stranger would break on this branch

- `DECISIONS.md` here is the both-appended tail; the resolution rule is `D-0083`'s ascending-id
  union, and `scripts/check-decisions.py --citations` is the re-check. This branch's id is `D-0088`
  and `main` also holds `D-0089`: a census of `^## D-00(8[5-9]) ` over every local and remote ref,
  taken by this tick after the merge, puts `D-0088` on `issue-388` and `origin/issue-388` alone and
  `D-0089` on `main`, and every `D-0085` heading in every ref is the same #355 entry — so no id here
  was minted twice, and the out-of-order tail is the union's shape, not a collision. Re-derive the
  taken-set over every ref before claiming one; `factory/next-decision-id.sh` reads what has landed,
  not siblings that have not.
- `manifest.py record` must be the LAST command of a re-capture and must name every frame it took
  with `--shot`. The driver `pages.py` records itself at the end of each of its own passes, so a run
  prints `carries <rev>, which is newer than its own pixels` notes off its own intermediate records;
  the final nine-named `record` is the one whose output is the verdict, and `check`'s carried-frame
  clause is what tells a carried row from a re-taken one.
- `plock` at the repo root is a build product and stays untracked (`#1264`), and `fixture.sh` refuses
  a binary older than the sources compiled into it — build first, every time, or the frames photograph
  a page nobody edited.
- The issue's own second ask is already delivered and merged-green here: `TestEachNavEntryNamesThePageItLinksTo`
  (`internal/web/page_heading_test.go:136`) reads each `<nav>` entry against the heading of the page
  its href names, which is the relationship nothing in the suite read before this branch.

## Record — the issue-387 conflict tick (PR #467: the base sync this branch met — `origin/main@623f685` (the #465 merge, carrying #466's capture pass `D-0086` and #465's label gate `H-0304`..`H-0306`) landed on this branch's skip-link work `D-0087`; TWO conflicted paths — `testing/capture/pages.py`, where both parents wrote into the same probe table at the same junction, and `DECISIONS.md`, where the shipped machine union refused once and the refusing seam was adjudicated on the merged bytes before it was let back in; this hand-off was stacked by hand with the issue-386 head carried whole as the Record beneath it; the pass is `D-0089`; merging belongs to the supervisor)

State read before this file wrote a byte, by this tick: the dispatch was the queue's `conflict`
row for PR #467 and the read-back agreed with the brief — `mergeable=CONFLICTING`,
`mergeStateStatus=DIRTY`, `reviewDecision=APPROVED`, head `1db3327`, base `main`, worktree
clean, no `factory/HALT`, branch matches. `git fetch --prune origin` rc 0; `origin/main` is
`623f685`, the branch fork sits at `963c2e4`, and `git merge origin/main` conflicted on exactly
those TWO paths. The allocator was asked after that fetch (`D-0089`, journalled `REFRESH 102s
union`), asked again later and refused itself with `REFRESH-BLIND 415s union`, which was answered
by `git fetch --prune origin` — rc 0, `origin/main` still `623f685` — and a re-read journalling
`REFRESH 1s union` immediately before the entry was appended; `D-0088` is not an inexplicable gap
— `refs/heads/issue-388` carries that heading, which is what moved the union's ceiling past it.
The order of the next two moves is NOT the rollback order the skill wants, and it is recorded
rather than smoothed: the dispatch arrived with `git merge origin/main` already in flight in this
worktree from the supervisor's own attempt, so the conflict surface was read from
`git merge-tree --write-tree`, which touches no working tree, and the empty checkpoint `89a48bc`
could only be taken ON TOP of that merge. While the merge is uncommitted the way back is
`git merge --abort`, which lands on a tree still holding that checkpoint and nothing else of this
judgement; once it is committed, force-push being denied here, the only move is a new commit on
top. That is what a checkpoint taken after the fact is worth, and a resolved conflict cannot be
un-resolved to slide one underneath it — replaying the merge instead would throw away the
adjudication recorded in `D-0089`.

## What this dispatch pushed

- One merge commit carrying `Decision: D-0089`, with the `D-0089` entry in `DECISIONS.md` in the same commit as what it decides.
- `testing/capture/pages.py` by hand with BOTH intents kept: trunk's answering `("returns", "pages.shoot", None)` at the navigation row this branch had left in the raise-only spelling, and this branch's three skip-link rows in that same kind. Which kind was not read off the neighbours — an instrumented copy of `_probe_every_rule` ran over the merged 84-rule registry twice on these bytes: the three rows as this branch wrote them (`hollow`) score 80 report, 2 report-and-raise, 2 raise-only; as merged they score 84 report, 1 report-and-raise, 0 raise-only, 0 silent. `python3 testing/capture/pages.py --self-test` exits 0 either way, so the gate cannot see the difference and the entry carries the measurement instead.
- The count paragraph above the table carries the re-measured split for the merged registry beside trunk's, each dated to the tree it measured: `EXPECTED_RULES` is this branch's 84 because the registry is the union of both parents' rules.
- `DECISIONS.md`, where the shipped union is the authority and it refused once: `UNION-REFUSED reason=id-collision … D-0085 … moved in OPPOSITE directions from base`. That seam is not two claims — this branch's only delta from base on `D-0085` is the blank separator line before its own next entry, while trunk's is the same coordinate re-aimed two lines down because trunk's `verify` recipe gained two steps above that one, so the merged tree decides it and `Makefile:155` is where the step is here. The seam was adjudicated theirs by rewriting this path's stage-2 blob in the index until that one entry matched base's bytes — the index only, never the worktree file — after which the shipped tool unioned the whole log: `UNION entries=73 ours=1 theirs=2 drift=0 repointed=14 files=1`, and it ran a second time with this file final, because the first run re-aimed fourteen pointers into a hand-off that then grew by eight lines and `check-decisions` read all fourteen landing eight lines low. Order that works: hand-off final, stage it, run the union, append the entry.
- One hazard found while doing that, because a stranger will reach for it: `git checkout -m -- DECISIONS.md` does NOT put a conflict back. It re-runs git's line-anchored three-way pass and stages the result, so the stage labelled "ours" arrives pre-filled with trunk's entries — measured here: after that checkout, `:2:DECISIONS.md` ended with trunk's re-capture sentence and carried `## D-0087` with no separator, neither of which is true of `HEAD:DECISIONS.md`. Restoring the three stages from the trees the merge names (`963c2e4`, `HEAD`, `origin/main`) and asserting each `:N:` against its blob is what made the union's inputs true again. Second papercut found in the same minute: once the index holds BOTH a stage-0 entry and the three stages — which is what the union's own `git add` left here — a plain `git add -- DECISIONS.md` returns 0 and clears nothing, and `git commit` answers `Committing is not possible because you have unmerged files`; `git update-index --force-remove -- DECISIONS.md` followed by `git add` is what takes the path to a single stage-0 entry, and `git ls-files -u` is the read that says so.
- This file, hand-stacked FIRST and staged, so the union's repointer aimed every pointer at the bytes this file ships: this head on top, the issue-386 conflict-tick head demoted whole to the Record beneath it with no line edited or dropped, and every Record below it carried as trunk shipped it. Zero-loss census: the multiset diff of nonblank lines against `origin/main:NEXT.md` is empty, and against this branch's own parent it is missing exactly one line — the superseded issue-384 level-1 heading, whose body trunk carries under its own demoted Record heading.

## What this push spent

- One decision id (`D-0089`), one checkpoint (`89a48bc`), one merge commit on a branch whose history is permanent here (squash and rebase merge are both off on this repository).
- Nothing, as far as the review is concerned — and that is as far as anything was measurable before the push: the `APPROVED` verdict on PR #467 was submitted 10:18:21Z against `1db3327…`, which is the head this branch holds while this file is written. Whether the head this push lands leaves it standing is read back afterwards; `factory_request_review` (`FACTORY_REVIEWER_TEAM`) owns any re-ask — a unit never asks, and this tick sends no poke. The post-push reads live in the pull-request comment.
- No GitHub write beyond the branch push this unit owes: no comment, no label, no reviewer ask, no issue touched, nothing merged.

## What is still owed (this tick's rows first, then every carried hand-off row, none silent)

1. Review on PR #467 after the push. Two reads, both plain, both quoted: at 10:18:21Z `gh pr view 467 --json reviews,reviewDecision,reviewRequests` carries ONE `APPROVED` submission from `hermesvicowa`, with `reviewDecision=APPROVED`, `reviewRequests` empty and head `1db3327…`; a read taken earlier the same morning showed a `DISMISSED` submission from `vicowa-qwen3-max` submitted 09:23:34Z against that same head. That trail is the API's and not this tick's — this tick dismissed nothing and asked no reviewer, and no push had been made while this file was written. What the next unit owes is the read AFTER the push: `gh pr view 467 --json reviewDecision,reviews,mergeable,mergeStateStatus,headRefOid` written to a file and read in full (no `--jq` — #270). Whether the approval survives the new head is that read's answer, because #269 measured a conflict-resolution push dismissing a `REJECTED` verdict and that is not the one carried here.
2. Two harness sentences still print `81` where the merged registry holds 84: arm 4's "all 81 rules reported as probed", and `_rule_probes_are_live`'s "`81 refusal rules hold` says nothing about it". Both quote the print at the head each was written, so they are measurements of their own merge and must NOT be bulk-repaired — the class is named in the carried Records below. Widening them is a capture-harness change with its own entry, not a conflict tick's.
3. Whether #387's skip-link bullet is delivered is the delivery unit's claim and this sync moved none of that behaviour: it re-spelled the vocabulary that witnesses three of its probe rows and re-measured the split. `D-0087`'s `Effect:` and PR #467's body carry the frames and the Go leg.
4. A phantom gate named in the dispatch text rather than in the tree: this tick's brief told the tick to run a shell file named `attack-decision-pointers.sh` under the scripts prefix, and no such path exists on `origin/main`, on this branch, or on any ref this clone holds, while the live skill's Step-4 line asks only for `make verify` and quotes its last line. Whoever owns the dispatch template owes that correction. This hand-off cannot name the phantom the way the brief did — the leg of `check-next-md` that refuses a hand-off naming a tool that is not there would refuse this file for it, which is the whole point of that leg and the proof the defect belongs upstream of the tick.
5. Every owed row in the Records beneath this head stands as written there, none re-adjudicated by this tick — including the `docs/labels.txt` re-read the issue-386 Record owes and the journalled-id class the earlier Records name.

## Constraints a stranger would break on this branch

- `DECISIONS.md` is append-only and keyed by id, and the union belongs to `scripts/merge-decisions.py`: it is atomic, it re-aims drifted citations itself, and it refuses while a non-log path is still conflicted. Hand-merging a log seam is what #381's 35 conflict ticks paid for.
- A row in `RULE_PROBES` is a measurement, not a spelling: run the new row through the probe pass and score whether it REPORTS, because a raise-only row green-lights exactly the gap `D-0086` closed and the self-test stays exit 0 either way.
- `gh pr edit` and `gh pr edit --add-reviewer` die on this account's retired `projectCards` field while looking like they worked; a pull-request body is written with `gh issue edit <N> --body-file` only after both type reads name the pull, because that same verb once destroyed an unrelated issue.
- Merging, approving, tagging and pushing `main` belong to the supervisor; force-push is denied and rebase merge is off, so a resolution here moves forward only as a new commit.

## Record — the issue-386 conflict tick (PR for `issue-386`: the second trunk sync this branch met — `origin/main@37e8370` (the #466 merge, carrying #463’s `H-0300`..`H-0303` and #466’s capture pass `D-0086`) landed on this branch’s `H-0304`/`H-0305` work; TWO conflicted paths: `factory/DECISIONS.md`, unioned by the shipped machine union after ONE seam adjudication (`H-0299`, where ours’ only delta from base was a stray separator blank over trunk’s disclosed re-aim) and zero anchor-debt rows, and `NEXT.md`, stacked by hand with both live heads carried whole as Records beneath this head; the pass is `H-0306`; merging belongs to the supervisor)

State read before this file wrote a byte, by this tick: the dispatch was the queue’s `conflict`
row for PR #465 — the brief’s live read (`CONFLICTING`/`DIRTY`) matched the read-back,
`reviewDecision=APPROVED`, head `0ed205b`, the five-row roll-up all `SUCCESS`, tree clean, no
`factory/HALT`. `git fetch --prune origin` rc 0; `origin/main` is `37e8370`, the branch fork
sits at `ad3201e` (the #462 merge), and `git merge --no-commit --no-ff origin/main` conflicted
on exactly TWO paths — `factory/DECISIONS.md` and `NEXT.md` — while `DECISIONS.md` auto-merged.
That raw merge was aborted and the checkpoint taken FIRST (`f927a7d`), because the resolution
is a judgement (`H-0306`). The allocator was asked after the fetch and before the checkpoint
(`H-0306`, journalled `REFRESH 34s union`, no `PENDING`, no `REFRESH-BLIND`) and re-read
immediately before the entry commit; this branch’s journal also carries the delivery tick’s
three disclosed rows (`H-0306` twice unearned, `H-0305` earned), named in `H-0306` itself
rather than papered over. The tool refused twice before accepting: `reason=other-conflict`
fixed the order — the hand-off had to be line-final before the log union, because the repointer
aims every `NEXT.md` pointer at the bytes this file ships — then one `id-collision` on
`H-0299`, adjudicated for theirs on the reading above and pre-patched into the stage so the
shipped tool, unchanged, decided the rest. The push spends the standing approval (#269 measured
a conflict push spends one); `make verify`’s verdict and the post-push review read are quoted
in the pull-request body (#1180: an `Effect:` written before the gate runs is a prophecy).

## What this dispatch pushed

- One merge commit carrying `Decision: H-0306`. `NEXT.md` was resolved by hand FIRST and
  staged: this head on top, the issue-386 delivery head demoted whole to a Record, the
  issue-384 conflict-tick head demoted whole to a Record with its delivery Record beneath
  it, and trunk's widened wording of the issue-383 record heading seated as the single
  heading for that body — every other line of both parents carried unedited.
- The shipped machine union against these final `NEXT.md` bytes: `UNION entries=297 ours=4 theirs=61 drift=8 repointed=14 files=1`, exit 0 — after the one seam above; zero `anchor-debt` rows, the 14 re-aims the tool opened and verified itself.
- Zero-loss censuses (nonblank line-multiset diffs run before staging): `git show 0ed205b:NEXT.md` against this file is missing exactly 2 lines, `git show 37e8370:NEXT.md` exactly 1 — the three headings the pruned table names, nothing else; `python3 scripts/check-decisions.py` on the staged pair rc 0: 369 entries, 1206 citations, `POINTER_DEBT` unmoved at 583 where both parents ship 583, 0 refused; `bash scripts/check-next-md.sh` on this file: its verdict is in the pull-request body.
- The `H-0306` entry, appended after `H-0305` at the unioned log's tail in id order —
  recorded in the same commit as the merge it decides, with the checkpoint above.

## What this push spent

- One decision id (`H-0306`), one checkpoint (`f927a7d`), one merge commit on a
  permanent-history branch (squash and rebase merge are both off here).
- The APPROVED verdict PR #465 carried: a push into a reviewed pull request dismisses it
  (#269), and the supervisor's poke owns the re-ask — a unit never asks.
- No GitHub write beyond the branch push this unit owes: no comment, no label, no reviewer
  ask, no issue touched, nothing merged.

## What is still owed (this tick's rows first, then every carried hand-off row, none silent)

1. Review on PR #465: whatever verdict the head before this push held is spent by it; the
   post-push read lives in the pull-request body, and `factory_request_review`
   (`FACTORY_REVIEWER_TEAM`) owns the re-ask.
2. `docs/labels.txt` is a dated snapshot and the label owner still owes the tracker re-read
   whenever a real new label lands — every owed row in the issue-386 delivery Record below
   stands as written there, including the CI-can-never-run-`--live` row and the prose-that-spells-
   the-flag row.
3. Every owed row in the two issue-384 Records below stands as written there — the three
   double-homed `factory/selftest.sh` rows the next repoint must read by sentence, the bare
   prose numbers that are measurements at their own merge and must NOT be "repaired", and the
   journalled-id class.
4. `#381`’s shape: this sync met the union with ONE refusal, and it was a separator blank,
   not a seam of competing claims — the measurement says why (disjoint entry tails), not
   that refusals are retired; the adjudication contract (`H-0303`) stands for the next
   seam. Neither ticket is a conflict tick’s to widen.
5. Every numbered owed row in the Records beneath, including the tail's numbered list down to
   the `#378` row trunk brought, remains owed where it says so.

## What this conflict tick pruned, and how to get it back

| Dropped | Where it is whole | Why it is a record and not an instruction |
|---|---|---|
| the issue-386 delivery head's level-1 heading (line 1 of the branch `NEXT.md`) | `git show 0ed205b:NEXT.md` | demoted to a Record heading below; its state read and all five record sections carried whole beneath it, so "this dispatch" names one unit |
| the issue-384 conflict-tick head's level-1 heading (line 1 of trunk's `NEXT.md`) | `git show 37e8370:NEXT.md` | demoted to a Record heading below; its state read, all four record sections and its own delivery Record carried whole beneath it |
| this branch's spelling of the issue-383 record heading | trunk's widened wording of the same heading, which adds the state-read clause | one heading per record body; the widened spelling is the one both Records beneath cite |

Everything else either parent held is carried forward unedited. The census behind this table
is the raw one-direction line-multiset diff of `git show 0ed205b:NEXT.md` and
`git show 37e8370:NEXT.md` against this file, run before staging; the only missing lines are
the three named above.

## Record — the issue-386 delivery unit (PR for `issue-386`: six documented `gh issue create` recipes named three labels this repository does not carry — `guard`, `testing`, `ux` — and nothing in the tree ever compared a documented name with the set GitHub actually has; `H-0304` re-points the six at names that exist and records the set in `docs/labels.txt`, `H-0305` ships `scripts/check-doc-labels.py`, which reads that file offline and the live API under `--live`, refuses drift in either direction, and refuses its own `verify` step being deleted; 12 self-test legs, the copy-the-gate-under-another-name launder test among them; merging belongs to the supervisor)

State read before this file wrote a byte, by this tick: branch `issue-386` cut from `ad3201e`
(`origin/main` at read time, which carries #462), tree clean at entry to this pass, no
`factory/HALT`, and `git merge-base --is-ancestor origin/main HEAD` answers true after the fetch
that refreshed the allocator's union — this branch is not behind trunk and no sync was needed.
`git ls-remote --heads origin issue-386` answered nothing before this push, so there is no upstream
to rewrite: the history below is written once and `allow_squash_merge`/`allow_rebase_merge` are both
false, so whatever merges is permanent history.

The id chain is disclosed rather than quiet, because three reads were needed to earn one id. The
allocator was run twice with the entry already in the working copy and both times it answered
`H-0306` — its own words name the reason: "H-0305 is in factory/DECISIONS.md and on no ref this
clone can see — so this read says the ceiling came from the log you are writing, not from a
sibling", and it told this unit to keep the id it had taken. That answer is true and useless to the
provenance pass, which wants a journalled allocation FOR the id this unit ships, so the entry was
held aside (the file truncated at its own heading, the bytes kept at `/tmp/opencode/decisions-H0305.md`)
and the third read issued `H-0305`. The journal therefore holds three rows for this branch:
`08:21:33Z DECISION-ID H-0306 (REFRESH-BLIND 921s union)`, `08:21:36Z DECISION-ID H-0306 (REFRESH 2s
union)` and the earning row `DECISION-ID H-0305 (REFRESH 29s union)`; `H-0304` was journalled at
`08:08:49Z (REFRESH 157s union)` before its own checkpoint. The restore was verified by md5 on both
sides (`24d9b59ff29948713a1710c6687cc2ca`) and `check-decisions.py` over the restored tree reads
rc 0 with 364 entries, 1189 citations and the anchor debt unmoved at 583 — the held-aside window
moved no byte that the gate can see.

## What this dispatch pushed

- Two decisions, two checkpoints, two effect commits. `H-0304` (checkpoint `be1a40f`, effect
  `d1c472b`) is the repair: four commands in `docs/issues-from-d-0058.md` and one in
  `docs/issues-from-d-0065.md` take `kind:chore,area:ui`, one takes `area:ui`, both pages name the
  ground file and the mapping, and `docs/labels.txt` holds the live 19 Nov read. `H-0305`
  (checkpoint `df77270`, this commit) is the gate that keeps it true.
- `scripts/check-doc-labels.py` censuses `--label` across every text file in the tree (369 files
  today) and refuses five things: a name the ground does not carry; a ground file that contradicts
  itself (a duplicate, a name out of C order, a whitespace-padded name no token can equal) or holds
  no names at all; an empty census reported as `commands=0` rather than as coverage; the gate's own
  source included in its own census — the exemption is by `samefile`, and a leg proves that copying
  the file under another name buys nothing; and its own `verify` step deleted, commented, or joined
  into an `&&` list where errexit cannot see it. It reports and does not judge the
  `--add-label`/`--remove-label`/`labels=` family — 25 occurrences today — because those act on an
  issue that already exists.
- Ground truth is `docs/labels.txt` offline and the API under `--live`, which refuses drift in BOTH
  directions and prints `SKIPPED` with exit 2, in words, when the API cannot be reached. That is
  #391's rule (a live arm that skips silently is a gate that prints) and #443's (the skip is the
  thing that must be witnessed), and it is why the default step reads a file rather than the
  network: `.github/workflows/ci.yml` carries no `gh auth` step and no `GH_TOKEN`, so a
  verify-time API call would be a skip on every CI run — measured by `grep -in
  'gh auth\|GH_TOKEN\|secrets\.' .github/workflows/ci.yml`, which answers nothing.
- `Makefile` gains two steps after the hand-off step: the `--self-test` step then the check step, as
  `check-decisions.py` and `check-denies.py` each do, and the gate reads those two lines back out of
  the Makefile and refuses if either goes missing or loses its exit status. Default run today:
  `commands=6 names=10 absent=0 inert=25 files=369 ground=18 verdict=clean`, exit 0; `--live` today
  against the real API: `names=10 absent=0`, exit 0, naming the ground as the live API re-read and
  identical to the file, 18 names each side; `--self-test`: 12 legs, exit 0.
- Anti-scope held: no label was created, renamed or deleted, and no `gh issue create` was run to
  prove a recipe — the claim this branch makes is name resolution, the half #386 measured as fatal.
  Nothing under `.opencode/skills/factory/` or `factory/` was touched, so #311's escalation name and
  #340's literal interpolation stand where those tickets left them.

## What this push spent

- No force-history: `issue-386` had no upstream before this push and none of these commits were
  rewritten. After the push the history is permanent — merge-commit-only repository.
- Two pointer spends, both digits-only and both repaired in the commit that moved them. The Makefile
  insertion shifted every `verify` line from 125 down by two, and seven anchored pointers named a
  line below the insertion (one at 125, two at 134, four at 153), so they now read 127, 136 and 155;
  each was opened on these bytes after the edit. This head's own insertion shifts every `NEXT.md`
  row below it by 136 lines, which took 14 anchored pointers into this file — the two rows seven other
  units cite, at 1582 and 1686 before this block and at 1718 and 1822 after, in `H-0252`,
  `H-0280`–`H-0284` and `H-0286` — off their rows; `check-decisions.py`
  refused them at rc 1 and the repair is its own — re-aim the numbers, then re-open the anchors.
- `python3 scripts/check-decisions.py` over the pair reads rc 0, and `POINTER_DEBT` is unmoved at
  583 and *judged* — 0 refused outright, 0 unopenable — with `git status --short --ignored` holding
  one ignored path this run created, `scripts/__pycache__/`, the interpreter's and not the logs'.
  The seven citations this entry adds each carry an anchor, so 1182 citations became 1189 with the
  debt unchanged.
- One live read: `--live` runs `gh label list` (read-only) against this repository. Nothing was
  created, posted or merged to produce these numbers.

## What is still owed (this tick's rows first, then every carried hand-off row, none silent)

- `docs/labels.txt` is a dated snapshot, so a label created on this repository after 19 Nov is
  REFUSED by `make verify` until a human re-reads the tracker:
  `gh label list --limit 200 --json name --jq '.[].name' | LC_ALL=C sort > docs/labels.txt`. A gate
  that refuses a real new label is the safer direction of failure, but it is still a queue for
  whoever owns labels, and the file's header is the only thing that says how old it is.
- CI can never run the truest verdict this gate has, because CI has no credential. If the loop ever
  gives CI a read-only token, the move is a THIRD step running `--live`, kept as its own step so its
  skip is visible in the log rather than folded into a passing one (#443). Until then `--live` is a
  human verb and `docs/factory.md` does not say so anywhere — a paragraph there naming when to run
  it is owed to whoever hits the first false-absent refusal.
- Prose that spells the flag beside a name is a documented command to this gate, and that includes
  prose in the decision logs and in this file: `guard,testing` as a value is refused here even in a
  sentence explaining that the name is absent. This unit re-worded three such sentences rather than
  widen the regex. A successor who finds that annoying should add a fixture-escape convention to the
  gate and assert it, not exempt a directory — leg 5 exists to refuse directory-shaped blindness.
- The 25 `--add-label`/`--remove-label`/`labels=` occurrences are counted and printed, not judged.
  Judging them needs a ground for labels-on-an-issue, which is a different question from this one;
  a successor who wants it should start from the count line, not from a wider regex.
- Outside the census, deliberately: a label named in prose, in a `gh --jq` filter or in a
  `--template` file. And gh's merge-or-replace semantics when the flag is given twice, one name each
  time, were never measured here, so the gate makes no claim about that form; whoever measures it should write
  the measurement into the docstring, not into a regex.
- Re-review is the supervisor's poke, not this unit's verb: `factory_request_review` addresses
  `FACTORY_REVIEWER_TEAM` for an open pull request whose `users` and `teams` lists are both empty
  (`gh pr edit --add-reviewer` exits 1 on this account's retired `projectCards` field and writes
  nothing, `H-0010`).
- Carried from the issue-383 head below, unchanged: re-review of its round-1 pointer fix is owed, on
  its own schedule bounded by `FACTORY_REVIEW_POKES`.
- Carried: issue-383's second half — the #246 row-2 shape where the tick pushed BEFORE the ceiling
  took it — is named in the 137 sentence's consequence only; a journal cross-reference between the
  failure line and the `TICK-UNDELIVERED` mirror would be a new ticket, not a leftover.
- Carried: `FACTORY_TICK_KILL_AFTER` is honoured by the one shipped `timeout` line; a second ceiling
  must print through the same `case` or it re-creates #383 exactly.
- Carried from the issue-380 head below, unchanged: the `H-0289` allocator collision (the
  journalled-id-no-union-can-retire class, two timestamps) — if `issue-381` wakes before that branch
  merges, one of the two entries has to move.
- Carried: issue-380's own unit never wrote a hand-off; #459's head below is the reconstruction and
  is the reason the shape question stays open.
- Carried: `python3 scripts/check-close-verb.py NEXT.md` answered rc 1 on two carried rows before
  this head, and this head adds no third (its own rows name issues without keywords, per #1256);
  `check-close-verb.py factory/DECISIONS.md` rc 1 is trunk-carried (`closes #211`), unchanged here.
- Carried: the `install` half of #377 is still an operator act (`systemctl --user daemon-reload` and
  a unit regeneration on each installed machine), not a unit's.

## What this head pruned, and how to get it back

- Nothing. The issue-383 head is demoted whole to a Record below — its heading, its state paragraph,
  its pushed/spent/review-round/owed/pruned sections all kept in full below this block. To read the
  pre-insert bytes: `git show ad3201e:NEXT.md`. The only content this block adds above it is this
  unit's own five sections and fourteen `Carried` rows folded from the head it supersedes.
- One mechanical cost, stated so the next unit does not re-derive it: this insertion moves every row
  below line 1 down by the length of this block, and the 14 anchored `NEXT.md` pointers named in
  `What this push spent` were re-aimed in the same commit. If you are reading a `NEXT.md:NNNN`
  citation and the sentence is not there, run `python3 scripts/check-decisions.py --citations
  NEXT.md` before believing your own eyes about the number.

## Record — the issue-384 conflict tick (PR for `issue-384`: #463 — the third live seam in a row met by the shipped machine union: 294 entries decided, seven seam rows adjudicated by anchor uniqueness on the merged bytes, ten anchor-debt rows seated by the sentences that cite them, zero content retired beyond six displaced quotations; the pass is `H-0303`; merging belongs to the supervisor)

State read before this file wrote a byte, by this tick: the dispatch was the queue's
`conflict` row for PR #463 — the brief's live read (`CONFLICTING`/`DIRTY`) matched the
read-back, `gh pr checks` projected 0 rows (a roll-up lag, not a verdict, and its exit is never
a verdict either — #1180), head `11e8878`, tree clean, no `factory/HALT`.
`git fetch --prune origin` rc 0; `origin/main` is `00012e7` (the #462 merge: `H-0296`..`H-0299`
plus its repoint), five commits past the base, and `git merge origin/main` conflicted on
exactly ONE path, `factory/DECISIONS.md` — `NEXT.md` and `factory/selftest.sh` arrived from
`ort` and are censused below like every resolution git makes without being asked. The allocator
was asked after the fetch and before the checkpoint (`H-0303`, journalled `REFRESH 145s union`,
no `PENDING`, no `REFRESH-BLIND`) — the same id the delivery head below disclosed as free on
its own 10 s read, and free it stayed: no ref and no tree carried a `## H-0303` heading at
either read. `make verify`'s verdict and the post-push review read are quoted in the
pull-request body (#1180: an `Effect:` written before the gate runs is a prophecy).

## What this dispatch pushed

- One merge commit carrying `Decision: H-0303`. The first live run of
  `python3 scripts/merge-decisions.py` refused `reason=id-collision ... H-0216` — H-0299's
  refusal loop (one id per `--plan-only` round) was skipped and the whole residue enumerated
  over the three index stages: seven same-id collisions (H-0216, H-0229, H-0240, H-0243,
  H-0259, H-0261, H-0298), then ten `anchor-debt` rows the union could not re-aim uniquely.
- Every seam adjudicated on anchor uniqueness against the merged tree, not preference — all
  seven landed theirs-side (each ours wording quoted text the merge duplicated; the exact
  quotings, and the digit-twin proof H-0240 and H-0243 needed before their anchors could
  decide, are in the entry) — and the ten refusals seated by the citing sentences: H-0299's own
  Effect names the escalation-fixture pointer as the first of the two byte-identical homes,
  H-0230's sentence distinguishes its leg row from its definition row, and H-0298 says in
  words that both parents cited the second `negated_status_refused` occurrence. Both index
  stages were pre-patched with the adjudicated rows (`git hash-object -w` +
  `git update-index --index-info`) and the shipped tool, unchanged, decided the rest:
  `UNION entries=294 ours=16 theirs=3 drift=44 repointed=135 files=1`, exit 0.
- Zero-loss censuses, digit-folded non-blank rows added since base `00012e7`, absent-count
  against the merged tree: `NEXT.md` 71/0 and 0/0; `factory/selftest.sh` 258/0 and 131/0;
  `factory/DECISIONS.md` theirs 176/0, ours 254/**6** — and the six are exactly the adjudicated
  seam quotations, so the only content retired is displaced quotings: no entry, gate or product
  line. `bash -n factory/selftest.sh` rc 0. `python3 scripts/check-decisions.py` rc 0 on these
  bytes: 366 entries, 1199 citations, `POINTER_DEBT` unmoved at 583 where both parents ship
  583, 0 refused.
- This head; the delivery head below demoted whole to a Record.

## What this push spent

- One decision id (`H-0303`), one checkpoint (`cb5abcf`), one merge commit on a
  permanent-history branch (squash and rebase merge are both off here).
- No GitHub write beyond the branch push this unit owes: no comment, no label, no reviewer ask
  (the poke is the supervisor's), no issue touched, nothing merged.
- The re-aims are the tool's, not hand moves: 135 pointers re-opened by the shipped repointer;
  the ten seats each kept their own anchor text and moved only the number.

## What is still owed (this tick's rows first, then every carried hand-off row, none silent)

1. Review on PR #463: whatever verdict the head `11e8878` held is spent by this push (#269
   measured a conflict push spends one); the post-push read lives in the pull-request body, and
   the poke (`factory_request_review`, `FACTORY_REVIEWER_TEAM`) owns the re-ask — a unit never
   asks.
2. Three double-homed rows in `factory/selftest.sh` are live hazards for the next repoint,
   censused this tick: the `FACTORY_ESCALATION_LABEL=plock-awaiting-a-person claim_rows` line
   sits byte-identical in two fixture bodies, the failure sentence and the `proc_leg` title
   share "aimed at the guard", and `busy_guard_refuses_a_live_tree` names both its definition
   and its call site. The seats this merge chose (first fixture home, the `proc_leg` line, the
   call site) follow the citing sentences — a successor must read the sentence, never re-number
   by proximity.
3. `H-0299`'s and `H-0298`'s bare prose numbers (`9986`, `22238`, `22084`, `7810`, `7699`) are
   measurements at the #462 merge while their `path:line` citations carry this merge's seats —
   disclosed in `H-0303`'s `What lost:`, and NOT to be "repaired": editing a published entry is
   the H-0237 class.
4. The allocator journalled `H-0303` to this unit and the entry lands in this commit — the
   journalled-id-no-union-can-retire class stays carried for the siblings whose entries never
   landed, unfixed and un-papered.
5. #381's shape, met a third sync in a row: the machine union still needs adjudication input
   for seam rows and anchor debt — this tick supplied it as index patches rather than edits of
   the conflicted file, which is the shape #440's cited-line-holds acceptance made legal.
   Neither ticket is a conflict tick's to widen.
6. Every owed row in the delivery Record below stands as written there — the
   `TICK-DELIVERY`/`pushed=` rows, the issue-383 carried rows, the `check-close-verb` rc-1
   census (this head adds no keyword-number match; the file's two carried `#425` hazard rows
   are untouched below), and everything the Records beneath carry.

## What this conflict tick pruned, and how to get it back

| Dropped | Where it is whole | Why it is a record and not an instruction |
|---|---|---|
| the issue-384 delivery head's level-1 heading (line 1 of the pre-merge `NEXT.md`) | `git show 11e8878:NEXT.md` | demoted to a Record heading below; its state read and all four record sections are carried whole beneath it, so "this dispatch" names one unit |

Everything else the file held is carried forward unedited. The census behind this table is the
raw one-direction line-multiset diff of `git show 11e8878:NEXT.md` against this file, run
before staging: the only missing line is the demoted heading itself.

## Record — the issue-384 delivery unit (PR for `issue-384`: what the pass says when a tick stops after delivering — the failure arm asks the delivery question too and journals `TICK-DELIVERY … pr= head= mergeable=` beside its `TICK-FAILED` (`H-0300`), `PRESERVED` now says whether the remote already holds the tip (`pushed=true` / `pushed=false` / `pushed=unknown`, `H-0301`), and the cause extractor drops a line whose own expectation pair is satisfied and prefers predicate-position failure lines over bare-token ones (`H-0302`); seven legs with controls witness the three — four `proc_leg`s, two of them controls, and three `mirror_leg`s for the push-state tri-state; merging belongs to the supervisor)
State read before this file wrote a byte, by this tick: the dispatch was the queue's
`branch_without_pr` row for `issue-384` — 8 commits and 4 files ahead of `origin/main`, no open
pull request, local only. Tree clean at entry, no `factory/HALT`, `git fetch --prune origin` rc 0,
`gh auth status` live as `virtualcodewarrior`, `gh pr list --state open --head issue-384` answered
`[]`, `gh issue view 384` answers OPEN with labels `bug,area:ops`, and
`git ls-files | grep -x plock` prints nothing. No new decision id was taken by this dispatch: its
single allocator read (journalled `DECISION-ID H-0303`, `REFRESH` 10 s union) is disclosed because
a journalled read consumes nothing (#442), and this head ships only what `H-0300`..`H-0302`
already record. The collision sweep this hand-off owes ran before the write: a grep for a
`## H-0300` heading (and `H-0301`, `H-0302`) over every `refs/heads` and `refs/remotes` this clone
names, `issue-384` and `HEAD` excepted, returned nothing on all three ids. `make verify`'s verdict
is quoted in the pull-request body, taken after the gate ran, not before (#1180: an `Effect:`
written before the gate runs is a prophecy).

## What this dispatch pushed

- The eight commits ahead of `origin/main` are three decision pairs and two support commits:
  `pre-H-0300`/`3cc8e44` (`factory_delivery_after_failure` in `factory/common.sh`, called from the
  failure arm beside the `TICK-FAILED` line), `pre-H-0301`/`f268a17` (the `pushed=` read inside
  `factory_preserve_unpushed`, carried on the `PRESERVED`, `PRESERVE-FAILED` and
  `PRESERVE-UNVERIFIED` sentences), `pre-H-0302`/`f8d830c` (the expectation-pair drop and the
  two-tier predicate-position pick in the cause extractor), `595a62b` (160 citation pointers
  re-aimed for rows the three insertions moved) and `c1667c4` (the seven legs). This dispatch
  adopts that branch and opens its pull request, and holds every one of the ticket's three demands
  and three leg-pairs against the diff before writing this file.
- `762c6b2`, this dispatch's code commit: the cause-filter comment was labelled `H-0303`, an id no
  decision log and no ref carries (the allocator still hands `H-0303` out as free) — a dangling
  decision citation, the #393 class, standing in this branch's own bytes. It now cites `H-0302`,
  whose `Chose:` is verbatim the two-tier design the note annotates. No line count moved, so that
  entry's two anchored `Effect:` citations still open.
- This hand-off. A line-multiset census of `git show origin/main:NEXT.md` against the file
  `707020b` shipped (run by the splice script and reported in that commit's message) found no lost
  line except the demoted level-1 heading itself — the same census shape #459's head ran.
- `ed997a2`, this dispatch's citation repair: the legs commit moved 135 `factory/selftest.sh`
  pointers (+240 above ~9900 and +262 above ~19700; the tree they were true at is `f8d830c`, which
  carries `3cc8e44`'s +4 above line 3416) and this hand-off's head block moved 14 `NEXT.md`
  pointers (+70). Re-aimed by row identity — each pointer opened at the blob it was true at, its
  line plus a 3-line context window matched into this tree, never a delta — with digits-only
  asserted by comparing both logs stripped of digits against HEAD.

## What this push spent

- Three commits (`762c6b2`, this hand-off and `ed997a2`) plus this top-up, and the `make verify`
  runs — the first answered rc 2 at the decision-log step on the 149 drifted pointers `ed997a2`
  repaired, and the verdict quoted in the pull request is taken after this file's final edit — one
  allocator read (disclosed above, nothing taken), and no GitHub write beyond the branch push and
  the pull-request creation this unit owes. No reviewer ask: the poke is the supervisor's, and
  this unit's branch had no prior review to spend.

## What is still owed (this tick's rows first, then every carried hand-off row, none silent)

- The failure-arm read-back covers the four classes that expect a pull request
  (`unclaimed|branch_without_pr|address_review|conflict`) — the exit=0 arm's own whitelist,
  ported. A new dispatch class that can leave a pull request behind must widen the `case` inside
  `factory_delivery_after_failure` and the exit=0 arm together; widening one alone re-creates the
  asymmetry #384 was filed for.
- `TICK-DELIVERY` is its own journal line beside `TICK-FAILED`, not a field inside it: keeping
  `TICK-FAILED` as the verb and exit 4 as the verdict is `H-0300`'s deliberate half. A reader who
  greps only `TICK-FAILED` rows will not see `pr=` — grep `TICK-DELIVERY` for the delivery, and do
  not soften `TICK-FAILED` into it.
- `pushed=unknown` is a read that failed, not a `false` (#228's polarity: a failed read is not
  evidence of the clean state — and here the clean-looking reading, "not pushed, all at risk!", is
  exactly the #384 false alarm). Anything downstream that branches on the field must treat
  `unknown` as neither copy-confirmed nor only-copy.
- The head oid `TICK-DELIVERY` prints is seven chars (jq `.[0:7]` of `headRefOid`), matching the
  ticket's example `head=5e24bb0`; a comparison against a 40-char head must fold first.
- Carried: the issue-383 rows in its record below stand as written — its re-review is the queue's
  path, bounded by `FACTORY_REVIEW_POKES`, and its second-half row (#246's row-2 shape) is a new
  ticket, not a leftover of this one.
- Carried: `python3 scripts/check-close-verb.py` answers rc 1 on `NEXT.md` (two hits, both carried
  rows) and rc 1 on `factory/DECISIONS.md` (ten hits); both files were scanned at `origin/main`
  and here and diffed, and the hit sets are identical — this head adds no keyword-number match to
  either.
- Carried: every numbered owed row and hard rule below this record is intact and owed where it
  says so; rows 53, 55 and 59 remain the bare-pointer debt census, each at its own head.

## Record — the issue-383 unit (PR for `issue-383`: `TICK-FAILED` writes one sentence per way a tick can die — 124 names the ceiling TERM arrived at, 137 names the ceiling, the grace, the SIGKILL and the trapped signal, `*)` says not-a-ceiling-death and explains both; the grace is `FACTORY_TICK_KILL_AFTER`, a knob the sentence prints; five selftest arms — three legs, a wording mutant of the shipped sentence, and a deletion control whose wedge holds the pass open for the outer bound because kill-after is what releases it; `H-0291`; its state read is its own clock's; merging belongs to the supervisor)
State read before this file wrote a byte, by this tick: branch `issue-383` rebased onto
`b6b8a55` (`origin/main` at read time, which carries #459's merge with `H-0288` and `H-0289`),
tree clean at entry to this pass, no `factory/HALT`. The id chain is disclosed rather than
quiet: this unit checkpointed first under `H-0288`, `origin/main` then took that id for #459's
refusal-arm entry while this unit was in flight. Two allocator reads were journalled and each
got one clause, because the first was the wrong log: `2026-11-19T02:44:11Z DECISION-ID D-0086
unit=unclaimed:383 branch=issue-383 (allocator, DECISIONS.md, REFRESH 285s union)` — the root
`D` log, named because the path is the selector and I typed the wrong prefix, an answer that
belongs to no entry — and the correct `factory/next-decision-id.sh factory/DECISIONS.md` read
answering `PENDING H-0291 is in factory/DECISIONS.md and on no ref this clone can see ... so
H-0291 is free`, ceiling `H-0290` on the union: the tool confirms the id this entry already
holds rather than issuing a new one. A hand union read over every `refs/heads` and
`refs/remotes` this clone names independently puts `H-0290` on `origin/issue-381` and nothing
at `H-0291` — so this unit ships
`H-0291`, and `git grep '^## H-0291'` over every `refs/heads` and `refs/remotes` this clone can
name answered nothing at read time. If `issue-381` merges after this branch is cut, nothing
collides: `origin/issue-381` holds `H-0290` and this branch holds `H-0291`. What DOES still
collide is the older pair `#459`'s head warns about — `H-0289` journalled for `unclaimed:381`
at `2026-11-19T00:50:21Z` and re-issued to #459 — and that row is carried below, not re-decided
here. `make verify`'s verdict and the post-push review read are quoted in the pull-request body
(#1180: an `Effect:` written before the gate runs is a prophecy), and this head is written after
the gate ran, not before.

## What this dispatch pushed

- One effect commit over four files, `Decision: H-0291`, on one checkpoint
  (`chore(checkpoint): pre-H-0291`, re-taken at the re-sync because the checkpoint first existed
  under the id #459 then took). `factory/factory.sh`'s failed-tick arm now `case`es on `$status`:
  124 prints the ceiling TERM landed at, 137 prints the ceiling, the grace, the SIGKILL and the
  reason that matters (the tick trapped the signal and would not stop), `*)` says not-a-ceiling-
  death and explains both codes. There is no default branch — a new ceiling means a new sentence.
- `FACTORY_TICK_KILL_AFTER` joins `FACTORY_TICK_TIMEOUT` in `factory/common.sh`: the same
  `timeout` line feeds it and the 137 sentence prints it, so the grace cannot be re-spelled into
  a journal that lies about the wait it imposed.
- Five arms in `factory/selftest.sh`: the 137 escalator (tick B still runs after a wedged A), the
  124 TERM death (asserts the 5 s ceiling it set and carries no escalation claim), the `*)` arm
  witnessed by the fail family's own shipped run at zero extra runtime, a wording mutant proving
  the 137 assertion bites the shipped sentence's exact claim, and a deletion control that removes
  `--kill-after` and shows the wedge holds the pass open until the outer bound — the #383 outage
  in miniature, asserted by requiring the outer 124|137 plus a journal that never reaches
  `TICK-FAILED`. The ceiling arms do not inherit `PROC_STUB`: the stub is built once in the box
  body, and a ceiling leg must rewrite the `opencode` stub, not the unit script.
- `docs/factory.md`'s constants table gains the `FACTORY_TICK_KILL_AFTER` row: the number the
  137 sentence prints is a journal-visible fact, so it cannot live only inside the `timeout`
  argument. (No cause table quoted the old sentence — `docs/factory.md:94` is a historical quote
  of the #212-era journal and stays verbatim.)

## What this push spent

- One branch force-history: none. `issue-383` had no upstream before this push; the rebase of the
  pre-H-0291 checkpoint happened before any ref left this machine, so no rewrite is in anyone's
  past. After the push the history is permanent: `allow_squash_merge` and `allow_rebase_merge`
  are both false, so whatever merges is a merge commit forever.
- The same standing spend every unit makes: the sync plus this unit's insertions moved the
  ground under the citations — **178 lines of `factory/DECISIONS.md` re-aimed, 242 numeric
  tokens, digits-only**, proved the way `#459`'s head prescribes: fold every digit in the
  committed diff and the folded diff shows only this unit's entry additions, nothing else moved.
  The 14 anchors whose new coordinate answered to more than one line were adjudicated by
  opening both candidates; the rest moved to coordinates where the anchor is unique.
  `POINTER_DEBT` itself is unmoved at 583 and judged again — with the two `__pycache__`
  directories the suite run left (ignored, the interpreter's not the logs') present, the
  comparison still came back judged: rc 0, 0 refused, 0 unopenable pointers.

## Review round 1 — `CHANGES_REQUESTED` at 2026-11-19T03:28:18Z (hermesvicowa, review 5968895974), answered

- The blocking ask: two source comments this branch added carried `H-0288 (#383)` while the
  decision this PR ships is `H-0291`; `H-0288` is the merged #380 lock-holder-journalling entry.
  Independently re-derived before anything changed: `git blame` dated both comments to this
  branch's effect commit `d845517`, and the third `H-0288` in the tree — `factory/factory.sh:2017`,
  inside the flock-refusal arm — comes from `2aa6dac`, an ancestor of `origin/main`, and cites the
  right decision for the arm it annotates, so it stays.
- The fix re-aims `factory/common.sh:152` and `factory/factory.sh:1472` to `H-0291 (#383)` —
  comment text only, two files, two lines. Verified on the tree after the fix: `git grep
  'H-0288 (#383)'` is empty, `python3 scripts/check-decisions.py` reads rc 0 with the same counts
  the reviewed head had (353 entries, 1164 citations, 583 debt, 0 refused), and
  `python3 scripts/check-decisions.py --citations` on both edited files names no pointer aimed at
  either edited row, so nothing the logs quote moved.
- This section's insertion moved every `NEXT.md` row below it down 25 lines, which took 14
  anchored `NEXT.md:1557`/`NEXT.md:1661` citations in six other units' entries (`H-0252`,
  `H-0280`–`H-0284`, `H-0286`) off their rows — `make verify` refused them at rc 1 and the repair
  is the checker's own: re-aim the numbers in the same commit, digits only, anchors re-opened
  after. That repair is why `H-0291`'s review round ships as two commits, not one.
- This push lands on a `CHANGES_REQUESTED` head. Whatever the review rows read after it, a
  re-review is owed, and the re-ask is the supervisor's poke (`factory_request_review`,
  `FACTORY_REVIEWER_TEAM`), not a unit's verb.

## What is still owed (this tick's rows first, then every carried hand-off row, none silent)

- Re-review of the round-1 pointer fix is owed: the post-push review read is quoted in the pull
  request, and the supervisor re-asks on its own schedule, bounded by `FACTORY_REVIEW_POKES`.
- The ticket's second half — the #246 row-2 shape where the tick pushed BEFORE the ceiling took
  it — is named in the 137 sentence's consequence only. The `TICK-UNDELIVERED` mirror already
  runs after every failed tick and is exercised by its own family (#364); this unit changed what
  the failure line says, not what the mirror checks. If a successor wants the two lines
  cross-referenced in the journal itself, that is a new ticket, not a leftover of this one.
- `H-0291`'s `Chose:` names the grace knob; `FACTORY_TICK_KILL_AFTER` is honoured by the one
  shipped `timeout` line. If a second ceiling ever appears (a per-unit ceiling, a budget-side
  kill), it must print through the same `case` or it re-creates #383 exactly.
- Carried from the issue-380 head below, unchanged: the `H-0289` allocator collision (the
  journalled-id-no-union-can-retire class, two timestamps) — if `issue-381` wakes before this
  branch merges, one of the two entries has to move.
- Carried: issue-380's own unit never wrote a hand-off; #459's head below is the reconstruction
  and is the reason the shape question stays open.
- Carried: `python3 scripts/check-close-verb.py NEXT.md` answers rc 1 on the two carried rows
  and this head adds no third (its own rows name issues without keywords); `check-close-verb.py
  factory/DECISIONS.md` rc 1 is trunk-carried (`closes #211`), unchanged here.
- Carried: the `install` half of #377 is still an operator act (`systemctl --user daemon-reload`
  and a unit regeneration on each installed machine), not a unit's.

## What this head pruned, and how to get it back

- Nothing. The issue-380 head is demoted whole to a Record below — a one-direction line-multiset
  census of `git show b6b8a55:NEXT.md` against this file holds exactly one missing line, the
  demoted level-1 heading itself, the same census #459's head ran. To read the pre-insert bytes:
  `git show b6b8a55:NEXT.md`.

## Record — the issue-380 conflict tick (PR #459, base sync onto `5188844`; ONE conflicted path, `factory/DECISIONS.md`, 27 hunks: 26 are the same sentence whose `path:line` anchors each parent re-pointed at its own tree and were re-aimed by the shipped gate's own refusal, 1 is the tail junction unioned by ascending id so both `H-0287` and `H-0288` ship; 72 drifted pointers moved digits-only; the pass is `H-0289`; the finding is that issue-380's own unit never wrote a hand-off at all, which the shape gate cannot see; merging belongs to the supervisor)

State read before this file wrote a byte, by this tick, raw JSON and never a `-q` template: PR #459
reads `OPEN`, `reviewDecision=APPROVED`, `mergeable=CONFLICTING`, `mergeStateStatus=DIRTY`, head
`2aa6dac7710564318a0b17b7096006576c4b2bfc`, five checks all `COMPLETED/SUCCESS` (`verify`,
`shellcheck`, `mutation sets`, `packages (amd64)`, `packages (arm64)`), and its one review row is
`hermesvicowa APPROVED` at `2026-11-19T00:44:37Z` against that same head `2aa6dac` — so this push
spends it, and the post-push read is quoted in the pull-request body rather than claimed here
(#1180). `git merge origin/main` conflicted on exactly ONE path, `factory/DECISIONS.md`, 27 hunks;
`NEXT.md`, `docs/factory.md`, `factory/common.sh` and `factory/selftest.sh` arrived from `ort`, and
`factory/factory.sh` needed no merge because trunk never touched it. The tree was clean at entry
(`git status --porcelain` empty, no `factory/HALT`), and this tick IS the `conflict:459` unit the
journal opened at `2026-11-19T01:14:19Z` in this tree, so no second writer holds it. The allocator
was asked after `git fetch --prune origin` and before the checkpoint (`H-0289`, journalled `REFRESH
28s union`, no `PENDING`, no `REFRESH-BLIND`) and `## H-0289` sat on no ref or tree this clone could
name at read time, and re-verified after `git fetch --prune origin` rather than trusted: `git grep '^## H-0289'` over every `refs/heads` and `refs/remotes` answers nothing and trunk ships 280 headings to `H-0287` — which is ALL that stands between this id and a collision, because the journal line 24 minutes above this tick's is `2026-11-19T00:50:21Z DECISION-ID H-0289 unit=unclaimed:381 branch=issue-381`, a second unit handed this same id that landed nothing (no remote ref, its local tip IS trunk's `51c87d3`), so the allocator re-issued it for want of anything readable that had taken it: the journalled-id-no-union-can-retire class with two timestamps on it. Re-read before honoring this id on any successor's authority — and if `issue-381` wakes before this branch merges, one of the two entries has to move to `H-0290`, because the allocator cannot see the other claim either way; only this file can.

## What this dispatch pushed

- One merge commit carrying `Decision: H-0289`, over one conflicted path. The tail junction is a
  union, not a choice: trunk's `H-0287` seated first, this branch's `H-0288` after it, both entries
  whole — the log ships 281 `## H-` headings, unique and ascending to `H-0288`, no duplicate, no
  marker, no line of either entry edited.
- The other 26 hunks are one defect seen 26 times: the SAME sentence on both sides whose `path:line`
  coordinates each parent had re-pointed at its own tree, so neither parent's number is true of the
  merged tree. Trunk's spelling was taken for the shared prose (the convention the carried entries
  call "trunk's body is the ground"), and every pointer the shipped gate then refused was re-aimed
  at the line the gate's own refusal named — 72 pointers across 48 rows, digits only, no row added
  or removed, offsets clustering at +1 (`factory/factory.sh`), +18 (`docs/factory.md`), +28 (one row
  of this branch's own `H-0288`) and +56 (`factory/common.sh`).
- 11 of those 72 anchors answered to MORE THAN ONE line in the merged tree, so the gate's answer was
  not a number but a set. Each was adjudicated by opening every candidate and the citing sentence —
  a call site over a comment that merely names it, `factory_journal_age`'s own `tail -n 1` over an
  unrelated one, the busy-guard `awk` over the census's `git` calls, the precondition paragraph over
  the later row that quotes it, the enumerating comment block over the shipped `printf` — and each
  then agreed with its parent's offset cluster, which is the cross-check, not the preference.
- Both intents verified rather than asserted: `check-decisions` rc 0 on these bytes (351 entries,
  1161 citations, 583 owing an anchor, 0 refused in every must-be-zero bucket, `POINTER_DEBT`
  unmoved at 583 and JUDGED again, the suspension having been the 72 themselves), and a second
  sweep written against no code of the gate's own reading 483 anchored citations and opening each
  target, which lands 480 and names three — all three artifacts of the sweep's cruder matcher (a
  possessive `'s` after the coordinate, and two bare basenames), one of which was then opened by
  hand and reads true.

## What this push spent

- One decision id (`H-0289`), one checkpoint (`24c4dd3`, the branch tip before the merge), one base
  sync (`5188844`).
- A merge commit on a permanent-history branch (both squash and rebase merge are off here), which is
  why the resolution was made by reading 27 hunks rather than by picking a side.
- No cloud spend, no third-party dependency, no live systemd object, no GitHub write before this
  file's own pull-request step.
- 72 citation re-aims — a repair, not a change of promise: every one keeps its own anchor text and
  moves only the number.

## What is still owed (this tick's rows first, then every carried hand-off row, none silent)

1. Review on PR #459: the approval this branch held (`hermesvicowa`, `2026-11-19T00:44:37Z`, filed at
   `2aa6dac`) is spent by this push, as #269 measured a conflict push spends one. There is no
   dismissal timestamp in any read a tick can make, so the push is the honest event to name. The
   poke (`factory_request_review`, `FACTORY_REVIEWER_TEAM`) owns the re-ask and a unit never asks, so
   `reviewDecision` reads `REVIEW_REQUIRED` and `mergeStateStatus` `BLOCKED` until it does.
2. The hand-off this unit is writing was never written by the unit that opened PR #459. Measured:
   `git log --oneline origin/main..HEAD -- NEXT.md` on `2aa6dac` is empty, so `issue-380` shipped a
   feature, an entry and a green gate with no hand-off of its own, and the merge therefore handed the
   supervisor trunk's head as if it were this branch's. `scripts/check-next-md.sh` leg 1 stayed green
   the whole time and CANNOT see this — it counts one level-1 heading naming *a* unit, never whether
   that unit is the one whose branch is under it (H-0286 already named the cause: "the gate reads
   structure and never authorship"). Named here and not opened: the leg that catches it is a harness
   change, and a conflict tick must not widen a gate it is standing in front of.
3. The 26-of-27 shape is #381, still open and still exact: every merge re-conflicts every open branch
   on these logs, the union is keyed by id and mechanical, and a model does it by hand. This tick
   spent the majority of its ceiling on digits. #440 is the sibling and its row above shows the
   damage: an anchor whose quote sits on several rows is accepted today, so 11 of 72 repairs here were
   a judgement the gate could have made and did not.
4. If a later conflict tick base-syncs this branch: the `H-0289` Effect quotes `check-decisions` as
   measured on THESE bytes; a union that moves the lines this entry re-aimed must re-aim by anchor,
   never by offset — the row that cites one coordinate twice is what an offset repair silently
   collapses (the slip `H-0287`'s head discloses at its own row 44 is that failure, measured).
5. Carried from the issue-377 Record below, none of it touched by this tick: its rows 1-5 stand as
   written there — the review-poke row, #377 half 1 (re-running `factory/install-timer.sh` rewrites
   and reloads the unit that drives the queue: an operator act, never a tick's, and until it happens
   the pass prints `UNIT-ENV-STALE` for the two names by design), its own base-sync warning, the
   journalled-id-no-union-can-retire class, and everything that head carries from `H-0286` — the two
   `#425` hazard rows and the `H-0252` `Follow-up:` claim resting on them, the #311/#334 operator
   waits, the exported-`FACTORY_TICK_UNIT` allocator legs and the `dash_ctl` locale row.

## What this conflict tick pruned, and how to get it back

| Dropped | Where it is whole | Why it is a record and not an instruction |
|---|---|---|
| the issue-377 dispatch head's level-1 heading (line 1 of `git show 5188844:NEXT.md`) | that blob | demoted to a Record heading below; its state read and all four of its record sections — pushed, spent, owed, pruned — are carried whole beneath it, so "this dispatch" names one unit |

Everything else the file held is carried forward unedited, including that Record's owed section and
every instruction section below the Records. No owed line was dropped: the census is the
one-direction line-multiset diff of `git show 5188844:NEXT.md` against this file, and it is printed
by the splice that wrote this head. `python3 scripts/check-close-verb.py NEXT.md` still rc 1 on the
two carried `#425` hazard rows, disclosed, unchanged: this unit edits no line another unit's claim
rests on, and it added no third.

## Record — the issue-377 unit (PR for `issue-377`: the pass now compares the generator's `Environment=` name-set against the installed unit and prints `UNIT-ENV-STALE` per missing name, or `UNIT-ENV-UNKNOWN` when either side cannot be read; the comparison lives inside the readable arm of `factory_unit_lines`; four selftest legs each carry their control; `H-0287`; the install half of #377 is an operator act and is NOT done by this unit; merging belongs to the supervisor)

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

60. **#378 left the reporting split measured, not pinned — and its derivation lives in prose.**
   `RULE_PROBES` now reads 81 report / 0 raise-only / 0 silent, measured by the instrumented
   classification `print` whose recipe is in the issue; `_rule_probes_are_live` still refuses only
   a pass in which NOTHING reports, on purpose (`D-0086`: a raise remains a legitimate witness for
   a rule whose contract IS an escaping refusal). So a future row may join the table witnessed by
   a raise alone and the suite stays green — the defence against that is this row and the
   paragraph above the table, both prose. If that becomes unacceptable, the repair is not a zero
   floor but a per-row label (`witness: raise`) that the pass counts and the table prints, so the
   split becomes visible state instead of a number in two comments. The per-rule derivation
   harness (one subprocess per rule, clean-tree baseline, word-shared candidate sweep,
   shipped-order re-score) was rebuilt for #378 from the issue's recipe and is not in the tree;
   the same instrument is what that labelling job would start from.
61. **A glued `sha` (PR #N) pair is refused by the gate from #433 on — what that gate deliberately
    does not reach, and the re-aim duty it adds to every base sync.** The rule holds each glued pair
    to the commit's own message: drift refuses, a dead object refuses, and either git question being
    unaskable fails closed rather than passing silently. What it does NOT reach: (i) class 2 — a row
    numbering itself inside `factory/DECISIONS.md`, the `this entry at 12481` shape #433 measured rot
    on the head that shipped it — because a row number has no artefact to open, and #433 refused to
    invent one; the predicate shape is item 51's row-level sibling, the decision is still open;
    (ii) a false claim phrased without the glue (`<sha>` — its landing is PR #N reads clean), because
    the glue is what makes a row assert a pair, and a looser window re-aims at whatever `PR #N` the
    sentence contains next. The duty it adds: a sync that rewrites a sentence holding a glued pair
    moves the pair — the shipped logs bind thirteen as #433 delivers, all inherited, and the refusal
    names the row,
    so re-aim in the same commit and edit the pair only where the commit's own message proves the row
    wrong. Measured at this unit: one shipped row was genuinely mis-glued (11533), 63 anchored rows
    drifted from this script's growth and all were this unit's doing (the base re-measured rc 0), and
    the ticket's own specimen plus a dead token both refused rc 1 against the real log with a
    byte-identical control restore.
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

## Record — issue-418: the reap read no longer times a killed pass by its own kill (branch `issue-418`; merging belongs to the supervisor, and the one close-verb on this work lives in the pull body)

State this pass read off the live tree before writing a byte (2026-11-20, review round): PR #484
answered `CHANGES_REQUESTED` with a review newer than head `8ff18fe`, the local tip was current
at trunk `49f7fa1`, and the two kill fixtures re-rendered the rc-2 sentence whose every colon
clause the tails refute — the control probe reproduced the review's numbers (the noise fixture's
pass line at 06:14:09.250069 inside the terminal 06:14:09.250469) before any byte changed.

State the first pass read off the live tree (2026-11-20): trunk `49f7fa1`
(#483 merged), `gh pr list --head issue-418` answered `[]`, the live journal still carried the
bug's own line — `REAP-HELD hold=0s how=deadline ended=660s open=1` at 06:25:09Z on 17 Nov —
and the two code paths were replayed over the real 16 Nov window first: trunk answers hold 0
with verdict `deadline`, the fixed read answers hold 5291 with the same verdict.

## What this branch ships

- `factory_reap_read` (factory/common.sh): for the killed verdicts `deadline` and `client` the
  hold now starts at the pass's own last line (`factory_reap_lastword`), never at the kill's
  `Main process exited`, which lands ~200 microseconds ahead of the terminal row and made an
  88-minute freeze measure `0s`; a killed pass whose last word does not beat the terminal stamp
  by a rounded second answers rc 2 — the contract's arm 2 — instead of printing the zero.
  The `clean` and `unexplained` arms keep H-0124's shape verbatim.
- `H-0329` in factory/DECISIONS.md: options, losses, and the before/after receipts; checkpoint
  37f09b2, taken before the edits.
- Three selftest fixtures transcribed from the live window (silent kill, 400-microsecond
  margin, measurable 1-second margin), two legs, and the live leg's pairing widened to both
  killed verdicts; the old leg premise ("this box's journal cannot produce a zero deadline")
  is retired because the journal produced exactly that zero on 17 Nov. Each fixture was also
  run through the trunk read beside the fixed one, so the kill legs are the bug's witness and
  not a tautology of the fix. The silent-kill fixture is the one that answers `0 deadline` rc 0
  on trunk and rc 2 here.
- The pointer census for the ~40 lines this commit inserts: 164 re-aimed rows in
  factory/DECISIONS.md (the four H-0325/H-0327 rows into docs/factory.md were already drifted
  by #481's merge and are repaired here), debt unmoved at 583, gate rc 0.

## Review round of 20 Nov — the rc-2 reason reworded (H-0331)

The #484 review measured that the single rc-2 sentence — "no `Main process exited` from systemd
and no line from the pass at or before that moment" — is false in both kill states this branch
routes through rc 2: their tails carry systemd's exit line, and the noise fixture's pass line
sits at or before the terminal stamp. The branch answers with (c) of H-0331: the enumeration is
reworded to the one fact all four rc-2 states share (no pass line beats the terminal stamp by a
whole rounded second; a killed tail's `Main process exited` stamps the ending, not the stop), the
docs bullet at docs/factory.md says exactly that, and `rp_pin_unknown` renders and pins the
reason on all three emitting fixtures — the noise leg now renders a sentence at all, closing the
width the review saw through. H-0329's Why is corrected in place: `client` is a hand-asked
SIGKILL, not a SIGABRT/SIGSEGV self-fault.

## What is still owed

- The supervisor merges; this pass merged nothing, and no artifact but the pull body carries a
  closing verb. The reviewer ask is the supervisor's poke, per AGENTS.md.
- The journal's own history is not repairable: `REAP-HELD hold=0s` rows written before this
  ships stay in the journal, so a `--status` over a window that includes 17 Nov still replays
  the old zero from the old code — re-measure on post-merge windows, not against that day.
- `make verify` inside a live tick fails one fence leg (`and nothing else does`, filed as #482):
  the tick's own OPENCODE_CONFIG_CONTENT leaks through `proc_run` into the control; until that
  lands, re-measure with `env -u OPENCODE_CONFIG_CONTENT bash factory/selftest.sh` (H-0327).
- After this merges, a later branch re-derives the pointer census with
  `python3 scripts/check-decisions.py --citations` (#389).

## Carried forward — verbatim, because the decision log is anchored into these two lines

The close-verb lesson of the inherited ledger stays on the two lines the twelve
factory/DECISIONS.md rows anchor into this file; the next rewrite of this file re-aims those
twelve rows exactly as this round did:

- The census row of the `issue-364` hand-off, carried because the rule outlives its round —
  quoted from it must name its clock. And the row about #425 says "Do not close #425 on this branch's
- The #425 instance, carried with it: neither proposed cause reproduced on the measuring tree,
  so the question stays open even though that unit measured a real
  RED on its own shell. Do not close #425 on this branch's measurement alone.

## Record — the issue-426 round (branch `issue-426`, PR open at writing time; this round ships `H-0345`, checkpoint `bf5c6cb`)

Appended at the foot rather than installed at the top, and that is a measured choice, not a shy
one: `factory/DECISIONS.md` anchors 36 pointers from 25 rows into this file at two destinations, so
a head written above line 540 moves every one of them and the re-aim lands inside other units'
`Effect:` prose — which is the work #426 told this unit not to do ("rebase your line into it, never
rewrite someone else's"). The `issue-423` head keeps its seat above, unedited; this append moves no
line anyone cites, and the two hazard rows still answer to 540 and 543.

- `.github/workflows/ci.yml`: a `concurrency:` block under `name:` — group `ci-<PR number>` for a
  `pull_request` event, `ci-<run_id>` for anything with no PR number, `cancel-in-progress` true only
  for a `pull_request`. Four pushes to one branch no longer queue four ~12-minute passes that a
  person has to cancel by hand (PR #401, 16 Nov: four runs, three cancelled mid-flight). A push to
  `main` and a `workflow_dispatch`ed debug run each land in a group of one, so nothing a person
  asked for on purpose can be stopped by a later push; `release.yml` still carries no key, on
  purpose — a cancelled half-published release is worse than a queue that makes someone wait.
- `factory/DECISIONS.md`: `H-0345` (allocator answered `H-0345` on a `REFRESH 0s union` read taken
  after `git fetch origin`), checkpoint `bf5c6cb`, effect commit `5a0f67d`. The block's 19 lines
  moved four rows in that log that aim into `ci.yml` — `H-0054` twice, `H-0266`, `H-0267` — and all
  four were re-aimed in that same commit with every destination opened on the shipped bytes first.
  The census re-run on `5a0f67d` reads 414 entries, 1341 citations, 583 owing an anchor, 0 refused:
  this round added one anchored pointer and no bare one, so `POINTER_DEBT` stayed 583 and was not
  touched.
- What this push spent: `python3 scripts/check-decisions.py`, `bash scripts/check-next-md.sh` and
  `bash scripts/check-commit-verbs.sh` on these bytes, all rc 0; the `make verify` verdict and the
  CI run this branch schedules are quoted in the pull body, never here. The group's positive test
  is one `cancelled` row from two pushes inside a single run's duration, and its receipts go in the
  pull body — an absent run is not a failed group, because `b8b36f2` is this repository's own record
  that an event can schedule nothing at all.

## What is still owed (issue-426's rows, none silent)

1. Nobody has totalled the runner-minutes a cancellation returns on this workflow. Cancellation is
   cooperative, so the saving is bounded by the gap between pushes, not by the run's length; the
   number is not measured here and no entry claims it.
2. The other two run-volume sources #425 names — a tick that cannot read a green local verdict
   pushes and re-pushes — stay open on their own authority. A `concurrency:` key settles neither.
3. The 36 anchored pointers at this file's two hazard rows still name the lines they name, because
   this append sits below all of them. Whoever rewrites the head re-aims them, and
   `python3 scripts/check-close-verb.py NEXT.md` is still rc 1 on those two rows — carried
   unchanged, disclosed, owed.

## Record — issue-424 unit (branch `issue-424`, PR open; the post-merge close check had never printed a verdict in seven measured merges — REST answers `state` lowercase while the four comparisons and every close-verb fixture carried GraphQL casing — repaired by normalising once inside `factory_close_read` (`H-0343`), re-planting the sixteen fixtures in the bytes the API sends, and dropping the merge belt's always-true `state != MERGED` clause (`H-0344`); merging belongs to the supervisor)

## What this round shipped (issue-428)

- `factory/selftest.sh`: a `run_selftest_d_legs` runner appended at the last append-safe junction (it
  runs before the tally, so leg files can never touch the summary), sourcing `factory/selftest.d/*.sh`
  at maxdepth one in `LC_ALL=C` byte order; absent or empty directory is silent-and-green. The old body
  stays inline and untouched — phase 1 only seats the runner and the first file.
- `factory/selftest.d/000-split-merge-clean.sh`: the first leg family as a file, eight legs: the
  fixture, its premise and null variants (one-sided merges clean), the #428 falsification control
  (two appended monolith tails must still CONFLICT — if this leg ever goes green the ticket's premise
  is dead), the repair sentence (two distinct leg files merge CLEAN and both land with distinct
  blobs), the kept hazard (same filename stays a visible `add/add` — no `merge=union`, per #381),
  the runner's own discovery pinned from the shipped text (sorted byte order, `*.sh`, maxdepth one,
  both quiet arms), a real merged-worktree end-to-end (both units' legs run, sorted), and the scan
  half of the migration hazard: `pos_unconfigured_repos` now also reads every leg file, with its
  control planted on a copy of the leg file itself.
- `.opencode/skills/factory/SKILL.md`: the new-leg procedure appended at EOF — a leg is a file,
  sourced mid-run, linted standalone, and a unit never migrates an existing leg.
- Decision `H-0348` over checkpoint `bd7d309`. `NEXT.md` kept its shape: this H1 replaced the
  `issue-424` head line-for-line and the demoted head rides at the foot as a Record, so the 40
  hazard-pair pointers into this file stayed exactly where they were read on 20 Nov.

## What is still owed by this round (issue-428)

- **Phase 2, un-ticketed as of 20 Nov**: move the inline legs into `factory/selftest.d/` in batches,
  each batch green in `make verify`, never rewriting an open branch. The five self-reading sites to
  carry `$BASH_SOURCE` with them were listed in `factory/selftest.sh` on 20 Nov at `:1538`, `:1718`,
  `:14923` (plus its control at `:14933`), `:14954`, and `:26603`–`:26612` — re-read them with
  `grep -n 'HERE/selftest.sh' factory/selftest.sh` before moving anything; they slide with every
  merge. In-flight branches keep appending to the old body until they merge, then move in a batch.
- **`NEXT.d/` split with its gate** (#428's second artifact): one file per claim plus pointing
  `scripts/check-next-md.sh` at the directory — deliberately not shipped here, because shipping the
  home without moving the gate would put claims where nothing reads them.
- The runner's discovery is pinned, but a leg file that prints to stdout would still land in the
  suite's verdict stream; nobody has planted one to see what the tally does with it.

## Record — #494's checks-failing round (issue-428, 2026-11-20; merging belongs to the supervisor)

- CI at head `45154ca` read `verify` and `shellcheck` FAILURE (run 44838665451), one root cause:
  the leg file this branch ships carried six shellcheck findings — 4 SC1091 (the leg sources a
  `runner.sh` it extracts into its scratch directory at runtime; no static path to follow) and
  2 SC2015 (the two quiet-arm rows inside `s000_runner_legs`). The phase-1 receipt
  `check-shell: rc 0, 33 scripts` did not hold on the pushed bytes: measured on the clean tree at
  `45154ca` the step prints `check-shell: 0.10.0 over 34 scripts` and exits 1 — a receipt that
  predates the thing it certifies; the correction is on the pull body.
- Fixed in one commit: `# shellcheck disable=SC1091` with its reason at each of the four call
  sites (the convention `factory/factory.sh`, `factory/install-timer.sh` and
  `factory/retention-census.sh` already ship), and a De Morgan rewrite of the two SC2015 arms —
  SC2015 has zero disable precedent in the repository, and the rewrite keeps both arms'
  semantics exactly (`s000_runner_legs` and `s000_merged_both_run` pass on the fixed bytes).
  Mechanical under tie-break 6, so no `H-` entry and no allocator call.
- Gate on the fixed tree with the tick env vars cleared: `make verify` ended `verify: OK`, rc 0
  (measured 20 Nov on the fixed tree; an independent second run at the same bytes printed the
  same last line).
  `./scripts/check-shell.sh` rc 0 over 34 scripts; `bash factory/selftest.sh`
  `964 passed, 0 failed`. #482 re-confirmed live this tick: with the tick env set, the suite reads
  `963 passed, 1 failed` and the single FAIL is the fence leg `and nothing else does`.
- This tick wrote nothing on GitHub beyond its own branch push and the #494 pull body; the
  post-push review state is read back raw and quoted there. Round read after the fix push:
  `CHANGES_REQUESTED` at `45154cac` (hermesvicowa, 2026-11-21T00:10:31Z, before that push) —
  blocking 1 was the six lint findings, answered by `ba92d75`; blocking 2 was the `check-shell
  rc 0 over 33 scripts` receipt in the body and in `H-0348`'s Effect, answered by the corrected
  sentences in this same round's second commit and the pull body. The re-review and the reviewer
  re-ask belong to the supervisor's poke.

## What this round shipped (issue-428 round 2, conflict tick for PR #494)

- The base sync: `origin/main` `eef3210` (the #493 merge, carrying `H-0347`) merged into `issue-428`
  as one merge commit — the only shape available here (squash and rebase merge are off, force-push
  is denied to a tick). The adjudication is `H-0350` over checkpoint `3a91850`; the interrupted tick
  had taken `H-0349` before its checkpoint and `issue-430` has since pushed that id, so this round
  yields and its entry says so (`H-0297` precedent).
- The three hunks, all unions: the `factory/DECISIONS.md` tail junction seats `H-0347` under `H-0346`
  and `H-0348` under `H-0347` in ascending id order (the gate's heading rule enforces it); the
  `factory/selftest.sh` tail keeps trunk's three baseline legs inline and seats this branch's
  `run_selftest_d_legs` runner behind them, still the last block before the tally; `NEXT.md` swapped
  its head line-for-line and both parents' heads demote in place to `## Record —`, bodies byte-kept,
  so the hazard pair moved four lines and the forty log pointers re-aimed by the gate's own refusals,
  digits only, landing on the `issue-417` Record copy; `H-0348`'s runner citation re-aimed `+108`.
- Zero-deletion census (digit-folded multiset, both parent blobs): zero absent rows both directions
  on both `factory/` files; `NEXT.md` one absent row per parent, each the spent raw head line the
  demotion re-wrote. Marker census 0 all three shapes; `bash -n` clean; `check-decisions` rc 0 with
  `POINTER_DEBT` 583 unmoved. The suite tally and `make verify` verdicts ride the pull body only
  (#1180), and the post-push review state is read back there, never here.
- This push spends the `APPROVED` read taken at `0ab46ae` — the #269/`H-0010` class; nobody has been
  re-asked: the ask is the supervisor's poke, not this unit's.

## What is still owed by this round (issue-428 round 2)

- Re-review of PR #494 after this push — the poke owns the ask; this unit asked nobody and merged
  nothing, and wrote no closing verb anywhere but `Refs` in the pull body.
- Round-1's owed items stand unchanged in the sections above: phase 2 (move inline legs into
  `factory/selftest.d/` in green batches, re-reading the self-reading sites first), the `NEXT.d/`
  split shipped only together with its gate, and the un-planted stdout-leg probe.
- Fleet carries still live: the two-word hazard-row repair (#428, #440 — the fragments still answer
  on four rows of this file), and the #482 fence poison — run the suite as CI runs it,
  `env -u FACTORY_LOG -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH make verify`, and inside a live
  tick add `-u OPENCODE_CONFIG_CONTENT -u FACTORY_SERVICE_UNIT`.

## What #457 shipped, and what is still owed (#457, 21 Nov)

- Shipped on `issue-457`: `scripts/check-close-verb.py` gained the branch leg -- a body on an
  `issue-<N>` branch that names `#N` with nothing now exits 1. A documented keyword, a
  `Refs`/`Part of` line, or any class the checker already reports counts as naming it; a bare `#N`
  does not, because a link registers with no queue. The branch is read from HEAD, `--branch NAME`
  overrides, `--branch ''` silences, and `--span-blind` reads stay branch-blind so
  `scripts/check-commit-verbs.sh` is unaffected. `factory/selftest.d/457-branch-body-claims.sh`
  runs six legs: the fixture premise (rev-parse answers 128 on an unborn HEAD, measured, so the
  fixture commits), silence refused, bare link refused, `Refs #12` on a branch claiming `#777`
  refused -- #454's specimen -- three controls green, and the override both ways. Falsified against
  a scratch copy with the refusal removed from the exit code: the four refusal legs went red while
  the prose still printed, the controls stayed green.
- Still owed, outside this unit's fence, both named in the ticket itself:
  - The supervisor-side row: triage reporting an OPEN pull whose head branch is `issue-<N>` and
    whose body names N with nothing. It touches the supervisor, which no unit owns, and #457's own
    text assigns that surface to whoever installed the timer.
  - Post-create body edits: a body that passed the checker at `gh pr create` and is edited
    afterwards to drop the reference is re-checked by nothing. The supervisor-side row is the only
    arm that catches it; H-0370 records shipping the unit-side arm alone and names these losses.
