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

		if registry.Shims == nil {
			t.Error("Shims map is nil")
		}
		if registry.Activations == nil {
			t.Error("Activations map is nil")
		}
		if registry.GlobalOn != false {
			t.Error("GlobalOn should be false by default")
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
			Shims: map[string]ShimEntry{
				"cat": {Original: "/usr/bin/cat", Config: "/project/ribbin.toml"},
			},
			Activations: map[int]ActivationEntry{
				1234: {PID: 1234, ActivatedAt: time.Now()},
			},
			GlobalOn: true,
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

		if !loaded.GlobalOn {
			t.Error("GlobalOn should be true")
		}
		if _, exists := loaded.Shims["cat"]; !exists {
			t.Error("cat shim should exist")
		}
		if len(loaded.Activations) != 1 {
			t.Errorf("expected 1 activation, got %d", len(loaded.Activations))
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
		if err := os.WriteFile(registryPath, []byte(`{"global_on": false}`), 0644); err != nil {
			t.Fatalf("failed to write registry: %v", err)
		}

		loaded, err := LoadRegistry()
		if err != nil {
			t.Fatalf("LoadRegistry error: %v", err)
		}

		if loaded.Shims == nil {
			t.Error("Shims should be initialized")
		}
		if loaded.Activations == nil {
			t.Error("Activations should be initialized")
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
			Shims:       make(map[string]ShimEntry),
			Activations: make(map[int]ActivationEntry),
			GlobalOn:    true,
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
			Shims:       make(map[string]ShimEntry),
			Activations: make(map[int]ActivationEntry),
			GlobalOn:    false,
		}
		if err := SaveRegistry(registry1); err != nil {
			t.Fatalf("SaveRegistry error: %v", err)
		}

		// Second save with different data
		registry2 := &Registry{
			Shims:       map[string]ShimEntry{"cat": {Original: "/bin/cat"}},
			Activations: make(map[int]ActivationEntry),
			GlobalOn:    true,
		}
		if err := SaveRegistry(registry2); err != nil {
			t.Fatalf("SaveRegistry error: %v", err)
		}

		// Load and verify
		loaded, err := LoadRegistry()
		if err != nil {
			t.Fatalf("LoadRegistry error: %v", err)
		}
		if !loaded.GlobalOn {
			t.Error("GlobalOn should be true")
		}
		if _, exists := loaded.Shims["cat"]; !exists {
			t.Error("cat shim should exist")
		}
	})
}

func TestPruneDeadActivations(t *testing.T) {
	registry := &Registry{
		Shims: make(map[string]ShimEntry),
		Activations: map[int]ActivationEntry{
			1:        {PID: 1, ActivatedAt: time.Now()},        // PID 1 always exists
			99999999: {PID: 99999999, ActivatedAt: time.Now()}, // Very unlikely to exist
		},
		GlobalOn: false,
	}

	registry.PruneDeadActivations()

	// PID 1 (init/launchd) should still exist
	if _, exists := registry.Activations[1]; !exists {
		t.Error("PID 1 should still exist after pruning")
	}

	// Dead PID should be removed
	if _, exists := registry.Activations[99999999]; exists {
		t.Error("dead PID should be removed after pruning")
	}
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
