package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// decodeTokenRequest reads a GraphQL request body into query + variables.
func decodeTokenRequest(t *testing.T, r *http.Request) (string, map[string]any) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("reading request body: %v", err)
	}
	var req struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshaling request: %v", err)
	}
	return req.Query, req.Variables
}

func TestClient_CreateProjectToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, vars := decodeTokenRequest(t, r)
		input, ok := vars["input"].(map[string]any)
		if !ok {
			t.Fatalf("expected input object, got %T", vars["input"])
		}
		if input["projectId"] != "proj-1" || input["environmentId"] != "env-1" || input["name"] != "ci" {
			t.Errorf("unexpected input: %v", input)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"projectTokenCreate":"tok-secret-value"}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	tok, err := client.CreateProjectToken("proj-1", "env-1", "ci")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "tok-secret-value" {
		t.Errorf("expected token tok-secret-value, got %q", tok)
	}
}

func TestClient_ListProjectTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, vars := decodeTokenRequest(t, r)
		if vars["projectId"] != "proj-1" {
			t.Errorf("expected projectId proj-1, got %v", vars["projectId"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"projectTokens":{"edges":[
			{"node":{"id":"t1","name":"ci","environmentId":"env-1","createdAt":"2026-07-01T00:00:00Z","displayToken":"tok-****"}},
			{"node":{"id":"t2","name":"staging","environmentId":"env-2","createdAt":"2026-07-02T00:00:00Z","displayToken":"sta-****"}}
		]}}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	tokens, err := client.ListProjectTokens("proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].ID != "t1" || tokens[0].Name != "ci" || tokens[0].EnvironmentID != "env-1" {
		t.Errorf("unexpected first token: %+v", tokens[0])
	}
}

func TestClient_DeleteProjectToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, vars := decodeTokenRequest(t, r)
		if vars["id"] != "t1" {
			t.Errorf("expected id t1, got %v", vars["id"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"projectTokenDelete":true}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	if err := client.DeleteProjectToken("t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
