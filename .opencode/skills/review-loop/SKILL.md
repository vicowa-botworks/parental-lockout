---
name: review-loop
description: Drive a GitHub PR through the full CI -> team review -> merge cycle. Use automatically right after creating a PR with `gh pr create` (this repo always runs the review loop after a PR is made), and whenever the user says "run the review loop", "review loop", "wait for review", or asks to take a PR through CI, review comments, and merge.
---

# PR Review Loop

Take the current PR from "pushed" to "merged" without supervision: wait for CI, fix failures,
request the team review, work through comments/change requests, and merge on approval. Never merge a
PR that is not approved with green checks.

Phases: **1 CI** -> **2 review (request + poll)** -> **3 handle feedback** -> **4 merge**. Any fix
pushed in phase 1 or 3 loops back to phase 1.

Repo constants (adjust only if the repo changes):

- Repo: `vicowa-botworks/parental-lockout`
- Review team: `vicowa-botworks/parental-lockout` (verify in step 0; the org may name it differently)
- Review poll interval: 300 s (5 minutes)
- Review poll budget: 12 polls (1 hour) per review cycle
- Max CI-fix iterations per loop run: 5
- Max review-feedback iterations per loop run: 5

## 0. Determine the PR and set up state

1. Find the open PR for the current branch:
   `gh pr list --head HEAD --state open --json number,title,url` If none, ask the user which PR
   number to use.
2. Verify prerequisites once: `gh auth status`, and a clean local working tree (commit pending work
   on the PR branch first if needed). Capture your own login (`gh api user --jq .login`) so your own
   comments/reviews are never treated as reviewer feedback.
3. Resolve the review team once, because a wrong slug fails the request silently for the user part:
   `gh api orgs/vicowa-botworks/teams --jq '[.[] | .slug]'`. Use the slug matching this repo; if the
   org has no matching team, request review from the repo's humans only and say so in the report.
4. Create a state file at `/tmp/opencode/review-loop-<N>.json` (use the Write tool):
   `{ "seenCommentIds": [], "reviewerLogins": [], "requestedHeadSha": null, "ciFixes": 0, "reviewFixes": 0 }`
   `reviewerLogins` accumulates the logins of every human reviewer who left review comments or
   requested changes; they must be re-requested after each pushed fix. Read the state file back at
   the start of every loop cycle so progress survives context loss.

## 1. CI phase - wait for CI, fix failures, loop until green

```bash
gh pr checks <N> --watch --interval=30
```

Run with a bash `timeout` of at least 900000 ms. If the call times out, run it again - it resumes
waiting for the same checks.

- Exit 0 (all pass) -> go to phase 2.
- Non-zero (failures) -> diagnose and fix:
  1. `gh pr checks <N> --fail-only` to see which checks failed.
  2. Get the failed run id:
     `gh run list --branch <head-branch> --limit 10 --json databaseId,displayTitle,conclusion`
     (head-branch from `gh pr view <N> --json headRefName`).
  3. `gh run view <runId> --log-failed`
  4. Fix on the PR branch, run the local verification suite (below), commit with a message like
     `fix: address CI - <summary>`, `git push`.
  5. Increment `ciFixes` in the state file. If it exceeds 5, stop the loop and report the recurring
     failure to the user.
  6. Go back to the top of phase 1.

Before every push of a fix, run the same gates CI runs (from the repo root) and do not push a fix
that fails any of them:

```bash
make verify
```

`make verify` is `gofmt` cleanliness, `go vet ./...`, `go build ./...`, `go test -race ./...` and the
zero-dependency guard. Never push a fix that passes CI locally but skips a gate.

If `gh pr view <N> --json mergeable,mergeStateStatus` shows `CONFLICTING`, resolve by
`git fetch origin && git merge origin/main` (never force-push), push, and restart phase 1.

## 2. Review phase - request the team review, then poll for a decision

Get the head SHA: `gh pr view <N> --json headRefOid --jq .headRefOid`.

If `requestedHeadSha` in the state file differs from the head SHA, request review once per head
revision, using the REST endpoint with a JSON body (NOTE: do NOT use `gh pr edit --add-reviewer`
here - it fails with a GraphQL `projectCards` deprecation error on this org). Always include the
team, plus every login in `reviewerLogins` so anyone who previously reviewed is re-requested in the
same call:

```bash
echo '{"team_reviewers":["vicowa-botworks/<team-slug>"],"reviewers":[<reviewerLogins...>]}' \
  | gh api -X POST repos/vicowa-botworks/parental-lockout/pulls/<N>/requested_reviewers --input -
```

(Omit the `reviewers` array if `reviewerLogins` is empty.) Verify with
`gh api repos/vicowa-botworks/parental-lockout/pulls/<N> --jq '{teams:[.requested_teams[].slug], users:[.requested_reviewers[].login]}'`
and store the head SHA in the state file. New commits reset the review decision, so re-requesting
after every pushed fix is expected and correct.

Then poll. Each poll is ONE bash call with `timeout` 360000 ms:

```bash
sleep 300
gh pr view <N> --json state,reviewDecision,mergeable --jq '{state, decision: .reviewDecision, mergeable}'
gh api repos/vicowa-botworks/parental-lockout/pulls/<N>/comments --jq '[.[] | {id, path, in_reply_to}]'
gh api repos/vicowa-botworks/parental-lockout/issues/<N>/comments --jq '[.[] | select(.user.type == "User") | .id]'
```

For each poll, compute new comment ids = ids not present in `seenCommentIds`, then act:

- `decision == "APPROVED"` -> phase 4.
- `decision == "CHANGES_REQUESTED"` or `decision == "REJECTED"`, or there are new comment ids ->
  phase 3.
- `state` is `MERGED` or `CLOSED` -> stop and report; do nothing else.
- Otherwise print a one-line status ("poll k/12 - still REVIEW_REQUIRED") and poll again. After 12
  polls with no decision, stop the loop and report: "1 hour elapsed, PR is still under review at
  <url>". Do not merge and do not keep polling.

Add every seen id to `seenCommentIds` (including ones handled or dismissed) and save the state file.

## 3. Feedback phase - validate and address review comments

For each new comment:

1. Line comments: fetch full text via `gh api repos/vicowa-botworks/parental-lockout/pulls/<N>/comments`
   (filter to the new ids, skip bot authors); general comments via
   `gh api repos/vicowa-botworks/parental-lockout/issues/<N>/comments` (filter to new ids, skip bot
   authors). Record every human author (who is not your own login) of a new line comment or general
   comment in `reviewerLogins` in the state file.
2. Also fetch the formal reviews and record in `reviewerLogins` every human author (not your own
   login) whose state is `CHANGES_REQUESTED` or `COMMENTED`:
   ```bash
   gh pr view <N> --json reviews --jq '[.reviews[] | select(.author.login != "<your-login>" and (.state == "CHANGES_REQUESTED" or .state == "COMMENTED")) | .author.login] | unique'
   ```
3. Validate each comment: open the referenced file/lines, reproduce the claimed problem, and judge
   whether it is genuine and in scope. For a behaviour claim, write the failing test first and watch
   it fail before changing any production code.
4. Valid -> implement the change (follow repo conventions; run the phase-1 local gates), commit as
   `fix: address review - <summary>`. If `reviewFixes` exceeds 5, stop and report to the user
   instead of ping-ponging forever.
5. Invalid -> make no code change; reply in the thread:
   ```bash
   gh api -X POST repos/vicowa-botworks/parental-lockout/pulls/<N>/comments -f in_reply_to=<COMMENT_ID> -f body="<concise why no change is needed>"
   ```
6. Push all approved fixes in one go, increment `reviewFixes`, save the state file, and IMMEDIATELY
   re-request review from every person in `reviewerLogins` (each reviewer who made review comments
   or change requests must see the new head):
   ```bash
   echo '{"reviewers":[<reviewerLogins...>]}' \
     | gh api -X POST repos/vicowa-botworks/parental-lockout/pulls/<N>/requested_reviewers --input -
   ```
   (Use the same REST endpoint as phase 2, not `gh pr edit`. If some logins are already pending
   reviewers the call is a harmless no-op for them; the team itself is re-requested in phase 2 after
   CI goes green.)
7. Go back to phase 1 (CI on the new head).

## 4. Merge phase - merge on approval

1. Re-verify state right before merging:
   `gh pr view <N> --json state,reviewDecision,mergeable,mergeStateStatus` must show `OPEN`,
   `APPROVED`, and a clean merge state; `gh pr checks <N>` must be green for the current head.
2. Merge (repo convention is merge commits): `gh pr merge <N> --merge`
3. Verify with `gh pr view <N> --json state,mergedAt,url` and report the merged PR URL. The loop is
   finished - do not start new work packages without the user's go-ahead.

## Hard rules

- NEVER merge unless `reviewDecision == APPROVED` AND all checks on the latest head SHA pass AND the
  merge state is clean.
- After pushing fixes for review feedback, you MUST re-request review from every reviewer who made
  review comments or change requests (`reviewerLogins`), in addition to the team. A pushed fix that
  nobody is asked to look at again is a failed loop cycle.
- Never re-request the TEAM for a head SHA already requested (track in the state file); individual
  re-requests in phase 3 happen immediately after each fix push.
- Ignore comments authored by bots (`user.type != "User"`); CI results arrive via checks, not
  comments.
- Never force-push to the PR branch; resolve conflicts by merging `origin/main`.
- Keep the user informed: one short line per poll and a summary on every state change (CI
  failed/fixed, review requested, comments handled, merge result).
- On unexpected conditions (auth failure, PR not found, merge failure, repeated CI flakes), stop the
  loop and report to the user with the exact failing output.
