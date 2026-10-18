#!/usr/bin/env bash
# forge-hint.sh -- prove that a lock cannot be judged from logind's LockedHint.
#
# Why this exists: issue #29 measured that the controlled user can write their own session's
# LockedHint with no privilege at all -- no root, no polkit prompt, one D-Bus call from an
# autostart file. The enforcement loop therefore re-asserts the lock on every tick of an open
# window regardless of what the session reports (internal/enforce), and the e2e harness judges a
# lock on two signals rather than one. Both of those decisions rest on a fact about systemd-logind
# that no test in this repo can re-check, because it is a property of the host, not of this code:
# the hint is writable by the session user. This script re-checks it.
#
# Two modes, and the first one is the one to run:
#
#   testing/e2e/forge-hint.sh              # as the desktop user. Locks nothing, needs no daemon.
#   sudo testing/e2e/forge-hint.sh casey   # as root, with the daemon enforcing.
#
# Unprivileged mode flips your own session's hint, reads it back, shows that no locker is running
# while it says "locked", and puts it back. It proves the attack surface. It does not lock the
# screen and it does not need the daemon.
#
# Root mode proves the defense: with the daemon enforcing on a target account, it forges the hint
# as that user and insists the loop keeps writing lock.asserted while the hint claims the session
# is already locked. A loop that believed the hint would go quiet here -- which is exactly the
# one-line-autostart defeat #29 is about. This mode DOES lock the target's screen, because the
# daemon is doing its job.
#
# Exit status: 0 the claim held, 1 it did not, 2 the host is not in a state to answer.

set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
# locker_pid, daemon_pid and session_of are shared with e2e.sh; see testing/e2e/lib.sh for
# the rule they implement and why it is a copy of internal/platform/locker.go's.
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

AUDIT=${AUDIT:-/var/log/plock/audit.jsonl}
WAIT=${WAIT:-20} # root mode: how long to watch for assertions; give it at least two poll intervals

say() { printf '%s %s\n' "$(date +%H:%M:%S)" "$*"; }

die() {
	say "cannot answer: $*" >&2
	exit 2
}

# object_path maps a session id to the logind object path that actually answers on it. Asked of
# logind rather than guessed from the id, because the path is not the id: for the session loginctl
# calls "3" this host answers on /org/freedesktop/login1/session/_33, while the form the
# documentation leads with is _3 and a plain /session/3 also answers. Candidates are tried against
# a READ of the property and the first one that answers is the one the write goes to -- writing to a
# path nobody owns is not evidence about who may write it, and the first run of this script proved
# that by reading a refusal as "the hint is no longer forgeable".
object_path() {
	local id=$1 cand got
	got=$(busctl call org.freedesktop.login1 /org/freedesktop/login1 \
		org.freedesktop.login1.Manager GetSession s "$id" 2>/dev/null |
		tr -d '"' | awk '{print $NF}')
	for cand in "$got" "/org/freedesktop/login1/session/$id" "/org/freedesktop/login1/session/_$id"; do
		case ${cand:-} in
		/org/freedesktop/login1/session/*) ;;
		*) continue ;;
		esac
		if busctl get-property org.freedesktop.login1 "$cand" \
			org.freedesktop.login1.Session LockedHint >/dev/null 2>&1; then
			printf '%s\n' "$cand"
			return 0
		fi
	done
	return 1
}

hint_of() { loginctl show-session "$1" -p LockedHint --value 2>/dev/null; }

# set_hint writes the hint as the session's own user. That is the point: nothing here escalates.
# Root running the check for somebody else drops to that uid first, so the call carries the child's
# credentials and not the daemon's.
set_hint() { # $1 session  $2 true|false  $3 uid-to-run-as (optional)
	local uid=${3:-}
	if [ -n "$uid" ] && [ "$uid" != "$(id -u)" ]; then
		# env, not sudo's own VAR=value form: sudo refuses to set an arbitrary variable unless
		# sudoers allows it, and a refusal here would read as "the hint is not forgeable".
		sudo -u "#$uid" env "XDG_RUNTIME_DIR=/run/user/$uid" \
			busctl call org.freedesktop.login1 "$1" org.freedesktop.login1.Session SetLockedHint b "$2"
	else
		busctl call org.freedesktop.login1 "$1" org.freedesktop.login1.Session SetLockedHint b "$2"
	fi
}

# asserted counts the loop's own lock.asserted lines for one account, so the root mode is measuring
# what the operator would read rather than a process list.
asserted() {
	python3 - "$AUDIT" "$1" <<'PY' 2>/dev/null
import json, sys
path, user = sys.argv[1], sys.argv[2]
n = 0
try:
    with open(path) as fh:
        for line in fh:
            try:
                r = json.loads(line)
            except ValueError:
                continue
            if r.get("kind") == "lock.asserted" and r.get("username") == user:
                n += 1
except OSError:
    sys.exit(3)
print(n)
PY
}

# restore_hint puts the session's hint back, once, however this script stops.
#
# testing/README.md states the house rule -- "both scripts back up before they write and
# restore on exit, including on Ctrl-C" -- and there was no trap here at all, which made that
# sentence untrue in exactly the window where it matters. Root mode forges the target's hint
# and then sleeps WAIT seconds (20 by default) before restoring it: Ctrl-C in that window left
# the child's session reading LockedHint=yes with nothing printed about it. Unprivileged mode
# has the same window between its forge and its restore.
FORGE_PATH=""    # the logind object path that was written
FORGE_UID=""     # the uid to write as; empty means whoever is running this
FORGE_RESTORE="" # what the hint read before
FORGE_DONE=""    # set once something has actually put it back

restore_hint() {
	[ -n "$FORGE_PATH" ] && [ -z "$FORGE_DONE" ] || return 0
	FORGE_DONE=yes
	if ! set_hint "$FORGE_PATH" "$FORGE_RESTORE" "$FORGE_UID" 2>/dev/null; then
		say "WARNING: the hint was NOT restored to '$FORGE_RESTORE'. Put it back as uid"
		say "  ${FORGE_UID:-$(id -u)} with:"
		say "  busctl call org.freedesktop.login1 $FORGE_PATH org.freedesktop.login1.Session SetLockedHint b $FORGE_RESTORE"
	fi
}
trap restore_hint EXIT
# The INT handler exits: a trap that only ran restore_hint would let the script carry on
# forging, which is the opposite of what Ctrl-C was pressed for.
trap 'restore_hint; exit 130' INT TERM

# ---------------------------------------------------------------- unprivileged mode
if [ "$(id -u)" != 0 ]; then
	sess=$(session_of)
	[ -n "$sess" ] || die "no desktop session for uid $(id -u); this has to run from the graphical login"
	path=$(object_path "$sess")
	[ -n "$path" ] || die "logind would not give an object path for session $sess"

	before=$(hint_of "$sess")
	say "session $sess ($path), uid $(id -u) ($(id -un)), hint starts at '$before'"
	if [ "$before" = "yes" ] && [ -z "$(locker_pid "$(id -u)")" ]; then
		# Worth saying out loud: this is the state the script goes on to manufacture on purpose,
		# and it is already true here. Restoring what was found is still the right contract -- the
		# operator may have a reason -- but a stale 'yes' is worth clearing deliberately.
		say "note: it already reads 'locked' with no locker running. This script puts back what it"
		say "  found rather than deciding the state is wrong; clear a stale one with"
		say "  loginctl unlock-session $sess"
	fi

	# Flip it, whatever it is. Writing false onto a session that is genuinely locked is the same
	# call as writing true onto one that is not, and it is the call that matters. Flipping back is
	# also how the session ends up holding its honest state again.
	want=true
	[ "$before" = "yes" ] && want=false
	FORGE_PATH=$path
	FORGE_RESTORE=$([ "$before" = "yes" ] && echo true || echo false)
	if err=$(set_hint "$path" "$want" 2>&1); then
		mid=$(hint_of "$sess")
		if { [ "$want" = true ] && [ "$mid" = "yes" ]; } ||
			{ [ "$want" = false ] && [ "$mid" = "no" ]; }; then
			forge=yes
		else
			forge=""
		fi
	else
		# The call did not succeed. Only an authorization refusal is the finding; anything else
		# -- an object path nobody owns, no bus, a sudo that will not carry the environment --
		# means the question was never asked, and reporting that as "not forgeable" would be a
		# false all-clear about a host this script did not actually test.
		mid=$(hint_of "$sess")
		case $err in
		*"not authorized"* | *"Interactive authorization"* | *policykit* | *polkit*)
			say "logind refused the write: $err"
			say "VERDICT: NOT forgeable -- the write was denied to uid $(id -u)."
			say "  If logind has learned to authorise SetLockedHint, docs/security.md §5 and #29"
			say "  need re-reading: the skip rule this project refuses may no longer be the risk it was."
			exit 1
			;;
		esac
		die "the SetLockedHint call failed for a reason that is not a refusal: ${err:-no output}"
	fi

	if [ -n "$forge" ]; then
		say "FORGED: the hint now reads '$mid' and nothing but a D-Bus call by uid $(id -u) did that."
		locker=$(locker_pid "$(id -u)")
		if [ "$want" = true ] && [ -z "$locker" ]; then
			say "and no locker is running (pid list empty) while the hint says 'locked'."
			say "  -> a check that trusts LockedHint alone would report a lock on a screen that is"
			say "     unlocked. That is the #29 defeat, live on this host, from an unprivileged process."
		fi
		restore_hint
		after=$(hint_of "$sess")
		say "restored: hint reads '$after'"
		[ "$after" = "$before" ] || say "WARNING: hint did not come back to '$before'"
		say "VERDICT: CONFIRMED -- LockedHint is writable by the session user with no privilege."
		say "  The defense is that nothing decides enforcement from it (internal/enforce re-asserts"
		say "  every tick, #29). Run this as root with the daemon enforcing to check that half."
		exit 0
	fi

	restore_hint
	say "VERDICT: NOT forgeable -- the call was accepted and the hint still reads '$mid'."
	say "  If logind has learned to authorise SetLockedHint, docs/security.md and #29 need"
	say "  re-reading: the skip rule this project refuses may no longer be the risk it was."
	exit 1
fi

# ---------------------------------------------------------------- root mode
TARGET=${1:-}
[ -n "$TARGET" ] || die "as root, name the account to test: sudo $0 <username>"
id -u "$TARGET" >/dev/null 2>&1 || die "no such user: $TARGET"
sess=$(session_of "$TARGET")
[ -n "$sess" ] || die "$TARGET has no desktop session to test"
path=$(object_path "$sess")
[ -n "$path" ] || die "logind would not give an object path for session $sess"
[ -r "$AUDIT" ] || die "cannot read $AUDIT -- is the daemon running, and did testing/e2e/e2e.sh configure it?"

plock_pid=$(daemon_pid)
[ -n "$plock_pid" ] || die "no plock daemon is running; start it (or run testing/e2e/e2e.sh) first"

say "target $TARGET (uid $(id -u "$TARGET")) session $sess, daemon pid $plock_pid"
before=$(asserted "$TARGET") || die "counting $AUDIT failed"
locker=$(locker_pid "$(id -u "$TARGET")")
say "lock.asserted for $TARGET so far: ${before:-0}; locker running: ${locker:-no}"

before_hint=$(hint_of "$sess")
say "forging the hint to 'yes' as uid $(id -u "$TARGET") (it was '$before_hint')"
set_hint "$path" true "$(id -u "$TARGET")" || die "the forged call failed outright"
FORGE_PATH=$path
FORGE_UID=$(id -u "$TARGET")
FORGE_RESTORE=$([ "$before_hint" = "yes" ] && echo true || echo false)
during=$(hint_of "$sess")
[ "$during" = "yes" ] || {
	# Exit 1, not 2. The header says 2 is "the host is not in a state to answer", and a hint
	# the child could not write is not an unanswerable question -- it is the claim not
	# holding, which is the finding the whole script exists to look for.
	say "VERDICT: DID NOT HOLD -- the forged SetLockedHint did not take; the hint reads '$during'."
	say "  The defense cannot be tested on a session whose hint the child cannot write. Run the"
	say "  unprivileged mode as $TARGET to check the surface on its own."
	exit 1
}

say "watching for $WAIT s while the hint claims the session is already locked..."
sleep "$WAIT"
after=$(asserted "$TARGET")
locker=$(locker_pid "$(id -u "$TARGET")")
say "lock.asserted now: ${after:-?} (was ${before:-0}); locker running: ${locker:-no}"

restore_hint

if [ -n "$after" ] && [ "$after" -gt "${before:-0}" ]; then
	say "VERDICT: HELD -- the loop asserted the lock $((after - before)) more time(s) while the"
	say "  session's own hint said it was already locked. A hint-trusting loop would have gone quiet."
	exit 0
fi
say "VERDICT: DID NOT HOLD -- no further lock.asserted while the hint said 'locked'."
say "  Either the loop is honouring a signal the child can write, or it was not enforcing on"
say "  $TARGET at all. Check 'plock status --json' before concluding which."
exit 1
