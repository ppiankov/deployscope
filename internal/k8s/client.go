package k8s

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const CacheDuration = 30 * time.Second

// ServiceStatus represents a single Kubernetes deployment's status.
type ServiceStatus struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Namespace     string            `json:"namespace"`
	Version       string            `json:"version"`
	Image         string            `json:"image"`
	Replicas      int32             `json:"replicas"`
	ReadyReplicas int32             `json:"ready_replicas"`
	Status        string            `json:"status"`
	Labels        map[string]string `json:"labels,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// Summary contains aggregate statistics.
type Summary struct {
	Total    int `json:"total"`
	Healthy  int `json:"healthy"`
	Degraded int `json:"degraded"`
	Down     int `json:"down"`
}

// Client wraps the Kubernetes clientset with caching.
type Client struct {
	clientset     kubernetes.Interface
	mu            sync.RWMutex
	cachedData    []ServiceStatus
	cachedSummary Summary
	cacheExpiry   time.Time
}

// NewClient creates a Client using in-cluster configuration.
func NewClient() (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Client{clientset: clientset}, nil
}

// NewClientWith creates a Client with a provided clientset (for testing).
func NewClientWith(cs kubernetes.Interface) *Client {
	return &Client{clientset: cs}
}

// FetchDeployments returns all labeled deployments with caching.
func (c *Client) FetchDeployments(ctx context.Context) ([]ServiceStatus, Summary, error) {
	c.mu.RLock()
	if c.cachedData != nil && time.Now().Before(c.cacheExpiry) {
		data, summary := c.cachedData, c.cachedSummary
		c.mu.RUnlock()
		return data, summary, nil
	}
	c.mu.RUnlock()

	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	deployments, err := c.clientset.AppsV1().Deployments("").List(fetchCtx, metav1.ListOptions{})
	if err != nil {
		return nil, Summary{}, fmt.Errorf("failed to list deployments: %w", err)
	}

	var services []ServiceStatus
	now := time.Now()
	var summary Summary

	for _, dep := range deployments.Items {
		version := dep.Spec.Template.Labels["app.kubernetes.io/version"]
		if version == "" {
			continue
		}

		image := ""
		if len(dep.Spec.Template.Spec.Containers) > 0 {
			image = dep.Spec.Template.Spec.Containers[0].Image
		}
		if imageAnnotation, ok := dep.Spec.Template.Annotations["app.kubernetes.io/image"]; ok {
			image = imageAnnotation + ":" + version
		}

		desired := int32(0)
		if dep.Spec.Replicas != nil {
			desired = *dep.Spec.Replicas
		}
		ready := dep.Status.ReadyReplicas

		status := "red"
		if ready > 0 && ready == desired {
			status = "green"
			summary.Healthy++
		} else if ready > 0 {
			status = "yellow"
			summary.Degraded++
		} else {
			summary.Down++
		}

		summary.Total++

		services = append(services, ServiceStatus{
			ID:            fmt.Sprintf("%s/%s", dep.Namespace, dep.Name),
			Name:          dep.Name,
			Namespace:     dep.Namespace,
			Version:       version,
			Image:         image,
			Replicas:      desired,
			ReadyReplicas: ready,
			Status:        status,
			Labels:        dep.Spec.Template.Labels,
			CreatedAt:     dep.CreationTimestamp.Time,
			UpdatedAt:     now,
		})
	}

	sort.Slice(services, func(i, j int) bool {
		statusOrder := map[string]int{"red": 0, "yellow": 1, "green": 2}
		if statusOrder[services[i].Status] != statusOrder[services[j].Status] {
			return statusOrder[services[i].Status] < statusOrder[services[j].Status]
		}
		return services[i].Name < services[j].Name
	})

	c.mu.Lock()
	c.cachedData = services
	c.cachedSummary = summary
	c.cacheExpiry = now.Add(CacheDuration)
	c.mu.Unlock()

	return services, summary, nil
}

// CacheExpiry returns the current cache expiry time.
func (c *Client) CacheExpiry() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cacheExpiry
}

// IsCached returns true if cached data is still valid.
func (c *Client) IsCached() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cachedData != nil && time.Now().Before(c.cacheExpiry)
}

// CheckReady verifies connectivity to the Kubernetes API.
func (c *Client) CheckReady(ctx context.Context) error {
	readyCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err := c.clientset.CoreV1().Namespaces().List(readyCtx, metav1.ListOptions{Limit: 1})
	if err != nil {
		log.Printf("Readiness check failed: %v", err)
		return err
	}
	return nil
}
