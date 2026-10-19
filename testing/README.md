# testing/

Checks that cannot run in CI, kept in the repo so they survive the operator.

Everything else in this project is automated: `make verify` builds, vets, races,
lints the dependency rule and cross-compiles; `scripts/mutate.py` runs the
mutation battery. Those gates need nothing but a checkout. What lives here needs
something a runner cannot have — a real desktop session, root, and a human
willing to type a password into a screen the test is deliberately locking.

That distinction is the whole admission rule. A check belongs here when it is
**manual, needs a real session, and is reproducible enough that somebody else
could run it from a clone**. A check that needs none of those belongs in
`go test`, and a check that needs only a checkout belongs in `make verify`.

There is a second category, and `capture/` is it: **reproducible from a clone,
needs a browser, needs no root and nobody standing there.** The 360 px acceptance
photographs are the checks in that category. They are not manual — a script takes
them — and they need no root and no unlocked screen, because their fixture points
the lock backend at `/bin/true` so a capture cannot lock the machine it runs on.
What they need is a browser, which a runner does not have, so they cannot be CI's
job; and what they produce is the artifact an acceptance criterion is actually
satisfied by, so they cannot live in a scratch directory either. The two POST
captures additionally need the account they capture to hold a real desktop
session, because that is what the page lists — which is a property of the page,
not of the harness, and the driver refuses rather than photographing an empty
table. `capture/README.md` says why the second category is admitted on the same
terms as the first.

## Why this folder exists at all

`docs/security.md` §9 ranks the end-to-end run as the project's largest residual
risk — *"The single most valuable thing that could happen to this project is
someone running it on the target desktop."* Issue #13 asked for that harness as
`scripts/e2e.sh`. It was written, run once by hand, and left in a scratch
directory outside the repository, which is the worst of the available outcomes:
the one check that outranks every unit test in the tree was invisible to a clone,
unnamed by any document, and one `rm -rf` away from never having existed.

`scripts/` is not its home either. Everything in `scripts/` runs unattended on
every push; these do not, and mixing them teaches the next reader that a red tick
in CI has something to do with what happens here.

## What is here

| path | what it does | needs |
|---|---|---|
| `e2e/e2e.sh` | the full loop against the real session: configure, serve, watch it lock, unlock by hand, watch it re-lock, read the audit trail | root, a desktop session, your password |
| `e2e/forge-hint.sh` | the #29 attack as a check: writes `LockedHint` with no privilege and insists enforcement does not believe it | nothing for the first half; root for the second |
| `e2e/lib.sh` | the two questions both scripts ask about the machine — "is a locker running for this uid" and "which session is the desktop" — in one place | sourced, never run |
| `capture/pages.py` + `pages-setup.sh` | measures the four GET-rendered admin pages at a 360 px viewport and photographs them | a browser, no root |
| `capture/lock.py` + `lock-setup.sh` | clicks the dashboard's real `Lock now` button so the POST carries the real CSRF token, then photographs the result page — and refuses to, if the page is not the verdict page | a browser, no root; the account must hold a session |
| `capture/testlock.py` + `testlock-setup.sh` | the same for `Check backends`, with the audit trail checked before the PNG is written | a browser, no root; the account must hold a session |
| `capture/cdp.py` | the plumbing all three need, and the refusals more than one page shares, in one place | imported, never run |

Read `e2e/README.md` before running either e2e script. `e2e.sh` locks your screen on
purpose, repeatedly, and there is no way to test that from a runner.

Read `capture/README.md` before quoting a screenshot in a PR. It lists what each driver
refuses, the negative control that proves those refusals fire, and which of the six
committed artifacts can be reproduced byte for byte — two of them cannot, and knowing
which is the difference between citing evidence and citing a picture.

## House rules

* **Nothing here runs in CI, and nothing here may be *claimed* by CI.** A script
  in this folder is evidence only when its transcript is pasted into a PR.
* **Every shell script here is shellcheck-linted** by the `shellcheck` job, which
  names shipped scripts explicitly. Add new ones to that list; a script nobody
  lints drifts. `lib.sh` is on the list even though it is only ever sourced.
* **Output goes under `<folder>/runs/`**, which is ignored. Commit the transcript
  you are quoting into the PR, never into the tree — a transcript contains a
  username, a hostname and a session id, and `capture/runs/` additionally holds a
  config carrying an admin credential hash and a `fixture.env` holding the
  fixture's password in cleartext.
* **Restore what you changed, including on Ctrl-C.** Both scripts back up before
  they write and install `EXIT` and `INT` handlers that put it back. Treat that as
  behaviour to preserve, not a description of how careful the scripts are: `e2e.sh`
  refuses to run at all if the backup of `/etc/plock` did not copy cleanly, because
  its cleanup used to infer "there was no config here" from a missing backup and
  delete a config it had failed to save. `forge-hint.sh` restores the hint from
  both handlers, because the window between forging and restoring is 20 seconds
  wide by default and an interrupt in it used to leave a session claiming it was
  locked.
