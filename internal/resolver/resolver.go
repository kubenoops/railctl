// Package resolver provides name-to-resource resolution for Railway resources.
package resolver

import (
	"fmt"
	"strings"

	"github.com/kubenoops/railctl/internal/types"
)

// ErrNotFound indicates a resource was not found.
type ErrNotFound struct {
	Resource string
	Name     string
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("%s '%s' not found", e.Resource, e.Name)
}

// ErrAmbiguous indicates multiple resources matched the name.
type ErrAmbiguous struct {
	Resource string
	Name     string
	Matches  []string
}

func (e ErrAmbiguous) Error() string {
	return fmt.Sprintf("ambiguous %s name '%s'. Matches:\n  - %s",
		e.Resource, e.Name, strings.Join(e.Matches, "\n  - "))
}

// Resource is a generic resource with ID and Name for resolution.
type Resource struct {
	ID   string
	Name string
}

// ResolveProject finds a project by name using exact match first, then substring.
// Returns an error if not found or if the name is ambiguous.
func ResolveProject(projects []types.Project, name string) (types.Project, error) {
	// Exact match first
	for _, p := range projects {
		if p.Name == name {
			return p, nil
		}
	}

	// Substring match (case-insensitive)
	nameLower := strings.ToLower(name)
	var matches []types.Project
	for _, p := range projects {
		if strings.Contains(strings.ToLower(p.Name), nameLower) {
			matches = append(matches, p)
		}
	}

	switch len(matches) {
	case 0:
		return types.Project{}, ErrNotFound{Resource: "project", Name: name}
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Name
		}
		return types.Project{}, ErrAmbiguous{Resource: "project", Name: name, Matches: names}
	}
}

// ResolveEnvironment resolves an environment name or ID to an Environment.
// Priority: exact name match > ID match > single substring match > error.
func ResolveEnvironment(environments []types.Environment, name string) (types.Environment, error) {
	// Try exact name match first
	for _, env := range environments {
		if env.Name == name {
			return env, nil
		}
	}

	// Try ID match (project tokens pass environment IDs directly)
	for _, env := range environments {
		if env.ID == name {
			return env, nil
		}
	}

	// Try case-insensitive substring match
	nameLower := strings.ToLower(name)
	var matches []types.Environment
	for _, env := range environments {
		if strings.Contains(strings.ToLower(env.Name), nameLower) {
			matches = append(matches, env)
		}
	}

	switch len(matches) {
	case 0:
		return types.Environment{}, ErrNotFound{Resource: "environment", Name: name}
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, env := range matches {
			names[i] = env.Name
		}
		return types.Environment{}, ErrAmbiguous{Resource: "environment", Name: name, Matches: names}
	}
}

// ResolveService resolves a service name to a ServiceDetail.
// Priority: exact match > single substring match > error.
func ResolveService(services []types.ServiceDetail, name string) (types.ServiceDetail, error) {
	// Try exact match first
	for _, svc := range services {
		if svc.Name == name {
			return svc, nil
		}
	}

	// Try case-insensitive substring match
	nameLower := strings.ToLower(name)
	var matches []types.ServiceDetail
	for _, svc := range services {
		if strings.Contains(strings.ToLower(svc.Name), nameLower) {
			matches = append(matches, svc)
		}
	}

	switch len(matches) {
	case 0:
		return types.ServiceDetail{}, ErrNotFound{Resource: "service", Name: name}
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, svc := range matches {
			names[i] = svc.Name
		}
		return types.ServiceDetail{}, ErrAmbiguous{Resource: "service", Name: name, Matches: names}
	}
}

// Resolve finds a resource by name using exact match first, then substring.
// Returns the ID of the matched resource or an error.
func Resolve(name string, resources []Resource) (string, error) {
	id, _, err := ResolveWithName(name, resources)
	return id, err
}

// ResolveWithName finds a resource by name and returns both ID and Name.
// Priority: exact match > single substring match > error.
func ResolveWithName(name string, resources []Resource) (id string, resolvedName string, err error) {
	// Exact match first
	for _, r := range resources {
		if r.Name == name {
			return r.ID, r.Name, nil
		}
	}

	// Substring match (case-insensitive)
	nameLower := strings.ToLower(name)
	var matches []Resource
	for _, r := range resources {
		if strings.Contains(strings.ToLower(r.Name), nameLower) {
			matches = append(matches, r)
		}
	}

	switch len(matches) {
	case 0:
		return "", "", ErrNotFound{Resource: "resource", Name: name}
	case 1:
		return matches[0].ID, matches[0].Name, nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Name
		}
		return "", "", ErrAmbiguous{Resource: "resource", Name: name, Matches: names}
	}
}
