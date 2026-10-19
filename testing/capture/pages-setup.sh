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

cat > "$CONFIG" <<'JSON'
{
  "version": 1,
  "admin": {},
  "settings": {
    "listen": "127.0.0.1:8501",
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
# widest realistic value in every column. The timestamps are written relative to now, and signing
# in appends an auth.success row of its own, so audit.png is one of the two artifacts in
# docs/screenshots/360/ that a second run cannot reproduce byte for byte — README.md names both
# and says why.
: > "$AUDIT"
python3 - "$AUDIT" <<'PY'
import json, sys, datetime
now = datetime.datetime.now(datetime.timezone.utc)
rows = [
    ("lock.asserted", "casey", "r-curfew", "Weeknight curfew including the weekend",
     "session 2 on seat seat0, re-asserted because the hint is not evidence"),
    ("lockout.entered", "casey", "r-allday", "All weekend, every hour", ""),
    ("override.granted", "alex", "", "", "30 minutes; reason: dentist appointment, travelling home afterwards"),
    ("config.changed", "", "", "", "profile school: rule Weeknight curfew including the weekend edited"),
    ("auth.login", "parent", "", "", "from 127.0.0.1"),
]
with open(sys.argv[1], "w") as f:
    for i, (kind, user, rid, rname, detail) in enumerate(rows):
        f.write(json.dumps({
            "at": (now - datetime.timedelta(minutes=i)).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z",
            "kind": kind, "username": user, "sessionId": "2" if kind.startswith("lock") else "",
            "ruleId": rid, "ruleName": rname, "detail": detail,
            "actor": "parent" if kind == "config.changed" else "",
        }) + "\n")
PY

capture_env
capture_done
