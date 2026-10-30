# parental-lockout

`plock` keeps a family computer unusable during bedtime, school hours and any
other window you decide it should be.

You define **lockout periods** per user — "Sun–Thu 21:00 → 06:30", "weekdays
08:15 → 13:30" — and `plock` makes sure the screen stays locked during them. If
the child dismisses the lock screen, it is locked again within 30 seconds, for as
long as the window lasts. A parent administers everything from a web app on the
local network, from the sofa or their phone.

## What it is not

This is **not** a time budget. There are no "40 minutes per day" quotas and no
minutes that accumulate or run out — the model is Google Family Link's sleep
time and school time, not an allowance you can spend down. See
[docs/PLAN.md](docs/PLAN.md) for the reasoning.

## Status

Working, and installed as a service today: the enforcement daemon, the platform backends,
the admin web UI, and the whole command surface — `serve`, `init`, `status`, `lock`,
`unlock`, `override`, `profile`, `user`, `doctor`, `version`. Every one of them does
something; none is a stub.

Other pieces still land ticket by ticket in the
[issue queue](https://github.com/vicowa-botworks/parental-lockout/issues), and
[docs/security.md](docs/security.md) §8 lists what is deliberately left undefended.

## Install

Linux with systemd. Both desktop backends are measured on real hardware — KDE Plasma 6.6.6
and GNOME 50.1 — and [docs/platforms.md](docs/platforms.md) carries each measurement with the
command that reproduces it. The difference that matters to an operator is corroboration:
Plasma confirms a lock on a second signal — a live `kscreenlocker_greet` parented to ksmserver
— which the session user cannot produce by writing a property, and GNOME has none, so on a
GNOME-only machine a lock is confirmed on the tick that moves logind's hint and reported
unconfirmed on every tick after it. You need root on the machine, and one other device on the
same network to act as the parent's browser; a phone is fine.

### From a package

Every release carries a `.deb` and an `.rpm` for amd64 and arm64 on the
[releases page](https://github.com/vicowa-botworks/parental-lockout/releases), with one
`SHA256SUMS` over all four:

```sh
sha256sum -c SHA256SUMS                   # before you install any of them
sudo dpkg -i plock_<version>_amd64.deb    # Debian, Ubuntu
sudo rpm -i plock-<version>-1.x86_64.rpm  # Fedora, RHEL
```

A package is built by staging with [`deploy/install.sh`](deploy/install.sh) rather than by a
second recipe, so it puts the same files in the same places with the same modes as the source
install below. That is not a claim the build makes about itself: `make package-verify` installs
both formats into throwaway Debian and Fedora images and compares what each one did — same
binary, same unit, same paths, same modes, nothing enabled — because two formats are exactly how
one program ends up behaving differently depending on whether the operator typed `apt` or `dnf`.

A package also starts and enables nothing and never writes the config file, for the same reasons
as below. What differs is only the word for "and delete my data":

```sh
sudo dpkg -r plock    # leaves the policy, the key and the audit trail alone
sudo dpkg -P plock    # deletes the policy and the key; keeps the audit trail
sudo rpm -e plock     # leaves everything; rpm has no purge
```

No package operation deletes the audit trail. `deploy/install.sh --uninstall --purge` does, and
says so before you run it — that is a decision to make once, in front of you, rather than one
that fits inside a word in a script.

### From source

```sh
git clone https://github.com/vicowa-botworks/parental-lockout
cd parental-lockout
sudo make install
```

`make install` runs the repo's gate (`make verify`) first and refuses to install a build that
fails it — installing an unverified build means the machine that is about to enforce tonight
runs a binary the repo would have rejected, and the failure it produces, a screen that stays
unlocked, is silent until morning. It then hands off to
[`deploy/install.sh`](deploy/install.sh), which drops:

| path | mode | what it is |
|---|---|---|
| `/usr/local/bin/plock` | 0755 | the one binary |
| `/etc/systemd/system/plock.service` | 0644 | the unit |
| `/etc/plock/` | 0700 | where the policy file goes |
| `/var/lib/plock/` | 0700 | the TLS certificate and its key |
| `/var/log/plock/` | 0750 | the audit trail |
| `/usr/share/doc/plock/docs/` | 0644 | these docs, for `systemctl status` |

It **starts and enables nothing**, and it **never writes the config file**. Both are yours to
decide in the first run below: a root daemon that locks other people's sessions should begin on
the day you decide, not the day somebody typed `make install`, and a policy file installed with
a default credential in it is worse than a missing one with a clear error.

To take it back off:

```sh
sudo make uninstall            # leaves /etc/plock, /var/log/plock and /var/lib/plock alone
sudo make uninstall PURGE=1    # deletes them, and nothing reconstructs the trail afterwards
```

The policy and the audit trail are your family's records, not the program's scratch space, so a
refresh install should not need the rules rewritten.

## Usage

### First run

Four commands, in this order. [docs/setup.md](docs/setup.md) walks the same path end to end
with the real output of every step pasted in — the certificate, reaching it from another
device, and the checks that prove the hardening in [`deploy/plock.service`](deploy/plock.service)
costs nothing on your desktop.

```sh
sudo plock init --username parent --password-stdin   # 1. the single admin account
sudo plock doctor                                    # 2. can this machine lock at all?
sudo systemctl enable --now plock                    # 3. start enforcing
journalctl -u plock -f                               # 4. look for "enforce: running"
```

Use `--password-stdin`, not `--password`: a password in argv is readable by every user on the
machine for as long as the process runs and stays in your shell history afterwards. It is
stored only as a PBKDF2-SHA256 verifier, and under 12 characters it is refused — this account
unlocks every lockout on the machine over the local network, so it is not a password to reuse
from somewhere less important. Re-running `init` without `--force` refuses to overwrite an
existing administrator, so a provisioning script that runs twice cannot silently reset the
password out from under the parent who set it.

Run `doctor` before you enable anything. It asks each configured backend whether it is usable
and, critically, whether a lock can be *confirmed* — and confirmation is the machine-specific
part: on Plasma it depends on ksmserver being alive and on `kscreenlocker_greet` being parented
by it, which is exactly what a distribution update changes. If `doctor` says nothing is usable,
`plock serve` refuses to start, and that refusal is correct behaviour rather than a bug to work
around.

`enforce: running` is the journal line to look for; it exists because its absence used to be
invisible — `serve` started the web UI, logged a cheerful line about listening, and enforced
nothing. Read the whole startup block rather than scanning for that one line: it names the
config file it actually opened, and `--config` at the wrong file is the one mistake that reads
as success, because the wrong document still loads and still validates; and it says which
backends answered the probe, which is the difference between a daemon that enforces and one
that merely serves the UI.

What is *not* in the journal is a schedule the evaluator could read but not obey. A profile
whose timezone is `Local` — which validates, and then moves every window whenever the host's
zone moves — is reported per account the tick its complaint set changes, in the audit trail as
`schedule.degraded` and in `plock status`'s `degraded` field, so `plock status` is the second
thing to run after the first start. A timezone that does not resolve at all is louder: the
command refuses to write it, `serve` refuses to load a file containing it, and `plock status`
names it in the same field.

### The web UI

`https://ADDRESS:8443/`. The default `listen` is `0.0.0.0:8443`, so the dashboard is already
reachable from the sofa. Narrowing it to `127.0.0.1:8443` is an edit to `settings.listen` in
`/etc/plock/config.json`, not a field on the settings page: what the daemon binds needs its own
way of saying "you have just cut yourself off", and it does not have one yet. TLS defaults to
`"auto"`, which mints a self-signed certificate on first start, so the browser warns once —
[docs/setup.md](docs/setup.md) §5 has the three ways to stop that, including bringing your own
from a private CA.

| page | what you do there |
|---|---|
| `/` | who is locked out right now, which rule says so, and when it ends — and the four actions: lock now, check the backends against one account, grant an override for 15/30/60 minutes or until the window ends, take it away |
| `/profiles` | create a profile, rename it, set its timezone, add a window, copy or delete one |
| `/users` | assign an account to a profile, switch enforcement off for one account, stop managing it |
| `/settings` | the master switch, the poll interval, TLS and its certificate files, the admin password |
| `/audit` | what enforcement did, and who overrode it |

Two things the pages stop short of: enabling or disabling a single rule is CLI-only
(`plock profile rule enable|disable`), as are the listen address and the backend list — the
settings page says so out loud rather than leaving you to hunt for the field.

No firewall rule is opened for you. If the dashboard is unreachable from the other device while
`curl` on the machine works, that is `ufw`/`firewalld` and it is your call — publishing a root
admin interface on the LAN is a choice worth making consciously rather than having a script make
it for you.

### The command line

Every administrative action is on the CLI as well, because the machine that needs configuring
is often a headless box reached over SSH. The two write the same fields in the same file, so
they cannot disagree about what a policy is. [docs/cli.md](docs/cli.md) documents every flag,
every JSON field and every failure code.

```
plock serve      run the daemon: the enforcement loop and the admin UI
plock init       create or replace the single admin account
plock status     what the daemon would do right now              (reads only)
plock lock       lock a session now, ignoring the schedule
plock unlock     grant a timed override, with a reason
plock override   list or clear overrides
plock profile    the schedules: list, show, add, edit, rm, rule …
plock user       the managed accounts: list, assign, enable, disable, rm
plock doctor     diagnose this machine                     (reads only, unless --try)
plock version    version, commit, and the moment the binary was built
```

A policy is profiles and rules, then accounts assigned to them:

```sh
sudo plock profile add schoolnight --name "School night" --timezone Europe/London
sudo plock profile rule add schoolnight bedtime --days mon,tue,wed,thu,sun \
     --start 21:00 --end 06:30 --name Bedtime
sudo plock profile rule add schoolnight school --days mon,tue,wed,thu,fri \
     --start 08:15 --end 13:30 --name "School hours"
sudo plock profile rule add schoolnight weekend --days sat,sun --all-day
sudo plock user assign casey schoolnight
```

Four things about those rules that are not the obvious reading:

- **An end earlier than its start runs overnight**, and belongs to the *starting* day — so
  `21:00–06:30` on `sun` is Sunday evening into Monday morning. `end == start` enforces nothing
  and is refused; it is deliberately not read as 24 hours, because that is the shape a parent
  makes by typing the same time into both boxes.
- **`--all-day` is the rule with no minute free.** `00:00`–`23:59` is not all day: it is 1439
  minutes, and the minute it releases is two 30-second re-lock ticks of free machine on a
  schedule the parent read as all day.
- **Timezone is per profile**, so a household in two zones is expressed rather than worked
  around. A zone that does not resolve is refused by the command rather than falling back to
  UTC at runtime, because a silently-wrong zone moves every window in the profile.
- **A wall time that a DST change makes ambiguous resolves toward holding the child out** — a
  curfew ending at a repeated hour holds until the clock jumps rather than releasing early.

Day to day:

```sh
sudo plock status                                  # anyone inside a window, and until when
sudo plock lock casey                              # bedtime is now
sudo plock unlock casey --minutes 20 --reason "film night"
sudo plock override list                           # who is free, until when, and why
sudo plock doctor --try casey                      # lock for real and say which backend won
```

Every one of those takes root, including the two that change nothing: the policy is `0600` root
because it holds the admin credential, so a command that reads it cannot be run by the account
it is about to lock out. `--config PATH` points any command at a different file, which is what
makes a scratch policy testable.

An override replaces any the account already holds and expires on its own — there is nothing to
undo. `--reason` is required and cannot be blank, because the reason is the whole audit value of
an override.

Three things worth knowing before you script any of it:

- **`locked`, `unconfirmed` and `failed` are three different sentences.** On Plasma,
  `loginctl lock-session` exits 0 while Plasma ignores the signal, so a zero status proves only
  that a lock was *requested*; the desktop backends are what actually prove it.
- **Exit codes are 0 success, 1 operational failure, 2 usage error.** A script that treats 2 as
  "the machine is broken" pages on a typo; one that treats 1 as "the user is not locked out"
  reports freedom where there is none.
- **`--json`** is accepted by `status`, `lock`, `override list`, `profile list`, `profile show`,
  `user list` and `doctor`. Success is one document on stdout; failure is one document on
  **stderr** with stdout left empty, so `plock status --json 2>&1 | jq` sees exactly one
  document either way.

Put the account or profile name **before** the flags, which is the shape every example here
uses:

```sh
sudo plock unlock casey --minutes 20 --reason "film night"
```

The name is lifted out before the flags are parsed, and it is lifted with the declared flag
types in hand, so the other order works as well. What does not work is an unquoted multi-word
value: `--reason film night` leaves `night` unclaimed and is refused with the sentence that
names the mistake (`if --reason has a space in it, quote the whole value`) rather than recording
`film` as the reason. `--` ends the flags and protects the name rather than suppressing it — the
first token after it is the account, so an account literally called `-x` is reachable — which
means your own flags go *before* it. Anything a command cannot claim is refused rather than
ignored, so a mistyped `--config` cannot quietly aim the command at
`/etc/plock/config.json` while you believe you named a scratch file.

### Building from source

Go 1.27 or newer, and nothing else — the project has **zero third-party dependencies** and CI
fails if `go.mod` ever grows one.

```sh
make build      # -> bin/plock
make verify     # gofmt, vet, build, race tests, dependency guard
make mutate     # put each known bug back and insist the suite notices
make run-dev    # the whole loop, unprivileged, against a throwaway config
```

`make run-dev` runs the enforcement loop and the admin UI with no desktop session and no root,
against a throwaway config under a dev root that uses the `command` backend. That backend really
execs, so the loop, the config, the audit writer and the fallback chain under test are the real
ones and only the thing that would touch a screen is a shell script. It prints the dev root's
paths, the URL, the account enforcement is aimed at, and a generated admin password. That account
is **you** unless you set `PLOCK_DEV_TARGET` — safe only because the locker is a script writing a
log line, and the script says so on the screen — and Ctrl-C stops it. Nothing outside the dev root
is created or changed.

## What this does not defend against

The full threat model, with the test behind every defense, is
[docs/security.md](docs/security.md). Its §8 lists what is deliberately not defended and §9
ranks the residual risk. The four that change how you should think about the product:

- **A child with root on the machine has won.** The daemon runs as root precisely so a child
  cannot disable it from their own session; that same root is what they would be escalating
  to. Everything here assumes root is not already theirs.
- **A boot from a USB stick bypasses all of it.** Nothing here is disk encryption. If the
  machine can be rebooted from external media, the policy is not enforcing — it is a file on
  a disk that somebody else gets to read.
- **Holding the power button defeats a lockout but not the policy.** Enforcement resumes when
  the daemon comes back up, and the window is computed from the clock, not from a countdown
  that was interrupted. The audit trail records the gap.
- **`plock` locks sessions; it does not block logins.** A locked screen is the mechanism.
  Somebody at the greeter can still start a *different* session, and a machine that sits at
  the greeter with no session to lock is a machine where there is nothing to enforce against.
  This is a bedtime lock, not a login control.

## Design

- [docs/PLAN.md](docs/PLAN.md) — architecture, data model, enforcement loop,
  platform abstraction, security posture.
- [docs/cli.md](docs/cli.md) — every command, flag, JSON field and failure code.
- [docs/setup.md](docs/setup.md) — install, first run, the certificate, and verifying the
  hardening on a live machine.
- [docs/security.md](docs/security.md) — the threat model: who the adversary is, what they
  control, every defense with the test behind it, and an explicit list of what is *not* defended.
- [docs/platforms.md](docs/platforms.md) — what was measured on real hardware, and where the
  design's platform claims came from.
- Target platform is KDE Plasma on Ubuntu, with GNOME measured on real hardware as well; the
  locking mechanism sits behind an interface, and the difference between the two — whether a
  confirmation rests on something the controlled user cannot write — is measured, not assumed.

## License

MIT
