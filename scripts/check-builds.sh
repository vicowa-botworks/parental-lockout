#!/bin/sh
# check-builds.sh -- insists the program still compiles for every platform the design claims.
#
# Why this exists: `go vet ./...` and `go test ./...` only ever look at the platform the
# developer is standing on. A file behind //go:build unix is therefore invisible to the gate
# unless the developer happens to be on a unix, and a syscall that exists only on Linux will sit
# in a unix-tagged file, green, until someone tries a build that the tag admits and the syscall
# does not. That is exactly how this repo shipped: internal/audit/posix.go is unix-tagged and
# calls syscall.Fchmodat, which Go's syscall package does not provide on darwin or freebsd, so
# `GOOS=darwin go build ./...` failed while `make verify` was green and docs/PLAN.md said the
# design was abstracted for non-Linux platforms.
#
# Scope, stated so nobody reads more into it than it holds: this checks the BUILD, not the tests.
# The test suites are host-platform code on purpose -- they poke at syscall.Umask, /proc, and
# directory modes, and a portable version of those assertions would test nothing. Cross-built
# binaries are compiled, not run.

set -e

GO=${GO:-go}

# linux/amd64 is the shipping target. darwin and windows are the two ways the abstraction gets
# exercised: darwin is a unix that is not Linux, so it catches a Linux-only syscall hiding behind
# `unix`; windows is not a unix at all, so it catches the !unix fallback drifting out of build.
for target in linux/amd64 darwin/arm64 windows/amd64; do
	os=${target%/*}
	arch=${target#*/}
	printf '  %-16s' "$target"
	if out=$(GOOS=$os GOARCH=$arch "$GO" build ./... 2>&1); then
		echo "ok"
	else
		echo "FAILED"
		echo "$out"
		echo ""
		echo "builds: $os/$arch does not compile."
		echo "If the failing file is behind a build tag, check whether the syscall it uses is"
		echo "Linux-only -- Go's syscall package is not uniform across unixes. The tag should"
		echo "name the platforms where the code actually compiles, and the counterpart file's"
		echo "tag has to be the exact complement or both files drop out and the symbols vanish."
		exit 1
	fi
done

echo "check-builds: all targets compile"
