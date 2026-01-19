package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/security"
	"github.com/happycollision/ribbin/internal/shim"
	"github.com/spf13/cobra"
)

var unshimAll bool
var unshimSearch bool

var unshimCmd = &cobra.Command{
	Use:   "unshim",
	Short: "Remove shims and restore original commands",
	Long: `Remove shims and restore original commands.

By default, this command removes shims only for commands listed in the
nearest ribbin.toml. Use flags to control which shims are removed.

For each shimmed command, ribbin:
  1. Removes the symlink at the command's path
  2. Renames <original>.ribbin-original back to <original>
  3. Updates the registry

Examples:
  ribbin unshim              # Remove shims listed in ribbin.toml
  ribbin unshim --all        # Remove all shims in the registry
  ribbin unshim --all --search  # Search filesystem for any orphaned shims`,
	RunE: runUnshim,
}

func init() {
	unshimCmd.Flags().BoolVar(&unshimAll, "all", false, "Remove all shims tracked in the registry, not just those in ribbin.toml")
	unshimCmd.Flags().BoolVar(&unshimSearch, "search", false, "Search common binary directories for shims (use with --all)")
}

// commonBinDirs returns common binary directories to search for shims.
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

func runUnshim(cmd *cobra.Command, args []string) error {
	// Load registry
	registry, err := config.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Determine paths to unshim based on flags
	var pathsToUnshim []string

	if unshimAll && unshimSearch {
		// Search common bin directories for sidecars
		binDirs, err := commonBinDirs()
		if err != nil {
			return fmt.Errorf("failed to get bin directories: %w", err)
		}
		sidecars, err := shim.FindSidecars(binDirs)
		if err != nil {
			return fmt.Errorf("failed to search for sidecars: %w", err)
		}
		// Convert sidecar paths to original paths (remove .ribbin-original suffix)
		for _, sidecar := range sidecars {
			originalPath := strings.TrimSuffix(sidecar, ".ribbin-original")
			pathsToUnshim = append(pathsToUnshim, originalPath)
		}
	} else if unshimAll {
		// Use paths from registry
		for _, entry := range registry.Shims {
			pathsToUnshim = append(pathsToUnshim, entry.Original)
		}
	} else {
		// Find nearest ribbin.toml and use commands from there
		configPath, err := config.FindProjectConfig()
		if err != nil {
			return fmt.Errorf("failed to find project config: %w", err)
		}
		if configPath == "" {
			return fmt.Errorf("no ribbin.toml found. Run 'ribbin init' to create one")
		}

		projectConfig, err := config.LoadProjectConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load project config: %w", err)
		}

		// For each command in project config, find its path in registry
		for commandName := range projectConfig.Shims {
			if entry, ok := registry.Shims[commandName]; ok {
				pathsToUnshim = append(pathsToUnshim, entry.Original)
			} else {
				// Try to find the command in PATH
				path, err := exec.LookPath(commandName)
				if err == nil {
					pathsToUnshim = append(pathsToUnshim, path)
				}
			}
		}
	}

	if len(pathsToUnshim) == 0 {
		fmt.Println("No shims to remove")
		return nil
	}

	// Track results
	var restored, skipped, failed int

	// Unshim each path
	for _, path := range pathsToUnshim {
		err := shim.Uninstall(path, registry)
		if err != nil {
			if strings.Contains(err.Error(), "sidecar not found") {
				fmt.Printf("Skipped %s: not shimmed\n", path)
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
