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

func TestClient_RegisterSSHKey_InputShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)

		input, ok := req.Variables["input"].(map[string]any)
		if !ok {
			t.Fatalf("input variable missing or wrong type: %v", req.Variables["input"])
		}
		if input["name"] != "railctl@host" {
			t.Errorf("input.name = %v, want railctl@host", input["name"])
		}
		if input["publicKey"] != "ssh-ed25519 AAAA me" {
			t.Errorf("input.publicKey = %v", input["publicKey"])
		}
		if input["workspaceId"] != "ws-1" {
			t.Errorf("input.workspaceId = %v, want ws-1", input["workspaceId"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"sshPublicKeyCreate": map[string]any{
					"id":          "key-1",
					"name":        "railctl@host",
					"fingerprint": "SHA256:abc",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	key, err := client.RegisterSSHKey("railctl@host", "ssh-ed25519 AAAA me", "ws-1")
	if err != nil {
		t.Fatalf("RegisterSSHKey() error: %v", err)
	}
	if key.ID != "key-1" || key.Fingerprint != "SHA256:abc" {
		t.Errorf("unexpected key: %+v", key)
	}
	if key.PublicKey != "ssh-ed25519 AAAA me" {
		t.Errorf("PublicKey not carried through: %q", key.PublicKey)
	}
}

func TestClient_RegisterSSHKey_PersonalOmitsWorkspaceId(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)

		input := req.Variables["input"].(map[string]any)
		if _, present := input["workspaceId"]; present {
			t.Errorf("workspaceId should be omitted for a personal key, got %v", input["workspaceId"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"sshPublicKeyCreate": map[string]any{"id": "k", "name": "n", "fingerprint": "SHA256:x"},
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	if _, err := client.RegisterSSHKey("n", "ssh-ed25519 AAAA me", ""); err != nil {
		t.Fatalf("RegisterSSHKey() error: %v", err)
	}
}

func TestClient_ListSSHKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"sshPublicKeys": []map[string]any{
					{"id": "k1", "name": "a", "fingerprint": "SHA256:one"},
					{"id": "k2", "name": "b", "fingerprint": "SHA256:two"},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	keys, err := client.ListSSHKeys("")
	if err != nil {
		t.Fatalf("ListSSHKeys() error: %v", err)
	}
	if len(keys) != 2 || keys[0].Fingerprint != "SHA256:one" {
		t.Errorf("unexpected keys: %+v", keys)
	}
}
