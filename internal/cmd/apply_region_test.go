package cmd

import (
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/diff"
)

func regionChangeSet() *diff.ChangeSet {
	return &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type:        diff.ChangeUpdate,
		ServiceName: "api",
		Fields:      []diff.FieldDiff{{Path: "deploy.region", Current: "europe-west4", Desired: "us-west1"}},
	}}}
}

// REQ-VOL-101 (REQ-APL-004 mechanics): a volume-bound region change fails the
// whole apply without --force (Railway migrates the volume, with downtime) and
// proceeds with it.
func TestPreflightRegionChanges_VolumeGuard(t *testing.T) {
	live := []diff.LiveService{{
		Name:    "api",
		Deploy:  diff.LiveDeployConfig{Region: "europe-west4", MultiRegion: map[string]int{"europe-west4": 1}},
		Volumes: []diff.LiveVolume{{MountPath: "/data"}},
	}}
	err := preflightRegionChanges(regionChangeSet(), live, false)
	if err == nil || !strings.Contains(err.Error(), "migrate") || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected migration error mentioning --force, got %v", err)
	}
	// --force acknowledges the migration and proceeds. (DEC-103)
	if err := preflightRegionChanges(regionChangeSet(), live, true); err != nil {
		t.Errorf("--force should allow the volume migration, got %v", err)
	}
}

// One --force acknowledges both consequences: volume migration AND collapsing
// a multi-region service. (REQ-VOL-101, DEC-015)
func TestPreflightRegionChanges_ForceVolumeMultiRegion(t *testing.T) {
	live := []diff.LiveService{{
		Name:    "api",
		Deploy:  diff.LiveDeployConfig{MultiRegion: map[string]int{"a": 2, "b": 5}},
		Volumes: []diff.LiveVolume{{MountPath: "/data"}},
	}}
	if err := preflightRegionChanges(regionChangeSet(), live, true); err != nil {
		t.Fatalf("--force should allow volume migration + collapse, got %v", err)
	}
	if err := preflightRegionChanges(regionChangeSet(), live, false); err == nil {
		t.Fatal("without --force a volume-bound region change must be refused")
	}
}

// REQ-APL-008: multi-region collapse needs --force.
func TestPreflightRegionChanges_CollapseGuard(t *testing.T) {
	live := []diff.LiveService{{
		Name:   "api",
		Deploy: diff.LiveDeployConfig{MultiRegion: map[string]int{"a": 2, "b": 5}},
	}}
	if err := preflightRegionChanges(regionChangeSet(), live, false); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected collapse error mentioning --force, got %v", err)
	}
	if err := preflightRegionChanges(regionChangeSet(), live, true); err != nil {
		t.Errorf("--force should allow collapse, got %v", err)
	}
}

// A single-region move with no volume passes the pre-flight.
func TestPreflightRegionChanges_AllowsCleanMove(t *testing.T) {
	live := []diff.LiveService{{
		Name:   "api",
		Deploy: diff.LiveDeployConfig{Region: "europe-west4", MultiRegion: map[string]int{"europe-west4": 1}},
	}}
	if err := preflightRegionChanges(regionChangeSet(), live, false); err != nil {
		t.Errorf("clean single-region move should pass pre-flight, got %v", err)
	}
}
