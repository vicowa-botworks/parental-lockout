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
# Which half raised it. The union is read as two maxima rather than one, because its two halves answer
# different questions. A ceiling some ref carries is a sibling that took the number. A ceiling ONLY the
# working file carries is the caller's own uncommitted entry, and the id printed above it is what a
# SECOND entry would take -- not a collision. Issue #320 measured the single-maximum read in this tree
# on 12 Nov: the log as committed answers `H-0145`; the same file with an uncommitted `## H-0145`
# heading answers `H-0146`, while no ref anywhere holds that heading. The skill schedules a re-read
# immediately before the entry commit, which is precisely when the inflation is certain, and a session
# handed one unattributed number has two wrong reactions -- yield to the phantom and open a gap no
# entry explains, or distrust the tool and stop. So the ceiling is attributed on stderr, as PENDING,
# whenever the working file alone raised it, and the refs' own ceiling is printed beside it. stdout
# still carries the id alone: the number is still the best answer, merely an unreserved one.
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
#
# Silence about that refresh is what issue #303 priced, so the read now reports it. `D-0058` was written
# twice on 11 Nov, by two units, for two different decisions, on three timestamps worth having straight:
# `06b7c29` (issue-296) committed its entry at 11:32:24-0500; the second unit's own `pre-D-0058`
# checkpoint `4537b9d` (issue-297, PR #300) is dated 12:10:04-0500, 2260s later, and that is the moment
# its allocator ran, because the skill takes the id immediately before the checkpoint; `1d3470b` carried
# that unit's entry in at 12:14:46-0500, 2542s after the first entry. #303 pairs 12:10:04 with `1d3470b`,
# so the "38 minutes" the incident is cited by and the 42min22s a re-measure of the two entries gives are
# both true -- of different pairs. The tree it happened in was one where `git show-ref | grep 297` and
# `git worktree list` both answer nothing. Both allocators were right about the refs they could see;
# neither said that was all it could see. The gate found the alias where the two logs finally met.
#
# So the id is printed with the provenance of the read that produced it, and stdout keeps carrying the
# id alone. It is not withheld: a refusal in front of a session is the failure this script's own header
# already records -- "a caller reduced to 'the allocator is broken' goes back to numbering by eye" --
# and the id is still the best answer available, merely an unreserved one. Two offline verdicts are
# made, below the union read:
#
#   coverage  can `refs/remotes` here learn that a sibling exists at all? A fetch refspec that names
#             one branch updates the trunk and nothing else, and a sibling's ids are on the branch it
#             pushed. `factory/factory.sh` journals exactly this as `REFRESH-BLIND` once per activation;
#             this command, the one that answers in the unit's own tree, said nothing.
#   clock     when did this repository last hear from origin? Newest of three clocks -- this worktree's
#             FETCH_HEAD, the common dir's FETCH_HEAD, and the newest reflog under `logs/refs/remotes`.
#             No one of the three is always there: `git clone` writes no FETCH_HEAD at all (measured on
#             git 2.53.0), `git worktree add` writes none in the new tree, and a fetch that moves nothing
#             reflogs nothing. Measured in the tree this fix was written in: own FETCH_HEAD 145s, common
#             FETCH_HEAD 202s, newest remote reflog 1724s -- and `max` is what keeps the verdict from
#             crying wolf about a refresh the activation did in the primary and this tree never saw.
#
# What no offline read can see, stated because the residual is the rule's cost and not an oversight: a
# sibling that pushed AFTER the last refresh of a correctly-configured clone. `docs/factory.md` accepts
# that window with the merge as the arbiter. The bound the clock arm applies is the window the arms do
# cover -- an entry a sibling wrote and pushed since the last refresh -- and `FACTORY_ID_MAX_AGE` sets
# it. It is the read's age, not a countdown to a collision: a verdict that fired on a provably current
# read would be noise, and noise gets ignored, which is why the ticket's literal test ("no
# `refs/remotes/origin/<my branch>`") was measured and not used -- dispatch pushes no unit branch, so
# that arm is true of every allocation this harness has ever made.
set -euo pipefail

USAGE='usage: next-decision-id.sh [--repo DIR] [--prefix D|H] [DECISIONS.md]

Print the next decision-log id for DIR. The path is the log being written and selects the prefix:
factory/DECISIONS.md uses H-, any other DECISIONS.md uses D-; --prefix overrides that.

The union read is the working file, HEAD, refs/heads and refs/remotes. Blobs that do not contain the
named log, and refs git cannot read, are ignored. The result is the first free id ABOVE every id seen;
gaps below the ceiling stay closed. No fetch is performed.

stderr carries how far that union can see: REFRESH with the age of the read, or REFRESH-BLIND with
the reason a sibling could be invisible here and the refresh that would close it. stdout stays the id
alone, and the exit status stays 0 either way -- a blind read is still the best read available, and a
refusal is what sends a session back to numbering by eye. FACTORY_ID_MAX_AGE (default 300 seconds) is
how stale the read may get before the clock arm calls it blind.
stderr attributes the ceiling when the working file alone raised it: a PENDING line names the heading
sitting in that file and on no ref -- the entry you are writing -- and the highest id the refs really
do carry. It is a note, not a refusal: the exit status stays 0 and stdout stays the id alone. A re-read
taken after you wrote your own entry therefore cannot be mistaken for a sibling taking it.

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

# Two maxima out of one union pass. `W ` and `R ` tag which half of the union a heading arrived from,
# the working file or HEAD and the branch refs; the tag is stripped before anything is matched, so the
# predicate below is byte-for-byte the one the single-maximum read ran and the split adds a source, not
# a second test. The `for-each-ref` call and the two ref classes beside it are plant targets in
# `selftest.sh` -- one control per ref class -- so keep both strings on the lines they are on.
union=$(
	{
		# An `if`, not `[ -f ] && cat` -- and not because the AND-list was ever fatal. That claim
		# shipped in this branch's first draft and is corrected in H-0073: bash exempts a failing
		# command inside an `&&` list from `set -e` (`bash -c 'set -euo pipefail; out=$( { [ -f /nope
		# ] && cat /nope; echo AFTER; } | cat ); echo $out'` prints `AFTER`), and measured on `91d992d`
		# a fixture whose DECISIONS.md is in HEAD and deleted from the working tree answered `D-0008`
		# with status 0. The `if` is kept because it is the shape that says what it does -- read the
		# file when it is there, skip it when it is not -- and a leg in `selftest.sh` pins that answer
		# now instead of a rumour about it. The `sed` above this block is the command that really died:
		# under `pipefail` a pipeline hands its status to the assignment, so a missing log ended the
		# script before any message (measured on `91d992d`, the same fixture without `--prefix`: status
		# 2, stdout and stderr both empty).
		if [ -f "$target" ]; then sed 's/^/W /' -- "$target"; fi

		# HEAD first, then every branch. A remote-tracking ref may name a commit the local
		# checkout cannot open (a shallow fetch, for example); the read below is allowed to
		# ignore that one. HEAD is explicit rather than trusting refs/heads because a detached
		# checkout still has a current tree whose new entry must count.
		{
			printf 'HEAD\n'
			git -C "$root" for-each-ref --format='%(refname)' refs/heads refs/remotes 2>/dev/null
		} | sort -u | while IFS= read -r ref; do
			git -C "$root" show "${ref}^{commit}:$rel" 2>/dev/null | sed 's/^/R /' || true
		done
	} | awk -v want="$prefix" '
	{
		src = substr($0, 1, 1)
		line = substr($0, 3)
	}
	substr(line, 1, 3) == "## " {
		id = substr(line, 4, 6)
		if (id !~ /^[DH]-[0-9][0-9][0-9][0-9]$/)
			next
		if (substr(id, 1, 1) != want)
			next
		n = substr(id, 3) + 0
		if (src == "W") {
			if (n > work)
				work = n
		} else if (src == "R") {
			if (n > refs)
				refs = n
		}
	}
	END { print work + 0, refs + 0 }
	'
)

work_max=${union%% *}
refs_max=${union##* }
ceiling=$work_max
if [ "$refs_max" -gt "$ceiling" ]; then
	ceiling=$refs_max
fi

fmt_id() { printf '%s-%04d' "$prefix" "$1"; }

next=$(fmt_id "$((ceiling + 1))")

# Where the ceiling came from (issue #320). `work > refs` holds in exactly one window: the highest
# heading in the repository is the one the caller is writing, so no ref carries it and the file being
# read is the only place it exists. Commit the entry and HEAD joins the union, the two maxima agree,
# and the note goes quiet by itself -- which is what makes this a verdict about the read rather than a
# guess about what the caller meant. The id printed over a self-raised ceiling is not somebody else's
# number; it is what a second entry in the same tick would take. That has to be said, because the
# other reading is the phantom: #320 measured `H-0145` on a tree whose refs stopped at `H-0144` and
# `H-0146` once an uncommitted `## H-0145` sat in the file, and the skill's re-read is scheduled at
# precisely that moment. A session handed a bare higher number renumbers into a gap that
# `docs/factory.md` says must never be inexplicable, or stops trusting the allocator. Both are wrong,
# and the second is wrong at the only guard left before a duplicate id.
#
# stdout is unchanged and the status stays 0, for the reason the header already gives: a refusal in
# front of a session sends it back to numbering by eye. Silence is a verdict too -- a ceiling some ref
# carries raises no note, so a higher answer with nothing on stderr is the collision this command
# exists to catch, and the yield rule in `docs/factory.md` is what to do with it.
if [ "$work_max" -gt "$refs_max" ]; then
	if [ "$refs_max" -gt 0 ]; then
		ref_ceiling=$(printf 'HEAD, refs/heads and refs/remotes top out at %s' "$(fmt_id "$refs_max")")
	else
		ref_ceiling='No ref here carries a '"$prefix"' id at all'
	fi
	printf 'next-decision-id: PENDING %s is in %s and on no ref: the ceiling came from the log you are\n  writing, not from a sibling. %s.\n' \
		"$(fmt_id "$work_max")" "$rel" "$ref_ceiling" >&2
	printf 'next-decision-id:   so %s is free -- and if %s is the entry you are about to commit, %s above\n  it is what a SECOND entry would take, not a claim on yours: keep the id you already took.\n' \
		"$(fmt_id "$work_max")" "$(fmt_id "$work_max")" "$next" >&2
fi

# ------------------------------------------------------------------- how far the union reaches
#
# The union read above is the whole of what this command knows, and the two verdicts below say how far
# it reaches. They are printed on stderr, never on stdout, because the id is what callers capture: the
# skill tells a session to "use exactly what it prints", and a marker in front of the number would be
# either parsed into it or ignored -- and an ignored marker is the silence this fix replaces.
#
# Both arms are read, and both reasons are printed when both are true: the first is the one to act on,
# the second is the one that is still true afterwards. The list is the plant target of two controls in
# `selftest.sh` -- drop one arm from it and the leg that pins that arm goes red -- so neither arm can be
# retired by an edit that leaves the other looking like it still covers both shapes.
blind_max=${FACTORY_ID_MAX_AGE:-300}
case $blind_max in
'' | *[!0-9]*)
	die "FACTORY_ID_MAX_AGE must be a whole number of seconds, not '$blind_max'"
	;;
esac

blind=''
blind_why=''
blind_age=''
blind_now=$(date -u +%s)

# Can `refs/remotes` here learn that a sibling exists at all? A single-branch clone's refspec names one
# branch, so every fetch this repository makes -- however recent -- updates the trunk and nothing else,
# and a sibling's ids are on the branch it pushed (H-0071, measured: `git fetch --prune origin main`
# leaves the allocator on its stale answer and still prints a line with `->` in it). The `*` test reads
# the whole refspec, not its prefix, matching `factory_refresh_origin`: the wildcard form
# `+refs/heads/*:refs/remotes/origin/*` starts with a star, and a test that asked only about the front
# of the line would call the healthy shape blind. An absent or unreadable refspec is not called blind
# here either, for the same reason -- the arms that report a real shape (no origin, no remote refs, a
# stale clock) name what they found, and this one would only be guessing.
blind_coverage() {
	local rs n
	if ! git -C "$root" remote get-url origin >/dev/null 2>&1; then
		blind='this checkout has no origin remote, so refs/remotes here holds nothing any other machine has ever pushed'
		return 1
	fi
	rs=$(git -C "$root" config --get-all remote.origin.fetch 2>/dev/null) || rs=''
	case $rs in
	'' | *\**) : ;;
	*)
		blind="this clone fetches only '$rs', so refs/remotes here can never learn a branch another clone pushed -- widen it with: git -C $root config remote.origin.fetch '+refs/heads/*:refs/remotes/origin/*'"
		return 1
		;;
	esac
	n=$(git -C "$root" for-each-ref --format='%(refname)' refs/remotes/origin 2>/dev/null |
		grep -c . || true)
	if [ "${n:-0}" -eq 0 ]; then
		blind='this clone holds no refs/remotes/origin ref, so the union read is local-only and a sibling cut from another clone is not in it'
		return 1
	fi
	return 0
}

# When did this repository last hear from origin? Newest of three clocks, because each is missing in a
# shape the others cover: `git clone` and `git worktree add` write no FETCH_HEAD (measured on 2.53.0, and
# it is why a clock built on FETCH_HEAD alone calls a fresh clone blind), the activation's fetch runs in
# the primary and writes only the common dir's FETCH_HEAD (H-0075 -- the refresh a tick gets without
# asking for it), and a fetch that moves nothing writes no reflog entry. Remote refs are reflogged in
# the common dir, so a fetch from any worktree of this clone is evidence for every one of them: the
# refs those clocks describe are the shared `refs/remotes`, which is exactly the union read above.
#
# The bound selects the risk window; it does not close it. A read N seconds old cannot see a push that
# landed inside those N seconds, and no offline read can tell "nothing new anywhere" from "new, and I
# have not looked" -- `selftest.sh` pins that limit on a fixture whose read is 0s old and still a
# duplicate, because a leg that pins only what a guard catches is a leg that lets the guard be read as
# a guarantee. Default 300s, the shortest realistic cycle for a sibling to write an entry and push it,
# chosen over 900s on the tie-break that takes the option which refuses rather than proceeding: #303's
# second allocator ran 2260s after the entry that made the id taken, and its own entry landed 2542s after
# that first one, so both bounds catch that shape, and the cost of the smaller one is one
# bounded `git fetch` per entry commit (0.679s measured for the no-op case against this repository's
# origin on 11 Nov, re-measured 0.53-0.60s three times the same day, against `FACTORY_FETCH_TIMEOUT`'s
# 120s ceiling) while the benefit is a narrower window in every direction.
blind_clock() {
	local gitdir common f m newest=0
	gitdir=$(git -C "$root" rev-parse --absolute-git-dir 2>/dev/null) || gitdir=''
	# `--path-format` is what makes this absolute in a linked worktree; the fallback keeps an older git
	# answering, and a relative common dir is relative to `$root` because that is where `-C` put git.
	common=$(git -C "$root" rev-parse --path-format=absolute --git-common-dir 2>/dev/null) ||
		common=$(git -C "$root" rev-parse --git-common-dir 2>/dev/null) || common=''
	case $common in
	'' | /*) : ;;
	*) common=$root/$common ;;
	esac
	for f in "$gitdir/FETCH_HEAD" "$common/FETCH_HEAD"; do
		[ -f "$f" ] || continue
		m=$(stat -c %Y -- "$f" 2>/dev/null) || continue
		case $m in
		'' | *[!0-9]*) continue ;;
		esac
		if [ "$m" -gt "$newest" ]; then
			newest=$m
		fi
	done
	if [ -d "$common/logs/refs/remotes" ]; then
		m=$(find "$common/logs/refs/remotes" -type f -printf '%T@\n' 2>/dev/null |
			cut -d. -f1 | sort -nr | head -n 1) || m=''
		case $m in
		'' | *[!0-9]*) : ;;
		*)
			if [ "$m" -gt "$newest" ]; then
				newest=$m
			fi
			;;
		esac
	fi
	if [ "$newest" -eq 0 ]; then
		blind='nothing in this checkout records a contact with origin -- no FETCH_HEAD and no refs/remotes reflog -- so the union is only what this tree was cut from'
		return 1
	fi
	blind_age=$((blind_now - newest))
	if [ "$blind_age" -lt 0 ]; then
		# A clock behind the filesystem (an NTP step, a restored tree). An age that reads negative is
		# not evidence of a fresh read either, so it is reported as zero rather than hidden.
		blind_age=0
	fi
	if [ "$blind_age" -gt "$blind_max" ]; then
		blind="the refs behind this answer were last refreshed ${blind_age}s ago, past the ${blind_max}s bound -- an entry a sibling wrote and pushed since then is not in the union"
		return 1
	fi
	return 0
}

for arm in coverage clock; do
	blind=''
	if ! blind_$arm; then
		if [ -n "$blind_why" ]; then
			blind_why="$blind_why; $blind"
		else
			blind_why=$blind
		fi
	fi
done

if [ -n "$blind_why" ]; then
	printf 'next-decision-id: REFRESH-BLIND %s\n' "$blind_why" >&2
	printf 'next-decision-id:   so %s is free only as far as this clone can see. Refresh, then re-read before the entry is committed: git -C %s fetch --prune origin\n' \
		"$next" "$root" >&2
else
	printf 'next-decision-id: REFRESH the union behind %s is %ss old (bound %ss); a sibling that pushed since then is invisible here\n' \
		"$next" "$blind_age" "$blind_max" >&2
fi

printf '%s\n' "$next"
