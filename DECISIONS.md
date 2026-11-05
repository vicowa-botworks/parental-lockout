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
