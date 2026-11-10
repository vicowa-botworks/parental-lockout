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
- Effect: `FACTORY_DELIVERED_WINDOW` in factory/common.sh; the merged read, the jq verdict, the refusal on stderr and the `delivered` row in factory/triage.sh; the `delivered)` arm in factory/factory.sh, which reports and dispatches nothing; in factory/selftest.sh, `claim_fixture`'s three planted open rows -- #228, #212 and #149, closed on live GitHub, with their stamps chosen rather than read -- the 128 KiB pad leg, the newest-wins and window legs, and the mutation count corrected in the section header -- legs 120 -> 176; `delivered` in docs/factory.md's class table with the two paragraphs on why the verb is not the verdict and what the bulk call costs.


## H-0054 · Forty-five legs exec'd the shipped classifier through a preflight check for a binary the gate does not install, so `verify` was red on CI and green on every box in the loop

- Question: PR #238's `verify` check answers FAILURE at `ae8aba8` with the other four green, and every one of the 45 failures is a leg in the merged-claim section H-0053 added. Does that section get the runtime it silently requires, does the requirement go, or does the gate install the thing it is not gating on?
- Options: install `opencode` in the `verify` job | narrow `factory_preflight` so the read-only `triage.sh` asks only for `git` and `gh` and only `factory.sh` asks for the model | put an `opencode` stub in the fixture bin the section already builds and pin `FACTORY_OPENCODE` to it | export `FACTORY_OPENCODE=git` in `claim_rows`, satisfying preflight with a binary every gate host has
- Chose: put an `opencode` stub in the fixture bin the section already builds and pin `FACTORY_OPENCODE` to it
- Why: the diagnosis was measured before it was accepted, because "CI has no opencode" is a claim until it is reproduced -- `env PATH=<PATH minus ~/.opencode/bin> bash factory/selftest.sh` at `ae8aba8` prints `131 passed, 45 failed` and `factory: missing on PATH: opencode. Nothing was started.`, which is CI's tally and CI's sentence on the same commit, while the untouched PATH prints 176 passed 0 failed. So the section was reading the host, not the fixture. The requirement is `factory/common.sh:313`, `for tool in git gh "$FACTORY_OPENCODE"`, reached from `factory/triage.sh:80`, and `.github/workflows/ci.yml:21-49` installs Go and nothing else. Tie-break 6 chose between the options before any preference here did: the section already answers the *other* thing preflight demands -- `gh` -- with a fixture shim in `$claim_bin` (`factory/selftest.sh:1463`), so supplying the second dependency the same way is the shape the surrounding code already takes, in one file, reversible by `git revert`. Narrowing preflight is the tempting one and the review's own sentence argues for it ("a read-only `--table` should not require an agent runtime that only `factory.sh:529` ever starts"), but it deletes a refusal from a check all three callers share (`factory.sh:104`, `install-timer.sh:69`, `triage.sh:80`), and tie-break 3 will not buy a green gate by weakening the check whose comment is "everything a tick needs, checked before a model is invoked"; both production paths are covered without it -- `factory.sh:104` runs the same preflight before any tick and `install-timer.sh:119-121` refuses to install the timer at all when `$FACTORY_OPENCODE` is not on PATH. Installing `opencode` in `verify` is the worst of the four, for the reason the review gives: the job would need an agent runtime to test a read-only queue report, and `ci.yml:62` already records ".opencode/ is not installed anywhere" as a fact about the tree. `FACTORY_OPENCODE=git` is one line and green and it is the wrong fact -- it satisfies preflight with a binary unrelated to the requirement, so the legs would stop exercising the shipped default (`common.sh:62` spells it `opencode`) at the one term preflight tests. Three legs pin what was chosen: the runtime the legs resolve is inside `$claim_bin`, the stub exits 78 with a sentence instead of starting a model (the same shape as the shim's 77 at a write), and `common.sh` still carries the requirement -- with a control that deletes the term from a copy and demands the scan say no, because a fixture answering for nothing is this section's own failure mode. Measured after, three ways on one box: 179 passed 0 failed with no `opencode` on PATH, 179 passed 0 failed with it installed, and 179 passed 0 failed with `FACTORY_OPENCODE=plock-agent-that-does-not-exist` exported -- that last one is green because of the pin, not the PATH.
- Checkpoint: 84e192c549e52671baf92d0ed17c12261f73d95f
- Effect: `$claim_bin/opencode` with its `chmod`, `FACTORY_OPENCODE=opencode` in `claim_rows`, and legs 176 -> 179 on this tree (btrfs, `/home/rodney/prj/wt/issue-235`; CI counted 176 on this branch too, as 131+45). What lost, as measured: the shipped `triage.sh --table` still refuses to run on a machine with no model runtime, which is the wrong refusal for a read-only report -- the fixture now hides that from the suite, so the residual lives in this entry rather than in a green tally, and the day anyone narrows preflight the stub is the thing to delete; and the stub answers `command -v` for a binary the section never execs, so if `triage.sh` ever learns to start a model its legs get exit 78 and a sentence rather than the behaviour -- the intended failure, and still one nobody has watched fail.


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
