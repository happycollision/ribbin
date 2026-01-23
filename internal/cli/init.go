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

  // ribbin - Intercept commands and redirect to project-approved alternatives
  //
  // Config reference: https://github.com/happycollision/ribbin/blob/main/docs/reference/config-schema.md
  //
  // Common use cases:
  //   Block commands:      https://github.com/happycollision/ribbin/blob/main/docs/how-to/block-commands.md
  //   Redirect commands:   https://github.com/happycollision/ribbin/blob/main/docs/how-to/redirect-commands.md
  //   AI agent guardrails: https://github.com/happycollision/ribbin/blob/main/docs/how-to/integrate-ai-agents.md
  //
  // Run 'ribbin config --example' to see a comprehensive example config.

  "wrappers": {
    // "npm": {
    //   "action": "block",
    //   "message": "This project uses pnpm"
    // }
  }

  // Scopes: Apply different rules to different directories.
  // Great for monorepos, or defining config outside the project entirely.
  //   Monorepo guide:     https://github.com/happycollision/ribbin/blob/main/docs/how-to/monorepo-scopes.md
  //   External config:    https://github.com/happycollision/ribbin/blob/main/docs/how-to/external-config.md
  // "scopes": {}
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
	fmt.Println("Run 'ribbin config --example' to see a comprehensive example config.")

	return nil
}
