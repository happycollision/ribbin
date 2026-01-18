package cli

import (
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage ribbin.toml configuration",
	Long: `Add, remove, list, and edit shim configurations without manual file editing.

Subcommands:
  list     Display all configured shims

Use "ribbin config <command> --help" for more information about a command.`,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configAddCmd)
	configCmd.AddCommand(configRemoveCmd)
	configCmd.AddCommand(configEditCmd)
}
