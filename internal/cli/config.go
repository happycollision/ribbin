package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var exampleFlag bool

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage ribbin.jsonc configuration",
	Long: `Add, remove, list, edit, and show wrapper configurations without manual file editing.

Flags:
  --example  Print comprehensive example config to stdout

Subcommands:
  list     Display all configured wrappers
  show     Show effective configuration with provenance tracking

Use "ribbin config <command> --help" for more information about a command.`,
	RunE: runConfig,
}

// ExampleConfig is a comprehensive example configuration for reference.
// This is exported so it can be validated in tests.
const ExampleConfig = `{
  "$schema": "` + LatestSchemaURL + `",

  // This example config demonstrates ribbin's key features.
  // Each section shows a different capability.

  "wrappers": {
    // Simple block - guide users to the right command
    "npm": {
      "action": "block",
      "message": "This project uses pnpm. Run 'pnpm install' instead."
    },

    // Block with passthrough - allow when run via approved scripts
    // Passthrough checks ancestor processes, not just the immediate parent.
    //
    // Regex syntax: https://pkg.go.dev/regexp/syntax
    // Test patterns: https://regex101.com/r/zFOgaB/1
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead",
      "paths": ["./node_modules/.bin/tsc"],
      "passthrough": {
        "invocation": ["pnpm run typecheck", "pnpm run build"],
        "invocationRegexp": ["make (test|build)"]
      }
    },

    // Redirect - run a wrapper script instead of the original
    "node": {
      "action": "redirect",
      "redirect": "./scripts/node-wrapper.sh",
      "message": "Using project's Node wrapper"
    },

    // Warn - show warning but allow command to proceed
    "curl": {
      "action": "warn",
      "message": "Consider using the project's API client at ./scripts/api.sh"
    }
  },

  // Scopes - different rules for different directories
  // Great for monorepos, or defining config outside the project entirely
  "scopes": {
    // Mixin (no path) - reusable rules that other scopes can extend
    "strict": {
      "wrappers": {
        "rm": {
          "action": "block",
          "message": "Use trash-cli for safe deletion"
        }
      }
    },

    // Frontend scope - stricter rules, inherits from root and mixin
    "frontend": {
      "path": "apps/frontend",
      "extends": ["root", "root.strict"],
      "wrappers": {
        "yarn": {
          "action": "block",
          "message": "Use pnpm in frontend"
        }
      }
    },

    // Backend scope - overrides root's npm block
    "backend": {
      "path": "apps/backend",
      "extends": ["root"],
      "wrappers": {
        "npm": {
          "action": "passthrough"
        }
      }
    }
  }
}
`

func runConfig(cmd *cobra.Command, args []string) error {
	if exampleFlag {
		fmt.Print(ExampleConfig)
		return nil
	}
	return cmd.Help()
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().BoolVarP(&exampleFlag, "example", "e", false, "Print comprehensive example config to stdout")
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configAddCmd)
	configCmd.AddCommand(configRemoveCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configShowCmd)
}
