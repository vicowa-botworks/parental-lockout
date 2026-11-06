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
  `"lockCommandPath": "/bin/true"` (`testing/capture/post-fixture.sh:43-45`), so the click is answered
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
