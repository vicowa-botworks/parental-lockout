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
	local verb=$1 unit=$2 tree=$3
	local dirt dirt_rc=0 how
	dirt=$(factory_tree_dirt "$tree") || dirt_rc=$?
	[ "$dirt_rc" -ne 1 ] || return 0

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
	case "$verb" in
	TICK-HALTED) return 4 ;;
	*) return 2 ;;
	esac
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
