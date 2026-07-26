GO ?= go
NODE ?= node
NPM ?= npm
BINARY ?= bin/qweather
BUILD_FLAGS ?=
PACKAGES ?= ./...
ZERO_SHA := 0000000000000000000000000000000000000000
VERSION_FILE := VERSION
VERSION ?= $(shell tr -d '[:space:]' < "$(VERSION_FILE)")
BUILD_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || printf '%s' unknown)
BUILD_TIME ?= $(shell git show -s --format=%cI HEAD 2>/dev/null || printf '%s' unknown)
BUILD_LDFLAGS := -X github.com/Nativu5/qweather-cli/internal/buildinfo.Version=$(VERSION) -X github.com/Nativu5/qweather-cli/internal/buildinfo.Commit=$(BUILD_COMMIT) -X github.com/Nativu5/qweather-cli/internal/buildinfo.BuildTime=$(BUILD_TIME)

.PHONY: all build test test-race vet fmt-check mod-verify e2e-compile diff-check ci-diff-check check npm-ci npm-test npm-pack-check npm-smoke npm-check release-pack release-verify clean help

all: build

build:
	@mkdir -p -- "$$(dirname -- "$(BINARY)")"
	$(GO) build $(BUILD_FLAGS) -ldflags "$(BUILD_LDFLAGS)" -o "$(BINARY)" ./cmd/qweather

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

npm-ci:
	cd packages/npm && $(NPM) ci --ignore-scripts

npm-test:
	cd packages/npm && $(NPM) test --ignore-scripts

npm-pack-check:
	cd packages/npm && $(NPM) run pack:check

npm-smoke: build
	$(NODE) packages/npm/scripts/smoke-install.js --binary "$(BINARY)"

npm-check: npm-test npm-pack-check npm-smoke

release-pack:
	$(GO) run ./tools/release pack --version "$(VERSION)" --output dist

release-verify:
	$(GO) run ./tools/release verify --version "$(VERSION)" --output dist

clean:
	rm -rf bin

help:
	@printf '%s\n' \
		'make build                  Build bin/qweather' \
		'make test                   Run the normal non-live Go test suite' \
		'make check                  Run every deterministic local gate' \
		'make npm-ci                 Install the exact npm shrinkwrap without lifecycle scripts' \
		'make npm-check              Run npm tests, pack reproducibility, and local install smoke' \
		'make release-pack           Build the six release archives once' \
		'make release-verify         Double-build and prove release artifact reproducibility' \
		'make clean                  Remove repository-local build output' \
		'make build BINARY=<path>    Build to an explicit output path' \
		'make build BUILD_FLAGS=<flags>  Pass flags to go build' \
		'make test PACKAGES=<pattern>  Test a focused Go package pattern'
