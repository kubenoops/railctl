package cmd

import (
	"fmt"
	"time"

	"github.com/kubenoops/railctl/internal/api"
)

// Terminal deployment statuses — no more changes expected.
var terminalStatuses = map[string]bool{
	"SUCCESS": true,
	"FAILED":  true,
	"CRASHED": true,
	"REMOVED": true,
	"SKIPPED": true,
}

// awaitDeployment polls the Railway API until the given deployment reaches
// a terminal status (SUCCESS, FAILED, CRASHED, etc.).
// It prints status transitions as they occur and respects the given timeout.
//
// A REMOVED deployment is not treated as a failure when a NEWER deployment
// exists for the service: Railway removes in-flight deployments when a newer
// rollout supersedes them (observed live 2026-09-03, railctl's deploy trigger
// racing Railway's own reconciliation), so await re-targets the replacement
// and keeps waiting. Only a removal with nothing newer to follow is an error.
func awaitDeployment(client api.APIClient, projectID, environmentID, serviceID, deploymentID, serviceName string, timeoutSeconds int) error {
	initialInterval := 5 * time.Second
	maxInterval := 30 * time.Second
	backoffFactor := 2.0
	pollInterval := initialInterval
	timeout := time.Duration(timeoutSeconds) * time.Second
	deadline := time.Now().Add(timeout)
	lastStatus := ""

	fmt.Printf("Awaiting deployment %s for '%s' (timeout: %ds)...\n", shortID(deploymentID), serviceName, timeoutSeconds)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for deployment %s after %ds", shortID(deploymentID), timeoutSeconds)
		}

		deployments, err := client.ListDeployments(projectID, environmentID, serviceID, 5)
		if err != nil {
			return fmt.Errorf("failed to poll deployment status: %w", err)
		}

		// Find our deployment
		var found bool
		var trackedCreatedAt time.Time
		for _, d := range deployments {
			if d.ID == deploymentID {
				found = true
				trackedCreatedAt = d.CreatedAt
				if d.Status != lastStatus {
					if lastStatus != "" {
						fmt.Printf("  %s → %s\n", lastStatus, d.Status)
					} else {
						fmt.Printf("  Status: %s\n", d.Status)
					}
					lastStatus = d.Status
				}

				if terminalStatuses[d.Status] {
					switch d.Status {
					case "SUCCESS":
						fmt.Printf("✓ Deployment %s succeeded for '%s'\n", shortID(deploymentID), serviceName)
						return nil
					case "FAILED", "CRASHED":
						// Try to fetch build logs for error details
						errMsg := fetchDeploymentError(client, deploymentID)
						if errMsg != "" {
							return fmt.Errorf("deployment %s %s for '%s': %s", shortID(deploymentID), d.Status, serviceName, errMsg)
						}
						return fmt.Errorf("deployment %s %s for '%s'", shortID(deploymentID), d.Status, serviceName)
					case "REMOVED":
						// Superseded? Follow the replacement instead of failing.
						if replacement, ok := newestAfter(deployments, trackedCreatedAt); ok {
							fmt.Printf("  %s superseded — following %s\n", shortID(deploymentID), shortID(replacement.ID))
							deploymentID = replacement.ID
							lastStatus = ""
							pollInterval = initialInterval
							break
						}
						return fmt.Errorf("deployment %s ended with status %s for '%s'", shortID(deploymentID), d.Status, serviceName)
					default:
						return fmt.Errorf("deployment %s ended with status %s for '%s'", shortID(deploymentID), d.Status, serviceName)
					}
				}
				break
			}
		}

		if !found {
			return fmt.Errorf("deployment %s not found in recent deployments for '%s'", shortID(deploymentID), serviceName)
		}

		time.Sleep(pollInterval)

		// Exponential backoff: increase interval up to maxInterval
		pollInterval = time.Duration(float64(pollInterval) * backoffFactor)
		if pollInterval > maxInterval {
			pollInterval = maxInterval
		}
	}
}

// newestAfter returns the newest non-REMOVED deployment created strictly
// after t. Only a strictly newer deployment can have superseded the tracked
// one — an older one (e.g. a previous SUCCESS still in the recent window)
// must never be re-targeted, or await would report success on a stale roll.
func newestAfter(deployments []api.Deployment, t time.Time) (api.Deployment, bool) {
	var best api.Deployment
	var found bool
	for _, d := range deployments {
		if !d.CreatedAt.After(t) || d.Status == "REMOVED" {
			continue
		}
		if !found || d.CreatedAt.After(best.CreatedAt) {
			best = d
			found = true
		}
	}
	return best, found
}

// shortID returns the first 8 characters of an ID, or the full ID if shorter.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// fetchDeploymentError tries to get a meaningful error message from build logs.
func fetchDeploymentError(client api.APIClient, deploymentID string) string {
	logs, err := client.GetBuildLogs(deploymentID, 20)
	if err != nil || len(logs) == 0 {
		return ""
	}
	return api.ExtractErrorFromLogs(logs)
}
