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
  tr -s '[:space:]' '\n' <"$1" | grep -E '^[0-9]+$' || true
}

# check_baseline <file> -- 0 usable, 1 unusable (reason on stderr).
#
# Three ways a baseline is unusable, and all three fail the same way: by making the
# watcher report reviews that are not new.
#
#  1. Missing. A typo'd path, or /tmp being cleaned on reboot, is a capture that never
#     happened. It must NOT read as "no known reviews" -- that is the poll-1 false alarm
#     this whole script exists to prevent. (An EMPTY file legitimately means "no reviews
#     yet", which is a brand-new PR, so emptiness is fine and absence is not.)
#  2. Non-empty but no ids in it -- the space-joined-with-trailing-space shape, or junk.
#  3. Contaminated with prose. `--capture 26 > BASE` with a `2>&1` in the history, or a
#     gh auth failure, writes `gh: Not Found (HTTP 404)`. Digit-run extraction pulls the
#     id 404 out of that, the guard sees "one id, fine", and every real review is new --
#     because the file holds no real ids at all. A baseline is machine-written by
#     --capture, so any non-numeric field means the capture did not produce ids.
check_baseline() {
  if [ ! -f "$1" ]; then
    echo "FATAL: baseline $1 does not exist; refusing to poll." >&2
    echo "       Capture it BEFORE polling: watch-pr.sh --capture <pr> > $1" >&2
    echo "       A missing baseline reads as 'no known reviews', which makes every" >&2
    echo "       existing review look new and fires the watcher on poll 1." >&2
    return 1
  fi
  if [ -s "$1" ] && [ -n "$(tr -s '[:space:]' '\n' <"$1" | grep -vE '^$|^[0-9]+$')" ]; then
    echo "FATAL: baseline $1 contains non-numeric fields; refusing to poll." >&2
    echo "       A baseline is written by --capture and holds only review ids. Prose in" >&2
    echo "       it means the capture failed (an error page, or stderr redirected in)," >&2
    echo "       and its digits would be mistaken for ids while the real ids are absent." >&2
    return 1
  fi
  if [ -s "$1" ] && [ -z "$(known_ids "$1")" ]; then
    echo "FATAL: baseline $1 is non-empty but contains no review ids; refusing to poll." >&2
    echo "       An unusable baseline reads as 'no known reviews', which makes every" >&2
    echo "       existing review look new and fires the watcher on poll 1." >&2
    return 1
  fi
  return 0
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
  fail=0

  full=$(mktemp); printf '5510608872\n5510609000\n' >"$full"
  # A: negative control. Nothing new must be reported.
  neg=$(diff_ids "$cur" "$full")
  if [ -z "$neg" ]; then
    echo "control A (full baseline -> nothing new): PASS"
  else
    echo "control A: FAIL, reported [$neg]"
    fail=1
  fi

  # B: positive control. Drop one id FROM THE BASELINE -- that is what "a new review
  # arrived" looks like. Dropping it from the current side instead checks nothing: that
  # id is then never examined, so the control reports nothing and can never fail.
  short=$(mktemp); grep -vx '5510608872' "$full" >"$short"
  pos=$(diff_ids "$cur" "$short")
  if [ "$pos" = "5510608872 hermesvicowa CHANGES_REQUESTED" ]; then
    echo "control B (baseline missing an id -> exactly that one): PASS"
  else
    echo "control B: FAIL, reported [$pos]"
    fail=1
  fi

  # C: the malformed shape that caused the false alarm -- one line, space-joined, with a
  # trailing space. It must still parse, not read as an empty baseline.
  joined=$(mktemp); printf '5510608872 5510609000 ' >"$joined"
  j=$(diff_ids "$cur" "$joined")
  if [ -z "$j" ]; then
    echo "control C (space-joined baseline still parses): PASS"
  else
    echo "control C: FAIL, reported [$j]"
    fail=1
  fi

  # D: the guard itself, invoked as the main path invokes it. An earlier version of this
  # control re-implemented the guard's condition inline, so editing the real guard could
  # break it while control D kept printing PASS -- the same "a control that tests a copy
  # of the thing" defect this script fixes elsewhere. It now calls check_baseline.
  junk=$(mktemp); printf 'pending\n' >"$junk"
  if check_baseline "$junk" 2>/dev/null; then
    echo "control D: FAIL, the junk baseline was accepted as usable"
    fail=1
  else
    echo "control D (unparsable non-empty baseline is fatal): PASS"
  fi

  # E: a baseline file that does not exist. This is the defect the cold review reproduced:
  # `[ -s ]` is false for a missing file, so the guard was skipped and the watcher fired on
  # poll 1 reporting every stale review as new -- the exact failure the header calls the
  # worst this tool can have.
  if check_baseline "$(mktemp -u)" 2>/dev/null; then
    echo "control E: FAIL, a missing baseline was accepted as usable"
    fail=1
  else
    echo "control E (missing baseline file is fatal): PASS"
  fi

  # F: a baseline contaminated by a failed capture. `gh: Not Found (HTTP 404)` yields the
  # digit run 404, which the old extractor accepted as an id while the real ids were absent.
  polluted=$(mktemp); printf 'gh: Not Found (HTTP 404)\n' >"$polluted"
  if check_baseline "$polluted" 2>/dev/null; then
    echo "control F: FAIL, a baseline of error text was accepted as usable"
    fail=1
  else
    echo "control F (prose in the baseline is fatal, not an id): PASS"
  fi

  # G: an EMPTY baseline is legitimate -- a brand-new PR has no reviews -- and must stay
  # usable, or the fix for E would make the common case unwatchable.
  empty=$(mktemp); : >"$empty"
  if check_baseline "$empty" 2>/dev/null; then
    echo "control G (empty baseline is legitimate, not fatal): PASS"
  else
    echo "control G: FAIL, a new PR's empty baseline was refused"
    fail=1
  fi

  rm -f "$full" "$short" "$joined" "$junk" "$polluted" "$empty"
  # The exit status is the interface a machine consumes. Printing FAIL and exiting 0
  # green-lights a broken watcher to exactly the harness this skill tells an agent to
  # build, which is why the controls exist at all.
  if [ "$fail" -ne 0 ]; then
    echo "SELFTEST FAILED"
    exit 1
  fi
  echo "SELFTEST OK"
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

# A non-numeric POLLS makes `seq` fail, the loop body never runs, and the script prints
# NO_CHANGE and exits 3 -- indistinguishable from "watched the whole budget, nothing
# happened". A non-numeric POLL_SECONDS makes `sleep` fail every iteration, turning a
# polite poll into a tight loop against the API. Both are one-line guards.
for n in "polls=$POLLS" "poll_seconds=$POLL_SECONDS"; do
  name=${n%%=*}
  val=${n#*=}
  case "$val" in
    '' | *[!0-9]*)
      echo "FATAL: $name must be a positive integer, got '$val'" >&2
      exit 2
      ;;
  esac
  if [ "$val" -lt 1 ]; then
    echo "FATAL: $name must be at least 1, got '$val'" >&2
    exit 2
  fi
done

check_baseline "$BASE" || exit 2

# Consecutive failures. `|| continue` alone makes a dead watcher -- expired auth, wrong
# repo, deleted PR -- report exactly what a healthy quiet watcher reports, which is the
# silent-deafness failure mode this script is otherwise careful about. After this many
# polls in a row that cannot be answered, stop and say so rather than exit 3.
deaf_after=5
deaf=0

for i in $(seq 1 "$POLLS"); do
  sleep "$POLL_SECONDS"
  if ! now=$(reviews_of "$PR" 2>/dev/null); then
    deaf=$((deaf + 1))
    if [ "$deaf" -ge "$deaf_after" ]; then
      echo "FATAL: $deaf consecutive failures reading PR #$PR reviews; stopping." >&2
      echo "       Run: gh api repos/$REPO/pulls/$PR/reviews --jq '.[].id'" >&2
      echo "       A watcher that cannot read the API must not exit 3: that is the" >&2
      echo "       exit code for 'watched the whole budget, nothing new'." >&2
      exit 2
    fi
    continue
  fi
  deaf=0
  new=$(diff_ids "$now" "$BASE")
  if [ -n "$new" ]; then
    echo "NEW_REVIEW for PR #$PR after $i polls:"
    echo "$new"
    exit 0
  fi
done

echo "NO_CHANGE for PR #$PR after $POLLS polls"
exit 3
