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
| `conflict` | `CONFLICTING/DIRTY` | supervisor: `git merge origin/main`, with the decision-log union tried first (#381: a both-logs conflict the shipped gate certifies is merged and pushed without a tick); a tick only if the merge still conflicts |
| `address_review` | newest `CHANGES_REQUESTED` is newer than the head commit, **and its body carries an ask** | a tick, and the ask rides in its instruction |
| `review_unreadable` | that same change request, with nothing in its body: empty, a bare path, an `@`-mention, or under 40 bytes | nothing: one throttled comment says what arrived, and the row waits |
| `checks_failing` | a check concluded failure | a tick |
| `unclaimed` | open issue nothing claims -- no closing reference from an open pull request, and no claim verb in an open body. A merged body that named it and was refused rides along as a `[merged:PR#…]` marker | a tick |
| `delivered` | an **issue**: a merged pull request claims it, it landed on `main`, the issue has said nothing since, and no earlier closure was rejected. A **branch** (#291): a merged pull request ran under that name into `main`, and the ref holds no content the trunk lacks. Either way the row names the pull request, **the branch that carried it**, the instant and the words | nothing: the supervisor logs it, and closes nothing |
| `parked` | an **issue** carrying the escalation label -- the name comes from `FACTORY_ESCALATION_LABEL` (`factory/common.sh`) and is read through `factory/escalation-label.sh`, never spelled here; it **must be a label this repository carries**, because a name it does not have parks nothing (#311, settled 18 Nov as H-0250 after six days of a default that could never fire). It is the marker a unit applies when it reaches one of the four decisions that are not its to make (#313). `delivered` outranks it: a fix verifiably on the trunk is a delivery, not a wait | nothing: the supervisor logs the row and dispatches no tick |
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

`parked` is the same blindness on the other side of an escalation (#313). `AGENTS.md` and the skill end
a unit that reaches a reserved decision with the same three steps -- comment on the issue, apply the
label, stop -- and only the comment was machine-visible: `grep -c labels` answered `0` for `triage.sh`
and `0` for `factory.sh` at `65e980b`, re-measured at `f5e0fe9`. An honest escalation ships no pull
request, and an open body's claim verb plus `closingIssuesReferences` are the only things standing
between its own ticket and the next tick. Measured 13 Nov: `#301` went out at 20:11, 21:07 and 22:03Z
and each pass ended `TICK-FAILED … exit=124 (124 means the 2700s ceiling)`, its `updatedAt` unmoved
since the previous day because the terminal write is the first thing a truncated budget costs -- while
`#284`, an open enhancement with no claim of any kind, never reached a tick at all behind it. So the
class is read exactly the way `delivered` is: `triage.sh` asks for `labels(first:10)` in the issue query
it was already making, and emits the row in the place `unclaimed` would have taken, which is why a real
delivery still wins the row.

Two things about that read are said on stderr rather than left implicit. The configured name has to be a
label the repository actually carries, so a pass that parks nothing says so in words, naming the name it
looked for; the alternative is a queue that quietly agrees with itself that nobody ever escalated
anything. For six days that sentence was true of the shipped default, and it was the only evidence that
the park arm was inert -- `PARKED` appears zero times in every journal this harness has kept, while units
had been escalating honestly since #301. #311 settled the name on 18 Nov (H-0250), so the finding is now
about a rename or a deletion rather than a default that could never fire. Where it goes changed in the same
change, because a finding nobody keeps is a finding nobody reads: `triage.sh` runs inside a command
substitution, so its stderr went to the service's stderr -- journald -- while `supervisor.log`, the file
the section above tells a person to read, recorded nothing. Every line of it now arrives as
`TRIAGE-STDERR`, bounded at twenty-five lines. And `labels(first:10)` is a bound that
fails one way only: a match at the tenth label is still a match, so the sentence names just the issues
that filled the list without matching anything, whose park status is unknown and which therefore stay
workable instead of being called parked. What the marker may not do is a leg rather than a sentence:
every label-bearing line in `triage.sh` may name `parked` and no other class, so a unit's own marker can
stop a dispatch and can neither pick its next assignment nor grade its own work as delivered.

The release is the one write the read above never states, and it belongs here because `delivered` states
its override outright: take the label off the issue -- `gh issue edit <N> --remove-label <the name the
row printed>`, the name `factory/escalation-label.sh` prints and never a spelling of it here -- and the
next pass reads that issue as workable again. Nothing is reopened and no marker is cleared, but the queue
reads only one of an escalation's two writes: `is_parked()` sees the label, never the thread, so the
label alone stops and releases the row while the comment is what the next dispatched tick will read as
the answer. What `delivered` gets from `FACTORY_DELIVERED_WINDOW`, a park does not have: a park has no
expiry, so the label is the only release and a label nobody removes prints `PARKED` on every pass. That
repetition is the `delivered)` arm's own choice -- a queue that reports a thing once and then goes quiet
is a queue nobody can audit -- except `delivered` stops printing on its own when the window runs out and
`parked` stops only when a person acts, which is why the row carries the name it read.

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
both stayed a coin flip landing about once in 200 passes. `H-0169` closes it at the source -- the
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

A tally sentence is one `printf` format, and one format has to be one word. #329 found both tallies
shipped with the literal closed immediately before a `\<newline>` and the rest of the sentence on an
indented line: the shell deletes the continuation and what is left between the two literals is that
line's indentation, a field separator, so `printf` received a format with no `\n` in it plus the literal
`offered: %s\n` as its first argument, and re-applied the format once per remaining argument. What an
operator got was three lines, the first of them the format string. `bash -n` and shellcheck both exit 0
on that file, and a leg that greps the sentence for a fragment is green on the result -- the format's own
words are exactly what got printed -- so the assertion here is the whole sentence, and
`scripts/check-scripts.py` refuses the shape in every shipped script (`H-0154`).

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

The same sentence, however, is a verdict about the trunk only, and #347 measured what happened while it
was heard as one about the tree: issue 328's 991-line repair sat uncommitted in `wt/issue-248` -- no
blob, no ref -- behind a `DELIVERED` line repeated every activation, because the class never opened the
worktree its row named. The arm now reads that tree with the same dirtiness reader a dispatch halt uses
and adds a second, report-only line: `DELIVERED-TREE-DIRTY` names the path and every dirty or untracked
file, and `DELIVERED-TREE-UNREADABLE` names the directory that cannot answer what branch it is on
(#232's stale entry). A clean tree, a missing tree, and a tree checked out under another name stay
silent -- the checked-out branch is the whole attribution -- and everything stays report-only, because
the uncommitted work a delivered tree holds is a person's: no stash, no bundle, no abort, the verbs
issue 328 nearly died of (H-0199).

Measured against GitHub and this machine's refs at ~21:00Z on 12 Nov, both ends from the same tree: **32
rows before the change, 69 after**. The 37 that appear are all `branch delivered`, every one of the 11
`branch_without_pr` rows is still offered, and the tally falls from three refs to two -- the one of them
a merged pull request names is now reported by number. Thirty-seven is what #291's own census predicted
(34 leftover refs, 33 silent, counted 11 Nov), and it is also the price: a supervisor pass wrote 13
journal lines the day before this paragraph and writes 50 after it. Nothing in the harness bounds that
file or the transcripts beside it -- `log()` appends to `$FACTORY_LOG/supervisor.log`
(`factory/common.sh:373`) and no code rotates, truncates or prunes it, nor the `<stamp>.<unit>.log`
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

### The ref that is not a contribution (#284)

`branch_without_pr` asks a ref one question: does it hold content the trunk lacks? The class name is
about the missing pull request, and both halves of the decision were being made by the wrong thing.
Two refusals now exist, one on the name and one on the content, and neither may start a tick.

**The name.** `factory_refuseto_work` (`factory/factory.sh`) listed the families with a slash and
nothing else -- `backup/* local/* integration/* wip/* tmp/*` -- so a hyphen spelling walked straight
past it and `preserve/` was not on the list at all. Measured over the 11 refs ahead of `main` with no
open pull request at 00:40Z on 14 Nov, that let exactly three through: `backup-tip` (7 files across 5
commits), `preserve/issue-222-stale-base` (2 across 2) and `preserve/issue-318-wip` (6 across 3),
while four `backup/` and `local/` refs were already refused and the four `issue-*` names in that census
are untouched by the pattern -- and none of those four prints a row today: `issue-248` reads
`delivered` behind merged PR #262, while `issue-309` and `issue-318` sit one commit ahead with zero
files, which is the empty commit that makes `git cherry` useless below. The journal agrees the arm was
firing for what a slash could reach: **24 `REFUSED branch` lines** between 11 Nov 09:54:49Z and
14:14:10Z, so the 12 Nov addendum's "refuses nothing" was false the day it was written;
what was true is that it refused only what a slash could reach. The pattern is `backup[-/]*` and the
same for each family, plus `preserve[-/]*`, and #291's ask-1 control blinds the separator to demand
`backup-tip` be dispatched again. Two costs, measured against the live names rather than reasoned:
`preserve-scroll-position` and `tmp-dir-cleanup` are refused, because a family word is also the first
token of a feature branch, and bare `backup` is not, because `[-/]` asks for a family or a path and
not for the word. `preserve[-/]*` is a decision written above the pattern, not a family inherited
from the list: a preservation branch says *keep this*, not *work on this*, and the arm has always made
that trade for `wip/`. The row survives either way, because the row is a report and the arm only stops
the dispatch -- `REFUSED branch:<name> the name says it is a backup, not a contribution`.
**The content.** A ref whose work is already sitting in an open pull request is not stranded work, and
`git cherry` cannot tell you that: it marks a commit `-` when the commit is **empty**, not when the
work landed. Measured at 00:50Z on 14 Nov, `issue-248`, `issue-309` and `issue-318` are each one empty
commit with zero files and all three read patch-equal to every non-draft open head -- a verdict about
the shape of a commit, never about whether work landed. The reading is bytes instead: the head itself if the ref is contained
in it, otherwise one `git diff` between the two trees restricted to the files the ref touches. The
merge-base form was written first and is wrong, because a change cherry-picked onto the pull request's
head still names the file in a merge-base diff; `br-pick` in the fixture is that case, and a leg pins
that the byte reading and not containment is what refused it. A draft head buys no refusal: a draft
never merges, so refusing a ref because a draft's bytes match it retires work no merge will ever
deliver. The case that taught it was draft #285 on `backup-tip`, titled "reference only, do not merge"
and closed unmerged at 16:34:58Z on 12 Nov. Up to
`FACTORY_DELIVERED_WINDOW` merged pull requests are consulted, and open heads up to
`FACTORY_DUP_SCAN` (40, against the 5 open heads measured 00:45Z on 14 Nov); past either bound the row says
`[dup-scan-limited:N …]` and draws no conclusion rather than a cheap one.

The refusal is a **mark**, not a class: `[content-on:PR#901]` rides in the `branch_without_pr` detail
and the arm in `factory/factory.sh` refuses on its first characters, the way the `delivered` arm reads
`[merged-pr:`. A class would have been the smaller diff and the worse one -- the detail is prose, and
inside it sits somebody else's branch name. Both refusals are pinned by running the supervisor over a
fixture ("the marked ref is refused by content, naming the pull request", "the hyphenated backup is
refused by name", "…while the ordinary branch is still worked") rather than by reading the arm.

No live ref triggers the content refusal today: the shipped classifier run against the real queue at
00:45Z on 14 Nov prints 11 `branch_without_pr` rows and 37 `branch delivered`, with no
`[content-on:PR#` mark and no `[dup-scan-limited` note -- the reading asks all five open heads and
concludes nothing. That half is
pinned by the fixture alone, and what it buys is the day a head and a branch carry the same commit.
What the name half changes live is three refs, `backup-tip`, `preserve/issue-222-stale-base` and
`preserve/issue-318-wip`: they keep their rows, because a row is a report, and lose their tick.

### The merge that closed nothing the body said it closed (#353)

A closing keyword is a request to GitHub, not an event in this harness, and until now nothing ever
asked whether the request was granted. `factory_verify_closure` (`factory/factory.sh`) is called at
the end of `factory_merge`, after the ledger is stamped, and it reads back every issue the merged body
named with a verb.

The population, all 180 merged pull requests here, read 14 Nov 2026 and classified by where the verb
sits in the markdown (`scripts/check-close-verb.py` ships the classifier, and the audit that counts
this table drives that same shipped pattern instead of a re-typed copy of it):

| where the keyword is | pairs | closed within 2 s of the merge | closed later |
|---|---|---|---|
| plain text | 100 | **98** | 2 — #207 at +159415 s, #211 at +27586 s |
| inline code, with no plain-text pair naming the same issue | 2 | **0** | 2 — #304 at +6086 s, #301 at +14472 s |
| a fenced block | 0 | — | nothing in this repository has ever done this |

Nothing at all landed in the three-to-120-second bucket, which is what lets the ≤2 s cluster be read as
the parser firing rather than as human latency: GitHub takes under two seconds or it did not do it. The
two inline-code specimens are PR #308 (`` `Closes #304` ``) and PR #338 (`` `Closes #301.` ``, on a line
that states the keyword is deliberate). #301 is the expensive one: it stayed open until a second unit
was dispatched to re-derive work `main` already had, and it was closed 14472 s after the merge that
claimed it, by that unit's own delivery comment rather than by the keyword.

**The pattern is GitHub's keyword set, and the verb has to touch the number.** `close closes closed fix
fixes fixed resolve resolves resolved`, then at most a colon, a comma or a tab, then `#N`. Both halves
are measurements. No `-ing` form is in GitHub's set, and the shipped line used to carry two of them
because the stems had been copied from `triage.sh`'s claim matcher rather than from the keyword list.
The copy was incomplete as well as over-wide: `fix(?:e[sd])?` has no `-ing` arm, so one merged body's
shape decided which `-ing` verb the harness believed -- `Closing #77` and `Resolving #78` scanned as
requests while `Fixing #79`, the same spelling one stem over, did not. Neither spelling was GitHub's.
The filler window mattered more. The pattern used to allow `[^#\n]{0,20}` -- twenty characters of
prose -- between verb and number, and a window invents claims: over the same 180 bodies it answers 115
plain-text matches where 100 exist. Seven of the invented pairs name real issues no sentence asked
about, seven name pull requests, which no closer can act on, and one names `#1180`, a cross-repository
citation that is not an artifact of this repository at all. Six of them ship as fixtures in the checker,
each quoted exactly as it was merged -- `pull request #292 has the fix. Refs #291 for context only.`
read as a request for #291, which is still open; `needs the boundary resolved by hand, and #1180 is
why` read as a request for #1180; and four more of the same shape, one of them naming an issue closed
29398 s *before* the merge that "closed" it. An invented closer is not a harmless over-read here: every
one of them becomes a
sentence written onto a live ticket by the function above, so a false positive is a false statement in
the artifact. Both halves are pinned twice now -- the checker carries the six merged lines as fixtures
that must not be reported as closers, and `factory/selftest.sh` runs the two shipped spellings of the
pattern, the jq one in `factory.sh` and the Python one in the checker, over one table of lines and
refuses any line where they disagree.

**One verb, every number it touches, and what kind of thing that number is.** The scan used to ask `jq`
for a `capture`, which answers the FIRST `#N` on a matching line and nothing else, so a line whose verb
is not the first thing on it reported the wrong ticket and never read the right one: `#7 is the reason.
Fixes #12.` came back as `7`, and GitHub's own multi-issue sentence came back as its first number alone.
`scan($re)` returns one entry per match -- the pattern's capture group, so bare digits -- so a number is
reported only when a keyword was touching it. Kind matters as much as position: a pull request is also an
issue to the REST API, so
`issues/282` answers `state: closed` with a real `closed_at`, and a body may point its verb at a merged
sibling -- #292's says `its fix merged as #282 at 11:01:37Z on 11 Nov`. The pass now reads
`pull_request` in the same call as `state` and `closed_at` (one read, because two reads of one artifact
can straddle a write, and a delta built across them is a delta nobody observed) and prints
`CLOSE-NOT-ISSUE pr:N ref:M` for a pull request instead of an issue verdict. The separator between those
three fields is U+001F, not a tab: tab is IFS *whitespace*, so `IFS=$'\t' read` collapses the empty
middle field of an open ticket and the kind lands in `closed_at` (measured). Both arms grade through one
`factory_close_verdict` now, because the re-probe arm carried its own copy of the verdict and that copy
had only the healthy branch: a ticket that closed during the probe with no readable timestamp printed
`closed unknown s after the merge, inside the window GitHub takes`, and one closed 40400 s after the
merge printed the same healthy sentence with the window at 120 s. And the re-probe is one wait for the
batch rather than one per ticket -- the sleep sat inside the loop, so five open tickets held
`factory_merge` for fifty seconds before the worktree cleanup ran.

**A commit message is text GitHub parses too, and nothing read it (H-0166).** The body file is checked
at review time; the messages were never checked by anything. Measured on the branch that added this
paragraph: `git log --format=%B origin/main..HEAD` at `17b37c7` answered two verbs adjacent to a number
inside one commit message, in the sentence written to warn about them -- the PR #245 shape from #1256,
one layer down. Both of its numbers happened to be artifacts already closed when the message was
written (#77, #78, read back with `gh issue view N --state all --json number,state,closedAt`), so that
merge would have changed nothing. That is luck about two numbers, not a property of the shape: the same
sentence over two open issues closes them with no review and no note. So `scripts/check-commit-verbs.sh`
runs the checker over every message between a basis ref and HEAD -- `origin/main`, or `main`, or the
`PLOCK_COMMITS_BASE` or first positional argument -- one commit at a time so a refusal names its commit,
and `make verify` runs it. Two rules about what happens next are worth stating, because both are places
where a guard can quietly become a lie:

- **The remedy depends on whether the commit is published.** Not published: rewrite the message. Already
  published: do *not* rewrite it -- `git revert` is the only rollback here, force-pushing an in-flight
  branch is forbidden, and every merge on this repository is a merge commit, so the message is permanent
  history and the claim cannot be recalled. Read the named artifacts back and decide deliberately, then
  write the decision into `factory/close-verb-waivers`: full sha, the exact number set the report prints,
  a reason that says what was read back. The waiver is checked for exactness and *reprinted on every
  run* -- a green line that hides an adjudication is indistinguishable from no adjudication. That only
  published claims are waivable is a reviewer's rule, not a mechanical one; what is mechanical is that a
  waiver naming fewer numbers than the report, or more, leaves it failing.
- **An undocumented inflection is refused even though nobody knows what GitHub does with it.** The
  published keyword set has no `-ing` form, and all 180 merged bodies of this repository contain exactly
  one adjacent `-ing` specimen (PR #47's `closing #46`), so the corpus cannot settle the question -- one
  observation, confounded because that same body also carries a documented keyword on the same number.
  The two sides of this tool answer the uncertainty in opposite directions on purpose: the *read* side,
  `factory_verify_closure`, whose verdict is written onto a live ticket, keeps the documented nine so it
  never invents a claim (H-0164); the *write* side refuses the superset, because being wrong there costs
  a reworded sentence and being wrong the other way costs an unplanned close. A documented keyword in a
  message is still allowed -- a leg pins that, because a guard that refuses every number in every message
  would pass this section's own bite test for the wrong reason.
- **Three scans, and only one of them strips code spans.** `scripts/check-close-verb.py` reads a *body*
  with spans stripped, because that a span is inert is a measurement made over merged bodies on 14 Nov
  (a keyword inside inline code and nowhere else fired 0 of 2). `factory_verify_closure` reads a body
  span-blind, because the specimen that started #353 *is* a verb inside a span and the read side has to
  see the disagreement between what was asked and what happened. `check-commit-verbs.sh` reads a
  *message* span-blind for a third reason: nobody has ever measured spans in a message, so no span is
  treated as a shelter there, and the direction of the doubt is the one that costs a reworded sentence
  rather than a close. Every one of the three is refusing to assume; a single shared "right answer" about
  backticks across all three would itself be the assumption.

**The scan is span-blind on purpose, and a tidier one would have missed the defect.** The obvious
implementation strips code blocks and inline spans first, because that models GitHub. This one matches
the verb wherever it appears, inside backticks included -- the same way `CLAIM_RE` in `triage.sh` does.
The specimen *is* a verb inside a code span: a span-stripping scan reports no claim, compares nothing,
and passes the merge that started this ticket. The disagreement between what the body asked for and what
the artifact got is only visible to a reader that reads the body the way the harness's other reader does
and then checks GitHub's answer. What closes and what does not is reported, not assumed.

**What it does when a ticket is still open: write on the artifact, close nothing.** Three options were
weighed (H-0156): close with a comment naming the merge commit, comment only, or halt the pass. Halting
is refused for the reason #341 documents -- one un-closed ticket must not park every later activation.
Closing is refused for the reason this repository has been taught twice: a verb is not evidence, and
#1256 is the record of a keyword firing on a line written to *negate* it. Whether the delivered artifact
met the ticket's requirements is a judgement with measurements behind it, and the merge does not carry
those measurements. So the failure is written into the issue, where the next reader of the issue finds
it, and the state change stays with a person.

The note keeps the quoted verb lines inside a code span of their own. That is load-bearing and a leg
pins it: a comment that put `Closes #301` in running text would close the ticket it is reporting on, by
accident, with no measurement behind it -- the exact failure mode being documented. The checker from
Step 5 reads the generated note and reports no firing keyword in it.

**The healthy path writes nothing.** A ticket closed within the window is not a fact worth a journal
line; #334 priced the pass at 37 lines already. Three sentences appear only on the abnormal branches:
`CLOSE-OPEN` (the ticket is still open after the re-probe, and a note was attempted), `CLOSE-LATE`
(closed, but `${FACTORY_CLOSE_FIRED_S}` -- default 120 -- seconds of slack separate a keyword from a
hand close; the measured population has zero pairs between 3 and 120 s, so any bound in that range
separates the same two sets, and 120 states a window rather than tuning one), and `CLOSE-UNTIMED`
(closed with no readable timestamp, which is not the same answer as a delta of zero). An issue that
cannot be read is `CLOSE-CHECK-UNKNOWN`, and a body that cannot be read is `CLOSE-CHECK-SKIPPED`:
neither is ever reported as an open ticket, because a failure to read is not a finding.

The re-probe is one sleep, `FACTORY_CLOSE_PROBE` seconds (default 10), because GitHub's closer is
asynchronous and a single `OPEN` read milliseconds after a merge means nothing; 10 s is five times the
widest delta in the measured population. A closer that lands between the two reads is `CLOSE-SETTLED`.

Nothing here closes an issue, and nothing here claims a requirement was met.

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

The order is what lets the marker be wrong, and for six days it was wrong: the instruction carried a name
this repository does not have -- eighteen labels, none of them matching -- so every escalation's write came
back refused and nothing parked, while the refusal itself went only to the service's stderr. #311 settled
that on 18 Nov (H-0250): the reader prints a label every repository is created with, so a stopped unit
needs no administrator before its marker lands. Measured 12 Nov
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
name a person can still supply; the other is a field GitHub has retired, so no label name at all makes
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

`SUPERVISOR-AT` is on every activation that clears preflight -- including one stopped by a
`factory/HALT` marker, since #331 hoisted the self-report loops ahead of the halt check -- the same
boundary `LOG-AT` documents, and the only activation that prints none of the self-report lines is a
preflight refusal, which journals nothing at all.
`SUPERVISOR-STALE`, `SUPERVISOR-FORKED` (the tree also holds commits
the trunk does not), `SUPERVISOR-NOBASE` (no `origin/<trunk>` ref to compare against) and
`SUPERVISOR-UNKNOWN` (git could not read the tree) appear only for the state each names, and
`factory/selftest.sh` pins all five against trees planted in each state. `dirty=` is a path count from
the same three content reads `factory_tree_dirt` uses -- never from `git status --porcelain`, for the
reason given in the section on dirty trees.
Since #342 the `SUPERVISOR-AT` line carries two more fields, `main=<sha>` and `behind=<n>`: the code
the activation runs and where the trunk stands, in one line a leg can pin instead of a reader grepping
two lines and joining them by hand. `main=?` and `behind=?` go with `SUPERVISOR-NOBASE`, and a `?` is
never printed as a `0` (#228's polarity).

**And since #342, the pass that prints `SUPERVISOR-STALE` acts on it, because acting is safe at that
instant and nowhere later.** A sentence was all the line ever was: by 13 Nov it had printed 92 times --
13h20m of a ten-minute cadence, with the guard reading the right tree at last since #260 -- and every
one of those passes went on to triage, allocate decision ids and halt on code 38 commits behind
`origin/main`, five of those commits executable harness files the running pass would have used. At the
point the line prints, the pass holds `factory/run/lock`, triage has only read the queue, no tick has
been dispatched, and the tree is clean by the same content read that named the state. After that point
a tick is running and the window is shut until a human shows up -- and a line that prints every ten
minutes forever is not a human.

`factory_supervisor_repair` (`factory/common.sh`) fast-forwards the tree the *installed unit*
`ExecStart`s -- the tree the loop actually runs, which #260 established is not necessarily either of
the two trees the lines above report -- to `origin/<trunk>`, when four conditions all hold of that
tree at that instant: it is the harness's own script, clean by the #228 content reads, `behind > 0`,
and `ahead = 0`. Every refusal is a named condition, printed and never acted on:

```
2026-11-15T09:00:00Z SUPERVISOR-AT 3962919 dirty=0 main=4f1a2c8 behind=3
2026-11-15T09:00:00Z SUPERVISOR-STALE origin/main is 3 commits ahead; the loop is not running merged code
2026-11-15T09:00:00Z SUPERVISOR-FF 3962919 -> 4f1a2c8 commits=3 tree=/srv/harness -- fast-forwarded the tree the installed unit execs to origin/main; this pass stops here with exit 9 and started no tick, the next activation (OnUnitInactiveSec=, seconds away) runs the merged code
2026-11-15T09:00:00Z SUPERVISOR-RERUN the pass stops here with exit 9: no triage, no id, no tick; the line above says what moved and the next activation runs it
```

(The shas are a fixture's own, in the manner of the excerpts above; the shapes are the shipped ones.)

- `SUPERVISOR-FF` -- the tree moved, the pass exits `9` before anything is dispatched, and the timer's
  `OnUnitInactiveSec=` hands the queue to the merged code seconds later. Stopping is not
  conservatism: the merge replaced the bytes of the script bash is reading incrementally, so a pass
  that *continued* would execute a mix of old and new code -- #260's reason against moving the tree,
  which still holds exactly. Exit-and-restart keeps the whole promise #260 made (bash never reads
  across the change) and closes the one gap it left: the gap was not that the move was unsafe, it was
  that nothing did it.
- `SUPERVISOR-FF-REFUSED <condition>` -- `dirty=n` (uncommitted work; #249's quarantine owns dirty
  trees and the repair will not compete with it), `diverged ahead=n` (a fork; only a merge or rebase
  moves that, and those stay human acts on a tree the timer execs), `not-ff-only` (every condition
  said fast-forward and git still refused -- the line quotes git's own `error:`/`fatal:` line, which
  is not its last line: measured against a locked index the last line is `Updating 568f92b..2fc8ebd`,
  and that distinction between *not safe* and *refused anyway* is the whole reason refusal is a token
  rather than a boolean).
- The streak and the escalation: an activation that reported staleness and did not repair it appends
  its condition to a ledger line (`supervisor.stale-streak`); a current tree or a completed repair
  clears it. `factory.sh --status` then prints a fifth verdict word, `STALE`, once the streak reaches
  `FACTORY_STALE_ESCALATE` (default 3), with the last refusal's condition in the sentence. `STALLED`
  is still asked first -- every repair sentence is written into `supervisor.log`, so a loop printing
  them is never stalled, and a loop gone silent has a newer fault than staleness. `STALE` replaces
  `WORKING`/`IDLE`, the two words behind which this outage sat for 92 activations.

`--dry-run` prints `DRY-RUN  git -C <tree> merge --ff-only origin/<trunk>` and touches nothing --
including the streak, because a dry run writes no ledger (H-0017). The exit `9` joins `8` in the
generated unit's `SuccessExitStatus=` (#246/#305's lesson: an unlisted deliberate stop journals as
`Failed with result 'exit-code'`, which points a reader away from the one line that explains it).

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

The unit carries a second thing besides the tree it execs: the `Environment=` set is the environment a
tick runs in, and #377 measured what never reading it costs. A live unit written 2026-11-10 carried
three `Environment=` lines while the generator emitted five -- the missing pair was
`FACTORY_REVIEWER_TEAM` (#241) and `FACTORY_SERVICE_UNIT` (#260) themselves -- and both components
still behaved correctly by falling back to script defaults, so the install was stale in a way every
component hid: the poke asked the team because a `${VAR-default}` said so, not because the install
asked, and the unit name resolved by the repository-name guess instead of the recorded line. So every
pass now compares the generator's `Environment=` **names** with the installed unit's and prints one
`UNIT-ENV-STALE` line per missing name (`factory_unit_env_lines`, `factory/common.sh`):

```
2026-11-17T14:02:11Z UNIT-ENV-STALE unit=<the installed unit> name=FACTORY_REVIEWER_TEAM -- the installed unit carries no Environment=FACTORY_REVIEWER_TEAM line, so it predates the generator that writes it; re-running factory/install-timer.sh is the fix and that is whoever maintains the timer's act, and until it happens the script answers this name from its own default (#377)
```

A warning, not a halt -- the pass works, and the drift removes behaviour only where an operator wanted
a lever the missing line carries: `Environment=FACTORY_REVIEWER_TEAM=` written *empty* is #241's
documented "ask the individual" override, unreachable from an install that predates the line. The
comparison reads names, never values, so the empty override counts as present and the verdict does
not nag the lever it exists to protect. And the repair is an operator act, never the pass's:
re-running the generator rewrites and reloads the unit that drives the queue while re-deriving `PATH`
from the caller's environment -- `H-0287` carries that split. Silence from this read means exactly
one thing, that the install carries every name the running generator writes; an unreadable generator
answers `UNIT-ENV-UNKNOWN` instead of silence, because a failed read is not evidence of the clean
state (#228's polarity, the same rule `UNIT-UNREADABLE` carries), and the uninstalled, foreign and
unreadable arms print no set verdict at all -- their own sentence is the honest one, and comparing an
expected set against a file that is absent, hand-written, or unreadable manufactures drift out of the
inability to see.

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

`LOG-AT` is on every activation that clears preflight, for the reason `SUPERVISOR-AT` and `UNIT-AT` are -- a
read nobody prints cannot be told apart from a read that is not there -- and it is also what the next pass
reads back as this pass's answer, so it is printed before the `PASS` line: a pass that halts at triage
writes no `PASS` at all, and that is exactly the pass somebody is trying to locate. **Where that boundary
is, and what sits outside it, since #331:** the pass's self-report lines -- `UNIT-AT`, `LOG-AT`, the reap
read and `SUPERVISOR-AT` -- are emitted by `factory_self_report`, ahead of the halt check, so an activation
stopped by a `factory/HALT` marker names the directory it writes `HALTED` into before it writes that line;
the hoist runs ahead of the pass's `flock`, and `H-0191` carries that call. What stays outside the
boundary: `PASS`, which reports what triage returned and cannot precede it, and the whole of a
`factory_preflight` refusal, which still journals nothing. The dead tail this section opens with is
therefore a pre-#331 journal: a lone `HALTED` line with no `LOG-AT` above it identifies a pass that ran
before this change, and `factory/selftest.sh` refuses that shape on every pass that runs after it. The previous directory comes from the
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


### The run directory holds a stream, not a verdict (#351, #311)

`factory/run/` holds the pass's lock and, for each stream, one more file. `factory_worktree`'s stderr goes
to `worktree-stderr.<pid>`, one per tick, so a refusal can be read back by name rather than folded into the
tree path — `2>&1` on that command substitution splices an error sentence into the value assigned to `tree`
and hands a later `git -C` a path with a period in it. **Both paths delete it**: the failure arm after
reading it into the journal line, the success path because a file left by a pass that got its tree asserts a
verdict nobody reached. Until #351 only the failure arm deleted it, and the file was named
`worktree-error.<pid>`. Since H-0250 the queue read follows the same rule with `triage-stderr.<pid>`: the
supervisor runs `triage.sh` inside a command substitution, so before that the classifier's own findings had
no file at all and reached only the service's stderr — journald — while the journal that this harness tells
a person to read stayed silent. Both streams are journalled and deleted on both paths; the sweep is the
backstop for a pass killed between them.

Measured at 00:54Z on 14 Nov: four of those files, all zero bytes, stamped 22:03, 22:59, 23:31 and 00:22Z —
the last four ticks, every one of them a **success** — while `grep -c 'no worktree' factory/log/supervisor.log`
answers **0**: the failure the name reports had never once been journaled on this machine. Three reads of that
directory on two days each concluded, from those four files, that the supervisor cannot build trees at all and
the queue is stalled. The `-mmin +180` sweep is the backstop for the one pass that cannot delete anything — a
tick killed by the wall-clock ceiling dies between the write and the delete — and it still matches the retired
name so the four files above do not outlive the change. `factory/selftest.sh` pins the three answers the sweep
must tell apart (retired name past the bound gone, stream past the bound gone, a fresh one spared because a
supervisor may be writing it now), pins that a tick which got its tree leaves the directory empty, and pins
that a refusal's reason reaches `TICK-SKIPPED … no worktree for <branch> ::` in the stream's own words; two
controls redden when the success-path delete or the sweep's retired term is taken out.

What the sweep is *not* for is the sentence that justified it for eight days — "the run directory collected
eight zero-byte `worktree-error.<pid>` files in a day", read as eight killed ticks. Four stood at once with
zero journaled failures, so they were successful builds, which is exactly why bounding by age only treated the
symptom: it leaves three hours of alarms standing in the directory at all times. Same rule as
`factory_journal_age`: a file that has to be maintained, and asserts something nobody confirmed, is itself the
defect.

### The conflict arm's stream lived outside that sweep (#364)

The sweep searches `$FACTORY_RUN` and nothing else. That was one of two places the stream could be written,
because the two call sites each named their own: `factory_tick` wrote `$FACTORY_RUN/worktree-stderr.$$`, and
`factory_resolve_conflict` wrote `$(mktemp)`, which lands under `$TMPDIR` — a directory the sweep does not
read, that is chosen per invocation by whatever `$TMPDIR` happens to say, and that on this machine is a
tmpfs. The conflict arm is the one that most often dies with the reason unwritten — it is the arm that
*blocks*, and `CONFLICT-BLOCKED` is the line that reads the file — so the pass that needed the backstop
was the one standing outside it. There was no second copy either: `die` inside a command substitution dies the
subshell, so the stream holds the reason once, and a pass SIGKILLed between the refusal and that line lost it
in the directory nobody opens **and** in the journal. That is the same outage as never writing it: the sweep is
what #351 called "the backstop for the one pass that cannot delete anything", and this pass could not delete
anything while getting no backstop at all.

Both arms now take the name from `factory_worktree_stream` (`factory/common.sh`), so there is one home and it
is the swept one. The legs in `factory/selftest.sh` do not read the two arms; they run the supervisor as a
process — a real conflict activation, `pr:900` with `mergeable: CONFLICTING` through the fixture's own `gh`
and `issue-900` held by a tree the supervisor did not build, so `factory_worktree` must refuse for its own
reason — and SIGKILL it at the `CONFLICT-BLOCKED` sentence through a `DEBUG` trap delivered by `BASH_ENV`. In
shipped code the run directory then holds exactly one `worktree-stderr.<pid>` containing `branch issue-900 is
checked out in …`, with no `CONFLICT-` line in the journal; the control puts the `mktemp` line back and the
same kill at the same sentence leaves the swept directory empty. A third leg refuses the drift that would undo
it: neither call site may spell the path out again, `factory/factory.sh` may not call `mktemp` on a code line
again, and the helper must answer with the swept path and the matched pattern, both.

`factory_worktree_lines` still writes its stderr with `mktemp`, on purpose and unchanged: it is not this
stream, and its failure arm prints git's own sentence straight into `WORKTREE-CENSUS-FAILED`, so those bytes
are not anybody's only copy of anything.

### The trunk's own verdict is read once per activation (#370)

`main` carried a push-workflow FAILURE for 1h49m04s while the loop journalled 36 `DELIVERED` rows and one
`MERGED` on top of it (measured 16 Nov on the API: run `44110925937`, queued 17:06:06Z, completed
18:55:10Z — the same 8274 s the ticket measures as "mergeable but held for 1h49m by a red check run").
Nothing in the harness reads the trunk's CI, and no check that does exist could have caught it: every
check read is aimed at a pull request — `factory_checks` asks `gh pr view --json statusCheckRollup`, and
#274's re-check reads a rollup again — and a `pull_request` check runs on a merge of the head with the
base tip. A pull request can therefore be green against a trunk that is red at the very commit its own
rollup vouches for. That is the class: *a green pull request says nothing about the branch it lands on*.

`factory_trunk_ci_lines` (`factory/common.sh`) is the repair — one read of the trunk's own runs, wired
into `factory_body` after the preflight `EXIT`/`DIRTY`/`LOCK` early returns and the `LOG-AT`/`UNIT-AT`/
`REFRESH`/`SUPERVISOR-AT` block, and once per activation rather than once per unit:

```
2026-11-18T06:12:03Z TRUNK-CI d4559fd5de2646e6ae4ad13755e81a50289c17d3 success run=44240957485 age=4021s since=2026-11-18T05:05:02Z
```

The command is `gh run list --branch "$FACTORY_BASE" --event push --limit 1 --json
databaseId,headSha,conclusion,status,createdAt` — the shape #370 measured with `gh` 2.46.0. The field
names in the ticket's *prose* (`workflowName`,`name`,`displayTitle`,`event`,`url`) are not the ones that
work on this binary: measured, `name` and `displayTitle` are not valid fields for `run list --json` and
the call dies. `createdAt` is the timestamp the line reports. One measured case splits the harness's
precedents: `gh api` answers `status`/`conclusion` UPPERCASE and `run list --json` answers lowercase —
the code compares `ascii_downcase` both sides.

Every arm prints a line; no arm prints nothing (H-0045's rule, and #228's polarity rule, ported — before
this read existed, an activation could not distinguish "the trunk is green" from "nobody asked"):

| arm | line |
|---|---|
| green trunk | the `TRUNK-CI <sha> <verdict> run=<id> age=<n>s since=<ts>` line above |
| red trunk | that line with `conclusion=failure`, plus `TRUNK-RED` naming the run id, the timestamp, the url-less pointer, and that the pass is reporting only |
| queued or `status != completed` or a conclusion not yet written | verdict `pending` — the rollup rule's own unfinished-shape case: completion is `status == completed AND conclusion != ""` |
| `gh` fails | `TRUNK-CI-UNKNOWN rc=2`, quoting gh's exit and gh's stderr — the `LOG-AT` rc convention (#359) |
| fetch bound expires | `TRUNK-CI-UNKNOWN rc=1`, naming `FACTORY_FETCH_TIMEOUT` — `REFRESH-TIMEOUT`'s convention; `timeout --signal=TERM --kill-after=5`, `TERM` lands inside the call and never reaches the `EXIT` trap |
| empty array | `TRUNK-CI-UNKNOWN rc=3` — the empty branch must be a *numbered sentence*, never an absence, so the polarity rule cannot be satisfied by silence |
| body off contract | `TRUNK-CI-UNKNOWN rc=4`: not an array, missing field, or an unreadable timestamp — never guessed, never silent |
| no `gh` | `TRUNK-CI-UNKNOWN rc=5` |
| `DRY_RUN=1` | a `DRY-RUN` line stating exactly what would run, and no call |

The bad-conclusion vocabulary is the shared `JQ_BAD_CONCLUSIONS` list in `factory/common.sh`, built from
`factory_rollup_rules`' bad list plus `cancelled` and `error` — the same list `JQ_CHECKS` uses, so the
two verdicts cannot drift apart by vocabulary. The trunk read is a verdict *reader*, not a third copy of
the rollup rule: `run list` answers one array with one row per run, so the rollup's dedup-and-prefer-
completed logic has nothing to do, and duplicating it would add an unexercised path. `cancelled` is red
because the rollup calls `CANCELLED` a fail; `skipped` and `neutral` are not, because the rollup does not
say they are. A `TRUNK-CI` line is the **trunk** verdict and a `CHECKS` line is a **pull request's**
verdict; nothing compares the two.

It is report-only. The read cannot halt the loop or refuse a merge on a red trunk, and that is the
decision, not an omission: a red trunk is the same shape as a stale trunk — a fact about a branch, not a
reason to stop delivering — and #341 measured what a standing halt with no escalation does to a queue.
#370 item 3 — whether a red trunk should refuse a `MERGE` — is an open **question** whose subject is the
merge gate, one of the classes the harness may not decide for itself; the boundary sentence is in "What
it is not allowed to do" below.

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
branch, bounded by `FACTORY_FETCH_TIMEOUT`, before it reads the queue -- so `refs/remotes` is
current by the time the allocator runs in a tick's shell. Since #342 the fetch belongs to every tree
the pass is about to compare or repair: the tree the harness runs from is fetched before the
supervisor's position is read, not only the primary checkout, or the fast-forward would be about to
merge an `origin/main` from whichever clock that tree last saw (and one fetch serves both when they
are the same directory, which they usually are). The command an operator types is still
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
`factory/run/lock` while the queue freezes behind a read. A clone whose refspec cannot fetch
`refs/heads/*` cannot learn a sibling's branch at all, and the activation says that too, as
`REFRESH-BLIND`. #309 is the half of that sentence the test missed: the test was a literal `*` anywhere in
the line, and `+refs/pull/*/head:refs/remotes/origin/pr/*` -- what someone adds to get pull-request heads
locally -- carries its star on the *destination* side while fetching nothing that ever fills
`refs/remotes`. Measured against the shipped allocator on a clone cut before its sibling pushed:
`D-0002` on stdout and `REFRESH … 0s old` on stderr, with `D-0002` sitting on `refs/heads/issue-sib`. So
the rule is one prefix per line, on the source side of the colon, and it is one rule for both components:
`blind_coverage` in the allocator is the read, and `factory_refresh_origin` journals from it -- through
`--get-all`, not `--get`, because `--get` answers with the LAST value of a multi-valued key (measured, git
2.53.0) and would call a healthy clone blind depending on the order two refspec lines were appended. Any
learning line satisfies it, so the `pr/*` line added *beside* the wildcard stays clean; calling that blind
costs a fetch on every entry commit and buys no wrong id. `+refs/*:refs/*` is called blind, which is the
true verdict for every tree this command reaches, because a bare mirror is refused as "not a readable git
checkout" before these arms run and a working clone carrying it cannot fill `refs/remotes` even when it
fetches -- git refuses the write, `fatal: refusing to fetch into branch …`.

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
comes back; where the line names a `config remote.origin.fetch` write instead, that write comes first,
because a fetch cannot widen a refspec. Nine counted legs and four controls cover it -- re-derive them
from the run rather than from this sentence, which was a row short of its own list until #309:

```bash
bash factory/selftest.sh 2>&1 | sed -n '/a current read says so/,/no origin is refused/p'
```

The two arms -- a refspec that cannot fetch a branch, and refs older than the bound -- each on a fixture
that trips that arm alone and each with a control deleting just that arm from the arm list; a clean read
pinned to stay quiet, with the control that makes the verdict unconditional so the same tree is called
blind too; the bound moved instead of the tree; a checkout with no origin at all; and the four shapes
#309 added at the coverage arm -- a `pr/*` refspec refused, the `pr/*` line merely added beside the
wildcard still clean, an absent key still not blind, a mirror's `+refs/*:refs/*` refused.

The activation's copy of the rule is pinned beside these, and had no pin at all before: a leg runs the
shipped `factory_refresh_origin` over a primary checkout planted with the `pr/*` refspec and refuses the
journal until it says `REFRESH-BLIND` *and* the allocator refuses the unit tree beside it -- one refspec,
one verdict, two components -- with a control deleting that arm's guard, which must send the journal quiet
while the allocator keeps refusing. Before #309 nothing in the file had ever run that function over a
narrowed refspec (measured: its only two call sites used the healthy wildcard), so deleting the whole arm
moved no leg while the comment beside it announced a control that did not exist. The limit is pinned too, on #303's own shape: a clone
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

Since #381 the steps above are what a *human or a tick* still does when two decisions reached for the
same number, because that collision is the case the code refuses on purpose. The routine shape -- each
side added its own entries, no digit shared -- is now machine work: the conflict arm runs
`scripts/merge-decisions.py` over the conflicted tree, the shipped gate arbitrates the union, and a
certified merge is committed, pushed and read back with no conflict tick at all. Every seam is a
refusal that leaves the old route intact: a shared digit, a pointer only one side drifted, a non-log
conflict in the same merge, and the merge is aborted and the tick dispatched exactly as before #381.
`factory/selftest.sh` pins both routes end to end through the real supervisor: a logs-only conflict
that must not cost a tick, and a mixed conflict that must still cost one.

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

### Provenance: which id the journal watched being earned

Acceptance prices the collision at the merge; it priced nothing about *attributing* one. #345 measured
the gap: neither side of the `D-0058` collision ran the allocator -- 4 of 52 checkpoint-creating ticks
executed it -- and afterwards every check saw two well-formed headings sharing one id with nothing
anywhere recording which side had earned it, because the allocator's answer existed as stdout for the
moment it printed. A number taken by eye was indistinguishable from a number handed out.

So the allocator's answer now leaves a witness, and every tick audits for its absence. Whenever the
allocator runs inside a tick context -- `factory_tick` exports `FACTORY_TICK_UNIT` and
`FACTORY_TICK_BRANCH`, and the allocator refuses to guess: a bare terminal run journals nothing -- its
answer is journalled in `supervisor.log` as `DECISION-ID <id> unit=<row> branch=<branch> (allocator,
<log>, <read>)`, failure-blind like the tree verdicts, because a write that cannot land must never eat
the id it is meant to accompany. At the end of every activation `factory_decision_provenance`
(`factory/common.sh`) compares each decision log's headings against that record: every id the branch
newly carries, with no allocation journalled for it on *that branch*, gets
`DECISION-ID-UNEARNED <id> … unit=<row> branch=<branch> log=<log>` -- attributed, once per unit,
recorded and never fatal. The report is deliberately about attribution, not behavior: an unearned id
is often harmless prose, and the human's verb is the yield procedure above, which a clean report would
have made a coin flip. An id earned on a sibling branch reports too -- that is the #345 shape, a
collision's other half is always somewhere the journal has never looked. And a tree whose `origin/main`
cannot be read journals `DECISION-ID-UNCHECKED` naming the failed read and renders no verdict, per the
`TICK-UNCHECKED` doctrine: every id in the tree is neither claimed nor refused by a read that did not
happen. `H-0195` carries the option set, including the two refused for measurement: dispensing ids at
claim time (a claim cannot know the unit will decide) and a gate-level duplicate refusal (it arrives at
merge, on the wrong branch's bytes, saying `duplicate` where the question is `whose`).

## The decision log is migrating to one file per entry (#429)

`H-0263` lands phase 1: `scripts/check-decisions.py` now reads a union of four sources -- the two
monoliths, then `DECISIONS/*.md`, then `factory/DECISIONS/*.md` -- and the two directories are optional,
because on 2026-11-18 neither exists at `origin/main` nor on any of the 11 heads the server lists, and
refusing an absent one would refuse every tree in flight. Three rules come with the layout. A filed
file's name is its id: `DECISIONS/H-0042.md` holding `## H-0043` is a renumbering that silently aliases
every citation to `H-0042`, so `layout_problems` refuses the pair. An id used twice in the union is
refused with both locations named -- one filed file beside one monolith heading included, which is the
rule that makes the split safe to move in batches. And a `*.md` filed *below* an entry directory is
refused by its path, because no id names a path segment and a file the reader skipped is an entry no
rule ever ran over.

Both forms stay valid until phase 3 completes, so **do not convert an entry by hand**: the allocator
(`factory/next-decision-id.sh`) and every writer still append to a monolith, which makes a hand-filed
entry a file nothing will produce again. What phase 1 does not do: emit files (that is phase 2), move
the 318 entries that exist today, or retire either monolith -- deleting one is a class reserved to a
human. The reading order is the monoliths first, then filed entries in filename order, and id
monotonicity stays the allocator's job: the positional sequence rule is left inside a file, because a
directory read in sorted order has no order a merge honours -- measured on a fixture where a back-filled
lower id sits in the directory and the union reads clean at `4 entries across 4 log(s)`.

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
| `FACTORY_TICK_KILL_AFTER` | `60` | the grace between TERM and SIGKILL when a tick is reaped, seconds (#383) — the number `TICK-FAILED` prints in its 137 sentence, which is why it is a constant and not a literal inside the `timeout` argument |
| `FACTORY_BUDGET` | `5400` | one pass's ceiling, so activations cannot overlap |
| `FACTORY_FETCH_TIMEOUT` | `120` | the ceiling on each `git fetch` an activation does (H-0075; #342 adds the harness tree beside the primary checkout, so a split arrangement fetches each); hit it and the pass dispatches on the refs it has, and says so |
| `FACTORY_ID_MAX_AGE` | `300` | how old the refs behind a `next-decision-id.sh` answer may be before the read is called `REFRESH-BLIND` (#303). The age of a read, not a reservation: see the allocator section above |
| `FACTORY_MAX_TICKS` | `3` | sessions one pass may start |
| `FACTORY_HALT_RELEASES` | `2` | how many times one unit's machine-written halt may remove itself when the tree it names is provably past its cause (#341); see the section under "Stopping it" |
| `FACTORY_STREAK_HALT` | `0` | whether one unit's second preserved accident halts the whole repository as well as costing that unit its turn (#341 demand 3). `0` is the shipped scope: the unit loses its pass and every other unit is ticked. `1` is the pre-#341 escalation, kept reachable because it is the shape the halt re-probe is written against and because an operator may decide one unit's repeats are worth stopping the queue for |
| `FACTORY_REVIEW_POKES` | `1800` | the review-poke floor, seconds |
| `FACTORY_ESCALATE_AFTER` | `86400` | how long in a non-progress state before one comment |
| `FACTORY_STALE_ESCALATE` | `3` | how many consecutive activations may report the installed tree behind and not repair it before `--status`'s verdict reads `STALE` (#342). Three: enough to ride out one pass whose refusal cause clears, short enough that nobody reads 92 journal lines to call the outage a defect |
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

**Where a unit's scratch belongs (#415).** A unit's own throwaway fixtures -- probe repositories,
fake trees, anything written to answer a question and thrown away -- belong in the scratch directory
the dispatch mints for it: outside every worktree, exported as `$FACTORY_SCRATCH`, named in the
tick's prompt beside the tree, deleted when the session ends whatever the session did to it,
including being killed mid-tick. They do not belong in the unit's own tree, not even in a
dot-directory: the content verdict cannot tell a half-finished deliverable from a scratch directory,
and a halt is global. Measured 15-16 Nov in the supervisor's journal: one `.probe/` left in
`wt/issue-328` wrote 39 `HALTED` lines, the last of them recording a cause already 7h39m old. If
`mktemp` cannot mint the directory the pass journals `SCRATCH-FAILED` and dispatches without the
sentence; the rule still stands, it just has no address to give. And when a halt does stand and its
cause is named as untracked directories, clear them with `git -C <tree> clean -ffd -n` **before**
`rm factory/HALT`: `git clean -fd` refuses to touch a directory containing a `.git` -- exactly what a
unit's fixture repositories are -- so over that directory the plain dry run says only
\`Would skip repository\`, never prints a \`Would remove\` line, and the work stands while the
resume re-halts on the same bytes (measured on git 2.53.0: plain said one skip line at exit 0;
\`clean -ffd -n\` said \`Would remove .probe/\`). The halt sentence carries that caveat for the same reason.

**What the supervisor stops for (#341 demand 3).** It writes the marker itself over two shapes, and
they have one thing in common: the work is nowhere else. A tree whose rescue was **refused** -- git
would not stash it, or the tree cannot be read back clean afterwards -- journals
`TREE-QUARANTINE-REFUSED` and then halts, `cause=dirty` or `cause=unreadable`, at whichever call site
found it (`TICK-HALTED` before a tick, `CONFLICT-HALTED` before a conflict resolution). And, with
`FACTORY_STREAK_HALT=1`, a unit whose tick has now stopped short of delivery twice in a row.

A second accident whose work the harness **did** preserve is no longer a repository-wide stop. The
first is absorbed silently (`TREE-QUARANTINED … attempt=1`, work in a stash and a patch, the queue
ticked on); the second used to be `TREE-STREAK` and a marker, and it is now
`UNIT-QUARANTINED <unit> attempt=2 others-continue stash=<sha> mirror=<path>`. The dispatcher takes
that as exit `6` for the one unit: no session is opened in the tree it just refused -- the journal
carries `TICK-SKIPPED` and no `TICK-START` for it -- the unit keeps its row in the queue, every other
unit is ticked by the same pass, and the pass exits `0`. The tick's `6` never leaves the pass, so
monitoring gains no code: `4` is still a failed tick and `8` is still a standing halt.

Why that shape specifically lost the halt: by the time the streak fires, the bytes are in a stash
**and** in a patch under `FACTORY_AT_RISK`, so the halt was no longer protecting anything -- it was
only refusing to guess again about work it had already copied in two places. And the streak was the one
halt whose cause was false in the same breath as the file: the stop had just stashed and mirrored the
bytes and cleaned the tree, so the marker went down over work it had itself made safe -- which is why
#405's re-probe can only mop this up one activation later, having stopped every other unit for the
interval and only twice per unit before `HALT-STUBBORN` leaves the resume to a person. Measured cost of
the old scope on #302: 14h34m and 77 activations of `HALTED` with 34 units idle, behind one unit's 2
files, until a person ran `rm`.

What replaces the halt's *visibility* is a notice, because an unticked unit is otherwise invisible in
the one place anyone looks: `IDLE` never appears while the rest of the queue is healthy, and `QUEUED`
merely stops shrinking. So the skip also posts on the unit's own issue --
`UNIT-QUARANTINE-NOTICED <unit> #<n>` in the journal, and a comment naming the attempt count, the stash
sha and the mirror path, because demand 3's whole claim is that nothing was lost and a notice that
cannot say where the bytes are is not that claim. It is throttled by `FACTORY_HALT_NOTICE_AFTER`, the
halt notice's own window, and the ledger stamp `quarantine-notice` is written only when `gh` exited 0: a
unit that keeps stopping short reaches a second accident again every time it has been ticked twice since
the last clean read, so a notice fired on each one would mail whoever owns it on the queue's own cadence,
and a stamp written on a failed post would silently retire the only sentence telling anyone the unit is
not being worked. A failed post journals
`UNIT-QUARANTINE-NOTICE-FAILED` with `gh`'s exit and its own words, and costs the unit nothing -- the
`UNIT-QUARANTINED` line is the record either way. Numbered units only, same as the halt notice:
`unclaimed:341` has an issue to comment on and `selftest` does not.

Nothing here is resumed by hand, because there is no marker to remove: `factory/factory.sh --status`
prints the unit's `ledger:` line with `quarantine-count=2` and `quarantined-at=…`, the journal's
`UNIT-QUARANTINED` line names the stash and the patch, and the unit is still a waiting row of the
queue -- the activation after a skip ticks it again, which is the difference between losing a turn and
being parked. What a person acts on is the work in `FACTORY_AT_RISK`. The halt path above is for the
case where there is no copy to read, and if the queue-wide stop is what you want anyway, the knob is
one `Environment=` line in the timer's unit file.

Nothing halted and the queue still is not moving: read the `code:` line of
`factory/factory.sh --status` first. A supervisor 121 commits behind the trunk prints the same
`halt: no` as one with nothing to do, and the journal's `SUPERVISOR-STALE` line is the only place that
says otherwise (#249, and the section above). Since #342 the `verdict:` line above it can also answer
`STALE` -- the installed tree behind and unrepaired for `FACTORY_STALE_ESCALATE` activations running,
with the last refusal's condition named in the sentence -- so the outage no longer needs a person to
grep the journal to call it a defect.

### A standing halt is re-asked every activation (#341)

A halt the harness wrote is now a question, not a verdict. The marker used to be a fact about the
instant it was written: every later activation printed its sentence and never asked the tree again,
and the streak halt is written one second AFTER the quarantine has already stashed and mirrored the
work it names -- so the named condition was false from the marker's birth and the queue froze anyway.
Measured on #302: 14h34m, 77 activations, 34 idle units, until a person ran `rm`.

Now the activation guard calls `factory_halt_reprobe` before it refuses. The marker written by
`factory_tree_halt` carries a machine line-tail -- `:: halt-target: unit=…|cause=…|mirror=…|tree=…`
-- and the re-probe asks that tree the content question (`factory_tree_dirt`) plus the two #323
state questions (`MERGE_HEAD`, unmerged index entries). The outcomes, each one a journal line:

- **`HALT-RELEASED <unit> cause-cleared tree-clean`** -- the named tree is provably clean and not
  half-merged: the marker is removed, the streak counter it came from is cleared, and this pass
  proceeds to triage. `HALTED` is not printed.
- **`HALTED …, re-probed this pass without release: …`** -- still dirty, half-merged, or git would
  not answer: the halt stands, and the sentence says this pass asked.
- **`HALT-STUBBORN <unit> releases=N`** -- the cause is cleared but the unit's release budget
  (`FACTORY_HALT_RELEASES`, above) is spent: the halt stands and the journal names both files a
  human resume touches (the marker and the counter). This is the trade this section sells: a unit
  that keeps stopping short of delivery can consume two more activations before a person is
  required -- two ticks against the 15h57m #248 counted read by nobody.
- **`HALTED …, and not re-probed since: …`** -- a hand-written marker. It names no target, there is
  nothing to ask, and nothing but the person who wrote it removes it. That rule predates #341 and
  survives it.

A dry run never re-probes: a release is a write, and dry runs write nothing to the real queue's halt
switch. The `HALT-NOTICED` issue comment stays the halt-write moment's, throttled once per hour; a
notice posted over a mirrored halt now names the stash and the mirror path and says plainly that
removing the marker loses nothing, because for that shape removing the marker loses nothing.

**Which markers exist to be re-asked, after demand 3.** Nothing in the four outcomes above changed, and
one knob-width changed what writes a marker at all -- worth naming here, because the fixture this
section's legs walk to is the streak halt, which the shipped default no longer produces; those legs set
`FACTORY_STREAK_HALT=1` to reach it, and the self-test pins both scopes. A halt is written now when the
work is **nowhere else**: a rescue that was refused (`TREE-QUARANTINE-REFUSED`, over the verdict's own
`cause=dirty` or `cause=unreadable`), from either call site -- before a tick, or before a conflict
resolution. A halt standing over a tree with a merge in it never releases on the dirt question, because
the re-probe asks git's index state as well and a half-merged tree is not clean however empty
`git status` reads. And a second
preserved accident writes one only under the knob. Under the shipped default that shape journals
`UNIT-QUARANTINED`, spends the unit its turn, tells its owner once, and lets the pass finish; the
section above, "Stopping it", carries the scope and the reason for it.

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

**A pull request is not the only thing a unit can be owed, and reading for only one filed a delivery as
a miss (#362, `H-0240`).** The gate a unit runs under ends its last turn by closing its issue when the
ticket is already satisfied by `main` (`.opencode/skills/factory/SKILL.md`, "When the ticket is already
satisfied by `main`"), and `factory/triage.sh` counts an issue claimed only by an **open** pull request:
that close claims nothing, so the read above answers empty and the one shape of done the read could not
tell from a miss was filed as a miss. Measured on `unclaimed:315` -- `TICK-UNDELIVERED … it stays where it
was` at `2026-11-14T01:21:41Z`, in the same minute that `gh issue view 315 --json state,closedAt` answered
`CLOSED` at `01:15:25Z` and `gh pr list --head issue-315 --state all` answered only the MERGED #316, the
historical delivery two days old. So an empty pull-request read asks the ticket as well. `CLOSED` journals
`TICK-CLOSED <unit> exit=0 issue #N reads CLOSED (<closedAt>)` and takes the shared clean-stop tail:
`TICK-DONE` journalled, `last-tick` stamped, `stuck-since` cleared, exit `0` -- the one write it skips
is the bundle. There was no pull request to owe; the line carries `` `gh issue reopen N` `` because a
verdict naming its own undo is one command from correct and the day's date is none to reconstruct.
`OPEN` keeps the old sentence unchanged. A ticket that could not be read, and a state word that is neither
-- `gh issue view` on a pull-request number answers `MERGED` -- journals `TICK-UNCHECKED` naming the read
that failed and renders no verdict, per the doctrine of that name: a read that failed is not a delivery
that failed. The read is gh's own projection, which answers the state UPPERCASE; REST, which
`factory_close_read` parses, answers lowercase, and that is the live defect #424 counts, so nothing here
borrows that helper. `stateReason` is not askable at the gh on the box the timer runs on: this box's
`/usr/bin/gh` 2.46.0 answers `Unknown JSON field: "stateReason"`; a 2.67.0 review box answers it
`COMPLETED`, so the claim is of its box and the field list holds on both. The mutating leg runs a
stubbed `gh`, pinning the call site and not the installed binary. Asking costs one view per stop,
which is not a rate worth protecting when the alternative is a six-minute-old close journalled as a
wasted 45-minute budget.

## Exit codes, because a timer has to tell these apart

`0` the pass is over · `3` the queue could not be read, so nothing was attempted · `4` a tick failed
· `7` another supervisor holds the lock · `8` halted · `9` the installed tree was fast-forwarded to
the trunk, so this pass stopped before dispatching and the next activation runs the merged code (#342).

`3` and `7` are the two that matter to monitoring: `3` means the token or the network is gone, and
`7` means two timers are pointing at the same repository. Since #380 the refusal also says so in the
journal at the moment it decides: `LOCK-SKIPPED holder=<pid>:<comm>[ +N] lock=<path>`, where `holder=` is
the oldest opener of the lock file as `fuser` names it, `+N` counting any descendants sharing the
descriptor, or `unknown` where fuser cannot answer -- and `unknown` still journals the line, because
an arm whose probe dies would recreate the silence the line ends. Before it, the six refusals
journald had shown since 1 Nov left ZERO rows in the retained `supervisor.log`: the one refusal its
retention still covers (14 Nov 14:39:41Z, first row 10 Nov) has no line beside it in the eleven
minutes around it, while the sentence naming the condition lived only in journald's stderr capture
-- all six occurrences findable there and none in the file -- and it named no holder: the diagnostic above could not be acted on from the file anyone actually reads (#380, H-0288). The holder
comes from `fuser`, not `/proc/locks`: measured 18 Nov, a live holder at pid `4104085` was row-named
`4104087`, which named no process, because `flock -n 9` makes the syscall in a child that execs and
dies; and a refused shell holds the file open itself, so both the arm and the probe close the
descriptor before asking, or the pass reports itself as the holder. `8` and `9` are the two the unit
files must
agree with: both are listed in the generated service's `SuccessExitStatus=`, **separated by spaces**,
because both are decisions the pass made on purpose. The separator is part of the mechanism, not
formatting: `systemd.service(5)` parses that value as a space-separated list, and on a comma systemd
does not skip one code, it ignores the whole assignment -- measured on the two renderings of the
generator's own line (systemd 259.9): `SuccessExitStatus=8,9` prints `Failed to parse value,
ignoring: 8,9` and lists neither code, at exit 0; `SuccessExitStatus=8 9` prints nothing. A
deliberate stop that journals as
`Failed with result 'exit-code'` points every reader away from the line that explains it (#246,
measured on code `4` hiding `Agent not found` for 152 activations; #305 added `8` to the line; #342
added `9` in the same commit that made the code reachable). `7` deliberately stays OUTSIDE the list,
and was weighed against adding it on #380: `8` and `9` stop a pass that had decided something, while
`7` did nothing at all, and a two-timer collision is the one condition on this page that a unit which
reports failure is the correct instrument for -- a green collision is a second timer nobody deletes.
The silence that made `7` feel like an unlisted failure is repaired where it lived, in the journal
(#380, H-0288), not by making the collision read as success.

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

A `TRUNK-RED` line grants no unit new authority. A unit reading a red trunk on its way past a journal
tail still may not halt the loop, refuse a merge, or hold a delivered pull request on the strength of
that line: the trunk read is report-only, and #370 item 3 — whether a red trunk should refuse a `MERGE`
— is an open question about the merge gate, a class a tick may not decide from a journal line. Making
that question a rule is a `DECISIONS.md` entry with a human behind it, not an arm of `factory_body`.

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

## A citation in a decision log carries its own anchor

`DECISIONS.md` and `factory/DECISIONS.md` hold backticked `path:line` citations, and the closing line
printed by `python3 scripts/check-decisions.py` is the only figure to trust — the debt line reads how
many it read and how many owe an anchor, and when the logs hold any `Effect:` diff tally a third line
reports how many, with none re-measured and the reason on the same line (`H-0307`, #253); `--citations`
names the exempt classes. Never copy its readings into prose as though they were still its readings. A coordinate is how
an entry says *what it measured*, so the log is re-runnable: `python3 -c "..."` at
`triage.sh:122`, the ratchet at `scripts/check-decisions.py:580`. A coordinate is also the one part
of an entry a
merge can invalidate without touching the entry. Merging `origin/main` into `issue-310` added lines to
files the other side cited, and two of `H-0139`'s pointers moved out from under their own sentences. The
judgement in that entry was right; only the pointer rotted, and nothing in `make verify` could see it.

So the citation carries its own evidence. A citation written with a quoted fragment is checked against
the tree being gated:

```
`AGENTS.md:52` "naming what is owed, and stop"
```

The fragment must be at least 16 characters, inside double quotes, with nothing between it and the
closing backtick but spaces and at most one apostrophe. It has to be found on the cited line, or
within the cited range
(`factory/common.sh:224-235`): no window of tolerance, because a pointer wrong by three lines still
lands on the wrong sentence, and a check that passes at ±5 is a check that reports drift it cannot name.
A quotation further away than that is not an anchor — and since #367 the refusal says which floor it
applied instead of claiming there was no quotation. **The quotation has to start on the coordinate's own
physical line**, and since #369 both newline-cut shapes say so in words naming the newline: these logs
wrap at ~110 columns, so a faithful fragment most often lands past one — either it opens on the NEXT
physical line, or it opens after the closing backtick and the newline ends it unclosed. The silent
version was measured and paid for: hit twice writing H-0171 (the gate said `the count went UP`, with no
error to read), and when #369's refusal caught the first two live rows, one of them had also drifted in
silence, from 1143 to 1746, while unchecked. A cut row refuses and is not debt, so the ratchet stays shut
while one exists; a quotation further down still lands in the debt (widening the class was measured and
refused: it reclassifies 3 of another unit's true pointers for a 0.6% dent, and buys a rule that accepts
the neighbouring sentence's quotation), and a wrapped tail inside an open code span is shell, not a cut
quotation — `--citations` names the cut rows in a section of its own.

Shapes that are understood and are not the same claim:

| written | meaning |
|---|---|
| `factory/common.sh:251` | a path, looked up exactly |
| `./DECISIONS.md:1847` | the root's copy — `./` is root-relative and is never matched by basename |
| `schedule.go:171` | a basename, allowed only while exactly one file answers to it |
| `gonee.md:3` | a bare name carrying an extension is a path, so a file that is gone is refused, not skipped — the extension is what tells it from the row below |
| `` `README.md:588` and the table at `:661-662` `` | a bare `:NNN` inherits the file named earlier **in its own sentence** — the sentence, not the physical line, since #365 measured that a wrapped paragraph here is several sentences naming several files; a sentence that names no file leaves the row in the open shape below; and a coordinate whose quoted anchor the newline cut — on the next line or dangling open — is refused by name, never left as silent debt (#369) |
| `` `AGENTS.md:46` now reads "…" ``, ``the rule is at `factory/factory.sh:245`` | asserts where the lines are **in the tree being gated** — refused outright without an anchor since #374; past-tensed with the date it was read it becomes an ordinary debt row |
| `origin/main:factory/selftest.sh:4120` | names the revision it was read at, so it owes no anchor and is not checked here |
| `pr:209`, `unclaimed:263`, `STATE_PROPS:1040` | a triage `class:PR` pair and a symbol: not citations, because the left half carries no extension, so nothing in the tree could ever answer to it |

**It is judged against the tree, never against the revision that wrote it.** That is the load-bearing
choice, and #318's own option 2 had it the other way. `AGENTS.md:38` was *correct* at `5be86e5`, the
revision `H-0139` measured, and wrong at `e41ce80`, the merge. Resolving at the cited revision therefore
passes the exact class of failure the issue was filed about, and refuses only entries that were false the
day they were written. It also needs git: `gate_reads_no_refs` in `factory/selftest.sh` refuses this gate
any git subcommand but `cat-file`, because a verdict that changes between a branch head, a merge preview
and CI reads as flakiness and gets re-run until it agrees. The gate reads the files in front of it and
answers the same question in every checkout.

**A sentence that says a line IS there today is itself a pointer, and it rots the same way.** "`AGENTS.md:46`
now reads …" and "the rule sits at `:437`" make the promise the anchor rule exists to keep — to *this* tree —
and the next merge into the cited file falsifies them while every check stays green. That is not hypothetical:
`H-0139`'s corrective bullet, the sentence telling a reader where to look, was measured by #374 to be false in
2 of its 4 coordinates six days after it was written, with `make verify` green over it. Since #374 the
classifier flags the adjacent verb forms — `now at`, `is at`, `sits at`, `lives at`, `points at`, `now reads`,
`moved to`, `still holds` at the coordinate's backtick, forward or reversed, including a verb wrapped onto the
next physical line — and refuses the unanchored rows outright; a past tense is never refused, because "was at
`:713`" and "read on 11 Nov" name the tree they measured and join the counted debt like any other unanchored
coordinate. Adjacency is load-bearing and was measured: the first implementation matched the verbs anywhere on
the row's line and pulled 12 true sentences into the refusal bucket — "`docs/factory.md` carrying … now
reading a census", and the revision-qualified "read `X:507` at `d7a5149`" family, where the preposition
attaches the read to a named head, not to the gated tree — so the matcher reads the text touching the opening
backtick and the self-test pins both controls must-not-be-refused. The writer's repair is one of two: anchor
the sentence, or past-tense it with the date it was read.

**What cannot be checked is counted, and the count may only fall.** Most of those citations predate this
rule and carry no anchor (some arrived as pointers that used to be skipped entirely — see the paragraph
on a name that carries an extension); the count lives in `scripts/check-decisions.py` as `POINTER_DEBT`,
and the only reading of it is the last line of `--citations`, `debt this gate ratchets on:` — a figure
copied into prose rots there (#375 caught this page holding two copies of it at once). Above it the run
is refused, and so is a run that measures *below* it — an unlowered number is a ratchet with the pawl
filed off, and the next unanchored citation hides in the slack. **Neither comparison runs while a single
pointer cannot be opened** (H-0170), because that number is not always a measurement of the logs: one
untracked `vendor/AGENTS.md` moves a dozen bare pointers from `unanchored` into the cannot-open list, and
the gate then refuses five other units' entries *and* orders `POINTER_DEBT` lowered — the one repair that
permanently disarms the ratchet, issued to a reader under a green-gate deadline. `tree_index` only ever
gains candidates, so stray content can only ever push the number down; the guard closes that direction and
the must-be-zero refusal keeps its own teeth. When it fires, the headline and `--citations` both say
`NOT judged this run` rather than `which may only fall`, so the pause cannot be read as a pass. A
merge from `main` raises the count with nothing written by you — measured on this branch's own
merge, 11 citations arrived in other units' entries
— so a merge re-measures with `--citations` and raises the constant in the same commit, which is the only
direction the ratchet permits a rise and the diff is the audit. A citation absent from the tree, or
ambiguous across two files, is refused outright rather than grandfathered: 54 were made resolvable by
writing the path, 17 of them delivered by that same merge, and folding that class into the baseline would
have hidden 54 dead pointers inside 517 with the headline green. A citation that points past the end of a
file is *not* refused on its own, and that is a choice, not an oversight: the unanchored pointers
cannot be length-checked without refusing sentences that are true on their own terms, so the length check
holds only where an anchor already proves what the line should say — measured at the head that ships the
sentence rule, 9 citations sit past the end of the file they name, all 9 unanchored: five `README.md`
rows an entry *reports* as read against another revision, one `docs/factory.md:5985` history row
`H-0159` names as history, and three rows `H-0121` writes as current — its `factory/selftest.sh` trio
whose enumeration sentence aims `:5875`, `:6521` and `:6638` at `factory/install-timer.sh` (617 lines)
because the clause names selftest.sh bare and the store inherits the last coordinate-named file. The
boundary narrows the mis-aim; it does not eliminate it. The line rule also saw 9 — four mis-aimed
continuations in `H-0110` and `H-0111`, which fell to the open shape, and the same five `README.md` rows.

**The files that teach this rule are checked by it, and that is a separate pass.** The rule reads the two
decision logs, so a false coordinate in the instruction that tells a unit what to write was invisible by
construction: the worked example sat in `.opencode/skills/factory/SKILL.md` six lines away from the sentence
it quoted, and the gate could not see it. `teaches_problems` now runs the same classifier over
`TEACHES_RULE` — that skill and `AGENTS.md` — and refuses a pointer there that is ambiguous, unopenable, or
anchored to text that has moved. It does NOT ratchet there: an entry in a log makes a promise forever, so an
unanchored coordinate is counted against `POINTER_DEBT`, while a guide is edited as a unit and its numbers
are re-read by this command, so an unanchored one is not a defect. `docs/factory.md` is deliberately outside
the pass — it is the reference, and the table that explains the shapes carries `gonee.md:3`, `pr:209` and a
`README.md:588` past a 349-line EOF on purpose, so checking it would refuse the guide's own examples.

**A name with an extension is a path, even bare, even when the file is gone.** `gonee.md:3` promises a
reader a line, and the promise is false whether the entry meant the root or a directory three levels
down — so it is refused, anchored or not. What stays exempt is a left half with no extension: `pr:209`
(a triage `class:PR` pair), `STATE_PROPS:1040` (a symbol), `D-0033:236` (an entry id), `verify:1` (a
verb). The extension is the only marker that survives both `triage.sh` and `internal/web/handlers.go`,
and it is a weaker test than the one that would be free — `git ls-files` answers "was this name ever
tracked?" in one read — but this rule reads no git, deliberately, so the weaker test is the price of a
verdict that is the same in CI, in a merge preview and at a branch head. The cost lands on prose that
*names a shape*: an illustrative `x.md:9` is refused too and has to be spelled without the colon. The
limit that remains, stated because it is the one a reader will hit: a bare `:NNN` whose sentence names no
file at all is not refused, because 175 rows of these logs are that shape inside wrapped paragraphs and
refusing them refuses 175 true sentences (occurrences, measured at `e66dc85d`: 312 bare
rows, 137 inheriting a file from their own sentence; 294 bare, 137 inheriting at `adddacc`, where #365
measured the move). The open shape is sentences naming the file as a bare backticked path and sentences
naming no file at all; the inheritance itself is still `H-0153`'s.

**The floor.** `POINTER_DEBT` is a number about a corpus, so it is a statement only about a tree whose
logs are that corpus. `POINTER_DEBT_AT` records how many entries the two logs held when it was measured,
and below that size the number is reported as not judged rather than applied. Measured, not designed:
`factory/selftest.sh`'s `gate_verdict_off_refs` leg copies this script into a synthetic two-entry
repository to prove the verdict is a reading of files and not of the ref store, and the first full run
after this rule landed answered that leg with *"the decision logs now hold 0 citations that owe an anchor,
below the recorded debt of 378: pay it down"* — a constant from this repository's logs refusing a
repository that has no such logs, in a checkout where the rule under test was someone else's. What that
costs: a run whose logs hold fewer entries escapes the ratchet, so deleting entries from a log is the one
way to move the floor. It is visible in a diff, the logs are append-only, and the refusal that is about a
pointer rather than a count — an anchored citation whose text is not on the line — has no floor at all.

Three findings came from the tool being wrong rather than the log, and all three are the hazard:

- The first census matched a **fully-qualified** citation by basename and took the first alphabetical
  candidate. `factory/DECISIONS.md:3178`, cited at `39dd88e54:factory/DECISIONS.md:3497` with its path
  spelled out in full, was measured against `DECISIONS.md` — 2621 lines at that revision, so it looked
  past EOF — while the file it names held 3531 and the coordinate lands on exactly the tally sentence the
  entry quotes. So the log's claim that these coordinates "all resolve" was true, and the audit's
  "citation false the day it was written" finding was the fabrication — produced inside the session sent
  to fix false findings. Resolution is now exact-path first and basename second, and refused when two
  files answer. (`README.md:588` is the token this log argues about most, and it is a different case: the
  root `README.md` has never exceeded 349 lines in its 12 revisions, so on the root reading it really is
  past EOF, which is what `H-0152` records. `H-0159` reads the same token both ways — see the Effect of
  `H-0162`, where the repair is disclosed rather than smoothed over.)
- The rule refused the disambiguation its own message recommended. `./DECISIONS.md:1847` was stripped to
  `DECISIONS.md`, matched by basename, and answered "two files". Advice that does not work is worse than
  none: the reader follows it and cannot tell what broke. `./` is now looked up as a path and nothing else,
  pinned by a self-test case.
- A bare `:NNN` used to be aimed by the last file resolved *on the same physical line*, and an entry in
  these logs is markdown wrapped at ~110 columns, so a physical line is often a whole paragraph naming
  several files. Four continuations were aimed by the wrong file: `H-0110`'s "`emit()` is defined TWICE in
  `factory/selftest.sh`, at `:2766` … and `:3731`" sat on a line whose earlier citation is
  `scripts/check-decisions.py:161`, and `H-0111`'s shim plants at `:2853` and `:3833` sat on a line whose
  earlier citation is `factory/triage.sh:600`. All four landed in the debt, where they cost nothing today
  — and all four were why the length check is anchor-only: applied to every citation the rule would have
  refused sentences that are each true on their own terms to catch zero false ones. The repair was a rule
  about sentences, not lines, and #365 measured and landed it: the boundary is now a sentence-final `.`
  or a block marker (`SENTENCE_BREAK`), which at the sync-merge tree `adddacc` aims 52 wrapped
  continuations the line rule left invisible and drops 10 aimed rows — the four above among them — and
  re-aims nothing: every aim the move keeps is the aim the line rule already gave it, including
  `H-0111`'s `:336` on its skill-file row. The refusal of the bare-path antecedent rests on the row it
  mis-aims (`D-0038`'s `:760` lands on a screenshot); the other `:336` row is the opposite edge.

Re-derive any of it:

```
python3 scripts/check-decisions.py                      # the counts, in the refusal-proof shape
python3 scripts/check-decisions.py --citations          # what the debt is made of, by target file
python3 scripts/check-decisions.py --citations AGENTS.md # every pointer aimed at one file: ask BEFORE editing it
```

## The tree claim: who holds this tree RIGHT NOW

`factory_worktree_busy` answers a question about the past -- "did a dispatch claim
this tree and never release it" -- and it can only answer it from the journal's
`TICK-START`. #421 measured what that cannot see: on 16 Nov a second unit was
dispatched into `wt/issue-341` 2m16s before a session started a merge in the same
resolution, 8m05s of two writers on one index, while the guard was asked at
15:31:50 about a dispatch whose `TICK-START` was being written at 15:31:50, by
that same dispatch. A journal cannot witness an activation's intentions before
its own first line, and a session merging outside the harness never writes one at
all. The instant question needs a different artifact: the **tree claim**.

A claim is a file under `$FACTORY_RUN/claim/` (`factory_claim_dir`) minted after
the content verdict and **before the first write** -- before `TICK-START` on the
tick path, before `git merge` on the inline conflict path -- carrying five fields
(`unit=`, `tree=`, `dispatched=`, `pid=`, `deadline=`). The deadline is the
minting activation's own bound on its own life, `FACTORY_TICK_TIMEOUT +
FACTORY_CLAIM_SLACK` from the mint, which is strictly less than the shipped
`RuntimeMaxSec`; past that moment nobody can still be working under the file, and
that proof is what licences a later reader to sweep it. `factory_claim_held TREE`
answers the probe in four polarities -- `0` held (stdout names the unit, its pid
and its deadline), `1` free, `2` unknown, `3` expired-and-swept -- and identity is
judged before matching: a member whose fields cannot be parsed, a directory where
a `.claim` file was, or a claim directory replaced by a file all answer `unknown`,
because it could BE this tree's claim and a guard whose read failed must not
answer free (the same polarity #232 earned for the busy guard, and for the same
root-walks-past-chmod reason).

`factory_tree_claim_check UNIT TREE` is the verb every arrival asks -- both
supervisor dispatch paths, and a person's or interactive session's own hands:
the Worktrees rule in `AGENTS.md` tells any writer heading into a harness tree
to check the claim, mint a `human:<name>` claim of its own before the first
write, and release it when done, so the 16 Nov writer and a harness tick refuse
each other through the same file. (A factory unit session mints nothing: its
tick minted the tree's claim before dispatching it, and there is deliberately
no self-exclusion -- the guard refuses whoever asks while a live claim stands.)
A refusal is journalled in words naming the holder
(`TREE-HELD <unit> <tree> held-by=<unit> <pid> <deadline> ...`) and costs nothing:
`NOT-STARTED` on the tick path (budget unchanged, the claim itself makes a later
pass the retry) and `CONFLICT-DEFERRED` on the conflict path (no model run is
queued for a merge that never happened). A release names its owner: the right
unit and a live pid remove the file; a release naming another writer leaves it in
place and says `TREE-CLAIM-FOREIGN` naming both, because that file is someone
else's working claim and deleting it re-opens the collision the file exists to
prevent. The release is the activation's last touch of the tree -- after the
teardown quarantine, after inventory -- inside the death traps as well as the
clean path.

The claim refuses only writers that ask. That is its whole reach and it is stated
here so nobody oversells it: a raw `git` in an unclaimed cwd is stopped by nothing
new, and a pass SIGKILLed between mint and release leaves a file that holds its
tree until the deadline passes -- at most `FACTORY_TICK_TIMEOUT +
FACTORY_CLAIM_SLACK`, never forever, which is the trade the deadline exists to
make (`H-0340`). The journal still carries the after-the-fact half: `factory_worktree_busy`
removes nothing while a tick's record is open; the claim removes the *race* the
record cannot see early enough.

Each shipped choice has a leg that mutates it, in `factory/selftest.sh`'s claim
family: standing the tick's refusal down reproduces the 16 Nov collision on demand
(`TREE-HELD` journalled and the dispatch runs anyway); muting the release leaves
the file outliving the pass; deleting the deadline test makes a dead claim read
held; answering `free` for the unknown lets the same garbage through; and the
conflict box only conflicts under the mutant, which is what proves the shipped leg
stopped a writer rather than a box that merges nothing either way. The mutant
conflict box shows the claim does not stop at one site: the `conflict:900` tick
the mutant queues meets the SAME claim at its own activation and is journalled
`TREE-HELD` and `NOT-STARTED` with no session run — a mutation of the inline site
still cannot reach a model run, because the dispatch guard holds the same tree
independently.
## The journal's retention contract, written down before anything bounds it

`factory/log/supervisor.log` is the harness's audit trail: which decision id was handed to which unit
and branch, which tree a tick was holding, where the log directory went when a self-test moved it.
Before this section the only prose about it was `docs/factory.md:1238` "the journal lives inside the
tree (`factory/log/supervisor.log`)" -- and #334 cites that sentence at `docs/factory.md:883`, which
the log's own growth had already slid 355 lines, which is why the sentence is quoted here beside its
number (`H-0139`'s rule). Nothing bounds the journal today -- no code rotates, truncates or prunes --
and bounding it is escalated, not owed by a tick (`H-0188`: deleting the operator's records is one of
the four classes a human keeps). What was owed is the sentence #334 wrote as "an accident of the
current readers, not a guarantee anybody wrote down". It is a guarantee now. Anyone who lands a bound
-- a size cap with `head -c`, a `tail -n`, a rename-rotation in the shape `internal/audit/audit.go`
already ships for the product, a keep-N prune -- must satisfy it.

Six readers were inventoried at one head. Four tolerate bounded history; two do not, and both of the
two are guards, so the failure mode is silent: the harness keeps running and merely stops refusing to
do something dangerous.

| reader | what it needs from `supervisor.log` | survives a bound? |
| --- | --- | --- |
| `factory_file_age` | the live file's mtime | yes -- a fresh file answers fresh |
| `factory_journal_age` | the **newest** line matching a key | yes -- it takes the last match |
| `factory_log_move_lines` | a tail window (`FACTORY_LOG_TAIL=400`) | yes -- it is already bounded |
| `factory_log_footprint` | no content at all, one `find` and stat (`H-0187`) | yes |
| `factory_worktree_busy` | the `TICK-START` line of a tick **still running** | **no** |
| `factory_decision_provenance` | **every** `DECISION-ID` line for a branch | **no** |

Both failures are pinned by legs in `factory/selftest.sh` that name the shape that breaks them. Run the
suite as `env -u FACTORY_TICK_UNIT bash factory/selftest.sh` -- the scrubbing #425 owes, and without it
two unrelated legs read red for a reason that is not yours.

**A running tick's `TICK-START` is the oldest line of its own tick, and the guard that reads it has no
code call site -- its caller is the rule `AGENTS.md` gives a session before it removes a tree.** Over
the 171 released ticks in the live journal at 2026-11-17 (199 `TICK-START`, 137 `TICK-DONE`, 34
`TICK-FAILED`; 28 of the 199 starts have no release line at that date, and the census pairs each
release with its own `TICK-START`, counting the lines strictly between) the start-to-release
distance is 1 line at the median, 16 at p90, 109 at the maximum: the supervisor is single-threaded
and writes almost nothing while the agent runs, so the journal is nearly frozen during
a tick. That is why a *window* is not this guard's hazard -- 400 lines is above the maximum observed
distance. The hazard is any bound that **moves the whole file away**, which is what a rename-rotation
does and what `internal/audit/audit.go`'s five-generation scheme does -- and copying that scheme here
verbatim is precisely the mistake, because `rotate()` moves the live file and opens a new one
(`internal/audit/audit.go:601`, called at `:236`) while the product has no running-tick claim to strand.
The harness does. Measured, four polarities: journal holding the line answers `0` busy; rename the
journal and keep the generation answers `1` free; rename it and prune the generation answers `1` free;
and the refusing arm, `2` unknown, comes from the guard's shape judgement -- not a regular file, or not
readable (the family pins a directory there; `chmod 000` measured `2` here too). The first three are
this section's leg; the fourth is pinned by the guard's own family (#361's control C, "an unreadable
journal must read 2, not 1"). **No rotation shape
produces `2`.** Every one produces `1` -- the answer that authorises the removal -- so a bound of that
shape restores the incident `H-0203` records rather than preventing it (`wt/issue-315` was removed
2026-11-14T01:14:40Z and its tick did not write its release line until 01:21:18Z): a session asks the
journal, the journal has moved away with the running tick's only line in it, the session removes the
tree. A reader aimed at `supervisor.log*` recovers the claim when a generation was kept and still reads
free with none kept, so keeping `.1` is necessary and not sufficient.

**The earned set is a whole-file read, and a branch outlives a pass.** A `DECISION-ID` line written when
a unit earns an id is the whole proof, for that branch, that a later heading was not taken by eye;
`factory_decision_provenance` reads every one of them, and ids are earned on a branch's first pass and
read on its last. The live journal needs no synthetic plant to show what a window costs: it holds 36
`DECISION-ID` lines over 7 branches, and **25 of the 36 sit outside the 400-line window the harness
already ships for a different question** (`FACTORY_LOG_TAIL`, #260's `LOG-AT`) -- 6 of the 7 branches
have at least one earned id above it, the furthest 1,252 lines from the end. Narrow the earned-set scan
to that window and honest branches start being reported `DECISION-ID-UNEARNED`, which is the report
that tells a human an id was fabricated. The reason to reach for the window is the reason #334 files:
a whole-file walk is assumed to get linearly more expensive. It is not the problem -- 50 whole-file
`grep -F` scans of the live journal (1,907,580 B) cost 34 ms, and of the same bytes replayed 15x
(28,613,700 B) 40 ms, warm -- and the control says the timer is not reading zero: `sha256sum` of that
28.6 MB costs 16 ms.

So the contract for a bound, in the order someone will implement it:

1. **Rotate at a tick boundary, after the release line, never mid-tick** -- or make
   `factory_worktree_busy` read the whole generation set (`supervisor.log*`) and treat the record as
   absent only when no generation holds it. Do not count on the `unknown` arm as a safety net: it
   answers for an *unreadable* journal, and a rotated journal is empty or missing, which answers free.
2. **Never narrow `factory_decision_provenance`'s earned-set scan to a window.** If the walk becomes
   expensive, bound it by *age* over the named generation set -- the requirement is that every
   `DECISION-ID` line for a branch stays reachable, not that one pass reads one file.
3. **Keep the newest bytes; a bound that keeps the head is worse than no bound.** `head -c` on a size
   cap keeps the *oldest* N bytes: it deletes every current reader's answer, including the live tick's
   `TICK-START` and the `LOG-AT` line `#260` needs to say where the record went.
4. **Keep a rotation's own trail legible.** #334's constraint -- a rotation that leaves an empty live
   file must still answer fresh to `factory_file_age`, and one that moves the file must not strand the
   `LOG-AT`/`LOG-MOVED` pair -- plus the `--status` line `H-0187` shipped, which prints
   `journal <bytes>; <N> files, <bytes> under <dir>; oldest <file> <age>`. A bound that prunes should
   make its own answer visible there, because a silently-shrinking audit trail is the thing this section
   exists to prevent.
 5. **Name the clock, and the zone, or refuse the answer.** "Keep N days of transcripts" is not yet a
   predicate, and three of the six spellings an implementer reaches for are wrong in three different ways.
   One read of one directory -- 227 transcripts, 58,792,819 B, the cut taken once into one variable, at
   2026-11-17T19:37:42Z against cut 2026-11-10T19:37:42Z -- and "keep 7 days" removes **11** files
   (2,766,311 B) on the seconds in the filename stamp, **11** on `mtime` read in UTC, **19** on the
   `YYYYMMDD` prefix against the cut day with `<=`, **0** with `<`, **0** on `find -mtime +7`, and **19**
   on `find ! -newermt` with the cut stamped bare. The three faults, in the order they bite. `+7` is not a
   seven-day predicate at all: GNU truncates an age to whole 24-hour periods, so it asks for *eight* days,
   and it answered empty at the same instant the seconds predicate answered 11 while `-mtime +6` named
   the `mtime` predicate's exact set, the two lists diffed and asserted non-empty first. The bare `! -newermt` stamp is parsed as **local time** -- this box is `-0500`,
   `/etc/localtime` being `America/Toronto` -- and answered 19, where the identical command under `TZ=UTC`
   and the same stamp written `2026-11-10T19:37:42+00:00` answered 11: eight files for three characters of
   suffix. And the day comparison is not a loose reading of the policy, it is another policy -- `<=` takes
   the whole 10 Nov cohort, `<` refuses every file in it, and both read as "seven days". Nothing in the
   tick's own environment settles the zone: the installed unit carries five `Environment=` lines and no
   `TZ=` anywhere in the file, so a pass inherits whatever zone the activation happens to arrive with.
   Therefore the answer this ticket asks a person for must name the number **and** the clock **and** the
   zone, and a bound must refuse an answer that names only the number rather than defaulting the rest --
   the tie-break this harness already applies to a missing input ("the option that refuses rather than
   proceeding when input is missing or empty"). A defaulted clock is a retention decision taken in a code
   review nobody convened, and `H-0188` is the record of why a tick does not get to take it.

Clause 5's re-measure, because those numbers are of one minute rather than of a head. From the repository
root: one epoch, six predicates, nothing written but `stat`:

```
cd factory/log && pat='2*T*Z.*.log' && t=$(( $(date -u +%s) - 7*86400 ))
cut=$(date -u -d "@$t" +%Y%m%dT%H%M%SZ); cutm=$(date -u -d "@$t" +%Y-%m-%dT%H:%M:%S)
printf 'total transcripts  %s\n' "$(find . -maxdepth 1 -name "$pat" -type f | wc -l)"
printf 'A filename-seconds %s\n' "$(find . -maxdepth 1 -name "$pat" -printf '%f\n' | awk -v c="$cut" 'substr($0,1,15)<c' | wc -l)"
printf 'B mtime-seconds    %s\n' "$(TZ=UTC find . -maxdepth 1 -name "$pat" -type f ! -newermt "$cutm" | wc -l)"
printf 'C day-string <=    %s\n' "$(find . -maxdepth 1 -name "$pat" -printf '%f\n' | awk -v c="${cut:0:8}" 'substr($0,1,8)<=c' | wc -l)"
printf 'D day-string <     %s\n' "$(find . -maxdepth 1 -name "$pat" -printf '%f\n' | awk -v c="${cut:0:8}" 'substr($0,1,8)< c' | wc -l)"
printf 'E find -mtime +7   %s\n' "$(find . -maxdepth 1 -name "$pat" -type f -mtime +7 | wc -l)"
printf 'F -newermt, no TZ  %s\n' "$(find . -maxdepth 1 -name "$pat" -type f ! -newermt "$cutm" | wc -l)"
```

The `total` line is the block's precondition, printed because the block's worst answer is a quiet one: six
zeroes beside `total 227` is the finding, and six zeroes beside `total 0` means the glob or the working
directory is wrong -- a failed `cd` on the first line takes the whole `&&` chain with it and leaves every
predicate counting an empty directory. That is the ordinary outcome of running the block anywhere but
here: `factory/log` is not tracked, `git ls-files factory/log` answering nothing even in the checkout
that holds it, so a fresh clone or a sibling worktree prints six zeroes for a directory it has never
written.

A and B are one predicate under two clocks, and they agree only while no transcript was written long
after it began. They agreed at the read above, 11 and 11; 86 seconds later the same block answered
**A 12, B 11**. What separated them is `20261110T193817Z.conflict:269.log`, named 19:38:17Z and last
written 20:05:48Z -- 27 minutes of appending after its own stamp -- so every cut inside that 27-minute
window calls the file old by name and young by `mtime`, and a bound that runs nightly crosses into such a
window at the same clock time each day. Which clock the answer names decides whether that file is
deleted. The directory is not a fixture while this is read either: it gained 86,213 B of transcripts in
the four minutes between the two reads the clause quotes, so a count copied out of this page is a
timestamp, and `total` is the line that says whether the six numbers below it mean anything.

Re-measure the inventory rather than trusting this page; it is one grep over the shipped files, and it
was true at one head:

```
grep -n 'supervisor\.log' factory/common.sh factory/factory.sh | grep -v ':[0-9]*:[[:space:]]*#'
```

Every number above is from the live journal on 2026-11-17: `journal 1907580 B; 239 files, 51969388 B
under factory/log; oldest 20261110T134828Z.unclaimed:213.log 6d13h` -- the footprint line the shipped
`factory_log_footprint` prints, which is also #334's premise measured forward: the issue opened at
"20 MB across 100 files" in three days and this is seven.

That block is now a tool, and the tool is gated: `factory/retention-census.sh`, whose
`--self-test` runs on every `make verify` as the `==> journal retention census` step. `--help` is
the whole surface -- `--dir DIR`, `--days N`, `--bytes N`, `--now STAMP`, `--self-test` -- and
what it prints is the six rows above: one cut taken once into one variable, the directory's own
total on the line above them, and the `refuse:` footer that says the run priced a number and chose
no clock. It is read-only by construction -- it prices an eviction and never performs one -- and
it refuses a `--days` with a `--bytes`, because the two shapes a bound takes are not one question.
Three things it does that the block above cannot.

It prices the day arithmetic honestly. `find -mtime +N` reads the real clock and accepts no
reference time, so a census pinned with `--now` cannot ask `find` what `find` would have said
seven days ago: it would answer for today and be stamped with the pinned instant. So row `E` is
GNU's own arithmetic, `int(age/86400) > N`, evaluated at the census clock, and the census prints
the `pinned:` line saying which of its six rows is a computation and which five are calls. The
self-test's live leg builds the one fixture this is checkable against -- ages measured from the
instant the leg runs, because `-mtime` has no other clock to read -- and asserts `E` equals `find
-mtime +7`'s own count *and* that `-mtime +7` and the seven-day predicate differ by at least one
file. The emulation is checked against the tool it emulates, not against this page's memory of it.

It prices a cap, which is the other shape. `--bytes N` sorts the transcripts oldest first and
evicts until the directory fits, once under each clock, and prints how many files the two clocks
keep differently: the count that says whether the clock is worth arguing about before anyone
writes a rotator. `--days` and `--bytes` are not interchangeable and neither one is a budget --
seven days is a different number of bytes every day it runs, and a byte cap is a different number
of days every day it runs -- so the census prints the directory's own measured rate and the bound
that number implies.

It refuses in the two states where a census means nothing. A directory holding no transcript that
matches the transcript name exits **3**, naming the glob and the directory: `0 B` there is a
statement about the pattern, not about the journal. And a census whose every reading is zero says
so in a `quiet:` line, because *the block's worst answer is a quiet one* is clause 5's sentence
and a zero is the answer a wrong glob and a genuinely young directory share -- `factory/log` is
untracked, so the ordinary place a unit runs this is a directory it has never written.

What a person still owes is unchanged: the number, the clock, the zone, and which generations
survive, and clause 5 still refuses an answer that names only the number. What moved is where the
cost of an answer lives -- half a second, one command, nothing written. Two runs over the live
journal three seconds apart priced the two shapes a maintainer has to choose between. At
2026-11-18T00:03:39Z, over 240 transcripts and 61,129,420 B, `--days 7` answered A 20 / B 19 / C
66 / D 19 / E 0 / F 27 files: the two second-resolution clocks disagreeing by one file *at the
same read*, and the day policy and the unqualified zone turning seven days into 66 files and 27.
At 00:03:41Z the same tool with `--bytes 20000000`, the figure the issue itself opens with, keeps
70 of the 240 and evicts 170 of them -- 41,151,232 B of the audit trail, a kept set the size of
2.4 days at the directory's own measured rate. Both are readings of one minute, and the tool
re-derives both in the time it takes to ask.

## A tick's session gets no MCP servers, because a grandchild nobody asked for is torn down loudly

The dispatch is `opencode --standalone`, chosen so that the 2700 s ceiling can actually reach the
server. A private instance reads the user's own configuration, and `opencode` starts every MCP
server that configuration declares as a child of the instance that read it -- so on a machine whose
`~/.config/opencode/opencode.json` names MCP servers, every activation puts those servers, and
everything they in turn start, inside the unit's cgroup. Read from inside an activation on 19 Nov,
by a session that asked the kernel and not a tool: `/proc/self/cgroup` named
`/user.slice/user-1000.slice/user@1000.service/app.slice/parental-lockout-factory.service`, and that
cgroup's `cgroup.procs` held twenty processes, three of them the three configured servers. One is
`StudioMCP.exe`, a Windows binary under Vinegar's Wine.

Nothing in the harness asks for an MCP server, and a ticket about Go and shell has no use for one.
What it gets instead is teardown, and systemd keeps that bill. `journalctl --user -u
parental-lockout-factory.service` read per day on 19 Nov: 17 Nov carries 24 lines of `State
'stop-sigterm' timed out. Killing.` and 24 kills, every one of them `winedevice.exe`; 18 Nov carries
6 and 6, again every one `winedevice.exe`; 19 Nov carries none up to the read at 20:18Z, and still
one activation that day held the teardown 44 s with nothing in the journal to explain it
(`REAP-HELD hold=44s how=unexplained`, 17:11:09Z -- the `--status` sentence for that is H-0123). The
kills say a Wine child does not answer `SIGTERM`; the unexplained 44 s says the hour is paid even
when nothing is killed. The day buckets above are this box's own, rendered at `-05:00`; the report in
the issue renders the same journal at `+05:30` and so counts a different eleven hours as "today",
which is why only the total -- 30 timeouts, every one of them `winedevice.exe` -- is a number two
readings of this machine agree on.

So the dispatch hands the session one policy on `OPENCODE_CONFIG_CONTENT`:
`{"policies":[{"action":"mcp.use","resource":"*","effect":"deny"}]}`, in the shell as
`FACTORY_MCP_FENCE` (`factory/common.sh`). The documented semantics are that a denied MCP server is
removed however it was configured, which is the whole fence. It travels as an environment layer so
that it belongs to the dispatch and not to a file: the timer, `factory.sh --once` and a hand-run
pass all fence the same way, and the configuration a person uses interactively is left alone.
`factory/selftest.sh` pins the value, the hand-over and the layer: that the shipped string parses as
one deny of every MCP server and that the parser refuses the near misses; that a real tick receives
it verbatim and sees nothing at all once the dispatch line is replaced by an inert one; and that a
private `opencode` instance reads the policy out of that layer where the same instance, handed no
policy, reads the fixture's declared server and no `mcp.use` statement. The two CLI legs SKIP where
`opencode` is not on PATH, and neither one spends a token.

What the fence has not yet been shown to do is stop a process, only stop a configuration: the
instrument for that is the unit's cgroup, and only an activation stands in one. The read is
`awk -F: '$1==0{print $3}' /proc/self/cgroup`, then that path's `cgroup.procs` under
`/sys/fs/cgroup`, from inside the first tick after the merge -- no MCP server should be in the list,
and the entry that decided this is `H-0312`.

What the fence is not: a sandbox, and not a statement about any other kind of child. A grandchild a
tick *does* start -- a compiler, a test runner -- is still in the unit's cgroup and still systemd's
to kill at the end of the activation; #390's other half, confining what lands there, is a
tamper-resistance question and has never been decided here.

## The `.cortexkit/` marker is tool state, deliberately untracked (#416, H-0325)

The agent tooling that manages this machine's context keeps a managed block at
`.cortexkit/.gitignore` in every tree it works in, and it writes and deletes that file outside git,
on its own schedule. The file was committed on 2026-10-13 in `55a34f8`, which turned the tool's
ordinary deletions into dirt: the primary checkout sat ` D .cortexkit/.gitignore` across 101
activations (#346), and once the main-checkout read began restoring committed-and-missing files
(#408), a tracked tool-owned file became a permanent restore subject -- the pass would write the
marker back every activation while the tooling deleted it again, journalled and endless. `H-0325`
untracked it (`git rm --cached`; the bytes stay on disk). This section is the record the tracked
marker used to be, written down because nothing checks a paragraph: the whole `.cortexkit/`
directory, including this marker in both of its states, present and absent, is ignored by the root
`.gitignore`, so nothing under it can show in a status read and the main-checkout arm has no
permanent subject in this checkout. The untracking travels with the merge: a clone whose copy is
clean at sync time has the working-tree file deleted by the sync itself (the tooling rewrites it
when it wants it, ignored either way), and a clone whose copy is modified at sync time meets one
modify/delete refusal, which the quarantine arm is built to absorb.
