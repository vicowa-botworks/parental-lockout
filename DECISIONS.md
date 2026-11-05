# Decision log

Every judgement call an unattended session made in this repository, with the options it was given
and the commit to roll back to. Written by the `factory` agent (`.opencode/skills/factory/SKILL.md`)
to the contract `scripts/check-decisions.py` enforces -- an entry that is missing a field, lists one
option, or names a checkpoint that `git revert` cannot be pointed at.

The format exists because of how an autonomous loop fails. Not by choosing badly -- by choosing and
not saying so, so that six weeks later the only trace of a real decision is code that nobody can
explain and an issue that says nothing. An entry is therefore only useful if it records what lost.

Rolling one back:

```bash
git log --grep 'Decision: D-0021' --format='%H %s'   # the effect commit
git revert <that sha>                                 # a forward fix; no history rewrite
```

`Checkpoint:` is the commit taken *before* the decision, never after. That order is load-bearing: a
checkpoint taken afterwards records the state the decision produced, which is the thing you already
have.

## Writing to this file before the gate exists

`scripts/check-decisions.py` is not in this tree yet -- the gate and the first twenty entries live on
`feat/factory-harness` (#217), still in review. This branch opens a log anyway, because a tick that
makes a judgement call and has nowhere to write it is the failure the log exists to prevent, and the
alternative -- waiting for #217 to merge -- silently exempts every branch in flight from the rule.

Two consequences, both deliberate:

- **The numbering continues past the highest id on any open branch.** #217's log ends at `D-0020`
  and `issue-202` has opened its own log at `D-0011`..`D-0015` for unrelated decisions, so ids are
  already colliding across branches; new ones start above every number in flight rather than adding
  a third `D-0013`.
- **The field contract is #217's, unchanged** -- `## D-NNNN · title` followed by `Question`,
  `Options`, `Chose`, `Why`, `Checkpoint` (plus `Effect`), with `Chose` repeating one option
  verbatim. So when #217 lands, `make verify` starts checking these entries instead of rejecting
  them for shape.

## D-0021 · The manifest's provenance is regenerated through the tool's own seam

- Question: `docs/screenshots/360/manifest.json` records `"head": 2d60366489…` and the `/audit` row's
  `sources_rev: 2d60366489…`, and that object is in no branch and not served by the remote -- the
  1 Nov rebase renamed it two and a half minutes after `manifest.py record` wrote it into the file.
  How does the file get provenance a reader can resolve?
- Options: `manifest.py --dir docs/screenshots/360 record` with its defaults, on the final tree |
  point both fields at `c82baa6` in the JSON by hand | call `record()` through its existing
  `head=`/`now=` seam with the commit the pages were last rendered from and the instant the frames
  were actually taken
- Chose: call `record()` through its existing `head=`/`now=` seam with the commit the pages were last
  rendered from and the instant the frames were actually taken
- Why: the first option was what the review recommended and it loses on its own documented rule.
  `record()` with no arguments takes `head` from `git rev-parse HEAD` and `captured_at` from
  `_utc_now()`, so running it today re-stamps nine frames that were photographed at
  `2026-11-01T23:03:53Z` with today's clock, and points `head` at the tip of a branch whose newest
  commits touch nothing a page renders from -- provenance for a run that photographed nothing, which
  is the rule #215 states in its own body ("provenance is what this run photographed, not what
  changed"). It would fix the unresolvable sha by making a different field untrue. The second option
  is refused because `manifest.json` is what `record()` writes: a value typed into it is gone at the
  next re-record, and a hand-typed sha is the exact object that went wrong here -- the file had to
  stop being something a person adjusts. The third keeps the tool as the only writer: the seam is
  already there for the self-test (`record(directory, head=None, run=git, now=None)`), so nothing is
  special-cased, and the two inputs are the two facts being asserted. Measured on this tree: the diff
  is two lines, `captured_at` and the other eight rows' `sources_rev` and all nine `png_sha256` come
  back byte-identical, and `manifest.py check` prints `9 photographs current`. `c82baa6` rather than
  the push-time tip because it is the fact the field is for: of the nine rows eight already record
  the newest commit touching their sources and every one of them is an ancestor of `c82baa6`, which
  is both the newest commit touching any declared source and the commit that changed what `/audit`
  renders. It resolves from any clone once this branch merges, and -- the part the phantom lacks --
  it is not a commit that a later rebase of *this* branch can rename away, which is how the bad value
  was minted in the first place.
- Checkpoint: 230d7f6a8340986e6f655e4fcc47691c060fd0ee
- Effect: docs/screenshots/360/manifest.json (`head`, and `/audit`'s `sources_rev`)

## D-0022 · The When cell gets two mutants, one per assertion

- Question: #214's review measured that `scripts/mutations/web.json` has three mutants on
  `audit.tmpl` (W157, W158, W159) and none of them touches the When cell, so this branch's decision is
  pinned by exactly one file, `internal/web/audit_row_zone_test.go`. Does a decision whose only pin is
  a test file need a mutant?
- Options: nothing -- the review calls the mutant optional and the test is the pin | one arm, putting
  `MST` back on the human-readable cell | two arms, that one and one putting a zone name in the
  `datetime` attribute
- Chose: two arms, that one and one putting a zone name in the `datetime` attribute
- Why: the first option is the one this repository has already been burned by. A mutant set proves
  which bugs the suite catches; it cannot notice a decision that has no mutant, and
  `scripts/check-mutations.py` runs every day on exactly that blind spot (memory #1048: a renamed or
  deleted anchor retires a pattern silently and the suite stays green). Deleting the new test file
  retires #210 the same way and nothing in `make verify` says so; a row in the set is the only place
  that claim survives the file it was written next to. The middle option was the review's suggestion
  and my first draft of the row, and it lost on measurement: that draft's note asserted the attribute
  "is killed by the same `want` in the same subtests, so a second row would add a count, not a
  killer", and probing it showed the assertion is a different one -- `attr !=
  "2026-03-18T21:33:05-04:00"` -- which fires with the cell text untouched. So the second arm has its
  own killer and the first arm's rationale was false as written; declaring both also matches the set's
  own practice for a two-sided condition, where W285 and W295 are declared as the two halves of one
  `&&` rather than inferred from each other. Both measured before being declared: `mutate.py --only
  W298,W299` returns `CAUGHT` for each, and the same layout edit applied by hand fails
  `TestTheAuditRowIsStampedTheSameWhoeverReadsIt` and all four of its zone subtests. What this costs:
  the set's tally moves 287 -> 289, so any branch quoting 287 is now wrong -- #215 is stacked on this
  branch and its body does -- and both rows are anchored inside the When cell, which is the region
  #1048 calls fragile: pull that expression into a template variable and both patterns match zero
  times, reported as NOT APPLIED, which is a build-shaped complaint rather than the lost decision it
  would be.
- Checkpoint: 39b60988ad0cb2361e98c6ff64a45089860b8475
- Effect: scripts/mutations/web.json (`W298`, `W299`, `expected_mutants`)
