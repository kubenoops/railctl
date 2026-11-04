package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/contexts"
	"github.com/kubenoops/railctl/internal/output"
)

var (
	setCredToken   string
	setCredTokenIn bool
)

// configSetCredentialsCmd stores a token under a name (FR-012). Exactly one
// input channel: --token or --token-stdin. The token's type is detected via
// one live probe and persisted so listings stay local (research D2);
// a failed probe fails the write. --token-stdin is the preferred path
// (FR-013) — it keeps the value out of shell history.
var configSetCredentialsCmd = &cobra.Command{
	Use:   "set-credentials <name>",
	Short: "Store a Railway token under a name",
	Long: `Store a Railway token under a name for reference by contexts.

Exactly one input channel is allowed:
  --token-stdin   read the token from stdin (preferred — keeps the value
                  out of your shell history; trailing newline is trimmed)
  --token <v>     pass the value directly (recorded in shell history)

The token's type (account, workspace, project) is detected with one live
API probe at write time and stored for display; token values are never
shown by any command.`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigSetCredentials,
}

// configGetCredentialsCmd lists credential names and types only (FR-014,
// FR-020).
var configGetCredentialsCmd = &cobra.Command{
	Use:     "get-credentials",
	Aliases: []string{"credentials"},
	Short:   "List stored credentials",
	Long: `List stored credentials by name and token type. Token values are
never displayed.`,
	RunE: runConfigGetCredentials,
}

// configDeleteCredentialsCmd removes a credential unless referenced by any
// context (FR-015).
var configDeleteCredentialsCmd = &cobra.Command{
	Use:     "delete-credentials <name>",
	Aliases: []string{"delete-credential"},
	Short:   "Delete a stored credential",
	Long: `Delete a stored credential. Refused while any context still
references it — delete or re-point those contexts first.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := loadOrNewConfig()
		if err != nil {
			return err
		}
		if err := f.DeleteCredential(args[0]); err != nil {
			return err
		}
		if err := contexts.Save(contexts.ConfigPath(), f); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Credentials %q deleted.\n", args[0])
		return nil
	},
}

func init() {
	configSetCredentialsCmd.Flags().StringVar(&setCredToken, "token", "", "Token value (use --token-stdin instead to keep it out of shell history)")
	configSetCredentialsCmd.Flags().BoolVar(&setCredTokenIn, "token-stdin", false, "Read the token from stdin (preferred)")
	configCmd.AddCommand(configSetCredentialsCmd)
	configCmd.AddCommand(configGetCredentialsCmd)
	configCmd.AddCommand(configDeleteCredentialsCmd)
}

func runConfigSetCredentials(cmd *cobra.Command, args []string) error {
	name := args[0]

	tokenValue := ""
	switch {
	case setCredToken != "" && setCredTokenIn:
		return fmt.Errorf("pass only one of --token or --token-stdin, not both")
	case setCredTokenIn:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read token from stdin: %w", err)
		}
		tokenValue = strings.TrimRight(string(data), "\r\n")
	case setCredToken != "":
		tokenValue = setCredToken
	default:
		return fmt.Errorf("a token is required: pass --token-stdin (preferred) or --token")
	}
	if tokenValue == "" {
		return fmt.Errorf("token value is empty")
	}

	tokenType, err := detectTokenType(tokenValue)
	if err != nil {
		return fmt.Errorf("token rejected: detection failed — %w", err)
	}

	f, err := loadOrNewConfig()
	if err != nil {
		return err
	}
	f.SetCredential(name, tokenValue, tokenType)
	if err := contexts.Save(contexts.ConfigPath(), f); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Credentials %q set (type: %s).\n", name, tokenType)
	return nil
}

// detectTokenType classifies a token with one live probe, reusing the
// detection surface whoami uses (internal/cmd/whoami.go).
func detectTokenType(tokenValue string) (string, error) {
	client := newAPIClient(tokenValue)
	isProject, err := client.IsProjectToken()
	if err != nil {
		return "", err
	}
	if isProject {
		return "project", nil
	}
	isWorkspace, err := client.IsWorkspaceToken()
	if err != nil {
		return "", err
	}
	if isWorkspace {
		return "workspace", nil
	}
	return "account", nil
}

// credentialOutput is the structured output format for credentials — names
// and types only (FR-014, FR-020).
type credentialOutput struct {
	Name      string `json:"name" yaml:"name"`
	TokenType string `json:"tokenType" yaml:"tokenType"`
}

func runConfigGetCredentials(cmd *cobra.Command, args []string) error {
	format, err := getOutputFormat()
	if err != nil {
		return err
	}
	f, err := contexts.Load(contexts.ConfigPath())
	if err != nil {
		return err
	}

	printer := output.NewPrinter(format)
	switch printer.Format() {
	case output.FormatJSON:
		return printer.PrintJSON(credentialsToOutput(f))
	case output.FormatYAML:
		return printer.PrintYAML(credentialsToOutput(f))
	default:
		table := output.NewTable("Name", "Type")
		for _, cred := range f.Credentials {
			table.AddRow(cred.Name, cred.TokenType)
		}
		return printer.PrintTable(table)
	}
}

func credentialsToOutput(f *contexts.File) []credentialOutput {
	out := make([]credentialOutput, len(f.Credentials))
	for i, cred := range f.Credentials {
		out[i] = credentialOutput{Name: cred.Name, TokenType: cred.TokenType}
	}
	return out
}
