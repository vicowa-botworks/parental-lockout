BIN     := bin/plock
MODULE  := github.com/vicowa-botworks/parental-lockout
PKG     := $(MODULE)/internal/version

GO      ?= go
GOFMT   ?= gofmt
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# The stamp values come from git and, in a release workflow, from a workflow
# input — they are data, never shell. They reach the recipe through the
# environment instead of being interpolated into the recipe text, so no
# metacharacter in a tag name can turn `make build` into code execution.
export PKG VERSION COMMIT DATE

# One -ldflags string, assembled by the shell from those variables. The inner
# double quotes survive into the string so Go's own -ldflags splitter keeps a -X
# value containing a space in one piece; the outer double quotes are what let
# the shell expand the variables in the first place.
LDFLAGS_ASM = ld="-s -w -X \"$${PKG}.Version=$${VERSION}\" -X \"$${PKG}.Commit=$${COMMIT}\" -X \"$${PKG}.Date=$${DATE}\""

.DEFAULT_GOAL := build

.PHONY: build fmt vet test verify check-scripts check-shell check-mutations mutate package package-verify clean install uninstall run-dev

build:
	@mkdir -p bin
	@$(LDFLAGS_ASM); \
	echo "  $(GO) build -trimpath -ldflags \"$$ld\" -o $(BIN) ./cmd/plock"; \
	$(GO) build -trimpath -ldflags "$$ld" -o $(BIN) ./cmd/plock

vet:
	$(GO) vet ./...

test:
	$(GO) test -race ./...

fmt:
	$(GOFMT) -w .

# The single gate. CI, the review loop and the local adversarial reviewer all
# call this and nothing else, so "passes locally" and "passes in CI" cannot
# drift apart.
#
# The attack suite is in here rather than in a job of its own, and the reason is the cost:
# 2.5 seconds measured, against 0.06 s for the self-test it attacks and 3m9s for this target
# overall. `check-mutations` is kept OUT of verify for the opposite reason -- it compiles the
# tree once per mutant and pays minutes, and a gate that slow gets skipped. A suite that runs no
# compiler and no battery has no such excuse, and #124's whole point is that a guard which only
# runs when somebody remembers it is a guard whose evidence nobody can reproduce.
#
# The capture self-test is here for the same reason and a sharper one: it is the ONLY gate that
# can see `testing/capture/*.py` at all. The mutants target `.go` and `.tmpl`, nothing imports
# `pages.py`, and shellcheck does not read Python -- which is how a refusal helper that raised
# from inside its caller's own `except` handler shipped past every check this repository has,
# stopping the capture loop after 1 page of 5 while still exiting 1. It starts no browser and
# writes nothing, so its cost is milliseconds.
#
# The screenshot manifest (#144) is a gate on evidence, not on code: it refuses a committed PNG in
# docs/screenshots/360/ whose page has changed since it was taken. It is in `verify` because the
# failure it prevents is an omission -- the capture driver refuses to write anything when any page
# overflows, so the stale artifact is not a bad write but no write at all, in a PR that still
# merges. The contract line above it is what makes the gate self-pinning: deleting the `check`
# clause below makes the self-test refuse, because a gate whose invocation can be deleted silently
# is a suggestion, and the cheapest way to pass a check is to stop calling it.
#
# The two empty-filter steps below keep the child's output and require the sentence, rather than
# discarding the output and asserting only "non-zero" (#159). "Non-zero" is what a crash returns
# too, so the old shape passed the moment the script died on the empty-argument path: the step is
# a claim about WHY the run failed -- that it refused because it matched nothing -- and a guard
# that cannot tell a refusal from a death is not asserting that claim. Nothing in the battery can
# pin either step either: no mutant in any set under `scripts/mutations/` names a file under
# `scripts/` (all 773 rows checked), and the attack suite's whole surface is `mutate.py
# --self-test`, so deleting a step here leaves `verify` green. That is why the shape is shared by
# both steps rather than fixed in one of them -- two invocations of the same script must not be
# asserted in two different strengths.
verify:
	@set -e; \
	echo "==> gofmt"; \
	dirty=$$($(GOFMT) -l .); \
	if [ -n "$$dirty" ]; then echo "not gofmt-clean:"; echo "$$dirty"; exit 1; fi; \
	echo "==> script and set-file encoding guards"; python3 scripts/check-scripts.py; \
	echo "==> mutation self-test"; python3 scripts/mutate.py --self-test; \
	echo "==> mutation-gate self-test"; python3 scripts/check-mutations.py --self-test; \
	echo "==> shellcheck"; $(MAKE) --no-print-directory check-shell; \
	echo "==> self-test attack suite"; python3 scripts/attack-self-test.py; \
	echo "==> capture refusal contract"; python3 testing/capture/pages.py --self-test; \
	echo "==> capture manifest contract"; python3 testing/capture/manifest.py --self-test; \
	echo "==> screenshot manifest"; python3 testing/capture/manifest.py check; \
	echo "==> empty selection refuses"; \
	if nomatch_out=$$(python3 scripts/attack-self-test.py zzzznomatch 2>&1); then \
		echo "a filter that matched nothing exited 0: a run that attempts nothing proves nothing"; \
		exit 1; \
	elif ! printf '%s' "$$nomatch_out" | grep -q "no row matched"; then \
		echo "the empty-selection step failed, but not as a refusal: a refusal says \"no row matched\","; \
		echo "and anything else means the script died -- which also exits non-zero, so the exit"; \
		echo "status alone cannot tell the two apart. Its output was:"; \
		printf '%s\n' "$$nomatch_out"; \
		exit 1; \
	fi; \
	echo "==> empty argument refuses"; \
	if empty_out=$$(python3 scripts/attack-self-test.py "" 2>&1); then \
		echo "an empty argument ran the suite: \"\" must mean a filter that matches nothing, not \"no filter\" -- otherwise passing \"\$$FILTER\" with FILTER unset turns this step into the whole gate, reported green, proving something else"; \
		exit 1; \
	elif ! printf '%s' "$$empty_out" | grep -q "no row matched"; then \
		echo "the empty-argument step failed, but not as a refusal: a refusal says \"no row matched\","; \
		echo "and anything else means the script died -- which also exits non-zero, so the exit"; \
		echo "status alone cannot tell the two apart. Its output was:"; \
		printf '%s\n' "$$empty_out"; \
		exit 1; \
	fi; \
	echo "==> go vet"; $(GO) vet ./...; \
	echo "==> go build"; $(GO) build ./...; \
	echo "==> go test -race"; $(GO) test -race ./...; \
	echo "==> version stamp"; mkdir -p bin; \
	trap 'rm -f bin/plock-stamp' EXIT HUP INT TERM; \
	$(GO) build -trimpath -ldflags "-X \"$${PKG}.Version=stamp-probe\"" -o bin/plock-stamp ./cmd/plock; \
	stamped=$$(./bin/plock-stamp version); \
	case "$$stamped" in \
	  *stamp-probe*) : ;; \
	  *) echo "binary did not record the version stamp; PKG is probably wrong: $$stamped"; exit 1 ;; \
	esac; \
	$(LDFLAGS_ASM); \
	$(GO) build -trimpath -ldflags "$$ld" -o bin/plock-stamp ./cmd/plock; \
	got=$$(./bin/plock-stamp version); rm -f bin/plock-stamp; \
	want="plock $${VERSION} (commit $${COMMIT}, built $${DATE})"; \
	if [ "$$got" != "$$want" ]; then \
	  echo "version stamp mismatch:"; echo "  got  $$got"; echo "  want $$want"; exit 1; \
	fi; \
	echo "==> dependencies"; ./scripts/check-deps.sh; \
	echo "==> cross-builds"; ./scripts/check-builds.sh; \
	echo "verify: OK"

# check-mutations is the fast half of `mutate`: it never runs a test, it only asks whether
# each mutant still matches the tree and still compiles once applied. That question is worth
# asking on its own because a mutant that no longer matches is not a failing test, it is a
# guard nobody is testing any more, and the battery reports it as a printed line.
#
# It is minutes rather than seconds because it compiles the tree per mutant, which is why it
# is not folded into verify -- a gate that slow gets skipped. CI runs it as its own job
# instead, where the cost lands on a runner.
# The self-test runs first and costs milliseconds: it checks the rules that make a green
# tick here mean something. Without it, deleting one of those rules is invisible --
# every real set still passes, because none of them currently violates the rule that
# was removed.
check-mutations:
	@python3 scripts/check-mutations.py --self-test
	@GO="$(GO)" python3 scripts/check-mutations.py

# The two halves of the gate that need no Go toolchain, as targets so each can be run on its own
# the way `make check-mutations` can. Both are already inside `verify` (#189); they exist here
# because the thing worth running alone is the thing that just failed, and because CI's shellcheck
# job calls the script through this name rather than repeating its file list.
check-scripts:
	@python3 scripts/check-scripts.py

check-shell:
	@SHELLCHECK="$${SHELLCHECK:-shellcheck}" ./scripts/check-shell.sh

# mutate puts each known bug back into the source one at a time and insists the suite
# notices. A green `go test` says the code passes the tests; it does not say the tests
# would fail if the code were wrong, and for a daemon whose failure mode is a lockout
# that silently stops happening, that difference is the whole product.
#
# It is deliberately not part of verify: it rewrites files in the working tree and
# takes minutes. Run it before pushing a branch, and quote its tally in the PR when
# claiming a fix is covered by a test that bites.
#
# scripts/mutate.py restores the tree between every mutant, refuses to start on a red
# baseline, and reports a mutant that does not compile as INVALID rather than caught.
# It depends on check-mutations so the fast guard cannot be skipped by habit: a stale set
# produces a tally that reads clean while checking nothing. "Fast" is relative to the
# battery, not to a build -- the checker compiles the tree once per mutant, so it measured
# 2m39s on the warm module cache of the machine that wrote it, 5m0s on a CI runner (run
# 38901067736) and 7m12s on another machine entirely. The number a cold cache pays is the
# one to plan against; the ratio that justifies the dependency is unaffected, because the
# alternative is discovering a retired mutant hours into a battery, or never.
# Its docstring lists the six ways a battery like this can report success while
# testing nothing, which is what the run checks for. The rules that decide a verdict,
# and the rules that decide what the snapshot copies, are pure functions, so
# `scripts/mutate.py --self-test` checks them in well under a second with no Go and no
# battery -- which is why verify runs it: a guard nobody runs is the same gap #76 was
# filed for.
mutate: check-mutations
	@GO="$(GO)" python3 -u scripts/mutate.py

# package builds the installable artifacts (.deb and .rpm) under dist/.
#
# It is not a prerequisite of `install` and not part of `verify`, for the reason the repo
# already draws between verify and mutate: packaging needs dpkg, and an rpm build needs a
# container runtime, so a gate that requires them is a gate people skip. What it does need
# is the payload coming from deploy/install.sh, which is why `make package` cannot invent a
# second layout for the same program -- see scripts/package-lib.sh.
#
# ARCH defaults to this machine's, because rpm builds only for the machine it runs on: an
# aarch64 rpm cannot be produced on x86_64 at all, so `--arch all` here would build three of
# four artifacts and then stop. The release workflow is where all four get built, one job per
# architecture on a runner of that architecture (`.github/workflows/release.yml`). A cross
# .deb is fine and useful: `make package ARCH=arm64` on x86_64 gives you one.
#
# The version is `git describe --tags`, and a dirty tree is refused: an artifact stamped with
# a commit that does not contain the code in it is a false report about what is locking your
# children's screens. PKG_VERSION=x.y.z overrides for a release build, ALLOW_DIRTY=1 for a
# throwaway.
PACKAGE ?= dist
ARCH ?= $(shell uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')

package:
	@scripts/package.sh --format all --arch $(ARCH) --out $(PACKAGE)

# package-verify installs both artifacts into two throwaway distributions and compares what
# each one did: same binary, same unit, same paths, same modes, nothing enabled, and the
# state surviving the teardown each format promises. It is the only check that can see the
# failure two formats make easy -- the .deb and the .rpm disagreeing.
#
# PACKAGE_VERIFY=1 is required by the script itself, so this target is the explicit form of
# "yes, pull two images and install a root daemon in each".
package-verify:
	@set -eu; \
	debs=$$(find $(PACKAGE) -maxdepth 1 -name '*.deb' -type f 2>/dev/null | LC_ALL=C sort); \
	rpms=$$(find $(PACKAGE) -maxdepth 1 -name '*.rpm' -type f 2>/dev/null | LC_ALL=C sort); \
	nd=$$(printf '%s\n' "$$debs" | sed '/^$$/d' | wc -l); \
	nr=$$(printf '%s\n' "$$rpms" | sed '/^$$/d' | wc -l); \
	if [ "$$nd" -ne 1 ] || [ "$$nr" -ne 1 ]; then \
		printf 'package-verify: want exactly one .deb and one .rpm in %s, found %s and %s\n' '$(PACKAGE)' "$$nd" "$$nr" >&2; \
		printf '%s\n%s\n' "$$debs" "$$rpms" | sed '/^$$/d' | sed 's/^/  /' >&2; \
		printf 'more than one is ambiguous: the alphabetically first artifact would be verified, and\nplock_0.0.0+32390ce_amd64.deb sorts before plock_0.3.0_amd64.deb, so the stale build\nwins and a green gate says nothing about the artifact you meant. rm -rf %s, or pass\nthe two files to scripts/package-verify.sh yourself.\n' '$(PACKAGE)' >&2; \
		exit 1; \
	fi; \
	PACKAGE_VERIFY=1 scripts/package-verify.sh --deb "$$debs" --rpm "$$rpms"

clean:
	rm -rf bin

# install builds, then hands off to deploy/install.sh. It does not start or enable anything:
# the script prints `systemctl enable --now plock` and stops, because a root daemon that
# locks other people's sessions should begin on the day the operator decides, not on the day
# somebody typed `make install`. See the header of deploy/install.sh for why it also never
# writes the config file.
#
# verify runs first on purpose. Installing an unverified build means the machine that is
# about to enforce tonight is running a binary the repo's own gate would have rejected, and
# the failure it produces -- a screen that stays unlocked -- is silent until morning.
install: verify
	@$(MAKE) --no-print-directory build
	@deploy/install.sh

uninstall:
	@deploy/install.sh --uninstall$(if $(PURGE), --purge,)

# run-dev runs the whole enforcement loop with no desktop session and no root, against a
# throwaway config under a dev root.
#
# It uses the `command` backend rather than internal/platform.Mock, and the reason is a
# decision rather than an oversight: Mock is deliberately not registered under an id,
# because a config-selectable backend that reports success and locks nothing is the exact
# fail-open this project refuses (mock.go). Pointing run-dev at Mock would mean the
# developer loop exercises a shape the product can never take. The command backend really
# execs, so the loop, the config, the audit writer and the fallback chain under test are
# the real ones, and only the thing that would touch a screen is a shell script.
#
# verify is a prerequisite for the same reason it is one for install: a loop started
# against an unverified build teaches you things about a binary the gate would have
# rejected.
run-dev: verify
	@$(MAKE) --no-print-directory build
	@BIN=$(BIN) deploy/run-dev.sh
