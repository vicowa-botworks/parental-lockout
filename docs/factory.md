# The delivery factory

How this repository's issues get to a merged pull request without anyone remembering to ask.

Read this if you have ever found an open pull request with green checks that nobody had looked at,
or an issue that a closed-over branch had already fixed. Both happened here on 4 November 2026; the
numbers are in `factory/DECISIONS.md` (this log's own ids are `H-`; the repository's are `D-`).

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
(H-0017).

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
That is why the first install can point at the harness branch (`factory/DECISIONS.md` H-0009) and why
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

## What "dirty" means, because the test decides when the queue stops

Both halting call sites ask `factory_tree_dirt` (`factory/common.sh`), and it asks git for the
**content** of the tree, four ways: staged against `HEAD`, worktree against the index, unmerged index
entries, and untracked files that are not ignored. It does not ask `git status`, which is the command
whose answer is derived from the stat cache; and it checks the **exit status of every read**, because
"git could not tell" is not evidence of safety. Any failure returns UNKNOWN and UNKNOWN halts.

The asymmetry is why it is written that way. A false halt costs one pass, retried on the next tick; a
false clean lets a model write into somebody's half-finished work, which is not recoverable. Measured
on git 2.53.0, this is what the `status`-based test did with a tree whose repository could not be
opened -- an empty `.git`, or a `GIT_DIR`/`GIT_WORK_TREE` pair left exported by whatever ran before:
`git status --porcelain` exits 128 printing nothing, the suppression in `$(... 2>/dev/null)` makes
that indistinguishable from "nothing to report", and the pass proceeds in a tree git would not
describe. A third shape, a `core.excludesFile` pointing at a directory, is the same story only on some
filesystems, and the qualifier is narrower than "some filesystems": git `fstat`s the exclude file and
reads `st_size == 0` as an empty list, and a directory's `st_size` is the size of its own entry data,
so it depends on what the directory holds -- measured at `8015951`, btrfs 0 empty, 2 with a
one-character entry, 12 with a six-character one; tmpfs 40 empty and 60 holding either. So that typo
refuses the moment the mistyped directory contains anything, on btrfs included, and the swallow needs
an *empty* directory on a filesystem that gives empty directories no size -- which is what the
fixture's directory is. There the patterns are dropped silently, the operator's own ignored files come
back as work, and the halt stops the queue over work that is not work.
`factory/selftest.sh` measures which of the two the box is running on and asserts only that one, with
a note naming the filesystem when the refusal is unreachable. A stat-only difference, the report #228
came in with, was tried in six variants and does *not* reproduce on this git, which refreshes the
index in memory before reporting; the verdict no longer depends on that question either way.

The refresh is the only read here that writes, so it is worth saying plainly what it can and cannot do.
It rewrites `.git/index`'s stat entries and nothing else: measured on a stat-only difference the index
changed and the file's bytes did not, and measured on an unresolved merge it wrote nothing and settled
nothing -- three index stages before and after, `MERGE_HEAD` still present, its own report the word
`needs merge` at exit 1. It is not a destructive read and it cannot tidy a conflict away. What remains
unsafe is that it writes at all, into a `.git` that may belong to a person working in the same tree.
A held `.git/index.lock` is the one shape of "busy" that shows: the refresh exits 128 naming the lock
file while the other four reads and `git status` all answer 0, and UNKNOWN is the right answer to that,
because the writes that follow the verdict are real ones (`merge --no-edit`, and `merge --abort` when
it fails). One shape of busy is invisible to every read and is open work in #232: a directory at mode
000 inside the tree makes `ls-files --others` warn and exit 0, and the verdict says clean.

The five refusal checks are redundant on purpose -- any one of them refusing is enough to halt -- and
the redundancy is measured, arm by arm, rather than asserted: replacing one arm's refusal test with a
constant-false test and running the probe leaves 70 legs green for four of the five arms, 4 legs red
for the tracked read (the unreadable file is the one planted shape where exactly one read refuses),
5 red with four arms gone and 8 red with all five. Those four are not a gap the probe can close: every
planted shape that stops one of them stops another as well. What the legs had to stop doing was
accepting *any* non-clean answer -- the unreadable leg used to pass while the verdict degraded from
`its tracked files could not be read (git diff exited 128)` to `modified: keepme.txt (git said: …
Permission denied)`, and both halt, but the second one sends the operator to commit or discard a file
git could not open.

`factory/selftest.sh` is the probe. It builds a throwaway repository plus a linked worktree of it --
the second is the shape #228 was measured on -- plants one state per leg, and prints the weaker test's
verdict next to its own on every line, so a leg that has stopped discriminating is visible rather than
merely green. It is a step in `make verify`, and it refuses when that step is deleted.

The halt is one function too, and that is the whole of why it is testable: `factory_tree_halt`
(`factory/common.sh`) asks the verdict, and either returns 0 or writes `factory/HALT` and returns the
code for the path that called it -- `4` for a tick, `2` for the conflict pass. Both call sites are
`factory_tree_halt … || return $?` and nothing else. They used to each carry their own copy of the
predicate, and nothing executed either copy: flipping one to halt on a clean tree, or deleting it, left
every leg green. So the probe drives the guard over the same planted states and asserts the four things
a halt has to get right -- that it halts, the exit code it leaves, that `factory/HALT` exists with the
resume step and the action matching the finding ("commit it or discard it" is not advice anyone can take
in a tree git cannot read), and one journal line per path carrying that path's own verb. A call site
that drops the `|| return $?` is caught by the step as well, because the alternative is a pass that
asks the question and throws the answer away.

## Exit codes, because a timer has to tell these apart

`0` the pass is over · `3` the queue could not be read, so nothing was attempted · `4` a tick failed
· `7` another supervisor holds the lock · `8` halted.

`3` and `7` are the two that matter to monitoring: `3` means the token or the network is gone, and
`7` means two timers are pointing at the same repository.

## What it is not allowed to do

`factory/factory.sh` merges only what the API *still* says is approved, mergeable, clean and green,
at the moment of the merge. The agent denies itself `gh pr merge`, `gh pr review`, every force push,
every recursive delete, pushing `main`, `git tag` with an argument, `gh release create`/`delete`/`edit`,
`gh api`, `go get`, `systemctl`, `git worktree remove`/`move`, and the `question` tool. Those denies are in
`.opencode/opencode.jsonc`, and they are the real boundary: `--auto` approves anything not explicitly
denied, so a permission left at `ask` in a headless run is a stall, not a guard.

Those denies bind because of *where* they are written and *how* they are spelled. A tool's rule is
either a bare `allow`/`ask`/`deny` keyed by the tool's name, or -- for `bash` -- a table of command
patterns. A pattern written at the top level is read as an **action**, and no tool reports an action
named after a shell command, so `--auto` approves the command anyway: measured with the same entry both
ways, flat `gh pr merge 999999` ran, nested under `bash` it returned `Permission denied: shell`. Check
the keying of any copy of this harness with `opencode debug agents --standalone` -- every rule must
come back as `"action": "shell"` with the pattern in `resource` (H-0013). Without `--standalone` the
CLI answers from a running server's startup config, which is a different question.

Spelling matters the same way, and getting it wrong is quieter. The matcher globs the **whole** parsed
command line, so a pattern that opens on a literal only matches a command that opens on that literal.
Measured on v2.2.5 at `869564a`: 17 bash rules, **7 of them inert** for exactly that reason --
`gh pr merge*` allowed `gh -R vicowa-botworks/parental-lockout pr merge 218`, which is the form `gh`'s
own `--repo` flag produces. So a denial whose program can *open* a command needs **both** shapes: the
`* ` form reaches the flag-inserted command and the literal form reaches the bare one, and neither
covers the other. Deleting the literals to fix round three is what re-opened the bare ones -- at
`37120a8`, `rm -rf`, `systemctl --user stop <the timer>` and `go get` went from denied to allowed in
the commit written to repair the list. Where a flag can sit between the program and the subcommand the
wildcard starts at the subcommand: `* pr merge*`, not `* gh pr merge*`; that is also why the `git` and
`gh` rules need no literal twin, since their patterns key on a subcommand and a command always has
something before it. Two families are deliberately narrower than their broad form: `* tag *` needs the
argument so `git tag` can still list, and `* release create*`/`delete*`/`edit*` leave `gh release list`
readable -- a deny that breaks a legitimate read teaches the next tick to route around denies.

`python3 scripts/check-denies.py`, which `make verify` runs as *agent deny list*, is what stops this
from being a matter of reading carefully. It resolves the agent's rules through the CLI, matches whole
commands against them, and asserts both directions: every denial refused in both spellings, and a
short allow-list (`git describe --tags --always`, `git tag`, `gh release list`, `git push origin feat`)
still allowed, because an over-reaching list shuts the harness down and a check that only asserts
denials passes a config that forbids `git commit`. It also refuses a config pattern that no command
here witnesses, and a resolved pattern the config does not contain. It asks the CLI with
`--standalone`, and that is not a detail: without it the CLI answers from a *running server's* startup
config, and measured on 6 Nov, replacing the file with the broken list and running the check in the
same breath still returned the good one -- 23 rules, green. A check that reads yesterday's ruleset is
worse than no check, because it prints a number. It skips, loudly, where opencode is not installed
(CI), and `CHECK_DENIES_REQUIRED=1` makes a skip a failure.

Two limits stay on the record, because both are the kind that gets forgotten. First, this reads the
resolved ruleset, not a decision: `opencode run --auto --agent factory` has never executed a command
inside these rules on either machine involved in #217 -- it dies with `UnknownError` on the reviewing
box -- so the matcher's semantics remain an assumption shared by the probe and the fix, and one landed
live tick would settle more than another round of pattern-shaping. Second, a whole-line glob cannot
tell a command from a quote, so these patterns match **inside arguments**: `git rm -r --cached legacy`
is denied, and so is `git commit -m "explain why * rm -r* is denied"`. That is expected behaviour, not
a bug to route around -- the response is to reword the message, or to write the note in a file with
`write` rather than in a `-m` string. Narrowing the patterns further is not available: `* rm -r ` with
a trailing space matches the quoted form just as well.

Keying these patterns under `bash` has a second consequence: the CLI's command scanner now parses every
command the agent types, so a command shape the scanner cannot analyse is refused outright rather than
passing unmatched. That is the price of a deny list that actually evaluates, and the skill states the
command shapes that keep a tick on the right side of it.

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
