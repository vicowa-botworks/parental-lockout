# shellcheck shell=bash
# shellcheck disable=SC2016,SC2034,SC2154
# 442-journal-claims.sh -- the allocator consumes what it journals (#442).
#
# #442 measured three units handed `H-0250` in 53 minutes from one journal: the taken-set was log
# headings only, so an id already journalled to a unit sat below the ceiling invisible, and the
# window was as wide as a whole tick (FACTORY_TICK_TIMEOUT). The repair reads the journal before
# answering: the LAST DECISION-ID line naming a candidate is its claim; another unit's claim on a
# branch that still exists as a ref here is taken and stepped over (with the whose-and-when trail
# journalled beside the answer); a claim whose branch exists nowhere is expired, because the entry
# it would have committed was never written -- issue-365 earned `H-0355`, died at the tick ceiling,
# and no branch ever followed. Same-unit claims keep their own id, so #320's re-read after writing
# an entry still answers the number earned.
#
# The polarities #442 asked for, and the house falsifier:
#   (a) two units, one journal, sequential reads -> different ids, and the answer says whose it
#       stepped over, on stderr and in the journalled line;
#   (b) one unit, two reads -> the same id, one line each, and NO step (the retry-after-conflict
#       polarity: without this leg the repair can silently break every re-read path);
#   (c) a claim whose branch never acquires a ref stops being taken (the #365 leak arm: claims
#       that never expire walk the id space upward on every dead tick);
#   (d) the claim map is a read of the FILE -- planted well-formed lines claim, and shapeless or
#       foreign-prefix lines do not;
#   (control) with the call neutered the collision reproduces verbatim -- two units, one id -- so
#       legs (a)/(d) measure the claim read and not the fixture.
jc_work=$work/jc442
jc_err=$jc_work/last-stderr
jc_alloc=$dpn_alloc

jc_say() { printf '            %s\n' "$1" >&2; }

# jc_run <repo> <logdir> <unit> <branch> [script] -- dpn_alloc_run's contract (the env strip is
# #425's: a supervisor tick exports FACTORY_TICK_UNIT, and a leg must pin its OWN attribution),
# with the branch a parameter because which claim is LIVE here is the whole question.
# shellcheck disable=SC2317  # reached from the legs below
jc_run() {
	local repo=$1 logdir=$2 unit=$3 branch=$4 script=${5:-$jc_alloc}
	mkdir -p -- "$logdir" || return 1
	( cd -- "$repo" && env -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH \
		FACTORY_LOG="$logdir" FACTORY_TICK_UNIT="$unit" \
		FACTORY_TICK_BRANCH="$branch" bash "$script" DECISIONS.md ) 2>"$jc_err"
}

# Fixture A: a bare repo (dpn_bare's shape, no remote, trunk carries D-0001) whose local branch
# `claim-alive` stands for the claimant branch #442 saw as `issue-334` -- present, so its claim is
# live. The allocator answers for units jca1 then jcb against ONE journal.
jc_a_repo=$jc_work/a-repo
jc_a_log=$jc_work/a-log
jc_a1='' jc_b=''
jc_a_build() {
	mkdir -p -- "$jc_work" || return 1
	dpn_bare "$jc_a_repo" || return 1
	jc_a1=$(jc_run "$jc_a_repo" "$jc_a_log" unclaimed:jca1 claim-alive) || {
		jc_say 'the allocator refused the first read; its stderr:'
		sed 's/^/              /' "$jc_err" >&2
		return 1
	}
	# The ref lands BEFORE the second read: the claim is live only while the branch exists, and
	# #442's middle line was a `REFRESH 0s` read four minutes after the claimant was handed its id.
	git -C "$jc_a_repo" branch claim-alive HEAD || return 1
	jc_b=$(jc_run "$jc_a_repo" "$jc_a_log" unclaimed:jcb branch-b) || {
		jc_say 'the allocator refused the second read; its stderr:'
		sed 's/^/              /' "$jc_err" >&2
		return 1
	}
	[ "$jc_a1" = D-0002 ] || { jc_say "the first read answered $jc_a1, not D-0002"; return 1; }
	grep -qF -- "DECISION-ID $jc_a1 unit=unclaimed:jca1 branch=claim-alive" "$jc_a_log/supervisor.log"
}
dpn_leg 'fixture: a live claim journalled by jca1' \
	'the first answer is D-0002 and journalled with its unit and branch' jc_a_build

# (a) The collision #442 measured is now refused: a DIFFERENT unit reading the same journal with
# the claimant branch present must not receive the claimed id, and must be told whose it skipped.
jc_a_step() {
	[ "$jc_b" = D-0003 ] || {
		jc_say "the second unit was answered $jc_b -- the live claim of unclaimed:jca1 on branch"
		jc_say 'claim-alive was consumed by nothing, which is the #442 defect itself'; return 1
	}
	grep -qF 'STEPPED-OVER D-0002' "$jc_err" || {
		jc_say 'the answer moved, but stderr never said it stepped over anything:'
		sed 's/^/              /' "$jc_err" >&2; return 1
	}
	grep -qF 'by unclaimed:jca1, whose branch claim-alive still exists' "$jc_err" || {
		jc_say 'the STEPPED-OVER line does not name the holder and the branch that keeps it live:'
		sed 's/^/              /' "$jc_err" >&2; return 1
	}
	grep -qF "DECISION-ID $jc_b unit=unclaimed:jcb" "$jc_a_log/supervisor.log" || {
		jc_say 'the stepped-over answer itself went unjournalled'; return 1
	}
	grep -qF '(D-0002 taken: journalled ' "$jc_a_log/supervisor.log" &&
		grep -qF 'by unclaimed:jca1 on branch claim-alive)' "$jc_a_log/supervisor.log"
}
dpn_leg 'two units one journal answer different ids' \
	'the second answer steps over the live claim and says whose it was, twice' jc_a_step

# (b) The polarity that stops the repair from breaking every retry path: the unit that already
# holds the claim still receives it, line after line, with no step. This is #320's re-read after
# writing an entry, and #442's note that five reads of issue-365 were correct by design.
jc_a_again() {
	local got lines_d2
	got=$(jc_run "$jc_a_repo" "$jc_a_log" unclaimed:jca1 claim-alive) || {
		jc_say 'the re-read refused outright'; return 1
	}
	[ "$got" = D-0002 ] || {
		jc_say "the owning unit was re-answered $got: same-unit idempotence is what separates"
		jc_say 'a claim from a consumption, and this arm losing is the repair eating retries'; return 1
	}
	grep -qF 'STEPPED-OVER' "$jc_err" && {
		jc_say 'a unit was stepped over its own journalled id'; return 1
	}
	lines_d2=$(grep -c 'DECISION-ID D-0002 unit=unclaimed:jca1' "$jc_a_log/supervisor.log" || true)
	[ "$lines_d2" = 2 ] || {
		jc_say "jca1 holds $lines_d2 D-0002 lines, expected 2 (the first read and the re-read)"; return 1
	}
	return 0
}
dpn_leg 'one unit two reads keep their id' \
	'the same answer, no step, and one journalled line per read' jc_a_again

# (c) The leak arm: an allocation whose branch never acquires a ref must stop being taken, or
# every dead tick inflates the id space forever (issue-365 is that case in the live journal).
jc_c_dead() {
	local c_repo=$jc_work/c-repo c_log=$jc_work/c-log first second
	dpn_bare "$c_repo" || return 1
	first=$(jc_run "$c_repo" "$c_log" unclaimed:jcd never-a-ref) || return 1
	second=$(jc_run "$c_repo" "$c_log" unclaimed:jce branch-e) || return 1
	if [ "$first" != D-0002 ] || [ "$second" != D-0002 ]; then
		jc_say "claimant branch never existed, yet the reads answered $first and $second;"
		jc_say 'a claim with no branch outliving its tick must expire (#365)'; return 1
	fi
	grep -qF 'STEPPED-OVER' "$jc_err" && {
		jc_say 'an expired claim still stepped the answer; it claims nothing'; return 1
	}
	return 0
}
dpn_leg 'a claim with no branch stops being taken' \
	'the next unit receives the very id the dead tick earned' jc_c_dead

# (d) The claim map is a read of the journal FILE: planted lines participate. Shapeless lines
# (no unit=, no branch=), foreign-prefix lines, and the `unknown` branch spelling are no claim;
# one appended well-formed live line IS, and moves the next answer by exactly one.
jc_d_planted() {
	local d_repo=$jc_work/d-repo d_log=$jc_work/d-log first second
	dpn_bare "$d_repo" || return 1
	mkdir -p -- "$d_log" || return 1
	{
		printf '%s DECISION-ID D-0002 (allocator, DECISIONS.md, REFRESH 0s union)\n' \
			'2026-11-21T00:00:00Z'
		printf '%s DECISION-ID H-0002 unit=ghost branch=ghost-alive\n' '2026-11-21T00:00:01Z'
		printf '%s DECISION-ID D-0002 unit=ghost branch=unknown\n' '2026-11-21T00:00:02Z'
	} >"$d_log/supervisor.log" || return 1
	first=$(jc_run "$d_repo" "$d_log" unclaimed:jcf branch-f) || return 1
	[ "$first" = D-0002 ] || {
		jc_say "planted shapeless/foreign/unknown lines moved the answer to $first: they are"
		jc_say 'not claims, and refusing them is what keeps other verbs in this journal inert'; return 1
	}
	git -C "$d_repo" branch ghost-alive HEAD || return 1
	printf '%s DECISION-ID D-0002 unit=ghost branch=ghost-alive\n' '2026-11-21T00:00:03Z' \
		>>"$d_log/supervisor.log" || return 1
	second=$(jc_run "$d_repo" "$d_log" unclaimed:jcg branch-g) || return 1
	[ "$second" = D-0003 ] || {
		jc_say "a planted well-formed live claim did NOT hold the id (answer: $second) -- the"
		jc_say 'claim map would be run-memory, not the journal this file writes'; return 1
	}
	grep -qF 'by ghost' "$jc_err" || {
		jc_say 'the step-over did not name the planted holder'; return 1
	}
	return 0
}
dpn_leg 'the claim map reads the journal file' \
	'well-formed lines claim, shapeless and foreign ones do not' jc_d_planted

# The control. Neuter the call and the collision reproduces exactly as #442 reported it: two
# units, one journal, one id -- so every leg above is measuring the claim read and not the
# fixture. The anchor is the call site's own tail comment, which exists to be unique.
jc_ctl_mutant() {
	local m=$jc_work/mutant.sh e_repo=$jc_work/e-repo e_log=$jc_work/e-log
	local first second
	dpn_mut "$dpn_alloc" 'the call is the read' \
		'# the claim read was deleted here for the control only' "$m" || return 1
	dpn_bare "$e_repo" || return 1
	git -C "$e_repo" branch claim-alive HEAD || return 1
	first=$(jc_run "$e_repo" "$e_log" unclaimed:jch claim-alive "$m") || return 1
	second=$(jc_run "$e_repo" "$e_log" unclaimed:jci branch-i "$m") || return 1
	[ "$first" = D-0002 ] || { jc_say "the mutant lost even the plain read ($first)"; return 1; }
	[ "$first" = "$second" ] || {
		jc_say "with the claim read deleted the units still disagreed ($first vs $second):"
		jc_say 'something ELSE is refusing ids now, and the leg above measures that, not this'; return 1
	}
	! grep -qF 'STEPPED-OVER' "$jc_err" || {
		jc_say 'the neutered script still stepped over something -- the plant landed nowhere'; return 1
	}
	return 0
}
dpn_leg 'control: without the read two units share an id' \
	'the #442 collision verbatim, so leg (a) is a mechanism not a fixture' jc_ctl_mutant
