package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ppiankov/deployscope/internal/k8s"
)

var (
	DeploymentsTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "deployscope_workloads_total",
		Help: "Total number of monitored workloads by status",
	}, []string{"status"})

	WorkloadReplicas = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "deployscope_workload_replicas",
		Help: "Desired replicas per workload",
	}, []string{"namespace", "name", "workload_type"})

	WorkloadReadyReplicas = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "deployscope_workload_ready_replicas",
		Help: "Ready replicas per workload",
	}, []string{"namespace", "name", "workload_type"})

	WorkloadStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "deployscope_workload_status",
		Help: "Workload status (1=current status): green=healthy, yellow=degraded, red=down",
	}, []string{"namespace", "name", "workload_type", "status"})

	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "deployscope_http_requests_total",
		Help: "Total HTTP requests by method, path, and status code",
	}, []string{"method", "path", "status_code"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "deployscope_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)

// UpdateWorkloadMetrics refreshes Prometheus gauges from current K8s data.
func UpdateWorkloadMetrics(services []k8s.ServiceStatus, summary k8s.Summary) {
	// Reset per-workload metrics to handle removed workloads
	WorkloadReplicas.Reset()
	WorkloadReadyReplicas.Reset()
	WorkloadStatus.Reset()

	DeploymentsTotal.WithLabelValues("green").Set(float64(summary.Healthy))
	DeploymentsTotal.WithLabelValues("yellow").Set(float64(summary.Degraded))
	DeploymentsTotal.WithLabelValues("red").Set(float64(summary.Down))

	for _, svc := range services {
		WorkloadReplicas.WithLabelValues(svc.Namespace, svc.Name, svc.WorkloadType).Set(float64(svc.Replicas))
		WorkloadReadyReplicas.WithLabelValues(svc.Namespace, svc.Name, svc.WorkloadType).Set(float64(svc.ReadyReplicas))

		for _, s := range []string{"green", "yellow", "red"} {
			val := float64(0)
			if svc.Status == s {
				val = 1
			}
			WorkloadStatus.WithLabelValues(svc.Namespace, svc.Name, svc.WorkloadType, s).Set(val)
		}
	}
}

// Handler returns the Prometheus metrics HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// Middleware wraps an http.Handler to record request metrics.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start).Seconds()

		path := normalizePath(r.URL.Path)
		HTTPRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(rw.statusCode)).Inc()
		HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func normalizePath(path string) string {
	switch path {
	case "/", "/health", "/ready", "/metrics",
		"/api/v1/services", "/api/v1/summary",
		"/api/v1/namespaces", "/api/v1/spec":
		return path
	default:
		if len(path) > 17 && path[:17] == "/api/v1/services/" {
			return "/api/v1/services/{id}"
		}
		return "/other"
	}
}
