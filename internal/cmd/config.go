package cmd

import (
	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/contexts"
)

// configCmd is the kubectl-style group managing local contexts and
// credentials (specs/001-config-contexts). Distinct from the declarative
// apply config files accepted by `railctl apply -f`.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage local contexts and credentials",
	Long: `Manage railctl's local contexts (saved targeting profiles) and
credentials (stored Railway tokens), kubectl-style.

The config file lives at ~/.railctl/config.yaml (override with
RAILCTL_CONFIG) and is created with 0600 permissions because it stores
tokens. Contexts reference credentials by name and hold optional workspace,
required project, and optional environment defaults for every command.

Examples:
  railctl config set-credentials work --token-stdin < token.txt
  railctl config set-context shop-prod -w my-team -p shop -e production --credentials work
  railctl config use-context shop-prod
  railctl config get-contexts`,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

// loadOrNewConfig loads the config file, treating a missing file as an empty
// config (first write creates it, FR-004).
func loadOrNewConfig() (*contexts.File, error) {
	f, err := contexts.Load(contexts.ConfigPath())
	if err != nil {
		return nil, err
	}
	if f == nil {
		f = &contexts.File{}
	}
	return f, nil
}
