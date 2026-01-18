package cli

import (
	"fmt"

	"github.com/dondenton/ribbin/internal/config"
	"github.com/spf13/cobra"
)

var onCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable ribbin shims",
	Long:  "Enable ribbin shims in the current shell session",
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
