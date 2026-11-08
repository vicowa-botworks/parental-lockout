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

# The verdict's whole output is one line. Not a formatting taste: the HALT file is read as its last
# line, and the journal line the halt appends is read as the last line of the journal, so a second
# line loses the resume step. This is the tight form of the invariant `expect_halt` also guards, and
# it is here because the stderr capture almost broke it -- `git update-index --refresh` prints
# "path: needs update" on its STANDARD output, and anything that lets that into the finding splits
# every halt in the run.
finding_one_line() { # tree -> 0 when the verdict printed exactly one line
	local n
	n=$(factory_tree_dirt "$1" 2>/dev/null | wc -l | tr -d '[:space:]')
	[ "$n" = 1 ]
}

# The plant's own claim, asked of the read that arm exists for: does `git diff` refuse to answer over
# this tree? Run raw, outside the verdict, so it reports what the box does rather than what the code
# under test decided. It is what lets the leg assert UNKNOWN instead of "anything but clean": with a
# git that reads the file anyway there is no refusal to pin, and the leg says so instead of certifying
# a verdict this box never produced.
tracked_read_refuses() { # tree -> 0 when `git diff --quiet` exits above 1
	local rc=0
	git -C "$1" diff --quiet >/dev/null 2>&1 || rc=$?
	[ "$rc" -gt 1 ]
}

# Every path, when there are three or fewer. The cap is honest in both directions or it is not a cap:
# seven problems printed as three read as a complete list, and a marker printed on a complete list
# teaches the reader to ignore the marker.
finding_uncapped() { # tree -> 0 when the finding names all three planted paths and says nothing about more
	local out names
	out=$(factory_tree_dirt "$1" 2>/dev/null)
	[ -n "$out" ] || return 1
	case "$out" in *"in all)"*) return 1 ;; esac
	names=$(printf '%s' "$out" | tr ' ' '\n' | grep -c '^extra-' || true)
	[ "$names" = 3 ]
}

# A reason that names the thing the operator would have to fix. `check`, not `expect`: this is about
# whether the plant produced a reason at all, and a git that words its warnings differently is a
# subject missing from the tree, not a wrong verdict.
verdict_names() { # tree needle -> 0 when the verdict's line carries it
	local out
	out=$(factory_tree_dirt "$1" 2>/dev/null)
	[ -n "$out" ] || return 1
	case "$out" in *"$2"*) return 0 ;; esac
	return 1
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

# --------------------------------------------------------------------------- the halt, not only the verdict
#
# The verdict being right is worth nothing at a call site that misreads it. Until H-0044 nothing in
# this repository executed the decision: the two halting paths each had their own copy of
# `[ "$dirt_rc" -ne 1 ]` and a mutation audit of them -- flip it to `-ne 0`, flip it to `-eq 0`, delete
# the guard -- left every leg green three times over. The legs below drive `factory_tree_halt`, the
# one guard both paths now share, over the same planted states, and assert the four things a halt has
# to get right: that it halts, which exit code it leaves, what the operator finds in the halt file,
# and what the journal says.
#
# The HALT file and the journal are part of the assertion rather than decoration. `factory_halted()`
# reads the file and nothing else, so a guard that returns 4 and writes no file stops one unit and
# lets the pass continue -- and the file's sentence is printed verbatim by `factory.sh --status`, which
# is the whole of what whoever finds the queue halted has to act on.
#
# Every path is redirected into the fixture box: this file's own promise is that it touches nothing in
# the repository, and `log_event` creates directories and appends. common.sh computes its defaults at
# source time, so the assignment has to happen after the source and the functions read these at call
# time.
FACTORY_DIR=$work/halt
# The three below are read only by functions sourced from common.sh, and shellcheck analyses this file
# on its own: `factory_paths()` creates the ledger and run directories, `log_event` appends to the
# journal. Leaving any of them at its default would point the probe's journal or halt file at the
# repository, which is the one thing this file's own contract forbids.
FACTORY_LOG=$FACTORY_DIR/log
# shellcheck disable=SC2034  # created by factory_paths(), which lives in the sourced common.sh
FACTORY_LEDGER=$FACTORY_DIR/ledger
# shellcheck disable=SC2034  # likewise: the lock and per-unit state live here
FACTORY_RUN=$FACTORY_DIR/run
FACTORY_HALT=$FACTORY_DIR/HALT

# expect_halt LABEL TREE WANT [SUBSTRING]
#   WANT is PROCEED or HALT -- what the supervisor does with the unit. A HALT leg requires all of:
#   exit 4 from the tick's verb, exit 2 from the conflict pass's, a HALT file (because that is what
#   `factory_halted()` tests), the `rm …` resume step inside it, the action that matches the finding,
#   and one journal line per path carrying that path's own verb. A PROCEED leg requires the guard to
#   have halted neither and to have written neither file: a halt that fires on a clean tree is the
#   false positive #228 was filed about, and it stops every other unit in the pass.
#   SUBSTRING is asserted inside the halt file's sentence -- the finding has to reach the operator,
#   not just the return code.
expect_halt() {
	local label=$1 tree=$2 want=$3 sub=${4:-}
	local c1=0 c2=0 h1="" hl=0 j1="" j2="" before=0 after=0 bad="" want_how drc=0

	factory_tree_dirt "$tree" >/dev/null || drc=$?
	case "$drc" in
	2) want_how="make the tree readable" ;;
	*) want_how="resolve the work it names" ;;
	esac
	if [ -f "$FACTORY_LOG/supervisor.log" ]; then
		before=$(wc -l <"$FACTORY_LOG/supervisor.log")
	fi

	factory_tree_halt TICK-HALTED selftest "$tree" >"$work/guard.out" 2>&1 || c1=$?
	if [ -e "$FACTORY_HALT" ]; then
		h1=$(tr '\n' ' ' <"$FACTORY_HALT")
		hl=$(wc -l <"$FACTORY_HALT" | tr -d '[:space:]')
	fi
	if [ -s "$FACTORY_LOG/supervisor.log" ]; then j1=$(tail -n 1 "$FACTORY_LOG/supervisor.log"); fi
	rm -f -- "$FACTORY_HALT"
	factory_tree_halt CONFLICT-HALTED selftest "$tree" >"$work/guard.out" 2>&1 || c2=$?
	# The halt file's sentence is read from the tick's call only. It is one `printf` in the guard, so
	# the two paths cannot disagree about it; what differs per path is the exit code and the journal
	# verb, and both are asserted below. Reading the same string twice would buy a leg that can only
	# fail twice.
	if [ -s "$FACTORY_LOG/supervisor.log" ]; then j2=$(tail -n 1 "$FACTORY_LOG/supervisor.log"); fi
	rm -f -- "$FACTORY_HALT"
	if [ -f "$FACTORY_LOG/supervisor.log" ]; then
		after=$(wc -l <"$FACTORY_LOG/supervisor.log")
	fi

	if [ "$want" = PROCEED ]; then
		[ "$c1" = 0 ] || bad="the tick was stopped (exit $c1)"
		[ "$c2" = 0 ] || bad="${bad:+$bad; }the conflict pass was stopped (exit $c2)"
		[ -z "$h1" ] || bad="${bad:+$bad; }a HALT file was written, so every other unit is stopped too"
		[ "$((after - before))" = 0 ] || bad="${bad:+$bad; }it journaled $((after - before)) line(s) for a unit it let through"
		want=PROCEED
	else
		[ "$c1" = 4 ] || bad="the tick left $c1, want 4"
		[ "$c2" = 2 ] || bad="${bad:+$bad; }the conflict pass left $c2, want 2"
		[ -n "$h1" ] || bad="${bad:+$bad; }no HALT file, so factory_halted() would not stop the pass"
		# One line, and it is an assertion rather than a formatting taste. Everything that reads this
		# file takes the LAST line: `log_event` appends the finding to a journal line, `tail -n 1` is how
		# the journal is read above, and a verdict that lets git's own chatter into its output splits the
		# sentence in two -- which happened while writing the stderr capture, where `update-index
		# --refresh`'s "path: needs update" (stdout, not stderr) went into the finding and every halt in
		# the run failed here with a message about journal lines.
		[ "$hl" = 1 ] || bad="${bad:+$bad; }the HALT file is $hl lines, want one: the resume step and\n          the reason are read as the last line, so a split sentence loses one of them"
		case "$h1" in *"rm $FACTORY_HALT"*) : ;; *) bad="${bad:+$bad; }the sentence names no resume step" ;; esac
		case "$h1" in *"$want_how"*) : ;; *) bad="${bad:+$bad; }the sentence promises the wrong action for a $drc" ;; esac
		case "$j1" in *TICK-HALTED*"$tree"*) : ;; *) bad="${bad:+$bad; }the journal line is not the tick's" ;; esac
		case "$j2" in *CONFLICT-HALTED*) : ;; *) bad="${bad:+$bad; }the conflict pass journaled the tick's verb" ;; esac
		[ "$((after - before))" = 2 ] || bad="${bad:+$bad; }it journaled $((after - before)) lines, want one per path"
		if [ -n "$sub" ]; then
			case "$h1" in *"$sub"*) : ;; *) bad="${bad:+$bad; }the finding never reaches the operator: ${h1:-<no file>}" ;; esac
		fi
		want=HALT
	fi

	if [ -n "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s halt: %s\n' "$label" "$bad"
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-34s halt:%-8s tick:%s merge:%s %s\n' "$label" "$want" "$c1" "$c2" "${h1:+$(printf '%.60s' "$h1")}"
	return 0
}

# The two call sites, pinned to the one shape that means "halt if the guard says so". The guard's own
# behaviour is legs above; what a leg cannot reach is a call site that calls it and drops the answer,
# so that is a check on the shipped text -- the same bargain as `wired_into_verify`, and the same
# reason: the failure is silent at run time.
halt_call_sites_in() { # dir -> prints the lines that are wrong, empty when the shape holds
	local root=$1 sites ticks merges
	sites=$(grep -rn --include='*.sh' --exclude-dir=.git -- 'factory_tree_halt ' "$root" 2>/dev/null |
		grep -v 'factory/common\.sh:' | grep -v 'factory/selftest\.sh:')
	if [ -z "$sites" ]; then
		printf 'no call site of factory_tree_halt anywhere\n'
		return 0
	fi
	printf '%s\n' "$sites" | grep -v '|| return \$?' || true
	# Both verbs have to be present, because deleting one call site leaves the other one correctly
	# shaped and nothing else in the repository would notice: the tick's guard vanishing is the same
	# hole as the guard being wrong, and it is a smaller diff. Measured -- deleting the tick's whole
	# line was the one mutation of ten that this check first missed.
	ticks=$(printf '%s\n' "$sites" | grep -c 'TICK-HALTED' || true)
	merges=$(printf '%s\n' "$sites" | grep -c 'CONFLICT-HALTED' || true)
	[ "$ticks" -ge 1 ] || printf 'no call site halts a tick (TICK-HALTED): the tree is asked and the answer dropped\n'
	[ "$merges" -ge 1 ] || printf 'no call site halts the conflict pass (CONFLICT-HALTED)\n'
	# A second copy of the decision is how the two sites drifted apart in the first place, and the
	# raw verdict is what a new copy would be written against.
	grep -rn --include='*.sh' --exclude-dir=.git -- 'dirt_rc' "$root" 2>/dev/null |
		grep -v 'factory/common\.sh:' | grep -v 'factory/selftest\.sh:' || true
	return 0
}

halt_call_sites_shaped() {
	local label="both halts share the guard" root bad
	root=$(cd -- "$HERE/.." && pwd)
	bad=$(halt_call_sites_in "$root")
	if [ -n "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s a halting path does not hand back the exit code it was given, or a\n' "$label"
		printf '          second copy of the decision is back:\n'
		printf '%s\n' "$bad" | while IFS= read -r line; do printf '            %s\n' "$line"; done
	else
		pass=$((pass + 1))
		printf '  ok    %-34s every call site is `… || return %%s`, one decision in common.sh\n' "$label"
	fi

	# The control, planted outside the repository: a call site that drops the return, and a second
	# copy of the predicate. Either one has to be flagged, or the ok above is the check being blind.
	mkdir -p "$work/sites/factory"
	printf '%s\n' \
		'#!/usr/bin/env bash' \
		'. "$(dirname "$0")/common.sh"' \
		'factory_tree_halt TICK-HALTED "$unit" "$tree"' \
		'local dirt_rc=0' \
		'dirt_rc=$(factory_tree_dirt "$tree")' \
		'if [ "$dirt_rc" -ne 1 ]; then :; fi' >"$work/sites/factory/planted.sh"
	if [ -n "$(halt_call_sites_in "$work/sites")" ]; then
		printf '        control: a planted call site that drops the return and a planted second copy\n'
		printf '        of the predicate are both flagged, so the ok above is the check working.\n'
	else
		fail=$((fail + 1))
		printf '  FAIL  %-34s neither a dropped return nor a second copy of the predicate\n' "control: $label"
		printf '          was flagged, so its silence about the repository means nothing.\n'
	fi
	return 0
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
	expect_halt "[$where] nothing at all" "$tree" PROCEED

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
	expect_halt "[$where] stat-only, same bytes" "$tree" PROCEED
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
	expect_halt "[$where] one byte changed" "$tree" HALT modified:

	reset_tree "$tree"
	printf 'ALPHA\nbeta\ngamma\n' >"$tree/tracked.txt"
	git -C "$tree" add -- tracked.txt
	# Staged, and the worktree left equal to the index -- what a merge that stopped for a decision
	# looks like. Writing the old bytes back here would put the change in BOTH places at once, where
	# `git diff` can see it, and the leg would pass without proving the arm it is here for.
	expect "[$where] staged only" "$tree" DIRTY strong staged-but-uncommitted:
	expect_halt "[$where] staged only" "$tree" HALT staged-but-uncommitted:

	reset_tree "$tree"
	printf 'brand new\n' >"$tree/untracked.txt"
	expect "[$where] an untracked file" "$tree" DIRTY "" untracked:
	expect_halt "[$where] an untracked file" "$tree" HALT untracked:

	# The cap, from both sides. Three names is what fits a one-line HALT file; what made the old cap a
	# lie was saying nothing about the rest -- ten problems arrived as three names and a trailing space,
	# and the reader stopped at the last comma. A marker printed on a complete list is the same lie the
	# other way, so this leg plants three (no marker, all named) and then five (three named, two
	# declared, five in all), in both trees.
	reset_tree "$tree"
	for f in 1 2 3; do printf 'x\n' >"$tree/extra-$f.txt"; done
	check "[$where] three named are all there are" finding_uncapped "$tree"
	for f in 4 5; do printf 'x\n' >"$tree/extra-$f.txt"; done
	expect "[$where] five untracked, three named" "$tree" DIRTY "" '(+2 more, 5 in all)'
	expect_halt "[$where] five untracked, three named" "$tree" HALT '(+2 more, 5 in all)'

	reset_tree "$tree"
	printf 'a build artifact\n' >"$tree/ignored-by-design.tmp"
	expect "[$where] only an ignored file" "$tree" CLEAN
	expect_halt "[$where] only an ignored file" "$tree" PROCEED

	rm "$tree/sub/nested.txt"
	expect "[$where] a tracked file deleted" "$tree" DIRTY
	expect_halt "[$where] a tracked file deleted" "$tree" HALT

	reset_tree "$tree"
	chmod +x "$tree/keepme.txt"
	# The shape that leaks: a mode-bit change makes `update-index --refresh` print "keepme.txt: needs
	# update", so this is the tree that catches any read's chatter reaching the finding.
	check "[$where] the finding is one line" finding_one_line "$tree"
	expect "[$where] the exec bit flipped" "$tree" DIRTY
	expect_halt "[$where] the exec bit flipped" "$tree" HALT

	reset_tree "$tree"
	rm "$tree/keepme.txt"
	ln -s /nonexistent "$tree/keepme.txt"
	expect "[$where] a file replaced by a link" "$tree" DIRTY
	expect_halt "[$where] a file replaced by a link" "$tree" HALT

	reset_tree "$tree"
	if git -C "$tree" merge --no-commit --no-ff side >/dev/null 2>&1; then
		printf '  SKIP  %-34s the merge resolved itself, so there is no conflict to plant\n' \
			"[$where] a conflicted index"
	else
		expect "[$where] a conflicted index" "$tree" DIRTY "" unmerged:
		expect_halt "[$where] a conflicted index" "$tree" HALT unmerged:
	fi

	# Failing closed: a tree git cannot read must not answer "clean, go ahead and write into it".
	# The verdict for it is UNKNOWN rather than DIRTY -- an unreadable file makes `git diff` itself
	# fail, and "I could not tell" is not evidence of safety, so it halts on the same path.
	#
	# It asserts UNKNOWN, and that is the whole point of the leg: `expect … HALT` accepts any answer
	# that is not CLEAN, which is the right question for the halt and no question at all about the
	# code path. Measured at 8015951 by hollowing one refusal test at a time -- a constant-false test,
	# because `[ "$rc" -gt 99 ]` is NOT hollow, a refusal is 128 -- every single arm could be deleted
	# with the suite at 70 passed / 0 failed, and the reason this leg stayed green while the verdict
	# degraded from `its tracked files could not be read (git diff exited 128)` to
	# `modified: keepme.txt (git said: … Permission denied)` is that both answers halt. The degraded
	# sentence is the harm: it tells the operator to "commit it or discard it on purpose" about a file
	# git could not open, and on a tree whose only other content is clean the same degradation is a
	# CLEAN that lets the tick work in a tree it cannot read. So the needle is the refusal's own words.
	#
	# Skipped, out loud, where the plant cannot bite: root reads anything, and a git that reads the
	# file anyway leaves no refusal to pin -- the same bargain `planted()` and `excludes_reads()`
	# struck, and for the same reason: asserting a verdict this git never produced is a green leg
	# about a subject that is not in the tree.
	reset_tree "$tree"
	if [ "$(id -u)" = 0 ]; then
		printf '  SKIP  %-34s running as root, so chmod 000 hides nothing\n' \
			"[$where] an unreadable file"
	else
		chmod 000 "$tree/keepme.txt"
		if tracked_read_refuses "$tree"; then
			expect "[$where] an unreadable file" "$tree" UNKNOWN "" could not be read
			expect_halt "[$where] an unreadable file" "$tree" HALT could not be read
		else
			printf '  SKIP  %-34s this git read the file anyway (git %s), so the plant leaves no\n' \
				"[$where] an unreadable file" "$(git --version | awk '{print $3}')"
			printf '        refusal to pin. The leg would assert a verdict this git never produced.\n'
		fi
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
	# The halt on an unreadable tree has to promise the OTHER action, and this is the shape that pins
	# it without depending on mode bits: `chmod 000` is invisible to root, so the leg in the loop that
	# carries the same arm is skipped there. A stale entry under $FACTORY_TREES is the real version of
	# this tree, and "commit it or discard it" is advice nobody can follow in it.
	expect_halt "a directory that is no repo" "$bare" HALT
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

# A repository whose global ignore file git cannot use -- `core.excludesFile` aimed at a directory, a
# typo in a config the supervisor never sets and cannot see, inherited by every `git -C` it runs
# because it is configuration and not per-tree state.
#
# What that does to the dirt test depends on the FILESYSTEM the fixture sits on, so the leg asks git
# which world it is in before it asserts anything. Measured on this box's git 2.53.0 with only
# `$TMPDIR` moved, with a file in the tree that only the global file would hide:
#
#   excludesFile            dir st_size   git status --porcelain        the verdict
#   a directory, tmpfs             40     128, prints NOTHING           UNKNOWN
#   a directory, btrfs              0     0, lists that file            DIRTY
#
# ...and the discriminator is not the filesystem alone. A directory's `st_size` is the size of its own
# entry data, so these two rows describe an EMPTY directory; put one entry in and the size -- and the
# verdict -- change on both filesystems (measured at 8015951 with `stat -c %s`, an entry named `x` and
# the same entry named `inside`):
#
#   the directory            empty   `x`   `inside`
#   btrfs                       0      2      12
#   tmpfs                      40     60      60
#
# So the second row is really "an empty directory on a filesystem that gives empty directories no
# size", and the same mistyped path refuses the moment it contains anything, btrfs included. The
# fixture's directory is empty, which is the whole reason the review box answered row two.
#
# git's `add_patterns()` opens the exclude file, `fstat`s it and branches on `st_size`: 0 is taken for
# an empty list and returns; any other size reaches `read_in_full()` on a directory fd, which is
# EISDIR and a 128 -- `fatal: cannot use %s as an exclude file`. So the refusal is a filesystem
# property wearing a git property's clothes, and the
# shipped claim that `status --porcelain` "exits 128 printing nothing" for this shape is true only
# where directories have a size. No shape of `core.excludesFile` is refused portably -- ENOENT,
# ENOTDIR and EACCES are all tolerated on 2.53.0 -- so the leg cannot be repaired by planting the
# same idea harder. It is repaired the way `planted()` repaired the stat leg: measure whether the
# plant made a read refuse, then assert only the verdict this box actually determined.
#
# The file only the global file would hide (`excluded-by-the-global-file.tmp`, deliberately absent
# from `.gitignore`) is what gives the second row a verdict at all. Without it the tree is clean on
# btrfs for a reason the plant has nothing to do with, and a leg that passes there certifies the
# CLEAN it was written to refuse. With it, the second row asserts the FALSE HALT direction the same
# plant produces: the global patterns are gone, the file is named as untracked, and the pass stops
# over work that is not work -- which is what the halt's sentence has to carry a reason for.
#
# Which read refuses first is a property of the git, not a promise, so every arm checks its own
# status; that also means no single arm here is pinned on its own, and deleting any one of the five
# checks has to be caught by the verdict as a whole.
excludes_reads() { # repo -> sets EXCLUDE_READS and EXCLUDE_WHY; 0 when the plant made at least one refuse
	local arm rc any=1
	EXCLUDE_READS=''
	EXCLUDE_WHY=0
	for arm in refresh cached diff unmerged others; do
		rc=0
		: >"$work/excl-err"
		case "$arm" in
		refresh) git -C "$1" update-index --refresh >/dev/null 2>>"$work/excl-err" || rc=$? ;;
		cached) git -C "$1" diff --cached --quiet >/dev/null 2>>"$work/excl-err" || rc=$? ;;
		diff) git -C "$1" diff --quiet >/dev/null 2>>"$work/excl-err" || rc=$? ;;
		unmerged) git -C "$1" ls-files --unmerged >/dev/null 2>>"$work/excl-err" || rc=$? ;;
		others) git -C "$1" ls-files --others --exclude-standard >/dev/null 2>>"$work/excl-err" || rc=$? ;;
		esac
		EXCLUDE_READS="$EXCLUDE_READS $arm=$rc"
		if [ "$rc" -gt 1 ]; then
			any=0
			# Did the refusal come with a reason, or is git silent here? Asked, because the leg is about
			# to assert that a reason reaches the operator, and a git that refuses without saying why
			# leaves nothing to assert -- the same bargain `planted()` struck for the stat leg.
			if [ -s "$work/excl-err" ]; then EXCLUDE_WHY=1; fi
		fi
	done
	return "$any"
}

git -C "$repo" config core.excludesFile "$work/excludes-is-a-directory"
mkdir -p "$work/excludes-is-a-directory"
printf 'only a global ignore file would hide this\n' >"$repo/excluded-by-the-global-file.tmp"
excl_status=""
excl_status_rc=0
excl_status=$(git -C "$repo" status --porcelain 2>/dev/null) || excl_status_rc=$?
if excludes_reads "$repo"; then
	if [ "$EXCLUDE_WHY" = 1 ]; then
		# The exit code alone is what `2>/dev/null` used to hand the operator: "exited 128" says
		# something refused and nothing about what or where. `git said:` is this repository's own marker,
		# asserted rather than git's wording, so the leg pins the carrying and not the phrasing.
		expect "an exclude file git refused" "$repo" UNKNOWN "" 'git said:'
		expect_halt "an exclude file git refused" "$repo" HALT 'git said:'
	else
		expect "an exclude file git refused silently" "$repo" UNKNOWN
		printf '        note: this git refused without printing a reason, so there is none to carry and\n'
		printf '        the leg asserts the verdict alone.\n'
	fi
	printf '        exit status of each read the verdict makes: %s\n' "${EXCLUDE_READS# }"
	if [ -z "$excl_status" ] && [ "$excl_status_rc" -gt 1 ]; then
		printf '        `status --porcelain` exited %s printing nothing: the replaced expression and\n' "$excl_status_rc"
		printf '        the diff-plus-untracked prescription both answer CLEAN on a tree whose state\n'
		printf '        git would not report. Only a verdict that checks statuses refuses.\n'
	fi
else
	expect "an exclude file git swallowed" "$repo" DIRTY "" untracked:
	printf '        exit status of each read the verdict makes: %s\n' "${EXCLUDE_READS# }"
	printf '        note: on %s a directory reports st_size %s, so git read the exclude file as an\n' \
		"$(stat -f -c %T -- "$work/excludes-is-a-directory")" \
		"$(stat -c %s -- "$work/excludes-is-a-directory")"
	printf '        empty list and refused nothing. Nothing was unreadable to git on this box, so the\n'
	printf '        leg asserts only what it did determine here: the operator'"'"'s global patterns are\n'
	printf '        gone and the file only they would hide is named as untracked. That is the FALSE\n'
	printf '        HALT direction -- the pass stops over work that is not work, with no warning from\n'
	printf '        git to say why. The refusal this leg exists to pin is unreachable on this\n'
	printf '        filesystem, and this note is the honest statement of that, not a green leg.\n'
fi

# ...unless the exclude file is one git cannot open at all, which is the same dropped patterns WITH a
# reason, on every filesystem there is. Measured on git 2.53.0 in this fixture, with one file that only
# the global ignore file would hide, and the five reads run the way the verdict runs them:
#
#   excludesFile          the five reads   `status --porcelain`            git's stderr
#   a `chmod 000` file    all rc=0         `?? excluded-by-the-global…`    `warning: unable to access
#                                                                          '…': Permission denied`
#   a symlink to itself   all rc=0         the same listing                the same warning, with
#                                                                          'Too many levels of symbolic links'
#   a dangling symlink    all rc=0         the same listing                nothing at all
#   a missing path        all rc=0         the same listing                nothing at all
#
# So the loop is planted rather than the mode bits: `chmod 000` is invisible to root and the leg the
# unreadable-file arm carries is already skipped there, while ELOOP catches whoever runs this. Either
# way the EXIT STATUS says nothing is wrong -- that is what makes this the false halt of the arm above,
# arrived at by a different road -- and the warning is the only thing in the whole run that says why the
# queue stopped. It used to be thrown away by the same `2>/dev/null` that turned a 128 into a clean
# tree. Asserted in both places it has to land: the verdict's line, and `factory/HALT`, which is the
# whole of what a person finds when the pass is stopped.
ln -s excludes-is-a-loop "$work/excludes-is-a-loop"
git -C "$repo" config core.excludesFile "$work/excludes-is-a-loop"
excl_warn=""
git -C "$repo" ls-files --others --exclude-standard >/dev/null 2>"$work/excl-warn" || true
excl_warn=$(head -n 1 "$work/excl-warn")
if [ -n "$excl_warn" ]; then
	expect "an exclude file git warned about" "$repo" DIRTY "" 'git said:'
	expect_halt "an exclude file git warned about" "$repo" HALT 'git said:'
	check "the reason names the exclude file" verdict_names "$repo" 'excludes-is-a-loop'
	printf '        git said `%s`, and the halt now says it back: the operator can see which file\n' "$excl_warn"
	printf '        to fix instead of being told to commit a file git was told to ignore.\n'
else
	expect "an exclude file git warned about" "$repo" DIRTY "" untracked:
	printf '        note: this git dropped the patterns saying nothing, so there is no reason to\n'
	printf '        carry and the leg asserts the verdict alone.\n'
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
	# `|| true` is load-bearing, not tidiness. The step runs `set -euo pipefail`, so a pipeline whose
	# FIRST command fails fails as a whole, and callers take the status into an assignment --
	# `hits=$(bad_refresh_calls "$root")` -- where errexit kills the script at that line: no FAIL line,
	# no tally, exit 1, and the gate has stopped running in silence. A scan whose subject is absent must
	# answer "nothing", which is a status of 0. Today the first grep matches only because this
	# repository's comments quote the bad spelling; delete those comments and the guard itself goes
	# missing. `scans_cannot_kill_the_step` below is the leg that keeps it true.
	grep -rnE --exclude-dir=.git --exclude-dir=__pycache__ \
		--include='*.sh' --include='*.py' --include='Makefile' --include='*.spec' \
		-- 'update-index[[:space:]][^|;&]*--quiet' "$1" 2>/dev/null |
		awk '{ code = $0; sub(/#.*/, "", code); if (code ~ /update-index[ \t][^|;&]*--quiet/) print }' |
		head -n 5 || true
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

# The dirt test this branch deleted, refused by its own text -- in every shipped script, not only in
# the copies of the predicate.
#
# The shipped-text guard used to look for two shapes: a halting call site that drops `|| return $?`,
# and a second copy of the predicate named `dirt_rc` or `tree_is_dirty`. Both were drawn from the one
# bug report this branch answers, and a scan drawn from a bug report is a scan for that bug. Measured
# on the head commit of #231 as it was filed: `factory/factory.sh` held no `status --porcelain`, so
# the scan was green while the file held the expression the branch exists to delete, written as
#
#     if [ -n "$(git -C "$tree" status --porcelain 2>/dev/null)" ]; then return 4; fi
#
# which is the same hole the ticket filed -- it answers a stat-cache question, and on the shapes the
# exclude leg measures it exits 128 printing nothing -- in a halting path, and nothing in this file
# could see it. That is how #228 was filed in the first place; a guard that cannot recognise the
# original crime coming back has not retired anything.
#
# The probe is exempt and says so: it has to keep asking the question the shipped code is forbidden to
# ask, because printing what the old expression answers is the only way the legs show the replaced
# behaviour was wrong rather than merely different. Comment lines are cut before the test for the same
# reason `bad_refresh_calls` cuts them -- this file's own prose names the expression to explain why it
# is refused, and a guard that refuses a quotation is a guard that gets widened away (H-0040).
naive_dirt_tests() { # dir -> shipped lines under factory/ that ask git the stat question
	if [ ! -d "$1/factory" ]; then
		# Loud, not empty. A scan pointed at a directory that is not there answers "no hits", and an
		# absence check that cannot tell an empty room from a locked one certifies nothing. One test
		# covers the count and the path the grep takes, so the two cannot drift apart.
		printf 'no factory/ directory under %s -- the scan has nothing to read\n' "$1"
		return 0
	fi
	# `|| true` for the reason in `bad_refresh_calls`: no match is an answer, not a failure, and the
	# caller's assignment would otherwise end the step before it can print the ok.
	grep -rn --include='*.sh' --exclude-dir=.git -- 'status --porcelain' "$1/factory" 2>/dev/null |
		grep -v 'factory/selftest\.sh:' |
		awk '{ code = $0; sub(/#.*/, "", code); if (code ~ /git[^|;&]*status --porcelain/) print }' |
		head -n 5 || true
}

# A gate that can be killed by the shape of the tree it is reading is not a gate. Both shipped-text
# scans are invoked as `hits=$(scan "$dir")`, and under `set -euo pipefail` a scan whose first grep
# finds nothing returns non-zero -- so the assignment ends the script at that line, with no FAIL line
# and no tally. This asks each scan directly for its status on a tree holding one ordinary script and
# nothing to find, in a subshell so that a failing one reports its status instead of ending the run.
# Measured before the `|| true` was added: this leg is red with exit 1 for both scans, which is the
# silent death it exists to catch.
scans_cannot_kill_the_step() { # -> 0 when every shipped-text scan answers 0 on a tree with no matches
	local fn quiet rc=0 bad=''
	quiet=$work/no-match/factory
	mkdir -p "$quiet"
	printf '%s\n' '#!/usr/bin/env bash' 'true' >"$quiet/quiet.sh"
	for fn in bad_refresh_calls naive_dirt_tests; do
		rc=0
		( "$fn" "$work/no-match" >/dev/null 2>&1 ) || rc=$?
		[ "$rc" = 0 ] || bad="${bad:+$bad, }$fn exited $rc"
	done
	if [ -n "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s a scan with nothing to find ends the step: %s. The\n' "no scan can kill the step" "$bad"
		printf '          assignment `hits=$(…)` under `set -euo pipefail` is where it dies, so\n'
		printf '          the gate stops running and prints neither a FAIL line nor a tally.\n'
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-34s a scan with nothing to find answers 0 and the step goes on\n' "no scan can kill the step"
}
scans_cannot_kill_the_step

naive_dirt_test_refused() {
	local label="no stat-question dirt test" root hits
	root=$(cd -- "$HERE/.." && pwd)
	hits=$(naive_dirt_tests "$root")
	if [ -n "$hits" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s a shipped script decides a halt from `status --porcelain`, which asks\n' "$label"
		printf '          a stat-cache question and, on the exclude-file shapes measured above,\n'
		printf '          exits 128 printing nothing -- the hole #228 was filed for:\n'
		printf '%s\n' "$hits" | while IFS= read -r hit; do printf '            %s\n' "$hit"; done
	else
		pass=$((pass + 1))
		printf '  ok    %-34s every halting path asks the content verdict\n' "$label"
	fi

	# Two controls, opposite polarities, both planted outside the repository so the answer about the
	# repository stays an honest statement about what is shipped. The first is the expression this
	# branch deleted from `factory/factory.sh`, verbatim: a scan that cannot see it is blind, and its
	# silence about the tree would mean nothing.
	mkdir -p "$work/naive/factory"
	printf '%s\n' '#!/usr/bin/env bash' \
		'# a halting path that went back to the stat question' \
		'if [ -n "$(git -C "$tree" status --porcelain 2>/dev/null)" ]; then return 4; fi' \
		>"$work/naive/factory/planted.sh"
	if [ -z "$(naive_dirt_tests "$work/naive")" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s the scan did not see the deleted expression planted for it, so its\n' "control: $label"
		printf '          silence about the repository means nothing. Fix the pattern.\n'
	else
		printf '        control: the expression this branch deleted is refused when it is planted in a\n'
		printf '        shipped script, so the ok above is the check working and not the check being blind.\n'
	fi

	# The second is the same expression inside a comment. Refusing prose is how this guard gets deleted:
	# the file that explains a rule would then be the file the rule stops.
	mkdir -p "$work/naive-prose/factory"
	printf '%s\n' '#!/usr/bin/env bash' \
		'# this used to be `if [ -n "$(git status --porcelain)" ]`, and that is the bug' \
		'factory_tree_halt TICK-HALTED "$unit" "$tree" || return $?' \
		>"$work/naive-prose/factory/prose.sh"
	if [ -n "$(naive_dirt_tests "$work/naive-prose")" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s the scan refuses a comment that only quotes the expression, so it is\n' "control: $label"
		printf '          asking about prose and will be widened away inside a week (H-0040).\n'
	else
		printf '        control: the same expression inside a comment is not refused, so the guard\n'
		printf '        reads code and not prose.\n'
	fi
	return 0
}
naive_dirt_test_refused


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
halt_call_sites_shaped

printf 'factory/selftest.sh: %d passed, %d failed\n' "$pass" "$fail"
if [ "$fail" -ne 0 ]; then
	exit 1
fi
