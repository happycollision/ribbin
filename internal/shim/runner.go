package shim

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/process"
)

// Run is the main entry point for shim mode.
// argv0 is the path to the symlink (e.g., /usr/local/bin/cat)
// args are the command-line arguments (os.Args[1:])
func Run(argv0 string, args []string) error {
	// 1. Get original path: argv0 + ".ribbin-original"
	originalPath := argv0 + ".ribbin-original"

	// 2. Verify sidecar exists
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		return fmt.Errorf("original binary not found at %s", originalPath)
	}

	// 3. Check RIBBIN_BYPASS=1 -> passthrough
	if os.Getenv("RIBBIN_BYPASS") == "1" {
		return execOriginal(originalPath, args)
	}

	// 4. Load registry, check if active
	registry, err := config.LoadRegistry()
	if err != nil {
		// If we can't load registry, passthrough
		return execOriginal(originalPath, args)
	}

	// 5. If not active -> passthrough
	if !isActive(registry) {
		return execOriginal(originalPath, args)
	}

	// 6. Find nearest ribbin.toml
	configPath, err := config.FindProjectConfig()
	if err != nil || configPath == "" {
		// No config found -> passthrough
		return execOriginal(originalPath, args)
	}

	// 7. Load project config
	projectConfig, err := config.LoadProjectConfig(configPath)
	if err != nil {
		// Can't load config -> passthrough
		return execOriginal(originalPath, args)
	}

	// Extract command name from argv0
	cmdName := extractCommandName(argv0)

	// Check if command is in config
	shimConfig, exists := projectConfig.Shims[cmdName]
	if !exists {
		// Command not in config -> passthrough
		return execOriginal(originalPath, args)
	}

	// 8. Handle action based on config
	switch shimConfig.Action {
	case "block":
		printBlockMessage(cmdName, shimConfig.Message)
		os.Exit(1)
		return nil // unreachable, but satisfies compiler

	case "redirect":
		// Validate redirect field is not empty
		if shimConfig.Redirect == "" {
			fmt.Fprintf(os.Stderr, "ribbin: redirect action specified but no redirect script configured for '%s', using original\n", cmdName)
			return execOriginal(originalPath, args)
		}

		// Resolve redirect script path
		scriptPath, err := resolveRedirectScript(shimConfig.Redirect, configPath)
		if err != nil {
			// Fail-open: warn and passthrough
			fmt.Fprintf(os.Stderr, "ribbin: redirect failed (%s), using original: %v\n", cmdName, err)
			return execOriginal(originalPath, args)
		}

		// Execute redirect script
		return execRedirect(scriptPath, originalPath, cmdName, args, configPath)

	default:
		// Unknown action or empty -> passthrough
		return execOriginal(originalPath, args)
	}
}

// isActive checks if ribbin is active (global_on OR ancestor PID in activations)
func isActive(registry *config.Registry) bool {
	// Check global_on flag
	if registry.GlobalOn {
		return true
	}

	// Prune dead activations first
	registry.PruneDeadActivations()

	// Check if any activation PID is an ancestor of this process
	for pid := range registry.Activations {
		isDescendant, err := process.IsDescendantOf(pid)
		if err == nil && isDescendant {
			return true
		}
	}

	return false
}

// execOriginal uses syscall.Exec to replace the current process with the original command
func execOriginal(path string, args []string) error {
	// Build argv: first element is the program path, followed by all arguments
	argv := append([]string{path}, args...)

	// Get current environment
	env := os.Environ()

	// Replace current process with the original command
	return syscall.Exec(path, argv, env)
}

// execRedirect executes a redirect script with ribbin environment context
func execRedirect(scriptPath, originalPath, cmdName string, args []string, configPath string) error {
	// Build argv: first element is the script path, followed by all arguments
	argv := append([]string{scriptPath}, args...)

	// Build environment with ribbin-specific variables
	env := os.Environ()
	env = append(env,
		"RIBBIN_ORIGINAL_BIN="+originalPath,
		"RIBBIN_COMMAND="+cmdName,
		"RIBBIN_CONFIG="+configPath,
		"RIBBIN_ACTION=redirect",
	)

	// Replace current process with the redirect script
	return syscall.Exec(scriptPath, argv, env)
}

// extractCommandName extracts the command name from a path
func extractCommandName(path string) string {
	// Get the base name of the path
	base := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		base = path[idx+1:]
	}
	return base
}

// printBlockMessage prints a nicely formatted error box
func printBlockMessage(cmd, message string) {
	// Default message if none provided
	if message == "" {
		message = "This command is blocked by ribbin."
	}

	// Build the message lines
	errorLine := fmt.Sprintf("ERROR: Direct use of '%s' is blocked.", cmd)
	bypassLine := fmt.Sprintf("Bypass: RIBBIN_BYPASS=1 %s ...", cmd)

	// Calculate the maximum line width
	lines := []string{errorLine, "", message, "", bypassLine}
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	// Add padding for the box
	boxWidth := maxLen + 4 // 2 spaces on each side

	// Print the box
	fmt.Fprintln(os.Stderr)
	printBoxTop(boxWidth)
	for _, line := range lines {
		printBoxLine(line, boxWidth)
	}
	printBoxBottom(boxWidth)
	fmt.Fprintln(os.Stderr)
}

// printBoxTop prints the top border of the box
func printBoxTop(width int) {
	fmt.Fprint(os.Stderr, "\u250c")
	for i := 0; i < width; i++ {
		fmt.Fprint(os.Stderr, "\u2500")
	}
	fmt.Fprintln(os.Stderr, "\u2510")
}

// printBoxBottom prints the bottom border of the box
func printBoxBottom(width int) {
	fmt.Fprint(os.Stderr, "\u2514")
	for i := 0; i < width; i++ {
		fmt.Fprint(os.Stderr, "\u2500")
	}
	fmt.Fprintln(os.Stderr, "\u2518")
}

// printBoxLine prints a line inside the box with proper padding
func printBoxLine(content string, width int) {
	padding := width - len(content) - 2 // -2 for the leading "  "
	fmt.Fprintf(os.Stderr, "\u2502  %s%s\u2502\n", content, strings.Repeat(" ", padding))
}
