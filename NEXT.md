# NEXT — issue-417: the conflict fetch defers with git's cause in the journal, behind the fetch bound (branch `issue-417`; the 20 Nov adoption pass ships it — merging belongs to the supervisor, and the one close-verb on this work lives in the pull body)

State this pass read off the live tree before writing a byte (2026-11-20): trunk still `0f343db`
(#481 merged 06:27Z), `gh pr list --head issue-417` answered `[]`, and every gate re-ran green on
the shipped bytes — `919 passed, 0 failed` on `env -u OPENCODE_CONFIG_CONTENT
bash factory/selftest.sh`, plus check-shell, check-denies, check-next-md and check-decisions.

## What this branch ships

- The conflict arm's fetch of `origin/$base` in `factory_resolve_conflict` no longer returns 2
  behind a silent `FAILED (128)`: it captures stderr to the swept `factory_fetch_stream()`
  (factory/common.sh), quotes git's FIRST stderr line into its own `CONFLICT-BLOCKED` sentence
  under the harness's `git said:` marker — the last line of a transport burst is the access-rights
  decoy — and it runs behind `FACTORY_FETCH_TIMEOUT` like the refresh arm's activation fetch, so
  a parked connection now answers `killed at the …s fetch bound` instead of costing 135 s a unit.
  `run_action` is off this call on purpose: it normalizes every failure to 1, and this arm must
  tell 124 from 128.
- `H-0327` in factory/DECISIONS.md carries the four options, the three losses and the measured
  receipts (the 16 Nov SSH burst; the empty-stderr 124 measured over a parking stub, where git
  retries ssh once, so one burst costs 2x the park); checkpoint a92a8bc.
- The selftest grew the transport family at its tail (a fast ssh failure, a parked ssh killed at
  the bound, and blinded controls for the clause, the bound and the sweep term), plus two harness
  knobs: `proc_extra_env` reaches the pass BEFORE the shipped env — a leg moving a name the
  shipped list re-spells must use a `proc_*` knob (`proc_fetch_timeout` is the new one), never
  the array — and journal needles that are substrings of `REFRESH-FAILED` rows must keep their
  leading space.
- The 219 `path:line` pointers this round's inserts slid were re-aimed by the checker's own
  census; `python3 scripts/check-decisions.py` reads rc 0 with the debt unmoved at 583.
- #417's third proposal is NOT here: port 443 of `ssh.github.com` is a machine `~/.ssh/config`
  edit, named in H-0327's `Revisit:` for whoever owns that file.

## What is still owed

- The supervisor merges; this pass merged nothing, and no artifact but the pull body carries a
  closing verb. The reviewer ask is the supervisor's poke, per AGENTS.md.
- `make verify` inside a live tick fails one fence leg (`and nothing else does`, filed as #482):
  the tick's own OPENCODE_CONFIG_CONTENT leaks through `proc_run` into the control; until that
  lands, re-measure with `env -u OPENCODE_CONFIG_CONTENT bash factory/selftest.sh` (H-0327).
- After this merges, a later branch re-derives the pointer census with
  `python3 scripts/check-decisions.py --citations` (#389); if the SSH burst recurs, `git said:`
  quotes git's answer and `killed at the …s fetch bound` names the parked connection.

## Carried forward — verbatim, because the decision log is anchored into these two lines

The close-verb lesson of the inherited ledger stays on the two lines the twelve
factory/DECISIONS.md rows anchor into this file; the next rewrite of this file re-aims those
twelve rows exactly as this round did:

- The census row of the `issue-364` hand-off, carried because the rule outlives its round —
  quoted from it must name its clock. And the row about #425 says "Do not close #425 on this branch's
- The #425 instance, carried with it: neither proposed cause reproduced on the measuring tree,
  so the question stays open even though that unit measured a real
  RED on its own shell. Do not close #425 on this branch's measurement alone.
