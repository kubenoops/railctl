package cmd

import (
	"fmt"
	"strings"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/spf13/cobra"
)

var (
	skipDeploymentFlag bool
)

var setVariableCmd = &cobra.Command{
	Use:     "variable KEY=VALUE [KEY2=VALUE2 ...]",
	Aliases: []string{"var", "v"},
	Short:   "Set one or more environment variables for a service",
	Long: `Set or update environment variables for a service in an environment.

You can set multiple variables in a single command by providing multiple KEY=VALUE pairs.
By default, setting variables triggers a new deployment. Use --skip-deployment to prevent this.

SERVICE-TO-SERVICE REFERENCES:
  Reference variables from other services using Railway's templating syntax:
    ${{service-name.VARIABLE_NAME}}
  
  This creates a service connection and resolves the variable at runtime.
  Use single quotes to prevent shell interpretation of special characters.

Required flags can be provided via environment variables:
  --project, -p     or RAILCTL_PROJECT
  --environment, -e or RAILCTL_ENVIRONMENT
  --service, -s     or RAILCTL_SERVICE`,
	Example: `  # Set a single variable
  railctl set variable DATABASE_URL=postgres://... -p myproject -e production -s web

  # Set multiple variables
  railctl set variable API_KEY=abc123 DEBUG=true PORT=3000 -p myproject -e production -s web

  # Set variable without triggering deployment
  railctl set variable FEATURE_FLAG=enabled --skip-deployment -p myproject -e production -s web

  # Reference another service's variable (use single quotes)
  railctl set variable 'DB_URL=${{postgres.DATABASE_URL}}' -p myproject -e production -s api

  # Reference shared variables
  railctl set variable 'API_KEY=${{shared.STRIPE_KEY}}' -p myproject -e production -s web

  # Using environment variables for context
  export RAILCTL_PROJECT=myproject
  export RAILCTL_ENVIRONMENT=production
  export RAILCTL_SERVICE=web
  railctl set variable NODE_ENV=production`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSetVariable,
}

func init() {
	setCmd.AddCommand(setVariableCmd)

	setVariableCmd.Flags().BoolVar(&skipDeploymentFlag, "skip-deployment", false, "Skip triggering a deployment after setting variables")
}

func runSetVariable(cmd *cobra.Command, args []string) error {
	// Parse KEY=VALUE pairs
	variables := make(map[string]string)
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid variable format '%s': expected KEY=VALUE", arg)
		}
		key := strings.TrimSpace(parts[0])
		value := parts[1] // Don't trim value - whitespace might be intentional

		if key == "" {
			return fmt.Errorf("invalid variable format '%s': key cannot be empty", arg)
		}

		variables[key] = value
	}

	token, err := getToken()
	if err != nil {
		return err
	}

	client := newAPIClient(token)

	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		ServiceName:     getService(),
		NeedEnvironment: true,
		NeedService:     true,
	})
	if err != nil {
		return err
	}

	// Set variables
	err = client.SetVariables(ctx.Project.ID, ctx.Environment.ID, ctx.Service.ID, variables, skipDeploymentFlag)
	if err != nil {
		return fmt.Errorf("failed to set variables: %w", err)
	}

	// Success message
	keys := make([]string, 0, len(variables))
	for k := range variables {
		keys = append(keys, k)
	}

	if len(keys) == 1 {
		fmt.Printf("Variable '%s' set successfully for service '%s' in environment '%s'\n",
			keys[0], ctx.Service.Name, ctx.Environment.Name)
	} else {
		fmt.Printf("%d variables set successfully for service '%s' in environment '%s'\n",
			len(keys), ctx.Service.Name, ctx.Environment.Name)
	}

	if skipDeploymentFlag {
		fmt.Println("(Deployment skipped)")
	} else {
		fmt.Println("(Deployment triggered)")
	}

	return nil
}
