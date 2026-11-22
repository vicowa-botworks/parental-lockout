# shellcheck shell=bash
# shellcheck disable=SC2016,SC2034,SC2154
# 506-refname-taken-set.sh -- the ref face of a filed log claims by NAME and by deduped CONTENT (#506).
#
# #505 shipped the sibling-blindness repair as a per-entry `git show` on every ref's filed copies,
# and its own review measured what that costs: 6.7 s -> 10.6 s on a 1372-ref clone -- one process
# spawn per entry per ref per answer, unbounded once phase 3 files hundreds of entries onto the
# trunk every ref descends from, because then every ref pays every entry. #506 keeps BOTH claims
# the old read made and drops the product: the basename claims (EMITS names the file to create and
# the gate refuses filename != heading, `layout_problems`), and the bytes claim too -- but read
# once per UNIQUE blob, because identical bytes claim identical ids on every ref that carries them
# and the union takes a maximum. The two polarities below pin each half of the union to a fixture
# the other half answers differently, and the shape legs pin the mechanism the speed comes from.
#
# The polarities, with the house falsifier:
#   (a) a filed NAME HIGHER than every heading anywhere, on a ref, moves the ceiling to name+1 --
#       while the file's own heading decoys the content half downward. The answer can only be
#       name+1 if the name was read;
#   (b) a heading INSIDE a ref's filed blob, HIGHER than every name anywhere, still claims its
#       number -- the content half of the union, kept, deduped, not deleted;
#   (c) an EMPTY filed file on a ref still claims: the content read finds nothing there and the
#       name claim must not care;
#   (d) the shape rule behind the speed rule: the dir arm of face_ref contains no blob read -- no
#       `git show`, no `cat-file` -- and the content claim runs once per sha, gated by `sort -u`;
#   (controls) mutants answer differently from the shipped script on the same fixtures, and the
#       shape greps go red on the planted regressions, so legs (a)-(d) measure mechanisms.
rn_work=$work/rn506
rn_err=$rn_work/last-stderr

rn_say() { printf '            %s\n' "$1" >&2; }

# rn_run <repo> <unit> <script> -- the allocator's stdout for the harness log, this leg's own
# attribution (#425's strip), stderr captured.
# shellcheck disable=SC2317  # reached from the legs below
rn_run() {
	( cd -- "$1" && env -u FACTORY_TICK_UNIT -u FACTORY_TICK_BRANCH \
		FACTORY_LOG="$rn_work/log" FACTORY_TICK_UNIT="$2" \
		bash "${3:-$dpn_alloc}" factory/DECISIONS.md ) 2>"$rn_err"
}

# Fixture: dpn_bare's shape (both monoliths on `main`, no remote), plus a sibling branch that
# survives as a ref here -- the shape #442 priced -- carrying ONE filed file whose NAME says
# H-0042 and whose CONTENT says H-0001. Names and headings are made to disagree on purpose:
# whichever number the answer carries names which half claimed. The fixture also carries the two
# shapes that are not claims, `notes.md` (no id) and `H-042.md` (wrong digit count), so an answer
# of H-0043 says they were refused too.
rn_repo=$rn_work/repo
rn_build() { # <repo> -- dpn_bare's shape plus the filed sibling branch (see above)
	local repo=$1
	mkdir -p -- "$rn_work" || return 1
	dpn_bare "$repo" || return 1
	(
		cd -- "$repo" || exit 1
		git checkout -qb filed-sib || exit 1
		mkdir -p factory/DECISIONS || exit 1
		printf '## H-0001 · decoy heading under a higher name\n' >factory/DECISIONS/H-0042.md || exit 1
		printf 'prose, no heading at all\n' >factory/DECISIONS/notes.md || exit 1
		printf 'prose under a wrong-digit name, no heading either\n' >factory/DECISIONS/H-042.md || exit 1
		git add -A && git commit -qm 'filed sibling: name says H-0042, heading says H-0001' || exit 1
		git checkout -q main
	) || return 1
}
rn_answer=''
rn_shipped() {
	rn_build "$rn_repo" || return 1
	rn_answer=$(rn_run "$rn_repo" unclaimed:rn1) || {
		rn_say 'the allocator refused the read; its stderr:'
		sed 's/^/              /' "$rn_err" >&2
		return 1
	}
	[ "$rn_answer" = H-0043 ] || {
		rn_say "the answer was $rn_answer, not H-0043: the filed NAME H-0042 on ref filed-sib"
		rn_say 'is a claim on that id whatever the bytes inside it say'
		return 1
	}
}
dpn_leg 'a filed name above every heading claims' \
	'the ceiling steps to the name, not the decoy heading inside it' rn_shipped

# (control) The name half removed and the content half left in place -- the exact complement of
# the shipped read on this fixture: the decoy heading H-0001 is the only thing left to claim, so
# the answer must fall back to H-0002. If the mutant answered H-0043 too, leg (a) would be
# measuring the fixture rather than the name read.
rn_ctl_nameless() {
	local m=$rn_work/mutant-nameless.sh got
	dpn_mut "$dpn_alloc" 'printf '"'"'R ## %s\n'"'"' "${base%.md}"' \
		'[DH]-[0-9][0-9][0-9][0-9]) : # the name claim is deleted, the blob ledger is left alone ;;' "$m" || return 1
	rn_build "$rn_work/repo2" || return 1
	got=$(rn_run "$rn_work/repo2" unclaimed:rn2 "$m") || {
		rn_say 'the nameless mutant refused the read; its stderr:'
		sed 's/^/              /' "$rn_err" >&2
		return 1
	}
	[ "$got" = H-0002 ] || {
		rn_say "the nameless mutant answered $got, not H-0002: with the names unread the"
		rn_say 'decoy heading H-0001 is the whole content claim, so anything else means the'
		rn_say 'mutant landed nowhere and leg (a) proves nothing about which half is read'
		return 1
	}
}
dpn_leg 'control: without the name read the answer falls' \
	'the content half alone ceilings this fixture at H-0002' rn_ctl_nameless

# (b) The OTHER half of the union: a heading inside a ref's filed blob, HIGHER than every filed
# NAME anywhere. #505's read claimed this, and #506 must keep claiming it from the deduped blob
# stage -- a repair that shipped names only would silently hand back an id this blob already
# holds on a ref. (The pair of fixtures is the lattice: (a) answers by name where content decoys
# down, this answers by content where names decoy down; a names-only read passes (a) and fails
# this, a contents-only read is the mutant below.)
rn_ctl_content() {
	local e_repo=$rn_work/heading-above-name got
	dpn_bare "$e_repo" || return 1
	(
		cd -- "$e_repo" || exit 1
		git checkout -qb filed-high || exit 1
		mkdir -p factory/DECISIONS || exit 1
		printf '## H-0099 · heading above its filed name\n' >factory/DECISIONS/H-0002.md || exit 1
		git add -A && git commit -qm 'filed H-0002.md whose heading claims H-0099' || exit 1
		git checkout -q main
	) || return 1
	got=$(rn_run "$e_repo" unclaimed:rn4) || {
		rn_say 'the allocator refused the read; its stderr:'
		sed 's/^/              /' "$rn_err" >&2
		return 1
	}
	[ "$got" = H-0100 ] || {
		rn_say "the answer was $got, not H-0100: the blob filed as H-0002.md on ref filed-high"
		rn_say 'carries ## H-0099, and the content claim #505 made must survive the dedup'
		return 1
	}
}
dpn_leg 'a heading above its name still claims from refs' \
	'the deduped content half is kept, not traded away for the name read' rn_ctl_content

# (control) The content half removed and the name half left in place -- the names-only script the
# review's tie-break refused. Same fixture, and the answer must fall back to H-0003: the union
# here is {H-0001 trunk, H-0002 name}, so leg (b) measures the content half and not the fixture.
rn_ctl_nameonly() {
	local m=$rn_work/mutant-nameonly.sh got
	dpn_mut "$dpn_alloc" 'git -C "$root" cat-file blob "$blob" 2>/dev/null || true' \
		': # the content claim is deleted; the names above are left alone' "$m" || return 1
	local h_repo=$rn_work/repo6
	dpn_bare "$h_repo" || return 1
	(
		cd -- "$h_repo" || exit 1
		git checkout -qb filed-high || exit 1
		mkdir -p factory/DECISIONS || exit 1
		printf '## H-0099 · heading above its filed name\n' >factory/DECISIONS/H-0002.md || exit 1
		git add -A && git commit -qm 'filed H-0002.md whose heading claims H-0099' || exit 1
		git checkout -q main
	) || return 1
	got=$(rn_run "$h_repo" unclaimed:rn5 "$m") || {
		rn_say 'the names-only mutant refused the read; its stderr:'
		sed 's/^/              /' "$rn_err" >&2
		return 1
	}
	[ "$got" = H-0003 ] || {
		rn_say "the names-only mutant answered $got, not H-0003: with no blob read anywhere the"
		rn_say 'filed name H-0002 is the whole claim, so anything else means the mutant'
		rn_say 'landed nowhere and leg (b) proves nothing about the deduped content half'
		return 1
	}
}
dpn_leg 'control: without the content read the answer falls' \
	'the name half alone ceilings this fixture at H-0003' rn_ctl_nameonly

# (c) The shape the content read was blind to by construction: an entry filed as an EMPTY file --
# a unit that ran EMITS, created the file, and died before writing the entry. The bytes hold
# nothing to find; the name is the whole claim, and this is the case the name half was added for.
rn_empty() {
	local e_repo=$rn_work/empty-repo got
	dpn_bare "$e_repo" || return 1
	(
		cd -- "$e_repo" || exit 1
		git checkout -qb filed-empty || exit 1
		mkdir -p factory/DECISIONS || exit 1
		: >factory/DECISIONS/H-0050.md || exit 1
		git add -A && git commit -qm 'filed empty entry: a name with no heading at all' || exit 1
		git checkout -q main
	) || return 1
	got=$(rn_run "$e_repo" unclaimed:rn3) || {
		rn_say 'the allocator refused the read; its stderr:'
		sed 's/^/              /' "$rn_err" >&2
		return 1
	}
	[ "$got" = H-0051 ] || {
		rn_say "an empty filed H-0050.md on a ref yielded $got, not H-0051: a unit that ran"
		rn_say 'EMITS and died holding the pen still claims the id EMITS named'
		return 1
	}
}
dpn_leg 'an empty filed file on a ref still claims' \
	'the name is the claim; the content half has nothing to find there' rn_empty

# (d) The shape rule behind the speed rule: #506 prices the SPAWN PRODUCT -- one listing-spawn
# per ref times one read-spawn per entry -- so the answer must read blobs once per unique sha and
# never from inside the per-ref arm. This reads the SHIPPED bytes, not a run: a leg that timed the
# box would go red on a busy supervisor, and the cost is a property of the code.
rn_dir_arm() { # <file?> -- just the dir arm of face_ref: the monolith arm's `git show` is legal
	# and must not trip the shape grep, so the extraction stops at the `else` that leaves the dir
	# case. The optional file argument lets the shape CONTROL extract the MUTANT through this same
	# extraction (#518 review): the control greps exactly the bytes leg (d) greps, and the awk
	# range has one definition in the file -- a copy of the range inside the control could drift
	# from the leg's, which is the blindness this control exists to catch.
	sed -n '/^face_ref()/,/^}/p' "${1:-$dpn_alloc}" | awk '/= dir \]; then/,/^\telse$/'
}
rn_blob_stage() { # the shipped stage that reads the ledger back: from the dedup gate to the read
	awk 'index($0, "sort -u -- \"$filed_blobs\"") { on = 1 }
	     on { print }
	     on && index($0, "cat-file blob") { exit }' "$1"
}
rn_shape() {
	local arm stage
	arm=$(rn_dir_arm)
	[ -n "$arm" ] || { rn_say 'face_ref could not be located, so nothing below is a fact'; return 1; }
	if printf '%s\n' "$arm" | grep -Eq 'git [^|;]*(show|cat-file)'; then
		rn_say 'face_ref reads blob CONTENT per path again -- the spawn product #506 priced,'
		rn_say 'planted back into the shipped file:'
		printf '%s\n' "$arm" | grep -En 'git [^|;]*(show|cat-file)' | sed 's/^/              /' >&2
		return 1
	fi
	printf '%s\n' "$arm" | grep -qF 'ls-tree' || {
		rn_say 'face_ref no longer lists the ref tree, so where do the names and shas come from?'
		return 1
	}
	stage=$(rn_blob_stage "$dpn_alloc")
	[ -n "$stage" ] || {
		rn_say 'the blob stage that reads the deduped ledger could not be located: the content'
		rn_say 'claim #505 made is gone, and this leg would be praising a names-only script'
		return 1
	}
	printf '%s\n' "$stage" | grep -q 'sort -u' || {
		rn_say 'the blob stage lost its dedup gate -- without sort -u the read is back to one'
		rn_say 'spawn per path per ref, which is the exact cost this unit was cut for:'
		printf '%s\n' "$stage" | sed 's/^/              /' >&2
		return 1
	}
	printf '%s\n' "$stage" | grep -q 'cat-file blob' || {
		rn_say 'the blob stage no longer reads the blobs the ledger names'
		return 1
	}
}
dpn_leg 'the dir arm lists; the blob stage dedupes' \
	'no read inside face_ref, and sort -u gates every cat-file' rn_shape

# (control) Plant the per-path content read back into the arm and the shape leg must go red on
# it -- the proof that rn_shape inspects the mechanism rather than passing on whatever the file
# happens to contain. The plant keeps the script parseable (dpn_mut checks), so the refusal is
# the shape. The grep runs over rn_dir_arm's OWN extraction of the mutant, not the whole
# function: the function always carries the monolith arm's legitimate `git show`, so a
# whole-function grep matches the shipped unmutated file (measured 22 Nov on the pre-repair
# copy: the grep matched `git -C "$root" show "${ref}^{commit}:$relx"`) and could never report
# the plant invisible -- the scope narrowing this control must survive. Extracting through the
# leg's own function is what makes the claim below falsifiable: measure the leg with a narrowed
# awk range and the plant falls outside the shared scope, and this leg goes red naming it.
rn_ctl_shape() {
	local m=$rn_work/mutant-shape.sh arm
	dpn_mut "$dpn_alloc" 'printf '"'"'%s\n'"'"' "${meta##* }" >>"$filed_blobs" ;;' \
		'git -C "$root" show "${ref}^{commit}:$path" 2>/dev/null | grep -m1 -E "^## [DH]-[0-9][0-9][0-9][0-9]" || true ;;' \
		"$m" || return 1
	arm=$(rn_dir_arm "$m")
	printf '%s\n' "$arm" | grep -Eq 'git [^|;]*(show|cat-file)' || {
		rn_say 'the planted content read is INVISIBLE to the shape grep -- the leg would pass'
		rn_say 'the blob read back in, which is the exact regression it exists to refuse'
		return 1
	}
}
dpn_leg 'control: the shape grep catches the blob read' \
	'the planted arm goes red, so leg (d) is a gate not a comment' rn_ctl_shape

# (control) And the dedup gate is a gate too: delete the `sort -u` and leg (d) goes red -- at the
# blob-stage LOCATE check first, not at the second grep: the mutant replaces the very line
# rn_blob_stage locates the stage by, so the stage cannot be found before any gate inside it is
# read (measured 22 Nov: rn_shape on this mutant fails at 'could not be located'). The locate
# address is load-bearing, and this control plants into that address and says so. Answers would
# not change (the union is a maximum and identical bytes claim identical ids) -- which is exactly
# why the cost has to be pinned on the code, not on a run.
rn_ctl_dedup() {
	local m=$rn_work/mutant-nodedup.sh stage
	dpn_mut "$dpn_alloc" 'sort -u -- "$filed_blobs" 2>/dev/null |' \
		'cat -- "$filed_blobs" 2>/dev/null |' "$m" || return 1
	stage=$(sed -n '/cat -- "\$filed_blobs"/,/cat-file blob/p' "$m")
	[ -n "$stage" ] || {
		rn_say 'the dedup mutant could not even be located by the stage address -- which means'
		rn_say 'the address itself is load-bearing and needs a look'
		return 1
	}
	printf '%s\n' "$stage" | grep -q 'sort -u' && {
		rn_say 'the dedup mutant STILL carries sort -u inside the blob stage: the plant landed'
		rn_say 'somewhere the shape leg never reads, so that leg is not reading the stage'
		return 1
	}
	return 0
}
dpn_leg 'control: deleting the dedup gate goes red' \
	'the stage without sort -u is exactly what rn_shape refuses' rn_ctl_dedup
