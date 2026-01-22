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

var wrapCmd = &cobra.Command{
	Use:   "wrap [config-files...]",
	Short: "Install wrappers for commands in ribbin.jsonc",
	Long: `Install wrappers for all commands defined in the specified ribbin.jsonc files.

By default, wraps commands from the nearest ribbin.jsonc in the current directory
(or parent directories). You can also specify config file paths explicitly.

For each command, ribbin:
  1. Locates the original binary (via PATH or explicit paths in config)
  2. Renames it to <original>.ribbin-original
  3. Creates a symlink to ribbin in its place

When the wrapped command is later invoked, ribbin intercepts the call and
takes the configured action (block, warn, or redirect) or passes through to
the original binary.

Security:
  - Critical system binaries (bash, sudo, ssh) are never wrapped
  - System directories (/bin, /usr/bin, /sbin) require --confirm-system-dir flag
  - All other directories are allowed by default

Examples:
  ribbin wrap                            # Wrap commands from nearest ribbin.jsonc
  ribbin wrap ./a.jsonc ./b.jsonc        # Wrap commands from specific configs
  ribbin wrap --confirm-system-dir       # Allow wrapping in /bin, /usr/bin, etc.`,
	Run: func(cmd *cobra.Command, args []string) {
		printGlobalWarningIfActive()

		// Step 1: Check for Local Development Mode
		// When ribbin is installed as a dev dependency (inside a git repo),
		// it can only wrap binaries within that same repository.
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

		// Step 2: Determine config files to process
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

		// Step 5: Process each config file
		var wrapped, skipped, failed int
		var refusedOutsideRepo []string

		for _, configPath := range configPaths {
			// Load project config
			projectConfig, err := config.LoadProjectConfig(configPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading config %s: %v\n", configPath, err)
				os.Exit(1)
			}

			if len(configPaths) > 1 {
				fmt.Printf("Processing %s...\n", configPath)
			}

			// Collect all wrappers from root and scopes
			allWrappers := make(map[string]config.WrapperConfig)

			// Add root-level wrappers
			for name, wrapperCfg := range projectConfig.Wrappers {
				allWrappers[name] = wrapperCfg
			}

			// Add wrappers from all scopes
			for scopeName, scopeCfg := range projectConfig.Scopes {
				for name, wrapperCfg := range scopeCfg.Wrappers {
					// If a wrapper with this name already exists, we could warn or skip
					// For now, scope wrappers override root wrappers
					if _, exists := allWrappers[name]; exists {
						fmt.Printf("Note: scope '%s' overrides wrapper for '%s'\n", scopeName, name)
					}
					allWrappers[name] = wrapperCfg
				}
			}

			for name, wrapperCfg := range allWrappers {
				var paths []string

				// If Paths is empty, resolve via wrap.ResolveCommand
				if len(wrapperCfg.Paths) == 0 {
					resolvedPath, err := wrap.ResolveCommand(name)
					if err != nil {
						fmt.Printf("Warning: command '%s' not found in PATH, skipping\n", name)
						continue
					}
					paths = []string{resolvedPath}
				} else {
					// Resolve relative paths relative to the config file's directory
					configDir := filepath.Dir(configPath)
					for _, p := range wrapperCfg.Paths {
						if filepath.IsAbs(p) {
							paths = append(paths, p)
						} else {
							absPath := filepath.Join(configDir, p)
							// Clean the path to resolve any . or .. components
							paths = append(paths, filepath.Clean(absPath))
						}
					}
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
							fmt.Printf("Skipping unsafe symlink '%s': %v\n", path, err)
							failed++
							continue
						}
						if symlinkInfo.ChainDepth > 0 {
							fmt.Printf("%s is a symlink ", filepath.Base(path))
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

					// Validate binary for wrapping (security check)
					if err := security.ValidateBinaryForShim(path, confirmSystemDir); err != nil {
						fmt.Printf("Failed to wrap '%s': %v\n", path, err)
						failed++
						continue
					}

					// Warn if in confirmation directory
					if security.RequiresConfirmation(path) && confirmSystemDir {
						fmt.Fprintf(os.Stderr, "WARNING: Wrapping binary in system directory\n")
						fmt.Fprintf(os.Stderr, "   Path: %s\n", path)
						fmt.Fprintf(os.Stderr, "   This may affect all users on the system\n\n")
					}

					// Check if already wrapped
					alreadyWrapped, err := wrap.IsAlreadyShimmed(path)
					if err != nil {
						fmt.Printf("Warning: could not check if '%s' is wrapped: %v\n", path, err)
						continue
					}
					if alreadyWrapped {
						fmt.Printf("Skipping '%s': already wrapped\n", path)
						skipped++
						continue
					}

					// Install wrapper
					if err := wrap.Install(path, ribbinPath, registry, configPath); err != nil {
						fmt.Printf("Failed to wrap '%s': %v\n", path, err)
						failed++
						continue
					}

					fmt.Printf("Wrapped '%s'\n", path)
					wrapped++
				}
			}
		}

		// Step 6: Save registry
		if err := config.SaveRegistry(registry); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving registry: %v\n", err)
			os.Exit(1)
		}

		// Step 7: Report refused paths in Local Development Mode
		if len(refusedOutsideRepo) > 0 {
			fmt.Printf("\nRefusing to wrap tools outside the repository:\n")
			for _, path := range refusedOutsideRepo {
				fmt.Printf("  - %s\n", path)
			}
		}

		// Step 8: Print summary
		fmt.Printf("\nSummary: %d wrapped, %d skipped, %d failed\n", wrapped, skipped, failed)

		// Step 9: Print warning about unwrapping before uninstall
		if wrapped > 0 {
			fmt.Fprintf(os.Stderr, "\nIMPORTANT: Run 'ribbin unwrap --global --search' (or 'ribbin recover')\n")
			fmt.Fprintf(os.Stderr, "before uninstalling ribbin. Failure to do so will result in recoverable,\n")
			fmt.Fprintf(os.Stderr, "but temporarily broken tools. See https://github.com/happycollision/ribbin#recovery\n")
		}
	},
}

func init() {
	wrapCmd.Flags().BoolVar(&confirmSystemDir, "confirm-system-dir", false,
		"Allow wrapping in system directories like /usr/local/bin (requires understanding security implications)")
}
