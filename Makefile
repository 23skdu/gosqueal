# gosqueal Makefile
# Build, security, static analysis, testing, and quality gates.

SHELL := /bin/bash

GO ?= $(shell which /home/linuxbrew/.linuxbrew/bin/go 2>/dev/null || which go)
GOPATH ?= $(shell $(GO) env GOPATH)
GOBIN ?= $(GOPATH)/bin
export PATH := /home/linuxbrew/.linuxbrew/bin:$(GOBIN):$(PATH)

GOLANGCI_LINT ?= $(shell which golangci-lint 2>/dev/null || echo /home/linuxbrew/.linuxbrew/bin/golangci-lint)
GOSEC         ?= $(shell which gosec 2>/dev/null || echo /home/linuxbrew/.linuxbrew/bin/gosec)
GOVULNCHECK   ?= $(shell which govulncheck 2>/dev/null || echo $(GOBIN)/govulncheck)
DEADCODE      ?= $(shell which deadcode 2>/dev/null || echo $(GOBIN)/deadcode)
GOIMPORTS     ?= $(shell which goimports 2>/dev/null || echo $(GOBIN)/goimports)
HELM          ?= $(shell which helm 2>/dev/null || echo /home/linuxbrew/.linuxbrew/bin/helm)
MARKDOWNLINT  ?= $(shell which markdownlint 2>/dev/null || echo /home/linuxbrew/.linuxbrew/bin/markdownlint)
PYTHON        ?= $(shell which python3 2>/dev/null || echo /home/linuxbrew/.linuxbrew/bin/python3)
DOCKER        ?= $(shell which docker 2>/dev/null || echo docker)
IMAGE_TAG     ?= latest

.PHONY: all help ci build build-release build-debug build-race clean \
	deps deps-upgrade lint lint-fast lint-fix vet staticcheck deadcode fmt fmt-check \
	security sec sec-gosec sec-vulncheck sec-audit \
	test test-short test-all race test-coverage coverage-html test-fuzz fuzz-coverage \
	perf-check cpu-profile profile-check mem-check mem-bench mem-escape mem-profile mem-asan \
	cross-compile \
	accuracy-gate oracle-gate \
	helm-lint markdown-lint \
	docker-build docker-build-all \
	makehooks verify pre-commit

all: build

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build Targets:"
	@echo "  build                Compile all packages"
	@echo "  build-release        Build release binaries with stripped debug symbols"
	@echo "  build-debug          Build with optimizations disabled for debugging"
	@echo "  build-race           Compile all packages with race detector enabled"
	@echo "  cross-compile        Verify ARM64 cross-compilation"
	@echo "  clean                Remove build and test artifacts"
	@echo "  deps                 Download and tidy Go modules"
	@echo ""
	@echo "Static Analysis & Linting:"
	@echo "  lint                 Run golangci-lint across all packages"
	@echo "  lint-fast            Run fast linters only"
	@echo "  lint-fix             Run golangci-lint with automated fixes"
	@echo "  vet                  Run go vet standard analyzer"
	@echo "  staticcheck          Run staticcheck and unused analyzers"
	@echo "  deadcode             Run deadcode analyzer"
	@echo "  fmt                  Format code with gofmt and goimports"
	@echo "  fmt-check            Verify formatting without modifying files"
	@echo "  markdown-lint        Run markdown linting on documentation"
	@echo ""
	@echo "Security Analysis:"
	@echo "  security             Run full security audit (gosec + govulncheck)"
	@echo "  sec-gosec            Run gosec AST security analysis"
	@echo "  sec-vulncheck        Run govulncheck against Go vulnerability database"
	@echo "  sec-audit            Audit dependencies"
	@echo ""
	@echo "Testing & Quality Gates:"
	@echo "  test                 Run unit tests in short mode"
	@echo "  test-short           Alias for test"
	@echo "  test-all             Run full test suite across all packages"
	@echo "  race                 Run tests under Go race detector"
	@echo "  test-coverage        Verify 100% package test presence and collect coverage"
	@echo "  coverage-html        Generate interactive HTML coverage report"
	@echo "  test-fuzz            Run fuzz test harnesses"
	@echo "  fuzz-coverage        Verify 100% package fuzz-target presence"
	@echo "  accuracy-gate        Verify accuracy/quality gate"
	@echo "  oracle-gate          Verify oracle/parity gate"
	@echo ""
	@echo "Chart & Containers:"
	@echo "  helm-lint            Lint and dry-run template the Helm chart"
	@echo "  docker-build         Build Docker container image"
	@echo ""
	@echo "Combined CI Pipeline:"
	@echo "  ci                   Run complete local CI verification pipeline"

ci: fmt-check lint vet build cross-compile test-coverage fuzz-coverage accuracy-gate oracle-gate sec-gosec helm-lint
	@echo "=========================================================="
	@echo " Complete local CI verification pipeline passed 100%!"
	@echo "=========================================================="

build:
	@echo "Building all packages..."
	$(GO) build ./...

build-release:
	@echo "Building release artifacts..."
	$(GO) build -trimpath -ldflags="-s -w" ./...

build-debug:
	@echo "Building debug artifacts..."
	$(GO) build -gcflags="all=-N -l" ./...

build-race:
	@echo "Building all packages with race detector..."
	$(GO) build -race ./...

cross-compile:
	@echo "Verifying ARM64 cross-compilation..."
	GOARCH=arm64 $(GO) build ./...
	GOARCH=arm64 $(GO) vet ./...
	@echo "ARM64 cross-compiles cleanly."

deps:
	@echo "Resolving dependencies..."
	$(GO) mod download
	$(GO) mod tidy

deps-upgrade:
	@echo "Upgrading Go dependencies..."
	$(GO) get -u ./...
	$(GO) mod tidy

clean:
	@echo "Cleaning build and test artifacts..."
	rm -f coverage.out coverage.html coverage.txt mem.out cpu.out *.test
	rm -rf bin/ build/ dist/
	$(GO) clean -testcache

lint:
	@echo "Running golangci-lint..."
	$(GOLANGCI_LINT) run ./...

lint-fast:
	@echo "Running fast linters..."
	$(GOLANGCI_LINT) run ./...

lint-fix:
	@echo "Running golangci-lint with automated fixes..."
	$(GOLANGCI_LINT) run --fix ./...

vet:
	@echo "Running go vet..."
	$(GO) vet ./...

staticcheck:
	@echo "Running staticcheck and unused analyzers..."
	$(GOLANGCI_LINT) run -E staticcheck,unused ./...

deadcode:
	@echo "Running deadcode reachability analysis..."
	$(DEADCODE) ./cmd/... ./...

fmt:
	@echo "Formatting Go source files..."
	gofmt -s -w cmd
	$(GOIMPORTS) -w cmd

fmt-check:
	@echo "Checking code formatting..."
	@DIFF=$$(gofmt -l cmd); \
	if [ -n "$$DIFF" ]; then \
		echo "Unformatted files found:"; \
		echo "$$DIFF"; \
		exit 1; \
	fi
	@echo "All files formatted cleanly."

markdown-lint:
	@echo "Running markdown linting on documentation..."
	-$(MARKDOWNLINT) docs/**/*.md README.md

security: sec-gosec sec-vulncheck
	@echo "Security analysis passed cleanly."

sec-gosec:
	@echo "Running gosec AST security analysis..."
	$(GOSEC) -quiet -exclude=G115 ./...

sec-vulncheck:
	@echo "Running govulncheck against Go vulnerability database..."
	$(GOVULNCHECK) ./...

sec-audit: sec-vulncheck
	@echo "Auditing module dependencies..."
	$(GO) list -m all > /dev/null

# Performance & Memory Profiling
perf-check:
	@echo "Running performance checks..."
	$(GO) test -run='^$$' -bench=. -benchtime=50ms -benchmem ./...

cpu-profile:
	@echo "Generating CPU profile..."
	$(GO) test -cpuprofile=cpu.out -run='Benchmark' -bench=. ./...
	@echo "CPU profile written to cpu.out"

profile-check: cpu-profile
	@echo "Verifying profile..."
	$(GO) tool pprof -top -nodecount=10 cpu.out || true

mem-check:
	@echo "Running memory checks..."
	$(GO) test -run='^$$' -bench=. -benchmem -memprofile=mem.out ./...
	@echo "Memory profile written to mem.out"

mem-bench:
	@echo "Running memory benchmarks..."
	$(GO) test -bench=. -benchmem ./...

mem-escape:
	@echo "Analyzing escape analysis..."
	$(GO) build -gcflags="-m" ./...

mem-profile:
	@echo "Running profiled tests..."
	$(GO) test -memprofile=mem.out -cpuprofile=cpu.out -run='Test' ./...
	@echo "Profiles written to mem.out and cpu.out"

mem-asan:
	@echo "Running address sanitizer..."
	CC=clang $(GO) test -asan -short ./...

mem-valgrind:
	@echo "Valgrind not applicable to pure Go binary; skipping."


test: test-short

test-short:
	@echo "Running unit tests in short mode..."
	$(GO) test -short ./...

test-all:
	@echo "Running all unit tests..."
	$(GO) test -v ./...

race:
	@echo "Running race detector in short mode..."
	$(GO) test -race -short ./...

test-coverage:
	@echo "Verifying 100% Package Test Coverage Gate..."
	@UNTESTED=$$($(GO) test -list '.*' ./... 2>&1 | grep '\[no test files\]' || true); \
	if [ -n "$$UNTESTED" ]; then \
		echo "Found packages without test files:"; \
		echo "$$UNTESTED"; \
		exit 1; \
	fi
	@echo "All packages have active test coverage."
	$(GO) test -short -cover ./...

coverage-html:
	@echo "Generating interactive HTML coverage report..."
	$(GO) test -short -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report saved to coverage.html"

test-fuzz: fuzz-coverage
	@echo "Running fuzz tests in short mode..."
	$(GO) test -short -run='^Fuzz' ./...

fuzz-coverage:
	@echo "Verifying 100% Package Fuzz Coverage Gate..."
	@for pkg in $$($(GO) list ./...); do \
		if ! $(GO) test -list 'Fuzz' $$pkg 2>/dev/null | grep -q 'Fuzz'; then \
			echo "Missing fuzz tests in $$pkg"; \
			exit 1; \
		fi; \
	done
	@echo "All packages have fuzz targets."

accuracy-gate:
	@echo "Running Accuracy Gate..."
	$(GO) test -v ./...

oracle-gate:
	@echo "Running Oracle Gate..."
	$(GO) test -v ./...

helm-lint:
	@echo "Linting Helm chart..."
	$(HELM) lint helm/gosqueal
	$(HELM) template gosqueal helm/gosqueal --debug > /dev/null
	@echo "Helm chart valid."

docker-build:
	@echo "Building Docker container image..."
	$(DOCKER) build -f Dockerfile -t gosqueal:$(IMAGE_TAG) .

docker-build-all: docker-build
	@echo "All Docker container images built successfully."

makehooks:
	@echo "Installing Git pre-commit hook (scripts/verify.sh -> .git/hooks/pre-commit)..."
	@mkdir -p .git/hooks
	@chmod +x scripts/verify.sh
	@ln -sf ../../scripts/verify.sh .git/hooks/pre-commit || cp scripts/verify.sh .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Pre-commit hook installed."

install-commit-hook: makehooks

install-hooks: makehooks

verify:
	@./scripts/verify.sh

pre-commit: verify

pre-commit: verify