# sharp-go — Makefile
#
# A Go image-processing library bound to libvips via cgo. These targets wrap
# the common `go` invocations. cgo is always required: libvips (>= 8.15) and
# pkg-config must be installed (see `make deps-help`).

GO        ?= go
PKGS      ?= ./...
PROXY_PORT ?= 3003

# Binaries built by `make install` / `make build-cli`.
BINDIR    ?= $(shell $(GO) env GOBIN)
ifeq ($(BINDIR),)
BINDIR    := $(shell $(GO) env GOPATH)/bin
endif

.DEFAULT_GOAL := help

## help: list available targets
.PHONY: help
help:
	@echo "sharp-go make targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | awk -F': ' '{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

## build: compile every package (library, CLIs, examples)
.PHONY: build
build:
	$(GO) build $(PKGS)

## test: run the full test suite (fixture tests skip if test/fixtures is absent)
.PHONY: test
test:
	$(GO) test $(PKGS)

## test-race: run the suite under the race detector
.PHONY: test-race
test-race:
	$(GO) test -race $(PKGS)

## test-v: run the suite verbosely
.PHONY: test-v
test-v:
	$(GO) test -v -count=1 $(PKGS)

## cover: run tests with a coverage profile (coverage.out) and print the summary
.PHONY: cover
cover:
	$(GO) test -coverprofile=coverage.out $(PKGS)
	$(GO) tool cover -func=coverage.out | tail -1

## bench: run benchmarks (no tests), with allocation stats
.PHONY: bench
bench:
	$(GO) test -run='^$$' -bench=. -benchmem $(PKGS)

## perf: run the encode-time sweep (gated behind PERF=1)
.PHONY: perf
perf:
	PERF=1 $(GO) test -run TestPerfSweep -v .

## fmt: gofmt-format all Go sources in place
.PHONY: fmt
fmt:
	gofmt -w .

## fmt-check: fail if any Go source is not gofmt-clean
.PHONY: fmt-check
fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "not gofmt-clean:"; echo "$$out"; exit 1; fi

## vet: run go vet
.PHONY: vet
vet:
	$(GO) vet $(PKGS)

## check: vet + race tests (use before pushing). Add fmt-check once `make fmt` is run.
.PHONY: check
check: vet test-race

## tidy: sync go.mod / go.sum
.PHONY: tidy
tidy:
	$(GO) mod tidy

## doctor: print detected libvips version + available loaders/savers
.PHONY: doctor
doctor:
	$(GO) run ./cmd/sharpgo-doctor

## build-cli: build the sharpgo + sharpgo-doctor CLIs into ./bin
.PHONY: build-cli
build-cli:
	$(GO) build -o bin/sharpgo ./cmd/sharpgo
	$(GO) build -o bin/sharpgo-doctor ./cmd/sharpgo-doctor

## install: install the sharpgo + sharpgo-doctor CLIs to $(BINDIR)
.PHONY: install
install:
	$(GO) install ./cmd/sharpgo ./cmd/sharpgo-doctor

## examples: build every program under examples/
.PHONY: examples
examples:
	$(GO) build -o /dev/null ./examples/...

## proxy: run the flagship image-optimization proxy (PROXY_PORT, default 3003)
.PHONY: proxy
proxy:
	PORT=$(PROXY_PORT) $(GO) run ./examples/proxy

## deps-help: print the libvips install command for common platforms
.PHONY: deps-help
deps-help:
	@echo "macOS:          brew install vips pkg-config"
	@echo "Debian/Ubuntu:  sudo apt install libvips-dev pkg-config"
	@echo "Alpine:         apk add vips-dev pkgconf"
	@echo "Windows:        vcpkg install vips"

## clean: remove build artifacts and the coverage profile
.PHONY: clean
clean:
	$(GO) clean
	rm -rf bin coverage.out
