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

## H-0029 · Resolve the issue before closing the superseding pull request, because triage's exclusion is an open PR

- Question: review of #223 said the right answer was to close it. What breaks first?
- Options: close the PR, then deal with #211 | close #211 first, then the PR | leave both open until a
  maintainer acts
- Chose: close #211 first, then the PR
- Why: `factory/triage.sh` keeps an issue out of the unclaimed queue by scanning **open** pull-request
  bodies for a close/refs/part-of mention (`:157-160`), and #223's body was the only thing holding #211
  out. Closing the PR first would have put #211 in front of a fresh lane within one activation -- a third
  implementation of a feature main has carried since 05:35Z, from a base 42 commits older, at roughly 45
  minutes of ceiling each. The order is not cosmetic and nothing in the repository says so: the same
  mechanism means a PR that *legitimately* parts of an issue is the only reason that issue is not queued,
  so any close has to look at what its body was shielding. Recorded here rather than only in the two
  close comments because it is a property of the supervisor, not of those issues.
- Checkpoint: aa9c1841a949ecd2ac4805ce28b6e0f97a658307
- Effect: none in code; the ordering is a rule for whoever closes next, and the remaining cases are in
  #224

## H-0030 · A sticky halt stays sticky, but its reason has to carry the resume command

- Question: the queue halted at 10:41Z on a dirty worktree and stayed halted through four activations,
  including one after the tree had been made clean. The reason line names the tree and nothing else.
  Self-heal, or make the stop readable?
- Options: clear `factory/HALT` automatically once the named tree is clean again | keep the halt sticky
  and write the resume step into the reason text | add `factory.sh --resume` and tell people to use it
  instead of `rm`
- Chose: keep the halt sticky and write the resume step into the reason text
- Why: a dirty worktree means a tick died in the middle of editing files, and the only thing standing
  between the next tick and a half-written file is this flag -- auto-clearing on `status --porcelain`
  empty would let a tick into a tree where a person may still be deciding what to keep, and the failure
  would look like the harness corrupting a branch. The measurable problem was legibility: from 12:37Z to
  13:11Z the journal said `HALTED a worktree was left dirty by a previous tick:
  /home/rodney/prj/wt/issue-222` and nothing more, while `rm factory/HALT` existed only in
  docs/factory.md:200, and `factory.sh --status` prints the same file verbatim (`common.sh:144`). The
  reason string now ends `-- resolve the tree (commit it to its branch, or discard it on purpose), then
  `rm <path>` to resume`, so whoever finds a stopped queue learns from the stop itself that the fix is a
  decision about the tree, not a flag to hunt for. Same commit sweeps the eight zero-byte
  `worktree-error.<pid>` files a day had accumulated in `factory/run/`: a tick killed by the ceiling dies
  between the write and the reader's `rm`, so the pass now deletes any older than three hours at startup
  -- young ones could belong to a supervisor running right now.
- Checkpoint: aa9c1841a949ecd2ac4805ce28b6e0f97a658307
- Effect: factory/factory.sh (`factory_tick`'s halt emit, the startup sweep)

## H-0033 · A deny list is re-checked by matching commands against it, not by reading it; and a whole-line glob matches inside a quote, which is documented rather than narrowed

- Question: `.opencode/opencode.jsonc` is the only thing standing between `opencode run --auto` and an
  irreversible command, and four rounds of review read it instead of testing it. How is it kept honest,
  and what is done about the fact that these patterns also match inside quoted arguments?
- Options: add `scripts/check-denies.py`, asserting every denial in both the bare and the
  flag-inserted spelling plus a short allow-list against the ruleset the CLI resolves, and write the
  quoted-argument collision into `docs/factory.md` as expected behaviour | narrow the patterns so they
  cannot match inside a quote | refuse to merge until one live tick has executed a refused command
  inside the rules | keep reviewing the list by reading it
- Chose: add `scripts/check-denies.py`, asserting every denial in both the bare and the
  flag-inserted spelling plus a short allow-list against the ruleset the CLI resolves, and write the
  quoted-argument collision into `docs/factory.md` as expected behaviour
- Why: reading the list is what produced the defect twice in opposite directions -- at `869564a` seven
  of seventeen patterns opened on a literal and could never match the flag-inserted form, and deleting
  those literals to repair it left `rm -rf`, `systemctl --user stop <the timer>` and `go get` allowed
  at `37120a8` in the bare form a model types first, which is what round four measured. Both are one
  command away now: against HEAD's own file the new check refuses exactly those six commands, and
  against the repaired file it passes 34. Narrowing is not available, because the matcher globs the
  whole parsed line and has no notion of quoting -- `git rm -r --cached legacy/dir` and
  `git commit -m "why * rm -r* is denied"` are the same bytes to it as the commands being denied, and a
  trailing space (`* rm -r `) reaches a quoted argument identically. So the collision is written down
  where the next tick will meet it, in the file whose job is to stop it routing around a denial: a
  refusal on a quoted argument is the design, and the repair is to reword the message. Two limits stay
  on the record rather than being argued away. The check models the matcher and does not exercise it --
  `opencode run --auto --agent factory` still dies `UnknownError` on the reviewing box, so no command
  has ever been executed inside these rules on either machine, and the third option remains the one
  thing that would settle the semantics; it is blocked behind this pull request, because the timer
  that starts the first tick is installed by merging it. And the ruleset has to be read from disk:
  `opencode debug agents` without `--standalone` answers from a running server's startup config,
  measured 6 Nov by replacing the file with the broken list and getting the good ruleset back -- so
  the check asks with `--standalone` and refuses any resolved pattern the file does not contain. That
  guard paid for itself at the new script's own first run, where a character class typed as `[ \t]`
  instead of `[ \t]*` matched no config pattern at all and would otherwise have made every config-side
  assertion vacuously true; it now names a broken reader as a broken reader.
- Checkpoint: 46d74ccb9dfdc35238fa9caeb92de2735d3106a1
- Effect: .opencode/opencode.jsonc, scripts/check-denies.py, Makefile (`agent deny list` step),
  docs/factory.md, .opencode/skills/factory/SKILL.md

## H-0034 · A check that needs the CLI checks nothing where the CLI is not, and a reader that cannot parse must never be allowed to hand back a verdict on the thing it reads

- Question: `scripts/check-denies.py` reads the ruleset `opencode` resolves, and a second release
  answers under different keys -- `permission` singular, `{"permission","pattern","action"}`, the tool
  named `bash` -- so the reader saw zero rules and reported the config as inert. Fix the reader, and
  what is done about the two things that made that survivable: CI has no CLI at all, and a false verdict
  here aborts `set -e` before the gates behind it?
- Options: normalise both vocabularies, turn "the reader parsed no shell denial" into its own loud skip
  naming the binary and version, and add `--self-test` -- synthetic rulesets in both shapes plus the two
  one-shaped regressions -- so the matcher is gated where no CLI exists | pin the harness to one opencode
  version and let the check skip everywhere else | make the check `CHECK_DENIES_REQUIRED=1`-only in CI so
  a missing CLI is at least visible | leave the reader as it is and document that the step is one box
- Chose: normalise both vocabularies, turn "the reader parsed no shell denial" into its own loud skip
  naming the binary and version, and add `--self-test` -- synthetic rulesets in both shapes plus the two
  one-shaped regressions -- so the matcher is gated where no CLI exists
- Why: the failure was not a wrong answer about a CLI, it was 51 imperative sentences about a correct
  file, in the voice of the rule that exists to catch mis-keying, and `docs/factory.md` tells a reader to
  confirm keying by looking for `"action": "shell"` -- which on 1.18.5 is how a good file gets "repaired".
  `make -n verify` starts `set -e;` with this step ninth, so a false verdict also silences the attack
  suite and both capture contracts behind it. Reproduced exactly rather than argued: a CLI written to
  1.18.5's behaviour against this tree's real config through the pre-fix reader printed
  `51 problem(s) ... (0 shell deny rules ...)`, 23 mis-keying accusations, 27 ALLOW-where-DENY, one
  missing `question` rule, and not one allow row failing, because a ruleset of nothing cannot fail an
  allow-list. The self-test exists because every live path here skips in CI: the patterns are a property
  of the patterns, so the table is assertable without a CLI, and only keying stays a live-only claim.
- Checkpoint: 509aaea2b7317e587f13332be50f0e397f0253f8
- Effect: `check-denies --self-test` gates 23 patterns in both shapes and nine synthetic cases in CI;
  the live pass line names binary and version; a third shape skips as a reader fault, and as a failure
  under `CHECK_DENIES_REQUIRED=1`.

## H-0035 · A merge is what the API says it is, not what `gh`'s exit status says, because `--delete-branch` fails after the merge succeeds

- Question: three pulls the journal recorded as `MERGE-FAILED` had in fact merged. Where does the
  supervisor's verdict come from, and what does it say when the two sources disagree?
- Options: read `.merged`/`.state` back from the API before deciding, and log gh's own last stderr line
  as the reason, naming a post-merge step when that is what failed | drop `--delete-branch` so gh has
  nothing left to fail at | trust gh and let a person notice the ledger disagrees | keep gh's verdict and
  re-run the merge until it exits 0
- Chose: read `.merged`/`.state` back from the API before deciding, and log gh's own last stderr line
  as the reason, naming a post-merge step when that is what failed
- Why: `gh pr merge --merge --delete-branch` returns 1 when the merge worked and the branch deletion did
  not, and a local branch held by a worktree always fails that deletion -- #221, #215 and #214 are
  `merged=true mergedBy=virtualcodewarrior` on the API against journal lines at 03:37:31Z, 05:35:30Z and
  10:41:31Z saying `gh pr merge failed`. Dropping `--delete-branch` fixes this instance and leaves the
  class: any post-merge step can fail, and an exit status is not a verdict about someone else's database.
  The class is worse than a wrong log line, because everything after the merge in that block -- the
  ledger entry, the worktree cleanup -- is skipped on a merge that happened. Three arms measured against
  the real tail: gh 1 with the API merged logs `MERGED pr:215 ... gh_exit=1 is a post-merge step, not
  the merge :: failed to delete local branch fix/audit-row-label ...` and returns 0; gh 1 with nothing
  merged returns 1 carrying gh's sentence; gh 0 is clean.
- Checkpoint: 509aaea2b7317e587f13332be50f0e397f0253f8
- Effect: a merge that succeeded is recorded as succeeded and the cleanup runs; a merge that failed
  names gh's own words instead of `gh pr merge failed`.

## H-0036 · A halt is a decision the supervisor made, so the unit reports success and the reason stays in the log

- Question: the first pass after `factory/HALT` was written left `Failed with result 'exit-code'` and
  `Failed to start` in the journal two lines under its own `HALTED` sentence. Does an intentional stop
  get a failing unit?
- Options: name the halt code `FACTORY_EXIT_HALT` in `common.sh` and list it in the unit's
  `SuccessExitStatus=`, so the exit code survives for anything that reads it and the unit stops claiming
  a crash | exit 0 from the halt arm and lose the distinct code | leave the unit failing so a stopped
  queue is loud in `systemctl --user status` | move the halt check into the unit as
  `ConditionPathExists=!/…/factory/HALT` so the pass is skipped rather than run
- Chose: name the halt code `FACTORY_EXIT_HALT` in `common.sh` and list it in the unit's
  `SuccessExitStatus=`, so the exit code survives for anything that reads it and the unit stops claiming
  a crash
- Why: this is the third instance of one defect in this harness -- `status=4/NOPERMISSION` stood in for
  `Agent not found` across 152 activations, and the ceiling printed an exit code while the session kept
  running -- and in each the unit's verdict was the only thing a reader outside the repository saw, and
  it pointed away from the cause. Red is not "loud" when it is wrong: a stop requested on purpose and a
  crash look identical, so the reader learns to ignore both. `ConditionPathExists=` was rejected because
  the halt carries a reason and a resume command, and a skipped unit runs nothing that could print them.
  Measured after the change: the same pass logs `HALTED a session is working address_review lanes ...`,
  exits 8, and the unit finishes `Finished parental-lockout-factory.service` with `Result=success`.
- Checkpoint: 509aaea2b7317e587f13332be50f0e397f0253f8
- Effect: `journalctl` shows a halt as a finished pass with its reason; the exit code is one constant in
  `common.sh`, named by both the script and the generated unit.

## H-0037 · `OnUnitInactiveSec` alone never arms after a boot, and an install that finds the timer running must restart it

- Question: nothing ran for 3h13m after the 15:58:48Z boot while the timer reported `active`. How is the
  first pass of a boot scheduled without letting a long pass stack a second supervisor behind it -- and
  what makes the reinstall actually take effect?
- Options: write both `OnStartupSec=` and `OnUnitInactiveSec=`, restart the timer in `--install` when it
  is already active, and say in the unit why each anchor exists | add `OnBootSec=` | add
  `Persistent=` back with an `OnCalendar=` | leave it and rely on a person noticing the queue is quiet
- Chose: write both `OnStartupSec=` and `OnUnitInactiveSec=`, restart the timer in `--install` when it
  is already active, and say in the unit why each anchor exists
- Why: a monotonic timer relative to a unit's *inactivation* needs that unit to have become inactive, so
  a service that has not run since the manager started has no such timestamp and the timer loads holding
  no elapse at all -- measured without a reboot on a purpose-built pair: `OnUnitInactiveSec=20s` bound to
  a never-run service sat `SubState=elapsed NextElapseUSecMonotonic=infinity`, unchanged after 30s, its
  `ExecStart` never touching its file. This systemd accepts both settings and the elapses are the union:
  adding `OnStartupSec=20s` turned `SubState` to `waiting`, gave the elapse a value, fired, and re-armed
  21s after that pass finished. `OnStartupSec` rather than `OnBootSec` because systemd.timer(5)
  recommends it for the per-user manager, which starts at first login -- anchored to the boot, a machine
  left up overnight would fire the factory the instant somebody logged in. The second half was found one
  step later: the reinstall that added the anchor wrote both lines and changed nothing, because
  `enable --now` is a no-op on a running timer and a running timer keeps the settings it was started
  with, so a fix can be true in the repository and absent from the machine.
- Checkpoint: 509aaea2b7317e587f13332be50f0e397f0253f8
- Effect: the installed timer is `SubState=waiting` with a finite elapse after a reinstall and after a
  boot, and a long pass still cannot stack; measured on this box at the reinstall that shipped this.

## H-0038 · A gate that skips on the machine doing the gating is not a gate; install the tool and let the box be red

- Question: `make verify` printed `verify: OK` twice on `bf81761` and CI returned red from both `verify`
  and `shellcheck` on the same commit. The shell step was a skip here because `shellcheck` is not
  installed, while CI runs it with `SHELLCHECK_REQUIRED=1`. Does a locally-green tree mean anything
  without the tool, and what goes in the reply to the reviewer?
- Options: install shellcheck v0.10.0 -- the CI release -- at `~/.local/bin` so this box runs the same
  gate, fix both findings, and say plainly in the PR thread that the two green gates quoted yesterday
  were green with the step skipped | report the CI result as an environment difference | widen
  `check-shell.sh` to ignore SC2015 and SC2034 so the two tools agree on what a finding is | treat CI as
  the only shell gate and stop quoting local `verify` for shell changes
- Chose: install shellcheck v0.10.0 -- the CI release -- at `~/.local/bin` so this box runs the same
  gate, fix both findings, and say plainly in the PR thread that the two green gates quoted yesterday
  were green with the step skipped
- Why: one of the two findings was a latent mislog of exactly the class this branch exists to remove --
  `gone=$(git branch -d …) && log_event BRANCH-CLEANED … || log_event BRANCH-KEPT …` runs the `||` arm
  when the *logging* in the success arm fails, so the ledger prints BRANCH-KEPT for a branch it just
  deleted -- and it was invisible to every local run, twice quoted to the reviewer as proof. A skip that
  prints nothing but `SKIPPED` and still exits 0 is how a machine that cannot check something ends up
  certifying it; the honest repair is to make the box able to check, not to argue the delta away. Both
  findings were mine, both from the merge-verdict commit, and both tools (0.9.0 in `verify`, 0.10.0 in
  the dedicated job) agreed.
- Checkpoint: 2a095564d215e8e6293b43af3d27e21b84160462
- Effect: `check-shell` here prints `0.10.0 over 26 scripts` and runs rather than skipping; the SC2015
  site is an if/else so the arm is chosen by the deletion verdict alone; `FACTORY_EXIT_HALT` keeps one
  home with the consumers named in the directive that silences the true positive.

## H-0039 · `--log` skipped the two rules that would have caught a fabricated checkpoint

- Question: a checker with two invocation modes has one set of rules; which mode runs which, and who says so?
- Options: the modes genuinely differ and the usage text names the difference | the rules are one set and `--log` only narrows the file | drop `--log` and always check both logs
- Chose: the rules are one set and `--log` only narrows the file
- Why: `checkpoint_problems` and `verify_invocation` moved out from behind `if path == LOG`, and the usage text now states what the code does. The two hidden rules are the only thing that catches an invented `Checkpoint:`, and `--log` is the mode an agent reaches for when it wants one branch's log checked. Measured on the same script against the same file: `--log DECISIONS.md` answered 3 problems with a fabricated sha absent, no argument answered 5 with it named. Four passes over #229's log in that mode is how its mis-transcribed checkpoint survived to be found by a reviewer instead. The usage text had it the other way -- "and only the rules that live inside an entry: checkpoint existence" -- so this is a documented-hazard contradiction as well as a bug: the promise is what made the silence look like a clean log. The duplicate-id rule is the honest exception and the text now says it is one: it compares logs, so over one file it can only find a repeat inside that file.
- Checkpoint: 1c0c89e99d69b7bfa4cab2382c91da85e9b3100e

## H-0040 · A guard that refuses working code is a guard that gets widened away

- Question: what does the `"$DRY_RUN"`-runs-as-a-program class actually cover?
- Options: any `$VAR` in command position | only a variable that only ever holds 0 or 1 | only the literal name `DRY_RUN`, matched by string
- Chose: only a variable that only ever holds 0 or 1
- Why: the chosen rule reads every assignment in every shipped script, so a name that holds anything else anywhere is not refused at all. The first draft took the first option, as the finding asked, and the tree's first run produced six false positives -- `testing/capture/fixture.sh:117` and `testing/e2e/e2e.sh:188` among them -- because `"$PLOCK_BIN" version` is the idiom for running a binary whose path the caller chose. A gate that refuses working code does not get fixed in the code; it gets widened, ignored, or deleted inside a week, and then the two real sites are unguarded again. The narrower rule still refuses both shapes that shipped (`if ! "$DRY_RUN"; then`, `"$DRY_RUN" || systemctl --user restart`), and its `--self-test` asserts the false-positive case as a first-class requirement: `"$PLOCK_BIN" version` staying green is now as much a failure to break as the bad line passing.
- Checkpoint: 1c0c89e99d69b7bfa4cab2382c91da85e9b3100e

## H-0041 · The dirt test asked git a stat question, so the queue could halt on work that was not there

- Question: what does the pass ask git when it decides a tree has somebody else's work in it, and what does it do when git will not answer at all?
- Options: `git status --porcelain`, one read, failures hidden by `2>/dev/null` | a content diff plus the untracked list, which is what the issue prescribes | four content reads, each checking its own exit status, with a failure halting as UNKNOWN
- Chose: four content reads, each checking its own exit status, with a failure halting as UNKNOWN
- Why: the issue's own prescription is the middle option and it has a hole the size of the reason the guard exists. A change that is staged with the worktree equal to the index -- the state of a merge that stopped for a decision, which is half of what this halt refuses -- is invisible to `git diff` and to `ls-files --others`: `factory/selftest.sh` prints the verdict for that leg as `DIRTY` with `naive:CLEAN` beside it, where `naive` is the middle option computed on the same tree. So the staged read is not redundant with the two the issue names, and the guard is wider than requested on purpose. The exit-status checks came from the destructive case, not from tidiness: pointed at a `.git` that holds nothing, or at an inherited `GIT_DIR`/`GIT_WORK_TREE` pair, `git status --porcelain` exits 128 printing nothing on git 2.53.0, `$(... 2>/dev/null)` turns that into an empty string, and the old test becomes `test -n ""` -- the tick works in a tree git would not describe. Pointed at a `core.excludesFile` that is a directory, three of the five reads die and two answer clean, so a verdict assembled from whichever reads answer says there is no work. **Corrected by H-0045, measured on one git with only `$TMPDIR` moved:** that last sentence is false in both of its claims. The excludesFile arm is a filesystem property, not a git one -- git reads `st_size == 0` as an empty exclude list and only a non-zero size reaches the `read()` that dies with EISDIR, and a directory reports size 0 on btrfs and 40 on tmpfs -- so on btrfs *no* read dies, the patterns are dropped in silence, and the verdict answers DIRTY naming a file the operator had told git to ignore. And on the filesystem where the plant does refuse, two reads die (`diff --cached`, `ls-files --others`) and three answer clean, not three and two; `factory/selftest.sh` prints all five statuses on the leg so the count is never re-derived by hand again. The rest of this entry stands: the empty `.git` and the poisoned `GIT_DIR`/`GIT_WORK_TREE` pair refuse on every filesystem tried, including btrfs. That is the direction that costs something: a false halt costs one pass and is retried on the next tick, a false clean costs the half-finished work. What lost is the single read. It is one process where this is five, and `status` is the only command here that formats renames and submodule state, so whoever needs those adds an arm rather than inheriting one -- which is why `docs/factory.md` now names the four reads instead of saying only "dirty". Also measured, against the report that prompted this: the stat-only difference the ticket describes did not reproduce in six planted variants on this git, which refreshes the index in memory before it reports, and none of `status`, `diff`, `diff-files` or `git describe --dirty` reported a tree dirty for it; the probe says that on its own line rather than pinning a leg that cannot fail, and the legs that do discriminate are the staged, unmerged, untracked and ignored ones. What did reproduce is the opposite direction, in the sentence above.
- Checkpoint: 5591286
- Effect: `factory_tree_dirt` in factory/common.sh, both call sites in factory/factory.sh, and factory/selftest.sh wired into `make verify`, where it refuses if that step is deleted

## H-0042 · A killed sweep put the bytes back and left the timestamp of the restore behind

- Question: when `check-mutations.py` restores a file it mutated, is the promise about the file's contents or about the state of the tree?
- Options: the contents only, so the filesystem keeps the restore's timestamp | capture each file's full stat before mutating and replay it on restore | move the file's mtime to the value git's own index entry already holds, and only where git reports no content difference
- Chose: move the file's mtime to the value git's own index entry already holds, and only where git reports no content difference
- Why: restoring by writing the original bytes leaves the content where it started and the filesystem somewhere else. Measured on git 2.53.0, a box committed at `aaa`, mutated to `bbb`, restored with the same bytes: the index entry still says `mtime: 1794103571:46218175` while the file says `(1794103571, 51538821)`. That divergence is the fingerprint of a sweep that ran, and it survives a kill precisely because writing bytes back is the last thing that happens. Said plainly, because it bounds what this is worth: on this git no reader tried reports the tree for it -- `status --porcelain`, `diff --stat`, `diff-files --raw` and `git describe --always --dirty` all re-read the bytes when the stat data disagrees, and answered clean -- so this is not the halt in #228, which H-0041 fixes, and the probe in `factory/selftest.sh` says the same about its own stat leg. It is the class the ticket's title names, removed for the cost of one `utime` rather than left for the next filesystem with coarser timestamps. Writing a timestamp onto a file this tool does not own is dangerous exactly one way: some reader trusts the cache and calls a changed file clean. Measured the other way round on purpose -- cached mtime and size restored exactly, content one byte different at the same length, the only shape that can slip a stat cache -- `status` still said ` M x.go`, `diff-files --raw` said `M`, `git describe --dirty` said `-dirty`; the guard is additionally asserted on the timestamp itself, because a git that re-reads would report the same verdict with the guard deleted. The value written is git's own, not a timestamp captured before the run, which keeps the journal format untouched -- `--restore` still replays a journal an older version wrote, and the journal is the crash path, where a format change is worse than a stale timestamp -- and lands the entry in git's racy window rather than in the past, which is the value a stat cache is built to trust. `git update-index -- <path>` was measured and not used: for a content-identical restore it is a no-op in all three spellings (`--refresh`, `--really-refresh`, plain), so it cannot clear what it does not rewrite, and the plain form STAGES a file whose content did change -- a deeper intrusion into a tree this tool is only borrowing, and one the guard could not undo. `dirty_paths()` and the corpse note keep their `git status --porcelain` on purpose: that sentence answers "is this mutant a corpse in the tree you are standing in", for a person who is standing in their own tree, and it must report the same dirt their own `git status` reports. What lost: the pre-run mtime itself. A file that differs from the index keeps the newer timestamp, because a difference in content is not something a timestamp may be adjusted over, so the tree is byte-clean and not state-identical, and anyone auditing a sweep by `ls -l` sees the restore's time on the files git agrees about and the edit's time on the ones it does not.
- Checkpoint: f09eae24cf60c3a3bd959259c92eaa0122247f91
- Effect: `settle_stat_cache` in scripts/check-mutations.py, called from both restore exits and from `--restore`; `EXPECTED_RULES` 95 -> 99, two rules for the two exits, one `rule_fires` control that re-planting the bytes by hand still leaves the fingerprint, and one that drives the destructive case through the function itself

## H-0043 · The probe refreshed with an option git refuses, and its stat leg never asked whether it had planted anything

- Question: the stat leg of `factory/selftest.sh` asserts a verdict about git's stat cache. Is the cache actually disagreeing at the moment the verdict is asked, and who checks the git invocations the probe itself makes?
- Options: leave the leg as written, trusting the write-back to have left a fingerprint and the refresh before it to have run | measure the fingerprint with `git ls-files --debug` before asking the verdict and fail the leg when it is absent | measure it, and additionally refuse the refused `update-index` spelling anywhere in the shipped scripts
- Chose: measure it, and additionally refuse the refused `update-index` spelling anywhere in the shipped scripts
- Why: both refreshes in the fixture were written `git update-index --quiet --refresh`, and on git 2.53.0 that is not a refresh at all -- measured side by side in one repository: `--quiet --refresh` exits 129 with `error: unknown option 'quiet'` on stderr, `-q --refresh` exits 0, plain `--refresh` exits 0. Both call sites carry `>/dev/null 2>&1 || true`, which is what made it invisible: the reason is destroyed and the line reads exactly like a refresh that ran. It changed no verdict -- `factory_tree_dirt` refreshes correctly on its own, and deleting the fixture's pre-plant refresh leaves all 32 legs green (measured) -- so this is not a wrong answer, it is a probe whose comment claims a state it never settled, in the one leg whose entire subject is that state. The same one word written into `factory_tree_dirt`'s own refresh is catastrophic instead of merely dishonest: 129 is that read's exit status, so the first arm returns UNKNOWN and every tree in the pass, every clean one included, halts the pass. That is what the shipped-text check is for, and it is why the guard is a leg rather than a note: nothing at run time would ever say so. The leg now measures the fingerprint instead of inferring it from the write that was meant to cause it -- git's cached mtime for the path against the file's own -- and it measures *before* `expect`, because the verdict's own refresh absorbs a new mtime into a writable index (measured: after a plain `--refresh`, cached and file agree), so asking afterwards finds no fingerprint whatever the tree looked like when the verdict was formed. Each half is compared as a number: git prints `44033527` and `date +%N` prints `044033527`, the same instant, and a string comparison reports a fingerprint that is not there -- the precise failure in which a leg believes it planted something and passes while pinning nothing. Discrimination measured in a scratch clone: delete the write-back and the touch and four legs go red, both trees times both files; write the refused spelling back into `reset_tree` and the shipped-text check goes red in the repository scan, which is the real path, not the planted control; delete the pre-plant refresh and nothing goes red, which is the honest statement of what the leg pins -- the state, not the command that was supposed to produce it. The text check cuts everything after the first `#` before it tests, because its first three hits were prose: `factory/common.sh`'s own note quoting "`update-index --refresh`, `diff --quiet` and `ls-files --others` exit 128", this file's table of exit statuses, and the new comment here that quotes the 129 to explain it. A gate that refuses a comment is a gate that gets widened away inside a week, and H-0040 on this very file's other guard is that lesson already paid for. Both structural checks gained a control in the opposite polarity -- the wiring scan re-run on the recipe with the step's line deleted has to say absent, the spelling scan re-run on a planted file has to say present -- because a clean line is only evidence when the check can also be un-clean. Cost: two `ls-files --debug`, two `stat`, two `date` per tree, 0.31 s to 0.34 s for the whole step, still under the 0.4 s the Makefile records; five more legs, 27 -> 32. What lost: the text check is textual, so a refused spelling assembled at run time or left inside a comment after a shell command on the same line walks through it, and the fingerprint check trusts `ls-files --debug` to print the on-disk cache rather than a value refreshed in memory -- measured that it does (a same-bytes write leaves cached `1794109272:991140337` against a file at `1794109273:…` while `status` answers clean), but that is one git's behaviour, exactly the contingency the stat leg itself already carries a note for.
- Checkpoint: f61eb639137c66adc7eeea87a028e3933916b843
- Effect: `planted()` and `check()` in factory/selftest.sh, the two planted legs in each tree, `bad_refresh_calls()`/`refused_refresh_spelling()` with its planted control, the same control added to `wired_into_verify`, and both fixture refreshes spelled the way git accepts

## H-0044 · The halt is factored into one guard the probe drives, because nothing had ever executed either copy of it

- Question: #228's second requirement is that the probe assert the tick does not halt on stat-only dirt and does halt on a real edit. The legs added under H-0041 drive `factory_tree_dirt` and stop there. Does anything execute the decision -- the lines in `factory.sh` that turn a verdict into a halt -- and if not, is the shared-guard refactor #228 forbids (H-0040's rule against re-implementing a decision in a script) the right way to make it reachable?
- Options: keep the two copies of the predicate in `factory.sh` and test only the verdict | factor the halt into one guard in `common.sh` that both call sites delegate to, and drive that guard from the probe over the planted trees | factor it and additionally pin the call sites' shape in the shipped text, since a site can delegate and then discard the answer
- Chose: factor it and additionally pin the call sites' shape in the shipped text, since a site can delegate and then discard the answer
- Why: the audit was run before choosing, and it is the whole argument -- three mutations of the halting lines each left every leg green: flipping both predicates from `-ne 1` to `-ne 0`, flipping both to `-eq 0`, which lets UNKNOWN through, and deleting one call site outright. The probe drove `factory_tree_dirt` directly, so the code that decides whether a model is sent into a tree was executed by nothing, and a verdict that is right is worth nothing at a site that misreads it. Factor rather than mock: #228 asks for the halt at the guard, and `scripts/mutate.py`'s `rule_rejects` proves a probe can call a function out of `factory.sh`, but the predicate lives inside two large functions that also shell out to `gh`, so reaching them end-to-end means a fake `gh` and a fixture the repository has no precedent for -- and the Makefile comment above this very step says a gate no one runs is a suggestion, which applies with more force to a fixture nobody can reproduce. The guard returns 0 to proceed and otherwise the caller's own code, `4` for a tick and `2` for the conflict pass, chosen by the verb it is handed; the mapping is now one `case` the probe asserts for both paths instead of a `return 4` and a `return 2` nobody runs. The legs assert the four things a halt has to get right, not just the boolean: the exit code, that `factory/HALT` exists (because `factory_halted()` reads the file and nothing else -- a guard that returns 4 and writes no file stops one unit and lets the pass continue), that the sentence inside it names the resume step and the action matching the finding, and that each path journals its own verb. Discrimination measured over ten mutants in a scratch clone, every one caught by the leg that owns it: never halt, 17 red; halt on a clean tree, 23; UNKNOWN proceeds, 3; the two codes swapped, 17; no HALT file, 17; the dirty-tree advice given to a tree git cannot read, 3; both paths journalling the tick's verb, 17; a call site dropping `|| return $?`, the shape check red; a call site deleted, red; a duplicated predicate reintroduced, red. Two of those ten survived at first and both were the probe's own defects, which is the lesson worth keeping: a mutation written with `'\t\t…'` in single quotes hands python a backslash-t, the pattern is never found, and the run prints SURVIVED for a mutant that never existed -- so a pattern-not-found now aborts the whole probe; and replacing the first `log_event "$verb` in the file attacks the TICK arm, where `$verb` already reads `TICK-HALTED`, an equivalent program that survives for the right reason and says nothing about the arm it was meant to attack. Cost: eleven planted states times two trees now run the guard twice, roughly 260 more git processes, the step 0.34 s -> 1.0 s, legs 32 -> 56. It also fixes a smaller bug in the same family on the way, which sharing is what makes impossible to duplicate: the conflict pass wrote a HALT file with neither the UNKNOWN-versus-DIRTY distinction nor the matching action, so an unreadable tree found mid-merge was told to commit or discard work that cannot be seen; it now says the same thing the tick says, and its sentence loses "and the supervisor will not touch it". What lost, stated as measured: nothing runs `factory.sh`, so the codes are asserted at the guard and not observed from `factory.sh --once` on a dirty tree -- that needs the GitHub API, and is the honest boundary of this probe -- and the call-site rule is textual, so a site that saves the code into a variable and returns it a line later walks straight through it.
- Checkpoint: cd78fcc9eb1356dedd33adbea1b1ea51ee52113a
- Effect: `factory_tree_halt` in factory/common.sh; both halting call sites in factory/factory.sh reduced to one delegated line each; `expect_halt`, `halt_call_sites_in`, `halt_call_sites_shaped` and the `FACTORY_*` redirect into the fixture box in factory/selftest.sh; the "What dirty means" section of docs/factory.md

## H-0045 · The exclude leg asked git a question the filesystem answers, so `make verify` was red on btrfs and green on tmpfs

- Question: the `core.excludesFile`-is-a-directory leg asserts UNKNOWN unconditionally, and on the review box it answered CLEAN. Is there a plant that makes git refuse on every filesystem, or does the leg have to stop asserting a refusal it did not measure?
- Options: keep `expect … UNKNOWN` and skip the leg where a directory has no size | re-plant the same idea harder, as a shape git refuses everywhere | ask git first whether the plant made any of the five reads refuse, then assert only that arm's verdict and print a note naming the filesystem when the refusal is unreachable
- Chose: ask git first whether the plant made any of the five reads refuse, then assert only that arm's verdict and print a note naming the filesystem when the refusal is unreachable
- Why: reproduced on one box before choosing, with only `$TMPDIR` moved: `56 passed, 0 failed` on tmpfs, `55 passed, 1 failed — an unreadable exclude file: got CLEAN, want UNKNOWN` on btrfs, same commit, same git 2.53.0, so `make verify` -- the first gate `factory/` ever has -- is red or green depending on where the fixture was written. The mechanism is git's `add_patterns()`: it opens the exclude file, `fstat`s it and branches on `st_size`, taking 0 for an empty list and returning, while a non-zero size reaches `read_in_full()` on a directory fd, which is EISDIR and `fatal: cannot use %s as an exclude file` at 128. A directory reports `st_size` 0 on btrfs and 40 on tmpfs (both measured here; 4096 on ext4, measured by review on #231 on a box that has it), so the refusal is a filesystem property wearing a git property's clothes. The second option was probed and not assumed: with a two-file repo and one git, `core.excludesFile` aimed at a missing path, a non-directory, a `chmod 000` file, and an empty file all answer `rc=0` with the patterns dropped and a warning at most, and only the directory refuses -- where it refuses -- so no plant exists that git refuses portably and the leg cannot be repaired harder. The chosen shape is `planted()`'s bargain from H-0043 applied to the leg that was still inferring: `excludes_reads()` runs the five reads and returns whether any exceeded 1, and each arm asserts what that answered. The leg also plants `excluded-by-the-global-file.tmp`, deliberately absent from `.gitignore`, because without it the btrfs arm asserts a CLEAN the plant has nothing to do with -- the tree was already clean -- which is the leg certifying the verdict it was written to refuse; with it, that arm asserts what the dropped pattern list really does: the operator's own ignored file is named as untracked and the pass halts over work that is not work, which is the false halt of the class H-0041 exists to refuse and which git says nothing about in this shape. Discrimination, each mutant run on both filesystems against backups taken after the fix: hollow all five exit-status checks and the step is 52/4 on tmpfs with this leg among them (`got DIRTY, want UNKNOWN`) -- the leg still pins the fix wherever the refusal is reachable; keep the measurement but hard-wire the arm to the refusal branch and btrfs goes 55/1 with `got DIRTY, want UNKNOWN`, the review's blocker returned and caught; remove the planted file and btrfs goes 55/1 with `got CLEAN, want DIRTY`, so the arm's verdict is caused by its plant and not incidental. Same commit fixes the three places that repeat the unqualified claim, because a reader on btrfs would otherwise chase a fatal their git never prints: `docs/factory.md`, this file's note in `factory/common.sh`, and H-0041 -- whose excludesFile sentence was wrong twice over, since on the filesystem where the plant *does* refuse, measured here, two reads die (`diff --cached`, `ls-files --others`) and three answer clean, not "three … and two". What lost, stated as measured: on btrfs the leg no longer pins the fix at all -- it pins the pattern-drop and prints a note saying the refusal is unreachable there, so a box on that filesystem has no coverage of the status-checking arms from this leg and a green run must not be read as having it; the leg's label differs per filesystem (`an exclude file git refused` / `an exclude file git swallowed`) so two boxes print the same tally with different line names; and the probe is still a probe of the box it runs on, which is the same contingency the stat leg already carries, now asserted rather than assumed.
- Checkpoint: bfe7a73360cbd2ddece7a42ab5520d89109d9b1f
- Effect: `excludes_reads()` and both arms of the exclude leg in factory/selftest.sh, the `excluded-by-the-global-file.tmp` plant, the filesystem qualifier in the `factory_tree_dirt` comment and in `docs/factory.md`, and a correction inside H-0041's own entry


## H-0046 · The verdict threw git's reason away with `2>/dev/null`, so a false halt arrived with no diagnosis and a capped finding read as a complete one

- Question: all five reads in `factory_tree_dirt` run `2>/dev/null` and every finding names at most three paths with nothing to say that the list was cut. The whole thesis of this branch is that the halt must print what the verdict actually saw -- so does the sentence owe the operator git's own words and the size of the list, or is the verdict finished once the code refuses correctly?
- Options: leave the sentence alone, since the verdict's answer is already correct in every leg | keep git's first stderr line and append it to the sentence on the arms that refuse (`rc > 1`) only | keep the first stderr line of any arm that printed something, refusal or not, and mark the path list as capped with the count it left out | append everything git printed, from every read, uncapped
- Chose: keep the first stderr line of any arm that printed something, refusal or not, and mark the path list as capped with the count it left out
- Why: the second option was measured and rejected, and the measurement is the reviewer's consequence on #231 made concrete. In the fixture box on git 2.53.0 as uid 1000, with one file that only `core.excludesFile` would hide, aimed at four shapes of exclude file and read the five ways the verdict reads it: a `chmod 000` file and a self-referential symlink both leave **all five reads at `rc=0`**, print `?? excluded-by-the-global-file.tmp`, and say `warning: unable to access '…': Permission denied` and `'…': Too many levels of symbolic links` respectively; a dangling symlink and a missing path print the same listing and say **nothing at all**. So the false halt of H-0045 -- the pass stopped over a file the operator told git to ignore -- is a verdict that answers DIRTY at exit status 0, and a rule of "carry the reason when a read refuses" carries nothing for it, because nothing refused. That is why the arm rule is "printed something", not "refused", and it is pinned in the opposite direction too: mutant M4 keeps the refusals and drops the reason from the finding clauses and goes 65 passed / 3 failed, all three on the warning plant. Choosing the first line rather than everything is because the sentence this feeds is one line in `factory/HALT`, and choosing the third option over the fourth is why `factory_git_said` caps at 200 characters *and prints the width it cut* -- a silent cut is the thing being fixed, so this function does not get to commit it. The cap marker is the same defect one line further on: ten problems arrived as three names and a trailing space, which reads as a complete list, and the marker's own polarity matters -- M3 (no marker) is 64/4 with the five-file legs red, M6 (marker printed on a complete list) is 66/2 with the three-file check red, so the cap is asserted honest from both sides rather than trusted. The bug found on the way is the reason the invariant is now named: `update-index --refresh` prints `path: needs update` on its **stdout**, so swapping its `2>/dev/null` for the capture while dropping `>/dev/null` put git's chatter in front of the finding, which split HALT's sentence across two lines and made the tick's journal line stop being the last line of the journal -- 14 legs red, and the message blamed the journal, naming nothing. `expect_halt` now asserts the HALT file is exactly one line and `finding_one_line()` asserts the verdict prints exactly one, both red under M1 (54/14), so the next person to widen a redirect meets the cause and not the symptom. What lost, stated as measured: one `mktemp` and one `rm` per verdict, and when `mktemp` fails the reads are silenced exactly as they were before -- the reason is diagnosis, never evidence, so a box with no `/tmp` must not answer differently; the fallback for that case points the capture at `/dev/null` and removes nothing there, because a root run that `chmod`'d `/dev/null` is already in this repository's history. The reason is git's wording, so every leg asserts this repository's own marker `git said:` and prints git's sentence beside it, and a git that refuses without speaking takes the note-and-assert-the-verdict-only path that H-0045 built, which is why `EXCLUDE_WHY` now exists. A finding with several contributing arms carries a reason per clause and so gets longer. The cap is still a cap: 400 untracked paths arrive as three names and a number. And the legs remain a probe of the box they run on -- tmpfs 68 passed / 0 failed, btrfs 67 passed / 0 failed, the one leg's difference being the exclude arm that only exists where a directory has a size.
- Checkpoint: d9c126ef7f76477e63481fed0410741e97c2b095
- Effect: `factory_tree_dirt` split into the wrapper that owns the scratch file and `factory_tree_dirt_read`, with `factory_git_said()` and `factory_path_list()` beside it and the reason clause on all five arms in factory/common.sh; in factory/selftest.sh, `finding_one_line()`, `finding_uncapped()`, `verdict_names()`, the three-then-five untracked plants, the symlink-loop exclude plant, `EXCLUDE_WHY` in `excludes_reads()`, the `git said:` assertions on both exclude arms, and the HALT-file line count inside `expect_halt`. Legs 56 -> 68 on tmpfs, 67 on btrfs; mutants M1 54/14, M2 63/5, M3 64/4, M4 65/3, M5 (stderr back to `/dev/null`) 63/5, M6 66/2, each restored from a backup taken after the fix with the tree confirmed unchanged


## H-0047 · The shipped-text guard only knew the two shapes named in the ticket, and a scan with nothing to find could end the gate in silence

- Question: `halt_call_sites_in` refuses a halting call site that drops `|| return $?` and a second copy of the predicate named `dirt_rc` or `tree_is_dirty` -- both drawn from this ticket's own bug. Should the guard also refuse the expression this branch deletes, `test -n "$(git status --porcelain)"`, wherever a shipped script calls it?
- Options: leave it, because the two shapes are the ones the ticket filed | refuse `status --porcelain` anywhere in the repository except this probe | refuse it in `factory/*.sh`, the supervisor's own code, exempting this probe by path, cutting comment lines, and refusing a scan that cannot see the directory it was pointed at | refuse it in `factory/*.sh` and also in `scripts/` and the `Makefile`
- Chose: refuse it in `factory/*.sh`, the supervisor's own code, exempting this probe by path, cutting comment lines, and refusing a scan that cannot see the directory it was pointed at
- Why: the first option is what let the filed defect through. Measured on the head commit as it arrived: `factory/factory.sh` carried no `status --porcelain` at all, so the guard printed `ok` over the file that held the very expression this branch exists to delete, written as `if [ -n "$(git -C "$tree" status --porcelain 2>/dev/null)" ]; then return 4; fi` -- a stat-cache question in a halting path, and 128-printing-nothing on the exclude-file shapes H-0045 measures. A scan drawn from a bug report is a scan for that bug, and #228 is the proof that this repository's halting paths can lose the expression without anyone noticing. The wider options were measured before refusing them: `scripts/package-lib.sh:32` runs `if [ -z "${ALLOW_DIRTY:-}" ] && [ -n "$(git status --porcelain)" ]` as a release-packaging guard where asking the cheap question and letting a human override it with `ALLOW_DIRTY` is the intent, and `testing/capture/manifest.py` compares a screenshot manifest against `git status` -- different subject, different question -- so a guard that fires in both places is a guard that gets widened away, which is H-0040's lesson on this repository's other gate. The probe is exempt by path because its job is to print what the replaced expression answers, and that exemption carries its own control so it cannot become a blind spot: M7 plants the deleted expression as code in the real `factory/factory.sh` and exactly one leg goes red (69 passed / 1 failed), while M8 plants the same text in a comment in the same real file and the step stays green (70 passed / 0 failed) -- a guard that refused prose would be deleted by the next person who explained the rule. The directory guard came out of a mutant rather than from argument: pointed at `$1/faktory` the scan answers "no hits", which is indistinguishable from clean, so `[ ! -d "$1/factory" ]` now prints a reason and M9 is 69/2 with that reason in the output. The larger half of this entry is what the sweep surfaced on the way: the step runs `set -euo pipefail`, both shipped-text scans are invoked as `hits=$(scan "$dir")`, and a scan whose first `grep` matches nothing returns non-zero, so the assignment ends the script on that line -- no FAIL line, no tally, exit 1. That was already true of `bad_refresh_calls`, merged one round ago; it survives today only because this repository's comments quote the bad spelling, so the first grep always matches, and the day somebody rewords that prose the guard stops running in silence. Both scans carry `|| true` now, and `scans_cannot_kill_the_step()` asks each for its status on a tree holding one ordinary script and nothing to find -- M10 removes both `|| true` and names both scans, M11 removes one and names exactly one, which is the per-scan reporting being real rather than a fixed message. What lost, as measured: the pattern is one expression, not a theory of dirt -- `git status --short`, `diff-index HEAD`, or a bare `update-index --refresh` deciding a halt are all still invisible to it, and the next new spelling needs a human again; the exemption is by path, so a second copy of the halt written inside `factory/selftest.sh` itself would go unseen; the comment cut is per line and from the first `#`, so an invocation that sits on the same line after a `#` inside a string is not seen -- the same rule `bad_refresh_calls` runs on, accepted there and here; and `|| true` means a scan that fails for an unrelated reason answers "nothing" instead of failing loudly, which is exactly why the unreadable-directory case is a separate condition and never part of the pipeline.
- Checkpoint: ee6326d89350236947ffc173d55d122dd677c43a
- Effect: `naive_dirt_tests()` and `naive_dirt_test_refused()` with its two opposite-polarity controls, `scans_cannot_kill_the_step()`, and the `|| true` and its comment on `bad_refresh_calls`, all in factory/selftest.sh; legs 68 -> 70 on tmpfs, 67 -> 69 on btrfs; M7 69/1, M8 70/0, M9 69/2, M10 69/1, M11 69/1


## H-0048 · The unreadable-file leg accepted any answer but clean, so four of the five refusal arms were unpinned and the comment's number for that was wrong

- Question: `expect "an unreadable file" … HALT` passes on anything that is not CLEAN, and the redundancy claim in `factory_tree_dirt`'s comment -- hollowing the tracked read's check "turns two legs red" -- measured 0 red at 8015951. Is the halt the whole contract and the number merely stale, or does the leg have to pin the refusal it exists for?
- Options: leave the leg at `HALT` and correct the comment to "0 red" | tighten the leg to `UNKNOWN` with the refusal's own words as the needle, behind a runtime probe that this git refuses on the plant at all | add a plant and a leg per arm so all five are pinned by name | cut the four arms that no leg pins and refuse on the tracked read alone
- Chose: tighten the leg to `UNKNOWN` with the refusal's own words as the needle, behind a runtime probe that this git refuses on the plant at all
- Why: the third option is what the reviewer's standard asks for and it is not reachable, which is what measuring instead of asserting found. **Superseded in part by H-0049: the option became reachable, and the sentence below claiming the review's numbers "described ALL arms hollowed" is false -- see that entry's correction bullet. The leg tightening chosen here stands.**
  Hollowing means replacing one arm's refusal test with a constant-false test, and the first sweep of this did not do that: it used `[ "$rc" -gt 1 ]` → `[ "$rc" -gt 99 ]`, which is still true, because a refusal is 128 -- five runs, five times 70 passed / 0 failed, measuring unmutated code, the same failure #1048 records for a mutant whose pattern no longer matches. The cure is the same too: prove the mutation bites before believing its tally, here by re-running the plant directly and watching the verdict change from `its tracked files could not be read (git diff exited 128)` to `modified: keepme.txt (git said: … Permission denied)`. Then, at 8015951: hollowing any ONE arm left 70 legs green for four of the five, and the reason was never the arms -- it was the leg. Both answers halt, so `expect … HALT` was satisfied while the sentence degraded from "its state could not be read" to "commit it to its branch, or discard it on purpose" about a file git could not open; the degraded sentence is the harm the branch is about, and it is invisible to a leg that only asks whether something stopped. Measured again at cf87ecc for the record -- 56/0 hollowing arms 1, 3 and 5 singly, 52/4 with all five gone -- so the claim "four of the five legs go red when its check is hollowed" describes ALL arms hollowed, never one, and the consequence on #231 that it did not reproduce was correct. After the tightening the matrix is: arm 3 alone → 4 legs red (both trees, both assertions), arms 1, 2, 4, 5 alone → 0, arms 2-5 → 5, all five → 8. Four arms stay unpinned by name and no plant in this suite can pin them, because each shape that stops one of those four stops another as well -- the exclude-file plant stops `diff --cached` and `ls-files --others` together, the inherited `GIT_DIR`/`GIT_WORK_TREE` pair stops the refresh and `--others` together, a poisoned `.git` stops all five -- which is what an unreadable tree actually does, and why refusing on the strength of one read was never an option (the fourth option, cut to one arm, is refused by the same table). Searching for the missing single-refuser plant is what found #232: with a directory at mode 000 inside the tree, all five reads exit 0, two of them print `Permission denied` on stderr, and the verdict answers CLEAN over an untracked file. That is filed, not fixed here, because "exit 0 but wrote to stderr = unreadable" would turn H-0045's documented DIRTY on the exclude warning into UNKNOWN and that is a halt-semantics trade with a false-halt cost of its own. What lost, as measured: the leg no longer accepts a git that reports the unreadable file as dirt -- it prints SKIP with the git version and the arm goes unpinned on such a box, which is honest and is also less; the looseness in `expect` itself is left in place -- `HALT` still means "anything but clean" there, because a leg whose subject is only the halt should not over-assert -- and this was the last leg asking a *verdict* that way: the suite is now 18 verdict legs (11 DIRTY, 4 UNKNOWN, 3 CLEAN) and 16 `expect_halt` calls, which ask a different question (exit 4 and 2, and a one-line HALT file); the tallies in the comment and in `docs/factory.md` are counts of this 70-leg suite at one commit and go stale as legs are added, so both carry the sha they were measured at; and the st_size law both files were carrying is corrected in the same pass -- "a directory is 0 on btrfs and 40 on tmpfs" was only true of an EMPTY directory (measured: btrfs 0/2/12 for empty, one entry named `x`, the same named `inside`; tmpfs 40/60), so the swallow needs an empty directory on a filesystem that sizes empty directories at zero, and the ext4 4096 figure this comment asserted is the reviewer's from a box with ext4, which this machine does not have (`findmnt`: btrfs, vfat, fuseblk, nfs, overlay).
- Checkpoint: e224f348982874d4e6339dee3828f24b6587be3e
- Effect: `tracked_read_refuses()` and the tightened unreadable leg in factory/selftest.sh, and the empty/non-empty rows in its exclude-leg table; in factory/common.sh, the corrected st_size paragraph, the arm-by-arm matrix table with the wrong number and why it was wrong, and a new paragraph on what the refresh writes (stat data only; a conflict survives it at three stages with `MERGE_HEAD` present; a held `.git/index.lock` is the one shape of busy it catches, where the other four reads and `status` all answer 0); the same two corrections plus the refresh line in docs/factory.md; issue #232 opened for the unreadable-directory CLEAN with its exit-status table and reproduction


## H-0049 · Four refusal arms had no plant that isolated them, so the isolation is built rather than found

- Question: after H-0048, one arm of five was pinned by name and four were pinned only for the halt. No state on disk makes exactly one of the five reads refuse, so is that the limit of what the probe can say about those four arms, or does the isolation get built?
- Options: ship the disclosure with four arms pinned for the halt only | build the isolation with a `git` first on PATH that fails exactly one invocation and passes every other argument list to the real binary | keep the first refusal's arm as the only decider and delete the other four checks | rewrite `factory/common.sh` at run time per leg so the read under test is stubbed in place
- Chose: build the isolation with a `git` first on PATH that fails exactly one invocation and passes every other argument list to the real binary
- Why: the gap was measurable, not rhetorical -- with an arm's `return 2` replaced by `:`, the unmerged and the untracked arm each left all 81 legs green, and no plant closed it: `core.excludesFile` at a directory, at a `chmod 000` file, a poisoned `.git`, an inherited `GIT_DIR`/`GIT_WORK_TREE` pair and a corrupt index each stop two or more reads at once, because that is what an unreadable tree does. Asking git for a state where exactly one of its five reads fails is asking the filesystem for a state it does not have, so the state is made instead. Be exact about what the shim pins: the ARM -- this read's refusal, on its own, halts the pass and names itself in the sentence -- and nothing whatever about git refusing that way by itself; the planted legs, in both trees, stay the only evidence for that claim, and the shipped text says which is which rather than letting a green leg from a mock be read as a fact about git. The fourth option was rejected because stubbing the code under test in place proves the test. What makes a built refusal mean anything is the opposite-polarity leg, and the file already has the discipline (`planted()`, `naive_dirt_test_refused()`): with the shim installed and its match pointing at nothing git ever runs, the tree has to read CLEAN, so the five legs cannot be green because the shim halts by construction and a match that grew too broad goes red on that leg first. The rows also gave one thing no plant gave: on the untracked arm `naive:` reads CLEAN, because the weaker shape pipes `ls-files --others` through `head` and tests the string, so a read that dies 128 with no output is a tree with nothing in it -- the false clean #228 was filed for, now on a shipped row rather than argued. This entry also carries a correction, because the mutation shape is the thing that was got wrong: the review's table (tracked arm 4 red, refresh arm 1 red at cf87ecc) is reproduced exactly by `return 2` -> `:`, and H-0048's claim that it "described ALL arms hollowed" is false -- a constant-false condition is the other shape and pins one arm at that commit. The two are not interchangeable and the shipped comment now says why: `:` keeps the arm's recognition and falls through, so the sentence goes out with a finding appended and the verdict answers CLEAN; a false condition removes recognition, so the arm reads the refusal as dirt and still halts, and only a leg asserting the refusal's words can tell. A number in this repository is a measurement, and one that does not name the mutation that produced it is not a disclosure. What lost, as measured: eleven of the 81 legs now run against a shim, so a git that renames one of these five command lines -- `--exclude-standard` folded into a config flag, say -- retires that leg, which is #1048's hazard accepted on purpose; the direction of that failure was chosen so it is safe (the arm never refuses, the leg goes red rather than the verdict going quiet). And the arm-by-arm tallies are counts of this 81-leg suite at one commit: both files carry the sha with them, and the shipped text now also records that a disclosure's numbers have to name the mutation shape and the leg count, or they read as a property of the code.
- Checkpoint: 10556e8e5a87016f6bacfe5eb95b10f5247ab8fa
- Effect: `shim_install()` and `refusal_alone()` with the inert-shim control in factory/selftest.sh, five arms pinned by name in both the verdict and the halt, legs 70 -> 81 on tmpfs and 80 on btrfs with the exclude arm's note; the two-shape matrix table and the two-error history in factory/common.sh's `factory_tree_dirt` header, with the reviewer's cf87ecc figures reproduced (52 of 56, 55 of 56); the redundancy paragraph in docs/factory.md rewritten for both shapes and the shim; a `Correction (H-0049)` bullet on H-0048's entry


## H-0050 · A read that answered at its success status and still said it could not open the file was taken as an answer, so the verdict called a tree with hidden work clean

- Question: #232 filed that a directory at mode 000 inside the tree makes the dirt verdict answer CLEAN over an untracked file inside it, and the only thing in the run that says the read did not finish is what git printed on stderr while exiting 0. Is "returned its success status with a sentence attached" a refusal, when H-0045 documents exactly such a sentence -- `core.excludesFile` unreadable, `warning: unable to access '…'`, both reads at 0 -- as a genuine finding this verdict must report as DIRTY? And when an incomplete read turns up at an arm whose verdict already holds another arm's finding, whose answer is it?
- Options: leave #232 filed and change nothing, because the five exit statuses are the contract and stderr is diagnosis only, which is what H-0046 chose for the sentence | any stderr on any of the five reads is UNKNOWN, always, wherever the verdict stands | an incomplete read that found nothing refuses, unless another arm has already named the work, in which case it is appended as `only partly read`; and a run that cannot make its capture scratch file refuses rather than sending stderr to `/dev/null` | match git's wording (`could not open directory`, `Permission denied`) and refuse only on those
- Chose: an incomplete read that found nothing refuses, unless another arm has already named the work, in which case it is appended as `only partly read`; and a run that cannot make its capture scratch file refuses rather than sending stderr to `/dev/null`
- Why: the plant reproduces twice, and the second time is worse than the ticket. Measured at `df6eb97` on git 2.53.0 as uid 1000, throwaway repositories under the fixture's `TMPDIR` (tmpfs), five reads each: a mode-000 `drafts/` holding one untracked file leaves the five statuses at 0,0,0,0,0 with `warning: could not open directory 'drafts/': Permission denied` on `ls-files --others` at 0, and the verdict answers CLEAN; a mode-000 `sub/` holding a MODIFIED tracked file leaves them at 0,0,0,0,0 with `sub/nested.txt: Permission denied` on `diff --quiet` at 0, and the verdict answers CLEAN. The second is not a file that went unlisted, it is an edit git asserted *unchanged*: a file `diff` cannot open is a file `diff` cannot diff, so the modification is affirmed absent rather than missed. The first plant keeps any tracked file out of the unreadable directory on purpose, so the untracked arm is the one a state on disk pins. Asked what the false clean actually costs, the answer is not the one the ticket implies. Run through the tick's own commands in that tree: `git clean -qfd` attacks the hidden directory and fails -- `warning: failed to remove drafts/: Directory not empty`, exit 1, and the file inside is still there afterwards -- `git status --porcelain` prints warnings only at exit 0, which is a second way the replaced expression answers clean, different from H-0044's silent 128 -- and the merge that follows aborts with `Your local changes to the following files would be overwritten by merge: sub/nested.txt`. So the measured harm is a pass that starts, a journal line that says clean, and a halt whose only diagnosis is a merge conflict nobody can reproduce from the tree, not the deletion of the hidden work; the variant where `main` does not touch the hidden file was not measured and nothing here claims what happens to it. The fourth option is the one this repository has already refused twice: a rule keyed to git's phrasing goes blind the day upstream rewords a warning, which is why every leg in the suite asserts this repository's own `git said:` marker and never a phrase of git's. The rule is therefore "it spoke", and the words ride to the operator unchanged -- pinned at both polarities, the helper always incomplete (89 legs red) and never incomplete (6). The second option is not wrong, it is less, and the difference is a sentence: on the exclude-file-in-a-symlink-loop plant the staged arm fires first, so that leg goes from `DIRTY untracked: excluded-by-the-global-file.tmp` -- which orders the operator to commit or discard a file they had explicitly told git to ignore -- to `UNKNOWN … its staged state could not be read (git diff --cached exited 0; git said: warning: unable to access '…')`, which names the thing to fix. Both halt the pass; the pass was already going to halt. What is genuinely new is a tree that is clean apart from configuration git cannot read now stopping the queue, because git did not apply the operator's own ignore patterns and so git's clean is not the operator's clean -- #228's asymmetry on a case #228 did not have. Sixteen real trees on this box -- this repository and its fifteen links -- were read the four non-writing ways before that sentence was written (64 reads) and every one of them was silent, so nothing that exists today halts on chatter that is already there; the refresh was deliberately left out of that audit because it rewrites `.git/index` and a worktree here may belong to a person working in it. The append half was not designed, it was caught by a plant that failed: the first cut of this refused on any incomplete read, and `a readable edit beside an unreadable directory` came back UNKNOWN having *dropped the edit* from the sentence -- it told somebody with `tracked.txt` modified in front of them to go make the tree readable. It answers `modified: tracked.txt (git said: sub/nested.txt: Permission denied); its untracked files were only partly read (git ls-files --others exited 0; …)` now, and the staged-and-unreadable tree answers `staged-but-uncommitted: tracked.txt; its tracked files were only partly read (…); its untracked files were only partly read (…)`. Note which arm reports in which tree: with a tracked file *inside* the unreadable directory the tracked arm speaks first, and when only untracked work is inside, the untracked arm does -- which is H-0048's lesson applied to a new branch: the legs assert the arm's own words, not "something complained". The `mktemp` change contradicts a sentence shipped one round ago and says so in the comment: H-0046 chose "a reason is diagnosis, never evidence, so a box with no `/tmp` must not answer differently", and that bargain is exactly what makes #232 invisible on the boxes whose `/tmp` is broken. Since the capture is the only thing that distinguishes a clean read from an unfinished one, a run that cannot have it cannot answer. This also reverses the direction of a claim the reviewer's own history made about the refresh: its `path: needs update` is on **stdout** (measured, and it is what H-0046's redirect split caused), so the refresh is the one arm whose success is two statuses, `0 1`, and a leg that warned it at 0 would be vacuous -- its row warns at 1. What lost, as measured: the exclude-warning leg's verdict is a different class than it was, so anything that ever keyed on `DIRTY` there keys on `UNKNOWN` now, and both are halt; any future git that prints anything harmless on one of these five reads halts the whole pass, which is a new way for a queue to stop with nothing wrong but a version bump; a machine with a full or unwritable `TMPDIR` now halts the pass rather than carrying on, and `check`/`tick`/`status`/`resume` all inherit that; the two legs that hold a readable problem beside an unreadable directory cannot carry `strong`, because the weaker shape does see that dirt and the marker asserts the opposite -- so their `naive:DIRTY` is printed without being pinned, which is right: the pass halts there either way and those legs are about which sentence the operator gets. the three arms no plant isolates have their completeness branch pinned by the built shim only, so those rows pin the wiring and not git's behaviour (H-0049's caveat, now on two branches); and eleven of the new legs run against a shim, so a git that renames one of these five command lines retires them, which is #1048's hazard accepted on purpose because the leg goes red rather than the verdict going quiet.
- Checkpoint: df6eb97a4d7f152ddb9f3a0128b697baa9dbd15c
- Effect: `factory_git_incomplete()` beside `factory_git_said` in factory/common.sh, the three-way disposition (refusal / refusal-unless-nothing-else-was-found / appended `only partly read`) in the tracked, unmerged and untracked arms, the `0 1` success list on the refresh, and the no-scratch-file refusal in `factory_tree_dirt`; in factory/selftest.sh, `read_warns()` with the arm and status it expects, `no_capture_says_unknown()` with its function-shadowing `mktemp`, four unreadable-directory plants in both trees (hidden untracked work, hidden edit, visible work plus an unreadable dir, staged work plus an unreadable dir), `warn_alone()` -- the shim's second mode, answering at the arm's own success status -- with its inert-warn control, and the exclude-warning leg retargeted from DIRTY to UNKNOWN with its note saying why; three of the new legs carry `strong`, so the probe asserts `status --porcelain` is blind on them rather than only that the verdict is right, and the marker bites both ways (setting it on the visible-work leg is 2 red legs); legs 81 -> 120 on tmpfs and 80 -> 119 on btrfs, the one-leg difference still the exclude-directory plant (refuses on tmpfs where its `st_size` is nonzero, swallows on btrfs where an empty directory has none); twelve mutations of the shipped file in `docs/factory.md`'s table, every one red -- 2, 3, 5, 6, 89, 6, 12, 4, 4, 2, 6, 2 -- each restored from a backup taken after the fix with the tree confirmed back at the two intended edits, including M8, which was 120 passed / 0 failed until the staged-and-unreadable plant was written for it


## H-0052 · A ticket already satisfied by `main` was handed to a tick, so the ending it needs is written down rather than improvised

- Id: this entered the harness log as `H-0050` on `issue-228`. That number is published on `main` as *A read that answered at its success status and still said it could not open the file was taken as an answer, so the verdict called a tree with hidden work clean*, and a merged entry's id never moves, so this one -- unmerged, therefore the one that can yield -- was renumbered while resolving the merge. It took `H-0052` rather than the next free number: `H-0051` is already spoken for by the same renumber on `issue-207`, whose pull request is open at the same time, and taking the number after it is what keeps these two from colliding on `main` the way they just collided here. The two decisions that share H-0050 share nothing but the digit: this one is *A ticket already satisfied by `main` was handed to a tick, so the ending it needs is written down rather than improvised*.
- Question: the queue dispatched #228 at 09:35Z on a branch with no commits ahead of `origin/main`, two and a half hours after its fix merged at 07:08:17Z as #231 behind a body whose first line is `Refs #228.`. Every ending `.opencode/skills/factory/SKILL.md` documents is a pull request or an escalation, and neither is what a satisfied ticket needs. Does the tick leave the phantom in the queue for a person, escalate it, or close it -- and does the choice get written anywhere?
- Options: leave it open, comment, and put the close in `NEXT:` | `needs-human` plus `Chose: escalated` and stop | open a pull request anyway -- a second probe restating the fix -- so the tick ends with the deliverable the skill asks for | measure it here, close it as completed, and file the unfinished half as its own ticket
- Chose: measure it here, close it as completed, and file the unfinished half as its own ticket
- Why: tie-break 6 settled it before any reasoning here did, because this repository already closes issues exactly this way -- #220's last comment is "Closed as satisfied by `main` -- the fixture's second rule now releases at the all-day rule's own instant, and the hour-independence is measured", #211's is "Satisfied by #215 as merged, with main's later refinements ... Reopen if any line of the ...", both by the account these sessions act through, each carrying its measurement and the reopen invitation. The first option was refused on a measured cost rather than a tidy one: `factory/triage.sh` takes a claim on an issue only from an *open* pull request -- GraphQL `pullRequests(states:OPEN,first:100){nodes{closingIssuesReferences}}` at `:122`, `gh pr list --state open --json number,body` scanned for `closes|fixes|resolves|refs|part of` at `:155` -- so a merged fix claims nothing, and #1256 makes `Refs` the *correct* body for partial work, which means the more correct a body is the more certainly its issue returns to `unclaimed`. This unit's ceiling is the invoice for that, and `unclaimed` is an input to the pass budget, so the queue also overstates the work left while it happens. The second was refused because escalation is reserved: #856's dependency, safety or tamper semantics, publishing, deleting data, and what a person has already decided. Verifying that a merged commit satisfies a ticket is none of those, and `needs-human` would park the identical phantom under a label that still nothing works. The third was refused as noise with a price attached -- the fix is pinned by 81 legs that this unit measured discriminating against twelve mutants, so a duplicate test is a file to maintain plus a pull request competing with #232's live work in the same verdict. What the close rests on is this session's measurement, not #231's prose: `factory/selftest.sh` 81 passed / 0 failed, `check-mutations.py --self-test` 99 rules hold, and over throwaway clones -- never this tree -- the halt-never-happens, UNKNOWN-proceeds, real-dirt-proceeds, codes-swapped, no-HALT-file, call-site-drops-return, call-site-deleted, revert-to-`status --porcelain`, revert-spelled-`status --short`, no-refresh and silenced-staged mutants all caught (55/26, 72/9, 49/32, 55/26, the four HALT-file legs, 80/1, 80/1, 35/46, 36/45, 79/2, 77/4), which is the three survivors #231 recorded now caught, and each `check-mutations.py` restore exit pinned by its own named rule when the settle is disabled at that exit alone. The unfinished part went out as #235 with the two lines and the cost, because a finding that lives only in a comment on a closed issue is invisible to every queue that reads issues -- the same rule that makes a branch with no pull request invisible. What lost, stated plainly: a close is a write into the queue with no gate behind it. `triage.sh` reads *open* issues, so closing removes the row, and a wrong close deletes work nobody will look for again -- `git revert` does not undo it, only `gh issue reopen` does, and only if somebody notices. Filing #235 is a second ungated write, and nothing checks that a filed ticket is well-posed rather than one tick's bad day. Documenting a PR-less ending makes it available for tickets that are merely hard, which is how a queue quietly loses them; the skill's answer is a measurement rather than a reading of the previous pull request, and a ticket you are not certain about staying open. And the mechanism that made `wt/issue-199` answer ` M` for bytes identical to `HEAD` is still unidentified -- closing #228 means nobody is now paid to find it. Accepted because #228 itself declares it unnecessary ("both are covered by the fix below, which is why the fix is content-based rather than 'refresh and hope'") and disclosed in the close comment rather than buried under the word "completed".
- Checkpoint: 3e6b5020d4a9fb47318d501b6f0421d83812e689
- Effect: the "When the ticket is already satisfied by `main`" section and the widened `PR:` hand-off line in `.opencode/skills/factory/SKILL.md`; #228 closed as completed with the measurement table and the not-established paragraph; #235 opened for the classifier half, with `triage.sh:122` and `:155` and both polarities of pin it asks for


## H-0053 · Both claim channels read only open pull requests, so an issue whose fix had already merged went back out as work

- Question: `factory/triage.sh` decides `claimed` from two channels -- the GraphQL `closingIssuesReferences` on open pull requests, and a body scan of open pull requests -- so a **merged** pull request claims nothing whatever its body says, and an open issue whose fix is already on `FACTORY_BASE` is printed `unclaimed` (#235). Does the queue get a fourth class read off merged pull requests, and what makes a merged body a delivery: the verb it uses, the branch it came from, or facts GitHub already holds?
- Options: close the issue on the strength of a merged claim | report a `delivered` row and dispatch nothing, the verdict resting on the merged body's claim verb alone | report a `delivered` row and dispatch nothing, the verdict built out of five facts GitHub already holds -- the body claims exactly one issue, it did not use `Part of #N`, the merge reached `FACTORY_BASE`, the issue has not been updated since the merge, and no closure of that issue has already been rejected -- read in one bulk call bounded by `FACTORY_DELIVERED_WINDOW`, with the row naming the PR it leans on and the window it read | require the merged pull request's head branch to be `issue-<N>` as well, the shape this repository's own tickets produce
- Chose: report a `delivered` row and dispatch nothing, the verdict built out of five facts GitHub already holds -- the body claims exactly one issue, it did not use `Part of #N`, the merge reached `FACTORY_BASE`, the issue has not been updated since the merge, and no closure of that issue has already been rejected -- read in one bulk call bounded by `FACTORY_DELIVERED_WINDOW`, with the row naming the PR it leans on and the window it read
- Why: the bug was measured twice in one day before it was written, and the second measurement is the one that argues against the first option. #228's fix merged at 07:08:17Z and a tick was dispatched for #228 at 09:35Z on a branch with nothing ahead of `origin/main` -- that is H-0050's ledger, closed by H-0052 as #236. Eight hours later #232's fix merged at 14:26:31Z as #234, whose only reference is `Refs #232`, and the queue printed `issue 232 unclaimed` on a tree containing the fix. Closing on the merged claim is what this queue must not be trusted with: closing is a human act on this repository's tracker (H-0020), GitHub's own parser closes an issue at any inflection of `close` standing next to a reference with no notion of negation (#1256 measured it firing on `This does not close #10.`), and the harm of a false `delivered` is a ticket nobody ever looks at again -- so the class reports, and the arm is one `log_event` line that starts no tick, writes no ledger row and closes nothing. The second option is what the ticket asks for and it is wrong on this repository's own data. Read against the open issues on 8 Nov, "a merged PR names it with a verb" was false four times in five: #227's `Refs #222` points at an issue closed 6 Nov at 13:17:20Z and **REOPENED** at 16:27:23Z with a measured correction, and because the reopen happened *before* the 8 Nov merge, `updatedAt` cannot see it -- only `stateReason` can; PR #229's body is `Refs #199. Refs #204. Refs #207. Refs #213.` with a "What this branch does not finish" section disclaiming #204 and saying #213 is untouched, so a bundle must not suppress at all; #213's `updatedAt` is 02:46:41Z, after that 01:33:46Z merge, so the silence rule vetoes it independently; #199 is vetoed by the count rule and nothing else. `Part of #N` is disqualifying because it is this repository's own spelling of partial work (H-0020, #1256). The fourth option was measured before being refused rather than argued away: nothing in the live merged set contradicts a head-branch rule, but `git branch -r` carries `fix/nav-marks-the-current-page`, `chore/ignore-plock-binary` and three `wip/*` branches, so a rule keyed to `issue-<N>` would be blind to the majority of the deliveries this repository actually makes -- and it would be a second predicate to keep true, where every fact the chosen verdict reads is one GitHub maintains. `stateReason` is not reachable through `gh`'s `--json` at all: measured `Unknown JSON field` on both `gh issue list` and `gh search issues`, so it is taken from the GraphQL field folded into the query triage.sh already makes, which costs no second call -- and since `gh api` is denied to a tick, every claim in this entry was verified by running the shipped `triage.sh`, never by an ad-hoc call. Two findings came out of writing the legs rather than out of the predicate, and both are load-bearing now. The first is that a claim is a verb plus **every** number still in the same list. #221's shipped body puts `Refs #219, #220, #204` on one line and `(Part of #219 only; the umbrella is not finished …)` eight lines above it, and open PR #233 writes `Refs #220 and #222`; a verb plus one number reads either line as a sole claim on its first issue, which on the merged channel is `Closes #10, #11` silently delivering #10 -- the exact bundle this class exists to refuse, arriving through a comma. Two sentences about two issues are still two sentences, because the second number has neither verb nor list marker in front of it, and that is what keeps #218's `Not here: #149's per-control commit policy` from retiring #149. The second is the size of the read. The bulk call is `gh pr list --state merged --limit $FACTORY_DELIVERED_WINDOW --json number,body,mergedAt,baseRefName` -- one call, not one per pull request -- and the first version passed its 632,601-byte payload to `jq` as an argument, which Linux refuses above 128 KiB: `jq: Argument list too long`, and the verdict answered `{}`. Three legs went red and all three were the live-table legs, because the fixture's payloads are about two kilobytes -- the suite was green over a classifier that cannot run on the repository it ships in. The payload goes in on standard input now, one leg pads a body past 128 KiB to keep the real shape in the suite, and a scan that cannot run says so on stderr and in the detail rather than answering `{}`, because `{}` is the same shape as "nothing has been delivered"; the length probe sits outside the pipeline on purpose, since `set -euo pipefail` turns a failed read into an ended pass (measured: 4 legs). `gh pr list --state merged` answers newest-**created** first (234, 231, 229, 227, 226, 221, 218, 217), so the window is a bound on the read and not a statement about dates -- #217 was created 4 Nov and merged 8 Nov and sits below five PRs that merged before it, which the comment says out loud because a count window and a date window are not the same object. Nineteen single-edit mutations of the shipped files, all red at this commit: sole-claim 3, `Part of` 1, any-base 1, post-merge comment 1, empty `updatedAt` 1, any-stateReason 1, reopen veto 1, verb list minus `refs` 21, last-number-only 2, no list tail 3, oldest-claim-wins 1, window a constant 1, argument instead of stdin 2, silent refusal 1, missing `|| :` 4, row without the PR 2, row without the window 2, delivered rows still dispatched 12, arm starts a tick 1. Two of them were written because a leg was vacuous, and the sentence this entry first carried about *why* was itself the error. **Correction, measured the same day, after this entry was committed:** it asserted that #232 "has since been closed on GitHub -- a tick closed it as `already satisfied: origin/main..HEAD is empty`", that it therefore left the open-issue read, and that the read returned four rows. All three are false and not one of them was measured. `gh issue view` at 15:22Z on 8 Nov answers OPEN for #232 with `updatedAt 2026-11-08T07:30:46Z`, unchanged since the morning, and the fixture quotes that row with exactly that stamp (factory/selftest.sh:1606) beside merged #234 carrying GitHub's own body (:1580). What is true is narrower, and the shipped comment at factory/selftest.sh:1598 already said it: **#228, #212 and #149 are closed on live GitHub**, so no open-issue listing can quote them, their `updatedAt` values are chosen rather than read, and that is why the nine-row table plants them open -- planting them also made the claim-count rule decisive on #199, which no live row had been pinning. The name this entry gave the plant, `claim_open_rows`, does not exist either: the rows are a here-doc in `claim_fixture`'s `issues.json`, and `claim_open` is a different helper that plants an open pull request. The vacuity was real, but it is a fact about a leg's assertion and not about GitHub's state -- a leg written to assert `unclaimed` about a row the classifier has just moved tests the classifier's absence -- so the delivered legs assert the new class (`#232 delivered by #234`, `#228 delivered by #231`) and the veto keeps the opposite polarity (`Part of #232` is partial, not delivered). A third fact came out of the same re-read and is in docs/factory.md now: running the shipped `triage.sh` against GitHub at 15:22Z printed no `delivered` row at all, because open-claim precedence runs first and that channel drops an issue named *anywhere* in any open body -- #232 is named by the pull request that fixes it, #222 by draft #233's `Refs #220 and #222` -- while the three issue rows printed, #237, #213 and #199, were all `unclaimed`. A tally in this log has to name the mutation that produced it; a sentence about GitHub's state has to be read from GitHub, and this one was an inference about why two of my own legs had gone quiet, which is exactly the shape of error that reads like documentation. What lost, as measured: the window is a bound on the read, so an issue whose fix sits below `FACTORY_DELIVERED_WINDOW` merged pull requests keeps the verdict it always had, `unclaimed`, forever -- the row prints the window so an operator can raise it, but nothing here notices staleness; the verdict reads **claims**, not diffs, so a merged PR that says `Refs #N` and fixes something else entirely delivers #N, and that is the residual risk of the whole class rather than an edge case in it; the list tail is line-local, so a list folded across a newline, or a `#N` after a full stop, reads as two separate claims and suppresses -- it errs toward `unclaimed`, which is the safe direction and still wrong; `part of` is matched as a phrase, so `partially addresses #N` is not disqualified; `updatedAt` is GitHub's value for *any* activity, so a bot comment re-queues a delivered issue, and that is the price of the human override rather than a separate flaw; newest-claim-wins means two merged PRs claiming one issue report the newer one and no leg says which is the real fix; the fixture's three planted open rows disagree with live GitHub on purpose, so when #228 or #212 is legitimately claimed by a later pull request the fixture has to be re-read rather than left to drift; the fixture is two kilobytes and the real payload 632,601 bytes, so exactly one leg knows that; and the whole live table is a reading of 8 Nov, dated in the file because a tally that does not name its date is a description of nothing.
- Checkpoint: a9f81c44d28fec1f17b519c34bda3e7a44fa1413
- Effect: `FACTORY_DELIVERED_WINDOW` in factory/common.sh; the merged read, the jq verdict, the refusal on stderr and the `delivered` row in factory/triage.sh; the `delivered)` arm in factory/factory.sh, which reports and dispatches nothing; in factory/selftest.sh, `claim_fixture`'s three planted open rows -- #228, #212 and #149, closed on live GitHub, with their stamps chosen rather than read -- the 128 KiB pad leg, the newest-wins and window legs, and the mutation count corrected in the section header -- legs 120 -> 176; `delivered` in docs/factory.md's class table with the two paragraphs on why the verb is not the verdict and what the bulk call costs.


## H-0054 · Forty-five legs exec'd the shipped classifier through a preflight check for a binary the gate does not install, so `verify` was red on CI and green on every box in the loop

- Question: PR #238's `verify` check answers FAILURE at `ae8aba8` with the other four green, and every one of the 45 failures is a leg in the merged-claim section H-0053 added. Does that section get the runtime it silently requires, does the requirement go, or does the gate install the thing it is not gating on?
- Options: install `opencode` in the `verify` job | narrow `factory_preflight` so the read-only `triage.sh` asks only for `git` and `gh` and only `factory.sh` asks for the model | put an `opencode` stub in the fixture bin the section already builds and pin `FACTORY_OPENCODE` to it | export `FACTORY_OPENCODE=git` in `claim_rows`, satisfying preflight with a binary every gate host has
- Chose: put an `opencode` stub in the fixture bin the section already builds and pin `FACTORY_OPENCODE` to it
- Why: the diagnosis was measured before it was accepted, because "CI has no opencode" is a claim until it is reproduced -- `env PATH=<PATH minus ~/.opencode/bin> bash factory/selftest.sh` at `ae8aba8` prints `131 passed, 45 failed` and `factory: missing on PATH: opencode. Nothing was started.`, which is CI's tally and CI's sentence on the same commit, while the untouched PATH prints 176 passed 0 failed. So the section was reading the host, not the fixture. The requirement is `factory/common.sh:313`, `for tool in git gh "$FACTORY_OPENCODE"`, reached from `factory/triage.sh:80`, and `.github/workflows/ci.yml:21-49` installs Go and nothing else. Tie-break 6 chose between the options before any preference here did: the section already answers the *other* thing preflight demands -- `gh` -- with a fixture shim in `$claim_bin` (`factory/selftest.sh:1463`), so supplying the second dependency the same way is the shape the surrounding code already takes, in one file, reversible by `git revert`. Narrowing preflight is the tempting one and the review's own sentence argues for it ("a read-only `--table` should not require an agent runtime that only `factory.sh:529` ever starts"), but it deletes a refusal from a check all three callers share (`factory.sh:104`, `install-timer.sh:69`, `triage.sh:80`), and tie-break 3 will not buy a green gate by weakening the check whose comment is "everything a tick needs, checked before a model is invoked"; both production paths are covered without it -- `factory.sh:104` runs the same preflight before any tick and `install-timer.sh:119-121` refuses to install the timer at all when `$FACTORY_OPENCODE` is not on PATH. Installing `opencode` in `verify` is the worst of the four, for the reason the review gives: the job would need an agent runtime to test a read-only queue report, and `ci.yml:62` already records ".opencode/ is not installed anywhere" as a fact about the tree. `FACTORY_OPENCODE=git` is one line and green and it is the wrong fact -- it satisfies preflight with a binary unrelated to the requirement, so the legs would stop exercising the shipped default (`common.sh:62` spells it `opencode`) at the one term preflight tests. Three legs pin what was chosen: the runtime the legs resolve is inside `$claim_bin`, the stub exits 78 with a sentence instead of starting a model (the same shape as the shim's 77 at a write), and `common.sh` still carries the requirement -- with a control that deletes the term from a copy and demands the scan say no, because a fixture answering for nothing is this section's own failure mode. Measured after, three ways on one box: 179 passed 0 failed with no `opencode` on PATH, 179 passed 0 failed with it installed, and 179 passed 0 failed with `FACTORY_OPENCODE=plock-agent-that-does-not-exist` exported -- that last one is green because of the pin, not the PATH.
- Checkpoint: 84e192c549e52671baf92d0ed17c12261f73d95f
- Effect: `$claim_bin/opencode` with its `chmod`, `FACTORY_OPENCODE=opencode` in `claim_rows`, and legs 176 -> 179 on this tree (btrfs, `/home/rodney/prj/wt/issue-235`; CI counted 176 on this branch too, as 131+45). What lost, as measured: the shipped `triage.sh --table` still refuses to run on a machine with no model runtime, which is the wrong refusal for a read-only report -- the fixture now hides that from the suite, so the residual lives in this entry rather than in a green tally, and the day anyone narrows preflight the stub is the thing to delete; and the stub answers `command -v` for a binary the section never execs, so if `triage.sh` ever learns to start a model its legs get exit 78 and a sentence rather than the behaviour -- the intended failure, and still one nobody has watched fail.


## H-0055 · A comment told the next author that `emit()` caps the detail column at 80 columns, and `emit()` has never capped anything

- Question: `factory/triage.sh:444` says "emit() caps the detail at 80 columns, so this is sized for that cap", and `factory/selftest.sh` repeats it. `emit` is a six-field `printf` with no width directive and nothing in either file truncates a field. Does the column grow a cap to match the sentence, or does the sentence go?
- Options: cap the detail at 80 columns inside `emit()`, the first remedy the review offers | delete the clause and say nothing about length | delete the clause and bound the phrase in `CLAIM_RE` so the column cannot grow | delete the clause, state what actually bounds the column, and pin the behaviour with legs
- Chose: delete the clause, state what actually bounds the column, and pin the behaviour with legs
- Why: measured before anything was written, twice over, because the review's premise and my own sentence both needed checking. `emit` at `factory/triage.sh:82` is byte-identical to main's at `:79` -- `printf '%s\037%s\037%s\037%s\037%s\037%s\n'`, no `%s.80`, no `${var:0:80}` -- and a repo-wide grep for a 80-column claim finds only the two sentences that assert it, so the cap is invented, in the file it describes and in the suite that cites it. Capping in `emit()` was refused on what it would also do: the same column carries the *issue title* on every `unclaimed` row, GitHub allows 256 characters of title, main has always printed them whole, and `factory.sh` splits the row on U+001F and logs the detail verbatim -- so truncating to fit one sentence about one class is a behaviour change to every class, bought to repair a comment, and it would need legs of its own for the thing nobody reported. Tie-breaks 4 and 5 both land on the prose anyway. Deleting the clause and stopping there repeats the error sideways, because what replaces it has to be true, and the truth is neither "80" nor "unbounded": the phrase that reaches the detail comes out of `delivered_map`'s `select(($one | length) == 1)`, so it names exactly one DISTINCT issue; `CLAIM_RE` puts `[^#\n]{0,20}` between the verb and that number; and the list tail `(?:[ \t]*[,&/|][ \t]*#[1-9][0-9]*|[ \t]+and[ \t]+#[1-9][0-9]*)*` is unbounded only in a way the sole-claim rule has already disarmed, because a second DISTINCT number fails the `unique_by(.n)` select and the row never prints at all -- so the only body that can lengthen the column is one repeating the same number. Measured both ways rather than argued: the 12d plant, forty `#232`s in one sentence, comes back `delivered` with a detail of 302 columns, tail intact, printed whole (the number taken from a deliberately impossible `claim_detail_wider_than 232 99999` and the leg deleted once it had said 302); and the width leg by itself is a weak pin -- a cap at 200 would clear it while eating the end of the row -- so the third leg asserts `claim_detail_has 232 '(window 100)'` survives 302 columns of phrase. `CLAIM_RE` was not touched, which is tie-break 3 holding: narrowing what counts as a sole claim to tidy a column trades the predicate this class exists for against cosmetics, and the regex's own list tail is load-bearing for #221 and #233 (H-0053).
- Checkpoint: 72c4995ff913dd378ea2fb3285c5c43288af2cb7
- Effect: the comment above the `delivered` row in factory/triage.sh and the one over the live-table legs in factory/selftest.sh, plus `claim_detail_wider_than` and the 12d group -- three legs, 179 -> 182 on this tree (btrfs, `/home/rodney/prj/wt/issue-235`). What lost, as measured: the row is compact by choice and the corrected comment says "checkable at a glance", which is a style instruction with no gate behind it -- a future row carrying a paragraph of prose clears every leg here, because what the legs pin is that nothing truncates, not that anything is short; 302 columns is a property of the plant, not of the code, so the figure in both files is an illustration and reads as one, and a body with four thousand repeats would print 30 KiB into the table and into `factory.sh`'s log line; and a pathological body therefore still makes `triage.sh --table` ugly on purpose, refused rather than fixed, because the alternative was shortening the field a person re-checks.
