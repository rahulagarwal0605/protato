# Protato Makefile

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Default registry URL (can be overridden)
REGISTRY_URL ?=

# Go build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE) -X main._defaultRegistryURL=$(REGISTRY_URL)"

# Binary name
BINARY := protato

# Directories
BUILD_DIR := build
DIST_DIR := dist

.PHONY: all build clean test lint install dev help

# Default target
all: build

# Build the binary
build:
	@echo "Building $(BINARY) $(VERSION)..."
	@go build $(LDFLAGS) -o $(BINARY) .

# Build with race detector
build-race:
	@echo "Building $(BINARY) with race detector..."
	@go build -race $(LDFLAGS) -o $(BINARY) .

# Install to GOPATH/bin
install:
	@echo "Installing $(BINARY)..."
	@go install $(LDFLAGS) .

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@gofumpt -w .

# Tidy modules
tidy:
	@echo "Tidying modules..."
	@go mod tidy

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY)
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@rm -f coverage.out coverage.html

# Development build (fast, no optimizations)
dev:
	@echo "Building for development..."
	@go build -o $(BINARY) .

# Build for all platforms
build-all: clean
	@mkdir -p $(DIST_DIR)
	@echo "Building for linux/amd64..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-amd64 .
	@echo "Building for linux/arm64..."
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-arm64 .
	@echo "Building for darwin/amd64..."
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-amd64 .
	@echo "Building for darwin/arm64..."
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-arm64 .
	@echo "Building for windows/amd64..."
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-windows-amd64.exe .

# Create release archives
release: build-all
	@echo "Creating release archives..."
	@cd $(DIST_DIR) && for f in $(BINARY)-*; do \
		if [ "$${f##*.}" = "exe" ]; then \
			zip "$${f%.exe}.zip" "$$f"; \
		else \
			tar czf "$$f.tar.gz" "$$f"; \
		fi \
	done

# Show help
help:
	@echo "Protato Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build        - Build the binary"
	@echo "  build-race   - Build with race detector"
	@echo "  install      - Install to GOPATH/bin"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage"
	@echo "  lint         - Run linter"
	@echo "  fmt          - Format code"
	@echo "  tidy         - Tidy modules"
	@echo "  deps         - Download dependencies"
	@echo "  clean        - Clean build artifacts"
	@echo "  dev          - Development build"
	@echo "  build-all    - Build for all platforms"
	@echo "  release      - Create release archives"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION      - Version string (default: git describe)"
	@echo "  REGISTRY_URL - Default registry URL to embed"

