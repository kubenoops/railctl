package config

import (
	"fmt"
	"os"
	"strings"
)

// ExpandEnvRefs expands $env(VAR_NAME) patterns in a string by looking up
// os.Getenv(VAR_NAME). Returns an error if the referenced variable is not set.
// Railway service references like ${{service.VAR}} are preserved as-is.
func ExpandEnvRefs(value string) (string, error) {
	var result strings.Builder
	remaining := value

	for {
		idx := strings.Index(remaining, "$env(")
		if idx == -1 {
			result.WriteString(remaining)
			break
		}

		// Write everything before the $env( marker.
		result.WriteString(remaining[:idx])

		// Find the closing paren.
		rest := remaining[idx+len("$env("):]
		end := strings.Index(rest, ")")
		if end == -1 {
			// No closing paren — write the rest literally.
			result.WriteString(remaining[idx:])
			break
		}

		varName := rest[:end]
		envVal, ok := os.LookupEnv(varName)
		if !ok {
			return "", fmt.Errorf("environment variable %q is not set (referenced by $env(%s))", varName, varName)
		}

		result.WriteString(envVal)
		remaining = rest[end+1:]
	}

	return result.String(), nil
}

// ExpandServiceConfigEnvRefs expands $env() references in all string fields
// of a ServiceConfig (image, deploy fields, variable values, registry fields).
// Returns a list of errors for any missing env vars.
func ExpandServiceConfigEnvRefs(svc *ServiceConfig) []error {
	var errs []error

	expand := func(field *string) {
		if *field == "" {
			return
		}
		val, err := ExpandEnvRefs(*field)
		if err != nil {
			errs = append(errs, err)
			return
		}
		*field = val
	}

	expand(&svc.Image)
	expand(&svc.Deploy.StartCommand)
	expand(&svc.Deploy.RestartPolicy)
	expand(&svc.Deploy.HealthcheckPath)
	expand(&svc.Registry.Username)
	expand(&svc.Registry.Password)

	for k, v := range svc.Variables {
		val, err := ExpandEnvRefs(v)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		svc.Variables[k] = val
	}

	return errs
}

// ExpandConfigEnvRefs expands $env() references in all services in a Config.
// Returns an error containing all expansion failures, or nil if all succeeded.
func ExpandConfigEnvRefs(cfg *Config) error {
	var allErrs []error

	for i := range cfg.Services {
		errs := ExpandServiceConfigEnvRefs(&cfg.Services[i])
		allErrs = append(allErrs, errs...)
	}

	if len(allErrs) > 0 {
		msgs := make([]string, len(allErrs))
		for i, e := range allErrs {
			msgs[i] = e.Error()
		}
		return fmt.Errorf("env expansion errors:\n%s", strings.Join(msgs, "\n"))
	}

	return nil
}
