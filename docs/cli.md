# `plock` — the command line

Every administrative action is available on the command line as well as in the web UI,
because the machine that needs configuring is often a headless box reached over SSH. The
two write the same config file and the same fields, so they cannot disagree about what a
policy is.

    plock <command> [flags]
    plock help <command>

Config defaults to `/etc/plock/config.json` and every command takes `--config PATH`.
Commands that change nothing are marked *reads only*; the rest write the config file
atomically and are picked up by the daemon on its next tick.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | operational failure — the config could not be read, a backend was unreachable, the named user does not exist |
| 2 | usage error — bad flag, missing argument, unknown command |

The distinction is the point of the table. A script that treats 2 as "the machine is
broken" will page on a typo; a script that treats 1 as "the user is not locked out" will
report freedom where there is none.

## `--json`

`status`, `lock`, `profile list`, `profile show`, `user list`, `override list` and
`doctor` accept `--json`.

- **Success**: one JSON document on stdout, nothing on stderr, exit 0.
- **Failure**: one JSON document on **stderr**, **nothing on stdout**, exit 1.

      {"error": "config_unreadable", "message": "config: /etc/plock/config.json: unsupported config schema version: found 0, this build reads only 1"}

Failure goes on stderr and leaves stdout empty so that `plock status --json 2>&1 | jq`
sees exactly one document. Putting the error on stdout instead interleaves a JSON object
with a human line in the merged stream, and putting it on both gives `jq` a second
document to fail on. The exit code is the primary signal; an empty stdout is the
secondary one, and it means *there is no report*, which is true.

Usage errors (exit 2) stay plain text even with `--json`: they mean the command line was
wrong, which is a human reading the usage text, and 2 already carries that.

### Failure codes

`error` is a stable key, not a sentence — branch on it and show `message`. Renaming one is
a breaking change.

| Code | Means |
|---|---|
| `config_unreadable` | the config file could not be read or decoded |
| `config_unstatable` | the config file's metadata could not be read |
| `config_unwritable` | a change could not be saved |
| `no_backend` | no configured lock backend could be built |
| `sessions_failed` | the session list could not be read at all; a partly readable one is used and the gap is reported (#168) |
| `no_such_user` | the named account is not managed by plock |
| `no_lockable_session` | the account is managed but has no lockable session here |
| `lock_failed` | the lock was attempted and nothing reached the desktop |
| `no_such_profile` | the named profile does not exist |
| `output_failed` | the report could not be written |

Field names are as stable as these codes and are listed per command below.

## `plock status` — *reads only*

    plock status [--json] [--config PATH]

What the daemon would do right now: enforcement on or off, which backends this machine
offers, and for every managed user whether they are inside a lockout window, which rule
says so, when it ends, and when the next one opens.

| Field | Type | Notes |
|---|---|---|
| `enforcement` | bool | the master switch |
| `pollSeconds` | int | tick interval |
| `configPath` | string | the file actually read |
| `configModifiedAt` | string | RFC 3339, UTC |
| `backends` | []string | configured order |
| `unknownBackends` | []string | names that resolved to nothing; absent when there are none |
| `daemon` | object | where the enforcement engine is actually running — the four states below |
| `daemon.systemd` | bool | false = systemctl could not be executed, which is not the same finding as "not running" |
| `daemon.running` | bool | the unit has a main process right now |
| `daemon.inUserSlice` | bool | true = the engine is inside somebody's login session, and declines that account's sessions |
| `daemon.uid` | int | the account whose session the engine is inside; absent for a system unit |
| `daemon.locationKnown` | bool | false = it is running and this command could not read its cgroup, so nothing is marked |
| `daemon.askError` | string | absent when systemd answered. Non-empty = systemd is installed and refused, so **neither `running` value is an answer** |
| `warnings` | []string | absent when there are none |
| `users` | []object | one per managed user |
| `users[].username` | string | |
| `users[].profile` | string | assigned profile id |
| `users[].enabled` | bool | false = the loud off switch |
| `users[].lockout` | bool | inside a window right now |
| `users[].rule` | string | the rule that says so; absent when there is no window |
| `users[].ruleName` | string | |
| `users[].startedAt` | string | RFC 3339 |
| `users[].endsAt` | string | RFC 3339 — **render this, never re-derive from the rule's clock text** |
| `users[].nextStart` | string | RFC 3339 |
| `users[].override` | string | the override's reason; absent when none is on file |
| `users[].overrideUntil` | string | RFC 3339 |
| `users[].degraded` | []string | the schedule was widened, not honoured — see below |
| `sessions` | []object | logins this machine can see |
| `sessions[].username` | string | |
| `sessions[].session` | string | logind session id |
| `sessions[].uid` | int | |
| `sessions[].type` | string | `x11`, `wayland`, `tty`, … |
| `sessions[].seat` | string | absent for a session with no seat |
| `sessions[].lockable` | bool | false for a `class=manager` session |
| `sessions[].locked` | bool | logind's `LockedHint` |
| `sessions[].declinedByDaemon` | bool | the engine will not lock this one — absent unless it is |

`daemon` is why `declinedByDaemon` means anything. `plock status` runs as the person typing
it, so the engine's location cannot come from the shell's surroundings: it is read from the
daemon's own cgroup, the same file the engine reads about itself. Four states are kept
apart because each supports a different sentence, and collapsing any pair makes the report
assert what the probe did not establish:

| `daemon` | What the report says |
|---|---|
| `systemd: true, askError` set, `running: false` | systemd would not say. The report says so and marks nothing — this is a container that was not booted into systemd, or a stale `SYSTEMD_ADDRESS` |
| `systemd: true, running: false` | the engine is not running, so nothing is being declined — and nothing will lock |
| `running: true, locationKnown: true, inUserSlice: true` | the engine declines the sessions of `uid`, including the one it is running in |
| `running: true, locationKnown: false` | the engine is running and this command cannot tell whose slice it is in; **no session is marked**, and a warning says so |

The last row is the one to read carefully. An unmarked session list is not evidence that
the engine declines nobody — it is evidence that this command could not look.

`askError` is the difference between an absent answer and a refused one, and it is decided by
`errors.Is` against the runner's sentinel rather than by matching error prose: the runner
rewrites the standard library's wording on purpose, so a sentence match silently stopped
recognising "there is no systemctl here" and reported a machine with no init system as a
machine whose service is stopped.

`users[].degraded` is the one to read before believing `lockout`. A rule plock could not
read cleanly is enforced at its widest reading rather than dropped — so a parent is never
silently released by a typo — but the parent is entitled to know the window is a
safe superset rather than the one they wrote.

`endsAt` deserves its own warning: a window that overlaps a DST fold is held for its
written span plus the whole width of the fold, so the clock can read *before* the written
start while the account is still held.

## `plock lock`

    plock lock <user> [--all] [--json] [--config PATH]

Locks the session now, ignoring the schedule. Writes nothing: the account returns to
whatever its schedule says on the daemon's next tick. `--all` takes every managed user who
has a lockable session, and is what to run after changing a backend.

A machine answers in three different ways and they are three different sentences:

| Outcome | Means |
|---|---|
| `locked` | a backend confirmed the screen is locked |
| `unconfirmed` | the lock was asked for and nothing could prove it took |
| `failed` | nothing reached the desktop at all |

`unconfirmed` is not `failed`. On Plasma, `loginctl lock-session` exits 0 while Plasma
ignores the signal, so a zero status reports that a lock was *requested*; the desktop
backends are what actually prove it.

A session listing that could only be partly read is used as far as it reaches, and the gap is
said on stderr (#168): the warning means the census was short, so a session missing from it
has not been found to be absent. `sessions_failed` is reserved for a listing that read
nothing at all — and for the one case a short listing cannot answer, where the account asked
about is the row that was lost. "has no session on this machine" is a logout, and a census
that lost a row is not evidence for one.

| Field | Type |
|---|---|
| `target` | string — the username, or `all` |
| `locked`, `unconfirmed`, `refused` | int |
| `sessions` | []object |
| `sessions[].session` | string |
| `sessions[].username` | string |
| `sessions[].locked` | bool |
| `sessions[].unconfirmed` | bool |
| `sessions[].detail` | string — why not, when it did not lock |

`refused` counts sessions plock will not attempt, such as a `class=manager` session. They
are counted rather than skipped silently, because "locked 0 of 1" with no reason reads as
a broken backend.

## `plock unlock`

    plock unlock <user> --minutes N --reason TEXT [--by NAME] [--config PATH]

Grants temporary freedom from every window until N minutes from now, replacing any
override already held. Same action and same field as the web UI's unlock button.

`--minutes` is required, 1–1440. `--reason` is required and must not be blank — the reason
is the whole audit value of an override. `--by` defaults to the invoking user.

The account returns to its schedule on its own; there is nothing to undo. To end an
override early, `plock override clear`.

## `plock override`

    plock override list [--json] [--config PATH]
    plock override clear <user> [--config PATH]

`list` shows every override in the config, active or not, with who granted it and why.
An expired override is still listed, because "why is this account free" is a question about
the file as well as about right now.

| Field | Type |
|---|---|
| `overrides` | []object |
| `overrides[].username` | string |
| `overrides[].until` | string |
| `overrides[].reason` | string |
| `overrides[].by` | string |
| `overrides[].active` | bool |

## `plock profile`

    plock profile list [--json]
    plock profile show <id> [--json]
    plock profile add <id> [--name TEXT] [--timezone TZ] [--copy-from <id>]
    plock profile edit <id> [--name TEXT] [--timezone TZ]
    plock profile rm <id>
    plock profile rule add <id> <rule-id> --days LIST --start HH:MM --end HH:MM
                             [--name TEXT] [--all-day] [--disable]
    plock profile rule rm <id> <rule-id>
    plock profile rule enable|disable <id> <rule-id>

Profiles are the schedules; users are assigned to one.

`--timezone` must be an IANA zone the system knows, such as `Europe/London`. The zone is
per-profile so a household in two zones is expressed rather than worked around.

`--days` is a comma list of `mon,tue,wed,thu,fri,sat,sun`.

`--start`/`--end` are local wall times, `HH:MM`. A window whose end is earlier than its
start runs overnight. An ambiguous wall time — repeated at a fall-back, nonexistent at a
spring-forward — resolves deterministically toward holding the child out: a curfew ending
at a repeated hour holds until the clock jumps rather than releasing early.

`--all-day` declares a rule that holds all day on the named days, with no clock times.
It is distinct from a rule that plock had to *widen* because it could not read one: a
declared all-day rule is what the parent asked for and is never reported as degraded.

`profile list`:

| Field | Type |
|---|---|
| `profiles` | []object |
| `profiles[].id` | string |
| `profiles[].name` | string |
| `profiles[].rules` | int |
| `profiles[].assigned` | int — users pointing here; decides whether `rm` can succeed |

`profile show` emits the profile itself: `id`, `name`, `timezone`, and `rules[]` with
`id`, `name`, `days[]`, `start`, `end`, `allDay`, `enabled`.

## `plock user`

    plock user list [--json]
    plock user assign <user> <profile>
    plock user enable <user>
    plock user disable <user>
    plock user rm <user>

Assigning replaces the previous assignment; the old profile stays in the file until
`profile rm` removes it. Disabling stops an account being locked out at all — the loud off
switch, and `status` reports it as such.

| Field | Type |
|---|---|
| `users` | []object |
| `users[].username` | string |
| `users[].profile` | string |
| `users[].enabled` | bool |
| `users[].profileExists` | bool — false is a dangling assignment |

## `plock doctor` — *reads only, unless `--try`*

    plock doctor [--json] [--try <user>] [--config PATH]

Answers "is this working, and if not what do I do". Exits 1 when a check fails, and
`--json` names which in `failed`.

| Flag | Means |
|---|---|
| `--json` | the report as one document on stdout |
| `--try <user>` | actually lock that user and report which backend won |
| `--config PATH` | config file |

`--try` is the one flag that changes the machine, so it cannot be combined with `--json`: a
script must never be able to lock a screen as a side effect of asking a question.

| Field | Type |
|---|---|
| `checks` | []object, in the order they ran |
| `checks[].name` | string — the stable key to branch on |
| `checks[].status` | string — `ok`, `warn`, `fail`, `skip` |
| `checks[].detail` | string |
| `checks[].fix` | string — what to do, when there is something to do |
| `failed` | []string — the names that returned `fail` |
| `warned` | []string |
| `generatedAt` | string — RFC 3339, so a saved report can be recognised as stale |

`skip` means the check could not run here, which is not the same as passing. The two are
kept apart because "we did not look" reading as "we looked and it was fine" is the exact
failure mode of a diagnostic tool.

`failed` duplicates names out of `checks` deliberately: every consumer re-filtering the
array is how "name the failing check" ends up implemented five different ways.

### Check names

| Name | Asks |
|---|---|
| `root` | is the process root |
| `config.file` | is the config present, a regular file, and 0600 |
| `config.loads` | does it decode at this schema version |
| `config.writable` | could the daemon save it |
| `backends` | is each configured backend usable here, and which would a lock try first |
| `backends.unknown` | names in `lockBackends` that resolve to nothing |
| `daemon` | is the service running and enabled |
| `daemon.slice` | will any account be declined — the running service's session if there is one, otherwise this shell's |
| `enforcement` | is enforcement on, and is the poll interval sane |
| `tls` | is the certificate present and unexpired |
| `unenforced` | is a managed user inside a window with a session that is not locked |

`unenforced` is the check that answers "it is not working" when every individual component
reports itself healthy.

`daemon.slice` answers from the running service when there is one — its `MainPID`'s cgroup,
the same read `plock status` marks its rows with — and from this shell only when no service
is running, because then the daemon in question is the one about to be started from here.
`is-active` on its own does not establish whose session the unit is in, so an active service
whose cgroup cannot be read is reported as *cannot say*, not as the reassuring answer.

## `plock init`

    plock init [--username NAME] (--password PASS | --password-stdin) [--force]
               [--config PATH] [--state-dir PATH]

Creates or replaces the single administrator account, non-interactively.

`--password-stdin` is the form to use in a script: a password in argv is readable by every
user on the machine for as long as the process lives, via `/proc/<pid>/cmdline`.

The password is stored only as a PBKDF2-SHA256 verifier and never in any recoverable form.
`--force` replaces an existing administrator; without it, `init` refuses to overwrite one.

## `plock serve`

    plock serve [--config PATH]

Runs the daemon: the enforcement loop and the admin web UI. Root. Reads the same config the
CLI edits, so a `plock profile` change takes effect on the next tick with no restart.

## `plock version`

Prints the version, the commit, and the Go toolchain the binary was built with.

## Positional arguments

`lock`, `unlock`, `override clear`, `profile show`, `user assign` and the rest take the
account or profile name **first**, before the flags:

    plock unlock casey --minutes 20 --reason "film night"

Go's `flag` package stops parsing at the first non-flag argument, so the other order
silently parses zero flags. The name is lifted out before parsing, and it is lifted with
the declared flag types in hand: `--reason film night` is ambiguous without them, and
naively taking the last token as the positional would record `film` as the reason while
using `night` as the account name. An unknown flag is assumed to take a value, so a flag's
value can never be mistaken for the name.

`--` ends the flags and protects the name rather than suppressing it: the first token after
it is the account name, so an account literally called `-x` is reachable —
`plock unlock --minutes 5 --reason "late" -- -x`. Everything after `--` stops being a flag,
including a token that spells one, so put your flags *before* it. A management command that
finds an argument it did not claim refuses instead of ignoring it, because the alternative
is `plock user rm -- bob --config /tmp/test.json` dropping `--config` and deleting the user
from `/etc/plock/config.json` while the operator believes they named a scratch file.
