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
	Use:   "add [config-path] <command>",
	Short: "Add a new wrapper configuration",
	Long: `Add a new wrapper configuration to a config file.

If no config path is provided, uses the nearest ribbin.jsonc or ribbin.local.jsonc.

For block actions, specify --action block and optionally --message.
For redirect actions, specify --action redirect and --redirect (script path).

Examples:
  ribbin config add tsc --action block --message "Use pnpm typecheck"
  ribbin config add ./ribbin.jsonc tsc --action block --message "Use pnpm typecheck"
  ribbin config add npm --action redirect --redirect ./scripts/npm.sh
  ribbin config add curl --action block --message "Use the project API client" --paths /bin/curl,/usr/bin/curl`,
	Args: cobra.RangeArgs(1, 2),
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
	var configPath string
	var cmdName string
	var err error

	if len(args) == 2 {
		configPath = args[0]
		cmdName = args[1]
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("config file not found: %s", configPath)
		}
	} else {
		cmdName = args[0]
		configPath, err = config.FindProjectConfig()
		if err != nil {
			return fmt.Errorf("failed to find config: %w", err)
		}
		if configPath == "" {
			return fmt.Errorf("ribbin.jsonc not found. Run 'ribbin init' first.")
		}
	}

	// Validate action
	if addAction != "block" && addAction != "redirect" {
		return fmt.Errorf("action must be either 'block' or 'redirect'")
	}

	// Load existing config to check for duplicates
	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if command already exists
	if _, exists := cfg.Wrappers[cmdName]; exists {
		return fmt.Errorf("wrapper for command '%s' already exists. Use 'ribbin config edit' to modify it.", cmdName)
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

	// Build wrapper config from flags
	shimConfig := config.ShimConfig{
		Action:   addAction,
		Message:  addMessage,
		Redirect: addRedirect,
		Paths:    addPaths,
	}

	// Add wrapper to config
	if err := config.AddShim(configPath, cmdName, shimConfig); err != nil {
		return fmt.Errorf("failed to add wrapper: %w", err)
	}

	// Show success message
	fmt.Printf("Added wrapper for '%s'\n", cmdName)
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
	fmt.Printf("\nRun 'ribbin wrap' to install the wrapper.\n")

	return nil
}
