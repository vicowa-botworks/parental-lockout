BIN     := bin/plock
MODULE  := github.com/vicowa-botworks/parental-lockout
PKG     := $(MODULE)/internal/version

GO      ?= go
GOFMT   ?= gofmt
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# The values arrive from git and, in a release workflow, from a workflow input —
# so they are data, not shell. Escape single quotes for interpolation into the
# single-quoted recipe string, and wrap each -X argument in double quotes so
# Go's own -ldflags splitter keeps a value containing a space in one piece.
shquote = $(subst ','\'',$(1))

LDFLAGS := -s -w \
	-X "$(PKG).Version=$(call shquote,$(VERSION))" \
	-X "$(PKG).Commit=$(call shquote,$(COMMIT))" \
	-X "$(PKG).Date=$(call shquote,$(DATE))"

.DEFAULT_GOAL := build

.PHONY: build fmt vet test verify clean

build:
	@mkdir -p bin
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/plock

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
	$(GO) build -trimpath -ldflags '-X "$(PKG).Version=stamp-probe"' -o bin/plock-stamp ./cmd/plock; \
	stamped=$$(./bin/plock-stamp version); rm -f bin/plock-stamp; \
	case "$$stamped" in \
	  *stamp-probe*) : ;; \
	  *) echo "binary did not record -X $(PKG).Version; PKG is probably wrong: $$stamped"; exit 1 ;; \
	esac; \
	if [ "$(COMMIT)" != "none" ]; then \
	  mkdir -p bin; $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/plock-stamp ./cmd/plock; \
	  full=$$(./bin/plock-stamp version); rm -f bin/plock-stamp; \
	  case "$$full" in \
	    *"$(COMMIT)"*) : ;; \
	    *) echo "binary did not record COMMIT=$(COMMIT): $$full"; exit 1 ;; \
	  esac; \
	fi; \
	echo "==> dependencies"; ./scripts/check-deps.sh; \
	echo "verify: OK"

clean:
	rm -rf bin
