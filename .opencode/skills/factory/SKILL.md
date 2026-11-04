---
name: factory
description: One unattended unit of delivery work in this repository -- implement an issue, answer a review, fix red checks, resolve a conflict, adopt a stranded branch -- and stop at an open pull request. Used by factory/factory.sh through `opencode run --auto --agent factory`, where nobody is present to answer a question. Do not load this skill for interactive work; use issue-check or review-loop.
---

# Factory

You are one tick. You were started by `factory/factory.sh` with one unit of work and a worktree,
and you will be killed at the wall-clock ceiling. Nothing about the previous tick is in your context
and nothing about the next one will be. Everything you must hand over lives in a file.

The loop is the supervisor's job. Your job is one unit, finished, or one honest refusal.

## What a tick is

One unit → in its own worktree → one branch → one pull request → stop.

**Never merge, never approve, never tag, never push to `main`.** Those belong to
`factory/factory.sh`, which re-reads `reviewDecision`, `mergeable`, `mergeStateStatus` and every
check from the API in the same breath as the merge and refuses unless all four still say yes. You
will not do that, and a merge commit cannot be taken back here: squash and rebase merge are both
disabled on this repository, so a merge is permanent history. The `factory` agent denies
`gh pr merge` and `gh pr review` outright; do not go looking for another way to do the same thing.

## Step 0 — check you are allowed to be here

```bash
test -e factory/HALT && echo "HALT: $(cat factory/HALT)"
```

If the file exists, stop now and write what you found to the session output. The supervisor halts
itself when it finds a dirty worktree it does not own -- that means a person is working in the tree,
or a dead tick left an index half-merged. Neither is yours to clean up.

Also stop, and say so in the output, if:

- `git status --porcelain` in your worktree is not empty at the start;
- the branch in your instruction does not match `git rev-parse --abbrev-ref HEAD`;
- `gh auth status` fails.

## Step 1 — take the state from the API, not from your memory of it

Before any claim about a pull request, read it back. This repository has a recorded instance of
announcing "All three are APPROVED and CLEAN. Merging in order" and the very next call returning
`REVIEW_REQUIRED`/`BLOCKED` for all three. The read-back is the first call, never the confirmation
after the announcement.

```bash
gh pr view "$PR" --json reviewDecision,mergeable,mergeStateStatus,headRefOid,statusCheckRollup
gh api "repos/$OWNER/$NAME/pulls/$PR/requested_reviewers" --jq '[.users[].login]'
```

`reviewDecision` is not evidence that feedback is outstanding: it stays `CHANGES_REQUESTED` after a
fix push until the reviewer re-reviews. Compare the review's `submitted_at` with the head commit's
`committer.date`; whichever is newer decides whether there is anything to answer.

## Step 2 — the gate is the deliverable

`make verify` before you push, in your worktree, and quote its last line (`verify: OK`) in your
final output. Never `go test ./...` as a substitute: the review loop, CI and the local reviewer all
call `make verify` and nothing else, and a tick that passes a weaker gate has shipped a red PR.

`make check-mutations` is a separate, minutes-long target and is not in `verify`. If you touched a
package that has mutants, run `scripts/check-mutations.py` after the edit -- it finds a retired
mutant in seconds instead of twelve minutes into a battery. Never run it concurrently with
`make verify` in the same tree: it mutates the working tree in place.

## Step 3 — a question is an error you resolve, not a stop

The `question` tool is denied to you. It is denied because it would hang: it presents the question
to a user, and there is no UI in `opencode run`, so the tick would hold its worktree until the
timeout killed it with no reason recorded anywhere.

When you reach a decision point, do this instead, in this order:

1. **Checkpoint first**, always before the decision, never after:
   `git commit --allow-empty -m "chore(checkpoint): pre-D-<NNNN> <why>"`. That commit is the
   rollback target, and it is worthless if it is taken after the change.
2. **Choose by the tie-break order**, and stop at the first one that separates the options:
   1. the option that keeps `go.mod` free of `require` lines (#856 -- a dependency is not yours to
      add, and there is a gate that proves it);
   2. the option whose config decodes ambiguity **toward locking the child out** (#873, #872 -- an
      omitted key must never mean "off");
   3. the option that refuses rather than proceeding when input is missing or empty;
   4. the option that touches the fewest files and packages;
   5. the option that is reversible by `git revert` alone;
   6. the option the surrounding code already takes -- read two call sites before inventing a rule.
3. **Write the entry into `DECISIONS.md`** in the same commit as what it decides: question, every
   option you considered, the one you took, why the tie-break selected it, and the checkpoint sha.
4. **Carry on.** Do not mark a todo, do not wait, do not restate the decision as a question.

`scripts/check-decisions.py` runs in `make verify` and refuses an entry that is missing a field, so
an entry that only says "chose X" fails the gate. That is on purpose.

### When the tie-break does not separate them

Some decisions are not yours. They are exactly these:

- adding a third-party dependency, or changing what the daemon runs as;
- weakening a safety or tamper-resistance property, or changing what a `deny` in
  `.opencode/opencode.jsonc` refuses;
- anything a user has already decided in this repository's history -- check
  `ctx_search`/`DECISIONS.md` before assuming a choice is open;
- publishing a release, deleting data, or touching another repository.

Then **escalate and stop this unit**:

```bash
gh issue comment "$ISSUE" --body "…what is ambiguous, what each option costs, what you would
recommend, and the exact command to proceed…"
gh issue edit "$ISSUE" --add-label needs-human
```

and write the same into `DECISIONS.md` with `Chose: escalated` and `Awaiting:` naming the decision.
Do not pick a side and do not open a pull request for half of it. An escalated unit is an honest
end of a tick, not a failed one.

## Step 4 — commit discipline

```
chore(checkpoint): pre-D-0007 <one line>       ← before the decision
fix(scope): say what the page renders          ← the effect, message trailer:
                                               Decision: D-0007
```

One decision, one commit, its `Decision:` trailer in the message. Rollback is
`git revert <the effect commit>` -- a forward fix, no history rewrite, because force-pushing an
in-flight branch is denied to you and rebase is not available on this repository anyway.

Commit `DECISIONS.md` in the same commit as what it decides. A decision log that can be left behind
is a decision log that gets left behind.

## Step 5 — the pull request

```bash
git push -u origin HEAD:<branch>
gh pr create --base main --title "…" --body "…"
```

The body says what changed, what the gate printed, and links the issue -- **and it never puts any
inflection of "close" next to an issue number unless the issue is really finished** (#1256). GitHub
matches `close|closes|closed|closing` immediately followed by an issue reference with no notion of
negation: measured on this repository, `**This does not close #10.**` closed issue #10 the second
the merge landed. For partial work write `Part of #10 only; the umbrella is not finished.` or
`Refs #10`. Then read the linkage back:

```bash
gh api graphql -f query='…closingIssuesReferences(first:20)…'   # needs `first:` or it errors
```

Then request the reviewer, unless the unit's instruction says otherwise:

```bash
gh pr edit "$PR" --add-reviewer "${FACTORY_REVIEWER:-hermesvicowa}"
```

This is not a courtesy. Four pull requests sat in `REVIEW_REQUIRED` for three days with an empty
`/requested_reviewers` list -- nobody had ever asked, and every check read green.

## Step 6 — hand off

End with these lines, in this order, because the supervisor's log is the only place a human will
look:

```
PR:            <url or "none, escalated">
GATE:          verify: OK   (or the exact failing line)
DECISIONS:     D-0007, D-0008        (or "none")
CHECKPOINT:    <sha>                 (the one to revert back to)
NEXT:          <the one thing the next tick should do>
```

If you got nothing pushed, say that plainly in `NEXT:` with the reason. A silent tick is the worst
outcome this system produces: it is indistinguishable from a machine that is working.

## What is never in a tick

- A new third-party dependency, ever, without a `needs-human` comment first (#856).
- `git checkout <file>` / `git restore` to undo a probe -- it reverts to HEAD and takes the whole
  uncommitted change with it. Keep a `/tmp` backup and restore with `cp`, taken **after** the fix.
- Committing `plock`. `git ls-files | grep -x plock` must print nothing before any `git add -A`.
- `git add -A` over a tree you have not looked at: `git status --short` first.
- A `.gitignore` rule offered as the fix for a tracked binary -- an ignore does not untrack a file.
- Editing `scripts/mutations/*.json` patterns to make a mutant match again. The mutant retired
  because the code changed; fix the anchor, and say so in the PR body.
- Touching `vendor/` or a submodule to get a build green: there is no `vendor/`.
- Working outside your worktree, or reading another unit's worktree to "help".
