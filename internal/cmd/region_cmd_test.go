package cmd

import (
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// --- pure helpers ---

func TestResolveRegion(t *testing.T) {
	client := &api.MockClient{ListRegionsFunc: func() ([]types.Region, error) {
		return []types.Region{
			{Name: "us-west2", ID: "us-west2"},
			{Name: "europe-west4", ID: "europe-west4-drams3a"},
		}, nil
	}}

	// Short name (case-insensitive) resolves to the full region ID.
	if got, err := resolveRegion(client, "Europe-West4"); err != nil || got != "europe-west4-drams3a" {
		t.Errorf("short-name resolve = %q, %v; want europe-west4-drams3a", got, err)
	}
	// Full ID also resolves to itself.
	if got, err := resolveRegion(client, "europe-west4-drams3a"); err != nil || got != "europe-west4-drams3a" {
		t.Errorf("full-ID resolve = %q, %v; want europe-west4-drams3a", got, err)
	}
	if _, err := resolveRegion(client, "mars-1"); err == nil || !strings.Contains(err.Error(), "us-west2") {
		t.Errorf("unknown region should list valid names, got %v", err)
	}
	if _, err := resolveRegion(client, "  "); err == nil {
		t.Error("empty region should error")
	}
}

func TestEffectiveRegionReplicas(t *testing.T) {
	five := 5
	cases := []struct {
		name     string
		live     map[string]int
		flat     int
		target   string
		explicit *int
		want     int
	}{
		{"explicit wins", map[string]int{"us-west2": 2}, 0, "us-west2", &five, 5},
		{"target live count", map[string]int{"us-west2": 3}, 0, "us-west2", nil, 3},
		{"default-placed flat", nil, 4, "us-west2", nil, 4},
		{"fallback 1", nil, 0, "us-west2", nil, 1},
		{"single-region move preserves count", map[string]int{"europe-west4": 2}, 0, "us-west2", nil, 2},
		{"multi-region collapse to absent target → 1", map[string]int{"a": 2, "b": 5}, 0, "us-west2", nil, 1},
	}
	for _, c := range cases {
		if got := effectiveRegionReplicas(c.live, c.flat, c.target, c.explicit); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestIsRegionNoop(t *testing.T) {
	if !isRegionNoop(map[string]int{"us-west2": 2}, "us-west2", 2) {
		t.Error("exact single-region match should be no-op")
	}
	if isRegionNoop(map[string]int{"us-west2": 2}, "us-west2", 3) {
		t.Error("replica mismatch is not a no-op")
	}
	if isRegionNoop(map[string]int{"us-west2": 1, "europe-west4": 1}, "us-west2", 1) {
		t.Error("multi-region is never a single-region no-op")
	}
	if isRegionNoop(nil, "us-west2", 1) {
		t.Error("default placement is always a change")
	}
}

func TestCheckRegionCollapse(t *testing.T) {
	multi := map[string]int{"a": 1, "b": 1}
	if err := checkRegionCollapse(multi, "a", false); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Errorf("multi-region collapse without force should error, got %v", err)
	}
	if err := checkRegionCollapse(multi, "a", true); err != nil {
		t.Errorf("--force should allow collapse, got %v", err)
	}
	if err := checkRegionCollapse(map[string]int{"a": 1}, "a", false); err != nil {
		t.Errorf("single-region is not a collapse, got %v", err)
	}
}

func TestCheckVolumeRegionChange(t *testing.T) {
	svcID := "svc-1"
	attached := &api.MockClient{ListVolumesFunc: func(p, e string) ([]api.VolumeInstance, error) {
		return []api.VolumeInstance{{Volume: api.Volume{Name: "data"}, MountPath: "/data", ServiceID: &svcID}}, nil
	}}
	// Without force: refused, naming the migration and --force. (REQ-VOL-100)
	err := checkVolumeRegionChange(attached, "p", "e", "svc-1", "api", false)
	if err == nil || !strings.Contains(err.Error(), "migrate") || !strings.Contains(err.Error(), "--force") {
		t.Errorf("attached volume should refuse with migration/--force message, got %v", err)
	}
	// With force: proceeds (Railway migrates the volume). (DEC-103)
	if _, err := captureStdout(t, func() error {
		return checkVolumeRegionChange(attached, "p", "e", "svc-1", "api", true)
	}); err != nil {
		t.Errorf("--force should acknowledge the migration, got %v", err)
	}
	other := "svc-2"
	ok := &api.MockClient{ListVolumesFunc: func(p, e string) ([]api.VolumeInstance, error) {
		return []api.VolumeInstance{{Volume: api.Volume{Name: "data"}, ServiceID: &other}}, nil
	}}
	if err := checkVolumeRegionChange(ok, "p", "e", "svc-1", "api", false); err != nil {
		t.Errorf("volume on another service should not block, got %v", err)
	}
}

// regionCapture records the target region + replica count from the
// environmentPatchCommit (the entry with a non-nil value is the target region).
type regionCapture struct {
	called   bool
	region   *string
	replicas *int
}

func (c *regionCapture) fromMRC(mrc map[string]any) {
	c.called = true
	for region, v := range mrc {
		entry, ok := v.(map[string]any)
		if !ok {
			continue // nil = removal
		}
		r := region
		c.region = &r
		if n, ok := entry["numReplicas"].(int); ok {
			c.replicas = &n
		}
	}
}

func baseRegionMock(cap *regionCapture) *api.MockClient {
	return &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) { return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil },
		ListEnvironmentsFunc: func(string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		ListRegionsFunc: func() ([]types.Region, error) {
			return []types.Region{
				{Name: "us-west2", ID: "us-west2"},
				{Name: "europe-west4", ID: "europe-west4-drams3a"},
			}, nil
		},
		CommitMultiRegionConfigFunc: func(_, _ string, mrc map[string]any, _ string) error {
			cap.fromMRC(mrc)
			return nil
		},
	}
}

// setRegionCmdGlobals saves/restores the globals and flag state region uses.
func setRegionCmdGlobals(t *testing.T) {
	t.Helper()
	oc, ot, op, oe, os_, oimg := newAPIClient, token, project, environment, service, serviceImage
	t.Cleanup(func() {
		newAPIClient, token, project, environment, service, serviceImage = oc, ot, op, oe, os_, oimg
		createServiceCmd.Flags().Set("region", "")
		createServiceCmd.Flags().Lookup("region").Changed = false
		updateServiceCmd.Flags().Set("region", "")
		updateServiceCmd.Flags().Lookup("region").Changed = false
		updateServiceCmd.Flags().Set("replicas", "0")
		updateServiceCmd.Flags().Lookup("replicas").Changed = false
		updateServiceCmd.Flags().Set("force", "false")
		updateServiceCmd.Flags().Lookup("force").Changed = false
	})
}

// REQ-CMD-001/002/004: create --region writes the region (default 1 replica), validated first.
func TestRunCreateService_WithRegion(t *testing.T) {
	setRegionCmdGlobals(t)
	cap := &regionCapture{}
	m := baseRegionMock(cap)
	m.CreateServiceFunc = func(_, _, name, _ string, _ *api.RegistryCredentials) (types.Service, error) {
		return types.Service{ID: "svc-1", Name: name}, nil
	}
	token, project, environment, serviceImage = "t", "my-project", "production", "nginx:latest"
	newAPIClient = func(string) api.APIClient { return m }
	createServiceCmd.Flags().Set("region", "us-west2")

	if _, err := captureStdout(t, func() error { return runCreateService(createServiceCmd, []string{"web"}) }); err != nil {
		t.Fatalf("runCreateService error: %v", err)
	}
	if !cap.called || cap.region == nil || *cap.region != "us-west2" {
		t.Fatalf("expected region write us-west2, got called=%v region=%v", cap.called, cap.region)
	}
	if cap.replicas == nil || *cap.replicas != 1 {
		t.Errorf("create --region with no --replicas should write 1, got %v", cap.replicas)
	}
}

// REQ-CMD-005: create falls back to RAILCTL_REGION.
func TestRunCreateService_RegionFromEnv(t *testing.T) {
	setRegionCmdGlobals(t)
	t.Setenv("RAILCTL_REGION", "europe-west4")
	cap := &regionCapture{}
	m := baseRegionMock(cap)
	m.CreateServiceFunc = func(_, _, name, _ string, _ *api.RegistryCredentials) (types.Service, error) {
		return types.Service{ID: "svc-1", Name: name}, nil
	}
	token, project, environment, serviceImage = "t", "my-project", "production", "nginx:latest"
	newAPIClient = func(string) api.APIClient { return m }

	if _, err := captureStdout(t, func() error { return runCreateService(createServiceCmd, []string{"web"}) }); err != nil {
		t.Fatalf("runCreateService error: %v", err)
	}
	if cap.region == nil || *cap.region != "europe-west4-drams3a" {
		t.Fatalf("expected region from env europe-west4 (→ europe-west4-drams3a), got %v", cap.region)
	}
}

// REQ-CMD-004: unknown region on create fails before the service is created.
func TestRunCreateService_UnknownRegionFailsFast(t *testing.T) {
	setRegionCmdGlobals(t)
	created := false
	m := baseRegionMock(&regionCapture{})
	m.CreateServiceFunc = func(_, _, name, _ string, _ *api.RegistryCredentials) (types.Service, error) {
		created = true
		return types.Service{ID: "svc-1", Name: name}, nil
	}
	token, project, environment, serviceImage = "t", "my-project", "production", "nginx:latest"
	newAPIClient = func(string) api.APIClient { return m }
	createServiceCmd.Flags().Set("region", "mars-1")

	err := runCreateService(createServiceCmd, []string{"web"})
	if err == nil {
		t.Fatal("expected unknown-region error")
	}
	if created {
		t.Error("service must not be created when region validation fails")
	}
}

func updateRegionMock(cap *regionCapture, live map[string]int, flat int, vols []api.VolumeInstance) *api.MockClient {
	m := baseRegionMock(cap)
	m.ListServicesFunc = func(_, _ string) ([]types.ServiceDetail, error) {
		return []types.ServiceDetail{{ID: "svc-1", Name: "api", MultiRegion: live, Replicas: flat, Region: singleRegionOf(live)}}, nil
	}
	m.ListVolumesFunc = func(_, _ string) ([]api.VolumeInstance, error) { return vols, nil }
	m.DeployServiceInstanceFunc = func(_, _ string) (string, error) { return "dep-1", nil }
	return m
}

func singleRegionOf(live map[string]int) string {
	if len(live) == 1 {
		for r := range live {
			return r
		}
	}
	return ""
}

// REQ-CMD-002/006: update --region on a 3-replica service preserves the count and deploys.
func TestRunUpdateService_RegionPreservesReplicasAndDeploys(t *testing.T) {
	setRegionCmdGlobals(t)
	cap := &regionCapture{}
	m := updateRegionMock(cap, map[string]int{"europe-west4": 3}, 0, nil)
	deployed := false
	m.DeployServiceInstanceFunc = func(_, _ string) (string, error) { deployed = true; return "dep-1", nil }
	token, project, environment = "t", "my-project", "production"
	newAPIClient = func(string) api.APIClient { return m }
	updateServiceCmd.Flags().Set("region", "us-west2")

	if _, err := captureStdout(t, func() error { return runUpdateService(updateServiceCmd, []string{"api"}) }); err != nil {
		t.Fatalf("runUpdateService error: %v", err)
	}
	if cap.region == nil || *cap.region != "us-west2" {
		t.Fatalf("expected region us-west2, got %v", cap.region)
	}
	if cap.replicas == nil || *cap.replicas != 3 {
		t.Fatalf("expected preserved 3 replicas, got %v", cap.replicas)
	}
	if !deployed {
		t.Error("region change should trigger a deployment")
	}
}

// REQ-CMD-006/DEC-022: already-in-region is a no-op (no write, no deploy).
func TestRunUpdateService_RegionNoop(t *testing.T) {
	setRegionCmdGlobals(t)
	cap := &regionCapture{}
	m := updateRegionMock(cap, map[string]int{"us-west2": 2}, 0, nil)
	deployed := false
	m.DeployServiceInstanceFunc = func(_, _ string) (string, error) { deployed = true; return "dep-1", nil }
	token, project, environment = "t", "my-project", "production"
	newAPIClient = func(string) api.APIClient { return m }
	updateServiceCmd.Flags().Set("region", "us-west2")

	out, err := captureStdout(t, func() error { return runUpdateService(updateServiceCmd, []string{"api"}) })
	if err != nil {
		t.Fatalf("runUpdateService error: %v", err)
	}
	if cap.called {
		t.Error("no-op must not write region config")
	}
	if deployed {
		t.Error("no-op must not deploy")
	}
	if !strings.Contains(out, "already in region") {
		t.Errorf("expected no-op message, got: %q", out)
	}
}

// REQ-VOL-100: region change on a volume-bound service refused without --force.
func TestRunUpdateService_RegionBlockedByVolume(t *testing.T) {
	setRegionCmdGlobals(t)
	svcID := "svc-1"
	cap := &regionCapture{}
	m := updateRegionMock(cap, nil, 1, []api.VolumeInstance{{Volume: api.Volume{Name: "data"}, MountPath: "/data", ServiceID: &svcID}})
	token, project, environment = "t", "my-project", "production"
	newAPIClient = func(string) api.APIClient { return m }
	updateServiceCmd.Flags().Set("region", "us-west2")

	err := runUpdateService(updateServiceCmd, []string{"api"})
	if err == nil || !strings.Contains(err.Error(), "migrate") || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected migration error mentioning --force, got %v", err)
	}
	if cap.called {
		t.Error("region must not be written when refused by the volume guard")
	}
}

// REQ-CMD-006 (m1): region equal but replicas differ is NOT a no-op — the write
// fires with the new count and deploys.
func TestRunUpdateService_RegionEqualReplicasDifferWrites(t *testing.T) {
	setRegionCmdGlobals(t)
	cap := &regionCapture{}
	m := updateRegionMock(cap, map[string]int{"us-west2": 2}, 0, nil)
	deployed := false
	m.DeployServiceInstanceFunc = func(_, _ string) (string, error) { deployed = true; return "dep-1", nil }
	token, project, environment = "t", "my-project", "production"
	newAPIClient = func(string) api.APIClient { return m }
	updateServiceReplicas = 5
	updateServiceCmd.Flags().Set("region", "us-west2")
	updateServiceCmd.Flags().Set("replicas", "5")

	if _, err := captureStdout(t, func() error { return runUpdateService(updateServiceCmd, []string{"api"}) }); err != nil {
		t.Fatalf("runUpdateService error: %v", err)
	}
	if !cap.called || cap.region == nil || *cap.region != "us-west2" || cap.replicas == nil || *cap.replicas != 5 {
		t.Fatalf("expected write us-west2:5, got called=%v region=%v replicas=%v", cap.called, cap.region, cap.replicas)
	}
	if !deployed {
		t.Error("a replica change on the same region should still deploy")
	}
}

// REQ-VOL-100: one --force acknowledges both the volume migration and the
// multi-region collapse — the write proceeds. (DEC-103, DEC-015)
func TestRunUpdateService_ForceProceedsWithVolumeMigration(t *testing.T) {
	setRegionCmdGlobals(t)
	svcID := "svc-1"
	cap := &regionCapture{}
	m := updateRegionMock(cap, map[string]int{"us-west2": 2, "europe-west4": 5}, 0,
		[]api.VolumeInstance{{Volume: api.Volume{Name: "data"}, MountPath: "/data", ServiceID: &svcID}})
	token, project, environment = "t", "my-project", "production"
	newAPIClient = func(string) api.APIClient { return m }
	updateServiceCmd.Flags().Set("region", "us-west2")
	updateServiceCmd.Flags().Set("force", "true")

	out, err := captureStdout(t, func() error { return runUpdateService(updateServiceCmd, []string{"api"}) })
	if err != nil {
		t.Fatalf("--force should proceed with the migration, got %v", err)
	}
	if !cap.called || cap.region == nil || *cap.region != "us-west2" {
		t.Fatalf("expected region write us-west2 with --force, got called=%v region=%v", cap.called, cap.region)
	}
	if !strings.Contains(out, "migrate") {
		t.Errorf("expected a migration warning in output, got: %q", out)
	}
}

// REQ-CMD-009: collapsing a multi-region service requires --force.
func TestRunUpdateService_MultiRegionCollapseNeedsForce(t *testing.T) {
	setRegionCmdGlobals(t)
	cap := &regionCapture{}
	m := updateRegionMock(cap, map[string]int{"us-west2": 2, "europe-west4": 5}, 0, nil)
	token, project, environment = "t", "my-project", "production"
	newAPIClient = func(string) api.APIClient { return m }
	updateServiceCmd.Flags().Set("region", "us-west2")

	if err := runUpdateService(updateServiceCmd, []string{"api"}); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected collapse error mentioning --force, got %v", err)
	}
	if cap.called {
		t.Error("collapse must not write without --force")
	}

	// With --force: writes the target region's live count (DEC-025).
	updateServiceCmd.Flags().Set("force", "true")
	if _, err := captureStdout(t, func() error { return runUpdateService(updateServiceCmd, []string{"api"}) }); err != nil {
		t.Fatalf("forced collapse error: %v", err)
	}
	if cap.region == nil || *cap.region != "us-west2" || cap.replicas == nil || *cap.replicas != 2 {
		t.Fatalf("forced collapse should write us-west2:2, got region=%v replicas=%v", cap.region, cap.replicas)
	}
}

// REQ-CMD-008: bare --replicas on a region-placed service routes through the map.
func TestRunUpdateService_BareReplicasRoutesThroughRegion(t *testing.T) {
	setRegionCmdGlobals(t)
	cap := &regionCapture{}
	m := updateRegionMock(cap, map[string]int{"us-west2": 2}, 0, nil)
	token, project, environment = "t", "my-project", "production"
	newAPIClient = func(string) api.APIClient { return m }
	updateServiceReplicas = 4
	updateServiceCmd.Flags().Set("replicas", "4")

	if _, err := captureStdout(t, func() error { return runUpdateService(updateServiceCmd, []string{"api"}) }); err != nil {
		t.Fatalf("runUpdateService error: %v", err)
	}
	if cap.region == nil || *cap.region != "us-west2" {
		t.Fatalf("bare --replicas on region-placed service should target existing region, got %v", cap.region)
	}
	if cap.replicas == nil || *cap.replicas != 4 {
		t.Fatalf("expected 4 replicas, got %v", cap.replicas)
	}
}
