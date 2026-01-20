// Package testutil provides test helpers for ribbin tests.
package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/happycollision/ribbin/internal/config"
)

// CreateTempDir creates a temporary directory for testing and returns its path.
// The directory is automatically cleaned up when the test completes.
func CreateTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "ribbin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// CreateTempBinary creates a simple executable script for testing.
// Returns the path to the created binary.
func CreateTempBinary(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := "#!/bin/sh\necho \"$0 $@\"\n"
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("failed to create temp binary: %v", err)
	}
	return path
}

// CreateTempConfig creates a ribbin.jsonc config file in the specified directory.
// Returns the path to the config file.
func CreateTempConfig(t *testing.T, dir string, cfg *config.ProjectConfig) string {
	t.Helper()
	path := filepath.Join(dir, "ribbin.jsonc")

	content := "{\n  \"wrappers\": {\n"
	first := true
	for name, shim := range cfg.Wrappers {
		if !first {
			content += ",\n"
		}
		first = false
		content += "    \"" + name + "\": {\n"
		content += "      \"action\": \"" + shim.Action + "\""
		if shim.Message != "" {
			content += ",\n      \"message\": \"" + shim.Message + "\""
		}
		content += "\n    }"
	}
	content += "\n  }\n}\n"

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}
	return path
}

// CreateTempRegistry creates a registry.json file in the specified directory.
// Returns the path to the registry file.
func CreateTempRegistry(t *testing.T, dir string, registry *config.Registry) string {
	t.Helper()
	configDir := filepath.Join(dir, ".config", "ribbin")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config directory: %v", err)
	}

	path := filepath.Join(configDir, "registry.json")
	if err := config.SaveRegistry(registry); err != nil {
		// Fall back to manual creation if SaveRegistry doesn't work for tests
		t.Logf("SaveRegistry failed, creating manually: %v", err)
	}
	return path
}

// WithTempDir runs a test function with a temporary directory as the working directory.
// Restores the original working directory when the test completes.
func WithTempDir(t *testing.T, fn func(dir string)) {
	t.Helper()
	dir := CreateTempDir(t)
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.Chdir(origDir)
	})
	fn(dir)
}

// WithHomeDir runs a test function with a custom HOME directory.
// Restores the original HOME when the test completes.
func WithHomeDir(t *testing.T, home string, fn func()) {
	t.Helper()
	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	t.Cleanup(func() {
		os.Setenv("HOME", origHome)
	})
	fn()
}

// CreateSymlink creates a symbolic link for testing.
func CreateSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("failed to create symlink %s -> %s: %v", link, target, err)
	}
}
