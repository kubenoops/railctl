// Package apply executes a diff.ChangeSet against the Railway API,
// reconciling live state with the desired configuration.
package apply

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/config"
	"github.com/kubenoops/railctl/internal/diff"
	"github.com/kubenoops/railctl/internal/types"
)

// Opts controls apply behavior.
type Opts struct {
	DryRun bool      // if true, only print what would happen
	Prune  bool      // if true, delete unmanaged services (already in ChangeSet)
	Output io.Writer // where to print progress (default: os.Stdout)
}

// Result holds the outcome of an apply operation.
type Result struct {
	Created []string // names of created services
	Updated []string // names of updated services
	Deleted []string // names of deleted services
	Errors  []error  // non-fatal errors encountered
}

// Apply executes a ChangeSet to reconcile Railway state with the desired config.
// It processes changes in order: creates first, then updates, then deletes.
// For each service being created or updated, it also handles variables, volumes,
// domains, and TCP proxies.
//
// The configMap maps service names to their full ServiceConfig (needed for
// variables, volumes, networking that aren't in the ChangeSet fields).
func Apply(client api.APIClient, cs *diff.ChangeSet, projectID, envID string, configMap map[string]config.ServiceConfig, opts Opts) *Result {
	if opts.Output == nil {
		opts.Output = os.Stdout
	}

	result := &Result{}

	// Separate changes by type.
	var creates, updates, deletes []diff.ResourceChange
	for _, rc := range cs.Changes {
		switch rc.Type {
		case diff.ChangeCreate:
			creates = append(creates, rc)
		case diff.ChangeUpdate:
			updates = append(updates, rc)
		case diff.ChangeDelete:
			deletes = append(deletes, rc)
		}
	}

	// --- Dry run ---
	if opts.DryRun {
		for _, rc := range creates {
			fmt.Fprintf(opts.Output, "Would create service '%s'\n", rc.ServiceName)
		}
		for _, rc := range updates {
			desc := summarizeUpdateFields(rc.Fields)
			fmt.Fprintf(opts.Output, "Would update service '%s'%s\n", rc.ServiceName, desc)
		}
		for _, rc := range deletes {
			fmt.Fprintf(opts.Output, "Would delete service '%s'\n", rc.ServiceName)
		}
		return result
	}

	// --- Process creates ---
	for _, rc := range creates {
		if err := applyCreate(client, rc, projectID, envID, configMap, opts.Output); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("create %s: %w", rc.ServiceName, err))
			continue
		}
		result.Created = append(result.Created, rc.ServiceName)
	}

	// --- Process updates ---
	// Cache the service list once for ID lookups.
	var services []types.ServiceDetail
	if len(updates) > 0 || len(deletes) > 0 {
		var err error
		services, err = client.ListServices(projectID, envID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("listing services for update/delete: %w", err))
			return result
		}
	}

	for _, rc := range updates {
		if err := applyUpdate(client, rc, projectID, envID, configMap, services, opts.Output); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("update %s: %w", rc.ServiceName, err))
			continue
		}
		result.Updated = append(result.Updated, rc.ServiceName)
	}

	// --- Process deletes ---
	for _, rc := range deletes {
		if err := applyDelete(client, rc, services, opts.Output); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("delete %s: %w", rc.ServiceName, err))
			continue
		}
		result.Deleted = append(result.Deleted, rc.ServiceName)
	}

	return result
}

// applyCreate handles a single ChangeCreate operation.
func applyCreate(client api.APIClient, rc diff.ResourceChange, projectID, envID string, configMap map[string]config.ServiceConfig, w io.Writer) error {
	name := rc.ServiceName
	cfg := configMap[name]

	fmt.Fprintf(w, "Creating service '%s'...\n", name)

	// Build registry credentials.
	var creds *api.RegistryCredentials
	if cfg.Registry.Username != "" && cfg.Registry.Password != "" {
		creds = &api.RegistryCredentials{
			Username: cfg.Registry.Username,
			Password: cfg.Registry.Password,
		}
	}

	svc, err := client.CreateService(projectID, envID, name, cfg.Image, creds)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	// Clean up instances in other environments (Railway quirk).
	cleanupOtherEnvironments(client, projectID, envID, svc.ID, w)

	// Apply deploy config if any fields are non-zero.
	startCmd, restartPolicy, maxRetries, replicas, hcPath, hcTimeout := buildDeployConfigFromConfig(cfg.Deploy)
	if startCmd != nil || restartPolicy != nil || maxRetries != nil || replicas != nil || hcPath != nil || hcTimeout != nil {
		if err := client.UpdateServiceInstanceConfig(svc.ID, envID, startCmd, restartPolicy, maxRetries, replicas, hcPath, hcTimeout); err != nil {
			return fmt.Errorf("applying deploy config: %w", err)
		}
	}

	// Set variables.
	if len(cfg.Variables) > 0 {
		if err := client.SetVariables(projectID, envID, svc.ID, cfg.Variables, true); err != nil {
			return fmt.Errorf("setting variables: %w", err)
		}
	}

	// Create volume.
	if cfg.Volume.MountPath != "" {
		if _, err := client.CreateVolume(projectID, envID, svc.ID, cfg.Volume.MountPath); err != nil {
			return fmt.Errorf("creating volume: %w", err)
		}
	}

	// Create domain.
	if cfg.Networking.Domain.Port > 0 {
		domain, err := client.CreateServiceDomain(svc.ID, envID)
		if err != nil {
			return fmt.Errorf("creating domain: %w", err)
		}
		if err := client.UpdateServiceDomainPort(domain.ID, cfg.Networking.Domain.Port); err != nil {
			return fmt.Errorf("setting domain port: %w", err)
		}
	}

	// Create TCP proxy.
	if cfg.Networking.TCPProxy.Port > 0 {
		if _, err := client.CreateTCPProxy(cfg.Networking.TCPProxy.Port, envID, svc.ID); err != nil {
			return fmt.Errorf("creating TCP proxy: %w", err)
		}
	}

	fmt.Fprintf(w, "✓ Service '%s' created\n", name)
	return nil
}

// applyUpdate handles a single ChangeUpdate operation.
func applyUpdate(client api.APIClient, rc diff.ResourceChange, projectID, envID string, configMap map[string]config.ServiceConfig, services []types.ServiceDetail, w io.Writer) error {
	name := rc.ServiceName
	cfg := configMap[name]

	serviceID, err := findServiceID(services, name)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Updating service '%s'...\n", name)

	imageChanged, newImage, deployFields, varAdded, varRemoved, volumeChanged, domainChanged, tcpChanged := extractFieldChanges(rc.Fields)

	// Update image.
	if imageChanged {
		var creds *api.RegistryCredentials
		if cfg.Registry.Username != "" && cfg.Registry.Password != "" {
			creds = &api.RegistryCredentials{
				Username: cfg.Registry.Username,
				Password: cfg.Registry.Password,
			}
		}
		if err := client.UpdateServiceInstance(serviceID, envID, newImage, creds); err != nil {
			return fmt.Errorf("updating image: %w", err)
		}
	}

	// Update deploy config.
	if len(deployFields) > 0 {
		startCmd, restartPolicy, maxRetries, replicas, hcPath, hcTimeout := buildDeployConfigUpdate(deployFields)
		if err := client.UpdateServiceInstanceConfig(serviceID, envID, startCmd, restartPolicy, maxRetries, replicas, hcPath, hcTimeout); err != nil {
			return fmt.Errorf("updating deploy config: %w", err)
		}
	}

	// Update variables.
	if len(varAdded) > 0 {
		if err := client.SetVariables(projectID, envID, serviceID, varAdded, true); err != nil {
			return fmt.Errorf("setting variables: %w", err)
		}
	}
	for _, varName := range varRemoved {
		if err := client.DeleteVariable(projectID, envID, serviceID, varName); err != nil {
			return fmt.Errorf("deleting variable %s: %w", varName, err)
		}
	}

	// Volume changes.
	if volumeChanged {
		fmt.Fprintf(w, "  Warning: volume changes detected for '%s' but volumes cannot be updated in place\n", name)
	}

	// Domain changes: check existing domains first (idempotent).
	if domainChanged {
		domains, err := client.ListDomains(projectID, envID, serviceID)
		if err != nil {
			return fmt.Errorf("listing domains: %w", err)
		}

		var domainID string
		if len(domains.ServiceDomains) > 0 {
			domainID = domains.ServiceDomains[0].ID
		} else if len(domains.CustomDomains) > 0 {
			domainID = domains.CustomDomains[0].ID
		}

		if domainID != "" {
			// Domain exists — update port if needed.
			if cfg.Networking.Domain.Port > 0 {
				if err := client.UpdateServiceDomainPort(domainID, cfg.Networking.Domain.Port); err != nil {
					return fmt.Errorf("setting domain port: %w", err)
				}
			}
		} else if cfg.Networking.Domain.Port > 0 {
			// No domain exists — create one.
			domain, err := client.CreateServiceDomain(serviceID, envID)
			if err != nil {
				return fmt.Errorf("creating domain: %w", err)
			}
			if err := client.UpdateServiceDomainPort(domain.ID, cfg.Networking.Domain.Port); err != nil {
				return fmt.Errorf("setting domain port: %w", err)
			}
		}
	}

	// TCP proxy changes: check existing proxies first, delete old one if port changed.
	if tcpChanged && cfg.Networking.TCPProxy.Port > 0 {
		existingProxies, err := client.ListTCPProxies(envID, serviceID)
		if err != nil {
			return fmt.Errorf("listing TCP proxies: %w", err)
		}

		// Delete any existing proxy that doesn't match the desired port.
		for _, tp := range existingProxies {
			if tp.ApplicationPort != cfg.Networking.TCPProxy.Port {
				if err := client.DeleteTCPProxy(tp.ID); err != nil {
					return fmt.Errorf("deleting old TCP proxy (port %d): %w", tp.ApplicationPort, err)
				}
			}
		}

		// Check if desired proxy already exists.
		proxyExists := false
		for _, tp := range existingProxies {
			if tp.ApplicationPort == cfg.Networking.TCPProxy.Port {
				proxyExists = true
				break
			}
		}

		// Create only if it doesn't already exist.
		if !proxyExists {
			if _, err := client.CreateTCPProxy(cfg.Networking.TCPProxy.Port, envID, serviceID); err != nil {
				return fmt.Errorf("creating TCP proxy: %w", err)
			}
		}
	}

	fmt.Fprintf(w, "✓ Service '%s' updated\n", name)
	return nil
}

// applyDelete handles a single ChangeDelete operation.
func applyDelete(client api.APIClient, rc diff.ResourceChange, services []types.ServiceDetail, w io.Writer) error {
	name := rc.ServiceName

	serviceID, err := findServiceID(services, name)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Deleting service '%s'...\n", name)

	if err := client.DeleteService(serviceID); err != nil {
		return fmt.Errorf("deleting service: %w", err)
	}

	fmt.Fprintf(w, "✓ Service '%s' deleted\n", name)
	return nil
}

// cleanupOtherEnvironments removes service instances from non-target environments.
// Railway creates services in all environments by default.
func cleanupOtherEnvironments(client api.APIClient, projectID, targetEnvID, serviceID string, w io.Writer) {
	time.Sleep(500 * time.Millisecond)

	allEnvs, err := client.ListEnvironments(projectID)
	if err != nil {
		fmt.Fprintf(w, "Warning: could not list environments for cleanup: %v\n", err)
		return
	}

	for _, env := range allEnvs {
		if env.ID == targetEnvID {
			continue
		}

		maxRetries := 3
		var lastErr error
		for attempt := 0; attempt < maxRetries; attempt++ {
			if attempt > 0 {
				backoff := time.Duration(1<<uint(attempt-1)) * time.Second
				time.Sleep(backoff)
			}

			lastErr = client.DeleteServiceInstance(serviceID, env.ID)
			if lastErr == nil {
				break
			}
		}

		if lastErr != nil {
			fmt.Fprintf(w, "Warning: could not remove service instance from environment '%s': %v\n", env.Name, lastErr)
		}
	}
}

// findServiceID looks up a service ID by name from a list of services.
func findServiceID(services []types.ServiceDetail, name string) (string, error) {
	for _, svc := range services {
		if svc.Name == name {
			return svc.ID, nil
		}
	}
	return "", fmt.Errorf("service %q not found", name)
}

// buildDeployConfigFromConfig extracts pointer args from a DeployConfig for
// UpdateServiceInstanceConfig. Returns nil pointers for zero-value fields.
func buildDeployConfigFromConfig(dc config.DeployConfig) (startCmd, restartPolicy *string, maxRetries, replicas *int, healthcheckPath *string, healthcheckTimeout *int) {
	if dc.StartCommand != "" {
		startCmd = &dc.StartCommand
	}
	if dc.RestartPolicy != "" {
		restartPolicy = &dc.RestartPolicy
	}
	if dc.MaxRetries != 0 {
		maxRetries = &dc.MaxRetries
	}
	if dc.Replicas != 0 {
		replicas = &dc.Replicas
	}
	if dc.HealthcheckPath != "" {
		healthcheckPath = &dc.HealthcheckPath
	}
	if dc.HealthcheckTimeout != 0 {
		healthcheckTimeout = &dc.HealthcheckTimeout
	}
	return
}

// buildDeployConfigUpdate extracts pointer args from FieldDiffs for
// UpdateServiceInstanceConfig.
func buildDeployConfigUpdate(fields []diff.FieldDiff) (startCmd, restartPolicy *string, maxRetries, replicas *int, healthcheckPath *string, healthcheckTimeout *int) {
	for _, f := range fields {
		switch f.Path {
		case "deploy.startCommand":
			v := f.Desired
			startCmd = &v
		case "deploy.restartPolicy":
			v := f.Desired
			restartPolicy = &v
		case "deploy.maxRetries":
			if n, err := strconv.Atoi(f.Desired); err == nil {
				maxRetries = &n
			}
		case "deploy.replicas":
			if n, err := strconv.Atoi(f.Desired); err == nil {
				replicas = &n
			}
		case "deploy.healthcheckPath":
			v := f.Desired
			healthcheckPath = &v
		case "deploy.healthcheckTimeout":
			if n, err := strconv.Atoi(f.Desired); err == nil {
				healthcheckTimeout = &n
			}
		}
	}
	return
}

// extractFieldChanges categorizes FieldDiffs by field group.
func extractFieldChanges(fields []diff.FieldDiff) (imageChanged bool, newImage string, deployFields []diff.FieldDiff, varAdded map[string]string, varRemoved []string, volumeChanged bool, domainChanged bool, tcpChanged bool) {
	varAdded = make(map[string]string)

	for _, f := range fields {
		switch {
		case f.Path == "image":
			imageChanged = true
			newImage = f.Desired
		case strings.HasPrefix(f.Path, "deploy."):
			deployFields = append(deployFields, f)
		case strings.HasPrefix(f.Path, "variables."):
			varName := strings.TrimPrefix(f.Path, "variables.")
			if f.Desired == "" {
				// Variable removed.
				varRemoved = append(varRemoved, varName)
			} else {
				// Variable added or changed.
				varAdded[varName] = f.Desired
			}
		case f.Path == "volume.mountPath":
			volumeChanged = true
		case f.Path == "networking.domain.port":
			domainChanged = true
		case f.Path == "networking.tcpProxy.port":
			tcpChanged = true
		}
	}
	return
}

// summarizeUpdateFields builds a parenthetical description of what changed
// for dry-run output (e.g., " (image: node:18 -> node:20)").
func summarizeUpdateFields(fields []diff.FieldDiff) string {
	var parts []string
	for _, f := range fields {
		if f.Current != "" && f.Desired != "" {
			parts = append(parts, fmt.Sprintf("%s: %s -> %s", f.Path, f.Current, f.Desired))
		} else if f.Desired != "" {
			parts = append(parts, fmt.Sprintf("%s: (new) %s", f.Path, f.Desired))
		} else {
			parts = append(parts, fmt.Sprintf("%s: %s (removed)", f.Path, f.Current))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}
