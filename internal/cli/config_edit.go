package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/spf13/cobra"
)

var (
	editAction        string
	editMessage       string
	editRedirect      string
	editPaths         []string
	editClearMessage  bool
	editClearPaths    bool
	editClearRedirect bool
)

var configEditCmd = &cobra.Command{
	Use:   "edit [config-path] <command>",
	Short: "Edit an existing wrapper configuration",
	Long: `Edit an existing wrapper configuration in a config file.

If no config path is provided, uses the nearest ribbin.jsonc or ribbin.local.jsonc.

Only specified fields are updated; others remain unchanged.

Examples:
  ribbin config edit tsc --message "Use 'bun run typecheck'"
  ribbin config edit ./ribbin.jsonc tsc --message "Use 'bun run typecheck'"
  ribbin config edit npm --action redirect --redirect /usr/local/bin/pnpm
  ribbin config edit node --redirect ./scripts/new-wrapper.sh
  ribbin config edit tsc --clear-message
  ribbin config edit cat --paths /bin/cat,/usr/bin/cat`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runConfigEdit,
}

func init() {
	configEditCmd.Flags().StringVar(&editAction, "action", "", "Change action type: \"block\" or \"redirect\"")
	configEditCmd.Flags().StringVar(&editMessage, "message", "", "Update message")
	configEditCmd.Flags().StringVar(&editRedirect, "redirect", "", "Update redirect script path")
	configEditCmd.Flags().StringSliceVar(&editPaths, "paths", nil, "Update binary path restrictions")
	configEditCmd.Flags().BoolVar(&editClearMessage, "clear-message", false, "Clear the message field")
	configEditCmd.Flags().BoolVar(&editClearPaths, "clear-paths", false, "Clear the paths restrictions")
	configEditCmd.Flags().BoolVar(&editClearRedirect, "clear-redirect", false, "Clear the redirect field (only if changing to block)")
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
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

	// Check if at least one flag is specified
	if !cmd.Flags().Changed("action") &&
		!cmd.Flags().Changed("message") &&
		!cmd.Flags().Changed("redirect") &&
		!cmd.Flags().Changed("paths") &&
		!editClearMessage &&
		!editClearPaths &&
		!editClearRedirect {
		return fmt.Errorf("no flags specified. Nothing to update.")
	}

	// Load existing configuration
	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Verify command exists
	existingShim, exists := cfg.Wrappers[cmdName]
	if !exists {
		return fmt.Errorf("command '%s' is not configured. Use 'ribbin config add' to create it.", cmdName)
	}

	// Build updated configuration starting from existing
	updatedShim := existingShim
	changes := make([]string, 0)

	// Apply updates from flags
	if cmd.Flags().Changed("action") {
		if editAction != "block" && editAction != "redirect" {
			return fmt.Errorf("action must be either 'block' or 'redirect'")
		}
		oldValue := existingShim.Action
		updatedShim.Action = editAction
		changes = append(changes, fmt.Sprintf("  action: %q -> %q", oldValue, editAction))
	}

	if cmd.Flags().Changed("message") {
		oldValue := existingShim.Message
		updatedShim.Message = editMessage
		if oldValue == "" {
			changes = append(changes, fmt.Sprintf("  message: (none) -> %q", editMessage))
		} else {
			changes = append(changes, fmt.Sprintf("  message: %q -> %q", oldValue, editMessage))
		}
	}

	if editClearMessage {
		if existingShim.Message != "" {
			changes = append(changes, fmt.Sprintf("  message: %q -> (cleared)", existingShim.Message))
		}
		updatedShim.Message = ""
	}

	if cmd.Flags().Changed("redirect") {
		oldValue := existingShim.Redirect
		updatedShim.Redirect = editRedirect
		if oldValue == "" {
			changes = append(changes, fmt.Sprintf("  redirect: (none) -> %q", editRedirect))
		} else {
			changes = append(changes, fmt.Sprintf("  redirect: %q -> %q", oldValue, editRedirect))
		}
	}

	if editClearRedirect {
		if updatedShim.Action == "redirect" {
			return fmt.Errorf("cannot clear redirect when action=redirect")
		}
		if existingShim.Redirect != "" {
			changes = append(changes, fmt.Sprintf("  redirect: %q -> (cleared)", existingShim.Redirect))
		}
		updatedShim.Redirect = ""
	}

	if cmd.Flags().Changed("paths") {
		oldValue := existingShim.Paths
		updatedShim.Paths = editPaths
		if len(oldValue) == 0 {
			changes = append(changes, fmt.Sprintf("  paths: (none) -> [%s]", strings.Join(editPaths, ", ")))
		} else {
			changes = append(changes, fmt.Sprintf("  paths: [%s] -> [%s]", strings.Join(oldValue, ", "), strings.Join(editPaths, ", ")))
		}
	}

	if editClearPaths {
		if len(existingShim.Paths) > 0 {
			changes = append(changes, fmt.Sprintf("  paths: [%s] -> (cleared)", strings.Join(existingShim.Paths, ", ")))
		}
		updatedShim.Paths = nil
	}

	// Validate: if action=redirect, redirect field must not be empty
	if updatedShim.Action == "redirect" && updatedShim.Redirect == "" {
		return fmt.Errorf("redirect field required when action=redirect")
	}

	// Update wrapper
	if err := config.UpdateShim(configPath, cmdName, updatedShim); err != nil {
		return fmt.Errorf("failed to update wrapper: %w", err)
	}

	// Show success message with summary of changes
	fmt.Printf("Updated wrapper for '%s':\n", cmdName)
	if len(changes) > 0 {
		for _, change := range changes {
			fmt.Println(change)
		}
	} else {
		fmt.Println("  (no changes)")
	}

	return nil
}
