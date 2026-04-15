# Model Metadata Collection Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
BINARY_NAME=model-extractor
BINARY_UNIX=$(BINARY_NAME)_unix

# Build parameters
BUILD_DIR=build
MAIN_PATH=./cmd/model-extractor

# Default data paths
REDHAT_MODELS_INDEX_PATH=data/models-index.yaml
VALIDATED_MODELS_INDEX_PATH=data/validated-models-index.yaml
OTHER_MODELS_INDEX_PATH=data/other-models-index.yaml
REDHAT_CATALOG_OUTPUT_PATH=data/models-catalog.yaml
VALIDATED_CATALOG_OUTPUT_PATH=data/validated-models-catalog.yaml
OTHER_CATALOG_OUTPUT_PATH=data/other-models-catalog.yaml
REDHAT_MCP_SERVERS_INDEX_PATH=data/redhat-mcp-servers-index.yaml
REDHAT_MCP_SERVERS_CATALOG_OUTPUT_PATH=data/redhat-mcp-servers-catalog.yaml
PARTNER_MCP_SERVERS_INDEX_PATH=data/partner-mcp-servers-index.yaml
PARTNER_MCP_SERVERS_CATALOG_OUTPUT_PATH=data/partner-mcp-servers-catalog.yaml
COMMUNITY_MCP_SERVERS_INDEX_PATH=data/community-mcp-servers-index.yaml
COMMUNITY_MCP_SERVERS_CATALOG_OUTPUT_PATH=data/community-mcp-servers-catalog.yaml

# Container parameters
CONTAINER_RUNTIME?=$(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null || echo docker)
DOCKER_IMAGE_NAME?=quay.io/opendatahub/odh-model-metadata-collection
DOCKER_IMAGE_TAG?=latest
DOCKER_FULL_IMAGE_NAME=$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)

.PHONY: all build build-report clean test test-coverage lint fmt vet deps check help run process process-models process-redhat-models process-validated-models process-other-models process-redhat-mcp process-partner-mcp process-community-mcp report run-with-report docker-build

# Default target
all: check build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

# Build the metadata report tool
build-report:
	@echo "Building metadata-report..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/metadata-report ./cmd/metadata-report

# Build for linux
build-linux:
	@echo "Building $(BINARY_NAME) for Linux..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_UNIX) $(MAIN_PATH)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf output/
	rm -f data/hugging-face-redhat-ai-validated-*.yaml
	rm -f $(REDHAT_MCP_SERVERS_CATALOG_OUTPUT_PATH)
	rm -f $(PARTNER_MCP_SERVERS_CATALOG_OUTPUT_PATH)
	rm -f $(COMMUNITY_MCP_SERVERS_CATALOG_OUTPUT_PATH)

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out

# Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. ./...

# Lint the code
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Format the code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Vet the code
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

# Check code formatting
fmt-check:
	@echo "Checking code formatting..."
	@files=$$($(GOFMT) -l .); \
	if [ -n "$$files" ]; then \
		echo "The following files need formatting:"; \
		echo "$$files"; \
		exit 1; \
	fi

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Run all checks
check: fmt-check vet lint


# Run the application with default settings
run: build
	@echo "Running model extractor..."
	./$(BUILD_DIR)/$(BINARY_NAME)

# Process Red Hat models index
process-redhat-models: build
	@echo "Processing Red Hat models..."
	./$(BUILD_DIR)/$(BINARY_NAME) \
		--input $(REDHAT_MODELS_INDEX_PATH) \
		--output-dir output/redhat \
		--catalog-output $(REDHAT_CATALOG_OUTPUT_PATH)

# Process validated models index
process-validated-models: build
	@echo "Processing validated models..."
	./$(BUILD_DIR)/$(BINARY_NAME) \
		--input $(VALIDATED_MODELS_INDEX_PATH) \
		--output-dir output/validated \
		--catalog-output $(VALIDATED_CATALOG_OUTPUT_PATH) \
		--skip-default-static-catalog

# Process other models index
process-other-models: build
	@echo "Processing other models..."
	./$(BUILD_DIR)/$(BINARY_NAME) \
		--input $(OTHER_MODELS_INDEX_PATH) \
		--output-dir output/other \
		--catalog-output $(OTHER_CATALOG_OUTPUT_PATH) \
		--skip-default-static-catalog

# Process all model indexes (redhat, validated, other)
process-models: process-redhat-models process-validated-models process-other-models

# Process Red Hat MCP servers with input/output paths
process-redhat-mcp: build
	@echo "Processing Red Hat MCP servers catalog..."
	./$(BUILD_DIR)/$(BINARY_NAME) \
	  	--mcp-index $(REDHAT_MCP_SERVERS_INDEX_PATH) \
	  	--mcp-catalog-output $(REDHAT_MCP_SERVERS_CATALOG_OUTPUT_PATH) \
	  	--skip-huggingface --skip-enrichment --skip-catalog

# Process Partner MCP servers with input/output paths
process-partner-mcp: build
	@echo "Processing Partner MCP servers catalog..."
	./$(BUILD_DIR)/$(BINARY_NAME) \
	  	--mcp-index $(PARTNER_MCP_SERVERS_INDEX_PATH) \
	  	--mcp-catalog-output $(PARTNER_MCP_SERVERS_CATALOG_OUTPUT_PATH) \
	  	--skip-huggingface --skip-enrichment --skip-catalog

# Process Community MCP servers with input/output paths
process-community-mcp: build
	@echo "Processing Community MCP servers catalog..."
	./$(BUILD_DIR)/$(BINARY_NAME) \
	  	--mcp-index $(COMMUNITY_MCP_SERVERS_INDEX_PATH) \
	  	--mcp-catalog-output $(COMMUNITY_MCP_SERVERS_CATALOG_OUTPUT_PATH) \
	  	--skip-huggingface --skip-enrichment --skip-catalog

# Process all model indexes and MCP server catalogs
process: process-models process-redhat-mcp process-partner-mcp process-community-mcp

# Generate metadata completeness report
report: build-report
	@echo "Generating metadata report..."
	./$(BUILD_DIR)/metadata-report

# Run full pipeline: extract + report
run-with-report: run report

# Quick development iteration
dev: fmt vet test build

# Full CI pipeline
ci: deps check test build

# Create release build
release: clean
	@echo "Creating release build..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) -ldflags="-w -s" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

# Initialize go module (only run once)
init-module:
	@echo "Initializing Go module..."
	$(GOMOD) init github.com/opendatahub-io/model-metadata-collection

# Update dependencies
update-deps:
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Generate documentation
docs:
	@echo "Generating documentation..."
	@if command -v godoc >/dev/null 2>&1; then \
		echo "Documentation server starting at http://localhost:6060"; \
		godoc -http=:6060; \
	else \
		echo "godoc not installed. Install it with: go install golang.org/x/tools/cmd/godoc@latest"; \
	fi


# Setup development environment
setup:
	@echo "Setting up development environment..."
	$(GOMOD) download
	@echo "Installing development tools..."
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOGET) golang.org/x/tools/cmd/godoc@latest

# Docker targets

# Build container image
docker-build:
	@if [ "$(CONTAINER_RUNTIME)" = "podman" ]; then \
		podman build -t $(DOCKER_FULL_IMAGE_NAME) .; \
	elif [ -f ~/.docker/config.json ]; then \
		DOCKER_BUILDKIT=1 $(CONTAINER_RUNTIME) build \
			--secret id=dockerconfig,src=$$HOME/.docker/config.json \
			--build-arg BUILDKIT_INLINE_CACHE=1 \
			-t $(DOCKER_FULL_IMAGE_NAME) .; \
	else \
		$(CONTAINER_RUNTIME) build -t $(DOCKER_FULL_IMAGE_NAME) .; \
	fi
	
# Show help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  build-report - Build the metadata report tool"
	@echo "  clean        - Clean build artifacts and output"
	@echo "  test         - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  benchmark    - Run benchmarks"
	@echo "  lint         - Run linters"
	@echo "  fmt          - Format code"
	@echo "  vet          - Run go vet"
	@echo "  fmt-check    - Check code formatting"
	@echo "  deps         - Download dependencies"
	@echo "  check        - Run all checks (fmt-check, vet, lint)"
	@echo "  run          - Run with default settings"
	@echo "  process      - Process all model indexes and MCP server catalogs"
	@echo "  process-models          - Process all model indexes (redhat, validated, other)"
	@echo "  process-redhat-models   - Process Red Hat models index only"
	@echo "  process-validated-models - Process validated models index only"
	@echo "  process-other-models    - Process other models index only"
	@echo "  process-redhat-mcp      - Process Red Hat MCP servers catalog"
	@echo "  process-partner-mcp     - Process Partner MCP servers catalog"
	@echo "  process-community-mcp   - Process Community MCP servers catalog"
	@echo "  report       - Generate metadata completeness report"
	@echo "  run-with-report - Run extraction then generate report"
	@echo "  dev          - Quick development iteration"
	@echo "  ci           - Full CI pipeline"
	@echo "  release      - Create optimized release build"
	@echo "  update-deps  - Update dependencies"
	@echo "  docs         - Generate documentation"
	@echo "  setup        - Setup development environment"
	@echo "  docker-build - Build Docker image"
	@echo "  help         - Show this help"
