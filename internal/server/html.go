package server

func getHTMLPage() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>DeployScope API v1</title>
    <style>
        body { font-family: sans-serif; max-width: 1200px; margin: 40px auto; padding: 20px; }
        h1 { color: #333; }
        .endpoint { background: #f5f5f5; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .method { display: inline-block; padding: 5px 10px; border-radius: 3px; font-weight: bold; }
        .get { background: #61affe; color: white; }
        code { background: #eee; padding: 2px 6px; border-radius: 3px; }
        .note { background: #e8f4e8; padding: 12px; border-radius: 5px; margin: 15px 0; }
    </style>
</head>
<body>
    <h1>DeployScope API v1</h1>
    <p>RESTful API for monitoring Kubernetes workloads (Deployments, StatefulSets, DaemonSets)</p>

    <div class="note">
        <strong>CLI available:</strong> <code>deployscope status --format json</code> for one-shot queries.
        See <code>deployscope --help</code> for all commands.
    </div>

    <h2>Endpoints</h2>

    <div class="endpoint">
        <span class="method get">GET</span>
        <strong>/api/v1/services</strong>
        <p>List all workloads with pagination, filtering, and sorting</p>
        <p><strong>Query parameters:</strong></p>
        <ul>
            <li><code>page</code> - page number (default: 1)</li>
            <li><code>limit</code> - page size (default: 100, max: 1000)</li>
            <li><code>namespace</code> - filter by namespace</li>
            <li><code>status</code> - filter by status (green/yellow/red)</li>
            <li><code>name</code> - search by name (contains)</li>
            <li><code>version</code> - filter by version</li>
            <li><code>type</code> - filter by workload type (deployment/statefulset/daemonset)</li>
            <li><code>sort</code> - sort field (name, namespace, version, status, replicas; prefix "-" for desc)</li>
        </ul>
        <p><strong>Example:</strong> <code>/api/v1/services?namespace=production&amp;status=green&amp;type=statefulset</code></p>
    </div>

    <div class="endpoint">
        <span class="method get">GET</span>
        <strong>/api/v1/services/{namespace}/{name}</strong>
        <p>Get details for a specific workload</p>
    </div>

    <div class="endpoint">
        <span class="method get">GET</span>
        <strong>/api/v1/summary</strong>
        <p>Get aggregate statistics (total, healthy, degraded, down)</p>
    </div>

    <div class="endpoint">
        <span class="method get">GET</span>
        <strong>/api/v1/namespaces</strong>
        <p>List all namespaces with workload counts</p>
    </div>

    <div class="endpoint">
        <span class="method get">GET</span>
        <strong>/api/v1/spec</strong>
        <p>Get OpenAPI specification</p>
    </div>

    <div class="endpoint">
        <span class="method get">GET</span>
        <strong>/metrics</strong>
        <p>Prometheus metrics (workload health gauges, HTTP request counters, Go runtime)</p>
    </div>

    <h2>Resources</h2>
    <ul>
        <li><a href="/api/v1/spec">OpenAPI Specification</a></li>
        <li><a href="/metrics">Prometheus Metrics</a></li>
        <li><a href="/health">Health Check</a></li>
        <li><a href="/ready">Readiness Check</a></li>
    </ul>

    <p><em>For agent integration see <code>docs/SKILL.md</code> in the repository</em></p>
</body>
</html>`
}
