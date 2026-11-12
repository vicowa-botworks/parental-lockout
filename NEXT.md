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
