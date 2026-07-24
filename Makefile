GO ?= go
BINARY ?= bin/qweather
BUILD_FLAGS ?=
PACKAGES ?= ./...
ZERO_SHA := 0000000000000000000000000000000000000000

.PHONY: all build test test-race vet fmt-check mod-verify e2e-compile diff-check ci-diff-check check clean help

all: build

build:
	@mkdir -p -- "$$(dirname -- "$(BINARY)")"
	$(GO) build $(BUILD_FLAGS) -o "$(BINARY)" ./cmd/qweather

test:
	$(GO) test $(PACKAGES)

test-race:
	$(GO) test -race $(PACKAGES)

vet:
	$(GO) vet $(PACKAGES)

fmt-check:
	test -z "$$(gofmt -l .)"

mod-verify:
	$(GO) mod verify

e2e-compile:
	$(GO) test -tags=e2e ./tests/e2e -run '^$$'

diff-check:
	git diff --check

ci-diff-check:
	@if [ -z "$(BASE_SHA)" ] || [ "$(BASE_SHA)" = "$(ZERO_SHA)" ]; then \
		git diff-tree --check --no-commit-id -r HEAD; \
	else \
		git diff --check "$(BASE_SHA)...HEAD"; \
	fi

check: fmt-check mod-verify test test-race vet build e2e-compile diff-check

clean:
	rm -rf bin

help:
	@printf '%s\n' \
		'make build                  Build bin/qweather' \
		'make test                   Run the normal non-live Go test suite' \
		'make check                  Run every deterministic local gate' \
		'make clean                  Remove repository-local build output' \
		'make build BINARY=<path>    Build to an explicit output path' \
		'make build BUILD_FLAGS=<flags>  Pass flags to go build' \
		'make test PACKAGES=<pattern>  Test a focused Go package pattern'
