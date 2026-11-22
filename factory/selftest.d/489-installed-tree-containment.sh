# shellcheck shell=bash
# shellcheck disable=SC2016,SC2034,SC2154
# 489-installed-tree-containment.sh -- the supervisor-pass fixtures resolve the INSTALLED unit
# inside the fixture box, never the machine's.
#
# The incident (#489, measured in the real installed reflog 16-20 Nov): the timer unit exports
# `Environment=FACTORY_SERVICE_UNIT=<its own name>` about itself; `factory.sh` forwards its own
# environment into each tick; a `make verify` inside that tick sourced `factory/selftest.sh`, whose
# preamble scrubbed four names but not this one; the supervisor-pass fixtures ran real supervisors
# whose `factory_service_unit` first arm honoured the inherited name, `factory_installed_tree`
# answered the OPERATOR's checkout, and the stale-repair arm fast-forwarded it -- 22 unjournalled
# `Fast-forward` entries with no `SUPERVISOR-FF` line anywhere, because the arm believed it was
# merging a fixture.
#
# What the legs pin, and the polarities:
#   (a) the pin: a pass whose environment carries the tick's exported pair FIRST and the shipped
#       proc_run pin values second -- the exact ordering `env` resolves last-wins -- does not
#       resolve the poisoned name: the planted behind-and-clean canary (named by a planted unit
#       file's `ExecStart`, which is what `factory_installed_tree` reads) keeps its HEAD and no
#       canary path appears in journal or stdout. Run against the pre-pin shipped list this leg is
#       RED: the poison wins, the arm merges, the canary moves -- that is the incident;
#   (b) the ownership gate: the same launcher with the poison RESOLVING -- a planted unit file the
#       pinned knobs reach -- must come back as `SUPERVISOR-FF-REFUSED not-this-pass`, not a merge:
#       the installed tree != the tree this pass supervises, so no write happens even though every
#       position condition for the FF holds. Run against an ungated arm this leg is RED: the arm
#       FFs the canary, which is the incident's write; and if the pair stops being honoured at all
#       the arm says nothing either, which the leg also refuses -- (a)'s silence must stay
#       containment, not an arm that never ran;
#   (c) the shipped shape: `proc_run`'s env list carries both names pinned, legs move them only
#       through the knobs, and no `proc_extra_env` entry carries the unit name -- that array lands
#       AFTER the shipped list, so it is the override channel the incident needs.
#
# Why a direct launcher and not `proc_run` itself: the defect lives in what is BEFORE the shipped
# list (the exported tick environment surviving the preamble), and by design no channel in
# `proc_run` puts a value there -- pin (c) forbids the one array that could. So this file assembles
# the shipped list literally, poison first, and `env`'s last-wins ordering does the rest. `git` is
# the real binary inside these passes (the proc family stubs `gh` only), so the repair arm's
# `git merge --ff-only` would really move the canary -- which is what (a) forbids and (b)
# demonstrates about the pair being honoured.
it489_work=$work/inst489
it489_bin=$it489_work/bin
it489_repo=$it489_work/repo
it489_units=$it489_work/poison-units
mkdir -p "$it489_work" "$it489_bin" "$it489_units" "$it489_work/home" \
	"$it489_work/units-pass"

# The harness the passes run from is a COPY of the shipped files placed INSIDE the fixture repo and
# committed there, exactly as the proc family commits its scripts into `$proc_up`: the question is
# which installed tree THAT supervisor resolves, and an uncommitted copy would read dirty and put
# the pass into quarantine before the repair arm was ever reached -- a refusal for the wrong reason
# is a leg that proves nothing. The payload directory, the `gh` stub and the model-CLI stub are the
# proc family's, copied so that a triage which succeeds is the default shape and not one this leg
# planted -- and because the pass preflights the model CLI's presence on PATH even when
# `FACTORY_MAX_TICKS=0` means it is never run.
cp -R "$proc_bin/gh" "$it489_bin/gh"
cp -R "$proc_bin/opencode" "$it489_bin/opencode"
chmod +x "$it489_bin/gh" "$it489_bin/opencode"
cp -R "$proc_fix" "$it489_work/fix"
# `--json` is a field list, and the shim enforces it from `fields.sh` beside the payloads: install
# it on the copy rather than trusting what the family directory happens to hold today.
install_gh_fields "$it489_work/fix"

# One fixture trunk with one commit that CARRIES the shipped scripts: the supervisor's status reads
# want a real checkout, its dirt verdict wants a clean one, and `FACTORY_BASE=main` is what the
# repair arm resolves its branch name from.
git init -q "$it489_repo"
git -C "$it489_repo" config user.email fixture@invalid
git -C "$it489_repo" config user.name fixture
git -C "$it489_repo" checkout -qb main
cp -R "$HERE" "$it489_repo/factory"
it489_harness=$it489_repo/factory
printf 'fixture trunk\n' >"$it489_repo/tracked.txt"
git -C "$it489_repo" add -A
git -C "$it489_repo" commit -qm 'fixture: the trunk and shipped scripts a supervisor pass starts on'

it489_say() { printf '%s\n' "$*" | sed 's/^/            /' >&2; }

# it489_worktree_clean <dir> -- the #228 family's plumbing verdict, not the porcelain question:
# the census in `selftest.sh` (`naive_dirt_tests`) refuses `status --porcelain` in any shipped
# script outside the probe, because that read exits 128 printing nothing on the exclude-file
# shapes the #228 legs measured, and a precondition that dies that way reads as "clean". rc 1 is
# dirty, rc 2 is git refusing to answer; the caller refuses both.
it489_worktree_clean() {
	local rc=0
	git -C "$1" diff --quiet >/dev/null 2>&1 || rc=$?
	git -C "$1" diff --cached --quiet >/dev/null 2>&1 || rc=$?
	[ -z "$(git -C "$1" ls-files --others --exclude-standard 2>/dev/null)" ] || rc=${rc:-1}
	return "$rc"
}

# it489_canary <dir> -- build a clone at <dir>/repo of a bare origin whose `main` is ONE commit
# ahead of the clone's HEAD: exactly the behind-and-clean state the stale-repair arm merges. Sets
# it489_canary_dir / _behind / _ahead. The pair is per-run so one pass moving a canary cannot
# vacuate a later leg's precondition; behind-and-clean is CHECKED, not assumed.
it489_canary() {
	local dir=$1
	mkdir -p "$dir"
	git init -q --bare "$dir/origin.git"
	git -C "$dir/origin.git" config user.email fixture@invalid
	git -C "$dir/origin.git" config user.name fixture
	git init -q "$dir/repo"
	git -C "$dir/repo" config user.email fixture@invalid
	git -C "$dir/repo" config user.name fixture
	git -C "$dir/repo" checkout -qb main
	printf 'a\n' >"$dir/repo/file.txt"
	git -C "$dir/repo" add -A
	git -C "$dir/repo" commit -qm 'a: the commit the installed tree holds'
	git -C "$dir/repo" remote add origin "$dir/origin.git"
	git -C "$dir/repo" push -q origin main
	printf 'b\n' >"$dir/repo/file.txt"
	git -C "$dir/repo" commit -qam 'b: the commit trunk advanced with'
	git -C "$dir/repo" push -q origin main
	git -C "$dir/repo" reset -q --hard HEAD~1
	git -C "$dir/repo" fetch -q origin
	[ "$(git -C "$dir/repo" rev-list --count HEAD..origin/main)" = 1 ] || return 1
	it489_worktree_clean "$dir/repo" || return 1
	it489_canary_dir=$dir/repo
	it489_canary_behind=$(git -C "$dir/repo" rev-parse HEAD)
	it489_canary_ahead=$(git -C "$dir/repo" rev-parse origin/main)
}

# it489_unit <file> <tree> -- a unit file naming <tree> the way the installer writes one:
# `factory_installed_tree` reads the single `ExecStart=` line and strips the
# `/factory/factory.sh` suffix, so that line IS the installed-tree claim. A file with no
# `ExecStart` would answer "not derivable", not "installed", and would prove nothing.
it489_unit() {
	printf '[Unit]\nDescription=plock fixture unit\n[Service]\nExecStart=%s/factory/factory.sh\n' \
		"$2" >"$1"
}

# it489_pass <tag> [NAME=value ...] -- a REAL supervisor pass (not `--dry-run`: the defect was a
# write, and a `would` note would prove a decision, not a reach) against the copied shipped files.
# `env -i` admits nothing ambient; the shipped proc_run list is spelled out below, and everything
# the caller appends lands AFTER it -- which is where `env` takes last-wins, and where a leg must
# NOT put the poison when the point of the run is that the shipped list wins.
it489_pass() {
	local tag=$1 rc=0
	shift
	env -i PATH="$it489_bin:/usr/bin:/bin" HOME="$it489_work/home" \
		PROC_FIXTURE="$it489_work/fix" PROC_GH_REC="$it489_work/$tag.ghrec" \
		FACTORY_HARNESS="$it489_harness" FACTORY_DIR="$it489_harness" FACTORY_ROOT="$it489_repo" \
		FACTORY_REPO=example/repo FACTORY_REVIEWER=someoneelse \
		FACTORY_OPENCODE=opencode FACTORY_AGENT=factory \
		FACTORY_LOG="$it489_work/$tag.log" FACTORY_LEDGER="$it489_work/$tag.ledger" \
		FACTORY_RUN="$it489_work/$tag.run" FACTORY_TREES="$it489_work/$tag.trees" \
		FACTORY_AT_RISK="$it489_work/$tag.risk" FACTORY_HALT="$it489_work/$tag.HALT" \
		FACTORY_BASE=main FACTORY_MAX_TICKS=0 FACTORY_BUDGET=30 \
		"$@" \
		timeout --signal=TERM --kill-after=5 90 bash "$it489_harness/factory.sh" \
		>"$it489_work/$tag.out" 2>&1 || rc=$?
	cat -- "$it489_work/$tag.log/supervisor.log" "$it489_work/$tag.out" \
		>"$it489_work/$tag.journal" 2>/dev/null || : >"$it489_work/$tag.journal"
	it489_pass_rc=$rc
}

# What the SHIPPED `proc_run` list supplies for the installed-unit pair, read out of the file
# rather than assumed: leg (a) must replay what the shipped code would put after the poison, so a
# tree without the pin replays no pin and goes RED -- a leg that supplied its own fix would pass
# on the broken tree it exists to catch. `$proc_work` re-aims at this fixture's box; the knob
# lines are outside the `env` command and never enter the extraction.
it489_pin_env=()
while IFS= read -r it489_pin_line; do
	it489_pin_env+=("$it489_pin_line")
done < <(
	awk '/env PATH="\$proc_bin:\$PATH"/{f=1} f{print} f&&/timeout --signal/{exit}' \
		"$HERE/selftest.sh" |
		tr -d '"' |
		sed -e 's/^[[:space:]]*//' -e "s#\$proc_work#$it489_work#g" |
		grep -E '^FACTORY_(UNITS_DIR|SERVICE_UNIT)=' || true
)

# ---- (a) the pin beats the exported poison. The poison is first; whatever the shipped list
# supplies lands second, where `env` takes last-wins -- the ordering the fixed `proc_run` produces
# when a tick exports the pair, and the ordering the incident produced when it did not.
it489_containment() {
	it489_canary "$it489_work/canary-plain" || {
		it489_say 'precondition: the plain canary is not behind-and-clean.'; return 1
	}
	it489_unit "$it489_units/probe-poison.service" "$it489_canary_dir"
	local canary=$it489_canary_dir behind=$it489_canary_behind
	it489_pass plain \
		FACTORY_SERVICE_UNIT=probe-poison.service FACTORY_UNITS_DIR="$it489_units" \
		"${it489_pin_env[@]}"
	grep -q 'SUPERVISOR-AT' "$it489_work/plain.journal" || {
		it489_say 'the plain pass never journaled SUPERVISOR-AT, so nothing below was observed.'
		sed 's/^/              /' "$it489_work/plain.journal" >&2
		return 1
	}
	[ "$(git -C "$canary" rev-parse HEAD)" = "$behind" ] || {
		it489_say "the behind-and-clean canary moved to $(git -C "$canary" rev-parse HEAD),"
		it489_say "expected $behind. The tick's exported pair won over the shipped pin and a"
		it489_say 'fixture supervisor fast-forwarded the tree the poisoned unit names -- #489,'
		it489_say 'reproduced against the shipped files.'
		return 1
	}
	if grep -qF -- "$canary" "$it489_work/plain.journal"; then
		it489_say 'the pinned pass named the poisoned canary in its journal:'
		grep -F -- "$canary" "$it489_work/plain.journal" | sed 's/^/              /' >&2
		return 1
	fi
	return 0
}
claim_leg 'the shipped pin beats a tick-exported unit pair' it489_containment

# ---- (b) the ownership gate: a pass must never fast-forward a tree it does not supervise. Plant
# the unit file BEFORE the pass (a file planted after the pass it claims to condition is a fixture
# that never conditioned anything). Pre-gate this leg is RED -- the arm FFs the canary, which is
# #489's write against a tree the pass does not run; post-gate it must answer the refusal.
it489_pinned() {
	local dir=$it489_work/canary-real
	it489_canary "$dir" || {
		it489_say 'precondition: the pinned canary is not behind-and-clean.'; return 1
	}
	mkdir -p "$dir/units"
	it489_unit "$dir/units/probe-poison.service" "$it489_canary_dir"
	local canary=$it489_canary_dir behind=$it489_canary_behind
	it489_pass pinned-fixture \
		FACTORY_SERVICE_UNIT=probe-poison.service FACTORY_UNITS_DIR="$dir/units"
	grep -q 'SUPERVISOR-AT' "$it489_work/pinned-fixture.journal" || {
		it489_say 'the pinned pass never journaled SUPERVISOR-AT:'
		sed 's/^/              /' "$it489_work/pinned-fixture.journal" >&2
		return 1
	}
	if grep -qF "SUPERVISOR-FF-REFUSED not-this-pass tree=$canary" "$it489_work/pinned-fixture.journal"; then
		[ "$(git -C "$canary" rev-parse HEAD)" = "$behind" ] || {
			it489_say 'the ownership gate named the canary and merged it anyway.'
			return 1
		}
		it489_say 'shape: SUPERVISOR-FF-REFUSED not-this-pass: named, refused, unmoved.'
		return 0
	fi
	grep -q "SUPERVISOR-FF .* tree=$canary " "$it489_work/pinned-fixture.journal" && {
		it489_say 'the repair arm fast-forwarded a tree this pass does not supervise: the'
		it489_say 'ownership gate is gone and #489 writes again -- installed != FACTORY_ROOT'
		it489_say 'must come back as SUPERVISOR-FF-REFUSED not-this-pass before any merge.'
		return 1
	}
	it489_say 'the pass pinned to the canary neither FF-d it nor refused it as not-this-pass --'
	it489_say 'the installed-unit pair is NOT honoured by the fixtures, so the ok on the pin leg'
	it489_say 'above would be the silence of an arm that never ran:'
	sed 's/^/              /' "$it489_work/pinned-fixture.journal" >&2
	return 1
}
claim_leg 'the repair arm refuses to merge a tree it does not supervise' it489_pinned

# ---- (c) the shipped shape, as a prohibition on the seam itself.
it489_shape() {
	local self="$HERE/selftest.sh"
	grep -qF 'FACTORY_UNITS_DIR="$proc_work/units-pass" FACTORY_SERVICE_UNIT=proc-pass-factory.service' "$self" || {
		it489_say 'proc_run no longer ships both installed-unit names pinned to fixture values;'
		it489_say 'a tick-exported pair can reach a pass again (#489).'
		return 1
	}
	grep -q 'pass_knobs+=(FACTORY_UNITS_DIR="$proc_units_dir")' "$self" || {
		it489_say 'the fixture-unit-directory knob is gone from pass_knobs, so a leg that needs'
		it489_say 'a planted fixture unit can only reach for proc_extra_env again.'
		return 1
	}
	grep -q 'pass_knobs+=(FACTORY_SERVICE_UNIT="$proc_service_unit")' "$self" || {
		it489_say 'the fixture-unit-name knob is gone from pass_knobs (same reason).'
		return 1
	}
	if grep -q 'proc_extra_env=.*FACTORY_SERVICE_UNIT' "$self"; then
		it489_say 'a proc_extra_env entry is carrying FACTORY_SERVICE_UNIT again. That array'
		it489_say 'lands AFTER the shipped list -- it is the override channel the incident needs,'
		it489_say 'and the installed-unit pair must travel on proc_units_dir/proc_service_unit.'
		return 1
	fi
	return 0
}
claim_leg 'proc_run ships the pair pinned; legs move it only by knobs' it489_shape
