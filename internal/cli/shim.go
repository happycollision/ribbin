package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/security"
	"github.com/happycollision/ribbin/internal/wrap"
	"github.com/spf13/cobra"
)

var confirmSystemDir bool

var shimCmd = &cobra.Command{
	Use:   "shim",
	Short: "Install shims for commands in ribbin.toml",
	Long: `Install shims for all commands defined in the nearest ribbin.toml.

This command finds the ribbin.toml in the current directory (or parent
directories), then creates shims for each command listed in [wrappers.*] sections.

For each command, ribbin:
  1. Locates the original binary (via PATH or explicit paths in config)
  2. Renames it to <original>.ribbin-original
  3. Creates a symlink to ribbin in its place

When the shimmed command is later invoked, ribbin intercepts the call and
takes the configured action (block, warn, or redirect) or passes through to
the original binary.

Security:
  - Critical system binaries (bash, sudo, ssh) are never shimmed
  - User directories (~/.local/bin, ~/go/bin) and /usr/local/bin are allowed
  - System directories (/bin, /usr/bin, /sbin) require --confirm-system-dir flag

Examples:
  ribbin shim                            # Install shims for commands in ribbin.toml
  ribbin shim --confirm-system-dir       # Allow shimming in /bin, /usr/bin, etc.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Step 1: Check for Local Development Mode
		// When ribbin is installed as a dev dependency (inside a git repo),
		// it can only shim binaries within that same repository.
		localDevCtx, err := security.DetectLocalDevMode()
		if err != nil {
			// Log warning but continue - don't block on detection errors
			fmt.Fprintf(os.Stderr, "Warning: could not detect local dev mode: %v\n", err)
		}
		if localDevCtx != nil && localDevCtx.IsLocalDev {
			fmt.Printf("Local Development Mode active\n")
			fmt.Printf("  ribbin location: %s\n", localDevCtx.RibbinPath)
			fmt.Printf("  repository root: %s\n\n", localDevCtx.RepoRoot)
		}

		// Step 2: Find nearest ribbin.toml
		configPath, err := config.FindProjectConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding config: %v\n", err)
			os.Exit(1)
		}
		if configPath == "" {
			fmt.Fprintf(os.Stderr, "No ribbin.toml found. Run 'ribbin init' to create one.\n")
			os.Exit(1)
		}

		// Load project config
		projectConfig, err := config.LoadProjectConfig(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config %s: %v\n", configPath, err)
			os.Exit(1)
		}

		// Step 3: Load registry
		registry, err := config.LoadRegistry()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading registry: %v\n", err)
			os.Exit(1)
		}

		// Step 4: Get ribbin binary path
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

		// Step 5: Process each command in config.Shims
		var shimmed, skipped, failed int
		var refusedOutsideRepo []string

		for name, shimCfg := range projectConfig.Wrappers {
			var paths []string

			// If Paths is empty, resolve via wrap.ResolveCommand
			if len(shimCfg.Paths) == 0 {
				resolvedPath, err := wrap.ResolveCommand(name)
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

				// Check if path is a symlink and display information
				info, err := os.Lstat(path)
				if err != nil {
					fmt.Printf("Warning: cannot stat '%s': %v, skipping\n", path, err)
					continue
				}
				if info.Mode()&os.ModeSymlink != 0 {
					symlinkInfo, err := security.GetSymlinkInfo(path)
					if err != nil {
						fmt.Printf("⚠️  Skipping unsafe symlink '%s': %v\n", path, err)
						failed++
						continue
					}
					if symlinkInfo.ChainDepth > 0 {
						fmt.Printf("ℹ️  %s is a symlink ", filepath.Base(path))
						if symlinkInfo.ChainDepth > 1 {
							fmt.Printf("(depth %d) ", symlinkInfo.ChainDepth)
						}
						fmt.Printf("-> %s\n", symlinkInfo.FinalTarget)
					}
				}

				// Check Local Development Mode restrictions
				if localDevCtx != nil && localDevCtx.IsLocalDev {
					if err := localDevCtx.ValidateBinaryPath(path); err != nil {
						refusedOutsideRepo = append(refusedOutsideRepo, path)
						skipped++
						continue
					}
				}

				// Validate binary for shimming (security check)
				if err := security.ValidateBinaryForShim(path, confirmSystemDir); err != nil {
					fmt.Printf("Failed to shim '%s': %v\n", path, err)
					failed++
					continue
				}

				// Warn if in confirmation directory
				if security.RequiresConfirmation(path) && confirmSystemDir {
					fmt.Fprintf(os.Stderr, "WARNING: Shimming binary in system directory\n")
					fmt.Fprintf(os.Stderr, "   Path: %s\n", path)
					fmt.Fprintf(os.Stderr, "   This may affect all users on the system\n\n")
				}

				// Check if already shimmed
				alreadyShimmed, err := wrap.IsAlreadyShimmed(path)
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
				if err := wrap.Install(path, ribbinPath, registry, configPath); err != nil {
					fmt.Printf("Failed to shim '%s': %v\n", path, err)
					failed++
					continue
				}

				fmt.Printf("Shimmed '%s'\n", path)
				shimmed++
			}
		}

		// Step 6: Save registry
		if err := config.SaveRegistry(registry); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving registry: %v\n", err)
			os.Exit(1)
		}

		// Step 7: Report refused paths in Local Development Mode
		if len(refusedOutsideRepo) > 0 {
			fmt.Printf("\nRefusing to shim tools outside the repository:\n")
			for _, path := range refusedOutsideRepo {
				fmt.Printf("  - %s\n", path)
			}
		}

		// Step 8: Print summary
		fmt.Printf("\nSummary: %d shimmed, %d skipped, %d failed\n", shimmed, skipped, failed)
	},
}

func init() {
	shimCmd.Flags().BoolVar(&confirmSystemDir, "confirm-system-dir", false,
		"Allow shimming in system directories like /usr/local/bin (requires understanding security implications)")
}
