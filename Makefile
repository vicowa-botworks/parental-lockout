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

.PHONY: build fmt vet test verify check-mutations mutate clean install uninstall run-dev

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
verify:
	@set -e; \
	echo "==> gofmt"; \
	dirty=$$($(GOFMT) -l .); \
	if [ -n "$$dirty" ]; then echo "not gofmt-clean:"; echo "$$dirty"; exit 1; fi; \
	echo "==> mutation self-test"; python3 scripts/mutate.py --self-test; \
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
