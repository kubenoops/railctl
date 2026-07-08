package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_ListEnvironments_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"data": map[string]any{
				"project": map[string]any{
					"environments": map[string]any{
						"edges": []map[string]any{
							{
								"node": map[string]any{
									"id":        "env-1",
									"name":      "production",
									"updatedAt": "2024-01-15T10:30:00Z",
									"serviceInstances": map[string]any{
										"edges": []map[string]any{
											{"node": map[string]string{"serviceId": "svc-1", "serviceName": "api"}},
											{"node": map[string]string{"serviceId": "svc-2", "serviceName": "web"}},
										},
									},
								},
							},
							{
								"node": map[string]any{
									"id":        "env-2",
									"name":      "staging",
									"updatedAt": "2024-01-14T08:00:00Z",
									"serviceInstances": map[string]any{
										"edges": []map[string]any{},
									},
								},
							},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	envs, err := client.ListEnvironments("proj-1")
	if err != nil {
		t.Fatalf("ListEnvironments() error: %v", err)
	}

	if len(envs) != 2 {
		t.Fatalf("expected 2 environments, got %d", len(envs))
	}
	if envs[0].Name != "production" {
		t.Errorf("first env name = %q, expected 'production'", envs[0].Name)
	}
	if envs[0].ServiceCount != 2 {
		t.Errorf("first env ServiceCount = %d, expected 2", envs[0].ServiceCount)
	}
	if len(envs[0].Services) != 2 {
		t.Errorf("first env Services length = %d, expected 2", len(envs[0].Services))
	}
	if envs[0].Services[0].Name != "api" {
		t.Errorf("first service name = %q, expected 'api'", envs[0].Services[0].Name)
	}
	if envs[0].UpdatedAt.IsZero() {
		t.Error("first env UpdatedAt should not be zero")
	}
	if envs[1].ServiceCount != 0 {
		t.Errorf("second env ServiceCount = %d, expected 0", envs[1].ServiceCount)
	}
}

func TestClient_CreateEnvironment_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"data": map[string]any{
				"environmentCreate": map[string]any{
					"id":   "env-new",
					"name": "development",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	env, err := client.CreateEnvironment("proj-1", "development")
	if err != nil {
		t.Fatalf("CreateEnvironment() error: %v", err)
	}

	if env.ID != "env-new" {
		t.Errorf("env ID = %q, expected 'env-new'", env.ID)
	}
	if env.Name != "development" {
		t.Errorf("env Name = %q, expected 'development'", env.Name)
	}
}

func TestClient_DeleteEnvironment_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"data": map[string]any{
				"environmentDelete": true,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	err := client.DeleteEnvironment("env-1")
	if err != nil {
		t.Fatalf("DeleteEnvironment() error: %v", err)
	}
}

func TestClient_DeleteEnvironment_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"data": map[string]any{
				"environmentDelete": false,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	err := client.DeleteEnvironment("env-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to delete") {
		t.Errorf("expected 'failed to delete' error, got: %v", err)
	}
}
