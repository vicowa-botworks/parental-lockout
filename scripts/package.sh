#!/bin/sh
# package.sh -- build the installable artifacts for this tree.
#
#   scripts/package.sh [--format deb|rpm|all] [--arch amd64|arm64|all] [--out DIR]
#
# Environment:
#   PKG_VERSION   the version to stamp; otherwise `git describe --tags`, and a dirty tree
#                 is refused unless ALLOW_DIRTY=1 (scripts/package-lib.sh)
#   BIN_SRC       an already-built binary to package instead of building one
#   RPM_BUILD_IMAGE  image used for the rpm when rpmbuild is absent (default fedora:43)
#
# What ships, and where, is deploy/install.sh's business. This script's business is only
# the two container formats around it, which is why `make package` cannot invent a third
# layout for the same program.
#
# It is not part of `make verify`, and that is the same split the repo already draws
# between verify and mutate: a gate that needs dpkg, rpmbuild and a container runtime is a
# gate people skip, and the thing it would catch -- a payload that drifted from the
# installer -- is caught by scripts/package-verify.sh in CI on every push.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=scripts/package-lib.sh disable=SC1091
. "$here/package-lib.sh"

FORMAT=all
ARCH=all
OUT=dist
BIN=${BIN_SRC:-}

die() {
  printf 'package: %s\n' "$*" >&2
  exit 1
}

while [ $# -gt 0 ]; do
  case "$1" in
    --format) FORMAT=${2:-}; shift 2 ;;
    --arch) ARCH=${2:-}; shift 2 ;;
    --out) OUT=${2:-}; shift 2 ;;
    --bin) BIN=${2:-}; shift 2 ;;
    -h | --help)
      sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) die "unknown argument '$1' (--help)" ;;
  esac
done

case "$FORMAT" in deb | rpm | all) ;; *) die "--format takes deb, rpm or all" ;; esac

arches=
case "$ARCH" in
  all) arches="amd64 arm64" ;;
  amd64 | x86_64) arches=amd64 ;;
  arm64 | aarch64) arches=arm64 ;;
  *) die "--arch takes amd64, arm64 or all" ;;
esac

check_version
mkdir -p "$OUT"

work=$(mktemp -d "${TMPDIR:-/tmp}/plock-pkg-XXXXXX")
trap 'rm -rf "$work"' EXIT INT TERM

printf 'package: version %s (deb %s, rpm %s-%s)\n' \
  "$(pkg_version)" "$(deb_version)" "$(rpm_version)" "$(rpm_release)"

for arch in $arches; do
  case "$FORMAT" in deb | all)
    root=$work/root-deb-$arch
    stage "$root" "$arch" "$BIN"
    PKG_ARCH=$arch "$here/package-deb.sh" "$root" \
      "$OUT/plock_$(deb_version)_$(deb_arch "$arch").deb"
    rm -rf "$root"
    ;;
  esac
  case "$FORMAT" in rpm | all)
    root=$work/root-rpm-$arch
    stage "$root" "$arch" "$BIN"
    PKG_ARCH=$arch "$here/package-rpm.sh" "$root" \
      "$OUT/plock-$(rpm_version)-$(rpm_release).$(rpm_arch "$arch").rpm"
    rm -rf "$root"
    ;;
  esac
done

# One checksums file per release, in the form `sha256sum -c` reads from the same directory.
# It is written here rather than by the release workflow so that a local `make package`
# produces exactly the set of files a release would, and a discrepancy is visible before
# anything is published.
(
  cd "$OUT"
  find . -maxdepth 1 \( -name '*.deb' -o -name '*.rpm' \) -type f -printf '%P\n' |
    LC_ALL=C sort | xargs -r sha256sum >SHA256SUMS
)

printf 'package: wrote %s\n' "$OUT"
(
  cd "$OUT"
  find . -maxdepth 1 \( -name '*.deb' -o -name '*.rpm' -o -name 'SHA256SUMS' \) -type f \
    -printf '  %P  %s bytes\n' | LC_ALL=C sort
)
