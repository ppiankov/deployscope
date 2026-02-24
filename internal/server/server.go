package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ppiankov/deployscope/internal/k8s"
)

const (
	APIVersion   = "v1"
	maxPageSize  = 1000
	defaultPage  = 1
	defaultLimit = 100
)

// PaginatedResponse represents a paginated API response.
type PaginatedResponse struct {
	Data       []k8s.ServiceStatus `json:"data"`
	Pagination Pagination          `json:"pagination"`
	Summary    k8s.Summary         `json:"summary"`
	Meta       Meta                `json:"meta"`
}

// Pagination contains pagination metadata.
type Pagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int   `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
	NextPage   *int  `json:"next_page,omitempty"`
	PrevPage   *int  `json:"prev_page,omitempty"`
	Links      Links `json:"links"`
}

// Links contains HATEOAS links.
type Links struct {
	Self  string  `json:"self"`
	First string  `json:"first"`
	Last  string  `json:"last"`
	Next  *string `json:"next,omitempty"`
	Prev  *string `json:"prev,omitempty"`
}

// Meta contains response metadata.
type Meta struct {
	Version     string     `json:"version"`
	Timestamp   time.Time  `json:"timestamp"`
	Cached      bool       `json:"cached"`
	CacheExpiry *time.Time `json:"cache_expiry,omitempty"`
}

// ErrorResponse represents an API error.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
	Meta  Meta        `json:"meta"`
}

// ErrorDetail contains error details.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Server handles HTTP requests for the deployscope API.
type Server struct {
	k8s        *k8s.Client
	corsOrigin string
}

// New creates a Server.
func New(client *k8s.Client, corsOrigin string) *Server {
	return &Server{k8s: client, corsOrigin: corsOrigin}
}

// RegisterRoutes registers all HTTP routes on the given mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	mux.HandleFunc("/api/v1/services", s.handleServices)
	mux.HandleFunc("/api/v1/services/", s.handleServiceByID)
	mux.HandleFunc("/api/v1/summary", s.handleSummary)
	mux.HandleFunc("/api/v1/namespaces", s.handleNamespaces)
	mux.HandleFunc("/api/v1/spec", s.handleSpec)

	mux.HandleFunc("/api/services", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/v1/services", http.StatusMovedPermanently)
	})
}

func (s *Server) setCORS(w http.ResponseWriter) {
	if s.corsOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	}
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed", "")
		return
	}

	services, summary, err := s.k8s.FetchDeployments(r.Context())
	if err != nil {
		log.Printf("Error fetching services: %v", err)
		sendError(w, http.StatusInternalServerError, "FETCH_ERROR", "Failed to fetch services", "")
		return
	}

	cached := s.k8s.IsCached()

	query := r.URL.Query()

	filters := map[string]string{
		"namespace": query.Get("namespace"),
		"status":    query.Get("status"),
		"name":      query.Get("name"),
		"version":   query.Get("version"),
	}

	filtered := filterServices(services, filters)

	page, _ := strconv.Atoi(query.Get("page"))
	if page < 1 {
		page = defaultPage
	}

	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	sortBy := query.Get("sort")
	if sortBy != "" {
		sortServices(filtered, sortBy)
	}

	paginated, pagination := paginateServices(filtered, page, limit)
	pagination.Links = buildLinks(r, pagination)

	response := PaginatedResponse{
		Data:       paginated,
		Pagination: pagination,
		Summary:    summary,
		Meta: Meta{
			Version:   APIVersion,
			Timestamp: time.Now(),
			Cached:    cached,
		},
	}

	if cached {
		expiry := s.k8s.CacheExpiry()
		response.Meta.CacheExpiry = &expiry
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-API-Version", APIVersion)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding services response: %v", err)
	}
}

func (s *Server) handleServiceByID(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)

	if r.Method != "GET" {
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed", "")
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/services/"), "/")
	if len(parts) != 2 {
		sendError(w, http.StatusBadRequest, "INVALID_ID", "Service ID must be in format: namespace/name", "")
		return
	}

	namespace, name := parts[0], parts[1]
	id := fmt.Sprintf("%s/%s", namespace, name)

	services, _, err := s.k8s.FetchDeployments(r.Context())
	if err != nil {
		log.Printf("Error fetching services: %v", err)
		sendError(w, http.StatusInternalServerError, "FETCH_ERROR", "Failed to fetch services", "")
		return
	}

	for _, svc := range services {
		if svc.ID == id {
			response := map[string]interface{}{
				"data": svc,
				"meta": Meta{
					Version:   APIVersion,
					Timestamp: time.Now(),
					Cached:    s.k8s.IsCached(),
				},
			}

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-API-Version", APIVersion)
			if err := json.NewEncoder(w).Encode(response); err != nil {
				log.Printf("Error encoding service response: %v", err)
			}
			return
		}
	}

	sendError(w, http.StatusNotFound, "NOT_FOUND", "Service not found", fmt.Sprintf("No service with ID: %s", id))
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)

	if r.Method != "GET" {
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed", "")
		return
	}

	_, summary, err := s.k8s.FetchDeployments(r.Context())
	if err != nil {
		log.Printf("Error fetching summary: %v", err)
		sendError(w, http.StatusInternalServerError, "FETCH_ERROR", "Failed to fetch summary", "")
		return
	}

	response := map[string]interface{}{
		"data": summary,
		"meta": Meta{
			Version:   APIVersion,
			Timestamp: time.Now(),
			Cached:    s.k8s.IsCached(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-API-Version", APIVersion)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding summary response: %v", err)
	}
}

func (s *Server) handleNamespaces(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)

	if r.Method != "GET" {
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed", "")
		return
	}

	services, _, err := s.k8s.FetchDeployments(r.Context())
	if err != nil {
		log.Printf("Error fetching namespaces: %v", err)
		sendError(w, http.StatusInternalServerError, "FETCH_ERROR", "Failed to fetch namespaces", "")
		return
	}

	type NamespaceInfo struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	nsMap := make(map[string]int)
	for _, svc := range services {
		nsMap[svc.Namespace]++
	}

	var namespaces []NamespaceInfo
	for ns, count := range nsMap {
		namespaces = append(namespaces, NamespaceInfo{Name: ns, Count: count})
	}

	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})

	response := map[string]interface{}{
		"data": namespaces,
		"meta": Meta{
			Version:   APIVersion,
			Timestamp: time.Now(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-API-Version", APIVersion)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding namespaces response: %v", err)
	}
}

func (s *Server) handleSpec(w http.ResponseWriter, r *http.Request) {
	spec := getOpenAPISpec(r)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(spec); err != nil {
		log.Printf("Error encoding spec response: %v", err)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, "OK")
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.k8s.CheckReady(r.Context()); err != nil {
		http.Error(w, "Not ready", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, "Ready")
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, getHTMLPage())
}

func sendError(w http.ResponseWriter, statusCode int, code, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-API-Version", APIVersion)
	w.WriteHeader(statusCode)

	response := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
		Meta: Meta{
			Version:   APIVersion,
			Timestamp: time.Now(),
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding error response: %v", err)
	}
}

func filterServices(services []k8s.ServiceStatus, filters map[string]string) []k8s.ServiceStatus {
	var filtered []k8s.ServiceStatus

	for _, svc := range services {
		match := true

		if ns, ok := filters["namespace"]; ok && ns != "" {
			if svc.Namespace != ns {
				match = false
			}
		}

		if status, ok := filters["status"]; ok && status != "" {
			if svc.Status != status {
				match = false
			}
		}

		if name, ok := filters["name"]; ok && name != "" {
			if !strings.Contains(strings.ToLower(svc.Name), strings.ToLower(name)) {
				match = false
			}
		}

		if version, ok := filters["version"]; ok && version != "" {
			if svc.Version != version {
				match = false
			}
		}

		if match {
			filtered = append(filtered, svc)
		}
	}

	return filtered
}

func paginateServices(services []k8s.ServiceStatus, page, limit int) ([]k8s.ServiceStatus, Pagination) {
	total := len(services)
	totalPages := (total + limit - 1) / limit

	if page < 1 {
		page = 1
	}
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}

	offset := (page - 1) * limit
	end := offset + limit
	if end > total {
		end = total
	}

	var paginatedData []k8s.ServiceStatus
	if offset < total {
		paginatedData = services[offset:end]
	}

	pagination := Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}

	if pagination.HasNext {
		nextPage := page + 1
		pagination.NextPage = &nextPage
	}
	if pagination.HasPrev {
		prevPage := page - 1
		pagination.PrevPage = &prevPage
	}

	return paginatedData, pagination
}

func buildLinks(r *http.Request, pagination Pagination) Links {
	baseURL := fmt.Sprintf("%s://%s%s", scheme(r), r.Host, r.URL.Path)
	query := r.URL.Query()

	buildURL := func(page int) string {
		q := make(map[string][]string)
		for k, v := range query {
			q[k] = v
		}
		q["page"] = []string{strconv.Itoa(page)}
		params := ""
		for k, vs := range q {
			for _, v := range vs {
				if params != "" {
					params += "&"
				}
				params += k + "=" + v
			}
		}
		return fmt.Sprintf("%s?%s", baseURL, params)
	}

	links := Links{
		Self:  buildURL(pagination.Page),
		First: buildURL(1),
		Last:  buildURL(pagination.TotalPages),
	}

	if pagination.HasNext {
		next := buildURL(*pagination.NextPage)
		links.Next = &next
	}
	if pagination.HasPrev {
		prev := buildURL(*pagination.PrevPage)
		links.Prev = &prev
	}

	return links
}

func scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func sortServices(services []k8s.ServiceStatus, sortBy string) {
	desc := strings.HasPrefix(sortBy, "-")
	field := strings.TrimPrefix(sortBy, "-")

	sort.Slice(services, func(i, j int) bool {
		var less bool
		switch field {
		case "name":
			less = services[i].Name < services[j].Name
		case "namespace":
			less = services[i].Namespace < services[j].Namespace
		case "version":
			less = services[i].Version < services[j].Version
		case "status":
			statusOrder := map[string]int{"red": 0, "yellow": 1, "green": 2}
			less = statusOrder[services[i].Status] < statusOrder[services[j].Status]
		case "replicas":
			less = services[i].Replicas < services[j].Replicas
		default:
			less = services[i].Name < services[j].Name
		}

		if desc {
			return !less
		}
		return less
	})
}
