package cli

import (
	"fmt"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/spf13/cobra"
)

var onCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable ribbin shims globally",
	Long: `Enable ribbin shims globally for all shells.

When enabled globally, all shimmed commands will be intercepted regardless
of which shell they are run from. This is useful when you always want
protection enabled.

For per-shell activation, use 'ribbin activate' instead.

Example:
  ribbin on    # Enable shims globally`,
	Run: func(cmd *cobra.Command, args []string) {
		registry, err := config.LoadRegistry()
		if err != nil {
			fmt.Printf("Error loading registry: %v\n", err)
			return
		}

		if registry.GlobalOn {
			fmt.Println("Shims are already globally enabled")
			return
		}

		registry.GlobalOn = true

		if err := config.SaveRegistry(registry); err != nil {
			fmt.Printf("Error saving registry: %v\n", err)
			return
		}

		fmt.Println("✓ Shims are now globally enabled")
	},
}
