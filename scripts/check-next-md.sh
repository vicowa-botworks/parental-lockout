#!/usr/bin/env bash
# check-next-md.sh -- the gate for NEXT.md, the file one tick hands to the next (#337).
#
# Why this file exists. The ticket quotes a dispatch that told a tick, verbatim, that
# "`scripts/check-next-md.sh` is the gate the loop tells you about, and it runs only when NEXT.md is in
# the diff -- quote its verdict in the pull-request body". Measured on `main` at `80dd8bf` on 15 Nov,
# inside the fenced tree:
#
#   $ bash scripts/check-next-md.sh                 -> No such file or directory, rc=127
#   $ grep -rn 'check-next-md' --exclude-dir=.git . -> 0 hits repo-wide
#   $ git log -S 'check-next-md' --all --oneline    -> 0 hits on every ref this clone has
#
# so a body that "quoted its verdict" was quoting a file that has never existed on any ref. And the
# file that sentence gates is real and is written by ticks -- NEXT.md, tracked since `120ccd1`, five
# commits from three units -- while `grep -n 'NEXT' Makefile` answers nothing: nothing read it and
# nothing checked it, which is how one merge resolution silently dropped another unit's in-flight
# warning (#337's own measurement: `grep -n '#296' NEXT.md` empty on main, last touch `ccf852c`).
#
# The root of it is an obligation with no artifact. The skill this loop dispatches with opens, at
# `.opencode/skills/factory/SKILL.md:10`, with "and nothing about the next one will be. Everything you
# must hand over lives in a file." -- and never names that file. Every unit therefore improvises one,
# and one unit improvised a gate for it too. So this ships BOTH halves: the file has a shape checked
# here, the skill carries the contract as text, and leg 3 refuses the moment either half goes missing.
#
# What each leg refuses, and why that is the leg worth having:
#
#   1. NEXT.md's shape. One level-1 heading that names the unit that wrote it (a hand-off that does not
#      say who is speaking cannot be recognised as stale), at least two sections, and a section that
#      names what is still owed -- the only sentence the next tick actually needs.
#   2. A hand-off or contract sentence naming a repository path that is not there. That is the #337
#      defect itself, kept as a permanent leg: a sentence telling the next tick to run, or to quote the
#      verdict of, a tool that does not exist.
#   3. The contract losing its source: the skill must still carry the hand-off section, and `make
#      verify` must still run this step. The pin asks for a recipe command that runs this file WITHOUT
#      `--self-test`, because `check-decisions.py`'s own pin is satisfied by its `--self-test` line
#      (#391: delete the gate step, `make verify` keeps going with no decision check at all). A gate
#      whose invocation can be deleted silently is a suggestion, and a gate invoked only as its own
#      self-test is a suggestion that prints.
#
# What this deliberately does NOT check, measured rather than assumed:
#
#   * Paths named in code, and in docs/factory.md. Four exist today and every one is a true sentence:
#     `factory/next-decision-id.sh`, `factory/selftest.sh` and docs/factory.md all name
#     `factory/DECISION.md` -- the singular path the allocator is measured refusing on -- and
#     `testing/capture/cdp.py` names `testing/capture/runs/<fixture>/capture.json`. A leg that refuses a
#     quoted measurement is a leg that gets widened away inside a week (H-0040 is that lesson, learned on
#     this repository's other gate), so leg 2 reads only the three files whose whole purpose is
#     instructions to whoever reads them: NEXT.md, the skill, and AGENTS.md.
#   * Whether what NEXT.md says is still TRUE. Whether a claimed-open issue is still open is a live-API
#     question, and `make verify` runs on a fresh clone with no token and in CI with neither; a gate that
#     needs the network is a gate people route around (`check-denies.py` is the precedent for skipping
#     loudly). A stale-but-well-formed hand-off therefore still costs a merge to notice, and the
#     contract says so out loud rather than pretending this leg covers it.
#
# Usage: check-next-md.sh [--root DIR] [--self-test] [--help]. Reads only; writes nothing.
# File-level, and it has to sit above the first command: a directive below `set -e` applies to the next
# declaration only, and ten findings stayed live until the comment moved. Backticks are this repository's
# markdown idiom and they are the point of the strings below -- the messages quote a command the way the
# skill quotes it, the fixtures write a NEXT.md that reads like a real one, and the two awk programs are
# awk source where `$0` is a field and not a shell expansion. Nothing here wants to expand.
# shellcheck disable=SC2016
set -euo pipefail
# CDPATH would let a decoy directory answer the relative `cd` that resolves the default root, and the
# whole point of the anchor is that the verdict is about THIS tree -- scripts/check-shell.sh's header
# carries the measurement (a foreign tree produced a green run that linted none of ours).
CDPATH=
unset CDPATH

# Absolute before anything else: `--self-test` re-invokes this file as a child, and a relative $0 stops
# resolving the moment the caller is standing anywhere else.
SELF=${BASH_SOURCE[0]}
SELF=$(cd -- "$(dirname -- "$SELF")" && pwd)/$(basename -- "$SELF")
ROOT=$(cd -- "$(dirname -- "$SELF")/.." && pwd)
SELFTEST=0

USAGE='usage: check-next-md.sh [--root DIR] [--self-test] [--help]

Checks the tick hand-off file (NEXT.md) and the contract that tells a tick to write it:

  * NEXT.md has exactly one level-1 heading naming the unit that wrote it, at least two sections, and
    a section naming what is still owed
  * NEXT.md, .opencode/skills/factory/SKILL.md and AGENTS.md name no repository path that is missing
  * the skill still carries the hand-off section, and `make verify` still runs this step -- not only
    this step'"'"'s self-test -- and this file'

PROBLEMS=
COUNT=0

add_problem() { # <one sentence>
	PROBLEMS="${PROBLEMS}${1}
"
	COUNT=$((COUNT + 1))
}

# --------------------------------------------------------------------------- leg 1: the shape
#
# The extractor for leg 2 is a variable rather than a here-doc so `--self-test` can hand it to a child
# run of this same file against a fixture tree.
PATHS_AWK='
{
  n = split($0, words, /[ \t`;()]+/)
  for (i = 1; i <= n; i++) {
    p = words[i]
    sub(/[].,;:!?).]+$/, "", p)
    sub(/:[0-9]+(-[0-9]+)?$/, "", p)
    if (p !~ /^(scripts|factory|testing|deploy|docs|\.opencode)\//) continue
    if (p !~ /\.(sh|py|go|md|json|jsonc|tmpl|yaml|yml|txt|service|spec)$/) continue
    if (p ~ /[*?<>\\|$]/) continue
    print p
  }
}
'

leg_shape() { # <root>
	local root=$1 file n_h1 n_h2 h1 owed
	file=$root/NEXT.md
	if [ ! -e "$file" ]; then
		add_problem "no NEXT.md at $file: the hand-off the skill promises (\"Everything you must hand over lives in a file\") has no file. Write it, or amend the contract in the skill in the same commit"
		return 0
	fi
	if [ ! -s "$file" ]; then
		add_problem "$file is empty: a hand-off that says nothing is the silent tick this loop exists to end"
		return 0
	fi
	n_h1=$(grep -c '^# ' "$file" || true)
	if [ "$n_h1" -ne 1 ]; then
		add_problem "$file carries $n_h1 level-1 heading(s), not exactly one: one hand-off per branch, or the reader cannot tell which unit is speaking"
		return 0
	fi
	h1=$(grep -m1 '^# ' "$file" || true)
	if ! printf '%s\n' "$h1" | grep -qE '(issue-[A-Za-z0-9._-]+|#[0-9]+)'; then
		add_problem "the heading \`$h1\` names no unit (want \`issue-<n>\` or \`#<n>\`): a hand-off that does not say who wrote it is indistinguishable from a stale one, which is how a dropped line became invisible"
	fi
	n_h2=$(grep -c '^## ' "$file" || true)
	if [ "$n_h2" -lt 2 ]; then
		add_problem "$file has $n_h2 \`## \` section(s); the contract wants at least two -- what this unit did, and what the next one owes"
	fi
	if ! owed=$(grep -im1 '^## .*owed\|^## .*outstanding\|^## .*unmeasured\|^## .*still open' "$file" || true); then
		owed=
	fi
	if [ -z "$owed" ]; then
		add_problem "$file has no heading naming what is still owed (one of owed, outstanding, unmeasured, still open): that heading is the only sentence the next tick reads, and its absence is what makes a unit re-derive work main already has"
		return 0
	fi
	# The ticket's third shape: no orphaned bullet. Two ways a hand-off orphans one, and both are the
	# class this unit is about -- a merge keeps a heading and drops what was under it, or a unit appends a
	# note at the foot of the file under no heading at all. So: a list item above the first `## ` belongs
	# to no section, and a list item with nothing after the marker is a bullet that says nothing. Both are
	# shape, so both are answerable from the file alone; what a line MEANS -- whether the pull request it
	# names has since merged -- is owed item 1, because that answer lives behind an API this gate may not
	# call (see the entry's Why).
	local orphan
	orphan=$(awk '
		/^## / { section = 1; next }
		{
			line = $0
			if (line ~ /^[ \t]*[-*][ \t]/ || line ~ /^[ \t]*[0-9]+[.)][ \t]/) {
				sub(/^[ \t]*([-*][ \t]+|[0-9]+[.)][ \t]+)/, "", line)
				gsub(/[ \t]/, "", line)
				if (!section) printf "above the first section on line %d: %s\n", NR, $0
				else if (line == "") printf "empty after its marker on line %d: %s\n", NR, $0
			}
		}
	' "$file")
	if [ -n "$orphan" ]; then
		while IFS= read -r o; do
			[ -n "$o" ] || continue
			add_problem "$file has an orphaned bullet -- $o: a queue line outside a section is the one a merge drops without noticing, and an empty marker is a promise with nothing in it. Put it under a heading, or delete it"
		done <<EOF
$orphan
EOF
		return 0
	fi
	printf '  ok    %-32s %s line(s), 1 heading naming the unit, %s section(s), owed section: %s\n' \
		"NEXT.md holds the shape" "$(wc -l <"$file" | tr -d '[:space:]')" "$n_h2" "$owed"
	return 0
}

# ---------------------------------------------------------------------------- leg 2: phantom paths
leg_paths() { # <root>
	local root=$1 rel f p missing='' scanned=0
	for rel in NEXT.md .opencode/skills/factory/SKILL.md AGENTS.md; do
		f=$root/$rel
		[ -f "$f" ] || continue
		scanned=$((scanned + 1))
		# `|| true` is load-bearing: this runs under `set -euo pipefail`, and a file that names no path
		# is an answer ("nothing"), not an error.
		missing="${missing}$(awk "$PATHS_AWK" "$f" | sort -u | while IFS= read -r p; do
			[ -e "$root/$p" ] || printf '%s %s\n' "$rel" "$p"
		done || true)"
	done
	if [ -n "$missing" ]; then
		while IFS=' ' read -r rel p; do
			[ -n "$p" ] || continue
			add_problem "$rel names \`$p\`, which is not in the tree: a sentence telling the next tick to run, or to quote the verdict of, a tool that does not exist is the #337 defect -- it cannot be run, so the verdict quoted for it was invented. Name a path that exists, or describe the tool in words"
		done <<EOF
$missing
EOF
		return 0
	fi
	printf '  ok    %-32s %s file(s) read, every repository path they name is there\n' \
		"no phantom tool path" "$scanned"
	return 0
}

# -------------------------------------------------------------------------- leg 3: the contract
#
# The Makefile pin reads the `verify:` recipe as command CHUNKS (the text between `;`), the way
# scripts/check-decisions.py's does, because a filename substring is satisfied by a comment, by a
# commented-out step, and by an echo that names the script while running something else. It then insists
# on one chunk that runs this file with no `--self-test` in it -- the half that pin is missing (#391) --
# and one that runs the self-test. A commented line is dropped before the split: make never hands it to
# a shell, so nothing after its `;` runs either.
MAKEFILE_AWK='
/^verify:/ { inr = 1; next }
inr && /^[^ \t]/ { inr = 0 }
!inr { next }
{
  line = $0
  sub(/^[ \t]+/, "", line)
  if (substr(line, 1, 1) == "#") next
  sub(/\\$/, "")
  m = split($0, chunks, /;/)
  for (i = 1; i <= m; i++) {
    c = chunks[i]
    sub(/^[ \t]+/, "", c)
    sub(/[ \t]+$/, "", c)
    if (substr(c, 1, 1) == "#") continue
    if (c ~ /^bash[ \t]+scripts\/check-next-md\.sh([ \t]|$)/) {
      if (c ~ /--self-test/) self += 1
      else plain += 1
    }
  }
}
END { printf "%d %d\n", plain + 0, self + 0 }
'

leg_contract() { # <root>
	local root=$1 skill makefile counts plain selft
	skill=$root/.opencode/skills/factory/SKILL.md
	makefile=$root/Makefile
	if [ ! -f "$skill" ]; then
		add_problem "no \`$skill\`: the tick-facing contract has no source in the tree, which is what let a dispatch invent a gate for NEXT.md"
	elif ! grep -q 'NEXT\.md' "$skill"; then
		add_problem "\`$skill\` never mentions NEXT.md: the skill promises a hand-off file and names no artifact, so the next unit improvises one -- and improvised this gate's name for it"
	elif ! grep -q 'scripts/check-next-md\.sh' "$skill"; then
		add_problem "\`$skill\` mentions NEXT.md but not \`scripts/check-next-md.sh\`: the contract and the gate part company, and a tick follows the sentence the gate cannot see"
	fi
	if [ ! -f "$makefile" ]; then
		add_problem "no \`$makefile\` above $root, so nothing can be claimed about the gate"
		return 0
	fi
	counts=$(awk "$MAKEFILE_AWK" "$makefile")
	plain=${counts%% *}
	selft=${counts##* }
	if [ "$plain" -eq 0 ]; then
		add_problem "nothing under \`verify:\` runs \`bash scripts/check-next-md.sh\` (a --self-test invocation does not count -- that exact hole is #391: the step deleted, the self-test left behind, the gate running nothing): add the step back"
		return 0
	fi
	if [ "$selft" -eq 0 ]; then
		add_problem "nothing under \`verify:\` runs \`bash scripts/check-next-md.sh --self-test\`, so the fixtures below run only when someone remembers they exist"
		return 0
	fi
	printf '  ok    %-32s %s line(s) in the skill, %s step(s) and %s self-test step(s) in `make verify`\n' \
		"the contract has a source" "$(grep -c 'NEXT\.md' "$skill" || true)" "$plain" "$selft"
	return 0
}

run_legs() { # <root>
	leg_shape "$1"
	leg_paths "$1"
	leg_contract "$1"
}

# ------------------------------------------------------------------------------- the self-test
#
# Fixtures are built under $TMPDIR and the gate is invoked on them as a CHILD with --root, so every
# refusal below is the shipped code refusing, not a copy of its logic. Nothing here reads or writes the
# gated tree (#1807), and the box goes on exit rather than waiting for a sweep.
SELFTEST_BOX=
ST_TOTAL=0
ST_BAD=0
selftest_cleanup() {
	[ -n "$SELFTEST_BOX" ] && rm -rf -- "$SELFTEST_BOX"
	return 0
}

write_good_tree() { # <dir>
	local box=$1
	mkdir -p "$box/.opencode/skills/factory" "$box/scripts"
	printf '# NEXT — issue-7 unit\n\nState at push: branch `issue-7`.\n\n## What this unit shipped\n\n- one gate.\n\n## What is still owed (all listed, none silent)\n\n1. nothing further.\n\n## Hard rules for whoever picks this up\n\n- run `bash scripts/check-next-md.sh`.\n' >"$box/NEXT.md"
	printf 'The contract is in scripts/check-next-md.sh and the skill.\n' >"$box/AGENTS.md"
	printf '## Step 6 — hand off\n\nWrite NEXT.md and run `bash scripts/check-next-md.sh`.\n' \
		>"$box/.opencode/skills/factory/SKILL.md"
	printf '#!/usr/bin/env bash\n' >"$box/scripts/check-next-md.sh"
	{
		printf 'verify:\n'
		printf '\t@set -e; \\\n'
		printf '\techo "==> hand-off contract"; bash scripts/check-next-md.sh --self-test; \\\n'
		printf '\techo "==> hand-off file"; bash scripts/check-next-md.sh; \\\n'
		printf '\techo "==> shellcheck"; $(MAKE) --no-print-directory check-shell; \\\n'
		printf '\techo "verify: OK"\n\n'
		printf 'clean:\n\trm -rf bin\n'
	} >"$box/Makefile"
}

st_refuse() { # <label> <needle> -- the shipped gate must refuse the box, for the named reason
	local label=$1 needle=$2 out rc=0
	out=$(bash "$SELF" --root "$SELFTEST_BOX" 2>&1) || rc=$?
	if [ "$rc" -eq 0 ]; then
		printf '  FAIL  %-46s PASSED a tree it must refuse (%s)\n' "$label" "$needle"
		ST_BAD=$((ST_BAD + 1))
	elif ! printf '%s\n' "$out" | grep -qF -- "$needle"; then
		printf '  FAIL  %-46s refused, but not with the named sentence (%s)\n' "$label" "$needle"
		printf '%s\n' "$out" | sed 's/^/            /'
		ST_BAD=$((ST_BAD + 1))
	else
		printf '  ok    %-46s refused for the reason named\n' "$label"
	fi
	ST_TOTAL=$((ST_TOTAL + 1))
}

st_accept() { # <label> -- the null variant: a tree that satisfies the contract must still pass
	local label=$1 out rc=0
	out=$(bash "$SELF" --root "$SELFTEST_BOX" 2>&1) || rc=$?
	if [ "$rc" -ne 0 ]; then
		printf '  FAIL  %-46s REFUSED a tree that satisfies the contract\n' "$label"
		printf '%s\n' "$out" | sed 's/^/            /'
		ST_BAD=$((ST_BAD + 1))
	else
		printf '  ok    %-46s accepted\n' "$label"
	fi
	ST_TOTAL=$((ST_TOTAL + 1))
}

self_test() {
	SELFTEST_BOX=$(mktemp -d "${TMPDIR:-/tmp}/check-next-md-selftest-XXXXXX")
	trap selftest_cleanup EXIT INT TERM

	write_good_tree "$SELFTEST_BOX"
	st_accept "a tree that satisfies the contract"

	write_good_tree "$SELFTEST_BOX"
	rm -f "$SELFTEST_BOX/NEXT.md"
	st_refuse "NEXT.md deleted" "no NEXT.md at"

	write_good_tree "$SELFTEST_BOX"
	printf '' >"$SELFTEST_BOX/NEXT.md"
	st_refuse "NEXT.md emptied" "is empty"

	write_good_tree "$SELFTEST_BOX"
	printf '\n# A second unit\n\n## What is still owed\n\n- x\n' >>"$SELFTEST_BOX/NEXT.md"
	st_refuse "two level-1 headings" "not exactly one"

	write_good_tree "$SELFTEST_BOX"
	printf '# NEXT — some unit\n\n## What this unit shipped\n\n- x\n\n## What is still owed\n\n- y\n' \
		>"$SELFTEST_BOX/NEXT.md"
	st_refuse "the heading names no unit" "names no unit"

	write_good_tree "$SELFTEST_BOX"
	printf '# NEXT — issue-7 unit\n\n## What this unit shipped\n\n- x\n\n## Notes\n\n- y\n' \
		>"$SELFTEST_BOX/NEXT.md"
	st_refuse "nothing is named as owed" "still owed"

	write_good_tree "$SELFTEST_BOX"
	printf '# NEXT — issue-7 unit\n\nAll good.\n' >"$SELFTEST_BOX/NEXT.md"
	st_refuse "one section, no structure" "at least two"

	write_good_tree "$SELFTEST_BOX"
	printf '# NEXT — issue-7 unit\n\n- a note under no heading\n\n## What this unit shipped\n\n- x\n\n## What is still owed\n\n- y\n' \
		>"$SELFTEST_BOX/NEXT.md"
	st_refuse "a bullet above the first section" "orphaned bullet"

	write_good_tree "$SELFTEST_BOX"
	printf '# NEXT — issue-7 unit\n\n## What this unit shipped\n\n- \n\n## What is still owed\n\n- y\n' \
		>"$SELFTEST_BOX/NEXT.md"
	st_refuse "a bullet with nothing after the marker" "empty after its marker"

	write_good_tree "$SELFTEST_BOX"
	printf '\n- quote the verdict of `scripts/check-next-md-OLD.sh` in the body\n' >>"$SELFTEST_BOX/NEXT.md"
	st_refuse "a hand-off names a script that is not there" 'names `scripts/check-next-md-OLD.sh`'

	write_good_tree "$SELFTEST_BOX"
	printf '\n- copy it out of /tmp/opencode/scripts/ghost.sh first\n' >>"$SELFTEST_BOX/NEXT.md"
	printf '\n- the sets live under scripts/mutations/*.json\n' >>"$SELFTEST_BOX/NEXT.md"
	st_accept "an absolute path and a glob are not tool paths"

	write_good_tree "$SELFTEST_BOX"
	sed -i '/check-next-md\.sh/d' "$SELFTEST_BOX/Makefile"
	st_refuse "both verify steps are deleted" "add the step back"

	write_good_tree "$SELFTEST_BOX"
	sed -i '/check-next-md\.sh; /d' "$SELFTEST_BOX/Makefile"
	st_refuse "only the self-test step survives (#391)" "does not count"

	write_good_tree "$SELFTEST_BOX"
	sed -i '/--self-test/d' "$SELFTEST_BOX/Makefile"
	st_refuse "the self-test step is deleted" "someone remembers"

	write_good_tree "$SELFTEST_BOX"
	printf '## Step 6 — hand off\n\nLeave a hand-off.\n' >"$SELFTEST_BOX/.opencode/skills/factory/SKILL.md"
	st_refuse "the skill stops naming the file" "never mentions NEXT.md"

	write_good_tree "$SELFTEST_BOX"
	printf '## Step 6 — hand off\n\nWrite NEXT.md and run the gate.\n' \
		>"$SELFTEST_BOX/.opencode/skills/factory/SKILL.md"
	st_refuse "the skill names the file, not the gate" "part company"

	write_good_tree "$SELFTEST_BOX"
	rm -f "$SELFTEST_BOX/.opencode/skills/factory/SKILL.md"
	st_refuse "the skill file is gone" "no source in the tree"

	# The witness, and the reason this self-test step exists next to the plain one. A step can never
	# witness its own deletion -- the deleted clause is the only thing that ran the program which would
	# have noticed -- and `make verify` proved that on 14 Nov (#391): the decision-log step was deleted,
	# its `--self-test` line stayed, and the gate ran nothing while the recipe printed OK. So the
	# surviving half of this pair asks the question about the Makefile this file SHIPS IN, at $ship and
	# not at --root: delete the plain step in any later commit and this is the line that turns red from
	# inside the step that survived. Its own control is two cases up: "both verify steps are deleted" and
	# "only the self-test step survives" run the same awk over a fixture recipe with the lines removed,
	# so the refusal below is a working refusal and not a check that cannot see.
	local ship counts plain selft
	ship=$(cd -- "$(dirname -- "$SELF")/.." && pwd)
	ST_TOTAL=$((ST_TOTAL + 1))
	if [ ! -f "$ship/Makefile" ]; then
		printf '  FAIL  %-46s no Makefile above %s, so the wiring cannot be asked\n' \
			"the shipped verify step is still there" "$ship"
		ST_BAD=$((ST_BAD + 1))
	else
		counts=$(awk "$MAKEFILE_AWK" "$ship/Makefile")
		plain=${counts%% *}
		selft=${counts##* }
		if [ "$plain" -eq 0 ] || [ "$selft" -eq 0 ]; then
			printf '  FAIL  %-46s the recipe this file ships in runs %s step(s) and %s self-test\n' \
				"the shipped verify step is still there" "$plain" "$selft"
			printf '          step(s). A `verify:` that no longer runs `bash scripts/check-next-md.sh` is\n'
			printf '          #391 one gate later: the check exists, nothing invokes it, and the step that\n'
			printf '          printed this is the only witness left. Put the clause back.\n'
			ST_BAD=$((ST_BAD + 1))
		else
			printf '  ok    %-46s %s step(s) and %s self-test step(s) still in `make verify`\n' \
				"the shipped verify step is still there" "$plain" "$selft"
		fi
	fi

	if [ "$ST_BAD" -ne 0 ]; then
		printf 'check-next-md self-test: %d case(s), %d NOT refused or accepted as named\n' "$ST_TOTAL" "$ST_BAD"
		return 1
	fi
	printf 'check-next-md self-test: %d case(s), every refusal raised for the sentence named, both null variants accepted, the shipped step witnessed\n' \
		"$ST_TOTAL"
	return 0
}

usage() { printf '%s\n' "$USAGE"; }

main() {
	while [ $# -gt 0 ]; do
		case "$1" in
		--root)
			[ $# -ge 2 ] || { printf 'check-next-md: --root needs a directory\n' >&2; exit 2; }
			ROOT=$2
			shift 2
			;;
		--self-test)
			SELFTEST=1
			shift
			;;
		--help | -h)
			usage
			exit 0
			;;
		*)
			printf 'check-next-md: unknown argument `%s`\n' "$1" >&2
			usage >&2
			exit 2
			;;
		esac
	done
	if [ "$SELFTEST" -eq 1 ]; then
		if self_test; then
			exit 0
		fi
		exit 1
	fi
	run_legs "$ROOT"
	if [ "$COUNT" -ne 0 ]; then
		printf '%s' "$PROBLEMS" | while IFS= read -r line; do
			[ -n "$line" ] || continue
			printf '  FAIL  %s\n' "$line"
		done
		printf 'check-next-md: %d problem(s), listed above\n' "$COUNT"
		exit 1
	fi
	printf 'check-next-md: OK\n'
	return 0
}

main "$@"
