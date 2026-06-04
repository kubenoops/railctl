// Package config provides declarative config loading, validation, and schema
// types for railctl service definitions.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents a full declarative config file.
type Config struct {
	Project     string          `yaml:"project,omitempty"`
	Environment string          `yaml:"environment,omitempty"`
	Services    []ServiceConfig `yaml:"services"`
}

// ServiceConfig describes a single service's desired state.
type ServiceConfig struct {
	Name       string            `yaml:"name"`
	Image      string            `yaml:"image"`
	Deploy     DeployConfig      `yaml:"deploy,omitempty"`
	Networking NetworkingConfig  `yaml:"networking,omitempty"`
	Volume     VolumeConfig      `yaml:"volume,omitempty"`
	Variables  map[string]string `yaml:"variables,omitempty"`
	Registry   RegistryConfig    `yaml:"registry,omitempty"`
}

// DeployConfig holds deployment-related settings for a service.
type DeployConfig struct {
	StartCommand       string `yaml:"startCommand,omitempty"`
	RestartPolicy      string `yaml:"restartPolicy,omitempty"`
	MaxRetries         int    `yaml:"maxRetries,omitempty"`
	Replicas           int    `yaml:"replicas,omitempty"`
	HealthcheckPath    string `yaml:"healthcheckPath,omitempty"`
	HealthcheckTimeout int    `yaml:"healthcheckTimeout,omitempty"`
}

// NetworkingConfig holds networking settings for a service.
type NetworkingConfig struct {
	Domain   DomainConfig   `yaml:"domain,omitempty"`
	TCPProxy TCPProxyConfig `yaml:"tcpProxy,omitempty"`
}

// DomainConfig holds domain-related networking settings.
type DomainConfig struct {
	Port int `yaml:"port,omitempty"`
}

// TCPProxyConfig holds TCP proxy settings.
type TCPProxyConfig struct {
	Port int `yaml:"port,omitempty"`
}

// VolumeConfig holds volume mount settings for a service.
type VolumeConfig struct {
	MountPath string `yaml:"mountPath,omitempty"`
}

// RegistryConfig holds private registry credentials.
type RegistryConfig struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// legacyConfig represents the old single-service config format used for
// backward compatibility detection and conversion.
type legacyConfig struct {
	Service struct {
		Name  string `yaml:"name"`
		Image string `yaml:"image"`
	} `yaml:"service"`
	Deploy struct {
		StartCommand             string `yaml:"startCommand"`
		RestartPolicyType        string `yaml:"restartPolicyType"`
		RestartPolicyMaxRetries  int    `yaml:"restartPolicyMaxRetries"`
		NumReplicas              int    `yaml:"numReplicas"`
		HealthcheckPath          string `yaml:"healthcheckPath"`
		HealthcheckTimeout       int    `yaml:"healthcheckTimeout"`
	} `yaml:"deploy"`
	Domain struct {
		Port int `yaml:"port"`
	} `yaml:"domain"`
	Networking struct {
		TCPProxyPort int `yaml:"tcpProxyPort"`
	} `yaml:"networking"`
	Volume struct {
		MountPath string `yaml:"mountPath"`
	} `yaml:"volume"`
	Variables map[string]string `yaml:"variables"`
}

// convertLegacy converts a legacy single-service config to the new format.
func convertLegacy(data []byte) (*Config, error) {
	var legacy legacyConfig
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("unmarshaling legacy config: %w", err)
	}

	svc := ServiceConfig{
		Name:  legacy.Service.Name,
		Image: legacy.Service.Image,
		Deploy: DeployConfig{
			StartCommand:       legacy.Deploy.StartCommand,
			RestartPolicy:      legacy.Deploy.RestartPolicyType,
			MaxRetries:         legacy.Deploy.RestartPolicyMaxRetries,
			Replicas:           legacy.Deploy.NumReplicas,
			HealthcheckPath:    legacy.Deploy.HealthcheckPath,
			HealthcheckTimeout: legacy.Deploy.HealthcheckTimeout,
		},
		Networking: NetworkingConfig{
			Domain: DomainConfig{Port: legacy.Domain.Port},
			TCPProxy: TCPProxyConfig{Port: legacy.Networking.TCPProxyPort},
		},
		Volume:    VolumeConfig{MountPath: legacy.Volume.MountPath},
		Variables: legacy.Variables,
	}

	return &Config{
		Services: []ServiceConfig{svc},
	}, nil
}

// isLegacyFormat checks whether raw YAML data uses the old single-service
// format by looking for a top-level "service" key with a "name" field and
// no top-level "services" key.
func isLegacyFormat(data []byte) bool {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false
	}

	if _, hasServices := raw["services"]; hasServices {
		return false
	}

	svc, hasService := raw["service"]
	if !hasService {
		return false
	}

	svcMap, ok := svc.(map[string]any)
	if !ok {
		return false
	}

	_, hasName := svcMap["name"]
	return hasName
}

// Load reads a YAML config file from path and returns a validated Config.
// If the file uses the old single-service format, it is automatically
// converted to the new multi-service format.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg *Config

	if isLegacyFormat(data) {
		cfg, err = convertLegacy(data)
		if err != nil {
			return nil, fmt.Errorf("converting legacy config: %w", err)
		}
	} else {
		cfg = &Config{}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("unmarshaling config: %w", err)
		}
	}

	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadDir reads all *.yaml and *.yml files from a directory (sorted
// alphabetically), loads each one, and merges all services into a single
// Config. If multiple files specify project or environment, they must agree
// or an error is returned. The merged result is validated before returning.
func LoadDir(dir string) (*Config, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading config directory: %w", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	if len(files) == 0 {
		return nil, fmt.Errorf("no YAML files found in %s", dir)
	}

	merged := &Config{}

	for _, name := range files {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}

		var fileCfg *Config
		if isLegacyFormat(data) {
			fileCfg, err = convertLegacy(data)
			if err != nil {
				return nil, fmt.Errorf("converting legacy config in %s: %w", name, err)
			}
		} else {
			fileCfg = &Config{}
			if err := yaml.Unmarshal(data, fileCfg); err != nil {
				return nil, fmt.Errorf("unmarshaling %s: %w", name, err)
			}
		}

		// Merge project/environment, requiring agreement.
		if fileCfg.Project != "" {
			if merged.Project != "" && merged.Project != fileCfg.Project {
				return nil, fmt.Errorf("conflicting project: %q (previous) vs %q (in %s)", merged.Project, fileCfg.Project, name)
			}
			merged.Project = fileCfg.Project
		}
		if fileCfg.Environment != "" {
			if merged.Environment != "" && merged.Environment != fileCfg.Environment {
				return nil, fmt.Errorf("conflicting environment: %q (previous) vs %q (in %s)", merged.Environment, fileCfg.Environment, name)
			}
			merged.Environment = fileCfg.Environment
		}

		merged.Services = append(merged.Services, fileCfg.Services...)
	}

	if err := Validate(merged); err != nil {
		return nil, err
	}

	return merged, nil
}

// validRestartPolicies lists the allowed restart policy values.
var validRestartPolicies = map[string]bool{
	"ON_FAILURE": true,
	"ALWAYS":     true,
	"NEVER":      true,
}

// Validate checks a Config for correctness. It collects all validation errors
// and returns them joined with newlines.
func Validate(cfg *Config) error {
	var errs []string

	if len(cfg.Services) == 0 {
		errs = append(errs, "at least one service must be defined")
	}

	seen := make(map[string]bool)
	for i, svc := range cfg.Services {
		prefix := fmt.Sprintf("service[%d]", i)

		if svc.Name == "" {
			errs = append(errs, fmt.Sprintf("%s: name is required", prefix))
		} else {
			if seen[svc.Name] {
				errs = append(errs, fmt.Sprintf("%s: duplicate service name %q", prefix, svc.Name))
			}
			seen[svc.Name] = true
			prefix = fmt.Sprintf("service %q", svc.Name)
		}

		if svc.Image == "" {
			errs = append(errs, fmt.Sprintf("%s: image is required", prefix))
		}

		// Validate registry credentials: both or neither.
		if (svc.Registry.Username != "" && svc.Registry.Password == "") || (svc.Registry.Username == "" && svc.Registry.Password != "") {
			errs = append(errs, fmt.Sprintf("%s: both registry username and password must be set if either is provided", prefix))
		}

		// Normalize and validate restart policy.
		if svc.Deploy.RestartPolicy != "" {
			normalized := strings.ToUpper(svc.Deploy.RestartPolicy)
			if !validRestartPolicies[normalized] {
				errs = append(errs, fmt.Sprintf("%s: invalid restartPolicy %q (must be ON_FAILURE, ALWAYS, or NEVER)", prefix, svc.Deploy.RestartPolicy))
			} else {
				cfg.Services[i].Deploy.RestartPolicy = normalized
			}
		}

		if svc.Deploy.MaxRetries != 0 && svc.Deploy.RestartPolicy == "" {
			errs = append(errs, fmt.Sprintf("%s: maxRetries requires restartPolicy to be set", prefix))
		}

		if svc.Deploy.Replicas != 0 && svc.Deploy.Replicas < 1 {
			errs = append(errs, fmt.Sprintf("%s: replicas must be >= 1, got %d", prefix, svc.Deploy.Replicas))
		}

		if port := svc.Networking.Domain.Port; port != 0 && (port < 1 || port > 65535) {
			errs = append(errs, fmt.Sprintf("%s: domain port must be between 1 and 65535, got %d", prefix, port))
		}

		if port := svc.Networking.TCPProxy.Port; port != 0 && (port < 1 || port > 65535) {
			errs = append(errs, fmt.Sprintf("%s: tcpProxy port must be between 1 and 65535, got %d", prefix, port))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation failed:\n%s", strings.Join(errs, "\n"))
	}

	return nil
}
