package cmd

import (
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// backupTestClient returns a MockClient wired with a single volume "data"
// (instance vi-1) plus the given overrides applied by the caller.
func backupTestMock() *api.MockClient {
	svcID := "svc-1"
	return &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		ListVolumesFunc: func(projectID, environmentID string) ([]api.VolumeInstance, error) {
			return []api.VolumeInstance{
				{ID: "vi-1", Volume: api.Volume{ID: "vol-1", Name: "data"}, ServiceID: &svcID, MountPath: "/data"},
			}, nil
		},
	}
}

func TestRunCreateBackup_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origName := createBackupName
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		createBackupName = origName
	}()

	var capturedInstanceID, capturedName string
	mock := backupTestMock()
	mock.CreateVolumeBackupFunc = func(volumeInstanceID, name string) (string, error) {
		capturedInstanceID = volumeInstanceID
		capturedName = name
		return "wf-1", nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	createBackupName = "pre-migration"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	if err := createBackupCmd.RunE(createBackupCmd, []string{"data"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedInstanceID != "vi-1" {
		t.Errorf("expected instance vi-1, got %q", capturedInstanceID)
	}
	if capturedName != "pre-migration" {
		t.Errorf("expected name pre-migration, got %q", capturedName)
	}
}

func TestRunCreateBackup_VolumeNotFound(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	token = "test-token"
	project = "my-project"
	environment = "production"
	newAPIClient = func(tkn string) api.APIClient { return backupTestMock() }

	if err := createBackupCmd.RunE(createBackupCmd, []string{"nonexistent"}); err == nil {
		t.Error("expected error for unknown volume")
	}
}

func TestRunGetBackups_List(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origSchedules := getBackupsSchedules
	origOutput := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		getBackupsSchedules = origSchedules
		outputFormat = origOutput
	}()

	called := false
	mock := backupTestMock()
	mock.ListVolumeBackupsFunc = func(volumeInstanceID string) ([]api.VolumeBackup, error) {
		called = true
		if volumeInstanceID != "vi-1" {
			t.Errorf("expected instance vi-1, got %q", volumeInstanceID)
		}
		return []api.VolumeBackup{{ID: "b-1", Name: "backup-1", CreatedAt: "2026-07-01T00:00:00Z", ScheduleID: "s1"}}, nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	getBackupsSchedules = false
	outputFormat = "json"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	if err := getBackupsCmd.RunE(getBackupsCmd, []string{"data"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected ListVolumeBackups to be called")
	}
}

func TestRunGetBackups_Schedules(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origSchedules := getBackupsSchedules
	origOutput := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		getBackupsSchedules = origSchedules
		outputFormat = origOutput
	}()

	called := false
	mock := backupTestMock()
	mock.ListVolumeBackupSchedulesFunc = func(volumeInstanceID string) ([]api.BackupSchedule, error) {
		called = true
		return []api.BackupSchedule{{ID: "s1", Kind: "DAILY", Cron: "0 0 * * *", RetentionSeconds: 518400}}, nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	getBackupsSchedules = true
	outputFormat = "json"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	if err := getBackupsCmd.RunE(getBackupsCmd, []string{"data"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected ListVolumeBackupSchedules to be called")
	}
}

func TestRunRestoreBackup_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origVolume := restoreBackupVolume
	origYes := restoreBackupYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		restoreBackupVolume = origVolume
		restoreBackupYes = origYes
	}()

	var capturedBackupID, capturedInstanceID string
	mock := backupTestMock()
	mock.RestoreVolumeBackupFunc = func(backupID, volumeInstanceID string) error {
		capturedBackupID = backupID
		capturedInstanceID = volumeInstanceID
		return nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	restoreBackupVolume = "data"
	restoreBackupYes = true
	newAPIClient = func(tkn string) api.APIClient { return mock }

	if err := restoreBackupCmd.RunE(restoreBackupCmd, []string{"b-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedBackupID != "b-1" || capturedInstanceID != "vi-1" {
		t.Errorf("unexpected restore args: backup=%q instance=%q", capturedBackupID, capturedInstanceID)
	}
}

func TestFormatBackupTime(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "-"},
		{"valid RFC3339", "2026-07-01T13:45:00Z", "2026-07-01 13:45"},
		{"invalid falls back to raw", "not-a-time", "not-a-time"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatBackupTime(tt.in); got != tt.want {
				t.Errorf("formatBackupTime(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatRetention(t *testing.T) {
	tests := []struct {
		name    string
		seconds int
		want    string
	}{
		{"zero", 0, "-"},
		{"negative", -100, "-"},
		{"under a day", 3600, "3600s"},
		{"exactly one day", 86400, "1d"},
		{"six days", 518400, "6d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatRetention(tt.seconds); got != tt.want {
				t.Errorf("formatRetention(%d) = %q, want %q", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestRunDeleteBackup_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origVolume := deleteBackupVolume
	origYes := deleteBackupYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		deleteBackupVolume = origVolume
		deleteBackupYes = origYes
	}()

	var capturedBackupID, capturedInstanceID string
	mock := backupTestMock()
	mock.DeleteVolumeBackupFunc = func(backupID, volumeInstanceID string) error {
		capturedBackupID = backupID
		capturedInstanceID = volumeInstanceID
		return nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	deleteBackupVolume = "data"
	deleteBackupYes = true
	newAPIClient = func(tkn string) api.APIClient { return mock }

	if err := deleteBackupCmd.RunE(deleteBackupCmd, []string{"b-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedBackupID != "b-1" || capturedInstanceID != "vi-1" {
		t.Errorf("unexpected delete args: backup=%q instance=%q", capturedBackupID, capturedInstanceID)
	}
}

func TestRunDeleteBackup_Cancelled(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origVolume := deleteBackupVolume
	origYes := deleteBackupYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		deleteBackupVolume = origVolume
		deleteBackupYes = origYes
		deleteBackupCmd.SetIn(nil)
	}()

	deleteCalled := false
	mock := backupTestMock()
	mock.DeleteVolumeBackupFunc = func(backupID, volumeInstanceID string) error {
		deleteCalled = true
		return nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	deleteBackupVolume = "data"
	deleteBackupYes = false
	deleteBackupCmd.SetIn(strings.NewReader("n\n"))
	newAPIClient = func(tkn string) api.APIClient { return mock }

	if err := deleteBackupCmd.RunE(deleteBackupCmd, []string{"b-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteCalled {
		t.Error("expected delete to be cancelled, but DeleteVolumeBackup was called")
	}
}

func TestRunDeleteBackup_VolumeNotFound(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origVolume := deleteBackupVolume
	origYes := deleteBackupYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		deleteBackupVolume = origVolume
		deleteBackupYes = origYes
	}()

	token = "test-token"
	project = "my-project"
	environment = "production"
	deleteBackupVolume = "nonexistent"
	deleteBackupYes = true
	newAPIClient = func(tkn string) api.APIClient { return backupTestMock() }

	if err := deleteBackupCmd.RunE(deleteBackupCmd, []string{"b-1"}); err == nil {
		t.Error("expected error for unknown volume")
	}
}
