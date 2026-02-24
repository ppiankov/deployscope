package server

import (
	"fmt"
	"net/http"
)

func getOpenAPISpec(r *http.Request) map[string]interface{} {
	baseURL := fmt.Sprintf("%s://%s", scheme(r), r.Host)

	return map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "DeployScope API",
			"description": "RESTful API for monitoring Kubernetes deployment statuses",
			"version":     APIVersion,
		},
		"servers": []map[string]interface{}{
			{"url": baseURL + "/api/v1", "description": "API v1"},
		},
		"paths": map[string]interface{}{
			"/services": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "List all services",
					"description": "List all services with pagination, filtering, and sorting",
					"parameters": []map[string]interface{}{
						{
							"name": "page", "in": "query",
							"description": "Page number",
							"schema":      map[string]interface{}{"type": "integer", "default": 1, "minimum": 1},
						},
						{
							"name": "limit", "in": "query",
							"description": "Page size",
							"schema":      map[string]interface{}{"type": "integer", "default": 100, "minimum": 1, "maximum": 1000},
						},
						{
							"name": "namespace", "in": "query",
							"description": "Filter by namespace",
							"schema":      map[string]string{"type": "string"},
						},
						{
							"name": "status", "in": "query",
							"description": "Filter by status",
							"schema":      map[string]interface{}{"type": "string", "enum": []string{"green", "yellow", "red"}},
						},
						{
							"name": "name", "in": "query",
							"description": "Search by name (contains)",
							"schema":      map[string]string{"type": "string"},
						},
						{
							"name": "version", "in": "query",
							"description": "Filter by version",
							"schema":      map[string]string{"type": "string"},
						},
						{
							"name": "sort", "in": "query",
							"description": "Sort field (name, namespace, version, status, replicas). Prefix '-' for desc",
							"schema":      map[string]interface{}{"type": "string", "example": "-name"},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Successful response",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]string{"$ref": "#/components/schemas/PaginatedResponse"},
								},
							},
						},
					},
				},
			},
			"/services/{namespace}/{name}": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Get service by ID",
					"description": "Get details for a specific service",
					"parameters": []map[string]interface{}{
						{
							"name": "namespace", "in": "path", "required": true,
							"description": "Service namespace",
							"schema":      map[string]string{"type": "string"},
						},
						{
							"name": "name", "in": "path", "required": true,
							"description": "Service name",
							"schema":      map[string]string{"type": "string"},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Successful response"},
						"404": map[string]interface{}{"description": "Service not found"},
					},
				},
			},
			"/summary": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Get summary statistics",
					"description": "Get aggregate statistics for all services",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Successful response"},
					},
				},
			},
			"/namespaces": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "List all namespaces",
					"description": "List all namespaces with service counts",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Successful response"},
					},
				},
			},
		},
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"ServiceStatus": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":             map[string]string{"type": "string", "example": "production/my-service"},
						"name":           map[string]string{"type": "string"},
						"namespace":      map[string]string{"type": "string"},
						"version":        map[string]string{"type": "string"},
						"image":          map[string]string{"type": "string"},
						"replicas":       map[string]string{"type": "integer"},
						"ready_replicas": map[string]string{"type": "integer"},
						"status":         map[string]interface{}{"type": "string", "enum": []string{"green", "yellow", "red"}},
						"created_at":     map[string]string{"type": "string", "format": "date-time"},
						"updated_at":     map[string]string{"type": "string", "format": "date-time"},
					},
				},
				"PaginatedResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"data":       map[string]interface{}{"type": "array", "items": map[string]string{"$ref": "#/components/schemas/ServiceStatus"}},
						"pagination": map[string]string{"$ref": "#/components/schemas/Pagination"},
						"summary":    map[string]string{"$ref": "#/components/schemas/Summary"},
						"meta":       map[string]string{"$ref": "#/components/schemas/Meta"},
					},
				},
				"Pagination": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"page":        map[string]string{"type": "integer"},
						"limit":       map[string]string{"type": "integer"},
						"total":       map[string]string{"type": "integer"},
						"total_pages": map[string]string{"type": "integer"},
						"has_next":    map[string]string{"type": "boolean"},
						"has_prev":    map[string]string{"type": "boolean"},
						"links":       map[string]string{"$ref": "#/components/schemas/Links"},
					},
				},
				"Links": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"self":  map[string]string{"type": "string", "format": "uri"},
						"first": map[string]string{"type": "string", "format": "uri"},
						"last":  map[string]string{"type": "string", "format": "uri"},
						"next":  map[string]string{"type": "string", "format": "uri"},
						"prev":  map[string]string{"type": "string", "format": "uri"},
					},
				},
				"Summary": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"total":    map[string]string{"type": "integer"},
						"healthy":  map[string]string{"type": "integer"},
						"degraded": map[string]string{"type": "integer"},
						"down":     map[string]string{"type": "integer"},
					},
				},
				"Meta": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"version":      map[string]string{"type": "string"},
						"timestamp":    map[string]string{"type": "string", "format": "date-time"},
						"cached":       map[string]string{"type": "boolean"},
						"cache_expiry": map[string]string{"type": "string", "format": "date-time"},
					},
				},
			},
		},
	}
}
