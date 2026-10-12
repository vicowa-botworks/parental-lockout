#!/bin/sh
# Fails when the build has acquired a third-party dependency.
#
# This project ships as one static binary with nothing to install alongside it,
# which only holds while go.mod stays empty. The rule is cheap to state and
# easy to break by accident with an IDE auto-import, so it is checked in CI
# rather than trusted.
set -eu

cd "$(dirname "$0")/.."

GO=${GO:-go}
MODULE=github.com/vicowa-botworks/parental-lockout

if [ ! -f go.mod ]; then
	echo "check-deps: no go.mod in $(pwd)" >&2
	exit 1
fi

if grep -Eq '^[[:space:]]*require([[:space:]]|\()' go.mod; then
	echo "check-deps: go.mod has a require line; this project is stdlib-only." >&2
	echo "Remove the dependency or get the rule changed deliberately." >&2
	exit 1
fi

extra=$("$GO" list -m all 2>/dev/null | grep -v -E "^${MODULE}([[:space:]]|$)" || true)
if [ -n "$extra" ]; then
	echo "check-deps: unexpected modules in the build list:" >&2
	echo "$extra" >&2
	exit 1
fi

echo "check-deps: stdlib only"
