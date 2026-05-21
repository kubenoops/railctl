package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_ListVolumes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": {
				"project": {
					"environments": {
						"edges": [
							{
								"node": {
									"id": "env-1",
									"volumeInstances": {
										"edges": [
											{
												"node": {
													"volume": {"id": "vol-1", "name": "my-data"},
													"mountPath": "/app/data",
													"serviceId": "svc-1",
													"currentSizeMB": 125.5,
													"sizeMB": 5000
												}
											}
										]
									}
								}
							}
						]
					}
				}
			}
		}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	volumes, err := client.ListVolumes("proj-1", "env-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(volumes) != 1 {
		t.Fatalf("expected 1 volume, got %d", len(volumes))
	}
	if volumes[0].Volume.Name != "my-data" {
		t.Errorf("expected name 'my-data', got %q", volumes[0].Volume.Name)
	}
	if volumes[0].MountPath != "/app/data" {
		t.Errorf("expected mountPath '/app/data', got %q", volumes[0].MountPath)
	}
}

func TestClient_ListVolumes_NoMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": {
				"project": {
					"environments": {
						"edges": [
							{
								"node": {
									"id": "env-other",
									"volumeInstances": {"edges": []}
								}
							}
						]
					}
				}
			}
		}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	volumes, err := client.ListVolumes("proj-1", "env-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(volumes) != 0 {
		t.Errorf("expected 0 volumes, got %d", len(volumes))
	}
}

func TestClient_CreateVolume(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": {
				"volumeCreate": {
					"id": "vol-new",
					"name": "volume_01"
				}
			}
		}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	vol, err := client.CreateVolume("proj-1", "env-1", "svc-1", "/app/data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vol.ID != "vol-new" {
		t.Errorf("expected ID 'vol-new', got %q", vol.ID)
	}
}

func TestClient_DeleteVolume(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"volumeDelete": true}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	err := client.DeleteVolume("vol-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_DeleteVolume_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"volumeDelete": false}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	err := client.DeleteVolume("vol-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestClient_UpdateVolumeName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"volumeUpdate": {"id": "vol-1", "name": "new-name"}}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	err := client.UpdateVolumeName("vol-1", "new-name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_UpdateVolumeMountPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"volumeInstanceUpdate": true}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	err := client.UpdateVolumeMountPath("vol-1", "svc-1", "env-1", "/new/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_AttachVolume(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"volumeInstanceUpdate": true}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	err := client.AttachVolume("vol-1", "svc-1", "env-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_AttachVolume_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"volumeInstanceUpdate": false}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	err := client.AttachVolume("vol-1", "svc-1", "env-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestClient_DetachVolume(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"volumeInstanceUpdate": true}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	err := client.DetachVolume("vol-1", "env-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_DetachVolume_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"volumeInstanceUpdate": false}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	err := client.DetachVolume("vol-1", "env-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
