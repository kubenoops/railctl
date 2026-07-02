package diff

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/config"
)

// ChangeType represents what kind of change is needed.
type ChangeType int

const (
	// ChangeNone indicates no change is required.
	ChangeNone ChangeType = iota
	// ChangeCreate indicates a new resource must be created.
	ChangeCreate
	// ChangeUpdate indicates an existing resource must be updated.
	ChangeUpdate
	// ChangeDelete indicates an existing resource must be deleted.
	ChangeDelete
)

// FieldDiff represents a change to a single field.
type FieldDiff struct {
	Path    string // e.g., "image", "deploy.replicas", "variables.PORT"
	Current string // current value (empty if create)
	Desired string // desired value (empty if delete)
}

// ResourceChange represents a change to a service or its sub-resources.
type ResourceChange struct {
	Type        ChangeType
	ServiceName string      // name of the service
	Fields      []FieldDiff // individual field-level diffs
}

// ChangeSet is the full set of changes needed to reconcile desired → live state.
type ChangeSet struct {
	Changes []ResourceChange
}

// HasChanges returns true if there are any create, update, or delete operations.
func (cs *ChangeSet) HasChanges() bool {
	for _, c := range cs.Changes {
		if c.Type != ChangeNone {
			return true
		}
	}
	return false
}

// Summary returns a human-readable summary like "2 to create, 1 to update, 0 to delete".
func (cs *ChangeSet) Summary() string {
	var creates, updates, deletes int
	for _, c := range cs.Changes {
		switch c.Type {
		case ChangeCreate:
			creates++
		case ChangeUpdate:
			updates++
		case ChangeDelete:
			deletes++
		}
	}
	return fmt.Sprintf("%d to create, %d to update, %d to delete", creates, updates, deletes)
}

// LiveService represents the current state of a service in Railway.
// This is populated from the API before diffing.
type LiveService struct {
	Name       string
	Image      string
	Deploy     LiveDeployConfig
	Variables  map[string]string // current variable values
	Volumes    []LiveVolume
	Domains    []LiveDomain
	TCPProxies []LiveTCPProxy
}

// LiveDeployConfig holds the current deploy configuration for a live service.
type LiveDeployConfig struct {
	StartCommand       string
	RestartPolicy      string
	MaxRetries         int
	Replicas           int
	HealthcheckPath    string
	HealthcheckTimeout int
}

// LiveVolume represents a volume attached to a live service.
type LiveVolume struct {
	MountPath string
}

// LiveDomain represents a domain attached to a live service.
type LiveDomain struct {
	Domain string
	Port   int
}

// LiveTCPProxy represents a TCP proxy attached to a live service.
type LiveTCPProxy struct {
	ApplicationPort int
	ProxyPort       int
	Domain          string
}

// Compute compares desired service configs against live Railway state
// and returns the set of changes needed to reconcile.
// If prune is true, services in live that are NOT in desired will be marked for deletion.
func Compute(desired []config.ServiceConfig, live []LiveService, prune bool) *ChangeSet {
	cs := &ChangeSet{}

	// Build a map of live services by name.
	liveMap := make(map[string]LiveService, len(live))
	for _, ls := range live {
		liveMap[ls.Name] = ls
	}

	// Track which live services are accounted for.
	desiredNames := make(map[string]bool, len(desired))

	for _, d := range desired {
		desiredNames[d.Name] = true

		ls, exists := liveMap[d.Name]
		if !exists {
			// Service does not exist in live — create.
			cs.Changes = append(cs.Changes, buildCreateChange(d))
			continue
		}

		// Service exists — compare fields.
		fields := compareService(d, ls)
		if len(fields) > 0 {
			cs.Changes = append(cs.Changes, ResourceChange{
				Type:        ChangeUpdate,
				ServiceName: d.Name,
				Fields:      fields,
			})
		}
	}

	if prune {
		for _, ls := range live {
			if !desiredNames[ls.Name] {
				cs.Changes = append(cs.Changes, buildDeleteChange(ls))
			}
		}
	}

	return cs
}

// buildCreateChange builds a ChangeCreate ResourceChange from a desired service config.
func buildCreateChange(d config.ServiceConfig) ResourceChange {
	var fields []FieldDiff

	if d.Image != "" {
		fields = append(fields, FieldDiff{Path: "image", Desired: d.Image})
	}

	// Deploy fields.
	fields = append(fields, deployCreateFields(d.Deploy)...)

	// Variables.
	fields = append(fields, variableCreateFields(d.Variables)...)

	// Registry credentials.
	fields = append(fields, registryFields(d.Registry)...)

	// Volume.
	if d.Volume.MountPath != "" {
		fields = append(fields, FieldDiff{Path: "volume.mountPath", Desired: d.Volume.MountPath})
	}

	// Networking.
	if d.Networking.Domain.Port != 0 {
		fields = append(fields, FieldDiff{Path: "networking.domain.port", Desired: fmt.Sprintf("%d", d.Networking.Domain.Port)})
	}
	if d.Networking.TCPProxy.Port != 0 {
		fields = append(fields, FieldDiff{Path: "networking.tcpProxy.port", Desired: fmt.Sprintf("%d", d.Networking.TCPProxy.Port)})
	}

	return ResourceChange{
		Type:        ChangeCreate,
		ServiceName: d.Name,
		Fields:      fields,
	}
}

// deployCreateFields returns FieldDiffs for all non-zero deploy fields.
func deployCreateFields(dc config.DeployConfig) []FieldDiff {
	var fields []FieldDiff
	if dc.StartCommand != "" {
		fields = append(fields, FieldDiff{Path: "deploy.startCommand", Desired: dc.StartCommand})
	}
	if dc.RestartPolicy != "" {
		fields = append(fields, FieldDiff{Path: "deploy.restartPolicy", Desired: dc.RestartPolicy})
	}
	if dc.MaxRetries != 0 {
		fields = append(fields, FieldDiff{Path: "deploy.maxRetries", Desired: fmt.Sprintf("%d", dc.MaxRetries)})
	}
	if dc.Replicas != 0 {
		fields = append(fields, FieldDiff{Path: "deploy.replicas", Desired: fmt.Sprintf("%d", dc.Replicas)})
	}
	if dc.HealthcheckPath != "" {
		fields = append(fields, FieldDiff{Path: "deploy.healthcheckPath", Desired: dc.HealthcheckPath})
	}
	if dc.HealthcheckTimeout != 0 {
		fields = append(fields, FieldDiff{Path: "deploy.healthcheckTimeout", Desired: fmt.Sprintf("%d", dc.HealthcheckTimeout)})
	}
	return fields
}

// variableCreateFields returns FieldDiffs for all desired variables, masking sensitive keys.
func variableCreateFields(vars map[string]string) []FieldDiff {
	if len(vars) == 0 {
		return nil
	}
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var fields []FieldDiff
	for _, k := range keys {
		desired := vars[k]
		if api.IsSensitiveKey(k) {
			desired = api.MaskValue(desired)
		}
		fields = append(fields, FieldDiff{Path: "variables." + k, Desired: desired})
	}
	return fields
}

// registryFields returns registry-credential fields (password masked), or nil
// unless both are set (matching registryCreds).
func registryFields(r config.RegistryConfig) []FieldDiff {
	if r.Username == "" || r.Password == "" {
		return nil
	}
	return []FieldDiff{
		{Path: "registry.username", Desired: r.Username},
		{Path: "registry.password", Desired: api.MaskValue(r.Password)},
	}
}

// buildDeleteChange builds a ChangeDelete ResourceChange from a live service.
func buildDeleteChange(ls LiveService) ResourceChange {
	var fields []FieldDiff

	if ls.Image != "" {
		fields = append(fields, FieldDiff{Path: "image", Current: ls.Image})
	}

	// Deploy fields.
	if ls.Deploy.StartCommand != "" {
		fields = append(fields, FieldDiff{Path: "deploy.startCommand", Current: ls.Deploy.StartCommand})
	}
	if ls.Deploy.RestartPolicy != "" {
		fields = append(fields, FieldDiff{Path: "deploy.restartPolicy", Current: ls.Deploy.RestartPolicy})
	}
	if ls.Deploy.MaxRetries != 0 {
		fields = append(fields, FieldDiff{Path: "deploy.maxRetries", Current: fmt.Sprintf("%d", ls.Deploy.MaxRetries)})
	}
	if ls.Deploy.Replicas != 0 {
		fields = append(fields, FieldDiff{Path: "deploy.replicas", Current: fmt.Sprintf("%d", ls.Deploy.Replicas)})
	}
	if ls.Deploy.HealthcheckPath != "" {
		fields = append(fields, FieldDiff{Path: "deploy.healthcheckPath", Current: ls.Deploy.HealthcheckPath})
	}
	if ls.Deploy.HealthcheckTimeout != 0 {
		fields = append(fields, FieldDiff{Path: "deploy.healthcheckTimeout", Current: fmt.Sprintf("%d", ls.Deploy.HealthcheckTimeout)})
	}

	// Variables — sorted, with masking.
	keys := make([]string, 0, len(ls.Variables))
	for k := range ls.Variables {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		current := ls.Variables[k]
		if api.IsSensitiveKey(k) {
			current = api.MaskValue(current)
		}
		fields = append(fields, FieldDiff{Path: "variables." + k, Current: current})
	}

	// Volumes.
	for _, v := range ls.Volumes {
		if v.MountPath != "" {
			fields = append(fields, FieldDiff{Path: "volume.mountPath", Current: v.MountPath})
		}
	}

	// Domains.
	for _, d := range ls.Domains {
		if d.Port != 0 {
			fields = append(fields, FieldDiff{Path: "networking.domain.port", Current: fmt.Sprintf("%d", d.Port)})
		}
	}

	// TCP Proxies.
	for _, tp := range ls.TCPProxies {
		if tp.ApplicationPort != 0 {
			fields = append(fields, FieldDiff{Path: "networking.tcpProxy.port", Current: fmt.Sprintf("%d", tp.ApplicationPort)})
		}
	}

	return ResourceChange{
		Type:        ChangeDelete,
		ServiceName: ls.Name,
		Fields:      fields,
	}
}

// compareService compares a desired config against a live service and returns field diffs.
func compareService(d config.ServiceConfig, ls LiveService) []FieldDiff {
	var fields []FieldDiff

	// Image.
	if d.Image != ls.Image {
		fields = append(fields, FieldDiff{Path: "image", Current: ls.Image, Desired: d.Image})
	}

	// Deploy fields.
	fields = append(fields, compareDeployConfig(d.Deploy, ls.Deploy)...)

	// Variables.
	fields = append(fields, compareVariables(d.Variables, ls.Variables)...)

	// Volume mount path.
	liveMountPath := ""
	if len(ls.Volumes) > 0 {
		liveMountPath = ls.Volumes[0].MountPath
	}
	if d.Volume.MountPath != liveMountPath {
		fields = append(fields, FieldDiff{Path: "volume.mountPath", Current: liveMountPath, Desired: d.Volume.MountPath})
	}

	// Domain port: check if any live domain matches the desired port.
	liveDomainPort := 0
	for _, dom := range ls.Domains {
		if dom.Port == d.Networking.Domain.Port {
			liveDomainPort = dom.Port
			break
		}
	}
	// If no match found, use the first domain's port as the "current" value for the diff.
	if liveDomainPort == 0 && len(ls.Domains) > 0 {
		liveDomainPort = ls.Domains[0].Port
	}
	if d.Networking.Domain.Port != liveDomainPort {
		fields = append(fields, FieldDiff{
			Path:    "networking.domain.port",
			Current: fmt.Sprintf("%d", liveDomainPort),
			Desired: fmt.Sprintf("%d", d.Networking.Domain.Port),
		})
	}

	// TCP proxy port: check if any live proxy matches the desired port.
	liveTCPPort := 0
	for _, tp := range ls.TCPProxies {
		if tp.ApplicationPort == d.Networking.TCPProxy.Port {
			liveTCPPort = tp.ApplicationPort
			break
		}
	}
	// If no match found, use the first proxy's port as the "current" value for the diff.
	if liveTCPPort == 0 && len(ls.TCPProxies) > 0 {
		liveTCPPort = ls.TCPProxies[0].ApplicationPort
	}
	if d.Networking.TCPProxy.Port != liveTCPPort {
		fields = append(fields, FieldDiff{
			Path:    "networking.tcpProxy.port",
			Current: fmt.Sprintf("%d", liveTCPPort),
			Desired: fmt.Sprintf("%d", d.Networking.TCPProxy.Port),
		})
	}

	// Creds can't be diffed (never returned), so re-assert them when the service
	// is already changing.
	if len(fields) > 0 {
		fields = append(fields, registryFields(d.Registry)...)
	}

	return fields
}

// compareDeployConfig compares desired and live deploy configurations.
// compareDeployConfig treats a zero-value field as unmanaged (like the create
// path), so an undeclared field never diffs against or overwrites Railway's defaults.
func compareDeployConfig(d config.DeployConfig, l LiveDeployConfig) []FieldDiff {
	var fields []FieldDiff

	if d.StartCommand != "" && d.StartCommand != l.StartCommand {
		fields = append(fields, FieldDiff{Path: "deploy.startCommand", Current: l.StartCommand, Desired: d.StartCommand})
	}
	if d.RestartPolicy != "" && d.RestartPolicy != l.RestartPolicy {
		fields = append(fields, FieldDiff{Path: "deploy.restartPolicy", Current: l.RestartPolicy, Desired: d.RestartPolicy})
	}
	if d.MaxRetries != 0 && d.MaxRetries != l.MaxRetries {
		fields = append(fields, FieldDiff{
			Path:    "deploy.maxRetries",
			Current: fmt.Sprintf("%d", l.MaxRetries),
			Desired: fmt.Sprintf("%d", d.MaxRetries),
		})
	}
	if d.Replicas != 0 && d.Replicas != l.Replicas {
		fields = append(fields, FieldDiff{
			Path:    "deploy.replicas",
			Current: fmt.Sprintf("%d", l.Replicas),
			Desired: fmt.Sprintf("%d", d.Replicas),
		})
	}
	if d.HealthcheckPath != "" && d.HealthcheckPath != l.HealthcheckPath {
		fields = append(fields, FieldDiff{Path: "deploy.healthcheckPath", Current: l.HealthcheckPath, Desired: d.HealthcheckPath})
	}
	if d.HealthcheckTimeout != 0 && d.HealthcheckTimeout != l.HealthcheckTimeout {
		fields = append(fields, FieldDiff{
			Path:    "deploy.healthcheckTimeout",
			Current: fmt.Sprintf("%d", l.HealthcheckTimeout),
			Desired: fmt.Sprintf("%d", d.HealthcheckTimeout),
		})
	}

	return fields
}

// compareVariables compares desired and live variable maps.
// Variables starting with RAILWAY_ in live are skipped (Railway-injected).
// Sensitive keys have their values masked.
func compareVariables(desired, live map[string]string) []FieldDiff {
	var fields []FieldDiff

	// Sort desired keys for deterministic output.
	desiredKeys := make([]string, 0, len(desired))
	for k := range desired {
		desiredKeys = append(desiredKeys, k)
	}
	sort.Strings(desiredKeys)

	// Check desired variables against live.
	for _, k := range desiredKeys {
		dv := desired[k]
		lv, exists := live[k]

		currentVal := lv
		desiredVal := dv
		if api.IsSensitiveKey(k) {
			currentVal = api.MaskValue(lv)
			desiredVal = api.MaskValue(dv)
		}

		if !exists {
			// New variable.
			fields = append(fields, FieldDiff{Path: "variables." + k, Current: "", Desired: desiredVal})
		} else if dv != lv {
			// Changed variable.
			fields = append(fields, FieldDiff{Path: "variables." + k, Current: currentVal, Desired: desiredVal})
		}
	}

	// Check live variables not in desired (removals).
	liveKeys := make([]string, 0, len(live))
	for k := range live {
		liveKeys = append(liveKeys, k)
	}
	sort.Strings(liveKeys)

	for _, k := range liveKeys {
		// Skip Railway-injected variables.
		if strings.HasPrefix(k, "RAILWAY_") {
			continue
		}
		if _, exists := desired[k]; !exists {
			currentVal := live[k]
			if api.IsSensitiveKey(k) {
				currentVal = api.MaskValue(currentVal)
			}
			fields = append(fields, FieldDiff{Path: "variables." + k, Current: currentVal, Desired: ""})
		}
	}

	return fields
}
