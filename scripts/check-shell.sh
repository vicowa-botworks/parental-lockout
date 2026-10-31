#!/usr/bin/env bash
# check-shell.sh -- shellcheck over every shell script this repository ships.
#
# Why this is in `make verify` (issue #189): CI's shellcheck job takes 6-7 seconds, and `verify`
# had no shellcheck step at all, so a script-only change passed the pre-push gate with a lint
# error in it and found out nine minutes later on a runner. Same gap as the script-encoding
# guard, same cost, same fix.
#
# The list is derived from git rather than spelled out. The CI job used to carry a list of names,
# and a list of names is a list that goes stale: the next script added by a PR is not linted by
# anything until someone remembers to add it, and nothing fails when they don't. Two pathspecs,
# because one is not enough. `*.sh` is the set of scripts the repository ships by extension;
# `deploy/packaging/deb/*` is the set it ships by convention -- dpkg maintainer scripts are
# `postinst`, `prerm` and `postrm`, with no extension to match, and they are exactly the files the
# hand-maintained list had picked up. Dropping them would have been silent in the worst way: the
# step keeps printing a green tick and a smaller number. The invariant that makes the glob safe is
# that everything under that directory IS a maintainer script, so a non-script appearing there is
# itself the thing to catch.
#
# Measured on main the derived list was twelve files; on this branch it is twenty-one (five
# packaging drivers, three maintainer scripts, and the deriving script itself).
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
# CDPATH makes `cd` consult the environment before the current directory, and the anchor below is
# a relative `cd` whenever the script was invoked as `bash scripts/check-shell.sh` (dirname of that
# $0 is `scripts`, which neither starts with `/` nor `./`). Measured with a decoy directory holding
# its own scripts/ and its own git tree: the step printed `over 1 scripts` and exited 0 -- a pass
# that linted one file from another repository and none of ours. Neither guard downstream
# can see it: one asks whether the binary exists, the other whether the list is empty, and a
# foreign list is neither. Of the three shapes this file exists to refuse -- linted nothing, linted
# a subset, linted the wrong thing -- this was the one that could still exit 0.
CDPATH=
unset CDPATH

SHELLCHECK=${SHELLCHECK:-shellcheck}
# `-`, not `:-`: an empty value is an answer ("exclude nothing"), not a request for the default.
EXCLUDE_PREFIX=${SHELLCHECK_EXCLUDE_PREFIX-.opencode/}
# CI sets this. Without it, a wrong path in the job's `env:` would print the skip note below and
# exit 0 -- a shellcheck job that green-ticks having linted nothing is worse than no shellcheck
# job, because it is the same colour as the real thing.
SHELLCHECK_REQUIRED=${SHELLCHECK_REQUIRED:-}

# Resolved before the anchor, not after it. `command -v` looks a relative path up in the current
# directory, so doing it after the `cd` above resolves `SHELLCHECK=./local-sc` against the
# repository root and prints "not installed" for a binary sitting in the directory the caller was
# standing in -- a green run that linted nothing, which is the exact shape this script exists to
# refuse. Absolute paths and bare names on PATH are unaffected; only a caller-supplied relative
# path could tell the difference, and it is the caller who would be surprised.
case "$SHELLCHECK" in
  # Absolute paths are already resolved -- and matching them here would be a real breakage, since
  # CI passes one: `/tmp/...` matches `*/*` and would be rewritten onto the repository root.
  /*) ;;
  */*) [ -f "$SHELLCHECK" ] && SHELLCHECK="$PWD/${SHELLCHECK#./}" ;;
esac

# bash 4.4 at minimum, and said out loud. `mapfile -d` is what reads the NUL-delimited list below
# and arrived in 4.4 (`mapfile` itself in 4.0), while `#!/usr/bin/env bash` takes whatever is first
# on PATH -- macOS still ships 3.2, RHEL 7 ships 4.2. Without this the script dies at the mapfile
# with `mapfile: command not found` and rc 127, which does refuse rather than pass, but says
# nothing about a step whose entire claim is "N scripts linted".
if [ "${BASH_VERSINFO[0]}" -lt 4 ] ||
   { [ "${BASH_VERSINFO[0]}" -eq 4 ] && [ "${BASH_VERSINFO[1]}" -lt 4 ]; }; then
  printf 'check-shell: bash 4.4 or newer is required (the file list is read NUL-delimited by\n' >&2
  printf 'mapfile -d); this is %s. Refusing rather than linting an unknown subset.\n' "$BASH_VERSION" >&2
  exit 1
fi

# Anchored to the repository, not to the caller, and after the resolution above so that a
# caller-relative binary path still means what the caller meant. `git ls-files` limits its pathspec
# to the current directory and prints paths relative to it, so an unanchored run from testing/
# lints a subset -- 8 of 13 when measured on main -- and calls that a pass. It is the same shape
# the empty-list guard below refuses
# at zero. It gets worse than incomplete: shellcheck follows `source` only across the batch it is
# handed, so the subset also produced 11 SC2034/SC1091 findings for files that are clean when the
# whole set is passed. Both directions wrong, and only for whoever runs this directly, which is
# the reader of this script.
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."

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
#
# Through a temp file rather than a process substitution: `set -o pipefail` does not cover
# `< <(git …)`, so a git failure would leave an empty array and the only thing noticing would be
# the guard below -- which would then blame the pathspec when git is what failed. Here git's own
# complaint is followed by the reason that actually applies.
list=$(mktemp) || exit 1
trap 'rm -f "$list"' EXIT
if ! git ls-files -z '*.sh' 'deploy/packaging/deb/*' > "$list"; then
  printf 'check-shell: git ls-files failed, so there is no list of scripts to lint.\n' >&2
  printf 'That is usually a checkout outside a git tree, not a pathspec problem.\n' >&2
  exit 1
fi
all=()
mapfile -d '' -t all < "$list"

files=()
for f in "${all[@]}"; do
  # Prefix test by stripping, not with `case`: an empty pattern in a case arm matches every
  # path, so `SHELLCHECK_EXCLUDE_PREFIX=` -- "exclude nothing" -- would exclude the whole set and
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
  # that lints twenty-one, and the way that happens is the interesting one: a pathspec change, a
  # checkout outside a git tree, or an exclusion that grew. `verify`'s empty-selection step
  # refuses for exactly this reason.
  printf 'check-shell: no shell scripts found (excluded prefix %s), so this checked nothing.\n' \
    "$EXCLUDE_PREFIX" >&2
  printf 'That is not a pass. Check the pathspec and the exclusion rather than the exit status.\n' >&2
  exit 1
fi

printf 'check-shell: %s over %s scripts\n' "$("$SHELLCHECK" --version | sed -n 's/^version: //p')" "${#files[@]}"
"$SHELLCHECK" "${files[@]}"
