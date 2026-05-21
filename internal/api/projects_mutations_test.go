package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_CreateProject_Success(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		query := req["query"].(string)

		var response map[string]any

		if strings.Contains(query, "workspaces") {
			response = map[string]any{
				"data": map[string]any{
					"me": map[string]any{
						"workspaces": []map[string]any{
							{"id": "ws-123"},
						},
					},
				},
			}
		} else if strings.Contains(query, "projectCreate") {
			response = map[string]any{
				"data": map[string]any{
					"projectCreate": map[string]any{
						"id":   "proj-new",
						"name": "my-new-project",
						"environments": map[string]any{
							"edges": []map[string]any{
								{"node": map[string]string{"id": "env-1", "name": "production"}},
							},
						},
					},
				},
			}
		}

		requestCount++
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	project, err := client.CreateProject("my-new-project")
	if err != nil {
		t.Fatalf("CreateProject() error: %v", err)
	}

	if project.ID != "proj-new" {
		t.Errorf("project ID = %q, expected 'proj-new'", project.ID)
	}
	if project.Name != "my-new-project" {
		t.Errorf("project Name = %q, expected 'my-new-project'", project.Name)
	}
	if len(project.Environments) != 1 {
		t.Errorf("expected 1 environment, got %d", len(project.Environments))
	}
}

func TestClient_DeleteProject_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		query := req["query"].(string)

		var response map[string]any

		if strings.Contains(query, "workspaces") {
			response = map[string]any{
				"data": map[string]any{
					"me": map[string]any{
						"workspaces": []map[string]any{
							{"id": "ws-123"},
						},
					},
				},
			}
		} else if strings.Contains(query, "projectDelete") {
			response = map[string]any{
				"data": map[string]any{
					"projectDelete": true,
				},
			}
		}

		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	err := client.DeleteProject("proj-1")
	if err != nil {
		t.Fatalf("DeleteProject() error: %v", err)
	}
}

func TestClient_DeleteProject_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"data": map[string]any{
				"projectDelete": false,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	err := client.DeleteProject("proj-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to delete") {
		t.Errorf("expected 'failed to delete' error, got: %v", err)
	}
}
