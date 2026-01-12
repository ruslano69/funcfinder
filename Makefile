# Makefile for funcfinder toolkit
.PHONY: all build test clean install uninstall coverage fmt vet lint help release

# Variables
VERSION := 1.4.0
BINARIES := funcfinder stat deps complexity
BUILD_DIR := build
DIST_DIR := dist
GO := go
GOFLAGS := -v
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"

# Colors for output
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m

# Default target
all: build

## help: Display this help message
help:
	@echo "$(COLOR_BOLD)funcfinder toolkit v$(VERSION)$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Available targets:$(COLOR_RESET)"
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_-]+:.*?##/ { printf "  $(COLOR_BLUE)%-15s$(COLOR_RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(COLOR_BOLD)%s$(COLOR_RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Building

## build: Build all binaries
build:
	@echo "$(COLOR_GREEN)Building funcfinder toolkit v$(VERSION)...$(COLOR_RESET)"
	@$(GO) build $(GOFLAGS) $(LDFLAGS) -o funcfinder ./cmd/funcfinder
	@echo "  ✓ funcfinder"
	@$(GO) build $(GOFLAGS) $(LDFLAGS) -o stat ./cmd/stat
	@echo "  ✓ stat"
	@$(GO) build $(GOFLAGS) $(LDFLAGS) -o deps ./cmd/deps
	@echo "  ✓ deps"
	@$(GO) build $(GOFLAGS) $(LDFLAGS) -o complexity ./cmd/complexity
	@echo "  ✓ complexity"
	@echo "$(COLOR_GREEN)✅ All binaries built successfully!$(COLOR_RESET)"

## build-all: Build binaries for all platforms
build-all: clean
	@echo "$(COLOR_GREEN)Building for all platforms...$(COLOR_RESET)"
	@mkdir -p $(DIST_DIR)
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			ext=""; \
			if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
			echo "Building for $$os/$$arch..."; \
			for bin in $(BINARIES); do \
				GOOS=$$os GOARCH=$$arch $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$$bin-$$os-$$arch$$ext ./cmd/$$bin; \
			done; \
		done; \
	done
	@echo "$(COLOR_GREEN)✅ Cross-platform build complete!$(COLOR_RESET)"

## install: Install binaries to /usr/local/bin
install: build
	@echo "$(COLOR_YELLOW)Installing to /usr/local/bin...$(COLOR_RESET)"
	@sudo cp funcfinder /usr/local/bin/
	@sudo cp stat /usr/local/bin/
	@sudo cp deps /usr/local/bin/
	@sudo cp complexity /usr/local/bin/
	@echo "$(COLOR_GREEN)✅ Installation complete!$(COLOR_RESET)"

## uninstall: Remove binaries from /usr/local/bin
uninstall:
	@echo "$(COLOR_YELLOW)Uninstalling from /usr/local/bin...$(COLOR_RESET)"
	@sudo rm -f /usr/local/bin/funcfinder
	@sudo rm -f /usr/local/bin/stat
	@sudo rm -f /usr/local/bin/deps
	@sudo rm -f /usr/local/bin/complexity
	@echo "$(COLOR_GREEN)✅ Uninstallation complete!$(COLOR_RESET)"

##@ Testing

## test: Run all tests
test:
	@echo "$(COLOR_GREEN)Running tests...$(COLOR_RESET)"
	@$(GO) test -v -race ./internal/...

## test-coverage: Run tests with coverage
test-coverage:
	@echo "$(COLOR_GREEN)Running tests with coverage...$(COLOR_RESET)"
	@$(GO) test -v -race -coverprofile=coverage.txt -covermode=atomic ./internal/...
	@$(GO) tool cover -func=coverage.txt | tail -1

## coverage: Generate HTML coverage report
coverage: test-coverage
	@$(GO) tool cover -html=coverage.txt -o coverage.html
	@echo "$(COLOR_GREEN)Coverage report: coverage.html$(COLOR_RESET)"

## bench: Run benchmarks
bench:
	@echo "$(COLOR_GREEN)Running benchmarks...$(COLOR_RESET)"
	@$(GO) test -bench=. -benchmem ./internal/...

##@ Code Quality

## fmt: Format code
fmt:
	@echo "$(COLOR_GREEN)Formatting code...$(COLOR_RESET)"
	@$(GO) fmt ./...
	@echo "$(COLOR_GREEN)✅ Code formatted$(COLOR_RESET)"

## vet: Run go vet
vet:
	@echo "$(COLOR_GREEN)Running go vet...$(COLOR_RESET)"
	@$(GO) vet ./...
	@echo "$(COLOR_GREEN)✅ No issues found$(COLOR_RESET)"

## lint: Run golangci-lint (requires golangci-lint installed)
lint:
	@echo "$(COLOR_GREEN)Running linter...$(COLOR_RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
		echo "$(COLOR_GREEN)✅ Linting complete$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)⚠️  golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(COLOR_RESET)"; \
	fi

## check: Run all quality checks (fmt, vet, lint, test)
check: fmt vet test
	@echo "$(COLOR_GREEN)✅ All checks passed!$(COLOR_RESET)"

##@ Development

## run: Run funcfinder on itself (dogfooding)
run: build
	@echo "$(COLOR_GREEN)Analyzing funcfinder codebase...$(COLOR_RESET)"
	@./funcfinder --inp ./internal --source go --map

## analyze: Run all tools on the codebase
analyze: build
	@echo "$(COLOR_GREEN)Running full analysis...$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)=== Functions ===$(COLOR_RESET)"
	@./funcfinder --inp ./internal --source go --tree
	@echo ""
	@echo "$(COLOR_BOLD)=== Statistics ===$(COLOR_RESET)"
	@./stat ./internal -l go
	@echo ""
	@echo "$(COLOR_BOLD)=== Dependencies ===$(COLOR_RESET)"
	@./deps . -l go
	@echo ""
	@echo "$(COLOR_BOLD)=== Complexity ===$(COLOR_RESET)"
	@./complexity -l go -nosimple ./internal

## watch: Watch for changes and rebuild (requires entr)
watch:
	@if command -v entr >/dev/null 2>&1; then \
		echo "$(COLOR_GREEN)Watching for changes...$(COLOR_RESET)"; \
		find . -name '*.go' | entr -c make build; \
	else \
		echo "$(COLOR_YELLOW)⚠️  entr not installed. Install with: brew install entr (macOS) or apt install entr (Linux)$(COLOR_RESET)"; \
	fi

##@ Maintenance

## clean: Remove built binaries and artifacts
clean:
	@echo "$(COLOR_YELLOW)Cleaning...$(COLOR_RESET)"
	@rm -f $(BINARIES)
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@rm -f coverage.txt coverage.html
	@echo "$(COLOR_GREEN)✅ Cleanup complete$(COLOR_RESET)"

## deps: Download dependencies
deps:
	@echo "$(COLOR_GREEN)Downloading dependencies...$(COLOR_RESET)"
	@$(GO) mod download
	@echo "$(COLOR_GREEN)✅ Dependencies downloaded$(COLOR_RESET)"

## tidy: Tidy go.mod
tidy:
	@echo "$(COLOR_GREEN)Tidying go.mod...$(COLOR_RESET)"
	@$(GO) mod tidy
	@echo "$(COLOR_GREEN)✅ go.mod tidied$(COLOR_RESET)"

## update: Update dependencies
update:
	@echo "$(COLOR_GREEN)Updating dependencies...$(COLOR_RESET)"
	@$(GO) get -u ./...
	@$(GO) mod tidy
	@echo "$(COLOR_GREEN)✅ Dependencies updated$(COLOR_RESET)"

##@ Release

## release: Create a new release (use: make release VERSION=1.5.0)
release:
	@echo "$(COLOR_GREEN)Creating release v$(VERSION)...$(COLOR_RESET)"
	@if [ -z "$(VERSION)" ]; then \
		echo "$(COLOR_YELLOW)⚠️  VERSION not specified. Usage: make release VERSION=1.5.0$(COLOR_RESET)"; \
		exit 1; \
	fi
	@echo "Updating version in files..."
	@# Update version in main files
	@sed -i 's/Version = "[^"]*"/Version = "$(VERSION)"/' cmd/funcfinder/main.go
	@sed -i 's/Version = "[^"]*"/Version = "$(VERSION)"/' cmd/stat/main.go
	@sed -i 's/Version = "[^"]*"/Version = "$(VERSION)"/' cmd/deps/main.go
	@sed -i 's/Version = "[^"]*"/Version = "$(VERSION)"/' cmd/complexity/main.go
	@echo "Creating git tag..."
	@git tag -a v$(VERSION) -m "Release v$(VERSION)"
	@echo "$(COLOR_GREEN)✅ Release v$(VERSION) ready!$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Push with: git push origin v$(VERSION)$(COLOR_RESET)"

## version: Show current version
version:
	@echo "funcfinder v$(VERSION)"
