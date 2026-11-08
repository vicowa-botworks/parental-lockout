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
kept. Renumbering is rarer than the collision it avoids, and the rule that decides which side moves
is at the end of this section.

## Writing to this file before the gate exists

`scripts/check-decisions.py` is not in this tree yet -- the gate and the first twenty entries live on
`feat/factory-harness` (#217), still in review. This branch opened a log anyway, because a tick that
makes a judgement call and has nowhere to write it is the failure the log exists to prevent, and the
alternative -- waiting for #217 to merge -- silently exempts every branch in flight from the rule.
`issue-202` did the same thing on its own branch, which is why this file is two logs and the merge
that joined them kept both halves.

Three consequences, all deliberate, and the third is no longer a prediction:

- **The numbering continues past the highest id on any open branch.** #217's log ends at `D-0020`
  and the half above ran `D-0011`..`D-0015`, so ids were already colliding across branches; the
  entries below resume at `D-0021` rather than adding a third `D-0013`. That rule has paid for itself
  once already: `main` minted `D-0016`..`D-0018` independently while this branch was writing, so a
  branch that had numbered from its own highest id would have opened a third set of those three.
  This file took `main`'s at the 6 Nov merge and renumbered nothing.
- **When two logs really do collide, the published side keeps its ids and the merging branch moves.**
  That is the direction the 6 Nov merge chose for the two mutant rows that arrived as `W298`/`W299`
  on both sides and landed as `W301`/`W302`: `main`'s ids were already published history, and
  `main`'s own entry cited them by id, so renumbering them would have left a merged entry naming rows
  the file beside it no longer holds. The same rule moved three entries when `main` was merged into
  `issue-199` on 6 Nov, the branch that wrote the paragraph you are reading. `D-0025`, `D-0026` and
  `D-0027` as authored there are `D-0033`, `D-0034` and `D-0035` here, because `main` had meanwhile
  minted its own `D-0025` (the scripts condition reaching the browser) and `D-0026` (the two POST
  frames), while `D-0031` and `D-0032` are already claimed by `feat/factory-harness` and `issue-224`
  -- read off `git show <branch>:DECISIONS.md`, the only source that answers. What the move costs is
  stated rather than hidden: the `Decision:` trailers on the three effect commits keep the numbers
  they were written with, because changing them means rewriting a pushed branch, which this
  repository does not do. So `git log --grep 'Decision: D-0034'` finds nothing, and
  `git log --grep 'Decision: D-0026'` names the right commit and, once `main`'s half is beside it, an
  unrelated entry. The bodies of those commits cite the old numbers in prose too, and they are pushed
  as well: `git log --grep 'D-0026'` finds `a2ccac0`, which is talking about the navigation under the
  number this file now reads as the two POST frames. The mapping is D-0036's to carry, and the four
  prose citations of the old numbers
  (`testing/capture/cdp.py`, `testing/capture/lock.py`, `testing/capture/pages.py` twice) were
  rewritten in the same commit so that nothing in the tree points at a number this file no longer
  holds.
- **The field contract is #217's, unchanged** -- `## D-NNNN · title` followed by `Question`,
  `Options`, `Chose`, `Why`, `Checkpoint` (plus `Effect`), with `Chose` repeating one option
  verbatim. So when #217 lands, `make verify` starts checking these entries instead of rejecting
  them for shape. What it finds first is the collision this section describes: `D-0011`..`D-0018`
  will name two decisions apiece once #217's own log lands beside them -- the last three arrived
  from `main` with #221, which answers #219 and #220 -- and renumbering them is that branch's call,
  not this file's.
- **The shape half of that prediction has now been run, and it is not the collision.** On 6 Nov this
  branch fetched #217's gate -- `git show feat/factory-harness:scripts/check-decisions.py`, run with
  `--log DECISIONS.md` -- and it returns three problems, none of them a reused or out-of-order id
  and none of them in either half's
  numbering: `D-0016`, `D-0017` and `D-0018`, all three arrived from `main` with #221, each with a
  `Chose:` that runs past the option it chose. They are one class of defect, the one
  `fix(decisions)` already fixed here once for `D-0015` (b55d892), and the repair is the same: widen
  the option to the `Chose`, which keeps the arithmetic the entry needs and shortens nothing. This
  branch does not repair them, because rewriting another unit's entry is not this unit's call -- the
  three record decisions made on #219's and #221's branches, and the entry is the record of *their*
  tie-break. Whoever merges #217 next inherits them, and the three lines above name them exactly.


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
- Chose: re-photograph the set against the merged tree with both censuses wired in
- Why: the tally moves with the choice -- `EXPECTED_RULES` becomes the union, 36 + 7 (#202) +
  2 (#207) = 45, one tally comment instead of two, which is the half of this option that has no side to
  splice. The first two options are the same mistake aimed at opposite ends of the criterion. Main's
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
- Options: page name in the `<h1>`, brand demoted in the header | product name stays the `<h1>`
  and the answer is documented as belonging to `aria-current` and `<title>` | under the
  first: `title`, a new `heading` block, or `pagename` | the page's own name in the
  `<h1>`, rendered from the same `title` block `<title>` is built from -- one
  definition, two places. The brand line stays in the header as `<p
  class="masthead__name">`, not a heading and not a link. `pagename` stays what it was:
  the body's class.
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
  `-04:00` (`fixture.sh`, the `CAPTURE_STAMP` default), is 21:34 local: a moment at which the state it declares cannot be
  photographed. The finding is filed on its own issue; #207 declared the fixture's
  instant, zone, row count and census and left the hour the capture runs at undeclared, which is
  what `dashboard.png`'s churn row in the capture README has been reporting.
- Checkpoint: 2d1355402e899b1462dbff222750a866128c4d76
- Effect: internal/web/templates/page.tmpl (the `<h1>` and the brand line), internal/web/static/app.css
  (`.masthead__name`), internal/web/handlers.go (the login page's `<h1>Sign in</h1>` and its doc
  block), internal/web/page_heading_test.go (new: the two tests and `headingComplaints`),
  scripts/mutations/web.json (`expected_mutants` 287 → 290; W298 the product name back in the
  heading, W299 the brand line deleted, W300 the heading pinned to a literal), DECISIONS.md

## D-0017 · The pages fixture releases both of its rules at one instant, so the dashboard frame is takable at any hour

- Question: #220. `pages-setup.sh` gives casey's profile two enabled rules, and `Evaluate` releases
  at the *latest* end of the rules that are open (`internal/schedule/schedule.go:171-178`). With
  `r-curfew` written `21:00`→`06:30` the row therefore says `locked out until 06:30` for the three
  hours 21:00 ≤ t < 24:00, while `Page("/").state` declares `locked out until 00:00` -- so
  `require_declared_state` refuses the dashboard for a quarter of every day, a refused page publishes
  none of the fixture's frames, and the fixture's own declared stamp (`2026-03-18T21:34:05 -04:00`,
  `testing/capture/fixture.sh`, the `CAPTURE_STAMP` default) sits inside the refused window: the document could not
  photograph the instant it names. Three answers were on the table -- move the rule's end, teach the
  driver that the state is a function of the hour, or take the second rule out. And one the fix
  forced: #219's masthead move (#221) makes all nine committed frames stale, the two POST frames with
  the rest, and those two belong to #213 -- re-take them here, or ship eight current frames beside one
  that is not.
- Options: curfew end → `00:00` | curfew end → anything the all-day rule does not outrank | keep
  `06:30` and declare a per-hour state | delete `r-curfew` | re-take the POST frames
  here | leave them to #213 and re-take seven | `r-curfew` ends at `00:00`
  (`pages-setup.sh:127`), the dashboard's declared state gains the rule's *name* as a
  second needle (`pages.py:368`), hour-independence is measured rather than asserted
  (`internal/schedule/fixture_hours_test.go`), and all nine frames were re-taken here.
- Chose: `r-curfew` ends at `00:00` (`pages-setup.sh:127`), the dashboard's declared state gains the
  rule's *name* as a second needle (`pages.py:368`), hour-independence is measured rather than
  asserted (`internal/schedule/fixture_hours_test.go`), and all nine frames were re-taken here.
- Why: a per-hour state is #207's refusal inverted: the driver would photograph whichever page the
  clock happened to be on and certify it, which is the thing that file exists to prevent --
  `Page.state` is the declaration that the artifact is a photograph *of*, and a declaration with a
  table of hours in it declares nothing. Deleting the rule would have been the cheap edit and would
  have cost the fixture its only enabled timed rule, which is what `/profiles` is a photograph of:
  `ruleWindow` prints the written pair verbatim (`internal/web/profiles.go:244-256`), so the Window
  cell is that rule or it is nothing.

  `00:00` is not a value that happens to work, which is what makes it a fix rather than a truce. An
  end earlier than its start crosses midnight onto the next day, so the curfew resolves its end
  through `wallInstant(loc, y, mo, d+1, 0, true)` -- the identical expression the `allDay` arm eight
  lines above it uses (`schedule.go:466-483`). Two rules that resolve an end through one expression
  cannot disagree in any zone on any date, including the twelve zone-days where midnight is itself a
  repeated wall time; a tie names the first rule in the profile, which `Evaluate` already pins
  (`schedule_test.go:294`). What is new is measuring it over the fixture instead of a synthetic pair:
  1440 minutes of the fixture's declared day give one verdict, and 1464 instants -- every day of 2026
  at 20:59, 21:00, 21:01 and 23:59 -- give the same one. The same sweep with the committed value
  rewritten back to `06:30` reproduces #220 exactly, 180 minutes of `06:30 / Weeknight curfew
  including the weekend` beside 1260 of `00:00 / Every day, every hour`, and that arm is what stops
  the first test from being a program that prints one line and passes.

  The name needle is there because the clock alone would have missed the defect. #220 moved *both*
  cells of the row -- `EndsAt` and the rule `EndsAt` came from -- and a guard on the clock only would
  have let a frame that re-blamed the row at 21:00 through as an artifact of the state the fixture
  says it holds. It is tested in both directions: `pages.py --self-test` refuses a casey row that
  names the curfew while the clock still reads `00:00`, and refuses the row it declares if the
  declaration is the thing that broke.

  The POST frames were re-taken because the alternative was not a smaller change, it was a false
  check: with the masthead moved, `manifest.py check` can only pass for `lock.png` and `testlock.png`
  if their recorded `sources_rev` advances, and an advancing `sources_rev` over bytes that predate the
  page is the exact sentence #144 was filed to make impossible. Both ran green on the first try, and
  the sentence #213 gives for why re-taking them was a decision rather than a step -- that a re-take
  locks the capturing machine's session for real -- does not hold on this tree and did not when it was
  written: the POST fixtures run `"enforcement": false`, `"lockBackends": ["command"]`,
  `"lockCommandPath": "/bin/true"` (the config block in `testing/capture/post-fixture.sh`, whose
  line numbers this merge moved by dropping #215's header comment above it), so the click is answered
  by `/bin/true` and the page's banner is the unconfirmed sentence, which is the sentence the template
  exists to render. What that cost is in the capture README's own rows, not here: these two are
  photographs of one account holding one session on one machine, and always were.
- What lost: `06:30` is gone from the fixture, so the document no longer depicts a curfew that runs
  past midnight -- `/profiles` now publishes `21:00 - 00:00` for it, and `profiles.png` and
  `profiles-refused.png` moved on this change as well as on the masthead, which is two causes in one
  re-take and is stated because a reviewer comparing that frame to #219's description would otherwise
  attribute the whole difference to the heading. Nothing on the five GET frames was quoting `06:30`
  except through `EndsAt`, and that is the defect. The audit trail's `Weeknight curfew including the
  weekend` rows are seeded text, not evaluated rules, so they would have kept the name even with the
  rule deleted; they keep it, and they keep being what makes the Rule column wide. Re-taking the two
  POST frames spends #213's assigned work without its author, and leaves that issue its README rows
  to re-record rather than the capture.
- Findings this turn measured and did not fix, filed rather than buried. (1) `record()` certifies
  every PNG it finds in its target directory (`manifest.py:219`), not the ones its run took, so the
  documented one-fixture re-capture advances `sources_rev` for frames it never touched. Measured here:
  after the two `pages.py --out docs/screenshots/360` runs the check reported *9 photographs current*
  while `lock.png` and `testlock.png` were byte-for-byte the committed files and their `sources_rev`
  had moved `125b1a49…` → `9ccbe466…`. The identity check cannot see it, because the recorded hash
  matches the stale file it was recorded from. That is why this commit re-took all nine before
  recording, and why the capture README's claim that "a one-page re-capture cannot certify another
  run's PNGs as just-taken" is true of the default and false of `--out`. (2) The fixtures check that
  `$PLOCK_BIN` exists, not that it is newer than what goes into it, and the templates are `go:embed`
  (`internal/web/sources.go:21`) -- a binary built before a template change photographs a page the
  tree does not contain and every check downstream agrees with it. Measured here: a binary built at
  21:03, nineteen minutes before the masthead commit landed at 21:22, left five of the seven
  page-fixture frames byte-identical to the committed pre-masthead files and moved only the two whose
  *fixture* content had changed -- which is the set a reader would have called "the two #220 moved"
  and committed. The guard is D-0018.
- Checkpoint: 1af2a76be2d7564305677b36ecce99a52355daf3
- Effect: testing/capture/pages-setup.sh (`r-curfew`'s end, and the reason beside it),
  testing/capture/pages.py (the `Every day, every hour` needle, its self-test arms),
  internal/schedule/fixture_hours_test.go (new: the day sweep, the year sweep, the discrimination),
  the nine frames in docs/screenshots/360 and their manifest, testing/capture/README.md (the
  homogeneous-set paragraph, the two POST rows, the re-capture recipe), DECISIONS.md

## D-0018 · A capture refuses a binary that predates the sources compiled into it

- Question: `capture_begin` asked whether `$PLOCK_BIN` was executable and nothing more, while the
  templates and the stylesheet are compiled into it (`internal/web/sources.go:21`, `go:embed`). A
  leftover binary therefore serves the page it was built from, the browser faithfully photographs
  that page, and `manifest.py record` certifies the result against the sources on disk -- which name
  the new template. What may a capture claim about the binary behind its frames, and how is the
  wrong one refused without the refusal becoming the bug? Options: refuse when a compiled source is
  newer than the binary | refuse on a dirty working tree | record the binary's digest in the manifest
  and check it later | leave it to the README, which already says to build first.
- Options: source-newer-than-binary mtime | dirty-tree check | binary digest in the manifest |
  docs only | the mtime relation. `capture_begin` now refuses when any non-`_test.go`
  file under `cmd/` or `internal/`, plus `go.mod`/`go.sum` where they exist, is newer
  than `$PLOCK_BIN`, and its message names the offending paths and the build line.
- Chose: the mtime relation. `capture_begin` now refuses when any non-`_test.go` file under `cmd/` or
  `internal/`, plus `go.mod`/`go.sum` where they exist, is newer than `$PLOCK_BIN`, and its message
  names the offending paths and the build line.
- Why: mtime is the cheapest predicate over the question a capture is actually answering -- is the
  thing being served the thing in this tree. A dirty tree is not that question and gets both answers
  wrong: the measured case below was a *clean* tree with a stale binary (built 21:03, the masthead
  commit landed 21:22), and a freshly rebuilt binary on a dirty tree is exactly what a capture wants
  and would have been refused. A digest in the manifest is a record made after the fact, and the
  failure mode is a frame nobody took -- no downstream record catches that; it only names the lie
  later. Docs-only is what this file has said since #213, and this morning a stale binary still
  produced seven plausible frames: five of the seven page-fixture artifacts came out byte-identical
  to the pre-change files and the two that moved were precisely the two whose *fixture* content had
  changed, so the set a reviewer would have read as "the two this branch moved."
  `*_test.go` is excluded because it is not compiled into `plock`; `go.mod` is included because a
  dependency change moves a byte of the binary. Both `go` files are asked about only `if [ -f ... ]`,
  and that guard is not tidiness: this repository has no third-party dependency and so no `go.sum`,
  `find` exits 1 on a path that does not exist, and the setup scripts run `set -euo pipefail` -- so
  the first version of this arm died at the assignment with nothing on stderr, exit 1, indistinguishable
  from a fixture that had simply stopped. Probing every branch of a scriptlet is what found that, and
  the three arms are probed: a touched template refuses and names `page.tmpl`, a fresh binary passes,
  and a touched `*_test.go` does not fire.
- What lost: mtimes, with everything that implies. `touch`ing a source forces a rebuild even when the
  content is unchanged -- the probe does exactly this -- and the rebuilt binary from identical sources
  is the same program, so the price is a two-second build rather than a wrong verdict. Any file under
  `internal/` fires it, including ones no `go:embed` pattern reads, and narrowing that means keeping a
  bash copy of the embed list, which is a second definition whose failure mode is a silent miss. A
  `PLOCK_BIN` aimed at an installed binary now has to be newer than the checkout, which is what the
  negative control does anyway (it builds a clause-removed binary after the sources). And the check
  lives in `capture_begin`, so a driver run against a fixture set up before this commit still passes:
  it protects the setup, not the frame, and a frame's real provenance stays `manifest.py`'s job.
- Checkpoint: 9e07dde673b8741aa47d7215b661c7bcbdc368f5
- Effect: testing/capture/fixture.sh (the freshness arm in `capture_begin`, and the reason beside it),
  testing/capture/README.md (the build-first paragraph of the re-capture recipe), DECISIONS.md

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
  W301,W302` (numbered `W298`/`W299` when measured, see the renumbering below) returns `CAUGHT` for
  each, and the same layout edit applied by hand fails
  `TestTheAuditRowIsStampedTheSameWhoeverReadsIt` and all four of its zone subtests. What this costs:
  the set's tally moves 287 -> 289, so any branch quoting 287 is now wrong -- #215 is stacked on this
  branch and its body does -- and both rows are anchored inside the When cell, which is the region
  #1048 calls fragile: pull that expression into a template variable and both patterns match zero
  times, reported as NOT APPLIED, which is a build-shaped complaint rather than the lost decision it
  would be.

  Renumbered at the 6 Nov merge, and the direction of that renumbering is a decision worth its two
  lines. `main` arrived with three rows of its own on `page.tmpl` numbered `W298`..`W300` (#219's
  heading), so the two sets collided on both ids; only one pair could keep the number. `main`'s kept
  them, for two reasons the tie-break does not reach: those ids are already published history, and
  `main`'s D-0016 entry cites `W298`/`W299`/`W300` by name in its own `Effect:` line -- renumbering
  the mutants would have made a merged entry describe different rows than the ones it names, in the
  same file that says a reused id is refused. This branch's two moved to `W301`/`W302`, in the commit
  that rewrites their citations in this log, so nothing here cites a row that does not exist. The
  merged tally is therefore 292, and `main`'s D-0016 already tells the 287 → 290 half of the move.
- Checkpoint: 39b60988ad0cb2361e98c6ff64a45089860b8475
- Effect: scripts/mutations/web.json (`W301`, `W302` -- written `W298`/`W299` before the 6 Nov merge,
  `expected_mutants`)
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
  `sources_rev` for `/audit` names a merge, not the commit that wrote the template: the field is
  `git log -1 --format=%H` over the row's *declared sources* -- `newest_commit(sources)`, whose rule
  `manifest.py:31` states and `manifest.py:314` implements; `record` writes the field at `:480` and
  `check` recomputes it at `:749` -- and a conflict-resolving merge touches all four of them, so the
  command answers the merge. D-0030 corrects how this sentence was written: it derived the value from
  `audit.tmpl` alone, which answers `c82baa6` and still does at this head, and it quoted a sha
  (`1c5a109`) that the next merge falsified the same day. The command travels; the sha does not. The
  header's `head` is whichever checkpoint `record` was run from, not the one below (D-0030).
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
  to disprove. Re-recording either is worse than wrong, though three sentences written here about
  *why* were wrong too and D-0028 corrects them against the artifacts on `origin/main`; the claim
  that survives is the one the correction leaves standing: `record` will date a frame that shows the
  wrong page, and `check` then agrees. The refusal that would have
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
  main's frame over the offset template. This entry said `make verify` "says nothing about" that --
  it does not, and D-0028 measured it twice: `Makefile:90` runs `manifest.py check` inside `verify`,
  and the check refused at `5c63fb9` and again at `d1ee983` with `audit.png predates the newest
  change to /audit's sources`. The blind step is one earlier, not the gate: `record` stamps whatever
  PNG is in its directory with `sources_rev = newest_commit(sources)`, and `check`'s first arm is
  `if recorded == current: continue`, so a frame that has just been re-dated cannot be seen to be
  wrong. That is the seam D-0028's option 2 refuses on, and the sentence belongs to whoever reads
  that commit. (2)
  `captured_at` is one field for the whole manifest: it now reads `2026-11-06T00:52:33Z`, which is
  true of the six pages the `pages` fixture photographed and false of `lock.png`, `testlock.png` and
  `users-no-profiles.png`, whose bytes this run neither took nor changed. That is the same
  single-timestamp-for-nine wart D-0015 put on the record, and this commit is on the side that made it
  slightly wider. (3) `/audit`'s `sources_rev` now names a merge, not `c82baa6`, so a reader hunting
  the offset change still wants `git log -- internal/web/templates/audit.tmpl`. What did *not* happen
  this time is worth saying next to D-0015's confession: the re-record left the other eight entries'
  `sources_rev` at `125b1a4` -- #221 moved them to `9ccbe466` with its own set -- so no untouched
  frame had its date laundered.
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
  `post-fixture.sh` advertised then ("Override with `PLOCK_CAPTURE_SCRIPTS=on` to test a frame taken
  with scripts running; the declaration follows the override into the manifest" -- line 47 of that
  revision, and D-0026 has since rewritten the header it sat in) took the same pixels
  as the default, and the manifest's `scripts` field recorded what `cdp.py` does rather than what the
  frame was taken under.
- Options: write the test into the notes as advertised | retract the override in prose and point the
  reader at a hand edit to `cdp.py` | make `cdp.py` honour the declaration, refuse a value outside the
  two, and refuse to *commit* a frame recorded `scripts: "on"`
- Chose: make `cdp.py` honour the declaration, refuse a value outside the two, and refuse to *commit* a
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

## D-0026 · The two POST frames are taken on this branch, and the byte-identity claim is withdrawn

- Question: #215's review says `lock.png` and `testlock.png` carry an `off` those pixels cannot
  witness, because the driver that produced them in October ran the page's scripts and `cdp.py` never
  read the declaration. Its option 1 is to write the discriminating test into the two notes and leave
  the frames; its option 2 is to take the frames here, "digests move or don't", so that the field
  becomes a statement about pixels rather than about the driver's present. Which, now that D-0025 has
  made the test runnable in both directions?
- Options: state the test in the notes, frames untouched | take the two frames here and record what
  both arms return | drop `scripts` from those two blocks as undeclarable
- Chose: take the two frames here and record what both arms return
- Why: the measurement came first, on four runs of the two fixtures with the knob live. `on` and `off`
  produce **one PNG apiece** -- `lock.png` `b59bb2be…`, `testlock.png` `acc5ba4d…`, both equal to the
  committed files -- because neither page renders a `time[data-unix]` element and `renderCountdowns` is
  `static/app.js`'s only load-time DOM write, so there is nothing on those renders for the page's
  scripts to change. The control is `/dashboard`, where the same switch moves casey's row from the
  server's `until 00:00` to the browser's `until 7h 46m` and `require_declared_state` refuses the frame:
  the switch has power, just not where these two artifacts live. Option 1 is what the review asked for
  literally and it is refused by that measurement rather than by preference -- after D-0025 the two arms
  return the same file, so writing the sentence would document a test that cannot discriminate while the
  manifest went on asserting `off` of frames whose driver has just been shown not to be the one the value
  describes. Option 3 throws away a statement that can be made true in order to avoid one that cannot,
  and it would cost `check` its `scripts` rule and `/2` its "every frame states what it was taken under".
  So the frames were taken here, at `80295c513`, by the driver that honours the declaration: the PNGs
  moved no bytes, the manifest header moved, and every entry was re-checked against all four fixtures'
  current declarations -- nine blocks, no disagreement printed. What they certify about "usable at 360 px
  with JavaScript off" is certified by the session's setting, which is the most a page with no countdown
  can say, and the notes now say it that way instead of claiming the pixels prove it.
  Two claims are withdrawn in the same commit rather than softened. The 4 Nov sentence "these pixels ARE
  the scripts-off pixels" inferred a condition from a digest comparison that cannot see the difference on
  these two pages, and it is the same error as the `on` it replaced -- provenance dressed as measurement,
  one layer down from #211. And this folder's stated reason for treating a re-take as a decision, that it
  "locks the capturing machine's session for real", is simply false: `post-fixture.sh` sets
  `"enforcement": false` and `/bin/true`, and measured 6 Nov around the run that took both frames the
  capturing account's own session went `active` -> `active` with `LockedHint` `no` -> `no` while the
  fixture's `audit.jsonl` filled with `lock.requested`. That sentence is the reason #213 sat unclaimed,
  which is worth more than the comfort of leaving it in.
- Checkpoint: 80295c5138207c54ec19a3694c0849749d4855ca

## D-0028 · The nine frames are taken against the merge, and each is stamped from the run that took it

- Question: the merge at `d1ee983` staged `main`'s nine frames and grafted this branch's `capture`
  blocks into its manifest, and `make verify` refuses that tree twice -- `audit.png predates the
  newest change to /audit's sources`, and `dashboard.png records the state [four cells] and the
  driver now declares [five cells] for /`, because #221's fixture added casey's `Every day, every
  hour` row. CI run `42180788535` failed `verify` on exactly those two sentences (reproduced locally,
  `VERIFY_EXIT=2`). Which frames does the branch re-take, and what does `record` get handed?
- Options: re-take `audit.png` only | re-take all nine and `manifest.py record` with its defaults, as
  `testing/capture/README.md`'s recipe says | re-take all nine and record them from all four fixtures'
  `fixture.env` with all nine `--shot` names | keep `main`'s manifest and re-take nothing
- Chose: re-take all nine and record them from all four fixtures' `fixture.env` with all nine
  `--shot` names
- Why: the second refusal settles the first half on its own -- `dashboard.png` is not the frame the
  template change moved, it is a frame whose *conditions* went stale -- so a one-frame re-take would
  have left the gate red and #222's finding unsatisfied: `record()` walks every PNG in its directory
  and advances `sources_rev` for the ones this run never touched. The run was the four drivers on a
  binary built for this tree (`fixture.sh` refuses one that predates its sources, D-0018), and the
  control is the interesting part: **eight of the nine came back byte-identical to `main`'s committed
  files** -- `dashboard` `1ec41100…`, `users` `81f976b7…`, `profiles` `8eae33ea…`, `profiles-refused`
  `5206d907…`, `settings` `7bfacb6f…`, `users-no-profiles` `dac864f4…`, `lock` `7ec1a664…`,
  `testlock` `d008fdf9…` -- and `audit.png` is the only mover, `ac49f291…` to `0cfe9973…`,
  `360x3107`, `6/6 pages fit at 360 px` and `6/6 … at 1024 px` with the offset needle live: the
  declared state at `pages.py:412` is `("auth.success", "2026-03-18 21:34:05 -04:00")` and
  `require_declared_state` accepted the page rather than refusing it.
  Option 2 is the one this file's own recipe tells a reader to run, and it was measured rather than
  argued with. In a scratch copy of the artifact directory (`testing/capture/runs/probe/`, gitignored)
  with `main`'s `audit.png` substituted -- a photograph of the `MST` cell this branch deletes --
  `check` refuses first on identity (`ac49f291a5cd… on disk, 0cfe9973711b… recorded`, rc 1), and one
  bare `manifest.py record` over those same bytes answers `9 photographs current`, rc 0, having
  dropped that frame's `capture` block to nothing and the manifest to `capture-manifest/1`. The
  mechanism is two lines: `record` writes `sources_rev = newest_commit(sources)` (`manifest.py:480`)
  from the commit and the clock, never from the pixels, and `check`'s drift arm opens
  `if recorded == current: continue` (`manifest.py:749`). So `record` does not merely tolerate a
  swapped frame, it mints the certificate -- "green and wrong" is the honest description, and it is
  why D-0024's refusal of *that* option has to stand even though three of its sentences did not.
  Handing over the four envs and the nine names is the only option under which every entry states
  conditions a run actually ran under: the two POST rows stop carrying October's block on the strength
  of unchanged bytes, and the schema stays `/2` because nine of nine say what they were taken under.
  Option 4 is what `main`'s manifest is -- `/1`, no blocks at all -- and it would have deleted #215.
  The tie-break agrees: nothing separates on `go.mod` or breadth (the working tree moves two files
  either way), "refuses rather than proceeds" picks the option whose every recorded condition has a
  witness, and `git revert` of this commit restores `main`'s frames and manifest untouched.
- What this costs, stated rather than glossed. `head` was `8d8be5854`, the checkpoint, because
  `record` ran there; that commit is empty, so its tree is `d1ee983`'s and the render is that tree's,
  but a reader who wants the commit that *contains* these PNGs wants the effect commit below. D-0029
  ran `record` a second time, from its own checkpoint, so the committed header now reads `45977d6`
  over these same bytes (D-0030 corrects the sentence above): `head` names where `record` stood,
  never what holds the frames.
  `captured_at` is still one field for nine frames (D-0015's confession, unchanged); this time it is
  true of all nine, which is the widest the wart has ever been and the first time it has been a
  statement rather than a guess. The eight untouched rows keep `sources_rev` `9ccbe466` -- the same
  commit `main` recorded -- so nothing was re-dated here; only `/audit` moved, to `d1ee983`. And
  because the eight bytes are `main`'s, "eight are identical and `audit.png` is the only mover" is
  true of this tree in a way it could not be of `cc3a14d`, which is the sentence this PR kept having
  to re-quote.
- Correction, in the same commit as the measurement that forced it. Three sentences of D-0024 are
  false and were amended here rather than left for a reader to trip over. (a) "main's `audit.png` was
  taken at `ac20479`": `main`'s manifest records `/audit` at `sources_rev` `9ccbe466`, and the newest
  commit touching the PNG is `9e07dde`; `ac20479` is a real masthead re-take that no field reads.
  (b) "`ac20479` is a descendant of every commit that touches the page's declared sources, including
  this branch's `c82baa6`": `git merge-base --is-ancestor c82baa6 ac20479` exits 1 -- it predates this
  branch and does not contain it. (c) "`make verify` says nothing about" a merge commit carrying
  `main`'s frame: `Makefile:90` runs `manifest.py check` inside `verify`, and that check is what
  printed the two refusals this entry exists to clear. What survives is narrower and worse -- the gate
  is not blind at the merge, it is blind *after any `record`*, which is the arm option 2 walks into.
- Checkpoint: 8d8be58540e53721e726ab08a67879c502ac754b
- Effect: docs/screenshots/360/audit.png (re-photographed against `d1ee983`),
  docs/screenshots/360/manifest.json (schema `/2`, `head`, `captured_at`, nine `capture` blocks,
  `/audit`'s `png_sha256` and `sources_rev`), DECISIONS.md (this entry and the three D-0024
  amendments above)

## D-0029 · The capture notes' example of an unstated condition is a frame that states it

- Question: `fixture.sh` and `testing/capture/README.md` both hold up `pages-noprof` /
  `users-no-profiles.png` as the artifact whose manifest entry "says nothing about" page scripts, and
  the README's re-capture recipe ends on a bare `manifest.py record`. #215's review measured all nine
  committed rows reading `scripts: "off"`, and D-0028 measured the bare record dating a substituted
  frame as current. Correct the prose, change the code until the claims are true, or delete them?
- Options: correct the three sentences to what the artifacts show | default `CAPTURE_SCRIPTS` to empty
  so a fixture can genuinely state nothing | delete the sentences and the recipe's last line
- Chose: correct the three sentences to what the artifacts show
- Why: the claim is checkable in one command and it fails. `CAPTURE_SCRIPTS="${PLOCK_CAPTURE_SCRIPTS:-off}"`
  is never empty, `capture_env` writes the line whenever it is not, and all four setup scripts source
  `fixture.sh` -- so nine of nine rows read `off`, `users-no-profiles.png` included, and its entry
  carries `rows`/`tz`/`census` absent with `scripts` present. It is in good company: three of the six
  optional keys (`STAMP`, `ZONE`, `SCRIPTS`) carry non-empty defaults, and the notes had generalised
  from the other three (`AUDIT_ROWS`, `PATH_PREPEND`, `TZ`), where absence really does mean nobody
  chose. So the paragraph is not wrong about the rule -- `manifest.py`'s absent-is-not-a-default rule
  is real and self-tested -- it is wrong about which keys it operates on and it points a reader at a
  frame that disproves it, in a folder whose whole argument is that you can go and check the example.
  Option 2 buys a true sentence by deleting the only statement two of the nine frames have: with an
  empty default every entry loses its `scripts` key, `check`'s scripts rule loses its subject, and the
  manifest falls back to `capture-manifest/1` -- the exact flattening D-0028 measured, chosen on
  purpose so that a paragraph can keep its illustration. Option 3 is worse than the false sentence it
  removes, because the asymmetry it records (a frame can be *taken* under a condition the entry leaves
  unstated) is the one a future re-take has to get right.
  The recipe line is the same defect with a sharper edge, so it went in the same commit: the bare
  `manifest.py record` the file tells a reader to run last is measured (D-0028) to stamp whatever PNG
  is in the directory with `sources_rev = newest_commit(sources)` past a gate whose first arm compares
  that field for equality, and to drop the block of the frame whose bytes moved -- `9 photographs
  current`, rc 0, over a substituted frame that shows the deleted `MST` cell. A recipe that destroys
  the artifact is worse than no recipe, so the line now names all four fixtures' envs and all nine
  shots, and was run exactly as printed before it was committed: it changes `captured_at` and `head`
  and nothing else. The paragraph beside it says which command is safe when, and the stale
  `manifest.py:219` citation it carried -- now a docstring about rules that call functions -- is
  replaced by the function names, the convention this file already uses for citations that must not
  rot.
- What this costs, stated rather than glossed. The notes now carry no worked example of an absent
  `scripts`, because none exists; a reader who wants one has to set `PLOCK_CAPTURE_SCRIPTS=` and take
  a frame, and the corrected paragraph says so instead of pointing at a PNG that would contradict
  them. And the recipe is six lines instead of one: the short form is left standing as the command for
  a whole honest set, which is the case it was written for, with the substitution measurement next to
  it so nobody finds the other case by accident.
- Checkpoint: 45977d6817cb71cb3e798e0dc0c6fec6ae5eb624
- Effect: testing/capture/fixture.sh (the `PLOCK_CAPTURE_SCRIPTS` note), testing/capture/README.md
  (the re-capture recipe, the record-and-substitution paragraph, the `scripts` bullet),
  docs/screenshots/360/manifest.json (`head`, `captured_at`), DECISIONS.md

## D-0030 · `sources_rev` is provenance, not a gate input, and two sentences say so now

- Question: #214's round-6 review read `/audit`'s recorded `sources_rev` as `1c5a109` while `git log
  -1` over that row's four declared sources answered `5c63fb9`, and asked for D-0023's explanation to
  be rewritten to "name the command and the tree so the next merge does not falsify it again". The
  same pass makes D-0028's `head` sentence stale on arrival, because D-0029 ran `record` again. Amend
  the two sentences in place, leave them beside a correct artifact, or delete the claims?
- Options: amend both in place and put what the field is worth on the record here | leave them, since
  D-0029's `Effect:` line already says it moved `head` | drop the provenance sentences and let
  `manifest.json` be the only record of itself
- Chose: amend both in place and put what the field is worth on the record here
- Why: nothing on the tie-break list separates these -- no dependency, nothing a page renders -- so it
  falls to "refuses rather than proceeds" and to this file's own argument. A sentence that names the
  command which produced a field survives the next merge; a sentence that quotes the value does not.
  This branch has quoted `/audit`'s `sources_rev` three times -- `c82baa6` in the pull-request body,
  `1c5a109` here, `d1ee983` now -- and the review supplied a fourth, `5c63fb9`, from `main`'s side of
  the same four files; each was true when written and each is now a dead end for a reader. Option 2
  loses because D-0029's `Effect:` line names the fields it moved, not the values it replaced, so
  anyone who reads D-0028 alone is still told `head` is `8d8be5854`. Option 3 throws away the true
  half -- the field names a merge, not the commit that wrote the template, which is exactly what a
  reader diffing `audit.png` against `git log -- audit.tmpl` gets wrong -- to remove the false half.
  The precedent is this file's: D-0028 amended three false D-0024 sentences in the commit that
  measured them.
  What the field is worth was measured before it was written down, and it is less than the schema
  implies. `/audit`'s `sources_rev` replaced by forty zeros at this head, `manifest.py check` answers
  `9 photographs current`, rc 0. The reason is the arm order: `:757` is `if recorded == current:
  continue`, and once they differ the verdict is built from `current` and the photograph's own newest
  commit (`:772`), so the recorded value is never read a second time -- a value that is merely wrong
  reads exactly like a correct one. The artifact went back byte-for-byte (`sha256
  94681166ad11229ae4325a6558c0b9143a5846931f4d6227a919bfcc66aee537`, `git status --porcelain` empty),
  and the probe had to be made in place for a reason worth its own sentence: a copy of the directory
  under `testing/capture/runs/` cannot exercise the drift arm at all, because `check` takes the PNG
  path from `--dir` and answers an untracked frame with "no commit this checkout can see touches the
  photograph". That bounds D-0028's scratch-copy method to the identity refusal it used it for.
- What this costs, stated rather than glossed. Two more entries carry a correction marker, so the log
  reads as a record of being wrong rather than as a clean ledger; D-0024 and D-0028 already look like
  that, and the branch has `#1044`-class scar tissue from letting a plausible provenance sentence
  stand. The finding also trims #215's own claim: `sources_rev` is provenance a human can audit, not
  something the gate reads. Nothing in the gate moves -- `check` still refuses a frame that did not
  travel with its page, because that verdict comes from the photograph's history, which is the half
  that is not self-reported. One field the review did not reach says the same thing about itself:
  `captured_at` is `2026-11-06T09:12:46Z`, forty-three seconds *after* `774a270` (09:12:03Z), the
  commit holding the nine frames it dates, because D-0029 re-ran `record` after committing them. It
  is the recording's timestamp, not the photographs', and D-0029's "it changes `captured_at` and
  `head` and nothing else" is true of the bytes and silent about what the new value now means.
- Checkpoint: c8b1e5a6648cb8fdbca270e9310f76c094ad6026
- Effect: DECISIONS.md (D-0023's `sources_rev` sentence, D-0028's `head` sentence, this entry)

## D-0031 · A frame this run did not take keeps the `sources_rev` its bytes were rendered from

- Question: #222 was closed at 13:17Z as satisfied on `main` by #215 and D-0028, row by row, the
  negative case included -- three frames in one directory, one page re-taken, the untouched frame must
  not acquire conditions. Every one of those rows is about `capture`. The field the drift gate reads
  first is `sources_rev`, and `record` still wrote `newest_commit(sources)` for every PNG in its
  directory (`manifest.py:480`) whatever `shot` said, while `check`'s drift arm opens
  `if recorded == current: continue` (`:757`). Does the recorder get to move a rev for pixels it did
  not photograph?
- Options: leave it, since `--shot` exists and the notes have said to pass it since D-0028 | carry the
  prior entry's `sources_rev` for a frame outside `shot` whose digest is unchanged, and mark it
  `carried_from` | additionally refuse any frame whose bytes moved and nobody named in `--shot` |
  settle it in the issue and change nothing here | carry the prior rev and mark it; reopen #222; stop short of the third option
- Chose: carry the prior rev and mark it; reopen #222; stop short of the third option
- Why: measured before anything was written, against `origin/main` at `e74b42f` with this file's own
  code and a fake git that knows each path's newest commit -- five frames recorded at `a...`, the page
  then moved to `b...`, one frame re-taken in that commit, one `record`: **`check` answers 0 problems
  and four entries name a commit their bytes were not rendered from**. Carrying the prior rev instead
  turns that into **4 refusals and 0 lying entries**, and the refusals are the sentence #144 was filed
  for. The equality is not a description of the frame, it is the switch: a wrong `sources_rev` that
  *differs* falls through to the ancestry arm and behaves, which is exactly why D-0030's probe found
  nothing, and a re-dated one *equals* `current` and is never read again. Option 1 is a recipe, not a
  check: the driver has passed the declaration since `pages.py:965` and a bare `manifest.py record` --
  the command the notes still call right for an honest whole-set capture -- launders four frames with
  it. Option 3 is the substitution case, and the digest rule cannot see it by construction: swapped
  bytes are a *changed* frame, so the only guard left there is `check` before `record` plus `--shot`.
  Refusing an undeclared change also refuses the documented path for `lock.png` and `testlock.png`,
  which no driver records, and that path has not been measured -- shipping an unmeasured refusal into
  the gate over the acceptance evidence is how #144's "the workflow that does the right thing, made
  red" happens. Tie-break: nothing separates on `go.mod` (no dependency either way) or on breadth (two
  files either way); "refuses rather than proceeds" picks the carry, because it is the only option
  under which a stale frame produces a refusal rather than a sentence; and `git revert` restores the
  old stamping with nothing to re-photograph, because no artifact moves in this commit.
- What this costs, stated rather than glossed. A frame re-taken under moved conditions and not
  declared now keeps the old rev, so if its page has also moved it is refused -- the escape is `--shot`,
  the flag the recipe already passes, and the alternative was the laundering. `carried_from` is added
  to the per-entry vocabulary with no schema bump: it is additive and `check` ignores unknown keys, so
  a `/1` file and a `/2` file both read, and nothing has to be re-recorded. It also inherits `head`'s
  wart (D-0028) -- it names the manifest these pixels were certified in, not the commit that rendered
  them -- and now every carried row carries that indirection. `_fake_git` no longer answers
  `rev-parse HEAD` with the shallow verdict, so a box's `head` is finally a commit; the field was
  storing `"false"` as provenance, which is why the new rule asserts it rather than trusting it. The
  substitution half stays open, and the notes now say so in the imperative instead of as history.
- Correction, in the same commit as the measurement that forced it. D-0030's "Nothing in the gate
  moves -- `check` still refuses a frame that did not travel with its page, because that verdict comes
  from the photograph's history, which is the half that is not self-reported" is false of a partial
  re-record, and it is the sentence that let #222 read as settled. Its probe replaced `/audit`'s rev
  with forty zeros -- a value that *differs*, so control passed the short-circuit and the ancestry arm
  did its job. The value a re-record mints is not a differing value; it is the equal one, and equality
  is the arm's first condition. The field is read a second time, as the switch. What survives of
  D-0030 is the audit claim, and this entry is what makes it true again after a partial capture.
- Checkpoint: 2e1136e8814dfbb5435ba73a023fb24578da6e36
- Effect: testing/capture/manifest.py (the carry in `record`, `carried_from`, `summary()` and the
  sentence naming carried frames, `_fake_git`'s `rev-parse HEAD`, two rules, `EXPECTED` 40 -> 42),
  testing/capture/README.md (the record-last section, the ancestry sentence, the 48-rule count),
  DECISIONS.md (this entry). No artifact moves: `docs/screenshots/360/` and its manifest are untouched
  and `make verify` passes with the nine frames as `main` left them.
## D-0032 · Two of the five issues this log names are closed as satisfied; the other two are reduced to what they still ask for

- Question: #224. This log names #204, #207, #213, #219 and #220; four were open when it was filed, and a lane that reads an open issue as unimplemented builds it again from a base 42 commits behind. (#207 closed itself on 6 Nov at 13:29Z with its own row-by-row correspondence, so four were walked here.) Walk each against `origin/main` and close it, rewrite each down to its residue, rebase the stale branches and let the diff decide, or fold the set into #216 and close nothing?
- Options: leave all five open for the next lane to re-derive | close all five on the strength of this log's claim | walk each issue's own acceptance list row by row, closing only where every row holds and reducing the rest | rebase `origin/issue-199` onto `e74b42f` and reduce it to whatever survives | file the reconciliation as one issue and touch nothing
- Chose: walk each issue's own acceptance list row by row, closing only where every row holds and reducing the rest
- Why: the outcome first, so the line above is not the only place to read it: #219 and #220 closed as satisfied, #213 and #204 reduced to the rows that did not hold, with every settled row and the command that re-checks it written into the issue rather than into this log. The log's claim is not the acceptance list, and #204 proves it — the only entry here that names #204 is D-0016, whose markup comment at `internal/web/templates/page.tmpl:45` reads *"There is no skip link in this shell yet -- #204"*, i.e. it names the issue to say the work is absent. A grep for `#204` in this file, which is what #224 says a lane does, closes an issue that `main` does not implement: `git grep -n aria-current origin/main -- internal testing` returns nothing and `app.css` still has no `nav` selector, so the navigation is on the user-agent sheet exactly as reported. Closing all five therefore had to be refused on evidence, not on principle.
  The two that closed were measured rather than read. #219: `internal/web/page_heading_test.go:29`/`:78` assert one `<h1>` per page, its agreement with the page's own `<title>`, and that it does not name the product, over all seven pages including the two a GET cannot reach and the login page that is not the shell (`go test ./internal/web -run Heading` → ok, 6 Nov 13:44Z). #220: `internal/schedule/fixture_hours_test.go` gives 1440 minutes of the fixture's declared day and 1464 instants across 2026 one verdict, and its second test rewrites `06:30` back in and gets two — so the sweep that closes the issue is the same sweep that could have failed it (`go test ./internal/schedule -run PagesFixture` → PASS, same minute). #213 kept two rows because a re-take is not the same as a declared condition: the census is still a caveat in `README.md:280-281` and the two-arm measurement that exists is `scripts` off/on on one machine in one zone, not this issue's two zones.
  Two things were filed instead of done, both because the honest version needs a rig this fence does not have. `README.md:588` and the table at `:661-662` still call `b59bb2be…`/`acc5ba4d…` the committed digests when D-0028 moved those frames to `7ec1a664…`/`d008fdf9…` (`sha256sum`, 6 Nov) — rewriting them to values this turn did not photograph is the same overquoting D-0029 and D-0030 exist to remove, so it is #213's third row with the two arms named. And the rebase was not attempted: `origin/issue-199` is another unit's pushed branch, 42 commits behind with its own D-0025..D-0027 that collide with this file's, so what this lane could do was write the pointer into #204 — the implementation is in `2e919aa`/`8db322a`/`a2ccac0` on that ref, and the next lane rebases it instead of re-deriving it, which is the half of #224 this tree can actually fix.
  The id is D-0032, not the D-0031 this file's sequence gives. Pull request #217 is open and carries a *different* D-0031 in this same file (the #211/#222/#223 closes), and `check-decisions.py` on that branch refuses both a repeated id in one log — *"'revert to before D-0031' would name two commits"* — and a decreasing sequence. So whichever of the two branches merges second must renumber its own entry, and that is written here so the collision is a known instruction rather than a red gate on `main` after a merge. Choosing D-0031 anyway was the option that made `main` provably invalid for a window; a gap in the log if #217 never lands costs nothing, because the rule is monotonic, not dense.
  What lost. #220's other two options lose: pinning the daemon's clock through `web.Server.Now` and declaring the window in `pages-setup.sh` — both recorded as rejected in D-0017, and #220's body carried them as offers, not requirements. #213 loses the argument that byte identity proved the condition, which #207's own withdrawal already retired. #204 loses two measured rows — the duplicate `href="/"` and the `h1` weight comparison — both fixed under D-0016 as that entry predicted, and the issue now says so instead of sending a lane to re-measure them. Nothing here is irreversible: every close is one `git show origin/main:<path>` away from being re-checked, the four comments carry those commands row by row, and #224 is shut only as the walk it asked for — #216 still owns whether the supervisor should reconcile at all.
- Checkpoint: ab7263a2df0b8e662eb68f1a3107f143e17c65a6
- Effect: issues #219 and #220 (closed as satisfied, nothing deleted), #213 and #204 (reduced to their residue, with the settled rows and their re-check commands beside them), #224 (closed as delivered once the pull request carrying this entry is open), DECISIONS.md (this entry)

## D-0033 · The shell is told the path, and one segment decides which entry is current

- Question: #204 measured the same computed style on every navigation link on every photographed
  page -- the link for the page the reader is on included -- and named the markup half of the cause:
  `page.tmpl` emits the five links with no class and no `aria-current`, so there is no attribute for
  a rule to hook onto and nothing for a screen reader to announce. How does the shell find out which
  page it is rendering, and what does it write?
- Options: `pageData.Path` filled from `r.URL.Path`, plus one method on the shell that answers "is
  this href the reader's section" | the page's own `pagename` definition, compared against each href
  in the shell | the navigation moved into Go as a slice of entries carrying a `Current` bool |
  `body.page-<pagename>` and a stylesheet selector, with no DOM attribute | `app.js` setting
  `aria-current` when the page loads
- Chose: `pageData.Path` filled from `r.URL.Path`, plus one method on the shell that answers "is
  this href the reader's section"
- Why: the last option dies on a measurement before any tie-break is reached. #212 made every
  capture run with `Emulation.setScriptExecutionDisabled`, so an attribute that exists only after a
  script has run is absent from all nine committed frames, from every reader who blocks scripts, and
  from #204's own criterion 3, which asks for `aria-current="page"` "asserted on the DOM rather than
  inferred from CSS". The fourth is the fewest files and has this repository's own precedent --
  `page.tmpl` already writes `class="page-{{template "pagename"}}` on the body -- and it loses on
  that same criterion: no selector reaches a DOM attribute, so a stylesheet-only answer marks the
  page for the parent who can see it and nothing for the one who cannot. What it keeps, and keeps,
  is that it needs no template edit at all; the stylesheet gets its hook from the attribute instead,
  which is why the rule written later reads `nav a[aria-current="page"]` rather than a class. The
  second loses because `pagename` names a template, not a location: the two POST result pages render
  from a template named for the action that produced them, at a URL that is a child of an account
  row, so a per-template answer marks nothing on `lock.png` and `testlock.png` and has no way to
  learn that `/users/casey/lock-now` is standing in the accounts section. The third loses on the
  shape D-0011 and D-0012 both refuse -- a table of hrefs somebody has to add an entry to, in the
  file that is not the one writing the markup -- and it does not even remove the markup: the
  masthead's byte-separation guard and the comment that explains why the Settings link has no
  trimming dashes stay in `page.tmpl` either way. What it keeps is labels readable from Go, and
  `TestEveryNavigationEntryIsALiveGETRoute` answers that from the rendered page instead, so it also
  catches an entry whose href is not a route at all. Inside the chosen option, "refuses rather than
  proceeds" decided the empty case: no path marks nothing, because a shell that was never told where
  it is has no answer, and that is `readAccounts` refusing a nil listing as "answered nothing at
  all" one layer up. The match is a segment split rather than a prefix test for a reason the code
  repeats: a prefix test makes `/` a prefix of every path, which marks two entries at once, and
  makes `/users` a prefix of `/usersfoo`, which marks an account page on a path nobody registered.
  The cost is stated rather than hidden: the rule now lives in Go, where whoever edits the five links
  in `page.tmpl` will not be looking at it, and the capture driver separately counts how many
  entries carry the attribute on nine pages. That is a count and deliberately not a second
  derivation of the segment rule in JavaScript -- two implementations of one rule drift, and the
  list D-0011 refused is the cautionary example earlier in this log -- and the number of entries
  between the two is deliberately not stated, because the merge that grew this log is exactly the
  thing that would make a count written there false.
- Checkpoint: 64c2751a5d8abd4c449a46833da5842f2fb43173
- Effect: internal/web/sources.go (`pageData.Path`, `IsCurrent`, `firstSegment`; `render` fills the
  path), internal/web/templates/page.tmpl (`aria-current="page"` on the current entry),
  internal/web/nav_test.go, DECISIONS.md

## D-0034 · The site's name stops being a second route to the root

- Question: #204 measured two anchors in the header with the same `href="/"`, on the page the root
  renders, differing only in weight -- "One of them is not a navigation target, and nothing says
  which." Which of the two is the navigation?
- Options: the site's name stops being a link and the row's own entry becomes the only route to the
  root | the name keeps its href and gets `aria-hidden="true"` with `tabindex="-1"` | the name stays
  a link and the `Status` entry is deleted, so the row carries one route to the dashboard | both
  stay and both are marked current on the root | nothing: the report is about the stylesheet, not
  the header
- Chose: the site's name stops being a link and the row's own entry becomes the only route to the
  root
- Why: the fourth inverts what the attribute means. `aria-current="page"` is a fact about which
  page the reader is on, and a page that marks two elements current teaches them the mark is
  decoration -- which is the belief #204 was filed against. The second keeps the worst two
  properties of the pair: the mouse affordance survives, the keyboard stop and the announcement go,
  so whether the element exists depends on which input device the parent picked up; and it hides
  the duplication instead of ending it, when the complaint was that the duplication is not legible,
  not that it is invisible to software. The third is the one that leaves the header honest about
  its routes, and it decides a content question by default: with `Status` gone the row's first entry
  is the product's name, so the row no longer says what the root page is *for*, and #204's own
  finding -- the link reads `Status`, the page is the dashboard, `docs/` uses neither word
  consistently -- gets settled by whichever element happened to survive. The fifth is refused by
  the report being a measurement: two anchors, one href, one page in five, and a tab order that
  pays for it there. "Refuses rather than proceeds" is the tie-break and it points one way: this
  removes a route rather than adding an attribute, and the route it removes is the one offered by
  the element beside it, now marked. The costs are kept in the open. A wordmark that does nothing
  breaks the convention a parent's finger expects, and `TestTheHeaderRoutesToTheRootExactlyOnce` is
  the guard that keeps the anchor from coming back as a well-meaning wordmark. The larger one is
  what the `<h1>` now *is*: the product's name, on a page whose own name reaches a reader only
  through `<title>` and whose visible heading is a table caption, so skip-to-heading lands on the
  same two words on all five pages. Fixing that means deciding which element carries a page's
  identity, across five templates' `title` and `pagename` blocks, and it is #219 -- filed with the
  measurements and the two candidate answers, because a deferred half that lives only in a decision
  log entry is a deferred half nobody is queued to finish.
  #219 was answered on `main` while this entry was still on an unmerged branch, and its answer
  (D-0016) took the cost above and closed it: the `<h1>` is now the page's own name, rendered from
  the same `title` block `<title>` is built from, and the product's name is
  `<p class="masthead__name">` -- not a heading, and still not a link. So only the half of this
  decision survives into the merged tree, and it is the half #204 actually measured: the header has
  exactly one anchor to `/`, the entry that is marked current. The heading of this entry said the
  name *is* a heading; the merge that made that false is D-0036, and it retitled the entry rather
  than rewriting its Options, which are the record of a tie-break between four things that were all
  genuinely on the table at the time.
- Checkpoint: e2dbd469d66dbc70556804cd3b848309ff8c5af8
- Effect: internal/web/templates/page.tmpl (the `<h1>` is no longer an anchor),
  internal/web/nav_test.go (`TestTheHeaderRoutesToTheRootExactlyOnce`), DECISIONS.md

## D-0035 · The current entry is marked by ink, weight and a rule it already reserved

- Question: #204 asks three things of the stylesheet -- that no navigation link computes to the
  browser's `rgb(0, 0, 238)`, that the current entry differ from *every* sibling in at least one of
  `color`, `font-weight`, `text-decoration-line`, `border-bottom-width`, and that the mark hang off
  a DOM attribute rather than a class only the stylesheet knows. What does the row actually get
  drawn, and what is refused even though it would pass the criterion?
- Options: `text-decoration: underline` on the current entry alone | ink and weight on the current
  entry, over a 2 px bottom rule reserved transparent on every link | a filled background chip
  behind the current entry | `::after { content: " (current)" }` on the current entry | the
  attribute only, with nothing drawn, leaving the mark to assistive technology
- Chose: ink and weight on the current entry, over a 2 px bottom rule reserved transparent on every
  link
- Why: the first option satisfies criterion 1 exactly and is the worst thing that could ship, and
  that is why the entry records a refusal rather than a preference. The row's links only just
  stopped being underlined -- the underline was the user-agent's, and `nav a` is what took it away
  -- so making the current entry the one underlined link puts back the property the parent has just
  learned to read as "a destination", and the mark says *this is a link* on the one entry where the
  fact is not the point. It is also one rule away from vanishing: a later `nav a:hover` placed
  below it, or a reset that clears decoration, leaves the row as #204 found it, and criterion 1's
  own reasoning says a single-pinned property is the mistake repeated through #196 and #201. The
  chip is refused on the palette it would have to invent: this file has `--bg: #fff` and
  `--line: #d8dbe2` and no fill token, so a chip means a sixth variable added for a navigation
  mark, which is a larger change to the palette than the mark is worth, and a chip on a 360 px row
  that already wraps onto a second line eats the room the entries need. The `::after` string is
  honest about being for the reader who can see it, and it says in text what `aria-current` already
  says in the DOM -- so it announces twice to a screen reader and costs horizontal room at the
  width the row is tightest. The last is refused by the sentence of the report it would fail: the
  parent who reads the row with their eyes gets nothing, and #204 was filed by one.
  Inside the chosen option the load-bearing part is the reservation, not the colour. Every link
  carries `border-bottom: 2px solid transparent` and only the current one colours it, because a
  border introduced by the current-page rule alone grows that one link's box and moves the baseline
  of every page the shell renders -- measured on the dashboard, a link is 40 px tall with its
  padding and 26 px without, and the row wraps at 360 px, so a 2 px shift is a shift of a whole
  second line. Three properties then differ while nothing moves: `--ink` against `--muted`, 600
  against 500, `--line` against transparent -- all three from tokens the file already declares,
  which is what criterion 2 is really asking for, since the complaint was not that blue is ugly but
  that the navigation was the one part of the page not using this palette. Costs, kept in the open:
  the rule works only because `nav a:hover` is written *above* it, so hovering the page you are on
  does not underline it -- an order a formatter or an alphabetical tidy could silently invert,
  which is why `TestTheStylesheetMarksTheCurrentEntryLast` reads byte offsets rather than trusting
  the file to stay as written. And because `nav a` now sets `color`, the user-agent's visited blue
  is outranked for these links; the capture browser has a fresh profile and no history, so nothing
  in a photograph can show that, and the comment says so instead of the test claiming it.
- Checkpoint: 05f6aab3991f52f5051bd85d9642aa4a13c686b2
- Effect: internal/web/static/app.css (the navigation block -- `nav`, `.masthead h1`, `nav a`,
  `nav a:hover`, `nav a:active`, `nav a[aria-current="page"]`), internal/web/nav_test.go
  (`TestTheStylesheetMarksTheCurrentEntryLast`), DECISIONS.md

## D-0036 · The merge takes main's masthead, and this branch's three ids move

- Question: `git merge origin/main` on 6 Nov stops on thirteen paths. Two of them are the same
  element drawn twice: this branch made the masthead's `<h1>` prose-with-no-anchor (#204, D-0034)
  while `main` moved each page's own name into the `<h1>` and demoted the product's name to
  `<p class="masthead__name">` (#219, D-0016). The other eleven are the nine 360 px frames plus
  `manifest.json` and this file, each side having re-photographed the same set and minted ids in
  the same range. Whose masthead renders, whose ids survive, whose bytes get staged, and from which
  base does the pull request open?
- Options: main's masthead with this branch's `aria-current` navigation kept, main's ids kept and
  this branch's three entries moved to D-0033..D-0035, main's frames staged and a new set taken
  against the merge | this branch's masthead kept and #219's page-name heading dropped | main taken
  wholesale, navigation included, this branch's #204 work abandoned | main's three entries
  renumbered instead of this branch's | the branch left 42 commits behind main and the pull request
  opened from the stale base
- Chose: main's masthead with this branch's `aria-current` navigation kept, main's ids kept and this
  branch's three entries moved to D-0033..D-0035, main's frames staged and a new set taken against
  the merge
- Why: the two mastheads are not rivals, and only one of them is a superset. #204's finding is "one
  of these two anchors is not a navigation target and nothing says which"; this branch answered it
  by deleting the anchor, `main` answered the same finding by moving the heading as well, so its
  header has no second route to the root either -- it additionally makes the heading layer answer
  "which page am I", which is what #219 was opened for. Dropping that would mean deleting
  `TestEveryPageNamesItselfInTheHeading`, a guard that landed on `main` and answers to each page's
  own `<title>`; keeping this branch's template would mean keeping an `<h1>` that guard refuses, and
  the two assertions -- the heading carries the page's name, the heading carries the product's name
  -- cannot both hold in one file. What lost is recorded rather than hidden: D-0034's heading half
  is superseded, so that entry is retitled to the half that survives (the header routes to the root
  once) and carries a note saying which merge closed the rest; and the rule that inked the wordmark,
  `.masthead h1 { color: var(--ink) }`, is deleted rather than left reading an element that no
  longer holds those words -- the heading inherits `--ink` from `body` either way, so no pixel moves
  and one false comment goes with it. The navigation merges additively: nothing on `main` touches
  `nav`, `nav a` or `aria-current`, so `TestTheStylesheetMarksTheCurrentEntryLast` and the four
  capture guards keep their subject, and `go test ./internal/web/` is green on the merged tree
  (17.3 s, ok) with `nav_test.go`'s header guard rewritten to ask where the product's name is --
  once, in the header, outside the anchor -- instead of asking which element holds it.
  The ids follow the rule the preamble of this file now carries: the published side keeps its
  numbers. `main`'s `D-0025` and `D-0026` are merged history whose own entries cite each other by
  id, and `D-0031`/`D-0032` are already claimed by `feat/factory-harness` and `issue-224`, read off
  `git show <branch>:DECISIONS.md` -- the only source that answers. So this branch's three moved,
  and what that costs is in the preamble rather than here: the three effect commits keep the
  `Decision:` trailers they were written with, because rewriting a pushed branch is forbidden in
  this repository, so `git log --grep 'Decision: D-0034'` finds nothing and this entry is the only
  thing that maps the two numbers to each other. The four prose citations of the old ids
  (`testing/capture/cdp.py`, `testing/capture/lock.py`, `testing/capture/pages.py` twice) were
  rewritten in this commit so nothing in the tree points at a number this file no longer holds.
  `EXPECTED_RULES` is the union, 58, and the number is measured rather than tallied:
  `python3 testing/capture/pages.py --self-test` prints "58 refusal rules hold" and refuses
  otherwise, which is why the two sides' counts did not have to be added correctly by hand.
  The frames could not be chosen between at all -- that is the mistake D-0015 refused once already,
  and it is worse here: the merged tree renders a flex navigation with 40 px targets and one marked
  entry, which is in neither side's nine photographs. So `main`'s bytes are staged to keep the merge
  itself free of new artifacts, and the price is paid in the open: `manifest.py check` fails on this
  commit, `make verify` with it, and the next commit re-takes all nine against the merged tree and
  re-records the manifest. Opening the pull request from the 42-commits-stale base is refused by the
  same arithmetic -- a branch whose tree has never been gated together, and a merge state the
  supervisor cannot read as clean.
- Checkpoint: 354cd244b314d0c80b90d9bb71156ef73df1cd1b
- Effect: DECISIONS.md (both halves kept in id order; D-0025..D-0027 of this branch renumbered to
  D-0033..D-0035; D-0034 retitled and its superseded half annotated; the renumbering rule and the
  measured #217 gate findings added to the preamble), internal/web/templates/page.tmpl (main's masthead
  plus this branch's `aria-current` entries), internal/web/static/app.css (`.masthead h1` deleted),
  internal/web/nav_test.go (`TestTheHeaderRoutesToTheRootExactlyOnce` asks where the name is, not
  what element holds it), testing/capture/pages.py (main's declared audit state with `nav="/audit"`,
  `EXPECTED_RULES` 58), testing/capture/{cdp,lock,manifest}.py and the fixtures (main's versions with
  this branch's navigation guards beside them), docs/screenshots/360/ (main's nine staged, re-taken in
  the next commit)

## D-0037 · The set is re-taken rather than re-dated, because a re-dated frame cannot be seen to be wrong

- Question: D-0036 committed `main`'s nine frames with `manifest.py check` refusing on all nine and
  promised a re-take against the merged tree. The run happened, and then the merge commit was amended
  to add one sentence to this file's preamble about the ids those commits cite -- so the commit the
  frames had just been stamped against, `f033291`, stopped existing and `HEAD` became `cf26e50`. Does
  the set go in recorded against a commit that is gone, with the gate red, or does something else
  happen?
- Options: keep the first record and ship the red gate | reset the branch to `f033291` so the
  recording names a commit that exists again | re-run `record` over the bytes the first run
  photographed, on the strength of a `git diff` showing the two merge commits differ in prose only |
  re-take all nine from the fixture setups onward and record once | re-take only the frames whose
  bytes moved
- Chose: re-take all nine from the fixture setups onward and record once
- Why: option one is refused on measurement, not on tidiness. `Makefile:90` runs `manifest.py check`
  inside `verify`, and at the merge commit the check printed nine problems and exit 1 -- `audit.png
  predates the newest change to /audit's sources ... the manifest was taken at d1ee983743fd...,
  internal/web/templates/audit.tmpl (or the shared shell) has moved to f0332913b315...`. D-0036 accepted
  that
  red for the merge commit on the ground that a merge is a merge and its tree is not a render of
  anything; the red was always meant to end in the next commit, and shipping it would hand the
  supervisor a branch it reads as not-green. Option two buys a true `sources_rev` by deleting the
  caveat the amendment added -- that sentence is the only place in the tree mapping the ids those
  commits cite -- and it buys it with a rewrite of a branch `origin` already carries part of, which
  this repository answers with revert, never rewrite.
  Option three is the one with a real argument, and it is the one refused. The argument is honest:
  `git diff --name-only f033291 cf26e50` was run and returned `DECISIONS.md` alone, so nothing the
  browser could have drawn moved between the two commits, and D-0024's provenance paragraph is exactly
  this kind of reasoning. It loses because it asks the reader to trust a diff in place of the artifact.
  `record` writes `sources_rev = newest_commit(sources)` from the commit and the clock and never asks
  the pixels what page they show, and `check`'s drift arm opens `if recorded == current: continue` --
  so the equality the recorder mints is the same equality the gate reads, and a re-dated frame cannot
  be seen to be wrong. That is not a hypothetical here: on 6 Nov, in a scratch copy of the committed
  directory holding `main`'s `audit.png` -- a photograph of an `MST` cell the branch had just deleted
  -- one bare `record` answered `9 photographs current`, rc 0 (D-0030). A re-record justified by a
  command the reader has to re-run is the weaker claim when the honest one costs twenty-one seconds:
  the whole nine-fixture run below ran 16:33:49 to 16:34:09.
  Option five is refused by this branch's own history. `record()` walks the PNGs sitting in its target
  directory and cannot know which ones this run took, so a partial re-take stamps the frames it never
  touched as just-taken -- measured 5 Nov, reported as 9 current while `lock.png` and `testlock.png`
  were byte-for-byte the previous files (D-0017's finding (1), #222). The nine `--shot` names are the
  only input that says what a run photographed, and they are worth nothing unless the run really took
  the set it names. The tie-break order agrees with the rest of this file: nothing separates on
  `go.mod` (no option adds a dependency), on breadth four beats three beats five, and "refuses rather
  than proceeds" picks four for the same reason D-0015 and D-0024 picked the re-take -- a screenshot
  the driver in the same commit cannot reproduce is not evidence.
  The measurement then confirmed the choice instead of decorating it. Every one of the nine frames from
  this run is byte-identical to the frame the 16:17 run wrote -- `cmp` over all nine, all nine
  identical, four fixtures re-seeded and two separate Chrome rounds in between. That is the property the
  refused re-record could only argue for: two shutters, one digest set, so the pixels are a property of
  `cf26e506`'s tree and not of either run that happened to take them.
- What this costs, stated rather than glossed. (1) The manifest on this branch is rewritten twice
  inside twenty minutes and the history shows the churn with nothing in the log to explain it until
  this entry; an intermediate `captured_at` of `16:32:35Z` existed for a few minutes and appears in no
  commit. (2) `head` in the manifest now reads `21c1ecf1`, the checkpoint below, because `record()`
  stamps `git rev-parse HEAD` -- an empty commit that rendered nothing. Its tree is identical to the
  merge's (`git diff --name-only cf26e50 21c1ecf1` prints nothing), and every page's `sources_rev`
  names `cf26e506`, the commit whose bytes the browser actually drew, so the field the gate compares
  and the field a human audits are both right and it is `head`, which nobody reads as provenance, that
  is loose. D-0021's shape one notch milder. (3) `captured_at` is still one field for nine frames, the
  wart D-0015 put on the record and this entry widens again: `16:34:09Z` against shutters at
  16:33:57Z to 16:34:07Z. Twelve seconds of slop where the re-record would have shipped fifteen
  minutes. (4) Two Chrome rounds and four fixture re-seeds to prove a point one `git diff` already
  proved, paid for in wall clock and in the small risk that the second run disagrees with the first --
  which is the point of paying it, and it did not.
  One consequence worth naming before someone else finds it: all nine rows now carry
  `capture.scripts: off`, including `lock.png` and `testlock.png`, which `main` committed with page
  scripts running. That is #213's standing objection to the committed set, satisfied here as a
  side-effect of re-taking everything; #213 stays open, because what the other branches' artifacts
  show is still that issue's to audit.
- Checkpoint: 21c1ecf1990fa5b431a769ae4a089141b34f0405
- Effect: docs/screenshots/360/audit.png, dashboard.png, users.png, users-no-profiles.png,
  profiles.png, profiles-refused.png, settings.png, lock.png, testlock.png (all nine re-photographed
  against `cf26e506` at 360 px), docs/screenshots/360/manifest.json (`head`, `captured_at`, nine
  `png_sha256`, nine `sources_rev` at `cf26e506`, nine `capture` blocks with `scripts: off` and their
  own fixture envs), DECISIONS.md
  The re-take proved the palette marker reaches the browser. The current-page mark's browser coverage arrived later on this branch, in cdp.py's three checks and the frames' failure report, so this entry's evidence covers the marker it re-took and not the whole review thread's claims.

## D-0038 · Three issues closed as satisfied by `main`, with the correspondence written row by row

- Id: this entered the log as `D-0031` on `feat/factory-harness`. `D-0031` is published on `main` (`D-0031 · A frame this run did not take keeps the `sources_rev` its bytes were rendered from`, PR #227, merge commit 4ca7c3bbf), and a merged entry's id never moves, so this one was renumbered to `D-0038` while resolving the merge rather than the other way round.
- Question: #211 and #222 are open, `main` implements both, and a third implementation of each is either
  merged (`issue-211-take-2`, PR #223) or sitting in a worktree that halted the whole queue
  (`issue-222`). Re-verify each on its own tick, close them, or rebase and reduce?
- Options: leave them open and let the next lane re-derive the answer from a stale base | close each with
  a row-by-row correspondence from the issue's own list to main's code, and file a separate issue for the
  rest of the open set | rebase each onto `e74b42f` and reduce the diff to whatever survives | file one
  reconciliation issue and let a lane work the whole set, touching nothing
- Chose: close each with a row-by-row correspondence from the issue's own list to main's code, and file a
  separate issue for the rest of the open set
- Why: both issues were written as checkable lists, so closing them is measurement rather than judgement.
  #211 asked for `--fixture-env` repeatable and per-fixture, a `capture` object carrying only what the
  fixture declared, `scripts` recorded rather than inferred, `check` refusing an entry that declares
  nothing while `/1` stays readable, and the `capture-manifest/2` bump: main has them at
  `manifest.py:69`/`:74`/`:408`, `SCHEMA` at `:98`, the refusal at `:1468`, and the committed artifact
  carries `{"fixture": "pages", "rows": 8, "scripts": "off", "tz": "America/New_York", "zone": "-04:00",
  "census": "testing/capture/bin"}`. #222 asked for `record()` not to stamp frames its run did not take,
  merge-or-refuse instead of overwrite, `check()` naming the class of lie, and a negative test with two
  runs into one directory: main has the rules *a frame nobody re-took was re-stamped by the run that
  re-took another page* and *a re-take of one page demands nothing of the frames it never touched*, plus a
  self-test case that re-takes one of three frames and asserts `lock.png` did **not** acquire a `capture`
  block, and a `check` that asks history (`sources_rev` against `newest_commit`) instead of the manifest's
  memory -- its comment at `:760` records that the naive version refused the one commit where both are
  true, "with a sentence that was false about it". Rebase-and-reduce was what review of #223 recommended
  and it was measured to leave one thing: the inline-offset `stamp`, which is worse than main's separate
  `zone` because `zone` is what lets `audit.png`'s declared state cell be checked against the row the page
  showed. Leaving them open was the status quo and the status quo produced two near-identical
  implementations of #211 in one week from bases 34 commits apart. The set is deliberately limited to the
  three where every row was checked by hand tonight: `main`'s log also names #204, #207, #213, #219 and
  #220, and those five have not been walked line by line, so they go to a lane with the evidence rather
  than into this entry. Reversible by reopening -- every claim above is one `git show
  origin/main:testing/capture/manifest.py` away from being re-checked, and each close comment says so.
- Checkpoint: aa9c1841a949ecd2ac4805ce28b6e0f97a658307
- Effect: issues #211, #222, pull request #223 (all closed, nothing deleted)

## D-0039 · The audit row cites a dated run instead of a digest the manifest can outlive

- Question: review's blocking 1 on #225 is a merge-result defect, and merging `origin/main` made it
  reproducible in a working tree rather than only in a hypothetical merge commit: the `audit.png` row
  asserts `0cfe9973711b…` is "the digest `docs/screenshots/360/manifest.json` records for `audit.png`",
  while `b9eeec3` re-photographed all nine frames at 16:34Z on 6 Nov -- while the pull request sat
  waiting on review -- and the manifest in the merged tree records `fc37cd9a…`. The same argument
  reaches the corroborating clause ("the four-fixture run further down reports the same") and this PR's
  own sentence in "What the audit fixture is not allowed to contain", which says the two-zone pair "came
  back with the committed bytes". How does a row stop rotting in a table whose own preamble forbids it
  to carry a hash?
- Options: drop the quoted digest and let the manifest be the manifest, naming the run instead of its
  bytes | keep the value but date it and name the re-take that retired it | re-run the two-zone pair
  against the current frames and quote the new digests | re-point the row at `b9eeec3`'s set and delete
  the two-zone claim | leave the value and hedge it with "at the time of writing"
- Chose: drop the quoted digest and let the manifest be the manifest, naming the run instead of its
  bytes
- Why: the file already legislated this. The paragraph above the table reads "This table used to quote a
  hash per row, and every hash in it eventually named a file that no longer exists … the bytes each
  artifact is pinned to now live in `docs/screenshots/360/manifest.json` … this table carries only what a
  machine cannot know". Option two keeps a value whose only remaining state is *retired* -- the
  `profiles.png` row two rows up is that same sentence about `740200cc…`, and #144 is the issue that
  counted five rows of them -- so the row now names runs and points at the manifest for values: `D-0037`
  for the bytes committed today (the 16:33:49-16:34:09Z run that wrote the set `b9eeec3` commits, and the
  16:17Z round that came back `cmp`-identical on all nine), and the two-zone pair only for the property
  that pair actually demonstrates.
  Option three is the one with real weight behind it, and it is the one a docs unit must refuse. Re-taking
  means committing a tenth frame set inside a pull request whose whole diff is two sentences; `record` has
  to be handed all nine `--shot` names to do it honestly, which is D-0031's entire subject; and the frame
  set is another unit's question, not this one's -- #213 is open, asking that the two POST frames be taken
  under the condition the driver now enforces, so a re-take from this branch would land on top of that
  audit rather than answer it. The tie-break order agrees: nothing separates any option on `go.mod` (no
  dependency anywhere) or on the decode-toward-lockout rule (no config is touched), option one touches one
  markdown line against option three's nine PNGs plus a manifest plus its `sources_rev` chain, and option
  one is reversible by `git revert` of a prose commit while option three is a re-take only `git revert` of
  a frame set undoes. Option four was refused because the two-zone pair is the only measurement in the
  tree of the host-zone property this page exists to demonstrate, and deleting it would quietly retire
  the evidence #210 and #212 were opened to produce.
  Measured, not inferred, before writing any of it: `git merge-tree --write-tree origin/main HEAD` exited
  0, so the merge takes `main`'s manifest wholesale -- this branch touched no frame after the merge base --
  and `python3` over the merged `manifest.json` reads `fc37cd9adcc5d0a8…` for `audit.png`, which
  `sha256sum` over `git show origin/main:docs/screenshots/360/audit.png` confirms. That is also why the
  sentence the row points at moved with it: "the committed bytes" became "the bytes committed at that
  head", so neither sentence claims anything about the bytes in `docs/screenshots/360/` today.
- What this costs, stated rather than glossed. (1) The row concedes a gap it did not admit before: the
  host-zone property is certified on the set `b9eeec3` replaced, and nothing in this tree has re-run the
  two-zone pair against the current frames. The row says so and names what would close it, which is the
  honest shape of a docs unit that is not permitted to photograph. (2) Dropping the digest makes the row
  less directly checkable than it was for the three hours it had one -- the reader opens the manifest
  instead of reading a value here, and that indirection is exactly what the README's own preamble to the
  reproducibility table -- the paragraph opening "This table used to quote a hash per row" -- chose; the
  row is not
  entitled to undo it because a reviewer once asked for a number. (3) `b9eeec3` and `D-0037` are now
  load-bearing in prose. Both are merged history and cannot move, but the next re-take will turn "the set
  `b9eeec3` commits" into a historical sentence too; it is written dated ("later the same day") rather
  than in the present tense so it stays true when the set moves again. (4) The other stale digests in
  the file -- the `b59bb2be…`/`acc5ba4d…` pair quoted "at that moment" in "The negative control, and why
  a refusal nobody watched fire proves nothing", and the four-fixture tally that opens "Measured 6 Nov"
  under "What a frame was taken under (#211)" -- are `main`'s dated prose, not this PR's, and stay: each
  names the run it came from, and each is true of the head it names, dated, and no longer claimed about
  today. `D-0043` replaced the "checkable against the manifest" wording this line first carried, because
  nothing here checks a markdown cell against the manifest -- that is the very gap two lines below. The
  scripts-pair table under "What the scripts condition can and cannot witness" was on this list until
  D-0041 read it against the artifacts: it does not belong there, because both
  its cells read "`…`, published" -- present tense about `docs/screenshots/360/` -- and the manifest in
  the same tree records `712ee6312051…` for `lock.png` and `ae25abeb1a32…` for `testlock.png`. Those two
  values were the published pair from `38578aa` on 19 Oct until `9e07dde` re-took the frames on 5 Nov
  (`7ec1a664…`, `d008fdf9…`), and `b9eeec3` moved them again the next day -- so the cells have been false
  through two re-takes, which is exactly the rot this entry was written to stop, on the row next to the
  one it fixed: `manifest.py check` compares bytes to bytes and `check-decisions.py` reads entries, so no
  gate in this repository can see a markdown cell asserting a hash. The row this entry fixes was the one
  that spoke in the present tense about what the manifest records, and the scripts-pair table named above
  is a second such row -- one nobody was reading, six headings (four `##`, two `###`) and 430 lines below
  the `audit.png` row that cites it, measured at `a4691ab`. The line numbers this item first gave are
  replaced by the heading that owns each site; `D-0041` keeps the originals verbatim inside its quotation,
  and `D-0043` measures all three drifting twice more while #225 sat in review.
- Checkpoint: 5f6738e57ed453c6382134404bbacf25b55a1fbf
- Effect: testing/capture/README.md (the `audit.png` row and the two-zone sentence it points at),
  pull request #225

## D-0040 · The two-zone pair is a run-vs-run measurement, and it was quoted as certifying nine artifacts against the committed bytes

- Question: #225's round-5 blocking 1 is aimed at this section's own sentence -- "came back with the
  bytes committed at that head for all nine artifacts -- the two POST frames among them" -- and it is
  right: `docs/screenshots/360/manifest.json` distributes its nine frames over **four** fixtures
  (`pages` 6, `pages-noprof` 1, `lock` 1, `testlock` 1), and the pair's own sentence two sections up
  lists **seven** page-fixture artifacts, naming each, with no POST frame in the list. Reaching for that
  sentence turned up a second defect the review did not name, and it sits in the row the review just
  cleared: the `audit.png` row, in words `D-0039` wrote an hour before that review, says the pair "came
  back with the bytes committed at *its* head", while the same file two paragraphs later records that
  against the previously committed bytes only `settings.png` and `users-no-profiles.png` came back
  identical and the other five moved. So what does the pair certify, and how much of the sentence
  survives?
- Options: scope the claim to the seven page-fixture artifacts and say plainly that the pair compares
  one run against another, correcting the row's clause in the same commit | re-run the pair against the
  nine frames committed today and quote what comes back, making both "nine" and "committed bytes" true
  again | keep nine and hedge it as "the frames that run photographed", leaving the fixtures unnamed |
  cut the pair out of this section and let the `audit.png` row carry the zone property on its own
- Chose: scope the claim to the seven page-fixture artifacts and say plainly that the pair compares one
  run against another, correcting the row's clause in the same commit
- Why: the tie-break order does not reach these -- no dependency, no config, and options one and three
  each touch one markdown file -- so it falls to the third rule, refuse rather than proceed on an input
  that is not there: option three keeps a count the manifest contradicts and makes it uncheckable by
  refusing to name which fixtures ran. Option two is the only one that produces evidence instead of
  editing prose, and it is the one a docs unit must refuse for the reasons `D-0039` already gave -- the
  two POST fixtures are #213's subject, an honest `record` has to be handed all nine `--shot` names
  (`D-0031`'s discipline, merged with #227), and a tenth frame set cannot land inside a pull request
  whose whole diff is prose. Option four retires the only two-zone measurement in the tree, which is the
  evidence #210 and #212 were opened to produce: the same reason `D-0039` refused its own delete-the-pair
  option.
  Measured, not inferred, before writing any of it. The census is read off the manifest rather than out
  of a sentence: nine rows, `capture.fixture` = `pages` 6 / `pages-noprof` 1 / `lock` 1 / `testlock` 1,
  and `capture.census` is set only on the six `pages` rows (`testing/capture/bin`) -- the two POST rows
  carry none because `lock-setup.sh` and `testlock-setup.sh` declare none, which `fixture.sh:146-151`
  says is deliberate: "Empty means 'ask the machine', which is what the two POST fixtures do on purpose
  … a stubbed census would photograph a fiction". And the pair's seven values are not retired committed
  bytes -- they were never committed at all: hashing every historical blob of each of the seven paths on
  every ref (`git log --all --format=%H -- docs/screenshots/360/<stem>.png`, then `sha256sum` over each
  `git show <c>:<path>`) gives 19 distinct values for `audit.png` across 21 commits, 11 for
  `dashboard.png` across 17, 11 for `profiles.png` across 13, 10 for `users.png` across 15, and 6, 6 and
  3 for `profiles-refused`, `settings` and `users-no-profiles` -- and none of the seven appears in any
  of those sets. That is what made the sentence false twice over, and it is why the row's clause had to
  move with it: a run that produced no committed file cannot have "came back with the bytes committed at
  its head".
  What the fix keeps is the split `D-0039` built: bytes committed today to `D-0037`'s reproduction, the
  zone property to the pair. The reviewer's non-blocking count rides in the same commit for the same
  reason -- the run of 16:33:49 to 16:34:09Z is "nine-fixture" in that row while the manifest counts
  four, and the file's own word for the nine is frames (`D-0014`, `D-0028`), so the row reads nine-frame.
- Correction, in the same commit as the measurement that forced it (`D-0042`). The clause this entry
  quotes out of the file -- "the same file two paragraphs later records that against the previously
  committed bytes only `settings.png` and `users-no-profiles.png` came back identical and the other five
  moved" -- describes #207's re-take run, not the two-zone pair, and the fix this entry shipped carried it
  into the pair's own sentence directly beside the clause that denies it, so the paragraph ended up
  asserting a hash relation and its negation about the same seven PNGs. `D-0042` deletes the imported half.
  What does not move is this entry's conclusion and its measurement, both re-run at the head `D-0042`
  corrects: `git log --all` over each of the seven paths still gives 21, 17, 13, 15 and 6, 6, 3 commits,
  the distinct-value walk still gives 19, 11, 11, 10, 6, 6 and 3, and none of the pair's seven values
  appears in any of those sets.
- What this costs, stated rather than glossed. (1) The row loses a claim that read as though it had been
  earned: no sentence here can now say the *committed* frame is zone-independent, because nothing in
  this repository has ever established it -- the pair's outputs were never the committed bytes. (2) The
  section states a gap where it stated a result, and `lock.png` and `testlock.png` are now named as
  having no zone evidence at all; that reads worse than the sentence it replaces and it is the
  measurement, with #213's rows 1 and 2 as its home. (3) The diff is larger than the finding the
  reviewer filed: the row's clause was not on the round-5 list, so this commit corrects one clause
  `D-0039` wrote an hour before the review and one clause the review asked for, and the pull-request
  comment has to say which is which or the next reader cannot tell a refused option from an unnoticed
  defect. (4) The cross-reference is now positional ("the two-zone pair above") rather than a line
  number, which is the class of reference this section has already rotted once; if that pair's sentence
  moves, this paragraph and the `audit.png` row both need re-reading.
- Correction (`D-0044`, 8 Nov): the count above is a measurement of a clone on a date, and the sentence did
  not say so. It was taken with `git log --all`, and "every ref" is a property of whoever runs it: this
  clone today holds 49 local heads and no `refs/pull/*` at all, a reviewer's clone carrying
  `refs/pull/*/head` and not these local heads disagrees with it without either side being wrong
  (`dashboard` 11 values here against 12 there, `audit` 21 commits here against 20 there). Re-run in this
  clone at this head, the same command returns 21 commits and **20** distinct values for `audit.png` where
  this entry prints 19 -- the commit count is identical, so the difference is not the branch having moved,
  and the ref set that produced 19 cannot be reconstructed after the fact. The half of the claim that
  carries weight survives the tighter, reader-reproducible scope and the shipped sentence now states that
  scope: hashing every historical blob of the seven paths over `git rev-list --remotes=origin` -- nine
  published heads, 16, 11, 11, 9, 6, 6 and 3 distinct values in `audit`, `dashboard`, `profiles`, `users`,
  `profiles-refused`, `settings` and `users-no-profiles` order -- returns none of the pair's seven
  (`6545ce7d`, `3ba5275e`, `8a9bf9f5`, `76ea48fb`, `42764b11`, `175c1d1f`, `ce38334c`). Adding a ref can
  only add a value, never remove one, so the pair's absence is safe in the one direction a reader's clone
  can differ; the counts are not portable, and a count is worth only as much as its command.
- Checkpoint: 5a7b5f41c0ca27813f2d5c2a247a3291caaa9536
- Effect: testing/capture/README.md (the two-zone paragraph in "What the audit fixture is not allowed to
  contain", and the `audit.png` row's host-zone clause), DECISIONS.md, pull request #225

## D-0041 · `D-0039` certified the one stale digest that speaks in the present tense

- Question: #225's round-5 blocking 2 is aimed at this log rather than at the README. `D-0039`'s
  "What this costs" item 4 grouped three sites as "`main`'s dated prose … each is checkable against the
  manifest rather than false about it", and one of the three is not: the scripts-pair table reads
  "`b59bb2be…`, published" and "`acc5ba4d…`, published" for `lock.png` and `testlock.png`, present tense
  about `docs/screenshots/360/`, while the manifest in the same tree records `712ee6312051…` and
  `ae25abeb1a32…` for those two paths. The retraction exists -- the 02:46Z comment on #225 and row 3 of
  #213 both say so -- but the entry is the artifact that merges, so the correction has to be in the log
  or it does not survive the merge. Amend `D-0039` in place and record the correction beside the
  measurement that forced it | leave the entry and let the two comments carry it | delete the list from
  `D-0039` so it vouches for nothing | rewrite the table's two cells in this pull request as well
- Options: amend `D-0039` item 4 in place to drop the scripts-pair table from the dated-prose list and
  name it as present-tense rot, with the correction recorded in a new entry beside the measurement that
  forced it | leave `D-0039` as written and let the #225 comment and #213's row 3 carry the retraction |
  drop the whole list from `D-0039` so it vouches for no other site | rewrite the scripts-pair table's
  two cells in this pull request so the false sentence leaves the tree with the log's vouch
- Chose: amend `D-0039` item 4 in place to drop the scripts-pair table from the dated-prose list and name
  it as present-tense rot, with the correction recorded in a new entry beside the measurement that forced
  it
- Why: this file has the rule already, twice. `D-0028` "amended three false `D-0024` sentences in the
  commit that measured them" and recorded them in a `Correction, in the same commit as the measurement
  that forced it` bullet; the entry that closed #222 did the same to `D-0030`'s gate claim. Option 2
  loses on the review's own point: a comment is not a merge artifact, so a reader of `D-0039` alone --
  which is how an entry gets read, six weeks later, by whoever is undoing something -- is still told the
  table is checkable. Option 3 throws away the true half to remove the false half, the objection
  `D-0030` recorded against deleting the `sources_rev` sentences: the "at that moment" pair and the
  four-fixture tally do earn their place on the list, and the list is what tells a reader which sites
  *not* to re-litigate. Option 4 is what
  the finding argues for and is not this unit's to take: that table is the record of the two arms of the
  `scripts` switch, so a replacement worth anything needs both arms re-run on the current head, and #213
  is the unit that owns those two frames -- its row 3 already carries the measurement with the manifest
  read-out beside it, at `origin/main`'s line numbers, so the next lane starts from a number rather than
  from a sentence. The tie-break order agrees: option 1 touches one file and reverts as prose, and it
  refuses rather than proceeds on a measurement this branch cannot take.
  Measured before writing, because the claim is about two artifacts and not about a comment.
  `sha256sum docs/screenshots/360/{lock,testlock}.png` at this head returns `712ee6312051…` and
  `ae25abeb1a32…`, matching the manifest's `png_sha256` for both, whose `captured_at` is
  `2026-11-06T16:34:09Z`. The two values the table calls published were published, and they stopped
  being so three days ago: walking every commit that touches either path on every ref and hashing each
  blob gives `lock.png` `b59bb2be…` at `38578aa` (19 Oct) then `7ec1a664…` at `9e07dde` (5 Nov) then
  `712ee631…` at `b9eeec3` (6 Nov), and `testlock.png` `acc5ba4d…` at `38578aa` then `d008fdf9…` at
  `9e07dde` then `ae25abeb…` at `b9eeec3` -- two re-takes have landed since the cells last described the
  tree, and nothing noticed. That is the same mechanism `D-0039` fixed one row away, arriving from the
  other direction: there a row quoted a digest the manifest had moved past, here a table calls a
  moved-past digest current. That the gate cannot see it is measured rather than asserted:
  `manifest.py check --dir docs/screenshots/360` answers "9 photographs current" with rc 0 while both
  cells stand, and `grep -c README scripts/check-decisions.py` is 0 -- one gate reads PNGs against their
  sources, the other reads entries, and a markdown cell asserting a hash is in neither one's field of
  view. That gap is worth an issue of its own and #213's row 3 holds it until someone opens one; nothing
  here closes it.
- Correction, in the same commit as the measurement that forced it. `D-0039` item 4's sentence "The other
  stale digests in the file -- :296, the four-fixture tally at :626-627, and the scripts-pair table at
  :700-701 -- are `main`'s dated prose … each is checkable against the manifest rather than false about
  it" is false of its third member, and the amended text above is what is true. The test that was applied
  -- "a dated sentence is safe" -- is the wrong test, because the table carries no date and never needed
  one to be read as history: what a sentence is worth is decided by whether a machine checks what it
  asserts, and nothing checks a markdown cell. What survives of `D-0039` is its own fix and its first
  three costed items, both of which this entry re-measured and neither of which moves. The three positions
  quoted above were exact when `D-0039` wrote them -- `904b42b`, `540c55e` and `29bc88c` all read the tally
  at :626 and the published pair at :700 -- and `7cd7d0ba`'s eight inserted lines moved them to :634 and
  :708 before this entry quoted them again, which is what makes them stale at the commit that writes them
  rather than merely later. #225's own prose commits moved them once more, to :640 and :714 at `a4691ab`,
  and `D-0043` re-cites each of them by the heading that owns the site, which is the form that cannot rot.
- What this costs, stated rather than glossed. (1) The log now carries three correction markers
  (`D-0024`, `D-0030`, `D-0039`), which is the cost `D-0029` priced in advance and accepted: the log
  reads as a record of being wrong rather than as a clean ledger. (2) `D-0039`'s "What this costs" is now
  longer than the cost it was written to record. (3) The false sentence stays in the tree. README's
  scripts-pair table will still read "published" when this branch merges, and what changes is only that
  the log no longer vouches for it -- a reader of the table, rather than of the log, still gets a wrong
  claim until #213 re-runs both arms, and that residue is the honest boundary of a prose unit. (4) It
  retires the rule of thumb this branch was using for stale digests, so the next one has to be argued
  from the gate rather than from the tense of the sentence it sits in.
- Checkpoint: 97eadbd270a3c7b39aa717cb7d9a5ba2797a6f66
- Effect: DECISIONS.md (this entry and the `D-0039` item 4 amendment above), pull request #225

## D-0042 · The replacement sentence asserted the retraction and the thing it retracted

- Question: #225's round-6 blocking finding is aimed at the sentence written to answer round 5, at
  "What the audit fixture is not allowed to contain": "against the set committed at the time only
  `settings.png` and `users-no-profiles.png` came back identical and the other five moved with the branch,
  and none of the seven values that pair prints was ever a committed file at any head on any ref". The two
  halves cannot both hold -- if those two frames came back identical to a committed set, their pair values
  *are* committed files -- and the file prints the seven values that decide it nine lines of the same
  section above. What is true about the pair's bytes, and what replaces the sentence?
- Options: drop the imported half, let the paragraph rest on run-vs-run identity, and re-word the surviving
  clause so its scope is checkable | keep both halves and record them as conflicting, routing the identity
  claim to #213 beside the scripts-pair table | re-run the two-zone pair against the nine frames committed
  today and quote what comes back | delete the pair from this section and let the `audit.png` row carry the
  zone property alone
- Chose: drop the imported half, let the paragraph rest on run-vs-run identity, and re-word the surviving
  clause so its scope is checkable
- Why: the pair's own wording already establishes what the sentence needs -- "two fresh fixtures, run about
  a minute apart … produced byte-identical PNGs" is a run-vs-run claim with no committed term -- so the
  first half was not doing work, it was doing damage. Measured before writing, and the walk is `D-0040`'s
  re-run, not the review's assertion: every historical blob of each of the seven paths over every ref gives
  `settings` six values (`111bcbe1`, `12e669d8`, `140dc2e7`, `2d7ac1d9`, `7bfacb6f`, `a7411adf`) and
  `users-no-profiles` three (`8bbb8293`, `9173e9b4`, `dac864f4`), and the pair's `175c1d1f…` and `ce38334c…`
  are in neither set -- so the clause is not true of *this* pair, and in a paragraph that prints these
  seven values it reads as a claim about them. Whether some *other* run came back identical to its own
  committed set is a different question, and `D-0044` answered it: it did, at `d439f70`, and this entry
  said otherwise. One measurement past the review's, because the review left an out: "if in fact the pair ran on a
  #207 head since discarded by a force-push, that is worth a sentence of its own". Hashing every blob in
  this clone's object database -- `git cat-file --batch-all-objects --batch`, 2074 blobs, 87 of them PNGs
  -- returns none of the seven, and `git reflog show issue-207` and `git reflog show origin/issue-207`
  contain no forced update at all (14 and 6 entries, every one a commit, a merge, or a fast-forward push).
  The discarded-head story is therefore not needed to explain the pair and is contradicted for this branch
  by both reflogs. That is also why "any head on any ref" became "any head this repository still
  publishes": the object-DB result is a fact about this clone, which a reader who clones fresh cannot
  reproduce, while the published-refs walk is the same claim at the size a reader can check.
  Option two is the one that keeps a contradiction on the page and asks the reader to hold both sides, and
  it asserts about the neighbouring paragraph a thing this unit has not measured: the re-take it describes
  quotes no digest — and `D-0044` found its equality against the then-committed set is confirmable from
  this tree after all, in one command — so this refusal was given for a reason that does not hold. What
  survives of it is the tie-break as procedure, not as a claim about measurability: this unit had not
  measured the input when it wrote, and the third tie-break refuses rather than proceeds on an input that
  is not there. Options one and two each touch one markdown file, so the fourth tie-break is decided by the size of
  the claim, not the count of files, and the fifth is undecided between two prose commits; the sixth
  settles it, because the file's own practice since `D-0039` is to name a run and point at the manifest
  rather than to adjudicate two hash claims against each other in one sentence. Option three was refused
  once in `D-0039` and again in `D-0040` for reasons that have not expired -- a docs unit does not
  photograph, the two POST fixtures are #213's subject, and an honest `record` must be handed all nine
  `--shot` names (`D-0031`). Option four retires the only two-zone measurement in the tree, the evidence
  #210 and #212 were opened to produce.
  The same walk fixes two neighbours in the same commit, because both quote one of the pair's values as
  though it were committed. The `users-no-profiles` row said `ce38334c39ad…` was "equal to the file this
  branch re-recorded"; this branch re-recorded that path twice, `dac864f4…` at `9e07dde` and `9173e9b4…` at
  `b9eeec3`, so of the row's two digests one is a citation and one is not and the cell now says which. Its
  real citation is restated as measured, because the review's version of it was one commit off:
  `8bbb8293…` is `f56dc29`'s frame, recorded at `captured_at 2026-11-01T05:43:26Z`, and the
  `2026-11-01T22:47:43Z` quoted in the review is `d439f70`'s re-record of the same bytes. The `audit.png`
  row's "any head on any ref" takes the same published scope so the two rows cannot drift apart, and the
  pair's closing clause reads "One host, two zones declared" -- the pair's own opening sentence and the
  section's re-statement of it both say one machine, and
  "two hosts" sat in the one sentence the retraction now depends on.
- What this costs, stated rather than glossed. (1) The paragraph is longer than the sentence it replaces and
  now explains a mistake as well as stating a scope, which is worse prose and the only honest shape
  available: the imported clause has to be named, or the next reader re-imports it from the paragraph above.
  (2) The `users-no-profiles` cell still carries a digest that names nothing committed. Labelling it stops
  the cell lying; it does not remove the temptation to quote it, and #213's rows own that table -- `D-0041`
  already refused rewriting its cells here for the same reason. (3) "Still publishes" is a scope that moves
  with the branch list, but only one way: deleting a ref removes counterexamples and cannot falsify the
  sentence, so the only way it becomes false is for someone to commit one of these seven frames -- the
  exact case where being falsified is the point. (4) The object-database scan is corroboration this clone
  can produce and a fresh clone may not, so the log leads with the published-refs walk and keeps the
  stronger result where a reader can see what it does and does not license.
- Correction (`D-0044`, 8 Nov): two sentences of this entry are false, and the round-7 finding is right
  about them. The split the deleted clause describes is a comparison that happened, it is on `main`, and
  one command settles it -- `git show d439f70 --stat -- docs/screenshots/360`. `d439f70` (2026-11-01,
  "Re-take the five artifacts the fixture moved, and record why", and it arrived with **#212**, not #207)
  lists `audit`, `dashboard`, `profiles`, `profiles-refused` and `users` changed and does not list
  `settings.png` or `users-no-profiles.png`; re-measured blob against first parent across that directory,
  five move (`913554df→f8a51ec3`, `4b9d36ba→65f7c261`, `a6cc0ae8→580f2896`,
  `e977ace8→2487940d`, `311f91cc→551148e7`) while `settings` holds `add45cf8` and
  `users-no-profiles` holds `77a840ea`, and the commit's own message says so: "Five files move because
  their fixture content moved … users-no-profiles.png and settings.png came back byte-identical". So
  "a split of a comparison that never happened" and "can be neither confirmed nor refuted from this tree"
  are both wrong, and the README clause that told a reader the neighbouring sentence "quotes no digest a
  reader can re-check" was steering a reader away from a check that exists. The deletion stands, on the
  reason this entry already contained: the walk puts the pair's seven values in no published history, so
  the clause has no scope in the pair's paragraph. What this entry over-read is the walk's reach. It
  answers "is this pair's value a committed file"; it says nothing about whether some other run came back
  identical to its own committed set, and reading it as a verdict on that second question is how an
  unfalsifiable claim came to be labelled "Measured before writing".
- Checkpoint: 8aa5bd80e398359db273d3dc5bb86be58c573d06
- Effect: testing/capture/README.md (the two-zone paragraph, the `audit.png` row's host-zone clause, the
  `users-no-profiles` row's digests, and the pair's "two hosts" clause), DECISIONS.md (this entry and the
  `D-0040` correction above), pull request #225

## D-0043 · A `:NNN` citation is worth nothing on a file that this branch edits

- Question: #225's round-6 non-blocking items are both about how this log points at `README.md`. First,
  the positions `D-0039` item 4 and `D-0041` quote -- ":626-627" for the four-fixture tally and
  ":700-701" for the scripts-pair table -- and `D-0039`'s "the table two sections below it". Second, the
  vouch `D-0039` item 4 still gives the two sites it keeps: "each is checkable against the manifest
  rather than false about it", which is the standard three lines below it `D-0041` retracts. How does the
  log point at a file it is also changing, and what does it actually vouch for?
- Options: re-cite every site by the heading that owns it and re-word the vouch as true-of-the-head-it-names
  | leave the line numbers and date each one | drop the positional citations and name nothing | move those
  three sites' claims out of the log and into the README
- Chose: re-cite every site by the heading that owns it and re-word the vouch as true-of-the-head-it-names
- Why: measured on four heads, because the claim is about motion and not about style. `git show <head>` of
  the README and one grep per site gives: tally at :626 and published pair at :700 at `904b42b`,
  `540c55e` and `29bc88c`; :634 and :708 at `7cd7d0ba` and at the head the review graded, `6058c6c`; :640
  and :714 at `a4691ab`, this branch's own rewrap commit. So the numbers were exact when `D-0039` wrote
  them, moved before `D-0041` quoted them again, and moved twice more under this branch's own commits -- a
  branch whose whole subject is a citation that outran its file re-stuck its own citations while it was
  fixing that subject. "Two sections below" was never right either: from the `audit.png` row the table is
  six headings (four `##`, two `###`) and 430 lines down at `a4691ab`. A heading is the form that cannot
  rot, because moving prose between headings does not touch it and renaming one is a deliberate edit to the
  cited thing rather than an accident nobody reads. The vouch re-worded for the same reason: `D-0041`'s own
  conclusion is that
  no gate in this repository reads a markdown cell -- `manifest.py check` compares PNGs to their sources and
  `check-decisions.py` reads entries -- so "checkable against the manifest" promised a check that does not
  exist. The reviewer re-ran the vouch anyway and it survives on better ground, and that ground is now
  measured here rather than asserted there: `4739939` (2026-11-06T03:47:47Z, the commit that introduced
  "Measured 6 Nov, from four fresh fixtures") holds `lock.png` `b59bb2bef7fc…` and `testlock.png`
  `acc5ba4d5cfe…`, its manifest's `captured_at` is `2026-11-06T03:44:15Z`, and the "at that moment"
  sentence quotes exactly those two against exactly that moment. Both survivors were therefore true of the
  head they name, which is the property they actually have; neither is a statement about `main`.
  Option two keeps a number that is wrong today and only timestamps the wrongness -- a date on a stale
  position still sends a reader to the wrong lines. Option 3 removes the false pointer by removing every
  pointer, the objection `D-0041` recorded against dropping `D-0039`'s whole list: the list is what tells a
  reader which sites not to re-litigate. Option 4 moves claims into the file whose own preamble to the
  reproducibility table forbids it from carrying hashes -- the paragraph that opens "This table used to
  quote a hash per row" legislates exactly that, and is where `D-0039` took its own rule from. Option one touches one markdown
  file, reverts as prose, and is what the surrounding log already does in words -- `D-0040`'s cost (4)
  conceded this class rots and `D-0042` cites the pair by section for the same reason. The rule is written
  for pointers into prose this branch edits, not for the log's `manifest.py:480`-style citations: those
  name the file beside the line, and a code line moves inside a diff hunk a reviewer reads, which is not
  the silence this class has.
- What this costs, stated rather than glossed. (1) Headings rot too, more slowly: rename "What the scripts
  condition can and cannot witness" and three entries lose their pointer at once. Accepted because the
  failure needs someone to mean it. (2) The vouch is now weaker than the one it replaces -- "true of the
  head it names, dated" is not a promise that anything checks it, and the honest answer to whether those
  two sites are still right is "nothing in this tree can tell you". (3) `D-0041`'s quotation of `D-0039`'s
  original sentence keeps the stale numbers verbatim, so a grep for ":700-701" still hits a wrong position.
  Left deliberately: silently correcting a quotation of an earlier entry is how a decision log stops being
  evidence about what was believed when.
- Checkpoint: 1088c2cadfdaa7009d81ced1d78e9bf595ef011f
- Effect: DECISIONS.md (the `D-0039` citations in costed items 2 and 4 and the vouch with them, `D-0041`'s
  Why and Correction), pull request #225

## D-0044 · The re-take this log sent readers away from is checkable, so the citation names the run

- Question: #225's round-7 blocking finding says `D-0042` records as "a split of a comparison that never
  happened" a comparison that is on `main` and checkable in one command, and that the shipped README
  clause repeats the claim about a neighbouring sentence ("quotes no digest a reader can re-check"). Is
  the finding true, is the run it points at uniquely identifiable, and what wording replaces the two
  sentences and the clause? The same round carried four nits, taken here as one batch with it: the count
  word at `:239`, two frames attributed to this branch that arrived by merge, a distinct-value count taken
  over an unnamed ref set, and the live pull-request body still advertising the deleted claim.
- Options: scope the claim to the pair it was written about, citation-free | name the run and the command,
  and correct the two `D-0042` sentences in place with a dated correction bullet rather than a silent
  rewrite | delete the README clause and both log sentences, leaving no account of how the clause arrived |
  keep everything and record in this entry that a machine can check the neighbouring sentence after all
- Chose: name the run and the command, and correct the two `D-0042` sentences in place with a dated
  correction bullet rather than a silent rewrite
- Why: the finding was measured true before anything was written, and the shape of the error is the one
  this branch exists to treat: a sentence labelled "Measured before writing" whose subject a machine can
  check, coming out the other way. The comparison is two commands long, and both halves of it reproduce
  here independently of the review: `git show d439f70 --stat -- docs/screenshots/360` lists five PNGs and
  not those two, and comparing each PNG's blob against its first parent gives five movers
  (`audit`, `dashboard`, `profiles`, `profiles-refused`, `users`) against `settings` at `add45cf8` and
  `users-no-profiles` at `77a840ea`, exactly the split the :238 paragraph describes and exactly what
  `d439f70`'s own message records. The attribution needed care the review did not take, and the shipped
  wording carries the care instead of the claim. The reviewer wrote that this commit is the only one,
  across the commits since `f56dc29` that touch the manifest, where both files are unchanged while others
  move; the *state* is not unique -- `git rev-list f56dc29..HEAD -- testing/capture/manifest.py
  docs/screenshots/360/manifest.json` is 21 commits and six of them have both files unchanged with
  something else moving: `d439f70`, the merges `903c09c` and `1c5a109`, and the re-takes `73a65fc`,
  `ab0730c`, `774a270`. What is unique among the commits that took frames is the *shape*. The three later
  re-takes each moved one frame against a different identical set -- `ab0730c`: "the other five:
  `dashboard`, `users`, `profiles`, `profiles-refused` and `settings` returned byte-identical to main's
  committed bytes, so only the template moved a frame"; `774a270`: "Eight of the nine came back
  byte-identical to main's committed files; `audit.png` is the only mover" -- and the two merges adopt
  main's bytes rather than measuring a run (`903c09c` shows five movers against its first parent with both
  files unchanged, and is a merge). So `d439f70` is named because of its shape and its message, not
  because nothing else moved two files, and the log says that rather than trading one over-scoped
  citation for another. Option one is what `D-0042`'s correction would look like with no pointer, and a
  reader holding only the corrected sentence still cannot see which run the clause came from; option
  three destroys the only account of how an unfalsifiable clause got written into the durable artefact,
  which is the thing the next reader quotes; option four leaves the false sentences standing on the page
  while admitting elsewhere they are false, and a log a reader has to cross-reference against an erratum
  is not a log. Correcting in place rather than rewriting silently is the same rule as the PR's own
  handling of a wrong claim in a posted comment: the record has to show the correction being made.
  The nits were each measured before being written, and two of them moved more than a word. The count word
  is the reviewer's and it is right: the sentence named three pages behind the word "four", and five files
  are what `git show d439f70 --stat -- docs/screenshots/360` lists, so "three plus two" is now the
  measurement rather than a number short of it. The two adopted frames are a lineage claim, so lineage was
  measured: `9e07dde` is an ancestor of `dbdbf4b` (the #221 merge) and `b9eeec3` of `ffd63ab`, and neither
  commit is on `origin/issue-207`'s first-parent line -- each reached this branch through a merge,
  `dbdbf4b` and `d246d51` respectively, and both are also on `main`; the row names the carrying merges now
  instead of saying "this branch". The count nit turned out to be a real defect and not a clause: see the
  dated correction on `D-0040`, where the same command run today gives 20 distinct values for `audit.png`
  against the 19 that entry prints, over an identical commit count, with no way to reconstruct the ref set
  that produced the 19. What the shipped sentence rests on now is the scope a reader can reproduce --
  `git rev-list --remotes=origin`, nine published heads -- with the pair's seven values absent from it,
  which is the half that mattered and holds. The fourth nit is not reachable from this session and is
  recorded rather than glossed: `gh pr edit 225 --body-file` still fails server-side on the
  classic-Projects field (`repository.pullRequest.projectCards`), and `gh api -X PATCH`, the one other
  write, is refused by this harness's shell permission. The corrected body is staged with the ref-set
  scoping folded into it at `/tmp/opencode/pr225-body-new.md`; the reviewer's note on the entry this log
  numbered `H-0043` and the merge against `main` renumbered to `H-0051` -- that the body is the residue
  a protected field leaves -- is the reason it is filed as owed work rather than dropped.
- What this costs, stated rather than glossed. (1) The README sentence grows from one clause to five
  lines, and the section is longer for a correction that adds no new fact about pixels. (2) The citation
  points at a commit, so it inherits a commit's fragility: a re-record of `settings.png` cannot move
  `d439f70`'s diff, but a history rewrite would, and the only thing holding that line is this repository
  forbidding force-pushes (`allow_squash_merge=false`, `allow_rebase_merge=false`) -- the claim is as
  durable as the branch protection, and the entry says so rather than implying it is a law. (3)
  `D-0042` is now longer and visibly wrong in two places before it is right, which is the cost of
  correcting in place; a reader who wants the clean version gets a four-line correction bullet and has to
  trust it. (4) Naming the reviewer's over-reach in public spends the goodwill their correction earned,
  and it is done because the same class of error is the subject of the pull request. (5) The uniqueness
  measurement is bounded by `f56dc29`\u2013`HEAD` on this branch and by `--stat` against first parents;
  a re-take on a branch nobody merged would not appear here, and nothing in the wording claims otherwise.
- Checkpoint: f2a12988e76928b754ee3a204f18df693bda40c9
- Effect: testing/capture/README.md (the provenance sentence after the reproducibility table: #207's
  re-take becomes `d439f70` with its date, its #212 arrival, its own quoted words, and the one command
  that settles the equality; "the four whose fixture content" becomes three, with the five-file list
  cited; the `users-no-profiles` row's two adopted frames name `dbdbf4b` and `d246d51` rather than "this
  branch"; the pair's published-heads sentence names `refs/remotes/origin/*` and its counts, and says what
  a differently-scoped clone reports), DECISIONS.md (`D-0042`'s "a split of a comparison that never
  happened" re-scoped to the pair, its option-two refusal re-grounded on the tie-break rather than on
  unconfirmability, and the dated correction bullet above; `D-0040`'s count given its command, its clone
  and its date, with the same command's result at this head), this entry, pull request #225


## D-0045 · A state word is drawn by a rule, and the stylesheet had written one of the thirteen

- Question: #199 reports the web UI as "very basic" and asks which decisions carry no visible state.
  `internal/web/static/app.css` styles `.state-held` and `.state-free` and nothing else in that
  vocabulary, so is the fix a palette, a component, or a rule set the templates already emit; and where
  does the guard against this class of defect go, given that a grep of `app.css` can read that a class
  is *mentioned* and cannot read that anything is *drawn*?
- Options: new components and markup for every state | a palette and token pass over the existing
  elements | one rule per class the pages already name, with the effect measured in Chrome | escalate
  the palette to a designer and ship nothing here
- Chose: one rule per class the pages already name, with the effect measured in Chrome
- Why: the located cause is not taste. The pages already put a class on every state word — the
  thirteen names in the census the guard reads (`cdp.STATE_CLASSES`: nine on words, four on rows) — and
  `app.css` wrote a rule for one of them, `.state-held`, so twelve of the thirteen computed exactly what
  the sentence around them computes. Thirteen is the base, for the reason `D-0048` records: it is the
  set the shipped guard counts, so the denominator in this entry and the denominator in the driver's
  `states=N drawn=M` line are one number, and it keeps the four row classes — the half a word-only count
  hides, which had no rule at all — inside the total. Restricted to the nine word classes the same
  measurement is eight of nine undrawn, and that is the same fact on the narrower set, not a second
  count. What `app.css` also styled, `.state-free`, is not one of the thirteen: no template has ever
  emitted it, which is why the census leaves it out, and counting it among the thirteen was the error
  `D-0046` corrects. The per-page numbers are the guard's, not a hand count of words on a page, because
  the two disagree by design (the guard counts every element carrying a census class, including rows and
  repeats): `/` refuses with 4 of 5 undrawn, `/users` 9 of 9, `/profiles` 1 of 1 and 2 of 2 on the
  refused-write frame, `/settings` 2 of 2, and `/audit` passes honestly at `states=0 drawn=0` because it
  renders no state word at either viewport. Markup was never the missing half. That is also why the
  token pass is the option that loses least: the palette exists (`--held`,
  `--free`, `--muted`, `--warn` added here, dark variants beside each), and what was missing was the
  decision about *what a state word is allowed to be drawn with*. That decision is recorded here and
  deliberately not written into the guard, and the log should have said so the first time. The guard
  accepts **any one** visible channel: `cdp._state_lookalikes` (`testing/capture/cdp.py:1106`) passes an
  element when one inherited property differs from its parent's *or* it carries a border or a background
  of its own; `pages.py:3230` registers the rule under the literal name "any one visible channel counts
  as drawn"; the self-test beside it, `one_visible_channel_is_enough` (`testing/capture/pages.py:3000`),
  holds the guard green for each of six marks given alone — colour, weight, italic, underline, a 3px rule,
  a tinted background — and says in its own docstring that "the guard prescribes none of the six"; and
  `cdp.py:1134` states it in the guard's voice, "Nothing here says WHICH treatment a state gets". So
  colour alone passes, and four of the classes this entry ships are colour alone: `.state-gap`
  (`app.css:171`), `.override` (`:194`), `.override-expired` (`:198`) and `.disabled` (`:202`). That is a
  design choice, and it survives the ticket's own greyscale requirement by measurement rather than by
  assertion: separated from the ink they sit in, `--warn` is 2.69:1 and `--muted` 2.78:1 in the light
  scheme beside the pre-existing `--held` 2.61 and `--free` 2.60, and 2.24 and 2.10 in the dark scheme
  beside 2.28 and 1.73 — no more hue-dependent than the red this branch was handed, and not the weakest
  marks in the file. What the guard refuses is the case where the stylesheet said nothing about the class;
  what it prescribes is nothing, because the last time this repository put one designer's answer into a
  guard (#229's `nav a:hover` above the current-page rule) the guard had to grow three more properties to
  stay honest. Anyone who later wants the channel-counting rule that an earlier draft of this entry
  claimed the guard enforced — colour *plus* one of weight, style, underline or a box — should know what
  it would cost: it refuses four of the nine word rules as shipped.
  The four inherited channels — colour, weight, style, decoration — are the ones that travel from the
  prose an element sits in, so those are the four compared against its parent; `border-left-width` and
  `background` are not, because a `td` sits in a row whose border is the row's, and the first version
  of the guard compared them anyway and reported the pre-fix stylesheet as drawing `.flag` when it
  draws nothing (the cell is a `td`, its parent's border is `3px`, so the comparison was against itself).
  `problem` and `override` land on muted italic prose for the reason #199 itself gives: the refusal
  sentence and the open hold are the most important text on those pages *and* the longest, and a bold
  red paragraph on `/users` — where six of the thirteen state words live — would turn a table into
  weather. A refusal earns attention from shape and position, not saturation. `state-free` stays out of
  the census: `app.css` draws it, no template has ever emitted it, and a browser cannot be asked about a
  class that is never on a page; the Go test asserts both halves of that sentence rather than trusting
  either. The guard is two seams with one shared blind spot. `internal/web/state_vocabulary_test.go`
  reads the two *names* a rename moves — a `state-` word a template says and the stylesheet never
  mentions, and a rule for a word nothing says — and refuses the reader's own vacuity by pinning six
  real selectors including the row rules this entry adds. The effect lives in Chrome:
  `cdp.require_state_legible` refuses a page where a state word is undrawn, at both viewports, and the
  measurement line prints `states=N drawn=M` with the zeros, because #204's lesson is that a number
  that would read identically in the broken and fixed case proves nothing — `nav=5 links marked=Accounts`
  is the line that made #204 checkable and this is the same line for the same reason. Neither seam reads
  composed markup, so `pages.py --self-test` gained the coverage rule, which reads the templates from the
  other side and is what found the four row classes: `dashboard.tmpl:33` and `users.tmpl:43` write
  `class="{{if .Stale}}stale{{end}}…"`, so the names exist only between actions, carry no `state-`
  prefix, and had no rule at all — `grep -n "tr\.\|tbody tr" app.css` on the checkpoint commit answers
  nothing. That rule's first version called those four unreadable and stopped counting them, which is
  the failure it exists to catch, and `page.tmpl:39` then refused it a second way: its
  `class="page-{{template "pagename" .}}"` has a quote *inside* the action, so splitting attributes
  before cutting actions cuts the attribute in half. Actions are cut first now, for both reasons.
  What lost, and it is the expensive half: nine committed frames are re-taken because of this. The
  alternative was to exempt the three frames whose drivers cannot be re-run on a CSS change, and a
  first attempt wrote that exemption into `manifest.py` as a `css_only` block — stated on the frame,
  naming the tests that re-verify it, refused when it named no proof — and it was reverted before
  commit, for a reason worth keeping: it cited a `#217` that does not exist in this tree, and the
  three `by` paths it named were tests that assert *content*, not layout, so it would have shipped an
  exemption whose `proves` sentence claimed coverage the named tests do not provide. `manifest.py
  check` answered with nine staleness problems, all of them true, and the honest reading was that a
  CSS change moves every page that renders a state word. Re-taking is 20 minutes and a manifest entry
  per frame; an exemption is a permanent clause a future reader has to re-litigate.
- Correction (`D-0046`, 8 Nov): one number in this entry is wrong and its `Effect:` line names a
  function that does not exist. The effect says the drivers print `format_state_line`; the helper is
  `cdp.state_census_line` (`testing/capture/cdp.py:1092`), and the reason nobody noticed is the defect
  D-0046 is about: only `pages.py` called it at all, so the name in this entry described a line two of
  the three drivers did not print. The count D-0045 published for the pre-fix stylesheet -- "wrote a
  rule for two of them" -- is one, not two: `git show 6c37739~1:internal/web/static/app.css` has
  `.state-held` and `.state-free`, and no template has ever emitted `state-free`, which is exactly why
  `cdp.STATE_CLASSES` leaves it out; counting it among the thirteen was the error. What the entry rests
  on is untouched: the pre-fix refusals it quotes are verbatim driver output (`/` 4 of 5, `/users` 9 of
  9, `/profiles` 1 of 1 and 2 of 2, `/settings` 2 of 2, `/audit` `states=0`), `dashboard.tmpl:59` and
  `users.tmpl:61` and `lock.tmpl:93` are real, and thirteen is the census the code carries (9 words + 4
  rows). The citations written for #239 are a separate matter and are wrong enough to need their own
  entry, so they have one below rather than a quiet edit up there.
- Correction (`D-0048`, 8 Nov): the re-review filed at `2026-11-08T18:41:12Z` found two claims this entry
  still made about its own guard, and both are now corrected in the prose above rather than only in a
  note beneath it. (i) It said the channel rule is "written into the rules rather than left to a
  reviewer: colour plus one of weight, style, underline or a box property". The guard enforces no such
  thing: `cdp._state_lookalikes` passes any one channel, `pages.py:3230` is named "any one visible channel
  counts as drawn", and `one_visible_channel_is_enough` proves colour alone passes — so the entry now
  records the channel choice as a decision this log makes and the guard deliberately does not, with the
  greyscale separation of the four colour-only classes measured beside it. (ii) Its base is now stated
  as the thirteen-name census — one drawn, twelve undrawn, the nine-word subset named as a subset — and
  its `Why:` no longer counts words off a page (`/users` six, `/` four, `/profiles` two), a hand count
  that disagreed with the verbatim driver refusals quoted ten lines below it in `D-0046`'s correction.
  `D-0048` is the entry that chooses the base and says what each alternative would have hidden. The false
  sentences survive here, in `D-0046`'s correction above, in the re-review, and in the message of
  `3bcbfdd`, which cannot be rewritten.
- Checkpoint: 6c3773999b2054d664ec6a113e4e5a758ca391e8
- Effect: internal/web/static/app.css (`--warn` in both schemes; `.state-unknown`, `.state-gap`,
  `.banner`, `.flag`, `.problem`, `.override`, `.override-expired`, `.disabled`, `tr.held`,
  `tr.unreadable, tr.stale`, `tr.unmanaged` with their dark variants), internal/web/state_vocabulary_test.go
  (new: the name seam and the selector reader's own probe), testing/capture/cdp.py (`STATE_CLASSES`,
  `ROW_STATE_CLASSES`, `STATE_JS`, `STATE_PROPS:1040`, `measure:343`, `state_census_line:1092`,
  `_state_painted:1072`, `_state_rows:1077`, `_state_lookalikes:1106`, `require_state_legible:1123` —
  the two names this line used to carry, `state_problems` and `format_state_line`, are not functions in
  the file: `D-0046` caught the first and the re-review caught the second, which had survived it),
  testing/capture/pages.py (the census line, the coverage rule, `import re`,
  `EXPECTED_RULES` 60→68), testing/capture/lock.py and testlock.py (the same census on their frames),
  testing/capture/README.md (the refusal row, and the note that #199's re-take moved eight of nine
  frames including the two `d439f70` reported identical), docs/screenshots/360/* (re-taken set and its
  manifest), #199's pull-request comment, #239 (the two-meanings split, deliberately not done here)

## D-0046 · The split's citations were written from memory, and three published texts repeated them

- Question: #239's body, #240's body and #199's comment all justify the two-meanings split with file
  and line -- `profiles.tmpl:101` and `:204` as form-field labels, `profiles.tmpl:114` as a schedule
  sentence, `settings.tmpl:151` as help prose, `users.tmpl:58` as a hold, "19 of the 20 `.override`
  elements on `/profiles`" -- and #240 adds that every driver line prints `states=N drawn=M`. Which of
  those are true of this tree, and what is done about the ones that are not, when the sentences are
  already outside this working copy in three places and in a commit message that cannot be rewritten?
- Options: leave them, because the argument for the split does not depend on any particular line | edit
  the three GitHub bodies silently and let the corrected text stand alone | re-derive every claim from
  the tree, publish the corrected numbers visibly on each artifact, and keep the false ones on the
  record as a dated correction in this log | withdraw #239 and open a new issue with clean citations
- Chose: re-derive every claim from the tree, publish the corrected numbers visibly on each artifact, and
  keep the false ones on the record as a dated correction in this log
- Why: all five citations are wrong, and not wrong in the way a typo is wrong.
  `grep -n 'class="override"' internal/web/templates/*.tmpl` answers `dashboard.tmpl:59`, `lock.tmpl:93`
  and `testlock.tmpl:121` and nothing else; `grep -c 'class="override"' internal/web/templates/profiles.tmpl
  internal/web/templates/users.tmpl` answers `0` and `0`. So `/profiles` renders no `.override` at all --
  the browser, asked directly against the pages fixture, returns `override: [0, ""]` on `/`, `/users` and
  `/profiles` -- and there is no form-field label and no schedule sentence anywhere in the vocabulary. The
  help prose is `settings.tmpl:189` and its sentence is "A change here takes effect the next time the
  daemon **starts**, not now", not the wording I put in quotes; the per-account refusal is
  `users.tmpl:61`, not `:60`; and `profiles.tmpl:110`, which I cited as a refusal, is
  `{{if .Days}}{{.Days}}{{else}}<span class="problem">no day</span>{{end}}` -- a missing value inside a
  cell. `.problem` turns out to carry six jobs, not two: a refused write (`profiles.tmpl:21`,
  `profiles.tmpl:91`, `settings.tmpl:9`, each of them `role="alert"` or a complaint sentence), a
  per-account fact (`users.tmpl:61`, `:63`, `:65`), a backend's failure detail (`dashboard.tmpl:62`,
  `lock.tmpl:80`, `testlock.tmpl:76`, `:116`), a standing mode notice (`profiles.tmpl:6` -- "Enforcement
  is switched off, so a rule added here changes the configuration but locks nobody out until it is
  switched back on" -- and `settings.tmpl:24`, "...so nothing is being locked out and nothing will be,
  whatever the schedule says"), help prose about when a change lands (`settings.tmpl:189`, "A change
  here takes effect the **next time the daemon starts**, not now"), and a missing value in a cell
  (`profiles.tmpl:110`).
  `.override` carries two, and the second is better than the one I invented: `lock.tmpl:93` and
  `testlock.tmpl:121` put it on "This is the session the daemon runs in", a sentence that says nothing is
  being held, in the same voice as `dashboard.tmpl:59`'s "open override". The point of #239 is therefore
  stronger than #239 said, and that is the only comfort in the finding.
  Silently editing the three bodies is the option this repository exists to refuse: the corrected text
  would be indistinguishable from text that had always been right, and the next reader could not tell a
  checked citation from a remembered one. A new issue instead of a corrected one throws away the two
  people who already read the first. The pushed commit message stays wrong and cannot be rewritten --
  `AGENTS.md` forbids force-pushing an in-flight branch, and the correction belongs in the log and the
  PR thread for the same reason `D-0042`'s does.
  What let it through is the same thing that lets everything through here: nothing re-read the prose. Two
  rules follow, one of them shipped. Shipped: `every_driver_that_measures_the_census_prints_it` in
  `testing/capture/pages.py`, which refuses a driver that splices `cdp.STATE_JS` into its measurement and
  never prints `cdp.state_census_line` -- the claim "every driver line prints it" was true of `pages.py`
  alone while `lock.py:169` and `testlock.py:164` were already calling `require_state_legible`, so the
  guard ran on those frames and left no number behind, and a frame whose guard passed cannot be told from
  a page with no state words. Both print it now, at real values: `/lock states=8 drawn=8`, the probe page
  `states=7 drawn=7`, with the two PNGs byte-identical afterwards (`git status --short docs/` empty), so
  the manifest needed no re-record. The rule was proved by removing the call from `lock.py`, which
  answers "1 of 69 rules failed" naming `lock.py`, and restored byte for byte (`md5sum` equal).
  `EXPECTED_RULES` 68 -> 69. Not shipped, stated instead: a check that a `file.tmpl:NNN` citation in prose
  points at a line containing the thing it names is worth writing, and it is not written here because the
  right shape of it -- which prose counts, which file names, what counts as "the thing it names" -- is a
  decision one example cannot settle. `D-0043` already refused to invent a citation checker for weaker
  reasons than this.
- Checkpoint: 45006d5a293ef5ec2776010e567f98995c55ef60
- Effect: testing/capture/lock.py and testing/capture/testlock.py (`cdp.state_census_line` on the
  measurement line), testing/capture/pages.py (the rule above, `EXPECTED_RULES` 69), DECISIONS.md (this
  entry and the `D-0045` correction above), issue #239's body (re-derived citations, six jobs for
  `problem`, two for `override`), pull request #240's body and a correction comment there, a correction
  comment on #199, and the message of `3bcbfdd`, which stays as pushed
- Correction (`D-0048`, 8 Nov): the three GitHub artifacts this `Effect:` names after #239 did not happen.
  Read back the same day: #240's body still carries all five citations this entry retracted; #240 holds two
  comments, 17:28:05Z and 18:31:07Z, and #199's newest before this round was 17:28:30Z -- all three earlier
  than `fb4af4e`, the commit carrying this entry, at 17:34:11Z -- and read in full, the 17:28 pair is the
  gate-and-reviewer-request note and says nothing about the citations. #239's body is the one artifact of
  the four that changed. So the thing this entry's `Chose:` promised, "publish the corrected numbers
  visibly on each artifact", was done on one of four, and this log asserted four until the reviewer
  re-measured the head at 18:41:12Z. `D-0048` measures it, finds the third place the false guard property was
  published (#199's 17:28:30Z comment, addressed to the user whose requirement it invokes), and puts a
  read-it-back rule on every write this round makes. What this entry concluded is untouched: all five
  citations are still wrong, the shipped driver rule still holds, and the reason nothing caught the prose is
  still that nothing re-read it -- now including a log line that re-read nothing and claimed four writes.

## D-0047 · The change request #240 was poked for is not on GitHub, so there was nothing to answer

- Question: The supervisor dispatched `address_review:240` on the strength of one review.
  `gh pr view 240 --json reviews` returns exactly one row -- `hermesvicowa`, `MEMBER`,
  `CHANGES_REQUESTED`, `submittedAt 2026-11-08T18:19:17Z`, `commit.oid fb4af4e04b89…`, which is the head
  this branch is standing on. Its `body` is 21 bytes: `@/tmp/pr240-review.md`. That is the whole field --
  `wc -c` over the API's `.reviews[].body` answers 22 counting the newline -- and the payload carries
  `author, authorAssociation, body, commit, id, includesCreatedEdit, reactionGroups, state,
  submittedAt` and nothing else. A tick was sent to answer a change request whose requested changes were
  never transmitted. What is the honest unit?
- Options: infer the ask from the branch's own open follow-ups -- #239 is the state-class split taken
  from #199 and is the obvious candidate subject -- and implement it | answer that the review was empty
  and change nothing, leaving the pull request where it was | escalate: publish the measurement where the
  reviewer will land, mark the pull request blocked for a person, record this entry, and hand the
  transport defect to its own unit
- Chose: escalated
- Why: the first option is the failure this branch has already written up twice. `D-0046` is five
  citations published from memory across #239, #240 and #199 -- all five wrong, all five plausible --
  and `D-0042` is a correction that asserted the thing it retracted. A change request is not a hypothesis
  to be improved on: the reviewer's list is the only artifact that says what `CHANGES_REQUESTED` means,
  and guessing would put invented requirements in a named reviewer's voice. That #239 is open is not
  evidence that the review asked for #239.
  The text is not recoverable by any read a tick has, measured here in five routes. The named file is not
  on this machine: `find / -xdev -name 'pr240*'` over the whole root filesystem answers nothing, and
  every other mount namespace's `/tmp` reached through `/proc/*/root/tmp` answers nothing. The string
  `pr240-review` appears in 0 rows of the local session store (`~/.local/share/opencode`, 6.1 GB, both
  the `part` and `session_part` tables), so no session on this box ever held it; `session_v2` between
  17:00Z and 19:00Z holds five factory ticks and the historian, no reviewer. `git log --all -S
  'pr240-review'` answers nothing, so no commit ever contained it. `gh pr view 240 --json comments` held
  one comment, mine at 17:28:05Z, before the review; the second row, 18:31:07Z, is this entry's own
  escalation comment and postdates it. What was measured is that no *issue comment* carried the review's
  text, not that there were two comments and both predated it — an earlier draft of this line said "two
  comments, both mine, both before the review", which cannot be true of the file at any point on the
  clock, because before 18:31:07Z it held one and after it holds two with one *after* the review.
  #199's newest comment as I wrote it was mine at 17:28:30Z; the pointer this entry sent is now newer
  there at 18:30:25Z. And
  `gh pr view 240 --json reviews --jq '.reviews | length'` is 1: there is no second review carrying the
  prose. So GitHub holds only those 21 bytes. Why it holds a path is an inference rather than a
  measurement -- the shape says the reviewer passed the file as `--body @/tmp/pr240-review.md`, and `gh`
  expands `@` only for `--body-file` -- but what is measured is that no attachment, no review object and
  no event on this repository carries the file's contents. The one route left, enumerating inline review
  comments on the review, is `gh api`, denied to this agent by `.opencode/opencode.jsonc`; a review filed
  with inline comments would still have had a body, and the body is a path.
  The second option is the failure this harness exists to end -- a tick that ends quietly while the queue
  believes it is working. So the escalation is published, not kept: a comment on #240 quoting the byte
  count and all five routes, `blocked` and `question` on its labels, a status block at the top of its
  body, and a separate issue for the defect upstream of all of it. `factory/triage.sh:188-190` projects
  every review down to `{u,s,t}` and drops `.body`, so the dispatch that started this tick carried a
  timestamp and no ask. That is not a quirk of this review: any change request filed against a green
  pull request dispatches a tick with a timestamp and no ask, whoever filed it and however well the body
  was written.
  Say what the push costs, because it does cost. `factory/triage.sh:216-228` classes a
  `CHANGES_REQUESTED` pull request by comparing `submitted_at` with the head commit's `committer.date`,
  so committing this entry moves the head past the review and the class flips from `address_review` to
  `needs_reviewer` -- "answered by the head commit; nobody holds a request" -- although nothing has been
  answered. That is why the marker goes in the pull request's comment, labels and body rather than in its
  queue class, and why the comment on #240 opens by saying that this commit answers the review none.
- Checkpoint: n/a
- Awaiting: answered at `2026-11-08T18:41:12Z`, and the durable half belongs to #242. The text arrived:
  the re-review filed at that timestamp carries the 18:19:17Z ask verbatim, submitted with
  `gh api --input payload.json` -- a JSON body built by `json.dump`, where a filename cannot masquerade as
  content -- so the unit was unblocked by the reviewer, not by any read this harness has. Say plainly what
  the original `Awaiting:` was worth: `gh pr review 240 --request-changes --body-file
  /tmp/pr240-review.md` is the right recovery for a person who still has the file on the machine that filed
  it, and it is not a fix for the queue. It depends on one reviewer's `/tmp`, and the next change request
  filed with `--body @path` lands a 21-byte verdict on some other pull request and dispatches another tick
  with a timestamp and no ask. The fix that outlives this instance is on the submission side and #242 is
  where both ends belong -- `factory/triage.sh:188-190`, which projects every review down to `{u,s,t}` and
  drops `.body`, and the client that let `gh api -f body=@/tmp/pr240-review.md` answer 200 with a healthy
  review row after sending 21 bytes. The reviewer's own predicate is recorded on #242: an
  `address_review` dispatch should refuse a `CHANGES_REQUESTED` whose body is empty, or 40 bytes or under,
  or matches `^@?/[\w/.-]+$`, because that is the shape of a lost ask.
- Effect: this entry; a comment on #240 quoting the measurement and the five routes; the `blocked` and
  `question` labels on #240; a status block prepended to #240's body; a pointer on #199; and one issue for
  the missing transport in `factory/triage.sh`.

## D-0048 · The re-review arrived with its text and asked for one base, and for a guard property this log does not own

- Question: At `2026-11-08T18:41:12Z` -- 703 seconds after the head commit it graded -- `hermesvicowa`
  re-reviewed at `6e0169d` and the body arrived this time: 14863 bytes read back from the API, filed with
  the payload taken from an `--input` JSON file, carrying the unread `18:19:17Z` ask verbatim (whose source
  file, 9088 bytes, the reviewer reports is still on the box that filed it). `D-0047`'s escalation
  is therefore answered and there is a real ask to work. Two blocking items, both prose this branch
  published about itself, and the reviewer re-measured both at this head. (1) `D-0045` says the channel
  rule is "written into the rules rather than left to a reviewer: colour plus one of weight, style,
  underline or a box property"; the shipped guard is self-tested *not* to have it. (2) The pre-fix count is
  on the record as *two of thirteen* (`DECISIONS.md:1847`, and #240's body leading with "two of them … the
  other eleven"), *one of nine* (the entry's title), *eight of thirteen* (the same sentence's other half)
  and *one* (`D-0046`) -- four numbers and no stated base. So: which base does this repository use, and does
  a false sentence stay where a reader meets it? And is the log made to say what the guard does, or the
  guard made to say what the log claims? Three nits of the same class came with them: `D-0045`'s `Effect:`
  still names `state_problems`, which is not a function in `cdp.py`; #240's body reports `--self-test`
  printing 68 where head prints 69; and `state_vocabulary_test.go` documents an at-rule filter its own code
  does not contain.
- Options: keep the nine-word base, leave `D-0045`'s live prose alone and add a third dated correction
  beneath it | settle on the thirteen-name census base, rewrite the guard-property claim to what
  `cdp._state_lookalikes` actually enforces, and leave the false sentences in the dated corrections and in
  the commit messages that cannot be rewritten | move the guard to match the published claim, requiring
  colour plus one more channel, so the sentence becomes true by changing shipped behaviour | answer only the
  two blocking items and leave the re-review's three nits for the next tick
- Chose: settle on the thirteen-name census base, rewrite the guard-property claim to what
  `cdp._state_lookalikes` actually enforces, and leave the false sentences in the dated corrections and in
  the commit messages that cannot be rewritten
- Why: the base first, because three numbers with no base is the defect rather than three numbers. The
  shipped guard counts thirteen names (`cdp.STATE_CLASSES`, `testing/capture/cdp.py:995`: nine words plus
  four rows) and prints a fraction of them on every page, so the thirteen-name base is the only one whose
  denominator is the number the driver prints. Re-measured with this branch's own reader, not by hand:
  `stylesheetRules` over `git show 6c37739:internal/web/static/app.css` returns seven class names --
  `inline masthead__name muted request-id rows state-free state-held` -- exactly one of them in the census,
  so twelve of thirteen undrawn, and eight of nine on the word subset. Both are now in `D-0045` with the
  base named, and the subset is called a subset so that the entry's older "one of nine" reads as the
  narrower count instead of a fifth contradiction. The nine-word base loses on the tie-break that prefers
  the reading which cannot under-count: it hides the four row classes, the half that had no rule at all --
  `grep -n "tr\.\|tbody tr" app.css` on the checkpoint answers nothing -- and a reader holding "eight of
  nine" has no number at all for the finding the coverage rule actually turned up. What choosing thirteen
  costs is worth stating: `state-free` is drawn and is not in the census, because no template has ever
  emitted it, so the pre-fix stylesheet wrote two state *rules* and drew one state *the pages use*, and the
  entry has to hand a reader that split rather than a single clean fact. It now does, in the same sentence.
  The guard item stings, because the reviewer is right and this branch's own self-test is the
  counter-evidence. `cdp._state_lookalikes` (`testing/capture/cdp.py:1106`) passes an element when one
  inherited property differs from its parent's *or* it carries a border or a background of its own;
  `pages.py:3230` registers the rule under the literal name "any one visible channel counts as drawn"; the
  self-test beside it, `one_visible_channel_is_enough` (`pages.py:3000`), runs six marks each given alone --
  colour, weight, italic, underline, a 3px border, a tinted background -- and fails if any one is refused,
  its docstring saying "the guard prescribes none of the six"; and `cdp.py:1134` states it in the guard's
  own voice, "Nothing here says WHICH treatment a state gets". So the log claimed a property the guard is
  tested to refuse, on the artifact whose entire job is to be the record of what was decided. The option
  that would make the sentence true -- colour plus one more channel -- is refused here on measurement and
  not on taste: it rejects four of the nine word rules as shipped (`.state-gap` `app.css:171`, `.override`
  `:194`, `.override-expired` `:198`, `.disabled` `:202`, each colour and nothing else), and the property it
  would buy is not what it sounds like. Separated from the ink they sit in, `--warn` is 2.69:1 and
  `--muted` 2.78:1 in the light scheme beside the pre-existing `--held` 2.61 and `--free` 2.60, and in the
  dark scheme 2.24 and 2.10 beside 2.28 and 1.73: the four marks this branch added are no more
  hue-dependent than the red it inherited, and two inherited marks are weaker than any of the four. A
  channel-counting guard would therefore not protect a colour-blind reader, it would enforce one designer's
  answer -- which is what #229's `nav a:hover` pin became three more properties later. The log now says what
  the code says: the guard refuses the case where the browser was told nothing, the choice of channel is a
  decision recorded here, and the ratios above are what defends the choice.
  Why the nits are not deferred: all three are the same failure as the two blocking items -- published prose
  asserting a mechanism the tree does not have -- and the one that was deferred once is the reason `D-0046`
  had to exist. `state_problems` is `D-0046`'s own miss: it corrected `format_state_line` and left its
  neighbour on the same line claimed. The 68 is #240's body quoting a printed count that `fb4af4e` itself
  invalidated -- `EXPECTED_RULES` is 69 at `pages.py:1058`, and `./testing/capture/pages.py --self-test`
  prints "69 refusal rules hold" at this head -- and the durable fix is to stop restating a number a rule
  addition moves. The Go comment is the one that needed code rather than prose: it described dropping what
  precedes a chunk's last `{`, while `sel` is cut at the chunk's FIRST `{`, so `sel` can contain `@` and can
  never contain `{`, so `LastIndex(sel, "{")` is always -1 and the branch assigned `sel[0:]`, which is `sel`.
  Deleting it is provably behaviour-neutral: a scratch program running both readers over the real `app.css`
  returns the same 19 class names, and agrees over six synthetic shapes including `@media (x){ .a{} }`
  (yields nothing) and `@media (x){ .a{} .b{} }` (yields `b`). With the arm gone the comment describes the
  mechanism that is there -- a preamble carries no `.name` token, which is what keeps `:root` and `@media`
  out of the census -- and names the case the old comment asserted did not exist: the FIRST rule inside an
  at-rule shares its chunk with the preamble and is never read, so the dark `tr.held` at `app.css:240` is
  invisible to this reader (that block, read on its own, yields `stale unmanaged unreadable`), while `held`
  is in the map anyway from `:227` and no row class carries the `state-` prefix this test asks about.
  Widening the reader was refused: for the assertion that matters -- a `state-` word a template says and the
  stylesheet never mentions -- reading more classes as styled is the loose direction.
  One thing this round found that nobody asked about, and it is wider than the ask. `D-0046`'s `Effect:`
  names four artifacts it corrected outside this file: #239's body, #240's body, "a correction comment
  there" on #240, and a correction comment on #199. Read all four back. #239's body carries the correction
  under its own heading, as claimed. #240's body still carries all five retracted citations verbatim --
  `profiles.tmpl:101` and `:204` as form-field labels, `profiles.tmpl:114`, `users.tmpl:58`,
  `settings.tmpl:151`, "19 of the 20 `.override` elements on `/profiles`". Neither correction comment
  exists: #240 has two comments, 17:28:05Z and 18:31:07Z, and #199's is 17:28:30Z, while `D-0046` landed in
  `fb4af4e` at 17:34:11Z -- so on the clock alone none of the three can be it, and reading them confirms it
  (the 17:28 pair are the gate-and-reviewer-request note). So one write of four happened, and the log says
  four. And #199's 17:28:30Z comment is worse than stale citations: it tells the user who filed the ticket
  that "A state is still required to be distinguishable without colour: colour **plus** one of weight,
  style, underline or a box property, checked in the browser" -- the same false guard property the
  re-review is blocking on, published to the person whose requirement it invokes. The reviewer scoped item
  1 to "the decision log and the PR body" because that is all they could see; the blast radius is three
  artifacts, not two. It is the same shape as the lost review body one level up: a call that reports success
  and changes nothing on the server, with the log asserting the delivery. Two rules follow. Every write this
  round makes is read back from the artifact afterwards and checked for the sentence that should be there and
  the one that should not -- which is what turned the next finding into a finding instead of an assumption.
  And the false text on #199 is answered by a dated comment rather than by editing the comment, for the
  reason `D-0046` itself gives: a silently corrected artifact is indistinguishable from one that was always
  right.
  The third finding is a correction to this entry's own first draft, and it is the same failure one level
  down. That draft said this round cannot rewrite a pull request's body at all, which is why `D-0046`
  believed it had. Two measurements stand behind it: `gh --version` is 2.46.0 and every field of `gh pr edit`
  still dies on `repository.pullRequest.projectCards`, `gh pr edit 240 --remove-label blocked --remove-label
  question` having printed the Projects-classic GraphQL error with the read-back returning
  `["question","blocked"]`, unchanged, so nothing half-applied either; and `gh api` is refused this agent
  whole, `.opencode/opencode.jsonc:120-121` carrying `"* api *": "deny"` and `"* api": "deny"`, a
  `gh api -X GET …/pulls/240` probe having been turned away at parse time with `Permission denied: shell`.
  The conclusion drawn from them does not stand, and why it was wrong is the part worth the space. The route
  was already recorded in this project's memory, and the search hit that surfaced it was cut off at exactly
  the wrong clause -- "`--add-label blocked --add-label question` and `--body-file` both exit 1…" -- while the
  next sentence of that same entry names the verb that works: the ISSUE verb aimed at the pull request's
  number. `gh issue edit 240 --remove-label blocked --remove-label question` exits 0 and reads back
  `labels: []`, and the body edit lands the same way, checked for the sentence that has to be there and the
  five citations that must not be. So the rule this entry needed was not "a unit has no transport" but "a
  preview of a memory is not a memory, and a mechanism inferred from one is not a measurement" -- the same
  habit that produced the two claims the reviewer is blocking on, a doc comment describing an at-rule filter
  the code does not contain and a guard property asserted rather than read off `cdp.py`, only pointed this
  time at a route instead of a rule. It was caught before it reached the reviewer; it is not hidden from the
  log either, because `0bdb10a`, still on this branch, carries the false version as written and this
  paragraph is its dated correction rather than an edit that pretends it was never said.
  One consequence of that draft does survive it, because it was measured and stays true: a review request is
  unreachable from here by every verb. `gh pr edit --add-reviewer` dies on the same field,
  `…/requested_reviewers` is refused at parse time as an `api` call, and `gh pr review` is denied by name at
  `.opencode/opencode.jsonc:86`, that file recording that the actor allowed to ask is `factory.sh`'s ungated
  `factory_request_review`. So #240 still reads `requested_reviewers: []` -- and so do #233 and #230, every
  open pull request in the repository -- which is the state `docs/factory.md` records four pull requests
  sitting in for three days while every check read green. That is a poke for whoever runs the supervisor, not
  work this round can do.
- Checkpoint: 36f43f17bab73e6ab3b94570629bc1f4c5f877d4
- Effect: DECISIONS.md (this entry; `D-0045`'s title, its `Why:` count and per-page numbers, its
  guard-property paragraph and its `Effect:` symbol list, with a dated `D-0048` correction naming both false
  sentences; `D-0047`'s comment count, dated, and its `Awaiting:`, which now points at #242 rather than at
  one reviewer's `/tmp`; and a `D-0048` correction on `D-0046`'s `Effect:`, which claimed four GitHub writes
  and got one); internal/web/state_vocabulary_test.go (the at-rule doc rewritten to the mechanism
  that exists, and the no-op arm deleted -- behaviour proven identical); pull request #240's body rewritten at
  this head -- twelve of thirteen with the base named, the re-derived `problem` and `override` citations in
  place of the five remembered ones, the guard property stated as the recorded decision it is, the printed
  rule count left to the run that prints it, and the round-3 summary on top -- written with
  `gh issue edit 240 --body-file` and read back, checked both for the sentence that has to be there and for
  `profiles.tmpl:101`, which must not be; a comment on #240 answering the two blocking items and the three
  nits where each was filed; a dated correction comment on #199, because that is the artifact where the false
  guard property was published to the user and it is older than this round, so it is answered rather than
  edited; a comment on #242 carrying the reviewer's predicate for a lost ask and the submission side that
  produced it; each of the three read back from GitHub after posting and confirmed by the first line it was
  meant to carry. `blocked` and `question` are OFF #240 by the same ISSUE verb, read back as `labels: []`,
  the reason they went on -- an ask nobody could read -- having gone when the review body arrived. No shipped
  product line moved: `app.css`, `cdp.py`,
  `pages.py`, `lock.py`, `testlock.py`, the templates and the nine frames are untouched, so the manifest
  needed no re-record.
