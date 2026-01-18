package cli

import (
	"fmt"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/spf13/cobra"
)

var offCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable ribbin shims globally",
	Long: `Disable ribbin shims globally for all shells.

When disabled globally, shimmed commands will pass through to the original
binaries without interception (unless a specific shell has been activated
with 'ribbin activate').

Example:
  ribbin off   # Disable shims globally`,
	Run: func(cmd *cobra.Command, args []string) {
		registry, err := config.LoadRegistry()
		if err != nil {
			fmt.Printf("Error loading registry: %v\n", err)
			return
		}

		if !registry.GlobalOn {
			fmt.Println("Shims are already globally disabled")
			return
		}

		registry.GlobalOn = false

		if err := config.SaveRegistry(registry); err != nil {
			fmt.Printf("Error saving registry: %v\n", err)
			return
		}

		fmt.Println("✓ Shims are now globally disabled")
	},
}
