// Package contexts implements the local railctl config file: named Contexts
// (targeting profiles) and Credentials (stored Railway tokens), per
// specs/001-config-contexts. The file is secret material: created 0600 and
// never rendered by callers (FR-020).
package contexts

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Context is a named targeting profile. Project is required on create;
// Workspace, Environment, and Credentials (a Credential name reference) are
// optional (FR-005).
type Context struct {
	Name        string `yaml:"name"`
	Workspace   string `yaml:"workspace,omitempty"`
	Project     string `yaml:"project"`
	Environment string `yaml:"environment,omitempty"`
	Credentials string `yaml:"credentials,omitempty"`
}

// Credential is a named Railway token. TokenType is detected at write time
// and persisted for display; Token is the secret value.
type Credential struct {
	Name      string `yaml:"name"`
	Token     string `yaml:"token"`
	TokenType string `yaml:"tokenType"`
}

// File is the aggregate root persisted at the config path.
type File struct {
	CurrentContext string       `yaml:"currentContext,omitempty"`
	Contexts       []Context    `yaml:"contexts,omitempty"`
	Credentials    []Credential `yaml:"credentials,omitempty"`
}

// ConfigPath returns the config file location: the RAILCTL_CONFIG
// environment variable (single path, no merge lists) or, when unset or
// empty, ~/.railctl/config.yaml (FR-001, ADR-0001).
func ConfigPath() string {
	if p := os.Getenv("RAILCTL_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// No home: the env override is the only usable location.
		return ".railctl/config.yaml"
	}
	return filepath.Join(home, ".railctl", "config.yaml")
}

// Load reads the config file at path. A missing file means "no config" and
// returns (nil, nil) — never an error (FR-004). An existing file with
// permissions looser than 0600 is refused (FR-002, enforced by
// checkPermissions).
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if err := checkPermissions(path); err != nil {
		return nil, err
	}
	f := &File{}
	if err := yaml.Unmarshal(data, f); err != nil {
		return nil, fmt.Errorf("config file %s is not valid YAML: %w", path, err)
	}
	if err := f.validateReferences(); err != nil {
		return nil, fmt.Errorf("config file %s: %w", path, err)
	}
	return f, nil
}

// validateReferences enforces the no-dangling invariant at load: every
// context's credential reference must resolve (FR-021).
func (f *File) validateReferences() error {
	for _, ctx := range f.Contexts {
		if ctx.Credentials == "" {
			continue
		}
		if f.Credential(ctx.Credentials) == nil {
			return fmt.Errorf("context '%s' references credential '%s' which does not exist",
				ctx.Name, ctx.Credentials)
		}
	}
	return nil
}

// Save atomically persists the aggregate at path: it writes a temp file in
// the same directory with 0600 permissions and renames it over the target
// (FR-002 create perms; atomicity per research D5).
func Save(path string, f *File) error {
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	dir := filepath.Dir(path)
	// Bootstrap the config directory on first write (fresh machine:
	// ~/.railctl does not exist yet). 0700 matches the 0600 file posture —
	// no group/other access to secret material (PR #96 review).
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".railctl-config-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after successful rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("failed to set 0600 permissions on temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("failed to replace config file %s: %w", path, err)
	}
	return nil
}

// checkPermissions refuses a config file readable or writable by group or
// others, naming the remediation (FR-002).
func checkPermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return fmt.Errorf("config file %s must have 0600 permissions (currently %o); run: chmod 600 %s",
			path, perm, path)
	}
	return nil
}
