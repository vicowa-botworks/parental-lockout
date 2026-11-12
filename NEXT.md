# Next work for the factory on this branch

- Nothing queued by this unit. `#310` is answered by pull request #312, and the ask on #307 is
  already held by the queue's own poke, so the only action left on this branch is the supervisor's:
  read `reviewDecision`, mergeability and every check from the API and merge if they all still say
  yes. Nothing here merges it, and no reviewer request was filed from this branch.
- One finding is left for a separate unit and is named on both artifacts rather than widened into
  this one: the comment on the review denial in `.opencode/opencode.jsonc` cites `D-0013` for the
  keying of the deny table, where the entry that keys it under `bash` is `H-0013`
  (`factory/DECISIONS.md:274`), and `DECISIONS.md`'s own head records that `D-0013` was minted
  twice -- so that citation points at two candidates and neither is the one meant. A citation pass
  over the `D-`/`H-` references in that file is its own unit's work; this branch corrected only the
  citation it could prove wrong today (`D-0010` to `H-0010`, in `.opencode/skills/factory/SKILL.md`).
- **`main` is red, and it is not this branch's doing.** CI push run `43140612253` on `5be86e5` -- the
  merge of #314, still the tip -- failed its `verify` job at 02:32:40Z while `shellcheck`,
  `mutation sets` and both `packages` jobs passed, and #314's own branch run was green: the red is in
  the merge result, a tree no run checked before it landed. This branch's run `43147284060` on
  `e41ce80` is green on all five checks on top of that same base, and `make verify` here ended
  `verify: OK` twice, which leaves runner host state or something this diff carries. The failing
  leg's log is unreadable from a tick (`gh run view --log-failed` answers zero bytes, `gh api` is
  denied), so it is filed as #319 with the reproduction. Read that before merging anything onto
  `main`, #312 included -- every other queue row still reads green over a broken trunk.
- The line-number hazard this branch's own merge created is filed as #318 with its census (272
  citations across the two logs, the two this merge moved, one that has pointed past end-of-file for
  months). Fixing the class changes what a governing document requires of an entry, so it is a unit's
  work with a chosen option, not something to slip in beside a conflict resolution.
