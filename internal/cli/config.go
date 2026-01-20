package cli

import (
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage ribbin.jsonc configuration",
	Long: `Add, remove, list, edit, and show shim configurations without manual file editing.

Subcommands:
  list     Display all configured shims
  show     Show effective configuration with provenance tracking

Use "ribbin config <command> --help" for more information about a command.`,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configAddCmd)
	configCmd.AddCommand(configRemoveCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configShowCmd)
}
