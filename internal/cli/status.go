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

		// Wrapped tools section - separate known from discovered orphans
		fmt.Println()
		fmt.Println("Wrapped Tools:")

		var knownWrappers []config.WrapperEntry
		var discoveredOrphans []config.WrapperEntry

		for _, entry := range registry.Wrappers {
			if entry.Config == "(discovered orphan)" {
				discoveredOrphans = append(discoveredOrphans, entry)
			} else {
				knownWrappers = append(knownWrappers, entry)
			}
		}

		if len(knownWrappers) == 0 && len(discoveredOrphans) == 0 {
			fmt.Println("  (none)")
		} else {
			if len(knownWrappers) > 0 {
				fmt.Println("  Known wrappers:")
				for _, entry := range knownWrappers {
					fmt.Printf("    %s\n", entry.Original)
					fmt.Printf("      (from %s)\n", entry.Config)
				}
			}

			if len(discoveredOrphans) > 0 {
				if len(knownWrappers) > 0 {
					fmt.Println()
				}
				fmt.Printf("  ⚠️  Discovered orphans (%d):\n", len(discoveredOrphans))
				for _, entry := range discoveredOrphans {
					fmt.Printf("    %s\n", entry.Original)
				}
				fmt.Println()
				fmt.Println("  These were found by 'ribbin find' but not created by a config file.")
				fmt.Println("  To clean them up, run:")
				fmt.Println("    ribbin unwrap --all")
			}
		}

		fmt.Println()
		fmt.Println("💡 Tip: Run 'ribbin find --all' to search your entire system for unknown sidecars.")
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
