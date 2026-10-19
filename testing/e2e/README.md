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

Of those two branches, the 2026-10-18 transcript exercised the human one: a person at the
console unlocked, and the loop re-locked within the poll. The `INCONCLUSIVE` branch has
not been run — it is the branch that *declines* to produce a verdict, so nothing was at
stake in it and no transcript quotes it. Say so if you cite it.

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

## What the harness accepts as a locker, and why it is not the daemon's answer

`lib.sh:locker_pid` requires three things of a process before it counts, and they are
`internal/platform/locker.go`'s three requirements, kept in the same order:

1. **the process runs as the uid whose lock is being judged.** A harness that walks every
   process on the box is satisfied by a *sibling account's* locker, and on the
   two-account machine this tool exists for that attributes one child's lockout to
   another.
2. **the executable is the expected program, in a file and directory that uid cannot
   write.** `argv[0]` is whatever the caller passed to `execve`, and `/proc/<pid>/exe`
   only says which file is running — so without the ownership test an impostor needs a
   binary merely *named* `kscreenlocker_greet`, which the session user can put anywhere
   they own. A file they own, or one that is group- or world-writable, is a file they
   chose the bytes of.
3. **for the KDE locker, its parent is `ksmserver`, passing requirement 2 as well.** The
   locker is a child of the process that owns `org.freedesktop.ScreenSaver` and answers
   the `Lock` call, so this makes the check about a process tree rather than one name:
   two `execve` calls as the session user produce a tree whose names are both lies. The
   GNOME locker has no measured parent, so requirement 3 is not applied to it and the
   transcript says so instead of implying the same strength.

That is a copy of a Go function, and deliberately so. The obvious alternative is to ask
the daemon, which already implements the rule — but then the harness's *independent*
signal comes from the code under test, a buggy corroborator agrees with itself, and the
transcript still says PASS. The harness is the reading a reviewer can check without
trusting the thing it is reviewing. What must not drift is the rule, so it lives in one
file with the Go function named as its source.

The same reasoning picks the session. `loginctl list-sessions` prints a literal `-` where
a session has no tty, so "has a tty" is two tests (`$7 != ""` **and** `$7 != "-"`), not
one: measured on the review host, a filter that only excluded the empty string matched all
four user sessions, none of which had a tty, three of which were remote `ssh` sessions.
For the unprivileged forge that is not cosmetic — pick a remote session instead of the
desktop one and the script forges a hint on a session that was never the subject, and the
"no locker is running" it then reports is trivially true there.

## Output

`runs/` is ignored. A transcript names a user, a host and a session id — quote it in
the PR, do not commit it.
