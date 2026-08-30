package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kubenoops/railctl/internal/api"
)

// resolveRegion validates a requested region name against the available Railway
// regions (case-insensitive on the canonical name) and returns the canonical
// name. An unknown region yields an error listing the valid names. (REQ-CMD-003)
// resolveRegion validates a requested region against the shipped list and returns
// the full region ID used on the wire. The input may be a short name
// (e.g. "asia-southeast1") or a full ID (e.g. "asia-southeast1-eqsg3a"), matched
// case-insensitively. The region list is hardcoded (ListRegions never errors), so
// there is no fallback path — an unknown region is a hard error listing the
// valid short names.
func resolveRegion(client api.APIClient, requested string) (string, error) {
	if strings.TrimSpace(requested) == "" {
		return "", fmt.Errorf("--region requires a region name")
	}
	return api.ResolveRegionID(requested)
}

// writeServiceRegion pins a service to a single region via environmentPatchCommit:
// it sets the target region (with numReplicas) and removes every other currently
// present region (the patch merges, so others must be explicitly nulled). current
// is the live placement map (region → replicas), used to know which to remove.
func writeServiceRegion(client api.APIClient, environmentID, serviceID, region string, numReplicas int, current map[string]int) error {
	if numReplicas < 1 {
		numReplicas = 1
	}
	mrc := map[string]any{
		region: map[string]any{"numReplicas": numReplicas},
	}
	// Null every other known region (shipped set + legacy default + live regions)
	// so the result is exactly one region — Railway otherwise materializes the
	// implicit default alongside the target.
	for _, r := range api.RegionsToClear(region, current) {
		mrc[r] = nil
	}
	return client.CommitMultiRegionConfig(environmentID, serviceID, mrc, fmt.Sprintf("railctl: set region to %s", region))
}

// checkVolumeRegionChange guards a region change when the service has a volume
// attached in the environment. Railway auto-migrates the volume to the new
// region and holds the deployment (downtime, size-dependent) while it runs, so
// the change is refused unless force acknowledges the migration. (REQ-VOL-100 /
// DEC-103)
func checkVolumeRegionChange(client api.APIClient, projectID, environmentID, serviceID, serviceName string, force bool) error {
	volumes, err := client.ListVolumes(projectID, environmentID)
	if err != nil {
		return fmt.Errorf("failed to check for attached volumes: %w", err)
	}

	for _, v := range volumes {
		if v.ServiceID != nil && *v.ServiceID == serviceID {
			if force {
				fmt.Printf("Warning: Railway will migrate volume '%s' (mounted at %s) to the new region; "+
					"service '%s' will be down while the migration runs.\n", v.Volume.Name, v.MountPath, serviceName)
				return nil
			}
			return fmt.Errorf(
				"refusing region change: service %q has volume %q (mounted at %s) attached in this environment.\n"+
					"Railway will migrate the volume to the new region and the service will be DOWN for the\n"+
					"duration of the migration (longer for larger volumes).\n"+
					"Re-run with --force to proceed with the migration.",
				serviceName, v.Volume.Name, v.MountPath)
		}
	}
	return nil
}

// checkRegionCollapse refuses to collapse a service placed in more than one
// region down to a single region unless force is set. (REQ-CMD-009 / DEC-015)
func checkRegionCollapse(live map[string]int, target string, force bool) error {
	if len(live) <= 1 || force {
		return nil
	}
	regions := make([]string, 0, len(live))
	for r := range live {
		regions = append(regions, api.ShortRegionName(r))
	}
	sort.Strings(regions)
	return fmt.Errorf(
		"service is currently placed in %d regions (%s); setting --region %s would drop the others.\n"+
			"Re-run with --force to intentionally collapse the service to a single region.",
		len(live), strings.Join(regions, ", "), api.ShortRegionName(target))
}

// effectiveRegionReplicas resolves the replica count to write for a region
// placement (DEC-016/DEC-025), preserving the service's current scale so a
// region move never silently scales down:
//   - an explicit --replicas wins;
//   - else the target region's live count when the service is already placed there;
//   - else, for a single-region service moving to a new region, that region's count;
//   - else the live flat count when the service is default-placed;
//   - else 1.
//
// For a multi-region service (len(live) > 1) collapsing to a target that is not
// among the live regions, none of the preserve branches match and it defaults to
// 1 (DEC-025). It never returns < 1.
func effectiveRegionReplicas(live map[string]int, flatReplicas int, target string, explicit *int) int {
	switch {
	case explicit != nil:
		if *explicit < 1 {
			return 1
		}
		return *explicit
	case live[target] > 0:
		return live[target]
	case len(live) == 1:
		for _, n := range live {
			if n > 0 {
				return n
			}
		}
		return 1
	case len(live) == 0 && flatReplicas > 0:
		return flatReplicas
	default:
		return 1
	}
}

// isRegionNoop reports whether the live placement already equals exactly the
// target single-region entry with the effective replica count. (DEC-022)
func isRegionNoop(live map[string]int, target string, effReplicas int) bool {
	return len(live) == 1 && live[target] == effReplicas
}
