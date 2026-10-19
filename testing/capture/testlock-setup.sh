#!/usr/bin/env bash
# Fixture for the BACKEND-PROBE result page at 360 px: /users/<you>/test-lock.
#
# Identical fixture to lock-setup.sh apart from the port, and deliberately so: the two result
# pages are supposed to render the same session table and the same Detail cell, so a fixture that
# differed between them would let one page's capture pass on a state the other page never shows.
# The difference that matters is in the driver, not here — the probe capture reads the audit trail
# before it writes the PNG and refuses anything but `lock.probed`, because a page that looked
# right and had actually locked the screen would otherwise be indistinguishable in the artifact.
#
# The config and the safeties are in post-fixture.sh.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

RUN_NAME=testlock
RUN_PORT=8503
RUN_CDP=9335
# shellcheck source=testing/capture/post-fixture.sh
. "$HERE/post-fixture.sh"
