# Decision log

Every judgement call made unattended in this repository, with the options it was given and the
commit to roll back to. Written by the `factory` agent (`.opencode/skills/factory/SKILL.md`).

Rolling one back:

```bash
git log --grep 'Decision: D-0016' --format='%H %s'   # the effect commit
git revert <that sha>                                 # a forward fix; no history rewrite
```

`Checkpoint:` is the commit taken *before* the decision, never after, and that order is
load-bearing: a checkpoint taken afterwards records the state the decision produced, which is the
thing you already have.

**The ids are shared across the repository, not per branch.** `git log --grep 'Decision: D-0016'`
has to name exactly one commit, and a branch-local numbering makes "roll back to before D-0016"
ambiguous the moment two branches are open. So this log starts where the branches already open in
this repository stopped counting, and nothing here renumbers anything:

- D-0001–D-0010 are on `feat/factory-harness` (PR #217), the branch that installs the log's gate;
- D-0011–D-0014 are on `issue-202` (PR #218), the sibling of the navigation work;
- D-0015 onward are here, on `issue-204`.

When these files meet, every half is kept. A conflict in this file is a good outcome: it means two
units both recorded something rather than one of them quietly not writing it down.

## D-0015 · The navigation work stands on the approved capture branch, not on `main`

- Question: #204 changes `page.tmpl` and `app.css`, which are declared sources of all nine
  artifacts in `docs/screenshots/360/`, so `manifest.py check` -- a step of `make verify` -- refuses
  the tree as soon as either file moves. Which branch does the unit start from, given that the
  three candidates differ in whether a re-photograph of those nine frames would be evidence anybody
  can reproduce?
- Options: start from main | start from the approved capture branch
  (fix/verdict-clock-keeps-its-zone, #209) | start from the audit-label branch stacked on it
  (fix/audit-row-label, #214) | start from the manifest-conditions branch stacked on that
  (fix/manifest-records-conditions, #215)
- Chose: start from the approved capture branch (fix/verdict-clock-keeps-its-zone, #209)
- Why: the re-record tax is the same size on every option -- nine frames -- so the tie-break that
  separates them is what the paid tax buys. Measured on this machine at this tip, before any edit:
  all four drivers re-run from their own fixtures and **nine of nine committed PNGs come back
  byte-identical** (`md5sum` of `testing/capture/runs/*/*.png` against `docs/screenshots/360/`,
  including `lock.png` and `testlock.png`). On `main` the repository's own record says the
  opposite -- `testing/capture/README.md` keeps a table whose `dashboard.png` and `audit.png` rows
  read **no, in another state**, on a page that is a branch of the clock and a trail whose height
  was a run counter (#207 causes 1 and 2) -- and that record is the reason #209's branch exists.
  So `main` would have me commit two frames I cannot attribute to a declared state, which is the
  state of affairs #207 spent seven causes escaping; the two deeper branches (#214, #215) only add
  an audit-page fix and a `capture-manifest/2` schema that re-records the same nine frames again
  with their conditions, neither of which the navigation depends on. The fourth option was also the
  cheapest to hold: #209 carries `main` in full (`git merge-base --is-ancestor origin/main
  origin/fix/verdict-clock-keeps-its-zone` is true), so getting onto it was a fast-forward with no
  merge commit of its own.
- What this entry is *not* evidence for: the four drivers were run at the chosen tip and nowhere
  else. What `main` does to the nine frames is quoted from this repository's own record, not
  measured here, and this entry says so rather than inheriting the comparison as a measurement.
- What it costs: this branch carries #209's five commits until #209 merges, so the pull request
  reads as a stack and its diff is against `fix/verdict-clock-keeps-its-zone` rather than `main`;
  and when #215 lands, its re-record of the same nine frames has to be re-done on top of this
  change, because the shell moves underneath it.
- Checkpoint: 410f1320744aae382ccd5b50d96fc5f255370995
- A note on the checkpoint's shape: no `--allow-empty` marker commit was taken for this one,
  because the state before the decision is a named remote ref that already exists and is already
  reachable -- `git rebase --onto main dce832d4427dc48b8161860339f6ace166aa65a8 issue-204` is the
  whole rollback -- and an empty commit under a fast-forward is the first thing a rebase drops.
  Every entry below this one has an ordinary marker commit.
- Effect: the branch's own base (no content commit; `issue-204` moved from `410f132` to
  `dce832d4427dc48b8161860339f6ace166aa65a8` by fast-forward), and this file

## D-0053 · The preserved `.Page` design is delivered as the record of a drop, not as work to merge

- Question: this branch is the only ref that carries a checkpoint and an allocated id for the
  `.Page` shell-comparison design of #204, and it has no pull request anywhere; the content commit
  `7bface3` is also on `origin/wip/issue-204-page-comparison-preserved`, whose draft dock (#230) is
  closed. Deliver this as work, deliver it as a drop, or delete it?
- Options: rebase onto main and ship the `.Page` design | rebase onto main and port only
  `TestTheResultPagesMarkNothing` | deliver the branch unchanged as an open pull request that
  records the drop | delete the branch and leave no pull request
- Chose: deliver the branch unchanged as an open pull request that records the drop
- Why: the first two options are refused by measurement rather than by preference. Both designs
  state something about the same two paths and the statements contradict: `main`'s
  `internal/web/nav_test.go:197-198`, read at `origin/main`, wants `["/users"]` marked on
  `/users/casey/lock-now` and `/users/casey/test-lock`, while this branch's
  `internal/web/pages_test.go:381` refuses *any* `aria-current` in the render of those same two
  POSTs. No tree holds both verdicts, so the second option has nothing to port -- the property it
  would port is false on the design `main` chose. Choosing between the designs is not an open
  question either: #229 merged the path design at `2026-11-08T01:33:46Z` and its review and
  `testing/capture/cdp.py` both defend it, and reopening a settled design is one of the classes
  this skill reserves for a person. Independently of all that, the branch is red at its own tip:
  `make verify` at `badba83` stops at `==> screenshot manifest` with 9 of 9 frames refused, because
  `7bface3` moved the shared shell (`internal/web/templates/page.tmpl`) and no frame was re-taken --
  as its own message admits, "unrun-and-unlanded". Its log cannot meet `main`'s as written either:
  `DECISIONS.md:29` opens this log at `D-0015` while `main` carries a `D-0015` at its line 211, so
  `main`'s `scripts/check-decisions.py`, run against the two logs concatenated, adds three refusals
  naming that id -- "'revert to before D-0015' would name two commits" -- the failure this file's
  own preamble forbids. Between the last two options the ticket settles it: a branch dropped only
  in a local log is indistinguishable from one never attempted. The option taken also touches the
  fewest files (this one) and is undone by `git revert` of a single commit.
- What this entry is *not* evidence for: the four capture drivers were not re-run, so nothing here
  claims the nine frames would come back byte-identical once re-photographed -- only that none was
  taken after `7bface3`. The contradiction is proven by reading the two assertions at their file and
  line, not by running a merged suite, and the log collision is proven with `main`'s checker against
  a concatenation in `/tmp`, not against a test merge of the two trees.
- What it costs: `main` keeps nothing of this design. What survives is what is already on refs --
  `origin/wip/issue-204-page-comparison-preserved` at `7bface3` and the older
  `origin/wip/issue-204-nav-preserved` at `d076168` -- plus #230's closed thread. No bytes are lost
  by not merging; the pull request exists so the verdict is visible to the queue that counts
  pull requests and nothing else.
- Checkpoint: badba830938a45c7f935973c7fdf863f0610930a
- A note on this checkpoint's shape: it is the tip this tick started from, not a marker commit it
  took. `badba83` is itself the `pre-D-0053` marker, taken by the tick that allocated the id and was
  killed before writing the entry. `git rev-parse HEAD` at the start of this tick was that same sha
  with `git status --porcelain` empty, so nothing moved between the marker and the decision that
  precedes it, and a second identical marker would only bury the first.
- A note on the id: this base (merge-base `dce832d`) predates the harness and has no `factory/`
  directory, so `factory/next-decision-id.sh` is not here to run. The id came from `origin/main`'s
  copy of the allocator, extracted to `/tmp` and aimed back at this worktree with `--repo`, which
  printed `D-0053` -- agreeing with the marker above, and one above `main`'s highest entry, D-0052.
- Effect: this file, and the pull request for `issue-204`. No source, template or stylesheet is
  touched by this commit, and `7bface3` and `badba83` are left exactly as they are -- unrewritten --
  because both shas are cited by name in #204's thread and in #230.
