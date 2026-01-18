package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set by ldflags at build time
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:     "ribbin",
	Short:   "Command shimming tool",
	Long:    "ribbin - blocks direct tool calls and redirects to project-specific alternatives",
	Version: Version,
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("ribbin %s\n", Version))
	rootCmd.Flags().BoolP("version", "V", false, "Print version information")
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(shimCmd)
	rootCmd.AddCommand(unshimCmd)
	rootCmd.AddCommand(activateCmd)
	rootCmd.AddCommand(onCmd)
	rootCmd.AddCommand(offCmd)
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}
