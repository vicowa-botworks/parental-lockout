# Decision log

Every judgement call made unattended in this repository, with the options it was given and the
commit to roll back to. Written by the `factory` agent (`.opencode/skills/factory/SKILL.md`).

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
