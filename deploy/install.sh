#!/bin/sh
# install.sh -- put plock on a machine as a service, or take it back off.
#
# Run without arguments to install, --uninstall to remove. Both need root.
#
# The one thing this script never does is start or enable the service. Enabling a root
# daemon that locks other people's sessions is the operator's decision, made once, in front
# of them -- so the script prints the command and stops. A script that quietly ran
# `systemctl enable --now` would mean the first lockout happened on the machine's next boot
# without anyone having decided it should.
#
# It also never writes /etc/plock/config.json. That file is the policy and it holds the
# admin credential; `plock init` creates it, deliberately, with a password the operator
# types. An installer that dropped in a default config would either ship a known credential
# or overwrite a family's rules, and both are worse than a missing file with a clear error.

set -eu

# DESTDIR stages the install under a prefix without touching the live system, which is what
# a package build needs and also what makes this script testable without root: with DESTDIR
# set, every path is rebased and systemctl is not called at all. Staging a unit file must
# never daemon-reload the machine doing the staging -- that is the standard packaging rule,
# and here it doubles as the reason the destructive branches can be exercised for real.
DESTDIR=${DESTDIR:-}

BIN_SRC=${BIN_SRC:-bin/plock}
UNIT_SRC=${UNIT_SRC:-deploy/plock.service}
BIN_DST=${DESTDIR}/usr/local/bin/plock
UNIT_DST=${DESTDIR}/etc/systemd/system/plock.service
CONF_DIR=${DESTDIR}/etc/plock
STATE_DIR=${DESTDIR}/var/lib/plock
LOG_DIR=${DESTDIR}/var/log/plock
DOC_DIR=${DESTDIR}/usr/share/doc/plock

# Operator-facing paths. These are the destinations the unit itself names -- its
# ExecStart is /usr/local/bin/plock no matter what prefix the files were staged
# under -- so the guidance a packager sees on a staged run is the guidance an
# operator needs, rather than a path inside a build root.
BIN_AT=/usr/local/bin/plock
CONF_AT=/etc/plock
STATE_AT=/var/lib/plock
LOG_AT=/var/log/plock

say() { printf '%s\n' "$*"; }
die() { printf 'install: %s\n' "$*" >&2; exit 1; }

# systemctl is only ever asked about plock, and never when staging.
sysd() {
  [ -n "$DESTDIR" ] || systemctl "$@"
}

[ "${1:-}" = "--help" ] && {
  printf '%s\n' "usage: $0 [--uninstall [--purge]]" "" \
    "Install plock as a system service, or remove it." "" \
    "  --uninstall   remove the binary, the unit and the docs" \
    "  --purge       with --uninstall: also delete the policy, the audit trail and the" \
    "                certificate. There is no undo for the trail." \
    "  --help        this" "" \
    "Environment:" \
    "  DESTDIR      stage under a prefix instead of the live system (no systemctl)" \
    "  BIN_SRC      binary to install  (default bin/plock)" \
    "  UNIT_SRC     unit file to install (default deploy/plock.service)" "" \
    "Neither starting nor enabling the service is this script's doing; it prints the" \
    "command and waits."
  exit 0
}

# The root requirement is about what the script touches: /usr/local/bin, /etc/systemd, and
# the system manager. Under DESTDIR none of those are touched, and a package build is
# routinely run as an unprivileged user precisely so it cannot write outside the prefix -- so
# demanding root there would be demanding a permission for a thing that is not being done.
if [ -z "$DESTDIR" ] && [ "$(id -u)" != 0 ]; then
  die "needs root (this installs a system service), or set DESTDIR to stage it"
fi

# A staged install has no opinion about the live manager, so the check that this is a
# systemd machine only applies when we are about to talk to one.
[ -n "$DESTDIR" ] || command -v systemctl >/dev/null 2>&1 || die "no systemctl: this is not a systemd machine"

# ---------------------------------------------------------------------------
# uninstall
# ---------------------------------------------------------------------------

if [ "${1:-}" = "--uninstall" ] || [ "${1:-}" = "--purge" ]; then
  # Two spellings of the same verb, one flag: `--uninstall` is the verb the ticket names and
  # the one that reads correctly on its own; `--purge` alone is what somebody reaching for
  # `make uninstall --purge` types. Both mean "uninstall", and only --purge adds "and take
  # the family's data with it".
  purge=no
  # --purge is a valid first word on its own, so it has to be read before the shift that
  # removes the verb. Reading only the tail meant `install.sh --purge` silently performed a
  # plain uninstall and then reported that the trail had been left behind -- which is the
  # safe direction to fail, but it is still a command that did not do what was typed.
  [ "${1:-}" != "--purge" ] || purge=yes
  shift
  for a in "$@"; do
    case "$a" in
      # Either word may come first, so the verb has to be tolerated in the tail as well as
      # in $1. Refusing it would make `--purge --uninstall` die on the word that names the
      # thing being asked for.
      --uninstall) : ;;
      --purge) purge=yes ;;
      *) die "unknown argument for --uninstall: $a" ;;
    esac
  done

  # Refuse while the unit is still loaded. Removing the unit file out from under a running
  # service leaves a daemon enforcing from a deleted definition -- `systemctl status` then
  # reports a unit that cannot be reloaded, and the next reboot is the first anyone finds
  # out. Making the operator say stop first is one typed command and removes the whole class.
  if sysd list-unit-files plock.service >/dev/null 2>&1 \
     && [ -n "$(sysd show -p LoadState --value plock.service 2>/dev/null || true)" ] \
     && [ "$(sysd show -p LoadState --value plock.service 2>/dev/null || echo missing)" != "not-found" ]; then
    if [ "$(sysd is-active plock 2>/dev/null || true)" = "active" ] \
       || [ "$(sysd is-enabled plock 2>/dev/null || true)" = "enabled" ]; then
      die "plock is still enabled or running. Stop it yourself first:

    systemctl disable --now plock

    Then re-run: $0 --uninstall"
    fi
  fi

  rm -f "$UNIT_DST"
  rm -f "$BIN_DST"
  rm -rf "$DOC_DIR"
  sysd daemon-reload
  sysd reset-failed plock.service 2>/dev/null || true

  if [ "$purge" = yes ]; then
    # Asked for by name, on the command line, by a root shell. This is the only path that
    # destroys the audit trail, so it is the only one that has to be spelled out.
    rm -rf "$CONF_DIR" "$LOG_DIR" "$STATE_DIR"
    say "removed the binary, the unit and the docs, and purged:"
    say "  $CONF_AT $LOG_AT $STATE_AT"
    say
    say "The audit trail is gone. Nothing in this project can reconstruct when enforcement
ran or who granted an override."
    exit 0
  fi

  # Deliberately left behind, and said out loud rather than left as a surprise:
  #   /etc/plock/config.json    the policy, and the admin credential hash
  #   /var/log/plock/           the audit trail
  # The trail is the record of when enforcement ran and when an override was granted. A
  # parent may still need it months later, and an uninstall is the wrong moment to decide
  # it is worthless. Reinstalling onto these same directories is the normal case -- a
  # package refresh, a moved binary path -- and it should not require rewriting every rule
  # and re-issuing the certificate.
  say "removed the binary, the unit and the docs."
  say
  say "Left in place, because they are not the program's files, they are yours:"
  say "  $CONF_AT   the policy and the admin credential"
  say "  $LOG_AT    the audit trail"
  say "  $STATE_AT  the TLS certificate and key"
  say
  say "To remove those too, once you are sure you do not need the trail:"
  say "  deploy/install.sh --uninstall --purge"
  exit 0
fi

[ $# -eq 0 ] || die "usage: $0 [--uninstall]"

# ---------------------------------------------------------------------------
# install
# ---------------------------------------------------------------------------

[ -f "$BIN_SRC" ] || die "no binary at $BIN_SRC -- run \`make build' first (or set BIN_SRC)"
# Executing it is the check. A file that exists is not necessarily the program: a stale
# binary from another branch, an architecture mismatch from a cross build, or a truncated
# copy from a half-finished scp all pass a plain existence test, and the failure would
# surface as a crash loop on a machine that is supposed to be enforcing tonight.
if ! "$BIN_SRC" version >/dev/null 2>&1; then
  die "$BIN_SRC does not run on this machine. Try: $BIN_SRC version"
fi

# 0755, not 0700: the daemon execs nothing by path that a user runs, but `plock doctor` and
# `plock status` are meant to be runnable by the admin, and a root-only binary makes the
# documented troubleshooting steps fail with a permission error that looks like a bug in
# plock rather than in the install.
install -d -m 0755 "$(dirname "$BIN_DST")"
install -m 0755 "$BIN_SRC" "$BIN_DST"

install -d -m 0755 "$(dirname "$UNIT_DST")"
install -m 0644 "$UNIT_SRC" "$UNIT_DST"

# The unit's Documentation= lines point here, so `systemctl status plock` can tell the
# operator what the hardening does without them opening a browser on the wrong machine.
if [ -d docs ]; then
  install -d -m 0755 "$DOC_DIR"
  install -d -m 0755 "$DOC_DIR/docs"
  for f in docs/*.md; do
    [ -f "$f" ] && install -m 0644 "$f" "$DOC_DIR/docs/"
  done
fi

# Created here rather than left to StateDirectory=/LogsDirectory= so the modes are right
# before the first start, and so a pre-existing directory with the wrong owner is reported
# now instead of at 03:00 when the daemon cannot write its trail.
#
# /etc/plock is 0700 and not 0755: the config inside is 0600 root-owned, but a readable
# directory still leaks which accounts have profiles and lets an attacker time edits against
# the reload interval. The mode is the smaller exposure either way.
#
# The check on an existing directory mirrors the writer's own rule exactly, and that is the
# reason it is not stricter: internal/audit rejects a WORLD-writable parent and deliberately
# tolerates a group-writable one, because /var/log is root:adm 0755 or 0775 on many
# distributions. An installer that demanded 0700 on /var/log/plock's parent would refuse the
# standard layout of the distros this is written for. So: world-writable or not root-owned is
# a hard stop; everything else is left alone.
for d in "$CONF_DIR" "$STATE_DIR" "$LOG_DIR"; do
  if [ -d "$d" ]; then
    # The owner check is about the live system's security, and a staged tree has no owner
    # that means anything -- packaging sets ownership when the package is built, which is why
    # the install below only asks for ownership when it is not staging.
    if [ -z "$DESTDIR" ]; then
      owner=$(stat -c '%U' "$d")
      [ "$owner" = root ] || die "$d is owned by $owner, not root. The daemon reads a root-only
    policy from it and writes an audit trail it must be able to trust:
      chown root:root $d"
    fi
    # -o+w matches any of the world bits that grant write; a plain 4 would miss 6 and 7.
    if find "$d" -maxdepth 0 -perm -o+w | grep -q .; then
      die "$d is world-writable ($(stat -c '%a' "$d")). The audit writer refuses a
    world-writable parent rather than write a trail anyone could have edited, so the daemon
    would refuse to start. Fix it:
      chmod o-w $d"
    fi
  elif [ -n "$DESTDIR" ]; then
    install -d -m 0700 "$d"
  else
    install -d -m 0700 -o root -g root "$d"
  fi
done
# The state directory holds the private key; the log directory holds the trail. Neither is
# readable by the accounts being locked out, and 0700 is what the unit asks systemd for.
chmod 0700 "$CONF_DIR" "$STATE_DIR"
chmod 0750 "$LOG_DIR"

sysd daemon-reload

say "installed:"
say "  $BIN_AT"
say "  $UNIT_DST"
say "  $CONF_AT $STATE_AT $LOG_AT"
say
say "Nothing is running yet, and nothing will start until you say so."
say
if [ -f "$CONF_DIR/config.json" ]; then
  say "$CONF_AT/config.json already exists, so the policy and the admin credential were
left exactly as they were."
else
  # There is no interactive prompt. `plock init` is non-interactive by design and requires
  # both the account name and where the password comes from, so the guidance has to show the
  # flags rather than imply the program will ask. --password-stdin is the form to lead with:
  # a password in argv is visible to every user on the machine for the life of the process
  # and stays in the shell history afterwards.
  say "There is no policy file yet. Create one, naming the account and reading the password"
  say "off stdin so it never appears in argv or your shell history:"
  say
  say "    $BIN_AT init --username YOUR_LOGIN --password-stdin"
  say
  say "Then read it back and check the rules are the ones you meant:"
  say "    $BIN_AT status"
fi
say
say "Then check this machine can actually lock its screen before you make it a service:"
say "    $BIN_AT doctor"
say
say "Start it, when you are ready:"
say
say "    systemctl enable --now plock"
say
say "and watch it come up:"
say "    systemctl status plock"
say "    journalctl -u plock -f"
