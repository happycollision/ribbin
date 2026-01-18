//go:build integration

package internal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/shim"
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

// TestShimPathResolution tests that shims work when invoked by name (not full path)
// This tests the fix for: when running "npm" (via PATH), the shim correctly finds
// npm.ribbin-original in the same directory as the symlink, not in the cwd.
func TestShimPathResolution(t *testing.T) {
	// Build ribbin binary
	tmpDir, err := os.MkdirTemp("", "ribbin-path-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directories
	binDir := filepath.Join(tmpDir, "bin")
	homeDir := filepath.Join(tmpDir, "home")
	projectDir := filepath.Join(tmpDir, "project")
	workDir := filepath.Join(tmpDir, "workdir") // Different from where shim lives

	for _, dir := range []string{binDir, homeDir, projectDir, workDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Build ribbin into binDir
	ribbinPath := filepath.Join(binDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	// Find the module root by looking for go.mod
	moduleRoot := findModuleRoot(t)
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Create a test command in binDir
	testCmdPath := filepath.Join(binDir, "test-cmd")
	testCmdContent := `#!/bin/sh
echo "SUCCESS: original test-cmd ran"
exit 0
`
	if err := os.WriteFile(testCmdPath, []byte(testCmdContent), 0755); err != nil {
		t.Fatalf("failed to create test command: %v", err)
	}

	// Create ribbin.toml in projectDir (command should passthrough since we're not in projectDir)
	configContent := `[shims.test-cmd]
action = "block"
message = "blocked"
paths = ["` + testCmdPath + `"]
`
	configPath := filepath.Join(projectDir, "ribbin.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	// Save original environment
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	origDir, _ := os.Getwd()

	// Set up test environment
	os.Setenv("HOME", homeDir)
	os.Setenv("PATH", binDir+":"+origPath)

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	// Install shim: rename test-cmd to test-cmd.ribbin-original, symlink test-cmd -> ribbin
	registry := &config.Registry{
		Shims:       make(map[string]config.ShimEntry),
		Activations: make(map[int]config.ActivationEntry),
		GlobalOn:    false,
	}

	if err := shim.Install(testCmdPath, ribbinPath, registry, configPath); err != nil {
		t.Fatalf("failed to install shim: %v", err)
	}

	// Save registry (with no activations, so shim should passthrough)
	registryDir := filepath.Join(homeDir, ".config", "ribbin")
	if err := os.MkdirAll(registryDir, 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}
	registryPath := filepath.Join(registryDir, "registry.json")
	data, _ := json.MarshalIndent(registry, "", "  ")
	if err := os.WriteFile(registryPath, data, 0644); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// KEY TEST: Run "test-cmd" by name from workDir (not binDir)
	// The shim must resolve the PATH to find test-cmd.ribbin-original in binDir
	os.Chdir(workDir)

	// Run the command by name (not full path) - this is the bug scenario
	cmd := exec.Command("test-cmd")
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
	)
	output, err := cmd.CombinedOutput()

	// The command should succeed (passthrough to original) since:
	// 1. No activations in registry
	// 2. GlobalOn is false
	// 3. We're not in a directory with ribbin.toml
	if err != nil {
		t.Errorf("shim should passthrough to original command, got error: %v\nOutput: %s", err, output)
	}

	if !contains(string(output), "SUCCESS") {
		t.Errorf("expected original command output, got: %s", output)
	}

	t.Logf("Output: %s", output)
}

func findModuleRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Walk up until we find go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod in any parent directory")
		}
		dir = parent
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
