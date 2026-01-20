package cli

import (
	"fmt"
	"os"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/wrap"
	"github.com/spf13/cobra"
)

// printGlobalWarningIfActive prints a warning banner to stderr if global mode is active.
// Call this at the start of every CLI command to alert users.
func printGlobalWarningIfActive() {
	registry, err := config.LoadRegistry()
	if err != nil {
		return // Silently fail - don't block CLI on registry errors
	}
	if registry.GlobalActive {
		fmt.Fprintln(os.Stderr, "⚠️  GLOBAL MODE ACTIVE - All wrappers firing everywhere")
		fmt.Fprintln(os.Stderr, "   Run 'ribbin deactivate --global' to disable")
		fmt.Fprintln(os.Stderr, "")
	}
}

// Version is set by ldflags at build time
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "ribbin",
	Short: "Command wrapping tool",
	Long: `ribbin intercepts commands and can block, warn, or redirect them.

It intercepts calls to specified commands (like tsc, npm, cat) and takes
configured actions: display error messages (block), show warnings (warn),
or execute custom scripts (redirect). This helps enforce project conventions
for both humans and AI agents.

Quick start:
  ribbin init        Create a ribbin.toml config file
  ribbin wrap        Install wrappers for commands in ribbin.toml
  ribbin activate    Activate ribbin (config, shell, or global)
  ribbin status      Show activation status

For more information, see https://github.com/happycollision/ribbin`,
	Version: Version,
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("ribbin %s\n", Version))
	rootCmd.Flags().BoolP("version", "V", false, "Print version information")
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(wrapCmd)
	rootCmd.AddCommand(unwrapCmd)
	rootCmd.AddCommand(activateCmd)
	rootCmd.AddCommand(deactivateCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(recoverCmd)

	// Set version for metadata in wrap package
	wrap.Version = Version
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}
