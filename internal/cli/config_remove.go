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
	configRemoveForce bool
)

var configRemoveCmd = &cobra.Command{
	Use:   "remove [config-path] <command>",
	Short: "Remove a wrapper configuration",
	Long: `Remove a wrapper configuration from a config file.

If no config path is provided, uses the nearest ribbin.jsonc or ribbin.local.jsonc.

Prompts for confirmation unless --force is used.

Examples:
  ribbin config remove tsc              Remove 'tsc' wrapper (with confirmation)
  ribbin config remove ./ribbin.jsonc tsc --force   Remove from specific config
  ribbin config remove npm --force      Remove 'npm' wrapper without confirmation`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runConfigRemove,
}

func init() {
	configRemoveCmd.Flags().BoolVar(&configRemoveForce, "force", false, "Skip confirmation prompt")
}

func runConfigRemove(cmd *cobra.Command, args []string) error {
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

	// Load and check configuration
	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Verify command exists in config
	shimCfg, exists := cfg.Wrappers[cmdName]
	if !exists {
		return fmt.Errorf("command '%s' is not configured", cmdName)
	}

	// Confirmation prompt (unless --force)
	if !configRemoveForce {
		// Show current configuration being removed
		fmt.Printf("Current configuration for '%s':\n", cmdName)
		fmt.Printf("  Action: %s\n", shimCfg.Action)
		if shimCfg.Message != "" {
			fmt.Printf("  Message: %s\n", shimCfg.Message)
		}
		if shimCfg.Redirect != "" {
			fmt.Printf("  Redirect: %s\n", shimCfg.Redirect)
		}
		if len(shimCfg.Paths) > 0 {
			fmt.Printf("  Paths: %s\n", strings.Join(shimCfg.Paths, ", "))
		}
		fmt.Println()

		// Prompt for confirmation
		fmt.Printf("Remove wrapper for '%s'? [y/N] ", cmdName)
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			return fmt.Errorf("operation cancelled")
		}
	}

	// Remove wrapper
	if err := config.RemoveShim(configPath, cmdName); err != nil {
		return fmt.Errorf("failed to remove wrapper: %w", err)
	}

	// Show success message
	fmt.Printf("Removed wrapper for '%s'\n", cmdName)
	fmt.Printf("Run 'ribbin unwrap' to remove the installed wrapper.\n")

	return nil
}
