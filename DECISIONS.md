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
git log --grep 'Decision: D-0007' --format='%H %s'   # the effect commit
git revert <that sha>                                 # a forward fix; no history rewrite
```

`Checkpoint:` is the commit taken *before* the decision, never after. That order is load-bearing: a
checkpoint taken afterwards records the state the decision produced, which is the thing you already
have. See the recorded hazard in the project rules -- restoring a backup taken before a later edit
silently un-does that edit, and the suite stays green because the tests were written against the
fixed file.

## Bootstrap entries

D-0001 through D-0005 were recorded in one commit rather than one each. They are one design, agreed
in one conversation before any of it existed, and splitting them across five commits would have put
a checkpoint in front of a decision that had already been taken -- which is the mistake the field
exists to prevent. From the first real tick onward, one decision is one commit.

## D-0001 · The harness lives in the repository, not in a shared skill

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

## D-0002 · Mechanical GitHub actions are bash; a model is only for judgement

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

## D-0003 · Never merge unapproved; poll forever, at a relaxed interval

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

## D-0004 · `question` is denied to the agent; a question becomes a logged decision

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

## D-0005 · One decision is one commit, and rollback is `git revert`

- Question: how is an autonomous decision made reversible?
- Options: rewrite history back to before it | commit a checkpoint before it and revert the effect
  afterwards | keep no record and rely on the branch being closed
- Chose: commit a checkpoint before it and revert the effect afterwards
- Why: the third is how work gets lost quietly, and the first is forbidden here on its own terms --
  `review-loop` refuses to force-push a branch under review, and this repository has
  `allow_squash_merge=false` and `allow_rebase_merge=false`, so history is already merge-commit
  shaped. A checkpoint commit plus one effect commit carrying a `Decision:` trailer gives the same
  power as a rewrite, forward-only: `git log --grep 'Decision: D-0007'` finds the effect, `git
  revert` undoes it, and both are visible to whoever arrives next.
- Checkpoint: 122a80ee2b4a324562e8de82a025a47a6e4dae07
- Effect: .opencode/skills/factory/SKILL.md

## D-0006 · A ref that is a backup by its name is reported, not worked

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

## D-0007 · Pull-request work outranks issue work, which outranks orphaned branches

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

## D-0008 · The decision log is gated inside `make verify`, and the gate pins itself

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

## D-0009 · Install the timer against the harness checkout, not the primary one, until this merges

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
