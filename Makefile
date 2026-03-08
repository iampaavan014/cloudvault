# CloudVault - Multi-Cloud Kubernetes Storage Cost Intelligence
# Standardized Makefile for CNCF-compliant projects

.PHONY: all build build-agent build-cli test unittest test-coverage test-verbose fmt lint vet clean deps help release version

# Project variables
PROJECT_NAME := cloudvault
BINARY_AGENT := cloudvault-agent
BINARY_CLI   := cloudvault
BUILD_DIR    := bin
VERSION      ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0-alpha")
COMMIT       := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE   := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Go variables
GO       := go
GOFLAGS  := -v
LDFLAGS  := -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE)
GCFLAGS  := 

# Tools
GOLINT   := $(BUILD_DIR)/golangci-lint
GOLINT_VERSION := v1.64.8
KIND_CLUSTER   := kind-multi-node-cluster
HELM_CHART     := ./deploy/charts/cloudvault

# Help target - auto-documented
help: ## Display this help message
	@echo "CloudVault Build System"
	@echo "-----------------------"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

all: deps fmt vet lint build test ## Build and test everything

deps: ## Download and tidy dependencies
	@echo "📦 Tidying dependencies..."
	$(GO) mod tidy
	$(GO) mod download

fmt: ## Format Go source code
	@echo "🎨 Formatting code..."
	$(GO) fmt ./...
	@echo "✅ Code formatted"

vet: ## Run go vet
	@echo "🔍 Running go vet..."
	$(GO) vet ./...

lint: build-web ## Run golangci-lint
	@echo "🔍 Running linters..."
	@if [ ! -f $(GOLINT) ]; then \
		echo "📦 Installing golangci-lint $(GOLINT_VERSION)..."; \
		mkdir -p $(BUILD_DIR); \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(BUILD_DIR) $(GOLINT_VERSION); \
	fi
	$(GOLINT) run ./... --timeout=5m

generate: ## Generate eBPF Go code via bpf2go
	@echo "🧬 Generating eBPF code..."
	@if command -v clang >/dev/null 2>&1 && command -v llvm-strip >/dev/null 2>&1; then \
		$(GO) generate ./...; \
		echo "✅ eBPF code generated"; \
	else \
		echo "⚠️  Warning: clang or llvm-strip not found. eBPF generation skipped."; \
	fi

build: build-agent build-cli ## Build all binaries

build-agent: generate ## Build cloudvault-agent
	@echo "🔨 Building $(BINARY_AGENT)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_AGENT) cmd/agent/main.go

build-cli: build-web generate ## Build cloudvault (CLI)
	@echo "🔨 Building $(BINARY_CLI)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_CLI) cmd/cli/main.go

build-web: ## Build Web UI assets
	@echo "🎨 Building Web UI..."
	cd web && npm install && npm run build
	@mkdir -p pkg/dashboard
	@rm -rf pkg/dashboard/dist
	@cp -r web/dist pkg/dashboard/
	@echo "✅ Web UI built and assets synced"

test: build-web ## Run all tests with race detection
	@echo "🧪 Running full test suite..."
	$(GO) test -v -race -cover ./...

unittest: ## Run unit tests with coverage
	@echo "🧪 Running unit tests..."
	$(GO) test -v -short -coverprofile=coverage.out ./...
	@$(GO) tool cover -func=coverage.out | grep total | awk '{print "Total Coverage: " $$3}'

test-coverage: unittest ## Open coverage report in browser
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "📊 Report generated: coverage.html"

clean: ## Remove build artifacts
	@echo "🧹 Cleaning up..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html coverage.txt
	@rm -rf pkg/dashboard/dist
	@rm -rf web/dist
	@echo "✅ Cleaned"

docker: ## Build Docker images
	@echo "🐳 Building Docker images..."
	docker build -t cloudvault/agent:$(VERSION) -f deploy/Dockerfile .
	docker build -t cloudvault/ai:$(VERSION) -f deploy/Dockerfile.ai .
	@echo "✅ Docker images built"

release: build-web ## Create multi-platform release binaries
	@echo "📦 Building release binaries..."
	@mkdir -p $(BUILD_DIR)/release
	@for os in linux darwin; do \
		for arch in amd64 arm64; do \
			echo "  - $$os/$$arch"; \
			GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS) -s -w" -o $(BUILD_DIR)/release/$(BINARY_CLI)-$$os-$$arch cmd/cli/main.go; \
		done; \
	done
	@echo "  - windows/amd64"; \
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS) -s -w" -o $(BUILD_DIR)/release/$(BINARY_CLI)-windows-amd64.exe cmd/cli/main.go
	@echo "✅ Release binaries ready in $(BUILD_DIR)/release/"

version: ## Show version info
	@echo "CloudVault $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Date:   $(BUILD_DATE)"

production-deploy: docker ## Build, containerize, and deploy to local kind cluster
	@echo "🚀 Deploying $(PROJECT_NAME) to $(KIND_CLUSTER)..."
	@kind load docker-image cloudvault/agent:$(VERSION) --name $(KIND_CLUSTER)
	@kind load docker-image cloudvault/ai:$(VERSION) --name $(KIND_CLUSTER)
	@echo "📦 Updating Helm dependencies..."
	@helm dependency update $(HELM_CHART)
	@echo "☸️ Installing Helm chart..."
	@helm upgrade --install $(PROJECT_NAME) $(HELM_CHART) \
		-n $(PROJECT_NAME) --create-namespace \
		--set argo.enabled=true \
		--set image.tag=$(VERSION) \
		--set ai.tag=$(VERSION)
	@echo "✅ Deployment initiated"
