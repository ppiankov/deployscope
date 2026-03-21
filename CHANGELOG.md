# Changelog

All notable changes to this project will be documented in this file.

## [0.2.0] - 2026-03-21

### Added
- **Agent-native CLI** — Cobra subcommands: `serve`, `status`, `namespaces`, `init`, `doctor`, `version`
- **StatefulSet and DaemonSet support** — monitors all three workload types with type-specific health logic
- **Prometheus /metrics endpoint** — workload health gauges, HTTP request metrics, Go runtime collectors
- **Grafana dashboards** — deployment health and self-monitoring dashboard JSON files
- **Integration pointer annotations** — `deployscope.dev/owner`, `tier`, `gitops-repo`, `gitops-path`, `oncall`, `runbook`, `dashboard`, `depends-on`, `health-endpoint`, `deep-health`, `deep-health-detail`
- **Opt-out annotation** — `deployscope.dev/ignore: "true"` makes workloads invisible to agents
- **Deterministic routing** — status output includes action/reason/priority based on tier + health
- **Agent readiness score** — `doctor` reports annotation coverage and cluster readiness percentage
- **`--format json`** — all CLI commands support structured JSON output
- **`--unhealthy` filter** — status command can show only degraded/down workloads
- **`--redact` flag** — scrubs sensitive values from annotation output
- **`last_transition` timestamp** — per-workload transition time from K8s conditions
- **SKILL.md** — ANCC-compliant agent interface contract in docs/
- **GHCR Docker image** — multi-arch (linux/amd64, linux/arm64) published on tag push

### Changed
- Refactored main.go to delegate to Cobra CLI
- RBAC updated to include statefulsets and daemonsets
- Dockerfile updated to Go 1.25
- go.mod updated to Go 1.25

## [0.1.1] - 2026-03-18

### Added
- GHCR Docker image build in release workflow

### Changed
- Dockerfile Go version 1.23 → 1.25

## [0.1.0] - 2026-02-24

### Added
- Initial release
- REST API with pagination, filtering, sorting
- Embedded HTML dashboard
- In-memory cache with 30s TTL
- OpenAPI specification
- Health and readiness probes
- CORS support
- Cross-platform release binaries
