# Threat model

This is the document `docs/PLAN.md` §13 asks for: what `plock` defends against, what it does not,
and — for every claim that it *does* — the test that would fail if the defense were removed.

A claim with no test behind it is not a defense, it is a hope. Where a claim rests on an e2e step
rather than a unit test, that is said, and where nothing stands behind it, it is listed under
**Not defended** instead of being quietly omitted.

Run `make verify` to check everything here that is checked automatically.

---

## 1. The adversary

**A child with a normal desktop account on the machine.** Not an attacker with root, not a
network attacker, not a malicious package.

That boundary is worth stating precisely, because most of the design's shape comes from it and
from what it excludes. The child is:

- **Technically competent.** They will read the man page, open a terminal, and edit files. Assume
  they know what a symlink is, what `$PATH` is, and what `chmod` does.
- **Able to run arbitrary code as themselves.** No defense here assumes the child cannot execute a
  program. Anything they can do to a file they own, they will try.
- **Able to persist within their own account** across logout, reboot, and daemon restart.
- **Not able to become root.** If they can, they have already won and no user-space daemon can
  help. Every "the daemon runs as root so a child cannot disable it" claim (`docs/PLAN.md` §4) is
  a claim about *that* adversary.

### What the adversary controls

| They control | Why it matters here |
|---|---|
| Files and directories they own | symlinks, FIFOs, and planted binaries can appear on paths the daemon reads or execs |
| Their own session's logind properties | `SetLockedHint` is authorized by uid alone, so the lock state the daemon reads is **writable by the child** |
| Their own account's environment | `$PATH`, locale, and anything else inherited by a process they start |
| The process table | they can start and stop processes, including ones that *look* like the screensaver locker |
| The wall clock, within their session | not directly — the daemon reads the system clock — but timezone and DST behaviour are theirs to influence through config if config were writable, which it is not |

### What the adversary does not control

| They do not control | Basis |
|---|---|
| `/etc/plock/` and its contents | root-owned, `0600`; every path that reads or writes it refuses a symlink, a FIFO, a directory, and a loose mode (§2) |
| The daemon's lifetime | systemd-owned; a session user cannot stop a root unit |
| The audit trail | written by root into a private directory; a session user cannot read, replace, or redirect it (§3) |
| The admin credential | PBKDF2 verifier in the config file, which is §2 |

---

## 2. Trust boundary: the config file

`/etc/plock/config.json` is the entire persistent state of the daemon and the thing the child most
wants changed. It is root-owned mode `0600`, so the direct attack — edit it — is closed by the
file mode. Everything below is about the indirect attacks: making the daemon read, write, or trust
something other than what it thinks it is reading.

| Claim | Test |
|---|---|
| A symlink at the config path is refused on read | `TestLoadRefusesASymlinkedConfig`, `TestOpenNoFollowRefusesASymlink` |
| A symlink is refused on write, and by the open itself rather than a check that ran before it | `TestSaveRefusesSymlink`, `TestSaveRefusesSymlink` |
| A FIFO at the config path cannot park the loader | `TestLoadDoesNotBlockOnAFIFO`, `TestOpenNoFollowNeverBlocksOnAFIFO` |
| A FIFO swapped in *after* a pre-open check is still caught | `TestOpenReadableRefusesAFIFOSwappedInAfterTheCheck` |
| A non-regular file is refused on write | `TestSaveRefusesNonRegularFile` |
| A directory is refused | `TestLoadRefusesADirectory` |
| Mode is tightened on the descriptor we hold, not on the path — so a swap between open and chmod cannot redirect it | `TestTightenModeIgnoresAPathSwappedAfterTheOpen`, `TestLoadTightensALooseMode` |
| Saving creates the file `0600` regardless of umask | `TestSaveCreatesMode0600` |
| A failed or refused save leaves no orphan temp file | `TestFailedSaveLeavesNoTempFile`, `TestRefusedSaveCreatesNoTempFile`, `TestLoadReapsStaleTempFiles` |
| A document from a version this build cannot read is refused rather than half-parsed | `TestLoadRefusesUnknownVersion`, `TestSaveRefusesAVersionItCannotRead` |

**Why the FIFO cases are security tests and not robustness tests.** The config is loaded inside the
enforcement tick. An open that blocks does not crash anything and leaves nothing in the log — it
stops the re-locking. A child who can create a named pipe at a path the daemon opens has turned off
enforcement without touching the daemon. `O_NONBLOCK` plus a shape check on the descriptor is what
makes that loud instead of silent.

### Ambiguity resolves toward the lockout

A malformed or partial document must never be a reason to enforce less.

| Claim | Test |
|---|---|
| An unreadable document is an error, not an empty config | `TestLoadRejectsUnreadableDocuments` |
| A rule that cannot be fully read enforces the **widest** reading consistent with what could be read, and says so | `docs/PLAN.md` §6; schedule suite |
| A malformed rule whose start equals its end still locks out — the safe-superset policy outranks the zero-length-window rule | schedule suite |
| A missing `enforcement` key is not Go's `false` | config suite, via the `*bool` shadow pattern |
| A missing or broken admin credential is an error at load, not a login form that silently denies everyone | `TestAMissingCredentialIsAnError`, `TestABrokenCredentialIsAnErrorRatherThanADenial` |

---

## 3. Trust boundary: the audit trail

The trail is the operator's only evidence that anything happened. Two failure modes matter, and they
are opposites: it can be **silently discarded**, or it can be **redirected somewhere the child can
reach**.

| Claim | Test |
|---|---|
| The trail directory is created private, and stays private under a restrictive umask | `TestTheDirectoryIsCreatedPrivate`, `TestACreatedDirectoryIsPrivateEvenUnderARestrictiveUmask` |
| The file mode is tightened regardless of umask | `TestTheFileModeIsTightenedRegardlessOfUmask` |
| A widely readable trail is refused rather than quietly kept | `TestAWidelyReadableAuditFileIsRefused` |
| A trail owned by another account is refused | `TestAAuditFileOwnedByAnotherAccountIsRefusedAtAnyUmask` |
| A symlink at the audit path is refused, including one planted after the first write | `TestASymlinkAtTheAuditPathIsRefused`, `TestASymlinkAtTheAuditPathIsRefusedByTheOpenItself`, `TestAPathReplacedByASymlinkAfterTheFirstWriteIsNamedAsASymlink` |
| A FIFO at the audit path fails fast instead of parking the tick | `TestAFIFOAtTheAuditPathFailsFastInsteadOfParkingTheTick` |
| Rotation neither fails nor tears a line under concurrency | `TestRotationUnderConcurrencyNeverFailsOrTearsALine` |
| Rotation bounds are counted from bytes already on disk, not from process start | audit suite |

**Claim with a weaker basis than the rest.** `TestTheNullDeviceIsRefusedAsAnAuditPath` covers the
"not a regular file" check on the held descriptor, and it is the one security test in this document
that **skips in CI**: reaching `/dev/null`'s shape needs a root run, because `/dev` is root's and the
directory guard fires first. The realistic version of that attack is a FIFO, which *is* covered
unprivileged above. What is covered only under root is a character device at the audit path — which
the child cannot create without root anyway. Stated rather than glossed, because "every claim has a
test" is only honest if the one test that does not run in CI is named.

---

## 4. Trust boundary: the child's session

The daemon reaches into a session the child owns. That is the only place it deliberately crosses
into the adversary's territory, so it is the most constrained.

| Claim | Test |
|---|---|
| Commands run as the session user, with the real primary group — never uid-only with gid 0 | `TestDropPrivilegesSetsTheRealPrimaryGroup` |
| A failed privilege drop is surfaced, not swallowed | `TestExecRunnerSurfacesAFailedPrivilegeDrop` |
| An unknown or malformed account record is refused before any drop | `TestDropPrivilegesRefusesAnUnknownAccount`, `TestDropPrivilegesRefusesAMalformedRecord` |
| uid 0 is refused as a lock target, at both the shared check and the desktop backend | `TestCheckSessionTargetRefusesUID0`, `TestDesktopLockRefusesUID0` |
| The logind backend pins its argv and refuses what it must not lock | `TestLogindLockPinsArgvAndRefusesWhatItMustNotLock` |
| An implausible session id is refused rather than interpolated into a bus call | `TestLockConfirmedRefusesAnImplausibleSessionID`, `TestShowRefusesAnImplausibleID` |
| The child's session is found from the listing only; a property logind answers empty is not a fallback | `TestSessionsTakesTheUIDFromTheListingOnly`, `TestSessionsFallsBackToTheListingForNameAndUID` |

### The session table is attacker-adjacent input

`loginctl` output decides *who gets locked*. A parser that guesses can drop a session, and a dropped
session is a child who is not locked and nobody the wiser.

| Claim | Test |
|---|---|
| Columns are mapped by header name, never by position | `TestParseSessionTable*` |
| A row with fewer fields than the header is an error, not a skipped row | `TestParseSessionTable*` |
| A row with *more* fields is rejected unless the last column is the known multi-word one | `TestParseSessionTableRefusesSINCEBeforeTheLastColumn` |
| A non-numeric uid is rejected | `TestParseSessionTableRejectsANonNumericUID` |
| An empty table is zero sessions, not an error | `TestParseSessionTableEmptyTableIsZeroSessions` |
| A `class=manager` session with no seat is filtered out | platform suite, per the multi-session-per-uid constraint |

### The daemon's own session

A daemon that locks the session it is running in cannot be stopped: stopping it needs a keyboard,
and the keyboard is behind the lock it just put up. The override path does not rescue this, because
granting one needs the web UI or the CLI on the machine that is now locked.

So the enforcement loop declines to lock a session that lives in the daemon's own per-user slice,
and says so once in the audit trail and once on stderr at startup.

| Claim | Test |
|---|---|
| The signal is this process's cgroup, which a controlled user cannot write | `TestUserSliceUIDRefusesEverythingThatIsNotTheMarker` |
| The marker is found at any depth, in both cgroup-hierarchy forms | `TestUserSliceUIDFindsTheMarkerAtAnyDepth`, `TestUserSliceUIDReadsTheCgroupV1Form` |
| Only whole path elements count, so a uid embedded in a longer name does not match | `TestUserSliceUIDMatchesWholeElementsOnly` |
| An unreadable `/proc` answers "outside any user slice", which is the answer that keeps enforcement on | `TestReadSelfUserSliceAnswersOutsideWhenProcIsUnreadable` |
| A uid match alone never skips: the pair is required | `TestTheOwnSessionRuleNeedsBothHalvesOfThePair` |
| Only the daemon's own uid is skipped; every other account is still locked | `TestOnlyTheDaemonsOwnUidIsSkipped` |
| The refusal is announced once, not on every 30-second tick | `TestTheOwnSessionIsAnnouncedOnceNotEveryTick` |
| The rule is one predicate, and both halves are required | `TestDeclinesNeedsBothHalvesOfThePair` |
| The dashboard is handed the same location the engine is | `TestServeHandsTheDashboardTheSameLocationItHandsTheEngine`, `TestNewHandsTheSelfLocationToTheServer` |
| Both halves are asserted, not just the page's — the engine is assembled by a function a test can call, and a dropped field is caught | `TestServeHandsTheDashboardTheSameLocationItHandsTheEngine`, `TestServeHandsTheEngineItsLogSink` |
| A declined account is not rendered as locked out, and names the fix on the row | `TestADeclinedAccountIsNotShownAsLockedOut` |
| One declined session among several still counts as held | `TestOneDeclinedSessionAmongSeveralDoesNotMarkTheAccountUnenforced` |
| A logged-out account is not reported as declined | `TestAnAccountWithNoSessionIsNotReportedAsDeclined` |
| The one-sentence banner names the decline among its causes | `TestTheBannerNamesTheDeclineAmongItsCauses` |
| `serve` resolves the location and warns when the rule is live | `TestServeSaysWhenItWillNotLockItsOwnSession` |
| The line is split at the first two colons, so a `::` inside a path element cannot move the match | `TestUserSliceUIDSplitsTheLineAtTheFirstTwoColonsNotAtAnyColon` |

**Why the page has to know too.** The dashboard derived `LockedOutNow` from the schedule alone, so
the moment the engine gained a reason to decline, the page could announce "casey is locked out
until 06:30" about a child sitting at an unlocked screen — and the banner is the sentence a parent
reads on a phone from another room. An audit line the operator never tails is not a substitute for a
page that states the opposite. Both halves are therefore handed the *same* `SelfLocation` value from
one place in `serve`, and the page renders a declined account as scheduled-and-unenforced with the
fix named on the row.

**Why not process ancestry.** `Session.Leader`'s comment used to instruct the caller to compare the
session leader against its own ancestry. Measured on a live Plasma session, a shell in a GUI
terminal traces `bash → the agent → systemd(6695)`, and 6695 is the leader of the
`user@1000.service` *manager* row — not of session 3, the seat0/tty2 desktop on the screen. A
systemd user manager reparents GUI processes onto itself, so ancestry reaches the manager session
(which `IsLockable` already refuses) and never reaches the one that matters. A guard built on it
would silently never fire, which is worse than no guard at all: it reads as a guard that passed.

**Why the uid comparison is enough here.** Issue #67 is right that uid alone does not settle the
policy question — an admin and a child sharing one account share a uid, and nothing available to a
root daemon separates their sessions. But this is not asking *"is this the child's session?"*; it is
asking *"would locking this session lock the screen I am sitting behind?"*, and for that question
the uid is the right and only signal. Where the two genuinely share an account the rule declines
and says so in the trail rather than going quiet, which is the same rule §4 already applies to a
session it cannot read.

**Scope: the daemon, not every process that can lock a screen.** The rule belongs to the
long-running loop, and the CLI deliberately does not consult it. `plock lock <user>` and
`plock lock --all` lock the operator's own session if it is in the set, because a one-shot
lock is not a lockout: the operator types their password and is back at their desktop in one
keystroke. The daemon's re-assertion is a different thing — it re-locks every tick for the
whole window, so a daemon running behind a locked screen can never be stopped, reconfigured,
or turned off from the account it is enforcing against. That asymmetry is the whole rule, and
it is why the guard sits in `internal/enforce` rather than in `platform.Chain.Lock`.

Two consequences are written down rather than left to be discovered:

- `plock lock --all` is documented as a smoke test, and it is the command someone types from
  the desktop they are about to lose. It now says so on stderr when the account it is about to
  lock is the one they are typing in, and still locks it — the warning is not a refusal,
  because refusing would break the smoke test for the person who means it.
- `plock status` reports a declined session, and says where the answer came from. The
  label cannot come from the slice of whoever ran `status` — doctor and status are run from
  a desktop terminal by an operator whose own session is in a user slice even when the
  daemon is a system unit that declines nobody, so guessing from the CLI's own process
  would put a confident wrong label on the row. It comes from the daemon's cgroup instead
  (`platform.LocationOf`, the same parser the daemon runs on itself), and the report keeps
  three states apart: not running, running and inside `uid`'s session, and running but
  unreadable — where nothing is marked and a warning says the command could not look
  (`TestStatusMarksTheSessionTheDaemonRunsIn`,
  `TestStatusSaysItCannotTellRatherThanDecliningNobody`,
  `TestStatusSeparatesNoSystemdFromNotRunning`).
  `doctor`'s `daemon.slice` check answers from the same probe rather than concluding the
  unit's location from `is-active`, so the two commands cannot disagree about where the
  daemon is (`TestCheckDaemonSlice`); it consults the shell's own slice only when no service
  is running, which is the one case where the daemon in question really is the one about to
  be started from there.

---

## 5. Trust boundary: "did the lock actually take?"

This is the boundary the child attacks most directly, because `SetLockedHint` is authorized by uid
alone. **The lock state the daemon reads is a value the adversary can write.**

| Claim | Test |
|---|---|
| A hint that was already true before we asked is discounted; the chain keeps going | platform suite (`Chain.Lock` pre-read) |
| A backend corroborates only if it *says* it corroborates — not because it implements an interface | `TestAGnomeOnlyChainNeverConfirmsOffAPreSetHint` |
| The locker-identity check resolves the real binary via `/proc/<pid>/exe` and does not trust `argv[0]` | `TestTheIdentityCheckRejectsAForgeableProcessTree`, `TestAForgeableFixtureTreeIsRejected` |
| Matching uses the full cmdline, not the 15-character `comm` | `TestLockerRunningMatchesOnCmdlineNotTheTruncatedName` |
| The effective uid is checked, not the real one | `TestLockerRunningMatchesTheEffectiveUidNotTheRealOne` |
| A zero exit status is not evidence the lock took | platform suite (`Confirmer`, `ErrUnconfirmed`) |
| The lock budget bounds the whole operation, and its expiry is never reported as success | `TestChainLockBudgetOnlyTightensACallerDeadline`, `TestChainLockBudgetExpiryIsNeverReportedAsSuccess` |
| A chain with no backends refuses loudly | `TestChainWithNoBackendsRefusesLoudly` |
| The premise of this whole section — that `SetLockedHint` is authorized by uid alone — still holds on the target host | `testing/e2e/forge-hint.sh`, unprivileged mode: flips your own session's hint, reads it back, shows no locker running while it says "locked", restores it. Re-run from a clone, not from memory; the development host was measured 2026-10-18 (`no` → `yes` → restored, uid 1000, no polkit prompt). Its root mode checks the other half: that the loop keeps asserting while the hint lies. |
| A cancelled context is a shutdown, not a lock outcome | enforce suite |

---

## 6. Trust boundary: the admin web UI

The server is on `main` (`internal/web`, `plock serve`). The claims below were written before
the code existed; each one now names a test that runs.

| Claim | Test |
|---|---|
| The password verifier's cost cannot be silently downgraded | `TestADowngradedCostIsRefused` |
| The iteration floor is enforced at the trust boundary, not in the decoder | auth suite |
| Constant-time comparison is used for the credential check, and the guard follows the operands rather than the identifiers | `internal/auth` structural test |
| Ten attempts then blocked, and a blocked key is refused without doing any KDF work | `TestTenAttemptsThenBlocked`, `TestABlockedKeyIsRefusedWithoutAnyVerificationWork` |
| The throttle is per key and safe under concurrency | `TestThrottleIsConcurrencySafe` |
| Revoking all sessions catches one started just before the revoke | `TestRevokeAllCatchesASessionStartedJustBeforeIt` |
| Expired sessions are swept without being looked up | `TestExpiredSessionsAreSweptWithoutBeingLookedUp` |
| A weak admin password is refused, on both surfaces that can set one, against one shared floor | `TestInitRefusesAWeakPassword`, `TestAShortNewPasswordIsRefusedByTheSameFloorAsTheCLI` |
| The password form never puts either password on the page, in the audit trail, or in the log &mdash; on the paths that succeed included | `TestARefusalNeverCarriesEitherPassword` |
| A wrong current credential on the rotation form is refused as a failed login, not as a failed rotation | `TestAWrongCurrentPasswordIsRefusedWithTheLoginSentence` |
| Rotating to the password you just proved you know is refused, so a no-op cannot end every session | `TestRotatingToTheSamePasswordIsRefused` |
| The listener binds where it says it binds, verified on the wire | `internal/web` transport test |
| Rotating the admin credential ends the sessions minted under it, and the new password works without a restart | `TestARotatedCredentialEndsSessionsMintedUnderTheOldOne`, `TestTheRotatedPasswordWorksAndTheOldOneStopsWorking` |
| The credential is read from the config, so a value handed to the server cannot outrank the file | `TestAHandedAuthenticatorDoesNotOutrankTheConfiguration` |
| A config rewrite that leaves the password alone keeps the operator's own session | `TestAConfigRewriteThatKeepsThePasswordKeepsTheSessions` |
| A transient config read error is not treated as a rotation, and starting the watcher is not one either | `TestAConfigReadFailureDoesNotLogAnybodyOut`, `TestStartingTheWatcherDoesNotRevokeAnything` |
| An assignment names an account the machine has, and a listing that could not be read is answered as the server's failure rather than as "no such account" | `TestAssigningAnAccountNobodyHasIsRefused`, `TestAFailedAccountListingIsNotAnsweredAsNoSuchAccount` |

---

## 7. The CLI refuses rather than repairs

`plock init` runs as root and touches directories that may already exist with modes it did not
choose. `chmod`ing a path a root process did not open is an arbitrary-chmod primitive, so the rule
throughout is: **refuse, and say which guard fired.**

| Claim | Test |
|---|---|
| A loose state directory is refused rather than chmodded | `TestInitRefusesALooseStateDirRatherThanChmoddingIt` |
| A symlinked state directory, or a symlinked *ancestor*, is refused | `TestInitRefusesASymlinkedStateDir`, `TestInitRefusesASymlinkedAncestor` |
| A state directory that is a file, or an ancestor that is a file, is refused | `TestInitRefusesAStateDirThatIsAFile`, `TestInitRefusesAnAncestorThatIsAFile` |
| A state directory owned by another account is refused | `TestInitRefusesAStateDirOwnedByAnotherAccount` |
| The config directory's ceiling is 0755 — a packager's `/etc/plock` is accepted, a group-writable one is not, and the mode is left alone either way | PR #51 |
| A config that loads but is invalid is never written | PR #50 |

The last two rows cite pull requests rather than tests, which §10 counts as a gap: those tests are
written and under review but not in this tree yet. Replace the citations with the test names once
they merge — and until then these two claims are the only ones in this file that a `go test` run on
`main` cannot check.

---

## 8. Not defended

Listed because an unlisted gap is indistinguishable from an oversight.

| Not defended | Why this is the right answer |
|---|---|
| **An adversary with root** | No user-space daemon can defend against this. The daemon's root-ness is a barrier to the *session* user, and nothing stronger. |
| **Blocking a login at the greeter** | Out of scope by design (`docs/PLAN.md` §2). The child can log in; the session is re-locked within one poll interval. |
| **Power states across reboot** | Out of scope by design. A reboot during bedtime is re-locked at the next poll after the session returns. |
| **A raw VT session** | Deliberately kept lockable even though it can never be confirmed. Refusing `Type == "tty"` would make `Sessions()` report nobody, the loop find nothing to do, and the machine go quiet while the child keeps typing — the same outcome for the child, minus the signal. The loud permanent `ErrUnconfirmed` is the safe failure. |
| **Application or website filtering** | Out of scope by design. |
| **The child reading the audit trail** | They cannot — §3. The operator can: `internal/audit` has a reader that follows rotation, but no command exposes it yet, so today the trail is read with `tail` and `jq` over a root-owned file. |
| **GNOME corroboration of a lock** | No second signal has been measured on GNOME hardware. A GNOME-only config therefore reports `lock.unconfirmed` on transitions. Issue #35. |
| **A child sharing an account with the admin, when the daemon was started from that desktop** | The daemon declines to lock its own login session, because locking it removes the only way to stop the daemon. This never applies to the supported deployment — a systemd system unit lives under `system.slice` and shares no slice with any user, so the rule is inert and every account is enforced. Where it does fire, `plock serve` says so on stderr at startup, the trail carries one `lock.unenforceable` line naming `deploy/plock.service` as the fix, and `plock status` marks the affected session `declinedByDaemon` instead of leaving it to look like a failing backend (§4). |
| **A non-Linux build** | The guards fail closed — `ownerIsWriter` and `fileUID` return `errOwnershipUnsupported` and the writer refuses — but the *race* protection is lost: without `O_NOFOLLOW` a path can still be swapped between check and open. v0.1 ships Linux only. |
| **A malicious package or supply chain** | There are zero third-party modules, which is the whole mitigation available (`scripts/check-deps.sh`). |
| **A child-writable directory on the daemon's `PATH`** | Every program the daemon starts is resolved by `ExecRunner.Run` against the *daemon's* own `os.Getenv("PATH")`, so if the child can write a directory on that PATH, the child's `gdbus` or `loginctl` is what runs — including one that prints `()` and exits 0, which is exactly the reply the void-reply check accepts. Resolution does not change this and never claimed to: `LookPath` walks the same PATH `exec` would. Closing it means refusing to run a program whose location cannot be trusted, and a refusal does not lock anyone out, so the fix fails in the direction this project treats as the dangerous one. It needs a decision about which is worse — a lockout that silently stops, or a lockout that can be spoofed — and that is a product call, not a detail. See `internal/platform/exec.go`. |

## 9. Residual risk, honestly ranked

1. **The e2e step has no transcript in a PR.** `docs/PLAN.md` §13 asks for a run against a real KDE
   session with the transcript in the PR. One run happened, on the development host, on 2026-10-15:
   the screen locked, was confirmed on both signals, re-locked 10 s after a manual unlock, and left
   three audit lines behind. It counts for less than nothing here, because the harness that produced
   it lived in a scratch directory outside the repository and its transcript was never attached to
   anything — neither the run nor its evidence can be reproduced from a clone. The harness is now
   `testing/e2e/e2e.sh` and the run has to be repeated from it, with the transcript pasted into a
   PR, before this item comes off the list. Nothing in this document substitutes for that run: every
   claim above is a unit test, and unit tests here run against fixtures for the bus and `/proc`.
2. **The Plasma premise is unmeasured.** `docs/PLAN.md` §7 records that Plasma "historically"
   ignores logind's `Lock` signal. The chain's shape does not depend on which way that turns out,
   but the *operator experience* does: if Plasma honours it, the chain confirms on logind and the
   desktop backend is a no-op; if it does not, the desktop backend is load-bearing.
3. **`gdbus` and `loginctl` are resolved from the daemon's `$PATH`.** A child who can write a
   directory on that PATH can answer the lock call themselves. Root's `PATH` normally contains
   nothing world-writable, so this is a host assumption rather than a hole. It is *not* the child's
   environment doing the resolving: `exec.CommandContext` calls `LookPath` when it is constructed,
   reading the daemon's `os.Getenv("PATH")`, before `cmd.Env` is assigned — measured with two
   copies of one tool, `cmd.Path` is the daemon's copy and the daemon's copy runs. The exposure is
   the daemon's own PATH, which is why it applies identically to `loginctl` and why resolving the
   name in advance closes nothing.
4. **The audit trail proves the daemon acted, not that the screen locked.** That distinction is the
   whole content of §5, and it survives every defense there: on a backend that cannot corroborate,
   the honest answer is "unconfirmed", and the trail says so.

---

## 10. Keeping this document true

Three mechanisms, in decreasing order of how much they catch:

1. **`make verify`** — gofmt, vet, build, `-race` tests, the version stamp, the zero-dependency
   guard, and cross-builds for linux/amd64, darwin/arm64 and windows/amd64.
2. **`scripts/check-mutations.py`** — every mutant pattern in `scripts/mutations/` must apply
   exactly once and compile. Run it before any battery and after any rebase or merge; it derives its
   root from its own location, so run it from inside the worktree.
3. **`make mutate`** — puts each known bug back, one at a time, and insists the suite notices. A
   green `go test` says the code passes the tests; it does not say the tests would fail if the code
   were wrong. For a daemon whose failure mode is a lockout that silently stops happening, that
   difference is the whole product.

A claim in this file that no test backs is a bug in this file. If you find one, the fix is to write
the test — not to delete the claim, and not to weaken the guard to match the test.
