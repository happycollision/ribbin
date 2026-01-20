package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/spf13/cobra"
)

var (
	configShowJSON    bool
	configShowCommand string
)

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show effective shim configuration with provenance",
	Long: `Display the effective shim configuration for the current working directory.

Shows which config file applies, which scope matches (if any), and lists
all effective shims with their sources. This is useful for understanding
how scope inheritance and extends work together.

Examples:
  ribbin config show                    Show all effective shims
  ribbin config show --json             Output in JSON format
  ribbin config show --command npm      Show only the 'npm' shim configuration`,
	RunE: runConfigShow,
}

func init() {
	configShowCmd.Flags().BoolVar(&configShowJSON, "json", false, "Output in JSON format")
	configShowCmd.Flags().StringVar(&configShowCommand, "command", "", "Filter to specific command")
}

// configShowOutput represents the JSON output structure for config show
type configShowOutput struct {
	ConfigPath string                       `json:"config_path"`
	Scope      *scopeOutput                 `json:"scope,omitempty"`
	Shims      map[string]resolvedShimJSON  `json:"shims"`
}

type scopeOutput struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type resolvedShimJSON struct {
	Action      string           `json:"action"`
	Message     string           `json:"message,omitempty"`
	Redirect    string           `json:"redirect,omitempty"`
	Paths       []string         `json:"paths,omitempty"`
	Source      shimSourceJSON   `json:"source"`
}

type shimSourceJSON struct {
	FilePath string          `json:"file_path"`
	Fragment string          `json:"fragment"`
	Overrode *shimSourceJSON `json:"overrode,omitempty"`
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	// Get effective config for current working directory
	configPath, matchedScope, shims, err := config.GetEffectiveConfigForCwd()
	if err != nil {
		return fmt.Errorf("failed to get effective config: %w", err)
	}

	if configPath == "" {
		return fmt.Errorf("No ribbin.toml found. Run 'ribbin init' to create one.")
	}

	// Filter by command if specified
	if configShowCommand != "" {
		if resolved, ok := shims[configShowCommand]; ok {
			shims = map[string]config.ResolvedShim{
				configShowCommand: resolved,
			}
		} else {
			return fmt.Errorf("command '%s' not found in effective configuration", configShowCommand)
		}
	}

	// Output based on format
	if configShowJSON {
		return outputShowJSON(configPath, matchedScope, shims)
	}
	return outputShowText(configPath, matchedScope, shims)
}

func outputShowJSON(configPath string, matchedScope *config.MatchedScope, shims map[string]config.ResolvedShim) error {
	output := configShowOutput{
		ConfigPath: configPath,
		Shims:      make(map[string]resolvedShimJSON),
	}

	if matchedScope != nil {
		output.Scope = &scopeOutput{
			Name: matchedScope.Name,
			Path: matchedScope.Config.Path,
		}
	}

	for name, resolved := range shims {
		output.Shims[name] = convertResolvedShimToJSON(resolved)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

func convertResolvedShimToJSON(resolved config.ResolvedShim) resolvedShimJSON {
	result := resolvedShimJSON{
		Action:   resolved.Config.Action,
		Message:  resolved.Config.Message,
		Redirect: resolved.Config.Redirect,
		Paths:    resolved.Config.Paths,
		Source:   convertShimSourceToJSON(resolved.Source),
	}
	return result
}

func convertShimSourceToJSON(source config.ShimSource) shimSourceJSON {
	result := shimSourceJSON{
		FilePath: source.FilePath,
		Fragment: source.Fragment,
	}
	if source.Overrode != nil {
		overrode := convertShimSourceToJSON(*source.Overrode)
		result.Overrode = &overrode
	}
	return result
}

func outputShowText(configPath string, matchedScope *config.MatchedScope, shims map[string]config.ResolvedShim) error {
	// Print config file path
	fmt.Printf("Config: %s\n", configPath)

	// Print scope info
	if matchedScope != nil {
		scopePath := matchedScope.Config.Path
		if scopePath == "" {
			scopePath = "."
		}
		fmt.Printf("Scope:  %s (path: %s)\n", matchedScope.Name, scopePath)
	} else {
		fmt.Printf("Scope:  (root)\n")
	}

	// Check if there are any shims
	if len(shims) == 0 {
		fmt.Println("\nNo effective shims configured")
		return nil
	}

	// Get sorted command names for consistent output
	commands := make([]string, 0, len(shims))
	for cmd := range shims {
		commands = append(commands, cmd)
	}
	sort.Strings(commands)

	fmt.Println("\nEffective shims:")

	for _, cmd := range commands {
		resolved := shims[cmd]
		fmt.Printf("  %s\n", cmd)
		fmt.Printf("    action:  %s\n", resolved.Config.Action)

		if resolved.Config.Message != "" {
			fmt.Printf("    message: %q\n", resolved.Config.Message)
		}

		if resolved.Config.Redirect != "" {
			fmt.Printf("    redirect: %s\n", resolved.Config.Redirect)
		}

		if len(resolved.Config.Paths) > 0 {
			fmt.Printf("    paths: %v\n", resolved.Config.Paths)
		}

		// Print source with fragment
		fmt.Printf("    source:  %s#%s\n", resolved.Source.FilePath, resolved.Source.Fragment)

		// Print override chain if present
		if resolved.Source.Overrode != nil {
			printOverrideChain(resolved.Source.Overrode, 1)
		}
	}

	return nil
}

func printOverrideChain(source *config.ShimSource, depth int) {
	indent := "             " // aligns with "source:  "
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	fmt.Printf("%s(overrides %s#%s)\n", indent, source.FilePath, source.Fragment)
	if source.Overrode != nil {
		printOverrideChain(source.Overrode, depth+1)
	}
}
