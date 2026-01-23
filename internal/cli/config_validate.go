package cli

import (
	"fmt"
	"os"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/spf13/cobra"
)

var configValidateCmd = &cobra.Command{
	Use:   "validate [path]",
	Short: "Validate a ribbin.jsonc config file",
	Long: `Validate a ribbin.jsonc config file against the JSON schema.

If no path is provided, validates the nearest ribbin.jsonc.

Exit codes:
  0 - Valid (may include warnings about unknown properties)
  1 - Invalid (schema validation failed)`,
	RunE: runConfigValidate,
}

func init() {
	configCmd.AddCommand(configValidateCmd)
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	var configPath string
	var err error

	if len(args) > 0 {
		configPath = args[0]
	} else {
		configPath, err = config.FindProjectConfig()
		if err != nil {
			return fmt.Errorf("failed to find config: %w", err)
		}
		if configPath == "" {
			return fmt.Errorf("no ribbin.jsonc found. Run 'ribbin init' to create one")
		}
	}

	fmt.Printf("Validating %s...\n", configPath)

	// Read the file
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Validate and get both errors and warnings
	errors, warnings := config.ValidateAgainstSchemaWithDetails(content)

	if len(errors) > 0 {
		fmt.Println("\u2717 Invalid")
		fmt.Println("\nErrors:")
		for _, e := range errors {
			fmt.Printf("  - %s\n", e)
		}
		os.Exit(1)
	}

	fmt.Println("\u2713 Valid")

	if len(warnings) > 0 {
		fmt.Println("\nUnknown properties (harmless, but not part of schema):")
		for _, w := range warnings {
			fmt.Printf("  - %s\n", w)
		}
	}

	return nil
}
