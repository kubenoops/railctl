package cmdutil

import (
	"fmt"
	"strings"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// DeleteProtectionVar is the name of the environment-level (shared,
// serviceless) variable that marks an environment as delete-protected.
const DeleteProtectionVar = "DELETE_PROTECTION"

// isTruthy reports whether a DELETE_PROTECTION value enables protection.
// The truthy set is exactly: "true", "1", "yes", "on" — case-insensitive,
// after trimming surrounding whitespace. Everything else (unset, empty,
// "false", "0", "no", "off", or any other value) counts as unprotected.
func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

// CheckDeleteProtection returns an error if the environment carries a truthy
// DELETE_PROTECTION shared variable. Truthy means one of "true", "1", "yes",
// "on" (case-insensitive, trimmed); any other value — or the variable being
// absent — leaves the environment unprotected.
//
// There is deliberately no bypass flag: the only way to delete a protected
// environment is to unset (or falsify) the shared variable first. If the
// shared variables cannot be read, the check fails closed — protection cannot
// be verified, so deletion is refused.
func CheckDeleteProtection(client api.APIClient, projectID string, env types.Environment) error {
	value, protected, err := environmentDeleteProtection(client, projectID, env)
	if err != nil {
		return err
	}
	if protected {
		return fmt.Errorf("environment '%s' is delete-protected (%s=%s) — unset the shared variable %s on that environment to allow deletion",
			env.Name, DeleteProtectionVar, value, DeleteProtectionVar)
	}
	return nil
}

// CheckProjectDeleteProtection returns an error if any of the project's
// environments carries a truthy DELETE_PROTECTION shared variable (see
// CheckDeleteProtection for the truthy set). All protected environments are
// collected so the error names every offender at once. If any environment's
// shared variables cannot be read, the check fails closed.
func CheckProjectDeleteProtection(client api.APIClient, project types.Project, envs []types.Environment) error {
	var protectedNames []string
	for _, env := range envs {
		_, protected, err := environmentDeleteProtection(client, project.ID, env)
		if err != nil {
			return err
		}
		if protected {
			protectedNames = append(protectedNames, env.Name)
		}
	}
	if len(protectedNames) > 0 {
		return fmt.Errorf("project '%s' contains delete-protected environment(s): %s — unset the shared variable %s on them to allow deletion",
			project.Name, strings.Join(protectedNames, ", "), DeleteProtectionVar)
	}
	return nil
}

// environmentDeleteProtection reads the environment's shared variables and
// reports the raw DELETE_PROTECTION value and whether it is truthy. A read
// failure is returned as a wrapped error so callers fail closed.
func environmentDeleteProtection(client api.APIClient, projectID string, env types.Environment) (value string, protected bool, err error) {
	vars, err := client.GetSharedVariables(projectID, env.ID)
	if err != nil {
		return "", false, fmt.Errorf("failed to verify delete protection for environment '%s': %w", env.Name, err)
	}
	value = vars[DeleteProtectionVar]
	return value, isTruthy(value), nil
}
