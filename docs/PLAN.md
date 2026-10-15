# Parental Lockout — Design & Implementation Plan

## 1. Problem

Parents need to enforce *bedtime* and *school time* on a family computer running
KDE Plasma on Ubuntu. The child must not be able to defeat the restriction from
their own account.

The model is **lockout periods only** — like Google Family Link sleep time — not
cumulative quotas. We do **not** track "40 minutes per day"; we track "between
21:00 and 06:30 this account is not usable".

Because a determined child can dismiss the lock screen (kill the locker, switch
VTs, reboot into a live USB), the lockout is **re-asserted every 30 seconds** for
as long as the lockout window is active.

## 2. Non-goals (v0.1)

- Time budgets / quotas / "minutes remaining".
- Application or website filtering.
- Blocking the *login* of a user (we lock the *session*; login at the greeter is
  still possible and the session is re-locked within one poll interval).
- Accounting for power states across reboot (a reboot during bedtime just gets
  re-locked at the next poll after the session comes back).
- Non-Linux platforms (the design is abstracted for them, but v0.1 ships Linux only).

## 3. Language and dependencies

**Go.** Reasons:

- One static binary; no interpreter or runtime to install on the target.
- The standard library covers everything this app needs: `net/http`,
  `html/template`, `crypto/tls`, `crypto/x509` (self-signed cert bootstrap),
  `crypto/pbkdf2` (password hashing, stdlib since Go 1.24), `encoding/json`,
  `embed` (ship the web UI inside the binary).
- Trivial cross-compilation, which is how the "cross-platform" ambition stays
  cheap later.
- A root-owned daemon written in Python would depend on the system interpreter
  and its churn; Rust would need third-party crates for the HTTP server.

**Third-party module count: zero.** `go.mod` has no `require` entries. This is a
hard project rule and CI enforces it.

## 4. Shape of the system

```
                      ┌────────────────────────────────────────────┐
  parent's browser ──▶│  plock serve  (root, systemd)              │
   HTTPS :8443        │                                            │
                      │  web ─▶ api ─▶ config ─▶ /etc/plock/…json  │
                      │                 │                          │
                      │                 ▼                          │
                      │            schedule  (pure: is it bedtime?)│
                      │                 │                          │
                      │                 ▼                          │
                      │            enforce (30 s tick)             │
                      │                 │                          │
                      │                 ▼                          │
                      │            platform (lock backends)        │
                      └─────────────────┬──────────────────────────┘
                                        │ loginctl / qdbus / gdbus
                                        ▼
                              child's KDE Plasma session
```

One binary, `plock`, with subcommands. `plock serve` is the daemon; the CLI
subcommands talk to the same on-disk config so a parent can also administer the
machine from a terminal.

### Package layout

```
cmd/plock/            main + subcommand dispatch
internal/config/      config schema, validation, atomic store, hot reload
internal/schedule/    lockout-period evaluation (pure functions, no I/O)
internal/platform/    platform abstraction + backends (logind, kde, gnome, mock)
internal/enforce/     the 30-second enforcement loop and override state
internal/audit/     append-only JSONL audit log
internal/auth/        admin credentials (PBKDF2), login sessions, throttling
internal/web/         HTTP server, middleware, handlers, embedded templates
internal/users/       discovery of human accounts on the machine
```

Everything the daemon does goes through an interface so the enforcement loop can
be tested with a fake clock and a fake platform.

## 5. Data model

Stored as a single JSON document at `/etc/plock/config.json`, mode `0600`, root
owned, written atomically (temp file + `fsync` + `rename`).

```json
{
  "version": 1,
  "admin": {
    "username": "parent",
    "password": "pbkdf2-sha256$600000$<b64salt>$<b64hash>",
    "updatedAt": "2026-10-12T09:00:00Z"
  },
  "settings": {
    "listen": "0.0.0.0:8443",
    "tls": "auto",
    "pollSeconds": 30,
    "enforcement": true,
    "lockBackends": ["logind", "kde", "gnome"]
  },
  "profiles": [
    {
      "id": "schoolnight",
      "name": "School night",
      "timezone": "Europe/Berlin",
      "rules": [
        { "id": "r1", "name": "Bedtime", "days": ["mon","tue","wed","thu","sun"],
          "start": "21:00", "end": "06:30", "enabled": true },
        { "id": "r2", "name": "School hours", "days": ["mon","tue","wed","thu","fri"],
          "start": "08:15", "end": "13:30", "enabled": true },
        { "id": "r3", "name": "Holiday", "days": ["sat","sun"],
          "allDay": true, "enabled": true }
      ]
    }
  ],
  "users": [
    { "username": "kiddo", "profileID": "schoolnight", "enabled": true }
  ],
  "overrides": [
    { "username": "kiddo", "until": "2026-10-12T20:40:00Z",
      "reason": "homework with teacher", "by": "parent" }
  ]
}
```

Notes:

- **A rule is a wall-clock window on a set of weekdays.** `end < start` means
  the window crosses midnight and belongs to the *starting* day. `21:00–06:30` on
  `[…,"sun"]` therefore means Sunday evening into Monday morning. `end == start`
  enforces nothing and is rejected at save; it is deliberately not read as a
  24-hour window, because that is the shape a parent makes by typing the same time
  into both boxes.
- **`"allDay": true` is the rule with no minute free.** It replaces both clock
  ends, which are then not read and need not be present. `00:00–23:59` is *not*
  all day: it is 1439 minutes, and the minute it releases is two 30-second re-lock
  ticks of free machine on a schedule the parent read as all day. The flag is a
  field rather than a reserved `24:00` because `24:00` is `00:00` on the following
  day, which is exactly where America/Havana, America/Santiago and
  Atlantic/Azores put their clock change — a reserved value there would need its
  own disambiguation in the one place where the endpoint choice is already
  load-bearing. The form (#10) renders it as an "All day" checkbox so nobody
  types the boundary.
- **A user has exactly one profile**; a profile may be shared by several users.
  This matches Timekpr's rule-set-per-user and keeps the UI honest.
- **An override is a temporary unlock** with an expiry, so a parent can grant
  "30 more minutes" without editing the schedule.
- Timezone is per profile so a family with a second home or a travelling laptop
  behaves sanely; it defaults to the machine's local zone.

## 6. Schedule evaluation

Pure package, no I/O, fully table-tested:

```go
func (p *Profile) Evaluate(now time.Time) Verdict
type Verdict struct {
    Active     bool      // is the user locked out at `now`?
    Rule       *Rule     // which rule is responsible
    StartedAt  time.Time // when the current window opened
    EndsAt     time.Time // when it closes
    NextStart  time.Time // next window opening; set whether or not Active
}
```

Algorithm: for each enabled rule, test the windows that started on each of the
last three local dates — the day before yesterday, yesterday, and today — each
counting only if that date's weekday is in `days`. Three, not two, because a zone
can delete a calendar date: five Pacific zones crossed the dateline with a 24h
forward jump, so a window that opened the evening before a deleted date closes two
local dates later, and a two-day walk released the account for about 22 hours in
the middle of its own lockout. Take the **latest**
`EndsAt` among active windows, because that is how long the child stays locked
out; the earliest would release them while a second rule still holds. `NextStart`
scans the next 8 days of candidate window openings. DST is handled by resolving
each end through the profile's `*time.Location` — a start takes the earliest
instant whose clock reads the written time, an end the latest, and a time the
clock jumped over resolves to the first instant after the jump — never by adding
durations to a day, and never by trusting `time.Date`'s answer for an ambiguous
or missing time, which is wrong in the fail-open direction in both hemispheres.

## 7. Platform abstraction

```go
type Platform interface {
    Name() string
    Sessions(ctx context.Context) ([]Session, error)
    Locker
    Probe(ctx context.Context) []BackendStatus
}

type Session struct {
    ID, Username string
    UID          uint32
    Type         string // x11 | wayland | tty
    State        string // active | closing
    Remote       bool
    Locked       bool   // logind LockedHint, best effort
}

type Locker interface {
    Lock(ctx context.Context, s Session) error
}
```

Backends, tried in configured order, first success wins:

| id | mechanism | notes |
|---|---|---|
| `logind` | `loginctl lock-session <id>` as root | works on GNOME and most DEs; also the source of `LockedHint` |
| `kde` | `qdbus org.kde.KScreen /Screen lock` (Plasma 6) then `org.kde.ksmserver` (Plasma 5), executed **as the session user** with `XDG_RUNTIME_DIR`/`DBUS_SESSION_BUS_ADDRESS` pointed at their bus | Plasma historically ignores logind's `Lock` signal, so this is the one that actually works on KDE |
| `gnome` | `gdbus call --session --dest org.gnome.ScreenSaver … Lock` as the session user | future-proofing |
| `command` | operator-supplied shell command, run as the session user | escape hatch for anything we have not written a backend for |

Running a command inside someone else's desktop session is the subtle part: root
has no session bus. The helper builds the environment from
`/run/user/<uid>/bus` and drops privileges with `SysProcAttr.Credential`, no
`sudo` dependency.

### What the live target actually does (measured, systemd 259)

Recorded here because #6 builds on it and each item cost real probing:

- `loginctl list-sessions --output=json` is accepted, **ignored**, and prints the
  plain table with exit 0. The flag that works is `--json=short`. A backend on the
  wrong flag does not fail — it feeds a table to a JSON parser every 30 seconds and
  enforces nothing forever.
- The listing carries no `Remote` and no `LockedHint`. Lock state and remoteness
  require a per-session `show-session`, so the cheap `class=user` + has-a-seat filter
  runs first and the detail call only happens for real desktop sessions.
- `show-session` prints `Key=value` in neither the requested order nor alphabetical
  order, so properties are read into a map. An unknown property is **not** an error:
  exit 0 with no line at all, which makes a typo'd property indistinguishable from an
  unset one. An unknown session id exits 1 with "No session ... known".
- One uid can own several sessions. Alongside the desktop session there is typically
  a `class=manager` session with no seat, `Type=unspecified`, `State=active`,
  `Remote=no`. Neither `Remote` nor `State=closing` drops it, so `isLockable` also
  requires a real session type — otherwise the loop calls `lock-session` on a
  non-desktop session every 30 seconds forever.
- The `class` field is not a fixed set. A second machine on the same systemd was found
  with `class=manager-early`, and with `class=user` sessions that have no seat because
  they are SSH. Both are refused, but only because the class check is an allowlist and
  the seat check requires a console: on a host with no `Remote=yes` in sight, the seat
  requirement is what actually protects a remote admin.
- When `SysProcAttr.Credential` is set, the runtime calls `setgid(cred.Gid)`
  **unconditionally** before `setuid(cred.Uid)`. A `Credential` with only `Uid` set
  therefore does not leave the group alone: it sets group 0, so a "downgraded" helper
  runs as the session user with the root group. Any backend that drops privileges must
  resolve the real primary group; `platform.dropPrivileges` does this and #6 must call
  it rather than building a `Credential` by hand.
- Sessions vanish between the listing and the detail call during an ordinary logout.
  That single failure mode is skipped; every other per-session error is reported while
  the sessions that did answer are still returned, because a partially broken backend
  must not stop enforcement for the whole machine.
- **`org.freedesktop.login1.Session` has no `UID` property.** The uid is on the `User`
  object. Because an unknown property is exit 0 with no line, asking for `UID` yields
  nothing and no error, so a fallback built on it can never fire against a real
  `loginctl` while still passing every fake:
  `loginctl show-session 3 -p Name -p Type -p State -p Remote -p UID -p LockedHint`
  prints four lines and no `UID`. The listing is the only uid source.
- **An empty session table is not an error, and it has three shapes.** `loginctl`
  prints the bare header, or `N sessions listed.`, or — for an empty table — the
  legend `printf("No %s.\n", type)` from `loginctl.c`. Only the count form looks like
  a trailer, so recognising just that one turns "nobody is home" into a backend
  failure on every machine at 3am, in `Sessions()` and in `doctor` alike.
- **`loginctl lock-session` on a raw VT exits 0 and does nothing.** logind only emits
  the `Lock` D-Bus signal; `LockedHint` is written by the desktop environment calling
  `SetLockedHint`, and nothing on a VT ever calls it. `CanLock` is not a substitute —
  it is class-based, so it says `yes` for a VT. Such a session is therefore asked
  anyway, and the resulting permanent `ErrUnconfirmed` is the signal; refusing it would
  make the loop report nobody for that child and go quiet.
- **The hint is written asynchronously**, so reading `LockedHint` the instant
  `lock-session` returns races the desktop and loses on any machine where the desktop
  takes a few milliseconds. The chain re-reads for a bounded window instead; without
  that, a lock that lands is reported as a failure and the next backend locks twice.
- **Every `loginctl` call carries `--no-ask-password`.** The lock action is
  `auth_admin_keep`, so a `plock doctor` run by an ordinary user would otherwise open a
  polkit prompt and wait on a machine with no terminal.
- **`SINCE` is the one column whose value contains spaces**, so it is only safe as the
  last column: anywhere else every column after it shifts, and each shifted value is
  individually plausible (`CLASS` reading `no`), which is the silent misread the
  name-based mapping exists to prevent. The layout is refused rather than guessed.

`State` in the struct above is wider than `active | closing`. Only `closing` is
excluded, so any other value stays lockable — the safe direction, since a session
logind considers live but not current can still be walked up to. `active` and
`closing` are observed on the target; `online` is what systemd documents for a
remote session that is not current, and is **not** verified on this host because it
had no remote session when the fixtures were captured.

`plock doctor` probes every backend against the live machine and reports which
ones actually work — that turns "it doesn't lock on my KDE" into a one-command
diagnosis.

## 8. Enforcement loop

Every `pollSeconds` (default **30**):

1. Reload config if the file's mtime changed (hot reload, no restart needed).
2. If `settings.enforcement` is false, idle.
3. `platform.Sessions()` → group by username.
4. For each *enabled, managed* user with a live non-remote session:
   - `schedule.Evaluate(now)` → not active ⇒ clear our "should be locked" flag, continue.
   - An unexpired override exists ⇒ skip (and log once when it expires).
   - Session already reports `Locked == true` ⇒ skip (avoids re-showing the lock
     screen under the user's fingers).
   - Otherwise `platform.Lock()` and append an audit event.
5. Re-locking on every tick while unlocked is what defeats "child dismisses the
   lock screen": the worst case is 30 seconds of access.

The loop is a plain `for { select { case <-ticker.C: … } }` with a context, so
tests drive it with a manual ticker.

## 9. Web application

- `net/http` + Go 1.22 method-aware `ServeMux`, `html/template` and static
  assets embedded with `embed.FS`. No JS framework, no build step — the CSS is
  hand-written and the only JS is progressive enhancement for row deletion.
- **Transport:** HTTPS by default. On first start the daemon mints a self-signed
  ECDSA P-256 certificate valid for the machine's hostnames and LAN IPs into
  `/var/lib/plock/tls/`. `tls: off` is allowed for a trusted-LAN setup and prints
  a loud warning; `tls: custom` takes operator cert/key paths.
- **Auth:** a single admin account created by `plock init`. Passwords are stored
  as PBKDF2-SHA256, 600 000 iterations, 16-byte salt, 32-byte key, compared with
  `crypto/subtle.ConstantTimeCompare`. Optional PAM-backed login against real
  system accounts is left as a documented extension point (`auth.Authenticator`
  interface) because it would require cgo and a third-party module.

  Three rules here are load-bearing rather than stylistic:

  - *The encoding records the cost it was made at* (`pbkdf2-sha256$600000$salt$key`),
    so raising `DefaultIterations` in a later release does not lock the parent out
    of the one account that turns enforcement off.
  - *A cost floor of 100 000 is enforced where stored data is trusted, not in the
    decoder.* A config edited (or corrupted) to say `"10"` still verifies the
    correct password, so nothing would notice that the KDF had become something a
    laptop cracks in seconds. `Parse` decodes; `CheckStoredCost` refuses, and only
    the login path knows the bytes came from `/etc/plock`.
  - *The username is not a fast path.* It is compared after the password hash is
    computed, so a wrong username costs the same as a wrong password. Short-circuiting
    on it would answer in microseconds for one and a quarter second for the other,
    which is a free oracle for the half of the credential that is not secret.

  `plock init` refuses passwords under 12 characters — the KDF cost is what makes
  an offline crack expensive, and length is what makes the password not the first
  thing a guesser tries. It takes `--password-stdin` because a password in argv is
  readable by any user on the machine for the life of the process and lands in the
  shell history, and it refuses to replace an existing account without `--force`,
  so a provisioning script that runs twice cannot silently reset a password the
  parent has since written down.
- **Sessions:** 32 random bytes, stored in memory, `HttpOnly` + `SameSite=Strict`
  + `Secure` cookie, 12-hour sliding expiry, wiped on daemon restart. At most 32
  live sessions; the cap evicts whoever has gone longest without a request, so it
  cannot be used to lock the parent out by filling the store first. Changing the
  password revokes every session.
- **CSRF:** per-session token required on every state-changing request.
- **Throttling:** 10 attempts per 10 minutes per key, then an exponential penalty
  doubling from 1s to a 30s cap. A wrong login costs a quarter-second of PBKDF2
  before the throttle is consulted, so the KDF is the first line and the throttle
  exists to stop a flood from turning into a permanent lockout: above 200 failures
  across all keys in 10 minutes, everything is refused for 5 minutes. That global
  cooldown is a **fixed window measured from the moment it trips** — a failure
  arriving during it does not extend it, because an extensible cooldown lets the
  attack itself hold the parent out indefinitely.
- **Headers:** `Content-Security-Policy: default-src 'self'`,
  `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
  `Referrer-Policy: no-referrer`.

### Screens

| route | purpose |
|---|---|
| `/login` | admin sign-in |
| `/` | dashboard: every managed user, locked-out now?, which rule, ends when, next window, live sessions, last action |
| `/users` | human accounts detected on the machine; enable + attach a profile |
| `/profiles` | list, create, delete profiles |
| `/profiles/{id}` | edit a profile's rules (day grid, start, end, enabled) |
| `/users/{name}/override` | unlock for 15/30/60 min or until the end of the rule |
| `/audit` | tail of the audit log |
| `/settings` | change password, poll interval, enforcement master switch, TLS |

## 10. Safety and tamper-resistance

- Runs as root under systemd with `Restart=always`; nothing in the child's
  account can stop it without a root password.
- Config and audit log are `0600` root-owned; the store refuses to write if the
  file is a symlink or not owned by root.
- Every mutation is audited (who, what, when, old → new) to
  `/var/log/plock/audit.jsonl`, size-rotated.
- `settings.enforcement` is the kill switch a parent can flip; there is
  deliberately **no** kill switch reachable without authentication.
- The daemon never runs a shell as root for a lock action: lock commands run
  either as root against `loginctl`, or **downgraded** to the session user.
- Clock jumps and DST are handled by re-evaluating from wall-clock time on every
  tick rather than by scheduling timers.

## 11. Testing

- `schedule`: table-driven — overnight wrap, all-week, single day, DST spring
  forward and fall back, month/year boundaries, multiple overlapping rules.
- `config`: round-trip, atomic replace, permission enforcement, unknown-version
  refusal, concurrent read during write.
- `platform`: fake `exec.CommandContext` runner asserting exact argv and env;
  logind JSON parsing; backend fallback order.
- `enforce`: fake clock + fake platform; asserts lock-on-entry, re-lock while
  unlocked, no re-lock when `LockedHint` is true, override handling, release on
  window exit, hot reload.
- `auth`: hash/verify, wrong password, throttle behaviour, session expiry.
- `web`: `httptest` over every route — authz (401 vs 200), CSRF rejection,
  happy-path form posts, validation errors.
- CI (GitHub Actions): `gofmt -l`, `go vet ./...`, `go build ./...`,
  `go test -race ./...`, and a check that `go.mod` has no requirements.

## 12. Delivery

`make build` → `bin/plock`; `make install` drops the binary at
`/usr/local/bin/plock`, the systemd unit at `/etc/systemd/system/plock.service`,
and creates `/etc/plock/`. `plock init --username parent --password …`
bootstraps the admin account; `systemctl enable --now plock` starts it.
Ubuntu packaging (a `.deb`) is explicitly out of scope for v0.1; the install
script and the unit file are enough to run it.

## 13. Ticket breakdown

Tracked as GitHub issues, in build order — each one is independently reviewable
and lands on top of the previous. Issue numbers start at 2 because 1 was a
throwaway permission probe that GitHub would not let us delete.

| issue | ticket |
|---|---|
| #2  | Go project scaffolding, CI, zero-dependency guard |
| #3  | Config schema and atomic store |
| #4  | Schedule engine (lockout-period evaluation) |
| #5  | Platform abstraction and systemd-logind backend |
| #6  | KDE Plasma backend and the backend chain |
| #7  | Enforcement engine (30-second re-lock loop) |
| #8  | Admin auth: PBKDF2, sessions, throttling, `plock init` |
| #9  | Web server core: TLS bootstrap, routing, middleware |
| #10 | Web UI: dashboard, users, profiles, rules, overrides, audit |
| #11 | CLI: status, lock, unlock, doctor, JSON output |
| #12 | systemd unit, install script, operator documentation |
| #13 | Hardening pass, end-to-end smoke test, v0.1.0 release |
