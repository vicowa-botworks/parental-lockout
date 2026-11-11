# Issues owed by `D-0058` — exact `gh issue create` invocations

`D-0058` found five things about `testing/capture/pages.py --self-test` while building
`scripts/attack-capture-self-test.py`, and deliberately closed none of them: #297 asked for a runner,
not for a changed verdict. Four are gaps in the guard that are each one small edit wide; the fifth is
the standing rule that a suite which runs no compiler belongs in `make verify`.

Each was reproduced on this branch on 11 Nov by running the attack against a copy of the tree (never
the gated tree — #1807) and reading the patched self-test's own transcript. The attack that
reproduces each one is named in the body, and every row is still in the suite, so closing an issue
turns its row red with `HOLE CLOSED <attack> — the guard now refuses this, so retire the row and, if
it was the last in its class, the class with it`. That is the acceptance test, and it is automatic.

Nothing here is filed by this branch: filing is one command each, run from the repository root.

---

## 1. A rule whose body was emptied is invisible to the count — 3 of 3 emptied rules came back green

```
gh issue create --label guard,testing --title \
  'pages.py --self-test: a rule whose body was emptied keeps the count at 76 and prints green' \
  --body-file /tmp/issue-hollow-rule.md
```

**Body.** `EXPECTED_RULES` pins how many rules ran and says nothing about what any of them checked.
`scripts/attack-capture-self-test.py` empties three refusal rules —
`a_refused_run_says_where_its_frames_are`, `each_fixture_keeps_its_refused_frames_apart`,
`a_kept_frame_is_moved_out_of_the_publish_queue` — by inserting `return` under each `def`, and all
three print `capture self-test: 76 refusal rules hold`. No rule's content is covered by a sibling of
another, so the count is the only thing watching, and the count cannot see inside a body. This is
#297's title read literally, and #125's eviction hazard from the end #125 does not reach: a counted
rule cannot police its own call site.

What closes it is the Go battery's design applied to Python: one probe per rule that breaks the thing
that rule names and insists THAT rule goes red — a per-rule probe, so a hollowed rule retires a row
instead of passing a gate. The suite already has the harness: `_run_main`, `_refusal_box` and `_raiser`
are enough to drive one refusal per rule from inside the tree.

Not accepted: adding more counted rules. That raises the number a hollowed rule has to match.

---

## 2. `rule()`'s crash arm is the only place a crashing rule becomes a problem, and nothing checks it

```
gh issue create --label guard,testing --title \
  'pages.py --self-test: a rule that throws is recorded as a passing rule when the crash arm is emptied' \
  --body-file /tmp/issue-crash-arm.md
```

**Body.** `rule()`'s `except Exception` arm appends `f"{why}: {type(exc).__name__}: {exc}"` and is the
only thing standing between a crashing rule and a green run. Emptying that one line — the line whose
own comment says a crashing rule "has not passed" — is invisible: the canary pair never throws, so it
passes whatever the arm does, and every other rule is still counted. The row is
`a rule that throws is silenced in the rule helper` in `scripts/attack-capture-self-test.py`, and it
prints `76 refusal rules hold`.

Why it matters more than it looks: the arm is dead code on a clean tree, so the row above proves
only half of it. The other half was measured by driving a real crash through both versions of the
arm — `cdp.refused_dir` patched to `raise RuntimeError('boom')`, which is a crash in the code *under*
the rules, not in a rule's test harness. With the arm intact the run reports
`capture self-test: 17 of 76 rules failed`, exit 1, and names each crashed rule
(`a refusal on every page still measures every page: RuntimeError: boom`, and sixteen more). With
the arm's one `problems.append` replaced by `pass`, the **same** crash returns exit 0 and
`capture self-test: 76 refusal rules hold`, and stderr contains the string `boom` zero times.
Seventeen broken rules become a clean bill of health by deleting the line whose own comment says "a
rule that throws has not passed". That is the moment the arm exists for — the code under a rule
breaking — and it is the moment the run would lie. `pages.py:1636-1640` records why the arm's
`{why}` prefix was fought for: when `keep_refused` was hollowed, four rules threw at once and finding
them meant bisecting, which is what the comment calls the reason the sentence names the rule.

What closes it is four lines: register a third canary like the existing pair, deliberately raising,
and assert its own `{why}: {type(exc).__name__}` sentence arrives in `probe_bad`. That is a
`pages.py` change and `D-0058` declined it on scope, not on merit.

---

## 3. A refusal's kept frame is never read — an empty PNG satisfies every sentence about it

```
gh issue create --label testing --title \
  'capture refusal contract: the kept frame is only ever listed, never read, so an empty PNG passes' \
  --body-file /tmp/issue-empty-frame.md
```

**Body.** `_FakeSession.screenshot()` hands `cdp.capture_refused` a payload; the rules then read the
kept directory back — names, counts, the note's sentences, the transcript's pointer at each path —
and nothing reads a single byte of what landed there. Replacing `fh.write(data)` with `fh.write(b"")`
(`cdp.py:190`, read back at `4537b9d`) keeps the transcript and the directory in perfect agreement
about a frame with no picture in it, and prints `capture self-test: 76 refusal rules hold`. The row is
`the kept frame is written as an empty file`.

This is the property `#211` and `D-0054` exist to hold, arriving from the side nobody attacked: a kept
frame is supposed to be the evidence-shaped thing a refusal says it is, and "there is a file at the
path the transcript names" is not a statement about the frame. `manifest.py` already decodes pixels to
refuse a published frame whose bytes are older than its page (`D-0056`) — it looks at pixels for the
published set and never once for the refused one.

What closes it is one clause in `a_refused_run_says_where_its_frames_are`: the kept file's bytes equal
the fake's payload, per viewport. Not accepted: comparing file size alone, which is the same
assertion with a zero-byte hole in it.

---

## 4. The verdict's last links are one edit above everything a rule compares

```
gh issue create --label guard,testing --title \
  'pages.py --self-test: if bad:, the reach comparison and `if counts != [1, 0]` police nothing that polices them' \
  --body-file /tmp/issue-verdict-floor.md
```

**Body.** Three statements decide the verdict and none of them is inside a rule:
`if bad:` (where a problem becomes an exit code), `if len(ran) != EXPECTED_RULES:` with its `return 1`
(the reach check), and `if counts != [1, 0] or len(probe_bad) != 1:` (the only sentence proving the
rule helper is not hollow). Switched off, each leaves the run green: the canaries are still registered
and their counts still computed, just no longer compared. Four rows in
`scripts/attack-capture-self-test.py` cover them (`the verdict's problem list is switched off`,
`the reach check is switched off`, `the reach check runs and returns success`,
`the canary comparison is switched off`) and all four print `76 refusal rules hold`.

This is a floor, not an oversight: a check cannot police its own deletion, and a rule that policed the
count check would have to hold the number that check exists to hold. It is listed because the shape
that beats it exists and is used elsewhere in this repository — `mutate.py` and `attack-self-test.py`
both keep their verdict one call away from a *separate* suite that attacks it, and
`manifest.py` cross-witnesses its two `make verify` steps so each one survives the other's deletion.
What closes it here is the same move: the two attack suites already run inside `make verify` beside
the thing they attack, so wiring `pages.py --self-test`'s own invocation can be pinned from
`attack-capture-self-test.py` — which is exactly what `wiring_problems()` now does for the *attack
suite's* two steps. Doing it for the verdict's three comparisons needs a decision about who owns the
number, and that is a `pages.py` decision rather than a runner decision.

Interim cover, already in place and worth naming so nobody re-does it: the four rows that empty
`rule()` itself are all caught, and they are caught *because* the canary comparison is live. The
comparison is unguarded; its absence is not unnoticed.

---

## 5. `manifest.py --self-test` has no attack suite, and `lock.py`/`testlock.py` have no self-test at all

```
gh issue create --label guard,testing --title \
  'the capture tree has three unaudited surfaces: manifest.py --self-test, lock.py, testlock.py' \
  --body-file /tmp/issue-capture-coverage.md
```

**Body.** #297's finding generalises past the file it was filed against. `testing/capture/manifest.py`
carries 44 self-test rules (`capture manifest self-test: 44 rules hold`, measured 11 Nov) and is in
`make verify`; nothing attacks it, so `D-0058`'s class-1 hole applies to all 44 with no runner in
existence. `testing/capture/lock.py` and `testing/capture/testlock.py` are the two drivers that
photograph the result pages, and `pages.py:17-19` is the record of what they carry: the result pages
"cannot be reached this way and have their own drivers (`lock.py`, `testlock.py`), which is where the
click-the-shipped-button refusals live." They have **no** `--self-test`, so no gate can see their
refusal contract at all; `D-0054` also left both of them disposing of their staged frame with
`discard_stage()` — `lock.py:251` and `testlock.py:225` — which is the behaviour #220 removed from
`pages.py`.

The runner to write first is a third file beside the two that exist, over `manifest.py --self-test`:
`scripts/attack-capture-self-test.py` is 41 rows and 3.4 s and its whole structure is reusable —
the box, the anchor-once assertion, the four verdicts, the hole list — because it was copied from
`attack-self-test.py` for the same reason. The drivers need the step before the attacks: a `--self-test`
that can be attacked has to exist first, and `D-0054`'s `What lost:` already names their half as owed.

Scope note so the two issues are not confused: this one is coverage of surfaces with no runner. #297
is the runner for the surface that already has one, and it is closed by
`scripts/attack-capture-self-test.py`.
