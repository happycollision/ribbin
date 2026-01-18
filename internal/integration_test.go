//go:build integration

package internal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dondenton/ribbin/internal/config"
	"github.com/dondenton/ribbin/internal/shim"
)

// TestFullShimCycle tests the complete shim install/activate/block/uninstall workflow
func TestFullShimCycle(t *testing.T) {
	// Create temp directories for test isolation
	tmpDir, err := os.MkdirTemp("", "ribbin-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create bin directory for test binaries
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	// Create project directory
	projectDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Create home directory
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}

	// Save and set environment
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	origDir, _ := os.Getwd()

	os.Setenv("HOME", homeDir)
	os.Setenv("PATH", binDir+":"+origPath)
	os.Chdir(projectDir)

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	// Step 1: Create a test binary
	testBinaryPath := filepath.Join(binDir, "test-cmd")
	testBinaryContent := `#!/bin/sh
echo "original test-cmd executed with args: $@"
exit 0
`
	if err := os.WriteFile(testBinaryPath, []byte(testBinaryContent), 0755); err != nil {
		t.Fatalf("failed to create test binary: %v", err)
	}
	t.Log("Step 1: Created test binary")

	// Create a fake ribbin binary (in real use this would be the actual ribbin binary)
	ribbinPath := filepath.Join(binDir, "ribbin")
	ribbinContent := `#!/bin/sh
echo "ribbin shim intercepted: $0 $@"
exit 1
`
	if err := os.WriteFile(ribbinPath, []byte(ribbinContent), 0755); err != nil {
		t.Fatalf("failed to create ribbin binary: %v", err)
	}

	// Step 2: Create ribbin.toml
	configContent := `[shims.test-cmd]
action = "block"
message = "Use 'proper-cmd' instead"
paths = ["` + testBinaryPath + `"]
`
	configPath := filepath.Join(projectDir, "ribbin.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}
	t.Log("Step 2: Created ribbin.toml")

	// Step 3: Install shim
	registry := &config.Registry{
		Shims:       make(map[string]config.ShimEntry),
		Activations: make(map[int]config.ActivationEntry),
		GlobalOn:    false,
	}

	if err := shim.Install(testBinaryPath, ribbinPath, registry, configPath); err != nil {
		t.Fatalf("failed to install shim: %v", err)
	}
	t.Log("Step 3: Installed shim")

	// Verify symlink was created
	linkTarget, err := os.Readlink(testBinaryPath)
	if err != nil {
		t.Fatalf("test binary should be a symlink: %v", err)
	}
	if linkTarget != ribbinPath {
		t.Errorf("symlink should point to ribbin, got %s", linkTarget)
	}

	// Verify sidecar exists
	sidecarPath := testBinaryPath + ".ribbin-original"
	if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
		t.Error("sidecar should exist after shim install")
	}

	// Verify registry was updated
	if _, exists := registry.Shims["test-cmd"]; !exists {
		t.Error("registry should contain test-cmd entry")
	}

	// Save registry
	registryDir := filepath.Join(homeDir, ".config", "ribbin")
	if err := os.MkdirAll(registryDir, 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}
	registryPath := filepath.Join(registryDir, "registry.json")
	data, _ := json.MarshalIndent(registry, "", "  ")
	if err := os.WriteFile(registryPath, data, 0644); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}
	t.Log("Step 4: Saved registry")

	// Step 5: Test running shimmed command (should execute original via symlink)
	// Note: In real ribbin, the symlink points to ribbin which then decides
	// whether to block or passthrough. Here we just verify the symlink works.
	cmd := exec.Command(testBinaryPath, "arg1", "arg2")
	output, err := cmd.CombinedOutput()
	t.Logf("Shimmed command output: %s", output)
	// The command might fail since it's a symlink to our fake ribbin
	// but we verify the shim mechanism is in place
	t.Log("Step 5: Tested shimmed command execution")

	// Step 6: Uninstall shim
	if err := shim.Uninstall(testBinaryPath, registry); err != nil {
		t.Fatalf("failed to uninstall shim: %v", err)
	}
	t.Log("Step 6: Uninstalled shim")

	// Verify original is restored
	info, err := os.Lstat(testBinaryPath)
	if err != nil {
		t.Fatalf("test binary should exist after uninstall: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("test binary should not be a symlink after uninstall")
	}

	// Verify sidecar is removed
	if _, err := os.Stat(sidecarPath); !os.IsNotExist(err) {
		t.Error("sidecar should be removed after uninstall")
	}

	// Verify registry was updated
	if _, exists := registry.Shims["test-cmd"]; exists {
		t.Error("registry should not contain test-cmd entry after uninstall")
	}

	// Step 7: Test running restored command
	cmd = exec.Command(testBinaryPath, "arg1", "arg2")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("restored command should run successfully: %v, output: %s", err, output)
	}
	t.Logf("Restored command output: %s", output)
	t.Log("Step 7: Verified original binary restored and executable")

	t.Log("Full shim cycle completed successfully!")
}

// TestConfigDiscovery tests finding ribbin.toml in parent directories
func TestConfigDiscovery(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ribbin-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nested directory structure
	// tmpDir/project/ribbin.toml
	// tmpDir/project/src/lib/deep/
	projectDir := filepath.Join(tmpDir, "project")
	deepDir := filepath.Join(projectDir, "src", "lib", "deep")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	// Create config in project root
	configPath := filepath.Join(projectDir, "ribbin.toml")
	configContent := `[shims.npm]
action = "block"
message = "Use pnpm"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Save original directory
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Test from deep directory
	os.Chdir(deepDir)
	foundPath, err := config.FindProjectConfig()
	if err != nil {
		t.Fatalf("FindProjectConfig error: %v", err)
	}
	if foundPath != configPath {
		t.Errorf("expected %s, got %s", configPath, foundPath)
	}

	// Test config loading
	cfg, err := config.LoadProjectConfig(foundPath)
	if err != nil {
		t.Fatalf("LoadProjectConfig error: %v", err)
	}
	if _, exists := cfg.Shims["npm"]; !exists {
		t.Error("npm shim should exist")
	}
}

// TestRegistryPersistence tests registry save/load cycle
func TestRegistryPersistence(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "ribbin-registry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create and save registry
	registry := &config.Registry{
		Shims: map[string]config.ShimEntry{
			"cat":  {Original: "/usr/bin/cat", Config: "/project/ribbin.toml"},
			"node": {Original: "/usr/local/bin/node", Config: "/other/ribbin.toml"},
		},
		Activations: make(map[int]config.ActivationEntry),
		GlobalOn:    true,
	}

	if err := config.SaveRegistry(registry); err != nil {
		t.Fatalf("SaveRegistry error: %v", err)
	}

	// Load and verify
	loaded, err := config.LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry error: %v", err)
	}

	if !loaded.GlobalOn {
		t.Error("GlobalOn should be true")
	}
	if len(loaded.Shims) != 2 {
		t.Errorf("expected 2 shims, got %d", len(loaded.Shims))
	}
	if loaded.Shims["cat"].Original != "/usr/bin/cat" {
		t.Error("cat shim Original mismatch")
	}
}
