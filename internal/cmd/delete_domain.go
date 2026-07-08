package cmd

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/resolver"
	"github.com/spf13/cobra"
)

var deleteDomainYes bool

var deleteDomainCmd = &cobra.Command{
	Use:   "domain <name>",
	Short: "Delete a custom domain from a service",
	Long: `Delete a user-owned custom domain from a service by name.

Only custom domains can be deleted with this command; Railway-generated
domains (*.up.railway.app) are managed via 'create service --generate-domain'
and the Railway dashboard.

Deletion is permanent: traffic to the domain stops routing immediately, and
re-adding it later requires DNS re-verification.`,
	Args: cobra.ExactArgs(1),
	Example: `  railctl delete domain app.example.com -s api -p my-project -e production
  railctl delete domain app.example.com -s api -p my-project -e production --yes`,
	RunE: runDeleteDomain,
}

func init() {
	deleteDomainCmd.Flags().BoolVarP(&deleteDomainYes, "yes", "y", false, "Skip confirmation prompt")
	deleteCmd.AddCommand(deleteDomainCmd)
}

func runDeleteDomain(cmd *cobra.Command, args []string) error {
	domainName := args[0]

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

	// Resolve the domain within the service's custom domains for a friendly
	// prompt + a not-found error that lists what does exist.
	domains, err := client.ListDomains(ctx.Project.ID, ctx.Environment.ID, ctx.Service.ID)
	if err != nil {
		return fmt.Errorf("failed to list domains: %w", err)
	}
	var found *api.CustomDomain
	for i := range domains.CustomDomains {
		if domains.CustomDomains[i].Domain == domainName {
			found = &domains.CustomDomains[i]
			break
		}
	}
	if found == nil {
		available := make([]string, len(domains.CustomDomains))
		for i := range domains.CustomDomains {
			available[i] = domains.CustomDomains[i].Domain
		}
		return resolver.ErrNotFound{
			Resource:  "custom domain",
			Name:      domainName,
			In:        fmt.Sprintf("on service '%s'", ctx.Service.Name),
			Available: available,
		}
	}

	if !deleteDomainYes {
		fmt.Fprintf(cmd.OutOrStdout(), "Delete custom domain '%s' from service '%s' in %s/%s? [y/N]: ",
			found.Domain, ctx.Service.Name, ctx.Project.Name, ctx.Environment.Name)
		reader := bufio.NewReader(cmd.InOrStdin())
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Fprintln(cmd.OutOrStdout(), "Deletion cancelled.")
			return nil
		}
	}

	if err := client.DeleteCustomDomain(found.ID); err != nil {
		return fmt.Errorf("failed to delete custom domain: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Custom domain '%s' deleted.\n", found.Domain)
	return nil
}
