package api

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kubenoops/railctl/internal/types"
)

// hardcodedRegions is the canonical Railway region set. The live `regions` query
// is not authorized for project tokens (verified 2026-08-29), so railctl ships
// the list — there is no live-query fallback. Name is the short, user-facing form
// (datacenter suffix stripped); ID is the full region ID used on the wire (the
// multiRegionConfig key). Source: docs.railway.com/reference/regions (all four
// validated live). Update this list when Railway adds regions.
var hardcodedRegions = []types.Region{
	{Name: "us-west2", ID: "us-west2", Country: "US", Location: "California, USA"},
	{Name: "us-east4", ID: "us-east4-eqdc4a", Country: "US", Location: "Virginia, USA"},
	{Name: "europe-west4", ID: "europe-west4-drams3a", Country: "NL", Location: "Amsterdam, Netherlands"},
	{Name: "asia-southeast1", ID: "asia-southeast1-eqsg3a", Country: "SG", Location: "Singapore"},
}

// ListRegions returns the shipped Railway region list. It never errors and makes
// no API call (the live `regions` query is not authorized for project tokens).
func (c *Client) ListRegions() ([]types.Region, error) {
	out := make([]types.Region, len(hardcodedRegions))
	copy(out, hardcodedRegions)
	return out, nil
}

// ResolveRegionID resolves a short region name or full region ID (matched
// case-insensitively against the shipped list) to the full region ID used on
// the wire. Both the imperative --region flag and the declarative
// deploy.region resolve through here: committing a short name verbatim places
// the service on Railway's LEGACY (non-metal) region of that name, which
// breaks volume migrations ("Reset region due to volume migration failure" —
// volumes cannot migrate between metal and non-metal regions). Unknown values
// are a hard error listing the valid short names.
func ResolveRegionID(requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return "", fmt.Errorf("a region name is required")
	}
	names := make([]string, 0, len(hardcodedRegions))
	for _, r := range hardcodedRegions {
		names = append(names, r.Name)
		if strings.EqualFold(r.Name, requested) || strings.EqualFold(r.ID, requested) {
			return r.ID, nil
		}
	}
	sort.Strings(names)
	return "", fmt.Errorf("unknown region %q. Available regions: %s (see 'railctl get regions')",
		requested, strings.Join(names, ", "))
}

// ShortRegionName maps a full region ID to its short, user-facing name, or
// returns the input unchanged if it isn't a known region ID.
func ShortRegionName(id string) string {
	for _, r := range hardcodedRegions {
		if r.ID == id {
			return r.Name
		}
	}
	return id
}

// legacyDefaultRegionID is Railway's implicit default placement region for new
// services. It is not in the metal region list, but committing a single region
// makes Railway materialize this default alongside it — so it must be nulled to
// achieve true single-region placement.
const legacyDefaultRegionID = "iad"

// RegionsToClear returns the region IDs to remove (set null) for single-region
// placement of target: the shipped region IDs, the legacy default, and any
// currently-placed regions — minus the target. Nulling a region that isn't
// present is a harmless no-op, so this reliably collapses to one region without
// needing to know the service's (implicit) default up front.
func RegionsToClear(target string, current map[string]int) []string {
	set := map[string]bool{legacyDefaultRegionID: true}
	for _, r := range hardcodedRegions {
		set[r.ID] = true
	}
	for r := range current {
		set[r] = true
	}
	delete(set, target)
	out := make([]string, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	return out
}

// environmentPatchCommitMutation stages and commits an environment config patch.
// Region placement is written this way (not via serviceInstanceUpdate): the patch
// is an EnvironmentConfig setting services.<id>.deploy.multiRegionConfig.
const environmentPatchCommitMutation = `
mutation($environmentId: String!, $patch: EnvironmentConfig!, $commitMessage: String) {
	environmentPatchCommit(environmentId: $environmentId, patch: $patch, commitMessage: $commitMessage)
}
`

// CommitMultiRegionConfig sets a service's region placement by committing an
// environment patch. mrc maps region ID → value, where a value of {"numReplicas": N}
// places/scales the service in that region and a nil value removes it. Single-region
// placement passes the target region plus nil for every other currently-present region.
func (c *Client) CommitMultiRegionConfig(environmentID, serviceID string, mrc map[string]any, commitMessage string) error {
	patch := map[string]any{
		"services": map[string]any{
			serviceID: map[string]any{
				"deploy": map[string]any{
					"multiRegionConfig": mrc,
				},
			},
		},
	}
	_, err := c.execute(environmentPatchCommitMutation, map[string]any{
		"environmentId": environmentID,
		"patch":         patch,
		"commitMessage": commitMessage,
	})
	return err
}
