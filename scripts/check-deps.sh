#!/bin/sh
# Fails when the build has acquired a third-party dependency.
#
# This project ships as one static binary with nothing to install alongside it,
# which only holds while the module graph stays empty. The rule is cheap to
# state and easy to break by accident with an IDE auto-import, so it is checked
# in CI rather than trusted.
#
# The check FAILS CLOSED. A check that could not run has not passed, so every
# scan reports its own errors instead of having them swallowed, an empty result
# from a scanner means "did not scan" rather than "found nothing", and "no
# match" is never confused with "could not look".
set -eu

cd "$(dirname "$0")/.."

GO=${GO:-go}
MODULE=github.com/vicowa-botworks/parental-lockout
status=0

if [ ! -f go.mod ]; then
	echo "check-deps: no go.mod in $(pwd) — nothing was checked." >&2
	exit 1
fi

# Scanner stderr goes to its own file: merging it into the captured value would
# let a warning masquerade as a result, and dropping it would hide the reason a
# scan failed.
scanErr=$(mktemp)
trap 'rm -f "$scanErr"' EXIT INT TERM HUP

# Every module in the tree, not just the root one. A nested go.mod is invisible
# to `go build ./...`, so it would pass the gate silently and become a live
# dependency the moment someone runs `go work init` or a `go mod edit -replace`.
mods=$(find . -name go.mod -not -path './.git/*' 2>"$scanErr") || {
	echo "check-deps: the go.mod scan failed:" >&2
	cat "$scanErr" >&2
	exit 1
}
if [ -s "$scanErr" ]; then
	echo "check-deps: the go.mod scan reported problems:" >&2
	cat "$scanErr" >&2
	exit 1
fi
if [ -z "$mods" ]; then
	echo "check-deps: the go.mod scan found nothing; nothing was verified." >&2
	exit 1
fi

# find does not follow directory symlinks, so a symlink pointing outside the
# repository hides a whole tree from the scan above. One pointing inside it
# hides nothing, because the real directory is scanned where it lives, so only
# the escaping kind is rejected.
repo=$(pwd -P)
links=$(find . -type l -xtype d -not -path './.git/*' 2>"$scanErr") || {
	echo "check-deps: the symlink scan failed:" >&2
	cat "$scanErr" >&2
	exit 1
}
if [ -s "$scanErr" ]; then
	echo "check-deps: the symlink scan reported problems:" >&2
	cat "$scanErr" >&2
	exit 1
fi
while IFS= read -r link; do
	[ -n "$link" ] || continue
	target=$(readlink -f "$link")
	case "$target" in
		"$repo"|"$repo"/*) : ;;
		*)
			echo "check-deps: $link points outside the repository at $target; find cannot scan it for modules." >&2
			status=1
			;;
	esac
done <<LINKS
$links
LINKS

# The heredoc keeps the loop in this shell, so `status` survives it; a piped
# `while` would run in a subshell and throw the verdict away.
while IFS= read -r mod; do
	if [ ! -f "$mod" ]; then
		echo "check-deps: $mod disappeared mid-check." >&2
		status=1
		continue
	fi
	rc=0
	grep -Eq '^[[:space:]]*require([[:space:]]|\()' "$mod" || rc=$?
	case $rc in
		0)
			echo "check-deps: $mod has a require line; this project is stdlib-only." >&2
			status=1
			;;
		1) : ;;
		*)
			# grep could not read the file. "No match" is 1; anything above that
			# is an error, and reading an unreadable file as clean is a pass won
			# by not looking.
			echo "check-deps: cannot read $mod; nothing was verified about it." >&2
			exit 1
			;;
	esac
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
