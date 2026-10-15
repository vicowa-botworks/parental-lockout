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
the admin web UI, and the CLI's diagnosis and bootstrap commands. Other pieces land ticket by
ticket in the [issue queue](https://github.com/vicowa-botworks/parental-lockout/issues).

To put it on a machine: **[docs/setup.md](docs/setup.md)** — build, install, policy,
certificate, and the checks that prove the hardening in
[`deploy/plock.service`](deploy/plock.service) actually costs nothing on your desktop.

### The command surface, honestly split

`plock` prints every subcommand it knows about, including the ones that are still stubs, so
the list below is not the interesting part — which of them do something is.

Real, and covered by tests:

```
plock serve     run the enforcement daemon and the admin web UI
plock init      create or replace the single admin account
plock doctor    diagnose whether locking actually works on this machine
plock version   print version information
```

Accepted by the binary and currently a stub that prints `plock <cmd>: not implemented yet`:

```
plock status    show enforcement state and per-user lockout status
plock lock      lock a user's session immediately, ignoring the schedule
plock unlock    grant a temporary override that suspends enforcement
```

Those three are ticket #11. They are listed by `plock` with no flags of their own declared,
so `plock lock -h` shows the intended usage and the not-implemented line together, and each
stub exits non-zero — a script will not mistake one for success.

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

## Build

Go 1.27 or newer, and nothing else — the project has **zero third-party
dependencies** and CI fails if `go.mod` ever grows one.

```sh
make build     # -> bin/plock
make verify    # gofmt, vet, build, race tests, dependency guard
make mutate    # put each known bug back and insist the suite notices
```

Installing is `make install`, which runs `make verify` first and then hands off to
[`deploy/install.sh`](deploy/install.sh). It builds, installs and prints
`systemctl enable --now plock` — it never runs it. See
[docs/setup.md](docs/setup.md) for what to do between those two commands.

## Design

- [docs/PLAN.md](docs/PLAN.md) — architecture, data model, enforcement loop,
  platform abstraction, security posture.
- [docs/security.md](docs/security.md) — the threat model: who the adversary is, what they
  control, every defense with the test behind it, and an explicit list of what is *not* defended.
- [docs/platforms.md](docs/platforms.md) — what was measured on real hardware, and where the
  design's platform claims came from.
- Target platform is KDE Plasma on Ubuntu; the locking mechanism sits behind an
  interface so GNOME and others can follow.

## License

MIT
