package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize ribbin in the current directory",
	Long: `Create a ribbin.toml configuration file in the current directory.

The generated file contains commented examples showing how to configure
shims for common use cases like blocking tsc or npm.

After creating the config, edit it to add your shims, then run 'ribbin shim'
to install them.

Example:
  ribbin init
  # Edit ribbin.toml to configure your shims
  ribbin shim`,
	RunE: runInit,
}

const defaultConfig = `# ribbin configuration
# See https://github.com/happycollision/ribbin for documentation

# Example: Block direct tsc usage
# [shims.tsc]
# action = "block"
# message = "Use 'pnpm run typecheck' instead"

# Example: Block npm in pnpm projects
# [shims.npm]
# action = "block"
# message = "This project uses pnpm"
`

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	configPath := filepath.Join(cwd, "ribbin.toml")

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("ribbin.toml already exists in %s", cwd)
	}

	// Write the default config
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write ribbin.toml: %w", err)
	}

	fmt.Printf("Created %s\n", configPath)
	fmt.Println("\nEdit the file to add your shim configurations, then run 'ribbin shim' to install them.")

	return nil
}
