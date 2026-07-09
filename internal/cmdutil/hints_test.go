package cmdutil

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// withHintCapture swaps hintWriter for a buffer and restores it after the test.
func withHintCapture(t *testing.T) *bytes.Buffer {
	t.Helper()
	orig := hintWriter
	buf := &bytes.Buffer{}
	hintWriter = buf
	t.Cleanup(func() { hintWriter = orig })
	return buf
}

func TestMaybeLeastPrivilegeHint(t *testing.T) {
	tests := []struct {
		name           string
		isProjectToken bool
		outputIsText   bool
		noHints        bool
		wantHint       bool
	}{
		{"broad token + text mode -> hint", false, true, false, true},
		{"project token -> never", true, true, false, false},
		{"machine output -> suppressed", false, false, false, false},
		{"RAILCTL_NO_HINTS -> suppressed", false, true, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := withHintCapture(t)
			origText := OutputIsText
			OutputIsText = tt.outputIsText
			t.Cleanup(func() { OutputIsText = origText })
			if tt.noHints {
				t.Setenv(NoHintsEnv, "1")
			} else {
				t.Setenv(NoHintsEnv, "")
			}

			maybeLeastPrivilegeHint(tt.isProjectToken)

			got := buf.Len() > 0
			if got != tt.wantHint {
				t.Errorf("hint emitted = %v, want %v (output: %q)", got, tt.wantHint, buf.String())
			}
			if tt.wantHint && !strings.Contains(buf.String(), "project token") {
				t.Errorf("hint should mention a project token, got: %q", buf.String())
			}
		})
	}
}

func protectionMock(vars map[string]string, readErr error) *api.MockClient {
	return &api.MockClient{
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			if readErr != nil {
				return nil, readErr
			}
			return vars, nil
		},
	}
}

func TestRequireDeletable(t *testing.T) {
	env := types.Environment{ID: "env-1", Name: "production"}

	t.Run("protected blocks with a resource-named message", func(t *testing.T) {
		client := protectionMock(map[string]string{DeleteProtectionVar: "true"}, nil)
		err := RequireDeletable(client, "proj-1", env, "volume", "data")
		if err == nil {
			t.Fatal("expected an error for a protected environment")
		}
		for _, want := range []string{"cannot delete volume 'data'", "production", "delete-protected", "unprotect environment production"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error missing %q: %v", want, err)
			}
		}
	})

	t.Run("unprotected proceeds", func(t *testing.T) {
		client := protectionMock(map[string]string{DeleteProtectionVar: "false"}, nil)
		if err := RequireDeletable(client, "proj-1", env, "service", "api"); err != nil {
			t.Errorf("unexpected error on unprotected env: %v", err)
		}
	})

	t.Run("absent variable proceeds", func(t *testing.T) {
		client := protectionMock(map[string]string{}, nil)
		if err := RequireDeletable(client, "proj-1", env, "backup", "b-1"); err != nil {
			t.Errorf("unexpected error when protection is absent: %v", err)
		}
	})

	t.Run("read failure fails closed", func(t *testing.T) {
		client := protectionMock(nil, errors.New("Not Authorized"))
		if err := RequireDeletable(client, "proj-1", env, "volume", "data"); err == nil {
			t.Error("expected a fail-closed error when shared variables cannot be read")
		}
	})
}
