# testing/capture — the 360 px acceptance photographs

Six PNGs in `docs/screenshots/360/` are acceptance evidence for the criterion *"every page is
usable at 360 px with JavaScript off"*. They are not decoration and they are not hand-drawn:
each one is what a real browser reported at a real 360 px viewport, and the scripts here are what
took them.

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

## What each driver refuses

A screenshot is evidence only because the thing that took it refuses to take it in the wrong
state. The refusals are the artifact's whole value, so they are listed here and the ones more than
one page needs live in `cdp.py`.

| driver | refuses when |
|---|---|
| `pages.py` | a page answers a different URL than it was sent to; the page has no table or no cells (an empty fixture fits at any width); any cell lacks `data-label`; **the page overflows 360 px** — checked before the PNG is written, per page |
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

Measured by running each driver and comparing md5 against the committed file. The three
reproducible ones were also confirmed stable across two consecutive runs of the same fixture.

| artifact | does the in-repo driver reproduce the committed file? | why |
|---|---|---|
| `users.png` | **yes** — `003ee3f8…` | |
| `profiles.png` | **yes** — `c31bdd55…` | |
| `testlock.png` | **yes** — `7f63a7e0…` | the probe page's session Detail cell is the self-session note alone when the account's sessions are lockable, so there is no seam and the #108 rule has nothing to terminate; on a machine whose sessions are not lockable the cell carries a reason and the rule applies — which on `main` means a refusal, not a different image |
| `lock.png` | **yes** — `473451a9…` | only since #113 merged. On the branch that added these drivers, before that merge, the committed file was the pre-fix image and the driver *refused* to produce it — which is the same fact pointing the other way, and the reason this row used to read "no" |
| `dashboard.png` | **no** | the page renders `until <clock>` / `next at <clock>` from `EndsAtClock` and `NextLockAtClock`, so the image is a function of the time of day. Two runs minutes apart agree; a run hours later does not |
| `audit.png` | **no, by construction** | signing in appends an `auth.success` row to the very trail the page photographs, so every capture adds a row and two runs of one fixture cannot agree |

So four of the six can be regenerated and two cannot. The honest claim about a one-shot artifact is
the driver's report — `scrollWidth`, `innerWidth`, the cell census, `FITS` — not the pixels. That
is worth saying out loud, because a PR that links `audit.png` is linking an image nobody can
regenerate, and the thing a reviewer can re-run is the script.

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
  `lock.asserted`, `lock.unconfirmed`, `lockout.entered`, `override.granted`, `auth.failure` and
  `auth.success` are all present in `plock`; `auth.login` and `config.changed` are both absent.
* **Detail text is copied from the emitter, not paraphrased** — `assertedDetail` and
  `unconfirmedDetail` in `internal/enforce/enforce.go`, the `"%s until %s by %s: %s"` format in
  `internal/web/override.go`, the throttle key in `internal/web/handlers.go`. Two shapes fall out
  of reading those and neither is guessable: `srv.Audit` sets `Actor` and `Detail` and never
  `Username`, so a row the web layer wrote renders `nobody` in the Account column; and only an
  *enabled* user has engine rows, so a lockout line for a disabled account is a state the engine
  never reaches.
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

`--overwrite` is a second flag on purpose. Writing this README and then testing its own
invocation replaced two committed artifacts with no error and no warning — overwriting is what the
flag is for. Replacing committed evidence should take two tokens, not a tab-completion.

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
  fails on the directive instead of following it. The Python half has no lint gate, because this
  repo has no Python environment to put one in.
