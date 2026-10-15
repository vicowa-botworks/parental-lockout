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

.PHONY: build fmt vet test verify mutate clean install uninstall

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
# Its docstring lists the four ways a battery like this can report success while
# testing nothing, which is what the run checks for.
mutate:
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
