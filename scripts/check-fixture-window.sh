#!/bin/sh
# Fails when a *_test.go fixture spells "always on" as start 00:00 with end 23:59.
#
# The pair reads to a human as 24/7 and evaluates as released for the last minute of every
# day, because Rule's `end` is exclusive -- internal/schedule/schedule.go states it: "a rule
# that ends at 06:30 has released the account by 06:30:00". A fixture that needs "the account
# is inside a window" and builds it from this pair is therefore true all day except for one
# minute a day, and a doctor-style acceptance test built that way was red for 60 seconds per
# day with no repro outside the minute (#355, P ~ 1 in 1440 per run at a random instant).
# The window semantics themselves are correct and pinned by internal/schedule/schedule_test.go
# and allday_test.go; the trap is the pair in a fixture that did not mean "released at 23:59".
#
# The repair is the documented spelling: "allDay": true widens the window past the clock ends
# (allday_test.go pairs it with exactly these ends and asserts active at 23:59:59).
#
# A fixture that pins the gap ON PURPOSE is the object of its own test, not the trap. Say so
# with `plock-window: intentional` on a comment line within three lines above the rule, and
# the guard passes it; what it refuses is the pair with neither allDay nor a stated reason.
#
# The check FAILS CLOSED, in the shape of check-deps.sh: a failed scan is reported, not
# swallowed, and an empty file list means "did not scan", not "found nothing". The line window
# is a floor and not a parser -- it reads a rule object as the three physical lines a fixture
# is written on -- so the failure direction is a false positive an author opts out of with the
# marker, never a silent pass.
# The awk program and the refusal messages are single-quoted precisely so nothing in them
# expands; that non-expansion is what those lines are about.
# shellcheck disable=SC2016
set -eu

cd "$(dirname "$0")/.."

scanErr=$(mktemp)
trap 'rm -f "$scanErr" violations' EXIT INT TERM HUP

# git ls-files, not a shell glob: a fixture in a file nothing scans is the miss this guard
# exists to make impossible, and the tracked list is what `make verify` on a clean tree covers.
# NUL-delimited output is NOT used because command substitution cannot carry a NUL byte --
# `$(git ls-files -z)` silently fuses names -- so the newline list is taken only after it is
# verified to contain no name a line-split would corrupt.
files=$(git ls-files -- '*_test.go' 2>"$scanErr") || {
	echo "check-fixture-window: the file scan failed:" >&2
	cat "$scanErr" >&2
	exit 1
}
if [ -s "$scanErr" ]; then
	echo "check-fixture-window: the file scan reported problems:" >&2
	cat "$scanErr" >&2
	exit 1
fi
if [ -z "$files" ]; then
	echo "check-fixture-window: the file scan found no *_test.go; nothing was verified." >&2
	exit 1
fi
if printf '%s\n' "$files" | grep -Eq '[[:space:]]'; then
	echo "check-fixture-window: a test filename contains whitespace; this guard refuses to" \
		"split its list by line rather than scan half a name." >&2
	exit 1
fi

# One awk per file: a single awk process would carry one file's trailing lines into the next
# file's three-line window.
printf '%s\n' "$files" | xargs -I{} sh -c 'awk '"'"'
{ line[FNR] = $0 }
END {
	for (i = 1; i <= FNR; i++) {
		if (line[i] !~ /"start": *"00:00"/) continue
		# A rule object as a fixture writes it: this line plus the next two.
		win = line[i] " " line[i + 1] " " line[i + 2]
		if (win !~ /"end": *"23:59"/) continue
		# allDay is written where the rule object opens, which the repair proposed in #355
		# shows is ABOVE the wrapped start line -- so it is looked for two lines up as well
		# as down.
		if (line[i - 2] " " line[i - 1] " " win !~ /allDay/) {
			ctx = win " " line[i - 1] " " line[i - 2] " " line[i - 3]
			if (ctx !~ /plock-window: intentional/)
				printf "%s:%d: %s\n", FILENAME, i, line[i]
		}
	}
}
'"'"' "$1"' _ {} > violations 2>"$scanErr" || {
	echo "check-fixture-window: the scan itself failed:" >&2
	cat "$scanErr" >&2
	exit 1
}
if [ -s "$scanErr" ]; then
	echo "check-fixture-window: the scan reported problems:" >&2
	cat "$scanErr" >&2
	exit 1
fi

if [ -s violations ]; then
	echo 'check-fixture-window: a fixture pairs "start": "00:00" with "end": "23:59", and that' \
		'pair is NOT "always on": `end` is exclusive (internal/schedule/schedule.go), so the' \
		'window releases the account every day from 23:59:00 to 00:00:00 -- the minute that' \
		'made a #355 acceptance test red with no repro outside itself.'
	echo 'Write "allDay": true for a rule that must never release (the spelling' \
		'internal/schedule/allday_test.go pins active at 23:59:59), or, if pinning the gap' \
		'minute is the point of the test, put `plock-window: intentional` on a comment line' \
		'within three lines above the rule and say why. The rule refuses the pair with' \
		'neither.'
	cat violations >&2
	exit 1
fi

exit 0
