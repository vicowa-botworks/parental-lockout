#!/usr/bin/env bash
# next-decision-id.sh -- the next decision id that no local branch ref already claims.
#
# Issue #253. The allocator used to be prose: the skill says "number from the highest id in the file
# you are writing to." That is an instruction to read the current checkout, and a decision id is a
# reference that outlives one checkout. Three branches cut from the same trunk each saw H-0060 as the
# next free value and each wrote an H-0061; H-0065, H-0066 and H-0068 then had to move. This script
# makes the allocator answer the question the prose actually needs: is this id already taken on a
# sibling branch?
#
# What "free" means. The id is one higher than the highest id of this prefix anywhere in the repo's
# local refs -- the working file, HEAD, every refs/heads ref and every refs/remotes ref. It is not the
# lowest unused number. The logs deliberately retain gaps: H-0031 and H-0032 were retired when the
# harness prefixes were split from D ids, and H-0057 is a permanent gap created by a documented
# renumbering. A "smallest gap" allocator would recycle those numbers and turn every historical
# citation into an alias.
#
# This is a collision-free read, not a lock. Two processes running at the same instant can still see
# the same ceiling and receive the same id; the harness fixes that by cutting each unit from an
# up-to-date trunk and re-reading immediately before the entry is committed. The allocator therefore
# deliberately never fetches. It reads refs the machine already has, exactly as #249's staleness
# verdict does: a network read in front of a budget can park a pass (H-0013, H-0056). Refresh refs
# explicitly before relying on the answer if a sibling may have pushed since the last fetch.
set -euo pipefail

USAGE='usage: next-decision-id.sh [--repo DIR] [--prefix D|H] [DECISIONS.md]

Print the next decision-log id for DIR. The path is the log being written and selects the prefix:
factory/DECISIONS.md uses H-, any other DECISIONS.md uses D-; --prefix overrides that.

The union read is the working file, HEAD, refs/heads and refs/remotes. Blobs that do not contain the
named log, and refs git cannot read, are ignored. The result is the first free id ABOVE every id seen;
gaps below the ceiling stay closed. No fetch is performed.'

die() {
	printf 'next-decision-id: %s\n' "$*" >&2
	exit 2
}

repo=$PWD
log=''
prefix=''

while [ $# -gt 0 ]; do
	case "$1" in
	--repo)
		[ $# -ge 2 ] || die '--repo needs a directory'
		repo=$2
		shift 2
		;;
	--prefix)
		[ $# -ge 2 ] || die '--prefix needs D or H'
		prefix=$2
		shift 2
		;;
	--help | -h)
		printf '%s\n' "$USAGE"
		exit 0
		;;
	--)
		shift
		[ $# -le 1 ] || die 'expected at most one log path'
		[ $# -eq 0 ] || log=$1
		break
		;;
	-*)
		die "unknown option: $1"
		;;
	*)
		[ -z "$log" ] || die "unexpected argument: $1"
		log=$1
		shift
		;;
	esac
done

case "$prefix" in
'' | D | H) ;;
*) die "--prefix must be D or H, not $prefix" ;;
esac

[ -n "$log" ] || log='DECISIONS.md'

# An explicit --repo has to win over the caller's path. The selftest runs from the real worktree and
# asks about a fixture path named `DECISIONS.md`; resolving that first against the caller would make
# every fixture read this repository's log and certify the wrong subject.
if [ -d "$repo" ]; then
	repo=$(cd -- "$repo" && pwd) || die "$repo is not a readable directory"
fi

root=$(git -C "$repo" rev-parse --show-toplevel 2>/dev/null) ||
	die "$repo is not a readable git checkout"

target=$log
if [ "$repo" != "$PWD" ] && [ -f "$repo/$log" ]; then
	target=$repo/$log
elif [ ! -f "$target" ] && [ -f "$repo/$log" ]; then
	target=$repo/$log
fi

case "$target" in
"$root"/*) rel=${target#"$root/"} ;;
/*) die "$target is outside $root" ;;
*) rel=$target ;;
esac

# The file's name is the normal selector. If it has no headings yet -- a fresh harness adoption, or
# --repo pointing at a branch before the log exists -- D is the product default; H is only inferred
# from factory/DECISIONS.md rather than from a prefix found in a sibling branch, because that would
# let the answer depend on branch ordering.
if [ -z "$prefix" ]; then
	case "$rel" in
	factory/DECISIONS.md) prefix=H ;;
	*) prefix=$(sed -n 's/^## \([DH]\)-[0-9][0-9][0-9][0-9].*/\1/p' "$target" 2>/dev/null |
		head -n 1) ;;
	esac
	[ -n "$prefix" ] || prefix=D
fi

highest=$(
	{
		[ -f "$target" ] && cat -- "$target"

		# HEAD first, then every branch. A remote-tracking ref may name a commit the local
		# checkout cannot open (a shallow fetch, for example); the read below is allowed to
		# ignore that one. HEAD is explicit rather than trusting refs/heads because a detached
		# checkout still has a current tree whose new entry must count.
		{
			printf 'HEAD\n'
			git -C "$root" for-each-ref --format='%(refname)' refs/heads refs/remotes 2>/dev/null
		} | sort -u | while IFS= read -r ref; do
			git -C "$root" show "${ref}^{commit}:$rel" 2>/dev/null || true
		done
	} | awk -v want="$prefix" '
	substr($0, 1, 3) == "## " {
		id = substr($0, 4, 6)
		if (id !~ /^[DH]-[0-9][0-9][0-9][0-9]$/)
			next
		if (substr(id, 1, 1) != want)
			next
		n = substr(id, 3) + 0
		if (n > highest)
			highest = n
	}
	END { print highest + 0 }
	'
)

printf '%s-%04d\n' "$prefix" "$((highest + 1))"
