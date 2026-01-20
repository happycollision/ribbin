package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/security"
	"github.com/happycollision/ribbin/internal/wrap"
	"github.com/spf13/cobra"
)

var unwrapGlobal bool
var unwrapSearch bool

var unwrapCmd = &cobra.Command{
	Use:   "unwrap [config-files...]",
	Short: "Remove wrappers and restore original commands",
	Long: `Remove wrappers and restore original commands.

By default, removes wrappers only for commands listed in the nearest ribbin.toml.
You can also specify config file paths explicitly.

Use flags to control which wrappers are removed:
  --global       Remove all wrappers tracked in the registry
  --search       Search filesystem for orphaned wrappers (use with --global)

For each wrapped command, ribbin:
  1. Removes the symlink at the command's path
  2. Renames <original>.ribbin-original back to <original>
  3. Updates the registry

Examples:
  ribbin unwrap                         # Remove wrappers from nearest ribbin.toml
  ribbin unwrap ./a.toml ./b.toml       # Remove wrappers from specific configs
  ribbin unwrap --global                # Remove all wrappers in the registry
  ribbin unwrap --global --search       # Search filesystem for any orphaned wrappers`,
	RunE: runUnwrap,
}

func init() {
	unwrapCmd.Flags().BoolVar(&unwrapGlobal, "global", false, "Remove all wrappers tracked in the registry, not just those in ribbin.toml")
	unwrapCmd.Flags().BoolVar(&unwrapSearch, "search", false, "Search common binary directories for wrappers (use with --global)")
}

// commonBinDirs returns common binary directories to search for wrappers.
// It uses validated home directory paths to prevent environment variable injection.
func commonBinDirs() ([]string, error) {
	home, err := security.ValidateHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot get home directory: %w", err)
	}

	return []string{
		"/usr/bin",
		"/usr/local/bin",
		"/opt/homebrew/bin",
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, "go", "bin"),
	}, nil
}

func runUnwrap(cmd *cobra.Command, args []string) error {
	printGlobalWarningIfActive()

	// Load registry
	registry, err := config.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Determine paths to unwrap based on flags and args
	var pathsToUnwrap []string

	if unwrapGlobal && unwrapSearch {
		// Search common bin directories for sidecars
		binDirs, err := commonBinDirs()
		if err != nil {
			return fmt.Errorf("failed to get bin directories: %w", err)
		}
		sidecars, err := wrap.FindSidecars(binDirs)
		if err != nil {
			return fmt.Errorf("failed to search for sidecars: %w", err)
		}
		// Convert sidecar paths to original paths (remove .ribbin-original suffix)
		for _, sidecar := range sidecars {
			originalPath := strings.TrimSuffix(sidecar, ".ribbin-original")
			pathsToUnwrap = append(pathsToUnwrap, originalPath)
		}
	} else if unwrapGlobal {
		// Use paths from registry
		for _, entry := range registry.Wrappers {
			pathsToUnwrap = append(pathsToUnwrap, entry.Original)
		}
	} else {
		// Determine config files to process
		var configPaths []string
		if len(args) > 0 {
			// Use explicitly specified config files
			for _, arg := range args {
				absPath, err := filepath.Abs(arg)
				if err != nil {
					return fmt.Errorf("error resolving path %s: %w", arg, err)
				}
				configPaths = append(configPaths, absPath)
			}
		} else {
			// Find nearest ribbin.toml
			configPath, err := config.FindProjectConfig()
			if err != nil {
				return fmt.Errorf("failed to find project config: %w", err)
			}
			if configPath == "" {
				return fmt.Errorf("no ribbin.toml found. Run 'ribbin init' to create one")
			}
			configPaths = []string{configPath}
		}

		// Process each config file
		for _, configPath := range configPaths {
			projectConfig, err := config.LoadProjectConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load project config %s: %w", configPath, err)
			}

			// For each command in project config, find its path in registry
			for commandName := range projectConfig.Wrappers {
				if entry, ok := registry.Wrappers[commandName]; ok {
					pathsToUnwrap = append(pathsToUnwrap, entry.Original)
				} else {
					// Try to find the command in PATH and check if it has a sidecar
					path, err := exec.LookPath(commandName)
					if err == nil {
						// Only add if it looks like it was wrapped (has sidecar)
						if wrap.HasSidecar(path) {
							pathsToUnwrap = append(pathsToUnwrap, path)
						}
					}
				}
			}
		}
	}

	if len(pathsToUnwrap) == 0 {
		fmt.Println("No wrappers to remove")
		return nil
	}

	// Track results
	var restored, skipped, failed int

	// Unwrap each path
	for _, path := range pathsToUnwrap {
		err := wrap.Uninstall(path, registry)
		if err != nil {
			if strings.Contains(err.Error(), "sidecar not found") {
				fmt.Printf("Skipped %s: not wrapped\n", path)
				skipped++
			} else {
				fmt.Printf("Failed %s: %v\n", path, err)
				failed++
			}
		} else {
			fmt.Printf("Restored %s\n", path)
			restored++
		}
	}

	// Save registry
	if err := config.SaveRegistry(registry); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	// Print summary
	fmt.Printf("\nSummary: %d restored, %d skipped, %d failed\n", restored, skipped, failed)

	return nil
}
