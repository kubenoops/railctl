package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// deleteFileTestConfig declares db (with a volume) first, then api — so the
// deletion order must be api, db (reverse manifest order), volume last.
const deleteFileTestConfig = `
services:
  - name: db
    image: postgres:16
    volume:
      mountPath: /data
  - name: api
    image: nginx:latest
`

// saveDeleteFileGlobals saves/restores the delete -f flag globals on top of
// the shared apply globals (client factory, -p/-e, token).
func saveDeleteFileGlobals(t *testing.T) {
	t.Helper()
	cleanup := saveApplyGlobals()
	origFile := deleteFile
	origYes := deleteYes
	t.Cleanup(func() {
		cleanup.restore()
		deleteFile = origFile
		deleteYes = origYes
		deleteCmd.SetIn(nil)
		deleteCmd.SetOut(nil)
		deleteCmd.SetErr(nil)
	})
}

// deleteFileMock wires a MockClient with two live services (db with an
// attached volume, api) and records every delete call in *ops.
func deleteFileMock(ops *[]string) *api.MockClient {
	dbID := "svc-db"
	return &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "test-project"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "test-env"}}, nil
		},
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{
				{ID: dbID, Name: "db", Source: "postgres:16"},
				{ID: "svc-api", Name: "api", Source: "nginx:latest"},
			}, nil
		},
		ListVolumesFunc: func(projectID, environmentID string) ([]api.VolumeInstance, error) {
			return []api.VolumeInstance{
				{ID: "vi-1", Volume: api.Volume{ID: "vol-1", Name: "db-data"}, MountPath: "/data", ServiceID: &dbID},
			}, nil
		},
		DeleteServiceFunc: func(id string) error {
			*ops = append(*ops, "service:"+id)
			return nil
		},
		DeleteVolumeFunc: func(volumeID string) error {
			*ops = append(*ops, "volume:"+volumeID)
			return nil
		},
	}
}

func TestDeleteFile_ReverseOrderAndVolumes(t *testing.T) {
	saveDeleteFileGlobals(t)

	var ops []string
	mock := deleteFileMock(&ops)
	token = "test-token"
	project = "test-project"
	environment = "test-env"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	deleteFile = writeTestConfig(t, deleteFileTestConfig)
	deleteYes = true

	var buf bytes.Buffer
	deleteCmd.SetOut(&buf)

	if err := runDeleteFile(deleteCmd, []string{}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Reverse manifest order: api first, db second, declared volume after.
	want := []string{"service:svc-api", "service:svc-db", "volume:vol-1"}
	if len(ops) != len(want) {
		t.Fatalf("expected ops %v, got %v", want, ops)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Errorf("op[%d] = %q, want %q (full: %v)", i, ops[i], want[i], ops)
		}
	}

	out := buf.String()
	if !strings.Contains(out, "2 services deleted, 1 volumes deleted, 0 skipped (not found)") {
		t.Errorf("expected summary line, got:\n%s", out)
	}
}

func TestDeleteFile_AbsentServiceSkipped(t *testing.T) {
	saveDeleteFileGlobals(t)

	var ops []string
	mock := deleteFileMock(&ops)
	// Only api exists live; declared db (and its volume) are absent.
	mock.ListServicesFunc = func(projectID, envID string) ([]types.ServiceDetail, error) {
		return []types.ServiceDetail{{ID: "svc-api", Name: "api", Source: "nginx:latest"}}, nil
	}
	mock.ListVolumesFunc = func(projectID, environmentID string) ([]api.VolumeInstance, error) {
		return nil, nil
	}
	token = "test-token"
	project = "test-project"
	environment = "test-env"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	deleteFile = writeTestConfig(t, deleteFileTestConfig)
	deleteYes = true

	var buf bytes.Buffer
	deleteCmd.SetOut(&buf)

	if err := runDeleteFile(deleteCmd, []string{}); err != nil {
		t.Fatalf("expected no error when a declared service is absent, got: %v", err)
	}

	if len(ops) != 1 || ops[0] != "service:svc-api" {
		t.Errorf("expected only api to be deleted, got ops: %v", ops)
	}

	out := buf.String()
	if !strings.Contains(out, "Service 'db' not found — skipping.") {
		t.Errorf("expected skip message for db, got:\n%s", out)
	}
	if !strings.Contains(out, "1 services deleted, 0 volumes deleted, 1 skipped (not found)") {
		t.Errorf("expected summary line, got:\n%s", out)
	}
}

func TestDeleteFile_ConfirmationDeclined(t *testing.T) {
	saveDeleteFileGlobals(t)

	var ops []string
	mock := deleteFileMock(&ops)
	token = "test-token"
	project = "test-project"
	environment = "test-env"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	deleteFile = writeTestConfig(t, deleteFileTestConfig)
	deleteYes = false

	var buf bytes.Buffer
	deleteCmd.SetOut(&buf)
	deleteCmd.SetIn(strings.NewReader("n\n"))

	if err := runDeleteFile(deleteCmd, []string{}); err != nil {
		t.Fatalf("expected no error on declined confirmation, got: %v", err)
	}

	if len(ops) != 0 {
		t.Errorf("expected nothing deleted after 'n', got ops: %v", ops)
	}

	out := buf.String()
	if !strings.Contains(out, "[y/N]") {
		t.Errorf("expected confirmation prompt, got:\n%s", out)
	}
	if !strings.Contains(out, "Deletion cancelled.") {
		t.Errorf("expected cancellation message, got:\n%s", out)
	}
}

func TestDeleteFile_ProjectTokenFlagFree(t *testing.T) {
	saveDeleteFileGlobals(t)

	var ops []string
	mock := deleteFileMock(&ops)
	// Project-token shape: scope baked into the token, no -p/-e flags.
	mock.IsProjectTokenFunc = func() (bool, error) { return true, nil }
	mock.GetProjectContextFunc = func() (string, string, error) { return "proj-1", "env-1", nil }
	mock.GetProjectFunc = func(id string) (types.Project, error) {
		return types.Project{ID: id, Name: "test-project"}, nil
	}
	mock.ListProjectsFunc = nil // project tokens cannot enumerate projects
	token = "test-token"
	project = ""
	environment = ""
	newAPIClient = func(tkn string) api.APIClient { return mock }

	deleteFile = writeTestConfig(t, deleteFileTestConfig)
	deleteYes = true

	var buf bytes.Buffer
	deleteCmd.SetOut(&buf)

	if err := runDeleteFile(deleteCmd, []string{}); err != nil {
		t.Fatalf("expected flag-free delete -f to work under a project token, got: %v", err)
	}

	want := []string{"service:svc-api", "service:svc-db", "volume:vol-1"}
	if len(ops) != len(want) {
		t.Fatalf("expected ops %v, got %v", want, ops)
	}
}

// TestDeleteCmd_ParentBehavior covers the coexistence of the group RunE with
// its subcommands: bare `railctl delete` (no -f, no args) shows help without
// erroring, and subcommand names still dispatch to the subcommands.
func TestDeleteCmd_ParentBehavior(t *testing.T) {
	saveDeleteFileGlobals(t)

	deleteFile = ""

	var buf bytes.Buffer
	deleteCmd.SetOut(&buf)

	if err := deleteCmd.RunE(deleteCmd, []string{}); err != nil {
		t.Fatalf("expected bare 'delete' to show help without error, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Available resources") && !strings.Contains(out, "Usage:") {
		t.Errorf("expected help output, got:\n%s", out)
	}

	// Subcommand dispatch still wins when a subcommand name is given.
	c, _, err := rootCmd.Find([]string{"delete", "service", "api"})
	if err != nil {
		t.Fatalf("Find(delete service): %v", err)
	}
	if c.Name() != "service" {
		t.Errorf("expected 'delete service' to dispatch to the service subcommand, got %q", c.Name())
	}

	// A stray non-subcommand argument is rejected, not silently swallowed.
	if err := deleteCmd.RunE(deleteCmd, []string{"servce"}); err == nil {
		t.Error("expected an unknown-command error for a typo'd subcommand")
	}
}
