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

# The heredoc is unquoted for one line: the listen address has to carry $RUN_PORT, so the port
# is named once, in capture_begin. A quoted heredoc here meant the port appeared twice, and
# the obvious edit -- changing the capture_begin line -- left the daemon on the old port and
# the driver probing the new one, which reads as "the daemon never answered".
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
        {"id": "r-allday", "name": "All weekend, every hour", "days": ["sat", "sun"],
         "allDay": true, "enabled": true},
        {"id": "r-curfew", "name": "Weeknight curfew including the weekend",
         "days": ["mon", "tue", "wed", "thu", "fri", "sat", "sun"],
         "start": "21:00", "end": "06:30", "enabled": true}
      ]
    }
  ],
  "users": [
    {"username": "casey", "profileID": "school", "enabled": true},
    {"username": "alex", "profileID": "school", "enabled": false}
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
# The timestamps are relative to now and signing in appends an auth.success row of its own, so
# audit.png is one of the artifacts in docs/screenshots/360/ that a second run cannot reproduce
# byte for byte -- README.md names each and says why.
: > "$AUDIT"
kinds=$(python3 - "$AUDIT" <<'PY'
import json, sys, datetime

# (kind, username, sessionId, ruleId, ruleName, detail, actor) -- detail copied from the emitter
# that writes it: assertedDetail / unconfirmedDetail in internal/enforce/enforce.go, the
# "%s until %s by %s: %s" format in internal/web/override.go, and the throttle key in
# internal/web/handlers.go. Only casey is enabled in the config above, so only casey has engine
# rows: a lockout line for a disabled account is a state the engine never reaches.
rows = [
    ("lock.asserted", "casey", "2", "r-allday", "All weekend, every hour",
     "re-asserted although the session reported itself locked; that report is not evidence the screen is locked", ""),
    ("lock.unconfirmed", "casey", "2", "r-curfew", "Weeknight curfew including the weekend",
     "a backend accepted the lock; nothing confirmed the screen locked", ""),
    ("lockout.entered", "casey", "2", "r-allday", "All weekend, every hour", "", ""),
    ("override.granted", "", "", "", "",
     "casey until 2026-10-20T02:00:00Z by parent: dentist appointment, travelling home afterwards", "admin"),
    ("auth.failure", "", "", "", "", "127.0.0.1", "admin"),
]

# astimezone() with no argument is the machine's local zone, which is what the daemon's own
# writer stamps. "Z" here would put a second zone in the page's When column.
#
# Rows go in the file OLDEST first, because that is what a writer that only ever appends
# produces -- and audit.Reader.Tail does not sort, it reads the tail and returns it reversed on
# that assumption. A fixture that seeds newest-first therefore renders oldest-first: the first
# version of this trail did exactly that, and the page labelled "newest first" showed the login
# at the top and then five rows climbing.
now = datetime.datetime.now().astimezone()
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
