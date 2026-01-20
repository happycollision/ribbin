package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/spf13/cobra"
)

var (
	addAction       string
	addMessage      string
	addRedirect     string
	addPaths        []string
	addCreateScript bool
)

var configAddCmd = &cobra.Command{
	Use:   "add <command>",
	Short: "Add a new shim configuration",
	Long: `Add a new shim configuration to ribbin.toml.

For block actions, specify --action block and optionally --message.
For redirect actions, specify --action redirect and --redirect (script path).

Examples:
  ribbin config add tsc --action block --message "Use pnpm typecheck"
  ribbin config add npm --action redirect --redirect ./scripts/npm.sh
  ribbin config add curl --action block --message "Use the project API client" --paths /bin/curl,/usr/bin/curl`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigAdd,
}

func init() {
	configAddCmd.Flags().StringVar(&addAction, "action", "", "Action type: \"block\" or \"redirect\" (required)")
	configAddCmd.Flags().StringVar(&addMessage, "message", "", "Message to display")
	configAddCmd.Flags().StringVar(&addRedirect, "redirect", "", "Script path for redirect action (required if action=redirect)")
	configAddCmd.Flags().StringSliceVar(&addPaths, "paths", nil, "Restrict to specific binary paths")
	configAddCmd.Flags().BoolVar(&addCreateScript, "create-script", true, "Create scaffold script if doesn't exist (default true for redirect)")

	configAddCmd.MarkFlagRequired("action")
}

func runConfigAdd(cmd *cobra.Command, args []string) error {
	cmdName := args[0]

	// Validate action
	if addAction != "block" && addAction != "redirect" {
		return fmt.Errorf("action must be either 'block' or 'redirect'")
	}

	// Find ribbin.toml
	configPath, err := config.FindProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to find config: %w", err)
	}

	if configPath == "" {
		return fmt.Errorf("ribbin.toml not found. Run 'ribbin init' first.")
	}

	// Load existing config to check for duplicates
	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if command already exists
	if _, exists := cfg.Wrappers[cmdName]; exists {
		return fmt.Errorf("shim for command '%s' already exists. Use 'ribbin config edit' to modify it.", cmdName)
	}

	// Validate redirect-specific requirements
	if addAction == "redirect" {
		if addRedirect == "" {
			return fmt.Errorf("--redirect is required when action=redirect")
		}

		// Handle script creation if enabled
		if addCreateScript {
			// Check if script already exists
			if _, err := os.Stat(addRedirect); os.IsNotExist(err) {
				// Prompt user to create script
				fmt.Printf("Create %s with template? [Y/n] ", addRedirect)
				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read input: %w", err)
				}

				response = strings.TrimSpace(strings.ToLower(response))
				if response == "" || response == "y" || response == "yes" {
					// Create scaffold script
					if err := GenerateRedirectScript(addRedirect, cmdName); err != nil {
						return fmt.Errorf("failed to create script: %w", err)
					}
					fmt.Printf("Created redirect script at %s\n", addRedirect)
					fmt.Printf("Edit this script to customize behavior before using.\n\n")
				}
			}
		}
	}

	// Build ShimConfig from flags
	shimConfig := config.ShimConfig{
		Action:   addAction,
		Message:  addMessage,
		Redirect: addRedirect,
		Paths:    addPaths,
	}

	// Add shim to config
	if err := config.AddShim(configPath, cmdName, shimConfig); err != nil {
		return fmt.Errorf("failed to add shim: %w", err)
	}

	// Show success message
	fmt.Printf("Added shim for '%s'\n", cmdName)
	fmt.Printf("  Action: %s\n", addAction)
	if addMessage != "" {
		fmt.Printf("  Message: %s\n", addMessage)
	}
	if addRedirect != "" {
		fmt.Printf("  Redirect: %s\n", addRedirect)
	}
	if len(addPaths) > 0 {
		fmt.Printf("  Paths: %s\n", strings.Join(addPaths, ", "))
	}
	fmt.Printf("\nRun 'ribbin shim %s' to install the shim.\n", cmdName)

	return nil
}
