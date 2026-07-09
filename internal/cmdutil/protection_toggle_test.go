package cmdutil

import (
	"fmt"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
)

// toggleMock returns a MockClient whose shared variables start as initial and
// records the last SetSharedVariables payload into *captured.
func toggleMock(initial map[string]string, captured *map[string]string) *api.MockClient {
	live := make(map[string]string, len(initial))
	for k, v := range initial {
		live[k] = v
	}
	return &api.MockClient{
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			out := make(map[string]string, len(live))
			for k, v := range live {
				out[k] = v
			}
			return out, nil
		},
		SetSharedVariablesFunc: func(projectID, environmentID string, variables map[string]string) error {
			*captured = variables
			return nil
		},
	}
}

func TestSetDeleteProtection_SetTruePreservesOthers(t *testing.T) {
	var captured map[string]string
	client := toggleMock(map[string]string{"OTHER": "x", "SECOND": "y"}, &captured)

	if err := SetDeleteProtection(client, "proj-1", "env-1", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured[DeleteProtectionVar] != "true" {
		t.Errorf("expected %s=true, got %q", DeleteProtectionVar, captured[DeleteProtectionVar])
	}
	if captured["OTHER"] != "x" || captured["SECOND"] != "y" {
		t.Errorf("expected other shared vars preserved, got %v", captured)
	}
}

func TestSetDeleteProtection_ClearWritesFalse(t *testing.T) {
	var captured map[string]string
	client := toggleMock(map[string]string{"OTHER": "x", DeleteProtectionVar: "true"}, &captured)

	if err := SetDeleteProtection(client, "proj-1", "env-1", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured[DeleteProtectionVar] != "false" {
		t.Errorf("expected %s=false, got %q", DeleteProtectionVar, captured[DeleteProtectionVar])
	}
	// The guard must treat the cleared value as unprotected.
	if isTruthy(captured[DeleteProtectionVar]) {
		t.Errorf("expected cleared value to be falsy, %q is truthy", captured[DeleteProtectionVar])
	}
	if captured["OTHER"] != "x" {
		t.Errorf("expected OTHER preserved, got %v", captured)
	}
}

func TestSetDeleteProtection_ReadErrorFailsClosed(t *testing.T) {
	client := &api.MockClient{
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			return nil, fmt.Errorf("boom")
		},
		SetSharedVariablesFunc: func(projectID, environmentID string, variables map[string]string) error {
			t.Fatal("SetSharedVariables must not be called when the read fails")
			return nil
		},
	}
	if err := SetDeleteProtection(client, "proj-1", "env-1", true); err == nil {
		t.Fatal("expected error when the shared-variable read fails, got nil")
	}
}

func TestEnvironmentIsProtected(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{"truthy_true", "true", true},
		{"truthy_1", "1", true},
		{"truthy_on", "on", true},
		{"falsy_false", "false", false},
		{"falsy_empty", "", false},
		{"falsy_garbage", "garbage", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var captured map[string]string
			client := toggleMock(map[string]string{DeleteProtectionVar: tc.value}, &captured)
			got, err := EnvironmentIsProtected(client, "proj-1", "env-1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("EnvironmentIsProtected(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestEnvironmentIsProtected_AbsentIsUnprotected(t *testing.T) {
	var captured map[string]string
	client := toggleMock(map[string]string{"OTHER": "x"}, &captured)
	got, err := EnvironmentIsProtected(client, "proj-1", "env-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected unprotected when DELETE_PROTECTION is absent")
	}
}
