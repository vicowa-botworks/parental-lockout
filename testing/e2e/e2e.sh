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

# The locker rule and the loginctl reading live in one file, sourced by both scripts here.
# See testing/e2e/lib.sh for why they are a copy of internal/platform/locker.go and why the
# harness keeps its own copy rather than asking the daemon.
# The directive path is relative to where CI runs shellcheck (the repository root),
# which is the only place the file can be found without a --source-path flag.
# shellcheck source=testing/e2e/lib.sh
. "$HERE/lib.sh"

case ${1:-} in
-h | --help)
	awk 'NR>1 { if (!/^#/) exit; sub(/^# ?/, ""); print }' "$0"
	exit 0
	;;
esac

# The desktop user, unless named on the command line. Deliberately not keyed to a tty number:
# tty2 is this machine's layout, and a box whose graphical seat is tty1 would have found no
# target at all. What identifies the session is class=user with a tty attached to it -- and
# loginctl writes a literal "-" where a tty would be, so "has a tty" is two tests, not one.
TARGET=${1:-$(loginctl list-sessions --no-legend 2>/dev/null |
	awk '$6=="user" && $7!="" && $7!="-"{print $3; exit}')}

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

# locker_pid, trusted_exe and session_of come from lib.sh. The rule they implement is
# internal/platform/locker.go's, and lib.sh says why the harness keeps its own copy of it
# instead of asking the daemon: a second signal read out of the code under test is the code
# under test agreeing with itself.
#
# Every call below passes the target's uid. That is not a refinement -- without it a root
# harness walks every process on the machine and a locker belonging to a different account
# satisfies the check, which on the two-account machine this tool is for means confirming one
# child's lockout and attributing it to another.

# has_tty is whether there is somebody here to be asked. Step 5 needs a human; if this is running
# over ssh-without-tty or from a runner, the honest answer is "that step did not happen".
has_tty() { (exec </dev/tty) 2>/dev/null; }

# ---------------------------------------------------------------- preflight
[ "$(id -u)" = 0 ] || { echo "run me as root: sudo $0" >&2; exit 1; }
[ -x "$PLOCK" ] || { echo "no binary at $PLOCK -- run 'make build' first" >&2; exit 1; }
[ -n "$TARGET" ] || { echo "could not work out the desktop user; pass it as an argument" >&2; exit 1; }
TARGET_UID=$(id -u "$TARGET" 2>/dev/null) || { echo "no such user: $TARGET" >&2; exit 1; }

# Whether /etc/plock existed is recorded here, at the moment it is true. cleanup used to infer
# it from the backup directory, and `set -uo pipefail` has no -e, so a failed cp -a printed
# "backed up the existing /etc/plock" and then cleanup's else branch deleted the operator's
# real config while logging that there had never been one.
HAD_CONFDIR=no
[ -d "$CONFDIR" ] && HAD_CONFDIR=yes

mkdir -p "$RUNDIR" || {
	echo "cannot create $RUNDIR" >&2
	exit 1
}
: >"$TRANSCRIPT" || {
	echo "cannot write $TRANSCRIPT" >&2
	exit 1
}
if [ "$HAD_CONFDIR" = yes ]; then
	if ! cp -a "$CONFDIR" "$BACKUP"; then
		# Refusing here is the point. Without a verified copy there is nothing to restore,
		# and the only honest thing left is to leave the real config alone -- not to
		# "restore" it by removing it, and not to restore a half-written backup over it.
		say "the backup of $CONFDIR failed; refusing to touch it"
		say "  nothing was changed. Fix the copy (space, or a path unreadable to root) and re-run."
		exit 1
	fi
	say "backed up the existing $CONFDIR to $BACKUP"
fi

cleanup() {
	# INT runs cleanup and the EXIT trap then runs it again on the way out. Guarded because
	# the second pass would report a restore that never happened, in the transcript that is
	# the deliverable.
	[ -z "${CLEANED:-}" ] || return 0
	CLEANED=yes
	say "--- cleaning up"
	if [ -n "${DAEMON_PID:-}" ]; then
		kill "$DAEMON_PID" 2>/dev/null
		sleep 1
		kill -9 "$DAEMON_PID" 2>/dev/null
	fi
	if [ "$HAD_CONFDIR" = yes ]; then
		if [ -d "$BACKUP" ]; then
			rm -rf "$CONFDIR"
			if cp -a "$BACKUP" "$CONFDIR"; then
				say "restored the previous $CONFDIR"
			else
				say "WARNING: the restore FAILED. $CONFDIR holds the harness's config."
				say "  Recover it by hand: rm -rf $CONFDIR && cp -a $BACKUP $CONFDIR"
			fi
		else
			# Nothing to restore from, so nothing gets removed. The harness's own config
			# stays where it is and the operator is told, rather than a deletion being
			# reported as a restore.
			say "WARNING: no backup at $BACKUP; leaving $CONFDIR exactly as it is"
		fi
	else
		rm -rf "$CONFDIR"
		say "removed $CONFDIR (there was none before)"
	fi
	say "left $STATEDIR and $AUDIT in place for inspection; remove with:"
	say "  sudo rm -rf $STATEDIR /var/log/plock"
	say "--- done. transcript: $TRANSCRIPT"
}
trap cleanup EXIT
# `trap cleanup INT` ran cleanup and then RETURNED: Ctrl-C killed the daemon inside cleanup,
# the script carried on through steps 4-8 against a dead daemon, printed a verdict about it,
# and ran cleanup a second time at exit. Stopping is what Ctrl-C means.
trap 'cleanup; exit 130' INT TERM

SESS=$(session_of "$TARGET")
[ -n "$SESS" ] || {
	echo "$TARGET has no graphical session (class=user with a tty). Log them in, or name the" >&2
	echo "right account: sudo $0 <username>" >&2
	exit 1
}

say "=== plock end-to-end against the real session"
say "binary   : $PLOCK"
say "target   : $TARGET (uid $TARGET_UID) session $SESS"
say "desktop  : $(loginctl show-session "$SESS" -p TTY -p Type -p State -p Active --value 2>/dev/null | paste -sd' ' -)"
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
	hint=$(loginctl show-session "$SESS" -p LockedHint --value 2>/dev/null)
	locker=$(locker_pid "$TARGET_UID")
	if [ "$hint" = "yes" ] && [ -n "$locker" ]; then
		locked=yes
		say "CONFIRMED at $((i * 2))s: LockedHint=yes and the locker is running (pid $locker)"
		say "  accepted by: $(locker_rule "$(exe_name "$locker")")"
		break
	fi
	[ $((i % 5)) -eq 0 ] && say "  ...$((i * 2))s: LockedHint=${hint:-?} locker=${locker:-none}"
done
[ -n "$locked" ] || say "NOT CONFIRMED after 48s -- LockedHint=${hint:-?} locker=${locker:-none}"

say
say "=== 4. what the machine looks like while locked"
run "loginctl show-session $SESS -p LockedHint -p Type -p State -p Active"
if [ -n "$locker" ]; then
	run "ps -o pid,ppid,uid,comm,args -p $locker"
	# The pid is gone by the time anyone reads the transcript, and the claim that made it a
	# locker is about a file on disk, so the file and its ownership go in the record.
	while IFS= read -r line; do say "$line"; done < <(describe_exe "$locker")
else
	say "no locker process to describe -- the lock did not take, or it took on a backend this"
	say "host does not recognise; add its executable to locker_pid() in lib.sh rather than trusting the hint"
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
		hint=$(loginctl show-session "$SESS" -p LockedHint --value 2>/dev/null)
		locker=$(locker_pid "$TARGET_UID")
		if [ "$hint" = "yes" ] && [ -n "$locker" ]; then
			relocked=yes
			say "RE-LOCKED at $((i * 2))s after the unlock: hint=yes and the locker is running (pid $locker)"
			say "  accepted by: $(locker_rule "$(exe_name "$locker")")"
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
