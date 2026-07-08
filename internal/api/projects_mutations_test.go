package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestClient_CreateProject_WorkspaceToken verifies that a workspace-scoped token
// (which resolves to an empty workspace ID) creates a project by sending a null
// workspaceId — the API infers the workspace from the token, so no -w is required.
func TestClient_CreateProject_WorkspaceToken(t *testing.T) {
	var mutationVars map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		query := req["query"].(string)

		switch {
		case strings.Contains(query, "workspaces"):
			// Probe 1 (me.workspaces) denied → not an account token.
			json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]any{{"message": "Not Authorized"}}})
		case strings.Contains(query, "projectCreate"):
			if v, ok := req["variables"].(map[string]any); ok {
				mutationVars = v
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"projectCreate": map[string]any{
				"id": "proj-ws", "name": "ws-project",
				"environments": map[string]any{"edges": []map[string]any{{"node": map[string]string{"id": "env-1", "name": "production"}}}},
			}}})
		case strings.Contains(query, "projects"):
			// Probe 2 (projects listing) succeeds → workspace-scoped token.
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"projects": map[string]any{"edges": []any{}}}})
		}
	}))
	defer server.Close()

	client := NewClient("ws-token")
	client.apiURL = server.URL

	project, err := client.CreateProject("ws-project")
	if err != nil {
		t.Fatalf("CreateProject() error: %v", err)
	}
	if project.ID != "proj-ws" {
		t.Errorf("project ID = %q, expected 'proj-ws'", project.ID)
	}
	if _, present := mutationVars["workspaceId"]; present {
		t.Errorf("workspaceId should be omitted (null) for a workspace token; got %v", mutationVars["workspaceId"])
	}
}

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
