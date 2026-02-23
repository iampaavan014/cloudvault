.PHONY: all build build-agent build-cli test clean deps fmt lint run help

# Variables
BINARY_AGENT=cloudvault-agent
BINARY_CLI=cloudvault
BUILD_DIR=bin
GO=go
GOFLAGS=-v
GOTEST=go test
GOFMT=gofmt
GOLINT=golangci-lint

# Build info
VERSION?=v0.1.0-alpha
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE)"

all: deps fmt build test

## help: Display this help message
help:
	@echo "CloudVault - Multi-Cloud Kubernetes Storage Cost Intelligence"
	@echo ""
	@echo "Available targets:"
	@echo "  make build        - Build all binaries"
	@echo "  make build-web    - Build Web UI assets"
	@echo "  make build-agent  - Build agent binary only"
	@echo "  make build-cli    - Build CLI binary only"
	@echo "  make test         - Run all tests"
	@echo "  make unittest     - Run unit tests with coverage"
	@echo "  make test-coverage - Generate HTML coverage report"
	@echo "  make test-verbose - Run tests with verbose output"
	@echo "  make fmt          - Format Go code"
	@echo "  make lint         - Run linters"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make deps         - Download dependencies"
	@echo "  make run          - Run CLI locally"
	@echo "  make docker       - Build Docker image"

## deps: Download Go module dependencies
deps:
	@echo "📦 Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy
	@echo "✅ Dependencies ready"

## build: Build all binaries
build: build-web build-agent build-cli
	@echo "✅ Build complete!"

## build-web: Build the CloudVault Web UI
build-web:
	@echo "🎨 Building Web UI..."
	cd web && npm install && npm run build
	@echo "📦 Copying web assets..."
	@mkdir -p pkg/dashboard
	@rm -rf pkg/dashboard/dist
	@cp -r web/dist pkg/dashboard/
	@echo "✅ Web UI built and assets copied."

## build-agent: Build the CloudVault agent
build-agent:
	@echo "🔨 Building $(BINARY_AGENT)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_AGENT) cmd/agent/main.go
	@echo "✅ Agent built: $(BUILD_DIR)/$(BINARY_AGENT)"

## build-cli: Build the CloudVault CLI
build-cli: build-web
	@echo "🔨 Building $(BINARY_CLI)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI) cmd/cli/main.go
	@echo "✅ CLI built: $(BUILD_DIR)/$(BINARY_CLI)"

## test: Run all tests
test:
	@echo "🧪 Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	@echo "✅ Tests complete"

## unittest: Run unit tests with coverage report
unittest:
	@echo "🧪 Running unit tests..."
	@$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo ""
	@echo "📊 Coverage Summary:"
	@go tool cover -func=coverage.out | grep total | awk '{print "Total Coverage: " $$3}'
	@echo "✅ Unit tests complete"

## test-coverage: Generate HTML coverage report
test-coverage: unittest
	@echo "📊 Generating HTML coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report generated: coverage.html"
	@echo "💡 Open with: open coverage.html"

## test-verbose: Run tests with verbose output
test-verbose:
	@echo "🧪 Running tests (verbose)..."
	@$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./... -count=1
	@echo "✅ Tests complete"

## fmt: Format Go code
fmt:
	@echo "🎨 Formatting code..."
	$(GOFMT) -s -w .
	@echo "✅ Code formatted"

## lint: Run linters (requires golangci-lint)
lint:
	@echo "🔍 Running linters..."
	@command -v $(GOLINT) >/dev/null 2>&1 || { \
		echo "📦 golangci-lint not found. Installing..."; \
		$(MAKE) dev-deps; \
	}
	$(GOLINT) run ./...
	@echo "✅ Linting complete"

## clean: Remove build artifacts
clean:
	@echo "🧹 Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.txt coverage.out coverage.html
	@rm -rf pkg/dashboard/dist
	@rm -rf web/dist
	@echo "✅ Clean complete"

## run: Run the CLI locally
run: build-cli
	@echo "🚀 Running CloudVault CLI..."
	./$(BUILD_DIR)/$(BINARY_CLI) cost --kubeconfig ~/.kube/config

## run-agent: Run the agent locally
run-agent: build-agent
	@echo "🚀 Running CloudVault Agent..."
	./$(BUILD_DIR)/$(BINARY_AGENT) --kubeconfig ~/.kube/config --interval 1m

## docker: Build Docker images
docker:
	@echo "🐳 Building Docker images..."
	docker build -t cloudvault/agent:$(VERSION) -f deploy/Dockerfile .
	docker build -t cloudvault/agent:latest -f deploy/Dockerfile .
	docker build -t cloudvault/ai:$(VERSION) -f deploy/Dockerfile.ai .
	docker build -t cloudvault/ai:latest -f deploy/Dockerfile.ai .
	@echo "✅ Docker images built"

## cluster-clean: Remove all Helm-managed CloudVault resources from the cluster
cluster-clean:
	@echo "🧹 Removing CloudVault Helm release..."
	-helm uninstall cloudvault -n cloudvault 2>/dev/null || true
	@echo "✅ Cluster cleaned"

## production-deploy: Build images and deploy via Helm (single command)
production-deploy: build docker
	@echo "☸️  Deploying CloudVault to cluster via Helm..."
	@# Load images into kind cluster (no registry required)
	-kind load docker-image cloudvault/agent:latest --name cloudvault-test
	-kind load docker-image cloudvault/ai:latest --name cloudvault-test
	@echo "📦 Running Helm upgrade --install..."
	helm upgrade --install cloudvault ./deploy/charts/cloudvault \
		--namespace cloudvault \
		--create-namespace \
		--set image.repository=cloudvault/agent \
		--set image.tag=latest \
		--set ai.repository=cloudvault/ai \
		--set ai.tag=latest \
		--atomic \
		--timeout 5m
	@echo ""
	@echo "✅ Production deployment complete!"
	@echo "💡 Access dashboard: kubectl port-forward svc/cloudvault-dashboard 8080:8080 -n cloudvault"
	@echo "💡 Access AI svc:    kubectl port-forward svc/cloudvault-ai 5005:5005 -n cloudvault"

## install: Install CLI to /usr/local/bin
install: build-cli
	@echo "📦 Installing CloudVault CLI..."
	sudo cp $(BUILD_DIR)/$(BINARY_CLI) /usr/local/bin/
	@echo "✅ Installed to /usr/local/bin/$(BINARY_CLI)"

## dev-deps: Install development dependencies
dev-deps:
	@echo "📦 Installing development dependencies..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "✅ Development dependencies installed"

## release: Create a release build (Linux, macOS, Windows)
release:
	@echo "📦 Building release binaries..."
	@mkdir -p $(BUILD_DIR)/release
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_CLI)-linux-amd64 cmd/cli/main.go
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_CLI)-darwin-amd64 cmd/cli/main.go
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_CLI)-darwin-arm64 cmd/cli/main.go
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_CLI)-windows-amd64.exe cmd/cli/main.go
	@echo "✅ Release binaries built in $(BUILD_DIR)/release/"

## version: Show version information
version:
	@echo "CloudVault $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Built:  $(BUILD_DATE)"
