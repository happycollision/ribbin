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
shims for common use cases.

Shim actions:
  block    - Display error message and exit
  warn     - Display warning but allow command to proceed
  redirect - Execute custom script instead of original command

For redirect actions, you specify a script path that receives the original
arguments and environment variables like RIBBIN_ORIGINAL_BIN, RIBBIN_COMMAND,
and RIBBIN_CONFIG. Relative paths resolve from ribbin.toml directory.

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
# ribbin intercepts calls to specified commands and can either block them with
# helpful error messages or redirect them to alternative commands. This is useful for:
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

# Example: Block curl, suggest project API client
# [shims.curl]
# action = "block"
# message = "Use the project's API client at ./scripts/api.sh instead"
# paths = ["/bin/curl", "/usr/bin/curl"]  # Optional: only block specific paths

# Example: Redirect npm to pnpm (absolute path)
# [shims.npm]
# action = "redirect"
# redirect = "/usr/local/bin/pnpm"
# message = "This project uses pnpm"

# Example: Custom wrapper script (relative path)
# [shims.node]
# action = "redirect"
# redirect = "./scripts/node-wrapper.sh"

# Example: Enforce TypeScript project config
# [shims.tsc]
# action = "redirect"
# redirect = "./scripts/typecheck.sh"
# message = "Using project-specific TypeScript configuration"
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
