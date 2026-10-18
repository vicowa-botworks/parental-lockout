#!/usr/bin/env bash
# e2e.sh -- run the whole enforcement loop against the real desktop session on this machine.
#
# Why this exists: `docs/security.md` §9 ranks this run as the project's largest residual risk,
# and `docs/PLAN.md` §13 (issue #13) asked for it by name. Every other check in the repo is a unit
# test running against fixtures for the bus and /proc -- including the ones security.md cites when
# it says a lockout is enforced. Nothing here substitutes for a real Plasma session, which is why
# this lives in `testing/` rather than in CI: a runner has no seat, no logind, and no operator to
# type a password.
#
# What it costs, stated up front because the script cannot be run casually: it locks YOUR screen
# on purpose, several times, and you will need your password to get back in. It always cleans up,
# including on Ctrl-C, and it restores /etc/plock to whatever was there before.
#
#   make build                      # leaves ./plock at the repo root
#   sudo testing/e2e/e2e.sh         # target defaults to the desktop user
#   sudo testing/e2e/e2e.sh casey   # or name it
#   PLOCK=/path/to/plock sudo -E testing/e2e/e2e.sh
#
# sudo drops the environment unless you give it -E, which matters only for PLOCK and RUNDIR.
#
# Output: testing/e2e/runs/<timestamp>/transcript.txt -- that file is the deliverable, and it is
# what issue #13's first acceptance criterion asks to be pasted into the PR.

set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
REPO=$(cd "$HERE/../.." && pwd)
PLOCK=${PLOCK:-$REPO/plock}

case ${1:-} in
-h | --help)
	awk 'NR>1 { if (!/^#/) exit; sub(/^# ?/, ""); print }' "$0"
	exit 0
	;;
esac

# The desktop user, unless named on the command line. Deliberately not keyed to a tty number:
# tty2 is this machine's layout, and a box whose graphical seat is tty1 would have found no
# target at all. What identifies the session is class=user with a tty attached to it.
TARGET=${1:-$(loginctl list-sessions --no-legend 2>/dev/null | awk '$6=="user" && $7!=""{print $3; exit}')}

CONFDIR=/etc/plock
STATEDIR=/var/lib/plock
AUDIT=/var/log/plock/audit.jsonl

# Everything the run writes goes in one timestamped directory under the script, so two runs never
# overwrite each other's evidence and nobody has to remember where the last one went. The
# hardcoded /tmp paths this replaced are how a transcript written on one machine came to be read
# as the project's e2e result.
RUNDIR=${RUNDIR:-$HERE/runs/$(date +%Y%m%d-%H%M%S)}
TRANSCRIPT=${TRANSCRIPT:-$RUNDIR/transcript.txt}
DAEMONLOG=$RUNDIR/daemon.log
BACKUP=$RUNDIR/backup

say() { printf '%s %s\n' "$(date +%H:%M:%S)" "$*" | tee -a "$TRANSCRIPT"; }

# run records the command and its output together. It hands the string to a fresh shell rather
# than eval'ing it in this one, so a probe that backgrounds or redirects cannot disturb the
# harness's own state.
run() {
	printf '\n$ %s\n' "$*" >>"$TRANSCRIPT"
	bash -c "$*" >>"$TRANSCRIPT" 2>&1
}

# locker_pid prints the pid of a running screen locker, and nothing if there is none.
#
# Why not pgrep: the obvious `pgrep -f 'kscreenlocker_greet|gsd-screensaver'` matches command
# LINES, so any shell whose argv merely contains that pattern -- a wrapper, a `bash -c`, an editor
# with the string in its buffer -- reports a locker that does not exist. Measured on the
# development host: it returned two matches, both of them shells. The alternative, `pgrep -x
# kscreenlocker_greet`, can never match at all: it compares against comm, which the kernel holds to
# 15 characters and the binary's name is 19, and pgrep says so itself ("pattern that searches for
# process name longer than 15 characters will result in zero matches"). `ps -C` does match -- the
# 2026-10-15 transcript shows it listing the locker -- but it answers "is one of these running"
# with a table, and the harness needs the pid it is about to describe.
#
# This second signal exists precisely because LockedHint is writable by the session user (#29), so
# a matcher that can be fed a string is one step from the thing it was written to guard against.
# /proc/<pid>/exe is the executable the kernel actually started, and nothing a user types into a
# command line can make it point at a locker that is not running.
locker_pid() {
	local p exe
	for p in /proc/[0-9]*; do
		exe=$(readlink -- "$p/exe" 2>/dev/null) || continue
		case ${exe##*/} in
		kscreenlocker_greet | gsd-screensaver)
			basename -- "$p"
			return 0
			;;
		esac
	done
	return 1
}

# The target's desktop session id. Inlined four times before; the re-lock check needs it as
# well, and a fifth copy of an awk over loginctl is how two of them end up disagreeing.
session_id() {
	loginctl list-sessions --no-legend 2>/dev/null |
		awk -v u="$(id -u "$TARGET")" '$2==u && $6=="user" && $7!="" {print $1; exit}'
}

# has_tty is whether there is somebody here to be asked. Step 5 needs a human; if this is running
# over ssh-without-tty or from a runner, the honest answer is "that step did not happen".
has_tty() { (exec </dev/tty) 2>/dev/null; }

# ---------------------------------------------------------------- preflight
[ "$(id -u)" = 0 ] || { echo "run me as root: sudo $0" >&2; exit 1; }
[ -x "$PLOCK" ] || { echo "no binary at $PLOCK -- run 'make build' first" >&2; exit 1; }
[ -n "$TARGET" ] || { echo "could not work out the desktop user; pass it as an argument" >&2; exit 1; }

mkdir -p "$RUNDIR"
: >"$TRANSCRIPT"
if [ -d "$CONFDIR" ]; then
	cp -a "$CONFDIR" "$BACKUP"
	say "backed up the existing $CONFDIR to $BACKUP"
fi

cleanup() {
	say "--- cleaning up"
	if [ -n "${DAEMON_PID:-}" ]; then
		kill "$DAEMON_PID" 2>/dev/null
		sleep 1
		kill -9 "$DAEMON_PID" 2>/dev/null
	fi
	if [ -d "$BACKUP" ]; then
		rm -rf "$CONFDIR"
		cp -a "$BACKUP" "$CONFDIR"
		say "restored the previous $CONFDIR"
	else
		rm -rf "$CONFDIR"
		say "removed $CONFDIR (there was none before)"
	fi
	say "left $STATEDIR and $AUDIT in place for inspection; remove with:"
	say "  sudo rm -rf $STATEDIR /var/log/plock"
	say "--- done. transcript: $TRANSCRIPT"
}
trap cleanup EXIT INT TERM

say "=== plock end-to-end against the real session"
say "binary   : $PLOCK"
say "target   : $TARGET (uid $(id -u "$TARGET"))"
say "desktop  : $(loginctl list-sessions --no-legend | awk -v u="$(id -u "$TARGET")" '$2==u && $6=="user"{print $7, $8}' | head -1)"
say "run dir  : $RUNDIR"
run "$PLOCK version"

say
say "=== 1. doctor, before any configuration exists"
run "$PLOCK doctor"

# ---------------------------------------------------------------- configure
say
say "=== 2. init, then a profile that is enforcing right now, all day"
# An all-day rule so the lockout is active whenever the script runs; a narrower window would
# make the result depend on the time of day, which is the opposite of a regression test.
printf 'e2e-not-a-real-password-please-change\n' |
	"$PLOCK" init --username e2e-admin --password-stdin --force >>"$TRANSCRIPT" 2>&1 || {
	say "init failed"
	tail -20 "$TRANSCRIPT"
	exit 1
}

python3 - "$CONFDIR/config.json" "$TARGET" <<'PY' >>"$TRANSCRIPT" 2>&1
import json, sys
path, user = sys.argv[1], sys.argv[2]
d = json.load(open(path))
d["settings"] = {"pollSeconds": 5, "enforcement": True,
                 "lockBackends": ["logind", "kde", "gnome"]}
d["profiles"] = [{"id": "e2e", "name": "e2e", "timezone": "UTC", "rules": [
    {"id": "r1", "name": "always", "days": ["mon","tue","wed","thu","fri","sat","sun"],
     "allDay": True, "enabled": True}]}]
d["users"] = [{"username": user, "profileID": "e2e", "enabled": True}]
json.dump(d, open(path, "w"), indent=2)
print("wrote config for", user)
PY
run "ls -l $CONFDIR/"
run "$PLOCK status --json"
run "$PLOCK doctor"

# ---------------------------------------------------------------- the run
say
say "=== 3. starting the daemon. YOUR SCREEN WILL LOCK. Have your password ready."
say "    (the poll is 5s in this config, so the lock is re-asserted about every 5s until you"
say "     unlock -- that re-assertion is the thing being tested)"
"$PLOCK" serve --config "$CONFDIR/config.json" --state-dir "$STATEDIR" >"$DAEMONLOG" 2>&1 &
DAEMON_PID=$!
say "daemon pid $DAEMON_PID"

locked=""
hint=""
locker=""
for i in $(seq 1 24); do
	sleep 2
	hint=$(loginctl show-session "$(session_id)" -p LockedHint --value 2>/dev/null)
	locker=$(locker_pid)
	if [ "$hint" = "yes" ] && [ -n "$locker" ]; then
		locked=yes
		say "CONFIRMED at $((i * 2))s: LockedHint=yes and the locker is running (pid $locker)"
		break
	fi
	[ $((i % 5)) -eq 0 ] && say "  ...$((i * 2))s: LockedHint=${hint:-?} locker=${locker:-none}"
done
[ -n "$locked" ] || say "NOT CONFIRMED after 48s -- LockedHint=${hint:-?} locker=${locker:-none}"

say
say "=== 4. what the machine looks like while locked"
run "loginctl show-session $(session_id) -p LockedHint -p Type -p State -p Active"
if [ -n "$locker" ]; then
	run "ps -o pid,ppid,uid,comm,args -p $locker"
else
	say "no locker process to describe -- the lock did not take, or it took on a backend this"
	say "host does not recognise; add its executable to locker_pid() rather than trusting the hint"
fi
run "$PLOCK status --json"
run "ls -l /var/log/plock/ 2>&1"

say
relocked=""
if has_tty; then
	say "=== 5. UNLOCK NOW with your password, then press Enter here."
	say "    (the daemon is still running, so it will lock again within one poll -- 5s in this"
	say "     config -- and that is the point)"
	read -r </dev/tty || true

	say "waiting to see whether the daemon re-locks a session that was just unlocked..."
	say "    (both signals again, deliberately: LockedHint is writable by the session user with no"
	say "     privilege, so a re-lock judged on the hint alone can be pronounced on a screen that"
	say "     never locked. That is the bug #29 is about, and a test trusting the hint cannot see it.)"
	for i in $(seq 1 20); do
		sleep 2
		hint=$(loginctl show-session "$(session_id)" -p LockedHint --value 2>/dev/null)
		locker=$(locker_pid)
		if [ "$hint" = "yes" ] && [ -n "$locker" ]; then
			relocked=yes
			say "RE-LOCKED at $((i * 2))s after the unlock: hint=yes and the locker is running (pid $locker)"
			break
		fi
		if [ "$hint" = "yes" ] && [ -z "$locker" ]; then
			say "  ...$((i * 2))s: hint=yes but NO locker -- a forged hint, not a lock"
		fi
	done
	[ -n "$relocked" ] || say "did NOT re-lock (both signals) within 40s"
else
	# No terminal to wait at. Saying so is the only honest option: a transcript that quietly
	# skipped the unlock reads as "it did not re-lock", which is a different finding and a worse
	# one, and the verdict below would report it as a PARTIAL failure of the daemon.
	relocked=skipped
	say "=== 5. SKIPPED: no controlling terminal, so nobody could be asked to unlock."
	say "    The re-lock half of this run has NOT been tested. Run this from a real tty."
fi

# ---------------------------------------------------------------- the trail
say
say "=== 6. the audit trail -- the operator's only evidence"
run "wc -l $AUDIT 2>&1"
run "tail -12 $AUDIT 2>&1"
# There is no `plock audit` command yet: #47 lands the reader as a library, and the CLI surface
# is still open. Reading the file directly is the honest version of this step, and the gap is
# recorded in docs/security.md's "Not defended" table.
run "python3 -c \"import json,sys,collections; c=collections.Counter(json.loads(l)['kind'] for l in open('$AUDIT')); print(dict(c))\" 2>&1"

say
say "=== 7. the daemon's own log"
printf '\n'
cat "$DAEMONLOG" >>"$TRANSCRIPT"
tail -25 "$DAEMONLOG" | sed 's/^/    /'

say
say "=== 8. verdict"
if [ "$relocked" = skipped ]; then
	say "INCONCLUSIVE: it locked and confirmed, but the re-lock step was skipped for want of a terminal."
elif [ -n "$locked" ] && [ -n "$relocked" ]; then
	say "PASS: the screen locked, was confirmed by both signals, and re-locked after the unlock."
elif [ -n "$locked" ]; then
	say "PARTIAL: it locked and confirmed, but did not re-lock within 40s."
else
	say "FAIL: the screen never reached LockedHint=yes with the locker running."
fi
say "The transcript is the deliverable: $TRANSCRIPT"
