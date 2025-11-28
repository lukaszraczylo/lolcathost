.PHONY: build test lint vet staticcheck clean install uninstall fmt

# Build variables
BINARY_NAME=lolcathost
VERSION?=1.0.0
BUILD_DIR=./build
LDFLAGS=-ldflags "-s -w -X main.appVersion=$(VERSION)"

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt
GOMOD=$(GOCMD) mod

# Default target
all: lint test build

# Build the binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/lolcathost

# Build for all platforms
build-all: build-darwin-arm64 build-darwin-amd64 build-linux-arm64 build-linux-amd64

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/lolcathost

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/lolcathost

build-linux-arm64:
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/lolcathost

build-linux-amd64:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/lolcathost

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run single test
test-run:
	$(GOTEST) -v -run $(TEST) ./...

# Run benchmarks
bench:
	$(GOTEST) -bench=. -benchmem ./...

# Linting
lint: vet staticcheck

vet:
	$(GOVET) ./...

staticcheck:
	@command -v staticcheck >/dev/null 2>&1 || { echo "Installing staticcheck..."; go install honnef.co/go/tools/cmd/staticcheck@latest; }
	staticcheck ./...

# Format code
fmt:
	$(GOFMT) ./...

# Tidy dependencies
tidy:
	$(GOMOD) tidy

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Install locally (for development)
install: build
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installed to /usr/local/bin/$(BINARY_NAME)"
	@echo "Run 'sudo lolcathost --install' to set up the daemon"

# Uninstall
uninstall:
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Removed /usr/local/bin/$(BINARY_NAME)"
	@echo "Note: Run 'sudo lolcathost --uninstall' first to remove the daemon"

# Development helpers
dev: fmt lint test build

# Run the TUI (requires daemon to be installed)
run: build
	$(BUILD_DIR)/$(BINARY_NAME)

# Run as daemon (requires sudo)
run-daemon: build
	sudo $(BUILD_DIR)/$(BINARY_NAME) --daemon

# Show help
help:
	@echo "Available targets:"
	@echo "  all            - Lint, test, and build"
	@echo "  build          - Build the binary"
	@echo "  build-all      - Build for all platforms"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-run       - Run specific test (use TEST=TestName)"
	@echo "  bench          - Run benchmarks"
	@echo "  lint           - Run linters (vet + staticcheck)"
	@echo "  fmt            - Format code"
	@echo "  tidy           - Tidy go.mod"
	@echo "  clean          - Clean build artifacts"
	@echo "  install        - Install binary to /usr/local/bin"
	@echo "  uninstall      - Remove binary from /usr/local/bin"
	@echo "  dev            - Format, lint, test, and build"
	@echo "  run            - Run the TUI"
	@echo "  run-daemon     - Run as daemon (requires sudo)"
