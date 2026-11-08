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

# The plant's claim for the OTHER branch, asked outside the verdict for the same reason: does this read
# come back at its success status WITH something to say? #232's hazard is an incomplete read that
# nothing refuses, so a leg about it is worth nothing on a box where the plant makes git refuse (that
# box is pinned by the unreadable-file leg) or where git says nothing at all (nothing to pin). `arm` is
# the two reads a mode-000 directory is measured to speak on; stdout goes to /dev/null because
# `ls-files --others` puts its list there and the question is about stderr.
read_warns() { # tree arm [want] -> 0 when that read exited `want` (0 unless told) and printed to stderr
	local tree=$1 arm=$2 want=${3:-0} rc=0 errf=$work/read-err
	: >"$errf"
	case "$arm" in
	diff) git -C "$tree" diff --quiet >/dev/null 2>>"$errf" || rc=$? ;;
	others) git -C "$tree" ls-files --others --exclude-standard >/dev/null 2>>"$errf" || rc=$? ;;
	*) return 1 ;;
	esac
	[ "$rc" = "$want" ] && [ -s "$errf" ]
}

# The capture is evidence since #232, so a run that cannot make its scratch file cannot answer at all.
# Built rather than planted: `mktemp` is shadowed inside a command substitution, so the shadow reaches
# `factory_tree_dirt`'s own call and nothing else in this file -- the probe's scratch space, and every
# other leg, still get the real binary. Asserting the reason's own words is the point: the leg the
# verdict would otherwise answer is CLEAN, which is the thing being refused.
no_capture_says_unknown() { # tree -> 0 when a run with no scratch file refuses, and says why
	local out rc=0
	# shellcheck disable=SC2317  # the shadow reads as unreachable code, and it is: to everything except
	# the `mktemp` call `factory_tree_dirt` makes from inside this substitution. The leg is not
	# decorative -- restoring the `/dev/null` fallback is 2 red legs, mutation M12 in docs/factory.md.
	out=$(mktemp() { return 1; }; factory_tree_dirt "$1" 2>&1) || rc=$?
	[ "$rc" = 2 ] || return 1
	case "$out" in *"could not be checked for completeness"*) return 0 ;; esac
	return 1
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

	# The same idea one level up, and this time nothing refuses: a directory nobody can open, with work
	# inside it. Measured on git 2.53.0 at 99f165d, both of the plants below answered CLEAN before the
	# completeness test existed, on every read at or below its success status (#232). Two plants, because
	# they are the two arms' own states and they hide different things: an untracked file nobody has
	# committed, and an EDIT to a tracked file that `diff` reports as unchanged because it could not open
	# the file -- the worse of the two, and the half the issue did not name. The first keeps any tracked
	# file out of the directory so the UNTRACKED arm is the one a state on disk pins; in the second git
	# chokes on the tracked file, so the sentence names that arm. `sub/` is the fixture's own tracked
	# directory, which is what makes the second plant an edit and not a new file.
	#
	# The precondition is the plant's own claim, asked outside the verdict like `planted()` and
	# `tracked_read_refuses()`: this box has to warn instead of refusing. A box where it refuses is
	# already pinned by the unreadable-file leg above, and a box where git says nothing while skipping
	# the directory has no incomplete read to pin -- there the verdict is a CLEAN over somebody's work,
	# which is the bug, not a green leg.
	reset_tree "$tree"
	mkdir -p "$tree/drafts"
	printf 'uncommitted work\n' >"$tree/drafts/patch.txt"
	if [ "$(id -u)" = 0 ]; then
		printf '  SKIP  %-34s running as root, so chmod 000 hides nothing\n' \
			"[$where] work inside an unreadable dir"
	else
		chmod 000 "$tree/drafts"
		if read_warns "$tree" others; then
			expect "[$where] work inside an unreadable dir" "$tree" UNKNOWN strong \
				'git ls-files --others exited 0'
			expect_halt "[$where] work inside an unreadable dir" "$tree" HALT \
				'its untracked files could not be read'
			check "[$where] the reason names the directory" verdict_names "$tree" 'drafts/'
		else
			printf '  SKIP  %-34s this git skipped the directory saying nothing\n' \
				"[$where] work inside an unreadable dir"
			printf '        (git %s), so there is no incomplete read to pin here. The verdict is a\n' \
				"$(git --version | awk '{print $3}')"
			printf '        CLEAN over untracked work on such a box, which is #232, not a pass.\n'
		fi
		chmod 755 "$tree/drafts"
		git -C "$tree" clean -qfd -- drafts >/dev/null 2>&1 || true
	fi

	reset_tree "$tree"
	if [ "$(id -u)" = 0 ]; then
		printf '  SKIP  %-34s running as root, so chmod 000 hides nothing\n' \
			"[$where] an edit inside an unreadable dir"
	else
		printf 'edited by a person, and unreadable to git\n' >"$tree/sub/nested.txt"
		chmod 000 "$tree/sub"
		if read_warns "$tree" diff; then
			expect "[$where] an edit inside an unreadable dir" "$tree" UNKNOWN strong \
				'git diff exited 0'
			expect_halt "[$where] an edit inside an unreadable dir" "$tree" HALT \
				'its tracked files could not be read'
			check "[$where] the reason names the edited file" verdict_names "$tree" 'nested.txt'
		else
			printf '  SKIP  %-34s this git refused or stayed silent over the unreadable\n' \
				"[$where] an edit inside an unreadable dir"
			printf '        directory (git %s), so this shape is not in the tree to pin.\n' \
				"$(git --version | awk '{print $3}')"
		fi
		chmod 755 "$tree/sub"
		reset_tree "$tree"
	fi

	# The other half of the rule, which is what keeps the first half from becoming a machine that
	# swallows every finding git gives. Both trees below hold a readable problem AND an unreadable
	# directory, and both have to answer DIRTY naming the problem -- not UNKNOWN telling somebody who has
	# an edit in front of them to go make the tree readable. Three shapes, and the legs name all three:
	# the incomplete read's OWN finding survives (`untracked: loose-visible.txt`, `modified:
	# tracked.txt`, each carrying git's warning in its clause), and where a read came back empty and the
	# dirt belongs to another arm, `its … were only partly read` is appended rather than replacing the
	# answer. `[ -z "$raw$finding" ]` on the two `ls-files` arms, the success-status lists on the two
	# `diff` arms, and the third branch in each of the three arms that can have a finding in hand are the
	# ways this goes wrong, and each goes red under its own mutation.
	reset_tree "$tree"
	printf 'work nobody has committed, in a readable place\n' >"$tree/loose-visible.txt"
	mkdir -p "$tree/drafts"
	printf 'and more of it, in a directory git cannot open\n' >"$tree/drafts/patch.txt"
	if [ "$(id -u)" = 0 ]; then
		printf '  SKIP  %-34s running as root, so chmod 000 hides nothing\n' \
			"[$where] visible work and an unreadable dir"
	else
		chmod 000 "$tree/drafts"
		if read_warns "$tree" others; then
			expect "[$where] visible work and an unreadable dir" "$tree" DIRTY "" untracked:
			check "[$where] the finding keeps the warning" verdict_names "$tree" 'git said:'
		else
			printf '  SKIP  %-34s no incomplete read on this git (git %s), so there is\n' \
				"[$where] visible work and an unreadable dir" \
				"$(git --version | awk '{print $3}')"
			printf '        nothing for the finding to be preferred over.\n'
		fi
		chmod 755 "$tree/drafts"
		reset_tree "$tree"
	fi

	reset_tree "$tree"
	if [ "$(id -u)" = 0 ]; then
		printf '  SKIP  %-34s running as root, so chmod 000 hides nothing\n' \
			"[$where] an edit and an unreadable dir"
	else
		printf 'ALPHA\nbeta\ngamma\n' >"$tree/tracked.txt"
		chmod 000 "$tree/sub"
		if read_warns "$tree" diff 1; then
			expect "[$where] an edit and an unreadable dir" "$tree" DIRTY "" modified:
			check "[$where] the edit is not downgraded" verdict_names "$tree" 'tracked.txt'
			check "[$where] the part that was missed is said" verdict_names "$tree" 'only partly read'
		else
			printf '  SKIP  %-34s this git did not report the readable edit AND warn\n' \
				"[$where] an edit and an unreadable dir"
			printf '        about the unreadable directory from the same read (git %s), so there\n' \
				"$(git --version | awk '{print $3}')"
			printf '        is no finding to be preferred over an incomplete read here.\n'
		fi
		chmod 755 "$tree/sub"
		reset_tree "$tree"
	fi

	# The third way the guard earns its place, and the only one that reaches the STAGED arm's finding:
	# work in the index, the worktree equal to the index for that file, and a modified tracked file
	# inside a directory nobody can open. Measured on git 2.53.0: the refresh answers 1 with
	# `sub/nested.txt: needs update` on its stdout (not stderr, which is what makes the refresh the one
	# arm whose success is two statuses), `diff --cached` answers 1, and `diff --quiet` answers 0 with
	# `sub/nested.txt: Permission denied` -- so the finding is in hand from the staged arm and the
	# tracked read has to append rather than replace. It does, and the untracked read appends beside it.
	# Deleting the guard from the tracked arm is mutation M8 and this is the leg that goes red on it;
	# nothing else in the suite reaches the tracked arm with a finding already in hand.
	reset_tree "$tree"
	if [ "$(id -u)" = 0 ]; then
		printf '  SKIP  %-34s running as root, so chmod 000 hides nothing\n' \
			"[$where] staged work and an unreadable dir"
	else
		printf 'STAGED\nbeta\ngamma\n' >"$tree/tracked.txt"
		git -C "$tree" add -- tracked.txt
		printf 'edited, and unreadable to git\n' >"$tree/sub/nested.txt"
		chmod 000 "$tree/sub"
		if [ -n "$(git -C "$tree" diff --cached --name-only 2>/dev/null)" ] && read_warns "$tree" diff 0; then
			expect "[$where] staged work and an unreadable dir" "$tree" DIRTY strong \
				'staged-but-uncommitted:'
			check "[$where] the tracked read is said to be partial" \
				verdict_names "$tree" 'tracked files were only partly read'
		else
			printf '  SKIP  %-34s this git did not hold the staged finding and warn over\n' \
				"[$where] staged work and an unreadable dir"
			printf '        the unreadable directory from the same pair of reads (git %s).\n' \
				"$(git --version | awk '{print $3}')"
		fi
		chmod 755 "$tree/sub"
		reset_tree "$tree"
	fi

	# The capture is evidence since #232, so losing it is not a verdict: the same run, with `mktemp`
	# shadowed so there is nowhere to keep what the reads print, has to refuse. Built rather than
	# planted, and pinned by its own words -- the answer this leg refuses is CLEAN, which is what the
	# old fallback handed back.
	check "[$where] no scratch file, no verdict" no_capture_says_unknown "$tree"
done

# ------------------------------------------------------ one refusal at a time, by construction
#
# Four of the five refusal arms were pinned for the halt and not for the refusal, and no state on disk
# closes that: every shape measured -- `core.excludesFile` at a directory or a `chmod 000` file, a
# poisoned `.git`, an inherited `GIT_DIR`/`GIT_WORK_TREE` pair, a corrupt index -- stops two or more
# reads at once, because that is what an unreadable tree does. Hollowing says the same from the other
# side: with the tracked arm's `return 2` replaced by `:` four legs go red, with the refresh's or the
# staged's one each, and with the unmerged or untracked arm's none.
#
# So the isolation is built rather than found. A `git` first on PATH fails exactly one invocation and
# hands every other argument list to the real binary untouched; one leg per arm asserts that arm's own
# sentence, in the verdict and in the halt. Say what that pins and what it does not: it pins the ARM --
# this read's refusal, on its own, halts the pass and names itself -- and it makes no claim that git
# refuses this way by itself. The planted legs above are the evidence for that, in both trees, on the
# filesystems that can or cannot supply it.
#
# Two things in these rows are worth reading rather than skipping. `naive:` on the untracked arm says
# CLEAN: the weaker shape pipes `ls-files --others` through a head and tests the string, so a read that
# died with 128 and no output is a tree with nothing in it -- the false clean #228 was filed for, on
# one row. And the control below is not decoration: with the shim's match pointing at something git
# never runs, these five legs would all still be green if the shim halted by construction, so the leg
# that says "nothing refused" is what makes the five that say "this one refused" mean anything.
shim_install() { # match [exit] -> a bin dir to put first on PATH
	local match=$1 code=${2:-128} dir=$work/arm-shim real
	real=$(command -v git)
	mkdir -p "$dir"
	cat >"$dir/git" <<SHIM
#!/bin/sh
# Answers exactly one git invocation -- refusing (128) or, since #232, succeeding with a warning -- and
# hands every other argument list to the real binary untouched.
case " \$* " in
*" $match "*)
	printf 'shim: %s said something and exited %s\n' '$match' '$code' >&2
	exit $code
	;;
esac
exec '$real' "\$@"
SHIM
	chmod +x "$dir/git"
	printf '%s' "$dir"
}

reset_tree "$repo"
refusal_alone() { # match arm needle
	local match=$1 arm=$2 needle=$3 shim saved=$PATH
	shim=$(shim_install "$match")
	PATH="$shim:$PATH"
	hash -r
	expect "refusal alone: $arm" "$repo" UNKNOWN "" "$needle"
	expect_halt "refusal alone: $arm" "$repo" HALT "$needle"
	PATH=$saved
	hash -r
}
refusal_alone 'update-index --refresh' refresh 'its state could not be read'
refusal_alone 'diff --cached --quiet' staged 'its staged state could not be read'
refusal_alone 'diff --quiet' tracked 'its tracked files could not be read'
refusal_alone 'ls-files --unmerged' unmerged 'its index could not be read'
refusal_alone 'ls-files --others --exclude-standard' untracked 'its untracked files could not be read'

# The control, in the polarity every check in this file carries: a shim whose match names nothing git
# ever runs has to leave the tree CLEAN. Without it the five legs above could be green because the shim
# stops everything it sees, and a refusal check that cannot say "nothing refused" says nothing about
# the arms it is meant to isolate. It doubles as the plant's own precondition: this tree is clean, so
# the only thing the five legs can be reacting to is the refusal.
control_path=$PATH
control_shim=$(shim_install 'definitely-not-a-git-subcommand')
PATH="$control_shim:$PATH"
hash -r
expect 'refusal control: inert shim' "$repo" CLEAN
PATH=$control_path
hash -r

# ------------------------------------------------- one incomplete read at a time, by construction
#
# The refusal has a twin that no status test reaches, and it needs the same isolation: a read that
# returns its success status and writes to stderr anyway. Nothing on disk produces that for three of the
# five arms -- the mode-000 directory plants above stop two arms at once because that is what an
# unreadable directory does -- so, exactly as H-0049 built the refusals, the shape is made: the same
# shim, answering with a warning at the arm's own success status instead of 128.
#
# What these rows pin is the ARM's completeness branch -- this read's warning, on its own, halts the
# pass and names itself and the status it answered at -- and, like the refusal rows, nothing whatever
# about git warning that way by itself. The two plants above are the only evidence for that, and they
# pin the untracked and the tracked arm from real directories. The exit code is in the needle on
# purpose: `git diff exited 0` is the completeness branch, `git diff exited 128` is the refusal, and a
# leg that asserted only `could not be read` would stay green when one degraded into the other -- the
# exact mistake H-0048 was written about, one branch further on. The refresh answers at 1 as its normal
# report, so its row warns at 1: that is the arm whose success is not a single status.
warn_alone() { # match arm needle [exit]
	local match=$1 arm=$2 needle=$3 code=${4:-0} shim saved=$PATH
	shim=$(shim_install "$match" "$code")
	PATH="$shim:$PATH"
	hash -r
	expect "warns alone: $arm" "$repo" UNKNOWN "" "$needle"
	expect_halt "warns alone: $arm" "$repo" HALT "$needle"
	PATH=$saved
	hash -r
}
warn_alone 'update-index --refresh' refresh 'git update-index --refresh exited 1' 1
warn_alone 'diff --cached --quiet' staged 'git diff --cached exited 0'
warn_alone 'diff --quiet' tracked 'git diff exited 0'
warn_alone 'ls-files --unmerged' unmerged 'git ls-files --unmerged exited 0'
warn_alone 'ls-files --others --exclude-standard' untracked 'git ls-files --others exited 0'

# The control again, because a shim that warns at exit 0 has to be shown doing nothing to a clean tree
# before five legs that say "this one warned" mean anything: with a match naming nothing git runs, the
# verdict is CLEAN. It also pins the rule's other half from the only direction that can -- a read that
# answers and says NOTHING is an answer.
control_path=$PATH
control_shim=$(shim_install 'definitely-not-a-git-subcommand' 0)
PATH="$control_shim:$PATH"
hash -r
expect 'warns control: inert shim' "$repo" CLEAN
PATH=$control_path
hash -r

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
# way the EXIT STATUS says nothing is wrong -- and since #232 that is no longer the whole story, because
# a read that answers at 0 while printing `warning: unable to access '…'` is now read as an incomplete
# read and the verdict comes back UNKNOWN, not the DIRTY this leg used to assert. Both halt the pass;
# what changed is the sentence and the reason for it. It used to name the file the operator had told git
# to ignore and tell them to commit or discard it, which is advice nobody can take; it now names the
# file git could not read, which is. The warning remains the only thing in the whole run that says why
# the queue stopped, and it used to be thrown away by the same `2>/dev/null` that turned a 128 into a
# clean tree. Asserted in both places it has to land: the verdict's line, and `factory/HALT`, which is
# the whole of what a person finds when the pass is stopped. What is genuinely NEW here is the halt on a
# tree that is clean apart from the unreadable config -- that one used to proceed -- and the two
# completeness legs in the loop above, plus the sixteen real trees read the four non-writing ways
# before this paragraph was written to check that nothing already on this box trips it -- 64 reads, all
# silent. The refresh is kept out of that audit on purpose: it rewrites `.git/index`, and a worktree on
# this box may belong to a person working in it.
ln -s excludes-is-a-loop "$work/excludes-is-a-loop"
git -C "$repo" config core.excludesFile "$work/excludes-is-a-loop"
excl_warn=""
git -C "$repo" ls-files --others --exclude-standard >/dev/null 2>"$work/excl-warn" || true
excl_warn=$(head -n 1 "$work/excl-warn")
if [ -n "$excl_warn" ]; then
	expect "an exclude file git warned about" "$repo" UNKNOWN "" 'git said:'
	expect_halt "an exclude file git warned about" "$repo" HALT 'git said:'
	check "the reason names the exclude file" verdict_names "$repo" 'excludes-is-a-loop'
	printf '        git said `%s`, and the halt now says it back: the operator can see which file\n' "$excl_warn"
	printf '        to fix. It no longer tells them to commit a file they had ignored, because #232\n'
	printf '        made "answered 0 with a warning" a refusal rather than a finding.\n'
else
	expect "an exclude file git warned about" "$repo" DIRTY "" untracked:
	printf '        note: this git dropped the patterns saying nothing, so there is no reason to\n'
	printf '        carry, nothing is incomplete by the rule above, and the leg asserts the verdict\n'
	printf '        alone -- the false halt on the ignored file, which is what it always was.\n'
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

# ---------------------------------------------------------------- the merged claim (#235)
#
# Everything above this line is about a tree on disk. This section is about a queue, and it lives
# here rather than in a file of its own because it needs the same `pass`/`fail` counters, the same
# $work cleanup, and the same `make verify` step -- a second gate under a name nobody runs is how one
# gate ends up not running (#189 is that story).
#
# What it pins is `delivered`: the verdict that an OPEN issue is already answered by a MERGED pull
# request. The class exists because the queue cannot see that fact. Measured 8 Nov on this
# repository, twice in one day: #228's fix merged at 07:08:17Z and a tick was dispatched for #228 at
# 09:35Z on a branch with nothing ahead of `origin/main`, its ceiling spent re-deriving what `main`
# already had (H-0050); eight hours later #232's fix merged at 14:26:31Z as #234, whose only
# reference is `Refs #232`, and `factory/triage.sh` on the tree containing that fix still printed
# `issue 232 unclaimed`.
#
# A fixture `gh` is the only way to drive this, so the fixture is the whole argument and its shape was
# chosen on purpose:
#
#   * The payloads are the live ones, read off GitHub on 8 Nov with `gh pr list --state merged
#     --json number,body,mergedAt,baseRefName`, `gh issue list --state open --json number,updatedAt`
#     and the same GraphQL query triage.sh makes. Bodies are abridged to the sentences that carry a
#     verdict; every number, timestamp, reference and state reason is the live one. Where a leg needs
#     an issue that is not open on GitHub any more -- #149 -- the leg says so on its own line.
#     Abridging them turned out to be a hazard and not only a convenience: the real merged payload for
#     the same window is 632,601 bytes and the fixture is about two kilobytes, and that gap is exactly
#     where the first version of this verdict died on the live repository. The leg that reproduces it
#     pads a body back past 128 KiB rather than pretending the fixture is the repository.
#   * The payloads go through the caller's OWN `--jq` and `--limit`, run by the real jq. A change to
#     what triage.sh asks for -- its field list, its `join`, its window -- changes what these legs
#     see, so the reads are under test and not a transcription of them.
#   * The shim exits 9 on any call it has no payload for and 77 on anything that writes. A leg cannot
#     write to the queue it is only supposed to describe, and cannot go green by answering a call
#     nobody planned for.
#   * One fixture, plus nineteen single-edit mutations of the shipped files, each named for the rule it
#     breaks. The live table by itself proves little -- seven of the nine rows are NOT delivered, so a
#     classifier that printed `unclaimed` for everything would pass it. Every rule gets the polarity
#     where it bites and the polarity where the very same row gets through. The mutations are not
#     decoration: three of the legs below were written because a mutation survived, and the numbers are
#     in H-0053 rather than here, because a tally that does not name the commit it was measured at is a
#     description of nothing.
#
# The live table these legs are built from, measured 8 Nov. It is the reason the verb is not the
# verdict:
#
#   #232  merged PR #234 `Refs #232`   DELIVERED   H-0050 is the fix
#   #228  merged PR #231 `Refs #228`   DELIVERED   the tick that found this closed it as satisfied
#   #222  merged PR #227 `Refs #222`   not         closed 6 Nov 13:17:20Z, REOPENED 6 Nov 16:27:23Z
#                                                 with a corrected row; #227's body says what stays open
#   #213  merged PR #229 `Refs #213`   not         a bundle, and it spoke again at 02:46:41Z after
#   #204  merged PR #229 `Refs #204`   not         the 01:33:46Z merge: vetoed twice, so the count
#   #199  merged PR #229 `Refs #199`   not         rule alone is decided by the #199 row alone
#   #149  merged PR #218, no verb      not         "Not here: #149's per-control commit policy"
#
# #222 and #232 are identical in every way the verb can see -- one reference each, the same head
# branch shape, silence on the issue after the merge -- and only `stateReason` separates them. Hence
# the leg that hands triage.sh #222's body exactly as GitHub holds it, "Why `#222` was reopened"
# included, with the REOPENED record taken out, and demands `delivered`. A classifier that read
# bodies for a sentence about unfinished work would fail that leg, which is precisely why it exists.
claim_work=$work/claims
claim_bin=$claim_work/bin
claim_dir=$claim_work/factory
claim_repo=$claim_work/repo
claim_fix=$claim_work/fixture
claim_err=$claim_work/stderr
claim_rows_file=$claim_work/rows
claim_window=100
mkdir -p "$claim_bin" "$claim_dir" "$claim_repo" "$claim_fix"

# The repository is here for two preflight questions and nothing else: `is this a git repository` has
# to answer yes, and the orphan pass has to be able to fail `rev-parse origin/main`. With no remote
# the orphan half prints its own warning and contributes no rows, which is also what keeps the row set
# exactly the issue set the leg planted. No leg is about this tree.
git init -q "$claim_repo"
git -C "$claim_repo" -c user.email=fixture@invalid -c user.name=fixture \
	commit -q --allow-empty -m 'fixture seed'

cat >"$claim_bin/gh" <<'CLAIMSHIM'
#!/usr/bin/env bash
# Fixture gh for the merged-claim legs in factory/selftest.sh. It answers exactly the reads
# factory/triage.sh makes, from the payload files $CLAIM_FIXTURE holds, and refuses everything else.
#
# The two refusals are the point. An unhandled call exits 9, because a shim that shrugs and prints
# `[]` at a call nobody planned for turns a leg green without exercising anything. Anything that
# writes exits 77, because `gh` is otherwise the real binary on a real PATH and a leg that commented
# on a real issue would be writing to the queue it exists only to describe.
set -eu
here=${CLAIM_FIXTURE:?CLAIM_FIXTURE names the payload directory}
jqprog=''
limit=''
json=''
state=''
args=("$@")
i=0
while [ "$i" -lt "${#args[@]}" ]; do
	case "${args[$i]}" in
	--jq) jqprog=${args[$((i + 1))]} ;;
	--limit) limit=${args[$((i + 1))]} ;;
	--json) json=${args[$((i + 1))]} ;;
	--state) state=${args[$((i + 1))]} ;;
	esac
	i=$((i + 1))
done

emit() { # emit <payload> -> through the caller's own --limit and --jq, both run by the real jq
	local file=$1
	if [ ! -f "$file" ]; then
		printf 'claim-shim: no payload for this call: %s\n' "$file" >&2
		exit 8
	fi
	if [ -n "$jqprog" ]; then
		if [ -n "$limit" ]; then
			# --limit is under test, not worked around: the window leg means nothing unless a
			# smaller --limit really returns fewer rows. A non-numeric --limit makes jq fail,
			# which is the right answer to a caller that passed one.
			jq -c --argjson n "$limit" 'if type == "array" then .[:$n] else . end' <"$file" |
				jq -r "$jqprog"
		else
			jq -r "$jqprog" <"$file"
		fi
	elif [ -n "$limit" ]; then
		jq -c --argjson n "$limit" 'if type == "array" then .[:$n] else . end' <"$file"
	else
		cat "$file"
	fi
}

# Anything that changes the queue is refused by VERB, matched before the dispatch below can answer
# it from a payload. A dozen spelled-out call shapes would be a list that goes stale the day triage.sh
# learns a new verb, and the failure that would cost is a leg writing to the queue it exists to
# describe. One line, because bash will not carry a pattern list across a newline.
case "$1:$2" in
'pr:merge' | 'pr:edit' | 'pr:ready' | 'pr:review' | 'pr:comment' | 'pr:request-changes' | 'pr:delete' | 'issue:close' | 'issue:reopen' | 'issue:comment' | 'issue:edit' | 'issue:lock')
	printf 'claim-shim: a leg tried to WRITE: %s\n' "$*" >&2
	exit 77 ;;
esac

case "$1 $2" in
'auth status') exit 0 ;;
'api user') emit "$here/user.json" ;;
'api graphql')
	[ -z "${CLAIM_GRAPHQL_FAIL:-}" ] || {
		printf 'claim-shim: the queue query fails on purpose\n' >&2
		exit 6
	}
	emit "$here/graphql.json" ;;
'pr list')
	case "$state" in
	open)
		# One read, two field lists: the bulk PR fields, then the bodies for the second claim
		# channel. Distinguished by --json, which is the only thing that differs between them; a
		# shim that served the wrong one would make `claimed_issues` silently empty.
		case "$json" in
		*body*) emit "$here/pr-bodies.json" ;;
		*) emit "$here/pr-open.json" ;;
		esac ;;
	merged)
		[ -z "${CLAIM_MERGED_FAIL:-}" ] || {
			printf 'claim-shim: the merged read fails on purpose\n' >&2
			exit 6
		}
		# Not a guess at a bad day: `gh` answers a rate-limited or half-written response on stdout
		# with prose while still exiting 0, which is how the payload arrived non-JSON in the report
		# this leg is modelled on. The verb is run through jq on every other call, so this one has
		# to bypass emit and write the bytes itself.
		[ -z "${CLAIM_MERGED_GARBAGE:-}" ] || {
			printf 'gh: could not read the pull request bodies (http 502)\n'
			exit 0
		}
		emit "$here/pr-merged.json" ;;
	*)
		printf 'claim-shim: unhandled pr list --state %s\n' "$state" >&2
		exit 9 ;;
	esac ;;
'issue list') emit "$here/issues.json" ;;
'api '*)
	# `${2:-}` is the PATH, and getting this wrong is the fixture's own version of the failure this
	# section exists to catch. `api_get` (common.sh) calls `gh api <path> --jq <program>`, so $2 is the
	# path and $3 is the literal `--jq`: while this read `${3:-}`, no arm below could ever match, every
	# api read exited 9, and `api_get` handed its caller the DEFAULT -- `[]` for reviews, `` for
	# requested reviewers. Both are indistinguishable from a correct answer, so every leg stayed green
	# over a fixture that had never been asked. Measured by adding the #241 legs, which are the first
	# ones whose payload is not the empty list: `#246` with a team holding the request answered
	# `needs_reviewer` because the shim never emitted `reviewers.json` at all.
	case "${2:-}" in
	*requested_reviewers*) emit "$here/reviewers.json" ;;
	*/reviews*) emit "$here/reviews.json" ;;
	*/commits/*) emit "$here/commit.json" ;;
	*)
		printf 'claim-shim: unhandled api path %s\n' "${2:-}" >&2
		exit 9 ;;
	esac ;;
*)
	printf 'claim-shim: unhandled call: %s\n' "$*" >&2
	exit 9 ;;
esac
CLAIMSHIM
chmod +x "$claim_bin/gh"

# The model runtime, also from the fixture, and this one is load-bearing rather than tidy.
# `factory_preflight` (`factory/common.sh:313`) asks for `$FACTORY_OPENCODE` on PATH, every script
# in this directory calls it (`factory/triage.sh:80`) and the legs below exec the shipped
# `triage.sh` -- so all of them were passing or failing on whether the machine running the suite
# happened to have `opencode` installed. Measured at ae8aba8 on a PATH with no `opencode` anywhere,
# which is what the `verify` job is (.github/workflows/ci.yml:21-49 installs Go and nothing else):
# `131 passed, 45 failed`, every failure in this section, stderr `factory: missing on PATH:
# opencode. Nothing was started.` -- the same tally CI printed on the same commit, and the reason
# PR #238 was BLOCKED. Nothing in the section can see that from a box with opencode installed,
# which is every developer's box and no runner.
#
# The stub answers `command -v` and refuses to be run. triage.sh reads the queue and starts no
# model, so a leg that ever exec'd this gets exit 78 and a sentence about it -- the same shape as
# the `gh` shim's refusal to write.
cat >"$claim_bin/opencode" <<'CLAIMRUNTIME'
#!/usr/bin/env bash
# Fixture model runtime for the merged-claim legs in factory/selftest.sh. It exists to satisfy the
# `$FACTORY_OPENCODE` term of factory_preflight and to refuse, loudly, to be anything else.
printf 'claim-runtime: a leg tried to start a model: %s\n' "$*" >&2
exit 78
CLAIMRUNTIME
chmod +x "$claim_bin/opencode"

# The live payloads, abridged as described in the header. The bodies are GitHub's own text.
cat >"$claim_fix/pr-merged.json" <<'JSON'
[
 {"number":234,"baseRefName":"main","mergedAt":"2026-11-08T14:26:31Z",
  "body":"fix(factory): a read that answered at its success status said it could not open the file\n\nRefs #232.\n"},
 {"number":231,"baseRefName":"main","mergedAt":"2026-11-08T07:08:17Z",
  "body":"fix(factory): the dirt verdict asks a content question, so a killed mutation run no longer halts the queue\n\nRefs #228.\n"},
 {"number":229,"baseRefName":"main","mergedAt":"2026-11-08T01:33:46Z",
  "body":"feat(web): the navigation is in the palette and marks the page you are on\n\nRefs #199. Refs #204. Refs #207. Refs #213.\n\n## What this branch does not finish\n\n`#204` stays open on purpose, and two things it owns are not here: name the root page once, and the skip link.\n\n`#213` is untouched as an audit.\n"},
 {"number":227,"baseRefName":"main","mergedAt":"2026-11-08T01:44:14Z",
  "body":"fix(capture): a frame this run did not take keeps the rev its bytes were rendered from\n\nRefs #222.\n\n### Why `#222` was reopened\n\nIt was closed as satisfied row by row, and every row was about `capture`. The row that is wrong is the field the gate reads.\n\n### What stays open, stated rather than glossed\n"},
 {"number":218,"baseRefName":"main","mergedAt":"2026-11-06T00:02:30Z",
  "body":"feat: packages and a release process\n\nCloses #202.\n\nNot here: #149's per-control commit policy.\n"}
]
JSON

# `stateReason` rides on the issue rows because that is the only way a leg can change it, and it is
# consumed by the queue query rebuilt below -- `gh issue list` is asked for number,title,updatedAt
# and nothing else.
#
# Which `updatedAt` values are live: #235, #232, #222, #213, #204 and #199 exactly as GitHub reported
# them on 8 Nov. #228, #212 and #149 are CLOSED there, so no open-issue listing can quote them and
# their stamps are chosen rather than read -- #228's sits before its merge at 07:08:17Z, because an
# issue that spoke after its merge is the veto case, and that veto has two legs of its own already.
# #212 and #149 are planted open for the same reason: the legs that use them are about the shape of a
# sentence in a body, not about those two issues.
cat >"$claim_fix/issues.json" <<'JSON'
[
 {"number":235,"title":"An issue whose fix is already merged goes back to unclaimed","updatedAt":"2026-11-08T09:43:28Z","stateReason":null},
 {"number":232,"title":"A directory git cannot open makes the dirt verdict answer CLEAN","updatedAt":"2026-11-08T07:30:46Z","stateReason":null},
 {"number":228,"title":"A killed mutation run leaves a worktree that halts every unit","updatedAt":"2026-11-06T02:11:40Z","stateReason":null},
 {"number":222,"title":"manifest.py record certifies screenshots its run did not take","updatedAt":"2026-11-06T16:27:23Z","stateReason":"REOPENED"},
 {"number":213,"title":"lock.png and testlock.png were taken with page scripts running","updatedAt":"2026-11-08T02:46:41Z","stateReason":null},
 {"number":212,"title":"The capture artifacts of one run do not reproduce","updatedAt":"2026-11-04T17:12:33Z","stateReason":null},
 {"number":204,"title":"The navigation is unstyled and does not mark the current page","updatedAt":"2026-11-08T01:35:56Z","stateReason":null},
 {"number":199,"title":"Improve the design of the web UI","updatedAt":"2026-11-01T19:23:23Z","stateReason":null},
 {"number":149,"title":"Per-control commit policy","updatedAt":"2026-10-27T00:00:00Z","stateReason":null}
]
JSON

printf '%s' '[]' >"$claim_fix/open-claims.json"
printf '%s' '[]' >"$claim_fix/pr-open.json"
printf '%s' '[]' >"$claim_fix/pr-bodies.json"
printf '%s' '{"login":"someone-else"}' >"$claim_fix/user.json"
printf '%s' '{"users":[],"teams":[]}' >"$claim_fix/reviewers.json"
printf '%s' '[]' >"$claim_fix/reviews.json"
printf '%s' '{"commit":{"committer":{"date":"2026-11-08T00:00:00Z"}}}' >"$claim_fix/commit.json"
cp "$claim_fix/pr-merged.json" "$claim_work/pr-merged.live.json"
cp "$claim_fix/issues.json" "$claim_work/issues.live.json"

# The queue query's payload, derived from the two lists the legs actually edit. Deriving it is what
# stops the state reason a leg planted in `issues.json` from disagreeing with the one triage.sh
# reads: two copies of one fact, and one gate blind to the other, is #189's whole complaint.
claim_graphql() {
	jq -cn --slurpfile i "$claim_fix/issues.json" --slurpfile c "$claim_fix/open-claims.json" '
		{data: {repository: {
			pullRequests: {nodes: ($c[0] | map({number: .number,
				closingIssuesReferences: {nodes: (.claims | map({number: .}))}}))},
			issues: {nodes: ($i[0] | map({number: .number, stateReason: .stateReason}))}
		}}}' >"$claim_fix/graphql.json"
}
claim_graphql

# The shipped classifier, end to end, with the fixture's gh first on PATH. Nothing is stubbed between
# the two: preflight, the jq programs, the claim channels and the emit line all run.
#
# `FACTORY_OPENCODE` is pinned to the shipped default (`factory/common.sh:62`) rather than inherited,
# so the runtime preflight finds is the fixture's in $claim_bin on every host. Left to the
# environment it would name whatever the operator exports, and the section would be green or red
# depending on a variable nobody in it is testing.
claim_rows() {
	: >"$claim_err"
	PATH="$claim_bin:$PATH" \
		FACTORY_DIR="$claim_dir" FACTORY_ROOT="$claim_repo" \
		FACTORY_REPO=example/repo FACTORY_REVIEWER=someoneelse \
		FACTORY_OPENCODE=opencode \
		FACTORY_LOG="$claim_dir/log" FACTORY_LEDGER="$claim_dir/ledger" FACTORY_RUN="$claim_dir/run" \
		FACTORY_DELIVERED_WINDOW="$claim_window" CLAIM_FIXTURE="$claim_fix" \
		CLAIM_MERGED_FAIL="${CLAIM_MERGED_FAIL:-}" CLAIM_GRAPHQL_FAIL="${CLAIM_GRAPHQL_FAIL:-}" \
		CLAIM_MERGED_GARBAGE="${CLAIM_MERGED_GARBAGE:-}" \
		"${CLAIM_TRIAGE:-$HERE/triage.sh}" 2>"$claim_err" | tr '\037' '\t' >"$claim_rows_file" || true
	if [ -n "${CLAIM_DEBUG:-}" ]; then
		printf 'CLAIM DEBUG triage rc-carrying stderr:\n' >&2
		sed 's/^/    /' "$claim_err" >&2
		printf 'CLAIM DEBUG rows:\n' >&2
		sed 's/^/    /' "$claim_rows_file" >&2
	fi
	cat "$claim_rows_file"
}

row_for() { awk -F'\t' -v k="$1" '$2 == k { print; exit }' "$claim_rows_file"; }
claim_class_is() { # claim_class_is <key> <class>
	local row got
	row=$(row_for "$1")
	[ -n "$row" ] || {
		printf '            no row with key %s; the pass printed:\n' "$1" >&2
		sed 's/^/            /' "$claim_rows_file" >&2
		return 1
	}
	got=$(printf '%s' "$row" | cut -f3)
	[ "$got" = "$2" ] || {
		printf '            %s answers %s, not %s: %s\n' "$1" "$got" "$2" "$(printf '%s' "$row" | cut -f4)" >&2
		return 1
	}
}
claim_detail_has() { # claim_detail_has <key> <needle>
	local d
	d=$(row_for "$1" | cut -f4)
	case "$d" in
	*"$2"*) return 0 ;;
	esac
	printf '            key %s detail is: %s\n' "$1" "$d" >&2
	return 1
}
claim_has_row() { # claim_has_row <key> -> the table says something about it. On the merged channel
	# this is the assertion that distinguishes "claimed, then refused" from "never looked at": a
	# bundle and a stranger both answer `unclaimed`, and only the open channel drops a row outright.
	local row
	row=$(row_for "$1")
	[ -n "$row" ] || {
		printf '            nothing in the table mentions %s; the pass printed:\n' "$1" >&2
		sed 's/^/            /' "$claim_rows_file" >&2
		return 1
	}
}
claim_rows_for() { # claim_rows_for <key>… -> the table says something about every one of them
	local k
	for k in "$@"; do
		claim_has_row "$k" || return 1
	done
}
claim_table_has() { # claim_table_has <kind> <key> [class] -> a row of THAT kind, and that class
	local row got
	row=$(awk -F'\t' -v kind="$1" -v k="$2" '$1 == kind && $2 == k { print; exit }' "$claim_rows_file")
	if [ -z "$row" ]; then
		printf '            no %s row for %s at all; the pass printed:\n' "$1" "$2" >&2
		sed 's/^/            /' "$claim_rows_file" >&2
		return 1
	fi
	[ -z "${3:-}" ] && return 0
	got=$(printf '%s' "$row" | cut -f3)
	[ "$got" = "$3" ] || {
		printf '            %s %s answers %s, not %s\n' "$1" "$2" "$got" "$3" >&2
		return 1
	}
}
claim_no_row() { # claim_no_row <key> -> 0 when nothing at all is reported about it
	[ -z "$(row_for "$1")" ] || {
		printf '            %s is still in the table: %s\n' "$1" "$(row_for "$1")" >&2
		return 1
	}
}
claim_err_has() { # claim_err_has <needle> -> the pass said this on stderr
	case "$(cat "$claim_err")" in
	*"$1"*) return 0 ;;
	esac
	printf '            stderr was: %s\n' "$(tr '\n' '|' <"$claim_err")" >&2
	return 1
}
claim_rows_say_nothing_about() { # claim_rows_say_nothing_about <kind>
	! grep -q "^$1"$'\t' "$claim_rows_file"
}

# The runtime question, asked as legs. This section execs the shipped triage.sh, so it inherits
# factory_preflight's requirement for a model runtime -- see the stub above $claim_bin/opencode --
# and three things about that arrangement are worth pinning: the stub is what the legs resolve, the
# stub cannot run a model, and the requirement still exists. The third is the one that goes quiet:
# if preflight ever stops asking, the stub satisfies nothing and the section keeps its green anyway.
claim_runtime_refuses() { # the fixture runtime is a PATH entry, never a model
	local out rc=0
	out=$("$claim_bin/opencode" run --auto --agent factory 2>&1) || rc=$?
	if [ "$rc" = 0 ]; then
		printf '            the fixture runtime exited 0, so a leg that started a model would never notice\n' >&2
		return 1
	fi
	case "$out" in
	*'tried to start a model'*) return 0 ;;
	esac
	printf '            the fixture runtime refused without saying why (rc %s): %s\n' "$rc" "$out" >&2
	return 1
}
claim_runtime_is_the_fixtured_one() { # the runtime the legs resolve lives in the fixture
	local got
	got=$(PATH="$claim_bin:$PATH" command -v opencode) || got=''
	if [ "$got" != "$claim_bin/opencode" ]; then
		printf '            the legs resolve opencode at %s -- outside the fixture, so on this host,\n' "${got:-nothing}" >&2
		printf '            and on a host without it the whole section goes red as it did in CI.\n' >&2
		return 1
	fi
}
claim_preflight_asks_for_a_runtime() { # claim_preflight_asks_for_a_runtime <common.sh>
	grep -qF -- 'for tool in git gh "$FACTORY_OPENCODE"; do' "$1" 2>/dev/null
}

# One leg, with the reporter this section needs. `check`'s FAIL sentence blames a plant that did not
# happen in a tree, which is the wrong diagnosis for a queue row: here the row is either present with
# the right class or it is not, and the row itself is the evidence, so it is printed.
claim_leg() { # claim_leg <label> command...
	local label=$1
	shift
	if "$@"; then
		pass=$((pass + 1))
		printf '  ok    %-40s %s\n' "$label" "$*"
	else
		fail=$((fail + 1))
		printf '  FAIL  %-40s %s\n' "$label" "$*"
	fi
	return 0
}

# Mutate one list at a time, from the live fixtures, so what a leg breaks is readable as the filter.
claim_merged() { jq -c "$1" "$claim_work/pr-merged.live.json" >"$claim_fix/pr-merged.json"; }
claim_issues() {
	jq -c "$1" "$claim_work/issues.live.json" >"$claim_fix/issues.json"
	claim_graphql
}
claim_open() { # claim_open <json array of {number,title,body,claims}>
	printf '%s' "$1" >"$claim_fix/open-claims.json"
	jq -c '[.[] | {number, title, headRefName: "fix/planted", isDraft: false,
		reviewDecision: "", mergeable: "UNKNOWN", mergeStateStatus: "UNKNOWN",
		updatedAt: "2026-11-08T05:00:00Z", headRefOid: "0000000", statusCheckRollup: []}]' \
		"$claim_fix/open-claims.json" >"$claim_fix/pr-open.json"
	jq -c '[.[] | {number, body}]' "$claim_fix/open-claims.json" >"$claim_fix/pr-bodies.json"
	claim_graphql
}
claim_reset() {
	CLAIM_MERGED_FAIL=''
	CLAIM_GRAPHQL_FAIL=''
	CLAIM_MERGED_GARBAGE=''
	# Not a fixture to reset so much as a leak to close: a leg that points the harness at a mutated
	# triage.sh for a control must not hand that copy to the next leg by accident.
	CLAIM_TRIAGE=''
	claim_window=100
	claim_merged '.'
	claim_issues '.'
	claim_open '[]'
}
claim_open '[]'

# ------------------------------------------------------ the live table, as measured
claim_rows >/dev/null
claim_leg '#232 delivered by #234' claim_class_is 232 delivered
claim_leg '#228 delivered by #231' claim_class_is 228 delivered
claim_leg '#222 not delivered: REOPENED' claim_class_is 222 unclaimed
claim_leg '#213 not delivered: bundled' claim_class_is 213 unclaimed
claim_leg '#204 not delivered: bundled' claim_class_is 204 unclaimed
claim_leg '#199 not delivered: bundled' claim_class_is 199 unclaimed
claim_leg '#149 not delivered: no verb' claim_class_is 149 unclaimed
claim_leg '#235 has nothing merged for it' claim_class_is 235 unclaimed
claim_leg '#212 has nothing merged for it' claim_class_is 212 unclaimed

# The row is the deliverable, so it has to be checkable without re-running anything: which PR, merged
# when, found in what window. `emit` caps the detail at 80 columns, which is why the row carries a
# number and an instant rather than a sentence about them.
claim_leg 'the row names PR #234' claim_detail_has 232 'PR #234'
claim_leg 'the row names the merge instant' claim_detail_has 232 '2026-11-08T14:26:31Z'
claim_leg 'the row says which window it read' claim_detail_has 232 'window 100)'

# ------------------------------------------------------ each rule, at both polarities
# 1. the read itself. No merged list, nothing may be claimed: `unclaimed` is the refusal, and the
#    half that failed is named, because "the queue is empty" and "the queue could not be read" cannot
#    print the same thing.
claim_reset
CLAIM_MERGED_FAIL=1
claim_rows >/dev/null
claim_leg 'unreadable merged list delivers nothing' claim_class_is 232 unclaimed
claim_leg '…and names the half that failed' claim_err_has 'merged pull requests unreadable'

# 2. the bound. `gh pr list --state merged` answers newest-CREATED first, so #231 is the second row
#    regardless of when these branches merged -- which is exactly why a window bounds the read rather
#    than the age of the work. Both halves: what a window of 1 can see it still reports, and what it
#    cannot see it stops reporting, and the row prints the window it used.
claim_reset
claim_window=1
claim_rows >/dev/null
claim_leg 'window 1 delivers the row it sees' claim_class_is 232 delivered
claim_leg 'window 1 loses the claim past it' claim_class_is 228 unclaimed
claim_leg 'window 1 prints the window it used' claim_detail_has 232 'window 1)'

# 3. the human's override: one comment after the merge and the ticket is work again. Only the
#    timestamp moves, so the row that was delivered a moment ago is the control.
claim_reset
claim_issues 'map(if .number == 232 then .updatedAt = "2026-11-08T15:00:00Z" else . end)'
claim_rows >/dev/null
claim_leg 'a comment after the merge re-queues it' claim_class_is 232 unclaimed
claim_leg '…and leaves its neighbour delivered' claim_class_is 228 delivered

# 3b. a missing timestamp is not a quiet issue. Without it the question "did the issue speak after
#     the claim?" cannot be asked, so the verdict refuses rather than silence a live ticket.
claim_reset
claim_issues 'map(if .number == 232 then .updatedAt = "" else . end)'
claim_rows >/dev/null
claim_leg 'an unreadable updatedAt refuses' claim_class_is 232 unclaimed

# 4. `Part of #N`, this repository's own spelling of partial work (#1256). Same PR, same merge, same
#    issue: only two words differ, and the row has to go the other way.
claim_reset
claim_merged 'map(if .number == 234 then .body = "Part of #232; it does not close it.\n" else . end)'
claim_rows >/dev/null
claim_leg '`Part of #232` is partial, not delivered' claim_class_is 232 unclaimed

# 5. the merge has to have reached the trunk. #216's stacked-base chain is this shape every day until
#    the bottom pull request merges, and a merge into an open branch is that branch's news.
claim_reset
claim_merged 'map(if .number == 234 then .baseRefName = "issue-220" else . end)'
claim_rows >/dev/null
claim_leg 'a merge into a branch delivers nothing' claim_class_is 232 unclaimed

# 6. a rejected closure, and the proof that the FIELD carries it. The second leg hands triage.sh
#    #222's body exactly as GitHub holds it -- "Why `#222` was reopened", "What stays open" -- with
#    the REOPENED record removed, and demands `delivered`. A classifier that read bodies for a
#    sentence about unfinished work would fail this leg, which is the point of writing it.
claim_reset
claim_issues 'map(if .number == 222 then .stateReason = null else . end)'
claim_rows >/dev/null
claim_leg '#222 delivered once REOPENED is gone' claim_class_is 222 delivered
claim_leg '…on the body that says it is unfinished' claim_detail_has 222 'Refs #222'
claim_reset
claim_issues 'map(if .number == 222 then .stateReason = "COMPLETED" else . end)'
claim_rows >/dev/null
claim_leg 'a COMPLETED reason is not a rejection' claim_class_is 222 delivered

# 7. a bundle. #229 references four issues and disclaims two of them in its own body; the verdict
#    refuses all four, and accepts #213 the moment that body references only #213. The second half is
#    what proves it is the COUNT of references doing the work and not the prose and not the mention.
#
#    #213's own stamp is moved back before the merge for all three legs. As measured it is
#    02:46:41Z, after #229 merged at 01:33:46Z, so the live row is vetoed twice over and would say
#    nothing about the bundle rule. The live row where the count alone decides is #199, quiet since
#    1 Nov.
claim_reset
claim_issues 'map(if .number == 213 then .updatedAt = "2026-11-08T01:00:00Z" else . end)'
claim_merged 'map(if .number == 229 then .body = "feat(web): the navigation\n\nRefs #213.\n" else . end)'
claim_rows >/dev/null
claim_leg 'one reference from #229 delivers #213' claim_class_is 213 delivered
claim_reset
claim_issues 'map(if .number == 213 then .updatedAt = "2026-11-08T01:00:00Z" else . end)'
claim_merged 'map(if .number == 229 then .body = "feat(web): the navigation\n\nRefs #213. Refs #213.\n" else . end)'
claim_rows >/dev/null
claim_leg 'the same issue twice is one claim' claim_class_is 213 delivered
claim_reset
claim_issues 'map(if .number == 213 then .updatedAt = "2026-11-08T01:00:00Z" else . end)'
claim_merged 'map(if .number == 229 then .body = "feat(web): the navigation\n\nRefs #213. Refs #207.\n" else . end)'
claim_rows >/dev/null
claim_leg 'two issues is a cross-reference' claim_class_is 213 unclaimed

# 8. a mention with no verb claims nothing. #218's sentence about #149 is the live text ("Not here:
#    #149's per-control commit policy"), and PR #215's about #213 is the one the shipped comment in
#    triage.sh already quotes. #149 and #212 are planted open; both legs are about the shape.
claim_reset
claim_open '[{"number":215,"title":"fix(capture): the artifacts of one run reproduce","body":"Refs #212.\n\n**#213 stands and is unchanged.**\n","claims":[]}]'
claim_rows >/dev/null
claim_leg 'an open mention does not claim #213' claim_class_is 213 unclaimed
claim_leg '…while the verb in it claims #212' claim_no_row 212

# 9. the open channel outranks the merged one, by both of its halves: GitHub's own closing reference,
#    and a verb in an open body -- the form #1256 forces, and the one that registers no reference at
#    all. This is the case that used to be the common one and is now a comment in the shipped code.
claim_reset
claim_open '[{"number":236,"title":"fix(factory): the classifier","body":"Part of #232 only; the umbrella is not finished.","claims":[232]}]'
claim_rows >/dev/null
claim_leg 'an open closing reference outranks #234' claim_no_row 232
claim_leg '…and that PR is still polled itself' claim_class_is 236 needs_reviewer
claim_reset
claim_open '[{"number":236,"title":"fix(factory): the classifier","body":"Refs #232.","claims":[]}]'
claim_rows >/dev/null
claim_leg 'an open `Refs #232` outranks #234' claim_no_row 232

# 10. the newest merged claim is the one that counts, and a merge nobody can date is no evidence.
claim_reset
claim_merged '. + [{number: 240, baseRefName: "main", mergedAt: "2026-11-09T00:00:00Z", body: "Refs #232.\n"}]'
claim_rows >/dev/null
claim_leg 'the newest claim is the one named' claim_detail_has 232 'PR #240'
claim_reset
claim_merged '. + [{number: 241, baseRefName: "main", mergedAt: null, body: "Refs #235.\n"}]'
claim_rows >/dev/null
claim_leg 'a merge with no date is no evidence' claim_class_is 235 unclaimed

# 11. the queue payload going unreadable. `delivered` must not be the one row that survives a failed
#     read by falling back on a guess, so it goes the way the closing-reference read already went:
#     the issue half is skipped for the pass and says so.
claim_reset
CLAIM_GRAPHQL_FAIL=1
claim_rows >/dev/null
claim_leg 'an unreadable queue prints no issue rows' claim_rows_say_nothing_about issue
claim_leg '…and the merged claims went with them' bash -c '! grep -q delivered "$1"' _ "$claim_rows_file"

# ------------------------------------------------------ a list, and a payload with a size
#
# 12. a claim can name several issues in one sentence, and this repository's shipped bodies spell it
#     both ways: merged PR #221 `Refs #219, #220, #204`, and open PR #233 `Refs #220 and #222`
#     (measured 8 Nov, both verbatim out of `gh pr list --json body`). A verb plus ONE number reads
#     either line as a claim on the first issue only, and both halves of that are wrong in
#     opposite directions: on the open channel it leaves the second issue to a duplicate tick, and on
#     the merged channel it turns `Closes #10, #11` into a SOLE claim on #10 -- the bundle this
#     verdict exists to refuse, arriving through a comma instead of a second sentence.
#
#    Which two numbers go in the list is the leg's whole power, and getting it wrong was measured:
#    with #228 as the second number a scan that read only the LAST number of the phrase left all 175
#    legs green, because #228 is delivered by PR #231 whatever PR #234 says. The pair that can see it
#    is one whose ends are each otherwise unclaimed -- #235, which nothing merges into, and #213,
#    which #229 only bundles. Then a scan taking the last number delivers #213 and a scan taking the
#    first delivers #235, and each failure surfaces on the OTHER end's leg.
claim_reset
claim_merged 'map(if .number == 234 then .body = "fix: two issues, one sentence\n\nCloses #235, #213.\n" else . end)'
claim_rows >/dev/null
claim_leg 'a comma list claims its first number' claim_class_is 235 unclaimed
claim_leg '…and its last number too' claim_class_is 213 unclaimed
claim_leg '…and neither row is dropped' claim_rows_for 235 213
claim_reset
claim_merged 'map(if .number == 234 then .body = "fix: two issues, one sentence\n\nRefs #235 and #213.\n" else . end)'
claim_rows >/dev/null
claim_leg 'an `and` between numbers is a list' claim_class_is 235 unclaimed
claim_leg '…on both sides of the word' claim_class_is 213 unclaimed
claim_reset
claim_open '[{"number":233,"title":"fix(web): the users page names its own blocking profile","body":"Refs #212 and #213.\n","claims":[]}]'
claim_rows >/dev/null
claim_leg 'an open list claims its first issue' claim_no_row 212
claim_leg '…and its second, on the same line' claim_no_row 213

# 12b. the other side of a list rule: the separator has to BE a separator. Two sentences about two
#      issues are one claim, because the second number has no verb and no list marker in front of it
#      -- `#218`'s live "Not here: #149's per-control commit policy" is the shape, and a rule that
#      swallowed that number would retire #149 from the queue on a sentence saying the opposite.
claim_reset
claim_merged 'map(if .number == 234 then .body = "fix: one issue\n\nRefs #232. The same run also covers #228.\n" else . end)'
claim_rows >/dev/null
claim_leg 'a second sentence is not a list' claim_class_is 232 delivered
claim_reset
claim_merged 'map(if .number == 234 then .body = "fix: one issue\n\nRefs #232.\n\nAlso seen in #228.\n" else . end)'
claim_rows >/dev/null
claim_leg 'a second line is not a list' claim_class_is 232 delivered

# 12c. the live body, with both marks in it at once. #221 writes `Part of #219 only; the umbrella is
#      not finished` eight lines above `Refs #219, #220, #204`, so #219 is refused by two rules and
#      #220 and #204 by one. #219 is planted open because the real one is closed.
claim_reset
claim_issues '. + [{number: 219, title: "The web UI is unusable at 360 px", updatedAt: "2026-11-05T18:00:00Z", stateReason: null}]'
claim_merged '. + [{number: 221, baseRefName: "main", mergedAt: "2026-11-06T03:37:29Z",
	body: "feat(web): the 360 px pass, first tranche\n\nI could not tell what the palette said am I\" once it had loaded (Part of #219 only; the umbrella is not finished — the nine 360 px frames are still open).\n\nRefs #219, #220, #204\n"}]'
claim_rows >/dev/null
claim_leg '#219 refused on Part of and on the list' claim_class_is 219 unclaimed

# 13. the payload has a SIZE. Every other leg in this section feeds triage.sh a fixture of about two
#     kilobytes; the live read it stands for returned 632,601 bytes for 100 merged bodies on 8 Nov.
#     That difference is not a detail -- it is the reason the first version of this verdict did
#     nothing on the real repository: `jq --argjson m "$merged_json"` passes the payload as ONE
#     process argument, Linux refuses any single argument over 128 KiB (MAX_ARG_STRLEN) however large
#     `getconf ARG_MAX` is, the kernel answered E2BIG, and the fallback quietly answered `{}`. Every
#     leg here stayed green throughout, because a two-kilobyte fixture fits in an argument. So this
#     leg's plant is measured before it is believed and the assertion is the ordinary one.
claim_reset
claim_merged '.[0].body = (.[0].body + ([range(2000)]
	| map("a verification section the length real pull-request bodies run to, because they do.\n")
	| add))'
claim_leg 'the plant is past one argument' bash -c '[ "$(wc -c <"$1")" -gt 131072 ]' _ "$claim_fix/pr-merged.json"
claim_rows >/dev/null
claim_leg 'a payload past 128 Ki still delivers' claim_class_is 232 delivered
claim_leg '…and nothing was refused on the way' bash -c '! grep -q refused "$1"' _ "$claim_err"
claim_reset

# 14. a payload that is not JSON. `gh` answers some failures with prose on stdout at exit 0, and the
#     scan's own exit status is then a success: jq reads nothing, prints its complaint, and can answer
#     `null`. `{}` is a legitimate answer to this question, so the only thing that separates "no
#     merged claim qualifies" from "the scan did not happen" is the SHAPE of what came back, which is
#     what the leg asserts -- and the refusal has to be spoken, because the alternative is the silent
#     `unclaimed` this whole class was filed against.
claim_reset
# One open pull request planted beside the garbage, so the leg below has a row to ask about: the
# point is that the OTHER half of the table is still produced, and with an empty open list there
# would be nothing to notice.
claim_open '[{"number":237,"title":"fix(web): the users page names its own blocking profile","body":"Refs #212.\n","claims":[212]}]'
CLAIM_MERGED_GARBAGE=1
claim_rows >/dev/null
claim_leg 'a payload that is not JSON delivers nothing' claim_class_is 232 unclaimed
claim_leg '…and says the scan refused' claim_err_has 'merged-claim scan refused'
# The control for the `|| :` on triage.sh's scan. Without it `set -e` ends the pass at the failed
# substitution and every other row goes with it -- a broken merged read would then stop the whole
# queue, which is a worse failure than the one being refused. Measured that way before the `:` was
# added: the table came back empty and the only words on stderr were jq's.
claim_leg '…and the pull-request rows survive' claim_table_has pr 237 needs_reviewer
claim_leg '…and an unrelated issue is still listed' claim_table_has issue 235 unclaimed
CLAIM_MERGED_GARBAGE=
claim_reset

# 15. the runtime the legs need is the fixture's. Every leg above execs the shipped triage.sh, and
#     factory_preflight wants a model runtime before it will read anything, so this section -- and
#     only this section -- was green or red depending on whether the machine running it happened to
#     have `opencode` installed. That is how PR #238's `verify` went red while every box in the loop
#     stayed green. Measured at ae8aba8 on a PATH with no `opencode` anywhere: `131 passed, 45
#     failed`, stderr `factory: missing on PATH: opencode. Nothing was started.` -- CI's tally on the
#     same commit, because .github/workflows/ci.yml installs Go and nothing else.
claim_leg 'the fixture runtime is what the legs resolve' claim_runtime_is_the_fixtured_one
claim_leg '…and it refuses to start a model' claim_runtime_refuses
claim_leg '…and preflight still asks for one' claim_preflight_asks_for_a_runtime "$HERE/common.sh"
# Control for the leg above, outside the repository: a common.sh with that term deleted has to read
# as no runtime requirement, or the ok is the scan being blind. That is this section's own failure
# arriving from the other direction -- a fixture that answers for something nobody asks for.
mkdir -p "$work/nopreflight/factory"
grep -vF -- 'for tool in git gh "$FACTORY_OPENCODE"; do' "$HERE/common.sh" >"$work/nopreflight/factory/common.sh"
if claim_preflight_asks_for_a_runtime "$work/nopreflight/factory/common.sh" 2>/dev/null; then
	fail=$((fail + 1))
	printf '  FAIL  %-40s a common.sh with the term deleted still asks for a runtime,\n' "control: preflight runtime term"
	printf '          so the ok above is the scan being blind.\n'
else
	printf '        control: a copy with the term deleted reads as no requirement, so the ok\n'
	printf '        above is the scan working and not the check being blind.\n'
fi

# --------------------------------------------------------------- the supervisor's arm
#
# `delivered` is the first class factory.sh has no work for, so the pair of invariants worth pinning
# is: nothing in the dispatcher treats it as work, and every class the classifier can print has an
# arm -- because a class that reaches the `*` arm halts the whole pass with UNKNOWN-CLASS, which is a
# queue that stops on a class name.
delivered_arm_reports_only() {
	local arm
	arm=$(awk '/^[[:space:]]*delivered\)/{f=1} f{print} f&&/;;/{exit}' "$HERE/factory.sh")
	[ -n "$arm" ] || {
		printf '            factory.sh has no `delivered)` arm at all\n' >&2
		return 1
	}
	case "$arm" in
	*start_tick* | *'gh '*)
		printf '            the arm does more than report:\n' >&2
		printf '%s\n' "$arm" | sed 's/^/              /' >&2
		return 1 ;;
	esac
	case "$arm" in
	*log_event*) return 0 ;;
	esac
	printf '            the arm neither starts a tick nor reports anything:\n' >&2
	printf '%s\n' "$arm" | sed 's/^/              /' >&2
	return 1
}
claim_leg 'the delivered arm only reports' delivered_arm_reports_only
# Its control, planted outside the repository: the same scan over a copy whose arm starts a tick has
# to say no, or the ok above is the check being blind.
mkdir -p "$work/armstart/factory"
awk '/^[[:space:]]*delivered\)/{print; print "\t\t\tstart_tick \"issue\" \"$key\" \"$detail\"; continue"; next}
	{print}' "$HERE/factory.sh" >"$work/armstart/factory/factory.sh"
if (HERE=$work/armstart/factory delivered_arm_reports_only) 2>/dev/null; then
	fail=$((fail + 1))
	printf '  FAIL  %-40s an arm that starts a tick was accepted, so the ok above\n' "control: delivered arm"
	printf '          is the check being blind.\n'
else
	printf '        control: the same scan over a copy whose arm calls start_tick says no, so the\n'
	printf '        ok above is the check working and not the check being blind.\n'
fi

case_arm_for() { # case_arm_for <factory.sh> <class> -> 0 when some case pattern names it
	claim_case_words "$1" | grep -qx -- "$2"
}
claim_case_words() { # claim_case_words <factory.sh> -> every lower-case case-pattern word it holds
	# Anchored on the line and on the closing paren, so an argument parser's `--max-ticks)` and an
	# exit-code `0)` cannot pass for a class arm: the character class excludes `-` and digits.
	grep -oE '^[[:space:]]+[a-z_]+( *\| *[a-z_]+)*\)' "$1" 2>/dev/null |
		# The tab has to go too: every case pattern in factory.sh is indented with tabs, and a word
		# that still carries one matches no class name exactly, so every class looked undispatched.
		tr -d ' \t)' | tr '|' '\n' | sort -u
}
triage_emitted_classes() { # triage_emitted_classes <triage.sh> -> every class it can put in column 3
	# Two shapes, on purpose. Every class is emitted inline today, so the awk scan is the live one;
	# the `class=` scan is there for the refactor that decides into a variable and passes "$class",
	# which the awk scan cannot see -- and a check that goes blind on a refactor is exactly the
	# silence this whole section is written against.
	{
		grep -oE '^[[:space:]]*class=[a-z_]+' "$1" 2>/dev/null | sed 's/.*=//'
		awk '{ line = $0; sub(/^[ \t]+/, "", line); if (line ~ /^#/) next;
			  for (i = 1; i < NF; i++)
			    if ($i ~ /emit$/ && $(i + 3) ~ /^[a-z_]+$/) print $(i + 3) }' "$1"
	} | sort -u
}

triage_classes_without_an_arm() { # <root> -> the classes triage.sh can print that factory.sh lacks
	local root=$1 classes class
	classes=$(triage_emitted_classes "$root/factory/triage.sh")
	if [ -z "$classes" ]; then
		printf 'no class could be read out of triage.sh, so this check is blind\n'
		return 0
	fi
	for class in $classes; do
		case_arm_for "$root/factory/factory.sh" "$class" ||
			printf '%s: triage.sh can print it and factory.sh has no arm for it\n' "$class"
	done
}
triage_arm_coverage() {
	local label="every class has a dispatch arm" root bad
	root=$(cd -- "$HERE/.." && pwd)
	bad=$(triage_classes_without_an_arm "$root")
	if [ -n "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s a row would reach the `*` arm and halt the pass with\n' "$label"
		printf '          UNKNOWN-CLASS: a queue that stops because of a class name.\n'
		printf '%s\n' "$bad" | while IFS= read -r line; do printf '            %s\n' "$line"; done
	else
		pass=$((pass + 1))
		printf '  ok    %-40s every class triage.sh can print has an arm; `delivered` starts no tick\n' "$label"
	fi

	# Both controls, planted outside the repository. A presence check that cannot say absent is the
	# check being blind, and there are two ways to be blind here: a class nobody looks for, and an
	# arm nobody deleted.
	mkdir -p "$work/arms/factory"
	sed 's/emit pr "$number" conflict/emit pr "$number" phantom_class/' \
		"$root/factory/triage.sh" >"$work/arms/factory/triage.sh"
	cp "$root/factory/factory.sh" "$work/arms/factory/factory.sh"
	if [ -n "$(triage_classes_without_an_arm "$work/arms")" ]; then
		printf '        control: a class planted in triage.sh with no arm beside it is reported,\n'
		printf '        so the ok above is the check working and not the check being blind.\n'
	else
		fail=$((fail + 1))
		printf '  FAIL  %-40s a planted class with no arm was not reported.\n' "control: $label"
	fi
	mkdir -p "$work/arms2/factory"
	cp "$root/factory/triage.sh" "$work/arms2/factory/triage.sh"
	sed '/^[[:space:]]*delivered)/,+2d' "$root/factory/factory.sh" >"$work/arms2/factory/factory.sh"
	if [ -n "$(triage_classes_without_an_arm "$work/arms2")" ]; then
		printf '        control: deleting the `delivered)` arm is reported too -- it is five lines\n'
		printf '        out of a two-hundred-line dispatcher and nothing else would notice.\n'
	else
		fail=$((fail + 1))
		printf '  FAIL  %-40s deleting the delivered arm was not reported.\n' "control: $label"
	fi
	return 0
}
triage_arm_coverage
claim_reset

# --------------------------------------------------------------- who the review poke asks (#241)
#
# H-0055 moves the review request from one named person to the organisation's team, and H-0056 decides
# what becomes of an individual request the poke did not make. Both are worth legs here rather than a
# grep for `team_reviewers[]` in factory.sh, and the reason is #1048 in shell clothing: a pattern
# matches text, so it reports the absence of a match as the absence of a bug. Delete the line and a
# presence check goes blind; move the line into a function and it says "not found" about working code.
# These legs therefore RUN the poke and read what it asked the API for -- and every load-bearing one
# is run a second time against a copy with the change reverted, where it has to fail, because an
# assertion that cannot tell `team_reviewers[]` from `reviewers[]` is not asserting #241 at all.
#
# How the function is run without starting the supervisor: `factory_request_review` lives in
# factory.sh, which cannot be sourced -- sourcing it runs a pass. So the legs cut the function out of
# the named file and source that into a shell holding common.sh, which is exactly its dependency set:
# `log_event`, `factory_ledger_*`, `factory_age` and `run_action` all live there. The one factory.sh
# call it makes is `factory_maybe_notice`, the operator-nudge, and the harness replaces it with a
# printed line: the nudge is about how long a unit has been stuck, not about who is asked, and a leg
# that reached it would want to see it happen rather than have it POST a comment.
poke_root=$work/poke
mkdir -p "$poke_root/bin"

# A `gh` that keeps a copy of every argument list it was handed. The poke's contract is not "a call
# happened" -- it is "a call happened against `team_reviewers[]`", and both halves of that are
# invisible in an exit status: same endpoint, same verb, same 200 for either list.
#
# The patterns with a space in them are quoted (`*'-X POST'*`), which is not cosmetic: an unquoted
# space ends the pattern, and bash reports the rest of the arm as `syntax error near unexpected
# token 'POST*'` at parse time -- so the shim dies before its first line runs and records nothing.
cat >"$poke_root/bin/gh" <<'POKEGH'
#!/usr/bin/env bash
# Fixture `gh` for the poke legs in factory/selftest.sh. Records, then answers what the poke needs to
# hear. The POST branch comes before the requested_reviewers branch on purpose: a POST matches both.
#
# It answers the way the real binary does: the payload goes through the CALLER's own `--jq` program,
# with any `--raw-field k=v` bound as `--arg k`. A shim that dumps the raw body instead makes every
# read-back fail for a reason unrelated to the code under test -- which is how this fixture briefly
# made the team's 422 path look like a POKE-FAILED and, worse, let the users-only control "pass" for
# that reason rather than for the one it claims.
printf '%s\n' "$*" >>"$POKE_CALLS"
answer() { # answer <payload> -> through the caller's --jq, with --raw-field bound as --arg
	local payload=$1 jqprog='' who='' pair
	local args=("$@") i=0
	while [ "$i" -lt "${#args[@]}" ]; do
		case "${args[$i]}" in
		--jq) jqprog=${args[$((i + 1))]} ;;
		--raw-field)
			pair=${args[$((i + 1))]}
			case "$pair" in
			who=*) who=${pair#*=} ;;
			*)
				printf 'poke-shim: a --raw-field this fixture does not bind: %s\n' "$pair" >&2
				exit 9
				;;
			esac ;;
		esac
		i=$((i + 1))
	done
	if [ -n "$jqprog" ]; then
		printf '%s\n' "$payload" | jq -r --arg who "$who" "$jqprog"
	else
		printf '%s\n' "$payload"
	fi
}
# There is deliberately no `api user` arm. factory_preflight reads the caller's own login to refuse a
# reviewer who is the same account, and `poke_run` runs the poke and nothing else -- an arm here would
# be a fixture answering a question nobody asked, which is how this section went green in CI while
# reading the host (#1048's shape, in a shim). If the poke ever learns to ask, the refusal below is
# the leg's first sentence.
case "$*" in
*'-X POST'*)
	if [ "${POKE_POST_FAIL-0}" = 1 ]; then
		printf 'gh: HTTP 422: Review requests already requested reviewers: cinemaitor\n' >&2
		exit 1
	fi
	answer '{"users":[],"teams":[]}' "$@"
	exit 0
	;;
*requested_reviewers*)
	list=${POKE_LIST-}
	[ -n "$list" ] || list='{"users":[],"teams":[]}'
	answer "$list" "$@"
	exit 0
	;;
esac
printf 'poke-shim: unhandled call: %s\n' "$*" >&2
exit 9
POKEGH
chmod +x "$poke_root/bin/gh"

poke_run() { # poke_run <factory.sh> <FACTORY_REVIEWER> <FACTORY_REVIEWER_TEAM>
	local src=$1 reviewer=$2 team=$3
	local dir rc=0
	dir=$(mktemp -d "$poke_root/run-XXXXXX")
	poke_dir=$dir
	poke_calls=$dir/calls
	poke_log=$dir/journal
	: >"$poke_calls"
	# The function, from the file the leg names. An empty extract is reported, not skipped: a leg
	# that ran nothing would otherwise print an ok over the absence of the thing it tests.
	awk '/^factory_request_review\(\)/{f=1} f{print} f&&/^\}/{exit}' "$src" >"$dir/poke.sh"
	if [ ! -s "$dir/poke.sh" ]; then
		printf '            no factory_request_review() could be cut out of %s -- every leg in\n' "$src" >&2
		printf '            this section is reading nothing at all.\n' >&2
		return 1
	fi
	(
		set -eu
		# shellcheck source=./common.sh
		# shellcheck disable=SC1091
		. "$HERE/common.sh"
		# shellcheck disable=SC1090,SC1091  # the function cut out of factory.sh a few lines above
		. "$dir/poke.sh"
		# shellcheck disable=SC2317,SC2329
		factory_maybe_notice() { printf 'NOTICE %s %s\n' "$1" "$2"; }
		POKE_CALLS=$poke_calls POKE_POST_FAIL=${POKE_POST_FAIL-0} POKE_LIST=${POKE_LIST-} \
			PATH="$poke_root/bin:$PATH" \
			FACTORY_REPO=example/repo FACTORY_OWNER=example FACTORY_NAME=repo \
			FACTORY_LOG=$dir/log FACTORY_LEDGER=$dir/ledger FACTORY_RUN=$dir/run \
			FACTORY_REVIEW_POKES=1800 DRY_RUN=${DRY_RUN-0} \
			FACTORY_REVIEWER=$reviewer FACTORY_REVIEWER_TEAM=$team \
			factory_request_review 241
	) >"$poke_log" 2>&1 || rc=$?
	printf 'rc=%s\n' "$rc" >>"$poke_log"
	return "$rc"
}

poke_post_has() { # <needle> -> the poke POSTed with this argument on the same line
	if awk -v n="$1" '/-X POST/ && index($0, n) { f = 1 } END { exit !f }' "$poke_calls"; then
		return 0
	fi
	printf '            the recorded calls were:\n' >&2
	sed 's/^/              /' "$poke_calls" >&2
	printf '            none of them a POST carrying %s\n' "$1" >&2
	return 1
}
poke_no_post_with() { # <needle> -> no POST was made with this argument
	if awk -v n="$1" '/-X POST/ && index($0, n) { f = 1 } END { exit !f }' "$poke_calls"; then
		printf '            a POST carries %s, and it should not:\n' "$1" >&2
		sed 's/^/              /' "$poke_calls" >&2
		return 1
	fi
	return 0
}
poke_no_write_at_all() { # nothing at all was sent to the API
	if [ -s "$poke_calls" ]; then
		printf '            the poke was configured to ask nobody and still made %s call(s):\n' \
			"$(wc -l <"$poke_calls")" >&2
		sed 's/^/              /' "$poke_calls" >&2
		return 1
	fi
	return 0
}
poke_no_delete() { # H-0056: the poke never un-asks somebody it did not ask
	if grep -q -- '-X DELETE' "$poke_calls"; then
		printf '            the poke revoked a request it did not make:\n' >&2
		sed 's/^/              /' "$poke_calls" >&2
		return 1
	fi
	return 0
}
poke_log_has() { # <needle> -> the supervisor's journal says this
	if grep -qF -- "$1" "$poke_log"; then
		return 0
	fi
	printf '            the journal was:\n' >&2
	sed 's/^/              /' "$poke_log" >&2
	printf '            with nothing matching %s\n' "$1" >&2
	return 1
}
poke_log_says_nothing_about() { # <needle> -> the journal never printed this
	if grep -qF -- "$1" "$poke_log"; then
		printf '            the journal says %s, and it must stay silent:\n' "$1" >&2
		sed 's/^/              /' "$poke_log" >&2
		return 1
	fi
	return 0
}
poke_rc_is() { # <n> -> the function returned this, which is how a refusal is told from a success
	local want=$1 got
	got=$(sed -n 's/^rc=\(.*\)$/\1/p' "$poke_log" | tail -n 1)
	[ "$got" = "$want" ] || {
		printf '            factory_request_review returned %s, not %s:\n' "${got:-nothing}" "$want" >&2
		sed 's/^/              /' "$poke_log" >&2
		return 1
	}
}

# 1. the default asks the team, and the journal says so. Two assertions with different teeth: the
#    argument list, and the line a human reads in `journalctl` when a pull request stalls. Before
#    #241 the journal printed `$FACTORY_REVIEWER` whatever had been POSTed, so the second one is not
#    decoration on the first -- it is the only record of the request that anyone outside this run
#    ever sees.
poke_asks_the_team() {
	poke_run "$1" hermesvicowa cinemaitor || true
	poke_post_has 'team_reviewers[]=cinemaitor'
}
poke_never_asks_the_person_too() {
	poke_run "$HERE/factory.sh" hermesvicowa cinemaitor || true
	poke_no_post_with 'reviewers[]=hermesvicowa'
}
poke_journal_names_the_team() {
	poke_run "$HERE/factory.sh" hermesvicowa cinemaitor || true
	poke_log_has 'POKED pr:241 asked team cinemaitor' &&
		poke_log_says_nothing_about 'asked hermesvicowa' &&
		poke_rc_is 0
}
claim_leg 'the poke asks the team' poke_asks_the_team "$HERE/factory.sh"
claim_leg '…and does not also ask the login' poke_never_asks_the_person_too
claim_leg '…and the journal says which it asked' poke_journal_names_the_team

# 1b. control. The function exactly as shipped, with the team arm's argument and label put back the way
#     they were on 4 Nov -- same endpoint, same verb, a green-looking poke, and one person notified.
#     The leg above must go red over it, or that ok is the check being blind. The copy is made by
#     replacing one exact line rather than by a `sed` pattern: a control that silently matches nothing
#     is the control equivalent of a mutant that never applied (#1048), and the python below exits 3
#     with the line it could not find if the arm is ever reworded.
mkdir -p "$poke_root/reverted"
python3 - "$HERE/factory.sh" "$poke_root/reverted/factory.sh" <<'PY'
import sys
src, dst = sys.argv[1], sys.argv[2]
old = '\tteam:*) arg="team_reviewers[]=$who" label="team $who" ;;'
new = '\tteam:*) arg="reviewers[]=$who" label="$who" ;;'
text = open(src, encoding='utf-8').read()
if old not in text:
    print('the team arm in factory.sh is not the line this control knows: ' + old)
    sys.exit(3)
open(dst, 'w', encoding='utf-8').write(text.replace(old, new, 1))
PY
if poke_asks_the_team "$poke_root/reverted/factory.sh" 2>/dev/null; then
	fail=$((fail + 1))
	printf '  FAIL  %-40s a function reverted to `reviewers[]` still passed the\n' "control: team arm"
	printf '          team leg, so the ok above is the check being blind.\n'
else
	printf '        control: the same leg over a copy whose team arm posts `reviewers[]` says\n'
	printf '        no, so the ok above is the poke asking a team and not the check being blind.\n'
fi

# 2. the individual form, kept working. This is the half that a repository outside this org lives on,
#    and it is the half a "just use the team" change deletes: with the team set empty the poke must
#    still POST `reviewers[]` and must not mention a team at all.
poke_asks_the_person_when_no_team() {
	poke_run "$HERE/factory.sh" someoneelse '' || true
	poke_post_has 'reviewers[]=someoneelse' && poke_no_post_with 'team_reviewers[]'
}
poke_journal_names_the_person() {
	poke_run "$HERE/factory.sh" someoneelse '' || true
	poke_log_has 'POKED pr:241 asked someoneelse' && poke_rc_is 0
}
claim_leg 'an empty team leaves the individual form' poke_asks_the_person_when_no_team
claim_leg '…named as a login, not a team' poke_journal_names_the_person

# 3. neither configured. Not a silent no-op, and not a POST with an empty list: GitHub takes that as
#    "no reviewers requested", triage reclassifies the pull request `needs_reviewer`, and the loop
#    H-0016 was built to end comes back one argument shape away. It refuses, every pass, and says
#    which two variables would fix it.
poke_refuses_with_no_target() {
	poke_run "$HERE/factory.sh" '' '' || true
	poke_no_write_at_all && poke_log_has 'POKE-NONE pr:241' && poke_rc_is 1
}
claim_leg 'no target configured refuses, silently' poke_refuses_with_no_target
# Its control, the other way round: a guard that cannot fire must be caught, because the dangerous
# direction here is the empty POST -- a call that returns 200 and requests nobody. The planted copy
# makes the guard's test false rather than deleting it, which is the difference between testing that
# the guard fires and testing that the code still runs (#1048's lesson again: an inert mutation
# reports a clean run).
mkdir -p "$poke_root/noguard"
python3 - "$HERE/factory.sh" "$poke_root/noguard/factory.sh" <<'PY'
import sys
src, dst = sys.argv[1], sys.argv[2]
old = '\tif [ -z "$target" ]; then'
new = '\tif [ -n "$target" ] && [ -z "$target" ]; then'
text = open(src, encoding='utf-8').read()
if old not in text:
    print('the no-target guard in factory.sh is not the line this control knows: ' + old)
    sys.exit(3)
open(dst, 'w', encoding='utf-8').write(text.replace(old, new, 1))
PY
if (
	poke_run "$poke_root/noguard/factory.sh" '' '' || true
	poke_no_write_at_all && poke_log_has 'POKE-NONE pr:241' && poke_rc_is 1
) 2>/dev/null; then
	fail=$((fail + 1))
	printf '  FAIL  %-40s a poke whose no-target guard cannot fire still passed the\n' "control: no-target guard"
	printf '          refusal leg, so the ok above is the check being blind.\n'
else
	printf '        control: with the guard made unable to fire the same leg goes red -- the empty\n'
	printf '        POST is what it refuses, and nothing refuses it there.\n'
fi

# 4. 422 -- the request is already out there. The read-back is what tells this from a 404, and it has
#    to look in `teams[]`: a reader that only knows `users[]` calls a team-requested pull request
#    POKE-FAILED, which is a red line in the journal about a request that is sitting right there, and
#    the same wrong answer reaches `--dry-run`'s only sibling, the throttle.
poke_already_holds_a_team_request() {
	local POKE_POST_FAIL=1
	local POKE_LIST='{"users":[],"teams":[{"name":"cinemAItor","slug":"cinemaitor","id":19038850}]}'
	poke_run "$HERE/factory.sh" hermesvicowa cinemaitor || true
	poke_log_has 'POKE-ALREADY pr:241 team cinemaitor already holds a request' && poke_rc_is 0
}
claim_leg 'a held team request is not a failure' poke_already_holds_a_team_request

# 4b. the same read-back over the other two ways a team's name reaches a reader. `slug` is what the
#     current REST schema makes required; `login` is the spelling that field had before the slug rename,
#     and it is the one that arrives as `org/team`; `name` is the display name, which this org spells
#     `cinemAItor` over the slug `cinemaitor`. Each of the three has to answer ALREADY, because the
#     alternative reading is POKE-FAILED -- a red journal line about a request that is sitting right
#     there, retried forever.
poke_reads_a_prefixed_login() {
	local POKE_POST_FAIL=1
	local POKE_LIST='{"users":[],"teams":[{"login":"vicowa-botworks/cinemaitor"}]}'
	poke_run "$HERE/factory.sh" hermesvicowa cinemaitor || true
	poke_log_has 'POKE-ALREADY pr:241 team cinemaitor already holds a request' && poke_rc_is 0
}
claim_leg '…an older payload spells it `login`' poke_reads_a_prefixed_login

poke_reads_the_display_name() {
	local POKE_POST_FAIL=1
	local POKE_LIST='{"users":[],"teams":[{"id":19038850,"name":"cinemAItor","privacy":"secret"}]}'
	poke_run "$HERE/factory.sh" hermesvicowa cinemaitor || true
	poke_log_has 'POKE-ALREADY pr:241' && poke_rc_is 0
}
claim_leg '…and a display name in another case' poke_reads_the_display_name

# 4c. control for the read-back: the same two legs over a copy whose read-back is the shipped
#     pre-#241 program, `[.users[].login]`. Both must go red -- that program answers "nobody holds
#     this" to every team request whatever the schema, which is POKE-FAILED.
mkdir -p "$poke_root/usersonly"
python3 - "$HERE/factory.sh" "$poke_root/usersonly/factory.sh" <<'PY'
import sys
src, dst = sys.argv[1], sys.argv[2]
old = '--jq "$JQ_REQUESTED_HOLDS"'
new = '--jq \'[.users[]? | .login // ""] | index($who) != null\''
text = open(src, encoding='utf-8').read()
if old not in text:
    print('the read-back program in factory.sh is not the one this control knows: ' + old)
    sys.exit(3)
open(dst, 'w', encoding='utf-8').write(text.replace(old, new, 1))
PY
# The control passes when the leg it stands for goes red here: the users-only read must answer
# POKE-FAILED to a request that is plainly in the payload, the way it answers to every team request
# whatever the schema -- which is precisely the wrong report this change had to fix.
poke_users_only_readback_red() {
	local POKE_POST_FAIL=1
	local POKE_LIST='{"users":[],"teams":[{"slug":"cinemaitor"}]}'
	poke_run "$poke_root/usersonly/factory.sh" hermesvicowa cinemaitor || true
	poke_log_has 'POKE-FAILED pr:241' && poke_rc_is 1
}
claim_leg 'control: a users-only read-back misses it' poke_users_only_readback_red

# 5. 404 -- the name resolves to nobody. Failure, and the sentence has to carry the override that fixes
#    it, because the commonest version of this is a display name pasted where a slug belongs
#     (`cinemAItor` asks for `cinemaitor`) and the person reading the journal is not in the org's
#     team settings page.
poke_failed_names_the_override() {
	local POKE_POST_FAIL=1
	local POKE_LIST='{"users":[],"teams":[]}'
	poke_run "$HERE/factory.sh" hermesvicowa cinemAItor || true
	poke_log_has 'POKE-FAILED pr:241 team cinemAItor could not be requested' &&
		poke_log_has 'FACTORY_REVIEWER_TEAM' && poke_rc_is 1
}
claim_leg 'an unknown team fails, and says what to set' poke_failed_names_the_override

# 6. H-0056: a request the poke did not make stays where it is. Adding a team does not remove an
#    individual from the list and this function does not either, on the success path or on the
#    already-holding path. The leg is about a call that must NOT happen, so its control is the same
#    function with the DELETE added -- the shape a "clean up the stale request" fix would take.
poke_never_revokes_a_request() {
	poke_run "$HERE/factory.sh" hermesvicowa cinemaitor || true
	poke_no_delete || return 1
	local POKE_POST_FAIL=1
	local POKE_LIST='{"users":[{"login":"hermesvicowa"}],"teams":[]}'
	poke_run "$HERE/factory.sh" hermesvicowa cinemaitor || true
	poke_no_delete
}
claim_leg 'the poke leaves a request it did not make' poke_never_revokes_a_request
# Its control: the same function with the tidy-up a reviewer would write -- revoke the stale individual
# request once the team has been asked. That is the exact shape H-0056 refuses, and the leg above has
# to go red over it.
mkdir -p "$poke_root/revokes"
python3 - "$HERE/factory.sh" "$poke_root/revokes/factory.sh" <<'PY'
import sys
src, dst = sys.argv[1], sys.argv[2]
anchor = '\t\tlog_event "POKED pr:$pr asked $label"'
add = ('\t\trun_action gh api -X DELETE "repos/$FACTORY_OWNER/$FACTORY_NAME/pulls/$pr/requested_reviewers"'
       ' -f "reviewers[]=$FACTORY_REVIEWER" --silent || true')
text = open(src, encoding='utf-8').read()
if anchor not in text:
    print('the POKED line this control hangs the DELETE on is gone from factory.sh: ' + anchor)
    sys.exit(3)
open(dst, 'w', encoding='utf-8').write(text.replace(anchor, anchor + '\n' + add, 1))
PY
poke_revokes_when_told_to() {
	poke_run "$poke_root/revokes/factory.sh" hermesvicowa cinemaitor || true
	poke_no_delete
}
if poke_revokes_when_told_to 2>/dev/null; then
	fail=$((fail + 1))
	printf '  FAIL  %-40s a poke that DELETEs the individual request still passed\n' "control: no revoke"
	printf '          the leg, so the ok above is the check being blind.\n'
else
	printf '        control: the same function with a revoke added after the POST goes red on the\n'
	printf '        same leg, so the ok above is the absence of a DELETE and not a blind grep.\n'
fi

# 7. --dry-run still prints and still does not write. The poke has always honoured that, and the new
#    argument has to appear in the printed line for the same reason it has to appear in the POST:
#    `--dry-run` is how an operator checks who a pass is about to ask before it asks them.
poke_dry_run() {
	local DRY_RUN=1
	poke_run "$HERE/factory.sh" hermesvicowa cinemaitor || true
	poke_log_has '-f team_reviewers[]=cinemaitor' &&
		poke_log_has 'WOULD-POKE pr:241 request review from team cinemaitor' &&
		[ ! -e "$poke_dir/ledger/pr:241.last-poke" ]
}
claim_leg '--dry-run prints the team and writes nothing' poke_dry_run

# ------------------------------------------------------ who preflight is allowed to refuse (#241)
#
# `factory_preflight` refuses to start when FACTORY_REVIEWER is the same account the supervisor acts
# as, because GitHub drops a self-review on the floor and the wait then looks like a slow reviewer.
# That check was written when one variable named one person. With a team configured by default, the
# person running the supervisor is very likely a member of it -- so the unconditioned check refuses to
# start the supervisor over a request it would never have made, which is a false stall on the new
# default's own happy path. These legs run the real preflight over the claim section's fixture: a real
# `git init`-ed repository, the fixture `gh` whose `api user` answers `someone-else`, and
# `FACTORY_REPO` set so the remote read is skipped.
preflight_target() { # preflight_target <reviewer> <team> [common.sh] -> rc 0 when preflight starts
	local reviewer=$1 team=$2 src=${3:-$HERE/common.sh}
	local dir rc=0
	dir=$(mktemp -d "$poke_root/pre-XXXXXX")
	(
		set -eu
		# shellcheck disable=SC1090,SC1091  # the leg's own copy of common.sh, which may be a patched one
		. "$src"
		FACTORY_ROOT=$claim_repo FACTORY_REPO=example/repo \
			FACTORY_REVIEWER=$reviewer FACTORY_REVIEWER_TEAM=$team \
			FACTORY_OPENCODE=opencode CLAIM_FIXTURE="$claim_fix" \
			FACTORY_LOG=$dir/log FACTORY_LEDGER=$dir/ledger FACTORY_RUN=$dir/run \
			PATH="$claim_bin:$PATH" \
			factory_preflight
	) >"$dir/out" 2>&1 || rc=$?
	poke_log=$dir/out
	return "$rc"
}
preflight_refuses_a_self_review() {
	# `preflight_target` returns 0 when the pass STARTS, so the refusal is the non-zero arm. Getting
	# this backwards is how the leg first shipped: it read as a pass on the sentence in the log it was
	# never supposed to be looking at.
	if preflight_target someone-else ''; then
		printf '            preflight accepted a reviewer who IS the supervisor:\n' >&2
		sed 's/^/              /' "$poke_log" >&2
		return 1
	fi
	poke_log_has 'FACTORY_REVIEWER is this same account' || return 1
	poke_log_has 'FACTORY_REVIEWER_TEAM'
}
preflight_starts_for_a_team_it_belongs_to() {
	preflight_target someone-else cinemaitor && return 0
	printf '            preflight refused to start although the request goes to a team, and that\n' >&2
	printf '            that is the new default asking a team, and its own happy path:\n' >&2
	sed 's/^/              /' "$poke_log" >&2
	return 1
}
claim_leg 'a self-review is still refused' preflight_refuses_a_self_review
claim_leg '…and membership of the team is not one' preflight_starts_for_a_team_it_belongs_to

# Control: the guard with its target condition taken out -- the pre-#241 form, which compares the two
# logins whatever the target. It must refuse the leg above, or that ok is the guard being absent
# rather than aimed.
mkdir -p "$poke_root/alwaysself"
python3 - "$HERE/common.sh" "$poke_root/alwaysself/common.sh" <<'PY'
import sys
src, dst = sys.argv[1], sys.argv[2]
old = '\tif [ "$(factory_review_target)" = "user:$FACTORY_REVIEWER" ] && [ -n "$FACTORY_REVIEWER" ] &&'
new = '\tif [ -n "$FACTORY_REVIEWER" ] &&'
text = open(src, encoding='utf-8').read()
if old not in text:
    print('the self-review guard in common.sh is not the line this control knows: ' + old)
    sys.exit(3)
open(dst, 'w', encoding='utf-8').write(text.replace(old, new, 1))
PY
if preflight_target someone-else cinemaitor "$poke_root/alwaysself/common.sh"; then
	fail=$((fail + 1))
	printf '  FAIL  %-40s a guard that ignores the target still started the pass,\n' "control: self-review guard"
	printf '          so the ok above is the guard being gone, not aimed.\n'
else
	printf '        control: the same case over a copy whose guard compares the two logins whatever\n'
	printf '        the target says no -- refused to start over a request it would never have made.\n'
fi

# ------------------------------------------------------ what the classifier counts (#241)
#
# The other half of the change, and the half the issue did not name. `triage.sh` decides
# `needs_reviewer` from `/requested_reviewers`, and a team request arrives in `teams[]` while
# `users[]` stays empty. So a classifier that reads only users calls every team-requested pull
# request unasked -- which does not merely misreport it, it hands it back to the poke: the row is
# `needs_reviewer`, the arm calls `factory_request_review`, GitHub 422s, and the queue re-reads a
# request that is already outstanding every FACTORY_REVIEW_POKES forever. These legs run the shipped
# classifier over a fixture whose only unusual fact is which list is populated.
claim_requested() { # <json> -> who holds a request on every planted pull request
	printf '%s' "$1" >"$claim_fix/reviewers.json"
}

claim_reset
claim_open '[{"number":246,"title":"fix(factory): the poke asks the team","body":"Planted for the #241 legs; no issue reference.\n","claims":[]}]'
claim_requested '{"users":[],"teams":[{"name":"cinemAItor","slug":"cinemaitor","id":19038850}]}'
claim_rows >/dev/null
claim_leg 'a team request is somebody being asked' claim_class_is 246 awaiting_review
claim_leg '…named as a team in the row' claim_detail_has 246 'team:cinemaitor'

claim_reset
claim_open '[{"number":246,"title":"fix(factory): the poke asks the team","body":"Planted for the #241 legs; no issue reference.\n","claims":[]}]'
claim_requested '{"users":[{"login":"hermesvicowa"}],"teams":[]}'
claim_rows >/dev/null
claim_leg 'an individual request still counts' claim_class_is 246 awaiting_review

# The other two spellings of the team's name, through the classifier rather than the poke. `login`
# pre-dates the slug rename and arrives as `org/team`; `name` is the display name, which this org
# capitalises differently from its slug. A row that misses either one is not a cosmetic misreport: it
# is `needs_reviewer`, and `needs_reviewer` is an instruction to poke again.
claim_reset
claim_open '[{"number":246,"title":"fix(factory): the poke asks the team","body":"Planted for the #241 legs; no issue reference.\n","claims":[]}]'
claim_requested '{"users":[],"teams":[{"login":"vicowa-botworks/cinemaitor"}]}'
claim_rows >/dev/null
claim_leg '…however the payload spells it' claim_class_is 246 awaiting_review
claim_leg '…with the prefix off the row' claim_detail_has 246 'team:cinemaitor'

claim_reset
claim_open '[{"number":246,"title":"fix(factory): the poke asks the team","body":"Planted for the #241 legs; no issue reference.\n","claims":[]}]'
claim_requested '{"users":[],"teams":[{"id":19038850,"name":"cinemAItor"}]}'
claim_rows >/dev/null
claim_leg '…or names it only to look at' claim_class_is 246 awaiting_review

# Nobody asked at all. This is the leg that keeps the team half from being written as something that
# is always true -- a program that emitted `team:` for an empty list would satisfy both legs above and
# retire `needs_reviewer`, the class the whole supervisor exists for.
claim_reset
claim_open '[{"number":246,"title":"fix(factory): the poke asks the team","body":"Planted for the #241 legs; no issue reference.\n","claims":[]}]'
claim_requested '{"users":[],"teams":[]}'
claim_rows >/dev/null
claim_leg 'nobody on either list is still needs_reviewer' claim_class_is 246 needs_reviewer

# Control: the shipped classifier with the read put back to `[.users[].login]`, over the team-only
# fixture. It must answer `needs_reviewer` -- the re-poke loop this change is guilty of if the
# classifier is not fixed with the poke.
mkdir -p "$poke_root/clstriage"
python3 - "$HERE/triage.sh" "$poke_root/clstriage/triage.sh" <<'PY'
import sys
src, dst = sys.argv[1], sys.argv[2]
old = '"$JQ_REQUESTED_LIST"'
new = '\'[.users[]? | .login] | join(",")\''
text = open(src, encoding='utf-8').read()
if old not in text:
    print('the requested-reviewers read in triage.sh is not the one this control knows: ' + old)
    sys.exit(3)
open(dst, 'w', encoding='utf-8').write(text.replace(old, new, 1))
PY
cp "$HERE/common.sh" "$poke_root/clstriage/common.sh"
chmod +x "$poke_root/clstriage/triage.sh"
claim_reset
claim_open '[{"number":246,"title":"fix(factory): the poke asks the team","body":"Planted for the #241 legs; no issue reference.\n","claims":[]}]'
claim_requested '{"users":[],"teams":[{"slug":"cinemaitor"}]}'
CLAIM_TRIAGE="$poke_root/clstriage/triage.sh" claim_rows >/dev/null
if claim_class_is 246 needs_reviewer; then
	printf '        control: the same fixture through a classifier that reads only `users[]` answers\n'
	printf '        needs_reviewer -- the row that re-pokes a request already outstanding. So the\n'
	printf '        ok above is the team being counted, and not the fixture forgetting to ask.\n'
else
	CLAIM_TRIAGE='' claim_rows >/dev/null
	fail=$((fail + 1))
	printf '  FAIL  %-40s a users-only classifier still called it awaiting_review,\n' "control: triage read"
	printf '          so the ok above is the check being blind. It answered: %s\n' "$(row_for 246)"
fi
CLAIM_TRIAGE=''
claim_requested '{"users":[],"teams":[]}'
claim_reset


printf 'factory/selftest.sh: %d passed, %d failed\n' "$pass" "$fail"
if [ "$fail" -ne 0 ]; then
	exit 1
fi
