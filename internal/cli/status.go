package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show ribbin activation status",
	Long: `Show ribbin activation status.

Displays the current state of ribbin including:
  - Global activation status
  - Shell activation(s) with PIDs
  - Config activation(s) with paths
  - Wrapped tools and their mappings

Example:
  ribbin status`,
	Run: func(cmd *cobra.Command, args []string) {
		printGlobalWarningIfActive()

		// Load registry
		registry, err := config.LoadRegistry()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading registry: %v\n", err)
			os.Exit(1)
		}

		// Prune dead shell activations for accurate status
		registry.PruneDeadShellActivations()

		fmt.Println("Ribbin Status")
		fmt.Println("=============")
		fmt.Println()

		// Activation section
		fmt.Println("Activation:")

		// Global status
		if registry.GlobalActive {
			fmt.Println("  Global:  active")
		} else {
			fmt.Println("  Global:  inactive")
		}

		// Shell activations
		if len(registry.ShellActivations) == 0 {
			fmt.Println("  Shell:   inactive")
		} else {
			fmt.Printf("  Shell:   %d active\n", len(registry.ShellActivations))
			for pid, entry := range registry.ShellActivations {
				ago := formatTimeAgo(entry.ActivatedAt)
				fmt.Printf("    - PID %d (activated %s)\n", pid, ago)
			}
		}

		// Config activations
		if len(registry.ConfigActivations) == 0 {
			fmt.Println("  Configs: none active")
		} else {
			fmt.Printf("  Configs: %d active\n", len(registry.ConfigActivations))
			for path, entry := range registry.ConfigActivations {
				ago := formatTimeAgo(entry.ActivatedAt)
				fmt.Printf("    - %s (activated %s)\n", path, ago)
			}
		}

		// Wrapped tools section
		fmt.Println()
		fmt.Println("Wrapped Tools:")
		if len(registry.Wrappers) == 0 {
			fmt.Println("  (none)")
		} else {
			for name, entry := range registry.Wrappers {
				fmt.Printf("  %s -> %s\n", name, entry.Original)
				fmt.Printf("    (from %s)\n", entry.Config)
			}
		}
	},
}

// formatTimeAgo returns a human-readable string like "2h ago" or "15m ago"
func formatTimeAgo(t time.Time) string {
	d := time.Since(t)

	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		if minutes == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", minutes)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1d ago"
	}
	return fmt.Sprintf("%dd ago", days)
}
