package cli

import (
	"bufio"
	"fmt"
	"os"
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
	var results []wrap.UnwrapResult

	// Unwrap each path
	for _, path := range pathsToUnwrap {
		result := unwrapSinglePath(path, registry)
		results = append(results, result)
	}

	// Save registry
	if err := config.SaveRegistry(registry); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	// Print summary
	printUnwrapSummary(results)

	return nil
}

// unwrapSinglePath handles unwrapping a single binary with conflict detection
func unwrapSinglePath(path string, registry *config.Registry) wrap.UnwrapResult {
	result := wrap.UnwrapResult{BinaryPath: path}

	// Check for hash conflict before unwrapping
	hasConflict, currentHash, originalHash := wrap.CheckHashConflict(path)
	if hasConflict {
		result.Conflict = true
		resolution := handleConflict(path, currentHash, originalHash, registry)
		result.Resolution = resolution

		switch resolution {
		case wrap.ResolutionSkipped:
			result.Success = true // Skipping is a valid successful outcome
			return result
		case wrap.ResolutionCleanup:
			// Remove sidecar files, keep current binary
			err := wrap.CleanupSidecarFiles(path, registry)
			if err != nil {
				result.Error = err
				result.Success = false
			} else {
				result.Success = true
			}
			return result
		case wrap.ResolutionRestored:
			// Fall through to normal restore
		}
	}

	// Normal unwrap
	err := wrap.Uninstall(path, registry)
	if err != nil {
		result.Error = err
		result.Success = false
	} else {
		result.Success = true
		if hasConflict {
			result.Resolution = wrap.ResolutionRestored
		}
	}

	return result
}

// handleConflict presents the conflict to the user and gets their resolution choice
func handleConflict(path, currentHash, originalHash string, registry *config.Registry) wrap.ConflictResolution {
	fmt.Printf("\n⚠️  Conflict detected for %s\n", path)
	fmt.Println("The tool appears to have been reinstalled since ribbin wrapped it.")
	fmt.Println()
	fmt.Println("For details on how ribbin wrapping works, see:")
	fmt.Println("  https://github.com/happycollision/ribbin#how-wrapping-works")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  1. Do nothing - leave current binary and ribbin sidecar files")
	fmt.Println("  2. Clean up   - remove sidecar files, keep current binary")
	fmt.Println("  3. Restore    - replace current binary with original from sidecar")
	fmt.Println()
	fmt.Print("Choose [1/2/3]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("\nError reading input, skipping this file")
		return wrap.ResolutionSkipped
	}

	input = strings.TrimSpace(input)
	switch input {
	case "1":
		fmt.Println("→ Skipping (no changes made)")
		return wrap.ResolutionSkipped
	case "2":
		fmt.Println("→ Cleaning up sidecar files")
		return wrap.ResolutionCleanup
	case "3":
		fmt.Println("→ Restoring original from sidecar")
		return wrap.ResolutionRestored
	default:
		fmt.Println("→ Invalid choice, skipping (no changes made)")
		return wrap.ResolutionSkipped
	}
}

// printUnwrapSummary prints a formatted summary of all unwrap operations
func printUnwrapSummary(results []wrap.UnwrapResult) {
	var restored, skipped, cleanedUp, failed []string
	var conflictResolutions []string

	for _, r := range results {
		if r.Success {
			if r.Conflict {
				switch r.Resolution {
				case wrap.ResolutionSkipped:
					skipped = append(skipped, r.BinaryPath)
					conflictResolutions = append(conflictResolutions,
						fmt.Sprintf("  %s → skipped (no changes made)", r.BinaryPath))
				case wrap.ResolutionCleanup:
					cleanedUp = append(cleanedUp, r.BinaryPath)
					conflictResolutions = append(conflictResolutions,
						fmt.Sprintf("  %s → cleaned up (kept current binary)", r.BinaryPath))
				case wrap.ResolutionRestored:
					restored = append(restored, r.BinaryPath)
					conflictResolutions = append(conflictResolutions,
						fmt.Sprintf("  %s → restored original", r.BinaryPath))
				}
			} else {
				restored = append(restored, r.BinaryPath)
			}
		} else {
			if r.Error != nil && strings.Contains(r.Error.Error(), "sidecar not found") {
				skipped = append(skipped, r.BinaryPath)
				fmt.Printf("Skipped %s: not wrapped\n", r.BinaryPath)
			} else {
				failed = append(failed, r.BinaryPath)
				fmt.Printf("Failed %s: %v\n", r.BinaryPath, r.Error)
			}
		}
	}

	// Print success messages for non-conflict restores
	for _, r := range results {
		if r.Success && !r.Conflict && r.Resolution == wrap.ResolutionNone {
			fmt.Printf("Restored %s\n", r.BinaryPath)
		}
	}

	fmt.Println()
	fmt.Println("Unwrap Summary")
	fmt.Println("==============")

	if len(restored) > 0 {
		fmt.Println()
		fmt.Println("✓ Restored:")
		for _, p := range restored {
			fmt.Printf("  %s\n", p)
		}
	}

	if len(conflictResolutions) > 0 {
		fmt.Println()
		fmt.Println("⚠️  Conflicts resolved:")
		for _, line := range conflictResolutions {
			fmt.Println(line)
		}
		fmt.Println()
		fmt.Println("For details on how ribbin wrapping works, see:")
		fmt.Println("  https://github.com/happycollision/ribbin#how-wrapping-works")
	}

	if len(failed) > 0 {
		fmt.Println()
		fmt.Println("✗ Failed:")
		for _, p := range failed {
			fmt.Printf("  %s\n", p)
		}
	}

	// Final counts
	fmt.Printf("\nTotal: %d restored, %d skipped, %d cleaned up, %d failed\n",
		len(restored), len(skipped), len(cleanedUp), len(failed))
}
