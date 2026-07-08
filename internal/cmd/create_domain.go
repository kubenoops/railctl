package cmd

import (
	"fmt"
	"strings"

	"github.com/kubenoops/railctl/internal/apply"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/spf13/cobra"
)

var createDomainPort int

var createDomainCmd = &cobra.Command{
	Use:   "domain <name>",
	Short: "Create a custom domain on a service",
	Long: `Create a user-owned custom domain (e.g. app.example.com) on a service.

Railway returns the DNS record(s) you must add at your DNS provider — a
CNAME/A record for routing and, until the domain is verified, a TXT record
for verification. Verification is manual (DNS propagation); check progress
with 'railctl get domains'.

Use --port to route the domain to a specific application port. When omitted,
Railway auto-detects the port.

For Railway-generated domains (*.up.railway.app) use
'railctl create service --generate-domain' or
'railctl update service --generate-domain' instead.`,
	Args: cobra.ExactArgs(1),
	Example: `  railctl create domain app.example.com -s api -p my-project -e production
  railctl create domain app.example.com -s api --port 3000 -p my-project -e production`,
	RunE: runCreateDomain,
}

func init() {
	createDomainCmd.Flags().IntVar(&createDomainPort, "port", 0, "Application port the domain routes to (default: Railway auto-detects)")
	createCmd.AddCommand(createDomainCmd)
}

func runCreateDomain(cmd *cobra.Command, args []string) error {
	domainName := args[0]

	// Basic sanity: a custom domain is a FQDN the user owns.
	if !strings.Contains(domainName, ".") {
		return fmt.Errorf("invalid domain '%s': a custom domain must be a fully qualified name like app.example.com", domainName)
	}

	if cmd.Flags().Changed("port") {
		if err := validatePort(createDomainPort, "port"); err != nil {
			return err
		}
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

	created, err := client.CreateCustomDomain(ctx.Project.ID, ctx.Environment.ID, ctx.Service.ID, domainName, createDomainPort)
	if err != nil {
		return fmt.Errorf("failed to create custom domain '%s': %w", domainName, err)
	}

	// Same DNS-record rendering as `apply` uses for declared customDomains.
	apply.PrintCustomDomainDNS(created, cmd.OutOrStdout())
	return nil
}
