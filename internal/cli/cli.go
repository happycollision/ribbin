package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ribbin",
	Short: "Command shimming tool",
	Long:  "ribbin - blocks direct tool calls and redirects to project-specific alternatives",
}

func init() {
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
