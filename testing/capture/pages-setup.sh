#!/usr/bin/env bash
# Throwaway state for the four GET-rendered pages at 360 px.
#
# The config's job here is width. Every page is measured at 360 px, so the fixture has to contain
# the widest realistic values the templates can be handed — an account held by an all-day rule, a
# rule name long enough to want to wrap, a profile name with several words in it, and an audit
# trail with enough events to render a table rather than a sentence. A fixture of short names
# would report 360 px for a page that overflows in any real deployment, and the measurement would
# be a measurement of the fixture.
#
# Nothing is locked by this run: the lock backend is the command backend pointed at /bin/true,
# and the two accounts here hold no session on this machine.
set -euo pipefail
# The directive path is relative to where CI runs shellcheck (the repository root),
# which is the only place the file can be found without a --source-path flag.
# shellcheck source=testing/capture/fixture.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/fixture.sh"

capture_begin pages 8501 9333

# What this fixture declares about itself, before the config is written. These are the CAPTURE_*
# names, not the PLOCK_CAPTURE_* ones: fixture.sh is sourced at the top of this file and has already
# folded any operator override into them, and capture_env reads these at the end of the run. The
# ${VAR:-} shape keeps that override working for the one pair of values a driver might legitimately
# want to move — see the stamp in fixture.sh.
#
# Seven seeded rows plus the one the sign-in appends. `capture_done` refuses if the heredoc below
# stops producing that number, and pages.py refuses if the rendered page does not show it.
CAPTURE_AUDIT_ROWS="${PLOCK_CAPTURE_AUDIT_ROWS:-8}"
# The census comes from bin/loginctl in this folder rather than from the machine's logind, so the
# Sessions cell is the fixture's and not a report on whoever ran the capture (#207 cause 6). The two
# POST fixtures deliberately do not set this: their whole subject is a real lock on a real session.
CAPTURE_PATH_PREPEND="${PLOCK_CAPTURE_PATH_PREPEND:-$CAPTURE_DIR/bin}"
# The reader's zone, declared because the audit page used to print a zone NAME chosen by the host
# (see fixture.sh: the `MST` layout, #207's seventh cause, #210's fix). America/New_York is the zone
# the fixture's own wall times are written in, so the clock and any label the page might grow agree,
# on every host.
CAPTURE_TZ="${PLOCK_CAPTURE_TZ:-America/New_York}"

# The heredoc is unquoted for one line: the listen address has to carry $RUN_PORT, so the port
# is named once, in capture_begin. A quoted heredoc here meant the port appeared twice, and
# the obvious edit -- changing the capture_begin line -- left the daemon on the old port and
# the driver probing the new one, which reads as "the daemon never answered".
#
# The first rule is `allDay` on all seven days, and that is a decision about the photograph rather
# than about parenting. `dashboard.tmpl` chooses between *locked out*, *not now; next at* and
# *nothing scheduled* from `schedule.Evaluate(p, now)`, so a fixture whose account is only held on
# the weekend photographs a different page on Monday at 14:00 than on Sunday at 22:00 -- same
# fixture, same binary, two documents, and nothing in this folder said which of the three had been
# taken (#207 cause 5). Held every hour of every day, the row is the held branch whatever the host's
# clock says, and the cell's clock is the midnight the rule was written with.
#
# Which is only true on a tree that carries #208's fix. `resolveReading` built its candidates in UTC
# and handed that back as the answer's zone, so on the two days a year a window's end sits within 27h
# of a clock change, this same page says `until 05:00` for a rule that says midnight. The declared
# state check in pages.py is what notices: run this capture on a tree without that fix and it refuses
# rather than committing a frame whose sentence is wrong. That is the honest coupling between the two
# issues -- the fixture does not paper over the defect, it declines to photograph it.
#
# The curfew rule stays for the audit page's sake: its rows quote `Weeknight curfew including the
# weekend`, and that name is part of what makes the Rule column wide.
#
# alex is on a profile of its own, with one rule that is switched off, and the reason is the same
# cause. The cell's branches are `locked out until`, `scheduled, not enforced`, `not now; next at`
# and `nothing scheduled` (internal/web/templates/dashboard.tmpl:36-55), and the second of the two
# time-telling ones is a property of the hour: verdictRule (internal/dashboard/dashboard.go:310-319)
# hands back NextStart while nothing is running and EndsAt once it is, so an account with a live
# curfew says `not now; next at 21:00` at noon and `nothing scheduled` at ten to ten. Two fixtures of
# one document at two times of day -- the acceptance criterion in #207 -- cannot both be photographed
# by a row like that; it would be a third page, which is precisely what the driver now refuses to
# commit. A profile with no enabled rule is `nothing scheduled` at every hour of every weekday, so
# the frame says the same thing on Sunday at 22:00 as on Wednesday at 14:00.
#
# What that costs is worth naming rather than burying: the `next at` branch is no longer in any
# photograph, and it cannot be, because there is no rule that is never running and still has a next
# start. Its home is now TestOutsideTheWindowTheRowShowsTheNextOpening in
# internal/dashboard/dashboard_test.go, which pins the sentence and the clock without depending on
# when it ran. A branch that a byte-stable artifact cannot hold is a branch a test holds instead.
cat > "$CONFIG" <<JSON
{
  "version": 1,
  "admin": {},
  "settings": {
    "listen": "127.0.0.1:$RUN_PORT",
    "tls": "off",
    "pollSeconds": 3600,
    "enforcement": true,
    "lockBackends": ["command"],
    "lockCommandPath": "/bin/true"
  },
  "profiles": [
    {
      "id": "school",
      "name": "School nights and weekends",
      "timezone": "America/New_York",
      "rules": [
        {"id": "r-allday", "name": "Every day, every hour", "days": ["mon", "tue", "wed", "thu", "fri", "sat", "sun"],
         "allDay": true, "enabled": true},
        {"id": "r-curfew", "name": "Weeknight curfew including the weekend",
         "days": ["mon", "tue", "wed", "thu", "fri", "sat", "sun"],
         "start": "21:00", "end": "06:30", "enabled": true}
      ]
    },
    {
      "id": "quiet",
      "name": "Rules switched off",
      "timezone": "America/New_York",
      "rules": [
        {"id": "r-quiet", "name": "Weekend, until somebody turns it back on", "days": ["sat", "sun"],
         "allDay": true, "enabled": false}
      ]
    }
  ],
  "users": [
    {"username": "casey", "profileID": "school", "enabled": true},
    {"username": "alex", "profileID": "quiet", "enabled": false}
  ]
}
JSON
chmod 600 "$CONFIG"

capture_admin

# A trail long enough that the audit page renders its table rather than a sentence, with the
# widest realistic value in every column. Every row is written the way its emitter writes it, and
# the kinds are then checked against the binary rather than asserted here: this fixture has now
# seeded two events the application cannot produce (`auth.login`, then `config.changed`), and both
# times the photograph was the thing that lied. Two shapes matter and neither is decoration:
#
#   * `srv.Audit` (cmd/plock/serve.go) sets Actor and Detail and never Username, so a row the web
#     layer wrote renders `nobody` in the Account column. Seeding a username on one of those rows
#     photographs a column the product cannot fill.
#   * The engine stamps local time and the page prints each row's own zone, so a trail written in
#     UTC renders `15:48:17 UTC` beside `11:52:18 EDT` in one column -- and because the fixture's
#     UTC clock sat four hours ahead of the local one, the newest event appeared *below* events
#     four hours in its future on a page that says "newest first". Seeded rows now carry the
#     emitter's offset, so the column shows one zone and reads in the order it claims to.
#
# What is left of #207's cause 2 is the row the sign-in appends, and the driver owns that one: it
# re-stamps it to the declared instant and refuses if the appended row is not the `auth.success` the
# web layer was writing when this fixture was written. `capture_done`, which this file calls last,
# refuses if the rows seeded here and the number declared at the top disagree.
: > "$AUDIT"
kinds=$(python3 - "$AUDIT" "$CAPTURE_REPO/internal/platform/testdata/list-sessions.json" \
  "$CAPTURE_STAMP" "$CAPTURE_ZONE" <<'PY'
import json, sys, datetime

# The payload the platform backend quotes when session enumeration comes back in a shape it cannot
# read, read from internal/platform/testdata/list-sessions.json rather than retyped here. That file
# is named at internal/platform/logind_test.go:86 as "what a live systemd 259 printed", and its
# field set is exactly what logind.go's listEntry decodes: class, idle, leader, seat, session,
# since, tty, uid, user. The hand-written blob this replaces kept idleHint, remoteHost, service,
# type and state, none of which `loginctl list-sessions` prints, so the note below -- "detail copied
# from the emitter", written after two rows got photographed that the product could not produce --
# was false on the one row whose text matters most. Naming the file is also the only version of
# this that stays true when systemd changes.
with open(sys.argv[2]) as fh:
    LOGINCTL = ("enumerate sessions: command: loginctl list-sessions: unrecognised output: "
                + fh.read().strip())

# (kind, username, sessionId, ruleId, ruleName, detail, actor) -- detail copied from the emitter
# that writes it: assertedDetail / unconfirmedDetail in internal/enforce/enforce.go, the
# "%s until %s by %s: %s" format in internal/web/override.go, and the throttle key in
# internal/web/handlers.go. Only casey is enabled in the config above, so only casey has engine
# rows: a lockout line for a disabled account is a state the engine never reaches.
rows = [
    ("lock.asserted", "casey", "2", "r-allday", "Every day, every hour",
     "re-asserted although the session reported itself locked; that report is not evidence the screen is locked", ""),
    # #177: the row a parent most needs to recognise, and the longest sentence the engine
    # writes. Seeded with the counted arm rather than the first-release arm because the two
    # are different strings and this is the one that says how often it has happened.
    ("lock.released", "casey", "2", "r-curfew", "Weeknight curfew including the weekend",
     "the screen has come back unlocked 2 times since this lockout began, each time after a confirmed lock; locked again now. Only the session user can write the signal this is inferred from, so this is either someone dismissing the lock or someone writing the hint, and neither happens on a machine nobody is touching", ""),
    ("lock.unconfirmed", "casey", "2", "r-curfew", "Weeknight curfew including the weekend",
     "a backend accepted the lock; nothing confirmed the screen locked", ""),
    ("lockout.entered", "casey", "2", "r-allday", "Every day, every hour", "", ""),
    ("override.granted", "", "", "", "",
     "casey until 2026-10-20T02:00:00Z by parent: dentist appointment, travelling home afterwards", "admin"),
    ("auth.failure", "", "", "", "", "127.0.0.1", "admin"),
    # Oldest row (the writer reverses this list). #140: a backend's error text quotes things, and
    # what it quotes has no spaces in it. This is the shape the daemon writes when session
    # enumeration comes back in a format it cannot read -- `enumerate sessions: %v` around the
    # captured loginctl listing, which is argv[2] and is read below rather than retyped here.
    #
    # It is seeded rather than emergent on purpose. The row that took this page to 893 px came
    # from the running daemon failing to enumerate THIS machine's sessions; on a box where that
    # enumeration succeeds, or fails with a shorter sentence, the audit page fits at 360 px having
    # been asked for nothing, and the capture reports 360 px while proving nothing. The fixture is
    # now the thing that decides how hard the page is pushed, and require_stress_row in pages.py
    # refuses when this row stops being on the page.
    #
    # What this row is NOT is worth saying, because the wrong version of it sat here and argued
    # that the CSS rule was unnecessary. It used to claim a long token is not by itself an
    # overflow at 360 px, "measured, 320 characters of unbreakable hex fit with no wrap rule
    # anywhere in app.css". That measurement does not reproduce: in the audit Detail cell at a
    # 360 px viewport, 320 characters of hex are 2848 px of content in a 312 px box. And "no wrap
    # rule anywhere in app.css" is now false on its face -- app.css carries two of them.
    #
    # What decides the width is the longest run between the browser's break opportunities, and
    # those are not only spaces: a `[{…},{…}]` blob breaks between objects at `},`, which is why
    # the 893 px figure looked emergent -- it was whatever the daemon happened to quote. A single
    # JSON object does not break at all: 117 characters, 808 px. require_stress_row pins the
    # rule; require_unbreakable in pages.py is the floor on this row's realism and carries the
    # same correction. It is here so the width check is never run against a page that could not
    # have overflowed.
    ("platform.error", "", "", "", "", LOGINCTL, ""),
]

# The declared instant (argv 3 and 4, from fixture.sh), not the host's clock. The line this replaces
# was `datetime.datetime.now().astimezone()`: `now()` put the wall clock into every row's `at`, and
# `astimezone()` with no argument put the HOST's zone there too, so the When column was a function of
# when the photograph was taken and of whose machine took it -- #207's cause 1, and half of why
# audit.png had never been re-taken.
#
# An offset is written rather than a zone name because that is what the daemon's writer stamps: the
# trail on disk holds an instant with an offset, never a name, and the page prints each row in the
# zone the row itself carries. "Z" here would have put a second zone in the column beside the seeded
# rows' -04:00 -- the README's own note about a UTC clock sitting four hours ahead of the local one
# and putting the newest event below events in its future.
# Rows go in the file OLDEST first, because that is what a writer that only ever appends
# produces -- and audit.Reader.Tail does not sort, it reads the tail and returns it reversed on
# that assumption. A fixture that seeds newest-first therefore renders oldest-first: the first
# version of this trail did exactly that, and the page labelled "newest first" showed the login
# at the top and then five rows climbing.
now = datetime.datetime.fromisoformat(sys.argv[3] + sys.argv[4])
with open(sys.argv[1], "w") as f:
    for i, (kind, user, sid, rid, rname, detail, actor) in enumerate(reversed(rows)):
        at = (now - datetime.timedelta(minutes=len(rows) - i)).isoformat(timespec="milliseconds")
        f.write(json.dumps({
            "at": at, "kind": kind, "username": user, "sessionId": sid,
            "ruleId": rid, "ruleName": rname, "detail": detail, "actor": actor,
        }) + "\n")
        print(kind)
PY
)
# The claim above is checkable, so check it. Go keeps a string constant in the binary verbatim, so
# a kind that appears nowhere in it is a kind nothing in the product writes -- which is how
# `auth.login` and then `config.changed` got photographed. This is a floor, not a proof: it says
# the word exists in the binary, not that an emitter reaches it, which is why the rows above copy
# their detail text from the emitter rather than paraphrasing it.
while IFS= read -r kind; do
  grep -qaF "$kind" "$PLOCK_BIN" || {
    echo "REFUSED: the fixture seeds $kind and $PLOCK_BIN does not contain that string, so no" >&2
    echo "emitter writes it. The audit page would be photographed showing an event the daemon" >&2
    echo "cannot produce." >&2
    exit 1
  }
done <<< "$kinds"

capture_env
capture_done
