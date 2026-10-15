#!/bin/sh
# run-dev.sh -- run the whole enforcement loop on a machine with no desktop session
# and no root.
#
# Why this needs its own backend rather than the test double: internal/platform.Mock is
# deliberately not registered under an id, because a config-selectable backend that
# reports success and locks nothing is the exact fail-open this project refuses (see
# mock.go's comment). So `make run-dev` cannot honestly point at Mock. It points at the
# `command` backend instead, which really execs a program and returns that program's exit
# status -- so the loop under test is the real loop, the real config, the real audit
# writer and the real fallback chain, and only the thing at the end that touches a screen
# is a shell script.
#
# What you get:
#
#   * a config, state directory and audit trail under one dev root, so nothing here
#     touches /etc, /var/lib or /var/log
#   * the daemon running unprivileged, because the command backend stays as whoever
#     started it rather than dropping to the session uid
#   * a lock program that appends to a log, so you can watch the loop ask
#   * the admin UI, so the web paths can be exercised against a live enforcement loop
#
# It refuses to run if the dev root already holds a config, unless --fresh. That is not
# fussiness: the audit trail and the override list are the interesting state, and a
# script that silently wiped them would be a script you stop trusting by the third run.

set -eu

BIN=${BIN:-bin/plock}
DEV_ROOT=${PLOCK_DEV_ROOT:-/tmp/plock-dev}
USERNAME=${PLOCK_DEV_USER:-dev}
LISTEN=${PLOCK_DEV_LISTEN:-127.0.0.1:8443}
POLL=${PLOCK_DEV_POLL:-5}
TARGET=${PLOCK_DEV_TARGET:-${USER:-$(id -un)}}
FRESH=0
QUIET=0

usage() {
	cat <<'EOF'
usage: deploy/run-dev.sh [--fresh] [--quiet]

Run plock serve unprivileged against a throwaway config that uses the `command`
lock backend, so the enforcement loop can be exercised with no graphical session
and no root.

  --fresh   discard an existing dev root and start over
  --quiet   do not print the dev root and credentials on startup

Environment:
  PLOCK_DEV_ROOT     where the throwaway state lives (default /tmp/plock-dev)
  PLOCK_DEV_USER     admin login name            (default dev)
  PLOCK_DEV_LISTEN   listen address              (default 127.0.0.1:8443;
                     plain HTTP -- the config sets tls: off)
  PLOCK_DEV_POLL     settings.pollSeconds        (default 5)
  PLOCK_DEV_TARGET   account to enforce          (default the invoking user)
  PLOCK_DEV_PASSWORD admin password; generated and printed if unset
  BIN               the plock binary to run      (default bin/plock)

Ctrl-C stops the daemon. Nothing outside the dev root is created or changed.
EOF
}

while [ $# -gt 0 ]; do
	case $1 in
	--fresh) FRESH=1 ;;
	--quiet) QUIET=1 ;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		printf 'run-dev: unknown argument %s\n\n' "$1" >&2
		usage >&2
		exit 2
		;;
	esac
	shift
done

# The daemon refuses a tick below config.MinPollSeconds, and it is right to. Checked here
# so the refusal names PLOCK_DEV_POLL and happens before anything is written, rather than
# arriving from `plock init` after the dev root already exists with half a config in it.
# The number lives in Go; if the two ever disagree, TestTheDevScriptsPollFloor says so.
if [ "$POLL" -lt 5 ] 2>/dev/null; then
	printf 'run-dev: PLOCK_DEV_POLL=%s is below the daemon floor of 5 seconds\n' "$POLL" >&2
	exit 2
fi
case $POLL in
'' | *[!0-9]*)
	printf 'run-dev: PLOCK_DEV_POLL=%s is not a number of seconds\n' "$POLL" >&2
	exit 2
	;;
esac

if [ ! -x "$BIN" ]; then
	printf 'run-dev: %s is not an executable; build it first with: make build\n' "$BIN" >&2
	exit 1
fi

CONFIG="$DEV_ROOT/config.json"
STATE="$DEV_ROOT/state"
AUDIT="$DEV_ROOT/audit.jsonl"
LOCKER="$DEV_ROOT/lock.sh"
LOCK_LOG="$DEV_ROOT/lock-calls.log"

if [ -f "$CONFIG" ] && [ "$FRESH" -ne 1 ]; then
	printf 'run-dev: %s already exists.\n' "$CONFIG" >&2
	printf 'run-dev: re-run with --fresh to discard it, or drop --fresh to keep it.\n' >&2
	exit 1
fi

if [ "$FRESH" -eq 1 ] && [ -d "$DEV_ROOT" ]; then
	# Only ever the dev root, and only ever this script's own files. A rm -rf on a
	# variable that a typo in PLOCK_DEV_ROOT could point somewhere else is the reason
	# this checks the marker rather than trusting the name.
	if [ -f "$DEV_ROOT/.plock-dev" ]; then
		rm -rf "$DEV_ROOT"
	else
		printf 'run-dev: %s is not a dev root (no .plock-dev); refusing to delete it\n' "$DEV_ROOT" >&2
		exit 1
	fi
fi

# The dev root holds a config with a password verifier and an audit trail, so it is
# created 0700 rather than at the umask -- /tmp is world-readable by definition, and a
# directory anyone can enter is a poor home for either.
#
# $STATE is deliberately NOT created here. `plock init` creates the state directory itself
# at 0700 and refuses one that already exists more open than that, so a mkdir here running
# at the usual 0775 umask would make init refuse to start -- which is init being right, and
# this script being wrong about who owns the directory.
# mkdir's -m is not applied to parents it creates, so the mode is set after the fact on
# the one directory this script owns. A PLOCK_DEV_ROOT two levels deep would otherwise
# leave its parent at the umask while looking, from the line above, like it had been
# tightened.
mkdir -p "$DEV_ROOT"
chmod 700 "$DEV_ROOT"
: >"$DEV_ROOT/.plock-dev"

# The lock program. It records the argv it was called with and succeeds, which is the
# minimum that makes the loop's real shape observable: how often it asks, which session
# it names, and what a non-zero exit does to the audit trail. Change `exit 0` to
# `exit 1` here to watch the unconfirmed path.
cat >"$LOCKER" <<EOF
#!/bin/sh
# Written by deploy/run-dev.sh. One line per call so \`tail -f\` reads as a tick trace.
#
# The log path is baked in by expansion at generation time, and the runtime shell
# expansions below are escaped so they survive into this file. The first version used a
# quoted heredoc, which left \\$LOCK_LOG literal in a script that had no such variable:
# the redirect failed, the script fell through to its own exit 0, and the loop reported a
# lock accepted while nothing was ever written. A locker that silently does nothing is
# precisely the thing this backend exists not to be.
{
	printf '%s' "\$(date -u +%H:%M:%S)"
	for a in "\$@"; do printf ' %s' "\$a"; done
	printf '\\n'
} >>'$LOCK_LOG'
exit 0
EOF
chmod 755 "$LOCKER"
: >>"$LOCK_LOG"

# Settings first, admin second. plock init loads an existing document and preserves it
# (loadOrStart treats only "not there" as fresh), so writing the settings here and letting
# init add the credential is the order that keeps both. The reverse would be a hand-written
# PBKDF2 verifier, which is not a thing a shell script should be doing.
cat >"$CONFIG" <<EOF
{
  "version": 1,
  "settings": {
    "listen": "$LISTEN",
    "tls": "off",
    "pollSeconds": $POLL,
    "enforcement": true,
    "lockBackends": ["command"],
    "lockCommandPath": "$LOCKER",
    "lockCommandArgv": ["--session", "%SESSION%", "--user", "%USER%", "--uid", "%UID%"]
  },
  "profiles": [
    {
      "id": "dev",
      "name": "dev loop",
      "timezone": "UTC",
      "rules": [
        {
          "id": "always",
          "days": ["mon", "tue", "wed", "thu", "fri", "sat", "sun"],
          "start": "00:00",
          "end": "23:59",
          "enabled": true
        }
      ]
    }
  ],
  "users": [
    { "username": "$TARGET", "profileID": "dev", "enabled": true }
  ],
  "overrides": []
}
EOF

if [ -z "${PLOCK_DEV_PASSWORD:-}" ]; then
	PASSWORD=$("$BIN" version >/dev/null 2>&1 && head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-16)
else
	PASSWORD=$PLOCK_DEV_PASSWORD
fi

# --password-stdin, never --password: a password in argv is readable by any user on the
# machine for as long as the process runs and lands in the shell history. init's own usage
# text says the same thing, and a dev script that ignored it would be the example people
# copy.
printf '%s\n' "$PASSWORD" | "$BIN" init --username "$USERNAME" --password-stdin \
	--config "$CONFIG" --state-dir "$STATE" >/dev/null

if [ "$QUIET" -ne 1 ]; then
	cat <<EOF
plock dev loop
  config   $CONFIG
  state    $STATE
  audit    $AUDIT
  locker   $LOCKER
  calls    $LOCK_LOG        (tail -f this to watch the loop ask)
  ui       http://$LISTEN/
  admin    $USERNAME
  target   $TARGET  (the account enforcement acts on)
  password $PASSWORD

Enforcement is aimed at $TARGET, which defaults to you. That is safe here only
because the locker is a script that writes a log line: nothing in this tree refuses
to lock the session the daemon is running in, so with a real backend pointed at your
own account you would be locking yourself out of the machine you are using to stop
it. Set PLOCK_DEV_TARGET to another local account if you want the same loop without
aiming it at yourself.

The command backend does not confirm a lock, so the audit trail will carry
lock.unconfirmed. That is the honest reading, not a fault: a script exiting 0 is
not evidence a screen locked. Ctrl-C to stop.
EOF
fi

# exec so Ctrl-C reaches the daemon directly rather than killing this script and
# orphaning the daemon behind it.
exec "$BIN" serve \
	--config "$CONFIG" \
	--state-dir "$STATE" \
	--audit-path "$AUDIT"
