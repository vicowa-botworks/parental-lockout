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

# halt_pair_problems TICK_SENTENCE TICK_LINES CONFLICT_SENTENCE CONFLICT_LINES
#   One line per way the two markers the queue leaves behind differ from each other; empty when the
#   operator is handed the same instruction whichever path stopped. #237 item 1 asked for that
#   property and this file never held it: `expect_halt` read the tick's marker and `rm -f`'d the
#   conflict pass's unread, on the argument that the guard is one `printf` so the two cannot differ.
#   That is true of this revision and is the shape of the argument that let them differ in the first
#   place -- H-0044's own entry records the conflict arm writing a marker "with neither the
#   UNKNOWN-versus-DIRTY distinction nor the matching action", and the sentence in #237's `:339` ends
#   at the tree's name, so the operator of a queue stopped by the merge path reads where the trouble
#   is and not how to resume it. Eleven hours is what that cost.
#
#   The strings arrive already joined by `tr` (one line, for the leg's own message), so a file that
#   split its sentence in two is invisible in the string and the LINE COUNTS are the only thing that
#   can see it. That arm is last and only reached when the strings agree, because when they differ
#   the disagreement is the finding and the counts are its symptom.
halt_pair_problems() {
	local s1=$1 n1=$2 s2=$3 n2=$4
	if [ -z "$s1" ] && [ -n "$s2" ]; then
		printf 'the tick left no HALT file while the conflict pass did (%s), so the two paths disagree about whether the queue is stopped\n' "$s2"
	elif [ -n "$s1" ] && [ -z "$s2" ]; then
		printf 'the conflict pass left no HALT file while the tick did (%s), so a queue stopped by the merge path leaves nothing for factory_halted() to find\n' "$s1"
	elif [ "$s1" != "$s2" ]; then
		printf 'the two halts leave the operator different markers -- tick: %s | conflict: %s\n' "$s1" "$s2"
	elif [ "$n1" != "$n2" ]; then
		printf 'the two HALT files are %s and %s lines, and the sentences match only because `tr` joined them\n' "$n1" "$n2"
	fi
	return 0
}

# expect_halt LABEL TREE WANT [SUBSTRING]
#   WANT is PROCEED or HALT -- what the supervisor does with the unit. A HALT leg requires all of:
#   exit 4 from the tick's verb, exit 2 from the conflict pass's, a HALT file (because that is what
#   `factory_halted()` tests), the `rm …` resume step inside it, the action that matches the finding,
#   and one journal line per path carrying that path's own verb. BOTH markers are read -- the tick's
#   and the conflict pass's -- before either is deleted, and the HALT arm requires the two to be the
#   same bytes: #237 item 1 is that the two halt writers cannot leave the operator different
#   sentences. A PROCEED leg requires the guard to
#   have halted neither and to have written neither file: a halt that fires on a clean tree is the
#   false positive #228 was filed about, and it stops every other unit in the pass.
#   SUBSTRING is asserted inside the halt file's sentence -- the finding has to reach the operator,
#   not just the return code.
expect_halt() {
	local label=$1 tree=$2 want=$3 sub=${4:-}
	local c1=0 c2=0 h1="" h2="" hl=0 hl2=0 j1="" j2="" before=0 after=0 bad="" want_how drc=0
	local pair=""

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
	# The conflict pass's marker is read before it is removed. It used to be deleted unread, on the
	# argument that the guard holds one `printf` so the two paths cannot disagree -- a statement about
	# this revision's source, not a property this file keeps, and #237 is the record of the two having
	# differed (`H-0044`: "the conflict pass wrote a HALT file with neither the UNKNOWN-versus-DIRTY
	# distinction nor the matching action", and the ticket's `:339` sentence ends at the tree's name
	# with no `rm factory/HALT` anywhere in it). Equality with the tick's sentence is asserted below;
	# `tr` above would fold a split sentence back into the same string, so the line count is asked of
	# this marker as well rather than inferred from the tick's.
	if [ -e "$FACTORY_HALT" ]; then
		h2=$(tr '\n' ' ' <"$FACTORY_HALT")
		hl2=$(wc -l <"$FACTORY_HALT" | tr -d '[:space:]')
	fi
	if [ -s "$FACTORY_LOG/supervisor.log" ]; then j2=$(tail -n 1 "$FACTORY_LOG/supervisor.log"); fi
	rm -f -- "$FACTORY_HALT"
	if [ -f "$FACTORY_LOG/supervisor.log" ]; then
		after=$(wc -l <"$FACTORY_LOG/supervisor.log")
	fi

	if [ "$want" = PROCEED ]; then
		[ "$c1" = 0 ] || bad="the tick was stopped (exit $c1)"
		[ "$c2" = 0 ] || bad="${bad:+$bad; }the conflict pass was stopped (exit $c2)"
		[ -z "$h1" ] || bad="${bad:+$bad; }a HALT file was written, so every other unit is stopped too"
		[ -z "$h2" ] || bad="${bad:+$bad; }the conflict pass wrote a HALT file for a tree it let through"
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
		# The PAIR, not only the tick's. #237 item 1 is about the marker the *conflict* pass leaves --
		# the one an operator finds when it was the merge path that stopped -- and the two writers did
		# once disagree (H-0044: "the conflict pass wrote a HALT file with neither the UNKNOWN-versus-
		# DIRTY distinction nor the matching action", and the ticket's :339 sentence ends at the tree's
		# name). The comparison is a function of its own so the plant below can try the same code on a
		# disagreement it builds; a leg cannot prove a comparison that only exists inline.
		pair=$(halt_pair_problems "$h1" "$hl" "$h2" "$hl2")
		# No `tr` over the report: the helper prints at most one line (its arms are an if-chain) and the
		# command substitution has already taken the newline off. Folding and deleting separators here
		# would eat every semicolon inside the two markers, and a message that silently rewrites the
		# sentence it is quoting is one fewer thing an operator can trust.
		[ -z "$pair" ] || bad="${bad:+$bad; }$pair"
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

# --------------------------------------- the marker each verb leaves in the operator's hand (#237)
#
# `#237` counts two halt writers that left the operator different sentences. The conflict one is in the
# ticket: `a working tree was found dirty and the supervisor will not touch it: <tree>` -- the tree, and
# nothing about how to resume. The tick's ended `… resolve the tree (commit it to its branch, or discard
# it on purpose) … rm factory/HALT …`. The queue was stopped for eleven hours on the first of those,
# because the marker is the whole of what whoever finds a halted queue reads before they act, and it
# does not say what to do.
#
# H-0044 unified the writers, so both verbs reach the same `printf` in `factory_tree_halt` today. That
# is a property of this revision's source and not one this file kept: `expect_halt` drove both verbs and
# read the marker from the tick's call only, then `rm -f`'d the conflict pass's unread -- its own comment
# argued that "reading the same string twice would buy a leg that can only fail twice". The premise is
# today's source; the conclusion is a suite that stays green while the merge path hands out a different
# instruction, which is the state #237 was filed for and which H-0044's entry describes in the past
# tense. A drift cannot be caught by a leg that does not look at the drifted thing.
#
# So the comparison is a function (`halt_pair_problems`, above) and the legs that already drive both
# verbs now run it. What is here is the evidence that it can say the other word. The plants are
# behavioural, not source mutations, and that is the only shape that works: one edit to the shared
# `printf` changes both verbs at once, so the tick's own read fires first and nothing is learned about
# the conflict read. To prove the conflict marker is consulted, the two verbs have to be made to
# disagree -- so the shipped guard runs unchanged for both verbs and the conflict verb's marker is then
# rewritten, exactly as a second conflict-specific writer would. Same tree, same guard, same order; the
# only difference between the four rows is what is in the file afterwards.
po_s='' po_n=0
halt_pair_of() { # verb tree -> the marker that verb leaves, in $po_s (joined) and $po_n (lines)
	po_s='' po_n=0
	rm -f -- "$FACTORY_HALT"
	factory_tree_halt "$1" selftest "$2" >"$work/guard.out" 2>&1 || true
	if [ -e "$FACTORY_HALT" ]; then
		po_s=$(tr '\n' ' ' <"$FACTORY_HALT")
		po_n=$(wc -l <"$FACTORY_HALT" | tr -d '[:space:]')
	fi
	return 0
}

# halt_pair_planted MODE TREE -> `halt_pair_problems` over the pair the SHIPPED guard leaves, with MODE
# applied to the conflict pass's marker afterwards. `same` is the shipped state and the assertion; the
# other three are the arms, each named by the needle its caller requires.
halt_pair_planted() { # mode tree
	local mode=$1 tree=$2 s1='' n1=0 s2='' n2=0
	halt_pair_of TICK-HALTED "$tree"
	s1=$po_s
	n1=$po_n
	halt_pair_of CONFLICT-HALTED "$tree"
	s2=$po_s
	n2=$po_n
	case "$mode" in
	drift)
		# The ticket's `:339`, verbatim in shape, written over the marker the shipped guard just made:
		# the tree, and no resume step. This is what the pre-H-0044 world looks like, and what any
		# future conflict-specific writer reintroduces.
		printf '%s\n' "a working tree was found dirty and the supervisor will not touch it: $tree" \
			>"$FACTORY_HALT"
		;;
	vanished)
		# Halts the queue with the exit code and writes nothing: `factory_halted()` is a file test, so
		# the next unit walks past a halt that stopped nothing.
		rm -f -- "$FACTORY_HALT"
		;;
	nonewline)
		# The one state where the two sentences are the same WORDS and the files are not: a `printf` that
		# loses its `\n`. `tr` cannot see it -- both join to the same string -- so it is the count arm's
		# own plant, and the reason the counts are asked instead of inferred from the sentence.
		printf '%s' "$s2" >"$FACTORY_HALT"
		;;
	same) : ;;
	esac
	if [ -e "$FACTORY_HALT" ]; then
		s2=$(tr '\n' ' ' <"$FACTORY_HALT")
		n2=$(wc -l <"$FACTORY_HALT" | tr -d '[:space:]')
	else
		s2=''
		n2=0
	fi
	halt_pair_problems "$s1" "$n1" "$s2" "$n2"
}

halt_markers_agree() {
	local label='both halts leave one marker' tree=$repo out="" bad="" mode="" needle=""
	local body="" stripped="" plant_out="" got=""

	reset_tree "$tree"
	printf 'ALPHA\nbeta\ngamma\n' >"$tree/tracked.txt"

	# The shipped state: one instruction, whichever path stopped. Everything the operator needs has to
	# be in the marker the merge path leaves, and this is the row that says it is. Written to a file
	# rather than a command substitution on purpose: a substitution runs the plant in a subshell and the
	# marker the plant left is held in `$po_s`/`$po_n`, which the parent would never see -- measured,
	# the row below reading an empty pair and reporting "nothing to compare" for a run that had written
	# both markers.
	plant_out=$work/halt-pair-same.out
	halt_pair_planted same "$tree" >"$plant_out"
	out=$(cat -- "$plant_out")
	if [ -n "$out" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s the two halt verbs leave different things to read:\n' "$label"
		printf '%s\n' "$out" | sed 's/^/          /'
	elif [ -z "$po_s" ] || [ "$po_n" != 1 ]; then
		# The pair check says "no disagreement" just as happily when neither verb wrote anything. A leg
		# whose ok means "identical" has to refuse the identical-and-absent case or it is certifying an
		# empty pair, and po_s/po_n are the conflict pass's own last read.
		fail=$((fail + 1))
		printf '  FAIL  %-34s the check is silent because there is nothing to compare: the conflict\n' "$label"
		printf '          pass left a %s-line marker holding %s\n' "${po_n:-nothing}" "${po_s:-<empty>}"
	else
		pass=$((pass + 1))
		printf '  ok    %-34s one sentence, %s line, both verbs\n' "$label" "$po_n"
	fi

	# The arms, in the polarity every check in this file carries: each plant has to be named by the arm
	# it is here for, or the ok above is the comparison being blind.
	for mode in drift vanished nonewline; do
		case "$mode" in
		drift) needle='different markers' ;;
		vanished) needle='left no HALT file' ;;
		nonewline) needle='lines, and the sentences match' ;;
		esac
		out=$(halt_pair_planted "$mode" "$tree")
		case "$out" in
		*"$needle"*) : ;;
		*) bad="${bad:+$bad; }$mode: ${out:-<no problem reported>}" ;;
		esac
	done
	if [ -n "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s a planted disagreement was not flagged, so the ok above is the\n' "control: $label"
		printf '          comparison being blind, not the two verbs agreeing:\n'
		printf '            %s\n' "$bad"
	else
		printf '        control: the marker rewritten with the ticket'"'"'s resume-less sentence, the\n'
		printf '        marker deleted, and the marker written without its final newline are each named\n'
		printf '        by their own arm, so the ok above is the comparison separating the states.\n'
	fi

	# The other half of the same obligation: the legs have to actually run this. Twenty legs drive both
	# verbs over planted trees and none of them says out loud that the pair is what they check, so the
	# wiring is asked of the shipped text -- with a control that deletes the call and requires the
	# question to change its answer.
	body=$(awk '/^expect_halt\(\) \{/{f=1} f{print} f && /^\}/{exit}' "$HERE/selftest.sh")
	stripped=$work/halt-pair-unwired.sh
	printf '%s\n' "$body" | sed '/halt_pair_problems/d' >"$stripped"
	if ! printf '%s\n' "$body" | grep -q 'halt_pair_problems'; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s expect_halt never calls it, so every leg above that drives both\n' "wiring: $label"
		printf '          verbs still reads the tick'"'"'s marker only and this whole section is prose.\n'
	elif printf '%s\n' "$stripped" | grep -q 'halt_pair_problems'; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s the scan still finds the call after every line naming it is\n' "control: wiring: $label"
		printf '          deleted, so it matches something else and its ok means nothing.\n'
	else
		printf '        wiring: every expect_halt leg runs the pair check, and deleting the call is\n'
		printf '        reported by the same predicate (the stripped copy is %s).\n' "${stripped##*/}"
	fi

	reset_tree "$tree"
	rm -f -- "$FACTORY_HALT"
	return 0
}
halt_markers_agree

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
	UNREADABLE)
		# #267's third arm: `factory_tree_dirt` could not answer the halt, answered the action, and the
		# action said clean. Nothing was quarantined and nothing may be -- but NEITHER sentence this arm
		# used to own is true of the state, which is the whole defect. "was found dirty" claims a verdict
		# that never happened; "nothing was dirty" claims a clean nobody read.
		want_rc=0
		[ "$rc" = 0 ] || bad="the unit was stopped (exit $rc)"
		[ ! -e "$FACTORY_HALT" ] ||
			bad="${bad:+$bad; }an unreadable verdict beside a clean action halted the queue"
		[ "$after_mirrors" = "$before_mirrors" ] ||
			bad="${bad:+$bad; }it wrote a mirror for a tree the action called clean"
		[ "$(q_dirt "$tree")" = "$before" ] ||
			bad="${bad:+$bad; }it changed a tree it could not read: $before path(s) before, $(q_dirt "$tree") after"
		q_journal_has 'cause=verdict-unreadable' ||
			bad="${bad:+$bad; }the journal took no arm, so a pass that could not read the tree left no trace of it"
		q_journal_has 'cause=never-dirty' &&
			bad="${bad:+$bad; }an unreadable verdict journalled as 'nothing was dirty' -- a clean verdict from a tree nobody read"
		q_journal_has 'cause=verdict-disagreed' &&
			bad="${bad:+$bad; }an unreadable verdict journalled as a disagreement -- the line claims a dirty verdict it never got"
		# And the reason has to be IN the line: an arm name with nothing behind it is #267's second
		# unestablished point in its own words, "the line reports a cause it does not know". The needle is
		# the VERDICT's own sentence, the one this leg planted -- not a word from this file. Measured the
		# first time this leg ran: the check searched for "could not be read", the shipped sentence says
		# "the dirt verdict was unreadable and the action found nothing", and so the check passed with the
		# reason deleted from the line -- a leg satisfied by its own prose, which is the exact thing every
		# HALT leg here names its arm to avoid. `its state could not be read` appears nowhere else.
		case "$(grep -F 'cause=verdict-unreadable' "$FACTORY_LOG/supervisor.log" | tail -n 1)" in
		*'its state could not be read'*) : ;;
		*) bad="${bad:+$bad; }the line names no reason from the verdict, so the journal still cannot be read for its cause" ;;
		esac
		;;
	PROCEED)
		want_rc=0
		[ "$rc" = 0 ] || bad="the unit was stopped (exit $rc)"
		[ ! -e "$FACTORY_HALT" ] || bad="${bad:+$bad; }a HALT file was written for a tree with nothing in it"
		[ "$after_mirrors" = "$before_mirrors" ] || bad="${bad:+$bad; }a clean tree still produced a mirror"
		# Both halves, because the missing one was the bug (#267). One `TREE-CLEAN` search was
		# satisfied by the DISAGREEMENT arm, and this leg's tree -- the leg is literally labelled
		# "nothing was dirty" -- is the tree that used to take it. So the ok below read "the journal
		# never says which arm it took" as satisfied while the journal was saying a rescue had
		# happened. `cause=` names the ARM, so the prose above it can be reworded without this leg
		# silently going green or red.
		q_journal_has 'cause=never-dirty' ||
			bad="${bad:+$bad; }the healthy path took no arm the journal names, so nothing here says which one it took"
		q_journal_has 'cause=verdict-disagreed' &&
			bad="${bad:+$bad; }a tree that was never dirty journalled a disagreement -- the rescue log claims a rescue nobody performed"
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
# A ONE-SHOT shim: it refuses the first matching git invocation and lets every later one through.
# `shim_install` above is stateless, and statelessness is exactly what will not reach #267's third arm.
# That arm's state is "the verdict the halt read could not be read, the verdict the action read said
# clean" -- a difference between TWO reads of one tree, which no on-disk fixture can hold still. A
# stateless shim fails both reads, the quarantine then refuses, and the pass takes the refusal arm
# instead. So the counter lives beside the shim and one install owns it.
shim_install_first() { # match [exit] -> a bin dir to put first on PATH
	local match=$1 code=${2:-128} dir=$work/arm-shim-one real
	real=$(command -v git)
	# Wiped, not `mkdir -p`d over. The counter is the whole difference between this shim and
	# `shim_install`, so a marker left by the previous install silently turns the NEXT leg's plant into
	# an inert shim -- measured the first time this ran: the primary tree took `cause=verdict-unreadable`
	# and the worktree copy of the same leg answered `cause=never-dirty`, because the leg before it had
	# already spent the refusal. Same rule as `q_arm`'s plant-inside-the-helper, one mechanism over.
	rm -rf -- "$dir"
	mkdir -p "$dir"
	cat >"$dir/git" <<SHIM
#!/bin/sh
# Answers the FIRST matching invocation only; every later call is the real binary, untouched.
case " \$* " in
*" $match "*)
	if [ ! -e "$dir/answered" ]; then
		: >"$dir/answered"
		printf 'shim: %s said something and exited %s\n' '$match' '$code' >&2
		exit $code
	fi
	;;
esac
exec '$real' "\$@"
SHIM
	chmod +x "$dir/git"
	printf '%s' "$dir"
}

# q_unreadable LABEL TREE WANT MATCH
#   WANT is UNREADABLE for the plant and PROCEED for the inert control, and both go through the same
#   helper for the reason `q_arm`'s control states: five refusals prove nothing until an inert shim is
#   shown NOT refusing. Here it is sharper still -- an inert one-shot shim must land on
#   `cause=never-dirty`, or the unreadable arm above would be a property of the fixture rather than of
#   the failing read.
q_unreadable() { # label tree want match
	local label=$1 tree=$2 want=$3 match=$4 shim="" saved=$PATH
	q_reset "$tree"
	shim=$(shim_install_first "$match")
	PATH="$shim:$PATH"
	hash -r
	expect_quarantine "$label" "$tree" "$want"
	PATH=$saved
	hash -r
	return 0
}

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

	# The two reads of ONE tree, disagreeing in the direction no refusal arm can reach (#267).
	# `update-index --refresh` is the read that turns an unreadable tree into `rc 2` (`common.sh`'s
	# `its state could not be read`), so failing it exactly once leaves the halt's verdict unknown and
	# the action's verdict readable and clean -- the only state that reaches `cause=verdict-unreadable`.
	# The tree is deliberately left CLEAN: dirty it and the second read finds work, the quarantine
	# runs, and the pass is on the ordinary rescue path again rather than here.
	q_unreadable "[$where] verdict unreadable, action clean" "$tree" UNREADABLE 'update-index --refresh'
	q_unreadable "[$where] an inert one-shot shim says nothing was dirty" "$tree" PROCEED \
		'definitely-not-a-git-subcommand'
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
#
# Both connections carry `pageInfo.hasNextPage: false` because the shipped query asks for it: a page
# that does not answer the bound reads as a page that stopped short (H-0103), and the refusal takes
# the whole issue half with it -- which is how this fixture came to print `PASS units=0` and eleven
# red legs the day the two changes met. A fake that answers less than the code asks for is not a
# smaller GitHub; it is a different one.
printf '{"login":"supervisor"}\n' >"$proc_fix/user.json"
printf '{"data":{"repository":{"pullRequests":{"pageInfo":{"hasNextPage":false},"nodes":[]},"issues":{"pageInfo":{"hasNextPage":false},"nodes":[{"number":900,"stateReason":"NONE"},{"number":901,"stateReason":"NONE"}]}}}}\n' >"$proc_fix/graphql.json"
printf '[]\n' >"$proc_fix/pr-open.json"
printf '[]\n' >"$proc_fix/pr-bodies.json"
printf '[]\n' >"$proc_fix/pr-merged.json"
printf '[{"number":900,"title":"unit A: the tree a dead tick left dirty","updatedAt":"2026-11-10T00:00:00Z"},{"number":901,"title":"unit B: the clean unit behind it","updatedAt":"2026-11-10T00:00:01Z"}]\n' >"$proc_fix/issues.json"
# The tick's delivery read (`gh pr list --head <branch>`). It is not what any leg below asserts; it
# is answered so a tick ends the way a delivered tick ends, and the journal carries only the property
# under test rather than `TICK-UNDELIVERED` and a bundle write for a unit that never existed.
printf '[{"number":9001}]\n' >"$proc_fix/pr-head.json"

# `--json` is a field list, and a fixture that ignores it answers a question nobody asked.
#
# `gh pr list --json a,b` replies with exactly `a` and `b`, and refuses a name the payload has not
# got. Both fixture shims used to parse `--json` for one thing only -- telling two payloads apart --
# and then hand over the whole file. So a shipped read that STOPPED asking for a field kept receiving
# it, and every leg that asserts something about that field stayed green while the read went blind.
# This is the inverse of the hazard this file's own header names ("a change to what triage.sh asks
# for ... changes what these legs see"), and #279 measured the sentence false at this commit:
#
#   `--json number,title,updatedAt` at factory/triage.sh:563 reduced to `--json number,title`,
#   `bash factory/selftest.sh` -> `387 passed, 0 failed`, identical to the unpunctured run, while the
#   debug rows still printed `issue 232 delivered` -- a verdict `delivered_claim` (triage.sh:509)
#   refuses to render on an empty `updatedAt`. The field arrived from the fixture, not from the read.
#
# One heredoc, installed into both payload directories, because the rule being fixed here is one that
# lived in a single branch of one shim and nowhere else: H-0108 (PR #278) projected the merged
# payload at its own dispatch site, and #279 counted six payload sites still served blind. A rule
# written at a call site is a rule for that call site.
install_gh_fields() { # install_gh_fields <payload-dir> -> the --json semantics both fixture shims obey
	[ -d "$1" ] || {
		printf 'selftest: no payload directory at %s, so the field list would go unenforced\n' "$1" >&2
		return 1
	}
	cat >"$1/fields.sh" <<'GHFIELDS'
# fields.sh -- what `gh --json` does to a payload, for the fixture `gh` in factory/selftest.sh.
# Installed by `install_gh_fields`, sourced by both fixture shims, never run directly.
#
# `jq -n` in the projection is not decoration: `--slurpfile` binds the payload and leaves the input
# empty, so a filter written against `.` runs zero times and writes nothing at exit 0 (H-0108's
# discovery, written out because it fails silently and in exactly the direction that reads as green).
gh_json_payload() { # <payload-file> <field-list> -> that payload as `gh --json` answers it
	local file=$1 want=${2:-} carry missing field
	# No `--json` means the caller asked for the endpoint's own document -- `gh api user`,
	# `gh api graphql` -- which has no field list to obey. Verbatim, as before.
	if [ -z "$want" ]; then
		cat "$file"
		return 0
	fi
	# `--json` answers a list of rows. Anything else behind it means the dispatch above served the
	# wrong payload to this call, which is the failure that makes a claim list silently empty.
	if ! jq -e 'type == "array"' "$file" >/dev/null 2>&1; then
		printf '%s: --json was asked of a payload that is not a list of rows: %s\n' \
			"${0##*/}" "$file" >&2
		return 9
	fi
	carry=$(jq -r '[.[] | keys[]] | unique | join(",")' "$file") || return 9
	# A field no row carries is refused out loud, the way `gh` refuses it, rather than answered with
	# an empty column. Silence is how a fixture quietly becomes the specification: a read that asks
	# for something the repository holds and the fixture does not would get nothing back and every
	# leg beside it would call that a pass. An EMPTY payload carries nothing by definition, so it
	# says nothing about the field list -- `gh` checks a name against its own schema, not against the
	# rows that happen to be in the window, and the unlegged reads run against `[]` every pass.
	missing=$(jq -rn --arg j "$want" --arg c "$carry" '
		($j | split(",")) as $want | ($c | split(",")) as $have
		| if $c == "" then "" else (($want - $have) | join(",")) end') || return 9
	if [ -n "$missing" ]; then
		local IFS=,
		for field in $missing; do
			printf 'Unknown JSON field: "%s"\n' "$field" >&2
		done
		printf 'Available fields: %s\n' "$carry" >&2
		return 1
	fi
	jq -c --arg j "$want" '
		($j | split(",")) as $f
		| [.[] | with_entries(select(.key as $k | $f | index($k)))]' "$file"
}
GHFIELDS
}
install_gh_fields "$proc_fix"

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
# shellcheck disable=SC1090 -- installed by install_gh_fields, beside the payloads it projects
. "$here/fields.sh"
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

emit() { # emit <payload> -> the fields the caller named, then its own --jq, both run by the real jq
	local file=$1 src
	if [ ! -f "$file" ]; then
		printf 'proc-shim: no payload for this call: %s\n' "$file" >&2
		exit 8
	fi
	# The field list is obeyed, not merely parsed to choose a payload. gh_json_payload is the same
	# file CLAIMSHIM sources, because #279 is a fix that existed in one shim and not the other.
	src=$(gh_json_payload "$file" "$jsonf") || exit 1
	if [ -n "$jqprog" ]; then
		printf '%s\n' "$src" | jq -r "$jqprog"
	else
		printf '%s\n' "$src"
	fi
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

# systemd's journal, for the reaping read every activation now makes (#246). Without this the fixture
# would read the OPERATOR's real journal: the leg's verdict would change whenever a real pass held, and
# a check whose answer comes from the machine rather than from what was planted discriminates nothing --
# the same objection `H-0125` raises about a `journalctl` stub that honours its own flags. `PROC_REAP`
# picks the transcript so one stub serves both the shipped run (`clean`, the default, so an ordinary
# activation looks ordinary) and a planted hold (`held`). The transcripts are the box's own 11 Nov
# activations with the unit renamed, and they terminate on `Deactivated.` rather than `Finished <unit>`
# on purpose: a fresh fixture box has no `$FACTORY_SERVICE_UNIT`, so its unit name is derived, and only
# the name-blind arms of the parser may be load-bearing here.
cat >"$proc_bin/journalctl" <<'PROCJ'
#!/usr/bin/env bash
case "${PROC_REAP:-clean}" in
held)
	printf '%s\n' \
		'2026-11-11T08:44:08.869637+00:00 h factory.sh[1969281]: 2026-11-11T08:44:08Z END rc=0 ticks_started=1' \
		'2026-11-11T08:45:08.999972+00:00 h systemd[4485]: fixture.service: State '"'"'stop-sigterm'"'"' timed out. Killing.' \
		'2026-11-11T08:45:09.000077+00:00 h systemd[4485]: fixture.service: Killing process 1971081 (winedevice.exe) with signal SIGKILL.' \
		'2026-11-11T08:45:09.002985+00:00 h systemd[4485]: fixture.service: Deactivated.'
	;;
none) : ;;
clean)
	printf '%s\n' \
		'2026-11-10T17:09:10.131057+00:00 h factory.sh[828665]: 2026-11-10T17:09:10Z END rc=0 ticks_started=1' \
		'2026-11-10T17:09:10.131574+00:00 h systemd[4485]: fixture.service: Deactivated.'
	;;
esac
# The journal-location read of #260 asks the same backend a different question -- which directory did
# the pass BEFORE this one write to -- and gets the same answer, so the same stub serves it. The window
# it is handed holds no `LOG-AT` line by default, which is the unknown arm and also the honest state of
# a fixture box whose unit has no earlier activation. `PROC_LOG_PREV` plants one, because both
# polarities have to be plantable: "the loop moved trees" and "the loop stayed put" are two fixtures,
# and the machine's own journal can only ever be one of them.
if [ -n "${PROC_LOG_PREV:-}" ]; then
	printf '%s\n' \
		'2026-11-10T17:09:09.000000+00:00 h factory.sh[828000]: 2026-11-10T17:09:09Z LOG-AT '"$PROC_LOG_PREV"
fi
exit 0
PROCJ
chmod +x "$proc_bin/journalctl"

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
proc_j2=$proc_journal

# The legs name the activation they read, because the second activation sees the tree the first one
# rescued: by then it is clean, and a leg that asks that pass whether a quarantine happened is asking
# the wrong pass. The first version of this section did exactly that and printed five failures over a
# run in which activation one had quarantined, mirrored and carried on correctly -- the journal of the
# activation *after* the accident says nothing about the accident, which is the same trap the section
# exists to keep the harness out of, arriving at the fixture instead of the production code.
proc_did_run() {
	local sessions
	# The fixture before the mutant. `proc_plant` asks git how many paths it left dirty, so this is a
	# second opinion and not the leg restating its own wish. It used to carry a second reason -- that a
	# tree nobody dirtied reached the same `TREE-CLEAN` as the failure being controlled for and the
	# journal could not tell the two apart. #267 fixed that sentence, so the count is now only what it
	# always primarily was: proof there was work here for a rescue to describe. The arm itself is pinned
	# by the two legs that read this fixture's journal below, not by this count.
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

# ------------------------------------- the run directory holds a stream, not a verdict (#351)

# `factory_worktree`'s stderr goes to a per-PID file so a refusal can be read back by name. Until
# #351 that file was `worktree-error.<pid>` and only the FAILURE arm deleted it, so every tick that
# GOT its tree left a zero-byte file whose name asserts a failure. Measured at 00:54Z on 14 Nov:
# four of them in `factory/run/` (`worktree-error.{3101739,3420682,4010611,747393}`, all 0 bytes,
# stamped 22:03, 22:59, 23:31 and 00:22Z -- the last four ticks), while
# `grep -c 'no worktree' factory/log/supervisor.log` answers 0: the failure the name reports has
# never once been journaled on this machine. Two separate reads of that directory, days apart,
# concluded the supervisor cannot build trees at all and the queue is stalled.

# So the stream is `worktree-stderr.<pid>`, both paths delete it, and the sweep keeps matching the
# retired name so files written before this change do not become permanent for lack of anything that
# looks for them. Both halves of the fix are in the supervisor, so the pin runs the supervisor as a
# process over the fixture rather than reading the arm.

# Every count here is over the glob `worktree-*`, not one name. What a reader does at 3 a.m. is count
# that glob and call the number `failed ticks`; a leg that grepped the new name alone would go green
# on a rename that put the alarm right back.

# Markers in the run directory of the scenario built last -- one name per line, sorted.
proc_markers() {
	find "$proc_rundir" -maxdepth 1 -name 'worktree-*' -printf '%f\n' 2>/dev/null | LC_ALL=C sort
}

# One per line, indented, for a failure message. Through sed rather than an unquoted printf
# argument, which is the same split without asking shellcheck to forgive it.
proc_markers_indented() {
	proc_markers | sed 's/^/                /'
}

proc_no_markers() { # <which pass> -- a pass that should have left nothing behind
	local got count
	got=$(proc_markers)
	count=$(printf '%s\n' "$got" | grep -c . || true)
	[ -z "$got" ] || {
		printf '            %s left %s file(s) in the run directory, each named after a verdict\n' "$1" "$count" >&2
		printf '            it did not earn. A reader counts these and calls the count "failed\n' >&2
		printf '            ticks"; the directory holds:\n' >&2
		proc_markers_indented >&2
		return 1
	}
	return 0
}

proc_marker_present() { # <which pass> <name> -- the sweep has to SPARE a file as well as take one
	proc_markers | grep -qxF -- "$2" || {
		printf '            %s: %s is gone. The sweep took a file it was told to spare -- anything\n' "$1" "$2" >&2
		printf '            younger than the bound could belong to a supervisor running right now.\n' >&2
		proc_markers_indented >&2
		return 1
	}
	return 0
}

proc_marker_absent() { # <which pass> <name> <why it had to go>
	if proc_markers | grep -qxF -- "$2"; then
		printf '            %s: %s survived, and %s. The directory holds:\n' "$1" "$2" "$3" >&2
		proc_markers_indented >&2
		return 1
	fi
	return 0
}

proc_took_tree() { # <journal> <rc> -- the success path ran, so a marker count means what it claims
	local line
	[ "$2" = 0 ] || {
		printf '            the pass exited %s rather than 0, so its run directory says nothing about\n' "$2" >&2
		printf '            a tick that got its tree. Its journal:\n' >&2
		sed 's/^/              /' "$1" | cut -c1-150 >&2
		return 1
	}
	line=$(proc_journal_line "$1" 'TICK-START unclaimed:900')
	[ -n "$line" ] || {
		printf '            unit A was never ticked, so no tree was asked for and no stream file was\n' >&2
		printf '            opened: a marker count over an untouched run directory would pass for the\n' >&2
		printf '            wrong reason. The journal:\n' >&2
		sed 's/^/              /' "$1" | cut -c1-150 >&2
		return 1
	}
	case "$line" in
	*' tree='*) return 0 ;;
	*) printf '            TICK-START carries no tree= field, so this pass did not take a tree:\n' >&2
	printf '              %s\n' "$line" >&2
	return 1 ;;
esac
}

# proc_mutate_last <needle> -> the fixture's factory.sh with the LAST line carrying needle replaced
# by $PROC_MUT_LINE (empty deletes it). The neighbour below, proc_mutate_factory, refuses any anchor
# that does not occur exactly once -- the right rule, and unusable here, because both arms of the same
# `if` carry the same `rm -f -- "$werr"` sentence. A helper that took the FIRST match would control the
# failure path while claiming to blind the success path, so this one takes the last and says so.
# PROC_MUT_LINE arrives from the caller as a command prefix and awk reads it out of ENVIRON, because
# awk's -v would eat the backslashes in a replacement line. Every refusal below is the same rule: a
# dead mutant is not a guilty one, so a control that cannot be planted has to be loud.
proc_mutate_last() { # <needle>
	local needle=$1 hits
	hits=$(grep -cF -- "$needle" "$HERE/factory.sh" || true)
	[ "$hits" -ge 1 ] || {
		printf '            nothing in factory/factory.sh carries %s, so this control ran shipped\n' "$needle" >&2
		printf '            code while reporting a mutation.\n' >&2
		return 1
	}
	if ! awk -v n="$hits" -v needle="$needle" '
		{
			if (index($0, needle) && ++seen == n) {
				if (length(ENVIRON["PROC_MUT_LINE"]) > 0) print ENVIRON["PROC_MUT_LINE"]
				next
			}
			print
		}' "$HERE/factory.sh" >"$proc_dir/factory.sh"; then
		printf '            the mutant could not be written at all: the pass would have run shipped code\n' >&2
		printf '            or none. A dead mutant is not a guilty one.\n' >&2
		return 1
	fi
	[ -s "$proc_dir/factory.sh" ] || {
		printf '            the mutant is empty: a dead mutant, not a guilty one.\n' >&2
		return 1
	}
	if [ -n "${PROC_MUT_LINE:-}" ]; then
		grep -qF -- "$PROC_MUT_LINE" "$proc_dir/factory.sh" || {
			printf '            the mutant line is not in the copy, so the control ran shipped code:\n' >&2
			printf '              %s\n' "$PROC_MUT_LINE" >&2
			return 1
		}
	fi
	bash -n "$proc_dir/factory.sh" || {
		printf '            the mutant does not parse: the pass would die before it journalled a row,\n' >&2
		printf '            and the control would read as the effect it claims to test.\n' >&2
		return 1
	}
	cmp -s "$HERE/factory.sh" "$proc_dir/factory.sh" && {
		printf '            the mutant is byte-identical to the shipped file, so this control\n' >&2
		printf '            measured nothing.\n' >&2
		return 1
	}
	return 0
}

# Scenario one: the tick that gets its tree -- the path that used to leave the alarm behind.
proc_new marker-success
proc_plant
proc_run marker-success-1
proc_ms_j=$proc_journal
proc_ms_rc=$proc_rc

proc_leg 'a tick that got a tree' 'TICK-START names the tree, so the success path is what ran' \
	proc_took_tree "$proc_ms_j" "$proc_ms_rc"
proc_leg 'leaves no marker behind' 'a pass that got its tree holds no worktree-* file when it finishes' \
	proc_no_markers 'a pass that got its tree'

# Scenario two: the tick that does not. `factory_worktree` refuses a branch it finds checked out in a
# tree it does not own, with a sentence of its own -- and it can only read that sentence before
# deleting the file, which is what the reason in the journal proves.
proc_new marker-refused
git -C "$proc_repo" worktree add -q -b issue-900 "$proc_work/elsewhere" origin/main || true
proc_run marker-refused-1
proc_mf_j=$proc_journal

proc_mf_refused_by_name() {
	local line
	line=$(proc_journal_line "$proc_mf_j" 'no worktree for issue-900 ::')
	[ -n "$line" ] || {
		printf '            the pass never journaled a refusal for issue-900, so nothing below is a\n' >&2
		printf '            verdict on the path that reads the stream. Its journal:\n' >&2
		sed 's/^/              /' "$proc_mf_j" | cut -c1-150 >&2
		return 1
	}
	case "$line" in
	*'checked out in'*) return 0 ;;
	*) printf '            the refusal did not carry the sentence the stream held, so the file was\n' >&2
	printf '            deleted before it was read, or never read:\n              %s\n' "$line" >&2
	return 1 ;;
esac
}

proc_mf_no_placeholder() {
	if [ -n "$(proc_journal_line "$proc_mf_j" 'common.sh gave no reason')" ]; then
		printf '            the refusal printed the fallback, which is what an empty or unreadable\n' >&2
		printf '            stream produces: the read is the thing this leg is for.\n' >&2
		sed 's/^/              /' "$proc_mf_j" | cut -c1-150 >&2
		return 1
	fi
	return 0
}

proc_leg 'a refusal journals why' 'the reason came out of the stderr file and into the journal line' \
	proc_mf_refused_by_name
proc_leg 'not from the fallback' 'the placeholder is absent, so the stream was read and was not empty' \
	proc_mf_no_placeholder
proc_leg 'and the refusal cleans up' 'the failure arm still deletes the stream, after reading it' \
	proc_no_markers 'a tick that was refused its tree'

# Scenario three: the sweep. It exists for a tick killed between the write and the delete, and it has
# to tell three planted files apart: the retired name past the bound (goes), the new name past the
# bound (goes), and anything inside the bound (stays, because a live supervisor may be writing it).
proc_new marker-sweep
proc_plant
: >"$proc_rundir/worktree-error.400001"
: >"$proc_rundir/worktree-stderr.400002"
: >"$proc_rundir/worktree-error.400003"
touch -d '4 hours ago' -- "$proc_rundir/worktree-error.400001" "$proc_rundir/worktree-stderr.400002"
proc_run marker-sweep-1

proc_leg 'the sweep takes old names' 'a worktree-error. file past the bound goes, though nothing writes that name now' \
	proc_marker_absent 'the sweep pass' 'worktree-error.400001' \
	'a file nothing writes any more must not outlive the sweep that was its only cleaner'
proc_leg 'and it takes the stream' 'a worktree-stderr. file past the bound goes too: the killed tick is what this sweep is for' \
	proc_marker_absent 'the sweep pass' 'worktree-stderr.400002' \
	'a tick killed between the write and the delete leaves exactly this file'
proc_leg 'and spares what is fresh' 'a marker under the bound survives, because a supervisor may be writing it right now' \
	proc_marker_present 'the sweep pass' 'worktree-error.400003'

# ---------------------------------------------------------------------- the two controls
#
# Control one deletes the success-path delete. If the leg above still passes with that line gone it
# was never watching the line. A control that cannot be planted is reported by the leg, not swallowed:
# `|| true` would silence a dead mutant, which is the false pass this file keeps chasing.
proc_new marker-ctl-delete
PROC_MUT_LINE='	: # MUTANT-351 the success-path delete is gone' \
	proc_mutate_last 'rm -f -- "$werr"' || proc_ctl_delete_planted=0
proc_plant
proc_run marker-ctl-delete-1
proc_cd_j=$proc_journal
proc_cd_rc=$proc_rc

proc_ctl_one_marker() {
	local got count
	[ "${proc_ctl_delete_planted:-1}" != 0 ] || {
		printf '            the mutant could not be planted, so this control ran shipped code and its\n' >&2
		printf '            silence proves nothing about the delete it was meant to remove.\n' >&2
		return 1
	}
	got=$(proc_markers)
	count=$(printf '%s\n' "$got" | grep -c . || true)
	[ "$count" = 1 ] || {
		printf '            with the success-path delete removed the pass left %s file(s), not the one\n' "$count" >&2
		printf '            that line accounts for, so the shipped leg above was not counting this\n' >&2
		printf '            file. The directory holds:\n' >&2
		proc_markers_indented >&2
		return 1
	}
	case "$got" in
	worktree-stderr.*) return 0 ;;
	*) printf '            the surviving marker is %s, not worktree-stderr.<pid>, so what the\n' "$got" >&2
	printf '            control left behind is not the file the shipped path writes.\n' >&2
	return 1 ;;
esac
}

proc_leg 'the control still ticks' 'the mutant pass took its tree, so the marker below has an owner' \
	proc_took_tree "$proc_cd_j" "$proc_cd_rc"
proc_leg 'the delete is what cleaned it' 'blinding the success-path rm leaves exactly one worktree-stderr file standing' \
	proc_ctl_one_marker

# Control two takes the retired name out of the sweep's pattern. Same shape: the leg that reads the
# shipped file is only worth anything if removing the term it depends on brings the file back.
proc_new marker-ctl-legacy
PROC_MUT_LINE='	find "$FACTORY_RUN" -maxdepth 1 \( -name "worktree-stderr.*" \) -mmin +180 -delete 2>/dev/null || true # MUTANT-351' \
	proc_mutate_last "-o -name 'worktree-error.*'" || proc_ctl_legacy_planted=0
proc_plant
: >"$proc_rundir/worktree-error.400001"
touch -d '4 hours ago' -- "$proc_rundir/worktree-error.400001"
proc_run marker-ctl-legacy-1

proc_ctl_legacy_blinded() {
	[ "${proc_ctl_legacy_planted:-1}" != 0 ] || {
		printf '            the mutant could not be planted, so this control ran shipped code: the file\n' >&2
		printf '            it reports as spared was spared by the sweep it was meant to blind.\n' >&2
		return 1
	}
	proc_marker_present 'the blinded sweep' 'worktree-error.400001'
}

proc_leg 'the retired name is the sweep' 'blinding the -o term leaves the old worktree-error file standing' \
	proc_ctl_legacy_blinded

# ---------------------------------------------------- queue order ranks the OLDER unit first (#307)
#
# Every leg above reads the shipped fixture, where both units are stamped in the same pass and the
# tie-break by number decides. So `TICK-START` for A before B is true whether the dispatcher's age key
# ranks oldest-first, youngest-first, or not at all -- and it was: the key shipped ascending, which on
# a duration is youngest-first, and the only leg of 521 that ever noticed was `…then starts a tick for
# unit B`, going red at a second boundary rather than on a diff (CI run 43105189339, 21:40:59Z on
# 11 Nov: the two stamps are 4-8 ms apart, so the fixture lands on the far side about once in 200 runs;
# 0 of 40 unseeded runs on this box crossed, that CI run did). On the live queue the same key is
# H-0007 inverted: `NEXT unclaimed 303 age=0s` at 20:22:20Z on 11 Nov, with #296 and #297 waiting since
# 16:13:25Z in the same rank.
#
# These legs plant the ages instead of hoping for a boundary -- a 4-8 ms crossing cannot be provoked by
# running more times -- and they ask it in BOTH polarities. One polarity alone is not a test of the
# age: a dispatcher that ignored `first-seen` and sorted on the number alone sends unit 900 first every
# time, which is exactly what the first polarity looks like. The second polarity makes 901 the older
# one, and only a key that really reads the age keeps it in front.
proc_rank_stamp() { # <ledger-id> <seconds-ago> -> a `first-seen` that old, planted before the pass can
	# stamp the row itself. The id is the kind and the number -- triage's first column is `kind`, so the
	# dispatcher's ledger key is `issue:900`. Planting the class-shaped `unclaimed:900` instead writes a
	# file nothing reads, which is the mistake factory.sh's own ranking comment names.
	#
	# Two ages planted here have to be further apart than the clock that reads them. `factory_age` counts
	# whole seconds (`date -u +%s`, common.sh:923) and the ranking loop gives every row its OWN call
	# (factory.sh:1197), so a pair planted one second apart comes back EQUAL whenever the two calls
	# straddle a second boundary -- and `proc_older_first` reads equal as a failure at :3569. Measured on
	# the shipped read path (the two calls, the shipped `factory_class_rank` and the re-serialise between
	# them, 10.5ms a trial): 2 ties in 300 at 2s/1s. A pair planted two seconds apart cannot be tied by a
	# straddle, because one boundary moves one age by exactly 1s -- it would take a 2s stall between two
	# key reads to do it, against a measured gap under 10ms. 0 ties in 300 at 3s/1s, same harness.
	printf '%s\n' "$(date -u -d "-${2} seconds" +%Y-%m-%dT%H:%M:%SZ)" >"$proc_stash/$1.first-seen"
}
proc_rank_next_age() { # <journal> <key> -> the `age=` the pass printed for that unit; empty if it never ranked one
	awk -v n="NEXT unclaimed $2 " \
		'index($0, n) && match($0, /age=[0-9]+s/) { print substr($0, RSTART + 4, RLENGTH - 5); exit }' "$1"
}
proc_rank_first() { # <journal> -> 900 | 901 | neither: whose TICK-START the journal reached first
	awk '
		index($0, "TICK-START unclaimed:900") && !la { la = NR }
		index($0, "TICK-START unclaimed:901") && !lb { lb = NR }
		END {
			if (!la && !lb) print "neither"
			else if (!la) print "901"
			else if (!lb) print "900"
			else print (la < lb ? "900" : "901")
		}' "$1"
}
proc_rank_run() { # <tag> <a-age-s> <b-age-s> -> a two-unit queue first seen THAT long ago, one pass over it
	local tag=$1
	proc_new "rank-$tag"
	proc_rank_stamp 'issue:900' "$2"
	proc_rank_stamp 'issue:901' "$3"
	proc_plant
	proc_run "rank-$tag"
}
proc_older_first() { # <900|901> -> that unit was dispatched first AND the pass ranked it the older one
	local want=$1 other=901 aw ao got
	[ "$want" = 901 ] && other=900
	aw=$(proc_rank_next_age "$proc_journal" "$want")
	ao=$(proc_rank_next_age "$proc_journal" "$other")
	got=$(proc_rank_first "$proc_journal")
	if [ -z "$aw" ] || [ -z "$ao" ]; then
		printf '            one unit was never ranked -- unit %s printed `age=%s`, unit %s\n' \
			"$want" "${aw:-nothing}" "$other" >&2
		printf '            printed `age=%s` -- so there is no order here to read:\n' "${ao:-nothing}" >&2
		sed 's/^/              /' "$proc_journal" | cut -c1-150 >&2
		return 1
	fi
	if [ "$got" != "$want" ]; then
		printf '            unit %s was the older of the two (%ss against %ss) and unit %s was\n' \
			"$want" "$aw" "$ao" "$got" >&2
		printf '            dispatched first, so the queue is ordered by something other than age.\n' >&2
		printf '            The young one gets the turn every pass, and the old one waits forever:\n' >&2
		grep -E 'NEXT unclaimed|TREE-QUARANTINED|TICK-START' "$proc_journal" |
			sed 's/^/              /' | cut -c1-150 >&2
		return 1
	fi
	if [ "$aw" -le "$ao" ]; then
		printf '            it went first, but the pass called it the YOUNGER of the two (%ss\n' "$aw" >&2
		printf '            against %ss), so the order came from the number and not the age -- the\n' "$ao" >&2
		printf '            tie-break wearing the age key'"'"'s clothes.\n' >&2
		return 1
	fi
	return 0
}
# The two polarities are not equally exposed, and the difference is read order, not luck: the fixture's
# queue is [900, 901] (:2940, :2944), so the ranking loop reads 900's age first in BOTH legs. When the
# older row is the one read SECOND -- `young-number`, where 901 is the old one -- a straddle widens the
# gap instead of erasing it, and 0 of 300 trials tied at the shipped 1s/2s. When the older row is read
# FIRST -- `old-first`, and the retired-key control, which plants the same pair -- a straddle equalises
# it. Both pairs are now 2s apart so neither leg depends on which row the queue happens to lead with,
# and so a fixture that later grows a row in front does not quietly move the exposure somewhere else.
proc_rank_run old-first 3 1
proc_leg 'the older unit is dispatched first' \
	'A stamped 3s ago ticked before B stamped 1s ago, and the NEXT ages say so' proc_older_first 900

proc_rank_run young-number 1 3
proc_leg '…and the older one is not the lower number' \
	'B stamped 3s ago ticked before A stamped 1s ago -- the age decides, not the key' proc_older_first 901

# proc_mutate_sort -> the fixture's factory.sh with the age key put back the way it shipped until #307:
# `-k2,2nr` becomes `-k2,2n`, one character, on the one line that ranks. The anchor carries the key's
# neighbour (`-k2,2nr -k3,3n)`) because the comment above the line quotes the same flags, and a mutant
# that edited only the prose would plant nothing while still reporting success.
proc_mutate_sort() {
	local hits
	hits=$(awk 'index($0, "-k2,2nr -k3,3n)") { c++ } END { print c + 0 }' "$HERE/factory.sh")
	if [ "$hits" != 1 ]; then
		printf '            the age key occurs %s time(s) in factory/factory.sh, so this control\n' "$hits" >&2
		printf '            cannot say which line it retired.\n' >&2
		return 1
	fi
	awk 'index($0, "-k2,2nr -k3,3n)") { gsub(/-k2,2nr/, "-k2,2n") } { print }' \
		"$HERE/factory.sh" >"$proc_dir/factory.sh" || return 1
	if ! grep -qF -- '-k1,1n -k2,2n -k3,3n)' "$proc_dir/factory.sh" ||
		grep -qF -- '-k2,2nr -k3,3n)' "$proc_dir/factory.sh"; then
		printf '            the copy does not run the retired spelling, so the control below would be\n' >&2
		printf '            shipped code describing a mutant that was never planted.\n' >&2
		return 1
	fi
	return 0
}
proc_new rank-sort-retired
# The same 2s-apart pair as `old-first`, and the same exposure: this plant is read 900-then-901, so at
# 2s/1s a straddle made the two ages equal and the retired key then won the NUMBER tie-break with 900 --
# which trips the third check below and reddens this control for a reason that has nothing to do with
# the `r`.
proc_rank_stamp 'issue:900' 3
proc_rank_stamp 'issue:901' 1
proc_mutate_sort || proc_sort_planted=0
proc_plant
proc_run rank-sort-retired
proc_retired_key_ranks_youngest() {
	if [ "${proc_sort_planted:-1}" = 0 ]; then
		printf '            the mutant was never planted, so this leg ran shipped code.\n' >&2
		return 1
	fi
	if proc_older_first 900 >/dev/null 2>&1; then
		printf '            with the `r` taken off the age key the shipped predicate still passed, so\n' >&2
		printf '            the ok above ranks nothing: it would pass on a queue that never read\n' >&2
		printf '            `first-seen` at all. The direction is the whole content of that leg.\n' >&2
		return 1
	fi
	if [ "$(proc_rank_first "$proc_journal")" != 901 ]; then
		printf '            the retired key dispatched %s first, not 901 -- the fixture no longer\n' \
			"$(proc_rank_first "$proc_journal")" >&2
		printf '            reaches the state this mutant is described by, so neither leg here is\n' >&2
		printf '            pinning a direction any more.\n' >&2
		return 1
	fi
	return 0
}
proc_leg 'control: the retired key ranks youngest' \
	'`-k2,2n` puts B first; so the ok above really ranked the age' proc_retired_key_ranks_youngest

# The healthy unit's own sentence, asserted (#267). A note used to sit here: it printed what the journal
# said of unit B and pinned nothing, on the stated grounds that a leg on the wording would be a leg that
# refuses the fix. The fix has landed, so the wording is now the thing under test and both halves get
# pinned -- on `cause=`, which names the ARM, which is what that objection was really about.
#
# Half one alone is worth nothing. Delete the disagreement arm outright and "the healthy path says
# nothing false" passes while the harness has also thrown away the one case the arm exists for. So the
# control after this leg plants that case -- dirt the verdict sees, gone by the time the action looks --
# and demands the disagreement sentence come out. Two legs, one per half, because each is the other's
# proof that the first half means something.
proc_journal_has() { # <journal> <needle> <needle> -> 0 when ONE line carries both
	awk -v a="$2" -v b="$3" 'index($0, a) && index($0, b) { found = 1 } END { exit found ? 0 : 1 }' "$1"
}

proc_healthy_unit_says_so() {
	local line tree
	line=$(proc_journal_line "$proc_j1" 'TREE-CLEAN unclaimed:901')
	[ -n "$line" ] || {
		printf '            unit B got no `TREE-CLEAN` line at all, so the journal says nothing about\n' >&2
		printf '            the tree it stopped. Silence is the other way this arm goes wrong: an\n' >&2
		printf '            operator cannot tell a clean tree from a pass that never looked:\n' >&2
		sed 's/^/              /' "$proc_j1" | cut -c1-150 >&2
		return 1
	}
	if proc_journal_has "$proc_j1" 'unclaimed:901' 'cause=verdict-disagreed'; then
		printf '            the healthy unit was journalled as a disagreement:\n              %s\n' "$line" >&2
		printf '            That is the sentence #267 is about. It tells the operator a person or a\n' >&2
		printf '            dead tick had dirt in this tree and no longer does, about a worktree\n' >&2
		printf '            `factory_worktree` cut minutes earlier and nothing ever wrote to.\n' >&2
		return 1
	fi
	proc_journal_has "$proc_j1" 'unclaimed:901' 'cause=never-dirty' || {
		printf '            unit B took neither arm, so the journal still does not say which one it\n' >&2
		printf '            took -- and "which arm" is all this line was ever for:\n              %s\n' "$line" >&2
		return 1
	}
	# The second opinion, off the journal rather than out of it: the tree the line names, read after
	# the pass. A leg that only greps for the new words passes on a build that renamed the token and
	# went on lying, and `proc_did_run` two legs up already refuses to let a journal restate its own
	# wish -- same rule, applied to the sentence this leg was written for.
	tree=${line#*'TREE-CLEAN unclaimed:901 '}
	tree=${tree%% *}
	if [ -z "$tree" ] || [ ! -d "$tree" ]; then
		printf '            the line names no tree a reader can go and look at: %s\n' "$line" >&2
		return 1
	fi
	[ "$(q_dirt "$tree")" = 0 ] || {
		printf '            it says nothing was dirty while the very tree it names holds %s path(s):\n' \
			"$(q_dirt "$tree")" >&2
		printf '              %s\n' "$tree" >&2
		return 1
	}
	# Activation 2 as well, and for a different reason. By then unit A's tree has been rescued and
	# unit B's was never dirtied, both units have ticked and the streak is cleared -- so this is the
	# pass with the LEAST excuse for a rescue line, and the one the note never asked. Absorbed from the
	# first attempt at this unit, which the supervisor quarantined out from under it: mirror
	# `issue-267-quarantine-20261110T213522Z-efa97d027.patch` in `FACTORY_AT_RISK`, stash `efa97d027`.
	# That version asked both activations and this branch had not thought to.
	if grep -qF 'cause=verdict-disagreed' "$proc_j2"; then
		printf '            the SECOND activation journalled a disagreement. Both trees were clean\n' >&2
		printf '            by then and both units had ticked, so nothing in this pass had an excuse:\n' >&2
		grep -F 'cause=verdict-disagreed' "$proc_j2" | sed 's/^/              /' | cut -c1-150 >&2
		return 1
	fi
	for u in 900 901; do
		proc_journal_has "$proc_j2" "unclaimed:$u" 'cause=never-dirty' || {
			printf '            and `unclaimed:%s` took no named arm in the all-healthy activation,\n' "$u" >&2
			printf '            so a pass with nothing to rescue does not say so either:\n' >&2
			grep -F "TREE-CLEAN unclaimed:$u" "$proc_j2" | sed 's/^/              /' | cut -c1-150 >&2
			return 1
		}
	done
	return 0
}
proc_leg 'the clean unit is not credited with a rescue' \
	'901 journalled cause=never-dirty, no disagreement line, and its tree is clean on disk' \
	proc_healthy_unit_says_so

# Half two: the arm the disagreement sentence is TRUE for, still reachable, on the same fixture. Planted
# rather than hoped for, because the state is "the verdict the halt reads said dirty, the verdict the
# action reads said clean" and on a quiescent box nothing between those two lines ever changes a tree --
# which is why #263's controls plant their failures too. The plant takes the work away in exactly that
# gap. It is inert for unit B, whose verdict is empty, and that is what makes ONE run carry both halves:
# A has to disagree, B must not.
proc_new verdict-disagrees
proc_mutate 'dirt=$(factory_tree_dirt "$tree" 2>/dev/null) || dirt_rc=$?' \
	'	if [ -n "$dirt" ]; then git -C "$tree" checkout -- . >/dev/null 2>&1; git -C "$tree" clean -fdq . >/dev/null 2>&1; fi # MUTANT-267A the work is gone by the time the action looks' ||
	proc_267a_planted=0
proc_plant
proc_pd=$proc_planted
proc_run disagrees-1
proc_jd=$proc_journal

proc_disagreement_still_reachable() {
	local line
	if [ "${proc_267a_planted:-1}" = 0 ]; then
		printf '            the race was never planted, so this control ran shipped code and the arm\n' >&2
		printf '            it exists to keep reachable was never asked to fire.\n' >&2
		return 1
	fi
	[ "${proc_pd:-0}" = 2 ] || {
		printf '            the fixture tree has %s dirty path(s), not the two this section plants.\n' \
			"${proc_pd:-nothing}" >&2
		printf '            With nothing to take away the two reads cannot disagree, and the journal\n' >&2
		printf '            below would then be unit B reporting that it was never dirty -- true, and\n' >&2
		printf '            not a thing that proves the arm still works.\n' >&2
		return 1
	}
	line=$(proc_journal_line "$proc_jd" 'TREE-CLEAN unclaimed:900')
	proc_journal_has "$proc_jd" 'unclaimed:900' 'cause=verdict-disagreed' || {
		printf '            the verdict said dirty, the action found nothing, and the journal STILL\n' >&2
		printf '            did not take the disagreement arm. Unit A'"'"'s line: %s\n' "${line:-none}" >&2
		printf '            Delete that arm and the leg above goes green with it. That is why both\n' >&2
		printf '            legs run, and why this one is a control and not a note.\n' >&2
		sed 's/^/              /' "$proc_jd" | cut -c1-150 >&2
		return 1
	}
	# The finding survives into the line. #267's second unestablished point is that "`factory_tree_dirt`
	# 's verdict is not journalled, so a reader cannot reconstruct from the log which arm each line
	# took"; a disagreement arm that names nothing is still unreadable for the same reason.
	case "$line" in
	*tracked.txt*scratch-notes.txt*) : ;;
	*) printf '            the disagreement line names neither of the two paths the verdict reported,\n' >&2
		printf '            so it still cannot be debugged from the journal alone:\n              %s\n' "$line" >&2
		return 1 ;;
	esac
	# And the arm must be honest about what it did not do. If the quarantine ran, the action found
	# something, and this is the ordinary rescue path wearing this control's name.
	[ -z "$(proc_journal_line "$proc_jd" 'TREE-QUARANTINED unclaimed:900')" ] || {
		printf '            unit A was quarantined, so the action found work to stash and the two\n' >&2
		printf '            reads never disagreed -- this control measures nothing.\n' >&2
		return 1
	}
	[ "$(q_dirt "$proc_trees/issue-900")" = 0 ] || {
		printf '            the planted work is still on disk (%s path(s)), so the state the line\n' \
			"$(q_dirt "$proc_trees/issue-900")" >&2
		printf '            describes did not happen: %s\n' "$proc_trees/issue-900" >&2
		return 1
	}
	# Both arms in the one journal, or the control has only proved the sentence prints for everything
	# -- which is the defect, rearranged.
	proc_journal_has "$proc_jd" 'unclaimed:901' 'cause=never-dirty' || {
		printf '            and unit B did not take the never-dirty arm in the very run where A\n' >&2
		printf '            disagreed, so this journal does not hold both arms at once:\n' >&2
		sed 's/^/              /' "$proc_jd" | cut -c1-150 >&2
		return 1
	}
	[ "$proc_rc" = 0 ] || {
		printf '            the disagreement arm changed the activation'"'"'s exit to %s. It must\n' "$proc_rc" >&2
		printf '            still hand the unit to a tick exactly as it did before: a false halt costs\n' >&2
		printf '            a turn, and #263 measured what a halt costs the queue.\n' >&2
		return 1
	}
	return 0
}
proc_leg 'the disagreement arm still fires when it is true' \
	'A: cause=verdict-disagreed naming both planted paths, no quarantine, tree clean, exit 0; B: never-dirty' \
	proc_disagreement_still_reachable

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
# #237, planted rather than waited for: by the time the NEXT pass prints the halt, the cause is two
# hours old. A marker aged here is the whole of the leg -- the second activation must not be able to
# print the first activation's line verbatim, and the only fact that could tell them apart is the
# marker's own age, which is the mtime `factory_tree_halt` just stamped by writing the file.
if [ -e "$proc_halt" ]; then
	touch -d '2 hours ago' "$proc_halt"
else
	printf '        note: the halting control left no marker, so the aged-marker assertion below has\n'
	printf '        nothing to age. `proc_halt_freezes` reports the missing marker as its own failure.\n'
fi
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
	# #237: the line has to say how old its own cause is, not merely that a halt exists. Three
	# activations seconds apart over one eight-hour-old marker wrote byte-identical `HALTED` lines
	# until this branch; the marker above was aged by hand before the second pass, so a `2h` in this
	# line can only have come from asking the marker, and a line without it is a nine-hour outage
	# printing as news.
	if ! proc_journal_line "$proc_journal" 'HALTED ' | grep -q 'cause recorded 2h'; then
		printf '            the second activation reported the halt but not its age, so the line\n' >&2
		printf '            still reads as a fresh detection of a cause planted two hours old for\n' >&2
		printf '            this very assertion. Its line:\n' >&2
		printf '              %s\n' "$(proc_journal_line "$proc_journal" 'HALTED ')" >&2
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
		printf '            reaches `cause=never-dirty` and never gets as far as the mutant, which
' >&2
		printf '            is planted after `TREE-QUARANTINED`. This control would be shipped code
' >&2
		printf '            describing a rescue that had nothing to rescue, not the control.
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

# ------------------------------------------------------- the reaping a pass reports about itself (#246)
#
# Two legs the reap section at the end of this file cannot write, because they need an activation, not
# a transcript: that a real pass writes the reaping into the supervisor's journal, and writes it BEFORE
# triage. The order is the claim worth a process -- `TRIAGE-FAILED` returns 3 four lines after the
# reaping loop, so a pass that never got a queue still says what the last one cost; and on 8 Nov the
# pass printed `END rc=4` and the journal's next line came 67 minutes later. The transcript comes from
# the fixture's `journalctl` stub, so both legs answer the same way on a box with no user bus.
proc_new reap-held
# `export`, not a bare assignment: `proc_run` hands the pass to `env`, and a variable that is only set
# in this shell never reaches the stub three processes down -- which is how this leg first failed, on a
# pass that dutifully reported the clean transcript it was given.
export PROC_REAP=held
proc_run reap-held
unset PROC_REAP

proc_reap_written_by_the_pass() {
	local line=''
	line=$(proc_journal_line "$proc_journal" 'REAP-HELD ')
	[ -n "$line" ] || {
		printf '            an activation that held the queue for a minute was not written to the\n' >&2
		printf '            supervisor journal by the next pass. That journal:\n' >&2
		sed 's/^/              /' "$proc_journal" | cut -c1-150 >&2
		printf '            If a REAP-UNKNOWN is above, the fixture'"'"'s journalctl stub was bypassed\n' >&2
		printf '            and the pass read this machine instead of the planted transcript.\n' >&2
		return 1
	}
	case "$line" in
	*'hold=60s how=deadline'*) : ;;
	*) printf '            it wrote the hold with the wrong number or the wrong cause:\n' >&2
		printf '              %s\n' "$line" >&2
		printf '            want hold=60s (the planted 60.133 s) and how=deadline.\n' >&2
		return 1 ;;
	esac
	awk '
		index($0, "REAP-HELD ") { r = NR }
		index($0, " PASS ") { p = NR }
		END { exit (r && p && r < p) ? 0 : 1 }' "$proc_journal" || {
		printf '            the reaping is not in the journal BEFORE the PASS line, so a pass that\n' >&2
		printf '            fails at triage never reports it: reaping at line %s, PASS at line %s.\n' \
			"$(awk 'index($0,"REAP-HELD "){print NR; exit}' "$proc_journal")" \
			"$(awk 'index($0," PASS "){print NR; exit}' "$proc_journal")" >&2
		return 1
	}
	return 0
}
proc_leg 'the pass writes a held teardown down' \
	'REAP-HELD hold=60s how=deadline, above the PASS line' proc_reap_written_by_the_pass

proc_reap_clean_too() {
	# The mark is unconditional, which is the half a hold-only leg would miss: `SUPERVISOR-AT` and
	# `UNIT-AT` are on every pass for the reason H-0118 keeps re-learning -- a read nobody prints is a
	# read that cannot be told apart from a read that is not there. So a healthy activation has to say
	# so, and a journal with NO reap line at all is the failure, not the success.
	local line=''
	line=$(proc_journal_line "$proc_j1" 'REAP-CLEAN ')
	[ -n "$line" ] || {
		printf '            the shipped activation wrote no REAP-CLEAN line. Its journal:\n' >&2
		sed 's/^/              /' "$proc_j1" | cut -c1-150 >&2
		printf '            A REAP-UNKNOWN here means the fixture read this machine'"'"'s journal, and\n' >&2
		printf '            a REAP-HELD means the planted transcript changed under the legs above.\n' >&2
		return 1
	}
	case "$line" in
	(*HELD*) printf '            a clean teardown was filed as a hold:\n              %s\n' "$line" >&2
		return 1 ;;
	esac
	return 0
}
proc_leg '…and a clean one, on every pass' \
	'REAP-CLEAN on the shipped activation, with no hold word' proc_reap_clean_too

# ------------------------------------------------ the journal's own location, on a real activation (#260)
#
# The replay section at the end of this file can plant any transcript it likes. What it cannot plant is
# an activation, and these legs need one for the reason #283 filed the reap legs as blocking: a read
# that exists only as a function is a read the loop may never perform, and `H-0120` measured exactly
# that for a feature whose whole output is a journal line. Three claims need the process, not the call:
#
#   that the pass emits `LOG-AT` at all, on an ordinary pass with nothing planted;
#   that it emits it BEFORE the `PASS` line, which is what makes it the record the next pass reads back
#     on a pass that halts at triage and writes no `PASS` (the same ordering claim the reap legs make);
#   that the copy really lands in the journal being LEFT -- the file an operator is still tailing, and
#     the one the supervisor's own `log_event` will never write to again.
#
# Both polarities, because #260 ask 3 asks for a line when the journal moved and silence when it did
# not, and a leg that only plants a move never tests the silence.

lm_old=$proc_work/log-move-old
proc_new log-move
mkdir -p -- "$lm_old/factory/log"
# A journal with a line in it already. The copy is an APPEND, and a leg that planted an empty file
# could not tell appending to a journal from truncating one -- which is the destructive case here,
# since the file being written to belongs to a checkout this pass is not running from.
printf '2026-11-10T17:09:09Z PASS units=1 dry=0 budget=120s root=old trees=/wt\n' \
	>"$lm_old/factory/log/supervisor.log"
# `export`, for the reason the reap legs learned: `proc_run` hands the pass to `env`, and a variable set
# only in this shell never reaches a stub three processes down.
export PROC_LOG_PREV=$lm_old/factory/log
proc_run log-move
unset PROC_LOG_PREV

proc_log_at_on_every_pass() {
	local line=''
	line=$(proc_journal_line "$proc_journal" ' LOG-AT ')
	[ -n "$line" ] || {
		printf '            a pass wrote no LOG-AT line, so the pass after it cannot ask where this\n' >&2
		printf '            one wrote, and neither can an operator. Its journal:\n' >&2
		sed 's/^/              /' "$proc_journal" | cut -c1-150 >&2
		printf '            A LOG-MOVED-UNKNOWN above means the stub was bypassed and the pass read\n' >&2
		printf '            this machine'"'"'s journal instead of the planted one.\n' >&2
		return 1
	}
	case "$line" in
	(*"$proc_log"*) : ;;
	*) printf '            LOG-AT does not name the directory the pass was writing to (%s):\n' "$proc_log" >&2
		printf '              %s\n' "$line" >&2
		return 1 ;;
	esac
	awk '
		index($0, " LOG-AT ") { a = NR }
		index($0, " PASS ") { p = NR }
		END { exit (a && p && a < p) ? 0 : 1 }' "$proc_journal" || {
		printf '            LOG-AT is not in the journal BEFORE the PASS line: LOG-AT at line %s,\n' \
			"$(awk 'index($0," LOG-AT "){print NR; exit}' "$proc_journal")" >&2
		printf '            PASS at line %s. A pass that halts at triage writes no PASS at all, and\n' \
			"$(awk 'index($0," PASS "){print NR; exit}' "$proc_journal")" >&2
		printf '            it is exactly that pass the next one has to be able to locate.\n' >&2
		return 1
	}
	return 0
}
proc_leg 'the pass names its own journal directory' \
	'LOG-AT naming FACTORY_LOG, above the PASS line' proc_log_at_on_every_pass

proc_log_moved_names_both_files() {
	local n='' line='' seed='' copy=''
	n=$(grep -c ' LOG-MOVED ' "$proc_journal" || true)
	[ "$n" = 1 ] || {
		printf '            the pass wrote %s LOG-MOVED line(s), want exactly 1. Its journal:\n' "$n" >&2
		sed 's/^/              /' "$proc_journal" | cut -c1-150 >&2
		return 1
	}
	line=$(proc_journal_line "$proc_journal" ' LOG-MOVED ')
	case "$line" in
	*"old=$lm_old/factory/log"*) : ;;
	*) printf '            the move does not name the journal being left:\n              %s\n' "$line" >&2
		return 1 ;;
	esac
	case "$line" in
	*"new=$proc_log"*) : ;;
	*) printf '            the move does not name the journal being entered:\n              %s\n' "$line" >&2
		return 1 ;;
	esac
	case "$line" in
	*'tail=ok'*) : ;;
	*) printf '            the move does not report writing the copy (want tail=ok):\n' >&2
		printf '              %s\n' "$line" >&2
		return 1 ;;
	esac
	# The copy is the half the installer could not promise: it goes into the file whose tail went dead,
	# and it must not disturb the line that was already there.
	seed=$(head -n 1 "$lm_old/factory/log/supervisor.log")
	copy=$(tail -n 1 "$lm_old/factory/log/supervisor.log")
	[ "$(wc -l <"$lm_old/factory/log/supervisor.log")" = 2 ] || {
		printf '            the journal being left is no longer one seed line plus one copy: it has\n' >&2
		printf '            %s lines, so this pass rewrote somebody else'"'"'s record.\n' \
			"$(wc -l <"$lm_old/factory/log/supervisor.log")" >&2
		return 1
	}
	case "$seed" in
	('2026-11-10T17:09:09Z PASS units=1 dry=0 budget=120s root=old trees=/wt') : ;;
	*) printf '            the line that was already in the old journal is gone or altered:\n' >&2
		printf '              %s\n' "$seed" >&2
		return 1 ;;
	esac
	case "$copy" in
	(*copy=old-journal*) : ;;
	*) printf '            the line appended to the old journal does not say it is the copy:\n' >&2
		printf '              %s\n' "$copy" >&2
		return 1 ;;
	esac
	case "$copy" in
	*"new=$proc_log"*) : ;;
	*) printf '            the copy does not name where the loop went:\n              %s\n' "$copy" >&2
		return 1 ;;
	esac
	return 0
}
proc_leg '…and says so in the journal it is leaving' \
	'one LOG-MOVED naming both files, appended to the old journal' proc_log_moved_names_both_files

# The other polarity, on its own fixture: the same shipped pass, the same stub, one line of the planted
# journal different -- the previous pass named THIS directory. Silence is the required output, and a
# second copy in some other file is the failure.
lm_stay=$proc_work/log-stay-old
proc_new log-stay
mkdir -p -- "$lm_stay/factory/log"
printf '2026-11-10T17:09:09Z PASS units=1 dry=0 budget=120s root=old trees=/wt\n' \
	>"$lm_stay/factory/log/supervisor.log"
export PROC_LOG_PREV=$proc_log
proc_run log-stay
unset PROC_LOG_PREV

proc_log_stay_is_silent_about_a_move() {
	local at='' moved=''
	at=$(proc_journal_line "$proc_journal" ' LOG-AT ')
	moved=$(proc_journal_line "$proc_journal" ' LOG-MOVED ')
	[ -n "$at" ] || {
		printf '            the unchanged-journal fixture wrote no LOG-AT either, so it proves the\n' >&2
		printf '            read is silent for the wrong reason:\n' >&2
		sed 's/^/              /' "$proc_journal" | cut -c1-150 >&2
		return 1
	}
	[ -z "$moved" ] || {
		printf '            a pass that writes where the last one wrote reported a move:\n' >&2
		printf '              %s\n' "$moved" >&2
		printf '            that is the false alarm that makes an operator stop reading the mark.\n' >&2
		return 1
	}
	[ "$(wc -l <"$lm_stay/factory/log/supervisor.log")" = 1 ] || {
		printf '            it wrote into the neighbour journal anyway (%s lines):\n' \
			"$(wc -l <"$lm_stay/factory/log/supervisor.log")" >&2
		printf '              %s\n' "$(tail -n 1 "$lm_stay/factory/log/supervisor.log")" >&2
		return 1
	}
	return 0
}
proc_leg '…and says nothing when the journal stayed put' \
	'LOG-AT present, LOG-MOVED absent, neighbour journal untouched' proc_log_stay_is_silent_about_a_move

# The third state on a real pass: nothing in the window records a directory. This is the ordinary state
# of a fixture box whose unit has never run, and of a real box whose journal holds no earlier pass of
# this unit -- so the arm has to be reachable without a stub personality of its own. Absence of
# LOG-MOVED must not be the only thing the pass has to say (#228).
proc_new log-unknown
proc_run log-unknown

proc_log_unknown_is_a_sentence_not_silence() {
	local line='' moved=''
	line=$(proc_journal_line "$proc_journal" 'LOG-MOVED-UNKNOWN ')
	[ -n "$line" ] || {
		printf '            a pass that could not find a record said nothing about it, which is the\n' >&2
		printf '            same silence as a journal that has not moved:\n' >&2
		sed 's/^/              /' "$proc_journal" | cut -c1-150 >&2
		return 1
	}
	case "$line" in
	(*'rc=2'*) : ;;
	*) printf '            the window was empty of records but the sentence is not the rc=2 arm:\n' >&2
		printf '              %s\n' "$line" >&2
		printf '            rc=1 is the read failing, rc=3 a damaged record; naming the wrong one\n' >&2
		printf '            sends the operator to the wrong fix.\n' >&2
		return 1 ;;
	esac
	case "$line" in
	(*"$proc_log"*) : ;;
	*) printf '            the refusal does not say where this pass is writing:\n              %s\n' "$line" >&2
		return 1 ;;
	esac
	moved=$(proc_journal_line "$proc_journal" ' LOG-MOVED ')
	[ -z "$moved" ] || {
		printf '            a pass that read no record still reported a move:\n              %s\n' "$moved" >&2
		return 1
	}
	return 0
}
proc_leg '…and refuses in words when it cannot read' \
	'LOG-MOVED-UNKNOWN rc=2 naming this directory, no LOG-MOVED' proc_log_unknown_is_a_sentence_not_silence

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

# ------------------------------------------------- the halt line names its own age (#237)
#
# `factory/HALT` is written once and then merely read: the first thing every later activation does is
# print the stored sentence and exit. So a line carrying only that sentence says the same thing at
# 04:30 and at 14:42, and #237 is the record of what that costs -- four identical `HALTED` lines on
# 8 Nov over a tree that had been cleaned an hour before the last of them, #248 counting 115 of them
# across 15h57m. Measured again on this branch before the fix: one marker with an eight-hour-old
# mtime, three passes at 03:37:54Z, :57 and :59, three `HALTED` lines byte-identical once their own
# stamps are stripped.
#
# These legs plant the marker's mtime, because the mtime IS the instant the cause was seen --
# `factory_tree_halt` is that file's only writer -- so every claim below is about a line printed from
# a state this file set, not about a journal this file hopes for.
q_halt_plant() { # minutes_ago -> a marker whose mtime says its cause was recorded then
	local age=$1
	rm -f -- "$FACTORY_HALT"
	printf 'a working tree was found with uncommitted work: %s -- what it holds: tracked.txt\n' \
		"$q_verdict_tree" >"$FACTORY_HALT"
	touch -d "-${age} minutes" "$FACTORY_HALT"
}
q_halt_wants() { # label needle line why
	local label=$1 want=$2 line=$3 why=$4
	case "$line" in
	*"$want"*)
		pass=$((pass + 1))
		printf '  ok    %-34s %s\n' "$label" "$why"
		;;
	*)
		fail=$((fail + 1))
		printf '  FAIL  %-34s %s\n' "$label" "${line:-<the composer printed nothing>}"
		printf '          want `%s` in the line. The marker was planted aged for it, so a line\n' "$want"
		printf '          without it is the composer not saying the thing, not the state being wrong.\n'
		;;
	esac
	return 0
}

echo "factory/selftest.sh: the halt line names its own age (#237)"
q_halt_plant 90
q_halt_line=$(factory_halt_report)
q_halt_wants "halt line: names the age" "1h30m" "$q_halt_line" \
	"a cause recorded 90 minutes ago reads as 1h30m, the unit every other elapsed figure uses"
q_halt_wants "halt line: says it was not re-probed" "not re-probed" "$q_halt_line" \
	"the halted path never asks the tree again, and the line may not imply that it did"
q_halt_wants "halt line: still quotes the sentence" "tracked.txt" "$q_halt_line" \
	"the age is added to the operator's sentence, never traded for it"

# Two shapes of marker a naive reuse of #248's primitive reads as a fresh detection, both measured on
# this branch before the composer was written. It clamps -- its answer is `$((now > m ? now - m : 0))`
# -- so a marker whose time is ahead of this clock (an NTP correction after a suspend, a VM restored
# to a snapshot, a marker copied back with a future stamp) comes back 0 and the line reads `cause
# recorded 0s ago`, which is the sentence a detection three seconds old prints; planted 7200s ahead it
# printed that sentence verbatim. And when `stat` cannot read the marker it answers 999999999, which
# `factory_age_human` spells `never`, dating the cause before the machine. Both are wrong-as-its-own-
# opposite, the class #237 is filed against, so the composer asks `stat` itself and refuses both in
# words -- while still speaking the age in `factory_age_human`'s unit, which is the drift H-0113
# exists to prevent.
q_halt_plant_at() { # epoch -> a marker carrying that exact mtime
	local e=$1
	rm -f -- "$FACTORY_HALT"
	printf 'a working tree was found with uncommitted work: %s -- what it holds: tracked.txt\n' \
		"$q_verdict_tree" >"$FACTORY_HALT"
	touch -d "@$e" "$FACTORY_HALT"
}
q_halt_now=$(date -u +%s)

q_halt_plant_at "$((q_halt_now + 7200))"
q_halt_line=$(factory_halt_report)
q_halt_wants "halt line: ahead of this clock" "ahead of this clock" "$q_halt_line" \
	"a marker 7200s newer than now is said to be ahead, not clamped into a fresh detection"
case "$q_halt_line" in
*"0s ago"*)
	fail=$((fail + 1))
	printf '  FAIL  %-34s %s\n' "control: not clamped to 0s" "$q_halt_line"
	printf '          `factory_file_age` answers 0 for a time ahead of the clock, and `0s ago` is what\n'
	printf '          a detection three seconds old prints: the line would say the opposite.\n'
	;;
*)
	printf '        control: the same line never prints `0s ago`, so the ok above is a verdict about\n'
	printf '        the future time and not the generic youngest-possible answer.\n'
	;;
esac

# The other polarity, planted rather than assumed: a cause seen a few seconds ago STILL reads in
# seconds. Without it the arm above could be the composer's default, and `0s` is a reading the truth
# needs. Planted 7s back and matched to 59s so the leg cannot flake on a second boundary.
q_halt_plant_at "$((q_halt_now - 7))"
q_halt_line=$(factory_halt_report)
case "$q_halt_line" in
*"cause recorded "[0-9]"s ago"* | *"cause recorded "[0-9][0-9]"s ago"*)
	pass=$((pass + 1))
	printf '  ok    %-34s a cause seen seconds ago is still read in seconds\n' "halt line: young is young"
	;;
*)
	fail=$((fail + 1))
	printf '  FAIL  %-34s %s\n' "halt line: young is young" "$q_halt_line"
	printf '          want a seconds reading for a marker planted 7s ago. The future arm must be an\n'
	printf '          arm, not the shape every halt line takes.\n'
	;;
esac

# The unreadable time. Nothing on this filesystem reports an mtime it cannot read while the file
# exists -- `chmod 000` still answers `stat`, measured -- so the arm is reached the way `no_capture_
# says_unknown` reaches its own: shadow the command for one call, inside a substitution so it cannot
# leak into the legs below. Reachable for real when the marker is removed between `factory_halted()`
# and this read.
q_halt_time_unreadable() {
	# shellcheck disable=SC2317  # the shadow reads as unreachable code, and it is: to everything
	# except the `stat` call the composer makes. Restoring the pass-through, so the line prints an age
	# for a file whose time was never read, is this leg turning red.
	stat() { return 1; }
	factory_halt_report
}
q_halt_line=$(q_halt_time_unreadable)
q_halt_wants "halt line: an unreadable time is said" "could not be read" "$q_halt_line" \
	"an absent time is reported as absent -- no age is exactly what a fresh detection looks like"

# One line, three readers. `factory_verdict`, `factory.sh --status` and the journal's `HALTED` line
# all print this composer, for H-0044's reason one field over: readers each composing their own line
# are readers free to disagree about what whoever finds a stopped queue is told. The journal's own
# line is pinned at the process level, in `proc_halt_freezes` below, over a marker aged by that leg.
q_halt_plant 90
q_halt_line=$(factory_halt_report)
q_halt_verdict=$(factory_verdict)
if [ "$q_halt_verdict" = "HALTED $q_halt_line" ]; then
	pass=$((pass + 1))
	printf '  ok    %-34s the verdict is the composer with its verb on the front\n' "halt readers: one line"
else
	fail=$((fail + 1))
	printf '  FAIL  %-34s %s\n' "halt readers: one line" "$q_halt_verdict"
	printf '          want the verdict to be `HALTED ` plus exactly:\n            %s\n' "$q_halt_line"
	printf '          Two formats for one fact is how #237'"'"'s four prints stopped meaning anything.\n'
fi

ageless_halt_prints() { # dir -> shipped lines under factory/ that print the sentence with no age
	if [ ! -d "$1/factory" ]; then
		printf 'no factory/ directory under %s -- the scan has nothing to read\n' "$1"
		return 0
	fi
	local f
	# Each FILE is walked, not grep's matching lines: the guard has to know which function body a hit
	# sits in, and a stream filtered to the matches has no headers in it to say so. `selftest.sh` is
	# skipped because the probe carries the shape it refuses -- the controls below plant it as a
	# string, and a guard that flags its own fixture is a guard that gets deleted. The exclusion is
	# honest only because the controls are planted OUTSIDE $1, so the answer about the repository
	# stays an answer about what is shipped.
	for f in "$1"/factory/*.sh; do
		[ -f "$f" ] || continue
		case "$f" in
		*/selftest.sh) continue ;;
		esac
		awk -v path="${f#"$1"/}" '
			# A header opens a body and `^}` closes it -- the shape every function in factory/ is
			# written in, and the shape `halt_call_sites_in` already reads the same files with.
			/^[a-z_]+\(\) \{/ { fn = $1; sub(/\(\).*/, "", fn); next }
			/^\}/ { fn = ""; next }
			{
				code = $0
				sub(/#.*/, "", code)           # prose is not a print: the file that explains the
				                              # rule must not be the file the rule stops (H-0040)
				# The two allowed holders: the composer itself, and `factory_notice_halt`, which
				# quotes the sentence ONCE into the issue of the unit at the moment of detection --
				# an age there would always read `0s`, and the notice carries its own timestamp.
				if (index(code, "factory_halt_reason") && fn != "factory_halt_report" &&
					fn != "factory_notice_halt") printf "%s:%d: %s\n", path, NR, $0
			}
		' "$f"
	done | head -n 5 || true
}

ageless_halt_refused() {
	local label="no ageless halt line" root hits
	root=$(cd -- "$HERE/.." && pwd)
	hits=$(ageless_halt_prints "$root")
	if [ -n "$hits" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s a shipped script prints the stored halt without saying how old it\n' "$label"
		printf '          is, which is the line that made eleven hours read as eleven detections:\n'
		printf '%s\n' "$hits" | while IFS= read -r hit; do printf '            %s\n' "$hit"; done
	else
		pass=$((pass + 1))
		printf '  ok    %-34s every reader of the marker prints it through the composer\n' "$label"
	fi

	# Control one, the shipped shape this branch deleted, verbatim and planted outside the repository:
	# a scan that cannot see it is blind, and its silence about the tree would certify nothing.
	mkdir -p "$work/ageless/factory"
	printf '%s\n' '#!/usr/bin/env bash' \
		'# a halt reader that went back to the bare sentence' \
		'log_event "HALTED $(factory_halt_reason)"' \
		>"$work/ageless/factory/planted.sh"
	if [ -z "$(ageless_halt_prints "$work/ageless")" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s the deleted line, planted, was not flagged, so the ok above is\n' "control: $label"
		printf '          the scan being blind. Fix the pattern.\n'
	else
		printf '        control: the line this branch replaced is refused when it is planted in a\n'
		printf '        shipped script, so the ok above is the check working and not the check being blind.\n'
	fi

	# Control two, the same text inside a comment. Refusing prose is how this guard dies: the file
	# that explains the rule would be the file the rule stops (H-0040, same argument as `naive:`).
	mkdir -p "$work/ageless-prose/factory"
	printf '%s\n' '#!/usr/bin/env bash' \
		'# this used to be `log_event "HALTED $(factory_halt_reason)"`, and that is the bug' \
		'log_event "HALTED $(factory_halt_report)"' \
		>"$work/ageless-prose/factory/prose.sh"
	if [ -n "$(ageless_halt_prints "$work/ageless-prose")" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s the scan refuses a comment that only quotes the line, so it reads\n' "control: $label"
		printf '          prose and will be widened away inside a week.\n'
	else
		printf '        control: the same text inside a comment is not refused, and the line that\n'
		printf '        prints it through the composer is not refused either.\n'
	fi

	# Control three, the allowed polarity: the composer's own body must NOT be flagged. Without this
	# the ok above could just as well mean the scan matches nothing at all.
	mkdir -p "$work/ageless-allowed/factory"
	printf '%s\n' '#!/usr/bin/env bash' \
		'factory_halt_report() {' \
		'	local reason' \
		'	reason=$(factory_halt_reason)' \
		'	printf "%s" "$reason"' \
		'}' \
		>"$work/ageless-allowed/factory/composer.sh"
	if [ -n "$(ageless_halt_prints "$work/ageless-allowed")" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-34s the composer'"'"'s own body was flagged, so the guard does not know\n' "control: $label"
		printf '          which holder it allows, and every real reader would be reported too.\n'
	else
		printf '        control: the composer holding the sentence is allowed, so the refusal above\n'
		printf '        names a reader outside it and not the primitive everybody shares.\n'
	fi
	return 0
}
ageless_halt_refused

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
#   * The payloads go through the caller's OWN `--json`, `--limit` and `--jq`, run by the real jq. A
#     change to what triage.sh asks for -- its field list, its `join`, its window -- changes what
#     these legs see, so the reads are under test and not a transcription of them. That is what the
#     sentence said when this section was written, and `--json` made it untrue for the field list
#     half: the shims parsed the flag to choose a payload and then answered the whole file whatever
#     was named, so reducing a read's field list changed nothing these legs could see (#279, and the
#     measurement under `install_gh_fields`). `gh_json_payload` is what makes it true again.
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

install_gh_fields "$claim_fix"

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
# shellcheck disable=SC1090 -- installed by install_gh_fields, beside the payloads it projects
. "$here/fields.sh"
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

emit() { # emit <payload> -> the fields the caller named, then its --limit and --jq, all real jq
	local file=$1 src
	if [ ! -f "$file" ]; then
		printf 'claim-shim: no payload for this call: %s\n' "$file" >&2
		exit 8
	fi
	# `--json` used to be read only to tell two payloads apart (see the dispatch below) and then
	# dropped, so every payload here answered for fields nobody asked for. #279.
	src=$(gh_json_payload "$file" "$json") || exit 1
	if [ -n "$jqprog" ]; then
		if [ -n "$limit" ]; then
			# --limit is under test, not worked around: the window leg means nothing unless a
			# smaller --limit really returns fewer rows. A non-numeric --limit makes jq fail,
			# which is the right answer to a caller that passed one.
			printf '%s\n' "$src" | jq -c --argjson n "$limit" 'if type == "array" then .[:$n] else . end' |
				jq -r "$jqprog"
		else
			printf '%s\n' "$src" | jq -r "$jqprog"
		fi
	elif [ -n "$limit" ]; then
		printf '%s\n' "$src" | jq -c --argjson n "$limit" 'if type == "array" then .[:$n] else . end'
	else
		printf '%s\n' "$src"
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
	#
	# #241's legs found the same index independently, from the other payload: a planted team request
	# read `needs_reviewer` because the shim never emitted `reviewers.json`, and the fixture answered
	# with `api_get`'s default -- an empty list, indistinguishable from a correct answer. Two sections
	# each rediscovering one character is the reason both stories are told here: an unreachable arm
	# fails as an empty queue, never as an error.
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

# The live payloads, abridged as described in the header. The bodies are GitHub's own text, and the
# `headRefName` values are GitHub's own too -- read back per pull request with
# `gh pr view <N> --json number,headRefName,mergedAt,baseRefName` on 11 Nov, not guessed from the
# issue numbers. They do not all agree with the issue a body names, and that is the fact #277 is
# about: #229 was branch `issue-199` while naming four issues, and #240/#243 sit on the same branch.
cat >"$claim_fix/pr-merged.json" <<'JSON'
[
 {"number":234,"baseRefName":"main","headRefName":"issue-232","mergedAt":"2026-11-08T14:26:31Z",
  "body":"fix(factory): a read that answered at its success status said it could not open the file\n\nRefs #232.\n"},
 {"number":231,"baseRefName":"main","headRefName":"issue-228","mergedAt":"2026-11-08T07:08:17Z",
  "body":"fix(factory): the dirt verdict asks a content question, so a killed mutation run no longer halts the queue\n\nRefs #228.\n"},
 {"number":229,"baseRefName":"main","headRefName":"issue-199","mergedAt":"2026-11-08T01:33:46Z",
  "body":"feat(web): the navigation is in the palette and marks the page you are on\n\nRefs #199. Refs #204. Refs #207. Refs #213.\n\n## What this branch does not finish\n\n`#204` stays open on purpose, and two things it owns are not here: name the root page once, and the skip link.\n\n`#213` is untouched as an audit.\n"},
 {"number":227,"baseRefName":"main","headRefName":"issue-222","mergedAt":"2026-11-08T01:44:14Z",
  "body":"fix(capture): a frame this run did not take keeps the rev its bytes were rendered from\n\nRefs #222.\n\n### Why `#222` was reopened\n\nIt was closed as satisfied row by row, and every row was about `capture`. The row that is wrong is the field the gate reads.\n\n### What stays open, stated rather than glossed\n"},
 {"number":218,"baseRefName":"main","headRefName":"issue-202","mergedAt":"2026-11-06T00:02:30Z",
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
# The fixture is not a GraphQL server: it answers the same file whatever the query asks for, so the
# bound has to be planted here rather than derived from the query text. `CLAIM_PAGE_PRS` and
# `CLAIM_PAGE_ISSUES` trip `pageInfo.hasNextPage` on one connection each; both default to `false`,
# which is what every other leg in this section reads. Because the fixture cannot notice the query
# forgetting to ask, a leg over the shipped source does that instead -- see
# `claim_query_asks_for_its_bound`.
claim_graphql() {
	jq -cn --slurpfile i "$claim_fix/issues.json" --slurpfile c "$claim_fix/open-claims.json" \
		--argjson prs "${CLAIM_PAGE_PRS:-false}" --argjson iss "${CLAIM_PAGE_ISSUES:-false}" '
		{data: {repository: {
			pullRequests: {pageInfo: {hasNextPage: $prs}, nodes: ($c[0] | map({number: .number,
				closingIssuesReferences: {nodes: (.claims | map({number: .}))}}))},
			issues: {pageInfo: {hasNextPage: $iss}, nodes: ($i[0] | map({number: .number, stateReason: .stateReason}))}
		}}}' >"$claim_fix/graphql.json"
}
claim_graphql

# The shipped classifier, end to end, with the fixture's gh first on PATH. Nothing is stubbed between
# the two: preflight, the jq programs, the claim channels and the emit line all run.
#
# `claim_script` is for the one leg that has to run a single-edit mutant of triage.sh -- a read whose
# field list is one field short. Empty, every leg runs the shipped file, and that is the default this
# section was written with.
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
		"${claim_script:-$HERE/triage.sh}" 2>"$claim_err" |
		tr '\037' '\t' >"$claim_rows_file" || true
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
claim_row_says_nothing_about() { # claim_row_says_nothing_about <key> <phrase>
	# The negative twin of the one above, and the row has to be THERE for "nothing" to mean anything:
	# a row that vanished would pass every absence test in this file, which is the trap #235's own
	# report calls the worst outcome ("an issue that vanishes from the table is work nobody looks
	# for again"). Note the marker is absent from the WHOLE row, not just the detail -- a note that
	# leaked into another column would be a field-boundary bug worth failing on.
	local row
	row=$(row_for "$1")
	[ -n "$row" ] || {
		printf '            no row for %s, so "says nothing" would be vacuous\n' "$1" >&2
		return 1
	}
	case "$row" in
	*"$2"*)
		printf '            key %s says it after all: %s\n' "$1" "$row" >&2
		return 1 ;;
	esac
}
claim_detail_starts_with() { # claim_detail_starts_with <key> <needle>
	# The anchor contract with `factory.sh`'s `unclaimed` arm: the classifier writes the marker as the
	# first characters of the detail and the arm matches it there. Checked at the row level rather
	# than by reading either file, because the contract is about what crosses the tab.
	local d
	d=$(row_for "$1" | cut -f4)
	[ -n "$d" ] || {
		printf '            no detail for %s\n' "$1" >&2
		return 1
	}
	case "$d" in
	"$2"*) return 0 ;;
	esac
	printf '            key %s detail does not START with %s -- it is: %s\n' "$1" "$2" "$d" >&2
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
claim_detail_wider_than() { # claim_detail_wider_than <key> <columns>
	local d n
	d=$(row_for "$1" | cut -f4)
	n=${#d}
	[ "$n" -gt "$2" ] || {
		printf '            key %s detail is %s columns, not more than %s: %s\n' "$1" "$n" "$2" "$d" >&2
		return 1
	}
}
claim_rows_say_nothing_about() { # claim_rows_say_nothing_about <kind>
	! grep -q "^$1"$'\t' "$claim_rows_file"
}
claim_issue_rows_at_least() { # claim_issue_rows_at_least <n> -> the table printed at least n issue rows
	local n
	n=$(awk -F'\t' '$1 == "issue"' "$claim_rows_file" | wc -l)
	[ "$n" -ge "$1" ] || {
		printf '            the table printed %s issue rows, not %s or more\n' "$n" "$1" >&2
		return 1
	}
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
claim_query_asks_for_its_bound() { # claim_query_asks_for_its_bound <triage.sh> -> both connections ask
	# The fixture is not a GraphQL server -- it answers the same file whatever the query asks for --
	# so a query that stopped requesting `pageInfo` would keep every planted leg green. Two
	# occurrences are the two connections; a third would only ever make this more true.
	[ "$(grep -oF -- 'pageInfo{hasNextPage}' "$1" 2>/dev/null | wc -l)" -ge 2 ]
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
	# `isDraft` and `headRefName` join `reviewDecision` as pass-throughs for the same reason and with the
	# same safety: they used to be constants, and a constant is a door that cannot be opened. #291 needs
	# a DRAFT pull request in this fixture -- an issue that only a draft names is the one suppression
	# that can never resolve itself -- and a head branch of its own, because the sentence that names the
	# suppression prints it. Defaults kept identical to the constants, so all the existing callers answer
	# exactly what they answered.
	jq -c '[.[] | {number, title, headRefName: (.headRefName // "fix/planted"), isDraft: (.isDraft // false),
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
	CLAIM_PAGE_PRS=''
	CLAIM_PAGE_ISSUES=''
	# Not a fixture to reset so much as a leak to close: a leg that points the harness at a mutated
	# triage.sh for a control must not hand that copy to the next leg by accident.
	claim_script=''
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
# when, found in what window. Nothing truncates the column to hold any of that -- `emit` prints every
# field with a bare `%s` and no slice, and 12d below measures how long it can get -- so the row
# carries a number and an instant rather than a sentence about them because a person reads this table
# at a glance. The field COUNT is deliberately not quoted here: `emit` grew a seventh field on `main`
# one day after this sentence was first written, and a comment that counts fields ages the same way
# the comment that invented an 80-column cap did.
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

# 11b. the payload answering only as far as its own bound. Both connections ask for the first 100
#      rows, so a busier repository gets a true answer to a smaller question -- and the smaller
#      question is not the one `delivered` asks. The harm is asymmetric across the two sides: an
#      issue cut off the end of the issue page loses its `stateReason`, and a reopened closure with
#      its veto gone reads as DELIVERED; a pull request cut off the end hides an OPEN claim, which is
#      the duplicate tick H-0053 was written about. So the read asks each connection for
#      `pageInfo{hasNextPage}` and either `true` routes into the refusal the unreadable payload above
#      already takes -- "the read stopped" and "the read did not happen" both mean this table cannot
#      speak, and neither may print `unclaimed`. The refusal says which, because an operator reading
#      the log needs to know whether to fix the connection or the token.
claim_reset
CLAIM_PAGE_ISSUES=true
claim_issues '.'
claim_rows >/dev/null
claim_leg 'a truncated issue page is not a verdict' claim_rows_say_nothing_about issue
claim_leg '…and the merged claims go with it' bash -c '! grep -q delivered "$1"' _ "$claim_rows_file"
claim_leg '…and the refusal names the bound, not an empty queue' claim_err_has 'pagination bound'
claim_reset

# …and the same tripped on the pull-request connection, with an open pull request planted so the half
# that is NOT refused has a row to point at: the veto stops the issue half, never the whole table.
claim_open '[{"number":237,"title":"fix(web): the users page names its own blocking profile","body":"Refs #212.\n","claims":[212]}]'
CLAIM_PAGE_PRS=true
claim_graphql
claim_rows >/dev/null
claim_leg 'a truncated pull-request page is not a verdict either' claim_rows_say_nothing_about issue
claim_leg '…and the pull-request rows are untouched' claim_table_has pr 237 needs_reviewer

# The fixture answers the same file whatever the query asks for, so nothing above can notice the
# shipped query forgetting to ask. This scans the source instead, with a control that deletes both
# occurrences from a copy: a leg that cannot go quiet is a leg that is not reading anything.
claim_leg 'the queue query asks for its own bound' claim_query_asks_for_its_bound "$HERE/triage.sh"
mkdir -p "$work/nobound"
sed 's/pageInfo{hasNextPage}//g' "$HERE/triage.sh" >"$work/nobound/triage.sh"
if claim_query_asks_for_its_bound "$work/nobound/triage.sh" 2>/dev/null; then
	fail=$((fail + 1))
	printf '  FAIL  %-40s a triage.sh with both occurrences deleted still asks for a bound,\n' "control: query bound term"
	printf '          so the ok above is the scan being blind.\n'
else
	printf '        control: a copy with both deleted reads as no bound asked for, so the ok\n'
	printf '        above is the scan working and not the check being blind.\n'
fi
CLAIM_PAGE_ISSUES=
CLAIM_PAGE_PRS=
claim_reset

# 11c. the OTHER bound, which fails quietly. `gh issue list --limit 100` gives the 101st open issue no
#      row at all -- not `unclaimed`, not `delivered`, nothing -- and silence about a bound is how a
#      queue gets read as a census. The pass counts the rows it read and says so at the limit, so the
#      pair of legs is the warning and its absence at nine rows.
claim_reset
claim_issues '[range(1;101)] | map({number: ., title: "A planted open issue", updatedAt: "2026-11-01T00:00:00Z", stateReason: null})'
claim_rows >/dev/null
claim_leg 'a full issue page says the read stopped short' claim_err_has 'at its own limit'
claim_leg '…and it still printed the rows it did read' claim_issue_rows_at_least 100
claim_reset
claim_rows >/dev/null
claim_leg '…and says nothing below the limit' bash -c '! grep -q "at its own limit" "$1"' _ "$claim_err"

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

# 12d. how long the delivered row's detail can be -- a question a comment on that row used to answer
#      by claiming `emit()` caps it at 80 columns. Nothing caps anything: `emit` prints every field
#      with a bare `%s` and nothing is truncated on its way out. The sole-claim rule is what bounds the
#      phrase, and it bounds DISTINCT numbers, not characters -- so the route to a long column is a
#      list that repeats one number, and the row is still a legitimate `delivered` when it does.
#      The plant below is that body: forty `#232`s in one sentence, which is what a real body looks
#      like when someone pastes a checklist into the verb line. Measured at this commit and again
#      after the merge with `origin/main` below: the detail is 302 columns, the row is still
#      `delivered`, and the `(window 100)` tail is intact.
claim_reset
claim_merged 'map(if .number == 234 then .body = ("fix: one issue, a long list\n\nRefs "
	+ ([range(40)] | map("#232") | join(", ")) + ".\n") else . end)'
claim_rows >/dev/null
claim_leg 'a list of one number repeated is one claim' claim_class_is 232 delivered
claim_leg '…and no cap shortens what lands in the detail' claim_detail_wider_than 232 80
# The width alone is not the property. A cap at 200 would clear that leg and still eat the end of
# the row, and the end is the part a person re-checks -- so the assertion that matters is that the
# tail is still there after 302 columns of it.
claim_leg '…and the tail of the row survives the length' claim_detail_has 232 '(window 100)'
claim_reset

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

# 16. the refusal the scan used to throw away (#277). Everything above asks what the classifier
#     DECIDED. These ask what it SAID about the claims it refused, because the defect was not a wrong
#     verdict: #235's own fix arrived as PR #238, merged 8 Nov on branch `issue-235` under a body
#     naming eight issues, and the delivered scan read that body, refused it on the count rule, and
#     printed `unclaimed` with nothing further. Three days later the supervisor dispatched #235 anyway
#     -- `TICK-START unclaimed:235 2026-11-10T22:57:23Z`, `TICK-DONE 23:14:23Z` in the journal,
#     seventeen minutes of ceiling to rediscover what the scan had already read and discarded.
#
#     Four things these legs keep apart, because the defect class is those four being confused:
#
#       * THE VERDICT IS UNCHANGED. #199 and #213 are still `unclaimed` and still get their tick. A
#         row that stopped being dispatched here would retire work the refusals were correct to keep:
#         #253 was refused by the count arm at #255 and NEEDS the tick that row dispatched -- #257
#         delivered it five hours later -- and #246 sits in this very fixture refused by a `Part of`
#         that told the truth about unfinished work.
#       * THE NOTE IS ONLY ON A REFUSAL. Not on a delivered row, not on a mention that carries no
#         verb (#149, leg 6), not on an issue that spoke after the merge (#213), not on one whose
#         closure was rejected (#222), and not on anything when the merged read failed (leg 7).
#       * THE BRANCH IS A FACT, NEVER A REASON. #232 is delivered while its own `headRefName` sits in
#         the same column, and #213's note names `issue-199` -- another ticket's branch. A signal that
#         decided from that column would have claimed #246.
#       * THE TICK IS TOLD WHAT TO DO WITH IT, and the two ends are one change: the arm in
#         `factory.sh` is keyed to the marker anchored at the START of the detail, and the legs below
#         check both ends and the pairing with the two documents. Half of this, on its own, is exactly
#         what #272 was filed for.
#
#     One thing these legs cannot show, said rather than left to be inferred: the fixture is a stub,
#     so a `gh pr list` that stopped asking for `headRefName` would have been handed the field anyway
#     and every leg here would stay green. That is why the stub now projects the payload down to the
#     fields the caller named, which is what the real `gh` does. The branch-naming legs below only
#     pass while the read actually asks.
#
#     Corrected by the merge of `origin/main` (`4405119`) on 11 Nov, in the sentence that named the
#     mechanism, because the merge removed the mechanism it named. This section used to carry its own
#     copy of the projection at the `pr list --state merged` site; the trunk now supplies it once, for
#     every payload in both shims, from `gh_json_payload` -- installed beside the payloads by
#     `install_gh_fields` and obeyed by both `emit()`s (`H-0109`, PR #280). The copy was deleted rather
#     than kept beside the rule, measured rather than read: unpunctured the merged tree is
#     `437 passed, 0 failed`, and shortening the shipped merged read to
#     `--json number,body,mergedAt,baseRefName` still answers `434 passed, 3 failed` with the same
#     three branch-naming legs red as it did with the copy in place. Two copies of one rule is the
#     thing #279 was filed to end.
claim_reset
claim_rows >/dev/null
claim_leg '#199 keeps its tick' claim_class_is 199 unclaimed
claim_leg '…and the row carries the merged body it refused' claim_detail_has 199 '[merged:PR#229'
claim_leg '…the note leads the detail, as the arm expects' claim_detail_starts_with 199 '[merged:PR#'
claim_leg '…naming the branch that carried it' claim_detail_has 199 'on `issue-199`'
claim_leg '…and the instant it merged' claim_detail_has 199 '2026-11-08T01:33:46Z'
claim_leg '…and the rule that refused it' claim_detail_has 199 'names it in a 4-issue body'
claim_leg '…with the title still readable behind it' claim_detail_has 199 'Improve the design of the web UI'
claim_leg '…and a delivered row carries no note' claim_row_says_nothing_about 232 '[merged:PR#'
claim_leg '…nor the issue nothing merged has named' claim_row_says_nothing_about 235 '[merged:PR#'
claim_leg '…nor a mention that carries no verb' claim_row_says_nothing_about 149 '[merged:PR#'
claim_leg '…nor an issue that spoke after the merge' claim_row_says_nothing_about 213 '[merged:PR#'
claim_leg '…nor one whose closure was rejected' claim_row_says_nothing_about 222 '[merged:PR#'
claim_reset

# The reopen veto on its own. In the fixture above #222 is refused twice over -- REOPENED and an
# `updatedAt` later than the merge -- so the legs cannot tell which arm declined. Moving the stamp
# before the merge leaves exactly one reason, and the row must still say nothing: a person who
# reopened a ticket has already read the merged body that the reopen says did not finish it.
claim_issues 'map(if .number == 222 then .updatedAt = "2026-11-07T00:00:00Z" else . end)'
claim_rows >/dev/null
claim_leg 'the reopen veto alone still declines the note' claim_row_says_nothing_about 222 '[merged:PR#'
claim_reset

# The `Part of` arm, carrying its own reason. This is the arm that must NOT become a verdict: #246 is
# in the live queue right now because this arm refused PR #254's `Part of #246`, and #246 still owns
# a requirement nobody has built -- the `branch:` line in the note is there so the tick can go and
# look, and nothing else.
claim_merged 'map(if .number == 234 then .body = "Part of #232; the umbrella is not finished.\n" else . end)'
claim_rows >/dev/null
claim_leg '`Part of #232` still refuses the verdict' claim_class_is 232 unclaimed
claim_leg '…and names the pull request that said it' claim_detail_has 232 '[merged:PR#234'
claim_leg '…and which phrase, not the count rule' claim_detail_has 232 'a `Part of` phrase in the same body'
claim_leg '…and the branch, as a fact to go and read' claim_detail_has 232 'on `issue-232`'
claim_reset

# The sibling of a bundled claim: #213 inside #229's list. The verb matters -- `#213 and #212 shipped
# together.` is what leg 4 plants and it carries no claim verb at all, which is the #149 case (a
# cross-reference), so no claim is made and no claim is refused. `Refs` plus the list is the shape that
# reaches the count arm, and the branch named in the resulting note belongs to a DIFFERENT ticket,
# which is both why a branch name is evidence rather than a rule and why the note is worth carrying to
# a tick whose own number never appears in it. #213's stamp moves to before the merge because the
# fixture has it speaking at 02:46:41Z, seventy-three minutes after #229 merged -- the veto the leg
# eleven lines above pins -- and this leg is about the count arm, not that one.
claim_merged 'map(if .number == 229 then .body = "Refs #213 and #212.\n" else . end)'
claim_issues 'map(if .number == 213 then .updatedAt = "2026-11-07T00:00:00Z" else . end)'
claim_rows >/dev/null
claim_leg 'a bundled sibling is carried, not dropped' claim_class_is 213 unclaimed
claim_leg '…on a branch that is not its own ticket' claim_detail_has 213 'on `issue-199`'
claim_reset

# Two merged bodies naming one issue -- #199's real history, three pull requests on branch
# `issue-199`. The newest is quoted, because the newest is the one that could have finished it, and
# the count is said, because "one body mentioned it" and "three did" are different things to a reader.
claim_merged '. + [{"number":243,"baseRefName":"main","headRefName":"issue-199","mergedAt":"2026-11-09T00:08:13Z","body":"Refs #199. Refs #204.\n"}]'
claim_rows >/dev/null
claim_leg 'two merged bodies say so on one row' claim_detail_has 199 '(2 merged bodies name it)'
claim_leg '…and the newest is the one quoted' claim_detail_has 199 '[merged:PR#243'
claim_leg '…and the older one is not also named' claim_row_says_nothing_about 199 'PR#229'
claim_reset

# The note at the size that broke the other scan. This is leg 12's plant -- the 128 KiB body appended
# after `Refs #232.` -- with #199's stamp moved back before the merge, so the delivered arm's own veto
# no longer hides it. What the leg needs to show is that neither scan quits when a payload grows large:
# `delivered` keeps answering (the appended junk carries no second number, so #232 is still delivered
# exactly as leg 12 says) and a REFUSED claim keeps its note instead of quietly arriving as a bare
# title, which is the silent row this whole section is about.
claim_merged '.[0].body = .[0].body + "
" + ("x" * 131072)'
claim_issues 'map(if .number == 199 then .updatedAt = "2026-11-01T00:00:00Z" else . end)'
claim_rows >/dev/null
claim_leg 'a payload past 128 Ki still notes the refusal' claim_detail_has 199 '[merged:PR#229'
claim_leg '…and it is the whole note, not a truncated one' claim_detail_has 199 'names it in a 4-issue body] '
claim_leg '…while #232 is still delivered beside it' claim_class_is 232 delivered
claim_reset

# Both scans read the same merged payload, so the two ways that payload fails have to be refused twice
# as well. The garbage case is the one that matters: `gh` answers http 502 with prose on stdout and
# exit 0, and the second scan's jq is just as unparseable as the first -- so it says so, instead of
# printing a table of rows that quietly lost their notes.
claim_reset
claim_open '[{"number":237,"title":"fix(web): the users page names its own blocking profile","body":"Refs #212.\n","claims":[212]}]'
CLAIM_MERGED_GARBAGE=1
claim_rows >/dev/null
claim_leg 'a payload that is not JSON notes nothing' claim_row_says_nothing_about 199 '[merged:PR#'
claim_leg '…and the refused-claim scan says it refused' claim_err_has 'refused-claim scan refused'
CLAIM_MERGED_GARBAGE=
claim_reset
CLAIM_MERGED_FAIL=1
claim_rows >/dev/null
CLAIM_MERGED_FAIL=
claim_leg 'a merged read that fails notes nothing' claim_row_says_nothing_about 199 '[merged:PR#'
claim_reset

# ------------------------------------------------ the branch a delivered row names, and the draft (#291)
#
# Two halves, one measurement behind each.
#
# (1) A `delivered` row used to name the pull request and the instant and the phrase, and not the branch
# -- although the payload has carried `headRefName` on this very read since #277. A branch is deleted
# when its pull request merges, so the pass that reports delivery is the LAST moment the name exists:
# #291 had to reconstruct `issue-249` from a commit id six hours after #252 merged, because six hours
# earlier the row could have said it. The name must come out of the same row the claim came out of and
# never be assembled from the issue number -- which is the shape this repository has already refused
# twice (#254 merged on `issue-246` under `Part of #246` with #246 unfinished; #253's third instruction
# was reported delivered by #255 and was not on `main`). So the discriminating plant is a merged pull
# request whose head branch is NOT `issue-<n>`: `fix/weird-branch` naming #212. On that row the shipped
# code prints the branch it read, and a copy that builds the name from the number prints `issue-212`,
# which is a sentence about a branch that does not exist. The control runs exactly that copy.
#
# (2) An issue any open pull request names gets no row at all (triage.sh:682). While that pull request
# can still merge, the silence ends when it does. A DRAFT never merges, so the silence has no end:
# measured on the live queue at 13:36Z on 11 Nov, #241, #284 and #222 have no row of any class and the
# only thing naming them is draft #285 (`backup-tip`, title ending `reference only, do not merge`) or
# draft #233 -- #241's own fix having merged as #282 at 11:01:37Z that morning, so nobody will be told
# about that ticket again. The fix is a sentence, not a class, for the two reasons #277 measured: a
# class that starts a tick re-implements the draft, and a class that starts no tick retires real work.
# Whether #241 should be closed and #285 deleted are two judgement calls about somebody else's pull
# request, and they are the operator's; the harness's failure was hiding the pair.
#
# The polarities are the point again: a draft is reported, a mergeable pull request is not, a draft
# GitHub itself says closes the issue is work in flight and is not, and one real claim anywhere on the
# issue is enough to leave the sentence unsaid. The control deletes the filter that makes those four
# answers differ and demands the sentence fire on a pull request that can still deliver.

claim_err_says_nothing() { # claim_err_says_nothing <needle> -- paired with a positive leg beside it
	if grep -qF -- "$1" "$claim_err"; then
		printf '            stderr said %s; it said:\n' "$1" >&2
		sed 's/^/            /' "$claim_err" >&2
		return 1
	fi
}
# One exact-string edit of the shipped `triage.sh`, run as the classifier. This section's two controls
# both change text inside a jq program, where `claim_mutate_triage`'s `sed "s/$anchor/$repl/"` would
# read `\(` as a group -- and that helper is defined below this point besides, so bash has not met it
# yet when these legs run. Exact strings, one occurrence or a refusal, and the same two refusals the
# helper makes: no change at all, and a copy that does not parse.
claim_mutate_text() { # claim_mutate_text <dest> <exact old> <new>
	local dest=$work/$1/triage.sh old=$2 new=$3
	mkdir -p "$work/$1" || return 1
	cp "$HERE/common.sh" "$work/$1/common.sh" || return 1
	python3 - "$HERE/triage.sh" "$dest" "$old" "$new" <<'PY' || return 1
import io
import sys

src, dst, old, new = sys.argv[1:5]
text = io.open(src, encoding="utf-8").read()
if text.count(old) != 1:
    sys.exit("the text to replace occurs %d time(s) in factory/triage.sh, so the mutant\n"
             "would not land where it is described:\n  %s" % (text.count(old), old))
io.open(dst, "w", encoding="utf-8").write(text.replace(old, new, 1))
PY
	if ! bash -n "$dest"; then
		printf '            the mutant does not even parse\n' >&2
		return 1
	fi
	chmod +x "$dest"
	claim_script=$dest
	claim_rows >/dev/null
	claim_script=''
	# A mutant whose jq died is not a mutant that changed its verdict. The shape checks in `triage.sh`
	# are good enough that a broken program still prints a table, so the only sign is the diagnostic --
	# and a leg that goes on to assert "the bug came back" against a run that computed nothing would
	# report a reproduction it never saw. H-0131's controls failed this exact way, on `common.sh`.
	if grep -qF 'jq: error' "$claim_err"; then
		printf '            the mutant died rather than changed its verdict:\n' >&2
		tr '\n' '|' <"$claim_err" | sed 's/^/            /' >&2
		printf '\n' >&2
		return 1
	fi
}

claim_rows >/dev/null
claim_leg 'the delivered row names the branch that carried it' claim_detail_has 232 'on `issue-232`'

# A head branch that is not the issue's own name, so a name built from the number cannot pass for it.
claim_merged '. + [{number: 258, baseRefName: "main", headRefName: "fix/weird-branch",
	mergedAt: "2026-11-09T00:00:00Z", body: "fix(capture): the rig names the rev its frames came from\n\nRefs #212.\n"}]'
claim_rows >/dev/null
claim_leg '…and it is the branch the payload named' claim_detail_has 212 'on `fix/weird-branch`'
claim_leg '…not the one the issue number would suggest' claim_row_says_nothing_about 212 'issue-212'

claim_branch_from_number() { # the mutant: the branch assembled from the number, as `#291` forbids
	claim_mutate_text delivmut \
		'else " on `\($c.branch)`" end)
		    + " at \($c.at) says' \
		'else " on `issue-\($c.issue)`" end)
		    + " at \($c.at) says'
}
if claim_branch_from_number; then
	if claim_detail_has 212 'on `issue-212`'; then
		printf '        control: the same fixture through a copy that builds the branch name from the\n'
		printf '        issue number prints `issue-212` for a pull request that sat on\n'
		printf '        `fix/weird-branch` -- so the ok above is the row quoting the payload, and not\n'
		printf '        a name that happens to agree with it.\n'
		claim_rows >/dev/null
	else
		fail=$((fail + 1))
		printf '  FAIL  %-40s a branch name built from the number printed the same thing the\n' "control: triage read"
		printf '          shipped code prints, so the ok above proves nothing about where the\n'
		printf '          name came from. The mutant answered: %s\n' "$(row_for 212)"
	fi
else
	fail=$((fail + 1))
	printf '  FAIL  %-40s the mutant refused to build, so nothing was compared\n' "control: triage plant"
fi
claim_reset

# A merged row whose head branch GitHub no longer names at all: the row keeps its verdict and keeps
# quiet about a branch, rather than printing a name nobody read.
claim_merged '. + [{number: 259, baseRefName: "main", headRefName: null,
	mergedAt: "2026-11-09T00:00:00Z", body: "feat: packages\n\nCloses #149.\n"}]'
claim_rows >/dev/null
claim_leg 'a merged row with no head branch says nothing about one' claim_row_says_nothing_about 149 'on `'
claim_leg '…and it is still delivered' claim_class_is 149 delivered
claim_reset

# The draft, exactly as the live one is shaped: not a draft of a work in progress, but a tombstone that
# says do not merge in its title, on a `backup/`-shaped branch, naming the issue with `Refs`.
claim_open '[{"number":260,"title":"do not merge: the record of a drop","body":"Refs #212.\n","claims":[],
	"isDraft":true,"headRefName":"backup-tip"}]'
claim_rows >/dev/null
claim_leg 'an issue only a draft names is named on stderr' claim_err_has '#212 (named only by #260 on `backup-tip`)'
claim_leg '…and it still gets no row, because a row is an offer' claim_no_row 212
claim_reset

# The same pull request one click from ready is a different fact, and the sentence has to know that.
claim_open '[{"number":260,"title":"the record of a drop, marked ready","body":"Refs #212.\n","claims":[],
	"isDraft":false,"headRefName":"issue-212"}]'
claim_rows >/dev/null
claim_leg 'a pull request that can still deliver is not named' claim_err_says_nothing 'named only by'
claim_leg '…and its issue is still the one row that is missing' claim_no_row 212

# GitHub's own `closingIssuesReferences` is the other way a draft is work in progress rather than a
# tombstone: the pull request is a draft today and the link says what it will close.
claim_open '[{"number":260,"title":"work in progress on the capture rig","body":"Refs #212.\n","claims":[212],
	"isDraft":true,"headRefName":"issue-212"}]'
claim_rows >/dev/null
claim_leg 'a draft GitHub says closes the issue is in flight' claim_err_says_nothing 'named only by'

# One real claim anywhere outranks the tombstone, even beside one.
claim_open '[{"number":260,"title":"do not merge: the record of a drop","body":"Refs #212.\n","claims":[],
		"isDraft":true,"headRefName":"backup-tip"},
	{"number":261,"title":"the capture rig, again","body":"Fixes #212.\n","claims":[],
		"isDraft":false,"headRefName":"issue-212"}]'
claim_rows >/dev/null
claim_leg 'one claim from a pull request that can merge is enough' claim_err_says_nothing 'named only by'
claim_leg '…while the issue stays off the table either way' claim_no_row 212

claim_draft_filter_off() { # the mutant: every claim counts as draft-hidden
	claim_mutate_text draftmut \
		'| [ $rows[] | select(.draft) | .issue as $i | select(($live | index($i)) | not) ]' \
		'| [ $rows[] ]'
}
# The plant still in the fixture is the pair above: the issue is named by a draft AND by a mergeable
# pull request, which is the case the shipped code stays silent about.
if claim_draft_filter_off; then
	if claim_err_has 'named only by'; then
		printf '        control: the same fixture, with the filter that separates a draft from a\n'
		printf '        mergeable pull request deleted, names an issue a real pull request is still\n'
		printf '        about to deliver -- so the ok above is the filter deciding, not the sentence\n'
		printf '        never being reached.\n'
	else
		fail=$((fail + 1))
		printf '  FAIL  %-40s deleting the draft filter changed nothing, so the ok above is\n' "control: triage read"
		printf '          the sentence being unreachable for a reason of its own.\n'
	fi
else
	fail=$((fail + 1))
	printf '  FAIL  %-40s the mutant refused to build, so nothing was compared\n' "control: triage plant"
fi
claim_reset

# The other half of the change: `factory.sh`'s `unclaimed` arm turns the marker into the instruction
# that tells the tick what the bracket means, and nothing in the repository but these legs says the two
# halves have to move together. `#272` is the precedent -- its third instruction was shipped by a pull
# request that changed `triage.sh` and `selftest.sh` and left `docs/factory.md` on `grep -c yield` = 0.
unclaimed_note_instructs() { # unclaimed_note_instructs <dir holding factory.sh> -> the arm is keyed
	local f=$1/factory.sh arm
	[ -f "$f" ] || {
		printf '            no factory.sh at %s -- the argument is the directory holding it\n' "$f" >&2
		return 2
	}
	# The arm, from its label to the first `;;` that closes a case entry inside it. The inner
	# `case "$detail"` is why the window ends at the first `;;` rather than at the arm's own: the
	# instruction is inside the window either way, and a window that ran to the arm's end would
	# include the next class and pass on someone else's text.
	arm=$(awk '/^[[:space:]]*unclaimed\)/{f=1} f{print} f&&/^[[:space:]]*;;[[:space:]]*$/{exit}' "$f")
	[ -n "$arm" ] || {
		printf '            no `unclaimed)` arm in %s\n' "$f" >&2
		return 1
	}
	# Anchored at the start of the pattern, because the unanchored form is the failure being guarded:
	# the rest of the column is an issue title somebody typed, and `*MARK*` would let a title that
	# happens to contain the token buy itself an instruction the classifier never wrote.
	printf '%s' "$arm" | grep -qE '^[[:space:]]*"\$FACTORY_MERGED_MARK"\*\)' || {
		printf '            the arm does not match the shared marker anchored at the detail start\n' >&2
		return 1
	}
	printf '%s' "$arm" | grep -qF -- 'already satisfied by' || {
		printf '            the arm keys on the marker but sends the tick no instruction\n' >&2
		return 1
	}
}
unclaimed_note_control() { # each mutant loses one of the three things the ok above claims
	local ctrl=$claim_work/note-arm m built=1
	for m in no-marker unanchored no-instruction; do
		mkdir -p "$ctrl/$m" || { built=0; continue; }
		case $m in
		no-marker)
			grep -vF -- '"$FACTORY_MERGED_MARK"*)' "$HERE/factory.sh" >"$ctrl/$m/factory.sh" ;;
		unanchored)
			sed 's|^[[:space:]]*"\$FACTORY_MERGED_MARK"\*)|*"$FACTORY_MERGED_MARK"*)|' \
				"$HERE/factory.sh" >"$ctrl/$m/factory.sh" ;;
		no-instruction)
			sed 's/already satisfied by/no such phrase in/' "$HERE/factory.sh" >"$ctrl/$m/factory.sh" ;;
		esac
		[ -s "$ctrl/$m/factory.sh" ] || built=0
	done
	if [ "$built" != 1 ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s a control copy could not be written\n' 'control: the note, the arm'
		return 0
	fi
	# Each copy has to be checked for what it lost BEFORE it is asked to fail, or a copy that lost
	# everything fails for the wrong reason and the leg above is never aimed at what it names.
	grep -qF -- '"$FACTORY_MERGED_MARK"*)' "$ctrl/no-marker/factory.sh" && {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the no-marker copy still holds the marker arm,\n' 'control: the note, the arm'
		printf '          so the ok above was never aimed at the arm it names.\n'
		return 0
	}
	grep -qE '^[[:space:]]*\*"\$FACTORY_MERGED_MARK"\*\)' "$ctrl/unanchored/factory.sh" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the unanchored copy did not become a substring match,\n' 'control: the note, the arm'
		printf '          so nothing tested the anchor the leg above claims.\n'
		return 0
	}
	grep -qF -- 'already satisfied by' "$ctrl/no-instruction/factory.sh" && {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the no-instruction copy still carries the instruction,\n' 'control: the note, the arm'
		printf '          so the ok above does not need it.\n'
		return 0
	}
	for m in no-marker unanchored no-instruction; do
		if unclaimed_note_instructs "$ctrl/$m" 2>/dev/null; then
			fail=$((fail + 1))
			printf '  FAIL  %-40s it accepted the `%s` copy\n' 'control: the note, the arm' "$m"
			printf '          so the ok above is a grep that any copy passes.\n'
			return 0
		fi
	done
	printf '  ok    %-40s each copy loses one of the three halves\n' \
		'control: the note, the arm'
	printf '          and the leg says no -- the arm, the anchor, and the instruction are each read.\n'
	return 0
}
claim_leg '…and the arm is keyed to the shared marker' unclaimed_note_instructs "$HERE"
unclaimed_note_control

# Both documents have to keep saying it, for the reason #272 was filed: PR #253 shipped the classifier
# half and left the guide on `grep -c yield` = 0, and the tick's behaviour is only reproducible if the
# half that lives in `SKILL.md` -- what the bracket means and what to do with it -- travels with the
# half that lives here -- that the marker is evidence and not a verdict. Four clauses again, each a
# whole phrase, because `grep -qF 'merged'` over a 900-line guide is satisfied by any mention of the
# word and the leg would print ok while reading a different sentence.
merged_note_docs() { # merged_note_docs <root> -> the rule and the reason, in both documents
	local root=$HERE/.. skill guide
	[ -z "${1:-}" ] || root=$1
	skill=$root/.opencode/skills/factory/SKILL.md
	guide=$root/docs/factory.md
	[ -f "$skill" ] && [ -f "$guide" ] || return 2
	# The guide: the class rule, and the measured cost that made it worth a row at all.
	grep -qF 'never a verdict' "$guide" &&
		grep -qF 'already read and discarded' "$guide" &&
		# The tick's own document: what the bracket is, and what it is not licensed to conclude.
		grep -qF 'REFUSED claim, not a verdict' "$skill" &&
		grep -qF 'does not make the ticket done' "$skill"
}
merged_note_docs_control() {
	local ctrl=$claim_work/merged-note-docs
	if ! mkdir -p "$ctrl/rules/docs" "$ctrl/rules/.opencode/skills/factory" \
		"$ctrl/reasons/docs" "$ctrl/reasons/.opencode/skills/factory"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control trees could not be written\n' \
			'control: the note, both documents'
		return 0
	fi
	# One clause out of each document, twice: the two rule clauses, then the two reason clauses. Same
	# shape as `voided_docs_control` two sections above, which is the shape that caught #272.
	if ! grep -vF 'never a verdict' "$HERE/../docs/factory.md" \
			>"$ctrl/rules/docs/factory.md" ||
		! grep -vF 'REFUSED claim, not a verdict' "$HERE/../.opencode/skills/factory/SKILL.md" \
			>"$ctrl/rules/.opencode/skills/factory/SKILL.md" ||
		! grep -vF 'already read and discarded' "$HERE/../docs/factory.md" \
			>"$ctrl/reasons/docs/factory.md" ||
		! grep -vF 'does not make the ticket done' "$HERE/../.opencode/skills/factory/SKILL.md" \
			>"$ctrl/reasons/.opencode/skills/factory/SKILL.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control copies could not be written\n' \
			'control: the note, both documents'
		return 0
	fi
	if grep -qF 'never a verdict' "$ctrl/rules/docs/factory.md" ||
		grep -qF 'REFUSED claim, not a verdict' "$ctrl/rules/.opencode/skills/factory/SKILL.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the rule plant left a rule in place, so the\n' \
			'control: the note, both documents'
		printf '          leg above was never aimed at the clauses it names.\n'
		return 0
	fi
	if grep -qF 'already read and discarded' "$ctrl/reasons/docs/factory.md" ||
		grep -qF 'does not make the ticket done' "$ctrl/reasons/.opencode/skills/factory/SKILL.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the reason plant left a reason in place, so\n' \
			'control: the note, both documents'
		printf '          the leg above does not need the reasons it claims to need.\n'
		return 0
	fi
	if merged_note_docs "$ctrl/rules" || merged_note_docs "$ctrl/reasons"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it accepted a pair with one rule clause deleted\n' \
			'control: the note, both documents'
		printf '          from one document and one reason clause from the other, so the ok above is a\n'
		printf '          phrase match and not the four halves it names.\n'
		return 0
	fi
	printf '  ok    %-40s each copy loses one clause from each document\n' \
		'control: the note, both documents'
	printf '          and the leg says no; all four are read, not a word nearby.\n'
	return 0
}
claim_leg 'the note is in both documents' merged_note_docs
merged_note_docs_control

# ------------------------------------------------------ the field list, as a contract (#279)
#
# Everything above asks triage.sh questions about a queue. These ask the FIXTURE whether it answers
# the way `gh` does, and they exist because for six of its seven payload sites the answer was no:
# both shims' `emit()` parsed `--json` to choose WHICH file to print and then printed the whole file.
# A read that stopped requesting a field therefore kept receiving it from the fixture, and every leg
# that asserts something about that field stayed green while the shipped read went blind. That is the
# inverse of the hazard this section's own header promises against, and #279 is the second time in a
# week the promise was the thing that was false -- H-0108 (PR #278) was written while adding
# `headRefName` to the merged read, because the branch-naming legs would otherwise have proved
# nothing, and it projected that one channel at its one call site.
#
# Three groups, for the three shapes the blindness takes:
#
#   A  a field the caller did NOT name does not come back;
#   B  a field the payload does NOT carry is refused out loud, not answered with an empty column;
#   C  the shipped read, one field shorter, stops producing the verdict that field carries.
#
# B is the one that keeps the fixture from quietly becoming the specification. `labels` is a field
# `gh issue list` answers for every issue on GitHub and this fixture has never held; asked for before
# #279 it came back as no column at all, at exit 0, which is indistinguishable from the repository
# saying "no labels". A is the one #279 measured. C is the demonstration the issue said to run first:
# it is written here as a leg so the plant cannot rot back into a sentence in a pull request.
claim_reset
claim_gh() { # claim_gh <gh args…> -> the claim fixture's own gh, payload directory wired in
	PATH="$claim_bin:$PATH" CLAIM_FIXTURE="$claim_fix" "$claim_bin/gh" "$@"
}
claim_proc_gh() { # claim_proc_gh <gh args…> -> the PROCESS fixture's gh. Its record file is a scratch
	# one: this probe is not an activation, and a line in $proc_rec would be a gh call no supervisor
	# pass made in the journal every leg above reads.
	PATH="$proc_bin:$PATH" PROC_FIXTURE="$proc_fix" \
		PROC_GH_REC="$work/fields-proc.rec" "$proc_bin/gh" "$@"
}
# A. the projection, on stdout, where the shipped read will look at it.
claim_json_names_only() { # --json number,title answers exactly number and title, on every row
	local out rc=0 err=$work/fields.err
	out=$(claim_gh issue list --state open --limit 100 --json number,title 2>"$err") || rc=$?
	if [ "$rc" != 0 ]; then
		printf '            the read failed (rc %s): %s\n' "$rc" "$(tr '\n' '|' <"$err")" >&2
		return 1
	fi
	# `stateReason` is in issues.json and is not in the field list. It used to be handed over, and
	# triage.sh reads stateReason from GraphQL -- triage.sh:154 records why not from `gh issue list`
	# -- so nothing else here can tell the two channels apart.
	if ! printf '%s' "$out" |
		jq -e 'length > 0 and all(.[]; (keys | sort) == ["number","title"])' >/dev/null; then
		printf '            --json number,title answered: %s\n' "${out:0:160}" >&2
		return 1
	fi
}
claim_proc_json_names_only() { # …and the process shim, which serves the whole supervisor run
	local out rc=0
	out=$(claim_proc_gh issue list --state open --limit 100 --json number,title 2>&1) || rc=$?
	if [ "$rc" != 0 ]; then
		printf '            the read failed (rc %s): %s\n' "$rc" "${out:0:160}" >&2
		return 1
	fi
	if ! printf '%s' "$out" |
		jq -e 'length > 0 and all(.[]; (keys | sort) == ["number","title"])' >/dev/null; then
		printf '            --json number,title answered: %s\n' "${out:0:160}" >&2
		return 1
	fi
}
claim_json_answers_known() { # the same call with a field the payload DOES carry is not refused
	local out rc=0
	out=$(claim_gh issue list --state open --json number,updatedAt 2>&1) || rc=$?
	if [ "$rc" != 0 ]; then
		printf '            a field the payload carries was refused (rc %s): %s\n' "$rc" "${out:0:160}" >&2
		return 1
	fi
	if ! printf '%s' "$out" | jq -e 'length > 0 and all(.[]; has("updatedAt"))' >/dev/null; then
		printf '            --json number,updatedAt answered: %s\n' "${out:0:160}" >&2
		return 1
	fi
}
# B. the refusal, shaped like `gh`'s own: a non-zero exit that names the field.
claim_json_refuses_unknown() {
	local out rc=0
	out=$(claim_gh issue list --state open --json number,updatedAt,labels 2>&1) || rc=$?
	if [ "$rc" = 0 ]; then
		printf '            --json …,labels exited 0 answering %s, so a field this fixture has\n' "${out:0:80}" >&2
		printf '            never carried reads as GitHub saying "none", and the fixture is the spec.\n' >&2
		return 1
	fi
	case "$out" in
	*'Unknown JSON field: "labels"'*) return 0 ;;
	esac
	printf '            it refused (rc %s) without naming the field: %s\n' "$rc" "${out:0:200}" >&2
	return 1
}
# C. the plant. One field off one read -- `factory/triage.sh:563`, the queue read -- planted the way
#    proc_mutate plants its two: the anchor must occur exactly once, or the leg under it is the
#    fixture being blind rather than the read being short. Measured before gh_json_payload existed,
#    this exact edit left the suite at `387 passed, 0 failed` while #232 still printed `delivered`.
claim_mutate_triage() { # <dest> <anchor> <replacement> -> one edit of the shipped triage.sh, or a refusal
	local dest=$1 anchor=$2 repl=$3 hits
	mkdir -p "$work/mutant"
	cp "$HERE/common.sh" "$work/mutant/common.sh" || return 1
	hits=$(awk -v a="$anchor" 'index($0, a) { c++ } END { print c + 0 }' "$HERE/triage.sh")
	[ "$hits" = 1 ] || {
		printf '            the anchor occurs %s time(s) in factory/triage.sh, so the edit would\n' "$hits" >&2
		printf '            not land where it is described. Anchor:\n              %s\n' "$anchor" >&2
		return 1
	}
	sed "s/$anchor/$repl/" "$HERE/triage.sh" >"$dest" || return 1
	if ! grep -qF -- "$repl" "$dest"; then
		printf '            the edit is not in the copy, so the leg under it ran shipped code\n' >&2
		return 1
	fi
	if grep -qF -- "$anchor" "$dest"; then
		printf '            the shipped field list is still in the copy beside the edit, so the\n' >&2
		printf '            read would not have been short after all\n' >&2
		return 1
	fi
	chmod +x "$dest"
	return 0
}
claim_leg 'the shim answers only the fields named' claim_json_names_only
claim_leg '…and a named field still arrives' claim_json_answers_known
claim_leg '…and the process shim obeys it too' claim_proc_json_names_only
claim_leg 'a field the fixture lacks is refused' claim_json_refuses_unknown
claim_leg '…and a field it carries is not' claim_json_answers_known
claim_leg 'the plant edits one read, once' claim_mutate_triage \
	"$work/mutant/triage.sh" '--json number,title,updatedAt' '--json number,title'
claim_script=$work/mutant/triage.sh
claim_rows >/dev/null
claim_leg 'a read one field short delivers nothing' claim_class_is 232 unclaimed
claim_script=''
# The shipped read beside it, so the FAIL above cannot be the mutant dir being broken rather than the
# field being load-bearing: the same fixture, the same legs, the file as it ships.
claim_rows >/dev/null
claim_leg '…while the shipped read still does' claim_class_is 232 delivered
claim_reset

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
# `FACTORY_REVIEWER_TEAM` is pinned to the shipped default (`factory/common.sh:83`) for the same reason
# `FACTORY_OPENCODE` is: inherited, the two legs that read this journal would be green or red depending
# on what the operator exports. It is also the line that makes the ask below mean something -- the poke
# prints the target it resolved, so `request review from team workers` is the shipped default asking
# end to end through factory.sh, not a login pasted here.
ask_dispatch() { # run the shipped supervisor, dry, over whatever is planted
	mkdir -p "$claim_work/dtrees"
	PATH="$claim_bin:$PATH" CLAIM_FIXTURE="$claim_fix" \
		FACTORY_DIR="$claim_harness" FACTORY_ROOT="$claim_repo" \
		FACTORY_REPO=example/repo FACTORY_REVIEWER=someoneelse \
		FACTORY_REVIEWER_TEAM=workers \
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
# it as quoted material, and the branch for a missing ask ends in the escalation label and a stop
# rather than a guess.
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
	*FACTORY_ESCALATION_LABEL*) return 0 ;;
	esac
	printf '            the empty-ask branch ends without the escalation label (the name comes\n' >&2
	printf '            from `common.sh`, never spelled out) and a stop, so the one move that\n' >&2
	printf '            invents an ask is still open to it\n' >&2
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

# =============== the escalation label: one name, one home, and no vote in the queue (#315)
#
# `AGENTS.md` tells a stopped unit to label the artifact and stop. Which label, the repository cannot
# say: `gh label list` answers eighteen names and the one every file used to spell out is not among
# them (measured 12 Nov). #311 owns that question -- create the name, or point the instruction at a
# label that exists -- and it is parked, so it is not this unit's to answer. What is true whichever way
# it is answered is the shape pinned here, and it is the half that could land while #311 waits:
#
#   * the name lives in ONE file. Five used to spell it out in full -- `AGENTS.md`, `SKILL.md` twice,
#     `docs/factory.md`, and the tick instruction in `factory/factory.sh` -- with a sixth, a leg in
#     this file, pinning one of the five. A name in five files is a name that goes stale in one of them
#     and stays green in the other four, which is how an instruction came to order a write GitHub
#     refuses (a probe on 12 Nov, `gh issue edit 311 --add-label zzprobe-does-not-exist`, printed
#     `'zzprobe-does-not-exist' not found` and exited 1, and the labels read back unchanged);
#   * there is ONE way to read it -- `factory/escalation-label.sh` -- and what it prints is the bytes
#     the assignment in `common.sh` holds, not a second copy of them;
#   * the marker has NO say in what the supervisor starts. Measured 12 Nov, `grep -c label
#     factory/triage.sh` answers 0 and the classifier's two issue classes are `unclaimed` and
#     `delivered`. A label the queue obeyed would move the queue's decisions into configuration that
#     no gate reads, and #315 asked by name that this fix not become that.
#
# Every leg below READS the name from the reader instead of typing it. A leg that spelled it would be
# the sixth copy of the string this unit exists to delete, and it would keep passing after a rename.
esc_live() { # the files whose mentions are instructions, one path per line
	printf '%s\n' factory/common.sh factory/escalation-label.sh factory/factory.sh \
		factory/triage.sh factory/selftest.sh AGENTS.md docs/factory.md \
		.opencode/skills/factory/SKILL.md
}
# The two decision logs are NOT in that list, and that is deliberate rather than an oversight: an
# entry records what was believed when it was written, so re-editing history to match a renamed label
# turns every citation in it into a lie. A name in a log is a fossil. A name in one of the seven files
# above is an instruction some later tick carries into a write against GitHub.

esc_ctrl_copy() { # esc_ctrl_copy <dest-root> -- the live files, outside the repository
	local dest=$1 f
	mkdir -p "$dest" || return 1
	while IFS= read -r f; do
		mkdir -p "$dest/$(dirname -- "$f")" || return 1
		cp "$HERE/../$f" "$dest/$f" || return 1
	done <<LIST
$(esc_live)
LIST
	return 0
}

esc_one_home() { # esc_one_home [root] -- the name is spelled once, and that once is common.sh
	local root=$HERE/.. name f hit bad=0
	local -a files=(factory/common.sh factory/escalation-label.sh factory/factory.sh
		factory/triage.sh factory/selftest.sh AGENTS.md docs/factory.md
		.opencode/skills/factory/SKILL.md)
	[ -z "${1:-}" ] || root=$1
	if ! name=$(bash "$root/factory/escalation-label.sh" 2>&1); then
		printf '            factory/escalation-label.sh refuses: %s\n' "$name" >&2
		printf '            Nothing can then be compared, and a unit that stops cannot even name\n' >&2
		printf '            the label it is about to write.\n' >&2
		return 1
	fi
	[ -n "$name" ] || {
		printf '            the reader prints an empty name: `--add-label \"\"` is a marker aimed\n' >&2
		printf '            at nothing, and it is the write that would look like an escalation\n' >&2
		return 1
	}
	if [ "$(grep -c '^FACTORY_ESCALATION_LABEL=' "$root/factory/common.sh")" -ne 1 ]; then
		printf '            common.sh does not hold exactly one `FACTORY_ESCALATION_LABEL=` line,\n' >&2
		printf '            so there is no one place a rename lands\n' >&2
		return 1
	fi
	for f in "${files[@]}"; do
		[ -f "$root/$f" ] || continue
		hit=$(grep -cF -- "$name" "$root/$f" 2>/dev/null || true)
		if [ "$f" = factory/common.sh ]; then
			[ "${hit:-0}" -ge 1 ] || {
				printf '            common.sh, the one home, does not contain the name at all\n' >&2
				bad=1
			}
		elif [ "${hit:-0}" -ne 0 ]; then
			printf '            %s spells the name out (%s line%s) -- a place to go stale while\n' \
				"$f" "$hit" "$([ "$hit" -eq 1 ] || printf s)" >&2
			printf '            the home moves and this leg passes on the other six\n' >&2
			grep -nF -- "$name" "$root/$f" | sed 's/^/              /' >&2
			bad=1
		fi
	done
	[ "$bad" -eq 0 ]
}
claim_leg 'the escalation label is spelled once' esc_one_home

esc_reader_agrees() { # esc_reader_agrees [root] -- what it prints IS the assignment's text
	local root=$HERE/.. out def lines
	[ -z "${1:-}" ] || root=$1
	if [ ! -x "$root/factory/escalation-label.sh" ]; then
		printf '            factory/escalation-label.sh is not executable, and the command every\n' >&2
		printf '            document tells a stopped unit to run is `$(factory/escalation-label.sh)`\n' >&2
		return 1
	fi
	if ! out=$(cd / && "$root/factory/escalation-label.sh" 2>&1); then
		printf '            it refuses from an unrelated directory: %s\n' "$out" >&2
		return 1
	fi
	lines=$(printf '%s\n' "$out" | wc -l)
	[ "$lines" -eq 1 ] || {
		printf '            it printed %s lines: the whole output is pasted into --add-label, so\n' "$lines" >&2
		printf '            a banner or a warning here is a label name with a sentence attached\n' >&2
		return 1
	}
	# Read the literal out of the assignment line rather than sourcing the file: comparing the reader
	# against a second sourcing of the same line proves nothing, while this proves the line's own text
	# is what a tick writes to GitHub.
	def=$(awk -F':-|}' '/^FACTORY_ESCALATION_LABEL=/{print $2}' "$root/factory/common.sh")
	[ -n "$def" ] || {
		printf '            the assignment in common.sh is no longer the `:-…}` shape the leg\n' >&2
		printf '            reads, so the comparison below has stopped comparing anything\n' >&2
		return 1
	}
	[ "$out" = "$def" ] || {
		printf '            the command prints `%s` and common.sh defines `%s`. One of the two is a\n' \
			"$out" "$def" >&2
		printf '            copy, and the copy is what ends up on the artifact\n' >&2
		return 1
	}
	return 0
}
claim_leg 'the reader prints the definition' esc_reader_agrees

esc_not_a_park() { # esc_not_a_park [triage.sh to read] -- the marker decides nothing about a unit
	local src=${1:-$HERE/triage.sh} hits
	# Comment lines may mention the word freely -- this is about what the classifier DOES, and a leg
	# that reddened on a sentence of prose would be widened away inside a month.
	hits=$(grep -nF -- 'label' "$src" 2>/dev/null |
		grep -v '^[0-9]*:[[:space:]]*#' || true)
	if [ -n "$hits" ]; then
		printf '            the classifier reads or writes a label, so a marker now decides what\n' >&2
		printf '            the supervisor starts -- and no gate reads label configuration:\n' >&2
		printf '%s\n' "$hits" | sed 's/^/              /' >&2
		return 1
	fi
	return 0
}
claim_leg 'the label has no vote in the queue' esc_not_a_park

esc_controls() {
	# Three plants, outside the repository, one per leg above. Each is checked to have landed before
	# it is asserted, because a plant that did not land and a leg that is blind are the same green.
	local ctrl=$claim_work/esc-label name
	name=$(bash "$HERE/escalation-label.sh" 2>/dev/null) || name=''
	[ -n "$name" ] || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the reader gave the controls nothing to plant\n' \
			'control: the escalation label'
		return 0
	}
	if ! mkdir -p "$ctrl/drift" "$ctrl/stub" "$ctrl/nodef" "$ctrl/park"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control trees could not be written\n' \
			'control: the escalation label'
		return 0
	fi
	# 1. A sixth mention, planted in prose the way it would arrive in real life: someone writing the
	#    command out because it is shorter than reading it.
	esc_ctrl_copy "$ctrl/drift" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the drift copy could not be written\n' 'control: the escalation label'
		return 0
	}
	printf '\ngh issue edit 7 --add-label %s\n' "$name" >>"$ctrl/drift/AGENTS.md" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the mention could not be planted\n' 'control: the escalation label'
		return 0
	}
	if ! grep -qF -- "$name" "$ctrl/drift/AGENTS.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant is not in the copy, so the leg above was never aimed\n' \
			'control: the escalation label'
		printf '          at a sixth mention\n'
		return 0
	fi
	if esc_one_home "$ctrl/drift" 2>/dev/null; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s a copy with the name spelled out in AGENTS.md still passes, so the\n' \
			'control: one home'
		printf '          ok above counts nothing but the file it happens to read first\n'
	else
		printf '        control: a copy with a sixth mention planted in AGENTS.md is refused, so the\n'
		printf '        ok above is a census of every live file and not of one.\n'
	fi
	# 2. A reader that no longer reads: the stub prints its own literal, and the definition is
	#    untouched. This is the plant that says the leg compares two things rather than one.
	esc_ctrl_copy "$ctrl/stub" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the stub copy could not be written\n' 'control: the escalation label'
		return 0
	}
	if ! { printf '%s\n' "#!/usr/bin/env bash" "printf 'not-the-definition\\n'" \
		>"$ctrl/stub/factory/escalation-label.sh" &&
		chmod +x "$ctrl/stub/factory/escalation-label.sh"; }; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the stub reader could not be planted\n' 'control: the escalation label'
		return 0
	fi
	if ! grep -qF 'not-the-definition' "$ctrl/stub/factory/escalation-label.sh"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the stub is not in the copy, so the leg above was never aimed\n' \
			'control: the reader'
		printf '          at a reader that stopped reading\n'
		return 0
	fi
	if esc_reader_agrees "$ctrl/stub" 2>/dev/null; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s a reader that prints its own literal still passes, so the leg is\n' \
			'control: the reader'
		printf '          reading one side and calling it an agreement\n'
	else
		printf '        control: a reader stubbed to print its own literal is refused, so the ok above\n'
		printf '        really is the two spellings being compared.\n'
	fi
	# 3. The home deleted. The reader must refuse rather than answer empty, and both legs must go red
	#    on the refusal -- an escalation that prints nothing is a `--add-label` aimed at no name.
	esc_ctrl_copy "$ctrl/nodef" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the no-definition copy could not be written\n' \
			'control: the escalation label'
		return 0
	}
	grep -v '^FACTORY_ESCALATION_LABEL=' "$HERE/common.sh" >"$ctrl/nodef/factory/common.sh" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the definition could not be removed\n' 'control: the escalation label'
		return 0
	}
	if bash "$ctrl/nodef/factory/escalation-label.sh" >/dev/null 2>&1; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the reader printed a name with no definition behind it, so a tick\n' \
			'control: no definition'
		printf '          would take an invented label to GitHub and believe the marker landed\n'
	elif esc_one_home "$ctrl/nodef" 2>/dev/null || esc_reader_agrees "$ctrl/nodef" 2>/dev/null; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s a tree with no definition still passes a leg above\n' \
			'control: no definition'
	else
		printf '        control: with the assignment deleted the reader refuses and both legs above go\n'
		printf '        red, so neither one is reading a string it carried with it.\n'
	fi
	# 4. The park this unit must not become: one label test planted in the classifier.
	esc_ctrl_copy "$ctrl/park" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the park copy could not be written\n' 'control: the escalation label'
		return 0
	}
	printf '\tcase $labels in\n\t*"$name"*) class=delivered ;;\n\tesac\n' \
		>>"$ctrl/park/factory/triage.sh" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the label test could not be planted\n' 'control: the escalation label'
		return 0
	}
	if esc_not_a_park "$ctrl/park/factory/triage.sh" 2>/dev/null; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s a classifier that classes on a label still passes, so the ok above\n' \
			'control: the park'
		printf '          is not guarding the two classes it names\n'
	else
		printf '        control: a classifier planted with one label test is refused, so the ok above\n'
		printf '        is the reason this repair marks an artifact and leaves the queue alone.\n'
	fi
}
esc_controls

# ---- the step: record committed, marker read from its home, verdict read back (#315's defects 1 and 2)
#
# A leg on the DOCUMENT is the only test there is for an instruction, and #272's finding is the reason
# this repository writes them anyway: the fix shipped in the script, the guide kept prescribing the old
# shape, and the suite was green. What is pinned here is the ORDER -- the thing whose absence let a
# refused write take the record with it -- plus the read-back that decides what a tick is allowed to
# claim, and the sentence that refuses an invented name.

esc_block() { # esc_block [skill] -- the escalation step, up to the next heading
	awk '/^Then \*\*escalate and stop this unit/{f=1; next}
		f && /^#{2,} /{exit}
		f' "${1:-$HERE/../.opencode/skills/factory/SKILL.md}"
}
esc_step_ordered() { # esc_step_ordered [skill] -- entry, then marker, then the read of the marker
	local skill=${1:-$HERE/../.opencode/skills/factory/SKILL.md} block commit_at label_at read_at line
	if [ ! -f "$skill" ]; then
		printf '            %s is not there, so there is no step to read\n' "$skill" >&2
		return 1
	fi
	block=$(esc_block "$skill")
	if [ -z "$block" ]; then
		printf '            the escalation step could not be found in the skill -- the sentence\n' >&2
		printf '            `Then **escalate and stop this unit` is what the step starts with, and\n' >&2
		printf '            a tick that cannot find it will improvise one\n' >&2
		return 1
	fi
	commit_at=$(printf '%s\n' "$block" | grep -n '^git commit' | head -1 | cut -d: -f1)
	label_at=$(printf '%s\n' "$block" | grep -n -- '--add-label' | head -1 | cut -d: -f1)
	read_at=$(printf '%s\n' "$block" | grep -n -- '--json labels' | head -1 | cut -d: -f1)
	if [ -z "$commit_at" ]; then
		printf '            the step never commits the escalated entry: the record lives only in a\n' >&2
		printf '            session that may be reaped before it finishes\n' >&2
		return 1
	fi
	if [ -z "$label_at" ]; then
		printf '            the step applies no marker at all, so an escalation is invisible to\n' >&2
		printf '            whoever reads the queue looking for the four escalate-classes\n' >&2
		return 1
	fi
	if [ "$commit_at" -ge "$label_at" ]; then
		printf '            the marker is applied on line %s and the record committed on line %s.\n' \
			"$label_at" "$commit_at" >&2
		printf '            Under `set -e` a refused label kills the step where it stands, and with\n' >&2
		printf '            the write first that takes `Awaiting:` down with it (#311, #315)\n' >&2
		return 1
	fi
	if [ -z "$read_at" ]; then
		printf '            the marker is written and never read back. An exit status is not the\n' >&2
		printf '            label landing (#1180), and here the name is one GitHub is known to refuse\n' >&2
		return 1
	fi
	line=$(printf '%s\n' "$block" | sed -n "${label_at}p")
	case "$line" in
	*escalation-label.sh*) ;;
	*)
		printf '            the marker line is `%s` -- it does not read the name from its home,\n' "$line" >&2
		printf '            so whatever is typed there is a copy that can go stale\n' >&2
		return 1 ;;
	esac
	case "$block" in
	*'NEXT:'*) ;;
	*)
		printf '            a refused marker has nowhere to be reported: `NEXT:` is the line the\n' >&2
		printf '            supervisor log keeps for the next pass\n' >&2
		return 1 ;;
	esac
	case "$block" in
	*'Never invent a label name'*) ;;
	*)
		printf '            the step does not refuse an invented name. A label that exists is not a\n' >&2
		printf '            decision that was made, and #311 is still open\n' >&2
		return 1 ;;
	esac
	case "$block" in
	*'gh label create'*) ;;
	*)
		printf '            the step does not name `gh label create` as refused, which is the one\n' >&2
		printf '            move that answers #311 by writing configuration nobody approved\n' >&2
		return 1 ;;
	esac
	return 0
}
claim_leg 'the escalation step commits its record before its marker' esc_step_ordered

esc_step_control() {
	# Four plants, each in its own copy: the order the file had before this commit, and three shapes it
	# drifts back to. Every plant is checked to have landed before it is asserted.
	local ctrl=$claim_work/esc-step skill
	mkdir -p "$ctrl" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control directory could not be written\n' \
			'control: the escalation step'
		return 0
	}
	# a. the marker before the commit: the order, inverted.
	skill=$ctrl/order.md
	cp "$HERE/../.opencode/skills/factory/SKILL.md" "$skill" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the skill could not be copied\n' 'control: the escalation step'
		return 0
	}
	# The `--add-label` line lifted above the `git commit` line, inside the escalation step only --
	# elsewhere in the skill both strings appear for other reasons, and a plant that edited those would
	# be testing a different document. Nothing else moves, so the only thing this copy differs in is the
	# one thing the leg claims to read.
	if ! python3 - "$skill" <<'PY'
import re
import sys
p = sys.argv[1]
lines = open(p, encoding='utf-8').read().split('\n')
start = next((i for i, l in enumerate(lines)
              if l.startswith('Then **escalate and stop this unit')), None)
if start is None:
    sys.exit(1)
end = next((i for i in range(start + 1, len(lines)) if re.match(r'^#{2,3} ', lines[i])), len(lines))
seg = lines[start:end]
lab = [i for i, l in enumerate(seg) if l.startswith('gh issue edit') and '--add-label' in l]
com = [i for i, l in enumerate(seg) if l.startswith('git commit')]
if len(lab) != 1 or len(com) != 1 or lab[0] < com[0]:
    sys.exit(1)   # already inverted, or the anchors are not the two lines this plant is about
row = seg.pop(lab[0])
com = [i for i, l in enumerate(seg) if l.startswith('git commit')]
seg.insert(com[0], row)
lines[start:end] = seg
open(p, 'w', encoding='utf-8').write('\n'.join(lines))
PY
	then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the order could not be inverted, so nothing tests it\n' \
			'control: the order'
	else
		if esc_step_ordered "$skill" 2>/dev/null; then
			fail=$((fail + 1))
			printf '  FAIL  %-40s a step that commits the record after the marker still passes,\n' \
				'control: the order'
			printf '          so the ok above is not reading the order, only the presence\n'
		else
			printf '        control: with the commit moved below the label the leg goes red, so the ok\n'
			printf '        above is the order and not the three commands happening to be there.\n'
		fi
	fi
	# b. the read-back deleted.
	awk '!/--json labels/{print}' "$HERE/../.opencode/skills/factory/SKILL.md" >"$ctrl/noread.md"
	if ! grep -qF -- '--add-label' "$ctrl/noread.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the read-back plant deleted the write too\n' 'control: the read-back'
	elif esc_step_ordered "$ctrl/noread.md" 2>/dev/null; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s a step with no read-back still passes, so the tick is free to\n' \
			'control: the read-back'
		printf '          report a marker that was never applied\n'
	else
		printf '        control: with `--json labels` deleted the leg goes red, so the read-back is\n'
		printf '        load-bearing in the ok above rather than incidental.\n'
	fi
	# c. the prohibition deleted.
	grep -vF 'Never invent a label name' "$HERE/../.opencode/skills/factory/SKILL.md" >"$ctrl/nostop.md"
	if ! grep -qF -- '--add-label' "$ctrl/nostop.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the prohibition plant deleted the write too\n' 'control: the prohibition'
	elif esc_step_ordered "$ctrl/nostop.md" 2>/dev/null; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s a step that never refuses an invented name still passes\n' \
			'control: the prohibition'
	else
		printf '        control: with the refusal sentence deleted the leg goes red, so the ok above\n'
		printf '        needs the prohibition and not merely the example command.\n'
	fi
	# d. the read-back that reads the wrong thing: same position, different field -- the plant that
	#    says the leg asks for the LABELS read and not for any old `gh` call.
	sed 's/--json labels/--json title/' "$HERE/../.opencode/skills/factory/SKILL.md" >"$ctrl/wrongread.md"
	if grep -qF -- '--json labels' "$ctrl/wrongread.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the field swap did not land\n' 'control: the wrong read'
	elif esc_step_ordered "$ctrl/wrongread.md" 2>/dev/null; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s a step that reads the title back instead of the labels passes, so\n' \
			'control: the wrong read'
		printf '          the leg is counting `gh` lines and not reading what they read\n'
	else
		printf '        control: a read-back aimed at `title` rather than `labels` is refused, so the\n'
		printf '        ok above is the right read, not merely a read.\n'
	fi
}
esc_step_control

esc_guide_agrees() { # esc_guide_agrees [root] -- the guide carries the order and the limit
	local root=$HERE/.. g
	[ -z "${1:-}" ] || root=$1
	g=$root/docs/factory.md
	[ -f "$g" ] || return 2
	# Two clauses, each whole: the order and what it buys, and the limit on what a marker means.
	# Whole phrases, because `grep -qF 'escalation'` over a 1,000-line guide is satisfied by any
	# sentence containing the word and the leg would print ok about a different paragraph.
	grep -qF 'the order is the mechanism' "$g" &&
		grep -qF 'the unit is parked or released' "$g"
}
claim_leg '…and the guide says the order and the limit too' esc_guide_agrees

esc_guide_control() {
	local ctrl=$claim_work/esc-guide
	if ! mkdir -p "$ctrl/order/docs" "$ctrl/limit/docs"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control trees could not be written\n' 'control: the guide'
		return 0
	fi
	if ! grep -vF 'the order is the mechanism' "$HERE/../docs/factory.md" \
			>"$ctrl/order/docs/factory.md" ||
		! grep -vF 'the unit is parked or released' "$HERE/../docs/factory.md" \
			>"$ctrl/limit/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control copies could not be written\n' 'control: the guide'
		return 0
	fi
	if grep -qF 'the order is the mechanism' "$ctrl/order/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the order plant left the clause in place, so the leg above was\n' \
			'control: the guide'
		printf '          never aimed at it\n'
		return 0
	fi
	if grep -qF 'the unit is parked or released' "$ctrl/limit/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the limit plant left the clause in place\n' 'control: the guide'
		return 0
	fi
	if esc_guide_agrees "$ctrl/order" 2>/dev/null || esc_guide_agrees "$ctrl/limit" 2>/dev/null; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it accepted a guide with one of the two clauses deleted, so the ok\n' \
			'control: the guide'
		printf '          above is one phrase match and not the pair it names\n'
	else
		printf '        control: a guide with either clause deleted is refused. This is the #272 shape:\n'
		printf '        the instruction and the guide are read by different readers, and a fix that\n'
		printf '        lands in one of them is a fix that leaves the other prescribing the old thing.\n'
	fi
}
esc_guide_control

# ---- the verb: one command marks either artifact, and it is not the `pr` one (#315's defect 2)
#
# #311 owns what the marker is CALLED. Nobody owns what writes it, because only one of the two
# spellings reaches a label on this account. Measured 12 Nov on gh 2.46.0, one 67-character probe name
# aimed at pull request #314, labels read before and after: the `pr` verb died on
# `repository.pullRequest.projectCards` for exit 1 -- gh reads that retired field on its way in, so the
# call ends before it resolves the label name -- while the issue verb with the same name answered
# `'zzprobe-…' not found`, also exit 1, and the labels read back empty both times. Same status, two
# different failures, and only one of them is a name a person can still supply. So a stopped unit told
# only to "label it" reaches for the natural spelling, gets a sentence about Projects, and reports an
# escalation that left no marker -- which is #311's failure mode with a new error message attached.
#
# The census below refuses a file that PRESCRIBES the dead verb: a line that begins with it, the shape
# a command takes in a fenced block or a prompt string. It deliberately does not redden on prose that
# quotes the probe mid-sentence, because this repository's evidence is its measurements and a leg that
# ate them would be widened away inside a month -- the same objection `esc_not_a_park` records against
# reddening on a sentence of prose. Its bound is stated in H-0153: a prescription written inside a
# sentence is invisible here, which is why the arm check below, in code rather than prose, is the
# load-bearing half.
esc_pr_verb() { # esc_pr_verb [root] -- the marker's one working verb, named where it is needed
	local root=$HERE/.. f hit arm bad=0
	local -a files=(factory/common.sh factory/escalation-label.sh factory/factory.sh
		factory/triage.sh factory/selftest.sh AGENTS.md docs/factory.md
		.opencode/skills/factory/SKILL.md)
	[ -z "${1:-}" ] || root=$1
	for f in "${files[@]}"; do
		[ -f "$root/$f" ] || continue
		hit=$(grep -nE -- '^[[:space:]]*gh pr edit.*--(add|remove)-label' "$root/$f" 2>/dev/null || true)
		if [ -n "$hit" ]; then
			printf '            %s prescribes a label write with the `pr` verb -- that call ends on\n' "$f" >&2
			printf '            the retired projectCards field before gh reads the name, so nothing\n' >&2
			printf '            can follow this instruction and the failure says "Projects", not "label":\n' >&2
			printf '%s\n' "$hit" | sed 's/^/              /' >&2
			bad=1
		fi
	done
	# The empty-ask branch is the only place in the harness that orders a PULL REQUEST to be marked.
	arm=$(awk '/^\taddress_review\)/,/^\t\t;;/' "$root/factory/factory.sh")
	if [ -z "$arm" ]; then
		printf '            factory_tick has no address_review arm, so the three checks below are\n' >&2
		printf '            reading an empty string and calling it an instruction\n' >&2
		return 1
	fi
	case "$arm" in
	*'gh issue edit'*) : ;;
	*)
		printf '            the branch orders a pull request to be marked and names no command, or\n' >&2
		printf '            names one that is not the issue verb. A tick fills that gap with the\n' >&2
		printf '            natural spelling, which cannot label anything here, and the marker is\n' >&2
		printf '            refused while the unit believes it escalated\n' >&2
		bad=1 ;;
	esac
	case "$arm" in
	*'--json labels'*) : ;;
	*)
		printf '            the branch writes a marker and never reads it back, so the tick has\n' >&2
		printf '            nothing to decide its own claim against (#1180)\n' >&2
		bad=1 ;;
	esac
	case "$arm" in
	*'escalation-label.sh'*) : ;;
	*)
		printf '            the branch names no home for the name, so what reaches the label flag is\n' >&2
		printf '            whatever the tick remembers, which is the copy #315 deleted\n' >&2
		bad=1 ;;
	esac
	# Instruction and guide, both (#272's shape: a fix in one leaves the other prescribing the old thing).
	if ! grep -qF 'A pull request takes THIS verb' \
		"$root/.opencode/skills/factory/SKILL.md" 2>/dev/null; then
		printf '            the skill no longer tells a stopped unit that the issue verb is the one\n' >&2
		printf '            for a pull request, so the step has drifted back to half its story\n' >&2
		bad=1
	fi
	if ! grep -qF 'Both artifacts take `gh issue edit`' "$root/docs/factory.md" 2>/dev/null; then
		printf '            the guide no longer says which verb writes a marker, so a reader who\n' >&2
		printf '            learns the step from docs/factory.md instead of the skill gets no verb\n' >&2
		bad=1
	fi
	[ "$bad" -eq 0 ]
}
claim_leg 'the escalation marker names the verb that works' esc_pr_verb

esc_pr_verb_control() {
	# Three plants, one per surface: the prescription restored, the command lifted out of the tick's
	# own instruction, and the guide's sentence deleted. Each is checked to have landed first, because
	# a plant that did not land and a leg that is blind report the same green.
	local ctrl=$claim_work/esc-verb skill fs guide
	mkdir -p "$ctrl/verb/docs" "$ctrl/verb/factory" "$ctrl/verb/.opencode/skills/factory" \
		"$ctrl/arm/docs" "$ctrl/arm/factory" "$ctrl/arm/.opencode/skills/factory" \
		"$ctrl/guide/docs" "$ctrl/guide/factory" "$ctrl/guide/.opencode/skills/factory" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control trees could not be written\n' 'control: the verb'
		return 0
	}
	skill=$ctrl/verb/.opencode/skills/factory/SKILL.md
	fs=$ctrl/arm/factory/factory.sh
	guide=$ctrl/guide/docs/factory.md
	esc_ctrl_copy "$ctrl/verb" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the prescription copy could not be written\n' 'control: the verb'
		return 0
	}
	esc_ctrl_copy "$ctrl/arm" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the arm copy could not be written\n' 'control: the verb'
		return 0
	}
	esc_ctrl_copy "$ctrl/guide" || {
		fail=$((fail + 1))
		printf '  FAIL  %-40s the guide copy could not be written\n' 'control: the verb'
		return 0
	}
	# 1. The dead verb put back as a runnable line, exactly where the working one now stands.
	printf 'gh pr edit 7 --add-label x\n' >>"$skill"
	if ! grep -qE -- '^gh pr edit.*--add-label' "$skill"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the prescription is not in the copy, so the census above was never\n' \
			'control: the verb'
		printf '          aimed at a line that orders the dead verb\n'
		return 0
	fi
	if esc_pr_verb "$ctrl/verb" 2>/dev/null; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s a file that prescribes the dead verb still passes, so the census is\n' \
			'control: the verb'
		printf '          reading file names and not their contents\n'
	else
		printf '        control: a copy holding one `gh pr edit … label` command line is refused, so\n'
		printf '        the ok above is a census of the eight live files and not of one.\n'
	fi
	# 2. The two commands taken out of the tick's instruction: the branch is back to saying "label it".
	grep -v -- '--json labels' "$ctrl/arm/factory/factory.sh" |
		grep -v '^gh issue edit ' >"$fs" || true
	if grep -q -- '--json labels' "$fs" || grep -q '^gh issue edit ' "$fs"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the arm copy still carries the marker commands, so the arm checks\n' \
			'control: the instruction'
		printf '          above were never aimed at a branch that names none\n'
		return 0
	fi
	if esc_pr_verb "$ctrl/arm" 2>/dev/null; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s a branch that orders a marker naming no verb and no read-back still\n' \
			'control: the instruction'
		printf '          passes, so the arm checks are decoration\n'
	else
		printf '        control: with the two commands lifted out of the instruction the leg goes red,\n'
		printf '        so the ok above is the tick being handed the verb, not a word it can guess.\n'
	fi
	# 3. The guide's sentence deleted, leaving the instruction intact.
	grep -vF 'Both artifacts take' "$ctrl/guide/docs/factory.md" >"$guide" || true
	if grep -qF 'Both artifacts take' "$guide"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the guide plant left the clause in place, so the leg above was never\n' \
			'control: the guide'
		printf '          aimed at it\n'
		return 0
	fi
	if esc_pr_verb "$ctrl/guide" 2>/dev/null; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s a guide with the verb sentence deleted still passes, so the skill and\n' \
			'control: the guide'
		printf '          the guide are not being read as two documents\n'
	else
		printf '        control: deleting the guide sentence alone is refused, so the instruction and\n'
		printf '        the guide are each read, and one cannot carry the other'"'"'s silence.\n'
	fi
}
esc_pr_verb_control

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
	*'WOULD-POKE pr:269 request review from team workers'*) return 0 ;;
	esac
	printf '            the supervisor took 269 somewhere other than a request to the team. Its journal:\n' >&2
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

# ---------------------------------------------------------- who asks, in both documents
#
# `AGENTS.md` used to read "Requesting a reviewer is part of opening a pull request, not a later
# courtesy" and stop, which reads as an instruction to whoever opened the pull request -- and that
# session has no verb for the ask. `gh pr edit --add-reviewer` is dead on this account's retired
# `projectCards` field (H-0010), the raw API is denied whole by the `bash` table in
# `.opencode/opencode.jsonc`, and the one method-scoped
# carve-out that would reach the endpoint was weighed and withheld (`efa4e29`) because a whole-line
# glob cannot name one POST. The rule has only ever held because `factory_request_review` does the
# asking, a component on the far side of the boundary a tick runs inside. #310 is a tick that spent a
# review round finding that out, so both documents now carry their half and a leg reads four whole
# phrases -- one rule and one reason in each -- because `grep -qF 'reviewer'` over either file is
# satisfied by any sentence that mentions one at all.
ask_target_docs_say_it() { # [root] -> 0 when AGENTS.md and SKILL.md each carry their rule and reason
	local root=$HERE/.. agents skill
	[ -z "${1:-}" ] || root=$1
	agents=$root/AGENTS.md
	skill=$root/.opencode/skills/factory/SKILL.md
	[ -f "$agents" ] && [ -f "$skill" ] || return 2
	# AGENTS: who does the asking, and why a unit cannot do it.
	grep -qF 'poke, not whoever opened the pull request' "$agents" &&
		grep -qF 'field and writes nothing' "$agents" &&
		# SKILL: what a unit does instead, and why the verb is withheld rather than simply missing.
		grep -qF 'redirecting the ask is still not yours' "$skill" &&
		grep -qF 'was weighed and withheld' "$skill"
}
ask_target_docs_control() {
	local ctrl=$claim_work/ask-target-docs
	if ! mkdir -p "$ctrl/rules/.opencode/skills/factory" \
		"$ctrl/reasons/.opencode/skills/factory"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control trees could not be written\n' \
			'control: who asks, both documents'
		return 0
	fi
	# One clause out of each document, twice: the two rule clauses, then the two reason clauses. Same
	# shape as `voided_docs_control` one section above, which is the shape that caught #272.
	if ! grep -vF 'poke, not whoever opened the pull request' "$HERE/../AGENTS.md" \
		>"$ctrl/rules/AGENTS.md" ||
		! grep -vF 'redirecting the ask is still not yours' \
			"$HERE/../.opencode/skills/factory/SKILL.md" \
		>"$ctrl/rules/.opencode/skills/factory/SKILL.md" ||
		! grep -vF 'field and writes nothing' "$HERE/../AGENTS.md" \
		>"$ctrl/reasons/AGENTS.md" ||
		! grep -vF 'was weighed and withheld' \
			"$HERE/../.opencode/skills/factory/SKILL.md" \
		>"$ctrl/reasons/.opencode/skills/factory/SKILL.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the control copies could not be written\n' \
			'control: who asks, both documents'
		return 0
	fi
	if grep -qF 'poke, not whoever opened the pull request' "$ctrl/rules/AGENTS.md" ||
		grep -qF 'redirecting the ask is still not yours' \
			"$ctrl/rules/.opencode/skills/factory/SKILL.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the rule plant left a rule in place, so the\n' \
			'control: who asks, both documents'
		printf '          leg above was never aimed at the clauses it names.\n'
		return 0
	fi
	if grep -qF 'field and writes nothing' "$ctrl/reasons/AGENTS.md" ||
		grep -qF 'was weighed and withheld' \
			"$ctrl/reasons/.opencode/skills/factory/SKILL.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the reason plant left a reason in place, so\n' \
			'control: who asks, both documents'
		printf '          the leg above does not need the reasons it claims to need.\n'
		return 0
	fi
	if ask_target_docs_say_it "$ctrl/rules" || ask_target_docs_say_it "$ctrl/reasons"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it accepted a pair with one rule clause deleted\n' \
			'control: who asks, both documents'
		printf '          from one document and one reason clause from the other, so the ok above is a\n'
		printf '          phrase match and not the four halves it names.\n'
		return 0
	fi
	printf '  ok    %-40s each copy loses one clause from each document\n' \
		'control: who asks, both documents'
	printf '          and the leg says no; all four are read, not a word nearby.\n'
	return 0
}
claim_leg 'who asks a reviewer is in both documents' ask_target_docs_say_it
ask_target_docs_control

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

# --------------------------------------------- the branch scan (#291): content, not commits
#
# `branch_without_pr` is the one class the queue invents out of a local ref, and until #291 it was
# bought with a commit COUNT. `rev-list --count` cannot tell 400 changed lines from an empty commit,
# and Step 3 of the factory skill makes every tick take a `git commit --allow-empty` rollback target
# BEFORE the judgement call it may never reach. A tick killed by the wall-clock ceiling between the
# checkpoint and its effect commit therefore leaves a branch ahead of `main` whose whole delta is
# zero bytes, and `factory.sh:675` tells the next tick "The work is already written" about it. That
# is not a hypothetical: measured 11 Nov, `issue-249` was one such commit ahead -- `git show
# --numstat` printed no file list and `git diff --stat c98f75f f64ddcd` printed nothing -- and a model
# was dispatched at it.
#
# The fixture is a real repository rather than a planted table because the verdict is a git question.
# Every branch below is genuinely cut, committed, merged or orphaned, and the classifier answers from
# `merge-base` and `diff --name-only` exactly as it does against this machine's own refs -- which is
# how these legs would catch a merge-base mistake (asking `main..branch` instead, say) rather than
# only a missing `if`.
#
# The two polarities are the argument, exactly as #284 measured them for the sibling arm. A queue
# that stops reporting empty branches must keep reporting full ones: "skip branches with no content"
# degrades into "skip branches" by changing one character, and a fixture that can only plant an empty
# branch cannot tell those two apart. So `br-work` and `br-delete` exist to be KEPT, `br-empty` and
# `br-merge-empty` exist to be DROPPED, and `br-nobase` exists to prove that a read which failed is
# not a verdict. The three control legs below each apply one `sed` to the shipped file and demand that
# the named bug come back; a control that cannot bring it back is the check being blind, and it fails
# the run.
branch_work=$work/branches
branch_bin=$branch_work/bin
branch_fix=$branch_work/fixture
branch_repo=$branch_work/repo
branch_dir=$branch_work/harness
branch_err=$branch_work/stderr
branch_rows_file=$branch_work/rows
branch_script=''
mkdir -p "$branch_bin" "$branch_fix" "$branch_dir"

# The claim fixture's `gh`, copied rather than shared: these legs answer the same reads with their own
# payloads, and a leg that edited a payload both sections read would move the other section's verdicts.
# Copied rather than rewritten, so the refusals are the shipped ones -- no write can reach a real
# queue from here, and no unhandled read can answer `[]` and pass for an empty queue.
cp "$claim_bin/gh" "$branch_bin/gh"
cp "$claim_bin/opencode" "$branch_bin/opencode"
install_gh_fields "$branch_fix"
printf '[]' >"$branch_fix/issues.json"
printf '[]' >"$branch_fix/pr-open.json"
printf '[]' >"$branch_fix/pr-bodies.json"
printf '[]' >"$branch_fix/reviews.json"
printf '%s' '{"users":[],"teams":[]}' >"$branch_fix/reviewers.json"
printf '%s' '{"login":"someone-else"}' >"$branch_fix/user.json"
printf '%s' '{"data":{"repository":{"pullRequests":{"pageInfo":{"hasNextPage":false},
	"nodes":[]},"issues":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}' |
	jq -c . >"$branch_fix/graphql.json"
# No merged pull request in the window, so every branch below is judged on its content alone and a
# `delivered` row here can only have come from the branch half of the scan.
printf '[]' >"$branch_fix/pr-merged.json"
cp "$branch_fix/pr-merged.json" "$branch_work/pr-merged.live.json"

git init -q -b main "$branch_repo"
git -C "$branch_repo" config user.email fixture@invalid
git -C "$branch_repo" config user.name fixture
git -C "$branch_repo" config commit.gpgsign false
printf 'trunk\n' >"$branch_repo/trunk.txt"
git -C "$branch_repo" add -A
git -C "$branch_repo" commit -qm 'fixture: the trunk'
# `origin/main` as a ref, not a remote: the scan asks `rev-parse origin/main` and then reads refs, and
# nothing here is going to push. A bare `git remote add` would only invite the fixture to reach the
# network.
git -C "$branch_repo" update-ref refs/remotes/origin/main refs/heads/main

# br-work -- one file, one commit. The row these legs exist to keep.
git -C "$branch_repo" checkout -qb br-work main
printf 'work\n' >"$branch_repo/work.txt"
git -C "$branch_repo" add -A
git -C "$branch_repo" commit -qm 'fixture: one file of work'

# br-delete -- a deletion is content. A verdict counting added bytes would call this branch empty and
# lose a person's work exactly the way it loses an empty checkpoint.
git -C "$branch_repo" checkout -qb br-delete main
git -C "$branch_repo" rm -q trunk.txt
git -C "$branch_repo" commit -qm 'fixture: one file removed'

# br-empty -- the dead tick: `--allow-empty`, the shape of `f64ddcd` down to the message.
git -C "$branch_repo" checkout -qb br-empty main
git -C "$branch_repo" commit -q --allow-empty -m 'chore(checkpoint): pre-H-0000 nothing follows it'

# br-merge-empty -- that same checkpoint after the trunk moved on and was merged in, which is what
# `issue-249` had actually become by the time anybody read it: TWO commits ahead, still zero bytes. The
# content question has to be asked from the MERGE BASE, or this branch is charged for the trunk's own
# file and offered all the same -- so this branch is the leg that catches a `main..branch` diff.
git -C "$branch_repo" checkout -q main
printf 'trunk moves on\n' >>"$branch_repo/trunk.txt"
git -C "$branch_repo" commit -qam 'fixture: the trunk moves on'
git -C "$branch_repo" update-ref refs/remotes/origin/main refs/heads/main
git -C "$branch_repo" checkout -qb br-merge-empty br-empty
git -C "$branch_repo" merge -q --no-edit refs/remotes/origin/main

# br-nobase -- a history with nothing in common with the trunk, so `merge-base` has no answer and the
# content question cannot be asked at all. A failed read keeps the row and says it is unverified.
git -C "$branch_repo" update-ref refs/heads/br-nobase \
	"$(git -C "$branch_repo" commit-tree "$(git -C "$branch_repo" hash-object -t tree /dev/null)" \
		-m 'fixture: an unrelated root commit')"
git -C "$branch_repo" checkout -q main

# The shipped classifier over the fixture repository, with the fixture `gh` first on PATH. The issue
# and pull-request halves read empty lists, so every row these legs see is a branch row.
branch_rows() {
	: >"$branch_err"
	PATH="$branch_bin:$PATH" \
		FACTORY_DIR="$branch_dir" FACTORY_ROOT="$branch_repo" \
		FACTORY_REPO=example/repo FACTORY_REVIEWER=someoneelse \
		FACTORY_OPENCODE=opencode \
		FACTORY_LOG="$branch_dir/log" FACTORY_LEDGER="$branch_dir/ledger" FACTORY_RUN="$branch_dir/run" \
		FACTORY_DELIVERED_WINDOW=100 CLAIM_FIXTURE="$branch_fix" \
		"${branch_script:-$HERE/triage.sh}" 2>"$branch_err" |
		tr '\037' '\t' >"$branch_rows_file" || true
	if [ -n "${BRANCH_DEBUG:-}" ]; then
		printf 'BRANCH DEBUG triage stderr:\n' >&2
		sed 's/^/    /' "$branch_err" >&2
		printf 'BRANCH DEBUG rows:\n' >&2
		sed 's/^/    /' "$branch_rows_file" >&2
	fi
	cat "$branch_rows_file"
}

branch_row_for() { awk -F'\t' -v k="$1" '$1 == "branch" && $2 == k { print; exit }' "$branch_rows_file"; }
branch_rows_printed() { awk -F'\t' '$1 == "branch" { printf "            %s %s %s\n", $2, $3, $4 }' "$branch_rows_file" >&2; }
branch_class_is() { # branch_class_is <branch> <class>
	local row got
	row=$(branch_row_for "$1")
	[ -n "$row" ] || {
		printf '            no branch row for %s; the pass printed:\n' "$1" >&2
		branch_rows_printed
		return 1
	}
	got=$(printf '%s' "$row" | cut -f3)
	[ "$got" = "$2" ] || {
		printf '            %s answers %s, not %s: %s\n' "$1" "$got" "$2" "$(printf '%s' "$row" | cut -f4)" >&2
		return 1
	}
}
branch_no_row() { # branch_no_row <branch>
	local row
	row=$(branch_row_for "$1")
	[ -z "$row" ] || {
		printf '            %s printed a row the queue would dispatch at nothing: %s\n' "$1" "$row" >&2
		return 1
	}
}
branch_detail_has() { # branch_detail_has <branch> <needle>
	local d
	d=$(branch_row_for "$1" | cut -f4)
	case "$d" in
	*"$2"*) return 0 ;;
	*)
		printf '            %s detail is %s, which does not carry %s\n' "$1" "$d" "$2" >&2
		return 1
		;;
	esac
}
branch_err_has() { # branch_err_has <needle> -- the tally sentences, which are not rows
	if ! grep -qF -- "$1" "$branch_err"; then
		printf '            stderr never said %s; it said:\n' "$1" >&2
		sed 's/^/            /' "$branch_err" >&2
		return 1
	fi
}
# shellcheck disable=SC2317  # reached only as the predicate argument of claim_leg, which is how every
#                             # leg in this file runs; the disable is on the new helper alone because the
#                             # others are each also called directly from a control body
branch_err_lacks() { # branch_err_lacks <needle> -- a sentence that prints its own format is the bug
	if grep -qF -- "$1" "$branch_err"; then
		printf '            stderr says %s, which no sentence here is meant to print; it said:\n' "$1" >&2
		sed 's/^/            /' "$branch_err" >&2
		return 1
	fi
}
branch_rows_are() { # branch_rows_are <n> -- the whole branch half, so a stray row cannot hide
	local n
	n=$(awk -F'\t' '$1 == "branch"' "$branch_rows_file" | wc -l | tr -d '[:space:]')
	[ "$n" = "$1" ] || {
		printf '            %s branch rows, not %s:\n' "$n" "$1" >&2
		branch_rows_printed
		return 1
	}
}

branch_rows >/dev/null
claim_leg 'a branch with one file keeps its row' branch_class_is br-work branch_without_pr
claim_leg '…and the detail counts files, not commits' branch_detail_has br-work '1 file(s) across 1 commit(s)'
claim_leg 'a deletion is content' branch_class_is br-delete branch_without_pr
claim_leg '…counted as one file, not zero' branch_detail_has br-delete '1 file(s) across 1 commit(s)'
claim_leg 'a zero-byte checkpoint gets no row' branch_no_row br-empty
claim_leg '…nor the one that merged the trunk in' branch_no_row br-merge-empty
claim_leg '…and only what has content prints' branch_rows_are 3
claim_leg 'the skip is said out loud, not silently' branch_err_has 'hold no content the trunk lacks'
claim_leg '…naming both branches it dropped' branch_err_has 'br-empty br-merge-empty'
# The two fragments above survive the bug #329 measures, which is why they were green while stderr read
# `triage: offered: %s\n branch(es) …`. Each sentence is asserted whole, so a format split across a
# continuation line -- where the next line's indentation is a field separator, not a join -- reddens.
claim_leg '…as one sentence, not its own format string' branch_err_has \
	'triage: 2 branch(es) ahead of main hold no content the trunk lacks, so none is offered: br-empty br-merge-empty'
claim_leg '…and no format slot reaches an operator' branch_err_lacks '%s'
claim_leg '…nor an escaped n standing where a newline was meant' branch_err_lacks '\n'
claim_leg 'an unrelated history is still offered' branch_class_is br-nobase branch_without_pr
claim_leg '…marked as content not compared' branch_detail_has br-nobase 'content not comparable to main'
claim_leg '…and the failed read is reported' branch_err_has 'no merge base with main'
claim_leg '…whole, not re-applied from a split format' branch_err_has \
	'triage: 1 branch(es) ahead of main have no merge base with main, so their content could not be compared; they are still offered, unverified: br-nobase'

# One `sed` on the shipped file, run through the same runner. A mutation that changes no byte is
# refused: it would report the bug as reproduced without having reproduced anything.
#
# Two ways this fixture died the first time it ran, and both are staged against here rather than
# discovered again by the next person. `triage.sh:51` sources `common.sh` from its OWN directory, so
# a copy anywhere else exits before printing a row; and the runner execs the file directly, so without
# the bit nothing runs at all. Either one leaves `$branch_rows_file` empty, and an empty rows file
# makes "this branch got no row" TRUE for every branch in the fixture -- so a control asserting the
# bug came back would have said the bug came back. Hence the floor below: a mutant that prints no
# branch row at all is reported as dead, never as guilty.
branch_mutant_dir=$work/branch-mut/factory
mkdir -p "$branch_mutant_dir"
cp "$HERE/common.sh" "$branch_mutant_dir/common.sh"
branch_mutate() { # branch_mutate <sed-expr>
	local expr=$1
	sed "$expr" "$HERE/triage.sh" >"$branch_mutant_dir/triage.sh" || return 1
	if cmp -s "$HERE/triage.sh" "$branch_mutant_dir/triage.sh"; then
		printf '            the mutation changed no byte of triage.sh, so it proves nothing\n' >&2
		return 1
	fi
	if ! bash -n "$branch_mutant_dir/triage.sh"; then
		printf '            the mutation does not even parse\n' >&2
		return 1
	fi
	chmod +x "$branch_mutant_dir/triage.sh"
	branch_script=$branch_mutant_dir/triage.sh
	branch_rows >/dev/null
	branch_script=''
	if ! awk -F'\t' '$1 == "branch" { found = 1 } END { exit !found }' "$branch_rows_file"; then
		printf '            the mutant printed no branch row at all, so it DIED rather than changed\n' >&2
		printf '            its verdict -- every "no row" assertion below would read as a pass. It\n' >&2
		printf '            said:\n' >&2
		sed 's/^/            /' "$branch_err" >&2
		return 1
	fi
}
# Control 1: the content test gone. `issue-249` has to come back, or legs 5-9 are guarding a rule
# that was never there. Both empty branches are asserted: one is what the dead tick leaves, the other
# is what that same tick looks like after the trunk moved and somebody merged it in.
branch_control_empty_back() {
	branch_mutate 's/\[ "\$comparable" = 1 \] && \[ -z "\$delta" \]/[ "$comparable" = 0 ] \&\& [ -z "$delta" ]/' ||
		return 1
	branch_class_is br-empty branch_without_pr && branch_class_is br-merge-empty branch_without_pr
}
claim_leg 'control: dropping the content test re-offers it' branch_control_empty_back
# Control 2: the degradation #284 names -- one character turns "skip empty branches" into "skip
# branches". `br-work` has to go AND `br-nobase` has to stay, or this control cannot tell "the rule
# skips everything" from "the fixture is broken", which is the distinction the whole leg exists for.
branch_control_skip_all() {
	branch_mutate 's/if \[ "\$comparable" = 1 \] && \[ -z "\$delta" \]; then/if [ "$comparable" = 1 ]; then/' ||
		return 1
	branch_no_row br-work && branch_class_is br-nobase branch_without_pr
}
claim_leg 'control: skipping on content skips everything' branch_control_skip_all
# Control 3: the unjudgeable arm. A `comparable` that is never 1 has to show up in the row as
# unverified rather than as a byte count this pass did not measure -- and `br-nobase`, which is the
# branch that arm is FOR, has to keep its row.
branch_control_all_unjudged() {
	branch_mutate 's/^\t\tcomparable=1$/\t\tcomparable=0/' || return 1
	branch_detail_has br-work 'content not comparable to main' &&
		branch_detail_has br-nobase 'content not comparable to main'
}
claim_leg 'control: an unjudgeable read says so' branch_control_all_unjudged
# Control 4: the merge base, which is the half of this fix that looks like a detail. Ask the content
# question of `main`'s TIP instead and a zero-byte branch is charged for the trunk's own work: measured
# on the live `issue-249` on 11 Nov, 0 files from the merge base (`c98f75f`, PR #252's head) against 21
# files from the tip -- 187 commits of other people's work sitting between the two. `br-empty` is that
# case: the trunk moved on after it was cut and it never merged the trunk in, so a tip-diff would find
# `trunk.txt` "missing" and offer it as content.
branch_control_tip_not_base() {
	branch_mutate 's/--name-only "\$merge_base" "\$sha"/--name-only "$main_sha" "$sha"/' || return 1
	branch_class_is br-empty branch_without_pr && branch_detail_has br-empty '1 file(s)'
}
claim_leg 'control: asked of the tip, the empty branch is back' branch_control_tip_not_base
# Control 5: the bug #329 measured on `origin/main`, applied back to the shipped file as the bytes that
# shipped. A literal closed immediately before `\<newline>` with the rest of the sentence on an indented
# line is TWO words: the shell deletes the continuation and what is left between the literals is the
# indentation, a field separator. So printf gets a format that ends mid-sentence with no `\n` and its
# first argument is the literal `offered: %s\n`, which POSIX printf re-applies once per remaining
# argument -- three lines, the first of them its own format string. `bash -n` and shellcheck both pass
# it, and the fragment needles above survive it, which is exactly why this control is the leg that works.
branch_control_split_format() {
	branch_mutate "s@so none is offered: %s\\\\n' \\\\@so none is '\\\\\\n\\t\\t\\t'offered: %s\\\\n' \\\\@" ||
		return 1
	if branch_err_has 'triage: 2 branch(es) ahead of main hold no content the trunk lacks, so none is offered: br-empty br-merge-empty'; then
		printf '            the sentence came out whole, so the split never took effect and this\n' >&2
		printf '            control cannot see the bug it is named for\n' >&2
		return 1
	fi
	branch_err_has '%s'
}
claim_leg 'control: a format continued onto an indent prints itself' branch_control_split_format

# ------------------ a merged pull request's leftover ref is reported, not dropped (#291, ask 2)
#
# Ask 1 above made the row depend on content. This is the half #292 did not deliver, and the comment on
# #291 at 2026-11-12T17:59:30Z names it as what is still owed: the branch scan consults only OPEN pull
# requests (`pr_json`, :129), so a ref whose pull request had merged neither was offered as work nor was
# reported delivered -- it was dropped. A ref level with the trunk never even reaches the content
# question: `[ "$ahead" -gt 0 ] || continue` passes it by and the pass says nothing about it, ever.
# Measured on the shipped classifier at `1ad0760` over a throwaway repository (the probe whose shape
# this fixture mirrors): `br-shipped`, the leftover of a pull request that merged cleanly, printed no row
# of any class and left no trace but a count of branches holding nothing.
#
# The live blast radius, both ends run on 12 Nov at ~21:0xZ from this tree over the same refs, once with
# `bash factory/triage.sh` at `1ad0760` and once with the same command at the fix: **32 rows before, 69
# after.** The 37 that appear are all `branch delivered`; the 11 `branch_without_pr` rows are the same 11
# afterwards, so nothing that was offered stopped being offered; and the stderr tally of content-free refs
# falls from three (`issue-248 issue-309 issue-318`) to two (`issue-309 issue-318`), because the one of
# those three that a merged pull request names is now reported by number instead of counted. Thirty-seven
# is what #291's own census predicted: 34 leftover refs, 33 of them silent, counted 11 Nov.
#
# The polarity that keeps this from being a rule that retires work by its name is `br-reopened`: a name
# that merged once and was then re-cut with a file `main` lacks. It keeps its `branch_without_pr` row,
# and control 7 takes the content question out of the merged arm and demands that branch turn up
# `delivered` -- the harm, reproduced on demand. #254 merged on `issue-246` under `Part of #246` with
# #246 unfinished and three merged pull requests sit on `issue-199`, which is why a name is never a
# verdict on its own (H-0108) and why this arm is a name AND the same content question ask 1 asks.
#
# The merged payload is planted HERE, not at the top of the section, because the legs above answer a
# queue in which nothing has merged -- and that is exactly what keeps `br-empty`'s verdict about content
# rather than about a pull request (`branch_rows_are 3` counts those rows and would fail on 4). The refs
# below are cut in the same repository after those legs and their controls have run, so one tree holds
# both halves and the shipped classifier sees a queue with merged pull requests in it at last.
cat >"$branch_fix/pr-merged.json" <<'JSON'
[
 {"number":777,"baseRefName":"main","headRefName":"br-shipped","mergedAt":"2026-11-11T02:05:20Z",
  "body":"fix(factory): the thing that shipped\n\nRefs #777.\n"},
 {"number":778,"baseRefName":"main","headRefName":"br-deadtick","mergedAt":"2026-11-11T03:05:20Z",
  "body":"fix(factory): the other thing, and the checkpoint that outlived it\n\nRefs #778.\n"},
 {"number":779,"baseRefName":"main","headRefName":"br-reopened","mergedAt":"2026-11-11T04:05:20Z",
  "body":"fix(factory): merged once and re-cut afterwards\n\nRefs #779.\n"},
 {"number":780,"baseRefName":"feature/x","headRefName":"br-stacked","mergedAt":"2026-11-11T05:05:20Z",
  "body":"fix(factory): merged into a stacked branch, so none of it reached main\n\nRefs #780.\n"},
 {"number":781,"baseRefName":"main","headRefName":"br-many","mergedAt":"2026-11-10T01:02:03Z",
  "body":"fix(factory): the first pull request to run under this name\n\nRefs #781.\n"},
 {"number":782,"baseRefName":"main","headRefName":"br-many","mergedAt":"2026-11-11T06:07:08Z",
  "body":"fix(factory): the second one, and the name it consumed\n\nRefs #782.\n"}
]
JSON

git -C "$branch_repo" config user.email branch@invalid
git -C "$branch_repo" config user.name branch
# The work that merged, and main taking it: the ref is 0 ahead and 0 files ahead, the ordinary leftover
# of an ordinary merge whose branch GitHub did not delete.
git -C "$branch_repo" checkout -qb br-shipped main
printf 'shipped\n' >"$branch_repo/shipped.txt"
git -C "$branch_repo" add -A
git -C "$branch_repo" commit -qm 'branch: the work that merged'
git -C "$branch_repo" checkout -q main
git -C "$branch_repo" merge -q --no-edit br-shipped
git -C "$branch_repo" update-ref refs/remotes/origin/main refs/heads/main

# The measured shape: GitHub deleted the ref after the merge, and the `--allow-empty` checkpoint that
# Step 3 takes BEFORE a judgement call survived locally. One commit ahead, zero bytes -- which is what
# #293's ref looked like at 16:49Z on 12 Nov, offered to a second tick as "already written".
git -C "$branch_repo" checkout -qb br-deadtick main
git -C "$branch_repo" commit -q --allow-empty -m 'chore(checkpoint): pre-H-0000 nothing follows it'

git -C "$branch_repo" checkout -qb br-reopened main
printf 'a second, undelivered helping\n' >"$branch_repo/reopen.txt"
git -C "$branch_repo" add -A
git -C "$branch_repo" commit -qm 'branch: work that never merged'

# A merge that never reached the trunk (#216's stacked chain): its leftover still owes its content, so
# it is not delivered and it is still 0 ahead -- the arm's base filter, with no row to confuse it with.
git -C "$branch_repo" checkout -qb br-stacked main
git -C "$branch_repo" commit -q --allow-empty -m 'chore(checkpoint): pre-H-0001 stacked merge, nothing on main'

# Two pull requests under one name. The newest is the merge that consumed the name, and the row says how
# many ran under it: an attribution that hides its own ambiguity is the one a reader cannot check.
git -C "$branch_repo" checkout -qb br-many main
printf 'the first\n' >"$branch_repo/first.txt"
git -C "$branch_repo" add -A
git -C "$branch_repo" commit -qm 'branch: the first pull request'
git -C "$branch_repo" checkout -q main
git -C "$branch_repo" merge -q --no-edit br-many
git -C "$branch_repo" checkout -q br-many
printf 'the second\n' >"$branch_repo/second.txt"
git -C "$branch_repo" add -A
git -C "$branch_repo" commit -qm 'branch: the second pull request'
git -C "$branch_repo" checkout -q main
git -C "$branch_repo" merge -q --no-edit br-many
git -C "$branch_repo" update-ref refs/remotes/origin/main refs/heads/main
git -C "$branch_repo" checkout -q main

branch_rows_count() { awk -F'\t' '$1 == "branch"' "$branch_rows_file" | wc -l | tr -d '[:space:]'; }
branch_rows_delta_is() { # branch_rows_delta_is <n> -- this block's rows, against its own baseline
	local got want
	got=$(branch_rows_count)
	want=$((branch_rows_before + $1))
	[ "$got" = "$want" ] || {
		printf '            %s branch rows, expected %s (%s at the baseline plus %s):\n' \
			"$got" "$want" "$branch_rows_before" "$1" >&2
		branch_rows_printed
		return 1
	}
}

branch_err_lacks() { # branch_err_lacks <needle> -- one pass may say one thing about one ref
	if grep -qF -- "$1" "$branch_err"; then
		printf '            stderr still tallies %s as a branch with nothing to offer while the table\n' "$1" >&2
		printf '            reports it delivered: the same verdict twice, in two sentences that can\n' >&2
		printf '            disagree. Its stderr:\n' >&2
		sed 's/^/            /' "$branch_err" >&2
		return 1
	fi
}

branch_field_is() { # branch_field_is <branch> <column> <value> -- the columns carry the claim
	local got
	got=$(branch_row_for "$1" | cut -f"$2")
	[ "$got" = "$3" ] || {
		printf '            %s column %s reads %s, not %s. Its row:\n' "$1" "$2" "$got" "$3" >&2
		branch_rows | tr '\t' ' ' | sed 's/^/            /' >&2
		return 1
	}
}

branch_rows >/dev/null
claim_leg 'a merged ref at zero ahead is reported, not dropped' branch_class_is br-shipped delivered
claim_leg '…naming the pull request, the name and the instant' \
	branch_detail_has br-shipped 'merged PR #777 on `br-shipped` at 2026-11-11T02:05:20Z'
claim_leg '…and how far back the merged read looked' branch_detail_has br-shipped '(window 100)'
claim_leg '…and naming no issue, because a ref is not a ticket' branch_field_is br-shipped 6 '-'
claim_leg 'the measured issue-249 shape is reported as well' branch_class_is br-deadtick delivered
claim_leg '…and said once, not tallied again as a skipped ref' branch_err_lacks br-deadtick
claim_leg '…while a checkpoint no pull request ran is only a tally' branch_no_row br-empty
claim_leg '…and named there, beside its two neighbours' branch_err_has 'br-empty br-merge-empty br-stacked'
claim_leg 'two merges on one name report the newest' branch_detail_has br-many 'merged PR #782 on `br-many` at 2026-11-11T06:07:08Z'
claim_leg '…and do not hide that two ran' branch_detail_has br-many '2 merged pull requests ran under that name'
claim_leg 'a merge that did not reach main delivers nothing' branch_no_row br-stacked
claim_leg 'a name that merged once, re-cut with a file main lacks, is still offered' branch_class_is br-reopened branch_without_pr
claim_leg 'the rows and the reports together, so no stray row hides' branch_rows_are 7

# Control 5: the merged test inverted, and a clean merge is silent again. That is the shipped behaviour
# at `1ad0760`, held permanently: the ref passes the ahead test, holds nothing, and no arm says anything
# about it. The leg asks for both halves -- no row AND the tally named it -- because a mutant that
# deleted the whole block would also pass a leg that only counted rows, and the tally is what the
# classifier printed then.
branch_control_merged_off() {
	branch_mutate 's/\[ -n "\${shipped_pr:-\}" \] && \[ "\$comparable" = 1 \]/[ -z "${shipped_pr:-}" ] \&\& [ "$comparable" = 1 ]/' ||
		return 1
	branch_no_row br-shipped && branch_no_row br-deadtick &&
		branch_err_has br-deadtick && branch_class_is br-reopened branch_without_pr
}
claim_leg 'control: dropping the merged test returns the silence' branch_control_merged_off

# Control 6: the base filter off. A pull request merged into another branch left its work off `main`, so
# the leftover still owes it; a filter that ignores which trunk a merge reached buries it as `delivered`.
# This substitution is the widest of the three -- the same predicate guards the two issue scans, and
# `issues.json` is empty for every leg in this section, so no issue row is asserted either side of it:
# what is pinned here is the branch row the filter is protecting.
branch_control_any_base() {
	branch_mutate 's/select(\.baseRefName == \$base)/select(true)/' || return 1
	branch_class_is br-stacked delivered && branch_class_is br-work branch_without_pr
}
claim_leg 'control: a filter off main reports a stacked branch as delivered' branch_control_any_base

# Control 7: the conjunct that #291 was written about, on the other side this time. With the content
# question gone from the merged arm, a name is the whole verdict -- and `br-reopened`, whose pull request
# merged once and was then re-cut with a file the trunk does not have, becomes a delivery. Its real work
# is then offered by nobody, in the same tone of voice that reports a merge. That sentence is what the
# #277 header refuses for branches, and the issue half already carries its own version of it.
branch_control_name_only() {
	branch_mutate 's/if \[ -n "\${shipped_pr:-\}" \] && \[ "\$comparable" = 1 \] && \[ -z "\$delta" \]; then/if [ -n "${shipped_pr:-}" ]; then/' ||
		return 1
	branch_class_is br-reopened delivered
}
claim_leg 'control: a name alone, and the re-cut branch is buried' branch_control_name_only

# What the supervisor DOES with one of these rows -- end to end, not by reading the arm. `delivered` is
# reached through the CLASS, not the kind: the pass reads kind into its `unit` variable
# (`factory.sh:1071`) and its arm keys on the class alone, so `factory_class_is_workable` (:843) and the
# arm itself (:1151) take a branch row with no new handler -- and #291 asks for exactly that ("no new arm
# is needed"). A text scan would prove the arm exists; these three legs prove the arm runs on a branch.
#
# The payload names `issue-902`, which is a ref at origin/main -- zero ahead, so there is nothing here
# but the attribution. Restored to `[]` for the sections below, none of which runs the supervisor again
# (the last `proc_run` before this one is `reap-held`, :4081).
printf '%s' '[{"number":790,"baseRefName":"main","headRefName":"issue-902","mergedAt":"2026-11-12T04:05:06Z","body":"fix(fixture): the pull request that consumed the branch\n\nRefs #790.\n"}]' \
	>"$proc_fix/pr-merged.json"
proc_new delivered-ref
git -C "$proc_repo" branch issue-902 origin/main
proc_run delivered-ref-1

proc_logged_delivered() {
	proc_journal_line "$proc_journal" 'DELIVERED branch/issue-902' | grep -qF -- 'reported, not worked'
}
proc_no_session() {
	grep -q 'issue-902' "$proc_ticks" && return 1
	! grep -qE 'NEXT [a-z_]+ issue-902' "$proc_journal"
}
proc_wrote_nothing() {
	# `proc_rec` is every call the fixture `gh` answered, and the shim already exits 77 on a write it was
	# not told about, so this asks the question in the open: a `delivered` row must not be able to close,
	# merge or edit anything on the way past.
	! grep -qE '^(issue|pr) (close|reopen|merge|edit|review|comment)' "$proc_rec"
}
proc_leg 'the supervisor logs the leftover as delivered' \
	'its journal names DELIVERED branch/issue-902 and says reported, not worked' proc_logged_delivered
proc_leg '…and starts no session for it' \
	'no tick ran issue-902 and no NEXT row offers it' proc_no_session
proc_leg '…and closes nothing on the strength of a ref' \
	'the fixture gh recorded no close, merge, edit, review or comment' proc_wrote_nothing
printf '[]' >"$proc_fix/pr-merged.json"

# --------------------------------------------------- the ref whose content is somebody else's open pull request (#284)
#
# The name arm in `factory/factory.sh` refuses a safety copy by how it is spelled, and #284 measured
# the hole: `backup/*` needs a literal slash, so `backup-tip` was handed a worktree and a model and
# spent a ceiling re-deriving a change that was already on an open pull request. `factory/triage.sh`
# now asks the question of the content instead of the name, and these legs are the only place either
# reading is exercised -- measured on this repository at the time of writing, NO content-bearing ref
# duplicates an open pull request's head (the closest is two files clear of all five heads), so the
# live queue cannot reddle a blind reading. Everything the legs conclude, they conclude in a fixture.
#
# Three plants, and the difference between them is the whole design:
#
#   br-anc   one file. `pr-anc` is this branch plus a commit, so br-anc's tip is an ANCESTOR of the
#            pull request's head -- the containment reading, the one a plain `git log` would show.
#   br-pick  the same file with the same bytes from its OWN commit off main. Neither tip contains the
#            other, and `git cherry` has nothing to say about it; the bytes are what match. This is
#            `backup-tip`'s shape exactly: pre-rebase originals of commits on another branch.
#   br-div   the same file name, different bytes. Nothing is duplicated and the ref keeps a plain row.
#
# br-div is the leg that matters most: a reading that concluded on file NAMES would refuse it, and the
# harm it would do is the harm this issue was written about, pointed the wrong way -- a person's real
# work reported as a copy of someone else's, offered by nobody, in a journal line that sounds certain.
branch_detail_lacks() { # branch_detail_lacks <branch> <needle>
	local d
	# The row has to exist before its absence-of-needle is a finding. Eleven legs in this block passed
	# as `ok` while the classifier had died and printed no rows at all -- a helper that reads an empty
	# detail as "does not contain" turns every negative assertion into a reward for a broken fixture.
	[ -n "$(branch_row_for "$1")" ] || {
		printf '            %s printed no row, so "its detail lacks %s" would be a verdict about nothing\n' \
			"$1" "$2" >&2
		return 1
	}
	d=$(branch_row_for "$1" | cut -f4)
	case "$d" in
	*"$2"*)
		printf '            %s detail carries %s, which it must not: %s\n' "$1" "$2" "$d" >&2
		return 1
		;;
	esac
}

# An open pull request whose head is a branch OF THIS FIXTURE REPOSITORY, so the content question has
# two real commit graphs to compare. `headRefOid` is read off the ref rather than typed, because a
# planted oid that does not match the ref would test the reader's tolerance for a stale sha and not
# the reading itself -- and `install_gh_fields` would refuse the field if it were missing entirely.
branch_plant_open() { # branch_plant_open <number> <head-ref> [draft]
	local n=$1 ref=$2 draft=${3:-false} oid
	oid=$(git -C "$branch_repo" rev-parse --verify -q "${ref}^{commit}") || {
		printf '            fixture: no commit at %s\n' "$ref" >&2
		return 1
	}
	# `-n` is load-bearing and the failure is invisible without it: `jq '<program>'` with no file
	# reads STDIN, the suite has no stdin, so it evaluates the program ZERO times, exits 0, and writes
	# an empty file. The redirect has already truncated the payload by then, so the classifier's next
	# read gets `[]` from a fixture that was supposed to hold a pull request, `triage.sh` dies on
	# "gh pr list failed", and every leg in the section reads the empty table as a verdict. Measured:
	# the first cut of this helper omitted `-n` and eleven legs went red on a working classifier.
	jq -c -n --argjson n "$n" --arg h "$ref" --arg o "$oid" --argjson d "$draft" \
		'[{number:$n, title:"fixture: the open pull request", headRefName:$h, isDraft:$d,
		   reviewDecision:"REVIEW_REQUIRED", mergeable:"MERGEABLE", mergeStateStatus:"BEHIND",
		   updatedAt:"2026-11-13T00:00:00Z", headRefOid:$o, statusCheckRollup:[]}]' \
		>"$branch_fix/pr-open.json" || return 1
	jq -e 'type == "array" and length == 1 and (.[0].headRefOid | length) >= 7' \
		"$branch_fix/pr-open.json" >/dev/null || {
		printf '            fixture: the planted payload is not one row with a head commit\n' >&2
		return 1
	}
}

cp "$branch_fix/pr-open.json" "$branch_work/pr-open.none.json"
# The merged payload is emptied for this block, and the row count below depends on it: the `delivered`
# sub-section above leaves merged pull requests planted, and a count leg that straddles two sections'
# payloads is counting whatever the previous section happened to leave. With no merged row in the
# window, every row this block sees is a `branch_without_pr` row, which is also what makes
# `branch_rows_are` a statement about this block's plants rather than about the file's history.
printf '[]' >"$branch_fix/pr-merged.json"
branch_rows >/dev/null
branch_rows_before=$(branch_rows_count)
git -C "$branch_repo" checkout -qb br-anc main
printf 'content two branches will share\n' >"$branch_repo/anc.txt"
git -C "$branch_repo" add -A
git -C "$branch_repo" commit -qm 'fixture: the content that gets copied'
git -C "$branch_repo" checkout -qb pr-anc br-anc
printf 'a second commit, touching nothing of br-anc\n' >"$branch_repo/later.txt"
git -C "$branch_repo" add -A
git -C "$branch_repo" commit -qm 'fixture: the pull request moves on'
git -C "$branch_repo" checkout -qb br-pick main
printf 'content two branches will share\n' >"$branch_repo/anc.txt"
git -C "$branch_repo" add -A
git -C "$branch_repo" commit -qm 'fixture: the same bytes, its own commit'
git -C "$branch_repo" checkout -qb br-div main
printf 'the same path, different bytes\n' >"$branch_repo/anc.txt"
git -C "$branch_repo" add -A
git -C "$branch_repo" commit -qm 'fixture: a name collision, not a duplicate'
git -C "$branch_repo" checkout -q main

branch_plant_open 901 pr-anc || claim_leg 'fixture: an open pull request on pr-anc' false

branch_rows >/dev/null
claim_leg 'a ref inside an open pull request keeps its class' branch_class_is br-anc branch_without_pr
claim_leg '…and its detail opens with the mark, naming the pull request' \
	branch_detail_has br-anc "${FACTORY_DUP_MARK}901]"
claim_leg '…and says which reading concluded it' \
	branch_detail_has br-anc 'every commit on it is already on that head'
claim_leg '…while the head of that pull request prints no orphan row' branch_no_row pr-anc
claim_leg 'a re-cut copy with the same bytes reaches the same conclusion' \
	branch_detail_has br-pick 'every file it touches holds the same bytes on that head'
claim_leg '…and it is the BYTE reading that got there, not containment' \
	branch_detail_lacks br-pick 'every commit on it is already on that head'
claim_leg 'the same path with different bytes is not a duplicate' \
	branch_detail_lacks br-div "$FACTORY_DUP_MARK"
claim_leg '…and it keeps the row a real contribution earns' branch_class_is br-div branch_without_pr
claim_leg 'a neighbour with its own file is not called a duplicate' \
	branch_detail_lacks br-work "$FACTORY_DUP_MARK"
# Not a literal count. This block runs after the `delivered` sub-section, whose plants persist as refs
# in the same repository, so an absolute number here is a statement about the previous section's
# leftovers as much as about these three branches -- and it was wrong on the first run for exactly that
# reason. The baseline is taken with the payload empty and the assertion is about the DELTA: three refs
# added, three rows appear, nothing else materialises.
claim_leg 'the three new refs add three rows and nothing else appears' branch_rows_delta_is 3

# A DRAFT head is not an input, and the reason is #285: a pull request titled "reference only, do not
# merge" was for three days the only record the queue held of #284 and #222. Refusing a ref whose
# content lives only on such a thing hides the work for as long as the tombstone stands; working it
# costs one ceiling. Same bytes, same head, `isDraft` true -- the only difference in the fixture is the
# flag, so the leg measures the flag and nothing else.
branch_plant_open 901 pr-anc true || claim_leg 'fixture: the same head, as a draft' false
branch_rows >/dev/null
claim_leg 'a draft head buys no refusal' branch_detail_lacks br-anc "$FACTORY_DUP_MARK"
claim_leg '…so the ref is still offered, workable and plain' branch_class_is br-anc branch_without_pr
claim_leg '…and the draft flag moves the mark, never the row count' branch_rows_delta_is 3

# The bound, on the side that costs something. One head is asked, the matching head is the second in
# the payload, and the row must say the scan stopped rather than print a verdict that reads complete.
cp "$branch_work/pr-open.none.json" "$branch_fix/pr-open.json"
# Two open heads, in the order the bound sees them: the NON-matching one first, so a scan that stops
# after one head has stopped before the head that would have matched. The order is the leg -- the same
# payload with the matching head first would print the mark and no limit note, and the leg would be
# measuring which row `gh` happened to answer first.
branch_plant_open 902 br-div || claim_leg 'fixture: a head that does not match' false
jq -c -n --slurpfile b "$branch_fix/pr-open.json" --argjson n 901 \
	--arg h pr-anc --arg o "$(git -C "$branch_repo" rev-parse --verify -q 'pr-anc^{commit}')" \
	'$b[0] + [{number:$n, title:"fixture: the matching head", headRefName:$h, isDraft:false,
	   reviewDecision:"REVIEW_REQUIRED", mergeable:"MERGEABLE", mergeStateStatus:"BEHIND",
	   updatedAt:"2026-11-13T00:00:00Z", headRefOid:$o, statusCheckRollup:[]}]' \
	>"$branch_work/pr-open.two.json"
jq -e 'type == "array" and length == 2' "$branch_work/pr-open.two.json" >/dev/null ||
	claim_leg 'fixture: two open heads, the non-matching one first' false
cp "$branch_work/pr-open.two.json" "$branch_fix/pr-open.json"
FACTORY_DUP_SCAN=1 branch_rows >/dev/null
claim_leg 'a scan that ran out says so in the row it left unmarked' \
	branch_detail_has br-anc '[dup-scan-limited:1 open pull request(s) past FACTORY_DUP_SCAN=1 were not asked]'
claim_leg '…and does not also claim the duplicate it never asked about' \
	branch_detail_lacks br-anc "$FACTORY_DUP_MARK"
FACTORY_DUP_SCAN=40 branch_rows >/dev/null
claim_leg '…and the same pair, asked, concludes on the second head' \
	branch_detail_has br-anc "${FACTORY_DUP_MARK}901]"

cp "$branch_work/pr-open.none.json" "$branch_fix/pr-open.json"

# Control: the content reading blinded at the one line both readings write through. The ref must keep
# its row -- a mutant that deleted the row as well would pass a leg that only looked for the mark --
# and the detail must go back to counting files with nothing said about the open pull request. Without
# this, a refactor that drops the assignment reads as "no duplicates today", which is what the live
# queue already says and therefore indistinguishable from the reading working.
#
# The scenario is planted first, on purpose. The first cut of this control ran after the payload was
# restored to `[]`, where it passed without anything to refuse -- a control that is only ever run over
# an empty queue cannot fail, which is the same defect the fixture's own header accuses a shim of.
branch_control_dup_off() {
	branch_plant_open 901 pr-anc || return 1
	branch_rows >/dev/null
	branch_detail_has br-anc "${FACTORY_DUP_MARK}901]" || {
		printf '            the scenario is not planted, so this control has nothing to blind\n' >&2
		return 1
	}
	branch_mutate 's/dup_pr=$onum/dup_pr='"''"'/g' || return 1
	branch_class_is br-anc branch_without_pr && branch_detail_lacks br-anc "$FACTORY_DUP_MARK" &&
		branch_class_is br-pick branch_without_pr &&
		branch_detail_lacks br-pick "$FACTORY_DUP_MARK"
}
claim_leg 'control: blinding the content reading hides the duplicate, not the branch' \
	branch_control_dup_off

cp "$branch_work/pr-open.none.json" "$branch_fix/pr-open.json"
branch_rows >/dev/null

# The supervisor's half, run as a process rather than read as an arm. The ticket asks for both readings
# to be pinned and for a control that reddens when the arms are blinded, and the arm is where the cost
# actually lands: #284's `backup-tip` was refused by nothing, given a worktree and a model, and spent a
# ceiling re-deriving a change that was already in review.
#
# Three branches, one payload. `br-copy` is contained in `pr-copy`, whose head the payload names as
# open pull request #903, so the classifier marks it and the supervisor must refuse it *for that
# reason*, in the journal, naming the number. `backup-hyphen` is the ticket's specimen spelled with the
# separator that missed the glob. `work-real` is the polarity: ordinary work with no pull request, which
# must still be dispatched by a pass that has just refused two branches -- a refusal arm too eager is
# the same bug pointing the other way, and it is the one that silences a person's work.
#
# The pull request itself is planted with no checks and no reviewer, which classifies it
# `needs_reviewer`: a wait. It has to be a wait, because a `merge_now` row would have the fixture try to
# merge a pull request that exists only to hold a head commit, and `checks_failing` would buy a tick for
# a PR nobody is going to fix.
printf '[]\n' >"$proc_fix/pr-merged.json"
proc_new dup-arm
git -C "$proc_repo" checkout -qb br-copy origin/main
printf 'the content that is already under review\n' >"$proc_repo/copied.txt"
git -C "$proc_repo" add -A
git -C "$proc_repo" commit -qm 'fixture: content on a ref with no pull request of its own'
git -C "$proc_repo" checkout -qb pr-copy br-copy
printf 'and a second file the pull request adds\n' >"$proc_repo/pr-only.txt"
git -C "$proc_repo" add -A
git -C "$proc_repo" commit -qm 'fixture: the pull request head'
git -C "$proc_repo" checkout -qb backup-hyphen origin/main
printf 'a safety copy named with the wrong separator\n' >"$proc_repo/safety.txt"
git -C "$proc_repo" add -A
git -C "$proc_repo" commit -qm 'fixture: a backup by every reading but the glob'
git -C "$proc_repo" checkout -qb work-real origin/main
printf 'work that owes a pull request\n' >"$proc_repo/real.txt"
git -C "$proc_repo" add -A
git -C "$proc_repo" commit -qm 'fixture: ordinary work, no name that means anything'
git -C "$proc_repo" checkout -q main
jq -c -n --arg o "$(git -C "$proc_repo" rev-parse --verify -q 'pr-copy^{commit}')" \
	'[{number:903, title:"fixture: the open pull request", headRefName:"pr-copy", isDraft:false,
	   reviewDecision:"REVIEW_REQUIRED", mergeable:"MERGEABLE", mergeStateStatus:"BEHIND",
	   updatedAt:"2026-11-13T00:00:00Z", headRefOid:$o, statusCheckRollup:[]}]' \
	>"$proc_fix/pr-open.json"
proc_run dup-arm

proc_journal_says() { # proc_journal_says <needle>
	proc_journal_line "$proc_journal" "$1" | grep -qF -- "$1" || {
		printf '            the journal never said: %s\n            it said:\n' "$1" >&2
		sed 's/^/            /' "$proc_journal" >&2
		return 1
	}
}
proc_starts_no_session() { # proc_starts_no_session <branch>
	grep -q -- "$1" "$proc_ticks" && {
		printf '            a session ran for %s; the tick log:\n' "$1" >&2
		sed 's/^/            /' "$proc_ticks" >&2
		return 1
	}
	! grep -qE "NEXT [a-z_]+ $1( |\$)" "$proc_journal"
}
proc_offers() { # proc_offers <branch> -- refused neighbours must not swallow this one
	grep -qE "NEXT [a-z_]+ $1( |\$)" "$proc_journal" || grep -q -- "$1" "$proc_ticks" || {
		printf '            %s was neither offered nor worked; the journal:\n' "$1" >&2
		sed 's/^/            /' "$proc_journal" >&2
		return 1
	}
}
claim_leg 'the pass ran at all' test -s "$proc_journal"
claim_leg 'the marked ref is refused by content, naming the pull request' \
	proc_journal_says 'REFUSED branch:br-copy its content is already on the head of open pull request #903'
claim_leg '…and no session is started for it' proc_starts_no_session br-copy
claim_leg 'the hyphenated backup is refused by name' \
	proc_journal_says 'REFUSED branch:backup-hyphen the name says it is a backup, not a contribution'
claim_leg '…and no session is started for it either' proc_starts_no_session backup-hyphen
claim_leg '…while the ordinary branch is still worked' proc_offers work-real

# Control: both arms blinded in the CLONE (a disposable copy, which is why this one can edit a file the
# shipped legs read). `[-/]` back to `/`, and the mark matched against a token no detail can begin with.
# The two refs that were just refused must then be offered -- if a leg here stays green, the refusal it
# pins is coming from somewhere other than the arm it names, which is exactly how a "refused" journal
# line once turned out to be a `MERGE-REFUSED` wearing another rule's clothes.
proc_dup_blind_arms() {
	local f=$proc_dir/factory.sh before after
	before=$(md5sum "$f" | cut -d' ' -f1)
	sed -i 's/backup\[-\/\]\*/backup\/*/' "$f"
	sed -i 's/"\$FACTORY_DUP_MARK"\*/"NEVER-MATCHES-A-DETAIL-START"/' "$f"
	after=$(md5sum "$f" | cut -d' ' -f1)
	[ "$before" != "$after" ] || {
		printf '            the mutation changed no byte of the cloned factory.sh\n' >&2
		return 1
	}
	bash -n "$f" || {
		printf '            the mutated factory.sh does not parse\n' >&2
		return 1
	}
	proc_run dup-arm-blind
	proc_offers br-copy && proc_offers backup-hyphen
}
claim_leg 'control: blind the arms and both refs are dispatched again' proc_dup_blind_arms

printf '[]\n' >"$proc_fix/pr-open.json"
printf '[]\n' >"$proc_fix/pr-merged.json"

# --------------------------------------------------------------- who the review poke asks (#241)
#
# H-0116 moves the review request from one named person to the organisation's team, and H-0117 decides
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
		printf 'gh: HTTP 422: Review requests already requested reviewers: workers\n' >&2
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
poke_no_delete() { # H-0117: the poke never un-asks somebody it did not ask
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
	poke_run "$1" hermesvicowa workers || true
	poke_post_has 'team_reviewers[]=workers'
}
poke_never_asks_the_person_too() {
	poke_run "$HERE/factory.sh" hermesvicowa workers || true
	poke_no_post_with 'reviewers[]=hermesvicowa'
}
poke_journal_names_the_team() {
	poke_run "$HERE/factory.sh" hermesvicowa workers || true
	poke_log_has 'POKED pr:241 asked team workers' &&
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
	local POKE_LIST='{"users":[],"teams":[{"name":"Workers","slug":"workers","id":19038850}]}'
	poke_run "$HERE/factory.sh" hermesvicowa workers || true
	poke_log_has 'POKE-ALREADY pr:241 team workers already holds a request' && poke_rc_is 0
}
claim_leg 'a held team request is not a failure' poke_already_holds_a_team_request

# 4b. the same read-back over the other two ways a team's name reaches a reader. `slug` is what the
#     current REST schema makes required; `login` is the spelling that field had before the slug rename,
#     and it is the one that arrives as `org/team`; `name` is the display name, which the fixture
#     capitalises as `Workers` over the slug `workers`. The 11 Nov re-measure says the live team reads
#     `workers` in both fields, but the case-fold must survive another rename. Each of the three has to answer ALREADY, because the
#     alternative reading is POKE-FAILED -- a red journal line about a request that is sitting right
#     there, retried forever.
poke_reads_a_prefixed_login() {
	local POKE_POST_FAIL=1
	local POKE_LIST='{"users":[],"teams":[{"login":"vicowa-botworks/workers"}]}'
	poke_run "$HERE/factory.sh" hermesvicowa workers || true
	poke_log_has 'POKE-ALREADY pr:241 team workers already holds a request' && poke_rc_is 0
}
claim_leg '…an older payload spells it `login`' poke_reads_a_prefixed_login

poke_reads_the_display_name() {
	local POKE_POST_FAIL=1
	local POKE_LIST='{"users":[],"teams":[{"id":19038850,"name":"Workers","privacy":"secret"}]}'
	poke_run "$HERE/factory.sh" hermesvicowa workers || true
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
	local POKE_LIST='{"users":[],"teams":[{"slug":"workers"}]}'
	poke_run "$poke_root/usersonly/factory.sh" hermesvicowa workers || true
	poke_log_has 'POKE-FAILED pr:241' && poke_rc_is 1
}
claim_leg 'control: a users-only read-back misses it' poke_users_only_readback_red

# 5. 404 -- the name resolves to nobody. Failure, and the sentence has to carry the override that fixes
#    it, because a rename leaves the old slug and the old display name looking alike to a reader who
#     has not re-taken the org listing (`cinemAItor` asked after the move to `workers`), and the person
#     reading the journal is not in the org's team settings page.
poke_failed_names_the_override() {
	local POKE_POST_FAIL=1
	local POKE_LIST='{"users":[],"teams":[]}'
	poke_run "$HERE/factory.sh" hermesvicowa cinemAItor || true
	poke_log_has 'POKE-FAILED pr:241 team cinemAItor could not be requested' &&
		poke_log_has 'FACTORY_REVIEWER_TEAM' && poke_rc_is 1
}
claim_leg 'an unknown team fails, and says what to set' poke_failed_names_the_override

# 6. H-0117: a request the poke did not make stays where it is. Adding a team does not remove an
#    individual from the list and this function does not either, on the success path or on the
#    already-holding path. The leg is about a call that must NOT happen, so its control is the same
#    function with the DELETE added -- the shape a "clean up the stale request" fix would take.
poke_never_revokes_a_request() {
	poke_run "$HERE/factory.sh" hermesvicowa workers || true
	poke_no_delete || return 1
	local POKE_POST_FAIL=1
	local POKE_LIST='{"users":[{"login":"hermesvicowa"}],"teams":[]}'
	poke_run "$HERE/factory.sh" hermesvicowa workers || true
	poke_no_delete
}
claim_leg 'the poke leaves a request it did not make' poke_never_revokes_a_request
# Its control: the same function with the tidy-up a reviewer would write -- revoke the stale individual
# request once the team has been asked. That is the exact shape H-0117 refuses, and the leg above has
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
	poke_run "$poke_root/revokes/factory.sh" hermesvicowa workers || true
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
	poke_run "$HERE/factory.sh" hermesvicowa workers || true
	poke_log_has '-f team_reviewers[]=workers' &&
		poke_log_has 'WOULD-POKE pr:241 request review from team workers' &&
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
	preflight_target someone-else workers && return 0
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
if preflight_target someone-else workers "$poke_root/alwaysself/common.sh"; then
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
# classifier over a fixture whose only unusual fact is which list is populated. The payload goes in
# through `claim_reviewers`, the helper #272 added for the same endpoint; a second helper writing the
# same fixture file would be one more thing that can disagree about its default.
claim_reset
claim_open '[{"number":246,"title":"fix(factory): the poke asks the team","body":"Planted for the #241 legs; no issue reference.\n","claims":[]}]'
claim_reviewers '{"users":[],"teams":[{"name":"Workers","slug":"workers","id":19038850}]}'
claim_rows >/dev/null
claim_leg 'a team request is somebody being asked' claim_class_is 246 awaiting_review
claim_leg '…named as a team in the row' claim_detail_has 246 'team:workers'

claim_reset
claim_open '[{"number":246,"title":"fix(factory): the poke asks the team","body":"Planted for the #241 legs; no issue reference.\n","claims":[]}]'
claim_reviewers '{"users":[{"login":"hermesvicowa"}],"teams":[]}'
claim_rows >/dev/null
claim_leg 'an individual request still counts' claim_class_is 246 awaiting_review

# The other two spellings of the team's name, through the classifier rather than the poke. `login`
# pre-dates the slug rename and arrives as `org/team`; `name` is the display name, which the fixture
# capitalises differently from its slug. The 11 Nov reading says both fields currently read `workers`,
# but the classifier must not depend on that agreement. A row that misses either one is not cosmetic: it
# is `needs_reviewer`, and `needs_reviewer` is an instruction to poke again.
claim_reset
claim_open '[{"number":246,"title":"fix(factory): the poke asks the team","body":"Planted for the #241 legs; no issue reference.\n","claims":[]}]'
claim_reviewers '{"users":[],"teams":[{"login":"vicowa-botworks/workers"}]}'
claim_rows >/dev/null
claim_leg '…however the payload spells it' claim_class_is 246 awaiting_review
claim_leg '…with the prefix off the row' claim_detail_has 246 'team:workers'

claim_reset
claim_open '[{"number":246,"title":"fix(factory): the poke asks the team","body":"Planted for the #241 legs; no issue reference.\n","claims":[]}]'
claim_reviewers '{"users":[],"teams":[{"id":19038850,"name":"Workers"}]}'
claim_rows >/dev/null
claim_leg '…or names it only to look at' claim_class_is 246 awaiting_review

# Nobody asked at all. This is the leg that keeps the team half from being written as something that
# is always true -- a program that emitted `team:` for an empty list would satisfy both legs above and
# retire `needs_reviewer`, the class the whole supervisor exists for.
claim_reset
claim_open '[{"number":246,"title":"fix(factory): the poke asks the team","body":"Planted for the #241 legs; no issue reference.\n","claims":[]}]'
claim_reviewers '{"users":[],"teams":[]}'
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
claim_reviewers '{"users":[],"teams":[{"slug":"workers"}]}'
claim_script="$poke_root/clstriage/triage.sh" claim_rows >/dev/null
if claim_class_is 246 needs_reviewer; then
	printf '        control: the same fixture through a classifier that reads only `users[]` answers\n'
	printf '        needs_reviewer -- the row that re-pokes a request already outstanding. So the\n'
	printf '        ok above is the team being counted, and not the fixture forgetting to ask.\n'
else
	claim_script='' claim_rows >/dev/null
	fail=$((fail + 1))
	printf '  FAIL  %-40s a users-only classifier still called it awaiting_review,\n' "control: triage read"
	printf '          so the ok above is the check being blind. It answered: %s\n' "$(row_for 246)"
fi
claim_script=''
claim_reviewers '{"users":[],"teams":[]}'
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

# Does the allocator say WHICH half of the union raised the ceiling? Issue #320: the skill schedules a
# re-read immediately before the entry commit, and by then the entry is already in the working file the
# union reads -- so the answer comes back one above the caller's own heading, which is exactly what a
# stolen id looks like. The answer is a note on stderr naming the heading that sits in the file and on
# no ref, and the highest id the refs really carry. Two legs, because a note printed on every answer is
# a note read once: one fixture has the entry uncommitted with every ref below it, the other has the
# same entry committed and prints the same id, and only the note moves between them.
alloc_marker_leg() { # <label> <repo> <path> <prefix> <want id> <yes|no> <pending id> <refs ceiling id> <fact>
	local label=$1 repo=$2 path=$3 prefix=$4 want=$5 wantmark=$6 pend=$7 top=$8 fact=$9
	local got='' rc=0 mark=no miss='' id=''
	if [ -n "$prefix" ]; then
		got=$(bash "$alloc_script" --repo "$repo" --prefix "$prefix" "$path" 2>"$alloc_err") || rc=$?
	else
		got=$(bash "$alloc_script" --repo "$repo" "$path" 2>"$alloc_err") || rc=$?
	fi
	if [ "$rc" != 0 ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the allocator refused:\n' "$label"
		sed 's/^/          /' "$alloc_err"
		return 0
	fi
	if [ "$got" != "$want" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it answered %s, expected %s.\n' "$label" "$got" "$want"
		return 0
	fi
	if grep -q '^next-decision-id: PENDING ' "$alloc_err"; then mark=yes; fi
	if [ "$mark" != "$wantmark" ]; then
		fail=$((fail + 1))
		if [ "$wantmark" = yes ]; then
			printf '  FAIL  %-40s it answered %s and said nothing about which\n' "$label" "$got"
			printf '          half of the union raised the ceiling. %s\n' "$fact"
		else
			printf '  FAIL  %-40s it printed PENDING over a ceiling a ref already\n' "$label"
			printf '          carries, so the note cannot tell your own entry from one a sibling\n'
			printf '          took -- and a moved ceiling stops being the one answer that means\n'
			printf '          yield. %s\n' "$fact"
		fi
		return 0
	fi
	# The note has to carry the two numbers the caller cannot get any other way: the heading it is
	# standing on, and the ceiling the refs hold. A tag with neither leaves the session guessing which
	# of `H-0041` and `H-0042` it was handed -- which is the same silence, wearing a prefix.
	if [ "$wantmark" = yes ]; then
		for id in "$pend" "$top"; do
			grep -q -- "$id" "$alloc_err" || miss="$miss $id"
		done
		if [ -n "$miss" ]; then
			fail=$((fail + 1))
			printf '  FAIL  %-40s the note never names%s, so the marker is a\n' "$label" "$miss"
			printf '          prefix and not an answer:\n'
			sed 's/^/          /' "$alloc_err"
			return 0
		fi
	fi
	pass=$((pass + 1))
	printf '  ok    %-40s %s\n' "$label" "$fact"
	return 0
}

# One plant per direction, each aimed at the single comparison that decides the note: a plant that
# keeps the shipped verdict would show the leg certifying its fixture. The id is asserted to be
# unchanged by the plant as well, because a plant that moved the number would be a control for the
# ceiling, not for the attribution.
alloc_marker_control() { # <label> <dir> <sed expr> <repo> <path> <prefix> <what the plant broke>
	local label=$1 dir=$2 expr=$3 repo=$4 path=$5 prefix=$6 fact=$7
	local bad='' got='' rc=0 ship='' ship_mark=no plant=no
	bad=$(alloc_plant "$dir" "$expr" || true)
	if [ -z "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant could not be written or did\n' "control: $label"
		printf '          nothing change; the leg has stopped pinning the comparison it names.\n'
		return 0
	fi
	# The shipped verdict, read here rather than remembered from the leg above: the control's whole
	# claim is a difference between the two scripts on one fixture, so both halves are measured.
	if [ -n "$prefix" ]; then
		ship=$(bash "$alloc_script" --repo "$repo" --prefix "$prefix" "$path" 2>"$alloc_err") || true
	else
		ship=$(bash "$alloc_script" --repo "$repo" "$path" 2>"$alloc_err") || true
	fi
	if grep -q '^next-decision-id: PENDING ' "$alloc_err"; then ship_mark=yes; fi
	if [ -n "$prefix" ]; then
		got=$(bash "$bad" --repo "$repo" --prefix "$prefix" "$path" 2>"$alloc_err") || rc=$?
	else
		got=$(bash "$bad" --repo "$repo" "$path" 2>"$alloc_err") || rc=$?
	fi
	if [ "$rc" != 0 ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the planted allocator refused:\n' "control: $label"
		sed 's/^/          /' "$alloc_err"
		return 0
	fi
	if [ "$got" != "$ship" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the planted allocator answered %s where the shipped\n' \
			"control: $label" "$got"
		printf '          one answers %s -- the plant moved the ceiling as well as the note, so\n' "$ship"
		printf '          it controls the wrong thing.\n'
		return 0
	fi
	if grep -q '^next-decision-id: PENDING ' "$alloc_err"; then plant=yes; fi
	if [ "$plant" = "$ship_mark" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant left the note verdict at %s, exactly\n' \
			"control: $label" "$ship_mark"
		printf '          what the shipped script does, so the ok above is the fixture agreeing and\n'
		printf '          not the comparison: %s\n' "$fact"
		return 0
	fi
	# No `pass` increment: a control's ok is uncounted in this suite, the way `alloc_control` and
	# `alloc_docs_control` both leave it.
	printf '  ok    %-40s it flips the note on the same fixture and\n' "control: $label"
	printf '          the same id, so the leg above is reading the comparison: %s\n' "$fact"
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

# What a moved ceiling means is stated in three surfaces -- the skill a tick reads, this guide, and the
# allocator's own comment above the note -- and for one revision two of them stated mutually exclusive
# rules for the same signal (#321's review, against `487d572`). The newer sentence made the collision "a
# moved ceiling with nothing on stderr", and PR #307 had just made that unachievable: every answer now
# carries a `REFRESH` or `REFRESH-BLIND` line beside the id and no flag silences it. Measured on a fresh
# fixture at that head, the shipped allocator writes 460 bytes of stderr with the tree clean, 855 with
# the caller's own heading uncommitted and 460 one commit later -- never zero, so a unit following the
# newer rule can never satisfy it. The repair that would make it satisfiable again, nesting the refresh
# block inside the note's arm, is worse than the bug: `bash -n` and shellcheck both pass and a clean
# tree answers with zero bytes, so the disclosure is dead and only the suite knows it.
#
# So the discriminator is the absence of the note, and it is pinned as a clause for the reason
# `alloc_yield_rule` gives -- `grep -q PENDING` over 12,000 lines is a word, not a rule. The clause is
# spelled identically in all three surfaces because that is the property under test: a paraphrase in one
# file is exactly how the two rules came apart. The negative half is why the leg exists -- the retired
# claim must not come back in any surface, because there is no longer any answer it applies to -- and
# the third phrase is the post-commit case the same review named: a ceiling your own committed entry put
# on `HEAD` is quiet too, and a unit that reads quiet as yield renumbers its own finished entry.
alloc_collision_signal() { # [root] -> 0 when all three surfaces read the note, never the silence
	local root=$HERE/.. skill guide script
	[ -z "${1:-}" ] || root=$1
	skill=$root/.opencode/skills/factory/SKILL.md
	guide=$root/docs/factory.md
	script=$root/factory/next-decision-id.sh
	[ -f "$skill" ] && [ -f "$guide" ] && [ -f "$script" ] || return 2
	grep -qF 'no PENDING line' "$skill" &&
		grep -qF 'no PENDING line' "$guide" &&
		grep -qF 'no PENDING line' "$script" &&
		grep -qF 'durable includes yours' "$skill" &&
		grep -qF 'durable includes yours' "$guide" &&
		! grep -qF 'nothing on stderr' "$skill" &&
		! grep -qF 'nothing on stderr' "$guide" &&
		! grep -qF 'nothing on stderr' "$script"
}

alloc_signal_tree() { # <dir> -> the three surfaces copied into the layout the predicate walks
	local d=$1
	mkdir -p -- "$d/.opencode/skills/factory" "$d/docs" "$d/factory" || return 1
	cp -- "$HERE/../.opencode/skills/factory/SKILL.md" \
		"$d/.opencode/skills/factory/SKILL.md" || return 1
	cp -- "$HERE/../docs/factory.md" "$d/docs/factory.md" || return 1
	cp -- "$HERE/../factory/next-decision-id.sh" "$d/factory/next-decision-id.sh" || return 1
}

# Three plants, one per half of the predicate, each checked for having moved only the half it names --
# a plant that changes two halves at once cannot say which one the refusal above was reading, which is
# the failure the two-plant controls in `alloc_yield_control` and `alloc_docs_control` exist to avoid.
alloc_collision_signal_control() {
	local drift=$alloc_work/signal-paraphrased
	local silence=$alloc_work/signal-silence-restored
	local third=$alloc_work/signal-no-third-case

	# Plant 1: the clause paraphrased in the guide. Nothing is deleted and no retired claim returns, so a
	# refusal here can only be the wording -- the half that came apart at `487d572`.
	if ! alloc_signal_tree "$drift" ||
		! sed 's/no PENDING line/no note on stderr/' "$HERE/../docs/factory.md" \
			>"$drift/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the first control copy could not be\n' \
			'control: the signal, paraphrased'
		printf '          written.\n'
		return 0
	fi
	if grep -qF 'no PENDING line' "$drift/docs/factory.md" ||
		grep -qF 'nothing on stderr' "$drift/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant left the clause in place or brought the\n' \
			'control: the signal, paraphrased'
		printf '          retired claim back, so nothing was aimed at.\n'
		return 0
	fi
	if alloc_collision_signal "$drift"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it accepted a guide paraphrasing the clause, so\n' \
			'control: the signal, paraphrased'
		printf '          the ok above reads an idea and not the wording one file\n'
		printf '          had already drifted from.\n'
		return 0
	fi

	# Plant 2: the retired claim restored to the script's own comment, clause left where it is. Only the
	# negative half can refuse this copy.
	if ! alloc_signal_tree "$silence" ||
		! printf '# a higher answer with nothing on stderr is the collision.\n' \
			>>"$silence/factory/next-decision-id.sh"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the second control copy could not be\n' \
			'control: the signal, silence restored'
		printf '          written.\n'
		return 0
	fi
	if ! grep -qF 'nothing on stderr' "$silence/factory/next-decision-id.sh" ||
		! grep -qF 'no PENDING line' "$silence/factory/next-decision-id.sh"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant either did not write the claim or took\n' \
			'control: the signal, silence restored'
		printf '          the clause with it, so the refusal below proves nothing.\n'
		return 0
	fi
	if alloc_collision_signal "$silence"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it accepted a surface that still calls silence the\n' \
			'control: the signal, silence restored'
		printf '          collision -- the exact rule #321 was asked to retire.\n'
		return 0
	fi

	# Plant 3: the post-commit case deleted from the skill, the discriminator untouched elsewhere in the
	# same file. Proves the leg reads the third shape and not just the discriminator.
	if ! alloc_signal_tree "$third" ||
		! grep -vF 'durable includes yours' "$HERE/../.opencode/skills/factory/SKILL.md" \
			>"$third/.opencode/skills/factory/SKILL.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the third control copy could not be written\n' \
			'control: the third case, deleted'
		return 0
	fi
	if grep -qF 'durable includes yours' "$third/.opencode/skills/factory/SKILL.md" ||
		! grep -qF 'no PENDING line' "$third/.opencode/skills/factory/SKILL.md" ||
		! grep -qF 'durable includes yours' "$third/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant left the clause in, deleted the\n' \
			'control: the third case, deleted'
		printf '          discriminator with it, or emptied the guide too -- a refusal\n'
		printf '          here would not be about the third case.\n'
		return 0
	fi
	if alloc_collision_signal "$third"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it accepted a skill that never tells a unit the\n' \
			'control: the third case, deleted'
		printf '          committed ceiling may be its own.\n'
		return 0
	fi

	# No `pass` increment: a control's ok is uncounted in this suite, the way `alloc_control`,
	# `alloc_docs_control` and `alloc_yield_control` all leave it.
	printf '  ok    %-40s it refuses one surface paraphrasing the clause, so\n' \
		'control: the signal, paraphrased'
	printf '          the leg above pins the wording, not a notion sitting near it.\n'
	printf '  ok    %-40s it refuses a surface calling silence the collision,\n' \
		'control: the signal, silence restored'
	printf '          so the negative half is read as well as the positive one.\n'
	printf '  ok    %-40s it refuses a skill with the committed-ceiling clause\n' \
		'control: the third case, deleted'
	printf '          gone and the discriminator untouched, so that clause is read too.\n'
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
	#
	# And so is the verdict's procedure (#303). A marker nobody is told to act on is a log line: the
	# allocator can say REFRESH-BLIND in perfect accuracy and the tick writes the entry anyway, which is
	# why the sentence a session has to obey -- `REFRESH-BLIND` means fetch and re-read -- is pinned in
	# both documents rather than left in the script's own help text.
	grep -q 'factory/next-decision-id\.sh' "$skill" &&
		grep -q 'factory/next-decision-id\.sh' "$guide" &&
		grep -q 'git fetch' "$skill" && grep -q 'git fetch' "$guide" &&
		grep -qF 'REFRESH-BLIND' "$skill" && grep -qF 'REFRESH-BLIND' "$guide" &&
		grep -qF 'means fetch and re-read' "$skill" &&
		grep -qF 'means fetch and re-read' "$guide"
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
	# The second plant, aimed at the half the leg above gained in #303: the same two documents with the
	# verdict's procedure deleted and the marker left in place. A copy that still passes would mean the
	# leg is satisfied by the word REFRESH-BLIND appearing somewhere, which is what the activation's own
	# journal line already was -- a fact nobody was told to do anything with.
	local ctrl2=$alloc_work/docs-no-verdict-procedure
	if ! mkdir -p -- "$ctrl2/.opencode/skills/factory" "$ctrl2/docs" ||
		! grep -vF 'means fetch and re-read' "$HERE/../.opencode/skills/factory/SKILL.md" \
			>"$ctrl2/.opencode/skills/factory/SKILL.md" ||
		! grep -vF 'means fetch and re-read' "$HERE/../docs/factory.md" \
			>"$ctrl2/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the second control copies could not be\n' \
			'control: the procedure, not just the marker'
		printf '          written.\n'
		return 0
	fi
	if grep -qF 'means fetch and re-read' "$ctrl2/.opencode/skills/factory/SKILL.md" \
		"$ctrl2/docs/factory.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant left the procedure in place, so the\n' \
			'control: the procedure, not just the marker'
		printf '          leg above was never aimed at the sentence it names.\n'
		return 0
	fi
	if ! grep -qF 'REFRESH-BLIND' "$ctrl2/.opencode/skills/factory/SKILL.md"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant deleted the marker as well as the\n' \
			'control: the procedure, not just the marker'
		printf '          procedure, so a refusal here proves nothing about either one.\n'
		return 0
	fi
	if alloc_docs_reference "$ctrl2"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it accepted documents that name the marker and\n' \
			'control: the procedure, not just the marker'
		printf '          tell nobody to act on it, so the ok above pins a word and not a rule.\n'
		return 0
	fi
	printf '  ok    %-40s it refuses documents with the refresh deleted, so\n' \
		'control: the refresh, not just the path'
	printf '          the ok above pins the boundary, not just the path.\n'
	printf '  ok    %-40s it refuses documents that keep the marker and drop\n' \
		'control: the procedure, not just the marker'
	printf '          the sentence telling a session to act on it.\n'
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

# ------------------------------------------------------------------ what the read says about itself
#
# Everything above asks the allocator for a number, and every one of those legs passes today on the
# version that handed `D-0058` to two units 2260s apart (#303: `06b7c29` at 11:32:24-0500, the second
# unit's `pre-D-0058` checkpoint `4537b9d` at 12:10:04-0500). That is not a hole in those legs:
# both units were handed the *right* number for the refs they could see. What was missing is the
# command saying that its union was not the world, so the legs below read stderr as well as stdout.
#
#   the marker  REFRESH-BLIND on stderr, naming the reason, and the id still on stdout at exit 0. A
#               refusal is the wrong arm for a blind read, and `selftest.sh` already pins why: a
#               session handed an allocator that will not answer goes back to numbering by eye (#253),
#               which is the defect this script exists to end.
#   the arms    coverage (can `refs/remotes` here learn a sibling at all?) and clock (when did this
#               repository last hear from origin?). One control per arm deletes that arm from the arm
#               list and the leg that pins it has to go silent -- which is also the proof that the
#               other arm is not quietly carrying the leg.
#   the limit   #303's own shape, 0 seconds after a clone: the sibling pushed after the last contact,
#               so no offline read can see it and the marker is correctly ABSENT. Pinned as an ok with
#               the id still a duplicate (`alloc_boundary` measures the same tree after a fetch:
#               D-0002 before, D-0008 after) because a guard whose boundary is not pinned gets read as
#               a guarantee by whoever finds it next.
#   the bound   the aged fixture again, quiet under a generous FACTORY_ID_MAX_AGE -- the knob pinned by
#               a difference in verdict rather than by a grep for its name.

# shellcheck disable=SC2317  # reached through the command substitutions in the legs below
alloc_run_one() { # <script> <repo> <path> <prefix> <max-age|-> -> the id; stderr in $alloc_err
	local script=$1 repo=$2 path=$3 prefix=$4 maxage=$5
	if [ "$maxage" = '-' ]; then
		if [ -n "$prefix" ]; then
			bash "$script" --repo "$repo" --prefix "$prefix" "$path" 2>"$alloc_err"
		else
			bash "$script" --repo "$repo" "$path" 2>"$alloc_err"
		fi
	else
		if [ -n "$prefix" ]; then
			env FACTORY_ID_MAX_AGE="$maxage" \
				bash "$script" --repo "$repo" --prefix "$prefix" "$path" 2>"$alloc_err"
		else
			env FACTORY_ID_MAX_AGE="$maxage" \
				bash "$script" --repo "$repo" "$path" 2>"$alloc_err"
		fi
	fi
}

# One blind shape. The marker has to be there, name the reason this leg is about, and not name the other
# arm's reason -- a leg that fires for either reason pins neither.
# shellcheck disable=SC2317  # reached from the legs below
alloc_blind_leg() { # <label> <repo> <path> <prefix> <id> <says> <not-says> <max-age|-> <fact>
	local label=$1 repo=$2 path=$3 prefix=$4 want=$5 says=$6 nosay=$7 maxage=$8 fact=$9
	local got='' rc=0
	if ! got=$(alloc_run_one "$alloc_script" "$repo" "$path" "$prefix" "$maxage"); then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the allocator refused:\n' "$label"
		sed 's/^/          /' "$alloc_err"
		return 0
	fi
	if [ "$got" != "$want" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the verdict moved the id: got %s, expected %s.\n' "$label" "$got" "$want"
		printf '          A blind read still owes the caller the best number it has, and stdout is the\n'
		printf '          only stream a caller captures. %s\n' "$fact"
		return 0
	fi
	if ! grep -q 'REFRESH-BLIND' "$alloc_err"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it answered %s and said nothing about the read:\n' "$label" "$got"
		sed 's/^/          /' "$alloc_err"
		printf '          That is #303 exactly: the number went out as though the union were the world.\n'
		return 0
	fi
	if ! grep -qF -- "$says" "$alloc_err"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it went blind without naming "%s":\n' "$label" "$says"
		sed 's/^/          /' "$alloc_err"
		printf '          A marker with no reason in it is a shrug, and a tick cannot fetch its way out\n'
		printf '          of a shrug.\n'
		return 0
	fi
	if [ -n "$nosay" ] && grep -qF -- "$nosay" "$alloc_err"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the other arm is doing this leg'"'"'s work:\n' "$label"
		sed 's/^/          /' "$alloc_err"
		printf '          %s must not be in the verdict this leg is about.\n' "$nosay"
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-40s %s\n' "$label" "$fact"
	return 0
}

# The clean shape: the id, no marker, and the provenance line that says how old the read was.
# shellcheck disable=SC2317  # reached from the legs below
alloc_clear_leg() { # <label> <repo> <path> <prefix> <id> <max-age|-> <fact>
	local label=$1 repo=$2 path=$3 prefix=$4 want=$5 maxage=$6 fact=$7
	local got='' rc=0
	if ! got=$(alloc_run_one "$alloc_script" "$repo" "$path" "$prefix" "$maxage"); then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the allocator refused:\n' "$label"
		sed 's/^/          /' "$alloc_err"
		return 0
	fi
	if [ "$got" != "$want" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s got %s, expected %s.\n' "$label" "$got" "$want"
		return 0
	fi
	if grep -q 'REFRESH-BLIND' "$alloc_err"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it called a %s a blind read:\n' "$label" "$repo"
		sed 's/^/          /' "$alloc_err"
		printf '          A verdict that fires on a provably current tree is noise, and noise gets\n'
		printf '          ignored -- which is why the ticket'"'"'s literal test (no\n'
		printf '          refs/remotes/origin/<my branch>) was measured and not used: dispatch pushes no\n'
		printf '          unit branch, so that arm is true of every allocation this harness has made.\n'
		return 0
	fi
	if ! grep -q 'REFRESH' "$alloc_err"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s it printed no provenance line at all:\n' "$label"
		sed 's/^/          /' "$alloc_err"
		printf '          Silence is what the read said when it was blind, so silence here cannot mean\n'
		printf '          it was clear.\n'
		return 0
	fi
	pass=$((pass + 1))
	printf '  ok    %-40s %s\n' "$label" "$fact"
	return 0
}

# A control for the two helpers above: the shipped script with one arm deleted from the arm list (or
# with the verdict made unconditional), run against the SAME fixture. Either way the id must not move --
# the plant narrows the verdict, not the arithmetic.
# shellcheck disable=SC2317  # reached from the controls below
alloc_verdict_control() { # <label> <dir> <sed> <repo> <path> <prefix> <id> <silent|loud> <fact>
	local label=$1 dir=$2 expr=$3 repo=$4 path=$5 prefix=$6 want=$7 expect=$8 fact=$9
	local bad got='' rc=0
	bad=$(alloc_plant "$dir" "$expr" || true)
	if [ -z "$bad" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the plant could not be written or did\n' "control: $label"
		printf '          nothing change; the check has stopped pinning the read it names.\n'
		return 0
	fi
	if [ -n "$prefix" ]; then
		got=$(bash "$bad" --repo "$repo" --prefix "$prefix" "$path" 2>"$alloc_err") || rc=$?
	else
		got=$(bash "$bad" --repo "$repo" "$path" 2>"$alloc_err") || rc=$?
	fi
	if [ "$rc" != 0 ] || [ "$got" != "$want" ]; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the planted allocator stopped answering\n' "control: $label"
		printf '          %s: got %s at status %s, so there is nothing to compare the\n' \
			"$want" "${got:-<nothing>}" "$rc"
		printf '          leg above against.\n'
		return 0
	fi
	if [ "$expect" = silent ] && grep -q 'REFRESH-BLIND' "$alloc_err"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the planted allocator still called it blind, so the\n' "control: $label"
		printf '          leg above was never decided by the arm this plant removes.\n'
		return 0
	fi
	if [ "$expect" = loud ] && ! grep -q 'REFRESH-BLIND' "$alloc_err"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s an unconditional verdict still left the clean tree\n' "control: $label"
		printf '          quiet, so the ok above pins a fixture, not a reading.\n'
		return 0
	fi
	printf '  ok    %-40s %s\n' "control: $label" "$fact"
	return 0
}

decision_ids() {
	local base current multi repo wt bad alloc_empty alloc_nolog alloc_nopfx alloc_bleed
	local alloc_notlog alloc_dirlog alloc_headonly alloc_open alloc_committed

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

	# -------------------------------------- whose ceiling is it? (issue #320, the phantom collision)
	#
	# The leg above pins that the working file raises the ceiling. This pins what the command *says*
	# about that raise, because the raise is the ordinary case at the exact moment the skill tells a
	# session to re-read: immediately before the entry commit, the entry is already in the file the
	# union reads. Measured in `/home/rodney/prj/wt/issue-320` on 12 Nov against `factory/DECISIONS.md`
	# (git 2.53.0), the shipped allocator answered `H-0145` with the log as committed and `H-0146` after
	# an uncommitted `## H-0145` was appended, with no ref holding that heading anywhere -- so the
	# re-read came back wearing the exact face of the collision it exists to catch. A session holding
	# that has two reactions and both cost the log: yield to the phantom and open a gap `docs/factory.md`
	# says must never be inexplicable, or stop trusting the allocator at the last guard before a
	# duplicate id.
	#
	# Two fixtures differing by one commit and nothing else -- including the printed id. Only the note
	# moves, which is the whole claim: the id stays the best answer available, and stderr now says whose
	# heading the ceiling walked past.
	alloc_pending_pair() { # <dir left uncommitted> <dir with the same entry committed>
		local open=$1 committed=$2 d
		for d in "$open" "$committed"; do
			git init -q "$d" 2>/dev/null || return 1
			git -C "$d" config user.email fixture@invalid || return 1
			git -C "$d" config user.name fixture || return 1
			git -C "$d" checkout -qb trunk || return 1
			mkdir -p -- "$d/factory" || return 1
			printf '## H-0040 · the trunk ceiling\n' >"$d/factory/DECISIONS.md" || return 1
			git -C "$d" add -A || return 1
			git -C "$d" commit -qm 'fixture: the ceiling before the pending entry' || return 1
			printf '\n## H-0041 · the entry being written now\n' >>"$d/factory/DECISIONS.md" || return 1
		done
		git -C "$committed" add -A || return 1
		git -C "$committed" commit -qm 'fixture: the same entry, one commit later' || return 1
	}

	alloc_open=$alloc_work/pending-uncommitted
	alloc_committed=$alloc_work/pending-committed
	if ! alloc_pending_pair "$alloc_open" "$alloc_committed"; then
		printf '  SKIP  %-40s the pending-entry pair could not be\n' 'the provenance of the ceiling'
		printf '          written; every leg below is about a tree that does not exist.\n'
	else
		alloc_marker_leg 'a pending entry names itself' "$alloc_open" factory/DECISIONS.md H \
			H-0042 yes H-0041 H-0040 \
			'the same H-0042, with PENDING naming the heading on no ref and the refs ceiling'
		alloc_marker_leg 'a committed entry is not called pending' "$alloc_committed" \
			factory/DECISIONS.md H H-0042 no - - \
			'and quiet one commit later: HEAD carries the heading, so nothing is unexplained'
		# One plant per direction, each on the fixture whose verdict it must flip. The never-fires plant
		# puts the phantom back exactly as #320 measured it; the always-fires plant calls a committed
		# heading pending, which is the note read as noise and ignored -- a marker on every answer is a
		# marker read once, and the silence that used to mean yield goes with it.
		alloc_marker_control 'a pending entry names itself' "$alloc_work/ctrl-no-pending" \
			's|\[ "$work_max" -gt "$refs_max" \]|[ "$work_max" -gt 999999 ]|' \
			"$alloc_open" factory/DECISIONS.md H \
			'take the note away and the re-read is silent again: 146 over your own 145'
		alloc_marker_control 'a committed entry is not called pending' \
			"$alloc_work/ctrl-always-pending" \
			's|\[ "$work_max" -gt "$refs_max" \]|[ "$work_max" -ge 0 ]|' \
			"$alloc_committed" factory/DECISIONS.md H \
			'print it whatever the ceiling and a moved ceiling stops being the yield signal'
	fi

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

	gen_leg 'a missing note is the collision signal' \
		'all three surfaces read the note; none calls silence the collision' \
		alloc_collision_signal
	alloc_collision_signal_control

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

	# ------------------------------------- what the command says when its union is blind (#303)
	#
	# Every leg above asks for a number, and every one of them passed on the version that handed
	# `D-0058` to two units 2260s apart on 11 Nov -- `06b7c29` (issue-296) wrote its entry at
	# 11:32:24-0500; the second unit's `pre-D-0058` checkpoint `4537b9d` (issue-297, PR #300) is dated
	# 12:10:04-0500, and its entry `1d3470b` followed at 12:14:46-0500, in a tree where
	# `git show-ref | grep 297` answers nothing. Those legs are not wrong; both units were handed the
	# right number for the refs they could see. What was missing was the command saying that its union
	# was not the world. So these legs read
	# stderr as well as stdout, one per arm, each on a fixture built to trip that arm alone, and each
	# with a control that deletes the arm from the arm list to show the leg goes silent without it.
	#
	# The fixture is the incident rather than a caricature of it: a bare origin, a clone with a unit
	# worktree (the shape `common.sh` cuts), and a second clone that writes `D-0007` and pushes it while
	# the first is asleep. `git clone` is the detail that makes it #303 and not a general ref test: a
	# clone writes no FETCH_HEAD at all (measured on git 2.53.0), and neither does `git worktree add`, so the
	# clock that keeps a fresh unit tree quiet is the shared remote reflog -- the source that a clock
	# built on FETCH_HEAD alone would have missed, calling every clean tree blind.
	blind_root=$alloc_work/blind-read
	blind_origin=$blind_root/origin.git
	if ! mkdir -p -- "$blind_root"; then
		printf '  SKIP  %-40s the blind-read fixture could not be\n' 'the blind-read legs'
		printf '          written; every leg below is about a tree that does not exist.\n'
		return 0
	fi
	git init -q --bare "$blind_origin" || return 0
	git --git-dir="$blind_origin" config user.email fixture@invalid || return 0
	git --git-dir="$blind_origin" config user.name fixture || return 0
	git clone -q "$blind_origin" "$blind_root/main" >/dev/null 2>&1 || return 0
	git -C "$blind_root/main" config user.email fixture@invalid || return 0
	git -C "$blind_root/main" config user.name fixture || return 0
	git -C "$blind_root/main" checkout -qb trunk || return 0
	printf '## D-0001 · fixture trunk\n' >"$blind_root/main/DECISIONS.md" || return 0
	git -C "$blind_root/main" add -A >/dev/null || return 0
	git -C "$blind_root/main" commit -qm 'fixture: the trunk a queue runs from' || return 0
	git -C "$blind_root/main" push -q -u origin trunk || return 0
	# The tree a tick is actually started in, so the legs run where a tick runs and not where the clone
	# was made.
	git -C "$blind_root/main" worktree add -qb unit "$blind_root/unit" trunk >/dev/null 2>&1 || return 0
	# The sibling: its own clone, an entry far above the trunk, delivered while nobody is looking.
	git clone -q "$blind_origin" "$blind_root/other" >/dev/null 2>&1 || return 0
	git -C "$blind_root/other" config user.email fixture@invalid || return 0
	git -C "$blind_root/other" config user.name fixture || return 0
	git -C "$blind_root/other" checkout -qb issue-z origin/trunk || return 0
	printf '\n## D-0007 · an entry pushed from a second clone\n' >>"$blind_root/other/DECISIONS.md" ||
		return 0
	git -C "$blind_root/other" add -A >/dev/null || return 0
	git -C "$blind_root/other" commit -qm 'fixture: a sibling pushed while the first machine slept' ||
		return 0
	git -C "$blind_root/other" push -q origin issue-z || return 0

	# The residual, pinned on the shape that produces it rather than argued in prose. This clone is
	# seconds old and origin already holds `D-0007`: the number is a duplicate and the marker is
	# correctly absent, because between the last contact and this read anything could have been pushed
	# and no offline read can tell that from "nothing new anywhere". `alloc_boundary` above measures the
	# same tree after a fetch -- `D-0002` before, `D-0008` after -- which is what the marker points a
	# tick at, and `FACTORY_ID_MAX_AGE` is how wide a window the command refuses to be silent about.
	alloc_clear_leg 'a current read says so, quietly' "$blind_root/unit" DECISIONS.md D D-0002 - \
		'no marker on a 0s read and D-0002 while origin holds D-0007: the bound is a risk window, not a promise'
	# The control for the leg above, planted the other way round: a verdict made unconditional has to
	# call this same clean tree blind. If it does not, the ok above is a fixture agreeing and not a
	# reading -- the shape H-0069 counts quiet SKIPs for.
	alloc_verdict_control 'the clean read is a reading, not silence' "$alloc_work/ctrl-blind-always" \
		's|if \[ -n "\$blind_why" \]; then|if [ -n planted ]; then|' \
		"$blind_root/unit" DECISIONS.md D D-0002 loud \
		'make the verdict unconditional and the clean tree is called blind too'

	# Coverage: the shape `factory/factory.sh` journals once per activation as REFRESH-BLIND (H-0071)
	# while the command answering inside the unit said nothing at all.
	git -C "$blind_root/main" config remote.origin.fetch \
		'+refs/heads/trunk:refs/remotes/origin/trunk' || return 0
	alloc_blind_leg 'a narrow refspec is refused as blind' "$blind_root/unit" DECISIONS.md D D-0002 \
		'fetches only' 'last refreshed' - \
		'the refspec is named and the widen command printed, and the id is still D-0002'
	alloc_verdict_control 'a narrow refspec is refused as blind' "$alloc_work/ctrl-no-coverage-arm" \
		's|for arm in coverage clock|for arm in clock|' \
		"$blind_root/unit" DECISIONS.md D D-0002 silent \
		'drop the coverage arm and the same tree reads clear, so the leg above is the arm'
	git -C "$blind_root/main" config remote.origin.fetch \
		'+refs/heads/*:refs/remotes/origin/*' || return 0

	# The clock: nothing wrong with the clone, the refs are just old. This is the shape #303 was -- a
	# sibling wrote and pushed an entry in a window nobody in this tree had looked at since.
	find "$blind_root/main/.git/logs/refs/remotes" -type f -exec touch -d '2 hours ago' {} + || return 0
	alloc_blind_leg 'a stale union is refused as blind' "$blind_root/unit" DECISIONS.md D D-0002 \
		'last refreshed' 'fetches only' - \
		'2h past the 300s bound, with the age and the exact fetch to run in the sentence'
	alloc_verdict_control 'a stale union is refused as blind' "$alloc_work/ctrl-no-clock-arm" \
		's|for arm in coverage clock|for arm in coverage|' \
		"$blind_root/unit" DECISIONS.md D D-0002 silent \
		'drop the clock arm and the same aged tree reads clear, so the leg above is the arm'
	# The bound is a knob, pinned on the fixture from the two legs above with nothing about the tree
	# changed -- a widening of the bound, and the verdict is what moves.
	alloc_clear_leg 'the bound is the knob, not the fixture' "$blind_root/unit" DECISIONS.md D D-0002 \
		86400 'the aged tree reads clear at FACTORY_ID_MAX_AGE=86400, so the bound is what fired'
	# The third coverage shape, on a checkout with no remote at all: both arms agree here (there is no
	# origin and no record of ever contacting one), so the leg asks only for the reason a person can act
	# on first. `$alloc_bleed` is the fixture the cross-repo leg above already answers at D-0004.
	alloc_blind_leg 'a clone with no origin is refused as blind' "$alloc_bleed" DECISIONS.md D D-0004 \
		'no origin remote' '' - \
		'a checkout that has never had a sibling to learn from says so instead of answering quietly'

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
# wraps the same rows as `{"users":[…]}` -- the shape `factory/factory.sh` and `factory/triage.sh` read
# there, now through the shared `$JQ_REQUESTED_HOLDS` / `$JQ_REQUESTED_LIST` in `factory/common.sh`,
# which count `teams[]` as well (#241) -- and it had been copied out of that context into this one.
# Issue #270.
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

# --- the reaping read, replayed through a stub journalctl (#246, filed by #283's review) ----------
#
# `factory_reap_lines` and `factory_reap_lastword` each run `journalctl` by bare name, so one
# directory on PATH replaces the whole backend and every transcript below is a file. That seam is the
# reason these legs exist rather than a note in a body: `H-0120` measured `grep -c factory_reap
# factory/selftest.sh` as 0 on the merged tree, on main and on the branch, for a feature whose whole
# output is a `HELD 60s to the stop deadline` alarm. #283's review filed the absence as blocking 2 --
# `factory_reap_lastword` was defined inside `factory_reap_read`'s body, whose only call site is the
# right end of a pipeline, so the definition was created in a subshell and died there: a leg calling
# it from outside was rc 127 as shipped. `H-0123` moved it to top level as this section's precondition.
#
# Fixtures and not the host journal, because the shapes have to be PLANTABLE. The escalation opening
# in the second BEFORE the terminal line is the case the exclusion in `factory_reap_lastword` exists
# for, and on this box's live journal it is 14 activations out of 15 -- but a journal cannot be
# planted, only waited for, and #246 says the suite must pass "standalone with no host journal at
# all". So the live read is a leg too, the last one, and it degrades to a skip.
#
# Two stub personalities, and the difference between them is itself a leg. `RP_DUMB=1` makes the stub
# ignore `--until` and `-n` and hand over the whole file. A fix that left the window's boundary to
# `journalctl` cannot pass the legs that way, which is precisely why `factory_reap_lastword` applies
# the comparison itself -- any stub honouring `--until` correctly would be checking its own arithmetic
# instead of the read's. `-u`, `-t`, `-o` and `--utc` are accepted and dropped: a stub that enforced
# flags would be testing the stub.

rp_dir=$work/reap
rp_bin=$rp_dir/bin
mkdir -p "$rp_bin"

rp_say() { printf '%s\n' "$*" | sed 's/^/            /' >&2; }

rp_stub() {
	cat >"$rp_bin/journalctl" <<'STUB'
#!/usr/bin/env bash
# One fixture file, the two flags that change what the read sees, and a mode that implements neither.
set -u
until='' n=0
while [ $# -gt 0 ]; do
	case $1 in
	(--until) until=${2:-}; shift 2 ;;
	(-n) n=${2:-0}; shift 2 ;;
	(*) shift ;;
	esac
done
awk -v u="$until" -v n="$n" -v dumb="${RP_DUMB:-0}" '
	!dumb && u != "" && $1 > u { next }
	{ line[++c] = $0 }
	END { for (i = (dumb || n + 0 <= 0 || c <= n) ? 1 : c - n + 1; i <= c; i++) print line[i] }
' "$RP_FIXTURE"
STUB
	# `factory_unit_state` asks `systemctl` for six properties. No leg asserts their VALUES; they are
	# pinned so the clause is byte-stable and the manager is never touched -- except in the order they
	# are ANSWERED, which is not the order asked (H-0126), and that is what a leg below depends on.
	cat >"$rp_bin/systemctl" <<'STUB'
#!/usr/bin/env bash
# Answers ONLY the properties it is asked for, in the order systemd answers them rather than the order
# asked. A stub that answered the whole set whatever it was given would let a dropped `-p` look
# present, which is the arm `reap: not-loaded is not healthy` exists to notice.
asked=''
while [ $# -gt 0 ]; do
	case $1 in
	(-p) asked="$asked ${2:-}"; shift 2 ;;
	(*) shift ;;
	esac
done
want() { case " $asked " in (*" $1 "*) return 0 ;; (*) return 1 ;; esac }
want LoadState && printf 'LoadState=%s\n' "${RP_LOAD-loaded}"
want MainPID && printf 'MainPID=0\n'
want Result && printf 'Result=success\n'
want SubState && printf 'SubState=dead\n'
want ActiveState && printf 'ActiveState=inactive\n'
want TasksCurrent && printf 'TasksCurrent=[not set]\n'
exit 0
STUB
	chmod +x "$rp_bin/journalctl" "$rp_bin/systemctl"
}
rp_stub

# One transcript, verbatim in `journalctl -o short-iso-precise --utc`: `<stamp> <host> <comm>[pid]:
# <message>`. The stamps are this box's real 11 Nov 08:44 activation -- `END rc=0` at 08:44:08.869637,
# `State 'stop-sigterm' timed out.` at 08:45:08.999972, the terminal line at 08:45:09.002985, a hold of
# 60.133 s -- with the unit renamed, because the read asks `factory_service_unit` and the legs plant
# their own answer to that question rather than inheriting this repository's name.
cat >"$rp_dir/escalation" <<'F'
2026-11-11T08:44:08.865920+00:00 h systemd[4485]: Starting plrev283b.service - planted (one pass)...
2026-11-11T08:44:08.865920+00:00 h factory.sh[1969281]: 2026-11-11T08:44:08Z QUEUED 13 unit(s) remain
2026-11-11T08:44:08.869608+00:00 h factory.sh[1969281]: 2026-11-11T08:44:08Z END rc=0 ticks_started=1
2026-11-11T08:44:08.869637+00:00 h factory.sh[1969281]: factory: 1 tick(s), rc=0.
2026-11-11T08:45:08.999972+00:00 h systemd[4485]: plrev283b.service: State 'stop-sigterm' timed out. Killing.
2026-11-11T08:45:09.000077+00:00 h systemd[4485]: plrev283b.service: Killing process 1971081 (winedevice.exe) with signal SIGKILL.
2026-11-11T08:45:09.002807+00:00 h systemd[4485]: plrev283b.service: Failed with result 'timeout'.
2026-11-11T08:45:09.002985+00:00 h systemd[4485]: Failed to start plrev283b.service - planted (one pass).
2026-11-11T08:45:09.003052+00:00 h systemd[4485]: plrev283b.service: Consumed 4min 933ms CPU time.
F
# The same transcript with the exclusion taken out, which is what the pattern #283 shipped amounted
# to: `systemd(4485): ` matches no line a journal emits, so filtering on it is the same as not
# filtering. `reap: the exclusion matches a real rendering` computes the answer both ways as its
# control for the control -- the exclusion has to EARN the 60, and the number it earns without it is
# 0: `hold` measured from systemd's own `Failed with result` line, 178 microseconds before the
# terminal one, while the queue was frozen for a minute.
#
# A clean reaping, the real 10 Nov 17:09 activation: the pass's last line at 17:09:10.131057 and
# systemd's `Finished` at .131574, so the truth is 0.517 ms and the answer is 0 and clean. The
# `Consumed` line lands after the terminal one, which is also what the real journal does.
cat >"$rp_dir/clean" <<'F'
2026-11-10T17:09:10.128067+00:00 h factory.sh[828665]: 2026-11-10T17:09:10Z QUEUED 23 unit(s) remain
2026-11-10T17:09:10.131018+00:00 h factory.sh[828665]: 2026-11-10T17:09:10Z END rc=0 ticks_started=1
2026-11-10T17:09:10.131057+00:00 h factory.sh[828665]: factory: 1 tick(s), rc=0.
2026-11-10T17:09:10.131574+00:00 h systemd[4485]: Finished plrev283b.service - planted (one pass).
2026-11-10T17:09:10.131686+00:00 h systemd[4485]: plrev283b.service: Consumed 5min 12.793s CPU time.
F
# The boundary: a line the unit wrote AFTER its own activation ended -- a descendant that outlived the
# pass, which is the whole reason a hold exists. The naive read takes it as the last word and prints
# 16 s for an 11 s hold. `stop-sigterm` is in the window so the verdict word is pinned too, and the
# hold is above `FACTORY_REAP_NOTICE_SECS` either way: the leg is about WHICH stamp was chosen, not
# about which side of the notice line it lands.
cat >"$rp_dir/after" <<'F'
2026-11-11T04:00:00.000000+00:00 h factory.sh[7]: 2026-11-11T04:00:00Z TICK-START issue/1
2026-11-11T04:00:01.000000+00:00 h factory.sh[7]: 2026-11-11T04:00:01Z END rc=0 ticks_started=1
2026-11-11T04:00:11.999972+00:00 h systemd[4485]: plrev283b.service: State 'stop-sigterm' timed out. Killing.
2026-11-11T04:00:12.000000+00:00 h systemd[4485]: Failed to start plrev283b.service - planted (one pass).
2026-11-11T04:00:17.000000+00:00 h factory.sh[9]: a line from a process that outlived the activation
F
# A pass whose last line QUOTES a journal line -- `HALT` sentences quote the manager, and
# `factory_reap_line` itself prints one. An exclusion written as a substring match anywhere in the
# line drops it (`H-0123` shipped that form), which leaves the read with no work line and the answer
# unknown, on a transcript that plainly has one. The anchored form keeps it: 5.7 s, over the notice
# line, so the shape is `6 unexplained`. The quoted text is a `Consumed` line rather than a verdict
# line on purpose -- the latter is systemd's cause sentence and would set the verdict through the
# `case` in `factory_reap_read`, which is a different claim than this fixture is making.
cat >"$rp_dir/quoted" <<'F'
2026-11-11T06:00:00.400000+00:00 h factory.sh[21]: 2026-11-11T06:00:00Z HALT quoting the manager: systemd[4485]: plrev283b.service: Consumed 4min 933ms CPU time over 11min wall clock.
2026-11-11T06:00:06.100000+00:00 h systemd[4485]: Failed to start plrev283b.service - planted (one pass).
F
# A finished activation with no line from the unit at all. systemd wrote nothing about the exit
# because the main process exited 0, and the tail holds only the manager's own lines: there is no
# start point, so the answer is unknown. Before `H-0124` this printed `reaped in 0s`.
cat >"$rp_dir/no-worker" <<'F'
2026-11-11T02:55:00.100000+00:00 h systemd[4485]: Starting plrev283b.service - planted (one pass)...
2026-11-11T02:56:06.131574+00:00 h systemd[4485]: Finished plrev283b.service - planted (one pass).
2026-11-11T02:56:06.131686+00:00 h systemd[4485]: plrev283b.service: Consumed 5min 12.793s CPU time.
F
# Nothing finished: an activation still running, so no terminal line exists.
cat >"$rp_dir/open" <<'F'
2026-11-11T05:00:00.000000+00:00 h systemd[4485]: Starting plrev283b.service - planted (one pass)...
2026-11-11T05:00:01.000000+00:00 h factory.sh[11]: 2026-11-11T05:00:01Z TICK-START issue/2
F
# Half a second either side of the notice line. The truth is 5.2 s, which is `clean` under
# `FACTORY_REAP_NOTICE_SECS=5`; arithmetic that truncates each stamp to its own second (9 - 3) reads
# 6 and files a 5.2 s teardown as unexplained.
cat >"$rp_dir/half-second" <<'F'
2026-11-11T03:00:03.900000+00:00 h factory.sh[5]: 2026-11-11T03:00:03Z END rc=0 ticks_started=1
2026-11-11T03:00:09.100000+00:00 h systemd[4485]: Finished plrev283b.service - planted (one pass).
F
# Two completed activations inside one 400-line window, which is the ordinary state of a journal that
# has been kept for an hour: the older one reaped clean, the newer one held 11 s. `reap: the newest
# completed activation is the one reported` needs both, because one activation cannot tell a backwards
# read from a forwards one.
cat >"$rp_dir/two-activations" <<'F'
2026-11-10T17:09:10.131018+00:00 h factory.sh[828665]: 2026-11-10T17:09:10Z END rc=0 ticks_started=1
2026-11-10T17:09:10.131057+00:00 h factory.sh[828665]: factory: 1 tick(s), rc=0.
2026-11-10T17:09:10.131574+00:00 h systemd[4485]: Finished plrev283b.service - planted (one pass).
2026-11-11T04:00:00.000000+00:00 h systemd[4485]: Starting plrev283b.service - planted (one pass)...
2026-11-11T04:00:01.000000+00:00 h factory.sh[7]: 2026-11-11T04:00:01Z END rc=0 ticks_started=1
2026-11-11T04:00:11.999972+00:00 h systemd[4485]: plrev283b.service: State 'stop-sigterm' timed out. Killing.
2026-11-11T04:00:12.000000+00:00 h systemd[4485]: Failed to start plrev283b.service - planted (one pass).
F

rp_ask() { # rp_ask <fixture> <dumb 0|1> -> the read's TAB line; the read's status is the status
	local f=$1 dumb=${2:-0}
	RP_FIXTURE=$f RP_DUMB=$dumb RP_UNIT=plrev283b.service PATH="$rp_bin:$PATH" \
		bash -c '. "$1/common.sh" 1>/dev/null
			factory_service_unit() { printf "%s\n" "$RP_UNIT"; }
			factory_reap_lines | factory_reap_read' _ "$HERE"
}

rp_sentence() { # rp_sentence <fixture> <dumb> -> the `reaping:` line, manager stubbed like rp_ask
	local f=$1 dumb=${2:-0}
	RP_FIXTURE=$f RP_DUMB=$dumb RP_UNIT=plrev283b.service PATH="$rp_bin:$PATH" \
		bash -c '. "$1/common.sh" 1>/dev/null
			factory_service_unit() { printf "%s\n" "$RP_UNIT"; }
			factory_reap_line' _ "$HERE"
}

rp_fields() { # rp_fields <TAB line> -> hold|how|open. The age is dropped on purpose: it counts from
	local hold='' how='' age='' open=''     # the fixture's stamp to NOW, so pinning its value would
	IFS=$'\t' read -r hold how age open <<EOF
$1
EOF
	printf '%s|%s|%s\n' "${hold-}" "${how-}" "${open-}"   # go red as the afternoon wears on, the bug
}                                                       # #246 was written about, in this suite.

reap_leg_escalation() {
	local got='' rc=0 honest_shape='' shape=''
	got=$(rp_ask "$rp_dir/escalation" 1) || rc=$?
	[ "$rc" -eq 0 ] || { rp_say "the read refused a transcript that has both ends (rc=$rc)"; return 1; }
	shape=$(rp_fields "$got")
	# 60.133 s between the pass's last line and the terminal one. A read that took systemd's own line
	# as the last word answers 0 (the newest manager line is 178 us earlier); one that truncated each
	# stamp to its second answers 61; the shipped read on this box's live journal answered 1.
	[ "$shape" = "60|deadline|0" ] || {
		rp_say "a 60.133 s hold that ended at the stop deadline read as: $shape"
		rp_say "want 60|deadline|0, the raw line was: $got"; return 1; }
	got=$(rp_ask "$rp_dir/escalation" 0) || rc=$?
	honest_shape=$(rp_fields "$got")
	[ "$honest_shape" = "$shape" ] || {
		rp_say "honouring --until and -n changes the answer: '$honest_shape' against '$shape'"
		rp_say "the number must be the read's, whichever end does the bounding"; return 1; }
	return 0
}
claim_leg 'reap: escalation opens the second before' reap_leg_escalation

reap_leg_control() {
	local got='' rc=0 shape=''
	got=$(rp_ask "$rp_dir/clean" 1) || rc=$?
	[ "$rc" -eq 0 ] || { rp_say "the control transcript was refused (rc=$rc)"; return 1; }
	shape=$(rp_fields "$got")
	[ "$shape" = "0|clean|0" ] || {
		rp_say "a 0.517 ms clean reaping read as: $shape (want 0|clean|0)"; return 1; }
	got=$(rp_ask "$rp_dir/clean" 0) || rc=$?
	[ "$(rp_fields "$got")" = "$shape" ] || {
		rp_say "the control moved with the stub's personality: $got"; return 1; }
	return 0
}
claim_leg 'reap: a clean reaping still reads clean' reap_leg_control

reap_leg_filter() {
	local shape='' naive='' last='' dumb_shape=''
	shape=$(rp_fields "$(rp_ask "$rp_dir/escalation" 1)")
	[ "$shape" = "60|deadline|0" ] || { rp_say "the bracket transcript read $shape, want 60|deadline|0"; return 1; }
	# The control for the control, in the suite's `NAIVE` shape: the same window measured by the read
	# with its exclusion taken out. `systemd\([0-9]+\)` -- what #283 shipped -- excludes no line, so
	# removing the filter and excluding nothing are the same program, and the number it produces is
	# the one an inert filter prints. If this ever equals the answer above, no leg here depends on the
	# exclusion and this leg says so instead of passing.
	last=$(awk -v until='2026-11-11T08:45:09.002985+00:00' '
		($1 "") > (until "") { next }
		{ last = $1 }
		END { if (last != "") print last }' "$rp_dir/escalation")
	[ -n "$last" ] || { rp_say 'the control found no line at all, so the fixture is wrong'; return 1; }
	# The same subtraction the read does, repeated here on purpose: this is the control, so it has to
	# be computed by the leg and not asked of the thing under test.
	local tn='' wn=''
	tn=$(factory_reap_usec '2026-11-11T08:45:09.002985+00:00') || {
		rp_say 'factory_reap_usec refused the fixture stamp'; return 1; }
	wn=$(factory_reap_usec "$last") || { rp_say "factory_reap_usec refused '$last'"; return 1; }
	naive=$(((tn - wn + 500000) / 1000000))
	[ "$naive" = 0 ] || {
		rp_say "the filter-free control reads ${naive}s, want 0s -- the fixture no longer puts a"
		rp_say "manager line inside the terminal second, so it cannot detect an inert exclusion"
		return 1; }
	[ "$naive" != "$shape" ] || {
		rp_say "filtering and not filtering both read $shape, so nothing here pins the exclusion"
		return 1; }
	# And the other direction: the exclusion must not eat a line from the unit that merely QUOTES a
	# manager line. Measured at H-0123's substring form, this transcript answered rc 2 -- unknown, no
	# work line -- because the quoted text contains ` systemd[4485]: ` anywhere in the line.
	local got='' rc=0
	got=$(rp_ask "$rp_dir/quoted" 1) || rc=$?
	[ "$rc" -eq 0 ] || {
		rp_say "a pass whose last line quotes systemd was refused (rc=$rc)"
		rp_say "that is the substring form of the exclusion dropping the unit's own line"; return 1; }
	dumb_shape=$(rp_fields "$got")
	[ "$dumb_shape" = "6|unexplained|0" ] || {
		rp_say "the quoting pass read $dumb_shape, want 6|unexplained|0"; return 1; }
	return 0
}
claim_leg 'reap: the exclusion matches a real rendering' reap_leg_filter

reap_leg_boundary() {
	local got='' shape='' other='' rc=0
	shape=$(rp_fields "$(rp_ask "$rp_dir/after" 1)")
	[ "$shape" = "11|deadline|0" ] || {
		rp_say "a line written after the terminal one moved the hold: $shape, want 11|deadline|0"
		rp_say "16|deadline|0 is the read taking a survivor's line for its predecessor's last word"
		return 1; }
	got=$(rp_ask "$rp_dir/after" 0) || rc=$?
	other=$(rp_fields "$got")
	[ "$other" = "$shape" ] || {
		rp_say "with the flags ignored the same transcript reads $other, not $shape"
		rp_say "so the window is being bounded by journalctl, not by factory_reap_lastword"; return 1; }
	return 0
}
claim_leg 'reap: nothing after the terminal line counts' reap_leg_boundary

reap_leg_no_worker() {
	local got='' rc=0 line=''
	got=$(rp_ask "$rp_dir/no-worker" 1) || rc=$?
	[ "$rc" -eq 2 ] || { rp_say "rc=$rc, want 2 (arm 2 of factory_reap_read's contract)"; return 1; }
	[ -z "$got" ] || { rp_say "arm 2 printed a verdict anyway: '$got'"; return 1; }
	line=$(rp_sentence "$rp_dir/no-worker" 1)
	case $line in
	(*'nothing in the tail says when its work stopped'*) : ;;
	(*) rp_say "the reaping line for an unread teardown is not the unknown sentence:"; rp_say "$line"
		rp_say "what it must not be is 'reaped in 0s', the healthy case's own words"; return 1 ;;
	esac
	case $line in
	(*'reaped in'* | *'HELD'*)
		rp_say "the unread teardown still printed a hold:"; rp_say "$line"; return 1 ;;
	esac
	return 0
}
claim_leg 'reap: no worker line is unknown, not zero' reap_leg_no_worker

reap_leg_open() {
	local got='' rc=0 line=''
	got=$(rp_ask "$rp_dir/open" 1) || rc=$?
	[ "$rc" = 1 ] || { rp_say "an activation with no terminal line answered rc=$rc, want 1"; return 1; }
	[ -z "$got" ] || { rp_say "it printed a verdict for a pass that has not ended: '$got'"; return 1; }
	line=$(rp_sentence "$rp_dir/open" 1)
	case $line in
	(*'has no finished activation'*) : ;;
	(*) rp_say "the reaping line for a pass that has not ended is not the no-finished sentence:"
		rp_say "$line"; rp_say "and it must not be the unread one either: those are different facts"
		return 1 ;;
	esac
	case $line in
	(*'nothing in the tail says when its work stopped'*)
		rp_say "both unknowns printed the same sentence, so the reader is sent to the wrong place:"
		rp_say "$line"; return 1 ;;
	esac
	return 0
}
claim_leg 'reap: an open pass is the other unknown' reap_leg_open

reap_leg_rounding() {
	local shape=''
	shape=$(rp_fields "$(rp_ask "$rp_dir/half-second" 1)")
	[ "$shape" = "5|clean|0" ] || {
		rp_say "a 5.2 s teardown read $shape, want 5|clean|0"
		rp_say "6|unexplained means each stamp was truncated to its own second before subtracting,"
		rp_say "which files a clean teardown as unexplained at FACTORY_REAP_NOTICE_SECS=5"
		return 1; }
	return 0
}
claim_leg 'reap: the hold is the truth, not the seconds' reap_leg_rounding

reap_leg_live() {
	# The read against the journal that is actually here, as a shape and not a value: what the last
	# pass cost this machine changes minute to minute, and a leg that pinned it would be a clock. What
	# is invariant is the contract -- four tab-separated fields, a numeric hold, a verdict word -- plus
	# one substantive pairing: a verdict of `deadline` with a hold of 0 is the exact shape `H-0124`
	# removed, so if this box's journal can produce a deadline it cannot produce a zero one.
	local got='' rc=0 shape='' hold='' how='' age='' open=''
	if ! journalctl --user --no-pager -n 1 >/dev/null 2>&1; then
		printf '        skip  the live journal: this box has no user journal to read\n'
		return 0
	fi
	got=$(factory_reap_lines | factory_reap_read) || rc=$?
	if [ "$rc" -ne 0 ]; then
		[ -z "$got" ] || { rp_say "a refused read printed a verdict anyway: '$got'"; return 1; }
		case $rc in
		(1 | 2) return 0 ;;
		esac
		rp_say "factory_reap_read exited $rc; its contract offers 0, 1 and 2 only"; return 1
	fi
	shape=$(rp_fields "$got")
	IFS=$'\t' read -r hold how age open <<EOF
$got
EOF
	case $hold in
	('' | *[!0-9]*) rp_say "the hold field is not a number: '$got'"; return 1 ;;
	esac
	case $age in
	('' | *[!0-9]*) rp_say "the age field is not a number: '$got'"; return 1 ;;
	esac
	case $how in
	(clean | deadline | client | unexplained) : ;;
	(*) rp_say "unrecognised verdict '$how': $got"; return 1 ;;
	esac
	[ "$how" != deadline ] || [ "$hold" -gt 0 ] || {
		rp_say "the live journal produced a deadline hold of 0s: $got"
		rp_say "that is the H-0124 shape: held by the deadline, measured against itself"; return 1; }
	return 0
}
claim_leg 'reap: the live journal, if there is one' reap_leg_live

reap_leg_reachable() {
	local got='' rc=0
	# #283's Blocking 2, pinned rather than assumed. The function used to be defined inside
	# `factory_reap_read`'s body, whose only call site is the right-hand end of a pipeline, so the
	# definition was created in a subshell and died with it: every call from outside -- which is what
	# each of these legs is -- answered 127. `type factory_reap_lastword` after
	# `factory_reap_lines | factory_reap_read` was the reviewer's measurement; this is the same fact
	# as a leg, with the value it returns rather than only its name.
	got=$(RP_FIXTURE=$rp_dir/escalation RP_DUMB=1 RP_UNIT=plrev283b.service PATH="$rp_bin:$PATH" \
		bash -c '. "$1/common.sh" 1>/dev/null
			factory_service_unit() { printf "%s\n" "$RP_UNIT"; }
			factory_reap_lastword "2026-11-11T08:45:09.002985+00:00"' _ "$HERE") || rc=$?
	if [ "$rc" -ne 0 ]; then
		rp_say "calling factory_reap_lastword from outside answered rc=$rc"
		[ "$rc" -ne 127 ] || rp_say "127 is the nesting again: the definition dies with its subshell"
		return 1
	fi
	[ "$got" = '2026-11-11T08:44:08.869637+00:00' ] || {
		rp_say "it named something else as the pass's last words: '$got'"
		rp_say "want the END line's own stamp, not the manager's 'Consumed' line after it"; return 1; }
	got=''
	got=$(RP_FIXTURE=$rp_dir/no-worker RP_DUMB=1 RP_UNIT=plrev283b.service PATH="$rp_bin:$PATH" \
		bash -c '. "$1/common.sh" 1>/dev/null
			factory_service_unit() { printf "%s\n" "$RP_UNIT"; }
			factory_reap_lastword "2026-11-11T02:56:06.131574+00:00"' _ "$HERE") || rc=$?
	[ "$rc" -eq 1 ] || { rp_say "on a transcript with no line from the pass it answered rc=$rc, want 1"; return 1; }
	[ -z "$got" ] || { rp_say "and it printed a stamp anyway: '$got'"; return 1; }
	return 0
}
claim_leg 'reap: the last-words read is reachable' reap_leg_reachable

reap_leg_live_keys() {
	local line=''
	# `H-0118` sold the live half as the reading that can see a hold in progress. Stripped of its keys
	# it said `inactive dead success [not set]`, in the order systemd chose rather than the order asked
	# (`ActiveState SubState TasksCurrent Result` in, `ActiveState SubState Result TasksCurrent` out),
	# so the clause had to be memorised to be read at all -- on the one line whose whole purpose is to
	# be read by somebody who has not memorised it.
	line=$(rp_sentence "$rp_dir/clean" 1)
	case $line in
	(*'live: plrev283b.service LoadState=loaded MainPID=0 Result=success SubState=dead ActiveState=inactive TasksCurrent=[not set]'*) : ;;
	(*) rp_say "the live half does not carry each key beside its value:"; rp_say "$line"
		rp_say "want the six 'key=value' pairs the stub answered, in the order it answered them"
		return 1 ;;
	esac
	# and the manager is still not being asked: nothing here may reach the bus.
	case $line in
	(*parental-lockout-factory*) rp_say "the live half named the real unit, so the stub was bypassed:"
		rp_say "$line"; return 1 ;;
	esac
	return 0
}
claim_leg 'reap: the live half names its keys' reap_leg_live_keys

reap_leg_unloaded() {
	local line=''
	# A unit the manager never loaded answered `inactive dead success [not set]` to the four properties
	# the clause asked for -- the live half of a healthy `reaping:` line about a unit that does not
	# exist (`no-such-unit.service`, measured). `LoadState` is what separates the two, and it is the
	# same distinction #228 is about: not-loaded is a fact, not the clean state.
	line=$(RP_LOAD=not-found rp_sentence "$rp_dir/open" 1)
	case $line in
	(*'LoadState=not-found'*) : ;;
	(*) rp_say "an unloaded unit's live half does not say so:"; rp_say "$line"
		rp_say "silent about LoadState is the shape that reads healthy for a unit nothing can load"
		return 1 ;;
	esac
	return 0
}
claim_leg 'reap: not-loaded is not healthy' reap_leg_unloaded

# --------------------------------------------------------------------------- the reaping in the journal
#
# `factory_reap_journal_lines` (#246) writes the previous activation's teardown into the pass's own
# journal. These legs replay the same transcripts through the same stub, and add one thing the read's
# legs did not need: a `systemctl` that reports being asked, because a pass that queried the manager
# about its own unit would be a pass deciding about the timer that runs it -- the authority `H-0118`
# had to disclose borrowing six times.

rp_journal() { # rp_journal <fixture> <dumb> -> the `REAP*` line(s); rc is the composer's
	local f=$1 dumb=${2:-0}
	RP_FIXTURE=$f RP_DUMB=$dumb RP_UNIT=plrev283b.service PATH="$rp_bin:$PATH" \
		bash -c '. "$1/common.sh" 1>/dev/null
			factory_service_unit() { printf "%s\n" "$RP_UNIT"; }
			factory_reap_journal_lines' _ "$HERE"
}

reap_leg_journal_held() {
	local out='' n=0
	out=$(rp_journal "$rp_dir/escalation" 1) || { rp_say "the composer refused a transcript it must answer"; return 1; }
	n=$(printf '%s\n' "$out" | grep -c .)
	[ "$n" = 1 ] || { rp_say "it printed $n lines for one activation, want exactly 1:"; rp_say "$out"; return 1; }
	case $out in
	('REAP-HELD hold=60s how=deadline '*'ended='*open=*) : ;;
	(*) rp_say "a 60.133 s hold at the stop deadline was not filed as a hold:"; rp_say "$out"
		rp_say "want REAP-HELD hold=60s how=deadline ... ended= ... open="; return 1 ;;
	esac
	case $out in
	(*clean*) rp_say "the held teardown's line also says clean, so grepping HELD lies:"
		rp_say "$out"; return 1 ;;
	esac
	return 0
}
claim_leg 'reap: a held teardown is written to the journal' reap_leg_journal_held

reap_leg_journal_clean() {
	local out=''
	out=$(rp_journal "$rp_dir/clean" 1) || { rp_say "the composer refused the control transcript"; return 1; }
	case $out in
	('REAP-CLEAN hold=0s how=clean'*) : ;;
	(*) rp_say "a 0.517 ms clean reaping is not filed as clean:"; rp_say "$out"; return 1 ;;
	esac
	case $out in
	(*HELD*) rp_say "and it is filed as a hold as well:"; rp_say "$out"; return 1 ;;
	esac
	return 0
}
claim_leg 'reap: a clean reaping is not filed as a hold' reap_leg_journal_clean

reap_leg_journal_unknown() {
	local one='' two=''
	one=$(rp_journal "$rp_dir/no-worker" 1) || rp_say "the composer's rc is the leg's, not its own"
	two=$(rp_journal "$rp_dir/open" 1)
	case $one in
	('REAP-UNKNOWN rc=2 '*unread*) : ;;
	(*) rp_say "an unread teardown is not the rc=2 unknown:"; rp_say "$one"; return 1 ;;
	esac
	case $two in
	('REAP-UNKNOWN rc=1 nothing in the journal window has finished'*) : ;;
	(*) rp_say "a pass that has not ended is not the rc=1 unknown:"; rp_say "$two"; return 1 ;;
	esac
	[ "$one" != "$two" ] || { rp_say "both unknowns printed one sentence, so the reader is sent to the wrong place"; return 1; }
	# The polarity this mark exists for (#228): an unread teardown must not borrow the healthy case's
	# number. `H-0124` removed exactly this from the `--status` sentence and it is cheaper to re-grow
	# here, where nobody is reading the line aloud.
	case $one$two in
	(*hold=* | *reaped* | *HELD*)
		rp_say "an unknown printed a hold anyway:"; rp_say "$one"; rp_say "$two"
		rp_say 'want no hold= field and no verdict word on either'; return 1 ;;
	esac
	return 0
}
claim_leg 'reap: both unknowns are marked, and neither prints a hold' reap_leg_journal_unknown

reap_leg_journal_latest() {
	local out='' n=0
	# Two completed activations in one window, the older clean and the newer held 11 s. The read walks
	# the stream backwards and breaks at the `Starting` that follows the terminal line, so the newer
	# activation is the one reported. A composer that took the oldest match instead -- the shape a
	# forward grep falls into -- answers 0s/clean about the older one and the 11 s hold is never said.
	out=$(rp_journal "$rp_dir/two-activations" 1) || { rp_say "the composer refused the two-activation transcript"; return 1; }
	n=$(printf '%s\n' "$out" | grep -c .)
	[ "$n" = 1 ] || { rp_say "it printed $n lines for a whole window of activations, want 1:"; rp_say "$out"; return 1; }
	case $out in
	('REAP-HELD hold=11s how=deadline'*) : ;;
	(*) rp_say "it did not report the NEWEST completed activation:"; rp_say "$out"
		rp_say "want REAP-HELD hold=11s how=deadline, from the second activation in the fixture"
		rp_say "hold=0s how=clean is the older one, i.e. the read scanning forward"; return 1 ;;
	esac
	return 0
}
claim_leg 'reap: the newest completed activation is the one reported' reap_leg_journal_latest

reap_leg_journal_no_bus() {
	local mark=$rp_bin/nomgr/systemctl out=''
	# `factory_reap_line` appends `factory_unit_state`, and correctly: `--status` is a human asking. A
	# pass writing its own journal must not. The marker stub shadows the inert one on PATH and files a
	# note the moment anything runs it.
	mkdir -p "$rp_bin/nomgr"
	printf '#!/usr/bin/env bash\n: > "%s.asked"\nprintf %s\n' "$mark" "'ActiveState=inactive\n'" >"$mark"
	chmod +x "$mark"
	out=$(RP_FIXTURE=$rp_dir/escalation RP_DUMB=1 RP_UNIT=plrev283b.service \
		PATH="$rp_bin/nomgr:$rp_bin:$PATH" \
		bash -c '. "$1/common.sh" 1>/dev/null
			factory_service_unit() { printf "%s\n" "$RP_UNIT"; }
			factory_reap_journal_lines' _ "$HERE") || rp_say "the composer failed with the marker stub first"
	[ ! -e "$mark.asked" ] || { rp_say "the journal composer asked systemctl -- it wrote $mark.asked"
		rp_say "a pass must not query the manager about the unit it is running inside (#246)"
		rm -f "$mark.asked"; return 1; }
	case $out in
	(*REAP-HELD*) : ;;
	(*) rp_say "and with the marker stub installed it did not even produce the line:"; rp_say "$out"
		rm -f "$mark.asked"; return 1 ;;
	esac
	rm -f "$mark.asked"
	return 0
}
claim_leg 'reap: the journal line never asks the manager' reap_leg_journal_no_bus

reap_leg_journal_agrees() {
	local sent='' mark='' want=''
	# The two composers read one parse, and the thing that must not drift between them is the VERDICT,
	# not the prose: a `--status` that says HELD over a journal line that says CLEAN is worse than either
	# alone, because the operator and the watcher then disagree about whether to act. So each side is
	# reduced to one word here and the words are compared -- a plant that retitles the journal mark while
	# leaving `how=deadline` in it used to walk through the version of this leg that only looked at
	# `how=`, which is how the comparison below got written.
	rp_verdict_line() { # <line> -> held | clean | unknown | none. `--status` spells the verdict `HELD`
		# or `reaped in`, the journal spells it `REAP-HELD`/`REAP-CLEAN`, and the unknown is the one word
		# both sides share -- so the three arms are tried in this order and a line that matches none is
		# `none`, which is a failure rather than a fourth state.
		case $1 in
		(*REAP-UNKNOWN* | *'unknown --'*) printf 'unknown' ;;
		(*REAP-HELD* | *'HELD '*) printf 'held' ;;
		(*REAP-CLEAN* | *'reaped in'*) printf 'clean' ;;
		(*) printf 'none' ;;
		esac
	}
	for f in escalation clean no-worker open; do
		case $f in
		escalation) want=held ;;
		clean) want=clean ;;
		*) want=unknown ;;
		esac
		sent=$(rp_sentence "$rp_dir/$f" 1)
		mark=$(rp_journal "$rp_dir/$f" 1)
		[ "$(rp_verdict_line "$sent")" = "$want" ] || {
			rp_say "--status says $(rp_verdict_line "$sent") about the $f transcript, want $want:"
			rp_say "$sent"; return 1; }
		[ "$(rp_verdict_line "$mark")" = "$want" ] || {
			rp_say "the journal says $(rp_verdict_line "$mark") about the $f transcript, want $want:"
			rp_say "$mark"; return 1; }
	done
	return 0
}
claim_leg 'reap: the journal and --status agree on the verdict' reap_leg_journal_agrees

# The call site is pinned by two process legs further up (`the pass writes a held teardown down`,
# `…and a clean one, on every pass`), which run the shipped `factory.sh` and read its journal. A text
# scan of `factory.sh` used to sit here; it is gone because it claimed no such harness existed and one
# does -- `proc_leg` has been running the supervisor as a process since #262, and a scan that asserts
# the absence of a stronger check is a lie waiting to be cited.

# --- where the journal was, replayed through a stub journalctl (#260 ask 3) -------------------------
#
# The four `proc_*` legs further up prove a real pass performs the read. What they cannot prove is which
# of the read's arms produced its answer, because a fixture box's journal holds exactly one shape at a
# time -- so the arms are replayed here from transcripts, the way #246's reaping read is. Same seam, same
# reason (`H-0120`): the feature is a mark, three refusals, a copy into a file that is going dead, and a
# write that must not go through a symlink -- and a suite that never renders them cannot tell a shipped
# arm from one nobody wired.
#
# Two stub personalities again. `LM_DUMB=1` ignores `-n` and hands over the whole file: a read that left
# the window's size to `journalctl` would pass every leg that way, so the legs that depend on the bound
# assert it against a stub that HONOURS it, and then check the refusal names `FACTORY_LOG_TAIL`.

lm_dir=$work/log-move
lm_bin=$lm_dir/bin
mkdir -p "$lm_bin"

lm_say() { printf '%s\n' "$*" | sed 's/^/            /' >&2; }

lm_stub() {
	cat >"$lm_bin/journalctl" <<'STUB'
#!/usr/bin/env bash
# One fixture file, the one flag that changes what the read sees, and a mode that implements neither.
set -u
n=0
while [ $# -gt 0 ]; do
	case $1 in
	(-n) n=${2:-0}; shift 2 ;;
	(*) shift ;;
	esac
done
awk -v n="$n" -v dumb="${LM_DUMB:-0}" '
	{ line[++c] = $0 }
	END { for (i = (dumb || n + 0 <= 0 || c <= n) ? 1 : c - n + 1; i <= c; i++) print line[i] }
' "$LM_FIXTURE"
STUB
	chmod +x "$lm_bin/journalctl"
}
lm_stub

# The window the read asks for, through the stub. The status is the parser's: 0 it found a record, 1 the
# window holds none, 2 it holds one that is damaged.
lm_ask() { # <fixture> <dumb 0|1> -> <dir>\t<stamp>\t<how>
	LM_FIXTURE=$1 LM_DUMB=${2:-0} LM_UNIT=plrev260b.service PATH="$lm_bin:$PATH" \
		bash -c '. "$1/common.sh" 1>/dev/null
			factory_service_unit() { printf "%s\n" "$LM_UNIT"; }
			factory_log_journal_tail | factory_log_prev_record' _ "$HERE"
}
lm_rc() { # <fixture> <dumb> -> the parser's status only
	LM_FIXTURE=$1 LM_DUMB=${2:-0} LM_UNIT=plrev260b.service PATH="$lm_bin:$PATH" \
		bash -c '. "$1/common.sh" 1>/dev/null
			factory_service_unit() { printf "%s\n" "$LM_UNIT"; }
			factory_log_prev_record >/dev/null <"$LM_FIXTURE"' _ "$HERE"
}
lm_fields() { # <TAB line> -> dir|stamp|how
	local d='' s='' h=''
	IFS=$'\t' read -r d s h <<EOF
$1
EOF
	printf '%s|%s|%s\n' "${d-}" "${s-}" "${h-}"
}
lm_move() { # <fixture> <tail N> <this-log-dir> -> the pass's sentences
	LM_FIXTURE=$1 LM_DUMB=0 LM_UNIT=plrev260b.service LM_TAIL=$2 LM_NEW=$3 PATH="$lm_bin:$PATH" \
		bash -c '. "$1/common.sh" 1>/dev/null
			factory_service_unit() { printf "%s\n" "$LM_UNIT"; }
			FACTORY_LOG_TAIL=$LM_TAIL
			factory_log_move_lines "$LM_NEW"' _ "$HERE"
}

# A pass running this code writes BOTH marks in the same second: `LOG-AT` at 10:10:00.100000 and its
# `PASS` line 100 ms later. A "newest line wins" parser therefore takes the PASS every time and answers
# from `root=`, and `how=recorded` never appears in a live journal -- not a loud failure, just an
# inference quietly replacing the fact. The second fixture is what a pre-#260 journal holds: PASS lines
# only, so the answer has to be derived and say so.
cat >"$lm_dir/both-marks" <<'F'
2026-11-12T10:10:00.100000+00:00 h factory.sh[2]: 2026-11-12T10:10:00Z LOG-AT /tree/a/factory/log
2026-11-12T10:10:00.200000+00:00 h factory.sh[2]: 2026-11-12T10:10:00Z PASS units=0 dry=0 budget=5400s root=/tree/a trees=/wt
F
cat >"$lm_dir/pass-only" <<'F'
2026-11-12T10:00:00.100000+00:00 h factory.sh[1]: 2026-11-12T10:00:00Z QUEUED 7 unit(s) remain
2026-11-12T10:00:01.000000+00:00 h factory.sh[1]: 2026-11-12T10:00:01Z PASS units=3 dry=0 budget=5400s root=/old/tree trees=/wt
2026-11-12T10:00:02.000000+00:00 h factory.sh[1]: factory: 1 tick(s), rc=0. Journal: /old/tree/factory/log/supervisor.log
F
# The hazard #247 measured for the tick-trailer read, in this read's shape: the ticket that filed the
# requirement QUOTES a PASS block in its own body, so a tick handed that brief writes a journal line
# CONTAINING `2026-11-09T…Z PASS … root=/wrong/tree`. A substring match reads a directory out of a quote
# and reports a move that never happened. The systemd line is the second half: journald writes the
# manager's own lines into the same window, and an unanchored filter lets anything in them answer too.
cat >"$lm_dir/quoted" <<'F'
2026-11-12T10:00:00.100000+00:00 h factory.sh[1]: 2026-11-12T10:00:00Z NEXT unclaimed:260 brief quotes: 2026-11-09T00:00:00Z PASS units=29 dry=0 budget=5400s root=/wrong/tree trees=/x
2026-11-12T10:00:00.500000+00:00 h factory.sh[1]: 2026-11-12T10:00:00Z HALT quoting an old line verbatim: 2026-11-09T00:00:01Z LOG-AT /quoted/dir/factory/log
2026-11-12T10:00:01.000000+00:00 h systemd[4485]: plrev260b.service: LOG-AT /manager/quotes/this
2026-11-12T10:00:04.000000+00:00 h systemd[4485]: 2026-11-12T10:00:04Z LOG-AT /manager/echoes/this
2026-11-12T10:00:02.000000+00:00 h systemd[4485]: Finished plrev260b.service - planted (one pass).
2026-11-12T10:00:03.000000+00:00 h factory.sh[1]: factory: 1 tick(s), rc=0.
F
# A record that is there and broken, against a window that holds none. Different sentences, and both
# different from "it has not moved", which is the arm neither is allowed to reach.
cat >"$lm_dir/damaged" <<'F'
2026-11-12T10:00:00.100000+00:00 h factory.sh[1]: 2026-11-12T10:00:00Z LOG-AT
2026-11-12T10:00:01.000000+00:00 h factory.sh[1]: 2026-11-12T10:00:01Z LOG-AT   
F
: >"$lm_dir/nothing"
# Four lines with the record FIRST, so a window of three cannot reach it and the default can. That is the
# knob leg: it has to fail at one size and succeed at another, or the bound is decoration.
cat >"$lm_dir/four-deep" <<'F'
2026-11-12T09:00:00.100000+00:00 h factory.sh[1]: 2026-11-12T09:00:00Z LOG-AT /deep/tree/factory/log
2026-11-12T09:00:01.100000+00:00 h factory.sh[1]: 2026-11-12T09:00:01Z QUEUED 2 unit(s) remain
2026-11-12T09:00:02.100000+00:00 h factory.sh[1]: 2026-11-12T09:00:02Z TICK-START issue/1
2026-11-12T09:00:03.100000+00:00 h factory.sh[1]: 2026-11-12T09:00:03Z END rc=0 ticks_started=1
F

lm_leg_recorded_beats_derived() {
	local shape=''
	shape=$(lm_fields "$(lm_ask "$lm_dir/both-marks" 1)")
	[ "$shape" = "/tree/a/factory/log|2026-11-12T10:10:00Z|recorded" ] || {
		lm_say "a pass that NAMED its directory answered: $shape"
		lm_say "want the LOG-AT directory with how=recorded, not the PASS root= with how=derived."
		lm_say "Every pass writes both lines 100 ms apart, so preferring the newest line means"
		lm_say 'preferring the inference, and `recorded` would never appear in a live journal.'; return 1; }
	shape=$(lm_fields "$(lm_ask "$lm_dir/pass-only" 1)")
	[ "$shape" = "/old/tree/factory/log|2026-11-12T10:00:01Z|derived" ] || {
		lm_say "a journal of PASS lines only answered: $shape"
		lm_say "want the root= field plus the /factory/log suffix, with how=derived on the line"; return 1; }
	# The polarity of the second fixture is load-bearing: if the two answers were interchangeable the
	# first assertion above would pass on a parser that cannot read `LOG-AT` at all.
	[ "$shape" != "$(lm_fields "$(lm_ask "$lm_dir/both-marks" 1)")" ] || {
		lm_say "both fixtures answer the same, so neither leg can tell the marks apart"; return 1; }
	lm_say "named answers recorded, unnamed answers derived, and the two differ" >&2
	return 0
}
claim_leg 'log-move: a named directory beats the inferred one' lm_leg_recorded_beats_derived

lm_leg_quoted_unread() {
	local rc=0 naive=''
	lm_rc "$lm_dir/quoted" 1 || rc=$?
	[ "$rc" = 1 ] || {
		lm_say "a window whose only PASS and LOG-AT are both QUOTED answered rc=$rc, want 1 (no record)."
		lm_say "It read a directory out of a quoted transcript or a systemd line -- which is how a"
		lm_say "pass gets accused of moving the journal by a ticket body quoting an old PASS."; return 1; }
	# The control for the control: what an unanchored read answers on this same fixture. It has to be a
	# DIFFERENT verdict, or the fixture cannot detect the hazard and this leg is decoration (#247's
	# `reap: the exclusion matches a real rendering` writes the same shape of control).
	naive=$(awk '{ if (match($0, /root=\/[^ ]+/)) { print substr($0, RSTART + 5, RLENGTH - 5); exit } }' \
		"$lm_dir/quoted")
	[ "$naive" = /wrong/tree ] || {
		lm_say "the unanchored control read '$naive', want /wrong/tree -- the fixture no longer"
		lm_say "holds a quotable root= field, so it cannot detect an unanchored parser"; return 1; }
	# Which rule refuses the MANAGER's lines, named rather than assumed. One planted line carries a
	# harness stamp inside its MESSAGE (`systemd[4485]: 2026-…Z LOG-AT /manager/echoes/this`), which is
	# the shape the stamp anchor cannot refuse and the syslog filter can. The control takes the SHIPPED
	# awk program, drops only the filter line, and runs the same window: if that changes nothing, this
	# fixture cannot pin the filter at all -- every manager line in it is already refused by the stamp
	# rule -- and the leg says so out loud instead of letting the filter take credit for an anchor.
	local nofilter=$lm_dir/nofilter.awk unfiltered=0
	python3 - "$HERE/common.sh" "$nofilter" <<'PY'
import sys

src, out = sys.argv[1], sys.argv[2]
text = open(src).read()
a = text.index('factory_log_prev_record() {')
b = text.index("\tawk '", a) + len("\tawk '")
c = text.index("\n\t'\n", b)
lines = text[b:c].split('\n')
keep = [l for l in lines if 'systemd\\[' not in l]
assert len(keep) == len(lines) - 1, 'the filter line is not the one line expected of the shipped program'
open(out, 'w').write('\n'.join(keep))
PY
	awk -f "$nofilter" <"$lm_dir/quoted" >/dev/null 2>&1 || unfiltered=$?
	[ "$unfiltered" = 0 ] || {
		lm_say "dropping the syslog filter from the shipped program answered rc=$unfiltered, the same"
		lm_say "as the shipped read -- so nothing in this fixture can be pinned to that filter. Plant a"
		lm_say "manager line whose MESSAGE carries a harness stamp, or stop crediting the filter."
		return 1
	}
	lm_say "the stamp anchor refuses the quoted PASS; the filter is what refuses the manager echo" >&2
	return 0
}
claim_leg 'log-move: a quoted PASS names no directory' lm_leg_quoted_unread

lm_leg_damaged_vs_absent() {
	local rc=0 empty=1
	lm_rc "$lm_dir/damaged" 1 || rc=$?
	[ "$rc" = 2 ] || { lm_say "a LOG-AT naming nothing answered rc=$rc, want 2 (damaged)"; return 1; }
	rc=0
	lm_rc "$lm_dir/nothing" 1 || rc=$?
	[ "$rc" = 1 ] || { lm_say "an empty window answered rc=$rc, want 1 (no record)"; return 1; }
	empty=$rc
	[ "$empty" != 2 ] || {
		lm_say "the damaged arm and the empty arm answer alike, so the operator cannot tell them"; return 1; }
	# And the refusal sentences differ too, which is what an operator actually reads.
	local s2='' s3=''
	s2=$(lm_move "$lm_dir/nothing" 400 "$lm_dir/new")
	s3=$(lm_move "$lm_dir/damaged" 400 "$lm_dir/new")
	case $s2 in
	(*'LOG-MOVED-UNKNOWN rc=2'*) : ;;
	*) lm_say "an empty window did not reach the rc=2 sentence:"; lm_say "$s2"; return 1 ;;
	esac
	case $s2 in
	(*FACTORY_LOG_TAIL*) : ;;
	*) lm_say "the rc=2 refusal does not name the knob that would widen the window:"
		lm_say "$s2"; return 1 ;;
	esac
	case $s3 in
	(*'LOG-MOVED-UNKNOWN rc=3'*) : ;;
	*) lm_say "a damaged record did not reach the rc=3 sentence:"; lm_say "$s3"; return 1 ;;
	esac
	case $s3 in
	(*'LOG-MOVED '*) lm_say "a pass that read a broken record still claimed a move:"
		lm_say "$s3"; return 1 ;;
	esac
	# Both refusals still name where this pass is writing: the mark is unconditional, and a pass that
	# cannot answer about the past still says where the present record is.
	case $s2 in
	(*"LOG-AT $lm_dir/new"*) : ;;
	*) lm_say "the rc=2 refusal does not print LOG-AT for this pass's own directory:"
		lm_say "$s2"; return 1 ;;
	esac
	lm_say "damaged is rc=3, absent is rc=2, and both still say where they are writing" >&2
	return 0
}
claim_leg 'log-move: a damaged record is not an absent one' lm_leg_damaged_vs_absent

lm_leg_tail_bound() {
	local out=''
	# -n 3 on this fixture cuts the record off. If the bound never reached journalctl, the record would
	# be found at every size and this leg would pass on a read with no window at all -- so the FAILING
	# size is asserted first, against the stub that honours `-n`.
	out=$(lm_move "$lm_dir/four-deep" 3 "$lm_dir/new")
	case $out in
	(*'LOG-MOVED-UNKNOWN rc=2'*) : ;;
	*) lm_say "FACTORY_LOG_TAIL=3 over a 4-line window whose record is oldest did not run out:"
		lm_say "$out"
		lm_say "Either the bound is not asked of journalctl, or the fixture no longer puts the"; return 1 ;;
	esac
	out=$(lm_move "$lm_dir/four-deep" 400 "$lm_dir/new")
	case $out in
	(*'LOG-MOVED-UNKNOWN'*)
		lm_say "the same window at FACTORY_LOG_TAIL=400 still cannot see the record, so raising"
		lm_say "the knob changes nothing and the refusal that names it is a lie:"
		lm_say "$out"; return 1 ;;
	esac
	case $out in
	(*'LOG-MOVED old=/deep/tree/factory/log'*) : ;;
	*) lm_say "the full window found the record but did not report the move:"
		lm_say "$out"; return 1 ;;
	esac
	case $out in
	(*'how=recorded'*) : ;;
	*) lm_say "the move names a directory the window recorded as derived:"
		lm_say "$out"; return 1 ;;
	esac
	lm_say "3 lines cannot reach the record, 400 can, and the sentence counts the lines it read" >&2
	return 0
}
claim_leg 'log-move: the window is a knob, and the refusal names it' lm_leg_tail_bound

lm_leg_symlink_not_written_through() {
	local dir=$work/log-move-sym out='' rc=0 victim=''
	rm -rf "$dir"
	mkdir -p "$dir/old/factory/log"
	victim=$dir/victim
	# The shape a journal path becomes when a tree is moved by hand and a link is left behind -- and the
	# shape #874 is about in general: a privileged writer following a link it never opened writes
	# wherever the link points. `-e` reports a dangling symlink as NOT THERE, which is the trap: the
	# first cut of this guard folded the link test into an `||` behind `! -e`, never reached it, and the
	# append went straight through and created the target.
	ln -s "$victim" "$dir/old/factory/log/supervisor.log"
	out=$(LM_FIXTURE=/dev/null LM_DUMB=0 LM_UNIT=plrev260b.service LM_TAIL=400 LM_NEW="$dir/new" \
		LM_LINK=$dir/old/factory/log PATH="$lm_bin:$PATH" bash -c '
			. "$1/common.sh" 1>/dev/null
			factory_service_unit() { printf "%s\n" "$LM_UNIT"; }
			printf "%s\n" "2026-11-12T10:00:00.100000+00:00 h factory.sh[1]: 2026-11-12T10:00:00Z LOG-AT $LM_LINK" \
				>"$2/log-move-sym.journal"
			LM_FIXTURE=$2/log-move-sym.journal
			export LM_FIXTURE
			factory_log_move_lines "$LM_NEW"' _ "$HERE" "$work") || rc=$?
	[ "$rc" = 0 ] || { lm_say "the read exited $rc on a planted symlink; want 0 with a verdict"; return 1; }
	case $out in
	(*'tail=skipped refuse=old-journal-is-a-symlink'*) : ;;
	*) lm_say "a symlinked old journal was refused by the wrong clause, or not refused by name."
		lm_say "Want tail=skipped refuse=old-journal-is-a-symlink: the link clause is the one"
		lm_say "that refuses here, and the sentence has to say so -- \`skipped\` alone is five states."
		printf '%s\n' "$out" | sed 's/^/              /' >&2
		return 1 ;;
	esac
	if [ -e "$victim" ]; then
		lm_say "the copy went THROUGH the symlink and created $victim."
		lm_say "That is an arbitrary-write: the path in the journal record is not a path this"
		lm_say 'harness opened, and `-e` answers false for a dangling link, so the file test has'
		lm_say "to refuse a link BEFORE it asks what is there -- as its own clause, not the arm"
		lm_say "of an || that short-circuits first. That is where the first cut lost it."
		return 1
	fi
	[ -L "$dir/old/factory/log/supervisor.log" ] || {
		lm_say "the fixture lost its symlink, so it no longer tests the hazard"; return 1; }
	lm_say "a symlinked old journal is refused and nothing was created behind it" >&2
	return 0
}
claim_leg 'log-move: a symlinked old journal is not written through' lm_leg_symlink_not_written_through

lm_leg_unwritable_is_said_not_shown() {
	local dir=$work/log-move-ro out='' err=''
	rm -rf "$dir"
	mkdir -p "$dir/old/factory/log" "$dir/bin"
	: >"$dir/old/factory/log/supervisor.log"
	chmod 0444 "$dir/old/factory/log/supervisor.log"
	printf '#!/usr/bin/env bash\nprintf "%%s\\n" "2026-11-12T10:00:00.100000+00:00 h factory.sh[1]: 2026-11-12T10:00:00Z LOG-AT %s/old/factory/log"\n' \
		"$dir" >"$dir/bin/journalctl"
	chmod +x "$dir/bin/journalctl"
	# stderr on its own: the verdict is on stdout, which this swallows on purpose. What is checked is that
	# the shell's own `Permission denied` never reaches a pass. A failed REDIRECTION is reported by the
	# shell before that command's own `2>/dev/null` takes effect, so the suppressor has to sit on a group
	# around the append -- and a leg asserting only `tail=failed` let the raw error into the tick log.
	err=$(PATH="$dir/bin:$PATH" LM_UNIT=plrev260b.service bash -c '
		. "$1/common.sh" 1>/dev/null
		factory_service_unit() { printf "%s\n" "$LM_UNIT"; }
		factory_log_move_lines "$2/new"' _ "$HERE" "$dir" 2>&1 >/dev/null)
	[ -z "$err" ] || {
		lm_say "a refused copy leaked text to stderr, which lands in the tick log as a shell error:"
		printf '%s\n' "$err" | sed 's/^/              /' >&2
		chmod 0644 "$dir/old/factory/log/supervisor.log"
		return 1
	}
	out=$(PATH="$dir/bin:$PATH" LM_UNIT=plrev260b.service bash -c '
		. "$1/common.sh" 1>/dev/null
		factory_service_unit() { printf "%s\n" "$LM_UNIT"; }
		factory_log_move_lines "$2/new"' _ "$HERE" "$dir")
	case $out in
	(*'tail=failed'*) : ;;
	*) lm_say "an unwritable old journal did not answer tail=failed:"
		printf '%s\n' "$out" | sed 's/^/              /' >&2
		chmod 0644 "$dir/old/factory/log/supervisor.log"; return 1 ;;
	esac
	case $out in
	(*'LOG-MOVED old='*) : ;;
	*) lm_say "a refused copy suppressed the move sentence, which is the operator's only clue:"
		printf '%s\n' "$out" | sed 's/^/              /' >&2
		chmod 0644 "$dir/old/factory/log/supervisor.log"; return 1 ;;
	esac
	chmod 0644 "$dir/old/factory/log/supervisor.log"
	lm_say "a refused copy is a field on the verdict, not an exception on stderr" >&2
	return 0
}
claim_leg 'log-move: a refused copy is a verdict, not a shell error' lm_leg_unwritable_is_said_not_shown

# The five clauses of the copy guard, each made to answer for itself (#260, H-0149).
#
# `tail=skipped` is what every refusal looks like, and the sentence printed beside it named ONE of them --
# "found no directory there" -- whatever had refused. Measured on 12 Nov against five planted old
# journals: an old directory that is a link, one that is not writable, a journal file that is a link, and
# a journal file that is a FIFO or a directory, each printed "found no directory there" beside a directory
# that was there and readable. The operator's next move differs per clause -- only `no-such-directory`
# means there is no file left to tail -- and a false sentence about the state it reports is exactly what
# #260 was filed about, reproduced inside the fix for it. So the guard answers `refuse=<clause>`, and this
# leg refuses a chain that collapses into one token or a legend that drifts from it.
#
# Every case is collected rather than returned from: a probe that dies at its first finding under-reports
# exactly like a gate that skips (the note on `check`). The FIFO is the one shape that would HANG a
# regression -- `>>` on a FIFO with no reader blocks forever, which is #246's parked-pass shape -- so
# that call is the one wrapped in `timeout`, the same spelling the process legs use.
lm_leg_refusal_names_the_clause() {
	local dir=$work/log-move-clauses out='' bad=0 victim='' shape='' want='' fix='' rc=0 sw='' fields=''
	victim=$dir/victim
	rm -rf "$dir"
	mkdir -p "$dir/dtarget" "$dir/rodir" "$dir/flink" "$dir/dirfile" "$dir/fifo" "$dir/plain"
	printf 'someones file\n' >"$victim"
	ln -s "$dir/dtarget" "$dir/dlink"
	# The same link pointing at NOTHING: `-d` follows links, so a chain that asks `-d` first answers
	# "no such directory" here and never reaches the link clause. This is the shape a tree moved by hand
	# leaves behind -- the 10 Nov event -- and the one the probe found mislabelled on 12 Nov.
	ln -s "$dir/target-gone" "$dir/danglink"
	ln -s "$victim" "$dir/flink/supervisor.log"
	mkdir -p "$dir/dirfile/supervisor.log"
	mkfifo "$dir/fifo/supervisor.log"
	: >"$dir/plain/supervisor.log"
	chmod 0500 "$dir/rodir"

	# shape:want  -- `plain:` wants no refusal at all, and is the polarity the six above need.
	for sw in 'gone:no-such-directory' 'dlink:old-directory-is-a-symlink' \
		'danglink:old-directory-is-a-symlink' 'rodir:old-directory-not-writable' \
		'flink:old-journal-is-a-symlink' 'dirfile:old-journal-not-a-regular-file' \
		'fifo:old-journal-not-a-regular-file' 'plain:'; do
		shape=${sw%%:*}
		want=${sw#*:}
		fix=$dir/j-$shape
		printf '2026-11-12T10:00:00.100000+00:00 h factory.sh[1]: 2026-11-12T10:00:00Z LOG-AT %s/%s\n' \
			"$dir" "$shape" >"$fix"
		rc=0
		out=$(LM_FIXTURE=$fix LM_UNIT=plrev260b.service LM_NEW=$dir/new PATH="$lm_bin:$PATH" \
			timeout --signal=TERM --kill-after=5 20 bash -c '
				. "$1/common.sh" 1>/dev/null
				factory_service_unit() { printf "%s\n" "$LM_UNIT"; }
				factory_log_move_lines "$LM_NEW"' _ "$HERE" 2>&1) || rc=$?
		if [ "$rc" = 124 ]; then
			lm_say "$shape: the pass blocked instead of refusing -- a FIFO opened for append with"
			lm_say "no reader parks the whole pass, which is #246's parked activation."
			bad=$((bad + 1))
			continue
		fi
		# The fields, not the prose: the legend spells `refuse=` and `tail=` as words on the same line,
		# so a substring test on the whole sentence cannot tell the field from its own documentation.
		fields=$(printf '%s\n' "$out" | sed -n 's/.*LOG-MOVED \(.*\) -- .*/\1/p' | head -1)
		if [ -n "$want" ]; then
			case " $fields " in
			(*" tail=skipped refuse=$want "*) : ;;
			*) lm_say "$shape: answered fields: ${fields:-<none>}"
				lm_say "$out"
				lm_say "want tail=skipped refuse=$want as the FIELDS on the line. Five clauses"
				lm_say "answer skipped; naming the wrong one sends an operator to look for a tree"
				lm_say "that is still there, and naming none sends them to the legend for a fact"
				lm_say "this pass knew."
				bad=$((bad + 1)) ;;
			esac
			# The legend on the line enumerates every clause, so it cannot drift behind the chain.
			case $out in
			(*old-directory-is-a-symlink*no-such-directory*old-directory-not-writable*old-journal-is-a-symlink*old-journal-not-a-regular-file*) : ;;
			*) lm_say "$shape: the sentence does not name all five clauses, so a reader cannot"
				lm_say "tell a refused old journal from a deleted tree."; bad=$((bad + 1)) ;;
			esac
		else
			case " $fields " in
			(*' tail=ok '*) : ;;
			*) lm_say "plain: a real directory and a real regular file answered: ${fields:-<none>}"
				lm_say "want tail=ok -- the refusals above would be decoration without it."
				bad=$((bad + 1)) ;;
			esac
			case " $fields " in
			(*refuse=*) lm_say "plain: an ATTEMPTED copy still named a refusal clause: $fields"
				lm_say "refuse= means nothing was opened; it must be absent when the append ran."
				bad=$((bad + 1)) ;;
			esac
			grep -q 'LOG-MOVED' "$dir/plain/supervisor.log" || {
				lm_say "plain: the copy did not land in the old journal."; bad=$((bad + 1)); }
		fi
	done

	# Nothing was written through or into anything it refused.
	grep -q '^someones file$' "$victim" || {
		lm_say "the copy appended THROUGH a symlinked old journal into $victim."; bad=$((bad + 1)); }
	[ ! -e "$dir/rodir/supervisor.log" ] || {
		lm_say "the copy wrote into a directory it had just called not writable."; bad=$((bad + 1)); }
	[ ! -e "$dir/dtarget/supervisor.log" ] || {
		lm_say "the copy followed a linked old directory into its target."; bad=$((bad + 1)); }
	[ ! -e "$dir/target-gone" ] || {
		lm_say "the append went THROUGH a dangling linked directory and CREATED its target."
		lm_say "That is the arbitrary write #874 refuses, and the reason the link clause is the"
		lm_say "first one asked -- \`-d\` answers false for a dangling link, so a chain that asks"
		lm_say "\`-d\` first both mislabels the verdict and reaches the append."
		bad=$((bad + 1)); }
	[ ! -e "$dir/dirfile/supervisor.log/supervisor.log" ] || {
		lm_say "the copy treated a directory as the old journal file."; bad=$((bad + 1)); }
	chmod 0700 "$dir/rodir"
	[ "$bad" = 0 ] || return 1
	lm_say "five clauses, five names, and the one that means the tree is gone says only that" >&2
	return 0
}
claim_leg 'log-move: each copy refusal names its own clause' lm_leg_refusal_names_the_clause

# The call site is pinned by the four `proc_*` legs further up (`the pass names its own journal
# directory`, `…and says so in the journal it is leaving`, `…and says nothing when the journal stayed
# put`, `…and refuses in words when it cannot read`), which run the shipped factory.sh and read the
# journal it wrote. No text scan of factory.sh sits here, for the reason the note above the reap legs
# gives: a scan that asserts a stronger harness does not exist is a lie waiting to be cited.

# ======================================= the merge that closed nothing the body said it closed (#353)
#
# `factory_verify_closure` (`factory/factory.sh`) asks, after a merge, whether the issues the merged
# body named with a verb actually closed. These legs run the SHIPPED function: its bytes, plus the
# `CLOSER_RE` line it reads, are lifted out of `factory/factory.sh` at run time and sourced over three
# stubs -- `api_get` answered from a plant directory, `log_event` appending to a journal, `run_action`
# recording the write instead of performing it. Nothing re-types the component, so a leg cannot pass
# on a paraphrase of it.
#
# The fidelity limit, stated instead of glossed. The `proc_*` tick fixture further up runs the shipped
# `factory.sh` end to end and is the right home for the strongest kind of call-site claim, but its
# queue answers `pr-merged.json` as `[]` and its stub lists `pr:merge` among the writes it refuses, so
# no pass in that fixture reaches a merge -- and this component is only interesting AT a merge. So
# `the call site sits in factory_merge, after the ledger stamp` is a call-site pin and asserts only
# that the line is there and in order. Nothing here claims no stronger harness exists.

cv_work="$work/close-verb"
cv_plant="$cv_work/plant"
cv_rec="$cv_work/rec"
cv_script="$HERE/../scripts/check-close-verb.py"
mkdir -p "$cv_plant" "$cv_rec"

# Lift the shipped function, and the pattern line it reads, out of the shipped file -- and refuse to
# run anything if the lift came up short. An empty extraction would make every leg below a pass over
# nothing, which is the dead mutant in a costume.
cv_extract() {
	{
		grep -m1 '^CLOSER_RE=' "$1" || return 1
		# Both functions the component calls. Lifting only `factory_verify_closure` was measured here:
		# the legs that read a delta failed while the ones that read a note passed, and the trace said
		# `factory_close_delta: command not found` -- a fixture missing a helper, not a defect in the
		# component.
		# ONE span, from the first helper through the closing brace of the component. Two ranges that
		# each stop at the first `^}` silently drop whichever helpers sit between them, and the
		# component now calls three: `factory_close_delta`, `factory_close_read` and
		# `factory_close_verdict`. This block already documents being bitten once by a helper the lift
		# forgot (`factory_close_delta: command not found`), so the two newest are asserted below as
		# well -- a leg that fails with command-not-found is a broken fixture, not a verdict.
		awk '/^factory_close_delta\(\) \{/{f=1} /^factory_verify_closure\(\) \{/{v=1}
		     f{print} v && /^\}$/ {exit}' "$1" || return 1
	} >"$cv_work/fvc.sh"
	[ -s "$cv_work/fvc.sh" ] || return 1
	grep -q 'CLOSE-OPEN' "$cv_work/fvc.sh" || return 1
	grep -q 'factory_close_delta()' "$cv_work/fvc.sh" || return 1
	grep -q 'factory_close_read()' "$cv_work/fvc.sh" || return 1
	grep -q 'factory_close_verdict()' "$cv_work/fvc.sh" || return 1
	[ "$(grep -c '^CLOSER_RE=' "$cv_work/fvc.sh")" = 1 ]
}

# One plant per REST path, slashes turned into underscores. A path with no plant answers the caller's
# own default, exactly as the shipped `api_get` does when `gh api` fails: that is the case the
# unreadable-ticket leg needs, and the reason the stub reproduces the default rather than inventing one.
#
# A plant whose JSON is an ARRAY is a sequence of answers, one per call, and the last repeats. The
# re-probe needs a ticket that reads OPEN first and CLOSED second; without that the asynchronous
# closer -- the reason the re-probe exists at all -- could not be exercised.
cv_api_get() {
	local path=$1 filt=$2 def=$3 f pick n
	f="$cv_plant/$(printf '%s' "$path" | tr '/' '_').json"
	[ -f "$f" ] || {
		printf '%s' "$def"
		return 0
	}
	if jq -e 'type == "array"' "$f" >/dev/null 2>&1; then
		n=0
		[ -f "$cv_rec/seq" ] && n=$(cat "$cv_rec/seq")
		pick=$(jq -c --argjson i "$n" 'if (length - 1) < $i then .[-1] else .[$i] end' "$f" 2>/dev/null) || {
			printf '%s' "$def"
			return 0
		}
		printf '%s' "$((n + 1))" >"$cv_rec/seq"
		printf '%s' "$pick" | jq -r "$filt" 2>/dev/null || printf '%s' "$def"
		return 0
	fi
	jq -r "$filt" "$f" 2>/dev/null || printf '%s' "$def"
}

# cv_run <script> <pr> -> the journal in $cv_work/journal, notes in $cv_rec/<n>.body
cv_run() {
	(
		set +eu
		# shellcheck disable=SC2034  # the repository's own name and the two windows are read by the
		# shipped function sourced below -- by everything except a static read of this file.
		FACTORY_OWNER=example FACTORY_NAME=repo FACTORY_CLOSE_PROBE=0 FACTORY_CLOSE_FIRED_S=120
		# The three stubs a factory.sh function actually needs. They are assigned as functions here,
		# and `cv_api_get` is installed under the name the component calls, so the shipped bytes are
		# what runs.
		# shellcheck disable=SC2317  # the shadows read as unreachable code, and they are: to everything
		# except the function below them, which calls all three by name. Same reason as the shadows at
		# the head of this file. One directive per shadow, because the directive ends with the command
		# it precedes -- measured here, where the first one covered `api_get` and left its two siblings.
		api_get() { cv_api_get "$@"; }
		# shellcheck disable=SC2317
		log_event() { printf '%s\n' "$*" >>"$cv_work/journal"; }
		# shellcheck disable=SC2317
		run_action() {
			printf '%s\n' "$*" >>"$cv_rec/calls"
			if [ "$1" = "gh" ] && [ "$2" = "issue" ] && [ "$3" = "comment" ]; then
				# A plant named `fail-note` refuses the note write. Without a switch the stub always
				# succeeds, so `CLOSE-NOTE-FAILED` is an arm nobody has ever run -- which is #357's
				# review, non-blocking 2, stated for this branch in particular.
				if [ -e "$cv_plant/fail-note" ]; then
					return 1
				fi
				printf '%s' "${6:-}" >"$cv_rec/$4.body"
			fi
			return 0
		}
		# shellcheck disable=SC1090
		source "$1"
		factory_verify_closure "$2"
	)
}

cv_reset() {
	rm -rf "$cv_plant" "$cv_rec"
	mkdir -p "$cv_plant" "$cv_rec"
	: >"$cv_work/journal"
	# The repository's own two specimens, at their measured deltas: #301, closed 14472 s after PR #338
	# merged, and #304, closed 6086 s after PR #308 merged.
	printf '%s\n' '{"body":"The work is finished.\n\nCloses #301 — the whole repair.\n","merged_at":"2026-11-13T19:15:34Z","merge_commit_sha":"90f3c21"}' >"$cv_plant/pulls_77.json"
	printf '%s\n' '{"state":"OPEN","closed_at":null}' >"$cv_plant/issues_301.json"
	printf '%s\n' '{"body":"One fix.\n\nCloses #9.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_78.json"
	printf '%s\n' '{"state":"CLOSED","closed_at":"2026-11-13T19:15:36Z"}' >"$cv_plant/issues_9.json"
	printf '%s\n' '{"body":"Two tickets, one verb.\n\nCloses #304 on merge.\n","merged_at":"2026-11-12T00:45:14Z"}' >"$cv_plant/pulls_79.json"
	printf '%s\n' '{"state":"CLOSED","closed_at":"2026-11-12T02:26:40Z"}' >"$cv_plant/issues_304.json"
	printf '%s\n' '{"body":"Closed with no clock.\n\nCloses #11.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_80.json"
	printf '%s\n' '{"state":"CLOSED","closed_at":null}' >"$cv_plant/issues_11.json"
	printf '%s\n' '{"body":"A ticket nobody can read.\n\nCloses #12.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_81.json"
	printf '%s\n' '{"body":"Nothing named with a verb here, only prose.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_82.json"
	printf '%s\n' '{"body":"Part of #14 only; the umbrella is not finished.\n\nRefs #15.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_83.json"
	printf '%s\n' '{"body":"The keyword is deliberate. `Closes #8.` — inside a span.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_84.json"
	printf '%s\n' '{"state":"OPEN","closed_at":null}' >"$cv_plant/issues_8.json"
	printf '%s\n' '{"body":"In flight.\n\nCloses #10.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_85.json"
	printf '%s\n' '[{"state":"OPEN","closed_at":null},{"state":"CLOSED","closed_at":"2026-11-13T19:15:37Z"}]' >"$cv_plant/issues_10.json"
}

cv_reset
cv_extract "$HERE/factory.sh"

# --- the specimen: the body asked, the ticket stayed open -----------------------------------------
cv_run "$cv_work/fvc.sh" 77 >/dev/null 2>&1
claim_leg 'a verb naming an open ticket is CLOSE-OPEN' bash -c \
	'grep -q "CLOSE-OPEN pr:77 issue:301" "$1" && ! grep -q "CLOSE-SETTLED pr:77" "$1"' _ \
	"$cv_work/journal"
claim_leg '…and the note goes to the ticket, not to the pull request' bash -c \
	'grep -q "^gh issue comment 301 --body" "$1/calls" &&
	 ! grep -q "gh pr comment" "$1/calls" || {
		printf "\trec holds [%s]\n" "$(ls "$1" 2>&1 | tr "\n" " ")"
		printf "\tcalls head [%s]\n" "$(head -c 140 "$1/calls" 2>&1 | tr "\n" "|")"
		exit 1
	}' _ "$cv_rec"
claim_leg '…and the note quotes the verb line inside a span of its own' bash -c \
	'grep -qF "Closes #301 — the whole repair." "$1/301.body" && grep -qF "$2" "$1/301.body"' _ \
	"$cv_rec" '```'
claim_leg '…and the closing-keyword checker runs over the note it wrote' bash -c \
	'python3 "$1" "$2/301.body" >"$2/verdict" 2>&1 || [ -s "$2/verdict" ]' _ "$cv_script" "$cv_rec"
claim_leg '…which finds the quoted verb inert and nothing that fires' bash -c \
	'grep -q "inert #301" "$1/verdict" && ! grep -qE "closes #|CLOSES DESPITE" "$1/verdict"' _ "$cv_rec"
claim_leg '…and says in words that nothing was closed' bash -c \
	'grep -qF "Nothing has been closed by this note, on purpose" "$1/301.body"' _ "$cv_rec"

# --- the design decision: the scan reads spans the way GitHub does not ----------------------------
cv_reset
cv_run "$cv_work/fvc.sh" 84 >/dev/null 2>&1
claim_leg 'a verb only ever inside a code span is still noticed' bash -c \
	'grep -q "CLOSE-OPEN pr:84 issue:8" "$1"' _ "$cv_work/journal"
claim_leg '…because the specimen that started this was exactly that' bash -c \
	'grep -qF "\`Closes #8.\`" "$1/8.body"' _ "$cv_rec"

# --- the two shipped spellings of one rule ----------------------------------------------------------
# `CLOSER_RE` in `factory/factory.sh` is an Oniguruma program handed to jq; `CLOSER` in
# `scripts/check-close-verb.py` is a Python one. Two files, one rule, and the rule is the thing that
# cost a unit of work -- so the drift between them is a defect with a price on it, and until now
# NOTHING compared the two: every leg in this section exercises the shipped function, and the function
# never reads the checker's pattern. Neither spelling is re-typed here. The jq one is lifted out of
# `factory.sh` on the line below and the Python one is imported out of the checker, so a leg cannot
# pass on a paraphrase of either -- which is the only reason it is a test of agreement rather than a
# second copy of the rule.
#
# Both engines see ONE LINE AT A TIME, the only surface they share: the checker classifies whole bodies
# with spans stripped and that is its own business, pinned by its own `--self-test`. The table is the
# whole disagreement set plus the load-bearing agreements -- every spelling in GitHub's documented
# keyword set, the two `-ing` forms that used to be in the shipped pattern, the separator forms, the
# merged lines a 20-character filler window used to turn into closers, `Refs`, a negation, the specimen,
# and GitHub's own multi-issue sentence.
cv_spellings_agree() {
	local line py sh re_line
	re_line=$(grep -m1 "^CLOSER_RE=" "$HERE/factory.sh" | sed "s/^CLOSER_RE='//; s/'\$//")
	[ -n "$re_line" ] || {
		printf '\tCLOSER_RE could not be read out of factory/factory.sh\n'
		return 1
	}
	while IFS= read -r line; do
		[ -n "$line" ] || continue
		py=$(python3 -c '
import importlib.util, sys
spec = importlib.util.spec_from_file_location("cv", sys.argv[1])
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)
print(" ".join(sorted(set(mod.CLOSER.findall(sys.argv[2])))))
' "$HERE/../scripts/check-close-verb.py" "$line") || {
			printf '\tthe checker could not classify [%s]\n' "$line"
			return 1
		}
		sh=$(printf '%s\n' "$line" | jq -Rsr --arg re "$re_line" \
			'[scan($re) | .[0] | ltrimstr("#")] | unique | join(" ")' 2>/dev/null) || {
			printf '\tjq refused the shipped pattern on [%s]\n' "$line"
			return 1
		}
		if [ "$py" != "$sh" ]; then
			printf '\tline    [%s]\n\tchecker [%s]\n\tfactory  [%s]\n' "$line" "$py" "$sh"
			return 1
		fi
	done <<'LINES'
Closes #301 — the whole repair.
Closes: #77
close, #77
Closed #12 by the merge
Fixed #13
Fixes #14
Resolve #15
Resolves #16
Resolved #17
Closes #301 and Closes #304.
Closing #77
Resolving #78
Fixing #79
Closes the write half of #10's remaining criteria
Also fixes item 1 of #25 in passing
pull request #292 has the fix. Refs #291 for context only.
needs the boundary resolved by hand, and #1180 is why
Refs #167 rather than closing it: criterion 2 is #168's
**This does not close #10.**
The harness reads `Closes #8` inside a span
Part of #14 only; the umbrella is not finished
LINES
	return 0
}
claim_leg 'the two shipped spellings of the closer answer the same on every line of the table' \
	cv_spellings_agree

# The same repair at the function level, because the agreement leg above cannot see what the scan does
# with a number. A body whose only verb sits twenty characters from the number it used to reach has to
# produce NO verdict and NO write: `pull request #292 has the fix. Refs #291 for context only.` used to
# be scanned as a request for #291 -- which is still open, so the function would then have written a
# note onto #291 saying its own merge failed to close it, about a keyword that was never aimed at it.
cv_leg_no_window() {
	cv_reset
	printf '%s\n' \
		'{"body":"The repair is elsewhere: pull request #292 has the fix. Refs #291 for context only.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_86.json"
	cv_run "$cv_work/fvc.sh" 86 >/dev/null 2>&1
	if [ -s "$cv_work/journal" ]; then
		printf '\tthe windowed line still produced a verdict: %s\n' "$(head -1 "$cv_work/journal")"
		return 1
	fi
	if [ -s "$cv_rec/calls" ]; then
		printf '\tthe windowed line still wrote somewhere: %s\n' "$(head -1 "$cv_rec/calls")"
		return 1
	fi
	return 0
}
claim_leg 'a verb twenty characters away from a number is not scanned as a closer at all' \
	cv_leg_no_window

# --- the healthy path is silent; an in-flight closer is not a failure -----------------------------
cv_reset
cv_run "$cv_work/fvc.sh" 78 >/dev/null 2>&1
claim_leg 'a ticket closed two seconds after the merge says nothing at all' bash -c \
	'[ ! -s "$1" ] && [ ! -e "$2/9.body" ]' _ "$cv_work/journal" "$cv_rec"
cv_reset
cv_run "$cv_work/fvc.sh" 85 >/dev/null 2>&1
claim_leg 'a closer still in flight is re-probed, not reported' bash -c \
	'grep -q "CLOSE-SETTLED pr:85 issue:10" "$1" && [ ! -e "$2/10.body" ]' _ \
	"$cv_work/journal" "$cv_rec"

# --- the two ways a close can be somebody else doing it -------------------------------------------
cv_reset
cv_run "$cv_work/fvc.sh" 79 >/dev/null 2>&1
claim_leg 'a close 6086 s after the merge is named a hand close' bash -c \
	'grep -q "CLOSE-LATE pr:79 issue:304" "$1" && grep -q "6086s after the merge" "$1" &&
	 [ ! -e "$2/304.body" ]' _ "$cv_work/journal" "$cv_rec"
cv_reset
cv_run "$cv_work/fvc.sh" 80 >/dev/null 2>&1
claim_leg 'a closed ticket with no timestamp is untimed, not a delta of zero' bash -c \
	'grep -q "CLOSE-UNTIMED pr:80 issue:11" "$1" && ! grep -q "0s after the merge" "$1"' _ \
	"$cv_work/journal"
# The helper on its own, because `date -u -d "" +%s` SUCCEEDS on this box: measured, it prints
# 1794614400 with rc 0, which is today's midnight. A `closed_at` the API returns as null reaches the
# helper as an empty string, so a guard that only asks whether `date` worked turns a missing clock into
# a delta of a few thousand seconds -- and the pass then prints the most confident sentence it has,
# "the keyword did not fire, something else closed it", about a ticket whose close time was never
# recorded. Refusing to answer is the only honest verdict, and `CLOSE-UNTIMED` is what says so.
cv_delta_empty() {
	local out
	out=$(
		# shellcheck disable=SC1090,SC1091  # the lifted bytes, by path; nothing static to follow.
		source "$cv_work/fvc.sh"
		factory_close_delta 2026-11-13T19:15:34Z ''
	)
	[ "$out" = "unknown" ] || {
		printf '\tthe helper answered %s for a missing timestamp, and date -d "" answers %s with rc 0\n' \
			"${out:-<empty>}" "$(date -u -d '' +%s 2>/dev/null)"
		return 1
	}
}
claim_leg 'a missing timestamp is the word unknown, not the number date hands back' cv_delta_empty

# --- what a reader that cannot read has to say ----------------------------------------------------
cv_reset
cv_run "$cv_work/fvc.sh" 81 >/dev/null 2>&1
claim_leg 'a ticket that cannot be read is UNKNOWN, not open' bash -c \
	'grep -q "CLOSE-CHECK-UNKNOWN pr:81 issue:12" "$1" && ! grep -q "CLOSE-OPEN pr:81" "$1"' _ \
	"$cv_work/journal"
cv_reset
rm -f "$cv_plant/pulls_82.json"
cv_run "$cv_work/fvc.sh" 82 >/dev/null 2>&1
claim_leg 'a body that cannot be read skips the check in words' bash -c \
	'grep -q "CLOSE-CHECK-SKIPPED pr:82 the merged body could not be read" "$1"' _ "$cv_work/journal"

# --- what is not a closer -------------------------------------------------------------------------
cv_reset
cv_run "$cv_work/fvc.sh" 82 >/dev/null 2>&1
claim_leg 'a body naming nothing with a verb compares nothing' bash -c \
	'[ ! -s "$1" ] && [ ! -s "$2/calls" ]' _ "$cv_work/journal" "$cv_rec"
cv_reset
cv_run "$cv_work/fvc.sh" 83 >/dev/null 2>&1
# `[ ! -e ]`, not `! grep`: the healthy path never creates the call log, so grepping it printed
# `grep: ... /rec/calls: No such file or directory` on every run and the `!` counted that error as
# the pass -- a leg that passes because its own read failed (#357's review, non-blocking 7).
claim_leg 'Refs and Part of are not closers and get no verdict' bash -c \
	'[ ! -s "$1" ] && [ ! -e "$2/calls" ]' _ "$cv_work/journal" "$cv_rec"


# --- one verb, every number it touches, and what the number is ------------------------------------
# #357's review's blocking 3: the scan used `capture`, which answers the FIRST `#N` on a matching line
# and nothing else. Two shapes break on that, both measured on this box with the shipped pattern before
# the repair: `Closes #301 and Closes #304.` -- GitHub's own multi-issue sentence -- came back as `301`
# alone, and `#7 is the reason. Fixes #12.` came back as `7`, which is not the ticket the verb belongs
# to. `scan($re)` yields every match of the pattern, so a number only appears if a keyword touched it.
cv_reset
printf '%s\n' '{"body":"Two tickets, one verb.\n\nCloses #91 and Closes #92.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_87.json"
printf '%s\n' '{"state":"CLOSED","closed_at":"2026-11-13T21:45:34Z"}' >"$cv_plant/issues_91.json"
printf '%s\n' '{"state":"CLOSED","closed_at":"2026-11-13T21:12:14Z"}' >"$cv_plant/issues_92.json"
cv_run "$cv_work/fvc.sh" 87 >/dev/null 2>&1
claim_leg 'one verb over two numbers verdicts BOTH of them' bash -c \
	'grep -q "CLOSE-LATE pr:87 issue:91" "$1" && grep -q "CLOSE-LATE pr:87 issue:92" "$1" &&
	 [ "$(wc -l <"$1")" = 2 ]' _ "$cv_work/journal"

cv_reset
printf '%s\n' '{"body":"The number first is not the number asked about.\n\n#7 is the reason. Fixes #12.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_88.json"
printf '%s\n' '{"state":"CLOSED","closed_at":"2026-11-13T21:45:34Z"}' >"$cv_plant/issues_7.json"
printf '%s\n' '{"state":"CLOSED","closed_at":"2026-11-13T22:45:34Z"}' >"$cv_plant/issues_12.json"
cv_run "$cv_work/fvc.sh" 88 >/dev/null 2>&1
claim_leg 'a verb verdicts the number it touches, not the first number on the line' bash -c \
	'grep -q "CLOSE-LATE pr:88 issue:12" "$1" && ! grep -q "issue:7" "$1" &&
	 [ "$(wc -l <"$1")" = 1 ]' _ "$cv_work/journal"

# A verb aimed at a pull request is not a closing request: GitHub's closer moves issues. The fixture
# plants the REST shape a pull request actually carries on the issues route -- `state: closed`, a real
# `closed_at`, and a non-null `pull_request` -- because without the third field the pass reports a
# merged sibling as a ticket GitHub failed to close. The raw REST read is refused in this session
# (`gh api repos/.../issues/282` prints `Permission denied: shell`), so the payload is planted here
# rather than quoted from a live call.
cv_reset
printf '%s\n' '{"body":"Its fix merged as a sibling.\n\nCloses #282.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_89.json"
printf '%s\n' '{"state":"closed","closed_at":"2026-11-13T19:15:35Z","pull_request":{"url":"https://api.example/repos/repo/pulls/282"}}' >"$cv_plant/issues_282.json"
cv_run "$cv_work/fvc.sh" 89 >/dev/null 2>&1
claim_leg 'a verb aimed at a pull request is named, and gets no issue verdict' bash -c \
	'grep -q "CLOSE-NOT-ISSUE pr:89 ref:282" "$1" && ! grep -qE "CLOSE-(SETTLED|LATE|OPEN|UNTIMED)" "$1" &&
	 [ ! -e "$2/282.body" ]' _ "$cv_work/journal" "$cv_rec"

# --- the re-probe arm grades by the same rule as the first read -----------------------------------
# #357's review's non-blocking 1, reproduced against the bytes as shipped: a ticket that read OPEN then
# CLOSED with `closed_at: null` printed `CLOSE-SETTLED ... closed unknown s after the merge, inside the
# window GitHub takes`, and one closed 40400 s after the merge printed the same healthy sentence with
# the window at 120 s. Both arms now call `factory_close_verdict`, so both say what the first read says.
cv_reset
printf '%s\n' '{"body":"Closes it late and with no clock.\n\nCloses #93.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_90.json"
printf '%s\n' '[{"state":"OPEN","closed_at":null},{"state":"CLOSED","closed_at":null}]' >"$cv_plant/issues_93.json"
cv_run "$cv_work/fvc.sh" 90 >/dev/null 2>&1
claim_leg 'a close that lands during the re-probe with no clock is UNTIMED, not settled' bash -c \
	'grep -q "CLOSE-UNTIMED pr:90 issue:93" "$1" && ! grep -q "CLOSE-SETTLED pr:90" "$1" &&
	 ! grep -q "unknown s after the merge" "$1"' _ "$cv_work/journal"
cv_reset
printf '%s\n' '{"body":"Closes it nine hours late.\n\nCloses #94.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_91.json"
printf '%s\n' '[{"state":"OPEN","closed_at":null},{"state":"CLOSED","closed_at":"2026-11-14T06:28:54Z"}]' >"$cv_plant/issues_94.json"
cv_run "$cv_work/fvc.sh" 91 >/dev/null 2>&1
claim_leg 'a close that lands during the re-probe 40400 s late is LATE, not settled' bash -c \
	'grep -q "CLOSE-LATE pr:91 issue:94" "$1" && grep -q "40400s after the merge" "$1" &&
	 ! grep -q "CLOSE-SETTLED pr:91" "$1"' _ "$cv_work/journal"

# --- the arms the section had no way to reach -----------------------------------------------------
# #357's review's non-blocking 2: five branches of this function had no leg. Four are lit here. The
# fifth -- the `scan_state = failed` skip -- stays dark on purpose: it fires only when `jq` itself
# fails on a well-formed body, and the only way to make that happen in this harness is to break the
# `jq` the body read uses too, which lands the run in the `CLOSE-CHECK-SKIPPED ... body could not be
# read` arm instead. A leg there would be a leg about the fixture.
cv_reset
printf '%s\n' '{"body":"Names a ticket that answers neither way.\n\nCloses #95.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_92.json"
printf '%s\n' '{"state":"MERGED","closed_at":"2026-11-13T19:15:36Z"}' >"$cv_plant/issues_95.json"
cv_run "$cv_work/fvc.sh" 92 >/dev/null 2>&1
claim_leg 'a state that is neither OPEN nor CLOSED is UNKNOWN, in words' bash -c \
	'grep -q "CLOSE-CHECK-UNKNOWN pr:92 issue:95 read back state=MERGED, which is neither answer" "$1"' \
	_ "$cv_work/journal"
cv_reset
printf '%s\n' '{"body":"Answers neither way, then answers it again.\n\nCloses #96.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_93.json"
printf '%s\n' '[{"state":"OPEN","closed_at":null},{"state":"MERGED","closed_at":null}]' >"$cv_plant/issues_96.json"
cv_run "$cv_work/fvc.sh" 93 >/dev/null 2>&1
claim_leg 'the re-probe names a state that is neither answer too' bash -c \
	'grep -q "read back state=MERGED after the re-probe, which is neither answer" "$1" &&
	 ! grep -q "CLOSE-SETTLED pr:93" "$1"' _ "$cv_work/journal"
cv_reset
printf '%s\n' '{"body":"A merge the API has no timestamp for.\n\nCloses #97.\n"}' >"$cv_plant/pulls_94.json"
printf '%s\n' '{"state":"OPEN","closed_at":null}' >"$cv_plant/issues_97.json"
cv_run "$cv_work/fvc.sh" 94 >/dev/null 2>&1
claim_leg 'no merged_at means this pass is not after a merge, and says so' bash -c \
	'grep -q "CLOSE-CHECK-SKIPPED pr:94 the API reports no merged_at" "$1" &&
	 [ ! -s "$2/calls" ]' _ "$cv_work/journal" "$cv_rec"
cv_reset
printf '%s\n' '{"body":"Closed before it was merged.\n\nCloses #98.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_95.json"
printf '%s\n' '{"state":"CLOSED","closed_at":"2026-11-13T18:15:34Z"}' >"$cv_plant/issues_98.json"
cv_run "$cv_work/fvc.sh" 95 >/dev/null 2>&1
claim_leg 'a close that predates the merge is named with its negative delta' bash -c \
	'grep -q "CLOSE-LATE pr:95 issue:98" "$1" && grep -q -- "-3600s after the merge" "$1"' \
	_ "$cv_work/journal"
cv_reset
printf '%s\n' '{"body":"Still open, and the note cannot be written.\n\nCloses #99.\n","merged_at":"2026-11-13T19:15:34Z"}' >"$cv_plant/pulls_96.json"
printf '%s\n' '{"state":"OPEN","closed_at":null}' >"$cv_plant/issues_99.json"
: >"$cv_plant/fail-note"
cv_run "$cv_work/fvc.sh" 96 >/dev/null 2>&1
claim_leg 'a note that cannot be written is journalled, not swallowed' bash -c \
	'grep -q "CLOSE-OPEN pr:96 issue:99" "$1" && grep -q "CLOSE-NOTE-FAILED pr:96 issue:99" "$1" &&
	 ! grep -q "issue:99 the issue could not be read" "$1"' _ "$cv_work/journal"
cv_reset

# One wait for the batch, not one per open ticket (#357's review, non-blocking 6): the sleep is outside
# the loop, so the shipped source has exactly one `sleep` between the two passes and none inside either
# loop body. Read as text because a leg cannot time a 0-second probe, and the defect was a count.
cv_one_sleep() {
	local body start
	start=$(grep -n '^factory_verify_closure() {' "$cv_work/fvc.sh" | cut -d: -f1)
	body=$(awk -v a="$start" 'NR>a' "$cv_work/fvc.sh")
	# Counted on the variable, not on the word: the comment above the batch line says "the sleep was
	# inside the loop", and a count of the word reads that sentence as a second call site.
	[ "$(printf '%s\n' "$body" | grep -c 'sleep "$FACTORY_CLOSE_PROBE"')" = 1 ] || return 1
	printf '%s\n' "$body" | grep -q 'if \[ -n "$opens" \]; then sleep "$FACTORY_CLOSE_PROBE"; fi'
}
claim_leg 'the probe waits once for the batch, not once per open ticket' cv_one_sleep

# --- the call site --------------------------------------------------------------------------------
cv_leg_call_site() {
	local here end stamped verify
	here=$(grep -n '^factory_merge() {' "$HERE/factory.sh" | cut -d: -f1)
	end=$(awk -v a="$here" 'NR>a && /^\}/ {print NR; exit}' "$HERE/factory.sh")
	stamped=$(awk -v a="$here" -v b="$end" \
		'NR>=a && NR<=b && /factory_ledger_now "pr:\$pr" merged-at/ {print NR; exit}' "$HERE/factory.sh")
	verify=$(awk -v a="$here" -v b="$end" \
		'NR>=a && NR<=b && /factory_verify_closure "\$pr"/ {print NR; exit}' "$HERE/factory.sh")
	[ -n "$stamped" ] && [ -n "$verify" ] && [ "$verify" -gt "$stamped" ]
}
claim_leg 'the call site sits in factory_merge, after the ledger stamp' cv_leg_call_site

# --- control: does this section bite? -------------------------------------------------------------
# Empty the shipped function's body and the open-ticket legs must go red. A section that stays green
# over a function that does nothing is not a section about that function.
cv_control() {
	local mut="$cv_work/fvc-empty.sh"
	awk '/^factory_verify_closure\(\) \{/ {print; print "\treturn 0"; skip=1; next}
	     skip && /^\}/ {skip=0; print; next}
	     skip {next} {print}' "$HERE/factory.sh" >"$mut" || return 1
	grep -q 'CLOSE-OPEN' "$HERE/factory.sh" || return 1
	if grep -q 'CLOSE-OPEN' "$mut"; then return 1; fi
	cv_reset
	cv_run "$mut" 77 >/dev/null 2>&1
	if grep -q 'CLOSE-OPEN' "$cv_work/journal"; then
		printf '\tthe emptied function still wrote CLOSE-OPEN, so the leg is not reading the function\n'
		return 1
	fi
	if [ -e "$cv_rec/301.body" ]; then
		printf '\tthe emptied function still wrote the note, so the note is not coming from it\n'
		return 1
	fi
	cv_reset
	cv_run "$cv_work/fvc.sh" 77 >/dev/null 2>&1
	grep -q 'CLOSE-OPEN' "$cv_work/journal" || {
		printf '\tthe shipped function never wrote CLOSE-OPEN, so the control proved nothing\n'
		return 1
	}
	return 0
}
claim_leg 'control: the legs die when the function body is emptied' cv_control

# --- the same rule, over the messages this branch carries (H-0166) --------------------------------
#
# A commit message is text GitHub parses, and until today only the body file was checked. The guard is
# `scripts/check-commit-verbs.sh`, and the reason it exists is this branch: `git log --format=%B
# origin/main..HEAD` at 17b37c7 answered two adjacent `-ing` forms in `8c9999b`, in the sentence that
# was warning about them. These legs answer the question that measurement raises -- does the guard bite,
# and does it stop where it is supposed to? Every one of them runs the SHIPPED script, by its real path,
# against a fixture repository, because the script reads the repository it is called from and nothing
# else. One repo per case: the verdict depends on which basis refs resolve, so `main`, `trunk` and "no
# basis at all" cannot share a directory.
cv_cmsg_repo() { # <dir> <branch> <message>... -- one commit per message, the first of them the basis
	local dir=$1 branch=$2 msg
	shift 2
	git init -q -b "$branch" "$dir" || return 1
	for msg in "$@"; do
		git -C "$dir" -c user.email=fixture@invalid -c user.name=fixture -c commit.gpgsign=false \
			commit -q --allow-empty -m "$msg" || return 1
	done
	return 0
}

cv_cmsg() { # <dir> [args]... -> the script's combined output; its exit status is the caller's $?
	local dir=$1
	shift
	(cd "$dir" && sh "$HERE/../scripts/check-commit-verbs.sh" "$@" 2>&1)
}

mkdir -p "$cv_work/cmsg"
cv_cmsg_repo "$cv_work/cmsg/claim" main 'the basis commit' 'so Closing #99 scanned as a closer' || true
cv_cmsg_repo "$cv_work/cmsg/keyword" main 'the basis commit' 'Closes #98 with the documented keyword' || true
cv_cmsg_repo "$cv_work/cmsg/waived" main 'the basis commit' 'so Closing #99 scanned as a closer' || true
cv_cmsg_repo "$cv_work/cmsg/stale" main 'the basis commit' 'so Closing #99 scanned as a closer' || true
cv_cmsg_repo "$cv_work/cmsg/nobasis" trunk 'the only commit, on a branch nothing names' || true
cv_cmsg_repo "$cv_work/cmsg/empty" main 'the basis commit' 'a second commit' || true
mkdir -p "$cv_work/cmsg/waived/factory" "$cv_work/cmsg/stale/factory"
printf '%s 99 the number the report prints, with a reason\n' \
	"$(git -C "$cv_work/cmsg/waived" rev-parse HEAD)" >"$cv_work/cmsg/waived/factory/close-verb-waivers"
printf '%s 98 the wrong number on purpose\n' \
	"$(git -C "$cv_work/cmsg/stale" rev-parse HEAD)" >"$cv_work/cmsg/stale/factory/close-verb-waivers"

# The wiring, in the shape of the file's own wiring leg: a guard that runs only when somebody remembers
# it is a guard whose evidence nobody can reproduce. The control deletes the step's line from a copy of
# the recipe and demands that the same scan then says no -- otherwise the pattern is matching some other
# line and the ok is the check being blind.
cv_cmsg_in_verify() { # <makefile> -> 0 when the `verify:` recipe invokes the script
	awk '/^verify:/{inr=1; next} inr && /^[^ \t]/{inr=0} inr' "$1" |
		grep -q 'scripts/check-commit-verbs\.sh'
}

cv_cmsg_wired() {
	local makefile=$HERE/../Makefile stripped=$cv_work/cmsg/Makefile
	if [ ! -f "$makefile" ]; then
		printf '\tno Makefile above %s, so nothing invokes the guard\n' "$HERE"
		return 1
	fi
	cv_cmsg_in_verify "$makefile" || {
		printf '\tnothing under `verify:` runs scripts/check-commit-verbs.sh\n'
		return 1
	}
	sed '/check-commit-verbs/d' "$makefile" >"$stripped"
	if cv_cmsg_in_verify "$stripped"; then
		printf '\tthe scan still finds the step after its line is deleted, so it matches something else\n'
		printf '\tin the recipe and the ok would mean nothing\n'
		return 1
	fi
	printf '\tinvoked from the verify recipe, and gone once its own line is deleted\n'
	return 0
}
claim_leg 'the commit-message guard is wired into make verify, and its line is what matches' \
	cv_cmsg_wired

cv_leg_cmsg_bites() {
	local out sha first
	out=$(cv_cmsg "$cv_work/cmsg/claim" HEAD~1) && {
		printf '\ta message carrying Closing #99 exited 0, so nothing is refused\n'
		return 1
	}
	sha=$(git -C "$cv_work/cmsg/claim" rev-parse --short HEAD)
	first=$(printf '%s\n' "$out" | head -n 1)
	printf '%s\n' "$out" | grep -q "$sha" || {
		printf '\tthe refusal does not name the commit that carries it: %s\n' "$first"
		return 1
	}
	printf '%s\n' "$out" | grep -q 'REFUSED, NOT A KEYWORD #99' || {
		printf '\tthe refusal does not name #99: %s\n' "$first"
		return 1
	}
	printf '%s\n' "$out" | grep -q "#99 is named by \`Closing #99\`" || {
		printf '\tthe refusal does not name the verb, so a writer could not fix it: %s\n' "$first"
		return 1
	}
	return 0
}
claim_leg 'a claim in a commit message is refused and names commit, number and verb' cv_leg_cmsg_bites

# The control in the other polarity, and the load-bearing one. Without it the leg above would pass on
# any rule that dislikes a number in a message -- including one broken enough to refuse every commit
# this repository has ever made. A documented keyword is a claim the parser certainly understands and a
# writer certainly means, so the guard has to let exactly that through.
cv_leg_cmsg_keyword() {
	cv_cmsg "$cv_work/cmsg/keyword" HEAD~1 >/dev/null || {
		printf '\tthe documented keyword was refused, so this is a ban on numbers, not on forms\n'
		return 1
	}
	return 0
}
claim_leg 'control: the documented keyword in a message is a claim that is allowed' cv_leg_cmsg_keyword

# The waiver, both ways. It is not a disable switch: it names the commit and the exact number set the
# report prints, and when it matches, the finding is reprinted rather than swallowed -- a suppressed
# adjudication is indistinguishable from no adjudication.
cv_leg_cmsg_waived() {
	local dir=$cv_work/cmsg/waived out
	out=$(cv_cmsg "$dir" HEAD~1) || {
		printf '\tthe exact waiver did not clear the run: %s\n' "$(printf '%s\n' "$out" | head -n 1)"
		return 1
	}
	printf '%s\n' "$out" | grep -q 'WAIVED' || {
		printf '\tthe waiver cleared the run without reprinting what it waived: %s\n' "$out"
		return 1
	}
	return 0
}
claim_leg 'an exact waiver clears the run and prints the adjudication' cv_leg_cmsg_waived

cv_leg_cmsg_stale() {
	local dir=$cv_work/cmsg/stale out
	out=$(cv_cmsg "$dir" HEAD~1) && {
		printf '\ta waiver naming #98 covered a report about #99\n'
		return 1
	}
	printf '%s\n' "$out" | grep -q 'stale record, not a shelter' || {
		printf '\tthe stale waiver failed without saying why: %s\n' "$(printf '%s\n' "$out" | head -n 1)"
		return 1
	}
	return 0
}
claim_leg 'a waiver naming the wrong number is a stale record, not a shelter' cv_leg_cmsg_stale

# Fails closed: a scan that could not run is not a pass. `trunk` with no `main` and no `origin/main` is
# the world where no basis resolves -- the repository a unit started on its own branch, or the shallow
# clone whose base ref was never fetched. Both read as green by a guard that defaults to silence.
cv_leg_cmsg_nobasis() {
	local dir=$cv_work/cmsg/nobasis out
	out=$(cv_cmsg "$dir") && {
		printf '\tthe run passed with no basis ref at all: %s\n' "$(printf '%s\n' "$out" | head -n 1)"
		return 1
	}
	printf '%s\n' "$out" | grep -q 'nothing was scanned' || {
		printf '\tthe run failed without saying nothing was scanned: %s\n' "$(printf '%s\n' "$out" | head -n 1)"
		return 1
	}
	return 0
}
claim_leg 'no basis ref fails, in the words nothing was scanned' cv_leg_cmsg_nobasis

# The other side of the same rule: an empty range IS nothing to scan, and says so, rather than printing
# a bare ok that a reader cannot tell from a scan that silently found nothing.
cv_leg_cmsg_empty() {
	local dir=$cv_work/cmsg/empty out
	out=$(cv_cmsg "$dir" HEAD) || {
		printf '\tan empty range failed: %s\n' "$(printf '%s\n' "$out" | head -n 1)"
		return 1
	}
	printf '%s\n' "$out" | grep -q '0 commits between' || {
		printf '\tan empty range did not say so in words: %s\n' "$out"
		return 1
	}
	return 0
}
claim_leg 'an empty range says there was nothing to read' cv_leg_cmsg_empty


# The tally names the world it was counted in, because it is not one number everywhere. H-0045
# measured the `core.excludesFile` leg printing TWO legs where git refuses a directory exclude file
# and ONE where it swallows it, and the question is answered by the filesystem under `$TMPDIR`, not
# by the repository: measured at f60c367 on one machine with only `TMPDIR` moved, 191 passed 0 failed
# under a tmpfs `$TMPDIR` and 190 passed 0 failed under a btrfs one, the leg's own line switching
# from `an exclude file git refused` to `an exclude file git swallowed`. So a tally quoted without its
# world is a number nobody can re-check, which is what this log has kept tripping over -- and a delta
# is safe where an absolute is not, because both ends of one move together. No `|| printf 'unknown'`
# behind either substitution: a fallback nobody can make fail is a branch nobody has seen run, and a
# blank field on this line is already the evidence that the command behind it failed.
printf 'factory/selftest.sh: %d passed, %d failed  (fixture root %s on %s, git %s)\n' \
	"$pass" "$fail" "$work" \
	"$(stat -f -c %T -- "$work")" \
	"$(git --version | sed 's/^git version //')"
reached_verdict=1
if [ "$fail" -ne 0 ]; then
	exit 1
fi
