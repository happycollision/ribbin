package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ValidateHomeDir returns a validated home directory path.
// It verifies the path exists, is a directory, is absolute, and is owned by the current user.
func ValidateHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot get home directory: %w", err)
	}

	// Verify it's an absolute path
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("home directory is not absolute: %s", home)
	}

	// Check for path traversal sequences
	if strings.Contains(home, "..") {
		return "", fmt.Errorf("home directory contains path traversal: %s", home)
	}

	// Verify it exists and is a directory
	info, err := os.Stat(home)
	if err != nil {
		return "", fmt.Errorf("home directory does not exist: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("home is not a directory: %s", home)
	}

	// Verify ownership (should be owned by current user)
	if err := verifyOwnership(home); err != nil {
		return "", fmt.Errorf("home directory ownership issue: %w", err)
	}

	return home, nil
}

// ValidateEnvPath validates and canonicalizes a path from an environment variable.
// It returns an error if the variable is not set or contains suspicious content.
func ValidateEnvPath(envVar string) (string, error) {
	value := os.Getenv(envVar)
	if value == "" {
		return "", fmt.Errorf("environment variable %s not set", envVar)
	}

	// Check for path traversal
	if strings.Contains(value, "..") {
		return "", fmt.Errorf("path traversal in %s: %s", envVar, value)
	}

	// Check for null bytes
	if strings.Contains(value, "\x00") {
		return "", fmt.Errorf("null byte in %s: suspicious input", envVar)
	}

	// Canonicalize
	clean := filepath.Clean(value)
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("cannot resolve env path: %w", err)
	}

	return abs, nil
}

// GetConfigDir returns a validated XDG config directory for ribbin.
// It follows the XDG Base Directory specification.
func GetConfigDir() (string, error) {
	// Check XDG_CONFIG_HOME first
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		validated, err := ValidateEnvPath("XDG_CONFIG_HOME")
		if err != nil {
			return "", fmt.Errorf("invalid XDG_CONFIG_HOME: %w", err)
		}

		// Verify it exists or can be created
		info, err := os.Stat(validated)
		if err == nil && !info.IsDir() {
			return "", fmt.Errorf("XDG_CONFIG_HOME is not a directory: %s", validated)
		}

		return filepath.Join(validated, "ribbin"), nil
	}

	// Fall back to ~/.config
	home, err := ValidateHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", "ribbin"), nil
}

// GetStateDir returns a validated XDG state directory for ribbin.
// It follows the XDG Base Directory specification.
func GetStateDir() (string, error) {
	// Check XDG_STATE_HOME first
	if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" {
		validated, err := ValidateEnvPath("XDG_STATE_HOME")
		if err != nil {
			return "", fmt.Errorf("invalid XDG_STATE_HOME: %w", err)
		}

		// Verify it exists or can be created
		info, err := os.Stat(validated)
		if err == nil && !info.IsDir() {
			return "", fmt.Errorf("XDG_STATE_HOME is not a directory: %s", validated)
		}

		return filepath.Join(validated, "ribbin"), nil
	}

	// Fall back to ~/.local/state
	home, err := ValidateHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "state", "ribbin"), nil
}

// SafeExpandPath expands ~ prefix and validates the result.
// It returns a canonicalized absolute path.
func SafeExpandPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return "", fmt.Errorf("null byte in path: suspicious input")
	}

	// Expand ~ prefix
	if strings.HasPrefix(path, "~/") {
		home, err := ValidateHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	} else if path == "~" {
		return ValidateHomeDir()
	}

	// Check for path traversal before cleaning
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path traversal detected: %s", path)
	}

	clean := filepath.Clean(path)
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path: %w", err)
	}

	return abs, nil
}

// verifyOwnership checks if a file/directory is owned by the current user.
func verifyOwnership(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		// Can't check ownership on this platform, allow it
		return nil
	}

	currentUID := uint32(os.Getuid())
	if stat.Uid != currentUID {
		return fmt.Errorf("not owned by current user (uid %d != %d)", stat.Uid, currentUID)
	}

	return nil
}

// ValidateRegistryPath returns a validated path for the ribbin registry file.
func ValidateRegistryPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot get config directory: %w", err)
	}

	return filepath.Join(configDir, "registry.json"), nil
}

// EnsureConfigDir creates the ribbin config directory if it doesn't exist.
// It returns the validated path to the directory.
func EnsureConfigDir() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create config directory: %w", err)
	}

	return configDir, nil
}

// EnsureStateDir creates the ribbin state directory if it doesn't exist.
// It returns the validated path to the directory.
func EnsureStateDir() (string, error) {
	stateDir, err := GetStateDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return "", fmt.Errorf("cannot create state directory: %w", err)
	}

	return stateDir, nil
}
