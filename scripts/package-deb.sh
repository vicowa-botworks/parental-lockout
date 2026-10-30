#!/bin/sh
# package-deb.sh -- wrap a staged tree into a .deb.
#
#   scripts/package-deb.sh <staged-root> <out.deb>
#
# The payload is whatever deploy/install.sh staged, unchanged. This script adds only the
# `DEBIAN/` metadata directory, which is why the file list is not written here: the
# installer is the single answer to "where does plock live", and scripts/package-verify.sh
# is what checks the two formats agree.
set -eu

ROOT=${1:-}
OUT=${2:-}

die() {
  printf 'package-deb: %s\n' "$*" >&2
  exit 1
}

if [ -z "$ROOT" ] || [ -z "$OUT" ]; then
  die "usage: $0 <staged-root> <out.deb>"
fi
[ -d "$ROOT" ] || die "no such staging root: $ROOT"
[ -f "$ROOT/DEBIAN/control" ] && die "$ROOT already has a DEBIAN directory; stage it clean"

# shellcheck source=scripts/package-lib.sh disable=SC1091
. "$(dirname "$0")/package-lib.sh"

command -v dpkg-deb >/dev/null 2>&1 || die "no dpkg-deb; install dpkg (or build the .deb in a container)"

ARCH=${PKG_ARCH:-amd64}
VERSION=$(deb_version)
HOMEPAGE=${PKG_HOMEPAGE:-https://github.com/vicowa-botworks/parental-lockout}
MAINTAINER=${PKG_MAINTAINER:-vicowa-botworks <vicowa-botworks@users.noreply.github.com>}
SIZE=$(payload_size "$ROOT")

# The metadata directory is created inside the root because that is the shape dpkg-deb
# builds: everything under the root, except DEBIAN/, is the payload.
mkdir -p "$ROOT/DEBIAN"

# `Depends: systemd` is honest rather than defensive: this package installs a unit file and
# its scriptlets talk to systemctl. A system without it cannot run plock as a service, and
# finding that out at install time is better than at first boot.
#
# There is no `conffiles`: the package ships no configuration. /etc/plock is an empty
# directory whose contents are created by `plock init`, and declaring a conffile that does
# not exist is how a package ends up "repairing" a policy file on upgrade.
cat >"$ROOT/DEBIAN/control" <<CONTROL
Package: plock
Version: $VERSION
Architecture: $(deb_arch "$ARCH")
Maintainer: $MAINTAINER
Homepage: $HOMEPAGE
Description: Parental screen-lockout daemon
 plock enforces lockout periods on the accounts of a shared Linux desktop: it holds a
 child's session locked outside the hours the operator allows, re-asserting the lock on
 every tick so nothing the controlled account can write decides when enforcement stops.
 .
 The daemon runs as root precisely so the controlled account cannot turn it off.
Section: admin
Priority: optional
Installed-Size: $SIZE
Depends: systemd
CONTROL

# The scriptlets are checked into deploy/packaging/deb/ rather than generated here. They
# run as root on someone's family machine, they are the only part of this that takes
# action on an existing system, and a reviewer should be able to read them as they ship
# rather than as the shell that emits them happens to write them.
for s in postinst prerm postrm; do
  src=$(dirname "$0")/../deploy/packaging/deb/$s
  [ -f "$src" ] || die "missing maintainer script $src"
  install -m 0755 "$src" "$ROOT/DEBIAN/$s"
done

# md5sums is not required by dpkg, but it is what makes `debsums` able to answer "has
# anything on this machine changed the daemon's files" -- a question a locking daemon is
# entitled to be asked.
(
  cd "$ROOT"
  find . -type f ! -path './DEBIAN/*' -printf '%P\n' | LC_ALL=C sort |
    while IFS= read -r f; do
      [ -n "$f" ] || continue
      md5sum -- "$f"
    done >DEBIAN/md5sums
)

# SOURCE_DATE_EPOCH clamps every mtime in the archive and the changelog-free date dpkg
# would otherwise take from the clock, so two builds of one commit produce one file.
: "${SOURCE_DATE_EPOCH=$(git log -1 --format=%ct 2>/dev/null || echo 0)}"
export SOURCE_DATE_EPOCH

mkdir -p "$(dirname "$OUT")"
rm -f "$OUT"

# --root-owner-group: the payload must be root:root in the archive, not whatever uid built
# it. Without it an unprivileged build records the builder's uid and the installed binary
# belongs to a user account that does not exist on the target.
#
# -Zgzip -z9 --uniform-compression: dpkg's defaults have changed between releases, so
# pinning them is what keeps a rebuilt artifact byte-identical rather than merely
# equivalent.
dpkg-deb --root-owner-group --uniform-compression -Zgzip -z9 --build "$ROOT" "$OUT" >/dev/null

printf 'package-deb: wrote %s (%s, %s bytes)\n' "$OUT" "$VERSION" "$(wc -c <"$OUT" | tr -d ' ')"
