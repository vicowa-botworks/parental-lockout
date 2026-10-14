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

This has not been answered, because answering it means locking a live desktop and this box
has a person working at it. It is the first thing to check on any new machine, and the
expected answer is stated so that a wrong one is obvious:

**Expected: yes.** `ksmserver` calls `SetLockedHint(true)` on its own logind session when
the locker starts. If it does not, a KDE lock can never be confirmed, and `Chain.Lock`
will call **every** backend on **every** poll and finish with `ErrUnconfirmed` — the screen
gets locked, the daemon reports it did not, and the audit log is wrong.

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
is not an input to the "can we stop?" decision. Corroboration that the session user cannot
forge — a locker process parented by that session's compositor — is the obvious next
signal and is deliberately *not* implemented yet; it belongs in a backend, not in the
engine.

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
and which one a lock would go to. Exits non-zero when nothing can lock — which is the
state that otherwise looks identical to "nothing needed locking".

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
