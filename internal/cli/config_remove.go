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
	Use:   "remove <command>",
	Short: "Remove a wrapper configuration",
	Long: `Remove a wrapper configuration from ribbin.jsonc.

Prompts for confirmation unless --force is used.

Examples:
  ribbin config remove tsc              Remove 'tsc' wrapper (with confirmation)
  ribbin config remove npm --force      Remove 'npm' wrapper without confirmation`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigRemove,
}

func init() {
	configRemoveCmd.Flags().BoolVar(&configRemoveForce, "force", false, "Skip confirmation prompt")
}

func runConfigRemove(cmd *cobra.Command, args []string) error {
	cmdName := args[0]

	// Find ribbin.jsonc
	configPath, err := config.FindProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to find config: %w", err)
	}

	if configPath == "" {
		return fmt.Errorf("ribbin.jsonc not found. Run 'ribbin init' first.")
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
