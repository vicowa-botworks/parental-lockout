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
# What is never allocated. An id is printed only for a path that IS a decision log: named
# DECISIONS.md, and a regular file in the working tree or a blob in HEAD. A union with no headings in
# it is not an empty log, it is a wrong path, and answering it prints <prefix>-0001 -- the most-cited
# number in either log, so a typo would be answered with a guaranteed collision (issue #253, measured:
# `--prefix H factory/DECISION.md` printed H-0001 and exited 0). Existence is not the test, because
# every wrong path in a checkout exists: at `c5aa1eb` `--prefix H docs/factory.md`, `README.md`,
# `AGENTS.md`, `factory` and `factory/` each printed H-0001 and exited 0 (H-0076). A fresh adoption is
# one `touch` away and still gets its honest D-0001, from headings the log does not have yet rather
# than from silence.
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
gaps below the ceiling stay closed. No fetch is performed.

Only the decision log itself is answered: DECISIONS.md or factory/DECISIONS.md, as a regular file in the
working tree or a blob in HEAD. Any other path is refused, including one that exists -- allocating from
an empty union means answering <prefix>-0001, a taken id in every log this repository has.'

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

# `--repo` names the repository the log lives in, and it is also what a relative log path is read
# against -- there is no second reading. The first version resolved the path against the caller and
# reached for `$repo/$log` only when the caller's was missing. That is not a fallback, it is a
# cross-repo read: measured on a fixture repository with no log in it, `--repo <fixture>
# DECISIONS.md` printed D-0052, an id out of THIS repository's log -- once with a commit and once
# with none. Every selftest fixture writes its log before the leg runs, which is why seven legs and
# four controls never stood where the fallback was. `pwd` after `cd` is the physical path, so a
# caller in a symlinked tree resolves the way its own repository does.
if [ -d "$repo" ]; then
	repo=$(cd -- "$repo" && pwd) || die "$repo is not a readable directory"
fi

root=$(git -C "$repo" rev-parse --show-toplevel 2>/dev/null) ||
	die "$repo is not a readable git checkout"

case $log in
/*) target=$log ;;
*) target=$repo/$log ;;
esac

case "$target" in
"$root"/*) rel=${target#"$root/"} ;;
/*) die "$target is outside $root" ;;
*) rel=$target ;;
esac

# The union has to contain the log. An empty one has exactly one answer available, <prefix>-0001, and
# that is the most-cited number either log has -- so a mistyped path would be answered with a
# guaranteed collision, the thing this command exists to refuse. Asking for a fresh log is one `touch`
# and a title line, and it still gets its honest D-0001. `git cat-file` asks about an object and never
# about a ref, so this stays true of the tree as it sits on disk and off every fetch path (H-0056).
#
# Two tests, and the question is "is this the log", not "is anything sitting there" (H-0076). The name
# first: `docs/factory.md` -- the file this command's own documentation lives in -- `README.md` and
# `AGENTS.md` are regular files, and `factory` is a directory in the tree and a tree in HEAD, and at
# `c5aa1eb` each of those four was answered `H-0001` with exit 0. Then the type, because a name alone
# still admits a directory: `git cat-file -e` is true of a tree, so a checkout holding `DECISIONS.md/`
# as a directory would be answered too.
#
# Both tests are inside this function rather than beside it, and that is the control's requirement, not
# a style choice. The shipped control neuters the guard by rewriting its call site (`selftest.sh`'s
# `s,^log_is_here ||,: log_is_here ||,`); a name test written in front of the function is therefore
# invisible to every plant the suite has, and the leg that pins it would be an ok nothing can falsify.
log_is_here() { # -> 0 when the named path IS a decision log; sets $why to the reason it is not
	why=''
	case ${rel##*/} in
	DECISIONS.md) ;;
	*)
		why="that is not a decision log's name -- this repository keeps its ids in DECISIONS.md and factory/DECISIONS.md and reads nothing else"
		return 1
		;;
	esac
	# The type HEAD holds at that path: `blob` for a file, `tree` for a directory, empty for nothing.
	# Comparing it is the whole second test -- the predicate this branch replaced asked only whether
	# *anything* was there (`cat-file -e`), which a directory named DECISIONS.md satisfies. The suite's
	# type control plants that predicate back, so the leg is known to be decided by the verdict.
	head_kind=$(git -C "$root" cat-file -t "HEAD:$rel" 2>/dev/null || true)
	if [ -f "$target" ] || [ "$head_kind" = blob ]; then
		return 0
	fi
	why='it is neither a regular file in the working tree nor a blob in HEAD'
	return 1
}
log_is_here ||
	die "no decision log at $rel in $root: ${why:-it is not the decision log}. An empty union is a wrong path, not a free log: it answers with the first id of the range (D-0001 or H-0001), which every log this repository has already used. Name DECISIONS.md or factory/DECISIONS.md, or create the file first."

# The file's name is the normal selector. If it has no headings yet -- a fresh harness adoption -- D is
# the product default; H is only inferred from factory/DECISIONS.md rather than from a prefix found in
# a sibling branch, because that would let the answer depend on branch ordering. The `|| true` is
# load-bearing and not decoration: this substitution was the script's whole failure mode. Under
# `set -euo pipefail` a missing file makes `sed` exit 2, a command-substitution assignment inherits
# that status, and `set -e` ends the script there -- no id, no message, nothing on stderr. A caller
# reduced to "the allocator is broken" goes back to numbering by eye, which is the defect #253 is about.
if [ -z "$prefix" ]; then
	case "$rel" in
	factory/DECISIONS.md) prefix=H ;;
	*) prefix=$(sed -n 's/^## \([DH]\)-[0-9][0-9][0-9][0-9].*/\1/p' "$target" 2>/dev/null |
		head -n 1 || true) ;;
	esac
	[ -n "$prefix" ] || prefix=D
fi

highest=$(
	{
		# An `if`, not `[ -f ] && cat`: a log that is in HEAD but not in the working tree makes that
		# AND-list false, and under `set -e` a false list is a status, which is the same trap the
		# prefix inference above documents.
		if [ -f "$target" ]; then cat -- "$target"; fi

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
