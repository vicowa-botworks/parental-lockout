# shellcheck shell=bash
# shellcheck disable=SC2016,SC2034,SC2154
# 000-split-merge-clean.sh -- the first leg family that arrives as its own file, and the witness
# for why (#428).
#
# #428 measured the shared append tail: every unit that adds a leg appends a tail to
# `factory/selftest.sh`, two tails in one window conflict by construction, and a merge resolution
# has already dropped a unit's hand-off once (#337). The repair is one file per leg under
# `factory/selftest.d/`, sourced in sorted order by `run_selftest_d_legs`. This file is that
# repair's witness, and the first half of it is the falsification the ticket asked for:
#
#   control: two branches appending to the shipped `factory/selftest.sh` MUST still conflict. If
#     this leg ever passes on a conflict-free read, the tail was never the cause and #428's premise
#     is the thing to re-open -- that is why this direction is a leg and not a comment.
#   null variant: one-sided merges of each append branch answer clean, so a CONFLICT in the control
#     is the two tails meeting and not the merge machinery or the fixture failing.
#   repair: two branches adding distinct files under `factory/selftest.d/` merge CLEAN, both files
#     land with their own bytes, and after a real merge the runner sources all of them in LC_ALL=C
#     byte order.
#   kept hazard: two branches adding the SAME filename still conflict (`add/add`), because #381
#     weighed a `merge=union` driver that would merge a same-name append silently and this
#     repository chose the visible collision.
#   the runner: extracted from the shipped `factory/selftest.sh` by text and executed against
#     planted directories, so what is pinned is the loop the file ships, not a transcription of it.
#   the migration hazard #428 names: `pos_unconfigured_repos` (defined in the inline body) reads
#     `factory/selftest.sh` alone, so a leg that MOVES into this directory takes its `git init`
#     sites out of the identity scan silently. The last leg here runs that shipped function over
#     every file in this directory and plants its control on a stripped copy of THIS file.
#
# The contract this file is written against (see the block above `run_selftest_d_legs` in
# `factory/selftest.sh`): sourced, not executed; the fixture root `$work`, `$HERE`, and `claim_leg`
# are the parent's; assertions go through `claim_leg` and detail to stderr, because `set -e` is on
# and a bare failing command in a sourced file kills the suite.
s000_work=$work/selftest-d-split
s000_repo=$s000_work/repo

s000_say() { printf '            %s\n' "$1" >&2; }

s000_stub() { # <file> <marker> -- a leg stub that logs its marker when the runner sources it
	cat >"$1" <<STUB
printf '%s\n' '$2' >> "\${SPLIT000_LOG:?}"
STUB
}

s000_build() { # build the fixture repository; refusal text on stderr, rc 1 on failure
	mkdir -p "$s000_repo/factory/selftest.d" || return 1
	# -b main is host independence again: init.defaultBranch is the box's, and a fixture that
	# inherits it gets a `main` that may be called master the moment CI's git disagrees.
	git init -q -b main "$s000_repo" || return 1
	git -C "$s000_repo" config user.email split@example.invalid
	git -C "$s000_repo" config user.name "selftest split leg"
	git -C "$s000_repo" config commit.gpgsign false
	cp "$HERE/selftest.sh" "$s000_repo/factory/selftest.sh" || return 1
	s000_stub "$s000_repo/factory/selftest.d/000-main-placeholder.sh" main000 || return 1
	git -C "$s000_repo" add -A || return 1
	git -C "$s000_repo" commit -q -m 'ship the suite with a leg directory' || return 1
	# Two units that each do what every unit before #428 did: append a tail to the monolith.
	local u
	for u in alpha beta; do
		git -C "$s000_repo" checkout -q -b "unit-mono-$u" || return 1
		{
			printf '\n# unit %s appended its tail the way #428 describes\n' "$u"
			printf 's000_mono_%s() { :; }\n' "$u"
		} >>"$s000_repo/factory/selftest.sh" || return 1
		git -C "$s000_repo" commit -q -am "unit $u appends a tail" || return 1
		git -C "$s000_repo" checkout -q main || return 1
	done
	# Two units that each do what #428 makes possible: add a file nobody else touches.
	local pair
	for pair in 'unit-dir-alpha 810-alpha alpha' 'unit-dir-beta 820-beta beta'; do
		# shellcheck disable=SC2086  # the triple is one branch/file/marker record, split is the point
		set -- $pair
		git -C "$s000_repo" checkout -q -b "$1" || return 1
		s000_stub "$s000_repo/factory/selftest.d/$2.sh" "$3" || return 1
		git -C "$s000_repo" add factory/selftest.d || return 1
		git -C "$s000_repo" commit -q -m "unit $3 adds leg file $2" || return 1
		git -C "$s000_repo" checkout -q main || return 1
	done
	# Two units that collide on the same leg filename -- the hazard the split keeps, on purpose.
	for pair in 'unit-same-one 900-same one' 'unit-same-two 900-same two'; do
		# shellcheck disable=SC2086  # the triple is one branch/file/marker record, split is the point
		set -- $pair
		git -C "$s000_repo" checkout -q -b "$1" || return 1
		s000_stub "$s000_repo/factory/selftest.d/$2.sh" "$3" || return 1
		git -C "$s000_repo" add factory/selftest.d || return 1
		git -C "$s000_repo" commit -q -m "unit $3 adds leg file $2" || return 1
		git -C "$s000_repo" checkout -q main || return 1
	done
	return 0
}
claim_leg 'split fixture: repo with tails and leg files' s000_build

# The premise of every branch in this file: the fixture holds the shipped bytes, and each merge
# below is read while the repository still holds its `main`, so no branch is merged into itself.
s000_premise() {
	git -C "$s000_repo" rev-parse -q --verify main >/dev/null || {
		s000_say 'the fixture has no main branch'; return 1; }
	cmp -s "$s000_repo/factory/selftest.sh" "$HERE/selftest.sh" || {
		s000_say 'the fixture does not hold the shipped factory/selftest.sh byte for byte'; return 1; }
	local a b
	a=$(git -C "$s000_repo" rev-parse "unit-mono-alpha:factory/selftest.sh")
	b=$(git -C "$s000_repo" rev-parse "unit-mono-beta:factory/selftest.sh")
	[ "$a" != "$b" ] || { s000_say 'the two append branches hold the same blob'; return 1; }
	git -C "$s000_repo" show "unit-mono-alpha:factory/selftest.sh" | tail -n 2 | grep -q s000_mono_alpha || {
		s000_say 'the alpha tail is not at the end of its branch copy'; return 1; }
	git -C "$s000_repo" show "unit-mono-beta:factory/selftest.sh" | tail -n 2 | grep -q s000_mono_beta || {
		s000_say 'the beta tail is not at the end of its branch copy'; return 1; }
	# The null variant: each one-sided merge is clean, so a CONFLICT below is the two tails meeting.
	local one
	for one in unit-mono-alpha unit-dir-alpha unit-same-one; do
		git -C "$s000_repo" merge-tree --write-tree main "$one" >/dev/null 2>&1 || {
			s000_say "the one-sided merge of $one did not read clean; the fixture is the suspect"; return 1; }
	done
	return 0
}
claim_leg 'split fixture: shipped bytes, clean one-sides' s000_premise

# The FALSIFICATION leg of #428. The ticket claims the shared append tail is what conflicts; this
# is that claim, executable. If it ever answers clean against real appended tails, the tail is not
# the cause and #428's premise is falsified -- which is exactly the outcome the ticket asked to
# have a leg for, on the record either way.
s000_mono_conflicts() {
	local out rc=0
	out=$(git -C "$s000_repo" merge-tree --write-tree unit-mono-alpha unit-mono-beta 2>&1) || rc=$?
	[ "$rc" -ne 0 ] || {
		s000_say 'two appended tails merged CLEAN -- the shared-tail premise is falsified:'
		s000_say 'appends to factory/selftest.sh do not conflict by construction, and #428 owes a re-read'; return 1; }
	printf '%s\n' "$out" | grep -q CONFLICT || {
		s000_say "rc=$rc without a CONFLICT line -- not the verdict shape the leg reads"; return 1; }
	printf '%s\n' "$out" | grep -qF 'CONFLICT (content): Merge conflict in factory/selftest.sh' || {
		s000_say "the conflict named is not the shared tail:"; s000_say "$out"; return 1; }
	return 0
}
claim_leg 'control: two appended monolith tails conflict' s000_mono_conflicts

# The repair itself: with the append target moved into the directory, the same two-intentions merge
# is clean and BOTH additions land. Blob inequality is asserted because GitHub's ort resolves a
# both-added file with identical bytes silently -- only the different-name case is what units get,
# and only byte checks say so.
s000_d_clean() {
	local out rc=0 tree n
	out=$(git -C "$s000_repo" merge-tree --write-tree unit-dir-alpha unit-dir-beta 2>&1) || rc=$?
	[ "$rc" -eq 0 ] || { s000_say 'two distinct leg files did NOT merge clean:'; s000_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -q CONFLICT && { s000_say 'a clean rc still carried a CONFLICT line'; return 1; }
	tree=$(printf '%s\n' "$out" | head -n 1)
	n=$(git -C "$s000_repo" ls-tree -r --name-only "$tree" -- factory/selftest.d | wc -l)
	[ "$n" -eq 3 ] || {
		s000_say "the merged tree holds $n leg files, expected 3 (placeholder, 810-alpha, 820-beta):"
		s000_say "$(git -C "$s000_repo" ls-tree -r --name-only "$tree" -- factory/selftest.d)"; return 1; }
	local ab bb
	ab=$(git -C "$s000_repo" rev-parse "$tree:factory/selftest.d/810-alpha.sh")
	bb=$(git -C "$s000_repo" rev-parse "$tree:factory/selftest.d/820-beta.sh")
	[ "$ab" != "$bb" ] || { s000_say 'both leg files landed as the same blob'; return 1; }
	git -C "$s000_repo" cat-file -e "$tree:factory/selftest.d/810-alpha.sh" || {
		s000_say 'alpha lost its file'; return 1; }
	git -C "$s000_repo" cat-file -e "$tree:factory/selftest.d/820-beta.sh" || {
		s000_say 'beta lost its file'; return 1; }
	return 0
}
claim_leg 'two leg files under selftest.d merge clean' s000_d_clean

# The hazard the split deliberately keeps: two units choosing the same filename collide as a
# visible add/add conflict, which is what #381 weighed against a `merge=union` driver and what
# this repository chose. If this leg goes red, somebody made same-name appends merge silently.
s000_same_name_conflicts() {
	local out rc=0
	out=$(git -C "$s000_repo" merge-tree --write-tree unit-same-one unit-same-two 2>&1) || rc=$?
	[ "$rc" -ne 0 ] || {
		s000_say 'the same leg filename merged clean -- a silent union merge has arrived:'; return 1; }
	printf '%s\n' "$out" | grep -q 'CONFLICT (add/add)' || {
		s000_say "the same-name collision did not read as add/add:"; s000_say "$out"; return 1; }
	printf '%s\n' "$out" | grep -qF 'factory/selftest.d/900-same.sh' || {
		s000_say "the add/add names the wrong path:"; s000_say "$out"; return 1; }
	return 0
}
claim_leg 'a same-name leg file still conflicts, visibly' s000_same_name_conflicts

# The runner loop, pinned from the shipped text rather than transcribed: extracted by awk from
# `factory/selftest.sh` and EXECUTED against planted directories. A leg file can never witness
# its own discovery, so this is the loop the hazard actually lives in.
s000_runner_extract() { # -> prints the shipped function, or refuses
	local fn
	fn=$(awk '/^run_selftest_d_legs\(\)/,/^}/' "$HERE/selftest.sh")
	[ -n "$fn" ] || { s000_say "no run_selftest_d_legs in $HERE/selftest.sh -- the scan has nothing to read"; return 1; }
	printf '%s\n' "$fn"
}

s000_runner_legs() {
	local fn
	fn=$(s000_runner_extract) || return 1
	case "$fn" in
	*'sort -z'*) ;;
	*) s000_say 'the extracted loop does not sort NUL-safely -- it moved out from under this leg'; return 1; ;;
	esac
	printf '%s\n' "$fn" >"$s000_work/runner.sh" || return 1
	local d=$s000_work/d
	mkdir -p "$d/00-sub" || return 1
	local n
	for n in 010-a 1-c 10-b; do
		s000_stub "$d/$n.sh" "$n" || return 1
	done
	# maxdepth one and `*.sh` are both pinned by something that WOULD log if the glob let it in.
	s000_stub "$d/9-zz.txt" txt || return 1
	s000_stub "$d/00-sub/5.sh" sub || return 1
	# Byte order, not collation: '0' < '1', and '-' (0x2d) < '0' -- `LC_ALL=C` is the contract.
	printf '010-a\n1-c\n10-b\n' >"$s000_work/want.log" || return 1
	local rc=0
	( SPLIT000_LOG=$s000_work/order.log; SELFTEST_LEG_DIR=$d; . "$s000_work/runner.sh"; run_selftest_d_legs ) \
		|| rc=$?
	[ "$rc" -eq 0 ] || { s000_say "the extracted loop exited $rc on a planted directory"; return 1; }
	cmp -s "$s000_work/order.log" "$s000_work/want.log" || {
		s000_say 'the loop did not source exactly the *.sh files at maxdepth one, in LC_ALL=C order:'
		s000_say "$(cat "$s000_work/order.log" 2>/dev/null)"; return 1; }
	# The two quiet arms: an empty directory and an absent one are green with nothing sourced.
	local rc2=0 rc3=0
	mkdir -p "$s000_work/empty"
	( SPLIT000_LOG=$s000_work/empty.log; SELFTEST_LEG_DIR=$s000_work/empty; . "$s000_work/runner.sh"; run_selftest_d_legs ) || rc2=$?
	[ "$rc2" -eq 0 ] && [ ! -e "$s000_work/empty.log" ] || {
		s000_say "an empty leg directory is not silent-and-green (rc=$rc2)"; return 1; }
	( SPLIT000_LOG=$s000_work/absent.log; SELFTEST_LEG_DIR=$s000_work/no-such-dir; . "$s000_work/runner.sh"; run_selftest_d_legs ) || rc3=$?
	[ "$rc3" -eq 0 ] && [ ! -e "$s000_work/absent.log" ] || {
		s000_say "an absent leg directory is not silent-and-green (rc=$rc3)"; return 1; }
	return 0
}
claim_leg 'the runner sources every .sh once, in LC_ALL=C order' s000_runner_legs

# The end-to-end sentence a base sync needs: after a real merge of the two leg-file branches, the
# merged working tree's directory carries BOTH units' legs and the runner runs all three stubs --
# the placeholder that was in main first, because byte order puts `000-` before `810-`.
s000_merged_both_run() {
	local rc=0
	git -C "$s000_repo" checkout -q -b mergecheck unit-dir-alpha || rc=$?
	[ "$rc" -eq 0 ] || { s000_say "could not open the merge branch ($rc)"; return 1; }
	git -C "$s000_repo" merge -q --no-edit unit-dir-beta || rc=$?
	[ "$rc" -eq 0 ] || {
		s000_say "the two leg-file branches did not merge in a real worktree ($rc):"
		s000_say "$(git -C "$s000_repo" status --short)"; return 1; }
	[ -f "$s000_repo/factory/selftest.d/810-alpha.sh" ] || { s000_say 'alpha lost its file in the merge'; return 1; }
	[ -f "$s000_repo/factory/selftest.d/820-beta.sh" ] || { s000_say 'beta lost its file in the merge'; return 1; }
	local fn
	fn=$(s000_runner_extract) || return 1
	printf '%s\n' "$fn" >"$s000_work/runner.sh" || return 1
	local rc2=0
	( SPLIT000_LOG=$s000_work/merged.log
	  SELFTEST_LEG_DIR=$s000_repo/factory/selftest.d
	  . "$s000_work/runner.sh"; run_selftest_d_legs ) || rc2=$?
	[ "$rc2" -eq 0 ] || { s000_say "the runner refused the merged directory (rc=$rc2)"; return 1; }
	printf 'main000\nalpha\nbeta\n' >"$s000_work/merged-want.log"
	cmp -s "$s000_work/merged.log" "$s000_work/merged-want.log" || {
		s000_say 'the merged directory did not run both units'"'"' legs in sorted order:'
		s000_say "$(cat "$s000_work/merged.log" 2>/dev/null)"; return 1; }
	return 0
}
claim_leg 'after a real merge both units'"'"' legs run, sorted' s000_merged_both_run

# The migration hazard #428 names: the inline scan `pos_unconfigured_repos` reads
# `factory/selftest.sh` alone, so every `git init` that moves into this directory leaves the
# identity scan silently, and a leg that vacuously finds nothing is the failure mode #360
# measured. This leg runs the shipped function over every file here, and plants its control the
# way #360's did -- on a copy, outside the repository, with THIS file's identity stripped.
s000_identity_covers_dir() {
	type pos_unconfigured_repos >/dev/null 2>&1 || {
		s000_say 'pos_unconfigured_repos is not defined by the inline body -- the scan moved or died'; return 1; }
	local f gap bad=0
	for f in "$HERE"/selftest.d/*.sh; do
		[ -e "$f" ] || continue
		gap=$(pos_unconfigured_repos "$f")
		if [ -n "$gap" ]; then
			s000_say "a repository in ${f#"$HERE"/} was created without an identity within 3 lines:"
			s000_say "$gap"
			bad=1
		fi
	done
	[ "$bad" -eq 0 ] || return 1
	# The control, planted outside the repository: a copy of this file with the identity stripped
	# must come back as a gap. A scan that says nothing about it is blind to this directory.
	local control=$s000_work/identity-control
	mkdir -p "$control" || return 1
	sed '/config user\.email/d' "${BASH_SOURCE[0]}" >"$control/000-split-merge-clean.sh" || return 1
	grep -q 'git init' "$control/000-split-merge-clean.sh" || {
		s000_say 'the control copy has no init in it, so it tests nothing'; return 1; }
	pos_unconfigured_repos "$control/000-split-merge-clean.sh" | grep -q 'git init' || {
		s000_say 'the identity scan is blind to files in this directory -- a moved leg leaves its init sites unscanned'; return 1; }
	return 0
}
claim_leg 'the identity scan reads every leg file too' s000_identity_covers_dir
