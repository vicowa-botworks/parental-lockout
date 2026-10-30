# shellcheck shell=sh
# package-lib.sh -- the parts of packaging that both formats must agree on.
#
# Sourced by scripts/package.sh, scripts/package-deb.sh and scripts/package-rpm.sh.
# Not meant to be run, which is why there is no shebang: a library that can be executed is a
# library whose functions someone will eventually run without the caller's guards.
#
# Everything here exists so that "the .deb and the .rpm contain the same program in the
# same places" is one definition rather than two. The formats differ in their metadata
# and in their scriptlets; they must not differ in their payload, because a family whose
# daemon behaves differently depending on whether the operator typed `apt install` or
# `dnf install` is a bug nobody will connect to the installer.

# ---------------------------------------------------------------------------
# versions
# ---------------------------------------------------------------------------

# pkg_version prints the version to stamp, from PKG_VERSION or from git.
#
# A release artifact built from a dirty tree is a lie about what is on the operator's
# machine: `plock version` would report a commit that does not contain the code that is
# locking their children out. So a dirty tree is refused, not sanitised. ALLOW_DIRTY=1
# exists for a developer who wants a throwaway package and is about to delete it.
pkg_version() {
  if [ -n "${PKG_VERSION:-}" ]; then
    printf '%s\n' "$PKG_VERSION"
    return 0
  fi
  if [ ! -d .git ] && ! git rev-parse --git-dir >/dev/null 2>&1; then
    die "no PKG_VERSION and this is not a git checkout; set PKG_VERSION=x.y.z"
  fi
  if [ -z "${ALLOW_DIRTY:-}" ] && [ -n "$(git status --porcelain 2>/dev/null)" ]; then
    die "refusing to package a dirty tree (it would stamp a commit that does not contain this code).
     Commit it, or set ALLOW_DIRTY=1 for a throwaway build."
  fi
  # --tags so a tag is the version when there is one; --always so an untagged tree still
  # produces something; --dirty so the refusal above is not the only thing standing
  # between a modified tree and a stamped artifact.
  git describe --tags --always --dirty 2>/dev/null || die "git describe failed"
}

# is_tagged says whether the version came from a tag.
#
# It is the difference between two kinds of odd version string. A tag named `v1.0rc1` is
# somebody's decision and deserves a refusal, because rpm would sort it against `1.0.1` in
# a way that surprises whoever writes the upgrade. An untagged tree is nobody's decision --
# it is a checkout between releases, and this repo had no tags at all for its first two
# hundred commits -- so it gets a placeholder rather than an error.
#
# The obvious test, "does it start with a digit", is wrong: `git describe --always` falls
# back to the short sha, and half of all shas start with a digit. So the question is asked
# of git instead -- an untagged describe is exactly HEAD's short sha, and anything else has
# a tag name in front of it.
is_tagged() {
  if [ -n "${PKG_VERSION:-}" ]; then
    # An explicit PKG_VERSION is a release claim: numeric means "treat it as a tag", and
    # check_version refuses the rest rather than guessing.
    case "${PKG_VERSION#v}" in
      [0-9]*) return 0 ;;
      *) return 1 ;;
    esac
  fi
  d=$(pkg_version)
  d=${d%-dirty}
  [ "$d" = "$(git rev-parse --short HEAD 2>/dev/null)" ] && return 1
  case "$d" in
    *-g[0-9a-f]*) return 0 ;; # <tag>-<n>-g<sha>: a tag is reachable
    *[!0-9a-f]*) return 0 ;;  # contains a dot or a letter beyond hex: a tag name
  esac
  return 1
}

# deb_version prints the same version in the form dpkg accepts.
#
# dpkg's version field is `epoch:upstream-revision` and a hyphen is the delimiter
# between upstream and revision, so `v0.3.0-12-gabc1234` from git describe cannot go in
# as written: dpkg would read upstream `0.3.0` and revision `12-gabc1234`, and the
# revision would then sort wrongly against the next build. `+` is legal in both halves
# and is what Debian's own git-to-version convention uses, so every hyphen becomes one.
# The leading `v` is a tag prefix, not part of the version.
deb_version() {
  raw=$(pkg_version | sed -e 's/^v//')
  if is_tagged; then
    printf '%s\n' "$raw" | sed -e 's/-/+/' -e 's/[-_]/./g'
  else
    printf '0.0.0+%s\n' "$(printf '%s' "$raw" | tr -c 'A-Za-z0-9.+' '.')"
  fi
}

# rpm_version prints the numeric head; rpm_release carries everything else.
#
# rpm disallows a hyphen in Version for the same reason dpkg does. The commit count and
# sha that git describe appends go to Release rather than being flattened into Version,
# because rpm compares Version with its own rpmvercmp and a `+gabc1234` tail sorts
# against a real release in a way that surprises whoever is writing an upgrade.
rpm_version() {
  base=$(pkg_version | sed -e 's/^v//' -e 's/[-_].*$//')
  if is_tagged; then
    printf '%s\n' "$base"
  else
    printf '0.0.0\n'
  fi
}

rpm_release() {
  v=$(pkg_version | sed -e 's/^v//')
  if is_tagged; then
    tail=$(printf '%s\n' "$v" | sed -e 's/^[0-9][0-9.]*//' -e 's/^[-_]//' -e 's/[-_]/./g')
    if [ -z "$tail" ]; then
      printf '1\n'
    else
      printf '1.%s\n' "$tail"
    fi
  else
    # Release 0.* : an untagged build must sort below `1` of the same version, or a
    # developer's build would look like an upgrade to the machine it is installed on. The
    # whole describe goes in -- there is no numeric head to strip when there is no tag.
    #
    # printf '%s', not '%s\n': tr's complement set includes the newline, so a trailing
    # format newline becomes a trailing dot in the version. And `\.*`, not `\.+` -- in a BRE
    # an unescaped `+` is a literal plus, so `\.+$` matches nothing and the dot survives.
    tail=$(printf '%s' "$v" | tr -c 'A-Za-z0-9.+' '.')
    printf '0.untagged.%s\n' "$tail"
  fi
}

# check_version refuses a version the tools would silently mangle. Both formats accept a
# narrower character set than git describe can produce -- a tag named `v1.0 rc1` has a
# space, and dpkg would take it and rpm would reject it, so the two packages would be
# built from two different strings. Better to stop here than to find out on an operator.
check_version() {
  v=$(deb_version)
  case "$v" in
    '' | *[!A-Za-z0-9.+~:]*)
      die "version '$v' is not a valid dpkg version (letters, digits and . + ~ : only)"
      ;;
  esac
  rv=$(rpm_version)
  case "$rv" in
    '' | *[!A-Za-z0-9.+_~]*)
      die "rpm Version '$rv' is not a valid rpm Version"
      ;;
  esac
  # A tag that is not purely numeric is the case worth refusing: it is a decision somebody
  # made, and rpmvercmp compares a letter run and a digit run differently, so `1.0rc1`
  # sorts *above* `1.0.1`. An untagged tree is already handled by the 0.0.0 placeholder.
  if is_tagged; then
    case "$rv" in
      *[A-Za-z]*)
        die "tag '$(pkg_version)' is not a numeric rpm Version (rpmvercmp would sort it against later releases unpredictably)"
        ;;
    esac
  fi
}

# ---------------------------------------------------------------------------
# architectures
# ---------------------------------------------------------------------------

# go_arch maps the package architecture name to GOARCH.
go_arch() {
  case "$1" in
    amd64 | x86_64) printf 'amd64\n' ;;
    arm64 | aarch64) printf 'arm64\n' ;;
    *) die "unknown architecture '$1'" ;;
  esac
}

# deb_arch and rpm_arch print the same machine's name in each format's spelling.
deb_arch() {
  case "$1" in
    amd64 | x86_64) printf 'amd64\n' ;;
    arm64 | aarch64) printf 'arm64\n' ;;
    *) die "unknown architecture '$1'" ;;
  esac
}

rpm_arch() {
  case "$1" in
    amd64 | x86_64) printf 'x86_64\n' ;;
    arm64 | aarch64) printf 'aarch64\n' ;;
    *) die "unknown architecture '$1'" ;;
  esac
}

# ---------------------------------------------------------------------------
# the payload
# ---------------------------------------------------------------------------

# stage builds the binary and lays it out with deploy/install.sh, into $1, for arch $2.
#
# The staging is done by the installer rather than by this script, and that is the whole
# design: install.sh already knows where plock lives, what mode each piece has, and which
# directories the unit expects. A packager that copied files by hand would be a second
# answer to those questions, and the two would drift the first time a path moved. The
# DESTDIR support in install.sh exists for exactly this -- it rebases every path and
# never calls systemctl.
#
# $3 is the binary to stage; when empty the binary is built here.
stage() {
  root=$1
  arch=$2
  bin=$3

  # The build's umask is part of the artifact, so it is pinned rather than inherited.
  #
  # `install -d` without -m, and the plain `mkdir` calls below, take the mode of every parent
  # directory in the payload from the environment: under umask 002 the staged `./etc/` and
  # `./usr/` are 0775, under 022 they are 0755, and the two `.deb` files of one commit then
  # have different sha256. Measured on 30 Oct: same commit, same SOURCE_DATE_EPOCH,
  # `umask 002` → 74f5466…, `umask 022` → 03a3ad6…, and repeating 002 reproduced the first.
  #
  # Nothing here can catch it afterwards. dpkg-deb records the tree it is handed, rpm builds
  # its own parents from %files so the .rpm never showed the drift, and package-verify.sh
  # filters exactly these scaffolding directories out of its FILE comparison -- so the gate
  # that exists to prove the two formats agree was blind to the one input they disagreed on.
  # Cosmetic on install (everything is root:root and a pre-existing directory keeps its own
  # mode), but SHA256SUMS is a promise about bytes, and a promise that holds only under one
  # umask is not a promise.
  umask 022

  rm -rf "$root"
  mkdir -p "$root"

  if [ -z "$bin" ]; then
    mkdir -p "$root/.build"
    # CGO_ENABLED=0: a static binary has no libc to mismatch with the target
    # distribution's, which is the difference between a package that installs on Debian 12
    # and one that segfaults on RHEL 9 because a glibc symbol moved.
    #
    # The stamp variables are pinned here rather than left to the Makefile's `?= defaults,
    # because two builds of one commit must produce one artifact -- and the Makefile's
    # default DATE is the wall clock. A package whose `plock version` reports the minute it
    # happened to be compiled is both unreproducible and a lie about the code: the honest
    # build date for a release artifact is the commit's own date.
    : "${SOURCE_DATE_EPOCH=$(git log -1 --format=%ct 2>/dev/null || echo 0)}"
    export SOURCE_DATE_EPOCH
    VERSION=$(pkg_version)
    COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo none)
    DATE=$(date -u -d "@$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)
    export VERSION COMMIT DATE
    CGO_ENABLED=0 GOOS=linux GOARCH=$(go_arch "$arch") \
      make --no-print-directory build BIN="$root/.build/plock"
    bin=$root/.build/plock
  fi

  [ -x "$bin" ] || die "binary $bin is not executable"
  # BIN_ARCH is what lets the installer check a cross-built binary at all: it cannot exec an
  # aarch64 program on an x86_64 box, so without it the arm64 package could not be staged. With
  # it, install.sh checks the ELF header against the architecture this package claims, which is
  # the assertion the exec was standing in for -- and a more specific one.
  DESTDIR="$root" BIN_SRC="$bin" BIN_ARCH="$arch" deploy/install.sh >/dev/null

  # The build artefact is not part of the payload, and leaving it under the staging root
  # would ship a second copy of the program at /usr/local/bin/.build/plock.
  rm -rf "$root/.build"

  # The state and log directories are created by install.sh so the modes are right before
  # the first start. They are then taken back out of the payload, because a package that
  # owns a directory holding an audit trail and a private key is a package that deletes
  # them on erase.
  #
  # Creating them is still the right thing, so the post-install scriptlets do it -- with
  # the same modes, and only when absent. That is the difference between "the installer
  # made sure" and "the package owns your evidence".
  rmdir "$root/var/lib/plock" "$root/var/log/plock" 2>/dev/null ||
    die "$root/var/lib/plock and $root/var/log/plock were not staged empty; the staging step changed"
  # /var/lib and /var/log are now empty scaffolding; dpkg and rpm both record them, and
  # neither distribution wants a third-party package declaring it owns /var/log.
  rmdir "$root/var/lib" "$root/var/log" "$root/var" 2>/dev/null || true
}

# payload_size prints the installed size in KiB, which is what dpkg's Installed-Size and
# rpm's %files size are meant to be.
payload_size() {
  du -sk "$1" 2>/dev/null | cut -f1
}
