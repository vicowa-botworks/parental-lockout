#!/bin/sh
# Fails when a commit message on this branch carries a claim GitHub may act on.
#
# Why this is a separate check and not part of the body check (H-0166, 14 Nov): a commit message is
# text GitHub parses exactly like a pull-request body, and its keywords act the moment the commit
# reaches the default branch. The body file was checked at review time and the messages never were.
# Measured on this repository's own branch: `git log --format=%B origin/main..HEAD` over the commits
# of PR #357 answered two adjacent forms in `8c9999b`, in the very sentence written to warn about them
# -- the shape `#1256` records from PR #245, one layer down, in a place no check looked. Both of its
# numbers happened to be already-closed artifacts that day, so the merge would have done nothing. That
# is luck about two numbers and not a property of the shape: the same sentence over two OPEN issues
# closes them with no review, no note and no trace but a `closed` event.
#
# The scan is span-blind, which is the whole reason this file exists rather than calling the checker
# with its body defaults. A code span being inert is a MEASUREMENT, made over merged PR bodies on
# 14 Nov: a keyword inside inline code and nowhere else fired 0 of 2. Nobody has ever measured what
# GitHub does with a backticked verb in a commit message, and the specimen that started this had its
# verbs inside backticks. So no span is treated as a shelter here, and the direction of the doubt is
# the cheap one to be wrong in: a false refusal costs a reworded sentence, a false pass closes a
# ticket nobody asked to close.
#
# The check FAILS CLOSED. A basis ref that cannot be resolved means nothing was scanned, and that is
# reported as a failure, not a green line -- same rule as `scripts/check-deps.sh`. An empty range
# really is nothing to scan, and says so in words.
set -eu

# The checker travels with this script, so it is found by where THIS file sits; the repository whose
# messages are read is wherever the caller stands. That split is what makes the guard testable: a leg
# can `cd` into a fixture repo and run this file by its real path, the way `make verify` runs it from
# the root. It also means the waiver file belongs to the repository under scan, not to this script's.
here=$(dirname "$0")
CHECKER=$here/check-close-verb.py
if [ ! -f "$CHECKER" ]; then
	echo "check-commit-verbs: the checker '$CHECKER' is not there; nothing was scanned." >&2
	exit 1
fi
if ! root=$(git rev-parse --show-toplevel 2>&1); then
	echo "check-commit-verbs: no repository at $(pwd); nothing was scanned:" >&2
	printf '%s\n' "$root" >&2
	exit 1
fi
# The adjudicated list: one line per claim that stands and cannot be recalled, keyed by the FULL commit
# sha and the exact numbers it claims. It is not a disable switch. A claim is waivable only when its
# message is already published -- history here is never rewritten (AGENTS.md: `git revert`, never a
# force-push, and every merge on this repository is a merge commit, so a merged message is permanent)
# -- and a waiver names its commit and its numbers exactly, so it cannot cover a claim nobody has read
# yet. Anything found that the list does not name is a failure, and every waiver is REPRINTED on every
# run: a suppressed finding is a finding nobody can audit.
WAIVERS=$root/factory/close-verb-waivers

basis=""
if [ $# -gt 1 ]; then
	echo "check-commit-verbs: usage: scripts/check-commit-verbs.sh [BASE]; got $(( $# - 1 )) arguments." >&2
	exit 1
fi
if [ $# -eq 1 ]; then
	basis=$1
elif [ -n "${PLOCK_COMMITS_BASE:-}" ]; then
	basis=$PLOCK_COMMITS_BASE
else
	for candidate in origin/main main; do
		if git rev-parse --verify -q "$candidate" >/dev/null 2>&1; then
			basis=$candidate
			break
		fi
	done
fi

if [ -z "$basis" ]; then
	echo "check-commit-verbs: no basis ref -- neither \$PLOCK_COMMITS_BASE nor origin/main nor main resolves;" >&2
	echo "  nothing was scanned, so this is not a pass. Pass the basis as an argument." >&2
	exit 1
fi
if ! git rev-parse --verify -q "$basis" >/dev/null 2>&1; then
	echo "check-commit-verbs: the basis '$basis' is not a rev this repository resolves; nothing was scanned." >&2
	exit 1
fi

if ! commits=$(git log --format='%H' "$basis..HEAD" 2>&1); then
	echo "check-commit-verbs: 'git log $basis..HEAD' failed; nothing was verified:" >&2
	printf '%s\n' "$commits" >&2
	exit 1
fi

# One scan per commit, then. One blob for the whole range would report a finding without naming the
# commit that carries it, and the remedy is per-message. The heredoc keeps the loop in this shell so
# `status` survives it -- a piped `while` runs in a subshell and throws the verdict away.
status=0
count=0
waived=0
while IFS= read -r sha; do
	[ -n "$sha" ] || continue
	count=$(( count + 1 ))
	if ! msg=$(git log -1 --format=%B "$sha" 2>&1); then
		echo "check-commit-verbs: the message of $sha could not be read; nothing was verified about it:" >&2
		printf '%s\n' "$msg" >&2
		exit 1
	fi
	rc=0
	# The checker writes its own per-number explanation; this file adds only the identity of the
	# message, because a refusal that does not name a commit cannot be acted on.
	out=$(printf '%s\n' "$msg" | python3 "$CHECKER" --span-blind - 2>&1) || rc=$?
	case $rc in
		0) : ;;
		1)
			# The finding numbers, read off the report's own per-number lines: each failure class --
			# negated, inert, refused -- writes exactly one of those per number, and nothing else does.
			nums=$(printf '%s\n' "$out" | sed -n 's/^    #\([1-9][0-9]*\) is .*/\1/p' | sort -un | paste -sd, -)
			waive=""
			if [ -f "$WAIVERS" ]; then
				waive=$(awk -v s="$sha" '$1 == s { print $2 }' "$WAIVERS")
			fi
			if [ -n "$waive" ] && [ "$nums" = "$waive" ]; then
				# Adjudicated, and reprinted rather than swallowed: the waiver is a record with a
				# reason in it, and a green line that hid it would be the finding all over again.
				awk -v s="$sha" '$1 == s { sub(/^[^ ]+ +[^ ]+ +/, ""); print "  WAIVED  " substr(s, 1, 10) ": " $0 }' "$WAIVERS"
				waived=$(( waived + 1 ))
				continue
			fi
			status=1
			echo "check-commit-verbs: $(git log -1 --format='%h (%s)' "$sha") carries a claim:" >&2
			printf '%s\n' "$out" | sed 's/^-/  /' >&2
			if [ -n "$waive" ]; then
				echo "  The waiver for this commit names $waive and the report says $nums, so it does" >&2
				echo "  not cover this. A waiver naming fewer numbers than the report, or more, is a" >&2
				echo "  stale record, not a shelter." >&2
			fi
			;;
		*)
			# 2 is "could not be read" and anything else is the checker dying. Neither is a pass: "no
			# match" must never be paid for with a silent skip.
			echo "check-commit-verbs: $sha could not be checked; '$CHECKER' did not answer:" >&2
			printf '%s\n' "$out" >&2
			exit 1
			;;
	esac
done <<COMMITS
$commits
COMMITS

if [ "$count" -eq 0 ]; then
	echo "check-commit-verbs: 0 commits between $basis and HEAD; there was nothing to read."
	exit 0
fi
if [ "$status" -ne 0 ]; then
	echo "check-commit-verbs: FAILED. What to do depends on whether the commit has been published." >&2
	echo "  Not published: rewrite it. The documented keyword in plain text, or \`Refs\` with the" >&2
	echo "  unfinished part named -- the two forms the report distinguishes." >&2
	echo "  Already published: do NOT rewrite it. Force-pushing an in-flight branch is forbidden here" >&2
	echo "  (AGENTS.md) and every merge on this repository is a merge commit, so the message is" >&2
	echo "  permanent history and the claim cannot be recalled. Read each named artifact back instead" >&2
	echo "  -- gh issue view N --state all --json number,state,closedAt -- and if it is still open," >&2
	echo "  decide deliberately: close it on purpose, or leave a note on it naming the message that" >&2
	echo "  claims it and what is owed. Then add the waiver line, which is that decision on disk." >&2
	exit 1
fi

echo "check-commit-verbs: $count message(s) between $basis and HEAD, no claim outside the keyword set"
if [ "$waived" -ne 0 ]; then
	echo "check-commit-verbs: $waived of them stand adjudicated; see $WAIVERS."
fi
