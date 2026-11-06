# Decision log

Every judgement call an unattended session made in this repository, with the options it was given
and the commit to roll back to. Written by the `factory` agent (`.opencode/skills/factory/SKILL.md`)
and checked by `scripts/check-decisions.py`, which runs in `make verify` and refuses an entry that
is missing a field, lists one option, or names a checkpoint that `git revert` cannot be pointed at.

The format exists because of how an autonomous loop fails. Not by choosing badly -- by choosing and
not saying so, so that six weeks later the only trace of a real decision is code that nobody can
explain and an issue that says nothing. An entry is therefore only useful if it records what lost.

Rolling one back:

```bash
git log --grep 'Decision: H-0007' --format='%H %s'   # the effect commit
git revert <that sha>                                 # a forward fix; no history rewrite
```

`Checkpoint:` is the commit taken *before* the decision, never after. That order is load-bearing: a
checkpoint taken afterwards records the state the decision produced, which is the thing you already
have. See the recorded hazard in the project rules -- restoring a backup taken before a later edit
silently un-does that edit, and the suite stays green because the tests were written against the
fixed file.

## Bootstrap entries

H-0001 through H-0005 were recorded in one commit rather than one each. They are one design, agreed
in one conversation before any of it existed, and splitting them across five commits would have put
a checkpoint in front of a decision that had already been taken -- which is the mistake the field
exists to prevent. From the first real tick onward, one decision is one commit.

### Why these ids are `H-` and the repository's are `D-`

Measured 6 Nov while merging `origin/main` into this branch: `main`'s `DECISIONS.md` carried
`D-0011`..`D-0015` as *product* decisions (a control's typeface, `:has()` reaching a tick box's words)
while this file carried `D-0011`..`D-0015` as *harness* decisions (the tick's `$PWD`, the worktree cut,
the deny keying). Neither was wrong: a tick numbers from the highest id it can see, and a worktree cut
before `DECISIONS.md` existed sees none of it -- so two unattended loops each opened the log at 11. The
harness log now lives here, at `factory/DECISIONS.md`, with its own prefix, and the product log keeps
`D-` where it was.

**Older text still cites these as `D-0001`..`D-0020`** -- pull-request bodies, commit messages, and the
occasional line in `docs/factory.md` that predates the split. `D-0013` in anything written before 6 Nov
means `H-0013` here. Nothing was renumbered out of existence; the numbers moved prefix, not order.

## H-0001 · The harness lives in the repository, not in a shared skill

- Question: where does the delivery loop live -- in this repository, or in the shared agent-skill
  tree where every project could use it?
- Options: in this repository | in the shared skills repository
- Chose: in this repository
- Why: the loop is only trustworthy where this repository's own gate runs. `.opencode/` is already
  tracked for `issue-check` and `adversarial-review`, and a harness that lives outside the tree it
  drives cannot pin `make verify`, cannot be reviewed with the change it is merged alongside, and
  cannot be reverted with the code it was written for. The generality the second option offers is
  bought later by lifting the scripts once they have survived a real queue -- nothing in them may
  name this repository, which is why `FACTORY_REPO` is read from `origin` and `FACTORY_REVIEWER` is
  an override.
- Checkpoint: 122a80ee2b4a324562e8de82a025a47a6e4dae07
- Effect: factory/factory.sh, factory/triage.sh, factory/common.sh, .opencode/skills/factory/SKILL.md

## H-0002 · Mechanical GitHub actions are bash; a model is only for judgement

- Question: does a tick perform every action, or does the supervisor do the ones that need no
  judgement itself?
- Options: a tick does everything, so there is one code path | the supervisor does the mechanical
  ones in bash and starts a session only for judgement
- Chose: the supervisor does the mechanical ones in bash and starts a session only for judgement
- Why: measured on 4 Nov, four of the five open pull requests needed one `gh` call and no judgement
  at all -- request a reviewer, merge an approved PR. Routing those through a session means they
  wait for a session, and a session is exactly the thing that has been dying. It also puts a model
  in the path of an irreversible action for no reason: `factory_merge` re-reads `reviewDecision`,
  `mergeable`, `mergeStateStatus` and every check from the API immediately before merging and
  refuses unless all four still say yes.
- Checkpoint: 122a80ee2b4a324562e8de82a025a47a6e4dae07
- Effect: factory/factory.sh

## H-0003 · Never merge unapproved; poll forever, at a relaxed interval

- Question: what does the loop do with a pull request that is green but has no review?
- Options: merge it once every check is green | park it after N days and report | keep asking for a
  review forever, slowing to a 30-minute cadence, and never merge it
- Chose: keep asking for a review forever, slowing to a 30-minute cadence, and never merge it
- Why: the user's rule, given directly. An approval is a human statement about code, and a machine
  that can substitute its own judgement about a green tick has replaced the only control the process
  has left. The park half of the second option is what loses work -- #212 and #214 were already
  three days invisible when this was decided, and a parked unit is invisible with a label on it. So
  the wait is unbounded and loud instead: `FACTORY_REVIEW_POKES=1800`, `FACTORY_ESCALATE_AFTER=86400`
  posts one comment a day saying what is blocking and that nothing has been parked.
- Checkpoint: 122a80ee2b4a324562e8de82a025a47a6e4dae07
- Effect: factory/common.sh, factory/factory.sh

## H-0004 · `question` is denied to the agent; a question becomes a logged decision

- Question: how does a headless session handle a decision point -- stall, or decide?
- Options: leave `question` available and let the tick stall | deny the tool and require a
  checkpoint-plus-log procedure instead | deny the tool and let the model improvise
- Chose: deny the tool and require a checkpoint-plus-log procedure instead
- Why: `question` pauses and presents the question to a user, and `opencode run` renders no question
  UI. Leaving it available does not produce an answer, it produces a worktree held until the
  wall-clock ceiling kills the tick, with no reason recorded anywhere. Denied, the call returns an
  error to the model, and there has to be something to do instead of improvising -- hence the
  tie-break order in the skill (dependency rule #856 first, then decode-ambiguity-toward-lockout
  #873, then refuse-rather-than-proceed, then blast radius, then reversibility, then what the
  surrounding code already does) and the escalation list for the four classes of decision that are
  not the agent's: dependencies, security semantics, publishing, deleting data.
- Checkpoint: 122a80ee2b4a324562e8de82a025a47a6e4dae07
- Effect: .opencode/opencode.jsonc, .opencode/skills/factory/SKILL.md

## H-0005 · One decision is one commit, and rollback is `git revert`

- Question: how is an autonomous decision made reversible?
- Options: rewrite history back to before it | commit a checkpoint before it and revert the effect
  afterwards | keep no record and rely on the branch being closed
- Chose: commit a checkpoint before it and revert the effect afterwards
- Why: the third is how work gets lost quietly, and the first is forbidden here on its own terms --
  `review-loop` refuses to force-push a branch under review, and this repository has
  `allow_squash_merge=false` and `allow_rebase_merge=false`, so history is already merge-commit
  shaped. A checkpoint commit plus one effect commit carrying a `Decision:` trailer gives the same
  power as a rewrite, forward-only: `git log --grep 'Decision: H-0007'` finds the effect, `git
  revert` undoes it, and both are visible to whoever arrives next.
- Checkpoint: 122a80ee2b4a324562e8de82a025a47a6e4dae07
- Effect: .opencode/skills/factory/SKILL.md

## H-0006 · A ref that is a backup by its name is reported, not worked

- Question: the classifier finds branches with commits and no pull request; should the supervisor
  open a pull request for every one?
- Options: open one for each, and let review reject it | refuse the names that say backup and report
  them | refuse every orphaned branch and report them all
- Chose: refuse the names that say backup and report them
- Why: measured on the first pass over this repository, five branches had no open pull request --
  `backup/audit-page-pre-rebase`, `backup/pre-rebase-66`, `local/e2e-rig`, `fix/nav-marks-the-current-page`
  and `chore/ignore-plock-binary`, one of them seven commits deep. The last two are unmerged work
  worth adopting; the first three are safety copies somebody chose to keep, and a machine turning
  one into a contribution is a machine deciding what a person meant. The third option was rejected
  because the seventh-commits-deep branch is exactly the thing this class exists to catch: work
  already paid for and invisible to every queue that reads issues and pull requests.
- Checkpoint: 122a80ee2b4a324562e8de82a025a47a6e4dae07
- Effect: factory/factory.sh (`factory_refuseto_work`)

## H-0007 · Pull-request work outranks issue work, which outranks orphaned branches

- Question: within one pass, what orders the units a model is offered?
- Options: oldest `first-seen` first, regardless of kind | work nearest to merged first, age as the
  tie-break | whatever triage happened to return
- Chose: work nearest to merged first, age as the tie-break
- Why: this is a deliberate reversal of a queue's usual oldest-first rule, and it is warranted by a
  measurement rather than by taste. `red checks` and `conflict` mean a pull request that cannot be
  merged; `unclaimed` means work that has not started. The state this repository has actually been
  observed stuck in is a green pull request nobody is looking at, and a red one is one edit away from
  that. An orphaned branch is last because its work is already written and its only deadline is the
  day somebody prunes refs. Age still decides within a kind, so #199 cannot be starved by a stream of
  new issues, and the lowest number breaks a tie because every unit is tied on the very first pass.
- Checkpoint: 122a80ee2b4a324562e8de82a025a47a6e4dae07
- Effect: factory/factory.sh (`factory_class_rank`)

## H-0008 · The decision log is gated inside `make verify`, and the gate pins itself

- Question: what makes an unattended session actually write the log -- the gate, CI, or a pre-commit
  hook?
- Options: check it in `make verify` | check it only in CI | a pre-commit hook | nothing, and trust
  the skill's instructions
- Chose: check it in `make verify`
- Why: a pre-commit hook is per-clone and unversioned, so a fresh worktree -- which is every worktree
  the supervisor creates -- has no hook at all; CI checks it after the push, which is the same rule
  arriving late enough to require another commit; and the fourth option is what every previous session
  in this repository already demonstrated it does, which is nothing at all. `verify` is the gate this
  repository already treats as the deliverable, and the check costs milliseconds. The gate then
  asserts its own invocation in `verify`, because the cheapest way to pass a gate is to delete the
  line that calls it: measured by removing both lines, and the self-test refuses with
  `a gate whose invocation has been deleted is a suggestion`.
- Checkpoint: 55b4a2041f70eee42bbc7282c59f8fbe94204bad
- Effect: scripts/check-decisions.py, Makefile

## H-0009 · Install the timer against the harness checkout, not the primary one, until this merges

- Question: which checkout does the supervisor's systemd unit point at, while the harness itself is
  still an open pull request?
- Options: wait for the merge, then install | point the unit at the harness worktree and reinstall
  after the merge | point it at the primary checkout and let it fail until the merge lands
- Chose: point the unit at the harness worktree and reinstall after the merge
- Why: the third option is the one a person gets by accident and it is the worst of the three,
  because it fails quietly: the timer fires, systemd finds no script at the `ExecStart` path, the unit
  exits 127, and the journal is the only place that says so -- so `install-timer.sh` now refuses when
  `factory/factory.sh` is not executable in the target tree, rather than installing a timer that
  looks alive. The first option leaves the queue exactly as stuck as it is today, which is the state
  this whole change exists to end, and the merge is the one step the factory is forbidden to perform
  on its own pull request. The second is reversible in one command and unblocks the half that is
  already unblocked: four review requests and one merge need no tick and no merged skill file.
- Checkpoint: c8da095c350d472dab540579538eb5d8bc85bdbe
- Effect: factory/install-timer.sh

## H-0010 · Request the review through REST, not `gh pr edit --add-reviewer`

- Question: which command asks a human for a review?
- Options: `gh pr edit --add-reviewer` | REST `POST /pulls/N/requested_reviewers` | leave it to the
  reviewer's own notification rules
- Chose: REST `POST /pulls/N/requested_reviewers`
- Why: the CLI verb is broken on this account and fails *quietly in the log, loudly in the exit code*.
  Measured on 4 Nov while opening #217: `gh pr edit 217 --add-reviewer hermesvicowa` exited 1 with
  nothing but `GraphQL: Projects (classic) is being deprecated …
  (repository.pullRequest.projectCards)` on stderr, and `/requested_reviewers` stayed empty; the REST
  POST on #217, #214, #212 and #209 all returned the reviewer and read back as requested. The whole
  reason `needs_reviewer` exists is that nothing had ever asked, so a poke path that silently does
  nothing rebuilds the original bug with a log line claiming otherwise. Writing and reading the same
  fact through one route is also what keeps the two from disagreeing.
- Checkpoint: 6ac22d91504b0f4be7f2fb2f9c76c46da358972e
- Effect: factory/factory.sh (`factory_request_review`)

## H-0011 · Name the tick's project with `$PWD`, because a unit's `WorkingDirectory` is not one

- Question: how does a headless tick get told which project it is inside, when the project is what
  defines the agent, its `deny` rules and the skill the instruction points at?
- Options: export `PWD` for the CLI | hand the CLI a directory argument | define the `factory` agent
  globally in `~/.config/opencode/opencode.json`
- Chose: export `PWD` for the CLI
- Why: in the generated unit (`Environment=PWD=@REPO@`) and again at the call site in `run_tick`
  (`env PWD="$FACTORY_HARNESS"`), which is the half that fixes a pass started from anywhere else.
  Measured on 5 Nov, on the unit itself. `opencode run` reads `$PWD` and chdir*s *there* -- a
  `$PWD` naming a directory that does not exist fails with `Error: Failed to change directory to
  <path>` -- and a real `$PWD` beats the process's own cwd (`PWD=<primary checkout>` with cwd=/tmp
  resolved the primary's agents, not /tmp's). With no `$PWD` at all the CLI never moves and the
  session opens in the background server's directory, here `/home/rodney` where
  `opencode-cli serve --service` has run since 1 Nov. Nothing defines an agent there, so
  `--agent factory` died in 90 ms on `Error: Agent not found: "factory"` for **152 passes running**,
  4-5 Nov, while `supervisor.log` recorded only `exit=1` and systemd called the unit
  `status=4/NOPERMISSION`; the CLI's own words appeared nowhere in the journal (0 hits for
  `Agent not found` in `journalctl --user -u parental-lockout-factory.service` since 4 Nov), because
  the tick's output is redirected into its per-tick log. The third option is the dangerous one, not
  the safe one: it moves the tick's boundary out of the repository, where `make verify`, this log and
  every review can see it, into a `$HOME` file no clone carries and nothing gates -- and the one rule
  in the present config that the CLI resolves at all is `question: deny`, a tool name, so a copy made
  there would inherit finding 1 of the `#217` review (why the eleven command-shaped rules bind nothing
  yet) without inheriting the review.
  The second option does not exist:
  `opencode run [flags] [<message...>]` takes a message and no directory; only the top-level command
  has the positional. So the fix is not a property of this one unit, and the unit half is the part a
  person reading `systemctl cat` can see. Both point at the harness tree (`FACTORY_DIR/..`) and not
  at `FACTORY_ROOT`, because `FACTORY_ROOT` is the *primary* checkout and that tree holds no
  `.opencode/opencode.jsonc` until this branch merges (H-0009). A second change rides along because
  the same outage was unreadable for 29 hours: `TICK-FAILED` now carries the last line of the tick's
  own log, so the cause is in the journal rather than only in a 47-byte file nobody opens.
- Checkpoint: 53a0850c4bc6ccb37a6004f4f2aab9cf57ed68f9
- Effect: factory/common.sh (`FACTORY_HARNESS`), factory/factory.sh (`run_tick`),
  factory/install-timer.sh (generated `[Service]` unit), docs/factory.md

## H-0012 · Cut the worktree from the branch when the branch is already committed, and name the tree that owns it

- Question: what builds a tick's worktree when the unit's branch already exists locally but nothing
  is checked out at `$FACTORY_TREES/<slug>`?
- Options: try the checked-out form (`worktree add "$path" "$branch"`) ahead of the two creating
  forms and stop with the other tree's path when one holds it | keep the two `-b` attempts and let the
  unit fail | move whatever worktree git reports as holding the branch, into place
- Chose: try the checked-out form (`worktree add "$path" "$branch"`) ahead of the two creating
  forms and stop with the other tree's path when one holds it
- Why: measured 5 Nov on this repository, not inferred. Both creating attempts answer
  `fatal: a branch named 'fix/audit-row-label' already exists` -- `worktree add -b` cannot be used for
  a branch that exists, and it is the `-b` form in both attempts -- so every unit that is already a
  pull request was unworkable: `pr:209`, `pr:214`, `pr:215` and `pr:217` produced 285
  `could not create a worktree` lines in the journal on 5 Nov alone, and `git worktree list` shows the
  branches were checked out, clean, the whole time -- six trees under `/tmp/opencode`, left there by
  interactive sessions, which is the location `common.sh` already rules out for unmerged work because
  a reboot deletes it. The third option is refused rather than deferred: a tree this supervisor did
  not create may hold a person's uncommitted work, and one of those six does (`/tmp/opencode/wt-204`,
  four dirty paths, copied out to `~/prj/at-risk` on 4 Nov precisely because it was not safe to
  touch). So the message names the other tree and the one command that relocates it, which turns a
  silent queue jam into an instruction, and touches nothing. The checked-out form costs nothing in the
  cases that used to work: it is guarded by `show-ref --verify refs/heads/$branch`, and a branch that
  is only on origin still falls through to `--track -b` exactly as before.
- Checkpoint: c07b9c5ca2496376454ebcc34507ed8ebc3235e6
- Effect: factory/common.sh (`factory_worktree`)

## H-0013 · Key the deny rules under `bash`, because a flat command pattern denies nothing

- Question: a headless tick has `--auto` on, so the deny list in `.opencode/opencode.jsonc` is the
  whole boundary. Why did the previous list not bind?
- Options: leave the rules flat and trust the CLI to recognise a command pattern as a rule | nest the
  command patterns under the `bash` tool key, where a pattern is a `resource` | drop the deny list and
  keep only the rule that a tick cannot merge
- Chose: nest the command patterns under the `bash` tool key, where a pattern is a `resource`
- Why: measured on v2.2.5 in this tree, both ways, same entries. Flat, `opencode debug agents` reads
  `"git push --force*": "deny"` as `{"action": "git push --force*", "resource": "*", "effect": "deny"}`
  -- an ACTION, and no tool reports an action named after a shell command, so no check ever consults
  it; the probe `gh pr merge 999999` under `--auto` ran and reached `gh`. Nested, the same entries read
  as `{"action": "shell", "resource": "gh pr merge*", "effect": "deny"}` and the identical probe comes
  back `Permission denied: shell` without starting `gh`. A `deny` that parses as a rule for a tool that
  does not exist is not a weak rule, it is no rule, and the file read as a serious one for a week.
  Two things follow that are recorded rather than fixed. The matcher is a wildcard over the whole
  parsed command line, not a prefix matcher, so the push rules are written `* push --force*` --
  `git -C <tree> push --force` is the form this harness actually uses, and a rule beginning `git push`
  would have let it through even with the keying right. And it cannot spell "this API call is a read":
  `-X` moves around in the arguments, so `gh api` is denied whole and the two reads a tick used to do
  with it move to first-class commands (`gh pr view --json reviewRequests`) or to the supervisor. That
  costs a read the skill previously instructed: `gh pr view --json` has no `closingIssuesReferences`
  field (measured: `Unknown JSON field`), so only GraphQL can ask GitHub what a body closed, GraphQL is
  `gh api`, and `factory/triage.sh` -- which is not an agent and is not gated by this file -- is now the
  place that read happens. The skill says so instead of instructing a command that will be refused.
- Checkpoint: a4c714c62abe7c8b00892a766ed4161ae7adc450
- Effect: .opencode/opencode.jsonc, .opencode/skills/factory/SKILL.md

## H-0014 · Run each tick against a private server, so the ceiling is a kill and not a hang-up

- Question: the supervisor killed a tick at its 2700 s ceiling, recorded `TICK-FAILED exit=124`,
  reported the tree as clean, and the tree kept changing for four more minutes. What is the ceiling
  actually bounding?
- Options: bound the wall clock and accept that the session keeps running | pass `--standalone` so the
  tick's server is our child and TERM reaches it | have the supervisor kill sessions by id at the
  ceiling, through the server's API
- Chose: pass `--standalone` so the tick's server is our child and TERM reaches it
- Why: measured on the first long tick, 5 Nov. `timeout` fires at 22:15:07Z and the supervisor logs the
  failure; file mtimes in that tick's worktree are 22:17:34Z, 22:17:57Z and 22:19:18Z, and `git status`
  keeps reporting a different set of paths -- `logind.go`, then `sessionbus.go`, then `command.go`,
  then `profiles.go`, none of which is the issue's area -- while `opencode session list` in that
  project still shows three `factory unclaimed:204` sessions from before the ceiling. The default
  `opencode run` is a client of the shared background server, so the session and every process it
  spawns live in another process tree: the ceiling detaches the supervisor from the work, it does not
  stop the work. Nothing was merged, because a tick cannot merge (H-0006), but the supervisor's
  statement that a unit failed and its tree is nobody's is false while that session is editing it, and
  the model budget burns with nobody accounting for it. Killing by session id is refused: the operation
  is not exposed to a shell caller (measured -- `POST /session/<id>/abort` answers 405 on every port
  this CLI will talk to, and the `session.abort` operation id errors inside the CLI), so it is not
  something the harness can rely on. `--standalone` costs a server startup per tick and moves the
  conversation out of the shared session store; the per-tick log named by `TICK-START` was always the
  record, so nothing that is read afterwards changes.
- Checkpoint: a4c714c62abe7c8b00892a766ed4161ae7adc450
- Effect: factory/factory.sh (`factory_tick`)

## H-0015 · Put the gate's whole toolchain in the unit's PATH, and refuse to install without it

- Question: a tick ran 45 minutes, never ran a build, and died at the ceiling. Where did the compiler
  go?
- Options: build the unit's PATH from the two binaries the installer itself needs (`gh`, `opencode`) |
  name every binary `make verify` shells out to and refuse the install when one is absent | let the
  tick discover it and log the failure
- Chose: name every binary `make verify` shells out to and refuse the install when one is absent
- Why: the third option is what was happening, and the log is the part nobody reads. Measured: that
  tick's own capture step ends in `FileNotFoundError: [Errno 2] No such file or directory: 'go'`, and
  the unit's PATH was `~/.opencode/bin:/usr/bin:/usr/local/bin:/usr/bin:/bin` -- `go` and `gofmt` are
  in `~/.local/bin` on this machine, and a unit inherits no shell profile, so a compiler that works in
  a terminal does not exist for the service. The failure also arrives in the wrong place: 230 KB into
  a Python traceback inside a per-tick log, two levels of indirection from the `TICK-FAILED` line that
  is the only thing the journal shows. A tick that cannot compile cannot satisfy the one condition it
  is required to satisfy, so the installer now asks the machine for `make git python3 go gofmt` as well
  and stops with the names when one is missing. It costs a person one sentence at install time instead
  of 45 minutes of model budget and a queue that looked busy.
- Checkpoint: a4c714c62abe7c8b00892a766ed4161ae7adc450
- Effect: factory/install-timer.sh

## H-0016 · `factory_resolve_conflict` answers three ways; only a real conflict opens a tick

- Question: the function already logs `CONFLICT-SKIPPED pr:217 attempted 10s ago`, and the caller then
  queued a tick for the same unit anyway. What was the second answer for?
- Options: keep one non-zero status and let the caller treat every refusal as work for a model | give
  the function three answers -- done, needs a reader, not now -- and make the caller act on the third
  | throttle the caller instead of the function
- Chose: give the function three answers -- done, needs a reader, not now -- and make the caller act on
  the third
- Why: a tick is the expensive thing here: 2700 s of ceiling, a model run, in a tree that cannot even
  compile the language (H-0015). With one non-zero status, a conflict ten minutes old, a working tree
  left dirty by a dead tick, a failed `fetch` and a failed `push` each meant "send it to a model", so
  the log carried a self-explanation -- "we tried 10 seconds ago" -- that changed nothing about what
  the pass did next. Two of those are worth naming separately. The dirty-tree branch also writes
  `$FACTORY_HALT`, and queueing a tick behind it spends a model run on a unit the pass has just decided
  to stop for; and a push that failed after a clean local merge is a permissions or network fault that
  the next activation retries by itself, not a conflict anyone can read their way out of. So `0` merged,
  `1` means origin/main genuinely does not merge in and only a reader can keep both intents, `2` means
  try again on the next activation; the caller logs `CONFLICT-DEFERRED` for `2` and opens a tick only
  for `1`. An unrecognised status is a supervisor bug and sets `rc=4` rather than being treated as
  either answer.
- Checkpoint: a4c714c62abe7c8b00892a766ed4161ae7adc450
- Effect: factory/factory.sh (`factory_resolve_conflict`, `factory_body`)

## H-0017 · A dry run writes no ledger

- Question: `--dry-run` prints `DRY-RUN` for every action it does not take. It also stamped ledger
  files. Does that matter, when a ledger file is just a timestamp?
- Options: leave it -- the ledger is bookkeeping, not an action | suppress the ledger writers under
  `--dry-run`, and keep the journal real | suppress the journal too, so a dry run leaves no trace
- Chose: suppress the ledger writers under `--dry-run`, and keep the journal real
- Why: a ledger file is a timestamp with consequences, and those consequences are the queue's
  behaviour. `first-seen` is what makes a unit old enough to be picked first; `stuck-since` starts the
  clock that produces an escalation comment; `last-notice` is the throttle that stops the supervisor
  commenting at a maintainer every ten minutes; `conflict-at` is the ten-minute guard in H-0016. A dry
  run that writes them has rehearsed a state change: after one, the next real pass quotes `after
  86400s` for a wait that never happened, and a maintainer who was never contacted has already been
  throttled. "Print, don't do" cannot be partial, because the half that is done is the half a later run
  reads. The journal stays real for the opposite reason -- a dry run that records nothing is a dry run
  nobody can audit afterwards, and `log_event` is the artefact a person reads to decide whether the
  plan was right.
- Checkpoint: a4c714c62abe7c8b00892a766ed4161ae7adc450
- Effect: factory/common.sh (`factory_ledger_set`, `factory_ledger_clear`)

## H-0018 · Quote the unit heredocs: an interpolated template executes its own comments

- Question: adding a comment to the generated unit made the installer run `gh --help`, print
  `Unit network-online.target could not be found.`, and report `Permission denied` on a path under
  `factory/run/`. What was executing?
- Options: escape the backticks in the new comment text and leave the heredoc interpolating | quote the
  heredoc (`<<'UNIT'`) and pass every value through the `@TOKEN@` renderer instead | keep the
  interpolation and add a guard that rejects backticks in `install-timer.sh`
- Chose: quote the heredoc (`<<'UNIT'`) and pass every value through the `@TOKEN@` renderer instead
- Why: both unit templates are `$(cat <<UNIT | render ...)` with an unquoted delimiter, so the shell
  expands the body before `render` ever sees it -- every backtick pair inside the unit is a command
  substitution executed by whoever runs `--install`, and every `$VAR` is taken from the installer's
  environment. That is the second time this harness has shipped it: the first pass escaped `\$PWD` in
  one comment, which is the signature of having found the bug and fixed the instance. The instance fix
  cannot hold, because the comments in this file are the documentation and documentation names things in
  backticks -- `Wants=`, `gh`, `loginctl show-user $USER --property=Linger` -- and each one is a
  command. Measured after the change: the dry run renders with no subprocess noise, no `@TOKEN@` left
  unsubstituted, and the comment text intact, including a literal `$USER` that would otherwise have
  frozen this box's username into every other user's unit. The renderer gained `sed_replacement` in the
  same pass for the matching hole on the other side: a value containing `&` used to paste the matched
  text into the unit, and `FACTORY_REVIEWER=a#b` used to install `Environment=FACTORY_REVIEWER=a`.
  Measured both, and a name with a `#` in it now survives whole.
- Checkpoint: a4c714c62abe7c8b00892a766ed4161ae7adc450
- Effect: factory/install-timer.sh (`sed_replacement`, `render`, both unit templates)

## H-0019 · The timer's two load-bearing claims were both false, and the real condition is linger

- Question: `docs/factory.md` says `Persistent=true` is what re-runs a pass missed while the machine
  was off, and that systemd kills a long oneshot at the 90 s default regardless of the tick ceiling.
  Are either of those true here?
- Options: keep the prose and the flag as they are -- they are conventional settings | delete both
  claims, cite the manual pages that refute them, and write down what actually decides whether an
  unattended timer runs | add an explicit `TimeoutStartSec=90` to match the prose
- Chose: delete both claims, cite the manual pages that refute them, and write down what actually
  decides whether an unattended timer runs
- Why: systemd.timer(5): `Persistent=` "only has an effect on timers configured with `OnCalendar=`",
  and this unit has none -- so the flag was inert and the sentence about missed runs described nothing,
  while `Persistent=yes` still shows in `systemctl --user show` and told a reader the opposite of the
  truth. systemd.service(5): `TimeoutStartSec` "Defaults to `DefaultTimeoutStartSec=` set in the
  manager, except when `Type=oneshot` is used, in which case the timeout is disabled by default" --
  measured here (systemd 259): the live unit reported `TimeoutStartUSec=infinity` against a manager
  default of 1 min 30 s, and a pass ran 45 minutes untouched, so the claim that a tick would be killed
  at 90 s is not merely wrong but wrong in the direction that matters: nothing bounded the wedged pass
  at all, and while it holds `factory/run/lock` every later activation says "another supervisor holds
  the lock" and does nothing. The generator now writes `TimeoutStartSec` explicitly at the same value as
  `TimeoutStopSec`, because a default that depends on `Type` is a default that changes the day someone
  changes `Type`. What is really load-bearing is `Linger`, and the honest statement of it is the check
  rather than a reading: at 22:17Z on 5 Nov `loginctl show-user rodney --property=Linger` answered `no`
  -- the user manager, and therefore this timer, does not run when nobody is logged in -- and at
  22:32:51Z `/var/lib/systemd/linger/rodney` appeared, root-owned, so it had been switched on
  underneath this work by something running as root, between the two times I measured it. A fact that
  changes while the doc about it is being written is a fact a tool has to re-check, so `--install` now
  runs that check and warns when it is off. No flag in a generated unit affects it either way. The third correction is the pair
  `After=network-online.target` / `Wants=` it orders: measured, `systemctl --user status
  network-online.target` answers `Unit network-online.target could not be found.` -- the target belongs
  to the system instance, where NetworkManager-wait-online.service pulls it in -- so the pass was
  ordered after a unit that cannot load, and implied a network wait it does not perform. It does not
  need one: `gh` failing is a visible `TRIAGE-FAILED` with rc=3 and nothing attempted, and the next
  activation is ten minutes away.
- Checkpoint: a4c714c62abe7c8b00892a766ed4161ae7adc450
- Effect: factory/install-timer.sh (both unit templates), docs/factory.md

## H-0020 · A claim on an issue needs a verb, not a mention

- Question: `triage.sh` treats an issue as claimed when an open pull request already covers it, so it
  does not open a second pull request for the same work. Which pull-request bodies count as covering
  an issue?
- Options: any open body that contains `#NNN` | only the closing references GitHub itself registers |
  any body that names the number with a verb -- `closes`, `fixes`, `resolves`, `refs`, `part of`
- Chose: any body that names the number with a verb -- `closes`, `fixes`, `resolves`, `refs`, `part of`
- Why: measured against the live open bodies on 5 Nov, the mention rule reported fourteen numbers
  claimed -- 144, 149, 199, 202, 206, 207, 209, 210, 211, 212, 213, 214, 215, 216 and 1048, the last
  one a memory id inside a table cell -- while the verb rule reports five: 202, 207, 210, 211, 216,
  which are precisely the ones the bodies claim. On the pull-request half a false claim is merely
  untidy; on the issue half it is a withdrawal, because a claimed issue is dropped from `unclaimed` and
  never offered to a tick. Two open issues were sitting there: #199, "Improve the design of the web UI",
  and #213, whose own status was the subject of a sentence in another pull request ("#213 stands and is
  unchanged") -- naming an issue to say it is *not* being finished is the exact sentence that reads as
  a claim to a mention matcher. The second option is refused for the reason in #1256: this repository
  writes `Part of #10 only` and `Refs #10` on purpose, because an adjacent inflection of "close" next
  to an umbrella issue closes it at merge time, and GitHub registers neither of those forms -- a
  classifier that reads only closing references reports finished work as untouched and opens a
  duplicate pull request for it. The verb list is therefore the union of what GitHub registers and what
  this repository writes, and nothing else.
- Checkpoint: a4c714c62abe7c8b00892a766ed4161ae7adc450
- Effect: factory/triage.sh (`mentioned_issues`)

## H-0021 · Two decision logs, two id spaces: the harness log moves to `factory/DECISIONS.md`

- Question: merging `origin/main` into this branch produced an add/add conflict in `DECISIONS.md` whose
  two halves used the same ids for different decisions. Which half moves?
- Options: renumber this branch's twenty harness entries | renumber `main`'s five product entries | keep
  one file and let the ids repeat | give each log its own file and its own id prefix
- Chose: give each log its own file and its own id prefix
- Why: product `D-` stays in `DECISIONS.md`, harness `H-` moves to `factory/DECISIONS.md`. Measured while
  resolving the merge on 6 Nov. `main` carries `D-0011`..`D-0015` as page-design
  decisions (a control's typeface, `:has()` reaching a tick box's words, three refusals measured through
  Chrome); this branch carries `D-0011`..`D-0020` as harness decisions (the tick's `$PWD`, the worktree
  cut, the deny keying). The merge base `3b2f594` has no `DECISIONS.md` at all, which is the whole
  mechanism: a tick numbers from the highest id it can *see*, a worktree cut before this file existed sees
  none of it, and both loops therefore opened their log at 11. The damage is not the duplicate headings,
  it is the citations -- `main`'s own `D-0015` cites `D-0001` and `D-0010`, and neither exists on `main`
  (`git show origin/main:DECISIONS.md | grep -c '^## D-0001'` answers 0), so a reversible record had
  become unresolvable prose. Renumbering either side silently invalidates pull requests that were written
  against it: #217 and its review cite the harness `D-0011`..`D-0020` right now, while merged #218's body
  cites the product ones. A prefix keeps both sets of citations true, makes the collision structurally
  impossible rather than merely noticed, and costs one mapping line in each log.
  `scripts/check-decisions.py` now reads both files and refuses an id that appears in both, so a future
  reuse fails the gate instead of the reader. Two smaller things came with it: the same gate refused
  `main`'s `D-0015` because its `Chose:` line carried the chosen option *plus* the arithmetic that
  followed it and therefore matched no recorded option -- the field is now the option and the arithmetic
  sits in `Why:`, text unchanged -- and this log's own entries were rewritten `D-` to `H-` in place, with
  the header stating that anything written before 6 Nov citing `D-0013` means `H-0013`.
- Checkpoint: 26d4b5f8948cf19c30d5cb762651e095ab84059a
- Effect: DECISIONS.md, factory/DECISIONS.md, scripts/check-decisions.py,
  .opencode/skills/factory/SKILL.md, docs/factory.md, README.md, AGENTS.md

## H-0022 · A tick is delivered when a pull request names its branch, not when it exits 0

- Question: `TICK-DONE` means the session exited 0. The first unclaimed issue this harness worked
  properly produced nine commits and no pull request. What should the supervisor believe?
- Options: trust the exit status and clear the unit | ask GitHub for a pull request on that branch after
  every tick and treat its absence as the unit not having moved | require the tick to print the pull
  request url in its last line
- Chose: ask GitHub for a pull request on that branch after every tick and treat its absence as the unit
  not having moved
- Why: measured 6 Nov on issue #199. `TICK-START unclaimed:199` 01:05:27Z, `TICK-DONE` 01:30:43Z, exit 0;
  the branch `issue-199` is nine commits ahead of `main`, is not on `origin` at all
  (`git ls-remote --heads origin` lists only `fix/audit-row-label` and `fix/manifest-records-conditions`),
  and the tick's final line was "Reading the battery and byte-repeatability results will decide the PR
  body's evidence table, so I'm holding the push until those logs land." The unit fell straight back into
  `unclaimed` -- the same 01:05 pass had just printed `NEXT unclaimed 199` -- so every activation re-offers
  the issue while the work sits invisible in one worktree, which is the queue-jam this harness exists to
  end, now with a green log line next to it. The exit code measures whether the *program* stopped, and a
  model that runs out of ceiling while waiting for its own background measurement stops politely. A pull
  request is the only evidence available to a shell, and it is exactly what the agent's own authority ends
  at (#1823), so the check is `gh pr list --head` and the answer is `TICK-UNDELIVERED` with status 4, which
  leaves `stuck-since` standing. A failed *read* is reported as `TICK-UNCHECKED` rather than as a missing
  pull request: the supervisor must not convert its own outage into somebody else's failure. The skill now
  tells the tick the consequence -- push and open the pull request even with unmeasured claims, saying so
  in the body -- because the check alone would just move the surprise into the log.
- Checkpoint: 26d4b5f8948cf19c30d5cb762651e095ab84059a
- Effect: factory/factory.sh (`factory_tick`), .opencode/skills/factory/SKILL.md

## H-0023 · Refuse where the refusal will be read, and start a throttle at the attempt

- Question: for `pr:217` the supervisor logged "not a merge a model can settle; the next activation
  retries" every ten minutes, when the branch is not reachable at all. Where did the real reason go?
- Options: leave `factory_worktree`'s `die` to reach the journal and reword the caller's line | capture
  the refusal's stderr at the call site and log it as `CONFLICT-BLOCKED`, and take the throttle stamp
  after the worktree is obtained | have `factory_worktree` return the reason on stdout
- Chose: capture the refusal's stderr at the call site and log it as `CONFLICT-BLOCKED`, and take the
  throttle stamp after the worktree is obtained
- Why: `factory_worktree` refuses with `die`, and this is the second shape of a bug already recorded here
  -- a `die` inside `$(...)` exits the subshell, so the status crosses the assignment and the sentence
  does not. Measured 6 Nov across the passes from 00:49Z: `CONFLICT-DEFERRED pr:217 not a merge a model can
  settle` three times, and no `checked out in` line anywhere in the unit's journal, so the only account of
  it in the repository asserted a reason that had nothing to do with the truth -- the branch is checked out
  in the tree the supervisor itself runs from, which H-0012 deliberately refuses to move. The stamp was the
  second half: `conflict-at` was written *before* the worktree call, so a branch this supervisor cannot
  reach still started the 600 s throttle, and each following pass printed `CONFLICT-SKIPPED attempted Ns
  ago` over the one line that could have explained it. Two rules, both general: a refusal must be logged by
  the thing that refused, in the file a person reads the queue from, and a throttle must measure the work,
  not the intention to work.
- Checkpoint: 26d4b5f8948cf19c30d5cb762651e095ab84059a
- Effect: factory/factory.sh (`factory_resolve_conflict`, and the `2)` arm of its caller)

## H-0024 · Commands the permission scanner cannot parse are refused, so keep one statement per call

- Question: a tick that used to run any `gh pr view` now dies repeating `Error: Portable shell scanner
  cannot analyze command: unterminated-quote`. Is that the deny table denying?
- Options: read it as a deny and move on | flatten the deny table again so unparseable commands run |
  keep the table nested and teach the agent command shapes the scanner can parse
- Chose: keep the table nested and teach the agent command shapes the scanner can parse
- Why: measured 5 Nov on `address_review:214`: one `gh pr view` carrying a `-q` program with `\n` escapes
  and several nested quotes, refused on every retry, `TICK-FAILED exit=124` at the 2700 s ceiling with the
  review never answered -- the reason line only because of H-0011's change to log the tick's own failure
  line. It is not a deny and it is not a bug in the command: H-0013 moved the patterns under `bash` so that
  they are evaluated, and evaluating them means parsing every command this agent types, so a shape the
  parser cannot analyse is now fatal where it used to be merely unmatched. That is the honest price of a
  working deny list and it belongs on the record, because flattening the table would "fix" the error by
  making `gh pr merge` runnable again -- the failure mode this harness was built to close. The fix is the
  shape, not the rule: one statement per call, `--json` with a short `--jq`, no escaped newlines inside a
  shell quote. The skill's own examples already have that shape; this tick had invented a worse one.
- Checkpoint: 26d4b5f8948cf19c30d5cb762651e095ab84059a
- Effect: .opencode/skills/factory/SKILL.md (Step 1)

## H-0025 · A conflict is resolved against the base the pull request names, not against `main`

- Question: `CONFLICT-RESOLVED pr:215 merged origin/main into fix/manifest-records-conditions` printed
  twice, the pull request stayed `CONFLICTING` both times, and nothing was wrong with the sentence. Which
  part is the harness getting at?
- Options: keep merging `main` and let a tick sort out the real base | merge the branch named by
  `baseRefName`, refusing rather than guessing when it cannot be read | refuse to touch any pull request
  whose base is not `main` and call it out of scope
- Chose: merge the branch named by `baseRefName`, refusing rather than guessing when it cannot be read
- Why: #215 is based on `fix/audit-row-label` -- another open pull request, deliberately, because the
  capture work needs the audit-row change that #214 makes, and the tick that stacked them said so in its
  own merge message ("this is the PR's base"). Measured 6 Nov: `git merge-base --is-ancestor origin/main
  73a65fc` answers yes -- `main` was already in the head -- so the mechanical resolver's merge was a no-op
  dressed as a fix, and the API still reported `CONFLICTING` because the conflict is against the base. The
  log line was therefore the most expensive kind of wrong: a supervisor that reports a problem closed while
  the queue hands the same unit to the next pass, which is the failure mode H-0011 and H-0022 were written
  about, arriving from a direction nobody had pointed a test at. Refusing rather than falling back to
  `main` when the base cannot be read is the same rule as H-0023: a refusal with a reason in the log beats
  a confident action on a guessed branch. The third option was rejected because stacked pull requests are
  what this harness produces on purpose -- one issue's fix waiting on another's -- and declaring them out
  of scope would leave the queue's most common shape permanently jammed. The tick's prompt moves with the
  resolver: it used to read "does not merge with main" and "the supervisor already tried `git merge
  origin/main`", which sent a model to re-verify a merge that had never been attempted, and it now names
  the base and says plainly when that base is another open pull request that may land first.
- Checkpoint: c4d7e08d7eef44445e67ea1ce79cb1d434358e24
- Effect: factory/factory.sh (`factory_resolve_conflict`, the `conflict)` arm of `factory_tick`)

## H-0026 · Repair the three entries that disagree with the gate, rather than relax the gate

- Question: `origin/main` is 42 commits ahead of this branch, and merging it turns `make verify` red --
  `D-0016`, `D-0017`, `D-0018` each have a `Chose:` that is string-equal to none of its recorded
  `Options`. Which side changes?
- Options: relax the predicate so `Chose:` may elaborate an option (with a `--self-test` case for the new
  rule) | rewrite each entry's `Chose:` down to the terse option it chose | name the chosen wording in
  each `Options:` line, keeping the elaboration where the evidence is | leave the entries and let whoever
  merges next discover it
- Chose: name the chosen wording in each `Options:` line, keeping the elaboration where the evidence is
- Why: the reviewer measured the weakenings and none of them works -- exact, substring and prefix are all
  `False` for at least one of the three -- so this is not a choice between a strict gate and a lenient
  one, it is a choice about what the record should say. Relaxing the predicate would have been a rule
  invented to fit three entries that were written before the rule existed, and every weakening is a hole
  an unlucky future entry falls into. Trimming `Chose:` would have deleted evidence: what those entries
  actually chose was a *combination* -- `D-0017` is `curfew end → 00:00` *and* `re-take the POST frames
  here`, two of the six options at once, which the terse list could not express at all. Naming the
  compound choice as a selectable option says what was really on the table and keeps the entry's claim
  that the losing options were written down. Fourth option rejected outright: the tick that inherits a red
  gate has been told to leave the tree green, so a gate that disagrees with the body of entries it
  inherits will be "fixed" by whoever is least able to judge it. Two things this exposed are worth
  keeping: the gate only ever reads its own tree, so it can be green while the tree it is about to
  inherit is red -- measurable before the merge with `git merge-tree --write-tree origin/main <branch>`
  piped into `git archive` and the checker -- and one consequence of my own was in the same measurement:
  my previous round's `D-0015` field repair auto-merged with main's own edit of that entry into one entry
  carrying two `Why:` fields, which the gate reports as "the last one wins and the first one is silently
  the one nobody reads". The merge takes main's log wholesale; the harness's own entries are already
  separate (H-0021).
- Checkpoint: 869564a7ef19fb8349afe57c6596ca74cad8343b
- Effect: DECISIONS.md (the merged log plus `D-0016`, `D-0017`, `D-0018`)

## H-0027 · Every deny pattern opens with a wildcard, because the matcher globs the whole command line

- Question: seven of the seventeen `bash` denies in `.opencode/opencode.jsonc` match nothing. Fix the
  seven, or fix the reason?
- Options: prefix only the seven measured patterns | make the whole table wildcard-led and re-derive each
  family's shape from the flags that can precede the subcommand | keep literal patterns and add a
  supervisor-side guard that greps the tick log for the commands it meant to block
- Chose: make the whole table wildcard-led and re-derive each family's shape from the flags that can
  precede the subcommand
- Why: the second time in this harness's life that a deny list was shipped without measuring what it
  resolves to, and the reason is now understood rather than patched: the matcher globs the *whole* parsed
  command line, so a pattern opening on a literal matches only a command opening on that literal.
  Measured on v2.2.5 at `869564a` with `opencode debug agent factory`: `gh pr merge*` allowed
  `gh -R vicowa-botworks/parental-lockout pr merge 218`, because `gh`'s own `--repo` flag sits between the
  program and the subcommand. That is why the fix is the *shape rule* and not the seven names -- any
  literal-led pattern added next week is the same bug -- and why the wildcard has to start at the
  subcommand (`* pr merge*`) rather than at the program. Prefixing seven and leaving the rule untaught was
  rejected for the same reason `docs/factory.md` was wrong in the last round: the paragraph describing the
  whole-line match was re-read, the patterns were not. Broadening everything to the widest glob was
  rejected too: `* tag*` also denies `git describe --tags`, which the packaging gate runs, and a deny that
  breaks a legitimate read teaches the next tick to route around denies -- so `* tag *` keeps the argument
  requirement that separates creating a tag from listing them, `* release create|delete|edit*` leave
  `gh release list` readable, and `* rm -r*`/`* rm --recursive*` replace two patterns that covered exactly
  one of `-rf`, `-fr`, `-r -f`, `--recursive --force`. `gh api` widens from a literal to `* api *` plus
  `* api`, which is what its own comment already claimed ("denied whole rather than filtered by method").
  The scanner is now named as the reason this is load-bearing rather than cosmetic: keying these patterns
  under `bash` means every command the agent types is parsed, so an inert pattern is invisible in exactly
  the way an unevaluated one was.
- Checkpoint: 869564a7ef19fb8349afe57c6596ca74cad8343b
- Effect: .opencode/opencode.jsonc, docs/factory.md, .opencode/skills/factory/SKILL.md

## H-0028 · A guard is only as real as the case that disables it, so point a case at every shape that mutes one

- Question: `check-decisions.py --self-test` returns 0 against a Makefile whose gate line is commented
  out, and the cross-log rule added in H-0021 has no case at all. What is the repair?
- Options: fix the commented-line bug and leave the cross-log rule inline and untested | fix the bug, add
  the fourth Makefile case, and extract the cross-log rule into a function so `--self-test` can assert it
  both ways | trust both rules because they are simple
- Chose: fix the bug, add the fourth Makefile case, and extract the cross-log rule into a function so
  `--self-test` can assert it both ways
- Why: `invocation_problems` split each recipe line on `;` and *then* discarded chunks beginning with `#`,
  so `#\techo "== decision log"; python3 scripts/check-decisions.py; \` put the invocation in chunk two,
  out of reach of the comment test -- the gate and its own self-test both answered 0 while `make verify`
  handed that line to no shell at all. The docstring asserted the refuted premise ("a chunk beginning with
  `#` is a comment and cannot match"), which is how a wrong belief survives: the code says one thing, the
  sentence above it says another, and only a shell disagrees. Commenting is now tested at the line, before
  the split. The cross-log rule is extracted for the same reason as the Makefile cases: review measured
  that replacing its loop with `for ident in []` left the self-test green at 12 fail / 6 pass, so
  `SKILL.md` was advertising a check nothing runs; it now has one case that must refuse (`D-0012` in both
  logs) and one that must pass (`D-0012` and `H-0012` are different decisions in different logs, which is
  the whole point of the prefix split). Three smaller guards go in on the same principle: `gh pr list
  --head ""` is not a filter and answers with somebody else's open pull request, so an empty branch would
  let the delivery check certify delivery off unrelated work; `git fetch origin <base>` exits 0 and creates
  nothing in a single-branch or shallow clone, so the base fix would die as `not something we can merge`
  and be logged as a conflict that does not exist -- exactly the misdiagnosis H-0025 was written to end,
  arriving from the clone's shape; and the stranded-branch prompt told a tick it could drop the work in a
  local log while the new delivery check charged that same tick as undelivered forever, so the prompt now
  requires the pull request and says plainly to put a "drop this" argument in its body.
- Checkpoint: 869564a7ef19fb8349afe57c6596ca74cad8343b
- Effect: scripts/check-decisions.py, factory/factory.sh
