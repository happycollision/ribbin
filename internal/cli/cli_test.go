package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"

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

// createTestConfig creates a ribbin.jsonc in the specified directory
func createTestConfig(t *testing.T, dir string, content string) string {
	t.Helper()

	configPath := filepath.Join(dir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return configPath
}

func TestRootCommand(t *testing.T) {
	// Test that root command has expected subcommands
	subcommands := rootCmd.Commands()
	expectedCmds := []string{"wrap", "unwrap", "activate", "deactivate", "status", "recover"}

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

func TestActivateGlobalCommand(t *testing.T) {
	tempHome, _, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flags before each test
	resetActivateFlags := func() {
		activateConfig = false
		activateShell = false
		activateGlobal = false
	}

	t.Run("enables global activation when disabled", func(t *testing.T) {
		resetActivateFlags()
		registry := &config.Registry{
			Wrappers:          make(map[string]config.WrapperEntry),
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
			GlobalActive:      false,
		}
		createTestRegistry(t, tempHome, registry)

		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		activateGlobal = true
		activateCmd.Run(activateCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if output == "" {
			t.Error("expected output from activate --global command")
		}

		// Verify registry was updated
		loaded, _ := config.LoadRegistry()
		if !loaded.GlobalActive {
			t.Error("GlobalActive should be true after 'activate --global' command")
		}
	})

	t.Run("reports already globally active", func(t *testing.T) {
		resetActivateFlags()
		registry := &config.Registry{
			Wrappers:          make(map[string]config.WrapperEntry),
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
			GlobalActive:      true,
		}
		createTestRegistry(t, tempHome, registry)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		activateGlobal = true
		activateCmd.Run(activateCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if output == "" {
			t.Error("expected output about already globally active")
		}
	})
}

func TestDeactivateGlobalCommand(t *testing.T) {
	tempHome, _, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flags before each test
	resetDeactivateFlags := func() {
		deactivateConfig = false
		deactivateShell = false
		deactivateGlobal = false
		deactivateAll = false
		deactivateEverything = false
	}

	t.Run("disables global when enabled", func(t *testing.T) {
		resetDeactivateFlags()
		registry := &config.Registry{
			Wrappers:          make(map[string]config.WrapperEntry),
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
			GlobalActive:      true,
		}
		createTestRegistry(t, tempHome, registry)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		deactivateGlobal = true
		deactivateCmd.Run(deactivateCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)

		// Verify registry was updated
		loaded, _ := config.LoadRegistry()
		if loaded.GlobalActive {
			t.Error("GlobalActive should be false after 'deactivate --global' command")
		}
	})

	t.Run("reports already disabled", func(t *testing.T) {
		resetDeactivateFlags()
		registry := &config.Registry{
			Wrappers:          make(map[string]config.WrapperEntry),
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
			GlobalActive:      false,
		}
		createTestRegistry(t, tempHome, registry)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		deactivateGlobal = true
		deactivateCmd.Run(deactivateCmd, []string{})

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

func TestActivateShellCommand(t *testing.T) {
	tempHome, _, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flags before each test
	resetActivateFlags := func() {
		activateConfig = false
		activateShell = false
		activateGlobal = false
	}

	t.Run("activates for current shell", func(t *testing.T) {
		resetActivateFlags()
		registry := &config.Registry{
			Wrappers:          make(map[string]config.WrapperEntry),
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
			GlobalActive:      false,
		}
		createTestRegistry(t, tempHome, registry)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		activateShell = true
		activateCmd.Run(activateCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)

		// Verify activation was added
		loaded, _ := config.LoadRegistry()
		ppid := os.Getppid()
		if _, exists := loaded.ShellActivations[ppid]; !exists {
			t.Error("activation should be added for parent shell PID")
		}
	})

	t.Run("is idempotent for shell activation", func(t *testing.T) {
		resetActivateFlags()
		ppid := os.Getppid()
		registry := &config.Registry{
			Wrappers: make(map[string]config.WrapperEntry),
			ShellActivations: map[int]config.ShellActivationEntry{
				ppid: {PID: ppid},
			},
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
			GlobalActive:      false,
		}
		createTestRegistry(t, tempHome, registry)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		activateShell = true
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

func TestDeactivateEverything(t *testing.T) {
	tempHome, _, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flags before each test
	resetDeactivateFlags := func() {
		deactivateConfig = false
		deactivateShell = false
		deactivateGlobal = false
		deactivateAll = false
		deactivateEverything = false
	}

	t.Run("clears all activation state", func(t *testing.T) {
		resetDeactivateFlags()
		registry := &config.Registry{
			Wrappers: make(map[string]config.WrapperEntry),
			ShellActivations: map[int]config.ShellActivationEntry{
				12345: {PID: 12345},
			},
			ConfigActivations: map[string]config.ConfigActivationEntry{
				"/some/config.toml": {},
			},
			GlobalActive: true,
		}
		createTestRegistry(t, tempHome, registry)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		deactivateEverything = true
		deactivateCmd.Run(deactivateCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)

		// Verify everything was cleared
		loaded, _ := config.LoadRegistry()
		if loaded.GlobalActive {
			t.Error("GlobalActive should be false after --everything")
		}
		if len(loaded.ShellActivations) != 0 {
			t.Error("ShellActivations should be empty after --everything")
		}
		if len(loaded.ConfigActivations) != 0 {
			t.Error("ConfigActivations should be empty after --everything")
		}
	})
}

func TestStatusCommand(t *testing.T) {
	tempHome, _, cleanup := setupTestEnv(t)
	defer cleanup()

	t.Run("displays status", func(t *testing.T) {
		registry := &config.Registry{
			Wrappers:          make(map[string]config.WrapperEntry),
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
			GlobalActive:      false,
		}
		createTestRegistry(t, tempHome, registry)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		statusCmd.Run(statusCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if output == "" {
			t.Error("expected output from status command")
		}

		// Check for expected sections
		if !bytes.Contains([]byte(output), []byte("Ribbin Status")) {
			t.Error("expected 'Ribbin Status' header in output")
		}
		if !bytes.Contains([]byte(output), []byte("Activation:")) {
			t.Error("expected 'Activation:' section in output")
		}
	})
}

func TestCommonBinDirs(t *testing.T) {
	dirs, err := commonBinDirs()
	if err != nil {
		t.Fatalf("commonBinDirs() error = %v", err)
	}
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

func TestPrintGlobalWarningIfActive(t *testing.T) {
	t.Run("prints warning when global is active", func(t *testing.T) {
		tempHome, _, cleanup := setupTestEnv(t)
		defer cleanup()

		registry := &config.Registry{
			Wrappers:          make(map[string]config.WrapperEntry),
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
			GlobalActive:      true,
		}
		createTestRegistry(t, tempHome, registry)

		// Capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		printGlobalWarningIfActive()

		w.Close()
		os.Stderr = oldStderr

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if output == "" {
			t.Error("expected warning output when global is active")
		}
		if !bytes.Contains([]byte(output), []byte("GLOBAL MODE ACTIVE")) {
			t.Error("expected 'GLOBAL MODE ACTIVE' in warning output")
		}
		if !bytes.Contains([]byte(output), []byte("deactivate --global")) {
			t.Error("expected deactivation hint in warning output")
		}
	})

	t.Run("prints nothing when global is inactive", func(t *testing.T) {
		tempHome, _, cleanup := setupTestEnv(t)
		defer cleanup()

		registry := &config.Registry{
			Wrappers:          make(map[string]config.WrapperEntry),
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
			GlobalActive:      false,
		}
		createTestRegistry(t, tempHome, registry)

		// Capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		printGlobalWarningIfActive()

		w.Close()
		os.Stderr = oldStderr

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if output != "" {
			t.Errorf("expected no output when global is inactive, got: %q", output)
		}
	})

	t.Run("handles missing registry gracefully", func(t *testing.T) {
		_, _, cleanup := setupTestEnv(t)
		defer cleanup()

		// Don't create a registry - it won't exist

		// Capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		printGlobalWarningIfActive()

		w.Close()
		os.Stderr = oldStderr

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		// Should not print anything when registry doesn't exist
		if output != "" {
			t.Errorf("expected no output when registry doesn't exist, got: %q", output)
		}
	})
}
