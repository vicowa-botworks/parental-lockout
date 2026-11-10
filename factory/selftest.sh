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
# Four groups of checks are about the shipped text rather than a planted tree, because the failure
# they guard is silent at run time -- a step nobody runs, a git invocation git refuses, a systemd unit
# that says something other than what its generator was handed, and a document whose own commands do
# not run on the machine that reads it:
#
#   wired_into_verify   deleting the line in `verify:` would delete the only gate on `factory/`.
#   refused_refresh…    a refresh written with an option `update-index` does not accept exits 129
#                       into the `2>/dev/null || true` wrapped around it, which is the same output
#                       as a refresh that worked.
#   the two deadlines   `install-timer.sh` rendering one number for both of a unit's deadlines held
#                       the queue in `stop-sigterm` for 91 minutes on 8 Nov (issue #246). These run
#                       the generator with `--dry-run` and read the unit it would have written.
#   the skill's --jq    Step 1 of `.opencode/skills/factory/SKILL.md` tells a tick to run a `--jq`
#                       program that cannot run on the gh the timer executes, and nothing had ever
#                       executed the document to find out (issue #270). This leg runs every quoted
#                       program against a canned payload built from the fields that same line asks
#                       for -- prose a tick runs verbatim is prose that drifts from the binary.
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

pass=0
fail=0
reached_verdict=0
work=$(mktemp -d "${TMPDIR:-/tmp}/plock-factoryselftest-XXXXXX")
# Timestamp zero for the at-risk box. The leg that checks the suite never mirrored fixture dirt into
# the operator's real box has to ask *this run* the question, not the box: `$FACTORY_AT_RISK`'s default
# is where the supervisor puts the only copy of work it rescued from a dead tick, so on the machine
# that runs the timer the box is legitimately non-empty -- and it gets more so every time the harness
# does its job. Measured at 450a628: the supervisor quarantined this very unit's half-finished file at
# 15:12:15Z and wrote `issue-263-quarantine-…-2008e6100.patch` there, and `make verify` on the next
# tick in that tree failed with "1 quarantine patch(es) in ~/prj/at-risk". A leg that goes red because
# a rescue happened is a leg someone will make green by moving the rescue out of the way -- and the
# thing it moves is, by definition, work that exists nowhere else. So the assertion is about the run:
# nothing NEWER than this marker may appear in the box.
at_risk_mark=$work/at-risk-marker
: >"$at_risk_mark"
# The tally at the bottom of this file is the only line that says how many legs ran, and a run that
# dies on its way there never prints it. Measured 9 Nov on CI: a fixture that needed the host's git
# identity (H-0059) ended the script at its own command, this trap wiped the fixture, and the log
# stopped at git's fatal -- no FAIL line, no `N passed, M failed`, and nothing that said where 217
# legs became 236. Finding that meant re-running the suite with the box's identity taken away. So the
# trap now says the one thing a dead run cannot: how far it got, that it never reached a verdict, and
# the status it died with. `reached_verdict` is set by the tally line alone, so a run that reported
# never says it did not.
cleanup() {
	local rc=$?
	if [ "$reached_verdict" != 1 ]; then
		printf 'factory/selftest.sh: RUN INCOMPLETE -- died with exit %d after %d ok and %d FAIL, before any verdict\n' \
			"$rc" "$pass" "$fail" >&2
	fi
	chmod -R u+rwX "$work" 2>/dev/null || true
	rm -rf -- "$work"
}
trap cleanup EXIT INT TERM

# The real mirror box, as it stands BEFORE this file can write anywhere. `quarantine_stays_in_the_fixture`
# compares against this. Taken here, at the top of this file beside the marker and not at the leg two
# thousand lines later -- and with no line number quoted for it, because the one this comment carried
# (`at line ~92`) was eight off the day it was written on `main` and eighteen off after this merge. The
# half being recorded is the half this suite could damage: by the time the leg runs, the quarantine legs
# have each written a mirror and the box can no longer be distinguished from what they did. A global on
# purpose -- it is a reading of the machine, not of a fixture.
at_risk_at_start=''
if [ -d "${HOME:-/var/tmp}/prj/at-risk" ]; then
	at_risk_at_start=$(find "${HOME}/prj/at-risk" -name '*-quarantine-*.patch' 2>/dev/null | sort)
fi

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
	local root=$1 sites ticks merges inner v
	sites=$(grep -rn --include='*.sh' --exclude-dir=.git -- 'factory_tree_stop ' "$root" 2>/dev/null |
		grep -v 'factory/common\.sh:' | grep -v 'factory/selftest\.sh:')
	if [ -z "$sites" ]; then
		printf 'no call site of factory_tree_stop anywhere\n'
		return 0
	fi
	printf '%s\n' "$sites" | grep -v '|| return \$?' || true
	# Both verbs have to be present, because deleting one call site leaves the other one correctly
	# shaped and nothing else in the repository would notice: the tick's guard vanishing is the same
	# hole as the guard being wrong, and it is a smaller diff. Measured -- deleting the tick's whole
	# line was the one mutation of ten that this check first missed.
	ticks=$(printf '%s\n' "$sites" | grep -c 'TICK-HALTED' || true)
	merges=$(printf '%s\n' "$sites" | grep -c 'CONFLICT-HALTED' || true)
	[ "$ticks" -ge 1 ] || printf 'no call site stops a tick (TICK-HALTED): the tree is asked and the answer dropped\n'
	[ "$merges" -ge 1 ] || printf 'no call site stops the conflict pass (CONFLICT-HALTED)\n'
	# One level down, the same duty. Since #248 the halt is the fallback inside `factory_tree_stop`,
	# so a quarantine arm that reaches the halt and drops its exit code is the identical bug the sites
	# above are pinned against -- and nothing outside this check would see it, because the quarantine
	# arms return 0 and a green pass continues with a tree it believes it cleaned.
	inner=$(awk '/^factory_tree_stop\(\) \{/{f=1} f{print} f && /^\}/{exit}' "$root/factory/common.sh" 2>/dev/null)
	if [ -z "$inner" ]; then
		printf 'factory_tree_stop is not defined in %s/factory/common.sh\n' "$root/factory/common.sh"
	else
		printf '%s\n' "$inner" | grep 'factory_tree_halt ' | grep -v '|| return \$?' || true
		for v in TICK-HALTED CONFLICT-HALTED; do
			printf '%s\n' "$inner" | grep -q "factory_tree_halt $v" ||
				printf 'factory_tree_stop never halts with %s: that path cleans nothing and stops nobody\n' "$v"
		done
	fi
	# A second copy of the decision is how the two sites drifted apart in the first place, and the
	# raw verdict is what a new copy would be written against.
	grep -rn --include='*.sh' --exclude-dir=.git -- 'dirt_rc' "$root" 2>/dev/null |
		grep -v 'factory/common\.sh:' | grep -v 'factory/selftest\.sh:' || true
	return 0
}

halt_call_sites_shaped() {
	local label="both halts share the guard" root bad v
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

	# The control, planted outside the repository: a call site that drops the return, a halt arm inside
	# the guard that drops its own, and a second copy of the predicate. Any one of them has to be
	# flagged, or the ok above is the check being blind.
	mkdir -p "$work/sites/factory"
	printf '%s\n' \
		'#!/usr/bin/env bash' \
		'. "$(dirname "$0")/common.sh"' \
		'factory_tree_stop TICK-HALTED "$unit" "$tree"' \
		'local dirt_rc=0' \
		'dirt_rc=$(factory_tree_dirt "$tree")' \
		'if [ "$dirt_rc" -ne 1 ]; then :; fi' >"$work/sites/factory/planted.sh"
	printf '%s\n' \
		'#!/usr/bin/env bash' \
		'factory_tree_stop() {' \
		'	local verb=$1' \
		'	factory_tree_halt TICK-HALTED "$unit" "$tree"' \
		'	return 0' \
		'}' >"$work/sites/factory/common.sh"
	if [ -n "$(halt_call_sites_in "$work/sites")" ]; then
		printf '        control: a planted site that drops the return, a planted halt arm that drops\n'
		printf '        its own, and a planted second copy of the predicate are all flagged, so the ok\n'
		printf '        above is the check working.\n'
	else
		fail=$((fail + 1))
		printf '  FAIL  %-34s neither a dropped return, nor a dropped halt arm, nor a second\n' "control: $label"
		printf '          copy of the predicate was flagged, so its silence about the repository means nothing.\n'
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

# ------------------------------------------------- the quarantine, and the halt that follows it
#
# Issue #248 measured what a dirty worktree used to cost: the tick that died on `issue-247` left four
# modified paths, and because `factory_halted()` is read before triage, 22 units and 15h57m of queue
# went past printing one true sentence into a journal nobody was watching. `factory_tree_stop` is the
# response now -- mirror the tree, clean it, let the rest of the queue run -- and the halt is what is
# left for the cases where that cannot be done for certain.
#
# The legs below drive that function over planted dirt and assert the two things the halt could never
# assert: that the work still exists afterwards *somewhere readable*, and that somebody is told when
# the queue really does stop. Each one has its opposite planted beside it, because an assertion that
# cannot say the other word is not measuring anything:
#
#   the mirror is complete      a tree with one edit and one untracked file has to produce a patch
#                               naming BOTH; the untracked-only leg is the case a `git diff` loses
#                               entirely, so a mirror written from the working tree passes the first
#                               leg and fails that one.
#   a refusal touches nothing   an uncreatable mirror directory must leave the tree exactly as dirty
#                               as it was and halt anyway. This is the arm that proves the cleaner
#                               cannot eat work, and it is the only reason the cleaner is allowed.
#   the streak halts            a second quarantine with no tick between them stops the queue, and the
#                               leg after it shows `TICK-START` bringing the counter back down -- so
#                               the streak leg is not a leg that simply always halts.
#   a rehearsal rehearses       `--dry-run` returns the halt's answer while touching nothing at all,
#                               including the marker. A dry run that wrote `factory/HALT` would halt
#                               the real queue from a demonstration.
#   a halt speaks               the notice posts, is throttled, and says so when the post fails --
#                               with the failure leg asserting the ledger is NOT stamped, because a
#                               stamped failure is a halt that never tells anyone twice.
#
# FACTORY_AT_RISK is redirected to the fixture before the first quarantine and never left at its
# default: the default is a real directory holding the only copies of real work, and a probe that
# wrote there would be a probe that silently tidied the thing it exists to describe.
FACTORY_AT_RISK=$work/at-risk
mkdir -p "$FACTORY_AT_RISK"

q_gh=$work/qbin
q_rec=$work/gh-calls
mkdir -p "$q_gh"
cat >"$q_gh/gh" <<'QGHS'
#!/usr/bin/env bash
# Fixture gh for the halt-notice legs. It records the call, keeps a copy of the body it was handed,
# and exits as $Q_GH_RC says. Anything that is not `issue comment` exits 9: a shim that answers `[]`
# to a call nobody planned for turns a leg green without exercising anything (CLAIMSHIM's rule, and
# the same reason).
set -u
rec=${Q_GH_REC:?Q_GH_REC names the record file}
printf '%s\n' "$*" >>"$rec"
if [ "${1:-}" = issue ] && [ "${2:-}" = comment ]; then
	prev=""
	for a in "$@"; do
		if [ "$prev" = --body-file ]; then
			cp -- "$a" "$rec.body" 2>/dev/null || true
		fi
		prev=$a
	done
	exit "${Q_GH_RC:-0}"
fi
exit 9
QGHS
chmod +x "$q_gh/gh"

q_state() { # tree -> clean tree, no marker, no stale stash, empty journal; the LEDGER IS LEFT ALONE
	local tree=$1
	rm -f -- "$FACTORY_HALT"
	reset_tree "$tree"
	# The exclude-file legs above leave `core.excludesFile` pointing at a symlink loop, and since #232
	# a read that prints `warning: unable to access` is an incomplete read: every verdict in this tree
	# answers UNKNOWN from there on. Measured -- every quarantine leg below then reported
	# `quarantine refused: git stash exited 0 and the tree cannot be read back clean`, which is the
	# harness being right and the fixture being poisoned. These legs are about a tree with work in it, so
	# the config the other legs planted is taken back. (It is also a fact worth knowing on its own: the
	# quarantine refuses rather than stashing into a tree git will not read back, so a unit whose halt
	# says "cannot be read back clean" has an exclude-file problem, not a dirty-tree problem.)
	git -C "$tree" config --unset core.excludesFile >/dev/null 2>&1 || true
	git -C "$tree" stash clear >/dev/null 2>&1 || true
	mkdir -p "$FACTORY_LOG"
	: >"$FACTORY_LOG/supervisor.log"
}

q_reset() { # tree [unit] -> q_state plus that unit's ledger, for a leg that starts from nothing
	local tree=$1 unit=${2:-selftest}
	q_state "$tree"
	factory_ledger_clear "$unit" quarantine-count
	factory_ledger_clear "$unit" halt-notice
}

q_dirty() { # tree -> plant one edit and one untracked file, the pair a mirror has to survive
	printf 'quarantine me\n' >>"$tree/tracked.txt"
	printf 'nobody else has this\n' >"$tree/scratch-notes.txt"
}

q_dirt() { # tree -> how many paths git calls dirty (the fixture's own reading, never a verdict)
	local n=0
	n=$(git -C "$1" status --porcelain 2>/dev/null | grep -c .) || n=0
	printf '%s\n' "$n"
}

q_stash_is_ours() { # tree unit -> 0 when refs/stash exists and carries our own subject
	local s subj
	s=$(git -C "$1" rev-parse -q --verify refs/stash 2>/dev/null) || return 1
	subj=$(git -C "$1" log -1 --format=%s "$s" 2>/dev/null)
	# Same substring rule as the guard: git writes the subject as `On <branch>: <message>`.
	case "$subj" in *"supervisor quarantine $2 "*) return 0 ;; esac
	return 1
}

# The mirror path, taken from the journal line the quarantine wrote (`… stash=<sha> mirror=<path>`).
# The first version of this helper took the newest file in the box, and that is not the same thing: the
# names sort as `<tree>-quarantine-<second>-<stash sha>.patch`, so two quarantines in one second sort
# by stash sha and a leg could grep a previous leg's patch and call it a pass. Asking the journal is
# also the better contract -- it asserts the harness told the reader where the copy is.
q_mirror_of() { # -> path from the newest TREE-QUARANTINED journal line, empty when it names none
	local line="" p=""
	line=$(grep -F 'TREE-QUARANTINED' "$FACTORY_LOG/supervisor.log" 2>/dev/null | tail -n 1) || true
	[ -n "$line" ] || {
		printf '\n'
		return 0
	}
	p=$(printf '%s\n' "$line" | sed -n 's/.*mirror=\([^ ]*\).*/\1/p')
	[ -n "$p" ] || printf '\n'
	printf '%s\n' "$p"
}

# How many mirrors exist, for the legs that have to say "no new file". An existence test on "a patch
# in the box" cannot tell a fresh mirror from one an earlier leg wrote, and that is exactly the
# difference a dry run is being asked about.
q_mirror_count() {
	local n=0
	n=$(find "$FACTORY_AT_RISK" -name '*-quarantine-*.patch' 2>/dev/null | grep -c .) || n=0
	printf '%s\n' "$n"
}

q_journal_has() { # needle -> 0 when the fixture journal carries it
	[ -s "$FACTORY_LOG/supervisor.log" ] || return 1
	grep -qF -- "$1" "$FACTORY_LOG/supervisor.log"
}

# expect_quarantine LABEL TREE WANT [UNIT] [NEEDLES] [ATTEMPT] [WHY]
#   NEEDLES is a comma-separated list of paths the mirror has to contain -- the edit-and-untracked leg
#   asks for both, the untracked-only leg for one. Hard-coding both in the arm made that leg fail for
#   the wrong reason: it is the leg whose whole subject is a tree with nothing tracked to diff.
#   ATTEMPT is the value the streak counter is expected to hold afterwards.
#   WHY is for the HALT expectation only: a comma-separated list of substrings the journal's
#   `TREE-QUARANTINE-REFUSED` line has to carry, and therefore the name of the ARM that refused. With
#   nothing here a HALT leg asserts only that some refusal happened, and five of the six refusal arms
#   in `factory_tree_quarantine` catch each other's fall -- measured 10 Nov: making the
#   `cannot be created` arm inert left `no mirror, so no clean` printing ok, because the render arm two
#   lines below refused instead. Commas split, not spaces: the sentences carry spaces on purpose.
expect_quarantine() { # label tree want [unit] [needles] [attempt] [why]
	local label=$1 tree=$2 want=$3 unit=${4:-selftest} needles=${5:-tracked.txt} want_n=${6:-1}
	local why=${7:-}
	local rc=0 out="" before="" after_mirrors=0 before_mirrors=0 bad="" m="" want_rc=0 needle rest=""

	before=$(q_dirt "$tree")
	before_mirrors=$(q_mirror_count)
	out=$(factory_tree_stop TICK-HALTED "$unit" "$tree" 2>&1) || rc=$?
	after_mirrors=$(q_mirror_count)
	m=$(q_mirror_of)

	case "$want" in
	QUARANTINE)
		want_rc=0
		[ "$rc" = 0 ] || bad="the unit was stopped (exit $rc)"
		[ "$(q_dirt "$tree")" = 0 ] || bad="${bad:+$bad; }the tree is still dirty, so the pass could not have continued"
		[ ! -e "$FACTORY_HALT" ] || bad="${bad:+$bad; }a HALT file was written, so every other unit stopped too"
		q_stash_is_ours "$tree" "$unit" || bad="${bad:+$bad; }nothing is in refs/stash under our own subject"
		# Not "one more file in the box": the patch is named `<tree>-quarantine-<second>-<stash>.patch`,
		# and re-quarantining identical bytes produces the same stash commit, so the honest second write
		# lands on the same path. Nothing is lost -- identical content, and the journal names the file
		# that exists -- so the arm asserts the named file is there, is not empty, and carries this leg's
		# own dirt. Counting files is kept for the arms whose claim is that NOTHING was written.
		[ -n "$m" ] || bad="${bad:+$bad; }the journal names no mirror path, so a reader cannot find the copy"
		if [ -n "$m" ]; then
			[ -f "$m" ] || bad="${bad:+$bad; }the mirror it names, $m, is not a file"
			[ -s "$m" ] || bad="${bad:+$bad; }the mirror $m is zero bytes -- a rescue that is an empty file"
			for needle in ${needles//,/ }; do
				grep -q -- "$needle" "$m" || bad="${bad:+$bad; }the mirror $m does not contain $needle"
			done
		fi
		q_journal_has "TREE-QUARANTINED $unit" || bad="${bad:+$bad; }the journal never says the tree was quarantined"
		q_journal_has "attempt=$want_n" ||
			bad="${bad:+$bad; }the journal never prints attempt=$want_n; the counter reads $(factory_ledger_get "$unit" quarantine-count)"
		[ "$(factory_ledger_get "$unit" quarantine-count)" = "$want_n" ] ||
			bad="${bad:+$bad; }the streak counter reads $(factory_ledger_get "$unit" quarantine-count), want $want_n"
		;;
	HALT)
		want_rc=4
		[ "$rc" = "$want_rc" ] || bad="the tick left $rc, want $want_rc"
		[ -e "$FACTORY_HALT" ] || bad="${bad:+$bad; }no HALT file, so factory_halted() would not stop the pass"
		q_journal_has 'TREE-QUARANTINE-REFUSED' ||
			bad="${bad:+$bad; }the journal never says the quarantine was refused, so the halt looks like the old one"
		# Which arm refused, in its own words. The generic marker above says only that the function
		# stopped, and five of the six refusal arms refuse with the same shape -- so without this the
		# leg cannot tell that the arm it was written for was deleted and a later one caught the fall.
		rest=$why
		while [ -n "$rest" ]; do
			needle=${rest%%,*}
			if [ "$needle" = "$rest" ]; then rest=''; else rest=${rest#*,}; fi
			q_journal_has "$needle" ||
				bad="${bad:+$bad; }the refusal line names no '$needle', so any refusal would satisfy this leg"
		done
		[ "$after_mirrors" = "$before_mirrors" ] ||
			bad="${bad:+$bad; }it wrote a mirror anyway, so a refusal is not the honest arm"
		[ "$(q_dirt "$tree")" = "$before" ] ||
			bad="${bad:+$bad; }a refusal changed the tree: $before path(s) before, $(q_dirt "$tree") after -- the arm that cannot save the work must not move it"
		;;
	STREAK)
		want_rc=4
		[ "$rc" = "$want_rc" ] || bad="the tick left $rc, want $want_rc -- a halt that re-asks whether the tree is dirty after cleaning it evaporates here"
		[ -e "$FACTORY_HALT" ] || bad="${bad:+$bad; }the second accident did not halt the queue"
		q_journal_has 'TREE-STREAK' || bad="${bad:+$bad; }the journal does not name the streak as the reason"
		q_journal_has 'TREE-QUARANTINED' || bad="${bad:+$bad; }the second quarantine was not journaled either"
		q_journal_has "attempt=$want_n" ||
			bad="${bad:+$bad; }the journal never counted this as attempt $want_n, so the streak is not being counted"
		[ "$(q_dirt "$tree")" = 0 ] || bad="${bad:+$bad; }the tree was left dirty while the queue was halted"
		# The work is in a stash AND named by the journal, so the halt is not the only copy.
		[ -n "$m" ] && [ -s "$m" ] || bad="${bad:+$bad; }the halt names no usable mirror, so the work is only in a stash"
		;;
	PROCEED)
		want_rc=0
		[ "$rc" = 0 ] || bad="the unit was stopped (exit $rc)"
		[ ! -e "$FACTORY_HALT" ] || bad="${bad:+$bad; }a HALT file was written for a tree with nothing in it"
		[ "$after_mirrors" = "$before_mirrors" ] || bad="${bad:+$bad; }a clean tree still produced a mirror"
		q_journal_has 'TREE-CLEAN' || bad="${bad:+$bad; }the journal never says which arm it took"
		;;
	DRYRUN)
		want_rc=4
		[ "$rc" = "$want_rc" ] || bad="a dry run let the unit through (exit $rc, want $want_rc)"
		[ "$(q_dirt "$tree")" = "$before" ] ||
			bad="${bad:+$bad; }it changed the tree: $before path(s) before, $(q_dirt "$tree") after"
		[ ! -e "$FACTORY_HALT" ] || bad="${bad:+$bad; }it wrote $FACTORY_HALT, halting the real queue from a rehearsal"
		q_stash_is_ours "$tree" "$unit" && bad="${bad:+$bad; }it stashed real work"
		[ "$after_mirrors" = "$before_mirrors" ] ||
			bad="${bad:+$bad; }it wrote a mirror: $before_mirrors before, $after_mirrors after"
		q_journal_has 'DRY-RUN TREE-QUARANTINE' || bad="${bad:+$bad; }the journal never says it was a rehearsal"
		;;
	*)
		fail=$((fail + 1))
		printf '  FAIL  %-34s no such expectation: %s\n' "$label" "$want"
		return 0
		;;
	esac

	if [ -n "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s %s (want %s)\n' "$label" "$bad" "$want"
		[ -z "$out" ] || printf '          it said: %s\n' "$(printf '%s' "$out" | tr '\n' ' ' | cut -c1-500)"
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-34s %-10s exit=%s dirt=%s %s\n' "$label" "$want" "$rc" "$(q_dirt "$tree")" \
		"${m:+mirror=$(basename -- "$m")}"
	return 0
}

# A refusal arm, planted rather than hoped for. `shim_install` (near the top of this file, written for
# #228's five unreadable reads) answers exactly one git invocation and hands every other argument list
# to the real binary. Three of `factory_tree_quarantine`'s refusal arms have no on-disk shape that
# reaches them -- git will `stash push` into any tree a fixture can build -- so the failure is made
# rather than searched for. The plant is inside the helper on purpose: a leg that asserts a refusal
# must not be able to pass on the dirt the previous leg left behind.
#
# `want` routes NEEDLES twice: HALT asks the journal for the refusing arm's own words, QUARANTINE asks
# the mirror for the paths the patch has to carry. The control is the same helper with a match naming
# nothing git ever runs, because three refusals prove nothing unless an inert shim is shown quarantining
# -- otherwise every one of them is satisfied by a shim that stops everything it sees.
q_arm() { # label tree want match exit needles
	local label=$1 tree=$2 want=$3 match=$4 code=$5 needles=$6 shim="" saved=$PATH
	q_reset "$tree"
	q_dirty "$tree"
	shim=$(shim_install "$match" "$code")
	PATH="$shim:$PATH"
	hash -r
	case "$want" in
	HALT) expect_quarantine "$label" "$tree" HALT selftest '' 1 "$needles" ;;
	*) expect_quarantine "$label" "$tree" QUARANTINE selftest "$needles" 1 ;;
	esac
	PATH=$saved
	hash -r
	return 0
}

echo "factory/selftest.sh: the quarantine that replaced the halt, and the notice the halt never sent"
q_dry_saved=$DRY_RUN

for tree in "$repo" "$wt"; do
	where=primary
	if [ "$tree" = "$wt" ]; then
		where=worktree
	fi

	# One accident, honestly absorbed: the tree is clean again, the work is in a stash AND in a patch,
	# the queue kept running, and the journal says so with a count. Both paths are named in the mirror
	# because a `git diff` alone would have lost the untracked file, and the untracked file is the
	# category of work that has no other copy anywhere.
	# One accident, honestly absorbed: the tree is clean again, the work is in a stash AND in a patch,
	# the queue kept running, and the journal says so with a count. Both paths are named in the mirror
	# because a `git diff` alone would have lost the untracked file, and the untracked file is the
	# category of work that has no other copy anywhere.
	q_reset "$tree"
	q_dirty "$tree"
	expect_quarantine "[$where] edit + untracked" "$tree" QUARANTINE selftest "tracked.txt,scratch-notes.txt" 1

	# The second accident on the unit that has just had one, with the ledger deliberately left alone:
	# two accidents in a row on the same unit is the pattern that means a person should look, so this
	# is the leg where the halt is the right answer -- and the only place the streak can be shown to
	# halt, because a halt that re-asks whether the tree is dirty after cleaning it evaporates.
	q_state "$tree"
	q_dirty "$tree"
	expect_quarantine "[$where] the same unit again" "$tree" STREAK selftest tracked.txt 2

	# The control for the leg above, and it is a real control: a tick entering the unit clears the
	# streak, so after it the very same shape is absorbed again instead of halting. Without this the
	# streak leg would only be demonstrating that the function always halts -- precisely the behaviour
	# #248 was filed against, where one dirty tree stopped twenty-two other units for 15h57m.
	q_reset "$tree"
	factory_ledger_set selftest quarantine-count 2
	factory_quarantine_reset selftest
	q_dirty "$tree"
	expect_quarantine "[$where] a tick ends the streak" "$tree" QUARANTINE selftest tracked.txt 1

	q_reset "$tree"
	expect_quarantine "[$where] nothing was dirty" "$tree" PROCEED

	q_reset "$tree"
	q_dirty "$tree"
	# Saved and restored, NOT `DRY_RUN=1 expect_quarantine …`. Bash leaves an assignment that precedes a
	# *function* call in effect after the function returns, so the prefix form silently left the whole
	# rest of the run in dry-run mode -- and because `factory_ledger_clear` and `factory_notice_halt` are
	# both dry-run no-ops, the notice legs downstream then posted nothing and the ledger never cleared.
	# Measured: two legs simply stopped printing. A rehearsal flag that leaks past the rehearsal is the
	# same class as a dry run that writes: it changes what the run believes it was.
	DRY_RUN=1 expect_quarantine "[$where] dry run touches nothing" "$tree" DRYRUN
	DRY_RUN=$q_dry_saved

	# A tree whose only work is untracked: `git diff` and `git diff --cached` are both empty here, so
	# this is the shape where a mirror written from the working tree is worth nothing and the mirror's
	# content is the only proof the harness captured what it moved.
	q_reset "$tree"
	printf 'only ever untracked\n' >"$tree/never-committed.txt"
	expect_quarantine "[$where] untracked only" "$tree" QUARANTINE selftest never-committed.txt 1

	q_reset "$tree"
	q_dirty "$tree"
	# A regular file where the mirror directory has to be created: `mkdir -p` over a plain non-existent
	# path simply succeeds, and that is what this leg first planted -- it reported a quarantine that
	# worked and proved nothing about the refusal arm.
	: >"$work/blocker"
	FACTORY_AT_RISK=$work/blocker/underneath \
		expect_quarantine "[$where] no mirror, so no clean" "$tree" HALT selftest '' 1 'cannot be created'
	q_reset "$tree"
	FACTORY_AT_RISK=$work/at-risk

	# --------------------------------------------- the refusal arms no on-disk state can reach (H-0079)
	#
	# Two measurements, both taken on 10 Nov against the code as shipped, both of them the reason these
	# legs exist rather than those.
	#
	# (1) With the `git stash failed` guard taken out and the `git stash push` call left in place -- the
	# exact deletion this repository's own rule calls a retired arm (the condition became `… && false`,
	# so the push still ran and only the refusal went blind) -- the suite reported `344 passed, 0 failed`
	# at `450a628` plus this commit. Issue #248's second acceptance leg, "make the stash fail… and assert
	# BOTH halves: the tree is still dirty AND the halt is written", had never been written at all:
	# nothing in the file reaches an arm whose failure only git can produce. That is what the shim is for,
	# and the three legs below are the three arms it plants. The first run of this measurement reported
	# `286 passed, 0 failed` on the tree the fix was written in, which was 22 commits behind `main`; the
	# number here is the one this branch reproduces, and a measurement nobody can re-run on the delivered
	# base is not a measurement.
	#
	# (2) With the `cannot be created` arm above made inert, that leg still printed `ok`, because the
	# render arm two lines below caught the fall and refused with its own sentence. A HALT leg that
	# asserts only `TREE-QUARANTINE-REFUSED` therefore measures "something refused", not "the arm I came
	# for refused" -- which is why every leg here, including the one above, now names its arm.
	#
	# `git refuses to stash` is the leg #248 asked for by name, and the one measurement (1) above is
	# what says it was missing: it is the only leg that plants a failing `git stash push`, and the push
	# arm is the one that decides whether the queue halts with the work still in the tree. The two render
	# legs are also the pair that pins the refusal's REASON. The arm used to run
	# `git stash show -p … >"$tmp" 2>&1` inside a capture: stderr pointed at the file the arm deletes,
	# the captured variable was always empty, and every mirror refusal printed `… :: ` and nothing after
	# the colon (measured against the shipped function: `no mirror could be written to
	# /…/repo-quarantine-….patch :: `). The first leg asks for git's own sentence in the journal line and
	# goes red the day the redirect order is put back; the second distinguishes the half where git
	# SUCCEEDED and produced no bytes, which is a different cause and was reported with one sentence.
	q_arm "[$where] git refuses to stash" "$tree" HALT 'stash push' 128 \
		'git stash failed,shim: stash push'
	q_arm "[$where] git refuses to render" "$tree" HALT 'stash show -p' 128 \
		'rendering stash,shim: stash show -p'
	q_arm "[$where] the render comes back empty" "$tree" HALT 'stash show -p' 0 \
		'rendered an empty mirror'
	q_arm "[$where] an inert shim still quarantines" "$tree" QUARANTINE 'definitely-not-a-git-subcommand' 128 \
		'tracked.txt,scratch-notes.txt'
done

# The mirror has to land in the fixture box and nowhere else. `FACTORY_AT_RISK` defaults to a real
# directory whose whole contents are work that exists nowhere else, so a leg that inherited the
# default would tidy the thing it is describing -- and would do it quietly, because the file it moves
# is by definition a file nobody was watching.
# at_risk_diff <before> <after> -> one line per patch that appeared (`+`, with its mtime) or vanished
# (`-`) between two listings of the mirror box. Its own function because the leg needs a control, and a
# control needs something to hand a doctored input: the box itself cannot be doctored without moving real
# work, which is the one thing this suite exists to forbid. A file the box is hiding -- unreadable, or
# gone between the listing and the question -- prints `mtime unreadable` rather than nothing, so the
# line never disappears from the report. The `- ` half needs `printf --`: without the separator bash
# reads the format string as an option and prints nothing at all (`printf: - : invalid option`, to
# stderr, and an empty string where the whole point of the line was the vanished file) -- found by the
# control below on the first run, which is the argument for it.
at_risk_diff() {
	local before=$1 after=$2 f
	printf '%s\n' "$after" | while IFS= read -r f; do
		[ -n "$f" ] || continue
		printf '%s\n' "$before" | grep -qxF -- "$f" ||
			printf '+ %s (mtime %s)\n' "$f" "$(date -u -r "$f" +%Y-%m-%dT%TZ 2>/dev/null || printf 'unreadable')"
	done
	printf '%s\n' "$before" | while IFS= read -r f; do
		[ -n "$f" ] || continue
		printf '%s\n' "$after" | grep -qxF -- "$f" || printf -- '- %s (gone)\n' "$f"
	done
	return 0
}

quarantine_stays_in_the_fixture() {
	local label="the mirror box is the fixture's" stray real by_mark where now changed
	# `|| true` on every count in this function and the one below it, and it is load-bearing rather
	# than tidy: `find` exits 1 on a directory it cannot open, a command substitution propagates the
	# pipeline's status under `pipefail`, and an assignment that fails under `set -e` ends the script
	# -- no tally, the trap eats the fixture, and the CI symptom this comment block is about. Measured
	# on this box with the marker missing, the shipped shape: `rc=1` and every line after the
	# assignment never ran. What the insulation buys is not nothing, though: it turns "the suite dies"
	# into "the count comes back 0", and 0 is the GREEN answer here. That trade is only affordable
	# because `at_risk_predicate_provable` below asks the same expression to separate two files it can
	# see, and requires 2/1/1 -- a masked 0 there goes red, so a masked 0 here means something.
	stray=$(find "$FACTORY_AT_RISK" -name '*-quarantine-*.patch' 2>/dev/null | wc -l | tr -d '[:space:]') || true
	# Ask whether the box is there before counting what is in it. Two reasons, one of which this line
	# paid for: a GitHub runner has no ~/prj/at-risk, and `find` exits 1 on a directory that does not
	# exist, so the assignment below -- a command substitution whose pipeline is under
	# `set -euo pipefail` at the top of this file, not at a line number somebody has to re-derive --
	# took the whole suite down with it. Measured on the runner at
	# this commit: the last quarantine leg printed at 17:08:12.47Z, `Makefile:103: verify] Error 1`
	# followed 30ms later, and no `N passed, M failed` summary was ever reached, while the same commit
	# here read 242 passed / 0 failed because this machine has the directory. It is the same trap this
	# file already documents twice (see the `|| true is load-bearing` note), and I wrote it in again.
	# The other reason is honesty: where the box does not exist the second half of this assertion
	# cannot fail, so the line says which of the two it checked rather than claiming both.
	if [ -d "$HOME/prj/at-risk" ]; then
		# NOT "the box is empty". The box is the machine's, and this suite is not the only thing that
		# writes into it: `factory/factory.sh` mirrors a second dirty stop there for real, and the timer
		# that runs `factory/factory.sh` is installed on the same box this gate runs on. Measured 10 Nov:
		# `make verify` went red with `1 quarantine patch(es) in /home/rodney/prj/at-risk -- this file has
		# written to the directory that holds the only copies of real work`, about a patch named
		# `issue-263-quarantine-20261110T151215Z-2008e6100.patch` -- written at 15:12:15Z by the
		# supervisor quarantining a real unit, 37 minutes before this file's first run at 15:49Z, with
		# nothing in the box newer than that. Two failures at once: the sentence blamed this suite for an
		# event it did not cause, and the gate became unpassable on exactly the machines where the loop is
		# live -- which is every machine the loop matters on. So the assertion is the one it always meant:
		# THIS RUN did not touch the box, compared against the snapshot taken before any leg could write.
		# It is the stronger claim as well. A box this suite emptied is as much a bug as one it littered,
		# and "empty" passed that while "unchanged" goes red on both directions.
		#
		# Two mechanisms ask about the box here and only one of them decides. The list comparison below
		# is the verdict, and it is the arm main brought: it is mtime-blind, so it sees a patch appear
		# whether or not its stamp moved. `-newer` the marker taken at the top of this file is kept as the
		# cross-check, because it answers something the diff cannot -- of what is in the box, what carries
		# a stamp from after this run started. Neither of them asks the box whether it is EMPTY, which is
		# the question H-0081 records as unanswerable on any machine where the loop is live. See the
		# marker's own note above `cleanup()`.
		#
		# What `-newer` cannot see, priced: it compares mtimes, so its count is only as honest as the
		# stamps. The shipped writer creates the patch with a fresh `>`, so a rescue this run made is
		# always newer than the marker. A rescue carried in some other way -- `cp -p`, `tar -x` without
		# `-m`, a restore out of the box itself -- keeps its ORIGINAL mtime and the marker counts it as
		# "already there, left alone". That is the case the comparison is the verdict FOR: it sees the
		# file appear with the old stamp and goes red. So the priced hole is closed by the arm beside it
		# rather than left to the reader, and the marker's own failure mode -- a stamp newer than the
		# marker on a file that predates the run -- is printed below instead of failed on.
		now=$(find "$HOME/prj/at-risk" -name '*-quarantine-*.patch' 2>/dev/null | sort) || true
		where="$HOME/prj/at-risk"
		changed=$(at_risk_diff "$at_risk_at_start" "$now")
		if [ -n "$changed" ]; then
			fail=$((fail + 1))
			printf '  FAIL  %-34s the real box changed while this file ran:\n' "$label"
			printf '%s\n' "$changed" | sed 's#^#          #'
			printf '          A `+` this suite did not write means the supervisor mirrored real work\n'
			printf '          during the gate. The command that names the unit is:\n'
			printf '            grep -h TREE-QUARANTINE ~/prj/*/factory/log/supervisor.log | tail\n'
			return 0
		fi
		real=$(printf '%s\n' "$now" | grep -c . || true)
		# The cross-check, not the verdict: what in the box is stamped from after this run started?
		by_mark=$(find "$HOME/prj/at-risk" -name '*-quarantine-*.patch' -newer "$at_risk_mark" 2>/dev/null | wc -l | tr -d '[:space:]') || true
	else
		real=0
		by_mark=0
		where="$HOME/prj/at-risk (no box on this host)"
	fi
	if [ "$stray" -lt 1 ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s no mirror was ever written to %s, so the legs above never\n' "$label" "$FACTORY_AT_RISK"
		printf '          proved a mirror exists -- their ok means nothing.\n'
	elif [ "$real" != 0 ]; then
		pass=$((pass + 1))
		printf '  ok    %-34s %s patch(es) in the fixture; the %s already in %s are the\n' "$label" "$stray" "$real" "$where"
		printf '          same set this file started with, so none of them came from this run.\n'
	else
		pass=$((pass + 1))
		printf '  ok    %-34s %s patch(es) in the fixture, none in %s\n' "$label" "$stray" "$where"
	fi
	# The two ways of asking, required to agree. The verdict above is the list comparison; this is
	# the mtime question. They disagree when a patch predates the run yet carries a stamp from after
	# the marker -- so the marker leg alone would have called someone else's rescue this suite's
	# work, the false blame H-0084 was filed for. Printed, not failed on: a weaker signal may
	# annotate the report, it may not decide when enforcement stops. Silence is agreement.
	if [ "${by_mark:-0}" != 0 ]; then
		printf '        note: the marker says %s patch(es) in %s are newer than this run\n' "$by_mark" "$where"
		printf '        started, and the snapshot says the box never changed -- so they were already\n'
		printf '        there with a later stamp, and `-newer` alone would have blamed this file.\n'
	fi
	# The control on the comparison itself, on synthetic lists so it runs on a runner with no
	# ~/prj/at-risk as well. Every `ok` above is a claim about `at_risk_diff`, and an ok from a
	# comparison that cannot see a change is the check being blind -- the failure mode this file already
	# names in its own comments more than any other. Three independent arms, not an if/elif chain: an
	# `elif` after the first failure skips the other two, so a single mutant hid two of the three
	# directions (measured 10 Nov, blinding the comparator: `356 passed, 1 failed` with only the
	# `appeared` arm reported, the `gone` arm never reached).
	if [ -n "$(at_risk_diff '/a/one.patch' '/a/one.patch')" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s an unchanged box was reported as changed:\n' 'control: the diff sees a change'
		printf '          %s\n' "$(at_risk_diff '/a/one.patch' '/a/one.patch')"
	fi
	if [ "$(at_risk_diff '' '/a/two.patch')" != '+ /a/two.patch (mtime unreadable)' ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s a patch that appeared was not named:\n' 'control: the diff sees a change'
		printf '          %s\n' "$(at_risk_diff '' '/a/two.patch')"
	fi
	if [ "$(at_risk_diff '/a/three.patch' '')" != '- /a/three.patch (gone)' ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s a patch that vanished was not named:\n' 'control: the diff sees a change'
		printf '          %s\n' "$(at_risk_diff '/a/three.patch' '')"
	fi
	return 0
}
quarantine_stays_in_the_fixture

# The predicate the leg above now rests on, tried on a box this file owns. It has to be run: with the
# box empty by definition on a runner, the only way `real` can come back wrong is by coming back 0 from
# a `find` that matched nothing -- a mistyped marker path does exactly that, and 0 is the green answer.
# So this plants one rescue from before the run and one written during it, dated a decade apart on
# either side of the marker so no clock granularity is involved, and asks the same expression to tell
# them apart. If it cannot separate two files it can see, its silence about the real box means nothing.
at_risk_predicate_provable() {
	local label='the box check can say the other word' box wrote kept total
	box=$(mktemp -d "$work/at-risk-predicate-XXXX")
	printf 'rescued before this run started\n' >"$box/issue-1-quarantine-20000101T000000Z-aaa.patch"
	touch -d '2000-01-01 00:00:00' "$box/issue-1-quarantine-20000101T000000Z-aaa.patch"
	touch -d '2010-01-01 00:00:00' "$box/marker"
	printf 'written by the run itself\n' >"$box/issue-2-quarantine-20200101T000000Z-bbb.patch"
	touch -d '2020-01-01 00:00:00' "$box/issue-2-quarantine-20200101T000000Z-bbb.patch"
	# Same `|| true` as the box leg, and here it costs nothing to be uniform: this leg requires
	# 2 / 1 / 1, so a `find` that was stopped from looking returns 0 and goes RED. That is the asymmetry
	# the box leg's insulation leans on -- an ignored failure is safe where the expected answer is
	# non-zero, and dangerous where it is zero.
	wrote=$(find "$box" -name '*-quarantine-*.patch' -newer "$box/marker" 2>/dev/null | wc -l | tr -d '[:space:]') || true
	kept=$(find "$box" -name '*-quarantine-*.patch' ! -newer "$box/marker" 2>/dev/null | wc -l | tr -d '[:space:]') || true
	total=$(find "$box" -name '*-quarantine-*.patch' 2>/dev/null | wc -l | tr -d '[:space:]') || true
	if [ "$total" = 2 ] && [ "$wrote" = 1 ] && [ "$kept" = 1 ]; then
		pass=$((pass + 1))
		printf '  ok    %-34s planted 2, the check counts %s as this run and %s as earlier, so\n' "$label" "$wrote" "$kept"
		printf '          the ok above is the predicate answering and not a find that matched nothing.\n'
	else
		fail=$((fail + 1))
		printf '  FAIL  %-34s planted 2 (one dated 2000, one 2020, marker 2010); the check\n' "$label"
		printf '          answered total=%s this-run=%s earlier=%s. The leg above reads that as clean.\n' \
			"$total" "$wrote" "$kept"
	fi
	return 0
}
at_risk_predicate_provable

# The insulation, asked whether it does anything. Every count in the two legs above ends in `|| true`,
# which the last paragraph explains as the difference between "the suite dies mid-file" and "the count
# comes back 0". An `ok` on the insulated arm alone would prove nothing -- a `find` that simply
# succeeded would print the same 0 -- so this leg runs both arms, the same assignment with and without
# the `|| true`, against the same two absent paths: a box that is not there and a marker that was never
# created. The insulated arm has to return 0 and stay alive; the uninsulated one has to die. If the
# second arm ever comes back alive, the first arm's 0 has stopped meaning that `|| true` saved it, and
# the note printed by the failure is the same one this file prints everywhere else: the ok is the check
# working, not the check being blind.
the_insulation_is_load_bearing() {
	local label='a count survives a box it cannot open' w2 alive dead
	w2=$(mktemp -d "$work/insulation-XXXX")
	alive=$(bash -c 'set -euo pipefail
		x=$(find "$1" -name "*-quarantine-*.patch" -newer "$2" 2>/dev/null | wc -l | tr -d "[:space:]") || true
		printf "%s" "$x"' _ "$w2/no-such-box" "$w2/no-such-marker") || alive='(died)'
	dead=$(bash -c 'set -euo pipefail
		x=$(find "$1" -name "*-quarantine-*.patch" -newer "$2" 2>/dev/null | wc -l | tr -d "[:space:]")
		printf "%s" "$x"' _ "$w2/no-such-box" "$w2/no-such-marker") || dead='(died)'
	if [ "$alive" = 0 ] && [ "$dead" = '(died)' ]; then
		pass=$((pass + 1))
		printf '  ok    %-34s insulated: rc=0 and the count is 0, and the shell lives; the\n' "$label"
		printf '          same assignment without `|| true` dies right there (rc non-zero, nothing\n'
		printf '          printed), which is what makes the 0 above an answer and not a rescue.\n'
	else
		fail=$((fail + 1))
		printf '  FAIL  %-34s insulated=%s uninsulated=%s. Expected 0 and (died): if the bare\n' "$label" "$alive" "$dead"
		printf '          form survives too, the `|| true` above is decoration and the counts may be\n'
		printf '          silently 0 for a reason nobody is testing for.\n'
	fi
	rm -rf "$w2"
	return 0
}
the_insulation_is_load_bearing

# The leg the review of #250 asked for by name -- "whatever lands, please add the leg that composes the
# two calls" -- and the reason it exists: the streak legs above call `factory_tree_stop` twice with
# nothing between them, which is a shape the shipped pass never has. Between the two calls the shipped
# pass puts a TICK-START and a session, and the reset that used to sit there made `attempt=2`
# unreachable for every tick unit: five abandoned ticks in one tree counted `attempt=1` five times and
# halted nothing. This drives the order the pass actually runs -- quarantine, tick, verdict, next
# pass -- and never calls the increment itself.
streak_composed_with_a_tick() { # tree where -> the counter and the tick that follows it
	local tree=$1 where=$2 label bad="" rc=0 v=""
	label="a streak survives its tick ($where)"

	# Pass 1: the tree is dirty from a dead tick, so this pass quarantines it and starts a tick. The
	# tick then dies the same way the last one did -- leaving work in the tree -- and the pass ends by
	# asking the tree what it thinks.
	q_reset "$tree" selftest
	q_dirty "$tree"
	rc=0
	factory_tree_stop TICK-HALTED selftest "$tree" >/dev/null 2>&1 || rc=$?
	[ "$rc" = 0 ] || bad="the first pass stopped the unit (exit $rc) instead of quarantining it"
	printf 'the next tick died here too\n' >>"$tree/tracked.txt"
	factory_quarantine_after_tick selftest "$tree"
	[ "$(factory_ledger_get selftest quarantine-count)" = 1 ] ||
		bad="${bad:+$bad; }the tick that left the tree dirty cleared the streak to $(factory_ledger_get selftest quarantine-count); it has to stay at 1"
	q_journal_has 'TREE-AFTER-TICK selftest streak=armed' ||
		bad="${bad:+$bad; }the journal never says the streak stayed armed after the tick"

	# Pass 2: exactly the same opening -- the tree is dirty, nobody cleaned it, this is what the dead
	# tick left -- and this is the pass that is allowed to conclude that something keeps dying here.
	rc=0
	factory_tree_stop TICK-HALTED selftest "$tree" >/dev/null 2>&1 || rc=$?
	[ "$rc" = 4 ] || bad="${bad:+$bad; }the second pass left $rc, want 4: the streak cannot fire, so a tree that keeps killing ticks quarantines forever"
	[ -e "$FACTORY_HALT" ] || bad="${bad:+$bad; }the second accident did not halt the queue"
	q_journal_has 'TREE-STREAK' || bad="${bad:+$bad; }the journal does not name the streak as the reason"
	q_journal_has 'attempt=2' ||
		bad="${bad:+$bad; }the journal never counted attempt=2; the counter reads $(factory_ledger_get selftest quarantine-count)"

	# The control, and it is the same shape with one thing changed: the tick finishes and leaves the
	# tree clean. Without it every assertion above is satisfied by a function that always halts -- the
	# exact behaviour #248 was filed against.
	q_reset "$tree" selftest
	q_dirty "$tree"
	factory_tree_stop TICK-HALTED selftest "$tree" >/dev/null 2>&1 || true
	factory_quarantine_after_tick selftest "$tree"
	q_journal_has 'TREE-AFTER-TICK selftest streak=cleared' ||
		bad="${bad:+$bad; }a tick that left the tree clean did not clear the streak"
	q_dirty "$tree"
	rc=0
	factory_tree_stop TICK-HALTED selftest "$tree" >/dev/null 2>&1 || rc=$?
	[ "$rc" = 0 ] || bad="${bad:+$bad; }a clean tick followed by an honest accident halted anyway (exit $rc)"
	[ "$(factory_ledger_get selftest quarantine-count)" = 1 ] ||
		bad="${bad:+$bad; }the streak after the clean tick reads $(factory_ledger_get selftest quarantine-count), want 1"
	[ ! -e "$FACTORY_HALT" ] || bad="${bad:+$bad; }the control wrote a HALT file"

	# The third arm, planted rather than argued: a tree git will not read back is not evidence that the
	# tree is fine. The exclude-file loop at `$work/excludes-is-a-loop` is the shape #228 measured; on a
	# git that drops the patterns saying nothing it is simply DIRTY, which arms the streak for the other
	# reason, so the wording is only demanded when the verdict really is UNKNOWN.
	q_reset "$tree" selftest
	q_dirty "$tree"
	factory_tree_stop TICK-HALTED selftest "$tree" >/dev/null 2>&1 || true
	git -C "$tree" config core.excludesFile "$work/excludes-is-a-loop"
	v=$(verdict "$tree")
	factory_quarantine_after_tick selftest "$tree"
	[ "$(factory_ledger_get selftest quarantine-count)" = 1 ] ||
		bad="${bad:+$bad; }a tree reading ${v%%$'\t'*} cleared the streak to $(factory_ledger_get selftest quarantine-count); neither UNKNOWN nor DIRTY is a clean verdict"
	case "$v" in
	UNKNOWN*)
		q_journal_has 'streak=armed the tree could not be read' ||
			bad="${bad:+$bad; }the verdict was UNKNOWN and the journal did not say so -- it printed the dirty sentence instead, which is the wrong reason"
		;;
	esac
	git -C "$tree" config --unset core.excludesFile >/dev/null 2>&1 || true
	q_state "$tree"

	if [ -n "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s %s\n' "$label" "$bad"
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-34s quarantine, tick, verdict, next pass -> attempt=2 halts\n' "$label"
	return 0
}

for q_tree in "$repo" "$wt"; do
	q_where=primary
	[ "$q_tree" = "$wt" ] && q_where=worktree
	streak_composed_with_a_tick "$q_tree" "$q_where"
done

# A streak is only fair if it can end. The counter is what makes "halt on the second accident" safe,
# and a counter that only ever grows turns that safety rule into the #248 outage with a delay built in:
# any unit that has been quarantined once in its life halts the queue forever on its next accident,
# however long ago the first was and however many ticks ran in between. So the clearing of the streak
# is a shipped-text fact the same way the halt call sites are -- the behaviour is exercised by the legs
# above, the call site is what nothing else would notice.
streak_reset_after_tick_end() { # dir -> 0 when the tick's streak is cleared once the session has run
	awk '
		/TICK-START \$unit tree=/ { start = NR }
		/timeout --signal=TERM --kill-after=/ { ran = NR }
		/factory_quarantine_after_tick "\$unit" "\$tree"/ { if (ran && NR > ran && start) found = 1 }
		END { exit found ? 0 : 1 }
	' "$1/factory/factory.sh" 2>/dev/null
}

streak_reset_gated_on_the_tree() { # dir -> 0 when that clear sits behind the tree's own verdict
	awk '
		/^factory_quarantine_after_tick\(\)/ { f = 1 }
		f && /verdict=\$\(factory_tree_dirt "\$tree"\)/ { asked = NR }
		f && /factory_quarantine_reset "\$unit"/ { if (asked) found = 1 }
		END { exit found ? 0 : 1 }
	' "$1/factory/common.sh" 2>/dev/null
}

streak_reset_after_conflict() { # dir -> 0 when a resolved conflict clears the pr unit's streak
	awk '
		/CONFLICT-RESOLVED pr:\$pr/ { hit = NR }
		/factory_quarantine_reset "pr:\$pr"/ { if (hit && NR - hit <= 6) found = 1 }
		END { exit found ? 0 : 1 }
	' "$1/factory/factory.sh" 2>/dev/null
}

streak_reset_wired() {
	local label="a tick ends the streak" root=$HERE/.. bad=""
	streak_reset_after_tick_end "$root" ||
		bad="no factory_quarantine_after_tick of \"\$unit\" \"\$tree\" after the session has run"
	streak_reset_gated_on_the_tree "$root" ||
		bad="${bad:+$bad; }the clear is not behind factory_tree_dirt, so a tick clears the streak whatever its tree looks like"
	streak_reset_after_conflict "$root" || bad="${bad:+$bad; }no reset follows CONFLICT-RESOLVED"
	if [ -n "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s %s, so the quarantine counter only ever rises and the\n' "$label" "$bad"
		printf '          second honest accident of any unit halts every other unit forever.\n'
	else
		pass=$((pass + 1))
		printf '  ok    %-34s cleared after the tick on a clean tree, and at CONFLICT-RESOLVED\n' "$label"
	fi

	# The controls: the same scans over copies with the thing each scan looks for taken out. A scan that
	# still says yes is matching something else -- here, matching the function's own definition in
	# common.sh, which is exactly how the previous version of this leg could read "wired" at a call site
	# that made the streak unreachable.
	mkdir -p "$work/streak/factory"
	sed '/factory_quarantine_after_tick "\$unit" "\$tree"/d' "$root/factory/factory.sh" >"$work/streak/factory/factory.sh"
	cp "$root/factory/common.sh" "$work/streak/factory/common.sh"
	if streak_reset_after_tick_end "$work/streak"; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s still finds the call after deleting it, so the scan\n' "control: $label"
		printf '          matches something else and the ok above means nothing.\n'
	else
		printf '        control: deleting the call after the session is refused, so the ok above is\n'
		printf '        the scan working and not the scan matching a definition.\n'
	fi
	sed '/verdict=\$(factory_tree_dirt "\$tree")/d' "$root/factory/common.sh" >"$work/streak/factory/common.sh"
	if streak_reset_gated_on_the_tree "$work/streak"; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s still finds the gate after deleting the verdict, so the\n' "control: $label"
		printf '          clear is not pinned to the tree and an unreadable tree would end the streak.\n'
	else
		printf '        control: taking the verdict out of the function is refused too -- the clear is\n'
		printf '        pinned to the tree, not merely to a tick having been attempted.\n'
	fi
	return 0
}
streak_reset_wired

# ------------------------------------------------------------- the mirror's other half: the bytes
#
# `#250`'s mirror is rendered out of the stash, and for a binary path that rendering declines by
# default: `git stash show -p` prints `Binary files a/shot.png and b/shot.png differ` and no payload,
# so the mirror is 45 bytes of refusal, the `[ ! -s ]` guard passes, the tree is cleaned, and the
# journal advertises `mirror=<absolute path>` that restores nothing. The legs above count paths and so
# cannot see it -- they plant text. These plant bytes and ask the only question that matters: does the
# mirror, applied to a fresh clone of the base, put the original digests back?
q_mkbin_repo() { # where -> a throwaway repo holding one committed binary and one committed text file
	local d="$work/binrepo-$1"
	mkdir -p "$d"
	git init -q "$d"
	git -C "$d" config user.email selftest@invalid
	git -C "$d" config user.name selftest
	printf 'SHOT\210\000\001\002OLD\000\377\n' >"$d/shot.png"
	printf 'before\n' >"$d/notes.md"
	git -C "$d" add -A
	git -C "$d" commit -qm 'the committed capture'
	printf '%s\n' "$d"
}

q_stash_of() { # -> the stash the newest TREE-QUARANTINED journal line names, empty when it names none
	local line=""
	line=$(grep -F 'TREE-QUARANTINED' "$FACTORY_LOG/supervisor.log" 2>/dev/null | tail -n 1) || true
	printf '%s\n' "$line" | sed -n 's/.*stash=\([^ ]*\).*/\1/p'
}

mirror_carries_its_binary() { # where -> apply the shipped mirror to a fresh clone and compare digests
	local where=$1 label bad="" rc=0 qout="" d="" clone="" want_a="" want_b="" want_c="" m="" s=""
	label="the mirror carries its binary ($where)"
	d=$(q_mkbin_repo "$where")
	# The unit ledger, not just the tree: the streak legs above leave `selftest` armed at 1 on purpose,
	# and a quarantine that inherits that count halts on its first accident -- which is the new arm
	# working, but is not what this leg is asking about.
	factory_ledger_clear selftest quarantine-count
	factory_ledger_clear selftest halt-notice
	mkdir -p "$FACTORY_LOG"
	: >"$FACTORY_LOG/supervisor.log"
	printf 'SHOT\210\000\001\002NEW\000\377\376\n' >"$d/shot.png"
	printf 'SHOT\210\000\001\002ORPHAN\000\n' >"$d/orphan.png"
	printf 'after\n' >>"$d/notes.md"
	want_a=$(sha256sum "$d/shot.png" | cut -d' ' -f1)
	want_b=$(sha256sum "$d/orphan.png" | cut -d' ' -f1)
	want_c=$(sha256sum "$d/notes.md" | cut -d' ' -f1)

	rc=0
	qout=$(factory_tree_stop TICK-HALTED selftest "$d" 2>&1) || rc=$?
	[ "$rc" = 0 ] || bad="the unit was stopped (exit $rc): $(printf '%s' "$qout" | tr '\n' ' ' | cut -c1-300)"
	m=$(q_mirror_of)
	s=$(q_stash_of)
	[ -n "$m" ] && [ -s "$m" ] || bad="${bad:+$bad; }the journal named no usable mirror"

	# The mirror must contain the payload, and the text edit in the same tree has to survive with it:
	# `git apply` refuses a whole patch when one binary hunk has no header, so a mirror that carries
	# only the sentence takes the notes.md edit down with it.
	if [ -n "$m" ]; then
		grep -q 'GIT binary patch' "$m" ||
			bad="${bad:+$bad; }the mirror has no 'GIT binary patch' block -- it is the rendering that declines"
		grep -q 'Binary files' "$m" &&
			bad="${bad:+$bad; }the mirror carries the sentence git prints instead of the bytes"
		clone="$work/binclone-$where"
		if git clone -q "$d" "$clone" 2>/dev/null && (cd "$clone" && git apply "$m" 2>/dev/null); then
			[ "$(sha256sum "$clone/shot.png" | cut -d' ' -f1)" = "$want_a" ] ||
				bad="${bad:+$bad; }applying the mirror did not restore shot.png byte for byte"
			[ "$(sha256sum "$clone/orphan.png" | cut -d' ' -f1)" = "$want_b" ] ||
				bad="${bad:+$bad; }applying the mirror did not restore the untracked orphan.png"
			[ "$(sha256sum "$clone/notes.md" | cut -d' ' -f1)" = "$want_c" ] ||
				bad="${bad:+$bad; }applying the mirror did not restore the text edit"
		else
			bad="${bad:+$bad; }the mirror would not apply to a fresh clone of the base at all"
		fi
		# The opposite polarity, measured off the same stash rather than argued: the rendering without
		# `--binary` is non-empty AND unusable, which is the pair the size guard cannot tell apart. An
		# assertion that cannot say "refused" here is an assertion that never applied anything.
		if [ -n "$s" ] && git -C "$d" stash show -p --include-untracked "$s" >"$work/nobin-$where.patch" 2>/dev/null; then
			[ -s "$work/nobin-$where.patch" ] || bad="${bad:+$bad; }control: the flagless rendering was empty, so nothing here isolates the flag"
			clone="$work/binclone2-$where"
			if git clone -q "$d" "$clone" 2>/dev/null && (cd "$clone" && git apply "$work/nobin-$where.patch" 2>/dev/null); then
				bad="${bad:+$bad; }control: the flagless mirror applied, so the ok above does not rest on the flag"
			else
				printf '        control: the same stash rendered without --binary is a non-empty patch that\n'
				printf '        `git apply` refuses outright -- which is what the size guard could not see.\n'
			fi
		else
			bad="${bad:+$bad; }control: the stash could not be re-rendered, so the flagless arm never ran"
		fi
	fi

	if [ -n "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s %s\n' "$label" "$bad"
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-34s mirror applies to a fresh clone; all three digests match\n' "$label"
	return 0
}

mirror_carries_its_binary capture

# `refs/stash` is repo-global across linked worktrees, so the stack a rescue pops is not its own. The
# two legs below were the review's non-blocking findings, and both are the class the whole file is
# about: one restores somebody else's work into this unit's tree, the other leaves a file behind that
# tells the next reader about a process that no longer exists.
unstash_does_not_reach_past_its_own_entry() {
	local label="a rescue undoes its own stash" d="" sha1="" sha2="" out="" bad=""
	d=$(q_mkbin_repo unstash)
	printf 'mine\n' >>"$d/notes.md"
	git -C "$d" stash push -u -qm "supervisor quarantine selftest x"
	sha1=$(git -C "$d" rev-parse -q --verify refs/stash)
	printf 'theirs\n' >>"$d/notes.md"
	git -C "$d" stash push -u -qm "supervisor quarantine other-unit y"
	sha2=$(git -C "$d" rev-parse -q --verify refs/stash)
	[ -n "$sha1" ] && [ -n "$sha2" ] && [ "$sha1" != "$sha2" ] ||
		bad="the fixture did not produce two stash entries; it produced '${sha1:-none}' and '${sha2:-none}'"
	# Ours is under someone else's now. The pop must stop and name the entry, not take the top one.
	out=$(factory_tree_unstash "$d" "$sha1")
	[ -z "$(git -C "$d" status --porcelain)" ] ||
		bad="${bad:+$bad; }it popped the stack anyway, restoring the top entry into a tree that never held it"
	printf '%s' "$out" | grep -q 'was NOT put back' ||
		bad="${bad:+$bad; }it did not say the work was left in stash: ${out:-it said nothing}"
	printf '%s' "$out" | grep -qF -- "${sha1:0:9}" ||
		bad="${bad:+$bad; }the sentence does not name the sha the work is actually in"
	# The control: with its own entry on top, the same call must still do the rescue.
	out=$(factory_tree_unstash "$d" "$sha2")
	git -C "$d" status --porcelain | grep -q 'notes.md' ||
		bad="${bad:+$bad; }control: with its own entry on top it restored nothing (said: ${out:-nothing})"
	if [ -n "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s %s\n' "$label" "$bad"
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-34s refuses a stack it does not own, names stash %s\n' "$label" "${sha1:0:9}"
	return 0
}

rescue_leaves_no_temporary_behind() {
	local label="a rescue leaves no temp behind" d="" before=0 after=0 bad=""
	leak_probe() {
		find "$FACTORY_RUN" -name 'quarantine-error.*' 2>/dev/null | grep -c . || true
	}
	d=$(q_mkbin_repo leak)
	printf 'dirt from a dead tick\n' >>"$d/notes.md"
	factory_ledger_clear selftest quarantine-count
	before=$(leak_probe)
	factory_tree_stop TICK-HALTED selftest "$d" >/dev/null 2>&1 || true
	after=$(leak_probe)
	[ "$after" = "$before" ] ||
		bad="a successful quarantine left $((after - before)) quarantine-error file(s) in $FACTORY_RUN"
	# The control: one planted file has to move the number, or the ok above is a count of nothing.
	printf '' >"$FACTORY_RUN/quarantine-error.999999"
	after=$(leak_probe)
	[ "$after" != "$before" ] || bad="control: a planted quarantine-error.999999 was not counted"
	rm -f -- "$FACTORY_RUN/quarantine-error.999999"
	if [ -n "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s %s\n' "$label" "$bad"
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-34s the run directory is the same size after a rescue as before\n' "$label"
	return 0
}
unstash_does_not_reach_past_its_own_entry
rescue_leaves_no_temporary_behind

# ---------------------------------------------------------------- the notice a halt never sent
#
# `factory_maybe_notice` is the only thing in the harness that speaks to a human, and it returns early
# for any unit that is not `pr:` -- so the halt of 9 Nov, on `unclaimed:247`, said nothing to anyone
# for 15h57m. These legs drive the halt itself with a fixture `gh` on PATH and assert the four things
# that make a notice real: it goes on the unit's own issue, it carries the marker's sentence, it is
# throttled, and a failed post is journaled without stamping the throttle.
notice_legs() {
	local label unit=unclaimed:247 tree=$repo n=0 body=""
	q_reset "$tree" "$unit"
	q_dirty "$tree"
	: >"$q_rec"
	mkdir -p "$FACTORY_LOG"
	: >"$FACTORY_LOG/supervisor.log"

	# The halt path, not factory_tree_stop: the notice belongs to the halt, so a leg that reached it
	# through a refused quarantine would be asserting the refusal's plumbing instead.
	if ! (PATH="$q_gh:$PATH" Q_GH_REC=$q_rec Q_GH_RC=0 factory_tree_halt TICK-HALTED "$unit" "$tree") >"$work/notice.out" 2>&1; then
		: # exit 4 is the halt's answer and the point of the call
	fi
	n=$(grep -c . "$q_rec" 2>/dev/null || true)
	body=$(cat "$q_rec.body" 2>/dev/null || printf '')

	if [ "$n" != 1 ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s the halt posted %s comments, want exactly one (record: %s)\n' "$label" "${n:-0}" "$(tr '\n' '|' <"$q_rec" 2>/dev/null)"
		return 0
	fi
	case "$body" in
	*'halted the whole queue'*) : ;;
	*)
		fail=$((fail + 1))
		printf '  FAIL  %-34s the comment does not say the queue stopped: %s\n' "$label" "${body:0:120}"
		return 0 ;;
	esac
	case "$body" in
	*"$tree"*) : ;;
	*)
		fail=$((fail + 1))
		printf '  FAIL  %-34s the comment never names the tree, which is the one thing a reader\n' "$label"
		printf '          cannot guess: %s\n' "${body:0:120}"
		return 0 ;;
	esac
	q_journal_has "HALT-NOTICED $unit #$unit" || q_journal_has "HALT-NOTICED $unit" || {
		fail=$((fail + 1))
		printf '  FAIL  %-34s the journal never records the notice, so a pass that spoke to a human\n' "$label"
		printf '          leaves no trace of having done it.\n'
		return 0
	}
	[ -n "$(factory_ledger_get "$unit" halt-notice)" ] || {
		fail=$((fail + 1))
		printf '  FAIL  %-34s the throttle was not stamped, so the next pass posts the same comment.\n' "$label"
		return 0
	}
	pass=$((pass + 1))
	printf '  ok    %-34s one comment on #%s, carrying the marker sentence\n' "a halt comments on its unit" "${unit##*:}"

	# The throttle: the same halt again, an hour of silence away, must not speak twice.
	rm -f -- "$q_rec" "$q_rec.body"
	: >"$FACTORY_LOG/supervisor.log"
	(PATH="$q_gh:$PATH" Q_GH_REC=$q_rec Q_GH_RC=0 factory_tree_halt TICK-HALTED "$unit" "$tree") >"$work/notice2.out" 2>&1 || true
	if [ -s "$q_rec" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s it posted again inside the throttle window:\n' "control: the notice is throttled"
		while IFS= read -r line; do printf '            %s\n' "$line"; done <"$q_rec"
	else
		pass=$((pass + 1))
		printf '  ok    %-34s the second halt within the hour is silent\n' "control: the notice is throttled"
	fi

	# A post that fails has to be said AND left unstamped, or the harness believes it told someone.
	q_reset "$tree" "$unit"
	q_dirty "$tree"
	: >"$q_rec"
	rm -f -- "$q_rec.body"
	(PATH="$q_gh:$PATH" Q_GH_REC=$q_rec Q_GH_RC=1 factory_tree_halt TICK-HALTED "$unit" "$tree") >"$work/notice3.out" 2>&1 || true
	bad=""
	q_journal_has 'HALT-NOTICE-FAILED' ||
		bad="a refused post left no trace, so a halt nobody was told about reads in the journal like one everybody was"
	[ -z "$(factory_ledger_get "$unit" halt-notice)" ] ||
		bad="${bad:+$bad; }it stamped the throttle anyway, so the failure is never retried"
	if [ -n "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s %s\n' "a failed notice is said" "$bad"
	else
		pass=$((pass + 1))
		printf '  ok    %-34s journaled as a failure, throttle left clear for the next pass\n' "a failed notice is said"
	fi

	# A unit that names no issue -- the shape `expect_halt` above uses -- must not reach for gh at all.
	q_reset "$tree" "$unit"
	q_dirty "$tree"
	: >"$q_rec"
	rm -f -- "$q_rec.body"
	(PATH="$q_gh:$PATH" Q_GH_REC=$q_rec Q_GH_RC=0 factory_tree_halt TICK-HALTED selftest "$tree") >"$work/notice4.out" 2>&1 || true
	if [ -s "$q_rec" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s it commented on something for a unit that names no number:\n' "control: no number, no comment"
		while IFS= read -r line; do printf '            %s\n' "$line"; done <"$q_rec"
	else
		pass=$((pass + 1))
		printf '  ok    %-34s a unit with no issue number posts nothing\n' "control: no number, no comment"
	fi
	return 0
}
notice_legs

# --------------------------------------- the pass, run as a process, past the quarantine (#263)
#
# Every leg above drives a *function*. `expect_quarantine` calls `factory_tree_stop`, the mirror legs
# call `factory_tree_quarantine`, `stop_wiring` greps `factory.sh` for its call sites. None of them
# starts `factory/factory.sh`, and #263 measured what that costs: 54 mentions of `factory.sh` in this
# file at the tip this section was written against, every one of them a read of the text, and
#
#   asked of this file at 450a628, the tip this section was written against:
#     grep -c 'factory\.sh'                                                →  54
#     grep -cE '(bash|sh|exec|env)[[:space:]]+"?\$[A-Za-z_/]*factory\.sh'  →  0
#
# With the section in, the second one answers 1 once its own comments are out of the count
# (`grep -v '^[[:space:]]*#'`), and the one row is the `bash "$proc_dir/factory.sh"` inside `proc_run`
# below -- the thing this section adds. Asked with comments it answers 2, because the sentence you are
# reading quotes the pattern; that is why the honest form is written here rather than the bare grep.
# Every claim of the shape "factory.sh does X" was a claim about source text. The one property
# #248 was filed for is exactly such a claim -- a dirty tree is mirrored and the *pass carries on*,
# rather than writing `$FACTORY_HALT` and freezing 22 units for 15h57m -- and it lives in one line,
# `factory_tree_stop TICK-HALTED "$unit" "$tree" || return $?` above the `TICK-START` it lets happen.
#
# `--dry-run` cannot stand in for this, which is why the dry-run legs in the ask section do not
# already cover it: the dispatcher prints `DRY-RUN  tick unit=` and `continue`s before `factory_tick`
# is ever called, so a rehearsal never reaches `factory_tree_stop` at all. `factory_tree_stop`'s own
# `--dry-run` arm says the same thing from the other side -- it refuses to rehearse a rescue. The
# property is a property of the process, so it takes a process to test it.
#
# The fixture is the smallest queue that can express it: a throwaway trunk carrying the three shipped
# scripts, a clone of it (the supervisor fetches `origin` and cuts unit trees from `origin/main`, so
# it needs a real origin), issue 900 with a worktree planted dirty the way a killed tick leaves one,
# and issue 901 clean behind it. `gh` is a shim answering from payload files and refusing every write
# with 77; `opencode` is a stub that records which unit it was started for and exits 0, which is the
# state every post-tick arm was written for. Everything the run can reach is redirected into `$work`:
# `FACTORY_AT_RISK` included, because its default is the operator's real box holding the only copies
# of real work, and a leg that mirrored fixture dirt in there would be a leg that tidied the thing it
# exists to describe.
#
# Then the same fixture with one line of `common.sh` changed, twice over, because a green here is
# worth nothing unless the same fixture can be made red: the halt restored to its pre-#248 shape, and
# a quarantine that succeeds but reports failure. What those two measure is written where they run,
# and one of them contradicts #263's own account of the blast radius -- said there, not here, so the
# sentence is next to the number.
#
# What it costs, and on which machine it costs it. This is the first leg in the file to run the loop,
# so it pays for a fetch, three `git worktree add`, and a supervisor process per activation -- seven
# activations and three mutants. Two timed runs at each base, six at this head; every number below is
# a whole `bash selftest.sh`, on the host named beside it, because the one variable that decides
# whether a quoted cost means anything is the box it was taken on (H-0048's rule, and this paragraph is
# the case it was written for):
#
#   rodney-9950x, btrfs, ~/prj/at-risk present with 20 entries -- measured 10 Nov. The two base rows
#     were timed in a throwaway clone checked out at each sha (a base has to be timed in a tree that
#     holds a base's files, and this worktree holds none of them); the head row is this checkout:
#       450a628 (the section's base) . 338 legs, 337 passed 1 failed ... 10.67-10.71 s
#       adbe6e3 (origin/main) ........ 346 legs, 345 passed 1 failed ... 11.45-11.54 s
#       this head .................... 358 legs, all passed ........... 13.13-13.32 s, six runs
#     This head is twelve legs above main -- 358 minus 346, not 358 minus 345 -- and about +1.73 s
#     over it, so 0.14 s a leg. Eleven of those twelve are the section and the twelfth is `H-0087`'s
#     control, and the subtraction is over TOTALS because both bases carry a red leg: reading the two
#     `passed` columns against each other is how an eleven-leg section gets written down as twelve,
#     which is precisely what this paragraph said until `H-0089` corrected it. The spread between two
#     runs of the same tree is 0.2 s here, an eighth of the effect being claimed, so these are ranges
#     on purpose: a difference quoted to three decimals is a difference nobody measured. The one
#     failure at both bases is `the mirror box is the fixture's` reading the real 15:12Z rescue as this
#     file's crime -- the leg `H-0081` exists to fix, and it can only fail on a host that has the box.
#   the #264 reviewer's host, btrfs, no ~/prj/at-risk (one run each side, not two):
#       450a628 ......... 337 legs, none failed ... 28.8 s
#       that branch tip . 348 legs, none failed ... 34.9 s   (+6.1 s for the section's 11)
#       its base total sits one leg below this box's -- the same fs-dependent split #238 and #257
#       recorded on other legs, where the box leg has nothing to be red about -- and its branch tip
#       predates three commits of review fixes, so its head row is not this head's row.
#
# 0.14 s a leg here against 0.55 s there, the same fixture: four times the cost per leg, on top of a
# whole-suite run 2.7 times slower. Neither pair is the other's and neither is wrong, so the sentence
# that quotes one has to say which. Where the section
# came from: the tick before this one wrote it in 16 minutes and was stopped at its ceiling with the
# file still uncommitted, and `factory_tree_quarantine` mirrored it to
# `~/prj/at-risk/issue-263-quarantine-20261110T151215Z-2008e6100.patch` at 15:12:15Z; this is that file,
# `git apply`-ed onto the same blob it was cut from (`89687df`) plus the third control below and the
# numbers re-measured here. The mirror is the reason the paragraph says what it costs.

proc_work=$work/supervisor
proc_bin=$proc_work/bin
proc_fix=$proc_work/fixture
proc_rec=$proc_work/gh-calls
proc_up=$proc_work/up
mkdir -p "$proc_bin" "$proc_fix"

# One trunk, one commit, holding the three shipped scripts and a file a leg can dirty. The scripts are
# committed (not copied in afterwards) so the clone is clean the way a real checkout is: an untracked
# file here would put `dirty=1` in every `SUPERVISOR-AT` line and make the pass look like it was
# working in a tree somebody had left half-finished -- the very state the quarantine exists to refuse.
mkdir -p "$proc_up/factory"
git init -q "$proc_up"
git -C "$proc_up" config user.email fixture@invalid
git -C "$proc_up" config user.name fixture
git -C "$proc_up" checkout -qb main
cp "$HERE/common.sh" "$HERE/factory.sh" "$HERE/triage.sh" "$proc_up/factory/"
chmod +x "$proc_up/factory/factory.sh" "$proc_up/factory/triage.sh"
printf 'alpha\nbeta\ngamma\n' >"$proc_up/tracked.txt"
git -C "$proc_up" add -A
git -C "$proc_up" commit -qm 'fixture harness: the shipped scripts, and a file to leave uncommitted'

# The queue, as the shim serves it: two open issues, no pull request in any state, nothing merged. So
# `triage.sh` answers exactly two `unclaimed` rows, and the unit behind the quarantined one is a row
# the pass has to reach on its own -- not a row this file planted as an expectation.
printf '{"login":"supervisor"}\n' >"$proc_fix/user.json"
printf '{"data":{"repository":{"pullRequests":{"nodes":[]},"issues":{"nodes":[{"number":900,"stateReason":"NONE"},{"number":901,"stateReason":"NONE"}]}}}}\n' >"$proc_fix/graphql.json"
printf '[]\n' >"$proc_fix/pr-open.json"
printf '[]\n' >"$proc_fix/pr-bodies.json"
printf '[]\n' >"$proc_fix/pr-merged.json"
printf '[{"number":900,"title":"unit A: the tree a dead tick left dirty","updatedAt":"2026-11-10T00:00:00Z"},{"number":901,"title":"unit B: the clean unit behind it","updatedAt":"2026-11-10T00:00:01Z"}]\n' >"$proc_fix/issues.json"
# The tick's delivery read (`gh pr list --head <branch>`). It is not what any leg below asserts; it
# is answered so a tick ends the way a delivered tick ends, and the journal carries only the property
# under test rather than `TICK-UNDELIVERED` and a bundle write for a unit that never existed.
printf '[{"number":9001}]\n' >"$proc_fix/pr-head.json"

cat >"$proc_bin/gh" <<'PROCGH'
#!/usr/bin/env bash
# Fixture gh for the process legs. One recorded line per call, payloads from $PROC_FIXTURE, and two
# refusals: an unhandled read exits 9 (a shim that shrugs and prints `[]` turns a leg green without
# exercising anything -- CLAIMSHIM's rule), and a write to the queue exits 77 except the two verbs a
# supervisor uses to *talk* rather than to change state.
#
# `issue comment` is allowed and recorded rather than refused, and it is allowed only because this
# `gh` is a file: it cannot reach GitHub. The halt control below needs the notice to succeed, because
# the halt-notice legs already own what a failed notice does, and a control whose journal reads
# `HALT-NOTICE-FAILED` is measuring the shim instead of measuring the halt.
set -eu
here=${PROC_FIXTURE:?PROC_FIXTURE names the payload directory}
rec=${PROC_GH_REC:?PROC_GH_REC names the record file}
printf '%s\n' "$*" >>"$rec"

case "$1:$2" in
'pr:merge' | 'pr:review' | 'pr:edit' | 'pr:request-reviewers' | 'issue:close' | 'issue:reopen' | 'issue:edit')
	printf 'proc-shim: a leg tried to CHANGE THE QUEUE: %s\n' "$*" >&2
	exit 77 ;;
esac

jqprog=''
state=''
jsonf=''
head=''
args=("$@")
i=0
while [ "$i" -lt "${#args[@]}" ]; do
	case "${args[$i]}" in
	--jq) jqprog=${args[$((i + 1))]} ;;
	--state) state=${args[$((i + 1))]} ;;
	--json) jsonf=${args[$((i + 1))]} ;;
	--head) head=${args[$((i + 1))]} ;;
	esac
	i=$((i + 1))
done

emit() { # emit <payload> -> through the caller's own --jq, run by the real jq
	local file=$1
	if [ ! -f "$file" ]; then
		printf 'proc-shim: no payload for this call: %s\n' "$file" >&2
		exit 8
	fi
	if [ -n "$jqprog" ]; then jq -r "$jqprog" <"$file"; else cat "$file"; fi
}

case "$1 $2" in
'auth status') exit 0 ;;
'api user') emit "$here/user.json" ;;
'api graphql') emit "$here/graphql.json" ;;
'issue list') emit "$here/issues.json" ;;
'issue comment') exit 0 ;;
'pr list')
	# One read, three payloads, told apart by the flag that differs between them -- the same rule
	# CLAIMSHIM records and for the same reason: a shim that serves the body payload to the field
	# read makes a claim list silently empty.
	if [ -n "$head" ]; then emit "$here/pr-head.json"; exit 0; fi
	case "$state" in
	open)
		case "$jsonf" in
		*body*) emit "$here/pr-bodies.json" ;;
		*) emit "$here/pr-open.json" ;;
		esac ;;
	merged) emit "$here/pr-merged.json" ;;
	*)
		printf 'proc-shim: unhandled pr list --state %s\n' "$state" >&2
		exit 9 ;;
	esac ;;
*)
	printf 'proc-shim: unhandled call: %s\n' "$*" >&2
	exit 9 ;;
esac
PROCGH
chmod +x "$proc_bin/gh"

# The model runtime. `factory_preflight` asks for `$FACTORY_OPENCODE` on PATH and `factory_tick` execs
# it through `timeout … env PWD=…`, so this stub is the thing the tick's exit status comes from. It
# records the unit it was started for -- which is the second, independent proof that a tick started:
# the journal line says the supervisor decided to, this file says a session was actually opened.
cat >"$proc_bin/opencode" <<'PROCRT'
#!/usr/bin/env bash
set -eu
title=''
prev=''
for a in "$@"; do
	[ "$prev" = --title ] && title=$a
	prev=$a
done
tree=$(printf '%s' "$*" | sed -n 's/.*Work only in this directory: \([^ ]*\).*/\1/p')
printf 'tick title=%s tree=%s\n' "${title:-none}" "${tree:-none}" >>"${PROC_TICKS:?PROC_TICKS names the tick record}"
# A control needs a session that FAILS. The title carries the unit (`factory unclaimed:900`), the same
# string the journal's TICK-START names, so this refuses one named unit's session and not a coin flip.
if [ -n "${PROC_TICK_FAILS:-}" ] && printf '%s' "$title" | grep -qF -- "$PROC_TICK_FAILS"; then
	printf 'fixture tick: %s fails, which is what the dispatcher is being asked about\n' "$PROC_TICK_FAILS"
	exit 1
fi
exit 0
PROCRT
chmod +x "$proc_bin/opencode"

# proc_new <name> -> a fresh clone of the fixture trunk in its own box, with every FACTORY_* path
# beside it. A fixture per activation set, because the control has to be the same fixture as the
# shipped run with ONE line different -- not a different queue, a different tree, a different shim.
proc_repo=''
proc_dir=''
proc_trees=''
proc_risk=''
proc_log=''
proc_stash=''
proc_rundir=''
proc_halt=''
proc_ticks=''
proc_out=''
proc_journal=''
proc_rc=0
proc_new() {
	local name=$1
	proc_repo=$proc_work/$name
	proc_dir=$proc_repo/factory
	proc_trees=$proc_work/trees-$name
	proc_risk=$proc_work/at-risk-$name
	proc_log=$proc_work/log-$name
	proc_stash=$proc_work/ledger-$name
	proc_rundir=$proc_work/run-$name
	proc_halt=$proc_work/HALT-$name
	proc_ticks=$proc_work/ticks-$name.log
	git clone -q "$proc_up" "$proc_repo"
	git -C "$proc_repo" config user.email fixture@invalid
	git -C "$proc_repo" config user.name fixture
	mkdir -p "$proc_trees" "$proc_risk" "$proc_log" "$proc_stash" "$proc_rundir"
	: >"$proc_ticks"
	: >"$proc_rec"
}

# Unit A's tree, planted the way the outage was found: a tree that exists, on its own branch, with a
# tracked edit and an untracked file nobody else holds. It exists before the pass so `factory_worktree`
# returns it instead of cutting a clean one -- that is the state, and the ledger of the pass is that it
# must be mirrored and cleaned, not that this file gets to say it is clean.
proc_plant() { # -> the planted tree, and $proc_planted counts the paths git agrees are dirty
	local tree=$proc_trees/issue-900
	git -C "$proc_repo" worktree add -q -b issue-900 "$tree" origin/main
	printf 'the work a tick died with\n' >>"$tree/tracked.txt"
	printf 'untracked and nowhere else\n' >"$tree/scratch-notes.txt"
	proc_planted=$(git -C "$tree" status --porcelain 2>/dev/null | grep -c . || true)
}

# proc_run <tag> -> the shipped supervisor, as a process, over the fixture. `timeout` because a hung
# pass must arrive as a failed leg, not as a wedged suite: this is the first leg here that can hang,
# and the file has a trap and a tally that a wedged run never reaches.
proc_run() {
	local tag=$1
	: >"$proc_log/supervisor.log"
	# One tick record per activation. Both activations of a fixture append to one file otherwise, and a
	# leg that counts sessions for "the pass" would then be counting two passes.
	proc_ticks=$proc_work/ticks-$tag.log
	: >"$proc_ticks"
	proc_rc=0
	env PATH="$proc_bin:$PATH" \
		PROC_FIXTURE="$proc_fix" PROC_GH_REC="$proc_rec" PROC_TICKS="$proc_ticks" \
		PROC_TICK_FAILS="${proc_tick_fails:-}" \
		FACTORY_DIR="$proc_dir" FACTORY_ROOT="$proc_repo" FACTORY_HARNESS="$proc_repo" \
		FACTORY_REPO=example/repo FACTORY_REVIEWER=someoneelse \
		FACTORY_OPENCODE=opencode FACTORY_AGENT=factory \
		FACTORY_LOG="$proc_log" FACTORY_LEDGER="$proc_stash" FACTORY_RUN="$proc_rundir" \
		FACTORY_TREES="$proc_trees" FACTORY_AT_RISK="$proc_risk" FACTORY_HALT="$proc_halt" \
		FACTORY_BASE=main FACTORY_MAX_TICKS=4 FACTORY_BUDGET=120 \
		FACTORY_TICK_TIMEOUT=30 FACTORY_FETCH_TIMEOUT=10 \
		timeout --signal=TERM --kill-after=10 180 bash "$proc_dir/factory.sh" \
		>"$proc_work/$tag.out" 2>&1 || proc_rc=$?
	proc_journal=$proc_work/$tag.journal
	cp -- "$proc_log/supervisor.log" "$proc_journal" 2>/dev/null || : >"$proc_journal"
	proc_out=$proc_work/$tag.out
}

# proc_mutate <anchor> <line> -> the fixture's common.sh with one line after the shipped one. It
# refuses unless the anchor occurs exactly once, because a mutant planted on the wrong line is a
# control that measures nothing while printing the loudest sentence in the section.
proc_mutate() {
	local anchor=$1 line=$2 hits
	hits=$(awk -v a="$anchor" 'index($0, a) { c++ } END { print c + 0 }' "$HERE/common.sh")
	[ "$hits" = 1 ] || {
		printf '            the mutant anchor occurs %s time(s) in factory/common.sh, so it would\n' "$hits" >&2
		printf '            not be planted where it is described. Anchor:\n              %s\n' "$anchor" >&2
		return 1
	}
	awk -v line="$line" -v a="$anchor" '{ print } index($0, a) { print line }' \
		"$HERE/common.sh" >"$proc_dir/common.sh" || return 1
	grep -qF -- "$line" "$proc_dir/common.sh" || {
		printf '            the mutant line is not in the copy, so the control ran shipped code\n' >&2
		return 1
	}
	return 0
}

proc_journal_line() { # <journal> <needle> -> the first line carrying needle, empty when there is none
	awk -v n="$2" 'index($0, n) { print; exit }' "$1"
}

# The legs. Each one asks about the shipped run: fixture `shipped`, activation 1 with unit A dirty.
#
# A leg prints what it REQUIRES, not the name of the function that asked. The first version printed
# `$*`, so every one of these lines ended in `proc_did_run` and the sentences lived only in the pull
# request -- the wrong artifact for the thing a person reads at 3 a.m., which is this output. A name
# says a function ran; it does not say two units had to be ticked. The name is still one line above
# every sentence in the source, and a failing predicate prints its own detail under the FAIL line, so
# nothing became harder to find; what was impossible to find from the log was the promise.
proc_leg() { # proc_leg <label> <requires> command...
	local label=$1 requires=$2
	shift 2
	if [ "$#" -eq 0 ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-36s %s\n' "$label" 'no command given, so this leg asked nothing at all'
		return 0
	fi
	if "$@"; then
		pass=$((pass + 1))
		printf '  ok    %-36s %s\n' "$label" "$requires"
	else
		fail=$((fail + 1))
		printf '  FAIL  %-36s %s\n' "$label" "$requires"
	fi
	return 0
}

proc_new shipped
proc_plant
proc_p1=$proc_planted
proc_run shipped-1
proc_j1=$proc_journal
proc_t1=$proc_ticks
proc_rc1=$proc_rc
proc_out1=$proc_out
proc_run shipped-2

# The legs name the activation they read, because the second activation sees the tree the first one
# rescued: by then it is clean, and a leg that asks that pass whether a quarantine happened is asking
# the wrong pass. The first version of this section did exactly that and printed five failures over a
# run in which activation one had quarantined, mirrored and carried on correctly -- the journal of the
# activation *after* the accident says nothing about the accident, which is the same trap the section
# exists to keep the harness out of, arriving at the fixture instead of the production code.
proc_did_run() {
	local sessions
	# The fixture before the mutant: a tree nobody dirtied reaches the same `TREE-CLEAN` the failure
	# being controlled for reaches, and the journal cannot tell the two apart. `proc_plant` asks git how
	# many paths it left dirty, so this is a second opinion and not the leg restating its own wish.
	if [ "${proc_p1:-0}" != 2 ]; then
		printf '            the fixture tree has %s dirty path(s), not the two this section plants, so
' "${proc_p1:-nothing}" >&2
		printf '            every line below describes a clean tree instead of the state under test.
' >&2
		return 1
	fi
	sessions=$(grep -c '^tick ' "$proc_t1" 2>/dev/null || true)
	[ "$proc_rc1" = 0 ] || {
		printf '            the supervisor exited %s rather than 0 (124 is the timeout refusing to\n' "$proc_rc1" >&2
		printf '            let a hung pass wedge this suite). Its output:\n' >&2
		tail -n 12 "$proc_out1" | sed 's/^/              /' >&2
		return 1
	}
	[ -s "$proc_j1" ] || {
		printf '            no journal at all, which is the quietest failure in this harness and the\n' >&2
		printf '            one the tally above would have printed as an ok three legs down.\n' >&2
		return 1
	}
	case "$(proc_journal_line "$proc_j1" ' PASS ')" in
	*'units=2'*) : ;;
	*) printf '            the queue was not two units, so a TICK-START for 901 below would prove\n' >&2
		printf '            nothing about a queue this pass was actually walking:\n              %s\n' \
			"$(proc_journal_line "$proc_j1" ' PASS ')" >&2
		return 1 ;;
	esac
	[ "${sessions:-0}" = 2 ] || {
		printf '            %s session(s) were opened for a 2-unit queue, so the runtime stub and the\n' "${sessions:-0}" >&2
		printf '            journal disagree about whether a tick ran:\n' >&2
		sed 's/^/              /' "$proc_t1" >&2
		return 1
	}
	return 0
}
proc_leg 'the pass runs, over two units' 'two units dispatched, one session opened for each' proc_did_run

proc_quarantined_and_mirrored() {
	local line mirror
	line=$(proc_journal_line "$proc_j1" 'TREE-QUARANTINED unclaimed:900')
	[ -n "$line" ] || {
		printf '            unit A was never quarantined. That activation'"'"'s journal:\n' >&2
		sed 's/^/              /' "$proc_j1" | cut -c1-150 >&2
		return 1
	}
	case "$line" in
	*'attempt=1'*) : ;;
	*) printf '            the first quarantine of a unit that has never been quarantined is not\n' >&2
		printf '            attempt=1, so the streak arithmetic the halt depends on is wrong here:\n' >&2
		printf '              %s\n' "$line" >&2
		return 1 ;;
	esac
	# The journal's finding is the sentence an operator acts on, so it has to name what was planted --
	# the verdict's own words, not a count (#228's rule about asking git the stat-cache question).
	case "$line" in
	*'modified: tracked.txt'*'scratch-notes.txt'*) : ;;
	*) printf '            the finding does not name both planted paths, so it sends a reader looking\n' >&2
		printf '            for one file where the tree left two:\n              %s\n' "$line" >&2
		return 1 ;;
	esac
	mirror=${line##*mirror=}
	[ -s "$mirror" ] || {
		printf '            the journal names a mirror that is not there: %s\n' "${mirror:-nothing}" >&2
		printf '            A `mirror=` that restores nothing is the #250 failure, and it is the one\n' >&2
		printf '            thing that makes this pass allowed to clean the tree at all.\n' >&2
		return 1
	}
	# The rescue has to land in the box this leg owns. `FACTORY_AT_RISK`'s default is the operator's
	# real `~/prj/at-risk`, holding the only copies of work that is not this suite's to duplicate.
	case "$mirror" in
	"$proc_risk"/*) : ;;
	*) printf '            the mirror left the fixture: %s is not under %s\n' "$mirror" "$proc_risk" >&2
		return 1 ;;
	esac
	# And the mirror is the dirt, not a file git wrote and emptied: one edited path and one new path
	# render two hunks in `stash show -p --include-untracked`, measured. A mirror of one of the two is
	# a half rescue that journals as a whole one.
	if [ "$(grep -c '^diff --git' "$mirror")" != 2 ]; then
		printf '            the mirror holds %s file(s); the tree left two behind, so it is a half\n' \
			"$(grep -c '^diff --git' "$mirror")" >&2
		printf '            rescue: %s\n' "$mirror" >&2
		return 1
	fi
	return 0
}
proc_leg 'quarantines A, mirrors both paths' 'A at attempt=1, both paths named, the mirror on disk' proc_quarantined_and_mirrored

proc_no_halt() {
	[ ! -e "$proc_halt" ] || {
		printf '            the pass wrote %s: %s\n' "$proc_halt" "$(tr -d '\n' <"$proc_halt")" >&2
		printf '            That file is read before triage, so writing it is how one dirty unit\n' >&2
		printf '            becomes every other unit'"'"'s outage -- the 15h57m #248 was filed for.\n' >&2
		return 1
	}
	if [ -n "$(proc_journal_line "$proc_j1" 'TICK-HALTED ')" ]; then
		printf '            it halted without writing the marker, which halts nothing and reads as\n' >&2
		printf '            a halt in every later reading of the journal:\n              %s\n' \
			"$(proc_journal_line "$proc_j1" 'TICK-HALTED ')" >&2
		return 1
	fi
	return 0
}
proc_leg '…and leaves no halt file' 'no halt marker, and no TICK-HALTED line either' proc_no_halt

# The clause #262's acceptance leg has been carrying unpinned since it was written: "still starts a
# tick for unit B". Two observables, because the journal line alone is #263's text scan one level up --
# the supervisor's own word for its decision. The stub's record is that decision's effect.
proc_ticked_b() {
	local started
	started=$(proc_journal_line "$proc_j1" 'TICK-START unclaimed:901')
	[ -n "$started" ] || {
		printf '            no TICK-START for the clean unit behind the quarantined one. Journal:\n' >&2
		sed 's/^/              /' "$proc_j1" | cut -c1-150 >&2
		return 1
	}
	case "$(cat "$proc_t1")" in
	*issue-901*) : ;;
	*) printf '            the journal says a tick started and no session was opened for it:\n' >&2
		printf '              %s\n' "$started" >&2
		sed 's/^/              /' "$proc_t1" >&2
		return 1 ;;
	esac
	# TICK-START has to come after the quarantine, or "past the quarantine" is a story about two
	# unrelated lines -- a pass that quarantined A last week would print both.
	awk '
		index($0, "TREE-QUARANTINED unclaimed:900") { q = NR }
		index($0, "TICK-START unclaimed:901") { t = NR }
		END { exit (q && t && t > q) ? 0 : 1 }' "$proc_j1" || {
		printf '            the TICK-START is not after the TREE-QUARANTINED, so the pass did not go\n' >&2
		printf '            past anything: quarantine at line %s, tick at line %s.\n' \
			"$(awk 'index($0,"TREE-QUARANTINED unclaimed:900"){print NR; exit}' "$proc_j1")" \
			"$(awk 'index($0,"TICK-START unclaimed:901"){print NR; exit}' "$proc_j1")" >&2
		return 1
	}
	return 0
}
proc_leg '…then starts a tick for unit B' 'B ticked after the quarantine, with a session for it' proc_ticked_b

# The other half of "the pass continued", and the half #263 did not ask for. A first quarantine
# returns 0 -- the tree is mirrored and clean, so the unit whose tree was rescued gets its turn in the
# same pass. Make `factory_tree_stop` report that success as a failure, which is the mutation #263 names
# first, and this is the assertion that notices.
proc_quarantined_unit_got_its_turn() {
	awk '
		index($0, "TREE-QUARANTINED unclaimed:900") { q = NR }
		index($0, "TICK-START unclaimed:900") { t = NR }
		END { exit (q && t && t > q) ? 0 : 1 }' "$proc_j1" || {
		printf '            unit A was rescued and then left without a turn: the quarantine is at\n' >&2
		printf '            line %s and its TICK-START is at line %s (0 means never).\n' \
			"$(awk 'index($0,"TREE-QUARANTINED unclaimed:900"){print NR; exit}' "$proc_j1")" \
			"$(awk 'index($0,"TICK-START unclaimed:900"){print NR; exit}' "$proc_j1")" >&2
		return 1
	}
	return 0
}
proc_leg '…and gives the rescued unit its turn' 'A ticked again, after its own quarantine' proc_quarantined_unit_got_its_turn

# A quarantine that survives its own activation and then freezes the queue tomorrow is the same outage
# wearing a delay, so the second activation is part of the claim rather than a bonus. Its tree is clean
# by now -- activation one rescued it -- so what this pass proves is that nothing was left armed: no
# halt file, no streak, and both units ticked again.
proc_next_activation_runs() {
	if [ "$proc_rc" != 0 ]; then
		printf '            the second activation exited %s, not 0.\n' "$proc_rc" >&2
		return 1
	fi
	if [ -n "$(proc_journal_line "$proc_journal" 'HALTED ')" ]; then
		printf '            the second activation halted: %s\n' \
			"$(proc_journal_line "$proc_journal" 'HALTED ')" >&2
		return 1
	fi
	if [ -n "$(proc_journal_line "$proc_journal" 'TREE-STREAK ')" ]; then
		printf '            a clean second activation, with a tick between them, armed the streak:\n' >&2
		printf '              %s\n' "$(proc_journal_line "$proc_journal" 'TREE-STREAK ')" >&2
		return 1
	fi
	awk '/TICK-START unclaimed:900/ { a = 1 } /TICK-START unclaimed:901/ { b = 1 }
		END { exit (a && b) ? 0 : 1 }' "$proc_journal" || {
		printf '            the queue was alive but no tick ran for both units on the next activation.\n' >&2
		sed 's/^/              /' "$proc_journal" | cut -c1-150 >&2
		return 1
	}
	if [ "$(cat "$proc_stash/unclaimed:900.quarantine-count" 2>/dev/null)" != 0 ]; then
		printf '            the streak is still armed at %s after a tick that ended clean, so the\n' \
			"$(tr -d '\n' <"$proc_stash/unclaimed:900.quarantine-count" 2>/dev/null || printf 'nothing')" >&2
		printf '            next honest accident halts every unit for a counter nobody reset.\n' >&2
		return 1
	fi
	return 0
}
proc_leg 'the next activation still works' 'exits 0, both units ticked, streak cleared to 0' proc_next_activation_runs

# An observation this section did not go looking for, and prints rather than asserts, because a process
# leg reads journals and a text scan never would have noticed it. Unit B's tree is cut by
# `factory_worktree` minutes earlier and nothing ever writes to it -- and still the journal says of it:
#
#   TREE-CLEAN unclaimed:901 <tree> was found dirty and is clean now; nothing was quarantined
#
# `factory_tick` calls `factory_tree_stop` on every tree before every tick, `factory_tree_quarantine`
# answers 1 for a tree with nothing to stash, and that arm's sentence is written for the case where the
# verdict and the action disagreed. So the ordinary, healthy path prints a line whose words say a person
# or a dead tick had dirt in it and no longer does. Nothing here asserts the wording -- pinning it would
# make the fix harder to land, not easier -- but the harness's own rule is that a failure left unspoken
# reads as success, and this is the same shape wearing the opposite sign: a sentence that reports an
# event that did not happen, in the log an operator is told to read. Worth an issue; it is not this
# unit's change to make, and a leg that asserts it would be a leg that refuses the fix.
if [ -n "$(proc_journal_line "$proc_j1" 'TREE-CLEAN unclaimed:901')" ]; then
	printf '        note: unit B was never dirty, and the pass still journalled "was found dirty and is\n'
	printf '        clean now" for it -- `factory_tree_stop` runs before every tick, and a clean tree\n'
	printf '        takes the disagreement arm of the quarantine. Asserted nowhere here; said out loud\n'
	printf '        because a reader of this file should know the journal says it.\n'
fi

# ------------------------------------------------------------------ the two controls
#
# Control one: the halt as it was before #248. One line, inserted into the fixture's copy of
# `factory_tree_stop` ahead of the quarantine, so a dirty tree writes the marker and returns the halt's
# answer instead of mirroring and continuing. Everything else -- the same fixture layout, the same
# queue, the same shims -- is the run above.
proc_new halt-restored
proc_mutate 'qerr_file="$FACTORY_RUN/quarantine-error.$$"' \
	'	if [ -n "$dirt" ]; then factory_tree_halt "$verb" "$unit" "$tree" || return $?; fi # MUTANT-263A' ||
	proc_halt_control_planted=0
proc_plant
proc_ph=$proc_planted
proc_run halt-1
proc_jh=$proc_journal
proc_run halt-2

proc_halt_freezes() {
	if [ "${proc_ph:-0}" != 2 ]; then
		printf '            the halting control'"'"'s fixture tree has %s dirty path(s), not two, so it
' "${proc_ph:-nothing}" >&2
		printf '            halts for the wrong reason -- a tree that was never dirtied halts nothing.
' >&2
		return 1
	fi
	if [ "${proc_halt_control_planted:-1}" = 0 ]; then
		printf '            the mutant was never planted, so this leg ran shipped code and proves\n' >&2
		printf '            nothing about the halt.\n' >&2
		return 1
	fi
	# The halt is the first activation's act; the freeze is the second activation's consequence. Asked
	# out of one journal they are the same sentence, and the outage #248 was filed for is exactly the
	# difference between them: 15h57m of passes that refused to start anything.
	if [ -z "$(proc_journal_line "$proc_jh" 'TICK-HALTED ')" ]; then
		printf '            the halting control did not halt, so nothing below it can show what the\n' >&2
		printf '            shipped pass above is not doing. Its journal:\n' >&2
		sed 's/^/              /' "$proc_jh" | cut -c1-150 >&2
		return 1
	fi
	if [ ! -e "$proc_halt" ]; then
		printf '            it halted without leaving the marker, so the next activation would not\n' >&2
		printf '            have stopped -- which is the whole of why the marker, not the log line,\n' >&2
		printf '            is what freezes a queue.\n' >&2
		return 1
	fi
	[ "$proc_rc" = 8 ] || {
		printf '            the activation after the halt exited %s; 8 is the halt'"'"'s own code, the\n' "$proc_rc" >&2
		printf '            one the installed unit lists in SuccessExitStatus.\n' >&2
		return 1
	}
	if [ -z "$(proc_journal_line "$proc_journal" 'HALTED ')" ]; then
		printf '            the second activation did not report the halt:\n' >&2
		sed 's/^/              /' "$proc_journal" | cut -c1-150 >&2
		return 1
	fi
	awk '/TICK-START/ { found = 1 } END { exit found ? 1 : 0 }' "$proc_journal" || {
		printf '            and it started a tick anyway, so the halt is not the freeze this suite\n' >&2
		printf '            exists to refuse:\n' >&2
		sed -n 's/^/              /p' "$proc_journal" | cut -c1-150 >&2
		return 1
	}
	return 0
}
proc_leg 'control: the halt restored freezes it' 'exit 8, marker on disk, next run reports it, no tick' proc_halt_freezes

# What the halting pass itself did with unit B, printed rather than asserted, because it is the
# sentence #263 gets wrong and a reader should see the number next to the claim: the dispatcher
# survives a tick's non-zero return (`case $rc_tick` falls through to `started++ ; rc=4`), so the unit
# behind a halted one still got its turn *in that activation*. The outage is every activation AFTER it,
# which is what the leg above asserts. #263 blames "every unit behind the quarantined one" on the
# strength of one return code; the halt file, read before triage, is the mechanism.
if [ -n "$(proc_journal_line "$proc_jh" 'TICK-START unclaimed:901')" ]; then
	printf '        note: the halting pass still ticked unit B -- the freeze is the NEXT activation'"'"'s,\n'
	printf '        which started nothing at all (exit %s). The halt file, not the return code.\n' "$proc_rc"
fi

# Control two: the quarantine succeeds -- the mirror is written, the tree is cleaned -- and then
# reports failure, which is the mutation #263 names first. Same fixture, one inserted line after the
# shipped `TREE-QUARANTINED` log.
proc_new stop-returns-4
proc_mutate 'log_event "TREE-QUARANTINED $unit' \
	'		if [ "$n" -lt 2 ]; then return 4; fi # MUTANT-263B a rescued tree reported as a halt' ||
	proc_stop_control_planted=0
proc_plant
proc_ps=$proc_planted
proc_run stop-1


proc_costs_its_own_unit_its_turn() {
	if [ "${proc_ps:-0}" != 2 ]; then
		printf '            this control'"'"'s fixture tree has %s dirty path(s), not two. A clean tree
' "${proc_ps:-nothing}" >&2
		printf '            reaches `TREE-CLEAN` for the same reason the mutant fails to reach it, and
' >&2
		printf '            the journal below cannot tell those apart: one is the control, one is nothing.
' >&2
		return 1
	fi
	if [ "${proc_stop_control_planted:-1}" = 0 ]; then
		printf '            the mutant was never planted, so this leg ran shipped code.\n' >&2
		return 1
	fi
	if [ -z "$(proc_journal_line "$proc_journal" 'TREE-QUARANTINED unclaimed:900')" ]; then
		printf '            the mutant did not quarantine, so it is not the mutant described:\n' >&2
		sed 's/^/              /' "$proc_journal" | cut -c1-150 >&2
		return 1
	fi
	awk '/TICK-START unclaimed:900/ { found = 1 } END { exit found ? 1 : 0 }' "$proc_journal" || {
		printf '            a successful quarantine that returns non-zero still got its tick, so the\n' >&2
		printf '            leg above that requires one is not pinning the return code after all.\n' >&2
		return 1
	}
	return 0
}
proc_leg 'control: a false failure costs its turn' 'B ticked, A quarantined, A got no tick' proc_costs_its_own_unit_its_turn

# Its blast radius too, measured rather than repeated from the issue: with the halt file never written,
# the dispatcher's `case $rc_tick` absorbs the failure and unit B is reached. So this mutant costs one
# unit one turn -- real, and smaller than the outage #263 describes. Saying so here is what keeps the
# section's other numbers worth reading.
if [ -n "$(proc_journal_line "$proc_journal" 'TICK-START unclaimed:901')" ]; then
	printf '        note: that mutant stopped only its own unit -- the clean unit behind it still\n'
	printf '        started (exit %s). The dispatcher treats a failed tick as a spent tick, not a\n' "$proc_rc"
	printf '        stopped pass; only the halt file ends the queue, and only for later activations.\n'
fi

# ------------------------------------------------------------------ the third control, unasked for
#
# Both controls above end with the clean unit STILL TICKED, which contradicts the blast radius #263
# describes ("every unit behind the quarantined one"). Left there, the section's central leg — "then
# starts a tick for unit B" — would be a leg that can only say yes, which is the thing this whole file
# exists to refuse. So the question is which edit to the shipped code actually stops B, and the answer
# is one line below `factory_tree_stop`: the dispatcher's `factory_tick … || rc_tick=$?`. Under
# `set -e` the bare call ends the pass on the spot; factory.sh carries the scar ("Measured: a tick that
# exits 124 … ended the whole pass on the spot, so the units behind it were never reached and the
# supervisor exited 5 with eight units still queued"). Make unit A's session fail — the state the guard
# was written for — and run it twice, once shipped and once with the guard taken, so the one variable
# is the guard and not the failure.
proc_mutate_factory() { # <anchor> <line> -> the fixture's factory.sh with the anchor line replaced
	local anchor=$1 line=$2 hits
	hits=$(awk -v a="$anchor" 'index($0, a) { c++ } END { print c + 0 }' "$HERE/factory.sh")
	[ "$hits" = 1 ] || {
		printf '            the mutant anchor occurs %s time(s) in factory/factory.sh, so it would not\n' "$hits" >&2
		printf '            be planted where it is described. Anchor:\n              %s\n' "$anchor" >&2
		return 1
	}
	awk -v line="$line" -v a="$anchor" '{ if (index($0, a)) print line; else print }' \
		"$HERE/factory.sh" >"$proc_dir/factory.sh" || return 1
	grep -qF -- "$line" "$proc_dir/factory.sh" || {
		printf '            the mutant line is not in the copy, so the control ran shipped code\n' >&2
		return 1
	}
	return 0
}

# A failing session for unit A only. Everything else is the shipped fixture.
proc_new fail-shipped
proc_plant
proc_pf=$proc_planted
proc_tick_fails=unclaimed:900
proc_run fail-1
proc_jf=$proc_journal
proc_tick_fails=''

proc_failed_tick_alive() {
	[ "${proc_pf:-0}" = 2 ] || {
		printf '            the failing fixture tree has %s dirty path(s), not the two this section\n' "${proc_pf:-nothing}" >&2
		printf '            plants, so this is not the state the dispatcher was written for.\n' >&2
		return 1
	}
	[ -n "$(proc_journal_line "$proc_jf" 'TICK-FAILED unclaimed:900')" ] || {
		printf '            unit A'"'"'s session did not fail, so nothing below asks the question:\n' >&2
		sed 's/^/              /' "$proc_jf" | cut -c1-150 >&2
		return 1
	}
	[ -n "$(proc_journal_line "$proc_jf" 'TICK-START unclaimed:901')" ] || {
		printf '            a FAILED tick stopped the pass dead: the unit behind it never started,\n' >&2
		printf '            which is the outage #263 describes and the guard exists to refuse.\n' >&2
		sed 's/^/              /' "$proc_jf" | cut -c1-150 >&2
		return 1
	}
	return 0
}
proc_leg 'a failed tick spends its unit, not the pass' "A's session exits 1, and B still gets its TICK-START" proc_failed_tick_alive

proc_new unguarded-dispatch
proc_mutate_factory \
	'factory_tick "$bclass" "$bkey" "$bdetail" "$bbranch" "$bissue" "$bask" || rc_tick=$?' \
	'	factory_tick "$bclass" "$bkey" "$bdetail" "$bbranch" "$bissue" "$bask" # MUTANT-263C' ||
	proc_dispatch_control_planted=0
proc_plant
proc_pg=$proc_planted
proc_tick_fails=unclaimed:900
proc_run unguard-1
proc_jg=$proc_journal
proc_tick_fails=''

proc_guard_is_the_difference() {
	if [ "${proc_pg:-0}" != 2 ] || [ "${proc_dispatch_control_planted:-1}" = 0 ]; then
		printf '            the third control did not reach its state (tree %s dirty path(s), mutant\n' "${proc_pg:-nothing}" >&2
		printf '            planted %s), so its silence about unit B means nothing.\n' \
			"$([ "${proc_dispatch_control_planted:-1}" = 0 ] && printf 'no' || printf 'yes')" >&2
		return 1
	fi
	[ -n "$(proc_journal_line "$proc_jg" 'TICK-FAILED unclaimed:900')" ] || {
		printf '            the mutant did not fail unit A either, so it is not the same fixture:\n' >&2
		sed 's/^/              /' "$proc_jg" | cut -c1-150 >&2
		return 1
	}
	if [ -n "$(proc_journal_line "$proc_jg" 'TICK-START unclaimed:901')" ]; then
		printf '            and the guard-taken pass still reached unit B, so the leg above that needs\n' >&2
		printf '            the guard is pinning something else:\n' >&2
		sed 's/^/              /' "$proc_jg" | cut -c1-150 >&2
		return 1
	fi
	# The pair, in one place: same failing unit, same queue, the guard the only difference.
	[ "$proc_rc" != 0 ] || {
		printf '            the guard-taken pass exited 0; the shipped one exits %s, and 0 here means\n' "$proc_rc1" >&2
		printf '            set -e never saw the failure, so the two runs are not the pair described.\n' >&2
		return 1
	}
	return 0
}
proc_leg 'control: the guard off, and B is never reached' 'the same fixture without the guard: B is never reached' proc_guard_is_the_difference

printf '        note: the third control is the one #263 described. Its journal ends at A (exit %s),\n' "$proc_rc"
printf '        while the shipped dispatcher reaches B with the same failing session. So the property\n'
printf '        "the pass continued past the quarantine" is held by TWO lines -- tree_stop returning 0\n'
printf '        on a rescue, and the `|| rc_tick=$?` that keeps a spent tick from ending the pass.\n'

# ---------------------------------------------------------------- one verdict: is the loop doing anything
#
# `--status` used to print a halt boolean and a ledger, and finding out whether the queue was alive
# meant reading a journal timestamp and comparing it with `date` -- which is precisely the reading
# nobody did while 115 `HALTED` lines went by. These legs build the journal rather than hope for it,
# so each word is produced by a state this file controls, and each is produced by a different state:
# four verdicts, four journals, no two the same age.
q_verdict_tree=$repo
q_stamp() { date -u -d "-${1} minutes" +%Y-%m-%dT%H:%M:%SZ; }
q_journal() { # minutes_ago mark -> a journal whose newest line is `mark` at that age
	local age=$1 mark=$2
	mkdir -p "$FACTORY_LOG"
	{
		printf '%s PASS units=3 dry=0\n' "$(q_stamp "$((age + 30))")"
		printf '%s WAIT pr:215 review_required\n' "$(q_stamp "$((age + 10))")"
		printf '%s %s unclaimed:247 tree=%s log=x.log\n' "$(q_stamp "$age")" "$mark" "$q_verdict_tree"
	} >"$FACTORY_LOG/supervisor.log"
	touch -d "-$age minutes" "$FACTORY_LOG/supervisor.log"
}
q_expect_verdict() { # label word [TICK_MINUTES|marker|none]
	local label=$1 want=$2 plant=${3:-none} out=""
	rm -f -- "$FACTORY_HALT"
	case "$plant" in
	marker)
		printf 'a working tree was found with uncommitted work: %s\n' "$q_verdict_tree" >"$FACTORY_HALT"
		q_journal 90 TICK-START
		;;
	none)
		mkdir -p "$FACTORY_LOG"
		: >"$FACTORY_LOG/supervisor.log"
		touch -d '-3 hours' "$FACTORY_LOG/supervisor.log"
		;;
	*)
		q_journal "$plant" TICK-START
		# A pass that ran and started nothing: the tick line is old, the journal is young.
		if [ "$want" = IDLE ]; then
			printf '%s QUEUED 4 unit(s) remain\n' "$(q_stamp 1)" >>"$FACTORY_LOG/supervisor.log"
			touch "$FACTORY_LOG/supervisor.log"
		fi
		;;
	esac
	out=$(factory_verdict)
	case "$out" in
	"$want "*)
		pass=$((pass + 1))
		printf '  ok    %-34s %s\n' "$label" "$out"
		;;
	*)
		fail=$((fail + 1))
		printf '  FAIL  %-34s %s\n' "$label" "${out:-<the verdict printed nothing>}"
		printf '          want the word %s first, and the state planted for it was `%s`\n' "$want" "$plant"
		;;
	esac
	return 0
}

echo "factory/selftest.sh: the one-word verdict -- four states, four journals, planted"
q_expect_verdict "verdict: a tick just ran" WORKING 4
q_expect_verdict "verdict: a pass ran, nothing was ready" IDLE 90
q_expect_verdict "verdict: the journal is old" STALLED 90
q_expect_verdict "verdict: the marker wins over all of it" HALTED marker
q_expect_verdict "verdict: no journal at all" STALLED none

# HALT is the one verdict with a second duty: it has to carry the marker's own sentence, because that
# sentence is the whole of what an operator gets. `factory_halted()` is also the first branch of every
# other leg's state, so this is the leg that stops HALTED from meaning merely "some word".
q_expect_verdict "verdict: HALTED quotes the reason" HALTED marker
FACTORY_AT_RISK=$work/at-risk

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
	# ${2}, the PATH -- not ${3}. This is written here because the wrong index shipped for the whole
	# life of this fixture and it is the reason #242 had no leg: `api_get` calls
	# `gh api <path> --jq <filter>`, so ${3} is the literal `--jq`, every one of the three per-pull-
	# request REST reads fell through to `exit 9`, and `api_get` answered its own default -- `[]`
	# for reviews, `` for the head commit's date, `` for the outstanding requests. Three plant files
	# (`reviews.json`, `commit.json`, `reviewers.json`) sat in the fixture unreachable by any call,
	# and no leg could produce a pull request in any review state at all. Which is precisely why the
	# class that consumed a tick on a 21-byte body had never been exercised: not one leg could say
	# `address_review`. Measured at this commit by planting a CHANGES_REQUESTED review on an open
	# pull request whose head predates it: the shipped fixture answered `needs_reviewer` with the
	# detail `answered by the head commit ` -- the head commit missing from a sentence that prints
	# `$head_date`, which is the read failing rather than the state being empty.
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
		"$HERE/triage.sh" 2>"$claim_err" | tr '\037' '\t' >"$claim_rows_file" || true
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
claim_open() { # claim_open <json array of {number,title,body,claims,reviewDecision?}>
	printf '%s' "$1" >"$claim_fix/open-claims.json"
	# `reviewDecision` is a per-row pass-through now, not a constant. It was `""` for every planted
	# pull request because no leg before the ask ones needed a decision, and a constant here is a
	# door that cannot be opened: the CHANGES_REQUESTED arm -- the one that decides between a tick,
	# a wait and #242's new refusal -- is unreachable from a fixture that can only say "no decision".
	# Defaulting it to "" inside jq keeps all eighteen existing callers answering what they answered.
	jq -c '[.[] | {number, title, headRefName: "fix/planted", isDraft: false,
		reviewDecision: (.reviewDecision // ""), mergeable: "UNKNOWN", mergeStateStatus: "UNKNOWN",
		updatedAt: "2026-11-08T05:00:00Z", headRefOid: "0000000", statusCheckRollup: []}]' \
		"$claim_fix/open-claims.json" >"$claim_fix/pr-open.json"
	jq -c '[.[] | {number, body}]' "$claim_fix/open-claims.json" >"$claim_fix/pr-bodies.json"
	claim_graphql
}
# The review rows GitHub would answer with for `/pulls/<n>/reviews` -- one payload for every pull
# request, because the shim's `*/reviews*` arm has one file behind it and no leg so far has needed
# two pull requests reviewed differently. `.body` is the field #242 is about, and it is the field the
# projection used to throw away, so the legs that need it are the ones that prove it survives.
claim_reviews() { # claim_reviews <json array of review rows>
	printf '%s' "${1:-[]}" >"$claim_fix/reviews.json"
}
# `/pulls/<n>/requested_reviewers`, in the same one-file-behind-one-endpoint arrangement. Nothing
# wrote this file before #272: it was created empty and stayed empty, which is precisely the half the
# dismissal arm needs. "nobody holds a request" and "somebody already does" are the two polarities of
# that rule, and a fixture that can only say the first cannot tell a re-ask from a wait -- it would
# print ok for a classifier that asked twice a day at a reviewer who is already looking.
claim_reviewers() { # claim_reviewers <json payload for /requested_reviewers>
	local payload=${1:-}
	[ -n "$payload" ] || payload='{"users":[],"teams":[]}'
	printf '%s' "$payload" >"$claim_fix/reviewers.json"
}
claim_reset() {
	CLAIM_MERGED_FAIL=''
	CLAIM_GRAPHQL_FAIL=''
	CLAIM_MERGED_GARBAGE=''
	claim_window=100
	claim_merged '.'
	claim_issues '.'
	claim_reviews '[]'
	claim_reviewers
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

# ============================================ the reviewer's ask, and the review that has none (#242)
#
# The dispatch that consumed a worktree, a ceiling and a model on a review whose body never arrived
# was invisible to this suite for a reason that is itself a leg now: the fixture's REST dispatch read
# `${3:-}` -- the literal `--jq` in `gh api <path> --jq <filter>` -- so every per-pull-request read
# exited 9, `api_get` answered its own default, and `reviews.json`, `commit.json` and
# `reviewers.json` were three plants no call could reach. No leg could put a pull request into ANY
# review state, so the class `address_review` had never been produced by the suite in its whole life.
# Measured at this commit before the shim was corrected: a planted `CHANGES_REQUESTED` newer than the
# head commit answered `needs_reviewer` with the detail `answered by the head commit ` -- `$head_date`
# printed empty, which is a failed read wearing the shape of an empty state.
#
# So the legs below are in four groups, and the order is the argument:
#
#   A  the predicate, `factory_ask_verdict`, over this repository's own bodies -- the 21-byte one
#      that started it, the 1002-byte shortest real ask, and the 40-byte boundary at both sides.
#   B  `factory_newest_ask`: that the right review is chosen and that the record cannot be spliced.
#   C  the shipped `triage.sh` end to end, which doubles as the proof the fixture plants are now
#      reachable: the detail has to quote a head-commit date that only `commit.json` can supply.
#   D  the shipped `factory.sh --dry-run`, which is the only way to watch the ask cross the two
#      re-serialisations between triage's row and the tick's argument without starting a model.
claim_harness=$claim_work/harness
mkdir -p "$claim_harness"
cp "$HERE/triage.sh" "$HERE/common.sh" "$HERE/factory.sh" "$claim_harness/"

# The predicate, asked directly: common.sh is sourced at the top of this file, so these legs test the
# shipped function rather than a copy of its logic. `expect` is the whole answer, verdict and byte
# count together -- a leg that checked only the verdict could be green while the count it reports to
# the human was wrong, and the count IS the finding on the pull request.
ask_verdict_says() { # ask_verdict_says <expect> <body>
	local got
	got=$(factory_ask_verdict "$2")
	if [ "$got" != "$1" ]; then
		printf '            a body of %s bytes answers %s, not %s:\n' "${#2}" "${got:-nothing}" "$1" >&2
		printf '              %s\n' "$(printf '%s' "$2" | cut -c1-70)" >&2
		return 1
	fi
}
# The seventh column, exactly. This is the transport's own contract: what triage.sh read from GitHub
# is what the dispatcher hands a tick, character for character, so the leg compares whole strings
# rather than looking for a word in them.
claim_ask_is() { # claim_ask_is <key> <exact ask column>
	local got
	got=$(row_for "$1" | cut -f7)
	[ "$got" = "$2" ] || {
		printf '            %s ask column is %s bytes, not the %s planted:\n' "$1" "${#got}" "${#2}" >&2
		printf '              got: %s\n            want: %s\n' "$(printf '%s' "$got" | cut -c1-90)" \
			"$(printf '%s' "$2" | cut -c1-90)" >&2
		return 1
	}
}
claim_ask_absent() { # the row exists and its ask column is empty -- the class that has no ask
	row_for "$1" >/dev/null
	[ -z "$(row_for "$1" | cut -f7)" ] || {
		printf '            %s carries an ask of %s bytes; this class must not have one\n' \
			"$1" "$(row_for "$1" | cut -f7 | wc -c | tr -dc '0-9')" >&2
		return 1
	}
}

# A --------------------------------------------------------------- the floor, both sides of it
#
# Every body here is either the one #242 measured or one measured on this repository's own pull
# requests (8 Nov: `gh pr list --state all --limit 200 --json number,reviews`, 86 rows of state
# CHANGES_REQUESTED, lengths 21 / 1002 / … / 14863). The floor is a claim about those numbers, so the
# legs are too.
claim_reset
claim_leg 'the 21-byte #240 body is a reference, not an ask' \
	ask_verdict_says 'unreadable:reference 21' '@/tmp/pr240-review.md'
claim_leg '…and a longer path is still nothing to answer' \
	ask_verdict_says 'unreadable:reference 42' '/home/reviewer/notes/pr240-review-notes.md'
claim_leg 'an absent body is empty' \
	ask_verdict_says 'unreadable:empty 0' ''
claim_leg 'a body of whitespace is empty too' \
	ask_verdict_says 'unreadable:empty 0' "$(printf '%s' "$(printf '\n\t \n')" | tr '[:cntrl:]' ' ' | tr -s ' ' | sed -e 's/^ //' -e 's/ $//')"
# The boundary, from both sides, because `FACTORY_ASK_MIN_BYTES=40` means nothing unless 39 and 40
# answer differently. This is the pair that catches `<` becoming `<=` and the constant moving. Both
# bodies are GENERATED at the stated length rather than typed: the first draft of these two legs
# carried hand-typed runs of `a` and each was off by four characters, so the legs were asserting that
# 43 bytes answer `below-floor 39` -- a leg whose fixture does not match its own expectation is a leg
# that fails for the wrong reason forever, and the point of a boundary leg is that the number in it
# is the number in the body.
claim_leg '39 bytes is below the floor' ask_verdict_says 'unreadable:below-floor 39' \
	"$(printf 'a%.0s' $(seq 1 39))"
claim_leg '40 bytes is an ask' ask_verdict_says 'readable 40' "$(printf 'a%.0s' $(seq 1 40))"
# The cost of any floor, stated at the floor's own price: a 21-byte ask in ordinary English is
# refused here exactly as the 21-byte path is. It is a leg rather than a comment because the day
# somebody softens the predicate, this is the leg that goes quiet, and the sentence in common.sh that
# concedes the cost has to have a thing that enforces it existing.
claim_leg 'a 21-byte ask in English is refused (the disclosed cost)' \
	ask_verdict_says 'unreadable:below-floor 21' 'Rename it and ship it'
# The measured short side of real data: the shortest ask this repository has actually received is 1002
# bytes, so the floor retires no working review. A body of that size is not pasted here; its shape
# is what matters, so it is generated and only its verdict asserted.
claim_leg 'the shortest real ask measured here (1002 bytes) is readable' \
	ask_verdict_says 'readable 1002' "$(printf 'r%.0s' $(seq 1 1002))"
# Two flattening legs, because the byte count is taken AFTER control characters become spaces, and
# each pins a different half of that one line of `tr`/`sed`.
#
# Padding: 37 characters with four spaces each side is 45 bytes raw and 37 trimmed, so the leg wants
# the refusal the trim earns and the pass an untrimmed count would give.
claim_leg 'padding is not content: 45 raw bytes, 37 of ask' \
	ask_verdict_says 'unreadable:below-floor 37' \
	"    $(printf 'a%.0s' $(seq 1 37))    "
# A newline becomes a space, it is not deleted: 16 + 23 characters across one newline is 40 flattened
# and 39 if the control character were dropped, so this leg is the difference between `tr` and
# `delete` and lands exactly on the boundary.
claim_leg 'a newline costs a space, not nothing' \
	ask_verdict_says 'readable 40' 'aaaaaaaaaaaaaaaa
aaaaaaaaaaaaaaaaaaaaaaa'
# Bytes, not characters, because that is the unit the measured failure was reported in: 37 ASCII
# characters plus one three-byte em dash is 40 bytes and 38 characters. A character count would
# refuse an ask the predicate accepts, and refuse it silently on any review written in prose.
claim_leg 'the floor is counted in bytes, not characters' \
	ask_verdict_says 'readable 40' 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa—'

# Its control, planted outside the repository. The `reference` arm is supposed to be independent of
# the byte floor -- the 44-byte path above is over it and is still refused -- so a copy of common.sh
# with the floor at zero has to go on refusing it, while going soft on the 21-byte English ask.
mkdir -p "$claim_work/nofloor"
sed 's/^FACTORY_ASK_MIN_BYTES=40$/FACTORY_ASK_MIN_BYTES=0/' "$HERE/common.sh" >"$claim_work/nofloor/common.sh"
if [ "$(grep -c '^FACTORY_ASK_MIN_BYTES=0$' "$claim_work/nofloor/common.sh")" != "1" ]; then
	fail=$((fail + 1))
	printf '  FAIL  %-40s the control copy is not a copy with the floor at\n' "control: floor at zero"
	printf '          zero, so whatever the two lines below say, they say nothing.\n'
else
	soft=$(PATH="$claim_bin:$PATH" bash -c \
		". '$claim_work/nofloor/common.sh' >/dev/null 2>&1; factory_ask_verdict '/home/reviewer/notes/pr240-review-notes.md'")
	soft2=$(PATH="$claim_bin:$PATH" bash -c \
		". '$claim_work/nofloor/common.sh' >/dev/null 2>&1; factory_ask_verdict 'Rename it and ship it.'")
	printf '        control: with the floor at zero in a copy outside the tree, the 44-byte path is\n'
	printf '        still %s and the English ask becomes %s -- so the reference arm is its own\n' "$soft" "$soft2"
	printf '        rule and the ok above is the floor working, not the floor being the only rule.\n'
fi

# B ------------------------------------------------------------- which review, and a record that holds
ask_planted_newest() { # the record built from whatever reviews.json holds right now
	factory_newest_ask "$(cat "$claim_fix/reviews.json")" 2>/dev/null
}
ask_newest_is() { # ask_newest_is <expected review id>
	local got
	got=$(ask_planted_newest | cut -f1)
	[ "$got" = "$1" ] || {
		printf '            the record names review %s, not %s -- the read is: %s\n' \
			"${got:-nothing}" "$1" "$(ask_planted_newest | cut -c1-120)" >&2
		return 1
	}
}
ask_newest_shape_is() { # every field of the record for the 4002 row planted below
	local rec when user verdict
	rec=$(ask_planted_newest)
	when=$(printf '%s' "$rec" | cut -f2)
	user=$(printf '%s' "$rec" | cut -f3)
	verdict=$(printf '%s' "$rec" | cut -f4)
	if [ "$when" != "2026-11-08T18:19:17Z" ] || [ "$user" != "hermesvicowa" ] ||
		[ "$verdict" != "unreadable:reference 21" ]; then
		printf '            the record is %s | %s | %s, not the submitter, instant and\n' "$user" "$when" "$verdict" >&2
		printf '            verdict the planted review carries.\n' >&2
		return 1
	fi
}
ask_newest_refuses() { # no change request, and an unreadable payload, both mean "no ask was read"
	local out rc
	rc=0
	out=$(factory_newest_ask '[]' 2>/dev/null) || rc=$?
	if [ "$rc" = 0 ] || [ -n "$out" ]; then
		printf '            an empty review list answered rc=%s [%s]; it has to be distinguishable\n' "$rc" "$out" >&2
		printf '            from a review whose body is empty, or the arms read silence as an ask.\n' >&2
		return 1
	fi
	rc=0
	out=$(factory_newest_ask 'this is not json' 2>/dev/null) || rc=$?
	if [ "$rc" = 0 ]; then
		printf '            a payload that is not the array GitHub sends answered rc=0 [%s].\n' "$out" >&2
		return 1
	fi
}
claim_reviews "$(jq -cn '[
	{"id":4001,"user":{"login":"hermesvicowa"},"state":"COMMENTED","submitted_at":"2026-11-08T21:00:00Z","body":"a comment, later than both reviews"},
	{"id":4002,"user":{"login":"hermesvicowa"},"state":"CHANGES_REQUESTED","submitted_at":"2026-11-08T18:19:17Z","body":"@/tmp/pr240-review.md"},
	{"id":4003,"user":{"login":"someone-else"},"state":"CHANGES_REQUESTED","submitted_at":"2026-11-07T10:00:00Z","body":"an older change request, with far more words in it than the newest one has"}
]')"
claim_leg 'the newest change request is the subject, not the newest review' \
	ask_newest_is 4002
claim_leg '…with its submitter, its instant and its verdict' ask_newest_shape_is
claim_reviews '[]'
claim_leg 'no change request is not the same answer as an empty one' ask_newest_refuses
claim_reset

# C ------------------------------------------------------------ the row, through the shipped classifier
#
# One planted pull request, `CHANGES_REQUESTED`, reviewed at 18:19:17Z against a head commit the
# fixture stamps at 00:00:00Z -- the shape #240 was in when the tick was dispatched at it. The body
# is the constant across the legs that follow; only the reviews change.
ASK_PLANTED_BODY='Split the third commit in two: the gate change and the fixture change retire different mutants. Then name the mutant you retired while doing it.'
ask_pr240() { # ask_pr240 <review-row-json> -> one open PR, one review, one unrelated issue
	claim_reset
	# The live issue set is inherited from the legs above, and by this point in the file it holds
	# seven unclaimed issues. A dry run over that set spends all three of FACTORY_MAX_TICKS on them
	# and DEFERS everything else -- which is exactly the interference that made the "no tick was
	# started" claim insensitive: with the supervisor mutated to dispatch a tick for
	# `review_unreadable` anyway, the tick was deferred by the budget and the leg still read green,
	# measuring the cap instead of the arm (measured 8 Nov). Two units here: the pull request, and
	# #232 as a row that waits with no ask.
	claim_issues '[.[] | select(.number == 232)]'
	claim_open '[{"number":240,"title":"An address_review dispatch carries no ask","body":"no claim here","claims":[],"reviewDecision":"CHANGES_REQUESTED"}]'
	claim_reviews "$1"
}
ask_iso() { printf '%s' "$1" | jq -Rs '{id:4001,user:{login:"hermesvicowa"},state:"CHANGES_REQUESTED",submitted_at:"2026-11-08T18:19:17Z",body:.}' | jq -c '[.]'; }

ask_pr240 "$(ask_iso "$ASK_PLANTED_BODY")"
claim_rows >/dev/null
claim_leg 'a real ask still gets a tick' claim_class_is 240 address_review
claim_leg '…and the tick is sent WITH the ask, byte for byte' claim_ask_is 240 "$ASK_PLANTED_BODY"
claim_leg '…and the journal detail names the review it read' claim_detail_has 240 'review 4001'
claim_leg '…and who filed it' claim_detail_has 240 'hermesvicowa'
claim_leg '…and how many bytes arrived' claim_detail_has 240 '144 bytes'
claim_leg '…while the wait arms carry no ask at all' claim_ask_is 232 ''

ask_pr240 "$(ask_iso '@/tmp/pr240-review.md')"
claim_rows >/dev/null
claim_leg '#242: 21 bytes of file reference is refused, not dispatched' claim_class_is 240 review_unreadable
claim_leg '…and the refusal carries no ask' claim_ask_absent 240
claim_leg '…quoting the body that arrived, because the symptom is not the diagnosis' \
	claim_detail_has 240 '@/tmp/pr240-review.md'
claim_leg '…saying no tick was started' claim_detail_has 240 'no tick was started'
claim_leg '…and naming the one command that puts it back in the queue' \
	claim_detail_has 240 'gh pr review 240 --request-changes --body-file'
claim_leg '…naming who filed it and when, so a person can find the review' \
	claim_detail_has 240 'hermesvicowa at 2026-11-08T18:19:17Z'
# The REASON, not only the size. A mutation that drops the `reference` arm leaves the row refusing
# for `below-floor` instead -- the same class, the same byte count, the same command, and a human
# told "your review is too short" when the actual fault is that it was never uploaded. That is the
# difference between a useful refusal and one that sends the reviewer off to write more words.
claim_leg '…and the reason it refused, not just the size' claim_detail_has 240 'which is reference'

ask_pr240 "$(ask_iso '')"
claim_rows >/dev/null
claim_leg 'an empty body is refused the same way' claim_class_is 240 review_unreadable
claim_leg '…and says zero bytes rather than an empty quote' claim_detail_has 240 '0 bytes'

# The polarity that says the new class is about a live dispatch and not about any unreadable review
# ever filed. Same pull request, same 21 bytes, same reviewer: the review is 25 hours older than the
# head commit, so it has been answered and the head is what is waiting. This is also the leg that
# proves the fixture is being read at all -- the detail has to interpolate a date that only
# `commit.json` can supply, and it could not while the shim was dispatching on `--jq`.
ask_pr240 "$(jq -cn '[{id:4001,user:{login:"hermesvicowa"},state:"CHANGES_REQUESTED",
	submitted_at:"2026-11-06T23:00:00Z",body:"@/tmp/pr240-review.md"}]')"
claim_rows >/dev/null
claim_leg 'an ANSWERED unreadable review is not the new class' claim_class_is 240 needs_reviewer
claim_leg '…and the head commit it was answered by is in the detail' \
	claim_detail_has 240 'answered by the head commit 2026-11-08T00:00:00Z'

# The bound. `emit` caps nothing and the journal prints one line per unit, so an unbounded ask would
# put a 14863-byte review into a log line and a model prompt without either file saying so. The body
# is 14863 bytes because that is the longest ask this repository has received; what the row must carry
# is the cut, the total, and the command that reads the rest.
ask_long() { # the row's ask column, bounded, for a 14863-byte review
	local got
	got=$(row_for 240 | cut -f7)
	if [ "${#got}" -gt $((FACTORY_ASK_MAX_BYTES + 200)) ]; then
		printf '            the ask column is %s bytes, past the bound plus its marker\n' "${#got}" >&2
		return 1
	fi
	case "$got" in
	*"[… cut at ${FACTORY_ASK_MAX_BYTES} of 14863 bytes"*'gh pr view 240 --json reviews]'*) return 0 ;;
	esac
	printf '            a 14863-byte body arrived as %s bytes ending:\n              …%s\n' \
		"${#got}" "$(printf '%s' "$got" | tail -c 120)" >&2
	return 1
}
ask_pr240 "$(ask_iso "$(printf 'x%.0s' $(seq 1 14863) | sed 's/xxx/xxx /g' | cut -c1-14863)")"
claim_rows >/dev/null
claim_leg 'a 14863-byte ask still gets its tick' claim_class_is 240 address_review
claim_leg '…arriving bounded, and saying so with the total and the read' ask_long

# The transport, at its worst moment: a body carrying a newline, a tab and a literal U+001F -- the
# delimiter the row itself is made of. The flattening in common.sh is the only thing between that
# review and a spliced column, so the leg asks for the whole row: one row for the pull request, the
# class where the class is, and the ask where the ask is.
ask_splice() {
	local n
	n=$(awk -F'\t' -v k=240 '$2 == k' "$claim_rows_file" | grep -c .)
	[ "$n" = "1" ] || {
		printf '            the pass printed %s rows for key 240, so a delimiter arrived in the body\n' "$n" >&2
		return 1
	}
	claim_class_is 240 address_review || return 1
	# Note the space before the comma: the U+001F in the planted body became a space, which is the
	# whole flattening contract, and a leg that expected the delimiter back would be asserting the
	# bug rather than the fix.
	claim_ask_is 240 'a newline a tab and the row delimiter itself , in one review body'
}
ask_pr240 "$(jq -cn '[{id:4001,user:{login:"hermesvicowa"},state:"CHANGES_REQUESTED",
	submitted_at:"2026-11-08T18:19:17Z",
	body:"a newline\na tab\tand the row delimiter itself\u001f, in one review body"}]')"
claim_rows >/dev/null
claim_leg 'a body holding the row delimiter cannot splice the row' ask_splice
claim_reset

# D ------------------------------------------------ the supervisor, in a dry run, ask in hand
#
# The one question the rows above cannot answer is whether the column survives the two re-serialisations
# between triage.sh's row and the tick's argument -- `workable`, then `ranked`, then `factory_tick`'s
# sixth parameter. `--dry-run` prints what a tick would be told, so the transport can be watched end
# to end without starting a model, and the shipped supervisor is the thing under test, from a copy of
# the three scripts it needs beside each other.
ask_dispatch() { # run the shipped supervisor, dry, over whatever is planted
	mkdir -p "$claim_work/dtrees"
	PATH="$claim_bin:$PATH" CLAIM_FIXTURE="$claim_fix" \
		FACTORY_DIR="$claim_harness" FACTORY_ROOT="$claim_repo" \
		FACTORY_REPO=example/repo FACTORY_REVIEWER=someoneelse \
		FACTORY_OPENCODE=opencode \
		FACTORY_LOG="$claim_work/dlog" FACTORY_LEDGER="$claim_work/dledger" \
		FACTORY_RUN="$claim_work/drun" FACTORY_TREES="$claim_work/dtrees" \
		"$claim_harness/factory.sh" --dry-run >"$claim_work/dry.out" 2>&1 || true
	cat "$claim_work/dry.out"
}
ask_journal() { cat "$claim_work/dlog/supervisor.log" 2>/dev/null; }
ask_dry_carries() { # the dry-run dispatch, followed by the ask on its own line
	case "$(cat "$claim_work/dry.out")" in
	*"tick unit=address_review:240"*) : ;;
	*) printf '            no tick was dispatched for 240 at all:\n' >&2
		sed 's/^/              /' "$claim_work/dry.out" >&2; return 1 ;;
	esac
	case "$(cat "$claim_work/dry.out")" in
	*"ask: $ASK_PLANTED_BODY"*) return 0 ;;
	*) printf '            the tick was dispatched without the ask after it. The dry run printed:\n' >&2
		sed 's/^/              /' "$claim_work/dry.out" >&2; return 1 ;;
	esac
}
ask_pr240 "$(ask_iso "$ASK_PLANTED_BODY")"
ask_dispatch >/dev/null
claim_leg 'the ask crosses the dispatcher into the tick' ask_dry_carries

# What the dry run cannot reach is the last step: `factory_tick` putting the column INTO the
# instruction. No leg can start a real tick -- the fixture runtime refuses on purpose and there is no
# origin to open a worktree from -- so this is a scan of the shipped arm, in the shape this file
# already uses for the `delivered` arm, with a control outside the tree. It says something narrower
# than the legs above and is labelled for it: the arm interpolates the ask, between markers that name
# it as quoted material, and the branch for a missing ask ends in `needs-human` instead of a guess.
ask_prompt_assembles_it() { # ask_prompt_assembles_it [factory.sh to read]
	local arm src=${1:-$HERE/factory.sh}
	arm=$(awk '/^\taddress_review\)/,/^\t\t;;/' "$src")
	[ -n "$arm" ] || {
		printf '            factory_tick has no address_review arm to read\n' >&2
		return 1
	}
	case "$arm" in
	*'$ask'*) : ;;
	*) printf '            the arm builds its prompt without the ask column at all\n' >&2; return 1 ;;
	esac
	case "$arm" in
	*'BEGIN REVIEWER ASK'*) : ;;
	*) printf '            the ask reaches the prompt unmarked: reviewer-controlled text arriving\n' >&2
		printf '            inside a model'"'"'s instructions with nothing saying it is quoted\n' >&2
		return 1 ;;
	esac
	case "$arm" in
	*'not instructions to you'*) : ;;
	*) printf '            the markers are there but the prompt never says the ask is not an\n' >&2
		printf '            instruction to the tick, which is the half that bounds its authority\n' >&2
		return 1 ;;
	esac
	case "$arm" in
	*needs-human*) return 0 ;;
	esac
	printf '            the empty-ask branch does not end in needs-human and a stop, so the one\n' >&2
	printf '            move that invents an ask is still open to it\n' >&2
	return 1
}
claim_leg 'the tick assembles the ask into its instruction' ask_prompt_assembles_it
# Control, planted outside the repository: the same scan over a copy whose arm has the interpolation
# and the marker taken out. Three lines of a two-hundred-line function, with everything else in the
# file unchanged -- which is exactly why a scan without a control would be worth nothing here.
mkdir -p "$claim_work/noprompt"
awk '/^\taddress_review\)/,/^\t\t;;/ {
		if ($0 ~ /\$ask/ || $0 ~ /BEGIN REVIEWER ASK/ || $0 ~ /not instructions to you/) next
		print; next
	}
	1' "$HERE/factory.sh" >"$claim_work/noprompt/factory.sh"
if [ "$(grep -c 'BEGIN REVIEWER ASK' "$claim_work/noprompt/factory.sh")" -ge \
	"$(grep -c 'BEGIN REVIEWER ASK' "$HERE/factory.sh")" ]; then
	fail=$((fail + 1))
	printf '  FAIL  %-40s the control copy still carries the marker, so it is\n' "control: the copy is a copy"
	printf '          not a copy with the ask removed and the line below proves nothing.\n'
elif ask_prompt_assembles_it "$claim_work/noprompt/factory.sh" 2>/dev/null; then
	fail=$((fail + 1))
	printf '  FAIL  %-40s the scan passes on a copy with the ask removed, so\n' "control: ask into the prompt"
	printf '          the ok above is the scan being blind.\n'
else
	printf '        control: a copy outside the tree with the interpolation and the marker deleted is\n'
	printf '        refused, so the ok above is the scan working and not the scan being blind.\n'
fi

ask_dry_starts_nothing() {
	# `tick unit=` is the only line the supervisor prints that means a tick started -- factory.sh:859,
	# `DRY-RUN  tick unit=%s:%s detail=%s`. The pattern is deliberately that exact text rather than
	# `*unit=review_unreadable*`: shellcheck caught the looser form swallowing a second pattern that
	# said the same thing (SC2221/SC2222), and a broad pattern here would also have to be re-read every
	# time a new log line learns the word. Nothing the refusal arm writes through `log_event` carries
	# `unit=` -- its line is `ASK-UNREADABLE pr:240 …` -- so this arm can only be tripped by a tick.
	case "$(cat "$claim_work/dry.out")$(ask_journal)" in
	*'tick unit=review_unreadable'*)
		printf '            a tick was started for the unreadable review:\n' >&2
		sed 's/^/              /' "$claim_work/dry.out" >&2
		return 1 ;;
	esac
	# The comment is written through run_action, which is also what makes a dry run show it: the
	# `DRY-RUN  gh pr comment 240 --body …` line is the proof the supervisor would have spoken to
	# the human, and the journal line only proves it spoke to itself.
	if grep -qF -- 'gh pr comment 240' "$claim_work/dry.out"; then
		case "$(ask_journal)" in
		*'ASK-UNREADABLE pr:240'*) return 0 ;;
		esac
		printf '            it tried to comment but never logged the finding, so a pass that declined\n' >&2
		printf '            is not auditable: the journal is the only record the queue keeps.\n' >&2
		return 1
	fi
	printf '            the dry run shows no comment being written for 240 at all, so the decline is\n' >&2
	printf '            silent: a queue that declines quietly looks exactly like a queue with\n' >&2
	printf '            nothing in it. The dry run printed:\n' >&2
	sed 's/^/              /' "$claim_work/dry.out" | grep -v '^[[:space:]]*$' >&2
	return 1
}
ask_pr240 "$(ask_iso '@/tmp/pr240-review.md')"
ask_dispatch >/dev/null
claim_leg 'a review_unreadable row starts no tick and is still reported' ask_dry_starts_nothing
claim_reset

# ================================= the verdict a push voided, and the ask that never fired (#272)
#
# The classifier used to count review ROWS to decide whether a pull request needed a reviewer. A push
# into a reviewed pull request voids its approvals without deleting them -- this repository does not
# carry a stale approval across a new head -- so the count said "reviewed" about a pull request whose
# only verdict was `DISMISSED`, and the class that asks came out as the class that waits.
#
# This is not a hypothesis about GitHub. #269 was measured across one fast-forward, unforced push
# (`c149fe9..ec74cf5`) on 10 Nov: `reviewDecision` `APPROVED` -> `REVIEW_REQUIRED`, the single row
# `hermesvicowa APPROVED` -> `hermesvicowa DISMISSED` at the same `c149fe9`,
# `/requested_reviewers` empty before and after. And the queue's own journal at
# `factory/log/supervisor.log`, on the box that runs the timer, says what it did with that state:
#
#   17:58:17Z POKED pr:269 requested review from hermesvicowa    no rows at all: it asked
#   19:17:23Z WAIT  pr:269 awaiting_review                        one voided row: it did not
#
# One activation visited #269 in the voided state and wrote the wait instead of the ask; the next saw
# it conflicting again and dispatched a conflict tick, which asks nobody either. It was re-approved at
# 19:50:20Z against the new head and merged at 20:18:19Z because a person read it, not because anything
# in the queue noticed. A voided approval is the ordinary end state of every conflict-resolution tick
# here -- squash and rebase merge are both disabled, force-push is denied, and reverting the base is
# not an operation -- so the shape below is the shape every tick arrives at.
#
# A verdict is `APPROVED` or `CHANGES_REQUESTED`: a state that says somebody formed an opinion about
# the code. `DISMISSED` and `COMMENTED` are not one, and the legs below are the rule at each polarity
# -- voided with nobody asked, voided with somebody already asked, comments only, nothing at all, and
# a verdict that still stands. The last two are the regressions: the never-reviewed sentence three legs
# above already pin has to survive the new arm unchanged, and an `APPROVED` must not be swept into it.
voided_pr() { # voided_pr <review rows> [requested_reviewers payload] [reviewDecision]
	# The plant is checked for being JSON before anything reads it, and that is not ceremony. The first
	# draft of the `COMMENTED` plant below was a jq object literal -- unquoted keys, which `jq -c` accepts
	# and JSON does not -- so `reviews.json` held bytes `jq` could not parse, `any_review` fell back to
	# `0`, and the leg that wanted "a comment is not a verdict" went GREEN off the never-reviewed arm with
	# the detail `no review and nobody asked since it opened`. A plant that does not parse is a leg that
	# measures the empty state and reports the state it asked about, which is the exact shape this suite
	# calls a leg that cannot fail. The paired detail assertion is what made it visible; this is what
	# stops it being visible only to whoever reads the failure text.
	printf '%s' "${1:-}" | jq -e 'type == "array"' >/dev/null 2>&1 || {
		printf '            voided_pr: the review plant is not a JSON array, so nothing can be concluded\n' >&2
		printf '            from what the classifier answers over it: %s\n' "${1:-}" >&2
		claim_reset # empty the fixture: a plant that failed to parse must not leave last leg's rows
		return 3    # behind it, or the leg below can read green off a payload it never planted
	}
	claim_reset
	# Two units, for the reason `ask_pr240` states: the live issue set holds seven unclaimed rows, the
	# tick budget is three, and a dry run that spends it on issues DEFERS everything else -- so a leg
	# that asks what the supervisor DID has to remove the interference first.
	claim_issues '[.[] | select(.number == 232)]'
	# `reviewDecision` is the third argument because the row set alone does not decide the arm: GitHub
	# answers `REVIEW_REQUIRED` over a voided verdict and `APPROVED` over a standing one, and the last
	# leg below needs the second to prove the new arm does not swallow it.
	claim_open "[{\"number\":269,\"title\":\"A conflict-resolution push dismisses the approval it needed\",\"body\":\"no claim here\",\"claims\":[],\"reviewDecision\":\"${3:-REVIEW_REQUIRED}\"}]"
	claim_reviews "$1"
	claim_reviewers "${2:-}"
}
# GitHub's own row for the state #269 was measured in: the voided approval, still carrying the
# `submittedAt` of the approval it was (the trap #272 names: reading that field as the dismissal moment
# dates the dismissal twenty minutes BEFORE the push that caused it) and the commit it was filed
# against. `commit_id` is REST's field for what `gh pr view --json reviews` prints as `commit.oid`.
voided_row() {
	jq -cn '[{id:5001,user:{login:"hermesvicowa"},state:"DISMISSED",
		submitted_at:"2026-11-10T18:23:50Z",
		commit_id:"c149fe9ab959dd3f253b744a847377a360ed02d4",
		body:"an approval a push has voided"}]'
}

voided_pr "$(voided_row)"
claim_rows >/dev/null
claim_leg 'a voided approval is not a review' claim_class_is 269 needs_reviewer
claim_leg '…and the row says what it concluded from' claim_detail_has 269 'voided by a push'
claim_leg '…naming the verdict and the commit it was filed against' \
	claim_detail_has 269 'hermesvicowa at c149fe9'
claim_leg '…and that nobody holds the request' claim_detail_has 269 'nobody holds a request'

# The polarity the fix must not lose: somebody already holds the request, so the queue waits. This is
# the leg the fixture could not have been written for until `claim_reviewers` existed -- with
# `/requested_reviewers` welded empty, a classifier that asked twice a day at a reviewer who is already
# looking had no leg to catch it.
voided_pr "$(voided_row)" '{"users":[{"login":"hermesvicowa"}],"teams":[]}'
claim_rows >/dev/null
claim_leg 'a voided verdict with an outstanding request is still a wait' claim_class_is 269 awaiting_review
claim_leg '…and the wait names who holds it' claim_detail_has 269 'requested of hermesvicowa'

# A comment is not a verdict either, and the same count swallowed it. `COMMENTED` is what GitHub leaves
# behind when somebody reviews files without approving, and `reviewDecision` reads `REVIEW_REQUIRED`
# over it exactly as it does over a dismissal.
voided_pr "$(jq -cn '[{id:5002,user:{login:"someone-else"},state:"COMMENTED",
	submitted_at:"2026-11-10T18:23:50Z",commit_id:"c149fe9ab959dd3f253b744a847377a360ed02d4",
	body:"a drive-by comment, which is not an approval"}]')"
claim_rows >/dev/null
claim_leg 'a comment row is not a verdict either' claim_class_is 269 needs_reviewer
claim_leg '…and says comments, not no review' claim_detail_has 269 'no verdict, 1 comment row'

# Two regressions, one per arm the new rule reaches past. The never-reviewed sentence is the one the
# #212/#214/#215 legs were written against and must arrive untouched; the standing `APPROVED` must not
# be swept into the new arm, and the fixture's `mergeStateStatus` is `UNKNOWN`, so the honest answer for
# an approved-but-not-clean pull request is the wait it always was.
voided_pr '[]'
claim_rows >/dev/null
claim_leg 'never reviewed keeps the sentence the #212 legs pin' \
	claim_detail_has 269 'no review and nobody asked since it opened'
voided_pr "$(jq -cn '[{id:5003,user:{login:"hermesvicowa"},state:"APPROVED",
	submitted_at:"2026-11-10T19:50:20Z",commit_id:"e4e2941f82d79aceb9cf2bb5773d4877e0b8db22",
	body:"the re-approval, against the new head"}]')" '' APPROVED
claim_rows >/dev/null
claim_leg 'a verdict that still stands is not the new arm' claim_class_is 269 awaiting_merge

# The row is only half of it: the class has to reach the arm that asks. `--dry-run` is the only way to
# watch that without a model, and the two lines below are the two journal lines #272 is about -- the
# one that must appear, and the one that must not.
voided_asked() {
	case "$(ask_journal)" in
	*'WOULD-POKE pr:269 request review from someoneelse'*) return 0 ;;
	esac
	printf '            the supervisor took 269 somewhere other than a request. Its journal:\n' >&2
	printf '%s\n' "$(ask_journal)" | sed 's/^/              /' | grep -v '^[[:space:]]*$' >&2
	return 1
}
voided_not_a_wait() {
	case "$(ask_journal)" in
	*'WAIT pr:269'*)
		printf '            the journal says WAIT for 269. That is the line #272 was opened with:\n' >&2
		printf '            `19:17:23Z WAIT pr:269 awaiting_review`, written while the pull request\n' >&2
		printf '            sat voided and unasked, on the one activation that saw it that way.\n' >&2
		return 1 ;;
	esac
	return 0
}
voided_pr "$(voided_row)"
ask_dispatch >/dev/null
claim_leg 'the voided row reaches the supervisor as an ask' voided_asked
claim_leg '…and no activation takes it to a wait' voided_not_a_wait
claim_reset

# Both documents have to keep saying it, and neither one is the whole rule. #253's third instruction
# was reported delivered by #255 and was not -- `grep -c yield` over `docs/factory.md` answered 0 at
# `91d992d`, with no paragraph about it in the merge diff -- and #272 is deliberately written twice:
# `SKILL.md` tells the tick to read the review state back after its push and names who re-asks and when,
# `docs/factory.md` tells whoever reads the queue what the classifier counts and how long the silence
# after a dismissal runs. A rule in only one of them is a rule the other half contradicts, and the
# contradiction is invisible from either one alone. Four clauses, each a whole phrase: `grep -qF
# 'DISMISSED'` over a 900-line guide is satisfied by any mention of the word anywhere in it, which is
# how a leg goes on printing ok while reading a different sentence.
voided_docs_say_it() { # [root] -> 0 when both documents carry their rule clause and their reason clause
	local root=$HERE/.. skill guide
	[ -z "${1:-}" ] || root=$1
	skill=$root/.opencode/skills/factory/SKILL.md
	guide=$root/docs/factory.md
	[ -f "$skill" ] && [ -f "$guide" ] || return 2
	# SKILL: the read the tick owes, and who closes the gap it discovers.
	grep -qF 'The push may have spent the approval' "$skill" &&
		grep -qF 'The supervisor re-asks on its next activation' "$skill" &&
		# The guide: the class rule, and the reason a row count is the wrong question.
		grep -qF 'no verdict that still stands' "$guide" &&
		grep -qF 'A `DISMISSED` row is still a row' "$guide"
}
voided_docs_control() {
	local ctrl=$claim_work/voided-docs
	if ! mkdir -p "$ctrl/rules/docs" "$ctrl/rules/.opencode/skills/factory" \
		"$ctrl/reasons/docs" "$ctrl/reasons/.opencode/skills/factory"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control trees could not be written\n' \
			'control: the voided verdict, both documents'
		return 0
	fi
	if ! grep -vF 'The push may have spent the approval' "$HERE/../.opencode/skills/factory/SKILL.md" \
		>"$ctrl/rules/.opencode/skills/factory/SKILL.md" ||
		! grep -vF 'no verdict that still stands' "$HERE/../docs/factory.md" \
			>"$ctrl/rules/docs/factory.md" ||
		! grep -vF 'The supervisor re-asks on its next activation' \
			"$HERE/../.opencode/skills/factory/SKILL.md" \
			>"$ctrl/reasons/.opencode/skills/factory/SKILL.md" ||
		! grep -vF 'A `DISMISSED` row is still a row' "$HERE/../docs/factory.md" \
			>"$ctrl/reasons/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control copies could not be written\n' \
			'control: the voided verdict, both documents'
		return 0
	fi
	if grep -qF 'The push may have spent the approval' \
		"$ctrl/rules/.opencode/skills/factory/SKILL.md" ||
		grep -qF 'no verdict that still stands' "$ctrl/rules/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the rule plant left a rule in place, so the\n' \
			'control: the voided verdict, both documents'
		printf '          leg above was never aimed at the clauses it names.\n'
		return 0
	fi
	if grep -qF 'The supervisor re-asks on its next activation' \
		"$ctrl/reasons/.opencode/skills/factory/SKILL.md" ||
		grep -qF 'A `DISMISSED` row is still a row' "$ctrl/reasons/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the reason plant left a reason in place, so\n' \
			'control: the voided verdict, both documents'
		printf '          the leg above does not need the reasons it claims to need.\n'
		return 0
	fi
	if voided_docs_say_it "$ctrl/rules" || voided_docs_say_it "$ctrl/reasons"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it accepted a pair with one rule clause deleted\n' \
			'control: the voided verdict, both documents'
		printf '          from one document and one reason clause from the other, so the ok above is a\n'
		printf '          phrase match and not the four halves it names.\n'
		return 0
	fi
	printf '  ok    %-40s each copy loses one clause from each document\n' \
		'control: the voided verdict, both documents'
	printf '          and the leg says no; all four are read, not a word nearby.\n'
	return 0
}
claim_leg 'the voided verdict is in both documents' voided_docs_say_it
voided_docs_control
claim_reset

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

# ------------------------------------------- the code the loop is running (#249)
#
# `install-timer.sh` renders `ExecStart` as a path, so the timer runs whatever that tree has checked
# out. Measured 8 Nov: the tree was `efa4e29` and `origin/main` was 121 commits later, so every merge
# since the checkout was invisible to the loop meant to act on them -- and a reinstall of that tree
# succeeded and said nothing. The legs below are the four checks #249 asks for, plus the two it does
# not name that the shape of the fix makes possible: which tree is asked of (there are two
# candidates and they are different directories), and that a stale tree can still be taken *off*.
#
# What makes these legs more than a grep for a word:
#
#   the count   a leg that prints `SUPERVISOR-STALE` for every state proves only that `grep` works, so
#               the same tree is asked twice -- behind 3, then fast-forwarded to the trunk, where both
#               sentences have to vanish and the sha has to have moved.
#   the sha     asserted against a planted commit that is known before the call, because a line that
#               prints *a* sha is a `date` in fancy clothes until it prints that one.
#   the fork    a fork and a stale tree coexist and each needs its own sentence, and only one of them
#               has a fast-forward. The fork legs assert `merge --ff-only` is NOT printed -- a
#               refusal that sends an operator to a command git rejects is a second bug.
#   the tree    `$FACTORY_HARNESS` (where the running script was read from) and `$FACTORY_ROOT` (where
#               the unit's ExecStart points) are the same directory only while the harness lives in the
#               primary checkout, which has never been true here (H-0009). Two install legs plant them
#               in opposite states so a call site that asks the wrong one goes red.
#   dirty=      the #228 polarity, run against the informational field: an edit that changes bytes
#               counts 1, the same bytes written back plus a touched mtime counts 0.
#
# Nothing here talks to GitHub or to systemd. `gh` is a shim that answers `auth status` and refuses
# anything else, `systemctl` and `loginctl` are shims, `go`/`gofmt`/`make`/`python3` are stubs so the
# legs do not read the host's toolchain (H-0054 is the precedent: a section that needs a binary the
# gate does not install is green on every box in the loop and red on CI), and the unit directory is
# `$pos_units` under $TMPDIR.
#
# Nor does it read the host's git identity. Every repository the section creates is given one where it
# is created, because a clone inherits no local config and a CI runner has no `~/.gitconfig` identity
# to fall back on -- which is exactly how this section died on its first CI run (H-0059, pinned by
# `no fixture repo inherits the host`).

pos_work=$work/position
pos_bin=$pos_work/bin
pos_up=$pos_work/upstream
pos_units=$pos_work/units
pos_log=$pos_work/log
mkdir -p "$pos_bin" "$pos_units" "$pos_log"

# One upstream, six commits, with a committed executable `factory/factory.sh` so a clone passes the
# installer's `-x` check the way a real checkout does -- and stays clean while doing it. An untracked
# file planted after the clone would have put `1 uncommitted` in every install message and made the
# dirty legs read their own fixture.
git init -q "$pos_up"
git -C "$pos_up" config user.email fixture@invalid
git -C "$pos_up" config user.name fixture
git -C "$pos_up" checkout -qb main
mkdir -p "$pos_up/factory"
printf '#!/usr/bin/env bash\nexit 0\n' >"$pos_up/factory/factory.sh"
chmod +x "$pos_up/factory/factory.sh"
# A tracked file to edit in the dirty legs. Planted here rather than written into a clone afterwards,
# because an untracked file is already counted as work and would put `1 uncommitted` in every message
# the install legs read.
printf 'seed\n' >"$pos_up/README.md"
git -C "$pos_up" add -A
git -C "$pos_up" commit -qm 'fixture harness'
for i in 2 3 4 5 6; do
	git -C "$pos_up" commit -q --allow-empty -m "merged $i"
done
pos_tip=$(git -C "$pos_up" rev-parse --short HEAD)

# Every repo this section creates is given an identity where it is created, and not by the box. A
# clone copies the upstream's *remote* and none of its local config, so `user.email` set on `$pos_up`
# says nothing about the clones -- and two of the statements below commit inside a clone. Measured
# 9 Nov: this box has a `~/.gitconfig` with an identity in it, a GitHub runner has none, and CI died
# at the first such commit with `Author identity unknown`, 217 legs printed and no verdict (H-0059).
# Identity is planted per repository rather than exported as `GIT_AUTHOR_*` for the section so that a
# future leg can still plant the *absence* of one on purpose; the host's is never what a leg reads.
pos_clone() { # pos_clone <name> -> a fresh clone of the fixture trunk, echoed as a path
	local path=$pos_work/tree-$1
	git clone -q "$pos_up" "$path"
	git -C "$path" config user.email fixture@invalid
	git -C "$path" config user.name fixture
	printf '%s\n' "$path"
}

# The box the installer runs on: shims for the four binaries it talks to, stubs for the four the
# generated unit's PATH is probed for. `git` is deliberately NOT here -- the verdict under test is a
# reading of real git, and a stubbed git would make every leg below vacuous.
cat >"$pos_bin/gh" <<'POSGH'
#!/usr/bin/env bash
# `gh auth status` is all the installer's preflight asks. Anything else exits 9: a leg that could
# reach GitHub would be writing to the queue it only exists to describe.
case "$1 $2" in "auth status") exit 0 ;; esac
printf 'pos-shim gh: unhandled call: %s\n' "$*" >&2
exit 9
POSGH
cat >"$pos_bin/systemctl" <<'POSSC'
#!/usr/bin/env bash
printf 'pos-shim systemctl %s\n' "$*"
exit 0
POSSC
printf '#!/usr/bin/env bash\nprintf "yes\\n"\nexit 0\n' >"$pos_bin/loginctl"
for pos_tool in opencode go gofmt make python3; do
	printf '#!/usr/bin/env bash\nexit 0\n' >"$pos_bin/$pos_tool"
done
chmod +x "$pos_bin"/*

# pos_install <root> <harness-dir> <args...> -> the installer's exit status, with its combined output
# in $pos_out and whatever it wrote in $pos_units. Every path it can reach is redirected into the
# fixture first: the real ~/.config/systemd/user and the user's own manager are not this file's to
# touch, which is the same rule the rest of this suite runs by.
pos_u=$pos_units/systemd/user
pos_unit_file=$pos_u/posfix-factory.service
mkdir -p "$pos_u"

# Whatever a previous leg left at the unit path, gone: file, symlink, or the empty directory two legs
# below plant on purpose. It is called from `pos_unit_exec`, and from nowhere else: the install legs
# above deliberately arrive at whatever state the previous one left, which IS their subject, so a clear
# inside `pos_install` turned `install refuses a stale tree` into a leg that no longer had a tree to
# refuse -- measured at `344 passed, 12 failed` when this helper was wired into that function.
pos_unit_clear() {
	if [ -z "${pos_unit_file:-}" ]; then
		[ -n "${pos_u:-}" ] || return 0
		printf 'pos_unit_clear: %s is set and %s is not\n' "${pos_u}" 'pos_unit_file' >&2
		return 1
	fi
	rm -f -- "$pos_unit_file" 2>/dev/null
	rmdir -- "$pos_unit_file" 2>/dev/null
	# A NON-EMPTY directory at that path is not this suite's doing and does not get deleted on a guess.
	if [ -d "$pos_unit_file" ]; then
		printf 'pos_unit_clear: %s is a directory that will not go away\n' "$pos_unit_file" >&2
		return 1
	fi
	return 0
}

pos_install() {
	local root=$1 harness=$2
	shift 2
	pos_rc=0
	pos_out=$(PATH="$pos_bin:$PATH" XDG_CONFIG_HOME="$pos_units" FACTORY_NAME=posfix \
		FACTORY_REPO=example/repo FACTORY_DIR="$harness/factory" FACTORY_ROOT="$root" \
		FACTORY_LOG="$pos_log" FACTORY_LEDGER="$pos_log/ledger" FACTORY_RUN="$pos_log/run" \
		bash "$HERE/install-timer.sh" "$@" 2>&1) || pos_rc=$?
}

pos_units_written() { find "$pos_units" -type f 2>/dev/null | wc -l | tr -d ' '; }
pos_say() { printf '%s\n' "$*" | sed 's/^/            /' >&2; }

# The states, planted by moving a fresh clone: an ancestor of the trunk is what "behind" means, and
# an empty local commit is what "a head main does not have" means. Both are the shapes the ticket
# prescribes, built with git rather than hand-forced.
pos_stale=$(pos_clone stale)
git -C "$pos_stale" checkout -qb unit HEAD~3
pos_tip3=$(git -C "$pos_up" rev-parse --short HEAD~3)

# A second stale tree, for the leg that moves one to the trunk. Without its own copy the leg would
# arrive at the install legs having already fixed the state they are about to refuse -- which is
# exactly what happened the first time this section ran: `install refuses a stale tree` went green
# for the wrong reason, because a leg above it had already fast-forwarded the tree.
pos_moved=$(pos_clone moved)
git -C "$pos_moved" checkout -qb unit HEAD~3

pos_fork=$(pos_clone fork)
git -C "$pos_fork" checkout -qb unit HEAD~2
git -C "$pos_fork" commit -q --allow-empty -m 'local work main does not have'

pos_detach=$(pos_clone detach)
git -C "$pos_detach" checkout -qb unit HEAD~2
git -C "$pos_detach" commit -q --allow-empty -m 'local work on a detached head'
git -C "$pos_detach" checkout -q --detach HEAD

pos_nobase=$(pos_clone nobase)
git -C "$pos_nobase" remote rename origin upstream
pos_plain=$pos_work/plain
mkdir -p "$pos_plain"

# leg: the lines for one tree, as one string. `pos_at <tree>` pulls the AT line out so a leg can pin
# the sha without re-implementing the sentence.

# --- the verdict, on trees that exist -------------------------------------------------------

pos_at_sha() { # the sha the AT line names for <tree>
	factory_supervisor_lines "$1" | awk '/^SUPERVISOR-AT/{print $2; exit}'
}
claim_leg 'AT names the planted commit' test "$(pos_at_sha "$pos_stale")" = "$pos_tip3"
claim_leg 'AT is a real read, not the trunk tip' test "$(pos_at_sha "$pos_stale")" != "$pos_tip"

stale_then_clean() {
	local tree=$1 out
	out=$(factory_supervisor_lines "$tree")
	printf '%s\n' "$out" | grep -q '^SUPERVISOR-STALE origin/main is 3 commits ahead; the loop is not running merged code' || {
		pos_say "the sentence #249 asks for, verbatim, is not there; got:"; pos_say "$out"; return 1; }
	# The half that stops this being a grep for a word: the same tree, moved to the trunk, prints
	# neither sentence -- and the sha it prints has moved with it.
	git -C "$tree" merge -q --ff-only origin/main || { pos_say 'the fixture could not fast-forward'; return 1; }
	out=$(factory_supervisor_lines "$tree")
	printf '%s\n' "$out" | grep -qE '^SUPERVISOR-(STALE|FORKED)' && {
		pos_say "a tree at origin/main still reports a problem:"; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -q "^SUPERVISOR-AT $pos_tip " || {
		pos_say "the AT line does not name the trunk tip it moved to:"; pos_say "$out"; return 1; }
	return 0
}
claim_leg 'behind 3, then silent once moved' stale_then_clean "$pos_moved"

forked_says_both() {
	local out
	out=$(factory_supervisor_lines "$pos_fork")
	printf '%s\n' "$out" | grep -q '^SUPERVISOR-STALE origin/main is 2 commits ahead' || {
		pos_say "the stale half of a forked tree is its own fact and is missing:"; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -q '^SUPERVISOR-FORKED unit holds 1 commit origin/main' || {
		pos_say "no FORKED line naming the branch and its count:"; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -q 'ff-only' && {
		pos_say "a fork has no fast-forward; the sentence recommends one anyway:"; pos_say "$out"; return 1; }
	return 0
}
claim_leg 'a fork says STALE and FORKED, no ff-only' forked_says_both

claim_leg 'a detached head is named, not blank' test \
	"$(factory_supervisor_lines "$pos_detach" | grep -c '^SUPERVISOR-FORKED detached holds 1 commit origin/main')" = 1

nobase_says_unknown() {
	local out rc=0 pos
	pos=$(factory_supervisor_position "$pos_nobase") || rc=$?
	[ "$rc" -eq 3 ] || { pos_say "expected exit 3 from a tree that cannot resolve origin/main, got $rc"; return 1; }
	# The raw verdict: the two counts unknown, the identity and the dirt still read. Field 1 is
	# asserted against the sha rather than against a sentence, so this is a read and not a print.
	[ "$(printf '%s' "$pos" | cut -f1)" = "$(git -C "$pos_nobase" rev-parse --short HEAD)" ] || {
		pos_say "the sha field is not the tree's own HEAD: $pos"; return 1; }
	if [ "$(printf '%s' "$pos" | cut -f3)" != '?' ] || [ "$(printf '%s' "$pos" | cut -f4)" != '?' ]; then
		pos_say "an unresolvable trunk ref must leave both counts unknown, not 0: $pos"
		return 1
	fi
	[ "$(printf '%s' "$pos" | cut -f5)" = 0 ] || {
		pos_say "the dirt count is read from the working tree, which does not need the trunk: $pos"; return 1; }
	out=$(factory_supervisor_lines "$pos_nobase")
	printf '%s\n' "$out" | grep -q '^SUPERVISOR-NOBASE origin/main cannot be resolved' || {
		pos_say "an unresolvable trunk ref is not a clean bill of health:"; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -qE '^SUPERVISOR-(STALE|FORKED)' && {
		pos_say 'a verdict that cannot see the trunk printed a verdict about the trunk anyway:'; pos_say "$out"; return 1; }
	# The sha is still worth printing: it is the thing that identifies the code, and it reads fine
	# without the trunk. Refusing to say anything at all would trade one silence for another.
	printf '%s\n' "$out" | grep -q "^SUPERVISOR-AT $(git -C "$pos_nobase" rev-parse --short HEAD) dirty=0" || {
		pos_say "the AT line went missing with the trunk ref:"; pos_say "$out"; return 1; }
	return 0
}
claim_leg 'no trunk ref says NOBASE, not 0 behind' nobase_says_unknown

unreadable_says_unknown() {
	local out rc=0
	rc=0
	factory_supervisor_position "$pos_plain" >/dev/null || rc=$?
	[ "$rc" -eq 2 ] || { pos_say "expected exit 2 from a plain directory, got $rc"; return 1; }
	out=$(factory_supervisor_lines "$pos_plain")
	printf '%s\n' "$out" | grep -q '^SUPERVISOR-UNKNOWN ' || {
		pos_say "a tree git cannot read must not print a clean verdict:"; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -q '^SUPERVISOR-AT' && {
		pos_say "there is no sha to print for a directory that is not a checkout:"; pos_say "$out"; return 1; }
	return 0
}
claim_leg 'an unreadable tree says UNKNOWN' unreadable_says_unknown

# `dirty=<n>`, both polarities of the lesson that halts the queue (#228): the count is a content
# question. The same bytes written back, with the mtime moved, is not work -- and `update-index
# --refresh` ahead of the read is the step that makes that true rather than hoped.
dirty_counts_bytes() {
	local tree=$1 got stamped
	got=$(factory_supervisor_position "$tree" | cut -f5)
	[ "$got" = 0 ] || { pos_say "a clean fixture must count 0, got '$got'"; return 1; }

	printf 'tracked bytes changed\n' >>"$tree/README.md"
	got=$(factory_supervisor_position "$tree" | cut -f5)
	[ "$got" = 1 ] || { pos_say "an edit that changes bytes must count 1, got '$got'"; return 1; }

	# The same bytes back, and the mtime moved into the past: bytes equal to `HEAD`, stat data that
	# disagrees. That is the state a killed `check-mutations.py` leaves, and the one #228 halts the
	# whole pass over. The plant is checked, not assumed -- the stamped mtime is read out of the index
	# with `ls-files --debug`, which is a read and not a refresh, so the check cannot clear the very
	# difference it is looking for.
	printf 'seed\n' >"$tree/README.md"
	touch -t 202001010000 "$tree/README.md"
	stamped=$(git -C "$tree" ls-files --debug -- README.md | awk '/mtime:/{print $2; exit}')
	stamped=${stamped%%.*}
	[ "$stamped" != "$(stat -c %Y "$tree/README.md")" ] || {
		pos_say 'the write-back left the index and the file agreeing, so the leg below proves nothing'
		return 1; }
	# The disclosure this file already makes about its own stat leg: on git 2.53 `status` refreshes the
	# index in memory and finds the bytes equal either way, so this half pins the invariant rather than
	# proving the `update-index --refresh` above earns its place. On a git or a filesystem that does
	# report stat dirt, this is the line that catches it.
	got=$(factory_supervisor_position "$tree" | cut -f5)
	[ "$got" = 0 ] || { pos_say "the same bytes written back must count 0, got '$got'"; return 1; }

	printf 'not committed\n' >"$tree/untracked-probe.txt"
	got=$(factory_supervisor_position "$tree" | cut -f5)
	[ "$got" = 1 ] || { pos_say "an untracked path must count 1, got '$got'"; return 1; }
	rm -f -- "$tree/untracked-probe.txt"
	return 0
}
claim_leg 'dirty= counts bytes, not mtime' dirty_counts_bytes "$pos_fork"

# A linked worktree is what the timer execs and what the supervisor reads. Its index lives under
# `.git/worktrees/<id>/`, not next to the files, so the verdict is asked in both shapes: one that is
# only right beside `.git/` would be wrong where it is actually called.
worktree_answers_alike() {
	local wt=$pos_work/tree-worktree both want
	git -C "$pos_fork" worktree add -q --detach "$wt" HEAD~1 || {
		pos_say 'the fixture could not add a linked worktree'; return 1; }
	# The same state, asked of both trees. Detaching the linked tree at HEAD~1 is not the same commit
	# as `unit`, so the comparison is between each tree's own verdict and its own `git rev-list`.
	want=$(git -C "$wt" rev-list --count HEAD..origin/main)
	both=$(factory_supervisor_position "$wt")
	[ "$(printf '%s' "$both" | cut -f3)" = "$want" ] || {
		pos_say "behind differs from git's own count in a linked worktree: want $want, got $both"; return 1; }
	printf '%s' "$both" | grep -qF "$(git -C "$wt" rev-parse --short HEAD)" || {
		pos_say "the worktree's own sha is not what was printed: $both"; return 1; }
	return 0
}
claim_leg 'a linked worktree answers alike' worktree_answers_alike

# --- the installer, end to end --------------------------------------------------------------
#
# These run the shipped `install-timer.sh`. Not a copy of its logic, and not a grep for its strings:
# the file is executed with its unit directory, its journal, its `gh` and its `systemctl` all pointed
# into the fixture, so the refusal legs prove the units were not written and the force legs prove
# they were.

pos_current=$(pos_clone current)

install_refuses_stale() {
	pos_install "$pos_stale" "$pos_stale" --install
	[ "$pos_rc" -ne 0 ] || { pos_say "a stale tree installed anyway; output was:"; pos_say "$pos_out"; return 1; }
	[ "$(pos_units_written)" = 0 ] || {
		pos_say "the refusal wrote $(pos_units_written) unit file(s) before refusing:"; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qF 'git -C '"$pos_stale"' merge --ff-only origin/main' || {
		pos_say "the refusal has to hand over the command that fixes it:"; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qF '3 commits behind origin/main' || {
		pos_say 'the refusal does not name the count:'; pos_say "$pos_out"; return 1; }
	return 0
}
claim_leg 'install refuses a stale tree, writes none' install_refuses_stale

install_force_installs() {
	pos_install "$pos_stale" "$pos_stale" --install --force
	[ "$pos_rc" -eq 0 ] || { pos_say "--force did not install (rc=$pos_rc):"; pos_say "$pos_out"; return 1; }
	[ "$(pos_units_written)" = 2 ] || {
		pos_say "--force wrote $(pos_units_written) unit file(s), want 2:"; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qF -- '--force given, installing' || {
		pos_say 'the forced install did not say it was forced:'; pos_say "$pos_out"; return 1; }
	# The journal is where the next reader looks, so the override has to be a line there with the
	# numbers in it -- and the unit name is derived from the repository, which is why this greps for
	# the verb and the fields rather than a full line it would have to re-derive.
	grep -qE 'TIMER-FORCED .* code=[0-9a-f]+ branch=unit behind=3 ahead=0' "$pos_log/supervisor.log" || {
		pos_say 'the override is not in the journal, so the next reader cannot see what was forced:'
		tail -n 5 "$pos_log/supervisor.log" | sed 's/^/            /' >&2; return 1; }
	return 0
}
claim_leg '--force installs and says so, twice' install_force_installs

install_current_silent() {
	pos_install "$pos_current" "$pos_current" --install
	[ "$pos_rc" -eq 0 ] || { pos_say "a current tree was refused (rc=$pos_rc):"; pos_say "$pos_out"; return 1; }
	# Paths first. A leg that greps the whole output for the words `behind`, `stale` and `fork` is
	# asking whether the installer said anything about the tree's *position*; it never meant to forbid
	# naming a checkout, and naming one is now half of what #260 prints -- the fixture's trees are
	# `tree-stale` and `tree-behind`, so the paths themselves carry the words.
	printf '%s' "$pos_out" | sed -e "s#$pos_work/[^ ']*#<checkout>#g" |
		grep -qiE 'behind|stale|fork|force' && {
		pos_say 'a tree at the trunk was told something about its position:'; pos_say "$pos_out"; return 1; }
	return 0
}
claim_leg 'a current tree installs without a word' install_current_silent

install_fork_no_ff() {
	pos_install "$pos_fork" "$pos_fork" --install
	[ "$pos_rc" -ne 0 ] || { pos_say 'a forked tree installed without a refusal:'; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qF 'merge --ff-only' && {
		pos_say 'a fork has no fast-forward; the refusal recommends one anyway:'; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qF '1 ahead of it' || {
		pos_say 'the refusal does not say the tree is ahead as well as behind:'; pos_say "$pos_out"; return 1; }
	# Wrapped for a terminal, so the phrase is matched across the line break it lands on -- a grep for
	# a sentence the printer splits in two is a grep that can never fire.
	printf '%s' "$pos_out" | tr '\n' ' ' | grep -qE 'fast-forward is +not available' || {
		pos_say 'the refusal does not say what a fork cannot do:'; pos_say "$pos_out"; return 1; }
	return 0
}
claim_leg 'the fork refusal names no fast-forward' install_fork_no_ff

# The polarity #249 does not spell out and the shape of the fix demands: a stale tree is the worst
# possible reason to be unable to take the timer off. `--remove` runs before the check by design.
install_remove_never_refused() {
	pos_install "$pos_stale" "$pos_stale" --remove
	[ "$pos_rc" -eq 0 ] || {
		pos_say "removing a timer was refused because the tree is old (rc=$pos_rc):"; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qi 'refusing' && {
		pos_say 'a removal must never be refused for staleness:'; pos_say "$pos_out"; return 1; }
	return 0
}
claim_leg 'a stale tree can still be taken off' install_remove_never_refused

# Which tree is asked of. The unit execs `$FACTORY_ROOT`, so that is the tree an install must judge --
# and the two here are in opposite states, so a call site that reaches for the tree the *script* came
# from goes red on one of these two legs rather than silently agreeing forever.
install_judges_the_execd_tree() {
	# FACTORY_ROOT stale, the script's own tree current: must refuse.
	pos_install "$pos_stale" "$pos_current" --install
	[ "$pos_rc" -ne 0 ] || {
		pos_say 'install judged the script'"'"'s own tree: a stale FACTORY_ROOT installed:'
		pos_say "$pos_out"; return 1; }
	# FACTORY_ROOT current, the script's own tree stale: must install.
	pos_install "$pos_current" "$pos_stale" --install
	[ "$pos_rc" -eq 0 ] || {
		pos_say "install refused a current tree because the harness it was run from is old:"; pos_say "$pos_out"; return 1; }
	return 0
}
claim_leg 'install judges the ExecStart tree' install_judges_the_execd_tree

install_nobase_refuses() {
	pos_install "$pos_nobase" "$pos_nobase" --install
	[ "$pos_rc" -ne 0 ] || { pos_say 'a tree with no trunk ref installed anyway:'; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qF 'cannot be resolved' || {
		pos_say 'the refusal does not name the missing trunk ref:'; pos_say "$pos_out"; return 1; }
	pos_install "$pos_nobase" "$pos_nobase" --install --force
	[ "$pos_rc" -eq 0 ] || { pos_say "--force did not override the missing ref (rc=$pos_rc):"; pos_say "$pos_out"; return 1; }
	return 0
}
claim_leg 'no trunk ref refuses, --force overrides' install_nobase_refuses

# A checkout with nothing in it. This is the state that reaches the UNKNOWN arm of the installer: a
# directory that is not a repository at all is refused two lines earlier, by `factory_preflight`, and
# a git checkout whose HEAD does not resolve yet is not -- it is a real repository, `rev-parse --git-dir`
# answers, and the only thing missing is a commit. `git init` in a fresh directory is how an operator
# gets there by accident: clone into the wrong path, copy the harness in, install.
pos_empty=$pos_work/tree-empty
mkdir -p "$pos_empty/factory"
# Nothing commits here today -- the leg below wants a HEAD that does not resolve. The identity is
# planted anyway because the section's invariant is about where an identity comes from, not about
# which legs happen to commit right now (H-0059).
git init -q "$pos_empty"
git -C "$pos_empty" config user.email fixture@invalid
git -C "$pos_empty" config user.name fixture
printf '#!/usr/bin/env bash\nexit 0\n' >"$pos_empty/factory/factory.sh"
chmod +x "$pos_empty/factory/factory.sh"

install_no_commit_refuses() {
	local rc=0
	factory_supervisor_position "$pos_empty" >/dev/null || rc=$?
	[ "$rc" -eq 2 ] || { pos_say "the fixture does not reach exit 2 from the verdict, got $rc"; return 1; }
	pos_install "$pos_empty" "$pos_empty" --install
	[ "$pos_rc" -ne 0 ] || { pos_say 'a checkout with no commits was armed:'; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qF 'not a readable git checkout' || {
		pos_say 'the refusal does not say the tree could not be read:'; pos_say "$pos_out"; return 1; }
	return 0
}
claim_leg 'a checkout with no commits is refused' install_no_commit_refuses

# --- the call sites in factory.sh -----------------------------------------------------------
#
# No leg here can run a real pass -- a pass reads GitHub and starts ticks -- so the two places the
# verdict reaches the journal and `--status` are pinned the way `wired_into_verify` pins its own
# step: a scan that is re-run against a copy with the call taken out, in the opposite polarity, so an
# ok means the scan can see and not that it is blind. A function that ships with no call site is the
# silence this file keeps being written against.
pos_verdict_calls() { # <factory.sh> -> every call site, with the tree each one asks of
	grep -oE 'factory_supervisor_(lines|position) "\$FACTORY_[A-Z]+"' "$1" 2>/dev/null | sort -u
}
# The `--status)` arm, to the `;;` that CLOSES it -- not to the first `;;` anywhere. The two are not the
# same text: the arm contains a `case` of its own, so the first `;;` is inside it, three tabs deep, and
# everything the arm prints after that point is invisible to this scan. Measured the day it mattered:
# the old one-liner stopped at the `;;` closing `case "$pos_rc" in` inside the `code:` arm, so a leg
# grepping this for anything later in the arm -- the `unit:` line #260 adds -- read nothing and could
# only ever fail. It is the same defect this section is about: a scan that cannot see the thing it
# claims to check. The closing `;;` is found by indentation, which is what `gofmt`-like discipline buys
# here: a nested arm is always deeper than the arm that owns it.
status_arm() { # <factory.sh> -> the `--status)` arm, to the `;;` that closes it
	awk '/^[[:space:]]*--status\)/{ f = 1; want = 1; print; next }
		f{
			if (want && $0 ~ /[^ \t]/) { ind = $0; sub(/[^ \t].*$/, "", ind); want = 0 }
			print
		}
		f && ind != "" && index($0, ind ";;") == 1 { exit }' "$1" 2>/dev/null
}

# Both call sites, and the tree each asks of. The pass reports the code under it (`$FACTORY_HARNESS`);
# `--status` answers for the same tree, because it is the same harness the person is standing in.
pos_call_sites() {
	local calls
	calls=$(pos_verdict_calls "$HERE/factory.sh")
	printf '%s\n' "$calls" | grep -qxF 'factory_supervisor_lines "$FACTORY_HARNESS"' || {
		pos_say "nothing logs the sentences in the pass. What the file asks of:"; pos_say "$calls"; return 1; }
	printf '%s\n' "$calls" | grep -qxF 'factory_supervisor_position "$FACTORY_HARNESS"' || {
		pos_say "--status does not ask for the position of the harness tree. It asks of:"; pos_say "$calls"; return 1; }
	status_arm "$HERE/factory.sh" | grep -qF 'code:' || {
		pos_say '--status does not print the code: line'; return 1; }
	# The control, planted outside the repository: the same two scans over a copy whose pass call is
	# deleted and whose --status asks of FACTORY_ROOT instead. If the scan still says yes it sees
	# nothing, and every ok above is the check being blind.
	mkdir -p "$work/poscalls/factory"
	sed -e '/factory_supervisor_lines "\$FACTORY_HARNESS"/d' \
		-e 's#factory_supervisor_position "\$FACTORY_HARNESS"#factory_supervisor_position "$FACTORY_ROOT"#' \
		"$HERE/factory.sh" >"$work/poscalls/factory/factory.sh"
	calls=$(pos_verdict_calls "$work/poscalls/factory/factory.sh")
	printf '%s\n' "$calls" | grep -qxF 'factory_supervisor_lines "$FACTORY_HARNESS"' && {
		pos_say 'control failed: the scan still found the deleted pass call'; return 1; }
	printf '%s\n' "$calls" | grep -qxF 'factory_supervisor_position "$FACTORY_ROOT"' || {
		pos_say 'control failed: the scan cannot see a call site pointed at the wrong tree'; return 1; }
	return 0
}
claim_leg 'both call sites ask for the harness tree' pos_call_sites
# --- the installed unit, read instead of asserted (#260) -----------------------------------------
#
# Everything above asks a *tree* where it stands. This asks a *unit* what it runs. The failure was a
# guard reading the position of a tree the loop never ran: `install-timer.sh` printed `The unit execs
# <FACTORY_ROOT>/factory/factory.sh`, composed from the template it was about to write, while
# `systemctl --user cat` on the same box printed a different checkout -- so the guard checked a tree
# the loop never ran. These legs therefore plant unit files (in `$pos_units`, never the operator's
# `~/.config/systemd/user`, which is what `$FACTORY_UNITS_DIR` is for), and the discriminating half is
# that the same call is asked twice across a rewrite of the file: a function that echoed
# `$FACTORY_ROOT` would get the first question right as well.
#
# The unit these legs plant is written by hand and not with the generator on purpose -- the generator
# writes what it intends, and the whole point of #260 is that the installed unit may say something
# other than what any generator intended. Every install here passes `--name posfix`, so the file the
# installer writes and the file these legs plant are the same file by construction. That is not a
# detail: `pos_install` exports `FACTORY_NAME=posfix` and `factory_preflight` overwrites it from
# `FACTORY_REPO` (common.sh `FACTORY_NAME=${FACTORY_REPO##*/}`), so the name the installer resolves on
# its own is `repo-factory` and a leg that trusted the exported variable planted its unit where nothing
# would ever read it -- which reads as "nothing armed", the wrong answer arriving as a pass.

# The install legs above leave whatever they wrote. Started empty so every assertion below reads only
# what this section planted.
rm -f -- "$pos_u"/*

pos_unit_exec() { # pos_unit_exec <lines for the [Service] section...>
	local l
	# Clearing first is load-bearing, not tidiness. Two legs below plant an EMPTY DIRECTORY at the unit
	# path to stand for "a unit file that cannot be read", and each removes it only on the path where
	# its assertion passes. Measured 10 Nov, probing what one collapsed arm costs: with the unreadable
	# arm made to answer like an uninstalled one, that leg returned early, the directory survived, and
	# every `>"$pos_unit_file"` after it died with `Is a directory` -- six legs red, four of them
	# reporting `installing a current tree over a moved unit failed (rc=1)` about the installer, which
	# had done nothing. One leg's failure must not arrive as four other legs' false accusations.
	pos_unit_clear || return 1
	printf '[Unit]\nDescription=planted\n[Service]\nType=oneshot\n' >"$pos_unit_file" || return 1
	for l in "$@"; do printf '%s\n' "$l" >>"$pos_unit_file" || return 1; done
	return 0
}

installed_tree_is_a_read() {
	local got rc
	# Three different fixture trees, none of them `$FACTORY_ROOT` and none the directory the suite
	# lives in: an answer equal to either of those was composed from a global, not read off the file.
	pos_unit_exec "ExecStart=$pos_stale/factory/factory.sh"
	rc=0
	got=$(factory_installed_tree "$pos_unit_file") || rc=$?
	[ "$rc" -eq 0 ] || { pos_say "a planted unit was refused (rc=$rc)"; return 1; }
	[ "$got" = "$pos_stale" ] || { pos_say "ExecStart names $pos_stale, the read answered '$got'"; return 1; }
	# Ask again after rewriting the file underneath the same call.
	pos_unit_exec "ExecStart=$pos_current/factory/factory.sh"
	rc=0
	got=$(factory_installed_tree "$pos_unit_file") || { pos_say 'the rewritten unit was refused'; return 1; }
	[ "$got" = "$pos_current" ] || { pos_say "the answer did not move with the file: '$got'"; return 1; }
	# systemd's own execution prefix (`-` ignores failure), and a comment that names ExecStart.
	pos_unit_exec '# ExecStart=/decoy/factory/factory.sh' "ExecStart=-$pos_fork/factory/factory.sh"
	rc=0
	got=$(factory_installed_tree "$pos_unit_file") || { pos_say "an '-' prefix was refused"; return 1; }
	[ "$got" = "$pos_fork" ] || {
		pos_say "a commented ExecStart was read as the command, or the prefix was left on: '$got'"
		return 1; }
	return 0
}
claim_leg 'the installed tree is read from the unit' installed_tree_is_a_read

installed_tree_refuses_to_guess() {
	local rc got
	# Nothing armed: rc 1 and NOTHING on stdout, so `UNIT-UNINSTALLED` cannot be printed with a path
	# the function invented.
	rm -f -- "$pos_unit_file"
	rc=0
	got=$(factory_installed_tree "$pos_unit_file") || rc=$?
	[ "$rc" -eq 1 ] || { pos_say "a missing unit answered rc=$rc, want 1"; return 1; }
	[ -z "$got" ] || { pos_say "a missing unit printed '$got'"; return 1; }
	# Something at the path that is not a readable regular file is UNKNOWN, not "nothing is armed" --
	# the #228 polarity, one read failing is not evidence of the clean state. A directory and not
	# `chmod 000`: root ignores a mode, and a leg that only goes red for non-root reads green in a
	# container, which is where the loop is being run.
	mkdir -p -- "$pos_unit_file"
	rc=0
	factory_installed_tree "$pos_unit_file" >/dev/null 2>&1 || rc=$?
	[ "$rc" -eq 4 ] || { pos_say "a directory at the unit path answered rc=$rc, want 4"; return 1; }
	rmdir -- "$pos_unit_file"
	# A dangling symlink is a unit file pointing nowhere: a third sentence, not the first one.
	ln -s "$pos_work/no-such-unit" "$pos_unit_file"
	rc=0
	factory_installed_tree "$pos_unit_file" >/dev/null 2>&1 || rc=$?
	[ "$rc" -eq 4 ] || { pos_say "a dangling symlink answered rc=$rc, want 4 and not 1"; return 1; }
	rm -f -- "$pos_unit_file"
	# A unit that is not this harness's: the raw value out, no tree invented out of it.
	pos_unit_exec 'ExecStart=/usr/bin/true --serve'
	rc=0
	got=$(factory_installed_tree "$pos_unit_file") || rc=$?
	[ "$rc" -eq 3 ] || { pos_say "a foreign ExecStart answered rc=$rc, want 3"; return 1; }
	[ "$got" = '/usr/bin/true --serve' ] || { pos_say "the raw ExecStart was not passed through: '$got'"; return 1; }
	# Two `ExecStart=` lines -- a hand-written unit, or one with a drop-in. Which checkout the loop runs
	# is not knowable from two answers, so this refuses to choose and prints both.
	pos_unit_exec "ExecStart=$pos_stale/factory/factory.sh" "ExecStart=$pos_current/factory/factory.sh"
	rc=0
	got=$(factory_installed_tree "$pos_unit_file") || rc=$?
	[ "$rc" -eq 3 ] || { pos_say "two ExecStart lines answered rc=$rc, want 3"; return 1; }
	printf '%s' "$got" | grep -qF "$pos_stale" || { pos_say "the first raw value is missing: '$got'"; return 1; }
	printf '%s' "$got" | grep -qF "$pos_current" || { pos_say "the second raw value is missing: '$got'"; return 1; }
	# A unit file that names no command at all.
	pos_unit_exec 'ExecStart='
	rc=0
	factory_installed_tree "$pos_unit_file" >/dev/null 2>&1 || rc=$?
	[ "$rc" -eq 3 ] || { pos_say "an empty ExecStart answered rc=$rc, want 3"; return 1; }
	return 0
}
claim_leg 'an unreadable or foreign unit is refused, not guessed' installed_tree_refuses_to_guess

unit_lines_name_both_trees() {
	local out
	pos_unit_exec "ExecStart=$pos_current/factory/factory.sh"
	out=$(factory_unit_lines "$pos_current" "$pos_unit_file")
	printf '%s\n' "$out" | grep -qF "UNIT-AT $pos_current" || {
		pos_say 'the matching case does not say which checkout the unit execs:'; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -qE 'UNIT-(DIVERGED|FOREIGN|UNINSTALLED|UNREADABLE)' && {
		pos_say 'the matching case also printed a complaint:'; pos_say "$out"; return 1; }
	# The same call, the file rewritten underneath it: exactly one DIVERGED line naming BOTH trees, and
	# no UNIT-AT beside it. A line that said only "stale" would be #249's line again, about the wrong
	# tree -- which is the exact shape of the 10 Nov journal, where the running tree printed neither.
	pos_unit_exec "ExecStart=$pos_stale/factory/factory.sh"
	out=$(factory_unit_lines "$pos_current" "$pos_unit_file")
	[ "$(printf '%s\n' "$out" | grep -c '^UNIT-DIVERGED ')" = 1 ] || {
		pos_say 'not exactly one DIVERGED line:'; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -qF "installed=$pos_stale" || {
		pos_say 'the DIVERGED line does not name the tree the unit execs:'; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -qF "this=$pos_current" || {
		pos_say 'the DIVERGED line does not name the tree the pass runs from:'; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -qF 'UNIT-AT' && {
		pos_say 'a divergent unit printed UNIT-AT as well, which is the silence this is against:'
		pos_say "$out"; return 1; }
	return 0
}
claim_leg 'UNIT-DIVERGED names the unit and the pass' unit_lines_name_both_trees

unit_lines_name_the_arms() {
	local out
	rm -f -- "$pos_unit_file"
	out=$(factory_unit_lines "$pos_current" "$pos_unit_file")
	printf '%s\n' "$out" | grep -qF "UNIT-UNINSTALLED unit=$pos_unit_file" || {
		pos_say 'the no-unit arm does not name the file it looked at:'; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -qE 'UNIT-(AT|DIVERGED)' && {
		pos_say 'an uninstalled unit claimed a tree:'; pos_say "$out"; return 1; }
	pos_unit_exec 'ExecStart=/usr/bin/true'
	out=$(factory_unit_lines "$pos_current" "$pos_unit_file")
	printf '%s\n' "$out" | grep -qF "UNIT-FOREIGN unit=$pos_unit_file" || {
		pos_say 'the foreign arm does not name the file:'; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -qF 'exec=/usr/bin/true' || {
		pos_say 'the foreign arm does not quote the ExecStart it found:'; pos_say "$out"; return 1; }
	rm -f -- "$pos_unit_file"
	mkdir -p -- "$pos_unit_file"
	out=$(factory_unit_lines "$pos_current" "$pos_unit_file")
	printf '%s\n' "$out" | grep -qF "UNIT-UNREADABLE unit=$pos_unit_file" || {
		pos_say 'the unreadable arm does not name the file:'; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -qiE 'UNIT-(AT|UNINSTALLED)' && {
		pos_say 'an unreadable unit was reported as nothing armed, which is the #228 polarity:'
		pos_say "$out"; return 1; }
	rmdir -- "$pos_unit_file"
	return 0
}
claim_leg 'each unit arm names the fact it states' unit_lines_name_the_arms

# Which file a pass looks at when it is not told. The unit names itself; a unit installed before #260
# does not, and then the repository's name answers; `--status` answers before the remote is resolved,
# so there is a third arm. Each is checked against a file planted at the name that arm is expected to
# produce, so an arm that resolves the wrong name reads nothing and reports that, rather than reading a
# neighbour and passing.
unit_name_resolution_arms() {
	local dir=$pos_work/names/systemd/user got rc
	mkdir -p "$dir"
	printf '[Service]\nExecStart=%s/factory/factory.sh\n' "$pos_current" >"$dir/renamed-factory.service"
	printf '[Service]\nExecStart=%s/factory/factory.sh\n' "$pos_current" >"$dir/fromrepo-factory.service"
	printf '[Service]\nExecStart=%s/factory/factory.sh\n' "$pos_current" >"$dir/tree-current-factory.service"
	# arm 1: what the unit records about itself beats both derivations. It is the only arm that
	# survives `install-timer.sh --name other`, because a name chosen on a command line leaves no
	# other trace a later activation can read.
	rc=0
	got=$(FACTORY_UNITS_DIR="$dir" FACTORY_SERVICE_UNIT=renamed-factory.service FACTORY_NAME=fromrepo \
		factory_installed_tree) || rc=$?
	{ [ "$rc" -eq 0 ] && [ "$got" = "$pos_current" ]; } || {
		pos_say "FACTORY_SERVICE_UNIT did not win the name (rc=$rc, answer '$got')"; return 1; }
	# arm 2: the repository's name, when the unit says nothing about itself.
	rc=0
	got=$(FACTORY_UNITS_DIR="$dir" FACTORY_SERVICE_UNIT='' FACTORY_NAME=fromrepo factory_installed_tree) || rc=$?
	{ [ "$rc" -eq 0 ] && [ "$got" = "$pos_current" ]; } || {
		pos_say "the repository's name did not resolve (rc=$rc, answer '$got')"; return 1; }
	# arm 3: the checkout's own directory, with neither set -- the `--status` case, which answers
	# before `factory_preflight` has read the remote. `$pos_current`'s directory is `tree-current`.
	rc=0
	got=$(FACTORY_UNITS_DIR="$dir" FACTORY_SERVICE_UNIT='' FACTORY_NAME='' FACTORY_ROOT="$pos_current" \
		factory_installed_tree) || rc=$?
	{ [ "$rc" -eq 0 ] && [ "$got" = "$pos_current" ]; } || {
		pos_say "the checkout's own directory did not resolve (rc=$rc, answer '$got')"; return 1; }
	# The control: a name with no file behind it answers "nothing armed", not the nearest neighbour.
	rc=0
	got=$(FACTORY_UNITS_DIR="$dir" FACTORY_SERVICE_UNIT=nope-factory.service factory_installed_tree) || rc=$?
	{ [ "$rc" -eq 1 ] && [ -z "$got" ]; } || {
		pos_say "a name with no unit file answered rc=$rc with '$got', want 1 and nothing"; return 1; }
	return 0
}
claim_leg 'the unit is found by its own record, then by repo, then by tree' unit_name_resolution_arms

# The same rule `pos_call_sites` applies to the halt verdict: a function that ships with no call site
# is a silence, and the scan that says it is wired has to prove it can see it unwired.
unit_verdict_calls() { # <factory.sh> -> every call site, with the tree each one asks of
	grep -oE 'factory_unit_lines "\$FACTORY_[A-Z]+"' "$1" 2>/dev/null | sort -u
}
unit_call_sites() {
	local hits control=$work/unitcalls/factory.sh needle arm
	hits=$(unit_verdict_calls "$HERE/factory.sh")
	printf '%s\n' "$hits" | grep -qxF 'factory_unit_lines "$FACTORY_HARNESS"' || {
		pos_say 'nothing logs the installed unit on a pass. What the file asks of:'; pos_say "$hits"; return 1; }
	# One predicate per arm of the `--status` reporter, and this is the shape the leg had to take:
	# asking only whether the arm contains `unit:` is satisfied by any three of its four printfs.
	# Measured 10 Nov -- deleting the matched arm's printf, the line a person reads on a healthy box,
	# left the suite at `355 passed, 1 failed` with that one failure the unrelated at-risk one. So each
	# arm is named by the phrase only it prints, and the control below deletes one arm's sentence at a
	# time and requires the scan to go blind for that arm alone.
	for arm in 'matched:unit:    %s -- %s' 'diverged:NOT the tree running this' \
		'none armed:none armed at' 'unreadable:cannot be read, so what the timer would exec is unknown' \
		'foreign:which is not this harness'; do
		needle=${arm#*:}
		status_arm "$HERE/factory.sh" | grep -qF -- "$needle" || {
			pos_say "--status has no ${arm%%:*} arm (${needle}), so the one command a person types"
			pos_say 'cannot see which checkout the timer execs'
			return 1; }
	done
	mkdir -p -- "$work/unitcalls/factory"
	sed -e '/factory_unit_lines "\$FACTORY_HARNESS"/d' \
		-e 's#unit:#unot:#g' "$HERE/factory.sh" >"$control"
	cmp -s "$HERE/factory.sh" "$control" && {
		pos_say 'the control edited nothing, so both assertions above prove nothing'; return 1; }
	printf '%s\n' "$(unit_verdict_calls "$control")" |
		grep -qxF 'factory_unit_lines "$FACTORY_HARNESS"' && {
		pos_say 'control failed: the scan still found the deleted pass call'; return 1; }
	status_arm "$control" | grep -qF 'unit:' && {
		pos_say 'control failed: the --status scan still matches a copy that prints no unit: line'
		return 1; }
	# The per-arm control: for each arm, a copy with ONLY that arm's sentence taken out must stop
	# satisfying that arm's predicate. A control that deletes all four proves the whole-leg version,
	# which is the assertion P5a showed to be worthless.
	for arm in 'matched:unit:    %s -- %s' 'diverged:NOT the tree running this' \
		'none armed:none armed at' 'unreadable:cannot be read, so what the timer would exec is unknown' \
		'foreign:which is not this harness'; do
		needle=${arm#*:}
		sed "s|$needle|X|" "$HERE/factory.sh" >"$control"
		cmp -s "$HERE/factory.sh" "$control" && {
			pos_say "control: nothing matched the ${arm%%:*} arm's phrase, so its assertion above"
			pos_say 'was never tested against a copy that lacks it'
			return 1; }
		status_arm "$control" | grep -qF -- "$needle" && {
			pos_say "control: the scan still sees the ${arm%%:*} arm after its sentence was deleted"
			return 1; }
	done
	return 0
}
claim_leg 'both consumers ask the unit' unit_call_sites

# The line that makes arm 1 of `factory_service_unit` exist. A unit installed by a pass cannot be found
# by name at activation time unless the install wrote the name down somewhere the activation can read,
# and `--name` is a decision that leaves no other trace: the unit is `$UNITS/<name>.service` and nothing
# else records `<name>`. So the generated unit exports it about itself, and this pins the rendering --
# not by grepping the template for the token, which a substituted unit satisfies even when the value is
# wrong, but by rendering under two different names and requiring both answers to move with it.
unit_records_its_own_name() {
	local a='' b=''
	pos_install "$pos_current" "$pos_current" --install --dry-run --name alpha >/dev/null 2>&1
	a=$pos_out
	pos_install "$pos_current" "$pos_current" --install --dry-run --name beta >/dev/null 2>&1
	b=$pos_out
	printf '%s' "$a" | grep -qxF 'Environment=FACTORY_SERVICE_UNIT=alpha.service' || {
		pos_say 'the rendered unit does not record the name it was installed under:'; pos_say "$a"; return 1; }
	printf '%s' "$b" | grep -qxF 'Environment=FACTORY_SERVICE_UNIT=beta.service' || {
		pos_say 'the recorded name did not follow --name:'; pos_say "$b"; return 1; }
	printf '%s' "$b" | grep -qF 'FACTORY_SERVICE_UNIT=alpha' && {
		pos_say 'the old name is still in the rendered unit:'; pos_say "$b"; return 1; }
	# An invariant rather than a control, and named as such: the discrimination above is the two names
	# moving together, which no single grep can fake. What this adds is that the name lives on its own
	# line and nowhere else, so the day the line is dropped the fix is a one-line render change and not a
	# hunt through the unit for the other place the string appears.
	if printf '%s\n' "$b" | grep -v '^Environment=FACTORY_SERVICE_UNIT=' |
		grep -qF 'FACTORY_SERVICE_UNIT=beta.service'; then
		pos_say 'the recorded name appears somewhere other than its own line, so removing that line'
		pos_say 'would not remove the value from the unit:'; pos_say "$b"; return 1
	fi
	return 0
}
claim_leg 'the unit records the name it was installed under' unit_records_its_own_name

# What an install says when it MOVES the loop to another checkout. This is the moment on 10 Nov that
# made a healthy-looking journal go silent, and it was invisible: `TIMER-INSTALLED` in one tree's log
# at 13:39:05Z, the next `PASS` in another tree's. So the move is said on stdout, journalled, and said
# by a dry run that writes nothing.
install_move_is_said() {
	local jline='' before='' after=''
	pos_unit_exec "ExecStart=$pos_stale/factory/factory.sh"
	rm -f -- "$pos_log/supervisor.log"
	pos_install "$pos_current" "$pos_current" --install --name posfix-factory
	[ "$pos_rc" -eq 0 ] || {
		pos_say "installing a current tree over a moved unit failed (rc=$pos_rc):"; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qF 'The loop MOVES' || {
		pos_say 'an install that moves the loop does not say so:'; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qF 'the HALT path' || {
		pos_say 'the move notice omits what moves with the tree:'; pos_say "$pos_out"; return 1; }
	jline=$(grep -h 'UNIT-TREE' "$pos_log/supervisor.log" 2>/dev/null | tail -n 1 || true)
	printf '%s' "$jline" | grep -qF "UNIT-TREE old=$pos_stale" || {
		pos_say 'the move is not in the journal, where the next reader looks:'
		pos_say "got: ${jline:-<no UNIT-TREE line in $pos_log/supervisor.log>}"
		pos_say "the install printed:"; pos_say "$pos_out"; return 1; }
	printf '%s' "$jline" | grep -qF "new=$pos_current" || {
		pos_say 'UNIT-TREE does not name the tree the loop moves to:'; pos_say "$jline"; return 1; }
	# The dry run has to say it too -- it exists to show what the real one would do -- and write
	# nothing while doing so. The unit is re-planted at the OLD tree first, because the install above
	# just replaced it with one that agrees, and an agreeing dry run has no move to report.
	pos_unit_exec "ExecStart=$pos_stale/factory/factory.sh"
	rm -f -- "$pos_log/supervisor.log"
	before=$(find "$pos_u" -type f 2>/dev/null | sort)
	pos_install "$pos_current" "$pos_current" --install --name posfix-factory --dry-run
	printf '%s' "$pos_out" | grep -qF 'the loop MOVES from' || {
		pos_say 'a dry run that would move the loop is silent about it:'; pos_say "$pos_out"; return 1; }
	after=$(find "$pos_u" -type f 2>/dev/null | sort)
	[ "$before" = "$after" ] || {
		pos_say 'the dry run wrote into the unit directory:'; pos_say "$after"; return 1; }
	[ -e "$pos_log/supervisor.log" ] && {
		pos_say "the dry run journalled: $(tail -n 1 "$pos_log/supervisor.log")"; return 1; }
	return 0
}
claim_leg 'an install that moves the loop says so, on and off' install_move_is_said

# The polarity of the whole #260 read: a refusal has to state where the loop runs TODAY. The shipped
# sentence was `The unit execs <the tree being installed>` -- the sentence that was false on 10 Nov,
# when the refusal was about one checkout and the unit execed another.
install_refusal_states_the_installed_tree() {
	local planted=''
	# The unit execs the FORK; the tree being refused is the STALE one. A refusal that prints only
	# what it is about to write cannot tell the operator which of the two the queue is running.
	pos_unit_exec "ExecStart=$pos_fork/factory/factory.sh"
	planted=$(cat "$pos_unit_file")
	pos_install "$pos_stale" "$pos_stale" --install --name posfix-factory
	[ "$pos_rc" -ne 0 ] || { pos_say 'a 3-behind tree installed anyway'; return 1; }
	printf '%s' "$pos_out" | grep -qF "$pos_fork" || {
		pos_say 'the refusal does not name the checkout the installed unit execs:'; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qiE 'loop runs today' || {
		pos_say 'the refusal names the installed tree without saying it is the live one:'
		pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qF "That tree is at $(git -C "$pos_fork" rev-parse --short HEAD) on unit" || {
		pos_say 'the refusal does not say where the INSTALLED tree stands. The tree it tells you to'
		pos_say 'fast-forward may not be the one that runs, and the running tree is the one that'
		pos_say 'needs the fetch:'; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -E 'That tree is at ' | grep -qF 'behind origin/main' || {
		pos_say 'the sentence about the installed tree gives no position against the trunk:'
		pos_say "$pos_out"; return 1; }
	# The false sentence is gone in both of its shapes: asserted for the tree under refusal, and
	# asserted as though there were only one checkout in the story.
	printf '%s' "$pos_out" | grep -qE "The unit execs $pos_stale" && {
		pos_say 'the refusal still asserts the unit execs the tree under refusal:'; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qF 'this tree IS the supervisor' && {
		pos_say 'the refusal called the tree under refusal the supervisor while the unit execed another:'
		pos_say "$pos_out"; return 1; }
	[ "$planted" = "$(cat "$pos_unit_file")" ] || {
		pos_say 'a refusal rewrote the installed unit:'; pos_say "$(cat "$pos_unit_file")"; return 1; }
	return 0
}
claim_leg 'a refusal says which checkout the loop runs today' install_refusal_states_the_installed_tree

# And the other side of the claim: when the unit and the tree agree, the harness says #249's sentence
# exactly once and invents no two-checkout story out of one.
install_agreeing_unit_says_so_once() {
	local n
	pos_unit_exec "ExecStart=$pos_stale/factory/factory.sh"
	pos_install "$pos_stale" "$pos_stale" --install --name posfix-factory
	[ "$pos_rc" -ne 0 ] || { pos_say 'a 3-behind tree installed anyway'; return 1; }
	n=$(printf '%s' "$pos_out" | grep -cF 'this tree IS the supervisor' || true)
	[ "$n" = 1 ] || { pos_say "#249's sentence should print exactly once, it printed $n:"; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qiE 'loop runs today' && {
		pos_say 'an agreeing install told a two-checkout story:'; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qF 'The loop MOVES' && {
		pos_say 'an agreeing install invented a move:'; pos_say "$pos_out"; return 1; }
	# The control for the leg above: the same installer, the unit pointed at a different tree, and the
	# sentence count goes to zero while the two-tree text appears. Without this the `1` above could be
	# a sentence that prints in both cases.
	pos_unit_exec "ExecStart=$pos_current/factory/factory.sh"
	pos_install "$pos_stale" "$pos_stale" --install --name posfix-factory
	[ "$pos_rc" -ne 0 ] || { pos_say 'a 3-behind tree installed anyway'; return 1; }
	n=$(printf '%s' "$pos_out" | grep -cF 'this tree IS the supervisor' || true)
	[ "$n" = 0 ] || {
		pos_say "the agreeing sentence still printed with a divergent unit ($n):"; pos_say "$pos_out"; return 1; }
	printf '%s' "$pos_out" | grep -qiE 'loop runs today' || {
		pos_say 'the divergent case did not take over the sentence:'; pos_say "$pos_out"; return 1; }
	return 0
}
claim_leg 'an agreeing unit and tree are stated once' install_agreeing_unit_says_so_once

# The rule H-0056 buys against `H-0013` (5min2s of a pass lost to one read) and `#879` (a blocking
# read that fails open by parking the loop): the verdict is a reading of `.git`, never a network call.
# A `fetch` here would be invisible in every other leg, because a fixture's trunk is a local path and
# a fetch against it succeeds -- so the pin is on the text, and the control deletes the word from the
# function's own body in a copy outside the repository rather than trusting that nothing happens to
# mention it today.
no_fetch_in_the_verdict() { # -> 0 when the shipped verdict asks git nothing that can block
	local body code control
	body=$(awk '/^factory_supervisor_position\(\)/,/^}/' "$HERE/common.sh")
	[ -n "$body" ] || { pos_say "no factory_supervisor_position in $HERE/common.sh -- the scan has nothing to read"; return 1; }
	# Comments are stripped first: the function's own prose is full of the word it must not execute,
	# and a scan that reads comments would report the explanation of the rule as a violation of it.
	code=$(printf '%s\n' "$body" | sed 's/#.*//')
	printf '%s\n' "$code" | grep -qE '\b(fetch|ls-remote|pull)\b' && {
		pos_say 'the verdict makes a network call; it runs inside a pass on a budget'; return 1; }
	# The control, planted outside the repository: a fetch written where the scan looks, in the copy of
	# the body the scan was just run over. If the scan cannot see that, the ok above is blindness.
	control=$pos_work/no-fetch-control
	mkdir -p "$control"
	printf '%s\n' "$body" | sed '0,/return 0/s//git fetch origin \/dev\/null; return 0/' >"$control/fn.sh"
	grep -q 'git fetch' "$control/fn.sh" || {
		pos_say 'the control could not plant a fetch, so the scan above proves nothing'; return 1; }
	printf '%s\n' "$(sed 's/#.*//' "$control/fn.sh")" | grep -qE '\b(fetch|ls-remote|pull)\b' || {
		pos_say 'the scan does not see a planted fetch -- it is blind, not clean'; return 1; }
	# Why a leg cannot catch this by behaviour alone: the fixture trunk is a local path, so a fetch
	# there would succeed in milliseconds and every leg would stay green. Pinning the path is what
	# makes the text scan the load-bearing half rather than a decoration beside a passing test.
	printf '%s\n' "$(git -C "$pos_fork" remote get-url origin)" | grep -q '^/' || {
		pos_say 'the fixture trunk is not a local path, so a network call here would not be cheap -- and this leg would be the wrong shape'; return 1; }
	return 0
}
claim_leg 'the verdict never fetches' no_fetch_in_the_verdict

# --- the fixture's own independence from the box ---------------------------------------------
#
# H-0059 is the reason this scan exists: a fixture that commits under whatever identity the host
# happens to have is green on every workstation in the loop and red on CI, and it was invisible until
# it was. A leg cannot re-run this suite under an empty `$HOME` -- that is a second copy of a
# fifty-second run inside itself -- so the pin is on the creation sites, the way `wired_into_verify`
# pins its step and `no_fetch_in_the_verdict` pins its word: scan the shipped file, then prove the
# scan is not blind by deleting one identity from a copy outside the repository and watching it
# report the gap.
#
# The rule is per repository, not per environment, because `git clone` copies the upstream's remote
# and none of its local config: identity set on the trunk says nothing about a clone of it. A site
# counts as configured when an identity appears on the creation line or within three lines of it --
# `git -c user.email=… commit` on the next line is as host-independent as a `config` call.
pos_unconfigured_repos() { # <file> -> every `git init`/`git clone` with no identity within 3 lines
	# No `\b` in here: awk reads that escape as a backspace, not a word boundary, and a scan built
	# out of it matches nothing at all -- which is exactly what the first version of this function
	# did, and the control below is what said so on the first run.
	awk '
		{ l = $0; sub(/^[ \t]+/, "", l) }
		l ~ /^#/ { next }
		l ~ /^git / && (l ~ / init/ || l ~ / clone/) {
			if (pend) { printf "%d: %s\n", pend, ptxt }
			pend = NR; ptxt = l
		}
		pend && NR > pend + 3 { printf "%d: %s\n", pend, ptxt; pend = 0 }
		pend && l ~ /user\.email/ && NR <= pend + 3 { pend = 0 }
		END { if (pend) printf "%d: %s\n", pend, ptxt }
	' "$1"
}
fixture_carries_identity() {
	local gaps control
	gaps=$(pos_unconfigured_repos "$HERE/selftest.sh")
	[ -z "$gaps" ] || {
		pos_say 'these fixture repositories inherit the host'"'"'s git identity, and a runner'
		pos_say 'has none (H-0059: `Author identity unknown` kills the run mid-suite):'
		pos_say "$gaps"
		return 1; }
	# The control, planted outside the repository: the identity deleted from the clone helper, in the
	# copy the scan was just run over clean. A scan that still says nothing is blind, not clean.
	control=$pos_work/identity-control
	mkdir -p "$control"
	sed '/git -C "\$path" config user\.email/d' "$HERE/selftest.sh" >"$control/selftest.sh"
	grep -q 'git clone' "$control/selftest.sh" || {
		pos_say 'the control copy has no clone in it, so it tests nothing'; return 1; }
	grep -q 'user\.email' "$control/selftest.sh" || {
		pos_say 'the control deleted every identity line, not one'; return 1; }
	pos_unconfigured_repos "$control/selftest.sh" | grep -q 'git clone' || {
		pos_say 'the control removed an identity and the scan still reported nothing'; return 1; }
	return 0
}
claim_leg 'no fixture repo inherits the host' fixture_carries_identity

# A run that dies before its tally leaves no tally behind. That is what H-0059 cost: the CI log ended
# at git's fatal with no number in it, and the only way to find where 217 legs stopped being 236 was
# to re-run the suite with the box's identity taken away. The trap's own text is extracted from the
# shipped file rather than pasted here, so the leg runs what a run runs, and the two scripts are the
# only states there are to know about: a death before the verdict, and a run that reaches it. Either
# half alone is a grep -- the death case without the clean one proves only that the word is printed
# for everything.
incomplete_reports_itself() {
	local box=$work/incomplete-trap fn out rc
	mkdir -p "$box/tree"
	fn=$(awk '/^cleanup\(\)/,/^}/' "$HERE/selftest.sh")
	[ -n "$fn" ] || { pos_say 'no cleanup() in the shipped file, so the leg would be testing a copy of it'; return 1; }
	printf '%s\n' "$fn" >"$box/cleanup.sh"
	grep -q 'RUN INCOMPLETE' "$box/cleanup.sh" || {
		pos_say 'the shipped trap carries no incomplete-run notice, so there is nothing to run:'
		pos_say "$fn"
		return 1; }

	# The trap, installed over a run that counts one leg and then fails under `set -e`. The verdict
	# line stays in the script after the death, where only a run that does not die can reach it -- so
	# "the verdict was not printed" is a fact about the death, not about a missing line.
	{
		printf 'set -euo pipefail\npass=7\nfail=2\nreached_verdict=0\nwork=%q\n' "$box/tree"
		cat "$box/cleanup.sh"
		printf 'trap cleanup EXIT INT TERM\n'
	} >"$box/head.sh"
	{
		cat "$box/head.sh"
		printf 'false\n'
		printf 'printf "factory/selftest.sh: %%d passed, %%d failed\\n" "$pass" "$fail"\n'
	} >"$box/death.sh"
	{
		cat "$box/head.sh"
		printf 'printf "factory/selftest.sh: %%d passed, %%d failed\\n" "$pass" "$fail"\n'
		printf 'reached_verdict=1\n'
	} >"$box/finish.sh"

	rc=0
	out=$(bash "$box/death.sh" 2>&1) || rc=$?
	[ "$rc" -ne 0 ] || { pos_say 'the planted death exited 0 -- nothing failed, so the case proves nothing'; return 1; }
	printf '%s\n' "$out" | grep -q 'RUN INCOMPLETE' || {
		pos_say 'a run that died before its verdict said nothing about it:'; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -q 'after 7 ok and 2 FAIL' || {
		pos_say 'the notice does not say how far the run got, which is the whole reason for it:'
		pos_say "$out"
		return 1; }
	printf '%s\n' "$out" | grep -q 'factory/selftest.sh: 7 passed, 2 failed' && {
		pos_say 'the run reached the verdict line after the planted death, so the case is not a death:'
		pos_say "$out"
		return 1; }

	mkdir -p "$box/tree"
	rc=0
	out=$(bash "$box/finish.sh" 2>&1) || rc=$?
	[ "$rc" -eq 0 ] || { pos_say "the clean run exited $rc:"; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -q 'factory/selftest.sh: 7 passed, 2 failed' || {
		pos_say 'the clean run did not print its verdict:'; pos_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -q 'RUN INCOMPLETE' && {
		pos_say 'a run that printed its verdict also said it had not:'; pos_say "$out"; return 1; }
	# The notice is an addition to the trap, not a replacement for it: the fixture still has to go.
	[ ! -e "$box/tree" ] || { pos_say 'the trap stopped cleaning up the fixture'; return 1; }
	return 0
}
claim_leg 'a death before the verdict is reported' incomplete_reports_itself

# ---------------------------------------------------------------- the unit's two deadlines (#246)
#
# `TimeoutStopSec` and `TimeoutStartSec` were one token, and on 8 Nov that coupling took the queue
# offline for 1 h 07 min: a tick's leaked Wine grandchild sat in the unit's cgroup ignoring SIGTERM,
# `KillMode=control-group` will not call the cgroup empty while one process lives, systemd waited out
# the 5460-second stop deadline before escalating to SIGKILL, and `OnUnitInactiveSec=` -- the setting
# that re-arms the queue -- starts counting only when the unit finishes going inactive. The unit sat at
# `SubState=stop-sigterm MainPID=0` with the timer still `active`. The fix is two numbers (H-0066).
#
# What is pinned here is the unit the generator *renders*, not the text of its template. The defect was
# in a substitution, and a check that greps a template can be satisfied by a template that renders
# something else: there are three ways to re-merge these two numbers that all leave the template
# looking right -- point the stop token back at `@STOP@`, point the start token at the stop token, or
# delete the substitution line and ship a unit carrying a literal `@STOP_TIMEOUT@`. So every leg below
# runs `install-timer.sh --install --dry-run`, which writes nothing and prints both units, and reads
# the values out of what it printed. `install-timer.sh` is the only thing that knows what a token
# becomes.
#
# Two stub binaries are on that one invocation's PATH because the generator refuses to start without
# `gh` and `opencode` on it, and this gate has no business needing a GitHub token or the CLI: `--dry-run`
# exits before the first systemctl call, so neither stub is ever asked to do anything. Everything the
# run could write is inside `$work` -- `FACTORY_ROOT` and `FACTORY_DIR` both name the copy, which is
# what `factory_paths`' three mkdirs follow -- so the tree this gate is checking stays untouched.
#
# The copy is a repository with a commit and `origin/main` at that commit, and that is not decoration:
# #249 made `install-timer.sh` refuse to render a tree that is not its own trunk, and it refuses before
# it prints anything. A fixture that fails that test produces no unit at all, so the section's first leg
# asks the generator whether it accepted the fixture at all, and a refusal is a FAIL rather than a SKIP
# -- six quiet SKIPs under a tally that reads `passed` is a gate that has stopped looking (H-0069).
#
# The bound on the stop deadline is asserted as a number rather than as a name because the number is
# the outage: a descendant that ignores SIGTERM holds the factory for exactly `TimeoutStopSec` seconds,
# and the value that stood here was 5460 for as long as nobody measured what it cost.
gen_stub=$work/stubbin
gen_root=$work/genrepo
# Which fixture the generator renders against. Every leg uses the armed one; only the control that
# asks whether a #249 refusal is real points it at the unarmed one, and it puts this back.
gen_tree=$gen_root
gen_out=$work/gen-out
gen_err=$work/gen-err
gen_svc=$work/gen-service
gen_tim=$work/gen-timer
# The ceiling a leaked descendant may hold the queue for, in seconds. Twice the shipped value: loose
# enough that a deliberate, argued change is possible, tight enough that re-deriving this line from
# the pass budget -- which is the defect -- cannot pass.
gen_stop_max=120

gen_setup() { # <dir> <arm: 1|0> -> the throwaway repo at <dir>; 0 leaves it a tree #249 will not arm
	local root=$1 arm=$2
	mkdir -p -- "$gen_stub" "$root/factory" || return 1
	# shellcheck disable=SC2317  # the stub is run by the generator, not by this file
	cat >"$gen_stub/gh" <<'STUB'
#!/bin/sh
# A stub. `install-timer.sh --install --dry-run` asks gh for its auth state and a login and nothing
# else: it exits before the first systemctl call, so nothing written here can reach GitHub. Its only
# job is to satisfy the two checks a real install would pass, so that this gate needs no token.
case "$1 $2" in
"auth status") exit 0 ;;
esac
printf 'stub-login\n'
exit 0
STUB
	printf '#!/bin/sh\nexit 0\n' >"$gen_stub/opencode" || return 1
	chmod +x -- "$gen_stub/gh" "$gen_stub/opencode" || return 1
	cp -- "$HERE/common.sh" "$HERE/factory.sh" "$root/factory/" || return 1
	chmod +x -- "$root/factory/factory.sh" || return 1
	# Identity is planted where the repository is created, the way every other fixture repository in
	# this file does it (H-0059): a CI runner has no `~/.gitconfig`, and the commit below dies there
	# with `Author identity unknown` and no verdict printed. `no fixture repo inherits the host` is
	# the leg that refuses a creation site which forgets this, and it refused this one -- the leg
	# arrived with `origin/main`, this section did not, and the merge that joined them is where the
	# two first met (H-0069).
	git init -q -- "$root" >/dev/null 2>&1 || return 1
	git -C "$root" config user.email fixture@invalid || return 1
	git -C "$root" config user.name fixture || return 1
	git -C "$root" remote add origin https://github.com/plock/plock.git >/dev/null 2>&1
	# A commit, and `origin/main` at that same commit, because #249 put a precondition in front of the
	# whole render: `install-timer.sh` asks `factory_supervisor_position "$FACTORY_ROOT"` before it
	# prints a single line and exits 1 on a tree that is not its own trunk -- with no commit it reads
	# "not a readable git checkout", with no trunk ref "cannot be resolved". Either way there is no
	# rendered unit, every leg below reads an empty file, and the tally still says `passed`. Measured
	# on the merge of origin/main at 3b56a05: six legs became six SKIPs that way. `arm=0` leaves both
	# out, which is what the control at the end of the section aims the predicate at.
	if [ "$arm" != 1 ]; then
		return 0
	fi
	git -C "$root" commit -q --allow-empty -m 'fixture: the trunk this tree is armable against' ||
		return 1
	git -C "$root" update-ref refs/remotes/origin/main HEAD
}

# `--dry-run` of one generator, with any number of VAR=value overrides. It renders against whatever
# `$gen_tree` names -- the armed fixture for every leg, the unarmed one for the control that asks
# whether a tree #249 refuses is refused. Stdout and stderr are left in $gen_out and $gen_err for the
# caller to read; the exit status is the generator's.
gen_dry_run() { # <generator> [VAR=value...]
	local gen=$1
	shift
	env PATH="$gen_stub:$PATH" \
		FACTORY_ROOT="$gen_tree" \
		FACTORY_DIR="$gen_tree/factory" \
		FACTORY_REPO='plock/plock' \
		FACTORY_REVIEWER='reviewer-other' \
		"$@" bash "$gen" --install --dry-run >"$gen_out" 2>"$gen_err"
}

# The two units, split out of the dry run's output by the `DRY-RUN  write <path>` lines that head them.
gen_split() {
	awk '/^DRY-RUN  write / { f = ($0 ~ /\.service$/); next } f' "$gen_out" >"$gen_svc"
	awk '/^DRY-RUN  write / { f = ($0 ~ /\.timer$/); next } f' "$gen_out" >"$gen_tim"
}

deadline_value() { # <unit file> <KEY> -> the value on the un-commented `KEY=` line
	awk -v k="$2" 'index($0, k "=") == 1 { print substr($0, length(k) + 2); exit }' "$1"
}

is_a_number() {
	case "$1" in
	'' | *[!0-9]*) return 1 ;;
	*) return 0 ;;
	esac
}

# The precondition #249 put in front of the entire render, asserted before anything reads a unit.
# A fixture the generator will not arm produces no output at all, and every leg below would then be
# reading an empty file -- which is a check that has stopped looking, not a check that passed. Status
# 2 (SKIP) is deliberately not on offer here: the subject is in this tree whether or not the
# generator agreed to render it. Measured on the merge of origin/main at 3b56a05 -- six SKIPs, tally
# still `passed`, because #249's arm arrived on the trunk side of that merge and this section's
# fixture predated it.
# shellcheck disable=SC2317  # reached only as the predicate argument of gen_leg
leg_generator_accepts_the_fixture() { # <the dry run's exit status>
	if [ "$1" = 0 ]; then
		return 0
	fi
	printf '            install-timer.sh --install --dry-run exited %s on the fixture, so no unit was\n' "$1" >&2
	printf '            rendered and there is nothing for the deadline legs to read. It is refusing\n' >&2
	printf '            its own #249 arm: the fixture is not a tree the generator would arm, which\n' >&2
	printf '            means a commit whose own origin/main is that same commit (gen_setup, arm=1).\n' >&2
	return 1
}

# FACTORY_BUDGET as `common.sh` states it, not as a literal repeated here: a leg that hardcodes 5400
# keeps passing the day somebody moves the default, and then it is measuring a number that no longer
# exists. Empty when the file cannot be read, which every caller treats as "this leg has no subject".
budget_default() {
	sed -n 's/^FACTORY_BUDGET=[^0-9]*\([0-9][0-9]*\)}.*/\1/p' "$HERE/common.sh" |
		head -n 1
}

# Each predicate reads the rendered unit already in $gen_svc (and $gen_tim), prints why it refused, and
# returns non-zero. Split out this way so a control can aim the same predicate at a planted generator's
# output instead of the shipped one.
# shellcheck disable=SC2317  # reached only as the predicate argument of gen_leg / gen_control
leg_both_deadlines_rendered() {
	local stop start
	stop=$(deadline_value "$gen_svc" TimeoutStopSec)
	start=$(deadline_value "$gen_svc" TimeoutStartSec)
	if is_a_number "$stop" && is_a_number "$start"; then
		return 0
	fi
	printf '            the service unit holds TimeoutStopSec=%s and TimeoutStartSec=%s;\n' \
		"${stop:-<absent>}" "${start:-<absent>}" >&2
	printf '            both have to be plain numbers or systemd falls back to its own defaults.\n' >&2
	return 1
}

# shellcheck disable=SC2317  # reached only as the predicate argument of gen_leg / gen_control
leg_deadlines_differ() {
	local stop start
	stop=$(deadline_value "$gen_svc" TimeoutStopSec)
	start=$(deadline_value "$gen_svc" TimeoutStartSec)
	if [ -n "$stop" ] && [ "$stop" != "$start" ]; then
		return 0
	fi
	printf '            both deadlines are %s s. One token serves both again, which is the defect:\n' \
		"${stop:-<absent>}" >&2
	printf '            one stubborn grandchild then holds the queue for the whole pass budget, and\n' >&2
	printf '            OnUnitInactiveSec= cannot re-arm until it is done (issue #246, 67 minutes).\n' >&2
	return 1
}

# shellcheck disable=SC2317  # reached only as the predicate argument of gen_leg / gen_control
leg_start_clears_budget() {
	local start budget
	start=$(deadline_value "$gen_svc" TimeoutStartSec)
	budget=$(budget_default)
	if [ -z "$budget" ]; then
		printf '            FACTORY_BUDGET could not be read out of factory/common.sh\n' >&2
		return 2
	fi
	if is_a_number "$start" && [ "$start" -ge "$budget" ]; then
		return 0
	fi
	printf '            TimeoutStartSec=%s but the pass ceiling is FACTORY_BUDGET=%s. A start deadline\n' \
		"${start:-<absent>}" "$budget" >&2
	printf '            below the budget kills a live tick mid-commit, and a tick killed mid-commit\n' >&2
	printf '            leaves a dirty worktree the next pass halts on. Splitting the two deadlines is\n' >&2
	printf '            about STOP, never about this one.\n' >&2
	return 1
}

# The two numbers have to be derived from different things, not merely happen to differ today. Start
# follows the budget; stop must not move when the budget does. Rendered twice, so a literal that
# satisfies both legs by accident cannot pass this one.
# shellcheck disable=SC2317  # reached only as the predicate argument of gen_leg / gen_control
leg_only_start_follows_the_budget() { # <budget>
	local budget start stop base_stop
	budget=$1
	start=$(deadline_value "$gen_svc" TimeoutStartSec)
	stop=$(deadline_value "$gen_svc" TimeoutStopSec)
	base_stop=$(deadline_value "$work/base-service" TimeoutStopSec)
	if is_a_number "$start" && [ "$start" -ge "$budget" ] &&
		[ -n "$stop" ] && [ "$stop" = "$base_stop" ] && [ "$start" != "$base_stop" ]; then
		return 0
	fi
	printf '            at FACTORY_BUDGET=%s the unit rendered TimeoutStartSec=%s TimeoutStopSec=%s;\n' \
		"$budget" "${start:-<absent>}" "${stop:-<absent>}" >&2
	printf '            start was meant to clear %s, and stop was meant to stay at %s -- the value it\n' \
		"$budget" "${base_stop:-<absent>}" >&2
	printf '            has at every budget since #246. A stop deadline that tracks the budget is the\n' >&2
	printf '            91-minute outage back with a different default.\n' >&2
	return 1
}

# shellcheck disable=SC2317  # reached only as the predicate argument of gen_leg / gen_control
leg_stop_is_a_reaping() {
	local stop
	stop=$(deadline_value "$gen_svc" TimeoutStopSec)
	if is_a_number "$stop" && [ "$stop" -ge 1 ] && [ "$stop" -le "$gen_stop_max" ]; then
		return 0
	fi
	printf '            TimeoutStopSec=%s. Nothing legitimate runs during stop, so this number is\n' \
		"${stop:-<absent>}" >&2
	printf '            only how long one descendant that ignores SIGTERM can hold the queue; it has\n' >&2
	printf '            to stay at most %s s, and it was 5460 s the night the queue went silent.\n' \
		"$gen_stop_max" >&2
	return 1
}

# shellcheck disable=SC2317  # reached only as the predicate argument of gen_leg / gen_control
leg_no_token_left_unrendered() {
	local left
	left=$(grep -hohE '@[A-Z_]+@' "$gen_svc" "$gen_tim" 2>/dev/null |
		sort -u | paste -sd' ' - || true)
	if [ -z "$left" ]; then
		return 0
	fi
	printf '            the rendered units still hold %s: a token with no substitution line in\n' "$left" >&2
	printf '            render(). systemd does not reject the line -- it reads a garbage deadline, or\n' >&2
	printf '            no deadline at all, and a oneshot with no deadline never stops by itself.\n' >&2
	return 1
}

# One leg, in the file's shape: ok, or FAIL with the predicate's own words underneath. Status 2 is a
# missing subject, which is a SKIP -- the same bargain as `wired_into_verify` refusing to guess about
# a Makefile that is not there.
gen_leg() { # <label> <what an ok means here> <predicate> [args...]
	local label=$1 fact=$2 rc=0
	shift 2
	# `"$@" && { … }`, not `if "$@"`: an `if` with no `else` returns 0 when the predicate refuses, so
	# `$?` afterwards is the `if`'s status rather than the predicate's -- and status 2, "the subject is
	# not in this tree", would then be reported as a verdict on the unit.
	"$@" && {
		pass=$((pass + 1))
		printf '  ok    %-40s %s\n' "$label" "$fact"
		return 0
	}
	rc=$?
	if [ "$rc" = 2 ]; then
		printf '  SKIP  %-40s the subject is not in this tree\n' "$label"
		return 0
	fi
	fail=$((fail + 1))
	printf '  FAIL  %-40s the check refused:\n' "$label"
	return 0
}

# A generator planted in `$work`: the shipped file, with one thing done to it that the shipped file no
# longer does. `FACTORY_ROOT` stays the one throwaway repo, because a copy is only ever asked to print
# a unit -- the repo is what the generator checks, not what it reads.
gen_planted() { # <dir> <sed expression> -> the planted generator's path
	local d=$1 expr=$2
	mkdir -p -- "$d" || return 1
	cp -- "$HERE/install-timer.sh" "$HERE/common.sh" "$d/" || return 1
	sed "$expr" "$d/install-timer.sh" >"$d/install-timer.new" || return 1
	mv -- "$d/install-timer.new" "$d/install-timer.sh" || return 1
	printf '%s' "$d/install-timer.sh"
}

# One control: the predicate that passes on the shipped unit has to refuse the planted one. A control
# that still says yes means the ok above is this file certifying itself out of the goodness of a
# pattern, which is the only way a green gate can coexist with the bug it was written for.
gen_control() { # <label> <dir> <sed expression> <predicate> [args...]
	local label=$1 dir=$2 expr=$3 pred=$4 bad
	shift 4
	bad=$(gen_planted "$dir" "$expr" || true)
	if [ -z "$bad" ]; then
		printf '  SKIP  %-40s the plant could not be written\n' "control: $label"
		return 0
	fi
	if ! gen_dry_run "$bad"; then
		printf '  SKIP  %-40s the planted generator would not run\n' "control: $label"
		return 0
	fi
	gen_split
	# The predicate's own refusal text is about a unit nobody is shipping, so it goes nowhere: a control
	# passes by refusing, and printing the refusal would read as a failed leg.
	if "$pred" "$@" 2>/dev/null; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it accepted the planted generator, so the ok\n' "control: $label"
		printf '          above is the check being blind, not the check working.\n'
		return 0
	fi
	printf '  ok    %-40s the same check refuses it\n' "control: $label"
	return 0
}

# The control for the armable leg, and it aims at the fixture rather than at the generator: the same
# predicate has to refuse a tree #249 will not arm. Built by the very same gen_setup one argument
# away from armable, so what gets refused is the missing precondition and not a paste of the setup
# that happens to be broken -- the reason H-0060's leg extracts the shipped cleanup() rather than
# quoting it. `$gen_tree` goes back before this returns, because every leg after it reads the armable
# fixture again.
gen_control_unarmed() {
	local bad_root=$work/genrepo-unarmed rc=0
	if ! gen_setup "$bad_root" 0; then
		printf '  SKIP  %-40s the unarmed fixture could not be built\n' \
			'control: a fixture with no trunk ref'
		return 0
	fi
	gen_tree=$bad_root
	gen_dry_run "$HERE/install-timer.sh" || rc=$?
	gen_tree=$gen_root
	if [ "$rc" = 0 ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it rendered a unit for a tree with neither a commit\n' \
			'control: a fixture with no trunk ref'
		printf '          nor a trunk ref, so the ok above is not #249%s refusal being read.\n' "'s"
		return 0
	fi
	if ! leg_generator_accepts_the_fixture "$rc" 2>/dev/null; then
		printf '  ok    %-40s the same check refuses it\n' \
			'control: a fixture with no trunk ref'
		return 0
	fi
	fail=$((fail + 1))
	printf '  FAIL  %-40s the predicate accepted it, so the ok above\n' \
		'control: a fixture with no trunk ref'
	printf '          is the check being blind, not the check working.\n'
	return 0
}

unit_deadlines() {
	local shown_stop shown_start budget2 rc=0
	if [ ! -f "$HERE/install-timer.sh" ]; then
		printf '  SKIP  %-40s install-timer.sh is not in this tree\n' "the unit's two deadlines"
		return 0
	fi
	if ! gen_setup "$gen_root" 1; then
		printf '  SKIP  %-40s the throwaway generator could not be built in %s\n' \
			"the unit's two deadlines" "$work"
		return 0
	fi
	gen_tree=$gen_root
	gen_dry_run "$HERE/install-timer.sh" || rc=$?
	gen_leg 'the fixture is a tree the generator arms' \
		'--install --dry-run exits 0 on it' \
		leg_generator_accepts_the_fixture "$rc"
	if [ "$rc" -ne 0 ]; then
		printf '          what the generator said, in its own words:\n'
		sed -n '1,3p' "$gen_err" | sed 's/^/          /'
		printf '          Every leg below reads the rendered unit, so none of them is reported -- and\n'
		printf '          this one is a FAIL, not a SKIP. The subject is in this tree; the gate has\n'
		printf '          simply stopped seeing it, which is the failure #246 is about wearing a gate:\n'
		printf '          six quiet SKIPs and a tally that still reads `passed` is a check that looks\n'
		printf '          like it is running. Measured on the merge of origin/main, 9 Nov.\n'
		gen_control_unarmed
		return 0
	fi
	gen_split
	# The stop deadline as shipped, kept for the leg that renders a second budget and asks which of
	# the two numbers moved.
	cp -- "$gen_svc" "$work/base-service"
	shown_stop=$(deadline_value "$gen_svc" TimeoutStopSec)
	shown_start=$(deadline_value "$gen_svc" TimeoutStartSec)

	gen_leg 'both deadlines are rendered' \
		'a TimeoutStopSec and a TimeoutStartSec, both plain numbers' \
		leg_both_deadlines_rendered
	gen_leg 'stop and start are different numbers' \
		"stop=${shown_stop}s is not start=${shown_start}s" \
		leg_deadlines_differ
	gen_leg 'start still clears FACTORY_BUDGET' \
		"${shown_start}s clears the pass ceiling" \
		leg_start_clears_budget
	gen_leg 'stop is a reaping, not a ceiling' \
		"${shown_stop}s, at most ${gen_stop_max}s" \
		leg_stop_is_a_reaping
	gen_leg 'every @TOKEN@ was substituted' \
		'no @TOKEN@ left in either rendered unit' \
		leg_no_token_left_unrendered

	# Rendered a second time, at a budget nobody hardcoded a value for. One render can satisfy "the
	# two numbers differ" with two literals that happen to differ; only a second budget shows whether
	# start is *derived* from it and stop is not.
	budget2=9000
	if gen_dry_run "$HERE/install-timer.sh" "FACTORY_BUDGET=$budget2"; then
		gen_split
		gen_leg "start follows FACTORY_BUDGET=$budget2" \
			"start moved, stop stayed at ${shown_stop}s" \
			leg_only_start_follows_the_budget "$budget2"
	else
		# The fixture was accepted a moment ago at the default budget, so a refusal here is not the
		# subject being absent -- it is this leg losing its subject. Same bargain as above.
		fail=$((fail + 1))
		printf '  FAIL  %-40s the generator refused a budget it had just\n' \
			'start follows the budget'
		printf '          accepted, so nothing was rendered at %s and the leg cannot say\n' "$budget2"
		printf '          which of the two numbers moved: %s\n' "$(sed -n '1,1p' "$gen_err")"
		return 0
	fi

	printf '        TimeoutStopSec=%s TimeoutStartSec=%s, from the generator in this tree -- the stop\n' \
		"$shown_stop" "$shown_start"
	printf '        deadline is how long one leaked grandchild can hold the queue. #246: was 5460.\n'

	gen_control 'stop re-coupled to the budget token' "$work/ctrl-coupled" \
		's/^TimeoutStopSec=@STOP_TIMEOUT@$/TimeoutStopSec=@STOP@/' leg_deadlines_differ
	gen_control 'the stop token left unsubstituted' "$work/ctrl-unsub" \
		'/^[[:space:]]*-e "s#@STOP_TIMEOUT@#/d' leg_no_token_left_unrendered
	gen_control 'start re-pointed at the stop token' "$work/ctrl-shrunk" \
		's/^TimeoutStartSec=@STOP@$/TimeoutStartSec=@STOP_TIMEOUT@/' leg_start_clears_budget
	# The same regression wearing the clothes a person would actually write it in: somebody takes the
	# complaint that 60 s is short for a slow teardown and re-derives the value from the budget. Both
	# numbers are still different at the default budget, so only the two legs that ask what each one is
	# *made of* can see it -- which is why both of them get a plant of their own.
	gen_control 'stop re-derived from the budget' "$work/ctrl-budget-a" \
		's/^STOP_TIMEOUT=60$/STOP_TIMEOUT=$((FACTORY_BUDGET + 60))/' \
		leg_only_start_follows_the_budget 5400
	gen_control 'stop re-derived from the budget' "$work/ctrl-budget-b" \
		's/^STOP_TIMEOUT=60$/STOP_TIMEOUT=$((FACTORY_BUDGET + 60))/' leg_stop_is_a_reaping
	gen_control 'both deadline lines gone' "$work/ctrl-gone" \
		'/^TimeoutStopSec=@/d; /^TimeoutStartSec=@/d' leg_both_deadlines_rendered
	# Last, because it borrows `$gen_tree` and hands it back: the fixture itself gets a plant, aimed
	# at the one thing every leg above depends on.
	gen_control_unarmed
	return 0
}
unit_deadlines

# ------------------------------------------------------------ decision ids across refs (#253)
#
# The allocator used to be prose: "number from the highest id in the file you are writing to." Three
# branches cut from the same trunk followed that sentence literally, saw the same H-0060, and all
# wrote H-0061. H-0065, H-0066 and H-0068 later had to move. The sentence is not wrong about the
# trunk; it is wrong about the concurrency. A decision id is a durable citation and a merge target,
# so an id that exists on an open sibling is already taken even when this checkout has never seen it.
#
# What gets pinned:
#
#   the union      a local branch and a remote-tracking branch each add one id. The current file stays
#                  at the trunk value, so a result above it can only come from a ref read.
#   the collision  add the allocator's own next answer to a second remote-only ref. The next call has
#                  to refuse that id and move one further -- this is the H-0061 race, made local.
#   the prefix     DECISIONS.md and factory/DECISIONS.md share the allocator without sharing a line
#                  count, which is what H-0068 had to repair by hand.
#   the ceiling    D-0002 is deliberately absent. The allocator returns D-0004, not D-0002: retired
#                  and renumbered ids are permanent reservations, not vacancies.
#   the current tree an uncommitted entry has to count before the commit exists, because the allocator
#                  is called immediately before the entry is written.
#   the worktree   a sibling unit's committed entry in a second worktree of the same clone is seen with
#                  no fetch. Every unit tree this harness makes is a worktree, so this is the case the
#                  skill's "re-read immediately before the entry commit" actually buys.
#   the boundary   a sibling pushed from another clone is NOT seen until this clone fetches. Measured
#                  both ways so the docs cannot overstate the re-read (H-0071), and the leg asserts the
#                  fetched half too: an answer that never moves would pass the unfetched half for free.
#   the offline read it reads refs already on the machine and fetches nothing. A blocking network read
#                  in a per-tick budget is a hazard this harness already paid for (H-0013, H-0056).
#   the gate       `scripts/check-decisions.py` asks git about objects and never about refs. The
#                  allocator may read refs because it only chooses a number; the gate's verdict has to
#                  be identical in CI, in the review loop and at the merge, so it cannot depend on
#                  what this checkout last fetched (#253's second instruction, H-0072). Pinned twice:
#                  a scan for ref-shaped git calls, and the verdict itself on a fixture whose sibling
#                  branch repeats an id.
#
# Both controls plant a narrower read than the shipped allocator has. If either control still returned
# the correct answer, the ok above would be proof of nothing: the test would pass because the fixture
# happens to agree, not because the allocator looked at the branch it was written for.

alloc_script=$HERE/next-decision-id.sh
alloc_work=$work/decision-id
alloc_err=$work/alloc-stderr

# shellcheck disable=SC2317  # reached through the command substitution in alloc_leg
alloc_run() { # <repo> <log path> <prefix> -> the allocator's stdout
	# An empty prefix means "do not pass --prefix at all", which is the only way to reach the arm that
	# infers the prefix from the log's own path. Every leg written before issue #253 passed a prefix,
	# so that arm had never run in the suite -- and it was the arm that died silently.
	if [ -n "${3:-}" ]; then
		bash "$alloc_script" --repo "$1" --prefix "$3" "$2" 2>"$alloc_err"
	else
		bash "$alloc_script" --repo "$1" "$2" 2>"$alloc_err"
	fi
}

# shellcheck disable=SC2317  # reached through the command substitution in alloc_leg
alloc_leg() { # <label> <repo> <log path> <prefix> <expected id> <what an ok proves>
	local label=$1 repo=$2 path=$3 prefix=$4 expected=$5 fact=$6 got rc=0
	if ! got=$(alloc_run "$repo" "$path" "$prefix"); then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the allocator refused:\n' "$label"
		sed 's/^/          /' "$alloc_err"
		return 0
	fi
	if [ "$got" = "$expected" ]; then
		pass=$((pass + 1))
		printf '  ok    %-40s %s\n' "$label" "$fact"
		return 0
	fi
	fail=$((fail + 1))
	printf '  FAIL  %-40s got %s, expected %s.\n' "$label" "$got" "$expected"
	printf '          The fixture is deliberately a ceiling above a lower gap: an allocator that\n'
	printf '          recycles missing numbers, reads only the current file, or misses one ref\n'
	printf '          returns the lower id here. %s\n' "$fact"
	return 0
}

alloc_plant() { # <dir> <sed expr> -> a copy of the shipped allocator with one thing taken out
	local dir=$1 expr=$2
	mkdir -p -- "$dir" || return 1
	if ! sed "$expr" "$alloc_script" >"$dir/new"; then
		return 1
	fi
	cmp -s "$dir/new" "$alloc_script" && return 1
	mv -- "$dir/new" "$dir/next-decision-id.sh" || return 1
	printf '%s' "$dir/next-decision-id.sh"
}

# The shipped allocator returns the right answer. A control passes only when the same call, aimed at
# the same fixture with one ref class missing, returns the older answer -- the one the bug would give.
# If it still returns the right one, the plant did not narrow this allocator and the ok is the test
# certifying its own fixture.
alloc_control() { # <label> <dir> <sed expr> <repo> <path> <prefix> <wrong answer> <what it missed>
	local label=$1 dir=$2 expr=$3 repo=$4 path=$5 prefix=$6 wrong=$7 fact=$8 bad got rc=0
	bad=$(alloc_plant "$dir" "$expr" || true)
	if [ -z "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant could not be written or did\n' "control: $label"
		printf '          nothing change; the check has stopped pinning the read it names.\n'
		return 0
	fi
	if ! got=$(
		if [ -n "$prefix" ]; then
			bash "$bad" --repo "$repo" --prefix "$prefix" "$path" 2>"$alloc_err"
		else
			bash "$bad" --repo "$repo" "$path" 2>"$alloc_err"
		fi
	); then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the planted allocator refused:\n' "control: $label"
		sed 's/^/          /' "$alloc_err"
		return 0
	fi
	if [ "$got" != "$wrong" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the narrowed allocator still said %s,\n' "control: $label" "$got"
		printf '          not the %s its missing read would produce. %s\n' "$wrong" "$fact"
		return 0
	fi
	printf '  ok    %-40s it falls back to %s, so the ok above is the\n' \
		"control: $label" "$wrong"
	printf '          ref being read, not the fixture agreeing by accident.\n'
	return 0
}

# A refusal has three parts, and the allocator shipped delivering none of them: a mistyped log path
# exited 0 with `H-0001` on stdout, and the same path without `--prefix` exited 2 with stdout AND
# stderr empty. So a refusal leg asserts all three at once -- that it refused, that it printed no id
# while refusing, and that it named the path it could not find. A bare exit status is what a session
# reads as a broken tool, and a session holding a broken allocator goes back to numbering by eye.
alloc_refuse() { # <label> <repo> <path that is not a log> <prefix> <what an ok proves>
	local label=$1 repo=$2 path=$3 prefix=$4 fact=$5
	local got='' rc=0
	if [ -n "$prefix" ]; then
		got=$(bash "$alloc_script" --repo "$repo" --prefix "$prefix" "$path" 2>"$alloc_err") || rc=$?
	else
		got=$(bash "$alloc_script" --repo "$repo" "$path" 2>"$alloc_err") || rc=$?
	fi
	if [ "$rc" = 0 ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it answered %s instead of refusing.\n' "$label" "$got"
		printf '          A union with no headings in it has one answer available, and it is the\n'
		printf '          first id of the range -- a number both logs opened with. %s\n' "$fact"
		return 0
	fi
	if [ -n "$got" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it exited %s and printed %s anyway.\n' "$label" "$rc" "$got"
		printf '          A refusal that also hands out an id is not a refusal.\n'
		return 0
	fi
	if [ ! -s "$alloc_err" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it exited %s saying nothing at all,\n' "$label" "$rc"
		printf '          on either stream. That is the failure mode this leg exists for: nothing to\n'
		printf '          read, so the caller cannot tell a wrong path from a broken allocator.\n'
		return 0
	fi
	if ! grep -qF -- "${path##*/}" "$alloc_err"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it refused without naming %s:\n' "$label" "$path"
		sed 's/^/          /' "$alloc_err"
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-40s %s\n' "$label" "$fact"
	return 0
}

# The cross-repo read, pinned as a difference rather than as a plant. Reproducing the old fallback
# means editing both arms of the path `case` together -- a target resolved against the caller is an
# absolute path outside the fixture, which the shipped script now refuses outright -- and a fixture
# bent until a two-edit plant survives is no longer the shape that broke. So this measures the two
# trees against each other instead: the fixture's answer, the caller's answer, and the assertion that
# they are not the same number. That last one is the sentence that keeps the ok from being an accident.
#
# The caller's tree is named, not inherited. Both halves used to be true of it: it was wherever the
# suite happened to be started from, so `make verify` -- which runs this file from the repository root
# -- counted it and any other cwd printed the SKIP below, and the leg that proves the cross-repo fix
# left the tally without a word. That is H-0069's failure (six quiet SKIPs under a tally that still
# reads `passed`), and this suite is the one that recorded it. `cd` inside the substitution, so the
# suite's own cwd is untouched.
alloc_reads_its_repo() { # <label> <fixture> <expected id> <what an ok proves>
	local label=$1 fixture=$2 want=$3 fact=$4
	local got='' caller=''
	got=$(bash "$alloc_script" --repo "$fixture" --prefix D DECISIONS.md 2>"$alloc_err") || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the allocator refused a log that is there:\n' "$label"
		sed 's/^/          /' "$alloc_err"
		return 0
	}
	caller=$(cd -- "$HERE/.." &&
		bash "$alloc_script" --prefix D DECISIONS.md 2>/dev/null) || caller=''
	if [ "$got" != "$want" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s got %s, expected %s.\n' "$label" "$got" "$want"
		return 0
	fi
	if [ -z "$caller" ]; then
		printf '  SKIP  %-40s %s has no DECISIONS.md, so there is no\n' "$label" "$HERE/.."
		printf '          caller answer to prove the difference against.\n'
		return 0
	fi
	if [ "$caller" = "$want" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the fixture holds %s, which is what the caller tree\n' "$label" "$want"
		printf '          holds too. The leg cannot tell the two repos apart, so it certifies nothing.\n'
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-40s %s (the caller tree answers %s)\n' "$label" "$fact" "$caller"
	return 0
}

# Issue #253's third instruction was reported delivered by #255 and was not: `grep -c yield` over
# `docs/factory.md` answered 0 at `91d992d`, and the merge diff adds no paragraph about it. A rule that
# lives in a pull-request body is a rule nobody resolving a conflict at the trunk will read, so this
# pins both halves -- who yields, and the force-push that is the reason that side is the correct one --
# because the sentence without its reason gets re-derived by the next collision, which is the defect
# #253 opened with.
#
# Both halves are pinned as whole phrases, not as words. `grep -qF 'force-push'` over this file is
# "the word appears somewhere in 842 lines": there is one occurrence today, so the two-plant control
# passes honestly, but a second unrelated mention -- a paragraph about rebase policy, a note in a
# changelog -- would satisfy the reason half from anywhere in the document and the leg would go on
# printing ok while reading a different sentence. The pinned string is the clause, so a mention that
# is not the reason cannot clear it. Scoping both greps to the yield section would be tighter still;
# it costs a section walker, and the clause is the cheaper half of the same guarantee.
alloc_yield_rule() { # [root] -> 0 when the guide states who yields and what makes it correct
	local root=$HERE/.. guide
	[ -z "${1:-}" ] || root=$1
	guide=$root/docs/factory.md
	[ -f "$guide" ] || return 2
	grep -qF 'side that has not merged yields' "$guide" &&
		grep -qF 'force-push, denied to every agent here' "$guide"
}

# Two plants, one per half, in the layout the predicate walks. A plant that removes nothing certifies
# nothing, so each is checked for having removed its phrase before the verdict is asked.
alloc_yield_control() {
	local ctrl=$alloc_work/docs-no-yield
	if ! mkdir -p -- "$ctrl/rules/docs" "$ctrl/reasons/docs"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control trees could not be written\n' \
			'control: the yield rule, rule and reason'
		return 0
	fi
	if ! grep -vF 'side that has not merged yields' "$HERE/../docs/factory.md" \
		>"$ctrl/rules/docs/factory.md" ||
		! grep -vF 'force-push, denied to every agent here' "$HERE/../docs/factory.md" \
			>"$ctrl/reasons/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control copies could not be written\n' \
			'control: the yield rule, rule and reason'
		return 0
	fi
	if grep -qF 'side that has not merged yields' "$ctrl/rules/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant left the rule in place, so the\n' \
			'control: the yield rule, rule and reason'
		printf '          leg above was never aimed at the sentence it names.\n'
		return 0
	fi
	if grep -qF 'force-push, denied to every agent here' "$ctrl/reasons/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant left the reason in place, so the\n' \
			'control: the yield rule, rule and reason'
		printf '          leg above does not need the reason it claims to need.\n'
		return 0
	fi
	if alloc_yield_rule "$ctrl/rules" || alloc_yield_rule "$ctrl/reasons"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it accepted a guide with the rule deleted\n' \
			'control: the yield rule, rule and reason'
		printf '          from one copy and the reason deleted from the other, so the ok above is a\n'
		printf '          phrase match and not the two halves it names.\n'
		return 0
	fi
	# No `pass` increment, deliberately: this suite prints a control's `ok` without adding it to the
	# tally, and `alloc_control` and `alloc_docs_control` both establish that. A control that counted
	# would make the number on the last line mean "legs" in some rows and "legs plus controls" in
	# others -- which is how #255 came to report "four counted controls" for four uncounted ones.
	printf '  ok    %-40s each copy loses one half and the leg says\n' \
		'control: the yield rule, rule and reason'
	printf '          no; both halves are read, not a phrase that happens to sit nearby.\n'
	return 0
}

alloc_no_fetch() {
	! grep -Eq '^[[:space:]]*(env[[:space:]]+[^[:space:]]+[[:space:]]+)?git[[:space:]]+fetch' \
		"$alloc_script"
}

alloc_docs_reference() { # [root] -> 0 when both documents name the allocator and its refresh
	local root=$HERE/.. skill guide
	[ -z "${1:-}" ] || root=$1
	skill=$root/.opencode/skills/factory/SKILL.md
	guide=$root/docs/factory.md
	[ -f "$skill" ] && [ -f "$guide" ] || return 2
	# The refresh is part of what gets pinned, not a garnish beside it. An allocator that stays offline
	# is safe only while the one thing that closes the other window is written down where a session
	# reads it, and H-0071 is the correction for a sentence that claimed the window was already closed.
	grep -q 'factory/next-decision-id\.sh' "$skill" &&
		grep -q 'factory/next-decision-id\.sh' "$guide" &&
		grep -q 'git fetch' "$skill" && grep -q 'git fetch' "$guide"
}

# The control for the leg above, planted outside the repository: the same two documents with every
# line naming the refresh deleted, in the layout the predicate walks. If the leg still passes on that
# copy it is checking that a path was mentioned somewhere, not that the boundary is documented.
alloc_docs_control() {
	local ctrl=$alloc_work/docs-no-refresh
	if ! mkdir -p -- "$ctrl/.opencode/skills/factory" "$ctrl/docs"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control tree could not be written\n' \
			'control: the refresh, not just the path'
		return 0
	fi
	if ! grep -v 'git fetch' "$HERE/../.opencode/skills/factory/SKILL.md" \
		>"$ctrl/.opencode/skills/factory/SKILL.md" ||
		! grep -v 'git fetch' "$HERE/../docs/factory.md" >"$ctrl/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control copies could not be written\n' \
			'control: the refresh, not just the path'
		return 0
	fi
	if grep -q 'git fetch' "$ctrl/.opencode/skills/factory/SKILL.md" "$ctrl/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant left a refresh in place, so the\n' \
			'control: the refresh, not just the path'
		printf '          leg below was never aimed at the thing it names.\n'
		return 0
	fi
	if alloc_docs_reference "$ctrl"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it accepted documents with no refresh named,\n' \
			'control: the refresh, not just the path'
		printf '          so the ok above pins a mention and not the boundary.\n'
		return 0
	fi
	printf '  ok    %-40s it refuses documents with the refresh deleted, so\n' \
		'control: the refresh, not just the path'
	printf '          the ok above pins the boundary, not just the path.\n'
	return 0
}

# Issue #253's second instruction, and the half a leg is worth pinning: the allocator reads refs and
# the gate must not. If `scripts/check-decisions.py` started enumerating refs, `make verify` would
# judge a branch by whatever this checkout last fetched -- H-0056's hazard moved into the one check
# that has to answer identically in CI, in the review loop and at the merge. A verdict that changes
# between two identical checkouts with no commit between them is worse than no verdict: it reads as a
# flaky gate and gets re-run until it agrees. So the gate may ask git about objects (a `Checkpoint:`
# that resolves to nothing is a claim about `git revert`, and the gate refuses that), never about refs.
gate_reads_no_refs() { # [path] -> 0 when that gate asks git for nothing but object lookups
	local file=$HERE/../scripts/check-decisions.py code
	[ -z "${1:-}" ] || file=$1
	[ -f "$file" ] || return 2
	# Comments are stripped first: the file's own text explains why it reads no refs, and a scan that
	# read prose would report the explanation of the rule as a violation of it.
	code=$(sed 's/#.*//' "$file")
	printf '%s\n' "$code" | grep -oE '"git",[[:space:]]*"[^"]+"' |
		grep -vE '"git",[[:space:]]*"cat-file"' >/dev/null && {
		pos_say 'the gate asks git for a subcommand that is not cat-file: only objects, never refs'
		return 1
	}
	printf '%s\n' "$code" | grep -q 'for-each-ref' && {
		pos_say 'the gate enumerates refs, so its verdict now depends on the last fetch'
		return 1
	}
	return 0
}

# The control, planted outside the repository: the shipped gate with a ref-enumerating vector appended
# and the same scan run over the copy. Appending is enough because the scan is over text, and the
# point of the control is only that the scan can see what it claims to refuse.
gate_ref_blind_control() {
	local ctrl=$alloc_work/gate-refs-control copy
	if ! mkdir -p -- "$ctrl"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control directory could not be written\n' \
			'control: the gate stays ref-blind'
		return 0
	fi
	copy=$ctrl/check-decisions.py
	{
		cat "$HERE/../scripts/check-decisions.py"
		printf '%s\n' 'SIBLINGS = ["git", "for-each-ref", "refs/remotes"]'
	} >"$copy" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control copy could not be written\n' \
			'control: the gate stays ref-blind'
		return 0
	}
	grep -q 'for-each-ref' "$copy" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant landed nowhere the scan reads, so the\n' \
			'control: the gate stays ref-blind'
		printf '          leg above was never aimed at the thing it names.\n'
		return 0
	}
	if gate_reads_no_refs "$copy" 2>/dev/null; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it accepted a gate that enumerates refs, so\n' \
			'control: the gate stays ref-blind'
		printf '          the ok above is the scan being blind, not the gate being clean.\n'
		return 0
	fi
	printf '  ok    %-40s it refuses a gate that enumerates refs, so the\n' \
		'control: the gate stays ref-blind'
	printf '          ok above is a reading of the shipped file, not a fixed phrase.\n'
	return 0
}

# The behaviour the text scan is a proxy for, on a repository of the gate's own shape: a copy of the
# shipped script at the same relative path -- it resolves both logs and the checkpoint lookup against
# its own directory, so the copy is the same gate in a different repository -- with a sibling branch
# that repeats an id. Both halves are needed: half one says a sibling's collision leaves the verdict
# alone, half two puts the same bytes in the working log and demands a refusal. Without half two, half
# one would be passed for free by a fixture the gate never read at all.
gate_verdict_off_refs() { # -> 0 when the verdict is a reading of two files, not of the ref store
	local repo=$alloc_work/gate-verdict sha out
	command -v python3 >/dev/null || return 2
	mkdir -p -- "$repo/scripts" "$repo/factory" || {
		pos_say "could not create $repo"
		return 1
	}
	cp "$HERE/../scripts/check-decisions.py" "$repo/scripts/" || {
		pos_say 'the shipped gate could not be copied into the fixture'
		return 1
	}
	git init -q -b trunk -- "$repo" || return 1
	git -C "$repo" config user.email fixture@invalid || return 1
	git -C "$repo" config user.name fixture || return 1
	git -C "$repo" commit -q --allow-empty -m 'fixture: a commit the entries can cite' || return 1
	sha=$(git -C "$repo" rev-parse HEAD) || return 1
	# Two valid logs, one entry each, citing a checkpoint that exists: half one is only worth reading
	# if this shape is one the gate answers when there is something to answer.
	printf '# Product decisions\n\n## D-0001 · fixture decision\n\n- Question: does the gate read sibling refs?\n- Options: it reads them | it reads only the two logs\n- Chose: it reads only the two logs\n- Why: the verdict has to be identical in CI and at the merge.\n- Checkpoint: %s\n' \
		"$sha" >"$repo/DECISIONS.md" || return 1
	printf '# Harness decisions\n\n## H-0001 · fixture decision\n\n- Question: does the gate read sibling refs?\n- Options: it reads them | it reads only the two logs\n- Chose: it reads only the two logs\n- Why: the verdict has to be identical in CI and at the merge.\n- Checkpoint: %s\n' \
		"$sha" >"$repo/factory/DECISIONS.md" || return 1
	git -C "$repo" add -A || return 1
	git -C "$repo" commit -qm 'fixture: the two logs' || return 1
	git -C "$repo" checkout -qb sibling || return 1
	printf '\n## D-0001 · the duplicate a sibling took\n\n- Question: the same id twice?\n- Options: yes | no\n- Chose: yes\n- Why: the collision the allocator exists to prevent.\n- Checkpoint: %s\n' \
		"$sha" >>"$repo/DECISIONS.md" || return 1
	git -C "$repo" add -A || return 1
	git -C "$repo" commit -qm 'fixture: a sibling takes the id twice' || return 1
	git -C "$repo" checkout -q trunk || return 1

	if out=$(cd "$repo" && python3 scripts/check-decisions.py 2>&1); then
		:
	else
		pos_say 'the gate refused a log that is clean on its own branch, because a sibling ref'
		pos_say 'repeats an id. That is the verdict reading the ref store, which is what #253 forbids:'
		pos_say "$out"
		return 1
	fi
	git -C "$repo" show 'sibling:DECISIONS.md' >"$repo/DECISIONS.md" || return 1
	if out=$(cd "$repo" && python3 scripts/check-decisions.py 2>&1); then
		pos_say 'the gate accepted a duplicate id sitting in the working log, so the clean answer'
		pos_say 'above was a fixture it never read rather than a ref boundary:'
		pos_say "$out"
		return 1
	fi
	printf '%s\n' "$out" | grep -q 'D-0001' || {
		pos_say 'the gate refused, but not for the id planted in the fixture:'
		pos_say "$out"
		return 1
	}
	return 0
}

# Both halves of the multi-clone boundary, in one reporter. The unfetched half alone would be passed
# for free by a fixture whose clone never learned about origin at all, so the fetched half is the
# control for it: the answer has to be the stale ceiling first and the sibling's id after.
alloc_boundary() { # <label> <repo> <log path> <prefix> <stale id> <fresh id> <what the pair proves>
	local label=$1 repo=$2 path=$3 prefix=$4 stale=$5 fresh=$6 fact=$7 before after
	if ! before=$(alloc_run "$repo" "$path" "$prefix"); then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the allocator refused:\n' "$label"
		sed 's/^/          /' "$alloc_err"
		return 0
	fi
	if ! git -C "$repo" fetch -q origin 2>"$alloc_err"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the fixture could not fetch its origin:\n' "$label"
		sed 's/^/          /' "$alloc_err"
		return 0
	fi
	if ! after=$(alloc_run "$repo" "$path" "$prefix"); then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the allocator refused after the fetch:\n' "$label"
		sed 's/^/          /' "$alloc_err"
		return 0
	fi
	if [ "$before" != "$stale" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the unfetched clone said %s, expected %s.\n' "$label" "$before" "$stale"
		printf '          It is meant to be blind there: a push moves refs/heads on the other\n'
		printf '          machine, not refs/remotes here. That is the boundary the docs state.\n'
		return 0
	fi
	if [ "$after" != "$fresh" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the fetched clone said %s, expected %s.\n' "$label" "$after" "$fresh"
		printf '          If a fetch does not move the answer, the first half above was a fixture\n'
		printf '          that never saw origin at all rather than a boundary. %s\n' "$fact"
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-40s %s\n' "$label" "$fact"
	return 0
}

decision_ids() {
	local base current multi repo wt bad alloc_empty alloc_nolog alloc_nopfx alloc_bleed
	local alloc_notlog alloc_dirlog alloc_headonly

	if [ ! -f "$alloc_script" ]; then
		printf '  SKIP  %-40s next-decision-id.sh is not in this tree\n' \
			'decision ids across refs'
		return 0
	fi
	if ! mkdir -p -- "$alloc_work"; then
		printf '  SKIP  %-40s the fixture directory could not be created\n' \
			'decision ids across refs'
		return 0
	fi

	# A trunk at D-0001/H-0010, a sibling that advanced it by one, and no collision yet. The checkout
	# is then returned to the trunk, so a high answer necessarily came from branch refs.
	repo=$alloc_work/repo
	git init -q -- "$repo" || return 0
	git -C "$repo" config user.email fixture@invalid || return 0
	git -C "$repo" config user.name fixture || return 0
	git -C "$repo" checkout -qb main || return 0
	mkdir -p -- "$repo/factory" || return 0
	printf '## D-0001 · fixture trunk\n' >"$repo/DECISIONS.md" || return 0
	printf '## H-0010 · fixture trunk\n' >"$repo/factory/DECISIONS.md" || return 0
	git -C "$repo" add -A || return 0
	git -C "$repo" commit -qm 'fixture: the trunk ceiling' || return 0

	git -C "$repo" checkout -qb feature-a || return 0
	{
		printf '\n## D-0003 · a sibling id below the ceiling\n'
	} >>"$repo/DECISIONS.md" || return 0
	{
		printf '\n## H-0011 · a sibling id below the ceiling\n'
	} >>"$repo/factory/DECISIONS.md" || return 0
	git -C "$repo" add -A || return 0
	git -C "$repo" commit -qm 'fixture: sibling refs claim the next id' || return 0
	git -C "$repo" update-ref refs/remotes/origin/feature-a HEAD || return 0
	git -C "$repo" checkout -q main || return 0

	# This control is before the collision, while the missing branch ref is the only reason a narrower
	# allocator could ever be wrong. It reads D-0002/H-0011: the current tree's gap rather than the
	# union's ceiling.
	alloc_control 'only the current checkout is read' "$alloc_work/ctrl-no-refs" \
		's|git -C "$root" for-each-ref|true #|' \
		"$repo" DECISIONS.md D D-0002 'it missed the sibling branch'
	alloc_control 'only the current checkout is read' "$alloc_work/ctrl-no-refs-h" \
		's|git -C "$root" for-each-ref|true #|' \
		"$repo" factory/DECISIONS.md H H-0011 'it missed the sibling branch'

	alloc_leg 'refs/heads choose the product id' "$repo" DECISIONS.md D D-0004 \
		"D-0004, not the current checkout's D-0002"
	alloc_leg 'refs/heads choose the harness id' "$repo" factory/DECISIONS.md H H-0012 \
		"H-0012, not the current checkout's H-0011"

	# Now plant exactly the id each allocator has just returned onto a second remote-only ref. The
	# local checkout remains unchanged, so if the next answer does not move, the allocator ignored
	# refs/remotes -- the precise way a sibling that has been pushed is missed.
	git -C "$repo" checkout -qb remote-collision main || return 0
	{
		printf '\n## D-0004 · a collision planted on a remote-only ref\n'
	} >>"$repo/DECISIONS.md" || return 0
	{
		printf '\n## H-0012 · a collision planted on a remote-only ref\n'
	} >>"$repo/factory/DECISIONS.md" || return 0
	git -C "$repo" add -A || return 0
	git -C "$repo" commit -qm 'fixture: the second ref takes the next id' || return 0
	git -C "$repo" update-ref refs/remotes/origin/collision HEAD || return 0
	git -C "$repo" checkout -q main || return 0
	git -C "$repo" branch -q -D remote-collision || return 0

	alloc_control 'only refs/heads is read' "$alloc_work/ctrl-no-remotes" \
		's|refs/heads refs/remotes|refs/heads|' \
		"$repo" DECISIONS.md D D-0004 'it missed the remote-only collision'
	alloc_control 'only refs/heads is read' "$alloc_work/ctrl-no-remotes-h" \
		's|refs/heads refs/remotes|refs/heads|' \
		"$repo" factory/DECISIONS.md H H-0012 'it missed the remote-only collision'

	alloc_leg 'a remote-only collision is refused' "$repo" DECISIONS.md D D-0005 \
		'D-0005, because D-0004 is already on origin/collision'
	alloc_leg 'a remote-only collision is refused' "$repo" factory/DECISIONS.md H H-0013 \
		'H-0013, because H-0012 is already on origin/collision'

	# A fresh repo with no commits and no branch refs. The only id there is the working file's entry
	# about to be committed; if this returns D-0001, the allocator was only reading git's committed
	# copies and each new session would be invited to take the same number.
	current=$alloc_work/current-checkout
	git init -q -- "$current" || return 0
	git -C "$current" config user.email fixture@invalid || return 0
	git -C "$current" config user.name fixture || return 0
	printf '## D-0009 · an uncommitted candidate\n' >"$current/DECISIONS.md" || return 0
	alloc_leg 'the current file counts before it is committed' "$current" DECISIONS.md D D-0010 \
		'D-0010, because the uncommitted working entry is already D-0009'

	# ---------------------------------------------------------- what the re-read can and cannot see
	#
	# The skill tells every session to re-run the allocator immediately before the entry commit. That
	# is only worth what the second read can see, and the two shapes a sibling arrives in differ, so
	# both are measured here (H-0071) rather than asserted in prose -- prose is what this whole section
	# exists to stop being.
	#
	# Another worktree of the same clone is the shape this harness actually produces: `common.sh` makes
	# every unit tree with `git worktree add`, and worktrees share `refs/heads`. So a sibling tick's
	# committed entry is visible from the primary with no fetch, and the re-read is real.
	wt=$alloc_work/worktrees
	git init -q -- "$wt/base" || return 0
	git -C "$wt/base" config user.email fixture@invalid || return 0
	git -C "$wt/base" config user.name fixture || return 0
	git -C "$wt/base" checkout -qb trunk || return 0
	printf '## D-0001 · fixture trunk\n' >"$wt/base/DECISIONS.md" || return 0
	git -C "$wt/base" add -A || return 0
	git -C "$wt/base" commit -qm 'fixture: the trunk before the worktrees' || return 0
	git -C "$wt/base" worktree add -qb sibling "$wt/sibling" trunk || return 0
	{
		printf '\n## D-0009 · a sibling unit in a second worktree\n'
	} >>"$wt/sibling/DECISIONS.md" || return 0
	git -C "$wt/sibling" add -A || return 0
	git -C "$wt/sibling" commit -qm 'fixture: the sibling worktree takes the next id' || return 0

	alloc_control 'only the current checkout is read' "$alloc_work/ctrl-no-worktree" \
		's|git -C "$root" for-each-ref|true #|' \
		"$wt/base" DECISIONS.md D D-0002 'it missed the sibling worktree'
	alloc_leg 'a sibling worktree is refused its id' "$wt/base" DECISIONS.md D D-0010 \
		'D-0010, read off the sibling worktree with no fetch'

	# Another clone is the shape the re-read cannot see: a push moves `refs/heads` over there, and this
	# clone's `refs/remotes` moves only when something fetches. origin is a local bare repo, so the leg
	# measures ref freshness and never a network -- the same reason the shipped allocator is allowed to
	# stay offline (H-0056).
	multi=$alloc_work/multi-clone
	git init -q --bare "$multi/origin.git" || return 0
	git --git-dir="$multi/origin.git" config user.email fixture@invalid || return 0
	git --git-dir="$multi/origin.git" config user.name fixture || return 0
	# The bare origin authors no commits and it still carries an identity: a fixture that inherits the
	# host's is the failure H-0059 records, and the scan over this file cannot tell a repo that will
	# never commit from one that will.
	git clone -q "$multi/origin.git" "$multi/a" >/dev/null 2>&1 || return 0
	git -C "$multi/a" config user.email fixture@invalid || return 0
	git -C "$multi/a" config user.name fixture || return 0
	git -C "$multi/a" checkout -qb trunk || return 0
	printf '## D-0001 · fixture trunk\n' >"$multi/a/DECISIONS.md" || return 0
	git -C "$multi/a" add -A || return 0
	git -C "$multi/a" commit -qm 'fixture: the trunk two machines share' || return 0
	git --git-dir="$multi/origin.git" fetch -q "$multi/a" 'HEAD:refs/heads/trunk' || return 0
	git -C "$multi/a" fetch -q origin || return 0
	# A second machine: its own clone, an id far above the trunk's, delivered to origin while clone a
	# is not looking. Fetching into the bare repo is how a push lands there.
	git clone -q "$multi/origin.git" "$multi/b" >/dev/null 2>&1 || return 0
	git -C "$multi/b" config user.email fixture@invalid || return 0
	git -C "$multi/b" config user.name fixture || return 0
	git -C "$multi/b" checkout -qb elsewhere origin/trunk || return 0
	{
		printf '\n## D-0007 · an entry pushed from a second clone\n'
	} >>"$multi/b/DECISIONS.md" || return 0
	git -C "$multi/b" add -A || return 0
	git -C "$multi/b" commit -qm 'fixture: a sibling pushed from elsewhere' || return 0
	git --git-dir="$multi/origin.git" fetch -q "$multi/b" 'HEAD:refs/heads/elsewhere' || return 0

	alloc_boundary 'an unfetched sibling push is invisible' "$multi/a" DECISIONS.md D \
		D-0002 D-0008 'blind until fetched, so the refresh is the other half'

	claim_leg 'the gate reads no branch refs' gate_reads_no_refs
	gate_ref_blind_control
	gen_leg 'a sibling id leaves the verdict alone' \
		'a duplicate on a sibling ref does not move the answer' \
		gate_verdict_off_refs

	gen_leg 'the allocator never fetches' 'no git fetch is reachable from its code' \
		alloc_no_fetch
	gen_leg 'the shared allocator is documented' \
		'the skill and the factory guide both name its path and the refresh' \
		alloc_docs_reference
	alloc_docs_control

	gen_leg 'the yield rule is written down' \
		'the guide says which side renumbers, and why that side is the one that can move' \
		alloc_yield_rule
	alloc_yield_control

	# ---- what the allocator does when there is nothing to read (issue #253, the half #255 left) ----
	#
	# Every leg above writes its fixture's log before the leg runs and passes --prefix, so not one of
	# them ever reached the two arms a mistyped path falls into. All three refusals below were
	# measured on the shipped script before this branch touched it, and the block that follows them
	# adds the four paths #257's review measured at `c5aa1eb`:
	#
	#   --prefix H factory/DECISION.md    printed H-0001 and exited 0. A union with no headings in it
	#                                     has exactly one answer available, and H-0001 is the first
	#                                     heading the real harness log has carried since its own first
	#                                     entry -- so a typo used to be answered with a collision.
	#   factory/DECISION.md               exited 2 with stdout AND stderr empty. `sed` fails on the
	#                                     missing file, the command-substitution assignment inherits
	#                                     status 2, and `set -e` ends the script before any message.
	#   --repo <a checkout with no log>   answered D-0052, this repository's own ceiling, both with a
	#                                     commit in the fixture and with none: the log path fell back
	#                                     to the caller's tree and read that log.
	#
	# The fixtures sit at H-0009 and D-0003 so the wrong answer is never ambiguous: the right path is
	# answered at H-0010/D-0004, and the empty union was answered with the log's first number.
	alloc_fixture() { # <dir> <log path> <body> -> 0 when the repo holds one commit, that log if a body
		local dir=$1 rel=$2 body=$3
		git init -q "$dir" 2>/dev/null || return 1
		git -C "$dir" config user.email fixture@invalid || return 1
		git -C "$dir" config user.name fixture || return 1
		if [ -n "$body" ]; then
			mkdir -p -- "$(dirname -- "$dir/$rel")" || return 1
			printf '%s\n' "$body" >"$dir/$rel" || return 1
		fi
		git -C "$dir" add -A || return 1
		git -C "$dir" commit -q --allow-empty -m 'fixture: the log a sibling branch would cite' || return 1
	}

	alloc_empty=$alloc_work/empty-union
	alloc_nolog=$alloc_work/repo-without-a-log
	alloc_nopfx=$alloc_work/prefix-inferred
	alloc_bleed=$alloc_work/repo-bleed
	if ! alloc_fixture "$alloc_empty" factory/DECISIONS.md '## H-0009 · the fixture ceiling' ||
		! alloc_fixture "$alloc_nolog" DECISIONS.md '' ||
		! alloc_fixture "$alloc_nopfx" factory/DECISIONS.md '# Harness decision log' ||
		! alloc_fixture "$alloc_bleed" DECISIONS.md '## D-0003 · the fixture trunk'; then
		printf '  SKIP  %-40s the throwaway repositories could not be\n' "the empty-union legs"
		printf '          written; every leg in this group is about a repo that does not exist.\n'
		return 0
	fi

	alloc_leg 'the same fixture asked correctly answers' "$alloc_empty" factory/DECISIONS.md H \
		H-0010 'the refusals below are about the path asked for, not a repo the command cannot read'

	alloc_refuse 'a mistyped log path refuses' "$alloc_empty" factory/DECISION.md H \
		'refusal is the only answer that is not a collision'
	# The comma delimiter is deliberate: the guard's call site is `log_is_here ||`, and `|` cannot be
	# both the sed delimiter and a literal here -- `\|` is alternation in a BRE, so an escaped plant
	# would match something else and change the wrong line.
	alloc_control 'a mistyped log path refuses' "$alloc_work/ctrl-no-guard" \
		's,^log_is_here ||,: log_is_here ||,' \
		"$alloc_empty" factory/DECISION.md H H-0001 \
		'without the guard a typo is answered with the first H id, taken since entry one'

	alloc_refuse 'a mistyped path with no prefix refuses' "$alloc_empty" factory/DECISION.md '' \
		'and names the path, where it used to exit 2 saying nothing on either stream'

	alloc_refuse 'a repository with no log refuses' "$alloc_nolog" DECISIONS.md D \
		'so a bare checkout is never answered out of the tree the command was invoked from'

	# The shape H-0073's `Effect:` claimed was fatal and was not: the log is in HEAD and gone from the
	# working tree. Pinned as an answer rather than as a refusal, because that is what it is -- the
	# guard passes on the blob in HEAD, the `if` skips a file that is not there, and the union reads
	# HEAD. Measured on `91d992d` before it was written, and it answered `D-0008` with status 0 there
	# too: no leg planted this shape, which is exactly why a claim about it survived one review.
	alloc_headonly=$alloc_work/log-in-head-not-in-tree
	if ! alloc_fixture "$alloc_headonly" DECISIONS.md '## D-0007 · the entry only HEAD still holds' ||
		! rm -f -- "$alloc_headonly/DECISIONS.md"; then
		printf '  SKIP  %-40s the deleted-from-the-tree fixture could not be\n' 'the HEAD-only log leg'
		printf '          written; the leg below is about a tree that does not exist.\n'
	else
		alloc_leg 'a log in HEAD but not in the tree answers from HEAD' "$alloc_headonly" \
			DECISIONS.md D D-0008 \
			'the union reads HEAD; the AND-list was never the trap H-0073 said it was'
	fi

	# ---- a path that exists and is still not the log (PR #257's review of the guard above) -------
	#
	# That guard asks whether the named path exists, and every wrong path in a real checkout exists.
	# Measured on `c5aa1eb`, in this repository, each of these printed `H-0001` and exited 0 when
	# called with `--prefix H`:
	#
	#   docs/factory.md   the 842-line guide that tells a session to run this command (its line 455)
	#                     -- the file a session is reading at the moment it reaches for the allocator
	#   README.md, AGENTS.md   the two other markdown files at the trunk
	#   factory, factory/      a directory in the working tree, a tree in HEAD, and `cat-file -e` is
	#                     true of a tree
	#
	# Worse than the mistyped name above, not merely different: the refusal this branch shipped says
	# *your path is wrong*, so a session handed `H-0001` for `docs/factory.md` reads that as "my log
	# was read" and writes a duplicate heading -- the silent collision, where an error message would
	# have been visible. And no leg could have seen the shape: every path named in this whole section
	# is `DECISIONS.md`, `factory/DECISIONS.md` or `factory/DECISION.md`, and all three are absent, so
	# an existence predicate and a log predicate agree on every fixture the suite owns (H-0076).
	#
	# Two fixtures, one per arm of the predicate, because the arms fail differently and one fixture
	# that trips both would prove neither: `$alloc_notlog` has the wrong name and the right type, and
	# `$alloc_dirlog` the right name and the wrong type.
	alloc_shapes() { # <dir holding docs and a factory/ dir> <dir where DECISIONS.md is a directory>
		local n=$1 d=$2
		git init -q "$n" 2>/dev/null || return 1
		git -C "$n" config user.email fixture@invalid || return 1
		git -C "$n" config user.name fixture || return 1
		mkdir -p -- "$n/docs" "$n/factory" || return 1
		printf '# The factory guide\n' >"$n/docs/factory.md" || return 1
		printf '# A harness note\n' >"$n/factory/guide.md" || return 1
		git -C "$n" add -A || return 1
		git -C "$n" commit -q -m 'fixture: a checkout with documents and no decision log' || return 1
		git init -q "$d" 2>/dev/null || return 1
		git -C "$d" config user.email fixture@invalid || return 1
		git -C "$d" config user.name fixture || return 1
		mkdir -p -- "$d/DECISIONS.md" || return 1
		printf '# notes, not a log\n' >"$d/DECISIONS.md/notes.md" || return 1
		git -C "$d" add -A || return 1
		git -C "$d" commit -q -m 'fixture: DECISIONS.md is a directory' || return 1
	}

	alloc_notlog=$alloc_work/path-that-exists
	alloc_dirlog=$alloc_work/directory-named-decisions-md
	if ! alloc_shapes "$alloc_notlog" "$alloc_dirlog"; then
		printf '  SKIP  %-40s the two not-a-log fixtures could not be\n' 'the not-a-log refusals'
		printf '          written; every leg below is about a tree that does not exist.\n'
	else
		alloc_refuse 'a document that exists is not the log' "$alloc_notlog" docs/factory.md H \
			'the guide that documents this command is not a log, and the refusal names it'
		alloc_refuse 'a directory is not the log' "$alloc_notlog" factory H \
			'the directory that holds both logs is neither of them'
		alloc_refuse 'a DECISIONS.md directory is not the log' "$alloc_dirlog" DECISIONS.md D \
			'the name matches, and a tree in HEAD is still not the log'
		# One plant per arm, each leaving the other arm standing -- both were run against the other
		# fixture to confirm exactly that: the name plant still refuses the tree, and the type plant
		# still refuses the mistyped name. The name plant opens the arm to every basename; the type
		# plant writes back the predicate this branch replaced, `git cat-file -e`, which a directory
		# named DECISIONS.md satisfies.
		alloc_control 'a document that exists is not the log' "$alloc_work/ctrl-no-name-test" \
			's|DECISIONS.md) ;;|*) ;;|' \
			"$alloc_notlog" docs/factory.md H H-0001 \
			'without the name test a guide, a README and an AGENTS file are all decision logs'
		alloc_control 'a DECISIONS.md directory is not the log' "$alloc_work/ctrl-any-type" \
			's|\[ "$head_kind" = blob \]|git -C "$root" cat-file -e "HEAD:$rel" 2>/dev/null|' \
			"$alloc_dirlog" DECISIONS.md D D-0001 \
			'asking only whether anything sits at that path in HEAD is the predicate this replaced'
	fi

	alloc_reads_its_repo '--repo reads the repo it was aimed at' "$alloc_bleed" D-0004 \
		'the answer is the fixture ceiling, never the caller tree'

	alloc_leg 'a heading-less log takes its prefix' "$alloc_nopfx" factory/DECISIONS.md '' \
		H-0001 'the arm that used to die silently now reaches a fresh harness log'
	alloc_control 'a heading-less log takes its prefix' "$alloc_work/ctrl-no-path-inference" \
		's|factory/DECISIONS.md) prefix=H|factory/DECISIONS.md) :|' \
		"$alloc_nopfx" factory/DECISIONS.md '' D-0001 \
		'without the path rule it answers in D, the wrong log, for the file it is standing in'

	return 0
}
decision_ids

# --------------------------------------------------------------------------- the refs the pass reads
#
# H-0075 (#256). Everything the pass decides is a reading of refs: `triage.sh` reports this tree's
# position against `origin/main`, `factory_worktree` cuts a new unit's branch from `origin/main`, and
# the allocator pinned by the section above reads every `refs/heads` and `refs/remotes` blob. Nothing
# in the harness fetched, so all three read the last time this clone looked, and `H-0071`'s second row
# -- a sibling pushed from another clone -- stayed open with only an operator's command to close it.
# The legs here pin the closing, and they pin the *shape* the closing needs, which is the part no
# prose would have caught: measured in a fixture, `git fetch --prune origin trunk` leaves the
# allocator at its stale answer while `git fetch --prune origin` moves it, because the sibling's ids
# are on the branch it pushed and not on the trunk. #256 asked for the trunk-limited form, so a leg
# that only asked "is there a fetch?" would have passed the command that fixes nothing.
#
#   the shape     the fetch is unrefspec'd, so it updates every branch ref (control: the trunk-only
#                 plant, which fetches successfully, learns nothing, and must leave the id stale).
#   the tree      the one fetch in the primary checkout moves the answer in a linked *unit worktree*,
#                 because `refs/remotes` is the common dir -- so a tick gets the refresh it never
#                 asked for. That is the leg's measurement, not an assumption beside it.
#   the bound     `timeout` with the config value, TERM first and a kill-after behind it (control: the
#                 same function with `timeout` taken out, which must be refused). Measured 9 Nov
#                 against an origin that accepts the connection and never answers: 124 at the bound,
#                 no `.lock` file left in `.git`, and the clone usable afterwards.
#   the polarity  a fetch that fails is one journal line and a return of 0, because the pass holds
#                 `factory/run/lock` (control: the same function with a `die` planted in the failure
#                 arm, which must be refused).
#   the cadence   one fetch call per activation, ahead of the triage read (control: a second call
#                 planted after the triage read, which must be refused on both counts).
#
# The legs source the shipped function out of `factory/factory.sh` rather than calling the script: a
# whole pass reads GitHub and starts ticks, which no leg may do. Sourcing an extraction keeps the
# subject the shipped code -- a copy of it would be a second source that drifts, which is why every
# other leg in this file extracts too.

refsh_work=$work/pass-refresh
refsh_script=$HERE/factory.sh
refsh_common=$HERE/common.sh
refsh_err=$work/pass-refresh.err

# shellcheck disable=SC2317  # reached through the command substitutions below
refsh_fn() { # <dir> [sed expr] -> a sourceable copy of the shipped factory_refresh_origin
	local dir=$1 expr=${2:-}
	mkdir -p -- "$dir" || return 1
	awk '/^factory_refresh_origin\(\)/,/^}/' "$refsh_script" >"$dir/shipped" || return 1
	grep -q 'fetch --prune origin >' "$dir/shipped" || {
		pos_say 'factory_refresh_origin is not in the shipped file, or no longer fetches'
		return 1
	}
	if [ -n "$expr" ]; then
		sed "$expr" "$dir/shipped" >"$dir/fn.sh" || return 1
		cmp -s "$dir/fn.sh" "$dir/shipped" && return 1
	else
		cp -- "$dir/shipped" "$dir/fn.sh" || return 1
	fi
	printf '%s' "$dir/fn.sh"
}

# The fixture #256 describes: a clone with a unit worktree, asleep, and a sibling that pushed an entry
# from a second clone. The entry is D-0007 so the two answers are far apart -- a blind `D-0002` and a
# refreshed `D-0008` -- and the trunk carries nothing above D-0001, so only a branch ref can move it.
# shellcheck disable=SC2317  # reached from the legs below
refsh_repo() { # <dir> -> 0 when the fixture is standing
	local d=$1
	mkdir -p -- "$d" || return 1
	# The bare origin authors no commits and it still carries an identity: a fixture that inherits the
	# host's is H-0059, and the scan over this file cannot tell a repo that will never commit from one
	# that will. The two config lines are inside the three lines the scan reads after the init.
	git init -q --bare "$d/origin.git" || return 1
	git --git-dir="$d/origin.git" config user.email fixture@invalid || return 1
	git --git-dir="$d/origin.git" config user.name fixture || return 1
	git clone -q "$d/origin.git" "$d/main" >/dev/null 2>&1 || return 1
	git -C "$d/main" config user.email fixture@invalid || return 1
	git -C "$d/main" config user.name fixture || return 1
	git -C "$d/main" checkout -qb trunk || return 1
	printf '## D-0001 · fixture trunk\n' >"$d/main/DECISIONS.md" || return 1
	git -C "$d/main" add -A >/dev/null || return 1
	git -C "$d/main" commit -qm 'fixture: the trunk a queue runs from' || return 1
	git -C "$d/main" push -q -u origin trunk || return 1
	# The tree a tick is actually started in. `common.sh` makes every unit tree with
	# `git worktree add`, so this is the shape the refresh has to reach.
	git -C "$d/main" worktree add -qb unit "$d/unit" trunk >/dev/null 2>&1 || return 1
	git clone -q "$d/origin.git" "$d/other" >/dev/null 2>&1 || return 1
	git -C "$d/other" config user.email fixture@invalid || return 1
	git -C "$d/other" config user.name fixture || return 1
	git -C "$d/other" checkout -qb issue-z origin/trunk || return 1
	printf '\n## D-0007 · an entry pushed from a second clone\n' >>"$d/other/DECISIONS.md" || return 1
	git -C "$d/other" add -A >/dev/null || return 1
	git -C "$d/other" commit -qm 'fixture: a sibling pushed while the first machine slept' || return 1
	git -C "$d/other" push -q origin issue-z || return 1
	return 0
}

# Runs one copy of the function the way the pass runs it, in the fixture's environment. `log_event`
# echoes as it journals, so stdout is the sentence a reader would see.
# shellcheck disable=SC2317  # reached from the legs below
refsh_run() { # <fn file> <tree> <fixture> -> the refresh's journal line
	local fn=$1 tree=$2 fx=$3
	FACTORY_DIR=$fx/harness FACTORY_ROOT=$tree FACTORY_BASE=trunk \
		FACTORY_LOG=$fx/harness/log FACTORY_LEDGER=$fx/harness/ledger \
		FACTORY_RUN=$fx/harness/run FACTORY_FETCH_TIMEOUT=30 DRY_RUN=0 \
		bash -c '. "$1"; . "$2"; factory_refresh_origin "$3"' sh \
		"$refsh_common" "$fn" "$tree" 2>"$refsh_err"
}

# shellcheck disable=SC2317  # reached from the legs below
refsh_alloc() { # <tree> -> the id a tick standing there would allocate
	bash "$alloc_script" --repo "$1" --prefix D DECISIONS.md 2>"$refsh_err"
}

pass_refresh_reaches_the_unit_tree() {
	local fx=$refsh_work/tree fn before after out rc=0
	refsh_repo "$fx" || { pos_say "the fixture could not be built in $fx"; return 1; }
	fn=$(refsh_fn "$fx/plain") || { pos_say 'the shipped function could not be extracted'; return 1; }
	before=$(refsh_alloc "$fx/unit")
	# Half one is the fixture's own honesty test: a clone that already knew about the push would make
	# the after answer worth nothing (the reason `alloc_boundary` asserts the blind half, same file).
	[ "$before" = "D-0002" ] || {
		pos_say "the unfetched unit tree already answered $before, not D-0002 -- the fixture is"
		pos_say 'not blind, so the moved answer below would certify the fixture, not the fetch'
		return 1
	}
	out=$(refsh_run "$fn" "$fx/main" "$fx") || rc=$?
	[ "$rc" -eq 0 ] || {
		pos_say 'the shipped refresh returned non-zero, which is a pass stopped by a fetch:'
		pos_say "$out"
		pos_say "$(cat "$refsh_err" 2>/dev/null)"
		return 1
	}
	printf '%s\n' "$out" | grep -q 'REFRESH origin' || {
		pos_say "the refresh returned 0 without saying what it did: $out"
		return 1
	}
	after=$(refsh_alloc "$fx/unit")
	[ "$after" = "D-0008" ] || {
		pos_say "a tick in the unit worktree still allocates $after, not D-0008."
		pos_say 'One fetch in the primary checkout has to move the answer there, because'
		pos_say 'refs/remotes is the common dir -- or the refresh never reaches the process it was'
		pos_say 'bought for. Its own line said:'
		pos_say "$out"
		return 1
	}
	return 0
}

# The control the leg above is only worth reading against: the command #256 asked for. It fetches, it
# succeeds, it reports zero moved, and the unit tree is exactly as blind as it was -- so the ok above
# is the shape of the fetch and not the presence of one.
pass_refresh_trunk_only_control() {
	local label='a trunk-limited fetch' fx=$refsh_work/ctrl-trunkonly fn out rc=0 got
	fn=$(refsh_fn "$fx/plant" 's#fetch --prune origin >#fetch --prune origin "$FACTORY_BASE" >#') || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant could not be written or did\n' "control: $label"
		printf '          nothing change; the leg above has stopped pinning the fetch shape.\n'
		return 0
	}
	refsh_repo "$fx" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the fixture could not be built\n' "control: $label"
		return 0
	}
	out=$(refsh_run "$fn" "$fx/main" "$fx") || rc=$?
	got=$(refsh_alloc "$fx/unit")
	# `0 moved` and a zero status: the trunk-only fetch has to *succeed* for this to discriminate. A
	# plant that failed would leave the id stale for the wrong reason and read as a pass.
	if [ "$rc" -ne 0 ] || ! printf '%s\n' "$out" | grep -q '0 moved'; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the planted fetch did not run and report nothing\n' "control: $label"
		printf '          moved (status %s), so the stale id below proves nothing:\n' "$rc"
		printf '          %s\n' "$out"
		return 0
	fi
	if [ "$got" != "D-0002" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the trunk-limited fetch moved the answer to %s,\n' "control: $label" "$got"
		printf '          so the ok above would not be the refspec being pinned.\n'
		return 0
	fi
	printf '  ok    %-40s it fetches cleanly, learns nothing, and leaves\n' "control: $label"
	printf '          the unit tree at D-0002 -- the leg above pins the refspec.\n'
	return 0
}

pass_refresh_is_bounded() { # [file] -> 0 when the shipped fetch is bounded by the config value
	local script=$refsh_script body
	[ -z "${1:-}" ] || script=$1
	body=$(awk '/^factory_refresh_origin\(\)/,/^}/' "$script")
	[ -n "$body" ] || { pos_say "no factory_refresh_origin in $script"; return 1; }
	printf '%s\n' "$body" |
		grep -qE 'timeout --signal=TERM --kill-after=[0-9]+ "\$FACTORY_FETCH_TIMEOUT"' || {
		pos_say 'the fetch is not wrapped in timeout carrying the config value, TERM first.'
		pos_say 'The bound is the whole answer to a hung fetch holding factory/run/lock.'
		return 1
	}
	grep -q 'FACTORY_FETCH_TIMEOUT=${FACTORY_FETCH_TIMEOUT:-' "$refsh_common" || {
		pos_say 'FACTORY_FETCH_TIMEOUT has no default in common.sh, so the bound is unset for'
		pos_say 'every pass that does not export one.'
		return 1
	}
	return 0
}

pass_refresh_unbounded_control() {
	local label='a fetch with no timeout' dir=$refsh_work/ctrl-no-timeout fn body
	fn=$(refsh_fn "$dir" 's#timeout --signal=TERM --kill-after=5 "\$FACTORY_FETCH_TIMEOUT"#env#') || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant could not be written or did\n' "control: $label"
		printf '          nothing change; the leg above has stopped pinning the bound.\n'
		return 0
	}
	body=$(awk '/^factory_refresh_origin\(\)/,/^}/' "$fn")
	if printf '%s\n' "$body" |
		grep -qE 'timeout --signal=TERM --kill-after=[0-9]+ "\$FACTORY_FETCH_TIMEOUT"'; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the scan still passes a function with no timeout\n' "control: $label"
		printf '          in it, so the ok above is the scan being blind.\n'
		return 0
	fi
	printf '  ok    %-40s it refuses the same function with timeout taken\n' "control: $label"
	printf '          out, so the ok above is a reading of the shipped text.\n'
	return 0
}

pass_failure_is_not_fatal() { # [file] -> 0 when a failed fetch can only leave a sentence behind
	local script=$refsh_script body
	[ -z "${1:-}" ] || script=$1
	body=$(awk '/^factory_refresh_origin\(\)/,/^}/' "$script")
	[ -n "$body" ] || { pos_say "no factory_refresh_origin in $script"; return 1; }
	printf '%s\n' "$body" | grep -qE '(^|[^[:alnum:]_])(die|exit)[[:space:]]' && {
		pos_say 'the refresh ends the process on some path. It runs inside the lock, so that is a'
		pos_say 'queue stopped by a fetch: a refusal here is a journal line and a return of 0.'
		return 1
	}
	printf '%s\n' "$body" | grep -qE 'return [1-9]' && {
		pos_say 'the refresh returns non-zero on some path, and the pass would treat it as a'
		pos_say 'failure the way every other call site does. It must not.'
		return 1
	}
	printf '%s\n' "$body" | grep -q 'REFRESH-TIMEOUT' || {
		pos_say 'nothing names the bound being hit -- 124 and 128 have to read differently in'
		pos_say 'the journal, because only one of them is a parked connection.'
		return 1
	}
	printf '%s\n' "$body" | grep -q 'REFRESH-FAILED' || {
		pos_say 'nothing names a plain fetch failure.'
		return 1
	}
	return 0
}

pass_fatal_failure_control() {
	local label='a fetch that ends the pass' dir=$refsh_work/ctrl-fatal fn body
	fn=$(refsh_fn "$dir" 's#^\t\twhy=\$(grep -m1#		die "origin unread"\n\t\twhy=$(grep -m1#') || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant could not be written or did\n' "control: $label"
		printf '          nothing change; the leg above has stopped pinning the polarity.\n'
		return 0
	}
	body=$(awk '/^factory_refresh_origin\(\)/,/^}/' "$fn")
	if ! printf '%s\n' "$body" | grep -qE '(^|[^[:alnum:]_])(die|exit)[[:space:]]'; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the scan accepted a function whose failure arm\n' "control: $label"
		printf '          dies, so the ok above is the scan being blind.\n'
		return 0
	fi
	printf '  ok    %-40s it refuses the same function with a die planted\n' "control: $label"
	printf '          in the failure arm, which is the halt H-0056 refuses here.\n'
	return 0
}

pass_one_fetch_per_activation() { # [file] -> 0 when one unrefspec'd fetch runs, once, before triage
	local script=$refsh_script calls sites triage
	[ -z "${1:-}" ] || script=$1
	calls=$(grep -cE '^[[:space:]]*[^#].*fetch --prune origin >' "$script" || true)
	[ "${calls:-0}" -eq 1 ] || {
		pos_say "$calls lines fetch the whole remote in $script; the pass wants exactly one fetch"
		pos_say 'per activation, and a second one is either per-unit or inside a verdict.'
		return 1
	}
	sites=$(grep -c 'factory_refresh_origin "\$(factory_main_checkout)"' "$script" || true)
	[ "${sites:-0}" -eq 1 ] || {
		pos_say "$sites call sites for factory_refresh_origin in $script -- one fetch per"
		pos_say 'activation, not one per unit.'
		return 1
	}
	sites=$(grep -n 'factory_refresh_origin "\$(factory_main_checkout)"' "$script" |
		head -n 1 | cut -d: -f1)
	triage=$(grep -n 'rows=\$("\$FACTORY_DIR/triage.sh")' "$script" | head -n 1 | cut -d: -f1)
	if [ -z "$sites" ] || [ -z "$triage" ]; then
		pos_say 'no call site for factory_refresh_origin, or no triage read to be ahead of.'
		return 1
	fi
	[ "$sites" -lt "$triage" ] || {
		pos_say "the call is on line $sites and the queue is read on line $triage -- behind the"
		pos_say 'read, the refresh only reports on the refs instead of dispatching against them.'
		return 1
	}
	return 0
}

pass_second_fetch_control() {
	local label='a fetch behind the triage read' dir=$refsh_work/ctrl-twice copy
	mkdir -p -- "$dir" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the directory could not be written\n' "control: $label"
		return 0
	}
	copy=$dir/factory.sh
	sed -e '/triage.sh"); then/a\	factory_refresh_origin "$(factory_main_checkout)"' \
		"$refsh_script" >"$copy" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the copy could not be written\n' "control: $label"
		return 0
	}
	[ "$(grep -c 'factory_refresh_origin "\$(factory_main_checkout)"' "$copy")" -gt 1 ] || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant landed nowhere, so the leg above was\n' "control: $label"
		printf '          never aimed at a second call.\n'
		return 0
	}
	if pass_one_fetch_per_activation "$copy"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the leg accepted a copy with a second fetch\n' "control: $label"
		printf '          behind the queue read, so its ok counts for nothing.\n'
		return 0
	fi
	printf '  ok    %-40s it refuses a copy with a second call behind\n' "control: $label"
	printf '          the triage read, so the ok above is cadence and position.\n'
	return 0
}

mkdir -p -- "$refsh_work"
gen_leg 'the pass refreshes before it dispatches' \
	'a sibling pushed from another clone is allocated against, in the unit worktree' \
	pass_refresh_reaches_the_unit_tree
pass_refresh_trunk_only_control
gen_leg 'the refresh is bounded by a timeout' \
	'timeout with the config value wraps the one fetch' \
	pass_refresh_is_bounded
pass_refresh_unbounded_control
gen_leg 'a failed fetch never stops the pass' \
	'no die, no non-zero return, both failure arms named' \
	pass_failure_is_not_fatal
pass_fatal_failure_control
gen_leg 'one fetch per activation' \
	'one fetch, one call site, ahead of the triage read' \
	pass_one_fetch_per_activation
pass_second_fetch_control

# ============================================ a tick that stopped clean and delivered nothing (#247)
#
# `exit=0` is the CLI's verdict on whether the session closed its turn. Two ticks on 8 Nov closed with
# 0 and delivered nothing, and the journal said the same seven words about both, so the first thing
# this section pins is the *sentence* -- which of the shapes a stop had, read off the end of the log.
# The second thing it pins is the bundle, because a reason does not recover a commit.
#
# Fixtures, not models: a leg cannot make a model misspell a tool name on command. The shapes are
# planted as tick logs on disk in the byte-for-byte shapes measured in
# `20261108T233014Z.unclaimed:246.log` (a misspelled call) and
# `20261108T215923Z.unclaimed:241.log` (a turn opener with nothing after it), and the detector is
# handed the file. What a fixture cannot prove is that a real CLI writes those bytes -- that is what
# the real-journal control at the end is for, and it says out loud when there is no journal to read.
#
# Two ways this section could be green and blind, both controlled rather than assumed:
#
#   A detector that scans the whole file instead of the final turn.
#   `20261108T234413Z.unclaimed:247.log` -- the tick handed this very issue -- carries nine
#   `<function=` hits, because the issue it was given quotes `<function=executae>` at it. A whole-file
#   grep reports a misspelled tool call for a tick that misspelled nothing. The leg
#   `a brief quoting the failure` plants exactly that shape and reddens the whole-file detector.
#
#   A mirror that prints PRESERVED for a file that is not a bundle.
#   The write is read back with `git bundle verify`; the shim leg refuses the verify and the leg fails
#   if the word PRESERVED appears anywhere in the journal.

stop_dir=$work/stops
mkdir -p "$stop_dir"

stop_leg() { # label fixture want_rc want_substring [absent_substring]
	local label=$1 f=$2 want=$3 want_in=$4 want_out=${5:-}
	stop_rc=0
	stop_out=$(factory_stop_reason "$f") || stop_rc=$?
	# A reason carrying a newline is a journal line spliced in two -- the same defect #242 found in a
	# review body. Every leg checks it, because every leg hands in hostile bytes.
	case "$stop_out" in
	*$'\n'*)
		fail=$((fail + 1))
		printf '  FAIL  %-40s the reason came back as %s lines; a journal line is one line.\n' \
			"$label" "$(printf '%s\n' "$stop_out" | grep -c .)"
		return 0
		;;
	esac
	if [ "$stop_rc" = "$want" ] &&
		{ [ -z "$want_in" ] || printf '%s' "$stop_out" | grep -qF -- "$want_in"; } &&
		{ [ -z "$want_out" ] || ! printf '%s' "$stop_out" | grep -qF -- "$want_out"; }; then
		pass=$((pass + 1))
		printf '  ok    %-40s rc=%s :: %s\n' "$label" "$stop_rc" "${stop_out:0:62}"
		return 0
	fi
	fail=$((fail + 1))
	printf '  FAIL  %-40s wanted rc=%s' "$label" "$want"
	[ -n "$want_in" ] && printf ', %s' "$want_in"
	[ -n "$want_out" ] && printf ', not %s' "$want_out"
	printf '; got rc=%s :: %s\n' "$stop_rc" "${stop_out:0:96}"
	return 0
}

# The #246 shape, transcribed: a turn opener, then the call, then the file ends. `exit=0`.
printf 'reading the queue now\n\033[0m$ \033[0mmake verify\nCLEAN\n> factory \302\267 default-model-fast\n<tool_call>\n<function=executae>\n<parameter=code>\nx\n</parameter>\n</function>\n' \
	>"$stop_dir/misspelled.log"
stop_leg 'a misspelled call names the name' "$stop_dir/misspelled.log" 0 'executae'

# The same markup one sentence later: prose, then the call. The name has to survive both shapes, and
# the second is the ordinary way a model reaches for a typo -- it says what it is about to do first.
printf '> factory \302\267 default-model-fast\nLet me re-read the ledger.\n<tool_call>\n<function=tools.opencode>\n<parameter=code>\nx\n</parameter>\n</function>\n</tool_call>\n' \
	>"$stop_dir/prose-then-call.log"
stop_leg 'prose then a call is still the cause' "$stop_dir/prose-then-call.log" 0 'tools.opencode'

# The #241 shape: the last thing in the file is the turn opener itself, 6411 lines of work ago.
printf 'check-decisions: 90 entries across 2 log(s)\nCLEAN\n> factory \302\267 default-model-fast\n' \
	>"$stop_dir/silent.log"
stop_leg 'a turn that emitted nothing' "$stop_dir/silent.log" 0 'emitted nothing'

# The polarity, in the shape the tick that opened this issue is written in. The CLI echoes the brief,
# and the brief QUOTES the failure -- `20261108T234413Z.unclaimed:247.log` carries nine `<function=`
# hits for exactly this reason, the earliest of them in the payload line at line 14. So the markup is
# planted at the TOP of the file and the last turn is a clean summary: a detector that reads the file
# instead of the final turn reports a misspelled tool call for a tick that misspelled nothing.
printf '<tool_call>\n<function=executae>\n<parameter=code>\nx\n</parameter>\n</function>\n\nso I will not repeat that mistake.\n> factory \302\267 default-model-fast\nReading the issue; the markup above is quoted at me, not typed by me.\n> factory \302\267 default-model-fast\nPushed `9f3c1a2`; pull request #247 is open and the reviewer is requested.\n' \
	>"$stop_dir/quoted.log"
stop_leg 'a brief quoting the failure' "$stop_dir/quoted.log" 1 'pull request #247 is open' 'not a tool'

# A delivered tick's last line, which is what arm (c) exists to quote rather than explain.
printf 'work done\n> factory \302\267 default-model-fast\nTwo things you should know rather than discover.\n' \
	>"$stop_dir/prose.log"
stop_leg 'ordinary prose is quoted, not diagnosed' "$stop_dir/prose.log" 1 'Two things you should know' 'not a tool'

# The two arms that refuse to invent a cause, and the sentence each owes.
stop_leg 'a missing log says it is missing' "$stop_dir/nothing-here.log" 2 'could not be read'
printf '' >"$stop_dir/empty.log"
stop_leg 'an empty log says it is empty' "$stop_dir/empty.log" 2 'no output at all'
# A log whose only content is turn openers has still ended on a silent turn, which is arm (b), and a
# log of nothing but newlines is not content at all -- the two refusals are different and each gets its
# own fixture, because "it said nothing" and "there was nothing to read" send a reader to different
# places.
printf '\n\n> factory \302\267 default-model-fast\n\n> factory \302\267 default-model-fast\n\n' \
	>"$stop_dir/openers.log"
stop_leg 'a log of only turn openers' "$stop_dir/openers.log" 0 'emitted nothing'
printf '\n \n\t\n' >"$stop_dir/blank.log"
stop_leg 'a log of only blanks' "$stop_dir/blank.log" 2 'no output at all'

# A run cut off in the middle of its call: the markup opens the turn and there is no closing tag to
# find. This is what makes the block's FIRST line a test rather than a second way of writing the same
# thing -- with only the closing-tag arm, a log that stops inside `<parameter=code>` reports prose.
printf 'looking at the ledger\n> factory \302\267 default-model-fast\n<tool_call>\n<function=executae>\n<parameter=code>\n' \
	>"$stop_dir/truncated.log"
stop_leg 'a call cut off mid-argument' "$stop_dir/truncated.log" 0 'executae'

# Hostile bytes where a tool name should be: two control characters, two hundred of one letter, and a
# trailing escape. The name goes on a journal line, so it is scrubbed, bounded, and never a second line.
long_name=$(printf 'x%.0s' $(seq 1 200))
printf '> factory \302\267 default-model-fast\n<tool_call>\n<function=ea\001\002b%sy\033[0m>\n</tool_call>\n' \
	"$long_name" >"$stop_dir/hostile.log"
stop_leg 'a hostile name is scrubbed' "$stop_dir/hostile.log" 0 'ea'
long_run=$(printf '%s' "$stop_out" | grep -c 'x\{61\}' || true)
ctl=$(LC_ALL=C printf '%s' "$stop_out" | grep -c '[[:cntrl:]]' || true)
if [ "${long_run:-0}" = "0" ] && [ "${ctl:-0}" = "0" ] && [ ${#stop_out} -le 230 ]; then
	pass=$((pass + 1))
	printf '  ok    %-40s no run past 60, no control byte, %s bytes\n' \
		"control: the name cannot run long" "${#stop_out}"
else
	fail=$((fail + 1))
	printf '  FAIL  %-40s %s bytes, long-run hits %s, control-byte lines %s.\n' \
		"control: the name cannot run long" "${#stop_out}" "$long_run" "$ctl"
fi

# The marker's wording, since which of the two sentences prints is the whole point of the change.
marker_leg() { # label fixture want_substring absent_substring
	local label=$1 f=$2 want=$3 absent=$4 out=''
	out=$(factory_stop_marker TICK-UNDELIVERED unclaimed:247 issue-247 "$f")
	case "$out" in
	*$'\n'*)
		fail=$((fail + 1))
		printf '  FAIL  %-40s the journal line is not one line.\n' "$label"
		return 0
		;;
	esac
	if printf '%s' "$out" | grep -qF -- "$want" && ! printf '%s' "$out" | grep -qF -- "$absent"; then
		pass=$((pass + 1))
		printf '  ok    %-40s %s\n' "$label" "${out:0:64}"
		return 0
	fi
	fail=$((fail + 1))
	printf '  FAIL  %-40s wanted %s and not %s; got %s\n' "$label" "$want" "$absent" "${out:0:96}"
	return 0
}
marker_leg 'an undelivered turn is refused' "$stop_dir/misspelled.log" \
	'exit=0 refused as completion' 'no log to read'
marker_leg 'a readable log is named for the reader' "$stop_dir/prose.log" 'see prose.log' \
	'no log to read'
marker_leg 'an ordinary stop keeps the plain verdict' "$stop_dir/prose.log" \
	'with no pull request on issue-247' 'refused as completion'
marker_leg 'an unreadable log points at nothing' "$stop_dir/nothing-here.log" \
	'no log to read' 'refused as completion'

# A branch that is empty has to be said, not printed as a space: `no pull request on  --` is the
# reading that sends somebody looking for a branch name that was never there.
no_branch=$(factory_stop_marker TICK-UNDELIVERED unclaimed:247 '' "$stop_dir/prose.log")
case "$no_branch" in
*'no pull request on <no branch>'*)
	pass=$((pass + 1))
	printf '  ok    %-40s the line names a missing branch instead of printing a hole\n' \
		'a stop with no branch'
	;;
*)
	fail=$((fail + 1))
	printf '  FAIL  %-40s an empty branch printed as a gap in the sentence: %s\n' \
		'a stop with no branch' "${no_branch:0:96}"
	;;
esac

# ---------------------------------------------------------------- the mirror of unpushed commits

mirror=$(mktemp -d "$work/mirror-XXXX")
git -c init.defaultBranch=main init -q --bare "$mirror/origin.git"
git --git-dir="$mirror/origin.git" config user.email selftest@example
git --git-dir="$mirror/origin.git" config user.name selftest
git -c init.defaultBranch=main init -q "$mirror/seed"
git -C "$mirror/seed" remote add origin "$mirror/origin.git"
git -C "$mirror/seed" -c user.email=selftest@example -c user.name=selftest \
	commit -q --allow-empty -m 'base: what origin/main holds'
git -C "$mirror/seed" push -q origin main
git -C "$mirror/seed" checkout -q -b issue-902
git -C "$mirror/seed" -c user.email=selftest@example -c user.name=selftest \
	commit -q --allow-empty -m 'the fix, committed'
git -C "$mirror/seed" -c user.email=selftest@example -c user.name=selftest \
	commit -q --allow-empty -m 'its decision entry'
git -C "$mirror/seed" checkout -q -b issue-903 origin/main
git -C "$mirror/seed" fetch -q origin

mirror_journal() { printf '%s/log/supervisor.log' "$mirror"; }
mirror_saved_log=$FACTORY_LOG
mirror_saved_risk=$FACTORY_AT_RISK
mirror_saved_path=$PATH
FACTORY_LOG=$mirror/log
FACTORY_AT_RISK=$mirror/at-risk
factory_paths

mirror_leg() { # label want_rc want(''none'|'!absent'|substring) unit tree branch
	local label=$1 want=$2 needle=$3 rc=0 match=1
	factory_preserve_unpushed "$4" "$5" "$6" >>"$mirror/run.out" 2>&1 || rc=$?
	case "$needle" in
	'') match=1 ;;
	'!'*)
		if grep -qF -- "${needle#!}" "$(mirror_journal)"; then match=0; fi
		;;
	*)
		if grep -qF -- "$needle" "$(mirror_journal)"; then match=1; else match=0; fi
		;;
	esac
	if [ "$rc" = "$want" ] && [ "$match" = "1" ]; then
		pass=$((pass + 1))
		printf '  ok    %-40s rc=%s :: %s\n' "$label" "$rc" \
			"$(tail -n 1 "$(mirror_journal)" | cut -c1-58)"
		return 0
	fi
	fail=$((fail + 1))
	printf '  FAIL  %-40s wanted rc=%s (%s); got rc=%s :: %s\n' "$label" "$want" \
		"${needle:-no journal line}" "$rc" "$(tail -n 1 "$(mirror_journal)" | cut -c1-70)"
	return 0
}

mirror_leg 'two commits ahead are mirrored' 0 'PRESERVED' unclaimed:902 "$mirror/seed" issue-902
bundle=$(grep -m1 PRESERVED "$(mirror_journal)" | sed -nE 's/.*PRESERVED ([^ ]+\.bundle).*/\1/p' || true)
if [ -n "$bundle" ] && git -C "$mirror/seed" bundle verify "$bundle" >/dev/null 2>&1; then
	pass=$((pass + 1))
	printf '  ok    %-40s git verifies it: %s bytes\n' "the bundle is a bundle" \
		"$(wc -c <"$bundle")"
else
	fail=$((fail + 1))
	printf '  FAIL  %-40s %s does not verify.\n' "the bundle is a bundle" "${bundle:-<no path in the line>}"
fi

# The form is part of the design, so it is pinned rather than implied: a prerequisite bundle is
# 18-28 KB for a real at-risk branch where full history is 17.6 MB, and full history would re-copy the
# 10 MB `plock` blob on every stop (#1264). `git bundle verify` says which form it holds by naming the
# ref it requires, so the leg asks git and not the author.
if [ -n "$bundle" ] &&
	git -C "$mirror/seed" bundle verify "$bundle" 2>&1 | grep -qF 'requires this ref'; then
	pass=$((pass + 1))
	printf '  ok    %-40s git names a prerequisite, so it is not a full re-copy\n' \
		'the bundle is a prerequisite bundle'
else
	fail=$((fail + 1))
	printf '  FAIL  %-40s git asked for no prerequisite -- that is a full-history bundle.\n' \
		'the bundle is a prerequisite bundle'
fi

# The recovery the journal line promises, run as written. This leg is what decided the bundle's form:
# a copy nobody can restore is a second copy of a feeling. Measured while writing it, `git clone
# <bundle>` on a machine holding nothing answers `error: Repository lacks these prerequisite commits`,
# so the shipped sentence is clone-the-remote-then-fetch, and that sentence is what runs here.
recovered=1
bref=$(grep -m1 PRESERVED "$(mirror_journal)" | sed -nE 's/.*refs\/heads\/([^`]+)`.*/\1/p' || true)
if [ -n "$bundle" ] && [ -n "$bref" ] &&
	git clone -q "$mirror/origin.git" "$mirror/recover" 2>"$mirror/clone.err" &&
	git -C "$mirror/recover" config user.email selftest@example &&
	git -C "$mirror/recover" config user.name selftest &&
	git -C "$mirror/recover" fetch "$bundle" "refs/heads/$bref" >"$mirror/fetch.out" 2>&1; then
	got=$(git -C "$mirror/recover" rev-parse FETCH_HEAD 2>/dev/null || printf none)
	want=$(git -C "$mirror/seed" rev-parse issue-902)
	n=$(git -C "$mirror/recover" rev-list --count main..FETCH_HEAD 2>/dev/null || printf 0)
	if [ "$got" = "$want" ] && [ "$n" = "2" ]; then recovered=0; fi
fi
if [ "$recovered" = "0" ]; then
	pass=$((pass + 1))
	printf '  ok    %-40s the tip it names restores the tip on the branch, 2 ahead of main\n' \
		"the sentence's own command works"
else
	fail=$((fail + 1))
	printf '  FAIL  %-40s clone plus the fetch the journal printed did not reproduce the tip.\n' \
		"the sentence's own command works"
	printf '          bundle: %s\n          clone stderr: %s\n          fetch: %s\n' \
		"${bundle:-<none>}" "$(tail -n 1 "$mirror/clone.err" | cut -c1-76)" \
		"$(tail -n 1 "$mirror/fetch.out" | cut -c1-76)"
fi

: >"$(mirror_journal)"
mirror_leg 'nothing ahead mirrors nothing' 1 '!PRESERVED' unclaimed:903 "$mirror/seed" issue-903
mirror_leg 'a branch that is gone is refused' 2 'PRESERVE-FAILED' unclaimed:900 "$mirror/seed" no-such-branch
mirror_leg 'no tree is refused, not guessed' 2 'no tree to read' unclaimed:899 "$mirror/gone" issue-902
mirror_leg 'no branch says which arm refused' 1 'PRESERVE-SKIPPED' unclaimed:898 "$mirror/seed" ''
mkdir -p "$mirror/ro" && chmod 500 "$mirror/ro"
FACTORY_AT_RISK=$mirror/ro/nope
mirror_leg 'an unusable mirror dir is said' 2 'could not be created' unclaimed:897 "$mirror/seed" issue-902
chmod 700 "$mirror/ro"
FACTORY_AT_RISK=$mirror/at-risk

# Two stops of one unit at two tips leave two files, not one written over the other: the second stop's
# commits are the ones nobody has read yet, and the unit is named the same both times.
n_before=$(find "$FACTORY_AT_RISK" -maxdepth 1 -type f -name '*.bundle' 2>/dev/null |
	wc -l | tr -d '[:space:]' || true)
git -C "$mirror/seed" checkout -q issue-902
git -C "$mirror/seed" -c user.email=selftest@example -c user.name=selftest \
	commit -q --allow-empty -m 'work the second stop did'
factory_preserve_unpushed unclaimed:902 "$mirror/seed" issue-902 >>"$mirror/run.out" 2>&1 || true
n_after=$(find "$FACTORY_AT_RISK" -maxdepth 1 -type f -name '*.bundle' 2>/dev/null |
	wc -l | tr -d '[:space:]' || true)
second=$(grep -c PRESERVED "$(mirror_journal)" || true)
newest=$(grep PRESERVED "$(mirror_journal)" | tail -n 1 | sed -nE 's/.*PRESERVED ([^ ]+\.bundle).*/\1/p' || true)
if [ "${n_after:-0}" -gt "${n_before:-0}" ] && [ "${second:-0}" -ge 1 ] &&
	[ -n "$newest" ] && git -C "$mirror/seed" bundle verify "$newest" >/dev/null 2>&1; then
	pass=$((pass + 1))
	printf '  ok    %-40s %s files, %s PRESERVED lines, newest verifies\n' \
		"a second stop is not written over the first" "$n_after" "$second"
else
	fail=$((fail + 1))
	printf '  FAIL  %-40s %s files before, %s after, %s PRESERVED lines.\n' \
		"a second stop is not written over the first" "${n_before:-0}" "${n_after:-0}" "${second:-0}"
fi

# The write is read back, so a file git will not verify is never called PRESERVED. The shim answers
# only `bundle verify` and hands `bundle create` to the real git, which is what makes the leg about
# the read-back rather than about the write.
: >"$(mirror_journal)"
shim=$(shim_install 'bundle verify' 1)
PATH="$shim:$PATH"
hash -r
mirror_leg 'a refused verify is not PRESERVED' 2 'PRESERVE-UNVERIFIED' unclaimed:896 "$mirror/seed" issue-902
mirror_leg 'and the word PRESERVED is not printed' 2 '!PRESERVED ' unclaimed:895 "$mirror/seed" issue-902
PATH=$mirror_saved_path
hash -r

FACTORY_LOG=$mirror_saved_log
FACTORY_AT_RISK=$mirror_saved_risk

# The default location, because a default that nests inside $FACTORY_TREES is a mirror `git worktree
# remove` can delete -- and the trees themselves are not under /tmp for exactly that lesson.
risk_default=$(env -u FACTORY_AT_RISK FACTORY_TREES=/a/b/wt bash -c \
	"FACTORY_DIR='$HERE'; . '$HERE/common.sh' >/dev/null 2>&1; printf '%s' \"\$FACTORY_AT_RISK\"")
if [ "$risk_default" = "/a/b/at-risk" ]; then
	pass=$((pass + 1))
	printf '  ok    %-40s FACTORY_TREES=/a/b/wt gives %s\n' "the mirror is beside the trees" \
		"$risk_default"
else
	fail=$((fail + 1))
	printf '  FAIL  %-40s FACTORY_TREES=/a/b/wt gave %s, not /a/b/at-risk.\n' \
		"the mirror is beside the trees" "${risk_default:-<nothing>}"
fi
risk_kept=$(env FACTORY_AT_RISK=/z/keep FACTORY_TREES=/a/b/wt bash -c \
	"FACTORY_DIR='$HERE'; . '$HERE/common.sh' >/dev/null 2>&1; printf '%s' \"\$FACTORY_AT_RISK\"")
if [ "$risk_kept" = "/z/keep" ]; then
	pass=$((pass + 1))
	printf '  ok    %-40s an explicit FACTORY_AT_RISK survives the derivation\n' \
		'the override is an override'
else
	fail=$((fail + 1))
	printf '  FAIL  %-40s the derivation printed over it: %s\n' 'the override is an override' \
		"${risk_kept:-<nothing>}"
fi

# The one place the two intents met, and the only arm of the merge that could silently lose a property
# someone had already paid for. #248 defines this box as outside `/tmp` because `/tmp/opencode/wt-204`
# and `wt-206` are gone that way; #247 derives it from wherever the trees live. Deriving alone lets one
# operator's `FACTORY_TREES=/tmp/…` carry the rescue into the exact directory that has already deleted
# unmerged work on this machine -- and it would do it quietly, because the box is by definition where
# nobody was looking. So the derivation refuses `/tmp`, a relative path, and an empty answer.
#
# The two legs control each other: the one above shows a normal trees path still derives (so this one
# cannot be passed by a function that always returns the home box), and this one shows the fallback
# fires for /tmp and a relative path and for nothing else.
risk_tmp=$(env -u FACTORY_AT_RISK FACTORY_TREES=/tmp/opencode/wt HOME=/home/fixture bash -c \
	"FACTORY_DIR='$HERE'; . '$HERE/common.sh' >/dev/null 2>&1; printf '%s' \"\$FACTORY_AT_RISK\"")
risk_rel=$(env -u FACTORY_AT_RISK FACTORY_TREES=../wt HOME=/home/fixture bash -c \
	"FACTORY_DIR='$HERE'; . '$HERE/common.sh' >/dev/null 2>&1; printf '%s' \"\$FACTORY_AT_RISK\"")
risk_plain=$(env -u FACTORY_AT_RISK FACTORY_TREES=/srv/queue/trees HOME=/home/fixture bash -c \
	"FACTORY_DIR='$HERE'; . '$HERE/common.sh' >/dev/null 2>&1; printf '%s' \"\$FACTORY_AT_RISK\"")
if [ "$risk_tmp" = "/home/fixture/prj/at-risk" ] &&
	[ "$risk_rel" = "/home/fixture/prj/at-risk" ] &&
	[ "$risk_plain" = "/srv/queue/at-risk" ]; then
	pass=$((pass + 1))
	printf '  ok    %-40s /tmp/opencode/wt and ../wt both give %s,\n' \
		'the mirror never lands where a reboot can' "${risk_tmp:-<nothing>}"
	printf '          and /srv/queue/trees still derives /srv/queue/at-risk\n'
else
	fail=$((fail + 1))
	printf '  FAIL  %-40s /tmp gave %s, ../wt gave %s, /srv/queue/trees gave %s.\n' \
		'the mirror never lands where a reboot can' "${risk_tmp:-<nothing>}" \
		"${risk_rel:-<nothing>}" "${risk_plain:-<nothing>}"
	printf '          The rescue must not sit in the directory that already deleted wt-204 and\n'
	printf '          wt-206, and it must still follow the trees when they are somewhere sane --\n'
	printf '          a fallback that always fires would pass the first two and fail the third.\n'
fi

# ------------------------------------------------- the shipped calls, and what deleting them costs
#
# A scan, not a drive: reaching either call site for real needs a gh read that answers "no pull
# request" and a model that fails in one of two shapes. What the scan cannot see is said here rather
# than implied by the ok -- it proves the marker is the thing whose output is logged, and that both
# stopping arms call the mirror. It does not prove the arms are reachable, and it would still pass if
# the marker were wired to print the wrong argument order. The legs above drive the functions.
stop_wiring() { # <factory.sh> -> every wiring gap in that copy
	local f=$1
	grep -qF 'log_event "$(factory_stop_marker TICK-UNDELIVERED' "$f" ||
		printf 'TICK-UNDELIVERED is logged without factory_stop_marker\n'
	[ "$(grep -cF 'factory_preserve_unpushed "$unit" "$tree" "$branch"' "$f")" -eq 2 ] ||
		printf 'fewer than two stopping arms mirror their commits\n'
	return 0
}
stop_wired() {
	local label='both stopping arms are wired' gaps
	gaps=$(stop_wiring "$HERE/factory.sh")
	if [ -n "$gaps" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the shipped file does not call what the legs above test:\n' "$label"
		printf '%s\n' "$gaps" | while IFS= read -r g; do printf '            %s\n' "$g"; done
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-40s the marker prints the line and both arms mirror\n' "$label"

	# The control, in the opposite polarity and planted outside the repository: both calls taken out
	# of a copy has to be reported, or the ok above is the check being blind.
	mkdir -p "$work/wired"
	sed -e 's|log_event "$(factory_stop_marker TICK-UNDELIVERED|log_event "TICK-UNDELIVERED $unit exit=0|' \
		-e '/factory_preserve_unpushed "\$unit" "\$tree" "\$branch" || true/d' \
		"$HERE/factory.sh" >"$work/wired/factory.sh"
	if [ -n "$(stop_wiring "$work/wired/factory.sh")" ]; then
		printf '        control: the marker call and both mirror calls, deleted from a copy, are\n'
		printf '        reported -- the ok above is the check working, not the check being blind.\n'
	else
		fail=$((fail + 1))
		printf '  FAIL  %-40s a copy with all three calls deleted read as wired.\n' "control: $label"
	fi
	return 0
}
stop_wired
PATH=$mirror_saved_path

# The mirror is a prerequisite bundle, so it is NOT a standalone clone, and the issue asked for the
# one thing that is therefore impossible: "`git clone` from that bundle reproduces the tip". Measured
# both ways on a clone of nothing. Pinned as a leg because the failure is the price of the form -- the
# form is what keeps a stop at 28 KB instead of 22 MB -- and the day somebody changes the form to full
# history to make the clone work, this is the line that says what was traded.
sc_rc=0
# The leg below asserts this clone FAILS. The two `config` lines are for the state it refuses: if the
# bundle ever becomes self-contained and the clone succeeds, the fixture still must not author under
# the host's identity (H-0059). A `config` against a directory git did not create is a no-op here --
# hence `|| true`, which is load-bearing and not decoration. They sit inside three lines of the clone
# because `pos_unconfigured_repos` skips comments, so prose above them buys them no room.
git clone "$bundle" "$mirror/selfcontained" >"$mirror/sc.out" 2>&1 || sc_rc=$?
git -C "$mirror/selfcontained" config user.email selftest@example 2>/dev/null || true
git -C "$mirror/selfcontained" config user.name selftest 2>/dev/null || true
if [ "$sc_rc" != "0" ] && grep -qF 'lacks these prerequisite commits' "$mirror/sc.out"; then
	pass=$((pass + 1))
	printf '  ok    %-40s clone-from-bundle answers %s\n' "the mirror is not self-contained" \
		"$(grep -m1 -oE '(error|fatal):.*' "$mirror/sc.out" | head -n 1 | cut -c1-46)"
else
	fail=$((fail + 1))
	printf '  FAIL  %-40s clone-from-bundle exited %s; the shipped sentence is now wrong.\n' \
		"the mirror is not self-contained" "$sc_rc"
	printf '          %s\n' "$(tail -n 1 "$mirror/sc.out" | cut -c1-100)"
fi

# ------------------------------------------------- the real failures, when this tree can see them
#
# The fixtures prove the detector reads the shape. Only the journal proves the shape is what a real CLI
# writes, so the two logs this issue is written about go through the same function whenever they are on
# disk. From a linked worktree `factory/log` is empty -- journals live in whichever tree ran the tick
# -- so this says which way it went instead of printing a silent ok.
stop_real() { # unit want_substring
	local unit=$1 want=$2 f
	for f in "$FACTORY_LOG"/*".$unit.log"; do
		[ -f "$f" ] || continue
		stop_leg "the real $(basename -- "$f" | cut -c1-8) log" "$f" 0 "$want"
		return 0
	done
	pass=$((pass + 1))
	printf '  ok    %-40s inert here: no *.%s.log in %s (%s file(s))\n' \
		"real log: $unit" "$unit" "$FACTORY_LOG" \
		"$(find "$FACTORY_LOG" -maxdepth 1 -type f 2>/dev/null | wc -l | tr -d '[:space:]' ||
			printf 0)"
	printf '        -- the fixtures above are the gate on this machine, and that is said here\n'
	printf '        rather than left as a silent ok.\n'
	return 0
}
stop_real 'unclaimed:246' 'executae'
stop_real 'unclaimed:241' 'emitted nothing'

# The mode of the scripts the timer execs, found the hard way by the sweep that came with this change.
#
# A mutation harness that replaces a file with `mv tmp file` restores the bytes and loses the
# executable bit: `awk > tmp && mv tmp factory.sh` wrote `factory/factory.sh` back at 0644, every leg
# still read green except two, and those two died with `Permission denied` -- a message that reads like
# a fixture bug and is really a tick that will never start. `make verify` runs these scripts by path or
# through `bash`, so nothing else in the gate notices, and `install-timer.sh` hands the unit a path to
# exec. Both halves are pinned: the file on this disk, and the mode in the index, because a worktree
# `chmod` is invisible to a fresh checkout and the hazard worth stopping is the one that gets committed.
worktree_exec_problems() { # <dir> <name>... -> the names that are not executable there
	local dir=$1 f bad=''
	for f in "${@:2}"; do
		[ -x "$dir/$f" ] || bad="$bad $f"
	done
	printf '%s' "${bad# }"
}
index_exec_problems() { # <repo> <name>... -> the names git does not hold at 100755
	local repo=$1 f mode bad=''
	for f in "${@:2}"; do
		mode=$(git -C "$repo" ls-files -s -- "factory/$f" 2>/dev/null | awk '{print $1}' || true)
		[ "$mode" = "100755" ] || bad="$bad $f(${mode:-untracked})"
	done
	printf '%s' "${bad# }"
}
exec_bits() { # the shipped four, in the tree and in the index
	local bad
	bad=$(worktree_exec_problems "$HERE" factory.sh triage.sh selftest.sh install-timer.sh)
	[ -n "$bad" ] && { printf 'not executable in the worktree: %s' "$bad"; return 1; }
	bad=$(index_exec_problems "$HERE/.." factory.sh triage.sh selftest.sh install-timer.sh)
	[ -n "$bad" ] && { printf 'not 100755 in the index: %s' "$bad"; return 1; }
	return 0
}
if bad_modes=$(exec_bits); then
	pass=$((pass + 1))
	printf '  ok    %-40s all four, in the worktree and in the index\n' "the timer's scripts are executable"
else
	fail=$((fail + 1))
	printf '  FAIL  %-40s %s -- the unit execs this path, and a tick that will not start looks\n' \
		"the timer's scripts are executable" "$bad_modes"
	printf '          like a machine that is idle.\n'
fi
mkdir -p "$work/execmodes"
cp "$HERE/factory.sh" "$work/execmodes/factory.sh" && chmod 644 "$work/execmodes/factory.sh"
if [ -n "$(worktree_exec_problems "$work/execmodes" factory.sh)" ]; then
	printf '        control: a 0644 copy outside the repository is reported by the same predicate, so\n'
	printf '        the ok above is the check working and not the check being blind.\n'
else
	fail=$((fail + 1))
	printf '  FAIL  %-40s a 0644 copy read as executable.\n' "control: the mode predicate"
fi
# ------------------------------------------------- the --jq programs the skill quotes
#
# Step 1 of `.opencode/skills/factory/SKILL.md` is quoted prose that a tick runs verbatim, and nothing
# in this repository had ever run it. Measured 10 Nov on gh 2.46.0 at `/usr/bin/gh`, the reviewer read
# exactly as shipped prints an error and exits 1 instead of answering who has been asked:
#
#     gh pr view 269 --json reviewRequests --jq '[.users[].login]'   ->  cannot iterate over: null  (1)
#     gh pr view 269 --json reviewRequests                          ->  {"reviewRequests":[]}
#
# `--json` hands `--jq` the WHOLE payload object, so the program has to name the field it asked for.
# `[.users[].login]` is the right program for the REST `/pulls/N/requested_reviewers` envelope, which
# wraps the same rows as `{"users":[…]}` -- what `factory/factory.sh:268` and `factory/triage.sh:283`
# read -- and it had been copied out of that context into this one. Issue #270.
#
# The payload is DERIVED from the same line's own `--json` list rather than keyed by the program,
# because a fixture keyed by the program can be invented to fit: `('.users', '{"users":[…]}')` is a
# green pair and still the bug. Deriving it lets the leg ask the question the API asks -- is every
# field this program names one that the call actually requested?
#
# Canned, never live, which is #270's own condition and H-0054's standing rule: a leg that calls the
# API reads green on a signed-in box and red wherever `gh auth status` has no token. `jq` is not a new
# requirement on this file -- `claim_graphql()` runs it unconditionally mid-suite, and shipped
# `factory/common.sh` calls it at :536, :544 and :618. The values are a live read of this repository's
# own API on 10 Nov (PR 273, which holds an outstanding request; PR 269, which does not), not a shape
# guessed from documentation, and the unmeasured-field refusal below is what stops anyone guessing the
# next one.
#
# What the scan refuses, in order, each with a planted control of its own:
#   * a `--jq` inside a fenced block that cannot be read as a program -- coverage cannot then be
#     narrowed in silence by writing the next program in a shape the leg skips;
#   * a field the program reads that the line's own `--json` list never asks for. That is the general
#     shape of the #270 bug, and it is also the arm that catches the QUIET variants: `.foo | length`
#     on an absent `foo` answers `0`, exits 0, and reports "nobody has been asked";
#   * a field with no measured fixture, which is a live read a person makes and not something this
#     file invents;
#   * a program that errors, prints `null`, or prints an empty shape against a payload that HOLDS a
#     row. The last one is why the fixture is not empty: `docs/factory.md:124` and
#     `factory/triage.sh:264` both record that `reviewRequests` lists only OUTSTANDING requests, so a
#     wrong program answering `[]` cannot be told apart from "asked and answered" -- and that
#     difference is what decides whether the supervisor nags at a reviewer or stays quiet.
skill_jq_fixture() { # <field> <rows|none> -> the value gh hands `--json <field>`
	case "$1:$2" in
	'reviewRequests:rows') printf '[{"__typename":"User","login":"fixture-reviewer"}]' ;;
	'reviewRequests:none') printf '[]' ;;
	'reviewDecision:'*) printf '"CHANGES_REQUESTED"' ;;
	'mergeable:'*) printf '"MERGEABLE"' ;;
	'mergeStateStatus:'*) printf '"BLOCKED"' ;;
	'headRefOid:'*) printf '"2a4f836237cb000ce18f9603a6953c9b24cb8ce9"' ;;
	# One row, and it is the measured one: every check on PR 273 concluded SUCCESS, so a red row here
	# would be a shape invented rather than read. A program that selects for a conclusion the fixture
	# does not hold is the one false refusal this table can produce, and the refusal says so.
	'statusCheckRollup:rows')
		printf '[{"__typename":"CheckRun","name":"verify","status":"COMPLETED","conclusion":"SUCCESS"}]' ;;
	'statusCheckRollup:none') printf '[]' ;;
	*) return 1 ;;
	esac
}

skill_jq_payload() { # <comma-separated fields> <arm> -> the object gh would hand `--jq`
	local fields=$1 arm=$2 f out='{}' v
	for f in ${fields//,/ }; do
		v=$(skill_jq_fixture "$f" "$arm") || return 1
		out=$(printf '%s' "$out" | jq -c --arg k "$f" --argjson v "$v" '. + {($k): $v}') || return 1
	done
	printf '%s' "$out"
}

# The scan's universe: the `--jq` lines inside fenced blocks, and only those. Line ~110 mentions the
# flag in the prose that tells a tick how to write one, and a check that refused the document for
# describing itself teaches whoever reads the next red tick that this check is noise.
skill_jq_lines() { # <file> -> <lineno><TAB><command> for every fenced line carrying --jq
	awk '/^```/ { fence = !fence; next } fence && /--jq/ { printf "%d\t%s\n", NR, $0 }' "$1"
}

skill_jq_gaps() { # <skill-file> -> one line per gap; 2 when the file is not here
	local file=$1 entry lineno cmd prog fields head f arm out rc tail_c bad shown
	# `shown` in its own statement: `local file=$1 shown=${file##*/}` expands every word before it
	# assigns any of them, so under `set -u` the second one reads `file` while it is still unset.
	shown=${file##*/}
	[ -f "$file" ] || return 2
	while IFS= read -r entry; do
		lineno=${entry%%$'\t'*}
		cmd=${entry#*$'\t'}
		# Single-quoted is the shape the document teaches; a bare token is the shape
		# `factory/common.sh:681` uses. Both are readable. Anything else is refused rather than
		# skipped, because a scan that skips is a scan whose coverage shrinks one edit at a time.
		# The leading character decides, BEFORE the bare form is tried: `--jq "[.a[].b]"` read as a
		# bare token hands jq `"[.a[].b]"`, which jq takes as a string LITERAL -- it prints the text,
		# exits 0, and looks answered. That is found by writing the control, not the arm: the plant
		# that puts the double quotes in read CLEAN until the quote was looked at first.
		prog=$(printf '%s' "$cmd" | sed -nE "s/.*--jq[[:space:]]+'([^']*)'.*/\1/p")
		if [ -z "$prog" ]; then
			tail_c=$(printf '%s' "$cmd" | sed -E 's/.*--jq[[:space:]]*//' | cut -c1)
			case "$tail_c" in
			'' | '`' | ',' | ')' | ']' | ';') continue ;;
			'"')
				printf '%s:%s  a double-quoted --jq program; single-quote it -- bash expands the\n          double form, so `--jq "[.a]|index($who)"` stops being the program it shows\n' \
					"$shown" "$lineno"
				continue ;;
			"'")
				printf '%s:%s  an --jq program that does not close on its own line\n' "$shown" "$lineno"
				continue ;;
			esac
			prog=$(printf '%s' "$cmd" | sed -nE 's/.*--jq[[:space:]]+([^[:space:]]+).*/\1/p')
			# A bare token has to LOOK like a jq program, or a `\` continuation is reported as the
			# unreadable thing it is instead of being run as a program nobody wrote.
			case "$prog" in
			'.'* | '['* | '{'* | [a-zA-Z_]*) ;;
			*)
				printf '%s:%s  an --jq program the scan cannot read (it starts `%s`); put the whole\n          read on one line, with the program in single quotes\n' \
					"$shown" "$lineno" "$tail_c"
				continue ;;
			esac
			case "$prog" in
			*'$'* | *'`'*)
				printf '%s:%s  an unquoted --jq program holding %s -- the shell expands it before gh\n          ever sees the program; single-quote it\n' \
					"$shown" "$lineno" '$'
				continue ;;
			esac
		fi
		fields=$(printf '%s' "$cmd" | sed -nE 's/.*--json[[:space:]]+([^[:space:]]+).*/\1/p')
		if [ -z "$fields" ]; then
			printf '%s:%s  a --jq on a line with no --json, so there is no payload to derive\n' \
				"$shown" "$lineno"
			continue
		fi
		bad=''
		for f in ${fields//,/ }; do
			skill_jq_fixture "$f" rows >/dev/null || bad="$bad $f"
		done
		if [ -n "$bad" ]; then
			printf '%s:%s  no measured fixture for --json%s -- read it live off a real pull request\n          and add the row here; a program pinned to a guessed payload asserts nothing\n' \
				"$shown" "$lineno" "$bad"
			continue
		fi
		head=$(printf '%s' "$prog" | grep -oE '\.[A-Za-z_][A-Za-z0-9_]*' | head -n 1 | sed 's/^\.//' ||
			true)
		if [ -z "$head" ]; then
			printf '%s:%s  the program reads no field, so nothing pins what it looked at\n' "$shown" "$lineno"
			continue
		fi
		case ",$fields," in
		*,"$head,"*) ;;
		*)
			printf '%s:%s  the program reads .%s, which --json %s never asks for -- gh hands --jq only\n          the fields listed, so .%s is the null that fails to iterate (#270)\n' \
				"$shown" "$lineno" "$head" "$fields" "$head"
			continue ;;
		esac
		for arm in rows none; do
			if ! out=$(skill_jq_payload "$fields" "$arm"); then
				printf '%s:%s  the %s payload could not be built\n' "$shown" "$lineno" "$arm"
				continue
			fi
			rc=0
			out=$(printf '%s' "$out" | jq -r "$prog" 2>&1) || rc=$?
			if [ "$rc" -ne 0 ]; then
				printf '%s:%s  exits %d on the %s payload: %s\n' "$shown" "$lineno" "$rc" "$arm" \
					"$(printf '%s' "$out" | head -n 1 | cut -c1-88)"
				continue
			fi
			case "$out" in
			*null*)
				printf '%s:%s  prints null on the %s payload, which a tick reads as an answer\n' \
					"$shown" "$lineno" "$arm"
				continue ;;
			esac
			if [ "$arm" = rows ]; then
				case "$out" in
				'' | '[]' | '{}' | '0' | 'false')
					printf '%s:%s  answers %s against a payload that holds a row -- either it reads a\n          field the payload does not carry (the #270 shape) or the fixture holds no row it\n          selects, and a wrong program and an honest "nobody was asked" are one sentence here\n' \
						"$shown" "$lineno" "'$out'" ;;
				esac
			fi
		done
	done < <(skill_jq_lines "$file")
	return 0
}

# One plant, in the opposite polarity to an arm, written outside the repository: an ok that has never
# seen its own refusal is the check being blind and not the check working (the rule this file already
# applies to every text leg).
skill_jq_control() { # <name> <sed program> <what the arm is>
	local name=$1 script=$2 arm=$3 copy gaps
	copy=$work/skilljq/ctl.md
	if ! sed -E "$script" "$skill_jq_doc" >"$copy" 2>/dev/null; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant could not be written.\n' "control: $name"
		return 0
	fi
	# The plant proves it fired before its result means anything (H-0055's finding (3), which came out
	# of a sweep where four mutations reported `0 failed` and none of them had applied). A copy equal to
	# the original says the document's wording moved, not that the arm works -- and it would otherwise
	# read as the SAME green the arm produces.
	if cmp -s "$copy" "$skill_jq_doc"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant matched nothing in this document, so\n' "control: $name"
		printf '          it proves nothing about the %s. The wording moved; move the plant with it.\n' "$arm"
		return 0
	fi
	gaps=$(skill_jq_gaps "$copy")
	if [ -z "$gaps" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the copy it mutated read as CLEAN, so the\n' "control: $name"
		printf '          leg above pins a phrase and not the %s.\n' "$arm"
		return 0
	fi
	printf '        control: %-28s -> %s\n' "$name" \
		"$(printf '%s' "$gaps" | head -n 1 | sed -E 's/^ctl\.md:[0-9]+ +//; s/ +-- .*//' | cut -c1-88)"
}

skill_jq_leg() { # the document a tick actually reads, and the three refusals
	local label='the --jq programs the skill quotes' gaps ran
	skill_jq_doc=$HERE/../.opencode/skills/factory/SKILL.md
	if [ ! -f "$skill_jq_doc" ]; then
		printf '  SKIP  %-40s %s is not in this tree\n' "$label" '.opencode/skills/factory/SKILL.md'
		printf '          -- the fixture trees have no `.opencode/`; CI checks out the real one.\n'
		return 0
	fi
	ran=$(skill_jq_lines "$skill_jq_doc" | wc -l | tr -d '[:space:]')
	gaps=$(skill_jq_gaps "$skill_jq_doc")
	if [ -n "$gaps" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s a quoted program does not answer, in %s:\n' \
			"$label" "${skill_jq_doc#"$HERE"/../}"
		printf '%s\n' "$gaps" | while IFS= read -r g; do printf '            %s\n' "$g"; done
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-40s %s program(s) run against the payload gh hands them,\n' "$label" "$ran"
	printf '          every field they read is one the call asks for, and neither is null\n'

	mkdir -p "$work/skilljq"
	before=$fail
	skill_jq_control 'the field name, back-revolved' \
		's|\.reviewRequests\[\]\.login|.users[].login|' 'field-name check'
	skill_jq_control 'the quiet one, an absent leaf' \
		's|\[\.reviewRequests\[\]\.login\]|[.reviewRequests[].isOutstanding // empty]|' 'empty-answer check'
	skill_jq_control 'a program the scan cannot read' \
		"s|--jq '[^']*'|--jq \"[.reviewRequests[].login]\"|" 'coverage of unreadable programs'
	if [ "$fail" = "$before" ]; then
		printf '        All three are planted in a copy outside the repository and reported, so the ok\n'
		printf '        above is each arm discriminating, not each arm being blind.\n'
	fi
	return 0
}
skill_jq_leg

printf 'factory/selftest.sh: %d passed, %d failed\n' "$pass" "$fail"
reached_verdict=1
if [ "$fail" -ne 0 ]; then
	exit 1
fi
