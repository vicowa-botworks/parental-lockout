# shellcheck shell=bash
# shellcheck disable=SC2016,SC2034,SC2154
# 497-last-slice-declaration.sh -- a declared last slice owes the keyword touching its number (#497).
#
# #497 measured eleven finished issues sitting open in the `delivered` queue behind merged bodies
# that named their own ticket with `Refs` and nothing else -- the oldest 6d01h -- because on a
# multi-slice ticket every slice is genuinely partial when its body is written, `Refs` is the
# correct verb each time, and no artifact recorded which slice completed the umbrella. The repair
# is the author's statement: `Last slice of #N.` in the body, which `scripts/check-close-verb.py`
# now refuses when no documented keyword touches that number. The phrase is NOT a GitHub keyword
# and `CLOSER_RE` in `factory/factory.sh` did not widen -- the read side never grows an unmeasured
# closer (#1256, H-0164) -- so everything the statement does, this checker is the only thing that
# does it.
#
# What the legs pin, and the polarities:
#   (a) the fixture premise first: the phrase matches NEITHER shipped closer pattern, and the
#       checker imports and classifies it -- a leg set that rested on the phrase accidentally
#       firing a closer would prove nothing about the arm;
#   (b) the refusal: the bare declaration exits 1 and says why, and the refusal sentence itself
#       carries no closer vocabulary beside a number (#1256's rule for this tool's own stdout);
#   (c) the healthy final slice: declaration plus keyword touching the same number exits 0;
#   (d) the controls that keep the arm from becoming a tax on honest partial slices: an undeclared
#       `Refs` body exits 0 (the row stays honestly `delivered`), a declaration NEGATED BEFORE the
#       phrase exits 0, a declaration inside a code span exits 0 (quoting the form is the inert
#       place for it, the same shelter #353 measured), and a `--span-blind` pipe -- the
#       commit-message read -- exits 0 because a message is not the author's statement about a
#       pull request;
#   (e) the negation POSITION rule: a negation in a LATER clause leaves the statement standing
#       and the refusal fires;
#   (f) one number, one verdict: a declared number counts as naming the branch claim, so on
#       `--branch issue-777` the refusal that speaks is the declaration's, not the branch leg's.
lsd_work=$work/ls497
# Absolute at load, the lesson 457-branch-body-claims.sh records: every leg `cd`s somewhere, and
# a relative $HERE would then resolve against the wrong tree.
lsd_script=$(cd -- "$HERE/../scripts" && pwd)/check-close-verb.py
lsd_out=$lsd_work/last-out

lsd_say() { printf '            %s\n' "$1" >&2; }

# lsd_want <label> <want-rc> [args...] -- the checker with $lsd_work as cwd, answer captured.
# Every leg names `--branch` explicitly, because the suite runs inside a real checkout whose
# branch would otherwise let the #457 leg speak beside the one under test.
lsd_want() {
	local label=$1 want=$2
	shift 2
	( cd -- "$lsd_work" && python3 "$lsd_script" "$@" ) >"$lsd_out" 2>&1
	local got=$?
	if [ "$got" != "$want" ]; then
		lsd_say "$label: exited $got, wanted $want -- the answer was:"
		sed 's/^/              /' "$lsd_out" >&2
		return 1
	fi
	return 0
}

lsd_fixture() {
	mkdir -p -- "$lsd_work" || return 1
	printf 'Delivers the last piece.\n\nLast slice of #777.\n' >"$lsd_work/bare.md" || return 1
	printf 'Delivers the ticket.\n\nLast slice of #777.\n\nCloses #777 in running text.\n' \
		>"$lsd_work/good.md" || return 1
	printf 'Refs #777; a partial slice.\n' >"$lsd_work/partial.md" || return 1
	printf 'This is not the last slice of #777; the umbrella stays open.\nRefs #777.\n' \
		>"$lsd_work/neg.md" || return 1
	printf 'The skill teaches `Last slice of #777` as the form.\nRefs #777.\n' \
		>"$lsd_work/quoted.md" || return 1
	printf 'Last slice of #777; the umbrella #778 is not finished.\n' >"$lsd_work/later.md" \
		|| return 1
	# The premise the whole file rests on, read from the shipped module rather than a re-typed
	# pattern: the declaration phrase fires no closer GitHub or the harness knows, and the checker
	# classifies the bytes it is handed. If the phrase ever starts matching CLOSER, this leg goes
	# red before any verdict below counts as evidence about the arm.
	# (The heredoc carries no `||` on its introducing line: the body starts after that physical
	# line ends, so a compound-command tail would swallow the python source -- measured here when
	# the file first ran.)
	if ! python3 - "$lsd_script" <<'PY' 2>"$lsd_work/premise-err"
import importlib.util
import sys

spec = importlib.util.spec_from_file_location("ccv", sys.argv[1])
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
fired = [s for s in ("Last slice of #777.", "last slice of: #777") if m.CLOSER.search(s)]
if fired:
    print("CLOSER matched the declaration phrase: %r" % fired, file=sys.stderr)
    sys.exit(1)
if m.CLOSER.findall("Closes #777") != ["777"]:
    print("CLOSER stopped matching its own keyword", file=sys.stderr)
    sys.exit(1)
got = m.classify("Last slice of #777.\n")
if "777" not in got["declared"] or "777" in got["firing"]:
    print("classify did not report a bare declaration: %r" % (got,), file=sys.stderr)
    sys.exit(1)
PY
	then
		lsd_say 'the shipped patterns do not have the shape this file assumes:'
		sed 's/^/              /' "$lsd_work/premise-err" >&2
		return 1
	fi
}
claim_leg 'fixture: the phrase fires no closer and the checker classifies it' lsd_fixture

# (b) The refusal, and #1256's rule about its wording.
lsd_bare() {
	lsd_want 'bare declaration' 1 --branch '' "$lsd_work/bare.md" || return 1
	grep -qF 'REFUSED, declared last slice' "$lsd_out" || {
		lsd_say 'the refusal fired but never named what it refused:'
		sed 's/^/              /' "$lsd_out" >&2
		return 1
	}
	! grep -qE 'closes #|CLOSES DESPITE' "$lsd_out" || {
		lsd_say 'the refusal itself carries closer vocabulary beside a number (#1256 rule):'
		sed 's/^/              /' "$lsd_out" >&2
		return 1
	}
}
claim_leg 'a declared last slice with no keyword is refused, in words' lsd_bare

# (c) The healthy final slice: the statement AND the keyword touching the same number.
lsd_good() {
	lsd_want 'declaration with its keyword' 0 --branch '' "$lsd_work/good.md" || return 1
	grep -qF 'closes #777' "$lsd_out"
}
claim_leg 'the declaration passes when its keyword is there' lsd_good

# (d) The arm is not a tax on honest partial work: an undeclared row stays honestly delivered.
lsd_partial() {
	lsd_want 'undeclared Refs body' 0 --branch '' "$lsd_work/partial.md"
}
claim_leg 'an undeclared partial slice passes untouched' lsd_partial

# (e) The negation POSITION rule, both directions.
lsd_negation() {
	lsd_want 'negation before the phrase' 0 --branch '' "$lsd_work/neg.md" || return 1
	lsd_want 'negation in a later clause' 1 --branch '' "$lsd_work/later.md" || return 1
	grep -qF 'REFUSED, declared last slice' "$lsd_out"
}
claim_leg 'a negation before the phrase flips it; one later does not' lsd_negation

# (e, cont.) A quoted declaration quotes a form; the span shelter #353 measured covers it because
# the same strip order decides both, and one shelter decided two ways would be two bugs.
lsd_span() {
	lsd_want 'declaration inside a code span' 0 --branch '' "$lsd_work/quoted.md"
}
claim_leg 'a quoted declaration states nothing' lsd_span

# (d, cont.) The third control: the commit-message read stays declaration-silent -- the phrase is
# the author's statement about a pull request, and a message is not one.
lsd_blind() {
	(cd -- "$lsd_work" && printf 'Last slice of #777.\n' |
		python3 "$lsd_script" --span-blind --branch '' -) >"$lsd_out" 2>&1 || {
		lsd_say 'the span-blind pipe read turned red -- the report must not read the leg there:'
		sed 's/^/              /' "$lsd_out" >&2
		return 1
	}
	! grep -qF 'REFUSED' "$lsd_out" || {
		lsd_say 'the declaration leg spoke on a span-blind read:'
		sed 's/^/              /' "$lsd_out" >&2
		return 1
	}
}
claim_leg 'the span-blind read stays declaration-silent' lsd_blind

# (f) One number, one verdict: the declaration names the branch claim (#457's ledger), so on a
# branch carrying the same number the sharper diagnosis is the one that speaks.
lsd_branch() {
	lsd_want 'declaration naming the branch claim' 1 --branch issue-777 "$lsd_work/bare.md" \
		|| return 1
	grep -qF 'REFUSED, declared last slice' "$lsd_out" || {
		lsd_say 'the declaration did not speak for its own branch number:'
		sed 's/^/              /' "$lsd_out" >&2
		return 1
	}
	! grep -qF 'REFUSED, branch' "$lsd_out" || {
		lsd_say 'both legs issued a verdict for one number -- a doubled verdict reads as two claims:'
		sed 's/^/              /' "$lsd_out" >&2
		return 1
	}
}
claim_leg 'the declaration names the branch claim and owns its verdict' lsd_branch
