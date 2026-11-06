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
	printf '  A tick needs that tree twice over as well: `--agent factory` and the skill the instruction\n' >&2
	printf '  points at both resolve from the project the CLI works in, and that is the tree `$PWD` names\n' >&2
	printf '  -- the `Environment=PWD` line in the generated unit -- not `WorkingDirectory`, and not the\n' >&2
	printf '  tree the unit is asked to deliver into. A tick whose project holds no `.opencode/` cannot\n' >&2
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
GH_PATH=$(command -v gh 2>/dev/null || printf '')
OC_PATH=$(command -v "$FACTORY_OPENCODE" 2>/dev/null || printf '')
[ -n "$GH_PATH" ] || die "gh is not on PATH, so the service could not reach GitHub"
[ -n "$OC_PATH" ] || die "$FACTORY_OPENCODE is not on PATH, so a tick could not start"
# The list is the *gate's* toolchain, not merely the two binaries this script shells out to. Measured
# on the first tick that ever started (5 Nov, 45 minutes, killed at its own ceiling): the tick's
# capture step died on `FileNotFoundError: [Errno 2] No such file or directory: 'go'`, because `go`
# and `gofmt` live in ~/.local/bin on this box and the unit's PATH was built from `gh` and `opencode`
# alone. A tick cannot pass the gate without a compiler, so the pass spent its whole budget failing to
# verify, and the only evidence was a Python traceback 230 KB into a log nobody opens. Naming every
# binary the gate needs, and refusing to install without one, moves that failure from forty-five
# minutes into a tick to one line at install time.
missing=''
BIN_DIRS=
for tool in make git python3 go gofmt; do
	found=$(command -v "$tool" 2>/dev/null || printf '')
	if [ -z "$found" ]; then
		missing+="$tool "
		continue
	fi
	BIN_DIRS+=$(dirname -- "$found")$'\n'
done
if [ -n "$missing" ]; then
	printf 'install-timer: not on PATH, so a tick could never pass the gate it is required to run: %s\n' \
		"$missing" >&2
	printf '  A unit inherits no shell profile; this script is where its PATH is decided. Put those\n' >&2
	printf '  binaries on PATH and re-run --install.\n' >&2
	exit 1
fi
BIN_DIRS+=$(dirname -- "$GH_PATH")$'\n'$(dirname -- "$OC_PATH")
BIN_DIRS=$(printf '%s\n' "$BIN_DIRS" | grep -v '^[[:space:]]*$' | sort -u | paste -sd: -)
# Everything substituted into a unit passes through here. `s#@X@#value#g` is only safe while the
# value holds none of `\ & #` or a newline: in a replacement `&` means "the text that matched" and `\`
# escapes the next character, `#` ends the expression early, and a newline splits the sed script. So a
# repository path containing `&` pastes the matched text into the unit, a reviewer name containing `#`
# truncates the line, and either way the installed unit says something other than what the generator
# was handed -- which is the one failure of a generator that nobody can see without diffing it against
# the install. Measured both ways: `FACTORY_REVIEWER=a#b` used to emit `Environment=FACTORY_REVIEWER=a`,
# and now emits `Environment=FACTORY_REVIEWER=a#b`.
sed_replacement() {
	printf '%s' "$1" | sed -e 's/[\\&#]/\\&/g'
}

render() {
	# One template for both modes of path substitution: `@REPO@` and friends are replaced here rather
	# than by `envsubst`, so this script has no dependency outside coreutils.
	sed -e "s#@REPO@#$(sed_replacement "$FACTORY_ROOT")#g" \
		-e "s#@NAME@#$(sed_replacement "$NAME")#g" \
		-e "s#@PATHS@#$(sed_replacement "$BIN_DIRS:/usr/local/bin:/usr/bin:/bin")#g" \
		-e "s#@INTERVAL@#$(sed_replacement "$INTERVAL")#g" \
		-e "s#@STOP@#$(sed_replacement "$STOP")#g" \
		-e "s#@REVIEWER@#$(sed_replacement "$FACTORY_REVIEWER")#g" \
		-e "s#@AGENT@#$(sed_replacement "$FACTORY_AGENT")#g"
}

service_unit=$(cat <<'UNIT' | render
# Generated by factory/install-timer.sh -- edit the generator, not this file.
#
# Type=oneshot: one pass per activation, and the timer waits for it to finish. This box is a
# workstation, and a factory that only runs while somebody is looking at it is the status quo with
# extra steps -- see the linger note in docs/factory.md for what actually has to be true for that
# sentence to hold.
[Unit]
Description=@NAME@ delivery supervisor (one pass)
Documentation=file:@REPO@/docs/factory.md
# No network-online.target here, and that is a correction rather than an omission. `Wants=` on a unit
# the manager cannot load does nothing, and the user manager cannot load this one: measured 5 Nov,
# `systemctl --user status network-online.target` answers `Unit network-online.target could not be
# found.` -- the target exists in the system instance, where systemd-networkd-wait-online.service
# pulls it in, not in a user's. So the pair was a dependency on nothing: it ordered the pass after
# something that never appears, and implied the pass waits for the network when it does not. It does
# not need to: `gh` failing is a visible failure (`TRIAGE-FAILED`, rc=3, nothing attempted), and the
# next activation is ten minutes away.

[Service]
Type=oneshot
WorkingDirectory=@REPO@
ExecStart=@REPO@/factory/factory.sh
# The unit reads none of your shell profile: name the directories gh and opencode live in.
Environment=PATH=@PATHS@
Environment=FACTORY_REVIEWER=@REVIEWER@
# Not a convenience, and not the same thing as WorkingDirectory above. The CLI takes its project --
# the agent, the agent's deny rules, and the skill the instruction points at -- from $PWD, and
# chdirs there; a unit chdirs and exports no PWD, so the CLI never moves and the session opens in
# the background server's own directory instead. Measured 5 Nov: 152 passes in a row died inside two
# seconds on Error: Agent not found: "@AGENT@", each one logged as a bare exit=1, because no agent is
# defined out there. factory.sh sets the same variable at the call site too, so a pass started from a
# terminal is correct as well; this line is the half a person can see in systemctl cat.
Environment=PWD=@REPO@
# Both bounds are the same number, and both are wider than FACTORY_BUDGET.
#
# TimeoutStopSec: systemd kills a oneshot at this point on stop, and a tick killed mid-commit leaves a
# dirty worktree -- which the next pass finds, refuses to touch, and halts on. Deriving it from the
# budget at the top of the generator is what keeps the two numbers from drifting apart.
#
# TimeoutStartSec is written down because the default for this Type is not a number anybody would
# guess. systemd.service(5): TimeoutStartSec "Defaults to DefaultTimeoutStartSec= set in the manager,
# except when Type=oneshot is used, in which case the timeout is disabled by default." Measured here
# (systemd 259): the live unit read `TimeoutStartUSec=infinity` while `DefaultTimeoutStartUSec=1min
# 30s` sat on the manager, and a pass ran 45 minutes without being touched. That is the documented
# behaviour, not a bug -- but the pass holds `$FACTORY_RUN/lock` for as long as it runs, and while it
# holds it every later activation prints "another supervisor holds the lock" and does nothing. So a
# pass that wedges -- waiting on a server that never answers -- stops the factory silently, with one
# journal line a pass as the only symptom. An explicit start deadline is the whole fix, and it is here
# rather than in a comment because a default that depends on `Type` is a default that changes when
# someone changes `Type`.
TimeoutStopSec=@STOP@
TimeoutStartSec=@STOP@
Nice=10
IOSchedulingClass=best-effort
IOSchedulingPriority=7
UNIT
)

timer_unit=$(cat <<'UNIT' | render
# Generated by factory/install-timer.sh -- edit the generator, not this file.
[Unit]
Description=@NAME@ delivery supervisor, every @INTERVAL@

[Timer]
# OnUnitInactiveSec, not OnBootSec/OnCalendar: the next pass starts an interval after the previous
# one *finished*, so a long pass cannot stack a second supervisor behind it.
OnUnitInactiveSec=@INTERVAL@
RandomizedDelaySec=2m
AccuracySec=1m
# No Persistent=. It used to be here, next to a comment claiming it was what caught this machine up
# after being off, and it was doing nothing. systemd.timer(5), Persistent=: "this setting only has an
# effect on timers configured with OnCalendar=. Defaults to false." This unit has no OnCalendar=, so
# the flag was inert, and the comment was load-bearing for nobody: the thing that actually decides
# whether the factory runs while the machine is unattended is whether the *user manager* is running at
# all. The check for that is `loginctl show-user $USER --property=Linger`, and `no` means the manager
# -- this timer with it -- stops when you log out. `--install` runs that check and warns, because the
# answer belongs to the machine and not to this repository; docs/factory.md names the command that
# turns it on.

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

# The one thing about an unattended timer that is not in a file this generator controls. A *user* timer
# runs in the user manager, and the user manager exists only while something keeps it alive: without
# linger it is stopped at logout and the timer with it, and nothing in `systemctl cat` says so. Checked
# at install because that is the moment a person is looking, and phrased as a check rather than a value
# because the answer is a property of the machine: on the box this harness was written on it was `no`
# until somebody ran the command at 22:32:51Z on 5 Nov (the mtime of /var/lib/systemd/linger/rodney),
# which is exactly why docs/factory.md names the check and not a reading.
linger=$(loginctl show-user "$(id -un)" --property=Linger --value 2>/dev/null || printf 'unknown')
if [ "$linger" = "no" ]; then
	printf '\n  Linger is off for %s: the user manager -- and with it this timer -- stops at logout,\n' "$(id -un)"
	printf '  so the factory will not run unattended. The command that changes it is:\n\n'
	printf '      loginctl enable-linger %s\n\n' "$(id -un)"
	log_event "LINGER-OFF user=$(id -un) timer=$NAME"
fi

log_event "TIMER-INSTALLED $NAME interval=$INTERVAL repo=$FACTORY_ROOT"
printf '\nInstalled:\n  %s\n  %s\n\n' "$SERVICE" "$TIMER"
systemctl --user list-timers "$NAME.timer" --no-pager 2>&1 | sed -n '1,3p'
printf '\nJournal:  journalctl --user -u %s.service -n 50\nRemove:   %s --remove\n' \
	"$NAME" "$0"
