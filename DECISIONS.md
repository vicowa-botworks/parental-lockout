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
git log --grep 'Decision: D-0011' --format='%H %s'   # the effect commit
git revert <that sha>                                 # a forward fix; no history rewrite
```

`Checkpoint:` is the commit taken *before* the decision, never after, and that order is
load-bearing: a checkpoint taken afterwards records the state the decision produced, which is the
thing you already have.

**This log starts at D-0011 on purpose.** Ids are shared across the repository rather than per
branch, because `git log --grep 'Decision: D-0011'` has to name exactly one commit and
`scripts/check-decisions.py` refuses a reused id. D-0001–D-0010 belong to the branch that installs
the decision-log gate and the `make verify` hook for it (`feat/factory-harness`, PR #217), which
was still open when the first entry below was written. When the two files meet, both halves are
kept; nothing here renumbers anything.

## Writing to this file before the gate exists

`scripts/check-decisions.py` is not in this tree yet -- the gate and the first twenty entries live on
`feat/factory-harness` (#217), still in review. This branch opened a log anyway, because a tick that
makes a judgement call and has nowhere to write it is the failure the log exists to prevent, and the
alternative -- waiting for #217 to merge -- silently exempts every branch in flight from the rule.
`issue-202` did the same thing on its own branch, which is why this file is two logs and the merge
that joined them kept both halves.

Two consequences, both deliberate:

- **The numbering continues past the highest id on any open branch.** #217's log ends at `D-0020`
  and the half above ran `D-0011`..`D-0015`, so ids were already colliding across branches; the
  entries below resume at `D-0021` rather than adding a third `D-0013`.
- **The field contract is #217's, unchanged** -- `## D-NNNN · title` followed by `Question`,
  `Options`, `Chose`, `Why`, `Checkpoint` (plus `Effect`), with `Chose` repeating one option
  verbatim. So when #217 lands, `make verify` starts checking these entries instead of rejecting
  them for shape. What it finds first is the collision this section describes: `D-0011`..`D-0015`
  will name two decisions apiece once #217's own log lands beside them, and renumbering them is that
  branch's call, not this file's.

## D-0011 · One typeface and one width model for the controls you type into

- Question: #202 measured every form control on `/settings` in Arial 13.33 px beside 16 px
  system-ui buttons, at three field widths on one page (65 px, 163 px, 185 px), with one selector
  -- the file's only input rule, `input[type="text"]` -- behind all of it. What is the shared model
  that replaces it?
- Options: `font: inherit` on `input`/`select`/`textarea` plus the existing `width: 100%;
  max-width: 22ch` model extended to every type a template types into, with `select` and the
  toggles excluded | one rule for every input, radios, checkboxes and selects included | `width:
  100%` with no `max-width`, so every field fills its column | keep `22ch` on `text` and add one
  rule per page for each of the other four types
- Chose: `font: inherit` on `input`/`select`/`textarea` plus the existing `width: 100%; max-width:
  22ch` model extended to every type a template types into, with `select` and the toggles excluded
- Why: the fourth option is the shape that caused the defect -- a list of types somebody has to
  remember -- so it loses on touch. The second loses on measurement: `width: 100%` on a radio is a
  312 px tick box at the artifact viewport, and a `select` capped at 22ch truncates a profile
  called "Long weekend during the school holidays", which is the exact failure the phone block in
  this stylesheet exists to prevent. The third loses on the reason the rule was written at all:
  the override-reason box lives in a table cell, a cell cannot shrink below the preferred width of
  its controls, and dropping the cap hands the page's width back to whichever control is widest.
  The first also takes the tie-break that follows the surrounding code: `text` already had this
  model, and `table.rows form select { max-width: 100% }` is already this file's way of clamping a
  select to the page rather than capping it. Two things the entry has to say out loud: the typeface
  and the width ship together because `ch` resolves against the element's own font, so
  `max-width: 22ch` is 163 px in Arial and 196 px in the page's 16 px -- the font alone moves every
  field without an edit to the rule that sizes it; and the type list is the positive one (the types
  you type into), so a type added to a template and missing from the list fails the capture loudly
  instead of silently inheriting `width: 100%` the way a complement list would.
- Checkpoint: 3b32af6e1d3424eb6f628575630de3b81f912e22
- Effect: internal/web/static/app.css (the `input, select, textarea` typeface rule and the
  typed-control width model, replacing `input[type="text"]`), DECISIONS.md

## D-0012 · The stylesheet reaches a tick box's words with `:has()`, not with a class

- Question: the enforcement pair, the transport trio, the seven day boxes, the all-day box and the
  enabled box all need their control kept with its own words. How does the stylesheet find them --
  by what they contain, or by a hook added to the markup at every site?
- Options: one `:has()` rule on a label that holds a radio or checkbox as its own child | a class on
  every toggle group in the templates, selected by the class | restructure the markup so each option
  is a block-level container of its own
- Chose: one `:has()` rule on a label that holds a radio or checkbox as its own child
- Why: the third is refused by what it does to the day picker -- seven day boxes want to flow and
  wrap as a grid, and one option per line is a column of seven rows. The second is the shape D-0011
  rejected for the type list, in a new place: a class is a list of sites somebody has to remember,
  and this repository already has markup hooks that nothing styles (`class="opt"` on the account
  row's labels appears in no rule at all), which is what a forgotten site looks like -- quiet. The
  `:has()` rule touches one file rather than four, and the thing a class was bought for -- a hook a
  browserless grep can pin -- is replaced by something stronger: the capture driver asks the browser
  for the computed `display` of every wrapping label on every photographed page, so a selector the
  engine drops is a refusal rather than an assumption. What the second option keeps that this one
  gives up: it would render the same way on a browser with no `:has()`. That browser gets the layout
  this file had before the rule either way, which is the same degradation the `overflow-wrap` pair
  at the top of the file is designed for, and it is not a browser any measurement in this repository
  can make.
- Checkpoint: 6743b8eb5db5cfe1cce57f47706333b68034df13
- Effect: internal/web/static/app.css (the `label:has(> input:is(…))` grouping rule), DECISIONS.md

## D-0013 · The control criterion is three refusals measured through Chrome, not a grep

- Question: a stylesheet defect is invisible to every Go test in the repository -- `check-mutations.py`
  carries no mutant in a `.css` file, and no `_test.go` renders a page. Where does the criterion for
  "every control in the page's typeface, at one width, with its tick box beside its words" run, and
  what does it refuse?
- Options: three refusals in `testing/capture/cdp.py`, measured through Chrome on every photographed
  page, wired into all four drivers | a grep in `internal/web` pinning the new selectors' presence | a
  per-control second viewport in `pages.py`, comparing each control's box at 360 px against 1024 px
- Chose: three refusals in `testing/capture/cdp.py`, measured through Chrome on every photographed
  page, wired into all four drivers
- Why: the grep was already tried by this repository and its limit is written down in the file that
  holds it -- `TestTheStylesheetPrintsTheNarrowLabels` pins `overflow-wrap: anywhere` with the comment
  that a grep "pins presence, not effect", because the value that carries the desktop half of #140 is
  watched by nothing that measures it. Pinning three more selectors by grep would add three more
  strings that pass on a rule the browser refuses to understand, which is exactly the failure mode of
  the `:has()` selector D-0012 chose. The second viewport answers a question #202 does not ask: the
  issue's measurements are a font mismatch and a width model, not an overflow above 360 px, and #149
  already owns that axis. A computed-style read is the only thing that can see either half of what is
  actually at risk here -- a `:has()` rule a browser drops without a word, or a type missing from a
  selector list -- because it reads what the engine applied rather than what the file claims. The cost
  is stated rather than hidden: a criterion that needs Chrome is not a criterion on a machine without
  it, so the grep is kept as well, in the same commit as the rules it pins, as the net between a live
  capture and a green build; and on the two pages only `lock.py` and `testlock.py` photograph, the
  guards pass having found one control each -- the chrome's Sign out button -- which is why the census
  counts print on every measurement line instead of a silence a reader has to trust.
- Checkpoint: bf2928c0711710f668d22cdf1182f37b6c7f8fa1
- Effect: testing/capture/cdp.py (`CONTROLS_JS`, `require_control_font`, `require_control_width`,
  `require_choice_groups`, `census_line`), testing/capture/pages.py (the splice, the three calls, the
  transcript, seven self-test rules), testing/capture/lock.py, testing/capture/testlock.py,
  internal/web/narrow_width_test.go, DECISIONS.md

## D-0014 · The nine frames are re-photographed in the commit that changes what they show

- Question: the stylesheet fix changes what six of the nine committed 360 px frames depict, and
  `manifest.py check` -- a step of `make verify` -- now refuses the tree because every page's pinned
  source revision moved. Does this branch ship the stylesheet with the old evidence under it, or
  re-photograph the artifacts?
- Options: re-record all nine here and commit the new bytes beside the rule that refuses the old ones
  | ship the stylesheet alone and leave the frames for a later unit | re-record only the six whose
  pixels moved and leave the other three pinned to their old sources
- Chose: re-record all nine here and commit the new bytes beside the rule that refuses the old ones
- Why: the third option is not a state the gate can hold -- `check` walks every entry and every
  page's sources have moved, so a half-recorded manifest is a refusal, not a shortcut, and a manifest
  that certifies two different stylesheets under one `captured_at` records nothing. The second is the
  state this repository spent three issues escaping: D-0010 measured that the nine committed frames
  did not reproduce, and the reason #202 stayed open for nine commits is that the only artifacts which
  could show the wrong font were the ones nobody re-photographs. What the first option costs, stated
  rather than glossed: nine committed bytes are invalidated as a side effect of a CSS change, and the
  frames are the very evidence that failed to catch the defect -- so the same act that makes the
  criterion's evidence agree with the fix is also the act that would have hidden a second defect in
  the same commit, which is why the guard landed first (D-0013) and the frames were taken from a tree
  that already refused the old look. Two more things the record owes: `head` in the new manifest names
  the checkpoint commit rather than the commit carrying the frames, because a capture cannot pin its
  own commit -- already true of every manifest on main; and three of the nine come back byte-identical
  (`lock.png`, `testlock.png`, `users-no-profiles.png`). This entry does not explain, frame by frame,
  why a given page's pixels moved: a font change re-metrics the page around a control as well as the
  control, so the mapping from the census to the bytes is not one-to-one, and inventing a reason per
  frame would be worse than leaving it open. What is on the record is the census, printed per page per
  viewport, which is what a reader can check the manifest against.
- Checkpoint: 8b33c5045f2af647d30b0c07f53a6f3d9d72ee1b
- Effect: docs/screenshots/360/ (six PNGs re-taken, two unchanged, manifest.json),
  testing/capture/README.md (the three refusals in the table, the census section, the rule count),
  DECISIONS.md

## D-0015 · A conflicted frame is re-photographed under both drivers, not chosen between

- Question: `origin/main` (3b2f594) and this branch each re-photographed the same five 360 px frames
  and each spliced a new census into `shoot()` -- #202 the three control claims, #207 the declared
  state and the audit row count. `git merge` reports CONFLICTING on six paths (`audit`, `dashboard`,
  `profiles`, `profiles-refused`, `users`, `manifest.json`) plus a content conflict in
  `EXPECTED_RULES`. Which side's bytes, and which side's checks?
- Options: this branch's five frames with main's guards spliced in | main's five frames with this
  branch's guards spliced in | one side taken wholesale, code and artifacts together | re-photograph
  the set against the merged tree with both censuses wired in
- Chose: re-photograph the set against the merged tree with both censuses wired in, and make
  `EXPECTED_RULES` the union -- 36 + 7 (#202) + 2 (#207) = 45, one tally comment instead of two
- Why: the first two options are the same mistake aimed at opposite ends of the criterion. Main's
  five frames are the pre-#202 stylesheet -- controls in the browser's own Arial beside 16 px
  system-ui, `/settings` sizing its fields three ways on one page -- so committing them re-files the
  exact defect #202 was opened for under the filenames the criterion calls its evidence. This
  branch's five are the pre-#207 fixture: a trail seeded at `datetime.now()`, a Sessions cell read
  from this machine's `loginctl`, a row count that is whatever the file happened to hold -- the three
  conditions `require_declared_state` and `require_declared_rows` now name and refuse. Either choice
  commits bytes the driver in the same commit cannot reproduce, and "a screenshot with no driver
  behind it is an assertion nobody can re-run" is this folder's founding sentence (#108, restated by
  D-0010). The measurement settled it before the tie-break was needed: none of the five came back
  from the merged driver matching either side -- all five moved to new bytes. What proves the re-take
  faithful rather than merely new is the control it produced: `settings.png`, the one frame this
  branch moved and main did not, came back byte-identical to this branch's committed file
  (`111bcbe1…`), so the merged fixture's clock, census, reader zone and declared state did not move
  that page, and each side's movement stays attributable to the change that caused it. The tie-break
  order says the same thing from the other side: nothing separates on `go.mod` (neither side adds a
  dependency) or on files touched (six conflicted paths either way); "refuses rather than proceeds"
  picks the re-take, because it is the only option under which both new refusals are live over the
  committed bytes; and the surrounding code already re-takes rather than re-labels -- #207 re-took six
  frames for a fixture change, D-0014 for a stylesheet change.

  What this costs, stated rather than glossed. The manifest re-records all nine entries, so
  `lock.png` and `testlock.png` -- not re-taken here, because re-taking them locks the capturing
  machine's session for real and that is #213's decision, not a step in this one -- come out with
  `sources_rev` at this branch's stylesheet commit over bytes that are still pre-#202 and still taken
  with page scripts running. `manifest.py check` is blind to exactly that: `record` writes the newest
  commit over the sources into `sources_rev`, and `check` returns early when `recorded == current`
  (`testing/capture/manifest.py:360`), so re-recording a manifest launders an untouched frame's date.
  A green `screenshot manifest` step after this merge is therefore no evidence about those two, and
  nobody should read it as such. Two more things belong on the record: the merged `shoot()` runs
  main's state and row-count guards on the artifact pass and the three control claims at both
  viewports, and both fired on the real run -- `trail: 1 appended row(s) re-stamped to
  2026-03-18T21:34:05-04:00; 8 rows, the fixture's declared count`, `reader zone:
  TZ=America/New_York for the daemon`, `/profiles … controls=42 typed=14 groups=16` -- so the union is
  wired and not merely concatenated; and the README's re-capture recipe predates #207's
  one-row-per-fixture rule, which is why this merge re-seeds before it takes (typed without its
  `pages-setup.sh` the driver exited 1 on `this fixture declares 8 rows on the page and the trail
  holds 9`).
- Checkpoint: 6c497cba8f8ea343cf5add0611d26f2b19a38efb
- Effect: docs/screenshots/360/ (the five conflicted frames re-taken against the merged tree, four
  unchanged, `manifest.json` re-recorded), testing/capture/pages.py (`EXPECTED_RULES` 45 and the
  merged tally comment), testing/capture/README.md (the rule count, the re-seed step in the
  re-capture recipe), DECISIONS.md

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
## D-0023 · The `/audit` frame that belongs to no branch is photographed, not chosen between

- Question: #215 is opened against `fix/audit-row-label`, not `main`, and both of those moved the
  same artifact. `origin/main` re-photographed `audit.png` against the fixed stylesheet on the
  template that still prints the reader's zone *name* (`c304336d…`); `fix/audit-row-label`
  re-recorded it against the template that prints the row's own *offset* on the pre-#202 stylesheet
  (`a1f19fca…`). Merging both leaves a tree whose `/audit` prints `2026-03-18 21:34:05 -04:00`
  inside the #218 stylesheet, and no committed PNG of it. Which bytes does the merged branch commit?
- Options: main's frame | the base branch's frame | either frame with `manifest.py record`
  re-pointed at it | re-photograph `/audit` against the merged tree and record that run's conditions
- Chose: re-photograph `/audit` against the merged tree and record that run's conditions
- Why: the gate answered before the tie-break was reached. `manifest.py check` on the merge commit
  (`1c5a109`) returned two problems, the first of which quotes the mismatch and names the repair --
  `audit.png records the state [['auth.success', '2026-03-18 21:34:05 EDT']] and the driver now
  declares [['auth.success', '2026-03-18 21:34:05 -04:00']] for /audit … re-take the frame rather
  than editing this entry to match` -- and the second is #144's drift sentence for the same frame.
  The two committed candidates are the same mistake aimed at opposite sides of the page: main's
  predates #214's template, so committing it certifies a When cell the merged template cannot
  render; the base branch's predates #218's stylesheet, so committing it re-files under an evidence
  filename the exact control rendering #202 was opened for. Re-recording either is worse than wrong,
  it is silently green: `record` recomputes `sources_rev` and `check` returns early when
  `recorded == current`, which is the laundering D-0015 already put on the record.
  The measurement then settled what the refusals implied. `./pages-setup.sh && ./pages.py` on the
  merged tree came back `6/6 pages fit at 360 px` and `6/6 … at 1024 px`, and `audit.png` arrived as
  `a113c5e4…` -- `360x3086`, the same canvas as main's frame and different pixels -- matching
  neither candidate, so the page this merge renders had genuinely never been photographed. What
  proves the re-take faithful rather than merely new is the other five: `dashboard`, `users`,
  `profiles`, `profiles-refused` and `settings` came back **byte-identical** to main's committed
  files (`b2c90782…`, `55120c43…`, `48e10e29…`, `17f76100…`, `111bcbe1…`), so the merged fixture's
  clock, census, reader zone and declared state moved none of them and only the template moved the
  sixth. The tie-break order agrees: nothing separates on `go.mod` (neither side adds a dependency)
  or on breadth (one PNG either way); "refuses rather than proceeds" picks the re-take, because it
  is the only option under which the driver's declared-state refusal is live over the committed
  bytes; and the surrounding code already re-takes rather than re-labels -- D-0014 for a stylesheet
  change, D-0015 for exactly this conflicted-frame shape, `ac20479` for the re-take itself.

  What this costs, stated rather than glossed. One frame is re-committed, so the other five
  page-fixture frames keep main's bytes *and* the `capture` block #215 recorded for the bytes they
  already had -- acceptable only because the fixture's own declaration did not move between the two
  runs (`fixture.sh` and `pages-setup.sh` changed in comments on both branches, and `record` said
  nothing on stderr about the eight it did not photograph), and it is the kind of carry-over that
  went wrong once before on this very file. `lock.png` and `testlock.png` keep their blocks and stay
  #213's: not re-taken here, because re-taking them locks the capturing machine's session for real.
  `sources_rev` for `/audit` now names the merge commit `1c5a109`, because that is the newest commit
  `git log` will show for `audit.tmpl` -- a merge, not the commit that wrote the template, and a
  reader looking for the offset change still wants `c82baa6`. The header's `head` is the checkpoint
  below, which is what `record` can see at the moment it writes.
- Checkpoint: cfad6e5cae88bfb2ffe96ec8191775926008aeed
- Effect: docs/screenshots/360/audit.png (re-photographed against the merged tree),
  docs/screenshots/360/manifest.json (that entry's `png_sha256`, its declared-state cell `EDT` →
  `-04:00`, its `sources_rev`, and the file header), DECISIONS.md

## D-0024 · The conflicted audit frame is re-photographed against the merge, and the merge commit is its provenance

- Question: `origin/main` (through #218, the #202 stylesheet) and this branch (#214, the offset
  template) each re-photographed `docs/screenshots/360/audit.png` and each rewrote that page's
  `manifest.json` entry, so `git merge origin/main` comes back CONFLICTING on those two paths plus an
  add/add in this file. Which bytes does the merged branch commit, and what does the manifest say
  about them?
- Options: main's frame | this branch's frame | either frame with `manifest.py record` run over it so
  the manifest stops refusing | re-photograph `/audit` against the merged tree and record that run |
  re-photograph the whole nine-frame set, as D-0015 did
- Chose: re-photograph `/audit` against the merged tree and record that run
- Why: neither committed candidate is a photograph of the tree this merge produces. Main's
  (`c304336d…`) is the #218 stylesheet over the template that still prints the reader's zone *name*,
  so committing it certifies a When cell that no longer exists in the tree beside it; this branch's
  (`a1f19fca…`) is the offset template over the pre-#202 stylesheet, so committing it files the exact
  control rendering #202 was opened for under the filename that rendering's own evidence is supposed
  to disprove. Re-recording either is worse than wrong -- `manifest.py check` on this schema is
  *blind* to the swap, and the reason is worth having written down: the drift rule is an ancestry
  test, main's `audit.png` was taken at `ac20479`, and `ac20479` is a descendant of every commit that
  touches the page's declared sources, including this branch's `c82baa6`. So main's frame passes the
  gate at the merge while depicting a page the merge cannot render. The refusal that would have
  caught it -- the manifest comparing the recorded declared state against the driver's -- is what #215
  adds; it is not on `main`, so the gate could not be the tie-breaker here and the re-take had to be
  chosen for the reason D-0015 gives: a screenshot the driver in the same commit cannot reproduce is
  not evidence.
  The measurement then confirmed the choice rather than decorating it. `./pages-setup.sh && ./pages.py
  --out docs/screenshots/360 --overwrite` on the merge commit `5c63fb9` printed `6/6 pages fit at 360
  px` and `6/6 … at 1024 px`, with both sides' guards live in the one transcript -- `trail: 1 appended
  row(s) re-stamped to 2026-03-18T21:34:05-04:00; 8 rows, the fixture's declared count` (this branch's
  declared state, whose needle is the offset) and `/profiles … controls=42 typed=14 groups=16`
  (#202's control census) -- and `/audit` arrived as `a113c5e4…`, `360x3086`, matching neither
  candidate. What proves the re-take faithful rather than merely new is the other five: `dashboard`,
  `users`, `profiles`, `profiles-refused` and `settings` came back **byte-identical** to main's
  committed files (`b2c90782…`, `55120c43…`, `48e10e29…`, `17f76100…`, `111bcbe1…`), so the merged
  fixture's clock, census, reader zone and declared state moved none of them and only the template
  moved the sixth. Then an independent check from the other direction: #215, which is stacked on this
  branch, merged `fix/audit-row-label` into itself at `1c5a109` and measured the same page at
  `a113c5e4…`. Two different merge orders, one digest -- the frame is a property of the two changes
  together, not of either run that took it.
  The fifth option is what D-0015 did to the same shape and it is refused here on measurement, not on
  economy: the five frames `main` moved are byte-reproducible under the merged driver *exactly as
  committed*, which is what the run just demonstrated, so re-committing them would move their
  `sources_rev` to say something nobody learned. `lock.png` and `testlock.png` stay #213's -- re-taking
  them locks this machine's session for real.
  Provenance was the part with a trap in it, so it is stated twice. `record()` takes `head` from
  `git rev-parse HEAD`, and a capture run *during* the merge would stamp `2a730aa` -- the checkpoint
  below, whose tree is this branch's old stylesheet. That is the unusable-by-construction case D-0021
  exists for, and the fix was ordering rather than a hand-typed field: the merge was committed first,
  so `head` and `/audit`'s `sources_rev` both name `5c63fb9`, a real commit on this branch whose tree
  is the tree the driver actually rendered. #215 could not do that and recorded its pre-render
  checkpoint instead; it says so in its own entry.
  The tie-break order agrees with the measurement: nothing separates on `go.mod` (neither side adds a
  dependency) or on breadth (one PNG either way); "refuses rather than proceeds" picks the re-take,
  because it is the only option under which `require_declared_state`'s `-04:00` needle is live over
  committed bytes; and the surrounding code already re-takes rather than re-labels -- `ac20479` for a
  stylesheet, D-0014 for the commit that ships one, D-0015 for exactly this conflicted-frame shape.
- What this costs, stated rather than glossed. (1) The merge commit `5c63fb9` on its own carries
  main's frame over the offset template, and `make verify` says nothing about it -- that is the
  blindness described above, not a pass, and the sentence belongs to whoever reads that commit. (2)
  `captured_at` is one field for the whole manifest: it now reads `2026-11-06T00:52:33Z`, which is
  true of the six pages the `pages` fixture photographed and false of `lock.png`, `testlock.png` and
  `users-no-profiles.png`, whose bytes this run neither took nor changed. That is the same
  single-timestamp-for-nine wart D-0015 put on the record, and this commit is on the side that made it
  slightly wider. (3) `/audit`'s `sources_rev` now names a merge, not `c82baa6`, so a reader hunting
  the offset change still wants `git log -- internal/web/templates/audit.tmpl`. What did *not* happen
  this time is worth saying next to D-0015's confession: the re-record left the other eight entries'
  `sources_rev` at `125b1a4`, so no untouched frame had its date laundered.
- Checkpoint: 2a730aaf2a64d502e27d3bf61e8c4ac33a231e37
- Effect: docs/screenshots/360/audit.png (re-photographed against `5c63fb9`),
  docs/screenshots/360/manifest.json (`head`, `captured_at`, `/audit`'s `png_sha256` and
  `sources_rev`), DECISIONS.md

## D-0025 · The scripts condition reaches the browser, and a frame it moves cannot be committed

- Question: #215's review asks for the discriminating test to be written into the two POST drivers'
  notes, because as written they "teach the next reader to trust a control that has no power". Before
  writing that sentence: does `PLOCK_CAPTURE_SCRIPTS` reach the browser at all? `grep -n SCRIPTS
  testing/capture/cdp.py` answers it -- the file names the env key nowhere, and the session is
  configured with `Emulation.setScriptExecutionDisabled, value=True`. So the override
  `post-fixture.sh:47-48` advertises ("Override with `PLOCK_CAPTURE_SCRIPTS=on` to test a frame taken
  with scripts running; the declaration follows the override into the manifest") took the same pixels
  as the default, and the manifest's `scripts` field recorded what `cdp.py` does rather than what the
  frame was taken under.
- Options: write the test into the notes as advertised | retract the override in prose and point the
  reader at a hand edit to `cdp.py` | make `cdp.py` honour the declaration, refuse a value outside the
  two, and refuse to *commit* a frame recorded `scripts: "on"`
- Chose: make `cdp.py` honour the declaration, refuse a value outside the two, and refuse to commit a
  frame recorded `scripts: "on"`
- Why: the first option is refused by the measurement above. A test that cannot produce a second
  outcome is not a discriminating test, and writing it into two notes would have re-planted #213's
  defect one layer down -- a field nobody can check by going and checking it. The second leaves the
  fixture declaring something the driver ignores, which is exactly how the field became a restatement
  of a constant. The tie-break order does not separate the second from the third on `go.mod` or on
  breadth (three files either way); "refuses rather than proceeds" picks the third, because it is the
  only option under which a typo'd condition stops the run instead of being recorded as written and
  photographed as the default; and the surrounding code already treats `fixture.env` that way --
  `_start` refuses a relative `PLOCK_CAPTURE_PATH_PREPEND` and `declared_stamp()` validates the
  instant rather than pasting it.
  What this costs, stated rather than glossed. A frame taken with the scripts running now exists, so
  the knob had to arrive with a guard: `check` refuses a committed entry recording `scripts: "on"`,
  because this folder's criterion is "usable at 360 px with JavaScript off" and such a frame has never
  tested it. That guard and the driver both needed pins in files no compiler ever opens:
  `pages.py --self-test` 47 -> 48 (reads `cdp.scripts_disabled` and `cdp.Capture`, and refuses the
  switch going back to a literal) and `manifest.py --self-test` 39 -> 40 (refuses a scripts-on frame
  green, and refuses the same block with `off` green too, so the guard cannot be a ban on the field).
  Both were hollowed and re-run before being declared: `value=scripts_off` -> `value=True` names the
  pages rule, deleting the `elif scripts == "on":` arm names the manifest rule, and `raw = None` in
  the helper names it as well -- the last of those is why the needle is `env.get("PLOCK_CAPTURE_SCRIPTS")`
  and not the bare key name, which survives in prose. One vocabulary now crosses a module boundary:
  `manifest.py script_values()` reads `cdp.SCRIPT_VALUES` instead of restating the pair, so a `check`
  that cannot import `cdp` refuses instead of answering, which is the same trade `driver_pages()`
  already makes. `README.md`'s scripts section is rewritten by D-0026 with the measurements this knob
  makes possible, because until this commit the claim it makes was not checkable.
- Checkpoint: d4d9c044b6cbeac8621ebd0c9e93999d01b527bb
