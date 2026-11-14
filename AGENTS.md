# Repository rules for agents

Read [docs/factory.md](docs/factory.md) before touching the delivery loop. Everything below is a
rule that exists because of a measured failure, and the measurement is cited.

## The queue belongs to the supervisor

`factory/factory.sh` owns the issue and pull-request queue once its timer is installed.

- Do not merge a pull request by hand that the supervisor could merge. It merges only what the API
  *still* reports as approved, mergeable, clean and green at the moment of the merge; a hand merge
  skips that and cannot be undone here — `allow_squash_merge=false` and `allow_rebase_merge=false`,
  so every merge is permanent history.
- Do not run `factory/factory.sh` while a tick may be running in a tree you are about to use. It
  takes `factory/run/lock`, which only keeps *supervisors* out of each other; a session and a tick
  can still share a worktree. Check `git status --short` before `git add`.
- Requesting a reviewer is part of opening a pull request, not a later courtesy -- and the component
  that does it is the supervisor's poke, not whoever opened the pull request. `factory_request_review`
  (`factory/factory.sh`) POSTs `pulls/N/requested_reviewers` for any open pull request whose `users`
  and `teams` lists are both empty, at most once per `FACTORY_REVIEW_POKES`, addressed to
  `FACTORY_REVIEWER_TEAM`. That component is what makes this rule true -- four pull requests once sat
  in `REVIEW_REQUIRED` for three days with an empty `/requested_reviewers` list while every check read
  green (`docs/factory.md`) -- and a unit has no verb of its own: `gh pr edit --add-reviewer` exits 1
  on this account's retired `projectCards` field and writes nothing (`H-0010`, re-measured on #307 and
  reported in #310). So a unit that thinks the ask is going to the wrong person says so on the pull
  request, names `FACTORY_REVIEWER_TEAM` or `FACTORY_REVIEWER`, and leaves the timer's environment to
  whoever installed it.
- A branch with commits and no pull request is work that has already been paid for and is invisible
  to every queue that reads issues. `factory/triage.sh` reports it as `branch_without_pr`. The same ref
  name with a merged pull request behind it and no content the trunk lacks is the opposite thing -- a
  delivery, reported as `branch delivered` and worked by nobody (#291). Telling the two apart is a
  content question, never the name: `issue-301` has 4 files across 2 commits ahead of `main` under a
  name whose earlier pull request merged, and an arm that stopped at the name retired that work.

## Decisions

`DECISIONS.md` is the record that makes an unattended judgement call reversible, and
`factory/DECISIONS.md` is the same record for the delivery harness itself, with `H-` ids so a
`D-0013` in a page-design context and an `H-0013` in a permissions context cannot collide, and
`scripts/check-decisions.py` enforces its shape inside `make verify`.

- Checkpoint **before** a judgement call (`chore(checkpoint): pre-D-NNNN`), then one effect commit
  carrying `Decision: D-NNNN`. A checkpoint taken afterwards records the state the decision produced,
  which is the thing you already have.
- Write the entry in the same commit as what it decides, and record what *lost*. An entry with one
  option is a description of what happened.
- Roll back with `git revert`, never a rewrite: force-pushing an in-flight branch is forbidden here.
- Four classes of decision are not an agent's to make — a third-party dependency, security or
  tamper-resistance semantics, publishing, deleting data. Comment on the issue, label it with the
  escalation label (`factory/escalation-label.sh` prints it; `factory/common.sh` is the
  only place that name is written), write the entry with `Chose: escalated` and `Awaiting:`
  naming what is owed, and stop. Adding a `require` line to `go.mod` needs the user, not a
  justification in a PR body.
- A `path:line` citation in a decision log is a promise that a reader can go and look, and a merge can
  take the ground from under it: `H-0139` cited `AGENTS.md:38` for the escalation instruction, and the
  merge of #312 slid that instruction down to `AGENTS.md:48-53` without editing a word of the entry. So
  write the measured sentence beside the coordinate — `AGENTS.md:52` "naming what is owed, and stop" — and
  `scripts/check-decisions.py` compares the quote with the line and refuses the entry when they part.
  A citation with no quote is allowed but counted, and `POINTER_DEBT` in that script may only fall;
  a merge that grows the logs re-measures it with `--citations` and records the number in the same
  commit. Before editing a file the logs quote, ask what you are about to move:
  `python3 scripts/check-decisions.py --citations <path>` lists every pointer aimed at it.
  That number is not judged at all while any citation cannot be opened, because a stray untracked
  directory moves pointers out of it, and no number measured against such a tree is a statement about
  the logs.

## Worktrees

Per-unit trees live in `$FACTORY_TREES` (default `../wt`), never under `/tmp` — a tree a reboot can
delete can also delete unmerged work. Remove one when its branch merges
(`git worktree remove`), and run `git worktree prune` after a machine restart. Six abandoned
worktrees, two of them on the same commit, are what happens otherwise.
