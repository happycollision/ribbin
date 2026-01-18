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

const defaultConfig = `# ribbin - Command shimming tool
# https://github.com/happycollision/ribbin
#
# ribbin intercepts calls to specified commands and blocks them with helpful
# error messages. This is useful for:
#
#   - Enforcing project conventions (e.g., "use pnpm, not npm")
#   - Preventing AI agents from running commands directly
#   - Redirecting to project-specific wrappers with correct config
#
# Quick start:
#   1. Uncomment and edit the examples below
#   2. Run 'ribbin shim' to install the shims
#   3. Run 'ribbin on' to enable globally (or 'ribbin activate' for this shell)
#
# Bypass: Set RIBBIN_BYPASS=1 to run the original command when needed.

# Example: Block direct tsc usage (use project's typecheck script instead)
# [shims.tsc]
# action = "block"
# message = "Use 'pnpm run typecheck' instead - it uses the project's tsconfig"

# Example: Enforce pnpm in this project
# [shims.npm]
# action = "block"
# message = "This project uses pnpm. Run 'pnpm install' instead."

# Example: Block cat, suggest bat for syntax highlighting
# [shims.cat]
# action = "block"
# message = "Use 'bat' for syntax highlighting"
# paths = ["/bin/cat", "/usr/bin/cat"]  # Optional: only block specific paths
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
