package apply

import (
	"io"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/config"
	"github.com/kubenoops/railctl/internal/diff"
	"github.com/kubenoops/railctl/internal/types"
)

func TestEffectiveApplyReplicas(t *testing.T) {
	single := &types.ServiceDetail{MultiRegion: map[string]int{"us-west2": 3}}
	multi := &types.ServiceDetail{MultiRegion: map[string]int{"a": 2, "b": 5}}
	def := &types.ServiceDetail{Replicas: 4}
	cases := []struct {
		name     string
		live     *types.ServiceDetail
		declared int
		target   string
		want     int
	}{
		{"declared wins", single, 7, "us-west2", 7},
		{"target live count", multi, 0, "a", 2},
		{"single-region move preserves", single, 0, "europe-west4", 3},
		{"default-placed flat", def, 0, "us-west2", 4},
		{"nil live → 1", nil, 0, "us-west2", 1},
	}
	for _, c := range cases {
		if got := effectiveApplyReplicas(c.live, c.declared, c.target); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestResolveApplyRegion(t *testing.T) {
	// Declared region forces the map path with the effective count, resolved to
	// the FULL region ID — a short name committed verbatim places the service
	// on the legacy non-metal region and breaks volume migrations.
	cfg := config.ServiceConfig{Name: "api", Deploy: config.DeployConfig{Region: "us-east4"}}
	live := &types.ServiceDetail{MultiRegion: map[string]int{"europe-west4-drams3a": 3}}
	region, repl, err := resolveApplyRegion(cfg, live, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if region == nil || *region != "us-east4-eqdc4a" || repl == nil || *repl != 3 {
		t.Errorf("declared region: got region=%v repl=%v; want us-east4-eqdc4a/3", region, repl)
	}

	// An unknown region is a hard error.
	bad := config.ServiceConfig{Name: "api", Deploy: config.DeployConfig{Region: "nowhere-central9"}}
	if _, _, err := resolveApplyRegion(bad, live, nil); err == nil {
		t.Error("unknown region must error")
	}

	// Bare replicas change on a single-region service routes through its region
	// with the new count — the LIVE key verbatim (it may be a legacy key).
	cfg2 := config.ServiceConfig{Name: "api", Deploy: config.DeployConfig{Replicas: 4}}
	live2 := &types.ServiceDetail{MultiRegion: map[string]int{"iad": 2}}
	fields := []diff.FieldDiff{{Path: "deploy.replicas", Desired: "4"}}
	region2, repl2, err := resolveApplyRegion(cfg2, live2, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if region2 == nil || *region2 != "iad" || repl2 == nil || *repl2 != 4 {
		t.Errorf("bare replicas: got region=%v repl=%v; want iad/4", region2, repl2)
	}

	// No region, no region-placed → flat path.
	if r, _, err := resolveApplyRegion(config.ServiceConfig{Name: "api"}, &types.ServiceDetail{}, nil); err != nil || r != nil {
		t.Errorf("default-placed no region should be flat path, got %v err=%v", r, err)
	}
}

// REQ-APL-001/007: apply create with deploy.region writes multiRegionConfig.
func TestApply_CreateWithRegion(t *testing.T) {
	var capRegion *string
	mock := &api.MockClient{
		CreateServiceFunc: func(_, _, name, _ string, _ *api.RegistryCredentials) (types.Service, error) {
			return types.Service{ID: "svc-1", Name: name}, nil
		},
		ListEnvironmentsFunc: func(string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		DeployServiceInstanceFunc: func(_, _ string) (string, error) { return "dep-1", nil },
		CommitMultiRegionConfigFunc: func(_, _ string, mrc map[string]any, _ string) error {
			capRegion = regionFromMRC(mrc)
			return nil
		},
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeCreate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "image", Desired: "nginx"}, {Path: "deploy.region", Desired: "us-west2"}},
	}}}
	configMap := map[string]config.ServiceConfig{"web": {Name: "web", Image: "nginx", Deploy: config.DeployConfig{Region: "us-west2"}}}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if capRegion == nil || *capRegion != "us-west2" {
		t.Errorf("expected region write us-west2, got %v", capRegion)
	}
}

// REQ-APL-007: a region-only manifest drift still writes via multiRegionConfig,
// preserving the live per-region replica count.
func TestApply_UpdateRegionPreservesReplicas(t *testing.T) {
	var capRegion *string
	var capReplicas *int
	mock := &api.MockClient{
		ListServicesFunc: func(_, _ string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web", MultiRegion: map[string]int{"europe-west4": 3}, Region: "europe-west4"}}, nil
		},
		DeployServiceInstanceFunc: func(_, _ string) (string, error) { return "dep-1", nil },
		CommitMultiRegionConfigFunc: func(_, _ string, mrc map[string]any, _ string) error {
			capRegion, capReplicas = regionAndReplicasFromMRC(mrc)
			return nil
		},
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "deploy.region", Current: "europe-west4", Desired: "us-west2"}},
	}}}
	configMap := map[string]config.ServiceConfig{"web": {Name: "web", Image: "nginx", Deploy: config.DeployConfig{Region: "us-west2"}}}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if capRegion == nil || *capRegion != "us-west2" {
		t.Errorf("expected region us-west2, got %v", capRegion)
	}
	if capReplicas == nil || *capReplicas != 3 {
		t.Errorf("expected preserved 3 replicas, got %v", capReplicas)
	}
}

// REQ-APL-006 (m3): apply MUST NOT consult RAILCTL_REGION — a manifest that omits
// deploy.region leaves placement unmanaged even when the env var is set.
func TestApply_IgnoresRailctlRegionEnv(t *testing.T) {
	t.Setenv("RAILCTL_REGION", "us-west2")
	regionWritten := false
	mock := &api.MockClient{
		ListServicesFunc: func(_, _ string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web", Replicas: 1}}, nil
		},
		DeployServiceInstanceFunc: func(_, _ string) (string, error) { return "dep-1", nil },
		CommitMultiRegionConfigFunc: func(_, _ string, _ map[string]any, _ string) error {
			regionWritten = true
			return nil
		},
	}
	// A replicas-only change on a default-placed service; manifest sets no region.
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "deploy.replicas", Current: "1", Desired: "2"}},
	}}}
	configMap := map[string]config.ServiceConfig{"web": {Name: "web", Image: "nginx", Deploy: config.DeployConfig{Replicas: 2}}}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if regionWritten {
		t.Error("apply must not write a region from RAILCTL_REGION")
	}
}

// regionFromMRC returns the target region (the entry with a non-nil value) from a
// committed multiRegionConfig patch.
func regionFromMRC(mrc map[string]any) *string {
	r, _ := regionAndReplicasFromMRC(mrc)
	return r
}

func regionAndReplicasFromMRC(mrc map[string]any) (*string, *int) {
	for region, v := range mrc {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		r := region
		if n, ok := entry["numReplicas"].(int); ok {
			return &r, &n
		}
		return &r, nil
	}
	return nil, nil
}
