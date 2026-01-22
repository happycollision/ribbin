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
	Long: `Create a ribbin.jsonc configuration file in the current directory.

The generated file contains commented examples showing how to configure
wrappers for common use cases.

Wrapper actions:
  block    - Display error message and exit
  warn     - Display warning but allow command to proceed
  redirect - Execute custom script instead of original command

For redirect actions, you specify a script path that receives the original
arguments and environment variables like RIBBIN_ORIGINAL_BIN, RIBBIN_COMMAND,
and RIBBIN_CONFIG. Relative paths resolve from ribbin.jsonc directory.

After creating the config, edit it to add your wrappers, then run 'ribbin wrap'
to install them.

Example:
  ribbin init
  # Edit ribbin.jsonc to configure your wrappers
  ribbin wrap`,
	RunE: runInit,
}

// LatestSchemaVersion is the current schema version used by ribbin init.
// Update this when releasing a new schema version.
const LatestSchemaVersion = "v1"

// LatestSchemaURL is the full URL to the latest schema version.
// Update the version in the path when releasing a new schema version.
const LatestSchemaURL = "https://github.com/happycollision/ribbin/schemas/" + LatestSchemaVersion + "/ribbin.schema.json"

const defaultConfig = `{
  "$schema": "` + LatestSchemaURL + `",

  // ribbin - Command wrapping tool
  // https://github.com/happycollision/ribbin
  //
  // ribbin intercepts calls to specified commands and can either block them with
  // helpful error messages or redirect them to alternative commands. This is useful for:
  //
  //   - Enforcing project conventions (e.g., "use pnpm, not npm")
  //   - Preventing AI agents from running commands directly
  //   - Redirecting to project-specific wrappers with correct config
  //
  // Quick start:
  //   1. Edit this file to configure your wrappers
  //   2. Run 'ribbin wrap' to install the wrappers
  //   3. Run 'ribbin activate --global' to enable everywhere
  //
  // Bypass: Set RIBBIN_BYPASS=1 to run the original command when needed.

  "wrappers": {
    // Example: Block direct tsc usage (use project's typecheck script instead)
    // "tsc": {
    //   "action": "block",
    //   "message": "Use 'pnpm run typecheck' instead - it uses the project's tsconfig"
    // },

    // Example: Enforce pnpm in this project
    // "npm": {
    //   "action": "block",
    //   "message": "This project uses pnpm. Run 'pnpm install' instead."
    // },

    // Example: Block direct calls but allow via pnpm scripts (passthrough)
    // "tsc": {
    //   "action": "block",
    //   "message": "Use 'pnpm run typecheck' instead",
    //   "passthrough": {
    //     "invocation": ["pnpm run typecheck", "pnpm run build"],
    //     "invocationRegexp": ["pnpm (typecheck|build)"]
    //   }
    // },

    // Example: Block curl, suggest project API client
    // "curl": {
    //   "action": "block",
    //   "message": "Use the project's API client at ./scripts/api.sh instead",
    //   "paths": ["/bin/curl", "/usr/bin/curl"]  // Optional: only block specific paths
    // },

    // Example: Custom wrapper script (relative path)
    // "node": {
    //   "action": "redirect",
    //   "redirect": "./scripts/node-wrapper.sh"
    // }
  }

  // Scopes: Different rules for different directories (great for monorepos)
  // "scopes": {
  //   // Frontend app: stricter rules
  //   "frontend": {
  //     "path": "apps/frontend",
  //     "extends": ["root"],  // Inherit root wrappers
  //     "wrappers": {
  //       "yarn": {
  //         "action": "block",
  //         "message": "Use pnpm in frontend"
  //       }
  //     }
  //   },
  //
  //   // Backend app: allow npm for legacy reasons
  //   "backend": {
  //     "path": "apps/backend",
  //     "extends": ["root"],
  //     "wrappers": {
  //       "npm": {
  //         "action": "passthrough"  // Override root's block
  //       }
  //     }
  //   },
  //
  //   // Mixin (no path): can only be extended by other scopes
  //   "hardened": {
  //     "wrappers": {
  //       "rm": {
  //         "action": "block",
  //         "message": "Use trash-cli instead"
  //       }
  //     }
  //   }
  // }
}
`

func runInit(cmd *cobra.Command, args []string) error {
	printGlobalWarningIfActive()

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	configPath := filepath.Join(cwd, "ribbin.jsonc")

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("ribbin.jsonc already exists in %s", cwd)
	}

	// Write the default config
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write ribbin.jsonc: %w", err)
	}

	fmt.Printf("Created %s\n", configPath)
	fmt.Println("\nEdit the file to add your wrapper configurations, then run 'ribbin wrap' to install them.")

	return nil
}
