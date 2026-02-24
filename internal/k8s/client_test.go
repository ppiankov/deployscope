package k8s

import (
	"context"
	"sync"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func int32Ptr(i int32) *int32 { return &i }

func newFakeDeployment(name, namespace, version string, replicas, ready int32) *appsv1.Deployment {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(replicas),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/version": version,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Image: "registry.example.com/" + name + ":" + version},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: ready,
		},
	}
	return dep
}

func TestFetchDeployments(t *testing.T) {
	objects := []runtime.Object{
		newFakeDeployment("svc-a", "prod", "1.0.0", 3, 3),
		newFakeDeployment("svc-b", "prod", "2.0.0", 3, 1),
		newFakeDeployment("svc-c", "staging", "0.1.0", 2, 0),
	}

	cs := fake.NewSimpleClientset(objects...)
	client := NewClientWith(cs)

	services, summary, err := client.FetchDeployments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(services))
	}

	if summary.Total != 3 {
		t.Errorf("expected total=3, got %d", summary.Total)
	}
	if summary.Healthy != 1 {
		t.Errorf("expected healthy=1, got %d", summary.Healthy)
	}
	if summary.Degraded != 1 {
		t.Errorf("expected degraded=1, got %d", summary.Degraded)
	}
	if summary.Down != 1 {
		t.Errorf("expected down=1, got %d", summary.Down)
	}

	// Sorted: red first, then yellow, then green
	if services[0].Status != "red" {
		t.Errorf("expected first service status=red, got %s", services[0].Status)
	}
	if services[1].Status != "yellow" {
		t.Errorf("expected second service status=yellow, got %s", services[1].Status)
	}
	if services[2].Status != "green" {
		t.Errorf("expected third service status=green, got %s", services[2].Status)
	}
}

func TestFetchDeploymentsSkipsUnlabeled(t *testing.T) {
	labeled := newFakeDeployment("labeled", "prod", "1.0.0", 1, 1)
	unlabeled := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "unlabeled", Namespace: "prod"},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Image: "test:latest"}},
				},
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
	}

	cs := fake.NewSimpleClientset(labeled, unlabeled)
	client := NewClientWith(cs)

	services, _, err := client.FetchDeployments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(services) != 1 {
		t.Fatalf("expected 1 service (unlabeled skipped), got %d", len(services))
	}
	if services[0].Name != "labeled" {
		t.Errorf("expected service name=labeled, got %s", services[0].Name)
	}
}

func TestCacheTTL(t *testing.T) {
	cs := fake.NewSimpleClientset(newFakeDeployment("svc", "ns", "1.0", 1, 1))
	client := NewClientWith(cs)

	// First call populates cache
	_, _, err := client.FetchDeployments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !client.IsCached() {
		t.Error("expected cache to be valid after fetch")
	}

	// Second call should use cache (same result)
	s2, _, err := client.FetchDeployments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s2) != 1 {
		t.Errorf("expected 1 cached service, got %d", len(s2))
	}

	// Expire cache manually
	client.mu.Lock()
	client.cacheExpiry = time.Now().Add(-1 * time.Second)
	client.mu.Unlock()

	if client.IsCached() {
		t.Error("expected cache to be expired")
	}
}

func TestCacheConcurrency(t *testing.T) {
	cs := fake.NewSimpleClientset(newFakeDeployment("svc", "ns", "1.0", 1, 1))
	client := NewClientWith(cs)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := client.FetchDeployments(context.Background())
			if err != nil {
				t.Errorf("concurrent fetch error: %v", err)
			}
		}()
	}
	wg.Wait()
}
