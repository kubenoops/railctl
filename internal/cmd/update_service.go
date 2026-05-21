package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/spf13/cobra"
)

var (
	updateServiceImage              string
	updateRegistryUsername          string
	updateRegistryPassword          string
	updateServiceYes                bool
	updateServiceSkipDeployment     bool
	updateServiceStartCommand       string
	updateServiceRestartPolicy      string
	updateServiceMaxRetries         int
	updateServiceReplicas           int
	updateServiceHealthcheckPath    string
	updateServiceHealthcheckTimeout int
	updateServiceGenerateDomain     int
	updateServiceGenerateTCP        int
	updateServiceRemoveDomain       bool
	updateServiceRemoveTCP          bool
	updateServiceAwait              bool
	updateServiceTimeout            int
)

var updateServiceCmd = &cobra.Command{
	Use:     "service [service-name]",
	Aliases: []string{"svc"},
	Short:   "Update a service's Docker image, registry credentials, or deploy configuration",
	Long: `Update a service's Docker image, registry credentials, or deploy configuration.

IMAGE UPDATES:
  Railway's API replaces the entire service configuration on update.
  - If you update the image for a PRIVATE registry, you MUST provide credentials
  - If you don't provide credentials, any existing credentials will be CLEARED
  - For public images, credentials are not needed

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
  
  Railway queries the health check path and expects HTTP 200 for a healthy service.
  New deployments activate only after passing health checks.

DOMAIN GENERATION:
  - --generate-domain: Generate a Railway domain (*.up.railway.app) for the given application port
    Safe to call multiple times (idempotent - skips if a domain already exists)
  - --remove-domain: Remove the first existing domain from the service

TCP PROXY:
  - --generate-tcp: Generate a TCP proxy for the given application port (e.g., 5432 for Postgres)
    Safe to call multiple times (idempotent - skips if a proxy for the same port already exists)
  - --remove-tcp: Remove the first existing TCP proxy from the service

Variable Value Syntax:
  Regular value:        "my-value"
  Service reference:    '${{service-name.RAILWAY_PRIVATE_DOMAIN}}'
  
Service references are resolved by Railway at runtime.
Use single quotes to prevent shell expansion.`,
	Args: cobra.ExactArgs(1),
	Example: `  # Update to a public image
  railctl update service api --image node:20-alpine -p my-project -e production

  # Update to a private image (MUST provide credentials)
  railctl update service app --image registry.example.com/myapp:v2 \
    --registry-username user --registry-password token -p my-project -e production

  # Update deploy configuration
  railctl update service api --restart-policy ON_FAILURE --max-retries 3 \
    --replicas 2 -p my-project -e production

  # Configure health checks for zero-downtime deployments
  railctl update service api --healthcheck-path /health --healthcheck-timeout 90 \
    -p my-project -e production

  # Combined update: image + deploy config + health checks
  railctl update service api --image myapp:v2 --replicas 3 \
    --healthcheck-path /api/health --healthcheck-timeout 60 \
    -p my-project -e production

  # Generate a Railway domain for the service
  railctl update service api --generate-domain 5678 -p my-project -e production

  # Remove the current domain from the service
  railctl update service api --remove-domain -p my-project -e production

  # Generate a TCP proxy for Postgres
  railctl update service db --generate-tcp 5432 -p my-project -e production

  # Remove the current TCP proxy from the service
  railctl update service db --remove-tcp -p my-project -e production`,
	RunE: runUpdateService,
}

func init() {
	// Image and credentials
	updateServiceCmd.Flags().StringVar(&updateServiceImage, "image", "", "New Docker image to deploy")
	updateServiceCmd.Flags().StringVar(&updateRegistryUsername, "registry-username", "", "Username for private Docker registry (env: RAILCTL_REGISTRY_USERNAME)")
	updateServiceCmd.Flags().StringVar(&updateRegistryPassword, "registry-password", "", "Password/token for private Docker registry (env: RAILCTL_REGISTRY_PASSWORD)")

	// Deploy configuration
	updateServiceCmd.Flags().StringVar(&updateServiceStartCommand, "start-command", "", "Start command for the service")
	updateServiceCmd.Flags().StringVar(&updateServiceRestartPolicy, "restart-policy", "", "Restart policy: ON_FAILURE, ALWAYS, NEVER")
	updateServiceCmd.Flags().IntVar(&updateServiceMaxRetries, "max-retries", 0, "Max restart retries (requires --restart-policy)")
	updateServiceCmd.Flags().IntVar(&updateServiceReplicas, "replicas", 0, "Number of replicas (horizontal scaling)")

	// Health check configuration
	updateServiceCmd.Flags().StringVar(&updateServiceHealthcheckPath, "healthcheck-path", "", "Health check endpoint path (e.g., /health)")
	updateServiceCmd.Flags().IntVar(&updateServiceHealthcheckTimeout, "healthcheck-timeout", 0, "Health check timeout in seconds (default: 300)")

	updateServiceCmd.Flags().BoolVarP(&updateServiceYes, "yes", "y", false, "Skip confirmation prompt")
	updateServiceCmd.Flags().BoolVar(&updateServiceSkipDeployment, "skip-deployment", false, "Skip triggering a new deployment")

	// Domain generation
	updateServiceCmd.Flags().IntVar(&updateServiceGenerateDomain, "generate-domain", 0, "Generate a Railway domain (*.up.railway.app) for the given application port")
	updateServiceCmd.Flags().BoolVar(&updateServiceRemoveDomain, "remove-domain", false, "Remove the first existing domain from the service")

	// TCP proxy generation
	updateServiceCmd.Flags().IntVar(&updateServiceGenerateTCP, "generate-tcp", 0, "Generate a TCP proxy for the given application port (e.g., 5432)")
	updateServiceCmd.Flags().BoolVar(&updateServiceRemoveTCP, "remove-tcp", false, "Remove the first existing TCP proxy from the service")

	// Await completion
	updateServiceCmd.Flags().BoolVar(&updateServiceAwait, "await-completion", false, "Wait for the deployment to reach a terminal status before returning")
	updateServiceCmd.Flags().IntVar(&updateServiceTimeout, "timeout", 600, "Timeout in seconds for --await-completion (default: 600)")

	updateCmd.AddCommand(updateServiceCmd)
}

func runUpdateService(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	token, err := getToken()
	if err != nil {
		return err
	}

	client := newAPIClient(token)

	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		ServiceName:     serviceName,
		NeedEnvironment: true,
		NeedService:     true,
	})
	if err != nil {
		return err
	}

	targetService := ctx.Service

	// Build registry credentials if provided
	creds := buildRegistryCredentials(updateRegistryUsername, updateRegistryPassword)

	// Check if we have deploy configuration flags
	hasDeployConfig := hasDeployConfigFlags(cmd)

	// Require at least one supported mutation
	if updateServiceImage == "" && creds == nil && !hasDeployConfig && updateServiceGenerateDomain <= 0 && updateServiceGenerateTCP <= 0 && !updateServiceRemoveDomain && !updateServiceRemoveTCP {
		return fmt.Errorf("at least one of --image, registry credentials, deploy configuration, --generate-domain, --generate-tcp, --remove-domain, or --remove-tcp is required")
	}

	// Validate restart policy and conflicting networking flags
	if err := validateDeployConfigFlags(cmd); err != nil {
		return err
	}
	if err := validateNetworkingMutationFlags(); err != nil {
		return err
	}
	// Check if we're updating image without credentials for a potentially private registry
	if err := warnPrivateRegistryUpdate(targetService.Source, updateServiceImage, creds, updateServiceYes); err != nil {
		return err
	}

	// Update image/credentials if provided
	if updateServiceImage != "" || creds != nil {
		err = client.UpdateServiceInstance(targetService.ID, ctx.Environment.ID, updateServiceImage, creds)
		if err != nil {
			return fmt.Errorf("failed to update service: %w", err)
		}
	}

	// Update deploy configuration if provided
	if hasDeployConfig {
		if err := applyUpdateDeployConfig(cmd, client, targetService.ID, ctx.Environment.ID); err != nil {
			return err
		}
	}

	// Trigger a new deployment for any service mutation unless --skip-deployment
	var deploymentID string
	if !updateServiceSkipDeployment && (updateServiceImage != "" || creds != nil || hasDeployConfig) {
		deploymentID, err = client.DeployServiceInstance(targetService.ID, ctx.Environment.ID)
		if err != nil {
			return fmt.Errorf("failed to trigger deployment: %w", err)
		}
	}

	// Generate domain if requested
	if updateServiceGenerateDomain > 0 {
		if err := validatePort(updateServiceGenerateDomain, "generate-domain"); err != nil {
			return err
		}
		if err := generateServiceDomain(client, ctx.Project.ID, ctx.Environment.ID, targetService.ID, updateServiceGenerateDomain); err != nil {
			return err
		}
	}

	// Remove domain if requested
	if updateServiceRemoveDomain {
		if err := removeServiceDomain(client, ctx.Project.ID, ctx.Environment.ID, targetService.ID); err != nil {
			return err
		}
	}

	// Generate TCP proxy if requested
	if updateServiceGenerateTCP > 0 {
		if err := validatePort(updateServiceGenerateTCP, "generate-tcp"); err != nil {
			return err
		}
		if err := generateTCPProxy(client, ctx.Environment.ID, targetService.ID, updateServiceGenerateTCP); err != nil {
			return err
		}
	}

	// Remove TCP proxy if requested
	if updateServiceRemoveTCP {
		if err := removeTCPProxy(client, ctx.Environment.ID, targetService.ID); err != nil {
			return err
		}
	}

	// Output success message
	printUpdateServiceResult(cmd, targetService.Name, updateServiceImage, creds, hasDeployConfig, deploymentID)

	// Await deployment completion if requested
	if updateServiceAwait && deploymentID != "" {
		return awaitDeployment(client, ctx.Project.ID, ctx.Environment.ID, targetService.ID, deploymentID, targetService.Name, updateServiceTimeout)
	}
	return nil
}

// buildRegistryCredentials creates RegistryCredentials from flags/env vars.
func buildRegistryCredentials(username, password string) *api.RegistryCredentials {
	if username == "" {
		username = os.Getenv("RAILCTL_REGISTRY_USERNAME")
	}
	if password == "" {
		password = os.Getenv("RAILCTL_REGISTRY_PASSWORD")
	}
	if username != "" && password != "" {
		return &api.RegistryCredentials{
			Username: username,
			Password: password,
		}
	}
	return nil
}

// validateDeployConfigFlags validates deploy config flag combinations.
func validateDeployConfigFlags(cmd *cobra.Command) error {
	if updateServiceRestartPolicy != "" {
		policy := strings.ToUpper(updateServiceRestartPolicy)
		if policy != "ON_FAILURE" && policy != "ALWAYS" && policy != "NEVER" {
			return fmt.Errorf("invalid restart policy '%s'. Must be one of: ON_FAILURE, ALWAYS, NEVER", updateServiceRestartPolicy)
		}
		updateServiceRestartPolicy = policy
	}

	if cmd.Flags().Changed("max-retries") && !cmd.Flags().Changed("restart-policy") {
		return fmt.Errorf("--max-retries requires --restart-policy")
	}

	if cmd.Flags().Changed("replicas") && updateServiceReplicas < 1 {
		return fmt.Errorf("--replicas must be >= 1")
	}

	return nil
}

func validateNetworkingMutationFlags() error {
	if updateServiceGenerateDomain > 0 && updateServiceRemoveDomain {
		return fmt.Errorf("--generate-domain and --remove-domain cannot be used together")
	}
	if updateServiceGenerateTCP > 0 && updateServiceRemoveTCP {
		return fmt.Errorf("--generate-tcp and --remove-tcp cannot be used together")
	}
	return nil
}

// warnPrivateRegistryUpdate warns when updating a private registry image without credentials.
func warnPrivateRegistryUpdate(currentSource, newImage string, creds *api.RegistryCredentials, skipPrompt bool) error {
	if newImage == "" || creds != nil {
		return nil
	}

	isPrivateRegistry := isPrivateDockerRegistry(newImage)
	if currentSource != "" {
		isPrivateRegistry = isPrivateRegistry || isPrivateDockerRegistry(currentSource)
	}

	if !isPrivateRegistry || skipPrompt {
		return nil
	}

	fmt.Printf("\n⚠️  WARNING: Updating image without credentials\n")
	fmt.Printf("   Current image: %s\n", currentSource)
	fmt.Printf("   New image:     %s\n", newImage)
	fmt.Printf("\n")
	fmt.Printf("   This appears to be a private registry. If you don't provide credentials,\n")
	fmt.Printf("   any existing credentials will be CLEARED and the deployment may FAIL.\n")
	fmt.Printf("\n")
	fmt.Printf("   To provide credentials, use:\n")
	fmt.Printf("     --registry-username <user> --registry-password <token>\n")
	fmt.Printf("   or set RAILCTL_REGISTRY_USERNAME and RAILCTL_REGISTRY_PASSWORD\n")
	fmt.Printf("\n")
	fmt.Printf("Continue without credentials? (y/N): ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Println("Update cancelled")
		return fmt.Errorf("update cancelled by user")
	}

	return nil
}

// applyUpdateDeployConfig applies deploy config changes from update flags.
func applyUpdateDeployConfig(cmd *cobra.Command, client api.APIClient, serviceID, envID string) error {
	var startCmd, restartPolicy, healthcheckPath *string
	var maxRetries, replicas, healthcheckTimeout *int

	if cmd.Flags().Changed("start-command") {
		startCmd = &updateServiceStartCommand
	}
	if cmd.Flags().Changed("restart-policy") {
		restartPolicy = &updateServiceRestartPolicy
	}
	if cmd.Flags().Changed("max-retries") {
		maxRetries = &updateServiceMaxRetries
	}
	if cmd.Flags().Changed("replicas") {
		replicas = &updateServiceReplicas
	}
	if cmd.Flags().Changed("healthcheck-path") {
		healthcheckPath = &updateServiceHealthcheckPath
	}
	if cmd.Flags().Changed("healthcheck-timeout") {
		healthcheckTimeout = &updateServiceHealthcheckTimeout
	}

	err := client.UpdateServiceInstanceConfig(serviceID, envID, startCmd, restartPolicy, maxRetries, replicas, healthcheckPath, healthcheckTimeout)
	if err != nil {
		return fmt.Errorf("failed to update deploy configuration: %w", err)
	}
	return nil
}

func removeServiceDomain(client api.APIClient, projectID, environmentID, serviceID string) error {
	domains, err := client.ListDomains(projectID, environmentID, serviceID)
	if err != nil {
		return fmt.Errorf("failed to check existing domains: %w", err)
	}

	if len(domains.CustomDomains) > 0 {
		d := domains.CustomDomains[0]
		if err := client.DeleteCustomDomain(d.ID); err != nil {
			return fmt.Errorf("failed to remove custom domain: %w", err)
		}
		fmt.Printf("Custom domain removed: %s\n", d.Domain)
		return nil
	}

	if len(domains.ServiceDomains) > 0 {
		d := domains.ServiceDomains[0]
		if err := client.DeleteServiceDomain(d.ID); err != nil {
			return fmt.Errorf("failed to remove domain: %w", err)
		}
		fmt.Printf("Domain removed: https://%s\n", d.Domain)
		return nil
	}

	fmt.Println("No domains to remove")
	return nil
}

func removeTCPProxy(client api.APIClient, environmentID, serviceID string) error {
	proxies, err := client.ListTCPProxies(environmentID, serviceID)
	if err != nil {
		return fmt.Errorf("failed to check existing TCP proxies: %w", err)
	}

	if len(proxies) == 0 {
		fmt.Println("No TCP proxies to remove")
		return nil
	}

	p := proxies[0]
	if err := client.DeleteTCPProxy(p.ID); err != nil {
		return fmt.Errorf("failed to remove TCP proxy: %w", err)
	}
	fmt.Printf("TCP proxy removed: %s:%d → port %d\n", p.Domain, p.ProxyPort, p.ApplicationPort)
	return nil
}

// printUpdateServiceResult prints the success message after an update.
func printUpdateServiceResult(cmd *cobra.Command, serviceName, image string, creds *api.RegistryCredentials, hasDeployConfig bool, deploymentID string) {
	if image != "" && creds != nil {
		fmt.Printf("Service '%s' updated to image '%s' (private registry)\n", serviceName, image)
	} else if image != "" {
		fmt.Printf("Service '%s' updated to image '%s'\n", serviceName, image)
	} else if creds != nil {
		fmt.Printf("Service '%s' registry credentials updated\n", serviceName)
	}

	if hasDeployConfig {
		fmt.Printf("Deploy configuration updated:\n")
		if cmd.Flags().Changed("start-command") {
			fmt.Printf("  - Start command: %s\n", updateServiceStartCommand)
		}
		if cmd.Flags().Changed("restart-policy") {
			fmt.Printf("  - Restart policy: %s\n", updateServiceRestartPolicy)
		}
		if cmd.Flags().Changed("max-retries") {
			fmt.Printf("  - Max retries: %d\n", updateServiceMaxRetries)
		}
		if cmd.Flags().Changed("replicas") {
			fmt.Printf("  - Replicas: %d\n", updateServiceReplicas)
		}
		if cmd.Flags().Changed("healthcheck-path") {
			fmt.Printf("  - Health check path: %s\n", updateServiceHealthcheckPath)
		}
		if cmd.Flags().Changed("healthcheck-timeout") {
			fmt.Printf("  - Health check timeout: %ds\n", updateServiceHealthcheckTimeout)
		}
	}

	if deploymentID != "" {
		fmt.Printf("Deployment triggered: %s\n", deploymentID)
	}
}

// isPrivateDockerRegistry detects if an image is from a private registry
// Returns true if the image appears to be from a private registry (not Docker Hub)
func isPrivateDockerRegistry(image string) bool {
	if image == "" {
		return false
	}

	// Docker Hub images don't have a domain prefix (e.g., "nginx:latest", "node:20")
	// Private registries have a domain (e.g., "gcr.io/...", "registry.example.com/...")
	slashIndex := strings.Index(image, "/")
	if slashIndex == -1 {
		return false
	}

	// Check if there's a dot or colon in the part before the first slash
	domainPart := image[:slashIndex]
	return strings.Contains(domainPart, ".") || strings.Contains(domainPart, ":")
}
