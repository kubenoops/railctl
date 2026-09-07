package cmd

import (
	"fmt"
	"time"

	"github.com/kubenoops/railctl/internal/api"
)

// Volume-migration polling bounds. Migration time scales with volume size, so
// the default horizon is generous; vars, not consts, so tests can shrink them.
var (
	migrationPoll = 15 * time.Second
	// DefaultMigrationTimeout is the wait before reporting a migration as
	// still-running (never as failed — see awaitVolumeMigration).
	DefaultMigrationTimeout = 20 * time.Minute
)

// awaitVolumeMigration waits for a service's attached volume to actually land
// in the target region after a region change.
//
// Deployment status is the WRONG signal for a migration: Railway takes the
// service down, copies the volume, and its move-deployment routinely ends
// FAILED or REMOVED while the migration itself succeeds — observed live
// 2026-09-07, where a completed migration (volume in us-west2, 835MB intact)
// sat under a FAILED deployment, and --await reported failure for a healthy
// service. The volume's region is the only truthful outcome, so poll that.
//
// Outcomes:
//   - volume reads the target region  → nil (migrated)
//   - volume reverts/stays at source  → error (Railway reset the migration,
//     a real failure an operator must re-issue)
//   - deadline with the volume still elsewhere → nil with an informational
//     message: a long copy is not a failure, and failing CI for a
//     still-running migration would be wrong.
func awaitVolumeMigration(client api.APIClient, projectID, environmentID, serviceID, serviceName, targetRegion string, timeoutSeconds int) error {
	target := api.ShortRegionName(targetRegion)
	fmt.Printf("Awaiting volume migration for '%s' → %s (timeout: %ds)...\n", serviceName, target, timeoutSeconds)

	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	lastRegion := ""
	for {
		region, name, found, err := serviceVolumeRegion(client, projectID, environmentID, serviceID)
		if err != nil {
			return fmt.Errorf("polling volume region: %w", err)
		}
		if !found {
			// No attached volume: nothing migrates, the placement change is
			// a plain config write.
			return nil
		}
		if region != lastRegion {
			fmt.Printf("  Volume '%s' in %s\n", name, region)
			lastRegion = region
		}
		if region == target {
			fmt.Printf("✓ Volume '%s' migrated to %s\n", name, target)
			return nil
		}
		if time.Now().After(deadline) {
			fmt.Printf("Volume '%s' still in %s after %ds — the migration may still be running (larger volumes take longer).\n",
				name, region, timeoutSeconds)
			fmt.Printf("Check with 'railctl get volumes -o wide'; re-run the region change with --force if Railway reset it.\n")
			return nil
		}
		time.Sleep(migrationPoll)
	}
}

// serviceVolumeRegion returns the region and name of the volume attached to
// the service in this environment, and whether one is attached at all.
func serviceVolumeRegion(client api.APIClient, projectID, environmentID, serviceID string) (region, name string, found bool, err error) {
	volumes, err := client.ListVolumes(projectID, environmentID)
	if err != nil {
		return "", "", false, err
	}
	for _, v := range volumes {
		if v.ServiceID != nil && *v.ServiceID == serviceID {
			return api.ShortRegionName(v.Region), v.Volume.Name, true, nil
		}
	}
	return "", "", false, nil
}
