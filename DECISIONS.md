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
  entries below resume at `D-0021` rather than adding a third `D-0013`. That rule has paid for itself
  once already: `main` minted `D-0016`..`D-0018` independently while this branch was writing, so a
  branch that had numbered from its own highest id would have opened a third set of those three.
  This file took `main`'s at the 6 Nov merge and renumbered nothing.
- **The field contract is #217's, unchanged** -- `## D-NNNN · title` followed by `Question`,
  `Options`, `Chose`, `Why`, `Checkpoint` (plus `Effect`), with `Chose` repeating one option
  verbatim. So when #217 lands, `make verify` starts checking these entries instead of rejecting
  them for shape. What it finds first is the collision this section describes: `D-0011`..`D-0018`
  will name two decisions apiece once #217's own log lands beside them -- the last three arrived
  from `main` with #221, which answers #219 and #220 -- and renumbering them is that branch's call,
  not this file's.

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
  `06:30` and declare a per-hour state | delete `r-curfew` | re-take the POST frames here | leave them
  to #213 and re-take seven
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
- Options: source-newer-than-binary mtime | dirty-tree check | binary digest in the manifest | docs only
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

## D-0032 · Two of the five issues this log names are closed as satisfied; the other two are reduced to what they still ask for

- Question: #224. This log names #204, #207, #213, #219 and #220; four were open when it was filed, and a lane that reads an open issue as unimplemented builds it again from a base 42 commits behind. (#207 closed itself on 6 Nov at 13:29Z with its own row-by-row correspondence, so four were walked here.) Walk each against `origin/main` and close it, rewrite each down to its residue, rebase the stale branches and let the diff decide, or fold the set into #216 and close nothing?
- Options: leave all five open for the next lane to re-derive | close all five on the strength of this log's claim | walk each issue's own acceptance list row by row, closing only where every row holds and reducing the rest | rebase `origin/issue-199` onto `e74b42f` and reduce it to whatever survives | file the reconciliation as one issue and touch nothing
- Chose: the row-by-row walk — #219 and #220 closed as satisfied, #213 and #204 reduced to the rows that did not hold, with every settled row and the command that re-checks it written into the issue rather than into this log
- Why: the log's claim is not the acceptance list, and #204 proves it — the only entry here that names #204 is D-0016, whose markup comment at `internal/web/templates/page.tmpl:45` reads *"There is no skip link in this shell yet -- #204"*, i.e. it names the issue to say the work is absent. A grep for `#204` in this file, which is what #224 says a lane does, closes an issue that `main` does not implement: `git grep -n aria-current origin/main -- internal testing` returns nothing and `app.css` still has no `nav` selector, so the navigation is on the user-agent sheet exactly as reported. Closing all five therefore had to be refused on evidence, not on principle.
  The two that closed were measured rather than read. #219: `internal/web/page_heading_test.go:29`/`:78` assert one `<h1>` per page, its agreement with the page's own `<title>`, and that it does not name the product, over all seven pages including the two a GET cannot reach and the login page that is not the shell (`go test ./internal/web -run Heading` → ok, 6 Nov 13:44Z). #220: `internal/schedule/fixture_hours_test.go` gives 1440 minutes of the fixture's declared day and 1464 instants across 2026 one verdict, and its second test rewrites `06:30` back in and gets two — so the sweep that closes the issue is the same sweep that could have failed it (`go test ./internal/schedule -run PagesFixture` → PASS, same minute). #213 kept two rows because a re-take is not the same as a declared condition: the census is still a caveat in `README.md:280-281` and the two-arm measurement that exists is `scripts` off/on on one machine in one zone, not this issue's two zones.
  Two things were filed instead of done, both because the honest version needs a rig this fence does not have. `README.md:588` and the table at `:661-662` still call `b59bb2be…`/`acc5ba4d…` the committed digests when D-0028 moved those frames to `7ec1a664…`/`d008fdf9…` (`sha256sum`, 6 Nov) — rewriting them to values this turn did not photograph is the same overquoting D-0029 and D-0030 exist to remove, so it is #213's third row with the two arms named. And the rebase was not attempted: `origin/issue-199` is another unit's pushed branch, 42 commits behind with its own D-0025..D-0027 that collide with this file's, so what this lane could do was write the pointer into #204 — the implementation is in `2e919aa`/`8db322a`/`a2ccac0` on that ref, and the next lane rebases it instead of re-deriving it, which is the half of #224 this tree can actually fix.
  The id is D-0032, not the D-0031 this file's sequence gives. Pull request #217 is open and carries a *different* D-0031 in this same file (the #211/#222/#223 closes), and `check-decisions.py` on that branch refuses both a repeated id in one log — *"'revert to before D-0031' would name two commits"* — and a decreasing sequence. So whichever of the two branches merges second must renumber its own entry, and that is written here so the collision is a known instruction rather than a red gate on `main` after a merge. Choosing D-0031 anyway was the option that made `main` provably invalid for a window; a gap in the log if #217 never lands costs nothing, because the rule is monotonic, not dense.
  What lost. #220's other two options lose: pinning the daemon's clock through `web.Server.Now` and declaring the window in `pages-setup.sh` — both recorded as rejected in D-0017, and #220's body carried them as offers, not requirements. #213 loses the argument that byte identity proved the condition, which #207's own withdrawal already retired. #204 loses two measured rows — the duplicate `href="/"` and the `h1` weight comparison — both fixed under D-0016 as that entry predicted, and the issue now says so instead of sending a lane to re-measure them. Nothing here is irreversible: every close is one `git show origin/main:<path>` away from being re-checked, the four comments carry those commands row by row, and #224 is shut only as the walk it asked for — #216 still owns whether the supervisor should reconcile at all.
- Checkpoint: ab7263a2df0b8e662eb68f1a3107f143e17c65a6
- Effect: issues #219 and #220 (closed as satisfied, nothing deleted), #213 and #204 (reduced to their residue, with the settled rows and their re-check commands beside them), #224 (closed as delivered once the pull request carrying this entry is open), DECISIONS.md (this entry)
