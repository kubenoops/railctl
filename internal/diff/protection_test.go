package diff

import (
	"bytes"
	"strings"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func TestComputeEnvironment_NilLeavesAlone(t *testing.T) {
	// Manifest omits deleteProtection (nil): never a change, even when live is
	// protected. A dropped line must never silently weaken the safety control.
	if ec := ComputeEnvironment(nil, true); ec != nil {
		t.Errorf("expected nil change when desired is nil and live protected, got %+v", ec)
	}
	if ec := ComputeEnvironment(nil, false); ec != nil {
		t.Errorf("expected nil change when desired is nil and live unprotected, got %+v", ec)
	}
}

func TestComputeEnvironment_TrueVsUnprotected(t *testing.T) {
	ec := ComputeEnvironment(boolPtr(true), false)
	if ec == nil {
		t.Fatal("expected a change: desired true vs live unprotected")
	}
	if !ec.DeleteProtection || ec.CurrentDeleteProtection {
		t.Errorf("expected desired=true current=false, got %+v", ec)
	}
}

func TestComputeEnvironment_TrueVsProtectedNoChange(t *testing.T) {
	if ec := ComputeEnvironment(boolPtr(true), true); ec != nil {
		t.Errorf("expected no change: desired true and already protected, got %+v", ec)
	}
}

func TestComputeEnvironment_FalseVsProtected(t *testing.T) {
	ec := ComputeEnvironment(boolPtr(false), true)
	if ec == nil {
		t.Fatal("expected a change: desired false vs live protected")
	}
	if ec.DeleteProtection || !ec.CurrentDeleteProtection {
		t.Errorf("expected desired=false current=true, got %+v", ec)
	}
}

func TestChangeSet_HasChanges_EnvironmentOnly(t *testing.T) {
	cs := &ChangeSet{Environment: &EnvironmentChange{DeleteProtection: true}}
	if !cs.HasChanges() {
		t.Error("expected HasChanges true when only an environment change exists")
	}
}

func TestRender_EnvironmentDeleteProtection(t *testing.T) {
	cs := &ChangeSet{
		Environment: ComputeEnvironment(boolPtr(true), false),
	}
	var buf bytes.Buffer
	Render(cs, &buf, false)
	out := buf.String()

	if !strings.Contains(out, "Environment (update)") {
		t.Errorf("expected 'Environment (update)' header, got:\n%s", out)
	}
	if !strings.Contains(out, "~ deleteProtection: false → true") {
		t.Errorf("expected 'deleteProtection: false → true', got:\n%s", out)
	}
}

func TestRender_EnvironmentDeleteProtectionOff(t *testing.T) {
	cs := &ChangeSet{
		Environment: ComputeEnvironment(boolPtr(false), true),
	}
	var buf bytes.Buffer
	Render(cs, &buf, false)
	out := buf.String()

	if !strings.Contains(out, "~ deleteProtection: true → false") {
		t.Errorf("expected 'deleteProtection: true → false', got:\n%s", out)
	}
}
