# NEXT — issue-418: the reap read no longer times a killed pass by its own kill (branch `issue-418`; merging belongs to the supervisor, and the one close-verb on this work lives in the pull body)

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
