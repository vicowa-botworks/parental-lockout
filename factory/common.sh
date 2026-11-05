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

# Cadences, seconds. A unit waiting on a reviewer is re-poked at most this often; the timer
# itself may fire far more often, because a wait-state tick spends no model tokens.
FACTORY_REVIEW_POKES=${FACTORY_REVIEW_POKES:-1800}
# After this long in any non-progress state, one comment with the evidence goes on the PR. It is
# a notice, not a park: the unit stays in the queue and keeps being retried.
FACTORY_ESCALATE_AFTER=${FACTORY_ESCALATE_AFTER:-86400}

FACTORY_LOG=${FACTORY_LOG:-$FACTORY_DIR/log}
FACTORY_LEDGER=${FACTORY_LEDGER:-$FACTORY_DIR/ledger}
FACTORY_RUN=${FACTORY_RUN:-$FACTORY_DIR/run}
FACTORY_HALT=${FACTORY_HALT:-$FACTORY_DIR/HALT}

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

# --------------------------------------------------------------------------- ledger
#
# One file per (unit, fact), content is a single line. Not one JSON document, on purpose: two
# writers appending to one JSON file is a corrupt file, and the supervisor and a tick are both
# writers. `cat` and `date` are the whole interface, so nothing has to parse to be right.

factory_ledger_path() { printf '%s/%s.%s\n' "$FACTORY_LEDGER" "$1" "$2"; }

factory_ledger_get() {
	local f
	f=$(factory_ledger_path "$1" "$2")
	[ -f "$f" ] && tr -d '\n' <"$f"
	return 0
}

factory_ledger_set() {
	factory_paths
	printf '%s\n' "$3" >"$(factory_ledger_path "$1" "$2")"
}

factory_ledger_now() { factory_ledger_set "$1" "$2" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"; }

factory_ledger_clear() { rm -f -- "$(factory_ledger_path "$1" "$2")"; }

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

# The tree for a branch, creating it from origin/main when absent. Prints the path.
factory_worktree() {
	local branch=$1 slug path main
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
	if ! "$@"; then
		log_event "FAILED ($?) $*"
		return 1
	fi
}
