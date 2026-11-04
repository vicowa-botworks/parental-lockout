#!/usr/bin/env bash
# shellcheck shell=bash disable=SC2016
# install-timer.sh -- put the supervisor on the user's own systemd instance, or take it off.
#
# Why a script and not two files to copy: the unit needs absolute paths for the repository, for
# `gh`, and for `opencode`, and a person copying an example out of a README gets those three wrong in
# ways that all look like "the timer fired and nothing happened". Every path here is asked of the
# machine at install time.
#
# The user instance, never the system one: nothing here needs root, and a timer that runs as root to
# move a repository's pull requests is a bigger incident than a stalled ticket.
#
# usage: install-timer.sh [--install | --remove] [--dry-run] [--name NAME]
set -euo pipefail

# shellcheck source=./common.sh
# shellcheck disable=SC1091
. "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/common.sh"

NAME=
MODE=

usage() {
	cat <<'USAGE'
usage: install-timer.sh [--install | --remove] [--dry-run] [--name NAME]

--install   write the two units, daemon-reload, enable --now the timer
--remove    disable --now the timer, remove the units, daemon-reload
--dry-run   print the units and the systemctl commands, write nothing
--name NAME   unit name without the .service/.timer suffix
            (default: <repo>-factory, e.g. parental-lockout-factory)

Installs into ~/.config/systemd/user and needs no root. The interval is FACTORY_INTERVAL (default
10m); a unit waiting on a reviewer is re-poked on FACTORY_REVIEW_POKES regardless, so the interval
buys responsiveness on newly-green work, not more reviews.
USAGE
}

while [ $# -gt 0 ]; do
	case "$1" in
	--install) MODE=install ;;
	--remove) MODE=remove ;;
	--dry-run) DRY_RUN=1 ;;
	--name)
		[ $# -ge 2 ] || die "--name wants a unit name"
		NAME=$2
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		printf 'install-timer: unknown argument: %s\n' "$1" >&2
		usage >&2
		exit 2
		;;
	esac
	shift
done

[ -n "$MODE" ] || {
	printf 'install-timer: say --install or --remove; there is no default, because the two are\n' >&2
	printf 'opposites and a mistyped flag should not take a running timer away.\n' >&2
	usage >&2
	exit 2
}

factory_preflight
[ -n "$NAME" ] || NAME="${FACTORY_NAME}-factory"

# The unit runs `ExecStart=$FACTORY_ROOT/factory/factory.sh`, so that path has to contain the harness
# *at boot time* -- which means the branch the timer points at has to be one where it is committed.
# A timer installed from a worktree whose branch is not merged therefore points at whatever the
# primary checkout happens to hold, and if main does not carry factory/ yet, every activation finds
# no script, exits 127, and looks like a machine that is trying.
if [ ! -x "$FACTORY_ROOT/factory/factory.sh" ]; then
	printf 'install-timer: %s has no executable factory/factory.sh.\n' "$FACTORY_ROOT" >&2
	printf '  The timer runs from the primary checkout, so the harness must be merged there first --\n' >&2
	printf '  or point the install at the tree that holds it: FACTORY_ROOT=<checkout> %s --install\n' "$0" >&2
	printf '  A tick has the same requirement twice over: `--agent factory` and the skill it loads both\n' >&2
	printf '  live in the tree the tick works in, so a tick on a branch cut from an unmerged main cannot\n' >&2
	printf '  start at all. The mechanical half -- review requests, merges, conflicts -- needs neither.\n' >&2
	exit 1
fi

UNITS="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
SERVICE="$UNITS/$NAME.service"
TIMER="$UNITS/$NAME.timer"
INTERVAL=${FACTORY_INTERVAL:-10m}
# The timer's ceiling has to clear the supervisor's own budget. systemd kills a oneshot at
# TimeoutStopSec, and a tick killed mid-commit leaves a dirty worktree -- which the next pass finds,
# refuses to touch, and halts on. Deriving it here is what keeps the two numbers from drifting apart.
STOP=$((FACTORY_BUDGET + 60))

systemctl_user() {
	if [ "$DRY_RUN" = "1" ]; then
		printf 'DRY-RUN  systemctl --user %s\n' "$*"
		return 0
	fi
	systemctl --user "$@"
}

if [ "$MODE" = "remove" ]; then
	systemctl_user disable --now "$NAME.timer" || true
	systemctl_user stop "$NAME.service" || true
	run_action rm -f -- "$SERVICE" "$TIMER"
	systemctl_user daemon-reload
	log_event "TIMER-REMOVED $NAME ($SERVICE, $TIMER)"
	exit 0
fi

# Where the binaries actually are. A systemd unit does not read a shell profile, so an `opencode`
# that works in a terminal is not an `opencode` the service can find -- and the symptom is a timer
# that fires, exits 0, and delivers nothing.
gh_path=$(command -v gh 2>/dev/null || printf '')
oc_path=$(command -v "$FACTORY_OPENCODE" 2>/dev/null || printf '')
[ -n "$gh_path" ] || die "gh is not on PATH, so the service could not reach GitHub"
[ -n "$oc_path" ] || die "$FACTORY_OPENCODE is not on PATH, so a tick could not start"
BIN_DIRS=$(printf '%s\n' "$gh_path" "$oc_path" |
	xargs -n1 dirname | sort -u | paste -sd: -)

render() {
	# One template for both modes of path substitution: `@REPO@` and friends are replaced here rather
	# than by `envsubst`, so this script has no dependency outside coreutils.
	sed -e "s#@REPO@#$FACTORY_ROOT#g" \
		-e "s#@NAME@#$NAME#g" \
		-e "s#@PATHS@#$BIN_DIRS:/usr/local/bin:/usr/bin:/bin#g" \
		-e "s#@INTERVAL@#$INTERVAL#g" \
		-e "s#@STOP@#$STOP#g" \
		-e "s#@REVIEWER@#$FACTORY_REVIEWER#g"
}

service_unit=$(cat <<UNIT | render
# Generated by factory/install-timer.sh -- edit the generator, not this file.
#
# Type=oneshot with Persistent=true on the timer: this box is a workstation, and a factory that only
# runs while somebody is looking at it is the status quo with extra steps.
[Unit]
Description=$NAME delivery supervisor (one pass)
Documentation=file:$FACTORY_ROOT/docs/factory.md
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
WorkingDirectory=$FACTORY_ROOT
ExecStart=$FACTORY_ROOT/factory/factory.sh
# The unit reads none of your shell profile: name the directories gh and opencode live in.
Environment=PATH=@PATHS@
Environment=FACTORY_REVIEWER=@REVIEWER@
# Longer than FACTORY_BUDGET so systemd never kills a tick mid-commit; a killed tick leaves a dirty
# worktree, and the next pass halts on it rather than cleaning it up.
TimeoutStopSec=@STOP@
Nice=10
IOSchedulingClass=best-effort
IOSchedulingPriority=7
UNIT
)

timer_unit=$(cat <<UNIT | render
# Generated by factory/install-timer.sh -- edit the generator, not this file.
[Unit]
Description=$NAME delivery supervisor, every @INTERVAL@

[Timer]
# OnUnitInactiveSec, not OnBootSec/OnCalendar: the next pass starts an interval after the previous
# one *finished*, so a long pass cannot stack a second supervisor behind it.
OnUnitInactiveSec=@INTERVAL@
RandomizedDelaySec=2m
Persistent=true
AccuracySec=1m

[Install]
WantedBy=timers.target
UNIT
)

if [ "$DRY_RUN" = "1" ]; then
	printf 'DRY-RUN  write %s\n%s\n' "$SERVICE" "$service_unit"
	printf 'DRY-RUN  write %s\n%s\n' "$TIMER" "$timer_unit"
	printf 'DRY-RUN  systemctl --user daemon-reload && systemctl --user enable --now %s.timer\n' "$NAME"
	exit 0
fi

mkdir -p -- "$UNITS"
printf '%s\n' "$service_unit" >"$SERVICE"
printf '%s\n' "$timer_unit" >"$TIMER"
systemctl_user daemon-reload
systemctl_user enable --now "$NAME.timer"

log_event "TIMER-INSTALLED $NAME interval=$INTERVAL repo=$FACTORY_ROOT"
printf '\nInstalled:\n  %s\n  %s\n\n' "$SERVICE" "$TIMER"
systemctl --user list-timers "$NAME.timer" --no-pager 2>&1 | sed -n '1,3p'
printf '\nJournal:  journalctl --user -u %s.service -n 50\nRemove:   %s --remove\n' \
	"$NAME" "$0"
