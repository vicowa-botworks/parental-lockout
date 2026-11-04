#!/usr/bin/env bash
# shellcheck shell=bash disable=SC2016
# factory.sh -- the supervisor. It owns the loop; a model never does.
#
# One invocation: read the queue, do every mechanical step that needs no judgement, run at most
# FACTORY_MAX_TICKS model ticks, print what it did, exit. A systemd timer or a `while :; sleep`
# re-arms it. Nothing remembers anything between invocations except the ledger and the decision
# log, both on disk -- so a killed supervisor costs one pass, and a restarted one re-derives the
# world from GitHub.
#
# The split is the point. Of the five pull requests that sat motionless on 4 Nov, four needed an
# action that is one `gh` call -- request a reviewer, merge an approved PR -- and needed no model
# at all. A design that routes those through a session is a design where they wait for a session.
#
#   needs_reviewer     request a review        bash, throttled
#   merge_now          merge, after reading    bash, never a model
#   conflict           merge origin/main in    bash when git resolves it, else one tick
#   awaiting_review    wait, re-poke, notify   bash, throttled
#   unclaimed          implement               one tick
#   address_review     respond to review       one tick
#   checks_failing     fix red checks          one tick
#   branch_without_pr  assess stranded work    one tick
#
# Exit codes, because a timer must be able to tell these apart:
#   0  the pass is over and nothing failed
#   3  the queue could not be read, so nothing was attempted
#   4  a tick failed -- a model run that did not finish inside its ceiling
#   7  another supervisor holds the lock, so this one did nothing
#   8  the halt file exists
#
# usage: factory.sh [--dry-run] [--max-ticks N] [--budget SECS] [--once] [--status]
set -euo pipefail

# shellcheck source=./common.sh
# shellcheck disable=SC1091
. "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/common.sh"

usage() {
	cat <<'USAGE'
usage: factory.sh [--dry-run] [--max-ticks N] [--budget SECS] [--once] [--status]

--dry-run     every action is printed as `DRY-RUN  gh …` and nothing is written to GitHub or to
              git. A tick is described instead of started, because a tick commits.
--max-ticks N model ticks this pass may start (default FACTORY_MAX_TICKS=3).
--budget S    stop opening new ticks after S seconds (default FACTORY_BUDGET=5400).
--once        at most one tick; the same as --max-ticks 1.
--status      print the halt state and the ledger, then exit without reading GitHub.

A tick runs the `factory` agent in a per-unit worktree through
`opencode run --auto --agent factory`. The instruction tells it to stop at the pull request and
never to merge: merging is this script's job and it is the one step a tick is not trusted with.
USAGE
}

while [ $# -gt 0 ]; do
	case "$1" in
	--dry-run) DRY_RUN=1 ;;
	--max-ticks)
		[ $# -ge 2 ] || die "--max-ticks wants a number"
		FACTORY_MAX_TICKS=$2
		shift
		;;
	--budget)
		[ $# -ge 2 ] || die "--budget wants seconds"
		FACTORY_BUDGET=$2
		shift
		;;
	--once) FACTORY_MAX_TICKS=1 ;;
	--status)
		factory_paths
		if factory_halted; then
			printf 'halt:    YES -- %s\n' "$(factory_halt_reason)"
		else
			printf 'halt:    no\n'
		fi
		printf 'repo:    %s\n' "$(git -C "$FACTORY_ROOT" remote get-url origin 2>/dev/null)"
		printf 'trees:   %s\n' "$FACTORY_TREES"
		printf 'journal: %s\n' "$FACTORY_LOG/supervisor.log"
		printf '\nledger:\n'
		shopt -s nullglob
		for f in "$FACTORY_LEDGER"/*; do
			printf '  %-34s %s\n' "${f##*/}" "$(tr -d '\n' <"$f")"
		done
		shopt -u nullglob
		exit 0
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		printf 'factory: unknown argument: %s\n' "$1" >&2
		usage >&2
		exit 2
		;;
	esac
	shift
done

if [ "$DRY_RUN" = "1" ]; then
	printf 'factory: DRY RUN -- nothing is written to GitHub, to git, or by a model\n'
fi

factory_preflight

if factory_halted; then
	log_event "HALTED $(factory_halt_reason)"
	exit 8
fi

# --------------------------------------------------------------------------- handlers

# A review request, at most once per FACTORY_REVIEW_POKES per pull request.
#
# Why this exists: `reviewRequests` was empty on #209, #212, #214 and #215 three days after they
# opened, and REST `/pulls/N/requested_reviewers` was empty too. Nothing had ever asked. Neither
# `issue-check` nor `review-loop` requests a reviewer, so a pull request could sit in
# REVIEW_REQUIRED forever with every check reading green.
factory_request_review() {
	local pr=$1 last age
	last=$(factory_ledger_get "pr:$pr" last-poke)
	age=$(factory_age "pr:$pr" last-poke)
	if [ -n "$last" ] && [ "$age" -lt "$FACTORY_REVIEW_POKES" ]; then
		log_event "POKE-SKIPPED pr:$pr asked ${age}s ago (< ${FACTORY_REVIEW_POKES}s)"
		factory_maybe_notice "pr:$pr" awaiting_review
		return 0
	fi
	# GitHub answers 422 when that reviewer is already requested. That is the state wanted, not a
	# failure to report, but it must not be counted as a fresh request either -- so the two are
	# logged differently and the ledger is stamped in both cases, which is what keeps the throttle
	# honest.
	if [ "$DRY_RUN" = "1" ]; then
		printf 'DRY-RUN  gh pr edit %s --add-reviewer %s\n' "$pr" "$FACTORY_REVIEWER"
		log_event "WOULD-POKE pr:$pr request review from $FACTORY_REVIEWER"
		return 0
	fi
	if run_action gh pr edit "$pr" --add-reviewer "$FACTORY_REVIEWER"; then
		factory_ledger_now "pr:$pr" last-poke
		log_event "POKED pr:$pr requested review from $FACTORY_REVIEWER"
		return 0
	fi
	if gh api "repos/$FACTORY_OWNER/$FACTORY_NAME/pulls/$pr/requested_reviewers" \
		--jq '[.users[].login] | index($who) != null' --raw-field who="$FACTORY_REVIEWER" \
		2>/dev/null | grep -q true; then
		factory_ledger_now "pr:$pr" last-poke
		log_event "POKE-ALREADY pr:$pr $FACTORY_REVIEWER already holds a request"
		factory_maybe_notice "pr:$pr" awaiting_review
		return 0
	fi
	log_event "POKE-FAILED pr:$pr gh pr edit --add-reviewer failed"
	return 1
}

# Merge, with every fact read back from the API in the same breath as the merge.
#
# The read-back is not a courtesy. #1180 records writing "All three are APPROVED and CLEAN.
# Merging in order" and the very next call returning REVIEW_REQUIRED/BLOCKED for all three. So the
# four facts are re-read here, at the moment of the merge, and the triage pass that ran some
# seconds ago is not trusted. `gh pr view` rather than `gh pr merge --admin`, which would merge
# anything it is pointed at.
factory_merge() {
	local pr=$1 branch=$2
	local state decision mergeable merge_state checks

	if ! state=$(gh pr view "$pr" --json reviewDecision,mergeable,mergeStateStatus 2>/dev/null); then
		log_event "MERGE-SKIPPED pr:$pr the pull request could not be read back"
		return 1
	fi
	decision=$(printf '%s' "$state" | jq -r '.reviewDecision // ""')
	mergeable=$(printf '%s' "$state" | jq -r '.mergeable // ""')
	merge_state=$(printf '%s' "$state" | jq -r '.mergeStateStatus // ""')
	checks=$(factory_checks "$pr")

	if [ "$decision" != "APPROVED" ] || [ "$mergeable" != "MERGEABLE" ] ||
		[ "$merge_state" != "CLEAN" ] || [ "$checks" != "pass" ]; then
		log_event "MERGE-REFUSED pr:$pr decision=$decision mergeable=$mergeable state=$merge_state checks=$checks"
		factory_maybe_notice "pr:$pr" awaiting_merge
		return 0
	fi

	# The one action here that cannot be undone locally: squash and rebase merge are both disabled
	# on this repository (#1264), so a merge commit is permanent history. It is still done in bash
	# rather than by a tick, because it needs no judgement and a model that is allowed to merge is a
	# model that can merge the wrong thing.
	if [ "$DRY_RUN" = "1" ]; then
		printf 'DRY-RUN  gh pr merge %s --merge --delete-branch\n' "$pr"
		log_event "WOULD-MERGE pr:$pr decision=$decision mergeable=$mergeable state=$merge_state checks=$checks"
		return 0
	fi
	if ! run_action gh pr merge "$pr" --merge --delete-branch; then
		log_event "MERGE-FAILED pr:$pr gh pr merge failed"
		return 1
	fi
	local merged
	merged=$(api_get "pulls/$pr" '.merged | tostring' 'unknown')
	log_event "MERGED pr:$pr merged_field=$merged"
	factory_ledger_clear "pr:$pr" stuck-since
	factory_ledger_clear "pr:$pr" last-notice
	factory_ledger_now "pr:$pr" merged-at
	# The tree goes with the branch. Leaving one behind is how /tmp/opencode acquired six
	# abandoned worktrees, two of them on the same commit.
	if [ -n "$branch" ] && [ -d "$FACTORY_TREES/$(factory_slug "$branch")" ]; then
		git -C "$(factory_main_checkout)" worktree remove --force \
			"$FACTORY_TREES/$(factory_slug "$branch")" 2>/dev/null ||
			log_event "WORKTREE-STALE $FACTORY_TREES/$(factory_slug "$branch") could not be removed"
	fi
	return 0
}

# Bring main into a branch that has fallen behind it.
#
# No model: `git merge origin/main` on a clean tree either resolves or it does not. When it does
# not, leaving a conflicted index behind is the worst outcome -- every later command in that tree
# inherits a half-merged state -- so the merge is aborted and the unit is handed to a tick that can
# read both sides and decide.
factory_resolve_conflict() {
	local pr=$1 branch=$2 tree
	local age
	age=$(factory_age "pr:$pr" conflict-at)
	if [ -n "$(factory_ledger_get "pr:$pr" conflict-at)" ] && [ "$age" -lt 600 ]; then
		log_event "CONFLICT-SKIPPED pr:$pr attempted ${age}s ago"
		return 1
	fi
	factory_ledger_now "pr:$pr" conflict-at

	tree=$(factory_worktree "$branch") || return 1
	if [ -n "$(git -C "$tree" status --porcelain 2>/dev/null)" ]; then
		log_event "CONFLICT-HALTED pr:$pr $tree is dirty -- a person or a dead tick owns it"
		printf '%s\n' "a working tree was found dirty and the supervisor will not touch it: $tree" \
			>"$FACTORY_HALT"
		return 1
	fi

	if ! run_action git -C "$tree" fetch origin main; then
		return 1
	fi
	if [ "$DRY_RUN" = "1" ]; then
		printf 'DRY-RUN  git -C %s merge --no-edit origin/main\n' "$tree"
		return 1
	fi
	if ! git -C "$tree" merge --no-edit origin/main >"$FACTORY_LOG/conflict-$pr.log" 2>&1; then
		git -C "$tree" merge --abort >/dev/null 2>&1 || true
		log_event "CONFLICT-UNRESOLVED pr:$pr origin/main does not merge cleanly into $branch -- queued for a tick"
		return 1
	fi
	if ! run_action git -C "$tree" push origin "HEAD:refs/heads/$branch"; then
		return 1
	fi
	log_event "CONFLICT-RESOLVED pr:$pr merged origin/main into $branch"
	factory_ledger_clear "pr:$pr" stuck-since
	return 0
}

# Nothing to do but wait. The only writing is the ledger and, past the threshold, one comment.
#
# Indefinite by design: a pull request that is merely unreviewed is not a failure, and parking it
# is what loses it. The notice is what makes waiting forever safe -- a human hears about it once a
# day, and the unit never leaves the queue.
factory_wait() {
	local unit=$1 key=$2 class=$3
	local id
	id="$unit:$(factory_slug "$key")"
	[ -n "$(factory_ledger_get "$id" first-seen)" ] || factory_ledger_now "$id" first-seen
	[ -n "$(factory_ledger_get "$id" stuck-since)" ] || factory_ledger_now "$id" stuck-since
	factory_ledger_set "$id" last-class "$class"
	log_event "WAIT $id $class"
	factory_maybe_notice "$id" "$class"
}

# One comment per FACTORY_ESCALATE_AFTER per unit, saying what is blocking and that nothing was
# parked. This is the only place the supervisor speaks to a human.
factory_maybe_notice() {
	local id=$1 class=$2 pr=${1#pr:}
	[ "$pr" != "$id" ] || return 0
	[ -n "$(factory_ledger_get "$id" stuck-since)" ] || return 0
	[ "$(factory_age "$id" stuck-since)" -ge "$FACTORY_ESCALATE_AFTER" ] || return 0
	[ "$(factory_age "$id" last-notice)" -ge "$FACTORY_ESCALATE_AFTER" ] || return 0

	run_action gh pr comment "$pr" --body "**Still waiting, and still polling.** The delivery supervisor has watched this pull request since $(factory_ledger_get "$id" stuck-since) and it is still in \`$class\`. Nothing has been parked. If a maintainer decision is what is missing, say so here and the supervisor will stop asking for the same thing again." ||
		return 0
	factory_ledger_now "$id" last-notice
	log_event "NOTICED $id $class after $(factory_age "$id" stuck-since)s"
}

# --------------------------------------------------------------------------- the tick
#
# One unit, one session, one worktree, one wall-clock ceiling. The session is disposable by
# construction: it is given no memory of previous passes, only what it reads from disk and the API,
# so a killed tick costs one unit and the next pass re-derives everything.

factory_tick() {
	local class=$1 key=$2 detail=$3 branch=$4 issue=$5
	local unit tree log id_line
	unit="$class:$(factory_slug "$key")"

	case "$class" in
	unclaimed)
		branch="issue-$key"
		id_line="Issue #$key: $detail"
		;;
	branch_without_pr)
		branch=$key
		id_line="Stranded branch \`$key\`: $detail. The work is already written. Assess it against the gate, then open its pull request or record in DECISIONS.md why it should be dropped."
		;;
	conflict)
		branch=${branch:-$(api_get "pulls/$key" '.head.ref' '')}
		id_line="Pull request #$key does not merge with main: $detail. The supervisor already tried \`git merge origin/main\` and it conflicted; resolve it deliberately, keeping both intents."
		;;
	checks_failing)
		branch=${branch:-$(api_get "pulls/$key" '.head.ref' '')}
		id_line="Pull request #$key has failing checks: $detail"
		;;
	address_review)
		branch=${branch:-$(api_get "pulls/$key" '.head.ref' '')}
		id_line="Pull request #$key has review feedback newer than its last commit: $detail"
		;;
	*)
		id_line="$class #$key: $detail"
		;;
	esac
	issue=${issue:-}

	if [ -z "$branch" ]; then
		log_event "TICK-SKIPPED $unit no branch to work on"
		return 4
	fi
	if ! tree=$(factory_worktree "$branch"); then
		log_event "TICK-SKIPPED $unit no worktree for $branch"
		return 4
	fi
	if [ -n "$(git -C "$tree" status --porcelain 2>/dev/null)" ]; then
		log_event "TICK-HALTED $unit $tree is dirty from an earlier run; refusing to work in it"
		printf '%s\n' "a worktree was left dirty by a previous tick: $tree" >"$FACTORY_HALT"
		return 4
	fi

	log="$FACTORY_LOG/$(date -u +%Y%m%dT%H%M%SZ).$(factory_slug "$unit").log"
	log_event "TICK-START $unit tree=$tree log=$(basename -- "$log")"

	# `--auto` approves permissions that are not explicitly denied; the deny rules in
	# .opencode/opencode.jsonc are what bounds that, and the `factory` agent denies `question`, so a
	# headless tick cannot stall on an answer nobody is there to give. See the skill for what it is
	# required to do instead: log, checkpoint, choose the recommended option.
	local status=0
	timeout --signal=TERM --kill-after=60 "$FACTORY_TICK_TIMEOUT" \
		"$FACTORY_OPENCODE" run --auto --agent "$FACTORY_AGENT" \
		--title "factory $unit" \
		"$id_line

Work only in this directory: $tree (branch \`$branch\`). Follow .opencode/skills/factory/SKILL.md
exactly: checkpoint before a judgement call, one decision per commit carrying its own \`Decision:\`
trailer, DECISIONS.md updated in the same commit as what it decides, and stop once the pull request
is open or updated. Merging belongs to the supervisor and is denied to you." \
		>"$log" 2>&1 || status=$?

	if [ "$status" -eq 0 ]; then
		log_event "TICK-DONE $unit see $(basename -- "$log")"
		factory_ledger_clear "$unit" stuck-since
		factory_ledger_now "$unit" last-tick
		return 0
	fi
	# 124 is the timeout's own code; anything else is the agent exiting non-zero. Both are a failed
	# unit, and neither is retried with a doubled budget -- the next pass treats it as fresh work.
	log_event "TICK-FAILED $unit exit=$status (124 means the ${FACTORY_TICK_TIMEOUT}s ceiling) see $(basename -- "$log")"
	factory_ledger_now "$unit" last-tick
	return 4
}

# --------------------------------------------------------------------------- the pass

factory_body() {
	local rows rc=0 started=0
	if ! rows=$("$FACTORY_DIR/triage.sh"); then
		log_event "TRIAGE-FAILED the queue could not be read; nothing was attempted"
		return 3
	fi
	# The first line is the column header; the delimiter is U+001F, not a tab, for the reason
	# recorded in triage.sh -- a tab is IFS whitespace and `read` would collapse an empty field and
	# slide every column after it one place left.
	rows=$(printf '%s\n' "$rows" | tail -n +2)
	log_event "PASS units=$(printf '%s\n' "$rows" | grep -c . || true) dry=$DRY_RUN budget=${FACTORY_BUDGET}s"

	local workable='' unit key class detail branch issue
	while IFS=$'\037' read -r unit key class detail branch issue; do
		[ -n "${unit:-}" ] || continue
		[ -n "$(factory_ledger_get "$unit:$(factory_slug "$key")" first-seen)" ] ||
			factory_ledger_now "$unit:$(factory_slug "$key")" first-seen
		case "$class" in
		merge_now)
			factory_merge "$key" "$branch" || rc=4
			;;
		conflict)
			# Git may resolve it by itself. If it cannot, the same unit is queued for a tick, which
			# is the only thing here that can read both sides of a conflict and keep both intents.
			factory_resolve_conflict "$key" "$branch" ||
				workable+="$(printf '%s\037%s\037%s\037%s\037%s\037%s' conflict "$key" "$detail" "$branch" "$issue" "$unit")"$'\n'

			;;
		needs_reviewer)
			factory_request_review "$key" || rc=4
			;;
		checks_failing | address_review | unclaimed | branch_without_pr)
			workable+="$(printf '%s\037%s\037%s\037%s\037%s\037%s' "$class" "$key" "$detail" "$branch" "$issue" "$unit")"$'\n'

			;;
		awaiting_merge | awaiting_review | checks_pending | draft)
			factory_wait "$unit" "$key" "$class"
			;;
		*)
			log_event "UNKNOWN-CLASS $unit/$key class=$class -- no handler, so nothing was done"
			;;
		esac
	done <<<"$rows"

	# Oldest `first-seen` wins, so a stream of new arrivals cannot starve an old unit. A unit seen
	# for the first time sorts last, which is right: the alternative puts an unmeasured unit at the
	# front of every pass.
	workable=$(printf '%s' "$workable" | sed -e '/^[[:space:]]*$/d')
	local best='' best_age='' best_key='' class2 key2 detail2 branch2 issue2
	while IFS=$'\037' read -r class2 key2 detail2 branch2 issue2 unit2; do
		[ -n "${class2:-}" ] || continue
		local age
		age=$(factory_age "$unit2:$(factory_slug "$key2")" first-seen)
		if [ -z "$best_age" ] || [ "$age" -lt "$best_age" ] ||
			{ [ "$age" -eq "$best_age" ] && [ "$key2" -lt "$best_key" ] 2>/dev/null; }; then
			best_age=$age
			best_key=$key2
			best=$(printf '%s\037%s\037%s\037%s\037%s' "$class2" "$key2" "$detail2" "$branch2" "$issue2")
		fi
	done <<<"$workable"

	if [ -n "$best" ]; then
		local bclass bkey bdetail bbranch bissue
		IFS=$'\037' read -r bclass bkey bdetail bbranch bissue <<<"$best"
		log_event "NEXT $bclass $bkey age=${best_age}s ($bdetail)"
		if [ "$DRY_RUN" = "1" ]; then
			printf 'DRY-RUN  tick unit=%s:%s detail=%s\n' "$bclass" "$bkey" "$bdetail"
		elif [ "$started" -lt "$FACTORY_MAX_TICKS" ]; then
			factory_tick "$bclass" "$bkey" "$bdetail" "$bbranch" "$bissue" || rc=4
			started=$((started + 1))
		fi
	else
		log_event "IDLE no unit needs a model this pass"
	fi

	log_event "END rc=$rc ticks_started=$started"
	printf '\nfactory: %s tick(s), rc=%s. Journal: %s\n' "$started" "$rc" "$FACTORY_LOG/supervisor.log"
	return "$rc"
}

# The lock, taken before anything is read so a second activation cannot interleave its triage with
# the first one's actions.
#
# Not `flock -n 9 || exit 7` written plainly at top level in a script that might be sourced or
# wrapped: the exit has to end the process that holds the descriptor, so the body is a function and
# the shell that owns descriptor 9 runs it. A supervisor that starts, does nothing and reports 0 is
# the quietest possible failure, and this file has seen that shape twice already.
exec 9>"$FACTORY_RUN/lock"
if ! flock -n 9; then
	printf 'factory: another supervisor holds %s, so nothing was done here.\n' "$FACTORY_RUN/lock" >&2
	exit 7
fi

factory_body
exit $?
