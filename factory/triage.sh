#!/usr/bin/env bash
# shellcheck shell=bash disable=SC2016
# triage.sh -- what is stopping this repository's queue, read from GitHub and git.
#
# Prints one tab-separated row per thing that needs something:
#
#   kind    key       class                detail                         branch  issue
#   pr      206       merge_now            approved, clean, checks pass   …       216
#   pr      215       conflict             CONFLICTING/DIRTY              …       213
#   pr      214       needs_reviewer       no review, nobody asked        …       211
#   pr      206       awaiting_review      answered, requested of …       …       216
#   issue   213       unclaimed            <title>                        -       213
#   branch  fix/x     branch_without_pr    2 ahead of main, no open PR    fix/x   -
#
# `class` is a verdict, not a field. The classes factory.sh calls workable are the ones worth a
# model; the rest are waits, and a wait costs no tokens.
#
# Everything here is derived from the API on this run and nothing else -- no cached queue, no
# remembered state. That is deliberate: the failure this repository keeps hitting is a claim about
# a PR that was true an hour ago (#1180), and a queue file is the same claim with a filename on it.
#
# Exit 0 with rows or without them. Exit 3 when the API or git could not be read -- a classifier
# that reports "nothing to do" because it could not ask is the worst kind of green.
#
# usage: triage.sh [--table] [--pr N]
set -euo pipefail

# shellcheck source=./common.sh
# shellcheck disable=SC1091
. "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/common.sh"

usage() {
	cat <<'USAGE'
usage: triage.sh [--table] [--pr N]

Reads open pull requests, open issues and origin's branches, and prints one TSV row per thing
that needs something:

  kind   one of pr | issue | branch
  key    the number, or the branch name
  class  merge_now | conflict | checks_failing | checks_pending | address_review |
         needs_reviewer | awaiting_review | awaiting_merge | draft | unclaimed |
         branch_without_pr
  detail why that class, with the timestamps the verdict was made from
  branch the PR head / orphan branch
  issue  the issue GitHub says the PR closes, when there is one

--table  same rows, aligned for a person instead of a pipe.
--pr N   the one PR, for "why is this one sitting there".
USAGE
}

TABLE=0
ONLY_PR=

while [ $# -gt 0 ]; do
	case "$1" in
	--table) TABLE=1 ;;
	--pr)
		[ $# -ge 2 ] || die "--pr wants a number"
		ONLY_PR=$2
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		printf 'triage: unknown argument: %s\n' "$1" >&2
		usage >&2
		exit 2
		;;
	esac
	shift
done

factory_preflight

emit() { printf '%s\037%s\037%s\037%s\037%s\037%s\n' "$1" "$2" "$3" "$4" "${5:--}" "${6:--}"; }

# U+001F between fields, for the reason recorded at the read loop below: a tab is IFS whitespace,
# and `read` collapses runs of those, so an empty field silently shifts every column after it.
header=$'kind\tkey\tclass\tdetail\tbranch\tissue'
mheader=$'kind\037key\037class\037detail\037branch\037issue'

# --------------------------------------------------------------------- pull requests
#
# One `gh pr list` carries every PR's own fields; the per-PR calls are only for what the list
# payload does not carry (review timestamps, outstanding requests, head commit date). Keeping the
# bulk read bulk is what makes a wait-state poll cheap enough to run from a timer.

pr_json=$(gh pr list --state open --limit 100 \
	--json number,title,headRefName,isDraft,reviewDecision,mergeable,mergeStateStatus,updatedAt,headRefOid,statusCheckRollup 2>/dev/null) ||
	pr_json=''
[ -n "$pr_json" ] || die "gh pr list failed, so the queue cannot be read"

# The check-rollup verdict comes from $JQ_CHECKS in common.sh, shared with the supervisor, so
# "green" cannot mean one thing to this table and another to the code that merges on it.
pr_rows=$(printf '%s' "$pr_json" | jq -r "
  .[] | [
    .number,
    (.title | gsub(\"[\\t\\r\\n]\"; \" \")),
    .headRefName,
    (.isDraft | tostring),
    (.reviewDecision // \"\"),
    (.mergeable // \"\"),
    (.mergeStateStatus // \"\"),
    (.updatedAt // \"\"),
    .headRefOid,
    ((.statusCheckRollup) | $JQ_CHECKS)
  ] | join(\"\\u001f\")") || die "could not read the pull-request payload"

# Which issue each PR claims, from GitHub's own parser rather than a regex over the body.
# GraphQL wants an explicit `first:` on closingIssuesReferences or the call fails with
# MISSING_PAGINATION_BOUNDARIES (#1256), and REST returns an empty list for this token, so this is
# the only route that answers.
#
# When it fails, the issue half is skipped for this pass rather than guessed. Assuming an issue is
# unclaimed while a PR for it exists opens a second PR for the same work, which is worse than
# doing nothing for ten minutes.
CLOSING_OK=1
closing_json=$(gh api graphql \
	-f query='query($o:String!,$n:String!){repository(owner:$o,name:$n){pullRequests(states:OPEN,first:100){nodes{number closingIssuesReferences(first:20){nodes{number}}}}}}' \
	-f o="$FACTORY_OWNER" -f n="$FACTORY_NAME" \
	--jq '[.data.repository.pullRequests.nodes[]|{n:.number,c:[.closingIssuesReferences.nodes[].number]}]' 2>/dev/null) ||
	closing_json=''
if [ -z "$closing_json" ]; then
	CLOSING_OK=0
	closing_json='[]'
	printf 'triage: closingIssuesReferences unreadable; issues skipped this pass\n' >&2
fi

issue_for_pr() {
	printf '%s' "$closing_json" | jq -r --argjson n "$1" '.[]|select(.n==$n)|.c[0] // empty' 2>/dev/null || true
}

claimed_issues=$(printf '%s' "$closing_json" | jq -c '[.[].c[]]' 2>/dev/null || printf '[]')

# A second claim channel, because GitHub's parser is deliberately narrow: `Closes #207` registers
# a closing reference and `Part of #207; it does not close it` does not. The second form is the one
# this repository writes on purpose (#1256 -- an adjacent inflection of "close" on an umbrella issue
# closes it the moment the merge lands), so a classifier that reads only closing references reports
# the issue as untouched and opens a second PR for work already under review. Measured on this very
# tree: #212 has no closing reference and is the fix for #207.
#
# So an issue counts as claimed if GitHub says a PR closes it OR any open PR body names it. The
# body read is one extra list call, not one per PR.
body_json=$(gh pr list --state open --limit 100 --json number,body 2>/dev/null || printf '[]')
mentioned_issues=$(printf '%s' "$body_json" | jq -c '
  [ .[] | (.body // "") | scan("#[1-9][0-9]*") | ltrimstr("#") | tonumber ] | unique' 2>/dev/null || printf '[]')
claimed_issues=$(jq -cn --argjson a "$claimed_issues" --argjson b "$mentioned_issues" '($a + $b) | unique' 2>/dev/null || printf '%s' "$claimed_issues")

rows=''

# Fields are separated by U+001F, not by a tab, and it is not cosmetics. `read` treats every
# whitespace character in IFS as a separator whose *runs* collapse and whose leading and trailing
# instances are stripped -- and a tab is whitespace. `reviewDecision` is legitimately empty on a PR
# nobody has reviewed, which is the single most important case this file reports; with a tab
# delimiter that empty field vanished and every column after it slid left, so a PR whose checks
# were merely queued was read as CONFLICTING. Measured on the first run of this script.
while IFS=$'\x1f' read -r number title branch draft decision mergeable merge_state updated oid checks; do
	[ -n "${number:-}" ] || continue
	if [ -n "$ONLY_PR" ] && [ "$ONLY_PR" != "$number" ]; then continue; fi

	issue=$(issue_for_pr "$number")

	if [ "$draft" = "true" ]; then
		rows+="$(emit pr "$number" draft "marked draft" "$branch" "$issue")"$'\n'
		continue
	fi

	# Two facts the list payload does not carry, and both of them decide a class:
	#   /reviews              -- when the newest change request was filed
	#   /requested_reviewers  -- whether anybody was asked at all. This is read from REST rather
	#                            than from `reviewRequests`, which lists only OUTSTANDING requests
	#                            and so cannot tell "never asked" from "asked and already answered".
	reviews=$(api_get "pulls/$number/reviews" \
		'[.[]|{u:(.user.login//""),s:.state,t:.submitted_at}]' '[]')
	newest_change=$(printf '%s' "$reviews" | jq -r '[.[]|select(.s=="CHANGES_REQUESTED")]|max_by(.t)|.t // empty' 2>/dev/null || true)
	any_review=$(printf '%s' "$reviews" | jq -r 'length' 2>/dev/null || printf '0')
	requested=$(api_get "pulls/$number/requested_reviewers" '[.users[].login]|join(",")' '')
	head_date=$(api_get "commits/$oid" '.commit.committer.date' '')

	case "$mergeable" in
	CONFLICTING)
		rows+="$(emit pr "$number" conflict "$mergeable/$merge_state; checks $checks" "$branch" "$issue")"$'\n'
		continue
		;;
	esac

	if [ "$checks" = "fail" ]; then
		rows+="$(emit pr "$number" checks_failing "a check concluded failure ($merge_state)" "$branch" "$issue")"$'\n'
		continue
	fi

	case "$decision" in
	APPROVED)
		if [ "$merge_state" = "CLEAN" ] && [ "$mergeable" = "MERGEABLE" ] && [ "$checks" = "pass" ]; then
			rows+="$(emit pr "$number" merge_now "approved, clean, every check passed" "$branch" "$issue")"$'\n'
		else
			rows+="$(emit pr "$number" awaiting_merge "approved; mergeStateStatus=$merge_state, checks=$checks" "$branch" "$issue")"$'\n'
		fi
		continue
		;;
	CHANGES_REQUESTED)
		# The decision string is not evidence that feedback is outstanding: it stays
		# CHANGES_REQUESTED after a fix push until the reviewer actively re-reviews. Measured on
		# #206 -- change request filed 2026-11-01T23:02Z, head commit 2026-11-04T13:58Z. So the
		# question is which of the two is newer, and a review older than the head commit has
		# already been answered.
		if [ -n "$newest_change" ] && [ -n "$head_date" ] &&
			[ "$(date -u -d "$newest_change" +%s)" -gt "$(date -u -d "$head_date" +%s)" ]; then
			rows+="$(emit pr "$number" address_review "review asked for changes at $newest_change, after the head commit at $head_date" "$branch" "$issue")"$'\n'
		elif [ -z "$requested" ]; then
			rows+="$(emit pr "$number" needs_reviewer "answered by the head commit $head_date; nobody holds a request" "$branch" "$issue")"$'\n'
		else
			rows+="$(emit pr "$number" awaiting_review "head commit $head_date, request outstanding with $requested" "$branch" "$issue")"$'\n'
		fi
		continue
		;;
	esac

	# No decision at all. Measured 4 Nov: #212, #214 and #215 had zero rows in /reviews and zero
	# entries in /requested_reviewers three days after opening. That is not a slow reviewer, it is
	# an unaddressed PR, and the two look identical to anything that reads only reviewDecision.
	if [ "$any_review" = "0" ] && [ -z "$requested" ]; then
		rows+="$(emit pr "$number" needs_reviewer "no review and nobody asked since it opened ($updated)" "$branch" "$issue")"$'\n'
	elif [ "$checks" = "pending" ]; then
		rows+="$(emit pr "$number" checks_pending "checks running; request outstanding with ${requested:-nobody}" "$branch" "$issue")"$'\n'
	else
		rows+="$(emit pr "$number" awaiting_review "requested of ${requested:-nobody}; decision ${decision:-none}" "$branch" "$issue")"$'\n'
	fi
done <<<"$pr_rows"

# --------------------------------------------------------------------- issues

if [ "$CLOSING_OK" = "1" ]; then
	while IFS=$'\x1f' read -r number title; do
		[ -n "${number:-}" ] || continue
		if printf '%s' "$claimed_issues" | jq -e --argjson n "$number" 'index($n) != null' >/dev/null 2>&1; then
			continue
		fi
		rows+="$(emit issue "$number" unclaimed "$title" '' "$number")"$'\n'
	done < <(gh issue list --state open --limit 100 --json number,title \
		--jq '.[] | [.number, (.title | gsub("[\t\r\n]"; " "))] | join("\u001f")' 2>/dev/null) ||
		printf 'triage: gh issue list failed; the issue half is empty this pass\n' >&2
fi

# --------------------------------------------------------------------- orphaned branches
#
# A branch with commits and no open PR is invisible to any queue that reads issues and PRs, which
# is how `fix/nav-marks-the-current-page` came to hold days of unmerged work with no record of it
# anywhere. That work is already paid for, so it is triaged rather than left.

main_sha=$(git -C "$FACTORY_ROOT" rev-parse origin/main 2>/dev/null || printf '')
seen_branches=''
# The branch this harness is standing on is, by definition, work in progress with no PR yet, and
# it is the tree the supervisor itself would have to work in. Without this the harness's own branch
# is the first orphan every pass reports and the supervisor starts by being handed its own
# unfinished commit. Asked of FACTORY_DIR, not FACTORY_ROOT: the primary checkout sits on whatever
# branch a human left there, and that branch is normally an open PR anyway.
current_branch=$(git -C "$FACTORY_DIR" symbolic-ref --short -q HEAD || printf '')
if [ -n "$main_sha" ]; then
	# Tab-separated here is safe where it was not for the pull-request rows: every field is
	# non-empty (an object name, a ref name, a literal), and a git ref name cannot contain a
	# character below 0x21 at all, so no field can be blanked or shifted.
	while IFS=$'\t' read -r sha branch where; do
		[ -n "${branch:-}" ] || continue
		branch=${branch#refs/heads/}
		branch=${branch#refs/remotes/origin/}
		[ "$branch" = "main" ] && continue
		[ "$branch" = "$current_branch" ] && continue
		# One verdict per name. A branch that exists both locally and on origin is reported once,
		# from the origin copy, because that is the copy a PR would be opened from.
		case "$seen_branches" in *"|$branch|"*) continue ;; esac
		seen_branches="$seen_branches|$branch|"
		if printf '%s' "$pr_json" | jq -e --arg b "$branch" 'any(.[]; .headRefName == $b)' >/dev/null 2>&1; then
			continue
		fi
		ahead=$(git -C "$FACTORY_ROOT" rev-list --count "$main_sha..$sha" 2>/dev/null || printf '0')
		[ "$ahead" -gt 0 ] || continue
		rows+="$(emit branch "$branch" branch_without_pr "$ahead commit(s) ahead of main, no open PR ($where)" "$branch" '')"$'\n'
	done < <(
		git -C "$FACTORY_ROOT" for-each-ref --format="%(objectname)"$'\t'"%(refname)"$'\torigin' \
			refs/remotes/origin/ 2>/dev/null
		git -C "$FACTORY_ROOT" for-each-ref --format="%(objectname)"$'\t'"%(refname)"$'\tlocal only' \
			refs/heads/ 2>/dev/null
	)
fi

rows=${rows%$'\n'}

if [ "$TABLE" = "1" ]; then
	printf '%s\n' "$header" | awk -F'\t' '{printf "%-6s %-36s %-18s %s\n", $1, $2, $3, $4}'
	printf '%s\n' "$rows" | tr '\037' '\t' |
		awk -F'\t' 'NF{printf "%-6s %-36s %-18s %s\n", $1, $2, $3, $4}'
else
	printf '%s\n' "$mheader"
	[ -n "$rows" ] && printf '%s\n' "$rows"
fi
exit 0
