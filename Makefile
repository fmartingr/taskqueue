PROJECT_NAME := tq
CGO_ENABLED ?= 0
HOST ?= 127.0.0.1
PORT ?= 7331
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X github.com/fmartingr/taskqueue.version=$(VERSION)

GOLANGCI_LINT_VERSION := v2.12.2

.DEFAULT_GOAL := help

## help: Display this help message
.PHONY: help
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | sort

# ── Build ────────────────────────────────────────────────────────

## frontend: Build the frontend into internal/web/public/ with Bun
.PHONY: frontend
frontend:
	bun run build

## build: Build the frontend and the production binary (frontend is embedded)
.PHONY: build
build: frontend build-go

## build-go: Build the binary from the committed frontend output (no Bun needed)
.PHONY: build-go
build-go:
	CGO_ENABLED=$(CGO_ENABLED) go build -ldflags "$(LDFLAGS)" -o $(PROJECT_NAME) ./cmd/tq

## build-snapshot: Build all platforms locally via goreleaser (snapshot)
.PHONY: build-snapshot
build-snapshot:
	goreleaser build --snapshot --clean

# ── Development ──────────────────────────────────────────────────

## dev: Run the server in dev mode with Bun watching the frontend
.PHONY: dev
dev:
	@trap 'kill 0' INT TERM EXIT; \
	bun run dev & \
	DEV=1 go run ./cmd/tq serve --host $(HOST) --port $(PORT)

## serve: Build and run the production binary
.PHONY: serve
serve: build
	./$(PROJECT_NAME) serve --host $(HOST) --port $(PORT)

# ── Quality ──────────────────────────────────────────────────────

## test: Run the Go test suite
.PHONY: test
test:
	go test ./...

## test-integration: Run the tests that drive the compiled binary
.PHONY: test-integration
test-integration:
	go test -tags integration ./internal/integration/

## test-frontend: Run the frontend unit tests with Bun
.PHONY: test-frontend
test-frontend:
	bun test frontend/

## format: Format code and tidy modules
.PHONY: format
format:
	go fmt ./...
	go mod tidy

## ci-lint: Run the linter (auto-installs golangci-lint for CI)
.PHONY: ci-lint
ci-lint:
	@which golangci-lint > /dev/null 2>&1 || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	golangci-lint run ./...

## lint: Run linters
.PHONY: lint
lint: ci-lint

# ── Cleanup ──────────────────────────────────────────────────────

## clean: Remove build artifacts (the built frontend is committed and is kept)
.PHONY: clean
clean:
	rm -f $(PROJECT_NAME)
	rm -rf dist/
