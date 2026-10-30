#!/bin/sh
# package-verify.sh -- install both packages on machines that do not exist and check what
# they did.
#
#   scripts/package-verify.sh --deb dist/plock_*.deb --rpm dist/plock-*.rpm
#
# Environment:
#   PACKAGE_VERIFY=1   required. Without it this script refuses to run.
#   PKG_DEB_IMAGE      default debian:13
#   PKG_RPM_IMAGE      default fedora:43
#   CONTAINER_RUNTIME  docker or podman, auto-detected
#
# Why it is gated behind an environment variable: it pulls two distribution images and
# installs a root daemon into each. That is the right thing for CI and the wrong thing to
# discover inside `make verify` on someone's laptop, in the same way `make mutate` is not
# part of verify -- a gate that surprises people is a gate they skip.
#
# What it is actually looking for is the one failure mode two formats make easy: the .deb
# and the .rpm containing the same program in *different places*, or with different modes,
# or with a scriptlet that only one of them got right. Those are invisible to any test that
# reads one package at a time, and they are invisible in production as a family whose
# daemon behaves differently depending on whether the operator typed apt or dnf.
#
# What it has already caught, on the tree that introduced it: rpm's install post-processing
# stripped the ELF payload, so the two packages shipped binaries that differed at byte 41
# with identical build info (fixed by %global __strip /bin/true in the spec).
set -eu

DEB=${DEB:-}
RPM=${RPM:-}
DEB_IMAGE=${PKG_DEB_IMAGE:-debian:13}
RPM_IMAGE=${PKG_RPM_IMAGE:-fedora:43}

die() {
  printf 'package-verify: %s\n' "$*" >&2
  exit 1
}

[ "${PACKAGE_VERIFY:-}" = 1 ] || die "refusing to pull images and install packages here.
     This is the CI gate; run it with PACKAGE_VERIFY=1 (see scripts/package-verify.sh)."

while [ $# -gt 0 ]; do
  case "$1" in
    --deb) DEB=${2:-}; shift 2 ;;
    --rpm) RPM=${2:-}; shift 2 ;;
    *) die "unknown argument '$1'" ;;
  esac
done

if [ -z "$DEB" ] || [ ! -f "$DEB" ]; then
  die "no --deb file (got ${DEB:-nothing})"
fi
if [ -z "$RPM" ] || [ ! -f "$RPM" ]; then
  die "no --rpm file (got ${RPM:-nothing})"
fi

RUNTIME=${CONTAINER_RUNTIME:-}
if [ -z "$RUNTIME" ]; then
  for c in podman docker; do
    command -v "$c" >/dev/null 2>&1 && RUNTIME=$c && break
  done
fi
[ -n "$RUNTIME" ] || die "no podman or docker; this gate needs a container runtime"

work=$(mktemp -d "${TMPDIR:-/tmp}/plock-pkgverify-XXXXXX")
trap 'rm -rf "$work"' EXIT INT TERM

# The artifacts are copied in because a container runtime may not be able to mount a path
# that is itself a mount, and `dist/` on a developer machine often is.
cp "$DEB" "$work/plock.deb"
cp "$RPM" "$work/plock.rpm"

# Both probes emit the same record types, so the comparison is a diff rather than a
# translation:
#
#   FILE <mode> <path>   a path the package owns, with its mode on the installed system
#   UNIT <sha256>        the installed unit file
#   BINARY <sha256>      the installed binary
#   VERSION <line>       `plock version` as the installed program reports it
#   ENABLED <yes|no>     whether anything wired the unit into a target
#   <TEARDOWN> <name>=<exists|absent>   where TEARDOWN is REMOVE, PURGE or ERASE
#
# Modes are octal as `stat` prints them. Ownership is not included: both formats are
# required to install root:root, and a package that failed that would fail the install.
#
# Directories that any distribution creates anyway are left out of FILE. dpkg -L reports
# every parent of every installed path while rpm -ql reports only what %files declares, so
# without this the two lists differ by the shape of /usr and nothing else -- a diff that
# would be re-explained on every run until whoever sees it learns to ignore it.
cat >"$work/common.sh" <<'COMMON'
is_scaffolding() {
  case "$1" in
    / | /. | /bin | /boot | /dev | /etc | /etc/systemd | /etc/systemd/system | /home | \
      /lib | /lib64 | /media | /mnt | /opt | /proc | /root | /run | /sbin | /srv | /sys | \
      /tmp | /usr | /usr/bin | /usr/lib | /usr/local | /usr/local/bin | /usr/local/sbin | \
      /usr/local/share | /usr/share | /usr/share/doc | /usr/share/man | /var | /var/cache | \
      /var/lib | /var/log | /var/opt | /var/spool) return 0 ;;
    *) return 1 ;;
  esac
}

# A sentinel in each, so "the state survived" means a file survived and not a directory
# happened to be left. An empty owned directory being removed is not data loss; a policy
# file being removed is, and only a file can tell the two apart.
plant_state() {
  printf 'sentinel\n' >/etc/plock/config.json
  printf 'sentinel\n' >/var/lib/plock/plock.key
  printf 'sentinel\n' >/var/log/plock/plock.log
}

report_state() {
  label=$1
  for p in /etc/plock/config.json /var/lib/plock/plock.key /var/log/plock/plock.log; do
    if [ -e "$p" ]; then
      printf '%s %s=exists\n' "$label" "${p#/}"
    else
      printf '%s %s=absent\n' "$label" "${p#/}"
    fi
  done
}
# "exists" is not "is still the same file". A scriptlet that deleted a sentinel and let
# something else create it again would report exists, so the upgrade path re-reads the
# contents plant_state wrote.
report_sentinels() {
  for p in /etc/plock/config.json /var/lib/plock/plock.key /var/log/plock/plock.log; do
    if [ -r "$p" ] && [ -f "$p" ] && [ "$(head -n 1 "$p")" = sentinel ]; then
      printf 'SENTINEL %s=kept\n' "${p#/}"
    else
      printf 'SENTINEL %s=lost\n' "${p#/}"
    fi
  done
}

report_enabled() {
  # "Nothing is running yet" is the product's promise. A symlink in a target's .wants
  # directory is what enabling looks like, and looking for it works on a machine with no
  # running systemd -- which is every container, and every build root.
  if find /etc/systemd/system -maxdepth 3 -path '*.wants/*plock*' 2>/dev/null | grep -q .; then
    echo 'ENABLED yes'
  else
    echo 'ENABLED no'
  fi
}
COMMON

cat >"$work/probe-deb.sh" <<'PROBE'
set -eu
# shellcheck source=/dev/null disable=SC1091
. /work/common.sh

# The package declares `Depends: systemd`, and dpkg enforces that at configure time, so the
# dependency has to be present for the install to be a real install. `--force-depends` here
# would test a package nobody could install, and skipping the dependency would hide the case
# where the declared dependency is wrong.
if ! dpkg -s systemd >/dev/null 2>&1; then
  apt-get update -qq >/dev/null 2>&1 || true
  apt-get install -y -qq --no-install-recommends systemd >/dev/null 2>&1 || {
    echo 'package-verify: could not install systemd, the declared dependency, in this image' >&2
    exit 1
  }
fi

dpkg -i /work/plock.deb >/tmp/dpkg.log 2>&1 || {
  echo 'package-verify: dpkg -i failed:' >&2
  cat /tmp/dpkg.log >&2
  exit 1
}
# A second install over the top: a path that only works from nothing is not an upgrade path,
# it is a first install that happens to be forgiving.
dpkg -i /work/plock.deb >/tmp/dpkg2.log 2>&1 || {
  echo 'package-verify: reinstalling the same .deb failed:' >&2
  cat /tmp/dpkg2.log >&2
  exit 1
}

echo "VERSION $(plock version)"
printf 'UNIT %s\n' "$(sha256sum /etc/systemd/system/plock.service | cut -d' ' -f1)"
printf 'BINARY %s\n' "$(sha256sum /usr/local/bin/plock | cut -d' ' -f1)"

for p in $(dpkg -L plock | LC_ALL=C sort); do
  [ -e "$p" ] || continue
  is_scaffolding "$p" && continue
  printf 'FILE %s %s\n' "$(stat -c '%a' "$p")" "$p"
done

report_enabled
plant_state

# An upgrade, with a family's state in place. The reinstall above only proves the second
# install does not fail; this proves it does not touch what the family depends on.
#
# The .deb's postrm runs with `upgrade` as its argument on this path and `purge` is the only
# branch that deletes, so a dropped argument would keep the state for the wrong reason --
# which is why the sentinels are re-read rather than merely re-stat'd, and why the erase note
# must not appear in this log.
#
# /var/lib/plock is chmodded to a mode the package never chose: %post creates directories only
# when absent, and a scriptlet that re-chmodded an existing one would be the arbitrary-chmod
# primitive the spec's comment refuses to be.
chmod 0701 /var/lib/plock
dpkg -i /work/plock.deb >/tmp/dpkg-u.log 2>&1 || {
  echo 'package-verify: upgrading over an installed .deb failed:' >&2
  cat /tmp/dpkg-u.log >&2
  exit 1
}
report_state UPGRADE
report_sentinels
printf 'KEPTMODE %s\n' "$(stat -c '%a' /var/lib/plock)"
printf 'ERASENOTE %s\n' "$(grep -c 'left in place' /tmp/dpkg-u.log || true)"

# remove, then purge. The two are different promises and the .deb has to keep both: remove
# leaves every byte of state, purge takes the policy and the key and leaves the trail.
dpkg -r plock >/tmp/dpkg-r.log 2>&1 || {
  echo 'package-verify: dpkg -r failed:' >&2
  cat /tmp/dpkg-r.log >&2
  exit 1
}
report_state REMOVE
dpkg -P plock >/tmp/dpkg-p.log 2>&1 || {
  echo 'package-verify: dpkg -P failed:' >&2
  cat /tmp/dpkg-p.log >&2
  exit 1
}
report_state PURGE
PROBE

cat >"$work/probe-rpm.sh" <<'PROBE'
set -eu
# shellcheck source=/dev/null disable=SC1091
. /work/common.sh

# Same rule as the .deb probe: the declared dependency is installed rather than worked
# around, so `rpm -i` checks the dependency the operator's machine will check.
rpm -q systemd >/dev/null 2>&1 || dnf -q -y install systemd >/dev/null 2>&1 || {
  echo 'package-verify: could not install systemd, the declared dependency, in this image' >&2
  exit 1
}

rpm -i /work/plock.rpm >/tmp/rpmi.log 2>&1 || {
  echo 'package-verify: rpm -i failed:' >&2
  cat /tmp/rpmi.log >&2
  exit 1
}

echo "VERSION $(plock version)"
printf 'UNIT %s\n' "$(sha256sum /etc/systemd/system/plock.service | cut -d' ' -f1)"
printf 'BINARY %s\n' "$(sha256sum /usr/local/bin/plock | cut -d' ' -f1)"

# rpm -qlp --dump lists the directories in the payload too, which -ql alone omits unless the
# spec says %dir; taking the union means the two formats report the same shape.
{
  rpm -ql plock
  rpm -qlp --dump /work/plock.rpm | awk '{print $1}' | grep '/$' || true
} | LC_ALL=C sort -u | while IFS= read -r p; do
  [ -e "$p" ] || continue
  is_scaffolding "$p" && continue
  printf 'FILE %s %s\n' "$(stat -c '%a' "$p")" "$p"
done

report_enabled
plant_state

# The upgrade case -- the one the spec's `$1` branches exist for and this gate never ran.
# %preun stops and disables only on an erase ($1 = 0); stopping on an upgrade would unlock
# every screen the daemon is holding because a package was updated. %postun prints its
# "left in place" note only on an erase too, so the note's absence here is the observable
# proof that the erase branch did not fire.
#
# `rpm -U` of an identical version refuses, so the same bytes go back with --force: what is
# under test is the scriptlet argument, not rpm's version comparison. Same two extra claims as
# the .deb probe, for the same reasons -- the sentinels re-read, and a foreign mode on a
# directory the package did not create.
chmod 0701 /var/lib/plock
rpm -Uvh --force /work/plock.rpm >/tmp/rpmu.log 2>&1 || {
  echo 'package-verify: rpm -U --force, the upgrade path, failed:' >&2
  cat /tmp/rpmu.log >&2
  exit 1
}
report_state UPGRADE
report_sentinels
printf 'KEPTMODE %s\n' "$(stat -c '%a' /var/lib/plock)"
printf 'ERASENOTE %s\n' "$(grep -c 'left in place' /tmp/rpmu.log || true)"

# rpm has no purge, so erase is the only teardown, and it must keep everything: see the note
# in deploy/packaging/plock.spec.
rpm -e plock >/tmp/rpme.log 2>&1 || {
  echo 'package-verify: rpm -e failed:' >&2
  cat /tmp/rpme.log >&2
  exit 1
}
report_state ERASE
PROBE

printf 'package-verify: %s in %s\n' "$DEB" "$DEB_IMAGE"
$RUNTIME run --rm --user 0 -v "$work":/work:ro "$DEB_IMAGE" sh /work/probe-deb.sh >"$work/deb.txt"

printf 'package-verify: %s in %s\n' "$RPM" "$RPM_IMAGE"
# The image has rpm, but not always cpio for the --dump query, so the tooling is installed
# rather than assumed.
$RUNTIME run --rm --user 0 -v "$work":/work "$RPM_IMAGE" \
  sh -c "dnf -q -y install cpio coreutils findutils >/dev/null 2>&1 && sh /work/probe-rpm.sh" \
  >"$work/rpm.txt"

# ---------------------------------------------------------------------------
# the comparison
# ---------------------------------------------------------------------------
fail=0
check() {
  if [ "$2" = "$3" ]; then
    printf '  ok   %s\n' "$1"
  else
    printf '  FAIL %s\n       deb: %s\n       rpm: %s\n' "$1" "$2" "$3"
    fail=$((fail + 1))
  fi
}

# stat prints 700; the expectations below are written 0700 the way a mode is written in
# prose. Compared as numbers so the two spellings cannot disagree.
check_mode() {
  label=$1
  path=$2
  want=$3
  file=$4
  got=$(grep "^FILE [0-7]* $path\$" "$file" | head -1 | cut -d' ' -f2)
  # stat prints 700; the expectation is written 0700 the way a mode is written in prose, so
  # the prose zero comes off before comparing. `$((8#700))` would be tidier, but a base
  # prefix inside arithmetic is a bashism and this script runs under dash.
  if [ "$got" = "${want#0}" ]; then
    printf '  ok   %s is %s in the %s\n' "$path" "$want" "$label"
  else
    printf '  FAIL %s is %s in the %s, want %s\n' "$path" "${got:-missing}" "$label" "$want"
    fail=$((fail + 1))
  fi
}

state_is() {
  label=$1
  file=$2
  teardown=$3
  path=$4
  want=$5
  if grep -q "^$teardown $path=$want\$" "$file"; then
    printf '  ok   %s\n' "$label"
  else
    printf '  FAIL %s (wanted %s=%s)\n' "$label" "$path" "$want"
    grep "^$teardown " "$file" | sed 's/^/       /'
    fail=$((fail + 1))
  fi
}

field() { grep "^$1 " "$2" | head -1 | cut -d' ' -f2-; }

echo "comparing what each package installed:"
check "the two packages contain the same binary" "$(field BINARY "$work/deb.txt")" "$(field BINARY "$work/rpm.txt")"
check "the unit file is identical" "$(field UNIT "$work/deb.txt")" "$(field UNIT "$work/rpm.txt")"
check "the version string matches" "$(field VERSION "$work/deb.txt")" "$(field VERSION "$work/rpm.txt")"
check "the .deb did not enable the service" "$(field ENABLED "$work/deb.txt")" "no"
check "the rpm did not enable the service" "$(field ENABLED "$work/rpm.txt")" "no"

grep '^FILE ' "$work/deb.txt" | sort >"$work/deb-files"
grep '^FILE ' "$work/rpm.txt" | sort >"$work/rpm-files"
if diff -u "$work/deb-files" "$work/rpm-files" >"$work/files.diff"; then
  printf '  ok   the two packages own the same %s paths with the same modes\n' \
    "$(wc -l <"$work/deb-files" | tr -d ' ')"
else
  printf '  FAIL the payloads differ:\n'
  sed 's/^/       /' "$work/files.diff"
  fail=$((fail + 1))
fi

# The security-relevant modes, asserted rather than merely equal: two formats agreeing on a
# wrong mode is still wrong. /etc/plock at 0755 leaks which accounts have profiles even
# though the config inside it is 0600.
for pair in "0700 /etc/plock" "0755 /usr/local/bin/plock" "0644 /etc/systemd/system/plock.service"; do
  want=${pair%% *}
  path=${pair#* }
  check_mode deb "$path" "$want" "$work/deb-files"
  check_mode rpm "$path" "$want" "$work/rpm-files"
done

# The erase-safety property. Neither package may *own* a state directory -- owning one means
# the package deletes an audit trail when it is removed -- and each format then keeps the
# promises its own teardown words make.
if grep -q '/var/lib/plock\|/var/log/plock' "$work/deb-files" "$work/rpm-files"; then
  printf '  FAIL a package owns a state directory; removing it would delete an audit trail\n'
  grep '/var/lib/plock\|/var/log/plock' "$work/deb-files" "$work/rpm-files" | sed 's/^/       /'
  fail=$((fail + 1))
else
  printf '  ok   neither package owns /var/lib/plock or /var/log/plock\n'
fi

echo "what each teardown left behind:"
# deb: `remove` keeps everything a family would want back after a reinstall.
state_is 'dpkg -r kept the policy' "$work/deb.txt" REMOVE etc/plock/config.json exists
state_is 'dpkg -r kept the key' "$work/deb.txt" REMOVE var/lib/plock/plock.key exists
state_is 'dpkg -r kept the trail' "$work/deb.txt" REMOVE var/log/plock/plock.log exists
# deb: `purge` means the policy and the credential go, and the trail is the one thing that
# must not.
state_is 'dpkg -P took the policy' "$work/deb.txt" PURGE etc/plock/config.json absent
state_is 'dpkg -P took the key' "$work/deb.txt" PURGE var/lib/plock/plock.key absent
state_is 'dpkg -P kept the audit trail' "$work/deb.txt" PURGE var/log/plock/plock.log exists
# rpm: erase is the only teardown and there is no word for "and delete my data".
state_is 'rpm -e kept the policy' "$work/rpm.txt" ERASE etc/plock/config.json exists
state_is 'rpm -e kept the key' "$work/rpm.txt" ERASE var/lib/plock/plock.key exists
state_is 'rpm -e kept the audit trail' "$work/rpm.txt" ERASE var/log/plock/plock.log exists

# An upgrade is the third promise, and the one neither format used to be asked about: a
# package update must not touch a family's rules, its key or its trail.
echo "and what an upgrade left alone:"
for f in deb rpm; do
  # `what`, not `label`: state_is and check_mode both assign to a global `label`, so a loop
  # variable with that name gets rewritten by the first call and every later line in this
  # section is printed with the previous verdicts glued onto it.
  case $f in
    deb) what='dpkg -i over an installed .deb' ;;
    rpm) what='rpm -U --force over an installed .rpm' ;;
  esac
  state_is "$what kept the policy" "$work/$f.txt" UPGRADE etc/plock/config.json exists
  state_is "$what kept the key" "$work/$f.txt" UPGRADE var/lib/plock/plock.key exists
  state_is "$what kept the audit trail" "$work/$f.txt" UPGRADE var/log/plock/plock.log exists
  for p in etc/plock/config.json var/lib/plock/plock.key var/log/plock/plock.log; do
    if grep -q "^SENTINEL $p=kept\$" "$work/$f.txt"; then
      printf '  ok   %s still holds the bytes it planted (%s)\n' "$p" "$f"
    else
      printf '  FAIL %s was replaced, not kept, by the %s upgrade\n' "$p" "$f"
      fail=$((fail + 1))
    fi
  done
  # 0701 is the mode the probe put there, not one either package knows about. Anything else
  # means a scriptlet chmodded a directory it did not create.
  check "$what left a foreign mode on /var/lib/plock alone" 701 "$(field KEPTMODE "$work/$f.txt")"
  # The erase note is the observable half of the `$1` branch: if it printed during an upgrade,
  # the scriptlet took the erase path and unlocked the screens with it.
  check "$what did not take the erase branch" 0 "$(field ERASENOTE "$work/$f.txt")"
done
if [ "$fail" -ne 0 ]; then
  printf 'package-verify: %d problem(s)\n' "$fail" >&2
  exit 1
fi
printf 'package-verify: both packages install the same program in the same places\n'
