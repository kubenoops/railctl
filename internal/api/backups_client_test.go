package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// decodeRequest reads a GraphQL request body into query + variables.
func decodeRequest(t *testing.T, r *http.Request) (string, map[string]any) {
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

func TestClient_ListVolumeBackupSchedules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, vars := decodeRequest(t, r)
		if vars["volumeInstanceId"] != "vi-1" {
			t.Errorf("expected volumeInstanceId vi-1, got %v", vars["volumeInstanceId"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"volumeInstanceBackupScheduleList":[
			{"id":"s1","name":"daily","cron":"0 0 * * *","kind":"DAILY","retentionSeconds":518400}
		]}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	schedules, err := client.ListVolumeBackupSchedules("vi-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(schedules))
	}
	if schedules[0].Kind != "DAILY" {
		t.Errorf("expected kind DAILY, got %q", schedules[0].Kind)
	}
	if schedules[0].RetentionSeconds != 518400 {
		t.Errorf("expected retention 518400, got %d", schedules[0].RetentionSeconds)
	}
}

func TestClient_SetVolumeBackupSchedules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query, vars := decodeRequest(t, r)
		if !strings.Contains(query, "volumeInstanceBackupScheduleUpdate") {
			t.Errorf("query missing mutation: %s", query)
		}
		if vars["volumeInstanceId"] != "vi-1" {
			t.Errorf("expected volumeInstanceId vi-1, got %v", vars["volumeInstanceId"])
		}
		kinds, ok := vars["kinds"].([]any)
		if !ok {
			t.Fatalf("expected kinds to be a list, got %T", vars["kinds"])
		}
		if len(kinds) != 2 || kinds[0] != "DAILY" || kinds[1] != "WEEKLY" {
			t.Errorf("unexpected kinds: %v", kinds)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"volumeInstanceBackupScheduleUpdate":true}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	if err := client.SetVolumeBackupSchedules("vi-1", []string{"DAILY", "WEEKLY"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_SetVolumeBackupSchedules_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, vars := decodeRequest(t, r)
		kinds, ok := vars["kinds"].([]any)
		if !ok {
			t.Fatalf("expected kinds to be a list (not null), got %T", vars["kinds"])
		}
		if len(kinds) != 0 {
			t.Errorf("expected empty kinds, got %v", kinds)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"volumeInstanceBackupScheduleUpdate":true}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	if err := client.SetVolumeBackupSchedules("vi-1", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ListVolumeBackups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"volumeInstanceBackupList":[
			{"id":"b1","name":"backup-1","createdAt":"2026-07-01T00:00:00Z","expiresAt":"2026-07-07T00:00:00Z","usedMB":10,"referencedMB":100,"scheduleId":"s1"},
			{"id":"b2","name":"manual","createdAt":"2026-07-02T00:00:00Z","expiresAt":"","usedMB":5,"referencedMB":50,"scheduleId":""}
		]}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	backups, err := client.ListVolumeBackups("vi-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("expected 2 backups, got %d", len(backups))
	}
	if backups[0].ScheduleID != "s1" {
		t.Errorf("expected scheduleId s1, got %q", backups[0].ScheduleID)
	}
}

func TestClient_CreateVolumeBackup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, vars := decodeRequest(t, r)
		if vars["name"] != "pre-migration" {
			t.Errorf("expected name pre-migration, got %v", vars["name"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"volumeInstanceBackupCreate":{"workflowId":"wf-1"}}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	workflowID, err := client.CreateVolumeBackup("vi-1", "pre-migration")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if workflowID != "wf-1" {
		t.Errorf("expected workflowId wf-1, got %q", workflowID)
	}
}

func TestClient_CreateVolumeBackup_NoName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, vars := decodeRequest(t, r)
		if vars["name"] != nil {
			t.Errorf("expected nil name, got %v", vars["name"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"volumeInstanceBackupCreate":{"workflowId":"wf-2"}}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	if _, err := client.CreateVolumeBackup("vi-1", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_RestoreVolumeBackup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, vars := decodeRequest(t, r)
		if vars["volumeInstanceBackupId"] != "b1" || vars["volumeInstanceId"] != "vi-1" {
			t.Errorf("unexpected vars: %v", vars)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"volumeInstanceBackupRestore":{"workflowId":"wf-1"}}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	if err := client.RestoreVolumeBackup("b1", "vi-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_DeleteVolumeBackup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"volumeInstanceBackupDelete":{"workflowId":"wf-1"}}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	if err := client.DeleteVolumeBackup("b1", "vi-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
