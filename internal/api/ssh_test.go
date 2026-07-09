package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_GetServiceInstanceID_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)

		if got := req.Variables["environmentId"]; got != "env-1" {
			t.Errorf("environmentId = %v, want env-1", got)
		}
		if got := req.Variables["serviceId"]; got != "svc-1" {
			t.Errorf("serviceId = %v, want svc-1", got)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"serviceInstance": map[string]any{"id": "instance-abc"},
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	id, err := client.GetServiceInstanceID("env-1", "svc-1")
	if err != nil {
		t.Fatalf("GetServiceInstanceID() error: %v", err)
	}
	if id != "instance-abc" {
		t.Errorf("id = %q, want instance-abc", id)
	}
}

func TestClient_GetServiceInstanceID_NoInstance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"serviceInstance": map[string]any{"id": ""}},
		})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	if _, err := client.GetServiceInstanceID("env-1", "svc-1"); err == nil {
		t.Error("expected an error when no active instance exists")
	}
}
