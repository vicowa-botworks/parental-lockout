#!/usr/bin/env bash
# check-shell.sh -- shellcheck over every shell script this repository ships.
#
# Why this is in `make verify` (issue #189): CI's shellcheck job takes 6-7 seconds, and `verify`
# had no shellcheck step at all, so a script-only change passed the pre-push gate with a lint
# error in it and found out nine minutes later on a runner. Same gap as the script-encoding
# guard, same cost, same fix.
#
# The list is derived from git rather than spelled out. The CI job used a list of twelve names,
# and a list of names is a list that goes stale: a thirteenth script added by the next PR is not
# linted by anything until someone remembers to add it, and nothing fails when they don't.
# `git ls-files '*.sh'` is the set of scripts the repository actually ships, so a new one is
# linted by the PR that adds it. Measured on main: the derived list is the same twelve files.
#
# `.opencode/` is excluded, for the reason the CI job gave when it was hand-maintained: that tree
# holds agent tooling that is installed nowhere and is written in bash with POSIX-undefined
# builtins. It is also untracked, so `git ls-files` already leaves it out -- the exclusion is
# here so that the day someone tracks one of those files, the answer is a deliberate edit to this
# line rather than a red tick that reads as noise.
#
# Skipping is loud, and skipping is not a failure. shellcheck is a Haskell binary this repository
# does not vendor, and CI fetches it at a pinned digest verified against its published hash --
# an unpinned third-party action running on every PR to a repo whose subject is a root daemon is
# its own problem. Requiring it locally would mean `make verify` fails on a fresh clone, which
# teaches people to run `go test ./...` instead of the gate. So the absence is printed as what it
# is -- one step that did not run -- and the version CI uses is named so the two can be compared.
#
# bash, not sh, for one reason: the file list is read NUL-delimited (see the note by the loop).
set -euo pipefail

# Anchored to the repository, not to the caller. `git ls-files` limits its pathspec to the current
# directory and prints paths relative to it, so an unanchored run from testing/ lints 8 of 13
# scripts and calls that a pass -- the same shape the empty-list guard below refuses at zero. It
# gets worse than incomplete: shellcheck follows `source` only across the batch it is handed, so
# the subset also produced 11 SC2034/SC1091 findings for files that are clean when the whole set
# is passed. Both directions wrong, and only for whoever runs this directly, which is the reader
# of this script.
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."

SHELLCHECK=${SHELLCHECK:-shellcheck}
# `-`, not `:-`: an empty value is an answer ("exclude nothing"), not a request for the default.
EXCLUDE_PREFIX=${SHELLCHECK_EXCLUDE_PREFIX-.opencode/}
# CI sets this. Without it, a wrong path in the job's `env:` would print the skip note below and
# exit 0 -- a shellcheck job that green-ticks having linted nothing is worse than no shellcheck
# job, because it is the same colour as the real thing.
SHELLCHECK_REQUIRED=${SHELLCHECK_REQUIRED:-}

if ! command -v "$SHELLCHECK" >/dev/null 2>&1; then
  if [ -n "$SHELLCHECK_REQUIRED" ]; then
    printf 'check-shell: %s is not installed and SHELLCHECK_REQUIRED is set, so this is a\n' \
      "$SHELLCHECK" >&2
    printf 'failure rather than a skip: the caller asked for the lint and did not get one.\n' >&2
    exit 1
  fi
  printf 'check-shell: SKIPPED -- %s is not installed, so no shell script was linted here.\n' \
    "$SHELLCHECK"
  printf 'CI runs shellcheck %s fetched at a digest (.github/workflows/ci.yml); install it, or\n' \
    "${SHELLCHECK_VERSION:-v0.10.0}"
  printf 'point SHELLCHECK at a copy, to get this step locally.\n'
  exit 0
fi

# NUL-delimited, because a path may contain a space and word splitting would hand shellcheck two
# of them. `git ls-files -z` prints one NUL-terminated path per file; mapfile -d '' reads that
# exactly. A newline-delimited read would be the same bug wearing a loop.
all=()
mapfile -d '' -t all < <(git ls-files -z '*.sh')

files=()
for f in "${all[@]}"; do
  # Prefix test by stripping, not with `case`: an empty pattern in a case arm matches every
  # path, so `SHELLCHECK_EXCLUDE_PREFIX=` -- "exclude nothing" -- would exclude all thirteen and
  # the run would refuse for having found nothing. Measured, and it is the wrong failure to make
  # of a flag whose whole job is to be turned off. Stripping also stops a prefix containing `*`
  # from being read as a glob.
  if [ -n "$EXCLUDE_PREFIX" ] && [ "${f#"$EXCLUDE_PREFIX"}" != "$f" ]; then
    continue
  fi
  files+=("$f")
done

if [ "${#files[@]}" -eq 0 ]; then
  # Not "nothing to do, success". A gate that lints zero files prints the same green as a gate
  # that lints twelve, and the way that happens is the interesting one: a pathspec change, a
  # checkout outside a git tree, or an exclusion that grew. `verify`'s empty-selection step
  # refuses for exactly this reason.
  printf 'check-shell: no shell scripts found (excluded prefix %s), so this checked nothing.\n' \
    "$EXCLUDE_PREFIX" >&2
  printf 'That is not a pass. Check the pathspec and the exclusion rather than the exit status.\n' >&2
  exit 1
fi

printf 'check-shell: %s over %s scripts\n' "$("$SHELLCHECK" --version | sed -n 's/^version: //p')" "${#files[@]}"
"$SHELLCHECK" "${files[@]}"
