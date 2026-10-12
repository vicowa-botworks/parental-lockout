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
          "start": "08:15", "end": "13:30", "enabled": true }
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

- **A rule is a wall-clock window on a set of weekdays.** `end <= start` means
  the window crosses midnight and belongs to the *starting* day. `21:00–06:30` on
  `[…,"sun"]` therefore means Sunday evening into Monday morning.
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
    NextStart  time.Time // next window opening, if Active == false
}
```

Algorithm: for each enabled rule, test two candidate windows — the one starting
"today" (if today's weekday is in `days`) and the one that started "yesterday"
and is still running (if yesterday's weekday is in `days`). Take the earliest
`EndsAt` among active windows. `NextStart` scans the next 8 days of candidate
window openings. DST is handled by doing all arithmetic in the profile's
`*time.Location` with wall-clock `time.Date`, never by adding durations.

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
  as PBKDF2-SHA256, 600 000 iterations, 16-byte salt, compared with
  `crypto/subtle.ConstantTimeCompare`. Optional PAM-backed login against real
  system accounts is left as a documented extension point (`auth.Authenticator`
  interface) because it would require cgo and a third-party module.
- **Sessions:** 32 random bytes, stored in memory, `HttpOnly` + `SameSite=Strict`
  + `Secure` cookie, 12-hour sliding expiry, wiped on daemon restart.
- **CSRF:** per-session token required on every state-changing request.
- **Throttling:** per-IP token bucket plus an exponential penalty after failed
  logins, so a LAN brute-force is not viable.
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
