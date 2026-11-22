#!/usr/bin/env bash
# shellcheck shell=bash disable=SC2016
#
# factory/stop-window-probe.sh — the attribution the journal cannot make (#477).
#
# The class. `REAP-HELD … how=unexplained` is a stop hold with a duration and no subject: the pass
# printed its last line, the unit stayed ACTIVE, and systemd wrote neither `State 'stop-sigterm'
# timed out. Killing.` (which the reap calls `deadline`) nor `with SIGKILL on client request.`
# (which it calls `client`). The retained supervisor journal holds one such row, `2026-11-19T17:11:09Z
# REAP-HELD hold=44s how=unexplained ended=624s open=1`, and every other reap on 19 Nov reads
# `REAP-CLEAN hold=0s how=clean`; #477 filed it, and `H-0312`'s `Follow-up:` named the gap when the
# #390 fence merged ("the fence removes the most likely subject of `H-0123`'s `how=unexplained` rows
# without closing that standing question: a 44 s stop with nothing in the journal is still a 44 s the
# timer cannot re-arm through"). The likeliest subject — the Wine family a tick used to spawn — is a
# hypothesis on a pre-fence sighting: `install-timer.sh`'s own comment records `winedevice.exe`
# sitting in this unit's cgroup with the main process gone on 8 Nov, and a `SIGTERM` that is ignored
# without escalating is exactly what `unexplained` looks like. A recurrence needs a subject, and the
# fence means the old answer may no longer be it.
#
# Why the read has to be standing there. The kernel forgets which processes were in a cgroup when
# the cgroup is gone, and journald forgets the window on its own schedule: re-reading the 19 Nov stop
# on this box,
#   journalctl --user -u parental-lockout-factory.service \
#       --since 2026-11-19T17:09:00 --until 2026-11-19T17:12:00
# answers `-- No entries --` (measured 22 Nov, and the oldest row the user journal then held was
# 2026-11-20T04:59). So the instrument cannot be pointed at the stop afterwards; it has to be in the
# cgroup while the stop happens.
#
# What "in the cgroup" means here, and why there is only one way to be in it. The stop path is
# `KillMode=control-group`, which `install-timer.sh` spells out beside the value it sets: "SIGTERM to
# the whole cgroup, SIGKILL when this expires, and the unit is not `inactive` until that is done --
# and the timer's `OnUnitInactiveSec=`, the setting that re-arms the queue, starts its interval at
# that moment" (`TimeoutStopSec=60`, `STOP_TIMEOUT` at `install-timer.sh:131`). So everything holding
# the stop is, at that instant, still a member of `cgroup.procs` — a flat pid list, which is the
# kernel's own answer to "who is late", with no walker and no depth cap. A member of that cgroup is
# only ever reached by being spawned inside it: the spawn in `factory/factory.sh` is therefore the
# instrument's whole footprint, one background line before `factory_body`, and the record is written
# by the SIGTERM handler on the way out.
#
# The hazard this class watches is an instrument that becomes the hold. This script is now a member
# of the cgroup it samples, so every choice is made against that:
#   - stdio is closed to `/dev/null` before anything else. A line this script wrote to the unit's
#     stdout would be a process line of the unit, which is exactly what `factory_reap_lastword`
#     reads back as the pass's last word (its comment is the measurement: `-t systemd` excludes the
#     unit's own output, so what survives is the unit's lines) — an instrument that writes becomes
#     the start of the next hold and rewrites the number it is watching;
#   - nothing is resolved at death: the unit name, the cgroup path, the record path and the
#     directory all come from the spawn, while the live pass can still answer for them;
#   - the handler forks nothing and calls nothing remote — a few small `/sys` and `/proc` reads, one
#     tmp write and one rename — so its worst-case share of its own death is a disk flush;
#   - it answers TERM, INT and HUP by writing and exiting, and keeps itself alive with a `sleep` it
#     `wait`s on, which a signal interrupts in milliseconds rather than at the end of an interval;
#   - it is disposable. Killing it costs only the attribution: the next `REAP-HELD` degrades to
#     `stop=none` or `stop=unwritten` and says which.
#
# Where it refuses to stand. `/proc/self/cgroup` must end in the unit name handed on argv, or the
# script exits without a word. A `factory.sh` run from a terminal would otherwise snapshot the
# invoking session's cgroup, and the next pass would attribute somebody's login shell to the
# supervisor's teardown — a wrong subject is worse than none, and `factory_reap_stop_window`
# re-checks the `unit=` field for the same reason. A manual pass outside the unit is therefore
# uninstrumented: its stop is neither held nor written down, and no field here pretends otherwise.
#
# What a record claims, and what it does not. It claims these processes were standing in the cgroup
# at the instant the handler ran, which for a stop that is the whole question; it does not claim they
# held this particular stop, which is why the reader matches the stamp into the reap's own window
# rather than trusting proximity. `sample=no` is the unread arm, and `stop=0` is its own fact: the
# subject died before the sample or was never in the cgroup. A probe killed by SIGKILL before its
# handler ran leaves only the `alive-<pid>` marker, which is what `stop=unwritten` reads.
#
# The contract with `factory_reap_stop_window` — one record per stop, written tmp+rename so a kill
# mid-write cannot leave a half record to chew:
#   unit=<name>        the unit this cgroup was (the reader refuses a record for another unit)
#   t=<epoch µs>       when the handler ran (the reader matches it into the activation's window and
#                      takes the newest match); a `date` without `%N` gets whole seconds, padded
#   probe=<pid>        this process           sleeper=<pid>   its sleep child — both filtered from
#                      the sample, because they are the observer
#   sample=no          only when `cgroup.procs` could not be read at all
#   proc <pid> <comm>  one line per other process standing there
# `alive-<pid>` marks an armed probe and is removed by the handler that writes; the sweep that
# retires day-old records and stray markers runs in the spawn, where forking is free — never in the
# handler. `-mmin` arithmetic rather than `-mtime +1`, because `factory/retention-census.sh:36`
# records that `-mtime +N` truncates the age and "IS NOT AN N-DAY PREDICATE".

set -u

swp_unit=${1:-}
swp_dir=${2:-}
[ -n "$swp_unit" ] || exit 0
[ -n "$swp_dir" ] || exit 0

# Before anything else, for the lastword reason in the header: nothing this script does may reach the
# unit's stdout or stderr.
exec <"/dev/null" >"/dev/null" 2>&1

# `0::/<path>` on a unified-controller v2 box. The two seams are for the selftest — a leg cannot move
# itself into a systemd cgroup, and a fixture's `cgroup.procs` is not a fixture's — and production
# leaves both unset.
swp_cgpath=${FACTORY_STOP_WINDOW_CGPATH:-}
if [ -z "$swp_cgpath" ]; then
	swp_cgpath=$(awk -F'::' '$1 == "0" { print $2; exit }' /proc/self/cgroup 2>/dev/null)
fi
[ -n "$swp_cgpath" ] || exit 0
case $swp_cgpath in
*"/$swp_unit") : ;;
*) exit 0 ;;
esac
swp_procs=${FACTORY_STOP_WINDOW_PROCS:-/sys/fs/cgroup$swp_cgpath/cgroup.procs}

mkdir -p -- "$swp_dir" 2>/dev/null || exit 0
# The directory's whole housekeeping, here rather than in the handler.
find "$swp_dir" -maxdepth 1 -type f \( -name 'stop-*' -o -name 'alive-*' \) -mmin +1440 \
	-delete 2>/dev/null || true
find "$swp_dir" -maxdepth 1 -type f -name 'stop-*.tmp' -mmin +60 -delete 2>/dev/null || true
swp_rec=$swp_dir/stop-$(date +%s)-$$
swp_alive=$swp_dir/alive-$$
swp_sleeper=''

swp_stop() {
	local swp_now='' swp_p='' swp_comm='' swp_list=''
	swp_now=$(date +%s%N 2>/dev/null)
	# One spelling of the clock for the reader: microseconds since the epoch. `date +%s%N` is not one
	# stamp but three -- GNU prints nine `%N` digits, a `date` without `%N` support prints ten, and a
	# busybox-style build prints thirteen -- and the reader matches this number against the journal's
	# own stamps, so the exponent is the contract, not the digits. Measured on this box the first time
	# the handler ran: the naive "keep 13" produced `t=1795308504447`, which is 1970 read as
	# microseconds -- a record that can never fall inside a window, and `stop=none` forever after.
	case $swp_now in
	???????????????????) swp_now=${swp_now:0:16} ;;   # s + 9 ns: drop the last three
	????????????????) : ;;                            # already microseconds
	?????????????) swp_now=${swp_now}000 ;;           # s + ms
	??????????) swp_now=${swp_now}000000 ;;           # whole seconds
	*) swp_now='' ;;                                  # no stamp, and no guess
	esac
	{
		printf 'unit=%s\n' "$swp_unit"
		printf 't=%s\n' "$swp_now"
		printf 'probe=%s\n' "$$"
		[ -n "$swp_sleeper" ] && printf 'sleeper=%s\n' "$swp_sleeper"
		if swp_list=$(cat -- "$swp_procs" 2>/dev/null); then
			for swp_p in $swp_list; do
				[ "$swp_p" = "$$" ] && continue
				[ "$swp_p" = "$swp_sleeper" ] && continue
				swp_comm=$(cat -- "/proc/$swp_p/comm" 2>/dev/null)
				printf 'proc %s %s\n' "$swp_p" "${swp_comm:-?}"
			done
		else
			printf 'sample=no\n'
		fi
	} >"$swp_rec.tmp" 2>/dev/null
	if mv -f -- "$swp_rec.tmp" "$swp_rec" 2>/dev/null; then
		rm -f -- "$swp_alive" 2>/dev/null
		exit 0
	fi
	rm -f -- "$swp_rec.tmp" 2>/dev/null
	exit 1
}
trap 'swp_stop' TERM INT HUP

# The marker says the handler is armed; a caller that must not race the install waits for this file.
: >"$swp_alive" 2>/dev/null
while :; do
	sleep 86400 &
	swp_sleeper=$!
	wait "$swp_sleeper"
done
