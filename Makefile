.PHONY: all build build-all clean test test-go test-bash test-cover cover-html vet lint vuln fmt precommit hooks help

.DEFAULT_GOAL := help

# Ensure Go- and Homebrew-installed tools are available when `make` runs under
# a non-login shell. Harmless when either location is absent.
# Preserve an explicitly selected reviewed toolchain ahead of helper bins.
GO_BIN := $(shell go env GOPATH)/bin
export PATH := $(PATH):$(GO_BIN):/opt/homebrew/bin:/usr/local/bin

BINARY_NAME := smoke
VERSION ?= dev
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOVET := $(GOCMD) vet
BUILDFLAGS := -trimpath
LDFLAGS := -ldflags "-w -s -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

all: test build ## Run tests then build

build: ## Build smoke for the current platform
	@mkdir -p bin
	CGO_ENABLED=0 $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/smoke-go

build-all: ## Cross-compile smoke for common release targets
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/smoke-go
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/smoke-go
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/smoke-go
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/smoke-go
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/smoke-go

clean: ## Remove build artifacts
	rm -rf bin coverage.out coverage.html

test: test-go test-bash ## Run Go and legacy Bash tests

test-go: ## Run Go tests
	$(GOTEST) -v -race ./...

test-bash: ## Run legacy Bash/BATS tests
	bats -r _test

test-cover: ## Run Go tests with coverage
	$(GOTEST) -v -coverprofile=coverage.out ./...
	@echo "Wrote coverage.out"

cover-html: test-cover ## Generate coverage.html from coverage.out
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Wrote coverage.html"

vet: ## Run go vet
	$(GOVET) ./...

lint: ## Run the explicitly installed golangci-lint v2; never fetch latest during verification
	@golangci-lint version 2>/dev/null | grep -q 'version 2\.' || { echo "Missing reviewed golangci-lint v2: install an immutable version aged at least seven days" >&2; exit 1; }
	golangci-lint run ./...

vuln: ## Run the explicitly installed govulncheck; never fetch latest during verification
	@command -v govulncheck >/dev/null || { echo "Missing reviewed govulncheck: install an immutable version aged at least seven days" >&2; exit 1; }
	govulncheck ./...

fmt: ## Format Go code and tidy modules
	$(GOCMD) fmt ./...
	$(GOCMD) mod tidy

precommit: ## Run repository pre-commit hooks
	pre-commit run -a

hooks: ## Install lefthook git hooks
	lefthook install

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'
