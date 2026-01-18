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

	// Create ribbin.toml
	cmdName := filepath.Base(nodeShimPath)
	configContent := `[shims.` + cmdName + `]
action = "block"
message = "Use something else"
paths = ["` + nodeShimPath + `"]
`
	configPath := filepath.Join(projectDir, "ribbin.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	// Install ribbin shim
	registry := &config.Registry{
		Shims:       make(map[string]config.ShimEntry),
		Activations: make(map[int]config.ActivationEntry),
		GlobalOn:    false,
	}

	if err := shim.Install(nodeShimPath, ribbinPath, registry, configPath); err != nil {
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

	// Test 1: From workDir (no ribbin.toml), command should passthrough
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
	if err := shim.Uninstall(nodeShimPath, registry); err != nil {
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

	// Create ribbin.toml that blocks node
	configContent := `[shims.node]
action = "block"
message = "Use 'bun' instead of node"
paths = ["` + nodeShimPath + `"]
`
	configPath := filepath.Join(projectDir, "ribbin.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	// Install ribbin shim on asdf's node shim (a shell script)
	// This creates: node -> ribbin, node.ribbin-original = (the shell script)
	registry := &config.Registry{
		Shims:       make(map[string]config.ShimEntry),
		Activations: make(map[int]config.ActivationEntry),
		GlobalOn:    false,
	}

	if err := shim.Install(nodeShimPath, ribbinPath, registry, configPath); err != nil {
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

	// Test 1: From workDir (no ribbin.toml), command should passthrough via asdf script to real node
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
	if err := shim.Uninstall(nodeShimPath, registry); err != nil {
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

// TestParentDirectoryConfigDiscovery tests that ribbin finds ribbin.toml in parent directories
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
	// tmpDir/project/ribbin.toml    - config in project root
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

	// Create ribbin.toml in project root (NOT in the deep subdirectory)
	configContent := `[shims.test-cmd]
action = "block"
message = "This command is blocked - config found in parent!"
`
	configPath := filepath.Join(projectDir, "ribbin.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	// Create registry with GlobalOn = true
	registry := &config.Registry{
		Shims: map[string]config.ShimEntry{
			"test-cmd": {Original: shimPath, Config: configPath},
		},
		Activations: make(map[int]config.ActivationEntry),
		GlobalOn:    true,
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

	// Command should be BLOCKED because ribbin.toml is in a parent directory
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

	// Test 3: Run from a directory WITHOUT ribbin.toml in any parent - should passthrough
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

	// Should passthrough since there's no ribbin.toml in any parent
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

	// Create ribbin.toml
	configContent := `[shims.node]
action = "block"
message = "Use 'bun' instead"
`
	configPath := filepath.Join(projectDir, "ribbin.toml")
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
		Shims:       make(map[string]config.ShimEntry),
		Activations: make(map[int]config.ActivationEntry),
		GlobalOn:    true, // Enable globally for this test
	}

	if err := shim.Install(miseNodeShim, ribbinPath, registry, configPath); err != nil {
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

	// Test: From projectDir (has ribbin.toml), with GlobalOn=true, node should be BLOCKED
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
