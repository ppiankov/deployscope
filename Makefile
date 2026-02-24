BINARY  := deployscope
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: all build test lint fmt clean deps docker-build help

all: deps fmt lint test build

build: ## Build binary
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/deployscope

test: ## Run tests with race detection
	go test -race -cover ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Format code
	gofmt -w .
	goimports -w .

deps: ## Download dependencies
	go mod download

docker-build: ## Build Docker image
	docker build -t $(BINARY):$(VERSION) .

clean: ## Clean build artifacts
	rm -rf bin/ coverage.out

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'
