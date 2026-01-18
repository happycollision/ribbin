package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const MaxSymlinkDepth = 10 // Prevent infinite loops and DoS attacks

// SymlinkInfo contains detailed information about a symlink chain
type SymlinkInfo struct {
	IsSymlink   bool     // Whether the original path is a symlink
	Target      string   // Direct target (first level)
	FinalTarget string   // Final target after resolving all symlinks
	ChainDepth  int      // Number of symlinks in the chain
	Chain       []string // All paths in the symlink chain
}

// SafeReadlink reads a symlink and validates its target.
// Unlike os.Readlink, it verifies the path is actually a symlink,
// resolves relative paths, and canonicalizes the target.
func SafeReadlink(linkPath string) (string, error) {
	// Must use Lstat, not Stat (don't follow symlinks)
	info, err := os.Lstat(linkPath)
	if err != nil {
		return "", fmt.Errorf("cannot stat link: %w", err)
	}

	// Verify it's actually a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("not a symlink: %s", linkPath)
	}

	// Read the target
	target, err := os.Readlink(linkPath)
	if err != nil {
		return "", fmt.Errorf("cannot read symlink: %w", err)
	}

	// If target is relative, resolve against link's directory
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(linkPath), target)
	}

	// Canonicalize target
	target, err = filepath.Abs(filepath.Clean(target))
	if err != nil {
		return "", fmt.Errorf("cannot resolve target: %w", err)
	}

	return target, nil
}

// ResolveSymlinkChain follows symlinks to the final target.
// Returns an error if:
// - The chain is too deep (>MaxSymlinkDepth)
// - A circular reference is detected
// - The final target is outside allowed directories
// - The final target is a critical system binary
func ResolveSymlinkChain(path string) (string, error) {
	current := path
	visited := make(map[string]bool)

	for depth := 0; depth < MaxSymlinkDepth; depth++ {
		// Check for cycles
		if visited[current] {
			return "", fmt.Errorf("circular symlink detected: %s", current)
		}
		visited[current] = true

		// Check current path
		info, err := os.Lstat(current)
		if err != nil {
			return "", fmt.Errorf("cannot stat %s: %w", current, err)
		}

		// If not a symlink, we're done
		if info.Mode()&os.ModeSymlink == 0 {
			// Validate final target is in allowed directory
			allowed := isWithinAllowedDirectory(current)
			if !allowed {
				return "", fmt.Errorf("symlink target outside allowed directories: %s", current)
			}

			// Check for critical system binaries
			if IsCriticalSystemBinary(current) {
				return "", fmt.Errorf("symlink points to critical system binary: %s", current)
			}

			return current, nil
		}

		// Read and validate symlink
		target, err := SafeReadlink(current)
		if err != nil {
			return "", fmt.Errorf("invalid symlink in chain: %w", err)
		}

		// Validate intermediate target is safe
		if err := validateSymlinkTargetSafety(current, target); err != nil {
			return "", err
		}

		current = target
	}

	return "", fmt.Errorf("symlink chain too deep (>%d): %s", MaxSymlinkDepth, path)
}

// ValidateSymlinkTargetSafe checks if a symlink target is safe.
// This is a more comprehensive version of ValidateSymlinkTarget in paths.go.
func ValidateSymlinkTargetSafe(link, target string) error {
	// Check for path traversal BEFORE resolving the path
	// This catches explicit traversal attempts like "../../etc/passwd"
	if strings.Contains(target, "..") {
		return fmt.Errorf("path traversal in symlink target: %s", target)
	}

	// Get absolute paths
	absLink, err := filepath.Abs(filepath.Clean(link))
	if err != nil {
		return fmt.Errorf("cannot resolve link: %w", err)
	}
	absTarget, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		return fmt.Errorf("cannot resolve target: %w", err)
	}

	return validateSymlinkTargetSafety(absLink, absTarget)
}

// validateSymlinkTargetSafety is the internal implementation
func validateSymlinkTargetSafety(absLink, absTarget string) error {
	// Check for path traversal FIRST (before directory checks)
	// This catches explicit traversal attempts like "../../etc/passwd"
	if strings.Contains(absTarget, ".."+string(filepath.Separator)) || strings.HasSuffix(absTarget, "..") {
		return fmt.Errorf("path traversal in symlink target: %s", absTarget)
	}

	// Check for critical system binaries (check this before directory validation)
	// This provides a more specific error message for attempts to shim critical binaries
	if IsCriticalSystemBinary(absTarget) {
		return fmt.Errorf("symlink points to critical system binary: %s -> %s",
			absLink, absTarget)
	}

	// Target must be in an allowed directory (checked last for clearer error messages)
	allowed := isWithinAllowedDirectory(absTarget)
	if !allowed {
		return fmt.Errorf("symlink target outside allowed directories: %s -> %s",
			absLink, absTarget)
	}

	return nil
}

// IsSymlinkSafe performs a comprehensive symlink safety check.
// It resolves the entire symlink chain and validates every step.
func IsSymlinkSafe(linkPath string) (bool, error) {
	// Resolve full chain
	_, err := ResolveSymlinkChain(linkPath)
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetSymlinkInfo returns detailed information about a symlink chain.
// This function is useful for debugging and displaying symlink information to users.
func GetSymlinkInfo(path string) (*SymlinkInfo, error) {
	info := &SymlinkInfo{
		Chain: []string{path},
	}

	// Check if it's a symlink
	stat, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot stat path: %w", err)
	}

	if stat.Mode()&os.ModeSymlink == 0 {
		info.IsSymlink = false
		info.FinalTarget = path
		return info, nil
	}

	info.IsSymlink = true

	// Get direct target
	target, err := SafeReadlink(path)
	if err != nil {
		return nil, err
	}
	info.Target = target

	// Resolve full chain
	current := target // Start from the direct target
	visited := make(map[string]bool)
	visited[path] = true // Mark original path as visited

	for depth := 1; depth < MaxSymlinkDepth; depth++ {
		if visited[current] {
			return nil, fmt.Errorf("circular symlink detected: %s", current)
		}
		visited[current] = true

		stat, err := os.Lstat(current)
		if err != nil {
			return nil, fmt.Errorf("cannot stat %s: %w", current, err)
		}

		if stat.Mode()&os.ModeSymlink == 0 {
			// This is the final target
			info.Chain = append(info.Chain, current)
			info.FinalTarget = current
			info.ChainDepth = depth
			return info, nil
		}

		// Still a symlink, add to chain and continue
		info.Chain = append(info.Chain, current)

		nextTarget, err := SafeReadlink(current)
		if err != nil {
			return nil, fmt.Errorf("invalid symlink: %w", err)
		}

		current = nextTarget
	}

	return nil, fmt.Errorf("symlink chain too deep (>%d)", MaxSymlinkDepth)
}

// NoSymlinksInPath checks that no component of the path is a symlink.
// This is useful for ensuring sidecar paths don't contain symlinks,
// which could be exploited in TOCTOU attacks.
// Note: This skips well-known system symlinks (like /var -> /private/var on macOS)
func NoSymlinksInPath(path string) error {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("cannot resolve path: %w", err)
	}

	// Well-known system symlinks that are safe to ignore (macOS-specific)
	systemSymlinks := map[string]bool{
		"/etc":     true, // /etc -> /private/etc on macOS
		"/tmp":     true, // /tmp -> /private/tmp on macOS
		"/var":     true, // /var -> /private/var on macOS
		"/private": false, // /private itself should not be a symlink
	}

	// Check each component
	parts := strings.Split(abs, string(filepath.Separator))
	current := string(filepath.Separator)

	for _, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)

		// Skip well-known system symlinks
		if _, isSystemSymlink := systemSymlinks[current]; isSystemSymlink {
			continue
		}

		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Path doesn't exist yet - OK
			}
			return fmt.Errorf("cannot stat %s: %w", current, err)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in path at %s", current)
		}
	}

	return nil
}

// ValidateSymlinkForShimming performs all necessary checks before shimming a symlink.
// This is the main entry point for validating symlinks during shim installation.
func ValidateSymlinkForShimming(linkPath string) (finalTarget string, err error) {
	// Ensure it's actually a symlink
	info, err := os.Lstat(linkPath)
	if err != nil {
		return "", fmt.Errorf("cannot stat path: %w", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		// Not a symlink - this is fine, just return the original path
		return linkPath, nil
	}

	// Resolve and validate the entire chain
	finalTarget, err = ResolveSymlinkChain(linkPath)
	if err != nil {
		return "", fmt.Errorf("cannot shim unsafe symlink: %w", err)
	}

	return finalTarget, nil
}
