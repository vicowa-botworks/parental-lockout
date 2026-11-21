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
# local refs -- the working tree, HEAD, every refs/heads ref and every refs/remotes ref. It is not the
# lowest unused number. The logs deliberately retain gaps: H-0031 and H-0032 were retired when the
# harness prefixes were split from D ids, and H-0057 is a permanent gap created by a documented
# renumbering. A "smallest gap" allocator would recycle those numbers and turn every historical
# citation into an alias.
#
# What the union reads depends on the layout the caller named, and that is the whole of #456: a monolith
# is one blob, so `show HEAD:DECISIONS.md` is the whole log, but a filed log (#429, H-0263) is a
# directory and a name read as a blob is nothing at all. So a filed path is walked -- every *.md under it
# in the working tree, and every *.md the ref's tree carries -- for the same maximum. The alternative is
# the collision #442 counts: an id reserved only as `factory/DECISIONS/H-0277.md` on a sibling's branch
# is invisible to a union that reads one path, and both units are then handed the same number in good
# faith by a tool that exists to refuse it.
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
# DECISIONS.md and a regular file in the working tree or a blob in HEAD, or named DECISIONS and a
# directory in the working tree or a tree in HEAD -- the filed spelling #429 phase 1 gave the same logs.
# A union with no headings in
# it is not an empty log, it is a wrong path, and answering it prints <prefix>-0001 -- the most-cited
# number in either log, so a typo would be answered with a guaranteed collision (issue #253, measured:
# `--prefix H factory/DECISION.md` printed H-0001 and exited 0). Existence is not the test, because
# every wrong path in a checkout exists: at `c5aa1eb` `--prefix H docs/factory.md`, `README.md`,
# `AGENTS.md`, `factory` and `factory/` each printed H-0001 and exited 0 (H-0076). A fresh adoption is
# one `touch` away and still gets its honest D-0001, from headings the log does not have yet rather
# than from silence. The two names are three characters apart and the two verdicts must not be
# confusable: the type test runs per layout, so a directory called `DECISIONS.md` is still refused and a
# directory called `DECISIONS` is still read. A trailing slash belongs to that same pair, because a slash
# is a claim about the face: `DECISIONS/` is the filed log with its shell-completed slash and is read,
# while `DECISIONS.md/` asks for a directory by the monolith's name and is refused (#456).
#
# Where the entry goes. An answer is a number and a destination, so stderr names the destination too:
# `EMITS` is the entry file to create -- the log's filed directory plus the id -- whichever face the
# caller named, because from #456 phase 2 a NEW entry is a file in either log while the monoliths keep
# only the entries that predate the instruction. That sentence is the only thing telling a unit which
# of the two layouts AGENTS.md means -- write the entry where the allocator says, and do not hand-file
# an existing entry into the other one.
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
#   coverage  can `refs/remotes` here learn that a sibling exists at all? The test is per line, on the
#             SOURCE side of the colon: only a line that fetches `refs/heads/*` can put another machine's
#             branch in the union. A refspec naming one branch updates the trunk and nothing else, and a
#             sibling's ids are on the branch it pushed; a star on the destination side
#             (`+refs/pull/*/head:refs/remotes/origin/pr/*`) fetches no branches either, which #309
#             measured as a clean read. `factory/factory.sh` journals exactly this as `REFRESH-BLIND`
#             once per activation, reading the same rule; this command, the one that answers in the
#             unit's own tree, said nothing.
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
#
# The answer is journalled when a unit was answered. #345 measured that neither unit of the `D-0058`
# collision ran this command at all -- each numbered by eye -- and nothing anywhere could tell an
# issued id from an eyeballed one. So a run that carries the dispatch's `FACTORY_TICK_UNIT` and
# `FACTORY_LOG` appends one `DECISION-ID` line to the supervisor's journal, and the end-of-tick pass
# compares every newly present heading against it (`factory_decision_provenance`, common.sh). A bare
# run -- a person, a test -- has no unit to journal for and writes nothing; a failed write is said on
# stderr and changes neither the id nor the exit.
#
# The same journal is READ before an answer is handed out (#442). #345 made an issued id visible; it
# did not make the allocator consume one: the taken-set is log headings only, so an id this command
# had just journalled to one unit sat below the ceiling invisible to the next unit's read, and three
# units were handed `H-0250` in 53 minutes. Every candidate from the ceiling up is now asked of that
# journal: the LAST line naming the id is the claim, and a live one -- another unit's, on a branch
# that still exists as a ref here -- steps the answer up and says whose id was skipped. A claim whose
# branch exists nowhere is expired: the entry it would have committed was never written, and a claim
# that never expires lets dead ticks walk the id space upward (issue-365 earned `H-0355` and
# never acquired a branch). A claim belonging to the unit being answered keeps its own id -- the
# cross-unit collision is what is refused, #320's same-unit re-read is what is preserved. This is a
# READ, offline and failure-blind like every other arm here, not the claim-time lease `H-0195`
# refused: a journal this run cannot read, or a claim taken on another machine, leaves the window
# exactly as wide as `REFRESH-BLIND` says, and the merge stays the arbiter of what this cannot see.
set -euo pipefail

USAGE='usage: next-decision-id.sh [--repo DIR] [--prefix D|H] [DECISIONS.md|DECISIONS]

Print the next decision-log id for DIR. The path is the log being written and selects both the prefix
and the layout: a path under factory/ uses H-, any other uses D-; --prefix overrides that. A name of
DECISIONS.md is a monolith (one file, headings appended); a name of DECISIONS is a filed log (a
directory, one entry per file). Both layouts are answered until #429 phase 3 retires the monoliths.

The union read is the working file, HEAD, refs/heads and refs/remotes -- for a filed log that is
every *.md under the directory in each, so an entry a sibling filed on a branch is counted as the claim
on its id that it is. Blobs that do not contain the named log, and refs git cannot read, are ignored.
The result is the first free id ABOVE every id seen; gaps below the ceiling stay closed. No fetch is
performed. The journal at FACTORY_LOG (default
DIR/factory/log) is then asked for live claims: an id whose newest DECISION-ID line belongs to a
different unit on a branch that still exists as a ref here is taken and stepped over, whatever the
headings say, and the answer names on stderr what it stepped over and whose it was.

stderr carries how far that union can see: REFRESH with the age of the read, or REFRESH-BLIND with
the reason a sibling could be invisible here and the repair that would close it -- a fetch when the
read is stale, a git config remote.origin.fetch write when a refspec cannot fetch a branch, because
a fetch cannot widen a refspec. EMITS names the entry file to create -- the filed directory of the
log plus the id, e.g. factory/DECISIONS/H-0042.md -- whichever face the caller named, because a new
entry is a file from #456 phase 2. stdout stays the id alone, and the exit status stays 0
either way -- a blind read is still the best read available, and a refusal is what sends a session
back to numbering by eye. FACTORY_ID_MAX_AGE (default 300 seconds) is how stale the read may get
before the clock arm calls it blind.
stderr attributes the ceiling when the working file alone raised it: a PENDING line names the heading
sitting in that file and on no ref this clone can see -- the entry you are writing -- and the highest id
the refs really do carry. It is a note, not a refusal: the exit status stays 0 and stdout stays the id
alone. A re-read taken after you wrote your own entry therefore cannot be mistaken for a sibling taking
it. The note being absent is the verdict on the other half: since the refresh arms print beside every
answer, it is the note that is read, never an empty stderr.

Only the decision log itself is answered: DECISIONS.md and factory/DECISIONS.md as a regular file in the
working tree or a blob in HEAD, and DECISIONS and factory/DECISIONS as a directory in the working tree or
a tree in HEAD. Any other path is refused, including one that exists -- allocating from an empty union
means answering <prefix>-0001, a taken id in every log this repository has.'

die() {
	printf 'next-decision-id: %s\n' "$*" >&2
	exit 2
}

repo=$PWD
log=''
prefix=''
# Which layout the caller named: `monolith` (one file, headings appended) or `filed` (a directory, one
# file per entry). Set by `log_is_here` from the name; the default is what every caller before #456 asked
# for, so a call that never reaches the guard cannot read it as a verdict about a filed log.
layout=monolith
# Set when the caller named a face of the log that is not itself present and the *other* face is: the
# filed question asked of a log still living in one file, or the old path asked of a log already split.
# It changes what the answer is for, not whether there is one.
other_rel=''
other_kind=''
other_here=no

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
# A filed log IS a directory, and `DECISIONS/` and `DECISIONS` are one request: the slash is what a
# person types when they mean a directory, and `${rel##*/}` on `DECISIONS/` yields the empty string, so
# the strip belongs here once, before any test reads the basename. One slash only, because a path ending
# in two is a typo and still has to be refused for what it is.
#
# The strip is remembered, because the slash is a claim about the face and not decoration. Stripping it
# quietly made `DECISIONS.md/` -- a request for a *directory* named after the monolith, which no layout
# ever writes -- read as the monolith: the review caught it, and it is one character away from the
# `DECISIONS.md` decoy `log_is_here` exists to refuse (H-0076). #456 asks for that pair in both
# directions, one leg per direction, so the flag travels to the guard instead of vanishing here.
log_slash=0
case "$log" in */) log_slash=1 ;; esac
log=${log%/}

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

# One log, two faces. #429 splits each log into a directory of entries without retiring the monolith, so
# until phase 3 `DECISIONS.md` and `DECISIONS/` are one id space -- the gate already enforces that
# reading: `cross_log_problems` refuses an id used in both forms and names both locations. The stem is the
# coordinate the two faces share, and every union read below goes through both of them. An allocator that
# counted only the face the caller named would answer a number the other face has already taken, which is
# exactly the collision #442 priced, wearing the migration's clothes.
case "$rel" in
*DECISIONS.md) stem=${rel%.md} ;;
*) stem=$rel ;;
esac

# Is this face of the log present -- as a regular file or a directory in the working tree, or as a blob or
# a tree in HEAD? The two tests are the guard's own, because the guard's lesson (H-0076) is that "present"
# has to be a type question: `git cat-file -e` answers yes to a directory and to a gitlink alike.
face_here() { # <root-relative path> <file|dir> -> 0 when that face is the log somewhere
	local relx=$1 kind=$2 kind_head
	kind_head=$(git -C "$root" cat-file -t "HEAD:$relx" 2>/dev/null || true)
	if [ "$kind" = dir ]; then
		if [ -d "$root/$relx" ] || [ "$kind_head" = tree ]; then
			return 0
		fi
		return 1
	fi
	if [ -f "$root/$relx" ] || [ "$kind_head" = blob ]; then
		return 0
	fi
	return 1
}

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
	local name=${rel##*/}
	# Both accepted names in one arm, so the suite's control that opens this guard to every basename has
	# one line to plant; everything below reads the name this guard accepted and decides nothing about
	# whether there is a log.
	case $name in
	DECISIONS.md | DECISIONS) ;;
	*)
		why="that is not a decision log's name -- this repository keeps its ids in DECISIONS.md and factory/DECISIONS.md, or in the filed directories DECISIONS and factory/DECISIONS beside them, and reads nothing else"
		return 1
		;;
	esac
	# The one-character twin of the decoy below: a *trailing slash* on the monolith's name asks for a
	# directory called `DECISIONS.md`, and no layout puts a directory there -- phase 1 files entries as
	# `DECISIONS/<id>.md`, never inside a directory named after the monolith. Without this test the
	# `${log%/}` above turns that ask into the monolith and answers an id for it (measured with this test
	# deleted: `H-0023`, exit 0), which is the acceptance #456 refuses while the decoy stays refused.
	if [ "$name" = DECISIONS.md ] && [ "$log_slash" = 1 ]; then
		why="that is not a decision log's name -- the trailing slash asks for a directory, and the monolith is a file; the filed logs are the directories DECISIONS and factory/DECISIONS beside them"
		return 1
	fi
	# The filed layout (#429, H-0263) is a directory of entries, one file per id, and the gate has read
	# it since phase 1. Accepting the name here is what makes a filed entry a claim: the alternative is
	# the collision #442 counts, where the union cannot see an id because the union reads one path.
	if [ "$name" = DECISIONS ]; then
		layout=filed
	fi
	# The OTHER face of the same log -- the monolith when a directory was named, the filed directory
	# when a file was -- is what makes a not-yet-present face answerable: asking the filed question
	# before anyone has filed anything is phase 2's first entry. It is asked about here rather
	# than beside the function for the same reason the two existing tests are: the shipped control neuters
	# this function by its call site, so a test written outside it is invisible to every plant.
	if [ "$layout" = filed ]; then
		other_rel=$stem.md
		other_kind="file"
	else
		other_rel=$stem
		other_kind="dir"
	fi
	other_here=no
	if face_here "$other_rel" "$other_kind"; then
		other_here=yes
	fi
	# The type HEAD holds at that path: `blob` for a file, `tree` for a directory, empty for nothing.
	# Comparing it is the whole second test -- the predicate this branch replaced asked only whether
	# *anything* was there (`cat-file -e`), which a directory named DECISIONS.md satisfies. The suite's
	# type control plants that predicate back, so the leg is known to be decided by the verdict.
	head_kind=$(git -C "$root" cat-file -t "HEAD:$rel" 2>/dev/null || true)
	if [ "$layout" = filed ]; then
		# A filed log is a directory in the tree or a tree in HEAD, and only the basename `DECISIONS`
		# reaches this arm: the decoy the monolith arm below exists for -- a *directory* named
		# `DECISIONS.md` -- is refused by that arm, exactly as it always was. Both directions are legs
		# in `selftest.sh`, because the two names differ by three characters and one wrong verdict is a
		# silent collision in one direction and a refusal-to-work in the other.
		if [ -d "$target" ] || [ "$head_kind" = tree ]; then
			return 0
		fi
		# The directory a log will have, not filed yet: asking the filed question before anyone has filed
		# anything is phase 2's first entry, and the union behind it is not empty -- it is the monolith
		# beside it. The refusal below stays the answer when neither face exists, which is the case the
		# empty-union refusal was written for.
		if [ "$other_here" = yes ]; then
			return 0
		fi
		why='it is neither a directory in the working tree nor a tree in HEAD'
		return 1
	fi
	if [ -f "$target" ] || [ "$head_kind" = blob ]; then
		return 0
	fi
	# The symmetric case: the monolith was retired on disk and only the entries are left. Naming the old
	# path must not refuse the pass; the id space is the log's, not one file's.
	if [ "$other_here" = yes ]; then
		return 0
	fi
	why='it is neither a regular file in the working tree nor a blob in HEAD'
	return 1
}
log_is_here ||
	die "no decision log at $rel in $root: ${why:-it is not the decision log}. An empty union is a wrong path, not a free log: it answers with the first id of the range (D-0001 or H-0001), which every log this repository has already used. Name DECISIONS.md, factory/DECISIONS.md, DECISIONS or factory/DECISIONS, or create the log first."

# The file's name is the normal selector. If it has no headings yet -- a fresh harness adoption -- D is
# the product default; H is only inferred from factory/DECISIONS.md rather than from a prefix found in
# a sibling branch, because that would let the answer depend on branch ordering. The `|| true` is
# load-bearing and not decoration: this substitution was the script's whole failure mode. Under
# `set -euo pipefail` a missing file makes `sed` exit 2, a command-substitution assignment inherits
# that status, and `set -e` ends the script there -- no id, and no word on either stream. A caller
# reduced to "the allocator is broken" goes back to numbering by eye, which is the defect #253 is about.
if [ -z "$prefix" ]; then
	case "$rel" in
	factory/DECISIONS.md) prefix=H ;;
	# The filed spelling of the same log takes its prefix from the same fact -- it stands in the
	# `factory/` tree -- and not from a heading read out of whichever entry the directory listed first.
	factory/DECISIONS) prefix=H ;;
	*)
		if [ "$layout" = filed ]; then
			# The root directory is the root log's filed form, so D, which is also the product default
			# two lines below; it is written out because the alternative is running `sed` over a
			# directory to learn nothing.
			prefix=D
		else
			prefix=$(sed -n 's/^## \([DH]\)-[0-9][0-9][0-9][0-9].*/\1/p' "$target" 2>/dev/null |
				head -n 1 || true)
		fi
		;;
	esac
	[ -n "$prefix" ] || prefix=D
fi

# Two maxima out of one union pass. `W ` and `R ` tag which half of the union a heading arrived from,
# the working file or HEAD and the branch refs; the tag is stripped before anything is matched, so the
# predicate below is byte-for-byte the one the single-maximum read ran and the split adds a source, not
# a second test. The `for-each-ref` call and the two ref classes beside it are plant targets in
# `selftest.sh` -- one control per ref class -- so keep both strings on the lines they are on.
#
# Both faces of the log are read, always: the monolith blob and the filed directory, whichever one the
# caller named. That is #456's requirement in one line -- the id space belongs to the log, and during the
# migration (#429) the log has two spellings -- and the two readers below are the only places the
# difference between them is expressed. Each face is read twice, in the working tree and in every ref,
# because a commit on any branch is a claim on an id and the working tree is the claim this unit has not
# made yet.
#
# Two shapes here carry history that was measured wrong once and is measured right now: the `if` around
# the working-tree read, and the tolerance on a ref that will not open. Both are corrected in H-0073, and
# a leg in `factory/selftest.sh` pins each answer instead of a rumour about it.
face_work() { # <root-relative path> <file|dir> -- one face of the log in the working tree, tagged
	local relx=$1 kind=$2 f
	if [ "$kind" = dir ]; then
		# Every entry file the directory holds, the caller's own uncommitted entry included -- that is
		# the working-tree half of the union in this layout, and the PENDING note below is what reads
		# it. `find` rather than a glob, because a glob with nothing to match hands `sed` the literal
		# `DECISIONS/*.md` and dies on it. The `|| true` on the pipeline is load-bearing, not
		# decoration, and for the reason H-0073 just above: under `pipefail` the assignment inherits
		# the pipeline's status, so one entry file deleted between the listing and the read would end
		# the script with no id and no word on either stream.
		if [ -d "$root/$relx" ]; then
			find "$root/$relx" -type f -name '*.md' -print 2>/dev/null |
				while IFS= read -r f; do sed 's/^/W /' -- "$f" || true; done || true
		fi
	elif [ -f "$root/$relx" ]; then sed 's/^/W /' -- "$root/$relx"; fi
}

# The same face in one ref. For a monolith that is the single `git show` this command has always run; for
# a filed log it is one `git show` per entry, and that listing is the half the old read was blind to. The
# blindness has a price list: an id that exists only as `factory/DECISIONS/H-0277.md` on a sibling's
# branch is a claim on that id, and #442 measured 7 of 31 ids reaching two or more branches while the
# union could not see the second copy. An entry filed below the prefix is counted here too -- refusing the
# nested file is the gate's job (`layout_problems`), not this read's.
face_ref() { # <ref> <root-relative path> <file|dir> -- one face as that ref carries it, tagged
	local ref=$1 relx=$2 kind=$3 p
	if [ "$kind" = dir ]; then
		# The directory as a plain pathspec, then the `.md` filter in the pipeline. The wildcard form
		# `"$relx/*.md"` -- what phase 1 shipped -- matches NOTHING here, measured on git 2.53.0 against
		# a ref that provably carries `factory/DECISIONS/H-0007.md`: the same `ls-tree` with the bare
		# directory as its pathspec prints the row, and `:(glob)` magic is refused outright
		# (`pathspec magic not supported by this command`). So a wildcard pathspec is not merely slower,
		# it is silent: every filed id on every sibling ref was invisible, which is the exact blindness
		# #442 priced at 7 of 31 ids. A ref git cannot open and a tree with no such directory contribute
		# nothing and say nothing: same tolerance as the monolith arm has always had, same reason --
		# `set -e` would end the script over one ref this clone cannot serve.
		git -C "$root" ls-tree -r --name-only "${ref}^{commit}" -- "$relx" 2>/dev/null |
			grep '\.md$' |
			while IFS= read -r p; do
				git -C "$root" show "${ref}^{commit}:$p" 2>/dev/null || true
			done | sed 's/^/R /' || true
	else
		git -C "$root" show "${ref}^{commit}:$relx" 2>/dev/null | sed 's/^/R /' || true
	fi
}

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
		face_work "$stem" dir
		face_work "$stem.md" file

		# HEAD first, then every branch. A remote-tracking ref may name a commit the local
		# checkout cannot open (a shallow fetch, for example); the read below is allowed to
		# ignore that one. HEAD is explicit rather than trusting refs/heads because a detached
		# checkout still has a current tree whose new entry must count.
		{
			printf 'HEAD\n'
			git -C "$root" for-each-ref --format='%(refname)' refs/heads refs/remotes 2>/dev/null
		} | sort -u | while IFS= read -r ref; do
			face_ref "$ref" "$stem" dir
			face_ref "$ref" "$stem.md" file
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

# ------------------------------------------------------------- the journal's live claims (#442)
#
# The ceiling above counts log HEADINGS, so an id another unit was handed -- journalled, not yet
# committed -- sits below it invisible, which is how 53 minutes delivered `H-0250` three times. Ask
# the journal before answering: from the ceiling up, the LAST `DECISION-ID` line naming a candidate
# is that id's claim; a claim of a DIFFERENT unit whose branch still exists as a ref here holds the
# id -- step over it and say whose was skipped (`H-0251 (H-0250 taken: journalled ... by ...)`
# travels in the journalled line, and a STEPPED-OVER line says it on stderr). A claim whose branch
# exists nowhere is expired, because the entry it would have committed was never written:
# issue-365 earned `H-0355`, died at the tick ceiling, and no branch ever followed. A claim of THIS
# unit keeps its own id, so #320's re-read after writing an entry still answers the number earned.
# Failure-blind and offline like every other arm: an unreadable journal answers with no claims, and
# the merge stays the arbiter of what a local read cannot see.
claims_dir=${FACTORY_LOG:-$root/factory/log}
claims_note=''

claim_branch_alive() { # -> 0 when a journalled claim's branch exists as a ref here; a claim lives no longer than its branch
	case ${1:-} in
	'' | unknown | HEAD) return 1 ;;
	esac
	git -C "$root" rev-parse -q --verify "refs/heads/$1" >/dev/null 2>&1 && return 0
	git -C "$root" rev-parse -q --verify "refs/remotes/origin/$1" >/dev/null 2>&1
}

claim_scan() { # steps `next` over live journal claims; records the trail in claims_note; never fails the answer
	local claims_map claim_n claim_row claim_id claim_unit claim_branch claim_ts
	[ -r "$claims_dir/supervisor.log" ] || return 0
	claims_map=$(awk -v want="$prefix" '
		$2 == "DECISION-ID" && $4 ~ /^unit=/ && $5 ~ /^branch=/ && substr($3, 1, 1) == want {
			last_u[$3] = substr($4, 6)
			last_b[$3] = substr($5, 8)
			last_t[$3] = $1
		}
		END {
			for (claim_scan_id in last_u)
				printf "%s\t%s\t%s\t%s\n", claim_scan_id, last_u[claim_scan_id], \
					last_b[claim_scan_id], last_t[claim_scan_id]
		}
	' "$claims_dir/supervisor.log" 2>/dev/null) || claims_map=''
	[ -n "$claims_map" ] || return 0
	claim_n=$((ceiling + 1))
	while :; do
		claim_id=$(fmt_id "$claim_n")
		claim_row=$(printf '%s\n' "$claims_map" | awk -F'\t' -v id="$claim_id" '$1 == id { print; exit }')
		[ -n "$claim_row" ] || break
		IFS=$'\t' read -r claim_id claim_unit claim_branch claim_ts <<<"$claim_row"
		if [ "$claim_unit" = "${FACTORY_TICK_UNIT-}" ]; then
			break
		fi
		claim_branch_alive "$claim_branch" || break
		if [ -n "$claims_note" ]; then
			claims_note="$claims_note; $claim_id taken: journalled $claim_ts by $claim_unit on branch $claim_branch"
		else
			claims_note="$claim_id taken: journalled $claim_ts by $claim_unit on branch $claim_branch"
		fi
		printf 'next-decision-id: STEPPED-OVER %s -- journalled %s by %s, whose branch %s still exists\n' \
			"$claim_id" "$claim_ts" "$claim_unit" "$claim_branch" >&2
		claim_n=$((claim_n + 1))
	done
	next=$(fmt_id "$claim_n")
}

# claim_scan ends 0 by construction -- it is failure-blind like every other arm -- so it needs no
# `|| :` guard, and the trail it records must be set in this shell, beside the answer it describes.
claim_scan # the call is the read: the journal is asked before the answer is handed out

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
# front of a session sends it back to numbering by eye. The absence of this note is a verdict too -- a
# ceiling some ref carries raises none, so a higher answer with no PENDING line is the collision this
# command exists to catch, and the yield rule in `docs/factory.md` is what to do with it. Read the
# note's absence and never an empty stderr: since PR #307 the two arms below print beside every answer
# and nothing silences them, so there is no answer that leaves stderr empty -- measured on one fixture
# at `487d572`, 460 bytes with the tree clean, 855 with the caller's own heading uncommitted, 460 one
# commit later, and only the middle number moves when this wording does. Nor would silence discriminate
# if something produced it: on that fixture a ceiling a sibling left on `refs/heads` and a ceiling the
# caller's own commit put on `HEAD` both answer `H-0042` with the note absent and identical stderr.
# Durable includes yours, so a committed heading means take the next id rather than yield the one
# already written; `docs/factory.md` carries both shapes as measured rows.
if [ "$work_max" -gt "$refs_max" ]; then
	if [ "$refs_max" -gt 0 ]; then
		ref_ceiling=$(printf 'HEAD, refs/heads and refs/remotes top out at %s' "$(fmt_id "$refs_max")")
	else
		ref_ceiling='No ref here carries a '"$prefix"' id at all'
	fi
	printf 'next-decision-id: PENDING %s is in %s and on no ref this clone can see -- so this read\n  says the ceiling came from the log you are writing, not from a sibling. %s.\n' \
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

# Can `refs/remotes` here learn that a sibling exists at all? The question is what the clone fetches
# FROM, so the test is a prefix on the SOURCE side of the colon, one per line: only a line that fetches
# `refs/heads/*` can put another machine's branch into the union. A single-branch clone's refspec names
# one branch, so every fetch this repository makes -- however recent -- updates the trunk and nothing
# else, and a sibling's ids are on the branch it pushed (H-0071, measured: `git fetch --prune origin
# main` leaves the allocator on its stale answer and still prints a line with `->` in it).
#
# A wildcard on the DESTINATION side is the other shape, and the star-anywhere test this replaces read
# it as a clean read: `+refs/pull/*/head:refs/remotes/origin/pr/*` -- what a person adds when they want
# pull-request heads locally -- fetches everything except the branches, it carries a star, and it left
# the allocator printing `D-0002` beside `REFRESH … 0s old` while `origin` already held `D-0002` on
# `refs/heads/issue-sib` (#309, measured 14 Nov on the shipped script). The same line is why the
# instruction that travels with a blind verdict has to name a config write: `git fetch` obeys the
# refspec it is given, so the unrefspec'd fetch the footer names cannot widen this clone (measured: the
# clone still held only `refs/remotes/origin/HEAD` and `…/main` after running it, same answer).
#
# It is read per line, and any learning line satisfies it, because the healthy wildcard and an added
# `pr/*` line coexist (`git config --add` is how that line gets there): measured on such a clone the
# union is complete (`refs/remotes/origin/issue-sib` is present, the answer is `D-0003`) and calling it
# blind would be the false blind, costing a fetch on every entry commit for no wrong id.
#
# An absent or unreadable refspec is still not called blind, for the reason it has always been -- the
# arms that report a real shape (no origin, no remote refs, a stale clock) name what they found, and
# this one would only be guessing (measured: origin, `refs/remotes/origin/*` present, and no
# `remote.origin.fetch` key at all answers `REFRESH`, clean).
#
# `+refs/*:refs/*`, a mirror's refspec, is called blind, and that is the true verdict for every shape
# this command can actually be aimed at: a bare mirror never reaches these arms -- it is refused as "not
# a readable git checkout" at exit 2 -- and a working clone carrying it cannot fill `refs/remotes` even
# when it fetches, because git refuses the write (`fatal: refusing to fetch into branch
# 'refs/heads/main' checked out at …`); measured on such a clone, the star-anywhere test answered
# `D-0002` over the sibling's `D-0002` with a clean `REFRESH` line beside it (H-0178).
blind_coverage() {
	local rs line learns=0 n
	if ! git -C "$root" remote get-url origin >/dev/null 2>&1; then
		blind='this checkout has no origin remote, so refs/remotes here holds nothing any other machine has ever pushed'
		return 1
	fi
	rs=$(git -C "$root" config --get-all remote.origin.fetch 2>/dev/null) || rs=''
	while IFS= read -r line; do
		[ -n "$line" ] || continue
		# `+` is the force flag and says nothing about what arrives; past the colon is the
		# destination, which is the half a `pr/*` refspec puts a star in.
		case ${line#+} in
		'refs/heads/*:'*) learns=1 ;;
		esac
	done <<<"${rs:-}"
	if [ -n "$rs" ] && [ "$learns" -ne 1 ]; then
		blind="this clone fetches only '${rs//$'\n'/ }', so refs/remotes here can never learn a branch another clone pushed -- widen it first, because a fetch cannot widen a refspec: git -C $root config remote.origin.fetch '+refs/heads/*:refs/remotes/origin/*'"
		return 1
	fi
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

# ------------------------------------------------------------------- what the answer costs the record
#
# A better message from this command cannot help a tick that never ran it, which is what #345
# measured: neither unit that collided on `D-0058` executed the allocator at all -- each ran
# `grep '^## D-' | tail` and took the next number by eye -- and nothing on the harness side could
# tell an issued id from an eyeballed one until a merge made two entries share it. So an answer
# handed to a UNIT is journalled here, in the supervisor's own journal, and the pass reads that
# journal at the end of the tick: a heading in the unit's logs with no journalled allocation
# reports `DECISION-ID-UNEARNED` (`factory_decision_provenance`, common.sh). The journal line is
# the provenance; the comparison is what makes skipping the allocator visible.
#
# Written only when a unit was answered: `FACTORY_TICK_UNIT` and `FACTORY_LOG` are set by
# `factory_tick` when it launches the session (`factory/factory.sh`), and a bare run -- a person
# checking, a selftest leg, any off-dispatch call -- has no unit to journal for and writes nothing.
# stdout keeps carrying the id alone and the exit stays 0 whatever this write does: the id is
# still the best number available, and a failed write that stays silent is a write that did not
# happen (#1180), so it says so on stderr instead.
if [ -n "${FACTORY_TICK_UNIT:-}" ] && [ -n "${FACTORY_LOG:-}" ]; then
	jrn_branch=${FACTORY_TICK_BRANCH:-}
	[ -n "$jrn_branch" ] ||
		jrn_branch=$(git -C "$root" rev-parse --abbrev-ref HEAD 2>/dev/null || true)
	jrn_read=REFRESH
	if [ -n "$blind_why" ]; then
		jrn_read=REFRESH-BLIND
	fi
	[ -z "$blind_age" ] || jrn_read="$jrn_read ${blind_age}s"
	jrn_read="$jrn_read union"
	# The trail of a step-over travels in the answer's own line (#442): a unit that sees a
	# surprising id must see whose id it skipped, not only which union read it came from. It is a
	# trailing note after the fixed fields, because both readers of this line -- the earned-set
	# read in common.sh and this command's own claim scan -- key on the positional fields.
	jrn_note=''
	[ -z "$claims_note" ] || jrn_note=" ($claims_note)"
	if printf '%s DECISION-ID %s unit=%s branch=%s (allocator, %s, %s)%s\n' \
		"$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$next" "$FACTORY_TICK_UNIT" \
		"${jrn_branch:-unknown}" "$rel" "$jrn_read" "$jrn_note" \
		>>"$FACTORY_LOG/supervisor.log" 2>/dev/null; then
		printf 'next-decision-id: journalled DECISION-ID %s (%s)\n' "$next" "$jrn_read" >&2
	else
		printf 'next-decision-id: JOURNAL-WRITE-FAILED %s -- %s is journalled nowhere, so the end-of-tick comparison will call it unearned\n' \
			"$FACTORY_LOG/supervisor.log" "$next" >&2
	fi
fi

# ------------------------------------------------------------------- where the entry goes
#
# The number is only half of what a writer needs; the other half is which file carries it. Before the
# split the answer was always the path the caller named, so naming it back was worth nothing. It is worth
# something now: `#429` phase 1 shipped a reader that opens both layouts while every writer appended to
# the monolith, and `AGENTS.md` tells a unit not to file an *existing* entry by hand for exactly that
# reason -- a hand-filed file is one nothing will produce again. Phase 2 ends the other half: a NEW
# entry is a file, in either log, so the answer names the entry file whichever face the caller asked
# about. A unit following the old brief and reading this line stops appending; a unit that ignores
# stderr still gets a correct id, which is all it ever got.
#
# stderr, because stdout is what a caller captures as the id: a marker in front of the number would be
# parsed into it or ignored, and an ignored marker is the silence this replaces (the same rule the PENDING
# and REFRESH notes follow above).
printf 'next-decision-id: EMITS %s/%s.md -- create that file with your entry; new entries are files, not monolith appends\n' \
	"$stem" "$next" >&2

printf '%s\n' "$next"
