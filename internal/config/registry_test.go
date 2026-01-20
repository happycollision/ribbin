package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/happycollision/ribbin/internal/testsafety"
)

func TestRegistryPath(t *testing.T) {
	path, err := RegistryPath()
	if err != nil {
		t.Fatalf("RegistryPath error: %v", err)
	}

	// Should end with .config/ribbin/registry.json
	if filepath.Base(path) != "registry.json" {
		t.Errorf("expected registry.json, got %s", filepath.Base(path))
	}

	dir := filepath.Dir(path)
	if filepath.Base(dir) != "ribbin" {
		t.Errorf("expected ribbin directory, got %s", filepath.Base(dir))
	}
}

func TestLoadRegistry(t *testing.T) {
	// Create temp home directory
	tmpHome, err := os.MkdirTemp("", "ribbin-test-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	// Save original HOME and set temp
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	t.Run("creates empty registry when file doesn't exist", func(t *testing.T) {
		registry, err := LoadRegistry()
		if err != nil {
			t.Fatalf("LoadRegistry error: %v", err)
		}

		if registry.Wrappers == nil {
			t.Error("Wrappers map is nil")
		}
		if registry.ShellActivations == nil {
			t.Error("ShellActivations map is nil")
		}
		if registry.ConfigActivations == nil {
			t.Error("ConfigActivations map is nil")
		}
		if registry.GlobalActive != false {
			t.Error("GlobalActive should be false by default")
		}
	})

	t.Run("loads existing registry", func(t *testing.T) {
		// Create registry directory
		registryDir := filepath.Join(tmpHome, ".config", "ribbin")
		if err := os.MkdirAll(registryDir, 0755); err != nil {
			t.Fatalf("failed to create registry dir: %v", err)
		}

		// Write a registry file
		registry := Registry{
			Wrappers: map[string]WrapperEntry{
				"cat": {Original: "/usr/bin/cat", Config: "/project/ribbin.jsonc"},
			},
			ShellActivations: map[int]ShellActivationEntry{
				1234: {PID: 1234, ActivatedAt: time.Now()},
			},
			ConfigActivations: map[string]ConfigActivationEntry{
				"/project/ribbin.jsonc": {ActivatedAt: time.Now()},
			},
			GlobalActive: true,
		}

		data, err := json.Marshal(registry)
		if err != nil {
			t.Fatalf("failed to marshal registry: %v", err)
		}

		registryPath := filepath.Join(registryDir, "registry.json")
		if err := os.WriteFile(registryPath, data, 0644); err != nil {
			t.Fatalf("failed to write registry: %v", err)
		}

		loaded, err := LoadRegistry()
		if err != nil {
			t.Fatalf("LoadRegistry error: %v", err)
		}

		if !loaded.GlobalActive {
			t.Error("GlobalActive should be true")
		}
		if _, exists := loaded.Wrappers["cat"]; !exists {
			t.Error("cat wrapper should exist")
		}
		if len(loaded.ShellActivations) != 1 {
			t.Errorf("expected 1 shell activation, got %d", len(loaded.ShellActivations))
		}
		if len(loaded.ConfigActivations) != 1 {
			t.Errorf("expected 1 config activation, got %d", len(loaded.ConfigActivations))
		}
	})

	t.Run("handles corrupted registry", func(t *testing.T) {
		// Create registry directory
		registryDir := filepath.Join(tmpHome, ".config", "ribbin")
		if err := os.MkdirAll(registryDir, 0755); err != nil {
			t.Fatalf("failed to create registry dir: %v", err)
		}

		// Write invalid JSON
		registryPath := filepath.Join(registryDir, "registry.json")
		if err := os.WriteFile(registryPath, []byte("not valid json{"), 0644); err != nil {
			t.Fatalf("failed to write registry: %v", err)
		}

		_, err := LoadRegistry()
		if err == nil {
			t.Error("expected error for corrupted registry")
		}
	})

	t.Run("initializes nil maps for backwards compatibility", func(t *testing.T) {
		// Create registry directory
		registryDir := filepath.Join(tmpHome, ".config", "ribbin")
		if err := os.MkdirAll(registryDir, 0755); err != nil {
			t.Fatalf("failed to create registry dir: %v", err)
		}

		// Write minimal registry (no maps initialized)
		registryPath := filepath.Join(registryDir, "registry.json")
		if err := os.WriteFile(registryPath, []byte(`{"global_active": false}`), 0644); err != nil {
			t.Fatalf("failed to write registry: %v", err)
		}

		loaded, err := LoadRegistry()
		if err != nil {
			t.Fatalf("LoadRegistry error: %v", err)
		}

		if loaded.Wrappers == nil {
			t.Error("Wrappers should be initialized")
		}
		if loaded.ShellActivations == nil {
			t.Error("ShellActivations should be initialized")
		}
		if loaded.ConfigActivations == nil {
			t.Error("ConfigActivations should be initialized")
		}
	})
}

func TestSaveRegistry(t *testing.T) {
	// Create temp home directory
	tmpHome, err := os.MkdirTemp("", "ribbin-test-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	// Save original HOME and set temp
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	t.Run("creates directory if missing", func(t *testing.T) {
		registry := &Registry{
			Wrappers:          make(map[string]WrapperEntry),
			ShellActivations:  make(map[int]ShellActivationEntry),
			ConfigActivations: make(map[string]ConfigActivationEntry),
			GlobalActive:      true,
		}

		if err := SaveRegistry(registry); err != nil {
			t.Fatalf("SaveRegistry error: %v", err)
		}

		// Verify file exists
		path, _ := RegistryPath()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("registry file was not created")
		}
	})

	t.Run("overwrites existing registry", func(t *testing.T) {
		// First save
		registry1 := &Registry{
			Wrappers:          make(map[string]WrapperEntry),
			ShellActivations:  make(map[int]ShellActivationEntry),
			ConfigActivations: make(map[string]ConfigActivationEntry),
			GlobalActive:      false,
		}
		if err := SaveRegistry(registry1); err != nil {
			t.Fatalf("SaveRegistry error: %v", err)
		}

		// Second save with different data
		registry2 := &Registry{
			Wrappers:          map[string]WrapperEntry{"cat": {Original: "/bin/cat"}},
			ShellActivations:  make(map[int]ShellActivationEntry),
			ConfigActivations: make(map[string]ConfigActivationEntry),
			GlobalActive:      true,
		}
		if err := SaveRegistry(registry2); err != nil {
			t.Fatalf("SaveRegistry error: %v", err)
		}

		// Load and verify
		loaded, err := LoadRegistry()
		if err != nil {
			t.Fatalf("LoadRegistry error: %v", err)
		}
		if !loaded.GlobalActive {
			t.Error("GlobalActive should be true")
		}
		if _, exists := loaded.Wrappers["cat"]; !exists {
			t.Error("cat wrapper should exist")
		}
	})
}

func TestPruneDeadShellActivations(t *testing.T) {
	registry := &Registry{
		Wrappers: make(map[string]WrapperEntry),
		ShellActivations: map[int]ShellActivationEntry{
			1:        {PID: 1, ActivatedAt: time.Now()},        // PID 1 always exists
			99999999: {PID: 99999999, ActivatedAt: time.Now()}, // Very unlikely to exist
		},
		ConfigActivations: make(map[string]ConfigActivationEntry),
		GlobalActive:      false,
	}

	registry.PruneDeadShellActivations()

	// PID 1 (init/launchd) should still exist
	if _, exists := registry.ShellActivations[1]; !exists {
		t.Error("PID 1 should still exist after pruning")
	}

	// Dead PID should be removed
	if _, exists := registry.ShellActivations[99999999]; exists {
		t.Error("dead PID should be removed after pruning")
	}
}

func TestConfigActivationHelpers(t *testing.T) {
	registry := &Registry{
		Wrappers:          make(map[string]WrapperEntry),
		ShellActivations:  make(map[int]ShellActivationEntry),
		ConfigActivations: make(map[string]ConfigActivationEntry),
		GlobalActive:      false,
	}

	t.Run("AddConfigActivation adds config", func(t *testing.T) {
		registry.AddConfigActivation("/path/to/ribbin.jsonc")

		if _, exists := registry.ConfigActivations["/path/to/ribbin.jsonc"]; !exists {
			t.Error("config should be added")
		}
	})

	t.Run("RemoveConfigActivation removes config", func(t *testing.T) {
		registry.RemoveConfigActivation("/path/to/ribbin.jsonc")

		if _, exists := registry.ConfigActivations["/path/to/ribbin.jsonc"]; exists {
			t.Error("config should be removed")
		}
	})

	t.Run("ClearConfigActivations clears all", func(t *testing.T) {
		registry.AddConfigActivation("/path/a.toml")
		registry.AddConfigActivation("/path/b.toml")
		registry.ClearConfigActivations()

		if len(registry.ConfigActivations) != 0 {
			t.Errorf("expected 0 config activations, got %d", len(registry.ConfigActivations))
		}
	})
}

func TestShellActivationHelpers(t *testing.T) {
	registry := &Registry{
		Wrappers:          make(map[string]WrapperEntry),
		ShellActivations:  make(map[int]ShellActivationEntry),
		ConfigActivations: make(map[string]ConfigActivationEntry),
		GlobalActive:      false,
	}

	t.Run("AddShellActivation adds shell", func(t *testing.T) {
		registry.AddShellActivation(12345)

		entry, exists := registry.ShellActivations[12345]
		if !exists {
			t.Error("shell activation should be added")
		}
		if entry.PID != 12345 {
			t.Errorf("expected PID 12345, got %d", entry.PID)
		}
	})

	t.Run("RemoveShellActivation removes shell", func(t *testing.T) {
		registry.RemoveShellActivation(12345)

		if _, exists := registry.ShellActivations[12345]; exists {
			t.Error("shell activation should be removed")
		}
	})

	t.Run("ClearShellActivations clears all", func(t *testing.T) {
		registry.AddShellActivation(111)
		registry.AddShellActivation(222)
		registry.ClearShellActivations()

		if len(registry.ShellActivations) != 0 {
			t.Errorf("expected 0 shell activations, got %d", len(registry.ShellActivations))
		}
	})
}

func TestProcessExists(t *testing.T) {
	t.Run("returns true for current process", func(t *testing.T) {
		if !processExists(os.Getpid()) {
			t.Error("current process should exist")
		}
	})

	t.Run("returns true for PID 1", func(t *testing.T) {
		// PID 1 is always the init process
		if !processExists(1) {
			t.Error("PID 1 should exist")
		}
	})

	t.Run("returns false for non-existent PID", func(t *testing.T) {
		// Use a very high PID that's unlikely to exist
		if processExists(99999999) {
			t.Error("PID 99999999 should not exist")
		}
	})
}
