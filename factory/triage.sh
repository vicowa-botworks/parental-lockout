#!/usr/bin/env bash
# shellcheck shell=bash disable=SC2016
# triage.sh -- what is stopping this repository's queue, read from GitHub and git.
#
# Prints one U+001F-separated row per thing that needs something, seven columns:
#
#   kind    key       class                detail                         branch  issue  ask
#   pr      206       merge_now            approved, clean, checks pass   …       216    -
#   pr      215       conflict             CONFLICTING/DIRTY              …       213    -
#   pr      214       needs_reviewer       no review, nobody asked        …       211    -
#   pr      206       awaiting_review      answered, requested of …       …       216    -
#   pr      240       address_review       review asked for changes …     …       240    <the ask>
#   pr      240       review_unreadable    21 bytes of body, no ask       …       240    -
#   issue   213       unclaimed            <title>                        -       213    -
#   issue   199       unclaimed            [merged:PR#229 on `issue-199`  -       199    -
#   issue   232       delivered            delivered by merged PR #234 …  -       232    -
#   branch  fix/x     branch_without_pr    3 file(s) across 2 commit(s)   fix/x   -      -
#
# `class` is a verdict, not a field. The classes factory.sh calls workable are the ones worth a
# model; the rest are waits, and a wait costs no tokens. `delivered` is neither: it is a report
# about an issue the queue is about to hand out again for no reason, and it is the one class whose
# whole job is to make factory.sh start nothing (#235). `review_unreadable` is the second of those,
# and it exists because the class next to it used to dispatch a tick at a review whose body had
# never arrived (#242) -- see the block in the CHANGES_REQUESTED arm.
#
# `ask` is the seventh column and the only one a reviewer writes. It is the reviewer's own words,
# bounded, carried in the row so the tick dispatched to answer them is dispatched WITH its input
# instead of having to go and find it -- the finding of it was the step that failed silently, because
# a tick that cannot read the ask cannot distinguish "the reviewer asked for nothing" from "the
# reviewer asked for something I cannot see". It is empty for every class that is not an ask, and it
# is empty for `review_unreadable` on purpose: that is the class that says there is no ask here.
#
# It is a column and not part of `detail` for one reason: `detail` is printed by the journal on
# every pass (`NEXT <class> <key> age=… (<detail>)`) and by `--table` for a person, and a measured
# ask here runs to 1002-14863 bytes (8 Nov, 86 change requests read off this repository's own
# history). A queue log whose busiest line is 14 KB of review prose is a queue log nobody reads, and
# the journal is the one place this harness assumes anybody looks.
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

Reads open and merged pull requests, open issues and origin's branches, and prints one TSV row per
thing that needs something (or, for `delivered`, one row per thing that needs nothing):

  kind   one of pr | issue | branch
  key    the number, or the branch name
  class  merge_now | conflict | checks_failing | checks_pending | address_review |
         review_unreadable | needs_reviewer | awaiting_review | awaiting_merge | draft |
         unclaimed | delivered | branch_without_pr
  detail why that class, with the timestamps the verdict was made from. One `unclaimed` row may open
         with `$FACTORY_MERGED_MARK` -- a merged pull request named the issue and the sole-claim or
         `Part of` arm refused it, so part of this ticket may already be on the trunk. The class and
         the tick are unchanged; see the refused-claim block.
  branch the PR head / orphan branch
  issue  the issue GitHub says the PR closes, when there is one
  ask    the reviewer's own words for `address_review`, bounded at
         FACTORY_ASK_MAX_BYTES with a named cut when the body is longer; empty for every
         other class, including `review_unreadable`, whose whole point is that there is
         no ask to carry

--table  same rows, aligned for a person instead of a pipe. It stays four columns wide; a row with
         an ask prints it under itself as an indented `ask:` line.
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

# Seven fields. The seventh is the reviewer's ask -- see the header -- and it is empty for every
# class that is not one. An empty LAST field survives `IFS=$'\037' read` exactly: U+001F is not IFS
# whitespace, so nothing collapses and nothing slides.
emit() { printf '%s\037%s\037%s\037%s\037%s\037%s\037%s\n' "$1" "$2" "$3" "$4" "${5:--}" "${6:--}" "${7:-}"; }

# U+001F between fields, for the reason recorded at the read loop below: a tab is IFS whitespace,
# and `read` collapses runs of those, so an empty field silently shifts every column after it.
header=$'kind\tkey\tclass\tdetail\tbranch\tissue\task'
mheader=$'kind\037key\037class\037detail\037branch\037issue\037ask'

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
# The same call also asks for every OPEN issue's `stateReason`, and it is not a garnish for the
# `delivered` verdict below: it is the only field that says "somebody has already rejected a closure
# of this issue once". #222 is the live case. It was closed as satisfied at 2026-11-06T13:17:20Z and
# reopened at 16:27:23Z the same day, with one row of the closure table corrected and file:line for
# it; PR #227 then merged on 2026-11-08 naming it `Refs #222`, under a section headed "Why `#222` was
# reopened", and its own "What stays open" paragraph says the issue is still not finished. Nothing
# else about #222 separates it from #232, whose merged `Refs` really did deliver it: same single
# reference, same head-branch shape, same silence on the issue after the merge. The field is the
# whole difference, and `gh` does not expose it -- `gh issue list --json stateReason` and
# `gh search issues --json stateReason` both answer `Unknown JSON field: "stateReason"` and print the
# fields they do have, measured 8 Nov -- so it is asked for where it exists.
#
# When it fails, the issue half is skipped for this pass rather than guessed. Assuming an issue is
# unclaimed while a PR for it exists opens a second PR for the same work, which is worse than
# doing nothing for ten minutes.
CLOSING_OK=1
queue_json=$(gh api graphql \
	-f query='query($o:String!,$n:String!){repository(owner:$o,name:$n){pullRequests(states:OPEN,first:100){pageInfo{hasNextPage}nodes{number closingIssuesReferences(first:20){nodes{number}}}}issues(states:OPEN,first:100){pageInfo{hasNextPage}nodes{number stateReason}}}}' \
	-f o="$FACTORY_OWNER" -f n="$FACTORY_NAME" 2>/dev/null) ||
	queue_json=''

# Both halves come out of the one payload, so `delivered` cannot cost a second call. Either half
# failing means the payload is not what was asked for, and the whole issue side of the table is
# skipped for the pass rather than rebuilt from a guess.
#
# A payload that answers PART of the question is the same hazard wearing a friendlier face, and both
# connections here are capped at `first:100` -- eleven open issues today, so the cap is invisible
# until the day it is not. A truncated issue list does not merely miss a `delivered` row: the
# `stateReason` veto for the reopened issue it fell off disappears, and a claim that lost its only
# veto reads as delivered -- the one mistake this class was built to refuse. A truncated PR list
# hides an OPEN claim, which is the duplicate-tick failure H-0053 was written about, arriving because
# the queue has more pull requests than the read asks for. So each connection is asked for
# `pageInfo{hasNextPage}` and `true` on either one routes into the same refusal as a payload that
# did not answer at all. `false` on BOTH is the only answer that clears it: a missing `pageInfo` is
# not a `false`, it is a payload that is not the one that was asked for, and tie-break 3 keeps that
# reading on the refusal side. The bound itself is not a verdict either way -- `hasNextPage` says the
# read stopped, which is a different sentence from "nothing is delivered", and it is printed as its
# own line so a log can tell the two apart.
closing_json=''
reopen_json=''
queue_truncated=''
if [ -n "$queue_json" ]; then
	closing_json=$(printf '%s' "$queue_json" | jq -c \
		'[.data.repository.pullRequests.nodes[]|{n:.number,c:[.closingIssuesReferences.nodes[].number]}]' 2>/dev/null) ||
		closing_json=''
	# Each open issue's last state transition, as `{"222":"REOPENED", …}`. `NONE` means the issue
	# was never closed, which is the ordinary case and no reason to distrust a claim.
	reopen_json=$(printf '%s' "$queue_json" | jq -c \
		'[.data.repository.issues.nodes[]|{(.number|tostring):(.stateReason // "NONE")}] | add // {}' 2>/dev/null) ||
		reopen_json=''
	# Deliberately not `if … then "yes" else "no"`: `all(… == false)` makes the explicit `false` the
	# only clearing value, so null (a renamed field, a payload with the connection missing) lands on
	# the refusal side rather than the verdict side. A jq that cannot answer is `yes` as well, by the
	# `||`, because a scan nobody read is not a scan that found nothing.
	queue_truncated=$(printf '%s' "$queue_json" | jq -r \
		'[.data.repository.pullRequests.pageInfo.hasNextPage, .data.repository.issues.pageInfo.hasNextPage]
		 | if all(.[]; . == false) then "no" else "yes" end' 2>/dev/null) ||
		queue_truncated='yes'
fi
if [ -z "$closing_json" ] || [ -z "$reopen_json" ]; then
	CLOSING_OK=0
	closing_json='[]'
	reopen_json='{}'
	printf 'triage: the queue payload did not answer; issues skipped this pass\n' >&2
elif [ "$queue_truncated" = "yes" ]; then
	CLOSING_OK=0
	closing_json='[]'
	reopen_json='{}'
	printf 'triage: the queue read stopped at its pagination bound (first:100); issues skipped this pass\n' >&2
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
# So an issue counts as claimed if GitHub says a PR closes it OR any open PR body names it with a
# VERB. "Names it" alone was the first version and it over-claims: measured on 5 Nov, the then-open
# bodies mentioned #213 (PR #215: "**#213 stands and is unchanged**"), #212 (PR #214: "which #212
# adds"), #202 and #149 (PR #218: "Not here: #149's per-control commit policy") and #206/#209/#214/#215
# (PR #217's own review table), none of which is anybody claiming an issue. On the issue half those
# rows are not cosmetic -- a mentioned issue is *dropped* from `unclaimed`, so every prose cross-reference
# retires a ticket from the queue silently, and the queue shrinks fastest right where the discussion is
# densest. A claim is a verb plus a number: closes/fixes/resolves (what GitHub would register), refs
# and "part of" (what this repository writes for partial work, #1256), and nothing else.
#
# Spelled once, in one variable, because the merged bodies below are read with this same expression
# and the two channels must not be able to disagree about what a claim is. A verb list that drifts
# apart would make `delivered` fire on sentences `claimed` refuses, which is the over-claim above
# wearing the other class's clothes.
#
# Still one body read per state, not one per PR.
#
# The tail of the expression is the part that was missing from the first version, and PR #221's
# shipped body is what found it: `Refs #219, #220, #204` on one line, and the sentence eight lines
# above it, `(Part of #219 only; the umbrella is not finished — …)`. A verb plus ONE number reads that
# line as a claim on #219 and nothing else, which is both wrong at once: it hides the two issues the
# same list claims, and it lets a PR written `Closes #10, #11` count as a SOLE claim on #10 -- the
# exact bundle shape this verdict refuses, arriving through a comma. So a claim is a verb plus a
# number plus any further numbers still in the same list: separated by a comma, a slash, an `&`, a
# `|` or an `and`, and nothing else, and staying on the one line (the separator class holds no
# newline, so two sentences never merge into one list). The separator has to be one of those or
# nothing, because "Refs #10 and the framing was re-shot for #11" is two sentences about two things,
# and a rule that swallowed the second number would be the over-claim again.
CLAIM_RE='(?i)\b(?:clos(?:e|es|ed|ing)|fix(?:e[sd])?|resolv(?:e|es|ed|ing)|refs?|part of)\b[^#\n]{0,20}#[1-9][0-9]*(?:[ \t]*[,&/|][ \t]*#[1-9][0-9]*|[ \t]+and[ \t]+#[1-9][0-9]*)*'

body_json=$(gh pr list --state open --limit 100 --json number,body 2>/dev/null || printf '[]')
mentioned_issues=$(printf '%s' "$body_json" | jq -c --arg re "$CLAIM_RE" '
  [ .[] | (.body // "")
    | scan($re)
    | scan("#[1-9][0-9]*") | ltrimstr("#") | tonumber ] | unique' 2>/dev/null || printf '[]')
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

	# Three facts the list payload does not carry, and all of them decide a class:
	#   /reviews              -- when the newest change request was filed, who filed it, and WHAT
	#                            IT ASKED FOR. The third one is new, and its absence is #242: the
	#                            projection below used to be `[.[]|{u,s,t}]`, so the ask was thrown
	#                            away here and the row could only say that a verdict existed.
	#   /requested_reviewers  -- whether anybody was asked at all. This is read from REST rather
	#                            than from `reviewRequests`, which lists only OUTSTANDING requests
	#                            and so cannot tell "never asked" from "asked and already answered".
	#
	# The whole review array is kept, unprojected, for one pass into factory_newest_ask. Projecting
	# it here was the bug, so the projection is gone rather than widened: a filtered copy that names
	# four fields is one more thing that can silently stop naming the fifth.
	reviews=$(api_get "pulls/$number/reviews" '.' '[]')
	# `id<TAB>when<TAB>user<TAB>verdict<TAB>body`, or empty when there is no change request. The
	# verdict and the timestamp come from the SAME read, which is what makes the arms below
	# exhaustive: a payload that could not be read leaves `newest_change` empty as well as the
	# verdict `none`, so there is no state that has a timestamp to compare and no body to judge.
	ask_rec=$(factory_newest_ask "$reviews" 2>/dev/null) || ask_rec=''
	ask_id='' newest_change='' ask_user='' ask_verdict='none' ask_flat='' ask_bytes=''
	if [ -n "$ask_rec" ]; then
		IFS=$'\t' read -r ask_id newest_change ask_user ask_verdict ask_flat <<<"$ask_rec"
		ask_bytes=${ask_verdict##* }
		ask_verdict=${ask_verdict%% *}
	fi
	# A review ROW is not a verdict, and counting rows is the wrong question. `DISMISSED` is what every
	# approval on a pull request becomes the moment anybody pushes to it -- this repository does not carry
	# a stale approval across a new head -- and `COMMENTED` was never a verdict whatever the endpoint
	# calls it. Measured on #269 (10 Nov) across one fast-forward push: one `DISMISSED` row and an empty
	# `/requested_reviewers` printed `WAIT pr:269 awaiting_review` in the supervisor's journal at
	# 19:17:23Z, where the never-reviewed case had printed `POKED pr:269` at 17:58:17Z. The row survived
	# the push; the review did not. #272.
	#
	# A payload that cannot be read leaves all three at their refusal values, and the refusal asks: an
	# unreadable `/reviews` answers `verdicts=0` and lands in the arm below, which costs one throttled
	# `gh` call. The other direction is the silence every arm above exists to prevent.
	any_review=$(printf '%s' "$reviews" | jq -r 'length' 2>/dev/null || printf '0')
	verdicts=$(printf '%s' "$reviews" |
		jq -r '[.[] | select((.state // "") == "APPROVED" or (.state // "") == "CHANGES_REQUESTED")] | length' \
			2>/dev/null || printf '0')
	# The newest voided verdict, in the shape a person reads off a journal line: who filed it and the
	# commit it was filed against, at the seven characters every other sha in this file uses.
	# `commit_id` is REST's field for `gh pr view --json reviews`'s `commit.oid`, and it is the reviewed
	# commit, NOT the moment of the dismissal -- there is no dismissal timestamp anywhere in the payload,
	# and `submitted_at` on a voided row is still the approval's own stamp, twenty minutes before the
	# push that voided it on #269. So the sentence names the push and never a time.
	voided=$(printf '%s' "$reviews" |
		jq -r '[.[] | select((.state // "") == "DISMISSED")] | max_by(.submitted_at // "")
			| if . == null then ""
			  else "\(.user.login // "someone") at \(if .commit_id then (.commit_id[0:7]) else "an unreadable commit" end)"
			  end' 2>/dev/null || printf '')
	# The program is `$JQ_REQUESTED_LIST` (common.sh), shared with the poke's own read-back so the two
	# cannot disagree about who holds a request. It counts `teams[]` as well as `users[]`, because that
	# is where a request addressed to a team lands -- and a row that called one `needs_reviewer` would
	# hand it back to the poke every FACTORY_REVIEW_POKES for a request that is already outstanding.
	# The `team:` prefix is what makes the detail column say who was asked instead of blending a team
	# into a list of logins.
	requested=$(api_get "pulls/$number/requested_reviewers" "$JQ_REQUESTED_LIST" '')
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
			# Two things this arm used to treat as one: a change request whose body arrived, and one
			# whose body did not (#242). The first is a tick's job. The second is not a tick's job at
			# any price, because the tick's only two moves are to read a file that was never uploaded
			# or to invent the ask -- and inventing is what this repository has measured over and over
			# (#1180's invented read-backs, D-0046's five invented citations in three published
			# artifacts). A human re-posts a missing ask in one command; a tick cannot synthesise one,
			# and it will not report that it did.
			#
			# There is no third arm here for "verdict none", and that is by construction rather than
			# by luck: `ask_verdict` and `newest_change` are read out of the one record
			# `factory_newest_ask` returns, so a payload that could not be read leaves the timestamp
			# empty as well as the verdict, and the guard above has already sent that row down to the
			# wait arms. A verdict always comes with the timestamp that made it reachable.
			case "$ask_verdict" in
			readable)
				# The ask rides in the row's seventh column so the dispatch carries its own input.
				# It is quoted material, not instructions: the sentence that says so is in the tick's
				# prompt (factory.sh), because the column is reviewer-controlled text arriving inside
				# a model's instructions and it should not have to be obvious that that needs saying.
				ask_carry=$(factory_ask_bound "$ask_flat" "$number")
				rows+="$(emit pr "$number" address_review "review asked for changes at $newest_change, after the head commit at $head_date (review $ask_id by ${ask_user:-someone}, $ask_bytes bytes; the ask is in the row's ask column)" "$branch" "$issue" "$ask_carry")"$'\n'
				;;
			unreadable:*)
				# The literal body, because "21 bytes" is the symptom and `@/tmp/pr240-review.md` is
				# the diagnosis, and the second one is what tells a person which mistake was made. Cut
				# at 160 bytes: `below-floor` bodies are under 40 by definition and a `reference` is
				# one token, so this bound only ever clips a pathological body.
				ask_lit=$(printf '%s' "$ask_flat" | cut -b1-160)
				rows+="$(emit pr "$number" review_unreadable "the newest change request (review $ask_id by ${ask_user:-someone} at $newest_change, after the head commit at $head_date) carries $ask_bytes bytes of body, which is ${ask_verdict#unreadable:} -- its whole text is [$ask_lit]; nobody can answer a review that was never uploaded, so no tick was started: re-post it with gh pr review $number --request-changes --body-file <file>" "$branch" "$issue")"$'\n'
				;;
			esac
		elif [ -z "$requested" ]; then
			rows+="$(emit pr "$number" needs_reviewer "answered by the head commit $head_date; nobody holds a request" "$branch" "$issue")"$'\n'
		else
			rows+="$(emit pr "$number" awaiting_review "head commit $head_date, request outstanding with $requested" "$branch" "$issue")"$'\n'
		fi
		continue
		;;
	esac

	# No verdict that still stands. Measured 4 Nov: #212, #214 and #215 had zero rows in /reviews and zero
	# entries in /requested_reviewers three days after opening. That is not a slow reviewer, it is
	# an unaddressed PR, and the two look identical to anything that reads only reviewDecision.
	#
	# The question is now "is there a verdict", not "are there rows", and the three details are the three
	# ways to have no answer: never reviewed (the 4 Nov shape, whose sentence the legs pin unchanged),
	# reviewed without approving (`COMMENTED`, which `reviewDecision` reads as REVIEW_REQUIRED exactly as
	# it reads a dismissal), and reviewed then voided by a push -- the last one printed a wait until #272.
	if [ "$verdicts" = "0" ] && [ -z "$requested" ]; then
		if [ -n "$voided" ]; then
			rows+="$(emit pr "$number" needs_reviewer "every verdict voided by a push ($voided); nobody holds a request" "$branch" "$issue")"$'\n'
		elif [ "$any_review" != "0" ]; then
			rows+="$(emit pr "$number" needs_reviewer "no verdict, $any_review comment row(s); nobody holds a request since it opened ($updated)" "$branch" "$issue")"$'\n'
		else
			rows+="$(emit pr "$number" needs_reviewer "no review and nobody asked since it opened ($updated)" "$branch" "$issue")"$'\n'
		fi
	elif [ "$checks" = "pending" ]; then
		rows+="$(emit pr "$number" checks_pending "checks running; request outstanding with ${requested:-nobody}" "$branch" "$issue")"$'\n'
	else
		rows+="$(emit pr "$number" awaiting_review "requested of ${requested:-nobody}; decision ${decision:-none}" "$branch" "$issue")"$'\n'
	fi
done <<<"$pr_rows"

# --------------------------------------------------------------------- merged claims
#
# An issue whose fix has already merged comes back here as `unclaimed`, because both channels above
# are scoped to OPEN pull requests and a merged one claims nothing whatever its body says. The
# measured invoice: #228's fix merged at 07:08:17Z and a tick was dispatched for #228 at 09:35Z on a
# branch with nothing ahead of `origin/main`; the ceiling went to re-deriving what `main` already had,
# and #228 would have been handed out again on the next activation. Measured again eight hours later
# on the same tree: #232's fix merged at 14:26:31Z as #234 with `Refs #232` as its only reference and
# the queue still prints `issue 232 unclaimed`.
#
# The verb alone cannot decide this, and that is the finding rather than an excuse. Read the live open
# issues against the merged bodies that name them (8 Nov, the shipped regex over `gh pr list --state
# merged --json number,body`):
#
#   issue  claim                     truth                                            what separates it
#   #232   PR #234 `Refs #232`       delivered -- H-0050 is the fix             issue never closed/reopened
#   #228   PR #231 `Refs #228`       delivered -- the tick that found this closed it  issue never closed/reopened
#   #222   PR #227 `Refs #222`       NOT delivered -- closed 6 Nov 13:17Z, REOPENED   stateReason REOPENED
#                                     6 Nov 16:27Z with a corrected row, and #227's
#                                     own body says what stays open
#   #213   PR #229 `Refs #213`       NOT delivered -- the same body's "What this      four issues claimed by one
#   #204   PR #229 `Refs #204`       branch does not finish" names both               body: a bundle is a list of
#   #199   PR #229 `Refs #199`                                                       cross-references
#
# So the verdict needs more than the sentence, and everything it uses is a fact GitHub already holds
# rather than a reading of prose: the claim is trusted only when it is the ONLY issue that body
# claims (a bundle's reference list is a cross-reference -- three of #229's four are disclaimed by
# #229 itself), only when the body did not use this repository's own spelling of partial work
# (`Part of #N`, #1256), only when the merge reached `FACTORY_BASE` rather than a stacked branch
# (#216's chain), only when the issue has said nothing since the merge (which is also the human's
# override: one comment puts the ticket back in the queue), and only when no previous closure of
# that issue has already been rejected (#222). Every rule there has a live counterexample behind it
# and a leg in factory/selftest.sh.
#
# What that leaves unsaid is the honest residue: a partial delivery whose body carries none of these
# marks is still reported `delivered`. So the verdict is built only out of facts GitHub already holds
# -- sole claim, no `Part of`, landed on `FACTORY_BASE`, issue quiet since, never reopened -- and never
# out of a sentence in a body saying the work is finished. What a body says about its own unfinished
# parts is prose, and prose is a person's job: which is why this is a report rather than a close, and
# why the row is printed instead of dropped. Dropping it is what the `unclaimed` channel does with an
# over-claim, and that is the lesson the open-body scan above carries in its own comment -- a dropped
# issue is work nobody will look for again.
# `headRefName` is in this field list for the refused-claim note below, and it is NOT an input to
# either verdict: `delivered` still reads four facts and no branch name. It is carried because it is
# the one fact that tells a reader whether a merged body was written FOR this ticket (`issue-235`) or
# merely mentions it while delivering another (`issue-253` naming #251, measured 10 Nov) -- and as a
# verdict a branch name is measurably false. Three merged pull requests sit on the branch `issue-199`
# (#229, #240, #243), #231 and #236 both sit on `issue-228`, and #254 was merged on `issue-246` under
# a `Part of #246` that #246 is still open to finish. So it is evidence handed to the reader and never
# the thing the scan decides with (H-0108).
merged_json=$(gh pr list --state merged --limit "$FACTORY_DELIVERED_WINDOW" \
	--json number,body,mergedAt,baseRefName,headRefName 2>/dev/null || printf '')
if [ -z "$merged_json" ]; then
	printf 'triage: merged pull requests unreadable; nothing is reported delivered this pass\n' >&2
	merged_json='[]'
fi

# One row per claimed issue: the newest merged claim wins, because the newest claim is the one that
# would have finished it. A body that claims several issues contributes nothing at all -- and "several"
# is counted over the NUMBERS a claim carries, not over the sentences, because of PR #221's shipped
# `Refs #219, #220, #204`: one sentence, three claims. `last` was the first version's choice, and it is
# the one that cannot see that list.
#
# The payload goes to jq on STANDARD INPUT, and that is a correctness decision rather than a style
# one. `--argjson m "$merged_json"` was the first version and it is dead on arrival against the real
# repository: 100 merged bodies are 632,601 bytes measured 8 Nov, and Linux refuses a single argument
# longer than 128 KiB (MAX_ARG_STRLEN, 32 pages) whatever `getconf ARG_MAX` says -- 2 MiB on this box,
# which is no help at all to one argument. The kernel returned E2BIG, jq printed `Argument list too
# long`, and the fallback answered `{}`, so the channel that exists to stop re-work reported nothing
# and the live queue kept printing `issue 232 unclaimed` exactly as before the fix. Measured, not
# inferred: `/usr/bin/jq: Argument list too long` against that payload, and `bash -x` showing
# `delivered_map=` followed by `delivered_map='{}'`. The self-test did not catch it because its fixture
# payload is 2 KiB -- which is the lesson about a fixture that abridges what it imitates, and
# factory/selftest.sh now carries a leg with a body past the limit for precisely that reason.
delivered_map=$(printf '%s' "$merged_json" | jq -c --arg base "$FACTORY_BASE" --arg re "$CLAIM_RE" '
  def claims: (.body // "") as $b
    | [ $b | scan($re) | . as $p
        | [ $p | scan("#[1-9][0-9]*") | {phrase: $p, n: (ltrimstr("#") | tonumber)} ] ]
    | flatten;
  [ .[]
    | select(.baseRefName == $base)
    | select((.mergedAt // "") != "")
    | . as $p
    | claims as $all
    | ($all | unique_by(.n)) as $one
    | select(($one | length) == 1)
    | select(($all | map(.phrase | ascii_downcase | startswith("part of")) | any) | not)
    | {issue: $one[0].n, pr: $p.number, at: .mergedAt, phrase: $one[0].phrase}
  ]
  | group_by(.issue) | map(max_by(.at))
  | map({(.issue | tostring): .}) | add // {}' 2>&1) || :
# `|| :` is load-bearing and a leg pins it: this file runs under `set -e`, and a command substitution
# that fails inside an assignment ENDS the pass there, which turns "the merged read was garbage" into
# a triage that prints no rows at all -- pull requests included. The refusal below is the point, not an
# exit. Measured: without the `:`, a non-JSON merged payload took the whole table with it and the only
# thing left on stderr was jq's own complaint.
#
# The result is checked by SHAPE, not by an exit status. `jq` printing a message and answering `null`
# is a success status, and `{}` is a legitimate answer here, so the only test that distinguishes "no
# merged claim qualifies" from "the scan did not run" is whether a map came back. The reason travels
# to stderr with the verdict; `${delivered_map:0:200}` because a jq diagnostic can carry a whole body.
if ! printf '%s' "$delivered_map" | jq -e 'type == "object"' >/dev/null 2>&1; then
	printf 'triage: the merged-claim scan refused, so nothing is reported delivered this pass: %s\n' \
		"${delivered_map:0:200}" >&2
	delivered_map='{}'
fi

# ------------------------------------------------------- the claims the arms refused (#277)
#
# The arms above refuse a claim for two reasons, and both of them are honest: a body naming several
# issues does not say which ones it finished, and `Part of #N` is this repository's own spelling of
# partial work. Refusing is right. What was wrong is that the refusal was then thrown away, so the
# next tick started from nothing and re-derived the answer by hand. Measured in the supervisor's own
# journal: `TICK-START unclaimed:235 2026-11-10T22:57:23Z`, `TICK-DONE 23:14:23Z` -- seventeen minutes
# of ceiling to discover that PR #238, merged 8 Nov on branch `issue-235` under a body naming eight
# issues, had already delivered #235's three requirements. The classifier had read that body, had
# refused it, and had said nothing about the refusal to anybody.
#
# So the refusal is kept and carried: this map holds, per issue, the newest MERGED claim that named it
# and was refused, with the reason the arms gave. It changes no verdict -- the class stays `unclaimed`,
# the tick still runs, nothing is closed, and `delivered` still needs all five facts it always needed.
# That is deliberate, and it is what separates this from the two shapes #277 proposed:
#
#   * A branch name is not a verdict. #254 merged on `issue-246` under `Part of #246` and #246 is
#     still open with a requirement nobody has built; #255 merged on `issue-253` naming #251 beside
#     #253, and `factory/selftest.sh` carries the record that #253's third instruction was reported
#     delivered by #255 and was not on `main`. Both would have been claimed by a branch-shaped signal.
#   * A class that starts no tick retires real work. #253 was refused by the count arm at #255 and
#     NEEDED the tick that row dispatched: #257 delivered it five hours later. #246 is in the queue as
#     `unclaimed` right now, on the strength of a refusal that was correct.
#
# The note is therefore evidence for the reader the arms decline to be, and the veto pair below is the
# same pair the verdict uses: an issue that spoke after the merge has already been re-queued by a
# person, and an issue whose closure was once rejected has already proved it needs one. Neither is
# given a note, because in both cases somebody has already read exactly this.
refused_map=$(printf '%s' "$merged_json" | jq -c --arg base "$FACTORY_BASE" --arg re "$CLAIM_RE" '
  def claims: (.body // "") as $b
    | [ $b | scan($re) | . as $p
        | [ $p | scan("#[1-9][0-9]*") | {phrase: $p, n: (ltrimstr("#") | tonumber)} ] ]
    | flatten;
  [ .[]
    | select(.baseRefName == $base)
    | select((.mergedAt // "") != "")
    | . as $p
    | claims as $all
    | ($all | unique_by(.n) | length) as $k
    | ($all | map(.phrase | ascii_downcase | startswith("part of")) | any) as $partial
    | $all[]
    | ( (if $k == 1 then [] else ["names it in a \($k)-issue body"] end)
      + (if $partial then ["a `Part of` phrase in the same body"] else [] end) ) as $why
    | select(($why | length) > 0)
    | {n, pr: $p.number, at: $p.mergedAt, branch: ($p.headRefName // ""), why: $why}
  ]
  | group_by(.n)
  | map(max_by(.at) + {bodies: (map(.pr) | unique | length)})
  | map({(.n | tostring): .}) | add // {}' 2>&1) || :
# Same `|| :` and the same reason: under `set -e` a failed substitution in an assignment ends the
# pass, and the second scan must not be able to take the table -- or the first scan's verdicts -- down
# with it. Checked by shape for the same reason as the map above: `{}` is a legitimate answer, so only
# the shape says whether the scan ran. What is lost when it does not is the note and nothing else, and
# that is said out loud rather than left as a silently shorter row.
if ! printf '%s' "$refused_map" | jq -e 'type == "object"' >/dev/null 2>&1; then
	printf 'triage: the refused-claim scan refused, so an `unclaimed` row says nothing about merged bodies: %s\n' \
		"${refused_map:0:200}" >&2
	refused_map='{}'
fi

# The verdict for one open issue, or nothing when nothing may be claimed about it. `updated` is the
# issue's own last activity and the comparison is on the first nineteen characters of GitHub's
# second-precision UTC stamps, so a payload that grew milliseconds cannot invert it. An unreadable
# `updatedAt` refuses rather than proceeding: without it there is no way to tell whether the issue
# spoke after the claim, and a `delivered` that silences a live ticket is the harm this whole block
# is about.
delivered_claim() { # delivered_claim <issue> <issue updatedAt>
	printf '%s' "$delivered_map" | jq -r --argjson n "$1" --arg updated "${2:-}" '
		.[$n | tostring] as $c
		| if $c == null then ""
		  elif $updated == "" then ""
		  elif ($updated[0:19]) > ($c.at[0:19]) then ""
		  else "merged PR #\($c.pr) at \($c.at) says `\($c.phrase)`"
		  end' || printf ''
		# No `2>/dev/null` on that jq, and not for tidiness. The empty answer is the conservative one
		# -- the issue stays `unclaimed` and gets its tick -- so a refusal here is invisible in the
		# table, which is the shape #235 was filed for. What a payload this repository has not
		# produced yet (a `mergedAt` that is not a string, say) therefore still says one line about
		# itself instead of quietly reproducing the bug.
}

# The refused-claim note for one open issue, or nothing. Three answers are nothing, and each of them
# is a different reason: no merged body named it at all (a stranger, not a refused claim -- the
# #149 case, where #218's "Not here: #149's per-control commit policy" carries no verb and so is not a
# claim for either scan), the issue spoke after the merge (a person has re-queued it deliberately), or
# a closure of it was already rejected (a person has already read exactly this). The stamp comparison
# is the same nineteen characters `delivered_claim` uses, so the two scans cannot disagree about
# whether an issue has been quiet.
refused_note() { # refused_note <issue> <issue updatedAt>
	printf '%s' "$refused_map" | jq -r \
		--argjson n "$1" --arg updated "${2:-}" --arg mark "$FACTORY_MERGED_MARK" '
		.[$n | tostring] as $c
		| if $c == null then ""
		  elif $updated == "" then ""
		  elif ($updated[0:19]) > ($c.at[0:19]) then ""
		  else
		    $mark + ($c.pr | tostring)
		    + (if $c.branch == "" then "" else " on `\($c.branch)`" end)
		    + " at \($c.at) -- \($c.why | join("; "))"
		    + (if $c.bodies > 1 then " (\($c.bodies) merged bodies name it)" else "" end)
		    + "] "
		  end' || printf ''
	# No `2>/dev/null`, for the reason `delivered_claim` carries: an empty answer here is invisible in
	# the table, so a refusal has to be able to speak. Without the redirect a jq complaint on stderr
	# names the payload that broke, and the row that quietly lost its note does not.
}

# Has a closure of this issue already been rejected? `REOPENED` is GitHub's own record of it, and one
# rejection is enough to disqualify the mechanical verdict for good: an issue that has already had a
# fix declared satisfied and withdrawn has, by the record, proved it needs a reader.
was_reopened() { # was_reopened <issue>
	printf '%s' "$reopen_json" | jq -e --argjson n "$1" '.[($n|tostring)] == "REOPENED"' >/dev/null 2>&1
}

# --------------------------------------------------------------------- issues

if [ "$CLOSING_OK" = "1" ]; then
	# The row list has a bound of its own, and it fails differently from the one above: a GraphQL
	# connection that stops mid-page loses a veto and can hand out a false `delivered`, while 101
	# open issues simply means the 101st gets no row at all -- not `unclaimed`, not `delivered`,
	# nothing. That is silent today, and silence about a bound is how a queue comes to be read as a
	# census. The counter costs nothing and the sentence names the number, so an operator can see
	# the table has stopped short instead of concluding the repository has no more open work.
	issue_limit=100
	issue_rows=0
	while IFS=$'\x1f' read -r number title updated; do
		[ -n "${number:-}" ] || continue
		issue_rows=$((issue_rows + 1))
		# GitHub says an open PR closes it, or an open PR body claims it with a verb.
		if printf '%s' "$claimed_issues" | jq -e --argjson n "$number" 'index($n) != null' >/dev/null 2>&1; then
			continue
		fi
		# Reaching this line is itself a fact: no OPEN pull request claims the issue, so the open
		# channel -- which outranks a merged claim, #216's stacked chain being that case every day
		# until its bottom PR merges -- had nothing to say.
		claim=$(delivered_claim "$number" "$updated")
		# A rejected closure once is enough: the record already says a fix was declared satisfied
		# here and withdrawn, which is proof the mechanical verdict needs a reader.
		if [ -n "$claim" ] && was_reopened "$number"; then
			claim=''
		fi
		if [ -n "$claim" ]; then
			# The row is the deliverable, so it carries what a person needs to check it without
			# re-running anything: the PR, the moment it merged, its words, and the window it was
			# found inside. Nothing caps the detail -- `emit` prints its fields with `printf '%s'`,
			# and the sentence shipped here claimed an 80-column cap that does not exist in this file
			# or in `emit` (measured 8 Nov at this commit: `grep -n 'cut -c' factory/triage.sh` finds
			# no such thing, and `emit` is seven `%s` and a newline). The length rule for a detail is
			# the journal's, not a cap's: `NEXT <class> <key> age=… (<detail>)` is printed every pass
			# and is the line a person actually reads, so this one is a number and an instant rather
			# than a sentence about them, and the prose that cannot fit in a journal line is the ask
			# column's job (H-0055).
			rows+="$(emit issue "$number" delivered "$claim (window $FACTORY_DELIVERED_WINDOW)" '' "$number")"$'\n'
		else
			# The refusal travels with the row instead of being dropped (#277). A tick that arrives at
			# this issue knowing `merged PR #229 named it on `issue-199` and the sole-claim rule
			# refused` starts from a diff to read; the same tick without the note starts from zero and
			# spends the ceiling re-deriving what the scan already knew, which is the seventeen minutes
			# measured above. It is put at the START of the detail because `factory.sh` matches the
			# marker anchored there, and what follows it is a title somebody typed.
			#
			# The class does not move. This row is still `unclaimed`, it still gets its tick, and the
			# only thing that changes is what the tick is told on the way in -- because the refusal
			# being right is exactly why the issue must keep being worked: #246 is here now, refused by
			# a `Part of` that told the truth about unfinished work.
			note=''
			if ! was_reopened "$number"; then
				note=$(refused_note "$number" "$updated")
			fi
			rows+="$(emit issue "$number" unclaimed "${note}${title}" '' "$number")"$'\n'
		fi
	done < <(gh issue list --state open --limit "$issue_limit" --json number,title,updatedAt \
		--jq '.[] | [.number, (.title | gsub("[\t\r\n]"; " ")), (.updatedAt // "")] | join("\u001f")' 2>/dev/null) ||
		printf 'triage: gh issue list failed; the issue half is empty this pass\n' >&2
	# `>=`, so exactly 100 warns too and the sentence is worded for that: it says the read is AT its
	# limit, not that work is waiting past it, because the read cannot tell those two apart either.
	if [ "$issue_rows" -ge "$issue_limit" ]; then
		printf 'triage: the open-issue read is at its own limit (%s rows); an issue past it gets no row of any class this pass\n' \
			"$issue_limit" >&2
	fi
fi

# --------------------------------------------------------------------- orphaned branches
#
# A branch with commits and no open PR is invisible to any queue that reads issues and PRs, which
# is how `fix/nav-marks-the-current-page` came to hold days of unmerged work with no record of it
# anywhere. That work is already paid for, so it is triaged rather than left.

main_sha=$(git -C "$FACTORY_ROOT" rev-parse origin/main 2>/dev/null || printf '')
seen_branches=''
# The two tallies the content verdict can end up with, kept as space-separated name lists because a
# git ref name cannot contain whitespace at all (`git check-ref-format` refuses it), so the list is
# also the sentence. Neither one is a row: both are said on stderr, the way every other degraded read
# in this file is said, because a queue row is an offer and neither of these has anything to offer.
empty_branches=''
empty_count=0
unjudged_branches=''
unjudged_count=0
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
		# Commits are not content (#291). `rev-list --count` cannot tell a branch that changes 400
		# lines from a branch whose only commit is the `git commit --allow-empty` rollback target that
		# Step 3 of the factory skill makes every tick take *before* a judgement call -- and a tick
		# killed by the wall-clock ceiling between that checkpoint and its effect commit leaves
		# exactly that: a ref ahead of `main` holding zero bytes. Measured 11 Nov on `issue-249`, one
		# commit ahead: `git show --numstat` printed no file list and `git diff --stat c98f75f f64ddcd`
		# printed nothing, and `factory.sh:675` turned the count into "The work is already written" and
		# dispatched a model at it. So the row is bought with a content question -- does this branch
		# carry a single byte the trunk lacks -- asked from the MERGE BASE and not from `main`'s tip,
		# so a branch that merged the trunk in is not charged for the trunk's own files.
		merge_base=$(git -C "$FACTORY_ROOT" merge-base "$main_sha" "$sha" 2>/dev/null || printf '')
		delta=''
		comparable=1
		if [ -z "$merge_base" ]; then
			comparable=0
		elif ! delta=$(git -C "$FACTORY_ROOT" diff --name-only "$merge_base" "$sha" 2>/dev/null); then
			comparable=0
			delta=''
		fi
		if [ "$comparable" = 1 ] && [ -z "$delta" ]; then
			# Nothing to deliver. Counted and said out loud instead of skipped in silence: a class
			# that quietly stops printing rows is the failure this harness refuses everywhere else
			# (`factory.sh:1143`: "A queue that reports a thing once and then goes quiet is a queue
			# nobody can audit"), and the pass cannot know from here whether the branch is a dead
			# tick's checkpoint or a person's abandoned probe. What it CAN say is that the difference
			# is empty -- which is a different sentence from "there is no branch", and gets said every
			# pass that still sees the ref rather than once.
			empty_branches="$empty_branches $branch"
			empty_count=$((empty_count + 1))
			continue
		fi
		# Unjudgeable keeps the row. Not being able to compare is not evidence there is nothing
		# there: two histories with no merge base in common, or a git that refused the diff, are both
		# reads that failed, and the tie-break for a missing input is to report rather than to drop
		# (#873's direction of travel -- ambiguity goes toward the safe side, and here the safe side
		# is that a person's work gets looked at). The detail says it is unverified rather than
		# asserting a byte count this pass could not measure.
		if [ "$comparable" != 1 ]; then
			unjudged_branches="$unjudged_branches $branch"
			unjudged_count=$((unjudged_count + 1))
		fi
		if [ "$comparable" = 1 ]; then
			nfiles=$(printf '%s\n' "$delta" | wc -l | tr -d '[:space:]')
			content="$nfiles file(s) across $ahead commit(s)"
		else
			content="$ahead commit(s), content not comparable to main"
		fi
		rows+="$(emit branch "$branch" branch_without_pr "$content ahead of main, no open PR ($where)" "$branch" '')"$'\n'
	done < <(
		git -C "$FACTORY_ROOT" for-each-ref --format="%(objectname)"$'\t'"%(refname)"$'\torigin' \
			refs/remotes/origin/ 2>/dev/null
		git -C "$FACTORY_ROOT" for-each-ref --format="%(objectname)"$'\t'"%(refname)"$'\tlocal only' \
			refs/heads/ 2>/dev/null
	)
	# Both tallies speak on every pass that sees such a branch, and neither one is a row. Naming the
	# branches is the point: a count alone says the scan is odd, a list of names says where to look.
	if [ "$empty_count" -gt 0 ]; then
		printf 'triage: %s branch(es) ahead of main hold no content the trunk lacks, so none is '\
			'offered: %s\n' "$empty_count" "${empty_branches# }" >&2
	fi
	if [ "$unjudged_count" -gt 0 ]; then
		printf 'triage: %s branch(es) ahead of main have no merge base with main, so their content '\
			'could not be compared; they are still offered, unverified: %s\n' \
			"$unjudged_count" "${unjudged_branches# }" >&2
	fi
fi

rows=${rows%$'\n'}

if [ "$TABLE" = "1" ]; then
	printf '%s\n' "$header" | awk -F'\t' '{printf "%-6s %-36s %-18s %s\n", $1, $2, $3, $4}'
	# The ask gets its own line under its row rather than a fifth column: it is measured in
	# kilobytes on this repository's own reviews, and a column is only as wide as the widest thing
	# a terminal has to scroll for. It is a continuation line, so `--table` stays four columns wide
	# for everything that parses it, and the row above still says how many bytes are below it.
	printf '%s\n' "$rows" | tr '\037' '\t' |
		awk -F'\t' 'NF {
			printf "%-6s %-36s %-18s %s\n", $1, $2, $3, $4
			if (NF >= 7 && $7 != "") printf "        ask: %s\n", $7
		}'
else
	printf '%s\n' "$mheader"
	[ -n "$rows" ] && printf '%s\n' "$rows"
fi
exit 0
