package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_ListProjects(t *testing.T) {
	// First call returns workspace ID, second returns projects
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			// GetWorkspaceID response
			w.Write([]byte(`{"data":{"me":{"workspaces":[{"id":"ws-1"}]}}}`))
			return
		}
		// ListProjects response
		w.Write([]byte(`{
			"data": {
				"projects": {
					"edges": [
						{
							"node": {
								"id": "proj-1",
								"name": "my-app",
								"updatedAt": "2024-01-01T00:00:00Z",
								"environments": {
									"edges": [
										{"node": {"id": "env-1", "name": "production"}}
									]
								},
								"services": {
									"edges": [
										{"node": {"id": "svc-1", "name": "web"}}
									]
								}
							}
						}
					]
				}
			}
		}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	projects, err := client.ListProjects()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	if projects[0].Name != "my-app" {
		t.Errorf("expected name 'my-app', got %q", projects[0].Name)
	}
	if projects[0].ID != "proj-1" {
		t.Errorf("expected ID 'proj-1', got %q", projects[0].ID)
	}
	if len(projects[0].Environments) != 1 {
		t.Errorf("expected 1 environment, got %d", len(projects[0].Environments))
	}
	if len(projects[0].Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(projects[0].Services))
	}
}

func TestClient_GetProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": {
				"project": {
					"id": "proj-1",
					"name": "my-app",
					"updatedAt": "2024-01-01T00:00:00Z",
					"environments": {
						"edges": [
							{
								"node": {
									"id": "env-1",
									"name": "production",
									"serviceInstances": {
										"edges": [
											{"node": {"serviceId": "svc-1", "serviceName": "web"}}
										]
									}
								}
							}
						]
					},
					"services": {
						"edges": [
							{"node": {"id": "svc-1", "name": "web"}}
						]
					}
				}
			}
		}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	project, err := client.GetProject("proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if project.Name != "my-app" {
		t.Errorf("expected name 'my-app', got %q", project.Name)
	}
	if len(project.Environments) != 1 {
		t.Errorf("expected 1 environment, got %d", len(project.Environments))
	}
	if len(project.Environments[0].Services) != 1 {
		t.Errorf("expected 1 service instance, got %d", len(project.Environments[0].Services))
	}
}

func TestClient_ListProjects_APIError(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			w.Write([]byte(`{"data":{"me":{"workspaces":[{"id":"ws-1"}]}}}`))
			return
		}
		w.Write([]byte(`{"errors":[{"message":"unauthorized"}]}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	_, err := client.ListProjects()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
