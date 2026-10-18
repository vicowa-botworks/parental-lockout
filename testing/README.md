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

Read `e2e/README.md` before running either. `e2e.sh` locks your screen on
purpose, repeatedly, and there is no way to test that from a runner.

## House rules

* **Nothing here runs in CI, and nothing here may be *claimed* by CI.** A script
  in this folder is evidence only when its transcript is pasted into a PR.
* **Both scripts are shellcheck-linted** by the `shellcheck` job, which names
  shipped scripts explicitly. Add new ones to that list; a script nobody lints
  drifts.
* **Output goes under `e2e/runs/`**, which is ignored. Commit the transcript you
  are quoting into the PR, never into the tree — a transcript contains a
  username, a hostname and a session id.
* **Restore what you changed.** Both scripts back up before they write and
  restore on exit, including on Ctrl-C. If a script leaves `/etc/plock` behind
  when there was none, that is a bug in the script.
