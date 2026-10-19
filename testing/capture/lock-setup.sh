#!/usr/bin/env bash
# Fixture for the LOCK RESULT page at 360 px: /users/<you>/lock-now.
#
# The page is the response to a POST from the dashboard, so the fixture's job is to make that
# POST answer with a table: an account that holds a real session here, a rule that holds that
# account right now, and a backend that reports success without proving anything — which is the
# state whose sentence this page exists to render honestly (#109).
#
# The config and the safeties are in post-fixture.sh, shared with the probe page.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

RUN_NAME=lock
RUN_PORT=8502
RUN_CDP=9334
# shellcheck source=testing/capture/post-fixture.sh
. "$HERE/post-fixture.sh"
