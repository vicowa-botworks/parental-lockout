---
name: issue-check
description: Check the project's open GitHub issues and address them one by one — a dedicated feature branch per issue, the full verification gate, a PR driven through the review loop to merge, then a main sync and cleanup of the merged local branch, repeated until the issue queue is empty. Use when the user says "check the issues", "address the issues", "work the issue list", "resolve open issues", or asks to drain the GitHub issue queue.
---

# Issue Check Loop

Work the project's open GitHub issues end to end without supervision: for each issue, triage it,
implement the fix on its own feature branch, verify, open a PR, drive it through the review loop to
merge, sync main, delete the merged local branch, and move to the next issue. Stop when the
open-issue queue is empty — or stop and report on any hard-stop condition below.

## 0. Setup

1. Get to a clean main:

   ```bash
   git fetch origin && git checkout main && git pull --ff-only && git status --short
   ```

   If the working tree is dirty, stop and report — never start an issue with pending changes.
2. List the open issues (oldest first):

   ```bash
   gh issue list --state open --json number,title,labels,createdAt
   ```

   If none: report "no open issues" and stop.
3. Make a todo list with one entry per issue (plus a final "queue drained" entry) so progress
   survives context loss.
4. If `gh issue view <N>` fails with a `projects (classic)` deprecation error, read issue bodies via
   the REST API instead:

   ```bash
   gh api repos/vicowa-botworks/parental-lockout/issues/<N> --jq '{title, body, labels: [.labels[].name]}'
   ```

## 1. Per-issue cycle (repeat for each issue, oldest first)

Re-fetch the open-issue list before starting each issue (new issues may have appeared); keep working
oldest first until the list is empty.

### 1.1 Triage

- Read the issue body in full and reproduce/trace it against the current code.
- **Bug that reproduces** → proceed to 1.2.
- **Feature request** → follow the repo's plan-first workflow before coding: check `docs/PLAN.md`
  covers it; if it does not, add the design section there first, then implement.
- **Invalid / duplicate / not reproducible** → make no branch; post a comment on the issue with the
  reproduction attempt and reasoning, and close it (`gh issue close <N>`) with that comment. Mark
  the todo done and go to the next issue.

### 1.2 Branch + implement

- Cut a branch from current main: `fix/<short-slug>` for bugs, `feature/<short-slug>` for features.
  One issue per branch — never bundle two issues in one branch or PR.
- Implement following the conventions of the surrounding code. In this repo specifically:
  - **Zero third-party modules.** `go.mod` must stay free of `require` lines; the standard library
    is the whole dependency budget. If an issue seems to need a module, stop and report instead of
    adding it.
  - Anything the enforcement loop touches goes behind an `internal/platform` or injected interface so
    it stays testable with a fake clock — no test may shell out to `loginctl` or touch a real
    session.
  - A new config field means a `config.Config` change plus a migration/defaults test proving an older
    on-disk document still loads.
  - Every user-visible mutation writes an audit event; every new route gets an authz test asserting
    the unauthenticated case is rejected.
  - Behaviour changes are documented in `docs/PLAN.md` and `README.md`.

### 1.3 Verify

Run the full gate from the repo root and get it green before committing:

```bash
make verify
```

That is `gofmt` cleanliness over the whole tree, `go vet ./...`, `go build ./...`,
`go test -race ./...`, and the zero-dependency guard. Then run the **adversarial-review** skill on
the diff and fix what it finds before opening the PR.

### 1.4 PR + review loop

- Commit (message naming the issue, e.g. `Add schedule evaluation engine (issue #N)`), push, and
  `gh pr create` with `Closes #N` in the body.
- Immediately run the **review-loop** skill on the new PR (CI → team review → merge). It handles CI
  fixes, conflict resolution (merge `origin/main` into the branch, never rebase), reviewer feedback
  (verify every claim against the source before changing code), and the merge gate.
  - `review-loop` is a **global** skill, versioned in `vicowa-botworks/opencode-skills`. Install it
    by cloning that repo **as** `~/.config/opencode/skills` — the whole directory, never just
    `SKILL.md`: the skill runs `watch-pr.sh` from beside it, so a partial install leaves the loop
    half-working with no error. If `opencode debug skill` does not list it on this host, that
    install is missing. Do **not** vendor a copy under `.opencode/skills/`, which silently shadows
    the shared one and goes stale.

### 1.5 Post-merge cleanup

```bash
git checkout main && git pull --ff-only && git branch -d <branch>
```

- Branch cleanup is **local only**: `git branch -d` (safe delete — it refuses if unmerged). Remote
  branches are auto-deleted by GitHub on merge; never `git push origin --delete`.
- Mark the todo done, report a one-line status (issue, PR number, merged), and go to the next issue.

## 2. Hard rules

- One issue = one branch = one PR. No bundling.
- Never force-push or rebase in-flight PR branches; resolve conflicts by merging `origin/main`,
  re-run the gate, push, re-request review.
- Never merge a PR that is not approved with green checks (the review-loop skill enforces this).
- If the review loop hits a hard stop (1-hour poll budget exhausted, >5 fix iterations, recurring CI
  failure, merge blocked), stop the loop and report the PR URL and the failing output — do not
  silently move on to the next issue with one left dangling.
- A dirty working tree at the start of an iteration is a stop-and-report condition.
- Keep the user informed: one short line per issue at triage, PR creation, merge, and cleanup; a
  final summary when the queue is drained (issues closed, PRs merged, branches cleaned up).
