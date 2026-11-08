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
| `unclaimed` | open issue nothing claims -- no closing reference from an open pull request, and no claim verb in an open body | a tick |
| `delivered` | a merged pull request claims it, it landed on `main`, the issue has said nothing since, and no earlier closure was rejected | nothing: the supervisor logs it, and closes nothing |
| `branch_without_pr` | commits ahead of `main`, no open pull request | a tick, to adopt or refuse the stranded work |
| `awaiting_review`, `awaiting_merge`, `checks_pending`, `draft` | a wait, not a failure | nothing, except the throttle and the daily notice |

`delivered` is the fourth class the queue could not see, and it was measured twice in one day before
it was written (#235). Both claim channels read *open* pull requests only, so a merged one claims
nothing whatever its body says, and an issue whose fix is already on `main` goes back out as
`unclaimed`. #228's fix merged at 07:08:17Z and a tick was dispatched for #228 at 09:35Z on a branch
with nothing ahead of `origin/main` -- the ceiling went to re-deriving what `main` already had
(H-0050). Eight hours later #232's fix merged at 14:26:31Z as #234, whose only reference is `Refs
#232`, and the queue printed `issue 232 unclaimed` on a tree containing the fix.

The claim verb is not the verdict, and that is the finding rather than a caveat. Read against the live
open issues, PR #229's `Refs #199. Refs #204. Refs #207. Refs #213.` is one body pointing at four
issues, two of which its own "What this branch does not finish" section disclaims; #218's body carries
"Not here: #149's per-control commit policy", which is a cross-reference with no verb on it at all;
and #227's `Refs #222` is a merged fix to an issue that was closed on 6 Nov at 13:17:20Z and
**REOPENED** at 16:27:23Z with a corrected row. #222 and #232 are identical in every way the verb can
see -- one reference each, the same head-branch shape, silence on the issue after the merge -- and only
`stateReason` separates them. So the verdict is built out of five facts GitHub already holds, and
never out of a sentence in a body about what is finished: the body claims exactly one issue, it did not
use this repository's own spelling of partial work (`Part of #N` -- H-0020, and #1256 for why an
adjacent inflection of "close" is never a harmless way to say it),
the merge reached `FACTORY_BASE`, the issue has not been updated since the merge, and no closure of
that issue has already been rejected. The last of those is also the human's override in the other
direction: one comment on the issue puts it back in the queue for a tick.

Precedence is unchanged -- `delivered` is reached only where nothing open claims the issue -- and that
has a consequence worth writing down before the class surprises anybody, because the open-claim channel
drops an issue named *anywhere* in any open body, not only one a pull request claims. Running the
shipped `triage.sh` against GitHub at 15:22Z on 8 Nov, #232 and #222 printed no row at all: the first is
named by the pull request that fixes it, the second by draft #233, whose body says `Refs #220 and #222`.
The issue rows that pass printed were #237, #213 and #199, every one `unclaimed`, the latter two held
out of `delivered` by the claim count. So `delivered` means "the queue will not work on this, and here
is the merged pull request it is leaning on", and it does not mean "every finished ticket appears
here" -- an open body naming an issue hides it from both classes at once.

Both reads are bounded, and a bounded read says so rather than answering as if it had seen everything.
The GraphQL call asks for the first 100 open pull requests and the first 100 open issues, so each
connection is also asked for `pageInfo{hasNextPage}`; either one answering `true` skips the whole issue
half for that pass with a line naming the pagination bound, because a truncated issue page is not a
smaller verdict but a `stateReason` veto that went missing -- and a claim with its veto gone reads
`delivered`, which is the one error this class was built not to make. A missing `pageInfo` reads as
truncated, not as `false`. The other bound is the open-issue listing itself, at `--limit 100`: past it
an issue gets no row of any class, so the pass counts the rows it read and warns at the limit. Measured
against GitHub at 16:29Z on 8 Nov on eleven open pull requests and three open issues, both connections
answered `false` and neither warning printed; neither is due to go quiet on its own, and a queue that
has outgrown its read should say so in the log rather than look finished.

A claim is a verb plus **every** number still in the same list, which is a rule that only measurement
would have produced. #221's shipped body has `Refs #219, #220, #204` on one line with
`(Part of #219 only; the umbrella is not finished …)` eight lines above it, and open PR #233 says
`Refs #220 and #222`; a verb plus ONE number reads either line as a claim on its first issue, which on
the merged channel turns `Closes #10, #11` into a sole claim on #10 -- the bundle this class refuses,
arriving through a comma. Two sentences about two issues are still two sentences, because the second
number has neither a verb nor a list marker in front of it, and that is what stops "Not here: #149"
from retiring #149 out of the queue.

The read itself is one bulk call -- `gh pr list --state merged --limit $FACTORY_DELIVERED_WINDOW
--json number,body,mergedAt,baseRefName` -- because the per-PR shape the channels above use would cost
one call per merged pull request. The window is a bound on the read, not on the claim: an issue whose
fix landed outside it keeps the verdict it always had, `unclaimed`, and the row prints the window it
leaned on so a stale-looking verdict can be re-checked by raising it. It is a bound on the read and also
on what a single pass costs, because merged bodies are big: 100 of them were 632,601 bytes on 8 Nov,
which is why the payload is handed to `jq` on standard input rather than as an argument -- Linux
refuses any one argument over 128 KiB, and a scan that cannot run must say so rather than answer
`{}`, which is the same shape as "nothing has been delivered".

The supervisor's arm is one `log_event` line. It starts no tick, writes no ledger row, and closes
nothing -- closing is a human act on this repository's tracker, and the harm of a false `delivered` is
a ticket nobody looks at again, which is why the row is reported rather than closed or dropped. The
ledger is not consulted: it answers "was this ever worked" and keeps answering it after the work
merges, which makes it exactly the wrong thing to ask when the question is "does anything still claim
this?"

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
`factory/selftest.sh` measures which of the two it is running on and asserts only that one, with
a note naming the filesystem when the refusal is unreachable. The filesystem that decides it is the one
under `$TMPDIR`, where the fixture is built, and not the one the repository sits on: measured at
`f60c367` on one machine with only `TMPDIR` moved, the same commit reads 191 legs with `$TMPDIR` on
tmpfs and 190 with it on btrfs, one leg of the exclude pair appearing and disappearing with the arm.
The tally line therefore prints its own fixture root, filesystem and git version, so a leg count quoted
anywhere carries the world it was counted in (H-0057). A stat-only difference, the report #228
came in with, was tried in six variants and does *not* reproduce on this git, which refreshes the
index in memory before reporting; the verdict no longer depends on that question either way.

The refresh is the only read here that writes, so it is worth saying plainly what it can and cannot do.
It rewrites `.git/index`'s stat entries and nothing else: measured on a stat-only difference the index
changed and the file's bytes did not, and measured on an unresolved merge it wrote nothing and settled
nothing -- three index stages before and after, `MERGE_HEAD` still present, its own report the word
`needs merge` at exit 1. It is not a destructive read and it cannot tidy a conflict away. What remains
unsafe is that it writes at all, into a `.git` that may belong to a person working in the same tree.
A held `.git/index.lock` is the one shape of "busy" that shows in an exit status: the refresh exits 128
naming the lock file while the other four reads and `git status` all answer 0, and UNKNOWN is the right
answer to that, because the writes that follow the verdict are real ones (`merge --no-edit`, and
`merge --abort` when it fails).

A second shape of busy shows in no status at all, and that was #232. With a directory inside the tree at
mode 000, `git ls-files --others --exclude-standard` prints `warning: could not open directory 'sub/':
Permission denied` and **exits 0**; `git diff --quiet` prints `sub/nested.txt: Permission denied` and
exits 0 on a tree whose only modification is that unreadable file. Every read answers at or below its
success status and the verdict says clean -- over an untracked file nobody enumerated in the first case,
and over an *edit* git asserted unchanged in the second, because a file it cannot open is a file it
cannot diff. So each read is now asked a sixth question, beside the five statuses: did it finish? A read
that returned its success status and still had something to say did not. What git printed on stderr is
therefore evidence as well as diagnosis, and the rule is deliberately about the fact that it spoke rather
than the words -- matching git's phrasing is how a check goes quietly blind the day upstream rephrases a
warning. Three dispositions, and the order matters:

- a refusal (`rc` above the success list) voids the verdict: UNKNOWN, as before;
- an incomplete read that found nothing, where the verdict holds nothing else, is UNKNOWN: its silence
  was never an answer, and this is the case that used to say clean;
- an incomplete read that found nothing, arriving where another arm already named the work, appends
  `its … were only partly read` to that finding instead of replacing it. Measured: a staged change
  beside an unreadable directory answers `staged-but-uncommitted: tracked.txt; its tracked files were
  only partly read (git diff exited 0; git said: sub/nested.txt: Permission denied); …` and stops the
  pass as dirt. The alternative tells somebody with an edit in front of them to go make the tree
  readable, and it also *loses the edit* from the sentence.

What that costs, and it changes halt semantics rather than a verdict: any git that prints something
harmless on one of these five reads halts the pass, and the shape that already exists is H-0045's
exclude-file warning -- `core.excludesFile` in a symlink loop answers `rc=0` with `warning: unable to
access '…'` on two of the reads. So the leg that plants it answers UNKNOWN naming the path git could not
read, where it used to answer DIRTY naming the file the operator told git to ignore and ordering them to
commit or discard it. Both halt the pass; the sentence and the reason changed. Genuinely new is the halt
on a tree that is clean apart from the unreadable config: it used to proceed, and it now stops because
git did not apply the operator's own ignore patterns, so git's clean is not the operator's clean -- the
same asymmetry #228 was filed on, applied to a case it did not have. Sixteen real trees on this box --
this repository and its fifteen links -- were read the four non-writing ways before that paragraph was
written (64 reads) and every one of them was silent; the refresh was deliberately not probed there,
because it rewrites `.git/index` and those trees may belong to a person working in them. So nothing that
exists today halts on existing chatter; what halts is whatever a future git starts saying. And because the capture is evidence now, a run that cannot make its
scratch file cannot answer: it refuses, naming `TMPDIR`, where it used to send stderr to `/dev/null` and
carry on -- #232 invisible on exactly the boxes whose `/tmp` is broken.

The five refusal checks are redundant on purpose -- any one of them refusing is enough to halt -- but
redundancy is a claim about the code, not a property you can see from reading it, so each arm is
measured: an arm is hollowed, one at a time, and the probe has to go red. Two ways of deleting an arm
give two different answers and neither is a substitute for the other. Replacing its `return 2` with `:`
means the arm sees the refusal and keeps going, so the sentence goes out with a finding appended and the
verdict ends CLEAN; replacing its condition with an always-false test means the arm never sees the
refusal and reads it as dirt, so it still halts and only a leg that asserts the refusal's own words can
tell. Both shapes now bite all five arms, because four of them had no plant that isolated them: every
state on disk that stops one of those four stops another as well, which is what an unreadable tree
actually does. The isolation is therefore built -- a `git` first on PATH that answers exactly one
invocation and passes every other argument list through untouched, one leg per arm, behind a control
whose match names nothing git ever runs. Since #232 that shim has two modes, and the second one is what
pins the completeness branch: refuse at 128, or return the arm's own success status with a line on
stderr. Five rows refuse, five rows warn, and the needle in each is the exit code git answered at --
`git diff exited 0` is the completeness branch and `git diff exited 128` is the refusal, so a leg cannot
stay green while one degrades into the other. The refresh's row warns at exit 1, because that read's
normal report is 1 and its `needs update` text is on stdout; a leg about it would be vacuous on a
success status of 0. That pins the arm's wiring; it makes no claim that git refuses
that way on its own, which is what the planted legs are for. What the rows also show is the weaker shape
at its worst: on the untracked arm the naive test says CLEAN, because a read that dies with 128 and no
output looks exactly like a tree with nothing in it.
The unreadable-file leg had to be tightened for any of this to mean anything. It used to assert only
that something stopped, and `expect … HALT` accepts any answer but clean, so the verdict could degrade
from `its tracked files could not be read (git diff exited 128)` to `modified: keepme.txt (git said: …
Permission denied)` with the leg green -- both halt, but the second tells the operator to commit or
discard a file git could not open. It asserts UNKNOWN and the refusal's words now, behind a runtime
check that this git refuses on the plant at all, and prints SKIP with the git version where it does not.

What the completeness branch is made of, measured as of H-0050 -- twelve mutations of
`factory/common.sh`, one at a time, against the 120-leg suite (119 on btrfs; the one-leg difference is
the exclude-directory plant, which refuses on tmpfs where its `st_size` is nonzero and swallows on btrfs
where an empty directory has none):

| the mutation | legs red |
|---|---|
| one arm's completeness branch deleted, per arm | refresh 2, staged 3, tracked 5, both `ls-files` arms 6 |
| the helper always incomplete, whatever it answered | 89 |
| the helper never incomplete, whatever it printed | 6 |
| `[ -z "$raw$finding" ]` removed from both `ls-files` arms | 12 |
| `[ -z "$finding" ]` removed from the tracked arm | 4 |
| the three `only partly read` append branches deleted | 4 |
| the refresh's success list `0 1` narrowed to `0` | 2 |
| the tracked arm's success list widened to `0 1` (dirt becomes unreadable) | 6 |
| the `/dev/null` fallback restored for a failed `mktemp` | 2 |

Two of those rows earned their legs from a mutant rather than from argument. Deleting the tracked arm's
guard left all 120 green: the only tree in the suite where the tracked read is incomplete *and* an
earlier arm has already found work is the staged-and-unreadable one, so that plant was written after the
mutant, and it is the row that goes red now. The `mktemp` leg was written when the capture became
evidence: restoring the old `/dev/null` fallback is two red legs, not a silent loosening.

Three of the new legs carry `strong`, which is the probe's one assertion about the *weaker* shape: it
refuses a leg whose dirt `git status --porcelain` can see, on the ground that the leg no longer pins the
arm it was written for. So those rows assert the weaker test is blind here, not merely that this verdict
is right. The marker cuts both ways -- setting it on the visible-work leg, where the weaker shape does see
the dirt beside an unreadable directory, is two red legs -- and that leg deliberately prints `naive:DIRTY`
unpinned, because the pass halts under either verdict and the leg is about which sentence the operator
gets.

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
