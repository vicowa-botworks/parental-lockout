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

Under construction. The command surface below is real; the behaviour lands ticket
by ticket in the [issue queue](https://github.com/vicowa-botworks/parental-lockout/issues).

```
plock serve     run the enforcement daemon and the admin web UI
plock init      create or replace the single admin account
plock status    show enforcement state and per-user lockout status
plock lock      lock a user's session immediately, ignoring the schedule
plock unlock    grant a temporary override that suspends enforcement
plock doctor    diagnose whether locking actually works on this machine
plock version   print version information
```

## Build

Go 1.27 or newer, and nothing else — the project has **zero third-party
dependencies** and CI fails if `go.mod` ever grows one.

```sh
make build     # -> bin/plock
make verify    # gofmt, vet, build, race tests, dependency guard
```

## Design

- [docs/PLAN.md](docs/PLAN.md) — architecture, data model, enforcement loop,
  platform abstraction, security posture.
- Target platform is KDE Plasma on Ubuntu; the locking mechanism sits behind an
  interface so GNOME and others can follow.

## License

MIT
