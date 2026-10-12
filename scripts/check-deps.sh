#!/bin/sh
# Fails when the build has acquired a third-party dependency.
#
# This project ships as one static binary with nothing to install alongside it,
# which only holds while the module graph stays empty. The rule is cheap to
# state and easy to break by accident with an IDE auto-import, so it is checked
# in CI rather than trusted.
#
# The check FAILS CLOSED. A check that could not run has not passed, so every
# command whose result the verdict depends on reports its own failure instead of
# being silenced.
set -eu

cd "$(dirname "$0")/.."

GO=${GO:-go}
MODULE=github.com/vicowa-botworks/parental-lockout
status=0

if [ ! -f go.mod ]; then
	echo "check-deps: no go.mod in $(pwd) — nothing was checked." >&2
	exit 1
fi

# Every module in the tree, not just the root one. A nested go.mod is invisible
# to `go build ./...`, so it would pass the gate silently and become a live
# dependency the moment someone runs `go work init` or a `go mod edit -replace`.
while IFS= read -r mod; do
	if grep -Eq '^[[:space:]]*require([[:space:]]|\()' "$mod"; then
		echo "check-deps: $mod has a require line; this project is stdlib-only." >&2
		status=1
	fi
done <<MODULES
$(find . -name go.mod -not -path './.git/*' | sort)
MODULES

# A workspace file can graft on modules this repository does not control, and
# nothing in the build would show why they appeared.
if [ -f go.work ]; then
	echo "check-deps: go.work found; a workspace can import modules outside this repo." >&2
	status=1
fi

# The build list is the ground truth beneath the textual checks above. If it
# cannot be computed the check has not run, so that is an error, not a pass.
if ! list=$("$GO" list -m all 2>&1); then
	echo "check-deps: '$GO list -m all' failed; nothing was verified:" >&2
	echo "$list" >&2
	exit 1
fi

extra=$(printf '%s\n' "$list" | grep -v -E "^${MODULE}([[:space:]]|$)" || true)
if [ -n "$extra" ]; then
	echo "check-deps: unexpected modules in the build list:" >&2
	printf '%s\n' "$extra" >&2
	status=1
fi

if [ "$status" -ne 0 ]; then
	echo "check-deps: FAILED. Remove the dependency, or change this rule deliberately." >&2
	exit 1
fi

echo "check-deps: stdlib only"
