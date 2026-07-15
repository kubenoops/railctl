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
	"github.com/kubenoops/railctl/internal/cmdutil"
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

	// Environment-level change (deleteProtection). Applied before per-service
	// work so protection is asserted early. A nil cs.Environment means the
	// manifest omitted deleteProtection (or live already matches) — leave it
	// alone.
	if cs.Environment != nil {
		if opts.DryRun {
			fmt.Fprintf(opts.Output, "Would set deleteProtection to %t\n", cs.Environment.DeleteProtection)
		} else {
			if err := setEnvironmentDeleteProtection(client, projectID, envID, cs.Environment.DeleteProtection); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("setting deleteProtection: %w", err))
			} else if cs.Environment.DeleteProtection {
				fmt.Fprintf(opts.Output, "✓ Environment delete-protected\n")
			} else {
				fmt.Fprintf(opts.Output, "✓ Environment delete protection removed\n")
			}
		}
	}

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

	creds := registryCreds(cfg.Registry)
	svc, err := client.CreateService(projectID, envID, name, cfg.Image, creds)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

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
		vol, err := client.CreateVolume(projectID, envID, svc.ID, cfg.Volume.MountPath)
		if err != nil {
			return fmt.Errorf("creating volume: %w", err)
		}
		// Backup schedules on the new volume instance.
		if len(cfg.Volume.BackupSchedules) > 0 {
			instanceID, err := findVolumeInstanceIDByVolume(client, projectID, envID, vol.ID)
			if err != nil {
				return fmt.Errorf("resolving volume instance for backup schedules: %w", err)
			}
			if err := client.SetVolumeBackupSchedules(instanceID, cfg.Volume.BackupSchedules); err != nil {
				return fmt.Errorf("setting backup schedules: %w", err)
			}
		}
	}

	// Create domain with its port in one call.
	if cfg.Networking.Domain.Port > 0 {
		if _, err := client.CreateServiceDomain(svc.ID, envID, cfg.Networking.Domain.Port); err != nil {
			return fmt.Errorf("creating domain: %w", err)
		}
	}

	// Create TCP proxy.
	if cfg.Networking.TCPProxy.Port > 0 {
		if _, err := client.CreateTCPProxy(cfg.Networking.TCPProxy.Port, envID, svc.ID); err != nil {
			return fmt.Errorf("creating TCP proxy: %w", err)
		}
	}

	// Custom domains (none exist yet on a new service).
	if err := reconcileCustomDomains(client, projectID, envID, svc.ID, cfg.Networking, nil, w); err != nil {
		return err
	}

	// Roll out the staged config explicitly — the same thing applyUpdate does.
	//
	// Do NOT rely on serviceCreate deploying implicitly: that is unreliable (a
	// multi-service apply routinely left most services with NO deployment at
	// all), and even when it does fire it races the config we stage above
	// (start command, variables, volume, networking), so the implicit rollout
	// can reflect the pre-config service. A service that exists with zero
	// deployments is a systemic failure, not an unhealthy deploy — the
	// deployment must exist even if it later crashes.
	if _, err := client.DeployServiceInstance(svc.ID, envID); err != nil {
		return fmt.Errorf("triggering initial deployment: %w", err)
	}

	fmt.Fprintf(w, "✓ Service '%s' created\n", name)
	return nil
}

// reconcileCustomDomains creates absent declared domains (printing DNS) and
// updates the port of existing ones. Port defaults to domain.port.
func reconcileCustomDomains(client api.APIClient, projectID, envID, serviceID string, net config.NetworkingConfig, live []api.CustomDomain, w io.Writer) error {
	byName := make(map[string]api.CustomDomain, len(live))
	for _, cd := range live {
		byName[cd.Domain] = cd
	}
	for _, cd := range net.CustomDomains {
		port := cd.Port
		if port == 0 {
			port = net.Domain.Port
		}
		existing, ok := byName[cd.Name]
		if !ok {
			created, err := client.CreateCustomDomain(projectID, envID, serviceID, cd.Name, port)
			if err != nil {
				return fmt.Errorf("creating custom domain %q: %w", cd.Name, err)
			}
			PrintCustomDomainDNS(created, w)
			byName[cd.Name] = created // avoid re-creating on duplicate declaration
			continue
		}
		if port > 0 && (existing.TargetPort == nil || *existing.TargetPort != port) {
			if err := client.UpdateCustomDomainPort(existing.ID, envID, port); err != nil {
				return fmt.Errorf("setting custom domain %q port: %w", cd.Name, err)
			}
		}
	}
	return nil
}

// PrintCustomDomainDNS prints the DNS records to configure: the routing record(s)
// (CNAME/A) from dnsRecords, plus the verification TXT, which Railway exposes
// separately as verificationDnsHost/verificationToken rather than in dnsRecords.
// Exported so `railctl create domain` renders the exact same output as apply.
func PrintCustomDomainDNS(cd api.CustomDomain, w io.Writer) {
	fmt.Fprintf(w, "  Custom domain '%s' created — add the following DNS record(s):\n", cd.Domain)
	if cd.Status == nil {
		fmt.Fprintf(w, "    (no DNS records returned; check the Railway dashboard)\n")
		return
	}
	for _, r := range cd.Status.DNSRecords {
		// Railway returns verbose enums (DNS_RECORD_TYPE_CNAME, DNS_RECORD_PURPOSE_TRAFFIC_ROUTE).
		recordType := strings.TrimPrefix(r.RecordType, "DNS_RECORD_TYPE_")
		line := fmt.Sprintf("    %-6s %s  →  %s", recordType, r.Hostlabel, r.RequiredValue)
		if p := strings.ToLower(strings.TrimPrefix(r.Purpose, "DNS_RECORD_PURPOSE_")); p != "" {
			line += fmt.Sprintf("  (%s)", p)
		}
		fmt.Fprintln(w, line)
	}
	if !cd.Status.Verified && cd.Status.VerificationToken != "" {
		fmt.Fprintf(w, "    %-6s %s  →  %s  (verification)\n", "TXT", cd.Status.VerificationDNSHost, cd.Status.VerificationToken)
	}
	if len(cd.Status.DNSRecords) == 0 && cd.Status.VerificationToken == "" {
		fmt.Fprintf(w, "    (no DNS records returned; check the Railway dashboard)\n")
	}
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

	imageChanged, newImage, deployFields, varAdded, varRemoved, volumeChanged, backupSchedulesChanged, domainChanged, tcpChanged := extractFieldChanges(rc.Fields)

	// Only staged changes need a deploy. Networking applies immediately and the
	// volume branch stages nothing, so neither is included.
	needsDeploy := imageChanged || len(deployFields) > 0 || len(varAdded) > 0 || len(varRemoved) > 0

	// Creds are write-only, so re-assert them — but only when a deploy will roll
	// them out (they're used at image-pull time), else they'd strand as pending.
	creds := registryCreds(cfg.Registry)
	if imageChanged || (creds != nil && needsDeploy) {
		image := ""
		if imageChanged {
			image = newImage
		}
		if err := client.UpdateServiceInstance(serviceID, envID, image, creds); err != nil {
			return fmt.Errorf("updating image/registry credentials: %w", err)
		}
	}

	// Update deploy config.
	if len(deployFields) > 0 {
		startCmd, restartPolicy, maxRetries, replicas, hcPath, hcTimeout := buildDeployConfigUpdate(deployFields)
		if err := client.UpdateServiceInstanceConfig(serviceID, envID, startCmd, restartPolicy, maxRetries, replicas, hcPath, hcTimeout); err != nil {
			return fmt.Errorf("updating deploy config: %w", err)
		}
	}

	// Read values from config, not the diff fields: those mask secrets, and
	// writing the mask would clobber the real value.
	if len(varAdded) > 0 {
		realVars := make(map[string]string, len(varAdded))
		for k := range varAdded {
			realVars[k] = cfg.Variables[k]
		}
		if err := client.SetVariables(projectID, envID, serviceID, realVars, true); err != nil {
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

	// Backup schedules apply immediately (no deploy needed). If the volume
	// doesn't exist yet (in-place volume creation isn't supported — see above),
	// warn instead of failing the whole update.
	if backupSchedulesChanged {
		instanceID, found, err := findServiceVolumeInstanceID(client, projectID, envID, serviceID)
		if err != nil {
			return fmt.Errorf("resolving volume instance for backup schedules: %w", err)
		}
		if !found {
			fmt.Fprintf(w, "  Warning: backup schedules not applied for '%s' (no volume yet): re-run 'apply' after the volume is created to set them\n", name)
		} else {
			if err := client.SetVolumeBackupSchedules(instanceID, cfg.Volume.BackupSchedules); err != nil {
				return fmt.Errorf("setting backup schedules: %w", err)
			}
			if len(cfg.Volume.BackupSchedules) == 0 {
				// Clearing schedules is destructive — warn and name what was removed.
				if prev := fieldCurrent(rc.Fields, "volume.backupSchedules"); prev != "" {
					fmt.Fprintf(w, "  Warning: backup schedules for '%s' cleared (were: %s)\n", name, strings.ReplaceAll(prev, ",", ", "))
				} else {
					fmt.Fprintf(w, "  Warning: backup schedules for '%s' cleared\n", name)
				}
			} else {
				fmt.Fprintf(w, "  Backup schedules set to [%s]\n", strings.Join(cfg.Volume.BackupSchedules, ", "))
			}
		}
	}

	// Domain changes: reconcile the target port (idempotent).
	if domainChanged {
		domains, err := client.ListDomains(projectID, envID, serviceID)
		if err != nil {
			return fmt.Errorf("listing domains: %w", err)
		}

		port := cfg.Networking.Domain.Port
		switch {
		case port == 0:
			// Removal: the manifest omitted networking.domain, so close the
			// service domain(s). Custom domains are user-owned and never
			// removed on absence (reconcileCustomDomains only adds/updates).
			for _, sd := range domains.ServiceDomains {
				if err := client.DeleteServiceDomain(sd.ID); err != nil {
					return fmt.Errorf("removing service domain %q: %w", sd.Domain, err)
				}
				fmt.Fprintf(w, "  ✓ removed domain %s\n", sd.Domain)
			}
		case len(domains.ServiceDomains) > 0:
			sd := domains.ServiceDomains[0]
			if sd.TargetPort == nil || *sd.TargetPort != port {
				if err := client.UpdateServiceDomainPort(sd.ID, sd.Domain, envID, serviceID, port); err != nil {
					return fmt.Errorf("setting domain port: %w", err)
				}
			}
		case len(domains.CustomDomains) > 0:
			// No service domain — fall back to an existing custom domain.
			cd := domains.CustomDomains[0]
			if cd.TargetPort == nil || *cd.TargetPort != port {
				if err := client.UpdateCustomDomainPort(cd.ID, envID, port); err != nil {
					return fmt.Errorf("setting custom domain port: %w", err)
				}
			}
		default:
			if _, err := client.CreateServiceDomain(serviceID, envID, port); err != nil {
				return fmt.Errorf("creating domain: %w", err)
			}
		}
	}

	// Only when the diff touched a custom domain, to skip an extra ListDomains call.
	customDomainChanged := false
	for _, f := range rc.Fields {
		if strings.HasPrefix(f.Path, "customDomain.") {
			customDomainChanged = true
			break
		}
	}
	if customDomainChanged && len(cfg.Networking.CustomDomains) > 0 {
		domains, err := client.ListDomains(projectID, envID, serviceID)
		if err != nil {
			return fmt.Errorf("listing domains: %w", err)
		}
		if err := reconcileCustomDomains(client, projectID, envID, serviceID, cfg.Networking, domains.CustomDomains, w); err != nil {
			return err
		}
	}

	// TCP proxy changes: reconcile to exactly the declared port. Desired port 0
	// (omitted block) removes any live proxy — this is how a service is
	// un-exposed declaratively (closing a public port).
	if tcpChanged {
		existingProxies, err := client.ListTCPProxies(envID, serviceID)
		if err != nil {
			return fmt.Errorf("listing TCP proxies: %w", err)
		}

		// Delete any existing proxy that doesn't match the desired port
		// (all of them when the desired port is 0 — a removal).
		for _, tp := range existingProxies {
			if tp.ApplicationPort != cfg.Networking.TCPProxy.Port {
				if err := client.DeleteTCPProxy(tp.ID); err != nil {
					return fmt.Errorf("deleting old TCP proxy (port %d): %w", tp.ApplicationPort, err)
				}
				if cfg.Networking.TCPProxy.Port == 0 {
					fmt.Fprintf(w, "  ✓ removed TCP proxy (port %d)\n", tp.ApplicationPort)
				}
			}
		}
		// Create the desired proxy if it doesn't already exist (skipped on
		// removal, where the desired port is 0 and all proxies were deleted).
		if cfg.Networking.TCPProxy.Port > 0 {
			proxyExists := false
			for _, tp := range existingProxies {
				if tp.ApplicationPort == cfg.Networking.TCPProxy.Port {
					proxyExists = true
					break
				}
			}
			if !proxyExists {
				if _, err := client.CreateTCPProxy(cfg.Networking.TCPProxy.Port, envID, serviceID); err != nil {
					return fmt.Errorf("creating TCP proxy: %w", err)
				}
			}
		}
	}

	// Roll out the staged changes (applyCreate rolls out its own explicitly).
	if needsDeploy {
		if _, err := client.DeployServiceInstance(serviceID, envID); err != nil {
			return fmt.Errorf("triggering deployment: %w", err)
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

// NOTE: apply previously ran a post-create cleanup loop (see the matching note
// in internal/cmd/create_service.go) deleting service instances from all
// non-target environments — a workaround for Railway's fork-era behavior of
// creating instances everywhere. Re-verified 2026-07-08: serviceCreate targets
// a single environment, so the workaround was removed.

// findServiceID looks up a service ID by name from a list of services.
func findServiceID(services []types.ServiceDetail, name string) (string, error) {
	for _, svc := range services {
		if svc.Name == name {
			return svc.ID, nil
		}
	}
	return "", fmt.Errorf("service %q not found", name)
}

// findServiceVolumeInstanceID returns the instance ID of the volume attached to
// the given service and whether one was found. A nil error with found=false
// means no volume is attached yet (distinct from an API failure).
func findServiceVolumeInstanceID(client api.APIClient, projectID, envID, serviceID string) (id string, found bool, err error) {
	volumes, err := client.ListVolumes(projectID, envID)
	if err != nil {
		return "", false, fmt.Errorf("listing volumes: %w", err)
	}
	for _, vi := range volumes {
		if vi.ServiceID != nil && *vi.ServiceID == serviceID {
			return vi.ID, true, nil
		}
	}
	return "", false, nil
}

// findVolumeInstanceIDByVolume returns the instance ID for a volume ID,
// retrying to absorb propagation lag right after a volume is created.
func findVolumeInstanceIDByVolume(client api.APIClient, projectID, envID, volumeID string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		volumes, err := client.ListVolumes(projectID, envID)
		if err != nil {
			lastErr = fmt.Errorf("listing volumes: %w", err)
			continue
		}
		for _, vi := range volumes {
			if vi.Volume.ID == volumeID {
				return vi.ID, nil
			}
		}
		lastErr = fmt.Errorf("volume instance for volume %q not found", volumeID)
	}
	return "", lastErr
}

// setEnvironmentDeleteProtection sets or clears the DELETE_PROTECTION shared
// variable on the environment. It delegates to cmdutil.SetDeleteProtection,
// whose write is clobber-safe (read-merge-write, preserving every other shared
// variable). Clearing writes "false" (Railway has no serviceless
// delete-shared-variable path; the guard treats "false" as unprotected).
func setEnvironmentDeleteProtection(client api.APIClient, projectID, envID string, protect bool) error {
	return cmdutil.SetDeleteProtection(client, projectID, envID, protect)
}

// registryCreds returns the configured private-registry credentials, or nil
// when either field is unset.
func registryCreds(r config.RegistryConfig) *api.RegistryCredentials {
	if r.Username == "" || r.Password == "" {
		return nil
	}
	return &api.RegistryCredentials{Username: r.Username, Password: r.Password}
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

// fieldCurrent returns the Current value of the FieldDiff with the given path,
// or "" when no such field is present.
func fieldCurrent(fields []diff.FieldDiff, path string) string {
	for _, f := range fields {
		if f.Path == path {
			return f.Current
		}
	}
	return ""
}

// extractFieldChanges categorizes FieldDiffs by field group.
func extractFieldChanges(fields []diff.FieldDiff) (imageChanged bool, newImage string, deployFields []diff.FieldDiff, varAdded map[string]string, varRemoved []string, volumeChanged bool, backupSchedulesChanged bool, domainChanged bool, tcpChanged bool) {
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
		case f.Path == "volume.backupSchedules":
			backupSchedulesChanged = true
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
