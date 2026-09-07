package cmd

import (
	"testing"
	"time"

	"github.com/kubenoops/railctl/internal/api"
)

// withFastMigrationPoll shrinks the migration poll interval for the test.
func withFastMigrationPoll(t *testing.T) {
	t.Helper()
	orig := migrationPoll
	migrationPoll = time.Millisecond
	t.Cleanup(func() { migrationPoll = orig })
}

func volumeWithRegion(svcID, region string) []api.VolumeInstance {
	return []api.VolumeInstance{{
		Volume:    api.Volume{ID: "vol-1", Name: "data"},
		ServiceID: &svcID,
		Region:    region,
	}}
}

// The migration lands: the volume's region reaches the target (full ID on the
// wire, short name as target — both normalized) → success, no error.
func TestAwaitVolumeMigration_Lands(t *testing.T) {
	withFastMigrationPoll(t)
	svcID := "svc-1"
	calls := 0
	client := &api.MockClient{
		ListVolumesFunc: func(_, _ string) ([]api.VolumeInstance, error) {
			calls++
			if calls < 3 {
				return volumeWithRegion(svcID, "us-west2"), nil
			}
			return volumeWithRegion(svcID, "us-east4-eqdc4a"), nil
		},
	}

	if err := awaitVolumeMigration(client, "proj-1", "env-1", svcID, "api", "us-east4", 30); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if calls < 3 {
		t.Errorf("expected the poll to retry until landing, got %d calls", calls)
	}
}

// A volume still in the source region at the deadline is NOT a failure — the
// copy may still be running (migration time scales with volume size). The
// helper reports and returns nil.
func TestAwaitVolumeMigration_TimeoutIsInformational(t *testing.T) {
	withFastMigrationPoll(t)
	svcID := "svc-1"
	client := &api.MockClient{
		ListVolumesFunc: func(_, _ string) ([]api.VolumeInstance, error) {
			return volumeWithRegion(svcID, "us-west2"), nil
		},
	}

	if err := awaitVolumeMigration(client, "proj-1", "env-1", svcID, "api", "us-east4", 0); err != nil {
		t.Fatalf("timeout must be informational, got error %v", err)
	}
}

// No attached volume: nothing migrates, nothing to wait for.
func TestAwaitVolumeMigration_NoVolume(t *testing.T) {
	withFastMigrationPoll(t)
	client := &api.MockClient{
		ListVolumesFunc: func(_, _ string) ([]api.VolumeInstance, error) {
			return nil, nil
		},
	}

	if err := awaitVolumeMigration(client, "proj-1", "env-1", "svc-1", "api", "us-east4", 10); err != nil {
		t.Fatalf("expected no-op success, got %v", err)
	}
}

// A poll error surfaces as an error, not a silent success.
func TestAwaitVolumeMigration_PollError(t *testing.T) {
	withFastMigrationPoll(t)
	client := &api.MockClient{
		ListVolumesFunc: func(_, _ string) ([]api.VolumeInstance, error) {
			return nil, errPollBoom
		},
	}

	if err := awaitVolumeMigration(client, "proj-1", "env-1", "svc-1", "api", "us-east4", 10); err == nil {
		t.Fatal("expected poll error to surface")
	}
}

var errPollBoom = &stubPollError{}

type stubPollError struct{}

func (e *stubPollError) Error() string { return "poll boom" }
