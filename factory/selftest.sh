#!/usr/bin/env bash
# shellcheck shell=bash disable=SC2016
# selftest.sh -- prove `factory_tree_dirt` answers a content question, on trees that exist.
#
# Why this is in `make verify` (issue #228). The supervisor halts the whole pass when a tree
# looks dirty -- `$FACTORY_HALT` stops every other unit, every review answer and every merge --
# and it used to ask that question with `test -n "$(git status --porcelain)"`. That command
# answers from git's stat cache, so it can say "dirty" about a tree whose bytes match `HEAD`;
# and a killed `check-mutations.py` is a machine for producing exactly that, because it rewrites
# tracked files in place and writes the original bytes back. The halt's own advice cannot undo
# it, so the pass stops on a difference that is not work.
#
# Every branch of the replacement gets a leg here, which is the rule this repository already
# applies to scriptlets: a branch nobody plants is a branch nobody has ever seen run.
#
# What the legs are chosen for is discrimination, not coverage. One of them is load-bearing in
# the specific sense that a *weaker* verdict gets it wrong:
#
#   the staged leg    a change that is staged with the worktree equal to the index -- a merge that
#                     stopped for a decision, which is the state this halt exists to refuse -- is
#                     clean to `git diff` and dirty to `git diff --cached`. Issue #228's literal
#                     prescription (a content diff, plus the untracked list) therefore walks
#                     straight through it. `NAIVE` below is that prescription, computed alongside
#                     every leg, and the staged leg is marked `strong`: it FAILS if the weaker shape
#                     turns out to see its dirt after all, because then the leg has stopped pinning
#                     the arm the fix added and reads as coverage it no longer provides. Widening
#                     the guard back to the shape the issue asked for is caught by the leg's own
#                     expectation -- the verdict would come back CLEAN where the answer is DIRTY.
#   the stat leg      the one the issue was written about. It pins the behaviour rather than
#                     proving a failure, and says so on its own line: measured on git 2.53.0, a
#                     stat-only difference does not make `status` report dirt either (status
#                     refreshes the index in memory and finds the bytes equal -- with
#                     `.git/index` unwritable, with `.git/` unwritable, and with
#                     `core.trustctime` on). A leg that cannot fail against the old expression is
#                     worth less than the others and must not print as if it were worth the same,
#                     which is what the note is for. On a git or a filesystem that does report
#                     stat dirt, the same line reads FALSE POSITIVE REFUSED -- and that is the day
#                     this leg starts earning its place. What it *does* prove is proved twice: the
#                     fingerprint is measured with `ls-files --debug` before the verdict is asked,
#                     so the leg cannot pass on a tree where the planting did not happen (H-0043).
#
# Two of the checks are about the shipped text rather than a planted tree, because the failure they
# guard is silent at run time -- a step nobody runs, and a git invocation git refuses:
#
#   wired_into_verify   deleting the line in `verify:` would delete the only gate on `factory/`.
#   refused_refresh…    a refresh written with an option `update-index` does not accept exits 129
#                       into the `2>/dev/null || true` wrapped around it, which is the same output
#                       as a refresh that worked.
#
# Both are re-run against a copy of the file they read with the thing they look for taken out or put
# in, in the opposite polarity to the real assertion: a presence check has to prove it can say absent,
# an absence check has to prove it can say present. Without that, a clean line is the check working
# and not the check being blind.
#
# Nothing here touches the repository: the fixture is its own throwaway repo in $TMPDIR plus one
# of its linked worktrees, because a linked worktree is what the supervisor actually tests and it
# keeps its index under `.git/worktrees/<id>/` rather than next to the files -- which is part of
# how the stat answer and the content answer come apart in it.
set -euo pipefail

HERE=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=./common.sh
# shellcheck disable=SC1091
. "$HERE/common.sh"

work=$(mktemp -d "${TMPDIR:-/tmp}/plock-factoryselftest-XXXXXX")
cleanup() {
	chmod -R u+rwX "$work" 2>/dev/null || true
	rm -rf -- "$work"
}
trap cleanup EXIT INT TERM

pass=0
fail=0

# The verdict under test, as `verb<TAB>detail`.
verdict() {
	local out rc=0
	out=$(factory_tree_dirt "$1") || rc=$?
	case "$rc" in
	0) printf 'DIRTY\t%s\n' "$out" ;;
	1) printf 'CLEAN\t\n' ;;
	2) printf 'UNKNOWN\t%s\n' "$out" ;;
	*) printf 'RC%s\t%s\n' "$rc" "$out" ;;
	esac
}

# The verdict #228 literally asks for -- a content diff plus the untracked list -- kept alongside
# as a second opinion, so a leg can show the two disagree and so this file can refuse a future
# edit that quietly becomes this.
naive_verdict() {
	local tree=$1
	if ! git -C "$tree" diff --quiet 2>/dev/null; then
		printf 'DIRTY\n'
		return
	fi
	if [ -n "$(git -C "$tree" ls-files --others --exclude-standard 2>/dev/null | head -n 1)" ]; then
		printf 'DIRTY\n'
		return
	fi
	printf 'CLEAN\n'
}

# The fingerprint itself: git's cached mtime for a path versus the mtime on the file.
#
# It is measured rather than assumed because a leg about the stat cache is worth nothing when the
# cache is not actually disagreeing, and the first version of the stat leg could not tell: the line
# that reads as settling the cache before planting was written with an option `update-index` does
# not accept (H-0043), so it exited 129 behind a `|| true` and the leg asserted a verdict about a
# state it had not confirmed. A planted state is proved by `ls-files --debug`, which is the same
# number git would read from the cache.
#
# The two spellings differ in their leading zeros -- git prints `44033527`, `date +%N` prints
# `044033527`, and those are the same instant -- so each half is compared as a number. Comparing
# the strings reports a fingerprint that is not there, which is the failure mode where a leg
# believes it planted something and passes while pinning nothing.
planted() { # tree rel -> 0 when git's cached mtime and the file's own disagree
	local cached live cs cn ls ln
	cached=$(git -C "$1" ls-files --debug -- "$2" 2>/dev/null |
		sed -n 's/^[[:space:]]*mtime:[[:space:]]*//p' | head -n 1)
	live=$(printf '%s:%s' "$(stat -c '%Y' -- "$1/$2" 2>/dev/null)" "$(date -r "$1/$2" +%N 2>/dev/null)")
	[ -n "$cached" ] && [ -n "$live" ] || return 1
	cs=${cached%%:*}; cn=${cached##*:}
	ls=${live%%:*}; ln=${live##*:}
	[ "$((10#${cs:-0}))" != "$((10#${ls:-0}))" ] || [ "$((10#${cn:-0}))" != "$((10#${ln:-0}))" ]
}

# An assertion that is not a verdict -- the subject of a leg, rather than its answer. Same contract
# as `expect`: it never returns non-zero, because a probe that dies at its first finding
# under-reports exactly like a gate that skips.
check() { # label command...
	local label=$1
	shift
	if "$@"; then
		pass=$((pass + 1))
		printf '  ok    %-34s %s\n' "$label" "$*"
	else
		fail=$((fail + 1))
		printf '  FAIL  %-34s the subject of this leg is not in the tree, so whatever the\n' "$label"
		printf '          verdict says afterwards proves nothing about it: %s\n' "$*"
	fi
	return 0
}

# expect LABEL TREE WANT [strong] [SUBSTRING]
#   WANT is CLEAN, DIRTY, HALT or UNKNOWN. HALT accepts either verdict that stops the pass, for the
#   legs where what is owed is "it does not say clean" rather than a specific reason.
#   `strong` says: this dirt is real and `git diff` plus the untracked list cannot see it, so
#   agreeing with that weaker shape here is the failure.
#   SUBSTRING is what the finding line has to name. It is what makes each ARM load-bearing rather
#   than merely harmless: an arm can be deleted without changing any leg's verdict (both diff arms
#   do report a conflicted path), and the sentence the halt prints is the only place its absence is
#   visible. Measured while writing this: with the unmerged arm deleted all legs still passed on
#   their verdicts, which is why the words are asserted too.
# Every path returns 0 on purpose. `set -e` is on, and the first version of this function let the
# FAIL branch end on `[ -n "$detail" ] && printf`, which returns 1 when there is no detail -- so the
# loop died after the first finding and the run reported one failure out of ten. A probe that stops
# at its first finding under-reports exactly like a gate that skips.
expect() {
	local label=$1 tree=$2 want=$3 strong=${4:-} sub=${5:-}
	local line verb detail naive
	line=$(verdict "$tree")
	verb=${line%%$'\t'*}
	detail=${line#*$'\t'}
	naive=$(naive_verdict "$tree")

	if [ "$want" = HALT ] && [ "$verb" != CLEAN ]; then
		want=$verb
	fi
	if [ "$verb" != "$want" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s got %s, want %s\n' "$label" "${verb:-<empty>}" "$want"
		if [ -n "$detail" ]; then
			printf '          %s\n' "$detail"
		fi
		return 0
	fi
	if [ "$strong" = strong ] && [ "$naive" = DIRTY ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s this leg is marked strong -- dirt the weaker shape cannot see --\n' "$label"
		printf '          but `git diff` + the untracked list see it too (%s), so the leg no longer\n' "$detail"
		printf '          pins the arm that was added for it. Plant the state that arm exists for.\n'
		return 0
	fi
	if [ -n "$sub" ] && [ "${detail#*"$sub"}" = "$detail" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s verdict is right but the finding does not name `%s`, so the halt\n' "$label" "$sub"
		printf '          would send the operator to the wrong thing: %s\n' "${detail:-<no finding printed>}"
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-34s %-7s naive:%-6s %s\n' "$label" "$verb" "$naive" "$detail"
}

# A known-clean starting point for every leg, so a leg cannot inherit another leg's planting.
# `reset --hard` is right here and nowhere else: this fixture is this script's own, and no person's
# work is in it. In the repository's own trees the same command would destroy uncommitted work,
# which is why the supervisor never runs it and halts instead.
reset_tree() {
	local tree=$1
	git -C "$tree" merge --abort >/dev/null 2>&1 || true
	git -C "$tree" rebase --abort >/dev/null 2>&1 || true
	git -C "$tree" reset --quiet --hard HEAD >/dev/null 2>&1 || true
	git -C "$tree" clean -qfd >/dev/null 2>&1 || true
	# Plain, with no option in front of --refresh. The long form of -q is not an option `update-index`
	# accepts at all (H-0043: 129, "unknown option 'quiet'"), and -q itself means "keep going when the
	# index needs updating", which is the exit status the verdict's first arm reads.
	git -C "$tree" update-index --refresh >/dev/null 2>&1 || true
}

# ------------------------------------------------------------------------ the fixture

repo=$work/repo
mkdir -p "$repo/sub"
git -C "$repo" init -q .
git -C "$repo" config user.email selftest@example.invalid
git -C "$repo" config user.name selftest
git -C "$repo" config commit.gpgsign false
printf 'alpha\nbeta\ngamma\n' >"$repo/tracked.txt"
printf 'delta\n' >"$repo/sub/nested.txt"
printf 'kept\n' >"$repo/keepme.txt"
printf 'ignored-by-design.tmp\n' >"$repo/.gitignore"
git -C "$repo" add -A
git -C "$repo" commit -qm fixture
base=$(git -C "$repo" rev-parse --abbrev-ref HEAD)

# Two commits that disagree about one file, so the conflict leg is a real merge that stopped, not
# an index hand-forced into a shape git was never asked to produce. `side` has to be *checked out*
# to take a commit: a branch created and then left where it was only renames a commit that already
# exists, and the merge the leg then plants resolves itself -- which is how this leg first ran,
# printing SKIP with green legs around it.
git -C "$repo" checkout -q -b side
printf 'written on side\n' >"$repo/keepme.txt"
git -C "$repo" commit -qam side-writes-it
git -C "$repo" checkout -q "$base"
printf 'written here\n' >"$repo/keepme.txt"
git -C "$repo" commit -qam here-writes-it-differently

wt=$work/wt
git -C "$repo" worktree add -q --detach "$wt" HEAD

echo "factory/selftest.sh: the dirt verdict, on a throwaway repo and one of its linked worktrees"

# ------------------------------------------------------------------------ the legs
# Each leg runs in both trees: a verdict that is only correct next to `.git/` would still be wrong
# where the supervisor calls it.
for tree in "$repo" "$wt"; do
	where=primary
	if [ "$tree" = "$wt" ]; then
		where=worktree
	fi

	reset_tree "$tree"
	expect "[$where] nothing at all" "$tree" CLEAN

	# The leg the issue is about: the bytes are HEAD's, the stat data is not. Refresh first so this
	# plants a fresh difference rather than inheriting one, then put the same bytes back the way the
	# sweep's restore does -- a write, not a `touch` -- because the fingerprint is what the halt
	# mistook for work.
	#
	# `check … planted` is what turns this from a restatement of the leg above it into the leg the
	# ticket came for: the verdict is only about the stat cache if the cache is actually disagreeing
	# when the question is asked, and that is measured here rather than inferred from the write that
	# was meant to cause it. Both halves are checked -- the written-back file and the touched one,
	# because they are the two ways a sweep and an operator each leave this state.
	#
	# Before `expect`, deliberately: `factory_tree_dirt` refreshes first, and a writable index
	# absorbs the new mtime as it re-reads the bytes. Asking afterwards would find no fingerprint
	# whatever the tree looked like when the verdict was formed.
	git -C "$tree" update-index --refresh >/dev/null 2>&1 || true
	sleep 0.02
	cp "$tree/tracked.txt" "$work/same-bytes"
	cat "$work/same-bytes" >"$tree/tracked.txt"
	touch "$tree/sub/nested.txt"
	check "[$where] fingerprint is planted" planted "$tree" tracked.txt
	check "[$where] touch planted too" planted "$tree" sub/nested.txt
	expect "[$where] stat-only, same bytes" "$tree" CLEAN
	if [ "$where" = primary ]; then
		if [ -n "$(git -C "$tree" status --porcelain 2>/dev/null)" ]; then
			printf '        FALSE POSITIVE REFUSED: `status --porcelain` called this tree dirty and\n'
			printf '        the content verdict did not halt on it. This leg just caught a regression.\n'
		else
			printf '        note: `status --porcelain` says clean here too (git %s), so this leg pins\n' \
				"$(git --version | awk '{print $3}')"
			printf '        the behaviour on this git rather than proving the old shape failed.\n'
		fi
	fi

	printf 'ALPHA\nbeta\ngamma\n' >"$tree/tracked.txt"
	expect "[$where] one byte changed" "$tree" DIRTY "" modified:

	reset_tree "$tree"
	printf 'ALPHA\nbeta\ngamma\n' >"$tree/tracked.txt"
	git -C "$tree" add -- tracked.txt
	# Staged, and the worktree left equal to the index -- what a merge that stopped for a decision
	# looks like. Writing the old bytes back here would put the change in BOTH places at once, where
	# `git diff` can see it, and the leg would pass without proving the arm it is here for.
	expect "[$where] staged only" "$tree" DIRTY strong staged-but-uncommitted:

	reset_tree "$tree"
	printf 'brand new\n' >"$tree/untracked.txt"
	expect "[$where] an untracked file" "$tree" DIRTY "" untracked:

	reset_tree "$tree"
	printf 'a build artifact\n' >"$tree/ignored-by-design.tmp"
	expect "[$where] only an ignored file" "$tree" CLEAN

	rm "$tree/sub/nested.txt"
	expect "[$where] a tracked file deleted" "$tree" DIRTY

	reset_tree "$tree"
	chmod +x "$tree/keepme.txt"
	expect "[$where] the exec bit flipped" "$tree" DIRTY

	reset_tree "$tree"
	rm "$tree/keepme.txt"
	ln -s /nonexistent "$tree/keepme.txt"
	expect "[$where] a file replaced by a link" "$tree" DIRTY

	reset_tree "$tree"
	if git -C "$tree" merge --no-commit --no-ff side >/dev/null 2>&1; then
		printf '  SKIP  %-34s the merge resolved itself, so there is no conflict to plant\n' \
			"[$where] a conflicted index"
	else
		expect "[$where] a conflicted index" "$tree" DIRTY "" unmerged:
	fi

	# Failing closed: a tree git cannot read must not answer "clean, go ahead and write into it".
	# The verdict for it is UNKNOWN rather than DIRTY -- an unreadable file makes `git diff` itself
	# fail, and "I could not tell" is not evidence of safety, so it halts on the same path. Skipped,
	# out loud, where the mode bits cannot bite: root reads anything.
	reset_tree "$tree"
	if [ "$(id -u)" = 0 ]; then
		printf '  SKIP  %-34s running as root, so chmod 000 hides nothing\n' \
			"[$where] an unreadable file"
	else
		chmod 000 "$tree/keepme.txt"
		expect "[$where] an unreadable file" "$tree" HALT
		chmod 644 "$tree/keepme.txt"
	fi
done

# A path that is not a repository: UNKNOWN, which is what both call sites halt on. Refusing on an
# unreadable tree costs one pass; proceeding on one hands a model someone else's half-done work.
bare=$work/not-a-repo
mkdir -p "$bare"
if git -C "$bare" rev-parse --git-dir >/dev/null 2>&1; then
	printf '  SKIP  %-34s $TMPDIR is inside a work tree\n' "a directory that is no repo"
else
	expect "a directory that is no repo" "$bare" UNKNOWN
fi

# A tree whose repository cannot be opened: a stale entry under `$FACTORY_TREES`, a `.git` that
# exists and holds nothing, an environment a wrapper exported and never unset. Two shapes, because
# they fail in opposite directions, and both measured on git 2.53.0.
#
# SHAPE A -- `GIT_DIR` at a `.git` directory that holds nothing, read with `git -C` the way the
# supervisor reads a tree. Every read refuses, and the replaced expression turns that into a pass:
#
#     git status --porcelain        rc=128  out=''   err='fatal: not a git repository'
#     test -n "$(... 2>/dev/null)"  ->  CLEAN, so the tick works in the tree
#
# `2>/dev/null` is what turns it: git said it could not answer, and the suppression made that
# indistinguishable from "nothing to report". A false halt costs a pass; a false clean costs whatever
# the model then writes. That asymmetry is what the verdict is built around, and this is the shape
# where the fix is provably load-bearing on this git.
#
# SHAPE B -- `GIT_DIR` at a *working* repository and `GIT_WORK_TREE` at a path that is not there, so
# git is being asked about two trees at once. The reads stop agreeing with one another:
#
#     update-index --refresh   rc=128   diff --cached --quiet   rc=0  (clean)
#     diff --quiet             rc=128   ls-files --unmerged     rc=0  (clean)
#     ls-files --others        rc=128
#
# Two arms answer "clean" out of the other repository's index, and the read that would have caught it
# fails. So the exit status of every read is checked, not just what it printed: a verdict built by
# diffing, listing the unmerged entries and listing the untracked files -- a literal reading of what
# the issue prescribes, which looks at none of those statuses -- returns CLEAN here. Which read
# refuses first is not stable across gits, so every arm checks rather than only the first.
poisoned_tree() { # label tree gitdir worktree
	local label=$1 tree=$2 gdir=$3 wtree=$4
	local line verb detail status="" status_rc=0 cached_rc=0 others_rc=0

	mkdir -p "$tree"
	printf 'a file in a tree that git cannot read\n' >"$tree/file.txt"
	export GIT_DIR="$gdir" GIT_WORK_TREE="$wtree"
	line=$(verdict "$tree")
	verb=${line%%$'\t'*}
	detail=${line#*$'\t'}
	status=$(git -C "$tree" status --porcelain 2>/dev/null) || status_rc=$?
	git -C "$tree" diff --cached --quiet 2>/dev/null || cached_rc=$?
	git -C "$tree" ls-files --others --exclude-standard >/dev/null 2>&1 || others_rc=$?
	unset GIT_DIR GIT_WORK_TREE

	if [ "$verb" != UNKNOWN ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s got %s, want UNKNOWN   [%s]\n' "$label" "${verb:-<empty>}" "$detail"
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-34s UNKNOWN %s\n' "$label" "$detail"
	if [ -z "$status" ] && [ "$status_rc" -gt 1 ]; then
		printf '        THE EXPRESSION THIS REPLACES ANSWERED CLEAN HERE: `status --porcelain`\n'
		printf '        exited %s with no output and `2>/dev/null` hid the reason, so the tick\n' "$status_rc"
		printf '        would have worked in this tree. This is the direction that costs the work.\n'
	fi
	if [ "$cached_rc" -eq 0 ]; then
		printf '        `diff --cached` answered clean while `ls-files --others` exited %s: a read\n' "$others_rc"
		printf '        that cannot open this tree still reports on it, and the verdict took none of\n'
		printf '        it. The exit statuses are what refuse here, not the diff itself.\n'
	fi
	return 0
}

poisoned_tree "a tree with no readable repo" "$work/poison-a/tree" \
	"$work/poison-a/tree/.git" "$work/poison-a/tree"
poisoned_tree "a GIT_DIR pair from elsewhere" "$work/poison-b/tree" \
	"$repo/.git" "$work/poison-b/no-such-work-tree"

# A repository that reads fine except for one arm. `core.excludesFile` aimed at a directory -- a
# typo in a global config, inherited by every `git -C` the supervisor runs because it is configuration
# and not per-tree state -- stops the dirt test with no work in the tree: `status --porcelain` exits
# 128 printing nothing, so the replaced expression answers CLEAN, and the naive prescription (diff,
# unmerged, untracked, reading output only) answers CLEAN as well. The verdict answers UNKNOWN, and
# the leg prints what each of its five reads actually returned rather than asserting which one died:
# on git 2.53.0 it is `diff --cached` and `ls-files --others` that fail and `update-index --refresh`
# and `diff --quiet` that answer clean. Which fails first is a property of the git, not a promise, so
# every arm checks its own status; that also means no single arm here is pinned on its own, and
# deleting any one of the five checks has to be caught by the verdict as a whole.
git -C "$repo" config core.excludesFile "$work/excludes-is-a-directory"
mkdir -p "$work/excludes-is-a-directory"
excl_status=""
excl_status_rc=0
excl_status=$(git -C "$repo" status --porcelain 2>/dev/null) || excl_status_rc=$?
expect "an unreadable exclude file" "$repo" UNKNOWN
excl_reads=""
for arm in refresh cached diff unmerged others; do
	excl_rc=0
	case "$arm" in
	refresh) git -C "$repo" update-index --refresh >/dev/null 2>&1 || excl_rc=$? ;;
	cached) git -C "$repo" diff --cached --quiet >/dev/null 2>&1 || excl_rc=$? ;;
	diff) git -C "$repo" diff --quiet >/dev/null 2>&1 || excl_rc=$? ;;
	unmerged) git -C "$repo" ls-files --unmerged >/dev/null 2>&1 || excl_rc=$? ;;
	others) git -C "$repo" ls-files --others --exclude-standard >/dev/null 2>&1 || excl_rc=$? ;;
	esac
	excl_reads="$excl_reads $arm=$excl_rc"
done
if [ -z "$excl_status" ] && [ "$excl_status_rc" -gt 1 ]; then
	printf '        exit status of each read the verdict makes: %s\n' "$excl_reads"
	printf '        `status --porcelain` exited %s printing nothing: the replaced expression and\n' "$excl_status_rc"
	printf '        the diff-plus-untracked prescription both answer CLEAN on a tree whose state\n'
	printf '        git would not report. Only a verdict that checks statuses refuses.\n'
fi
git -C "$repo" config --unset core.excludesFile


# Nobody writes the refused spelling again, because nothing at run time would say so. That spelling
# exits 129 printing its usage on stderr, and behind the `>/dev/null 2>&1 || true` every one of these
# calls carries it is indistinguishable from a refresh that ran: the verdict stays correct (its own
# refresh is spelled right), and the probe quietly stops settling anything. Measured on git 2.53.0:
#
#     git update-index --quiet --refresh   -> 129, stderr `error: unknown option 'quiet'`
#     git update-index -q --refresh        -> 0
#     git update-index --refresh           -> 0
#
# Worth the leg for the other direction too: the same option written into `factory_tree_dirt` would
# make 129 its first read's answer, so every tree in the pass -- every clean one included -- would
# come back UNKNOWN and halt. One character, and the queue stops for all of them.
#
# The pattern is built so the line that defines it cannot match itself: the literal inside this file
# is followed by a quote, not by whitespace, and the control's copy is assembled from two arguments.
#
# Everything from the first `#` is cut before the test, so the check asks about code and not about
# prose. It has to: two of the three lines this leg first flagged were comments that named the bad
# spelling to explain it, and a guard that refuses a comment is a guard that gets widened away
# inside a week -- H-0040 is that lesson already, learned on this repository's other gate.
bad_refresh_calls() { # dir
	grep -rnE --exclude-dir=.git --exclude-dir=__pycache__ \
		--include='*.sh' --include='*.py' --include='Makefile' --include='*.spec' \
		-- 'update-index[[:space:]][^|;&]*--quiet' "$1" 2>/dev/null |
		awk '{ code = $0; sub(/#.*/, "", code); if (code ~ /update-index[ \t][^|;&]*--quiet/) print }' |
		head -n 5
}

refused_refresh_spelling() {
	local label="no refresh git refuses" root hits
	root=$(cd -- "$HERE/.." && pwd)
	hits=$(bad_refresh_calls "$root")
	if [ -n "$hits" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s a shipped script asks git the refused way, which exits 129 into a\n' "$label"
		printf '          `2>/dev/null || true` and reads exactly like a refresh that ran:\n'
		printf '%s\n' "$hits" | while IFS= read -r hit; do printf '            %s\n' "$hit"; done
	else
		pass=$((pass + 1))
		printf '  ok    %-34s every shipped refresh is the spelling git accepts\n' "$label"
	fi

	# The control. A checker that cannot see the thing certifies the absence of nothing, so it is
	# handed the bad spelling in a scratch directory -- outside the repository, so the check above
	# stays an honest statement about what is shipped.
	mkdir -p "$work/spelling"
	printf '%s %s\n' 'git -C "$tree" update-index' '--quiet --refresh >/dev/null 2>&1 || true' \
		>"$work/spelling/planted.sh"
	if [ -n "$(bad_refresh_calls "$work/spelling")" ]; then
		printf '        control: the same check does see the spelling when it is planted, so the\n'
		printf '        clean answer above is the check working and not the check being blind.\n'
	else
		fail=$((fail + 1))
		printf '  FAIL  %-34s the check did not see a bad spelling planted for it, so its\n' "control: $label"
		printf '          silence about the repository means nothing. Fix the pattern.\n'
	fi
	return 0
}
refused_refresh_spelling


# The step's own invocation, pinned. Nothing in this repository referenced `factory/` before this
# change -- `grep -rn factory Makefile .github/workflows/` on `feat/factory-harness` returns nothing
# and there is no step-manifest checker (`git log --all -S'check-verify.sh'` is empty) -- so this file
# is the dirt verdict's only gate and the single line that invokes it would itself be unpinnable. The
# repository already has the rule for exactly this, in the verify recipe's own comment on the
# screenshot manifest: "a gate whose invocation can be deleted silently is a suggestion, and the
# cheapest way to pass a check is to stop calling it". So deleting the step from `verify:` makes the
# step itself refuse.
verify_runs_this() { # makefile -> 0 when the `verify:` recipe invokes this file
	awk '/^verify:/{inr=1; next} inr && /^[^ \t]/{inr=0} inr' "$1" | grep -q 'factory/selftest\.sh'
}

wired_into_verify() {
	local label="the step is wired into make verify" makefile=$HERE/../Makefile stripped
	if [ ! -f "$makefile" ]; then
		printf '  SKIP  %-34s no Makefile above %s\n' "$label" "$HERE"
		return 0
	fi
	if verify_runs_this "$makefile"; then
		pass=$((pass + 1))
		printf '  ok    %-34s invoked from the verify recipe\n' "$label"
	else
		fail=$((fail + 1))
		printf '  FAIL  %-34s nothing under `verify:` runs this file, so nothing executes it.\n' "$label"
		printf '          A guard that runs only when somebody remembers it is a guard whose evidence\n'
		printf '          nobody can reproduce: add `bash factory/selftest.sh` to the verify recipe.\n'
	fi

	# The control, in the opposite polarity: the same scan of the same recipe with the step's line
	# deleted has to say no. If it still says yes the pattern is matching something else in the
	# recipe and the ok above is this file certifying itself out of the goodness of a wildcard.
	mkdir -p "$work/wiring"
	stripped=$work/wiring/Makefile
	sed '/factory\/selftest\.sh/d' "$makefile" >"$stripped"
	if verify_runs_this "$stripped"; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s the scan still finds the step after its line is deleted, so it\n' "control: $label"
		printf '          matches something else in the recipe and the ok above means nothing.\n'
	else
		printf '        control: the same scan says the step is gone once its line is deleted, so\n'
		printf '        the ok above is the check working and not the check being blind.\n'
	fi
	return 0
}
wired_into_verify

printf 'factory/selftest.sh: %d passed, %d failed\n' "$pass" "$fail"
if [ "$fail" -ne 0 ]; then
	exit 1
fi
