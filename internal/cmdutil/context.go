// Package cmdutil provides shared utilities for CLI command handlers.
//
// It eliminates boilerplate by providing a single call to resolve
// the common project → environment → service chain that most commands need.
package cmdutil

import (
	"errors"
	"fmt"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/resolver"
	"github.com/kubenoops/railctl/internal/types"
)

// containedIn annotates a resolver.ErrNotFound with its containing resource
// (e.g. "in project 'my-app'") while preserving the error type — so
// errors.As(err, &resolver.ErrNotFound{}) keeps working and the available
// candidate names collected by the resolver keep rendering. Non-ErrNotFound
// errors (e.g. resolver.ErrAmbiguous) pass through unchanged.
func containedIn(err error, containerResource, containerName string) error {
	var nf resolver.ErrNotFound
	if errors.As(err, &nf) {
		nf.In = fmt.Sprintf("in %s '%s'", containerResource, containerName)
		return nf
	}
	return err
}

// Context holds the resolved IDs and metadata for a command execution.
// It is populated by ResolveContext and provides the resolved project,
// environment, and optionally service for downstream API calls.
type Context struct {
	Client      api.APIClient
	Project     types.Project
	Environment types.Environment
	Service     *types.ServiceDetail // nil if service was not required/resolved
}

// ResolveOpts controls which resources to resolve.
type ResolveOpts struct {
	// ProjectName is the user-provided project name/substring (required for most commands).
	ProjectName string
	// EnvironmentName is the user-provided environment name/substring (optional).
	EnvironmentName string
	// ServiceName is the user-provided service name/substring (optional).
	ServiceName string
	// NeedEnvironment indicates the command requires an environment.
	NeedEnvironment bool
	// NeedService indicates the command requires a service.
	NeedService bool
}

// ResolveContext resolves the project → environment → service chain in one call.
// It validates required fields and returns a populated Context or an error.
func ResolveContext(client api.APIClient, opts ResolveOpts) (*Context, error) {
	ctx := &Context{Client: client}

	isProjectToken, err := client.IsProjectToken()
	if err != nil {
		return nil, err
	}

	// Every command that resolves a context is by definition project-scoped, so
	// this is the one place to nudge broad-token users toward a project token.
	maybeLeastPrivilegeHint(isProjectToken)

	// --- Project resolution ---
	var project types.Project
	// environments caches the environment listing when the project-token
	// contradiction check below already fetched it, so the resolution step
	// further down does not repeat the API call.
	var environments []types.Environment
	if isProjectToken {
		projectID, environmentID, err := client.GetProjectContext()
		if err != nil {
			return nil, fmt.Errorf("failed to get project context from token: %w", err)
		}
		p, err := client.GetProject(projectID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch project from token: %w", err)
		}
		// A -p value naming a DIFFERENT project than the token's baked scope
		// is a contradiction: fail fast instead of silently operating on the
		// token's project. A value matching the token's project (by ID, exact
		// name, or unique substring) is consistent and proceeds silently.
		if opts.ProjectName != "" && opts.ProjectName != p.ID {
			if _, rErr := resolver.ResolveProject([]types.Project{p}, opts.ProjectName); rErr != nil {
				return nil, fmt.Errorf(
					"token is scoped to project '%s' (%s) but -p/--project '%s' was given — refusing to operate on a different project than requested; use a workspace or account token to target other projects",
					p.Name, p.ID, opts.ProjectName)
			}
		}
		// Same contradiction check for -e against the token's baked
		// environment. Project tokens CAN list their project's environments,
		// which yields the baked environment's name for the message.
		// When NeedEnvironment is false there is no environment target to
		// contradict, so a stray -e keeps being ignored as before.
		if opts.NeedEnvironment && opts.EnvironmentName != "" && environmentID != "" {
			environments, err = client.ListEnvironments(projectID)
			if err != nil {
				return nil, fmt.Errorf("failed to list environments: %w", err)
			}
			for _, env := range environments {
				if env.ID != environmentID {
					continue
				}
				// ResolveEnvironment matches by exact name, ID, or unique
				// substring against the single baked environment.
				if _, rErr := resolver.ResolveEnvironment([]types.Environment{env}, opts.EnvironmentName); rErr != nil {
					return nil, fmt.Errorf(
						"token is scoped to environment '%s' (%s) but -e/--environment '%s' was given — refusing to operate on a different environment than requested; use a workspace or account token to target other environments",
						env.Name, env.ID, opts.EnvironmentName)
				}
				break
			}
		}
		project = p
		opts.ProjectName = p.ID
		if opts.NeedEnvironment && environmentID != "" {
			opts.EnvironmentName = environmentID
		}
	} else {
		if opts.ProjectName == "" {
			return nil, fmt.Errorf("-p/--project is required. Use -p flag or set RAILCTL_PROJECT")
		}
		projects, err := client.ListProjects()
		if err != nil {
			return nil, fmt.Errorf("failed to list projects: %w", err)
		}
		project, err = resolver.ResolveProject(projects, opts.ProjectName)
		if err != nil {
			// ErrNotFound already carries the available project names;
			// ErrAmbiguous carries the matches. Both read well as-is.
			return nil, err
		}
	}
	ctx.Project = project

	// --- Environment resolution (if needed) ---
	if opts.NeedEnvironment {
		if opts.EnvironmentName == "" {
			return nil, fmt.Errorf("-e/--environment is required. Use -e flag or set RAILCTL_ENVIRONMENT")
		}

		if environments == nil {
			environments, err = client.ListEnvironments(project.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to list environments: %w", err)
			}
		}

		env, err := resolver.ResolveEnvironment(environments, opts.EnvironmentName)
		if err != nil {
			return nil, containedIn(err, "project", project.Name)
		}
		ctx.Environment = env
	}

	// --- Service resolution (if needed) ---
	if opts.NeedService {
		if opts.ServiceName == "" {
			return nil, fmt.Errorf("-s/--service is required. Use -s flag or set RAILCTL_SERVICE")
		}

		services, err := client.ListServices(project.ID, ctx.Environment.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list services: %w", err)
		}

		svc, err := resolver.ResolveService(services, opts.ServiceName)
		if err != nil {
			return nil, containedIn(err, "environment", ctx.Environment.Name)
		}
		ctx.Service = &svc
	}

	return ctx, nil
}
