package cli

import (
	"github.com/spf13/cobra"
)

var recoverCmd = &cobra.Command{
	Use:   "recover",
	Short: "Recover wrapped binaries (alias for unwrap --global --search)",
	Long: `Recover all wrapped binaries by searching common binary directories.

This is an alias for 'ribbin unwrap --global --search'. Use this command
to restore all original binaries before uninstalling ribbin, or to recover
from a situation where wrapped binaries were orphaned.

The command searches common binary directories:
  - /usr/bin
  - /usr/local/bin
  - /opt/homebrew/bin
  - ~/.local/bin
  - ~/go/bin

Example:
  ribbin recover   # Find and restore all wrapped binaries`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set the flags for unwrap command
		unwrapGlobal = true
		unwrapSearch = true

		// Run the unwrap command
		return runUnwrap(cmd, args)
	},
}
