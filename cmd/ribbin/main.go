package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/happycollision/ribbin/internal/cli"
	"github.com/happycollision/ribbin/internal/shim"
)

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
		// Pass the full path from os.Args[0] and remaining args
		if err := shim.Run(os.Args[0], os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", execName, err)
			os.Exit(1)
		}
	}
}
