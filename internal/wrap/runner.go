package wrap

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

// findSidecar attempts to locate the .ribbin-original sidecar file.
// It checks multiple locations in order:
// 1. Next to argv0 (e.g., if argv0 is "/path/to/tsc", checks "/path/to/tsc.ribbin-original")
// 2. Resolved to absolute path if argv0 is relative
// 3. Next to the executable (for dual-sidecar support with symlink chains)
// 4. Registry lookup (handles cases where argv0 doesn't match wrapped location)
// Returns the path to the sidecar, or empty string if not found.
func findSidecar(argv0 string) string {
	cmdName := filepath.Base(argv0)

	// Strategy 1: Check next to argv0
	sidecarPath := argv0 + ".ribbin-original"
	if _, err := os.Stat(sidecarPath); err == nil {
		return sidecarPath
	}

	// Strategy 2: If argv0 is relative or just a command name, resolve to absolute
	if !filepath.IsAbs(argv0) {
		if absPath, err := filepath.Abs(argv0); err == nil {
			sidecarPath = absPath + ".ribbin-original"
			if _, err := os.Stat(sidecarPath); err == nil {
				return sidecarPath
			}
		}
	}

	// Strategy 3: Check next to the executable (for symlink chains)
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		sidecarPath = filepath.Join(exeDir, cmdName+".ribbin-original")
		if _, err := os.Stat(sidecarPath); err == nil {
			return sidecarPath
		}
	}

	// Strategy 4: Look up in registry to find where this command was wrapped
	// This handles cases like `pnpm exec tsc` where argv0 doesn't match the wrapped location
	if registry, err := config.LoadRegistry(); err == nil {
		if entry, ok := registry.Wrappers[cmdName]; ok {
			sidecarPath = entry.Original + ".ribbin-original"
			if _, err := os.Stat(sidecarPath); err == nil {
				return sidecarPath
			}
		}
	}

	return ""
}

// Run is the main entry point for shim mode.
// argv0 is the path to the symlink (e.g., /usr/local/bin/cat)
// args are the command-line arguments (os.Args[1:])
func Run(argv0 string, args []string) error {
	// 1. Find the sidecar file
	// It could be at argv0 + ".ribbin-original" OR next to the actual executable
	sidecarPath := findSidecar(argv0)
	if sidecarPath == "" {
		return fmt.Errorf("original binary not found (no .ribbin-original sidecar found)")
	}

	// 2. Use sidecar as original path (may be a symlink, which is fine)
	originalPath := sidecarPath

	// Extract command name from argv0 (needed for verbose logging)
	cmdName := extractCommandName(argv0)

	// 4. Check RIBBIN_BYPASS=1 -> passthrough
	if os.Getenv("RIBBIN_BYPASS") == "1" {
		// Log bypass usage
		security.LogBypassUsage(originalPath, os.Getpid())
		verboseLogDecision(cmdName, "PASS", "RIBBIN_BYPASS=1")
		return execOriginal(originalPath, args)
	}

	// 4. Load registry
	registry, err := config.LoadRegistry()
	if err != nil {
		// If we can't load registry, passthrough
		verboseLogDecision(cmdName, "PASS", "registry not found")
		return execOriginal(originalPath, args)
	}

	// 5. Find nearest ribbin.jsonc (needed for activation check)
	configPath, err := config.FindProjectConfig()
	if err != nil || configPath == "" {
		// No config found -> passthrough
		verboseLogDecision(cmdName, "PASS", "no ribbin.jsonc found")
		return execOriginal(originalPath, args)
	}

	// 6. Check if active using three-tier activation model
	if !isActive(registry, configPath) {
		verboseLogDecision(cmdName, "PASS", "ribbin not active")
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

// isActive checks if ribbin is active using three-tier activation priority:
// Priority 1: GlobalActive - fires everything everywhere
// Priority 2: ShellActivations - all configs fire for descendant processes
// Priority 3: ConfigActivations - specific config fires for all shells
func isActive(registry *config.Registry, configPath string) bool {
	// Priority 1: Global overrides everything
	if registry.GlobalActive {
		return true
	}

	// Priority 2: Shell activation (any config fires for descendants)
	registry.PruneDeadShellActivations()
	for pid := range registry.ShellActivations {
		isDescendant, err := process.IsDescendantOf(pid)
		if err == nil && isDescendant {
			return true
		}
	}

	// Priority 3: Config-specific activation
	if configPath != "" {
		if _, ok := registry.ConfigActivations[configPath]; ok {
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

// shouldPassthrough checks if any ancestor process invocation matches passthrough conditions.
// Returns true if the shim should pass through to the original command.
func shouldPassthrough(pt *config.PassthroughConfig) bool {
	// Determine max depth (0 = unlimited)
	maxDepth := 0
	if pt.Depth != nil {
		maxDepth = *pt.Depth
	}

	// Get ancestor commands up to depth limit
	ancestorCmds, err := process.GetAncestorCommands(maxDepth)
	if err != nil || len(ancestorCmds) == 0 {
		return false
	}

	// Check each ancestor against patterns
	for _, cmd := range ancestorCmds {
		// Check exact matches
		for _, pattern := range pt.Invocation {
			if strings.Contains(cmd, pattern) {
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
			if re.MatchString(cmd) {
				return true
			}
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
		// Fall back to root wrappers if we can't get CWD
		shimConfig, exists := projectConfig.Wrappers[cmdName]
		return shimConfig, exists
	}

	// Find the best matching scope
	matchingScope := findBestMatchingScope(projectConfig, configPath, cwd)

	// Use Resolver to get effective shims
	resolver := config.NewResolver()
	effectiveShims, err := resolver.ResolveEffectiveShims(projectConfig, configPath, matchingScope)
	if err != nil {
		// If resolution fails, fall back to root wrappers
		shimConfig, exists := projectConfig.Wrappers[cmdName]
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
