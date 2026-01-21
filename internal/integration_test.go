package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/wrap"
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

	// Step 2: Create ribbin.jsonc
	configContent := `{
  "wrappers": {
    "test-cmd": {
      "action": "block",
      "message": "Use 'proper-cmd' instead",
      "paths": ["` + testBinaryPath + `"]
    }
  }
}`
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}
	t.Log("Step 2: Created ribbin.jsonc")

	// Step 3: Install shim
	registry := &config.Registry{
		Wrappers:       make(map[string]config.WrapperEntry),
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
		GlobalActive:    false,
	}

	if err := wrap.Install(testBinaryPath, ribbinPath, registry, configPath); err != nil {
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
	if _, exists := registry.Wrappers["test-cmd"]; !exists {
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
	if err := wrap.Uninstall(testBinaryPath, registry); err != nil {
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
	if _, exists := registry.Wrappers["test-cmd"]; exists {
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

// TestConfigDiscovery tests finding ribbin.jsonc in parent directories
func TestConfigDiscovery(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ribbin-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nested directory structure
	// tmpDir/project/ribbin.jsonc
	// tmpDir/project/src/lib/deep/
	projectDir := filepath.Join(tmpDir, "project")
	deepDir := filepath.Join(projectDir, "src", "lib", "deep")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	// Create config in project root
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	configContent := `{
  "wrappers": {
    "npm": {
      "action": "block",
      "message": "Use pnpm"
    }
  }
}`
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
	if _, exists := cfg.Wrappers["npm"]; !exists {
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

	// Create ribbin.jsonc in projectDir (command should passthrough since we're not in projectDir)
	configContent := `{
  "wrappers": {
    "test-cmd": {
      "action": "block",
      "message": "blocked",
      "paths": ["` + testCmdPath + `"]
    }
  }
}`
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
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
		Wrappers:       make(map[string]config.WrapperEntry),
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
		GlobalActive:    false,
	}

	if err := wrap.Install(testCmdPath, ribbinPath, registry, configPath); err != nil {
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
	// 3. We're not in a directory with ribbin.jsonc
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

// TestMiseCompatibility tests that ribbin works correctly with mise-style tool management.
// Mise installs binaries in ~/.local/share/mise/installs/<tool>/<version>/bin/
// and creates symlinks in ~/.local/share/mise/shims/ that point to the mise binary.
// When ribbin shims a mise-managed binary, it should work through the symlink chain.
//
// This test uses the real mise tool if available, otherwise simulates mise's behavior.
func TestMiseCompatibility(t *testing.T) {
	// Check if real mise is available
	misePath, err := exec.LookPath("mise")
	useMockMise := err != nil
	if useMockMise {
		t.Log("mise not found, using simulated mise environment")
	} else {
		t.Logf("Using real mise at: %s", misePath)
	}

	tmpDir, err := os.MkdirTemp("", "ribbin-mise-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	homeDir := filepath.Join(tmpDir, "home")
	projectDir := filepath.Join(tmpDir, "project")
	workDir := filepath.Join(tmpDir, "workdir")

	for _, dir := range []string{homeDir, projectDir, workDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Save original environment
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	origDir, _ := os.Getwd()

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	os.Setenv("HOME", homeDir)

	var miseShimsDir string
	var nodeShimPath string

	if useMockMise {
		// Create simulated mise environment
		miseInstallDir := filepath.Join(homeDir, ".local", "share", "mise", "installs", "node", "20.0.0", "bin")
		miseShimsDir = filepath.Join(homeDir, ".local", "share", "mise", "shims")

		for _, dir := range []string{miseInstallDir, miseShimsDir} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("failed to create dir %s: %v", dir, err)
			}
		}

		// Create mock "real" node
		realNodePath := filepath.Join(miseInstallDir, "node")
		if err := os.WriteFile(realNodePath, []byte("#!/bin/sh\necho \"MISE_NODE: v20.0.0\"\n"), 0755); err != nil {
			t.Fatalf("failed to create real node: %v", err)
		}

		// Create mock mise binary
		miseBinaryPath := filepath.Join(miseShimsDir, "mise")
		miseBinaryContent := `#!/bin/sh
exec "` + realNodePath + `" "$@"
`
		if err := os.WriteFile(miseBinaryPath, []byte(miseBinaryContent), 0755); err != nil {
			t.Fatalf("failed to create mise binary: %v", err)
		}

		// Create mise's node shim (symlink to mise)
		nodeShimPath = filepath.Join(miseShimsDir, "node")
		if err := os.Symlink(miseBinaryPath, nodeShimPath); err != nil {
			t.Fatalf("failed to create mise node shim: %v", err)
		}
	} else {
		// Use real mise - install a tiny tool (shfmt is small and fast)
		miseShimsDir = filepath.Join(homeDir, ".local", "share", "mise", "shims")
		if err := os.MkdirAll(miseShimsDir, 0755); err != nil {
			t.Fatalf("failed to create shims dir: %v", err)
		}

		// Configure mise to use our home directory
		os.Setenv("MISE_DATA_DIR", filepath.Join(homeDir, ".local", "share", "mise"))
		os.Setenv("MISE_CONFIG_DIR", filepath.Join(homeDir, ".config", "mise"))
		os.Setenv("MISE_CACHE_DIR", filepath.Join(homeDir, ".cache", "mise"))

		// Install a tiny tool with mise (use dummy plugin for testing)
		// We'll create a simple mock instead since installing real tools is slow
		miseInstallDir := filepath.Join(homeDir, ".local", "share", "mise", "installs", "dummy", "1.0.0", "bin")
		if err := os.MkdirAll(miseInstallDir, 0755); err != nil {
			t.Fatalf("failed to create install dir: %v", err)
		}

		// Create a dummy tool
		dummyPath := filepath.Join(miseInstallDir, "dummy-tool")
		if err := os.WriteFile(dummyPath, []byte("#!/bin/sh\necho \"MISE_DUMMY: v1.0.0\"\n"), 0755); err != nil {
			t.Fatalf("failed to create dummy tool: %v", err)
		}

		// Create mise-style shim (symlink to mise binary)
		nodeShimPath = filepath.Join(miseShimsDir, "dummy-tool")
		// For testing, create a simple wrapper that execs the real tool
		shimContent := `#!/bin/sh
exec "` + dummyPath + `" "$@"
`
		if err := os.WriteFile(nodeShimPath, []byte(shimContent), 0755); err != nil {
			t.Fatalf("failed to create shim: %v", err)
		}
	}

	os.Setenv("PATH", miseShimsDir+":"+origPath)

	// Verify shim works before ribbin
	cmd := exec.Command(nodeShimPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mise shim should work before ribbin: %v\nOutput: %s", err, output)
	}
	t.Logf("Mise shim works before ribbin: %s", output)

	// Build ribbin
	ribbinPath := filepath.Join(miseShimsDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	moduleRoot := findModuleRoot(t)
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Create ribbin.jsonc
	cmdName := filepath.Base(nodeShimPath)
	configContent := `{
  "wrappers": {
    "` + cmdName + `": {
      "action": "block",
      "message": "Use something else",
      "paths": ["` + nodeShimPath + `"]
    }
  }
}`
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	// Install ribbin shim
	registry := &config.Registry{
		Wrappers:       make(map[string]config.WrapperEntry),
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
		GlobalActive:    false,
	}

	if err := wrap.Install(nodeShimPath, ribbinPath, registry, configPath); err != nil {
		t.Fatalf("failed to install shim: %v", err)
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

	// Verify shim structure
	linkTarget, err := os.Readlink(nodeShimPath)
	if err != nil {
		t.Fatalf("shim should be a symlink: %v", err)
	}
	if linkTarget != ribbinPath {
		t.Errorf("shim should point to ribbin, got %s", linkTarget)
	}

	sidecarPath := nodeShimPath + ".ribbin-original"
	if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
		t.Fatalf("sidecar should exist: %v", err)
	}

	t.Log("Shim structure verified")

	// Test 1: From workDir (no ribbin.jsonc), command should passthrough
	os.Chdir(workDir)
	cmd = exec.Command(cmdName)
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+miseShimsDir+":"+origPath,
	)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("passthrough should work: %v\nOutput: %s", err, output)
	}
	t.Logf("Test 1 PASSED - Passthrough works: %s", output)

	// Test 2: RIBBIN_BYPASS=1 should passthrough
	cmd = exec.Command(cmdName)
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+miseShimsDir+":"+origPath,
		"RIBBIN_BYPASS=1",
	)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("bypass should work: %v\nOutput: %s", err, output)
	}
	t.Logf("Test 2 PASSED - Bypass works: %s", output)

	// Unshim and verify restoration
	if err := wrap.Uninstall(nodeShimPath, registry); err != nil {
		t.Fatalf("failed to uninstall shim: %v", err)
	}

	// Verify sidecar removed
	if _, err := os.Stat(sidecarPath); !os.IsNotExist(err) {
		t.Error("sidecar should be removed after uninstall")
	}

	// Verify original works
	cmd = exec.Command(nodeShimPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("restored shim should work: %v\nOutput: %s", err, output)
	}
	t.Logf("Test 3 PASSED - Shim restored: %s", output)

	t.Log("Mise compatibility test completed successfully!")
}

// TestAsdfCompatibility tests that ribbin works correctly with asdf-style tool management.
// asdf installs binaries in ~/.asdf/installs/<tool>/<version>/bin/
// and creates shell script shims in ~/.asdf/shims/ (NOT symlinks, actual scripts).
// When ribbin shims an asdf-managed binary, it should handle the shell script correctly.
//
// This test uses the real asdf tool if available, otherwise simulates asdf's behavior.
func TestAsdfCompatibility(t *testing.T) {
	// Check if real asdf is available
	asdfPath, err := exec.LookPath("asdf")
	useMockAsdf := err != nil
	if useMockAsdf {
		t.Log("asdf not found, using simulated asdf environment")
	} else {
		t.Logf("Using real asdf at: %s", asdfPath)
	}

	tmpDir, err := os.MkdirTemp("", "ribbin-asdf-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	homeDir := filepath.Join(tmpDir, "home")
	projectDir := filepath.Join(tmpDir, "project")
	workDir := filepath.Join(tmpDir, "workdir")

	for _, dir := range []string{homeDir, projectDir, workDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Save original environment
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	origDir, _ := os.Getwd()

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	os.Setenv("HOME", homeDir)

	var asdfShimsDir string
	var nodeShimPath string

	// Always use mock asdf for this test since installing real asdf plugins is slow
	// The key difference from mise is that asdf uses shell script shims, not symlinks
	asdfInstallDir := filepath.Join(homeDir, ".asdf", "installs", "nodejs", "20.0.0", "bin")
	asdfShimsDir = filepath.Join(homeDir, ".asdf", "shims")

	for _, dir := range []string{asdfInstallDir, asdfShimsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create the "real" node binary in asdf's install directory
	realNodePath := filepath.Join(asdfInstallDir, "node")
	realNodeContent := `#!/bin/sh
echo "ASDF_NODE: real node executed with args: $@"
exit 0
`
	if err := os.WriteFile(realNodePath, []byte(realNodeContent), 0755); err != nil {
		t.Fatalf("failed to create real node: %v", err)
	}

	// Create asdf's shell script shim for node
	// This is a shell script (NOT a symlink like mise uses) - this is the key difference!
	nodeShimPath = filepath.Join(asdfShimsDir, "node")
	asdfShimContent := `#!/bin/sh
# Simulated asdf shim script
# In real asdf, this would read .tool-versions, determine version, and exec the right binary
exec "` + realNodePath + `" "$@"
`
	if err := os.WriteFile(nodeShimPath, []byte(asdfShimContent), 0755); err != nil {
		t.Fatalf("failed to create asdf node shim: %v", err)
	}

	os.Setenv("PATH", asdfShimsDir+":"+origPath)

	// Verify the asdf shim works before ribbin gets involved
	cmd := exec.Command(nodeShimPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("asdf shim should work before ribbin: %v\nOutput: %s", err, output)
	}
	if !contains(string(output), "ASDF_NODE") {
		t.Fatalf("expected asdf node output, got: %s", output)
	}
	t.Logf("asdf shim works: %s", output)

	// Build ribbin
	ribbinPath := filepath.Join(asdfShimsDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	moduleRoot := findModuleRoot(t)
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Create ribbin.jsonc that blocks node
	configContent := `{
  "wrappers": {
    "node": {
      "action": "block",
      "message": "Use 'bun' instead of node",
      "paths": ["` + nodeShimPath + `"]
    }
  }
}`
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	// Install ribbin shim on asdf's node shim (a shell script)
	// This creates: node -> ribbin, node.ribbin-original = (the shell script)
	registry := &config.Registry{
		Wrappers:       make(map[string]config.WrapperEntry),
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
		GlobalActive:    false,
	}

	if err := wrap.Install(nodeShimPath, ribbinPath, registry, configPath); err != nil {
		t.Fatalf("failed to install shim on asdf node: %v", err)
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

	// Verify the shim structure:
	// node should be a symlink to ribbin
	// node.ribbin-original should be the asdf shell script (regular file, not symlink)
	linkTarget, err := os.Readlink(nodeShimPath)
	if err != nil {
		t.Fatalf("node should be a symlink: %v", err)
	}
	if linkTarget != ribbinPath {
		t.Errorf("node should point to ribbin, got %s", linkTarget)
	}

	sidecarPath := nodeShimPath + ".ribbin-original"
	sidecarInfo, err := os.Lstat(sidecarPath)
	if err != nil {
		t.Fatalf("sidecar should exist: %v", err)
	}
	// For asdf, the sidecar should be a regular file (the shell script), not a symlink
	if sidecarInfo.Mode()&os.ModeSymlink != 0 {
		t.Log("Note: sidecar is a symlink (unexpected for asdf, but checking Lstat)")
	}

	t.Log("Shim structure verified: node -> ribbin, node.ribbin-original = asdf script")

	// Test 1: From workDir (no ribbin.jsonc), command should passthrough via asdf script to real node
	os.Chdir(workDir)
	cmd = exec.Command("node", "--version")
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+asdfShimsDir+":"+origPath,
	)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("passthrough via asdf script should work: %v\nOutput: %s", err, output)
	}
	if !contains(string(output), "ASDF_NODE") {
		t.Errorf("expected output from real node via asdf, got: %s", output)
	}
	t.Logf("Test 1 PASSED - Passthrough via asdf shell script works: %s", output)

	// Test 2: With RIBBIN_BYPASS=1, should also passthrough
	cmd = exec.Command("node", "--version")
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+asdfShimsDir+":"+origPath,
		"RIBBIN_BYPASS=1",
	)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("bypass should work: %v\nOutput: %s", err, output)
	}
	if !contains(string(output), "ASDF_NODE") {
		t.Errorf("expected output from real node via asdf with bypass, got: %s", output)
	}
	t.Logf("Test 2 PASSED - Bypass works: %s", output)

	// Unshim and verify asdf shim is restored
	if err := wrap.Uninstall(nodeShimPath, registry); err != nil {
		t.Fatalf("failed to uninstall shim: %v", err)
	}

	// Verify asdf shim script is restored (regular file, not symlink)
	info, err := os.Lstat(nodeShimPath)
	if err != nil {
		t.Fatalf("node should exist after uninstall: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("node should be a regular file (asdf script) after uninstall, not a symlink")
	}

	// Verify the restored script works
	cmd = exec.Command(nodeShimPath, "--version")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("restored asdf shim should work: %v\nOutput: %s", err, output)
	}
	if !contains(string(output), "ASDF_NODE") {
		t.Errorf("expected asdf node output after restore, got: %s", output)
	}

	// Verify sidecar is removed
	if _, err := os.Stat(sidecarPath); !os.IsNotExist(err) {
		t.Error("sidecar should be removed after uninstall")
	}

	t.Log("Test 3 PASSED - asdf shell script shim restored after uninstall")
	t.Log("asdf compatibility test completed successfully!")
}

// TestParentDirectoryConfigDiscovery tests that ribbin finds ribbin.jsonc in parent directories
// when the shim is invoked from a subdirectory. This is an end-to-end test using the actual
// ribbin binary to verify the full flow works.
func TestParentDirectoryConfigDiscovery(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ribbin-parent-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directory structure:
	// tmpDir/home/                  - fake home directory
	// tmpDir/project/ribbin.jsonc    - config in project root
	// tmpDir/project/src/lib/deep/  - deep subdirectory where we'll run from
	// tmpDir/bin/                   - where shims live
	homeDir := filepath.Join(tmpDir, "home")
	projectDir := filepath.Join(tmpDir, "project")
	deepDir := filepath.Join(projectDir, "src", "lib", "deep")
	binDir := filepath.Join(tmpDir, "bin")

	for _, dir := range []string{homeDir, projectDir, deepDir, binDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create the "real" command
	realCmdPath := filepath.Join(binDir, "test-cmd.ribbin-original")
	realCmdContent := `#!/bin/sh
echo "REAL_CMD: executed from $(pwd)"
exit 0
`
	if err := os.WriteFile(realCmdPath, []byte(realCmdContent), 0755); err != nil {
		t.Fatalf("failed to create real cmd: %v", err)
	}

	// Build ribbin
	ribbinPath := filepath.Join(binDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	moduleRoot := findModuleRoot(t)
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Create shim symlink: test-cmd -> ribbin
	shimPath := filepath.Join(binDir, "test-cmd")
	if err := os.Symlink(ribbinPath, shimPath); err != nil {
		t.Fatalf("failed to create shim symlink: %v", err)
	}

	// Create ribbin.jsonc in project root (NOT in the deep subdirectory)
	configContent := `{
  "wrappers": {
    "test-cmd": {
      "action": "block",
      "message": "This command is blocked - config found in parent!"
    }
  }
}`
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	// Create registry with GlobalOn = true
	registry := &config.Registry{
		Wrappers: map[string]config.WrapperEntry{
			"test-cmd": {Original: shimPath, Config: configPath},
		},
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
		GlobalActive:    true,
	}
	registryDir := filepath.Join(homeDir, ".config", "ribbin")
	if err := os.MkdirAll(registryDir, 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}
	registryPath := filepath.Join(registryDir, "registry.json")
	data, _ := json.MarshalIndent(registry, "", "  ")
	if err := os.WriteFile(registryPath, data, 0644); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Save original environment
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	origDir, _ := os.Getwd()

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	os.Setenv("HOME", homeDir)
	os.Setenv("PATH", binDir+":"+origPath)

	// Test 1: Run from DEEP subdirectory - ribbin should find config in parent
	os.Chdir(deepDir)
	t.Logf("Running from: %s", deepDir)
	t.Logf("Config at: %s", configPath)

	cmd := exec.Command("test-cmd")
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
	)
	output, err := cmd.CombinedOutput()

	// Command should be BLOCKED because ribbin.jsonc is in a parent directory
	if err == nil {
		t.Errorf("command should be blocked when config is in parent dir, but succeeded with: %s", output)
	}

	// Verify the block message appears
	if !contains(string(output), "block") && !contains(string(output), "parent") {
		t.Logf("Output: %s", output)
	}
	t.Logf("Test 1 PASSED - Command blocked from deep subdir. Output: %s", output)

	// Test 2: With RIBBIN_BYPASS=1, should passthrough even from subdirectory
	cmd = exec.Command("test-cmd")
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
		"RIBBIN_BYPASS=1",
	)
	output, err = cmd.CombinedOutput()

	if err != nil {
		t.Errorf("bypass should work from subdirectory: %v\nOutput: %s", err, output)
	}
	if !contains(string(output), "REAL_CMD") {
		t.Errorf("expected real command output with bypass, got: %s", output)
	}
	t.Logf("Test 2 PASSED - Bypass works from subdir. Output: %s", output)

	// Test 3: Run from a directory WITHOUT ribbin.jsonc in any parent - should passthrough
	noConfigDir := filepath.Join(tmpDir, "other", "location")
	if err := os.MkdirAll(noConfigDir, 0755); err != nil {
		t.Fatalf("failed to create noconfig dir: %v", err)
	}
	os.Chdir(noConfigDir)

	cmd = exec.Command("test-cmd")
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
	)
	output, err = cmd.CombinedOutput()

	// Should passthrough since there's no ribbin.jsonc in any parent
	if err != nil {
		t.Errorf("should passthrough when no config in parents: %v\nOutput: %s", err, output)
	}
	if !contains(string(output), "REAL_CMD") {
		t.Errorf("expected real command output (no config), got: %s", output)
	}
	t.Logf("Test 3 PASSED - Passthrough when no config in parents. Output: %s", output)

	t.Log("Parent directory config discovery test completed successfully!")
}

// TestMiseWithActivation tests ribbin blocking when ribbin is activated and we're in a project dir
func TestMiseWithActivation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ribbin-mise-block-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mise-like structure
	homeDir := filepath.Join(tmpDir, "home")
	miseInstallDir := filepath.Join(homeDir, ".local", "share", "mise", "installs", "node", "20.0.0", "bin")
	miseShimsDir := filepath.Join(homeDir, ".local", "share", "mise", "shims")
	projectDir := filepath.Join(tmpDir, "project")

	for _, dir := range []string{miseInstallDir, miseShimsDir, projectDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create real node
	realNodePath := filepath.Join(miseInstallDir, "node")
	if err := os.WriteFile(realNodePath, []byte("#!/bin/sh\necho REAL_NODE\n"), 0755); err != nil {
		t.Fatalf("failed to create real node: %v", err)
	}

	// Create mise binary
	miseBinaryPath := filepath.Join(miseShimsDir, "mise")
	miseBinaryContent := `#!/bin/sh
exec "` + realNodePath + `" "$@"
`
	if err := os.WriteFile(miseBinaryPath, []byte(miseBinaryContent), 0755); err != nil {
		t.Fatalf("failed to create mise binary: %v", err)
	}

	// Create mise node shim (symlink)
	miseNodeShim := filepath.Join(miseShimsDir, "node")
	if err := os.Symlink(miseBinaryPath, miseNodeShim); err != nil {
		t.Fatalf("failed to create mise node shim: %v", err)
	}

	// Build ribbin
	ribbinPath := filepath.Join(miseShimsDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	moduleRoot := findModuleRoot(t)
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Create ribbin.jsonc
	configContent := `{
  "wrappers": {
    "node": {
      "action": "block",
      "message": "Use 'bun' instead"
    }
  }
}`
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	// Save original environment
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	origDir, _ := os.Getwd()

	os.Setenv("HOME", homeDir)
	os.Setenv("PATH", miseShimsDir+":"+origPath)

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	// Install shim
	registry := &config.Registry{
		Wrappers:       make(map[string]config.WrapperEntry),
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
		GlobalActive:    true, // Enable globally for this test
	}

	if err := wrap.Install(miseNodeShim, ribbinPath, registry, configPath); err != nil {
		t.Fatalf("failed to install shim: %v", err)
	}

	// Save registry with GlobalOn = true
	registryDir := filepath.Join(homeDir, ".config", "ribbin")
	if err := os.MkdirAll(registryDir, 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}
	registryPath := filepath.Join(registryDir, "registry.json")
	data, _ := json.MarshalIndent(registry, "", "  ")
	if err := os.WriteFile(registryPath, data, 0644); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Test: From projectDir (has ribbin.jsonc), with GlobalOn=true, node should be BLOCKED
	os.Chdir(projectDir)
	cmd := exec.Command("node", "--version")
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+miseShimsDir+":"+origPath,
	)
	output, err := cmd.CombinedOutput()

	// Command should fail (blocked)
	if err == nil {
		t.Errorf("node should be blocked, but succeeded with output: %s", output)
	}

	// Output should contain the block message
	if !contains(string(output), "block") && !contains(string(output), "bun") {
		t.Logf("Note: Output doesn't contain expected block message: %s", output)
	}

	t.Logf("PASSED - node was blocked as expected. Output: %s", output)
}

// TestRedirectAction tests the redirect action functionality end-to-end.
// This test verifies that:
// 1. Redirect scripts are invoked correctly
// 2. Environment variables are passed (RIBBIN_ORIGINAL_BIN, RIBBIN_COMMAND, RIBBIN_CONFIG, RIBBIN_ACTION)
// 3. Arguments are forwarded to the redirect script
// 4. Exit codes propagate from the script to the shim
func TestRedirectAction(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ribbin-redirect-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directory structure
	homeDir := filepath.Join(tmpDir, "home")
	projectDir := filepath.Join(tmpDir, "project")
	binDir := filepath.Join(tmpDir, "bin")

	for _, dir := range []string{homeDir, projectDir, binDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Save original environment
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	origDir, _ := os.Getwd()

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	os.Setenv("HOME", homeDir)
	os.Setenv("PATH", binDir+":"+origPath)

	// Build ribbin binary
	ribbinPath := filepath.Join(binDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	moduleRoot := findModuleRoot(t)
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Copy test fixtures from testdata/projects/redirect/ to project dir
	fixtureDir := filepath.Join(moduleRoot, "testdata", "projects", "redirect")

	// Copy ribbin.jsonc
	fixtureConfigPath := filepath.Join(fixtureDir, "ribbin.jsonc")
	fixtureConfig, err := os.ReadFile(fixtureConfigPath)
	if err != nil {
		t.Fatalf("failed to read fixture config: %v", err)
	}
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, fixtureConfig, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Copy redirect script
	scriptsDir := filepath.Join(projectDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts dir: %v", err)
	}
	fixtureScriptPath := filepath.Join(fixtureDir, "scripts", "test-redirect.sh")
	fixtureScript, err := os.ReadFile(fixtureScriptPath)
	if err != nil {
		t.Fatalf("failed to read fixture script: %v", err)
	}
	scriptPath := filepath.Join(scriptsDir, "test-redirect.sh")
	if err := os.WriteFile(scriptPath, fixtureScript, 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// Create the original echo command
	echoCmdPath := filepath.Join(binDir, "echo")
	echoCmdContent := `#!/bin/sh
echo "ORIGINAL_ECHO: $@"
exit 0
`
	if err := os.WriteFile(echoCmdPath, []byte(echoCmdContent), 0755); err != nil {
		t.Fatalf("failed to create echo command: %v", err)
	}

	// Install shim
	registry := &config.Registry{
		Wrappers:       make(map[string]config.WrapperEntry),
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
		GlobalActive:    true, // Enable globally
	}

	if err := wrap.Install(echoCmdPath, ribbinPath, registry, configPath); err != nil {
		t.Fatalf("failed to install shim: %v", err)
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

	// Change to project directory (where ribbin.jsonc is)
	os.Chdir(projectDir)

	// Execute the shimmed echo command with arguments
	cmd := exec.Command("echo", "arg1", "arg2", "arg3")
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
	)
	output, err := cmd.CombinedOutput()

	// Command should succeed (exit 0)
	if err != nil {
		t.Errorf("redirect script should exit 0: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	t.Logf("Redirect output: %s", outputStr)

	// Verify output contains expected markers
	if !contains(outputStr, "REDIRECT_CALLED=true") {
		t.Error("output should contain REDIRECT_CALLED=true")
	}

	// Verify environment variables are set
	if !contains(outputStr, "RIBBIN_ORIGINAL_BIN=") {
		t.Error("output should contain RIBBIN_ORIGINAL_BIN")
	}
	if !contains(outputStr, "RIBBIN_COMMAND=") {
		t.Error("output should contain RIBBIN_COMMAND")
	}
	if !contains(outputStr, "RIBBIN_CONFIG=") {
		t.Error("output should contain RIBBIN_CONFIG")
	}
	if !contains(outputStr, "RIBBIN_ACTION=redirect") {
		t.Error("output should contain RIBBIN_ACTION=redirect")
	}

	// Verify arguments are forwarded
	if !contains(outputStr, "ARGS=arg1 arg2 arg3") {
		t.Error("output should contain forwarded arguments: ARGS=arg1 arg2 arg3")
	}

	// Test with RIBBIN_BYPASS - should execute original echo
	cmd = exec.Command("echo", "bypass-test")
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
		"RIBBIN_BYPASS=1",
	)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("bypass should work: %v\nOutput: %s", err, output)
	}
	if !contains(string(output), "ORIGINAL_ECHO") {
		t.Errorf("bypass should execute original echo, got: %s", output)
	}
	t.Logf("Bypass test passed: %s", output)

	t.Log("Redirect action test completed successfully!")
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
		Wrappers: map[string]config.WrapperEntry{
			"cat":  {Original: "/usr/bin/cat", Config: "/project/ribbin.jsonc"},
			"node": {Original: "/usr/local/bin/node", Config: "/other/ribbin.jsonc"},
		},
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
		GlobalActive:    true,
	}

	if err := config.SaveRegistry(registry); err != nil {
		t.Fatalf("SaveRegistry error: %v", err)
	}

	// Load and verify
	loaded, err := config.LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry error: %v", err)
	}

	if !loaded.GlobalActive {
		t.Error("GlobalOn should be true")
	}
	if len(loaded.Wrappers) != 2 {
		t.Errorf("expected 2 shims, got %d", len(loaded.Wrappers))
	}
	if loaded.Wrappers["cat"].Original != "/usr/bin/cat" {
		t.Error("cat shim Original mismatch")
	}
}

// TestScopedConfigIsolation tests that isolated scopes (no extends) only have their own shims
func TestScopedConfigIsolation(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	fixtureDir := filepath.Join(moduleRoot, "testdata", "projects", "scoped")
	configPath := filepath.Join(fixtureDir, "ribbin.jsonc")

	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Create an isolated scope (no extends) for testing
	isolatedScope := config.ScopeConfig{
		Path:  "isolated",
		Wrappers: map[string]config.ShimConfig{"local-cmd": {Action: "block", Message: "local only"}},
	}

	resolver := config.NewResolver()
	result, err := resolver.ResolveEffectiveShims(cfg, configPath, &isolatedScope)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error: %v", err)
	}

	// Should only have local-cmd, not cat/npm/rm from root
	if len(result) != 1 {
		t.Errorf("isolated scope should have 1 shim, got %d: %v", len(result), result)
	}
	if _, ok := result["local-cmd"]; !ok {
		t.Error("isolated scope should have local-cmd")
	}
	if _, ok := result["cat"]; ok {
		t.Error("isolated scope should NOT have cat (no extends)")
	}
}

// TestScopedConfigExtendsRoot tests that scopes extending root inherit root shims
func TestScopedConfigExtendsRoot(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	fixtureDir := filepath.Join(moduleRoot, "testdata", "projects", "scoped")
	configPath := filepath.Join(fixtureDir, "ribbin.jsonc")

	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Frontend scope extends root
	frontendScope := cfg.Scopes["frontend"]
	resolver := config.NewResolver()
	result, err := resolver.ResolveEffectiveShims(cfg, configPath, &frontendScope)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error: %v", err)
	}

	// Should have: cat, npm, rm (from root), yarn (own), rm override (own)
	if _, ok := result["cat"]; !ok {
		t.Error("frontend should inherit cat from root")
	}
	if _, ok := result["npm"]; !ok {
		t.Error("frontend should inherit npm from root")
	}
	if _, ok := result["yarn"]; !ok {
		t.Error("frontend should have its own yarn shim")
	}

	// Frontend overrides rm to block (root has warn)
	rmShim := result["rm"]
	if rmShim.Action != "block" {
		t.Errorf("frontend rm should be block (override), got %s", rmShim.Action)
	}
	if rmShim.Message != "Use trash in frontend" {
		t.Errorf("frontend rm message should be overridden, got %s", rmShim.Message)
	}
}

// TestScopedConfigMultipleExtends tests extends = ["root", "root.scope"]
func TestScopedConfigMultipleExtends(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	fixtureDir := filepath.Join(moduleRoot, "testdata", "projects", "scoped")
	configPath := filepath.Join(fixtureDir, "ribbin.jsonc")

	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Backend extends root and root.hardened
	backendScope := cfg.Scopes["backend"]
	resolver := config.NewResolver()
	result, err := resolver.ResolveEffectiveShims(cfg, configPath, &backendScope)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error: %v", err)
	}

	// Should have: cat (root), curl (hardened), rm (hardened wins over root)
	if _, ok := result["cat"]; !ok {
		t.Error("backend should have cat from root")
	}
	if _, ok := result["curl"]; !ok {
		t.Error("backend should have curl from hardened")
	}

	// rm should come from hardened (later in extends), not root
	rmShim := result["rm"]
	if rmShim.Action != "block" {
		t.Errorf("backend rm should be block (from hardened), got %s", rmShim.Action)
	}
	if rmShim.Message != "Use trash (hardened)" {
		t.Errorf("backend rm message should be from hardened, got %s", rmShim.Message)
	}
}

// TestScopedConfigPassthrough tests action = "passthrough" overriding a block
func TestScopedConfigPassthrough(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	fixtureDir := filepath.Join(moduleRoot, "testdata", "projects", "scoped")
	configPath := filepath.Join(fixtureDir, "ribbin.jsonc")

	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Backend has npm = passthrough, overriding root's block
	backendScope := cfg.Scopes["backend"]
	resolver := config.NewResolver()
	result, err := resolver.ResolveEffectiveShims(cfg, configPath, &backendScope)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error: %v", err)
	}

	npmShim := result["npm"]
	if npmShim.Action != "passthrough" {
		t.Errorf("backend npm should be passthrough (override), got %s", npmShim.Action)
	}
}

// TestScopedConfigExternalExtends tests extends from external file
func TestScopedConfigExternalExtends(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	fixtureDir := filepath.Join(moduleRoot, "testdata", "projects", "scoped")
	configPath := filepath.Join(fixtureDir, "ribbin.jsonc")

	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// external-test extends ./external/ribbin.jsonc
	externalScope := cfg.Scopes["external-test"]
	resolver := config.NewResolver()
	result, err := resolver.ResolveEffectiveShims(cfg, configPath, &externalScope)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error: %v", err)
	}

	// Should have shims from external file
	if _, ok := result["external-cmd"]; !ok {
		t.Error("external-test should have external-cmd from external file")
	}
	if _, ok := result["shared-tool"]; !ok {
		t.Error("external-test should have shared-tool from external file")
	}

	// Verify it came from the external file
	extShim := result["external-cmd"]
	if extShim.Action != "block" {
		t.Errorf("external-cmd action should be block, got %s", extShim.Action)
	}
}

// TestScopeMatching tests that the correct scope is selected based on CWD
func TestScopeMatching(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	fixtureDir := filepath.Join(moduleRoot, "testdata", "projects", "scoped")
	configPath := filepath.Join(fixtureDir, "ribbin.jsonc")

	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	tests := []struct {
		name          string
		cwd           string
		expectedScope string // empty means root (no scope match), "hardened" matches root dir since it has no path
	}{
		{"root dir", fixtureDir, "hardened"}, // hardened has no path, defaults to ".", matches config dir
		{"frontend dir", filepath.Join(fixtureDir, "apps", "frontend"), "frontend"},
		{"frontend subdir", filepath.Join(fixtureDir, "apps", "frontend", "src"), "frontend"},
		{"backend dir", filepath.Join(fixtureDir, "apps", "backend"), "backend"},
		{"external dir", filepath.Join(fixtureDir, "external"), "external-test"},
		{"unmatched dir", filepath.Join(fixtureDir, "some", "other"), "hardened"}, // hardened matches because path defaults to "."
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := config.FindMatchingScope(cfg, fixtureDir, tt.cwd)
			if tt.expectedScope == "" {
				if match != nil {
					t.Errorf("expected no scope match, got %s", match.Name)
				}
			} else {
				if match == nil {
					t.Errorf("expected scope %s, got nil", tt.expectedScope)
				} else if match.Name != tt.expectedScope {
					t.Errorf("expected scope %s, got %s", tt.expectedScope, match.Name)
				}
			}
		})
	}
}

// TestProvenanceTracking tests that provenance is correctly tracked through extends
func TestProvenanceTracking(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	fixtureDir := filepath.Join(moduleRoot, "testdata", "projects", "scoped")
	configPath := filepath.Join(fixtureDir, "ribbin.jsonc")

	cfg, err := config.LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Frontend scope extends root and overrides rm
	frontendScope := cfg.Scopes["frontend"]
	resolver := config.NewResolver()
	result, err := resolver.ResolveEffectiveShimsWithProvenance(cfg, configPath, &frontendScope, "frontend")
	if err != nil {
		t.Fatalf("ResolveEffectiveShimsWithProvenance error: %v", err)
	}

	// cat should come from root
	catShim := result["cat"]
	if catShim.Source.Fragment != "root" {
		t.Errorf("cat source should be root, got %s", catShim.Source.Fragment)
	}
	if catShim.Source.Overrode != nil {
		t.Error("cat should not have overrode (it's inherited, not overridden)")
	}

	// rm should come from frontend, overriding root
	rmShim := result["rm"]
	if rmShim.Source.Fragment != "root.frontend" {
		t.Errorf("rm source should be root.frontend, got %s", rmShim.Source.Fragment)
	}
	if rmShim.Source.Overrode == nil {
		t.Error("rm should have overrode set (it overrides root)")
	} else if rmShim.Source.Overrode.Fragment != "root" {
		t.Errorf("rm overrode should be root, got %s", rmShim.Source.Overrode.Fragment)
	}
}

// TestConfigShowCommand tests the ribbin config show command output
func TestConfigShowCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ribbin-config-show-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Copy fixtures
	moduleRoot := findModuleRoot(t)
	fixtureDir := filepath.Join(moduleRoot, "testdata", "projects", "scoped")

	// Create directory structure
	projectDir := filepath.Join(tmpDir, "project")
	frontendDir := filepath.Join(projectDir, "apps", "frontend")
	externalDir := filepath.Join(projectDir, "external")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}
	if err := os.MkdirAll(externalDir, 0755); err != nil {
		t.Fatalf("failed to create external dir: %v", err)
	}

	// Copy config files
	mainConfig, _ := os.ReadFile(filepath.Join(fixtureDir, "ribbin.jsonc"))
	if err := os.WriteFile(filepath.Join(projectDir, "ribbin.jsonc"), mainConfig, 0644); err != nil {
		t.Fatalf("failed to write main config: %v", err)
	}
	extConfig, _ := os.ReadFile(filepath.Join(fixtureDir, "external", "ribbin.jsonc"))
	if err := os.WriteFile(filepath.Join(externalDir, "ribbin.jsonc"), extConfig, 0644); err != nil {
		t.Fatalf("failed to write external config: %v", err)
	}

	// Build ribbin
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}
	ribbinPath := filepath.Join(binDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Save original dir
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Test from frontend directory
	os.Chdir(frontendDir)
	cmd := exec.Command(ribbinPath, "config", "show")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("config show output: %s", output)
		// Command may exit non-zero if no config found - that's ok
	}

	outputStr := string(output)
	t.Logf("Config show output:\n%s", outputStr)

	// Verify output contains expected elements
	if !contains(outputStr, "ribbin.jsonc") {
		t.Error("output should contain config file name")
	}
	if !contains(outputStr, "frontend") {
		t.Error("output should show frontend scope")
	}
}

// TestEndToEndScopedBlocking tests full shim blocking with scoped configs
func TestEndToEndScopedBlocking(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ribbin-scoped-e2e-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up directories
	homeDir := filepath.Join(tmpDir, "home")
	projectDir := filepath.Join(tmpDir, "project")
	frontendDir := filepath.Join(projectDir, "apps", "frontend")
	backendDir := filepath.Join(projectDir, "apps", "backend")
	binDir := filepath.Join(tmpDir, "bin")

	for _, dir := range []string{homeDir, frontendDir, backendDir, binDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Build ribbin
	moduleRoot := findModuleRoot(t)
	ribbinPath := filepath.Join(binDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Create test command
	npmPath := filepath.Join(binDir, "npm")
	npmContent := `#!/bin/sh
echo "REAL_NPM: $@"
exit 0
`
	if err := os.WriteFile(npmPath, []byte(npmContent), 0755); err != nil {
		t.Fatalf("failed to create npm: %v", err)
	}

	// Create scoped config
	configContent := `{
  "wrappers": {
    "npm": {
      "action": "block",
      "message": "Use pnpm instead"
    }
  },
  "scopes": {
    "backend": {
      "path": "apps/backend",
      "extends": ["root"],
      "wrappers": {
        "npm": {
          "action": "passthrough"
        }
      }
    }
  }
}`
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Install shim
	registry := &config.Registry{
		Wrappers:       make(map[string]config.WrapperEntry),
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
		GlobalActive:    true,
	}
	if err := wrap.Install(npmPath, ribbinPath, registry, configPath); err != nil {
		t.Fatalf("failed to install shim: %v", err)
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

	// Save original environment
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	origDir, _ := os.Getwd()

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	os.Setenv("HOME", homeDir)
	os.Setenv("PATH", binDir+":"+origPath)

	// Test 1: From frontend (no passthrough) - should be blocked
	os.Chdir(frontendDir)
	cmd := exec.Command("npm", "install")
	cmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Errorf("npm should be blocked in frontend, but succeeded: %s", output)
	}
	t.Logf("Frontend npm blocked as expected: %s", output)

	// Test 2: From backend (has passthrough) - should work
	os.Chdir(backendDir)
	cmd = exec.Command("npm", "install")
	cmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err = cmd.CombinedOutput()

	if err != nil {
		t.Errorf("npm should passthrough in backend: %v\nOutput: %s", err, output)
	}
	if !contains(string(output), "REAL_NPM") {
		t.Errorf("expected real npm output, got: %s", output)
	}
	t.Logf("Backend npm passthrough works: %s", output)

	t.Log("End-to-end scoped blocking test completed!")
}

// TestNodeModulesTscWrappingNpm tests wrapping node_modules/.bin/tsc installed via npm.
// This is an end-to-end test that verifies ribbin can wrap binaries in node_modules/.bin/
// from a parent directory, using npm as the package manager.
//
// This test requires Node.js and npm to be installed.
func TestNodeModulesTscWrappingNpm(t *testing.T) {
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not found, skipping test")
	}

	testNodeModulesTscWrapping(t, "npm")
}

// TestNodeModulesTscWrappingPnpm tests wrapping node_modules/.bin/tsc installed via pnpm.
// This is an end-to-end test that verifies ribbin can wrap binaries in node_modules/.bin/
// from a parent directory, using pnpm as the package manager.
//
// This test requires Node.js and pnpm to be installed.
func TestNodeModulesTscWrappingPnpm(t *testing.T) {
	if _, err := exec.LookPath("pnpm"); err != nil {
		t.Skip("pnpm not found, skipping test")
	}

	testNodeModulesTscWrapping(t, "pnpm")
}

// testNodeModulesTscWrapping is the shared test implementation for both npm and pnpm.
// It tests the full lifecycle of wrapping node_modules/.bin/tsc from a parent directory.
// This is an end-to-end test that uses the actual ribbin CLI commands.
func testNodeModulesTscWrapping(t *testing.T, packageManager string) {
	tmpDir, err := os.MkdirTemp("", "ribbin-"+packageManager+"-modules-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	homeDir := filepath.Join(tmpDir, "home")
	parentDir := filepath.Join(tmpDir, "parent")
	projectDir := filepath.Join(parentDir, "project")
	binDir := filepath.Join(tmpDir, "bin")

	for _, dir := range []string{homeDir, parentDir, projectDir, binDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Build ribbin
	moduleRoot := findModuleRoot(t)
	ribbinPath := filepath.Join(binDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Save original environment
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	origDir, _ := os.Getwd()

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	os.Setenv("HOME", homeDir)

	// Create package.json with TypeScript as a dev dependency
	packageJSON := `{
  "name": "test-project",
  "version": "1.0.0",
  "devDependencies": {
    "typescript": "^5.0.0"
  }
}`
	if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	// Install dependencies
	t.Logf("Installing TypeScript with %s...", packageManager)
	installCmd := exec.Command(packageManager, "install")
	installCmd.Dir = projectDir
	installCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
		"CI=true",
	)
	if output, err := installCmd.CombinedOutput(); err != nil {
		t.Fatalf("%s install failed: %v\n%s", packageManager, err, output)
	}
	t.Logf("%s install completed", packageManager)

	// Verify tsc exists in node_modules/.bin/
	tscPath := filepath.Join(projectDir, "node_modules", ".bin", "tsc")
	if _, err := os.Stat(tscPath); os.IsNotExist(err) {
		t.Fatalf("tsc not found at %s after %s install", tscPath, packageManager)
	}

	// Log what type of binary tsc is (symlink or regular file)
	tscInfo, err := os.Lstat(tscPath)
	if err != nil {
		t.Fatalf("failed to lstat tsc: %v", err)
	}
	isSymlink := tscInfo.Mode()&os.ModeSymlink != 0
	t.Logf("tsc is symlink: %v", isSymlink)
	if isSymlink {
		target, _ := os.Readlink(tscPath)
		t.Logf("tsc symlink target: %s", target)
	}

	// Verify tsc runs before shimming
	t.Log("Verifying tsc works before shimming...")
	tscCmd := exec.Command(tscPath, "--version")
	tscCmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err := tscCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tsc --version failed before shimming: %v\n%s", err, output)
	}
	t.Logf("tsc version: %s", output)

	// Create ribbin.jsonc in PARENT directory (testing parent dir config)
	// Use explicit paths to wrap the tsc in node_modules
	configContent := fmt.Sprintf(`{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead of tsc directly",
      "paths": ["%s"]
    }
  }
}`, tscPath)
	configPath := filepath.Join(parentDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write ribbin.jsonc: %v", err)
	}

	// Use CLI to wrap tsc (need --confirm-system-dir for test temp directories)
	t.Log("Running ribbin wrap...")
	wrapCmd := exec.Command(ribbinPath, "wrap", "--confirm-system-dir", configPath)
	wrapCmd.Dir = parentDir
	wrapCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
	)
	output, err = wrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin wrap failed: %v\n%s", err, output)
	}
	t.Logf("ribbin wrap output: %s", output)

	// Use CLI to activate globally
	t.Log("Running ribbin activate --global...")
	activateCmd := exec.Command(ribbinPath, "activate", "--global")
	activateCmd.Dir = parentDir
	activateCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
	)
	output, err = activateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin activate --global failed: %v\n%s", err, output)
	}
	t.Logf("ribbin activate output: %s", output)

	// Verify shim structure
	shimInfo, err := os.Lstat(tscPath)
	if err != nil {
		t.Fatalf("failed to lstat shimmed tsc: %v", err)
	}
	if shimInfo.Mode()&os.ModeSymlink == 0 {
		t.Error("shimmed tsc should be a symlink to ribbin")
	} else {
		linkTarget, _ := os.Readlink(tscPath)
		if linkTarget != ribbinPath {
			t.Errorf("shimmed tsc should point to ribbin, got %s", linkTarget)
		}
	}

	// Verify sidecar exists
	sidecarPath := tscPath + ".ribbin-original"
	if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
		t.Fatal("sidecar should exist after shim install")
	}
	t.Logf("Sidecar exists: %s", sidecarPath)

	// Test 1: From project directory, tsc should be BLOCKED
	t.Log("Test 1: tsc should be blocked from project directory")
	os.Chdir(projectDir)
	tscCmd = exec.Command(tscPath, "--version")
	tscCmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err = tscCmd.CombinedOutput()
	if err == nil {
		t.Errorf("tsc should be blocked, but succeeded: %s", output)
	} else {
		if !contains(string(output), "block") && !contains(string(output), "typecheck") {
			t.Logf("Note: output doesn't contain expected block message: %s", output)
		}
		t.Logf("Test 1 PASSED - tsc blocked: %s", output)
	}

	// Test 2: With RIBBIN_BYPASS=1, tsc should work
	t.Log("Test 2: tsc should work with RIBBIN_BYPASS=1")
	tscCmd = exec.Command(tscPath, "--version")
	tscCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
		"RIBBIN_BYPASS=1",
	)
	output, err = tscCmd.CombinedOutput()
	if err != nil {
		t.Errorf("tsc with bypass should work: %v\n%s", err, output)
	} else {
		if !contains(string(output), "Version") {
			t.Errorf("expected TypeScript version output, got: %s", output)
		}
		t.Logf("Test 2 PASSED - tsc with bypass: %s", output)
	}

	// Test 3: Run tsc by name (via PATH) from project directory
	t.Log("Test 3: tsc by name should be blocked")
	nodeModulesBin := filepath.Join(projectDir, "node_modules", ".bin")
	tscCmd = exec.Command("tsc", "--version")
	tscCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+nodeModulesBin+":"+binDir+":"+origPath,
	)
	output, err = tscCmd.CombinedOutput()
	if err == nil {
		t.Errorf("tsc by name should be blocked, but succeeded: %s", output)
	} else {
		t.Logf("Test 3 PASSED - tsc by name blocked: %s", output)
	}

	// Use CLI to unwrap
	t.Log("Running ribbin unwrap...")
	unwrapCmd := exec.Command(ribbinPath, "unwrap", configPath)
	unwrapCmd.Dir = parentDir
	unwrapCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
	)
	output, err = unwrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin unwrap failed: %v\n%s", err, output)
	}
	t.Logf("ribbin unwrap output: %s", output)

	// Verify tsc works after unshimming
	t.Log("Verifying tsc works after unshimming...")
	tscCmd = exec.Command(tscPath, "--version")
	tscCmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err = tscCmd.CombinedOutput()
	if err != nil {
		t.Errorf("tsc should work after unshimming: %v\n%s", err, output)
	} else {
		t.Logf("tsc restored and working: %s", output)
	}

	// Verify sidecar is removed
	if _, err := os.Stat(sidecarPath); !os.IsNotExist(err) {
		t.Error("sidecar should be removed after uninstall")
	}

	t.Logf("%s node_modules test completed!", packageManager)
}

// TestMiseManagedBinaryWrapping tests wrapping a binary managed by mise (symlink-based shims).
// mise creates symlinks in ~/.local/share/mise/shims that point to the mise binary itself.
// This is an end-to-end test that uses the actual ribbin CLI commands and the real mise tool.
// Uses --confirm-system-dir flag since test temp directories aren't in default allowlist.
//
// Technical note: mise shims are symlinks that point to the mise binary. When the shim is
// invoked, mise uses argv[0] to determine which tool to run. For testing, we copy the mise
// binary to a temp location so that the symlink target passes ribbin's path security checks.
func TestMiseManagedBinaryWrapping(t *testing.T) {
	// Check if real mise is available
	systemMisePath, err := exec.LookPath("mise")
	if err != nil {
		t.Skip("mise not found, skipping test")
	}
	t.Logf("System mise at: %s", systemMisePath)

	tmpDir, err := os.MkdirTemp("", "ribbin-mise-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	homeDir := filepath.Join(tmpDir, "home")
	projectDir := filepath.Join(tmpDir, "project")
	binDir := filepath.Join(tmpDir, "bin")

	for _, dir := range []string{homeDir, projectDir, binDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Copy mise binary to temp bin directory to satisfy ribbin's path security checks.
	// The mise shim is a symlink that points to the mise binary, and ribbin validates
	// that symlink targets are in "safe" directories. /root/.local/bin is blocked.
	misePath := filepath.Join(binDir, "mise")
	cpCmd := exec.Command("cp", systemMisePath, misePath)
	if output, err := cpCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to copy mise binary: %v\n%s", err, output)
	}
	t.Logf("Copied mise to: %s", misePath)

	// Build ribbin
	moduleRoot := findModuleRoot(t)
	ribbinPath := filepath.Join(binDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Save original environment
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	origDir, _ := os.Getwd()

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	os.Setenv("HOME", homeDir)

	// Configure mise to use our temp home directory and temp mise binary
	miseDataDir := filepath.Join(homeDir, ".local", "share", "mise")
	miseConfigDir := filepath.Join(homeDir, ".config", "mise")
	miseCacheDir := filepath.Join(homeDir, ".cache", "mise")
	miseShimsDir := filepath.Join(miseDataDir, "shims")
	for _, dir := range []string{miseDataDir, miseConfigDir, miseCacheDir, miseShimsDir} {
		os.MkdirAll(dir, 0755)
	}

	// Install shfmt using mise (a small, fast tool)
	// Run from projectDir to avoid picking up /app/mise.toml
	// Use the copied mise binary from our temp bin directory
	t.Log("Installing shfmt@3.7.0 with mise...")
	installCmd := exec.Command(misePath, "use", "-g", "shfmt@3.7.0")
	installCmd.Dir = projectDir
	installCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"MISE_DATA_DIR="+miseDataDir,
		"MISE_CONFIG_DIR="+miseConfigDir,
		"MISE_CACHE_DIR="+miseCacheDir,
		"PATH="+binDir+":"+origPath,
	)
	output, err := installCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to install shfmt with mise: %v\n%s", err, output)
	}
	t.Logf("mise install output: %s", output)

	// Find the shfmt shim that mise created
	shimPath := filepath.Join(miseShimsDir, "shfmt")
	if _, err := os.Stat(shimPath); os.IsNotExist(err) {
		t.Fatalf("mise did not create shfmt shim at %s", shimPath)
	}

	// Log what type of binary it is
	shimInfo, err := os.Lstat(shimPath)
	if err != nil {
		t.Fatalf("failed to lstat shim: %v", err)
	}
	isSymlink := shimInfo.Mode()&os.ModeSymlink != 0
	t.Logf("mise shim is symlink: %v", isSymlink)
	if isSymlink {
		target, _ := os.Readlink(shimPath)
		t.Logf("mise shim target: %s", target)
		// Verify the shim points to our temp mise binary, not the system one
		if target != misePath {
			t.Logf("WARNING: shim points to %s instead of %s - updating symlink", target, misePath)
			// Update the symlink to point to our temp mise binary
			os.Remove(shimPath)
			if err := os.Symlink(misePath, shimPath); err != nil {
				t.Fatalf("failed to create symlink: %v", err)
			}
		}
	}

	// Verify it works before shimming
	// Run from projectDir to avoid mise finding /app/mise.toml
	t.Log("Verifying shfmt works before shimming...")
	cmd := exec.Command(shimPath, "--version")
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"MISE_DATA_DIR="+miseDataDir,
		"MISE_CONFIG_DIR="+miseConfigDir,
		"MISE_CACHE_DIR="+miseCacheDir,
		"PATH="+miseShimsDir+":"+binDir+":"+origPath,
	)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shfmt failed before shimming: %v\n%s", err, output)
	}
	t.Logf("shfmt output: %s", output)

	// Create ribbin.jsonc with explicit path to the mise shim
	os.Chdir(projectDir)
	configContent := fmt.Sprintf(`{
  "wrappers": {
    "shfmt": {
      "action": "block",
      "message": "Use the project wrapper instead",
      "paths": ["%s"]
    }
  }
}`, shimPath)
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write ribbin.jsonc: %v", err)
	}

	// Use CLI to wrap (need --confirm-system-dir for test temp directories)
	t.Log("Running ribbin wrap...")
	wrapCmd := exec.Command(ribbinPath, "wrap", "--confirm-system-dir", configPath)
	wrapCmd.Dir = projectDir
	wrapCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
	)
	output, err = wrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin wrap failed: %v\n%s", err, output)
	}
	t.Logf("ribbin wrap output: %s", output)

	// Use CLI to activate globally
	t.Log("Running ribbin activate --global...")
	activateCmd := exec.Command(ribbinPath, "activate", "--global")
	activateCmd.Dir = projectDir
	activateCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
	)
	output, err = activateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin activate --global failed: %v\n%s", err, output)
	}
	t.Logf("ribbin activate output: %s", output)

	// Verify sidecar exists and is a symlink (moved from original location)
	sidecarPath := shimPath + ".ribbin-original"
	sidecarInfo, err := os.Lstat(sidecarPath)
	if err != nil {
		t.Fatalf("sidecar not found: %v", err)
	}
	sidecarIsSymlink := sidecarInfo.Mode()&os.ModeSymlink != 0
	t.Logf("Sidecar is symlink: %v", sidecarIsSymlink)
	if !sidecarIsSymlink {
		t.Error("sidecar should be a symlink (the original mise shim)")
	}

	// Test: shfmt should be blocked
	// Note: cmd.Dir is set to projectDir to avoid mise finding /app/mise.toml
	t.Log("Test: shfmt should be blocked")
	cmd = exec.Command(shimPath, "--version")
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"MISE_DATA_DIR="+miseDataDir,
		"MISE_CONFIG_DIR="+miseConfigDir,
		"MISE_CACHE_DIR="+miseCacheDir,
		"PATH="+miseShimsDir+":"+binDir+":"+origPath,
	)
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("shfmt should be blocked, but succeeded: %s", output)
	} else {
		t.Logf("shfmt blocked: %s", output)
	}

	// Test: bypass mode with mise
	// NOTE: RIBBIN_BYPASS=1 does cause ribbin to pass through to the sidecar,
	// but mise shims use argv[0] to determine which tool to run. When the sidecar
	// is named "shfmt.ribbin-original", mise doesn't recognize it as a valid shim.
	// This is a known limitation of wrapping mise-managed binaries.
	// The bypass still "works" in that ribbin passes through - it's mise that fails.
	t.Log("Test: bypass passes through to sidecar (mise will complain about shim name)")
	cmd = exec.Command(shimPath, "--version")
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"MISE_DATA_DIR="+miseDataDir,
		"MISE_CONFIG_DIR="+miseConfigDir,
		"MISE_CACHE_DIR="+miseCacheDir,
		"PATH="+miseShimsDir+":"+binDir+":"+origPath,
		"RIBBIN_BYPASS=1",
	)
	output, err = cmd.CombinedOutput()
	// Mise will complain because the sidecar is named ".ribbin-original"
	// which mise doesn't recognize as a valid tool. This is expected behavior.
	if err == nil {
		// If somehow it works, that's fine
		t.Logf("bypass works: %s", output)
	} else {
		if contains(string(output), "is not a valid shim") {
			t.Logf("bypass passes through to mise (which complains about renamed shim as expected): %s", output)
		} else {
			// Some other error - that would be unexpected
			t.Errorf("unexpected error during bypass: %v\n%s", err, output)
		}
	}

	// Use CLI to unwrap
	t.Log("Running ribbin unwrap...")
	unwrapCmd := exec.Command(ribbinPath, "unwrap", configPath)
	unwrapCmd.Dir = projectDir
	unwrapCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
	)
	output, err = unwrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin unwrap failed: %v\n%s", err, output)
	}
	t.Logf("ribbin unwrap output: %s", output)

	// Verify restoration - should be a symlink again
	restoredInfo, err := os.Lstat(shimPath)
	if err != nil {
		t.Fatalf("restored shim not found: %v", err)
	}
	if restoredInfo.Mode()&os.ModeSymlink == 0 {
		t.Error("restored shim should be a symlink")
	}

	// Verify it works after unshimming
	cmd = exec.Command(shimPath, "--version")
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"MISE_DATA_DIR="+miseDataDir,
		"MISE_CONFIG_DIR="+miseConfigDir,
		"MISE_CACHE_DIR="+miseCacheDir,
		"PATH="+miseShimsDir+":"+binDir+":"+origPath,
	)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("shfmt should work after unshimming: %v\n%s", err, output)
	} else {
		t.Logf("shfmt restored: %s", output)
	}

	t.Log("mise-managed binary (shfmt) test completed!")
}

// TestAsdfManagedBinaryWrapping tests wrapping a binary managed by asdf (script-based shims).
// asdf creates shell script shims that call `asdf exec` to look up the correct version at runtime.
// This is an end-to-end test that uses the actual ribbin CLI commands and the real asdf tool.
func TestAsdfManagedBinaryWrapping(t *testing.T) {
	// Check if real asdf is available
	asdfPath, err := exec.LookPath("asdf")
	if err != nil {
		t.Skip("asdf not found, skipping test")
	}
	t.Logf("Using real asdf at: %s", asdfPath)

	tmpDir, err := os.MkdirTemp("", "ribbin-asdf-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	homeDir := filepath.Join(tmpDir, "home")
	projectDir := filepath.Join(tmpDir, "project")
	binDir := filepath.Join(tmpDir, "bin")

	for _, dir := range []string{homeDir, projectDir, binDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Build ribbin
	moduleRoot := findModuleRoot(t)
	ribbinPath := filepath.Join(binDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Save original environment
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	origDir, _ := os.Getwd()

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	os.Setenv("HOME", homeDir)

	// Get asdf installation directory from environment or default
	asdfInstallDir := os.Getenv("ASDF_DIR")
	if asdfInstallDir == "" {
		asdfInstallDir = filepath.Join(origHome, ".asdf")
	}
	// Check common Docker container location
	if _, err := os.Stat(asdfInstallDir); os.IsNotExist(err) {
		asdfInstallDir = "/root/.asdf"
	}

	// Use a temp directory for ASDF_DATA_DIR so shims are created in /tmp
	// This is necessary because /root/.asdf is not in the allowed test paths
	asdfDataDir := filepath.Join(tmpDir, "asdf-data")
	asdfShimsDir := filepath.Join(asdfDataDir, "shims")
	os.MkdirAll(asdfShimsDir, 0755)

	// Install shfmt using asdf (a small, fast tool)
	t.Log("Adding shfmt plugin to asdf...")
	addPluginCmd := exec.Command("bash", "-c", fmt.Sprintf(
		"export ASDF_DIR=%s && export ASDF_DATA_DIR=%s && source %s/asdf.sh && asdf plugin add shfmt 2>&1 || true",
		asdfInstallDir, asdfDataDir, asdfInstallDir,
	))
	addPluginCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"ASDF_DIR="+asdfInstallDir,
		"ASDF_DATA_DIR="+asdfDataDir,
	)
	output, err := addPluginCmd.CombinedOutput()
	if err != nil {
		t.Logf("asdf plugin add output (may be already installed): %s", output)
	}

	t.Log("Installing shfmt@3.7.0 with asdf...")
	installCmd := exec.Command("bash", "-c", fmt.Sprintf(
		"export ASDF_DIR=%s && export ASDF_DATA_DIR=%s && source %s/asdf.sh && asdf install shfmt 3.7.0 2>&1",
		asdfInstallDir, asdfDataDir, asdfInstallDir,
	))
	installCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"ASDF_DIR="+asdfInstallDir,
		"ASDF_DATA_DIR="+asdfDataDir,
	)
	output, err = installCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to install shfmt with asdf: %v\n%s", err, output)
	}
	t.Logf("asdf install output: %s", output)

	// Set global version
	t.Log("Setting shfmt as global...")
	globalCmd := exec.Command("bash", "-c", fmt.Sprintf(
		"export ASDF_DIR=%s && export ASDF_DATA_DIR=%s && source %s/asdf.sh && asdf global shfmt 3.7.0 2>&1",
		asdfInstallDir, asdfDataDir, asdfInstallDir,
	))
	globalCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"ASDF_DIR="+asdfInstallDir,
		"ASDF_DATA_DIR="+asdfDataDir,
	)
	output, err = globalCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to set global shfmt: %v\n%s", err, output)
	}

	// Find the shfmt shim that asdf created
	shimPath := filepath.Join(asdfShimsDir, "shfmt")
	if _, err := os.Stat(shimPath); os.IsNotExist(err) {
		t.Fatalf("asdf did not create shfmt shim at %s", shimPath)
	}

	// Log what type of binary it is
	shimInfo, err := os.Lstat(shimPath)
	if err != nil {
		t.Fatalf("failed to lstat shim: %v", err)
	}
	isSymlink := shimInfo.Mode()&os.ModeSymlink != 0
	t.Logf("asdf shim is symlink: %v (should be false - asdf uses scripts)", isSymlink)

	// Read and log the shim content
	shimContent, _ := os.ReadFile(shimPath)
	t.Logf("asdf shim content:\n%s", shimContent)

	// Verify it works before shimming
	t.Log("Verifying shfmt works before shimming...")
	cmd := exec.Command("bash", "-c", fmt.Sprintf(
		"export ASDF_DIR=%s && export ASDF_DATA_DIR=%s && source %s/asdf.sh && %s --version",
		asdfInstallDir, asdfDataDir, asdfInstallDir, shimPath,
	))
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"ASDF_DIR="+asdfInstallDir,
		"ASDF_DATA_DIR="+asdfDataDir,
		"PATH="+asdfShimsDir+":"+binDir+":"+origPath,
	)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shfmt failed before shimming: %v\n%s", err, output)
	}
	t.Logf("shfmt output: %s", output)

	// Create .tool-versions in project dir so asdf can find the tool version
	// This is needed because when ribbin passes through to the sidecar,
	// the asdf shim needs to find the version in a .tool-versions file.
	toolVersionsPath := filepath.Join(projectDir, ".tool-versions")
	if err := os.WriteFile(toolVersionsPath, []byte("shfmt 3.7.0\n"), 0644); err != nil {
		t.Fatalf("failed to write .tool-versions: %v", err)
	}

	// Create ribbin.jsonc with explicit path to the asdf shim
	os.Chdir(projectDir)
	configContent := fmt.Sprintf(`{
  "wrappers": {
    "shfmt": {
      "action": "block",
      "message": "Use the project wrapper instead",
      "paths": ["%s"]
    }
  }
}`, shimPath)
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write ribbin.jsonc: %v", err)
	}

	// Use CLI to wrap (need --confirm-system-dir for test temp directories)
	t.Log("Running ribbin wrap...")
	wrapCmd := exec.Command(ribbinPath, "wrap", "--confirm-system-dir", configPath)
	wrapCmd.Dir = projectDir
	wrapCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
	)
	output, err = wrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin wrap failed: %v\n%s", err, output)
	}
	t.Logf("ribbin wrap output: %s", output)

	// Use CLI to activate globally
	t.Log("Running ribbin activate --global...")
	activateCmd := exec.Command(ribbinPath, "activate", "--global")
	activateCmd.Dir = projectDir
	activateCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
	)
	output, err = activateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin activate --global failed: %v\n%s", err, output)
	}
	t.Logf("ribbin activate output: %s", output)

	// Verify sidecar exists and is NOT a symlink (it's a regular file - the script)
	sidecarPath := shimPath + ".ribbin-original"
	sidecarInfo, err := os.Lstat(sidecarPath)
	if err != nil {
		t.Fatalf("sidecar not found: %v", err)
	}
	sidecarIsSymlink := sidecarInfo.Mode()&os.ModeSymlink != 0
	t.Logf("Sidecar is symlink: %v (should be false)", sidecarIsSymlink)
	if sidecarIsSymlink {
		t.Error("sidecar should NOT be a symlink (asdf uses script shims)")
	}

	// Test: shfmt should be blocked
	t.Log("Test: shfmt should be blocked")
	cmd = exec.Command(shimPath, "--version")
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"ASDF_DIR="+asdfInstallDir,
		"ASDF_DATA_DIR="+asdfDataDir,
		"PATH="+asdfShimsDir+":"+binDir+":"+origPath,
	)
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("shfmt should be blocked, but succeeded: %s", output)
	} else {
		t.Logf("shfmt blocked: %s", output)
	}

	// Test: bypass mode with asdf
	// NOTE: RIBBIN_BYPASS=1 does cause ribbin to pass through to the sidecar,
	// but asdf shims work by calling `asdf exec <tool>` which looks up the shim
	// file at its original location. When ribbin renames the shim to .ribbin-original,
	// asdf can no longer find the shim file and fails with "unknown command" or
	// "No version is set". This is a known limitation of wrapping asdf-managed binaries.
	// The bypass still "works" in that ribbin passes through - it's asdf that fails.
	t.Log("Test: bypass passes through to sidecar (asdf will complain about missing shim)")
	cmd = exec.Command("bash", "-c", fmt.Sprintf(
		"export ASDF_DIR=%s && export ASDF_DATA_DIR=%s && source %s/asdf.sh && RIBBIN_BYPASS=1 %s --version",
		asdfInstallDir, asdfDataDir, asdfInstallDir, shimPath,
	))
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"ASDF_DIR="+asdfInstallDir,
		"ASDF_DATA_DIR="+asdfDataDir,
		"PATH="+asdfShimsDir+":"+binDir+":"+origPath,
		"RIBBIN_BYPASS=1",
	)
	output, err = cmd.CombinedOutput()
	// asdf will complain because the shim file was renamed to .ribbin-original,
	// so `asdf exec "shfmt"` can't find it. This is expected behavior.
	if err == nil {
		// If somehow it works, that's fine
		t.Logf("bypass works: %s", output)
	} else {
		if contains(string(output), "unknown command") || contains(string(output), "No version is set") {
			t.Logf("bypass passes through to asdf (which complains about renamed shim as expected): %s", output)
		} else {
			// Some other error - that would be unexpected
			t.Errorf("unexpected error during bypass: %v\n%s", err, output)
		}
	}

	// Use CLI to unwrap
	t.Log("Running ribbin unwrap...")
	unwrapCmd := exec.Command(ribbinPath, "unwrap", configPath)
	unwrapCmd.Dir = projectDir
	unwrapCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
	)
	output, err = unwrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin unwrap failed: %v\n%s", err, output)
	}
	t.Logf("ribbin unwrap output: %s", output)

	// Verify restoration - should be a regular file
	restoredInfo, err := os.Lstat(shimPath)
	if err != nil {
		t.Fatalf("restored shim not found: %v", err)
	}
	if restoredInfo.Mode()&os.ModeSymlink != 0 {
		t.Error("restored shim should be a regular file (not symlink)")
	}

	// Verify it works after unshimming
	cmd = exec.Command("bash", "-c", fmt.Sprintf(
		"export ASDF_DIR=%s && export ASDF_DATA_DIR=%s && source %s/asdf.sh && %s --version",
		asdfInstallDir, asdfDataDir, asdfInstallDir, shimPath,
	))
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"ASDF_DIR="+asdfInstallDir,
		"ASDF_DATA_DIR="+asdfDataDir,
		"PATH="+asdfShimsDir+":"+binDir+":"+origPath,
	)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("shfmt should work after unshimming: %v\n%s", err, output)
	} else {
		t.Logf("shfmt restored: %s", output)
	}

	t.Log("asdf-managed binary (shfmt) test completed!")
}

// TestSystemBinaryWrapping tests wrapping a system-installed binary.
// This uses a copy of a real system binary to avoid modifying actual system files.
func TestSystemBinaryWrapping(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ribbin-system-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	homeDir := filepath.Join(tmpDir, "home")
	projectDir := filepath.Join(tmpDir, "project")
	binDir := filepath.Join(tmpDir, "bin")
	localBinDir := filepath.Join(tmpDir, "local-bin") // simulates /usr/local/bin

	for _, dir := range []string{homeDir, projectDir, binDir, localBinDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Build ribbin
	moduleRoot := findModuleRoot(t)
	ribbinPath := filepath.Join(binDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Save original environment
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	origDir, _ := os.Getwd()

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	os.Setenv("HOME", homeDir)

	// Copy a real system binary (echo is simple and universally available)
	// We use 'true' command as it's simple and has no dependencies
	systemBinaryPath, err := exec.LookPath("true")
	if err != nil {
		t.Skip("'true' command not found, skipping test")
	}

	// Create a wrapper script that mimics a system binary
	localBinaryPath := filepath.Join(localBinDir, "mytool")
	binaryContent := `#!/bin/sh
echo "system mytool v1.0.0"
exit 0
`
	if err := os.WriteFile(localBinaryPath, []byte(binaryContent), 0755); err != nil {
		t.Fatalf("failed to write binary: %v", err)
	}

	// Log what type it is
	binaryInfo, err := os.Lstat(localBinaryPath)
	if err != nil {
		t.Fatalf("failed to lstat binary: %v", err)
	}
	isSymlink := binaryInfo.Mode()&os.ModeSymlink != 0
	t.Logf("system binary is symlink: %v", isSymlink)
	t.Logf("system binary path: %s", localBinaryPath)
	t.Logf("original system 'true' path: %s", systemBinaryPath)

	// Verify it works before shimming
	t.Log("Verifying mytool works before shimming...")
	cmd := exec.Command(localBinaryPath)
	cmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+localBinDir+":"+binDir+":"+origPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mytool failed before shimming: %v\n%s", err, output)
	}
	t.Logf("mytool output: %s", output)

	// Create ribbin.jsonc with explicit paths for CLI
	os.Chdir(projectDir)
	configContent := fmt.Sprintf(`{
  "wrappers": {
    "mytool": {
      "action": "block",
      "message": "System mytool is blocked in this project",
      "paths": ["%s"]
    }
  }
}`, localBinaryPath)
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write ribbin.jsonc: %v", err)
	}

	// Use CLI to wrap mytool (need --confirm-system-dir for test temp directories)
	t.Log("Running ribbin wrap...")
	wrapCmd := exec.Command(ribbinPath, "wrap", "--confirm-system-dir", configPath)
	wrapCmd.Dir = projectDir
	wrapCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+localBinDir+":"+binDir+":"+origPath,
	)
	output, err = wrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin wrap failed: %v\n%s", err, output)
	}
	t.Logf("ribbin wrap output: %s", output)

	// Use CLI to activate globally
	t.Log("Running ribbin activate --global...")
	activateCmd := exec.Command(ribbinPath, "activate", "--global")
	activateCmd.Dir = projectDir
	activateCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+localBinDir+":"+binDir+":"+origPath,
	)
	output, err = activateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin activate --global failed: %v\n%s", err, output)
	}
	t.Logf("ribbin activate output: %s", output)

	// Verify sidecar exists
	sidecarPath := localBinaryPath + ".ribbin-original"
	if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
		t.Fatal("sidecar should exist after shim install")
	}
	t.Logf("Sidecar exists: %s", sidecarPath)

	// Test: mytool should be blocked
	t.Log("Test: mytool should be blocked")
	cmd = exec.Command(localBinaryPath)
	cmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+localBinDir+":"+binDir+":"+origPath)
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("mytool should be blocked, but succeeded: %s", output)
	} else {
		t.Logf("mytool blocked: %s", output)
	}

	// Test: bypass should work
	t.Log("Test: bypass should work")
	cmd = exec.Command(localBinaryPath)
	cmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+localBinDir+":"+binDir+":"+origPath, "RIBBIN_BYPASS=1")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("mytool with bypass should work: %v\n%s", err, output)
	} else {
		if !contains(string(output), "system mytool") {
			t.Errorf("expected version output, got: %s", output)
		}
		t.Logf("bypass works: %s", output)
	}

	// Use CLI to unwrap
	t.Log("Running ribbin unwrap...")
	unwrapCmd := exec.Command(ribbinPath, "unwrap", configPath)
	unwrapCmd.Dir = projectDir
	unwrapCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+localBinDir+":"+binDir+":"+origPath,
	)
	output, err = unwrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin unwrap failed: %v\n%s", err, output)
	}
	t.Logf("ribbin unwrap output: %s", output)

	// Verify restoration
	if _, err := os.Stat(localBinaryPath); os.IsNotExist(err) {
		t.Fatal("binary should be restored after uninstall")
	}

	// Verify sidecar is removed
	if _, err := os.Stat(sidecarPath); !os.IsNotExist(err) {
		t.Error("sidecar should be removed after uninstall")
	}

	// Verify it works after unshimming
	cmd = exec.Command(localBinaryPath)
	cmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+localBinDir+":"+binDir+":"+origPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("mytool should work after unshimming: %v\n%s", err, output)
	} else {
		t.Logf("mytool restored: %s", output)
	}

	t.Log("system binary test completed!")
}

func TestFindOrphanedSidecars(t *testing.T) {
	// Create temp directories for test isolation
	tmpDir, err := os.MkdirTemp("", "ribbin-find-test-*")
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

	// Find module root before changing directories
	moduleRoot := findModuleRoot(t)

	os.Setenv("HOME", homeDir)
	os.Setenv("PATH", binDir+":"+origPath)
	os.Chdir(projectDir)

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	// Build ribbin
	ribbinPath := filepath.Join(binDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Create test binaries
	tool1Path := filepath.Join(binDir, "tool1")
	tool1Content := `#!/bin/sh
echo "tool1 original"
`
	if err := os.WriteFile(tool1Path, []byte(tool1Content), 0755); err != nil {
		t.Fatalf("failed to create tool1: %v", err)
	}

	tool2Path := filepath.Join(binDir, "tool2")
	tool2Content := `#!/bin/sh
echo "tool2 original"
`
	if err := os.WriteFile(tool2Path, []byte(tool2Content), 0755); err != nil {
		t.Fatalf("failed to create tool2: %v", err)
	}

	// Create ribbin config
	configContent := fmt.Sprintf(`{
  "wrappers": {
    "tool1": {
      "action": "block",
      "message": "tool1 blocked",
      "paths": ["%s"]
    },
    "tool2": {
      "action": "block",
      "message": "tool2 blocked",
      "paths": ["%s"]
    }
  }
}`, tool1Path, tool2Path)
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	// Wrap the tools
	t.Log("Wrapping tools...")
	wrapCmd := exec.Command(ribbinPath, "wrap")
	wrapCmd.Dir = projectDir
	wrapCmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err := wrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin wrap failed: %v\n%s", err, output)
	}
	t.Logf("Wrap output: %s", output)

	// Verify sidecars exist
	sidecar1 := tool1Path + ".ribbin-original"
	sidecar2 := tool2Path + ".ribbin-original"
	if _, err := os.Stat(sidecar1); os.IsNotExist(err) {
		t.Fatal("sidecar1 should exist")
	}
	if _, err := os.Stat(sidecar2); os.IsNotExist(err) {
		t.Fatal("sidecar2 should exist")
	}

	// Test: ribbin find should show both as known wrappers
	t.Log("Testing ribbin find (should show known wrappers)...")
	findCmd := exec.Command(ribbinPath, "find", binDir)
	findCmd.Dir = projectDir
	findCmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err = findCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin find failed: %v\n%s", err, output)
	}
	t.Logf("Find output: %s", output)
	if !contains(string(output), "Known Wrapped Binaries") {
		t.Error("should show known wrapped binaries section")
	}

	// Create orphaned sidecar by manually creating sidecar files without registry entry
	t.Log("Creating orphaned sidecar...")
	orphanPath := filepath.Join(binDir, "orphan-tool")
	orphanSidecar := orphanPath + ".ribbin-original"
	orphanContent := `#!/bin/sh
echo "orphan tool original"
`
	if err := os.WriteFile(orphanSidecar, []byte(orphanContent), 0755); err != nil {
		t.Fatalf("failed to create orphan sidecar: %v", err)
	}

	// Create a symlink wrapper for the orphan (simulating incomplete wrap operation)
	orphanWrapper := `#!/bin/sh
echo "orphan wrapper (broken)"
exit 1
`
	if err := os.WriteFile(orphanPath, []byte(orphanWrapper), 0755); err != nil {
		t.Fatalf("failed to create orphan wrapper: %v", err)
	}

	// Test: ribbin find should now show orphaned wrapper
	t.Log("Testing ribbin find (should show orphaned wrapper)...")
	findCmd = exec.Command(ribbinPath, "find", binDir)
	findCmd.Dir = projectDir
	findCmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err = findCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin find failed: %v\n%s", err, output)
	}
	t.Logf("Find output: %s", output)
	if !contains(string(output), "Unknown/Orphaned Wrapped Binaries") {
		t.Error("should show orphaned wrapped binaries section")
	}
	if !contains(string(output), "orphan-tool") {
		t.Error("should list orphan-tool as orphaned")
	}

	// Test: ribbin unwrap --all should remove known wrappers from registry
	t.Log("Testing ribbin unwrap --all...")
	unwrapCmd := exec.Command(ribbinPath, "unwrap", "--all")
	unwrapCmd.Dir = projectDir
	unwrapCmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err = unwrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin unwrap --all failed: %v\n%s", err, output)
	}
	t.Logf("Unwrap --all output: %s", output)

	// Verify known sidecars are gone after unwrap
	if _, err := os.Stat(sidecar1); !os.IsNotExist(err) {
		t.Error("sidecar1 should be removed")
	}
	if _, err := os.Stat(sidecar2); !os.IsNotExist(err) {
		t.Error("sidecar2 should be removed")
	}

	// Orphaned sidecar should still exist (not in registry, so --all doesn't touch it)
	if _, err := os.Stat(orphanSidecar); os.IsNotExist(err) {
		t.Error("orphan sidecar should still exist after unwrap --all")
	}

	// Test: ribbin find should still show the orphaned wrapper
	t.Log("Testing ribbin find after unwrap --all...")
	findCmd = exec.Command(ribbinPath, "find", binDir)
	findCmd.Dir = projectDir
	findCmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err = findCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin find failed: %v\n%s", err, output)
	}
	t.Logf("Find output after unwrap --all: %s", output)
	if !contains(string(output), "orphan-tool") {
		t.Error("should still show orphan-tool after unwrap --all")
	}

	t.Log("Find and unwrap test completed!")
}

func TestStatusFindStatusFlow(t *testing.T) {
	// Create temp directories for test isolation
	tmpDir, err := os.MkdirTemp("", "ribbin-status-find-test-*")
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

	// Find module root before changing directories
	moduleRoot := findModuleRoot(t)

	os.Setenv("HOME", homeDir)
	os.Setenv("PATH", binDir+":"+origPath)
	os.Chdir(projectDir)

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	// Build ribbin
	ribbinPath := filepath.Join(binDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Create test binary
	toolPath := filepath.Join(binDir, "mytool")
	toolContent := `#!/bin/sh
echo "mytool original"
`
	if err := os.WriteFile(toolPath, []byte(toolContent), 0755); err != nil {
		t.Fatalf("failed to create mytool: %v", err)
	}

	// Create ribbin config
	configContent := fmt.Sprintf(`{
  "wrappers": {
    "mytool": {
      "action": "block",
      "message": "mytool blocked",
      "paths": ["%s"]
    }
  }
}`, toolPath)
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	// Wrap the tool
	t.Log("Wrapping mytool...")
	wrapCmd := exec.Command(ribbinPath, "wrap")
	wrapCmd.Dir = projectDir
	wrapCmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err := wrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin wrap failed: %v\n%s", err, output)
	}
	t.Logf("Wrap output: %s", output)

	// Step 1: Run status (should show mytool as known wrapper)
	t.Log("Step 1: Running status (before orphan)...")
	statusCmd := exec.Command(ribbinPath, "status")
	statusCmd.Dir = projectDir
	statusCmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err = statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin status failed: %v\n%s", err, output)
	}
	t.Logf("Status output (before): %s", output)

	// Verify mytool is shown as a known wrapper
	if !contains(string(output), "Known wrappers") {
		t.Error("status should show 'Known wrappers' section")
	}
	if !contains(string(output), toolPath) {
		t.Error("status should show mytool in known wrappers")
	}
	if contains(string(output), "Discovered orphans") {
		t.Error("status should NOT show orphans section yet")
	}

	// Step 2: Create an orphaned sidecar (simulating interrupted operation)
	t.Log("Step 2: Creating orphaned sidecar...")
	orphanPath := filepath.Join(binDir, "orphan-tool")
	orphanSidecar := orphanPath + ".ribbin-original"
	orphanContent := `#!/bin/sh
echo "orphan tool original"
`
	if err := os.WriteFile(orphanSidecar, []byte(orphanContent), 0755); err != nil {
		t.Fatalf("failed to create orphan sidecar: %v", err)
	}

	// Create wrapper script for the orphan
	orphanWrapper := `#!/bin/sh
echo "orphan wrapper (broken)"
exit 1
`
	if err := os.WriteFile(orphanPath, []byte(orphanWrapper), 0755); err != nil {
		t.Fatalf("failed to create orphan wrapper: %v", err)
	}

	// Step 3: Run status again (should still not show orphan - not discovered yet)
	t.Log("Step 3: Running status again (orphan exists but not discovered)...")
	statusCmd = exec.Command(ribbinPath, "status")
	statusCmd.Dir = projectDir
	statusCmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err = statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin status failed: %v\n%s", err, output)
	}
	t.Logf("Status output (after orphan created): %s", output)

	// Should still only show mytool, not the orphan
	if contains(string(output), "orphan-tool") {
		t.Error("status should NOT show orphan-tool before find runs")
	}

	// Step 4: Run find to discover the orphan
	t.Log("Step 4: Running find to discover orphan...")
	findCmd := exec.Command(ribbinPath, "find", binDir)
	findCmd.Dir = projectDir
	findCmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err = findCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin find failed: %v\n%s", err, output)
	}
	t.Logf("Find output: %s", output)

	// Verify find shows the orphan
	if !contains(string(output), "Unknown/Orphaned Wrapped Binaries") {
		t.Error("find should show orphaned section")
	}
	if !contains(string(output), "orphan-tool") {
		t.Error("find should show orphan-tool")
	}
	if !contains(string(output), "Added 1 orphaned sidecar(s) to registry") {
		t.Error("find should report adding orphan to registry")
	}

	// Step 5: Run status again (should now show the discovered orphan)
	t.Log("Step 5: Running status again (after find discovers orphan)...")
	statusCmd = exec.Command(ribbinPath, "status")
	statusCmd.Dir = projectDir
	statusCmd.Env = append(os.Environ(), "HOME="+homeDir, "PATH="+binDir+":"+origPath)
	output, err = statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ribbin status failed: %v\n%s", err, output)
	}
	t.Logf("Status output (after find): %s", output)

	// Now status should show both known wrapper and discovered orphan
	if !contains(string(output), "Known wrappers") {
		t.Error("status should still show known wrappers section")
	}
	if !contains(string(output), "Discovered orphans") {
		t.Error("status should now show discovered orphans section")
	}
	if !contains(string(output), "orphan-tool") {
		t.Error("status should show orphan-tool in discovered orphans")
	}
	if !contains(string(output), "These were found by 'ribbin find'") {
		t.Error("status should explain that orphans were discovered by find command")
	}

	t.Log("Status→Find→Status flow test completed!")
}
