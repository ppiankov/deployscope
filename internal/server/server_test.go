package server

import (
	"testing"

	"github.com/ppiankov/deployscope/internal/k8s"
)

func TestFilterServicesByNamespace(t *testing.T) {
	services := []k8s.ServiceStatus{
		{Name: "a", Namespace: "prod", Status: "green"},
		{Name: "b", Namespace: "staging", Status: "green"},
		{Name: "c", Namespace: "prod", Status: "red"},
	}

	result := filterServices(services, map[string]string{"namespace": "prod"})
	if len(result) != 2 {
		t.Fatalf("expected 2 services, got %d", len(result))
	}
}

func TestFilterServicesByStatus(t *testing.T) {
	services := []k8s.ServiceStatus{
		{Name: "a", Status: "green"},
		{Name: "b", Status: "yellow"},
		{Name: "c", Status: "red"},
	}

	result := filterServices(services, map[string]string{"status": "red"})
	if len(result) != 1 {
		t.Fatalf("expected 1 service, got %d", len(result))
	}
	if result[0].Name != "c" {
		t.Errorf("expected name=c, got %s", result[0].Name)
	}
}

func TestFilterServicesByNameContains(t *testing.T) {
	services := []k8s.ServiceStatus{
		{Name: "api-gateway", Status: "green"},
		{Name: "web-frontend", Status: "green"},
		{Name: "api-users", Status: "green"},
	}

	result := filterServices(services, map[string]string{"name": "api"})
	if len(result) != 2 {
		t.Fatalf("expected 2 services matching 'api', got %d", len(result))
	}
}

func TestFilterServicesCaseInsensitive(t *testing.T) {
	services := []k8s.ServiceStatus{
		{Name: "API-Gateway", Status: "green"},
	}

	result := filterServices(services, map[string]string{"name": "api"})
	if len(result) != 1 {
		t.Fatalf("expected 1 service (case-insensitive), got %d", len(result))
	}
}

func TestFilterServicesEmptyFilters(t *testing.T) {
	services := []k8s.ServiceStatus{
		{Name: "a", Status: "green"},
		{Name: "b", Status: "red"},
	}

	result := filterServices(services, map[string]string{})
	if len(result) != 2 {
		t.Fatalf("expected 2 services (no filters), got %d", len(result))
	}
}

func TestPaginateServices(t *testing.T) {
	services := make([]k8s.ServiceStatus, 25)
	for i := range services {
		services[i] = k8s.ServiceStatus{Name: "svc"}
	}

	data, pagination := paginateServices(services, 1, 10)
	if len(data) != 10 {
		t.Errorf("expected 10 items, got %d", len(data))
	}
	if pagination.Total != 25 {
		t.Errorf("expected total=25, got %d", pagination.Total)
	}
	if pagination.TotalPages != 3 {
		t.Errorf("expected 3 pages, got %d", pagination.TotalPages)
	}
	if !pagination.HasNext {
		t.Error("expected HasNext=true")
	}
	if pagination.HasPrev {
		t.Error("expected HasPrev=false for page 1")
	}
}

func TestPaginateServicesLastPage(t *testing.T) {
	services := make([]k8s.ServiceStatus, 25)
	for i := range services {
		services[i] = k8s.ServiceStatus{Name: "svc"}
	}

	data, pagination := paginateServices(services, 3, 10)
	if len(data) != 5 {
		t.Errorf("expected 5 items on last page, got %d", len(data))
	}
	if pagination.HasNext {
		t.Error("expected HasNext=false on last page")
	}
	if !pagination.HasPrev {
		t.Error("expected HasPrev=true on page 3")
	}
}

func TestPaginateServicesEmpty(t *testing.T) {
	data, pagination := paginateServices(nil, 1, 10)
	if len(data) != 0 {
		t.Errorf("expected 0 items, got %d", len(data))
	}
	if pagination.Total != 0 {
		t.Errorf("expected total=0, got %d", pagination.Total)
	}
}

func TestPaginateServicesPageBeyondTotal(t *testing.T) {
	services := make([]k8s.ServiceStatus, 5)
	for i := range services {
		services[i] = k8s.ServiceStatus{Name: "svc"}
	}

	data, pagination := paginateServices(services, 100, 10)
	if len(data) != 5 {
		t.Errorf("expected 5 items (clamped to last page), got %d", len(data))
	}
	if pagination.Page != 1 {
		t.Errorf("expected page=1 (clamped), got %d", pagination.Page)
	}
}

func TestSortServicesByName(t *testing.T) {
	services := []k8s.ServiceStatus{
		{Name: "charlie"},
		{Name: "alpha"},
		{Name: "bravo"},
	}

	sortServices(services, "name")
	if services[0].Name != "alpha" || services[1].Name != "bravo" || services[2].Name != "charlie" {
		t.Errorf("sort by name failed: %v", services)
	}
}

func TestSortServicesDescending(t *testing.T) {
	services := []k8s.ServiceStatus{
		{Name: "alpha"},
		{Name: "charlie"},
		{Name: "bravo"},
	}

	sortServices(services, "-name")
	if services[0].Name != "charlie" || services[1].Name != "bravo" || services[2].Name != "alpha" {
		t.Errorf("sort by -name failed: %v", services)
	}
}

func TestSortServicesByStatus(t *testing.T) {
	services := []k8s.ServiceStatus{
		{Name: "a", Status: "green"},
		{Name: "b", Status: "red"},
		{Name: "c", Status: "yellow"},
	}

	sortServices(services, "status")
	if services[0].Status != "red" || services[1].Status != "yellow" || services[2].Status != "green" {
		t.Errorf("sort by status failed: got %s, %s, %s", services[0].Status, services[1].Status, services[2].Status)
	}
}
