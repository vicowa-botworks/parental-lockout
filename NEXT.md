# NEXT — issue-325 unit

State at push: branch `issue-325`, head `0bae2c4` (`Decision: D-0081`), PR body at
`/tmp/opencode/pr325-c-body.md`, gate read-back to be posted as a PR comment (#1180 — never
claimed before the run finishes). Merged with `origin/main` at `14209cb` in `73dbd91`.

## What this unit shipped

- `D-0079`: the attack suite's `verify:` clause is pinned in `WIRED` (merged earlier).
- `D-0080`: the canary count became the verdict's tail, `canary trip(s)` (merged earlier).
- `D-0081`: the tail counts the battery's PROBE-prefixed findings too, derived once at the
  battery call with arm 2's formula; the word is now `probe trip(s)`; `FAILED_SUMMARY` moved
  with it; two `DOCUMENTED_HOLES` texts disambiguated ("what the pass DID", not this count).

## What is still owed (all listed, none silent)

1. **The deleted-call hole** — `delete _rule_probes_are_live(registry, ran, bad) from
   self_test()` is still green (D-0076's class text). The promised closure is a tally of what
   the pass DID (probed, reported, arms, shapes) handed to the verdict as a third argument.
   D-0081 explicitly did NOT ship that and said so in both class texts. If a later tick ships
   it, `self_test_verdict`'s signature, arm 2's four shapes, the `shapes != 4` pin and the row
   anchored on `code = self_test_verdict(list(ran_shape), list(bad_shape), trips)` all move
   together — start from the class texts in `scripts/attack-capture-self-test.py`, not memory.
2. **The switched-off derivation** — deleting
   `trips += sum(1 for p in bad if p.startswith(f"{PROBE}:"))` loses the tail on battery-only
   trips (measured, D-0081 What lost). Pinning the tail mandatory would put the passing summary
   into a regex no row probes; that trade was made and recorded, don't relitigate without a
   new measurement.
3. **#325 itself stays open** — close it only when the issue's own test is met on the merged
   tree: neutering each named statement stops being silent or is a recorded floor. Slice three
   of the closure is this PR; check the issue body against `gh pr view` read-back before any
   close decision (and the no-`close #N` adjacency rule, #1256).

## Hard rules for whoever picks this up

- Merge origin/main FRESH before any verdict; `DECISIONS.md` collides by id-union, never
  rewrite a published entry; the attack suite may collide inside a class text — check whether
  main CLOSED a class (rows→CAUGHT) before unioning.
- Mutants run on `git archive` copies under `/tmp/opencode`, never the gated tree (#1807).
- `rm -rf` is refused by the harness here; use fresh unique directories.
- Checkpoint before the decision, one effect commit with `Decision:` trailer; re-run
  `factory/next-decision-id.sh DECISIONS.md` after a fresh fetch before taking an id.
