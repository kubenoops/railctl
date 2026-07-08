package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/skill"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Print the railctl usage skill (agent-friendly guide)",
	Long: `Print the embedded railctl usage skill — a self-contained Markdown guide
to railctl covering the command surface, declarative apply/diff, volume
backups, and the token model (account/workspace vs project-scoped tokens and
their limitations).

The guide is embedded into the binary at build time, so it always matches the
version you are running and needs no network access. It is written to be
consumed by AI agents as well as humans; pipe it to a file to save it as a
skill:

  railctl skill > railctl.skill.md`,
	Example: `  railctl skill
  railctl skill | less
  railctl skill > railctl.skill.md`,
	Args: cobra.NoArgs,
	// No token or network needed — this is static, embedded content.
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := fmt.Fprint(cmd.OutOrStdout(), skill.Content())
		return err
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)
}
