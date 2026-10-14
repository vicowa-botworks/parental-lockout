#!/usr/bin/env bash
# Watch a PR for NEW review activity.
#
# Why this exists instead of polling `reviewDecision`: on this repo the decision string
# stays CHANGES_REQUESTED after a fix push until the reviewer actively re-reviews, so a
# wait that breaks on the decision ends immediately on the STALE change request and reads
# as a fresh rejection. The only reliable signal is a review id that was not there when
# the wait started.
#
# usage:
#   watch-pr.sh <pr> <baseline-file> [polls]     wait for new review activity
#   watch-pr.sh --selftest                        check the comparison, both directions
#   watch-pr.sh --capture <pr> > BASELINE         write the baseline this script needs
#
# exit: 0 a new review appeared · 3 poll budget exhausted · 2 unusable baseline
#
# The baseline MUST be captured before polling starts, and it must be captured by
# --capture rather than by hand: the shape of that file is the single most common way
# this script has been broken in practice (see known_ids).
set -u

REPO=${PLOCK_REPO:-$(gh repo view --json nameWithOwner --jq .nameWithOwner 2>/dev/null)}
REPO=${REPO:-vicowa-botworks/parental-lockout}
POLL_SECONDS=${PLOCK_POLL_SECONDS:-30}

# one id per line, always.
#
# A space-joined baseline makes `grep -xF` see a single pattern, so every existing review
# looks new and the wait ends on poll 1 -- this exact mistake cost a full review cycle
# twice. It happens because the obvious capture (`--jq '.[].id' | tr '\n' ' '`) produces
# one line with a trailing space, and a strict `^[0-9]+$` filter then silently DROPS it:
# the baseline looks empty, which is worse than a crash, because "no known reviews" makes
# every existing review new. A silent degrade into a false alarm is the worst failure
# this tool can have, so whitespace is normalised here and a non-empty baseline that
# still yields no ids is fatal.
known_ids() {
  [ -f "$1" ] || return 0
  tr -c '0-9' '\n' <"$1" | grep -E '^[0-9]+$' || true
}

# diff_ids <current-list> <baseline-file>
#
# Prints the "<id> <user> <state>" lines whose id is not in the baseline.
#
# Deliberately NOT `grep -vxF - <(echo "$known")`: that form reads its patterns from
# stdin rather than from the process substitution, so an empty pattern set makes every
# stale review look new and the watcher fires instantly. It looked correct and passed
# review; the loop below is the version that has actually been exercised.
diff_ids() {
  local known
  known=$(known_ids "$2")
  printf '%s\n' "$1" | while read -r id rest; do
    [ -z "$id" ] && continue
    printf '%s\n' "$known" | grep -qxF "$id" || echo "$id $rest"
  done
}

reviews_of() {
  gh api "repos/$REPO/pulls/$1/reviews" \
    --jq '.[] | "\(.id) \(.user.login) \(.state)"'
}

# ---------------------------------------------------------------- selftest
#
# All three controls are SYNTHETIC, on both sides. An earlier version fetched the real
# review list and asserted it would look "not new" against a made-up baseline of
# 111/222/333 -- which can never pass, because the real id genuinely is absent from that
# baseline. A control that cannot pass is as useless as one that cannot fail, and this
# script has shipped a false alarm from exactly that class of mistake twice.
if [ "${1:-}" = "--selftest" ]; then
  cur="5510608872 hermesvicowa CHANGES_REQUESTED
5510609000 opencodevicowa APPROVED"

  full=$(mktemp); printf '5510608872\n5510609000\n' >"$full"
  # A: negative control. Nothing new must be reported.
  neg=$(diff_ids "$cur" "$full")
  [ -z "$neg" ] && echo "control A (full baseline -> nothing new): PASS" \
                || echo "control A: FAIL, reported [$neg]"

  # B: positive control. Drop one id FROM THE BASELINE -- that is what "a new review
  # arrived" looks like. Dropping it from the current side instead checks nothing: that
  # id is then never examined, so the control reports nothing and can never fail.
  short=$(mktemp); grep -vx '5510608872' "$full" >"$short"
  pos=$(diff_ids "$cur" "$short")
  [ "$pos" = "5510608872 hermesvicowa CHANGES_REQUESTED" ] \
    && echo "control B (baseline missing an id -> exactly that one): PASS" \
    || echo "control B: FAIL, reported [$pos]"

  # C: the malformed shape that caused the false alarm -- one line, space-joined, with a
  # trailing space. It must still parse, not read as an empty baseline.
  joined=$(mktemp); printf '5510608872 5510609000 ' >"$joined"
  j=$(diff_ids "$cur" "$joined")
  [ -z "$j" ] && echo "control C (space-joined baseline still parses): PASS" \
              || echo "control C: FAIL, reported [$j]"

  # D: the guard itself. A baseline with no digits in it must be refused, not treated as
  # "no known reviews".
  junk=$(mktemp); printf 'pending\n' >"$junk"
  if [ -s "$junk" ] && [ -z "$(known_ids "$junk")" ]; then
    echo "control D (unparsable non-empty baseline is fatal): PASS"
  else
    echo "control D: FAIL, the junk baseline was accepted as usable"
  fi

  rm -f "$full" "$short" "$joined" "$junk"
  exit 0
fi

if [ "${1:-}" = "--capture" ]; then
  [ -n "${2:-}" ] || { echo "usage: watch-pr.sh --capture <pr> > BASELINE" >&2; exit 2; }
  gh api "repos/$REPO/pulls/$2/reviews" --jq '.[].id'
  exit $?
fi

PR=${1:?usage: watch-pr.sh <pr> <baseline-file> [polls]  |  --selftest  |  --capture <pr>}
BASE=${2:?usage: watch-pr.sh <pr> <baseline-file> [polls]  (capture it with --capture BEFORE polling)}
POLLS=${3:-80}

if [ -s "$BASE" ] && [ -z "$(known_ids "$BASE")" ]; then
  echo "FATAL: baseline $BASE is non-empty but contains no review ids; refusing to poll." >&2
  echo "       An unusable baseline reads as 'no known reviews', which makes every" >&2
  echo "       existing review look new and fires the watcher on poll 1." >&2
  exit 2
fi

for i in $(seq 1 "$POLLS"); do
  sleep "$POLL_SECONDS"
  now=$(reviews_of "$PR" 2>/dev/null) || continue
  new=$(diff_ids "$now" "$BASE")
  if [ -n "$new" ]; then
    echo "NEW_REVIEW for PR #$PR after $i polls:"
    echo "$new"
    exit 0
  fi
done

echo "NO_CHANGE for PR #$PR after $POLLS polls"
exit 3
