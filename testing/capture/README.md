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
through the login form in the browser, measures, and writes its PNG to `runs/<name>/`. Add
`--out DIR` to write somewhere else; `--out ../../docs/screenshots/360` is how you re-capture the
committed artifacts themselves, and it is deliberately something you have to type.

Needs `google-chrome` and a `plock` binary. Needs **no** root, **no** desktop session and **no**
human: the fixture points the lock backend at the command backend with `/bin/true`, so a capture
cannot lock the machine it runs on. That is the whole reason a screenshot can be taken by a script
at all — the alternative is a test that locks the operator's screen.

Run them as the desktop user, not through sudo. The two POST fixtures capture the result page for
`$(id -un)`, because that page lists the sessions an account holds on this machine and a
config-only name holds none; under sudo the target becomes `root`, which holds no session, and the
driver's "no session rows" refusal fires. That refusal is the correct answer, not a bug to work
around.

## What each driver refuses

A screenshot is evidence only because the thing that took it refuses to take it in the wrong
state. The refusals are the artifact's whole value, so they are listed here and they live in
`cdp.py` where more than one page needs them.

| driver | refuses when |
|---|---|
| `pages.py` | a page answers a different URL than the one it was sent to; the page has no table or no cells (an empty fixture fits at any width); any cell lacks `data-label`; and at the end, if any page overflows |
| `lock.py` | the dashboard has no `lock-now` form, or no submit button, or no CSRF token with a value, or the form is not `method=post`; the click does not navigate; the result page has no banner sentence or no session rows; a cell lacks `data-label`; **a row claims the lock happened while its own Result cell says otherwise** (#109); **the Detail cell's sentence runs into the self-session note** (#108) |
| `testlock.py` | the same form and click refusals for `test-lock`; the page does not say *Nothing was locked*; fewer than two tables or no backend rows; the #108 seam rule; and **the audit trail contains anything but `lock.probed`** — checked *before* the PNG is written, because a refusal that fires after the file exists has already produced the thing it was meant to withhold |

The click is the point of the two POST drivers. `/users/<name>/lock-now` has no URL you may
navigate to — it is the response to a POST — so the only honest capture signs in, finds the form
the shipped template put on the dashboard, checks it carries a real CSRF field, and **clicks its
real button**. A hand-built request body would prove nothing about what the template sends, and a
capture of a CSRF refusal would photograph a refusal.

## The negative control, and why a refusal nobody watched fire proves nothing

An assertion never run against the alternative is unproven; a refusal never watched firing is the
same failure wearing a different hat. So each rule above has been shown to fire:

```sh
# 1. Build a binary with the clause under test removed.
#    internal/web/celltext.go:  return trimmed + "."   ->   return trimmed
go build -o /tmp/plock-broken ./cmd/plock

# 2. Point the capture at it. It must refuse, and it must not touch the previous PNG.
PLOCK_BIN=/tmp/plock-broken ./lock-setup.sh && PLOCK_BIN=/tmp/plock-broken ./lock.py
#   the lock result page: the detail runs into the self-session note
#   ('he request but cannot confirm it' then 'This is the session the daemon runs');
#   the page still renders a run-on, so this capture is the pre-fix page
#   exit 1, and runs/lock/lock.png is not written
```

That run is not hypothetical. It is what this branch does every time it is checked out on `main`,
because `main` does not have the fix yet: the driver refuses and writes nothing. The same driver,
pointed at a binary built from #113, reports `FITS at 360 px` and writes a PNG whose md5 is
`473451a946ff66ef631de40245652ca0` — byte-identical to the `lock.png` #113 commits.

## Which artifacts reproduce byte for byte, and which cannot

Measured on this machine by running each driver and comparing md5 against the committed file:

| artifact | in-repo driver reproduces it? | why |
|---|---|---|
| `users.png` | **yes**, identical | |
| `profiles.png` | **yes**, identical | |
| `testlock.png` | **yes**, identical | the probe page's session rows carry no lock detail, so its Detail cell is the self-session note alone — there is no seam, and the #108 rule skips a cell with nothing before the note rather than refusing a page that is correct |
| `lock.png` | **yes**, identical to the artifact #113 commits — and a **refusal**, not a different image, against a binary without the fix | the artifact and the fix ship together |
| `dashboard.png` | no | the page renders `until <clock>` / `next at <clock>` from `EndsAtClock` and `NextLockAtClock`, so the image is a function of the time of day. Two runs a minute apart are identical; a run hours later is not |
| `audit.png` | **no, by construction** | signing in appends an `auth.login` line to the very trail the page photographs, so every capture adds a row. Two runs against the *same* fixture differ |

So `audit.png` and `dashboard.png` are one-shot photographs, and the honest claim about them is
the driver's report — `scrollWidth`, `innerWidth`, the cell census, `FITS` — not the pixels. That
is worth saying out loud, because a PR that links `audit.png` is linking an image nobody can
regenerate, and the thing a reviewer can re-run is the script.

## House rules

* **A capture is evidence when the driver's transcript is pasted into the PR, and the PNG is
  quoted beside it.** The transcript is the part that says what was refused.
* **Never write into `docs/screenshots/360/` except with an explicit `--out`.** Overwriting a
  committed artifact is exactly the state a stale screenshot can fake: the file is current, the
  page it depicts is not.
* **A changed page makes its committed artifact stale by construction.** If a PR changes what a
  captured page renders, that PR re-captures it in the same branch — #103 shipped a PNG depicting
  the defect #108 later found, and the repo's own acceptance evidence argued against the fix.
* **`cdp.py` holds the shared refusals; a driver holds the checks about its own page.** The cell
  that #108 was about was copied between two pages, and a copied cell is a copied rule: the check
  that exists on one page is the check that misses the regression on the other.
* **`runs/` is ignored.** It holds a config with an admin credential hash, an audit trail with
  usernames, and a Chrome profile. Commit the transcript you are quoting, never the run.
* **The shell half is shellcheck-linted by name** in the `shellcheck` job, like everything else
  in `testing/`. The Python half has no lint gate, because this repo has no Python environment to
  put one in — `scripts/check-deps.sh` keeps the Go side at zero dependencies and the Python side
  is two tools and these drivers, run by whoever is holding the keyboard.
