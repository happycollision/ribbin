package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/happycollision/ribbin/internal/cli"
	"github.com/happycollision/ribbin/internal/wrap"
)

// resolveInPath looks up a command name in PATH and returns the full path
func resolveInPath(name string) (string, error) {
	return exec.LookPath(name)
}

func main() {
	// Mode detection: check if invoked as "ribbin" or as a shimmed command
	execName := filepath.Base(os.Args[0])

	if execName == "ribbin" {
		// CLI mode
		if err := cli.Execute(); err != nil {
			fmt.Fprintf(os.Stderr, "ribbin: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Shim mode - invoked as a shimmed command (e.g., "cat", "tsc")
		// Resolve the symlink path (not the target) from os.Args[0]
		shimPath := os.Args[0]
		if !filepath.IsAbs(shimPath) {
			// If invoked as just "npm" (not "/path/to/npm"), look it up in PATH
			// This gives us the symlink path, not the ribbin binary path
			if resolved, err := resolveInPath(shimPath); err == nil {
				shimPath = resolved
			}
		}
		if err := wrap.Run(shimPath, os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", execName, err)
			os.Exit(1)
		}
	}
}
