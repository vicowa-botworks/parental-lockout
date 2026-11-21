# shellcheck shell=bash
# shellcheck disable=SC2016,SC2034,SC2154
# 457-branch-body-claims.sh -- a body on an `issue-<N>` branch must name #N (#457).
#
# #457 measured the pull request that delivered #375 with a body naming `#375` zero times: the
# substring 375 appears three times in it (twice the branch string `issue-375`, once inside
# `selftest.sh:3756`), and no keyword or `Refs` line names the ticket, so GitHub registered no
# closing reference, triage counted the merged pull as claiming nothing -- correctly, by its
# design -- and re-dispatched the ticket eight minutes after the merge. `scripts/check-close-verb.py`
# now refuses that shape at write time: on a branch whose name claims a number, a body that names
# that number with NOTHING exits 1.
#
# What the legs pin, and the polarities:
#   (a) the fixture premise first: `git rev-parse --abbrev-ref HEAD` answers `issue-777` in the
#       fixture repo, and the checker sees that branch -- measured 21 Nov, an UNBORN branch answers
#       128/"HEAD", so the fixture carries a commit and this leg proves it before any verdict counts;
#   (b) the refusal: silence on a claiming branch exits 1 and names the branch and the number;
#   (c) the near-miss a human waves through: a bare `#777` link exits 1 too -- a link registers
#       with no queue, which is the whole mechanism of #457;
#   (d) the #454 specimen itself: a body healthy on its own terms (`Refs #12`) still exits 1 on
#       `issue-777`, because it names a different ticket than the branch carries;
#   (e) controls: the same silent bodies exit 0 on a branch with no number, through a
#       `--span-blind` pipe (the commit-message read `scripts/check-commit-verbs.sh` uses), and
#       under `--branch ''`; and `--branch issue-777` refuses inside the numberless repo, so the
#       override is proven independent of HEAD;
#   (f) #1256's rule applied to this tool's own stdout: the refusal sentence carries no closer
#       vocabulary adjacent to a number.
cbn_work=$work/cb457
# Absolute at load: every leg `cd`s into a fixture repo first, and a relative $HERE would then
# resolve against the fixture, not the tree (measured 21 Nov: the whole file went rc 2 on a
# python3 "can't open file" the first time it was run).
cbn_script=$(cd -- "$HERE/../scripts" && pwd)/check-close-verb.py
cbn_repo1=$cbn_work/repo-claims
cbn_repo2=$cbn_work/repo-plain
cbn_out=$cbn_work/last-out

cbn_say() { printf '            %s\n' "$1" >&2; }

# cbn_mkrepo <dir> <branch> -- a committed repo on a named branch. The commit is not decoration:
# rev-parse has no answer on an unborn HEAD (rc 128, measured above), and the checker -- correctly
# -- refuses to guess a branch it cannot read.
cbn_mkrepo() {
	mkdir -p -- "$1" || return 1
	git init -q "$1" || return 1
	git -C "$1" -c user.name=selftest -c user.email=selftest@invalid \
		commit --allow-empty -qm fixture || return 1
	git -C "$1" checkout -q -b "$2"
}

# cbn_run <dir> [args...] -- the checker with <dir> as cwd, answer captured to $cbn_out.
cbn_run() {
	local dir=$1
	shift
	( cd -- "$dir" && python3 "$cbn_script" "$@" ) >"$cbn_out" 2>&1
}

# cbn_want <label> <dir> <want-rc> [args...] -- run and compare exit codes, dumping the answer on
# a mismatch so a FAIL leg says what the checker actually said.
cbn_want() {
	local label=$1 dir=$2 want=$3
	shift 3
	cbn_run "$dir" "$@"
	local got=$?
	if [ "$got" != "$want" ]; then
		cbn_say "$label: exited $got, wanted $want -- the answer was:"
		sed 's/^/              /' "$cbn_out" >&2
		return 1
	fi
	return 0
}

cbn_fixture() {
	local got
	mkdir -p -- "$cbn_work" || return 1
	cbn_mkrepo "$cbn_repo1" issue-777 || return 1
	cbn_mkrepo "$cbn_repo2" topic-plain || return 1
	printf 'Delivers the leg.\n\nNothing names any ticket.\n' >"$cbn_work/silent.md" || return 1
	printf 'Refs #777.\n' >"$cbn_work/refs.md" || return 1
	printf 'See #777 for the finding.\n' >"$cbn_work/bare.md" || return 1
	printf 'Refs #12 -- a different ticket, spoken well.\n' >"$cbn_work/wrong.md" || return 1
	got=$(git -C "$cbn_repo1" rev-parse --abbrev-ref HEAD) || {
		cbn_say 'rev-parse refused the fixture repo; every leg below is about its answer'
		return 1
	}
	[ "$got" = issue-777 ] || {
		cbn_say "the fixture repo reads as branch $got, not issue-777"
		return 1
	}
	# The premise the whole file rests on: the checker, in that cwd, sees the claim when the body
	# names it -- a leg set where the checker could not see the branch would pass legs (e) vacuously.
	cbn_want 'premise: named body exits 0' "$cbn_repo1" 0 "$cbn_work/refs.md" &&
		grep -qF 'neutral #777' "$cbn_out"
}
claim_leg 'fixture: a committed repo whose branch claims #777' cbn_fixture

# (b) The refusal, and (f) its wording.
cbn_silent() {
	cbn_want 'silent body on a claiming branch' "$cbn_repo1" 1 "$cbn_work/silent.md" || return 1
	grep -qF 'REFUSED, branch issue-777 claims #777' "$cbn_out" || {
		cbn_say 'the refusal fired but never named the branch and the number together:'
		sed 's/^/              /' "$cbn_out" >&2
		return 1
	}
	! grep -qE 'closes #|CLOSES DESPITE' "$cbn_out" || {
		cbn_say 'the refusal itself carries closer vocabulary beside a number (#1256 rule):'
		sed 's/^/              /' "$cbn_out" >&2
		return 1
	}
}
claim_leg 'silence on an issue-777 branch is refused, in words' cbn_silent

# (c) The bare link -- the shape a writer believes is enough, and the exact hole #457 is.
cbn_bare() {
	cbn_want 'bare #777 link on a claiming branch' "$cbn_repo1" 1 "$cbn_work/bare.md" || return 1
	grep -qF 'REFUSED, branch issue-777 claims #777' "$cbn_out"
}
claim_leg 'a bare #777 link does not name the branch claim back' cbn_bare

# (d) #454's actual specimen: the body spoke, and not about this ticket.
cbn_wrong() {
	cbn_want 'body naming only #12 on a branch claiming #777' "$cbn_repo1" 1 "$cbn_work/wrong.md" ||
		return 1
	grep -qF 'REFUSED, branch issue-777 claims #777' "$cbn_out" || {
		cbn_say 'a body about another ticket silenced the leg -- the #454 hole exactly:'
		sed 's/^/              /' "$cbn_out" >&2
		return 1
	}
}
claim_leg 'Refs #12 does not answer a branch claiming #777' cbn_wrong

# (e) The three controls that keep the leg from becoming a tax: no number in the branch name, a
# commit-message pipe, and an explicit `--branch ''`.
cbn_controls() {
	cbn_want 'silent body on a numberless branch' "$cbn_repo2" 0 "$cbn_work/silent.md" || return 1
	(cd -- "$cbn_repo1" && printf 'Nothing names any ticket here.\n' |
		python3 "$cbn_script" --span-blind -) >"$cbn_out" 2>&1 || {
		cbn_say 'the span-blind pipe read turned red -- check-commit-verbs compat is broken:'
		sed 's/^/              /' "$cbn_out" >&2
		return 1
	}
	cbn_want "--branch '' silences the leg on a claiming branch" "$cbn_repo1" 0 \
		--branch '' "$cbn_work/silent.md"
}
claim_leg 'controls: numberless branch, message pipe, and branch silence' cbn_controls

# (e, cont.) The override works where HEAD does not: the numberless repo told to pretend.
cbn_override() {
	cbn_want '--branch issue-777 refuses inside the numberless repo' "$cbn_repo2" 1 \
		--branch issue-777 "$cbn_work/silent.md" || return 1
	grep -qF 'REFUSED, branch issue-777 claims #777' "$cbn_out"
}
claim_leg '--branch overrides HEAD in both directions' cbn_override
