package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/process"
	"github.com/spf13/cobra"
)

var activateCmd = &cobra.Command{
	Use:   "activate",
	Short: "Activate ribbin for the current shell",
	Long: `Activate ribbin for the current shell session.

This registers the parent shell's PID so that shims only trigger for
commands run from this shell (and its child processes). Use this for
per-session activation instead of global 'ribbin on'.

The activation is automatically cleaned up when the shell exits.

Example:
  ribbin activate   # Enable shims for this shell session only`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get parent shell PID
		shellPID := os.Getppid()

		// Verify shell process exists
		if !process.ProcessExists(shellPID) {
			fmt.Printf("Error: parent shell process (PID %d) does not exist\n", shellPID)
			return
		}

		// Load registry
		registry, err := config.LoadRegistry()
		if err != nil {
			fmt.Printf("Error loading registry: %v\n", err)
			return
		}

		// Check if already activated for this shell (idempotent)
		if _, exists := registry.Activations[shellPID]; exists {
			fmt.Printf("ribbin already activated for shell (PID %d)\n", shellPID)
			return
		}

		// Prune dead activations
		registry.PruneDeadActivations()

		// Add new activation entry
		registry.Activations[shellPID] = config.ActivationEntry{
			PID:         shellPID,
			ActivatedAt: time.Now(),
		}

		// Save registry
		if err := config.SaveRegistry(registry); err != nil {
			fmt.Printf("Error saving registry: %v\n", err)
			return
		}

		fmt.Printf("ribbin activated for shell (PID %d)\n", shellPID)
	},
}
