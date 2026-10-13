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
