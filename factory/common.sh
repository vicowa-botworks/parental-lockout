# shellcheck shell=bash disable=SC2016
# common.sh -- configuration, logging and the ledger shared by triage.sh and factory.sh.
#
# Sourced, never executed. Every entry point starts with `set -euo pipefail` itself so that
# running one directly cannot inherit a shell with the guards off.
#
# Why the split is three files and not one: triage is a pure read of GitHub and git and must be
# runnable on its own, by a human who wants to know why nothing is moving, and by the supervisor.
# A classifier that can only be reached through the thing it drives is not a diagnostic.

# --------------------------------------------------------------------------- configuration

# Every value is an environment override with a default, because the supervisor has to be
# startable from a systemd unit that carries no shell profile.
FACTORY_DIR=${FACTORY_DIR:-$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)}

# The repository, meaning its primary checkout. `git worktree list` prints the main working tree
# first, so a script committed inside the repo still resolves the same root from any linked tree.
# This matters twice over: gh resolves a repository from the *current directory*, and the trees
# directory below is derived from this path -- left as a linked worktree it would nest a `wt/`
# inside a `wt/` on every run from one.
_factory_main() {
	git -C "$FACTORY_DIR/.." worktree list --porcelain 2>/dev/null |
		awk '/^worktree /{print substr($0,10); exit}'
}
FACTORY_ROOT=${FACTORY_ROOT:-$(_factory_main)}
[ -n "$FACTORY_ROOT" ] || FACTORY_ROOT=$(cd -- "$FACTORY_DIR/.." && pwd)

# The tree that holds the harness itself -- `factory/`, `.opencode/opencode.jsonc` (where the
# `factory` agent and its `deny` rules are defined) and `.opencode/skills/factory/SKILL.md`. It is
# NOT the same directory as FACTORY_ROOT whenever the harness runs from a linked worktree, which is
# its entire life until this branch merges (D-0009).
#
# The distinction is what a tick starts or dies on. `opencode run` takes its project -- the agent,
# the agent's permission list, and the skills -- from `$PWD`, and chdir*s *there*: measured 5 Nov, a
# `$PWD` naming a directory that does not exist fails with `Error: Failed to change directory to
# <path>`, and a real `$PWD` beats the process's own cwd (`PWD=<primary checkout>` with cwd=/tmp
# resolved the primary checkout's agents, not /tmp's). With no `$PWD` in the environment the CLI
# never chdirs and the session opens in the *background server's* directory -- here `/home/rodney`,
# where `opencode-cli serve --service` was started -- which carries no `.opencode/` at all. A
# systemd unit sets a working directory and exports no `PWD`, so every tick died in under a second
# on `Error: Agent not found: "factory"`: 152 passes on 4-5 Nov, `exit=1`, nothing in the journal.
# The worse half of the same bug is what would have run had anything resolved out there: an agent from
# a directory outside the repository, with none of this project's config -- not `question: deny`, not
# the skill grants -- in force.
FACTORY_HARNESS=${FACTORY_HARNESS:-$(cd -- "$FACTORY_DIR/.." && pwd)}

# The repository, read from gh rather than hardcoded: a hardcoded owner/name is the bug
# watch-pr.sh refuses to carry -- guessing a repo means reporting somebody else's pull requests.
FACTORY_REPO=${FACTORY_REPO:-}
FACTORY_OWNER=${FACTORY_OWNER:-}
FACTORY_NAME=${FACTORY_NAME:-}

# The account that has approved every PR in this repository's history (measured 4 Nov: reviews
# on #195, #197, #201, #203, #206 all from hermesvicowa). Requesting a review is the only thing
# that ends a wait here, so the default is this repo's reviewer, and FACTORY_REVIEWER overrides
# it for any other repository.
FACTORY_REVIEWER=${FACTORY_REVIEWER:-hermesvicowa}

# Model invocation. `--agent factory` selects the project agent whose permissions deny
# `question`, which is what makes a headless tick unable to stall on an answer nobody will give.
FACTORY_OPENCODE=${FACTORY_OPENCODE:-opencode}
FACTORY_AGENT=${FACTORY_AGENT:-factory}
# A tick is one unit of work: build, test, push, PR, log. 45 minutes is generous for the
# slowest thing a unit does here (`make verify` measures 3m9s, and a fresh module cache on a
# cold worktree is the outlier), and it is the ceiling on how long a wedged session can hold
# the lock.
FACTORY_TICK_TIMEOUT=${FACTORY_TICK_TIMEOUT:-2700}
FACTORY_MAX_TICKS=${FACTORY_MAX_TICKS:-3}
# Supervisor wall-clock budget for one invocation, so a systemd `oneshot` cannot overlap the
# next activation of its own timer.
FACTORY_BUDGET=${FACTORY_BUDGET:-5400}
# How long the pass's one `git fetch` (H-0075) may hold the lock before it is called a hung
# connection and abandoned. Steady state is not the case this bounds: measured 9 Nov on this clone,
# a no-op `git fetch --prune origin` over 9 remote refs costs 0.68s. It bounds the pathological one,
# where the remote accepts the connection and says nothing, and #879 is the precedent for what a
# read that parks costs. A fifth of the timer's 600s cadence, so the worst case the bound allows is
# one activation that finds the lock held and exits 7 -- and the pass continues either way, because
# a queue that stops running is indistinguishable from a queue with nothing in it.
FACTORY_FETCH_TIMEOUT=${FACTORY_FETCH_TIMEOUT:-120}

# Where per-unit worktrees live. Deliberately outside /tmp: six abandoned trees were found in
# /tmp/opencode on 4 Nov, and a tree that a reboot can delete can also delete unmerged work.
# Derived from the *primary* checkout, not from FACTORY_ROOT. The two differ whenever the harness runs
# from a worktree, and deriving it from FACTORY_ROOT nests the trees inside it --
# `/home/rodney/prj/wt/wt`, which is what happened on the first timer pass and which nothing would
# ever prune. `--absolute-git-dir` answers the shared `.git`, whose parent directory names the primary
# checkout even when the caller is standing in a linked worktree.
factory_trees_default() {
	local gitdir primary
	# `--git-common-dir`, not `--absolute-git-dir`: from a linked worktree the latter answers
	# `…/.git/worktrees/<name>`, which does not end in `/.git`, so the case below fell through and the
	# nesting this exists to prevent came back unchanged. The common directory is the primary checkout's
	# `.git` no matter which worktree the caller is standing in.
	gitdir=$(git -C "$FACTORY_ROOT" rev-parse --path-format=absolute --git-common-dir 2>/dev/null ||
		git -C "$FACTORY_ROOT" rev-parse --git-common-dir 2>/dev/null |
			sed -e "s#^\.\./\.\.#${FACTORY_ROOT}#" || printf '')
	case "$gitdir" in
	*/.git) primary=${gitdir%/.git} ;;
	*) primary=$FACTORY_ROOT ;;
	esac
	printf '%s\n' "$(dirname -- "$primary")/wt"
}

FACTORY_TREES=${FACTORY_TREES:-$(factory_trees_default)}

# Where the supervisor keeps the only copy of anything it is about to stop holding: a dirty tree it is
# quarantining (#248), a branch no pull request carries (#247). One box for both rescues so a person
# has one place to look, and it is derived from the same parent as the trees directory -- so it lands
# beside them, `/home/rodney/prj/at-risk` on this box, and never *under* them: `git worktree remove`
# and a prune of `$FACTORY_TREES` must never be able to take the mirror with them.
#
# `/tmp` is refused, not derived into. The trees' parent is wherever an operator pointed
# FACTORY_TREES, and `/tmp/opencode/wt-204` and `wt-206` are already gone that way: a box a reboot can
# delete cannot be the surviving half of anything, which is the sentence #248 paid for and the reason
# this does not simply inherit the trees' location. So a derived path under `/tmp`, a relative path,
# and an empty answer all fall back to the home box rather than carrying the mistake into the rescue.
factory_at_risk_default() {
	local trees=${FACTORY_TREES:-$(factory_trees_default)} derived
	derived=$(dirname -- "${trees%/}")/at-risk
	case ${derived%/} in
	'' | . | .. | ./* | ../* | /tmp | /tmp/*) printf '%s\n' "$HOME/prj/at-risk" ;;
	*) printf '%s\n' "$derived" ;;
	esac
}

FACTORY_AT_RISK=${FACTORY_AT_RISK:-$(factory_at_risk_default)}

# Cadences, seconds. A unit waiting on a reviewer is re-poked at most this often; the timer
# itself may fire far more often, because a wait-state tick spends no model tokens.
FACTORY_REVIEW_POKES=${FACTORY_REVIEW_POKES:-1800}
# After this long in any non-progress state, one comment with the evidence goes on the PR. It is
# a notice, not a park: the unit stays in the queue and keeps being retried.
FACTORY_ESCALATE_AFTER=${FACTORY_ESCALATE_AFTER:-86400}

# A halt is more urgent than a wait: a wait costs one unit its turn, a halt costs every unit the
# queue. So the halt speaks on its own clock, an hour rather than a day, and it speaks on the issue
# or pull request the halted unit names rather than a bulletin nobody subscribes to (issue #248:
# 115 `HALTED` lines and 15h57m of frozen queue that only a person opening the journal could see).
FACTORY_HALT_NOTICE_AFTER=${FACTORY_HALT_NOTICE_AFTER:-3600}

# Longer than this with no pass at all, and `--status` calls it STALLED. The timer's own cadence is
# `OnUnitInactiveSec=600`, so one hour is roughly five missed activations: enough that a single slow
# pass is not reported as a stall, short enough that tonight's 91-minute freeze (#246, a leaked
# grandchild holding the cgroup while the timer read `active`) is.
FACTORY_STALL_SECS=${FACTORY_STALL_SECS:-3600}

# The timer this supervisor is triggered by, named rather than derived: `--status` has to quote what
# systemd will do next, and the unit's name is the one fact the script cannot read out of its own
# state -- it is decided by whoever ran `install-timer.sh`.
FACTORY_TIMER_UNIT=${FACTORY_TIMER_UNIT:-parental-lockout-factory.timer}

# How far back the `delivered` verdict looks: one `gh pr list --state merged --limit N` per pass, so
# the bound is what stops the read growing with the repository's age. It is a bound on the *read*,
# not on the claim -- an issue whose fix landed outside the window keeps the verdict it always had,
# `unclaimed`, so a window that is too small costs a wasted unit and a window that is too big costs
# nothing but a payload. Measured 8 Nov on this repository: 125 pull requests have merged, and a
# window of 100 reaches back to #52 at `2026-10-15T08:38:06Z`, so the twenty-five it does not read are
# the harness's first fortnight -- every issue the queue has delivered since 15 October is inside it.
#
# What the bound really cuts is not time. `gh pr list --state merged` answers newest-CREATED first,
# not newest-merged -- measured 8 Nov the first eight rows are 234, 231, 229, 227, 226, 221, 218, 217
# and their merge dates are 8, 8, 8, 8, 6, 6, 6, 8 November -- so a branch that lived a long time
# before it merged sits lower in the window than its merge date suggests. A window of seven would
# already be blind to #217, which merged the same day as #234. `delivered` prints the window it leaned
# on in its own row for exactly that reason: a bound that is not printed is a verdict nobody can
# re-check.
FACTORY_DELIVERED_WINDOW=${FACTORY_DELIVERED_WINDOW:-100}

# The base branch a merged pull request has to have landed on for its claim to mean the work is on
# the trunk. A pull request merged into another open pull request's branch is that branch's news, not
# `main`'s, and #216's stacked-base chain is the shape this refuses: reporting an issue delivered by a
# merge nobody can deploy is the false green this class exists to prevent.
FACTORY_BASE=${FACTORY_BASE:-main}

# The opening token of the refused-claim note `factory/triage.sh` puts in front of an `unclaimed`
# row's detail when a MERGED pull request names the issue and the sole-claim/`Part of` arms refuse it
# (#277). It is a shared literal rather than a sentence each side invents, because two files read it:
# the classifier writes it as the first characters of the detail, and `factory/factory.sh`'s `unclaimed`
# arm matches it at the START of the detail to decide whether to add the instruction that tells the
# tick what the bracket means. Anchored, not `*substring*`, because the rest of that column is an
# issue title somebody typed -- text this harness does not control -- and a title that happened to
# contain the token must not buy itself an instruction. `factory/selftest.sh` pins both ends: a row
# that reaches the table with the note, and a copy of `factory.sh` with the arm deleted.
FACTORY_MERGED_MARK=${FACTORY_MERGED_MARK:-'[merged:PR#'}

FACTORY_LOG=${FACTORY_LOG:-$FACTORY_DIR/log}
FACTORY_LEDGER=${FACTORY_LEDGER:-$FACTORY_DIR/ledger}
FACTORY_RUN=${FACTORY_RUN:-$FACTORY_DIR/run}
FACTORY_HALT=${FACTORY_HALT:-$FACTORY_DIR/HALT}

# The box named above is the one this file's own rescue writes into: `factory_tree_quarantine` mirrors
# a dirty tree there as a patch (#248) and `factory_preserve_unpushed` mirrors a branch there as a
# bundle (#247). It is deliberately not re-derived here -- two definitions of one variable is how one
# of the two intents silently stops applying, which is what the merge of #247 into #248's tree had to
# resolve. `~/prj/at-risk/` is the surviving half of `wt-204` and `wt-206` for exactly this reason.

# The installed unit, as a path. Every sentence this harness used to say about what the timer runs was
# composed from `$FACTORY_ROOT` -- `install-timer.sh` renders `ExecStart=@REPO@/factory/factory.sh` and
# then asserted the result -- and on 10 Nov that assertion was false about the unit that was installed
# (#260, and the functions below). So the unit is now asked, which needs to know which file it is:
#
#   FACTORY_SERVICE_UNIT   the unit's own file name. The generated unit exports this about itself, and
#                          it is the only answer that survives `install-timer.sh --name other`, because
#                          a name chosen on a command line is recorded nowhere a later pass can read.
#                          Units installed before #260 have no such line; the fallback in
#                          `factory_service_unit` then names the same file the default install wrote.
#   FACTORY_UNITS_DIR      an OVERRIDE for where user units live, empty by default. Empty rather than
#                          pre-derived because the derivation belongs to `factory_units_dir` alone:
#                          composing `${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user` here as well is
#                          two definitions of one path, the class of defect this whole file keeps
#                          being edited for (H-0077). It exists so `factory/selftest.sh` can plant a
#                          unit without reaching into the operator's `~/.config/systemd/user`.
FACTORY_SERVICE_UNIT=${FACTORY_SERVICE_UNIT:-}
FACTORY_UNITS_DIR=${FACTORY_UNITS_DIR:-}

# The exit code a halted supervisor leaves behind. It is a code the *machine* reads: the generated unit
# names it in `SuccessExitStatus=`, so the two files have to agree and one of them has to own it.
# shellcheck disable=SC2034  # read by factory.sh and install-timer.sh, both of which source this
# file: shellcheck analyses common.sh alone, so a constant that both consumers need looks dead here.
# The alternative -- duplicating the 8 in each -- is the drift this constant exists to prevent.
FACTORY_EXIT_HALT=8
DRY_RUN=${DRY_RUN:-0}

# --------------------------------------------------------------------------- directories

factory_paths() {
	mkdir -p -- "$FACTORY_LOG" "$FACTORY_LEDGER" "$FACTORY_RUN"
}

# --------------------------------------------------------------------------- logging

log() { printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }

# The supervisor's own journal. Everything a tick did goes to its own file; this records the
# decisions the *supervisor* made, which is what anyone reading a stall needs first.
log_event() {
	factory_paths
	log "$*" >>"$FACTORY_LOG/supervisor.log"
	log "$*"
}

die() {
	printf 'factory: %s\n' "$*" >&2
	exit 1
}

# --------------------------------------------------------------------------- halt switch

# Checked by the supervisor between units, and by the agent at every step boundary through the
# skill. Stopping between units is not enough on its own: a tick can hold a worktree for
# 45 minutes, and the only thing inside that window that can hear you is the agent itself.
factory_halted() { [ -e "$FACTORY_HALT" ]; }

factory_halt_reason() {
	if [ -e "$FACTORY_HALT" ]; then
		tr -d '\n' <"$FACTORY_HALT"
	else
		printf ''
	fi
}

# Seconds since a file was last touched; "never" when it does not exist, so a missing journal reads as
# the oldest possible thing rather than as zero.
factory_file_age() { # path -> seconds
	local m now
	now=$(date -u +%s)
	m=$(stat -c %Y -- "$1" 2>/dev/null || printf '0\n')
	if [ "$m" = 0 ]; then
		printf '999999999\n'
	else
		printf '%s\n' "$((now > m ? now - m : 0))"
	fi
}

# Seconds since the supervisor's journal last wrote a line carrying MARK -- the stamp is the line's own
# first token, because `log()` writes it -- so the answer comes from one grep over the journal rather
# than from anything that has to remember to be updated. Huge when no such line exists.
factory_journal_age() { # mark -> seconds
	local line="" then_ts=0 now=0
	line=$(grep -F -- "$1" "$FACTORY_LOG/supervisor.log" 2>/dev/null | tail -n 1)
	if [ -z "$line" ]; then
		printf '999999999\n'
		return 0
	fi
	then_ts=$(date -u -d "${line%% *}" +%s 2>/dev/null || printf '0\n')
	if [ "$then_ts" = 0 ]; then
		printf '999999999\n'
		return 0
	fi
	now=$(date -u +%s)
	printf '%s\n' "$((now > then_ts ? now - then_ts : 0))"
}

# The same durations the journal prints as stamps, in the shape a sentence wants: `15h57m`, `3m12s`.
# A raw 57420 is a number to divide; the complaint that starts an outage is "how long has it been?".
factory_age_human() { # seconds -> 4d2h | 15h57m | 3m12s | 42s | never
	local s=${1:-0}
	case "$s" in
	('' | *[!0-9]*) s=0 ;;
	esac
	if [ "$s" -ge 999999999 ]; then
		printf 'never\n'
	elif [ "$s" -ge 86400 ]; then
		printf '%dd%dh\n' "$((s / 86400))" "$(((s % 86400) / 3600))"
	elif [ "$s" -ge 3600 ]; then
		printf '%dh%02dm\n' "$((s / 3600))" "$(((s % 3600) / 60))"
	elif [ "$s" -ge 60 ]; then
		printf '%dm%02ds\n' "$((s / 60))" "$((s % 60))"
	else
		printf '%ds\n' "$s"
	fi
}

# Longer than this between a pass's last line and its unit going inactive, and the teardown is a hold
# rather than a reaping. Nothing legitimate runs in that window: the shipped unit has no `ExecStop=`,
# so the only thing systemd can be waiting for is a descendant that will not die. Measured on this box
# 11 Nov: a `Type=oneshot` whose main process exits goes inactive 0.3 ms after it exits, while the same
# unit carrying one `trap '' TERM` descendant sits in `stop-sigterm` for the whole stop deadline --
# 60.194 s against `TimeoutStopSec=60`, 20.135 s against 20. Five seconds separates the two cases by
# three orders of magnitude and a clean pass cannot reach it.
FACTORY_REAP_NOTICE_SECS=${FACTORY_REAP_NOTICE_SECS:-5}

# How many of the unit's journal lines the teardown read looks back over. The tail has to reach past
# everything the newest activation has written in order to get at the previous activation's terminal
# line: systemd spends two lines on an activation that reaps cleanly and eleven on one that escalates
# (it names every process it SIGKILLs), and a pass writes its own progress as it goes -- a few dozen
# lines, because a tick's chattiness goes to its own log file. 400 is several passes deep. A window
# too small to find a terminal line is reported as unknown, never as a clean read.
FACTORY_REAP_TAIL=${FACTORY_REAP_TAIL:-400}

# What systemd says it will do next, on one line. The journal alone is not enough, twice over: on 8
# Nov a unit sat `deactivating` for 91 minutes while the timer printed `active` (#246), and after the
# 6 Nov reboot the timer read `active` with `SubState=elapsed` and would not fire until something
# inactivated it first. Either one is invisible to a reader who only has the log.
factory_timer_state() {
	local out=""
	out=$(systemctl --user show "$FACTORY_TIMER_UNIT" -p ActiveState -p SubState -p TriggerNextElapseUSecRealtime 2>/dev/null |
		sed -n 's/^[A-Za-z]*=//p' | paste -sd' ' -) || out=""
	if [ -z "$out" ]; then
		printf 'no %s visible to this user\n' "$FACTORY_TIMER_UNIT"
	else
		printf '%s\n' "$out"
	fi
	return 0
}

# The stored halt, as one line -- and the one place that line is composed.
#
# Why the age belongs in it. `factory/HALT` is written once and afterwards merely read: the first
# thing a halted pass does is print the file's sentence and exit (`factory.sh`'s top-of-pass guard),
# so every activation after the detection prints exactly what the detection printed. Measured on
# this branch against the shipped scripts (#237): one marker with an 8-hour-old mtime, three passes
# at 03:37:54Z, 03:37:57Z and 03:37:59Z, and three `HALTED` lines byte-identical once their own
# stamps are stripped -- nothing in the line said the cause was eight hours old rather than three
# seconds old. The 8 Nov outage in `~/prj/wt/issue-207` is the same reading at scale: the marker was
# written at 04:30:02Z and the journal repeated the identical line at 14:09:09, 14:20:09, 14:31:01
# and 14:42:09, and #248 counted 115 of them across 15h57m.
#
# The age is the fact that tells those prints apart, and it is free: the marker's mtime IS the
# instant the cause was seen, because the only thing that writes this file is the guard that has
# just seen the cause. The age is spoken by `factory_age_human` -- the same unit every other elapsed
# figure in this journal uses (`15h57m`, `CONFLICT-SKIPPED attempted ${age}s ago`) and #248's own
# verdict -- so the halt cannot grow two formats for one fact. Asking for it goes through `stat`
# rather than `factory_file_age`, and the reason is measured below rather than stylistic.
#
# What it deliberately does not do is ask the tree again.
#
#   - The content verdict's first read is `git update-index --refresh`, measured (see the comment
#     above `factory_tree_dirt`) to rewrite `.git/index`'s stat entries. Re-probing on the halted
#     path would therefore write into the one tree the halt sentence swears the supervisor will not
#     touch.
#   - The four reads that do not write have a measured false-clean direction of their own -- the
#     `naive:` column in `factory/selftest.sh` says CLEAN where the content verdict says DIRTY -- so
#     a report-only verdict assembled from them would tell a person to `rm factory/HALT` over work it
#     cannot see.
#     And a verdict that self-cleared the marker would resume the queue, with nobody in the loop,
#     over a state the supervisor itself declared somebody else's.
#
# So the marker stays sticky, the line stops pretending to be new, and all three readers of the
# marker -- `factory_verdict`, `factory.sh --status`, and the journal's `HALTED` line -- print this
# one. `factory/selftest.sh` refuses a shipped script that prints the stored sentence without it.
factory_halt_report() { # -> 'cause recorded <age> ago, and not re-probed since: <sentence>'
	local reason mtime now
	reason=$(factory_halt_reason)
	# One `stat`, deliberately not `factory_file_age`. The unit stays the shared one -- every age here
	# is still spoken by `factory_age_human` -- but that primitive's answer is
	# `$((now > m ? now - m : 0))`, and the clamp is a lie on this particular line. Measured on this
	# branch: a marker planted 7200s ahead of this clock printed `cause recorded 0s ago`, the verbatim
	# sentence a detection three seconds old prints. A clock stepped backwards -- an NTP correction
	# after a suspend, a VM restored to a snapshot -- puts every marker written before it in that state
	# at once, which is #237's defect with the sign flipped: a line that says the opposite of what
	# happened, at scale. `#248`'s stall verdicts want that clamp, because `STALLED` asks only whether
	# the journal is old and a future stamp is not old. This line asks how old the cause is.
	mtime=$(stat -c %Y -- "$FACTORY_HALT" 2>/dev/null) || mtime=''
	case "$mtime" in
	'' | *[!0-9]*)
		# A time that is merely absent is the bug this line exists to fix: no age is what a fresh
		# detection looks like. Reachable for real when the marker is removed between
		# `factory_halted()` and this read. Say the time could not be read, visibly.
		printf 'cause recorded at a time that could not be read, and not re-probed since: %s' "$reason"
		return 0
		;;
	esac
	now=$(date -u +%s)
	if [ "$mtime" -gt "$now" ]; then
		printf 'cause recorded %s ahead of this clock, so its age is unknown, and not re-probed since: %s' \
			"$(factory_age_human "$((mtime - now))")" "$reason"
		return 0
	fi
	printf 'cause recorded %s ago, and not re-probed since: %s' \
		"$(factory_age_human "$((now - mtime))")" "$reason"
}

# systemd's own lines about this unit, plus the unit's own lines, oldest first, newest `FACTORY_REAP_TAIL`
# of them. Both are needed and neither alone is enough. The systemd lines are the only account of the
# stop path -- `State 'stop-sigterm' timed out. Killing.` is the sentence that says the queue was held
# open. The unit's own lines are the only record of when the pass's WORK stopped: systemd logs nothing
# at all when an oneshot exits 0, and the 11 Nov incident at 00:56 was exactly that -- `END rc=0` at
# 00:56:06Z, then a silent minute, then the escalation -- so a read filtered to systemd's identifier
# alone reports that 60-second hold as 0s.
factory_reap_lines() {
	# shellcheck disable=SC2312  # the unit name is a lookup, and a failing lookup prints nothing here
	journalctl --user --no-pager -u "$(factory_service_unit)" -t systemd -o short-iso -n "$FACTORY_REAP_TAIL" 2>/dev/null
}


# The live state of the unit, the way `factory_timer_state` gives the live state of the timer. This is
# the half that can see a hold while it is happening -- `SubState=stop-sigterm` with `TasksCurrent=10`
# and `MainPID=0` is the 8 Nov outage in three fields. It is an addition and never a requirement: the
# bus may be unreachable, and on the times that matter a tick is not allowed to ask the manager at all.
factory_unit_state() {
	local out=""
	out=$(systemctl --user show "$(factory_service_unit)" -p ActiveState -p SubState -p TasksCurrent -p Result 2>/dev/null |
		sed -n 's/^[A-Za-z]*=//p' | paste -sd' ' -) || out=""
	printf '%s\n' "$out"
	return 0
}

# Reads `factory_reap_lines` back and says what the last COMPLETED activation did -- completed, because
# the newest block is usually the pass running right now, which has no verdict in it yet.
#
#   stdout  <hold>TAB<how>TAB<age>TAB<open>
#             hold   seconds from the main process exiting to the unit going inactive, whole seconds
#             how    clean | deadline (the stop deadline expired) | client (a hand-escalated SIGKILL)
#                    | unexplained (it was held and nothing in the log says why)
#             age    seconds since that teardown
#             open   1 when a newer activation has started and not yet finished
#   exit 1  no terminal line in the tail: nothing finished inside the window, or there is no journal.
#           Unknown is its own answer (#228's polarity -- a read that failed is not the clean state).
factory_reap_read() {
	local -a msgs=() stamps=()
	local line="" stamp="" msg=""
	while IFS= read -r line || [ -n "$line" ]; do
		[ -n "$line" ] || continue
		stamp=${line%% *}
		case $line in
		(*"]: "*) msg=${line#*]: } ;;
		(*) continue ;;
		esac
		msgs+=("$msg")
		stamps+=("$stamp")
	done
# The pass's last words: the newest line written by a process of the unit at or before the moment its
# activation ended. Read separately, and read short, because a bounded tail of the whole stream is not
# bounded in TIME: a pass 25 minutes old had already written past any line-count window, so a fixed
# `-n` would find the terminal line only on a machine where the last pass said nothing. `-n 25` reaches
# back over the handful of systemd lines that sit between the last stdout line and the terminal one.
factory_reap_lastword() { # <terminal stamp> -> the stamp of the newest process line at or before it
	local until=$1 out=''
	[ -n "$until" ] || return 1
	out=$(journalctl --user --no-pager -u "$(factory_service_unit)" -o short-iso --until "$until" -n 25 2>/dev/null |
		awk '
			$0 !~ / systemd\([0-9]+\): / { last = $1 }
			END { if (last != "") print last }
		') || out=''
	[ -n "$out" ] || return 1
	printf '%s\n' "$out"
}

	local unit="" i=0 msg2="" hold=0 how='' sig='' work='' term='' open=0 t=0 w=0 now=0
	unit=$(factory_service_unit)
	for ((i = ${#msgs[@]}; i > 0; i--)); do
		msg2=${msgs[i - 1]}
		stamp=${stamps[i - 1]}
		case $msg2 in
		"Starting $unit"*)
			if [ -n "$term" ]; then
				break
			fi
			open=1
			;;
		"Main process exited, "* | "$unit: Main process exited, "*)
			[ -n "$work" ] || work=$stamp
			;;
		(*"timed out. Killing."*)
			sig=deadline
			;;
		(*"Failed with result 'timeout'."*)
			[ -n "$sig" ] || sig=deadline
			;;
		(*"with SIGKILL on client request."*)
			sig=client
			;;
		("Failed to start $unit "* | "Finished $unit "* | "Stopped $unit "* | *"Deactivated." | *"Deactivated after execution.")
			[ -n "$term" ] || term=$stamp
			;;
		esac
		# Anything the activation itself wrote, once the terminal line has been passed walking
		# backwards, is the pass's last words: the newest one first, so the first one met wins. It
		# predates the main process's actual exit by whatever the pass did without logging on its way
		# out, which makes the hold an overestimate -- the conservative direction for an alarm. A line
		# from the activation running right now is met before any terminal line, so it is not counted.
	done
	[ -n "$term" ] || return 1
	t=$(date -u -d "$term" +%s 2>/dev/null) || return 1
	if [ -z "$work" ]; then
		work=$(factory_reap_lastword "$term") || work=$term
	fi
	w=$(date -u -d "$work" +%s 2>/dev/null) || w=$t
	hold=$((t > w ? t - w : 0))
	now=$(date -u +%s)
	case $sig in
	(deadline | client) how=$sig ;;
	(*)
		if [ "$hold" -gt "$FACTORY_REAP_NOTICE_SECS" ]; then
			how=unexplained
		else
			how=clean
		fi
		;;
	esac
	printf '%s\t%s\t%s\t%s\n' "$hold" "$how" "$((now > t ? now - t : 0))" "$open"
}

# The `reaping:` line of `factory.sh --status`, in one sentence: what the last pass cost the queue to
# tear down, and what the unit is doing right now. Both halves are needed and neither is enough. The
# journal half is the only thing that survives a fast hold -- 60 s between activations that are 10
# minutes apart is a window a person almost never lands in, and on 8 Nov the whole 1 h 07 min freeze
# was visible only as a substate nobody had open. The live half is the only thing that sees a hold in
# progress, and it degrades to nothing when the bus is unreachable, which is when the journal half is
# read anyway.
factory_reap_line() {
	local raw='' hold='' how='' age='' open='' live=''
	raw=$(factory_reap_lines | factory_reap_read) || raw=''
	live=$(factory_unit_state)
	if [ -z "$raw" ]; then
		printf 'unknown -- journalctl --user has no finished activation for %s to read%s\n' \
			"$(factory_service_unit)" "${live:+; manager says: $live}"
		return 0
	fi
	IFS=$'\t' read -r hold how age open <<EOF
$raw
EOF
	case $how in
	clean)
		printf 'reaped in %s; the unit has been inactive for %s' \
			"$(factory_age_human "$hold")" "$(factory_age_human "$age")"
		;;
	deadline)
		printf 'HELD %s to the stop deadline -- a descendant of the pass ignored SIGTERM (systemd: "State '"'"'stop-sigterm'"'"' timed out. Killing."), and %s counts from the unit going INACTIVE, so the queue could not re-arm for all of it. Ended %s ago.' \
			"$(factory_age_human "$hold")" "$FACTORY_TIMER_UNIT" "$(factory_age_human "$age")"
		;;
	client)
		printf 'HELD %s, ended by a SIGKILL somebody asked for by hand -- nothing reaped it on its own, so the queue was frozen for all of it. Ended %s ago; `systemctl --user reset-failed %s` clears the failed reading.' \
			"$(factory_age_human "$hold")" "$(factory_age_human "$age")" "$(factory_service_unit)"
		;;
	*)
		printf 'HELD %s with nothing in the journal saying why. Ended %s ago.' \
			"$(factory_age_human "$hold")" "$(factory_age_human "$age")"
		;;
	esac
	[ "$open" = "1" ] && printf '; a pass is running now'
	[ -n "$live" ] && printf ' | live: %s %s' "$(factory_service_unit)" "$live"
	printf '\n'
}

# One verdict, so "is the factory alive?" is a command and not a diff between a journal timestamp and
# `date`. Issue #248 is why it exists: 115 `HALTED` lines across 15h57m, every one of them true, and
# nothing in the system that could be asked the question or would notice the answer.
#
#   HALTED   factory/HALT exists, with how long it has been there and its own sentence.
#   WORKING  a tick started inside FACTORY_STALL_SECS.
#   IDLE     the journal is younger than the window and no tick is running: every unit is waiting on a
#            review, a check or a conflict. That is a queue with nothing to do, not a broken queue.
#   STALLED  the journal itself is older than the window, or missing. The only verdict that means go.
#
# It reads local state only -- a marker, a file, one systemctl query -- because the times it matters
# most are the times GitHub may not be reachable either.
factory_verdict() {
	local tick pass
	if factory_halted; then
		printf 'HALTED %s\n' "$(factory_halt_report)"
		return 0
	fi
	tick=$(factory_journal_age 'TICK-START ')
	pass=$(factory_file_age "$FACTORY_LOG/supervisor.log")
	if [ "$tick" -le "$FACTORY_STALL_SECS" ]; then
		printf 'WORKING last tick started %s ago\n' "$(factory_age_human "$tick")"
	elif [ "$pass" -le "$FACTORY_STALL_SECS" ]; then
		printf 'IDLE journal last written %s ago, no tick started: every unit is waiting on a review, a check or a conflict\n' \
			"$(factory_age_human "$pass")"
	else
		printf 'STALLED nothing written to the journal for %s; timer: %s\n' \
			"$(factory_age_human "$pass")" "$(factory_timer_state)"
	fi
}

# --------------------------------------------------------------------------- ledger
#
# One file per (unit, fact), content is a single line. Not one JSON document, on purpose: two
# writers appending to one JSON file is a corrupt file, and the supervisor and a tick are both
# writers. `cat` and `date` are the whole interface, so nothing has to parse to be right.
#
# A ledger file is a timestamp with consequences: `first-seen` is what makes a unit old enough to be
# picked first, `stuck-since` starts the clock on the escalation comment, `last-notice` is what stops
# the supervisor commenting at a maintainer every ten minutes. So `--dry-run` may not write one. It
# used to: the pass printed `DRY-RUN` for every real action and then stamped the ledger anyway, so a
# dry run started the stuck-clock and refreshed the notice throttle, and the next real pass quoted
# `after 86400s` for a wait that had only ever been rehearsed on paper. "Print, don't do" cannot be
# partial, because the half that is done is the half that changes what a later run decides.
# (Reported by review on #217, 5 Nov; `log_event` stays real -- a dry run that records nothing is a
# dry run nobody can audit, and the journal is the one artefact worth reading afterwards.)

factory_ledger_path() { printf '%s/%s.%s\n' "$FACTORY_LEDGER" "$1" "$2"; }

factory_ledger_get() {
	local f
	f=$(factory_ledger_path "$1" "$2")
	[ -f "$f" ] && tr -d '\n' <"$f"
	return 0
}

factory_ledger_set() {
	[ "$DRY_RUN" = "1" ] && return 0
	factory_paths
	printf '%s\n' "$3" >"$(factory_ledger_path "$1" "$2")"
}

factory_ledger_now() { factory_ledger_set "$1" "$2" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"; }

factory_ledger_clear() {
	[ "$DRY_RUN" = "1" ] && return 0
	rm -f -- "$(factory_ledger_path "$1" "$2")"
}

factory_ledger_int() {
	local v
	v=$(factory_ledger_get "$1" "$2")
	case "$v" in ('' | *[!0-9]*) printf '0\n' ;; (*) printf '%s\n' "$v" ;; esac
}

# Seconds since a ledger timestamp; huge when there is no timestamp, so "never" means "ready".
factory_age() {
	local v now then_ts
	v=$(factory_ledger_get "$1" "$2")
	if [ -z "$v" ]; then
		printf '999999999\n'
		return 0
	fi
	now=$(date -u +%s)
	then_ts=$(date -u -d "$v" +%s 2>/dev/null || printf '0\n')
	printf '%s\n' "$((now > then_ts ? now - then_ts : 0))"
}

# --------------------------------------------------------------------------- GitHub reads
#
# Every read has the same shape and the same hazard: `gh api` writes a `{"message":...}` body to
# stdout when a request 404s, and a capture written as `v=$(gh api … || printf '')` keeps that
# body as the value. It then flows into a verdict as if it were data -- which is how a wrong
# repository name came to look like three pull requests that nobody had reviewed. So the failure
# and the error body are both turned into the caller's default, and a default is the only thing
# that may reach a verdict.

factory_valid_repo() {
	case "$1" in
	*/*) : ;;
	*) return 1 ;;
	esac
	case "$1" in */*/*) return 1 ;; esac
	case "${1%%/*}" in '' | *[!A-Za-z0-9._-]*) return 1 ;; esac
	case "${1##*/}" in '' | *[!A-Za-z0-9._-]*) return 1 ;; esac
	return 0
}

# api_get <path> <jq> <default>
api_get() {
	local out
	if ! out=$(gh api "repos/$FACTORY_OWNER/$FACTORY_NAME/$1" --jq "$2" 2>/dev/null); then
		printf '%s' "$3"
		return 0
	fi
	case "$out" in
	'{"message"'* | '{"errors"'*) printf '%s' "$3" ;;
	(*) printf '%s' "$out" ;;
	esac
}

# ------------------------------------------------------------------ the reviewer's ask
#
# Whether a change request's body is an ask anybody could answer, spelled once here because TWO
# readers ask the same question and they must not be able to disagree about the answer: `triage.sh`
# needs it to choose between spending a tick on a pull request and reporting it, and `factory.sh`
# needs it to say the same sentence on the pull request. Two predicates would drift exactly the way
# the two claim channels in triage.sh would have if `CLAIM_RE` were not spelled once (#1256), and
# the drift is invisible until a tick has been dispatched at nothing.
#
# Why there is a question at all (#242). The REST projection that fed the classifier kept three
# fields out of every review row -- user, state, submitted_at -- and dropped the body. So the unit
# prompt a tick received read `review asked for changes at <ts>, after the head commit at <ts>`: a
# verdict with no ask inside it, and a tick expected to go and find the ask on its own. On #240,
# 8 Nov, it did not exist. The newest `CHANGES_REQUESTED` body was **21 bytes, the literal text
# `@/tmp/pr240-review.md`** -- `gh` reads a file only for `--body-file`, so a reviewer's `--body
# @/tmp/…` stored the reference instead of the review -- and the worktree, the ceiling and the model
# were spent on a review that had never been uploaded. A tick cannot tell "the reviewer asked for
# nothing" from "the reviewer asked for something I cannot see", and a tick that guesses is the
# failure mode this repository keeps writing up: `D-0046` is five invented citations in three
# published artifacts.
#
# The floor is measured, not picked. Over the 200 most recent pull requests here (read 8 Nov with
# `gh pr list --state all --limit 200 --json number,reviews`, 86 rows whose state is
# CHANGES_REQUESTED): exactly one body is under 40 bytes -- the 21-byte file reference above -- and
# the next smallest is 1002 bytes. So 40 separates the broken body from all 85 real asks with a 25x
# margin on the near side, and no working review this repository has ever received is retired by
# it. The longest is 14863 bytes; 12000 carries 83 of the 86 into the prompt whole, and the three
# longer ones arrive cut with the total and the command that reads the rest named in the row.
#
# The `reference` arm is for what a byte floor cannot see: a body that is nothing but a path or an
# @-mention, however long. `/home/reviewer/notes/pr240-review-notes.md` is 42 bytes and 42 bytes of
# nothing, and a floor set high enough to catch it would start refusing real asks.
FACTORY_ASK_MIN_BYTES=40
FACTORY_ASK_MAX_BYTES=12000

# factory_ask_flat <raw-body> -> the body as the measurement and the human both see it
#
# Control characters become spaces, runs of spaces squeeze to one, both ends trim. One spelling,
# because the byte count and the text quoted to a person have to come from the same string: a body of
# four newlines is 4 bytes of newline and 0 bytes of ask, and the whole point of the predicate is to
# report the second number. `tr`, not deletion -- a newline between two words costs a space, so a
# body written across lines is measured as the prose it is.
factory_ask_flat() {
	printf '%s' "${1:-}" | tr '[:cntrl:]' ' ' | tr -s ' ' | sed -e 's/^ //' -e 's/ $//'
}

# factory_ask_verdict <body>
#   -> "readable <bytes>" | "unreadable:<reason> <bytes>", reason in {empty, reference, below-floor}
#
# It flattens its own input, so the count is the count of the ask rather than the count of whatever
# whitespace a reviewer's editor left around it -- and flattening is idempotent, so a caller holding
# already-flat text gets the same answer twice.
#
# Bytes, not characters, because that is the unit the measured failure was reported in and the unit
# the bound is applied in; for the ASCII markdown these reads return the same number.
factory_ask_verdict() {
	local flat bytes
	flat=$(factory_ask_flat "$1")
	bytes=$(printf '%s' "$flat" | wc -c | tr -dc '0-9')
	if [ "$bytes" -eq 0 ]; then
		printf 'unreadable:empty 0\n'
		return 0
	fi
	# No whitespace anywhere in it, and it names a path or a mention: that is a transport mistake,
	# not a sentence. A real ask has a space in it, which is what makes this shape decidable.
	case "$flat" in
	*[[:space:]]*) : ;;
	*[/@]*)
		printf 'unreadable:reference %s\n' "$bytes"
		return 0
		;;
	esac
	if [ "$bytes" -lt "$FACTORY_ASK_MIN_BYTES" ]; then
		printf 'unreadable:below-floor %s\n' "$bytes"
		return 0
	fi
	printf 'readable %s\n' "$bytes"
}

# factory_newest_ask <reviews-json>
#   -> `id<TAB>submitted_at<TAB>user<TAB>verdict<TAB>flattened-body`, one line, or nothing when the
#      pull request holds no CHANGES_REQUESTED review. Exit 1 in that case, and also when the
#      payload is not the array GitHub sends, because both mean "no ask was read" and neither is an
#      empty body.
#
# The body is the LAST field, and that is load-bearing: it is the only one a reviewer controls, so
# everything the caller needs to split is in front of it. Every control character in it becomes a
# space before it is returned, which is what keeps a body containing U+001F from splicing a column
# into the row it is later joined into -- the hazard that made U+001F the delimiter in the first
# place is the same one, arriving from the other direction.
#
# `max_by` picks the newest by `submitted_at`, the same rule the classifier used before this
# function existed, so adding the body did not change which review is the subject.
factory_newest_ask() {
	local meta id when user body flat verdict
	meta=$(printf '%s' "${1:-}" | jq -r '
		[.[] | select((.state // "") == "CHANGES_REQUESTED")]
		| max_by(.submitted_at // "")
		| if . == null then empty
		  else [(.id // 0 | tostring), (.submitted_at // ""), (.user.login // "")] | join("\u001f") end
	' 2>/dev/null) || meta=''
	[ -n "$meta" ] || return 1
	IFS=$'\037' read -r id when user <<<"$meta"
	body=$(printf '%s' "${1:-}" | jq -r '
		[.[] | select((.state // "") == "CHANGES_REQUESTED")]
		| max_by(.submitted_at // "") | .body // ""
	' 2>/dev/null) || body=''
	# Control characters to spaces, runs squeezed, both ends trimmed: the measurement and the text
	# the human reads are taken from the same string, so a body of four newlines is 0 bytes here and
	# not 4.
	flat=$(printf '%s' "$body" | tr '[:cntrl:]' ' ' | tr -s ' ' | sed -e 's/^ //' -e 's/ $//')
	verdict=$(factory_ask_verdict "$flat")
	printf '%s\t%s\t%s\t%s\t%s\n' "$id" "$when" "$user" "$verdict" "$flat"
}

# factory_ask_bound <flattened-body> <pr>
#   -> at most FACTORY_ASK_MAX_BYTES of it, and when there was more, a cut that names itself, the
#      total and the command that reads the rest.
#
# The limit is not a parameter. A bound the caller supplies is a bound every caller can get wrong,
# and there is exactly one measured number for it (83 of the 86 change requests this repository has
# ever received fit under it whole; see the block above `factory_ask_verdict`).
#
# The marker is part of the transport, not decoration. A silent cut is how this file's own worst
# habit -- a dispatch that looks complete because the payload is not empty -- comes back wearing a
# shorter payload: a tick handed 12000 of a 14863-byte review and told nothing would answer the
# first 12000 and report the ask as answered.
factory_ask_bound() {
	local flat=$1 pr=$2 n
	n=$(printf '%s' "$flat" | wc -c | tr -dc '0-9')
	if [ "${n:-0}" -le "$FACTORY_ASK_MAX_BYTES" ]; then
		printf '%s' "$flat"
		return 0
	fi
	# `-b`, not `-c`: `-c` is a character cut and follows the locale, so the bound would stop being
	# the number written above the moment a reviewer used an em dash. A cut can land inside a
	# multi-byte character, and the trailing bytes of one are cheaper than an unbounded prompt.
	# `head -c`, not `cut -b`: cut is line-oriented and TERMINATES its output with a newline, and
	# this function's output goes straight into the row's seventh column -- so the byte cut used to
	# insert a newline there, splitting the unit into two lines for the supervisor's read loop. The
	# row is newline-delimited and `emit` cannot carry a newline in a field, so a bound that adds one
	# is not a bound but a splice: measured 8 Nov with a 14863-byte body, the ask column arrived as
	# 12000 bytes and the marker -- the sentence that says the ask was cut -- on the line after it,
	# where the dispatcher read it as a row with an empty class. `head -c` writes the bytes and stops;
	# the flattening above guarantees no newline in the input that could make it line-oriented.
	printf '%s' "$flat" | head -c "$FACTORY_ASK_MAX_BYTES"
	printf ' [… cut at %s of %s bytes; the whole body: gh pr view %s --json reviews]' \
		"$FACTORY_ASK_MAX_BYTES" "$n" "$pr"
}

# --------------------------------------------------------------------------- checks
#
# How a `statusCheckRollup` becomes a verdict, defined once here because two different readers --
# triage's bulk read and a re-read immediately before a merge -- must not disagree about what green
# means. Two payload shapes share the list: a CheckRun carries `status` and a `conclusion` that is
# null until it completes; a StatusContext carries neither and only a `state`. Reading a queued
# CheckRun as green is how a merge lands on top of a run that had not failed yet, so an entry is
# unfinished unless its own shape says it is finished.
JQ_CHECKS='
  def badlist: ["FAILURE","TIMED_OUT","ACTION_REQUIRED","STARTUP_FAILURE","CANCELLED","ERROR"];
  def isbad: . as $v | badlist | index($v) != null;
  def isopen:
       ((.state // null) == "PENDING")
    or ((.status // "COMPLETED") != "COMPLETED")
    or ((.status != null) and (.conclusion == null));
  (. // []) as $r
  | ([$r[] | (.conclusion // .state // "") | select(isbad)] | length) as $bad
  | ([$r[] | select(isopen)] | length) as $open
  | if   $bad  > 0 then "fail"
    elif $open > 0 then "pending"
    else "pass" end'

# Checks for one pull request, through the same program as the bulk read. Prints pass, pending,
# fail, or unknown when the pull request could not be read at all.
factory_checks() {
	local out
	if ! out=$(gh pr view "$1" --json statusCheckRollup 2>/dev/null |
		jq -r "(.statusCheckRollup) | $JQ_CHECKS" 2>/dev/null); then
		printf 'unknown\n'
		return 0
	fi
	printf '%s\n' "${out:-unknown}"
}

# --------------------------------------------------------------------------- preflight

# Everything a tick needs, checked before a model is invoked. The measured failure this prevents
# is a session that spent its whole budget failing to read the API -- review-loop learned this
# the same way, and the sentence there is worth repeating: fail before starting work, not during.
factory_preflight() {
	local missing=() out

	for tool in git gh "$FACTORY_OPENCODE"; do
		command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
	done
	if [ "${#missing[@]}" -gt 0 ]; then
		die "missing on PATH: ${missing[*]}. Nothing was started."
	fi

	# `gh auth status` exits non-zero when the token cannot authenticate. It is the one check
	# whose failure means every other action in this script would fail, so it is fatal here and
	# retried by the next timer activation rather than parked.
	if ! out=$(gh auth status 2>&1); then
		die "gh auth status failed, so no GitHub call can be trusted:
$out"
	fi

	if [ ! -d "$FACTORY_ROOT/.git" ] && ! git -C "$FACTORY_ROOT" rev-parse --git-dir >/dev/null 2>&1; then
		die "$FACTORY_ROOT is not a git repository"
	fi

	# gh takes its repository from the working directory and has no `-C`, so the whole script runs
	# from the root it just resolved. Without this, `gh pr list` started from a linked worktree
	# reports the same repository by accident and a different one the moment the worktree is moved
	# or belongs to a fork.
	# A failed cd here would leave every later `gh` call resolving some other repository from
	# wherever the caller happened to be standing, which is the failure this function exists to
	# catch -- so it stops rather than continuing in the wrong directory.
	cd -- "$FACTORY_ROOT" || die "cannot enter $FACTORY_ROOT"

	# The repo identity, once. From the remote URL first: `gh repo view` resolves the repository
	# from the *current directory*, which is not this one when the supervisor is started from a
	# linked worktree, and there is no `gh -C` to say otherwise. The expression takes both the SSH
	# form (`git@host:owner/name.git`) and the HTTPS form (`https://host/owner/name.git`).
	#
	# The `.git` suffix is stripped before the match, not after it: `[^/]+` is greedy, so a pattern
	# ending in `(\.git)?$` happily captures `parental-lockout.git` as the repository name and
	# every API call under it answers 404. Measured on the first run of this script -- three of the
	# five PRs came back as having no reviews and no requested reviewers, which is exactly what a
	# wrong repository looks like when the caller swallows the error.
	if [ -z "$FACTORY_REPO" ]; then
		FACTORY_REPO=$(git -C "$FACTORY_ROOT" remote get-url origin 2>/dev/null |
			sed -En -e 's#\.git$##' -e 's#^([^/]+[:/])?([^/]+/[^/]+)$#\2#p')
		if [ -n "$FACTORY_REPO" ] && ! factory_valid_repo "$FACTORY_REPO"; then
			FACTORY_REPO=''
		fi
	fi
	if [ -z "$FACTORY_REPO" ]; then
		FACTORY_REPO=$( (
				cd -- "$FACTORY_ROOT" &&
					gh repo view --json nameWithOwner --jq .nameWithOwner 2>/dev/null
			) || true)
	fi
	[ -n "$FACTORY_REPO" ] || die "cannot determine the repository from origin; set FACTORY_REPO=owner/name"

	FACTORY_OWNER=${FACTORY_REPO%%/*}
	FACTORY_NAME=${FACTORY_REPO##*/}

	# The account the supervisor will act as must not be the reviewer it asks. GitHub refuses a
	# self-review silently enough that the wait would look like a slow reviewer.
	if [ "$FACTORY_REVIEWER" = "$(gh api user --jq .login 2>/dev/null || printf '')" ]; then
		die "FACTORY_REVIEWER is this same account ($FACTORY_REVIEWER); a PR cannot be reviewed by its author"
	fi

	factory_paths
}

# --------------------------------------------------------------------------- git

# The primary checkout, not the directory the script was called from: `git worktree add` and
# `git worktree remove` both have to be issued against the main working tree, and the supervisor
# is routinely started from inside a linked one.
factory_main_checkout() { _factory_main; }

# A safe directory name for a branch: `/` is legal in a branch name and illegal in a path
# component below one, and the repo's own convention (fix/…, feat/…) guarantees it shows up.
factory_slug() { printf '%s' "$1" | tr '/' '-'; }

# The tree for a branch, making one when it is absent: from origin/<branch> if the branch is only on
# the remote, from origin/main if it is nowhere yet, and from the branch itself if it is already
# committed here. A branch already checked out somewhere else is the one case that stops here, with
# the path of that other tree in the message. Prints the path.
factory_worktree() {
	local branch=$1 slug path main other
	slug=$(factory_slug "$branch")
	path="$FACTORY_TREES/$slug"
	if [ -d "$path" ]; then
		printf '%s\n' "$path"
		return 0
	fi
	mkdir -p -- "$FACTORY_TREES"
	main=$(factory_main_checkout)
	if [ "$DRY_RUN" = "1" ]; then
		printf '%s\n' "$path"
		return 0
	fi
	# Options before the path, in both attempts. `git worktree add "$path" --branch "$branch"
	# origin/main` -- which is what this line used to be -- is not a command line git accepts: options
	# after the positional path are rejected, and the `2>/dev/null` on the first attempt threw away the
	# usage dump that was the entire diagnosis. Measured on the first timer-driven pass: all three ticks
	# died here in under two seconds, because a worktree is only *created* for the new-branch case, and
	# every unit that needed one is an issue.
	# Three shapes of branch, not two. Every unit that is already a pull request -- and every stranded
	# branch -- names a branch that ALREADY EXISTS locally, and neither attempt below can build a tree
	# for one: `worktree add -b` answers `fatal: a branch named 'fix/audit-row-label' already exists`,
	# both attempts, every time. Measured 5 Nov: that is what wedged pr:209, pr:214, pr:215 and pr:217
	# for 29 hours once their trees turned out to live under /tmp/opencode instead of FACTORY_TREES --
	# four `could not create a worktree` lines a pass, and a queue that could not be emptied from it.
	# So take the checked-out form when the branch is already here. When some other worktree owns it,
	# name that worktree: moving a tree this supervisor did not create is a person's call, but saying
	# where it is has no cost and is the whole difference between a blocker and a mystery.
	if git -C "$main" show-ref --verify --quiet "refs/heads/$branch"; then
		other=$(git -C "$main" worktree list --porcelain 2>/dev/null |
			awk -v b="refs/heads/$branch" '
				/^worktree /{w=substr($0,10)}
				/^branch /{if (substr($0,8)==b) {print w; exit}}')
		if [ -n "$other" ]; then
			die "branch $branch is checked out in $other; bring it to $path with \`git worktree move $other $path\` when that tree is spent"
		fi
		if git -C "$main" worktree add "$path" "$branch" >>"$FACTORY_LOG/worktree.log" 2>&1; then
			printf '%s\n' "$path"
			return 0
		fi
		die "branch $branch exists locally but no worktree could be created at $path; see $FACTORY_LOG/worktree.log"
	fi
	if ! git -C "$main" worktree add --track -b "$branch" "$path" "origin/$branch" \
			>"$FACTORY_LOG/worktree.log" 2>&1; then
		# No such branch on origin, which is the normal case for an issue: cut it from origin/main.
		# Both attempts land in the log, because only the second message distinguishes "already checked
		# out in another worktree" from "the repository is locked" -- and both need reading after the
		# fact, when the tree is gone.
		if ! git -C "$main" worktree add -b "$branch" "$path" origin/main \
				>>"$FACTORY_LOG/worktree.log" 2>&1; then
			die "could not create a worktree for $branch at $path; see $FACTORY_LOG/worktree.log"
		fi
	fi
	printf '%s\n' "$path"
}

# git's own words, when a read said any. The FIRST line only, because that is the line that names the
# problem and the rest is git's prose about the same problem; and capped, because the sentence this
# feeds is one line in `factory/HALT` that a person reads in a terminal. A truncated line says it was
# truncated and by how much -- a list that stops without saying it stopped is how three paths came to
# read as ten (see `factory_path_list`), and this function exists to stop that, so it does not commit
# it a second time. Empty output means the read printed nothing, which is most reads, and is the
# honest shape: nothing here is invented to fill a sentence.
factory_git_said() { # errfile -> git's first stderr line, or nothing
	local line
	line=$(head -n 1 -- "$1" 2>/dev/null | tr -d '\r')
	[ -n "$line" ] || return 0
	if [ "${#line}" -gt 200 ]; then
		printf '%s (git line cut at 200 of %s chars)' "${line:0:200}" "${#line}"
	else
		printf '%s' "$line"
	fi
}

# Did a read that did not refuse actually FINISH? The distinction is #232, and it is invisible to any
# status test: measured on git 2.53.0 at 99f165d, uid 1000, with a directory inside the tree at mode
# 000, `git diff --quiet` prints `sub/nested.txt: Permission denied` and exits 0, and
# `git ls-files --others --exclude-standard` prints
# `warning: could not open directory 'drafts/': Permission denied` and exits 0. Both are the verdict's
# success answer with a sentence attached saying it was not earned, and both were a CLEAN over real
# uncommitted work until this function existed. So what a read PRINTED is evidence as well as
# diagnosis: a read that returned its success status and still said something did not complete, and a
# verdict built on an incomplete read is not evidence of safety -- the same thesis as the refusal arms,
# arrived at from the side the exit statuses cannot see.
#
# `success-statuses` is that arm's own list of answers that mean "I read the tree and I am telling you
# the truth about it": `0 1` for the refresh, whose 1 is `path: needs update` -- a normal report, not an
# error, and not a claim about cleanliness either, since the arms below are what name the dirt -- and
# `0` alone for the four reads whose 1 IS a finding. A status outside the list is either a refusal,
# which the arm's status test has already caught, or dirt, which halts anyway and carries git's line in
# its own clause. Keeping dirt out of the list is what makes the rule safe rather than merely strict: a
# read that reported work AND warned has still told the truth about the tree, and downgrading its
# finding to "could not be read" would send the operator to fix the wrong thing. The same bargain runs
# twice more, and both times it is the finding that wins. The two `ls-files` arms carry their finding in
# their output rather than their status, so they add `[ -z "$raw$finding" ]`; and an incomplete read
# that found nothing, arriving at a verdict that ALREADY holds another arm's finding, appends
# `its … were only partly read` to that finding instead of replacing it. The measured case is the one
# that decided it: a readable edit to `tracked.txt` beside a directory at mode 000 answered UNKNOWN,
# naming nothing but the untracked read's warning, and told somebody who has an edit in front of them to
# go make the tree readable. It answers DIRTY naming the edit now, with the warning carried beside it.
# What still has to refuse is the dangerous half, and only that half: a read that found NOTHING and said
# NOTHING may be clean, and one that found nothing while saying something is not.
#
# What this does not do is look at WHAT git said. Matching git's wording is how a check stops meaning
# anything the day upstream rephrases a warning -- the reason every leg asserts this repository's own
# marker (`git said:`) and never a phrase of git's -- so the rule is "it spoke", and the words ride on
# to the operator through `factory_git_said` unchanged.
factory_git_incomplete() { # errfile rc success-statuses -> 0 when the read returned one and still spoke
	local errf=$1 rc=$2
	case " $3 " in *" $rc "*) ;; *) return 1 ;; esac
	[ -s "$errf" ]
}

# Up to three paths, space separated, plus the count when there were more. `head -n 3` on its own is a
# claim that the list is complete: ten problems arrive as three names and a trailing space, the reader
# stops at the last comma, and the seven they did not see are the ones that were blocking the merge.
# The marker is one clause and makes the cap the reader's friend instead of their trap; the total is
# there too, because "and 4 more" and "and 400 more" send an operator to different actions.
factory_path_list() { # stdin: paths -> at most three, plus "(+N more, M in all)"
	local all n
	all=$(sort -u)
	[ -n "$all" ] || return 0
	n=$(printf '%s\n' "$all" | wc -l)
	if [ "$n" -gt 3 ]; then
		printf '%s(+%s more, %s in all)' "$(printf '%s\n' "$all" | head -n 3 | tr '\n' ' ')" \
			"$((n - 3))" "$n"
	else
		printf '%s' "$(printf '%s\n' "$all" | tr '\n' ' ')"
	fi
}

# Is there anything in TREE that nobody has committed? Prints one line naming what it found,
# and nothing when the tree is clean:
#
#   0  there is uncommitted work, and the line says which kind and where
#   1  clean -- every tracked file's bytes match the index, the index matches HEAD, and
#      nothing is untracked. Every read that had to answer did so AND said nothing: a read
#      that returns its success status with a sentence on stderr did not finish, and its
#      silence is not counted as an answer (see `factory_git_incomplete`, #232).
#   2  its state could not be read, or one of its reads answered without finishing. The
#      caller must treat that as dirt, not as clean: this verdict gates writing into a tree,
#      and "I could not tell" is not evidence of safety.
#
# Why this exists instead of `test -n "$(git status --porcelain)"`, which is what both call
# sites used (#228). `status` is being asked a *content* question -- "did anyone leave work
# here, or is this tree safe to write into" -- and it answers a *stat-cache* question: it
# reports a file dirty from its cached size/mtime/inode, which is data about the filesystem,
# not about the bytes. A killed `check-mutations.py` is a machine for making the two disagree:
# it rewrites tracked files in place and writes the original bytes back, so the content ends
# where it began and the stat data does not. On a halt that is the expensive kind of
# disagreement -- a halt is global (`$FACTORY_HALT` stops every unit in the pass), and the
# halt's own advice cannot undo it: "commit it to its branch, or discard it on purpose"
# commits nothing and discards nothing when there is no content difference, and the halt
# returns on the next pass.
#
# So the verdict is built out of content reads, each pinned by a leg in `factory/selftest.sh`:
#
#   `diff --cached` (index vs HEAD)  a staged-only change -- `git add` then the worktree put
#                                    back -- is invisible to `git diff`. Measured: status
#                                    DIRTY, `diff --quiet` CLEAN. It is also exactly the state
#                                    a half-merged index is, so dropping this arm would let
#                                    the one tree this halt exists to refuse walk straight
#                                    through. Issue #228's literal prescription (diff plus
#                                    untracked) has this hole; selftest.sh refuses it.
#   `diff` (worktree vs index)       an edit to a tracked file.
#   `ls-files --unmerged`            a conflicted index. Conflicts are index *entries*, not
#                                    files, so they are read from the index, not diffed.
#   `ls-files --others`              untracked files -- and the plumbing for them rather than
#                                    `status`, because `status` is the command whose
#                                    stat-derived answer is not trusted here. `--exclude-standard`
#                                    keeps build output out of it: an ignored file is not work.
#
# The `update-index --refresh` first is not belt-and-braces: it is what stops a stale stat
# cache answering at all, and it persists the recheck rather than redoing it per read. It
# exits 1 for files that need updating -- which is dirt the arms below go on to name, not an
# error -- so only 2 and above mean git could not do the job. That case returns 2, and both
# call sites halt: refusing on an unreadable tree costs one pass, proceeding on one costs a
# model writing into someone's half-done work.
#
# Checking each read's EXIT STATUS, and not just what it printed, is load-bearing rather than
# tidy, and the two shapes differ in a way that decides how the arms have to be written. Measured on
# git 2.53.0. The other half of the sentence is also load-bearing, and #232 is the proof: a clean status
# is only an answer if the read finished, so every arm asks the same two questions of its own read --
# did it refuse, and did it speak while refusing nothing -- and neither one is a substitute for the
# other. The statuses below are all measured; the stderr column exists only because H-0046 kept it.
#
# `GIT_DIR` at a `.git` directory holding nothing -- a stale entry under `$FACTORY_TREES` -- every
# read refuses (128, or 129 for the two `diff` forms), so the first guard is enough; that is also
# where the replaced expression goes wrong, because `status --porcelain` exits 128 printing nothing,
# and `test -n "$(… 2>/dev/null)"` reads that as clean.
#
# `GIT_DIR` at a working repository elsewhere with `GIT_WORK_TREE` at a path that is not there -- an
# environment a wrapper exported and never unset -- the reads stop agreeing with one another:
# `update-index --refresh`, `diff --quiet` and `ls-files --others` exit 128 while
# `diff --cached --quiet` and `ls-files --unmerged` exit 0, meaning clean, out of the OTHER
# repository's index. A verdict assembled from the two that answer reports no work in a tree it
# cannot open, and a verdict that looks only at output rather than status does the same.
#
# The third shape `factory/selftest.sh` plants is the mundane one: `core.excludesFile` aimed at a
# directory, which is configuration rather than per-tree state, so every `git -C` inherits it.
# Measured there on a filesystem where a directory has a size (tmpfs: 40): `refresh`, `diff` and
# `ls-files --unmerged` answer clean, `diff --cached` and `ls-files --others` exit 128, and
# `status --porcelain` exits 128 printing NOTHING -- so both the replaced expression and the
# diff-plus-untracked prescription answer CLEAN on a tree whose state git would not report.
#
# That 128 is a filesystem property, not a git promise, and the qualifier is load-bearing -- but the
# filesystem is only half of it, and the first version of this comment got the half wrong. git opens
# the exclude file, `fstat`s it and treats `st_size == 0` as an empty list; only a non-zero size
# reaches the `read()` that fails with EISDIR and dies. A directory's `st_size` is not a constant of
# its filesystem: it is the size of the directory's own entry data, so it depends on what the
# directory CONTAINS. Re-measured on this box at 8015951, `stat -c %s` on the directory `core.
# excludesFile` was aimed at: btrfs 0 when empty, 2 when it holds one entry named `x`, 12 when that
# entry is named `inside`; tmpfs 40 when empty and 60 holding either. ext4 reports 4096 for both --
# that figure is the reviewer's, from a box that has ext4; this machine has none (`findmnt`: btrfs,
# vfat, fuseblk, nfs, overlay), which is why the earlier sentence here said "btrfs and tmpfs, the two
# filesystems this box has" and named a third it had not measured.
#
# So the swallow needs an *empty* directory on a filesystem that gives empty directories no size, and
# the same typo refuses the moment the mistyped directory holds anything -- on btrfs included. The
# factory's own fixture directory is empty, which is the whole reason the review box answered the way
# it did. On btrfs with an empty directory nothing is refused, the operator's global patterns are
# silently dropped, and the verdict answers DIRTY naming a file git was told to ignore: the false
# halt, with no warning from git in that shape to say why. Both arms are measured by the leg, which
# asks git whether the plant refused before it asserts a verdict.
#
# Which read refuses first is a property of the git, not a promise, so each arm checks its own status
# and the first refusal wins. The five checks are therefore redundant on purpose, and this comment has
# been wrong about that twice, which is why the numbers below name the mutation each was taken with.
#
# First error: it claimed hollowing the tracked read's check "turns two legs red". At 8015951 it turned
# NONE red, and the cause was the leg, not the arm -- `expect … HALT` accepts any answer but CLEAN, so
# the verdict could degrade from `its tracked files could not be read (git diff exited 128)` to
# `modified: keepme.txt (git said: … Permission denied)` and the leg still passed. Both halt; only one
# of them tells the operator the truth, since the second sends them to "commit it to its branch, or
# discard it on purpose" about a file git could not open. The leg asserts UNKNOWN now (H-0048).
#
# Second error, in the same edit: the replacement text read the reviewer's table as though it had been
# taken with all five arms hollowed. It had not. There are two ways to delete an arm and they are not
# interchangeable -- a fact worth the number it cost:
#
#   * `return 2` -> `:` -- the arm SEES the refusal and keeps going. The finding is appended to the
#     refusal's sentence and the verdict ends up answering CLEAN. This is the shape the review used,
#     and at cf87ecc it reproduces that table exactly: the tracked arm 4 legs red (52 of 56), the
#     refresh arm 1 red (55 of 56, `a directory that is no repo`), the other three none.
#   * `[ "$rc" -gt 1 ]` -> `[ 1 -eq 0 ]` -- the arm does not see the refusal at all and reads it as
#     dirt, so it still halts and only a leg that asserts the refusal's own WORDS can tell. Stricter
#     of the two, and at 8015951 it pinned one arm of five: not zero, not four.
#
# And `[ "$rc" -gt 99 ]` hollows nothing -- a refusal is 128 -- so the first sweep of this measured
# unmutated code five times and reported five clean runs. That is #1048's failure in shell clothing,
# and the cure is the same: prove the mutation bites before believing its tally.
#
# That four arms were unpinned by every plant on disk is a property of the plants, not of the arms:
# each shape that stops one of them stops another as well, because that is what an unreadable tree
# does. So the isolation is built rather than found (H-0049) -- a `git` placed first on PATH that fails
# exactly one invocation and hands every other argument list to the real binary untouched, one leg per
# arm in `factory/selftest.sh`, each asserting that arm's own sentence in the verdict and in the halt,
# behind a control whose match names nothing git ever runs so the shim cannot halt by construction. Be
# clear about what that pins: the ARM, that this read's refusal alone stops the pass and says which
# read it was. It claims nothing about git refusing this way on its own; the planted legs above remain
# the only evidence for that, on the filesystems that can supply it. Legs red in the tree at e97fe83, of 81:
#
#   hollowed arm              `return 2` -> `:`   condition -> always false
#   refresh                             3                          2
#   staged (`diff --cached`)            3                          2
#   tracked (`diff`)                    6                          6
#   unmerged                            2                          2
#   untracked (`--others`)              2                          2
#   arms 2-5                           14                         13
#   all five                           20                         18
#
# The three-red rows in the first column are the one-line contract catching the fall-through: the
# refusal's printf plus the appended finding make a two-line HALT file, which `expect_halt` refuses.
# The six on the tracked arm are those two plus the unreadable-file leg in both trees. A verdict that
# refused only on the strength of one read would be a verdict that had to trust that read, and the
# point of the exercise is that no read here is trustworthy on its own; the redundancy is the design,
# and every arm of it now has a leg that names it.
#
# The same measurements say what the fix is and is not. A stat-only difference -- the state the
# issue describes -- does not make `status --porcelain` report dirt on this git: status refreshes
# the index in memory and finds the bytes equal, whether or not `.git/index` or even `.git/` is
# writable and whether or not `core.trustctime` is on; six planted variants read clean, and the
# probe says so on its own line rather than pretending to catch it. What IS reproducible here is
# the other direction, and it is the worse one: an unreadable tree answered CLEAN and the tick would
# have worked in it. A false halt costs a pass. A false clean costs the work.
#
# What the refresh is, since it is the only read here that WRITES. Measured at 8015951 in a throwaway
# repository, before and after one `git update-index --refresh`: on a stat-only difference it rewrote
# `.git/index` -- its sha256 changed and the cached mtime moved from `1794119396:675049893` to
# `1577836800:0` -- and left the file's bytes alone. On a tree with an unresolved conflict it wrote
# nothing and settled nothing: three index stages before and three after, `MERGE_HEAD` still there,
# the conflicted file's sha unchanged, and the refresh's own report was `tracked.txt: needs merge` at
# exit 1. So it is a metadata write, never a content write and never a resolution; the verdict that
# runs it cannot quietly tidy a conflict away, and the `unmerged` arm still sees three stages after it.
#
# That write is also what makes the refresh the read that notices a person. With `.git/index.lock`
# present -- somebody mid-`git add` in the same tree -- `update-index --refresh` exits 128 with
# `fatal: Unable to create '…/.git/index.lock': File exists.`, while all four other reads AND
# `status --porcelain` answer 0 over the same tree. So the one read that touches the tree is the one
# that refuses when the tree is busy, and UNKNOWN is the right answer to a held lock: the tick does
# not interleave with a human's index write, and the reason in the sentence names the lock file. What
# remains unsafe, and is accepted rather than fixed: the verdict writes into a `.git` that may belong
# to a person working in that tree, and `busy` is only visible in the one shape above. The guarded
# writes that follow the verdict are not metadata -- the conflict pass runs `merge --no-edit`
# (`factory/factory.sh:382`) and `merge --abort` when it fails (`:383`) -- which is exactly why the
# verdict must not mistake a busy or unreadable tree for a clean one. And one shape of "busy" is
# invisible to all five reads' statuses: with a directory inside the tree at mode 000, `ls-files --others`
# prints `warning: could not open directory 'sub/': Permission denied` and exits 0, `diff --quiet`
# prints `sub/loose.txt: Permission denied` and exits 0, and the verdict answered CLEAN over a tree
# holding an untracked file nobody could enumerate. Measured at 8015951 and filed as #232; the fix is
# the completeness test below, and it is made of the capture H-0046 added for the sentence. Nothing
# about the screen or the lockout is involved: the supervisor's tick never touches the
# display, and re-locking is the daemon's job on its own ticks (`internal/enforce/enforce.go:107`).
#
# What each read PRINTED on stderr is kept, and its first line rides in the sentence. `2>/dev/null` was
# doing two jobs at once, and only one of them was wanted: it is what turned a 128 into a pass in the
# expression this replaces, and it is also why the *opposite* failure has no explanation for the
# operator. Measured on git 2.53.0 with `core.excludesFile` aimed at a `chmod 000` file: all five reads
# answer 0, `git status --porcelain` prints `?? excluded-by-the-global-file.tmp` plus one line --
# `warning: unable to access '…': Permission denied` -- and this verdict answers DIRTY naming a file the
# operator had explicitly told git to ignore. That warning is the whole diagnosis of the false halt,
# and the old code threw it away before the sentence was written. Same bargain on the refusal arms,
# where git's line is `fatal: cannot use … as an exclude file` rather than a warning. A read that
# printed nothing adds nothing: the btrfs shape of the same plant drops the patterns in real silence,
# and a sentence with no reason in it is the accurate report of that.
#
# And since #232 the capture answers a second question, which is the one the exit statuses cannot ask.
# A read that returns its success status and writes to stderr did not finish, and every arm now refuses
# on that (`factory_git_incomplete`). Both halves of #232 are planted in `factory/selftest.sh` and both
# were a CLEAN at 99f165d on git 2.53.0, uid 1000, in throwaway repositories:
#
#   a mode-000 directory holding   the five reads                 the firing read
#   only untracked work            1,0,0,0,0 all silent except
#                                  `warning: could not open directory 'drafts/': Permission denied`
#                                  on `ls-files --others --exclude-standard`, rc=0
#   a MODIFIED tracked file in a   1(silent),0,0,0,0 with
#   mode-000 directory             `sub/nested.txt: Permission denied` on `diff --quiet`, rc=0
#                                  and the directory warning on `--others`, rc=0
#
# The second row is the worse one and the issue did not name it: `diff` could not open the file and
# reported it unchanged, so an edit somebody made is not merely unlisted, it is asserted to be absent.
# Note which read fires in each row: the tracked arm speaks first in the second tree because a tracked
# file sits in the unreadable directory beside the untracked one, which is why the legs assert the
# arm's own words rather than "something refused" (H-0048's lesson, applied to the new branch). The
# first row is planted with no tracked file inside the directory so that the untracked arm is the one
# pinned by a state on disk; the other three arms' completeness branches are pinned by the built shim
# only, because no shape on disk stops one of them and not another.
#
# What that costs, written before taking it, because it changes halt semantics and not just a verdict.
# Any git that prints something harmless to stderr on one of these five reads now halts the whole pass,
# and the shape that already exists today is H-0045's exclude-file warning: measured here with
# `core.excludesFile` in a symlink loop, `diff --cached` and `ls-files --others` both answer rc=0 with
# `warning: unable to access '…': Too many levels of symbolic links` while the other three reads stay
# silent. So the leg that plants it no longer answers DIRTY naming the file the operator told git to
# ignore -- it answers UNKNOWN naming git's warning, and the staged arm is the one that fires. Both
# halt: the pass stopped either way, and what changed is the sentence. It used to order the operator to
# "commit it to its branch, or discard it on purpose" about a file they had explicitly ignored; it now
# says the path git could not read. The genuinely new halt is the tree that is clean on its face and
# whose configuration git cannot read, which used to proceed and now stops on the ground that git did
# not apply the operator's own ignore patterns, so git's clean is not the operator's clean. The cost of
# that is a sticky global halt from a config typo nobody noticed, which is the same asymmetry #228 was
# filed on -- a false halt costs one pass, a false clean costs the work -- applied to a case #228 did
# not have. Sixteen real trees on this box -- this repository and its fifteen links -- were read the
# four NON-WRITING ways before this paragraph was written (64 reads) and every one of them was silent.
# The refresh is not in that audit: it rewrites `.git/index`, and those trees may belong to a person
# working in them. So nothing that exists today halts on chatter that is already there; what halts is
# whatever a future git starts saying.
#
# Each arm names at most three paths and then says how many it left out and how many there were; see
# `factory_path_list`.
factory_tree_dirt() {
	local errf rc=0
	# One scratch file for git's stderr, opened here and removed here, because the reads have several
	# exits and a `rm` on only one of them leaks a file per tree per pass. factory.sh already made this
	# bargain for the same reason (its `CONFLICT-BLOCKED` line keeps `factory_worktree`'s stderr in an
	# mktemp and journals it).
	#
	# When `mktemp` fails there is nowhere to keep a reason, and since #232 a reason is also evidence:
	# the only thing that distinguishes a clean tree from one whose reads silently came back unfinished
	# is what those reads printed. A run that cannot capture stderr therefore cannot answer, and it
	# refuses. This deliberately reverses H-0046's bargain for this function -- "a reason is diagnosis,
	# not evidence, and losing it must not change a verdict" -- because keeping it would leave the whole
	# of #232 invisible on exactly the boxes whose `/tmp` is broken, which is the failure this verdict
	# exists to refuse and not one it gets to be indifferent about. Say what that trades: a machine with
	# a full or unwritable `/tmp` now halts the pass with a reason instead of carrying on, and the
	# reason names `TMPDIR` because that is the thing to fix. Pointing the capture at `/dev/null`
	# instead -- the old fallback -- is not taken: it is the same silence with a filename on it, and the
	# run that chmod'd `/dev/null` is already on this repository's log.
	if errf=$(mktemp "${TMPDIR:-/tmp}/plock-dirt-XXXXXX" 2>/dev/null); then
		factory_tree_dirt_read "$1" "$errf" || rc=$?
		rm -f -- "$errf"
		return "$rc"
	fi
	printf 'its reads could not be checked for completeness (no scratch file for git%s stderr under %s, so a warning meaning "I could not read this" would be discarded)\n' \
		"'s" "${TMPDIR:-/tmp}"
	return 2
}

# The verdict proper, with the scratch file it writes git's stderr to. See `factory_tree_dirt` for
# what each arm is for; this is how each arm's reason is kept alongside its answer.
factory_tree_dirt_read() {
	local tree=$1 errf=$2
	local rc=0 out="" finding="" raw="" why=""
	: >"$errf"
	rc=0
	# `>/dev/null` stays: `update-index --refresh` writes "path: needs update" to its STANDARD output
	# for every file it re-stat'd, and the verdict's contract is one line. Dropping that redirect --
	# while keeping the stderr capture -- put the refresh's own chatter in front of the finding, which
	# split `factory/HALT`'s sentence across two lines and made the tick's journal line unreadable as
	# the last line of the file. The legs below catch it, and one of them now catches it by name.
	git -C "$tree" update-index --refresh >/dev/null 2>>"$errf" || rc=$?
	if [ "$rc" -gt 1 ] || factory_git_incomplete "$errf" "$rc" "0 1"; then
		why=$(factory_git_said "$errf")
		printf 'its state could not be read (git update-index --refresh exited %s%s)\n' \
			"$rc" "${why:+; git said: $why}"
		return 2
	fi

	rc=0
	: >"$errf"
	git -C "$tree" diff --cached --quiet 2>>"$errf" || rc=$?
	if [ "$rc" -gt 1 ] || factory_git_incomplete "$errf" "$rc" 0; then
		why=$(factory_git_said "$errf")
		printf 'its staged state could not be read (git diff --cached exited %s%s)\n' \
			"$rc" "${why:+; git said: $why}"
		return 2
	elif [ "$rc" -ne 0 ]; then
		out=$(git -C "$tree" diff --cached --name-only 2>>"$errf" | factory_path_list)
		why=$(factory_git_said "$errf")
		finding="${finding}staged-but-uncommitted: ${out:-and the index itself}${why:+ (git said: $why)}"
	fi

	rc=0
	: >"$errf"
	git -C "$tree" diff --quiet 2>>"$errf" || rc=$?
	# An incomplete read that turns up with nothing to add, to a verdict that has already found work, is
	# a clause and not a verdict: the pass halts either way, and the operator's action is the work. The
	# refusal above stays a refusal -- an unreadable tree voids the whole answer, which is what H-0044
	# and H-0045 pinned -- but a warning beside a finding that is already in hand does not.
	if [ "$rc" -gt 1 ] || { factory_git_incomplete "$errf" "$rc" 0 && [ -z "$finding" ]; }; then
		why=$(factory_git_said "$errf")
		printf 'its tracked files could not be read (git diff exited %s%s)\n' \
			"$rc" "${why:+; git said: $why}"
		return 2
	elif [ "$rc" -ne 0 ]; then
		out=$(git -C "$tree" diff --name-only 2>>"$errf" | factory_path_list)
		why=$(factory_git_said "$errf")
		[ -n "$finding" ] && finding="$finding; "
		finding="${finding}modified: ${out:-a tracked file}${why:+ (git said: $why)}"
	elif factory_git_incomplete "$errf" "$rc" 0; then
		why=$(factory_git_said "$errf")
		[ -n "$finding" ] && finding="$finding; "
		finding="${finding}its tracked files were only partly read (git diff exited 0${why:+; git said: $why})"
	fi

	# The unmerged arm reads the index, not the files: a conflict is three index entries, so there is
	# nothing to diff. `sort -u` because a conflicted path is one entry *per stage*: unsorted, one
	# file is named three times in the sentence the halt prints, which reads like three problems and
	# sends the reader looking for two files that are not there.
	#
	# Both `ls-files` reads have their exit status checked for the reason the whole arm exists: a
	# failure here is silent in exactly the dangerous direction. Measured on git 2.53.0 with an
	# inherited `GIT_DIR`/`GIT_WORK_TREE` pair pointing at another repository -- `ls-files --others`
	# exits 128 with no output while `diff --cached --quiet` and `ls-files --unmerged` answer 0, i.e.
	# clean. An unchecked substitution there reads as "no untracked files", which is the same
	# mistake `2>/dev/null` made in the expression this function replaces.
	#
	# And both have the completeness test, because #232's dangerous direction does not refuse: with a
	# directory at mode 000 and uncommitted work inside it, `ls-files --others --exclude-standard`
	# returns 0 with an EMPTY list and one warning, which the status test reads as "no untracked files"
	# exactly as loudly as the 128 above does. The empty list is the difference -- 128 at least says it
	# could not answer. The `[ -z "$raw" ]` half is the same bargain the `diff` arms strike by returning
	# their finding at rc=1: an incomplete read that found NOTHING may never be read as clean, while one
	# that found work has already stopped the pass, and its clause carries git's warning beside the
	# paths it did manage to name.
	rc=0
	: >"$errf"
	raw=$(git -C "$tree" ls-files --unmerged 2>>"$errf") || rc=$?
	# The three-way split is #232's rule in one place: a refusal voids the verdict, an incomplete read
	# that found nothing voids a CLEAN, and an incomplete read that found nothing beside a finding
	# somebody else already made is a clause on that finding rather than a different answer. `[ -z
	# "$raw$finding" ]` is the whole test because the two reads carry their finding in their output.
	if [ "$rc" -gt 1 ] || { factory_git_incomplete "$errf" "$rc" 0 && [ -z "$raw$finding" ]; }; then
		why=$(factory_git_said "$errf")
		printf 'its index could not be read (git ls-files --unmerged exited %s%s)\n' \
			"$rc" "${why:+; git said: $why}"
		return 2
	elif [ -n "$raw" ]; then
		out=$(printf '%s\n' "$raw" | awk '{print $4}' | factory_path_list)
		why=$(factory_git_said "$errf")
		[ -n "$finding" ] && finding="$finding; "
		finding="${finding}unmerged: ${out:-a conflicted path}${why:+ (git said: $why)}"
	elif factory_git_incomplete "$errf" "$rc" 0; then
		why=$(factory_git_said "$errf")
		[ -n "$finding" ] && finding="$finding; "
		finding="${finding}its index was only partly read (git ls-files --unmerged exited 0${why:+; git said: $why})"
	fi

	rc=0
	: >"$errf"
	raw=$(git -C "$tree" ls-files --others --exclude-standard 2>>"$errf") || rc=$?
	if [ "$rc" -gt 1 ] || { factory_git_incomplete "$errf" "$rc" 0 && [ -z "$raw$finding" ]; }; then
		why=$(factory_git_said "$errf")
		printf 'its untracked files could not be read (git ls-files --others exited %s%s)\n' \
			"$rc" "${why:+; git said: $why}"
		return 2
	elif [ -n "$raw" ]; then
		out=$(printf '%s\n' "$raw" | factory_path_list)
		why=$(factory_git_said "$errf")
		[ -n "$finding" ] && finding="$finding; "
		finding="${finding}untracked: ${out:-an untracked file}${why:+ (git said: $why)}"
	elif factory_git_incomplete "$errf" "$rc" 0; then
		why=$(factory_git_said "$errf")
		[ -n "$finding" ] && finding="$finding; "
		finding="${finding}its untracked files were only partly read (git ls-files --others exited 0${why:+; git said: $why})"
	fi

	if [ -n "$finding" ]; then
		printf '%s\n' "$finding"
		return 0
	fi
	return 1
}

# The halt, in one place, because the decision above it was written twice and executed by nothing.
# A mutation audit of this file's two call sites -- flip `[ "$dirt_rc" -ne 1 ]` to `-ne 0`, or to
# `-eq 0`, or delete the guard -- left every leg green, three mutations running (H-0044): the probe
# drove `factory_tree_dirt` directly, so the code that decides whether a model is sent into a tree
# had no test at all. A verdict that is right is worth nothing at a call site that misreads it.
#
# usage: factory_tree_halt VERB UNIT TREE
#   VERB is the journal word and it also chooses the exit code, because the two halting paths leave
#   different ones and both are already load-bearing: 4 is what a failed tick returns and what the
#   generated unit's `SuccessExitStatus` names, 2 is what the conflict pass returns to skip a unit.
#   Returning them from here rather than from each call site is the point -- the mapping is one case
#   statement the probe asserts for both verbs, instead of a `return 4` and a `return 2` that nothing
#   runs.
#
# Returns 0 only when the unit may proceed. The call site is
# `factory_tree_halt TICK-HALTED "$unit" "$tree" || return $?` and nothing else: that shape is pinned
# by the probe too, because a site that asks the question and drops the `|| return $?` is a site that
# reads the answer and throws it away, which is the same bug as misreading it with different syntax.
factory_tree_halt() {
	local verb=$1 unit=$2 tree=$3 state=${4:-} finding=${5:-}
	local dirt dirt_rc=0 how
	if [ "$state" = confirmed ]; then
		# The caller has already established the finding, and the tree may no longer show it. Since
		# #248 that is the normal case for the streak halt: the quarantine succeeds, the tree is clean
		# by the time the halt is reached, and re-asking "is it dirty?" answers no -- measured, the
		# streak leg journaled TREE-STREAK and then the pass continued with the queue unhalted. A halt
		# that depends on a condition it is about to remove is not a halt.
		dirt_rc=3
		dirt=${finding:-the work was already quarantined}
	else
		dirt=$(factory_tree_dirt "$tree") || dirt_rc=$?
		[ "$dirt_rc" -ne 1 ] || return 0
	fi

	# The action has to match the finding. "Commit it or discard it" is right for a half-done edit and
	# wrong for a tree git could not read, where there is nothing to commit and the next thing to do is
	# make the index readable again.
	case "$dirt_rc" in
	2) how="make the tree readable and its index rewritable, then \`git -C $tree update-index --refresh\` to see what is really there" ;;
	*) how="resolve the work it names (commit it to its branch, or discard it on purpose)" ;;
	esac
	case "$verb" in
	TICK-HALTED)
		log_event "$verb $unit $tree has uncommitted work (${dirt:-no reason returned}); refusing to work in it"
		;;
	*)
		log_event "$verb $unit $tree has uncommitted work (${dirt:-no reason returned}); refusing to merge into it"
		;;
	esac
	printf '%s\n' "a working tree was found with uncommitted work: $tree -- what it holds: ${dirt:-nothing, but the verdict could not be read}. $how, then \`rm $FACTORY_HALT\` to resume" \
		>"$FACTORY_HALT"
	# A halt nobody can see is a halt nobody resolves. The marker is a file in a directory no one
	# opens; the unit's own issue or pull request is where someone is already looking.
	factory_notice_halt "$unit"
	case "$verb" in
	TICK-HALTED) return 4 ;;
	*) return 2 ;;
	esac
}

# --------------------------------------------------------------------------- quarantine
#
# A worktree a dead tick left dirty used to stop the queue, and stopping it bought nothing: the edit
# sat there, uncommitted, and 22 units froze for 15h57m overnight 8->9 Nov while 115 identical
# `HALTED` lines went into a journal nobody was reading (issue #248, `~/prj/wt/issue-247`, four
# modified files and 801 insertions). The tree needed to be clean. That is not a person's job when
# the contents can be mirrored first, so the halt became the fallback rather than the reflex.
#
# The rule, and it is the whole function: clean a tree only after its contents exist somewhere other
# than the tree, and never report a mirror that was not written and read back. A refusal is meant to
# leave the tree as it was found, and it is honest to say how much of that the code buys: the arms
# after `git stash push` undo themselves with `factory_tree_unstash`, so the sentence is true only if
# that pop succeeds. If it does not, the tree is clean and the only copy is in `refs/stash` under the
# sha the refusal prints -- a recoverable outcome, not the advertised one. Before the stash nothing is
# moved at all, and a tree that is mid-merge is in that first group: `git stash push -u` refuses a
# half-merged index (`could not write index` / `needs merge`) and leaves `MERGE_HEAD` and every stage
# entry intact, so a tree stuck mid-merge keeps halting, with a `TREE-QUARANTINE-REFUSED` sentence in
# front of it and the resolution still there for a person. An auto-cleaner that can eat work is worse
# than the halt it replaced -- this repository keeps `~/prj/at-risk/` precisely because `/tmp` already
# ate some.
#
# The plumbing was measured on git 2.53.0, in a linked worktree, before any of this was written:
#
#   `git stash push -u -m <msg>` takes tracked edits *and* untracked files, leaves
#   `git status --porcelain` empty, and its subject carries the message handed to it -- which is how
#   the caller tells the stash on top of `refs/stash` is the one it just made and not an old one.
#   `git stash show -p --binary --include-untracked <sha>` renders both halves: 2 files out of 2 for
#   a tree holding one edit plus one new file, and 1 out of 1 for a stash holding only an untracked
#   file -- the case a plain `git diff` loses entirely, which is why the mirror is rendered out of the
#   stash and never out of the working tree. `--binary` is not decoration and the size guard cannot
#   see its absence: without it a binary path renders as the 45-byte sentence
#   `Binary files a/shot.png and b/shot.png differ`, so the mirror is non-empty, the tree is cleaned,
#   and the journal names a patch that
#   restores nothing -- and `git apply` refuses the *whole* patch, text edits included, with
#   `cannot apply binary patch … without full index line`. Measured on git 2.53.0 for a modified
#   tracked binary and a new untracked one; with the flag both come back as `GIT binary patch` and a
#   `git apply` into a fresh clone of the base restores them byte for byte. It matters here because
#   `docs/screenshots/360/*.png` are committed, so a killed capture tick leaves exactly that shape.
#
# usage: factory_tree_quarantine UNIT TREE
# stdout: `stash=<sha> mirror=<path>`, both belong in the journal line: a mirror nobody can locate
#         is a halt with extra steps.
# exit:   0 the tree is clean now and the mirror is on disk
#         1 there was nothing to quarantine; nothing was touched
#         2 refused -- nothing was touched, so the caller must halt
factory_tree_quarantine() {
	local unit=$1 tree=$2
	local dirt_rc=0 err="" sha="" subject="" patch="" tmp="" restore="" confirm="" confirm_rc=0
	local why=""

	if [ -z "$unit" ] || [ -z "$tree" ]; then
		printf 'quarantine refused: wants a unit and a tree, got %s/%s\n' "${unit:-nothing}" "${tree:-nothing}" >&2
		return 2
	fi
	# `git` itself decides whether this is a repository, because that is the question the action asks.
	# `factory_valid_repo` is not it and was the first guess: it validates a GitHub `owner/name` slug
	# (one slash, no nested path) for a gh argument, so it refuses every real worktree path -- measured,
	# `quarantine refused: /tmp/…/repo is not a repository`, on the fixture's own repo, three legs down.
	if ! git -C "$tree" rev-parse --git-dir >/dev/null 2>&1; then
		printf 'quarantine refused: %s is not a repository git will answer for\n' "$tree" >&2
		return 2
	fi

	# The same verdict the halt reads, so the two arms cannot disagree about what "dirty" means.
	# rc 1 is clean, rc 2 is a tree git would not answer for: neither is anything to stash, and the
	# unreadable one must not be touched at all -- `git stash` against an unwritable index is how a
	# half-written index is born.
	factory_tree_dirt "$tree" >/dev/null || dirt_rc=$?
	if [ "$dirt_rc" -eq 1 ]; then
		return 1
	elif [ "$dirt_rc" -ne 0 ]; then
		printf 'quarantine refused: the tree could not be read (rc=%s), so nothing was touched\n' "$dirt_rc" >&2
		return 2
	fi

	if ! err=$(git -C "$tree" stash push -u -m "supervisor quarantine $unit $(date -u +%Y-%m-%dT%H:%M:%SZ)" 2>&1); then
		printf 'quarantine refused: git stash failed :: %s\n' "$(printf '%s' "$err" | tr '\n\r' '  ' | cut -c1-160)" >&2
		return 2
	fi

	# `refs/stash` rather than `stash@{0}` because the file is the fact and the rev-parse is the
	# reading of it; a `stash push` that printed "No local changes to save" leaves whatever was
	# already there, which the subject check below is the only thing that catches.
	if ! sha=$(git -C "$tree" rev-parse -q --verify refs/stash 2>/dev/null); then
		printf 'quarantine refused: git stash exited 0 and left no refs/stash\n' >&2
		return 2
	fi
	subject=$(git -C "$tree" log -1 --format=%s "$sha" 2>/dev/null)
	# A substring, not a prefix, and that is a measurement: `stash push -m` writes the subject as
	# `On <branch>: <message>`, so a prefix test on the message matches nothing and every honest
	# quarantine is then refused as somebody else's work. Caught by the leg written for it, which is
	# the only reason it is not shipped that way -- a green gate would not have noticed, because the
	# refusal path also cleans nothing and halts, and the queue keeps looking halted either way.
	case "$subject" in
	*"supervisor quarantine $unit "*) : ;;
	*)
		printf 'quarantine refused: stash %s is not ours (subject: %s)\n' "${sha:0:9}" "${subject:-no subject}" >&2
		return 2
		;;
	esac

	# Did the stash actually take everything? Asked of the harness's one verdict, not of
	# `git status --porcelain`: the selftest refuses that read in any shipped script (H-0040 -- on the
	# exclude-file shapes #228 measured it exits 128 printing nothing), and a selftest leg caught it here
	# on the first run. Asking the verdict is also the better question: the confirmation should agree
	# with the tool that decided there was work.
	#
	# The exit codes are the verdict's, and they are not boolean: 0 is DIRTY, 1 is CLEAN, 2 is UNKNOWN.
	# Written as `if ! confirm=$(factory_tree_dirt …)` this refused the two arms that mean "nothing is
	# left" and accepted the one that means "the work is still here" -- caught by the legs, which is the
	# only reason it is not shipped that way. Ask for the number.
	confirm=$(factory_tree_dirt "$tree") && confirm_rc=0 || confirm_rc=$?
	if [ "$confirm_rc" -eq 0 ]; then
		printf 'quarantine refused: git stash exited 0 and %s is still dirty (%s); the work is in stash %s\n' \
			"$tree" "${confirm:-git named nothing}" "${sha:0:9}" >&2
		return 2
	elif [ "$confirm_rc" -ne 1 ]; then
		# An unreadable read is a refusal: the alternative is reporting a successful rescue over a tree
		# nobody has confirmed is clean, which is the claim this whole function exists to earn.
		printf 'quarantine refused: git stash exited 0 and the tree could not be read back (%s); the work is in stash %s\n' \
			"${confirm:-git gave no reason}" "${sha:0:9}" >&2
		return 2
	fi

	# Mirror it, then believe it. An empty patch is a refusal, not a success: it is what a stash that
	# held nothing renders as, and a zero-byte mirror is exactly the file that looks like a rescue.
	# Emptiness is the necessary half and not the sufficient one -- a payload-free rendering of a binary
	# tree is 45 bytes of git telling you it declined, which is not empty. `--binary` above is what
	# makes the bytes actually be in the file; the leg named `the mirror carries its binary` proves it
	# by applying the mirror to a fresh clone and comparing digests, not by counting bytes here.
	if ! mkdir -p -- "$FACTORY_AT_RISK" 2>/dev/null; then
		printf 'quarantine refused: %s cannot be created\n' "$FACTORY_AT_RISK" >&2
		restore=$(factory_tree_unstash "$tree" "$sha")
		[ -z "$restore" ] || printf 'quarantine: %s\n' "$restore" >&2
		return 2
	fi
	patch="$FACTORY_AT_RISK/$(basename -- "$tree")-quarantine-$(date -u +%Y%m%dT%H%M%SZ)-${sha:0:9}.patch"
	tmp="$patch.tmp.$$"
	# Redirect ORDER is the sentence. `>"$tmp" 2>&1` points stderr at the FILE -- the copy this arm
	# deletes two lines later -- and leaves the capture empty, so every mirror refusal printed `… :: `
	# with nothing after the colon. Measured 10 Nov against the shipped function, with `git stash show`
	# refusing: the journal read `no mirror could be written to /…/repo-quarantine-….patch :: `. The
	# worse half is the other direction: with stderr merged into the mirror, a read that exits 0 while
	# writing nothing to stdout still fills `$tmp` with its warning, the `[ ! -s ]` guard calls that a
	# patch, and the run journals a mirror that restores nothing -- H-0063's hazard, arriving through the
	# merge instead of through a binary. `2>&1 >"$tmp"` sends stderr to this command substitution's pipe,
	# where the printf can quote it, and stdout to the file, where only the patch belongs.
	if ! err=$(git -C "$tree" stash show -p --binary --include-untracked "$sha" 2>&1 >"$tmp"); then
		printf 'quarantine refused: rendering stash %s failed :: %s\n' "${sha:0:9}" \
			"$(printf '%s' "${err:-git exited non-zero and said nothing}" | tr '\n\r' '  ' | cut -c1-140)" >&2
		rm -f -- "$tmp"
		restore=$(factory_tree_unstash "$tree" "$sha")
		[ -z "$restore" ] || printf 'quarantine: %s\n' "$restore" >&2
		return 2
	elif [ ! -s "$tmp" ]; then
		# A different cause, and it used to share one sentence with the arm above: git answered with its
		# success status and produced no bytes. That is a stash that holds nothing this rendering can
		# carry, not a git that refused, and the fix for the two is not the same command. Say which.
		why='git exited 0 and wrote no bytes'
		[ -z "$err" ] ||
			why="$why; it said: $(printf '%s' "$err" | tr '\n\r' '  ' | cut -c1-120)"
		printf 'quarantine refused: stash %s rendered an empty mirror :: %s\n' "${sha:0:9}" "$why" >&2
		rm -f -- "$tmp"
		restore=$(factory_tree_unstash "$tree" "$sha")
		[ -z "$restore" ] || printf 'quarantine: %s\n' "$restore" >&2
		return 2
	fi
	if ! mv -f -- "$tmp" "$patch" 2>/dev/null || [ ! -s "$patch" ]; then
		printf 'quarantine refused: %s was written but cannot be read back\n' "$patch" >&2
		rm -f -- "$tmp"
		restore=$(factory_tree_unstash "$tree" "$sha")
		[ -z "$restore" ] || printf 'quarantine: %s\n' "$restore" >&2
		return 2
	fi

	printf 'stash=%s mirror=%s\n' "${sha:0:9}" "$patch"
	return 0
}

# Undo a quarantine that could not be completed, so "refused" means the tree is as it was and not
# half-moved. The caller has already been told to halt, so a failure here is not fatal -- but it is
# said, with the stash named, because after this point the only copy of the work is in `refs/stash`
# and a person needs its sha more than they need a clean exit status.
#
# usage: factory_tree_unstash TREE SHA  ->  stdout: nothing on success, one sentence on failure
factory_tree_unstash() {
	local tree=$1 sha=$2 top="" out=""
	# `refs/stash` is repo-global, not per-worktree: measured on git 2.53.0, a primary tree and a linked
	# worktree of the same repository answer the SAME sha for `refs/stash`, and one `git stash list`
	# shows both units' entries. A bare `git stash pop` therefore restores whatever is on top, which
	# after another unit's accident is somebody else's work, restored into this unit's tree. The subject
	# check in the caller is the right check and runs too early -- it proves the entry was ours when we
	# made it, not that it is still on top when we undo. `git stash pop "$sha"` is refused (`is not a
	# stash reference`, measured), so the pin is the re-check: pop only while the top is still ours, and
	# otherwise say where the work is rather than reaching past it. (`git stash apply "$sha"` does take
	# a non-top entry -- measured -- and is what a person is pointed at, because it restores the entry
	# without dropping anyone else's.)
	top=$(git -C "$tree" rev-parse -q --verify refs/stash 2>/dev/null)
	if [ -n "$top" ] && [ "$top" != "$sha" ]; then
		printf 'the work is in stash %s of %s and was NOT put back: stash %s is on top of it, and a bare pop would restore that one instead. Put this one back with `git stash apply %s`.\n' \
			"${sha:0:9}" "$tree" "${top:0:9}" "${sha:0:9}"
		return 0
	fi
	out=$(git -C "$tree" stash pop 2>&1) || {
		printf 'the work is still in stash %s of %s; it could not be put back :: %s\n' \
			"${sha:0:9}" "$tree" "$(printf '%s' "$out" | tr '\n\r' '  ' | cut -c1-140)"
		return 0
	}
	return 0
}

# Tell whoever owns the unit that the loop has stopped, once per FACTORY_HALT_NOTICE_AFTER.
#
# This exists because of one sentence in issue #248: nobody watches a user unit's start results after
# a reboot. `systemctl --user status` renders a deliberate halt as `code=exited, status=8`, which
# reads as a crash, and `factory/HALT` is a file in a directory only an operator opens. So the halt
# now goes onto the object that is stuck -- `gh issue comment` answers for an issue and for a pull
# request alike (measured 8 Nov on #240, the verb that survives `gh pr edit` dying on the retired
# `projectCards` field) -- and the journal records whether it landed.
#
# usage: factory_notice_halt UNIT   (0 always: a notice that cannot be posted must not mask the halt)
factory_notice_halt() {
	local unit=$1 number="" tmp="" out="" rc=0
	[ "$DRY_RUN" = "1" ] && return 0
	number=${unit##*:}
	case "$number" in
	('' | *[!0-9]*) return 0 ;;
	esac
	[ "$(factory_age "$unit" halt-notice)" -ge "$FACTORY_HALT_NOTICE_AFTER" ] || return 0

	factory_paths
	tmp="$FACTORY_RUN/halt-notice.$$"
	{
	# One printf with one format, and the whole sentence inside it. Written as a `\`-continued chain of
	# printf arguments, shellcheck reads the first fragment as a format with no placeholder and reports
	# that the rest of the sentence is *ignored* -- a notice to a human that silently loses its second
	# half is the failure class this harness keeps being about.
	printf 'The delivery supervisor has **halted the whole queue** on this unit.\n\n```\n%s\n```\n\nEvery other unit is stopped with it: the halt switch is read before triage, so nothing else is picked up, reviewed or merged until this is resolved. Nothing has been discarded and nothing will be -- say what should happen to what the tree holds and the queue resumes the moment the marker is removed.\n' \
		"$(factory_halt_reason)"
	} >"$tmp" 2>/dev/null || {
		rm -f -- "$tmp"
		return 0
	}

	out=$(gh issue comment "$number" --body-file "$tmp" 2>&1) || rc=$?
	if [ "$rc" -eq 0 ]; then
		factory_ledger_now "$unit" halt-notice
		log_event "HALT-NOTICED $unit #$number"
	else
		# No ledger stamp: the next pass has to try again. The halt is already logged by its own verb,
		# so this line carries only the failed write and what gh said about it.
		log_event "HALT-NOTICE-FAILED $unit #$number gh exit=$rc :: $(printf '%s' "$out" | tr '\n\r' '  ' | cut -c1-140)"
	fi
	rm -f -- "$tmp"
	return 0
}

# What a dirty tree found mid-pass actually costs.
#
# Mirror it, clean it, let the rest of the queue run; and if either half cannot be done, halt with the
# tree untouched exactly as before. A dirty unit loses its turn instead of everyone else's evening.
#
# The streak is the exception, and the reason it exists: a SECOND quarantine of the same unit says
# something keeps dying in that tree, and a third guess is how 801 lines of half-finished work get
# thrown away. So the streak halts -- now with the notice above making sure somebody hears it.
#
# usage: factory_tree_stop VERB UNIT TREE
# exit:  0 this unit's tree is clean and the pass carries on;
#        4 (tick) or 2 (conflict) the pass stops, as `factory_tree_halt` decided.
# The counter is what turns "halt on the second accident" into a rule that cannot eventually freeze a
# two-year-old unit for an honest accident it has never had before -- so it has to come back down, and
# where it comes back down is the whole design. A resolved conflict clears it here (`CONFLICT-RESOLVED`
# in `factory.sh`, the only progress a `pr:` unit can make); a tick unit's streak is ended by
# `factory_quarantine_after_tick` once the tick has run and the tree has been read, NOT at
# `TICK-START` -- see that function for the measurement that moved it.
factory_quarantine_reset() { # unit
	factory_ledger_set "$1" quarantine-count 0
}

# The end of a streak, asked at the only moment it has an answer: after the tick, of the tree.
#
# The review of #250 measured what the previous shape did. `factory_tick` ran
# `factory_tree_stop` -> `TICK-START` -> `factory_quarantine_reset` in one pass, so for a tick unit the
# order was always quarantine(1) then reset(0): the counter was cleared before the tick that could have
# proved the pattern, and `attempt=2` was unreachable on the tick path. Five passes over a tree whose
# tick dies at the ceiling every time printed `attempt=1` five times and halted nothing -- the streak
# existed, was wired, and could not fire for the one outage it was written for. The legs did not see it
# because they called the increment twice with nothing between them; what was never composed was the
# counter and the tick that follows it. Those legs now drive this order instead.
#
# Why the tree's verdict and not the tick's exit status: the question the streak asks is "did work
# appear in this tree again?", and both endings answer it -- a session that exits 0 and a session
# killed at the ceiling are distinguished by what they leave behind, which is exactly what the next
# pass would have to quarantine. UNKNOWN keeps the streak armed rather than clearing it: an unreadable
# tree is not evidence that the tree is fine, and the next pass refuses at `factory_tree_stop` before
# anything is stashed, so arming costs a halt that was the right answer anyway.
#
# Not guarded on `--dry-run`, because there is nothing to guard: a rehearsal never reaches
# `factory_tick` at all (`factory.sh`'s dispatcher prints and continues), and a dead guard here would
# be a claim about a path that does not exist.
#
# usage: factory_quarantine_after_tick UNIT TREE
# exit:  0 always. What the tick achieved is the tick's own answer; this one must not overwrite it.
factory_quarantine_after_tick() {
	local unit=$1 tree=$2 verdict="" rc=0
	verdict=$(factory_tree_dirt "$tree") && rc=0 || rc=$?
	case "$rc" in
	1)
		factory_quarantine_reset "$unit"
		log_event "TREE-AFTER-TICK $unit streak=cleared the tree is clean, so this tick did not repeat the accident"
		;;
	0)
		log_event "TREE-AFTER-TICK $unit streak=armed the tick left ${verdict:-git named nothing}; the next quarantine of this unit is the second and halts"
		;;
	*)
		log_event "TREE-AFTER-TICK $unit streak=armed the tree could not be read after the tick (${verdict:-git gave no reason}); the streak stays armed on an unreadable verdict"
		;;
	esac
	return 0
}

factory_tree_stop() {
	local verb=$1 unit=$2 tree=$3
	local q="" qrc=0 n="" dirt="" dirt_rc=0 qfinding="" qerr="" qerr_file=""

	# A rehearsal must not rehearse a rescue. Under `--dry-run` nothing is stashed, nothing is
	# mirrored and no marker is written -- a dry run that halted the real queue would be the same
	# mistake `factory/` already records about the ledger ("print, don't do" cannot be partial) --
	# but the answer stays the halt's answer, so a dry run cannot report a pass that would have run.
	if [ "$DRY_RUN" = "1" ]; then
		log_event "DRY-RUN TREE-QUARANTINE $unit $tree -- dirt left alone, no mirror, no halt file"
		if [ "$verb" = TICK-HALTED ]; then
			return 4
		fi
		return 2
	fi

	# What the quarantine is about, in the verdict's own words -- not a count from `git status`.
	# `naive_dirt_test_refused` in the selftest refuses a shipped script that asks git the stat-cache
	# question directly (H-0040, #228: on the exclude-file shapes it measures, that read exits 128
	# printing nothing), and a count written for a log line is exactly that read. `factory_tree_dirt`
	# is the one verdict, and its finding names the paths, which is also what a reader wants.
	#
	# The exit code is kept as well as the finding, and it is the CODE that discriminates (#267).
	# `|| true` threw it away, and that is what let one sentence cover three histories: rc 1 is "the
	# tree is clean", rc 0 is "dirty, here are the paths", rc 2+ is "git would not answer" -- and a
	# `qrc` of 1 from the action is compatible with all three. `H-0099` keeps all three apart rather
	# than folding them, because a line that names the wrong history in the rescue log is the defect
	# being fixed, not a detail of it. `dirt` alone cannot do this: the `:-` fallback two arms below
	# exists precisely because an rc-0 verdict can return no paths, so an empty `$dirt` is not the
	# same statement as "the verdict said clean".
	dirt=$(factory_tree_dirt "$tree" 2>/dev/null) || dirt_rc=$?
	qerr_file="$FACTORY_RUN/quarantine-error.$$"
	q=$(factory_tree_quarantine "$unit" "$tree" 2>"$qerr_file") || qrc=$?
	# Read it and remove it here, before any arm returns. The first version left the `rm -f` below the
	# success path's `return 0`, so every happy quarantine left a `run/quarantine-error.<pid>` behind --
	# measured, one per pass. A zero-byte file a pass is not an outage; a run directory that slowly
	# fills with other processes' pids is how an operator loses the ability to read it, and the file
	# names the wrong thing to ask about.
	qerr=$(factory_git_said "$qerr_file")
	rm -f -- "$qerr_file"

	if [ "$qrc" -eq 0 ]; then
		n=$(( $(factory_ledger_int "$unit" quarantine-count) + 1 ))
		factory_ledger_set "$unit" quarantine-count "$n"
		factory_ledger_now "$unit" quarantined-at
		log_event "TREE-QUARANTINED $unit ${dirt:-no finding returned} attempt=$n $q"
		if [ "$n" -ge 2 ]; then
			log_event "TREE-STREAK $unit attempt $n without a tick in between; halting rather than guessing again"
		else
			return 0
		fi
	elif [ "$qrc" -eq 1 ]; then
		# The action found nothing to stash. Clean is clean; what the line has to get right is WHY,
		# and only the verdict two lines above knows that, so it is asked (#267). Each arm carries a
		# machine-readable `cause=` token beside its sentence -- the house shape for a reason a leg
		# has to pin (`attempt=`, `streak=`, `mirror=`) -- so the selftest pins the ARM that was taken
		# and not the prose, and rewording a sentence cannot silently retire a leg.
		case "$dirt_rc" in
		1)
			# The healthy path. `factory_tick` calls `factory_tree_stop` on every tree before every
			# tick, so this is what a pass writes for a tree nothing ever touched -- and the one
			# sentence used to cover it, so the rescue log reported a rescue nobody had performed: 6
			# such lines in one day's `factory/log/supervisor.log`, against 0 dirty verdicts (#267).
			log_event "TREE-CLEAN $unit $tree cause=never-dirty nothing was dirty; nothing was quarantined"
			;;
		0)
			# The arm the sentence was written for, and the only case where it is true: the verdict the
			# halt reads and the verdict the action read disagreed, and the action's is the later one.
			# The finding is named because what made the false line undebuggable is that a reader could
			# not tell from the journal which arm any line had taken.
			log_event "TREE-CLEAN $unit $tree cause=verdict-disagreed was found dirty and is clean now; nothing was quarantined (${dirt:-git named nothing})"
			;;
		*)
			# Neither dirty nor clean, so neither sentence above is true and the line says so instead
			# of borrowing one. Reachable only when the two reads disagree in THIS direction: the
			# quarantine answers 1 solely on its own verdict saying clean (`common.sh:1366`), so an
			# unreadable verdict here is a second, different disagreement -- and `H-0099` records it as
			# the one arm of the three no leg here reaches.
			log_event "TREE-CLEAN $unit $tree cause=verdict-unreadable the dirt verdict was unreadable and the action found nothing; nothing was quarantined (${dirt:-git gave no reason})"
			;;
		esac
		return 0
	else
		q="$qerr"
		log_event "TREE-QUARANTINE-REFUSED $unit $tree ${q:-git gave no reason}"
	fi

	# Both halt arms keep the shape the probe pins (`factory_tree_halt … || return $?`,
	# `halt_call_sites_in`): a site that asks the question and drops the answer is the same bug as
	# misreading it, and that is true one level down as well. Each call is therefore one line ending in
	# `|| return $?` and naming its verb literally -- the first version wrapped the streak call across
	# two lines to pass the finding, and the pinned shape check refused it. It is right to: a check that
	# cannot see a call is a check that will be deleted, so the call stays visible rather than the check
	# being widened.
	#
	# A streak passes its own finding with `confirmed`, because the quarantine that just succeeded
	# removed the evidence the halt would otherwise go and look for. The refusal arm re-asks: there the
	# work is still on disk and the verdict's own words are the better sentence.
	if [ "$qrc" -eq 0 ]; then
		qfinding="quarantined $n times without a tick in between; copies in ${q:-nowhere the journal names}"
	fi
	case "$verb" in
	TICK-HALTED)
		if [ "$qrc" -eq 0 ]; then
			factory_tree_halt TICK-HALTED "$unit" "$tree" confirmed "$qfinding" || return $?
		else
			factory_tree_halt TICK-HALTED "$unit" "$tree" || return $?
		fi
		;;
	*)
		if [ "$qrc" -eq 0 ]; then
			factory_tree_halt CONFLICT-HALTED "$unit" "$tree" confirmed "$qfinding" || return $?
		else
			factory_tree_halt CONFLICT-HALTED "$unit" "$tree" || return $?
		fi
		;;
	esac
	return 0
}

# ------------------------------------------------- the code the loop is actually running
#
# #249: `install-timer.sh` renders `ExecStart` as a *path*, so the timer runs whatever that tree
# happens to have checked out, and nothing in the harness ever compared that checkout to the trunk.
# Measured 8 Nov: the tree was at `efa4e29` while `origin/main` was 121 commits later, and every
# merge in between was invisible to the loop whose job is to act on them. The journal proved it
# without meaning to -- the halt sentence printed there for sixteen hours was `efa4e29`'s wording,
# not `main`'s. So the tree's position against the trunk is not decoration, it is part of *which
# code a given pass is*, and it belongs in the same line that already says who the pass is.
#
# Two different trees can be "the code under test" and they are not the same directory:
#
#   the pass    the tree the running script was read from -- `$FACTORY_HARNESS`.
#   an install  the tree the unit it is about to write will exec -- `$FACTORY_ROOT`, which is what
#               `@REPO@` in the template becomes and what `ExecStart` will name.
#
# They are equal only while the harness lives in the primary checkout, which is the case that has
# never once been true on this repository (H-0009). So the tree is an argument here and never a
# global: a call site that picks the wrong one prints a sha for code that is not running, and the
# print is the kind nobody re-checks because it looks like a version number.
#
# What the verdict is measured against: `origin/$FACTORY_BASE` **as this checkout last learned it**.
# There is no `git fetch` in here and that is a decision rather than an oversight (H-0056): a fetch
# is a blocking network call inside a pass whose whole budget is spent before a tick opens, and this
# repository already owns the hazard of a read that parks (#879, where a blocking `open` on a FIFO
# fails the enforcement loop *open*). It is also the same clock the orphan pass already reads --
# `triage.sh`'s `rev-parse origin/main` -- and it is the reading that found #249 in the first place:
# `git rev-list --count HEAD..origin/main` answered 121 off the ref, with no fetch of its own. What
# the choice costs is written in `factory/DECISIONS.md` rather than hidden here: a repository nobody
# ever fetches reads `0 behind` forever.

# Where the trunk's remote-tracking ref lives, as one string both the compare and the sentence use.
factory_supervisor_base() { printf 'refs/remotes/origin/%s' "${FACTORY_BASE:-main}"; }

# `commit` or `commits`. The sentences #249 prescribes are written in the plural and grepped in the
# plural; a journal line that reads "is 1 commits ahead" is the kind of detail that makes a person
# doubt the number beside it, and the number is the whole content of the line.
factory_plural() {
	if [ "$1" = 1 ]; then
		printf 'commit'
	else
		printf 'commits'
	fi
}

# factory_supervisor_position <tree>
#   Where <tree> stands against the trunk, as five tab-separated fields on one line:
#
#       <short-sha>   <branch>   <behind>   <ahead>   <dirty>
#
#   exit 0  all five known. A detached head puts the word `detached` in <branch> rather than an
#           empty field: an empty field is what slides every column after it one place left, which
#           is the reason the supervisor's own row delimiter is U+001F and not a tab.
#   exit 2  git could not read the tree at all. All five are `unknown`/`?`, and a caller must not
#           print `0 behind` -- an unreadable tree is not a current one.
#   exit 3  the tree is readable but `origin/$FACTORY_BASE` is not, so <behind> and <ahead> are `?`
#           while the sha and the branch are still worth printing.
#
# <dirty> is a *count of paths* with uncommitted work, or `?` when the read itself failed. It is not
# the halt's verdict: `factory_tree_dirt` stays the one guard that stops a unit (#228), and this is
# an informational field in a journal line. It refreshes the index first for the reason #228
# documents -- the stat cache is what made `status` answer from mtime rather than from bytes -- and
# the refresh is best-effort, because a tree whose index cannot be written still has paths to count
# and the count being high is the safe direction for a field that only informs.
factory_supervisor_position() {
	local tree=$1 rc=0 out='' paths='' short='' branch='' behind='?' ahead='?' dirty='?'
	local base_shown
	base_shown=$(factory_supervisor_base)
	base_shown=${base_shown#refs/remotes/}

	rc=0
	out=$(git -C "$tree" rev-parse --short HEAD 2>&1) || rc=$?
	if [ "$rc" -ne 0 ]; then
		printf 'unknown\tunknown\tunknown\tunknown\t?\n'
		return 2
	fi
	short=$(printf '%s\n' "$out" | head -n 1)

	# `|| branch=detached`, and not a bare call: `symbolic-ref` exits 1 on a detached head, which is
	# a state a tree is legitimately in (a bisect, a pinned install, every one of this harness's own
	# unit trees once its branch merged), and `set -e` is on in every caller.
	branch=$(git -C "$tree" symbolic-ref --short -q HEAD 2>/dev/null) || branch='detached'
	[ -n "$branch" ] || branch='detached'

	# Best-effort, on purpose. When the index cannot be written the refresh fails and the count that
	# follows may include stat-only differences -- an over-count in a field that informs and halts
	# nothing, which is the direction to err. The refresh is also why the three reads below cost 6s
	# instead of 5min on the tree with 195 MB of testdata (#228, measured).
	git -C "$tree" update-index --refresh >/dev/null 2>&1 || true

	# Counted the way #228 decides a halt -- staged, unstaged, untracked, as three *content* reads --
	# and not from `git status --porcelain`. The selftest's own `no stat-question dirt test` leg
	# refuses a shipped script that answers a question of `status`, on the grounds that the porcelain
	# path answers CLEAN for real work when the stat cache is stale; `dirty=` is a question of content,
	# so it takes the same reads. Deduped, because a conflicted path is in both diff lists and would
	# otherwise count twice against one path.
	#
	# This is a count for a journal line; `factory_tree_dirt` stays the one verdict that halts a unit,
	# because it also weighs git's warnings and the unmerged statuses. What the two share is the rule,
	# not the function, and the rule is the thing the leg enforces.
	rc=0
	paths=$(
		{ git -C "$tree" diff --cached --name-only &&
			git -C "$tree" diff --name-only &&
			git -C "$tree" ls-files --others --exclude-standard; } 2>/dev/null
	) || rc=$?
	if [ "$rc" -eq 0 ]; then
		dirty=$(printf '%s\n' "$paths" | grep . | sort -u | grep -c . || true)
	fi
	[ -n "$dirty" ] || dirty='?'

	rc=0
	git -C "$tree" rev-parse --verify --quiet "$base_shown" >/dev/null 2>&1 || rc=$?
	if [ "$rc" -ne 0 ]; then
		printf '%s\t%s\t?\t?\t%s\n' "$short" "$branch" "$dirty"
		return 3
	fi

	# Counted from `HEAD` and the ref, never from a sha interpolated into the range: a short sha is
	# the shortest *unambiguous* prefix, which is a fact about today's object database, and a range
	# built out of one can silently mean a different commit after a fetch.
	rc=0
	out=$(git -C "$tree" rev-list --count "HEAD..$base_shown" 2>/dev/null) || rc=$?
	if [ "$rc" -eq 0 ]; then
		behind=$(printf '%s' "$out" | tr -dc '0-9')
		rc=0
		out=$(git -C "$tree" rev-list --count "$base_shown..HEAD" 2>/dev/null) || rc=$?
		if [ "$rc" -eq 0 ]; then
			ahead=$(printf '%s' "$out" | tr -dc '0-9')
		fi
	fi
	[ -n "$behind" ] || behind='?'
	[ -n "$ahead" ] || ahead='?'

	printf '%s\t%s\t%s\t%s\t%s\n' "$short" "$branch" "$behind" "$ahead" "$dirty"
	return 0
}

# The sentences for the pass's journal: one per fact, so a journal grep is the whole query. The AT
# line is unconditional -- a pass that does not say what it is running cannot be audited after the
# fact -- and the other three appear only for the state each one names. Nothing here halts anything:
# #249's remedy is to make the staleness loud, and moving a tree the timer is executing from is
# editing a script while bash is reading it. That stays a human act.
factory_supervisor_lines() {
	local tree=$1 rc=0 pos='' sha='' branch='' behind='' ahead='' dirty='' base=''
	pos=$(factory_supervisor_position "$tree") || rc=$?
	IFS=$'\t' read -r sha branch behind ahead dirty <<EOF
$pos
EOF
	base=$(factory_supervisor_base)
	base=${base#refs/remotes/}

	if [ "$rc" -eq 2 ]; then
		printf 'SUPERVISOR-UNKNOWN git could not read %s, so the code this pass is running is not known\n' "$tree"
		return 0
	fi
	printf 'SUPERVISOR-AT %s dirty=%s\n' "$sha" "$dirty"
	if [ "$behind" = '?' ]; then
		printf 'SUPERVISOR-NOBASE %s cannot be resolved in %s, so the loop cannot say whether it is running merged code\n' \
			"$base" "$tree"
		return 0
	fi
	if [ "$behind" != "0" ]; then
		printf 'SUPERVISOR-STALE %s is %s %s ahead; the loop is not running merged code\n' \
			"$base" "$behind" "$(factory_plural "$behind")"
	fi
	if [ "$ahead" != "0" ]; then
		# A fork and a stale tree coexist, and each needs its own sentence because each has its own
		# fix: one is a fast-forward, the other is not available on a tree holding commits the trunk
		# does not. Saying `merge --ff-only` here would send an operator to a command git refuses.
		printf 'SUPERVISOR-FORKED %s holds %s %s %s does not have; a fast-forward is not available on a fork\n' \
			"$branch" "$ahead" "$(factory_plural "$ahead")" "$base"
	fi
	return 0
}

# ------------------------------------------- the checkout the installed unit actually execs (#260)
#
# At 13:38Z on 10 Nov `--install` refused a tree, and the sentence it refused with was false about the
# unit that was installed:
#
#     install-timer: refusing to install a tree that is not the trunk.
#       /home/rodney/prj/parental-lockout is at b9b2e7c on main: 91 commits behind origin/main.
#       The unit execs /home/rodney/prj/parental-lockout/factory/factory.sh, so this tree IS the supervisor
#
# `systemctl --user cat` answered `ExecStart=/home/rodney/prj/wt/factory-harness/factory/factory.sh`.
# Nothing in the harness had ever read the installed unit -- `grep -nE "systemctl.*cat|systemctl.*show"`
# over the three shipped scripts returned one hit, and it asked the *timer* for `ActiveState`. So the
# staleness guard #252 shipped to shout about a stale loop checked a tree the loop never ran:
# `SUPERVISOR-STALE` printed zero times in the 3,861-line journal of the tree that was actually running
# (`e0d27d3`, 38 commits behind, `grep -c factory_tree_quarantine` = 0), while a 23-unit queue sat
# halted for 5h02m10s on a dirty tree the merged code knows how to quarantine.
#
# Everything below is therefore a read of the file systemd reads, before anything says a word about it.

# The directory user units live in. A function and not a global so a leg can point it at a fixture
# directory instead of at the operator's own units; the default is what `install-timer.sh` writes to,
# and the two have to agree or the pass reads a file the timer does not.
factory_units_dir() {
	printf '%s\n' "${FACTORY_UNITS_DIR:-${XDG_CONFIG_HOME:-${HOME:-/var/tmp}/.config}/systemd/user}"
}

# The service unit's file name, in the order the two authorities are known:
#
#   1. `$FACTORY_SERVICE_UNIT`, which the unit exports about itself -- the only answer that survives a
#      custom `--name`, and the reason `install-timer.sh` now writes the line;
#   2. the repository's name, which is what `install-timer.sh` derives when `--name` is not given;
#   3. the checkout's own directory name, for `--status`, which answers before `factory_preflight` has
#      resolved the remote.
#
# Arm 3 agrees with arm 2 for every checkout this repository has had. When it does not, the line that
# results prints the path it looked at, which is what makes a wrong guess checkable rather than fatal.
factory_service_unit() {
	if [ -n "$FACTORY_SERVICE_UNIT" ]; then
		printf '%s\n' "$FACTORY_SERVICE_UNIT"
	elif [ -n "$FACTORY_NAME" ]; then
		printf '%s-factory.service\n' "$FACTORY_NAME"
	else
		printf '%s-factory.service\n' "$(basename -- "${FACTORY_ROOT%/}")"
	fi
}

# factory_installed_tree [UNIT_FILE]
#   Reads the installed unit and prints the checkout its `ExecStart` script lives in.
#
#     exit 0  the tree. One `ExecStart=`, naming `<tree>/factory/factory.sh`.
#     exit 1  no unit file at that path, or a file naming no command: nothing is armed.
#     exit 4  something is there that cannot be read as a regular file. What would run is UNKNOWN, and
#             unknown is never printed as "nothing is installed" -- the #228 polarity, a read that
#             failed is not evidence of the clean state.
#     exit 3  the file names something other than this harness's script, or carries more than one
#             `ExecStart=` (a hand-written unit, a drop-in): the raw value(s) on stdout, comma-joined,
#             because the sentence has to quote what is really there instead of guessing a tree.
#
#   Why the file and not `systemctl --user cat`/`show`, which is how the operator found it: the manager
#   answers from the copy it loaded, so both are stale until `daemon-reload` -- and the installer writes
#   the file and reloads in the same breath (`install-timer.sh`, `systemctl_user daemon-reload`), so on
#   any tree this harness installed the two agree. The file is also the answer on a box with no user
#   manager, where the bus call fails: a check that fails on the machine where the queue is broken is
#   the check that was missing. `factory_timer_state` still asks the manager, because what it reports --
#   when systemd will fire next -- is in no file.
factory_installed_tree() {
	local file=${1:-} values='' count=0 value='' tree=''
	[ -n "$file" ] || file="$(factory_units_dir)/$(factory_service_unit)"
	# A dangling symlink is not "nothing installed": it is a unit file that points nowhere, which is a
	# different sentence and a different fix.
	if [ -L "$file" ] && [ ! -e "$file" ]; then
		return 4
	fi
	if [ ! -e "$file" ]; then
		return 1
	fi
	if [ ! -r "$file" ] || [ ! -f "$file" ]; then
		return 4
	fi

	# Every `ExecStart=` line, value only: `sub` takes the assignment away, then systemd's own execution
	# prefixes (`-` ignore-failure, `@`, `+`, `:`) go so `/bin/sh` is not read as a prefix. Nothing here
	# interprets quoting -- a unit whose ExecStart is quoted is not this generator's, and lands in the
	# exit-3 arm with its raw text, which is the honest answer.
	values=$(awk '/^[[:space:]]*ExecStart[[:space:]]*=/ { sub(/^[^=]*=/, ""); sub(/^[-@+!:]+/, ""); print }' \
		"$file" 2>/dev/null) || return 4
	count=$(printf '%s\n' "$values" | grep -c . || true)
	if [ "$count" != 1 ]; then
		printf '%s\n' "$values" | tr '\n' ',' | sed -e 's/,$//'
		return 3
	fi
	value=$(printf '%s\n' "$values" | head -n 1)
	case "$value" in
	*/factory/factory.sh) tree=${value%/factory/factory.sh} ;;
	esac
	# `${value%/factory/factory.sh}` is empty only for a script at the filesystem root, which is not a
	# checkout of anything; it falls through with the raw value rather than printing an empty tree.
	if [ -z "$tree" ]; then
		printf '%s\n' "$value"
		return 3
	fi
	printf '%s\n' "$tree"
	return 0
}

# factory_unit_lines <this-tree> [UNIT_FILE]
#   The pass's sentences about the installed unit: one per fact, so a journal grep is the whole query,
#   exactly as `factory_supervisor_lines` does it. Same polarity too -- it reports and never halts.
#   Moving a tree the timer executes from is editing a script while bash is reading it, and #249 left
#   that a human act; #260 changes only what gets said about it.
#
#   `UNIT-AT <tree>` is on every pass, the way `SUPERVISOR-AT` is: a read nobody prints is a read that
#   cannot be told apart from a read that is not there.
factory_unit_lines() {
	local this=${1:-$FACTORY_HARNESS} file=${2:-} installed='' rc=0
	[ -n "$file" ] || file="$(factory_units_dir)/$(factory_service_unit)"
	rc=0
	installed=$(factory_installed_tree "$file") || rc=$?
	case "$rc" in
	1)
		printf 'UNIT-UNINSTALLED unit=%s this=%s -- nothing is armed at that path, so this pass was not started by the timer (#260)\n' \
			"$file" "$this"
		;;
	4)
		printf 'UNIT-UNREADABLE unit=%s -- something is there that this pass cannot read, so what the timer would exec is unknown (#260)\n' \
			"$file"
		;;
	3)
		printf 'UNIT-FOREIGN unit=%s exec=%s -- that ExecStart is not this harness'"'"'s script, so the unit does not name a tree a pass can name (#260)\n' \
			"$file" "${installed:-<no ExecStart line>}"
		;;
	*)
		if [ "$installed" = "$this" ]; then
			printf 'UNIT-AT %s\n' "$installed"
		else
			printf 'UNIT-DIVERGED installed=%s this=%s -- the unit execs one checkout and this pass is running from another, so the code under test is not the code the loop will run next (#260)\n' \
				"$installed" "$this"
		fi
		;;
	esac
	return 0
}

# --------------------------------------------------- a stop that delivered nothing (#247)
#
# `exit=0` from `opencode run` means the session closed its turn. It does not mean the turn said
# anything, and it does not mean anything was pushed. Measured over the 196 tick logs this box has
# kept: six ticks closed with exit 0 and no pull request, and their last lines are three different
# things.
#
#   two end on a tool call the harness never dispatched, because the name is not a tool.
#     `20261108T233014Z.unclaimed:246.log` ends `<function=executae>`, 710 lines, 115 s alive;
#     `20261108T175614Z.unclaimed:239.log` is 20 lines and ends inside its very first call,
#     `<function=tools.opencode>`. When the CLI cannot match the model's markup to a tool it prints
#     the markup as text, the text becomes the reply, the turn closes, and the run exits 0.
#   two end on a turn opener with nothing after it -- `20261108T215923Z.unclaimed:241.log`, 6411
#     lines whose last line is `> factory · default-model-fast`, over a clean tree holding four
#     finished commits.
#   two end on ordinary prose, having stopped mid-plan without saying so.
#
# The exit code separates none of them, so the reason has to be read off the end of the log. That is
# all this reads: the shape of the last emission, never a guess about what the model meant.
#
# The polarity matters as much as the arms, and both halves of it are measured. Of the 26 ticks that
# DID deliver, exactly one ends on an undispatched call -- `20261108T060114Z.address_review:231.log`,
# whose last turn is nothing but `<function=<function_43 edit>` -- and it delivered because it had
# already pushed at line 1757, three lines before the turn that died. So this is a verdict on the last
# emission and never on the unit, which is why it is only ever printed by an arm that has already read
# that no pull request exists, and never instead of that read.
#
# Scoping to the final turn is load-bearing, and the tick that opened this issue is the proof:
# `20261108T234413Z.unclaimed:247.log` carries nine `<function=` hits before its last turn, because the
# issue it was handed quotes `<function=executae>` verbatim. A whole-file grep for the markup would
# have reported a misspelled tool call for a tick that misspelled nothing. The test is where the markup
# sits, not whether the file contains it -- and a brief that quotes a failure is the normal case in
# this queue, not an edge.
#
# The line the CLI prints to open an assistant turn, and the one the existing `TICK-FAILED` reason
# already refuses to quote as a cause. It is deliberately not pinned to one agent or model: the
# tick's own agent name and model sit in the middle of it.
FACTORY_TURN_OPENER='^[[:space:]]*>[[:space:]]'

# What the tick's own last emission says about why it stopped.
#
# The name in the markup arm is taken verbatim, not matched against a list of tools: `executae` is a
# typo, `tools.opencode` is a namespace used as a name, and `<function_43 edit` is mangled past
# recognition, and all three are facts the journal has to carry. A maintained list of legal names
# would go stale the day a tool is renamed and would then report the *list* as the cause -- #1044's
# failure with a different noun.
#
# Stdout is one sentence, bounded, safe to interpolate into a journal line. The return code is the
# verdict the caller words:
#   0  the turn was never delivered -- markup nothing dispatched, or a turn that emitted nothing.
#      `exit=0` is the CLI's, and this says out loud that it is not a delivery.
#   1  nothing of the kind; stdout quotes the log's last line, which may or may not explain anything.
#   2  the log could not be read, so no cause is invented for the stop.
# The text the model wrote in place of a function name: the last `<function=` in the given text, up to
# the tag's own `>`, with everything non-printable deleted and the result bounded. It goes on one
# journal line and it is the model's own bytes, so it is scrubbed and capped rather than trusted.
# Nothing in it is matched against a list of tools -- see factory_stop_reason.
_factory_call_name() { # text -> the name the model wrote, or nothing
	local raw=''
	raw=$(printf '%s\n' "$1" | grep -a -o '<function=[^>]*' | tail -n 1 || true)
	printf '%s' "${raw#'<function='}" | tr -dc '[:print:]' | tr -s ' ' | cut -c1-60
}

factory_stop_reason() { # logfile -> the reason on stdout
	local log=$1 clean='' block='' last='' first='' name=''
	if [ -z "$log" ] || [ ! -r "$log" ]; then
		printf 'the tick log could not be read (%s), so why it stopped is unknown\n' "${log:-<none>}"
		return 2
	fi
	clean=$(tr -d '\033\r' <"$log" 2>/dev/null) || clean=''
	# The test is "is there a non-blank byte", not "is the string empty": a log of nothing but newlines
	# is a non-empty string and every reader downstream would be handed a blank as if it were content.
	if [ -z "${clean//[[:space:]]/}" ]; then
		printf 'the tick log %s holds nothing -- the session wrote no output at all\n' "${log##*/}"
		return 2
	fi
	# The log's last line that says anything at all. A tick that died exactly at a turn opener -- the
	# #241 shape -- has nothing AFTER that opener, so this is read from the whole file and not out of
	# the final block: read from the block, the line the leg exists for is empty and the arm falls
	# through to quoting the previous turn's prose. That is the whole difference between naming the
	# failure and inventing one.
	last=$(printf '%s\n' "$clean" | grep -v '^[[:space:]]*$' | tail -n 1 || true)
	# The final block is everything after the last turn opener. With no opener in the file at all --
	# a tick that died before printing one -- the whole log is the block, which is the honest reading:
	# there was no earlier turn for the text to belong to.
	block=$(printf '%s\n' "$clean" | awk '
		/^>[[:space:]]/ { n = NR }
		{ lines[NR] = $0 }
		END { for (i = n + 1; i <= NR; i++) print lines[i] }
	')
	first=$(printf '%s\n' "$block" | grep -v '^[[:space:]]*$' | head -n 1 || true)
	name=$(_factory_call_name "$block")

	# The two markup shapes: the emission that closed the run *is* a call, either from its first line
	# or -- when the model wrote a sentence and then the call -- from its closing tag.
	case "$first" in
	'<tool_call>'* | '<function='*)
		printf 'it stopped on a tool call nothing dispatched -- `%s` is not a tool, so the CLI printed the call as text and the turn closed as a reply\n' \
			"${name:-<no name>}"
		return 0
		;;
	esac
	case "$last" in
	'</tool_call>' | '</function>')
		printf 'it stopped on a tool call nothing dispatched -- `%s` is not a tool, and the call markup, not a summary, is what closed the run\n' \
			"${name:-<no name>}"
		return 0
		;;
	esac
	if [ -n "$last" ] && printf '%s\n' "$last" | grep -q "$FACTORY_TURN_OPENER"; then
		printf 'it opened a turn and emitted nothing at all -- no call, no text, no summary\n'
		return 0
	fi
	last=$(printf '%s\n' "$clean" | grep -v '^[[:space:]]*$' |
		grep -v "$FACTORY_TURN_OPENER" | tail -n 1 || true)
	# No guard here for "and if that is empty too". The blank-only log returned above, and any log with
	# a non-blank line that is not a turn opener keeps that line through both greps -- so arm (c) always
	# has something to quote. A branch that cannot be reached is a branch no leg can prove, which is the
	# rule this repository applies to scriptlets (#1703), so it is absent rather than defended.
	printf 'its last line reads: %s\n' "$(printf '%s' "$last" | cut -c1-160)"
	return 1
}

# The journal line for a tick that stopped clean and delivered nothing. Split out of the call site so
# the sentence is something a leg can assert whole, rather than one that only exists once a model has
# been paid to fail in the right shape.
factory_stop_marker() { # verb unit branch logfile -> the journal line on stdout
	local verb=$1 unit=$2 branch=$3 log=$4 why='' rc=0 see=''
	why=$(factory_stop_reason "$log") || rc=$?
	# Naming the log is only an offer when there is one to open. Pointing a reader at a file the
	# detector could not read is the same invented pointer this harness keeps writing up.
	see="see $(basename -- "${log:-<no log>}")"
	if [ "$rc" = "2" ]; then
		# Pointing a reader at a file the detector could not read is the same invented pointer this
		# harness keeps writing up.
		see='no log to read'
	fi
	if [ "$rc" = "0" ]; then
		printf '%s %s exit=0 refused as completion (%s) :: %s :: no pull request on %s -- it stays where it was\n' \
			"$verb" "$unit" "$see" "$why" "${branch:-<no branch>}"
		return 0
	fi
	printf '%s %s exit=0 with no pull request on %s (%s) :: %s -- it stays where it was\n' \
		"$verb" "$unit" "${branch:-<no branch>}" "$see" "$why"
	return 0
}

# Mirror the commits a stopped tick left behind, before anything else can take them.
#
# What this survives is not a lost disk -- the bundle lands on the same disk as the worktree. What it
# survives is what actually happened twice: PR #238 merged the *pushed* head at `17:47:33Z` while the
# local branch kept six more commits no pull request ever carried, and `issue-241` ended a tick with
# four finished commits reachable from one directory on one machine and from no queue that reads pull
# requests. A branch is a name someone can delete; a bundle is a file.
#
# It is a *prerequisite* bundle, `origin/main..<branch>`, and the form was chosen on measurement:
#   * full history is 17,647,835 bytes for this repository's 687 commits and re-copies the 10 MB
#     `plock` blob on every write (#1264). The prerequisite form is 28,601 bytes for the real
#     `issue-241` (4 commits) and 18,551 bytes for `issue-235` (6).
#   * a prerequisite bundle does not clone on a machine holding nothing -- measured,
#     `error: Repository lacks these prerequisite commits`. So the recovery is a clone of the remote
#     and then `git fetch <bundle> refs/heads/<branch>`, measured to reproduce the tip exactly
#     (selftest leg `the bundle restores the tip where origin/main is all there is`). The
#     prerequisite is a commit on `main`, and a force-push to `main` is denied to everything here, so
#     the recovery cannot drift out from under the bundle.
#
# The write is read back, because this harness's most-repeated failure is a claim about a write that
# was never re-read (#1180): `git bundle verify` runs against the tree that holds the prerequisite,
# and a file that does not verify is reported as `PRESERVE-UNVERIFIED`, never as `PRESERVED`.
#
# Return: 0 mirrored, 1 nothing was owed, 2 refused. They are three codes because they are three
# different sentences, and "nothing ahead" reading as "mirrored" is how a safety net becomes fiction.
factory_preserve_unpushed() { # unit tree branch -> 0 mirrored, 1 nothing ahead, 2 refused
	local unit=$1 tree=$2 branch=$3
	local ahead='' tip='' path='' why='' rc=0
	if [ -z "$tree" ] || [ ! -d "$tree" ]; then
		log_event "PRESERVE-FAILED $unit no tree to read $branch from -- nothing was mirrored"
		return 2
	fi
	if [ -z "$branch" ]; then
		log_event "PRESERVE-SKIPPED $unit no branch, so there is nothing to mirror"
		return 1
	fi
	if ! ahead=$(git -C "$tree" rev-list --count "origin/$FACTORY_BASE..$branch" 2>/dev/null); then
		log_event "PRESERVE-FAILED $unit how far $branch is ahead of origin/$FACTORY_BASE could not be read -- nothing was mirrored"
		return 2
	fi
	case "$ahead" in
	'' | *[!0-9]*)
		log_event "PRESERVE-FAILED $unit origin/$FACTORY_BASE..$branch answered \`$ahead', not a count -- nothing was mirrored"
		return 2
		;;
	esac
	if [ "$ahead" -eq 0 ]; then
		return 1
	fi
	tip=$(git -C "$tree" rev-parse --short "$branch" 2>/dev/null) || tip=unknown
	path="$FACTORY_AT_RISK/branch-$(factory_slug "$unit")-${tip}.bundle"
	if ! mkdir -p -- "$FACTORY_AT_RISK" 2>/dev/null; then
		log_event "PRESERVE-FAILED $unit $ahead commit(s) on $branch are at risk and $FACTORY_AT_RISK could not be created -- nothing was mirrored"
		return 2
	fi
	if ! why=$(git -C "$tree" bundle create "$path" "origin/$FACTORY_BASE..$branch" 2>&1); then
		# `git bundle create` refuses an empty bundle itself (`fatal: Refusing to create empty
		# bundle.`), which is the same verdict the count above already reached. Getting here means the
		# two reads disagree, so git's own sentence goes in the journal rather than a paraphrase.
		log_event "PRESERVE-FAILED $unit $ahead commit(s) on $branch are at risk and no bundle was written :: $(printf '%s\n' "$why" | tr -d '\033\r' | grep -v '^[[:space:]]*$' | tail -n 1 | cut -c1-160)"
		return 2
	fi
	if ! why=$(git -C "$tree" bundle verify "$path" 2>&1); then
		log_event "PRESERVE-UNVERIFIED $unit $path exists but git refuses to verify it :: $(printf '%s\n' "$why" | tr -d '\033\r' | grep -v '^[[:space:]]*$' | tail -n 1 | cut -c1-160) -- do not trust this bundle to hold the work"
		return 2
	fi
	log_event "PRESERVED $path ($ahead commit(s) on $branch at $tip ahead of origin/$FACTORY_BASE) -- recover with \`git clone <remote> r.git && git -C r.git fetch $path refs/heads/$branch\`"
	return 0
}

# --------------------------------------------------------------------------- actions
#
# One place where anything that changes the world is printed or run, so --dry-run is a property
# of the whole script rather than of each call site.

run_action() {
	if [ "$DRY_RUN" = "1" ]; then
		printf 'DRY-RUN  %s\n' "$*"
		return 0
	fi
	log_event "ACTION $*"
	# `$?` inside `if ! "$@"; then` is the status of the *negated test*, which is 0 whenever the
	# command failed -- so every failure this file has ever logged read `FAILED (0)`, including three
	# `gh pr merge` calls that had in fact merged. Capture the status before anything can overwrite it.
	local rc=0
	"$@" || rc=$?
	if [ "$rc" -ne 0 ]; then
		log_event "FAILED ($rc) $*"
		return 1
	fi
}
