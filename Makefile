.PHONY: all build build-all clean test test-go test-bash test-cover cover-html vet lint vuln fmt precommit help

.DEFAULT_GOAL := help

# Ensure Homebrew-installed tools are available when `make` runs under a
# non-login shell. Harmless on non-Homebrew systems.
export PATH := /opt/homebrew/bin:/usr/local/bin:$(PATH)

BINARY_NAME := smoke-go
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

build: ## Build smoke-go for the current platform
	@mkdir -p bin
	CGO_ENABLED=0 $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/smoke-go

build-all: ## Cross-compile smoke-go for common release targets
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

lint: ## Run golangci-lint (installs if missing)
	@which golangci-lint >/dev/null || (echo "Installing golangci-lint..." && $(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

vuln: ## Run govulncheck (installs if missing)
	@which govulncheck >/dev/null || (echo "Installing govulncheck..." && $(GOCMD) install golang.org/x/vuln/cmd/govulncheck@latest)
	govulncheck ./...

fmt: ## Format Go code and tidy modules
	$(GOCMD) fmt ./...
	$(GOCMD) mod tidy

precommit: ## Run repository pre-commit hooks
	pre-commit run -a

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'
