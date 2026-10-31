#!/bin/sh
# package-rpm.sh -- build the .rpm from a staged tree.
#
#   scripts/package-rpm.sh <staged-root> <out.rpm>
#
# Environment:
#   PKG_ARCH          amd64|x86_64 or arm64|aarch64 (default amd64)
#   PKG_VERSION       the version to stamp (see scripts/package-lib.sh)
#   RPM_BUILD_IMAGE   the image to build in when this machine has no rpmbuild
#                     (default fedora:43)
#
# rpmbuild is not installed on a Debian-family developer machine, and asking everyone who
# wants to check a package to install the other distribution's toolchain is a bad trade. So
# the build runs in a container when it has to. The spec is the same file either way, which
# is the only reason that is safe: two different recipes would mean the artifact nobody
# tested locally is the one that ships.
set -eu

ROOT=${1:-}
OUT=${2:-}

die() {
  printf 'package-rpm: %s\n' "$*" >&2
  exit 1
}

if [ -z "$ROOT" ] || [ -z "$OUT" ]; then
  die "usage: $0 <staged-root> <out.rpm>"
fi
[ -d "$ROOT" ] || die "no such staging root: $ROOT"

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/.." && pwd)
# shellcheck source=scripts/package-lib.sh disable=SC1091
. "$here/package-lib.sh"

ARCH=${PKG_ARCH:-amd64}
SPEC=$repo/deploy/packaging/plock.spec
[ -f "$SPEC" ] || die "no spec at $SPEC"

# rpmbuild refuses a relative _topdir and a relative payload, so everything it is handed is
# absolute before the first call.
ROOT=$(cd "$ROOT" && pwd)
case "$OUT" in
  /*) ;;
  *) OUT=$PWD/$OUT ;;
esac

VERSION=$(rpm_version)
RELEASE=$(rpm_release)
RPM_ARCH=$(rpm_arch "$ARCH")

# rpm will not build for a foreign architecture -- `error: No compatible architectures found for
# build`, measured on the first arm64 run. Neither `--target aarch64` nor rpm's `_cross_platform`
# macro lifts it: the compat table in rpmrc does not put aarch64 and x86_64 in each other's list,
# and distros solve this by building in a chroot or on a machine of the target arch. So this does
# the same instead of pretending, and refuses here rather than dying halfway through a build after
# the .deb for the same architecture has already been written and looked like a success.
if [ "$RPM_ARCH" != "$(rpm_arch "$(uname -m)")" ]; then
  die "cannot build an rpm for $RPM_ARCH on $(uname -m): rpm builds only for the machine it runs on.
     Build it on the target architecture -- which is what the release workflow does, one job per
     architecture on a runner of that architecture -- or use --format deb --arch $ARCH, which
     crosses fine."
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/plock-rpm-XXXXXX")
# The tree is left behind on failure on purpose: an rpm build that fails halfway is a
# question about the build root, and rebuilding to reproduce it loses the answer.
trap 'printf "package-rpm: build root kept at %s\n" "$WORK" >&2' EXIT

mkdir -p "$WORK/rpmbuild/BUILD" "$WORK/rpmbuild/RPMS" "$WORK/rpmbuild/SOURCES" \
  "$WORK/rpmbuild/SPECS" "$WORK/rpmbuild/SRPMS"

# The payload is copied into the build root rather than referenced where it lies. Two
# reasons: a container build can only see what is mounted, and the tree left behind on a
# failure has to be the whole input -- a build root you can re-run by hand is the difference
# between diagnosing an rpm failure and guessing at it.
cp -a "$ROOT" "$WORK/payload"

# SOURCE_DATE_EPOCH is what rpmbuild stamps into the build time, so a rebuild of one commit
# does not differ by the clock.
: "${SOURCE_DATE_EPOCH=$(git log -1 --format=%ct 2>/dev/null || echo 0)}"
export SOURCE_DATE_EPOCH

# The build is driven by a generated script rather than by an inline command.
#
# `--define` takes ONE argument, `"name body"`: `--define _topdir /path` defines `_topdir`
# with an empty body and hands the path to rpm as a file to build, which is the otherwise
# inscrutable `Macro %_topdir has empty body`. Getting that single argument through a
# `sh -c` string into a container means quoting that survives two shells, and `--macros`
# is not an escape -- despite its help text it *replaces* the standard macro files, so
# `%{_builddir}` and `%{_dbpath}` vanish. A script written into the build root survives no
# quoting at all, is the same script on both paths, and is sitting there to re-run when a
# build fails.
write_build_script() {
  cat >"$WORK/run-rpmbuild.sh" <<EOF
#!/bin/sh
set -eu
if ! command -v rpmbuild >/dev/null 2>&1; then
  dnf -q -y install rpm-build >/dev/null 2>&1 || {
    echo 'package-rpm: no rpmbuild and dnf could not install it' >&2
    exit 1
  }
fi
# errexit is off across the build itself: the chown has to happen on the failure path too,
# or a failed build leaves the caller unable to delete their own build root.
set +e
rpmbuild \\
  --define "_topdir $1" \\
  --define "plock_version $VERSION" \\
  --define "plock_release $RELEASE" \\
  --define "plock_arch $RPM_ARCH" \\
  --define "plock_payload $2" \\
  -bb "$3"
rc=\$?
# The build runs as root because rpm has to be root to record root ownership. It must not
# *finish* as root: the artifact lands in a directory the calling user owns, and a root-owned
# .rpm in a developer's dist/ is an unremovable file and a failed build for whoever runs it
# as themselves. PKLOCK_CHOWN is the caller's uid:gid.
if [ -n "\${PKLOCK_CHOWN:-}" ]; then
  chown -R "\${PKLOCK_CHOWN}" "$1/.." 2>/dev/null || true
fi
exit \$rc
EOF
}

if command -v rpmbuild >/dev/null 2>&1; then
  printf 'package-rpm: rpmbuild %s (%s-%s)\n' "$VERSION" "$RELEASE" "$RPM_ARCH"
  write_build_script "$WORK/rpmbuild" "$WORK/payload" "$SPEC"
  sh "$WORK/run-rpmbuild.sh" >/dev/null
elif command -v docker >/dev/null 2>&1; then
  image=${RPM_BUILD_IMAGE:-fedora:43}
  printf 'package-rpm: no rpmbuild here; building in %s\n' "$image"
  # The repo is mounted read-only: a build that could rewrite the source tree it is
  # packaging is not a build, it is a build system that lies about what it built.
  #
  # --user 0 because rpm needs to record root ownership and chown inside its build root;
  # an unprivileged container cannot, and the resulting package would carry the builder's
  # uid.
  write_build_script /work/rpmbuild /work/payload /src/deploy/packaging/plock.spec
  docker run --rm --user 0 -e PKLOCK_CHOWN="$(id -u):$(id -g)" \
    -v "$repo":/src:ro -v "$WORK":/work "$image" \
    sh /work/run-rpmbuild.sh >/dev/null
else
  die "no rpmbuild and no docker; install rpm-build or point RPM_BUILD_IMAGE at a build image"
fi

built=$(find "$WORK/rpmbuild/RPMS" -name 'plock-*.rpm' -type f | head -1)
[ -n "$built" ] || die "the build produced no rpm under $WORK/rpmbuild/RPMS"

mkdir -p "$(dirname "$OUT")"
rm -f "$OUT"
mv "$built" "$OUT"

trap - EXIT
rm -rf "$WORK"

printf 'package-rpm: wrote %s (%s bytes)\n' "$OUT" "$(wc -c <"$OUT" | tr -d ' ')"
