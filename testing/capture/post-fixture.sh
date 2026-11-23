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

# Declared `off`, and measured in both directions — which is what the first two versions of this
# block did not do.
#
# #207 put `Emulation.setScriptExecutionDisabled` in the shared session (cdp.py). It did not read this
# declaration: the call was `value=True` whatever a fixture said, so the override advertised below was
# accepted and ignored and every frame was a scripts-off frame from the moment #207 landed. From D-0025
# `cdp.scripts_disabled` reads `PLOCK_CAPTURE_SCRIPTS`, so the knob below decides the browser and the
# transcript prints which way it went.
#
# What that bought, measured 6 Nov as four runs of these two fixtures (both arms, both fixtures):
# `PLOCK_CAPTURE_SCRIPTS=on` and the default produce ONE PNG each, and they have to. Neither page renders
# a `time[data-unix]` element, and rewriting those nodes is the only load-time DOM write in
# `static/app.js`, so these renders hold nothing the page's scripts could change. The control is
# `/dashboard`: with the scripts running its casey row reads `locked out until 7h 46m` instead of the
# server's `until 00:00`, and `pages.py` refuses that frame on its declared state.
#
# The digests this block quoted for that pair — `lock.png` `b59bb2be…`, `testlock.png` `acc5ba4d…`, "both
# equal to the committed files" — were not the committed files when they were written. Read out of the
# blobs: `38578aa` (19 Oct) `b59bb2be`/`acc5ba4d`; `a2ccac0` and `9e07dde` (5 Nov) `4ad6d2b0`/`2c87c1b7`
# and `7ec1a664`/`d008fdf9`; `b9eeec3` (6 Nov) `712ee631`/`ae25abeb`; `3bcbfdd` (8 Nov)
# `df499589`/`d285d1cd`, which is what is committed now. So the sentence was true of the branch it was
# measured on and stopped being true of `main` the day the masthead frames landed, and it sat in this
# file for four days saying "equal to the committed files" about two digests no head publishes. The pair
# was run again 10 Nov and both arms give one file apiece again — `df49958928685fa0…` and
# `d285d1cd0d0389b6…`, equal to the committed frames. That is the conclusion the arms support; the
# digests are only re-measurable the day they are quoted, which is #213 in one sentence.
#
# So the sentence this file carried until now — "all nine artifacts reproduced the committed digests,
# therefore these pixels ARE the scripts-off pixels" — inferred a condition from a byte comparison that
# cannot see the difference, on exactly the two artifacts whose value this field changed. It was the
# same mistake #211 refuses, wearing the opposite sign: the first version inferred `on` from commit
# dates, the second inferred `off` from digest identity, and neither pair of pixels could have told the
# two values apart. The claim is withdrawn here rather than softened.
#
# What makes `off` true of the committed frames is therefore provenance that actually holds: both were
# photographed by this branch's drivers at the commit the manifest's `head` names, with this
# declaration written to their `fixture.env` and read by the session that took them. That is #213's
# re-take, and it had to happen before this value could be stated — an October frame taken by a driver
# that ran the page's scripts could not be made honest by editing a field. `manifest.py check` refuses
# a committed entry recording `on`, because a frame taken with the scripts running has never tested
# this folder's criterion; taking one is still how you measure the switch, and its PNG belongs in
# `testing/capture/runs/`, not in `docs/screenshots/360`.
CAPTURE_SCRIPTS="${PLOCK_CAPTURE_SCRIPTS:-off}"

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

# How many lockable sessions the target account holds on this machine right now, taken before the
# browser starts and declared as a condition of the frame (#213). Both POST pages render one row per
# lockable session of `$CAPTURE_TARGET`, so this number is the page's shape: it decides the row count
# and therefore the height, and until now nothing anywhere said what it was. Asking the product
# rather than counting logind rows here is deliberate — see the note on `capture_declare_census` in
# fixture.sh for the measurements behind it.
#
# An operator declaration wins, and that is the negative control: `PLOCK_CAPTURE_SESSIONS=1` in front
# of this script describes a census this machine does not have, and the driver has to refuse the frame
# it would otherwise write.
if [ -z "$CAPTURE_SESSIONS" ]; then
  capture_declare_census "$PLOCK_BIN" "$CONFIG"
fi

capture_env
capture_done
