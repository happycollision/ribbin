package config

import (
	"encoding/json"
	"os"
	"syscall"
	"time"

	"github.com/happycollision/ribbin/internal/security"
)

// WrapperEntry tracks an installed wrapper in the registry
type WrapperEntry struct {
	// Original is the path to the original command being wrapped
	Original string `json:"original"`
	// Config is the path to the ribbin.jsonc that defines this wrapper
	Config string `json:"config"`
}

// ShellActivationEntry tracks an active ribbin shell session
type ShellActivationEntry struct {
	// PID of the shell process that activated ribbin
	PID int `json:"pid"`
	// ActivatedAt is when the session was activated
	ActivatedAt time.Time `json:"activated_at"`
}

// ConfigActivationEntry tracks activation of a specific config file
type ConfigActivationEntry struct {
	// ActivatedAt is when the config was activated
	ActivatedAt time.Time `json:"activated_at"`
}

// Registry is the global ribbin state stored in ~/.config/ribbin/registry.json
type Registry struct {
	// Wrappers maps command names to their wrapper entries
	Wrappers map[string]WrapperEntry `json:"wrappers"`
	// ShellActivations tracks active shell sessions (all configs fire for this shell)
	ShellActivations map[int]ShellActivationEntry `json:"shell_activations"`
	// ConfigActivations tracks per-config activation (config fires for all shells)
	ConfigActivations map[string]ConfigActivationEntry `json:"config_activations"`
	// GlobalActive indicates if ribbin is globally enabled (everything fires everywhere)
	GlobalActive bool `json:"global_active"`
}

// RegistryPath returns the path to the global registry file.
// It uses validated environment variables to prevent injection attacks.
func RegistryPath() (string, error) {
	return security.ValidateRegistryPath()
}

// LoadRegistry loads the global registry, creating an empty one if it doesn't exist
func LoadRegistry() (*Registry, error) {
	path, err := RegistryPath()
	if err != nil {
		return nil, err
	}

	// Check if file exists first (before acquiring lock)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Return empty registry if file doesn't exist
		return &Registry{
			Wrappers:          make(map[string]WrapperEntry),
			ShellActivations:  make(map[int]ShellActivationEntry),
			ConfigActivations: make(map[string]ConfigActivationEntry),
			GlobalActive:      false,
		}, nil
	}

	// SHARED LOCK for reading (allows concurrent reads)
	lock, err := security.AcquireSharedLock(path, 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer lock.Release()

	// Read registry
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, err
	}

	// Initialize maps if nil (for backwards compatibility)
	if registry.Wrappers == nil {
		registry.Wrappers = make(map[string]WrapperEntry)
	}
	if registry.ShellActivations == nil {
		registry.ShellActivations = make(map[int]ShellActivationEntry)
	}
	if registry.ConfigActivations == nil {
		registry.ConfigActivations = make(map[string]ConfigActivationEntry)
	}

	return &registry, nil
}

// PruneDeadShellActivations removes shell activation entries for processes that no longer exist.
func (r *Registry) PruneDeadShellActivations() {
	for pid := range r.ShellActivations {
		if !processExists(pid) {
			delete(r.ShellActivations, pid)
		}
	}
}

// ClearShellActivations removes all shell activations.
func (r *Registry) ClearShellActivations() {
	r.ShellActivations = make(map[int]ShellActivationEntry)
}

// ClearConfigActivations removes all config activations.
func (r *Registry) ClearConfigActivations() {
	r.ConfigActivations = make(map[string]ConfigActivationEntry)
}

// AddConfigActivation adds a config to the activation set.
func (r *Registry) AddConfigActivation(configPath string) {
	if r.ConfigActivations == nil {
		r.ConfigActivations = make(map[string]ConfigActivationEntry)
	}
	r.ConfigActivations[configPath] = ConfigActivationEntry{
		ActivatedAt: time.Now(),
	}
}

// RemoveConfigActivation removes a config from the activation set.
func (r *Registry) RemoveConfigActivation(configPath string) {
	delete(r.ConfigActivations, configPath)
}

// AddShellActivation adds a shell activation for the given PID.
func (r *Registry) AddShellActivation(pid int) {
	if r.ShellActivations == nil {
		r.ShellActivations = make(map[int]ShellActivationEntry)
	}
	r.ShellActivations[pid] = ShellActivationEntry{
		PID:         pid,
		ActivatedAt: time.Now(),
	}
}

// RemoveShellActivation removes a shell activation for the given PID.
func (r *Registry) RemoveShellActivation(pid int) {
	delete(r.ShellActivations, pid)
}

// processExists checks if a process with the given PID exists.
func processExists(pid int) bool {
	// Sending signal 0 checks if process exists without affecting it
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	// EPERM means the process exists but we don't have permission to signal it
	// (common on macOS for PID 1/launchd)
	if err == syscall.EPERM {
		return true
	}
	// ESRCH means no such process
	return false
}

// SaveRegistry writes the registry to disk, creating directories as needed
func SaveRegistry(r *Registry) error {
	path, err := RegistryPath()
	if err != nil {
		return err
	}

	// Ensure directory exists (needed before lock file can be created)
	if _, err := security.EnsureConfigDir(); err != nil {
		return err
	}

	// LOCK REGISTRY FILE
	lock, err := security.AcquireLock(path, 5*time.Second)
	if err != nil {
		return err
	}
	defer lock.Release()

	// Write to temp file first
	tmpPath := path + ".tmp"
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	// Remove destination if it exists (safe because we hold the lock)
	// This is necessary because AtomicRename uses O_EXCL which fails if file exists
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			os.Remove(tmpPath) // Cleanup temp file
			return err
		}
	}

	// ATOMIC RENAME
	if err := security.AtomicRename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Cleanup
		return err
	}

	return nil
}
