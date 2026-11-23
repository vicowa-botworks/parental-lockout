# testing/capture — the 360 px acceptance photographs

Six PNGs in `docs/screenshots/360/` are acceptance evidence for the criterion *"every page is
usable at 360 px with JavaScript off"*. They are not decoration and they are not hand-drawn:
each one is what a real browser reported at a real 360 px viewport, and the scripts here are what
took them.

**The measurement is no longer one viewport wide (#149).** `pages.py` walks `WIDTHS` — 360 px and
1024 px — and refuses a page at each of them separately. The committed set stays the 360 px one:
only the artifact viewport writes bytes, because two viewports staging a PNG under one basename
would mean whichever ran last decided what got published. See *Two viewports* below for why one
width was never a criterion.

This folder exists because of #108. The committed `lock.png` showed a run-on sentence —
*"...accepted the request but cannot confirm it This is the session the daemon runs inside."* —
on a page whose tests all passed. Every check that existed at capture time passed, and the one
check that would have refused the photograph did not exist yet. Whoever found it was looking at
the image. The lesson is not that the tests were weak; it is that **a screenshot with no driver
behind it is an assertion nobody can re-run**, and the driver was sitting in a scratch directory
outside the repository, invisible to a clone.

## Running a capture

```sh
go build -o plock ./cmd/plock          # or point PLOCK_BIN at a binary you already built
./pages-setup.sh    && ./pages.py
./pages-noprof-setup.sh && ./pages.py --fixture pages-noprof   # the accounts page, no profiles
./lock-setup.sh     && ./lock.py
./testlock-setup.sh && ./testlock.py
./pages.py --self-test   # the refusal contract only: no browser, no fixture, nothing written
```

Each `*-setup.sh` builds a throwaway fixture — its own config, state directory and audit trail
under `runs/`, on a port nothing else uses — and each driver starts the daemon on it, signs in
through the login form in the browser, measures, and writes its PNG to `runs/<name>/`. Run them
from any directory; every path is resolved from the script's own location.

Needs `google-chrome` and a `plock` binary. Needs **no root and no human** — the fixture points
the lock backend at the command backend with `/bin/true`, so a capture cannot lock the machine it
runs on. That is the whole reason a screenshot can be taken by a script at all: the alternative is
a test that locks the operator's screen.

**The two POST captures do need the account they capture to hold a real desktop session**, because
that is what the result page lists. The fixture takes the target from `$(id -un)`, so run them as
the desktop user; `PLOCK_CAPTURE_TARGET` overrides it for the case where you are logged in under
one name and the session belongs to another. Overriding does not buy you a way to capture without
a session — the driver refuses an account with no rows, and that refusal is correct, because the
zero-session sentence fits at any width and would report 360 px while measuring nothing.

**Before blaming the machine for one of those refusals, read `serve.log` in the fixture directory.**
A refusal that looks like "this account has no session" is also what a daemon that cannot *read*
the session table produces. Measured 27 Oct: both POST drivers refused on a box holding four
lockable `class=user` sessions on `seat0`, and `serve.log` had the reason —
`session list unavailable: command: loginctl list-sessions: unrecognised output: [{"session":"468",…`,
a field type systemd 259.9 had outrun (#167). The drivers were right to refuse and the
reproducibility table below was wrong to imply the hardware was the limit; the page they were
refusing to photograph said `the session table could not be read`, which is a daemon fault, not a
missing session.

## What each driver refuses

A screenshot is evidence only because the thing that took it refuses to take it in the wrong
state. The refusals are the artifact's whole value, so they are listed here and the ones more than
one page needs live in `cdp.py`.

| driver | refuses when |
|---|---|
| `pages.py` | a page answers a different URL than it was sent to; **a table page** has no table or no cells (an empty fixture fits at any width); **a form page** has no POST form or no fields (the sibling check — a `/settings` that rendered only its navigation also fits at 360 px, and dropping the check instead would have committed that as evidence it fits); any cell lacks `data-label`; **the page overflows its viewport** — measured at every width in `WIDTHS`, refused per page per viewport, and before the PNG is written; **the audit page** carries no seeded `platform.error` row, or its hardest unbreakable token is under 60 characters (#140: a page that could not have overflowed is not width evidence, and a page whose pressure came from the machine rather than the fixture is not evidence about the stylesheet); **a control disagrees with the page it sits on** — set in the browser's own typeface rather than the page's, sized by no `max-width` at all or by a different one from the other fields on that page, or a radio or checkbox whose label the stylesheet left inline so the box can wrap away from the words that name it (#202; every page, both viewports, before the PNG is written) |
| `lock.py` | the dashboard has no `lock-now` form, no submit button, no CSRF token with a value, or a method that is not `post`; the click does not navigate; the result page has no banner sentence or no session rows; a cell lacks `data-label`; a session row has no Result cell; **a row claims the lock happened while its own Result cell says otherwise** (#109); the gated clause is absent, meaning the self-session note never rendered; **the Detail cell's sentence runs into that note** (#108); the note appears in no cell at all, so the #108 rule would have examined nothing; **the click wrote no `lock.requested` line**; the page overflows; the three control claims (#202), which on this page are shown exactly one control — the chrome's Sign-out button — and report it as `controls=1 typed=0 groups=0` on the transcript rather than passing in silence |
| `testlock.py` | the same form and click refusals for `test-lock`; the page does not say *Nothing was locked*; fewer than two tables or no backend rows; the #108 seam rule and the same empty-rule guard; **the audit trail holds anything but `lock.probed`**; the page overflows; the three control claims (#202), vacuous here for the same reason — the transport radios those rules are about are on `/settings`, a `pages.py` frame |
| all three | the daemon the fixture started is not the process still serving its port; something already answers the DevTools endpoint before the browser is launched; `--out` resolves outside the repository, names a file, or names a directory holding PNGs without `--overwrite`; an unrecognised flag; **a word the page uses to report a state is drawn in exactly the voice of the
text it sits in** -- same colour, weight, style and decoration as its parent, no border and no
background of its own (`cdp.require_state_legible`, #199: nine class names carry a state and `app.css`
wrote a rule for one of them, so `/users` said *"in the configuration, not on the machine"*, *"not
managed"*, *"the account no longer exists"* and its own banner as ordinary prose at both viewports, and
`/` drew one of its four). The vocabulary is `cdp.STATE_CLASSES`, read off the templates by a rule in
`pages.py --self-test` rather than remembered; it is eleven word names and four row names today, because
#239 split the two that each meant two things and the two new names (`refusal`, `exempt`) are in it -- a
name the
census gains is a name Chrome is asked about on every photographed page, which is the half a rename
cannot quietly opt out of; what the browser computed is on the measurement line as
`states=N drawn=M`, zeros included, so `states=6 drawn=0` is readable after the fact |

Four of those deserve explaining, because each was a hole found by attacking this code rather
than by writing it.

**The trail is an allowlist, and it counts only this run's lines.** The probe page's second claim
is that nothing which asserts a lock was written. Refusing kinds that start with `lock.` does not
catch that, because `lockout.entered` starts with `locko` — the denylist missed the one kind whose
name is the whole worry. So the rule is inverted: `lock.probed` is the only line the probe click
may write, and any other line stops the capture until somebody admits it on purpose. And the check
looks only at the lines added since the driver started, because a fixture can be photographed
twice and a membership test against the whole file passes on the previous run's line.

**The overflow verdict is computed before the write.** A photograph of an overflowing page is the
artifact the criterion refuses; writing it and then exiting 1 leaves wrong evidence on disk under
the right filename.

**The control census reads what the browser computed, because #202 was invisible to everything
else.** Every control on every page was rendering in Arial 13.33 px beside the page's own 16 px
system-ui, and `/settings` was sizing its fields three ways at once -- 65 px, 163 px, 185 px. No Go
test can see that: the mutants target `.go` and `.tmpl`, and no `_test.go` renders a page. So
`CONTROLS_JS` rides on the one measurement every driver already takes and three refusals read the
computed style -- each visible control's font against `document.body`'s, the `max-width` of
everything you can type into, and the `display` of every label wrapping a toggle.

The last of those is the reason a grep would not have done. The stylesheet reaches a choice group
with `:has()`, and a selector an engine has never heard of is discarded without a word: the page
renders exactly as it did before the rule existed and every string in the file is still there to be
found. Computed `display` is the only thing in the browser that says whether the rule applied. The
`max-width` read has the same shape in the other direction: a field reporting `none` is sitting at
the user-agent's own twenty characters, so a page where every field agrees on nothing is refused as
well as a page where they disagree -- deleting the width rule entirely makes all the values match.

The read passes vacuously where there is nothing to read, and says so: the verdict page and the
probe page each carry one control, the chrome's Sign-out button, so their lines print
`controls=1 typed=0 groups=0`. That census on the transcript is the point -- a guard that examined
nothing must not report success silently, which is the same rule the #108 check follows when it
prints how many Detail cells carried the note.

**The daemon and the browser must be the ones the fixture started.** A port answering is not
evidence about who is answering it. With a second `plock serve` already holding the fixture's
port, the fixture's own daemon dies on `bind: address already in use`, the stranger answers the
readiness probe, and the capture signs in, clicks, measures and writes a PNG about a different
config, a different audit trail and a different set of accounts. The exit code is the only thing
that distinguishes the two cases, so `require_running` checks the process before the browser
touches anything and again immediately before the PNG is written.

The browser is harder, and probing the port before the launch was not enough. Chrome that loses
the port does not exit: it logs `bind() failed: Address already in use`, falls back to `::1` on
the *same port number*, and stays alive — so the pid check sees a healthy process and a pre-launch
probe is a race that a stranger arriving half a second late wins. Measured that way: the fixture's
own `chrome.log` recorded the failed bind, `require_running` returned normally, and the session
attached to the stranger's browser (`/json/version` returned its id, not the fixture's).

So the browser is *owned*, not probed. `--headless=new` does not write `DevToolsActivePort` into
`--user-data-dir` — a successful run leaves no such file anywhere under the profile — but Chrome
prints the endpoint it opened to stdout, and stdout is this fixture's `chrome.log`:

```
DevTools listening on ws://127.0.0.1:9333/devtools/browser/50f7bf28-…
```

That line names both the address Chrome bound and the id of the browser behind it, in a file a
stranger cannot have written. The driver requires the address to be `127.0.0.1:<CDP_PORT>` (which
is what catches the `::1` fallback), requires `/json/version` to advertise the same browser id,
and asks the endpoint again before every write.

## What a refusal leaves behind (#220)

A refused run publishes nothing, and that is the contract: a half-replaced artifact set is worse
than none, because every filename in the directory then looks current (#222). What it used to also
do is destroy the frames it had measured on the way out. Measured 6 Nov on `main`, with the one
condition the capture notes already document as refusing this fixture
(`PLOCK_CAPTURE_SCRIPTS=on ./pages-setup.sh && PLOCK_CAPTURE_SCRIPTS=on ./pages.py`):

```
REFUSED: /: the 'casey' row says ['locked out until 18h 16m', …]
  no file was written
  /users     @360   … FITS   …/runs/pages/users.png
  … four more like it
5/6 pages fit at 360 px (fixture pages); refused: /
Nothing was written to …/testing/capture/runs/pages.
```

Five sentences naming five files, and the directory held none of them — so the operator had the
refusal's prose and no way to look at the page that caused it. Every guard runs before the write,
which is right for a publish and was silently right for a look.

So a refused run now keeps what it measured, in `testing/capture/runs/<fixture>-refused/`, with the
refusals written beside them in `REFUSED.txt`. The frame of the page that refused is taken even
though it will never be published: the guard says what the page was expected to say, and only the
picture says what it did. Four properties are enforced in `cdp.py` rather than trusted:

* **Moved, not copied beside the queue.** `_PENDING` is emptied, so no later `publish()` can reach
  a refused run's bytes even if a future caller reaches it with a broken refusal count.
* **Cleared once, on opening.** A frame from yesterday's refusal beside today's is the same lie in
  a new costume, so whatever is in the directory when this run first opens it is deleted — and what
  this run writes after that is left alone. The clearing is announced, because a reader who expected
  to find something there is owed a record of when it went.
* **Never into `--out`.** The directory is named from `RUNS` and the fixture, never from `--out`,
  and `keep_refused` declines a target outside `runs/` or one that overlaps the run's own `--out` —
  in which case it destroys the frames and says so. A frame that could be read as the evidence this
  run refused to write is worth less than the bytes.
* **Named for the fixture.** `pages` and `pages-noprof` get separate drawers, for the reason
  `capture_out` gives about scratch directories: two fixtures in one directory produce frames whose
  names look like whoever wrote last.

None of it is evidence. Nothing records these frames, no manifest names them, and the summary says
so in the same breath as the count — because a PNG with no sentence under it is exactly what #211
says a frame is not, and these frames state less than most: the reason they exist is that the state
was *not* the declared one. `runs/` is gitignored, so they cannot reach a commit by accident.

The two POST drivers keep the same way now (#296). They did not: every guard ran, the one frame this
run measured was staged, the refusal printed, and the bytes were deleted on the way out -- while the
only file the refusal named was **the previous run's** `lock.png` sitting at `--out`, with the mtime
that says it is stale. So the two drivers whose artifact is a single page, and the two that post a
real state change, were the two where a refusal left a sentence pointing at a file this run did not
write and no picture of the page that refused. Measured 11 Nov on this branch by declaring one
session in `fixture.env` on a machine whose census is two -- `lock.py`, exit 1, `--out` still
`df499589…` afterwards:

```
REFUSED: the lock result page: the page renders 2 lockable session row(s) and the fixture declared 1. …
  a previous lock.png is still at …/runs/lock/lock.png -- mtime 2026-11-11T16:23:35Z, 132550 bytes -- and it is NOT from this run. Do not quote it as current.
  lock-refused/ cleared 1 file(s) from an earlier refusal; everything in it now came from this run
  what the page actually renders is at …/runs/lock-refused/lock@360.png -- this run refused to publish it, so it is not evidence; read it with REFUSED.txt beside it
  1 frame(s) this run kept are in …/runs/lock-refused, with REFUSED.txt beside them; nothing was written to …/runs/lock, and none of these frames is evidence
```

Four things about that are deliberate:

* **The frame is taken while the browser is still open.** The keeping happens inside
  `with cdp.Capture(...)`, because a torn-down CDP session answers nothing, and a keep after
  teardown is the same defect in new clothes: no frame, plus a note describing a page nobody looked
  at. `--self-test`'s fake browser now closes its session on `__exit__` the way the real one does, so
  moving the keeping back out is caught rather than admired in a comment.
* **`lock@360.png`, not `lock.png`.** The name with no viewport on it is the artifact an operator
  quotes, and the file standing under that name may belong to a previous run (#222).
* **`--out` is never touched**, and the stale file standing there is named, with its mtime, instead
  of deleted. Aim `--out` straight at `runs/lock-refused` and the keep *declines* -- the same guard
  that stops `pages.py` writing into a publish directory -- destroys its own frame, says that it did,
  and leaves the file it found there byte for byte.
* **One drawer per fixture.** `lock-refused` and `testlock-refused` are separate for the reason
  `pages` and `pages-noprof` are: two fixtures in one directory produce frames whose names look like
  whoever wrote last.

`--self-test` drives both drivers' `main()` with their capture stubbed to refuse, on both arms -- a
guard refusing while the browser is up, and a fixture refusing before one exists -- and reads the
result off the disk rather than from a stub: the drawer holds exactly `lock@360.png`, `REFUSED.txt`
carries the refusal and names the fixture, the announced count agrees with the directory, the
disposal ran exactly once (#149's dead-statement class, which is how these two turned out to have
been discarding *nothing* at all), and nothing was destroyed.

**One refusal, one disposal, even when the refusal's own report raises** (#304). Both drivers catch
the refusal twice — inside the `with`, and again outside it for a fixture that refused before a
browser existed — so anything that raises *inside* the first disposal is caught by the outer handler
and read as a second refusal. Measured 11 Nov on `origin/main` at `1e31cac`, with the operator's
stderr refusing one write (the first sentence printed after `REFUSED.txt` exists on disk): the note was
written twice — two `refusals (1):` blocks, the second one ending `refused: OSError: [Errno 9] Bad file
descriptor`, the diagnostic's words last — while the drawer held the one frame it had always held.
`cdp.SingleRefusal` decides that now, and it sets its flag *before* the call, so a disposal that dies
mid-report is still the run's only disposal; the outer arm reports what raised and writes nothing.

What it reports is chosen from the disk, not from the flag, because the flag can be true when nothing
was written. Kill `report_refusal`'s headline instead of the sentence after the note and the disposal
dies having produced no note and no frame: `rc 1`, no `REFUSED.txt`, `--out` untouched — and the first
version of this fix printed "that file holds the one note this run wrote" straight over that.

Three failures inside one disposal are attacked, in one rule — `pages.py --self-test`'s "a refusal whose
own report raises is disposed of once", one of its 78 — because each leaves something different on disk
to be honest about. `note` (stderr dying on the first sentence taken after `REFUSED.txt` exists) is
#304's double write, and wants one header, one block, and a note ending on the page's own words.
`headline` (stderr dying on `REFUSED:` itself) wants the transcript to say nothing was written and never
to claim a note. `shot` (the browser refusing the photograph once) wants the empty drawer blamed on the
browser — and that one is #304's own named repro, which measuring falsified: `capture_refused` catches,
prints `no frame of the refused page: …` and returns None, so a failing photograph cannot re-refuse
anything (`keeps 1`, one block). The double write needs a raise from a step nothing wraps, and the
summary `print` after the note is one. The cell stayed anyway, because the empty drawer it produces was
being explained by a sentence that blamed the fixture whatever the cause was; `cap is None` now chooses
the reason, and the fixture arm's wording is unchanged byte for byte.

`python3 scripts/attack-capture-self-test.py` — 48 attacks, 31 caught, 6 honest edits still green, 11
holes — patches each of the five places that could undo any of that (the flag, the branch that reads the
disk, the branch that reads `cap`, and the outer arm of either driver) into a copy of the tree and
requires a red verdict. Each of the five makes exactly one rule fail, `capture self-test: 1 of 78 rules
failed`, and across the five runs not one of the 44 problem lines names any other rule.

One boundary is left open, deliberately: the *fixture* arm has a single handler, so a report that dies
there is not mistaken for a second refusal — it simply escapes, and `main()` never returns. Measured on
the same matrix: `rc=RAISED OSError: [Errno 9] Bad file descriptor`, one block, `--out` intact. Stopping
that means deciding what every diagnostic in this package does when stderr has gone, which is a wider
question than one disposal, so it is written up in `docs/issues-from-d-0065.md` with the four-cell
matrix above and the one command that files it, rather than folded in here.

## The negative control, and why a refusal nobody watched fire proves nothing

An assertion never run against the alternative is unproven; a refusal never watched firing is the
same failure wearing a different hat. The control is the same for every rule above: build a binary
that removes the thing being pinned, point the capture at it, and require a refusal.

```sh
PLOCK_BIN=/tmp/plock-broken ./lock-setup.sh && PLOCK_BIN=/tmp/plock-broken ./lock.py
```

What "removes the thing" means depends on which tree you are on, and the difference is the whole
point of the exercise:

* **The #108 separator.** It used to be the literal space in `internal/web/templates/lock.tmpl`
  (`{{if .Detail}} {{end}}`), which meant that on `main` — before #113 — the control was free:
  build nothing, run `lock.py`, and it refused, because the page really did render the run-on.
  That is no longer reachable from `main`, because #113 merged and the terminator now lives in
  `internal/web/celltext.go` (`sentenceTerminated`). The control is therefore a mutation:
  `return trimmed + "."` → `return trimmed`, which is `W201`. Measured on this branch against a
  binary built with that clause removed:

  ```
  REFUSED: the lock result page: the detail runs into the self-session note
    ('he request but cannot confirm it' then 'This is the session the daemon runs');
    the page still renders a run-on, so this capture is the pre-fix page
    no file was written
  ```

  Exit 1, and nothing at `--out` — not an empty directory's worth of partial frames either,
  because captures are staged and only published once every page has passed.
* **The #109 rule**, controlled the same way on any tree: remove the gate on the self-session
  clause and the driver reports `a 'unconfirmed' row still claims the lock happened`.
* **The #213 census**, controlled without touching a line of Go: declare a census the machine does
  not have. `PLOCK_CAPTURE_SESSIONS=1 ./lock-setup.sh && ./testing/capture/lock.py --out …` on this
  box, measured 10 Nov:

  ```
  REFUSED: the lock result page: the page renders 2 lockable session row(s) and the fixture declared 1.
    The census decides this artifact's height, so a frame taken over a different census is a different
    page — re-run the setup script on a machine whose census is the one you mean to publish, or declare
    PLOCK_CAPTURE_SESSIONS=2 if this one is right and the fixture is out of date (#213).
    no file was written
  ```

  Exit 1, nothing staged, the committed `lock.png` still `df499589…`. The probe page is the second arm
  and it is the one that tests *which* rows are counted: `PLOCK_CAPTURE_SESSIONS=3` declares the session
  rows **plus** the backend row, and it reports `the page renders 2 lockable session row(s) and the
  fixture declared 3`, because `cdp.session_census` counts the rows with a `Where` cell and not the
  `tbody tr` of a page that has grown a second table. The third arm needs no declaration at all — point
  the census at a bus that is not there and the setup script refuses before the browser starts:

  ```
  $ DBUS_SYSTEM_BUS_ADDRESS=unix:path=/nonexistent/bus ./testing/capture/lock-setup.sh
  REFUSED: the census could not be read: live sessions: command: loginctl exited with 1:
    Failed to connect to system scope bus via local transport: No such file or directory
  ```

  That is the difference between a census of zero and a census that could not be had: `plock status
  --json` reports the second as an empty `sessions` list *and* a `warnings` entry, and the fixture
  reads the warning rather than guessing from the empty list.

Note the second half of that refusal message. "Exit 1, nothing written" is true and still
misleading: a PNG from an earlier run is sitting right there, and whoever reaches for it next has
no way to tell it predates the run that refused. Deleting it is not the answer either — with
`--out` pointing at the docs tree, the file being deleted would be the committed evidence. So the
refusal names the stale file and the mtime that says when it really came from.

Two more refusals exist because the first version of this package could destroy evidence while
doing its job:

* **Anything written outside `runs/` needs `--overwrite` as well as `--out`.** The guard used to
  fire only when the target already held PNGs, so `--out docs/screenshots/360/..` and `--out .`
  both passed it and the run wrote four new PNGs into tracked directories with no flag and no
  warning. It keys on where the target *is* now: anywhere that is not this package's own ignored
  `runs/` takes the flag.
* **A half-finished run publishes nothing.** `pages.py` catches a refusal per page and carries on,
  which used to mean it printed `4/4 pages fit at 360 px` and exited 0 after refusing one of them
  — and with `--out` on the docs tree it left three freshly captured pages beside one stale one,
  under committed filenames, which is the half-replaced evidence set this folder's own house rule
  forbids. Frames are staged and copied at the end, and the count printed is the count that
  happened.

**The two #207 refusals, and the control that fired on its own.** `require_declared_state` refuses a
page whose cells do not say what the artifact's declaration says, and `require_declared_rows` refuses
an audit page whose row count is not the fixture's. Both are covered in `pages.py --self-test`, and
both were watched failing the way they are meant to: with the two bodies replaced by `return`, the
self-test reports six problems — `a dashboard saying \`until 05:00\` for a rule that says midnight
passed the declared state`, `a dashboard with no casey row passed…`, `an empty row census passed…`,
`a page rendering 7 rows passed against 8 declared`, `…9 rows…`, `a page with no row census at all
passed…` — and 38/38 hold with them restored (the file was restored from a `/tmp` copy and compared
byte-identical, not with `git checkout`).

The second control was not staged. The first two-run comparison was run with `TZ` set on the host and
`PLOCK_CAPTURE_TZ` declared by the fixture — and the fixture wrote its `fixture.env` without the
`TZ` key, because `capture_env` had not been told to emit it. Run A rendered `2026-03-18 21:34:05
EDT`; run B, on the same branch, same fixture, same binary, host zone `Asia/Kolkata`, rendered
`2026-03-18 21:34:05 -0400` and **the declared state refused it**, with the row it saw quoted back.
The five PNGs that had already been written were identical to run A's, so a check built on the
md5s alone would have reported "six artifacts reproduce" over a seventh that had silently come from
another zone's label — and it was `report_refusal`'s `a previous audit.png is still at … it is NOT
from this run` line, not the file's presence, that kept the stale frame from being read as run B's
output. That is this folder's whole argument in one measurement: the refusal caught a defect in the
code that added the refusal.


Measured by running each driver and comparing md5 against the committed file. The reproducible
ones were confirmed stable across two consecutive runs of the same fixture.

**#207 changed the answer for every page-fixture artifact, so read this section as of that change.**
The criterion was never pinned to a *state*: a frame is a function of the fixture, the clock, the
zone, the session census and the page's own scripts, and six of those were undeclared. They are
declared now — the fixture's instant (`PLOCK_CAPTURE_STAMP`/`_ZONE`, with the sign-in's row
re-stamped to it by `cdp.pin_appended_rows`), its row count (`PLOCK_CAPTURE_AUDIT_ROWS`), its census
(`testing/capture/bin/loginctl` on the daemon's `PATH` for the page fixtures; and since #213 the number
itself, `PLOCK_CAPTURE_SESSIONS`, declared by the two POST fixtures — see "The negative control" for the
refusal it produces), its reader zone (`PLOCK_CAPTURE_TZ`), the
state each page must render (`Page.state`), and JavaScript off from the login onward
(`Emulation.setScriptExecutionDisabled`). The seventh condition, the zone label the audit page put on
each row, was a defect rather than a condition: `audit.tmpl` asked for `MST`, and Go gives a parsed
instant the reader's zone when the stored offset matches that zone's abbreviation. **#210 fixed it in
the template** — the When column now prints the offset the trail stores — which is the right home for
it: a fixture can pin a condition, it cannot make a wrong label correct.

The claim, measured on this branch rather than argued: **two fresh fixtures, run about a minute
apart, on the same machine under two different host timezones** (default, then `TZ=Asia/Kolkata` for
both the setup script and the driver) produced byte-identical PNGs for all seven page-fixture
artifacts — `dashboard` `3ba5275e…`, `users` `76ea48fb…`, `profiles` `8a9bf9f5…`,
`profiles-refused` `42764b11…`, `audit` `6545ce7d…`, `settings` `175c1d1f…`,
`users-no-profiles` `ce38334c…`. One host, two zones declared, no page differing.

Against the previously committed bytes, `settings.png` and `users-no-profiles.png` came back
**identical**, and the three whose *fixture content* this branch changed (`users`, `profiles`,
`profiles-refused`, moved by alex's new "Rules switched off" profile and by the rule rename to
"Every day, every hour") plus `dashboard` and `audit` moved with it — five, which is what
`git show d439f70 --stat -- docs/screenshots/360` lists. The two unchanged files are the
evidence that turning page scripts off did not move a page that has no `time[data-unix]` in it; it is
not a per-page control, and `/users` could not have been one here because its fixture content changed
on this branch for an unrelated reason. `dashboard.png` also grew, 1310 px → 1366 px: the Sessions
cell now holds the fixture's census row instead of a sentence, which is the point of declaring it.


Every hash in this table moved when the settings page added a link to the masthead, and that is
worth stating rather than slipping past: the shell is one template, so a change to the navigation
changes the pixels of *every* page, including the four this branch does not otherwise touch. The
committed photographs therefore had to be re-taken as a set. What proves the re-take is faithful
and not merely new is the control run: with the nav link removed again, `users.png` and
`profiles.png` came back byte-identical to the previous committed files (`003ee3f8…` and
`c31bdd55…`), so the only thing that moved them was the link.

The same control was run for the transport form (#161), which is a change inside one page rather than
in the shell: on a single fixture run `users.png` came back `b69eb0d1…` and `profiles.png`
`130edd13…`, both byte-identical to the committed files, so the only page the new fieldset moved is
`/settings`. That PR deliberately did **not** re-commit `dashboard.png` or `audit.png`: they churn for
the reasons in their own rows below, and replacing them would have been churn dressed up as evidence.
What it reports for those two is the driver's line, which is the claim a reviewer can re-run.

This table used to quote a hash per row, and every hash in it eventually named a file that no
longer exists: three of the five `yes` rows were quoting bytes no committed PNG carried when this
was last read, which is #144's complaint in miniature. The bytes each artifact is pinned to now
live in `docs/screenshots/360/manifest.json`, written by the driver at the end of the run that took
it and checked by `make verify` -- so the file↔run relationship is machine-checked and this table
carries only what a machine cannot know: whether a fresh run reproduces the committed file at all.
Measured again on 28 Oct against the manifest's own hashes, on the run that took the refused-write
page: `profiles.png`, `profiles-refused.png`, `settings.png` and `users.png` came back
byte-identical; `dashboard.png` and `audit.png` did not, which is why that PR commits the two that
reproduce and leaves the two that churn -- the same choice #161 made for the same reason.

#199 re-took the set on 8 Nov, on the run that gave the state vocabulary its rules in `app.css`, and
what that run did to this table's premise belongs here rather than in a row: **eight of the nine
frames moved** -- `settings.png` and `users-no-profiles.png` among them, the two `d439f70` reported
identical -- because a colour change is not a fixture change, and all eight render at least one state
word at 360 px (`/users` renders six). `audit.png` came back byte-identical, and the driver's line now
prints why in the same breath: `states=0`, no state word anywhere on that page at either viewport, so
there was nothing for a #199 rule to reach. So every "byte-identical to the committed file" sentence
above this line is a citation between two runs at two dates, and not a pointer into
`docs/screenshots/360/`; `manifest.json` is the pointer, which is what the paragraph above this one was
written to say.

| artifact | does the in-repo driver reproduce the committed file? | why |
|---|---|---|
| `users.png` | **yes** | |
| `profiles.png` | **yes** | re-taken by #147, which is the change that moved it. The `740200cc…` this row quoted had already stopped matching the committed file before that PR landed, and nothing noticed -- which is #144. Moved again by #138, whose refusal page is the artifact below |
| `profiles-refused.png` | **yes** | new with #138: the same URL photographed AFTER a refused write, so one page carries two artifacts and the manifest has to answer "which template does `profiles-refused` belong to" -- a *state* of a page is not a file named after it, so `template_for` asks the driver which `Page` declared the artifact and falls back to the stem only for the ones that declare nothing (`lock`, `testlock`). Reproduced byte-identical on the run that recorded the manifest |
| `settings.png` | **yes** | new with the settings page. It is the only page in the set refused on a *form* rather than on rows (`cdp.require_form`), because a page with no table would otherwise have been refused as an empty fixture -- and the lazy repair, dropping the check, would have let a /settings that rendered nothing but its navigation be committed as evidence that it fits. Re-taken by the transport form (#161), which took the page from `forms=3 fields=9` at 2069 px to `forms=4 fields=15` at 3511 px; the previous hash `42d47d9b…` is what the file was before that form, and the row is the only place that claim lives |
| `testlock.png` | **yes, on an account holding two lockable sessions** | the census is `PLOCK_CAPTURE_SESSIONS=2` on this machine as of 10 Nov — `tty` and `x11`, both `isLockable` — and `cdp.require_declared_sessions` refuses any other count, so this cell is a condition and not a recollection. Measured 10 Nov, fresh fixture, default zone: `session census: 2 lockable session(s), as uid 1000 through /usr/bin/loginctl`, `tables=2 backendRows=1 cells=8 unlabelled=0 states=7 drawn=7`, `scrollWidth=360 height=1311`, and the PNG is byte-identical to the committed `d285d1cd…`. The probe page's session Detail cell is the self-session note alone when the account's sessions are lockable, so there is no seam and the #108 rule has nothing to terminate (`note in 2 Detail cell(s), 0 with a sentence in front of it`); on a machine whose sessions are not lockable the cell carries a reason and the rule applies — which on `main` means a refusal, not a different image. The other census arm: an account holding four lockable sessions ran green on 27 Oct (`cells=14`, `a8617a2b…`) and produced a faithful capture of a *different page* — which is exactly what the refusal now stops from being committed silently. **This row said "one session" and quoted `cells=5`, height 1013 px until #213**: the frame had moved to two sessions at `3bcbfdd` (8 Nov) and nothing anywhere noticed |
| `lock.png` | **yes, on an account holding two lockable sessions** | measured 10 Nov on a fresh fixture: `session census: 2 lockable session(s), as uid 1000 through /usr/bin/loginctl`, `rows=2 cells=6 unlabelled=0`, `scrollWidth=360 height=1536`, banner `rodney was not locked.`, flag `2 of 2 sessions were asked to lock and nothing could prove the screen went down.`, `#108 seam: note in 2 Detail cell(s), 2 with a sentence in front of it` — and the PNG is byte-identical to the committed `df499589…`. Only reproducible since #113 merged: on the branch that added these drivers the committed file was the pre-fix image and the driver *refused* to produce it, which is the same fact pointing the other way, and the reason this row used to read "no". **The census is why this row was wrong for two days.** It quoted `rows=1 cells=3`, height 1142 px and "one session" — true of the frame at `38578aa` (19 Oct, `b59bb2be…`, 1121 px) and at `9e07dde` (5 Nov, `7ec1a664…`, 1142 px), overtaken by `b9eeec3` (6 Nov, `712ee631…`, 1164 px) and again by `3bcbfdd` (8 Nov, `df499589…`, 1536 px), the bytes committed now. The row kept saying *yes, in the declared state* through both moves because the state it described was a recollection: nothing declared the census, so nothing could disagree with it. `PLOCK_CAPTURE_SESSIONS` is that declaration and `cdp.require_declared_sessions` is the disagreement |
| `dashboard.png` | **yes, in the declared state** | was **no** on two counts, both closed. (1) *Time of day*: the row text is a branch of `schedule.Evaluate(p, now)` — `locked out until` / `scheduled, not enforced` / `not now; next at` / `nothing scheduled` — so the same fixture honestly built rendered a different document at another hour. Casey is now held by an `allDay` rule on all seven days and alex's profile has no enabled rule, and `Page("/").state` names the four cells (`locked out until 00:00`, `no desktop session`, `nothing scheduled`, `x11 · seat0 · locked`); `require_declared_state` refuses any other reading, so a frame taken outside the state reports instead of committing. What that costs is stated rather than buried: the `next at` branch is in no photograph and cannot be, because no rule is both never-running and still scheduled, so its home is `TestOutsideTheWindowTheRowShowsTheNextOpening`. (2) *The host*: the Sessions cell came from this machine's `loginctl` (`internal/platform/command.go:133`, resolved through `PATH` at `internal/platform/exec.go:74-79`), which is why the committed frame said `session list unavailable` while three fresh captures said `no desktop session` and `manifest.py check` called both current. `testing/capture/bin/loginctl` answers instead, and it gives the disabled account a session and the held one none — a session for a held, enabled account would be locked on the next tick and that lock would append a row to the trail `audit.png` photographs. **Coupled to #209:** on a tree without that fix this page says `until 05:00` for the midnight the rule was written with, and the declared state refuses the frame rather than committing a page whose sentence is wrong. **#220 closed the other way, at the fixture (5 Nov):** the fixture's second enabled rule now releases at the all-day rule's own instant (`pages-setup.sh:127`), so the row reads these words at every hour instead of only between 00:00 and 20:59, and the declared state gained the rule's *name* as a second needle because #220 moved both cells and a clock-only guard would have passed a frame that re-blamed the row at 21:00. Measured by `internal/schedule/fixture_hours_test.go`: 1440 minutes of the declared day and 1464 instants across 2026 give one verdict, and the same sweep with the old `06:30` written back gives two — 180 minutes of them, which is the disagreement #220 reported |
| `users-no-profiles.png` | **yes, measured** | new with #198: the `/users` page from a **second fixture** (`pages-noprof-setup.sh`), photographed because #192 changed what this page says when there is nothing to assign and no artifact showed it. Two fresh fixtures two runs apart gave `8bbb8293e0c0…` both times and both equal the committed file — that frame is `f56dc29`'s, recorded at `captured_at 2026-11-01T05:43:26Z` and re-recorded unchanged by `d439f70` at `2026-11-01T22:47:43Z`. Re-measured on #207's head under two host timezones, `ce38334c39ad…` came back both times: equal to each other, and equal to no frame this repository publishes — the two it carries that were re-recorded elsewhere and adopted by merge are `dac864f4…` (`9e07dde`, in at `dbdbf4b`, the #221 merge) and `9173e9b4…` (`b9eeec3`, upstream through `ffd63ab`, in at `d246d51`, the merge of `main`); neither commit is on this branch's first-parent line — so that second value is a run-vs-run citation, the same class as the seven in the two-zone pair, and not a pointer into `docs/screenshots/360/`. It is the only artifact here whose *fixture* differs rather than its *state* -- see "A second state, or a second fixture" |
| `audit.png` | **yes, measured** | measured 6 Nov, not merely by construction — and two runs stand behind that verdict, certifying two different things. For the bytes committed today, `D-0037`: the nine-frame run of 16:33:49 to 16:34:09Z wrote the set `b9eeec3` commits, and the earlier round it was compared against, at 16:17Z, came back `cmp`-identical on all nine — four fixtures re-seeded and two separate Chrome rounds between them — so a fresh run does reproduce these pixels, on the machine that photographed them. For the host-zone property this page exists to demonstrate, the two-zone pair below under "What the audit fixture is not allowed to contain", which certifies that property and nothing beside it: the two runs came back byte-identical **to each other** for the seven page-fixture artifacts, and none of the seven values that pair printed is a committed file at any head this repository still publishes, so it never entered the set `b9eeec3` replaced and it is not evidence about these or any other committed bytes — re-running the pair against the current set is what would close that. No digest is quoted for either run here, deliberately: `docs/screenshots/360/manifest.json` is the record of what is committed, and the value this row carried was replaced in that manifest by `b9eeec3` while the pull request waited on review — which is the rot the paragraph above this table was written to stop (#144). What changed is that the claim became checkable at all — `manifest.py check` will now refuse a re-taken frame, where the 28 Oct run above could only record that `audit.png` churned. It was **no** before that, and the mechanism was worth the issue: signing in over HTTP writes `auth.success` into the trail the page photographs (`internal/web/handlers.go:150`), so every capture appended a row and the height was a **run counter** — `360x15004` committed against `2936 px` from a fixture built from nothing, ~254 px per run, already ~92% of Chromium's 16 384 px capture surface (#144 has the bound). Three things now hold it: the fixture seeds its trail at a declared instant instead of `datetime.now().astimezone()`; `cdp.pin_appended_rows` re-stamps the sign-in's row to that instant and refuses if the count or the kind is anything but the eight declared `auth.success` rows — so an enforcement row cannot arrive with a plausible timestamp on it; and `require_declared_rows` refuses a *page* that renders a number other than the fixture's. A third cause surfaced while this was being written and is measured, not reasoned about: the When cell was formatted with `MST`, and Go's `time.Parse` adopts the local Location when a row's offset matches it, so one trail rendered `2026-03-18 21:33:05 EDT` under `TZ=America/New_York` and `2026-03-18 21:33:05 -0400` under `TZ=UTC` and under `TZ=Asia/Kolkata` (three runs of a nine-line program). `PLOCK_CAPTURE_TZ` pinned the reader's zone so the frame repeated; **#210 removed the dependency at its root** -- the column now prints the offset the trail stores, so no host zone can move it and the declared state for this page is `2026-03-18 21:34:05 -04:00` on every machine. The zone stays declared as a canary rather than a cause: the clocks on the other pages come from each profile's own declared zone (`wallClock`, `internal/dashboard/banner.go`), so a frame that disagrees with its declared state *while* the zone is set is the sign that a host-dependent label has come back. Say the other half too, because #210 removed a trip wire: this row used to claim the declared state would refuse an unloadable `TZ`, and under an offset layout it cannot -- Go's UTC fallback prints the same `-04:00` a loaded zone does |

So seven of the nine reproduce on any machine — the six from the `pages` fixture and one from
`pages-noprof` — and the two POST artifacts, `lock.png` and `testlock.png`, reproduce on an account
holding the census the frame declares, because those two pages photograph the real session census on
purpose: their subject *is* a lock on a real session, so they are the two fixtures that must not be
handed `bin/loginctl`. Since #213 that census is a declared condition of both — `PLOCK_CAPTURE_SESSIONS`,
written into `fixture.env` by `capture_declare_census` from the machine's own answer, checked against
the rendered rows by `cdp.require_declared_sessions`, and published in the manifest as `sessions` — so
"the number of sessions the capture had" is now a fact a run reads out of a file instead of one a
reader guesses from a height.
Re-taking those two was **#213**'s first half, and it is closed twice over, by two runs that disagree about
nothing. #221 took both frames on 5 Nov as part of re-photographing the whole set for the masthead,
and that ended the split: those two were the last frames taken with page scripts running, and the
nine committed files became one set again — every one photographed with scripts off from the login
onward. This branch took them again on 6 Nov, at the commit its manifest `head` names, and both came
back **byte-identical** to what was committed at that moment (`b59bb2be…`, `acc5ba4d…`) — the arm of
the scripts switch that produces the same file, because the only thing a page's own scripts could
change on those two pages is a countdown neither page renders. So the re-take moved the *provenance*,
not the pixels: those digests became the output of a run whose session really did disable the page's
scripts, which is what the manifest asserts and what the October frames could not witness. "What the
scripts condition can and cannot witness" below has the measurement, including that arm and the
control where the same switch does move a frame.

One hazard this folder used to state wrongly, and it is the reason #213 sat unclaimed for a week: the
notes here said a re-take "locks the capturing machine's session for real". It does not, and the
thing that would is `enforcement: true`. The POST fixtures set `"enforcement": false`,
`"lockBackends": ["command"]` and `"lockCommandPath": "/bin/true"` (the config block in
`post-fixture.sh`, not the header comment above it), so the click is answered by `/bin/true`, the
sentence the page renders is the *unconfirmed* one — which is the sentence the template exists to
render — and the operator's screen is never asked. Measured 6 Nov, around the run that took both
frames: the capturing account's own logind session went `active` → `active` and `LockedHint`
`no` → `no`, while the fixture's `audit.jsonl` filled with `lock.requested` and `lockbackend.probed`
lines aimed at uid 1000. Re-take them when they go stale; the operator's screen is not the price.
What that leaves open is the census itself: those two frames describe one account holding **two**
lockable sessions on one machine — `tty` plus `x11`, both `isLockable`, `PLOCK_CAPTURE_SESSIONS=2` in
their manifest entries. Another machine needs that census to reproduce them, and it now learns the
number from the manifest and is refused by the driver if its own logins disagree, rather than finding
out from a height in prose.

## What the audit fixture is not allowed to contain

`audit.png` used to be the one artifact here that a run could not reproduce, and that is exactly why a
fabricated row survived review twice: nobody could regenerate it and compare. #212 took the run counter
out from under it — the `rows` declaration, `cdp.pin_appended_rows`, `require_declared_rows` — so the
comparison this section assumed impossible is now a command, and the two-zone pair above was run on
6 Nov. Its scope has to be stated at its own size, because this paragraph used to claim more than the
run did. Two fresh fixtures, about a minute apart, on one machine under two host timezones, came back
byte-identical **to each other** for the seven page-fixture artifacts that pair names one by one. Not
nine: `lock.png` and `testlock.png` come from `lock-setup.sh` and `testlock-setup.sh`, and that pair
did not include them — `fixture.sh:146-151` kept their census undeclared at the time, because "a
stubbed census would photograph a fiction", so the session-census condition had nothing to be varied
against. **#213 closed that**, and the missing run was made on 10 Nov with the census declared: the
two POST fixtures were each set up and photographed twice on one head, once at the host's own zone and
once under `TZ=Asia/Kolkata` (the same second zone the seven-frame pair used), and all four frames came
back equal to the committed bytes — `lock.png` `df49958928685fa0…` from both arms, `testlock.png`
`d285d1cd0d0389b6…` from both, and equal again under a third run per fixture. The census is what makes
that claim mean something: the zone moved nothing *and* the number of rows the two pages rendered was
the declared `2` in every one of those runs, so the frames are the same page six times rather than the
same height by luck. And not about committed bytes: none of the seven values that pair prints is a committed file at
any head this repository still publishes — the walk is over a named set, `refs/remotes/origin/*`,
the nine published heads in this clone, where the seven paths carry 11, 9, 11, 6, 16, 6 and 3
distinct historical blobs in `dashboard`, `users`, `profiles`, `profiles-refused`, `audit`,
`settings` and `users-no-profiles` order and none of the seven values above is among them. A clone
with a different set of refs counts differently — one carrying `refs/pull/*/head` reported 12 values
for `dashboard` and 20 commits for `audit` — so the sentence is scoped to the command that produced
it rather than to “every ref”, and adding a ref can only add a value, never remove one. The pair
compares one run to the other and to nothing else.
That is the whole of what it says about bytes, and it is worth naming where the stronger clause came
from: the paragraph above the reproducibility table that opens "#207 changed the answer for every
page-fixture artifact" reads "Against the previously committed bytes, `settings.png` and
`users-no-profiles.png` came back **identical**", and that sentence is a re-take that happened: it is
`d439f70` (2026-11-01), which arrived with #212 rather than #207, its own message records "Five files move
because their fixture content moved … users-no-profiles.png and settings.png came back byte-identical",
and the equality needs one command, not a digest — `git show d439f70 --stat -- docs/screenshots/360`
lists five PNGs changed and does not list those two. What the clause is not is a claim about this pair.
Carrying its clause into the pair's own sentence asserted a hash
relation and its negation about the same seven PNGs, in the paragraph that prints the seven values
refuting the first half. What the pair establishes is that no host zone can move these pages, which is
the one thing worth quoting it for.

What has not changed is *why* the three rules below exist, and it is worth stating now that the old premise
is gone: reproducing the pixels proves the frame comes back, it does not prove the rows are ones the
product writes. The mistake was made twice, both times in the same way — a kind the fixture author
assumed the product emits. `auth.login` was caught in review; `config.changed` was caught later, and by
then the picture had already been committed.

Three rules keep the trail honest, all of them in `pages-setup.sh`:

* **Every kind is checked against the binary.** Go keeps a string constant in the executable
  verbatim, so a kind absent from `$PLOCK_BIN` is a kind nothing in the product writes. The
  fixture refuses if any seeded kind is missing. This is a floor, not a proof — it says the word
  exists, not that an emitter reaches it — and it catches exactly the two mistakes made so far:
  `lock.asserted`, `lock.released`, `lock.unconfirmed`, `lockout.entered`, `override.granted`,
  `auth.failure` and `auth.success` are all present in `plock`; `auth.login` and `config.changed`
  are both absent.
* **Detail text is copied from the emitter, not paraphrased** — `assertedDetail` and
  `unconfirmedDetail` in `internal/enforce/enforce.go`, the `"%s until %s by %s: %s"` format in
  `internal/web/override.go`, the throttle key in `internal/web/handlers.go`. Two shapes fall out
  of reading those and neither is guessable: `srv.Audit` sets `Actor` and `Detail` and never
  `Username`, so a row the web layer wrote renders `nobody` in the Account column; and only an
  *enabled* user has engine rows, so a lockout line for a disabled account is a state the engine
  never reaches. The `platform.error` row goes one step further and reads its payload from
  `internal/platform/testdata/list-sessions.json` rather than typing one: that file is what
  `logind_test.go:86` names as a live systemd's output, and its field set is exactly what
  `logind.go`'s `listEntry` decodes. The hand-written JSON this replaced carried `idleHint`,
  `remoteHost`, `service`, `type` and `state`, none of which `loginctl list-sessions` prints — so
  the rule was false on the one row whose text decides the page's width, and the real listing is
  also 251 unbreakable characters where the invented one was 192.
* **Rows go into the file oldest-first.** `audit.Reader.Tail` does not sort — it reads the tail and
  returns it reversed, on the assumption that a writer only ever appends. A fixture that seeds
  newest-first therefore renders oldest-first, and the page labelled "newest first" shows the
  login at the top followed by rows climbing. That one was found by looking at the capture, not by
  reading the fixture.

The timestamps carry the fixture's **declared** offset (`CAPTURE_ZONE` in `fixture.sh`, `-04:00`) and
not the machine's own, which is #207's cause 1. The reason the offset is written as an *offset* rather
than a zone name is the one `pages-setup.sh:243` gives: "that is what the daemon's writer stamps" —
`Writer.marshal` writes `time.RFC3339Nano` (`internal/audit/audit.go:264`), so the trail on disk holds an
instant with an offset and never a name, and a fixture that wrote a name would be writing a field the
product does not have.

The hazard the pinned offset answers is the one this sentence used to assert as a fact about the host: the
page prints the offset each row carries, so a trail that mixed a `Z` row with the seeded `-04:00` rows
renders `15:48:17 +00:00` beside `11:52:18 -04:00` in one column — two zones under a "newest first" label,
which cannot be read top-to-bottom. Choosing the offset by host made even that difference a property of
who ran the capture. The second cell could once have come back `EDT` as well: the column was formatted
with `MST`, and Go's `time.Parse` adopts the reader's own Location when a row's offset matches it, so one
trail rendered `21:33:05 EDT` under `TZ=America/New_York` and `21:33:05 -0400` under `TZ=UTC` and
`TZ=Asia/Kolkata` — the three-run table is in `internal/web/audit_row_zone_test.go:16-26`. **#210 fixed it
in the template** (PR #214 merged it): the cell prints `"2006-01-02 15:04:05 -07:00"`
(`internal/web/templates/audit.tmpl:98`), the offset the row stores, so no zone *name* can appear in that
column on any machine. What is left to guard is the mixed offset, not an invented label — which is why the
example above has offsets in it and no abbreviations.

## A second state, or a second fixture

Two artifacts can share a URL for two different reasons, and picking the wrong mechanism produces a
photograph that is impossible to tell from a correct one.

**A second state** (`profiles-refused.png`) is the same document reached by a different request. One
fixture, one daemon, one POST in front of the GET; the `Refusal` on the `Page` declares what to fill
and which sentence has to come back.

**A second fixture** (`users-no-profiles.png`) is a different *document*. `users.tmpl` chooses the
no-profiles branch on `{{else if $.Profiles}}`, which is a property of the config, not of the request
-- so it needs its own config, its own daemon, its own port and its own `runs/` directory. `--fixture`
names which one the driver photographs, `PAGES` says which artifacts belong to it, and the page's own
`needles` refuse a run that reached a document that does not say them. The last part is load-bearing:
the ordinary accounts page fits at 360 px, has rows and has a label in every cell, so every width and
census check passes on a photograph of the wrong thing.

An earlier version of this also wrote `PLOCK_CAPTURE_MODE` into `fixture.env` and had the driver
compare it with `--fixture`. That check cannot fire: `capture_begin` takes the run directory's name and
writes both halves from it, so there is no second source to disagree with -- and the self-test rule
built on it passed because the *fake* let `loaded` be set apart from the name, which is a state
production cannot reach. Deleted rather than commented. The needles are the falsifiable version, and
`a_page_from_the_wrong_document_refuses` is the rule that says so.

**What is deliberately not here:** the dangling-entry row -- an account whose `profileID` names a
profile that is gone. That document cannot be booted. `config.Load` validates nothing
(`internal/config/store.go:48`) but `serve.go:97` validates after it, measured:
`users[0].profileID: "school" does not name an existing profile`, and the daemon exits 1 before it
answers. The web UI cannot produce it either -- `DeleteProfile` answers `ErrProfileInUse` while an
account still names the profile -- so the only route in is editing the file *while the daemon is
running*, which `readConfig` picks up on the next request. Reaching that from a driver means editing
the fixture's config between boot and navigation: a state **transition**, which wants the `Refusal`
mechanism beside it and its own needle, not a static document. It stays pinned by
`TestTheUsersPageStillLetsADanglingEntryBeUnmanaged` and unpinned by any artifact.

## Re-capturing the committed artifacts

```sh
go build -o plock ./cmd/plock        # first, every time — see the binary note below
./pages-setup.sh       && ./pages.py --out docs/screenshots/360 --overwrite
./pages-noprof-setup.sh && ./pages.py --fixture pages-noprof --out docs/screenshots/360 --overwrite
./lock-setup.sh        && ./lock.py --out docs/screenshots/360 --overwrite
./testlock-setup.sh    && ./testlock.py --out docs/screenshots/360 --overwrite
cd "$(git rev-parse --show-toplevel)"   # `--dir` and `--fixture-env` are shell-relative, `--out` is not
python3 testing/capture/manifest.py record --dir docs/screenshots/360 \
  --fixture-env testing/capture/runs/pages/fixture.env \
  --fixture-env testing/capture/runs/pages-noprof/fixture.env \
  --fixture-env testing/capture/runs/lock/fixture.env \
  --fixture-env testing/capture/runs/testlock/fixture.env \
  --shot audit --shot dashboard --shot users --shot profiles --shot profiles-refused \
  --shot settings --shot users-no-profiles --shot lock --shot testlock
       # last, after every frame in the set is re-taken, and *named*: see the note below
```

`pages.py` records itself and knows what it photographed, so the flags above are for the set as a
whole: they are what keeps `lock.png` and `testlock.png` — whose drivers write no manifest — stamped
from the runs that took them instead of keeping October's block on the strength of unchanged bytes.

**Build first, and mean it.** The fixtures test that `$PLOCK_BIN` exists (`fixture.sh`), not that it
is newer than the sources compiled into it, and the templates are `go:embed`
(`internal/web/sources.go:21`) — so a binary left over from before a template edit serves the page
that binary was built from, and every measurement downstream agrees with it because the browser
faithfully photographed what the daemon actually said. Measured 5 Nov: a binary built nineteen minutes
before the commit that changed the masthead re-took the set and left five of the seven page-fixture
frames byte-identical to the pre-change files, moving only the two whose *fixture* content had changed
— a set that looks exactly like "the two this branch moved" and would have been committed as evidence
for a page nobody photographed. `fixture.sh` now refuses a binary older than the sources compiled into
it and names them; the build line above is the fix it asks for.

**Record last, and name what you took.** `record()` walks the PNGs sitting in its target directory
(`pngs_in`, and `record`'s loop over it), so the frames it may *re-date* have to come from somewhere
other than the directory listing: that is `--shot`, and since D-0031 it governs `sources_rev` as well
as the conditions block. A frame this run did not name, whose bytes are the bytes the manifest already
recorded, keeps the `sources_rev` it already had and gains `carried_from`, naming the manifest it came
from; the green sentence then says so (`4 of them are carried … this run did not re-take them`) instead
of counting nine. Before that, pointed at the committed directory, a partial re-capture advanced
`sources_rev` for the frames it left alone and `check` called those stale photographs current: measured
5 Nov after the two `pages.py` lines above, the check reported *9 photographs current* while
`lock.png` and `testlock.png` were byte-for-byte the previous files, and measured again 6 Nov on
`origin/main` with each path's own commit supplied, one `record` over four untouched frames answered
*0 problems* where the drift arm has four refusals to make. The identity check could not see it then
and cannot now, because the recorded hash matches the stale file it was recorded from — which is why
the carried rev, not the digest, is the thing that had to stop moving. Since #294 the naming is not a
courtesy but the precondition for recording at all: a frame whose bytes moved that the record does not
name is refused, so `--shot` lists every frame this run photographed and every frame it did not stays
carried.

The stronger form of the same hole, measured 6 Nov on the merge of #221: substitution, not staleness.
In a scratch copy of the committed directory with `main`'s `audit.png` — a photograph of the `MST`
cell the branch had just deleted — sitting in place of the current one, `check` refuses on identity
(`ac49f291a5cd… on disk, 0cfe9973711b… recorded`, rc 1); one bare `manifest.py record` over those same
bytes answers **`9 photographs current`, rc 0**. `record` writes `sources_rev = newest_commit(sources)`
from the commit and the clock and never asks the pixels what page they show, and `check`'s drift arm
opens `if recorded == current: continue` — so the equality the recorder mints is the same equality the
gate reads, and a re-dated frame cannot be seen to be wrong. D-0031 closes the staleness half, because
the recorder is no longer allowed to move a rev for pixels it did not write. **The substitution half
was open until #294**: the carry-over never touched it by design, because swapped bytes differ from the
manifest, so they are a *changed* frame, and a changed frame nobody names used to be re-dated anyway.
`record` refuses it now — a frame whose bytes are not the ones the manifest carries, and that this run
did not name with `--shot`, exits 1 and writes nothing, because there is no prior rev worth carrying
and no honest rev to mint. Re-measured 11 Nov at `d7a5149` through real git over a throwaway repository:
nine frames recorded at `A`, the masthead moved at `B` with `profiles.png` re-taken in that commit,
`audit.png` then replaced with another page's frame. `check` refused the swap on identity; one
`record --shot profiles` made it vanish from `check` and re-dated `audit.png` at `B`; and with
`--fixture-env runs/pages/fixture.env` supplied that frame also gained this run's `capture` block —
conditions this run never applied to it, #211 reopened for one frame. A refused record leaves the file
alone, so the identity finding is still there to be read afterwards. Two consequences: `--shot` is the
only input separating a re-take from a substitution, so it names **every** frame the run took,
including `lock.png` and `testlock.png`, which no driver records for; and the bare command is still
right for a set this folder took whole and honest, because those frames are unchanged and carry.

**A frame the manifest has never seen is the same mint with nothing to carry**, and `record` refuses
that too (#294) — but on a comparison, never on the absence: named by nobody *and* its own pixels last
committed at least one commit before its page. The absence cannot be the trigger, because "no entry" is
also what an honest capture looks like before its commit — `pages.py` records the frames it has just
photographed while every one of them is still untracked, and `fixture.sh` says *Commit last*. Refuse
the absence and `make capture` is red on its first run, which is #144's trap wearing the fix's clothes;
the rule `a frame new to the manifest with pixels older than its page is refused, and a frame with no
history at all is not` tests both orderings so that mistake cannot be made quietly. Minting matters more
for an unrecorded frame than for any other, because an unrecorded frame is the one case `check` *does*
see — `is committed with no entry in` fires on it — and recording it deletes that finding and installs
in its place the drift arm that compares a fabricated rev against nothing.

The specimen one step further along is in `git show 7e65d3b` — the state of *that* tree, not the
published set: there none of the nine carried, because `carried_from` arrived a commit later; the split
is in `git show 7ff2a67`, the blob `main` publishes today, where `lock.png` and `testlock.png` were the
only two of the nine with no `carried_from`, both revs minted at `2088043cb`. `7e65d3b` had just moved
all nine rows off `3bcbfdd`, six of the nine PNGs' own newest commit and an ancestor of `2088043`, onto
the page commit eighteen seconds after it landed. By this file's own model those two photographs predate
their page, and `manifest.py check` there answers *9 photographs current*, rc 0: the rev the gate would
compare is the one the record wrote. `lock.py` and `testlock.py` write no manifest, so those revs are
hand-typed from wherever whoever noticed was standing — the recipe below tells them what to type, and a
commit eighteen seconds old is not evidence about a frame taken four hours and twenty-one minutes
earlier. Closing it takes a re-take, not an edit: `./lock.py` and `./testlock.py`, then `record
--fixture-env runs/lock/fixture.env --fixture-env runs/testlock/fixture.env --shot lock --shot testlock`.

**That re-take ran on 11 Nov, all nine frames, and what it measured is the finding (#301).** Every fixture
was re-seeded and every driver run through the recipe below with `--out docs/screenshots/360 --overwrite`,
then `record` ran once naming all nine with `--shot` and handing over all four `fixture.env` files. **All
nine frames came back byte-identical to the committed files** — `sha256` equality, nine of nine, measured
twice, including `audit.png`, whose pixels are the oldest in the folder. So the seven rows #301 counted
were never a picture of an old page: `2088043cb` renamed the state classes and rewrote 69 lines of
`app.css` without moving one pixel on any of those seven, which is the claim its own message makes and
repeats ("two PNGs changed and seven byte-identical — the browser's half of the claim that `.exempt` moved
no pixels"), and the two frames that did change are the two rows that were honest throughout. What was
wrong was the record, not the photograph: seven rows certified their pixels through a `carried_from`
pointing at a manifest whose `head` was the mint, and nothing in this folder can tell that from a re-take.
The delivered diff is therefore seven deleted `carried_from` lines plus `captured_at` and `head` — **no PNG
byte and no `sources_rev` moved** — and that is also its limit: `current` is still `newest_commit(sources)`,
so the file still cannot distinguish the run that photographed nine frames from a hand edit, and `check`
still answers `9 photographs current` having asked history nothing about the seven. #302 owns that half.

**And #301's other remedy is not merely harsher — it cannot be satisfied, a trap worth keeping for whoever
reads the drift arm next.** Re-dating the seven rows to the newest commit over each PNG, measured in a
clone, makes `manifest.py check` exit 1 over exactly seven problems, each ending with the instruction:
*"Re-take it and re-run `manifest.py record`, in a commit after the one that changed the page or in that
same commit; what it may not be is older than the page."* That paragraph **is** that re-take and silences
none of the seven: bytes that did not move produce no commit touching the PNG, so `newest_commit(png)` can
never catch a page commit that moved no pixels. Any rule reading *"the last commit that changed this file's
bytes"* as *"the last time anyone photographed this page"* refuses a current frame forever, and the seven
it refuses are those whose page did not move — a verdict about a stylesheet edit, not about whether anyone
looked. #302 sketches exactly this arm, measured against the post-record manifest: same seven rows, exit 1.

**`--shot` is the driver's word, and since #302 that word is checkable.** The recipe names the frames
because nothing else says which ones a run took — and naming them used to be the end of it: `record`
dated a named frame at its *page's* newest commit and never asked what the frame's own pixels were last
committed at, while `check`'s drift arm opens `if recorded == current: continue`, so the equality the
recorder mints is the equality the gate reads. That is how the seven rows above read *current* for five
weeks. No later reading of history can order those bytes — a re-take whose pixels do not change never
produces a commit touching the PNG — so the answer had to come from the run. `cdp.publish` now leaves it
where it falls: `runs/<fixture>/capture.json`, beside the `fixture.env` that fixture was run with,
carrying the commit the capture stood in and the `sha256` of every frame it wrote. `record` reads it at
one moment only — the page has moved past this frame's pixels and this run claims the frame — and it
demands the frame, under the fixture whose driver photographs it, at those exact bytes; the entry then
gains `photographed`, and `check` re-asks the ordering from that block on every run after, which is the
re-take deadline this folder used to promise in prose and could not enforce. Two refusals follow and each
has a door: a page that moves past its own capture goes red, and the way out is the recipe above —
re-take, re-record — never an edit of `sources_rev`; and a `photographed` naming a commit this checkout
cannot order (a rebase, a capture taken in another clone) says which and points at `git fetch`. What
stays open is printed rather than hidden: `record`'s summary counts the rows dated on its own arithmetic
because no receipt covers them, since the alternative is refusing the ordering this folder prescribes —
capture, record, commit, in that order — which is #144's trap in the fix's clothes.

A receipt is one commit conservative, and that is not a defect to tighten: the driver photographs and
records **before** the operator commits, so the commit a receipt can name is the parent of the commit
that lands the photograph — for every honest capture, every time. The claim therefore cannot be the
only witness standing behind a claimed frame, or this folder's own recipe refuses itself. The one commit
that changes a template, re-takes its frame and re-records the manifest is legal here, and #302 lists it
among the orderings that must stay legal; measured 13 Nov with real git over a throwaway repository it
was refused, by the claim arm, on exactly that recipe — `/tmp/opencode/302b/probe.py` case 1 and
`claimpath.py`, which is the receipt-backed path at the self-test's own scale. `origin/main` accepts the
same ordering, via D-0056's comparison over the blob. So the frame's own newest commit now gets the same
vote the drift arm gives it: the claim clears when a commit at or after the page's touches the
photograph, pinned by the self-test rule *"a `--shot` frame whose own pixels moved with the page's commit
is not contradicted, though the receipt behind it is one commit conservative"*.

What that does not buy is the frame whose bytes did not travel. A re-take that changes no pixel produces
no commit touching the PNG, so the frame's history stays behind the page, the claim is the only witness
left, and a claim one commit conservative cannot tell a byte-identical re-take from a frame nobody
photographed at all. That row is red on `origin/main` as well — same probe, 9 problems on both trees,
#144's arm there and this arm here — and it is #338's finding, not a residue of this one. What #302
changes about it is the sentence: the repair named here is reachable (re-take, re-record — measured
green, `probe.py` cases 4 and 7), where the advice on `main`, to re-take the frame "in that same commit" the
page moved in, cannot be followed by anyone for a frame whose pixels did not move. It stays listed
rather than guarded because the one witness that could close it is the run's word about when it
photographed, and that word is what this arm already spends.

The `*-setup.sh` on each line is not ceremony. #207 made a fixture single-use for the sign-in,
because the sign-in appends the one row whose count the artifact's height is: re-running the driver
against a fixture it has already signed into is refused --
`this fixture declares 8 rows on the page and the trail holds 9` -- measured 5 Nov on the merge of
#202 and #207, where the recipe above was typed without its setup and exited 1. That refusal is the
check doing its job, not an obstacle to work around: a second sign-in really would make the height of
`audit.png` a run counter again. Re-seed, then take.

Each fixture writes its own `runs/<fixture>/` scratch directory and records its manifest there, so a
one-page re-capture cannot certify another run's PNGs as just-taken. Both invocations above land in
the committed directory only because `--out` names it.

`--out` resolves against the **repository**, not your shell's directory, and refuses a target
outside it. Both halves are measured: `--out docs/screenshots/360` typed from `testing/capture`
used to create `testing/capture/docs/screenshots/360/`, which the `runs/` ignore rule does not
cover, so the operator believed they had re-taken the evidence, the committed files were
untouched, and a later `git add -A` would have committed four PNGs nobody meant to make.

Two more holes in the same guard, both found by attacking it and both now closed. The path was
normalised rather than resolved, so a symlink under `runs/` walked out of the repository with the
check silent — it is `realpath`ed on both sides now. And `argv` was read by hand, so
`--out=docs/screenshots/360` and a mistyped `--outt` were both ignored and the driver exited 0
having written to `runs/` — the same harm from the other direction. Arguments go through `argparse`
now, so an unrecognised flag is an error.

A page change and its re-take may be committed together, or in either order, and the gate accepts
both — what it refuses is a photograph *older* than its page. So the workflow is: edit the page,
re-take with the command above, `git add` both, commit. `manifest.py record` runs as the last step of
a successful capture — the driver hands over the `fixture.env` of every fixture it photographed and
the artifacts it just took — which is why the manifest and the PNGs land in the same commit as each
other. Recording a frame a driver does not record (`lock.py` and `testlock.py` write no manifest) is
the one case that takes the flags by hand: `record --dir … --fixture-env runs/lock/fixture.env
--shot lock` — one `--fixture-env` per fixture re-seeded, and **one `--shot` per frame re-taken**, so
once both POST drivers have run it is `--shot lock --shot testlock` with both envs. The flags are not
interchangeable with the envs, and the difference is measured, not stylistic: against the pre-#294
recorder, `--fixture-env runs/lock/fixture.env --fixture-env runs/testlock/fixture.env --shot lock`
typed after *both* POST frames had been re-taken and the masthead had moved again recorded
`testlock.png` at the masthead's commit while the newest commit touching its own pixels sat one commit
behind — and `check`, which named the seven stale page frames one at a time, said nothing about that
one (`recorded == current` is the arm it skips on). One flag per fixture is the mistake; a flag names a
frame. Ordering is the other half of the honesty: type `record` in the same commit as the re-take, not
several commits after it, because `--shot` is the only thing that says when a frame was taken while
`sources_rev` is written from wherever the run is standing.

`--overwrite` is a second flag on purpose. Writing this README and then testing its own
invocation replaced two committed artifacts with no error and no warning — overwriting is what the
flag is for. Replacing committed evidence should take two tokens, not a tab-completion.

## The manifest: what a committed PNG is evidence about (#144)

The driver refuses to write anything when any page overflows. That makes the dangerous failure not
a bad artifact but **no artifact**: the run writes nothing, the PR merges anyway, and the old
photograph stays in `docs/` describing a page that has since grown two forms. Nothing in the
repository compared a committed PNG against the page it depicts, so `make verify` stayed green
through all of it. `profiles.png` on `main` is current only because #147 re-took the whole set;
before that, the newest commit touching it predated #141.

`testing/capture/manifest.py` closes it. A successful run writes `manifest.json` beside the PNGs:

```json
{"schema": "capture-manifest/2", "viewport": 360, "head": "a889d0d8b…",
 "captured_at": "2026-10-28T01:55:37Z",
 "pages": [{"page": "/profiles", "png": "profiles.png", "png_sha256": "1f711bea…",
            "sources": ["internal/web/templates/profiles.tmpl",
             "internal/web/templates/page.tmpl", "internal/web/static/app.css",
             "internal/web/static/app.js"],
            "sources_rev": "0a42139d…",
            "capture": {"fixture": "pages", "stamp": "2026-03-18T21:34:05", "zone": "-04:00",
             "tz": "America/New_York", "rows": 8, "census": "testing/capture/bin",
             "scripts": "off", "state": [["casey", "locked out until 00:00"], …]}}]}
```

The `capture` block is #211 and the section below it is why. A manifest whose every entry carries one
is `/2`; one where some frames still state nothing — which is what a partial re-record produces, since
only `pages.py` records and nine artifacts come from three drivers — stays `capture-manifest/1`, keeps
whatever blocks it has, and `check` reports how many of the entries state their conditions. `/2` is
claimed only when the promise holds for every entry, because `check` holds every entry to it.

and `make verify` runs `manifest.py check`, which refuses when:

- a PNG has no entry, or an entry has no PNG — the manifest and the artifacts have come apart;
- a PNG's bytes are not what the manifest recorded — it was replaced without re-running the driver;
- the newest commit over that page's `sources` is **newer than the newest commit over the PNG** —
  the page grew and the photograph did not;

The comparison is an ancestry test once it gets there, and the equality against `sources_rev` that
precedes it is not innocent: the arm opens `if recorded == current: continue`, so the value the
recorder wrote is what decides whether the history is consulted at all. That is how #222 passed — a
partial `record` mints `sources_rev = current` for frames it never photographed, and the ancestry test
below is then never reached for them. D-0031 stops the recorder writing that value for a frame it did
not take, which is the only reason the sentence "the comparison is an ancestry test" is true of a
partial re-capture as well as a whole one. The first version got the ancestry test itself wrong in the
direction that costs an hour: it compared the manifest's own commit with the page's, so a commit that
changed a template *and* re-took its photograph — the one commit in which both are true — was refused
by the very change that made the photograph current, with a sentence asserting the opposite.
`git rev-list --count <png>..<source>` asks the history instead of asking the manifest, so
a page change and its re-take may share a commit, a re-take with no page change is current, and the
failure this file is about still refuses. (The spelling matters: `--count <png> ^<source>` is zero for
a descendant as well as for an ancestor, which accepts exactly the case the rule exists to refuse. The
self-test caught it; the drift rule stopped firing.)
- a source has an **uncommitted** change — the common case, and the one `git log` cannot see;
- the driver photographs a page the manifest has no entry for — a fifth page added to `PAGES` and
  never photographed;
- the manifest was recorded at a viewport other than 360;
- an entry names no PNG — refused as an entry, not as a traceback from hashing the artifact
  directory;
- a manifest that declares `/2` and has an entry with no `capture` block — the schema promises
  conditions, so an entry without one is a hole in the promise rather than an old manifest;
- a block whose `fixture` is not the fixture the driver photographs for that page, a `scripts` value
  that is neither `on` nor `off`, or a `state` the driver no longer declares — the three ways a
  recorded condition becomes a confident wrong answer instead of an absent one;
- **the checkout has no history to date anything against.** A shallow clone, or a source no visible
  commit touches, produces `screenshot manifest was not checked` and nothing else — never a count of
  stale photographs. `actions/checkout@v4` defaults to `fetch-depth: 1`, and the verify job therefore
  sets `fetch-depth: 0`; without it every page in the repository looks stale on a runner whose
  screenshots may all be current, which is a sentence about CI that sends the reader to this folder.

The manifest carries no `scroll_width`. It held the viewport, not a measurement, and it cannot be
fixed by plumbing the measurement through: `record()` is called once, by `pages.py`, which has never
measured `lock.png` or `testlock.png` — two of the seven entries would still have been assertions. A
width belongs in the capture transcript, where it is measured and refused on every run, not copied
into a file that can only ever be older than the measurement it claims to record.

### What a frame was taken under (#211)

Two frames of the same page can be different evidence. The same template renders a different
photograph under a different clock, a different reader zone, a different number of seeded audit rows,
a different session census, or with page scripts running — and #207 proved that a manifest which
names none of those can be read as "current" by someone with no way to know the frame was taken on
the capturing machine's clock, from a census of whatever `loginctl` said that day. So the recorder
copies the conditions out of the run's `fixture.env` and `check` refuses when they stop agreeing with
the drivers:

- **the binding is the driver's, never a guess from the file name.** `pages.py` declares
  `Page.fixture`; `lock.py` and `testlock.py` declare `FIXTURE` beside the code that uses it.
  `manifest.py` imports both, because a second table of the same pairs is a second thing to forget to
  update, and the forgotten update attributes one fixture's clock to another fixture's pixels.
- **an absent condition is never filled in.** `pages-noprof` seeds no audit rows and declares no
  reader zone, so its entry has no `rows` and no `tz`. A `scripts: "off"` written because the driver
  disables scripts *today* would be #211 one layer down.
- **the driver's words, not a transcription.** `state` cells are read out of `PAGES`, so the manifest
  cannot keep certifying a reading of the page that the fixture no longer builds.
- **provenance is what this run photographed, not what changed.** A frame the driver re-took is
  stamped from this run's env even when its bytes are identical — move the fixture's clock and
  `settings.png` comes back unchanged, because nothing on it prints a time. Frames this run did not
  touch keep the block the manifest had, because that block is the only remaining evidence about
  those bytes; if the fixture's declaration has moved since, `record` says so on stderr rather than
  quietly overwriting or quietly keeping.
- **a path is recorded as the repository's.** `census` is an entry on the daemon's `PATH`, absolute in
  the env, and repo-relative in the manifest: a committed file that says
  `/home/rodney/prj/…/testing/capture/bin` describes one checkout on one machine and publishes a
  local path. A path genuinely outside the repository is passed through, not rewritten with `..`.

Measured 6 Nov, from four fresh fixtures on one head: all nine artifacts reproduce the committed digests
byte for byte — `lock.png` `b59bb2be…`, `testlock.png` `acc5ba4d…` — and `manifest.py check` passes
against the directory with nine entries and nine blocks, the two POST entries now stamped from the runs
that took them. The same nine digests were quoted on 4 Nov in support of a claim this branch has since
withdrawn: that because the files reproduced, the frames *were* the scripts-off frames. Both values of
`scripts` produce those files, on those two pages, so the digests could not have settled it — see "What
the scripts condition can and cannot witness" below for the measurement in both directions and for what
the field is entitled to say instead.

One lesson from building the guard, because it cost three attempts: **a CLI cannot be pinned by
reading its own text.** Grepping `main`'s source for `--fixture-env` survived the flag being renamed —
main's closing sentence contains the string. Grepping the rendered `--help` survived the flag being
deleted — the help of the flag next to it says "its conditions are taken from `--fixture-env`". The
rule passes the tokens to the parser and checks they arrive at `record`, which no sentence in the file
can satisfy. `--help` is deliberately not one of the tokens: argparse acts on it the moment it sees it
and never validates the rest, which is how the second fix stayed green.

What the source set is *not* is the Go code. It is the presentation layer -- templates and static
-- because that is what a photograph is evidence about at the pixel level, and a change that alters
a page's markup reaches a template in practice. A change that alters its **data** (a session census,
a clock, a lock verdict) is the churn the table above already refuses to call a reproduction, and
no manifest can make a time-varying page byte-stable.

Every page's source set includes `page.tmpl` and `app.css` on purpose. The shell is one template
and the stylesheet is one file, so a nav link or a wrap rule moves the pixels of all five pages;
a manifest that scoped each page to its own template would keep four stale photographs looking
current, which is the failure this is here to prevent.

Two decisions worth keeping:

- **mtimes are not evidence.** A checkout's mtimes are the checkout's, not the capture's, so a
  clone made today would find every PNG "newer" than every template. The comparison is against git
  history, the only clock here that means what it says.
- **the gate is pinned to its own invocation.** `manifest.py --self-test` reads the Makefile it
  ships in and refuses if the `manifest.py check` clause is gone. Deleting the clause is otherwise
  indistinguishable from a repository with no stale screenshots — both are a green `verify` — and
  a gate whose invocation can be deleted silently is a suggestion.

Measured, on this tree:

```
$ printf '\n/* probe */\n' >> internal/web/static/app.css && python3 testing/capture/manifest.py check
screenshot manifest: 7 problem(s):
  audit.png is held against /audit's sources, and internal/web/static/app.css has an uncommitted
  change: the page on disk is not the page in the photograph. Re-run testing/capture before
  committing.
```

and the same check refuses with `7 committed screenshot(s) … and no manifest.json` when the
manifest is moved away. Both directions of the parser that names that file are pinned, because the
first version of this file printed `nternal/web/static/app.css`: `git()` called a bare `strip()`,
which took the leading space off ` M path`, and the parser counted three characters. A sentence
with a broken path in it is worse than no sentence, and nothing but a rule that looks at the
string could tell.

### What the scripts condition can and cannot witness

`scripts` is the one recorded condition a reader cannot check by looking at the frame, and it is the one
that was easiest to assert from the wrong thing. It changed value twice on this branch and both changes
were inferences: first `on`, because the two POST frames predated #207 putting
`Emulation.setScriptExecutionDisabled` in the shared session; then `off`, because nine digests
reproduced. Measured 6 Nov, neither had a discriminating test behind it — and until today there was no
such test to run, because `cdp.py` called the method with `value=True` and never read
`PLOCK_CAPTURE_SCRIPTS`. The override `post-fixture.sh` advertised ("to test a frame taken with scripts
running, run `PLOCK_CAPTURE_SCRIPTS=on ./lock-setup.sh && ./lock.py`") was accepted, written into
`fixture.env`, recorded in the manifest, and ignored by the browser: the two arms were the same command.
A claim that could only be checked by a test which cannot produce a second outcome is how #213's finding
walked through four gates.

With the declaration wired through to the session (`cdp.scripts_disabled`, which now refuses a value
outside the two), both arms exist and both were run:

| fixture | default (`off`) | `PLOCK_CAPTURE_SCRIPTS=on` | what the pair shows |
|---|---|---|---|
| `lock` | `df499589…`, published | `df499589…`, published | one PNG from both arms — no `time[data-unix]` on the page, so nothing the page's scripts could change |
| `testlock` | `d285d1cd…`, published | `d285d1cd…`, published | the same, on the probe page |
| `pages` | publishes; casey's row reads `locked out until 00:00` | **refused** — the row reads `locked out until 7h 46m` | the switch has power exactly where the page carries a countdown, and `require_declared_state` is what notices |

Both POST rows were re-run 10 Nov and the digests are that run's. They are **not** the values this table
carried until now: it said `b59bb2be…` and `acc5ba4d…`, and those were last the committed files at
`38578aa` on 19 Oct — `a2ccac0`, `b9eeec3` and `3bcbfdd` each moved one or both of them, so the pair had
been quoting a frame that was no longer published while continuing to prove the thing it was run to
prove. The scripts conclusion survives the staleness (both arms still give one file, still the published
one); it is the *provenance* sentence in a digest table that has to be re-measured every time it is
quoted, which is the sentence #213 is about.

Three sentences are the whole content of the field:

- **Byte identity cannot discriminate on these two pages.** One file from both arms says the render
  holds nothing the page's scripts can touch; it says nothing about which way the switch was set. So the
  field is a statement about the *run* that produced the frame, and the only way to record it honestly is
  to take the frame in a run whose session you can describe. That is why #213's re-take had to land
  before `off` could be written down at all, and why a re-take that moves no pixels is not evidence that
  the value was already right.
- **`on` is how you measure the switch; it is not committable evidence.** `static/app.js`'s only
  load-time DOM write is `renderCountdowns`, so the dashboard is where the condition has witness —
  enough to move a state cell past what the fixture declares, which is the strongest form of "the
  browser's clock is in these pixels" this folder has. A frame taken with the page's scripts running has
  never tested this folder's criterion, *usable at 360 px with JavaScript off*, so `check` refuses an
  entry that records `on`, and such a PNG belongs in `testing/capture/runs/`, not in
  `docs/screenshots/360`.
- **Absent still means "not stated", never "off"** — and no shipped frame is absent, which is the
  easier half to believe because the example this bullet used to give was false. Three of the six
  optional keys `capture_env` writes carry a non-empty default (`STAMP`, `ZONE`, `SCRIPTS`), so every
  fixture that sources `fixture.sh` states them and all nine rows read `scripts: "off"` —
  `users-no-profiles.png` included. Absence does the work on the other three (`AUDIT_ROWS`,
  `PATH_PREPEND`, `TZ`), and that is where `pages-noprof` is the real example: no `rows`, no `tz`,
  nobody chose. `PLOCK_CAPTURE_SCRIPTS=` set empty is the only entry that states nothing, and even
  that frame is taken with the scripts off, because `cdp.scripts_disabled` reads an absent
  declaration as the criterion's own condition — so absence is never evidence that the page's scripts
  ran.

## Two viewports, because one was never a criterion (#149)

`WIDTHS = (360, 1024)` in `pages.py`, and `Session.set_width` moves the CDP emulation between the
passes — `width` only, with `height`, `deviceScaleFactor` and `mobile` repeated from `__init__`, so
a pass that disagrees with the other has exactly one variable to blame.

Why a second width had to be bought is the table #140 produced while it was being fixed:

| viewport | as shipped | `overflow-wrap: break-word` | `overflow-wrap: anywhere` |
|---|---|---|---|
| 360 px | 914 px | 360 px | 360 px |
| 1024 px | 1471 px | **1471 px** | 1024 px |

The middle column is the trap. `break-word` is the value a reasonable person reaches for first, it
satisfies the phone criterion completely, and it leaves the audit page a 1471 px document on a
1024 px screen. Before #149 nothing here looked at a desktop width, so that fix was invisible
rather than wrong.

Both halves of that claim are measured on this branch rather than argued — and both required
changing `app.css` and rebuilding `plock`, because the pages are served `default-src 'self'` with
no `'unsafe-inline'` (`internal/web/middleware.go`), so a `<style>` element injected over CDP is
silently blocked and reports `overflow-wrap: normal`. A first pass at this issue "proved" a correct
fix had no effect, by that route.

| the gutting, in a rebuilt binary | 360 px | 1024 px | what notices |
|---|---|---|---|
| delete `overflow-wrap: anywhere`, leaving `break-word` | 5/5 fit | `/audit` 1471 px, refused | the new criterion, and nothing else |
| keep the declaration, override it later with `body { overflow-wrap: break-word; }` | 5/5 fit | `/audit` 1471 px, refused | the new criterion; `TestTheStylesheetPrintsTheNarrowLabels` still reports `ok` |

The second row is the one worth reading twice. #147 pins the declaration with a grep, and its own
comment says the grep pins *presence*, not effect: it survives the declaration moving out of the
rule, losing its semicolon, or being overridden later. The third of those was performed here — the
grep passed and the only thing in the repository that noticed is the criterion this branch adds.

Five things about the shape, each because the obvious version was wrong:

* **The refusal is per page per viewport, and the summary names the viewport.** `5/5 pages fit at
  360 px` and `4/5 pages fit at 1024 px; refused: /audit` are two lines; a run that prints one of
  them is not evidence about the other.
* **Every viewport is walked even when the first refuses.** Stopping early reports a page broken at
  both widths as broken at one, and the second number is the one that says whether the fix is a
  wrap rule or a column nobody sized.
* **Only the artifact viewport writes bytes.** The desktop pass measures, refuses and prints; it
  never stages a PNG. `docs/screenshots/360/` is named for one width, and two passes staging one
  basename would let whichever ran last decide what gets published.
* **The pass proves the browser moved.** `shoot()` compares the `innerWidth` the browser reports
  against the width it asked for and refuses if they differ. Asking is not evidence: `set_width`
  can record `self.width = width` and never reach `Emulation.setDeviceMetricsOverride`, and every
  rule above the fake session is then green while both passes measure a 360 px document. This is
  also what makes the overflow threshold legitimate — comparing `scrollWidth` against the *requested*
  width is only honest while the two are pinned to agree.
* **A fixture refusal carries to the other viewport.** The guards that ask whether the fixture was
  pushed hard enough run on the artifact pass only, because one broken fixture is one finding. The
  consequence was not viewport-scoped: the desktop pass measured the same page and the summary
  printed `5/5 pages fit at 1024 px` about a page the run had just refused as non-evidence. Such a
  page now prints `UNTESTED (fixture)` and lands in a clause of its own —
  `4/5 pages fit at 1024 px; untested (fixture): /audit` — because it neither failed that viewport
  nor proved anything about it.

Nine of the twenty-nine self-test rules police this contract, and thirteen guttings were run against
them — `WIDTHS[:1]`, `if False:` on the refusal test, a summary line without its width, the
`set_width` **call at the call site** deleted, `set_width` moved after `goto`, every viewport
writing bytes, the overflow compared against the constant instead of the viewport, the fixture
guards run at both viewports, refusals filed against the wrong viewport, the artifact PNG not
staged, the desktop pass reporting as the artifact pass: **11 caught, 0 survived**.

Two more were run against the *body* of `set_width`, and they are the reason the fifth bullet exists.
Both were measured surviving all eighteen rules on the head they were aimed at, and the reason is
structural: `_ShootSession` is the fake that stands where `set_width` is, so every rule above it can
only observe that `shoot()` **asked**. Nothing in the repository mutates `testing/capture` either —
the thirteen sets are Go packages — so the self-test was the only thing standing there.

| the gutting, in `cdp.py` | before the assertion | after it, on a real capture |
|---|---|---|
| `Emulation.setDeviceMetricsOverride` deleted, `self.width` kept | survived, 18 rules green | `REFUSED: / rendered at 360 px when asked for 1024 px; the viewport never moved`, then `0/5 pages fit at 1024 px; refused: /, /users, /profiles, /audit, /settings`, nothing written, exit 1 |
| the argument ignored, `WIDTH` (360) re-sent every pass | survived, 18 rules green | the same refusal on every page, exit 1 |

The 360 line stays `5/5` in both rows, which is the point of measuring at all: the phone pass is
still telling the truth, and only the pass that never moved is refused. A pin that lives below the
fake is the only kind that can see these, so the assertion is in `shoot()` and the rule drives it
with a fake that reports `innerWidth=360` at a 1024 px request.

## The self-test, because nothing else in this repository can read these files

`./pages.py --self-test` runs 48 rules about what a refusal does. It starts no browser, spawns no
daemon, writes no file, and costs **0.03 s** for all of them, so `make verify` runs it
beside the two mutation
self-tests. What it cannot do is reach below its own fakes — see the two body guttings above, which
only a real capture sees.

It exists because this package is invisible to every other gate: the mutants target `.go` and
`.tmpl` only, nothing imports `pages.py`, and shellcheck does not read Python. That is how the
following shipped past `make verify`, `shellcheck` and a 751-mutant battery at once:

`cdp.refused()` ended in `raise SystemExit(1)`. Every call to it was the first line of a caller's
own `except` handler, so **every statement after every call to it in the package was dead code**.
Measured with a stubbed `shoot()` that raises this branch's own width refusal:

| | expected | measured on that head |
|---|---|---|
| pages measured | 5 | **1** — the loop died in page 1's handler |
| `discard_stage()` | 1 call | **0** |
| the `N/5 pages fit … refused: …` sentence | printed | **never** |
| exit code | 1 | 1 — an uncaught exception is also exit 1 |

The exit code is what hid it: a refusal is supposed to exit 1, so nothing looked wrong. The same
line was dead in `lock.py` and `testlock.py`, each of which called `discard_stage()` immediately
after — so a refused capture also left its staged bytes behind. The repair splits the reporter
(`cdp.report_refusal`, which reports and returns) from the decision, which stays with the caller
because only the caller knows whether one more page can still be measured. There is deliberately
no raising variant left beside it.

The reviewer's suggested one-word fix (`refused_pages.append(spec.url)`) was tried and measured
before this was written: **it changes nothing**, because the raise happens on the line above. It is
recorded here rather than quietly dropped.

Three of the 13 probes written against the new self-test initially **survived**, and each was a
hole in the pin rather than in the fix:

* the rule fed `{"longest": None}` — a key that is *present* — so it tested the wrong branch. The
  real case is the key being **absent**, because the token census is spliced on `has_rows`.
* two declaration rules were satisfied by `capture_out` refusing the `--self-test` flag through
  argparse, i.e. they passed on an unrelated refusal. `capture_out` is now stubbed for every rule
  that reaches `main()`, so the only refusal a rule can see is the one it is about.
* the canary pair only checked the problem list, so hollowing `ran.append((why, len(problems)))`
  down to `(why, 0)` still looked satisfied. It now requires both lists to agree: `[1, 0]` and 1.

After those repairs: **13 probes, 13 caught, 0 survived**, and `make verify` fails at
`==> capture refusal contract` (exit 2) when the root cause is re-introduced.

## House rules

* **A capture is evidence when the driver's transcript is pasted into the PR, and the PNG is
  quoted beside it.** The transcript is the part that says what was refused.
* **A changed page makes its committed artifact stale by construction.** If a PR changes what a
  captured page renders, that PR re-captures it in the same branch — #103 shipped a PNG depicting
  the defect #108 later found, and the repo's own acceptance evidence argued against the fix.
* **`cdp.py` holds the shared refusals; a driver holds the checks about its own page.** The cell
  that #108 was about was copied between two pages, and a copied cell is a copied rule: the check
  that exists on one page is the check that misses the regression on the other.
* **A rule that examined nothing must not report success silently.** Every branch of the #108
  check can skip, so the drivers require the self-session note to have appeared in at least one
  Detail cell — a reword of that sentence in the templates turns a silent loss of coverage into a
  refusal. What cannot be required is a *seam*: on the probe page a lockable session's cell is the
  note alone, so demanding a second sentence would refuse correct code. The count is printed
  instead — `#108 seam: note in 1 Detail cell(s), 0 with a sentence in front of it` — so the
  transcript says whether the rule actually looked at something, rather than leaving a reader to
  guess which kind of pass it was.
* **`runs/` is ignored, and it is not safe.** It holds a config carrying an admin credential hash,
  `fixture.env` with the fixture's password **in cleartext**, an audit trail with usernames, and a
  Chrome profile. Commit the transcript you are quoting, never the run.
* **The shell half is shellcheck-linted by name** in the `shellcheck` job, like everything else in
  `testing/`. The job runs from the repository root without `-x`, which is why the `source=`
  directives above the `. fixture.sh` lines are written repo-root-relative — otherwise the job
  fails on the directive instead of following it. The Python half has no *lint* gate, because this
  repo has no Python environment to put one in — but it does have a behavioural one, and the
  section above is why `pages.py --self-test` sits in `make verify` rather than in a README.
