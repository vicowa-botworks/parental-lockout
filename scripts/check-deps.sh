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
# being silenced, and an empty result from a scanner is treated as "did not
# scan" rather than "found nothing".
set -eu

cd "$(dirname "$0")/.."

GO=${GO:-go}
MODULE=github.com/vicowa-botworks/parental-lockout
status=0

if [ ! -f go.mod ]; then
	echo "check-deps: no go.mod in $(pwd) — nothing was checked." >&2
	exit 1
fi

# A symlinked directory hides a go.mod from the scan below, because find does
# not follow directory symlinks, and it can point outside this repository.
# There is no legitimate reason for one in this tree, so it is rejected rather
# than chased.
links=$(find . -type l -xtype d -not -path './.git/*' 2>&1) || {
	echo "check-deps: cannot scan for symlinks:" >&2
	echo "$links" >&2
	exit 1
}
if [ -n "$links" ]; then
	echo "check-deps: symlinked directories hide modules from this check:" >&2
	printf '%s\n' "$links" >&2
	status=1
fi

# Every module in the tree, not just the root one. A nested go.mod is invisible
# to `go build ./...`, so it would pass the gate silently and become a live
# dependency the moment someone runs `go work init` or a `go mod edit -replace`.
mods=$(find . -name go.mod -not -path './.git/*' 2>/dev/null | sort)
if [ -z "$mods" ]; then
	echo "check-deps: the go.mod scan found nothing; nothing was verified." >&2
	exit 1
fi

# The heredoc keeps the loop in this shell, so `status` survives it; a piped
# `while` would run in a subshell and throw the verdict away.
while IFS= read -r mod; do
	if [ ! -f "$mod" ]; then
		echo "check-deps: $mod disappeared mid-check." >&2
		status=1
		continue
	fi
	if grep -Eq '^[[:space:]]*require([[:space:]]|\()' "$mod"; then
		echo "check-deps: $mod has a require line; this project is stdlib-only." >&2
		status=1
	fi
done <<MODULES
$mods
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
