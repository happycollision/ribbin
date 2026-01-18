package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dondenton/ribbin/internal/config"
	"github.com/dondenton/ribbin/internal/shim"
	"github.com/spf13/cobra"
)

var shimCmd = &cobra.Command{
	Use:   "shim",
	Short: "Create a shim for a command",
	Long:  "Create a shim that intercepts calls to a specific command",
	Run: func(cmd *cobra.Command, args []string) {
		// Step 1: Find nearest ribbin.json
		configPath, err := config.FindProjectConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding config: %v\n", err)
			os.Exit(1)
		}
		if configPath == "" {
			fmt.Fprintf(os.Stderr, "No ribbin.json found in current directory or any parent\n")
			os.Exit(1)
		}

		// Load project config
		projectConfig, err := config.LoadProjectConfig(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config %s: %v\n", configPath, err)
			os.Exit(1)
		}

		// Step 2: Load registry
		registry, err := config.LoadRegistry()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading registry: %v\n", err)
			os.Exit(1)
		}

		// Step 3: Get ribbin binary path
		execPath, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting executable path: %v\n", err)
			os.Exit(1)
		}
		ribbinPath, err := filepath.EvalSymlinks(execPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving executable path: %v\n", err)
			os.Exit(1)
		}

		// Step 4: Process each command in config.Shims
		var shimmed, skipped, failed int

		for name, shimCfg := range projectConfig.Shims {
			var paths []string

			// If Paths is empty, resolve via shim.ResolveCommand
			if len(shimCfg.Paths) == 0 {
				resolvedPath, err := shim.ResolveCommand(name)
				if err != nil {
					fmt.Printf("Warning: command '%s' not found in PATH, skipping\n", name)
					continue
				}
				paths = []string{resolvedPath}
			} else {
				paths = shimCfg.Paths
			}

			// Process each path
			for _, path := range paths {
				// Check if command exists at this path
				if _, err := os.Stat(path); os.IsNotExist(err) {
					fmt.Printf("Warning: path '%s' does not exist, skipping\n", path)
					continue
				}

				// Check if already shimmed
				alreadyShimmed, err := shim.IsAlreadyShimmed(path)
				if err != nil {
					fmt.Printf("Warning: could not check if '%s' is shimmed: %v\n", path, err)
					continue
				}
				if alreadyShimmed {
					fmt.Printf("Skipping '%s': already shimmed\n", path)
					skipped++
					continue
				}

				// Install shim
				if err := shim.Install(path, ribbinPath, registry, configPath); err != nil {
					fmt.Printf("Failed to shim '%s': %v\n", path, err)
					failed++
					continue
				}

				fmt.Printf("Shimmed '%s'\n", path)
				shimmed++
			}
		}

		// Step 5: Save registry
		if err := config.SaveRegistry(registry); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving registry: %v\n", err)
			os.Exit(1)
		}

		// Step 6: Print summary
		fmt.Printf("\nSummary: %d shimmed, %d skipped, %d failed\n", shimmed, skipped, failed)
	},
}
