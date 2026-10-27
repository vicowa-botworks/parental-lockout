# Fixtures

`list-sessions.json`, `list-sessions.table`, `list-sessions.pretty.json`,
`show-session-desktop.txt` and `show-session-manager.txt` are captured verbatim from
a live KDE session on the target platform (Ubuntu, systemd 259.5-0ubuntu3.4, uid
1000, X11 on seat0) with:

    loginctl list-sessions --json=short      -> list-sessions.json
    loginctl list-sessions --output=json     -> list-sessions.table
    loginctl list-sessions --json=pretty     -> list-sessions.pretty.json
    loginctl show-session 3 -p Name -p Type -p State -p Remote -p UID -p LockedHint
    loginctl show-session 4 -p Name -p Type -p State -p Remote -p UID -p LockedHint

`list-sessions.table` is not a mistake: `--output=json` is accepted and ignored, and
this is the plain table it prints with exit status 0. It is the fixture the table
fallback is tested against precisely because a real binary produced it.

`show-session-desktop.txt` shows the property order is neither the requested order
nor alphabetical — `LockedHint` was asked for first and printed last.

The machine had exactly two sessions, one uid. It has no remote session and nothing
in `closing` state, so the fixtures cannot exercise those filter cases; the tests
that do are built from synthetic blocks and say so where they are defined.

## The 259.9 capture

`list-sessions.since-number.json` and `show-session-468.txt`,
`show-session-472.txt`, `show-session-567.txt`, `show-session-570.txt` are a second
machine of the same shape — Ubuntu, uid 1000, KDE — captured on 27 Oct under systemd
259.9-0ubuntu0.1:

    loginctl --no-ask-password list-sessions --json=short   -> list-sessions.since-number.json
    loginctl --no-ask-password show-session 468 -p Name -p Type -p State -p Remote -p LockedHint
      (and 472, 567, 570)                                   -> show-session-<id>.txt

It exists because of the field the first capture could not see. Both rows of
`list-sessions.json` carry `"since":null`, and 259.9 prints `"since":855186289603` on
every row it reports as idle — a number, where `listEntry.Since` was a `*string`. The
decode failed, `list()` decoded the whole array in one call, and the daemon saw no
sessions at all on a machine with four lockable ones (#167). A fixture that only ever
carries the value a field takes when it is absent cannot pin that field's type.

The two captures also disagree about `since` in a way nobody has resolved: the table
prints a wall clock (`Thu 2026-10-01 15:47:23 UTC`) and this JSON prints an integer.
Nothing in this repository reads the field, so the disagreement is recorded in
`since`'s comment rather than reconciled — inventing a unit would be claiming a meaning
neither capture establishes.
