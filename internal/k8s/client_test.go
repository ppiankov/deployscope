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
	return &appsv1.Deployment{
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
						{Image: "ghcr.io/example/" + name + ":" + version},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: ready,
		},
	}
}

func newFakeStatefulSet(name, namespace, version string, replicas, ready int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(replicas),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/version": version,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Image: "ghcr.io/example/" + name + ":" + version},
					},
				},
			},
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: ready,
		},
	}
}

func newFakeDaemonSet(name, namespace, version string, desired, ready int32) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/version": version,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Image: "ghcr.io/example/" + name + ":" + version},
					},
				},
			},
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: desired,
			NumberReady:            ready,
		},
	}
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

	// All should have workload_type=deployment
	for _, svc := range services {
		if svc.WorkloadType != "deployment" {
			t.Errorf("expected workload_type=deployment, got %s", svc.WorkloadType)
		}
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

func TestFetchStatefulSets(t *testing.T) {
	objects := []runtime.Object{
		newFakeStatefulSet("postgres", "data", "15.0", 3, 3),
		newFakeStatefulSet("redis", "data", "7.2", 3, 1),
	}

	cs := fake.NewSimpleClientset(objects...)
	client := NewClientWith(cs)

	services, summary, err := client.FetchDeployments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	if summary.Healthy != 1 || summary.Degraded != 1 {
		t.Errorf("expected healthy=1 degraded=1, got healthy=%d degraded=%d", summary.Healthy, summary.Degraded)
	}

	for _, svc := range services {
		if svc.WorkloadType != "statefulset" {
			t.Errorf("expected workload_type=statefulset, got %s", svc.WorkloadType)
		}
	}
}

func TestFetchDaemonSets(t *testing.T) {
	objects := []runtime.Object{
		newFakeDaemonSet("fluentd", "logging", "1.16", 5, 5),
		newFakeDaemonSet("node-exporter", "monitoring", "1.7", 5, 3),
	}

	cs := fake.NewSimpleClientset(objects...)
	client := NewClientWith(cs)

	services, summary, err := client.FetchDeployments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	if summary.Healthy != 1 || summary.Degraded != 1 {
		t.Errorf("expected healthy=1 degraded=1, got healthy=%d degraded=%d", summary.Healthy, summary.Degraded)
	}

	for _, svc := range services {
		if svc.WorkloadType != "daemonset" {
			t.Errorf("expected workload_type=daemonset, got %s", svc.WorkloadType)
		}
	}
}

func TestFetchMixedWorkloads(t *testing.T) {
	objects := []runtime.Object{
		newFakeDeployment("api", "prod", "1.0.0", 3, 3),
		newFakeStatefulSet("postgres", "prod", "15.0", 3, 3),
		newFakeDaemonSet("fluentd", "logging", "1.16", 5, 5),
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
	if summary.Total != 3 || summary.Healthy != 3 {
		t.Errorf("expected total=3 healthy=3, got total=%d healthy=%d", summary.Total, summary.Healthy)
	}

	types := map[string]bool{}
	for _, svc := range services {
		types[svc.WorkloadType] = true
	}
	if !types["deployment"] || !types["statefulset"] || !types["daemonset"] {
		t.Errorf("expected all three workload types, got %v", types)
	}
}

func TestCacheTTL(t *testing.T) {
	cs := fake.NewSimpleClientset(newFakeDeployment("svc", "ns", "1.0", 1, 1))
	client := NewClientWith(cs)

	_, _, err := client.FetchDeployments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !client.IsCached() {
		t.Error("expected cache to be valid after fetch")
	}

	s2, _, err := client.FetchDeployments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s2) != 1 {
		t.Errorf("expected 1 cached service, got %d", len(s2))
	}

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

func TestIgnoreAnnotation(t *testing.T) {
	visible := newFakeDeployment("visible", "prod", "1.0.0", 1, 1)
	ignored := newFakeDeployment("ignored", "prod", "1.0.0", 1, 1)
	ignored.Annotations = map[string]string{"deployscope.dev/ignore": "true"}

	cs := fake.NewSimpleClientset(visible, ignored)
	client := NewClientWith(cs)

	services, summary, err := client.FetchDeployments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(services) != 1 {
		t.Fatalf("expected 1 service (ignored skipped), got %d", len(services))
	}
	if services[0].Name != "visible" {
		t.Errorf("expected visible, got %s", services[0].Name)
	}
	if summary.Total != 1 {
		t.Errorf("expected total=1, got %d", summary.Total)
	}
}

func TestAnnotationExtraction(t *testing.T) {
	dep := newFakeDeployment("svc", "prod", "1.0.0", 3, 3)
	dep.Annotations = map[string]string{
		"deployscope.dev/owner":       "team-platform",
		"deployscope.dev/tier":        "critical",
		"deployscope.dev/oncall":      "#platform-oncall",
		"deployscope.dev/depends-on":  "postgres,redis",
		"deployscope.dev/gitops-repo": "github.com/org/infra",
	}
	dep.Spec.Template.Labels["app.kubernetes.io/managed-by"] = "argocd"

	cs := fake.NewSimpleClientset(dep)
	client := NewClientWith(cs)

	services, _, err := client.FetchDeployments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}

	svc := services[0]
	if svc.Owner == nil || *svc.Owner != "team-platform" {
		t.Errorf("expected owner=team-platform, got %v", svc.Owner)
	}
	if svc.Tier == nil || *svc.Tier != "critical" {
		t.Errorf("expected tier=critical, got %v", svc.Tier)
	}
	if svc.ManagedBy == nil || *svc.ManagedBy != "argocd" {
		t.Errorf("expected managed_by=argocd, got %v", svc.ManagedBy)
	}
	if len(svc.DependsOn) != 2 || svc.DependsOn[0] != "postgres" || svc.DependsOn[1] != "redis" {
		t.Errorf("expected depends_on=[postgres,redis], got %v", svc.DependsOn)
	}
	if svc.Integration.GitOpsRepo == nil || *svc.Integration.GitOpsRepo != "github.com/org/infra" {
		t.Errorf("expected gitops_repo, got %v", svc.Integration.GitOpsRepo)
	}
	if svc.Integration.Oncall == nil || *svc.Integration.Oncall != "#platform-oncall" {
		t.Errorf("expected oncall=#platform-oncall, got %v", svc.Integration.Oncall)
	}
}

func TestComputeStatus(t *testing.T) {
	tests := []struct {
		ready, desired int32
		want           string
	}{
		{3, 3, "green"},
		{1, 3, "yellow"},
		{0, 3, "red"},
		{0, 0, "red"},
	}
	for _, tt := range tests {
		got := computeStatus(tt.ready, tt.desired)
		if got != tt.want {
			t.Errorf("computeStatus(%d, %d) = %s, want %s", tt.ready, tt.desired, got, tt.want)
		}
	}
}
