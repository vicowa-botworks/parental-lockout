# NEXT — rescue-pre-h0234-reword (branch `rescue/pre-h0234-reword`, PR open against `main`; this unit ships no code. It assesses the stranded branch the queue reported as `branch_without_pr` and files the verdict on a pull request, per the rule that a dropped branch still needs one — a drop recorded only in a local log is indistinguishable from a branch never attempted. Verdict: **drop**. The branch is the pre-reword rescue of #334's journal-retention work, and that work is already on `main`: the same two subjects shipped as `6b7928c` and `4e19c12` via PR #431 (merge `449d6e7`), carrying decision `H-0234` and the `H-0235` hardening of the same legs. Gate verdicts ride the pull-request body (#1180); merging belongs to the supervisor and this unit merged nothing.)

## What this unit shipped

- The assessment, measured line-by-line rather than read off the commit subjects: of the 314
  non-blank lines the branch adds over its merge base (`95fb410`, the `pre-H-0234` checkpoint),
  299 are present verbatim on `origin/main` and the 15 that differ all have a named successor on
  trunk — the coordinate re-stamps (`docs/factory.md:1229`→`:1238`, `:874`→`:883`), the tightened
  tick census (171 released ticks of 199 starts, the 28 unreleased named), the `H-0203` incident
  record replacing "the 13 Nov incident", and the `tail`-pasted `earned-tail` mutant replaced by
  `H-0235`'s two-line plant with operand and shape guards. Nothing trunk lacks lives here.
- This hand-off file. The two branch commits (`9847ca5`, `96e40c1`, both 16 Nov) are unmodified by
  this unit — no production, docs or test byte was touched.

## What is still owed

- A human's hand on this pull request: the recommendation is to close it unmerged. The branch's
  three-dot diff adds 342 lines trunk already holds in reworded form, while the branch tree lacks
  9,905 lines trunk has grown in the two files since `95fb410`; a merge would conflict in both
  files and every possible resolution is a regression.
- #334 itself stays open and escalated (`H-0188`): the number, the clock, the zone, and which
  generations survive are still a person's to give. Nothing on this branch answers them and
  nothing in this assessment does either.
- Whoever acts on the close should confirm the `branch_without_pr` row leaves the queue once the
  ref carries a closed pull request; the ref itself is then a delivered-history name, not work.
- Trunk's live hand-off (the `issue-511` `NEXT.md`, read at `d4b2c93`) is untouched by this branch
  and stays authoritative — this file lives on a branch recommended for closing, so the one thing
  it must never do is merge over that one.

## Constraints a stranger would break

- Do not mine this branch for anything "lost": its two commits are the pre-reword pair of trunk's
  `4e19c12`/`6b7928c`. The content a reader of #334 actually wants — clause 5's clock-and-zone
  refusal and the read-only retention-census tool trunk grew later — exists only on trunk, never
  here, and was written after this branch was stranded.
- Do not sync this branch with `main` to make it mergeable: that manufactures a PR whose diff would
  be indistinguishable from trunk's own content and hides the drop verdict the body carries.
