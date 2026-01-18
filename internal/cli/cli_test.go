package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/happycollision/ribbin/internal/config"
)

// setupTestEnv creates a test environment with temp HOME and working directory
func setupTestEnv(t *testing.T) (tempHome string, tempDir string, cleanup func()) {
	t.Helper()

	// Create temp home directory
	tempHome, err := os.MkdirTemp("", "ribbin-test-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}

	// Create temp working directory
	tempDir, err = os.MkdirTemp("", "ribbin-test-dir-*")
	if err != nil {
		os.RemoveAll(tempHome)
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Save original values
	origHome := os.Getenv("HOME")
	origDir, _ := os.Getwd()

	// Set up test environment
	os.Setenv("HOME", tempHome)
	os.Chdir(tempDir)

	// Return cleanup function
	cleanup = func() {
		os.Setenv("HOME", origHome)
		os.Chdir(origDir)
		os.RemoveAll(tempHome)
		os.RemoveAll(tempDir)
	}

	return tempHome, tempDir, cleanup
}

// createTestRegistry creates a registry.json in the temp home
func createTestRegistry(t *testing.T, tempHome string, registry *config.Registry) {
	t.Helper()

	registryDir := filepath.Join(tempHome, ".config", "ribbin")
	if err := os.MkdirAll(registryDir, 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal registry: %v", err)
	}

	registryPath := filepath.Join(registryDir, "registry.json")
	if err := os.WriteFile(registryPath, data, 0644); err != nil {
		t.Fatalf("failed to write registry: %v", err)
	}
}

// createTestConfig creates a ribbin.toml in the specified directory
func createTestConfig(t *testing.T, dir string, content string) string {
	t.Helper()

	configPath := filepath.Join(dir, "ribbin.toml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return configPath
}

func TestRootCommand(t *testing.T) {
	// Test that root command has expected subcommands
	subcommands := rootCmd.Commands()
	expectedCmds := []string{"shim", "unshim", "activate", "on", "off"}

	cmdNames := make(map[string]bool)
	for _, cmd := range subcommands {
		cmdNames[cmd.Name()] = true
	}

	for _, expected := range expectedCmds {
		if !cmdNames[expected] {
			t.Errorf("expected subcommand %q not found", expected)
		}
	}
}

func TestOnCommand(t *testing.T) {
	tempHome, _, cleanup := setupTestEnv(t)
	defer cleanup()

	t.Run("enables shims when disabled", func(t *testing.T) {
		registry := &config.Registry{
			Shims:       make(map[string]config.ShimEntry),
			Activations: make(map[int]config.ActivationEntry),
			GlobalOn:    false,
		}
		createTestRegistry(t, tempHome, registry)

		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		onCmd.Run(onCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if output == "" {
			t.Error("expected output from on command")
		}

		// Verify registry was updated
		loaded, _ := config.LoadRegistry()
		if !loaded.GlobalOn {
			t.Error("GlobalOn should be true after 'on' command")
		}
	})

	t.Run("reports already enabled", func(t *testing.T) {
		registry := &config.Registry{
			Shims:       make(map[string]config.ShimEntry),
			Activations: make(map[int]config.ActivationEntry),
			GlobalOn:    true,
		}
		createTestRegistry(t, tempHome, registry)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		onCmd.Run(onCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if output == "" {
			t.Error("expected output about already enabled")
		}
	})
}

func TestOffCommand(t *testing.T) {
	tempHome, _, cleanup := setupTestEnv(t)
	defer cleanup()

	t.Run("disables shims when enabled", func(t *testing.T) {
		registry := &config.Registry{
			Shims:       make(map[string]config.ShimEntry),
			Activations: make(map[int]config.ActivationEntry),
			GlobalOn:    true,
		}
		createTestRegistry(t, tempHome, registry)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		offCmd.Run(offCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)

		// Verify registry was updated
		loaded, _ := config.LoadRegistry()
		if loaded.GlobalOn {
			t.Error("GlobalOn should be false after 'off' command")
		}
	})

	t.Run("reports already disabled", func(t *testing.T) {
		registry := &config.Registry{
			Shims:       make(map[string]config.ShimEntry),
			Activations: make(map[int]config.ActivationEntry),
			GlobalOn:    false,
		}
		createTestRegistry(t, tempHome, registry)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		offCmd.Run(offCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if output == "" {
			t.Error("expected output about already disabled")
		}
	})
}

func TestActivateCommand(t *testing.T) {
	tempHome, _, cleanup := setupTestEnv(t)
	defer cleanup()

	t.Run("activates for current shell", func(t *testing.T) {
		registry := &config.Registry{
			Shims:       make(map[string]config.ShimEntry),
			Activations: make(map[int]config.ActivationEntry),
			GlobalOn:    false,
		}
		createTestRegistry(t, tempHome, registry)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		activateCmd.Run(activateCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)

		// Verify activation was added
		loaded, _ := config.LoadRegistry()
		ppid := os.Getppid()
		if _, exists := loaded.Activations[ppid]; !exists {
			t.Error("activation should be added for parent shell PID")
		}
	})

	t.Run("is idempotent", func(t *testing.T) {
		ppid := os.Getppid()
		registry := &config.Registry{
			Shims: make(map[string]config.ShimEntry),
			Activations: map[int]config.ActivationEntry{
				ppid: {PID: ppid},
			},
			GlobalOn: false,
		}
		createTestRegistry(t, tempHome, registry)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		activateCmd.Run(activateCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		// Should report already activated
		if output == "" {
			t.Error("expected output about already activated")
		}
	})
}

func TestCommonBinDirs(t *testing.T) {
	dirs := commonBinDirs()
	if len(dirs) == 0 {
		t.Error("commonBinDirs should return some directories")
	}

	// Should include standard paths
	found := make(map[string]bool)
	for _, dir := range dirs {
		found[dir] = true
	}

	if !found["/usr/bin"] {
		t.Error("should include /usr/bin")
	}
	if !found["/usr/local/bin"] {
		t.Error("should include /usr/local/bin")
	}
}
