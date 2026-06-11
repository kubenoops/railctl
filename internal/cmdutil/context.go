// Package cmdutil provides shared utilities for CLI command handlers.
//
// It eliminates boilerplate by providing a single call to resolve
// the common project → environment → service chain that most commands need.
package cmdutil

import (
	"fmt"
	"os"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/resolver"
	"github.com/kubenoops/railctl/internal/types"
)

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

	// --- Project resolution ---
	var project types.Project
	if isProjectToken {
		projectID, environmentID, err := client.GetProjectContext()
		if err != nil {
			return nil, fmt.Errorf("failed to get project context from token: %w", err)
		}
		if opts.ProjectName != "" {
			fmt.Fprintf(os.Stderr, "Warning: -p/RAILCTL_PROJECT ignored — project token is already scoped to a specific project\n")
		}
		if opts.NeedEnvironment && opts.EnvironmentName != "" {
			fmt.Fprintf(os.Stderr, "Warning: -e/RAILCTL_ENVIRONMENT ignored — project token is already scoped to a specific environment\n")
		}
		p, err := client.GetProject(projectID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch project from token: %w", err)
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
			return nil, fmt.Errorf("project '%s' not found", opts.ProjectName)
		}
	}
	ctx.Project = project

	// --- Environment resolution (if needed) ---
	if opts.NeedEnvironment {
		if opts.EnvironmentName == "" {
			return nil, fmt.Errorf("-e/--environment is required. Use -e flag or set RAILCTL_ENVIRONMENT")
		}

		environments, err := client.ListEnvironments(project.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list environments: %w", err)
		}

		env, err := resolver.ResolveEnvironment(environments, opts.EnvironmentName)
		if err != nil {
			return nil, fmt.Errorf("environment '%s' not found in project '%s'", opts.EnvironmentName, project.Name)
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
			return nil, fmt.Errorf("service '%s' not found in environment '%s'", opts.ServiceName, ctx.Environment.Name)
		}
		ctx.Service = &svc
	}

	return ctx, nil
}
