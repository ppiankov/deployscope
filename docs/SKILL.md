# deployscope

Kubernetes deployment monitor — cognitive layer for autonomous agents. Mirror, not oracle.

## Install

```bash
# Homebrew
brew install ppiankov/tap/deployscope

# Binary
curl -LO https://github.com/ppiankov/deployscope/releases/latest/download/deployscope_linux_amd64.tar.gz
tar -xzf deployscope_linux_amd64.tar.gz
sudo mv deployscope /usr/local/bin/

# Go
go install github.com/ppiankov/deployscope/cmd/deployscope@latest

# Docker
docker pull ghcr.io/ppiankov/deployscope:latest
```

## Commands

### `deployscope serve`

Start HTTP server with REST API, web dashboard, Prometheus metrics.

```bash
deployscope serve --port 8080
```

**Flags:** `--port` (default: `$PORT` or `8080`)

### `deployscope status`

One-shot cluster health query. Connects to K8s, fetches all workloads, prints status, exits.

```bash
deployscope status                          # human table
deployscope status --format json            # machine-readable
deployscope status --unhealthy --format json # degraded/down only
```

**Flags:** `--format` (`table`, `json`), `--unhealthy` (filter to non-green only)

### `deployscope namespaces`

Namespace-level summary with team ownership.

```bash
deployscope namespaces --format json
```

**Flags:** `--format` (`table`, `json`)

### `deployscope init`

Generate config file and example annotation YAML.

```bash
deployscope init
# Creates: deployscope.yaml, deployscope-annotations.example.yaml
```

### `deployscope doctor`

Validate K8s connectivity, RBAC, annotation coverage, agent-readiness score.

```bash
deployscope doctor --format json
```

**Flags:** `--format` (`text`, `json`)

### `deployscope version`

```bash
deployscope version --format json
```

## Flags

All commands support `--format json` for machine-readable output. Human-readable by default.

## JSON Output

### `deployscope status --format json`

```json
{
  "summary": {
    "total": 47,
    "healthy": 44,
    "degraded": 2,
    "down": 1
  },
  "routing": {
    "action": "escalate",
    "reason": "1 critical-tier service(s) down",
    "targets": ["#platform-oncall"],
    "suggested_wo_priority": "P0"
  },
  "services": [
    {
      "id": "platform/auth-service",
      "name": "auth-service",
      "namespace": "platform",
      "workload_type": "deployment",
      "version": "1.5.0",
      "status": "red",
      "replicas": 3,
      "ready_replicas": 0,
      "owner": "team-platform",
      "tier": "critical",
      "managed_by": "argocd",
      "part_of": "auth-platform",
      "depends_on": ["postgres-platform", "redis-shared"],
      "integration": {
        "gitops_repo": "github.com/org/infra",
        "gitops_path": "clusters/prod/auth/",
        "oncall": "#platform-oncall",
        "runbook": "https://wiki.internal/auth-runbook",
        "dashboard": "https://grafana.internal/d/auth",
        "health_endpoint": null,
        "deep_health": null,
        "deep_health_detail": null
      }
    }
  ]
}
```

### `deployscope doctor --format json`

```json
{
  "k8s_connectivity": "ok",
  "total_workloads": 47,
  "annotation_coverage": {
    "owner": 0.72,
    "tier": 0.65,
    "gitops_repo": 0.55,
    "oncall": 0.51,
    "runbook": 0.38,
    "depends_on": 0.25
  },
  "agent_readiness": 0.61,
  "warnings": ["less than 30% of workloads have gitops-repo — agents cannot create PRs"]
}
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All services healthy |
| 1 | Error (K8s unreachable, RBAC denied) |
| 2 | Degraded (some services unhealthy) |

## What this does NOT do

- Does NOT mutate Kubernetes resources (read-only RBAC)
- Does NOT store history (reads current K8s state)
- Does NOT run ML or probabilistic classification
- Does NOT auto-remediate (reports state, agents decide action)
- Does NOT replace a CMDB (mirrors annotations, never invents data)
- Does NOT provide cluster discovery (single-cluster per binary)
- Does NOT diagnose root causes (use kubenow for OOM, CrashLoop, events)

## Parsing Examples

```bash
# Get all unhealthy services as JSON
deployscope status --unhealthy --format json | jq '.services[]'

# List service names that are down
deployscope status --format json | jq -r '.services[] | select(.status == "red") | .name'

# Get routing action
deployscope status --format json | jq -r '.routing.action'

# Check agent readiness score
deployscope doctor --format json | jq '.agent_readiness'

# Get GitOps repo for a specific service
deployscope status --format json | jq -r '.services[] | select(.name == "auth-service") | .integration.gitops_repo'

# List all owners across namespaces
deployscope namespaces --format json | jq -r '.[].owners[]' | sort -u
```
