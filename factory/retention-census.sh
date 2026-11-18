#!/usr/bin/env bash
# shellcheck shell=bash disable=SC2016
# retention-census.sh -- price a proposed bound on factory/log, and refuse to price one that is
# not specified far enough to enact.
#
# Issue #334 asks a person how much of the harness's own audit trail to keep, and `H-0188` is the
# record of why a tick does not answer that for them: deleting the operator's records is one of the
# four classes a human keeps. What a tick CAN do is make the answer's cost legible before it is
# given, and until now the only instrument for that was a block of prose in `docs/factory.md` --
# clause 5 of the journal's retention contract. Prose is the wrong place for this job, for a reason
# the clause itself names: "the block's worst answer is a quiet one". `factory/log` is not tracked
# by git, so a fresh clone, a sibling worktree, or a mistyped path all answer "nothing would be
# removed" about a directory they have never written, and that answer is the same colour as the true
# one. Prose can print `total` and advise a reader to look at it. A tool can refuse: exit 3 here,
# with the reason, whenever nothing matched the transcript name.
#
# What it is NOT. It chooses no number, no clock, no zone and no generation count, and it destroys
# nothing -- this file contains no removal, truncation or move of a census target, and the only
# thing it writes is its own temporary snapshot. Pricing a proposed answer is not enacting it; the
# enacted bound is the half that stays escalated.
#
# One clock, one cut, six readings. Every predicate is evaluated against ONE cut taken from ONE
# clock reading (`--now` pins it, which is also the seam the fixture drives), in the shape `H-0169`
# established for the ranking pass: a pass that reads the clock per row invents its own boundary,
# and a census whose six numbers came from six instants would measure the schedule rather than the
# policy. The six still disagree -- that disagreement IS the output, and averaging it away would be
# the same error as defaulting the zone.
#
# The six spellings are clause 5's, and each is what an implementer actually types:
#
#   A filename-seconds   the name stamp against the cut stamp. UTC by construction -- the name comes
#                        from `date -u` at the tick's first line, so no activation's zone touches it.
#   B mtime-seconds      `find ! -newermt` with the stamp pinned to UTC.
#   C day-string <=      the YYYYMMDD prefix against the cut DAY, taking that day whole.
#   D day-string <       the same comparison, refusing that day.
#   E find -mtime +N     GNU's whole-24-hour truncation. THIS IS NOT AN N-DAY PREDICATE: it
#                        truncates the age to whole days and asks for MORE than N, so `+7` means
#                        "older than eight days". It is the spelling that reads zero while the
#                        seven-day set is non-empty.
#   F -newermt, no TZ    the cut stamped bare, so `find` parses it in whatever zone the activation
#                        arrived with. The installed unit carries no `TZ=`, so today that is the
#                        box's zone, not a chosen one.
#
# A, C, D and E are computed from a snapshot taken once; B and F are the shipped `find` itself,
# deliberately, because F exists to report a quirk of that tool and re-implementing it in awk would
# report this script's model of GNU rather than the thing an implementer will run.
#
# E is the one spelling that CANNOT be handed the census clock: `-mtime` has no reference-time option,
# it subtracts from whatever `find` reads at the moment it is called. So E is this script's arithmetic
# of GNU's truncation, evaluated at the clock the census was given, and the pinning is not left to
# assertion alone -- `--self-test` plants a fixture at ages measured from the REAL clock and requires
# the shipped `find -mtime +7` and the emulation to name the same files, which is what says the
# emulation is GNU's and not this file's invention. When `--now` is pinned away from the real clock
# the output says so, because a census that quietly evaluated one of its six rows at a different
# instant from the other five would be reporting the schedule instead of the policy -- the exact
# defect `H-0169` was raised for.
#
# The rate line is the half a census cannot see. Take the cut 14 days back into a directory whose
# oldest entry is 7 days old and all six predicates answer zero -- truthfully. That zero is not the
# cost of the policy; it is the age of the directory. The steady-state bound is the measured rate
# times N, and `H-0187`'s footprint line plus this line are the two numbers that answer the question
# the ticket is really asking: how big does this get.
set -euo pipefail

_here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=./common.sh
# shellcheck disable=SC1091
. "$_here/common.sh"

# The transcript name, the one home for the pattern: `<UTC stamp>.<unit>.log`, the file factory.sh
# writes per activation beside the journal. `supervisor.log` does not match it, which is the point:
# the journal has its own rotation and is only ever reported here, never censused.
RC_GLOB='2*T*Z.*.log'

# The usage block quotes `$FACTORY_LOG` on purpose: it names the default, it does not read it, which
# is the same reason the awk programs in this file are single-quoted -- hence the file-level SC2016
# directive at the top, in the shape `factory/triage.sh:2` uses.
rc_usage() {
	printf '%s\n' \
		'usage: factory/retention-census.sh --days N  [--dir DIR] [--now EPOCH]' \
		'       factory/retention-census.sh --bytes N [--dir DIR] [--now EPOCH]' \
		'       factory/retention-census.sh --self-test' \
		'' \
		'Prices a proposed bound on the harness log directory. Writes nothing to it.' \
		'  --days N     what "keep N days" removes, under each of the six clocks clause 5 lists' \
		'  --bytes N    what a byte cap of N evicts oldest-first, under both clocks' \
		'  --dir DIR    the directory to census (default: $FACTORY_LOG)' \
		'  --now EPOCH  take the clock here instead of now, so a cut can be re-taken exactly' \
		'' \
		'exit 2  a missing number, two numbers, a non-positive integer, or a directory that is not' \
		'        a directory -- it refuses rather than defaulting, which is what clause 5 is' \
		'exit 3  nothing in the directory matches the transcript name: a census of nothing cannot be' \
		'        told from an empty policy, so it is a refusal and not a verdict'
}

rc_refuse() { # <exit code> <sentence>
	local code=$1
	shift
	printf 'retention-census.sh: %s\n' "$*" >&2
	exit "$code"
}

rc_is_natural() { # <value> -> 0 for a positive whole number, 1 for anything else
	case "${1:-}" in
	'' | 0 | *[!0-9]*) return 1 ;;
	*) return 0 ;;
	esac
}

rc_snapshot() { # <dir> <file> -> "name<TAB>size<TAB>mtime-epoch" per transcript, taken once
	find "$1" -maxdepth 1 -type f -name "$RC_GLOB" -printf '%f\t%s\t%T@\n' 2>/dev/null |
		awk -F '\t' 'BEGIN { OFS = "\t" } { sub(/\..*$/, "", $3); print }' >"$2"
}

rc_name_cut() { # <snapshot> <lt|le> <stamp, 8 or 15 chars> -> "<count>\t<bytes>"
	awk -v c="$3" -v cmp="$2" '
		BEGIN { FS = "\t"; n = 0; b = 0 }
		{
			k = (length(c) > 8) ? substr($1, 1, 15) : substr($1, 1, 8)
			if ((cmp == "le" && k <= c) || (cmp == "lt" && k < c)) { n++; b += $2 }
		}
		END { printf "%d\t%d\n", n, b }
	' "$1"
}

rc_find_cut() { # <dir> <TZ or empty> <find args...> -> "<count>\t<bytes>"
	local dir=$1 tz=$2
	shift 2
	if [ -n "$tz" ]; then
		TZ="$tz" find "$dir" -maxdepth 1 -type f -name "$RC_GLOB" "$@" -printf '%s\n' 2>/dev/null
	else
		find "$dir" -maxdepth 1 -type f -name "$RC_GLOB" "$@" -printf '%s\n' 2>/dev/null
	fi | awk '{ n++; b += $1 } END { printf "%d\t%d\n", n + 0, b + 0 }'
}

# Evict oldest-first until the directory fits the cap. The clock is an argument, not a decision:
# clause 5's "a byte cap sorted by the wrong clock" is the reason this takes two and reports both.
rc_evict() { # <snapshot> <cap> <mtime|name> <kept-names out> -> "<kept_n>\t<kept_b>\t<evict_n>\t<evict_b>"
	local key
	case "$3" in
	(name) key='1,1' ;;
	(mtime) key='3,3n' ;;
	(*)
		printf 'retention-census.sh: internal: no eviction clock %s\n' "$3" >&2
		exit 2
		;;
	esac
	sort -t "$(printf '\t')" -k "$key" -- "$1" |
		awk -v cap="$2" -v out="$4" '
			BEGIN { FS = "\t" }
			{ name[NR] = $1; size[NR] = $2; total += $2 }
			END {
				for (i = NR; i >= 1; i--) {
					if (kept + size[i] > cap) break
					kept += size[i]
					print name[i] > out
					kn++
				}
				close(out)
				printf "%d\t%d\t%d\t%d\n", kn + 0, kept + 0, NR - kn, total - kept
			}
		'
}

# GNU's `-mtime +N` arithmetic, evaluated at a clock handed to it. `-mtime` compares against the
# moment `find` runs and offers no reference time, so the shipped tool cannot be asked what a pinned
# cut would have removed -- and a census that called it anyway would answer a different question from
# the five rows printed beside it. `+N` is `floor(age/86400) > N`, which is why eight days is what a
# "seven day" policy of this spelling actually asks for.
rc_mtime_emu() { # <snapshot> <days> <now epoch> -> "<count>\t<bytes>"
	awk -F '\t' -v n="$2" -v now="$3" '
		BEGIN { c = 0; b = 0 }
		{ a = (now - $3) / 86400; if (int(a) > n) { c++; b += $2 } }
		END { printf "%d\t%d\n", c, b }
	' "$1"
}

rc_row() { # <letter> <count> <bytes> <what the spelling actually compares>
	printf '  %s %6s files %12s B  %s\n' "$1" "$2" "$3" "$4"
}

# ----------------------------------------------------------------------------- the census
rc_main() {
	local want='' days='' cap='' dir=${FACTORY_LOG:-} now=''
	while [ $# -gt 0 ]; do
		case "$1" in
		(--days | --bytes)
			[ $# -ge 2 ] || rc_refuse 2 "$1 needs its number. Neither is defaulted: a census that invented one would be the defaulted clock clause 5 exists to refuse"
			[ -z "$want" ] || rc_refuse 2 "$1 given alongside --$want: a census prices ONE proposed bound at a time, because the two answers are two different policies"
			want=${1#--}
			if [ "$want" = days ]; then days=$2; else cap=$2; fi
			shift 2
			;;
		(--dir)
			[ $# -ge 2 ] || rc_refuse 2 "--dir needs a path"
			dir=$2
			shift 2
			;;
		(--now)
			[ $# -ge 2 ] || rc_refuse 2 "--now needs an epoch in whole seconds"
			now=$2
			shift 2
			;;
		(-h | --help)
			rc_usage
			return 0
			;;
		(*)
			rc_refuse 2 "unrecognised argument '$1'. --help lists the three shapes"
			;;
		esac
	done

	case "$want" in
	(days) rc_is_natural "$days" || rc_refuse 2 "--days wants a positive whole number of days, not '$days'" ;;
	(bytes) rc_is_natural "$cap" || rc_refuse 2 "--bytes wants a positive whole number of bytes, not '$cap'" ;;
	('') rc_refuse 2 "no number to price: pass --days N or --bytes N. See --help, and clause 5 of the journal's retention contract in docs/factory.md for why there is no default" ;;
	esac
	[ -n "$dir" ] || rc_refuse 2 "no directory: \$FACTORY_LOG is empty and no --dir was given"
	[ -d "$dir" ] || rc_refuse 2 "'$dir' is not a directory, and this census will not report a bound for a path it cannot open"
	if [ -n "$now" ]; then
		rc_is_natural "$now" || rc_refuse 2 "--now wants an epoch in whole seconds, not '$now'"
	else
		now=$(date -u +%s)
	fi

	RC_TMP=$(mktemp -d "${TMPDIR:-/tmp}/retention-census.XXXXXX") || rc_refuse 2 "could not create a snapshot directory"
	trap 'rm -rf -- "$RC_TMP"' EXIT HUP INT TERM
	RC_SNAP="$RC_TMP/snapshot"
	rc_snapshot "$dir" "$RC_SNAP"

	local total_n total_b jbytes=0
	total_n=$(wc -l <"$RC_SNAP" | tr -d '[:space:]')
	if [ "$total_n" = 0 ]; then
		rc_refuse 3 "PRECONDITION-FAILED no file under '$dir' matches '$RC_GLOB'. Six zeroes here would read exactly like a clean policy, and this directory may simply not be the one holding the transcripts -- factory/log is untracked, so a fresh clone, a sibling worktree and a wrong --dir all answer nothing. Nothing was priced."
	fi
	total_b=$(awk -F '\t' '{ s += $2 } END { printf "%d", s + 0 }' "$RC_SNAP")
	[ -f "$dir/supervisor.log" ] && jbytes=$(stat -c '%s' -- "$dir/supervisor.log" 2>/dev/null || printf '0\n')

	printf 'census:  %s\n' "$dir"
	printf '         %s transcripts, %s B   journal %s B\n' "$total_n" "$total_b" "$jbytes"
	printf 'clock:   now %s   a bare -newermt stamp here parses in %s\n' \
		"$(date -u -d "@$now" +%Y-%m-%dT%H:%M:%SZ)" "$(date +%z)"

	# The measured rate: these bytes over this directory's own span. It prices the cut the census
	# cannot see, so it is printed for both verbs.
	local oldest span rate_day=''
	oldest=$(awk -F '\t' 'NR == 1 || $3 < m { m = $3 } END { printf "%d", m + 0 }' "$RC_SNAP")
	span=$(awk -v n="$now" -v o="$oldest" 'BEGIN { printf "%.2f", (n - o) / 86400 }')
	if [ "$(awk -v s="$span" 'BEGIN { print (s >= 1) ? 1 : 0 }')" = 1 ]; then
		rate_day=$(awk -v b="$total_b" -v s="$span" 'BEGIN { printf "%d", b / s }')
		printf 'rate:    %s B/day of transcripts, over the %s days this directory holds\n' "$rate_day" "$span"
	else
		printf 'rate:    n/a -- this directory holds %s days of history, too short to measure one\n' "$span"
	fi

	if [ "$want" = days ]; then
		rc_days "$days" "$now" "$dir" "$rate_day"
	else
		rc_bytes "$cap" "$dir" "$rate_day"
	fi

	printf '\nrefuse:  this priced a number. It chose no clock, no zone and no generations kept,\n'
	printf '         and it destroyed nothing. An answer to #334 owes all three: clause 5 of the\n'
	printf "         journal's retention contract refuses a policy that names only the number.\n"
}

rc_days() { # <days> <now> <dir> <rate bytes/day or empty>
	local days=$1 now=$2 dir=$3 rate_day=$4
	local cut cutstamp cutday cutnaive a b c d e f
	cut=$((now - days * 86400))
	cutstamp=$(date -u -d "@$cut" +%Y%m%dT%H%M%SZ)
	cutday=${cutstamp%%T*}
	cutnaive=$(date -u -d "@$cut" +%Y-%m-%dT%H:%M:%S)
	printf 'cut:     %s   keep %s days: remove everything older than this instant\n\n' "$cutstamp" "$days"

	a=$(rc_name_cut "$RC_SNAP" lt "$cutstamp")
	b=$(rc_find_cut "$dir" UTC ! -newermt "$cutnaive")
	c=$(rc_name_cut "$RC_SNAP" le "$cutday")
	d=$(rc_name_cut "$RC_SNAP" lt "$cutday")
	e=$(rc_mtime_emu "$RC_SNAP" "$days" "$now")
	f=$(rc_find_cut "$dir" '' ! -newermt "$cutnaive")

	rc_row A "${a%%	*}" "${a##*	}" 'filename-seconds: the name stamp, UTC by construction'
	rc_row B "${b%%	*}" "${b##*	}" 'mtime-seconds: the stamp pinned to UTC'
	rc_row C "${c%%	*}" "${c##*	}" 'day-string <=: the cut DAY, taking that day whole'
	rc_row D "${d%%	*}" "${d##*	}" 'day-string <: the cut DAY, refusing that day'
	rc_row E "${e%%	*}" "${e##*	}" "find -mtime +$days by GNU's arithmetic: it truncates the age, so this asks for $((days + 1)) days"
	rc_row F "${f%%	*}" "${f##*	}" "-newermt, no TZ: the same stamp unqualified, parsed in $(date +%z)"

	local spread
	spread=$(printf '%s %s %s %s %s %s\n' "${a%%	*}" "${b%%	*}" "${c%%	*}" "${d%%	*}" "${e%%	*}" "${f%%	*}" |
		awk '{ for (i = 1; i <= 6; i++) if (!seen[$i]++) n++ } END { printf "%d", n }')
	printf '\nspread:  one policy, six spellings, %s distinct answers. The count alone is not the policy: it\n' "$spread"
	printf '         has to name the clock and the zone beside the number, or nobody can enact it.\n'

	# `-mtime` reads the moment `find` runs and offers no reference time, so row E is arithmetic
	# rather than a call. Say so when the census clock is not the real one, because the alternative is
	# six rows that look like six measurements and one of them is not.
	local real
	real=$(date -u +%s)
	if [ "$(awk -v a="$now" -v b="$real" 'BEGIN { print ((a - b) < -60 || (a - b) > 60) ? 1 : 0 }')" = 1 ]; then
		printf 'pinned:  E is GNU arithmetic evaluated at the census clock, not a call to find: -mtime reads\n'
		printf '         the real clock and takes no reference time. This cut was taken %ss from the real\n' \
			"$(awk -v a="$now" -v b="$real" 'BEGIN { d = a - b; print (d < 0) ? -d : d }')"
		printf '         clock, so the other five rows and this one are not the same instant for find.\n'
		printf '         self-test pins the arithmetic against the shipped tool on a live-clock fixture.\n'
	fi

	if [ "${a%%	*}" = 0 ] && [ "${b%%	*}" = 0 ] && [ "${c%%	*}" = 0 ] && [ "${f%%	*}" = 0 ]; then
		printf '\nquiet:   every reading is zero. That is the directory being younger than the policy, not the\n'
		printf '         policy being cheap: a cut older than the oldest entry always reads zero today.\n'
	fi
	if [ "${e%%	*}" != "${a%%	*}" ]; then
		printf 'fault:   -mtime +%s names %s files where the name-stamp reading names %s. That is not a\n' \
			"$days" "${e%%	*}" "${a%%	*}"
		printf '         cheaper policy, it is GNU truncating the age: -mtime is not a predicate for %s days.\n' "$days"
	fi
	if [ "${b%%	*}" != "${f%%	*}" ]; then
		printf 'zone:    one stamp, %s files read in UTC and %s read in %s. The installed unit exports no\n' \
			"${b%%	*}" "${f%%	*}" "$(date +%z)"
		printf '         TZ=, so today the second number is the one an implementation would pay for.\n'
	fi
	if [ -n "$rate_day" ]; then
		printf 'bound:   %s days at the measured rate is %s B of transcripts, whatever this cut can see.\n' \
			"$days" "$((rate_day * days))"
		printf '         The journal is separate, and clause 5 keeps it unbounded by a transcript rule.\n'
	fi
}

rc_bytes() { # <cap> <dir> <rate bytes/day or empty>
	local cap=$1 dir=$2 rate_day=$3
	local m n kept_m kept_b evict_m evict_b kept_n kept_nb evict_n evict_nb differ
	: >"$RC_TMP/kept-mtime"
	: >"$RC_TMP/kept-name"
	m=$(rc_evict "$RC_SNAP" "$cap" mtime "$RC_TMP/kept-mtime")
	n=$(rc_evict "$RC_SNAP" "$cap" name "$RC_TMP/kept-name")
	IFS=$'\t' read -r kept_m kept_b evict_m evict_b <<<"$m"
	IFS=$'\t' read -r kept_n kept_nb evict_n evict_nb <<<"$n"
	differ=$(sort "$RC_TMP/kept-mtime" "$RC_TMP/kept-name" | uniq -u | wc -l | tr -d '[:space:]')

	printf 'cap:     %s B, evicting oldest-first until the directory fits\n\n' "$cap"
	printf '  mtime clock  keeps %s files %s B   evicts %s files %s B\n' "$kept_m" "$kept_b" "$evict_m" "$evict_b"
	printf '  name  clock  keeps %s files %s B   evicts %s files %s B\n' "$kept_n" "$kept_nb" "$evict_n" "$evict_nb"
	printf '\ndiffer:  %s transcripts are kept by one clock and evicted by the other. A byte cap with no\n' "$differ"
	printf '         clock named does not decide which of them the operator loses.\n'
	if [ -n "$rate_day" ]; then
		if [ "$kept_b" != 0 ]; then
			printf 'bound:   the kept set is %s B, about %s days at the measured rate.\n' \
				"$kept_b" "$(awk -v k="$kept_b" -v r="$rate_day" 'BEGIN { printf "%.1f", k / r }')"
		else
			printf 'bound:   the cap holds no transcript at all: the newest single file is bigger than it.\n'
		fi
	fi
}

# -------------------------------------------------------------------------- the Makefile pin
#
# The arithmetic above is worth nothing if nothing runs it, and `make verify` is the only thing in this
# repository that runs a `--self-test` on its own. So the census pins its own invocation, reading the
# `verify:` recipe as command CHUNKS the way scripts/check-next-md.sh's pin does: a filename substring
# is satisfied by a comment, by a commented-out step, and by an `echo` that names this file while
# running something else. #391 is the reason the chunk has to be a real invocation of this file --
# delete the step, and `make verify` keeps going as though the check still happened.
#
# This is the half `H-0248`'s clause 5 cannot enforce: a contract in prose says what an answer owes,
# and only a refused build says whether anyone computed it.
RC_MAKEFILE_AWK='
/^verify:/ { inr = 1; next }
inr && /^[^ \t]/ { inr = 0 }
!inr { next }
{
  line = $0
  sub(/^[ \t]+/, "", line)
  if (substr(line, 1, 1) == "#") next
  m = split(line, chunks, /;/)
  for (i = 1; i <= m; i++) {
    c = chunks[i]
    sub(/^[ \t]+/, "", c)
    sub(/[ \t]+$/, "", c)
    if (substr(c, 1, 1) == "#") continue
    if (c ~ /^bash[ \t]+factory\/retention-census\.sh[ \t]+--self-test([ \t;]|$)/) self += 1
  }
}
END { printf "%d\n", self + 0 }
'

rc_leg_makefile() { # <path to Makefile>
	local makefile=$1 n
	if [ ! -f "$makefile" ]; then
		rc_fail "no $makefile: the pin cannot be read, and a census whose arithmetic nothing runs is not pinned -- this leg fails closed rather than reporting an absence it did not look for"
		return
	fi
	if ! n=$(awk "$RC_MAKEFILE_AWK" "$makefile"); then
		rc_fail "reading the verify recipe in $makefile failed: same rule, fail closed"
		return
	fi
	if [ "$n" != 1 ]; then
		rc_fail "make verify runs this file's --self-test in $n places, expected 1. The line to keep in the verify recipe is: bash factory/retention-census.sh --self-test -- without it the six-clock arithmetic is prose again, which is the state #334 was filed against"
	fi
}

# ------------------------------------------------------------------------------ self-test
#
# This fixture is the only place the repository can pin retention arithmetic at all: `factory/log` is
# untracked, so a unit's worktree has no journal and the live one is the operator's audit trail -- the
# same reason `jc_leg_dpn_stale_picks` builds its own `factory/log` inside its temp dir. So the
# fixtures are built by hand against a pinned `--now`, with mtimes stamped by `touch -d`, which is the
# seam `H-0169`'s `--now` exists to provide.
#
# The fixture's own precondition is asserted before any verdict is trusted, in the shape the earned-id
# leg now carries it: run the census over a fixture whose files the glob does not match and every
# clock separates zero from zero -- six agreeing numbers, which is the one state in which this leg
# controls nothing. So the census's own total line is asserted to name the six planted files and to
# exclude the two decoys, and four pairs of readings are asserted to DISAGREE.
rc_self_test() {
	local t now
	t=$(mktemp -d "${TMPDIR:-/tmp}/retention-census-selftest.XXXXXX") || return 1
	# shellcheck disable=SC2064
	trap "rm -rf -- '$t'" EXIT
	rc_fails=0

	# Wiring first: the arithmetic below is only a gate while `make verify` runs it.
	rc_leg_makefile "$_here/../Makefile"

	# 2026-01-15T12:00:00Z, so a 7-day cut lands at 2026-01-08T12:00:00Z -- inside the 20260108
	# transcripts: 3 minutes after one of them started, 4 h before another, 5 h before a third.
	now=$(date -u -d '2026-01-15T12:00:00Z' +%s)
	rc_fixture "$t/log"
	mkdir -p -- "$t/empty" "$t/young"

	# Precondition. `6` and `2400` are the planted transcripts and their planted total; the two decoys
	# carry 18887 B between them and are older than every cut, so a glob that admitted either one
	# moves THIS line and every number below -- and a leg asserting only the numbers would still pass,
	# having measured a fixture nobody built.
	rc_census UTC "$t/log" --days 7 --now "$now" >"$t/total.out" 2>"$t/total.err"
	rc_assert-total '6 2400' "$t/total.out"
	rc_stderr-quiet "$t/total.err" 'the fixture census'

	# The six clocks at one pinned instant. Expected, straight off the planted table in rc_fixture:
	#   A name-stamp  4 files 1100 B   three cut-day names plus the late-append name
	#   B mtime UTC   3 files  600 B   the late-append file's mtime is 6 days old, not 8
	#   C day <=      5 files 1700 B   the whole cut day, including a name younger than the cut
	#   D day <       2 files  800 B   the day before, and the late-append name
	#   E -mtime +7   0 files    0 B   GNU truncation: nothing planted is 8 days old
	#   F stamp, UTC  3 files  600 B   the same instant as B, because the zone is the same
	#   F stamp, -05  4 files 1200 B   plus the 15:00Z transcript: the stamp is read 5 h late
	rc_clock A '4 1100' "$t" "$t/log" UTC 7 "$now"
	rc_clock B '3 600' "$t" "$t/log" UTC 7 "$now"
	rc_clock C '5 1700' "$t" "$t/log" UTC 7 "$now"
	rc_clock D '2 800' "$t" "$t/log" UTC 7 "$now"
	rc_clock E '0 0' "$t" "$t/log" UTC 7 "$now"
	rc_clock F '3 600' "$t" "$t/log" UTC 7 "$now"
	rc_clock F '4 1200' "$t" "$t/log" '<-05>5' 7 "$now"

	# The control that says this fixture separates the clocks AT ALL. Six readings that agreed would
	# turn every assertion above into a tautology, and the shape is cheap to write and cheap to lose:
	# a later edit that moves a planted mtime to agree with its name silently retires this leg.
	rc_rows-differ E A "$t" "$t/log" UTC 7 "$now"
	rc_rows-differ A B "$t" "$t/log" UTC 7 "$now"
	rc_rows-differ C D "$t" "$t/log" UTC 7 "$now"
	rc_rows-differ B F "$t" "$t/log" '<-05>5' 7 "$now"

	# The clock the census was given is pinned, so row E did not come from find at all -- and the leg
	# above cannot tell a faithful emulation from one that merely agrees with itself. This fixture's
	# ages are planted from the REAL clock, which is the only clock -mtime can read, and the shipped
	# `find -mtime +7` is run against it next to the emulation. Margins are 0.9 and 1.5 days against
	# the eight-day threshold, so the leg is not sensitive to the moment it runs: a leg that is red
	# near a threshold and green away from it is the #355 minute-a-day defect in another file.
	rc_live_fixture "$t/live"
	rc_census UTC "$t/live" --days 7 >"$t/live.out" 2>"$t/live.err" ||
		rc_fail 'pricing a live-clock fixture at 7 days exited non-zero'
	rc_stderr-quiet "$t/live.err" 'the live-clock census'
	rc_row-is A '2 300' "$t/live.out" 'the live fixture, name clock'
	rc_row-is E '1 200' "$t/live.out" 'the live fixture, emulation'
	local shipped
	shipped=$(rc_find_cut "$t/live" UTC -mtime +7 | tr '\t' ' ')
	if [ "$shipped" != '1 200' ]; then
		rc_fail "the shipped find -mtime +7 named [$shipped] on a fixture planted at 7.9, 8.5 and 2.0 days, not [1 200]: the fixture's precondition is that GNU's truncation admits only the 8.5-day file, and the comparison below would be comparing nothing"
	elif [ "$(rc_row-get E "$t/live.out")" != "$shipped" ]; then
		rc_fail "the emulation read [$(rc_row-get E "$t/live.out")] where the shipped find -mtime +7 read [$shipped] on the same fixture: row E is no longer GNU's arithmetic, and every -mtime fault this census prints is this file's opinion instead of the tool's"
	fi
	if ! grep -q '^fault:' "$t/live.out"; then
		rc_fail 'the live fixture printed no fault: line, though -mtime +7 names one file where the name clock names two'
	fi

	# A pinned cut has to say that row E was not a call to find. Silence here is the shape this whole
	# clause exists to refuse: a printed number with no statement of where it came from.
	if ! grep -q '^pinned:' "$t/total.out"; then
		rc_fail 'a census taken at a --now hours from the real clock printed no pinned: line, so its six rows read as six measurements when one of them is arithmetic'
	fi

	# The byte cap, both clocks, over planted sizes 300 200 100 600 500 700.
	#   by mtime: keeps 700 + 500 = 1200 B, evicting 4 files and 1200 B
	#   by name:  keeps 700 + 600 = 1300 B, evicting 4 files and 1100 B
	# The two kept sets share one transcript, so two of them are decided by the clock alone.
	rc_evict-assert mtime '2 1200 4 1200' "$t" "$t/log" 1300
	rc_evict-assert name '2 1300 4 1100' "$t" "$t/log" 1300
	rc_differ-assert 2 "$t" "$t/log" 1300

	# Every refusal below is a wrong answer the tool must not print as a verdict.
	rc_refuse-assert 2 'no number at all' "$t/log" UTC --now "$now"
	rc_refuse-assert 2 'both numbers' "$t/log" UTC --days 7 --bytes 1300 --now "$now"
	rc_refuse-assert 2 'zero days' "$t/log" UTC --days 0 --now "$now"
	rc_refuse-assert 2 'a fractional day' "$t/log" UTC --days 7.5 --now "$now"
	rc_refuse-assert 2 'a negative cap' "$t/log" UTC --bytes -1300 --now "$now"
	rc_refuse-assert 2 'a path that is not a directory' "$t/no-such-dir" UTC --days 7 --now "$now"
	rc_refuse-assert 3 'a directory holding no transcripts' "$t/empty" UTC --days 7 --now "$now"

	# The pinned clock survives every leg above it, asserted rather than assumed: bash scopes variables
	# dynamically, so a helper that assigns `now` without declaring it local rewrites its caller's
	# clock and every row priced after that is at the real instant while reading as the pinned one.
	# That is not a hypothetical -- it happened while this leg was being written, one helper's
	# unnecessary `now=$(date -u +%s)` was the whole cause, and the only symptom was a census of a
	# young directory that had quietly stopped being zero.
	if [ "$now" != "$(date -u -d '2026-01-15T12:00:00Z' +%s)" ]; then
		rc_fail "the census clock stopped being pinned somewhere above this line: now is [$now], not the pinned [$(date -u -d '2026-01-15T12:00:00Z' +%s)]. A helper assigned it without declaring it local, and every row priced below would be at the real clock and read as the pinned one."
	fi

	# The quiet-zero case is clause 5's own named worst answer, so it is asserted as a statement the
	# tool makes rather than left as a silence: a young directory priced at 7 days has to say that the
	# zero is the directory's age and not the policy's cost.
	rc_touch "$t/young/20260115T110000Z.issue:young.log" 100 '2026-01-15T11:00:00Z'
	if ! rc_census UTC "$t/young" --days 7 --now "$now" >"$t/quiet.out" 2>"$t/quiet.err"; then
		rc_fail 'a directory of one young transcript was refused instead of priced'
	elif ! grep -q '^quiet:' "$t/quiet.out"; then
		rc_fail 'the all-zero census printed no quiet: line -- clause 5 names the quiet zero its worst answer'
	fi

	if [ "$rc_fails" -gt 0 ]; then
		printf '    retention-census.sh --self-test FAILED: %s assertion(s)\n' "$rc_fails" >&2
		return 1
	fi
	return 0
}

# Six transcripts, 2400 B, and two decoys that must not be counted. Every stamp carries an explicit
# `Z`: `touch -d` reads an unqualified stamp in the caller's zone, and a fixture whose mtimes landed
# in the box's zone would assert arithmetic against a clock this leg is trying to measure.
rc_fixture() { # <dir>
	local d=$1
	mkdir -p -- "$d"
	rc_touch "$d/20260108T115700Z.issue:c1.log" 100 '2026-01-08T11:57:00Z' # 3 min older than the cut
	rc_touch "$d/20260108T080000Z.issue:c2.log" 200 '2026-01-08T08:00:00Z' # mid-day, both clocks
	rc_touch "$d/20260107T200000Z.issue:c3.log" 300 '2026-01-07T20:00:00Z' # the day before, both clocks
	# the late append: named 8 days ago, last written 6 days ago. This one file is why clause 5 says
	# name and mtime are different facts rather than proxies for one fact.
	rc_touch "$d/20260107T220000Z.issue:c5.log" 500 '2026-01-09T13:00:00Z'
	# 3 h younger than the cut and inside a 5 h zone window: the transcript the zone decides.
	rc_touch "$d/20260108T150000Z.issue:c6.log" 600 '2026-01-08T15:00:00Z'
	rc_touch "$d/20260115T110000Z.issue:c7.log" 700 '2026-01-15T11:00:00Z' # today, kept by every clock
	# decoys: the journal, with its own rotation, and a name that is not a transcript. Both are older
	# than every cut, so if either were counted the total line moves and every count below is wrong.
	rc_touch "$d/supervisor.log" 9999 '2025-12-01T00:00:00Z'
	rc_touch "$d/20251201T000000Z.issue:decoy.txt" 8888 '2025-12-01T00:00:00Z'
}

rc_touch() { # <path> <bytes> <mtime, ISO with an explicit Z>
	head -c "$2" /dev/zero >"$1"
	touch -d "$3" -- "$1"
}

rc_census() { # <TZ or empty> <dir> <args...> -> rc_main, in a subshell so its EXIT trap runs
	local tz=$1 dir=$2
	shift 2
	(
		TZ=${tz:-UTC}
		export TZ
		rc_main --dir "$dir" "$@"
	)
}

rc_census_err() { # <TZ or empty> <dir> <args...> -> whatever the census wrote on stderr
	local tz=$1 dir=$2
	shift 2
	(
		TZ=${tz:-UTC}
		export TZ
		# stderr, and only stderr, on this helper's stdout: the braces keep the group's stderr aimed at the
		# caller's capture while the command's own stdout is sent away first. `2>&1 >/dev/null` would say the
		# same thing here by accident of ordering, which is the reading SC2069 exists to refuse.
		{ rc_main --dir "$dir" "$@" 1>/dev/null; } 2>&1
	)
}

rc_census_total() { # <file> -> "<count> <bytes>" out of the census line
	awk '/transcripts,/ { print $1, $3; exit }' "$1"
}

rc_fail() {
	printf '    retention-census.sh: %s\n' "$1" >&2
	rc_fails=$((rc_fails + 1))
}

# A census writes its verdict on stdout and nothing on stderr, so a non-empty stderr is always someone
# else's message arriving in a file this leg reads as rows. The xtrace guard above is the case that
# found this; a warning from a future edit is the case it guards against.
rc_stderr-quiet() { # <err file> <what the run was>
	local n
	n=$(wc -c <"$1")
	n=$((n))
	if [ "$n" != 0 ]; then
		rc_fail "$2 wrote $n byte(s) on stderr where a census writes nothing: [$(tr '\n' ' ' <"$1" | cut -c1-160)]"
	fi
}

rc_assert-total() { # "<n> <bytes>" <file>
	local got
	got=$(rc_census_total "$2")
	if [ "$got" != "$1" ]; then
		rc_fail "FIXTURE-FAILED the census reported [$got], expected [$1]. The glob must take exactly the six planted transcripts and refuse the journal and the .txt; every number below would still print, and none of them would be about this fixture."
		printf '        ---- census output\n' >&2
		sed 's/^/          /' "$2" >&2
	fi
}

rc_clock() { # <letter> "<n> <bytes>" <tmp> <dir> <TZ> <days> <now>
	local letter=$1 want=$2 t=$3 dir=$4 tz=$5 n=$6 now=$7 got tag
	tag="$letter-$tz"
	if ! rc_census "$tz" "$dir" --days "$n" --now "$now" >"$t/$tag.out" 2>"$t/$tag.err"; then
		rc_fail "pricing 7 days in zone [$tz] exited non-zero"
		sed 's/^/          /' "$t/$tag.err" >&2
	else
		rc_stderr-quiet "$t/$tag.err" "clock $letter in zone [$tz]"
	fi
	got=$(awk -v l="$letter" '$1 == l { print $2, $4; exit }' "$t/$tag.out")
	if [ "$got" != "$want" ]; then
		rc_fail "clock $letter in zone [$tz] read [$got], expected [$want]"
		printf '        ---- census output\n' >&2
		sed 's/^/          /' "$t/$tag.out" >&2
	fi
}

rc_rows-differ() { # <a> <b> <tmp> <dir> <TZ> <days> <now>
	local a=$1 b=$2 t=$3 dir=$4 tz=$5 n=$6 now=$7 ga gb tag
	tag="cmp-$tz-$a-$b"
	rc_census "$tz" "$dir" --days "$n" --now "$now" >"$t/$tag.out" 2>"$t/$tag.err" || true
	if [ -s "$t/$tag.err" ]; then
		rc_fail "the comparison run for clocks $a and $b in zone [$tz] wrote to stderr where a census writes nothing: [$(tr '\n' ' ' <"$t/$tag.err" | cut -c1-160)] -- its rows below are read from a file that may not be a census"
	fi
	ga=$(awk -v l="$a" '$1 == l { print $2, $4; exit }' "$t/$tag.out")
	gb=$(awk -v l="$b" '$1 == l { print $2, $4; exit }' "$t/$tag.out")
	if [ "$ga" = "$gb" ]; then
		rc_fail "control failed: clocks $a and $b both read [$ga] in zone [$tz], so this fixture does not separate them and the assertions above measure nothing. A planted mtime has probably been moved to agree with its name."
	fi
}

# A fixture whose ages are measured from the instant the leg runs, because it exists to be compared
# against `-mtime`, and `-mtime` has no other clock to read. Ages are 7.9, 8.5 and 2.0 days: the
# first is inside the seven-day policy and outside GNU's eight-day truncation, which is the fault this
# census prints, and the other two sit more than a day clear of the threshold so the leg does not tip
# over it at an inconvenient hour. Name and mtime are the same instant here on purpose -- this leg is
# about the truncation, not about the clock split, and the planted mtimes are stamped with `@epoch` so
# no zone is consulted.
rc_live_fixture() { # <dir>
	local d=$1
	mkdir -p -- "$d"
	rc_live_one "$d" 682560 100 'live-a' #  7.9 days: named by the policy, missed by -mtime +7
	rc_live_one "$d" 734400 200 'live-b' #  8.5 days: named by both
	rc_live_one "$d" 172800 300 'live-c' #  2.0 days: named by neither
}

rc_live_one() { # <dir> <age in seconds> <bytes> <unit>
	local instant stamp
	instant=$(($(date -u +%s) - $2))
	stamp=$(date -u -d "@$instant" +%Y%m%dT%H%M%SZ)
	head -c "$3" /dev/zero >"$1/$stamp.$4.log"
	touch -d "@$instant" -- "$1/$stamp.$4.log"
}

# One row out of a census's own output, as "<count> <bytes>".
rc_row-get() { # <letter> <file>
	awk -v l="$1" '$1 == l { print $2, $4; exit }' "$2"
}

rc_row-is() { # <letter> "<n> <bytes>" <file> <what the run was>
	local got
	got=$(rc_row-get "$1" "$3")
	if [ "$got" != "$2" ]; then
		rc_fail "$4: row $1 read [$got], expected [$2]"
		printf '        ---- census output\n' >&2
		sed 's/^/          /' "$3" >&2
	fi
}

rc_evict-assert() { # <mtime|name> "<kept_n kept_b evict_n evict_b>" <tmp> <dir> <cap>
	local clock=$1 want=$2 t=$3 dir=$4 cap=$5 got
	if ! rc_census UTC "$dir" --bytes "$cap" >"$t/evict-$clock.out" 2>"$t/evict-$clock.err"; then
		rc_fail "pricing a cap of $cap exited non-zero"
		sed 's/^/          /' "$t/evict-$clock.err" >&2
	else
		rc_stderr-quiet "$t/evict-$clock.err" "the $clock-clock cap census"
	fi
	if [ "$clock" = mtime ]; then
		got=$(awk '/mtime clock/ { print $4, $6, $9, $11; exit }' "$t/evict-$clock.out")
	else
		got=$(awk '/name  clock/ { print $4, $6, $9, $11; exit }' "$t/evict-$clock.out")
	fi
	if [ "$got" != "$want" ]; then
		rc_fail "the $clock clock at a cap of $cap read [$got], expected [$want]"
		printf '        ---- census output\n' >&2
		sed 's/^/          /' "$t/evict-$clock.out" >&2
	fi
}

rc_differ-assert() { # <expected> <tmp> <dir> <cap>
	local want=$1 t=$2 dir=$3 cap=$4 got
	rc_census UTC "$dir" --bytes "$cap" >"$t/differ.out" 2>"$t/differ.err" || true
	if [ -s "$t/differ.err" ]; then
		rc_fail "the differ run at a cap of $cap wrote to stderr where a census writes nothing: [$(tr '\n' ' ' <"$t/differ.err" | cut -c1-160)]"
	fi
	got=$(awk '/^differ:/ { print $2; exit }' "$t/differ.out")
	if [ "$got" != "$want" ]; then
		rc_fail "the two clocks differ by [$got] transcripts at a cap of $cap, expected [$want]: with no clock named, the cap does not decide which set the operator loses, and this leg is what says so"
	fi
}

rc_refuse-assert() { # <code> <what> <dir> <TZ> <args...>
	local code=$1 what=$2 dir=$3 tz=$4 out rc=0
	shift 4
	out=$(rc_census "$tz" "$dir" "$@" 2>/dev/null) || rc=$?
	err=$(rc_census_err "$tz" "$dir" "$@")
	if [ "$rc" != "$code" ]; then
		rc_fail "'$what' exited [$rc], expected [$code]: $err$out"
	elif printf '%s' "$out$err" | grep -q 'transcripts,'; then
		rc_fail "'$what' exited [$code] AND printed a census -- a refusal must not also be a verdict"
	elif [ -z "$err" ]; then
		rc_fail "'$what' exited [$code] and said nothing on stderr -- a refusal has to name what it refused, and a census's rows are not that name"
	fi
}

# The legs below capture the census's stderr into files precisely in order to prove those files stay
# empty. An xtrace is written on that same descriptor, so tracing this script writes the trace INTO the
# captures and every row is then read out of trace noise -- measured, and the noise arrives as a FAILED
# fixture assertion rather than as a warning, which is the failure shape clause 5 refuses. Measured on
# `bash -x factory/retention-census.sh --self-test`: three legs fail and the messages quote lines like
# `[+ '\']`, so the refusal says the real cause instead of leaving a red gate whose symptom is elsewhere.
rc_refuse_xtrace() {
	case $- in
		# bash spells a command-line `-x` as UPPER_CASE X in $- and a `set -x` as lowercase x, measured:
		# `bash -x -c 'echo $-'` answers `hxBc`. Matching only `*x*` misses the invocation that matters.
		*[xX]*)
			printf '%s\n' "retention-census.sh: --self-test refuses to run under xtrace (\$- is '$-'): the legs capture the census's stderr to prove it stays empty, and a trace is written into exactly those captures. Run it plain, and trace a census instead: bash -x $0 --days 7 --dir <dir>" >&2
			exit 2
			;;
	esac
}

if [ "${1:-}" = --self-test ]; then
	rc_refuse_xtrace
	if rc_self_test; then
		exit 0
	fi
	exit 1
fi
rc_main "$@"
