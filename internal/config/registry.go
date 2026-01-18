package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// ShimEntry tracks an installed shim in the registry
type ShimEntry struct {
	// Original is the path to the original command being shimmed
	Original string `json:"original"`
	// Config is the path to the ribbin.json that defines this shim
	Config string `json:"config"`
}

// ActivationEntry tracks an active ribbin session
type ActivationEntry struct {
	// PID of the shell process that activated ribbin
	PID int `json:"pid"`
	// ActivatedAt is when the session was activated
	ActivatedAt time.Time `json:"activated_at"`
}

// Registry is the global ribbin state stored in ~/.config/ribbin/registry.json
type Registry struct {
	// Shims maps command names to their shim entries
	Shims map[string]ShimEntry `json:"shims"`
	// Activations tracks active shell sessions
	Activations map[int]ActivationEntry `json:"activations"`
	// GlobalOn indicates if ribbin is globally enabled
	GlobalOn bool `json:"global_on"`
}

// RegistryPath returns the path to the global registry file
func RegistryPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "ribbin", "registry.json"), nil
}

// LoadRegistry loads the global registry, creating an empty one if it doesn't exist
func LoadRegistry() (*Registry, error) {
	path, err := RegistryPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Return empty registry if file doesn't exist
		return &Registry{
			Shims:       make(map[string]ShimEntry),
			Activations: make(map[int]ActivationEntry),
			GlobalOn:    false,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, err
	}

	// Initialize maps if nil (for backwards compatibility)
	if registry.Shims == nil {
		registry.Shims = make(map[string]ShimEntry)
	}
	if registry.Activations == nil {
		registry.Activations = make(map[int]ActivationEntry)
	}

	return &registry, nil
}

// PruneDeadActivations removes activation entries for processes that no longer exist.
func (r *Registry) PruneDeadActivations() {
	for pid := range r.Activations {
		if !processExists(pid) {
			delete(r.Activations, pid)
		}
	}
}

// processExists checks if a process with the given PID exists.
func processExists(pid int) bool {
	// Sending signal 0 checks if process exists without affecting it
	err := syscall.Kill(pid, 0)
	return err == nil
}

// SaveRegistry writes the registry to disk, creating directories as needed
func SaveRegistry(r *Registry) error {
	path, err := RegistryPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
