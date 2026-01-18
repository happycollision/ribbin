package config

import (
	"bytes"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// AddShim adds a new shim configuration to the ribbin.toml file.
// Returns an error if the command already exists.
func AddShim(configPath, cmdName string, shimConfig ShimConfig) error {
	// Load existing config
	config, err := LoadProjectConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize shims map if nil
	if config.Shims == nil {
		config.Shims = make(map[string]ShimConfig)
	}

	// Check if command already exists
	if _, exists := config.Shims[cmdName]; exists {
		return fmt.Errorf("shim for command '%s' already exists", cmdName)
	}

	// Add new shim
	config.Shims[cmdName] = shimConfig

	// Write atomically with backup
	return atomicWrite(configPath, config)
}

// RemoveShim removes a shim configuration from the ribbin.toml file.
// Returns an error if the command doesn't exist.
func RemoveShim(configPath, cmdName string) error {
	// Load existing config
	config, err := LoadProjectConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if command exists
	if _, exists := config.Shims[cmdName]; !exists {
		return fmt.Errorf("shim for command '%s' not found", cmdName)
	}

	// Remove shim
	delete(config.Shims, cmdName)

	// Write atomically with backup
	return atomicWrite(configPath, config)
}

// UpdateShim updates an existing shim configuration in the ribbin.toml file.
// Returns an error if the command doesn't exist.
func UpdateShim(configPath, cmdName string, shimConfig ShimConfig) error {
	// Load existing config
	config, err := LoadProjectConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if command exists
	if _, exists := config.Shims[cmdName]; !exists {
		return fmt.Errorf("shim for command '%s' not found", cmdName)
	}

	// Update shim
	config.Shims[cmdName] = shimConfig

	// Write atomically with backup
	return atomicWrite(configPath, config)
}

// atomicWrite writes the config to disk atomically with backup and validation.
// This ensures that the config file is never left in a corrupted state.
func atomicWrite(configPath string, config *ProjectConfig) error {
	// Create backup if original file exists
	if _, err := os.Stat(configPath); err == nil {
		backup := configPath + ".backup"
		data, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read original file for backup: %w", err)
		}
		if err := os.WriteFile(backup, data, 0644); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Encode config to TOML
	var buf bytes.Buffer
	encoder := toml.NewEncoder(&buf)
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	data := buf.Bytes()

	// Write to temporary file
	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Validate the written file by trying to parse it
	var testConfig ProjectConfig
	if _, err := toml.DecodeFile(tmpPath, &testConfig); err != nil {
		// Cleanup temp file on validation failure
		os.Remove(tmpPath)
		return fmt.Errorf("validation failed: %w", err)
	}

	// Atomic rename from temp to final path
	if err := os.Rename(tmpPath, configPath); err != nil {
		// Cleanup temp file on rename failure
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
