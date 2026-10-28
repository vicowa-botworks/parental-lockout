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
| `pages.py` | a page answers a different URL than it was sent to; **a table page** has no table or no cells (an empty fixture fits at any width); **a form page** has no POST form or no fields (the sibling check — a `/settings` that rendered only its navigation also fits at 360 px, and dropping the check instead would have committed that as evidence it fits); any cell lacks `data-label`; **the page overflows its viewport** — measured at every width in `WIDTHS`, refused per page per viewport, and before the PNG is written; **the audit page** carries no seeded `platform.error` row, or its hardest unbreakable token is under 60 characters (#140: a page that could not have overflowed is not width evidence, and a page whose pressure came from the machine rather than the fixture is not evidence about the stylesheet) |
| `lock.py` | the dashboard has no `lock-now` form, no submit button, no CSRF token with a value, or a method that is not `post`; the click does not navigate; the result page has no banner sentence or no session rows; a cell lacks `data-label`; a session row has no Result cell; **a row claims the lock happened while its own Result cell says otherwise** (#109); the gated clause is absent, meaning the self-session note never rendered; **the Detail cell's sentence runs into that note** (#108); the note appears in no cell at all, so the #108 rule would have examined nothing; **the click wrote no `lock.requested` line**; the page overflows |
| `testlock.py` | the same form and click refusals for `test-lock`; the page does not say *Nothing was locked*; fewer than two tables or no backend rows; the #108 seam rule and the same empty-rule guard; **the audit trail holds anything but `lock.probed`**; the page overflows |
| all three | the daemon the fixture started is not the process still serving its port; something already answers the DevTools endpoint before the browser is launched; `--out` resolves outside the repository, names a file, or names a directory holding PNGs without `--overwrite`; an unrecognised flag |

Three of those deserve explaining, because each was a hole found by attacking this code rather
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

## Which artifacts reproduce byte for byte, and which cannot

Measured by running each driver and comparing md5 against the committed file. The reproducible
ones were confirmed stable across two consecutive runs of the same fixture.

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

| artifact | does the in-repo driver reproduce the committed file? | why |
|---|---|---|
| `users.png` | **yes** | |
| `profiles.png` | **yes** | re-taken by #147, which is the change that moved it. The `740200cc…` this row quoted had already stopped matching the committed file before that PR landed, and nothing noticed -- which is #144. Moved again by #138, whose refusal page is the artifact below |
| `profiles-refused.png` | **yes** | new with #138: the same URL photographed AFTER a refused write, so one page carries two artifacts and the manifest has to answer "which template does `profiles-refused` belong to" -- a *state* of a page is not a file named after it, so `template_for` asks the driver which `Page` declared the artifact and falls back to the stem only for the ones that declare nothing (`lock`, `testlock`). Reproduced byte-identical on the run that recorded the manifest |
| `settings.png` | **yes** | new with the settings page. It is the only page in the set refused on a *form* rather than on rows (`cdp.require_form`), because a page with no table would otherwise have been refused as an empty fixture -- and the lazy repair, dropping the check, would have let a /settings that rendered nothing but its navigation be committed as evidence that it fits. Re-taken by the transport form (#161), which took the page from `forms=3 fields=9` at 2069 px to `forms=4 fields=15` at 3511 px; the previous hash `42d47d9b…` is what the file was before that form, and the row is the only place that claim lives |
| `testlock.png` | **yes, on an account with one session** | the probe page's session Detail cell is the self-session note alone when the account's sessions are lockable, so there is no seam and the #108 rule has nothing to terminate; on a machine whose sessions are not lockable the cell carries a reason and the rule applies — which on `main` means a refusal, not a different image. **The page photographs the account's session census**, so the image is good for the machine it was taken on: measured 27 Oct on an account holding four lockable sessions, the driver ran green (`tables=2 backendRows=1 cells=14`, `FITS`, `note in 4 Detail cell(s), 0 with a sentence in front of it`) and produced `a8617a2b…`. That is a faithful capture of a different page, not a broken driver |
| `lock.png` | **yes, on an account with one session** | only since #113 merged. On the branch that added these drivers, before that merge, the committed file was the pre-fix image and the driver *refused* to produce it — which is the same fact pointing the other way, and the reason this row used to read "no". Same census caveat: on the four-session account above it ran green (`rows=4 cells=12`, banner `rodney was not locked.`, flag `4 of 4 sessions were asked to lock`) and produced `4fc93d11…` |
| `dashboard.png` | **no** | the page renders `until <clock>` / `next at <clock>` from `EndsAtClock` and `NextLockAtClock`, so the image is a function of the time of day. Two runs minutes apart agree; a run hours later does not. **A second host dependency, measured 28 Oct:** the Sessions cell renders whatever `Sessions()` returned when the frame was taken, and that is not fixture state either. The committed frame said `session list unavailable`; three fresh captures on the same host say `no desktop session`, same dimensions, different pixels. It is not #170 resurfacing — `git merge-base --is-ancestor a404b23 a889d0d8` says the polymorphic-`since` fix was already in the tree the old frame came from — and the two obvious host candidates were each removed from the driver's environment and ruled out: `DBUS_SESSION_BUS_ADDRESS` unset and `XDG_RUNTIME_DIR` unset both still render `no desktop session`. So the cell reports the machine, the cause is still open, and `manifest.py check` calls both frames current because `/`'s declared sources are `dashboard.tmpl` and `page.tmpl` and neither moves when the platform layer changes |
| `audit.png` | **no, by construction** | signing in appends an `auth.success` row to the very trail the page photographs, so every capture adds a row and two runs of one fixture cannot agree. The height that produces is a **run counter**, not a page measurement: the committed file had reached `360x15004` while a fixture built from nothing rendered **2936 px** with `cells=40` (measured 26 Oct on `main` at `1684b29`, and again from the transport branch). So the artifact was a function of how many times anyone has run a driver on their own machine, and 15004 px is already ~92% of Chromium's 16 384 px capture surface -- #144 carries the mechanism, the row census, and the proposed bound. **The counter is now measured rather than inferred:** on #178's head, run 1 on a fresh fixture gave `height=3056 cells=40` and run 2 on the same fixture gave `height=3310 cells=45` with `wc -l` agreeing at 9 rows, so ~254 px per appended row. #178 re-took the file at `360x3056` -- 7 seeded events plus the one sign-in, which is the shortest frame this page can produce and the first time the committed bytes came from a fresh fixture. That fixes the artifact and nothing about the mechanism: nothing refuses to write a frame off a trail that has already been photographed, so the next accidental second run re-commits a taller page and this row still says **no** |

So six of the eight can be regenerated and two cannot — and two of those six, `lock.png` and
`testlock.png`, reproduce only on an account holding the number of sessions the capture had,
because those two pages photograph the session census. The honest claim about an artifact is
therefore usually the driver's report — `scrollWidth`, `innerWidth`, the cell census, `FITS` —
not the pixels. That is worth saying out loud, because a PR that links `audit.png` is linking an
image nobody can regenerate, and the thing a reviewer can re-run is the script.

## What the audit fixture is not allowed to contain

`audit.png` is the one artifact here that a run cannot reproduce, so it is also the one where a
fabricated row would survive review: nobody can regenerate it and compare. It has therefore been
wrong twice, both times in the same way — a kind the fixture author assumed the product emits.
`auth.login` was caught in review; `config.changed` was caught later, and by then the picture had
already been committed.

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

The timestamps carry the machine's own UTC offset for the same reason the daemon's do: the page
prints each row's zone, so a trail stamped `Z` renders `15:48:17 UTC` beside `11:52:18 EDT` in one
column.

## Re-capturing the committed artifacts

```sh
./pages.py --out docs/screenshots/360 --overwrite
```

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
a successful capture, which is why the manifest and the PNGs land in the same commit as each other.

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
{"schema": "capture-manifest/1", "viewport": 360, "head": "a889d0d8b…",
 "captured_at": "2026-10-28T01:55:37Z",
 "pages": [{"page": "/profiles", "png": "profiles.png", "png_sha256": "1f711bea…",
            "sources": ["internal/web/templates/profiles.tmpl",
             "internal/web/templates/page.tmpl", "internal/web/static/app.css",
             "internal/web/static/app.js"],
            "sources_rev": "0a42139d…"}]}
```

and `make verify` runs `manifest.py check`, which refuses when:

- a PNG has no entry, or an entry has no PNG — the manifest and the artifacts have come apart;
- a PNG's bytes are not what the manifest recorded — it was replaced without re-running the driver;
- the newest commit over that page's `sources` is **newer than the newest commit over the PNG** —
  the page grew and the photograph did not;

The comparison is an ancestry test, not an equality against `sources_rev`, and the first version got
this wrong in the direction that costs an hour: it compared the manifest's own commit with the page's,
so a commit that changed a template *and* re-took its photograph — the one commit in which both are
true — was refused by the very change that made the photograph current, with a sentence asserting the
opposite. `git rev-list --count <png>..<source>` asks the history instead of asking the manifest, so
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

`./pages.py --self-test` runs 29 rules about what a refusal does. It starts no browser, spawns no
daemon, writes no file, and costs **0.036 s** for its twenty-nine rules, so `make verify` runs it
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
