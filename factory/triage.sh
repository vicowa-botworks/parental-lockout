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
#   issue   232       delivered            delivered by merged PR #234 …  -       232
#   branch  fix/x     branch_without_pr    2 ahead of main, no open PR    fix/x   -
#
# `class` is a verdict, not a field. The classes factory.sh calls workable are the ones worth a
# model; the rest are waits, and a wait costs no tokens. `delivered` is neither: it is a report
# about an issue the queue is about to hand out again for no reason, and it is the one class whose
# whole job is to make factory.sh start nothing (#235).
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
         needs_reviewer | awaiting_review | awaiting_merge | draft | unclaimed |
         delivered | branch_without_pr
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
	-f query='query($o:String!,$n:String!){repository(owner:$o,name:$n){pullRequests(states:OPEN,first:100){nodes{number closingIssuesReferences(first:20){nodes{number}}}}issues(states:OPEN,first:100){nodes{number stateReason}}}}' \
	-f o="$FACTORY_OWNER" -f n="$FACTORY_NAME" 2>/dev/null) ||
	queue_json=''

# Both halves come out of the one payload, so `delivered` cannot cost a second call. Either half
# failing means the payload is not what was asked for, and the whole issue side of the table is
# skipped for the pass rather than rebuilt from a guess.
closing_json=''
reopen_json=''
if [ -n "$queue_json" ]; then
	closing_json=$(printf '%s' "$queue_json" | jq -c \
		'[.data.repository.pullRequests.nodes[]|{n:.number,c:[.closingIssuesReferences.nodes[].number]}]' 2>/dev/null) ||
		closing_json=''
	# Each open issue's last state transition, as `{"222":"REOPENED", …}`. `NONE` means the issue
	# was never closed, which is the ordinary case and no reason to distrust a claim.
	reopen_json=$(printf '%s' "$queue_json" | jq -c \
		'[.data.repository.issues.nodes[]|{(.number|tostring):(.stateReason // "NONE")}] | add // {}' 2>/dev/null) ||
		reopen_json=''
fi
if [ -z "$closing_json" ] || [ -z "$reopen_json" ]; then
	CLOSING_OK=0
	closing_json='[]'
	reopen_json='{}'
	printf 'triage: the queue payload did not answer; issues skipped this pass\n' >&2
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
merged_json=$(gh pr list --state merged --limit "$FACTORY_DELIVERED_WINDOW" \
	--json number,body,mergedAt,baseRefName 2>/dev/null || printf '')
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

# Has a closure of this issue already been rejected? `REOPENED` is GitHub's own record of it, and one
# rejection is enough to disqualify the mechanical verdict for good: an issue that has already had a
# fix declared satisfied and withdrawn has, by the record, proved it needs a reader.
was_reopened() { # was_reopened <issue>
	printf '%s' "$reopen_json" | jq -e --argjson n "$1" '.[($n|tostring)] == "REOPENED"' >/dev/null 2>&1
}

# --------------------------------------------------------------------- issues

if [ "$CLOSING_OK" = "1" ]; then
	while IFS=$'\x1f' read -r number title updated; do
		[ -n "${number:-}" ] || continue
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
			# found inside. It is written to be checkable at a glance, which is not the same claim
			# as fitting a cap -- nothing truncates a field here. `emit` is the six-field printf at
			# the top of this file and the only length the phrase inherits is the one the claim
			# itself has: one distinct number, at most 20 characters of prose ahead of it, and a
			# list tail that can repeat that SAME number as often as a body likes, because the
			# sole-claim rule counts distinct issues, not characters. Measured with a planted body
			# repeating `#232` forty times: a 302-column detail, printed whole, `(window 100)` and
			# all, and the row still `delivered`. factory/selftest.sh 12d keeps that honest with a
			# leg rather than a sentence, since a comment that invents a bound is how the next
			# author sizes text against a limit that does not exist.
			rows+="$(emit issue "$number" delivered "$claim (window $FACTORY_DELIVERED_WINDOW)" '' "$number")"$'\n'
		else
			rows+="$(emit issue "$number" unclaimed "$title" '' "$number")"$'\n'
		fi
	done < <(gh issue list --state open --limit 100 --json number,title,updatedAt \
		--jq '.[] | [.number, (.title | gsub("[\t\r\n]"; " ")), (.updatedAt // "")] | join("\u001f")' 2>/dev/null) ||
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
