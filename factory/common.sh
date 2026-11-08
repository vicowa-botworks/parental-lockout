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

# Is there anything in TREE that nobody has committed? Prints one line naming what it found,
# and nothing when the tree is clean:
#
#   0  there is uncommitted work, and the line says which kind and where
#   1  clean -- every tracked file's bytes match the index, the index matches HEAD, and
#      nothing is untracked
#   2  its state could not be read. The caller must treat that as dirt, not as clean: this
#      verdict gates writing into a tree, and "I could not tell" is not evidence of safety.
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
# git 2.53.0.
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
# Measured there: `refresh`, `diff` and `ls-files --unmerged` answer clean, `diff --cached` and
# `ls-files --others` exit 128, and `status --porcelain` exits 128 printing NOTHING -- so both the
# replaced expression and the diff-plus-untracked prescription answer CLEAN on a tree whose state git
# would not report.
#
# Which read refuses first is a property of the git, not a promise, so each arm checks its own status
# and the first refusal wins. That makes the checks redundant on purpose, and the redundancy was
# measured rather than assumed: hollowing the tracked read's check turns two legs red, hollowing any
# one of the other four turns none red, because every planted shape has at least two reads fail --
# which is what an unreadable tree actually does. A verdict that refused only on the strength of one
# read would be a verdict that had to trust that read, and the point of the exercise is that no read
# here is trustworthy on its own.
#
# The same measurements say what the fix is and is not. A stat-only difference -- the state the
# issue describes -- does not make `status --porcelain` report dirt on this git: status refreshes
# the index in memory and finds the bytes equal, whether or not `.git/index` or even `.git/` is
# writable and whether or not `core.trustctime` is on; six planted variants read clean, and the
# probe says so on its own line rather than pretending to catch it. What IS reproducible here is
# the other direction, and it is the worse one: an unreadable tree answered CLEAN and the tick would
# have worked in it. A false halt costs a pass. A false clean costs the work.
factory_tree_dirt() {
	local tree=$1 rc=0 out="" finding="" raw=""
	git -C "$tree" update-index --refresh >/dev/null 2>&1 || rc=$?
	if [ "$rc" -gt 1 ]; then
		printf 'its state could not be read (git update-index --refresh exited %s)\n' "$rc"
		return 2
	fi

	rc=0
	git -C "$tree" diff --cached --quiet 2>/dev/null || rc=$?
	if [ "$rc" -eq 0 ]; then :; elif [ "$rc" -gt 1 ]; then
		printf 'its staged state could not be read (git diff --cached exited %s)\n' "$rc"
		return 2
	else
		out=$(git -C "$tree" diff --cached --name-only 2>/dev/null | sort -u | head -n 3 | tr '\n' ' ')
		finding="${finding}staged-but-uncommitted: ${out:-and the index itself}"
	fi

	rc=0
	git -C "$tree" diff --quiet 2>/dev/null || rc=$?
	if [ "$rc" -eq 0 ]; then :; elif [ "$rc" -gt 1 ]; then
		printf 'its tracked files could not be read (git diff exited %s)\n' "$rc"
		return 2
	else
		out=$(git -C "$tree" diff --name-only 2>/dev/null | sort -u | head -n 3 | tr '\n' ' ')
		[ -n "$finding" ] && finding="$finding; "
		finding="${finding}modified: ${out:-a tracked file}"
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
	rc=0
	raw=$(git -C "$tree" ls-files --unmerged 2>/dev/null) || rc=$?
	if [ "$rc" -gt 1 ]; then
		printf 'its index could not be read (git ls-files --unmerged exited %s)\n' "$rc"
		return 2
	fi
	if [ -n "$raw" ]; then
		out=$(printf '%s\n' "$raw" | awk '{print $4}' | sort -u | head -n 3 | tr '\n' ' ')
		[ -n "$finding" ] && finding="$finding; "
		finding="${finding}unmerged: ${out:-a conflicted path}"
	fi

	rc=0
	raw=$(git -C "$tree" ls-files --others --exclude-standard 2>/dev/null) || rc=$?
	if [ "$rc" -gt 1 ]; then
		printf 'its untracked files could not be read (git ls-files --others exited %s)\n' "$rc"
		return 2
	fi
	if [ -n "$raw" ]; then
		out=$(printf '%s\n' "$raw" | sort | head -n 3 | tr '\n' ' ')
		[ -n "$finding" ] && finding="$finding; "
		finding="${finding}untracked: ${out:-an untracked file}"
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
