# The delivery factory

How this repository's issues get to a merged pull request without anyone remembering to ask.

Read this if you have ever found an open pull request with green checks that nobody had looked at,
or an issue that a closed-over branch had already fixed. Both happened here on 4 November 2026; the
numbers are in `DECISIONS.md`.

## The one idea

**The loop is not in the agent.** A model session is a disposable worker; the thing that owns the
queue is a shell script started by a timer. When the session dies, one unit of work is lost and the
next activation re-derives the whole world from GitHub and git. Every design where the loop lives
inside a session has the same failure: the session ends, and the queue stops.

```
systemd timer (or `while :; do factory.sh; sleep 600; done`)
   └── factory/factory.sh            ← owns the loop, holds the lock, budgets the pass
         ├── factory/triage.sh       ← what needs something, read from the API on this run
         ├── mechanical actions      ← request a review, merge, merge main in, post a notice
         └── opencode run --auto --agent factory     ← one unit, one worktree, one ceiling
```

## Try it without touching anything

```bash
factory/factory.sh --dry-run            # every action printed, nothing written
factory/triage.sh --table               # just the verdict, for a human
factory/triage.sh --pr 206              # why this one is sitting there
factory/factory.sh --status             # halt state and the ledger
```

`--dry-run` is a property of the whole script, not of one call site: no GitHub write, no git write, no
model, and no ledger. It is the mode to reach for first, and it is what found three of the bugs in this
directory (`gh api` returning a `{"message":…}` 404 body as if it were data, a `.git` suffix leaking
into the repository name, and a `case` arm eaten by an unbalanced quote).

The ledger had to be added to that list because it was the exception: the two writers in `common.sh`
checked nothing, so a dry run stamped `first-seen`, `stuck-since`, `last-notice` and `conflict-at`
while printing `DRY-RUN` for everything else. A ledger file is not bookkeeping, it is a timestamp with
consequences -- `first-seen` decides queue order, `stuck-since` starts the escalation clock,
`last-notice` throttles comments at a maintainer -- so one rehearsed pass made the next real pass quote
a wait that never happened and stay quiet at somebody who had never been contacted. `log_event` is
deliberately *not* suppressed: a dry run you cannot audit afterwards is a dry run nobody trusts
(D-0017).

## What each class means

`triage.sh` prints one row per thing that needs something; `factory.sh` acts on the class.

| class | what it means | who acts |
|---|---|---|
| `merge_now` | approved, `MERGEABLE`, `CLEAN`, every check passed | supervisor, in bash, after re-reading all four from the API |
| `needs_reviewer` | no review **and** `/requested_reviewers` is empty | supervisor: request one, at most once per `FACTORY_REVIEW_POKES` |
| `conflict` | `CONFLICTING/DIRTY` | supervisor: `git merge origin/main`; a tick only if that conflicts |
| `address_review` | newest `CHANGES_REQUESTED` is newer than the head commit | a tick |
| `checks_failing` | a check concluded failure | a tick |
| `unclaimed` | open issue no open pull request references | a tick |
| `branch_without_pr` | commits ahead of `main`, no open pull request | a tick, to adopt or refuse the stranded work |
| `awaiting_review`, `awaiting_merge`, `checks_pending`, `draft` | a wait, not a failure | nothing, except the throttle and the daily notice |

Two of these are the reason the whole thing was worth building. `needs_reviewer` exists because
`gh pr list`'s `reviewRequests` lists only *outstanding* requests, so it cannot distinguish "never
asked" from "asked and answered" -- measured, four pull requests had an empty list *and* an empty
`/pulls/N/requested_reviewers`, three days after opening, with every check green. `branch_without_pr`
exists because a branch with commits and no pull request is invisible to every queue that reads
issues and pull requests; the first run of this script found five, one of them seven commits deep.

## Installing it

Generate the units; do not copy them out of a README. `factory/install-timer.sh` asks the machine
where `gh` and `opencode` actually live, derives the unit's `TimeoutStopSec` from `FACTORY_BUDGET`,
and refuses to install anything that would fail quietly:

```bash
factory/install-timer.sh --install --dry-run     # prints both units and the systemctl commands
factory/install-timer.sh --install
journalctl --user -u parental-lockout-factory.service -n 50
factory/install-timer.sh --remove                # the whole thing, reversibly
```

It installs into `~/.config/systemd/user`, so nothing needs root. What it writes, and why each line
is there:

```ini
# ~/.config/systemd/user/parental-lockout-factory.service
[Unit]
Description=parental-lockout-factory delivery supervisor (one pass)
Documentation=file:/home/rodney/prj/parental-lockout/docs/factory.md

[Service]
Type=oneshot
WorkingDirectory=/home/rodney/prj/parental-lockout
ExecStart=/home/rodney/prj/parental-lockout/factory/factory.sh
# Every binary `make verify` shells out to, not just the two the installer needs. A unit reads no
# shell profile, and here `go` and `gofmt` live in ~/.local/bin -- a tick whose PATH lacks them cannot
# pass the one gate it is required to pass, and finds that out 40 minutes into a run.
Environment=PATH=/home/rodney/.local/bin:/home/rodney/.opencode/bin:/usr/bin:/usr/local/bin:/usr/bin:/bin
Environment=FACTORY_REVIEWER=hermesvicowa
# The project the CLI works in -- agent, deny rules, skills -- comes from $PWD, not from
# WorkingDirectory. A unit exports a cwd and no PWD, so without this line every tick opens a session
# in the background server's home directory and dies on `Agent not found: "factory"`.
Environment=PWD=/home/rodney/prj/parental-lockout
# The same number twice, both wider than FACTORY_BUDGET. TimeoutStopSec keeps systemd from killing a
# tick mid-commit (a killed tick leaves a dirty worktree, and the next pass halts on it).
# TimeoutStartSec is here because a oneshot has NO start timeout at all: systemd.service(5) says the
# default is "disabled by default" for that Type, measured as `TimeoutStartUSec=infinity` on a manager
# whose DefaultTimeoutStartSec is 1min30s. With none, a wedged pass holds the run lock forever and
# every later activation quietly does nothing.
TimeoutStopSec=5460
TimeoutStartSec=5460
Nice=10
IOSchedulingClass=best-effort
IOSchedulingPriority=7
```

No `After=`/`Wants=network-online.target`. That target does not exist in a user manager --
`systemctl --user status network-online.target` answers `Unit network-online.target could not be
found.`; it belongs to the system instance, where NetworkManager-wait-online.service pulls it in -- so
the pair ordered the pass after a unit that can never load and implied a network wait the pass does not
perform. It does not need one: `gh` unreachable is a visible `TRIAGE-FAILED` with rc=3 and nothing
attempted, and the next activation is ten minutes away.

```ini
# ~/.config/systemd/user/parental-lockout-factory.timer
[Unit]
Description=parental-lockout-factory delivery supervisor, every 10m

[Timer]
# OnUnitInactiveSec, not OnBootSec/OnCalendar: the next pass starts an interval after the previous
# one *finished*, so a long pass cannot stack a second supervisor behind it.
OnUnitInactiveSec=10m
RandomizedDelaySec=2m
AccuracySec=1m

[Install]
WantedBy=timers.target
```

Then:

```bash
systemctl --user list-timers 'parental-lockout-*'
journalctl --user -u parental-lockout-factory.service -n 50
```

Ten minutes is not the review cadence -- a unit waiting on a reviewer is re-poked every
`FACTORY_REVIEW_POKES=1800` regardless of how often the timer fires. The tight interval is what
makes a *newly* green pull request merge within minutes of its last check finishing. A wait-state
pass costs no model tokens: it is `gh`, `jq` and a few ledger files.

**What makes this run while you are not looking at the machine is linger, not `Persistent=`.** That
flag used to be in the timer above, with a paragraph claiming it re-ran a pass missed while the box was
off. systemd.timer(5): `Persistent=` "only has an effect on timers configured with `OnCalendar=`", and
this unit has no `OnCalendar=` -- the flag was inert and the paragraph described nothing. What actually
decides it is whether your *user manager* is running when nobody is logged in:

```bash
loginctl show-user "$USER" --property=Linger    # `no` means the timer stops when you log out
loginctl enable-linger "$USER"                  # the one command that makes an unattended timer real
```

`install-timer.sh --install` runs that first command and prints the second when the answer is `no`,
because the reading belongs to the machine rather than to this repository -- on the box this harness was
written on it was `no` for a day and then `yes`, with no change to any file here in between.

**A tick needs this branch merged; the mechanical half does not.** `opencode run --agent factory`
resolves the agent, its `deny` rules and this skill from the project the CLI runs in -- which is the
tree `$PWD` names, not the worktree the instruction tells the tick to deliver into -- and a worktree
cut from an unmerged `main` has none of them, so a tick can only read them from the tree the harness
was started in. Get `$PWD` wrong and the session opens in whatever directory the background server
was started from, where nothing is defined at all: measured 5 Nov, 152 passes in a row died inside
two seconds on `Error: Agent not found: "factory"`, each one recorded as a bare `exit=1`, because a
systemd unit exports a working directory and no `PWD`. Review requests, merges and conflict
resolution read only the API and git, so they work from any checkout.
That is why the first install can point at the harness branch (`DECISIONS.md` D-0009) and why
`install-timer.sh` refuses rather than installing a unit whose `ExecStart` path does not exist: a
timer that fires into 127 looks identical, from the outside, to a timer that is working.

## Configuration

Everything is an environment override with a default, in `factory/common.sh`.

| variable | default | what it decides |
|---|---|---|
| `FACTORY_REPO` | from `origin` | `owner/name`; set it when the remote is a fork you do not want to file against |
| `FACTORY_REVIEWER` | `hermesvicowa` | who a review is requested from |
| `FACTORY_TREES` | `../wt` | where per-unit worktrees live; deliberately not under `/tmp` |
| `FACTORY_TICK_TIMEOUT` | `2700` | one session's ceiling, seconds |
| `FACTORY_BUDGET` | `5400` | one pass's ceiling, so activations cannot overlap |
| `FACTORY_MAX_TICKS` | `3` | sessions one pass may start |
| `FACTORY_REVIEW_POKES` | `1800` | the review-poke floor, seconds |
| `FACTORY_ESCALATE_AFTER` | `86400` | how long in a non-progress state before one comment |
| `SHELLCHECK` etc. | — | see `make check-shell` |

## Stopping it

```bash
printf 'why, in one line\n' > factory/HALT      # stop
rm factory/HALT                                  # resume
```

The file is checked between units *and* by the agent at every step boundary. Between units is not
enough on its own: a tick can hold a worktree for 45 minutes, and inside that window the only thing
that can hear you is the agent. The supervisor also writes this file itself when it finds a dirty
worktree it does not own -- a person is working there, or a dead tick left a half-merged index, and
neither is a machine's to clean up. To stop it hard: `systemctl --user stop plock-factory.timer`.

## Exit codes, because a timer has to tell these apart

`0` the pass is over · `3` the queue could not be read, so nothing was attempted · `4` a tick failed
· `7` another supervisor holds the lock · `8` halted.

`3` and `7` are the two that matter to monitoring: `3` means the token or the network is gone, and
`7` means two timers are pointing at the same repository.

## What it is not allowed to do

`factory/factory.sh` merges only what the API *still* says is approved, mergeable, clean and green,
at the moment of the merge. The agent denies itself `gh pr merge`, `gh pr review`, every force push,
every delete, pushing `main`, `git tag`, `gh release`, `go get`, and `question`. Those denies are in
`.opencode/opencode.jsonc`, and they are the real boundary: `--auto` approves anything not explicitly
denied, so a permission left at `ask` in a headless run is a stall, not a guard.

Those denies bind because of *where* they are written. A tool's rule is either a bare
`allow`/`ask`/`deny` keyed by the tool's name, or -- for `bash` -- a table of command patterns. A
pattern written at the top level is read as an **action**, and no tool reports an action named after a
shell command, so `--auto` approves the command anyway: measured with the same entry both ways, flat
`gh pr merge 999999` ran, nested under `bash` it returned `Permission denied: shell`. Check the keying
of any copy of this harness with `opencode debug agents` -- every rule must come back as
`"action": "shell"` with the pattern in `resource` (D-0013). It is also a wildcard match on the whole
parsed command line rather than a prefix match, which is why the push rules read `* push --force*`:
`git -C <tree> push --force` is the shape this harness actually uses.

And it is a constraint on a cooperative agent, not a sandbox: it binds the command shapes it can
spell. The boundary against irreversibility is that a tick cannot merge at all, and the supervisor
re-reads all four gate fields from the API in the same call that merges.

Merging cannot be undone here anyway: `allow_squash_merge=false` and `allow_rebase_merge=false`, so
every merge is permanent history.

## Lifting it out

Nothing in `factory/` may name this repository. `FACTORY_REPO` comes from `origin`, the reviewer is
an override, the gate it runs (`make verify`) is whatever the repository's Makefile calls verify,
and the agent's deny list is generic except `go get`. That is the price of the second half of this
task: once this has survived a real queue, `factory/` moves to a shared location and each repository
keeps only its `.opencode/opencode.jsonc` and `DECISIONS.md`. If a line here ever needs this
repository's name to work, that is a defect in it, not a convenience.
