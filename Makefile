# StarAPIHub Control Plane — Build & Test

DASHBOARD_DIR := dashboard
INTEGRATION_DIR := tests/integration
BINARY := $(DASHBOARD_DIR)/starapihub
VERSION := $(shell cat VERSION 2>/dev/null || echo dev)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILDINFO_PKG := github.com/starapihub/dashboard/internal/buildinfo
LDFLAGS := -X '$(BUILDINFO_PKG).Version=$(VERSION)' -X '$(BUILDINFO_PKG).BuildDate=$(BUILD_DATE)'

.PHONY: all build test test-unit test-integration lint clean help

all: build test-unit ## Build and run unit tests

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# --- Build ---

build: ## Build the starapihub CLI binary
	cd $(DASHBOARD_DIR) && go build -ldflags "$(LDFLAGS)" -o starapihub ./cmd/starapihub/

build-dashboard: ## Build the full dashboard Docker image
	cd $(DASHBOARD_DIR) && docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t starapihub/dashboard:local .

# --- Test ---

test: test-unit ## Run all tests (unit only; use test-integration for E2E)

test-unit: ## Run Go unit tests
	cd $(DASHBOARD_DIR) && go test -count=1 -timeout 120s ./...

test-integration: build ## Run integration tests against live Docker Compose stack
	cd $(INTEGRATION_DIR) && INTEGRATION=1 go test -count=1 -v -timeout 300s ./...

test-all: test-unit test-integration ## Run unit + integration tests

# --- Lint ---

lint: ## Run go vet
	cd $(DASHBOARD_DIR) && go vet ./...

# --- Clean ---

clean: ## Remove build artifacts
	rm -f $(BINARY)
	cd $(INTEGRATION_DIR)/compose && docker compose -f docker-compose.test.yml down -v --remove-orphans 2>/dev/null || true

# --- Image ---

build-patched-newapi: ## Build New-API image with Patch 001 applied
	docker build -t starapihub/new-api:patched -f deploy/Dockerfile.new-api-patched ../new-api

# --- Release ---

validate: build ## Run the full validation suite (unit + integration + smoke)
	@echo "=== Unit Tests ==="
	$(MAKE) test-unit
	@echo ""
	@echo "=== Integration Tests ==="
	$(MAKE) test-integration
	@echo ""
	@echo "=== Validation Complete ==="
	@echo "Update docs/version-matrix.md with validated versions."

rc-validate: ## Run full RC validation with evidence capture to artifacts/releases/
	bash scripts/rc-validate.sh
