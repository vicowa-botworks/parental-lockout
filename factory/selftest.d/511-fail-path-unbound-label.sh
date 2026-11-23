# shellcheck shell=bash
# shellcheck disable=SC2016,SC2034,SC2154
# 511-fail-path-unbound-label.sh -- a FAIL path must be able to SPEAK: every suite leg that
# expands the reporter idiom "$label" on its FAIL branch must assign it in its own body,
# because `set -u` is on for the whole run and a name declared `local` and never assigned is
# UNSET — it even shadows a global of the same name with nothing.
#
# The defect (#511, live in `notice_legs` since that leg was written): the function opened
# `local label unit=unclaimed:247 tree=$repo n=0 body=""` and never assigned label. Every
# FAIL branch of the function printf's "$label" first. Reproduced in this tick against the
# branch tip 23f51c6 in a throwaway clone, with the first FAIL branch forced by planting
# `!= 2` in the copy (a one-line mutation, nothing else moved):
#
#   factory/selftest.sh: line 3303: label: unbound variable
#   factory/selftest.sh: RUN INCOMPLETE -- died with exit 1 after 173 ok and 1 FAIL, before any verdict
#
# The `1 FAIL` on the second line is the whole indictment: a FAIL branch increments the
# counter BEFORE printing, so the run counted a failure it never reported — no FAIL line, no
# sentence naming the tree or the posting, and the suite died ~27k lines short of its verdict,
# every leg after notice_legs silently unrun. A FAIL that cannot print is worse than the bug
# it was written to catch: the bug gets a green-looking corpse.
#
# The linter does not catch the class (measured 22 Nov on 0.10.0: a file containing
# `f(){ local label; printf '%s\n' "$label"; }` is reported clean apart from SC2148), so the
# pin lives here, in the suite, next to the idiom.
#
# What the legs pin, and the polarities:
#   (a) the shipped bytes: no top-level function in `factory/selftest.sh` or in any leg file
#       under `factory/selftest.d/` expands "$label" in a printf while its own body contains
#       no `label=` assignment;
#   (b) the control: a copy of the shipped suite with the assignment the fix shipped taken
#       back out — the anchor counted to exactly one before the cut, the cut re-read after —
#       and the scanner must come back RED naming notice_legs. (a) without (b) is a scanner
#       that could be green on blindness; (b) without asserting the cut applied is a control
#       that could be red on a plant that never happened.
#
# What this scanner does NOT see, stated so nobody trusts it past its reach: any variable
# other than `label` (the FAIL-printf reporter idiom is name-specific and every other name
# needs its own judgement; `label` alone is also what the 67 `local label=` rows in the body
# of factory/selftest.sh make the convention, and the scanner flags exactly zero of them);
# locals filled by `read` rather than by `=` (legitimate, e.g. the loop var in
# run_selftest_d_legs); a function body a column-0 `}` inside a heredoc closes early — that
# would hide a defect, not invent one, and (b) proves the state machine reads notice_legs
# whole; and indirection (printf -v, eval, declare). This file IS inside the scanned set:
# its patterns spell the reference as awk regexes (`"\$label"`, `\$\{label`) whose raw bytes
# are not the literal substring the scanner searches, so it cannot flag its own patterns —
# while any future edit that puts a bare "$label" printf argument into a function here
# without a `label=` in that body is correctly flagged by leg (a).

# s511_scan <file> -- print every top-level function that prints "$label" with no `label=`
# in its body, as `file:line: function <name> ...`. A body is read from a column-0
# `name()` row to the next column-0 `}` row; indented definitions and closes (nested helpers,
# heredoc bodies) stay inside the outer body, which is what the outer body's FAIL branches
# expand.
s511_scan() {
	awk -v FNAME="$1" '
		/^[A-Za-z_][A-Za-z0-9_]*\(\)/ {
			if (infn) next            # nested or heredoc look-alike: the outer body owns it
			infn = 1; name = $1; sub(/\(\)/, "", name); start = NR
			hasref = 0; hasass = 0
			next
		}
		infn {
			if ($0 ~ /printf/ && index($0, "\"$label\"") > 0) hasref = 1
			if ($0 ~ /[[:space:]]label=/)                     hasass = 1
			if ($0 ~ /^\}/) {
				if (hasref && !hasass)
					printf "%s:%d: function %s expands the reporter \"$label\" with no label= in its body: the FAIL branch dies on set -u instead of reporting\n", FNAME, start, name
				infn = 0
			}
		}
	' "$1"
}

# s511_files -- the suite's shipped shell: the body and every leg file, byte order.
s511_files() {
	printf '%s\n' "$HERE/selftest.sh"
	[ -d "$HERE/selftest.d" ] && find "$HERE/selftest.d" -maxdepth 1 -type f -name '*.sh' | LC_ALL=C sort
	return 0
}

s511_shipped_clean() {
	local f out bad=0 n=0
	# A leg that cannot read the subject must not go green on the absence (#503's
	# vacuous-plant lesson, applied to the scan side): the body is the one file that
	# must exist for this leg to mean anything.
	[ -f "$HERE/selftest.sh" ] || {
		printf '            precondition: %s is not a readable file, so the scan below\n' "$HERE/selftest.sh" >&2
		printf '            proves nothing about the suite.\n' >&2
		return 1
	}
	while IFS= read -r f; do
		n=$((n + 1))
		[ -f "$f" ] || continue
		out=$(s511_scan "$f")
		if [ -n "$out" ]; then
			bad=$((bad + 1))
			printf '%s\n' "$out" | sed 's/^/            /' >&2
		fi
	done < <(s511_files)
	[ "$n" -gt 1 ] || {
		printf '            precondition: only %s file was offered to the scanner; the\n' "$n" >&2
		printf '            factory/selftest.d leg files are the other half of the scanned set.\n' >&2
		return 1
	}
	[ "$bad" = 0 ] || {
		printf '            %s file(s) carry a FAIL printf that dies on an unset local; a FAIL\n' "$bad" >&2
		printf '            that cannot print stops the suite before the verdict (#511).\n' >&2
		return 1
	}
	return 0
}
claim_leg 'a FAIL printf never expands an unset label' s511_shipped_clean

s511_control() {
	local cp=$work/511-mutant-selftest.sh out bad
	cp "$HERE/selftest.sh" "$cp" || {
		printf '            precondition: the fixture cannot copy the suite.\n' >&2
		return 1
	}
	# The anchor is the fix itself. Counted to exactly one BEFORE the cut so a rewrite of
	# notice_legs reddens this leg with a named cause instead of a control that silently
	# plants nothing — the vacuous-mutant failure the #503 family names. The two counts
	# that are not one are different defects and say different things: zero rows means
	# the declaration the fix shipped is not in the tree (this unit's fix was dropped or
	# never landed), two or more means notice_legs was rewritten around a copy of it.
	bad=$(grep -cF 'local label="a halt comments on its unit" unit=' "$cp" || true)
	if [ "$bad" != 1 ]; then
		if [ "$bad" = 0 ]; then
			printf '            factory/selftest.sh carries NO `local label="a halt comments on its\n' >&2
			printf '            unit" unit=` row: the fix this control exists to falsify is not in the\n' >&2
			printf '            tree, so leg (a) above cannot be trusted and the mutant cannot be\n' >&2
			printf '            planted. Land the notice_legs declaration first.\n' >&2
		else
			printf '            the notice_legs declaration row `local label="a halt comments on its\n' >&2
			printf '            unit" unit=` is not exactly one (counted %s), so the control has no\n' "$bad" >&2
			printf '            single place to plant and leg (a) has no clean falsifier. Re-derive\n' >&2
			printf '            this control against the current declaration; do not delete it.\n' >&2
		fi
		return 1
	fi
	sed -i 's/local label="a halt comments on its unit" unit=/local label unit=/' "$cp"
	grep -qF 'local label unit=unclaimed:247' "$cp" || {
		printf '            the cut did not apply: the mutant does not carry the filed\n' >&2
		printf '            declaration, so a RED verdict below would not be the defect.\n' >&2
		return 1
	}
	out=$(s511_scan "$cp")
	if [ -z "$out" ]; then
		printf '            the scanner called the MUTANT clean: it cannot see the defect it\n' >&2
		printf '            exists for, so the green verdict above is blindness, not bytes.\n' >&2
		return 1
	fi
	printf '%s\n' "$out" | sed 's/^/            /' >&2
	case "$out" in
	*notice_legs*) : ;;
	*)
		printf '            the mutant came back red, but not for notice_legs — the planted\n' >&2
		printf '            defect went unseen and whatever this red is, it is not the control.\n' >&2
		return 1 ;;
	esac
	return 0
}
claim_leg 'control: the filed notice_legs scans RED' s511_control
