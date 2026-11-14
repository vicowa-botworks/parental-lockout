# The delivery factory

How this repository's issues get to a merged pull request without anyone remembering to ask.

Read this if you have ever found an open pull request with green checks that nobody had looked at,
or an issue that a closed-over branch had already fixed. Both happened here on 4 November 2026; the
numbers are in `factory/DECISIONS.md` (this log's own ids are `H-`; the repository's are `D-`).

## The one idea

**The loop is not in the agent.** A model session is a disposable worker; the thing that owns the
queue is a shell script started by a timer. When the session dies, one unit of work is lost and the
next activation re-derives the whole world from GitHub and git. Every design where the loop lives
inside a session has the same failure: the session ends, and the queue stops.

```
systemd timer (or `while :; do factory.sh; sleep 600; done`)
   └── factory/factory.sh            ← owns the loop, holds the lock, budgets the pass
         ├── factory/triage.sh       ← what needs something, read from the API on this run
         ├── mechanical actions      ← request a review, merge, merge main in, post a notice
         └── opencode run --auto --agent factory     ← one unit, one worktree, one ceiling
```

## Try it without touching anything

```bash
factory/factory.sh --dry-run            # every action printed, nothing written
factory/triage.sh --table               # just the verdict, for a human
factory/triage.sh --pr 206              # why this one is sitting there
factory/factory.sh --status             # halt state, which code this is, and the ledger
```

`--dry-run` is a property of the whole script, not of one call site: no GitHub write, no git write, no
model, and no ledger. It is the mode to reach for first, and it is what found three of the bugs in this
directory (`gh api` returning a `{"message":…}` 404 body as if it were data, a `.git` suffix leaking
into the repository name, and a `case` arm eaten by an unbalanced quote).

The ledger had to be added to that list because it was the exception: the two writers in `common.sh`
checked nothing, so a dry run stamped `first-seen`, `stuck-since`, `last-notice` and `conflict-at`
while printing `DRY-RUN` for everything else. A ledger file is not bookkeeping, it is a timestamp with
consequences -- `first-seen` decides queue order, `stuck-since` starts the escalation clock,
`last-notice` throttles comments at a maintainer -- so one rehearsed pass made the next real pass quote
a wait that never happened and stay quiet at somebody who had never been contacted. `log_event` is
deliberately *not* suppressed: a dry run you cannot audit afterwards is a dry run nobody trusts
(H-0017).

## What each class means

`triage.sh` prints one row per thing that needs something; `factory.sh` acts on the class.

| class | what it means | who acts |
|---|---|---|
| `merge_now` | approved, `MERGEABLE`, `CLEAN`, every check passed | supervisor, in bash, after re-reading all four from the API |
| `needs_reviewer` | no verdict that still stands -- no rows at all, or every `APPROVED`/`CHANGES_REQUESTED` row voided by a push, or comments only -- **and** `/requested_reviewers` has neither `users` nor `teams` | supervisor: ask the team, at most once per `FACTORY_REVIEW_POKES` |
| `conflict` | `CONFLICTING/DIRTY` | supervisor: `git merge origin/main`; a tick only if that conflicts |
| `address_review` | newest `CHANGES_REQUESTED` is newer than the head commit, **and its body carries an ask** | a tick, and the ask rides in its instruction |
| `review_unreadable` | that same change request, with nothing in its body: empty, a bare path, an `@`-mention, or under 40 bytes | nothing: one throttled comment says what arrived, and the row waits |
| `checks_failing` | a check concluded failure | a tick |
| `unclaimed` | open issue nothing claims -- no closing reference from an open pull request, and no claim verb in an open body. A merged body that named it and was refused rides along as a `[merged:PR#…]` marker | a tick |
| `delivered` | an **issue**: a merged pull request claims it, it landed on `main`, the issue has said nothing since, and no earlier closure was rejected. A **branch** (#291): a merged pull request ran under that name into `main`, and the ref holds no content the trunk lacks. Either way the row names the pull request, **the branch that carried it**, the instant and the words | nothing: the supervisor logs it, and closes nothing |
| `branch_without_pr` | **content** ahead of `main` -- at least one file the trunk lacks, measured from the merge base -- and no open pull request | a tick, to adopt or refuse the stranded work |
| `awaiting_review`, `awaiting_merge`, `checks_pending`, `draft` | a wait, not a failure: a verdict still stands, or somebody already holds the request | nothing, except the throttle and the daily notice |

`delivered` is the fourth class the queue could not see, and it was measured twice in one day before
it was written (#235). Both claim channels read *open* pull requests only, so a merged one claims
nothing whatever its body says, and an issue whose fix is already on `main` goes back out as
`unclaimed`. #228's fix merged at 07:08:17Z and a tick was dispatched for #228 at 09:35Z on a branch
with nothing ahead of `origin/main` -- the ceiling went to re-deriving what `main` already had
(H-0050). Eight hours later #232's fix merged at 14:26:31Z as #234, whose only reference is `Refs
#232`, and the queue printed `issue 232 unclaimed` on a tree containing the fix.

The claim verb is not the verdict, and that is the finding rather than a caveat. Read against the live
open issues, PR #229's `Refs #199. Refs #204. Refs #207. Refs #213.` is one body pointing at four
issues, two of which its own "What this branch does not finish" section disclaims; #218's body carries
"Not here: #149's per-control commit policy", which is a cross-reference with no verb on it at all;
and #227's `Refs #222` is a merged fix to an issue that was closed on 6 Nov at 13:17:20Z and
**REOPENED** at 16:27:23Z with a corrected row. #222 and #232 are identical in every way the verb can
see -- one reference each, the same head-branch shape, silence on the issue after the merge -- and only
`stateReason` separates them. So the verdict is built out of five facts GitHub already holds, and
never out of a sentence in a body about what is finished: the body claims exactly one issue, it did not
use this repository's own spelling of partial work (`Part of #N` -- H-0020, and #1256 for why an
adjacent inflection of "close" is never a harmless way to say it),
the merge reached `FACTORY_BASE`, the issue has not been updated since the merge, and no closure of
that issue has already been rejected. The last of those is also the human's override in the other
direction: one comment on the issue puts it back in the queue for a tick.

Precedence is unchanged -- `delivered` is reached only where nothing open claims the issue -- and that
has a consequence worth writing down before the class surprises anybody, because the open-claim channel
drops an issue named *anywhere* in any open body, not only one a pull request claims. Running the
shipped `triage.sh` against GitHub at 15:22Z on 8 Nov, #232 and #222 printed no row at all: the first is
named by the pull request that fixes it, the second by draft #233, whose body says `Refs #220 and #222`.
The issue rows that pass printed were #237, #213 and #199, every one `unclaimed`, the latter two held
out of `delivered` by the claim count. So `delivered` means "the queue will not work on this, and here
is the merged pull request it is leaning on", and it does not mean "every finished ticket appears
here" -- an open body naming an issue hides it from both classes at once.

Both reads are bounded, and a bounded read says so rather than answering as if it had seen everything.
The GraphQL call asks for the first 100 open pull requests and the first 100 open issues, so each
connection is also asked for `pageInfo{hasNextPage}`; either one answering `true` skips the whole issue
half for that pass with a line naming the pagination bound, because a truncated issue page is not a
smaller verdict but a `stateReason` veto that went missing -- and a claim with its veto gone reads
`delivered`, which is the one error this class was built not to make. A missing `pageInfo` reads as
truncated, not as `false`. The other bound is the open-issue listing itself, at `--limit 100`: past it
an issue gets no row of any class, so the pass counts the rows it read and warns at the limit. Measured
against GitHub at 16:29Z on 8 Nov on eleven open pull requests and three open issues, both connections
answered `false` and neither warning printed; neither is due to go quiet on its own, and a queue that
has outgrown its read should say so in the log rather than look finished.

A claim is a verb plus **every** number still in the same list, which is a rule that only measurement
would have produced. #221's shipped body has `Refs #219, #220, #204` on one line with
`(Part of #219 only; the umbrella is not finished …)` eight lines above it, and open PR #233 says
`Refs #220 and #222`; a verb plus ONE number reads either line as a claim on its first issue, which on
the merged channel turns `Closes #10, #11` into a sole claim on #10 -- the bundle this class refuses,
arriving through a comma. Two sentences about two issues are still two sentences, because the second
number has neither a verb nor a list marker in front of it, and that is what stops "Not here: #149"
from retiring #149 out of the queue.

A claim the arms refuse is reported, not thrown away. When the count rule or `Part of` refuses a merged
body the issue keeps its `unclaimed` row and its tick -- both rules exist to stop a bundle over-claiming,
and #246 is in the queue today because `Part of` told the truth about work nobody has finished -- but the
row now opens with `[merged:PR#229 on \`issue-199\` at 2026-11-08T01:33:46Z -- names it in a 4-issue
body]`. That marker is evidence handed to whoever reads the row and it is never a verdict: it changes no
class, dispatches nothing extra, closes nothing, and the branch it names is a fact about the merged body
rather than a reason to decide anything. Two tickets can share a branch -- #229, #240 and #243 are all
`issue-199` -- and a branch can carry a body that only partly finished the ticket it is named for: #254
merged on `issue-246` under a `Part of` that #246 is still open to complete.

What the marker replaces is silence. A tick used to arrive at #235 with no way to know the question had
already been asked of PR #238's body, and it spent seventeen minutes of ceiling -- `TICK-START
unclaimed:235 2026-11-10T22:57:23Z` to `TICK-DONE 23:14:23Z` -- re-deriving what the classifier had
already read and discarded. So the refusal is handed down instead: `factory/factory.sh`'s `unclaimed`
arm matches the marker at the START of the detail, against the same literal the scan writes
(`FACTORY_MERGED_MARK` in `factory/common.sh`), and adds the instruction that tells the tick to read
that pull request's diff against the ticket's requirements before writing anything, and to use the
"already satisfied by `main`" protocol if every requirement measures as met there. Anchored and not a
substring, because the rest of the detail is an issue title somebody typed and a title must not be able
to buy itself an instruction. An issue that has spoken since the merge, or whose closure was once
rejected, is given no marker: in both cases a person has already read exactly that, and saying it again
is noise. Both ends are one change and `factory/selftest.sh` says so -- the row that reaches the table
with the note, the arm that reads it, and the two documents that explain it. (H-0108.)

The read itself is one bulk call -- `gh pr list --state merged --limit $FACTORY_DELIVERED_WINDOW
--json number,body,mergedAt,baseRefName,headRefName` -- because the per-PR shape the channels above use
would cost one call per merged pull request. The window is a bound on the read, not on the claim: an
issue whose fix landed outside it keeps the verdict it always had, `unclaimed`, and the row prints the
window it
leaned on so a stale-looking verdict can be re-checked by raising it. It is a bound on the read and also
on what a single pass costs, because merged bodies are big: 100 of them were 632,601 bytes on 8 Nov,
which is why the payload is handed to `jq` on standard input rather than as an argument -- Linux
refuses any one argument over 128 KiB, and a scan that cannot run must say so rather than answer
`{}`, which is the same shape as "nothing has been delivered".

The supervisor's arm is one `log_event` line. It starts no tick, writes no ledger row, and closes
nothing -- closing is a human act on this repository's tracker, and the harm of a false `delivered` is
a ticket nobody looks at again, which is why the row is reported rather than closed or dropped. The
ledger is not consulted: it answers "was this ever worked" and keeps answering it after the work
merges, which makes it exactly the wrong thing to ask when the question is "does anything still claim
this?"

Two of these are the reason the whole thing was worth building. `needs_reviewer` exists because
`gh pr list`'s `reviewRequests` lists only *outstanding* requests, so it cannot distinguish "never
asked" from "asked and answered" -- measured, four pull requests had an empty list *and* an empty
`/pulls/N/requested_reviewers`, three days after opening, with every check green. `branch_without_pr`
exists because a branch with commits and no pull request is invisible to every queue that reads
issues and pull requests; the first run of this script found five, one of them seven commits deep.

### Which unit a pass takes first (#307)

Within a class the queue is oldest-first, and the key that decides it counts **up**, so it sorts
descending: `factory_age_at` answers elapsed seconds since `first-seen`, measured against the one clock
reading the ranking pass takes before it ranks anything, so `sort -t$'\037' -k1,1n
-k2,2nr -k3,3n` puts the longest-waiting unit first and the number breaks the tie. The key shipped
ascending from `88848c5` until `H-0135`, which is `H-0007` written backwards -- the row stamped most
recently went first. On a live queue that is not a fairness problem but a starvation one, because every
pass mints a younger `first-seen` for whatever arrived since. Measured off `factory/log/supervisor.log`:
the pass at 20:22:20Z on 11 Nov printed `NEXT unclaimed 303 age=0s` -- a unit its own stamping loop had
written `first-seen` into one second earlier -- while #296 and #297, first seen at 16:13:25Z and 14,935s
old, sat in the same rank; all five `unclaimed` rows were still open two hours later
(`factory/triage.sh --table`).

Nothing in the suite noticed for seven days, because the fixture stamps both of its units inside one
second, and equal ages fall to the number tie-break -- which sends unit 900 first, the same answer the
correct key gives. What surfaced it was a second boundary. This page said for three days that the
boundary fell **between the two stamps** -- "stamped 900, did the worktree lookup and the `gh` shims,
and stamped 901 in the next second" -- and that story cannot invert anything: the stamping loop runs to
completion before the ranking loop reads a single age, so a stamp that lands a second later is a second
*younger* at every read, which is the order the key wants. The boundary is between the two **reads**.
`factory_age` called `date -u +%s` inside itself, so with both stamps in one second the first row
answered `0s` and the second `1s` whenever a boundary fell in the ~10 ms between the two calls, and the
descending key ranked the younger unit first. That is what CI run 43105189339 printed (`quarantine at
line 12, tick at line 8`), what run 43532152285 printed again on 14 Nov (`quarantine at line 14, tick at
line 10`, three days later and the same one line red), and what #335 caught under load on a
developer box with the arithmetic visible in the journal: `NEXT unclaimed 901 age=1s` above
`NEXT unclaimed 900 age=0s`, for two stamps one pass wrote 900-then-901.

Planting ages was the right repair for the legs that plant them, and no repair at all for the legs that
do not: `…then starts a tick for unit B` and #335's guard control both read the unstamped fixture, so
both stayed a coin flip landing about once in 200 passes. `H-0157` closes it at the source -- the
ranking pass takes one `date -u +%s` after the stamping loop and measures every age against it, which
makes the key a monotone function of `first-seen`: equal stamps tie, the number decides, and no interval
between two reads can turn a later stamp into an older one. `factory/selftest.sh` still plants the two
`first-seen` values and asks in **both** polarities -- one polarity alone is also satisfied by a
dispatcher that sorts on the number and never reads the age -- and retires the `r` in a control, so the
ok means the direction rather than the tie-break. Three more legs hold the clock itself: the boundary is
now *scheduled* by a stub `date` that advances one second per reading (a fixture cannot schedule it,
which is why this defect needed two CI runs to appear), the retired per-row reader is made to reproduce
the red pair on demand beside it, and a ranking site that goes back to reading the clock per row is
refused by the text, with that reading planted as the control.

The sentinel is the one behaviour that changes on the way. A unit with no `first-seen` answered
999999999, which used to sort last and now sorts first -- what `factory_age_at`'s own comment has always
promised ("never" means "ready"). It cannot reorder a queue whose ages are real, because the loop that
builds the rows stamps every workable unit before the loop that ranks them reads the age; the only paths
that skip a stamp are a dry run and a ledger that will not take a write, and both skip it for *every*
row, so there the rows tie on the sentinel and the number decides exactly as before.

### What a branch has to hold to be offered (#291)

Commits are not content. `rev-list --count` moves by one when `git commit --allow-empty` moves the
tree by nothing, and Step 3 of `.opencode/skills/factory/SKILL.md` makes every tick take exactly such a
commit -- the `chore(checkpoint): pre-H-…` rollback target -- *before* the judgement call it may never
reach. A tick killed by the wall-clock ceiling between the checkpoint and its effect commit therefore
leaves a ref ahead of `main` whose whole delta is zero bytes, and the arm at `factory.sh:675` announces
"The work is already written" and spends a model on it. Measured 11 Nov on `issue-249`, one commit
ahead: `git show --numstat` printed no file list and `git diff --stat c98f75f f64ddcd` printed nothing.

So the row is bought with a content question: `git merge-base origin/main <branch>`, then `git diff
--name-only <base> <branch>`, and a branch that changes no file is not offered. Three things are the
rule rather than the one-liner, and each has legs in `factory/selftest.sh`:

* **From the merge base, not from `main`'s tip.** The branch above had also merged the trunk in, and
  between its merge base (`c98f75f`, the head of the pull request that had already delivered it) and
  `main`'s tip sat 187 more commits. Ask the same question of the tip and that zero-byte branch reads
  as **21 changed files** -- other people's work, billed to it. Measured, not reasoned: 0 files from
  the base, 21 from the tip, at `origin/main` `a0c7fce`. Control 4 in the fixture asks it the wrong way
  and demands the branch come back.
* **A deletion is content.** `--name-only` lists removals, so a branch whose only act is deleting a
  file keeps its row. A verdict that counted added bytes would lose a person's work exactly the way the
  count lost a checkpoint's.
* **A read that failed is not a verdict.** No merge base -- unrelated history, or a git that refused the
  diff -- keeps the row, with `content not comparable to main` printed in its place and a warning naming
  the branch. Ambiguity about a missing input goes toward looking at the work, the same direction
  #873 takes for the config decoder.

What is dropped is said out loud every pass that still sees it -- `triage: 2 branch(es) ahead of main
hold no content the trunk lacks, so none is offered: br-empty br-merge-empty` -- on stderr rather than
as a row, because a row is an offer and there is nothing to offer, and because a class that quietly
stops printing rows is the failure `factory.sh` calls "a queue nobody can audit". Since #291 the
sentence carries only the refs nothing else can account for: a content-free ref that a merged pull
request names is reported by number as a `branch delivered` row instead (below), and the branch is left
out of this tally so one pass does not describe one ref twice in two sentences that can disagree. The dangerous edit here
is one character: "skip branches with no content" becomes "skip branches" and every real stranded branch
leaves the queue with the fake ones. That is why the fixture plants `br-work` and `br-delete` beside
`br-empty` and demands they stay, and why a control deletes the content test and demands the empty
branch returns.

The detail reports what was measured now -- `14 file(s) across 3 commit(s) ahead of main, no open PR
(local only)` -- because "2 commit(s) ahead" was a number that could be true about nothing. Running this
against GitHub and this machine's refs at 13:21Z on 11 Nov moved no row: all eight `branch_without_pr`
rows carry content (58, 15, 14, 6, 4, 4, 2 and 1 files) and none was empty. The live instance of the
defect, `issue-249`, had acquired a real commit of its own -- H-0130's -- two hours before this was
written. The fix is for the next one.

**A ref whose pull request merged reports the delivery (#291, ask 2).** GitHub deletes a branch when its
pull request merges only if the repository asks it to; this one does not, so the ref survives -- usually
at zero commits ahead, and often as nothing but the `--allow-empty` checkpoint that Step 3 takes before a
judgement call. Such a ref belongs to neither half of the classifier. It is level with the trunk, so the
arm above never asks it a question; and the scan that reports a delivery is keyed to `kind == issue`. It
was continued past in silence, which is the sentence #291's last comment names as still owed after
PR #293: "`issue-249` is now silent, which is the other half of #291: *a merged pull request's leftover
ref must be reported as delivered, not silently dropped*". The arm now reads the newest merge that ran
under the name off the `merged_json` the pass already holds -- `baseRefName == FACTORY_BASE`, so a pull
request merged into another branch delivers nothing (H-0103's stacked chain) -- and emits

```
branch  issue-249  delivered  merged PR #293 on `issue-249` at 2026-11-12T18:29:26Z, the ref holds no content the trunk lacks, and 2 merged pull requests ran under that name (window 100)
```

The row is bought by the same content question the arm above asks, and that conjunct is the whole design:
a name is evidence and never a verdict (H-0108, H-0148). `issue-301` is the counter-example sitting on
this repository -- 4 files across 2 commits ahead of `main`, and PR #312 merged under that name at
17:37:23Z on 12 Nov -- and an arm that stopped at the name reports it `delivered`, after which no queue
offers its real work again. Three controls hold the three ways that happens: the merged test inverted
(the silence returns, and the tally names the ref again), the base filter off (a stacked branch is buried),
the content conjunct off (`br-reopened`, merged once and re-cut with a file the trunk lacks, is buried).
No new supervisor arm was needed and none was added: its `delivered` arm keys on the class, so the row is
logged as `DELIVERED branch/issue-249 … reported, not worked` and nothing is dispatched or closed -- three
legs run the supervisor over a fixture whose merged pull request sits on `issue-902` and read that line,
the absence of a tick, and the absence of any write to the queue.

Measured against GitHub and this machine's refs at ~21:00Z on 12 Nov, both ends from the same tree: **32
rows before the change, 69 after**. The 37 that appear are all `branch delivered`, every one of the 11
`branch_without_pr` rows is still offered, and the tally falls from three refs to two -- the one of them
a merged pull request names is now reported by number. Thirty-seven is what #291's own census predicted
(34 leftover refs, 33 silent, counted 11 Nov), and it is also the price: a supervisor pass wrote 13
journal lines the day before this paragraph and writes 50 after it. Nothing in the harness bounds that
file or the transcripts beside it -- `log()` appends to `$FACTORY_LOG/supervisor.log`
(`factory/common.sh:340`) and no code rotates, truncates or prunes it, nor the `<stamp>.<unit>.log`
transcripts one per tick (`factory/factory.sh:766`). Measured in the primary checkout at 22:05Z on
12 Nov: **20 MB across 100 files** under `factory/log/`, oldest 10 Nov, individual transcripts up to
480 KB, `supervisor.log` at 2,397 lines. With 19, 47 and 20 passes journalled on 10, 11 and 12 Nov,
this branch adds roughly 700 to 1,700 lines a day to a file nothing prunes. Filed as its own ticket.

Two claims in the ticket measured false and are not implemented: an empty ref at zero ahead is not
`branch_without_pr` (the arm above refuses it, #292), and its census of 34 leftover refs was 37 by the
time it was counted.

### When the issue half knows a pull request exists (#291)

Two ways the issue half reads a pull request and says nothing about it, both found while working the
branch scan above, and both fixed by saying more rather than by moving a class.

**`delivered` names the branch.** The row used to give the pull request, the instant and the phrase,
although the same read has carried `headRefName` since #277. A branch is deleted when its pull request
merges, so the pass that reports delivery is the last moment the name exists: `issue-249`'s tip was
unreachable six hours after #252 merged, and #291 had to be written from a commit id. The row now reads
`merged PR #280 on \`issue-279\` at 2026-11-11T02:05:20Z says \`Refs #279\``, and the name comes out of
the same row the claim came out of -- never assembled from the issue number, which is the inference this
repository has been burnt by twice (#254 merged on `issue-246` under `Part of #246` with #246 unfinished;
#253's third instruction was reported delivered by #255 and was not on `main`). The legs plant a merged
pull request sitting on `fix/weird-branch` claiming #212, demand that name in the row, and run a control
copy that builds the name from the number: it prints `issue-212`, and the leg fails on it. A merged row
whose head branch GitHub no longer names says nothing about a branch and keeps its `delivered`.

**An issue only a draft names is named out loud.** Any open pull request whose body claims an issue
takes that issue out of the table entirely (`triage.sh`'s `claimed_issues`, by design: the open channel
outranks a merged claim, #216's stacked chain). While the pull request can still merge, the silence ends
when it does. A **draft** never merges, so the silence has no end. Measured on the live queue at 13:36Z
on 11 Nov, three open issues have no row of any class and nothing to look forward to:

| issue | the only thing naming it | what that pull request says about itself |
|---|---|---|
| #241 | draft #285 on `backup-tip`, `Refs #241` | title: `wip(factory): the pre-rebase copy of what #282 delivers -- reference only, do not merge` |
| #284 | draft #285 on `backup-tip`, `fix in #284` | the same |
| #222 | draft #233 on `wip/issue-220-refused-frames`, `Refs #222` | title: `wip(capture): the preserved refused-run implementation of #220 -- reference only` |

The first row is the one that settles it. **#241's fix merged as PR #282 at 11:01:37Z on 11 Nov**, two
hours before the pass above, on branch `issue-241` into `main`, under a body carrying exactly one claim --
`Refs #241.` -- with the issue silent since 02:22:33Z that morning and never closed, which is every input
the delivered scan asks for. So what the draft is suppressing is not a bare `unclaimed` row that might
have been worked, it is the delivered verdict the scan already computed. #285 is the pre-rebase copy of
the very work that closed the ticket, and its existence in the queue is what hides that the ticket closed.
Reading that silence as "nothing to do" is the #235 failure arriving by a new route. The pass now says this, on
stderr, every pass it is true -- the measured output, 13:36Z on 11 Nov:

```
triage: 3 open issue(s) get no row of any class because the only pull requests naming them are drafts: #284 (named only by #285 on `backup-tip`) #241 (named only by #285 on `backup-tip`) #222 (named only by #233 on `wip/issue-220-refused-frames`) -- a draft never merges, so neither scan reports these again
```

It is a sentence and not a class, for the two prices #277 measured: a class that starts a tick at an
issue whose work is sitting in a draft re-implements the draft, and a class that starts no tick retires
real work. Whether #241 should be closed and #285 deleted are judgements about somebody else's in-flight
pull request, and they are the operator's -- the harness's failure was hiding the pair, not failing to
resolve it. Four answers are "nothing said", and each is a different reason: the pull request is not a
draft (it can still deliver); GitHub's own `closingIssuesReferences` links it (a draft today, work in
progress); a non-draft pull request names the same issue anywhere (one real claim outranks a tombstone);
or the issue was never named at all. A control deletes the filter that separates a draft from a
mergeable pull request and demands the sentence fire on the one that can still deliver, so the four
`ok` lines above are that filter deciding.

Two claims in #291 that this section does *not* implement, because both measured false. It asked for
`headRefName` to be added to the merged read -- it has been in that field list since #277
(`triage.sh:492` at `a0c7fce`). And its live instance, "PR #280 is open right now and `triage.sh` cannot
know it, so #279 reads `unclaimed`", describes the state before 02:05:20Z on 11 Nov: #280 merged then,
its branch was deleted, and #279 now reads `delivered`, named by exactly the read the ticket said could
not see it. What was left standing is the half about not silently omitting a pull request, and it turned
out to point the other way -- the omission was never the merged scan missing an open pull request, it was
the open scan swallowing an issue a draft had claimed.

**Who the `needs_reviewer` row asks.** The poke addresses a TEAM (`FACTORY_REVIEWER_TEAM`, a team
slug), because a request is a question to whoever can answer it and a team request notifies every
member: the first one online replies, while an individual request waits for one specific person to
appear. Read the evidence for what it is, because it is thinner than the change: #240, the pull request
#241 was filed against, merged at 19:50:20Z on 8 Nov carrying four reviews and all four came from
`hermesvicowa` (18:19:17Z, 18:41:12Z, 19:06:38Z, approved 19:49:56Z), and the team's second member is
the org's machine user. Nothing measured that day shows a team producing a *new* name. The claim is
#241's and it is narrower: the poke should not have to guess who is online in order to reach everybody
who could be. An individual stays available as the override for a repository with no org
team: set the team empty and name the login. The classifier counts both lists, `users` and `teams`, so
a request the supervisor already made reads as `awaiting_review` rather than being poked again as
`needs_reviewer`, and the journal says which kind was asked (`POKED pr:<n> asked team workers`,
where the older line read `asked <login>`) instead of printing a login whatever was POSTed. Two things
the poke does not do: it does not ask a
draft (`#230` and `#233` are drafts on purpose -- a draft pull request is the author's claim channel,
not a review request, and the classifier routes it to `draft`, which is a wait and not a poke), and it
does not cancel an individual request it did not make (H-0117) -- the person who holds it may hold it
because somebody with better knowledge asked them.

### The approval a push voids

`reviewDecision` is a fact about a pull request's head, not a record of its past. This repository does
not carry a stale approval across a new commit, so a push into a reviewed pull request turns its
approval into a `DISMISSED` row -- and a conflicting branch here cannot be moved any other way: squash
and rebase merge are both disabled, force-push is denied to every agent in this repository, and
reverting the base is not an operation. A voided approval is therefore the ordinary end state of a
conflict-resolution tick, not an edge case. Measured on #269 on 10 Nov across one fast-forward push
(`c149fe9..ec74cf5`) and nothing else:

| | `reviewDecision` | the only row in `/pulls/269/reviews` |
|---|---|---|
| before the push | `APPROVED` | `hermesvicowa APPROVED` at `c149fe9` |
| after the push | `REVIEW_REQUIRED` | `hermesvicowa DISMISSED` at `c149fe9` |

A `DISMISSED` row is still a row, and `needs_reviewer` used to be decided by *how many* rows there
were. So the queue watched the unit it had just asked about go unserved, in its own journal:

```
17:58:17Z POKED pr:269 requested review from hermesvicowa    no rows at all: it asked
19:17:23Z WAIT  pr:269 awaiting_review                       one voided row: it did not
```

The first line is that journal line as it read on 10 Nov, before #241. The poke then printed
`$FACTORY_REVIEWER` whatever it had actually posted; it now prints the target it resolved, so the same
event reads `POKED pr:269 asked team workers` -- see *Who the `needs_reviewer` row asks* above. The
quotation is kept in the older spelling because that is what the box wrote that day.

The second line is the defect. One `DISMISSED` row, an empty `/pulls/269/requested_reviewers`,
`reviewDecision=REVIEW_REQUIRED`, and the count of rows answered "somebody has reviewed this". #269
was re-approved at `19:50:20Z` against the new head and merged at `20:18:19Z` because a person looked
at it; nothing in the queue asked, and nothing in either read-back says so.

So the arm now asks *is there a verdict that still stands* rather than *are there rows*. A verdict is
`APPROVED` or `CHANGES_REQUESTED` -- a state that says somebody formed an opinion about the code -- and
neither `DISMISSED` nor `COMMENTED` is one. With no surviving verdict and nobody holding a request the
row is `needs_reviewer`, and its detail names which of the three states it reached that conclusion
from: never reviewed, comments only, or the reviewer and the commit a voided verdict was filed
against. Asking is the cheap direction to err in -- the poke is throttled to one per
`FACTORY_REVIEW_POKES` and GitHub answers 422 for a request that is already outstanding -- and it is
the same `POST …/requested_reviewers` the web UI's *re-request review* button issues. Whether GitHub
accepts that call for a reviewer whose verdict was just voided is **not measured here**: #269 was
re-approved by hand before any activation could ask. If it is refused the arm logs `POKE-FAILED` and
stamps the throttle, which is a journal line naming a refusal rather than the silence it replaces.

What this does not do is make the re-ask instant, and the gap is worth knowing the shape of, because
it reads as a stalled queue. A poke is a supervisor act, so it lands on the *next* activation: ten
minutes at the shipped `FACTORY_INTERVAL`, or up to `FACTORY_REVIEW_POKES` (30m) when the pull request
was poked recently and is still inside that throttle window. Until then the unit is genuinely
unserved, and that is the tick's own half of this: `SKILL.md` Step 5 makes a tick read the review state
back *after* its push and write the dismissal into the pull-request body, so whoever is reading the
pull request learns in one line what the queue will not say for another ten minutes.

### The ask, and what happens when there is not one

`address_review` used to be decided from three fields -- reviewer, state, timestamp -- and the body was
projected away before anything read it. So a tick was dispatched at #240 with an instruction that said
*review asked for changes at 18:19:17Z, after the head commit at 17:24:05Z* and nothing else, because
the review's entire body was the 21 bytes `@/tmp/pr240-review.md`. It was filed with
`gh api -X POST …/pulls/240/reviews -f body=@/tmp/pr240-review.md`, and `-f` is `--raw-field`: it sends
the bytes after the `=` literally and reads no files, so GitHub stored the reference as the review and
answered **200 with a healthy-looking review row**. `gh pr review --body @file` fails the same way one
layer up, and only `--body-file` works on either path. The worktree, the ceiling and the model went to a
verdict with no ask in it (#242).

The row now has a seventh column, `ask`, and a verdict that decides whether it is fillable. Both come
out of one function, `factory_ask_verdict`, so the sentence written on a pull request and the
classifier that decides whether to spend a tick on it cannot disagree about what counts as an ask:

| body | verdict | why |
|---|---|---|
| empty, or nothing but whitespace | `unreadable:empty` | there is nothing to answer |
| no whitespace anywhere, and it contains `/` or `@` | `unreadable:reference` | it is a path or a mention, not a sentence -- the mistake, whatever its length |
| under 40 bytes | `unreadable:below-floor` | too short to be a review anyone could answer |
| anything else | `readable <n>` | it is an ask, and it rides in the tick's instruction |

Every number there is measured rather than chosen, from the 86 `CHANGES_REQUESTED` reviews on this
repository's 200 most recent pull requests (read 8 Nov): exactly one body is under 40 bytes -- the
21-byte reference above -- and the next smallest is 1002, so the floor retires no ask anyone has written
here. The longest is 14863 bytes; 12000 carries 83 of the 86 into a dispatch complete, and the three that
do not fit arrive cut, with the total and the command that reads the rest named in the row. A cut is
never silent, because a silent cut is a tick that answers 12000 bytes and reports the ask as answered.

The cost of a byte floor is stated where the floor is: a genuine 21-byte ask in ordinary English --
`Rename it and ship it` -- is refused exactly as the 21-byte path is. A selftest leg pins it, so the day
somebody softens the predicate, the leg that goes quiet is the one that knows about the cost.

The refusal is the half the ticket's own proposal would have missed. Handing the ask to the tick fixes
the dispatch that arrives; it does not fix the dispatch that should never have started, because a tick
cannot tell *"the reviewer asked for nothing"* from *"the reviewer asked for something I cannot see"*,
and the second reads exactly like the first when it is time to write a summary. This repository has
measured what happens next more often than anything else -- `D-0046` is five invented citations in
three published artifacts. So a change request with no ask in it gets **no tick**: the supervisor posts
one comment naming the review, its size, its shape, the text that arrived and the single command that
re-files it, throttled to once a day by the same clock as the daily notice, and the pull request stays
in the queue indefinitely -- `WAIT`, then `NOTICE`, never parked (H-0051). Re-filing a review is a human
act twice over: `gh pr review` is denied to a tick, and it is a verdict on someone else's work.

Two things travel with the column. It is quoted material, not instructions: it arrives inside a model's
instructions, so the instruction says so around it and names the authorities a reviewer cannot grant.
And an empty column on a review the classifier read as non-empty is a transport failure, not an empty
review -- the tick is told to read the body itself, and told that if it is not there it stops with
the escalation label (`factory/escalation-label.sh` prints it) rather than writing the fix it guessed.

### What an escalation leaves on the artifact, and what it does not

Four classes of decision are not a unit's to make -- a third-party dependency, security or
tamper-resistance semantics, publishing, deleting data -- and a unit that meets one stops. What it
leaves behind is three things in a fixed order, and the order is the mechanism: the comment naming what
is ambiguous; the entry with `Chose: escalated` and `Awaiting:` **committed** into the decision log; and
only then the marker, applied with the name read out of `factory/common.sh` through
`factory/escalation-label.sh`, then read back from GitHub in the same breath as anyone claims it
landed.

The order is what lets the marker be wrong. It is wrong today: #311 asks whether to create the name the
instruction used to carry, or point the instruction at a label this repository actually has -- it has
eighteen, and that is not one of them -- so until a person answers, the write is refused. Measured 12 Nov
on gh 2.46.0, a probe of exactly that shape -- `gh issue edit 311 --add-label zzprobe-does-not-exist` --
printed `'zzprobe-does-not-exist' not found` and `failed to update 1 issue` for exit 1, and the labels
read back unchanged. With the record committed first, a refusal like that costs one sentence in
`NEXT:`. With the label in the middle of the step, under `set -e`, it cost the record: the comment
written, the marker absent, `Awaiting:` never committed, and an escalation that reads exactly like a unit
that quietly gave up (#311, #315).

The verb travels with the name, and #315 measured it separately. Both artifacts take `gh issue edit`,
a pull request included, because `gh pr edit` cannot put a label on anything on this account. Measured
12 Nov on gh 2.46.0, one 67-character probe name aimed at pull request #314 with its labels read before
and after: `gh pr edit 314 --add-label zzprobe-…` died on
`repository.pullRequest.projectCards` for exit 1 -- gh reads that retired field on its way in, so the
call ends **before** it resolves the label name -- while `gh issue edit 314 --add-label zzprobe-…`
reached the label step and answered `'zzprobe-…' not found`, also exit 1, and the labels read back
unchanged both times. Two identical exit codes with two different meanings, and only one of them is a
name a person can still supply; the other is a field GitHub has retired, so no answer to #311 makes
`gh pr edit` work. That is why the empty-ask branch of `factory/factory.sh` now carries the two calls a
stopped unit runs -- the write with the issue verb, the read with `gh pr view` -- instead of the word
`label` on its own, and why a leg in `factory/selftest.sh` refuses any live file pairing `gh pr edit`
with a label flag.

What a reader may conclude from the marker is that a named human owes a decision on that artifact. What
no reader may conclude from it is that the unit is parked or released. `factory/triage.sh` reads no
label at all -- measured 12 Nov, `grep -c label factory/triage.sh` answers 0 -- so the classifier's two
issue classes stay `unclaimed` and `delivered`, and the marker is a signal for a person and nothing
else. Changing that later means moving the queue's decisions into configuration no gate reads, which is
why a leg in `factory/selftest.sh` refuses the word appearing anywhere in the classifier.

## Installing it

Generate the units; do not copy them out of a README. `factory/install-timer.sh` asks the machine
where `gh` and `opencode` actually live, derives the unit's `TimeoutStartSec` from `FACTORY_BUDGET`,
and refuses to install anything that would fail quietly:

```bash
factory/install-timer.sh --install --dry-run     # prints both units and the systemctl commands
factory/install-timer.sh --install               # refuses a tree that is not the trunk
factory/install-timer.sh --install --force       # anyway, and `TIMER-FORCED` in the journal
journalctl --user -u parental-lockout-factory.service -n 50
factory/install-timer.sh --remove                # the whole thing, reversibly
```

It installs into `~/.config/systemd/user`, so nothing needs root. What it writes, and why each line
is there:

```ini
# ~/.config/systemd/user/parental-lockout-factory.service
[Unit]
Description=parental-lockout-factory delivery supervisor (one pass)
Documentation=file:/home/rodney/prj/parental-lockout/docs/factory.md

[Service]
Type=oneshot
WorkingDirectory=/home/rodney/prj/parental-lockout
ExecStart=/home/rodney/prj/parental-lockout/factory/factory.sh
# Every binary `make verify` shells out to, not just the two the installer needs. A unit reads no
# shell profile, and here `go` and `gofmt` live in ~/.local/bin -- a tick whose PATH lacks them cannot
# pass the one gate it is required to pass, and finds that out 40 minutes into a run.
Environment=PATH=/home/rodney/.local/bin:/home/rodney/.opencode/bin:/usr/bin:/usr/local/bin:/usr/bin:/bin
Environment=FACTORY_REVIEWER=hermesvicowa
Environment=FACTORY_REVIEWER_TEAM=workers
# The project the CLI works in -- agent, deny rules, skills -- comes from $PWD, not from
# WorkingDirectory. A unit exports a cwd and no PWD, so without this line every tick opens a session
# in the background server's home directory and dies on `Agent not found: "factory"`.
Environment=PWD=/home/rodney/prj/parental-lockout
# Two numbers, because they bound two different moments. They used to be the same one, and that
# coupling is what the paragraph after this block is about.
# TimeoutStartSec is here because a oneshot has NO start timeout at all: systemd.service(5) says the
# default is "disabled by default" for that Type, measured as `TimeoutStartUSec=infinity` on a manager
# whose DefaultTimeoutStartSec is 1min30s. With none, a wedged pass holds the run lock forever and
# every later activation quietly does nothing. It is also the deadline that fires while a pass is
# *running*, so it is the one that could kill a tick mid-commit.
# TimeoutStopSec bounds only the reaping, after the pass has exited: SIGTERM to the whole cgroup,
# SIGKILL at this deadline, and the unit is not `inactive` until that finishes. One minute, because the
# only thing left to wait for is a grandchild that will not die.
TimeoutStopSec=60
TimeoutStartSec=5460
Nice=10
IOSchedulingClass=best-effort
IOSchedulingPriority=7
```

**The two deadlines were one number until 8 Nov, and it took the queue offline for 1 h 07 min.**
`OnUnitInactiveSec=` is the setting that keeps passes from stacking, and it is the one that re-arms the
queue after a pass -- so "this unit becomes inactive" is the event the whole loop waits on. (The timer's
other anchor, `OnStartupSec=`, fires once per manager start, which is why it rescued nothing here: this
manager had been up since long before the pass.) A pass that evening printed `END rc=4 ticks_started=1`
and its main process exited at 22:18:51Z, but the unit never went inactive: something the tick had run
left a Wine grandchild (`C:\windows\system32\winedevice.exe`) in the unit's cgroup,
`KillMode=control-group` will not call the cgroup empty while one descendant lives, and systemd waits
out `TimeoutStopSec` -- then 5460 s -- before escalating to `SIGKILL`. For 91 minutes the unit sat at
`ActiveState=deactivating SubState=stop-sigterm MainPID=0 TasksCurrent=10`, the timer sat at
`ActiveState=active SubState=running` with an **empty** `TriggerNextElapseUSecRealtime` and a `-` in
`list-timers`' next-fire column, and the journal held nothing after 22:18. Every command that reads as
health read as healthy. `systemctl --user kill -s KILL <unit>` at 23:06:55Z re-armed the timer
immediately.

A descendant that ignores `SIGTERM` is not hypothetical and is not a Windows curiosity -- anything a
tick runs can leak one, and the harness cannot enumerate them, so the answer is a bounded stop rather
than a smarter kill. Three things travel with that:

- `TimeoutStopSec` is now its own token at 60 s and `TimeoutStartSec` keeps the budget-derived ceiling.
  Shrinking the start ceiling by accident would kill a real tick at a minute, so `factory/selftest.sh`
  renders the shipped generator and asserts the two values are *different* and that start still clears
  the budget (H-0066).
- The only line in the whole system that recorded this failure is the unit's own substate, and nobody
  reads it. Whatever watches the loop has to print it:
  `systemctl --user show parental-lockout-factory.service -p ActiveState -p SubState -p TasksCurrent`,
  alongside the timer's `TriggerNextElapseUSecRealtime`. A timer that is `active` with no elapse is the
  least suspicious thing a stalled queue can look like: H-0037 is the same shape -- 3h13m of nothing
  after a boot, with the timer reporting `active`.
- After an escalated kill the unit ends `failed` -- `Result=exit-code`, from whatever the pass returned
  -- which does not stop the timer firing, but makes `systemctl --user status` read like an outage long
  after the queue recovered. Clear it with `systemctl --user reset-failed parental-lockout-factory.service`.

What remains owed is owed by every machine that does not already have it, and it is not a tick's to do:
the reinstall that puts 60 s on a unit -- `factory/install-timer.sh --install`, which has to restart a
timer that is already running (H-0037) -- is a manager write, and a manager call is denied to an
unattended unit. This box is not one of them. Its installed unit, written 2026-11-10 at
`~/.config/systemd/user/parental-lockout-factory.service`, reads `TimeoutStopSec=60` and
`TimeoutStartSec=5460`, and the 00:56 row in the table below is that 60 s firing on the shipped unit with
the same grandchild as the outage: 1 h 07 min became 60.18 s.

The reaping itself has been measured here since 11 Nov (issue #246, H-0118), with a transient unit and
a planted `trap '' TERM` descendant, so the stop path is no longer a reading of how `SIGKILL` behaves:

| the activation | main exit | the hold | what systemd reported as `Result=` |
| --- | --- | --- | --- |
| nothing leaked | 4 | 0.3 ms | `exit-code` |
| one leaked descendant, `TimeoutStopSec=60` | 4 | **60.194 s** | `exit-code` |
| the same, `TimeoutStopSec=20`, `OnUnitInactiveSec=10s` | 4 | **20.135 s**, next pass 30.4 s after the previous one's last line | `exit-code` |
| the real unit, 11 Nov 00:56 (`winedevice.exe` again) | 0 | **60.18 s** | `timeout` |
| the real unit, 8 Nov 22:18 -- the 91-minute outage | 4 | **4084 s**, ended by a hand-escalated kill | `exit-code` |

Where the numbers on `factory.sh --status`'s `reaping:` line come from, since one of them is a
subtraction and the word beside it is not what most people reach for:

- The END is the newest terminal line in the unit's own journal -- `Failed to start …`, `Finished …`,
  `Stopped …`, `Deactivated.` -- from `journalctl --user -u parental-lockout-factory.service -t systemd
  -o short-iso-precise --utc -n 400`. The 400 is a bound on how much is read, not the measurement: a
  pass 25 minutes old has written past any line-count window, so it is the newest *terminal* line the
  window holds, not the newest line.
- The START is `Main process exited` when systemd wrote one -- and it writes one only when the main
  process exits non-zero, measured as 4 activations out of the last 15 on this unit. An exit-0 pass
  writes nothing, so the start is the pass's own last stdout line, read by a second call *without*
  `-t systemd`: that filter excludes the unit's output entirely (0 of the 325 lines in a 400-line
  window carry it), which is why one call cannot answer both halves and why a read filtered to
  systemd's identifier reports the 00:56 hold as 0 s.
- The manager's own lines are dropped by the field the author occupies -- `<comm>[<pid>]:`, third in
  the rendering -- not by matching that text anywhere in the line. The harness's own sentences quote
  journal lines, and a line the unit wrote that quotes `systemd[4485]:` is still the unit's line.
- Stamps are microseconds and the window boundary is applied by the read, not by `--until`, because
  `--until` truncates to the resolution of the stamp it is handed. This is why: the 08:44 activation's
  `State 'stop-sigterm' timed out.` opens at 08:45:08.999972, a displayed second BEFORE its terminal
  line at 08:45:09.002985. Read the naive way, that hold is 1 s; it is 60.133 s.
- The word after the number is the *cause*, and it comes from the sentence, not from `Result=`.
  `deadline` is `State 'stop-sigterm' timed out. Killing.`, `client` is `with SIGKILL on client
  request.`, and anything else held past `FACTORY_REAP_NOTICE_SECS` (5 s) is `unexplained`. `Result=`
  is not usable for this: `timeout` is what the 00:56 pass that exited cleanly reported and
  `exit-code` what the 91-minute hold reported, which is backwards on the only polarity that matters
  (H-0118).
- The live half after `live:` is `systemctl --user show <unit> -p LoadState -p ActiveState -p SubState
  -p Result -p MainPID -p TasksCurrent`, and it is the half that sees a hold in progress:
  `MainPID=0` beside `TasksCurrent=10` is the main process gone and the cgroup not empty, which is the
  8 Nov outage in two fields. The keys are printed with the values because `systemctl show` answers in
  its own order, not the order asked (`LoadState` is there because the other four cannot distinguish a
  unit the manager never loaded from a healthy one -- they answer `inactive dead success [not set]`
  for `no-such-unit.service`). It degrades to nothing when the bus is unreachable, which is exactly
  when the journal half is read anyway.
- When either end of the subtraction is missing the line says so and prints no number, in one of two
  sentences: nothing has finished inside the window, or something finished and the journal cannot say
  when its work stopped. The second one used to print `reaped in 0s`, the healthy case's own words
  (H-0124).

**The same read is in the supervisor's journal on every activation, and not only in `--status`** (H-0129).
`--status` is a question to a timer and nobody asks a timer a question -- the argument that put
`SUPERVISOR-AT` and `UNIT-AT` in the journal (#249, #260) -- and a held teardown is now true for exactly
one `TimeoutStopSec`: 60 s inside a ten-minute cadence, in a substate nobody has open. Measured on this
box the morning before the line existed: five activations held the queue for about a minute each, between
08:05 and 09:42 (`last word -> terminal stamp` 08:05:36->08:06:36, 08:21:08->08:22:09,
08:44:08->08:45:09, 09:20:42->09:21:42, 09:41:42->09:42:42, every one `winedevice.exe` ignoring
`SIGTERM`), and `grep -c REAP` over the journal those passes wrote answered `0`.

Three marks, one line, always exactly one (`<age>` is how long ago the terminal line was written):

```
REAP-CLEAN hold=0s how=clean ended=<age>s open=1
REAP-HELD hold=60s how=deadline ended=<age>s open=1 -- parental-lockout-factory.service was still being torn down after the pass said its last line, and parental-lockout-factory.timer counts from the unit going INACTIVE, so the queue could not re-arm for all of it; `factory.sh --status` names the cause (#246)
REAP-UNKNOWN rc=2 a pass finished and nothing in the journal says when its work stopped, so the hold is unread, not zero (#246)
REAP-UNKNOWN rc=1 nothing in the journal window has finished, so this pass cannot say what the last one cost the queue (#246)
```

`REAP-HELD` is the mark to grep for, and `grep REAP` is the census: the three are exhaustive, so a
stretch of journal with no `REAP-` line in it is a harness that stopped looking, not a machine that never
held. The two sentences are the same two the `--status` line uses for the same two ways the read runs out
of input, and neither prints a `hold=` field -- an unknown is not the clean state (#228). `how=` carries
the cause (`deadline`, `client`, `unexplained`, as above) and the sentence deliberately names none of it:
only `deadline` is a descendant that ignored `SIGTERM`, and a mark that asserts the wrong cause is worse
than one that asserts none. `open=1` is normal and says this pass is itself an activation.

The line is written before the `git fetch` and before triage, so the pass that never reached a queue
still says what the last one cost: `TRIAGE-FAILED` returns 3 five lines after this loop, and a queue
that cannot be read is the pass an operator goes looking for (3h13m on 9 Nov). It is a read of
`journalctl`, never a manager call -- the bus stays `--status`'s addition, where a human is the one
asking. And it is one line per pass rather than one per hold by arithmetic, not by a dedup to maintain:
the read reports the newest *completed* activation, which from inside a running pass is always the pass
before this one, so a hold is written once, by the next activation -- the first thing in existence able
to say it. `ended=` is what makes a repeat read as a repeat rather than as a fresh hold, which is #237's
rule for any sentence printed on every activation.

No `After=`/`Wants=network-online.target`. That target does not exist in a user manager --
`systemctl --user status network-online.target` answers `Unit network-online.target could not be
found.`; it belongs to the system instance, where NetworkManager-wait-online.service pulls it in -- so
the pair ordered the pass after a unit that can never load and implied a network wait the pass does not
perform. It does not need one: `gh` unreachable is a visible `TRIAGE-FAILED` with rc=3 and nothing
attempted, and the next activation is ten minutes away.

```ini
# ~/.config/systemd/user/parental-lockout-factory.timer
[Unit]
Description=parental-lockout-factory delivery supervisor, every 10m

[Timer]
# OnUnitInactiveSec, not OnBootSec/OnCalendar: the next pass starts an interval after the previous
# one *finished*, so a long pass cannot stack a second supervisor behind it.
OnUnitInactiveSec=10m
RandomizedDelaySec=2m
AccuracySec=1m

[Install]
WantedBy=timers.target
```

Then:

```bash
systemctl --user list-timers 'parental-lockout-*'
journalctl --user -u parental-lockout-factory.service -n 50
```

Ten minutes is not the review cadence -- a unit waiting on a reviewer is re-poked every
`FACTORY_REVIEW_POKES=1800` regardless of how often the timer fires. The tight interval is what
makes a *newly* green pull request merge within minutes of its last check finishing. A wait-state
pass costs no model tokens: it is `gh`, `jq` and a few ledger files.

**What makes this run while you are not looking at the machine is linger, not `Persistent=`.** That
flag used to be in the timer above, with a paragraph claiming it re-ran a pass missed while the box was
off. systemd.timer(5): `Persistent=` "only has an effect on timers configured with `OnCalendar=`", and
this unit has no `OnCalendar=` -- the flag was inert and the paragraph described nothing. What actually
decides it is whether your *user manager* is running when nobody is logged in:

```bash
loginctl show-user "$USER" --property=Linger    # `no` means the timer stops when you log out
loginctl enable-linger "$USER"                  # the one command that makes an unattended timer real
```

`install-timer.sh --install` runs that first command and prints the second when the answer is `no`,
because the reading belongs to the machine rather than to this repository -- on the box this harness was
written on it was `no` for a day and then `yes`, with no change to any file here in between.

**A tick needs this branch merged; the mechanical half does not.** `opencode run --agent factory`
resolves the agent, its `deny` rules and this skill from the project the CLI runs in -- which is the
tree `$PWD` names, not the worktree the instruction tells the tick to deliver into -- and a worktree
cut from an unmerged `main` has none of them, so a tick can only read them from the tree the harness
was started in. Get `$PWD` wrong and the session opens in whatever directory the background server
was started from, where nothing is defined at all: measured 5 Nov, 152 passes in a row died inside
two seconds on `Error: Agent not found: "factory"`, each one recorded as a bare `exit=1`, because a
systemd unit exports a working directory and no `PWD`. Review requests, merges and conflict
resolution read only the API and git, so they work from any checkout.
That is why the first install can point at the harness branch (`factory/DECISIONS.md` H-0009) and why
`install-timer.sh` refuses rather than installing a unit whose `ExecStart` path does not exist: a
timer that fires into 127 looks identical, from the outside, to a timer that is working.

## Which code the loop is running

`ExecStart=@REPO@/factory/factory.sh` is a *path*. A path says where the supervisor lives, not what
it is, and the tree under it can be any commit anybody left there. Measured 9 Nov, from the tree's own
reflog: it had sat on `efa4e29` since 6 Nov 19:27Z and been fast-forwarded at 16:39:59Z -- **three
days**, with `origin/main` 121 commits away the whole time and `ahead 0`, so a fast-forward was always
available and nobody was told to take one. What that cost is checkable rather than rhetorical: the
halt path in that tree predates #228's hardening of the dirt verdict -- `git show efa4e29:factory/common.sh`
has no `update-index --refresh` and `origin/main` has eight of them, arriving in `cf87ecc`, `08bd015`
and `15352b3`, all three inside the 121 -- so the code that decides whether to stop the whole queue was
the version that reads a stale stat cache. And the reinstall a person reaches for when a loop is not
moving is a no-op with a healthy exit: run against a clone 33 commits behind its own trunk, the shipped
`--install` exited 0, wrote both unit files, and left one journal line reading `TIMER-INSTALLED`
(#249).

So the tree's position against the trunk is part of *which code a given pass is*, and the harness says
it in two places. The one command a person types when the queue is not moving:

```
$ factory/factory.sh --status
halt:    no
code:    3962919 (detached, 3 behind origin/main)
```

and the same fact in the journal, beside the `PASS` line, on every activation -- because nobody asks a
timer a question, and `units=0` is one word short of a lie when the reader is out of date:

```
2026-11-09T17:39:29Z PASS units=0 dry=0 budget=5400s root=/tmp/opencode/demo-ff trees=/tmp/opencode/wt
2026-11-09T17:39:29Z SUPERVISOR-AT 3962919 dirty=0
2026-11-09T17:39:29Z SUPERVISOR-STALE origin/main is 3 commits ahead; the loop is not running merged code
```

Both are from a clone detached three commits behind its own trunk, with the harness where
`install-timer.sh` puts it -- the same shape `factory/selftest.sh` builds for its `behind only` legs, so
the shas in them are that clone's own commits and match no branch here. A fork, or a dirty tree, is the
same two lines with more to say: `code: 95e1816 (issue-249, 0 behind origin/main, 1 commit ahead: a
fork, which a fast-forward will not fix) + 5 uncommitted path(s)` is the line from the tree this change
was written in, printed at 17:29:33Z while the work was in progress.

`SUPERVISOR-AT` is on every pass that reaches the pass body -- the same boundary `LOG-AT` documents, and
the same exception: a pass stopped by a `factory/HALT` marker writes none of the self-report lines.
`SUPERVISOR-STALE`, `SUPERVISOR-FORKED` (the tree also holds commits
the trunk does not), `SUPERVISOR-NOBASE` (no `origin/<trunk>` ref to compare against) and
`SUPERVISOR-UNKNOWN` (git could not read the tree) appear only for the state each names, and
`factory/selftest.sh` pins all five against trees planted in each state. `dirty=` is a path count from
the same three content reads `factory_tree_dirt` uses -- never from `git status --porcelain`, for the
reason given in the section on dirty trees.

The count is every commit reachable from `origin/main` that the tree lacks, which is the reading
`triage.sh` already uses for its `branch=` column. It is therefore *not* first-parent: the tree above
is parked three merge commits back on a history of merges and says 33, because 33 merged commits are
what it is missing. `git log --oneline HEAD..origin/main` is the list it counts.

Two trees can be "the code under test" and they are different directories, which is why the branch is
never asked of a global: `$FACTORY_HARNESS` is where the running script was read from -- what `--status`
and the pass report, because it is what is actually executing -- and `$FACTORY_ROOT` is what `@REPO@`
becomes, so `--install` judges that one, because it is the tree this install would arm. They are equal
only while the harness lives in the primary checkout, which is the arrangement `H-0009` rejects. What the
loop runs *today* is neither of those globals: it is the `ExecStart=` of the unit already installed, and
since #260 that is read (`factory_installed_tree`, `factory/common.sh`) instead of inferred.

This is what a reinstall of that tree now prints, verbatim:

```
install-timer: refusing to install a tree that is not the trunk.
  /tmp/opencode/demo-ff is at 82b37c3 on main: 3 commits behind origin/main.
  The installed unit execs this checkout, so this tree IS the supervisor: every merge
  since its checkout is invisible to the loop that exists to act on them, and every
  other check in this harness reads healthy while that is true (#249).

      git -C /tmp/opencode/demo-ff fetch origin main && git -C /tmp/opencode/demo-ff merge --ff-only origin/main

  Then re-run --install. --force installs this tree anyway, and says so in the journal.
```

and the same refusal where the tree holds commits the trunk does not, because the operator's next
command is different and the message has to be the one that is true:

```
install-timer: refusing to install a tree that is not the trunk.
  /tmp/opencode/demo-fork is at 9a81476 on detached: 3 commits behind origin/main, 1 ahead of it.
  The installed unit execs this checkout, so this tree IS the supervisor: every merge
  since its checkout is invisible to the loop that exists to act on them, and every
  other check in this harness reads healthy while that is true (#249).

  This tree's branch (detached) holds 1 commit origin/main does not have, so a fast-forward is
  not available. Merge it or rebase it onto the trunk, or arm a checkout the trunk
  already contains:

      FACTORY_ROOT=<checkout> /tmp/opencode/demo-fork/factory/install-timer.sh --install

  Then re-run --install. --force installs this tree anyway, and says so in the journal.
```

Neither of those two sentences about the unit was *earned* until #260. Both read `The unit execs
<FACTORY_ROOT>/factory/factory.sh`, composed from the `ExecStart=` line this script is about to
**write** -- and a refusal is exactly the case where nothing is written, so the sentence described a
unit that did not exist yet and was false whenever the unit that did exist pointed somewhere else.
Measured 10 Nov: the installed unit execed `~/prj/wt/factory-harness` while `--install` from
`~/prj/parental-lockout` printed that it execed `~/prj/parental-lockout`; the guard checked the
position of the second tree, the operator was sent to fast-forward it, and the tree the loop actually
ran sat 38 commits behind without ever being named. So the refusal now prints what the unit says,
including where **that** tree stands:

```
install-timer: refusing to install a tree that is not the trunk.
  /tmp/opencode/demo-ff is at 82b37c3 on main: 3 commits behind origin/main.
  Two trees are in play. The loop runs today from the one the unit execs: /tmp/opencode/demo-fork
  The unit execs /tmp/opencode/demo-fork/factory/factory.sh, NOT /tmp/opencode/demo-ff/factory/factory.sh (#260).
  That tree is at 9a81476 on detached: 3 commits behind origin/main, 1 ahead of it.

  The tree above is the code the queue is running right now. This install replaces
  the unit, so after it the loop moves to:
    /tmp/opencode/demo-ff
  Fix both, or read the journal the way it will read: a healthy-looking pass, from a
  stale tree nothing here reported as stale.

      git -C /tmp/opencode/demo-ff fetch origin main && git -C /tmp/opencode/demo-ff merge --ff-only origin/main

  Then re-run --install. --force installs this tree anyway, and says so in the journal.
```

Three other arms print instead when the reading does not answer -- no unit at that path, a unit whose
`ExecStart` is not this harness's script, a file that cannot be read -- and each names the file it
looked at. The third is not a synonym of the first: "I could not read it" and "nothing is armed" are
different statements about the machine, and the second one is reassuring (#228 is the same polarity in
`triage.sh`).

A **successful** install moves the loop the same way, and that is the moment a healthy-looking journal
goes quiet, because the journal lives inside the tree (`factory/log/supervisor.log`) along with the
HALT path and the working tree a tick may dirty. On 10 Nov it was invisible: `TIMER-INSTALLED` at
13:39:05Z in one tree's log, the next `PASS` in another's, nothing in the old file to explain the gap.
So the move gets a sentence in the block a person reads, and its own journal line:

```
2026-11-10T15:58:20Z UNIT-TREE old=/tmp/opencode/demo-fork new=/tmp/opencode/demo-ff unit=demo-factory
2026-11-10T15:58:20Z TIMER-INSTALLED demo-factory interval=10m repo=/tmp/opencode/demo-ff

Installed:
  /tmp/opencode/refusals/units/systemd/user/demo-factory.service
  /tmp/opencode/refusals/units/systemd/user/demo-factory.timer

  The loop MOVES. The unit installed today execs:
    /tmp/opencode/demo-fork
  this install replaces it with one that execs:
    /tmp/opencode/demo-ff

  That changes the journal, the HALT path, and which working tree a tick may dirty. An
  operator tailing the old journal will watch a healthy queue go silent at this moment:

    /tmp/opencode/demo-fork/factory/log/supervisor.log
```

Every block on this page was printed by the shipped script, not composed by hand: clone a fixture
trunk, `git reset --hard HEAD~3` for the stale tree, plant the unit named by `--name` under a scratch
`XDG_CONFIG_HOME`, then run `install-timer.sh --install` with `PATH` holding a `systemctl` shim so the
fixture never touches the operator's manager. The lines that are *not* the script's own -- the
`systemctl` calls and the `list-timers` table -- are elided from the quotes above. Where a block prints
a path to `install-timer.sh` -- the `FACTORY_ROOT=` example, the `Remove:` line -- that is `$0`, so it
reads as whatever path the script was invoked by; the demo path is kept here for readability.

`--status` answers the same question on its `unit:` line, because that command is what someone types
when they suspect this; until #260 it described only the tree they were standing in. A unit also
records its own name (`Environment=FACTORY_SERVICE_UNIT=<name>.service`) for the same reason: `--name`
is chosen on a command line and is recorded nowhere else, so a pass reading the default name would be
describing a unit that does not exist.

What the check deliberately does **not** do:

- **No fetch inside the verdict.** It compares against `origin/<trunk>` as this checkout last learned
  it, the same reading the orphan pass already makes. A fetch is a blocking network call in front of a
  budget, and this repository already owns the hazard of a read that parks (H-0056; `H-0013` cost
  5min2s of a pass for a local read). The command the refusal prints starts with the fetch, because an
  operator with a terminal cannot park a pass.
- **No automatic move.** A timer runs a path, and editing the script a shell is reading is not a thing
  to do from inside it.

The refusal differs by the operator's next command, because the two states have different fixes and
only one of them has a fast-forward: behind with no local commits prints
`git -C <tree> fetch origin main && git -C <tree> merge --ff-only origin/main`; a fork -- commits of its
own, which is what a tree looks like after a pass was interrupted mid-work -- prints that a
fast-forward is *not* available and names the three real choices. A stale tree can always still be
*removed*: `--remove` runs before the check, because the wrong reason to be unable to stop a broken
loop is that it is broken.

### The pass says where the journal is, and writes into the one it is leaving (#260)

The installer above says the move is coming. The pass is what says it happened, and it says it in the
journal that is about to become the only live one -- then writes a copy into the file going dead, which is
the half the installer cannot do. `UNIT-TREE` is printed by whichever tree the installer was run FROM, so
on 10 Nov it landed in the tree being left and the tree being entered never mentioned it. These are the
lines a pass emits (`factory_log_move_lines`, `factory/common.sh`):

```
2026-11-12T19:14:34Z LOG-AT /tmp/opencode/demo-ff/factory/log
2026-11-12T19:14:34Z LOG-MOVED old=/tmp/opencode/demo-fork/factory/log new=/tmp/opencode/demo-ff/factory/log prev-pass=2026-11-10T10:17:19Z how=recorded tail=ok -- the journal lives inside the tree the loop runs from, so this pass writes where the pass before it did not; whichever of the two files this line is in, the OTHER one is the tail an operator may still be reading, and its last line is the last thing this loop ever said there. The journal that went quiet is /tmp/opencode/demo-fork/factory/log/supervisor.log. The tail= field is about the copy of this line written into it: ok wrote it, failed the append itself was refused, skipped nothing was opened -- and skipped carries refuse= naming the clause that refused, because five clauses answer skipped and they are not one fact: old-directory-is-a-symlink, old-directory-not-writable, old-journal-is-a-symlink and old-journal-not-a-regular-file each name something sitting at the old path this pass will not write through or into, and only no-such-directory says there is nothing there to tail. The clauses in the order they are judged: old-directory-is-a-symlink, no-such-directory, old-directory-not-writable, old-journal-is-a-symlink, old-journal-not-a-regular-file (#260)
```

and that is the whole of what the old file gained, below the line that was already there:

```
2026-11-10T10:17:21Z HALTED code=3962919 -- a HALT file is present
2026-11-12T19:14:34Z LOG-MOVED old=/tmp/opencode/demo-fork/factory/log new=/tmp/opencode/demo-ff/factory/log prev-pass=2026-11-10T10:17:19Z how=recorded copy=old-journal -- …(#260)
```

`LOG-AT` is on every pass that reaches the pass body, for the reason `SUPERVISOR-AT` and `UNIT-AT` are -- a
read nobody prints cannot be told apart from a read that is not there -- and it is also what the next pass
reads back as this pass's answer, so it is printed before the `PASS` line: a pass that halts at triage
writes no `PASS` at all, and that is exactly the pass somebody is trying to locate. **Where that boundary
is, and what sits outside it:** all four of a pass's self-report lines -- `UNIT-AT`, `LOG-AT`,
`SUPERVISOR-AT` and `PASS` -- are emitted inside `factory_body`, and the halt check logs `HALTED` and
exits before `factory_body` is called, so a pass stopped by a `factory/HALT` marker writes none of them.
The dead tail this section opens with is precisely that `HALTED` line, left by a pass that never said which
directory it was writing to. `#331` holds the three ways to close that and the lock boundary a hoist has to
answer for. The previous directory comes from the
unit's own journal lines, not from a marker file (a record that has to be maintained is the defect
`factory_journal_age` exists to avoid), not from the tree (two of the 10 Nov trees were deleted inside the
hour), and not from `$FACTORY_LOG`, which is this pass's own answer. It is read with
`journalctl --user -u <unit> -n $FACTORY_LOG_TAIL` and **no** `-t systemd`: the measurement that gives the
reaping read its filter says a 400-line window taken WITH it holds 0 of the 325 lines the unit's own
processes wrote in it, and this read wants precisely those 325. Manager lines are refused instead by
anchoring in the third field, and both marks are anchored to the start of the message -- the ticket that
filed this requirement pastes a `PASS` block into its own body, and a substring match reads a move out of
a quote.

`how=` says whether the old directory was recorded or inferred: `recorded` from a previous `LOG-AT`,
`derived` from an older `PASS … root=<tree>` plus the `/factory/log` suffix, which is the path the
generated unit implies because it sets no `FACTORY_LOG`. Recorded wins regardless of line order, and that
is not a preference: a pass writes `LOG-AT` and then its own `PASS` a few hundred milliseconds later, so a
newest-line-wins rule answers `derived` on every pass and the fact never reaches the log.

`tail=` is about the copy, the only write here outside stdout. It is refused unless the old directory is a
real directory, writable, and not a link, and the file in it is absent or a real regular file -- with each
link test as its own clause, asked before anything that follows the path. A dangling symlink is `-e` false,
and the first cut folded the file's link test behind that shape check, never reached it, and the append
created the link's target. Writing a path this harness did not create, named by a record read off a log, is
the shape #874 names; #182 is what it costs (#260, `H-0148`).

`--dry-run` spares GitHub, git and the model, and it does not spare this write. Measured at head with
`DRY_RUN=1`: the neighbour's `supervisor.log` gained the `copy=old-journal` line and the pass reported
`tail=ok`. That is intended and it is the fourth thing a dry run touches -- `log_event` writes this pass's
own journal line in a dry run too, and the banner promises only the three -- but it belongs in the page so
the next reader does not file it as a bug (#330 review, finding 2).

Those five clauses are a chain, not one `&&` group, so the refusing one can be named: `tail=skipped` is
what all five look like, and `refuse=<clause>` says which. The sentence used to assert the commonest reason
-- "found no directory there" -- whatever had refused, and four of the five print it beside a directory that
exists: a linked old directory, an unwritable one, a linked old journal, a journal that is a FIFO or a
directory. Measured with those fixtures on 12 Nov, before the field existed, all four printed that sentence
(#260, `H-0149`). The order is the finding, not a style: `-d` and `-e` both follow links, so the first cut
asked them first and a *dangling* link at the old path answered `no-such-directory` -- unreachable link
clause, and the wrong advice for the one shape a hand-moved tree actually leaves. Asking what a path *is*
before asking what is behind it is the same rule twice, once for the directory and once for the file.

The clauses, in the order they are judged: `old-directory-is-a-symlink`, `no-such-directory`,
`old-directory-not-writable`, `old-journal-is-a-symlink`, `old-journal-not-a-regular-file`. Only
`no-such-directory` says there is nothing there to tail; the other four say something is sitting at the old
path and this pass will not write through or into it. `tail=failed` is the sixth state and carries no
`refuse=`, because there the append *was* attempted and the file itself refused it.

One `refuse=` arrives with advice attached, because on a live machine it is the common case rather than an
edge. `how=derived` means no old pass ever named its directory, so the path was built from that pass's
`root=` plus `/factory/log` -- the path the generated unit implies, since it sets no `FACTORY_LOG`. Whether
that path holds a journal depends on the tree, and both answers were measured here on 12 Nov: a record whose
`root=` is the main checkout resolves to a real journal and copies into it (`tail=ok`), while a record whose
`root=` is a worktree with no `factory/log` -- a fresh one, or one of the trees deleted inside the hour on
10 Nov -- answers `tail=skipped refuse=no-such-directory`:

```
2026-11-12T20:04:11Z LOG-MOVED old=/home/rodney/prj/wt/issue-260/factory/log new=/tmp/opencode/linectl2/newlog prev-pass=2026-11-10T10:17:19Z how=derived tail=skipped refuse=no-such-directory -- the journal lives inside the tree the loop runs from, so this pass writes where the pass before it did not; whichever of the two files this line is in, the OTHER one is the tail an operator may still be reading, and its last line is the last thing this loop ever said there. The old path is derived from that pass's own root= field, because it ran before a pass could name its journal directory; where it names the tree rather than the directory, FACTORY_LOG was overridden by hand. That is also why this answer often arrives with tail=skipped refuse=no-such-directory: a path built from root= points at no journal at all unless the old pass overrode FACTORY_LOG, and it says nothing about where that pass really wrote. The journal that went quiet is /home/rodney/prj/wt/issue-260/factory/log/supervisor.log. The tail= field is about the copy of this line written into it: ok wrote it, failed the append itself was refused, skipped nothing was opened -- and skipped carries refuse= naming the clause that refused, because five clauses answer skipped and they are not one fact: old-directory-is-a-symlink, old-directory-not-writable, old-journal-is-a-symlink and old-journal-not-a-regular-file each name something sitting at the old path this pass will not write through or into, and only no-such-directory says there is nothing there to tail. The clauses in the order they are judged: old-directory-is-a-symlink, no-such-directory, old-directory-not-writable, old-journal-is-a-symlink, old-journal-not-a-regular-file (#260)
```

Neither answer says where the old pass actually wrote, and that is what the sentence on the line now says
out loud: the derived path is where the unit would have put it, not where the pass said it put it. The only
record of the latter is a `LOG-AT` written by a pass of this build or newer, which the next pass reads and
reports as `how=recorded`.

A refusal to answer is said rather than left as silence, so that the absence of `LOG-MOVED` is never
evidence nothing moved:

```
2026-11-12T19:14:34Z LOG-MOVED-UNKNOWN rc=2 the journal window holds 0 line(s) and none of them records a log directory, so this pass cannot say whether the journal moved; it is writing to /tmp/opencode/demo-ff/factory/log. The window is FACTORY_LOG_TAIL=400 lines -- raise it if a single pass outwrites it, and expect this arm on a machine whose journal holds no earlier pass of this unit (#260)
```

`rc=1` is journalctl failing, `rc=2` a window that records no directory -- normal on a unit's first pass,
and the arm to expect on a box whose journal holds no earlier pass of this unit -- and `rc=3` a newest
record naming no directory. All three still print `LOG-AT`, because the pass always knows where it is
writing even when it cannot say where the last one was.

Every block above is output from the shipped function, with the demo trees the section above uses and the
journal supplied by the same `journalctl` stub `factory/selftest.sh` plants for its four `proc_*` legs --
`the pass names its own journal directory`, `…and says so in the journal it is leaving`, `…and says
nothing when the journal stayed put`, `…and refuses in words when it cannot read` -- which run the shipped
`factory.sh` and read the journal it wrote. Seven more legs replay the arms from transcripts: recorded over
derived, the quoted `PASS`, damaged against absent, the tail knob at both sizes, a symlinked old journal, an
unwritable one, and each of the five refusal clauses named separately against a plain old journal that
refuses nothing.

Two honest qualifications about the blocks above. The `journalctl` stub is a fixture, so every line here was
produced by the shipped function but not by a real activation; and the `refuse=no-such-directory` block is
the one that keeps real paths rather than the demo vocabulary -- its `old=` is a worktree on the machine
this was written on, which has no `factory/log`, and its `new=` is a scratch directory. It is quoted as a
machine answered it because that case only exists where a real tree is missing its log directory.


## Decision ids are allocated against refs, not prose

`DECISIONS.md` and `factory/DECISIONS.md` are append-only references. A tick that numbers from the
highest heading in *its checkout* is correct about the trunk and blind to the two siblings already
pushed against it. That is what produced three `H-0061` entries and the hand renumberings recorded at
`H-0065` and `H-0068`.

```bash
factory/next-decision-id.sh DECISIONS.md
factory/next-decision-id.sh factory/DECISIONS.md
```

The command reads the working log, `HEAD`, every `refs/heads` ref and every `refs/remotes` ref. Its
answer is one above the highest id of the selected prefix anywhere in that union. Gaps below the
ceiling are closed forever: a retired or renumbered id remains the thing a historical citation means,
so "first free" is not "smallest missing number".

It answers only for the decision log itself. The named path has to be named `DECISIONS.md` and be a
regular file in the working tree or a blob in `HEAD`, because a union with no headings in it has
exactly one answer available -- `<prefix>-0001`, the first heading both logs have carried since entry
one -- so an empty union is a mistyped path, and answering it is the collision this command exists to
refuse. Measured on the version #255 shipped, on three calls a session really makes:

| The call | It printed | It prints now |
|---|---|---|
| `--prefix H factory/DECISION.md` | `H-0001`, exit 0 | a refusal naming the path |
| `factory/DECISION.md`, no `--prefix` | nothing, exit 2, stdout and stderr both empty | a refusal naming the path |
| `--repo <a checkout with no log> DECISIONS.md` | `D-0052` — *this* repository's ceiling | a refusal naming the path |

The third row is the one that mattered: a relative log path fell back to the caller's tree whenever the
named one was missing, so `--repo` read whatever log the directory it was invoked from had, and
answered a question about one repository with a number from another. It now resolves against the
repository named, once, with no second reading. The second row was a `sed` failing under
`set -euo pipefail` inside a command-substitution assignment: the script ended with the substitution's
status and said nothing on either stream, and a session holding an allocator that says nothing reads
it as a broken tool and goes back to numbering by eye -- which is the defect this whole section is
about. A fresh log is one `touch` and a title line away, and its first entry still gets an honest
`D-0001` or `H-0001`, inferred from the path rather than from a heading that does not exist yet. Six
legs and two controls in `factory/selftest.sh` plant each row; the controls are what prove the
refusals are the guard and not a fixture that agrees by accident.

Existence turned out to be the wrong question, and asking it is how a wrong path gets the same answer
as a missing one. The version reviewed at `c5aa1eb` asked whether the named path was *there*, and every
wrong path in a checkout is there: `--prefix H docs/factory.md` printed `H-0001` and exited 0, and so
did `README.md`, `AGENTS.md`, `factory` and `factory/` -- the last a directory in the tree and a tree in
`HEAD`, which `git cat-file -e` answers true for. `docs/factory.md` is this guide, the file carrying the
paragraph you are reading, so it is what a session has open at the moment it reaches for the allocator.
That hole is worse than the typo above, not merely different: the refusal `c5aa1eb` ships tells the
caller *your path is wrong*, so a session handed `H-0001` for `docs/factory.md` reads it as "my log was
read" and writes a duplicate heading -- the silent collision, where an error message would have been
visible. The guard now asks whether the path *is* the log: the basename must be `DECISIONS.md`, and
what is behind it must be a regular file or a blob in `HEAD`. A guide, a README, a directory, and a
directory named `DECISIONS.md` are each refused and each is told why (`H-0076`); three more legs and two
more controls join the six and two above, one plant per arm of the predicate, so neither test can be
retired without a leg going red.

It reads the refs this machine already has and does not fetch, for the same reason #249's staleness
verdict does not: a blocking network read ahead of a pass budget can park the loop (H-0013, H-0056).
That makes the answer a collision-free *read*, not a lock, and the read has a boundary. It is stated as
a measurement here because the skill's second read is only worth what it can see (H-0071), and the two
shapes a sibling arrives in do not behave the same way:

| A sibling that… | Unfetched | After `git fetch` | Why |
|---|---|---|---|
| committed in another worktree of **this clone** | seen: `D-0002` → `D-0010` | seen | every unit tree is a worktree, and worktrees share `refs/heads` |
| pushed from **another clone** | missed: `D-0002`, while origin holds `D-0007` | seen: `D-0008` | a push moves `refs/heads` there; `refs/remotes` here moves on a fetch only |

So the skill's re-read immediately before the entry commit closes the same-machine window and nothing
else. The second row is closed by the activation itself (H-0075): `factory.sh` fetches origin -- every
branch, once, bounded by `FACTORY_FETCH_TIMEOUT`, before it reads the queue -- so `refs/remotes` is
current by the time the allocator runs in a tick's shell. The command an operator types is still
`git fetch --prune origin`, and the shape matters more than it looks: measured in a fixture,
`git fetch --prune origin main` updates the trunk only, leaves the allocator on its stale answer, and
still prints a line with `->` in it. A sibling's ids are on the branch it pushed.

The union has two halves and they mean opposite things, so the read separates them and says which one
raised the ceiling. A ceiling some ref carries is a sibling that took the number. A ceiling only the
working file carries is the entry you are writing, and the id printed above it is what a *second* entry
in the same tick would take. That distinction is not academic, because the re-read is scheduled for the
one moment the second case is certain: immediately before the entry commit, the heading is already in
the file. Measured in `wt/issue-320` on 12 Nov against `factory/DECISIONS.md` (git 2.53.0), the
single-maximum read answered `H-0145` with the log as committed and `H-0146` after an uncommitted
`## H-0145` was appended -- and no ref anywhere held that heading: `git grep -c` over `refs/heads` and
`refs/remotes` found the string in one sibling's prose as a citation and in no heading. A session handed
the bare `H-0146` has two reactions and both cost the log: yield to the phantom and leave a gap at 145
that no entry explains, or stop trusting the allocator at the last guard before a duplicate id. So the
ceiling is attributed on stderr, as `PENDING`, naming the heading it is standing on and the ceiling the
refs really hold. stdout is unchanged and the status stays 0, for the reason the script's own header
gives -- a refusal in front of a session sends it back to numbering by eye -- and the note goes quiet on
its own once the entry is committed, because `HEAD` joins the union and the two maxima agree. What
discriminates is therefore the **absence of the note**, never the emptiness of stderr: since PR #307
every answer carries a `REFRESH` or `REFRESH-BLIND` line beside the id and no flag silences it, so there
is no answer that leaves stderr empty. Measured at `487d572` on one fixture (git 2.53.0), the shipped
allocator writes 460 bytes with the tree clean, 855 with the caller's own heading uncommitted, and 460
one commit later -- never zero, so emptiness is not a reading anything can take. (A byte count carries
the fixture's path in the refresh line, so it moves a byte or two between fixtures; the claim is that
none of the three is zero.) A moved ceiling with
**no PENDING line** is therefore the collision, and the four steps below are what to do with it.

What the note's absence does not say is *whose* ref the ceiling sits on, and the two answers differ in
what they owe you. A ceiling a sibling left on `refs/heads` is the collision: you yield. A ceiling your
own committed entry put on `HEAD` is durable, and durable includes yours -- the next id is there to be
taken, and the entry already written is not the one to move. Measured on one fixture the two are
indistinguishable on the wire: `H-0042`, the note absent, byte-identical stderr, whether a sibling's
branch or your own commit carries `H-0041`. So the third case is told from the collision by what you did
one commit ago and not by the allocator's output, which is why it is written down rather than left to be
inferred. `factory/selftest.sh` pins both directions and plants one per direction -- the note deleted,
which puts the phantom back, and the note printed whatever the ceiling, because a marker on every answer
is a marker that gets read once and then ignored -- and one further leg pins this discriminator's
wording across the skill, this guide and the script, because for one revision the two rules for this one
signal sat fourteen lines apart in the same file and no leg could see the drift.

Two things the fetch cannot do. It does not make the id *reserved*: two ticks that allocate in the
same second still agree, and only the re-read immediately before the entry commit narrows that. And it
is bounded, so on an origin that accepts the connection and never answers the activation proceeds on
stale refs and says so -- `REFRESH-TIMEOUT`, with how old `origin/<trunk>` is -- rather than holding
`factory/run/lock` while the queue freezes behind a read. A clone whose refspec names one branch
cannot learn a sibling's branch at all, and the activation says that too, as `REFRESH-BLIND`.

That activation is one fetch at one moment, and the unit's own re-read can be an hour later. `D-0058`
was written twice on 11 Nov for exactly that reason, on three timestamps worth having straight:
`06b7c29` (issue-296) committed its entry at 11:32:24-0500; the second unit's own `pre-D-0058` checkpoint
`4537b9d` (issue-297, PR #300) is dated 12:10:04-0500 -- 2260s later, and that is the moment its
allocator ran, because the skill takes the id immediately before the checkpoint; `1d3470b` carried that
unit's entry in at 12:14:46-0500, 2542s after the first entry. #303 pairs 12:10:04 with `1d3470b`, so the
"38 minutes" the incident is cited by and the 42min22s a re-measure of the two entries gives are both
true, of different pairs. The tree it happened in was one where `git show-ref | grep 297` answers
nothing, and the allocator was right about every ref it could see and silent about the fact that that
was all it could see (#303). So every call now prints one line on stderr beside the id: `REFRESH` with
the age of the union behind the answer, or `REFRESH-BLIND` with the reason a sibling is invisible from
here and the exact refresh that would close it. stdout stays the id alone at exit 0 either way, because
a refusal in front of a session is the failure two sections above already records -- an allocator that
will not answer sends a session back to numbering by eye. The clock is the newest of three sources,
because none of the three is always there: measured 11 Nov in the tree this was written in, this
worktree's `FETCH_HEAD` was 145s old, the common dir's 202s, and the newest `refs/remotes` reflog 1724s.
A `git clone` writes no `FETCH_HEAD` at all and `git worktree add` writes none in the new tree, so a
clock built on `FETCH_HEAD` alone calls every fresh unit tree blind -- and a verdict that always fires
is read as noise, which is also why the ticket's own suggested test (no
`refs/remotes/origin/<the branch you are on>`) was measured and not used: dispatch pushes no unit
branch, so it is true of every allocation this harness has ever made.

What the marker buys is a procedure, and `factory/selftest.sh` pins the procedure rather than the
promise. One sentence, in the skill and here: `REFRESH-BLIND` means fetch and re-read before the entry is
committed -- run the `git fetch --prune origin` the line names, then write the entry with the number that
comes back. Five counted legs and four controls cover it: the two arms -- a refspec that names one
branch, and refs older than the bound -- each on a fixture that trips that arm alone and each with a
control deleting just that arm from the arm list; a clean read pinned to stay quiet, with the control
that makes the verdict unconditional so the same tree is called blind too; the bound moved instead of
the tree; and a checkout with no origin at all. The limit is pinned too, on #303's own shape: a clone
seconds old whose sibling pushed a second ago reads *clean* and answers `D-0002` while `origin` already
holds `D-0007`. No offline read can see that push, so `FACTORY_ID_MAX_AGE` (300s) is how wide a window
the command refuses to be silent about -- a risk window chosen from the time a sibling needs to write an
entry and push it, not a reservation -- and the merge stays the arbiter below.

`scripts/check-decisions.py` remains the last-resort gate for the consequences: it refuses a repeated
id in one log and one id claimed by both logs, but it is deliberately offline and cannot reserve an
id in advance. That is a rule, and `factory/selftest.sh` refuses the day it changes: the gate may ask
git about objects -- a `Checkpoint:` that resolves to nothing is a claim about `git revert`, so it has
to be checked -- and never about refs. A verdict that reads `refs/remotes` is a different verdict in CI
than at the merge, depending on what each tree last fetched, and a gate that disagrees with itself reads
as flaky and gets re-run until it agrees.

### When two branches take the same number, the side that has not merged yields

Two reads can still agree: measured, two calls in the same instant against one fixture both answered
`H-0010`. What that window needs -- two sessions allocating before either commits -- is not available on
this machine by mechanism rather than by habit. `factory/factory.sh` takes `exec 9>"$FACTORY_RUN/lock"`
and `flock -n 9` before it reads anything, so a second activation prints `another supervisor holds …, so
nothing was done here.` and exits 7; and it starts each tick in the foreground (`timeout … opencode
run`, no `&`) from one loop, so two sessions are never allocating at each other here. The window that
stays open is the one no local read closes: a second clone that has not fetched, or a person running a
unit by hand in another checkout while the timer runs one here (issue #256).

**The side that has not merged yields.** Its entry is the only one that can still move. `main`'s ids sit
in merged history, the only merge this repository offers is a merge commit whose ancestors are
permanent, and so moving an id that is already on `main` means rewriting published history -- a
force-push, denied to every agent here by `.opencode/opencode.jsonc`, in a repository where the thing
it would destroy is permanent rather than retried. The gate is the arbiter and it already says enough:
it names the id, both rows, and what a `revert to before H-NNNN` would now point at. Resolving it is
four steps, in this order:

1. merge `main` into the unit branch. The conflict is *in the log*, and the resolution keeps both
   entries -- one of them changes number, neither changes meaning;
2. re-run `factory/next-decision-id.sh` **on the merged tree**, so the ceiling it reads includes the
   ids that arrived with `main`;
3. renumber the heading and every citation of it in this branch's own files, then run
   `python3 scripts/check-decisions.py` over both logs before the resolution is committed;
4. record what moved, inside the entry, the way `H-0051` and `H-0052` do: the number it gave up, the
   branch that kept it, and that the two decisions share nothing but the digit. A gap in a log is
   citable when an entry says why it is empty, and inexplicable when it does not.

### Why there is no reservation

The alternative to a rule at the merge is a reservation before the write -- a fourth ref each session
claims, or an id handed out by the tracker. Neither is built. What decides it is the shape of the window
that is left, not the cost of a script:

**A reservation ref**, claimed with a compare-and-set `git update-ref`, says nothing to the other clone
unless it is pushed -- and a pushed reservation is a state-changing write in front of the tick budget,
where `H-0013` priced a blocking *read* at 5min2s of a pass and `H-0056` refused one inside a verdict
outright. Worse, a claim that outlives the session that made it is a lease: it needs an owner and an
expiry, or a killed tick reserves a number forever and nothing here reaps it.

**The supervisor allocating the id at dispatch** and naming it in the brief moves the race earlier
without closing it. The supervisor reads the same union this allocator reads and inherits the same
staleness, so a session whose re-read finds the handed-out number taken has to move anyway. It also
costs a write to the tracker per unit, on a loop that already explains why it does not put network reads
ahead of a budget.

**Accepting it, with the merge as the arbiter, is what is bought.** The arbiter already resolves every
content conflict before anything is merged; its verdict is the four lines that name the id and both
entries; and the yield rule above is what turns the resolution into a procedure instead of a judgement
call made by whoever merges second.

What acceptance makes cheap is the mitigation #255 named and declined: `factory/factory.sh` fetching the
trunk once per activation, before it dispatches, bounded by a `timeout` so a hung fetch cannot park a
pass. That is the supervisor's ref policy rather than the allocator's, so it is issue #256 with the
`H-0056` counter-argument written out on it -- not a paragraph here that only whoever reads this
section will ever find.

## Configuration

Everything is an environment override with a default, in `factory/common.sh` -- except
`FACTORY_ID_MAX_AGE`, which defaults inside `factory/next-decision-id.sh` so that the allocator keeps
answering in a shell that has never sourced the harness.

| variable | default | what it decides |
|---|---|---|
| `FACTORY_REPO` | from `origin` | `owner/name`; set it when the remote is a fork you do not want to file against |
| `FACTORY_REVIEWER_TEAM` | `workers` | the team a review is requested from, as a **slug**; a team notifies every member, so this is the form that ends a wait |
| `FACTORY_REVIEWER` | `hermesvicowa` | the individual a review is requested from -- used only when the team is set empty (`FACTORY_REVIEWER_TEAM=`), which is how a repository with no org team overrides this one |
| `FACTORY_TREES` | `../wt` | where per-unit worktrees live; deliberately not under `/tmp` |
| `FACTORY_AT_RISK` | `../at-risk` | where a stopped tick's unpushed commits are mirrored; beside `FACTORY_TREES`, never inside it |
| `FACTORY_TICK_TIMEOUT` | `2700` | one session's ceiling, seconds |
| `FACTORY_BUDGET` | `5400` | one pass's ceiling, so activations cannot overlap |
| `FACTORY_FETCH_TIMEOUT` | `120` | the ceiling on the one `git fetch` an activation does (H-0075); hit it and the pass dispatches on the refs it has, and says so |
| `FACTORY_ID_MAX_AGE` | `300` | how old the refs behind a `next-decision-id.sh` answer may be before the read is called `REFRESH-BLIND` (#303). The age of a read, not a reservation: see the allocator section above |
| `FACTORY_MAX_TICKS` | `3` | sessions one pass may start |
| `FACTORY_REVIEW_POKES` | `1800` | the review-poke floor, seconds |
| `FACTORY_ESCALATE_AFTER` | `86400` | how long in a non-progress state before one comment |
| `FACTORY_REAP_NOTICE_SECS` | `5` | the teardown that counts as held: past this, `REAP-CLEAN` becomes `REAP-HELD how=unexplained` unless systemd named a cause |
| `FACTORY_REAP_TAIL` | `400` | how many of the unit's journal lines the reaping read is allowed to look at -- a bound on the read, never the measurement |
| `FACTORY_LOG_TAIL` | `400` | how many of the unit's journal lines the read of the journal's own location looks back for the last line that named a directory (#260); a line-count window, so a pass that outwrites it is told `LOG-MOVED-UNKNOWN rc=2`, never "it has not moved" |
| `SHELLCHECK` etc. | — | see `make check-shell` |

## Stopping it

```bash
printf 'why, in one line\n' > factory/HALT      # stop
rm factory/HALT                                  # resume
```

The file is checked between units *and* by the agent at every step boundary. Between units is not
enough on its own: a tick can hold a worktree for 45 minutes, and inside that window the only thing
that can hear you is the agent. The supervisor also writes this file itself when it finds a dirty
worktree it does not own -- a person is working there, or a dead tick left a half-merged index, and
neither is a machine's to clean up. To stop it hard: `systemctl --user stop plock-factory.timer`.

Nothing halted and the queue still is not moving: read the `code:` line of
`factory/factory.sh --status` first. A supervisor 121 commits behind the trunk prints the same
`halt: no` as one with nothing to do, and the journal's `SUPERVISOR-STALE` line is the only place that
says otherwise (#249, and the section above).

## What "dirty" means, because the test decides when the queue stops

Both halting call sites ask `factory_tree_dirt` (`factory/common.sh`), and it asks git for the
**content** of the tree, four ways: staged against `HEAD`, worktree against the index, unmerged index
entries, and untracked files that are not ignored. It does not ask `git status`, which is the command
whose answer is derived from the stat cache; and it checks the **exit status of every read**, because
"git could not tell" is not evidence of safety. Any failure returns UNKNOWN and UNKNOWN halts.

The asymmetry is why it is written that way. A false halt costs one pass, retried on the next tick; a
false clean lets a model write into somebody's half-finished work, which is not recoverable. Measured
on git 2.53.0, this is what the `status`-based test did with a tree whose repository could not be
opened -- an empty `.git`, or a `GIT_DIR`/`GIT_WORK_TREE` pair left exported by whatever ran before:
`git status --porcelain` exits 128 printing nothing, the suppression in `$(... 2>/dev/null)` makes
that indistinguishable from "nothing to report", and the pass proceeds in a tree git would not
describe. A third shape, a `core.excludesFile` pointing at a directory, is the same story only on some
filesystems, and the qualifier is narrower than "some filesystems": git `fstat`s the exclude file and
reads `st_size == 0` as an empty list, and a directory's `st_size` is the size of its own entry data,
so it depends on what the directory holds -- measured at `8015951`, btrfs 0 empty, 2 with a
one-character entry, 12 with a six-character one; tmpfs 40 empty and 60 holding either. So that typo
refuses the moment the mistyped directory contains anything, on btrfs included, and the swallow needs
an *empty* directory on a filesystem that gives empty directories no size -- which is what the
fixture's directory is. There the patterns are dropped silently, the operator's own ignored files come
back as work, and the halt stops the queue over work that is not work.
`factory/selftest.sh` measures which of the two it is running on and asserts only that one, with
a note naming the filesystem when the refusal is unreachable. The filesystem that decides it is the one
under `$TMPDIR`, where the fixture is built, and not the one the repository sits on: measured at
`f60c367` on one machine with only `TMPDIR` moved, the same commit reads 191 legs with `$TMPDIR` on
tmpfs and 190 with it on btrfs, one leg of the exclude pair appearing and disappearing with the arm.
The tally line therefore prints its own fixture root, filesystem and git version, so a leg count quoted
anywhere carries the world it was counted in (H-0104). A stat-only difference, the report #228
came in with, was tried in six variants and does *not* reproduce on this git, which refreshes the
index in memory before reporting; the verdict no longer depends on that question either way.

The refresh is the only read here that writes, so it is worth saying plainly what it can and cannot do.
It rewrites `.git/index`'s stat entries and nothing else: measured on a stat-only difference the index
changed and the file's bytes did not, and measured on an unresolved merge it wrote nothing and settled
nothing -- three index stages before and after, `MERGE_HEAD` still present, its own report the word
`needs merge` at exit 1. It is not a destructive read and it cannot tidy a conflict away. What remains
unsafe is that it writes at all, into a `.git` that may belong to a person working in the same tree.
A held `.git/index.lock` is the one shape of "busy" that shows in an exit status: the refresh exits 128
naming the lock file while the other four reads and `git status` all answer 0, and UNKNOWN is the right
answer to that, because the writes that follow the verdict are real ones (`merge --no-edit`, and
`merge --abort` when it fails).

A second shape of busy shows in no status at all, and that was #232. With a directory inside the tree at
mode 000, `git ls-files --others --exclude-standard` prints `warning: could not open directory 'sub/':
Permission denied` and **exits 0**; `git diff --quiet` prints `sub/nested.txt: Permission denied` and
exits 0 on a tree whose only modification is that unreadable file. Every read answers at or below its
success status and the verdict says clean -- over an untracked file nobody enumerated in the first case,
and over an *edit* git asserted unchanged in the second, because a file it cannot open is a file it
cannot diff. So each read is now asked a sixth question, beside the five statuses: did it finish? A read
that returned its success status and still had something to say did not. What git printed on stderr is
therefore evidence as well as diagnosis, and the rule is deliberately about the fact that it spoke rather
than the words -- matching git's phrasing is how a check goes quietly blind the day upstream rephrases a
warning. Three dispositions, and the order matters:

- a refusal (`rc` above the success list) voids the verdict: UNKNOWN, as before;
- an incomplete read that found nothing, where the verdict holds nothing else, is UNKNOWN: its silence
  was never an answer, and this is the case that used to say clean;
- an incomplete read that found nothing, arriving where another arm already named the work, appends
  `its … were only partly read` to that finding instead of replacing it. Measured: a staged change
  beside an unreadable directory answers `staged-but-uncommitted: tracked.txt; its tracked files were
  only partly read (git diff exited 0; git said: sub/nested.txt: Permission denied); …` and stops the
  pass as dirt. The alternative tells somebody with an edit in front of them to go make the tree
  readable, and it also *loses the edit* from the sentence.

What that costs, and it changes halt semantics rather than a verdict: any git that prints something
harmless on one of these five reads halts the pass, and the shape that already exists is H-0045's
exclude-file warning -- `core.excludesFile` in a symlink loop answers `rc=0` with `warning: unable to
access '…'` on two of the reads. So the leg that plants it answers UNKNOWN naming the path git could not
read, where it used to answer DIRTY naming the file the operator told git to ignore and ordering them to
commit or discard it. Both halt the pass; the sentence and the reason changed. Genuinely new is the halt
on a tree that is clean apart from the unreadable config: it used to proceed, and it now stops because
git did not apply the operator's own ignore patterns, so git's clean is not the operator's clean -- the
same asymmetry #228 was filed on, applied to a case it did not have. Sixteen real trees on this box --
this repository and its fifteen links -- were read the four non-writing ways before that paragraph was
written (64 reads) and every one of them was silent; the refresh was deliberately not probed there,
because it rewrites `.git/index` and those trees may belong to a person working in them. So nothing that
exists today halts on existing chatter; what halts is whatever a future git starts saying. And because the capture is evidence now, a run that cannot make its
scratch file cannot answer: it refuses, naming `TMPDIR`, where it used to send stderr to `/dev/null` and
carry on -- #232 invisible on exactly the boxes whose `/tmp` is broken.

The five refusal checks are redundant on purpose -- any one of them refusing is enough to halt -- but
redundancy is a claim about the code, not a property you can see from reading it, so each arm is
measured: an arm is hollowed, one at a time, and the probe has to go red. Two ways of deleting an arm
give two different answers and neither is a substitute for the other. Replacing its `return 2` with `:`
means the arm sees the refusal and keeps going, so the sentence goes out with a finding appended and the
verdict ends CLEAN; replacing its condition with an always-false test means the arm never sees the
refusal and reads it as dirt, so it still halts and only a leg that asserts the refusal's own words can
tell. Both shapes now bite all five arms, because four of them had no plant that isolated them: every
state on disk that stops one of those four stops another as well, which is what an unreadable tree
actually does. The isolation is therefore built -- a `git` first on PATH that answers exactly one
invocation and passes every other argument list through untouched, one leg per arm, behind a control
whose match names nothing git ever runs. Since #232 that shim has two modes, and the second one is what
pins the completeness branch: refuse at 128, or return the arm's own success status with a line on
stderr. Five rows refuse, five rows warn, and the needle in each is the exit code git answered at --
`git diff exited 0` is the completeness branch and `git diff exited 128` is the refusal, so a leg cannot
stay green while one degrades into the other. The refresh's row warns at exit 1, because that read's
normal report is 1 and its `needs update` text is on stdout; a leg about it would be vacuous on a
success status of 0. That pins the arm's wiring; it makes no claim that git refuses
that way on its own, which is what the planted legs are for. What the rows also show is the weaker shape
at its worst: on the untracked arm the naive test says CLEAN, because a read that dies with 128 and no
output looks exactly like a tree with nothing in it.
The unreadable-file leg had to be tightened for any of this to mean anything. It used to assert only
that something stopped, and `expect … HALT` accepts any answer but clean, so the verdict could degrade
from `its tracked files could not be read (git diff exited 128)` to `modified: keepme.txt (git said: …
Permission denied)` with the leg green -- both halt, but the second tells the operator to commit or
discard a file git could not open. It asserts UNKNOWN and the refusal's words now, behind a runtime
check that this git refuses on the plant at all, and prints SKIP with the git version where it does not.

What the completeness branch is made of, measured as of H-0050 -- twelve mutations of
`factory/common.sh`, one at a time, against the 120-leg suite (119 on btrfs; the one-leg difference is
the exclude-directory plant, which refuses on tmpfs where its `st_size` is nonzero and swallows on btrfs
where an empty directory has none):

| the mutation | legs red |
|---|---|
| one arm's completeness branch deleted, per arm | refresh 2, staged 3, tracked 5, both `ls-files` arms 6 |
| the helper always incomplete, whatever it answered | 89 |
| the helper never incomplete, whatever it printed | 6 |
| `[ -z "$raw$finding" ]` removed from both `ls-files` arms | 12 |
| `[ -z "$finding" ]` removed from the tracked arm | 4 |
| the three `only partly read` append branches deleted | 4 |
| the refresh's success list `0 1` narrowed to `0` | 2 |
| the tracked arm's success list widened to `0 1` (dirt becomes unreadable) | 6 |
| the `/dev/null` fallback restored for a failed `mktemp` | 2 |

Two of those rows earned their legs from a mutant rather than from argument. Deleting the tracked arm's
guard left all 120 green: the only tree in the suite where the tracked read is incomplete *and* an
earlier arm has already found work is the staged-and-unreadable one, so that plant was written after the
mutant, and it is the row that goes red now. The `mktemp` leg was written when the capture became
evidence: restoring the old `/dev/null` fallback is two red legs, not a silent loosening.

Three of the new legs carry `strong`, which is the probe's one assertion about the *weaker* shape: it
refuses a leg whose dirt `git status --porcelain` can see, on the ground that the leg no longer pins the
arm it was written for. So those rows assert the weaker test is blind here, not merely that this verdict
is right. The marker cuts both ways -- setting it on the visible-work leg, where the weaker shape does see
the dirt beside an unreadable directory, is two red legs -- and that leg deliberately prints `naive:DIRTY`
unpinned, because the pass halts under either verdict and the leg is about which sentence the operator
gets.

`factory/selftest.sh` is the probe. It builds a throwaway repository plus a linked worktree of it --
the second is the shape #228 was measured on -- plants one state per leg, and prints the weaker test's
verdict next to its own on every line, so a leg that has stopped discriminating is visible rather than
merely green. It is a step in `make verify`, and it refuses when that step is deleted.

The halt is one function too, and that is the whole of why it is testable: `factory_tree_halt`
(`factory/common.sh`) asks the verdict, and either returns 0 or writes `factory/HALT` and returns the
code for the path that called it -- `4` for a tick, `2` for the conflict pass. Both call sites are
`factory_tree_halt … || return $?` and nothing else. They used to each carry their own copy of the
predicate, and nothing executed either copy: flipping one to halt on a clean tree, or deleting it, left
every leg green. So the probe drives the guard over the same planted states and asserts the four things
a halt has to get right -- that it halts, the exit code it leaves, that `factory/HALT` exists with the
resume step and the action matching the finding ("commit it or discard it" is not advice anyone can take
in a tree git cannot read), and one journal line per path carrying that path's own verb. A call site
that drops the `|| return $?` is caught by the step as well, because the alternative is a pass that
asks the question and throws the answer away.

## When a tick stops clean and delivers nothing

`exit=0` from `opencode run` is a verdict on whether the session closed its turn. It is not a verdict on
whether the turn said anything, and it is not a verdict on whether anything was pushed. Measured over the
202 tick logs the harness host keeps in `factory/log` (as at 9 Nov, 00:1Z): six ticks closed with 0 and
delivered nothing — two of them the pair #247 was written about, on 8 Nov, and four more before them —
and the journal said one seven-word sentence about all six: `TICK-UNDELIVERED … exit=0 with no pull
request on <branch>`. That was all the supervisor could see. Two shapes account for four of the six.

- **A tool call nothing dispatched.** The model asks for a tool by a name the CLI cannot match, the CLI
  prints the call as text, the text becomes the assistant's reply, the turn closes, the run exits 0.
  `20261108T233014Z.unclaimed:246.log` is 710 lines and ends, at line 710, with `<function=executae>`;
  `20261108T175614Z.unclaimed:239.log` is 20 lines and ends inside its very first call,
  `<function=tools.opencode>`. The name is printed and never judged, because a log records what the model
  typed and not what the CLI would have dispatched: `edit_file` appears 247 times in this corpus and every
  one of them is in the single log whose issue text quotes it back. Reading the logs cannot tell you
  whether a name is a tool today, and a list that decides would go stale the day the tool set moves.
- **An assistant turn that emitted nothing.** `20261108T215923Z.unclaimed:241.log` is 6411 lines and its
  last line is the turn opener `> factory · default-model-fast`. That tree was clean and held four
  finished commits on `issue-241`: work, on one directory on one machine, reachable from no queue that
  reads pull requests.

So `factory_stop_reason` (`factory/common.sh`) reads the shape of the tick's last emission and
`TICK-UNDELIVERED` now says which of the two it was: `exit=0 refused as completion` when the run ended on
undispatched call markup or on an empty turn, and the old plain verdict with the last line quoted when
nothing of the kind is there. It reads the text after the **last turn opener**, and that scoping is load
bearing: the tick handed #247 itself carries nine `<function=` hits, because the issue quotes
`<function=executae>` at it, so a whole-file grep reports a misspelled tool call for a tick that
misspelled nothing. `20261108T060114Z.address_review:231.log` is the other half of the polarity -- it
ends on `<function=<function_43 edit>` and delivered anyway, having pushed three lines earlier -- so the
reason is printed by the arm that has already established there is no pull request, and is never a
substitute for that read. A name is scrubbed of control bytes and cut at 60, because it is the model's
own bytes going onto a journal line.

**A reason does not recover a commit, so both stopping arms also mirror them.** `exit=0` with no pull
request, and a non-zero exit including the `FACTORY_TICK_TIMEOUT` ceiling, each call
`factory_preserve_unpushed`: `git bundle create $FACTORY_AT_RISK/branch-<unit>-<tip>.bundle
origin/main..<branch>`, then `PRESERVED <path> (<n> commit(s) on <branch> at <tip>)` with the restore
command on the same line, then 0 mirrored / 1 nothing was owed / 2 refused -- three codes because
"nothing ahead" and "it is saved" are different sentences. The write is read back with `git bundle
verify`; a file that does not verify is `PRESERVE-UNVERIFIED` and never `PRESERVED`, because a claim
about a write nobody re-read is this repository's most repeated failure. The mirror's own failure never
changes the tick's verdict: the unit goes back to the queue either way.

Recovery, from the line the journal prints:

```
git clone <remote-url> r.git && git -C r.git fetch <path> refs/heads/<branch>
```

The prerequisite form is what keeps the mirror cheap *and* what fixes that sentence: 28,601 bytes for
`issue-241`'s four commits and 18,551 for `issue-235`'s six, where full history is 17,647,835 bytes and
re-copies the 10 MB `plock` blob on every stop. It is not self-contained -- `git clone <bundle>` on a
machine holding nothing answers `error: Repository lacks these prerequisite commits` -- which is why the
shipped sentence names the clone first rather than an in-place `git bundle unbundle`.

**Neither change re-runs the unit, and it would not have been right to.** The queue already re-offers a
unit on a later pass, and that is what rescued #239: two undelivered stops at 17:56 and 18:12 and a 2700 s
ceiling at 21:47, then pull request #245 delivered it. A retry inside the same pass would have spent a
second model run on a typo in a tool name and on a clean tree holding four finished commits, neither of
which was shown to be transient, and it would have had to decide whether the first attempt left
half-applied work -- the question `TICK-HALTED` exists to stop a tick from answering. What a retry buys
is throughput, and the timer is already the thing that provides it.

## Exit codes, because a timer has to tell these apart

`0` the pass is over · `3` the queue could not be read, so nothing was attempted · `4` a tick failed
· `7` another supervisor holds the lock · `8` halted.

`3` and `7` are the two that matter to monitoring: `3` means the token or the network is gone, and
`7` means two timers are pointing at the same repository.

## What it is not allowed to do

`factory/factory.sh` merges only what the API *still* says is approved, mergeable, clean and green,
at the moment of the merge. The agent denies itself `gh pr merge`, `gh pr review`, every force push,
every recursive delete, pushing `main`, `git tag` with an argument, `gh release create`/`delete`/`edit`,
`gh api`, `go get`, `systemctl`, `git worktree remove`/`move`, and the `question` tool. Those denies are in
`.opencode/opencode.jsonc`, and they are the real boundary: `--auto` approves anything not explicitly
denied, so a permission left at `ask` in a headless run is a stall, not a guard.

Those denies bind because of *where* they are written and *how* they are spelled. A tool's rule is
either a bare `allow`/`ask`/`deny` keyed by the tool's name, or -- for `bash` -- a table of command
patterns. A pattern written at the top level is read as an **action**, and no tool reports an action
named after a shell command, so `--auto` approves the command anyway: measured with the same entry both
ways, flat `gh pr merge 999999` ran, nested under `bash` it returned `Permission denied: shell`. Check
the keying of any copy of this harness with `opencode debug agents --standalone` -- every rule must
come back as `"action": "shell"` with the pattern in `resource` (H-0013). Without `--standalone` the
CLI answers from a running server's startup config, which is a different question.

Spelling matters the same way, and getting it wrong is quieter. The matcher globs the **whole** parsed
command line, so a pattern that opens on a literal only matches a command that opens on that literal.
Measured on v2.2.5 at `869564a`: 17 bash rules, **7 of them inert** for exactly that reason --
`gh pr merge*` allowed `gh -R vicowa-botworks/parental-lockout pr merge 218`, which is the form `gh`'s
own `--repo` flag produces. So a denial whose program can *open* a command needs **both** shapes: the
`* ` form reaches the flag-inserted command and the literal form reaches the bare one, and neither
covers the other. Deleting the literals to fix round three is what re-opened the bare ones -- at
`37120a8`, `rm -rf`, `systemctl --user stop <the timer>` and `go get` went from denied to allowed in
the commit written to repair the list. Where a flag can sit between the program and the subcommand the
wildcard starts at the subcommand: `* pr merge*`, not `* gh pr merge*`; that is also why the `git` and
`gh` rules need no literal twin, since their patterns key on a subcommand and a command always has
something before it. Two families are deliberately narrower than their broad form: `* tag *` needs the
argument so `git tag` can still list, and `* release create*`/`delete*`/`edit*` leave `gh release list`
readable -- a deny that breaks a legitimate read teaches the next tick to route around denies.

`python3 scripts/check-denies.py`, which `make verify` runs as *agent deny list*, is what stops this
from being a matter of reading carefully. It resolves the agent's rules through the CLI, matches whole
commands against them, and asserts both directions: every denial refused in both spellings, and a
short allow-list (`git describe --tags --always`, `git tag`, `gh release list`, `git push origin feat`)
still allowed, because an over-reaching list shuts the harness down and a check that only asserts
denials passes a config that forbids `git commit`. It also refuses a config pattern that no command
here witnesses, and a resolved pattern the config does not contain. It asks the CLI with
`--standalone`, and that is not a detail: without it the CLI answers from a *running server's* startup
config, and measured on 6 Nov, replacing the file with the broken list and running the check in the
same breath still returned the good one -- 23 rules, green. A check that reads yesterday's ruleset is
worse than no check, because it prints a number. It skips, loudly, where opencode is not installed
(CI), and `CHECK_DENIES_REQUIRED=1` makes a skip a failure.

Two limits stay on the record, because both are the kind that gets forgotten. First, this reads the
resolved ruleset, not a decision: `opencode run --auto --agent factory` has never executed a command
inside these rules on either machine involved in #217 -- it dies with `UnknownError` on the reviewing
box -- so the matcher's semantics remain an assumption shared by the probe and the fix, and one landed
live tick would settle more than another round of pattern-shaping. Second, a whole-line glob cannot
tell a command from a quote, so these patterns match **inside arguments**: `git rm -r --cached legacy`
is denied, and so is `git commit -m "explain why * rm -r* is denied"`. That is expected behaviour, not
a bug to route around -- the response is to reword the message, or to write the note in a file with
`write` rather than in a `-m` string. Narrowing the patterns further is not available: `* rm -r ` with
a trailing space matches the quoted form just as well.

Keying these patterns under `bash` has a second consequence: the CLI's command scanner now parses every
command the agent types, so a command shape the scanner cannot analyse is refused outright rather than
passing unmatched. That is the price of a deny list that actually evaluates, and the skill states the
command shapes that keep a tick on the right side of it.

And it is a constraint on a cooperative agent, not a sandbox: it binds the command shapes it can
spell. The boundary against irreversibility is that a tick cannot merge at all, and the supervisor
re-reads all four gate fields from the API in the same call that merges.

Merging cannot be undone here anyway: `allow_squash_merge=false` and `allow_rebase_merge=false`, so
every merge is permanent history.

## Lifting it out

Nothing in `factory/` may name this repository. `FACTORY_REPO` comes from `origin`, the reviewer is
an override, the gate it runs (`make verify`) is whatever the repository's Makefile calls verify,
and the agent's deny list is generic except `go get`. Say the honest half of that rather than leaving
it out: the two review defaults do name this organisation (`workers`, `hermesvicowa`), exactly as
they always did -- they are the *values* of two overrides, the same class of thing as a reviewer's
login, and a repository that wants another target changes them in its unit or its environment rather
than editing the file. What the rule forbids is a line that needs the repository's *name* to work.
That is the price of the second half of this
task: once this has survived a real queue, `factory/` moves to a shared location and each repository
keeps only its `.opencode/opencode.jsonc` and `DECISIONS.md`. If a line here ever needs this
repository's name to work, that is a defect in it, not a convenience.
