# NEXT — issue-416: the cortexkit marker is untracked, so the main-checkout restore arm keeps its full semantics and loses its only permanent subject (branch `issue-416`, Refs #416; merging belongs to the supervisor, and the one close-verb on this work lives in the pull body)

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
