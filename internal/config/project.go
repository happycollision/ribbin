package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

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
}

// ProjectConfig represents a ribbin.toml project configuration file
type ProjectConfig struct {
	// Shims maps command names to their shim configurations
	Shims map[string]ShimConfig `toml:"shims"`
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
	var config ProjectConfig
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
