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

## What you may not run

`.opencode/opencode.jsonc` is the boundary, and only one of its two shapes is enforced. A tool's rule
is either a bare word -- `allow` | `ask` | `deny`, keyed by the tool's own name -- or, for `bash`, a
**table** of command patterns:

```jsonc
"permission": {
  "question": "deny",                                  // the tool is `question`
  "bash": { "* pr merge*": "deny" }                    // `bash` is a pattern table
}
```

A pattern written at the top level instead becomes an **action**, and no tool reports an action named
after a shell command, so `--auto` approves the command anyway. Measured both ways on v2.2.5 with the
same entry: flat, `gh pr merge 999999` ran; nested under `bash`, the same call came back
`Permission denied: shell`. Check your own keying before you credit a deny rule with stopping
anything -- `opencode debug agents --standalone` must show every rule as `"action": "shell"` with the
pattern in `resource`, never as the pattern in `action`. `--standalone` is not decoration: without it
the CLI answers from a running server's startup config, so a rule you just wrote, and a rule you just
broke, both read as whatever was true when that server started. Here that is checked for you --
`scripts/check-denies.py`, run by `make verify`, asserts both spellings of every denial and a short
allow-list against what the CLI resolves, and refuses a pattern nothing witnesses.

Two things that table is not. It is not a prefix list: a pattern is matched against the whole parsed
command line, anchored at both ends, which is why the push rules here read `* push --force*` and would
miss `git -C /some/tree push --force` if they began with `git` -- and equally why a program that can
*open* a command needs its literal twin as well: `* rm -r*` does not reach `rm -rf /tmp/x`, and the
bare form is what you type first. Both shapes, or the denial has a hole in it. And it is not a sandbox
-- it constrains a cooperative agent by the command shapes it can spell, and a whole-line glob matches
inside quoted arguments too, so `git commit -m "why * rm -r* is denied"` is refused and that is the
design, not a bug to route around. The list: `question` (headless, so a question is
a stall), `gh pr merge`, `gh pr review`, `gh api`, force-push, delete-push, any push to `main`,
`go get`, `git tag`, `gh release`, `systemctl`, `rm -rf`, and `git worktree remove|move`.

`gh api` is denied whole, which costs you two reads people expect a tick to have: who is currently
requested to review, and what a pull-request body closed. Take the first from `gh pr view --json
reviewRequests`, and do not attempt the second -- see Step 4. If a denied command is genuinely the
only way to finish, that is a decision, not an obstacle: write the DECISIONS.md entry, say in the pull
request what you could not do and why, and deliver the rest. Never wrap the denied call in `bash -c`,
`env`, `xargs`, `sh -c`, or a script that shells out to it -- that is the one way to turn a harness
gap into a trust problem.

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
gh pr view "$PR" --json reviewRequests --jq '[.reviewRequests[].login]'
```

`--json` hands `--jq` the whole payload object, so the program has to name the field the call asked for.
`--json reviewRequests` yields `{"reviewRequests":[…]}`; `[.users[].login]` -- the program for the
REST `/pulls/N/requested_reviewers` envelope, which wraps the same rows as `{"users":[…]}` and which is
what `factory/factory.sh` and `factory/triage.sh` read there, each through one shared program
(`$JQ_REQUESTED_HOLDS` and `$JQ_REQUESTED_LIST`, `factory/common.sh`, which counts `teams[]` as well as
`users[]` -- #241) -- answers `cannot iterate over: null` with exit 1
against this payload (measured 10 Nov on gh 2.46.0 at `/usr/bin/gh`, PR 269: issue #270). The quiet
variant is worse than that error: a program reading a field that is merely absent answers `[]`, which is
one and the same sentence with "nobody has been asked" and with "asked, and already answered" -- and
which of the three it is decides whether the supervisor nags a reviewer or stays quiet. Every `--jq`
program quoted in this document is now executed by `factory/selftest.sh` against a canned payload,
because this block is prose a tick runs verbatim and nothing until now had ever run it.

Spell those reads as two calls, in this shape. A command the permission scanner cannot parse is refused
outright, and the refusal looks like a tool bug rather than a denial: measured 5 Nov, a tick composed one
`gh pr view` with a `-q` program containing `\n` escapes and several nested quotes, and every attempt came
back `Error: Portable shell scanner cannot analyze command: unterminated-quote` -- 45 minutes of ceiling
gone, `TICK-FAILED exit=124`, nothing answered. Cite the binary as well as the version when you repeat
this: `command -v opencode && opencode --version`, both together. The measurement above is v2.2.5 at
`~/.opencode/bin/opencode` on the box that took it, which is the binary a tick meets because
`factory/common.sh`'s `FACTORY_OPENCODE` resolves `opencode` through `PATH` and `install-timer.sh`
bakes that `PATH` into the unit. On another box the same path reported 1.17.13 while the `opencode` on
`PATH` there reported 1.18.5, and the quoted string is not present in 1.18.5 -- so a version with no
path attached is a claim nobody can re-run, which is the objection `D-0015` makes about digests.
Re-measure before repeating it on another version, and name the version when you do. Now that the deny
rules are keyed under `bash` and therefore actually evaluated, the scanner parses every command this
agent runs, and a shape it cannot analyze is fatal instead of merely unmatched. One statement per call,
`--json` plus a short `--jq`, and never a program with escaped newlines inside a shell quote.


`reviewDecision` is not evidence that feedback is outstanding: it stays `CHANGES_REQUESTED` after a
fix push until the reviewer re-reviews. Compare the review's `submitted_at` with the head commit's
`committer.date`; whichever is newer decides whether there is anything to answer.

The same read is owed a second time *after* you push, and Step 5 says why: the push can spend the
approval the pull request already had, and nothing about the push itself reports that it did.

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

1. **Allocate the id before taking the checkpoint.** Run the allocator for the log you will write:

   ```bash
   factory/next-decision-id.sh DECISIONS.md
   factory/next-decision-id.sh factory/DECISIONS.md
   ```

   Use exactly what it prints. It takes the union of the current checkout, `HEAD`, `refs/heads` and
   `refs/remotes`, then returns the first free id above that ceiling. If it refuses, **your path is
   wrong, not the tool**: it answers only for the decision log itself -- a path whose name is
   `DECISIONS.md`, holding a regular file in the working tree or a blob in `HEAD` -- because
   an empty union has exactly one answer available -- `D-0001`/`H-0001`, the number both logs opened
   with -- and handing that out is the collision this command exists to refuse (`H-0073`). Read its
   message and name the log you mean; never fall back to numbering by eye. A relative path resolves
   against the repository it was aimed at (`--repo`), never against the directory you happen to be
   standing in. Never take the highest heading in
   the current file as the whole answer: a sibling branch's entry is already a durable citation and a
   merge target even though this checkout has never seen it (#253). Re-run the allocator immediately
   before the entry commit, **and take that re-read before you write the entry, not after it**. The
   union includes the working file, so once your own heading is in the log the re-read counts it and
   answers one above the id you hold: measured in `wt/issue-320` on 12 Nov, `H-0145` on the committed
   log and `H-0146` with an uncommitted `## H-0145` in the file and no ref holding it. That is not a
   collision, and the allocator says so -- a `PENDING` line on stderr names the heading that is in your
   working file and on no ref this clone can see, and the id the refs actually top out at. So read a
   moved answer by its stderr: **`PENDING` naming the id you already hold means nothing was taken** --
   keep that id, and do not renumber into a gap `docs/factory.md` says must never be inexplicable. A
   higher answer with **no PENDING line** is the real thing: some ref carries your id, and you yield the
   way that guide says. Read the note's absence, never the emptiness of stderr. Since PR #307 every
   answer carries a `REFRESH` or `REFRESH-BLIND` line beside the id and no flag silences it, so there is
   no answer that leaves stderr empty -- measured at `487d572` on one fixture, 460 bytes with the tree
   clean, 855 with your own heading uncommitted, 460 one commit later. Nor would silence discriminate if
   something produced it: a ceiling a sibling left on a ref and a ceiling your own committed entry put
   on `HEAD` print the same stderr, both without a note. The note goes quiet by itself once the entry is
   committed, because `HEAD` joins the union -- and durable includes yours, so a second entry in the
   same tick takes the next id instead of yielding the one already written.
   It is a read, not a reservation, and the second read sees only what moved **inside this clone**: a sibling that committed in another worktree counts, because worktrees share
   `refs/heads` (measured: `D-0002` before the sibling commits, `D-0010` after, with no fetch), while
   a sibling that pushed from a different clone does not, because an unfetched push never moves
   `refs/remotes` (measured: the same call answers `D-0002` while origin already holds `D-0007`, and
   `D-0008` one `git fetch` later). The supervisor now fetches origin once per activation, ahead of
   the queue read (`H-0075`), which is what makes a second machine's ids reachable at all -- but that
   is one fetch at one moment, and your tree may have been cut before it. So fetch it yourself as
   well, unrefspec'd: `git fetch --prune origin` updates every branch, while
   `git fetch --prune origin main` updates the trunk only and leaves the allocator blind to the branch
   a sibling pushed. When the journal holds `REFRESH-TIMEOUT` or `REFRESH-BLIND` for your tree, that
   activation dispatched on refs it could not check, and the re-read immediately before the entry
   commit is the only thing left between you and a duplicate id.
   Read the stderr line the allocator prints beside the id, not only the id. It says `REFRESH` with the
   age of the union behind the answer, or `REFRESH-BLIND` with the reason a sibling cut from another
   clone is not in that union at all -- a refspec that fetches one branch, or refs older than
   `FACTORY_ID_MAX_AGE` (300s). `REFRESH-BLIND` means fetch and re-read before you write the entry: run
   the `git fetch --prune origin` the line names, then take the number that comes back. That is the
   window #303 measured: two units were handed `D-0058` 2260s apart -- `06b7c29` wrote its entry at
   11:32:24-0500 and the second unit took its `pre-D-0058` checkpoint `4537b9d` at 12:10:04-0500, the
   moment its allocator ran -- because that allocator had a true answer about refs it could not see and
   said nothing about them. The marker
   never changes the id you were handed; it changes whether you may trust it. A clean read is not a
   promise either -- a sibling that pushed since the last refresh is invisible to any offline read -- so
   the merge stays the arbiter and the yield rule in `docs/factory.md` still applies.
2. **Checkpoint next**, always before the decision, never after:
   `git commit --allow-empty -m "chore(checkpoint): pre-D-<NNNN> <why>"`. That commit is the
   rollback target, and it is worthless if it is taken after the change.
3. **Choose by the tie-break order**, and stop at the first one that separates the options:
   1. the option that keeps `go.mod` free of `require` lines (#856 -- a dependency is not yours to
      add, and there is a gate that proves it);
   2. the option whose config decodes ambiguity **toward locking the child out** (#873, #872 -- an
      omitted key must never mean "off");
   3. the option that refuses rather than proceeding when input is missing or empty;
   4. the option that touches the fewest files and packages;
   5. the option that is reversible by `git revert` alone;
   6. the option the surrounding code already takes -- read two call sites before inventing a rule.
4. **Write the entry into the log of the tree you are delivering into**, in the same commit as what it
   decides: question, every option you considered, the one you took, why the tie-break selected it, and
   the checkpoint sha. Product work -- the daemon, the web UI, the capture driver, a test -- goes in
   `DECISIONS.md` with a `D-` id. Work on the delivery harness itself -- `factory/`, `.opencode/`,
   `scripts/check-*`, the timer -- goes in `factory/DECISIONS.md` with an `H-` id. The prefixes are not
   decoration: two loops once opened the same file at `D-0011` and each cited the other's numbers, so
   `main` carries `D-0011`..`D-0015` as page-design decisions while the harness decisions with those same
   numbers live in `factory/DECISIONS.md`. Use the allocator's prefix for the selected log;
   `scripts/check-decisions.py` refuses an id that appears in both logs.
5. **Carry on.** Do not mark a todo, do not wait, do not restate the decision as a question.

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

Then **escalate and stop this unit**, in this order. The order is the mechanism, not ceremony: the
record must be committed before the write that can be refused, and the write must be read back before
anything is allowed to claim it happened (#1180).

```bash
# 1. the comment, and the entry with `Chose: escalated` and `Awaiting:` naming the decision, committed.
#    An escalated entry carries `Checkpoint: n/a` -- it decided nothing -- so there is no checkpoint
#    commit to take first.
gh issue comment "$ISSUE" --body "…what is ambiguous, what each option costs, what you would
recommend, and the exact command to proceed…"
# write the entry into the log the ticket belongs to, then:
git add "$LOG"          # DECISIONS.md, or factory/DECISIONS.md for harness work
git commit -m "docs(decisions): escalate D-NNNN (awaiting a human)" -m "Decision: D-NNNN"

# 2. the marker, whose name comes out of its one home. A pull request takes THIS verb with its own
#    number -- `gh issue edit "$PR"` -- because `gh pr edit` cannot put a label on anything here, and
#    it fails for a reason that has nothing to do with labels. Measured below the block.
gh issue edit "$ISSUE" --add-label "$(factory/escalation-label.sh)"

# 3. the read-back, in the same breath as the claim. The read is the one place the `pr` verb is right:
#    `gh pr view "$PR" --json labels`.
gh issue view "$ISSUE" --json labels -q '[.labels[].name] | join(",")'
```

Step 3 decides what you may say. If the name is not in that output the marker is not on the artifact,
so say that on the issue, say it again in `NEXT:`, and stop. `gh` fails the write and exits 1 when the
name is not one the repository has -- measured 12 Nov on this box's gh 2.46.0:
`gh issue edit 311 --add-label zzprobe-does-not-exist` printed `'zzprobe-does-not-exist' not found` and
`failed to update 1 issue` for exit 1, and `gh issue view 311 --json labels` read back
`bug,area:ops,blocked`, unchanged -- and #311 has not yet said what the name *is*, so a refusal is a
result you report and not a step to retry harder. Doing the label last is what makes that refusal cost
one sentence instead of the record: under `set -e` a middle-of-the-step refusal used to kill the
escalation with the comment written, the label absent, and `Awaiting:` never committed, which is
indistinguishable from a unit that quietly gave up (#311, #315).

The verb is the other half of the step, and it is the half the harness left unsaid. Measured 12 Nov on
this box's gh 2.46.0, one 67-character probe name aimed at pull request #314 with its labels read
before and after: `gh pr edit 314 --add-label zzprobe-does-not-exist-and-sixty-seven-characters-long-so-nothing-can-land`
printed `GraphQL: Projects (classic) is being deprecated … (repository.pullRequest.projectCards)` for
exit 1, and `gh issue edit 314` with the same name printed
`'zzprobe-…' not found` / `failed to update 1 issue` for exit 1, and `gh pr view 314 --json labels`
read back empty in both cases -- nothing was written either way. Same exit, two different failures, and
only one of them is fixable by a person: gh 2.46.0 reads `projectCards` on its way into `pr edit`, so
that call dies **before** it resolves the label name, which means it cannot land a marker under *any*
name #311 might choose. `D-0047` measured the split from the other side -- `gh issue edit 240
--remove-label blocked --remove-label question` exited 0 and read back `labels: []` where `gh pr edit`
died on that field (`DECISIONS.md:2221-2231`) -- and it is the same reason `gh pr edit --add-reviewer`
is refused in Step 5. So: `gh issue edit` to write, on an issue or a pull request alike; `gh pr view`
to read.

Never invent a label name. Not by typing one from memory, not by substituting the nearest label that
exists, not with `gh label create`, and not by skipping the marker on the grounds that the comment is
enough. The name is `factory/escalation-label.sh`'s to print and #311's to settle; a reported refusal is
the honest end state of a step whose name is still owed, while an invented label is a marker that reads
as settled. Nothing outside that script's output may reach `--add-label`.

Do not pick a side and do not open a pull request for half of it. An escalated unit is an honest
end of a tick, not a failed one.

### When the ticket is already satisfied by `main`

A third thing can be wrong with a unit: the work is already finished. `factory/triage.sh` counts an
issue as claimed only by an **open** pull request -- GraphQL `pullRequests(states:OPEN,first:100)` and
its `closingIssuesReferences` at `:122`, `gh pr list --state open --json number,body` scanned for a
verb at `:155` -- so a merged fix claims nothing, and a body that names the issue with `Refs` (the
no-close-verb spelling #1256 requires) registers no closing reference either. The issue goes back to
`unclaimed`. Measured 8 Nov: #228's fix merged at 07:08:17Z and a tick was dispatched #228 at 09:35Z on
a branch with no commits ahead of `origin/main`, and the ceiling went to re-deriving what `main` had.

So read the ticket against the head before writing code, the way Step 1 reads a pull request rather
than remembering one. If every requirement in the ticket is met on `main`, the deliverable is the
evidence and not a diff:

The queue says when this case is possible. A `[merged:PR#…]` bracket in front of your ticket is a
REFUSED claim, not a verdict: a merged pull request named this issue, the classifier declined to read
that as delivery, and the bracket carries which pull request, on which branch, when it merged, and
which rule refused it. Read it before you write. `gh pr diff <N>` and `git log --oneline origin/main`,
then hold what that pull request changed against each requirement below and follow the five steps --
measure, do not quote the old body. The bracket does not make the ticket done, and it does not make it
not done: a body naming four issues does not say which four it finished, `Part of` says only that some
of it landed, and the branch in the bracket may belong to a different ticket entirely.

1. **Measure it; do not quote the merged body.** A pull-request body is a claim about code that has
   moved since. Run the probe it says it added, then break the thing it says is pinned and watch it go
   red. Assert each mutant's pattern actually applied before believing its tally: a pattern that is not
   found is a mutant that never existed, and its `SURVIVED` is not a result.
2. **Post the measurements on the issue**, including what they do *not* establish and whatever the
   merged entry already disclosed as unmeasured. Someone reads that comment to decide whether to reopen.
3. **Close it** with `gh issue close "$ISSUE" --reason completed`, then read `state` back from the API
   -- the read-back is the first call about a claim, never the confirmation after the announcement.
4. **File the unfinished part as its own issue**, with the file-and-line evidence and the measured cost.
   A finding that lives only in a comment on a closed issue is invisible to every queue that reads
   issues, which is the same rule that makes a branch with no pull request invisible.
5. Name `gh issue reopen` in the comment. It is the whole reason this ending is permitted.

What is dangerous about it, stated because the hazard is the rule: `triage.sh` reads *open* issues, so a
close takes the row out of the queue, and a close that is wrong deletes work nobody will look for again.
`git revert` does not undo it and nothing measures how often it is taken. That is why step 1 asks for a
measurement rather than a reading of the previous pull request, why step 2 keeps the not-established
part in the comment, and why a ticket you are not certain about stays open with a comment on it.

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
matches a closing keyword immediately followed by an issue reference, with no notion of negation:
measured on this repository, `**This does not close #10.**` closed issue #10 the second the merge
landed. For partial work write `Part of #10 only; the umbrella is not finished.` or `Refs #10`.

**And when the issue really is finished, put the verb where GitHub can see it: in plain text, touching
the number, never inside a code span.** A keyword wrapped in backticks asks GitHub for nothing. Measured
over all 180 merged pull requests here, read 14 Nov 2026 (`scripts/check-close-verb.py` ships the
classifier, and the count comes from that same pattern rather than a re-typed copy): a plain-text verb
closed its issue within two seconds of the merge **98 times in 100**, with nothing at all in the
three-to-120-second bucket; a verb inside inline code did it **0 times in 2** -- PR #308's
`` `Closes #304` `` left #304 to be closed by hand 6086 s later, and PR #338's `` `Closes #301.` `` sat
on a line stating the keyword was deliberate and left #301 open 14472 s past its merge, which is what
cost a second full unit on a ticket `main` had already delivered. `factory/triage.sh`'s claim pattern
takes a backtick as filler, so the harness reads a spanned verb as a claim anyway: the body looks
linked to every reader in this repository and does nothing to the artifact.

Two more things the parser does not do, both measured rather than assumed. It does not reach: the verb
has to touch the number (`Closes #10`, `Closes: #10`, `close, #10`), so do not write a sentence with
prose in between -- `Closes the write half of #10's remaining criteria` asks GitHub for nothing, and it
is the shape that cost a unit of work when the harness's own scanner claimed it. And it has no `-ing`
form: GitHub documents `close closes closed fix fixes fixed resolve resolves resolved` and nothing in
that list ends in `-ing`, while the harness's scanner had `closing` and `resolving` copied in from the
claim matcher. There is no specimen of either in these 180 bodies, so nothing here claims to have tested
GitHub with one -- the shipped pattern follows the documented set and drops both.

So do not eyeball this. Run the body file through the checker before you create the pull request --
it separates the four things that look alike and are not (a verb that fires, a verb on a line saying
`not` that fires anyway, a verb in a span that will not, and `Refs`/`Part of` that were never
closers):

```bash
python3 scripts/check-close-verb.py /tmp/pr-body.md    # exit 1 = something cannot do what it reads
gh pr create --base main --title "..." --body-file /tmp/pr-body.md
```

Everything in this page written inside a code span is a *quotation* of a form, which is the inert
place for one. Write the real thing in running text.

Do not try to verify the linkage afterwards, and do not reach for `gh api` to do it: `gh api` is denied
to this agent, and `gh pr view --json closingIssuesReferences` is not a field `gh` exposes (measured —
it answers `Unknown JSON field` and lists the fields it does have). A GraphQL read is the only way to
ask GitHub what a body closed, and that read belongs to the supervisor: `factory/triage.sh` reads
closing references plus verb-phrases out of every open body and treats either as a claim, which is why
writing the verb rule correctly the first time is the whole obligation. The supervisor asks the same
question a second time AT the merge: `factory_verify_closure` (`factory/factory.sh`) re-reads every
number the merged body's verb pointed at, compares the close timestamp with `merged_at`, and writes what
it found onto that ticket — a note in plain text when the ticket is still open past a re-probe, a
journal line when something else closed it. So a verb aimed at the wrong number is not a private
mistake: it becomes a sentence on somebody's artifact. If you are unsure whether a phrase will register,
do not write it: use `Refs #10`, and say in the pull-request body what is still unfinished.

Requesting the review is **not yours to do**, and the reason is measured, not stylistic: H-0010 found
`gh pr edit --add-reviewer` fails on this account with nothing but a `Projects (classic) is being
deprecated` line on stderr while `/requested_reviewers` stays empty -- a verb that exits 1 and looks
like it worked is how "nobody ever asked" came back after being fixed. The supervisor owns that path
(`factory_request_review`: the REST call, then a read-back of the same list), the raw API is denied to
this agent, and the supervisor pokes any open pull request whose reviewer list is empty whether or not
you say anything. So do not spend your ceiling on it, and do not report it as done.

What you owe the review is the rest of it: a body that says what changed and quotes the gate, and the
instruction that made the work non-obvious. The four pull requests that once sat in `REVIEW_REQUIRED`
for three days with an empty reviewer list are why the queue asks on its own now; a tick that
rediscovers that fact is a tick spending its ceiling on a command that cannot succeed.

**When the configured target is the wrong person, redirecting the ask is still not yours.** A unit
has no verb for it, and the narrow one that would give it one was weighed and withheld: a whole-line
allow for `gh api -X POST …/requested_reviewers` is method-blind, so the same pattern admits
`-X POST …/merge` -- the call `* pr merge*` exists to stop -- and that is how it was settled, in the
comment on the denial in `.opencode/opencode.jsonc` (`efa4e29`). Widening that table is a permission
grant on the harness, which `AGENTS.md` puts outside a unit, and `AGENTS.md` now names the component
that does the asking so the rule is not read as yours again. So the in-band verb is one sentence on
the pull request naming the keys the poke reads -- `FACTORY_REVIEWER_TEAM`, a team slug, or
`FACTORY_REVIEWER`, a login (`factory_review_target`, `factory/common.sh`) -- and whoever installed
the timer retargets its environment. Say it once, and do not call the ask missing: the supervisor asks
on its own schedule whenever the row reads `needs_reviewer` (no verdict that still stands and nobody
holding a request -- `docs/factory.md`), whatever your body says, and the poke does not un-ask what it
did not ask (H-0117), so whoever holds the request was placed there by that config or by a person who
knew better than the queue.

### The push may have spent the approval. Read the review state back.

Step 1's read is not the last one. A push into a pull request that already has a review voids the
approvals on it -- this repository does not carry a stale approval across a new head -- and it does it
without touching either command's exit status. Measured 10 Nov on #269, over one conflict-resolution
push (`c149fe9..ec74cf5`, a fast-forward, no force) and nothing else:

```
before  reviewDecision=APPROVED        hermesvicowa APPROVED   row at c149fe9
after   reviewDecision=REVIEW_REQUIRED hermesvicowa DISMISSED  the same row, same commit
```

A conflicting branch cannot be moved any other way here: squash and rebase merge are both disabled,
force-push is denied to you, and reverting the base is not an operation. So every tick that clears a
conflict on a branch that already has a review pays a re-review, and that is fine. What is not fine is
learning it from the supervisor's `BLOCKED` an hour later, with a body on the pull request claiming the
unit is ready.

So after the push, and before the hand-off, ask the same question again -- its own call, in the shape
Step 1 requires:

```bash
gh pr view "$PR" --json reviewDecision,reviews,mergeable,mergeStateStatus
```

No `--jq` on this one. `reviews` is an array of objects, #270 measures what a projection program costs
a tick on gh 2.46.0, and the `state` field is one glance at the rows.

When a row that read `APPROVED` or `CHANGES_REQUESTED` now reads `DISMISSED`, one line goes on the pull
request: who, the commit that review was filed against, and that `mergeStateStatus` reads `BLOCKED`
until a reviewer is asked again. The supervisor re-asks on its next activation -- ten minutes at the shipped
`FACTORY_INTERVAL`, bounded by the `FACTORY_REVIEW_POKES` floor, and `docs/factory.md` carries the
arithmetic -- which is why the sentence below says *yet*. If the pull request already exists, the body is rewritten with
`gh issue edit "$PR" --body-file <file>` -- `gh pr edit` fails on this account's retired
`projectCards` field while looking like it worked, the same failure as `--add-reviewer` above -- and
the read-back after a body write is the sentence being there, not the call returning. Do not ask the
reviewer yourself -- that stays the supervisor's path, above. Say in the body that the dismissal happened and
that nobody has been re-asked yet, and leave `NEXT:` in the hand-off saying the same, because a tick
that reports `verify: OK` beside a `REVIEW_REQUIRED` it never mentions has written exactly the claim
Step 1 exists to prevent.

The trap in that line's wording: a review row's `submittedAt` is when the review was submitted, never
when it was dismissed. #269's dismissed row still carries the approval's own stamp,
`2026-11-10T18:23:50Z`, so a dismissal dated from that field lands twenty minutes *before* the push
that caused it. There is no dismissal timestamp in any read available to a tick, so the honest sentence
names the push rather than the moment the approval died. `docs/factory.md` carries what the queue does
with a dismissal and how long the silence after one lasts.

## Before the ceiling: push, even if you are not finished

A tick's last act is a pull request, so never end with commits that live only in a worktree. If the
evidence you wanted has not landed and your time is nearly out, push the branch and open the pull request
anyway, with a body that says plainly which claims are still unmeasured and what would close them. The
alternative is the failure this harness was written to end: `TICK-DONE`, exit 0, and nine commits sitting
in one directory on a branch nothing references, while the queue hands the same issue to the next pass.
`factory.sh` checks for the pull request after every tick and logs `TICK-UNDELIVERED` when there is none,
and the unit stays where it was -- so an honest draft is strictly better than a tidy silence. Pushing a
branch and opening a pull request are inside your authority (#1823); merging is not.

## Step 6 — hand off

End with these lines, in this order, because the supervisor's log is the only place a human will
look:

```
PR:            <url or "none, escalated" or "none, #<issue> closed as satisfied by main">
GATE:          verify: OK   (or the exact failing line)
DECISIONS:     D-0007, D-0008        (or "none")
CHECKPOINT:    <sha>                 (the one to revert back to)
NEXT:          <the one thing the next tick should do>
```

If you got nothing pushed, say that plainly in `NEXT:` with the reason. A silent tick is the worst
outcome this system produces: it is indistinguishable from a machine that is working.

## What is never in a tick

- A new third-party dependency, ever, without escalating first (#856) -- the entry, the comment, the
  label, and the read-back that says the label landed.
- A label name of your own choosing. `--add-label` takes only what `factory/escalation-label.sh`
  prints, and `gh label create` is not a tick's move: what the escalation marker is called is #311's
  open question, and a label you invented reads as a decision somebody made (#315).
- `git checkout <file>` / `git restore` to undo a probe -- it reverts to HEAD and takes the whole
  uncommitted change with it. Keep a `/tmp` backup and restore with `cp`, taken **after** the fix.
- Committing `plock`. `git ls-files | grep -x plock` must print nothing before any `git add -A`.
- `git add -A` over a tree you have not looked at: `git status --short` first.
- A `.gitignore` rule offered as the fix for a tracked binary -- an ignore does not untrack a file.
- Editing `scripts/mutations/*.json` patterns to make a mutant match again. The mutant retired
  because the code changed; fix the anchor, and say so in the PR body.
- Touching `vendor/` or a submodule to get a build green: there is no `vendor/`.
- Working outside your worktree, or reading another unit's worktree to "help".
