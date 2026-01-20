package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ExtendsRef represents a parsed extends reference.
// Extends references allow scopes to inherit shims from other sources.
type ExtendsRef struct {
	// FilePath is the resolved absolute path to an external config file.
	// Empty for same-file references (IsLocal=true).
	FilePath string
	// Fragment identifies what to inherit: "root" for root shims,
	// or "root.scope-name" for a specific scope's shims.
	// Empty when inheriting an entire external file.
	Fragment string
	// IsLocal is true for same-file references ("root" or "root.scope-name").
	IsLocal bool
}

// ParseExtendsRef parses an extends reference string and returns an ExtendsRef.
// The configDir is the directory containing the TOML file with the extends directive,
// used to resolve relative file paths.
//
// Reference patterns:
//   - "root" → local, fragment="root"
//   - "root.backend" → local, fragment="root.backend"
//   - "../other.toml" → file path resolved relative to configDir, fragment="" (entire file)
//   - "./file.toml#root.x" → file path resolved, fragment="root.x"
//   - "/abs/path/ribbin.toml" → absolute path, fragment=""
func ParseExtendsRef(ref string, configDir string) (*ExtendsRef, error) {
	if ref == "" {
		return nil, fmt.Errorf("extends reference cannot be empty")
	}

	// Check for local references first: "root" or "root.scope-name"
	if isLocalRef(ref) {
		return &ExtendsRef{
			FilePath: "",
			Fragment: ref,
			IsLocal:  true,
		}, nil
	}

	// It's a file reference, possibly with a fragment
	filePath, fragment := splitFileAndFragment(ref)

	if filePath == "" {
		return nil, fmt.Errorf("invalid extends reference %q: missing file path", ref)
	}

	// Resolve the file path
	resolvedPath, err := resolveFilePath(filePath, configDir)
	if err != nil {
		return nil, fmt.Errorf("invalid extends reference %q: %w", ref, err)
	}

	return &ExtendsRef{
		FilePath: resolvedPath,
		Fragment: fragment,
		IsLocal:  false,
	}, nil
}

// isLocalRef returns true if the reference is a local same-file reference.
// Local references start with "root" and optionally have a scope suffix like "root.scope-name".
// They do NOT start with "/", "./", or "../" (file paths).
func isLocalRef(ref string) bool {
	// Must start with "root"
	if !strings.HasPrefix(ref, "root") {
		return false
	}

	// "root" by itself is local
	if ref == "root" {
		return true
	}

	// "root.something" is local (scope reference)
	if len(ref) > 4 && ref[4] == '.' {
		// Ensure there's something after the dot
		return len(ref) > 5
	}

	return false
}

// splitFileAndFragment splits a file reference into path and fragment parts.
// e.g., "./file.toml#root.x" → ("./file.toml", "root.x")
// e.g., "../other.toml" → ("../other.toml", "")
func splitFileAndFragment(ref string) (filePath, fragment string) {
	// Find the fragment separator
	idx := strings.LastIndex(ref, "#")
	if idx == -1 {
		return ref, ""
	}
	return ref[:idx], ref[idx+1:]
}

// resolveFilePath resolves a file path relative to configDir.
// Absolute paths are returned as-is. Relative paths are resolved from configDir.
func resolveFilePath(filePath string, configDir string) (string, error) {
	if filepath.IsAbs(filePath) {
		return filepath.Clean(filePath), nil
	}

	// Relative paths must start with "./" or "../"
	if !strings.HasPrefix(filePath, "./") && !strings.HasPrefix(filePath, "../") {
		return "", fmt.Errorf("relative path must start with './' or '../', got %q", filePath)
	}

	// Resolve relative to configDir
	resolved := filepath.Join(configDir, filePath)
	return filepath.Clean(resolved), nil
}
