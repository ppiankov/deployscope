# Project: deployscope

## Commands
- `make build` — Build binary
- `make test` — Run tests with race detection
- `make lint` — Run golangci-lint
- `make fmt` — Format with gofmt/goimports
- `make clean` — Clean build artifacts

## Architecture
- Entry: cmd/deployscope/main.go (minimal, delegates to internal/)
- internal/k8s/ — Kubernetes client, cache, deployment fetching
- internal/server/ — HTTP handlers, routes, HTML, OpenAPI spec

## Conventions
- Minimal main.go — create client, create server, ListenAndServe
- Internal packages: short single-word names (k8s, server)
- Struct-based domain models with json tags
- Standard Go formatting (gofmt/goimports)
- Version injected via LDFLAGS at build time

## Anti-Patterns
- NEVER use raw SQL without parameterization
- NEVER skip error handling — always check returned errors
- NEVER use init() functions unless absolutely necessary
- NEVER use global mutable state

## Verification
- Run `make test` after code changes (includes -race)
- Run `make lint` before marking complete
- Run `go vet ./...` for suspicious constructs
