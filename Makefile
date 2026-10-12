BIN     := bin/plock
MODULE  := github.com/vicowa-botworks/parental-lockout
PKG     := $(MODULE)/internal/version

GO      ?= go
GOFMT   ?= gofmt
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(PKG).Version=$(VERSION) \
	-X $(PKG).Commit=$(COMMIT) \
	-X $(PKG).Date=$(DATE)

.DEFAULT_GOAL := build

.PHONY: all build run-dev fmt vet test deps verify clean

all: build

build:
	@mkdir -p bin
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/plock

# Foreground development loop: temp config, mock platform, plain HTTP on
# localhost. Needs no root and no desktop session. Wired up in a later ticket.
run-dev: build
	$(BIN) serve --dev

vet:
	$(GO) vet ./...

test:
	$(GO) test -race ./...

fmt:
	$(GOFMT) -w .

deps:
	@./scripts/check-deps.sh

# The single gate. CI, the review loop and the adversarial reviewer all call
# this and nothing else, so "passes locally" and "passes in CI" cannot drift.
verify:
	@set -e; \
	echo "==> gofmt"; \
	dirty=$$($(GOFMT) -l .); \
	if [ -n "$$dirty" ]; then \
		echo "not gofmt-clean:"; echo "$$dirty"; exit 1; \
	fi; \
	echo "==> go vet"; $(GO) vet ./...; \
	echo "==> go build"; $(GO) build ./...; \
	echo "==> go test -race"; $(GO) test -race ./...; \
	echo "==> dependencies"; ./scripts/check-deps.sh; \
	echo "verify: OK"

clean:
	rm -rf bin
