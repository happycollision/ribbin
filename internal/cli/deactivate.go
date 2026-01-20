package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/spf13/cobra"
)

var deactivateConfig bool
var deactivateShell bool
var deactivateGlobal bool
var deactivateAll bool
var deactivateEverything bool

var deactivateCmd = &cobra.Command{
	Use:   "deactivate [config-files...]",
	Short: "Deactivate ribbin for configs, shell, or globally",
	Long: `Deactivate ribbin interception.

Scope flags (mutually exclusive):
  --config   Deactivate config(s) - DEFAULT
  --shell    Deactivate shell activation(s)
  --global   Turn off global mode

Modifier flags:
  --all         Deactivate ALL items in the chosen scope
  --everything  Nuclear option: deactivate global + all shells + all configs

Examples:
  ribbin deactivate                    # Deactivate nearest config
  ribbin deactivate ./a.toml ./b.toml  # Deactivate specific configs
  ribbin deactivate --config --all     # Deactivate ALL configs
  ribbin deactivate --all              # Same as --config --all
  ribbin deactivate --shell            # Deactivate current shell
  ribbin deactivate --shell --all      # Deactivate ALL shells
  ribbin deactivate --global           # Turn off global mode
  ribbin deactivate --everything       # Nuclear: global + all shells + all configs`,
	Run: func(cmd *cobra.Command, args []string) {
		printGlobalWarningIfActive()

		// Handle --everything first (ignores other flags)
		if deactivateEverything {
			runDeactivateEverything()
			return
		}

		// Check for mutually exclusive scope flags
		scopeCount := 0
		if deactivateConfig {
			scopeCount++
		}
		if deactivateShell {
			scopeCount++
		}
		if deactivateGlobal {
			scopeCount++
		}
		if scopeCount > 1 {
			fmt.Fprintf(os.Stderr, "Error: --config, --shell, and --global are mutually exclusive\n")
			os.Exit(1)
		}

		// Load registry
		registry, err := config.LoadRegistry()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading registry: %v\n", err)
			os.Exit(1)
		}

		// Determine mode
		if deactivateGlobal {
			// Turn off global mode
			if !registry.GlobalActive {
				fmt.Println("Global mode is already inactive")
				return
			}
			registry.GlobalActive = false
			if err := config.SaveRegistry(registry); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving registry: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Global mode deactivated")
			return
		}

		if deactivateShell {
			// Shell deactivation
			if deactivateAll {
				// Deactivate all shells
				count := len(registry.ShellActivations)
				if count == 0 {
					fmt.Println("No active shell activations")
					return
				}
				registry.ClearShellActivations()
				if err := config.SaveRegistry(registry); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving registry: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Deactivated %d shell activation(s)\n", count)
				return
			}

			// Deactivate current shell only
			shellPID := os.Getppid()
			if _, exists := registry.ShellActivations[shellPID]; !exists {
				fmt.Printf("Shell (PID %d) is not activated\n", shellPID)
				return
			}
			registry.RemoveShellActivation(shellPID)
			if err := config.SaveRegistry(registry); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving registry: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Deactivated shell (PID %d)\n", shellPID)
			return
		}

		// Config deactivation (default scope)
		if deactivateAll {
			// Deactivate all configs
			count := len(registry.ConfigActivations)
			if count == 0 {
				fmt.Println("No active config activations")
				return
			}
			registry.ClearConfigActivations()
			if err := config.SaveRegistry(registry); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving registry: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Deactivated %d config(s)\n", count)
			return
		}

		// Deactivate specific configs
		var configPaths []string
		if len(args) > 0 {
			// Use explicitly specified config files
			for _, arg := range args {
				absPath, err := filepath.Abs(arg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error resolving path %s: %v\n", arg, err)
					os.Exit(1)
				}
				configPaths = append(configPaths, absPath)
			}
		} else {
			// Find nearest ribbin.toml
			configPath, err := config.FindProjectConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error finding config: %v\n", err)
				os.Exit(1)
			}
			if configPath == "" {
				fmt.Fprintf(os.Stderr, "No ribbin.toml found. Run 'ribbin init' to create one.\n")
				os.Exit(1)
			}
			configPaths = []string{configPath}
		}

		// Deactivate each config
		deactivated := 0
		notActive := 0
		for _, configPath := range configPaths {
			if _, exists := registry.ConfigActivations[configPath]; !exists {
				fmt.Printf("Config not active: %s\n", configPath)
				notActive++
				continue
			}
			registry.RemoveConfigActivation(configPath)
			fmt.Printf("Deactivated config: %s\n", configPath)
			deactivated++
		}

		// Save registry
		if err := config.SaveRegistry(registry); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving registry: %v\n", err)
			os.Exit(1)
		}

		if deactivated > 0 {
			fmt.Printf("\n%d config(s) deactivated", deactivated)
			if notActive > 0 {
				fmt.Printf(", %d not active", notActive)
			}
			fmt.Println()
		}
	},
}

func runDeactivateEverything() {
	// Load registry
	registry, err := config.LoadRegistry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading registry: %v\n", err)
		os.Exit(1)
	}

	// Track what was deactivated
	globalWasActive := registry.GlobalActive
	shellCount := len(registry.ShellActivations)
	configCount := len(registry.ConfigActivations)

	// Nuclear option: clear everything
	registry.GlobalActive = false
	registry.ClearShellActivations()
	registry.ClearConfigActivations()

	// Save registry
	if err := config.SaveRegistry(registry); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving registry: %v\n", err)
		os.Exit(1)
	}

	// Report what was cleared
	fmt.Println("Deactivated everything:")
	if globalWasActive {
		fmt.Println("  - Global mode: deactivated")
	} else {
		fmt.Println("  - Global mode: was already inactive")
	}
	if shellCount > 0 {
		fmt.Printf("  - Shell activations: %d cleared\n", shellCount)
	} else {
		fmt.Println("  - Shell activations: none active")
	}
	if configCount > 0 {
		fmt.Printf("  - Config activations: %d cleared\n", configCount)
	} else {
		fmt.Println("  - Config activations: none active")
	}
}

func init() {
	deactivateCmd.Flags().BoolVar(&deactivateConfig, "config", false, "Deactivate config(s) (default if no scope flag specified)")
	deactivateCmd.Flags().BoolVar(&deactivateShell, "shell", false, "Deactivate shell activation(s)")
	deactivateCmd.Flags().BoolVar(&deactivateGlobal, "global", false, "Turn off global mode")
	deactivateCmd.Flags().BoolVar(&deactivateAll, "all", false, "Deactivate ALL items in the chosen scope")
	deactivateCmd.Flags().BoolVar(&deactivateEverything, "everything", false, "Nuclear option: deactivate global + all shells + all configs")
}
