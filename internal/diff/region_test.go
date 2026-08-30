package diff

import (
	"testing"

	"github.com/kubenoops/railctl/internal/config"
)

// fieldByPath is defined in diff_test.go.

// REQ-APL-003: region drift renders as deploy.region current→desired.
func TestCompareDeployConfig_RegionDrift(t *testing.T) {
	d := config.DeployConfig{Region: "us-west1"}
	l := LiveDeployConfig{Region: "europe-west4", MultiRegion: map[string]int{"europe-west4": 1}}
	f, ok := fieldByPath(compareDeployConfig(d, l), "deploy.region")
	if !ok || f.Current != "europe-west4" || f.Desired != "us-west1" {
		t.Errorf("expected region europe-west4→us-west1, got %+v (ok=%v)", f, ok)
	}
}

// REQ-APL-002/DEC-008: omitted region produces no region diff.
func TestCompareDeployConfig_OmittedRegionNoDiff(t *testing.T) {
	d := config.DeployConfig{Replicas: 2}
	l := LiveDeployConfig{Region: "us-west1", MultiRegion: map[string]int{"us-west1": 2}}
	if _, ok := fieldByPath(compareDeployConfig(d, l), "deploy.region"); ok {
		t.Error("omitted deploy.region must not diff")
	}
}

// REQ-APL-010/DEC-024: replicas compare against the per-region live count → idempotent.
func TestCompareDeployConfig_RegionPlacedReplicasIdempotent(t *testing.T) {
	d := config.DeployConfig{Region: "us-west1", Replicas: 3}
	l := LiveDeployConfig{Region: "us-west1", MultiRegion: map[string]int{"us-west1": 3}, Replicas: 0}
	fields := compareDeployConfig(d, l)
	if len(fields) != 0 {
		t.Errorf("matching region+per-region replicas should be idempotent, got %+v", fields)
	}
}

// REQ-APL-010: a real per-region replica change is detected (not masked by flat 0).
func TestCompareDeployConfig_RegionPlacedReplicaChange(t *testing.T) {
	d := config.DeployConfig{Region: "us-west1", Replicas: 5}
	l := LiveDeployConfig{Region: "us-west1", MultiRegion: map[string]int{"us-west1": 3}}
	f, ok := fieldByPath(compareDeployConfig(d, l), "deploy.replicas")
	if !ok || f.Current != "3" || f.Desired != "5" {
		t.Errorf("expected replicas 3→5, got %+v (ok=%v)", f, ok)
	}
}

// RES-1: a replicas-only manifest against a >1-region live service emits no replicas drift.
func TestCompareDeployConfig_MultiRegionNoReplicaDrift(t *testing.T) {
	d := config.DeployConfig{Replicas: 3}                              // no region
	l := LiveDeployConfig{MultiRegion: map[string]int{"a": 1, "b": 1}} // Region == ""
	if _, ok := fieldByPath(compareDeployConfig(d, l), "deploy.replicas"); ok {
		t.Error("replicas-only manifest must not drift a multi-region service (RES-1)")
	}
}

func TestEffectiveLiveReplicas(t *testing.T) {
	cases := []struct {
		name   string
		l      LiveDeployConfig
		region string
		wantN  int
		wantOK bool
	}{
		{"default flat", LiveDeployConfig{Replicas: 4}, "", 4, true},
		{"single region", LiveDeployConfig{MultiRegion: map[string]int{"x": 2}}, "", 2, true},
		{"multi with target", LiveDeployConfig{MultiRegion: map[string]int{"a": 2, "b": 5}}, "a", 2, true},
		{"multi no target → not comparable", LiveDeployConfig{MultiRegion: map[string]int{"a": 2, "b": 5}}, "", 0, false},
		{"multi collapse to absent region → not comparable", LiveDeployConfig{MultiRegion: map[string]int{"a": 2, "b": 5}}, "c", 0, false},
	}
	for _, c := range cases {
		n, ok := effectiveLiveReplicas(c.l, c.region)
		if n != c.wantN || ok != c.wantOK {
			t.Errorf("%s: got (%d,%v), want (%d,%v)", c.name, n, ok, c.wantN, c.wantOK)
		}
	}
}

// REQ-APL-009: create fields and delete diff carry region (single-region scope).
func TestRegionInCreateAndDeleteFields(t *testing.T) {
	create, _ := fieldByPath(deployCreateFields(config.DeployConfig{Region: "us-west1"}), "deploy.region")
	if create.Desired != "us-west1" {
		t.Errorf("create fields should include deploy.region desired, got %+v", create)
	}

	single := buildDeleteChange(LiveService{Name: "api", Deploy: LiveDeployConfig{Region: "us-west1", MultiRegion: map[string]int{"us-west1": 1}}})
	if _, ok := fieldByPath(single.Fields, "deploy.region"); !ok {
		t.Error("delete diff for a single-region service should include deploy.region")
	}

	multi := buildDeleteChange(LiveService{Name: "api", Deploy: LiveDeployConfig{MultiRegion: map[string]int{"a": 1, "b": 1}}})
	if _, ok := fieldByPath(multi.Fields, "deploy.region"); ok {
		t.Error("delete diff for a multi-region service shows no single region line")
	}
}
