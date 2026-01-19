package config

import (
	"encoding/json"
	"os"
	"syscall"
	"time"

	"github.com/happycollision/ribbin/internal/security"
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
			Shims:       make(map[string]ShimEntry),
			Activations: make(map[int]ActivationEntry),
			GlobalOn:    false,
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
