---
name: adversarial-review
description: Local pre-PR review gate. Spawn a fresh adversarial reviewer subagent that reads the branch diff cold and attacks it for correctness, security, and spec gaps, then fix every valid finding before the diff is pushed. Use after `make verify` is green and before `git push` / `gh pr create`, and whenever the user says "adversarial review", "review this locally", or "attack the diff".
---

# Adversarial Review (local, pre-PR)

The GitHub review loop costs wall-clock time and a human's attention. Everything that can be caught
locally must be caught locally. This skill is the gate between "the tests pass" and "this is worth
another person's time".

Phases: **1 build the brief** -> **2 attack (subagent)** -> **3 validate findings** -> **4 fix and
re-verify**. Loop 2–4 until a round produces no valid findings, capped at 3 rounds.

## 0. Preconditions

- `make verify` is green. Never send a subagent to review code that does not build.
- The diff is committed on its own branch, based on current `origin/main`.

## 1. Build the brief

Collect, and paste into the subagent prompt (it has no memory of this session):

- The ticket: issue number, title, and full body — the reviewer must be able to judge *scope*, not
  just style.
- The relevant contract from `docs/PLAN.md` (the section this ticket implements), verbatim.
- The exact diff: `git diff origin/main...HEAD --stat` followed by `git diff origin/main...HEAD`.
- The list of hard project rules: zero third-party modules; enforcement is always re-asserted on the
  poll tick; nothing reachable without authentication may weaken a lockout; config writes are atomic
  and root-only; lock commands never run a root shell when they can be downgraded to the session
  user.

## 2. Attack

Spawn a `general` subagent (foreground) with an explicitly adversarial brief. Do not summarise your
own intent favourably — hand it the ticket and let it decide whether the code delivers it.

Ask for a table of findings: `severity (blocker/major/minor) | file:line | claim | why it is wrong |
suggested fix`. Direct it at, in priority order:

1. **Lockout correctness** — can the child get screen time during a window? Off-by-one at window
   edges, midnight wrap, DST, weekday set, timezone, empty rule list, a rule that never ends.
2. **Privilege and auth** — unauthenticated route reachability, CSRF, cookie flags, timing-safe
   comparison, password hashing parameters, path traversal in any file path taken from input,
   command injection into a lock backend, running as root where the session user would do.
3. **State and concurrency** — data races on the config reload, the poll tick overlapping itself,
   partial writes, lost updates between the web handler and the enforcement loop.
4. **Spec gaps** — anything the ticket asks for that the diff does not do, and anything the diff does
   that the ticket did not ask for.
5. **Tests that assert nothing** — a test that would still pass if the implementation were deleted,
   or a fake that encodes the same bug as the code.

Tell it explicitly: no style nits, no formatting, no "consider renaming"; a finding must name a
concrete failure. Tell it to say "no findings" rather than invent filler.

## 3. Validate every finding

A subagent finding is a *claim*, not a fact. For each one:

- Open the file and line. Reproduce the reasoning.
- If it is a behaviour claim, **write the failing test first and watch it fail**. If you cannot make
  it fail, the finding is wrong.
- Reject with a one-line reason (kept in your reply, not in the code) anything that is out of ticket
  scope, speculative, or already handled elsewhere — cite the place that handles it.

Never "fix" a finding you have not reproduced; a wrong fix is worse than the reported bug.

## 4. Fix and re-verify

- Fix all accepted findings, adding the regression test that proved each one.
- Re-run `make verify`.
- If any fix changed behaviour a reviewer must see, run phase 2 again on the new diff. Stop after
  3 attack rounds and report anything still open rather than grinding.

## Hard rules

- Never open a PR without at least one clean adversarial round.
- Never forward a subagent finding to GitHub as a code comment; it is input to your judgment, not a
  verdict.
- Never weaken a test to make a finding go away.
- If the reviewer finds a real bug in code that predates this ticket, fix it in its own commit on
  this branch if it is small, or file it as a new issue if it is not — say which you did.
