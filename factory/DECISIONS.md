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


## H-0051 · A pull-request body a tick cannot rewrite: `gh pr edit` is dead on this account and the REST PATCH is denied

- Id: this entered the harness log as `H-0043` on `issue-207`. That number is published on `main` as *The probe refreshed with an option git refuses, and its stat leg never asked whether it had planted anything*, and a merged entry's id never moves, so this one -- unmerged, therefore the one that can yield -- was renumbered to `H-0051` while resolving the merge. The two decisions share no subject beyond the number: this one is *A pull-request body a tick cannot rewrite: `gh pr edit` is dead on this account and the REST PATCH is denied*.
- Question: the 8 Nov re-review of #225 asks, as its non-blocking item, for the pull-request body to be
  refreshed, "since the body is where the rollback command lives", and its rollback line had indeed gone
  wrong. `gh pr edit 225 --body-file …` exits 1 with `GraphQL: Projects (classic) is being deprecated …
  (repository.pullRequest.projectCards)` and writes nothing -- three attempts, each read back from the
  API: body still 2793 characters, still containing the stale "17 insertions and 8 deletions". `gh` is
  `/usr/bin/gh`, version 2.46.0 (2025-12-13 Ubuntu 2.46.0-4) -- both halves, because a version nobody can
  locate is a claim nobody can re-run. The one verb that sets a body on this account is the REST
  `PATCH /repos/{owner}/{repo}/pulls/{number}`, which reaches only through `gh api`: `.opencode/opencode.jsonc`
  denies it as `"* api *"` and `"* api"` inside the `bash` table, which is the keying that makes a rule
  evaluate rather than sit inert, and `D-0013` is the entry that chose whole-over-method filtering. How
  does a tick answer a finding it is not equipped to answer?
- Options: post the corrected lines as a comment, say plainly that the body is stale, and name the command
  that would set it | route the PATCH through a wrapper script so the denial is never literally spelled |
  escalate `needs-human` and ship nothing for this finding | leave the body stale and say nothing
- Chose: post the corrected lines as a comment, say plainly that the body is stale, and name the command
  that would set it
- Why: option two is the one act the harness treats as unredeemable -- "never wrap the denied call in
  `bash -c`, `env`, `xargs`, `sh -c`, or a script that shells out to it" -- and what is denied here is not
  a flag but the reason: `gh api` is withheld so that a unit of delivery work cannot rewrite the artifact
  a merge is authorised from. A wrapper that obtains the write anyway does not route around a pattern, it
  routes around the judgement, and it leaves no trace except the write. Option four is the silent tick this
  log exists to end. Option three over-reads the stakes: a stale body is a documentation defect, not an
  irreversible act, and the unit also owed both blocking findings, so parking #225 on `needs-human` over a
  body line would have held up review of prose that was already finished -- and the four pull requests
  that once sat in `REVIEW_REQUIRED` for three days are the precedent for what a tick that stops early
  costs. Option one costs nothing a human cannot undo: every corrected line (the two-file stat, the
  rollback pair, the `D-0038`/`D-0039` numbering) went into the comment beside the answer to the blocking
  items, so whoever wants the body refreshed has both the text and the call.
  The precedent is H-0010, measured four days earlier on this same account and the same verb family:
  `gh pr edit 217 --add-reviewer hermesvicowa` exited 1 with the identical `projectCards` error and left
  `/requested_reviewers` empty, and what was concluded there -- the REST path belongs to the supervisor,
  call plus read-back, not to the tick -- is this entry's conclusion one field over. What is new is that
  the field is the body, so the failure is not cosmetic: the text a reviewer is told to consult is the one
  text this agent cannot write. `--json` is not a way out either; `gh pr edit` on 2.46.0 has no such flag
  and answers with its usage text, so the read-back has to be a separate `gh pr view --json body`, which is
  why every claim above quotes a length and a substring test rather than an exit code.
- What this costs, stated rather than glossed. (1) #225's rollback instruction is correct in a comment and
  wrong in the body, and the body is what the queue and a passing human read first. (2) The failing verb is
  now witnessed twice, `--add-reviewer` at H-0010 and `--body-file` here, and neither the skill nor
  `.opencode/opencode.jsonc` says so, so the next tick spends its ceiling rediscovering it; the repair is
  one line beside the reviewer-request paragraph in `.opencode/skills/factory/SKILL.md` saying that
  `gh pr edit` is dead on this account for every field and that a body correction ships as a comment. That
  is named here rather than written there, because a product branch editing the harness's skill is how two
  loops end up citing each other's ids. (3) `H-0043` is minted past `H-0042`, the highest in this clone on
  any ref (`refs/heads/issue-228`, 40 commits ahead and un-published); if that branch numbers from its own
  maximum instead of reading the rule, this number collides and the published side keeps it.
- Checkpoint: 89382eee3bb92e6bd458e92d33d93cf73221c22e
- Effect: pull request #225 (a comment; the body is unchanged because it cannot be written from here),
  factory/DECISIONS.md

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
- Effect: `FACTORY_DELIVERED_WINDOW` in factory/common.sh; the merged read, the jq verdict, the refusal on stderr and the `delivered` row in factory/triage.sh; the `delivered)` arm in factory/factory.sh, which reports and dispatches nothing; in factory/selftest.sh, `claim_fixture`'s three planted open rows -- #228, #212 and #149, closed on live GitHub, with their stamps chosen rather than read -- the 128 KiB pad leg, the newest-wins and window legs, and the mutation count corrected in the section header -- legs 120 -> 176, **counted with `$TMPDIR` on tmpfs** -- the pair is box-dependent by one leg and the cause is H-0045's exclude-file arm, which prints two legs where git refuses a directory `core.excludesFile` and one where it swallows it, a question the filesystem under `$TMPDIR` answers rather than one the repository asks. Measured at f60c367 on one machine with only `TMPDIR` moved: 191 passed under a tmpfs `$TMPDIR`, 190 under a btrfs one, the leg's own line switching from `an exclude file git refused` to `an exclude file git swallowed`. So the re-read that counted 119 at `main` and 175 here was not another box -- both boxes are btrfs at the worktree -- and only the delta (+56) travels; the absolute is a property of the fixture root, and factory/selftest.sh's tally line now prints its own fixture root, filesystem and git version so a quoted number carries its world (H-0104); `delivered` in docs/factory.md's class table with the two paragraphs on why the verb is not the verdict and what the bulk call costs.


## H-0054 · Forty-five legs exec'd the shipped classifier through a preflight check for a binary the gate does not install, so `verify` was red on CI and green on every box in the loop

- Question: PR #238's `verify` check answers FAILURE at `ae8aba8` with the other four green, and every one of the 45 failures is a leg in the merged-claim section H-0053 added. Does that section get the runtime it silently requires, does the requirement go, or does the gate install the thing it is not gating on?
- Options: install `opencode` in the `verify` job | narrow `factory_preflight` so the read-only `triage.sh` asks only for `git` and `gh` and only `factory.sh` asks for the model | put an `opencode` stub in the fixture bin the section already builds and pin `FACTORY_OPENCODE` to it | export `FACTORY_OPENCODE=git` in `claim_rows`, satisfying preflight with a binary every gate host has
- Chose: put an `opencode` stub in the fixture bin the section already builds and pin `FACTORY_OPENCODE` to it
- Why: the diagnosis was measured before it was accepted, because "CI has no opencode" is a claim until it is reproduced -- `env PATH=<PATH minus ~/.opencode/bin> bash factory/selftest.sh` at `ae8aba8` prints `131 passed, 45 failed` and `factory: missing on PATH: opencode. Nothing was started.`, which is CI's tally and CI's sentence on the same commit, while the untouched PATH prints 176 passed 0 failed. So the section was reading the host, not the fixture. The requirement is `factory/common.sh:313`, `for tool in git gh "$FACTORY_OPENCODE"`, reached from `factory/triage.sh:80`, and `.github/workflows/ci.yml:21-49` installs Go and nothing else. Tie-break 6 chose between the options before any preference here did: the section already answers the *other* thing preflight demands -- `gh` -- with a fixture shim in `$claim_bin` (`factory/selftest.sh:1463`), so supplying the second dependency the same way is the shape the surrounding code already takes, in one file, reversible by `git revert`. Narrowing preflight is the tempting one and the review's own sentence argues for it ("a read-only `--table` should not require an agent runtime that only `factory.sh:529` ever starts"), but it deletes a refusal from a check all three callers share (`factory.sh:104`, `install-timer.sh:69`, `triage.sh:80`), and tie-break 3 will not buy a green gate by weakening the check whose comment is "everything a tick needs, checked before a model is invoked"; both production paths are covered without it -- `factory.sh:104` runs the same preflight before any tick and `install-timer.sh:119-121` refuses to install the timer at all when `$FACTORY_OPENCODE` is not on PATH. Installing `opencode` in `verify` is the worst of the four, for the reason the review gives: the job would need an agent runtime to test a read-only queue report, and `ci.yml:62` already records ".opencode/ is not installed anywhere" as a fact about the tree. `FACTORY_OPENCODE=git` is one line and green and it is the wrong fact -- it satisfies preflight with a binary unrelated to the requirement, so the legs would stop exercising the shipped default (`common.sh:62` spells it `opencode`) at the one term preflight tests. Three legs pin what was chosen: the runtime the legs resolve is inside `$claim_bin`, the stub exits 78 with a sentence instead of starting a model (the same shape as the shim's 77 at a write), and `common.sh` still carries the requirement -- with a control that deletes the term from a copy and demands the scan say no, because a fixture answering for nothing is this section's own failure mode. Measured after, three ways on one box: 179 passed 0 failed with no `opencode` on PATH, 179 passed 0 failed with it installed, and 179 passed 0 failed with `FACTORY_OPENCODE=plock-agent-that-does-not-exist` exported -- that last one is green because of the pin, not the PATH.
- Checkpoint: 84e192c549e52671baf92d0ed17c12261f73d95f
- Effect: `$claim_bin/opencode` with its `chmod`, `FACTORY_OPENCODE=opencode` in `claim_rows`, and legs 176 -> 179 with `$TMPDIR` on tmpfs on this box (`/tmp`; the btrfs worktree is not the variable -- H-0104). CI counted 176 on this branch too, as 131+45, so the runner's fixture root landed on the same arm of H-0045's leg as this box's `/tmp` did -- which is agreement about a git's behaviour, not a fact about the runner's filesystem, which is the number a runner with no `$TMPDIR` of its own prints. What lost, as measured: the shipped `triage.sh --table` still refuses to run on a machine with no model runtime, which is the wrong refusal for a read-only report -- the fixture now hides that from the suite, so the residual lives in this entry rather than in a green tally, and the day anyone narrows preflight the stub is the thing to delete; and the stub answers `command -v` for a binary the section never execs, so if `triage.sh` ever learns to start a model its legs get exit 78 and a sentence rather than the behaviour -- the intended failure, and still one nobody has watched fail.


## H-0055 · A dispatch carried the review's timestamp and not its text, so a tick was sent to answer a review whose body had never arrived

- Question: `address_review` is decided from three fields -- reviewer, state, timestamp -- and `factory/triage.sh`'s review read projected `.body` away before anything in the harness could read it, so `factory_tick` told the model "the review's substance is in the pull request's latest commit message and diff -- read them, **do not guess**" while handing it nothing but a timestamp. Does the reviewer's own text go into the instruction, and does a change request whose body never arrived still get a tick?
- Options: interpolate the body into the prompt and change nothing about when a tick starts | interpolate it, and tell the tick to stop if it cannot see an ask | carry the body as a row column and refuse the tick in the classifier when the body is empty, a bare path or an `@`-mention, or under 40 bytes, reporting the bytes, the reason and the one command that re-files it | take the timestamp out of the row and let the tick read the review itself from `gh pr view`
- Chose: carry the body as a row column and refuse the tick in the classifier when the body is empty, a bare path or an `@`-mention, or under 40 bytes, reporting the bytes, the reason and the one command that re-files it
- Why: the triggering body is 21 bytes. `#240`'s change request, filed at `18:19:17Z` on `8 Nov`, is `@/tmp/pr240-review.md` -- filed with `gh api -X POST repos/.../pulls/240/reviews -f body=@/tmp/pr240-review.md`, where `-f` is `--raw-field`, which sends the bytes after the `=` literally and reads no file. GitHub answered **200 with a healthy review row**, so nothing anywhere said the ask was missing, and a worktree, a ceiling and a model were spent on a verdict with no question in it. The first two options spend that same work anyway: a tick cannot tell *"the reviewer asked for nothing"* from *"the reviewer asked for something I cannot see"*, because from inside the tick the two arrive as the same empty string, and what it writes in that gap is the most-measured failure this repository has -- `D-0046` is five invented citations across three published artifacts, and the rule it produced (#1180) is that a claim written before the read-back is a prophecy. The fourth is refused because the timestamp is the only field that says *which* review the row means; the tick reading the body itself is kept, but as the fallback for a transport failure, not as the design. Both thresholds are measured, not chosen, and the unit had to be measured twice to be right. The first tally here used `jq`'s `length`, which counts **characters**, and was published on `#240` as bytes; the reviewer's own `18:41:12Z` body is 14777 characters and **14863 bytes** -- the predicate counts bytes, so the whole tally was re-done through python over the NUL-joined bodies. Measured that way over the 86 `CHANGES_REQUESTED` reviews on this repository's 200 most recent pull requests (`8 Nov`): five shortest **21 / 1002 / 2695 / 2986 / 3124**, longest **14863**, median **7569**, exactly **one** body under 40 bytes, and **83 of 86** at or under a 12000-byte bound. So a 40-byte floor retires no ask anyone has written here by a factor of 25, and 12000 carries 83 of 86 into a dispatch whole while the other three arrive cut with their total and the command that reads the rest. The shape arm exists because a floor cannot see the actual failure at any length: `/home/reviewer/notes/pr240-review-notes.md` is a 42-byte ask that is not an ask, and raising the floor to catch it starts refusing real prose. The predicate is `bytes < 40` -- strictly -- and that boundary is a deliberate divergence from the reviewer's own ask, which arrived independently at `18:41:12Z` as "refuse a `CHANGES_REQUESTED` whose body is <= 40 bytes or matches `^@?/[\w/.-]+$`": two people, two ends of the same pipe, the same rule, and the reviewer's side of it is the one that files reviews. 40 itself is accepted here because a floor that eats its own boundary buys nothing when the shortest real ask is 1002 bytes, and the leg pins `40 -> readable` and `39 -> below-floor` so the boundary cannot drift either way in silence. Three findings came out of writing the legs rather than out of the design, and all three are load-bearing now. **(1) No leg in the 1235-line suite had ever produced an `address_review` row.** The claim shim's REST arm dispatches on `${3}` -- the `--jq` expression -- where the fixture's own file-name normaliser is handed `${2}`, the path, so every `api … /pulls/N/reviews --jq '…'` the classifier made printed `claim-shim: unhandled api path` and answered empty. The legs still passed because the empty read made every planted pull request look like it had no reviews, and one of them asserts `needs_reviewer` for the PR the leg had just planted a change request on: it was pinning the bug. Fixed, the same fixture answers `needs_reviewer` with the detail `answered by the head commit ` -- an empty date -- which is the more general hazard: **a REST read that fails and answers empty is not a neutral failure here, it is the positive verdict "no review" wearing an empty state**, and `reviews | length > 0` is the only guard. **(2) A bound built on `cut -b 1-N` ends its output with a newline.** Found by writing the leg before the code it tests: the shipped row for a long review came back with `ask=12082` bytes for a 12000-byte bound and the row's own text split across two lines -- the trailer arrived as a separate record, and a newline inside a `\037`-delimited row is a splice into whatever reads it next, which here is a model prompt and a ledger line. `head -c` writes the exact bytes and nothing else; the leg pins the length, the tail, and that the run prints one row rather than two. **(3) A mutation is only evidence if it is verifiably applied.** An early pass reported `0 failed` for four mutations and all four had silently not applied -- two sed targets did not exist, and one added a class to a `case` *below* an arm bash had already matched, which is unreachable, so the tree was unmutated and the green was the baseline. Every mutation in the sweep now echoes the changed line back and refuses to run the suite if the proof grep fails; `sweep.sh` lives at `/tmp/opencode/sweep.sh` with this entry and its output is the tally. Sixteen single-edit mutations of the shipped files were run at this head, every one red, every one applied by line address inside the function that owns the line and echoed back before the suite ran: the fixture's `${3}` as shipped, 19 legs; the refusal arm deleted, 9; the `readable` arm made to never fire, 9; the ask column never filled, 4; the predicate's `reference` arm deleted, 4; the floor moved to 41, 4; the floor moved to 20, 4; the `BEGIN/END REVIEWER ASK` block deleted from the instruction, 4; the floor set to zero in the shipped file, 3; the dry run stopped printing the ask, 2; the predicate's empty arm deleted, 2; `review_unreadable` made to queue the unit itself, 1; the bound raised to 120000, 1; `head -c` put back to `cut -b`, 1; the second re-serialisation dropping the column, 1; the flattening removed from `factory_newest_ask`, 1. Restores are `cp` and a byte comparison after every case, never `git checkout` (#1044). Three of the sixteen came back `NOT APPLIED` on the first pass, and all three were the harness lying rather than the mutation failing. One proof was written as an extended-regular-expression pattern whose literal text ends in `"` followed by `)`; measured on a copy afterwards, `grep -qE` with that pattern answers "no match" -- exit 1, no diagnostic -- against a line containing the pattern's own text verbatim, while `grep -qF` with the same bytes matches the mutated file and refuses the unmutated one. The mutation had applied, was discriminating, and was reported as having done nothing. A second proof wrote `grep -qE '\t'`, which GNU grep does not expand, and failed the same way on a mutation that had applied. The third had two faults at once: the flattening call it meant to remove is not in `factory_ask_bound`, which is handed text already flat, but in `factory_newest_ask`, where the body is read out of the payload, and its proof string did not match the text the `sed` had written. A proof that matches nothing and a mutation that did not happen are the same exit status to a shell, which is the whole reason every proof in the sweep is a fixed-string grep now, and why a case whose proof fails is skipped and said out loud rather than counted as a result. A fourth mutation was thrown out after it had already produced a number. It added `review_unreadable` to the dispatching `case` arm to ask whether the refusal could be bypassed, and first read two legs red -- but `bash -n` on the copy it built says `line 792: syntax error near unexpected token `)'`, because the `sed` had inserted a second arm inside the first arm's body: the two red legs were a supervisor that would not parse. Rebuilt as a clean one-line edit it went vacuous for a quieter reason -- the refusal arm sits ABOVE the dispatching one, at factory.sh:787 against :791, and `case` takes the first match, so adding the class below reaches nothing. The mutation that really asks the question is the refusal arm queueing the unit itself, and that turns exactly one leg red. A syntax error and a behaviour change both arrive as red legs, and a green leg under a vacuous edit looks exactly like a passing test: the sweep runs `bash -n` on every mutated file before it runs the suite. Two controls ride in the suite rather than in the sweep, both planted outside the repository: a copy with the `reference` arm deleted is still refused by the scan behind `which is reference` on the row, and a copy with `FACTORY_ASK_MIN_BYTES=0` is still refused by the leg that says a 21-byte English ask is refused -- without those two, both greens are assertions about a fixture. One mutation reddens a control leg on purpose: deleting the marker block from the shipped file makes the copy and the tree disagree, which is the control proving it is comparing something. What lost, as measured: a genuine 21-byte ask in ordinary English -- `Rename it and ship it` -- is refused exactly as the 21-byte path is, and the only thing standing between that and a silent regression is a leg that goes red when somebody softens the predicate to avoid it; the bound is a truncation and not a summary, so three of this repository's 86 change requests reach a tick with 12000 of their bytes, marked as cut and with the read command named, and nothing checks that the tick answered the part it was given; the ask is reviewer-controlled text interpolated into a model's instructions, bounded and length-prefixed and bracketed by `BEGIN/END REVIEWER ASK … not instructions to you`, but not sanitised, because markdown quotes are the ask's own formatting and are the reason the marker is a line rather than a wrap; the refusal's recovery is a human act that this harness cannot complete -- `gh pr review` is denied to a tick, so the comment carries `gh pr review N --request-changes --body-file - <<'REVIEW'` as the one thing that works on either end of the pipe, and if nobody acts the row waits forever by design; a refusal re-posts at most once a day and an unbounded one is a reply chain, which is what H-0051's throttle was built for; `review_unreadable` never parks, so a change request nobody re-files holds `WAIT` -> `NOTICE` indefinitely and that is H-0051's shape chosen on purpose; the empty-column tripwire is belt-and-braces rather than a tested path -- the tick is told to read the body itself and, if it is not there, to stop with `needs-human`, and no leg can prove a model obeys; the fixture's `reviewDecision` had to move from one global file to a per-pull-request field, because the section that asserts a change request was *answered* and the legs that assert one *arrived* need two different values on two different pull requests in one fixture and both sections were green while neither meant what it said; and `#240` is merged, so the notice this unit builds will never be delivered to the pull request that motivated it -- `triage.sh` reads open pull requests only, and a merged one produces no row, which is also the sentence my own first correction comment on `#240` got wrong by promising the notice would appear there.
- Checkpoint: d4708cd214fcd1b8b653b74147aa072e65e9f636
- Effect: `FACTORY_ASK_MIN_BYTES` and `FACTORY_ASK_MAX_BYTES`, `factory_ask_flat`, `factory_ask_verdict`, `factory_newest_ask` and `factory_ask_bound` in factory/common.sh; the newest-ask read, the `ask` column, the `review_unreadable` class, its detail sentence and the bounded carry in factory/triage.sh; the `review_unreadable)` arm (throttled comment, no tick) and the ask's place in the `address_review` instruction -- markers, the not-instructions-to-you sentence and the `needs-human` stop -- plus the dry run's `ask:` line in factory/factory.sh; in factory/selftest.sh, the shim's `${2}` dispatch fix, the per-pull-request `reviewDecision`, and the ask legs, 179 -> 216; the two `address_review` rows and the section "The ask, and what happens when there is not one" in docs/factory.md. Two GitHub writes, each read back from the API after posting and before this line was written: the superseding correction on #240 at `20:35:41Z` -- `gh pr view 240 --json comments` answers 6 comments, the last carrying "Superseding the comment immediately above", "21, 1002, 2695, 2986, 3124" and "83 of the 86", and not carrying the sentence it retracts -- and the re-dated-`sources_rev` pointer on #199, which answers with `:480`, "0 problems over four frames" and `is_ancestor` in the newest comment. Neither write was in the record before this entry: `H-0050` named both artifacts and neither existed on the tracker until now.


## H-0056 · The supervisor runs whatever is checked out in its worktree, and a tree 121 commits behind the trunk passes every check in the repository

- Question: `install-timer.sh` renders `ExecStart=@REPO@/factory/factory.sh`, so a pass runs whatever that tree has checked out at the moment the timer fires, and nothing in the harness -- not `--status`, not the journal, not the unit, not `triage.sh` -- compared that checkout with `origin/main`. What is the comparison made against, who is told, and what is refused?
- Options: report it, and refuse it only where a human is standing | fetch the trunk at the top of every pass and compare against the fetched ref | fast-forward the tree automatically at install | halt the pass whenever the tree is behind | print the sha alone, with no comparison
- Chose: report it, and refuse it only where a human is standing
- Why: measured on the tree, not of it. `git rev-list --count efa4e29..origin/main` answers **121** with no network at all, and the tree's own reflog dates the checkout at `2026-11-06T19:27:50Z` against a fast-forward taken `2026-11-09T16:39:59Z` -- three days, from the remote-tracking ref the orphan pass already reads (`triage.sh:539-543`, whose `branch=` column prints exactly that number for a *unit's* branch and has never once been asked about the supervisor's own); `grep -c 'PASS units=' factory/log/supervisor.log` in that tree answers **204** and not one of them reads `units=0` -- the queue was never empty, it was being read by old code. The symptom is in the halt instead: 89 lines carry the sentence `a worktree was left dirty by a previous tick`, which `git grep` finds in no shipped script on `origin/main` or on this branch (`factory.sh:109` re-prints the reason stored in `factory/HALT`, so those lines are `efa4e29`'s wording echoed once an activation), the `issue-247` instance running 82 lines from `01:05:09Z` to `16:37:09Z` -- **15h32m** -- two minutes before the tree was fast-forwarded at `16:39:59Z`. What that code was missing is checkable: `git show efa4e29:factory/common.sh` has no `update-index --refresh` where `origin/main` has eight, arriving in `cf87ecc`, `08bd015` and `15352b3`, all three inside the 121 -- so the verdict that decides whether to stop the whole queue was the pre-#228 one, reading a stat cache #228 had already learned not to trust. And the reinstall a person reaches for is a no-op with a healthy exit: run today against a clone 33 commits behind its own trunk, the shipped `--install` exited **0**, wrote both unit files, and left one journal line reading `TIMER-INSTALLED`, because the only thing it asks of the tree is whether a file is executable. So the failure was never the absence of a number -- it was that no sentence anywhere read the tree the timer execs, and the act a person reaches for when a loop is not moving (reinstalling it) re-armed the very staleness they came to fix. `--status` and the journal are the report, because nobody asks a timer a question and `PASS units=0` is the same sentence for an empty queue and an out-of-date reader. `--install` is the refusal, and the reason it is a refusal and not a warning is `H-0034`'s: a person in a terminal is the one moment a second command costs nothing, and the warning is the thing that was already missing. The three other arms lose on evidence rather than taste. **Fetch in the pass** buys a number that is 200 ms fresher and pays for it in the one hazard this repository has been taught twice -- `H-0013` lost 5min2s of a pass to a *local* read, and `#879` is a blocking open that fails *open* by parking the loader; a pass runs on a 5400s budget with no `fetch` anywhere in it today, and a timer's network is a workstation's network. **Fast-forward at install** fixes the 8 Nov state and creates a worse one: the tree being moved is the tree the timer will exec, a fast-forward is a write to a checkout that may hold the interrupted work of a killed tick (`#228` is that exact state, and its lesson is that a machine does not clean up a person's tree), and a moved tree that fails halfway is a supervisor that will not start. **Halt when behind** is the tempting over-correction and it inverts the polarity the last two decisions spent their blood on: `H-0051` and `H-0052` narrowed halting so a check that cannot see stops the queue instead of passing it, and "old" is not that -- the queue's state would be unchanged and a halt would stop a working loop because a laptop had not fetched, which is a worse `units=0` than the one being fixed. Halting stays for a tree the harness does not own. Print the sha alone is what `#249` was filed against: a bare hex string looks like a version, so nobody asks which ref it names, and `H-0018`'s rule (a refusal names the command and the number) is why the printed sentence carries the count, the ref and the branch. Two things were found writing the legs, both reported rather than fixed quietly. `install refuses a stale tree, writes none` was **passing on a tree that was no longer stale**: `behind 3, then silent once moved` fast-forwards the same fixture to prove its second half, and the install leg ran after it -- so `tree-moved` is now its own clone, and the leg went red the moment the trees stopped being shared, which is the only reason anyone knows the leg was ever wrong. And the install leg now named `a checkout with no commits is refused` was vacuous in its first shape: `factory_preflight` refuses a plain directory six lines before the position check, so that exit was never the arm's; it is now `git init` with no commit -- the accident that actually reaches the UNKNOWN arm (a clone into the wrong path, the harness copied in, `--install`). One more measurement belongs in the record because the sentence was wrong before it was written: the count is not first-parent -- a tree detached at `origin/main~3` reports **33**, because the trunk's merge history carries side branches the tree also lacks. That is the reading `triage.sh` already prints, so the two agree, and `docs/factory.md` now says so beside the example rather than letting a reader find it in `git log`.
- Checkpoint: 95e181694da56a13668524ae54557d4839a5a571
- Effect: `factory_supervisor_base`, `factory_plural`, `factory_supervisor_position` and `factory_supervisor_lines` in `factory/common.sh`; the `SUPERVISOR-*` lines beside every `PASS` and the `code:` line of `--status` in `factory/factory.sh`; `--force`, the position check and `TIMER-FORCED` in `factory/install-timer.sh`; a `Which code the loop is running` section in `docs/factory.md` with the real output quoted, the `--install`/`--force` lines, `code:` in the `--status` sample, and the pointer from `Stopping it`; nineteen legs in `factory/selftest.sh`, 216 -> 235 on this tree (btrfs, `/home/rodney/prj/wt/issue-249`). What lost, as measured: a tree can read `0 behind` forever on a machine nobody ever fetches on, because there is no fetch in the verdict -- that is bought knowingly, against `H-0013` and `#879`, and it is why the printed command starts with the fetch and why `NOBASE` and `UNKNOWN` exist rather than defaulting to zero; `--force` is one flag away from silence, mitigated by the `TIMER-FORCED` journal line and by the `AT` line being on every pass, not removed; a tree that goes stale *after* install is reported and never refused, because the pass that notices it is the pass that cannot move it without editing the script bash is reading; `dirty=` is a second reader of the working tree beside `factory_tree_dirt`, so the two can disagree for a tree whose index cannot be read (the exclude-file leg prints `dirty=0` beside a halt) and that disagreement is a journal line rather than a verdict; `--status` answers for `$FACTORY_HARNESS` and not `$FACTORY_ROOT`, so a harness installed from one tree and pointed at another reports the code that ran rather than the code the other checkout holds -- deliberate, because `--status` is the question "what ran", and `install judges the ExecStart tree` is the leg that keeps the two from being confused in the one place a write follows.


## H-0058 · A worktree left dirty by a dead tick halted every other unit for 15h57m with nobody told

- Question: A tick killed at the timeout left one file modified in `~/prj/wt/issue-237`, and `factory/HALT` has no per-unit scope: the halt switch is read before triage, so 22 other units went undone for 15h57m while nobody was told. The only signals were a journal line in a log nobody reads and a file in a directory nobody opens, and `systemctl --user status` renders the deliberate exit 8 as `code=exited, status=8`, the shape of a crash. What does the harness do with a tree it cannot work in?
- Options: halt as before, and add an operator-visible line to the journal naming the age of the marker | quarantine: mirror the work to the at-risk box, stash it under a supervisor-subjected message, name both in the journal, clear the tree, and let the pass continue; halt only when the rescue cannot be completed or the same unit goes dirty twice without a tick between | always quarantine, with no streak rule and no refusal arms | quarantine without mirroring, the stash alone | reset the tree to HEAD and continue, since the tick that made the mess was killed anyway | keep halting and put a per-unit skip list next to the halt switch | stop halting on dirty trees entirely and let the next tick work in the dirt
- Chose: quarantine: mirror the work to the at-risk box, stash it under a supervisor-subjected message, name both in the journal, clear the tree, and let the pass continue; halt only when the rescue cannot be completed or the same unit goes dirty twice without a tick between
- Why: The halt is right about one thing and wrong about everything else. It is right that a harness must
  not guess at a person's uncommitted edit, which is why `git stash` alone is the wrong verb: #237's
  WIP was a real person's fix for the very outage being lived through, and silently shelving it buries
  it under a subject line nobody greps. It is wrong that the accident is allowed to speak for the whole
  queue. Quarantine changes who bears the cost -- the unit that had the accident waits, the other 22 do
  not -- and keeps every property the halt was chosen for: nothing is rewritten, the work leaves the
  tree only after git confirms the stash exists, and a mirror is written and re-read before the tree is
  declared clean. The mirror is three reads, not one, because "mirror the work" has to mean tracked
  plus staged plus untracked: measured on a fixture holding one of each, `git diff HEAD` alone
  reproduces 1 of the 3 paths and `git status --porcelain` shows 2 of the 3, since it never prints
  staged content. The halt is still the answer for the case that genuinely is ambiguous: a unit that
  goes dirty twice with no tick in between is evidence the quarantine is wrong about that tree, which is
  the original reason for stopping. Refusing to clean a tree it cannot mirror is the same discipline as
  H-0034's `--ff-only` -- an action that cannot be completed honestly reports a failure and leaves the
  tree alone rather than inventing an approximation. What was rejected and why: the journal-line-only
  fix was this issue's own first shape and was dropped because it makes the outage shorter, not
  smaller -- a person still has to notice; the stash-alone arm loses the untracked file with no copy
  anywhere; `reset --hard` deletes a person's work, the one thing the harness exists never to do; a
  per-unit skip list is the quarantine's cost borne by nobody, because the work then stays dirty with no
  copy and no notice; and working in the dirt means the next tick's `git add -A` commits a stranger's
  half-edit -- `git add -A` has swept the built binary into history four times (D-0015's evidence), so
  the harness does not type that command anywhere. Both the streak and the refusal still halt through
  `factory_tree_stop`, and a halt with no `HALT` file would misreport the state of the queue, so the
  marker stays global: what changed here is the accident rate, not the switch.
- Checkpoint: f4379061585a92d9990970b2545db57152a1345d
- Effect: `factory_tree_quarantine` and `factory_tree_stop` in `factory/common.sh`; both dirty-tree
  sites in `factory/factory.sh` (`TICK-HALTED` before a tick, `CONFLICT-HALTED` before a conflict
  commit) now call the stop function, and the `TICK-START` and `CONFLICT-RESOLVED` sites call
  `factory_quarantine_reset`, because a streak counter that only rises turns "halt on the second
  accident" into this issue's outage with a delay built in. The confirmation that the stash took
  everything asks `factory_tree_dirt`, not `git status --porcelain` -- `naive_dirt_test_refused` in the
  selftest caught that read in shipped code on its first run (H-0040). Two bugs the legs found rather
  than the reader: the confirmation first read `if ! confirm=$(factory_tree_dirt ...)`, and the
  verdict's exit codes are 0 dirty / 1 clean / 2 unknown, so as written it refused the two arms meaning
  "nothing is left" and accepted the one meaning "the work is still here"; and `factory_tree_halt`
  re-asked whether the tree was dirty, so the streak halt -- taken only after a successful quarantine,
  on a tree it had just cleaned -- logged `TREE-STREAK` and then let the pass continue, the halt
  evaporating on the condition it had itself removed. A red CI run on the #238 runner found a third:
  the selftest drives the shipped classifier, whose preflight refuses to start without
  `$FACTORY_OPENCODE`, on the host PATH, so a runner with no `opencode` launcher gets zero classifier
  rows and 45 assertions that print nothing; a fixture launcher plus a `the pass really ran the shipped
  classifier` leg -- whose own first version died calling that launcher without `CLAIM_FIXTURE`, the
  stub's own env contract -- makes a stub that never ran a red leg. `make selftest` was added because
  every other entry point to this suite begins with the Go guards, so on a machine with no Go toolchain
  the suite that checks the delivery loop cannot be invoked. `#248` is cited by name in the shipped
  files that carry the behaviour -- `factory/common.sh`, where `FACTORY_AT_RISK`'s own comment is the
  sentence explaining why the box exists, and the two call sites in `factory/factory.sh` -- and the
  rescue names itself where an operator looks: the journal line carries `mirror=<absolute path>`, and
  both the marker and the issue notice render `factory_halt_reason`. There is no per-file at-risk
  checklist to update, and this sentence claimed there was one until the grep showed otherwise; it is
  corrected in place rather than appended, because it was written before the files were. Suite: 242
  legs, 0 failed. Corrected 9 Nov after the review of #250 re-measured the base rather than repeating it: the
   addition is not +66. `ae8aba845` is 39 legs behind this branch's actual base, `main@e0d27d3`, and that
   base measures **216 legs on this box** (215 on the reviewer's -- the +/-1 filesystem-dependent spread
   this repository already documents), so the entry added **+26** legs, and the branch carrying it stands
   at 247 after H-0062..H-0064. `176` is not a wrong count of some head, it is a count of a head this
   suite was never run at, quoted as though it were the base; corrected in place with the date and the
   reason, for the reason the two lines below give. The
   sentence this replaces claimed that `factory/factory.sh:107`'s comment already named this entry for
   the age clause, and that a re-pointed comment would hand the age clause and the per-unit scope to
   `H-0062`. Neither held when it was checked: line 107 is the argument parser, the halt block is
   `factory.sh:113-117`, and the only id that comment names is `H-0036`, for why a halt exits 8 instead
   of reading as a crash. So the age of the marker in the journal line and the per-unit scope of the
   switch stay exactly where this entry puts them -- rejected arms, each with the reason it was refused
   and the reason it is still owed -- and no id is cited for work nothing holds. Corrected rather than
   appended to, for the reason the two lines above give: it was written before the files were, and a
   merged log that quietly agrees with itself is worse than one that admits it was wrong.


## H-0059 · The #249 fixture commits under whatever git identity the host has, and a CI runner has none

- Question: the position section builds its fixture repositories with `git init` and `git clone` and then commits inside two of them. Where does a commit's identity come from, which box is allowed not to have one, and what makes the next occurrence of this fail on the author's machine instead of on the one check that gates a merge?
- Options: plant the identity in each repository at its creation site | export `GIT_AUTHOR_*` and `GIT_COMMITTER_*` for the whole section | pass `-c user.email=… -c user.name=…` on the two commits | give the CI job a git identity | report it and change nothing
- Chose: plant the identity in each repository at its creation site
- Why: read off the runner's own log, not inferred from a green box. At `8772339` the rollup is `verify=FAILURE` with the other four checks green, and the same head prints `verify: OK` here (`236 passed, 0 failed`). The runner's last leg is `every class has a dispatch arm`; the next thing it prints is `Author identity unknown … fatal: empty ident name (for <runner@runnervm7aw5e.…>) not allowed`, then `make: *** [Makefile:95: verify] Error 128`, and across the entire failed log there are **0** lines matching `passed, .* failed` -- the suite ended at its own command and never reached a verdict. Reproduced before any edit by taking the box's identity away (`HOME` pointed at an empty directory, `GIT_CONFIG_GLOBAL` and `GIT_CONFIG_SYSTEM` at `/dev/null`, `GIT_AUTHOR_*`/`GIT_COMMITTER_*` unset): the same last leg, `EXIT=128`, 217 legs printed, no tally. The mechanism is that `git clone` copies the upstream's *remote* and none of its local config, so `user.email` set on `$pos_up` says nothing about a clone of it -- and two top-level statements in the section, `pos_fork` and `pos_detach`, commit inside a clone. Why `main` is green is the part that decides the shape of the fix: every pre-existing commit site is already identity-safe (`selftest.sh:467-469` configures the repository it has just created, `:1460-1461` passes `-c user.email=… -c user.name=…` on the commit), so the discipline predates this branch and #249's section is the first code in the file to break it -- which makes the per-repository answer also the answer the surrounding code already takes. The other four lose on evidence. **Export the env**: a `GIT_AUTHOR_*` export outranks repository config, so it would also override a leg that plants the *absence* of an identity on purpose -- and "no identity" is a state this harness cares about, being exactly the `UNKNOWN` arm the section spends four legs on; per-repository keeps the host out and leaves the env free for the leg that will want it. **`-c` on the two commits**: it fixes the two statements that happen to exist today and nothing else; the bug is a repository created without an identity, and the next leg that commits in it re-opens CI. **An identity in the CI job** fixes one job on one repository and leaves `make verify` -- the gate a reviewer runs on a fresh clone, which is the box that has no identity either -- still red. **Nothing** is the state that cost a merge cycle. What writing the leg found belongs in the record: `no fixture repo inherits the host` scans every `git init`/`git clone` in the file for an identity on the creation line or within three lines of it, and its control deletes the clone's identity in a copy outside the repository. The first version recognised creation lines with `\b`; awk reads `\b` inside a regex as a **backspace**, not a word boundary, so the scan matched nothing -- it called the shipped file clean *and* the control, with its identity deleted, equally clean. The control failed on the first run, and that is the only reason anyone knows the scan was ever blind: the file's own rule is that a presence check has to prove it can say absent. After the fix both other creation sites were checked individually -- deleting either identity line is reported (`2729: git init -q "$pos_up"`, `3085: git init -q "$pos_empty"`), and the shipped file reports nothing. What lost, as measured: the scan reads this file's text, so an identity arriving from a variable, from a heredoc, or four lines below its `git init` reads as a gap -- the window is 3 lines, and `pos_empty`'s comment had to move above the `git init` rather than sit between it and the identity, which is the first judgement the next person will have to re-make; a repository created by a spelling that is not `init` or `clone` on a line starting with `git ` is invisible to it; it pins `factory/selftest.sh` and not the harness, so the same dependence in `scripts/` or `testing/` is out of its reach; and no leg here makes a *first* occurrence loud -- a suite that dies before its tally prints nothing about how far it got, which is what this one's diagnosis had to reconstruct by hand and what H-0060 is about.
- Checkpoint: b89fac0ab20401a6ec1b5e611842281316bcb0f0
- Effect: `factory/selftest.sh` only -- `user.email`/`user.name` planted in `pos_clone()` and at `$pos_empty`, an independence paragraph in the section header beside the H-0054 precedent it follows, and `pos_unconfigured_repos` with the leg `no fixture repo inherits the host`. Legs 235 -> 236, measured twice at this head: `236 passed, 0 failed` on this tree (btrfs, git 2.53.0) and `236 passed, 0 failed` again with `HOME` empty and `GIT_CONFIG_GLOBAL=/dev/null`, the condition that killed the run. No shipped script changed: the red was in the gate, not in the harness.


## H-0060 · A run that dies before its tally leaves no tally behind, so the red tick says nothing about where it stopped

- Question: H-0059 had to be diagnosed from a CI log that ends at a tool's own fatal -- 217 legs printed, no `FAIL` line, no `N passed, M failed`, and no sentence anywhere saying the run had stopped early. What does the suite owe the reader of a run that does not finish, and how is that notice kept from becoming a second thing that can quietly be wrong?
- Options: report the incompleteness from the `EXIT` trap, with the counts and the status | print a running progress line per leg so the log always ends on one | leave it, and diagnose with `bash -x` when it happens | add a CI step that greps the log for the tally line | report it and change nothing
- Chose: report the incompleteness from the `EXIT` trap, with the counts and the status
- Why: the cost is the argument. `make: *** [Makefile:95: verify] Error 128` says a recipe failed; the 217 legs say the run got *somewhere*; nothing says the two facts belong to one failure, and the log's last line is git's error text, which is a symptom of a fixture, not a location in a 3300-line file. The reproduction that found the cause was a re-run of the suite with this box's git identity taken away -- an act only possible on a box that has one, which is the same asymmetry that made H-0059 invisible in the first place. That is the failure this harness exists to end, one level down: #248's whole thesis is that a tick which exits 0 and delivers nothing is worse than one that refuses, and a suite that exits 128 and reports nothing is the same shape wearing a gate. The four others lose on evidence. **A line per leg** buys, in every journal and every PR log, 236 lines that say what was already printed one line ago, to cover the one moment it matters; nobody reads this suite's stdout as it goes -- the supervisor reads it after the fact with no terminal attached. **`bash -x` when it happens** is what was done this time, by hand, on the wrong box, and it only works where the box reproduces; the class is precisely the one that does not reproduce here. **A CI step that greps for the tally** turns "no verdict" into a *second* red check saying the first thing, and it is CI-only -- the same death on a reviewer's fresh clone, which is the box with no identity too, still dies silent. **Nothing** is the state that cost a merge cycle. The trap is the only place that holds both the counts and the status the shell is dying with: measured on the runner, 128, because the command that ended the run was git and not a leg, and a status is the cause's own name. `reached_verdict` is set by the tally line alone, so the notice cannot contradict a run that did report, and it goes to stderr, so it cannot splice a reader of stdout. Both polarities are legs, because one half alone proves only that a word prints for everything: `a death before the verdict is reported` runs the shipped `cleanup()` -- extracted with `awk`, since a leg that pastes a copy of the function tests the paste -- over a planted death and over a clean finish, and asserts the notice, the counts inside it, that the verdict line which *is* in the script after the death was not reached, that the clean run prints the verdict and not the notice, and that the fixture is still gone. Two mutations, each echoed back as applied before the suite ran (H-0055's rule that a mutation which did not happen is not a result): an arm made unable to fire (`if [ 1 = 0 ]`) is caught by the death case with `said nothing about it`, and a notice stripped of its counts is caught by `does not say how far the run got`, which printed the stripped line verbatim. What lost, as measured: a trap cannot report its own deletion -- one `#` in front of `trap cleanup EXIT INT TERM` silences the notice and the cleanup together, and that is the floor `scripts/attack-self-test.py` states for a check policing its own removal, so it is listed rather than fixed; a `SIGKILL`, an OOM kill, or a `timeout -9` runs no trap at all, so the notice covers only a death the shell survives, and a pass killed at the wall-clock ceiling still looks like a machine that is working; the counts are as fresh as the last completed leg, so a death inside a leg's setup reports one fewer than began -- the line answers "how far the tally got", not "how many legs started"; it cannot name the leg that was in flight, which would mean setting a label before all 236 calls and is the next person's judgement, not this one's omission; and the "fixture still gone" assertion rode no mutation -- the two above reach the notice, not the cleanup tail -- so it is a guard written from a hazard rather than one proven red, and it is said here for that reason.
- Checkpoint: 0810fcf0ee4e527293e53ce57eac256f7bd0ab80
- Effect: `factory/selftest.sh` only -- `reached_verdict` beside the counters, the notice in `cleanup()` before the fixture is wiped, the flag set on the line after the tally prints, and the leg `a death before the verdict is reported`. Legs 236 -> 237 at this head.


## H-0061 · Two ids this branch took today are already taken by open siblings, so the second merge turns `main`'s gate red

- Question: `main` stops at `H-0055` and three other branches are open against it, each numbering from the same next-free value. This pass added `H-0057` and `H-0058`, which `issue-235`/`issue-246` and `issue-248` have already taken. Do the two unpushed entries move, and what gets said about an allocator that let four branches take `H-0056`?
- Options: renumber this branch's two unpushed entries to free ids, `H-0059` and `H-0060` | leave the numbers and let whichever merge lands second break `make verify` on `main` | rewrite the two local commits so their `Decision:` trailers match the new ids | give every branch its own id range | report it and change nothing
- Chose: renumber this branch's two unpushed entries to free ids, `H-0059` and `H-0060`
- Why: the table is read off every ref, not guessed, because the collision is only visible across branches: `H-0056` on `issue-235`@`afeb148`, `issue-241`@`63464a4`, `issue-246`@`84ccdf9` and this branch, inherited at `8772339`; `H-0057` on `issue-235`, `issue-246` and, until this commit, here; `H-0058` on `issue-248`@`9547949` and, until this commit, here. `scripts/check-decisions.py:128-131` refuses a repeated id in one log -- `id %s is already used at %s:%d; 'revert to before %s' would name two commits` -- so a merged pair is not untidy, it is a red `verify` on `main` plus a red `verify` in whichever PR has to carry the fix, and the gate's own sentence names why an id is not a label you may reuse: the log is the thing that makes `revert to before H-NNNN` mean one commit. Renumbering is free exactly once and this is that moment -- both entries live in commits that have never been pushed, the remote's `issue-249` is still `8772339`, so no force-push and no in-flight rewrite is involved -- and `H-0059`/`H-0060` were checked against every ref before being taken. Why the four others lose. **Leave them**: it moves a failure that is currently visible from a branch to `main`, which is the exact move this PR exists to stop making, and it does it to the one gate the supervisor cannot skip. **Rewrite the two local commits** so their trailers agree with the file: `AGENTS.md` forbids rewriting an in-flight branch and the strict reading is the right one here, because a `Decision:` trailer pointing at an entry that was renamed is still greppable history while a rebased-away commit is gone; the mapping is recorded in this entry instead. **Per-branch id ranges** is the real answer to the allocator and not this unit's to make -- it edits `scripts/check-decisions.py`, invalidates the numbering of three open branches mid-flight, and belongs to whoever owns the queue. **Nothing** is the state that produced four `H-0056`s. What lost, as measured: the inherited `H-0056` here is untouched and still collides with three siblings, so this branch's contribution to the mess goes from two collisions to one, not to zero, because `8772339` is pushed and the no-force-push rule outranks tidiness; the file now has a permanent gap at `H-0057`/`H-0058`, so anyone who counts entries against the highest id will read two as missing, and only this entry says where they are; the two commits that made those decisions keep `Decision: H-0057` and `Decision: H-0058` in their messages, so `git log --grep='Decision: H-0059'` finds this rename and not the work, and the follow-the-trailer habit breaks once, here, on purpose; nothing stops the next branch taking `H-0056` a fifth time, because the allocator is untouched and this entry is a notice, not a lock; and no leg pins any of it -- a leg that checked for collisions would have to read sibling branches, which is a network read inside a gate `H-0056`'s own entry keeps off the pass path.
- Checkpoint: 594242b3a7026b652dd931b2587c2732eaaca77b
- Effect: `factory/DECISIONS.md`, both entry headers and the cross-reference the first one makes to the second, and the seven comments in `factory/selftest.sh` that cite the old numbers. Re-measured at this head: `check-decisions` 95 entries with no duplicate, `make verify` last line `verify: OK`, `factory/selftest.sh: 237 passed, 0 failed`. The allocator is unchanged, so this is a band-aid over two of the four collisions and is named as one here rather than as a fix.


## H-0062 · A quarantine streak was cleared before the tick that could prove it, so it could not fire on the tick path at all

- Question: The streak is #248's evidence rule -- a SECOND quarantine of the same unit says something keeps dying in that tree, and a third guess is how work gets thrown away. The review of #250 drove the shipped functions in the pass's real order (`factory_tree_stop` -> `TICK-START` -> `factory_quarantine_reset`) over a tree whose tick dies leaving dirt, five passes, and got `attempt=1` five times with no HALT. Where does a streak end, so that it can both end and fire?
- Options: clear it at `TICK-START` as shipped, and re-point the selftest's adjacency pin at whatever shape follows | clear it after the session, gated on the tree reading clean, so a tick that leaves work behind leaves the streak armed | clear it after the session whatever the tree says, on the argument that a tick entering the unit is the progress the counter measures | require two quarantines of one unit inside a single activation instead of across activations | keep a timestamp instead of a counter and halt on two quarantines inside N minutes
- Chose: clear it after the session, gated on the tree reading clean, so a tick that leaves work behind leaves the streak armed
- Why: The shipped line answered a question the counter does not ask. "A tick has entered this unit, so whatever the previous one left behind has been dealt with" is true of the *dirt* -- the quarantine really did move it -- and false of the *pattern*, which is the only thing the counter exists to remember; one sentence asserted the first and destroyed the second. The tie-break that separates the two post-tick shapes is the third one, refuse rather than proceed on missing input: an UNKNOWN tree keeps the streak armed rather than clearing it, because a tree git will not read back is not evidence that the tree is fine (the same direction H-0050's verdict takes), and the next pass refuses at `factory_tree_stop` before anything is stashed, so arming on UNKNOWN costs at most a halt that was the right answer anyway. Nothing earlier in the order separates anything: no dependency, no config decode. The tree's verdict is asked rather than the session's exit status because the question is "did work appear in here again?", and both endings answer it -- a session that exits 0 and one killed at the ceiling are distinguished by what they leave behind, which is exactly what the next pass would have to quarantine. What lost, and why: `TICK-START` is unreachable by construction for a tick unit, measured above, and its own gate was enforcing the defect -- `streak_reset_after_tick_start` required the reset within 8 lines of `TICK-START`, so the check pinned the adjacency that made `attempt=2` impossible; the review's sharpest line is why that went unseen, and it is worth keeping as a rule: a control that deletes the reset proves the reset is *wired*, not that the streak survives being wired. The unconditional post-tick clear loses because it rebuilds the same blindness on a different clock -- a tree that kills every tick would never accumulate a second attempt. Two-quarantines-in-one-activation loses because the outage was five activations, not one. The timestamp loses on the tie-breaks: a second ledger key plus an `N` nobody has measured, on a file where #248's own entry already calls an invented threshold a guess.
- Checkpoint: 1554b3070b7b38040729a11afc9a1ac5d3f2e395
- Effect: `factory_quarantine_after_tick` in `factory/common.sh` (new, returns 0 always so it can never overwrite the tick's own answer); one call site in `factory/factory.sh`, placed after the `timeout ... || status=$?` line so it covers `TICK-DONE`, the early `TICK-UNDELIVERED` return and `TICK-FAILED` alike; the reset is gone from the `TICK-START` block, with the reason left in place of it; the `CONFLICT-RESOLVED` site stays and its comment now says why it is not redundant -- the conflict pass keeps its streak under `pr:$pr` and a conflict tick keeps its own under `conflict:<pr>`, two ledger keys, which was true before this commit and is now written down where someone would mistake them for one counter. Journal: `TREE-AFTER-TICK <unit> streak=cleared|armed ...`; the word chosen does not contain `TREE-STREAK`, because `expect_quarantine`'s STREAK arm greps that string and an armed-but-never-halted pass would have satisfied it -- the collision was found by reading the assertion, not by the suite. `factory/selftest.sh`: the leg the review asked for by name, `a streak survives its tick (primary|worktree)`, composes quarantine -> tick -> verdict -> next pass from the shipped functions and never calls the increment, asserts `attempt=2` halts, and controls it with the same shape where the tick leaves the tree clean -- plus the third arm, a tree reading UNKNOWN from #228's exclude-file loop must not end the streak; `streak_reset_after_tick_start` becomes `streak_reset_after_tick_end` (the call must come after the `timeout` line, not beside `TICK-START`) and gains a second scan, `streak_reset_gated_on_the_tree`, with a control each: deleting the call, and deleting the verdict inside the function. Suite: 244 legs, 0 failed, against 242 at `9547949`. Still owed, and stated because the shape can bite: the armed counter has no wall-clock decay. A unit that is quarantined and then never ticked again -- its PR merged, its branch gone from triage's view -- keeps the streak armed indefinitely, so two accidents a year apart can halt on the second with no tick between them because no tick *could* come between them. `quarantined-at` is already stamped, so the repair is an age test in `factory_quarantine_after_tick`'s armed arm; it is not written here because no measured outage needs it yet and H-0048 is about not inventing a number.


## H-0063 · The mirror rendered a binary path as a sentence about itself, and the size guard could not tell

- Question: The quarantine's rule is "clean a tree only after its contents exist somewhere other than the tree", and the guard that enforces it is `[ ! -s "$tmp" ]`. Measured on git 2.53.0: `git stash show -p --include-untracked` prints `Binary files a/shot.png and b/shot.png differ` for a tracked binary and omits the payload, so a tree holding one modified PNG produced a 45-byte non-empty mirror, the tree was cleaned, and the journal advertised `mirror=<path>` that restores nothing. What makes the claim true?
- Options: pass `--binary` to the `git stash show` that renders the mirror | leave the rendering alone and refuse any mirror whose text contains `Binary files`, treating a binary-bearing tree as unmirrorable and halting | after writing the mirror, verify it by applying it to a throwaway clone and comparing digests, and refuse when that fails | keep the flagless rendering and record the limit in the entry, since `refs/stash` still holds the bytes
- Chose: pass `--binary` to the `git stash show` that renders the mirror
- Why: Measured both ways on this box, git 2.53.0, one modified tracked binary plus one new untracked one: flagless, `git apply` refuses the *whole* patch -- `cannot apply binary patch … without full index line` -- so the text edit in the same tree goes down with the binary; with `--binary`, both paths come back as `GIT binary patch` and a `git apply` into a fresh clone of the base restores all three digests exactly. The tie-breaks that separate the options are the third and the fourth. Refusing on the sentence is the honest *reading* and the wrong *consequence*: committed PNGs are this repository's normal state (`docs/screenshots/360/*.png`), so a killed capture tick would halt every other unit over a rendering flag -- #248's outage re-imported by a check that cannot be satisfied. Verifying by applying is the right *test*, and the wrong place to run it: a clone per quarantine, beside the box that holds the only copies of real work, to re-check something git honoured in every measurement taken here; that verification belongs in the gate, which is where the new leg puts it. Recording the limit is restating a false claim more politely. What the guard was actually doing: `[ ! -s ]` measures presence, and git's refusal is 45 non-empty bytes, so the guard was never the thing that could see this -- the leg is.
- Checkpoint: 28f41bc1f68382956cce174d69f18fdfa83a49ad
- Effect: `--binary` on the one `git stash show -p` in `factory_tree_quarantine` (`factory/common.sh`), its plumbing comment corrected where it claimed the flagless form "renders both halves", and the emptiness comment extended to say what emptiness does not prove. `factory/selftest.sh`: `the mirror carries its binary (capture)` builds a throwaway repo with a committed PNG and a committed text file, modifies the PNG, adds an untracked binary, quarantines it, then applies the journal's own mirror to a fresh clone of the base and compares three digests -- the tracked binary, the untracked binary, the text edit -- because "the mirror restores nothing" and "the mirror takes the text edit down with it" are the same bug and only the second one is loud. Its control re-renders the same stash without the flag and asserts the patch is non-empty AND refused by `git apply`, which is the pair the size guard cannot distinguish; a control that applied would report itself. Suite: 245 legs, 0 failed, against 244 at `9797e7b`. The legs above this one count paths and plant text, which is why none of them could see it. Still owed, adjacent and untouched because the refusal still refuses: on this path stderr and stdout both go to `$tmp` (`err=$(git … >"$tmp" 2>&1)`), so a refused render journals `no mirror could be written to <path> ::` with nothing after the colon -- the reason is in the file that gets deleted, not in the sentence.


## H-0064 · A rescue popped the top of a stash stack it does not own, in a repository where that stack is shared

- Question: `factory_tree_unstash` undoes a quarantine that could not be finished, and it did it with a bare `git stash pop`. Measured on git 2.53.0: `refs/stash` is repo-global -- a primary tree and a linked worktree of the same repository answer the same sha, and one `git stash list` shows both units' entries -- so after another unit's accident the pop restores that unit's work into this unit's tree. `git stash pop "$sha"` is refused (`is not a stash reference`), so the entry cannot simply be named to the pop. How is an undo pinned to the thing it is undoing?
- Options: re-check `refs/stash` and pop only while its top is still the entry made moments ago; otherwise restore nothing and say which sha holds the work | locate the entry's index in `git stash list` and `git stash pop stash@{n}` so the right entry is restored and dropped | `git stash apply "$sha`, which does accept a non-top entry (measured), and leave the entry on the stack | keep the bare pop and treat the caller's subject check as the guard
- Chose: re-check `refs/stash` and pop only while its top is still the entry made moments ago; otherwise restore nothing and say which sha holds the work
- Why: The subject check in the caller is the right check and runs too early -- it proves the entry was ours when we made it, not that it is still on top when we undo; the review named that gap and it is the finding. Popping by index does restore the right entry, and lost on the third tie-break: it writes into a tree whose HEAD may have moved since the stash and whose index may be mid-merge, and its failure mode is a conflict *mid-pop*, which is the half-moved state the refusal arm exists to prevent; refusing and naming the sha leaves the work exactly where it already is safe. `apply <sha>` is what a person is pointed at in the sentence for precisely that reason -- it restores without dropping anyone else's entry -- but as shipped behaviour it leaves our entry on the stack forever, so the stack never drains and the next rescue meets a taller version of the same hazard. Keeping the bare pop was never an option: its failure is silent and it moves a stranger's work into a tree that then looks like its own. Landed with it, both from the same review: the `$FACTORY_RUN/quarantine-error.$$` file is read and removed where it is written instead of below the success path's `return 0` (measured: one zero-byte file named after a dead pid per happy pass), and the function's own promise -- "every refusal leaves the tree exactly as it was found" -- is rewritten to say what the code buys, which is a stash undone by a pop that can fail, plus the measured mid-merge arm (`git stash push -u` refuses a half-merged index and leaves `MERGE_HEAD` and every stage entry intact, so a tree stuck mid-merge keeps halting with the resolution still there for a person).
- Checkpoint: 5e0a9406c11840a1be8fd35d9a5aa8c210132e6d
- Effect: `factory_tree_unstash` in `factory/common.sh` (the pin, and the sentence that names both shas and the `git stash apply` command); `factory_tree_stop` reads git's sentence into `qerr` and removes the file immediately after the call, so the refusal arm prints the reason instead of re-reading a file that no longer exists at a point the success path never reaches; the plumbing comment above `factory_tree_quarantine` carries the honest version of "refused means untouched" and the mid-merge measurement. `factory/selftest.sh`: `a rescue undoes its own stash` puts a second unit's entry on top of ours in a throwaway repo and asserts the pop restores nothing, says `was NOT put back`, and names our sha -- with the control that the same call on its own entry still restores; `a rescue leaves no temp behind` counts `quarantine-error.*` around one successful rescue, with a planted file as its control. Suite: 247 legs, 0 failed, against 245 at `ac5f365`.


## H-0065 · Three ids this branch took are already taken on `main`, so the merge #250 was sent for would have turned `main`'s own gate red

- Question: `#250` reads `CONFLICTING`/`DIRTY` against `main`, and the reason `main` moved under this branch is worse than the two textual conflicts. `93ee5f6` merged #252, whose log entries are `H-0059`, `H-0060` and `H-0061` -- the same three ids this branch took across `3319571..97d5daf` for three unrelated decisions. `H-0058` is this branch's alone, because #252's own `H-0061` deliberately stepped over `H-0057` and `H-0058` to dodge exactly this. Whose numbers move, and what happens to the four already-pushed commits whose `Decision:` trailers name the old ones?
- Options: renumber this branch's three entries up to `H-0062`/`H-0063`/`H-0064`, the next ids free on every ref | renumber `main`'s three instead, since its prose is the younger of the two | keep both pairs and let the merged `main` fail `check-decisions.py` | give every branch its own id range by editing `scripts/check-decisions.py` | report the collision on #248 and change nothing
- Chose: renumber this branch's three entries up to `H-0062`/`H-0063`/`H-0064`, the next ids free on every ref
- Why: The ids are read off every ref, not guessed, because a collision is only visible across branches --
  the discipline `H-0061` names as its whole reason for existing. Measured here over `refs/heads` and
  `refs/remotes`, taking every `^## H-` heading each ref carries: above `H-0055` the union is `H-0056`
  (`origin/main`, `issue-235`, `issue-241`, `origin/issue-246`, `origin/issue-249`), `H-0057`
  (`issue-235`, `origin/issue-246`), `H-0058` (this branch alone), `H-0059`/`H-0060`/`H-0061` (`origin/main`
  AND this branch -- the collision), and nothing above `H-0061`. So `H-0062` is the first id no ref holds,
  and a renumber is free exactly once, before the merge makes someone else resolve it. Why these three lose
  their numbers while `main`'s three keep theirs: `main`'s sit in merged history, and moving them means
  rewriting `main`, which is refused here twice over -- `AGENTS.md` bars the rewrite and both squash and
  rebase merge are off on this repository, so a rewrite is a force-push this agent cannot make. This
  branch's three move as text in one file, no commit moves, so the same escape `H-0061` took is available
  without touching an in-flight branch. What was rejected and why. **Renumber `main`'s**: it moves a
  failure that is currently visible on a branch onto the trunk, the exact move #252 exists to stop making,
  and it does it to the one gate the supervisor cannot skip. **Keep both pairs**: `check()` at
  `scripts/check-decisions.py:128-131` refuses the repeated id for the reason already on the record --
  `'revert to before H-0059' would name two commits` -- and the rule next to it in the same loop,
  `if e["num"] < prev_num`, refuses a merged file that concatenates the two blocks, so a naive merge is red
  twice over, once for the duplicate and once for the order. **Per-branch id ranges** is the real answer to
  the allocator and is not this unit's to make: it edits the gate and invalidates the numbering of three
  open branches mid-flight. **Report and change nothing** is the state that produced this collision. What
  was lost, as measured: the four pushed commits `9797e7b`, `ac5f365`, `3196d2d` and `97d5daf` keep their
  `Decision: H-0059`/`H-0060`/`H-0061` trailers, and once this branch merges those trailers resolve on
  `main` to #252's three decisions rather than to this branch's -- strictly worse than `H-0061`'s case, where
  the ids it abandoned named nothing at all, because here following a trailer does not merely miss the
  commit, it arrives at the wrong decision; the mapping is in this entry and nowhere else. No selftest leg
  pins any of it, for the reason `H-0061` gives: a leg that tested for collisions would have to read
  sibling refs, which is a network read inside a pass `H-0056`'s own entry keeps off the pass path. And the
  allocator is untouched, so the next branch cut from `main` can claim `H-0062` alongside this one; this is
  the third band-aid over the same wound and is named as one, not as a fix.
- Checkpoint: 5ba87a029fa3c879c0349d97535b46843271971a
- Effect: `factory/DECISIONS.md` only, and only what has happened in this commit: the three headers `H-0059`
  → `H-0062`, `H-0060` → `H-0063`, `H-0061` → `H-0064`, the two cross-references inside `H-0058` that point
  at them (the leg arithmetic, now "247 after `H-0062..H-0064`", and the rejected arm that names `H-0062`),
  plus this entry. Read back rather than asserted: `grep -cE 'H-00(59|60|61)' factory/DECISIONS.md` prints
  `0`, and `python3 scripts/check-decisions.py` answers "95 entries across 2 log(s)" with rc 0 at this
  head. The merge is the NEXT commit and is deliberately not claimed here, because an `Effect:` naming a
  merge that has not run is the same prophecy this log has now been corrected twice for; that commit
  interleaves #252's `H-0056`/`H-0059`/`H-0060`/`H-0061` with this branch's `H-0058`/`H-0062`/`H-0063`/
  `H-0064` in the ascending order the gate's ordering rule demands, and keeps both sides' additions in
  `factory/common.sh`.


## H-0066 · Stop is a reaping, not a second copy of the pass ceiling

- Question: `install-timer.sh` renders `TimeoutStopSec` and `TimeoutStartSec` from one token,
  `STOP=$((FACTORY_BUDGET + 60))` -- 5460 s on this box. On 8 Nov one leaked grandchild held the unit in
  `stop-sigterm` and the queue did not fire for 1 h 07 min. What should the stop deadline be, and where
  should its number come from?
- Options: keep one token and bound the outage somewhere else | give stop its own token at a fixed 60 s
  and leave start budget-derived | give stop its own token with a `FACTORY_STOP_TIMEOUT` override
  defaulting to 60 s | set `KillMode=mixed`, or `process`, so a stubborn descendant cannot hold the
  cgroup, and leave the deadline alone
- Chose: give stop its own token at a fixed 60 s and leave start budget-derived
- Why: the two settings bound different moments and the shared token made them look like one concern.
  `TimeoutStartSec` fires *while* the pass runs -- it is the deadline that would kill a tick mid-commit,
  which is the hazard the comment on the stop line had cited, for the wrong setting -- and
  `docs/factory.md` repeated it. `TimeoutStopSec` begins once `ExecStart` has exited: the pass has
  printed its last line, and the only work left is reaping. Measured 8 Nov, issue #246:
  `END rc=4 ticks_started=1` at `22:18:51Z`, then `ActiveState=deactivating SubState=stop-sigterm
  MainPID=0 ControlPID=0 TasksCurrent=10` held by PID 1693618, `C:\windows\system32\winedevice.exe`,
  started 14 s into the activation; the timer the whole while `ActiveState=active SubState=running`,
  `TriggerNextElapseUSecRealtime` empty, `-` in `list-timers`' next-fire column, and the journal holding
  zero lines for the unit after 22:18. Recovered at `23:06:55Z` by an escalated `kill -s KILL` at the
  manager, after which the timer re-armed in the same second. Because `OnUnitInactiveSec=` counts its
  interval from the moment the unit finishes going inactive, the outage *is* `TimeoutStopSec`: 5460 s,
  stopped at 1 h 07 min only because a person killed it. At 60 s the same grandchild costs a minute and
  nothing else. Option three is refused because every other number in this generator is a fact about the
  machine -- interval, budget, reviewer, where the binaries live -- and this one is not: it is how long
  the harness will wait for a process group to die. A knob for it is also one setting away from
  re-merging the two numbers, and the merge is the defect, so the tie-break order lands on the choice
  that touches the fewest files and cannot be un-fixed by an environment variable. `KillMode` is the
  right instinct -- kill the group, stop waiting -- and is refused on evidence rather than taste: under
  `mixed` the escalation is still bounded by `TimeoutStopSec`, so it removes the wait only for a
  descendant that would have taken `SIGKILL` at all, while a process in uninterruptible sleep holds the
  cgroup to the deadline whatever the mode; `process` is worse, because leaving a tick's orphans running
  is precisely the dirty worktree the next pass halts and refuses to touch. Both change what is killed on
  *every* stop, including manager shutdown, and neither can be measured by a tick on this box: a
  manager call is denied to this agent in both spellings, so picking one would be picking a manual page
  nobody here re-ran. The issue draws the same line -- "the answer is a bounded stop, not a smarter
  kill". What lost, as measured: the machine is not yet the repository.
  `~/.config/systemd/user/parental-lockout-factory.service` still reads `TimeoutStopSec=5460` and
  `TimeoutStartSec=5460` (read this session, its lines 51-52), because putting 60 s on the box needs
  `install-timer.sh --install`, which is a manager write a tick may not make; H-0037 is why that
  reinstall must also restart the running timer, and it does. The live reproduction the issue asks for
  has therefore not been run -- H-0067 records what the shipped gate proves instead, and the exact
  commands still owed. And 60 s is a judgement, so the gate pins a *bound* (120 s) rather than this
  constant: a measured argument can move the number without editing the check.
- Checkpoint: 4cfa83abc7b4e8a2dfbd79e86513ccc59f86b5f1
- Effect: `STOP_TIMEOUT` and its substitution in `render()`, and the service template's `TimeoutStopSec`
  line and its comment, in factory/install-timer.sh; in docs/factory.md, the unit excerpt's two
  deadlines and their comment, the account of the 8 Nov outage with the `reset-failed` line and the
  status read a watchdog owes, and `TimeoutStartSec` in the sentence about what `--install` derives.
  This entry was written as `H-0056` and the commits that made it carry `Decision: H-0056`; `H-0068`
  says why the number moved, and `H-0056` on `main` is a different decision -- the stale-tree refusal.

## H-0067 · The deadline gate renders the shipped generator; it does not read its template

- Question: H-0066 splits one rendered number into two. What has to go red for the two deadlines to be
  re-merged, and where does a check that can see that live?
- Options: grep the template in `install-timer.sh` for the two `Timeout…Sec=` lines | run
  `install-timer.sh --install --dry-run` from `factory/selftest.sh` against a throwaway repo with stub
  `gh` and `opencode` on the PATH, and assert on the unit it prints | cut a throwaway service/timer
  pair under a transient user scope at `TimeoutStopSec=5s`, measure the stop, and put that in the gate |
  ship no gate and leave the two lines beside a comment that says not to merge them
- Chose: run `install-timer.sh --install --dry-run` from `factory/selftest.sh` against a throwaway repo
  with stub `gh` and `opencode` on the PATH, and assert on the unit it prints
- Why: the defect is a substitution, not a line of text, and three different regressions leave the
  template looking exactly right -- point the stop token back at `@STOP@`, point the start token at the
  stop token, or delete the substitution line and ship a unit carrying a literal `@STOP_TIMEOUT@`, which
  a manager reads as no deadline at all. A grep over the template passes all three. The generator is the
  only thing that knows what a token becomes, and `--dry-run` is the call that answers without writing:
  it prints both units and exits before the first manager call. The two stubs are what makes the leg run
  anywhere -- `install-timer.sh` refuses to start unless it can find `gh` and `opencode` on its PATH,
  and this gate has no business needing a GitHub token or the CLI -- and neither stub is ever asked to
  do anything, because the dry run never reaches a manager call. `FACTORY_ROOT` and `FACTORY_DIR` both
  name the copy so that even `factory_paths`' three `mkdir`s land in `$work`: a gate that dirties the
  tree it is checking can halt the very pass that runs it. Six legs, each with its own plant in the
  opposite polarity -- a copy of the generator carrying the bug, which the same predicate has to refuse
  -- because a check that cannot say absent is the check being blind, and that is how a green gate
  coexists with the outage it was written for. One leg needs a second render to mean anything: at
  `FACTORY_BUDGET=9000` start must move and stop must not, which is the only way to tell two derived
  numbers from two literals that happen to differ. The bound asserted on stop is 120 s, not the shipped
  60: the number *is* the outage a leaked descendant causes, so the gate refuses anything ceiling-sized
  while leaving room for a measured argument about the constant.
  What this gate cannot prove, on its own line so nobody mistakes it for the measurement: the live
  reproduction the issue asks for has not been run. A manager call is denied to this agent in both
  spellings, and this session measured the denial on the read-only form of it -- a `show` of the
  service asking for `ActiveState`, `SubState` and `TasksCurrent` came back `Permission denied: shell`.
  So no claim here rests on observed stop behaviour: not that 60 s reaps what 5460 s waited for, nor
  that the unit goes inactive a minute after the last leaked descendant dies. The commands owed are in
  the pull request body, and they are the issue's own pair: a transient unit whose `ExecStart` leaves a
  detached descendant and exits, once at `TimeoutStopSec=5s` -- assert inactive inside about 5 s, and
  the paired timer's next-fire non-empty again -- and once at 5460, which is tonight's outage and must
  come back `stop-sigterm` with an empty next-fire. That second arm is the one that makes the first one
  mean something. Using another manager client to get around the denial was considered and refused: the
  denial is on the manager, not on one binary's name, and routing a refused call through a different
  program is how a harness gap turns into a trust problem.
- Checkpoint: b05070735f644f2e297863489f2dd85cb06144bc
- Effect: the "unit's two deadlines" section of factory/selftest.sh -- `gen_setup`, `gen_dry_run`,
  `gen_split`, `deadline_value`, `budget_default`, six `leg_*` predicates, `gen_leg`, `gen_planted`,
  `gen_control` and `unit_deadlines` -- 216 -> 222 passing legs with six plants beside them, and the
  header's claim about how many legs read shipped text rather than a planted tree. This entry was
  written as `H-0057`, which `issue-235` also holds; `H-0068` says why the number moved.


## H-0068 · The two ids this branch took are taken on `main` too, so the merge this pull request asks for would have turned `main`'s gate red

- Question: `origin/main` is at `ef99ae2`, 30 commits ahead of this branch's merge-base with it.
  `git merge-tree`
  resolves every file but `factory/DECISIONS.md`, and the reason is worse than a textual conflict: the
  merge puts `H-0056` (this branch's stop deadline) beside `H-0056` (`#252`'s stale-tree refusal), so
  the merged log has one id naming two decisions. Whose numbers move, and what happens to the two
  already-pushed commits whose `Decision:` trailers name the old ones?
- Options: renumber this branch's two entries to the next ids free on every ref, `H-0066`/`H-0067` |
  renumber `main`'s `H-0056` instead, since this branch took its number first | keep both pairs and let
  the merged `main` fail `check-decisions.py` | give every branch its own id range by editing
  `scripts/check-decisions.py` | report the collision on #246 and change nothing
- Chose: renumber this branch's two entries to the next ids free on every ref, `H-0066`/`H-0067`
- Why: the refusal is measured on the merged bytes, not argued. `git merge-tree --write-tree
  origin/main HEAD` at `84ccdf9` prints `CONFLICT (content): Merge conflict in factory/DECISIONS.md`
  and nothing else, so its merged blob was extracted and run through the gate:
  `python3 scripts/check-decisions.py --log <merged blob>` answers **four** problems, exit 1 -- `id
  H-0056 is already used at :1122; 'revert to before H-0056' would name two commits`, two `out of
  order` rows, and the cross-log `one id, 2 decisions; a citation to it does not resolve`. So this is
  not untidiness deferred to the person who resolves the conflict; it is `main`'s own `make verify` red
  after a green merge, in the one gate the supervisor cannot skip. The ids were read off every ref
  before any was taken, which is the discipline `H-0061` names as its whole reason for existing: above
  `H-0055` the union over `refs/heads` and `refs/remotes/origin` is `H-0056` (`origin/main`,
  `issue-235`, `issue-241`, `issue-246`, and the already-merged local `issue-248`/`issue-249` and two
  backup branches), `H-0057` (`issue-235`, `issue-246`), `H-0058`..`H-0065` (`origin/main`), nothing
  above `H-0065`. `H-0066` is therefore the first id no ref holds. That measurement was re-taken
  mid-session and moved: the first read had `origin/main` at `93ee5f6` and its ceiling at `H-0061`, and
  by the time this entry was written `#250` had merged `issue-248`, which renumbered *its own* three
  entries to `H-0062`..`H-0064` and filed the notice as `H-0065` -- the same defect, third time in
  twenty-four hours, on three branches that each did the thing `H-0061` told them to do. Why the four
  others lose. **Renumber `main`'s**: its id sits in merged history and moving it means rewriting
  `main`, barred twice over -- `AGENTS.md` forbids the rewrite and both squash and rebase merge are off
  on this repository, so a rewrite is a force-push this agent cannot make and that no `git revert`
  undoes. **Keep both pairs** moves a failure that is currently visible on a branch onto `main`, which
  is the exact move this harness exists to stop making (the words are `H-0061`'s, and `H-0065` re-made
  the same argument for the same three ids one branch later). **Per-branch id ranges** is the real
  answer and is not this unit's to make: it edits `scripts/check-decisions.py`, invalidates the
  numbering of three open branches mid-flight, and belongs to whoever owns the queue; the tie-break
  order therefore lands on the option that touches the fewest files and is reversible by `git revert`
  alone. **Nothing** is the state that produced four `H-0056`s. The yield is recorded inside both moved
  entries' `Effect:` prose rather than in a new `Id:` field, because that is how `D-0050` was resolved
  at `392f451` and because the field set `check-decisions.py` recognises must stay the five it declares
  plus `Awaiting` and `Effect`. What lost, as measured: the two pushed commits keep `Decision: H-0056`
  and `Decision: H-0057` in their messages -- rewriting them is the same barred force-push -- so
  `git log --grep='Decision: H-0066'` finds this rename and not the work, and the follow-the-trailer
  habit breaks once, here, on purpose; `issue-235` and `issue-241` still hold their `H-0056`s and
  `issue-235` its `H-0057`, so this branch's contribution to the mess goes from two collisions to zero
  only for the merge it is asking for, and the next sibling to merge still has to do this dance; a
  fourth branch can take `H-0066` an hour from now, because the allocator is untouched and this entry
  is a notice, not a lock -- the only real fix is a reservation the gate can see, and no leg can be
  written for it without putting a read of sibling branches (a network read) inside a gate that
  `H-0056` on `main` deliberately keeps off any fetch path; and `main`'s permanent gap at `H-0057`
  means anyone counting entries against the highest id will read one as missing.
- Checkpoint: e3903d1053f028a408841e59aa860b8972b47d00
- Effect: `factory/DECISIONS.md` -- the two entry headers, the cross-reference each makes to the other,
  and the two sentences above that say where the old numbers went; one cite in `docs/factory.md` and one
  in `factory/selftest.sh`, both pointing at the stop-decision. No shipped script changed: the collision
  is in the log, not in the harness. Re-measured at this head: `check-decisions: 93 entries across 2
  log(s), clean`.


## H-0069 · Merging `main`'s #249 arm turned all six deadline legs into SKIPs, so the fixture became a tree the generator would arm and a refusal to render became a FAIL

- Question: `origin/main` brought #249 into `install-timer.sh`: the generator now asks
  `factory_supervisor_position "$FACTORY_ROOT"` before it prints a line, and exits 1 on a tree that is
  not its own trunk. This section's fixture is a `git init` with no commit and no trunk ref, and its
  creation site plants no git identity. What is the fixture now, and what does the section do when the
  generator declines to render -- because the answer it had was a SKIP.
- Options: make the fixture a tree the generator would arm (a commit, and `origin/main` at it), plant
  the identity where the repository is created, and fail the section when the generator refuses |
  keep the SKIP bargain and add only the missing commit | aim the legs at a rendered unit checked into
  the tree instead of the live generator | point `FACTORY_ROOT` at this real worktree, which is a
  checkout with a trunk | delete the six legs and rely on #249's own legs, which now cover the generator
- Chose: make the fixture a tree the generator would arm (a commit, and `origin/main` at it), plant the
  identity where the repository is created, and fail the section when the generator refuses
- Why: both halves of this are measured at `3b56a05`, on the merge commit with the log resolved and
  nothing else touched. `bash factory/selftest.sh` there prints `FAIL  no fixture repo inherits the
  host   fixture_carries_identity` and `SKIP  the unit's two deadlines   install-timer.sh --dry-run
  refused to run here: install-timer: /tmp/plock-factoryselftest-M86pkW/genrepo is not a readable git
  checkout`, and the section's own closing line -- `Every check below needs the generator; none of them
  is a verdict on the unit.` -- is the most honest sentence in a suite that then went on to print
  `267 passed`. That is #246's defect wearing a gate: not a wrong number, a check that stopped looking
  while the tally kept saying the checks ran. Six legs read an empty file and reported nothing at all.
  So the SKIP bargain, which was written for a different case, is withdrawn from both of its arms here.
  It was correct where it came from: a tree that has no `install-timer.sh` has no subject, and demanding
  one is a failure of the test, not of the code. Status 2 stays reserved for exactly that, and
  `leg_generator_accepts_the_fixture` refuses to offer it, because here the subject is in this tree and
  it is the *gate* that cannot see it -- the same distinction `leg_no_token_left_unrendered` draws
  against its own SKIP arm. Why a real commit and a real `refs/remotes/origin/main` rather than a stub:
  #249's verdict is a reading of git, and a fixture that fakes it would make the legs read a fixture
  instead of a repository -- `git commit --allow-empty` plus `update-ref` is four seconds. Why not
  `$FACTORY_ROOT` = this worktree: this branch is 30 commits behind the trunk right now, so the legs
  would go red on every merge and green only in the window after a rebase -- armable by construction is
  the property, armable by luck is not. Why the identity is planted at the creation site and not
  exported for the section: H-0059's rule, quoted at the line itself, and the leg that enforces it
  arrived on the trunk side of this merge and refused this fixture the first time the two met. Why not a
  checked-in rendered unit: `install-timer.sh` is the only thing that knows what a token becomes, and an
  artifact committed as a source is the failure mode H-0060 was written to refuse. Why not delete the
  six legs: #249's legs pin the *refusal*; nothing there says which number is on which deadline, which
  is the whole of #246. What lost, as measured: `gen_leg`'s FAIL tail changed from "the rendered unit is
  wrong:" to "the check refused:" -- one line of shared vocabulary the six legs did not ask for, chosen
  over a per-call wording so the helper stays one function. The section now builds a second, unarmed
  fixture per run, so a failure there costs one more refusal to diagnose. `gen_control_unarmed` borrows
  `$gen_tree` and hands it back, so a future leg added *after* it that assumes the armed tree is
  correct today and wrong the day someone reorders the section. And `arm` is an argument, not a type: a
  caller that omits it gets `arm=` , which is not `1`, which is an unarmed fixture, which is a FAIL --
  the loud direction, but a new way to break this section that only the leg now catches. The checkpoint
  is the merge commit and not its parent: the state this entry refuses is the merge with the blind
  fixture in it, so that is what a revert has to land on.
- Checkpoint: 9c96f3c7b82b717ae8121496ee3ec266a5e1d4d5
- Effect: `factory/selftest.sh` -- `gen_setup` takes `<dir> <arm>` and now plants `user.email`/
  `user.name` beside its `git init` and, at `arm=1`, an `--allow-empty` commit and
  `refs/remotes/origin/main` at it; `gen_dry_run` renders against `$gen_tree`; new
  `leg_generator_accepts_the_fixture` and `gen_control_unarmed`; both SKIP arms in `unit_deadlines`
  replaced by FAILs; `gen_leg`'s FAIL tail; two new paragraphs in the section header. Re-measured, same
  command before and after: `267 passed, 1 failed` with this section reporting one SKIP and reading
  nothing, to `275 passed, 0 failed`, eight to the tally -- the identity leg the merge broke and the
  seven this section prints now that it renders. The section itself prints fourteen `ok` lines, seven
  legs and seven controls, the controls not counted in the tally.


## H-0070 · The decision-id allocator reads every local branch ref, so a sibling's number is taken before this checkout meets it

- Question: the skill told every session to "number from the highest id in the file you are writing
  to," and `check-decisions.py` only judged the merged result. Three sessions followed that sentence
  literally, saw the same H-0060, and all wrote H-0061; the notice filed at H-0068 says plainly that
  "the allocator is untouched" and that a fourth branch can take the same number. What shared
  allocator replaces prose without turning decision numbering into a second delivery queue?
- Options: add `factory/next-decision-id.sh`, document it as the one allocation command, and pin its
  ref union and gap rule in `factory/selftest.sh` | move allocation into `scripts/check-decisions.py`
  so the existing gate also chooses the id | keep the prose and merely add "fetch and inspect every
  branch" | give each branch its own id range by changing the gate's id grammar | do nothing and keep
  renumbering after a collision is discovered
- Chose: add `factory/next-decision-id.sh`, document it as the one allocation command, and pin its
  ref union and gap rule in `factory/selftest.sh`
- Why: the defect is a read with the wrong scope. The old prose was accurate about one checkout and
  blind to the queue: `check-decisions.py` correctly refuses an id twice in a log and one id in both
  logs, but it reads the two working logs at the end of a session, when a branch's own new entry has
  already been written. H-0068 measured the cost -- H-0065/H-0066's renumbering, then this branch's
  H-0068/H-0069 renumbering -- and said the missing piece was a shared allocator. The new command asks
  git for the working file, `HEAD`, every `refs/heads` and every `refs/remotes` blob, then returns one
  above the selected prefix's highest id. The selftest proves the union rather than trusting the
  phrase: the current checkout remains at D-0001/H-0010 while local and remote refs hold D-0003/H-0011;
  the allocator answers D-0004/H-0012. It then plants exactly D-0004/H-0012 on a second remote-only ref
  and the answer moves to D-0005/H-0013, the H-0061 race caught locally. A planted allocator that drops
  all branch refs falls back to D-0002/H-0011, and one that drops `refs/remotes` falls back to the
  colliding D-0004/H-0012 -- so neither ok is the fixture agreeing by accident. D-0002 stays absent in
  the first fixture and is deliberately not chosen: retired and renumbered ids are permanent
  reservations, so "first free" means above the ceiling rather than the smallest gap. The command reads
  refs already on the machine and fetches nothing, following #249's staleness verdict: H-0013 and
  H-0056 already price a blocking read in a tick budget. Why the four alternatives lose. **Move
  allocation into `check-decisions.py`** puts a write-shaped question in a shape gate and still runs
  too late; an allocator must be callable before the checkpoint named in the entry. **Fetch in the
  allocator** makes an ordinary harness command depend on network timing and recreates the parked-read
  hazard the harness explicitly rejects. **Per-branch ranges** requires a new identity scheme and
  changes what every historical `H-NNNN` citation means -- larger than this ticket, as H-0061 and
  H-0068 both concluded. **Keep the prose** was measured to fail three times. What lost, as measured:
  this is a collision-free read, not a reservation -- two processes can still call it before either
  commits and receive the same answer, so the skill must re-run it immediately before the entry commit;
  refs that have never been fetched are invisible, so the unit still needs an up-to-date trunk (the
  allocator says so in its help and docs/factory.md names the operator's `git fetch --all --prune`);
  it now reads every ref blob, so allocation costs one `git show` per local ref rather than one
  heading scan; and `make verify` cannot close
  the cross-branch race on its own because the same two ids may each pass independently in separate
  worktrees. The selftest's local collision pins detection of a stale read, not prevention of a
  same-instant race.
- Checkpoint: 38ddd0bddcb47bd5151eec3e44951b385b76ecd0
- Effect: new `factory/next-decision-id.sh` (executable); `.opencode/skills/factory/SKILL.md` step 3
  now allocates before checkpointing and re-reads before committing; `docs/factory.md` gains "Decision
  ids are allocated against refs, not prose"; `factory/selftest.sh` gains seven counted legs and four
  counted controls. Re-measured after the write: `bash factory/selftest.sh` moved from the pre-change
  `275 passed, 0 failed` to `282 passed, 0 failed`; `factory/next-decision-id.sh factory/DECISIONS.md`
  printed H-0070, and `make check-shell` reported 28 scripts.


## H-0071 · The allocator's re-read is written as what it can see: a sibling worktree is visible, an unfetched push is not

- Question: H-0070 shipped the allocator and the skill's new step closed with "the second read catches
  a sibling merged or pushed while the session worked". One half of that sentence is true and one is
  not. Which half does the re-read actually cover, and what goes in the claim?
- Options: leave the sentence as written | rewrite the skill and `docs/factory.md` to the measured
  boundary and pin both halves in `factory/selftest.sh` | make the allocator fetch, so the claim
  becomes true | give the allocator an opt-in `--refresh` and tell the tick to use it | have
  `factory.sh` fetch the trunk before every dispatch
- Chose: rewrite the skill and `docs/factory.md` to the measured boundary and pin both halves in
  `factory/selftest.sh`
- Why: measurements first, in two fixtures under the selftest's own temp tree. Same clone, second
  worktree: the sibling commits `D-0009` in its own worktree, and the primary answers `D-0002` before
  that commit and `D-0010` after it, with no fetch anywhere in between -- worktrees share
  `refs/heads`, and every unit tree this harness makes is a worktree (`common.sh`'s `git worktree
  add`), so for the concurrency this machine actually runs, the re-read is real. Two clones: origin
  holds `D-0007` on a branch delivered by a second copy, and the first copy answers `D-0002` -- its own
  stale ceiling, while origin is already at `D-0007` -- then `D-0008` after one `git fetch`. An
  unfetched push never moves `refs/remotes`, so `merged` was true and `pushed` was not. The two
  documents also disagreed with each other: `docs/factory.md` already told the operator to refresh
  while the skill said the window was shut. What the false half costs: a session that believes it
  re-reads, gets the same number back, and concludes no sibling exists -- on a queue that runs on two
  machines that is issue #253's failure again, this time with a green allocator behind it. Tie-break:
  fewest files, reversible by `git revert` alone, and the shape the surrounding code already takes --
  `factory/common.sh`'s staleness verdict reads refs as this checkout last learned them and says so
  beside the code, and the conflict path fetches explicitly where it must (`git fetch origin "$base"`
  in `factory/factory.sh`). Why the four alternatives lose. **Leave the sentence** is the defect.
  **Fetch in the allocator** and **`--refresh`** both put a blocking network read in front of a per-tick
  budget, the hazard H-0013, H-0056 and #879 price, and the only use of an opt-in flag is that ticks
  opt in. **A fetch before every dispatch** reopens H-0056, a standing decision about the supervisor's
  ref policy, to shrink a window this entry documents instead; it is the follow-up named in the pull
  request, not this commit. The selftest now carries both shapes as behaviour instead of prose:
  `a sibling worktree is refused its id` answers `D-0010` off the sibling's own commit, with the
  narrower-read control falling back to `D-0002`, and `an unfetched sibling push is invisible` asserts
  both the blind answer and the fetched one -- the second half is what stops the first from being a
  fixture that never met origin at all. `the shared allocator is documented` now requires the refresh
  command in both documents as well as the path, and its control deletes every line naming the refresh
  and demands a refusal. What lost, as measured: the cross-machine window is still open until someone
  fetches -- this commit makes it legible, not shut; the fixture's origin is a local bare repository,
  so no leg measures real network latency or a fetch that hangs; and an allocator on a machine that has
  never contacted this origin still cannot see a sibling there, which is the same read-not-lock limit
  H-0070 recorded.
- Checkpoint: 5416659a40a4fe9ef0d1b5d84f9fb0f9e269e71b
- Effect: `.opencode/skills/factory/SKILL.md` step 1 states the boundary with both measurements and
  names `git fetch --prune origin` for a multi-machine queue; `docs/factory.md` gains the two-row table
  of what is seen and what is missed; `factory/selftest.sh` gains two counted legs and two counted
  controls, and its allocator doc-check now reads the refresh as well as the path. Re-measured after
  the write, from the files as committed: `bash factory/selftest.sh` moved from `282 passed, 0 failed`
  to `284 passed, 0 failed`; `make check-shell` reported 28 scripts with 0 findings; both documents read
  back with the sentences named above in them before this entry was written.


## H-0072 · The decision gate stays ref-blind, and what it may not read is now pinned twice

- Question: #253's second instruction is that `scripts/check-decisions.py` must not start reading
  sibling refs, because that makes the verdict depend on what this checkout last fetched. Nothing
  enforces it -- the rule lives in the issue, in this log, and in one comment beside the git call. What
  replaces the sentence, and with what shape of pin?
- Options: leave the instruction as prose | pin it with a text scan for ref-shaped git calls in
  `factory/selftest.sh` | pin it on behaviour, with a fixture repository whose sibling branch repeats an
  id run through a copy of the shipped gate | pin it with both, since the scan is breadth and the
  fixture is the property | move the ref union into the gate so it warns about a cross-branch duplicate
  instead of only at the merge
- Chose: pin it with both, since the scan is breadth and the fixture is the property
- Why: the two pins answer different questions and each can pass without the other. The scan
  (`the gate reads no branch refs`) refuses any git subcommand that is not `cat-file` and any
  `for-each-ref`, so a fetch or a `git show <ref>:path` added years from now for an unrelated reason is
  refused on the day it appears; its control appends a ref-enumerating vector to a copy of the gate and
  requires a refusal, because a scan that cannot see a planted one is blind, not clean. The fixture
  (`a sibling id leaves the verdict alone`) measures the property itself: a repository of the gate's own
  shape -- a byte-identical copy of the script at the same relative path, since it resolves both logs
  and the checkpoint lookup against its own directory -- whose sibling branch repeats `D-0001` while
  the working log is clean. The shipped gate answers clean; the same duplicate put into the working log
  is answered with `id D-0001 is already used`, and that second half is what stops the first from being
  a fixture the gate never read. Discrimination was then measured rather than argued: with a ref-reading
  arm planted in the working copy -- the exact change this entry forbids, `for-each-ref refs/heads` plus
  `git show <ref>:DECISIONS.md` fed through `check()` -- both legs go FAIL together and the selftest
  falls from `286 passed, 0 failed` to `284 passed, 2 failed`, the diagnostic naming
  `refs/heads/sibling:11`. The restored file compares byte-identical to the backup taken before the
  plant. Why the four alternatives lose. **Prose** is what this and H-0071 are both corrections for.
  **The scan alone** never shows the verdict actually holds on a tree that contains a collision, so it
  would keep passing if a future refactor read refs through a helper it does not pattern. **The fixture
  alone** refuses only a ref read that changes an id verdict; a `git fetch` bolted onto the gate for
  some other reason would pass it and still make the gate a network client. **Move the union into the
  gate** is the hazard itself, and it was measured: one commit, `82bbe713`, in two checkouts that differ
  only in which branches they hold -- with every branch present the ref-reading gate reports
  `1 decision-log problem`, and in a single-branch clone of the same commit it reports none; the shipped
  gate reports none in both. A gate whose answer is a function of `actions/checkout`'s depth, or of the
  last person to fetch, gets re-run until it agrees, which is how a red gate gets ignored. Tie-break:
  the option that refuses rather than proceeds (a clean answer from a tree that hides its refs is
  exactly the wrong answer), and the one that touches the fewest shipped files -- both legs are in
  `factory/selftest.sh`; the gate, the allocator and `make verify`'s shape are untouched. What lost, as
  measured: the scan is textual, so a ref read spelled through a variable holding `["git", "for-"
  "each-ref"]` would slip past it -- the fixture is what is load-bearing, and it is the leg that caught
  the plant; the fixture's verdict is about *id* rules only, so a future rule that legitimately needs
  the object store stays allowed by design (`Checkpoint:` resolution reads objects, not refs); neither
  leg can stop a *human* wiring the union into the gate and dismissing the FAIL -- what stops that is
  this entry and the merge that has to pass `make verify`.
- Checkpoint: 1d61a4113c50680af4f2941024f276e69f97dabf
- Effect: `factory/selftest.sh` gains two counted legs and two counted controls --
  `the gate reads no branch refs` with the planted-vector control, and
  `a sibling id leaves the verdict alone` with its own in-fixture falsifier; its header list gains the
  gate row. `docs/factory.md`'s decision-id section now states the gate's limit as a rule with its
  reason. No shipped check changed: `scripts/check-decisions.py`, `factory/next-decision-id.sh`,
  `factory/factory.sh` and the Makefile are untouched by this commit. Re-measured after the write, from
  the files as committed: `bash factory/selftest.sh` moved from `284 passed, 0 failed` to
  `286 passed, 0 failed`; `make check-shell` reported 28 scripts; the plant and its reversal are
  recorded above.


## H-0073 · The allocator handed `H-0001` to a mistyped path and the caller's own log to a bare `--repo`

- Question: #255 shipped #253's first instruction and left the ticket open for the race, so before
  deciding anything the shipped allocator was called the way a session calls it -- including the paths
  a session gets wrong. Three such calls, measured on `91d992d`: `--prefix H factory/DECISION.md`
  printed `H-0001` and exited 0; the same path without `--prefix` exited 2 with stdout *and* stderr
  empty; `--repo` aimed at a checkout that has no log printed `D-0052`, this repository's own ceiling,
  once with a commit in the fixture and once with none. What is an allocator to do when the union it
  has read is empty?
- Options: refuse when the named log is in neither the working tree nor `HEAD`, and resolve a relative
  path against the repository named with no second reading | keep answering from the empty union and
  let `check-decisions.py` catch the collision at `make verify` | create the missing log and allocate
  `D-0001` from it, which is what "fresh adoption" was documented to mean | keep the fallback to the
  caller's path and document that `--repo` is advisory
- Chose: refuse when the named log is in neither the working tree nor `HEAD`, and resolve a relative
  path against the repository named with no second reading
- Why: an empty union has exactly one answer available, `<prefix>-0001`, and both logs opened with
  that number -- so an empty union is never an empty log, it is a wrong path, and the one thing this
  command exists to refuse is the collision it was handing out. The second call is worse than the
  first: `prefix=$(sed ... | head -1)` fails on a missing file, a command-substitution assignment
  inherits status 2 under `set -euo pipefail`, and `set -e` ends the script before any message. A
  refusal with nothing on either stream is indistinguishable from a broken tool, and a session
  holding a broken allocator numbers by eye -- which is the prose allocator this ticket exists to
  replace, arrived at by a route no leg can see. The tie-break order decides it at its third rung,
  "refuse rather than proceed when input is missing or empty"; it also proves the documented arm was
  fiction. `docs/factory.md` and the code comment both promised `D-0001` for "--repo pointing at a
  branch before the log exists", and the measurement answers `D-0052`: the fallback took the read to
  the caller's log before the default was ever reached, which is also why the seven legs and four
  controls #255 shipped never saw the fallback -- every one of them writes its fixture's log and
  passes `--prefix`, so neither dead arm had a caller. Letting the gate catch it instead is the
  current behaviour and was measured not to be enough: `check-decisions.py` is ref-blind on purpose
  (H-0072), so it refuses a *merged* duplicate at the trunk, in the one gate the supervisor may not
  skip, which is the exact failure #253 opened with. Creating the log is a write made by a command
  whose whole job is to read. `Chose:` the last option and `--repo` stops meaning anything, and #249's
  staleness work is all about what a flag that does not bind costs. What lost, as measured: a fresh
  adoption is now one `touch` plus a title line before the first id -- a step the replaced behaviour
  did not ask for, and one that cannot break anything that worked, since the replaced behaviour was
  unreachable; a `--repo` aimed at a log-less tree now refuses where it used to answer, so a caller
  that depended on the bleed fails loudly (the only callers in this repository are the skill,
  `docs/factory.md` and the legs, and all three name a log that exists); every call now costs one
  more `git cat-file`, which asks about an object and not a ref, so the H-0072 boundary holds and the
  verdict stays off every fetch path; and the guard cannot be planted with the `|` delimiter the other
  plants use, because its call site is `log_is_here ||` -- the leg says so and uses commas rather than
  an escaped `\|`, which in a BRE would match a different line.
- Checkpoint: da773e6f007932eaa01ad5e961d3b3e24b2417ef
- Effect: `factory/next-decision-id.sh` -- the path `case` has one arm (`--repo`, else `$PWD`, never a
  fallback), a `log_is_here` guard ahead of prefix inference, `|| true` on the inference `sed`, and
  the union's `[ -f ] && cat` became an `if` so a log present in `HEAD` but not in the tree cannot
  trip the same trap. `factory/selftest.sh` -- `alloc_run` and `alloc_control` can now omit
  `--prefix`, which is how the inference arm finally gets a leg; six legs and two controls
  (`alloc_refuse` asserts a refusal refused, printed no id, and named the path; `alloc_reads_its_repo`
  pins the cross-repo read as a difference between two trees and FAILs if they ever agree).
  `.opencode/skills/factory/SKILL.md` says a refusal means the path is wrong and that numbering by eye
  is not the fallback; `docs/factory.md` carries the three measured calls as a table. Re-measured
  after the write: `bash factory/selftest.sh` moved from `286 passed, 0 failed` on the checkpoint to
  `292 passed, 0 failed`, and the two runs' line lists were diffed to confirm six new counted legs,
  two new control lines and no leg lost -- the suite prints a control's `ok` without adding it to the
  tally, so #255's "four counted controls" overstates it by four. The three probe calls now refuse
  naming the path; the calls that worked are unchanged (`D-0052` and `H-0073` here, a fixture at
  `D-0001` answers `D-0002`, a fresh uncommitted log answers `D-0001`, a heading-less
  `factory/DECISIONS.md` answers `H-0001` with no `--prefix`).
- Correction (`H-0073`, 10 Nov, on #257's review): one clause of this `Effect:` is false, and so is the
  code comment that came with it. The union's `[ -f ] && cat` did not become an `if` because "a log
  present in `HEAD` but not in the tree" could "trip the same trap" -- it could not. Bash exempts a
  failing command inside an `&&` list from `set -e`: `bash -c 'set -euo pipefail; out=$( { [ -f /nope ]
  && cat /nope; echo AFTER; } | cat ); echo $out'` prints `AFTER`, and measured on `91d992d` with
  `--prefix D`, a fixture whose `DECISIONS.md` is in `HEAD` and deleted from the working tree answered
  `D-0008` with status 0. The `sed` half of the same sentence is real and reproduces -- the same
  fixture, no `--prefix`, status 2 with both streams empty, because `pipefail` gives the pipeline's
  status to the command-substitution assignment -- and that is what the branch actually fixed. The `if`
  stays because it says what it does, not because it closes a failure mode; the claim was written before
  it was measured, the same hazard the entry two numbers later records about its own `Effect:` line.
  Nothing in the suite planted the deleted-from-the-tree shape either, so nothing could have
  contradicted it: `factory/selftest.sh` now answers it
  (`a log in HEAD but not in the tree answers from HEAD`, `D-0008`). This entry is corrected in a dated
  line rather than rewritten, because it is cited as evidence and a silently corrected decision log is
  indistinguishable from one that was always right.


## H-0074 · Two sessions can still take the same id, so the merge arbitrates it and the yield rule is written down

- Question: #253's item 4 says the same-minute collision "is the part worth a decision rather than a
  script", and #255 left it open. Measured on this tree: two allocator calls in the same instant against
  one fixture both print `H-0010`. Is a reservation bought, or is the merge the arbiter -- and what has
  to be written down for that answer to be a procedure rather than a shrug? Item 3 belongs to the same
  question, because the rule a collision is resolved with did not exist either: `grep -c yield` over
  `docs/factory.md` at `91d992d` answers 0, and `git diff f311075 91d992d -- docs/factory.md` adds no
  paragraph about it, though #255's body reported "**4. The tie-break, written down.** ... is in
  `docs/factory.md` now".
- Options: accept the race, make the merge the arbiter, and write the yield rule and its reason into
  `docs/factory.md` | reserve ids on a fourth ref claimed with a compare-and-set `git update-ref` |
  have the supervisor allocate the id at dispatch and name it in the tick's brief | give each branch its
  own id range, the option H-0070 also rejected
- Chose: accept the race, make the merge the arbiter, and write the yield rule and its reason into
  `docs/factory.md`
- Why: what the race needs to open is not reachable on the deployment, and not by habit but by
  mechanism. `factory/factory.sh` takes `exec 9>"$FACTORY_RUN/lock"` and `flock -n 9` before it reads
  anything -- a second activation prints `another supervisor holds …, so nothing was done here.` and
  exits 7 -- and it dispatches each tick in the foreground (`timeout … opencode run`, no `&`) from one
  loop, so two sessions are never allocating at each other on one machine; a sibling that has already
  committed is visible regardless, because every unit tree is a worktree and worktrees share
  `refs/heads` (the leg in this very section: `D-0002` before the sibling commits, `D-0010` after, with
  no fetch). The window that survives is a second clone that has not fetched, or a person running a
  unit in a second checkout while the timer runs one here. **A reservation ref** loses on reachability
  and on kind: a local ref says nothing to the other clone, and the moment it is pushed it is a
  state-changing write in front of the tick budget -- `H-0013` priced a blocking *read* at 5min2s of a
  pass and `H-0056` refused one inside a verdict -- while a claim that outlives the session that made it
  is a lease needing an owner and an expiry, which nothing in this harness reaps. **Supervisor
  allocation at dispatch** moves the race earlier without closing it: the supervisor reads the same
  union this allocator reads and inherits the same staleness, so a session whose re-read finds the
  handed-out number taken has to move anyway; it also costs a tracker write per unit. **Per-branch
  ranges** change what every historical `H-NNNN` citation means, which is why `H-0070` refused it and
  `H-0061` and `H-0068` before it. Acceptance is only honest if the arbiter is a procedure, so the guide
  now carries the rule -- the side that has not merged yields -- with the reason it is that side and not
  either: `main`'s ids sit in history that cannot be rewritten here, because the only merge offered is a
  merge commit whose ancestors are permanent and the rewrite that would move one is a force-push
  `.opencode/opencode.jsonc` denies every agent in this repository. The four steps end with the gate over
  both logs, and they cite `H-0051` and `H-0052` as the shape of the record, because a gap in a log is
  citable only when an entry says why it is empty. One leg and one control pin both halves, rule and
  reason, each with its own plant: what failed here was a delivered claim with no leg behind it, and a
  phrase in a pull-request body is read by nobody resolving a conflict at the trunk at 2am.
- Checkpoint: 18d07e37589ea220f274340bd4139d227abb52f1
- Effect: `docs/factory.md` gains "When two branches take the same number, the side that has not merged
  yields" and "Why there is no reservation", with the measured `H-0010`/`H-0010` and the reachability
  paragraph. `factory/selftest.sh` gains `alloc_yield_rule` with a two-plant control, and a comment
  recording that this suite's controls print `ok` without incrementing the tally -- so #255's "four
  counted controls" overstates it by four, and every count in both of this branch's entries says "legs"
  and "control lines" separately. No shipped check changes: `factory/next-decision-id.sh` is already a
  read that says it is one, and `H-0072` keeps ref-reading out of `scripts/check-decisions.py`. The
  mitigation acceptance makes cheap -- `factory/factory.sh` fetching the trunk once per activation,
  bounded by a `timeout` and non-fatal -- is deliberately not bought here, because it is the supervisor's
  ref policy and needs its own decision; it is filed and read back as issue #256 (`256 OPEN The
  supervisor dispatches against whatever refs this tree last fetched, so a second clone allocates stale
  ids`). Re-measured after the write: `bash factory/selftest.sh` moved from `292 passed, 0 failed` to
  `293 passed, 0 failed` -- one leg counted, one control line printed and not counted -- and the
  control's two plants each make the leg refuse, which is what proves the leg reads both halves.


## H-0075 · The activation fetches origin once, ahead of the queue read, so the pass dispatches against refs it has actually refreshed

- Question: `H-0056` forbids a fetch inside a verdict, and #256 asks for a refresh before the
  allocator runs. Where does the refresh go?
- Options: inside the verdicts that read refs | one unrefspec'd fetch per activation, ahead of the
  queue read | one fetch per unit, in `factory_tick` | operator discipline only, as the skill says today
- Chose: one unrefspec'd fetch per activation, ahead of the queue read
- Why: Every judgement the pass makes is a reading of refs, and nothing in the harness had ever
  fetched. `factory/triage.sh:538` takes `main_sha` from `git rev-parse origin/main` and `:563` counts
  `git rev-list --count "$main_sha..$sha"` for every branch with no open pull request -- that is the
  `$ahead` number in the `branch_without_pr` detail line, so an unfetched clone reports an orphan as
  further behind or ahead than it is; `factory_worktree` (`factory/common.sh`) cuts a unit with
  `git worktree add -b "$branch" "$path" origin/main`; `next-decision-id.sh` takes its ceiling from
  every `refs/heads` and `refs/remotes` blob. All three therefore read the last time this
  clone looked. That is not hypothetical here: taking this branch, `git for-each-ref` in the primary
  clone listed `origin/issue-253` carrying `H-0073` and `H-0074` while `origin/main` stopped at
  `H-0072`, and the file in the checkout ends at `H-0072` -- so a clone that had not fetched would
  allocate `H-0073`, the fourth entry under three numbers, which is exactly the shape `H-0070` was
  bought to stop. The allocator's answer in this tree was right (`H-0075`) only because some earlier
  person had fetched; nothing guaranteed it, and `docs/factory.md` told the operator to type the fetch
  themselves on a second machine.
  It is not `H-0056` to do this here, and the distinction is the decision. #256's own reasoning is
  correct about a *verdict*: `H-0072` keeps `scripts/check-decisions.py` ref-blind, `H-0074` measured
  one commit reporting `1 decision-log problem` in a clone holding every branch and none in a
  single-branch clone of the same commit, and a verdict whose answer depends on which refs a tree
  happens to hold gets re-run until it agrees. A gate must not read refs; a *dispatch* already does,
  unavoidably, because it is the thing that chooses which branch to cut. The supervisor is also already
  a network path -- every triage, review-poke and merge in `factory.sh` is a `gh` call -- so one `git
  fetch` per activation adds a call to a pass that spends minutes on the network already, and it adds
  it in the one place where the refs are about to be used.
  The shape is a measurement, not a guess, because the ticket asked for the wrong one. A fixture with a
  local bare origin, a clone holding a unit worktree, and a sibling that pushed `D-0007` from a second
  clone: `git fetch --prune origin trunk` -- the trunk-limited form #256 names -- returns 0 and the
  unit tree still allocates `D-0002`; `git fetch --prune origin` returns 0 and it allocates `D-0008`.
  The sibling's entry is on the branch it pushed, and a refspec-limited fetch updates the trunk only.
  Worse, the trunk-limited fetch *reports* `1 moved` when counted on `->`, because git prints
  `* branch trunk -> FETCH_HEAD` for an explicit refspec: the useless form looks in the journal like
  the useful one. Hence the counter reads `-> origin/`, and hence `factory/selftest.sh` ships the
  trunk-only plant as a counted control that must leave the id stale.
  Which tree: one fetch in the primary checkout moved the allocator's answer in a linked unit worktree
  of the same clone (`D-0002` in both trees before, `D-0008` in both after), because `refs/remotes`
  lives in the common directory -- the same sharing `H-0074` cites for the worktree half of the
  boundary, read the other way. So the argument is `factory_main_checkout` and a tick gets a refresh it
  never asks for; a fetch in the unit tree alone would have been the tick's, not the pass's.
  The bound is the answer to #256's counter-argument rather than a dismissal of it: the pass runs under
  `flock -n factory/run/lock`, so a fetch that parks does freeze the queue. Measured against an origin
  that accepts the connection and never answers -- `GIT_SSH_COMMAND='bash -c "exec sleep 60"'`,
  `FACTORY_FETCH_TIMEOUT=4` -- the call returns at the bound, leaves no `.lock` file in `.git`, and the
  clone works afterwards; the same holds under `SIGKILL`, which git cannot clean up after. `--prune`
  with a single-branch refspec does not destroy the union the allocator reads (a planted
  `refs/remotes/origin/ghost` survived it). The default, 120s, is a fifth of the timer's
  `OnUnitInactiveSec=600`, so the worst case the bound permits is one activation that finds the lock
  held, prints `supervisor: another pass holds the lock, skipping`, exits 7, and is over. And the
  function returns 0 whatever the fetch did, because the alternative -- a halt because origin was
  unreadable -- is the bug #248, 115 `HALTED` lines and a frozen queue. What a failed fetch costs is
  one journal line naming how blind the pass is: `REFRESH-TIMEOUT`/`REFRESH-FAILED` with the trunk
  ref's own age, and `REFRESH-BLIND` for a refspec that can never see a sibling's branch.
  The fixture also caught a real ordering fault, which is the reason it is a fixture and not prose: the
  first version aimed `>"$FACTORY_LOG/refresh.log"` at a directory nothing had created yet, because
  `log_event` is what calls `factory_paths`. The redirect fails as status 1 with an empty file, so a
  pristine tree reported `REFRESH-FAILED (1)` over an empty log -- a missing directory wearing the
  sound of a refused origin. `factory_paths` now runs first.
  Tie-break: reversible by `git revert` alone (one function, one call site, one config default), and
  the option that leaves every verdict's answer exactly where `H-0072` and `H-0074` pinned it.
  What lost, as measured: one network call per activation on the critical path *inside* the lock, and
  its whole cost lands on a slow or dead network -- that is the option's real price and the reason it
  is bounded rather than trusted. It makes the dispatch refs-dependent, so two clones can still choose
  differently inside the window between activations: the allocator is still a read and not a
  reservation (`H-0070`), and `H-0071`'s re-read before the entry commit is still the last line of
  defence, unchanged. A machine with no reachable origin now writes one `REFRESH-FAILED` every ten
  minutes, which is honest and noisy. `REFRESH-BLIND` exists because a single-branch clone cannot be
  fetched into seeing a sibling -- that is the residual the fetch cannot close.
  Why the alternatives lose. **Inside the verdicts** is what `H-0072` refuses, and the two legs that
  say so (`the gate reads no branch refs`, `the verdict never fetches`) would have to be inverted for
  it, trading a gate with one answer for `H-0074`'s measured two-clone divergence; it also puts a
  blocking read in front of `install-timer.sh --install`, a function with no budget of its own. **Per
  unit** costs up to three fetches a pass inside the tick ceilings, and fixes the smaller half: the
  tick's allocator would be current while `triage.sh`'s rows and the branch the worktree was cut from
  -- both decided before any tick exists -- stay stale. **Operator discipline only** is what shipped
  until now, and its measurement is the first paragraph: a live collision on an open branch with a
  sentence in `docs/factory.md` as the only remedy.
- Checkpoint: 96c54a1dc7d73b2c6ed750f2705c865b09a81313
- Effect: read back from `git diff --stat origin/main...HEAD` on this branch rather than restated from
  memory, because restating it is what made the first version of this entry name a `triage.sh` command
  that does not exist as written -- corrected by the follow-up commit on this branch, which touches only
  this paragraph and the matching comment in `factory/factory.sh`.
  `factory/factory.sh` gains `factory_refresh_origin` and one call in `factory_body`
  before the `triage.sh` read. `factory/common.sh` gains `FACTORY_FETCH_TIMEOUT` (default 120).
  `factory/selftest.sh` gains four counted legs (`the pass refreshes before it dispatches`,
  `the refresh is bounded by a timeout`, `a failed fetch never stops the pass`,
  `one fetch per activation`) and four controls, two of them planted outside the tree;
  `bash factory/selftest.sh` moved from `286 passed, 0 failed` to `290 passed, 0 failed`, and
  `make check-shell` reports `0.10.0 over 28 scripts` with no finding. `docs/factory.md`'s
  decision-id section closes the second row of `H-0071`'s table instead of assigning it to an
  operator, and the configuration table gains the timeout row.
  `.opencode/skills/factory/SKILL.md`'s allocator bullet no longer says nothing fetches on a tick's
  behalf. Not touched: `factory_supervisor_position`, `factory/next-decision-id.sh`,
  `scripts/check-decisions.py`, the Makefile.


## H-0076 · The guard asked whether the path exists, so every wrong path that exists was still answered

- Question: #257's review measured the guard H-0073 shipped, at `c5aa1eb`, and found it answering a
  different question than the one this command exists to ask. `[ -e "$target" ] || git cat-file -e
  "HEAD:$rel"` is true of anything that is *there*, and every wrong path in a real checkout is there.
  Re-measured on this branch's own head after the review, with `--prefix H`: `docs/factory.md`,
  `README.md`, `AGENTS.md`, `factory` and `factory/` each printed `H-0001` and exited 0 -- the
  collision, with exit 0. At the head this entry ships, the same five exit 2 naming the path while
  `DECISIONS.md` answers `D-0052` and `factory/DECISIONS.md` answers `H-0076`, both unchanged. Two
  things make it blocking rather than tidy. `docs/factory.md` is not a typo a session has to invent: it
  is the 842-line guide carrying the section that tells a session to run this command, so it is what a
  session has open at the moment it reaches for the allocator. And the text #255 wrote makes the
  residual worse instead of neutral -- `SKILL.md`'s step 3 and this guide both reframe a refusal as
  *your path is wrong*, so a session handed `H-0001` for a file that exists reads it as "my log was
  read" and writes the duplicate heading: a silent collision where an error message would have been
  visible. No leg can see the shape, either -- every path named in the whole `decision_ids` section is
  `DECISIONS.md`, `factory/DECISIONS.md` or `factory/DECISION.md`, all three absent, so an existence
  predicate and a log predicate agree on every fixture the suite owns. What is the predicate, and where
  does it have to live for a control to be able to falsify it?
- Options: fold the name and type tests inside `log_is_here`, and give each arm its own leg and its own
  plant | put the name test beside the function, ahead of the call site | keep the existence predicate
  and widen it to `! -d`, so a directory is refused | leave the allocator alone and let
  `check-decisions.py` catch the duplicate heading at the gate
- Chose: fold the name and type tests inside `log_is_here`, and give each arm its own leg and its own
  plant
- Why: the tie-break's third rung is the one H-0073 itself stopped at -- "refuse rather than proceed
  when input is missing or empty" -- and it settles this the same way, because `docs/factory.md` is the
  same *input* as `factory/DECISION.md`: a path that is not the log. The second option was measured,
  not dismissed: with the `case` in front of the function, the shipped control's plant
  (`s,^log_is_here ||,: log_is_here ||,`, which neuters `log_is_here` and nothing else) leaves a script
  that still refuses, and `control: a mistyped log path refuses` fails with "the planted allocator
  refused". A check the plant cannot reach is a check no control can falsify, so beside-the-function is
  not merely clumsier -- it is unleggable, which is the thing this suite refuses to ship. The `! -d`
  option closes one shape and admits the one that matters: it still answers `docs/factory.md`, and it
  leaves the guard and the read disagreeing about what counts as the log, which is how a later shape --
  a FIFO, a socket -- becomes a `cat` that parks the caller. Both tests are needed, not one: the name
  alone admits a directory named `DECISIONS.md` (a tree in `HEAD`, and `cat-file -e` is true of a tree),
  the type alone admits `docs/factory.md`, so each gets a fixture where only it can refuse. The fourth
  option moves a read into the gate H-0072 keeps ref-blind on purpose, and still answers the wrong file
  first. What lost: one `git cat-file` per call becomes `-t` instead of `-e` -- still a question about
  an object, never about a ref, so H-0072's boundary holds and the allocator stays off every fetch path;
  and a caller aiming at a file that exists but is not the log is now stopped instead of answered, so
  anything that depended on that fails loudly (the only callers in this repository are the skill, this
  guide, and the legs, and all three name a log that exists); and the refusal now carries two possible
  reasons, which is one more sentence to keep true in `docs/factory.md`.
- Checkpoint: 82a4acd526796b2292f1692c3acc951a91c0faed
- Effect: `factory/next-decision-id.sh` -- `log_is_here` asks for a basename of `DECISIONS.md` and then
  for a regular file in the tree or `git cat-file -t "HEAD:$rel"` equal to `blob`, and it records which
  of the two refused in `$why`, because the shipped message asserted "not in the working tree, not in
  HEAD" even when the file was sitting there. The header block and `--help` now say the log itself is
  what gets answered, not any path. `factory/selftest.sh` -- three legs and two controls over two
  fixtures built so each arm decides alone: a checkout holding `docs/factory.md` and a `factory/`
  directory and no log at all (refused by name, `factory` as a directory with it), and a checkout where
  `DECISIONS.md` is a *directory* (the name matches; only the type can refuse it). Each control's plant
  was run against the other fixture to prove it narrow: the name plant still refuses the tree, and the
  type plant -- which writes back `git cat-file -e`, the predicate this entry replaces -- still refuses
  the mistyped name. `docs/factory.md` carries the five `c5aa1eb` calls and why the "your path is
  wrong" wording made that shape worse than the typo; `.opencode/skills/factory/SKILL.md` step 3 says
  the same thing in the text a session is reading when it runs the command. Re-measured after the
  write: `bash factory/selftest.sh` moved from `293 passed, 0 failed` on the checkpoint's tree to `296
  passed, 0 failed` here, and the two runs' `ok`/`FAIL` line lists were diffed -- five lines added,
  three counted legs and two control lines printed without touching the tally, no leg lost, the only
  other differences the fixture tmpdir names and one quarantine count this suite already treats as
  run-dependent. This tree's baseline reads one above the review's `292` because the environmental
  `install-timer` leg FAILs without `go`/`gofmt` first on its stub PATH and passes here; the delta is
  three legs on either box.

## H-0077 · A branch carrying half of #247 sat 50 commits behind a main that had taken its id

- Question: branch `issue-247` holds the stop-reason detector and the bundle mirror as an un-gated
  `wip` commit, cut at `f289fa1` on 8 Nov. Main has since merged #250/#252/#254/#255/#258, defines
  `FACTORY_AT_RISK` as the quarantine box, and allocated `H-0056` twice. How is the work delivered?
- Options: re-cut the branch from `origin/main` and re-author the 801 insertions | merge `origin/main`
  in, resolve the overlap by union, and renumber the entry from the allocator | escalate, because a
  taken id looks like the kind of thing a person should arbitrate
- Chose: merge `origin/main` in, resolve the overlap by union, and renumber the entry from the
  allocator
- Why: a force-push to an in-flight branch is denied here and rebase merge is disabled, so "re-cut"
  means re-authoring, and re-authored text loses the measurements already written into it -- the
  196-log sweep that found six clean stops and exactly one delivered tick that also ended on an
  undispatched call, the 28,601-byte prerequisite bundle against 17,647,835 bytes of full history, and
  the `<function_43 edit` shape nobody re-derives from memory. An id is a reference that outlives a
  checkout, which is why `factory/next-decision-id.sh` exists (#253): whether `H-0056` is taken is a
  mechanical read of every ref, not a judgement, so the third arm had nothing to arbitrate. Union, not
  pick, where both sides defined `FACTORY_AT_RISK` -- two rescues that have to land in one box. That
  is the arm with the teeth: the first attempt here rebuilt `common.sh` from main's copy plus #247's
  section, reported a clean `bash -n`, and had silently dropped `factory_at_risk_default`; the retry
  is main's file plus #247's authored hunks as a patch, checked by asserting every `+` line is present.
  The proof the merge was worth its noise is that main's own checks then caught the branch's new code:
  `fixture_carries_identity` (H-0059) named three fixture repos inheriting the host's identity, and the
  index-mode leg named a script git no longer held at 100755. Neither fires against re-authored text.
- Checkpoint: cb29532
- Effect: `factory/common.sh` gains `factory_stop_reason`, `factory_stop_marker` and
  `factory_preserve_unpushed` with `factory_at_risk_default` (one definition, both rescues);
  `factory/factory.sh` wires `TICK-UNDELIVERED` through the marker and mirrors in both stopping arms
  (2 call sites); `factory/selftest.sh` carries the legs and `internal/enforce/run_test.go` returns to
  main's version, which the branch never touched. Measured on this tree:
  `bash factory/selftest.sh` -> `337 passed, 0 failed`.

## H-0078 · Deriving the at-risk box from the trees would have followed them into `/tmp`

- Question: #247 derives the mirror box from `$FACTORY_TREES`' parent so a worktree prune can never
  reach it; #248 pins the same box at `$HOME/prj/at-risk` *because* `/tmp/opencode/wt-204` and
  `wt-206` are already gone that way. H-0077 left one definition; which invariant gives when
  `FACTORY_TREES` is overridden?
- Options: keep #248's literal and delete the derivation | keep #247's derivation alone, first
  assignment wins | derive beside the trees and refuse a result under `/tmp`, a relative path, or an
  empty answer, falling back to the home box
- Chose: derive beside the trees and refuse a result under `/tmp`, a relative path, or an empty
  answer, falling back to the home box
- Why: the two are not in conflict on any real configuration -- with the trees at
  `/home/rodney/prj/wt` the derivation answers `/home/rodney/prj/at-risk`, the very directory #248
  names -- so both legs pass together and only an override separates them. Tie-break 3 (refuse rather
  than proceed on bad input) and tie-break 4 (fewest files) both land on the third arm: one function,
  one `case`. What decides it is that the failure is silent. The box holds, by definition, the only
  copy of work nobody was watching, so a `FACTORY_TREES=/tmp/…` would move the rescue into the
  directory that has already deleted unmerged work here and nothing would print. Taking either literal
  instead makes one rescue's mirror land where the other's legs do not look. Both directions are
  pinned, because a fallback that always fired would pass the two refusals and fail the third:
  `/tmp/opencode/wt` and `../wt` fall back, `/srv/queue/trees` still derives `/srv/queue/at-risk`.
- Checkpoint: f043975
- Effect: `factory/common.sh` `factory_at_risk_default` carries the refusal, and `factory/selftest.sh`
  gained the leg `the mirror never lands where a reboot can`. Measured on this tree:
  `bash factory/selftest.sh` -> `338 passed, 0 failed` (one leg above the merge's 337, none lost).

## H-0079 · A mirror refusal keeps its reason, and a render that wrote nothing stops being a rescue

- Question: `factory_tree_quarantine` renders the mirror inside a capture, as
  `err=$(git stash show -p --binary --include-untracked "$sha" >"$tmp" 2>&1)`. That order points stderr
  at the FILE -- the copy the arm deletes three lines later -- so `err` is always empty and the refusal
  prints `… :: ` with nothing after the colon. H-0063 shipped it that way knowing: its entry and #250's
  body both disclose the empty reason as "known and disclosed, not fixed here". Is the reason worth
  recovering, and by which shape, without losing the payload or the emptiness guard?
- Options: capture stderr into its own file and read it back with `factory_git_said`, as
  `factory_tree_dirt` and `factory_tree_stop` already do | reorder the redirects so stderr reaches the
  capture and stdout reaches the file | read the reason out of `$tmp` itself before the arm deletes it |
  leave the code alone and pin the dangling `::` shape with a text scan over the shipped file | reorder
  the redirects and split the conjoined guard into the two causes it conflates
- Chose: reorder the redirects and split the conjoined guard into the two causes it conflates
- Why: the merged stream is not only losing a sentence, it is capable of forging the rescue. Measured
  10 Nov against the shipped function with a fixture repo and a `git` that answers `stash show -p` with
  a sentence on stderr and exit 0 -- a read that succeeds and complains, exactly #232's shape: stdout
  empty, so `$tmp` should be empty; stderr merged, so `$tmp` holds the warning; `[ ! -s "$tmp" ]`
  therefore PASSES, the function returns 0, the tree is cleaned, and the journal advertises
  `TREE-QUARANTINED … mirror=/…/repo-quarantine-….patch` for a file whose bytes are a complaint and not
  a patch. The new leg says it in its own words: `it wrote a mirror anyway, so a refusal is not the
  honest arm; a refusal changed the tree: 2 path(s) before, 0 after`. That is H-0063's hazard -- a
  mirror that restores nothing -- arriving through the merge instead of through a binary, and the size
  guard cannot see it because the bytes it counts are the wrong bytes. With the order reversed, stderr
  goes to the command substitution's pipe where the printf can quote it and stdout goes to the file
  where only the patch belongs; nothing else about the read changes, and a rendering that works still
  writes the same bytes. Why the four alternatives lose. **An errfile of its own** is the option the
  tie-break order names first (it is what the surrounding code already takes, twice), and it is the one
  that lost: it buys a second scratch file whose removal has to be remembered on every exit of the arm,
  and `quarantine-error.$$` leaked one file per pass for exactly that reason until H-0064 found it. The
  reorder adds no file and leaves the arm shaped like the `git stash failed` arm two screens above,
  which already captures with `err=$(git … 2>&1)`. **Reading the reason out of `$tmp`** works only when
  git wrote something, and it cannot tell git's sentence from the first line of a half-written patch --
  reason and payload share one file, which is the defect being fixed. **The text scan** pins the symptom
  (`:: ` at end of line) and not the property; the leg that asks the journal for git's own words goes
  red on the redirect order AND on a deleted printf, and the scan goes red on neither of the two.
  **One conjoined guard** keeps reporting a refusal that exited 0 with no bytes using a sentence about
  not being able to write, and the two causes need different repairs -- a git that refused, and a stash
  that renders nothing. What lost, as measured: the reordered form rests on a redirect rule (`2>&1`
  binds whatever stdout points at at that moment) that reads like a typo to the next person, so the
  comment says so out loud and `git refuses to render` is the leg that reddens the day it is tidied
  back; the empty-render sentence no longer prints the patch path, because the arm deletes that file and
  a path to nothing is not diagnosis (the box is named by the successful arm's line); and
  `factory_git_said` keeps only the FIRST line of the refusal, so when the arm unhooks, `factory_tree_unstash`'s
  sentence -- where the work actually is -- is still dropped from the journal. That last one predates
  this commit and is written here because the measurement walked into it.
- Checkpoint: a4304b6
- Effect: `factory/common.sh` -- the render arm is two arms: `rendering stash <sha9> failed :: <git's
  own line>` and `stash <sha9> rendered an empty mirror :: git exited 0 and wrote no bytes[; it said:
  …]`, with `why=""` added to the function's locals. `factory/selftest.sh` -- `expect_quarantine` gains
  a seventh argument, WHY, asserted in the HALT arm as substrings the `TREE-QUARANTINE-REFUSED` line has
  to carry (commas split, not spaces: the sentences carry spaces); `no mirror, so no clean` now names
  `cannot be created`, which it did not use to; new helper `q_arm` plants the dirt, installs one poisoned
  subcommand through `shim_install`, runs the expectation and puts `PATH` back; four legs run in both
  trees -- `git refuses to stash`, `git refuses to render`, `the render comes back empty`, `an inert
  shim still quarantines`. `git refuses to stash` was added on this branch, not by the tick that wrote
  the entry above: with the entry written, the push arm was re-probed at `450a628` and `344 passed,
  0 failed` came back with its refusal made inert (`… 2>&1) && false`), so #248's own second acceptance
  leg -- "make the stash fail… and assert BOTH halves" -- was still unpinned by the six render legs
  alone. The third arm needed no code change: unlike stderr, a failing `git stash push` is captured by
  `err=$(… 2>&1)` already, so its sentence was intact and only the leg was missing.
  Provenance, because the branch moved and two ids changed. The fix was written 22 commits behind
  `main`, in a tree whose allocator predated H-0074/H-0075, and opened as `## H-0073` -- an id
  `origin/main` already publishes at line 1700 (the allocator entry), whose log now runs to H-0078.
  The unmerged side yields: `factory/next-decision-id.sh` answers H-0079 here, so this entry is H-0079
  and nothing on either side moves; the same collision #253 and H-0074 exist to end, and `check-decisions.py`
  would have refused the duplicate rather than the rebase catching it. Rebased onto `450a628` (local-only
  branch, no pull request, so no in-flight history was rewritten); `Checkpoint: a4304b6` is that
  checkpoint as it now stands, the rebased form of `36da883`, and the subject it carries still reads
  `pre-H-0073` because retitling a checkpoint is rewriting what the earlier tick believed. Both
  originals, `36da883` and `c997ff6`, are held in
  `~/prj/at-risk/branch-issue-248-c997ff6-20261110T141909Z.bundle` -- the 08:38 bundle named in the
  wip commit's own message covers only `36da883`, so the work commit itself had no copy off-box until
  this one was made.
  Measured on the delivered base (`450a628` + this commit), each with `bash factory/selftest.sh`, and
  each plant undone by `cp` from a `/tmp` backup rather than `git restore`: fixed tree -> `346 passed,
  0 failed`; the shipped render arm put back (`>"$tmp" 2>&1`, conjoined guard) -> `342 passed, 4 failed`,
  two legs for the refusal line that names no cause and two for the forged mirror, one of them printing
  `it wrote a mirror anyway, so a refusal is not the honest arm; a refusal changed the tree: 2 path(s)
  before, 0 after`; the `git stash failed` arm inert with this leg present -> `344 passed, 2 failed`,
  both naming `git stash failed`; the same arm inert WITHOUT this leg -> `344 passed, 0 failed`, which is
  the number that leg exists to make impossible. The redirect rule itself was checked apart from the
  function on a stand-in that writes a payload to stdout and a complaint to stderr and exits 0: shipped
  order captures nothing and leaves the file holding `payload` plus `fatal: cannot apply binary patch`
  -- the complaint inside the mirror, contaminating a patch that would otherwise have applied -- while
  the fixed order captures the complaint and leaves the file holding only the payload.

## H-0080 · The quarantined section is the work, and the ticket's blast radius is one line off

- Question: the tick before this one spent its whole life on #263 and was stopped at its ceiling with
  646 lines still uncommitted; `factory_tree_quarantine` mirrored them to
  `~/prj/at-risk/issue-263-quarantine-20261110T151215Z-2008e6100.patch` and `git stash`ed them at
  15:12:15Z, which is the state #247 and #250 built the mirror for. Re-apply the mirror and finish it,
  re-author the section from its own header, or escalate a piece of another unit's half-work?
- Options: `git apply` the mirror onto the blob it was cut from, add the control it lacks, re-measure
  every number in it | re-author from the header and the issue | escalate, because the author is a
  different session
- Chose: `git apply` the mirror onto the blob it was cut from, add the control it lacks, re-measure
  every number in it
- Why: an id is not authorship and a stash is not a lost unit -- the mirror exists so the next tick
  finishes the work instead of paying for it twice, and `git apply --check` against `89687df` answered
  clean, which is the proof the two are the same text. Re-authoring is what H-0077 measured as the
  expensive arm: it keeps the sentences and loses the measurements written into them. What the mirror
  did not have was the third control, and it needed one: its own two mutants both ended with the clean
  unit still ticked, so its central leg -- "then starts a tick for unit B" -- could only ever say yes,
  and a leg that cannot say the other word is the thing this file exists to refuse. Measured here,
  same fixture, unit A's session made to fail, the one variable the dispatcher's guard: with
  `|| rc_tick=$?` the pass reaches B; with it taken the journal stops at A and the process exits 4. So
  the property #248 was filed for is held by two lines, not the one #263 names. That also corrects the
  ticket, and the correction is printed by the legs rather than asserted: the two mutants it prescribes
  do NOT stop the units behind the quarantined one -- the halt marker freezes the NEXT activation (exit
  8, nothing started), and the false failure costs one unit its own turn (exit 4, B still started).
  Only the halt file ends a queue, and only for later activations. Re-measured numbers went in for the
  ones the mirror carried: 337 passed / 1 failed at `450a628` in 10.727 s / 10.438 s, 349 passed / 0
  failed with the section in 12.565 s / 12.476 s, and the `grep -c` pair in the header's own claim,
  which as written counted its own comment and answered 2 where it said 1.
- Checkpoint: 6367582
- Effect: `factory/selftest.sh` gains the supervisor-process section (ten legs, seven activations of
  the shipped `factory.sh` over a two-unit fixture, three mutants) and the check-decisions gate now
  reads this file. Nothing in `factory/factory.sh` or `factory/common.sh` moved: this unit adds tests
  and reports, and the hazard it found in the ticket's own account belongs in the PR body, not in a
  behaviour change nobody asked for.

## H-0081 · The at-risk box check was asking the box, not the run

- Question: `quarantine_stays_in_the_fixture` fails whenever `~/prj/at-risk` holds a quarantine patch,
  and that directory's entire purpose is to hold the only copies of work the supervisor rescued. It
  went red at 15:12:15Z when the supervisor mirrored this unit's own half-finished file there, and
  `make verify` on the next tick in that tree died at `Makefile:103` with "1 quarantine patch(es) in
  ~/prj/at-risk". Make it pass by moving the rescue aside, leave it red and report a gate nobody can
  turn green, or change the question it asks?
- Options: move or delete the patch, which is what a person under a red gate will do | leave it red
  and report it as #263's cost | ask this run, not the box: count patches newer than a marker taken at
  suite start, and prove the predicate can separate the two cases
- Chose: ask this run, not the box: count patches newer than a marker taken at suite start, and prove
  the predicate can separate the two cases
- Why: the assertion anyone actually wants is "this suite never mirrored fixture dirt into the
  operator's box", and that is a fact about the run -- the box's absolute contents say nothing, because
  a healthy harness makes the box non-empty forever. The other two arms are the failure: the first
  destroys the only copy of real work to make a test green, which is the exact hazard the box exists to
  prevent, and the second leaves a red gate in front of every later unit in this tree. `-newer`
  introduces its own vacuity, so it is not shipped unverified: `at_risk_predicate_provable` plants one
  patch dated 2000 and one dated 2020 either side of a 2010 marker and requires the same expression to
  answer 1 and 1 out of 2 -- a mistyped marker path answers 0, and 0 is the green answer. Measured: the
  leg prints `4 patch(es) in the fixture, none written by this run in /home/rodney/prj/at-risk
  (1 rescue(s) already there from before this run, left alone)`, and the runner with no such box still
  takes the `[ -d ]` arm H-0059's runner cost added.
- Checkpoint: 6367582
- Effect: `factory/selftest.sh` takes `at_risk_mark` beside `$work`, the box leg counts
  `-newer "$at_risk_mark"` and reports what it left alone, and `at_risk_predicate_provable` is the
  control. Both entries ride one commit because the leg text and the marker are one hunk: an isolated
  box-check commit would contain a `-newer` against a marker defined only in the other hunk, which
  answers 0 and passes for the wrong reason -- a tree that cannot be verified as it stands is not a
  checkpoint.

## H-0082 · A sha cited six times is not an object in this repository

- Question: `H-0080`, four comments in `factory/selftest.sh`, the pull-request body and `e334f4f`'s own
  commit message all date their measurements to `a7a656d`. `git rev-parse a7a656d` answers
  `unknown revision or path not in the working tree` and
  `git log --all --format='%h' | grep -c '^a7a656'` answers `0`. Replace the string everywhere and move
  on, or record the correction where a reader of the log will find it?
- Options: silently `sed` the six places, which leaves no trace that a citation was ever wrong | delete
  the affected entries as untrustworthy | correct the string, add this entry, and say in the pull
  request that the pushed commit message cannot be corrected
- Chose: correct the string, add this entry, and say in the pull request that the pushed commit message
  cannot be corrected
- Why: a decision log that has been quietly edited is indistinguishable from one that was always right,
  and the whole reason `H-0080` cites a base is so a reader can re-run the 337/1 number against it --
  a sha that resolves to nothing makes that claim unfalsifiable, which is the failure this repository
  names over and over. The numbers themselves are not in doubt: every one was measured on the commit
  now named, `450a628`, the merge of #259 and the parent of the checkpoint `6367582`; only the label was
  invented. Deleting the entries would throw away the third control and the blast-radius measurement
  with the typo. The fourth place is unreachable: a pushed commit message is not amendable here, because
  force-push is denied to a tick and both squash and rebase merge are disabled, so the correction has to
  live beside it rather than inside it, and `#1180` is the reason it is written as a dated entry instead
  of an edit.
- Checkpoint: e334f4f
- Effect: `factory/DECISIONS.md` `H-0080` and four comments in `factory/selftest.sh` now read
  `450a628`; this entry is the trace. `e334f4f`'s message still says `a7a656d`, permanently, and
  PR #264's body says so. `bash factory/selftest.sh` re-run on this commit: same 349 legs.
  Corrected 10 Nov by `H-0086`, which leaves these words as written rather than editing them: the
  "four comments in `factory/selftest.sh`" above are THREE -- `git show e334f4f:factory/selftest.sh`
  piped to `grep -n a7a656d` answers lines 78, 2275 and 2313. Four is the number of tree locations
  `d7d370d` rewrote -- 3 comments in `factory/selftest.sh` plus `H-0080` in this file -- read back
  as a count of comments in one file.

## H-0083 · Which checkout the timer execs is read from the unit, not composed from the tree standing in front of the script

- Question: `install-timer.sh` printed `The unit execs <FACTORY_ROOT>/factory/factory.sh` inside the
  very refusal that stops an install, and neither the pass nor `--status` said anything about the unit
  at all. #260 measured the two disagreeing on this box on 10 Nov: the installed unit execed
  `~/prj/wt/factory-harness` while `--install` from `~/prj/parental-lockout` printed that it execed
  `~/prj/parental-lockout`, so `H-0055`'s guard reported the position of a tree the loop never ran and
  sent the operator to fast-forward a checkout nothing executes. Where does the answer to "which
  checkout does the loop run today" come from, and what prints when it cannot be read?
- Options: keep composing it from `$FACTORY_ROOT` and add a caveat line | ask the manager with
  `systemctl --user cat` at every reading | read the unit file and give the four answers four
  sentences | escalate
- Chose: read the unit file and give the four answers four sentences
- Why: the file is what systemd reads at activation, so it answers on a machine with no running
  manager *and* inside `factory/selftest.sh`, which cannot call `systemctl` -- that file has faked it
  with a `pos-shim` since before this change for exactly that reason, and `H-0054` is the standing
  rule: a section needing a binary the gate does not install reads green on every box in the loop and
  red on CI. The cost is named rather than hidden: the harness now owns a parser for one key. It takes
  `ExecStart=` and systemd's execution prefixes (`-`, `@`, `+`, `!`, `:`) and does not interpret
  quoting -- a unit whose command is quoted is not this generator's, so it lands in the foreign arm
  carrying its raw text instead of being guessed at. Four sentences because of `#228`: "I could not
  read it" and "nothing is armed" are different claims about the machine and the second is reassuring,
  so collapsing them turns an unknown into a clean report. No fetch anywhere (`H-0056`, `H-0013`): the
  installed tree's position is asked of `factory_supervisor_position`, which reads `.git` and nothing
  else. The fourth option is refused by its own test -- nothing here changes what a tick is permitted
  to do; it changes a sentence the installer prints and the guard reads.
- Checkpoint: 20150cd
- Effect: `factory/common.sh` gains `factory_units_dir` (the `XDG_CONFIG_HOME` formula `install-timer.sh`
  kept a second copy of, which is `H-0077`'s two-definitions class), `factory_service_unit`,
  `factory_installed_tree` and `factory_unit_lines`; `factory/factory.sh` logs one `UNIT-*` line per
  activation and answers on `--status`'s `unit:` line; `factory/install-timer.sh` takes its refusal's
  two sentences from the unit, prints three further arms (nothing armed, a unit that does not exec this
  harness, a file that cannot be read), journals `UNIT-TREE old=… new=…` and says so in the block a
  person reads when an install moves the exec path, and renders `Environment=FACTORY_SERVICE_UNIT=` so
  a unit names itself; `factory/selftest.sh` gains ten legs and the `status_arm` scanner;
  `docs/factory.md` re-prints the refusal and the move notice from the shipped script.
  What lost, stated as measured: the reading answers "which checkout the timer WILL exec", not "which
  one is running now" -- a pass started before a reinstall still reports the old unit, because the new
  one has not been activated yet and nothing in a unit file says when systemd last read it. Discrimination,
  each mutant applied to the working tree from backups and restored before the next, every tally from
  `bash factory/selftest.sh` on this commit, and each `failed` count below including the `at-risk` leg
  that `H-0084` fixes: composing the answer from `$FACTORY_ROOT` (the shipped bug) -> `347 passed,
  9 failed`, eight legs naming the unit; an unreadable unit answered like an uninstalled one -> `353
  passed, 3 failed`; the render drops the recorded name -> `354 passed, 2 failed`; the pass stops
  calling the renderer -> `354 passed, 2 failed`; the false sentence restored in the refusal -> `353
  passed, 3 failed`, both installer legs red; every `unit:` printf in `--status` deleted -> `354
  passed, 2 failed`. Two probes found legs that were not legs. (1) Deleting ONLY the matched arm's
  `unit:` printf left `355 passed, 1 failed` -- "does the arm contain `unit:`" is satisfied by any
  three of four printfs, so the line a person reads on a healthy box was unpinned; the predicate is now
  one per arm, each named by the phrase only it prints, with a control that deletes one arm's sentence
  at a time and requires the scan to go blind for that arm alone (that control re-measured: deleting
  the matched arm is now `354 passed, 2 failed`). (2) The `status_arm` scanner shipped as a one-liner
  that stopped at the first `;;` it saw, which is inside the arm's own nested `case`, so the leg read
  eight lines of a forty-line arm: the same tree read `both consumers ask the unit FAIL` at 15:55:34Z
  and `ok` at 15:57Z with only the scanner widened. That is also why `unit_call_sites` reads a function
  body rather than trusting `grep -c 'unit:'`. The same probe exposed a fixture defect worth its own
  sentence, because it produced false accusations rather than a verdict: two legs plant an empty
  directory at the unit path to stand for an unreadable file and remove it only when their assertion
  passes, so one early return left the directory there and every later `>"$pos_unit_file"` died with
  `Is a directory` -- six legs red, four reporting `installing a current tree over a moved unit failed
  (rc=1)` about an installer that had done nothing. `pos_unit_exec` now clears the path first (file,
  symlink, or empty directory; a NON-empty directory is refused rather than deleted), and the collapsed
  arm costs the two legs that own it. Wiring that clear into `pos_install` instead was tried and is
  wrong: the install legs' subject is the state the previous leg left, and clearing it produced `344
  passed, 12 failed` with `install refuses a stale tree` no longer holding a stale tree.
## H-0084 · The mirror-box leg asserted the real box was empty, so a live supervisor made the gate unpassable and was blamed for it

- Question: `quarantine_stays_in_the_fixture` passes when `$HOME/prj/at-risk` holds no
  `*-quarantine-*.patch`. On 10 Nov `make verify` on this box read `1 quarantine patch(es) in
  /home/rodney/prj/at-risk -- this file has written to the directory that holds the only copies of
  real work`, about `issue-263-quarantine-20261110T151215Z-2008e6100.patch`. Its name stamps
  `20261110T151215Z`, `ls --time-style=full-iso` gives its mtime as `2026-11-10 10:12:15.85 -0500`, and
  the legs this file prints journal timestamps for ran at `15:56:41Z`-`15:56:48Z` in that very log --
  44 minutes after the patch was written, by a supervisor quarantining unit issue-263.
  `find /home/rodney/prj/at-risk -newermt '2026-11-10 15:40' -printf '%p\n'` returns nothing, so
  nothing in the box is newer than that write and this suite put nothing there. Should the gate assert
  the box is empty, or that this run did not change it?
- Options: leave it and have operators clear the box before `make verify` | narrow it to files whose
  mtime falls inside the run | snapshot the box at suite start and assert the set is unchanged |
  delete the leg
- Chose: snapshot the box at suite start and assert the set is unchanged
- Why: each rejected option breaks the thing the leg is for. Clearing the box is deleting mirrors to
  pass a test, and those patches are the only copies of work that exists nowhere else -- the object
  `H-0078` derives the box to protect and `H-0076` refuses the trees for the same reason. An mtime
  window is not identity: `factory_tree_quarantine` stamps the *sha* it mirrored into the filename, so
  a tick that re-mirrors an old dirty stop produces a name inside the window whose bytes are not this
  run's, and the window reads that as innocent; `H-0043`'s bargain is to measure and assert what the
  measurement answered, never to infer from a timestamp. Deleting the leg removes the only proof that
  the fixture's `$FACTORY_AT_RISK` override is honoured, which is what makes the `ok`s of the `q_arm`
  legs above it mean anything -- and this file's own history is a list of legs that passed while
  measuring nothing (`H-0054`'s `gh`-absent section, `H-0059`'s identity-starved fixture, `H-0079`'s
  `344 passed, 0 failed` with the push arm inert). `unchanged` is the stronger claim in both
  directions, which is the part to keep: a box this suite *emptied* is as much a bug as one it
  littered, and `empty` passed that silently while `unchanged` goes red on either. Naming each file
  with its own mtime is what lets the next reader separate "the supervisor wrote this during the gate"
  from "this suite wrote this" -- the ambiguity that produced the wrong accusation here -- and the
  `grep -h TREE-QUARANTINE ~/prj/*/factory/log/supervisor.log | tail` it prints is the command that
  names the unit.
- Checkpoint: f4f774f
- Effect: `factory/selftest.sh` — `quarantine_stays_in_the_fixture` now compares the box against a
  snapshot taken at line ~99, immediately after `trap cleanup`, and says so in its own ok line when the
  box already holds patches; the comparison is its own function, `at_risk_diff`, because the control
  needs something to hand a doctored input and the real box must never be doctored to test a guard
  whose job is that nothing moves work into it; the failure names each file with its own mtime in both
  directions and prints `grep -h TREE-QUARANTINE ~/prj/*/factory/log/supervisor.log | tail` as the
  command that identifies the unit that wrote it. `docs/factory.md` gains no line: the doc never
  claimed the box was empty, only that the legs mirror into the fixture.
  What lost, stated as measured: the leg no longer refuses a box that was already dirty when the suite
  started, so a run that begins with real patches present cannot tell an old patch apart from a
  same-name patch this suite wrote — mtime and name are printed for a human to make that call, which
  is strictly more than the old leg's accusation and less than a proof. Discrimination, mutants applied
  to the working tree from a backup and restored, every tally from `bash factory/selftest.sh`: the
  shipped assertion on this box -> `355 passed, 1 failed`, the failure the false accusation recorded in
  the Question above; the fixed leg -> `356 passed, 0 failed`; `at_risk_diff` blinded with an early
  `return 0` -> `356 passed, 2 failed`, both under `control: the diff sees a change`, one per
  direction; `printf --` reduced to `printf` in the vanished-file arm -> `printf: - : invalid option`
  on stderr and `356 passed, 1 failed`, the control catching a bash builtin rule that had already eaten
  the line on the leg's first draft. No leg writes into the real box on purpose, so the `+` direction
  is proven against synthetic lists in the control rather than by moving real work, and that limit is
  stated here rather than left implied: what is verified live is that this suite's own writes land in
  `$FACTORY_AT_RISK`, and that the box is the same set of paths at the end of a run as at its start.

## H-0085 · The log conflict was the cheap half: main had rewritten the function two of the three mutants are anchored on

- Question: PR #264 reads `CONFLICTING`/`DIRTY` against `adbe6e3` -- six commits of drift since
  `450a628`, and both sides appended to `factory/DECISIONS.md` at the same offset: main took
  `H-0079`, this branch took `H-0080`-`H-0082`. Inside that drift is #262's `08550d6`, which
  rewrites the mirror render inside `factory_tree_quarantine` and adds `why=""` to that function's
  locals -- the same function two of this section's three controls cut into, at the lines
  `qerr_file="$FACTORY_RUN/quarantine-error.$$"` and `log_event "TREE-QUARANTINED $unit`. Take
  git's conflict layout and let CI notice, keep one side's log text, renumber this side below
  `H-0079`, rebase onto main, or keep both sides' entries, ordered by id, and re-run the section
  against the merged tree?
- Options: take git's conflict layout and let CI notice | keep only the longer side's entries |
  renumber this side's three ids below `H-0079` | rebase onto `origin/main` | keep both sides'
  entries, ordered by id, and re-run the section against the merged tree
- Chose: keep both sides' entries, ordered by id, and re-run the section against the merged tree
- Why: no id moves on either side. `H-0080`-`H-0082` are cited by the pushed commits `e334f4f` and
  `d7d370d`, by this pull request's body, and by each other, so renumbering would turn every one of
  those citations into a pointer at a different entry -- the exact defect `H-0082` exists to record,
  one entry after it was written. `H-0079` has already paid the other version of this bill: its own
  entry records that it opened as `H-0073`, found the id taken on `origin/main`, and yielded the id.
  A position, unlike an id, is free to move, and it has to: `scripts/check-decisions.py` reads the
  order. Taking git's layout verbatim -- the first resolution made here, left in place and measured
  rather than guessed at -- answers `factory/DECISIONS.md:2159: H-0079 is out of order (D-0082 came
  before it); the log is a history and a history that jumps around cannot be bisected`. Ordering by
  id is what the allocator's contract already asserts: both sides numbered against the union of
  `refs/heads` and `refs/remotes`, so the numbers are the history's order and only the text
  disagreed. (That message names `D-0082`, which lives in the other log: `check-decisions.py:136`
  formats the predecessor as `"D-%04d" % prev_num` whichever log it is reading. Location and verdict
  are right, so no gate is bypassed and nothing here depends on it -- but a refusal that sends the
  reader to the wrong file is a real papercut, and it is filed rather than fixed under a review that
  asked for two named things.) What makes this more than a text merge is the anchor problem this
  repository has already been bitten by (#1048): `proc_mutate` and `proc_mutate_factory` each require
  their anchor to occur exactly once, so a retargeted line is a control that reports itself unplanted
  -- and a control that reports itself unplanted is only caught because the leg asks. So the merge
  was run, not read. On the merged tree: both `common.sh` anchors and the `factory.sh` anchor occur
  once each (`grep -c` = 1, 1, 1), `python3 scripts/check-decisions.py` -> `118 entries across 2
  log(s)` over the two sides' entries and `119` with this one added, and `bash factory/selftest.sh`
  -> `357 passed, 0 failed` in 13.5 s. Totals are
  box-dependent and this one is not the review's: this box's base is 349 and main contributes 8
  legs; the review's box reads 345 + 11 = 356. The same fs-dependent split #238 and #257 recorded --
  neither number is the other's, and both read 0 failed. That both sides survived intact was checked
  in both directions rather than assumed: `git diff HEAD -- factory/selftest.sh` deletes only the
  four lines this branch had already replaced with main's 7-argument `expect_quarantine`, and
  `git diff origin/main -- factory/selftest.sh` deletes only the four lines this branch had already
  replaced with `H-0081`'s marker -- each side loses its own superseded text and nothing of the
  other's. Rebase loses three ways: `allow_rebase_merge=false` on this repository, force-push is
  denied to a tick, and a `CHANGES_REQUESTED` review was submitted against `d7d370d` at 16:09:54Z,
  so rewriting the head would delete the artifact the review is about.
- Checkpoint: 2b3bfe6
- Effect: as this commit left it, `factory/DECISIONS.md` reads `H-0078`, `H-0079` (main),
  `H-0080`-`H-0082` (this branch), `H-0085` (this entry); no id changed on either side. Each number
  above was read back before this commit was made, per `#1180`: the gate printed 118 entries over
  the two sides' entries and 119 with this one (it says 120 on any head at or after `H-0086`), the
  suite printed 357/0, and the three anchors printed 1/1/1. What this commit does NOT do: `#264`'s
  body still asserts "Nothing here conflicts", which was true of main at `79b4757` and is false of
  `adbe6e3`. That sentence is a status claim, so it is corrected in the body itself, not by editing
  an entry.

## H-0086 · "Four comments" was four locations wearing one file's name

- Question: the review of `#264` measures three comments in `factory/selftest.sh` where `H-0082`'s
  `Question:` and `Effect:` both say four, and the pull-request body says four too. Re-measured on
  this tree: `git show d7d370d -- factory/selftest.sh | grep '^-.*a7a656d'` gives 3 lines,
  `-- factory/DECISIONS.md` gives 1. Rewrite `H-0082`'s two lines to the smaller number, delete the
  entry as untrustworthy, annotate `H-0082` with a dated correction and add this entry, or answer on
  the review thread and change nothing on disk?
- Options: rewrite `H-0082`'s two lines to read "three" | delete `H-0082` as untrustworthy |
  annotate `H-0082` with a dated correction and add this entry | answer the review thread and change
  nothing on disk
- Chose: annotate `H-0082` with a dated correction and add this entry
- Why: `#1180` is the rule and it was written twice in one day on this repository, both times about
  an `Effect:` field that described writes nobody had read back -- a decision record gets a dated
  correction, because a silently corrected log is indistinguishable from one that was always right,
  while a status claim is edited in place. The pull-request body is a status claim and gets edited;
  `H-0082` is a decision record and gets annotated, additively, with its own words left standing so a
  later reader can still see what was claimed. That is also what the review asked for -- "one line in
  the entry, one in the body" -- and the annotation shape delivers both where an in-place rewrite
  delivers only the second. Tie-break 6 settled it before reasoning did: `H-0082` itself is the
  precedent, having added a dated entry rather than editing `e334f4f`'s pushed message. Deletion
  loses most: the entry's archaeology is what makes `a7a656d` unrecoverable-by-sedimentation, and
  `#1180` names the value of keeping the wrong claim visible. Answering only on the thread is the
  failure `#1180` measured -- the record goes on saying four while a comment says three, and the
  next reader reads the record.
  What the number actually was is the part worth keeping, because it is not an off-by-one: `d7d370d`
  really did fix four things, and "the four places in the tree" is still in the body and still
  correct. One number was measured over the set (four tree locations, two files) and then reused as a
  count over a subset (comments in one file) inside the same paragraph. The heading's claim -- a sha
  "cited six times" -- survives on the count of citations, one per artifact: 3 comments plus `H-0080`
  plus the body plus `e334f4f`'s message (`git log -1 --format=%B e334f4f | grep -c a7a656d` -> 2
  mentions inside that one message). So the entry's thesis is intact and only one denominator inside
  it was wrong, which is the case for annotating rather than deleting.
- Checkpoint: 7a9fb0e
- Effect: `H-0082`'s `Effect:` now carries a dated annotation naming lines 78, 2275 and 2313; this
  entry is the trace, and nothing else on disk moved. `python3 scripts/check-decisions.py` -> 120
  entries across 2 logs, gate OK, read back before this commit rather than asserted by it. The
  body's two sentences
  are separated in `#264`'s rewrite -- three comments, four locations -- and that rewrite lands in a
  later push, so its read-back lives in the pull request and not here; if the rewrite does not
  arrive, this paragraph is the claim that failed, not the entry above.

## H-0087 · A count that cannot be taken ends the suite; an insulated one returns the green answer

- Question: the review of `#264` reports that the two `find` assignments `H-0081` added carry the
  failure mode this file documents two lines above them -- under `set -euo pipefail` a `find` that
  cannot open its directory makes the assignment fail and the script die (marker missing: exit 1,
  the next line never runs, the trap eats the fixture, no tally printed) -- and asks for `|| true`,
  noting it cannot fire today because the marker is created near the top of the file. Put it on the
  two assignments the review names, on every count in both box legs, on every count plus a leg that
  proves it, or leave the code and record the exposure?
- Options: `|| true` on the two assignments the review names | on every count in both box legs, with
  no new leg | on every count in both box legs and a leg that runs both arms, insulated and bare |
  leave the code alone and write the exposure into this entry
- Chose: on every count in both box legs and a leg that runs both arms, insulated and bare
- Why: measured before anything was edited, on a standalone probe rather than in this tree. The
  shipped shape -- the assignment under `set -euo pipefail`, box absent and marker absent -- printed
  nothing after the assignment and exited 1; the same line with `|| true` printed `real=0` and
  continued. So the reviewer's failure mode reproduces on this box, and so does what the insulation
  costs: the insulated answer is `0`, and `0` is exactly what makes the box leg print ok. Insulation
  trades a loud death for a silent green, and the only honest price for that is making the green
  answer falsifiable. Two things do. `at_risk_predicate_provable` already requires 2 / 1 / 1, so a
  masked 0 *there* goes red -- which is what makes a masked 0 in the box leg affordable rather than
  merely convenient, and the asymmetry (an ignored failure is safe where the expected answer is
  non-zero, dangerous where it is zero) is now written at the plant. The new leg
  `a count survives a box it cannot open` runs the same assignment against the same two absent paths
  both ways: insulated must return 0 and stay alive, bare must die. Without the bare arm the ok
  proves nothing, because a `find` that simply succeeded prints the same 0. Tie-break 3 -- refuse
  rather than proceed on bad input -- is the reason this is not option one: a bare `|| true` on the
  two named lines is tie-break 4's cheapest answer and it leaves the suite quiet about a box it
  cannot read, which is the shape this repository calls a failure left unspoken. The leg is what
  converts the insulation from silencing into testing, so widening the scope costs nothing the
  cheapest option would not have cost anyway -- and the wider scope is the point: the same shape is
  at `stray=` and in the predicate's three counts, and a half-insulated function dies on whichever
  line the reviewer did not name. What lost: the counts can now report 0 where the truth is unknown,
  which is strictly worse than before for anyone who reads a 0 as a measurement instead of an
  answer; the leg defends that only on this file's own terms. The `-newer` comparison has a second
  half no `|| true` reaches, priced in the comment rather than fixed: a rescue carried in with its
  original mtime (`cp -p`, `tar -x` without `-m`, a restore out of the box itself) counts as
  "already there, left alone" and can never redden the leg. It stays a disclosure because the fix
  for it is counting everything in the box, which is the unpassable gate `H-0081` exists to end.
- Checkpoint: 8b8fb91
- Effect: `factory/selftest.sh` -- `|| true` on all five counts (the box leg's `stray`, `real`,
  `kept`; the predicate's `wrote`, `kept`, `total`), the priced `-newer` disclosure, the new leg
  `a count survives a box it cannot open`, and the stale citation: what read
  ``set -euo pipefail` (line 59)' now says "at the top of this file, not at a line number somebody
  has to re-derive" (the review's non-blocking 5 -- the line was 63 at `450a628` and is 63 here, so
  the drift predates this branch and a number is the wrong thing to write). Measured on this tree
  after the change, both read back before this commit: `bash factory/selftest.sh` -> `358 passed,
  0 failed` (one leg above the merge's 357, none lost), `make check-shell` -> `check-shell: 0.10.0
  over 28 scripts`, exit 0. The checkpoint here was taken after the edits were drafted in the
  working tree, and its tree is the pre-edit state; the measurement that decided this was taken in
  `/tmp/opencode`, not in this tree, so the rollback target is the state the decision was made
  against.

## H-0088 · A leg line that names a function promises nothing, and a cost with no host is not a measurement

- Question: the review's non-blocking 3 and 4 are one defect at two heights. `proc_leg` ends every ok
  with the shell function it called, so ten sentences about what ten legs require live in the
  pull-request body and not in the output a person reads at 3 a.m.; and the section's cost comment
  quotes "1.8 s for twelve legs" from an unnamed host where the review's box measures 11 legs at
  +6.1 s. Print the requirement where the name is, keep every measured number with the box it came
  from beside it, print the sentence and keep the name on the line too, or answer on the thread and
  change neither?
- Options: print the requirement in place of the function name and keep every measured pair with the
  box it came from named beside it | print the requirement and keep the function name printed on the
  line as well | keep the name and move the sentences into each predicate's failure detail | answer
  the review thread and change neither
- Chose: print the requirement in place of the function name and keep every measured pair with the box
  it came from named beside it
- Why: measured on both halves before writing. Ten call sites, `grep -c '^proc_leg '` = 10, every one
  now naming what it requires, and no `$*` left in the runner so a name cannot creep back. The
  argument is positional rather than a comment or an env var because a missing argument is then
  detectable: `proc_leg 'label' cmd` leaves `"$#" -eq 0` and the leg fails as "no command given, so
  this leg asked nothing at all", which is the file's own vacuity rule applied to its own plumbing.
  The name is not lost -- it is on the same source line as its sentence, and a failing predicate
  already prints its own detail under the FAIL, so nothing became harder to trace; what was
  untraceable was the promise. On the cost, three pairs on this box (`rodney-9950x`, btrfs,
  `~/prj/at-risk` present with 20 entries; the two bases timed in a throwaway clone checked out at
  each sha, since a base has to be timed in a tree holding a base's files): `450a628` 337 passed /
  1 failed at 10.67-10.71 s, `adbe6e3` 345 / 1 at 11.45-11.54 s, this head 358 / 0 at 13.13-13.32 s
  over six runs -- 12 legs and about +1.7 s over the main it merges into. The review's box: 337 / 0
  at 28.8 s to 348 / 0 at 34.9 s, +6.1 s for 11 legs, its totals one leg lower on both sides. That is
  0.14 s per leg here against 0.55 s there, four times the cost for one fixture, on a suite 2.7 times
  slower: exactly the variable `H-0048` says decides whether a quoted number means anything, and this
  paragraph had quoted one as a property of the code. The run-to-run spread on one tree is 0.2 s, a
  ninth of the effect being claimed, which is why the rows above are ranges -- the old line quoted
  12.565 s, three decimals on a quantity with a 0.2 s floor. The count is the same disease one floor
  down: the review's "+11" was right for the branch it read and the twelfth leg is `H-0087`'s, so
  "twelve legs" and "11 legs at +6.1 s" were both true of different trees and neither sentence said
  which, which is how two correct counts read as a contradiction. Corrected 10 Nov by `H-0089`, which
  is about this sentence most: `12 legs and about +1.7 s` subtracts main's `passed` column (345) from
  the merge's (357) rather than its leg TOTAL (346), and both bases carry a red leg -- the box leg,
  `H-0081`'s subject -- so the section is ELEVEN legs and twelve is only what this head is above main,
  over totals. The count inside the paragraph whose subject was quoted numbers decaying had itself
  decayed. The base's single failure is now
  identified rather than hedged: on a host that has the box, `the mirror box is the fixture's` reads
  the real 15:12Z rescue as this file's crime (`FAIL ... 1 quarantine patch(es) in
  /home/rodney/prj/at-risk`), so the author's base pair reproduces on a second such box -- 337 / 1,
  10.67 s against their 10.727 s -- and the review's 0-failed base is a fact about a host without it.
  Both were always true; the sentence had room for neither. What lost: `%-36s` lets two labels run
  into their requirements, because the labels are 39 to 46 characters, and a clipped column reads like
  a badly written sentence rather than a narrow field -- wrapping each requirement to its own line
  would double the section's output for ten lines, and that trade was taken deliberately. Deeper: the
  requirement is prose no gate can read. `bash -n` and shellcheck cannot tell a sentence from a shell
  word, so a future leg added with a placeholder in that column prints confident nonsense beside the
  real ones and nothing objects; the positional guard catches a missing argument, never a meaningless
  one. The checkpoint here, like `H-0087`'s, was taken after the edits were drafted, and its tree is
  the pre-edit state; the measurements that decided this were taken in a clone, not in this tree.
- Checkpoint: 0c1a6a8
- Effect: `factory/selftest.sh` -- `proc_leg` now takes `<label> <requires> <command...>` with a guard
  that fails a leg given no command, ten call sites each naming what they require, and the cost
  comment rewritten as the two-host table above. Read back on this tree before this commit: `bash -n`
  clean, `make check-shell` -> `check-shell: 0.10.0 over 28 scripts` exit 0, `bash
  factory/selftest.sh` -> `358 passed, 0 failed`, and `python3 scripts/check-decisions.py` -> 122
  entries, gate OK. Not in this commit: the review's non-blocking 5, the stale `(line 59)`, went in
  with `H-0087`, and the pull-request body -- still carrying "Twelve legs for 1.8 s", "Nothing here
  conflicts" and "four comments" -- is a status claim corrected by the next push, whose read-back
  lives in the pull request and not here.

## H-0089 · An eleven-leg section, counted as twelve by subtracting two `passed` columns

- Question: the cost paragraph and `H-0088` both carry the section as "12 legs and about +1.7 s",
  derived as 357 minus 345. The tallies on this box are 338 legs at the base (337 passed, 1 failed),
  346 at main (345 passed, 1 failed), 357 at the merge and 358 at this head, so the section is eleven
  and twelve is only what this head is above main, over totals. Re-derive over totals in the comment
  and annotate `H-0088` with a dated correction | rewrite `H-0088`'s sentence to read eleven and
  change nothing else | leave the comment and answer on the review thread | count the section's legs
  out of the source instead of subtracting two tallies?
- Options: re-derive over totals in the comment and annotate `H-0088` with a dated correction |
  rewrite `H-0088`'s sentence to read eleven and change nothing else | leave the comment and answer on
  the review thread | count the section's legs out of the source instead of subtracting two tallies
- Chose: re-derive over totals in the comment and annotate `H-0088` with a dated correction
- Why: the mechanism is worth more than the number it produced. This file prints one line,
  `N passed, M failed`, and the subtraction a person reaches for is over the first column -- but the
  first column is not the number of legs. Both bases here carry exactly one red leg (`the mirror box
  is the fixture's`, which #267 is now about), so every `passed` figure on that page sits one below its
  total and the error is invisible in the line that produced it, because the two columns are printed
  side by side with no total anywhere. The repair is therefore not "write 11": it is to put both
  operands and the subtraction in the sentence, `358 minus 346`, so a reader can redo it and see which
  column was used. Tie-break 6 carried the shape of the repair: `H-0086` is two commits old and did
  precisely this -- an `Effect:` that had reused a count measured over one set as a count over a
  subset, corrected by an annotation plus a new entry while the original words stayed standing. Same
  defect family, a denominator read off the wrong set, so same repair, which is what keeps the log a
  history instead of a draft. Rewriting `H-0088` to read eleven is the silent edit `#1180` was written
  for, and it costs something specific here: the wrong count is the evidence for the thing that
  paragraph argues, that a quoted number decays, and deleting it removes the demonstration. Answering
  only on the thread leaves a comment that says twelve next to a table whose rows add to eleven, which
  is the state the review of this pull request was filed against. Counting legs out of the source is
  better arithmetic aimed at a different question: the cost being quoted is the whole suite's wall
  clock, so the divisor has to be the legs the suite gained, and `grep -c '^proc_leg '` cannot see the
  box legs or the controls that live elsewhere in the file. What lost: a comment that shows its
  arithmetic is longer than the number it carries -- the paragraph grew by about six lines to hold two
  subtractions and their operands, and the next person with a stopwatch will probably write `+1.7 s`
  anyway. And the noise ratio moved with the re-derivation: 0.2 s of run-to-run spread is an eighth of
  +1.73 s, not the ninth of +1.7 s that `H-0088` said, so a correction bought another correction one
  decimal over.
- Checkpoint: 414a0f2
- Effect: `factory/selftest.sh`'s cost paragraph -- rows now name their totals with the pass/fail split
  beside them (338 at the base, 346 at main, 358 here), the delta reads `358 minus 346`, `two timed
  runs each` became `two at each base, six at this head`, the reviewer's rows read `337 legs` and `348
  legs` with its branch tip labelled as predating three commits of review fixes rather than compared
  as if it were this head, and the per-leg sentence now says 0.14 s against 0.55 s instead of dividing
  a delta by legs it had not derived. `H-0088` carries the dated annotation; its words are untouched.
  Read back on this tree before this commit: `bash -n` clean, `make check-shell` -> `check-shell:
  0.10.0 over 28 scripts`, `bash factory/selftest.sh` -> `358 passed, 0 failed`,
  `python3 scripts/check-decisions.py` -> 123 entries, gate OK. Comments and this log only: two files,
  no `.go`, no recipe, no shipped script. `make verify` is being re-run on the resulting tip because
  the tip it passed (`4bfbf08`) is not this tip, and its number goes in the pull-request body only if
  it comes back green.

## H-0090 · Both sides rewrote the same box leg: the snapshot decides, the marker annotates

- Question: this branch merged `adbe6e3` at `e74d94b` and `main` moved again after that, so the second
  merge brings `H-0083` and `H-0084`. The log conflict is mechanical, but `H-0084` rewrites
  `quarantine_stays_in_the_fixture` -- the same leg `H-0081` rewrote, for the same hazard, with a
  different instrument: `main` compares the box against a snapshot taken at the top of the file
  (`at_risk_at_start` + `at_risk_diff`), this branch counts patches newer than a marker (`at_risk_mark`
  + `-newer`). Which one decides, and what becomes of the other, when the merge has to keep both
  intents and every id on both sides is already a durable citation?
- Options: `main`'s comparison as the verdict and the marker as a printed cross-check | the marker as
  the verdict and the comparison as the extra check | keep `main`'s arm only and delete the marker
  machinery with its two control legs | keep both as verdicts and fail on either | hand the conflict
  back as `needs-human`
- Chose: `main`'s comparison as the verdict and the marker as a printed cross-check
- Why: tie-break 3 -- refuse rather than proceed on ambiguous input -- separates them, and it does so
  against this branch's own evidence. The comparison is mtime-blind: it names a patch that appeared
  whether or not its stamp moved, and it goes red in both directions, so a box this suite *emptied* is
  caught by the same arm. The marker cannot decide without re-opening the exact false accusation
  `H-0084` was filed for: a rescue that predates the run but carries a stamp newer than the marker makes
  the marker arm print "this file has written to the directory that holds the only copies of real
  work". This branch priced that hole itself for the review's non-blocking 2 -- a `cp -p`, a `tar -x`
  without `-m`, or a restore out of the box keeps its ORIGINAL mtime -- and wrote that the leg
  "cannot go red on it" as though that were acceptable. It is acceptable only once something else can,
  and the arm beside it is that something. So the merge does not pick a winner and discard a loser: the
  comparison closes the priced hole and the marker's own failure mode is printed below the leg instead
  of failed on, which is #1748 read the other way round -- a signal someone else can influence may
  annotate the trail, it may never decide when enforcement stops. Keeping both as verdicts is refused
  for the same reason: two assertions that can disagree produce a red gate that blames the wrong party,
  and blaming the wrong party is the defect both entries were filed for. Deleting the marker machinery
  is `#1180`'s silent edit with extra steps -- `H-0081`, `H-0087` and `H-0089` record it, and
  `at_risk_predicate_provable` and `the_insulation_is_load_bearing` are two legs that would have to
  vanish from the tally in a commit whose body says the merge changed no behaviour. Escalating is
  refused because none of the four never-an-agent's calls is in play: no dependency, no permission, no
  publish, no deletion -- two arms of test code that a tie-break separates. `main`'s `stray=`
  assignment inherits this branch's `|| true` (`H-0087`, review non-blocking 1), because it is the same
  assignment shape under the same `pipefail`; and the log is re-ordered by id with no id moved on either
  side, because `H-0085`..`H-0089` are cited by the unpushed commits `e74d94b`..`59f4c5c` exactly as
  `H-0079` yielded an id once in `H-0085`.
- Checkpoint: aceac33
- Effect: `factory/selftest.sh`'s `quarantine_stays_in_the_fixture` -- locals now
  `stray real by_mark where now changed`; `main`'s `now`/`changed` and its FAIL-then-return block kept
  verbatim as the verdict; the count of what is in the box is `real` in `main`'s meaning and this
  branch's marker count is renamed `by_mark`, because one name carrying two meanings is exactly what the
  auto-merge handed back -- it had kept `real=` from `main` and `real=` from here, three lines apart,
  answering two different questions; the `ok` lines are `main`'s, since the comparison is what earned
  them; `kept` leaves this leg (the predicate control leg keeps its own); a three-line `note:` prints
  when `by_mark` and the comparison disagree; `|| true` on `stray=`, `now=` and `by_mark=`.
  `factory/DECISIONS.md` takes `H-0083` and `H-0084` between `H-0082` and `H-0085` byte for byte. One
  citation is corrected because this merge is what broke it: the snapshot's own comment read
  `at line ~92` when the assignment sat at 100 on `main` and sits at 110 here, so it now names its
  position in words and says why no number is quoted (the review's non-blocking 5, applied to the line
  the merge actually touched).
  Read back on this tree before this commit: `bash -n` clean on `selftest.sh`, `factory.sh`,
  `common.sh`, `install-timer.sh`; the three mutant anchors occur 1, 1, 1 in `common.sh`/`factory.sh`;
  `python3 scripts/check-decisions.py` -> 125 entries, gate OK; `bash factory/selftest.sh` ->
  **368 passed, 0 failed**, exit 0. The tally is accounted for rather than assumed: 358 at `59f4c5c`,
  and `main` added exactly 10 `claim_leg` calls (`git diff adbe6e3 origin/main -- factory/selftest.sh |
  grep -c '^+claim_leg '`), one `pass` increment each, so all ten are #265's and none of this section's
  legs changed count -- corroborated statically by the pass-increment sites, 47 here + 45 on `main`
  − 44 at the base = 48 in the merge. The new arms measured on their own: with `by_mark=1` the three
  `note:` lines print with no missing argument; `now=$(find /nope ... | sort) || true` survives
  `set -euo pipefail` at `rc=0` with an empty list while the same assignment without `|| true` dies at
  `rc=1` printing nothing; `at_risk_diff '' '/a/two.patch'` -> `+ /a/two.patch (mtime unreadable)` and
  `at_risk_diff '/a/three.patch' ''` -> `- /a/three.patch (gone)`, so the verdict still names both
  directions. A trap for whoever next merges into a conflicted tree, measured here: the same suite run
  while the index still held the three conflict stages printed `367 passed, 1 failed` with
  `the timer's scripts are executable ... not 100755 in the index: selftest.sh(100755 100755 100755)`
  -- `git ls-files -s` answers three rows for one unmerged path, so that leg reads the index and not
  the code, and the gate has to be run after `git add`.
  What lost: `main`'s `stray=` had no insulation and now carries it, so `the_insulation_is_load_bearing`
  is load-bearing for `main`'s line too; this branch's `ok` wording -- "none written by this run …
  (N rescue(s) already there from before this run, left alone)" -- gives way to `main`'s, which makes
  the box-leg output quoted in the pull-request body stale, and that body is what blocking 1 is about
  anyway; `H-0084` has no blank line before its heading on `main` and it is kept that way rather than
  tidied, because a merge commit is not where another decision's entry gets reformatted
  (`check-decisions.py` reads both shapes); and the marker arm no longer decides anything, so a case it
  used to fail is now a sentence an operator has to read rather than a gate that stops them.

## H-0091 · The out-of-order refusal prints the predecessor it read, not one it rebuilt

- Question: what name does an out-of-order refusal print for the entry before the offending one?
  `check()` carried only the number (`prev_num`) and the formatter re-attached a prefix by writing
  `"D-%04d" % prev_num`, so the harness log could only ever be told a `D-` name. Measured 10 Nov in
  `/home/rodney/prj/wt/issue-263` while resolving PR #264's `factory/DECISIONS.md` conflict: both
  sides appended at the same offset, git's layout put `H-0079` after `H-0082`, and the gate said
  `factory/DECISIONS.md:2159: H-0079 is out of order (D-0082 came before it)`. `D-0082` exists in
  neither log -- `DECISIONS.md` carries 39 entries and stops at `D-0052` -- so the one sentence a
  person reads while `make verify` is red sent them to grep for an entry that is not there. The
  location, the verdict and the exit status were all correct; only the citation was invented.
- Options: carry the previous entry's id string in the loop and print it as parsed | re-derive the
  prefix from the offending entry (`"%s-%04d" % (e["id"][0], prev_num)`) | print no predecessor, name
  only the offending entry | leave the formatter and widen the wording to "an earlier entry"
- Chose: carry the previous entry's id string in the loop and print it as parsed
- Why: the cheap tie-breaks do not separate the first two -- each is one line in `check()` and neither
  touches another file -- so what decides it is the sentence's only job: hand over a name a person can
  point `grep` at. Printing the id that was parsed makes the refusal quote the log. Re-deriving it
  reconstructs a name from a number plus an assumed prefix, which is the bug's own invention with the
  guess taken from the wrong side. That is measured, not reasoned: `--log` accepts any path and
  `parse()` imposes no prefix on a file (both logs match the same `ENTRY`), so a file holding both id
  spaces is reachable -- a merge that took both sides into one file produces exactly that, and
  `cross_log_problems` cannot refuse it here because pointed at one file it sees one label. Aimed at
  `## D-0082` followed by `## H-0079`, the carried id answers `(D-0082 came before it)`, which is what
  is actually above it; the re-derived prefix answers `(H-0082 came before it)`, an id in neither log,
  and `--self-test` stayed green doing it -- which is why there are three fixtures below and not the
  two the ticket asked for. Naming no predecessor is smaller still but deletes the actionable half of
  the rule: it exists so that "revert to before X" names one commit, and the offender's own number
  never tells whoever is mid-conflict which of the two sides moved. Widening the wording keeps a wrong
  number on the screen and buys nothing a `grep` does not already give.
- Checkpoint: 4b5c541
- Effect: `scripts/check-decisions.py` `check()` -- `prev_id` is set beside `prev_num` in the same
  branch (`if e["num"] > prev_num: prev_num, prev_id = e["num"], e["id"]`), so number and id cannot
  name two different decisions, and the message prints `prev_id`. Three `BAD` fixtures, not one:
  `out-of-order` (the existing `D-` log, which is where a hardcoded `D-` happens to be right and is
  the reason the bug survived `--self-test`), `out-of-order-harness` (the mirror the ticket asks for,
  built as `GOOD.replace("D-", "H-")` so the two differ only by prefix), and `out-of-order-mixed`,
  the both-spaces-in-one-file case that exists only to keep the re-derived variant from coming back.
  The single `expect_fail(..., "out of order")` becomes a three-row loop asserting the *id* named, not
  just the phrase -- the old needle was printed by the buggy message too, so the arm was covered while
  the name was wrong -- and each single-space log additionally asserts it cites no `D-`/`H-` id from
  the other space. Measured on this tree, each mutant applied to the working tree from a `/tmp` backup
  and restored, every line `python3 scripts/check-decisions.py --self-test; echo $?`: fixed -> `19
  cases that must fail, 9 that must pass`, exit 0; the shipped hardcoded `D-` -> exit 1,
  `out-of-order-harness: the refusal does not name H-0082 came before it`; the re-derived prefix ->
  exit 1, `out-of-order-mixed: the refusal does not name D-0082 came before it` (and exit 0 before the
  third fixture existed, measured); the arm deleted -> exit 1, all three rows reporting `expected a
  problem, got none`. The gate itself is unchanged: same location, same verdict, same exit 1, on the
  reproduction from the ticket (`--log` over an out-of-order `H-` file) -- the fix moves the name, and
  `check-decisions` still reports `117 entries across 2 log(s)` clean over the real logs. What lost,
  stated: the printed name is now the heading's own text rather than a form this file controls, so a
  future third id space sharing one log would be quoted across spaces without a refusal -- the mixed
  fixture pins the id, it does not forbid the arrangement, because `ENTRY` accepts `[DH]-` by design
  and this gate's rule against two logs meeting is `cross_log_problems`, which one arbitrary `--log`
  path cannot trip.
  Corrected 10 Nov by `H-0094`, which leaves these words as written rather than editing them: the
  `117 entries across 2 log(s)` above is the count at the checkpoint `4b5c541`, one short of the
  count at the commit that carries this entry. `python3 scripts/check-decisions.py` at `c149fe9`
  prints `118`, and the missing one is this entry -- the two logs hold 39 `## D-` headings and 79
  `## H-` headings there, 78 at `4b5c541`. This pull request's body said `118` in the same breath,
  so the record and the status claim disagreed with each other by exactly one entry, and the merge
  `6442e8f` moved the number twice more: 127 over the two sides' entries, 128 from `H-0093` on.

## H-0092 · The empty-log refusal names both headings, because it cannot know which log it reads

- Question: what heading does the `no entry found` refusal show the reader? `check()` printed one
  string for every file with a `D-` example in it, so aimed at an empty harness log it told that reader
  to look for a heading the file can never hold. Measured 10 Nov in this tree at `5a1a559`, repeating
  #268's own probe rather than quoting it: `printf '# Harness decisions\n\nNothing here yet\n' >
  /tmp/empty-h.md` then `python3 scripts/check-decisions.py --log /tmp/empty-h.md` printed
  `nothing in it matched ` + backticked `## D-0001 · …` and exited 1. Verdict, exit status and refusal
  were all correct, and this is a smaller thing than the citation `H-0091` removes: that message
  asserts a fact and invented it, this one illustrates a format. What made it worth an entry is that
  `--self-test` could not have caught it either way, and still could not after a one-word fix.
- Options: name both prefixes in the one string | pass the log's prefix in from `main()`/`parse()` and
  print only the one this file writes | print the grammar rather than an example (a
  `<prefix>-NNNN` heading with no concrete id) | name no id and widen the wording to "a decision
  heading" | change only the message and leave the assertion at the phrase it uses today
- Chose: name both prefixes in the one string
- Why: the cheap tie-breaks do not separate the first three -- each is one string in `check()`'s
  return, none adds a dependency or touches a second file, all three still refuse. What separates them
  is whether the sentence is allowed to assert something it cannot read. For the out-of-order arm there
  is a fact in the file, the predecessor's own heading, so `H-0091` carries that id and refuses to
  re-derive a prefix from a number plus a guess -- measured there, `--log` takes any path, `parse`
  imposes no prefix on it, and a merge that took both sides into one file is reachable. This arm
  returns before a single entry is parsed, so there is no fact to carry and any one prefix it prints is
  a guess about a file it did not read: `DECISIONS.md` and `factory/DECISIONS.md` match the same
  `ENTRY`, and `cross_log_problems` cannot tell one arbitrary `--log` path which space it belongs to.
  Naming both shapes is therefore the only sentence true of every log, and it is literally what the
  grammar accepts. An example beats the bare grammar because the sentence's one job -- the leg `H-0091`
  used for the same choice -- is to hand over something to paste or grep, and `<prefix>-NNNN` is a
  pattern a person has to compile before using it; widening to "a decision heading" loses the separator
  and the heading level as well as the id. Leaving the message alone is not an option that was
  considered and lost, it is the bug: the new fixture below is red against it, measured as M1.
- Checkpoint: 3fca303
- Effect: `scripts/check-decisions.py`. The refusal reads ``matched `## D-0001 · …` or `## H-0001 ·
  …``` `, so it describes `ENTRY` rather than guessing the file. The leg that keeps it is the larger
  part, because the unpinned assertion is the reason the bug was shippable: the `no-entries` case
  asserted only the substring "no entry found", a phrase printed whichever example sits in the
  sentence, so the arm was covered while its only id was wrong for half its readers. Two rows now
  assert each empty log is offered the heading its own space writes, over a new `BAD` fixture
  `no-entries-harness` -- the mirror the suite never had, since every other fixture is a `D-` log
  where the hardcoded example happens to be right -- and one loop sweeps every `BAD` fixture for the
  obligation both messages owe: an id in a refusal has to resolve, a *citation* as an entry in the log
  being read and an *example*, a backticked heading read exactly as printed, as a heading `ENTRY`
  parses. `ID_TOKEN` and `EXAMPLE_HEADING` are the two shapes; a citation is what survives deleting
  every example from the sentence, so a message cannot exempt its own invention by putting a `##` in
  front of it. Sweeping `BAD` rather than naming an arm is what makes it one rule for both messages
  and costs no edit when a fixture is added -- measured on PR #269's own text: its three fixtures
  spliced into `BAD` without its fix make this sweep refuse (`out-of-order-harness: the refusal cites
  D-0082, which is not an entry in the log it is reading`, exit 1), and splicing its fix as well
  returns exit 0. So the out-of-order message is held to the rule #268 asks it be held to, by a line
  that does not touch the lines #269 is editing. Measured on this tree, each mutant applied to the
  working tree from a `/tmp` backup and restored with `cp`, one line per row, every one
  `python3 scripts/check-decisions.py --self-test; echo $?`: control exit 0 at 28 cases that must
  fail and 9 that must pass, up from 17 and 9 at `3fca303` (+2 empty-log rows, +10 swept fixtures,
  -1 row the loop replaces); the shipped single `D-` example exit 1, `no-entries-harness: the refusal
  does not offer `## H-0001 · …``; the mirror single `H-` exit 1 naming `no-entries` instead, so the
  pin is symmetric and not `D-`-phobic; `## D-001 · …` exit 1, `offers '## D-001 · …' ... ENTRY does
  not parse`; an em-dash separator exit 1 the same way; `# D-0001 · …` exit 1 the same way, which is
  what the heading level is doing in the check -- an earlier draft rebuilt the heading from the id it
  had extracted and so could never fail, and M3 through M5 are that draft's refutation; an invented
  citation in the out-of-order arm (`"D-9999"`) exit 1, `cites D-9999`. The gate itself is unchanged:
  same verdict, same exit 1, on #268's reproduction, and the real logs still read
  `126 entries across 2 log(s)`.
  What lost. An empty `DECISIONS.md` is now also shown an `H-` heading it will never write: the message
  is a statement about the grammar, not about the file, and the majority reader pays for the minority's
  correctness. Accepted because the only way to make it shorter is to assert which file it is, and that
  is the guess `H-0091` refused eleven entries ago. Passing on `pass the prefix in` also forgoes the
  shorter message it buys -- if a trustworthy per-log id space is ever built, the two empty-log rows
  still pass unchanged, because they pin which heading a reader is offered and not how it was chosen.
  `self_test()`'s closing `print` still enumerates "the cross-log id rule and checkpoint existence"
  without naming this rule, deliberately: PR #269 rewrites those same three lines and two pull requests
  editing one caption is a conflict handed to the supervisor's merge path, so only the numbers inside
  that sentence move (17 to 28). The citation arm exempts an id printed as a heading even where that
  heading cites a real entry -- it is then still held to the parse arm, which is the seam between the
  two obligations and is stated rather than closed -- and an example printed *without* backticks would
  be read as a citation, which the one message that has an example does not do. One measurement about
  this entry's own id, because the allocator reads like a stolen id if nobody writes it down: the first
  two reads answered `H-0092`, the third answered `H-0093`, and the delta is this file -- the allocator
  unions the current checkout, so once the draft carrying `## H-0092` was in the working tree the next
  free id above it was 93. Aimed at the main checkout, where no draft exists, the same command still
  answers `H-0092`, and the ref union's own ceiling is `H-0091` (PR #269's). Nothing was renumbered and
  nothing collided; a later tick that re-reads after writing its own entry is reading itself.

## H-0093 · The same log conflict as `H-0085`, and this time the log is the only thing in the way

- Question: PR #269 reads `CONFLICTING`/`DIRTY` against `5a1a559` with all five checks `SUCCESS`.
  The conflict is one hunk of `factory/DECISIONS.md`: the merge base `91db846` ends at `H-0084`,
  `main` inserted `H-0080`-`H-0082` between `H-0079` and `H-0083` -- that insertion merged clean --
  and appended `H-0085`-`H-0090`, while this branch appended `H-0091` at the same offset, so git
  handed each side half of one hunk. This branch also edits `scripts/check-decisions.py`, the gate
  that reads this file's order, and `main` adds 936 lines to `factory/selftest.sh`. Take git's
  layout and let CI notice | keep only `main`'s log text and re-file this branch's entry | renumber
  `H-0091` under `H-0090` | rebase onto `origin/main` | keep both sides' entries, ordered by id, and
  re-measure the merged tree?
- Options: take git's conflict layout and let CI notice | keep only `main`'s entries and re-file this
  branch's | renumber `H-0091` under `H-0090` | rebase onto `origin/main` | keep both sides' entries,
  ordered by id, and re-measure the merged tree
- Chose: keep both sides' entries, ordered by id, and re-measure the merged tree
- Why: `H-0085` decided this class one entry and one pull request ago, on a smaller version of the
  same hunk, and tie-break 6 -- do what the surrounding history already does -- settles it before
  reasoning starts. What the re-measurement adds is that git's layout is not merely untidy here, it
  is red, and red differently under each gate: over the same bytes with `H-0091` above `H-0085`,
  this branch's `check-decisions.py --log` returns six refusals naming `H-0091 came before it`,
  while `origin/main`'s copy returns the same six naming `D-0091` -- an id in neither log, because
  `DECISIONS.md` stops at `D-0052` and that formatter rebuilds a prefix from a number. (The two runs
  differ in their totals for a reason with nothing to do with the fix: `main`'s copy was run from a
  directory that is not a git repository, so its checkpoint arm fires on all 88 entries. The six
  order refusals are the same six; only the name differs.) That is `H-0091` paying for itself on the
  day it merges -- the conflict that produced the invented name is the one that shows the fix.
  Ordering by id is not an invention of this merge either: it is what the allocator's contract
  already asserts, since both sides numbered against the union of `refs/heads` and `refs/remotes`,
  so the numbers are the history's order and only the text disagreed. No id moves, for the reason
  `H-0085` gives: `H-0091` is cited by the pushed commit `c149fe9`, by this pull request's body, and
  by a review that reads `APPROVED` at exactly that head, so renumbering would repoint a citation
  the approving review is pinned to. Rebase loses the three ways `H-0085` lists them -- no rebase
  merge on this repository, force-push denied to a tick, and an approving review that would stop
  describing the head it reviewed. Keeping one side's log text deletes the other's record, which is
  the thing this file exists to prevent. And the hazard #1048 makes merges dangerous -- a merge
  silently retiring a mutant anchor -- has no surface here, measured rather than assumed: no row in
  `scripts/mutations/*.json` names a file under `scripts/` (every `file` value is a `.go` or a
  `.tmpl`), and the merged `factory/selftest.sh` is byte-identical to the `main` copy that was green
  before the merge, so this branch contributed no leg and lost none.
- Checkpoint: efc31fb
- Effect: as this commit leaves it, `factory/DECISIONS.md` reads `H-0083` and `H-0084` (base),
  `H-0085`-`H-0090` (`main`), `H-0091` (this branch), `H-0093` (this entry), and no id moved on
  either side. "Both intents kept" was read back rather than declared, per `#1180`:
  `git diff origin/main -- factory/DECISIONS.md` deletes 0 content lines and adds 54 (one heading),
  `git diff c149fe9 -- factory/DECISIONS.md` deletes 0 content lines and adds 429 (nine headings) --
  each side loses none of the other's text. The two scripts did not interleave at all:
  `git diff origin/main -- factory/selftest.sh` and `git diff c149fe9 -- scripts/check-decisions.py`
  are both empty. On the merged tree: `python3 scripts/check-decisions.py` -> `127 entries across 2
  log(s)` exit 0, and `128` with this entry; `bash factory/selftest.sh` -> `368 passed, 0 failed` in
  14.1 s on this box (totals are box-dependent -- that is not another box's number and does not
  claim to be); `make verify` -> `verify: OK` in 1 m 34 s. What this commit does NOT do: the pull
  request's body still says `118 entries across 2 log(s)`, which was true of `c149fe9` and is false
  of this merge, so the body -- a status claim -- gets edited, per `#1180`; and `H-0091`'s `Effect:`
  says `117 entries`, which was the count at its checkpoint `4b5c541` and one short of its own
  commit -- that is a decision record, so `H-0094` annotates it instead of rewriting it.
  Corrected 10 Nov by `H-0094`, which likewise leaves these words standing: "one entry and one pull
  request ago" in the `Why:` above -- and in this merge's own message, which is committed and stays
  as committed, per the `e334f4f`/`a7a656d` precedent `H-0085`'s `Effect:` sets -- overstates the
  closeness. Measured distance from `H-0085` to `H-0093` is seven headings (`H-0086`-`H-0091` in
  between) and eight ids, `H-0092` being held on the sibling branch `issue-268`, which is the
  allocator's union of `refs/heads` declining to hand out an id already spoken for. One pull request
  ago is right: `H-0085` was `#264`.

## H-0094 · `H-0091` counts the log one entry short of its own commit

- Question: `H-0091`'s `Effect:` closes with "check-decisions still reports `117 entries across 2
  log(s)` clean over the real logs", and that sentence is being read into a log the merge just
  moved. Measured on this tree: `git show 4b5c541:DECISIONS.md | grep -c '^## D-0'` plus the same
  over `git show 4b5c541:factory/DECISIONS.md | grep -c '^## H-0'` is 39 + 78 = 117, while at
  `c149fe9` -- the commit that carries the entry -- it is 39 + 79 = 118, which is what the gate
  itself printed there. So 117 is the count at the checkpoint, not at the commit, and this pull
  request's body prints `118` while the entry that records the same change prints `117`. Rewrite
  `H-0091`'s line to read 118, delete the count and keep the verdict, leave the log and correct
  only the body, or annotate `H-0091` with a dated correction and add this entry?
- Options: rewrite `H-0091`'s `Effect:` line to read 118 | delete the count from `H-0091` and keep
  the sentence's verdict | leave the log alone and correct only the pull request's body | annotate
  `H-0091` with a dated correction and add this entry
- Chose: annotate `H-0091` with a dated correction and add this entry
- Why: `H-0086` is the precedent seven headings up this file and it acted on the same object -- an
  `Effect:` whose number does not match the command it names -- and it settled the shape there, so
  tie-break 6 decides before reasoning does: a decision record gets a dated correction, because a
  silently corrected log is indistinguishable from one that was always right, while a status claim
  is edited in place, and the pull request's body is the status claim. Rewriting in place is the
  smaller diff and is precisely what `#1180` refuses. Deleting the count loses the only arithmetic
  in the file that shows what the merge cost: 117 at the checkpoint `4b5c541`, 118 at the commit
  `c149fe9`, 127 at the merge `6442e8f`, 128 with `H-0093` in it -- four numbers, and the move from
  118 to 127 is `origin/main`'s nine entries, not a regression. Silence is the worst of the four
  because the sentence is stale twice over by then: off by one against its own commit (118 at
  `c149fe9`) and twelve against the head this entry lands on (129), and its one job
  was to say the gate had not changed. What lost, stated plainly: the entry now carries two numbers
  for one claim, and this file's gate cannot tell a corrected count from a contradictory one --
  `check()` checks field presence, id order, `Chose` membership and sha shape, never arithmetic --
  so the correction holds only because a person reads the prose.
- Checkpoint: 1205945
- Effect: `H-0091`'s `Effect:` keeps every word it had and gains the dated lines under it; this
  entry is the trace, and nothing was renumbered. Read back before this line was written, per
  `#1180`: the annotation alone leaves the count where `6442e8f` left it -- 39 `## D-` headings plus
  89 `## H-` headings, and `python3 scripts/check-decisions.py` there printed `128 entries across 2
  log(s)`, exit 0 -- because an annotation adds no heading, which is the difference between this
  commit and the rewrite it is not; with this entry added the same command prints `129`, exit 0.
  `python3 scripts/check-decisions.py --self-test` -> 19 must-fail, 9 must-pass, exit 0;
  `make verify` -> `verify: OK`. The same rule is applied one entry up: `H-0093` says `H-0085`
  settled this "one entry ago" where the measured distance is seven headings and eight ids, and it
  is annotated there rather than edited, while `6442e8f`'s message carries the same phrase and
  stays as committed because a message is permanent history and `#1180`'s remedy for that is the
  record beside it, not a rewrite. What this commit does not do: `#269`'s body is a status claim
  and is rewritten in place to the merged numbers rather than annotated, and the merge `6442e8f`
  stands as the thing that had to be re-measured.

## H-0095 · The read-back is owed after the push, because the push is what spends the approval

- Question: what does a tick owe a pull request whose approval its own push has just voided?
  Step 1 says read the review state before any claim about a pull request, and Step 5 says push and
  open or update the pull request -- and stops. A tick that follows that text exactly delivers a body
  quoting `verify: OK` while `reviewDecision` has moved underneath it.
- Options: nothing new, since Step 1's pre-flight read already covers "read before you claim" |
  re-read the review state after the push and name the dismissal in the pull request, in `SKILL.md`
  only | re-read and additionally let the tick re-ask the reviewer | re-read and have the supervisor
  rewrite the body on the tick's behalf when it sees the flip | write no rule and leave a comment
  beside Step 5
- Chose: re-read the review state after the push and name the dismissal in the pull request, in
  `SKILL.md` only
- Why: the first option is the bug, and it is measured rather than argued. Read back at this commit,
  live: `gh pr view 269 --json reviewDecision,reviews,reviewRequests` answers `REVIEW_REQUIRED`, one
  row -- `hermesvicowa`, `state: DISMISSED`, `commit.oid: c149fe9`, `submittedAt
  2026-11-10T18:23:50Z` -- and `reviewRequests: []`, with the head at `ec74cf5`. The only thing that
  happened between the approval and that row is the push that cleared `mergeable=CONFLICTING`, and
  there is no way to clear one without a push here: squash and rebase merge are both disabled,
  force-push is denied to a tick, and reverting the base is not an operation. So the state is
  reachable by every conflict-resolution tick and invisible to the instructions that govern one.
  Handing the re-ask to the tick (option three) is the move this harness has already refused twice:
  every `gh pr review` shape is in the deny list a tick runs under, and `gh pr edit --add-reviewer` is
  measured failing on this account with only a `Projects (classic)` line on stderr while
  `/requested_reviewers` stays empty (`D-0010`) -- a verb that looks like it worked is precisely how
  "nobody ever asked" came back once. Option four puts a second writer on a document the tick owns: the supervisor cannot know
  which claims in that body the dismissal invalidates, and the pull-request body is the review's
  input. Neither adds a dependency or touches the daemon, so the cheap tie-breaks do not separate
  them; what separates them is that the chosen one makes the tick *report the state it cannot fix*
  rather than proceed as though it could, and it is the one that touches the fewest files -- the
  supervisor's other half of this finding is `H-0097`, in its own commit, because it is a different
  file with a different blast radius. Corrected by that entry on 10 Nov: this read `H-0096`, because the
  allocator answered `H-0096` at checkpoint `f5dab5b` and `1d66965` put `main`'s own `## H-0096` -- the
  third log collision, about something else entirely -- in front of this branch while its checkpoint sat
  here. The citation is annotated rather than quietly rewritten: a corrected log that says nothing about
  having been corrected cannot be told apart from one that was always right. The bare comment (option
  five) is what this repository does not
  have, on the reasoning already in `Makefile`'s note above `verify:`: a rule nobody can watch go red
  is a suggestion, which is why `H-0097`'s legs read both documents.
- Checkpoint: f5dab5b
- Effect: `.opencode/skills/factory/SKILL.md`. Step 1 gains the pointer -- the same read is owed again
  after the push -- so the pre-flight paragraph no longer reads as the whole obligation. Step 5 gains
  "The push may have spent the approval. Read the review state back.", carrying: the two read-backs
  above with the push that separates them, the read spelled as its own call
  (`gh pr view "$PR" --json reviewDecision,reviews,mergeable,mergeStateStatus`) with no `--jq` and
  #270 named as the reason, the one-line disclosure (who, which commit, and that `mergeStateStatus`
  reads `BLOCKED` until a reviewer is asked again), the body-rewrite verb -- `gh issue edit "$PR"
  --body-file`, because `gh pr edit` dies on `projectCards` while looking fine -- with the rule that
  the read-back is the sentence being there and not the call returning, and the timestamp trap: a
  dismissed row's `submittedAt` is the approval's own stamp, so a dismissal dated from #269's row
  lands twenty minutes *before* the push that caused it.
  What lost: the tick still cannot re-ask, so between its push and the next activation the unit is
  genuinely unserviced, and the body has to say so rather than imply otherwise -- #241 wants the poke
  widened from one reviewer to the team and is not this entry's to settle. The disclosure is prose
  with a leg behind it for the *sentence in the file*, not for the sentence in any one pull request:
  nothing here can force a tick to write the line, only catch the day the instruction that tells it to
  is deleted. And `mergeStateStatus` was `UNKNOWN` rather than `BLOCKED` on the read that refreshed
  this entry, because GitHub recomputes it after a push, so what the instruction hands over is the
  field to read, not the word it will return.

## H-0096 · The log collides with the same neighbour a third time, and this time the silent half is the auto-merged gate

- Question: `#269` reads `CONFLICTING`/`DIRTY` against `origin/main` with all five checks `SUCCESS`
  (`verify`, `shellcheck`, `mutation sets`, `packages (amd64)`, `packages (arm64)`). The merge base is
  `5a1a559`; `main` took `#271` (`H-0092`) into `9593797` while this branch appended `H-0091`, `H-0093`
  and `H-0094` at the offset `main` used for `H-0092`, so `factory/DECISIONS.md` is the one conflicted
  file -- the third occurrence of the class `H-0085` settled and `H-0093` re-settled. What is new is not
  the conflict, which git announces, but the part it does not: `scripts/check-decisions.py`, the gate
  that reads this file's order, was edited by both sides inside the same three regions -- `check()`'s
  empty-return arm, the `BAD` fixtures, `self_test()`'s rows -- and git merged it without a word. A
  log conflict costs a merge; a silently retired leg costs a green suite that no longer checks the
  thing it was written to check. So what decides this is not a resolution but a measurement: does the
  merged gate still hold both sides' rules?
- Options: take git's conflict layout and let CI notice | keep only `main`'s entry and re-file this
  branch's three above it | renumber this branch's ids above `H-0092` | rebase onto `origin/main` |
  keep both sides' entries ordered by id and re-measure every leg on the merged tree
- Chose: keep both sides' entries ordered by id and re-measure every leg on the merged tree
- Why: tie-break 6 settles the layout before reasoning starts -- `H-0093` decided this exact shape one
  sibling branch ago and `H-0085` before it, and the ordering is not an invention of the merge but what
  the allocator already asserted: it refused to hand `H-0092` to this branch because `issue-268` held it
  in the `refs/heads` union, and it answers `H-0096` here with `issue-272` holding `H-0095`. Three
  things are measured this time rather than argued. **Git's layout is red, and red with a name that
  resolves:** over the bytes git handed back (`H-0091`, `H-0093`, `H-0094`, then `H-0092`) the merged
  gate answers `H-0092 is out of order (H-0094 came before it)`, exit 1; the same bytes against
  `origin/main`'s copy of the formatter answer `(D-0094 came before it)`, and `grep -c '^## D-0094'` is
  0 in both logs while `^## H-0094` is 1. So the conflict that produced `H-0091`'s invented name now
  demonstrates the fix on identical bytes -- the A/B `H-0093` could only describe from two trees is
  runnable in one. **No id moves, and `main`'s own entry is the reason:** `H-0092`'s `Why:` cites
  `H-0091` by name twice ("the assumption H-0091 refused", "the guess `H-0091` refused"), so
  renumbering this branch's entries would break a citation inside the entry this merge is bringing in --
  a worse version of the repointed citation `H-0093` refused, because that one pointed at this branch's
  own record and this one would edit the other side's. The review on `#269` put the same fork to the
  supervisor as finding 2 ("either 269 merges first, or this entry gets renumbered") after simulating
  only the append layout, which it measured red at `H-0091 is out of order (H-0092 came before it)`;
  ordering by id answers it without moving anything. **The auto-merge is where the loss would have
  hidden, so each side's leg was attacked in the merged tree**, each mutant applied to the working tree
  from a `/tmp` backup with a unique-anchor guard (an anchor found twice is not applied, exit 2), and
  every line `python3 scripts/check-decisions.py --self-test; echo $?`: control exit 0 at 32 must-fail
  and 9 must-pass; `M1`, `main`'s empty-log example put back to the single `## D-0001` -> exit 1,
  `no-entries-harness: the refusal does not offer ``## H-0001 · …``, so `#268`'s leg is held by this
  branch's suite and not merely present; `M2`, this branch's formatter put back to `"D-%04d" % prev_num`
  -> exit 1 in three lines, two of them `main`'s new sweep
  (`out-of-order-harness: the refusal cites D-0082, which is not an entry in the log it is reading`, at
  fixture lines 9 and 19) and one this branch's own row (`does not name H-0082 came before it`); `M3`,
  the re-derived prefix -> exit 1, again both rules (`out-of-order-mixed: the refusal cites H-0082,
  which is not an entry...` and `does not name D-0082 came before it`). Both behaviours are also read
  back live rather than through fixtures: an empty harness log is offered `## D-0001 · …` or
  `## H-0001 · …`, and a both-spaces file is refused naming `D-0082`, the heading actually above the
  offender.
- Checkpoint: 1690e06
- Effect: `factory/DECISIONS.md` leaves this commit reading `H-0090`, `H-0091` (this branch), `H-0092`
  (`main`), `H-0093`, `H-0094` (this branch), `H-0096` (this entry), with no id moved on either side.
  "Both intents kept" is read back, per `#1180`: `git diff --numstat origin/main -- factory/DECISIONS.md`
  and `git diff --numstat 1690e06 -- factory/DECISIONS.md` both report **0 deletions** -- insertions only,
  in either direction, so neither side lost a line of the other's text. The insertion totals are a
  function of this entry's own length, which is the trap `H-0094` records, so only the resolution is
  quoted as a number: 172 lines against `origin/main` and 82 against `1690e06`, measured before this
  entry was appended. Both read with `git diff --numstat`, which is what makes the zero exact: counting
  added lines with `grep` on `^+` plus one character, the method `H-0093`'s `Effect:` uses, misses every
  added blank line -- six and two of them here, and
  `git diff --name-only origin/main` names only `factory/DECISIONS.md` and `scripts/check-decisions.py`
  -- `factory/selftest.sh` is byte-identical to the `main` copy that was green before the merge, so this
  branch contributes no leg and loses none. `#1048`'s merge hazard has no surface, re-checked on the
  merged tree and not inherited from `H-0093`: across `scripts/mutations/*.json` every `"file"` value is
  a `.go` or a `.tmpl` (`NON_GO_TMPL_COUNT=0`) and nothing in that directory names `check-decisions`,
  `selftest` or `DECISIONS`. Read back on the merged tree with this entry in it:
  `python3 scripts/check-decisions.py` -> 131 entries across 2 log(s), exit 0 (39 `## D-` in
  `DECISIONS.md` plus 92 `## H-`; 130 before this entry, which is the number the resolution alone
  printed); `python3 scripts/check-decisions.py --self-test` -> 32 must-fail, 9 must-pass, exit 0 --
  `main`'s 28 plus the two fixtures this branch added to the swept `BAD` dict, minus the single
  out-of-order row its three-row loop replaces; `make verify` -> `verify: OK`, which runs
  `factory/selftest.sh` at `Makefile:113` (`368 passed, 0 failed` in 13.9 s standalone on this box --
  totals are box-dependent and this is not another box's number).
  What lost. `H-0091`'s third fixture stops being load-bearing in the way this pull request's own
  approving review measured it: that review got the re-derived prefix to exit 0 at 18 cases by deleting
  the mixed row, and on the merged tree `M3` is caught twice, so the row is no longer the only thing
  between that variant and green. It is kept, because the two rules are not the same rule -- `main`'s
  sweep refuses a citation that resolves nowhere, this row pins *which* resolvable id is named, and a
  mutant that printed a real-but-wrong predecessor (the first entry, say) satisfies the sweep and fails
  the row. Also lost, and kept for the same reason: the merged `self_test()` caption
  (`scripts/check-decisions.py:679-682`) is this branch's sentence and names neither of `H-0092`'s rules
  -- `H-0092` deliberately touched only the numbers inside it so two pull requests would not edit one
  caption, and the merge takes ours because theirs never moved. It is prose printed on a green run, so
  it is recorded here instead of being fixed inside a merge commit.
  What this commit does NOT do. The merge makes three sentences in the log stale and edits none of them:
  `H-0092`'s `Why:` says "the guess `H-0091` refused eleven entries ago" where the merged log puts
  `H-0091` one heading above it (`:2719` and `:2782`, zero headings between); `H-0093` and `H-0094` both
  say `H-0092` is "held on the sibling branch `issue-268`", true of the allocator when written and now a
  merged heading six lines away. `#1180`'s remedy for a stale record is a dated correction beside it, not
  a silent rewrite, and doing it here would have been worse than merely early: one deleted line from the
  other side's entry destroys the 0-deletion diff that is the whole evidence this resolution kept both
  intents. So this entry is the record, and the three sentence-level annotations remain a forward commit
  for whoever takes them -- the pull-request body names them so the next tick inherits them as work, not
  as a discovery.

## H-0097 · A voided verdict is not a review, so the queue re-asks instead of waiting on it

- Question: what does the queue owe a pull request whose only verdict a push has just voided? `#272`
  measured `#269` going `APPROVED` -> `REVIEW_REQUIRED` across one fast-forward, unforced conflict
  resolution (`c149fe9..ec74cf5`), its single row turning `hermesvicowa APPROVED` -> `hermesvicowa
  DISMISSED` at the same `c149fe9`, and `/requested_reviewers` empty on both sides of it. The
  supervisor's own journal says what `triage.sh` then made of that state:

  ```
  17:58:17Z POKED pr:269 requested review from hermesvicowa    no rows at all: it asked
  19:17:23Z WAIT  pr:269 awaiting_review                       one voided row: it did not
  ```

  `H-0095` made the tick see the dismissal and write it into the pull request. Nothing here made the
  queue act on it, so the two documents would have disagreed by design: the guide's table promising an
  ask, the classifier printing a wait, and `#269` moving only because a person read it -- re-approved
  19:50:20Z against the new head, merged 20:18:19Z. A voided approval is not an edge case here: squash
  and rebase merge are both disabled, force-push is denied to every agent, and reverting the base is not
  an operation, so *every* conflict-resolution tick ends in this state.
- Options: poke from `factory.sh`'s `awaiting_review` arm when `reviewDecision` reads `REVIEW_REQUIRED` | count only surviving verdicts in `triage.sh` and reuse `needs_reviewer` | add a `review_voided` class with its own arm and rank | leave the classifier alone and shorten `FACTORY_REVIEW_POKES` | widen the poke from one reviewer to the team while here (`#241`)
- Chose: count only surviving verdicts in `triage.sh` and reuse `needs_reviewer`
- Why: the handler-side poke is a second, weaker opinion in the path of a reviewer's verdict, which is
  the shape `factory.sh` already refuses for the same reason in `review_unreadable`'s arm. Weaker in the
  literal sense: by the time a row reaches `factory.sh` the reason has been flattened into a detail
  string, so that arm would have to re-parse prose to tell a dismissal from a comment. The class is the
  vocabulary this harness works in -- `docs/factory.md`'s table is the interface a person reads,
  `factory_class_rank` orders on it, and a class that reaches the `*` arm halts the whole pass with
  `UNKNOWN-CLASS`. `review_voided` is the same information with a new name: it costs a row in the table,
  a rank, and an arm, and buys nothing the detail does not already say, because the journal prints the
  detail -- `every verdict voided by a push (hermesvicowa at c149fe9); nobody holds a request` is the
  line a human reads, and it names which of the three no-verdict states the row reached its answer from.
  Shortening the poke floor misdiagnoses the throttle: `grep 'POKE-SKIPPED pr:269'` over the journal on
  the box that runs the timer answers nothing, because a `WAIT` row never consults the throttle -- the
  class that asks was never emitted, so a tighter floor only re-notices a silence faster. `#241` is a
  different question, *who* is asked rather than *whether*, and stays open; the detail is written so that
  widening has one place to change. What separates the two surviving options is tie-break 3, refusing
  rather than proceeding on input that says nothing: a `DISMISSED` row is still a row, and the count
  proceeded on it. The verdict question is `APPROVED` or `CHANGES_REQUESTED` -- somebody formed an
  opinion about the code -- and neither `DISMISSED` nor `COMMENTED` is one. `COMMENTED` was swallowed by
  the same count and `reviewDecision` reads `REVIEW_REQUIRED` over it exactly as over a dismissal, so the
  widening arrives with the fix rather than as a later patch, and its cost is stated rather than buried:
  a pull request whose reviewer commented without approving is now re-poked, at most once per
  `FACTORY_REVIEW_POKES`, where it used to wait quietly. A comment does not satisfy a required review, so
  asking is the honest reading of that state. Ambiguity resolves toward asking: an unreadable `/reviews`
  payload leaves `verdicts=0` and lands in the ask, which costs one throttled `gh` call, while the other
  direction is the silence every arm above exists to prevent -- the same direction `#873` fixed for the
  daemon's config.
  - Evidence, in both directions and in the same tree: with the thirteen legs in place and `triage.sh`
    untouched, `bash factory/selftest.sh` answers `373 passed, 8 failed` -- the eight are the two voided
    and comment-only states at the classifier, their four detail pins, and the two journal lines at the
    supervisor -- and `381 passed, 0 failed` with the fix in. Thirteen legs, counted off the tally (368
    before, 381 after) rather than off a printed column, which is `H-0089`'s lesson. Five were green
    before the fix because they are the polarities the fix must not break: voided-but-somebody-holds-the-
    request stays a wait, the never-reviewed sentence three `#212` legs pin arrives byte-identical, a
    standing `APPROVED` is not swept into the new arm, and the document pair carries its four clauses.
  - The fixture could not have been written this way before, and that is a finding: `/requested_reviewers`
    was created empty and nothing ever wrote it, so "somebody already holds the request" -- the polarity
    that separates a re-ask from noise at a reviewer who is already looking -- had no plant at all.
    `claim_reviewers` is added for it. `voided_pr` now also refuses a plant that is not JSON, because the
    first draft of the `COMMENTED` plant was a jq object literal: `reviews.json` held bytes `jq` could not
    parse, `any_review` fell back to `0`, and that leg read GREEN off the never-reviewed arm with the
    detail `no review and nobody asked since it opened`. A leg whose fixture does not parse measures the
    empty state and reports the state it asked about.
- Checkpoint: 7e30cb3
- Effect: `factory/triage.sh` -- two reads beside `any_review` (`verdicts`, `voided`) and three details in
  the arm; the class is unchanged, so `factory.sh`, `factory_class_rank` and the poke needed nothing.
  `factory/selftest.sh` -- the `#272` group between the `#242` ask legs and the `delivered` arm,
  `claim_reviewers` beside `claim_reviews`, `claim_reset` clearing it. `docs/factory.md` -- the
  `needs_reviewer` row reads *no verdict that still stands*, the `awaiting_review` row says what a wait
  now means, and a subsection "The approval a push voids" carries the two read-backs, the two journal
  lines, the arithmetic of the silence that follows a dismissal (`FACTORY_INTERVAL`, then the
  `FACTORY_REVIEW_POKES` floor), and what is not measured. `.opencode/skills/factory/SKILL.md` -- one
  clause in the Step 5 subsection, so the tick's body says *not yet* rather than *never*; the leg
  `the voided verdict is in both documents` reads SKILL's read and cadence against the guide's rule and
  reason, with a control whose two copies each lose one clause from each document.
  - What lost: the re-ask is not instant -- the next activation, ten minutes at the shipped interval or the
    rest of the 30-minute poke floor -- and the pull-request body has to carry that gap until then, which
    is `H-0095`'s half. Whether GitHub accepts `POST …/requested_reviewers` for a reviewer whose verdict
    was just voided is **not measured**: `#269` was re-approved by hand at 19:50:20Z before any activation
    could ask, so no such call exists anywhere. The legs assert the dispatch (`WOULD-POKE pr:269` in a dry
    run) and not an acceptance; if the call is refused the arm logs `POKE-FAILED` and stamps the throttle,
    which is a journal line naming a refusal in place of the silence, but still no ask. The poke asks one
    reviewer (`FACTORY_REVIEWER`); `#241` owns the widening. And `mergeStateStatus` was `UNKNOWN` on both
    the approved and the dismissed reads, so a merge is still only ever decided by a fresh read
    (`H-0095`).
  - This entry also corrects `H-0095`'s two forward citations, annotated in that entry rather than edited
    quietly: the merge `7e44e8e` brought `main`'s `## H-0096` after `H-0095` was committed, and an entry
    that changes what a sibling refers to says so in the log. The checkpoint keeps its name,
    `chore(checkpoint): pre-H-0096`, because retaking it after the merge would record the state this
    decision produced -- the thing the log already had -- and `H-0090` settled that a checkpoint is the
    tree the allocator was asked about, not the number the entry ended up with.
  - A previous tick reached the same rule from the same measurements and is parked, unmerged, in the
    supervisor's own quarantine. The activation that opened this tick found the tree dirty and wrote, in
    `/home/rodney/prj/parental-lockout/factory/log/supervisor.log`:

    ```
    2026-11-10T20:18:23Z TREE-QUARANTINED unclaimed:272 modified: docs/factory.md factory/selftest.sh factory/triage.sh  attempt=1 stash=d67d04599 mirror=/home/rodney/prj/at-risk/issue-272-quarantine-20261110T201823Z-d67d04599.patch
    ```

    That stash is 261 lines over the same three files -- `git diff d67d0459^ d67d0459` and the mirror patch
    each hold it whole -- written on top of `8ac3eed` and never committed, so it is in no queue any sweep
    reads. It is named here and in a comment on the pull request (`#273`, posted 20:32:31Z) rather than
    left in `git stash list`, because the failure this repository has written up most often is work that
    exists only where nobody looks: `AGENTS.md` calls the branch-shaped case `branch_without_pr` and this is
    the same defect with a filename on it. Whoever reads it next should read the bullet below first.
  - The one judgement the parked attempt and this entry do not share, recorded rather than quietly won. It
    counts a verdict as anything that is not `DISMISSED`, so a `COMMENTED` row stands and nobody is
    re-asked, and it pins that with a leg named `a comment is a review that stands, so nobody is re-asked`;
    this entry counts `APPROVED` or `CHANGES_REQUESTED`, so comments-only re-asks as well. **No measurement
    here decides it, and none is claimed:** the census is complete rather than sampled --
    `gh pr list --state all --limit 250 --json number,reviewDecision,reviews` answers 150 rows, which is every
    pull request this repository has ever had, 29 of them carry a `COMMENTED` row, and all 29 read
    `reviewDecision=APPROVED` because each also carries an approval. The comments-only-plus-`REVIEW_REQUIRED`
    state has never occurred on this account. What the choice rests on is the asymmetry of cost, and it is
    stated so a reader can weigh it: asking costs one throttled poke per `FACTORY_REVIEW_POKES` and GitHub
    answers 422 when a request is already outstanding, while not asking is the unbounded silence every arm
    above exists to prevent. **One premise of the widening is not measured here and is not presented as if
    it were:** that submitting a comment review clears that reviewer's outstanding request, which is what
    would make the parked reading a wait nobody owes rather than one somebody is still answering. No pull
    request on this account has ever been in that state, so there is nothing to read back -- the claim is
    GitHub's documented behaviour carried in, not a measurement, and the legs pin only what the classifier
    answers. A reviewer who prefers the narrower predicate changes one `select` and one leg; both fixtures
    exist, in two copies now.

## H-0098 · Step 1 hands a tick a `--jq` program that cannot run on the gh the timer executes, and nothing had ever executed the document to find out

- Question: `.opencode/skills/factory/SKILL.md:93` has told every tick to run `gh pr view "$PR" --json reviewRequests --jq '[.users[].login]'`, and that program does not run here: measured 10 Nov against PR 269 on gh 2.46.0 at `/usr/bin/gh` it prints `cannot iterate over: null` and exits 1, because `--json` hands `--jq` the WHOLE payload object -- `{"reviewRequests":[]}` -- while `.users` is the field of the REST `/pulls/N/requested_reviewers` envelope, which is what `factory/factory.sh:268` and `factory/triage.sh:283` read and where the program was copied from. The prose fix is one word. What is the deliverable: the corrected sentence, or the thing that stops the whole class -- a step quoted as prose and never run, drifting from the binary that executes it?
- Options: fix the line and cite the measurement in the pull-request body | fix the line and add a `factory/selftest.sh` leg that runs each quoted `--jq` program against a canned payload derived from that same line's own `--json` list | fix the line and add a leg that asks the live API what a real pull request answers | rewrite the read out of the document altogether and have `factory/triage.sh` print the reviewer list so no tick has to spell a program
- Chose: fix the line and add a `factory/selftest.sh` leg that runs each quoted `--jq` program against a canned payload derived from that same line's own `--json` list
- Why: the first tie-break that separates these is the third -- refuse rather than proceed -- and it separates them cleanly: the leg refuses a field it has no measured fixture for, a `--jq` on a line with no `--json`, and a program it cannot read, where every other option proceeds in silence in all three cases. The fourth tie-break (fewest files) does NOT separate them and is recorded as losing: prose-only touches one file, this touches two. It loses because the ticket's own diagnosis is the mechanism -- the Step 1 commands are quoted prose and nothing runs them, which is how they drift from the `gh` build the timer actually executes -- and a fix that removes one drifted sentence while leaving the mechanism in place is the same repair done once more next time. Canned rather than live is H-0054's standing rule, measured there rather than asserted: 45 legs exec'd a binary the gate does not install and CI answered `131 passed, 45 failed` with `factory: missing on PATH: opencode` while every box in the loop printed green; a leg that calls the API is green wherever a token is in the keyring and red on the runner that has none, which is the same failure one dependency over. `jq` is not a dependency smuggled in beside it -- `claim_graphql()` runs jq unconditionally mid-suite, and shipped `factory/common.sh` calls it at :536, :544 and :618. Deriving the payload from the line's own `--json` list, rather than keying a fixture by the program, is what makes the leg able to say no: the pair (`.users`, `{"users":[…]}`) passes and is still the bug, and H-0055's finding (1) is this repository's measured instance of exactly that -- a shim dispatching on the wrong argument answered empty for every review read, and one leg was green by PINNING THE BUG. Asking the live API (option three) buys a payload that exists only while somebody has an outstanding ask, which is the one row this leg needs and the row a CI runner cannot have. Option four moves the program into `triage.sh`, where the existing legs already cover it, but a tick's Step 1 read is a different question from the supervisor's queue read -- the tick is deciding whether to answer a review NOW, on one pull request -- and it is told to spell two calls because a composed one gets refused by the permission scanner.
- Checkpoint: 47d60eb026264240166e42c1d8a3eaf94448933a
- Effect: `.opencode/skills/factory/SKILL.md` Step 1 -- the program now reads `[.reviewRequests[].login]`, and the paragraph after the block states the rule the program broke (the payload shape, where the wrong one came from, and why the silent `[]` is worse than the error) and names the leg that now runs it; `factory/selftest.sh` -- `skill_jq_fixture`, `skill_jq_payload`, `skill_jq_lines`, `skill_jq_gaps`, `skill_jq_control`, `skill_jq_leg`, and a fourth row in the header's list of checks-about-shipped-text, 368 -> 369 legs (measured on this tree: `369 passed, 0 failed`, and `368 passed, 1 failed` with the program back-revolved in the document, exit 1, restored by `cp` and byte-compared). Three findings came out of writing the legs rather than out of the design, and all three are load-bearing. **(1) A control caught what the arm could not.** The plant that puts double quotes around the program read CLEAN on its first run: `--jq "[.a[].b]"` taken as a bare token hands the engine the four-quote-wrapped `"[.a[].b]"`, and jq reads that as a string LITERAL -- it prints the text, exits 0, and looks answered. gh would have run the program, because bash strips those quotes before gh sees them; the leg's engine does not, because the leg extracted the quotes as part of the token. The leading character now decides before the bare form is tried, and a bare token holding `$` is refused for the same reason. **(2) A plant that matches nothing has the same exit status as an arm that is blind.** A document variant that writes the program bare makes all three plants no-ops; they reported CLEAN, in the same words an arm's failure uses. A `cmp -s` against the original now runs before a plant's verdict counts, and a plant that changed nothing is its own FAIL -- "the wording moved; move the plant with it", which is H-0055's finding (3) arriving a fourth time in this repository. **(3) `local file=$1 shown=${file##*/}` aborts the suite.** bash expands every word of a `local` before assigning any of them, so under `set -u` the second reads `file` while it is still unset: `line 76: file: unbound variable`, and a run that dies before its tally is the failure this file's own trap exists to announce. Two engine facts were measured rather than assumed, because the leg asserts through real `jq` while the document is executed by gh's gojq: for this class both fail the same way (`jq: error: Cannot iterate over null (null)` at exit 5 where gh answers `cannot iterate over: null` at exit 1), so the leg asserts a nonzero status and never the sentence; and `jq -r` reproduces gh's printer on both shapes a program can return (`["hermesvicowa"]` for an array, `main` for a bare string) where `jq -c` does not match the string case. The fixture values are a live read of this repository on 10 Nov, not shapes guessed from documentation: PR 273 answered `{"reviewRequests":[{"__typename":"User","login":"hermesvicowa"}]}` and `[.reviewRequests[].login]` returns `["hermesvicowa"]` at exit 0 against it, PR 269 answers `{"reviewRequests":[]}` and the same program returns `[]` at exit 0, and the shipped program exits 1 against both. PR 264, which the ticket cited for the non-empty case, had by then gone empty -- the reviewer had been asked and had answered -- so the row shape came from 273 instead, which is the reason the leg is canned: the fact it pins outlives the pull request that happened to exhibit it. What lost, as measured: only `.opencode/skills/factory/SKILL.md` is scanned -- the sibling `issue-check` skill quotes one raw-API read (`--jq '{title, body, labels: [.labels[].name]}'`), correct as written, and NO leg runs it, because a raw-API read has no `--json` list to derive a payload from and a hand-keyed payload for it is exactly the invented-to-fit fixture this leg exists to forbid; widening the scan is a rule about API paths, not a path to add to a list. The shipped `[.users[].login]` uses at `factory/factory.sh:268` and `factory/triage.sh:283` are not covered and are not wrong -- their payload IS `{"users":[…]}` -- and proving that needs a raw API read, which is denied to a tick by the deny table, so the leg's scope rests on the documented REST envelope plus the fact that the poke path has run since #209; the day gh changes either envelope the leg says nothing about it. Field NAMES are not this leg's subject: line 92 asks for five fields and only gh validates those against the API, which is a live call, so `closingIssuesReferences` -- the documented-sounding field this very document records as answering `Unknown JSON field` -- stays un-pinned. `statusCheckRollup`'s row is the measured all-SUCCESS row, so a future program selecting a failed check trips the empty-shape refusal and a human has to measure and add the red row; the refusal says both readings out loud rather than picking one. The head-field rule is one reference deep: `.reviewRequests[].isOutstanding` is caught when it yields nothing (verified: `[.reviewRequests[].isOutstanding // empty]` exits 0 and prints NOTHING, which only the empty-shape arm sees) and a wrong leaf that yields a value, `.reviewRequests[].__typename`, is green. And the leg pins the programs a document QUOTES, not the ones a tick improvises -- 45 minutes of ceiling went on 5 Nov over a composed read the permission scanner could not parse, and no leg can see a command nobody wrote down.

## H-0099 · One `qrc=1` carries three histories, and the healthy one was journalled as a rescue (#267)

- Question: `factory_tree_stop` calls `factory_tree_quarantine` before every tick with no dirty test in
  front of it, and that function answers `1` for "nothing to stash" -- the same answer for "the verdict
  said dirty and the action found nothing" and for "nothing was ever dirty". One sentence covered both,
  so the ordinary path journalled a rescue. Measured on the live journal at the time of writing, read
  read-only out of `/home/rodney/prj/parental-lockout/factory/log/supervisor.log`: **19** lines carrying
  `was found dirty and is clean now`, every one of them dated 10 Nov, against **0** lines matching
  `TREE-DIRTY|SUPERVISOR-AT.*dirty=1` and **3** `TREE-QUARANTINED` events (`unclaimed:263`, `:267`,
  `:272`) -- three, not the four #267 reports; nothing here turns on which, and the count is quoted
  because the ratio is the whole finding. Line 332 is the cleanest of the nineteen: `21:14:17Z
  TREE-CLEAN unclaimed:267 /home/rodney/prj/wt/issue-267`, and the journal's own next line for that tree
  is `TICK-START`; the first dirt the file records for unit 267 is `TREE-AFTER-TICK … streak=armed the
  tick left modified: factory/common.sh factory/selftest.sh` eight minutes later, produced BY that tick.
  So the sentence was written about a tree with nothing in it, in the log #237 tells an operator to read.
  What has to be decided is what separates the histories and what each arm is allowed to claim.
- Options: split on `[ -z "$dirt" ]`, two arms (the fix direction #267 itself names) | split on the
  verdict's exit code, three arms | journal nothing on the healthy path and let the disagreement arm
  quote `$dirt` verbatim | reword the one sentence so it claims less | keep it and append a path count
- Chose: split on the verdict's exit code, three arms
- Why: the arm is named in the journal as `cause=never-dirty`, `cause=verdict-disagreed` and
  `cause=verdict-unreadable`, the shipped disagreement sentence is kept verbatim for the one state it is
  true of, and the finding is named inside it.
  **`$?` is the discriminator, not `$dirt`.** Two states break the ticket's own test. (a) An rc-0
  verdict can return no paths, and `[ -z "$dirt" ]` calls that "never dirty" -- the fallback
  `${dirt:-no finding returned}` in the success arm three lines away exists precisely because that state
  is expected. (b) The unreadable verdict (rc 2) prints its reason on stdout, so it is NON-empty and
  `[ -z "$dirt" ]` sends it to the DISAGREEMENT arm: "was found dirty", about a verdict that never
  answered. That is a new false line of the exact class being deleted, shipped from inside the fix.
  `factory_tree_dirt_read`'s `its state could not be read` / `return 2` is the producer, and it is now
  both true and legged. **The cheap tie-breaks do not separate the options**: two and three arms touch
  the same two files, none of the five adds a dependency or decodes a key (tie-breaks 1 and 2 are
  silent), all are one `git revert` (4), and 1 and 2 change the same arm (3). Tie-break 3 -- refuse
  rather than proceed on input you cannot read -- is what beats option 1: only the three-arm split says
  nothing untrue about a verdict that failed. Tie-break 6 beats option 3: the loop already journals a
  healthy-path verdict for every tree, `TREE-AFTER-TICK … streak=cleared` (`common.sh:1607`), so silence
  is not what the surrounding code does; and a named arm can be asserted, while silence can only ever be
  asserted as the absence of a string.
  **The ticket's premise was incomplete, and the gap is where the work was.** #267 says the behaviour
  "is asserted nowhere" and cites the print-only note in the process section. It was asserted -- at
  `factory/selftest.sh:1675`, `expect_quarantine`'s `PROCEED` arm, `q_journal_has 'TREE-CLEAN'`, on the
  leg labelled `[primary] nothing was dirty`. The healthy path was pinned TO the disagreement arm, so the
  real blocker was a leg in the unit-level section the ticket never named. Measured: forcing every
  `qrc=1` down the disagreement arm (`case "$dirt_rc" in` -> `case "0" in`) turns
  `[primary] nothing was dirty`, `[worktree] nothing was dirty` and `the clean unit is not credited with
  a rescue` red, exit 1.
  **Arms pinned, not prose.** Each arm carries a `cause=` token -- the house shape for a reason a leg
  must pin (`attempt=`, `streak=`, `mirror=`) -- so the sentences stay rewritable. Five mutants of
  `factory/common.sh`, each planted in the working tree from a `/tmp` copy and restored by `cp`:
  `case "1"` (nothing may claim a disagreement) reddens ONLY `the disagreement arm still fires when it is
  true`, which is the load-bearing result -- the healthy-path legs stay green when the arm is deleted,
  exactly the vacuity the note's own comment described, and exactly what the control exists to refuse;
  `case "0"` (the shipped bug) reddens four legs; replacing the disagreement sentence with the
  never-dirty one reddens only the control; stripping the finding from the disagreement line reddens only
  the control; stripping the reason from the unreadable line reddens the two `verdict unreadable, action
  clean` legs.
  **The last of those five first found the ATTACK broken, not the code.** The reason-check searched the
  journal for `could not be read` -- which the shipped sentence itself contained -- so the leg passed
  with the reason deleted from the line: measured, that mutant exited 0 at `374 passed, 0 failed`. The
  needle is now the verdict's own words, `its state could not be read`, and the sentence says
  `was unreadable` so the two can never be conflated again. It is the same failure every `HALT` leg in
  this file names its arm to avoid, re-imported by a fresh leg and caught by attacking the leg instead
  of reading it.
  **The third arm is reachable, so it is not left as a disclosure.** No on-disk fixture reaches it: the
  state is a difference between two reads of ONE tree, `factory_tree_quarantine` answering 1 solely on
  its own verdict (`common.sh:1366`). `shim_install_first` refuses the first `git update-index --refresh`
  with 128 and words -- the words are what make it UNKNOWN rather than clean since #232, and they are why
  this shim is written rather than calling `shim_install` with exit 1. The helper then failed its own
  first run: the one-shot marker survived the previous install, so the `[worktree]` copy of the leg
  answered `never-dirty` -- 1 failure of 374, reported here rather than fixed quietly, and the helper now
  `rm -rf`s its own directory the way `q_arm` resets the tree.
  What lost. **Silence on the healthy path is a real alternative with a body on disk, and this entry is
  where it is recorded**: the first attempt at #267 is in `FACTORY_AT_RISK` as
  `issue-267-quarantine-20261110T213522Z-efa97d027.patch` (stash `efa97d027`), quarantined by the
  supervisor at 21:35:22Z when that tick was found dirty and a new one was started into the same tree. It
  reaches the same diagnosis independently, splits the same arm, moves the same `PROCEED` pin, and chose
  to journal nothing on the healthy path -- with an argument this entry owes: the pass is already on the
  record for that tree in the `TICK-START` that follows and the `TREE-AFTER-TICK` after the tick, so the
  line is not the only evidence. Its one design idea this branch had not had is taken, credited in the leg
  that now implements it: ask the SECOND activation as well, where both trees are clean and both units
  have ticked, because that is the pass with the least excuse for a rescue line. What is lost against it
  is the journal volume argument -- this fix writes exactly one `TREE-CLEAN` line per healthy tick as
  before, so nothing about the size of the log improved, only its truth -- and an operator grepping
  `TREE-CLEAN` now finds three causes and must read the token; the search for a real event is
  `cause=verdict-disagreed`. Also lost: the note at `factory/selftest.sh:3057-3076`, whose observation is
  now a `FAIL` instead of a sentence on a green run; and the single-needle `q_journal_has 'TREE-CLEAN'`,
  replaced by two `cause=` needles in the `PROCEED` arm.
  What this commit does NOT do. The 19 lines already in `factory/log/supervisor.log` are left exactly as
  they are -- nothing here rewrites a journal, and the only annotation of them is this entry.
  `docs/factory.md` never documented `TREE-CLEAN` at all (`grep -rn "TREE-CLEAN" docs/` answers 0 lines),
  so there is no page to correct, but there is also still no page telling an operator what these lines
  mean; #237's advice to read them still has nothing behind it, and that is a ticket, not this one.
  The checkpoint named below was taken by the tick that allocated this id, before a word of the above was
  written, and the commit carrying this entry is the one `git revert` points at.
- Effect: `factory/common.sh` `factory_tree_stop` (the verdict's rc kept, one arm split into three) and
  `factory/selftest.sh` (the `PROCEED` pin moved, `UNREADABLE` added to `expect_quarantine`,
  `shim_install_first`/`q_unreadable` added, four legs in the quarantine loop, two in the process section,
  the `proc_did_run` and `proc_costs_its_own_unit_its_turn` comments corrected). `bash factory/selftest.sh`
  -> `374 passed, 0 failed` against `368 passed, 0 failed` on this same box before the change (totals are
  box-dependent and this is not another box's number); `make verify` -> `verify: OK`. No mutant set covers
  shell: across `scripts/mutations/*.json` the 80 distinct `"file"` values are all `.go` or `.tmpl`, so
  `#1048` has no surface here. Three lines read back off the fixtures after the change, not predicted:
  `TREE-CLEAN unclaimed:901 <tree> cause=never-dirty nothing was dirty; nothing was quarantined`,
  `TREE-CLEAN unclaimed:900 <tree> cause=verdict-disagreed was found dirty and is clean now; nothing was
  quarantined (modified: tracked.txt ; untracked: scratch-notes.txt )`, and `TREE-CLEAN selftest <tree>
  cause=verdict-unreadable … (its state could not be read (git update-index --refresh exited 128; git
  said: shim: update-index --refresh said something and exited 128))`.
- Checkpoint: 34663d76fae9bb7f2a4396db33fd093880f02858

## H-0100 · The log collides with the same neighbour a fourth time, and this time both sides' lines are counted instead of eyeballed

- Question: #275 opened `CONFLICTING` through nothing it had done -- `origin/main` moved to
  `d34c85c` (`#273`, carrying `H-0095`, `H-0096`, `H-0097`) while this unit was in flight.
  `factory/DECISIONS.md` is the one conflicted file: both sides appended at the same offset, base
  line 3066. The fourth occurrence of the class `H-0085` settled and `H-0093` and `H-0096`
  re-settled. What `H-0096` established is that the conflict is the cheap half -- `factory/selftest.sh`
  was edited by both sides and git merged it without a word, and a silently retired leg is a green
  suite that no longer checks the thing it was written to check. So the decision is not only which
  order the entries go in, but what counts as proof that the silent half survived.
- Options: take git's conflict layout and let CI notice | renumber `H-0099` above `H-0097` | rebase
  onto `origin/main` | keep both sides' entries ordered by id and count both sides' lines in the
  merged tree
- Chose: keep both sides' entries ordered by id and count both sides' lines in the merged tree
- Why: the ordering is what the allocator already asserted rather than what the merge invented -- it
  refused to hand this branch `H-0095`/`H-0097` because `issue-272` held them in the `refs/heads`
  union, so `H-0097 < H-0099` is the order the ids arrived in. `H-0096`'s reason carries over and is
  not re-litigated: git's own layout is red, because it appends this branch's entry where `main` put
  its own, and renumbering would edit the other side's citations. Rebasing is forbidden on an
  in-flight branch here. **What is new is the proof.** `H-0096` attacked each side's leg in the
  merged tree by hand, one mutant per arm; this time every line either side added is tested
  mechanically, because the failure being guarded is a line that vanished without leaving a conflict
  marker, and a hand-picked arm only finds the arms you thought to pick. Over the four files `main`
  touched, each diffed against the merge base `1d66965` and compared against the merged working tree:
  `factory/selftest.sh` -- main added 218 lines, **0 missing**; this branch added 265, **0 missing**.
  `factory/triage.sh` 39 added, 0 missing, none of mine; `SKILL.md` 41/0; `docs/factory.md` 42/0. No
  `.go` file is in `main`'s change set, so no mutant anchor can have been retired and
  `scripts/check-mutations.py` is not the gate this merge needs -- #1048's surface is `.go` and
  `.tmpl` only, measured across all 80 distinct `"file"` values.
  **The checkpoint below names a sha that is not the other parent, and that is recorded rather than
  quietly fixed.** Its message says `origin/main` moved to `a97ef21`; measured at the merge,
  `git rev-parse origin/main` is `d34c85c2f5478e0419af8c57704541d82d47bbc2`, and `a97ef21` is the tip
  of the *local* `main` in the primary checkout, **54 behind** `origin/main` and 0 ahead
  (`git rev-list --left-right --count main...origin/main` answers `0  54`). The claim was read off
  `git worktree list` -- a command that reports what a working tree has checked out, not what the
  remote holds -- instead of off `git rev-parse origin/main`. The checkpoint still does its job, which
  is to hold the tree state from before the judgement call, and rewriting its message would cost a
  force-push on an in-flight branch, which this repository forbids; so the correction lives here,
  dated, which is what separates a corrected record from one that was always right. The lesson is the
  same one #1180 keeps teaching in a new costume: the read that answers "where is the remote" is
  `git rev-parse origin/main`, and `git worktree list` is not it.
  What lost. Neither side's entry moved, so no citation broke; what lost is the empty checkpoint
  commit itself -- a real commit on the branch, standing for a tree state nothing can rebuild -- and
  whatever linearity a rebase would have bought, which is not on the table here.
- Effect: `factory/DECISIONS.md` is the only file this merge resolved by hand; `factory/selftest.sh`
  took git's auto-merge and was then counted, not read. After the merge the log reads `H-0095`,
  `H-0096`, `H-0097`, `H-0099` in id order, and `python3 scripts/check-decisions.py` answers
  `check-decisions: 135 entries across 2 log(s)` -- 132 on this branch before the merge, plus the two
  `main` brought and this entry. The merged tree then ran the whole gate: `make verify` ->
  `verify: OK`, exit 0, with `factory/selftest.sh: 387 passed, 0 failed` against the 374 this branch
  reported before the merge. The delta is not attributed to a count of legs -- `grep -c 'proc_leg '`
  reads 13 in this branch's file, 11 in `main`'s and 13 in the merged one, because `main`'s new legs
  are not `proc_leg` rows, so a total is the only honest number here. The per-line test above is the
  evidence that nothing stopped running: every line either side added is present in the merged tree.
- Checkpoint: 3e135a78e80039c95041dca376073fb48f575180

## H-0101 · The log collides with `H-0097` a fourth time, and neither side's green counts until a leg from each has been attacked on the merged tree

- Question: `#274` went `CONFLICTING` / `DIRTY` against `main` with all five checks `SUCCESS` and
  `reviewDecision=APPROVED` (read back 10 Nov at head `06e7f8c`, before anything here was touched). The
  base `1d66965` ends the harness log at `H-0096`; `main` appended `H-0095` and `H-0097`, this branch
  appended `H-0098` at the same offset, so `factory/DECISIONS.md` is the one file git could not merge.
  What git said nothing about is the larger half: `.opencode/skills/factory/SKILL.md` and
  `factory/selftest.sh` were edited by BOTH sides and merged without a word -- `H-0097`'s side added
  thirteen legs to the same file `H-0098` added one leg to -- and `#1048`'s recorded failure mode is an
  edit in one place silently retiring a check written in another, reported as a green suite. What is the
  deliverable: the splice that clears the conflict marker, or the proof that each side's check still
  says no on the merged tree?
- Options: keep both entries and push, `make verify` runs afterwards anyway | keep both entries ordered by id and re-measure every gate on the merged tree, quoting the merged numbers | keep both entries ordered by id, re-measure the gates, and attack one leg from EACH side on the merged tree with a unique-occurrence anchor | resolve `factory/DECISIONS.md` with `--ours` and let this branch's copy of the file win
- Chose: keep both entries ordered by id, re-measure the gates, and attack one leg from EACH side on the merged tree with a unique-occurrence anchor
- Why: the first two tie-breaks have nothing to bite on -- no `require` line is at stake (#856), and no
  config key decodes toward or away from a lockout (#873, #872) -- and the fourth (fewest files) does
  not separate the first three either, because in all three the merge itself moves the same five files.
  The third tie-break separates them, and cleanly: the chosen option refuses at three points where the
  others proceed on assumption. The resolver exits 2 unless the conflict is exactly one region holding
  exactly one `## ` heading per side; each plant exits 2 unless its anchor occurs exactly once in the
  file it is aimed at; and a plant that fails to turn a leg red exits 3 rather than being reported as a
  pass, including when the tree does not restore byte-identical. Option four is not a weaker choice but
  a destructive one: `--ours` deletes the 184 log lines `main` appended and the `triage.sh` classifier
  `H-0097` exists to pin, and it loses on that alone. The sixth tie-break was consulted before any of
  this was written and it names `H-0096`, whose merge commit attacked each side's leg on the merged tree
  under exactly this guard -- so this entry follows the practice the file already keeps instead of
  inventing a lighter rule for a fourth collision of the same shape.
- Checkpoint: 50f1b1ca77edf7ddd308ef12a6d877f9db70ca64
- Effect: this merge commit, `origin/main` (`d34c85c`) into `issue-270`. The rule is `H-0085`'s and
  `H-0093`'s and `H-0096`'s unchanged -- keep both, order by id -- and no id moved on either side.
  Six findings, all measured on the merged tree.
  - **The splice is a function of the bytes, not of the session.** It was applied twice, from two
    independent `git merge --no-commit --no-ff origin/main` attempts, once by hand and once by the
    resolver (`/tmp/resolve-274-log.py`, one region / one heading per side or it refuses); the second
    run `cmp`-identical to the first. The first hand attempt got the order backwards (`H-0098` above
    `H-0097`) and `check-decisions.py` would have caught it -- it is the resolver's comparison that was
    wrong, which is the reason the ordering rule is written down in code here and not trusted to the
    person doing the splice.
  - **The union is exactly additive, which is the evidence that neither side lost a line.** Per file,
    `numstat` against the base: ours + theirs = merged -- `SKILL.md` 12/1 + 51/0 = 63/1,
    `factory/selftest.sh` 256/3 + 225/0 = 481/3, `factory/DECISIONS.md` 9/0 + 184/0 = 193/0,
    `docs/factory.md` 0/0 + 50/2 = 50/2, `factory/triage.sh` 0/0 + 39/3 = 39/3. Against each parent the
    other side's text is untouched: merged vs `HEAD` is `184 0` in the log, merged vs `origin/main` is
    `9 0`. That is stronger than the 0-deletion diff `H-0096` leaned on, because a duplicated-then-
    collapsed hunk would show 0 deletions and still lose a side.
  - **Gates, resolution alone:** `python3 scripts/check-decisions.py` -> `134 entries across 2 log(s)`,
    exit 0 (135 with this entry); `python3 scripts/check-decisions.py --self-test` -> 32 must-fail, 9
    must-pass, exit 0; `bash factory/selftest.sh` -> `382 passed, 0 failed`, exit 0. The tally is itself
    a union test: 368 at the base, +13 from `main` (the count `H-0097` takes off the tally), +1 from
    this branch (`H-0098`) -> 382. Had either side's insertion displaced the other's in the same file,
    the number would be short and nothing else would have said so.
  - **Both sides' legs still discriminate, attacked one at a time.** Plant `--jq '[.users[].login]'`
    back into `.opencode/skills/factory/SKILL.md:93` (the program `H-0098` exists to retire) ->
    `381 passed, 1 failed`, exit 1, the failure is `the --jq programs the skill quotes`. Plant
    `"$verdicts"` back to `"$any_review"` in the `needs_reviewer` guard (the predicate `H-0097` exists
    to widen) -> `374 passed, 8 failed`, exit 1 -- the same eight `H-0097` counted on `main`
    (`373 passed, 8 failed` there; 381 - 8 there, 382 - 8 here, and this branch's leg still green beside
    them). Each plant was anchored with a unique-occurrence guard, applied to a `cp` backup and restored
    by `cp` -- never `git checkout`, which would have taken the merge with it -- and each restored
    byte-identical on a sha256 before/after.
  - **`#1048` has no surface in this merge, measured rather than assumed:** no `.go` or `.tmpl` file is
    touched, `NON_GO_TMPL_COUNT=0` across `scripts/mutations/*.json`, and no mutant file names
    `selftest`, `triage`, `DECISIONS` or `SKILL`, so the anchor battery had nothing to retire and was
    not run here (`make verify` ran; `make check-mutations` is a separate target and #1807 keeps it out
    of any tree that is being gated).
  - **The merge moved a citation this branch's own entry makes.** `H-0098`'s `Effect:` names
    `factory/triage.sh:283` for the shipped `[.users[].login]`; `H-0097`'s 39 added lines push that call
    to `factory/triage.sh:308` on the merged tree (`grep -n '\.users\[\]\.login'` -> 308).
    `factory/factory.sh:268` is unmoved, and both remain correct as written -- the payload there is the
    REST envelope, which is the whole point of `H-0098`. This entry is `#1180`'s dated correction beside
    the record, not a rewrite of it.
  - **Two sentences in the log are now stale and neither was edited.** `factory/DECISIONS.md:3178`
    (`H-0097`) says `381 passed, 0 failed` and "counted off the tally (368 before, 381 after)"; on the
    merged tree the tally is 382 and the extra leg is `H-0098`'s. `factory/DECISIONS.md:3259` (`H-0098`)
    says "368 -> 369 legs (measured on this tree: `369 passed, 0 failed`)"; on the merged tree it is 382
    and the other thirteen are `H-0097`'s. Each stays true of the tree it was measured on, and editing
    either one turns the `184 0` / `9 0` above into a deletion -- destroying the only evidence this
    resolution has that neither side lost a line. `H-0096` reached the same conclusion one collision
    ago and for the same reason; the staleness is recorded here instead.
  - **What lost, and what is still not established.** Option one lost the proof and option four lost
    `main`'s work; what the chosen option did not buy is stated rather than left implied. (a) Nothing
    here exercises `H-0097`'s classifier against the live API -- its legs are canned under `H-0054`'s
    rule, so `374 passed, 8 failed` pins the classifier's arithmetic, not GitHub's behaviour. (b)
    `H-0098`'s leg scans only `--jq` inside fenced blocks, and `main`'s new post-push read at
    `.opencode/skills/factory/SKILL.md:336` asks for `--json` with no `--jq`, so it is outside that
    universe by design -- measured: the scan finds exactly one program on `main`, one on this branch and
    one on the merge, so the merge neither widened nor narrowed the leg's reach. (c) The push this merge
    makes is expected to void `#274`'s `APPROVED` row under the rule `H-0095`/`H-0097` just wrote into
    Step 5; whether it did is a read after the push, and the pull request carries the answer, not this
    entry.

## H-0102 · A comment told the next author that `emit()` caps the detail column at 80 columns, and `emit()` has never capped anything

- Question: `factory/triage.sh:444` says "emit() caps the detail at 80 columns, so this is sized for that cap", and `factory/selftest.sh` repeats it. `emit` is a six-field `printf` with no width directive and nothing in either file truncates a field. Does the column grow a cap to match the sentence, or does the sentence go?
- Options: cap the detail at 80 columns inside `emit()`, the first remedy the review offers | delete the clause and say nothing about length | delete the clause and bound the phrase in `CLAIM_RE` so the column cannot grow | delete the clause, state what actually bounds the column, and pin the behaviour with legs
- Chose: delete the clause, state what actually bounds the column, and pin the behaviour with legs
- Why: measured before anything was written, twice over, because the review's premise and my own sentence both needed checking. `emit` at `factory/triage.sh:82` is byte-identical to main's at `:79` -- `printf '%s\037%s\037%s\037%s\037%s\037%s\n'`, no `%s.80`, no `${var:0:80}` -- and a repo-wide grep for a 80-column claim finds only the two sentences that assert it, so the cap is invented, in the file it describes and in the suite that cites it. Capping in `emit()` was refused on what it would also do: the same column carries the *issue title* on every `unclaimed` row, GitHub allows 256 characters of title, main has always printed them whole, and `factory.sh` splits the row on U+001F and logs the detail verbatim -- so truncating to fit one sentence about one class is a behaviour change to every class, bought to repair a comment, and it would need legs of its own for the thing nobody reported. Tie-breaks 4 and 5 both land on the prose anyway. Deleting the clause and stopping there repeats the error sideways, because what replaces it has to be true, and the truth is neither "80" nor "unbounded": the phrase that reaches the detail comes out of `delivered_map`'s `select(($one | length) == 1)`, so it names exactly one DISTINCT issue; `CLAIM_RE` puts `[^#\n]{0,20}` between the verb and that number; and the list tail `(?:[ \t]*[,&/|][ \t]*#[1-9][0-9]*|[ \t]+and[ \t]+#[1-9][0-9]*)*` is unbounded only in a way the sole-claim rule has already disarmed, because a second DISTINCT number fails the `unique_by(.n)` select and the row never prints at all -- so the only body that can lengthen the column is one repeating the same number. Measured both ways rather than argued: the 12d plant, forty `#232`s in one sentence, comes back `delivered` with a detail of 302 columns, tail intact, printed whole (the number taken from a deliberately impossible `claim_detail_wider_than 232 99999` and the leg deleted once it had said 302); and the width leg by itself is a weak pin -- a cap at 200 would clear it while eating the end of the row -- so the third leg asserts `claim_detail_has 232 '(window 100)'` survives 302 columns of phrase. `CLAIM_RE` was not touched, which is tie-break 3 holding: narrowing what counts as a sole claim to tidy a column trades the predicate this class exists for against cosmetics, and the regex's own list tail is load-bearing for #221 and #233 (H-0053).
- Checkpoint: 72c4995ff913dd378ea2fb3285c5c43288af2cb7
- Effect: the comment above the `delivered` row in factory/triage.sh and the one over the live-table legs in factory/selftest.sh, plus `claim_detail_wider_than` and the 12d group -- three legs, 179 -> 182 with `$TMPDIR` on tmpfs on this box (the world that decides the count, not the btrfs worktree -- H-0104). What lost, as measured: the row is compact by choice and the corrected comment says "checkable at a glance", which is a style instruction with no gate behind it -- a future row carrying a paragraph of prose clears every leg here, because what the legs pin is that nothing truncates, not that anything is short; 302 columns is a property of the plant, not of the code, so the figure in both files is an illustration and reads as one, and a body with four thousand repeats would print 30 KiB into the table and into `factory.sh`'s log line; and a pathological body therefore still makes `triage.sh --table` ugly on purpose, refused rather than fixed, because the alternative was shortening the field a person re-checks.


## H-0103 · A queue read that stopped at its own bound answered in the same shape as an empty queue, and one of the two shapes was a false `delivered`

- Question: the queue payload comes from `pullRequests(states:OPEN,first:100)` and `issues(states:OPEN,first:100)`, and the row list from `gh issue list --limit 100`. A `stateReason` for an issue that fell off the end is simply absent, and absent reads as "never reopened", so a truncated page can hand out a `delivered` the full page would have vetoed. Does the bound get asked about, does it get raised, or does the veto stop leaning on the bulk read?
- Options: raise both caps to 1000 and treat the bound as unreachable | ask each connection for `pageInfo{hasNextPage}` and route a `true` into the existing "the queue payload did not answer" refusal, leaving the caps where they are | give the veto a `gh issue view` of its own per open issue so it never depends on the bulk page | ask for the bound on the issue connection only, since that is the one with a veto to lose | ask each connection for `pageInfo{hasNextPage}` and route a `true` into the existing refusal, and additionally count what the open-issue listing returned so its different bound is spoken as well
- Chose: ask each connection for `pageInfo{hasNextPage}` and route a `true` into the existing refusal, and additionally count what the open-issue listing returned so its different bound is spoken as well
- Why: the two bounds fail in opposite directions and only one of them lies. An issue cut off the end of the issue page loses its `stateReason`, and a claim whose veto has vanished prints `delivered` -- the single error this class was built not to make, arriving because the repository has eleven open pull requests today and a hundred and one open issues some other day. A pull request cut off the end hides an OPEN claim, which is the duplicate tick H-0053 was written about. A row past the `gh issue list` bound is quieter still: it gets no row of any class, so nothing about the table says the repository has more work than the table read. Raising the numbers was refused because a bigger number is the same cliff further away and leaves the read just as unable to say it stopped; a per-issue `gh issue view` for the veto was refused because folding `stateReason` into the call triage.sh already makes was the point of H-0053 ("costs no second call"), and one call per open issue every ten minutes is the cheap read traded for the expensive one; the issue-connection-only version was refused because the PR connection carries the duplicate-tick harm and the veto costs the same two lines either way. Tie-break 3 set the reading of an absent field: the jq is `if all(.[]; . == false) then "no" else "yes" end`, so the explicit `false` is the only clearing answer and a `null` -- a renamed field, a connection that is not there -- lands on the refusal side, as does a jq that cannot answer at all, by the `||`. The fixture cannot test any of the query itself: it is not a GraphQL server, it answers the same file whatever is asked, so a leg scans the shipped source for the two occurrences and a control deletes both from a copy and demands the scan say no. Measured where only measurement reaches: the edited query was then run against live GitHub at 16:29Z on 8 Nov -- `./factory/triage.sh --table`, rc 0, empty stderr, three `unclaimed` issue rows -- which is the connection accepting the new selection set and both `hasNextPage` answering `false`, neither of which the fixture can say. Nine legs: issue-truncated refuses and names the bound rather than an empty queue, PR-truncated refuses and leaves the pull-request half its rows (`pr 237 needs_reviewer` still there), the source scan with its control, the 100-row plant warning and still printing its rows, and the same pass at nine rows saying nothing.
- Checkpoint: 0f06a773f8ccc09a740568794ab30450fecb0c11
- Effect: `pageInfo{hasNextPage}` on both connections in factory/triage.sh's queue query, `queue_truncated` and the `elif` arm that refuses the issue half with its own sentence, `issue_limit`/`issue_rows` and the limit warning after the issue loop; `claim_graphql`'s two planted flags with `CLAIM_PAGE_PRS`/`CLAIM_PAGE_ISSUES` cleared by `claim_reset`, `claim_query_asks_for_its_bound`, `claim_issue_rows_at_least`, and the 11b/11c groups -- legs 182 -> 191 with `$TMPDIR` on tmpfs on this box (190 with `$TMPDIR` on btrfs, the H-0045 arm; the world belongs with the number -- H-0104); the bounds paragraph in docs/factory.md. What lost, as measured: the refusal is coarse -- one truncated connection costs the whole issue half for the pass, so a repository at 101 open issues gets no `delivered` row at all until the read is widened, which is the safe direction (every issue keeps its tick) and still a class that stops at a size instead of degrading; `issue_limit` is a constant in the file rather than a `FACTORY_*` knob, so widening it is an edit and not an env var; the counter counts rows READ, not rows emitted, so exactly 100 open issues warns when in fact nothing was missing, and the sentence is worded for that ("at its own limit", not "work is waiting"); and the fixture still cannot notice an edit to the query text, so the leg that guards it names a file and a string rather than a behaviour -- deleting `pageInfo` from the query and forgetting the legs is caught by the scan, deleting the *veto* is caught by 11b, and deleting both at once is caught by nothing except the live run.


## H-0104 · Two correct boxes read two different leg counts for one commit, and the noun both of them named -- "the box" -- was the wrong one

- Question: H-0053's Effect says `legs 120 -> 176`. A re-read counted 119 at `main` and 175 at head on a machine described as btrfs, and attributed the difference to a filesystem rule recorded in H-0048. Every number in that sentence except the attribution is right. What does an absolute leg count have to cite to stay checkable, and does the suite get a way to stop needing the citation?
- Options: edit each entry to name the box it was counted on | edit each to name the filesystem under `$TMPDIR` and the git version | rule that only deltas are citable and leave the absolutes bare | make the suite print its own world on the tally line and correct the four absolutes this branch published to what was actually counted
- Chose: make the suite print its own world on the tally line and correct the four absolutes this branch published to what was actually counted
- Why: the attribution was checked before it was adopted, and it does not resolve -- `grep -n "absolute leg\|leg count\|beside an absolute\|count is the thing someone re-checks"` across `factory/DECISIONS.md`, `docs/factory.md` and `.opencode/skills/factory/SKILL.md` returns no such sentence in H-0048 or anywhere else. The convention that does exist is H-0045, whose own `Why` already carries this exact ±1 measured the same way: "reproduced on one box before choosing, with only `$TMPDIR` moved: `56 passed, 0 failed` on tmpfs, `55 passed, 1 failed`". Naming the wrong rule matters because the correction would then have been made against a rule that does not exist. The cause was reproduced rather than inherited, one variable at a time, on one machine at f60c367: default `/tmp` (tmpfs) is 191 passed 0 failed, `TMPDIR=$HOME/tmp-selftest-h56` (`/dev/nvme0n1p2`, btrfs) is 190 passed 0 failed, and the leg's own line changes with it -- `an exclude file git refused`, `refresh=0 cached=128 diff=0 unmerged=0 others=128` on the first, `an exclude file git swallowed` with all five statuses at 0 on the second, printing the arm's own note "on btrfs a directory reports st_size 0". That is why "the box" is the wrong noun: both boxes are btrfs at the worktree, `factory/selftest.sh:66` builds the fixture root with `mktemp -d "${TMPDIR:-/tmp}/plock-factoryselftest-XXXXXX"`, the exclude leg plants its directory under `$work`, and so the filesystem that answers git's question is the one under `$TMPDIR`, which on this box is not the one the repository is on. The chosen fix attacks the reporting rather than the arithmetic, tie-break 5's kind: a tally that prints its fixture root, filesystem and git makes the citation automatic for every future count instead of a convention people forget under time pressure, and it changes nothing that `make verify` reads -- the Makefile consumes this script's exit status, and no file in the repository parses its stdout (checked against `Makefile:105` and `.github/workflows/ci.yml`). Deltas were left alone because both ends of one shift together, which is why `+56` was never the thing in dispute. The four absolutes this branch published (H-0053's `120 -> 176`, H-0054's `176 -> 179`, H-0102's `179 -> 182`, H-0103's `182 -> 191`) were corrected in place to say `$TMPDIR` on tmpfs, and H-0054's and H-0102's parentheticals were wrong twice over -- they named the worktree's btrfs as the world when every one of those runs was counted with `$TMPDIR` on tmpfs.
- Checkpoint: 84584d8e4cb02c227f75091cf9c5ced1cf672af4
- Effect: the tally `printf` at the end of factory/selftest.sh now prints `fixture root <path> on <fstype>, git <version>`, with no fallback behind either substitution -- a branch nobody can make fail is a branch nobody has seen run, and a blank field is already the evidence; the four Effect counts above re-attributed in place, with H-0053's carrying the ±1 mechanism and this commit's pair of measurements; and the paragraph at docs/factory.md:304, which said the leg measures "which of the two the box is running on" -- the same wrong noun, in the file a reader trusts -- now names `$TMPDIR`, carries the 191/190 pair, and points at the tally line. No leg, deliberately: the line is a report, not a verdict, and the leg that would pin it would have to run the suite inside itself.
- What lost, as measured: the tally names the filesystem and the git but not the arm the exclude leg took, so 190 and 191 remain two numbers for one commit and the reason for the difference sits one line up in the log rather than on the tally line itself; nothing checks that an entry quoting a count names its world, so the rule here is prose with one automatic instance behind it, and a fifth entry that copies an old count without re-running repeats this exactly -- the older ~16 tallies across the other 49 entries were left as written, because each is a measurement of the session that wrote it and rewriting twenty entries to add a caveat each would bury the change this entry is attached to; and the printed filesystem is the one under `$work` at the moment the tally runs, so a run that moved `$TMPDIR` mid-pass -- nothing does today -- would name the wrong one.


## H-0105 · Two ids this branch took are taken on `main` for two other decisions, and the third sits in a gap `main`'s own allocator calls permanent

- Question: this branch published `H-0055`, `H-0056` and `H-0057` across `2462994..afeb148` on 8 Nov, on a branch that was never pushed. `origin/main` now carries `H-0055` and `H-0056` as two unrelated decisions, and `H-0057` is absent from `main` while `origin/main:factory/next-decision-id.sh:14` names it "a permanent gap created by a documented renumbering". The base sync #235's handoff asks for would put `H-0055` and `H-0056` in one log twice. Whose numbers move, and does the one id nothing else holds move with them?
- Options: renumber this branch's three entries to `H-0102`/`H-0103`/`H-0104`, the first ids free on every ref | renumber only the two that collide and keep `H-0057`, interleaving it between `main`'s `H-0056` and `H-0058` | renumber `main`'s two instead, whose prose is the younger of the two | keep both pairs and let the merged `main` fail `check-decisions.py` | fix the allocator to hand out per-branch ranges and renumber nothing here
- Chose: renumber this branch's three entries to `H-0102`/`H-0103`/`H-0104`, the first ids free on every ref
- Why: measured before anything was edited, because a collision is visible only across refs and never inside one checkout. `for r in $(git for-each-ref --format='%(refname)' refs/heads refs/remotes); do git show "$r:factory/DECISIONS.md" 2>/dev/null | grep -q '^## H-0055 ' && printf '%s ' "$r"; done` lists thirteen refs holding `H-0055` -- `main`, `origin/main`, `origin/HEAD`, `feat/factory-harness`, `issue-241`, `issue-247`, `issue-249`, `issue-253`, `issue-270`, `issue-272`, `origin/issue-270`, `backup-tip`, `backup/issue-249-session` -- and the same thirteen hold `H-0056`; `H-0057` is held by `issue-235` alone. The headings are not one decision under two numbers: `main`'s read `A dispatch carried the review's timestamp and not its text, so a tick was sent to answer a review whose body had never arrived` and `The supervisor runs whatever is checked out in its worktree, and a tree 121 commits behind the trunk passes every check in the repository`. What the merged file would be told is read off the shipped gate, not argued: `scripts/check-decisions.py:129`, `id %s is already used at %s:%d; 'revert to before %s' would name two commits`, and the ordering rule six lines later in the same loop, which refuses the naive append as well -- a block of `H-0055`/`H-0056`/`H-0057` left after `main`'s `H-0100` is `out of order (H-0100 came before it)`. This branch moves because its three entries live in commits that exist on no remote: `git ls-remote origin refs/heads/issue-235` prints nothing, so the renumber is text in one file and costs no force-push, which is the one moment `H-0061` and `H-0065` both name as free. `main`'s two stay because they are merged history, and moving merged history is a rewrite `AGENTS.md` bars and a force-push this agent is denied. Why `H-0057` goes with them although nothing else holds it, when tie-break 6 points the other way -- `H-0065` kept the one id its branch alone held (`H-0058`): two things outrank that precedent. It is not merely free on `main`, it is documented closed -- `origin/main:factory/next-decision-id.sh:14` refuses a smallest-gap allocator precisely because one "would recycle those numbers and turn every historical citation into an alias", and `H-0057` is its own example -- so taking it now is that recycle from the other side, and it turns a shipped sentence false the moment this merges. And placement: `H-0057 < H-0100`, so a kept `H-0057` must be threaded between `main`'s `H-0056` and `H-0058`, i.e. a hand edit inside the other side's 45-entry block, which is exactly where `H-0096` and `H-0100` record lines vanishing without leaving a conflict marker; moving all three keeps this branch's block contiguous above `main`'s, so the sync is one insert with nothing edited inside the other side's prose. Tie-break 3 set the reading of a documented refusal (a closed gap is not an available slot) and tie-break 5 is satisfied either way, because a text edit in one commit is reversible by `git revert` and no commit moved.
- Checkpoint: f6b30292c789cb3a05248e3075093d834b5c82a9
- Effect: `factory/DECISIONS.md` (the three headings `H-0055` → `H-0102`, `H-0056` → `H-0103`, `H-0057` → `H-0104`, plus six citations inside it) and `docs/factory.md:310` (one citation) -- nine lines in two files, `git diff --numstat` reading 8 and 1, every changed line an id and nothing else. The citations that moved are one in `H-0053`'s `Effect:`, one in `H-0054`'s `Effect:`, one in this branch's own `H-0102` `Effect:`, one in `H-0103`'s `Effect:` and three in `H-0104`'s `Why:`, plus the `docs` one. Two of those nine lines are inside `H-0053` and `H-0054`, which are merged entries: this edits their prose forward, it does not rewrite their commits, and this entry is the dated note `#1180` asks for -- without them the citations would point at a gap `main` says is permanent, the broken reference `H-0096` refused to create. No code and no leg: nothing that runs reads a decision id. Read back rather than asserted, four ways: `grep -cE '^## H-005[567] ' factory/DECISIONS.md` is `0` (no heading keeps an old id), `grep -cE 'H-005[567]' docs/factory.md` is `0`, `git grep -lE 'H-005[567]' -- . ':!factory/DECISIONS.md'` names no file, and all 28 surviving mentions in `factory/DECISIONS.md` are inside this record -- `awk '/^## H-0105 /,0' factory/DECISIONS.md | grep -oE 'H-005[567]' | wc -l` is 28, the same number -- so the old ids now exist only as the mapping this entry exists to be. `python3 scripts/check-decisions.py` answers `80 entries across 2 log(s)`, exit 0, where the same command printed `79` one line above. **The allocator's own path is not in this branch** -- `factory/next-decision-id.sh` landed upstream after the merge base `358fad3` -- so the read was taken from `origin/main`'s copy run against this repository: it printed `H-0102` before the move and `H-0105` for this record immediately before this commit, both above every id on every ref (the ceiling is `H-0101`, on `issue-270` and `origin/issue-270`). What lost, as measured: the three commits `2462994`, `f60c367`, `afeb148` keep `Decision: H-0055`, `Decision: H-0056`, `Decision: H-0057` in their messages and are not rewritten -- a rewrite is a force-push here -- so once this merges, two of those three trailers resolve on `main` to someone else's decision and the third resolves to a gap: following a trailer arrives at the wrong answer twice and at nothing once, `git log --grep='Decision: H-0102'` finds this rename rather than the work, and the mapping lives in this entry alone. The log also keeps a permanent hole where this branch's numbering ran, which is what the log's own note at `factory/DECISIONS.md:1042` ("Nothing was renumbered out of existence; the numbers moved") means in practice: anyone counting headings against the highest id reads three as missing. No leg pins any of it, for `H-0061`'s reason -- a collision leg would have to read sibling refs, a network read inside a pass the harness deliberately keeps off the network. And the allocator is untouched, so the next branch cut from a base older than `H-0101` can take `H-0102` alongside this one: this is the fourth renumbering over the same wound after `H-0061`, `H-0065` and `H-0068`, and it is a notice, not a lock. The base sync itself is the NEXT commit and is deliberately not claimed here, because an `Effect:` naming a merge that has not run is the prophecy this log has been corrected for twice already.


## H-0106 · The branch sat 121 commits behind a `main` that had already fixed the same sentence twice, and the merge that lost no line still ran 12 legs red

- Question: this branch carries six commits and no pull request, and `origin/main` moved from `358fad3` to `5cfda8d` -- 121 commits -- while it sat. `#235`'s handoff comment ends "Rebase, then gate"; the harness bars rewriting an in-flight branch, and `factory/selftest.sh` and `factory/DECISIONS.md` are the two files where a line can vanish with no conflict marker (`H-0096`). Which sync, and what counts as proof that the silent half survived?
- Options: rebase onto `origin/main`, as `#235`'s handoff comment asks | merge `origin/main` and trust the automatic merge | merge `origin/main`, then count both sides' added lines in the merged tree as `H-0100` does, and run the suite | take `main`'s side of all three conflicts and re-apply only this branch's legs | merge, and clear the red legs by loosening the pagination refusal so `main`'s fixture answers again
- Chose: merge `origin/main`, then count both sides' added lines in the merged tree as `H-0100` does, and run the suite
- Why: the rebase was available and refused. `git ls-remote origin refs/heads/issue-235` prints nothing, so no force-push is needed to rewrite these six commits -- what refused it is the rule that decided `H-0096` and `H-0105` one entry apart: `2462994`, `f60c367` and `afeb148` are cited by sha in `#235`'s own handoff comment and inside `H-0104`'s text ("measured at `f60c367` ... 191 passed 0 failed"), and a rebase mints three new shas for them, so the reader who follows a citation lands on a commit that no longer exists. A merge keeps both those shas and `main`'s 121. Tie-break 5: a merge is undone by `git revert -m 1`; a six-commit rewrite held in no remote is undone only from a log entry. This is written down as a deviation from the instruction in the ticket, which is the whole reason `H-0096` gave for choosing a merge over the rebase the ticket asked for too. What collided is three files, one region each. `factory/triage.sh`: both sides had already deleted the false 80-column clause from the same comment independently -- `main` in `34f95f7`, this branch in `2462994` -- so the conflict is two correct rewrites of one sentence, and `main`'s wording is the merged history's own account of its `H-0055`; `main` also grew `emit` to seven fields there, which is why its sentence says `seven %s`. `main`'s comment won and ten lines of prose yielded; every measurement leg survived, which is the distinction that matters, and `main`'s copy of `factory/selftest.sh` still carried the 80-column sentence at `origin/main:factory/selftest.sh:4120` -- that one was never touched on `main` and it now carries the true claim, so the false sentence exists nowhere in the tree. `factory/selftest.sh`: the region is the end of the file, `main`'s new supervisor section plus its bare tally against this branch's tally-with-its-world; resolved as `main`'s section, then this branch's `printf` in `main`'s slot, before `reached_verdict=1`, so `cleanup()`'s incomplete-run trap at `factory/selftest.sh:96` still sees the flag it tests -- and the leg that exercises that trap, `incomplete_reports_itself`, fabricates its own verdict line rather than reading the shipped one, which is why replacing the line cannot blind it. `factory/DECISIONS.md`: one region *inside* `H-0054`'s entry, because both sides edited that entry's `Effect:` line and both then appended at the same offset; resolved as this branch's `Effect:` (it carries `H-0104`'s re-attribution of the `176 -> 179` count, and taking `main`'s line would delete that correction), then `main`'s `H-0055` .. `H-0100`, then this branch's `H-0102` .. `H-0105` -- ascending, which is what the gate's order rule enforces. Then the count, because three auto-merged files carry more risk than the three that conflicted: a per-file presence test over every non-blank line each side added against the merge base, run against the merged tree -- `python3 /tmp/merged-lines-kept.py 358fad3 HEAD origin/main factory/selftest.sh factory/triage.sh docs/factory.md factory/DECISIONS.md`, kept outside the repository as `H-0096` did. `main` added 6623 / 148 / 470 / 2247 and this branch 126 / 55 / 18 / 31 -- the `added` columns are counted against `HEAD`, so the figure for `factory/DECISIONS.md` rises once this commit exists and carries this entry; `missing` is the column that carries the verdict and it does not move -- and the only gaps are 6 lines against `main` and 7 against this branch, both in `factory/selftest.sh`, plus the ten yielded comment lines in `factory/triage.sh` -- 23 lines, and every one is a line a hunk of this merge rewrote or superseded on purpose: three of `claim_graphql`'s jq program that gained `--argjson` and `pageInfo`, two of the 80-column comment, the supervisor fixture's payload line, this branch's own six-field and 302-column sentences, which the merge made false and this commit corrects, and the ten prose lines. Nothing vanished. What the line count could not see is the finding. `main`'s supervisor fixture planted a GraphQL queue payload with no `pageInfo`, and `H-0103`'s rule reads a missing bound as a page that stopped short, so the refusal took the issue half with it: `PASS units=0`, `IDLE no unit needs a model this pass`, and the eleven dispatch legs of `main`'s new quarantine section red -- `387 passed, 12 failed` on the first run. No line was missing from either side; the two changes were each correct and the composition was not. The fix is in the fake, not the rule (the fifth option was tried in reasoning and rejected: loosening the refusal so a payload without a bound reads as complete deletes `H-0103` and re-opens the false `delivered` it was written for -- a fixture is not a specification). `factory/selftest.sh:2712` now plants `hasNextPage: false` on both connections. The twelfth failure was `the timer's scripts are executable` reading `triage.sh(100755 100755 100755)` -- three stage entries for a file still unmerged in the index -- and it is recorded here so nobody re-reads it as a finding: it disappeared on `git add`.
- Checkpoint: cd25f052367b7453526b322a667d81f5374d92f1
- Effect: the merge of `origin/main` (`5cfda8d`) into `issue-235`, resolved in three files; `factory/selftest.sh:2712`, the supervisor fixture's queue payload, which gained `pageInfo` on both connections with the reason above it; three sentences of this branch's own prose corrected because the merge made them false -- `emit` is no longer countable as six fields (`grep -m1 '^emit()' factory/triage.sh | grep -o '%s' | wc -l` prints `7`), so both comments now assert the property (a bare `%s`, nothing sliced) and say in words why they refuse to name a field count, and the 302-column figure was re-measured rather than trusted: a probe leg `claim_detail_wider_than 232 999999` printed `key 232 detail is 302 columns`, the probe was deleted, and the sentence now names both commits it was measured at. Read back: `bash factory/selftest.sh` prints `factory/selftest.sh: 399 passed, 0 failed  (fixture root /tmp/plock-factoryselftest-<random> on tmpfs, git 2.53.0)` -- 387 from `main` plus this branch's 12, and the fixture root is a fresh `mktemp -d` per run, so only the two counts, the filesystem and the git version re-appear -- where the same command on the same tree an hour earlier printed `387 passed, 12 failed`; `python3 scripts/check-decisions.py` prints `140 entries across 2 log(s)` including this record; `git status --short` lists no `UU`. Two dated corrections about `H-0102`, which was written when `main`'s changes were not in this tree: its `Question:` says "`emit` is a six-field `printf`", true at `358fad3` and false now, and left as written; its `Effect:` claims the `factory/triage.sh` comment as its work, and `main` had made the same correction independently in `34f95f7`, so what survives of `2462994` there is the measurement, not the sentence. A third, about `H-0104`: its `Effect:` promises the gate number "printed by the bare `printf 'factory/selftest.sh: %d passed, %d failed\\n'` line", and this merge is the commit that moves that line into the world-naming one -- so the number the CI read-back quotes now arrives with the parenthetical, exactly as `H-0104`'s `Why:` asked it to. `make verify` has NOT been quoted here: run before this commit it exits 2 with `screenshot manifest: 9 problem(s)`, every one of them `internal/web/static/app.css has an uncommitted change`, which is the mid-merge tree (`git status --short` shows `M  internal/web/static/app.css` and four templates, staged by the merge) meeting `dirty_sources()` -- `H-0104`'s `Effect:` records the same gate refusing a merge in progress and the same order: commit, then gate. The full gate number belongs to the CI read-back on the pull request. What lost: ten lines of comment this branch wrote, to a sentence that says the same thing in the trunk's words -- the legs are the deliverable and the prose was the duplicate; a linear history, which was not on the table without rewriting cited shas; the checkpoint commit `cd25f05`, which now stands for a tree state nothing can rebuild (`H-0100`'s cost, paid again); and `H-0103`'s fixture coupling, which nothing pins -- the planted `pageInfo` at `factory/selftest.sh:2712` is coupled to the shipped query only by this entry, so the next field the queue query asks for does this again, and the leg that would catch it (assert the fixture answers every field the shipped query names) is NOT in this commit and is the `NEXT:` line of this unit's hand-off comment.

## H-0107 · The log collides a fifth time, and the splice is cut from the two parent blobs by line range because ids, not sides, decide the order

- Question: `#274` reads `CONFLICTING` / `DIRTY` against `main` with `reviewDecision=APPROVED` and all five checks `SUCCESS` (`verify`, `shellcheck`, `mutation sets`, `packages (amd64)`, `packages (arm64)`) — read back 10 Nov at head `1d139bb` before anything here was touched, `main` then at `5cfda8d`, merge base `d34c85c`. `factory/DECISIONS.md` is once more the only file git cannot merge: the two parents are byte-identical through line 3251 (the `diff` of those two ranges is empty) and both append after it — `main` with `H-0099` + `H-0100` (165 lines), this branch with `H-0098` + `H-0101` (100). This collision is not the four previous ones, where each side's block could simply be butted against the other: `H-0098` sorts BELOW both of main's entries and `H-0101` above both, so a side-order splice writes the log `H-0099`, `H-0100`, `H-0098`, `H-0101` — an out-of-order log, which this very gate refuses. And git again merged the larger half without a word: `factory/selftest.sh` carries 256 lines this branch added and 271 `main` added, auto-merged silently, which is `#1048`'s recorded shape — a check retired by an edit somewhere else, reported as a green suite. What is the deliverable: the splice that clears the marker, or the splice plus the evidence that neither side's checks were lost inside it?
- Options: take the conflicted file, delete the three marker lines and keep both blocks in side order | splice the two parent BLOBS by line range into id order and re-measure every gate on the merged tree | splice in id order, re-measure every gate, attack one leg from EACH side on the merged tree, and re-open every `file:line` the four colliding entries cite | resolve the file with `--ours`, since this branch is the one being delivered
- Chose: splice in id order, re-measure every gate, attack one leg from EACH side on the merged tree, and re-open every `file:line` the four colliding entries cite
- Why: the first two tie-breaks again have nothing to bite on — no `require` line is at stake (#856), and no config key decodes toward or away from a lockout (#873, #872) — and the fourth (fewest files) does not separate the first three, because all three move the same four files. The third tie-break separates them: it refuses at four points where the others proceed on assumption — the resolver stops unless the conflict is exactly one region whose two sides are whole `## ` entries, the splice is cut from `git show` output rather than from a hand-edited conflicted file so no hunk can be silently dropped, a plant that fails to turn a leg red is reported as a failure rather than as a pass, and a citation that no longer resolves is corrected in this entry rather than left to mislead. Option one is the one that actually loses here, and it loses on arithmetic rather than on taste: side order writes an id-descending log and `scripts/check-decisions.py` has an explicit out-of-order refusal for exactly that, so option one does not produce a mergeable tree at all. Option four is destructive — `--ours` deletes main's 165 log lines together with the three-arm `TREE-CLEAN` split and the 8 legs that pin it — and loses on that alone. Tie-break six was consulted first and names `H-0096`, `H-0100` and `H-0101`, all three of which re-measured on the merged tree rather than trusting the auto-merge; this entry keeps that rule and adds the citation audit because main's 296 new `selftest.sh` lines are exactly the thing that moves a `file:line` a prior entry quotes.
- Checkpoint: 999f9be43e7941c6f7278d88ef58543cd57dee7e
- Effect: `factory/DECISIONS.md` is the whole of the hand-resolution, cut by `sed` line range out of `git show HEAD:` and `git show origin/main:` — common `1..3251`, ours `3252..3260` (`H-0098`), main `3252..3415` (`H-0099`, `H-0100`), one blank, ours `3261..3350` (`H-0101`) — 3515 lines, markers gone, ids ascending `H-0095`…`H-0101`. Against the base that is +265 where the content union is +264: the one extra line is the blank between `H-0100` and `H-0101`, needed because each parent ended its own last entry without a trailing blank and every entry in this log is separated by exactly one. No content line of either parent was rewritten.
  **Additivity, counted not eyeballed.** Per file, lines each side added since `d34c85c` and how many of them are ABSENT from the merged tree: `factory/common.sh` ours 0/0 vs main 40/0; `factory/DECISIONS.md` 100/0 vs 165/0; `factory/selftest.sh` 256/0 vs 271/0; `.opencode/skills/factory/SKILL.md` 12/0 vs 0/0. Nothing missing in eight cells. The deletion side needs a second test, because a merge that reorders also shows deletions: every line the merged tree drops relative to a parent was checked against the lines that parent itself dropped relative to the base, as sets — `selftest.sh` 25 dropped vs HEAD against 25 dropped by main, `common.sh` 5 against 5, `DECISIONS.md` 0 against 0, `SKILL.md` 0 against 0; and from main's side 3/0/1 against ours' 3/0/1. Unexplained in either direction: zero. (`--numstat` says 26 dropped for `selftest.sh` against HEAD while the set says 25: diff counts one duplicated line twice. The set test is the one that answers; the numstat is quoted so the two can be reconciled by a reader.)
  **Both sides attacked on the merged tree.** Baseline `bash factory/selftest.sh` → `388 passed, 0 failed`, exit 0, 15 s. Plant A, this branch's leg: `SKILL.md:93`'s `[.reviewRequests[].login]` back-revolved to the shipped `[.users[].login]` (unique-occurrence anchor, asserted count 1) → `387 passed, 1 failed`, exit 1, failing leg `the --jq programs the skill quotes`. Plant B, main's legs: `common.sh:1649`'s `|| dirt_rc=$?` back to the pre-#267 `|| true` (the unique-occurrence anchor is the whole assignment — bare `|| dirt_rc=$?` occurs three times, at `:1268`, `:1365` and `:1649`, so the short form would have aimed at the wrong two) → `380 passed, 8 failed`, exit 1, and all eight name a `cause=` arm (`[primary]`/`[worktree]` × nothing-was-dirty, verdict-unreadable, inert-shim, plus the two process legs). Both plants restored by `cp` with `sha256sum` before == after, and `git diff` empty afterwards; neither used `git checkout`, which would have taken the merge with it. The tally arithmetic closes: this branch alone was 382, `main`'s tip is 387, the merge is 388 = 387 + this branch's one leg, which puts the base at 381 and main's contribution at 6 legs.
  **`#1048` has no surface here, measured rather than assumed.** All 847 mutant anchors in the thirteen `scripts/mutations/*.json` sets name `.go` (797) or `.tmpl` (50) files; none names anything under `factory/`, `.opencode/` or a `.md`. This merge touches only those three excluded kinds, so no mutant can have been retired by it, and `scripts/check-mutations.py` was therefore not run — it mutates this same working tree in place and must not overlap the gate below (#1807).
  **Citations re-opened.** Every `file:line` quoted by the four colliding entries was read on the merged tree: `SKILL.md:93`, `triage.sh:308` (`H-0101`'s own correction), `common.sh:1366`, `common.sh:1607`, `DECISIONS.md:3178`, `DECISIONS.md:3259` all resolve. Two do not, and they are worth naming because the reason is not drift: `H-0099` cites `factory/selftest.sh:1675` (`expect_quarantine`'s `PROCEED` arm, `q_journal_has 'TREE-CLEAN'`) and `:3057-3076` (the print-only note). Neither is at those lines on `main`'s tip or here — both resolve exactly against the BASE `d34c85c`, which is what the sentence is describing: where the assertion USED to be before the fix moved it. Read as base-relative they are correct (verified: base `:1675` is that `q_journal_has`, base `:3057` opens that note); read as merged-tree references they send a reader to `expect_quarantine`'s `UNREADABLE` arm. Recorded here rather than edited into `H-0099`, for the reason `H-0101` gave: editing either entry turns its `0` deletions above into a deletion and destroys the evidence this entry just spent its ceiling producing.
  **Sentences that go stale on this tree, deliberately not edited:** `:3417` `check-decisions: 135 entries` and `:3419` `387 passed` (`H-0100`, true of the tree it measured), `:3479` `381 passed, 1 failed`, `:3498` and `:3500` (`H-0101`'s own forecast of this exact staleness).
  **Gates, on the tree this commit leaves:** `python3 scripts/check-decisions.py` → `138 entries across 2 log(s)`, exit 0 (137 before this entry), `--self-test` → 32 must-fail / 9 must-pass, exit 0; `python3 scripts/check-denies.py` → exit 0; `bash factory/selftest.sh` → `388 passed, 0 failed`, exit 0; `make verify` → `verify: OK`, exit 0.
  **One live re-measurement, taken on the way in.** The `main` working copy of `.opencode/skills/factory/SKILL.md:93` still carries the program `H-0098` exists to fix — `#274` has not merged — so running the quoted read against `#274` today printed `cannot iterate over: null`, exit 1, and the merged copy's `[.reviewRequests[].login]` on the same call printed `[]`, exit 0. So the reviewer list for `#274` is empty, which is the supervisor's to fill and not this tick's to request.

## H-0108 · A refused merged claim is thrown away, so the ticket that created the verdict is re-dispatched

- Question: `delivered` refuses most of the merged claims it reads -- a body naming more than one
  issue, a body carrying `Part of` -- and then says nothing about the refusal, so the tick dispatched
  for that issue starts from zero. #235 is the measured cost of exactly that: its own fix arrived as
  PR #238, merged 8 Nov on branch `issue-235` under a body naming eight issues (refused by the count
  arm), and the journal holds `TICK-START unclaimed:235 2026-11-10T22:57:23Z` to `TICK-DONE 23:14:23Z`
  -- seventeen minutes of ceiling to rediscover what the classifier had already read. The ticket asks
  which signal reaches the fix; three candidates were on the table, and the one that looked cheapest
  had to be measured before it could be used.
- Options: trust the head-branch name -- a merged PR whose branch is `issue-N` delivers `#N` | a new
  class that dispatches no tick | a new `undelivered:` channel beside `pr:` and `issue:` | read the
  body's prose for a "does not finish" section | carry the refusal on the row as a marker and tell the
  tick what it means
- Chose: carry the refusal on the row as a marker and tell the tick what it means
- Why: the branch name was measured and lost. Over all 98 merged pull requests `gh pr list --state
  merged --limit 100` returns, a branch of the form `issue-N` would claim 15 closed issues (all already
  closed, so nothing *wrong* closes) and 4 open ones: **#246 four times** -- #254/#255/#256/#258 are all
  branch `issue-246` and #246 is open with a requirement nobody has built, so a branch-shaped rule would
  close the very ticket #277 cites as the case that must not be claimed -- plus #251, which #255's body
  names beside #253 while delivering only #253, whose fix is #257. It is also redundant where it is
  right: 66 of those issues are closed already and 53 are named in a verb-bearing body the classifier
  already reads. `headRefName` is carried on the read anyway, as the note's most useful fact -- a
  reader can go and check out the branch -- and is used to decide nothing. The new-class option lost on
  #235's own lesson: the reason `delivered` is a report and not a suppression is that a row which stops
  dispatching retires work, and the refused set is where the unfinished work demonstrably lives (#246
  is refused by a `Part of` that told the truth; #253 was refused by the count arm at #255 and needed
  the tick that row dispatched, which produced #257 five hours later). A separate `undelivered:`
  channel lost because `triage.sh --tsv` prints only `pr:` and `issue:` and `factory.sh` skips the
  `branch:` channel as a side effect of that filter (`docs/factory.md:486-495`) -- a third silent
  channel is how #251 stayed invisible. Prose lost on the rule this file has repeated since H-0050: a
  body's own sentence about its unfinished parts is a person's claim, and #273 proved one paragraph can
  carry six instructions. What the marker does instead is keep the verdict and hand the refusal down:
  `refused_note` puts `[merged:PR#229 on \`issue-199\` at … -- names it in a 4-issue body]` at the START
  of an `unclaimed` detail, `factory.sh`'s `unclaimed` arm matches the shared literal
  `FACTORY_MERGED_MARK` anchored there -- unanchored would let an issue title buy itself an instruction
  -- and appends the instruction that sends the tick to `gh pr diff` before it writes code. Both
  reopens-veto and the timestamp veto suppress it, because in those two cases a person has already read
  the merged body. Half of this moving without the other half is #272, so `merged_note_docs` pins the
  rule and the reason in `docs/factory.md` and `.opencode/skills/factory/SKILL.md` as well.
  What lost. The seventeen minutes are not recoverable, and a tick still runs on a ticket that may be
  finished -- the marker shortens the work, it does not cancel it, which is the price of the option
  that keeps refused work in the queue. The branch name is read and printed but decides nothing, so the
  note is wordier than a branch-only rule would have been. And the marker is one more anchored string
  shared between two shell files, which is a coupling that did not exist before and is pinned by one
  leg rather than by the type system.
  **The checkpoint below is the branch tip, not a `chore(checkpoint): pre-H-0108` commit, and that is a
  lapse rather than a convention.** The rule in `AGENTS.md` and Step 3 of the skill is to take the
  checkpoint before the call; this call was made, and the checkpoint was noticed afterwards. What
  `5cfda8d` does supply is the thing the field is for -- the tree the decision started from, a commit
  that exists and that `git revert` of the effect commit returns to -- so the entry is reversible as
  required, and what is missing is only the empty commit that would have marked the moment. Recorded
  here, dated, because a silently relaxed rule is indistinguishable from one that was never there.
- Effect: `factory/triage.sh` (merged read gains `headRefName`; the `refused_map` scan and
  `refused_note`; the `unclaimed` emit prefixed with the note behind the two vetoes; the header's
  `detail` column and the `--table` example), `factory/common.sh` (`FACTORY_MERGED_MARK`),
  `factory/factory.sh` (the `unclaimed` arm's instruction, plus its class table), `factory/selftest.sh`
  (+30 legs, 417 passed 0 failed against 387 on this same box before the change -- totals are
  box-dependent -- plus two control blocks that print without joining the tally, the shape
  `voided_docs_control` already uses), `docs/factory.md`, `.opencode/skills/factory/SKILL.md`. Read
  back off the live API after the change, not predicted: `./factory/triage.sh --table` prints
  `issue 246 unclaimed [merged:PR#254 on \`issue-246\` at 2026-11-10T01:39:17Z -- a \`Part of\` phrase
  in the same body] TimeoutStopSec reuses the tick ceiling…` -- class unchanged, note present -- and
  `issue 199 delivered merged PR #243 …`, where #199 moved to a sole claim in the hours since the
  probe, so no note. The anchor was tested against a forgery rather than assumed: with the value read
  out of `common.sh`, a detail beginning with the marker hits the arm and a title that merely contains
  `[merged:PR#1]` does not. Two findings came out of writing the legs, both fixed. The fixture shim
  `emit`s a payload file whatever the caller's `--json` asked for, so a read that stopped requesting a
  field would have kept receiving it and every branch-naming leg here would have stayed green -- the
  merged payload is now projected through `with_entries` to the fields actually named, which is what
  `gh` does. And the `jq --slurpfile` in that projection emitted nothing at all until `-n` was added:
  `--slurpfile` binds `$src` and leaves the input empty, so a filter without `-n` runs zero times and
  writes zero bytes with exit 0 -- a silent empty file, the same shape as the `delivered_map=`
  regression `triage.sh:470-479` records, and the reason the new legs assert the note is PRESENT on
  named rows rather than merely absent where it should be. That precedent is the reason the legs exist:
  the scar comment there is `bash -x` showing `delivered_map=` followed by `delivered_map='{}'`, and no
  leg of the day could see an always-empty map.
- Checkpoint: 5cfda8d7781daecab8209d849ac1643044e66844

## H-0109 · The fixture answers fields nobody asked for, so the projection goes into `emit()` and not into another call site

- Question: #279 found both fixture `gh` shims parsing `--json` for one thing only -- telling two
  payloads apart -- and then printing the whole payload file. A shipped read that *stopped* asking for
  a field therefore kept receiving it from the fixture, and every leg asserting something about that
  field stayed green while the read went blind. The issue asked for `H-0108`'s projection lifted out of
  its one branch and paired with a leg that fails when a caller asks for a field the fixture does not
  carry, and it asked for its own plant to be run first. The plant was run first, and it is the whole
  argument: `--json number,title,updatedAt` at `factory/triage.sh:563` reduced to `--json number,title`
  left `bash factory/selftest.sh` at `387 passed, 0 failed` -- identical in tally to the unpunctured
  run, exit 0 -- while the `CLAIM_DEBUG` rows still printed `issue 232 delivered`, a verdict
  `delivered_claim` (triage.sh:509) refuses to render on an empty `updatedAt`. Six payload sites, one
  demonstrated, none of them visible.
- Options: project at each dispatch site, as `H-0108` did for the merged read alone | lift the
  projection into each shim's `emit()` as two separate copies of the rule | one heredoc installed into
  both payload directories and sourced by both `emit()`s, `gh`'s refusal of a field no row carried
  included | leave the fixture alone and pin the field lists with a text check over `triage.sh`
- Chose: one heredoc installed into both payload directories and sourced by both `emit()`s, `gh`'s
  refusal of a field no row carried included
- Why: the first option carries its own refutation, measured in the same week as its fix -- `H-0108`
  projected the merged payload at the merged read, and #279 then counted **six** payload sites still
  served blind, because a rule written at a call site is a rule for that call site. The fourth option
  proves the shipped text says `updatedAt`; it does not prove the read *receives* it, and this class is
  precisely a suite that could not see its own subject (`triage.sh:470-479`'s always-empty
  `delivered_map`, and #242's `${3}` where `${2}` was meant), so a text check is that suite in a
  different costume. Between the middle two the tie-breaks separate them on fewest files and then on
  the reason `H-0108` exists at all: two copies is how one channel got fixed and six did not, and these
  two shims already claim to share every other rule by pointing at each other in comments
  (`CLAIMSHIM`'s write refusal, the process shim's `exit 9`) -- this makes one of those promises
  mechanical instead of aspirational. The refusal is the half the issue named, and it follows the house
  rule for a missing input: refuse rather than proceed. `gh` answers an unknown field with a non-zero
  exit and the field's name, so the fixture does the same, and `labels` -- a field `gh issue list`
  holds for every issue on GitHub and this fixture has never held -- now fails loudly where it used to
  answer no column at all at exit 0, which is indistinguishable from the repository saying "none". That
  is the moment a fixture quietly becomes the specification. `git revert` of the one commit below takes
  all of it back.
  **What lost.** Locality: a reader of one shim no longer sees the projection where the payload is
  printed, and pays a `$here/fields.sh` hop instead -- paid back with a comment at each `emit()` naming
  `install_gh_fields`, which is also where the measurement lives. The honest residue is
  `gh_json_payload`'s empty payload: `gh` checks a field name against its own schema, a fixture has
  only the rows in front of it, and every unlegged read runs against `[]` every pass, so an empty list
  carries no verdict about the field list and is not refused. The check is therefore only as good as a
  planted row, which is why the refusal leg asks the fixture that holds nine. And `H-0108`'s
  site-level branch on `issue-277` becomes redundant beside this one; it is not broken by it --
  re-projecting an already-projected payload to the same field list is the identity -- so whoever
  resolves the shared region is deleting a branch, not correcting one.
- Effect: `factory/selftest.sh` only, plus this entry. Both `emit()`s now answer
  `gh_json_payload <payload> <--json list>`: the document verbatim when no field list was passed (the
  `user` and `graphql` reads, where `gh` has no field contract to obey), otherwise the payload projected
  through `with_entries` with what no row carries refused by name. Eight legs: three that a field not
  named does not come back (the claim-shim, its control, and the same read off the **process** shim,
  which serves the whole supervisor run), two that a field the fixture lacks is refused and one it
  holds is not, and three that run #279's plant end to end -- the mutant's anchor counted before it is
  believed (`proc_mutate`'s rule), then the shipped read one field short must not deliver, then the
  shipped file as it ships must. Measured on rodney-9950x, btrfs, one `bash factory/selftest.sh` per
  row, the same fixture as `H-0100`'s table: unpunctured `395 passed, 0 failed` (was 387; eight legs,
  15.1 s -> 16.0 s); the plant with this fix in `375 passed, 20 failed`, exit 1, against the `0 failed`
  the same plant produced above; and the fix with its projection neutralised in both `emit()`s --
  everything else left in place -- `391 passed, 4 failed`, which is four of the eight saying the old
  answer out loud. `make check-shell` clean, 0.10.0 over 28 scripts. No `.go` file is touched, so no
  mutant anchor moves and `scripts/check-mutations.py` is not the gate this change needs (#1048's
  surface is `.go` and `.tmpl`).
  **Corrected at the merge, 11 Nov, and not quietly.** This entry said the measurement ran on
  `btrfs`; it copied that from #279's table and from `H-0100` instead of reading it. `H-0104`, which
  arrived on `origin/main` in `afeb148` while this entry was being written, is the decision that the
  filesystem which decides a count is the one under `$TMPDIR` holding the *fixture root* -- not the one
  the worktree is on -- and the merged runner now prints it: `407 passed, 0 failed (fixture root
  /tmp/plock-factoryselftest-… on tmpfs, git 2.53.0)`. So the box was right and the filesystem was
  wrong: every count above was taken with `$TMPDIR` at its default, which on this machine is tmpfs.
  What that class costs a count is `H-0045`'s measured ±1 -- one exclude-file leg flips its verdict
  between tmpfs and btrfs -- which is why the numbers above are stated as the plant's tallies and the
  deltas (`387 -> 395`, `0 -> 20 failed`, `4 failed` with the projection neutralised) rather than as a
  bare absolute: both ends of a delta are counted in one world. The counts themselves are unaffected by
  the correction, because none of them is the ±1 leg.
- Checkpoint: 3de3544298fbbc5e1c0ca5b471bbab9542fc5579
## H-0110 · The log collides with the same neighbour a fifth time, and this time it is the shipped gate, not the author, that says which order is right

- Question: `origin/main` moved from `5cfda8d` to `7fa162b` -- eleven commits -- while #278 sat open with five green checks, and `gh pr view 278` reads `mergeable=CONFLICTING`, `mergeStateStatus=DIRTY`. `git merge-tree --write-tree` names exactly one conflicted file, `factory/DECISIONS.md`: both sides appended past `H-0100`'s `Checkpoint:` at base line 3415. Fifth occurrence of the class `H-0085` settled, `H-0093`, `H-0096` and `H-0100` re-settled. What is new is that the ids do NOT collide this time -- this branch contributes `H-0108`, `main` contributes `H-0102`..`H-0106`, disjoint across every ref -- so the only judgement left is the order and the proof, and the proof is the half that has actually bitten: four of the seven files in the change set auto-merge, and an auto-merge retires a leg without ever leaving a marker (`H-0096`, `H-0106`).
- Options: take git's conflict layout and let CI notice | renumber `H-0108` to sit above `main`'s block | rebase onto `origin/main` | keep both sides' entries ordered by id, count every line either side added in the merged tree, and run the suite | merge, and clear any red leg by loosening this branch's `with_entries` projection so the fixture hands over fields the caller never asked for
- Chose: keep both sides' entries ordered by id, count every line either side added in the merged tree, and run the suite
- Why: git's layout is red, and it was measured rather than argued -- the conflict was resolved both ways into `/tmp` and each copy run through the shipped gate. Ours-first answers five problems (`factory/DECISIONS.md:3496: H-0102 is out of order (H-0108 came before it)`, and the same sentence for `H-0103`..`H-0106`), exit 1; id-ordered answers `check-decisions: 102 entries across 1 log(s)`, exit 0. So the order is not taste, it is `scripts/check-decisions.py:161`. Renumbering lost on a number this branch has already published: `gh pr view 278 --json body` carries `H-0108` at body line 36, so a renumber moves an id a reader outside this repository holds -- the exact cost `H-0105` paid only because two ids were genuinely taken, and none is here (measured: no ref but this branch holds a `## H-0108 ` heading, and this branch holds none of `H-0102`..`H-0106`). Rebasing is refused by the remote rather than by rule-of-thumb: `git rev-parse origin/issue-277` answers `b60fd1c`, so the branch is pushed and a rewrite needs `--force` -- denied to this agent by `.opencode/opencode.jsonc` and forbidden on an in-flight branch by `AGENTS.md` -- while a merge is undone by `git revert -m 1` (tie-break 5). The fifth option was prepared and never needed: the projection is the very thing `H-0108`'s legs exist to pin, and nothing went red to tempt it. Tie-break 4 lands here too -- one file resolved by hand out of the seven in the change set. Then the proof, because four files took git's auto-merge: per-file multiset presence over every non-blank `+` line each side added against base `5cfda8d`, run against the merged working tree, script kept outside the repository as `H-0096` did -- `factory/selftest.sh` main 137 added / 0 missing and this branch 340 / 0; `factory/triage.sh` 45/0 and 114/0; `factory/DECISIONS.md` 38/0 and 77/0; `docs/factory.md` 18/0 and 27/0; `factory/common.sh` 0/10; `factory/factory.sh` 0/31; `.opencode/skills/factory/SKILL.md` 0/8. Zero absent from the merged tree, in all seven. **What that number cannot see is the finding, and it is stated rather than assumed away:** 0 missing is equally consistent with two changes each correct and the composition wrong -- one entry ago `main`'s planted payload had no `pageInfo`, `H-0103`'s rule read the missing bound as a stopped page, and twelve legs went red over a merge that lost no line. So the composition was tested twice more. Where the two sides did touch one name: `emit()` is defined TWICE in `factory/selftest.sh`, at `:2766` (`main`'s supervisor shim, which grew to seven `%s`) and `:3731` (this branch's claim shim, which gained `with_entries`), and all four trees -- base `5cfda8d`, `HEAD`, `origin/main`, the merged tree -- carry exactly those two definitions at the same two addresses, so the two edits live in different fixtures and never met. And the suite: `bash factory/selftest.sh` prints `factory/selftest.sh: 429 passed, 0 failed (fixture root /tmp/plock-factoryselftest-AvYiOz on tmpfs, git 2.53.0)`. The 429 is the only figure measured in this commit; 399 (`H-0106`'s count on `main`) and 417 (`H-0108`'s count on this branch) are records, so `399 + 30 = 429` is a consistency check that both sides' legs ran, not a measurement. Then the live read, taken after the merge instead of predicted: `./factory/triage.sh --table` exits 0 and prints both halves in one table -- `issue 246 unclaimed [merged:PR#254 on \`issue-246\` at 2026-11-10T01:39:17Z -- a \`Part of\` phrase in the same body] TimeoutStopSec…` (this branch's note, class unchanged) beside `main`'s `issue 272 delivered merged PR #273 … (window 100)` -- with no pagination-bound line. Last, the mutant surface: neither side's change set contains a `.go` or a `.tmpl` file (`git diff --name-only 5cfda8d origin/main | grep -E '\\.(go|tmpl)$'` and the same against `HEAD` both print nothing), and the 80 distinct `"file"` values across `scripts/mutations/*.json` have zero overlap with the seven touched files (set intersection, measured), so `scripts/check-mutations.py` is not a gate this merge can need (#1048's surface is `.go`/`.tmpl`). What lost. A linear history, never on the table while the branch is pushed. The empty checkpoint `5e688b4` now stands for a tree state nothing can rebuild -- `H-0100`'s cost, paid a fifth time. And `H-0106`'s real fix is still not in this tree: nothing asserts that the fixtures answer every field the shipped reads name, so the next field either side adds to a query repeats this exactly, and this entry adds a number to that debt rather than paying it. Nothing of either side's prose yielded -- no line was traded for the other's wording, which is what `H-0106` lost ten lines to -- because this time the two sides did not write over the same sentence.
- Checkpoint: 5e688b42d6982fea22384b6823cb982d24dbc967
- Effect: the merge of `origin/main` (`7fa162b`) into `issue-277`; `factory/DECISIONS.md` is the only file resolved by hand -- one region, the append offset after `H-0100`, resolved as `main`'s `H-0102`..`H-0106` then this branch's `H-0108`, ascending because that is what `check-decisions.py:161` enforces, then this entry -- and the other six files took git's auto-merge and were counted, not read. Read back in this tree before the merge commit existed: `python3 scripts/check-decisions.py` answers `check-decisions: 142 entries across 2 log(s)` (141 one run earlier, this record being the one), and `make verify` -- run with the merge in progress, because this merge stages no static source and so does not trip `dirty_sources()` the way `H-0106`'s did -- exits 0 with `verify: OK`, `factory/selftest.sh: 429 passed, 0 failed`, `check-deps: stdlib only`, `screenshot manifest: 9 photographs current`, and `git status --short` carrying no `UU`. The branch is 13 commits ahead of `origin/main`'s merge base and `gh pr view 278` will be re-read after the push; the `mergeable` flip from `CONFLICTING` is the API's verdict, not this entry's. What lost is in the `Why:` above: the linear history, the rebuildable checkpoint, and the still-unpaid `H-0106` leg.

## H-0111 · The log collides a sixth time and this side's entries straddle main's, so the splice is cut at id positions -- and the citation audit is the half that found something

- Question: `#274` reads `OPEN`, `reviewDecision=APPROVED`, `mergeable=CONFLICTING`, `mergeStateStatus=DIRTY` -- read back 11 Nov at head `39dd88e` before anything here was touched -- `origin/main` at `4405119` (`git rev-parse origin/main`, the read `H-0100`'s correction names; `git worktree list` is not it), merge base `5cfda8d`. TWO files conflict this time, not one: `factory/DECISIONS.md` and `factory/selftest.sh`. The log is not the four previous append-after-`H-0100` collisions either: this branch holds `H-0098`, `H-0101`, `H-0107` and main holds `H-0102`..`H-0106` and `H-0109`, so ours belong ON BOTH SIDES of main's -- `H-0101` under main's `H-0102`, `H-0107` between main's `H-0106` and `H-0109` -- and git's diff put all nine entries inside one conflict region anyway, because both parents appended after their common `H-0100`. The second file is the silent half #1048 describes: ours appended the `skill_jq_leg` section to `factory/selftest.sh`, main replaced that file's tally `printf` with the one that names its world (`H-0104`), and the two regions are the same handful of lines at the end of the file. What is the deliverable -- the splice that clears two pairs of markers, or the splice plus the proof that a leg from each side still says no on the merged tree and that the `file:line` citations the previous five collision entries wrote down still resolve there?
- Options: take the conflicted file, delete the marker lines, keep both blocks in side order | re-cut the log from the two parent blobs at id positions and let `make verify` be the gate | re-cut at id positions, run the gates, attack one leg from EACH side on the merged tree, and re-open every `file:line` the colliding entries cite | resolve the log with `--ours` | resolve the log with `--theirs` and append this branch's three entries at the end | leave the conflict for the supervisor's own conflict path
- Chose: re-cut at id positions, run the gates, attack one leg from EACH side on the merged tree, and re-open every `file:line` the colliding entries cite
- Why: tie-breaks 1 and 2 have nothing to bite on -- no `require` line is at stake (#856) and no config key decodes toward or away from a lockout (#873, #872). Tie-break 3 separates the first three, on refusal points rather than taste: the resolver exits 2 unless this branch touched NONE of the 96 entry blocks the base already holds (measured, and it is what licenses taking main's trunk whole, because main also rewrote a line inside `H-0053`'s `Effect:` at line 1098 -- a trunk cut from the base would have silently reverted it), and exits 3 unless every line each side added since the base is present in the output and each side's new entries survive as unbroken runs in id order; the plant harness reports a plant that fails to turn a leg red as a failure, never as a pass. Option 1 does not produce a mergeable tree at all: side order writes `H-0107` above `H-0102`, and `scripts/check-decisions.py` carries an explicit out-of-order refusal that its own `--self-test` asserts both ways. Option 5 fails the same arithmetic in the other direction -- `H-0098` landing after `H-0109` is descending twice over. Option 4 is destructive: it deletes main's 128 log lines together with the 45 `factory/triage.sh` lines and 19 `docs/factory.md` lines that `H-0102`/`H-0103` exist to pin. Option 6 is the stall Step 3 refuses. Tie-break 4 does not separate 1-3: the same five files move whichever way the log is cut. Tie-break 5: a merge comes back with `git revert -m 1`, and nothing here needs a rewrite of an in-flight branch. Tie-break 6 was consulted first and names `H-0096`, `H-0100`, `H-0101` and `H-0107`, four collisions that each re-measured on the merged tree; `H-0107` added the citation audit, and this is the first collision where it found a broken citation instead of confirming the last one.
- Checkpoint: 3fa9c334ff78d6715e03428b2fdf5eb8134a9be9
- Effect: the merge commit this entry is in, `origin/main` (`4405119`) into `issue-270`. Two files resolved by hand; `docs/factory.md` and `factory/triage.sh` took git's auto-merge and were then counted, not read.
  - **The log.** Cut out of `git show` output by line range (`/tmp/opencode-resolve-274-log.py`), with main's file as the trunk and this branch's three entries re-inserted at their id position: 3657 lines, 105 entries `H-0001`..`H-0109` ascending, tail written `H-0098`(ours) `H-0101`(ours) `H-0102`..`H-0106`(main) `H-0107`(ours) `H-0109`(main) -- three-way alternating, which is precisely what no side-order splice can write. Against main's blob the whole change is `116 0`: three hunks (`3251a3252,3260`, `3416a3426,3516`, `3465a3566,3581`) and no deletion. The region above the conflict is byte-identical to git's own auto-merge through line 3425, so the hand-work is the tail and nothing else.
  - **The resolver was wrong twice and its own tests caught it, not the eye.** Its first output added 118 lines where the content says 116, and its prefix disagreed with git's auto-merge at line 2315. Two causes, both in the tool: the rule "an inserted block needs a blank after it" was being applied to MAIN's blocks too, and main's `H-0083` ends on a prose line with no blank before `H-0084`'s heading -- the splice was editing a file this branch does not own; and a blob that ends in a newline splits with a trailing empty element, so the join wrote a blank at EOF. Fixed at the source and re-cut; both cells now read what the content says. This is the payoff `H-0101` asked for when it insisted the ordering rule live in code: a hand splice of the same file would have shipped the stray blank into `H-0083` and nobody would have read it.
  - **`factory/selftest.sh`.** Both intents live in the last eleven lines. Ours' `skill_jq_leg` section stays whole; main's world-naming tally takes the slot the bare `printf` held, and the bare `printf` is dropped -- main's prints the same two numbers plus the fixture root, the filesystem under `$TMPDIR` and the git version, so it supersedes rather than replaces. Four lines deleted (three markers and the superseded `printf`); the file is 8999 lines and `bash -n` clean.
  - **Additivity, counted.** Per file, lines each side added since the base and how many are ABSENT from the merged tree (multiset), then the merged tree's drops against lines BOTH parents still hold (sets): `SKILL.md` 12/0 ours, 0/0 main, 1 drop 0 unexplained; `docs/factory.md` 0/0, 19/0, 2/0; `DECISIONS.md` 116/0, 128/0, 2/0; `selftest.sh` 256/0, 384/0, 21/0; `triage.sh` 0/0, 45/0, 2/0. Ten cells of "lost" all zero, every drop explained by one parent having dropped it, no marker anywhere (`/tmp/opencode-additivity-274.py`, which also covers `factory/common.sh`, `factory/factory.sh`, `Makefile`, `scripts/check-decisions.py`, `scripts/check-denies.py`, `factory/next-decision-id.sh` -- untouched by both sides, which is why the citations below could survive at all).
  - **The merged tree was red before it was green, and the reason was the index.** First run: `407 passed, 1 failed`, failing leg `the timer's scripts are executable`, printing `not 100755 in the index: selftest.sh(100755 100755 100755)`. That leg reads `git ls-files -s`, and while a path is mid-conflict the index holds THREE stage entries for it -- base, ours, theirs -- so the leg sees three rows for one file. Nothing about the resolution touched a mode: after the two resolved paths were staged, the same command reads `408 passed, 0 failed`, and 408 is the union arithmetic closing (`H-0109` records main's tip at 407, and this branch contributes exactly one leg, `the --jq programs the skill quotes`, green at that count). A tick that meets this red leg mid-merge and does not read the three rows will go hunting for a lost chmod.
  - **Both sides attacked on the merged tree.** Each plant's anchor asserted to occur exactly once before it is believed, applied to a `cp` backup, restored by `cp` -- never `git checkout`, which would take the merge with it -- and proved byte-identical on `sha256sum`, with `git status --short` unchanged after each. Plant A, this branch's leg: `[.reviewRequests[].login]` back-revolved to `[.users[].login]` in `.opencode/skills/factory/SKILL.md` -> `407 passed, 1 failed`, exit 1, failing leg `the --jq programs the skill quotes`. Plant B, main's #279 plant: the shipped `--json number,title,updatedAt` at `factory/triage.sh:600` shortened by one field -> `385 passed, 23 failed`, exit 1, and the two legs `H-0109` added end to end name it (`a read one field short delivers nothing`, `...while the shipped read still does`). Plant C, main's projection in the PROCESS shim (`gh_json_payload "$file" "$jsonf"` at `:2853` -> plain `cat`) -> `407 passed, 1 failed`, failing leg `...and the process shim obeys it too`. Plant D, the same in the CLAIM shim (`:3833`) -> `405 passed, 3 failed`, first failure `the shim answers only the fields named`. After all four, `408 passed, 0 failed`.
  - **#1048 has no surface here, measured:** 13 mutation sets, 847 anchors naming a file, 797 `.go` and 50 `.tmpl`, and zero naming anything under `factory/`, `.opencode/` or `docs/`, or any `.md`. This merge touches only the excluded kinds, so nothing could have been retired by it and `scripts/check-mutations.py` was not run -- it mutates this working tree in place and must not overlap the gate (#1807).
  - **Citations re-opened, and two are wrong.** On the merged tree: `SKILL.md:93`, `:97`, `:336` hold what `H-0098`/`H-0101` say of them; `factory/factory.sh:268` holds the REST-envelope `--jq '[.users[].login] | index($who) != null'`; `DECISIONS.md:3178`, `:3259`, `:3417`, `:3479`, `:3498`, `:3500` land exactly -- `H-0107`'s five internal line references survive only because every one points BELOW this branch's first insertion point at main's line 3416, and an entry citing a line under main's `H-0106` would not have survived. Two do not resolve: `H-0098`'s `factory/triage.sh:283`, and `H-0101`'s own dated correction of it to `:308`. Main's 45 new `triage.sh` lines moved the `requested_reviewers` read to **`:336`** (`grep -Fn '.users[].login' factory/triage.sh` -> one hit, 336). Neither entry was edited, for the reason all five collision entries give: an edit turns the `116 0` above into a deletion and destroys the evidence this entry spent its ceiling producing. This line is the dated correction `#1180` asks for.
  - **The audit's own probe lied once, and it is the same class of failure it is looking for.** `grep '\.users\[\]\[\]\.login' factory/factory.sh` answers "no match" on a file that holds that text at `:268` -- exit 1, no stderr -- while `grep -c 'users\[\]'` on the same file answers 1: in GNU BRE a DOUBLED escaped bracket pair matches nothing. Fixed strings find both sites. So every "no match" in an audit gets re-run with `-F` before it is believed, or the audit reports absence for presence and the citation looks fine.
  - **Gates, on the tree this commit leaves:** `python3 scripts/check-decisions.py` -> `145 entries across 2 log(s)` (144 before this entry), exit 0; `--self-test` -> 32 must-fail / 9 must-pass, exit 0; `python3 scripts/check-denies.py` -> 23 deny rules resolved by opencode v2.2.5 from disk `--standalone`, 34 commands refused or allowed as declared, exit 0; `bash factory/selftest.sh` -> `408 passed, 0 failed` (fixture root on tmpfs, git 2.53.0), exit 0; `make verify` -> `verify: OK`, exit 0. `verify` runs this suite and both decision gates and the deny gate inside itself (`Makefile:102-116`), so those numbers are quoted as its parts, not as a substitute for it.
  - **What lost, and what is still not established.** (a) The push this merge makes is expected to void `#274`'s approving row under the rule `H-0095`/`H-0097` wrote into Step 5; whether it did is a read after the push, and the pull request carries the answer, not this entry. (b) Plants B, C and D are canned legs under `H-0054`'s rule -- `385 passed, 23 failed` pins the classifier's arithmetic against a fixture, not GitHub's behaviour. (c) Main's `H-0102`..`H-0106` prose carries counts true of the tree each was measured on (182 -> 191 legs, 302 columns, 395 and 387 passed) and stale here by construction; deliberately not edited, and named so no reader takes them for this tree. (d) The three `/tmp` scripts that produced this evidence are not repository artifacts and die with the session -- the seventh collision re-derives them unless someone lands them, which is the residue `H-0101` disclosed and this entry repeats. (e) The zero-anchor `#1048` finding is a property of THIS file set; a merge that touches a `.go` file gets no comfort from it.

## H-0112 · The log collides a seventh time, and the second conflict is a rule this branch wrote at one call site that `main` has just generalised -- so the copy goes on a measurement, not on `main`'s word

- Question: `#278` reads `OPEN`, `reviewDecision=APPROVED`, `mergeable=CONFLICTING`,
  `mergeStateStatus=DIRTY` at head `dba8477` -- read back before anything here was touched, and the
  head is the sha this worktree holds, so the branch is pushed and a rewrite needs the `--force` this
  agent is denied. `origin/main` moved `7fa162b` -> `4405119` (`#280`, `issue-279`) while it sat; the
  merge base is `7fa162b`; `git merge-tree --write-tree` names TWO conflicted paths. The log is the
  settled class seventh time over (`H-0093`, `H-0096`, `H-0099`, `H-0100`, `H-0106`, `H-0110`,
  `H-0111`), and this branch's two entries straddle `main`'s -- ours are `H-0108` and `H-0110`,
  `main`'s is `H-0109` -- so git's layout puts `H-0110` above `H-0109`. The second file is the half
  that has always been silent: both sides inserted a new leg section at the same offset in
  `factory/selftest.sh`, and above that region `main` rewrote both fixture `emit()`s to obey the
  caller's `--json` list -- which is the same rule `H-0108` (this branch) wrote at ONE dispatch site,
  and `H-0109`'s `Effect:` tells whoever resolves the shared region to delete it. So: which splice,
  does this branch keep its copy of the rule, and what counts as proof that a leg from each side
  still says no on the merged tree and that the `file:line` citations the colliding entries wrote
  down still resolve there?
- Options: take git's conflict layout and keep both copies of the projection rule | re-cut the log at
  id positions from the two parent blobs and keep both copies | re-cut at id positions and delete this
  branch's copy because `main`'s entry says to | re-cut at id positions, delete the copy on a
  measurement that the trunk's rule pins the same three legs, and re-open every `file:line` the
  colliding entries cite | resolve the log with `--ours` | resolve the log with `--theirs` and append
  this branch's two entries at the end | leave the conflict for the supervisor's own conflict path
- Chose: re-cut at id positions, delete the copy on a measurement that the trunk's rule pins the same
  three legs, and re-open every `file:line` the colliding entries cite
- Why: tie-breaks 1 and 2 have nothing to bite on -- no `require` line is at stake (#856) and no config
  key decodes toward or away from a lockout (#873, #872). Option 1 is red, measured rather than argued:
  the two splices were written to `/tmp` at the same 3632 lines and each run through the shipped gate
  -- git's layout answers `H-0109 is out of order (H-0110 came before it)` at `:3558`, exit 1, and the
  id-ordered cut answers `check-decisions: 104 entries across 1 log(s)`, exit 0. That is
  `scripts/check-decisions.py`'s order rule, the same one `H-0110` was written for. Options 5 and 6 are
  arithmetic and a stall: `--theirs` deletes 84 log lines and 340 `factory/selftest.sh` lines that are
  this branch's whole delivery of #277, `--ours` deletes `main`'s 74 log lines and 240 fixture lines
  together with the #279 class `H-0109` exists to pin, and Step 3 refuses the seventh. Between the
  middle three, tie-break 3 -- refuse rather than proceed -- is what settles it, pointed at the
  resolver rather than at the outcome: option 2 ships a tree holding two copies of one rule and neither
  copy fails on its own, so nothing in it would ever report the duplication; option 3 accepts a claim
  read out of the other side's decision entry, which is a prophecy about a tree the author never saw --
  `H-0109` says the site-level branch "is not broken by it", which is true, and says nothing about
  whether the legs still bite without it. So the copy was deleted on this tree's own numbers, taken
  twice: unpunctured, `437 passed, 0 failed` with the copy in place and `437 passed, 0 failed` with it
  gone; and under the plant `H-0109` itself prescribes -- `--json number,body,mergedAt,baseRefName` at
  `factory/triage.sh:713`, the shipped merged read one field short -- `434 passed, 3 failed` both times,
  the SAME three legs (`…naming the branch that carried it`, `…and the branch, as a fact to go and
  read`, `…on a branch that is not its own ticket`). Two trees, one behaviour: the trunk's rule alone
  holds this branch's legs. Tie-break 6 was consulted before any of this and is the reason the
  measurement was expected to agree: the merged file ships the rule once, in `install_gh_fields` ->
  `gh_json_payload`, and its own comment at `factory/selftest.sh:2742-2745` names `H-0108`'s site-level
  copy as the thing #279 was filed to end -- keeping the copy would have kept the tree disagreeing with
  its own prose. Tie-break 4 does not separate 2-4 (the same two files move either way); tie-break 5
  covers all of them: the whole merge comes back with `git revert -m 1`, and `git revert` of `b60fd1c`
  would have taken the copy back anyway had the plant disagreed.
- Checkpoint: 6f51edb2b89f327d7ad0bf8cc15f41c3b538fb86
- Effect: the merge of `origin/main` (`4405119`) into `issue-277`; two files resolved by hand, and
  nothing else in the change set needed a marker (`git diff --name-only 7fa162b origin/main` is those
  two paths -- `main` touched no other file).
  **The log.** Cut out of `git show` output by heading position (`/tmp/opencode/resolve-278-log.py`,
  kept outside the repository as `H-0096` onward each did), with `main`'s blob as the trunk and this
  branch's two entries re-inserted at their id position: the tail reads `H-0106`(base) `H-0108`(ours)
  `H-0109`(main) `H-0110`(ours) then this entry -- ours on BOTH sides of main's, which is what no
  side-order splice writes. The cut refused to run twice before it ran: the script asserts this
  branch's prefix equals the base's (it does, one hunk, the append), asserts every difference `main`
  left above the append point is a blank line (it is exactly one, at base line 3417, and taking `main`
  as trunk keeps it -- `H-0111`'s warning is that a trunk cut from the base silently reverts whatever
  the trunk changed above the conflict), and asserts both sides' new entries appear verbatim as
  unbroken blocks. Against `main`'s blob the whole change is `91 0` -- insertions, deletions -- and
  against this branch's it is `76 0`, so neither side's text was edited, only ordered. 3632 lines, 104
  entries before this one.
  **`factory/selftest.sh`.** Both sections kept whole, this branch's #277 section first and `main`'s
  #279 section after it (`main`'s own header sentence reads the sections above it as the queue half),
  joined by the blank line the file uses between sections. The three conflict markers are the only
  lines that splice removes: 9088 lines, `bash -n` clean. That splice was then re-run from the saved
  conflicted file a second time with `--drop-site-branch`, and the first version of the tool had
  deleted one side's entire 287-line section before the arithmetic caught it -- the check that caught
  it is now in the script (`len(out) != len(lines) - 3 + 1`, plus a multiset pass over every
  non-marker line), because a splice that reads correctly at the seam can still be 287 lines short.
  The deletion is 16 lines: this branch's 8-line comment and its 8-line `jq -n --slurpfile` projection
  at the `pr list --state merged` site, back to the base's `emit "$here/pr-merged.json" ;;`. Three more
  lines of this branch's prose were corrected in place, not deleted: the #277 section's header promised
  the projection was supplied "`with_entries`, above", which the merge made false, so the sentence now
  names `install_gh_fields` / `gh_json_payload` and quotes both plant tallies -- the precedent for
  correcting shipped prose a merge falsifies is `H-0106`'s three sentences.
  **Additivity, counted.** Per file, non-blank lines each side added since `7fa162b` and how many are
  ABSENT from the merged tree as a multiset, then the merged tree's drops against lines BOTH parents
  still hold (`/tmp/opencode/additivity-278.py`): `DECISIONS.md` ours 84 added / 0 absent, main 74 / 0,
  0 dropped; `selftest.sh` ours 340 / 19, main 240 / 0, 0 dropped; `triage.sh` 114 / 0 and 0 / 0;
  `factory.sh` 31 / 0; `common.sh` 10 / 0; `docs/factory.md` 27 / 0; `SKILL.md` 8 / 0. Every `main` cell
  is zero, every auto-merged file is zero, and the 19 are enumerated line by line by the script: 16 are
  the deleted copy, 3 are the corrected sentence. No marker anywhere; `git status --short` carries no
  `UU` after the two paths are staged.
  **The merged tree, measured.** `bash factory/selftest.sh` -> `437 passed, 0 failed` (fixture root
  `/tmp/plock-factoryselftest-…` on tmpfs, git 2.53.0), exit 0. 437 is the only figure measured here;
  399 (`H-0106`'s count on `main`), 30 (`H-0108`'s legs) and 8 (`H-0109`'s legs) are records, so
  `399 + 30 + 8 = 437` is a consistency check that both sides' legs ran, not a measurement. Four plants,
  each anchor asserted to occur exactly once before it is believed, applied to a `cp` backup and
  restored by `cp` -- never `git checkout`, which would have taken the merge with it -- each proven
  byte-identical on `sha256sum` with `git status --short` unchanged after: (A) this branch's legs,
  WITHOUT the copy -- the merged read one field short -> `434 passed, 3 failed`, the three
  branch-naming legs; (B) the same plant WITH the copy restored -> the same `434 / 3`, the same three
  legs, which is the measurement option 3 could not offer; (C) `main`'s projection neutralised in the
  CLAIM shim (`gh_json_payload "$file" "$json"` -> `cat`) -> `434 passed, 3 failed`, failing
  `the shim answers only the fields named`, `a field the fixture lacks is refused`, `a read one field
  short delivers nothing` (`factory/selftest.sh:5049`, `:5052`, `:5058`) -- `main`'s legs are live and
  aimed at the merged fixture; (D) the same in the PROCESS shim (`"$jsonf"`) -> `436 passed, 1 failed`,
  `…and the process shim obeys it too` (`:5051`), which is what proves `main`'s auto-merged
  `install_gh_fields "$proc_fix"` and `. "$here/fields.sh"` survived a region neither side conflicted
  in. After the restores, `437 passed, 0 failed`. The live read, taken after the merge rather than
  predicted: `./factory/triage.sh --table` exits 0 with both halves in one table -- `pr 278 conflict
  CONFLICTING/DIRTY; checks pass`, `issue 279 delivered merged PR #280 … (window 100)` (`main`), `issue
  246 unclaimed [merged:PR#254 on \`issue-246\` … -- a \`Part of\` phrase in the same body]` (this
  branch's note, class unchanged), `pr 274 needs_reviewer every verdict voided by a push …` -- and no
  pagination-bound line.
  **`#1048` has no surface here, measured:** zero `.go` and zero `.tmpl` paths on either side of the
  change set; 13 mutation sets, 847 anchors naming a file, 80 distinct files, kinds `.go` 72 / `.tmpl`
  8; the intersection with the seven touched paths is empty. Nothing could have been retired by this
  merge, so `scripts/check-mutations.py` was not run -- it rewrites this working tree in place and must
  not overlap the gate (#1807).
  **Citations re-opened, and four are wrong -- none of them by this merge alone.** On the merged tree,
  with the position on `origin/main` beside it: `H-0109`'s `--json number,title,updatedAt` at
  `factory/triage.sh:563` is at **`:713`** here and was at **`:600`** on `main` and on the base --
  `main` touched no other line of `triage.sh`, so that citation never resolved and this merge moves it
  113 further. `H-0109`'s `delivered_claim (triage.sh:509)` is at **`:608`** here and **`:536`** on
  `main`. The `delivered_map=` scar cited by BOTH `H-0108` and `H-0109` as `triage.sh:470-479` sits at
  **`:506-509`** here and **`:494-497`** on the base -- wrong before either branch existed. And `H-0108`'s
  own `docs/factory.md:486-495`, cited for the `branch:`-channel filter, names text that file does not
  contain at all: `:486-495` is the `install-timer` refusal, `docs/factory.md` has no `--tsv` or
  `branch:`-skip passage, and `main` never touched the file -- so it was wrong in the commit that wrote
  it. Neither entry was edited, for the reason all six collision entries give: an edit turns the `91 0`
  above into a deletion and destroys the evidence this entry spent its ceiling producing. This
  paragraph is the dated correction `#1180` asks for, and it is the second collision running that a
  citation audit found breakage rather than confirming the last one (`H-0111` found two).
  **Gates, on the tree this commit leaves:** `bash factory/selftest.sh` -> `437 passed, 0 failed`, exit
  0; `python3 scripts/check-decisions.py` -> `check-decisions: 144 entries across 2 log(s)`, exit 0
  (143 before this entry); `make verify` -> `verify: OK`, exit 0, with `check-deps: stdlib only` and the
  screenshot manifest inside it; `git status --short` clean. `verify` runs the fixture suite and both
  decision gates inside itself, so those numbers are quoted as its parts, not as a substitute for it.
  **What lost, and what is still not established.** (a) Sixteen lines and one sentence of this branch's own
  prose, to the trunk's single copy of the rule -- the intent survives and is pinned from both
  directions (A pins the read, C and D pin the fixture), but a reader of `b60fd1c` alone will look for a
  projection at the dispatch site and find an `emit` call, and this entry is the only thing linking
  them. (b) A linear history, never on the table while the branch is pushed, and the checkpoint
  `6f51edb` now stands for a tree state nothing can rebuild -- `H-0100`'s cost, paid a sixth time.
  (c) Whether the push voids `#278`'s `APPROVED` row under the rule `H-0095`/`H-0097` wrote into Step 5
  is a read AFTER the push, not a claim here; the same live table already shows the class on the
  sibling (`pr 274 needs_reviewer every verdict voided by a push`), and the pull request carries its own
  answer. (d) Plants A-D are canned legs against a fixture: `434 passed, 3 failed` pins the suite's
  arithmetic, not GitHub's behaviour. (e) The citation audit is a property of these seven files, and the
  four findings say nothing about citations nobody re-opened: nothing in this harness resolves a
  `file:line` written in prose, which is why `H-0111` found two and this found four. A leg that greps
  both logs for `path:N` and asserts the named text is at that line is the unpaid work, and it is this
  unit's `NEXT:`. (f) The two `/tmp` scripts that produced this evidence are not repository artifacts and
  die with the session -- the eighth collision re-derives them unless someone lands them, which is
  `H-0101`'s and `H-0111`'s residue, repeated a third time.

## H-0113 · The halted pass reprints the stored sentence as news: the journal's `HALTED` line now names its own age, from one composer three readers share

- Id: this opened as `H-0054` on this branch -- `90c537b` is a checkpoint that names it -- taken
  before `#248` and `#228` had reached this checkout. `H-0054` is published on `main` as *Forty-five
  legs exec'd the shipped classifier through a preflight check for a binary the gate does not
  install*, and a merged entry's id never moves, so the number yielded: the allocator, re-run after
  the merge of `origin/main` (`d735667`, 175 commits ahead of this branch's base) and again before
  this commit, answers `H-0113`. The half the earlier tick wrote is inside this decision, not in an
  entry of its own -- it died with none -- and its helper is retired below rather than shipped.
- Question: #237 filed two defects. Its item 1 -- the conflict-path halt `:339` names its tree but
  not its resume step, while the tick halt `:490` carries one -- was answered on `main` by a change
  that postdates the ticket: H-0044 unified both writers into `factory_tree_halt`, whose sentence now
  ends ``then `rm $FACTORY_HALT` to resume`` for BOTH verbs (measured on this branch: driving each
  verb writes the same marker, and both marker texts contain the resume clause). Its item 2 landed
  half: #248 added `factory_file_age`, `factory_age_human` and `factory_verdict`, so `--status` can be
  asked how long the queue has been stopped -- but the line the outage was actually read from,
  `log_event "HALTED $(factory_halt_reason)"` at `factory/factory.sh:177`, still prints the stored
  sentence with no age. Measured here before touching anything: one marker with an 8-hour-old mtime,
  three shipped passes at 03:37:54Z, 03:37:57Z and 03:37:59Z, three `HALTED` lines byte-identical once
  their own stamps are stripped, and `--status` answering `HALTED 8h00m --` on one line against
  `marker written 2026-11-10T19:37:54Z, 28807s ago` on the next -- two formats for one fact, and the
  journal with neither. So: does the halted pass date its own cause, and where does the age come from
  -- a new helper, or the ones #248 left behind?
- Options: the journal line prints the marker's age through one composer all three readers share,
  speaking it in `factory_age_human`'s unit with the marker staying sticky | the halted path re-probes
  the named tree and self-clears the marker when it reads clean | ship the draft's own
  `factory_halt_age`, its own `stat`/`date` pair and its absolute-UTC format and all | leave the
  journal alone, because #248's verdict now answers the age
- Chose: the journal line prints the marker's age through one composer all three readers share,
  speaking it in `factory_age_human`'s unit with the marker staying sticky
- Why: the ticket itself called the re-probe worth deciding, and the measurement decides it against.
  Asking the tree again starts with `git update-index --refresh`, which is measured (the comment above
  `factory_tree_dirt`) to rewrite `.git/index`'s stat entries -- so a pass that refuses to touch a
  tree it does not own would write into it on the way out. The four reads that do not write are no
  better: their false-clean direction is what the `naive:` column in `factory/selftest.sh` exists to
  record, and a report-only verdict assembled from them would tell a person to `rm factory/HALT` over
  work it cannot see. A verdict that also self-cleared would resume the queue, unattended, over a
  state the supervisor had just declared somebody else's -- tie-break 3, refuse rather than proceed,
  and the reason the marker stays sticky while the line stops pretending to be new. The draft versus
  main's helpers is tie-break 6 settled by the merged tree itself, with one correction the first
  version of this commit got wrong and a probe caught: what had to be shared with #248 is the SPOKEN
  unit, `factory_age_human`, and shipping the draft alongside would have printed `written
  2026-11-10T19:37:54Z, 28807s ago` beside `HALTED 8h00m` in one `--status` output -- the drift H-0044
  gives the halt one writer to prevent. Reusing `factory_file_age` for the arithmetic, though, is
  wrong twice over and measured so. It clamps: its answer is `$((now > m ? now - m : 0))`, so a marker
  whose time is ahead of this clock -- an NTP correction after a suspend, a VM restored to a snapshot,
  a marker copied back with a future stamp -- comes back `0`, and the line says `cause recorded 0s
  ago`, which is exactly what a detection three seconds old says; measured on this branch with a
  marker planted 7200s ahead, printing the same sentence as the fresh one. And when `stat` fails it
  answers 999999999, which `factory_age_human` spells `never`, dating the cause before the machine.
  Both are the wrong-as-its-own-opposite class #237 is filed against, so the composer holds one `stat`
  and one `date`, refuses both readings in words, and still says the age in `factory_age_human`'s
  unit. Leaving the journal alone loses on where a stall is actually read from: `factory_verdict` is a
  command a person has to think to type, and #248 exists precisely because nobody typed it while 115
  `HALTED` lines went by -- the journal is the artifact the outage was diagnosed from and the only one
  that keeps being written. Tie-break 4 kept it to three files; tie-break 5 kept it reversible as one
  commit.
- What this costs, stated rather than glossed. (1) The draft's absolute instant is gone: the line says
  `cause recorded 8h00m ago`, not `written 2026-11-10T19:37:54Z`, and `stat -c %Y factory/HALT` is
  where the instant still lives. (2) `factory_age_human` truncates to the minute above an hour, so two
  activations inside the same minute print the same age for an hours-old cause -- measured here with
  activations 3 seconds apart on an 8-hour marker, all three reading `8h00m`. The journal's own stamp
  separates those lines, the shipped cadence between activations is minutes, and what #237 asked for
  was never a clock reading to the second. (3) The composer carries its own `stat`/`date` pair, which
  is the thing reusing #248's primitive was meant to avoid; what it does not duplicate is the
  formatting, which is the drift that mattered. (4) The guard allows `factory_notice_halt` to hold the
  bare sentence, because the notice is written once at the moment of detection into the unit's issue
  and an age there would always read `0s`; if a later change re-sends a notice on a fresh activation,
  that allowance is the hole, and the reason is in the scan's comment as well as here. (5) The scan
  skips `factory/selftest.sh`, because the probe carries the shape it refuses in its own planted
  controls -- so a regression written inside the probe is not flagged, which is why every control is
  planted outside the root it scans. (6) Item 1 is delivered by someone else's change and measured
  only as a property of the shipped writer, not by a new test: `factory/selftest.sh` already asserts
  both verbs write the marker (`the journal line is not the tick's`, `the conflict pass journaled the
  tick's verb`), and nothing here pins the words `to resume` inside the marker text.
- Checkpoint: fffdf8bcd224bdaff5f96d961d5ea3d0acc9b847
- Effect: `factory/common.sh` (`factory_halt_report` is the composer, one `stat` and one
  `factory_age_human`; `factory_verdict`'s HALTED arm calls it; the draft's `factory_halt_age` and its
  absolute-UTC format are retired), `factory/factory.sh` (the top-of-pass `HALTED` line),
  `factory/selftest.sh` (the age, the not-re-probed clause, the quoted sentence, the unreadable-time
  arm and the ahead-of-the-clock arm with their controls, the one-line-three-readers equality leg,
  `ageless_halt_prints` with its three controls, and an assertion in `proc_halt_freezes` that the
  second activation's line dates a marker that leg aged by hand), `factory/DECISIONS.md`

## H-0114 · The log collides an eighth time and the two sides interleave -- each has an entry on both sides of the other's -- so the splice is a sort, not a concat

- Question: `#278` reads `OPEN`, `reviewDecision=APPROVED`, `mergeable=CONFLICTING`,
  `mergeStateStatus=DIRTY` at head `a54c410` with all five checks `SUCCESS` -- read back before anything
  here was touched (`gh pr view 278 --json reviewDecision,mergeable,mergeStateStatus,headRefOid` answers
  `{"headRefOid":"a54c4102fe0b5e7a3d43e15bf4b7ba7f286fd0f4","mergeable":"CONFLICTING",
  "mergeStateStatus":"DIRTY","reviewDecision":"APPROVED"}`), and that is the fifth time this pull request
  has been approved and unmergeable at once. `origin/main` moved `4405119` -> `d735667` (PR #274,
  `issue-270`) while it sat approved, and `4405119` is already this branch's second parent, so the merge
  brings exactly #274's five commits. `git merge --no-commit --no-ff origin/main` conflicts in ONE file,
  `factory/DECISIONS.md`, and silently auto-merges the two that carry the checks: `factory/selftest.sh`
  (main +249 non-blank lines since base, this branch +334) and `.opencode/skills/factory/SKILL.md`
  (11 and 8) -- `H-0096`'s half again, and the larger one: the file that decides whether this branch's
  delivery is checked is not the file that conflicted. What is new in the class is the shape of the log
  conflict. Base has 102 entries; main contributes FOUR new ones (`H-0098`, `H-0101`, `H-0107`, `H-0111`,
  the first two filling holes base itself has -- base jumps `H-0097`->`H-0099` and `H-0100`->`H-0102`, the
  gap `H-0105` recorded as permanent) and this branch three (`H-0108`, `H-0110`, `H-0112`), and both sides
  appended at the same three base offsets (`-U0` hunk headers: main `-3251,0 +3252,9`, `-3416,0 +3426,91`,
  `-3466,0 +3567,16`, `-3541,0 +3658,20`; ours `-3416,0 +3417`, `-3466,0 +3468,80`, `-3541,0 +3623,165`),
  so git wrote three regions, not one. Ordered by id the correct tail is `H-0106`, `H-0107`(main),
  `H-0108`(ours), `H-0109`(base), `H-0110`(ours), `H-0111`(main), `H-0112`(ours): main's `H-0107` sits
  below this branch's `H-0108` and main's `H-0111` sits above this branch's `H-0110`, so for the first
  time in this class each side has an entry on BOTH sides of the other's, and neither side-order nor
  "append their block below ours" writes it -- one block spliced after the other can express at most one
  of the two crossings. What is the deliverable: the splice that clears the marker, or the proof that a
  leg from each side still says no on the merged tree?
- Options: take git's side-order layout and delete the marker lines | re-cut the log at id positions out of
  the two parent blobs, count every line either side added in the merged tree, and attack one leg from each
  side on it | resolve `factory/DECISIONS.md` with `--ours` or `--theirs` | renumber this branch's `H-0110`
  and `H-0112` to sit above `main`'s block | rebase onto `origin/main` | re-cut, and restore this branch's
  deleted site-level `--json` projection so its legs do not depend on `main`'s generalised helper
- Chose: re-cut the log at id positions out of the two parent blobs, count every line either side added in
  the merged tree, and attack one leg from each side on it
- Why: the order is the shipped gate's and both layouts were the same length, so the only difference
  between them is the order and the only witness worth asking is the gate. Git's side-order layout, built
  from the three stages (`git show :1:`, `:2:`, `:3:` and `git merge-file`) with the marker lines stripped,
  is 3917 lines and answers 2 problems at exit 1 -- `log.sideorder:3646: H-0107 is out of order (H-0108
  came before it)` and `log.sideorder:3900: H-0111 is out of order (H-0112 came before it)` -- while the
  id-ordered cut, also 3917 lines, answers `check-decisions: 109 entries across 1 log(s)` at exit 0
  (`scripts/check-decisions.py:161`, the same witness `H-0110` and `H-0111` cite). `--ours`/`--theirs` are
  arithmetic, not taste: `--theirs` deletes the 236 non-blank lines this branch added to the log, which
  are `H-0108`, `H-0110` and `H-0112` -- the whole record of #277 -- and `--ours` deletes main's 128,
  including the two entries that close base's own id gaps and the `H-0098` record whose effect is the
  `--jq` fix in the same pull request. Renumbering moves ids this branch has already published:
  `gh pr view 278 --json body` carries `H-0108`, `H-0110` and `H-0112` in prose a reader outside this
  repository holds, and `H-0105` paid the renumber cost once for ids that were genuinely taken -- none is
  here: `grep -c '^## H-0108 '` is 0 in base and main and 1 in ours, `'^## H-0107 '` is 1 in main and 0 in
  ours. Rebasing is refused by the remote rather than by habit: `git rev-parse origin/issue-277` answers
  `a54c410`, so the branch is pushed and a rewrite needs the `--force` `.opencode/opencode.jsonc` withholds
  from a tick, while a merge is undone by `git revert -m 1` (tie-break 5). Tie-break 4 lands here too: one
  file of the three resolved by hand. The sixth option was prepared, measured, and dropped on this tree's
  numbers rather than on `main`'s entry: `H-0112` deleted this branch's copy of the `--json` projection in
  favour of `main`'s `install_gh_fields`/`gh_json_payload`, and #274 grew `main`'s side further, so the
  merged file is re-checked to hold ONE rule, not two -- `with_entries` occurs exactly once in
  `factory/selftest.sh` (line 2799, inside `gh_json_payload` at 2764) and `emit()` exactly twice (2845 the
  process shim, 3825 the claim shim), both routing through that one helper, exactly as `H-0112` left it.
  Then the plant: neutralise the projection (`with_entries(select(.key as $k | $f | index($k)))` ->
  `select(true)`, the one occurrence) and the merged tree answers `435 passed, 3 failed`, exit 1, failing
  `the shim answers only the fields named`, `…and the process shim obeys it too` AND `a read one field
  short delivers nothing` -- a leg that reads as this branch's (#277's class arm) -- so `main`'s single
  copy carries this branch's legs here and a second copy would buy no coverage while re-creating the
  duplicate rule #279 was filed to end. What lost, stated as measured: the linear history, and the
  rebuildable checkpoint -- `33e76b5` is empty and stands for a tree state nothing can rebuild, `H-0100`'s
  cost paid an eighth time; the third entry this branch has written about a merge is itself one of the
  reasons the next merge conflicts, which is the real loss and is not fixed by anything in this commit
  (`H-0106`'s unpaid leg -- nothing asserts that the fixtures answer every field the shipped reads name --
  still stands, and #274's `H-0107`/`H-0098` push the log to 110 entries without touching it); and
  `--ours` on the two auto-merged files is not what I did but is what the gate cannot see, so both are
  counted AND planted against, below.
- Checkpoint: 33e76b5ebd98695e1b3fa2244cf355b38589e9ed
- Effect: the merge of `origin/main` (`d735667`) into `issue-277`; `factory/DECISIONS.md` is the only file
  resolved by hand, cut from the two parent blobs by line range (`git show` of each, no checkout) and
  emitted in id order, base spans verbatim between them. Read back as diffs, not asserted: against main's
  blob the splice is 241 insertions and 0 deletions; against this branch's blob it is 133 insertions
  and 2 deletions, and both deleted lines are blank separators (ours used two blanks before a heading, the
  base's own separator is one) -- so no non-blank line of either side's prose was edited, only ordered. The
  same two diffs over the finished file, this entry's 137 lines included, read 380 / 0 and 272 / 2: the
  deletions are still exactly those two blank lines in both directions, which is the claim that matters. The
  three files then pass a multiset test over every non-blank line each side added since base, each line
  matched against the merged tree with its multiplicity: main 11 / 128 / 249 missing 0, this branch 8 / 236
  / 334 missing 0 across `.opencode/skills/factory/SKILL.md`, `factory/DECISIONS.md`,
  `factory/selftest.sh`; the script is `/tmp/opencode/additivity.py`, kept outside the repository as
  `H-0096` did, and it prints a `missing=0` row for a side that added nothing so a silent side cannot be
  read as an absent one. 0 missing is not a verdict on the composition (`H-0106`: a merge that lost no
  line ran twelve legs red), so the suite was measured at four points ON ONE BOX in one session rather than
  added across entries: base `4405119` 407 passed 0 failed, main `d735667` 408, this branch `a54c410`
  437, the merged tree **`factory/selftest.sh: 438 passed, 0 failed`** (exit 0, fixture root
  `/tmp/plock-factoryselftest-*` on tmpfs, git 2.53.0) -- so the merge's delta (438 - 437) equals
  `main`'s delta over base (408 - 407 = 1) exactly: main's legs arrived, none of this branch's 30 was
  retired by the auto-merge. Then the second plant, from the other side: the shipped merged read one field
  short (`--json number,body,mergedAt,baseRefName,headRefName` -> drop `headRefName`, `factory/triage.sh:486`,
  the one occurrence) answers `435 passed, 3 failed`, exit 1, failing `…naming the branch that carried it`, `…and the branch, as a fact to go and read`, `…on a branch that is not its own ticket` -- this branch's branch-naming legs, which
  is the half `main` cannot see. Both plants applied to a `cp` backup and restored by `cp` -- never
  `git checkout`, which would have taken the in-progress merge with it -- and each file verified
  byte-identical afterwards (`sha256sum -c`, both OK) with `git status --short` unchanged and no `UU`
  anywhere; each anchor was counted to exactly 1 before it was believed, and both plant diffs were read
  before the suite was run. `python3 scripts/check-decisions.py` on the merged tree answers
  `check-decisions: 149 entries across 2 log(s)` (148 one run earlier, this record being the one; 110 of
  them in this harness log, ascending to `H-0114` with four holes -- `H-0031`, `H-0032`, `H-0057` predate
  this branch by a long way and `H-0113` is the one just below -- and the gate asks only for ascending, so
  a hole costs nothing), and
  `make verify` -- run with the merge in progress, because this merge stages no static source and so does
  not trip `dirty_sources()` the way `H-0106`'s did -- exits 0: `verify: OK`,
  `check-decisions: 149 entries across 2 log(s)`, `factory/selftest.sh: 438 passed, 0 failed` (fixture root
  `/tmp/plock-factoryselftest-uX1Z6g` on tmpfs, git 2.53.0), `check-shell: 0.10.0 over 28 scripts`,
  `check-deps: stdlib only`, `check-builds: all targets compile`, `/tmp/opencode/verify.merge.V1.log` at 677
  lines. That run is quoted, not vouched for -- this clause was written after it -- so the gate was run a
  second time over the finished text and its exit code belongs to the pull-request body, which is written
  after the push precisely so that it can carry a read-back instead of predicting one. The id is `H-0114`, not `H-0113`:
  `factory/next-decision-id.sh factory/DECISIONS.md` answers `H-0114` because `refs/heads/issue-237` -- a
  sibling worktree in the same clone, commit `fffdf8b chore(checkpoint): pre-H-0113 …` -- already claims
  `H-0113`, which the allocator sees through the shared `refs/heads` without a fetch; the hole is left
  open deliberately, because the gate requires ascending ids, not contiguous ones, and the alternative is
  taking an id another tree is about to commit. `#1048` has no surface here, measured rather than assumed:
  13 mutation sets, 847 anchors over 80 distinct files, and the intersection of those 80 with the 7 paths
  either side touched is empty; neither side's change set contains a `.go` or a `.tmpl` path (`git diff
  --name-only 4405119 origin/main` and the same against `HEAD` both print nothing), so
  `scripts/check-mutations.py` is not a gate that this merge can need. The id is `H-0114`, not `H-0113`:
  `factory/next-decision-id.sh factory/DECISIONS.md` answered `H-0114` on the clean checkout, because
  `refs/heads/issue-237` -- a sibling worktree in the same clone, `98fa4be`, whose `fffdf8b` is a
  `chore(checkpoint): pre-H-0113` -- already claims `H-0113`, which the allocator sees through the shared
  `refs/heads` with no fetch. Nothing anywhere claims `H-0114`: every ref in `refs/heads` and
  `refs/remotes` was read with `git show <ref>:factory/DECISIONS.md | grep -c '^## H-0114 '` and the answer
  is 0 in all of them. The re-read SKILL.md asks for immediately before the entry commit answers
  `H-0115` here, and that is not a collision -- it is the allocator reading THIS uncommitted entry, because
  its union includes the working tree. Measured in a clean clone at `a54c410` rather than inferred: the
  allocator answers `H-0114`, an uncommitted `## H-0114 ` heading is appended to that clone's log, it
  answers `H-0115`, the heading is reverted, `H-0114` again. So the re-read after the entry is written
  proves only that the entry exists, and taking its answer would move an id that is already correct --
  the collision test SKILL.md actually wants is the per-ref grep above, not a second allocator call. That
  is a documentation gap this commit found and did not fix: SKILL.md:167 says "Re-run the allocator
  immediately before the entry commit" and a tick that obeys it after writing the entry renumbers away
  the entry commit" and a tick that obeys it after writing the entry renumbers away from its own record.
  Nothing here merges this pull request:
  merging is the supervisor's and it merges only what the API still reports approved, mergeable, clean and
  green at the moment of the merge. A push is what spends an approval (`H-0095`, `H-0097`), so whatever
  `gh pr view 278` answers afterwards is the API's verdict and belongs in the pull-request body beside the
  push, not in this record written before it; if it reads `REVIEW_REQUIRED` again, asking is
  `factory_request_review`'s act on the next activation, not a tick's.

## H-0115 · #237's "the two halt writers cannot differ in what the operator is left holding" is true of the source and pinned by nothing, because the suite deleted the conflict pass's marker unread -- so the drift is planted behaviourally and every leg that drives both verbs now compares the pair

- Question: `#237` (opened `2026-11-08T15:11:54Z`, `unclaimed`, head branch `issue-237` sitting six
  commits ahead of `origin/main` and with no pull request -- the `branch_without_pr` case `AGENTS.md`
  names) files one hazard with two halves, measured on 8 Nov: `factory/factory.sh:490` wrote a halt
  marker ending in the resume step while `:339`, the conflict path, wrote
  `a working tree was found dirty and the supervisor will not touch it: $tree` and stopped -- tree, and
  no `rm factory/HALT` -- and the queue then sat eleven hours on that sentence while its `HALTED` line
  repeated byte-identically at 14:09:09, 14:20:09, 14:31:01 and 14:42:09Z for a cause that had been
  cleared at ~14:35Z. It asks for (1) the same resume clause in both writers, "so the two halt writers
  cannot differ in what the operator is left holding", and (2) a decision between re-probing the named
  tree and self-clearing, or keeping the marker sticky and printing its age. Half (2) is decided and
  committed on this branch -- `98fa4be` "fix(factory): the halted pass dates its own cause", entered as
  `H-0113` at checkpoint `fffdf8b`, `2026-11-10T22:47:13` local, three days after the ticket, with an
  earlier attempt parked in `698d31c` ("wip(factory): the halt line carries its age -- unfinished, no
  decision entry yet") behind the checkpoint that opened as `H-0054`, `90c537b`. Half (1) has been true
  of the shipped code since `H-0044` factored both writers into one guard: `origin/main` builds the
  marker in a single `printf` (`git show origin/main:factory/common.sh`, line 1298, ending
  ``then `rm $FACTORY_HALT` to resume``) and that entry says in as many words that the conflict sentence
  "loses 'and the supervisor will not touch it'". So what is left to deliver: write the wording again to
  satisfy the ticket's letter, ship the test that makes the property the ticket states actually held,
  ship nothing, or rebuild the halt path so a writer cannot drift?
- Options: write the conflict verb its own halt sentence and correct its wording | assert the pair inside
  `expect_halt` and add no fixture of its own | a named leg that plants each kind of marker disagreement
  and runs the comparison, with the comparison also moved into `expect_halt` | nothing -- the wording is
  fixed and the age is committed | rebuild the halt path so one writer cannot drift from another
- Chose: a named leg that plants each kind of marker disagreement and runs the comparison, with the
  comparison also moved into `expect_halt`
- Why: because the property #237 states is about what the operator is *handed*, and the suite drove both
  verbs and read the tick's marker only -- the old line 380 did `rm -f -- "$FACTORY_HALT"` on the
  conflict pass's file without reading it, on this file's own comment that the guard is one `printf` so
  reading it twice "would buy a leg that can only fail twice". That premise is a statement about one
  revision's source and the conclusion is a suite that stays green while the merge path hands out a
  different instruction, which is the state the ticket was filed for and which `H-0044` describes in the
  past tense ("the conflict pass wrote a HALT file with neither the UNKNOWN-versus-DIRTY distinction nor
  the matching action"). `H-0113` is the nearest miss worth naming: it measured this very thing --
  "measured on this branch: driving each verb writes the same marker, and both marker texts contain the
  resume clause" -- and kept the sentence in prose, not a leg, so the next reader has to trust that a
  three-day-old ad-hoc drive still describes the file. This is the leg that measurement should have left
  behind. The plant has to be behavioural rather than a source mutation: a mutation to the
  shared `printf` changes BOTH verbs at once, so the tick's read fires first and the conflict read -- the
  one the ticket is about -- is never distinguished. What the others lose. Writing the wording: a
  tree-wide grep for `>"$FACTORY_HALT"` returns `factory/common.sh:1365-1366` and test plants, one
  `printf` for both verbs, so there is no second sentence to correct and the commit would rewrite
  something already correct while claiming the find. Asserting the pair with no fixture of its own:
  nothing would show the comparison can report a disagreement, and a check added to close a gap no check
  could see, with nothing to show it sees, is the same object as the check that was missing -- the rule
  `H-0049` wrote when it built the refusal arms "rather than finding" them, because no state on disk
  isolates one arm either. Nothing at all: it leaves the property unprotected on the exact path that
  stalled the queue for eleven hours, and `$FACTORY_HALT` stops every unit in the pass. Rebuilding the
  path: `H-0044` already rebuilt it, and a second writer is the only thing the drift needs.
- Checkpoint: f5a69fd
- Effect: `factory/selftest.sh` only. `halt_pair_problems` is the comparison, four arms: a marker present
  on one path only, either way round; different bytes; equal bytes on unequal line counts. `expect_halt`
  reads the conflict pass's marker before deleting it and runs the comparison, so all twenty of its legs
  -- every one of which calls both verbs -- consult it, and its `h2` is now asserted empty, which is new:
  a conflict pass that halted the queue with an exit code and wrote no file used to pass the PROCEED legs.
  `halt_markers_agree` is the named leg: it drives the shipped guard on both verbs over one planted tree,
  requires the two markers to be the same bytes and one line each, refuses the identical-and-absent case
  (a command substitution hid the plant in a subshell -- measured, the row reporting "nothing to compare"
  for a run that had written both markers), then plants three disagreements and requires each to be named
  by its own arm; and it pins textually, with a stripped-copy control, that `expect_halt` calls the
  comparison at all. Measured on this tree in this order: baseline at `6412ff9` -- `446 passed, 0 failed`;
  after the change -- `447 passed, 0 failed`, zero `FAIL` rows; mutation A, the
  `pair=$(halt_pair_problems …)` call replaced by `pair=""` -- `447 passed, 1 failed`, the failure being
  the wiring row: `expect_halt never calls it, so the twenty legs above that drive both verbs still read
  the tick's marker only`; mutation B, four lines planted in the shipped `factory_tree_halt` rewriting the
  marker on the `CONFLICT-HALTED` path with the ticket's own resume-less sentence -- `411 passed, 37
  failed`, every one of the 37 naming `the two halts leave the operator different markers` and printing
  both sentences; `factory/common.sh` restored from a copy afterwards, `git diff -- factory/common.sh`
  empty. `./scripts/check-shell.sh` -- shellcheck 0.10.0 over 28 scripts, exit 0. `python3
  scripts/check-decisions.py` -- 151 entries, exit 0. Nothing here addresses half (2), because `98fa4be`
  addresses it; its three arms were re-run through the shipped function on this tree rather than taken on
  faith -- `factory_halt_report` over a marker stamped `touch -d "@$(( $(date -u +%s) - 7500 ))"` prints
  `cause recorded 2h05m ago, and not re-probed since: …`, one stamped 5400s ahead prints
  `cause recorded 1h30m ahead of this clock, so its age is unknown`, and a marker removed between
  `factory_halted()` and the read prints `at a time that could not be read`.
- **What lost, and what is not established.** (a) The plants are behavioural: they pin the comparison and
  the wiring, and claim nothing about git or the guard drifting this way on their own -- the shipped-state
  row and mutation B are the evidence about the shipped state. (b) The `nonewline` plant is the only
  reachable state in which the two sentences are the same WORDS and the files are not, because a sentence
  split across two lines also changes the `tr`-joined string and lands in the bytes arm; that is why the
  count arm is kept with a plant that reaches it rather than dropped as dead. (c) Mutation B is not a
  commit and cannot be re-run from the tree -- four lines planted, measured, restored -- so the recipe is
  in the pull-request body for whoever wants the number again. (d) The per-verb journal line still
  carries no resume clause (`log_event "$verb $unit $tree has uncommitted work (…); refusing to merge
  into it`): the marker and the top-of-pass `HALTED` line both do, and a log line is not the instruction.
  (e) No leg drives `factory_tree_stop` with `CONFLICT-HALTED`: the grep answer is one call site,
  `factory/factory.sh:507`, and every `expect_quarantine` leg passes `TICK-HALTED`, so the conflict
  path's quarantine-and-streak behaviour is unproven at that level even though the marker both verbs
  leave is now proven and the call-site shape is pinned by `halt_call_sites_in`. That is the follow-up
  this ticket's boundary exposed and it is NOT fixed here. (f) Half (2) is committed and **unmerged**, and
  that is the finding worth having: `git show origin/main:factory/common.sh | grep -c 'not re-probed
  since'` is 0, as is the same grep over `origin/main:factory/selftest.sh`, so every supervisor running
  from `origin/main` still prints the ageless line. A later instance of exactly the filed shape is on
  disk: nine `HALTED` lines `2026-11-10T12:07:09Z` through `13:38:09Z` in
  `wt/factory-harness/factory/log/supervisor.log` -- nine distinct timestamps,
  `cut -d' ' -f3- | sort -u | wc -l` = one distinct message, ninety-one minutes, tree `wt/issue-248` --
  written by a supervisor running out of `wt/factory-harness` at `a97ef21`, a tree that cannot contain
  `98fa4be`. The main checkout's journal runs to `2026-11-10T23:51` local with no HALT-family line in it,
  so the live supervisor moved and neither tree has the fix. The eleven hours can therefore recur until
  this branch merges; whether the timer points at the behind tree is not this ticket and nobody should
  read this as settled. (g) Half (2)'s other branch -- re-probe the tree and self-clear -- was decided
  against in `H-0113`, not here, and re-arguing it is not this entry's to do. (h) `docs/factory.md` quotes
  neither halt sentence, so no doc had to change; `will not touch it` appears nowhere in the shipped tree
  outside the new plant. (i) There is no live halted queue to observe, so half (2)'s verdict rests on
  reading and running the shipped function, not on a fresh stall.
- Correction (2026-11-11, the day after this was committed at `b080260`): two counts in the entry above are
  wrong, and the failure is the one `H-0104` named -- "a fifth entry that copies an old count without
  re-running repeats this exactly", with the twist that this count was never old because it was never
  counted. The Effect says `all twenty of its legs` and quotes the wiring row as `the twenty legs above
  that drive both verbs`. Measured on this tree instead: `expect_halt` has NINETEEN call sites
  (`grep -cE "^\s*expect_halt " factory/selftest.sh`) and runs FORTY-ONE legs per pass -- 35 `HALT` and 6
  `PROCEED`, counted off the run's own rows with `grep -cE "^  (ok|FAIL) .*halt:"` -- because the calls sit
  in loops over two trees. Mutation B's 37 failures are therefore those 35 halt legs plus the named leg
  plus its control row, which the entry asserted as "the 37 name X" without saying what the 37 are. The
  FAIL message is corrected in this commit and no longer carries a count at all (`every leg above that
  drives both verbs still reads the tick's marker only`), because the number belongs to the tree and the
  message outlives it. Mutation A re-run after the wording change: `447 passed, 1 failed`, one row,
  the wiring row, printing the corrected sentence. The baseline either side of it is unchanged at
  `447 passed, 0 failed`, and `make verify` on `b080260` printed `verify: OK` with exit 0 before any of
  this was touched. The two sentences above are left as written so the record shows what was asserted;
  this line is the record of what it measures.
- Roll back: `git revert <this commit>` -- the change is additive to `factory/selftest.sh`, and reverting
  it removes the named leg, the comparison and the `h2`/`hl2` reads, restoring `expect_halt` to the shape
  that deleted the conflict pass's marker unread.
