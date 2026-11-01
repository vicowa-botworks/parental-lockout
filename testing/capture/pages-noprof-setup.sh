#!/usr/bin/env bash
# The accounts page with NOTHING to assign (#198).
#
# A second FIXTURE, not a second state of one fixture. `profiles-refused` is the same document
# photographed one POST later, so one daemon serves both artifacts. The #192 branch is a property of
# the DOCUMENT -- `users.tmpl` picks it on `{{else if $.Profiles}}` -- so it needs a different config,
# a different daemon, and therefore a different run directory. Pointing the existing driver at this
# document would photograph four other pages against a fixture built for none of them, which is what
# `--fixture` in pages.py exists to refuse.
#
# This is the state the report was written from: a machine where nothing has been set up yet. There
# are no profiles, so the Assign dropdown has one option and that option is "not managed"; a control
# that cannot express an action, rendered as though it worked, and Save answered with a sentence about
# STOPPING management. The rows come from account DISCOVERY rather than from `users`, so a document
# with no accounts still renders a page -- which is exactly the fresh install.
#
# What this fixture deliberately does NOT contain is an account entry pointing at a profile that is
# gone, the other half of #192's fix. That document cannot be booted: serve.go:97 validates after
# Load (which itself validates nothing) and refuses it -- measured, `users[0].profileID: "school"
# does not name an existing profile`. The only route in is changing the file while the daemon is
# already running, because readConfig reloads per request. Reaching it here would mean the driver
# editing the fixture's config between boot and navigation, which is a state TRANSITION and wants the
# `Refusal` mechanism beside it, not a static document. It stays pinned by
# TestTheUsersPageStillLetsADanglingEntryBeUnmanaged and unpinned by any artifact; #198's follow-up
# says which.
set -euo pipefail
# shellcheck source=testing/capture/fixture.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/fixture.sh"

capture_begin pages-noprof 8503 9335

# No `profiles` key at all rather than an empty list: `nil` and `[]` are two Go values, and the one a
# fresh install actually writes is the one where the key was never added. `users.tmpl` branches false
# on both, so this is a choice about fidelity to the install, not about which makes the check pass.
#
# No `users` either, for the same reason. An entry naming no profile is refused at boot
# (validate.go:281 tests every entry against the profile set, and `""` names nothing), so claiming a
# managed-but-unassigned account here would produce a fixture that boots only if the daemon were
# started some other way -- and then the artifact would be a photograph of the workaround.
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
  }
}
JSON
chmod 600 "$CONFIG"

capture_admin

# An empty trail: this page renders no audit rows, and the one thing a seeded trail would buy is a
# temptation to point the ordinary driver at this run directory. PLOCK_CAPTURE_MODE is what refuses
# that, and a fixture with nothing in it is the version where the refusal is cheapest to see.
: > "$AUDIT"
capture_env
capture_done
