# Platform backends: what was measured, and how to verify it by hand

This file exists because the two lock commands the ticket named **do not exist on the
target platform**, and the tool it named fails in the one way a daemon cannot detect.
Everything below was measured on the box this daemon is being built for, and every claim
is reproducible with the commands given.

Target: Ubuntu, KDE Plasma **6.6.6**, X11, systemd **259.5**, one logged-in desktop session.

---

## 1. What actually locks the screen

| what the ticket said | measured result |
|---|---|
| `qdbus org.kde.KScreen /Screen lock` | **No such object path `/Screen`.** `org.kde.KScreen` is owned by `kscreen_backend`, the display-configuration daemon, and exposes only `/backend`. |
| `qdbus org.kde.ksmserver /KSMServer org.kde.KSMServerInterface.lockScreen` | **No such method.** `/KSMServer` exists; its methods are `closeSession currentSession isShuttingDown resetLogout restoreSession saveCurrentSession saveCurrentSessionAs sessionList` plus the standard three. Nothing containing "lock". |
| `gdbus … org.gnome.ScreenSaver … Lock` | The *gnome* command is also the *KDE* command — see below. |

What is really there:

```
$ busctl --user introspect org.kde.screensaver /ScreenSaver
…
method void org.freedesktop.ScreenSaver.Lock()
signal void org.kde.screensaver.AboutToLock()
```

ksmserver exports `/ScreenSaver` under three well-known names — `org.kde.screensaver`,
`org.freedesktop.ScreenSaver`, `org.kde.ksmserver` — and the interface is
`org.freedesktop.ScreenSaver` under all of them. The locker process is
`/usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet`, which owns **no** screensaver bus
name (the name `org.kde.screensaver` is a trap for anyone searching for it).

So `kde` and `gnome` are one mechanism with a different preference order for the bus name,
which is why they are one type in `desktop.go`.

## 2. Why `gdbus` and not `qdbus`

```
$ ls -l /usr/bin/qdbus
lrwxrwxrwx 1 root root 23 … /usr/bin/qdbus -> /etc/alternatives/qdbus
… -> /usr/bin/qtchooser
$ qdbus org.kde.screensaver /ScreenSaver
qdbus: could not find a Qt installation of ''
$ echo $?
0
```

**Exit 0.** A backend that shelled out to `qdbus` and trusted the exit status would report
a successful lock every 30 seconds forever while the screen never locked — and because
`Chain.Lock` only advances to the next backend when the lock *verifiably* took, a false
success also stops the fallback chain dead. This is the fail-open this daemon exists to
avoid, arriving through a symlink.

`/usr/bin/qdbus6` is the real Qt6 tool and does work. `gdbus` (glib) is present on KDE and
GNOME alike, reaches the same service, and has no Qt-selection step to get wrong, so it is
what the backend uses. Independently of which tool is used, the backend also **checks the
reply**: `Lock` is a void method, so `gdbus` prints exactly `()`. Anything else — including
empty output on a zero exit — is a failure.

Verify the reply shape without locking anything (this calls a harmless void method):

```
$ gdbus call --session --dest org.freedesktop.ScreenSaver --object-path /ScreenSaver \
    --method org.freedesktop.ScreenSaver.SimulateUserActivity
()
```

## 3. Confirmation cannot come from the desktop

Two candidate signals were measured, and **both are unusable**:

```
$ busctl --user get-property org.kde.screensaver /ScreenSaver \
    org.freedesktop.ScreenSaver Locked
Property org.freedesktop.ScreenSaver.Locked was not found in object /ScreenSaver

$ gdbus call --session --dest org.freedesktop.ScreenSaver --object-path /ScreenSaver \
    --method org.freedesktop.ScreenSaver.GetActive
(true,)
```

There is no `Locked` property. And `GetActive` returns **`(true,)` on a desktop that is
unlocked and in active use** — whatever it means (enabled rather than engaged, most
likely), it is not "locked". A backend that confirmed with `GetActive` would report every
lock as confirmed, forever, on every machine. It is not called anywhere in this package,
and a test asserts that no confirmation path asks it.

Confirmation therefore reads **logind's `LockedHint`**, which is what the `logind` backend
already reads:

```
$ loginctl show-session "$XDG_SESSION_ID" -p LockedHint
LockedHint=no
```

It is the only signal available, and it is **not** trustworthy as evidence: the session
owner can write it themselves, unauthenticated. Read §5 before treating any confirmation
as proof that a child was actually locked out.

## 4. The open question — answer this first on a real machine

> **Does locking via `org.freedesktop.ScreenSaver.Lock()` cause the desktop to write
> `LockedHint=true`?**

**Answered on 2026-10-28, in a KDE guest: yes.** Ubuntu 26.04, `plasma-desktop 4:6.6.6`,
sddm autologin of a second user on seat0, Wayland session. `org.freedesktop.ScreenSaver.Lock`
at `/ScreenSaver` on `org.kde.screensaver` — the daemon's own route, exactly as
`internal/platform/desktop.go` calls it — brought up the real Plasma locker, `LockedHint`
went `yes` and `GetActive` read `(true,)`, and the password cleared it. Rig, recipe and
screenshots: `/home/rodney/vm/kde179`, `s4-lock.sh`.

It is still the first thing to check on any new machine, because a `no` fails silently:
something in the session calls `SetLockedHint(true)` when the locker starts (on this build
that is the compositor, not ksmserver — see the corroboration section below), and if nothing
does, a KDE lock can never be confirmed and `Chain.Lock` will call **every** backend on
**every** poll and finish with `ErrUnconfirmed` — the screen gets locked, the daemon reports
it did not, and the audit log is wrong.

Recipe:

```bash
# 1. Note the current state.
loginctl show-session "$XDG_SESSION_ID" -p LockedHint        # expect: LockedHint=no

# 2. Lock through the interface the backend uses.
gdbus call --session --dest org.kde.screensaver --object-path /ScreenSaver \
    --method org.freedesktop.ScreenSaver.Lock

# 3. Immediately re-read the hint, from a second session or over SSH.
loginctl show-session "$XDG_SESSION_ID" -p LockedHint        # EXPECT: LockedHint=yes
```

If step 3 says `no` while the screen is visibly locked, **stop**: confirmation is broken on
this desktop and the daemon will never report a successful lock. Record it against the
issue for this platform rather than relaxing the confirmation.

### What the wrong answer costs in wall clock

Counted on a box with a session bus that owns `org.kde.screensaver` and never replies, with
the shipped default chain `[logind, kde, gnome]` and `loginctl` unaffected:

| Shape | Subprocess calls per `Chain.Lock` |
| --- | --- |
| bus owns the name, never answers | **18** (5 `gdbus` + 13 `loginctl`) |
| desktop accepts the lock, never writes the hint — the answer to the question above is `no` | **37** (2 `gdbus` + 35 `loginctl`), because every backend runs its own 11-read settle window |

Both figures are pinned by `TestDocumentedCallCountsPerChainLock`, which counts the
commands a default `[logind, kde, gnome]` chain actually issues. They moved from 17 and 36
when `Chain.Lock` gained the state read it takes before asking anybody (§5); a table nobody
re-measures is a table that quietly stops being true.

Each call is bounded by `DefaultCallTimeout` (10s), so those are 3m and 6m10s for **one
lock of one session**. That is why the per-command guard is not the whole answer and
`Chain.Lock` carries an operation-level bound of its own, `DefaultLockBudget` (25s, inside
one default tick): running out of it leaves the remaining backends unasked and returns an
error or `ErrUnconfirmed`, so the enforcement loop keeps the lockout and asks again next
tick.

The arithmetic that is left, stated rather than hidden: a tick that locks *N* sessions can
still take *N* × `DefaultLockBudget` in this shape. The engine's overlap guard is what
keeps that from stacking ticks — it skips the next one and says enforcement is landing
late.

## 5. The confirmation signal is writable by the person being locked out

**Measured on the target box, 2026-10-14.** `LockedHint` is not a secret. The session
owner can write it directly, with no authentication at all:

```console
$ id -u
1000                                    # the child's own uid; no sudo, no polkit prompt
$ gdbus call --system --dest org.freedesktop.login1 \
    --object-path /org/freedesktop/login1/session/_3 \
    --method org.freedesktop.login1.Session.SetLockedHint false
()                                      # accepted
$ loginctl show-session 3 -p LockedHint
LockedHint=no                           # and it took effect
```

The authorization surface, measured rather than inferred:

```console
$ grep -c '<action id=' /usr/share/polkit-1/actions/org.freedesktop.login1.policy
37
$ grep -ci hint /usr/share/polkit-1/actions/org.freedesktop.login1.policy
0
```

There is no polkit action covering a lock hint. `org.freedesktop.login1.lock-sessions`
gates the *request* to lock; the *report* that the lock took is ungated, because logind
has to let any desktop announce its own state. A session that genuinely cannot be locked
is refused on capability, not authorization (`NotSupported: Session does not support lock
screen` for the seatless manager session), so that error path is not what protects the
hint.

**Consequence.** An autostart entry that sets the hint to `true` at login makes every
confirmation succeed while the screen stays up. This is why the enforcement loop
([#29](https://github.com/vicowa-botworks/parental-lockout/issues/29)) re-asserts the lock
on every tick instead of skipping when a session reports itself locked:

> A signal the controlled user can write may make enforcement **quieter**. It must never
> decide when enforcement **stops**.

`Session.Locked` still carries the hint, because it is genuinely useful for reporting. It
is not an input to the "can we stop?" decision.

### What closes it, and what that leaves open

Two changes, and neither one is sufficient alone.

**1. `Chain.Lock` reads the hint before it asks anybody.** A hint that was already true
cannot be the consequence of a request that had not been made yet, so a yes from a backend
that cannot corroborate it counts only if the state changed under us. This is what makes
the attack fail: the chain stops believing the pre-set hint, keeps going, reaches the
desktop backend, and the screen actually locks.

It is one read per `Chain.Lock`, not one per backend, and it asks the backend that reports
the **raw** hint rather than the first one in the list. Order matters: a chain configured
`[kde logind]` that asked the corroborator would correctly hear "not locked" (no locker),
and then take logind's bare-hint yes at face value — the attack, arriving through the
configuration rather than the code.

**2. The KDE backend corroborates.** Measured on the target, 2026-10-14, with the session
genuinely locked:

```console
$ ps -eo pid=,ppid=,uid=,args= | grep kscreenlocker
2224790    7619  1000 /usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet --graceTime 5000 --ksldfd 56
$ ps -o args= -p 7619
/usr/bin/ksmserver
$ loginctl show-session 3 -p LockedHint
LockedHint=yes
```

The locker is a child of `ksmserver` — the process that owns `org.freedesktop.ScreenSaver`
and answers the `Lock` call. So a KDE confirmation requires **both** the hint and a live
`kscreenlocker_greet` owned by that session's uid whose parent is `ksmserver`. Unlocked
reads `LockedHint=no` with no locker running, which is the other half of the calibration.

Two details that cost a look:

- `ps` and `/proc/<pid>/comm` truncate the name to 15 characters — `kscreenlocker_g`. The
  check reads `cmdline`, which is not truncated, so it matches the real argv0 rather than a
  prefix that two programs could share.
- The walk reads `/proc` directly instead of running `ps`. This is a security check, and a
  check answered by a PATH lookup can be answered by whatever the session user put earlier
  in PATH — and the session user owns the environment these commands run in.

**Why the corroborator is not optional decoration.** The loop re-asserts every tick, so
from the second tick on a genuinely locked machine *always* reads "already locked".
Discounting a corroborated yes there would report `ErrUnconfirmed` forever on a locked
screen, and an operator trained to ignore that signal has no signal left. So the rule is
"an uncorroborated yes needs a transition", not "never trust the hint".

**What this does not prove, and what it now does.** An earlier draft of this page said the
corroborator moves the impostor's job to "get code running as a child of the session's own
compositor", and that `ptrace` was the next rung up. That was wrong, and it was wrong in the
direction that matters: the reviewer built the tree and ran the check against a real `/proc`.

The first version decided identity from `filepath.Base(argv[0])`. `argv[0]` is whatever the
caller passed to `execve`. Two `execve` calls as the session user — no `ptrace`, no
relationship to any compositor, nothing KDE running at all — produced a parent claiming to
be `/usr/bin/ksmserver` and a child claiming to be `kscreenlocker_greet`, and the KDE
backend reported a **corroborated** yes. The bar as implemented was the same bar as writing
the hint: run code as the session user, which is exactly what the `~/.config/autostart` file
in #29 already is.

So identity is resolved from `/proc/<pid>/exe`, a symlink the kernel maintains, for both
halves of the tree; the name is kept only as a cheap pre-filter. Two things are then
required of the resolved binary: its basename is the expected program, and the session user
could not have written it — not owned by them, not group- or world-writable. The canonical
*path* is deliberately not the test, because it differs by distribution
(`/usr/lib/x86_64-linux-gnu/libexec` on Ubuntu, `/usr/libexec` on Fedora) and pinning one
would break the real desktop to stop the fake one. What does not vary is that the session
user cannot write into the directory the real locker lives in.

The impostor therefore now needs a binary named `kscreenlocker_greet` that the session user
cannot modify, parented by one named `ksmserver` that the session user cannot modify — the
real pair, or a write that needs root. That is a bar raised, not a wall: the locker still
runs as the child's own uid, and nothing here proves the screen is locked.

`locker_test.go` drives a synthetic `/proc`, and a fixture *is* a claimed identity, so it
structurally cannot catch a false claim. `locker_forged_test.go` is the case that can: it
execs a helper with `argv[0] = "/usr/bin/ksmserver"` that spawns `/bin/sleep` with
`argv[0] = "kscreenlocker_greet"`, against the real `/proc`, and asserts rejection — with a
positive control that resolves our own binary first, so the rejection cannot be vacuous.

**Cost of the walk.** `lockerRunning` reads `/proc/<pid>/status` for every process on the
machine, and it runs on every `LockConfirmed`, which `confirmWithSettle` calls repeatedly
inside the settle window, per backend. The uid filter now runs before anything is resolved,
which is what keeps the expensive reads (`cmdline`, `exe`) to the session's own processes.
`ctx.Err()` bounds the walk against the lock budget but not against the tick; caching the
answer for the duration of one `Chain.Lock` is the obvious next step and is filed as a
follow-up rather than done here, because a cache of a security answer needs its own
reasoning about staleness.

**GNOME is not corroborating, and now that is a measurement rather than a guess.** GNOME
draws its lock screen inside `gnome-shell`, so there is no separate locker to find. The
candidate signal is `org.gnome.ScreenSaver.GetActive`, measured on 2026-10-28 on GNOME
50.1 in a throwaway VM (`/home/rodney/vm/gnome35`, recipe in issue #177):

- `GetActive` answers `(false,)` on an unlocked desktop. So unlike the KDE spelling of the
  same-named call, which answers `(true,)` unlocked (§3), the GNOME one does mean something.
- It is not usable as a corroborator. The bus name is owned by `gsd-screensaver` **running
  as the logged-in child's own uid**, and the same interface exports `SetActive(b)` to that
  bus. A signal the controlled user can write cannot corroborate a lock (#29), whatever it
  reads while nobody is touching it.
- `SetActive(false)` from the child's own session bus dismisses the lock shield with **no
  password**. The shield is a real lock — the wrong password is rejected and the right one
  unlocks — so this is not a screen that was never locked. `SetActive(true)` locks, and
  `GetActive` then reads `true` while logind's `LockedHint` reads `yes`: both of the
  signals the daemon has are inside the child's reach.
- It is not a GNOME quirk. The Plasma host exports the same method on the same-named
  interface, on the user's own bus:

  ```
  $ busctl --user introspect org.freedesktop.ScreenSaver /org/freedesktop/ScreenSaver
  org.freedesktop.ScreenSaver  interface - - -
  .SetActive    method  b  b  -
  ```

  Measured on 2026-10-28 in a KDE guest (`/home/rodney/vm/kde179`, #179), and the answer is
  an asymmetry rather than the symmetry this section assumed: **Plasma 6 honours
  `SetActive(true)` and refuses `SetActive(false)`.** From a desktop whose unlocked state was
  established first — password in, `GetActive` `(false,)`, `LockedHint` `no` — the child's
  `SetActive(true)` raised the real Plasma locker and both signals read locked; `SetActive(false)`
  then returned `(false,)` and changed nothing, and only the password cleared it. So the #177
  bypass is `gsd-screensaver`-specific, not a property of the interface. What *is* true on both
  desktops is the other direction: the child can raise a lock the daemon never asked for, and
  no signal available to the daemon distinguishes it from a real one.

  The same measurement moved the surface itself. On Plasma 6 / Wayland,
  `org.freedesktop.ScreenSaver` is owned by **`kwin_wayland`** at the child's uid and exports
  the interface at `/ScreenSaver`, while `/org/freedesktop/ScreenSaver` — the path that
  answers on the X11 host, where ksmserver owns the name — exports nothing there but the
  generic D-Bus interfaces. `GetActive`'s *unlocked* answer is owner-dependent too: `(true,)`
  from ksmserver, `(false,)` from kwin. So a probe must resolve the path rather than assume
  it, and there is no single answer to "what does `GetActive` mean" to design a corroborator
  against.

The GNOME backend therefore still reports itself uncorroborated, `Probe` says so in words,
and the consequence is unchanged: on a GNOME-only configuration a lock is confirmed on the
tick that changes the hint and reported unconfirmed on every tick after it. What the
measurement settles is #35: the signal it asked to have measured has now been measured, and
there is no GNOME lock signal that is not served by the child's own bus, so there is nothing
to wire in. What carries the case from here is `lock.released`, which makes a dismissal
visible instead of merely re-asserted away.

### The premise this whole design rests on is still unmeasured

`docs/PLAN.md` §7 justifies the desktop backend by saying Plasma "historically" ignores
logind's `Lock` signal. That is a carried-forward design premise, **not** a measurement:
confirming it means locking a live desktop, same as §4. Nothing in this repository claims
to have measured it, and `plock doctor` deliberately reports only the order backends are
asked in, never which one a lock would reach.

The premise is load-bearing in one place that does not need a live desktop: a **raw VT**.
There logind only emits the `Lock` D-Bus signal, nothing ever calls `SetLockedHint`, and
`loginctl lock-session` still exits 0 — so a zero exit is demonstrably not evidence of a
lock, and the chain's "confirm or keep going" shape is justified without any Plasma
assumption.

To measure the Plasma case, run §4's recipe and additionally compare the two paths:

```bash
# A: lock via logind, as root.
loginctl lock-session "$XDG_SESSION_ID"; echo "exit=$?"
loginctl show-session "$XDG_SESSION_ID" -p LockedHint   # does the hint move?
# B: unlock, then lock via the desktop interface (§4 step 2), and re-read.
```

If A moves the hint on current Plasma, the premise is out of date — the desktop backend
becomes a fallback rather than the mechanism, and the chain order in `lockBackends` is
worth revisiting. Record the result against the platform issue either way.

## 6. Root → the session user's bus

The daemon runs as root and root has no session bus, so the child is dropped to the
session uid and given an environment built for that session.

```
$ ls -l /run/user/1000/bus
srw-rw-rw- 1 rodney rodney 0 … /run/user/1000/bus
$ ls -ld /run/user/1000
drwx------ 13 rodney rodney 540 … /run/user/1000
```

The socket is world-writable but its parent directory is `0700`, so only the owning uid or
root can reach it. Both the root daemon and a child dropped to that uid can connect; a
child dropped to a *different* uid cannot. That is the right property and it is pinned by a
test.

`DISPLAY` and `WAYLAND_DISPLAY` are deliberately **not** set. The call is a D-Bus method
call and `gdbus` reaches the bus from `DBUS_SESSION_BUS_ADDRESS` alone. logind reports a
session's `Type` (`x11` / `wayland` / `tty`) but not its display number, so setting them
would mean guessing `:0` — which on a machine with a second X server points the child at
somebody else's screen, and would be set for a tty session that has no display at all.

## 7. Checking a machine end to end

```bash
sudo plock doctor --config /etc/plock/config.json
```

Prints, per configured backend and **in configured order**: usable or unusable, why not,
and which one a lock attempt would be offered to **first**. Not which one would do the
locking — a backend is finished with only once its lock is confirmed, so the lock that
lands may come from a later entry in the list. Exits non-zero when nothing can lock —
which is the state that otherwise looks identical to "nothing needed locking".

Then, on a machine you are allowed to lock:

```bash
sudo loginctl list-sessions --no-ask-password          # find the child's session id
sudo loginctl show-session <id> --no-ask-password \
    -p Class -p Seat -p Type -p State -p Remote -p LockedHint
```

Expected for a lockable child session: `Class=user`, a non-empty `Seat`, `Type` of `x11`
or `wayland`, `State=active`, `Remote=no`. Anything else and `isLockable` refuses it on
purpose — see the comment on that function before changing the filter.

## 8. Things that are deliberately *not* treated as lockable

- `Class=greeter` — the login screen. Locking it leaves an uninputtable wall where the
  child's account was never chosen.
- `Class=manager` / `manager-early` — no desktop to lock. This box has one per logged-in
  user, so a filter that ignores `Class` calls `lock-session` on it every 30 seconds.
- No `Seat` — not on any console.
- `Type` empty or `unspecified` — the same thing seen from the other side.
- `Remote=true` — usually the admin's own session; locking it locks the admin out.
- `State=closing` — racing the teardown for nothing.

A raw VT (`Type=tty`) **is** kept lockable. Locking it does nothing, so every poll returns
`ErrUnconfirmed` forever — and that loud, permanent, reported failure was chosen over
filtering VTs out, because filtering makes `Sessions()` report nobody, which is the same
outcome for the child with the signal removed.

## 9. Adding a backend, and the `command` backend

A backend is a Go `init()` calling `Register(id, ctor)`, where the constructor takes the
runner **and the settings**:

```go
func init() {
	Register("kde", func(r platform.Runner, _ config.Settings) (platform.Platform, error) { ... })
}
```

The settings argument exists for one registered backend so far, and its presence is the
whole difference between a backend that can be configured and one that cannot. A backend
that needs nothing from the document ignores the argument; nothing else changes.

`plock doctor` then reports it in configured order, and `serve` refuses to start if no
configured backend is usable. That refusal is load-bearing: a daemon that starts, logs a
cheerful "listening" line and enforces nothing is the worst failure mode this product has.

### `command` -- and why `mock` is not registered

`command` runs a program named by `settings.lockCommandPath`, with
`settings.lockCommandArgv` as its arguments. It exists so the whole daemon -- `serve`, the
enforcement tick, the audit writer, the fallback chain -- can run on a machine with no
graphical session and no root. `make run-dev` is that loop.

The obvious alternative was `platform.Mock`, and `Mock` is deliberately **not** registered:

> A backend named "mock" that the registry would happily build from a config is a way for
> `lockBackends` to end up pointing at something that reports success and locks nothing --
> the exact shape of failure this daemon cannot afford, because it looks like it is working.
> -- `internal/platform/mock.go`

Registering `Mock` to satisfy a developer loop would put that escape hatch in the file the
daemon reads at boot as root. `command` is safe where `Mock` is not because **it really
execs something**: "does nothing" is not a hidden property of a magic identifier, it is an
explicit absolute path in a root-owned 0600 file, visible in `plock doctor`, and it cannot
be pointed anywhere by anyone who cannot already write the policy. That is the same trust
boundary `docs/security.md` §2 puts the whole config behind.

Four rules the backend keeps, each of which is a fail-open if dropped:

| Rule | Why |
|---|---|
| The path must be absolute; a bare name is refused | A PATH lookup inside the root daemon's lock path is an attack surface. #53 established this for the shipped lock binaries (`internal/platform/exec.go`, `resolveProgram`), and a config field is the same door. Refused in `Validate` *and* at construction, because `platform.New` is reachable with a `Settings` nobody validated. |
| Session listing delegates to `logind` | A backend that lists nothing makes the enforcement loop find no sessions and enforce nothing while every component reports health. |
| It does not implement `Confirmer` | Nothing about a subprocess exiting 0 says the screen is locked. Inventing a second signal to corroborate one would be the rubber stamp §5 is entirely about (`docs/security.md` §5 says the same about the real backends). So a chain using it reports `lock.unconfirmed`, which on a dev box is the honest reading. |
| `Probe` answers the lock-capable question and never runs the program | #25 item 4's direction, the one `probeTarget` implements: a probe that answers from the *listing* reports the backend available on a machine where every lock is about to be refused. And `plock doctor` is what you reach for when you suspect locking is broken -- a probe that locked your screen while diagnosing it would be both a surprise and a false positive. |

**This is not a production option, and nothing here pretends it can be made one.** A
`lockCommandPath` pointing at `/bin/true` configures a daemon that reports every lock as
requested and locks nothing, and the daemon cannot tell that apart from a real locker.
`plock doctor` prints `not a production backend` on the backend's own line so the state is
visible where an operator would look. Nothing can *refuse* it on a real machine: the config
is root-only, so configuring it is already an act only root can perform.

### Token substitution

`%SESSION%`, `%USER%`, `%UID%`, `%SEAT%` and `%TTY%` are replaced in every configured
argument. A token whose field is empty substitutes to an empty argument rather than dropping
it, because the argument count is part of what a script parses and dropping one shifts every
later argument into a different option. If the argv names no token at all, the session id is
appended -- a command backend with no way to learn which session it was asked about can only
lock whatever it guessed, and on a single-session dev box that looks identical to working.

### Measured, on a machine with no desktop session under test

```
$ PLOCK_DEV_ROOT=/tmp/plock-dev make run-dev
plock serve: backend command is available
enforce: running, every 5s, config /tmp/plock-dev/config.json
web: listening on http://127.0.0.1:8443

$ cat /tmp/plock-dev/lock-calls.log
20:58:09 --session 3 --user rodney --uid 1000
20:58:14 --session 3 --user rodney --uid 1000
20:58:19 --session 3 --user rodney --uid 1000
```

Five seconds apart, on a config that says `pollSeconds: 5`. The first version of this loop
printed 30s and ticked 30s apart while announcing 5s; see the note in
`internal/enforce/enforce.go` before changing how the period is published.

The audit trail carries `lockout.entered` and one `lock.unconfirmed`, and the unconfirmed
line appears once rather than every tick -- that is §5's transition rule, not a missing
event.
