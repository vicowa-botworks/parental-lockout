# End-to-end checks

Two scripts, both manual, neither of which CI can run. Read the one you are about to
execute; each says what it costs at the top.

| script | what it answers | needs root? | locks your screen? |
|---|---|---|---|
| `e2e.sh` | does the whole loop work against a real desktop session — configure, serve, lock, unlock by hand, re-lock, audit trail | yes | yes, repeatedly, on purpose |
| `forge-hint.sh` | is `LockedHint` still writable by the controlled user, and does enforcement ignore it | only for the second half | only in root mode |

## `e2e.sh`

```sh
make build                      # leaves ./plock at the repo root
sudo testing/e2e/e2e.sh         # targets the desktop user; or pass a username
```

It writes `/etc/plock/config.json` (backed up first, restored on exit, including on
Ctrl-C), starts a daemon with a 5-second poll and an all-day rule, and waits for you
to unlock by hand so it can watch the loop lock the session again.

The transcript at `testing/e2e/runs/<timestamp>/transcript.txt` is the deliverable.
Issue #13's first acceptance criterion is that transcript pasted into a PR, and
`docs/security.md` §9 keeps the e2e run as the project's largest residual risk until
one exists.

Two things it will tell you honestly rather than gloss:

* **`INCONCLUSIVE`** if there is no controlling terminal. Step 5 needs a human to
  unlock; run over ssh without a tty, or from a runner, and it says the re-lock half
  was not tested instead of reporting that the daemon failed to re-lock.
* **`hint=yes but NO locker`** while it is waiting for a re-lock. That is not a
  timing flake — it is a forged hint, and it is the thing `forge-hint.sh` goes on to
  demonstrate on purpose.

## `forge-hint.sh`

```sh
testing/e2e/forge-hint.sh              # as the desktop user: locks nothing
sudo testing/e2e/forge-hint.sh casey   # as root, daemon enforcing: locks the target
```

The unprivileged half is the one to run on a whim. It flips your own session's
`LockedHint`, reads it back, shows that no locker is running while the hint claims a
lock, and puts the value back. That is issue #29's attack, and the entire enforcement
design — re-assert every tick, judge a lock by two signals — rests on it still being
true. It is a property of the host, so it has to be re-measured rather than trusted.

The root half proves the other side: it forges the hint as the target user and insists
the loop keeps writing `lock.asserted` while the session's own hint says it is already
locked. A loop that believed the hint would go quiet, which is the one-line-autostart
defeat #29 was filed for.

## Why the locker is found through `/proc/<pid>/exe`

Both scripts need a second signal besides the hint, and the obvious ways to get one are
both wrong on this project's target:

* `pgrep -f 'kscreenlocker_greet|gsd-screensaver'` matches **command lines**, so any
  shell whose arguments merely contain the pattern reports a locker that does not
  exist. Measured on the development host: two matches, both shells. A signal that can
  be fed a string is one step from the hint it was written to check.
* `pgrep -x kscreenlocker_greet` can never match. It compares against `comm`, which
  the kernel holds to 15 characters; the binary is 19, and pgrep warns you itself.
  (`ps -C` does match — the 2026-10-15 transcript shows it listing the locker — but it
  answers with a table, and the harness needs the pid it is about to describe.)

`/proc/<pid>/exe` is the executable the kernel actually started. Nothing a user types
makes it point at a locker that is not running.

## Output

`runs/` is ignored. A transcript names a user, a host and a session id — quote it in
the PR, do not commit it.
