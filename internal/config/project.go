package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/happycollision/ribbin/internal/security"
	"github.com/tailscale/hujson"
)

// ErrInvalidScopePath is returned when a scope path is invalid
var ErrInvalidScopePath = errors.New("invalid scope path")

// PassthroughConfig defines conditions under which a shim should pass through to the original command
type PassthroughConfig struct {
	// Invocation is a list of exact strings to match against the parent process invocation
	Invocation []string `json:"invocation,omitempty"`
	// InvocationRegexp is a list of regular expressions to match against the parent process invocation
	InvocationRegexp []string `json:"invocationRegexp,omitempty"`
}

// WrapperConfig defines the behavior for a wrapped command
type WrapperConfig struct {
	// Action is the behavior when the command is invoked: "block", "warn", "redirect"
	Action string `json:"action"`
	// Message is displayed when the command is blocked or warned
	Message string `json:"message,omitempty"`
	// Paths restricts the wrapper to specific binary paths
	Paths []string `json:"paths,omitempty"`
	// Redirect specifies the alternative command to execute (for "redirect" action)
	Redirect string `json:"redirect,omitempty"`
	// Passthrough defines conditions for passing through to the original command
	Passthrough *PassthroughConfig `json:"passthrough,omitempty"`
}

// ShimConfig is an alias for backwards compatibility during migration
type ShimConfig = WrapperConfig

// ScopeConfig defines a scoped configuration that applies to a specific path
type ScopeConfig struct {
	// Path is the directory path this scope applies to (relative to config dir, defaults to ".")
	Path string `json:"path,omitempty"`
	// Extends is a list of references to inherit wrappers from (see epic ribbin-3gj for syntax)
	Extends []string `json:"extends,omitempty"`
	// Wrappers maps command names to their wrapper configurations within this scope
	Wrappers map[string]WrapperConfig `json:"wrappers,omitempty"`
}

// ProjectConfig represents a ribbin.jsonc project configuration file
type ProjectConfig struct {
	// Schema is the JSON Schema URL for editor support
	Schema string `json:"$schema,omitempty"`
	// Wrappers maps command names to their wrapper configurations (root-level wrappers)
	Wrappers map[string]WrapperConfig `json:"wrappers,omitempty"`
	// Scopes maps scope names to their scoped configurations
	Scopes map[string]ScopeConfig `json:"scopes,omitempty"`
}

// ConfigFileName is the standard project configuration file name
const ConfigFileName = "ribbin.jsonc"

// LocalConfigFileName is the user-local override configuration file name.
// When present, it takes precedence over the standard config file.
const LocalConfigFileName = "ribbin.local.jsonc"

// FindProjectConfig walks up from the current working directory to find a ribbin config.
// It prefers ribbin.local.jsonc over ribbin.jsonc when both exist in the same directory.
// Returns the path to the config if found, or empty string if not found.
func FindProjectConfig() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		// Check for local config first (takes precedence)
		localConfigPath := filepath.Join(dir, LocalConfigFileName)
		if _, err := os.Stat(localConfigPath); err == nil {
			// Validate config path before returning
			if err := security.ValidateConfigPath(localConfigPath); err != nil {
				return "", fmt.Errorf("unsafe config file at %s: %w", localConfigPath, err)
			}
			return localConfigPath, nil
		}

		// Fall back to standard config
		configPath := filepath.Join(dir, ConfigFileName)
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

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse JSONC (JSON with comments) to standard JSON
	standardJSON, err := hujson.Standardize(data)
	if err != nil {
		return nil, fmt.Errorf("invalid JSONC: %w", err)
	}

	// Unmarshal JSON into config struct
	var config ProjectConfig
	if err := json.Unmarshal(standardJSON, &config); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
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
