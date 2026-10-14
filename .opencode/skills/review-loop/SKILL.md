---
name: review-loop
description: Drive a GitHub PR through the full CI -> team review -> merge cycle. Use automatically right after creating a PR with `gh pr create` (this repo always runs the review loop after a PR is made), and whenever the user says "run the review loop", "review loop", "wait for review", or asks to take a PR through CI, review comments, and merge.
---

# PR Review Loop

Take the current PR from "pushed" to "merged" without supervision: wait for CI, fix failures,
request review, work through comments/change requests, and merge on approval. Never merge a PR
that is not approved with green checks.

Phases: **0 resume sweep** -> **1 CI** -> **2 review (request + wait)** -> **3 handle feedback**
-> **4 merge**. Any fix pushed in phase 1 or 3 loops back to phase 1.

Repo constants (adjust only if the repo changes):

- Repo: `vicowa-botworks/parental-lockout`
- Reviewers: `hermesvicowa`, `opencodevicowa` — the repo's `User`-type collaborators. There is
  **no review team** on this repo; see step 2.1 before requesting anyone.
- Review poll interval: 30 s
- Review poll budget: 80 polls (~40 min) per wait. Re-arm rather than lengthen: a re-armed
  watcher costs nothing and a long one hides a state change for an hour.
- Max CI-fix iterations per loop run: 5
- Max review-feedback iterations per loop run: 5

## 0. Resume sweep — do this first, every session

### 0.1 State file

Keep one state file per PR at `/tmp/opencode/review-loop-<N>.json`:

```json
{ "seenCommentIds": [], "reviewerLogins": [], "requestedHeadSha": null, "ciFixes": 0, "reviewFixes": 0 }
```

`reviewerLogins` accumulates every human who left review feedback — they must be re-requested
after each pushed fix. Read the file back at the start of every cycle so progress survives context
loss, and add every comment id you have seen (handled or dismissed) so a comment is never
re-processed. The review baseline (2.2) is a separate file and serves the same purpose for
reviews.

### 0.2 Approved-but-unmerged sweep

A push auto-dismisses approvals on this repo, and a review can land while you are heads-down on
other work. Twice in one session a PR sat **approved and unmerged** until the user asked about it,
which cost a full cycle each time.

```bash
gh pr list --state open --json number,headRefName,reviewDecision,mergeStateStatus
```

For each open PR, read its **review list**, not just `reviewDecision`:

```bash
gh api repos/vicowa-botworks/parental-lockout/pulls/<N>/reviews \
  --jq '.[-3:][] | "\(.id) \(.user.login) \(.state) \(.submitted_at)"'
```

Note the snake_case: `gh api` returns the raw REST payload, so the field is `submitted_at`.
`submittedAt` is the GraphQL name and yields `null` here — a timestamp that silently reads as
nothing is exactly the kind of thing that makes a "newest review" comparison useless. If you want
camelCase, use `gh pr view --json reviews`.

If the newest review is `APPROVED` on the current head and `mergeStateStatus` is `CLEAN`, say so
and merge (phase 4's checks still apply). Do not quietly continue other work past an approval.

## 1. CI phase — wait for CI, fix failures, loop until green

```bash
gh pr checks <N> --watch --interval=30
```

Run with a bash `timeout` of at least 900000 ms. If the call times out, run it again — it resumes
waiting for the same checks.

- Exit 0 (all pass) -> go to phase 2.
- Non-zero (failures) -> diagnose and fix:
  1. Name the failing checks. There is **no `--fail-only` flag** on `gh pr checks` (and no
     `--json` either), so ask the API:
     ```bash
     gh api repos/vicowa-botworks/parental-lockout/commits/<headSha>/check-runs \
       --jq '.check_runs[] | select(.conclusion != "success") | "\(.name) \(.status) \(.conclusion)"'
     ```
  2. `gh run list --branch <head-branch> --limit 10 --json databaseId,displayTitle,conclusion`
     (head-branch from `gh pr view <N> --json headRefName`).
  3. `gh run view <runId> --log-failed`
  4. Fix on the PR branch, run `make verify`, commit, `git push`.
  5. Increment `ciFixes` in the state file. If it exceeds 5, stop and report the recurring failure.
  6. Back to the top of phase 1.

Before every push, run the same gates CI runs and do not push a fix that fails any of them:

```bash
make verify
```

`make verify` is `gofmt` cleanliness, `go vet ./...`, `go build ./...`, `go test -race ./...` and
the zero-dependency guard. Never push a fix that passes CI locally but skips a gate.

If `gh pr view <N> --json mergeable,mergeStateStatus` shows `CONFLICTING`, resolve by
`git fetch origin && git merge origin/main` (never force-push), push, and restart phase 1.

## 2. Review phase — request review, then wait on review IDs

### 2.1 Requesting

**Do not request a team.** `gh api orgs/vicowa-botworks/teams` returns **403** for this token, and
the org-team slug is not a repo collaborator — `team_reviewers` fails with **422**. Request the
`User`-type collaborators directly. `virtualcodewarrior` is the token's own login and cannot
review its own PR.

`gh pr edit --add-reviewer` fails on this repo with a projects-classic GraphQL error, so requests
go through REST:

```bash
gh api -X POST repos/vicowa-botworks/parental-lockout/pulls/<N>/requested_reviewers \
  --input - <<'EOF'
{"reviewers":["hermesvicowa","opencodevicowa"]}
EOF
```

The same GraphQL bug affects body edits: update a PR body with
`gh api -X PATCH repos/.../pulls/<N> --input -`, not `gh pr edit`.

### 2.2 Capturing the baseline — before polling, always

```bash
.opencode/skills/review-loop/watch-pr.sh --capture <N> > /tmp/opencode/review-baseline-<N>.txt
```

This is the step that has been got wrong most often, and every way it goes wrong produces the same
symptom: **the watcher fires on poll 1 reporting a review that was already there**, which reads as
a fresh rejection and burns a whole cycle.

- Capture it **before** the first poll. A baseline captured after a review lands makes that review
  invisible.
- Capture it with `--capture`. Do not hand-roll it: `--jq '.[].id' | tr '\n' ' '` produces one line
  with a trailing space, and a strict `^[0-9]+$` filter silently drops it, so the baseline reads as
  "no known reviews" and every existing review looks new.
- An empty file is legitimate (a brand-new PR has no reviews). A **non-empty** file with no digits
  in it is fatal, and the script refuses rather than degrading.

### 2.3 Waiting

`reviewDecision` is **not** a poll signal. It stays `CHANGES_REQUESTED` after a fix push until the
reviewer actively re-reviews, so a wait that breaks on the decision string ends immediately on the
stale change request. The only reliable signal is a review id that was not present when the wait
started.

```bash
.opencode/skills/review-loop/watch-pr.sh <N> /tmp/opencode/review-baseline-<N>.txt 80
```

Exit `0` a new review appeared (it prints the id, author and state) · `3` budget exhausted ·
`2` unusable baseline.

**Launch it as a harness-tracked background command**, never `nohup … &`. A detached process never
fires the completion notification, so the session goes deaf to the very event it is waiting for —
this happened, and the review sat unread. Do other useful work while it runs; you will be resumed
when it finishes.

Before trusting any watcher, run its self-test — all four controls are synthetic and two of them
exist because a control that *cannot fail* (or *cannot pass*) shipped twice:

```bash
.opencode/skills/review-loop/watch-pr.sh --selftest
```

When it reports a new review, read it in full before touching code, and verify every finding
against the current tree rather than assuming it.

### 2.4 Process management

`pgrep -f` / `pkill -f` match the command line of the shell running them, so a pattern like
`"watch-pr.sh 26"` kills your own shell or reports your own command as a live watcher. This bit
three times in one session. Use a bracketed pattern that cannot match itself
(`watch-pr[.]sh`), or wait on a sentinel line in the log instead of on a process.

## 3. Feedback phase — validate and address review comments

For each new review or comment:

1. Fetch full text: line comments via
   `gh api repos/vicowa-botworks/parental-lockout/pulls/<N>/comments`, general comments via
   `.../issues/<N>/comments`, formal reviews via `.../pulls/<N>/reviews`. Skip bot authors
   (`user.type != "User"`) and your own login.
2. **Validate before fixing.** Open the referenced file and lines, and reproduce the claimed
   problem against the current tree. Reviewer reports may target the pre-fix tree. For a behaviour
   claim, write the failing test first and watch it fail before changing any production code.
   A finding that does not reproduce is a finding to dismiss, with a reply saying so.
3. Valid -> implement, run `make verify`, commit as `fix: address review - <summary>`.
4. Invalid -> make no code change; reply in the thread:
   ```bash
   gh api -X POST repos/vicowa-botworks/parental-lockout/pulls/<N>/comments \
     -f in_reply_to=<COMMENT_ID> -f body="<why no change is needed>"
   ```
5. **Batch every fix into ONE push.** The ruleset sets `dismiss_stale_reviews_on_push: true` and
   `require_last_push_approval: true` with `bypass_actors: []`, so *any* push dismisses the
   approval and the head must be re-approved. Interleaving "fix, push, fix, push" forces the
   reviewer to re-review each time, and a reviewer who has already approved will not notice the
   dismissal — `reviewDecision` can read `APPROVED` while the newest entry is `DISMISSED`.
6. After pushing, re-request review from everyone who reviewed, then go back to phase 1.

If `reviewFixes` exceeds 5, stop and report to the user instead of ping-ponging forever.

## 4. Merge phase — merge on approval

1. Re-verify on the **exact current head**, not the last one you saw:
   ```bash
   gh pr view <N> --json state,headRefOid,mergeStateStatus,reviewDecision
   gh api repos/vicowa-botworks/parental-lockout/commits/<headSha>/check-runs \
     --jq '.check_runs[] | "\(.name) \(.status) \(.conclusion)"'
   gh api repos/vicowa-botworks/parental-lockout/pulls/<N>/reviews \
     --jq '.[] | "\(.id) \(.user.login) \(.state)"'
   ```
   `state == OPEN`, `mergeStateStatus == CLEAN`, checks green **on that sha**, and a live
   `APPROVED` — meaning the newest review from a human is `APPROVED`, not `DISMISSED` by a later
   push. `reviewDecision` alone is not sufficient for either direction of error.
2. Also read any `COMMENTED` review newer than the approval: a correction can ride in on a
   non-blocking comment, and merging over it loses the finding.
3. Merge (repo convention is merge commits): `gh pr merge <N> --merge`
4. Verify with `gh pr view <N> --json state,mergedAt,url` and report the merged PR URL.
   The follow-up branch delete may 404 because GitHub auto-deletes merged branches here — that is
   not a failure.
5. Close the ticket with a comment if the PR body's `Closes #N` did not do it, and file any
   non-blocking reviewer suggestions as an issue so they do not evaporate with the thread.

## Hard rules

- **NEVER merge unless** a live human `APPROVED` review exists on the current head, AND all checks
  pass on that sha, AND the merge state is clean.
- **Never use `gh pr merge --admin`** (or any bypass) to get past a review gate. A ruleset the user
  configured is not an obstacle to route around; if it blocks a merge that should proceed, say so
  and let the user decide.
- **Batch all review fixes into one push** (see 3.5). Any push dismisses the approval here.
- After pushing fixes, you MUST re-request review from every reviewer who commented or requested
  changes. A pushed fix nobody is asked to look at again is a failed loop cycle.
- **Never poll on `reviewDecision`.** Poll review IDs against a pre-captured baseline (2.2/2.3).
- Run `watch-pr.sh --selftest` before trusting a watcher, and never weaken a control to make it
  pass — a control that cannot fail is worse than no control.
- Ignore comments authored by bots (`user.type != "User"`); CI results arrive via checks.
- Never force-push to the PR branch; resolve conflicts by merging `origin/main`.
- Do not let tooling changes ride along in an implementation PR — skill and script fixes get their
  own branch and PR.
- Keep the user informed: one short line per state change (CI failed/fixed, review requested,
  review arrived, comments handled, merge result), not one per poll.
- On unexpected conditions (auth failure, PR not found, merge failure, repeated CI flakes), stop
  and report with the exact failing output.
