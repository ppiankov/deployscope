package k8s

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const CacheDuration = 30 * time.Second

// Integration holds deployscope.dev annotation pointers.
type Integration struct {
	GitOpsRepo *string `json:"gitops_repo"`
	GitOpsPath *string `json:"gitops_path"`
	Oncall     *string `json:"oncall"`
	Runbook    *string `json:"runbook"`
	Dashboard  *string `json:"dashboard"`
	HealthURL  *string `json:"health_endpoint"`
	DeepHealth *string `json:"deep_health"`
	DeepDetail *string `json:"deep_health_detail"`
}

// ServiceStatus represents a single Kubernetes workload's status.
type ServiceStatus struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	WorkloadType   string            `json:"workload_type"`
	Version        string            `json:"version"`
	Image          string            `json:"image"`
	Replicas       int32             `json:"replicas"`
	ReadyReplicas  int32             `json:"ready_replicas"`
	Status         string            `json:"status"`
	Labels         map[string]string `json:"labels,omitempty"`
	Owner          *string           `json:"owner"`
	Tier           *string           `json:"tier"`
	ManagedBy      *string           `json:"managed_by"`
	PartOf         *string           `json:"part_of"`
	DependsOn      []string          `json:"depends_on"`
	Integration    Integration       `json:"integration"`
	LastTransition *time.Time        `json:"last_transition"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
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

// FetchDeployments returns all labeled workloads (Deployments, StatefulSets, DaemonSets) with caching.
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

	var services []ServiceStatus
	var summary Summary

	// Fetch Deployments
	deployments, err := c.clientset.AppsV1().Deployments("").List(fetchCtx, metav1.ListOptions{})
	if err != nil {
		return nil, Summary{}, fmt.Errorf("failed to list deployments: %w", err)
	}

	for _, dep := range deployments.Items {
		version := dep.Spec.Template.Labels["app.kubernetes.io/version"]
		if version == "" {
			continue
		}

		allAnnotations := mergeAnnotations(dep.Annotations, dep.Spec.Template.Annotations)
		owner, tier, managedBy, partOf, dependsOn, integration, ignored := extractAnnotations(allAnnotations, dep.Spec.Template.Labels)
		if ignored {
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

		status := computeStatus(ready, desired)
		addToSummary(&summary, status)

		var condTimes []time.Time
		for _, cond := range dep.Status.Conditions {
			condTimes = append(condTimes, cond.LastTransitionTime.Time)
		}

		services = append(services, ServiceStatus{
			ID:             fmt.Sprintf("%s/%s", dep.Namespace, dep.Name),
			Name:           dep.Name,
			Namespace:      dep.Namespace,
			WorkloadType:   "deployment",
			Version:        version,
			Image:          image,
			Replicas:       desired,
			ReadyReplicas:  ready,
			Status:         status,
			Labels:         dep.Spec.Template.Labels,
			Owner:          owner,
			Tier:           tier,
			ManagedBy:      managedBy,
			PartOf:         partOf,
			DependsOn:      dependsOn,
			Integration:    integration,
			LastTransition: lastConditionTransition(condTimes),
			CreatedAt:      dep.CreationTimestamp.Time,
			UpdatedAt:      time.Now(),
		})
	}

	// Fetch StatefulSets
	statefulSets, err := c.clientset.AppsV1().StatefulSets("").List(fetchCtx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Warning: failed to list statefulsets: %v", err)
	} else {
		for _, ss := range statefulSets.Items {
			version := ss.Spec.Template.Labels["app.kubernetes.io/version"]
			if version == "" {
				continue
			}

			allAnnotations := mergeAnnotations(ss.Annotations, ss.Spec.Template.Annotations)
			owner, tier, managedBy, partOf, dependsOn, integration, ignored := extractAnnotations(allAnnotations, ss.Spec.Template.Labels)
			if ignored {
				continue
			}

			image := ""
			if len(ss.Spec.Template.Spec.Containers) > 0 {
				image = ss.Spec.Template.Spec.Containers[0].Image
			}

			desired := int32(0)
			if ss.Spec.Replicas != nil {
				desired = *ss.Spec.Replicas
			}
			ready := ss.Status.ReadyReplicas

			status := computeStatus(ready, desired)
			addToSummary(&summary, status)

			var ssTimes []time.Time
			for _, cond := range ss.Status.Conditions {
				ssTimes = append(ssTimes, cond.LastTransitionTime.Time)
			}

			services = append(services, ServiceStatus{
				ID:             fmt.Sprintf("%s/%s", ss.Namespace, ss.Name),
				Name:           ss.Name,
				Namespace:      ss.Namespace,
				WorkloadType:   "statefulset",
				Version:        version,
				Image:          image,
				Replicas:       desired,
				ReadyReplicas:  ready,
				Status:         status,
				Labels:         ss.Spec.Template.Labels,
				Owner:          owner,
				Tier:           tier,
				ManagedBy:      managedBy,
				PartOf:         partOf,
				DependsOn:      dependsOn,
				Integration:    integration,
				LastTransition: lastConditionTransition(ssTimes),
				CreatedAt:      ss.CreationTimestamp.Time,
				UpdatedAt:      time.Now(),
			})
		}
	}

	// Fetch DaemonSets
	daemonSets, err := c.clientset.AppsV1().DaemonSets("").List(fetchCtx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Warning: failed to list daemonsets: %v", err)
	} else {
		for _, ds := range daemonSets.Items {
			version := ds.Spec.Template.Labels["app.kubernetes.io/version"]
			if version == "" {
				continue
			}

			allAnnotations := mergeAnnotations(ds.Annotations, ds.Spec.Template.Annotations)
			owner, tier, managedBy, partOf, dependsOn, integration, ignored := extractAnnotations(allAnnotations, ds.Spec.Template.Labels)
			if ignored {
				continue
			}

			image := ""
			if len(ds.Spec.Template.Spec.Containers) > 0 {
				image = ds.Spec.Template.Spec.Containers[0].Image
			}

			desired := ds.Status.DesiredNumberScheduled
			ready := ds.Status.NumberReady

			status := computeStatus(ready, desired)
			addToSummary(&summary, status)

			var dsTimes []time.Time
			for _, cond := range ds.Status.Conditions {
				dsTimes = append(dsTimes, cond.LastTransitionTime.Time)
			}

			services = append(services, ServiceStatus{
				ID:             fmt.Sprintf("%s/%s", ds.Namespace, ds.Name),
				Name:           ds.Name,
				Namespace:      ds.Namespace,
				WorkloadType:   "daemonset",
				Version:        version,
				Image:          image,
				Replicas:       desired,
				ReadyReplicas:  ready,
				Status:         status,
				Labels:         ds.Spec.Template.Labels,
				Owner:          owner,
				Tier:           tier,
				ManagedBy:      managedBy,
				PartOf:         partOf,
				DependsOn:      dependsOn,
				Integration:    integration,
				LastTransition: lastConditionTransition(dsTimes),
				CreatedAt:      ds.CreationTimestamp.Time,
				UpdatedAt:      time.Now(),
			})
		}
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
	c.cacheExpiry = time.Now().Add(CacheDuration)
	c.mu.Unlock()

	return services, summary, nil
}

func computeStatus(ready, desired int32) string {
	if ready > 0 && ready == desired {
		return "green"
	}
	if ready > 0 {
		return "yellow"
	}
	return "red"
}

func addToSummary(summary *Summary, status string) {
	summary.Total++
	switch status {
	case "green":
		summary.Healthy++
	case "yellow":
		summary.Degraded++
	case "red":
		summary.Down++
	}
}

// lastConditionTransition returns the most recent condition transition time.
func lastConditionTransition(times []time.Time) *time.Time {
	if len(times) == 0 {
		return nil
	}
	latest := times[0]
	for _, t := range times[1:] {
		if t.After(latest) {
			latest = t
		}
	}
	return &latest
}

const annotationPrefix = "deployscope.dev/"

func mergeAnnotations(sets ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, s := range sets {
		for k, v := range s {
			merged[k] = v
		}
	}
	return merged
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func extractAnnotations(annotations map[string]string, labels map[string]string) (owner, tier, managedBy, partOf *string, dependsOn []string, integration Integration, ignored bool) {
	if annotations[annotationPrefix+"ignore"] == "true" {
		ignored = true
		return
	}

	owner = strPtr(annotations[annotationPrefix+"owner"])
	tier = strPtr(annotations[annotationPrefix+"tier"])
	managedBy = strPtr(labels["app.kubernetes.io/managed-by"])
	partOf = strPtr(labels["app.kubernetes.io/part-of"])

	if deps := annotations[annotationPrefix+"depends-on"]; deps != "" {
		for _, d := range strings.Split(deps, ",") {
			d = strings.TrimSpace(d)
			if d != "" {
				dependsOn = append(dependsOn, d)
			}
		}
	}

	integration = Integration{
		GitOpsRepo: strPtr(annotations[annotationPrefix+"gitops-repo"]),
		GitOpsPath: strPtr(annotations[annotationPrefix+"gitops-path"]),
		Oncall:     strPtr(annotations[annotationPrefix+"oncall"]),
		Runbook:    strPtr(annotations[annotationPrefix+"runbook"]),
		Dashboard:  strPtr(annotations[annotationPrefix+"dashboard"]),
		HealthURL:  strPtr(annotations[annotationPrefix+"health-endpoint"]),
		DeepHealth: strPtr(annotations[annotationPrefix+"deep-health"]),
		DeepDetail: strPtr(annotations[annotationPrefix+"deep-health-detail"]),
	}
	return
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
