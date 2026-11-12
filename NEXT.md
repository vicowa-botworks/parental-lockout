# Next work for the factory on this branch

- **`#296` is claimed by the unit that ran today and its work is in flight; the queue should not
  dispatch it again.** Items 1-3 of what it owed are on `main` from #299 (`14f1826`), and item 4 -- the
  correction to the two merged `Effect:` fields that defer the POST drivers "because their `--out` is a
  single named file", which is not a flag either driver has (`cdp.capture_out` declares
  `metavar="DIR"`, `testing/capture/cdp.py:2046-2049`; the named file is built inside the directory at
  `lock.py:240`) -- is pull request **#336** at `f35d150` (checkpoint `709ff26`, `Decision: D-0068`,
  `DECISIONS.md` only, `make verify` exit 0 `verify: OK`). Read back from the API when this line was
  written: `OPEN`, `headRefOid f35d150…`, `mergeable MERGEABLE`, `mergeStateStatus BLOCKED`, no
  approving review; #336's body carries the link keyword, so the ticket goes when that merges, and if it
  is refused #296 stays open with item 4 owed and nothing else on it. The line #299's tick left in this
  file -- "`#314`/`#296`'s second half is an open claim, not delivered work" -- is no longer here:
  `ccf852c` dropped it while resolving `NEXT.md` against main, so the measurement it asked for is now on
  the ticket, six mutations in a copy of the tree: hollowing `cdp.keep_refused` reddens **15 of 78**
  rules, deadening either driver's disposal statement reddens **3 of 78**, moving the keep after the
  browser closes reddens **3 of 78**, and reverting both drivers to `discard_stage()` reddens **3 of
  78**. Its other half is still true and is not this ticket's: no body-gutting row in
  `scripts/attack-capture-self-test.py` names `single_shot` or
  `a_single_frame_keep_cannot_be_read_as_evidence`, so gutting either rule body keeps the count at 78
  and prints green -- #322's class, with #326 naming both drivers as unaudited surfaces and #324 owing
  the read of the kept frame's bytes.
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
  - Re-read 12 Nov at the merge carrying `H-0144`, and the sentence above is the one that moved:
    `main`'s tip is now `131b180` (the merge of #316) and `gh run list --branch main` answers `success`
    for its push run `43152169732`, completed 04:02:45Z. So the trunk is green and `5be86e5` was the tip
    when the red was written. What #319 is still owed is the half a green run cannot answer -- why a
    merge result reached `main` with no run over it before it landed -- and the ticket stays open on that
    argument, not on a red tick. Quote the run id either way; this line has now been true twice.
- The line-number hazard this branch's own merge created is filed as #318 with its census (272
  citations across the two logs, the two this merge moved, one that has pointed past end-of-file for
  months). Fixing the class changes what a governing document requires of an entry, so it is a unit's
  work with a chosen option, not something to slip in beside a conflict resolution.
  - Grown by this branch's second merge on 12 Nov, by the same read: the six entries that merge joins
    carry 36 `path:NNN` coordinates between them, 23 still hold on the merged tree and 13 do not. Eleven
    of the thirteen are inside `H-0139`'s dated bullet and six of those quote the literal `needs-human`
    that `H-0140` retired out of five files, so there is no successor line to re-point them at -- which
    is the case a checker has to survive and a rewrite cannot. The full table is in `H-0144` and in the
    two dated bullets it appended, entry by entry, with the cause named per entry.
