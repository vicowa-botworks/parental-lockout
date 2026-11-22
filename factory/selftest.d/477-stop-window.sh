# shellcheck shell=bash
# shellcheck disable=SC2016,SC2034,SC2154
# 477-stop-window.sh -- the subject of `REAP-HELD … how=unexplained` (#477).
#
# One row in the retained supervisor journal has a duration and no subject:
# `2026-11-19T17:11:09Z REAP-HELD hold=44s how=unexplained ended=624s open=1`, while every other reap
# on 19 Nov reads `REAP-CLEAN hold=0s how=clean`. `unexplained` is not a verdict, it is the shape of
# the evidence: `factory_reap_read` names a hold `deadline` when systemd wrote `State 'stop-sigterm'
# timed out. Killing.` and `client` when it wrote `with SIGKILL on client request.`, so the third arm
# means the manager wrote nothing and the queue froze for 44 s with nobody in the record. `H-0312`'s
# `Follow-up:` kept that question open on purpose after the #390 fence merged, because the fence
# removes the likeliest old answer (`winedevice.exe`, measured in this unit's cgroup on 8 Nov) without
# answering it -- and the kernel forgets a cgroup's members when the cgroup is gone. Re-reading that
# stop after the fact is not possible on this box: `journalctl --user -u <unit> --since
# 2026-11-19T17:09:00 --until 2026-11-19T17:12:00` answers `-- No entries --` (measured 22 Nov, oldest
# retained row 2026-11-20T04:59). So the attribution is an instrument, `factory/stop-window-probe.sh`,
# spawned inside the cgroup by the supervisor pass and read back by the next pass's reap read as
# `stop=` on the `REAP-HELD` line and as a clause of the `--status` sentence.
#
# What the legs pin, and the polarities:
#   (a) the premise and its control, together: a record stamped inside the window the reap's own two
#       stamps define is found, and the same record 60 s outside it is not. Either alone is vacuous --
#       the first passes if the matcher accepts anything, the second passes if it accepts nothing;
#   (b) the subject list: newest record wins, `comm` cannot forge a field (`, ; =`, space and control
#       bytes flattened), a long list truncates at `FACTORY_STOP_WINDOW_MAX` with `,+K`, and the knob
#       is the knob;
#   (c) the four no-subject arms are four different facts, and the live-probe case is none of them:
#       an absent directory and a foreign unit are `none`, `sample=no` is `unread`, a sampled-empty
#       cgroup is `0`, an armed probe whose process is gone with no record is `unwritten`, and an
#       armed probe that is still running is this activation's observer, not evidence about the last;
#   (d) the guards answer without looking -- which the fixture proves by planting a record that WOULD
#       have matched, in a directory the guard is not asked about;
#   (e) the wiring into the shipped composer, through `rp_journal`/`rp_sentence`: a 44 s `unexplained`
#       transcript carries `stop=2:…` naming both holders when a record exists and `stop=none` when
#       it does not, and the `--status` sentence says what it caught or admits it cannot name one;
#   (f)-(i) the probe itself: it refuses a foreign cgroup and an empty argv in silence; it arms before
#       it is killable; `SIGTERM` writes `unit=` and a `t=` that is microseconds (a naive `%s%N`
#       truncation writes 1970, measured here first) and lists a real sibling with its `comm` while
#       filtering the observer's own pids; an unreadable `cgroup.procs` writes `sample=no`, not an
#       empty list; the handler removes the marker and leaves the unit's stdio at zero bytes; and it
#       dies in well under a second, because a probe slow to die is itself a holder of the stop it
#       watches; the day-old sweep runs at spawn and spares a fresh record;
#   (j) the spawn site: exactly one, inside `factory/factory.sh` before `factory_body`, addressed
#       through `FACTORY_DIR` with the unit name and the run dir, and backgrounded.

sw_work=$work/stop477
sw_probe=$HERE/stop-window-probe.sh
sw_unit=plrev283b.service # the unit rp_journal tells common.sh it is running as
sw_cg=/sys/fs/cgroup/user.slice/user-1000.slice/user@1000.service/app.slice/$sw_unit
# the 19 Nov stop, one clock with the transcript legs (e) writes: work line 17:10:25, terminal
# 17:11:09, so hold=44 and the window is [17:10:20, 17:11:14] at the default grace.
sw_term=$(date -u -d '2026-11-19 17:11:09' +%s)

sw_say() { printf '%s\n' "$*" | sed 's/^/            /' >&2; }
sw_us() { printf '%s000000' "$(date -u -d "$1" +%s)"; }   # the reader's clock unit
sw_rec() { printf 'unit=%s\nt=%s\n%s' "$3" "$4" "$5" >"$1/$2"; }
sw_age() { printf '%s' "$(($(date -u +%s) - sw_term))"; } # the age the read will compute

# sw_ask <dir> <hold> <age> [NAME=value ...] -> the stop= field. The directory is always named, so
# no leg can reach the real $FACTORY_RUN.
#
# The stub names the unit through the environment, the way `rp_journal`'s stub does with `RP_UNIT`,
# and NOT with `$2` inside the `-c` string: once `factory_service_unit` is a function, `$2` is that
# function's own positional list, which is empty, so the stub answers `` and the reader refuses every
# record for a unit mismatch -- three of these legs read `none` on exactly that fixture bug on 22 Nov
# while the legs that go through `rp_journal` stayed green, which is how it was caught.
sw_ask() {
	local dir=$1 hold=$2 age=$3
	shift 3
	env FACTORY_STOP_WINDOW_DIR="$dir" SW_LEG_UNIT="$sw_unit" "$@" bash -c '
		. "$1/common.sh" 1>/dev/null
		factory_service_unit() { printf "%s\n" "$SW_LEG_UNIT"; }
		factory_reap_stop_window "$2" "$3"' _ "$HERE" "$hold" "$age"
}
sw_said() { # sw_said <dir> <hold> <age> -> the clause the --status sentence carries
	local dir=$1 hold=$2 age=$3
	env FACTORY_STOP_WINDOW_DIR="$dir" SW_LEG_UNIT="$sw_unit" bash -c '
		. "$1/common.sh" 1>/dev/null
		factory_service_unit() { printf "%s\n" "$SW_LEG_UNIT"; }
		factory_reap_stop_saying "$2" "$3"' _ "$HERE" "$hold" "$age"
}
# sw_want <label> <want> <dir> <hold> <age> [NAME=value ...] -- the field, compared.
sw_want() {
	local label=$1 want=$2 dir=$3 hold=$4 age=$5 got=''
	shift 5
	got=$(sw_ask "$dir" "$hold" "$age" "$@")
	[ "$got" = "$want" ] || { sw_say "$label: want [$want], got [$got]"; return 1; }
	return 0
}

# (a) Premise and control, in one leg because each is the other's falsifier.
swp_477_premise() {
	local d=$sw_work/premise got=''
	mkdir -p -- "$d" || return 1
	sw_rec "$d" stop-a "$sw_unit" "$(sw_us '2026-11-19 17:10:52')" 'proc 701 held-thing
'
	got=$(sw_ask "$d" 44 "$(sw_age)")
	if [ "$got" != "1:701=held-thing" ]; then
		sw_say 'a record inside the window was not found, so the reader cannot attach a probe to a'
		sw_say "stop it was armed for. want [1:701=held-thing], got [$got]"
		return 1
	fi
	sw_rec "$d" stop-a "$sw_unit" "$(sw_us '2026-11-19 17:09:55')" 'proc 701 held-thing
'
	got=$(sw_ask "$d" 44 "$(sw_age)")
	if [ "$got" != none ]; then
		sw_say 'a record 60 s ahead of the work line matched a window bounded by hold+grace, so every'
		sw_say '"the probe caught it" below could be the previous activation talking. want [none],'
		sw_say "got [$got]"
		return 1
	fi
	return 0
}
claim_leg 'stop window: the window is a window' swp_477_premise

# (b) Newest wins, the sanitiser, the cap.
swp_477_subjects() {
	local d=$sw_work/subjects got=''
	mkdir -p -- "$d" || return 1
	sw_rec "$d" stop-old "$sw_unit" "$(sw_us '2026-11-19 17:10:40')" 'proc 10 older-holder
'
	sw_rec "$d" stop-new "$sw_unit" "$(sw_us '2026-11-19 17:10:52')" \
		'proc 20 first,holder
proc 21 second;holder
proc 22 third=holder
proc 23 fourth holder
proc 24 fifth
proc 25 sixth
proc 26 seventh
proc 27 eighth
proc 28 ninth
'
	got=$(sw_ask "$d" 44 "$(sw_age)")
	if [ "$got" != '9:20=first_holder,21=second_holder,22=third-holder,23=fourth_holder,24=fifth,25=sixth,26=seventh,27=eighth,+1' ]; then
		sw_say 'newest-record-wins, the sanitiser or the cap is wrong. Want the nine procs of the'
		sw_say 'NEWER record, `, ; =` and the space flattened, and `…,+1` at the default cap of 8:'
		sw_say "got [$got]"
		return 1
	fi
	sw_rec "$d" stop-new "$sw_unit" "$(sw_us '2026-11-19 17:10:52')" 'proc 20 older-holder
proc 21 second
'
	got=$(sw_ask "$d" 44 "$(sw_age)" FACTORY_STOP_WINDOW_MAX=1)
	[ "$got" = '2:20=older-holder,+1' ] || {
		sw_say "FACTORY_STOP_WINDOW_MAX=1 is not the knob: got [$got]"
		return 1
	}
	return 0
}
claim_leg 'stop window: one subject row, flattened and capped' swp_477_subjects

# (c) The four no-subject arms, plus the live probe.
swp_477_arms() {
	local d=$sw_work/arms age got=''
	mkdir -p -- "$d/foreign" "$d/unread" "$d/empty" "$d/unwritten" "$d/live" || return 1
	age=$(sw_age)
	sw_rec "$d/foreign" stop-x other.service "$(sw_us '2026-11-19 17:10:52')" 'proc 1 theirs
'
	sw_rec "$d/unread" stop-x "$sw_unit" "$(sw_us '2026-11-19 17:10:52')" 'sample=no
'
	sw_rec "$d/empty" stop-x "$sw_unit" "$(sw_us '2026-11-19 17:10:52')" ''
	sw_rec "$d/live" stop-x "$sw_unit" "$(sw_us '2026-11-19 17:10:52')" 'proc 5 real
'
	: >"$d/unwritten/alive-987654"
	touch -d '2026-11-19 17:10:52' "$d/unwritten/alive-987654" || return 1
	: >"$d/live/alive-$$" # this shell answers, so this marker says nothing about the last stop
	sw_want 'absent directory' none "$d/no-such-dir" 44 "$age" || return 1
	sw_want 'foreign unit' none "$d/foreign" 44 "$age" || return 1
	sw_want 'unread sample' unread "$d/unread" 44 "$age" || return 1
	sw_want 'sampled empty' 0 "$d/empty" 44 "$age" || return 1
	sw_want 'armed probe, no record' unwritten "$d/unwritten" 44 "$age" || return 1
	sw_want 'armed probe, still alive' '1:5=real' "$d/live" 44 "$age" || return 1
	rm -f -- "$d/unwritten/alive-987654"
	sw_want 'marker outside the window' none "$d/unwritten" 44 "$age" || return 1
	got=$(sw_said "$d/unread" 44 "$age")
	case $got in
	*"unread, not absent"*) : ;;
	*) sw_say "the unread clause does not say the subject is unread, not absent: [$got]"; return 1 ;;
	esac
	got=$(sw_said "$d/no-such-dir" 44 "$age")
	case $got in
	*"no stop-window evidence"*) : ;;
	*) sw_say "the absent clause does not admit that nothing names the subject: [$got]"; return 1 ;;
	esac
	return 0
}
claim_leg 'stop window: four absences are four facts' swp_477_arms

# (d) A malformed field answers before the directory is opened.
swp_477_guards() {
	local d=$sw_work/guards age
	mkdir -p -- "$d/yes" "$d/junk" || return 1
	age=$(sw_age)
	sw_rec "$d/yes" stop-x "$sw_unit" "$(sw_us '2026-11-19 17:10:52')" 'proc 1 would-match
'
	sw_want 'hold=abc' none "$d/yes" abc "$age" || return 1
	sw_want 'age negative' none "$d/yes" 44 -3 || return 1
	printf 'proc 1 no headers at all\n' >"$d/junk/stop-a"
	printf 'unit=%s\nt=not-a-number\nproc 1 x\n' "$sw_unit" >"$d/junk/stop-b"
	sw_want 'records with no usable stamp' none "$d/junk" 44 "$age" || return 1
	return 0
}
claim_leg 'stop window: a bad field never reaches a record' swp_477_guards

# (e) The shipped composer: the field on the journal line, the clause in the sentence.
swp_477_composer() {
	local d=$sw_work/composer out=''
	mkdir -p -- "$d/with" "$d/without" || return 1
	cat >"$sw_work/transcript" <<F
2026-11-19T17:10:25.000000+00:00 h factory.sh[4242]: 2026-11-19T17:10:25Z END rc=0 ticks_started=1
2026-11-19T17:11:09.000000+00:00 h systemd[4485]: Finished $sw_unit - planted (one pass).
F
	sw_rec "$d/with" stop-x "$sw_unit" "$(sw_us '2026-11-19 17:10:52')" \
		'proc 301 winedevice.exe
proc 302 wineserver
'
	out=$(export FACTORY_STOP_WINDOW_DIR="$d/with"
		rp_journal "$sw_work/transcript" 1) || { sw_say 'the composer refused its own transcript'; return 1; }
	if [ "$(printf '%s\n' "$out" | grep -c .)" != 1 ]; then
		sw_say "want exactly one REAP line for one activation, got:"
		sw_say "$out"
		return 1
	fi
	case $out in
	'REAP-HELD hold=44s how=unexplained '*stop=2:301=winedevice.exe,302=wineserver\ *) : ;;
	*) sw_say "the planted holders are not on the line, holding=44 with the subject missing:"
		sw_say "$out"
		return 1 ;;
	esac
	out=$(export FACTORY_STOP_WINDOW_DIR="$d/without"
		rp_journal "$sw_work/transcript" 1) || { sw_say 'the composer refused with no records'; return 1; }
	case $out in
	*stop=none\ --\ *) : ;;
	*) sw_say "an uninstrumented activation must say so rather than name nothing:"
		sw_say "$out"
		return 1 ;;
	esac
	out=$(export FACTORY_STOP_WINDOW_DIR="$d/with"
		rp_sentence "$sw_work/transcript" 1) || { sw_say 'the sentence composer refused'; return 1; }
	case $out in
	*'stop-window probe caught it: 301=winedevice.exe,302=wineserver'*) : ;;
	*) sw_say '--status has the subject and does not name it:'
		sw_say "$out"
		return 1 ;;
	esac
	out=$(export FACTORY_STOP_WINDOW_DIR="$d/without"
		rp_sentence "$sw_work/transcript" 1) || { sw_say 'the sentence composer refused, empty'; return 1; }
	case $out in
	*"no stop-window evidence"*) : ;;
	*) sw_say '--status should admit that nothing names the subject:'
		sw_say "$out"
		return 1 ;;
	esac
	return 0
}
claim_leg 'stop window: the field rides the REAP line' swp_477_composer

# swp_probe_stop <dir> <procs-file> -- arm the probe, then SIGTERM it the way the manager does.
# Prints the record path. The milliseconds to death go to <dir>/death.log, and the unit's stdio to
# <dir>/stdio.log, which the leg asserts is empty.
swp_probe_stop() {
	local d=$1 procs=$2 armed='' wrap='' t0='' t1='' rec='' f=''
	mkdir -p -- "$d" || return 1
	( FACTORY_STOP_WINDOW_CGPATH=$sw_cg FACTORY_STOP_WINDOW_PROCS=$procs \
		bash "$sw_probe" "$sw_unit" "$d" >"$d/stdio.log" 2>&1 ) &
	wrap=$!
	for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
		for f in "$d"/alive-*; do
			[ -e "$f" ] && armed=$f
		done
		[ -n "$armed" ] && break
		sleep 0.2
	done
	[ -n "$armed" ] || { sw_say "the probe never armed in $d, so nothing was ever killable"; return 1; }
	t0=$(date +%s%N)
	kill -TERM "$wrap" 2>/dev/null
	wait "$wrap" 2>/dev/null
	t1=$(date +%s%N)
	printf 'probe-death-ms %s\n' "$(( (t1 - t0) / 1000000 ))" >"$d/death.log"
	rec=$(find "$d" -maxdepth 1 -name 'stop-*' -print | head -n 1)
	[ -n "$rec" ] || { sw_say "SIGTERM wrote no record in $d"; return 1; }
	printf '%s\n' "$rec"
}

# (f) The probe refuses what it cannot attribute.
swp_477_probe_refuses() {
	local d=$sw_work/refuse out=$sw_work/refuse.out n=0
	mkdir -p -- "$d" || return 1
	rm -f -- "$out"
	FACTORY_STOP_WINDOW_CGPATH="${sw_cg%/*}/other.scope" FACTORY_STOP_WINDOW_PROCS=/dev/null \
		bash "$sw_probe" "$sw_unit" "$d/foreign" >"$out" 2>&1 || {
		sw_say 'a foreign cgroup must be a clean exit, not a refusal the pass has to survive'
		return 1
	}
	[ -s "$out" ] && { sw_say 'a foreign cgroup produced output:'; sw_say "$(cat "$out")"; return 1; }
	if [ -d "$d/foreign" ] && [ -n "$(ls -A "$d/foreign" 2>/dev/null)" ]; then
		sw_say 'a foreign cgroup left a file behind -- a wrong subject is worse than none:'
		ls -A "$d/foreign" | sed 's/^/              /' >&2
		return 1
	fi
	bash "$sw_probe" "" "$d/argv" >"$out" 2>&1 || { sw_say 'no unit name must still be a clean exit'; return 1; }
	bash "$sw_probe" "$sw_unit" "" >"$out" 2>&1 || { sw_say 'no directory must still be a clean exit'; return 1; }
	n=$(find "$d" -name 'stop-*' -o -name 'alive-*' | grep -c .)
	[ "$n" = 0 ] || { sw_say "the refusals left $n file(s) behind"; return 1; }
	return 0
}
claim_leg 'stop-window probe: refusal is silent and empty' swp_477_probe_refuses

# (g) The record a stop leaves behind.
swp_477_probe_record() {
	local d=$sw_work/record rec='' t='' now='' ms='' sib='' probe_pid='' slp=''
	mkdir -p -- "$d" || return 1
	sleep 20 &
	sib=$!
	printf '%s\n999999\n' "$sib" >"$sw_work/procs-list"
	rec=$(swp_probe_stop "$d" "$sw_work/procs-list" | tail -n 1) || {
		kill "$sib" 2>/dev/null
		return 1
	}
	kill "$sib" 2>/dev/null
	grep -qx "^unit=$sw_unit$" "$rec" || {
		sw_say "the record does not name its unit, so the reader must refuse it: $(cat "$rec")"
		return 1
	}
	t=$(sed -n 's/^t=\([0-9][0-9]*\)$/\1/p' "$rec" | head -n 1)
	now=$(date -u +%s)
	# the exponent, not just the digits: a stamp that reads as 1970 is the bug this leg exists for
	if [ -z "$t" ] || [ $(( now - t / 1000000 )) -gt 5 ]; then
		sw_say "t=$t is not within 5 s of now=$now READ AS MICROSECONDS: a truncated %s%N lands in"
		sw_say '1970 and the reader then answers none for every record the probe ever wrote'
		return 1
	fi
	grep -qx "proc $sib sleep" "$rec" || {
		sw_say "the sibling standing in the cgroup is not listed with its comm: $(cat "$rec")"
		return 1
	}
	grep -qx 'proc 999999 ?' "$rec" || {
		sw_say "a dead pid's comm must fall back to ?, not vanish: $(cat "$rec")"
		return 1
	}
	probe_pid=$(sed -n 's/^probe=\([0-9]*\)$/\1/p' "$rec")
	slp=$(sed -n 's/^sleeper=\([0-9]*\)$/\1/p' "$rec")
	[ -n "$probe_pid" ] || { sw_say "no probe= line, so the observer is unaccounted for: $(cat "$rec")"; return 1; }
	if grep -qx "proc $probe_pid .*" "$rec" || { [ -n "$slp" ] && grep -qx "proc $slp .*" "$rec"; }; then
		sw_say "the observer listed itself as a holder: $(cat "$rec")"
		return 1
	fi
	[ -z "$(find "$d" -maxdepth 1 -name 'alive-*' -print -quit)" ] || {
		sw_say 'the handler left its marker, and the next read calls this stop unwritten'
		return 1
	}
	[ -s "$d/stdio.log" ] && {
		sw_say 'the probe wrote the unit stdio, which is the last-word channel it must never touch:'
		sw_say "$(cat "$d/stdio.log")"
		return 1
	}
	ms=$(sed -n 's/^probe-death-ms \([0-9]*\)$/\1/p' "$d/death.log" 2>/dev/null)
	[ "${ms:-0}" -le 900 ] || {
		sw_say "the probe took ${ms} ms to die, which makes it a holder of the stop it watches"
		return 1
	}
	return 0
}
claim_leg 'stop-window probe: the stop record is the answer' swp_477_probe_record

# (h) An unreadable cgroup is its own fact, not an empty one.
swp_477_probe_unread() {
	local d=$sw_work/unread rec=''
	rec=$(swp_probe_stop "$d" "$sw_work/no-such-dir/cgroup.procs" | tail -n 1) || return 1
	grep -qx 'sample=no' "$rec" || {
		sw_say 'an unreadable cgroup.procs must write sample=no, not a silent empty list:'
		sw_say "$(cat "$rec")"
		return 1
	}
	return 0
}
claim_leg 'stop-window probe: unread is not empty' swp_477_probe_unread

# (i) Housekeeping runs where forking is free -- at spawn, before the handler is ever invoked.
swp_477_probe_sweep() {
	local d=$sw_work/sweep armed='' f='' wrap=''
	mkdir -p -- "$d" || return 1
	: >"$d/stop-old"
	: >"$d/stop-old.tmp"
	: >"$d/alive-987650"
	: >"$d/stop-fresh"
	touch -d '2 days ago' "$d/stop-old" "$d/stop-old.tmp" "$d/alive-987650" || return 1
	( FACTORY_STOP_WINDOW_CGPATH=$sw_cg FACTORY_STOP_WINDOW_PROCS=/dev/null \
		bash "$sw_probe" "$sw_unit" "$d" >/dev/null 2>&1 ) &
	wrap=$!
	for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
		for f in "$d"/alive-*; do
			[ -e "$f" ] || continue
			case $f in */alive-987650) : ;; *) armed=$f ;; esac
		done
		[ -n "$armed" ] && break
		sleep 0.2
	done
	if [ -z "$armed" ]; then
		sw_say 'the probe never armed, so the sweep was never reached'
		kill -TERM "$wrap" 2>/dev/null
		return 1
	fi
	# the sweep ran at spawn: the handler has not been invoked yet
	for f in stop-old stop-old.tmp alive-987650; do
		[ -e "$d/$f" ] && {
			sw_say "a two-day-old $f survived the spawn sweep"
			kill -TERM "$wrap" 2>/dev/null
			return 1
		}
	done
	[ -e "$d/stop-fresh" ] || {
		sw_say 'the sweep retired a fresh record, erasing the attribution it was given'
		kill -TERM "$wrap" 2>/dev/null
		return 1
	}
	kill -TERM "$wrap" 2>/dev/null
	wait "$wrap" 2>/dev/null
	[ -e "$d/stop-fresh" ] || { sw_say 'the handler deleted a record it did not write'; return 1; }
	[ -z "$(find "$d" -maxdepth 1 -name '*.tmp' -print -quit)" ] || {
		sw_say 'the handler left a tmp behind: a half record a later read may chew'
		return 1
	}
	return 0
}
claim_leg 'stop-window probe: housekeeping is not in the handler' swp_477_probe_sweep

# (j) One spawn site, in the pass, before the body.
swp_477_wiring() {
	local f=$HERE/factory.sh n=0 spawn='' body='' redir=''
	n=$(grep -c 'stop-window-probe\.sh' "$f")
	[ "$n" -le 3 ] || {
		sw_say "$n mentions of the probe in factory.sh; want the call and the comments that point at it"
		return 1
	}
	spawn=$(grep -n '^bash "\$FACTORY_DIR/stop-window-probe\.sh"' "$f" | head -n 1 | cut -d: -f1)
	if [ -z "$spawn" ]; then
		sw_say 'the spawn line is missing: it must be backgrounded, addressed through FACTORY_DIR, and'
		sw_say 'carry the unit name and the run directory'
		return 1
	fi
	# The stdio the spawn opens is the unit's, and a line there is a process line, which is what
	# factory_reap_lastword reads back as the pass's last word. bash prints its own error before any
	# line of the probe runs, so the channel has to be closed at the spawn, not inside the probe.
	redir=$(sed -n "$((spawn + 1)),$((spawn + 1))p" "$f")
	case $redir in
	*"</dev/null >/dev/null 2>&1 &"*) : ;;
	*) sw_say "the spawn does not close the unit's stdio (line $((spawn + 1))): [$redir]"
		return 1 ;;
	esac
	body=$(grep -n '^factory_body$' "$f" | head -n 1 | cut -d: -f1)
	if [ -z "$body" ] || [ "$spawn" -ge "$body" ]; then
		sw_say "the probe must be spawned by the pass (line $spawn) before factory_body (line"
		sw_say "${body:-none}), or it is not standing in the cgroup it samples"
		return 1
	fi
	return 0
}
claim_leg 'stop-window probe: spawned once, by the pass' swp_477_wiring
