package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/types"
	"github.com/spf13/cobra"
)

var createServiceCmd = &cobra.Command{
	Use:     "service <name> --image <image:tag>",
	Aliases: []string{"svc"},
	Short:   "Create a new service in a project from a Docker image",
	Long: `Create a new service in a project from a Docker image.
The service will be deployed to all environments in the project.

For private Docker registries, use --registry-username and --registry-password
flags, or set RAILCTL_REGISTRY_USERNAME and RAILCTL_REGISTRY_PASSWORD environment
variables. Note: Private registry support requires a Railway Pro plan.

DEPLOY CONFIGURATION:
  Configure how your service runs and scales:
  - --start-command: Override the container's default start command
  - --restart-policy: Control when the service restarts (ON_FAILURE, ALWAYS, NEVER)
  - --max-retries: Maximum restart attempts (requires --restart-policy)
  - --replicas: Number of instances to run (horizontal scaling)

HEALTH CHECK CONFIGURATION:
  Configure zero-downtime deployments:
  - --healthcheck-path: HTTP endpoint to check (e.g., /health, /api/health)
  - --healthcheck-timeout: Max seconds to wait for response (default: 300)

DOMAIN GENERATION:
  - --generate-domain: Generate a Railway domain (*.up.railway.app) for the given application port (e.g., 5678 for n8n)

TCP PROXY:
  - --generate-tcp: Generate a TCP proxy for the given application port (e.g., 5432 for Postgres)`,
	Args: cobra.ExactArgs(1),
	Example: `  railctl create service api --image node:18-alpine -p my-project
  railctl create service nginx --image nginx:latest -p my-project
  railctl create service app --image registry.example.com/myapp:v1 \
    --registry-username user --registry-password token -p my-project
  railctl create service api --image node:20 --start-command "npm start" \
    --restart-policy ON_FAILURE --max-retries 3 --replicas 2 -p my-project
  railctl create service api --image node:18-alpine --generate-domain 5678 -p my-project
  railctl create service db --image postgres:16 --generate-tcp 5432 -p my-project`,
	RunE: runCreateService,
}

var (
	serviceImage                    string
	createRegistryUsername          string
	createRegistryPassword          string
	createServiceStartCommand       string
	createServiceRestartPolicy      string
	createServiceMaxRetries         int
	createServiceReplicas           int
	createServiceHealthcheckPath    string
	createServiceHealthcheckTimeout int
	createServiceGenerateDomain     int
	createServiceGenerateTCP        int
)

func init() {
	createServiceCmd.Flags().StringVar(&serviceImage, "image", "", "Docker image to deploy (required)")
	createServiceCmd.Flags().StringVar(&createRegistryUsername, "registry-username", "", "Username for private Docker registry")
	createServiceCmd.Flags().StringVar(&createRegistryPassword, "registry-password", "", "Password/token for private Docker registry")

	// Deploy configuration flags
	createServiceCmd.Flags().StringVar(&createServiceStartCommand, "start-command", "", "Override container start command")
	createServiceCmd.Flags().StringVar(&createServiceRestartPolicy, "restart-policy", "", "Restart policy: ON_FAILURE, ALWAYS, NEVER")
	createServiceCmd.Flags().IntVar(&createServiceMaxRetries, "max-retries", 0, "Maximum restart attempts (requires --restart-policy)")
	createServiceCmd.Flags().IntVar(&createServiceReplicas, "replicas", 0, "Number of service instances")

	// Health check flags
	createServiceCmd.Flags().StringVar(&createServiceHealthcheckPath, "healthcheck-path", "", "HTTP health check endpoint")
	createServiceCmd.Flags().IntVar(&createServiceHealthcheckTimeout, "healthcheck-timeout", 0, "Health check timeout in seconds")

	// Domain generation
	createServiceCmd.Flags().IntVar(&createServiceGenerateDomain, "generate-domain", 0, "Generate a Railway domain (*.up.railway.app) for the given application port")

	// TCP proxy generation
	createServiceCmd.Flags().IntVar(&createServiceGenerateTCP, "generate-tcp", 0, "Generate a TCP proxy for the given application port (e.g., 5432)")

	createServiceCmd.MarkFlagRequired("image")
	createCmd.AddCommand(createServiceCmd)
}

func runCreateService(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	token, err := getToken()
	if err != nil {
		return err
	}

	client := newAPIClient(token)

	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		NeedEnvironment: true,
	})
	if err != nil {
		return err
	}

	// Build registry credentials if provided
	var creds *api.RegistryCredentials
	regUser := createRegistryUsername
	regPass := createRegistryPassword
	if regUser == "" {
		regUser = os.Getenv("RAILCTL_REGISTRY_USERNAME")
	}
	if regPass == "" {
		regPass = os.Getenv("RAILCTL_REGISTRY_PASSWORD")
	}
	if regUser != "" && regPass != "" {
		creds = &api.RegistryCredentials{
			Username: regUser,
			Password: regPass,
		}
	}

	// Validate restart policy if provided
	if createServiceRestartPolicy != "" {
		policy := strings.ToUpper(createServiceRestartPolicy)
		if policy != "ON_FAILURE" && policy != "ALWAYS" && policy != "NEVER" {
			return fmt.Errorf("invalid restart policy '%s'. Must be one of: ON_FAILURE, ALWAYS, NEVER", createServiceRestartPolicy)
		}
		createServiceRestartPolicy = policy
	}

	// Validate max-retries requires restart-policy
	if cmd.Flags().Changed("max-retries") && !cmd.Flags().Changed("restart-policy") {
		return fmt.Errorf("--max-retries requires --restart-policy")
	}

	// Validate replicas
	if cmd.Flags().Changed("replicas") && createServiceReplicas < 1 {
		return fmt.Errorf("--replicas must be >= 1")
	}

	// Create the service with image in the specified environment
	svc, err := client.CreateService(ctx.Project.ID, ctx.Environment.ID, serviceName, serviceImage, creds)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	if creds != nil {
		fmt.Printf("Service '%s' created with image '%s' (private registry) (ID: %s)\n", svc.Name, serviceImage, svc.ID)
	} else {
		fmt.Printf("Service '%s' created with image '%s' (ID: %s)\n", svc.Name, serviceImage, svc.ID)
	}

	// Clean up service instances in other environments
	cleanupOtherEnvironments(client, ctx.Project.ID, ctx.Environment, svc)

	// Apply deployment configuration if any flags were provided
	if hasDeployConfigFlags(cmd) {
		err = applyDeployConfig(cmd, client, svc.ID, ctx.Environment)
		if err != nil {
			return err
		}
	}

	// Generate domain if requested
	if createServiceGenerateDomain > 0 {
		if err := validatePort(createServiceGenerateDomain, "generate-domain"); err != nil {
			return err
		}
		if err := generateServiceDomain(client, ctx.Project.ID, ctx.Environment.ID, svc.ID, createServiceGenerateDomain); err != nil {
			return err
		}
	}

	// Generate TCP proxy if requested
	if createServiceGenerateTCP > 0 {
		if err := validatePort(createServiceGenerateTCP, "generate-tcp"); err != nil {
			return err
		}
		if err := generateTCPProxy(client, ctx.Environment.ID, svc.ID, createServiceGenerateTCP); err != nil {
			return err
		}
	}

	return nil
}

func cleanupOtherEnvironments(client api.APIClient, projectID string, targetEnv types.Environment, svc types.Service) {
	// Railway creates services in all non-fork environments by default,
	// so we need to explicitly remove instances from non-target environments
	time.Sleep(500 * time.Millisecond)

	allEnvs, err := client.ListEnvironments(projectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not list environments for cleanup: %v\n", err)
		return
	}

	cleanupErrors := []string{}
	for _, otherEnv := range allEnvs {
		if otherEnv.ID != targetEnv.ID {
			// Retry cleanup with exponential backoff
			maxRetries := 3
			var lastErr error
			for attempt := 0; attempt < maxRetries; attempt++ {
				if attempt > 0 {
					backoff := time.Duration(1<<uint(attempt-1)) * time.Second
					time.Sleep(backoff)
				}

				lastErr = client.DeleteServiceInstance(svc.ID, otherEnv.ID)
				if lastErr == nil {
					break
				}
			}

			if lastErr != nil {
				errMsg := fmt.Sprintf("environment '%s': %v (after %d retries)", otherEnv.Name, lastErr, maxRetries)
				cleanupErrors = append(cleanupErrors, errMsg)
			}
		}
	}

	if len(cleanupErrors) > 0 {
		fmt.Fprintf(os.Stderr, "\n⚠️  Warning: Could not remove service from other environments:\n")
		for _, errMsg := range cleanupErrors {
			fmt.Fprintf(os.Stderr, "  - %s\n", errMsg)
		}
		fmt.Fprintf(os.Stderr, "\nThe service was created successfully in '%s', but may also exist in other environments.\n", targetEnv.Name)
		fmt.Fprintf(os.Stderr, "You can manually delete unwanted instances using: railctl delete service %s -e <environment>\n\n", svc.Name)
	}
}

func hasDeployConfigFlags(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("start-command") ||
		cmd.Flags().Changed("restart-policy") ||
		cmd.Flags().Changed("max-retries") ||
		cmd.Flags().Changed("replicas") ||
		cmd.Flags().Changed("healthcheck-path") ||
		cmd.Flags().Changed("healthcheck-timeout")
}

func applyDeployConfig(cmd *cobra.Command, client api.APIClient, serviceID string, env types.Environment) error {
	var startCmd, restartPolicy, healthcheckPath *string
	var maxRetries, replicas, healthcheckTimeout *int

	if cmd.Flags().Changed("start-command") {
		startCmd = &createServiceStartCommand
	}
	if cmd.Flags().Changed("restart-policy") {
		restartPolicy = &createServiceRestartPolicy
	}
	if cmd.Flags().Changed("max-retries") {
		maxRetries = &createServiceMaxRetries
	}
	if cmd.Flags().Changed("replicas") {
		replicas = &createServiceReplicas
	}
	if cmd.Flags().Changed("healthcheck-path") {
		healthcheckPath = &createServiceHealthcheckPath
	}
	if cmd.Flags().Changed("healthcheck-timeout") {
		healthcheckTimeout = &createServiceHealthcheckTimeout
	}

	err := client.UpdateServiceInstanceConfig(
		serviceID,
		env.ID,
		startCmd,
		restartPolicy,
		maxRetries,
		replicas,
		healthcheckPath,
		healthcheckTimeout,
	)
	if err != nil {
		return fmt.Errorf("failed to apply deploy config to environment '%s': %w", env.Name, err)
	}
	fmt.Printf("Deploy configuration applied to environment '%s'\n", env.Name)
	return nil
}

// validatePort checks that a port number is within the valid range (1-65535).
func validatePort(port int, flagName string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("--%s must be between 1 and 65535, got %d", flagName, port)
	}
	return nil
}

// generateServiceDomain generates a Railway domain for a service, or prints the existing one.
// This is idempotent: if any domain (Railway or custom) already exists, it skips creation.
// If a domain exists but its targetPort differs from the requested port, it auto-updates.
func generateServiceDomain(client api.APIClient, projectID, environmentID, serviceID string, port int) error {
	// Check for existing domains first (both Railway and custom)
	domains, err := client.ListDomains(projectID, environmentID, serviceID)
	if err != nil {
		return fmt.Errorf("failed to check existing domains: %w", err)
	}

	// Custom domains take priority
	if len(domains.CustomDomains) > 0 {
		d := domains.CustomDomains[0]
		if port > 0 && (d.TargetPort == nil || *d.TargetPort != port) {
			if err := client.UpdateCustomDomainPort(d.ID, environmentID, port); err != nil {
				return fmt.Errorf("failed to update custom domain port: %w", err)
			}
			if d.TargetPort != nil {
				fmt.Printf("Custom domain exists: https://%s (port updated: %d → %d)\n", d.Domain, *d.TargetPort, port)
			} else {
				fmt.Printf("Custom domain exists: https://%s (port set: %d)\n", d.Domain, port)
			}
		} else {
			fmt.Printf("Custom domain exists: https://%s\n", d.Domain)
		}
		return nil
	}

	// Check Railway-generated service domains
	if len(domains.ServiceDomains) > 0 {
		d := domains.ServiceDomains[0]
		if port > 0 && (d.TargetPort == nil || *d.TargetPort != port) {
			if err := client.UpdateServiceDomainPort(d.ID, d.Domain, environmentID, serviceID, port); err != nil {
				return fmt.Errorf("failed to update domain port: %w", err)
			}
			if d.TargetPort != nil {
				fmt.Printf("Domain already exists: https://%s (port updated: %d → %d)\n", d.Domain, *d.TargetPort, port)
			} else {
				fmt.Printf("Domain already exists: https://%s (port set: %d)\n", d.Domain, port)
			}
		} else {
			fmt.Printf("Domain already exists: https://%s\n", d.Domain)
		}
		return nil
	}

	// No domains exist — create a new Railway domain with the port set at creation
	domain, err := client.CreateServiceDomain(serviceID, environmentID, port)
	if err != nil {
		return fmt.Errorf("failed to generate domain: %w", err)
	}

	if port > 0 {
		fmt.Printf("Domain generated: https://%s (port: %d)\n", domain.Domain, port)
		return nil
	}

	fmt.Printf("Domain generated: https://%s\n", domain.Domain)
	return nil
}

// generateTCPProxy generates a TCP proxy for a service on the given port, or prints the existing one.
// This is idempotent: if a proxy for the same port already exists, it skips creation.
func generateTCPProxy(client api.APIClient, environmentID, serviceID string, applicationPort int) error {
	// Check for existing TCP proxies first (idempotent)
	existing, err := client.ListTCPProxies(environmentID, serviceID)
	if err != nil {
		return fmt.Errorf("failed to check existing TCP proxies: %w", err)
	}

	for _, proxy := range existing {
		if proxy.ApplicationPort == applicationPort {
			fmt.Printf("TCP proxy already exists: %s:%d → port %d\n", proxy.Domain, proxy.ProxyPort, proxy.ApplicationPort)
			return nil
		}
	}

	// Create a new TCP proxy
	proxy, err := client.CreateTCPProxy(applicationPort, environmentID, serviceID)
	if err != nil {
		return fmt.Errorf("failed to generate TCP proxy: %w", err)
	}

	fmt.Printf("TCP proxy generated: %s:%d → port %d\n", proxy.Domain, proxy.ProxyPort, proxy.ApplicationPort)
	return nil
}
