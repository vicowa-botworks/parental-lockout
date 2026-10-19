#!/usr/bin/env bash
# lib.sh -- the checks both e2e scripts need, kept in one file on purpose.
#
# Sourced, not run. Everything here answers a question about the machine rather than about
# this repository, and the answers have to agree with each other: e2e.sh and forge-hint.sh
# both ask "is a locker running for this uid", and two copies of that rule are how one of
# them ends up printing a verdict the other would refuse.
#
# The locker rule is a copy of internal/platform/locker.go's three requirements, and it is a
# copy for a reason worth stating. The obvious alternative -- ask the daemon, which already
# implements them -- would make the harness's second signal come from the same code whose
# behaviour is under test: a buggy corroborator would agree with itself and the transcript
# would still say PASS. The harness is the independent reading. What must not drift is the
# rule, so it lives here once, with the Go file named as its source and the reason each
# requirement exists written next to it.
#
# shellcheck shell=bash
#
# Linted as part of CI's pinned shellcheck job. It is sourced rather than executed, so nothing here
# runs on its own and every function is answerable about a machine, not about this repository.

# not_writable_by <path> <uid> -- could this uid have put these bytes here?
#
# Owned by them is disqualifying on its own: they chose the contents. Group- or
# world-writable is equally disqualifying, because a binary anybody can append to is a binary
# they can change without touching its owner. A root-owned 0755 file under /usr passes both,
# which is the real desktop.
#
# This is the requirement that does the work. argv[0] is whatever the caller passed to execve
# and /proc/<pid>/exe only says which file is running, so without it an impostor needs a
# binary merely *named* kscreenlocker_greet -- and the session user can put one anywhere they
# own, which is exactly what locker.go's comment says the check is for.
not_writable_by() {
	local f=$1 u=$2 owner mode
	owner=$(stat -c '%u' -- "$f" 2>/dev/null) || return 1
	mode=$(stat -c '%a' -- "$f" 2>/dev/null) || return 1
	[ "$owner" != "$u" ] || return 1
	# %a is octal digits, possibly four (a setuid 4755). 8# forces octal regardless.
	[ $((8#$mode & 8#22)) -eq 0 ] || return 1
	return 0
}

# trusted_exe <pid> <program> <uid> -- is this pid running <program>, from a file the uid
# could not have written? Both the file and the directory holding it have to pass, because a
# writable directory lets the uid replace the file under a name they do not own.
trusted_exe() {
	local pid=$1 want=$2 uid=$3 exe dir
	[ -n "$pid" ] || return 1
	exe=$(readlink -- "/proc/$pid/exe" 2>/dev/null) || return 1
	# The kernel appends " (deleted)" when the file was unlinked or replaced under a running
	# process. The bytes at that path are gone, so nothing about them can be vouched for.
	case $exe in *" (deleted)") return 1 ;; esac
	[ "${exe##*/}" = "$want" ] || return 1
	not_writable_by "$exe" "$uid" || return 1
	dir=${exe%/*}
	[ -n "$dir" ] || return 1
	not_writable_by "$dir" "$uid"
}

# locker_pid <uid> -- the pid of a locker running as <uid>, and nothing if there is none.
#
# Why not pgrep: `pgrep -f 'kscreenlocker_greet|gsd-screensaver'` matches command LINES, so
# any shell whose argv merely contains that pattern -- a wrapper, a `bash -c`, an editor with
# the string in its buffer -- reports a locker that does not exist. Measured on the
# development host: two matches, both shells. `pgrep -x kscreenlocker_greet` can never match
# at all: it compares against comm, which the kernel holds to 15 characters and the binary's
# name is 19, and pgrep says so itself.
#
# Three requirements, in the order locker.go applies them, and the uid comes first because it
# is the cheapest read and it discards the whole machine except the session being locked:
#
#   1. the process runs as the uid whose lock is being judged. Without it a root harness walks
#      every process on the box and a sibling account's locker confirms the target's lockout --
#      on the two-account machine this tool is for, that attributes one child's lockout to
#      another.
#   2. the executable the kernel started is the expected program, from a file and directory the
#      uid cannot write.
#   3. for the KDE locker, its parent is ksmserver, passing requirement 2 as well. The locker
#      is a child of the process that owns org.freedesktop.ScreenSaver and answers the Lock
#      call, so requiring both halves makes the check about a process tree rather than one
#      name -- two execve calls as the session user produce a tree whose names are both lies.
#
# The GNOME locker has no measured parent, so requirement 3 is not applied to it and the
# answer says so rather than implying the same strength. That asymmetry is a fact about what
# has been observed on hardware, not a shortcut.
locker_pid() {
	local want_uid=${1:-} p uid exe name ppid
	[ -n "$want_uid" ] || {
		echo "locker_pid needs the uid whose locker to look for" >&2
		return 2
	}
	for p in /proc/[0-9]*; do
		uid=$(awk '/^Uid:/{print $2; exit}' "$p/status" 2>/dev/null) || continue
		[ "$uid" = "$want_uid" ] || continue
		exe=$(readlink -- "$p/exe" 2>/dev/null) || continue
		case $exe in *" (deleted)") continue ;; esac
		name=${exe##*/}
		case $name in
		kscreenlocker_greet)
			trusted_exe "${p##*/}" kscreenlocker_greet "$want_uid" || continue
			ppid=$(awk '/^PPid:/{print $2; exit}' "$p/status" 2>/dev/null) || continue
			trusted_exe "$ppid" ksmserver "$want_uid" || continue
			basename -- "$p"
			return 0
			;;
		gsd-screensaver)
			# No parent rule: nothing has been measured about which process spawns the
			# GNOME locker, and inventing a second signal to corroborate one is the rubber
			# stamp the threat model is entirely about.
			trusted_exe "${p##*/}" gsd-screensaver "$want_uid" || continue
			basename -- "$p"
			return 0
			;;
		esac
	done
	return 1
}

# locker_rule <name> -- which requirements a given locker name is judged by, for the
# transcript to say what was actually checked.
locker_rule() {
	case ${1:-} in
	kscreenlocker_greet)
		printf 'uid, /proc/exe plus file and directory writability, and a ksmserver parent passing both\n'
		;;
	gsd-screensaver)
		printf 'uid and /proc/exe plus writability; no parent rule, because no GNOME parent has been measured\n'
		;;
	*) printf 'unknown locker %s\n' "${1:-}" ;;
	esac
}

# exe_name <pid> -- the basename of the executable the kernel started.
exe_name() {
	local e
	e=$(readlink -- "/proc/${1:-}/exe" 2>/dev/null) || return 1
	printf '%s\n' "${e##*/}"
}

# describe_exe <pid> -- the evidence the rule used, in the transcript next to the verdict that
# used it. A PASS that names a pid nobody can re-examine after the run says less than one that
# records which file was trusted and who owns it, because the ownership is the claim.
describe_exe() {
	local pid=${1:-} exe ppid
	exe=$(readlink -- "/proc/$pid/exe" 2>/dev/null) || {
		printf 'pid %s: no /proc/exe answer\n' "$pid"
		return 1
	}
	printf 'trusted exe : %s\n' "$exe"
	stat -c '  file        : uid %u (%U) mode %a' -- "$exe" 2>/dev/null
	stat -c '  directory   : uid %u (%U) mode %a' -- "${exe%/*}" 2>/dev/null
	ppid=$(awk '/^PPid:/{print $2; exit}' "/proc/$pid/status" 2>/dev/null)
	printf '  parent %s  : %s\n' "${ppid:-?}" "$(ps -o args= -p "${ppid:-0}" 2>/dev/null)"
}

# session_of [user] -- the graphical session id of the named user, or of whoever is running
# this. One awk rather than the five copies this replaces.
#
# The tty test is $7 != "" AND $7 != "-". loginctl prints a literal dash for a session with
# no tty, so a test for "not empty" is true for every user-class session: measured on the
# review host, all four user sessions matched such a filter, none had a tty, and three were
# remote ssh sessions. For the unprivileged forge that is not cosmetic -- picking a remote
# session over the desktop one forges a hint on a session that was never the subject, and the
# "no locker is running" it then reports is trivially true there.
session_of() {
	local u
	if [ $# -ge 1 ] && [ -n "$1" ]; then
		u=$(id -u "$1" 2>/dev/null) || return 1
	else
		u=$(id -u)
	fi
	loginctl list-sessions --no-legend 2>/dev/null |
		awk -v u="$u" '$2==u && $6=="user" && $7!="" && $7!="-" {print $1; exit}'
}

# daemon_pid -- the pid of a running `plock serve`, found by the executable the kernel
# started AND the subcommand in its argv.
#
# Matching the exe name alone lets a hand-run `plock status` during the measurement window
# pass for the daemon, and the harness would then report that the loop kept asserting on the
# strength of a process that never enforced anything.
daemon_pid() {
	local p exe cmd
	for p in /proc/[0-9]*; do
		exe=$(readlink -- "$p/exe" 2>/dev/null) || continue
		[ "${exe##*/}" = plock ] || continue
		cmd=$(tr '\0' ' ' <"$p/cmdline" 2>/dev/null) || continue
		case $cmd in *" serve "*) ;; *) continue ;; esac
		basename -- "$p"
		return 0
	done
	return 1
}
