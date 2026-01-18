package shim

import (
	"fmt"
	"os"
	"path/filepath"
)

// resolveRedirectScript resolves a redirect script path relative to the config file
// or as an absolute path. Returns absolute path and error if not found/executable.
//
// If scriptPath is absolute, it validates the path directly.
// If scriptPath is relative, it resolves relative to the directory containing configPath.
func resolveRedirectScript(scriptPath string, configPath string) (string, error) {
	// Handle absolute paths
	if filepath.IsAbs(scriptPath) {
		return validateExecutable(scriptPath)
	}

	// Resolve relative to config directory
	configDir := filepath.Dir(configPath)
	absPath := filepath.Join(configDir, scriptPath)
	return validateExecutable(absPath)
}

// validateExecutable checks if a file exists and is executable.
// Returns the path if valid, or an error with a helpful message if:
// - The file doesn't exist
// - The path is a directory rather than a regular file
// - The file exists but is not executable
func validateExecutable(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("redirect script not found: %s", path)
		}
		return "", fmt.Errorf("cannot access redirect script: %w", err)
	}

	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("redirect script is not a regular file: %s", path)
	}

	// Check if executable (Unix permission bit)
	if info.Mode().Perm()&0111 == 0 {
		return "", fmt.Errorf("redirect script is not executable: %s (run: chmod +x %s)", path, path)
	}

	return path, nil
}
