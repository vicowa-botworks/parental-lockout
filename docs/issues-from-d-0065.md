# Issues owed by `D-0065` — exact `gh issue create` invocations

`D-0065` made one refusal dispose of itself once, in `cdp.SingleRefusal`, and deliberately stopped
there: the guard's job is the second disposal, and there is a second defect on the same path that it
cannot reach. One issue, reproduced on this branch on 11 Nov by driving the two single-frame drivers'
real `main()` in a copy of the tree with the browser faked and the operator's stderr refusing one write
(never the gated tree — #1807).

Nothing here is filed by this branch: filing is one command, run from the repository root.

---

## 1. A report that dies in the *fixture* arm escapes as a traceback, so the run's exit code is the laundered one and the operator reads nothing

```
gh issue create --label testing,ux --title \
  'lock.py/testlock.py: a refusal whose report raises in the fixture arm ends in a traceback, not a refusal' \
  --body-file /tmp/issue-report-dies-fixture.md
```

**Body.** `lock.py` and `testlock.py` catch their refusal twice. The inner arms run inside
`with cdp.Capture(...)`; the outer arm catches a fixture that refused before a browser existed.
`D-0065` added `cdp.SingleRefusal`, so a report that raises inside the first disposal can no longer be
re-refused by the outer arm — but only when there *is* an outer arm still to catch it. In the fixture
arm the disposal is the outer arm, so a `print` that dies has nothing left below it and escapes
`main()` as a traceback. Measured 11 Nov on the branch's own tree, both arms, two death points each, one
injection per run:

| arm | what dies | `main()` returned | blocks in `REFUSED.txt` | what the operator read |
|---|---|---|---|---|
| page | stderr dies on `REFUSED:` | `1` | none | the guard's own sentence |
| page | stderr dies after the note | `1` | 1 | the guard's own sentence |
| fixture | stderr dies on `REFUSED:` | `raised OSError: [Errno 9] Bad file descriptor` | none | nothing at all |
| fixture | stderr dies after the note | `raised OSError: [Errno 9] Bad file descriptor` | 1 | up to the stale-file line |

`--out` was byte-identical afterwards in all four cells, so nothing is destroyed; the loss is the
transcript and the type of the exit. `lock.py`'s own comment on the outer arm says it plainly — "a
traceback is not an answer; a refusal is" — and an escaping `OSError` is precisely the case that comment
was written for but does not cover. It also launders the exit code: the process still exits 1, because
Python exits 1 on an uncaught exception, so a caller cannot tell "the page refused" from "the reporter
died" by status, and `pages.py`'s rule for the same class (#149) calls that laundering out by name — an
exit code is not evidence a driver finished deciding.

What closes it is not `SingleRefusal` — the flag has nothing to suppress when there is one handler. It
is deciding the package-wide rule the docstring already claims: `report_refusal` and `capture_refused`
"report and never raise", which `capture_refused` honours (`except (Exception, SystemExit)` → prints
`no frame of the refused page: …` → returns `None`) and the bare `print`s in `refuse_single_shot` do
not. Either every operator-facing `print` in the disposal path goes through one helper that swallows a
dead stream and says so on whatever is left, or the drivers wrap the disposal itself and turn a dying
report into `return 1` plus the one sentence that survives. Both keep the note the run already wrote;
neither is smaller than a decision, which is why `D-0065`'s `What lost:` forfeits it rather than
shipping a fifth variant of the same `try`.

Not accepted: catching `BaseException` around `main()` and printing a traceback more politely. The
ticket is that a diagnostic died and the run said nothing about it; a nicer traceback still says
nothing about it.
