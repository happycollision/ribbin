package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/process"
	"github.com/spf13/cobra"
)

var activateConfig bool
var activateShell bool
var activateGlobal bool

var activateCmd = &cobra.Command{
	Use:   "activate [config-files...]",
	Short: "Activate ribbin for configs, shell, or globally",
	Long: `Activate ribbin interception.

By default (--config), activates the nearest ribbin.jsonc for all shells.
With --shell, activates all configs for the current shell only.
With --global, activates everything everywhere.

Scope flags (mutually exclusive):
  --config   Activate config(s) for all shells (DEFAULT)
  --shell    Activate all configs for current shell only
  --global   Activate everything everywhere

Examples:
  ribbin activate                        # Activate nearest config
  ribbin activate ./a.jsonc ./b.jsonc    # Activate specific configs
  ribbin activate --shell                # Activate for this shell
  ribbin activate --global               # Activate globally`,
	Run: func(cmd *cobra.Command, args []string) {
		printGlobalWarningIfActive()

		// Check for mutually exclusive flags
		flagCount := 0
		if activateConfig {
			flagCount++
		}
		if activateShell {
			flagCount++
		}
		if activateGlobal {
			flagCount++
		}
		if flagCount > 1 {
			fmt.Fprintf(os.Stderr, "Error: --config, --shell, and --global are mutually exclusive\n")
			os.Exit(1)
		}

		// Load registry
		registry, err := config.LoadRegistry()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading registry: %v\n", err)
			os.Exit(1)
		}

		// Determine activation mode (default is --config)
		if activateGlobal {
			// Global activation
			if registry.GlobalActive {
				fmt.Println("Ribbin is already globally active")
				return
			}
			registry.GlobalActive = true
			if err := config.SaveRegistry(registry); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving registry: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Ribbin is now globally active")
			return
		}

		if activateShell {
			// Shell activation
			shellPID := os.Getppid()

			// Verify shell process exists
			if !process.ProcessExists(shellPID) {
				fmt.Fprintf(os.Stderr, "Error: parent shell process (PID %d) does not exist\n", shellPID)
				os.Exit(1)
			}

			// Check if already activated for this shell (idempotent)
			if _, exists := registry.ShellActivations[shellPID]; exists {
				fmt.Printf("Ribbin already activated for shell (PID %d)\n", shellPID)
				return
			}

			// Prune dead shell activations
			registry.PruneDeadShellActivations()

			// Add new shell activation entry
			registry.AddShellActivation(shellPID)

			// Save registry
			if err := config.SaveRegistry(registry); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving registry: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Ribbin activated for shell (PID %d)\n", shellPID)
			return
		}

		// Config activation (default)
		var configPaths []string
		if len(args) > 0 {
			// Use explicitly specified config files
			for _, arg := range args {
				absPath, err := filepath.Abs(arg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error resolving path %s: %v\n", arg, err)
					os.Exit(1)
				}
				// Verify file exists
				if _, err := os.Stat(absPath); os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "Error: config file not found: %s\n", absPath)
					os.Exit(1)
				}
				configPaths = append(configPaths, absPath)
			}
		} else {
			// Find nearest ribbin.jsonc
			configPath, err := config.FindProjectConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error finding config: %v\n", err)
				os.Exit(1)
			}
			if configPath == "" {
				fmt.Fprintf(os.Stderr, "No ribbin.jsonc found. Run 'ribbin init' to create one.\n")
				os.Exit(1)
			}
			configPaths = []string{configPath}
		}

		// Activate each config
		activated := 0
		alreadyActive := 0
		for _, configPath := range configPaths {
			if _, exists := registry.ConfigActivations[configPath]; exists {
				fmt.Printf("Config already active: %s\n", configPath)
				alreadyActive++
				continue
			}
			registry.AddConfigActivation(configPath)
			fmt.Printf("Activated config: %s\n", configPath)
			activated++
		}

		// Save registry
		if err := config.SaveRegistry(registry); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving registry: %v\n", err)
			os.Exit(1)
		}

		if activated > 0 {
			fmt.Printf("\n%d config(s) activated", activated)
			if alreadyActive > 0 {
				fmt.Printf(", %d already active", alreadyActive)
			}
			fmt.Println()
		}
	},
}

func init() {
	activateCmd.Flags().BoolVar(&activateConfig, "config", false, "Activate config(s) for all shells (default if no flag specified)")
	activateCmd.Flags().BoolVar(&activateShell, "shell", false, "Activate all configs for current shell only")
	activateCmd.Flags().BoolVar(&activateGlobal, "global", false, "Activate everything everywhere")
}
