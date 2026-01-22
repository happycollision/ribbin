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

	if execName == "ribbin" || execName == "ribbin-next" {
		// CLI mode
		if err := cli.Execute(); err != nil {
			fmt.Fprintf(os.Stderr, "ribbin: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Shim mode - invoked as a shimmed command (e.g., "cat", "tsc")
		// We need to find the actual symlink path that was invoked.
		shimPath := os.Args[0]

		if !filepath.IsAbs(shimPath) {
			// If invoked as just "npm" (not "/path/to/npm"), try to resolve it

			// First, try looking it up in PATH
			if resolved, err := resolveInPath(shimPath); err == nil {
				shimPath = resolved
			} else {
				// PATH lookup failed (e.g., pnpm exec runs binaries not in PATH)
				// This happens when a package manager like pnpm executes a binary
				// directly without it being in PATH.

				// Convert to absolute path based on CWD
				// The sidecar lookup in wrap.Run() will handle finding the actual sidecar
				if absPath, err := filepath.Abs(os.Args[0]); err == nil {
					shimPath = absPath
				} else {
					shimPath = os.Args[0]
				}
			}
		}

		if err := wrap.Run(shimPath, os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", execName, err)
			os.Exit(1)
		}
	}
}
