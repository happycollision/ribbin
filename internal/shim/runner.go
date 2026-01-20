package shim

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/process"
	"github.com/happycollision/ribbin/internal/security"
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

	// Extract command name from argv0 (needed for verbose logging)
	cmdName := extractCommandName(argv0)

	// 3. Check RIBBIN_BYPASS=1 -> passthrough
	if os.Getenv("RIBBIN_BYPASS") == "1" {
		// Log bypass usage
		security.LogBypassUsage(originalPath, os.Getpid())
		verboseLogDecision(cmdName, "PASS", "RIBBIN_BYPASS=1")
		return execOriginal(originalPath, args)
	}

	// 4. Load registry, check if active
	registry, err := config.LoadRegistry()
	if err != nil {
		// If we can't load registry, passthrough
		verboseLogDecision(cmdName, "PASS", "registry not found")
		return execOriginal(originalPath, args)
	}

	// 5. If not active -> passthrough
	if !isActive(registry) {
		verboseLogDecision(cmdName, "PASS", "ribbin not active")
		return execOriginal(originalPath, args)
	}

	// 6. Find nearest ribbin.toml
	configPath, err := config.FindProjectConfig()
	if err != nil || configPath == "" {
		// No config found -> passthrough
		verboseLogDecision(cmdName, "PASS", "no ribbin.toml found")
		return execOriginal(originalPath, args)
	}

	// 7. Load project config
	projectConfig, err := config.LoadProjectConfig(configPath)
	if err != nil {
		// Can't load config -> passthrough
		verboseLogDecision(cmdName, "PASS", fmt.Sprintf("config load failed: %v", err))
		return execOriginal(originalPath, args)
	}

	// 8. Determine effective shims based on scope matching
	shimConfig, exists := getEffectiveShimConfig(projectConfig, configPath, cmdName)
	if !exists {
		// Command not in config -> passthrough
		verboseLogDecision(cmdName, "PASS", "no shim configured")
		return execOriginal(originalPath, args)
	}

	// 9. Check passthrough conditions
	if shimConfig.Passthrough != nil {
		if shouldPassthrough(shimConfig.Passthrough) {
			verboseLogDecision(cmdName, "PASS", "parent process matched passthrough rule")
			return execOriginal(originalPath, args)
		}
	}

	// 10. Handle action based on config
	switch shimConfig.Action {
	case "block":
		verboseLogDecision(cmdName, "BLOCKED", shimConfig.Message)
		printBlockMessage(cmdName, shimConfig.Message)
		os.Exit(1)
		return nil // unreachable, but satisfies compiler

	case "passthrough":
		// Explicit passthrough action - execute original binary
		verboseLogDecision(cmdName, "PASS", "explicit passthrough action")
		return execOriginal(originalPath, args)

	case "redirect":
		// Validate redirect field is not empty
		if shimConfig.Redirect == "" {
			verboseLogDecision(cmdName, "PASS", "redirect action but no script configured")
			fmt.Fprintf(os.Stderr, "ribbin: redirect action specified but no redirect script configured for '%s', using original\n", cmdName)
			return execOriginal(originalPath, args)
		}

		// Resolve redirect script path
		scriptPath, err := resolveRedirectScript(shimConfig.Redirect, configPath)
		if err != nil {
			// Fail-open: warn and passthrough
			verboseLogDecision(cmdName, "PASS", fmt.Sprintf("redirect failed: %v", err))
			fmt.Fprintf(os.Stderr, "ribbin: redirect failed (%s), using original: %v\n", cmdName, err)
			return execOriginal(originalPath, args)
		}

		// Execute redirect script
		verboseLogDecision(cmdName, "REDIRECT", shimConfig.Redirect)
		return execRedirect(scriptPath, originalPath, cmdName, args, configPath)

	default:
		// Unknown action or empty -> passthrough
		verboseLogDecision(cmdName, "PASS", "no action specified")
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

// shouldPassthrough checks if the parent process invocation matches any passthrough conditions.
// Returns true if the shim should pass through to the original command.
func shouldPassthrough(pt *config.PassthroughConfig) bool {
	parentCmd, err := process.GetParentCommand()
	if err != nil {
		// If we can't get parent command, don't passthrough
		return false
	}

	// Check exact matches
	for _, pattern := range pt.Invocation {
		if strings.Contains(parentCmd, pattern) {
			return true
		}
	}

	// Check regexp matches
	for _, pattern := range pt.InvocationRegexp {
		re, err := regexp.Compile(pattern)
		if err != nil {
			// Invalid regex, skip it
			continue
		}
		if re.MatchString(parentCmd) {
			return true
		}
	}

	return false
}

// getEffectiveShimConfig determines the effective shim configuration for a command
// by finding the best matching scope and using the Resolver to merge shim maps.
func getEffectiveShimConfig(projectConfig *config.ProjectConfig, configPath string, cmdName string) (config.ShimConfig, bool) {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		// Fall back to root shims if we can't get CWD
		shimConfig, exists := projectConfig.Shims[cmdName]
		return shimConfig, exists
	}

	// Find the best matching scope
	matchingScope := findBestMatchingScope(projectConfig, configPath, cwd)

	// Use Resolver to get effective shims
	resolver := config.NewResolver()
	effectiveShims, err := resolver.ResolveEffectiveShims(projectConfig, configPath, matchingScope)
	if err != nil {
		// If resolution fails, fall back to root shims
		shimConfig, exists := projectConfig.Shims[cmdName]
		return shimConfig, exists
	}

	shimConfig, exists := effectiveShims[cmdName]
	return shimConfig, exists
}

// findBestMatchingScope finds the scope with the deepest path that contains the CWD.
// Returns nil if no scope matches (meaning root shims should be used).
func findBestMatchingScope(projectConfig *config.ProjectConfig, configPath string, cwd string) *config.ScopeConfig {
	configDir := filepath.Dir(configPath)

	// Resolve symlinks in CWD to handle macOS /var -> /private/var symlink
	resolvedCwd, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		resolvedCwd = cwd
	}
	resolvedCwd = filepath.Clean(resolvedCwd)

	var bestMatch *config.ScopeConfig
	bestMatchDepth := -1

	for _, scope := range projectConfig.Scopes {
		scopePath := scope.Path
		if scopePath == "" {
			scopePath = "."
		}

		// Resolve scope path to absolute
		var absScopePath string
		if filepath.IsAbs(scopePath) {
			absScopePath = scopePath
		} else {
			absScopePath = filepath.Join(configDir, scopePath)
		}

		// Resolve symlinks in scope path for consistent comparison
		resolvedScopePath, err := filepath.EvalSymlinks(absScopePath)
		if err != nil {
			resolvedScopePath = absScopePath
		}
		resolvedScopePath = filepath.Clean(resolvedScopePath)

		// Check if CWD is within the scope path
		if isPathWithin(resolvedCwd, resolvedScopePath) {
			// Calculate depth (number of path components)
			depth := countPathComponents(resolvedScopePath)
			if depth > bestMatchDepth {
				bestMatchDepth = depth
				scopeCopy := scope
				bestMatch = &scopeCopy
			}
		}
	}

	return bestMatch
}

// isPathWithin checks if targetPath is within or equal to basePath.
func isPathWithin(targetPath, basePath string) bool {
	// Handle exact match
	if targetPath == basePath {
		return true
	}

	// Check if target is a subdirectory of base
	// Use filepath.Rel to determine relationship
	rel, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return false
	}

	// If the relative path starts with "..", target is not within base
	if strings.HasPrefix(rel, "..") {
		return false
	}

	return true
}

// countPathComponents counts the number of components in a path.
func countPathComponents(path string) int {
	// Clean the path first
	path = filepath.Clean(path)

	// Split by separator and count non-empty components
	parts := strings.Split(path, string(filepath.Separator))
	count := 0
	for _, part := range parts {
		if part != "" {
			count++
		}
	}
	return count
}
