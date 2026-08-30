package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// REQ-API-104: ListRegions returns the hardcoded canonical set (no API call).
func TestListRegions_Hardcoded(t *testing.T) {
	// A client with an unreachable URL proves no network call is made.
	client := NewClient("test-token")
	client.apiURL = "http://127.0.0.1:0"

	regions, err := client.ListRegions()
	if err != nil {
		t.Fatalf("ListRegions() error = %v", err)
	}
	if len(regions) != 4 {
		t.Fatalf("expected 4 hardcoded regions, got %d", len(regions))
	}
	// Name is the short, user-facing form; ID is the full wire ID.
	wantID := map[string]string{
		"us-west2":        "us-west2",
		"us-east4":        "us-east4-eqdc4a",
		"europe-west4":    "europe-west4-drams3a",
		"asia-southeast1": "asia-southeast1-eqsg3a",
	}
	for _, r := range regions {
		id, ok := wantID[r.Name]
		if !ok {
			t.Errorf("unexpected region %q", r.Name)
			continue
		}
		if r.ID != id {
			t.Errorf("region %q: ID = %q, want %q", r.Name, r.ID, id)
		}
		if r.Location == "" {
			t.Errorf("region %q missing location", r.Name)
		}
	}
	if ShortRegionName("asia-southeast1-eqsg3a") != "asia-southeast1" {
		t.Error("ShortRegionName should strip the datacenter suffix")
	}
}

// REQ-API-101: CommitMultiRegionConfig sends an environmentPatchCommit whose
// patch sets services.<id>.deploy.multiRegionConfig.
func TestCommitMultiRegionConfig_BuildsPatch(t *testing.T) {
	var vars map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		json.Unmarshal(body, &req)
		vars = req.Variables
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"environmentPatchCommit":"commit/ref"}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	mrc := map[string]any{
		"europe-west4-drams3a": map[string]any{"numReplicas": 2},
		"iad":                  nil, // removal
	}
	if err := client.CommitMultiRegionConfig("env-1", "svc-1", mrc, "set region"); err != nil {
		t.Fatalf("CommitMultiRegionConfig() error = %v", err)
	}

	if vars["environmentId"] != "env-1" {
		t.Errorf("environmentId = %v, want env-1", vars["environmentId"])
	}
	patch, _ := vars["patch"].(map[string]any)
	svcs, _ := patch["services"].(map[string]any)
	svc, _ := svcs["svc-1"].(map[string]any)
	deploy, _ := svc["deploy"].(map[string]any)
	got, ok := deploy["multiRegionConfig"].(map[string]any)
	if !ok {
		t.Fatalf("patch missing services.svc-1.deploy.multiRegionConfig: %v", patch)
	}
	entry, _ := got["europe-west4-drams3a"].(map[string]any)
	if entry["numReplicas"] != float64(2) {
		t.Errorf("numReplicas = %v, want 2", entry["numReplicas"])
	}
	if v, present := got["iad"]; !present || v != nil {
		t.Errorf("expected iad:null (removal), got present=%v val=%v", present, v)
	}
}

// REQ-API-102: placement is read from the deployment meta.
func TestMultiRegionFromMeta(t *testing.T) {
	meta := map[string]any{
		"serviceManifest": map[string]any{
			"deploy": map[string]any{
				"multiRegionConfig": map[string]any{
					"us-west2":             map[string]any{"numReplicas": float64(3)},
					"europe-west4-drams3a": map[string]any{"numReplicas": float64(1)},
				},
			},
		},
	}
	got := multiRegionFromMeta(meta)
	if got["us-west2"] != 3 || got["europe-west4-drams3a"] != 1 {
		t.Errorf("unexpected parse: %v", got)
	}

	if multiRegionFromMeta(map[string]any{}) != nil {
		t.Error("empty meta should yield nil")
	}
	if multiRegionFromMeta("not-a-map") != nil {
		t.Error("non-map meta should yield nil")
	}
}

// toServiceDetail derives the single Region only when exactly one region is present.
func TestToServiceDetail_RegionFromMeta(t *testing.T) {
	mk := func(mrc map[string]any) serviceNode {
		return serviceNode{ServiceInstances: serviceInstances{Edges: []serviceInstanceEdge{{Node: serviceInstanceNode{
			EnvironmentID: "env-1",
			LatestDeployment: &serviceInstanceDeployment{
				Meta: map[string]any{"serviceManifest": map[string]any{"deploy": map[string]any{"multiRegionConfig": mrc}}},
			},
		}}}}}
	}
	single := mk(map[string]any{"us-west2": map[string]any{"numReplicas": float64(2)}}).toServiceDetail("env-1")
	if single.Region != "us-west2" || single.MultiRegion["us-west2"] != 2 {
		t.Errorf("single: Region=%q MultiRegion=%v", single.Region, single.MultiRegion)
	}
	multi := mk(map[string]any{"us-west2": map[string]any{"numReplicas": float64(1)}, "iad": map[string]any{"numReplicas": float64(1)}}).toServiceDetail("env-1")
	if multi.Region != "" || len(multi.MultiRegion) != 2 {
		t.Errorf("multi: Region should be empty, MultiRegion should have 2: %q %v", multi.Region, multi.MultiRegion)
	}
}
