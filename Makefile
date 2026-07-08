.PHONY: build clean test test-e2e test-smoke install build-all gen gen-check

# Binary name
BINARY=railctl

# Build info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "local-build")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# ldflags target matches the var block in internal/cmd/root.go
LDFLAGS=-ldflags "-X github.com/kubenoops/railctl/internal/cmd.version=$(VERSION) -X github.com/kubenoops/railctl/internal/cmd.commit=$(COMMIT) -X github.com/kubenoops/railctl/internal/cmd.date=$(DATE)"

# Default target
all: build

# Build the binary
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/railctl

# Build for all platforms (for releases)
build-all:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 ./cmd/railctl
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 ./cmd/railctl
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 ./cmd/railctl
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 ./cmd/railctl

# Install to GOPATH/bin
install:
	go install $(LDFLAGS) ./cmd/railctl

# Clean build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist/
	rm -f coverage.out

# Run Go tests
test:
	go test -v ./...

# Run E2E tests (requires RAILWAY_TOKEN; loads via direnv from tests/e2e/.envrc)
test-e2e: build
	
	RAILCTL=$(CURDIR)/$(BINARY) go test -tags e2e -v -timeout 20m ./tests/e2e/...

# Run smoke E2E test only (~1min, full lifecycle, no permutations)
test-smoke: build
	
	RAILCTL=$(CURDIR)/$(BINARY) go test -tags e2e -v -run TestSmoke -timeout 3m ./tests/e2e/...

# Regenerate embedded assets (copies docs/railctl-skill.md into internal/skill/)
gen:
	go generate ./...

# Verify generated files are in sync with their sources (used by CI)
gen-check: gen
	@git diff --exit-code -- internal/skill/railctl-skill.md \
		|| { echo "internal/skill/railctl-skill.md is stale — run 'make gen' and commit."; exit 1; }

# Check code formatting
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run

# Show help
help:
	@echo "Usage: make [target] [VERSION=vX.Y.Z]"
	@echo ""
	@echo "Targets:"
	@echo "  build           - Build the railctl binary"
	@echo "  build-all       - Build for all platforms (linux/darwin, amd64/arm64)"
	@echo "  install         - Install to GOPATH/bin"
	@echo "  clean           - Remove build artifacts"
	@echo "  test            - Run Go unit tests"
	@echo "  test-e2e        - Build and run all E2E tests (needs RAILWAY_TOKEN, ~20min)"
	@echo "  test-smoke      - Build and run smoke E2E test (needs RAILWAY_TOKEN, ~1min)"
	@echo "  fmt             - Format Go code"
	@echo "  lint            - Run golangci-lint"
	@echo ""
	@echo "Examples:"
	@echo "  make build                  # Build with auto-detected version"
	@echo "  make build VERSION=v1.0.0   # Build with specific version"

