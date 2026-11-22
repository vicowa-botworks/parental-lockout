# shellcheck shell=bash
# shellcheck disable=SC2016,SC2034,SC2154
# 503-scratch-path-safe.sh -- the name the dispatcher mints for a unit's scratch is PATH-safe, so a
# session can prepend `$FACTORY_SCRATCH/bin` to `PATH` and have the prepend mean what it says.
#
# The defect (#503, live the whole time the scratch has existed): the mint at `factory/factory.sh`
# names the directory after the unit, and an `unclaimed:<n>` unit id carries a colon -- so the minted
# name did too, `/tmp/plock-unclaimed:448-XXXXXX`. PATH is colon-delimited, so every prepend through
# that directory contributes TWO search entries, `/tmp/plock-unclaimed` and `448-XXXXXX/bin`, and
# neither exists: the prepend silently stops working and a tick's stub binaries are not found. The
# prompt tells the tick "throwaway fixtures go there and nowhere else", so the first thing a unit
# does with the scratch is the thing that fails -- and it fails as `command not found` from a distant
# command, never as anything naming the mint.
#
# `factory_slug` is not the bug and stays as it is: it maps `/` and nothing else, and it is how every
# branch becomes a tree path, so widening it renames trees. The mint is the one place that knows the
# name is going to be used as path text, so the mint is where the `:` goes.
#
# What the legs pin, and the polarities:
#   (a) the ground the control stands on: a PATH prepend through a colon-bearing directory does NOT
#       find a stub planted inside it, and the same prepend through the same name without the colon
#       DOES. Run on a shell where the first half found the stub, this leg is RED and says so -- a
#       leg whose null variant cannot fail is a control that proves nothing;
#   (b) the shipped dispatch: a session-inside probe plants a stub in `$FACTORY_SCRATCH/bin`,
#       prepends that directory to PATH, and records whether `command -v` found it. GREEN needs the
#       minted path to carry no colon, to still echo the unit id (`plock-unclaimed-900-`), to be
#       outside the tree, and to have the stub FOUND through the prepend -- plus the teardown having
#       deleted it, since the `plock-` prefix is what the teardown's case matches. Against the pre-fix
#       mint this leg is RED on both the colon and the missing stub, which is what (c) demonstrates;
#   (c) the control: the same fixture with the mint mutated back to what #503 was filed on --
#       `plock-$(factory_slug "$unit")`, the colon surviving `tr '/' '-'` -- must come back with a
#       colon IN the minted path and the stub NOT found. If the mutant mint loses its colon this leg
#       fails out loud, because that is the moment the leg above stops discriminating.
#
# The probe records and never asserts, as the #415 family does: the session stub inherits the probe's
# exit status, so an asserting probe would hand the pass a TICK-FAILED before any leg could read what
# it learned. Every verdict is made here, where a FAIL can print its evidence.
sp503_rec=$work/scratch-503.probe
cat >"$work/scratch-503-session.sh" <<SP503
#!/usr/bin/env bash
# Runs INSIDE the fixture session through the PROC_TICK_ALLOC arm, where FACTORY_SCRATCH is
# observable only from the dispatched environment. Records, never asserts -- see the header.
set -u
tree=\$1
rec='$sp503_rec'
mint=\${FACTORY_SCRATCH:-}
printf 'unit %s\n' "\${FACTORY_TICK_UNIT:-none}" >>"\$rec"
if [ -z "\$mint" ]; then
	printf 'env-empty %s\n' "\$tree" >>"\$rec"
	exit 0
fi
# The stub is what a tick actually puts in its scratch: an executable reached through PATH. Planting
# it is the premise of the question, so a scratch that cannot hold it is its own recorded verdict.
if ! mkdir -p "\$mint/bin" 2>/dev/null ||
	! printf '#!/bin/sh\necho sp503-stub\n' >"\$mint/bin/sp503-stub" 2>/dev/null; then
	printf 'no-stub %s\n' "\$mint" >>"\$rec"
	exit 0
fi
chmod +x "\$mint/bin/sp503-stub" 2>/dev/null
found=\$(PATH="\$mint/bin:\$PATH" sh -c 'command -v sp503-stub' 2>/dev/null)
printf 'mint %s\nfound %s\n' "\$mint" "\${found:-none}" >>"\$rec"
exit 0
SP503
chmod +x "$work/scratch-503-session.sh"

sp503_say() { printf '%s\n' "$*" | sed 's/^/            /' >&2; }

# sp503_field <key> -- the last value the probe recorded under <key>, empty if it never got there.
sp503_field() { sed -n "s/^$1 //p" "$sp503_rec" 2>/dev/null | tail -n 1; }

# sp503_probe_run <tag> -- one fixture pass with the recording session installed, records cleared.
sp503_probe_run() {
	: >"$sp503_rec"
	proc_tick_alloc=$work/scratch-503-session.sh
	proc_plant
	proc_run "$1"
	proc_tick_alloc=''
}

# ---- (a) the ground, measured rather than assumed. The colon dir is the ticket's own name; the
# colon-free twin is what the fix mints.
sp503_world() {
	local colon_dir=$work/'plock-unclaimed:448-Zzzzzz' plain_dir=$work/plock-unclaimed-448-Zzzzzz
	local found=''
	mkdir -p "$colon_dir/bin" "$plain_dir/bin" || {
		sp503_say 'precondition: the fixture cannot build its two probe directories.'
		return 1
	}
	printf '#!/bin/sh\necho sp503-stub\n' >"$colon_dir/bin/sp503-stub"
	printf '#!/bin/sh\necho sp503-stub\n' >"$plain_dir/bin/sp503-stub"
	chmod +x "$colon_dir/bin/sp503-stub" "$plain_dir/bin/sp503-stub"
	# The colon-free twin FIRST: if even it is invisible, this suite's own scratch root carries a
	# colon of its own or the stub is not executable, and the null variant below would then read as
	# the world rule holding when it only says the fixture is broken.
	found=$(PATH="$plain_dir/bin:$PATH" sh -c 'command -v sp503-stub' 2>/dev/null)
	[ "$found" = "$plain_dir/bin/sp503-stub" ] || {
		sp503_say "control: a PATH prepend through the COLON-FREE directory found"
		sp503_say "${found:-nothing}. This suite's scratch root ($work) is not a usable PATH"
		sp503_say 'entry -- it carries a colon, or the stub is not executable -- so the null variant'
		sp503_say 'below could pass on a broken fixture. This file proves nothing until that is true.'
		return 1
	}
	found=$(PATH="$colon_dir/bin:$PATH" sh -c 'command -v sp503-stub' 2>/dev/null)
	if [ -n "$found" ]; then
		sp503_say "control: a PATH prepend through a COLON-BEARING directory found $found."
		sp503_say 'This whole file asserts that such a prepend splits into two entries and finds'
		sp503_say 'nothing, because that is what #503 was filed on. On a shell where it finds the'
		sp503_say 'stub the world rule is false and the legs below would pass on nothing.'
		return 1
	fi
	return 0
}
# The reporter here is `claim_leg <label> <command...>`: the leg's own name is what the suite prints,
# so the sentence each leg owes the reader is the comment above it, never a third argument -- passing
# the description as an argument makes the reporter try to EXECUTE it, which is how these three legs
# first shipped: `line 9406: the colon-free prepend finds the stub...: command not found`, three FAILs
# and not one assertion of mine run.
claim_leg 'fixture: a colon in a PATH entry splits it' sp503_world

# ---- (b) the shipped dispatch: the minted name is PATH text.
sp503_shipped() {
	local bad='' mint='' found='' unit='' want='' start=''
	proc_new scr503-s
	sp503_probe_run scr503-s
	unit=$(sp503_field unit)
	start=$(grep -o 'TICK-START [^ ]*' "$proc_journal" 2>/dev/null | head -n 1)
	case "${start:-}" in
	*'TICK-START unclaimed:'*) ;;
	*) bad="the pass dispatched no TICK-START unclaimed:<n> row (${start:-nothing}), so the fixture never put a colon in a unit id: the shape #503 is filed on is absent and nothing below is about it" ;;
	esac
	mint=$(sp503_field mint)
	case "$mint" in
	'') bad="${bad:+$bad; }the session recorded no scratch (rec: $(tr '\n' ';' <"$sp503_rec" 2>/dev/null))" ;;
	*:*) bad="${bad:+$bad; }the minted scratch $mint carries a colon, so a PATH prepend through it splits in two and finds nothing (#503)" ;;
	esac
	# The name still has to identify the unit it was minted for -- that is why the colon was in it.
	want="plock-${unit//:/-}"
	[ -z "$mint" ] || case "$(basename -- "$mint")" in
	"$want"-*) ;;
	*) bad="${bad:+$bad; }the minted scratch $(basename -- "$mint") no longer echoes the unit id (expected the form $want-XXXXXX)" ;;
	esac
	[ -z "$mint" ] || case "$mint" in
	"$proc_trees"/*) bad="${bad:+$bad; }the scratch $mint is inside the tree, which is the #415 halt" ;;
	esac
	found=$(sp503_field found)
	[ "$found" != 'none' ] ||
		bad="${bad:+$bad; }a stub planted in FACTORY_SCRATCH/bin and prepended to PATH was NOT found ($mint) -- the failure #503 reports, from the shipped mint"
	[ -z "$mint" ] || [ "$found" = "$mint/bin/sp503-stub" ] ||
		bad="${bad:+$bad; }the prepend found ${found:-nothing}, not the stub at $mint/bin/sp503-stub"
	[ -z "$mint" ] || [ ! -e "$mint" ] ||
		bad="${bad:+$bad; }the scratch $mint still exists after the pass, so the teardown's prefix case stopped matching"
	grep -q 'SCRATCH-FAILED' "$proc_journal" 2>/dev/null &&
		bad="${bad:+$bad; }the pass journaled SCRATCH-FAILED while handing out a directory"
	[ -n "$bad" ] && sp503_say "$bad"
	[ -z "$bad" ]
}
claim_leg 'the shipped mint is PATH-safe and echoes the unit' sp503_shipped

# ---- (c) the control: the mint exactly as #503 was filed on. `factory_slug` maps `/` alone, so the
# colon of `unclaimed:900` reaches the name. This is the proof that (b) measures that colon and not
# its own fixture. The planted line is byte-identical to the line being replaced, quote and all, so
# the control mutates the mint and nothing else; the quote travels as its own segment because a
# single-quoted argument cannot hold one (the shape `proc_mutate_factory`'s own callers use).
sp503_control() {
	local bad='' mint='' found='' q="'"
	proc_new scr503-c
	proc_mutate_factory 'plock-$(factory_slug "${unit//:/-}")' \
		'	scratch=$(mktemp -d "${TMPDIR:-/tmp}/plock-$(factory_slug "$unit")-XXXXXX" 2>/dev/null) || scratch='"$q$q"' # SP503-MUTANT: factory_slug maps / alone, so the unit id'"'"'s colon reaches the name' || {
		sp503_say 'the mint line moved out of the shape #503 names, so the control has nowhere to'
		sp503_say 'plant and the shipped leg above has no control. Anchor:'
		sp503_say '  plock-$(factory_slug "${unit//:/-}")'
		return 1
	}
	sp503_probe_run scr503-c
	mint=$(sp503_field mint)
	case "$mint" in
	'') bad="the mutant pass recorded no scratch (rec: $(tr '\n' ';' <"$sp503_rec" 2>/dev/null)), so nothing below was observed" ;;
	*:*) ;;
	*) bad="the mutant mint $mint has NO colon: the control no longer restores what #503 was filed on, so a colon-bearing mint could ship green beside a green suite" ;;
	esac
	found=$(sp503_field found)
	[ "$found" = 'none' ] ||
		bad="${bad:+$bad; }with a colon in the minted path the prepend STILL found ${found:-nothing}, so this fixture cannot see the split #503 reports"
	[ -n "$bad" ] && sp503_say "$bad"
	[ -z "$bad" ]
}
claim_leg 'control: the filed mint splits PATH and hides the stub' \
	'the mutant name carries the colon and the prepended stub is not found' sp503_control
