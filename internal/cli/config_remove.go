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
	Short: "Remove a shim configuration",
	Long: `Remove a shim configuration from ribbin.toml.

Prompts for confirmation unless --force is used.

Examples:
  ribbin config remove tsc              Remove 'tsc' shim (with confirmation)
  ribbin config remove npm --force      Remove 'npm' shim without confirmation`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigRemove,
}

func init() {
	configRemoveCmd.Flags().BoolVar(&configRemoveForce, "force", false, "Skip confirmation prompt")
}

func runConfigRemove(cmd *cobra.Command, args []string) error {
	cmdName := args[0]

	// Find ribbin.toml
	configPath, err := config.FindProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to find config: %w", err)
	}

	if configPath == "" {
		return fmt.Errorf("ribbin.toml not found. Run 'ribbin init' first.")
	}

	// Load and check configuration
	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Verify command exists in config
	shimCfg, exists := cfg.Shims[cmdName]
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
		fmt.Printf("Remove shim for '%s'? [y/N] ", cmdName)
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

	// Remove shim
	if err := config.RemoveShim(configPath, cmdName); err != nil {
		return fmt.Errorf("failed to remove shim: %w", err)
	}

	// Show success message
	fmt.Printf("Removed shim for '%s'\n", cmdName)
	fmt.Printf("Run 'ribbin unshim %s' to remove the installed shim.\n", cmdName)

	return nil
}
