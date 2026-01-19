package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/happycollision/ribbin/internal/security"
)

// ErrInvalidScopePath is returned when a scope path is invalid
var ErrInvalidScopePath = errors.New("invalid scope path")

// PassthroughConfig defines conditions under which a shim should pass through to the original command
type PassthroughConfig struct {
	// Invocation is a list of exact strings to match against the parent process invocation
	Invocation []string `toml:"invocation,omitempty"`
	// InvocationRegexp is a list of regular expressions to match against the parent process invocation
	InvocationRegexp []string `toml:"invocationRegexp,omitempty"`
}

// ShimConfig defines the behavior for a shimmed command
type ShimConfig struct {
	// Action is the behavior when the command is invoked: "block", "warn", "redirect"
	Action string `toml:"action"`
	// Message is displayed when the command is blocked or warned
	Message string `toml:"message,omitempty"`
	// Paths restricts the shim to specific binary paths
	Paths []string `toml:"paths,omitempty"`
	// Redirect specifies the alternative command to execute (for "redirect" action)
	Redirect string `toml:"redirect,omitempty"`
	// Passthrough defines conditions for passing through to the original command
	Passthrough *PassthroughConfig `toml:"passthrough,omitempty"`
}

// ScopeConfig defines a scoped configuration that applies to a specific path
type ScopeConfig struct {
	// Path is the directory path this scope applies to (relative to config dir, defaults to ".")
	Path string `toml:"path,omitempty"`
	// Extends is a list of references to inherit shims from (see epic ribbin-3gj for syntax)
	Extends []string `toml:"extends,omitempty"`
	// Shims maps command names to their shim configurations within this scope
	Shims map[string]ShimConfig `toml:"shims"`
}

// ProjectConfig represents a ribbin.toml project configuration file
type ProjectConfig struct {
	// Shims maps command names to their shim configurations (root-level shims)
	Shims map[string]ShimConfig `toml:"shims"`
	// Scopes maps scope names to their scoped configurations
	Scopes map[string]ScopeConfig `toml:"scopes"`
}

// FindProjectConfig walks up from the current working directory to find ribbin.toml
// Returns the path to ribbin.toml if found, or empty string if not found
func FindProjectConfig() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		configPath := filepath.Join(dir, "ribbin.toml")
		if _, err := os.Stat(configPath); err == nil {
			// Validate config path before returning
			if err := security.ValidateConfigPath(configPath); err != nil {
				return "", fmt.Errorf("unsafe config file at %s: %w", configPath, err)
			}
			return configPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding config
			return "", nil
		}
		dir = parent
	}
}

// LoadProjectConfig loads a project configuration from the specified path
func LoadProjectConfig(path string) (*ProjectConfig, error) {
	// Validate config path before loading
	if err := security.ValidateConfigPath(path); err != nil {
		return nil, fmt.Errorf("invalid config path: %w", err)
	}

	var config ProjectConfig
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return nil, err
	}

	// Validate scope paths
	configDir := filepath.Dir(path)
	for name, scope := range config.Scopes {
		if err := ValidateScopePath(scope.Path, configDir); err != nil {
			return nil, fmt.Errorf("scope %q: %w", name, err)
		}
	}

	return &config, nil
}

// ValidateScopePath validates that a scope path is safe.
// It must not contain ".." traversal and must resolve to a descendant of configDir.
// Empty path is valid (defaults to ".").
func ValidateScopePath(scopePath string, configDir string) error {
	// Empty path defaults to ".", which is always valid
	if scopePath == "" || scopePath == "." {
		return nil
	}

	// Reject paths containing ".." component
	// This catches "../foo", "foo/../bar", "foo/..", etc.
	cleaned := filepath.Clean(scopePath)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(filepath.Separator)+"..") {
		return fmt.Errorf("%w: path %q contains parent directory traversal", ErrInvalidScopePath, scopePath)
	}

	// For absolute paths, verify they're under configDir
	if filepath.IsAbs(scopePath) {
		absConfigDir, err := filepath.Abs(configDir)
		if err != nil {
			return fmt.Errorf("%w: cannot resolve config directory: %v", ErrInvalidScopePath, err)
		}
		// Check that the scope path is under the config directory
		rel, err := filepath.Rel(absConfigDir, scopePath)
		if err != nil {
			return fmt.Errorf("%w: cannot determine relative path: %v", ErrInvalidScopePath, err)
		}
		if strings.HasPrefix(rel, "..") {
			return fmt.Errorf("%w: absolute path %q is outside config directory", ErrInvalidScopePath, scopePath)
		}
	}

	return nil
}
