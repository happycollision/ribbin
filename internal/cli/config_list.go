package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/spf13/cobra"
)

var (
	configListJSON    bool
	configListCommand string
)

var configListCmd = &cobra.Command{
	Use:   "list [config-path]",
	Short: "List all wrapper configurations",
	Long: `Display all configured wrappers from a config file.

If no path is provided, uses the nearest ribbin.jsonc or ribbin.local.jsonc.

Shows command name, action type, and configuration details.

Examples:
  ribbin config list                    List wrappers from nearest config
  ribbin config list ./ribbin.jsonc     List wrappers from specific config
  ribbin config list --json             Output in JSON format
  ribbin config list --command tsc      Show only the 'tsc' wrapper configuration`,
	RunE: runConfigList,
}

func init() {
	configListCmd.Flags().BoolVar(&configListJSON, "json", false, "Output in JSON format")
	configListCmd.Flags().StringVar(&configListCommand, "command", "", "Filter to specific command")
}

func runConfigList(cmd *cobra.Command, args []string) error {
	var configPath string
	var err error

	if len(args) > 0 {
		configPath = args[0]
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("config file not found: %s", configPath)
		}
	} else {
		configPath, err = config.FindProjectConfig()
		if err != nil {
			return fmt.Errorf("failed to find config: %w", err)
		}
		if configPath == "" {
			return fmt.Errorf("No ribbin.jsonc found. Run 'ribbin init' to create one.")
		}
	}

	// Load the config
	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to parse ribbin.jsonc: %w", err)
	}

	// Check if config is empty
	if len(cfg.Wrappers) == 0 {
		fmt.Println("No wrappers configured")
		return nil
	}

	// Filter by command if specified
	shims := cfg.Wrappers
	if configListCommand != "" {
		if shimCfg, ok := cfg.Wrappers[configListCommand]; ok {
			shims = map[string]config.ShimConfig{
				configListCommand: shimCfg,
			}
		} else {
			return fmt.Errorf("command '%s' not found in configuration", configListCommand)
		}
	}

	// Output based on format
	if configListJSON {
		return outputJSON(shims)
	}
	return outputTable(shims)
}

func outputJSON(shims map[string]config.ShimConfig) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(shims); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

func outputTable(shims map[string]config.ShimConfig) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprintln(w, "COMMAND\tACTION\tDETAILS")

	// Get sorted keys for consistent output
	commands := make([]string, 0, len(shims))
	for cmd := range shims {
		commands = append(commands, cmd)
	}

	// Simple sort to ensure consistent ordering
	for i := 0; i < len(commands); i++ {
		for j := i + 1; j < len(commands); j++ {
			if commands[i] > commands[j] {
				commands[i], commands[j] = commands[j], commands[i]
			}
		}
	}

	for _, cmd := range commands {
		shimCfg := shims[cmd]
		details := formatDetails(shimCfg)
		fmt.Fprintf(w, "%s\t%s\t%s\n", cmd, shimCfg.Action, details)
	}

	return w.Flush()
}

func formatDetails(shimCfg config.ShimConfig) string {
	var parts []string

	// Add redirect target if present
	if shimCfg.Redirect != "" {
		parts = append(parts, shimCfg.Redirect)
	}

	// Add message if present
	if shimCfg.Message != "" {
		// For redirects, show message in parentheses if there's already a redirect
		if shimCfg.Redirect != "" {
			parts = append(parts, fmt.Sprintf("(%s)", shimCfg.Message))
		} else {
			parts = append(parts, shimCfg.Message)
		}
	}

	// Add paths if present
	if len(shimCfg.Paths) > 0 {
		pathList := strings.Join(shimCfg.Paths, ", ")
		parts = append(parts, fmt.Sprintf("(paths: %s)", pathList))
	}

	if len(parts) == 0 {
		return "-"
	}

	return strings.Join(parts, " ")
}
