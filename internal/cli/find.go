package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/spf13/cobra"
)

var findAll bool

var findCmd = &cobra.Command{
	Use:   "find [directory]",
	Short: "Find orphaned ribbin sidecars and config files",
	Long: `Find orphaned ribbin sidecars and unknown config files without removing them.

This command searches for:
  - .ribbin-original sidecar files (indicating wrapped binaries)
  - .ribbin-meta metadata files
  - ribbin.jsonc and ribbin.local.jsonc config files

By default, searches the current directory and subdirectories.
You can specify a different directory to search, or use --all to search the entire system.

This is useful for diagnosing ribbin state and finding orphaned wrappers that
may have been left behind from interrupted operations or manual file changes.

Examples:
  ribbin find                    # Search current directory recursively
  ribbin find /usr/local/bin     # Search specific directory
  ribbin find --all              # Search entire system (may be slow)`,
	RunE: runFind,
}

func init() {
	findCmd.Flags().BoolVar(&findAll, "all", false, "Search entire system instead of current directory")
}

func runFind(cmd *cobra.Command, args []string) error {
	printGlobalWarningIfActive()

	// Determine search root
	var searchRoot string
	if findAll {
		fmt.Println("⚠️  Searching your entire system for ribbin artifacts...")
		fmt.Println("This may take a while depending on your filesystem size.")
		fmt.Println()
		searchRoot = "/"
	} else if len(args) > 0 {
		// Use specified directory
		absPath, err := filepath.Abs(args[0])
		if err != nil {
			return fmt.Errorf("failed to resolve path %s: %w", args[0], err)
		}
		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("failed to access %s: %w", absPath, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("not a directory: %s", absPath)
		}
		searchRoot = absPath
		fmt.Printf("Searching %s for ribbin artifacts...\n\n", searchRoot)
	} else {
		// Use current directory
		var err error
		searchRoot, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		fmt.Printf("Searching %s for ribbin artifacts...\n\n", searchRoot)
	}

	// Load registry to compare against
	registry, err := config.LoadRegistry()
	if err != nil {
		fmt.Printf("Warning: failed to load registry: %v\n", err)
		fmt.Println("Continuing with search (registry comparison unavailable)")
		fmt.Println()
		registry = &config.Registry{Wrappers: make(map[string]config.WrapperEntry)}
	}

	// Track findings
	var sidecars []string
	var metadataFiles []string
	var configFiles []string
	var knownSidecars []string
	var unknownSidecars []string

	// Walk the directory tree
	err = filepath.Walk(searchRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip directories we can't access
			if os.IsPermission(err) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip .git directories
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Check if it's a ribbin artifact
		name := info.Name()

		if filepath.Ext(name) == ".ribbin-original" {
			sidecars = append(sidecars, path)

			// Check if this is tracked in registry
			originalPath := path[:len(path)-len(".ribbin-original")]
			isKnown := false
			for _, entry := range registry.Wrappers {
				if entry.Original == originalPath {
					isKnown = true
					break
				}
			}

			if isKnown {
				knownSidecars = append(knownSidecars, path)
			} else {
				unknownSidecars = append(unknownSidecars, path)
			}
		} else if filepath.Ext(name) == ".ribbin-meta" {
			metadataFiles = append(metadataFiles, path)
		} else if name == "ribbin.jsonc" || name == "ribbin.local.jsonc" {
			configFiles = append(configFiles, path)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error during search: %w", err)
	}

	// Add unknown/orphaned sidecars to the registry so we don't have to search again
	if len(unknownSidecars) > 0 {
		for _, sidecar := range unknownSidecars {
			originalPath := sidecar[:len(sidecar)-len(".ribbin-original")]
			commandName := filepath.Base(originalPath)

			// Add to registry with empty config to mark as "discovered orphan"
			registry.Wrappers[commandName] = config.WrapperEntry{
				Original: originalPath,
				Config:   "(discovered orphan)", // Mark as discovered, not from a config file
			}
		}

		// Save the updated registry
		if err := config.SaveRegistry(registry); err != nil {
			fmt.Printf("Warning: failed to save registry: %v\n", err)
		} else {
			fmt.Printf("\nAdded %d orphaned sidecar(s) to registry for tracking.\n", len(unknownSidecars))
		}
	}

	// Print results
	printFindResults(sidecars, metadataFiles, configFiles, knownSidecars, unknownSidecars)

	return nil
}

// searchForSidecars walks a directory tree and finds all .ribbin-original files
func searchForSidecars(searchRoot string) ([]string, error) {
	var sidecars []string

	err := filepath.Walk(searchRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip directories we can't access
			if os.IsPermission(err) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip .git directories
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Check if it's a ribbin sidecar
		if filepath.Ext(info.Name()) == ".ribbin-original" {
			sidecars = append(sidecars, path)
		}

		return nil
	})

	return sidecars, err
}

func printFindResults(sidecars, metadataFiles, configFiles, knownSidecars, unknownSidecars []string) {
	fmt.Println("Search Results")
	fmt.Println("==============")
	fmt.Println()

	if len(sidecars) == 0 && len(metadataFiles) == 0 && len(configFiles) == 0 {
		fmt.Println("No ribbin artifacts found.")
		return
	}

	if len(configFiles) > 0 {
		fmt.Println("Config Files:")
		for _, path := range configFiles {
			fmt.Printf("  %s\n", path)
		}
		fmt.Println()
	}

	if len(knownSidecars) > 0 {
		fmt.Println("✓ Known Wrapped Binaries (tracked in registry):")
		for _, path := range knownSidecars {
			originalPath := path[:len(path)-len(".ribbin-original")]
			fmt.Printf("  %s\n", originalPath)
		}
		fmt.Println()
	}

	if len(unknownSidecars) > 0 {
		fmt.Println("⚠️  Unknown/Orphaned Wrapped Binaries (NOT in registry):")
		for _, path := range unknownSidecars {
			originalPath := path[:len(path)-len(".ribbin-original")]
			fmt.Printf("  %s\n", originalPath)
		}
		fmt.Println()
		fmt.Println("These sidecars may be orphaned from interrupted operations.")
		fmt.Println("To clean them up, run:")
		fmt.Println("  ribbin unwrap --all --find")
		fmt.Println()
	}

	if len(metadataFiles) > 0 {
		fmt.Println("Metadata Files:")
		for _, path := range metadataFiles {
			fmt.Printf("  %s\n", path)
		}
		fmt.Println()
	}

	// Summary
	fmt.Printf("Summary: %d config(s), %d wrapped binary(ies) (%d known, %d orphaned), %d metadata file(s)\n",
		len(configFiles), len(sidecars), len(knownSidecars), len(unknownSidecars), len(metadataFiles))
}
