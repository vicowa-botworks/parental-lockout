#!/usr/bin/env bash
# The fixture shared by both POST-only result pages. Sourced by lock-setup.sh and
# testlock-setup.sh, which each name themselves and call this.
#
# Same shape as the pages fixture, and for the same reason: the page under test only exists as the
# response to a POST, and the POST has to be for an account that actually holds a session on this
# machine — a config-only name like "casey" has none, so the capture would photograph the
# zero-session sentence instead of the table. The current user is the one name here that provably
# holds sessions, so fixture.sh resolves the target from `id -un` rather than inventing one.
#
# Two deliberate safeties, both load-bearing:
#
#   enforcement: false        — the all-day weekend rule below would otherwise put this account
#                               inside a live window, and the enforcement loop would re-assert the
#                               lock on every tick while the screenshot was being taken.
#   lockBackends: ["command"]
#   lockCommandPath: /bin/true — the daemon asks a command that exits 0 and can prove nothing.
#                               Nothing on this machine's real desktop is touched by this run. For
#                               the lock page that is the point: the sentence under test is the
#                               UNCONFIRMED one, which is the sentence the template exists to
#                               render honestly. For the probe page it also works, because the
#                               command backend implements Prober, so that page has a real backend
#                               to report on — and a probe never sends a lock in any case, which
#                               the capture then checks the trail for.
#
# The rule is all-day and covers every day, so the account is inside a window whenever the capture
# runs. That is what makes the page show a verdict at all rather than "no rule holds this
# account".
#
# shellcheck source=testing/capture/fixture.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/fixture.sh"

capture_begin "$RUN_NAME" "$RUN_PORT" "$RUN_CDP"

cat > "$CONFIG" <<JSON
{
  "version": 1,
  "admin": {},
  "settings": {
    "listen": "127.0.0.1:$RUN_PORT",
    "tls": "off",
    "pollSeconds": 3600,
    "enforcement": false,
    "lockBackends": ["command"],
    "lockCommandPath": "/bin/true"
  },
  "profiles": [
    {
      "id": "always",
      "name": "All day, every day",
      "timezone": "America/New_York",
      "rules": [
        {"id": "r-allday", "name": "All day, every day of the week",
         "days": ["mon","tue","wed","thu","fri","sat","sun"],
         "allDay": true, "enabled": true}
      ]
    }
  ],
  "users": [
    {"username": "$CAPTURE_TARGET", "profileID": "always", "enabled": true},
    {"username": "casey", "profileID": "always", "enabled": true}
  ]
}
JSON
chmod 600 "$CONFIG"

capture_admin

# The trail starts empty on purpose. Both captures read it afterwards as the second half of their
# own claim: the lock page has to have written a line for the lock it says it requested, and the
# probe page must have written lock.probed and nothing that claims a lock. Pre-loading events
# would let either pass on someone else's audit line.
: > "$AUDIT"

capture_env
capture_done
