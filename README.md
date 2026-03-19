# deployscope

Lightweight Kubernetes deployment monitor with REST API and embedded web UI.

## What it is

DeployScope is a read-only service that monitors all deployments in a Kubernetes cluster and provides:

- REST API with pagination, filtering, and sorting
- Embedded web page with service statuses
- Color-coded status: green (healthy), yellow (degraded), red (down)
- In-memory caching with 30s TTL
- OpenAPI specification

## What it is NOT

- Not a replacement for Prometheus/Grafana — shows current state, not history
- Not an alerting system — display only
- Not a multi-cluster solution — works within a single cluster
- No database required — everything in memory

## Philosophy

Mirror, not oracle. Presents facts, lets users decide. Minimal permissions (read-only RBAC), minimal footprint (~5MB image), minimal resources (~30MB RAM).

## Quick start

### Download binary

```bash
# From GitHub Releases
curl -LO https://github.com/ppiankov/deployscope/releases/latest/download/deployscope_linux_amd64
chmod +x deployscope_linux_amd64
```

### Deploy to Kubernetes

```bash
# 1. Apply RBAC
kubectl apply -f deploy/rbac.yaml

# 2. Deploy
kubectl apply -f deploy/example.yaml

# 3. Access locally
kubectl port-forward -n monitoring svc/deployscope 8080:80
# http://localhost:8080
```

### Docker

```bash
docker pull ghcr.io/ppiankov/deployscope:latest
```

### Build from source

```bash
make build
# binary: bin/deployscope
```

## API

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/services` | List services (pagination, filters, sorting) |
| GET | `/api/v1/services/{ns}/{name}` | Get specific service |
| GET | `/api/v1/summary` | Aggregate statistics |
| GET | `/api/v1/namespaces` | List namespaces |
| GET | `/api/v1/spec` | OpenAPI specification |
| GET | `/health` | Liveness probe |
| GET | `/ready` | Readiness probe |

### Query parameters for `/api/v1/services`

| Parameter | Description | Example |
|-----------|-------------|---------|
| `page` | Page number (default: 1) | `?page=2` |
| `limit` | Page size (default: 100, max: 1000) | `?limit=50` |
| `namespace` | Filter by namespace | `?namespace=production` |
| `status` | Filter by status | `?status=red` |
| `name` | Search by name (contains) | `?name=api` |
| `version` | Filter by version | `?version=1.2.3` |
| `sort` | Sort field (`-` prefix for desc) | `?sort=-status` |

### Example response

```json
{
  "data": [
    {
      "id": "production/my-service",
      "name": "my-service",
      "namespace": "production",
      "version": "1.2.3",
      "replicas": 3,
      "ready_replicas": 3,
      "status": "green"
    }
  ],
  "pagination": { "page": 1, "total": 42, "total_pages": 1 },
  "summary": { "total": 42, "healthy": 38, "degraded": 3, "down": 1 }
}
```

## Configuration

Via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `CORS_ORIGIN` | (empty) | Allowed CORS origin. Empty = CORS disabled |

## Architecture

```
Kubernetes API (read-only)
       |
       v
+--------------+
|  deployscope |
|              |
|  K8s client  |--> 30s cache
|  HTTP server |--> REST API + HTML
|              |
+--------------+
       |
       v
  Browser / Grafana / CI
```

Cluster requirements:
- Kubernetes >= 1.23
- RBAC: read-only access to deployments and namespaces (see `deploy/rbac.yaml`)

Only deployments with label `app.kubernetes.io/version` are monitored.

## Known limitations

- Deployments only (not StatefulSets, DaemonSets)
- Single cluster only
- In-memory cache (data refreshes on restart)
- No API authentication

## License

[MIT](LICENSE)
