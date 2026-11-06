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

## D-0016 · The page's own name is the heading, and the product's name is a label above it

- Question: `internal/web/templates/page.tmpl` renders the site's one `<h1>` and it held the
  product's name, so no heading anywhere in the site answers "which page am I" (#219). The issue
  offers two defensible answers and calls the present state neither of them: make the `<h1>` the
  page's own name and demote the product's name to a non-heading, or leave the product's name in the
  `<h1>` and record that "which page am I" belongs to `aria-current` plus `<title>`. Under the first,
  where do the heading's words come from -- the page's existing `title` block, a new `heading` block
  per page, or the `pagename` slug?
- Options: page name in the `<h1>`, brand demoted in the header | product name stays the `<h1>` and
  the answer is documented as belonging to `aria-current` and `<title>` | under the first: `title`,
  a new `heading` block, or `pagename`
- Chose: the page's own name in the `<h1>`, rendered from the same `title` block `<title>` is built
  from -- one definition, two places. The brand line stays in the header as
  `<p class="masthead__name">`, not a heading and not a link. `pagename` stays what it was: the
  body's class.
- Why: the second option is a documentation change wearing this issue's clothes. It leaves the one
  heading every page shares answering the wrong question and adds a sentence saying that was meant;
  #219's consequence 2 is the reason that does not close, because it is about `<title>` being
  unreachable once the page has loaded, which is a property of the markup and not of the record.
  Consequence 1 is the same fact from the keyboard: the first heading of five pages is the same two
  words.

  The sub-question was settled by the templates before any tie-break was reached. `title` is already
  each page's name on all seven of them -- the shell writes
  `<title>{{template "title" .}} · parental lockout</title>` and the pages define "Status",
  "Accounts", "Audit trail", "Lockout profiles", "Settings", "Lock an account", "Check the lock
  backends" -- so a heading drawn from that block cannot drift from the tab strip, and drawing it
  from anything else creates a second definition of a page's name whose only failure mode is that
  the two disagree in exactly the places nobody re-reads. A new `heading` block is that second
  definition. `pagename` is a slug, not a name: it says `profiles` where the page says "Lockout
  profiles". The guard asserts the heading against the `<title>` rather than against a list of
  strings, so what is pinned is the relationship and not seven strings somebody could re-name in one
  of the two places.

  Tie-breaks: nothing separates them on `go.mod` (neither adds a dependency). Files touched favours
  the chosen side -- one line of shell markup and the login page's `<h1>`, against a `docs/` naming
  discussion and an `aria-current` story that is #204's scope. "Refuses rather than proceeds" picks
  the same side, because under the first option the decision comes with a check that can fail and
  under the second it comes with a paragraph.

  What lost, stated rather than buried. The brand stops being a link: it was
  `<h1><a href="/">parental lockout</a></h1>`, so the header carried two anchors with `href="/"` and
  now carries one. That is the half of #204 whose text says "the site's name is no longer a link,
  which is what made the duplicate visible as a duplicate", and it is undone here as a by-product of
  the demotion; #204 has no pull request as this is written and its own answer is the same demotion,
  so whichever of the two lands second resolves one hunk of `page.tmpl` rather than a disagreement,
  and a home link goes on the `<p>` if #204 wants one back. The `<caption>`s do not move: a caption
  names a table, and this issue is about the page. The root page's name settles on `Status` only in
  the sense that the heading, the nav entry and the tab strip now read the same word; calling that
  page the dashboard is a rename in three places and belongs to the issue that owns the navigation.
  All nine committed 360 px frames move, because the masthead is in every one of them -- D-0014 paid
  the same cost for the stylesheet and for the same reason, and `manifest.py check` refuses the set
  on an uncommitted `app.css` before anything else does.

  What this branch could not finish, measured at 21:05 EST rather than assumed. `pages.py` refuses
  the dashboard: `REFUSED: /: the 'casey' row says ['locked out until 06:30', …], and this artifact
  is a photograph of the state where it says 'locked out until 00:00'`. That is not this change. The
  fixture carries a second rule, `r-curfew` (21:00→06:30, all seven days), which the fixture has
  held since `14bc3f5`, and `schedule.Evaluate` takes the latest end across the rules that are open
  (`schedule.go:171-179`) -- so the state `Page("/").state` declares holds only between 00:00 and
  20:59 in the profile's zone. Measured by handing the fixture's own profile to `Evaluate`: `EndsAt`
  is `03-19 00:00 EDT` at 00:01, 06:29 and 20:59, and `03-19 06:30 EDT` at 21:00, 21:01 and 23:59.
  A refused page withholds the whole `pages` publish, so the five GET frames wait for that window
  with the dashboard, and the set cannot be re-taken before 05:00 UTC -- midnight in the zone the
  fixture names. The fixture's own declared instant, `CAPTURE_STAMP=2026-03-18T21:34:05` at
  `-04:00` (`fixture.sh:99`), is 21:34 local: a moment at which the state it declares cannot be
  photographed. The finding is filed on its own issue; #207 declared the fixture's
  instant, zone, row count and census and left the hour the capture runs at undeclared, which is
  what `dashboard.png`'s churn row in the capture README has been reporting.
- Checkpoint: 2d1355402e899b1462dbff222750a866128c4d76
- Effect: internal/web/templates/page.tmpl (the `<h1>` and the brand line), internal/web/static/app.css
  (`.masthead__name`), internal/web/handlers.go (the login page's `<h1>Sign in</h1>` and its doc
  block), internal/web/page_heading_test.go (new: the two tests and `headingComplaints`),
  scripts/mutations/web.json (`expected_mutants` 287 → 290; W298 the product name back in the
  heading, W299 the brand line deleted, W300 the heading pinned to a literal), DECISIONS.md
